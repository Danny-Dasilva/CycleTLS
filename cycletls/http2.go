package cycletls

import (
	"fmt"
	"strings"

	http2 "github.com/Danny-Dasilva/fhttp/http2"
)

// HTTP2Fingerprint represents an HTTP/2 client fingerprint
type HTTP2Fingerprint struct {
	Settings         []http2.Setting
	StreamDependency uint32
	Exclusive        bool
	PriorityOrder    []string
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
	settingsParts := strings.Split(settingsStr, ",")
	settings := make([]http2.Setting, 0, len(settingsParts))

	for _, setting := range settingsParts {
		var id, val uint32
		if strings.Contains(setting, ";") {
			// Handle alternate separator format
			_, err := fmt.Sscanf(setting, "%d;%d", &id, &val)
			if err != nil {
				return nil, fmt.Errorf("invalid setting format: %s", setting)
			}
		} else {
			// Handle standard format
			_, err := fmt.Sscanf(setting, "%d:%d", &id, &val)
			if err != nil {
				return nil, fmt.Errorf("invalid setting format: %s", setting)
			}
		}
		settings = append(settings, http2.Setting{ID: http2.SettingID(id), Val: val})
	}

	// Parse stream dependency
	var streamDependency uint32
	_, err := fmt.Sscanf(parts[1], "%d", &streamDependency)
	if err != nil {
		return nil, fmt.Errorf("invalid stream dependency: %s", parts[1])
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
		Settings:         settings,
		StreamDependency: streamDependency,
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

	return fmt.Sprintf("%s|%d|%d|%s", settingsStr, f.StreamDependency, exclusiveFlag, priorityStr)
}

// Apply configures the HTTP/2 connection with the specified fingerprint
func (f *HTTP2Fingerprint) Apply(conn *http2.Transport) {
	// Set HTTP/2 settings
	conn.Settings = f.Settings

	// Set priority and weight parameters
	// Note: Currently dummy implementation as utls/http2 doesn't expose these directly
	// In a real implementation, this would configure the priority tree
}