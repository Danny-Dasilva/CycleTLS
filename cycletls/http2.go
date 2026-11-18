package cycletls

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	http "github.com/Danny-Dasilva/fhttp"
	http2 "github.com/Danny-Dasilva/fhttp/http2"
)

// HTTP2Fingerprint represents an HTTP/2 client fingerprint.
//
// The string form is the Akamai format that tls.peet.ws, azuretls, curl_cffi
// and tls-client print:
//
//	SETTINGS|WINDOW_UPDATE|PRIORITY|PSEUDO_HEADER_ORDER
//
//	SETTINGS             id:value pairs separated by "," or ";"
//	WINDOW_UPDATE        connection window increment sent after SETTINGS
//	PRIORITY             "0" for no PRIORITY frames, or a comma-separated list
//	                     of streamID:exclusive:dependsOn:weight entries
//	                     (exclusive is 0 or 1, weight is 1-256). A bare "1" is
//	                     the 3.0.0 meaning: no list, exclusive HEADERS priority.
//	PSEUDO_HEADER_ORDER  a permutation of m,a,s,p (:method, :authority,
//	                     :scheme, :path). Empty keeps the User-Agent's order.
//
// Examples:
//
//	"1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p"
//	"1:65536;4:131072;5:16384|12517377|3:0:0:201,5:0:0:101,7:0:0:1,9:0:7:1,11:0:3:1,13:0:0:241|m,p,a,s"
type HTTP2Fingerprint struct {
	Settings       []http2.Setting
	ConnectionFlow uint32
	// StreamDependency is a deprecated alias for ConnectionFlow kept for
	// backward compatibility with the pre-3.0 Go API. New code should set
	// ConnectionFlow. NewHTTP2Fingerprint and Apply keep both fields in sync.
	//
	// Deprecated: use ConnectionFlow instead.
	StreamDependency uint32
	// Exclusive is the exclusive bit of the HEADERS frame priority. The string
	// form sets it only for the legacy bare "1" in the PRIORITY part.
	Exclusive bool
	// PriorityOrder holds the fourth part of the string as written: the
	// pseudo-header order in short form ("m", "a", "s", "p"). The name is
	// historical. PseudoHeaderOrder returns the full names.
	PriorityOrder []string
	// StreamDep is the parent stream dependency reported by the priority
	// frame (per RFC 7540 §5.3). When zero, Apply falls back to a
	// Chrome-like default of 0 (no parent).
	StreamDep uint32
	// Weight is the priority weight (0-255). When zero, Apply falls back
	// to a Chrome-like default of 255.
	Weight uint8

	// PriorityFrames are the PRIORITY frames sent after WINDOW_UPDATE and
	// before the first request, in order. Weight here is the wire byte, which
	// is the RFC 7540 weight minus one. The first request uses the stream after
	// the last frame's stream (last + 2).
	PriorityFrames []http2.PriorityFrame
	// NoPriorityFrames says the fingerprint asks for no PRIORITY frames at all
	// (the PRIORITY part is "0"). Without it a Firefox User-Agent makes fhttp
	// send Firefox's six legacy PRIORITY frames.
	NoPriorityFrames bool

	settingsSeparator string
}

// HTTP2PriorityOptions sets the priority fields of the HEADERS frame that opens
// each HTTP/2 request. In JSON it is either an object,
//
//	{"weight": 256, "exclusive": true, "streamDep": 0}
//
// or the literal false, which sends HEADERS without priority fields.
type HTTP2PriorityOptions struct {
	// Weight is the RFC 7540 weight, 1-256, as browsers and Akamai strings
	// report it. The wire byte is Weight-1. Zero means 256.
	Weight int `json:"weight"`
	// Exclusive sets the exclusive-dependency bit.
	Exclusive bool `json:"exclusive"`
	// StreamDep is the stream this one depends on. Zero means none.
	StreamDep uint32 `json:"streamDep"`
	// Disabled sends the HEADERS frame without priority fields. JSON false
	// sets it.
	Disabled bool `json:"-"`
}

// UnmarshalJSON accepts an object or false. true is read as "use the defaults".
func (p *HTTP2PriorityOptions) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	switch string(trimmed) {
	case "false":
		*p = HTTP2PriorityOptions{Disabled: true}
		return nil
	case "true":
		*p = HTTP2PriorityOptions{}
		return nil
	}

	type plain HTTP2PriorityOptions
	var decoded plain
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return fmt.Errorf("http2Priority must be an object or false: %w", err)
	}
	*p = HTTP2PriorityOptions(decoded)
	_, err := p.priorityParam()
	return err
}

// MarshalJSON writes false for a disabled priority and the object otherwise.
func (p HTTP2PriorityOptions) MarshalJSON() ([]byte, error) {
	if p.Disabled {
		return []byte("false"), nil
	}
	type plain HTTP2PriorityOptions
	return json.Marshal(plain(p))
}

// priorityParam converts the options to fhttp's wire form.
func (p *HTTP2PriorityOptions) priorityParam() (http2.PriorityParam, error) {
	weight := p.Weight
	if weight == 0 {
		weight = 256
	}
	if weight < 1 || weight > 256 {
		return http2.PriorityParam{}, fmt.Errorf("http2Priority weight must be 1-256, got %d", p.Weight)
	}
	if p.StreamDep > maxHTTP2StreamID {
		return http2.PriorityParam{}, fmt.Errorf("http2Priority streamDep must fit in 31 bits, got %d", p.StreamDep)
	}
	return http2.PriorityParam{
		StreamDep: p.StreamDep,
		Exclusive: p.Exclusive,
		Weight:    uint8(weight - 1),
	}, nil
}

// key is the part of the pooled-client key that depends on these options.
func (p *HTTP2PriorityOptions) key() string {
	if p == nil {
		return "nil"
	}
	if p.Disabled {
		return "off"
	}
	return fmt.Sprintf("%d:%t:%d", p.Weight, p.Exclusive, p.StreamDep)
}

const maxHTTP2StreamID = 1<<31 - 1

var pseudoHeaderNames = map[string]string{
	"m": ":method",
	"a": ":authority",
	"s": ":scheme",
	"p": ":path",
}

// NewHTTP2Fingerprint parses an Akamai-format HTTP/2 fingerprint. See
// HTTP2Fingerprint for the grammar.
func NewHTTP2Fingerprint(fingerprint string) (*HTTP2Fingerprint, error) {
	parts := strings.Split(fingerprint, "|")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid HTTP/2 fingerprint format: expected 4 parts, got %d", len(parts))
	}

	// Part 1: SETTINGS. Peers print both "," and ";" between pairs.
	settingsStr := strings.TrimSpace(parts[0])
	separator := ","
	if strings.Contains(settingsStr, ";") && !strings.Contains(settingsStr, ",") {
		separator = ";"
	}
	settingsParts := strings.FieldsFunc(settingsStr, func(r rune) bool { return r == ',' || r == ';' })
	if len(settingsParts) == 0 {
		return nil, fmt.Errorf("invalid setting format: %q - expected ID:VALUE", parts[0])
	}
	settings := make([]http2.Setting, 0, len(settingsParts))
	for _, setting := range settingsParts {
		idStr, valStr, ok := strings.Cut(strings.TrimSpace(setting), ":")
		if !ok {
			return nil, fmt.Errorf("invalid setting format: %s - expected ID:VALUE", setting)
		}
		id, err := strconv.ParseUint(idStr, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid setting format: %s", setting)
		}
		val, err := strconv.ParseUint(valStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid setting format: %s", setting)
		}
		settings = append(settings, http2.Setting{ID: http2.SettingID(id), Val: uint32(val)})
	}

	// Part 2: connection-level WINDOW_UPDATE increment.
	flow, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid connection flow: %s", parts[1])
	}

	f := &HTTP2Fingerprint{
		Settings:       settings,
		ConnectionFlow: uint32(flow),
		// Mirror into the deprecated alias so callers reading either field
		// observe the same value.
		StreamDependency:  uint32(flow),
		PriorityOrder:     strings.Split(parts[3], ","),
		settingsSeparator: separator,
	}

	// Part 3: PRIORITY frames, or the legacy 0/1 exclusive flag.
	priorityStr := strings.TrimSpace(parts[2])
	if strings.Contains(priorityStr, ":") {
		for _, entry := range strings.Split(priorityStr, ",") {
			frame, err := parsePriorityEntry(entry)
			if err != nil {
				return nil, err
			}
			f.PriorityFrames = append(f.PriorityFrames, frame)
		}
	} else {
		flag, err := strconv.ParseUint(priorityStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid priority part: %q - expected 0, 1 or streamID:exclusive:dependsOn:weight entries", parts[2])
		}
		f.Exclusive = flag != 0
		f.NoPriorityFrames = flag == 0
	}

	// Part 4: pseudo-header order. Empty keeps the User-Agent's order.
	if strings.TrimSpace(parts[3]) != "" {
		if _, err := pseudoHeaderOrder(f.PriorityOrder); err != nil {
			return nil, err
		}
	}

	return f, nil
}

// parsePriorityEntry parses one streamID:exclusive:dependsOn:weight entry.
func parsePriorityEntry(entry string) (http2.PriorityFrame, error) {
	bad := func(reason string) (http2.PriorityFrame, error) {
		return http2.PriorityFrame{}, fmt.Errorf("invalid priority entry %q: %s (expected streamID:exclusive:dependsOn:weight)", entry, reason)
	}

	fields := strings.Split(strings.TrimSpace(entry), ":")
	if len(fields) != 4 {
		return bad("need four fields")
	}
	streamID, err := strconv.ParseUint(fields[0], 10, 32)
	if err != nil || streamID == 0 || streamID > maxHTTP2StreamID {
		return bad("stream ID must be 1 to 2^31-1")
	}
	exclusive, err := strconv.ParseUint(fields[1], 10, 8)
	if err != nil || exclusive > 1 {
		return bad("exclusive must be 0 or 1")
	}
	dependsOn, err := strconv.ParseUint(fields[2], 10, 32)
	if err != nil || dependsOn > maxHTTP2StreamID {
		return bad("dependsOn must be 0 to 2^31-1")
	}
	weight, err := strconv.ParseUint(fields[3], 10, 16)
	if err != nil || weight < 1 || weight > 256 {
		return bad("weight must be 1 to 256")
	}

	return http2.PriorityFrame{
		FrameHeader: http2.FrameHeader{StreamID: uint32(streamID)},
		PriorityParam: http2.PriorityParam{
			StreamDep: uint32(dependsOn),
			Exclusive: exclusive == 1,
			Weight:    uint8(weight - 1),
		},
	}, nil
}

// pseudoHeaderOrder maps m,a,s,p tokens to pseudo-header names. All four must
// appear exactly once: fhttp skips a pseudo-header that the order leaves out,
// and a request without :path or :authority is not valid.
func pseudoHeaderOrder(short []string) ([]string, error) {
	order := make([]string, 0, len(short))
	seen := make(map[string]bool, len(short))
	for _, token := range short {
		token = strings.ToLower(strings.TrimSpace(token))
		name, ok := pseudoHeaderNames[token]
		if !ok || seen[token] {
			return nil, fmt.Errorf("invalid pseudo-header order %q: expected m, a, s and p once each", strings.Join(short, ","))
		}
		seen[token] = true
		order = append(order, name)
	}
	if len(order) != len(pseudoHeaderNames) {
		return nil, fmt.Errorf("invalid pseudo-header order %q: expected m, a, s and p once each", strings.Join(short, ","))
	}
	return order, nil
}

// PseudoHeaderOrder returns the pseudo-header names in the order the
// fingerprint asks for, or nil when it does not give a full order.
func (f *HTTP2Fingerprint) PseudoHeaderOrder() []string {
	order, err := pseudoHeaderOrder(f.PriorityOrder)
	if err != nil {
		return nil
	}
	return order
}

// String returns the string representation of the HTTP/2 fingerprint
func (f *HTTP2Fingerprint) String() string {
	// Format settings
	settingStrs := make([]string, len(f.Settings))
	for i, setting := range f.Settings {
		settingStrs[i] = fmt.Sprintf("%d:%d", setting.ID, setting.Val)
	}
	separator := f.settingsSeparator
	if separator == "" {
		separator = ","
	}
	settingsStr := strings.Join(settingStrs, separator)

	// Format the PRIORITY part: the frame list, or the legacy exclusive flag.
	priorityPart := "0"
	if len(f.PriorityFrames) > 0 {
		entries := make([]string, len(f.PriorityFrames))
		for i, frame := range f.PriorityFrames {
			exclusive := 0
			if frame.Exclusive {
				exclusive = 1
			}
			entries[i] = fmt.Sprintf("%d:%d:%d:%d", frame.StreamID, exclusive, frame.StreamDep, int(frame.Weight)+1)
		}
		priorityPart = strings.Join(entries, ",")
	} else if f.Exclusive {
		priorityPart = "1"
	}

	// Format pseudo-header order
	priorityStr := strings.Join(f.PriorityOrder, ",")

	return fmt.Sprintf("%s|%d|%s|%s", settingsStr, f.ConnectionFlow, priorityPart, priorityStr)
}

// Apply configures the HTTP/2 connection with the specified fingerprint.
//
// HeaderPriority is driven by the parsed Exclusive / StreamDep / Weight
// fields. When Weight is zero (e.g. fingerprint string did not carry
// priority info), a Chrome-like default of 255 is used. The deprecated
// StreamDependency alias is honoured when ConnectionFlow is unset.
//
// PriorityFrames are written as given. With NoPriorityFrames set and a Firefox
// transport, Apply switches the transport's Navigator to Chrome: fhttp has no
// other way to send zero PRIORITY frames and open stream 1 (an empty frame list
// would start at stream 0). SETTINGS and WINDOW_UPDATE still come from the
// fingerprint, so only the PRIORITY frames change.
func (f *HTTP2Fingerprint) Apply(conn *http2.Transport) {
	connectionFlow := f.ConnectionFlow
	if connectionFlow == 0 && f.StreamDependency != 0 {
		connectionFlow = f.StreamDependency
	}

	weight := f.Weight
	if weight == 0 {
		weight = 255
	}

	settings := &http2.HTTP2Settings{
		Settings:       f.Settings,
		ConnectionFlow: int(connectionFlow),
		HeaderPriority: &http2.PriorityParam{
			Exclusive: f.Exclusive,
			StreamDep: f.StreamDep,
			Weight:    weight,
		},
	}

	switch {
	case len(f.PriorityFrames) > 0:
		settings.PriorityFrames = f.PriorityFrames
	case f.NoPriorityFrames && conn.Navigator == http2.Firefox:
		// Keep Firefox's values for anything the fingerprint leaves unset
		// before handing the PRIORITY decision to the Chrome branch.
		if settings.ConnectionFlow == 0 {
			settings.ConnectionFlow = 12517377
		}
		if settings.Settings == nil {
			settings.Settings = []http2.Setting{
				{ID: http2.SettingHeaderTableSize, Val: 65536},
				{ID: http2.SettingInitialWindowSize, Val: 131072},
				{ID: http2.SettingMaxFrameSize, Val: 16384},
			}
		}
		conn.Navigator = http2.Chrome
	}

	conn.HTTP2Settings = settings
}

// applyHTTP2Priority applies the http2Priority option on top of whatever the
// fingerprint (or nothing) put on the transport.
func applyHTTP2Priority(conn *http2.Transport, priority *HTTP2PriorityOptions) error {
	if priority == nil {
		return nil
	}
	if priority.Disabled {
		if conn.HTTP2Settings != nil {
			conn.HTTP2Settings.HeaderPriority = nil
		}
		return nil
	}

	param, err := priority.priorityParam()
	if err != nil {
		return err
	}
	if conn.HTTP2Settings == nil {
		// No fingerprint: an HTTP2Settings with only HeaderPriority keeps
		// fhttp's per-browser SETTINGS, WINDOW_UPDATE and PRIORITY frames.
		conn.HTTP2Settings = &http2.HTTP2Settings{}
	}
	conn.HTTP2Settings.HeaderPriority = &param
	return nil
}

// parsedHTTP2Fingerprint parses rt.HTTP2Fingerprint once per roundTripper.
func (rt *roundTripper) parsedHTTP2Fingerprint() (*HTTP2Fingerprint, error) {
	rt.h2FingerprintOnce.Do(func() {
		if rt.HTTP2Fingerprint == "" {
			return
		}
		rt.h2Fingerprint, rt.h2FingerprintErr = NewHTTP2Fingerprint(rt.HTTP2Fingerprint)
	})
	return rt.h2Fingerprint, rt.h2FingerprintErr
}

// configureHTTP2Transport applies http2Fingerprint and http2Priority to a new
// HTTP/2 transport.
func (rt *roundTripper) configureHTTP2Transport(conn *http2.Transport) error {
	fingerprint, err := rt.parsedHTTP2Fingerprint()
	if err != nil {
		return fmt.Errorf("failed to parse HTTP/2 fingerprint: %v", err)
	}
	if fingerprint != nil {
		fingerprint.Apply(conn)
	}
	if err := applyHTTP2Priority(conn, rt.HTTP2Priority); err != nil {
		return fmt.Errorf("invalid HTTP/2 priority: %v", err)
	}
	return nil
}

// applyPseudoHeaderOrder lets the fourth part of http2Fingerprint decide the
// pseudo-header order. Callers set it from the User-Agent before RoundTrip;
// the fingerprint is the more specific instruction, so it wins.
func (rt *roundTripper) applyPseudoHeaderOrder(req *http.Request) {
	if rt.ForceHTTP3 || rt.HTTP2Fingerprint == "" || req.Header == nil {
		return
	}
	fingerprint, err := rt.parsedHTTP2Fingerprint()
	if err != nil || fingerprint == nil {
		// A bad fingerprint is reported when the transport is built.
		return
	}
	if order := fingerprint.PseudoHeaderOrder(); order != nil {
		req.Header[http.PHeaderOrderKey] = order
	}
}
