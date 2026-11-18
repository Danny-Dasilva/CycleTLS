package cycletls

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	http2 "github.com/Danny-Dasilva/fhttp/http2"
)

func TestNewHTTP2FingerprintParsesAkamaiFormat(t *testing.T) {
	f, err := NewHTTP2Fingerprint(readmeFirefoxH2)
	if err != nil {
		t.Fatalf("parse README example: %v", err)
	}

	type frame struct {
		id, dep   uint32
		exclusive bool
		weight    uint8
	}
	want := []frame{{3, 0, false, 200}, {5, 0, false, 100}, {7, 0, false, 0}, {9, 7, false, 0}, {11, 3, false, 0}, {13, 0, false, 240}}
	var got []frame
	for _, p := range f.PriorityFrames {
		got = append(got, frame{p.StreamID, p.StreamDep, p.Exclusive, p.Weight})
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PriorityFrames = %+v, want %+v", got, want)
	}
	if f.Exclusive || f.NoPriorityFrames {
		t.Errorf("Exclusive=%v NoPriorityFrames=%v, want both false for a frame list", f.Exclusive, f.NoPriorityFrames)
	}
	if order := f.PseudoHeaderOrder(); !reflect.DeepEqual(order, pseudoFirefox) {
		t.Errorf("PseudoHeaderOrder = %v, want %v", order, pseudoFirefox)
	}
	if got := f.String(); got != readmeFirefoxH2 {
		t.Errorf("String() = %q, want the input back", got)
	}

	// An exclusive entry, and "," between settings.
	f, err = NewHTTP2Fingerprint("1:65536,4:131072|100|3:1:0:256|a,m,p,s")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(f.PriorityFrames) != 1 || !f.PriorityFrames[0].Exclusive || f.PriorityFrames[0].Weight != 255 {
		t.Errorf("PriorityFrames = %+v, want one exclusive frame with wire weight 255", f.PriorityFrames)
	}
	if got := f.String(); got != "1:65536,4:131072|100|3:1:0:256|a,m,p,s" {
		t.Errorf("String() = %q", got)
	}
}

func TestNewHTTP2FingerprintLegacyFlagAndSeparators(t *testing.T) {
	cases := []struct {
		in               string
		exclusive        bool
		noPriorityFrames bool
		settings         int
	}{
		{"1:65536,2:0,4:6291456,6:262144|15663105|0|m,a,s,p", false, true, 4},
		{"1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p", false, true, 4},
		{"1:65536,2:0,4:6291456,6:262144|15663105|1|m,a,s,p", true, false, 4},
		{"1:65536;2:0,4:6291456;6:262144|15663105|0|m,a,s,p", false, true, 4},
		{"1:65536,2:0,4:6291456,14906:0|15663105|0|", false, true, 4},
	}
	for _, tc := range cases {
		f, err := NewHTTP2Fingerprint(tc.in)
		if err != nil {
			t.Errorf("%q: %v", tc.in, err)
			continue
		}
		if f.Exclusive != tc.exclusive || f.NoPriorityFrames != tc.noPriorityFrames || len(f.Settings) != tc.settings || len(f.PriorityFrames) != 0 {
			t.Errorf("%q: Exclusive=%v NoPriorityFrames=%v settings=%d frames=%d", tc.in, f.Exclusive, f.NoPriorityFrames, len(f.Settings), len(f.PriorityFrames))
		}
	}

	// An empty fourth part keeps the User-Agent's pseudo-header order.
	f, err := NewHTTP2Fingerprint("1:65536|15663105|0|")
	if err != nil {
		t.Fatal(err)
	}
	if order := f.PseudoHeaderOrder(); order != nil {
		t.Errorf("PseudoHeaderOrder for empty part = %v, want nil", order)
	}
}

func TestNewHTTP2FingerprintRejectsBadInput(t *testing.T) {
	for _, in := range []string{
		"1:65536|15663105|0",                  // three parts
		"|15663105|0|m,a,s,p",                 // no settings
		"1=65536|15663105|0|m,a,s,p",          // bad setting
		"1:65536|abc|0|m,a,s,p",               // bad window
		"1:65536|15663105|x|m,a,s,p",          // bad priority part
		"1:65536|15663105|3:0:0|m,a,s,p",      // three fields
		"1:65536|15663105|3:0:0:0|m,a,s,p",    // weight 0
		"1:65536|15663105|3:0:0:257|m,a,s,p",  // weight 257
		"1:65536|15663105|3:2:0:1|m,a,s,p",    // exclusive 2
		"1:65536|15663105|0:0:0:1|m,a,s,p",    // stream 0
		"1:65536|15663105|3:0:0:1,|m,a,s,p",   // trailing comma
		"1:65536|15663105|0|m,a,s",            // missing :path
		"1:65536|15663105|0|m,a,s,s",          // duplicate
		"1:65536|15663105|0|m,a,s,x",          // unknown token
		"1:65536|15663105|0|m,a,s,p,m",        // five tokens
		"70000:1|15663105|0|m,a,s,p",          // setting ID over 16 bits
		"1:4294967296|15663105|0|m,a,s,p",     // setting value over 32 bits
		"1:65536|15663105|2147483648:0:0:1|m", // stream ID over 31 bits
	} {
		if _, err := NewHTTP2Fingerprint(in); err == nil {
			t.Errorf("%q: parsed, want an error", in)
		}
	}
}

// Apply on a Firefox transport with NoPriorityFrames must not leave fhttp to
// write its six legacy PRIORITY frames, and must not pass an empty frame list
// (fhttp would then open stream 0).
func TestHTTP2FingerprintApplyFirefoxNoPriorityFrames(t *testing.T) {
	f, err := NewHTTP2Fingerprint("1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s")
	if err != nil {
		t.Fatal(err)
	}
	transport := &http2.Transport{Navigator: http2.Firefox}
	f.Apply(transport)
	if transport.Navigator != http2.Chrome {
		t.Errorf("Navigator = %q, want %q so that no PRIORITY frames are written", transport.Navigator, http2.Chrome)
	}
	if transport.HTTP2Settings.PriorityFrames != nil {
		t.Errorf("PriorityFrames = %v, want nil", transport.HTTP2Settings.PriorityFrames)
	}

	// The legacy "1" flag leaves a Firefox transport alone.
	f, err = NewHTTP2Fingerprint("1:65536|12517377|1|m,p,a,s")
	if err != nil {
		t.Fatal(err)
	}
	transport = &http2.Transport{Navigator: http2.Firefox}
	f.Apply(transport)
	if transport.Navigator != http2.Firefox {
		t.Errorf("Navigator = %q, want it unchanged for the legacy flag", transport.Navigator)
	}
}

func TestHTTP2PriorityOptionOnWire(t *testing.T) {
	const firefoxNoFrames = "1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s"

	cases := []struct {
		name     string
		options  Options
		want     *h2Priority
		wantPrio []h2Priority
		stream   uint32
	}{
		{
			name: "chrome exclusive 256",
			options: Options{
				HTTP2Fingerprint: chromeStyleH2, UserAgent: testChromeUA,
				HTTP2Priority: &HTTP2PriorityOptions{Weight: 256, Exclusive: true},
			},
			want:   &h2Priority{Exclusive: true, Weight: 255},
			stream: 1,
		},
		{
			name: "firefox weight 42",
			options: Options{
				HTTP2Fingerprint: firefoxNoFrames, UserAgent: testFirefoxUA,
				HTTP2Priority: &HTTP2PriorityOptions{Weight: 42},
			},
			want:   &h2Priority{Weight: 41},
			stream: 1,
		},
		{
			name: "weight zero means 256",
			options: Options{
				HTTP2Fingerprint: chromeStyleH2, UserAgent: testChromeUA,
				HTTP2Priority: &HTTP2PriorityOptions{},
			},
			want:   &h2Priority{Weight: 255},
			stream: 1,
		},
		{
			name: "stream dependency with README frames",
			options: Options{
				HTTP2Fingerprint: readmeFirefoxH2, UserAgent: testFirefoxUA,
				HTTP2Priority: &HTTP2PriorityOptions{Weight: 42, StreamDep: 13},
			},
			want:     &h2Priority{Weight: 41, StreamDep: 13},
			wantPrio: firefoxLegacyPriorities,
			stream:   15,
		},
		{
			name: "false with fingerprint sends none",
			options: Options{
				HTTP2Fingerprint: "2:0;3:100;4:2097152;9:1|10420225|0|m,s,a,p", UserAgent: testChromeUA,
				HTTP2Priority: &HTTP2PriorityOptions{Disabled: true},
			},
			want:   nil,
			stream: 1,
		},
		{
			name: "no fingerprint, chrome defaults kept",
			options: Options{
				UserAgent:     testChromeUA,
				HTTP2Priority: &HTTP2PriorityOptions{Weight: 100, Exclusive: true},
			},
			want:   &h2Priority{Exclusive: true, Weight: 99},
			stream: 1,
		},
		{
			name: "no fingerprint, firefox defaults kept",
			options: Options{
				UserAgent:     testFirefoxUA,
				HTTP2Priority: &HTTP2PriorityOptions{Weight: 42, StreamDep: 13},
			},
			want:     &h2Priority{Weight: 41, StreamDep: 13},
			wantPrio: firefoxLegacyPriorities,
			stream:   15,
		},
		{
			name: "false without fingerprint sends none",
			options: Options{
				UserAgent:     testChromeUA,
				HTTP2Priority: &HTTP2PriorityOptions{Disabled: true},
			},
			want:   nil,
			stream: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := captureHTTP2(t, tc.options)
			assertHeadersPriority(t, got.HeadersPriority, tc.want)
			assertPriorities(t, got.Priorities, tc.wantPrio)
			if got.HeadersStreamID != tc.stream {
				t.Errorf("first request stream = %d, want %d", got.HeadersStreamID, tc.stream)
			}
		})
	}

	// Without a fingerprint the option must not disturb the browser preset.
	got := captureHTTP2(t, Options{UserAgent: testChromeUA, HTTP2Priority: &HTTP2PriorityOptions{Weight: 100}})
	if s := got.settingsString(); s != "1:65536,3:1000,4:6291456,5:16384,6:262144" {
		t.Errorf("SETTINGS = %s, want the Chrome preset", s)
	}
	if got.WindowUpdate != 15663105 {
		t.Errorf("WINDOW_UPDATE = %d, want 15663105", got.WindowUpdate)
	}
}

func TestHTTP2PriorityInvalidWeightFailsRequest(t *testing.T) {
	client := Init()
	defer client.Close()
	// Nothing listens here; the request must fail on the option either way.
	resp, err := client.Do("https://127.0.0.1:1/", Options{
		UserAgent:     testChromeUA,
		HTTP2Priority: &HTTP2PriorityOptions{Weight: 300},
		Timeout:       2,
	}, "GET")
	if err == nil && resp.Status == 200 {
		t.Fatalf("weight 300 was accepted")
	}

	transport := &http2.Transport{Navigator: http2.Chrome}
	if err := applyHTTP2Priority(transport, &HTTP2PriorityOptions{Weight: 300}); err == nil {
		t.Errorf("applyHTTP2Priority accepted weight 300")
	}
	if err := applyHTTP2Priority(transport, &HTTP2PriorityOptions{Weight: -1}); err == nil {
		t.Errorf("applyHTTP2Priority accepted weight -1")
	}
}

func TestHTTP2PriorityJSON(t *testing.T) {
	decode := func(t *testing.T, body string) (Options, error) {
		t.Helper()
		var opts Options
		err := json.Unmarshal([]byte(body), &opts)
		return opts, err
	}

	opts, err := decode(t, `{"url":"https://example.com","http2Priority":{"weight":42,"exclusive":true,"streamDep":13}}`)
	if err != nil {
		t.Fatal(err)
	}
	if want := (&HTTP2PriorityOptions{Weight: 42, Exclusive: true, StreamDep: 13}); !reflect.DeepEqual(opts.HTTP2Priority, want) {
		t.Errorf("object: got %+v, want %+v", opts.HTTP2Priority, want)
	}

	opts, err = decode(t, `{"http2Priority":false}`)
	if err != nil {
		t.Fatal(err)
	}
	if opts.HTTP2Priority == nil || !opts.HTTP2Priority.Disabled {
		t.Errorf("false: got %+v, want Disabled", opts.HTTP2Priority)
	}
	if b := browserFromOptions(opts); b.HTTP2Priority == nil || !b.HTTP2Priority.Disabled {
		t.Errorf("browserFromOptions dropped http2Priority: %+v", b.HTTP2Priority)
	}

	for _, body := range []string{`{}`, `{"http2Priority":null}`} {
		opts, err = decode(t, body)
		if err != nil {
			t.Fatal(err)
		}
		if opts.HTTP2Priority != nil {
			t.Errorf("%s: got %+v, want nil", body, opts.HTTP2Priority)
		}
	}

	for _, body := range []string{`{"http2Priority":{"weight":300}}`, `{"http2Priority":"high"}`, `{"http2Priority":{"weight":-1}}`} {
		if _, err := decode(t, body); err == nil {
			t.Errorf("%s: decoded, want an error", body)
		}
	}

	out, err := json.Marshal(Options{HTTP2Priority: &HTTP2PriorityOptions{Disabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"http2Priority":false`) {
		t.Errorf("marshal disabled: %s", out)
	}
	out, err = json.Marshal(Options{HTTP2Priority: &HTTP2PriorityOptions{Weight: 42}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"http2Priority":{"weight":42,"exclusive":false,"streamDep":0}`) {
		t.Errorf("marshal object: %s", out)
	}
}

// Clients that differ only in the new options must not share a pool entry.
func TestClientKeyCoversHTTP2PriorityAndProxyHeaders(t *testing.T) {
	base := generateClientKey(Browser{UserAgent: testChromeUA}, 5, false, "http://127.0.0.1:8080")
	variants := map[string]Browser{
		"priority":          {UserAgent: testChromeUA, HTTP2Priority: &HTTP2PriorityOptions{Weight: 42}},
		"priority disabled": {UserAgent: testChromeUA, HTTP2Priority: &HTTP2PriorityOptions{Disabled: true}},
		"priority zero":     {UserAgent: testChromeUA, HTTP2Priority: &HTTP2PriorityOptions{}},
		"proxy headers":     {UserAgent: testChromeUA, ProxyHeaders: map[string]string{"X-Token": "a"}},
		"proxy headers b":   {UserAgent: testChromeUA, ProxyHeaders: map[string]string{"X-Token": "b"}},
	}
	seen := map[string]string{base: "base"}
	for name, browser := range variants {
		key := generateClientKey(browser, 5, false, "http://127.0.0.1:8080")
		if other, dup := seen[key]; dup {
			t.Errorf("%q shares a pool key with %q", name, other)
		}
		seen[key] = name
	}

	a := generateClientKey(Browser{ProxyHeaders: map[string]string{"A": "1", "B": "2", "C": "3"}}, 5, false, "")
	for i := 0; i < 20; i++ {
		if b := generateClientKey(Browser{ProxyHeaders: map[string]string{"C": "3", "B": "2", "A": "1"}}, 5, false, ""); a != b {
			t.Fatalf("pool key depends on map order")
		}
	}
}
