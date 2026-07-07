package cycletls

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/Danny-Dasilva/CycleTLS/cycletls/state"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

const (
	// pongWait is the time allowed to read the next pong message.
	pongWait = 30 * time.Second
	// pingPeriod is the interval between ping messages (before pong expiration).
	pingPeriod = (pongWait * 9) / 10
	// writeWait is the time allowed for writing a message.
	writeWait = 10 * time.Second
	// requestTimeout is the maximum duration for a single request.
	requestTimeout = 2 * time.Hour
	// MaxCreditsPerMessage is the maximum credit value a client can send in a
	// single credit message (1 GB). This prevents a malicious client from
	// sending MAX_UINT32 credits to exhaust server resources.
	MaxCreditsPerMessage uint32 = 1 << 30 // 1 GiB
)

// validateCredits checks that a credit value is within the allowed range.
func validateCredits(credits uint32) error {
	if credits > MaxCreditsPerMessage {
		return fmt.Errorf("credits %d exceed maximum allowed %d", credits, MaxCreditsPerMessage)
	}
	return nil
}

// ErrCommandSenderClosed is returned by safeCommandSender.Send when the sender
// has been closed.
var ErrCommandSenderClosed = fmt.Errorf("command sender closed")

// safeCommandSender wraps a WebSocket command channel with a mutex to
// prevent data races between concurrent sends and close operations.
type safeCommandSender struct {
	mu     sync.Mutex
	ch     chan WebSocketCommandV2
	closed bool
	// done is closed by Close to unblock any in-flight Send calls.
	done chan struct{}
	// wg tracks in-flight Send calls so Close can wait for them to leave the
	// channel send before closing ch (preventing a send-on-closed panic).
	wg sync.WaitGroup
}

// newSafeCommandSender creates a new safe command sender wrapping the given channel.
func newSafeCommandSender(ch chan WebSocketCommandV2) *safeCommandSender {
	return &safeCommandSender{ch: ch, done: make(chan struct{})}
}

// Send delivers a command through the protected channel, blocking until the
// command is queued, the context is cancelled, or the sender is closed.
// It never silently drops: a full channel causes it to wait rather than
// discard the command. Returns nil on success, ctx.Err() on cancellation, or
// ErrCommandSenderClosed if the sender has been closed. Thread-safe.
func (s *safeCommandSender) Send(ctx context.Context, cmd WebSocketCommandV2) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrCommandSenderClosed
	}
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()

	select {
	case s.ch <- cmd:
		return nil
	case <-s.done:
		return ErrCommandSenderClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close closes the channel. Safe to call multiple times and concurrently with
// Send. It signals in-flight senders to abort, waits for them to leave the
// channel send, then closes the channel so no send-on-closed panic is possible.
func (s *safeCommandSender) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.done)
	s.mu.Unlock()

	// Wait for in-flight Send calls to observe done and return before closing
	// the underlying channel.
	s.wg.Wait()
	close(s.ch)
}

// -----------------------------------------------------------------------------
// WebSocket abstraction for v2 flow control
// -----------------------------------------------------------------------------

// wsConnV2 abstracts WebSocket operations for testing and flexibility.
type wsConnV2 interface {
	ReadMessage() (int, []byte, error)
	WriteBinary([]byte) error
	Close() error
	SetCloseHandler(func(code int, text string) error)
}

// gorillaConnV2 wraps gorilla/websocket.Conn to implement wsConnV2.
type gorillaConnV2 struct {
	*websocket.Conn
}

// WriteBinary writes a binary message with proper deadline.
func (g gorillaConnV2) WriteBinary(data []byte) error {
	_ = g.Conn.SetWriteDeadline(time.Now().Add(writeWait))
	return g.Conn.WriteMessage(websocket.BinaryMessage, data)
}

// newGorillaConnV2 creates a new wrapped connection with pong handler.
func newGorillaConnV2(ws *websocket.Conn) wsConnV2 {
	_ = ws.SetReadDeadline(time.Now().Add(pongWait))

	ws.SetPongHandler(func(string) error {
		_ = ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	return gorillaConnV2{Conn: ws}
}

// -----------------------------------------------------------------------------
// V2 packet reading helpers
// -----------------------------------------------------------------------------

// readInitPacketV2 reads the first "init" packet sent by the client.
// It initializes the request and returns the initial credit window.
func readInitPacketV2(conn wsConnV2) (cycleTLSRequest, uint32, error) {
	msgType, payload, err := conn.ReadMessage()
	if err != nil {
		return cycleTLSRequest{}, 0, err
	}

	if msgType != websocket.BinaryMessage {
		return cycleTLSRequest{}, 0, fmt.Errorf("expected binary init packet")
	}

	return parseInitMessage(payload)
}

// readCreditPacketV2 reads "credit" packets sent by the client to replenish the window.
func readCreditPacketV2(conn wsConnV2, expectedRequestID string) (uint32, error) {
	msgType, payload, err := conn.ReadMessage()
	if err != nil {
		return 0, err
	}

	if msgType != websocket.BinaryMessage {
		return 0, fmt.Errorf("expected binary credit packet")
	}

	requestID, credits, err := parseCreditMessage(payload)
	if err != nil {
		return 0, err
	}

	if requestID != expectedRequestID {
		return 0, fmt.Errorf("unexpected request id %q", requestID)
	}

	return credits, nil
}

// readClientMessageV2 reads any client message (credit, ws_send, ws_close).
func readClientMessageV2(conn wsConnV2, expectedRequestID string) (ClientMessage, error) {
	msgType, payload, err := conn.ReadMessage()
	if err != nil {
		return ClientMessage{}, err
	}

	if msgType != websocket.BinaryMessage {
		return ClientMessage{}, fmt.Errorf("expected binary packet")
	}

	msg, err := parseClientMessage(payload)
	if err != nil {
		return ClientMessage{}, err
	}

	if msg.RequestID != expectedRequestID {
		return ClientMessage{}, fmt.Errorf("unexpected request id %q", msg.RequestID)
	}

	return msg, nil
}

// WebSocketCommandV2 represents a command to send to the target WebSocket.
type WebSocketCommandV2 struct {
	Type        string // "send", "close"
	MessageType int    // 1 = text, 2 = binary (for send)
	Data        []byte // message data (for send)
	CloseCode   int    // close code (for close)
	CloseReason string // close reason (for close)
}

// -----------------------------------------------------------------------------
// V2 WebSocket request handler (one connection per request)
// -----------------------------------------------------------------------------

// handleWSRequestV2 handles a single request using the v2 flow-control protocol.
// This is the new handler for ?v=2 connections.
func handleWSRequestV2(ws *websocket.Conn) {
	conn := newGorillaConnV2(ws)

	// Root context for the request with timeout
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	// errgroup allows:
	// - waiting for all goroutines
	// - propagating the first error
	g, ctx := errgroup.WithContext(ctx)

	// Write channel to WebSocket.
	// IMPORTANT: the dispatcher is the ONLY owner of closing this channel.
	writeCh := make(chan []byte, 32)

	var limiter *creditWindow
	var once sync.Once

	// cleanup definitively closes the connection.
	// It is safe to call multiple times.
	cleanup := func() {
		once.Do(func() {
			if limiter != nil {
				limiter.Close()
			}
			_ = conn.Close()
		})
	}

	defer cleanup()

	conn.SetCloseHandler(func(code int, text string) error {
		debugLogger.Println("websocket closed by peer", code, text)
		cancel()
		return nil
	})

	// -----------------------------------------------------------------
	// Ping goroutine and write
	// -----------------------------------------------------------------

	controlCh := make(chan struct{}, 1)

	// ping goroutine - sends periodic keepalive pings
	g.Go(func() error {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		defer close(controlCh) // Ensure writer goroutine unblocks when ping exits

		for {
			select {
			case <-ticker.C:
				select {
				case controlCh <- struct{}{}:
				default:
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	// -----------------------------------------------------------------
	// Init handshake
	// -----------------------------------------------------------------

	req, initialWindow, err := readInitPacketV2(conn)
	if err != nil {
		debugLogger.Print("init error: ", err)
		return
	}

	// Process the request with parent context
	res := processRequestV2(req, ctx)

	limiter = newCreditWindow(int64(initialWindow))
	res.limiter = limiter

	// -----------------------------------------------------------------
	// Writer goroutine (exclusive writer to the WebSocket)
	// -----------------------------------------------------------------

	g.Go(func() error {
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case _, ok := <-controlCh:
				if !ok {
					return nil
				}
				err := ws.WriteControl(
					websocket.PingMessage,
					nil,
					time.Now().Add(writeWait),
				)
				if err != nil {
					return err
				}
			case buf, ok := <-writeCh:
				if !ok {
					return nil
				}
				err := conn.WriteBinary(buf)
				if err != nil {
					return err
				}
			}
		}
	})

	// -----------------------------------------------------------------
	// WebSocket command channel (for bidirectional WebSocket support)
	// -----------------------------------------------------------------

	// Create command channel for WebSocket connections
	var wsCommandCh chan WebSocketCommandV2
	var cmdSender *safeCommandSender
	isWebSocket := req.Options.Protocol == "websocket"
	if isWebSocket {
		wsCommandCh = make(chan WebSocketCommandV2, 32)
		cmdSender = newSafeCommandSender(wsCommandCh)
		res.wsCommandCh = wsCommandCh
	}

	// -----------------------------------------------------------------
	// Client message reader loop (handles credits and WebSocket commands)
	// -----------------------------------------------------------------

	g.Go(func() error {
		defer cancel()
		if cmdSender != nil {
			// The reader is the sole owner of the command channel. Close waits
			// for any in-flight send and closes the channel exactly once, so no
			// send-on-closed or double-close is possible.
			defer cmdSender.Close()
		}

		debugLogger.Printf("[V2 Reader] Starting, isWebSocket=%v, protocol=%s", isWebSocket, req.Options.Protocol)

		for {
			if isWebSocket {
				// For WebSocket: handle all message types
				msg, err := readClientMessageV2(conn, req.RequestID)
				if err != nil {
					debugLogger.Printf("[V2 Reader] Error: %v", err)
					return err
				}

				debugLogger.Printf("[V2 Reader] Received message: method=%s", msg.Method)

				switch msg.Method {
				case "credit":
					// Issue #8: Validate credit values before applying
					if err := validateCredits(msg.Credits); err != nil {
						debugLogger.Printf("[V2 Reader] Invalid credits: %v", err)
						return err
					}
					limiter.Add(int64(msg.Credits))
				case "ws_send":
					debugLogger.Printf("[V2 Reader] Forwarding ws_send: type=%d, len=%d", msg.MessageType, len(msg.Data))
					// Block until the command is queued or the request is torn
					// down; never silently drop on a full channel.
					if err := cmdSender.Send(ctx, WebSocketCommandV2{
						Type:        "send",
						MessageType: msg.MessageType,
						Data:        msg.Data,
					}); err != nil {
						if ctx.Err() != nil {
							return ctx.Err()
						}
						debugLogger.Printf("[V2 Reader] ws_send could not be forwarded: %v", err)
						return err
					}
					debugLogger.Printf("[V2 Reader] ws_send forwarded successfully")
				case "ws_close":
					debugLogger.Printf("[V2 Reader] Forwarding ws_close")
					if err := cmdSender.Send(ctx, WebSocketCommandV2{
						Type:        "close",
						CloseCode:   msg.CloseCode,
						CloseReason: msg.CloseReason,
					}); err != nil {
						if ctx.Err() != nil {
							return ctx.Err()
						}
						debugLogger.Printf("[V2 Reader] ws_close could not be forwarded: %v", err)
						return err
					}
				}
			} else {
				// For HTTP/SSE: only handle credit messages
				credits, err := readCreditPacketV2(conn, req.RequestID)
				if err != nil {
					return err
				}
				// Issue #8: Validate credit values before applying
				if err := validateCredits(credits); err != nil {
					debugLogger.Printf("[V2 Reader] Invalid credits: %v", err)
					return err
				}
				limiter.Add(int64(credits))
			}
		}
	})

	// -----------------------------------------------------------------
	// Dispatcher
	// -----------------------------------------------------------------
	//
	// The dispatcher:
	// - produces frames
	// - writes to writeCh
	// - CLOSES writeCh when finished
	//

	g.Go(func() error {
		defer close(writeCh)

		sender := newFrameSender(ctx, writeCh)
		dispatcherAsyncV2(res, sender)

		// IMPORTANT:
		// At this point:
		// - no more messages will be produced
		// - the writer will drain writeCh then stop
		return nil
	})

	// -----------------------------------------------------------------
	// Final wait
	// -----------------------------------------------------------------

	if err := g.Wait(); err != nil {
		debugLogger.Println("request error:", err)
	}
}

// processRequestV2 processes a request for v2 flow control mode.
// It uses the parent context for cancellation.
func processRequestV2(request cycleTLSRequest, parentCtx context.Context) fullRequest {
	debugLogger.Printf("[V2] processRequestV2: protocol=%s, url=%s", request.Options.Protocol, request.Options.URL)

	// Handle protocol-specific clients
	switch request.Options.Protocol {
	case "websocket":
		debugLogger.Printf("[V2] Routing to dispatchWebSocketRequest")
		return dispatchWebSocketRequest(request)
	case "sse":
		return dispatchSSERequest(request)
	case "http3":
		return dispatchHTTP3Request(request)
	default:
		if request.Options.ForceHTTP3 {
			return dispatchHTTP3Request(request)
		}
		return dispatchHTTPRequestV2(request, parentCtx)
	}
}

// dispatchHTTPRequestV2 creates a request for v2 flow control mode.
func dispatchHTTPRequestV2(request cycleTLSRequest, parentCtx context.Context) fullRequest {
	// Use the parent context for cancellation
	ctx, cancel := context.WithCancel(parentCtx)

	// Create browser config
	browser := browserFromOptions(request.Options)

	// Default to true for connection reuse
	enableConnectionReuse := request.Options.EnableConnectionReuse != false

	client, err := newClientWithReuse(
		browser,
		request.Options.Timeout,
		request.Options.DisableRedirect,
		request.Options.UserAgent,
		enableConnectionReuse,
		request.Options.Proxy,
	)
	if err != nil {
		log.Printf("dispatchHTTPRequestV2: client creation failed: %v", err)
		return fullRequest{
			options: request,
			err:     fmt.Errorf("failed to create HTTP client: %w", err),
		}
	}

	// Build the HTTP request using the context-aware approach from the existing code
	req, err := buildHTTPRequest(request, ctx)
	if err != nil {
		return fullRequest{
			options: request,
			err:     err,
		}
	}

	return fullRequest{
		req:     req,
		client:  client,
		options: request,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// dispatcherAsyncV2 handles the request using flow control.
func dispatcherAsyncV2(res fullRequest, sender *frameSender) {
	limiter := res.limiter

	// Handle early errors
	if res.err != nil {
		sender.send(buildErrorFrame(res.options.RequestID, 400, res.err.Error()))
		return
	}

	// Handle SSE connections
	if res.sseClient != nil {
		dispatchSSEAsyncV2(res, sender)
		return
	}

	// Handle WebSocket connections
	if res.wsClient != nil {
		dispatchWebSocketAsyncV2(res, sender)
		return
	}

	if res.cancel != nil {
		defer res.cancel()
	}

	// Perform the HTTP request
	timeout := timeoutMilliseconds(res.options.Options.Timeout)
	resp, err := doRequestWithHeaderTimeout(res.ctx, res.cancel, res.client, res.req, timeout)
	if err != nil {
		parsedError := parseError(err)
		sender.send(buildErrorFrame(res.options.RequestID, parsedError.StatusCode, parsedError.ErrorMsg+"-> \n"+err.Error()))
		return
	}
	defer resp.Body.Close()

	// Get final URL after redirects
	finalUrl := res.options.Options.URL
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		finalUrl = resp.Request.URL.String()
	}

	// Send response frame
	responseFrame, err := buildResponseFrame(res.options.RequestID, resp.StatusCode, finalUrl, resp.Header)
	if err != nil {
		sender.send(buildErrorFrame(res.options.RequestID, 500, err.Error()))
		return
	}
	if !sender.send(responseFrame) {
		return
	}

	// Stream body with flow control
	const bufferSize = 8192
	chunkBuffer := make([]byte, bufferSize)
	ctx := res.ctx
	if ctx == nil {
		ctx = res.req.Context()
	}

	for {
		if ctx.Err() != nil {
			debugLogger.Printf("Request %s context canceled during processing", res.options.RequestID)
			return
		}

		n, readErr := resp.Body.Read(chunkBuffer)

		// Send any data we got before handling errors
		if n > 0 {
			if limiter != nil {
				if err := limiter.Acquire(ctx, int64(n)); err != nil {
					return
				}
			}
			if !sender.send(buildDataFrame(res.options.RequestID, chunkBuffer[:n])) {
				return
			}
		}

		// Handle read result
		if readErr == io.EOF {
			sender.send(buildEndFrame(res.options.RequestID))
			return
		}
		if readErr != nil {
			parsedError := parseError(readErr)
			sender.send(buildErrorFrame(res.options.RequestID, parsedError.StatusCode, parsedError.ErrorMsg))
			return
		}
	}
}

// dispatchSSEAsyncV2 handles SSE connections with flow control using native V2 protocol.
func dispatchSSEAsyncV2(res fullRequest, sender *frameSender) {
	limiter := res.limiter

	// Perform the SSE HTTP request
	timeout := timeoutMilliseconds(res.options.Options.Timeout)
	resp, err := doRequestWithHeaderTimeout(res.ctx, res.cancel, res.client, res.req, timeout)
	if err != nil {
		parsedError := parseError(err)
		sender.send(buildErrorFrame(res.options.RequestID, parsedError.StatusCode, parsedError.ErrorMsg+"-> \n"+err.Error()))
		return
	}
	defer resp.Body.Close()

	// Get final URL after redirects
	finalUrl := res.options.Options.URL
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		finalUrl = resp.Request.URL.String()
	}

	// Send response frame with headers
	responseFrame, err := buildResponseFrame(res.options.RequestID, resp.StatusCode, finalUrl, resp.Header)
	if err != nil {
		sender.send(buildErrorFrame(res.options.RequestID, 500, err.Error()))
		return
	}
	if !sender.send(responseFrame) {
		return
	}

	// Stream SSE body with flow control
	const bufferSize = 8192
	chunkBuffer := make([]byte, bufferSize)
	ctx := res.ctx
	if ctx == nil {
		ctx = res.req.Context()
	}

	for {
		if ctx.Err() != nil {
			debugLogger.Printf("SSE request %s context canceled during processing", res.options.RequestID)
			return
		}

		n, readErr := resp.Body.Read(chunkBuffer)

		// Send any data we got before handling errors
		if n > 0 {
			if limiter != nil {
				if err := limiter.Acquire(ctx, int64(n)); err != nil {
					return
				}
			}
			if !sender.send(buildDataFrame(res.options.RequestID, chunkBuffer[:n])) {
				return
			}
		}

		// Handle read result
		if readErr == io.EOF {
			sender.send(buildEndFrame(res.options.RequestID))
			return
		}
		if readErr != nil {
			parsedError := parseError(readErr)
			sender.send(buildErrorFrame(res.options.RequestID, parsedError.StatusCode, parsedError.ErrorMsg))
			return
		}
	}
}

// dispatchWebSocketAsyncV2 handles WebSocket connections with flow control using native V2 protocol.
// It supports bidirectional communication: receiving messages from the target server and
// sending messages from the TypeScript client via the wsCommandCh channel.
func dispatchWebSocketAsyncV2(res fullRequest, sender *frameSender) {
	requestID := res.options.RequestID

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in dispatchWebSocketAsyncV2 for request %s: %v", requestID, r)
		}
		state.UnregisterRequest(requestID)
		state.UnregisterWebSocket(requestID)
	}()

	// Connect to WebSocket endpoint
	conn, resp, err := res.wsClient.Connect(res.options.Options.URL)
	if err != nil {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		sender.send(buildWebSocketErrorFrame(requestID, statusCode, err.Error()))
		sender.send(buildEndFrame(requestID))
		return
	}
	defer conn.Close()

	// Extract negotiated protocol and extensions
	negotiatedProtocol := resp.Header.Get("Sec-WebSocket-Protocol")
	negotiatedExtensions := resp.Header.Get("Sec-WebSocket-Extensions")

	// Issue #3: Register both request and WebSocket for state tracking
	state.RegisterRequest(requestID, res.cancel)
	state.RegisterWebSocket(requestID, conn)

	// Issue #5: Send initial response with headers, send error frame on failure
	wsResponseFrame, wsRespErr := buildResponseFrame(requestID, resp.StatusCode, res.options.Options.URL, resp.Header)
	if wsRespErr != nil {
		sender.send(buildWebSocketErrorFrame(requestID, 500, wsRespErr.Error()))
		sender.send(buildEndFrame(requestID))
		return
	}
	if !sender.send(wsResponseFrame) {
		sender.send(buildWebSocketErrorFrame(requestID, 0, "failed to send response frame"))
		sender.send(buildEndFrame(requestID))
		return
	}

	// Issue #5: Send ws_open event, send error frame on failure
	wsOpenFrame, wsOpenErr := buildWebSocketOpenFrame(requestID, negotiatedProtocol, negotiatedExtensions)
	if wsOpenErr != nil {
		sender.send(buildWebSocketErrorFrame(requestID, 0, wsOpenErr.Error()))
		sender.send(buildEndFrame(requestID))
		return
	}
	if !sender.send(wsOpenFrame) {
		sender.send(buildWebSocketErrorFrame(requestID, 0, "failed to send ws_open frame"))
		sender.send(buildEndFrame(requestID))
		return
	}

	// If there's body data, send it as the first WebSocket message
	if res.options.Options.Body != "" {
		// Issue #7: Set write deadline before writing
		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		err := conn.WriteMessage(websocket.TextMessage, []byte(res.options.Options.Body))
		if err != nil {
			debugLogger.Printf("WebSocket write error: %s", err.Error())
		}
	}

	ctx := res.ctx
	if ctx == nil {
		ctx = res.req.Context()
	}

	// Read deadline timeout
	const wsReadDeadline = 30 * time.Second

	// Issue #2: Use context with cancel to ensure reader goroutine exits
	readerCtx, readerCancel := context.WithCancel(ctx)
	defer readerCancel() // Ensures reader goroutine exits when main loop returns

	// Channel for messages read from the target WebSocket.
	// Buffer of 2 prevents the reader goroutine from blocking when the main loop
	// exits: one slot for the in-flight message, one for the post-cancellation read.
	type wsReadResult struct {
		messageType int
		data        []byte
		err         error
	}
	readCh := make(chan wsReadResult, 2)

	// Reader goroutine with explicit cancellation via readerCtx.
	// When readerCancel is called, conn.ReadMessage may still be blocking;
	// the buffered channel ensures the goroutine can send its final result
	// without blocking, and the context check prevents further iteration.
	go func() {
		defer close(readCh) // Signal main loop that reader has exited
		for {
			_ = conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
			messageType, message, err := conn.ReadMessage()

			// Check context before attempting send to exit promptly
			if readerCtx.Err() != nil {
				return
			}

			select {
			case readCh <- wsReadResult{messageType, message, err}:
			case <-readerCtx.Done():
				return
			}

			if err != nil {
				return
			}
		}
	}()

	// Main loop: handle both incoming messages and outgoing commands
	for {
		select {
		case <-ctx.Done():
			if closeFrame, err := buildWebSocketCloseFrame(requestID, 1000, "Context canceled"); err == nil {
				sender.send(closeFrame)
			}
			sender.send(buildEndFrame(requestID))
			return

		case cmd, ok := <-res.wsCommandCh:
			if !ok {
				// Command channel closed, connection ending
				return
			}

			switch cmd.Type {
			case "send":
				// Send message to target WebSocket
				msgType := websocket.TextMessage
				if cmd.MessageType == 2 {
					msgType = websocket.BinaryMessage
				}
				// Issue #7: Set write deadline before writing
				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				err := conn.WriteMessage(msgType, cmd.Data)
				if err != nil {
					debugLogger.Printf("WebSocket send error: %s", err.Error())
					sender.send(buildWebSocketErrorFrame(requestID, 0, err.Error()))
				}

			case "close":
				// Close the target WebSocket connection
				closeCode := cmd.CloseCode
				if closeCode == 0 {
					closeCode = websocket.CloseNormalClosure
				}
				// Issue #7: Set write deadline before writing
				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				err := conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(closeCode, cmd.CloseReason))
				if err != nil {
					debugLogger.Printf("WebSocket close error: %s", err.Error())
				}
				if closeFrame, err := buildWebSocketCloseFrame(requestID, closeCode, cmd.CloseReason); err == nil {
					sender.send(closeFrame)
				}
				sender.send(buildEndFrame(requestID))
				return
			}

		case msg, ok := <-readCh:
			if !ok {
				// Reader goroutine exited and closed the channel
				return
			}
			if msg.err != nil {
				// Check if timeout - continue the loop
				if netErr, ok := msg.err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
					continue
				}

				if websocket.IsCloseError(msg.err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					if closeFrame, err := buildWebSocketCloseFrame(requestID, websocket.CloseNormalClosure, "Connection closed normally"); err == nil {
						sender.send(closeFrame)
					}
				} else {
					debugLogger.Printf("WebSocket read error: %s", msg.err.Error())
					sender.send(buildWebSocketErrorFrame(requestID, 0, msg.err.Error()))
				}
				sender.send(buildEndFrame(requestID))
				return
			}

			// Send ws_message frame to client
			if !sender.send(buildWebSocketMessageFrame(requestID, msg.messageType, msg.data)) {
				return
			}
		}
	}
}
