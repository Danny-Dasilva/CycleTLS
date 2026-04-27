package cycletls

import (
	"fmt"
	"strings"

	http2 "github.com/Danny-Dasilva/fhttp/http2"
)

// HTTP2Fingerprint represents an HTTP/2 client fingerprint.
//
// The fingerprint string format is:
//
//	settings|connectionFlow|exclusive|priorityOrder
//
// Example: "1:65536,2:0,4:6291456,6:262144|15663105|0|m,a,s,p"
type HTTP2Fingerprint struct {
	Settings       []http2.Setting
	ConnectionFlow uint32
	// StreamDependency is a deprecated alias for ConnectionFlow kept for
	// backward compatibility with the pre-3.0 Go API. New code should set
	// ConnectionFlow. NewHTTP2Fingerprint and Apply keep both fields in sync.
	//
	// Deprecated: use ConnectionFlow instead.
	StreamDependency uint32
	Exclusive        bool
	PriorityOrder    []string
	// StreamDep is the parent stream dependency reported by the priority
	// frame (per RFC 7540 §5.3). When zero, Apply falls back to a
	// Chrome-like default of 0 (no parent).
	StreamDep uint32
	// Weight is the priority weight (0-255). When zero, Apply falls back
	// to a Chrome-like default of 255.
	Weight uint8
}

// NewHTTP2Fingerprint creates a new HTTP2Fingerprint from string format
// Format: settings|streamDependency|exclusive|priorityOrder
// Example: "1:65536,2:0,4:6291456,6:262144|15663105|0|m,a,s,p"
func NewHTTP2Fingerprint(fingerprint string) (*HTTP2Fingerprint, error) {
	parts := strings.Split(fingerprint, "|")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid HTTP/2 fingerprint format: expected 4 parts, got %d", len(parts))
	}

	// Parse settings
	settingsStr := parts[0]

	// Determine the separator used in the settings string
	var settingsParts []string
	if strings.Contains(settingsStr, ";") && !strings.Contains(settingsStr, ",") {
		// If settings use semicolons exclusively, split by semicolon
		settingsParts = strings.Split(settingsStr, ";")
	} else {
		// Default to comma separator
		settingsParts = strings.Split(settingsStr, ",")
	}

	settings := make([]http2.Setting, 0, len(settingsParts))

	for _, setting := range settingsParts {
		var id, val uint32
		if strings.Contains(setting, ":") {
			// Handle standard format (ID:VALUE)
			_, err := fmt.Sscanf(setting, "%d:%d", &id, &val)
			if err != nil {
				return nil, fmt.Errorf("invalid setting format: %s", setting)
			}
		} else {
			return nil, fmt.Errorf("invalid setting format: %s - expected ID:VALUE", setting)
		}
		settings = append(settings, http2.Setting{ID: http2.SettingID(id), Val: val})
	}

	// Parse connection flow increment
	var connectionFlow uint32
	_, err := fmt.Sscanf(parts[1], "%d", &connectionFlow)
	if err != nil {
		return nil, fmt.Errorf("invalid connection flow: %s", parts[1])
	}

	// Parse exclusive flag
	var exclusiveFlag int
	_, err = fmt.Sscanf(parts[2], "%d", &exclusiveFlag)
	if err != nil {
		return nil, fmt.Errorf("invalid exclusive flag: %s", parts[2])
	}
	exclusive := exclusiveFlag != 0

	// Parse priority order
	priorityOrder := strings.Split(parts[3], ",")

	return &HTTP2Fingerprint{
		Settings:       settings,
		ConnectionFlow: connectionFlow,
		// Mirror into the deprecated alias so callers reading either field
		// observe the same value.
		StreamDependency: connectionFlow,
		Exclusive:        exclusive,
		PriorityOrder:    priorityOrder,
	}, nil
}

// String returns the string representation of the HTTP/2 fingerprint
func (f *HTTP2Fingerprint) String() string {
	// Format settings
	settingStrs := make([]string, len(f.Settings))
	for i, setting := range f.Settings {
		settingStrs[i] = fmt.Sprintf("%d:%d", setting.ID, setting.Val)
	}
	settingsStr := strings.Join(settingStrs, ",")

	// Format exclusive flag
	exclusiveFlag := 0
	if f.Exclusive {
		exclusiveFlag = 1
	}

	// Format priority order
	priorityStr := strings.Join(f.PriorityOrder, ",")

	return fmt.Sprintf("%s|%d|%d|%s", settingsStr, f.ConnectionFlow, exclusiveFlag, priorityStr)
}

// Apply configures the HTTP/2 connection with the specified fingerprint.
//
// HeaderPriority is driven by the parsed Exclusive / StreamDep / Weight
// fields. When Weight is zero (e.g. fingerprint string did not carry
// priority info), a Chrome-like default of 255 is used. The deprecated
// StreamDependency alias is honoured when ConnectionFlow is unset.
func (f *HTTP2Fingerprint) Apply(conn *http2.Transport) {
	connectionFlow := f.ConnectionFlow
	if connectionFlow == 0 && f.StreamDependency != 0 {
		connectionFlow = f.StreamDependency
	}

	weight := f.Weight
	if weight == 0 {
		weight = 255
	}

	conn.HTTP2Settings = &http2.HTTP2Settings{
		Settings:       f.Settings,
		ConnectionFlow: int(connectionFlow),
		HeaderPriority: &http2.PriorityParam{
			Exclusive: f.Exclusive,
			StreamDep: f.StreamDep,
			Weight:    weight,
		},
	}
}
