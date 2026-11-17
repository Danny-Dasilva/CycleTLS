package cycletls

import (
	"io"
	"strings"
	"testing"
)

// newTestSSEResponse builds an SSEResponse backed by an in-memory stream.
func newTestSSEResponse(stream string) *SSEResponse {
	return &SSEResponse{
		Scanner: newSSEScanner(strings.NewReader(stream)),
		client:  &SSEClient{},
	}
}

// TestNextEventReturnsEOFAtStreamEnd verifies that after all events are drained,
// NextEvent yields io.EOF so a caller looping on it gets a stop sentinel instead
// of a nil event with a nil error (a potential busy loop).
func TestNextEventReturnsEOFAtStreamEnd(t *testing.T) {
	r := newTestSSEResponse("data: hello\n\ndata: world\n\n")

	for _, want := range []string{"hello", "world"} {
		ev, err := r.NextEvent()
		if err != nil {
			t.Fatalf("unexpected error reading event: %v", err)
		}
		if ev == nil || ev.Data != want {
			t.Fatalf("expected event data %q, got %+v", want, ev)
		}
	}

	ev, err := r.NextEvent()
	if err != io.EOF {
		t.Errorf("expected io.EOF at end of stream, got event=%+v err=%v", ev, err)
	}
	if ev != nil {
		t.Errorf("expected nil event at end of stream, got %+v", ev)
	}
}

// TestNextEventLargeDataField verifies that a data field larger than bufio's
// 64KB default token limit is parsed successfully after the scanner buffer bump
// (i.e. no bufio.ErrTooLong).
func TestNextEventLargeDataField(t *testing.T) {
	payload := strings.Repeat("x", 100*1024) // 100KB > 64KB default
	r := newTestSSEResponse("data: " + payload + "\n\n")

	ev, err := r.NextEvent()
	if err != nil {
		t.Fatalf("unexpected error parsing large data field: %v", err)
	}
	if ev == nil || ev.Data != payload {
		got := 0
		if ev != nil {
			got = len(ev.Data)
		}
		t.Fatalf("expected data of length %d, got length %d", len(payload), got)
	}
}
