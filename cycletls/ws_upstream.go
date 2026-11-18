package cycletls

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	http "github.com/Danny-Dasilva/fhttp"
	"github.com/gorilla/websocket"
)

// Liveness of an upstream WebSocket. Reads carry no deadline, so a server that
// has nothing to say can stay quiet for as long as it likes. A quiet
// connection is pinged every wsPingInterval instead, and it is dropped only
// when nothing at all (no message, no ping, no pong) has arrived for
// wsIdleTimeout. Both are variables so a test can shorten them. A
// wsPingInterval of zero or less turns pings and the idle check off.
var (
	wsPingInterval = 30 * time.Second
	wsIdleTimeout  = 90 * time.Second
)

// upstreamWatch pings a quiet upstream WebSocket and closes it once it has
// been silent for the idle window.
type upstreamWatch struct {
	conn     *websocket.Conn
	interval time.Duration
	idle     time.Duration
	lastRead atomic.Int64
	timedOut atomic.Bool
	stop     chan struct{}
	stopOnce sync.Once
}

// watchUpstream starts the watch. Call it before the first read on conn, call
// touch after every message read, and Stop when the connection is done.
func watchUpstream(conn *websocket.Conn) *upstreamWatch {
	w := &upstreamWatch{
		conn:     conn,
		interval: wsPingInterval,
		idle:     wsIdleTimeout,
		stop:     make(chan struct{}),
	}
	w.touch()

	// Control frames count as signs of life too. The handlers run inside the
	// reader's ReadMessage call.
	conn.SetPongHandler(func(string) error {
		w.touch()
		return nil
	})
	answerPing := conn.PingHandler()
	conn.SetPingHandler(func(data string) error {
		w.touch()
		return answerPing(data)
	})

	if w.interval > 0 {
		go w.run()
	}
	return w
}

func (w *upstreamWatch) touch() {
	w.lastRead.Store(time.Now().UnixNano())
}

func (w *upstreamWatch) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			quiet := time.Since(time.Unix(0, w.lastRead.Load()))
			if w.idle > 0 && quiet >= w.idle {
				// Closing the connection is what wakes the blocked reader.
				w.timedOut.Store(true)
				_ = w.conn.Close()
				return
			}
			if quiet >= w.interval/2 {
				// WriteControl may run next to the connection's other writer.
				// A failed ping needs no handling here: the reader sees the
				// broken connection and reports the close.
				_ = w.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
			}
		}
	}
}

// Stop ends the watch. Safe to call more than once.
func (w *upstreamWatch) Stop() {
	w.stopOnce.Do(func() { close(w.stop) })
}

// closeInfo turns the error that ended the upstream reader into the close
// code and reason reported to the client. A close frame from the server keeps
// its own code. Anything else is 1006, the code for a connection that went
// away without a close frame.
func (w *upstreamWatch) closeInfo(err error) (code int, reason string) {
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		reason = closeErr.Text
		if reason == "" && (closeErr.Code == websocket.CloseNormalClosure || closeErr.Code == websocket.CloseGoingAway) {
			reason = "Connection closed normally"
		}
		return closeErr.Code, reason
	}
	if w.timedOut.Load() {
		return websocket.CloseAbnormalClosure, fmt.Sprintf("upstream sent nothing for %s, not even a pong", w.idle)
	}
	if err == nil {
		return websocket.CloseAbnormalClosure, "upstream connection lost"
	}
	return websocket.CloseAbnormalClosure, err.Error()
}

// webSocketHandshakeHeaders builds the headers for the WebSocket handshake:
// the caller's headers, plus User-Agent and Cookie from the userAgent and
// cookies options. gorilla/websocket matches handshake headers (Host, Upgrade,
// Connection, Sec-Websocket-*) by canonical key, so keys are canonical here.
func webSocketHandshakeHeaders(opts Options) http.Header {
	headers := make(http.Header)
	for k, values := range opts.requestHeaders() {
		key := http.CanonicalHeaderKey(k)
		headers[key] = append(headers[key], values...)
	}

	if opts.UserAgent != "" && headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", opts.UserAgent)
	}

	if len(opts.Cookies) > 0 {
		pairs := make([]string, 0, len(opts.Cookies)+1)
		if existing := strings.Join(headers.Values("Cookie"), "; "); existing != "" {
			pairs = append(pairs, existing)
		}
		for _, c := range opts.Cookies {
			pairs = append(pairs, c.Name+"="+c.Value)
		}
		headers.Set("Cookie", strings.Join(pairs, "; "))
	}
	return headers
}
