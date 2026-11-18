package cycletls

import (
	nhttp "net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// shortenWSIdle makes the upstream WebSocket liveness windows short enough to
// test: a ping every 40 ms, dead after 300 ms of total silence.
func shortenWSIdle(t *testing.T) (idle time.Duration) {
	t.Helper()
	oldPing, oldIdle := wsPingInterval, wsIdleTimeout
	wsPingInterval, wsIdleTimeout = 40*time.Millisecond, 300*time.Millisecond
	t.Cleanup(func() { wsPingInterval, wsIdleTimeout = oldPing, oldIdle })
	return wsIdleTimeout
}

// drainFor collects frames for d, failing if the dispatcher returns early.
func drainFor(t *testing.T, ch <-chan []byte, d time.Duration) [][]byte {
	t.Helper()
	var frames [][]byte
	timer := time.After(d)
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				t.Fatalf("dispatcher returned early; frames: %v", frameMethods(frames))
			}
			frames = append(frames, f)
		case <-timer:
			return frames
		}
	}
}

// Item 3. Before 3.0.1 an upstream WebSocket that said nothing for 30 s was
// destroyed: the read deadline fired, the legacy path then panicked the whole
// process on a repeated read, and the V2 path dropped the upstream without
// telling the client. Now a quiet upstream that still answers pings stays
// open, and when it does close the client gets ws_close and end.
func TestSilentWebSocketSurvivesIdleWindow(t *testing.T) {
	idle := shortenWSIdle(t)

	endings := []struct {
		name     string
		end      func(conn *websocket.Conn)
		wantCode int
	}{
		{
			name: "close_frame",
			end: func(conn *websocket.Conn) {
				_ = conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(4001, "server says bye"), time.Now().Add(time.Second))
			},
			wantCode: 4001,
		},
		{
			name: "dropped",
			end: func(conn *websocket.Conn) {
				_ = conn.UnderlyingConn().Close()
			},
			wantCode: websocket.CloseAbnormalClosure,
		},
	}

	for _, path := range dispatchPaths {
		for _, ending := range endings {
			t.Run(path.name+"/"+ending.name, func(t *testing.T) {
				var pings atomic.Int32
				release := make(chan struct{})
				t.Cleanup(func() {
					select {
					case <-release:
					default:
						close(release)
					}
				})

				upstream := newWSUpstream(t, false, func(conn *websocket.Conn, r *nhttp.Request) {
					answerPing := conn.PingHandler()
					conn.SetPingHandler(func(data string) error {
						pings.Add(1)
						return answerPing(data)
					})
					// Read so pings are answered, but never send a message.
					readDone := make(chan struct{})
					go func() {
						defer close(readDone)
						for {
							if _, _, err := conn.ReadMessage(); err != nil {
								return
							}
						}
					}()
					<-release
					ending.end(conn)
					select {
					case <-readDone:
					case <-time.After(2 * time.Second):
					}
				})

				ch := startDispatch(path, cycleTLSRequest{
					RequestID: "ws-idle",
					Options:   Options{URL: upstream.URL, Protocol: "websocket"},
				})

				// Stay silent for four idle windows. The connection must hold.
				frames := drainFor(t, ch, 4*idle)
				if got := frameMethods(frames); !reflect.DeepEqual(got, []string{"response", "ws_open"}) {
					t.Fatalf("frames while idle = %v, want only response and ws_open", got)
				}
				if pings.Load() == 0 {
					t.Error("upstream received no ping while idle")
				}

				// Now the upstream goes away. The client must hear about it.
				close(release)
				rest := collectFrames(t, path.name, ch)
				if got := frameMethods(rest); !reflect.DeepEqual(got, []string{"ws_close", "end"}) {
					t.Fatalf("frames after upstream closed = %v, want ws_close then end", got)
				}
				if code, _ := wsCloseInfo(t, rest); code != ending.wantCode {
					t.Errorf("close code = %d, want %d", code, ending.wantCode)
				}
			})
		}
	}
}

// An upstream that answers nothing at all, not even pings, is closed once the
// idle window has passed, and the client is told with code 1006.
func TestDeadWebSocketIsClosedAfterIdleWindow(t *testing.T) {
	idle := shortenWSIdle(t)

	for _, path := range dispatchPaths {
		t.Run(path.name, func(t *testing.T) {
			release := make(chan struct{})
			t.Cleanup(func() { close(release) })
			upstream := newWSUpstream(t, false, func(conn *websocket.Conn, r *nhttp.Request) {
				// Never read: pings go unanswered.
				<-release
			})

			started := time.Now()
			frames := collectFrames(t, path.name, startDispatch(path, cycleTLSRequest{
				RequestID: "ws-dead",
				Options:   Options{URL: upstream.URL, Protocol: "websocket"},
			}))
			if elapsed := time.Since(started); elapsed < idle {
				t.Errorf("closed after %v, before the %v idle window", elapsed, idle)
			}
			if got := frameMethods(frames); !reflect.DeepEqual(got, []string{"response", "ws_open", "ws_close", "end"}) {
				t.Fatalf("frames = %v, want response, ws_open, ws_close, end", got)
			}
			code, reason := wsCloseInfo(t, frames)
			if code != websocket.CloseAbnormalClosure || !strings.Contains(reason, "sent nothing") {
				t.Errorf("close = %d %q, want 1006 and a reason naming the silence", code, reason)
			}
		})
	}
}

// Item 7. Response.Headers keeps one value per name; HeaderValues keeps all.
func TestDoKeepsEveryResponseHeaderValue(t *testing.T) {
	srv := httptest.NewServer(nhttp.HandlerFunc(func(w nhttp.ResponseWriter, r *nhttp.Request) {
		w.Header().Add("Set-Cookie", "a=1; Path=/")
		w.Header().Add("Set-Cookie", "b=2; Path=/")
		w.Header().Add("X-Multi", "first")
		w.Header().Add("X-Multi", "second")
		w.Header().Set("X-Single", "only")
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	resp, err := CycleTLS{}.Do(srv.URL, Options{}, "GET")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]string{
		"Set-Cookie": {"a=1; Path=/", "b=2; Path=/"},
		"X-Multi":    {"first", "second"},
		"X-Single":   {"only"},
	} {
		if got := resp.HeaderValues[name]; !reflect.DeepEqual(got, want) {
			t.Errorf("HeaderValues[%s] = %q, want %q", name, got, want)
		}
	}
	// Headers is unchanged: the first value of each name.
	if resp.Headers["X-Multi"] != "first" || resp.Headers["X-Single"] != "only" {
		t.Errorf("Headers = %v, want the first value of each name", resp.Headers)
	}
}
