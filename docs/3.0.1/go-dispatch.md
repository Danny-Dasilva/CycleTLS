# 3.0.1: Go request dispatch fixes

Eight faults in the Go side of request dispatch, found in 3.0.0 and fixed on `impl/3.0.1-go-dispatch`. Each section says what was wrong and how it was shown, what changed, how it is tested, what a user will notice, and what was left out.

The shipped binary is built from `src/index.go`, which only registers `cycletls.WSEndpoint`. `main()` in `cycletls/index.go` never runs.

The probe scripts named below drive the built binary through the Node clients against local servers. "Before" is `origin/version/3.0.1` at `a19d74f`; "after" is this branch. The Go tests live in `cycletls/go_dispatch_test.go`, `cycletls/go_dispatch_ws_idle_test.go` and `cycletls/state/debug_logger_test.go`. They use `httptest` servers and call the dispatchers directly, on both the legacy path (`processRequest` + `dispatcherAsync`) and the V2 path (`processRequestV2` + `dispatcherAsyncV2`). None needs the network.

To show each test fails without the fix, `go_dispatch_test.go` was copied into an untouched export of the base branch and run there. Every test in it failed. The tests in the other two files name variables and fields that did not exist before, so they cannot compile on the base branch; for those the "before" evidence is the probe.

## New names

| Name | Kind | Meaning |
|---|---|---|
| `CYCLETLS_DEBUG` | environment variable | Set to anything but empty, `0`, `false`, `off` or `no` to get DEBUG lines on stdout |
| `state.DebugEnvVar`, `state.DebugEnabled()` | Go, exported | The variable's name and its parsed value |
| `Response.HeaderValues` | Go field, `map[string][]string`, JSON `headerValues` | Every value of each response header from `Do` |
| `WebSocketClient.UseProxyDialer(proxy.ContextDialer)` | Go method, exported | Send the WebSocket through a proxy dialer |
| `WebSocketClient.ConnectContext(ctx, url)` | Go method, exported | `Connect` with a context that can stop the handshake |
| `SSEClient.ConnectRequest(req, timeout)` | Go method, exported | Open an event stream with a request the caller built |
| `wsPingInterval`, `wsIdleTimeout` | package variables, unexported | 30 s and 90 s. See item 3 |

No option was added to `Options`.

## 1. Bad input killed the Go process

**What was wrong.** `processRequest`, `dispatchHTTP3Request`, `dispatchSSERequest` and `dispatchWebSocketRequest` called `log.Fatal` when the client or the request could not be built. An unknown proxy scheme, a proxy URL with no host, a bad method or a bad URL ended the process, and with it every other request. Node's restart check looks for `panic` or `fatal error` on stderr, and `log.Fatal` prints neither, so nothing restarted.

`fatal.js`, before:

```
"2_bad_proxy_scheme": "rejected \"Request timeout after 4000ms\" in 4002ms",
"pidAfter": "none (process gone)",
"3_good_again": "rejected \"Request timeout after 4000ms\" in 4003ms"
```

After:

```
"2_bad_proxy_scheme": "resolved status=400 data=\"failed to create HTTP client: scheme ftp is not supported\" in 0ms",
"pidAfter": "70119",   (same pid as before the bad request)
"3_good_again": "resolved status=200 data={\"ok\":true} in 1ms"
```

**What changed.** In `cycletls/index.go` the seven `log.Fatal` calls on the request path now return `fullRequest{options, err}` through a new helper, `requestError`. Both dispatchers already turned `res.err` into an error frame with status 400. An eighth `log.Fatal` in `WSEndpoint`, reached by a plain HTTP request to the port, is now a log line.

A search of `cycletls/` and `src/*.go` for `log.Fatal` and `panic(` found two more places. `src/index.go` calls `log.Fatal` when the listener fails to start, which is correct. `flow_control.go` panics on a negative credit value, which cannot be reached because credits arrive as unsigned 32-bit numbers.

A panic while a request is being built was the other way to die: `readSocket` builds requests on a goroutine with no `recover`. `safeProcessRequest` now wraps the build on both paths and turns a panic into a request error.

The early error returns also cancel the context they had created. `go vet` had been flagging three of them.

**Tests.** `TestBadInputReturnsErrorFrame`: for each path and each of plain HTTP, SSE, WebSocket and HTTP/3, a bad proxy scheme, a proxy with no host, a bad URL and a bad method each give exactly one error frame, and a good request afterwards still succeeds. On the base branch this test ends the test binary.

**What a user notices.** A request with bad options now fails by itself with status 400 and a message. On the legacy client it resolves with that status, as other Go-side errors do.

## 2. One bad request poisoned the legacy connection

**What was wrong.** `Options.UnmarshalJSON` takes a string or an array of strings as a header value. For anything else (`headers: { 'x-int': 1 }`) the decode failed, `readSocket` logged it and returned, and the handler left the socket open with nobody reading it. Node went on writing requests into it and each one timed out.

`poison.js`, before:

```
"2_number_header": "rejected \"Request timeout after 4000ms\" in 4001ms",
"3_good_again": "rejected \"Request timeout after 4000ms\" in 4002ms",
```

After:

```
"2_number_header": "resolved status=400 in 1ms",
"3_good_again": "resolved status=200 in 0ms",
```

**What changed.** In `readSocket` (`cycletls/index.go`), a message whose options do not decode is answered with an error frame, `invalid request options: ...`, addressed to the `requestId` read from the raw JSON, and the loop carries on. A message that is not JSON at all has no request ID to answer, so it is logged and skipped. The legacy handler in `WSEndpoint` now closes its socket when it finishes.

**Tests.** `TestLegacySocketSurvivesBadRequest` opens a real `?v=1` socket to `WSEndpoint`, sends a non-JSON message, a request with a numeric header value, then a good request, and expects an error frame for the bad one and a full response for the good one. On the base branch it times out with no frames at all.

**What a user notices.** The request with the bad value fails at once; the client keeps working.

**Left out.** Go still rejects a numeric header value rather than turning it into a string. The Node agent does that conversion before sending. The fourth line of `poison.js`, `timeout: -1`, is a Node-side fault and is unchanged.

## 3. A WebSocket silent for 30 seconds was destroyed

**What was wrong.** Both WebSocket loops set a 30 s read deadline before each read and treated the timeout as "try again". gorilla/websocket does not allow that: after any read error the connection is failed for good.

- Legacy path: the loop retried at once, and gorilla panics on the thousandth read of a failed connection. The reader goroutine had no `recover`, so the process died.
- V2 path: the reader goroutine exited after the timeout, the main loop saw the timeout and went round again, found the channel closed and returned without sending `ws_close` or `end`.

`wsidle.js` (upstream sends nothing for 41 s), before:

```
"upstreamClosed": ["30.1s upstream saw close code=1006", "30.2s upstream saw close code=1006"],
"legacyStdio": ["30.1s ... restarting in 500ms (attempt 1 of 3) -> panic: repeated read on failed websocket connection"],
"v2ReadyStateAfter": 1, "legacyReadyStateAfter": 1, "v2send": "ok"
```

After:

```
"upstreamClosed": [],
"legacyStdio": [],
"v2ReadyStateAfter": 1, "legacyReadyStateAfter": 1, "v2send": "ok"
```

Writing the test also showed that the legacy path never sent `end` after the upstream closed. Nothing closed `closeChan`, so `dispatchWebSocketAsync` waited for a cancel that never came and kept its goroutines and the connection.

**What changed.**

- New file `cycletls/ws_upstream.go`. `upstreamWatch` replaces the read deadline. Reads now block with no deadline. A goroutine pings the upstream every `wsPingInterval` (30 s) when nothing has arrived for half that time. Any message, ping or pong counts as a sign of life. Only when nothing at all has arrived for `wsIdleTimeout` (90 s) does it close the connection, which wakes the reader. Both are package variables so a test can shorten them. A `wsPingInterval` of zero or less turns pings and the idle check off.
- `dispatchWebSocketAsync` (legacy, `index.go`): the reader never reads again after an error. Whichever goroutine stops first ends the connection; the main routine closes the socket, waits for both goroutines, and sends `end`. The client gets one `ws_close`, with the server's own close code if it sent a close frame, or 1006 and the reason otherwise. The dispatcher also stops when the client's own socket to the Go process closes.
- `dispatchWebSocketAsyncV2` (`ws_handler_v2.go`): same watch, and every way the reader can end now sends `ws_close` then `end`. A failed write to the upstream sends `ws_error`, `ws_close` 1006 and `end` where it used to send `ws_error` and carry on.
- A read that ends the connection no longer sends `ws_error`. Both Node clients emit `'error'` for that frame, and an `'error'` event with no listener throws in Node. `ws_close` with code 1006 carries the same news.

**Tests.** `TestSilentWebSocketSurvivesIdleWindow` shortens the windows to 40 ms and 300 ms, connects to a server that answers pings and sends nothing, waits four idle windows, and expects only `response` and `ws_open`, with at least one ping seen upstream. Then the server closes, once with close code 4001 and once by dropping the TCP connection, and the test expects `ws_close` (4001 or 1006) followed by `end`. `TestDeadWebSocketIsClosedAfterIdleWindow` uses a server that never answers pings and expects `ws_close` 1006 and `end` after the idle window, not before. Both run on both paths.

**What a user notices.** A quiet feed stays connected. When the upstream goes away, the socket object gets `close` with the real code. The upstream now receives a ping about every 30 s while it is quiet. A close code other than 1000 or 1001 used to arrive as an error event; it now arrives as a close event with that code.

**Left out.**

- Browsers do not send WebSocket pings, so a server could tell this client from a browser by them. They only go out on a quiet connection, and the interval is a variable, but there is no option to turn them off from a request.
- `send()` on the Node side still reports success after the upstream is gone until the close frame arrives. That check is in the TypeScript clients.
- The V2 control socket between Node and Go has its own 30 s pong deadline with a 27 s ping period (`pongWait`, `pingPeriod`), and every V2 request ends after `requestTimeout`, two hours. Neither was touched.

## 4. V2 SSE dropped the caller's headers, and SSE was GET-only

**What was wrong.** `dispatchSSERequest` put the caller's headers into the `SSEClient` and built a bare placeholder request. `dispatchSSEAsyncV2` sent the placeholder, so the server got no caller headers, no `Accept` and no User-Agent. The legacy path used `SSEClient.ConnectWithTimeout`, which did send the headers but hard-coded GET with no body. Confirmed by reading; there is no probe. On the base branch the new test shows it:

```
method = "GET", want POST
body = "", want the caller's body
header Authorization = [], want ["Bearer token-1"]
header Accept = [], want ["text/event-stream"]
```

**What changed.** `dispatchSSERequest` builds the request with `buildHTTPRequest`, the function plain HTTP requests use, so headers, header order, method, body and User-Agent are all carried. `withSSEAccept` adds `Accept: text/event-stream` before the build, so it takes its place in the header order, unless the caller sent a non-empty `Accept` in any letter case. In `cycletls/sse.go`, `SSEClient.ConnectRequest` sends a prepared request; `Connect` and `ConnectWithTimeout` now build their GET and call it. The legacy dispatcher calls `ConnectRequest` with the built request, so both paths send the same thing.

V2 SSE and HTTP/3 requests were registered in the request tracker and never removed. `dispatcherAsyncV2` now removes them, and `processRequestV2` cancels SSE, HTTP/3 and WebSocket requests when the client's socket closes, so a stream blocked on a quiet upstream no longer outlives its client.

**Tests.** `TestSSERequestCarriesHeadersMethodAndBody` sends a POST with a body, three headers and a User-Agent on both paths and checks what the server received, including the default `Accept`. `TestSSECallerAcceptWins` sends `accept`, `Accept` and `ACCEPT` and expects exactly one `Accept` header with the caller's value, and GET when no method is given.

**What a user notices.** Authorization and other headers reach SSE endpoints from the default client. POST event streams work.

**Left out.** The legacy path still fails any reply that is not status 200 with `text/event-stream`, and still does not decode a compressed stream; V2 does neither check and does decode.

## 5. `enableConnectionReuse` read as false when left out

**What was wrong.** The field is a plain `bool`. The comments and the README say reuse is the default, but a JSON request that left the field out decoded to `false`. The default-export Node client never sets it, so every one of its requests built a client, opened a connection and closed it.

**What changed.** `Options.UnmarshalJSON` (`cycletls/types.go`) shadows the field with a `*bool`. Absent or `null` means true. An explicit `false` is still honoured.

**Decision on Go `Do`.** The struct zero value was not changed. A `bool` cannot tell "left out" from `false`, and making the field a `*bool` would break every caller that writes `EnableConnectionReuse: true`. So a Go caller who builds `Options` as a struct literal still gets reuse off unless they set the field. A comment in `Do` now says so.

**Tests.** `TestEnableConnectionReuseJSONDefault` decodes `{}`, `null`, `true` and `false`, alone and inside a request, and checks the struct zero value is still false. `TestJSONRequestsReuseConnectionByDefault` sends four JSON requests with the field left out to a TLS server and expects one connection; the base branch opens four. The connection-close tests added in 3.0.0 (`connection_close_test.go`) set the field in struct literals and still pass.

**What a user notices.** The default Node client keeps connections open between requests to the same host with the same fingerprint, proxy and cookies. Requests get faster and the server sees fewer handshakes. Pass `enableConnectionReuse: false` for a fresh connection per request.

## 6. WebSocket requests ignored `proxy`

**What was wrong.** `dispatchWebSocketRequest` never read `Options.Proxy`. `NewWebSocketClient` dials with gorilla and `http.ProxyFromEnvironment`, so a WebSocket request with `proxy` set left from the caller's own address. `userAgent` and `cookies` were dropped too. On the base branch the new test reports:

```
proxy saw 0 connections, want 1: the WebSocket did not use it
User-Agent = "Go-http-client/1.1", want the userAgent option
Cookie = "theme=dark", want the header and the cookies option joined
```

**What changed.**

- `WebSocketClient.UseProxyDialer` (`cycletls/websocket.go`) sets gorilla's `NetDialContext` to a proxy dialer and clears the environment proxy. `dispatchWebSocketRequest` builds that dialer with `newConnectDialer`, the function the HTTP path uses, so HTTP, HTTPS and SOCKS proxies all work, with the same `proxyInsecureSkipVerify` rule. `connect.go` was read, not edited. A proxy URL that cannot be used fails the request. A proxy that refuses the tunnel fails it too; there is no fallback to a direct connection.
- When the proxy speaks HTTP/2, the tunnel is one stream of a shared connection. `noDeadlineConn` drops deadlines on that tunnel, because a write deadline left by one WebSocket control frame would fail the proxy connection's later writes.
- `webSocketHandshakeHeaders` (`ws_upstream.go`) adds `User-Agent` from `userAgent` when the caller sent no such header, and adds the `cookies` option to any `Cookie` header.
- The handshake uses `ConnectContext`, so cancelling the request stops it.

**Tests.** `TestWebSocketUsesProxy`, on both paths: `ws://` through an HTTP CONNECT proxy, `wss://` through the same, and `wss://` through an HTTPS proxy that answers CONNECT over HTTP/2. Each checks the proxy saw one connection for the upstream, a message arrived through it, and the upstream received the User-Agent, the joined Cookie header and a custom header. `TestWebSocketProxyFailureDoesNotDialDirect` uses a proxy that answers 403 and checks the upstream was never reached.

**What a user notices.** `proxy` on `ws()` is honoured. The handshake carries the configured User-Agent in place of `Go-http-client/1.1`.

**Not done.** The WebSocket TLS handshake is still Go's stock `crypto/tls`. `ja3`, `ja4r`, `http2Fingerprint` and `headerOrder` do not reach it. That needs a uTLS handshake in gorilla's `NetDialTLSContext` with ALPN forced to `http/1.1`, and it was out of scope. With no `proxy` set, the WebSocket still follows `HTTP_PROXY` and `HTTPS_PROXY` from the environment, as before. SOCKS proxies go through the same dialer but have no test here.

## 7. Go `Do` gaps

**What was wrong.**

- `Do` built its `Browser` without `ServerName`, `DisableGrease` and `TLS13AutoRetry`, which `browserFromOptions` does pass. The SNI override and exact JA4R matching did not work through `Do`.
- `Do` called `DecompressBody`, which reads only the first `Content-Encoding` value and has no zstd.
- `Response.Headers` is `map[string]string` and kept only the first value of each header.

On the base branch:

```
gzip_then_br: body not decoded: got 101 bytes, want 18023
zstd:         body not decoded: got 56 bytes, want 18023
server saw SNI "", want the ServerName option
```

**What changed**, all in `Do` (`cycletls/index.go`) apart from the new field:

- The three missing fields were added to the `Browser` literal. The literal was kept, not swapped for `browserFromOptions`, because two other 3.0.1 branches add fields there.
- `decodeBufferedBody` runs the body through `newDecodedBody`, the streaming decoder in `content_decoding.go` that the dispatchers use. It undoes every coding in the header, in reverse order, and knows gzip, deflate, br and zstd. The decoded size is capped at 64 MiB with the existing `readLimited`. `utils.go` was not edited and `DecompressBody` is still there for other callers.
- When the body decodes, `Content-Encoding` and `Content-Length` are removed from the response headers, since they described the compressed bytes. When it does not decode, or would pass the cap, the body and headers come back as received, as before.
- `Response.HeaderValues` holds every value. `Headers` is unchanged.

**Tests.** `TestDoDecodesEveryContentEncoding` (gzip, `gzip, br`, zstd, `GZip`), `TestDoLeavesUndecodableBodyAlone`, `TestDoSendsServerName` (a TLS server records the SNI it was sent) and `TestDoKeepsEveryResponseHeaderValue`.

**What a user notices.** A Go caller gets zstd and multi-coded bodies decoded. `Response.Headers` no longer lists `Content-Encoding` or `Content-Length` for a body that was decoded; a caller that decoded zstd by hand after checking that header will now find it absent and the body already plain. `ServerName` takes effect.

**Left out.** `DisableGrease` and `TLS13AutoRetry` are passed through but have no test of their own here. `Do` still ignores `Protocol`, still leaves `PHeader-Order` handling as it was, and still reports a failed request as a `Response` with a nil error. `Queue`, `RespChan` and `RespChanV2` are still not wired up.

## 8. Debug output was always on

**What was wrong.** `state.DebugLogger` wrote to stdout always: 372 bytes for each V2 request. The default Node client spawned the Go process with piped stdio and never read it. When the 64 KB pipe filled, Go blocked inside the next `debugLogger.Printf`.

`pipeprobe2.js`, before and after:

```
default client, unread server stdout: 240/1500 requests ok in 5355ms; first failure: Request timeout
default client, unread server stdout: 1500/1500 requests ok in 1186ms; first failure: none
```

**What changed.** In `cycletls/state/globals.go` the logger writes to `io.Discard` unless `CYCLETLS_DEBUG` is set when the process starts. Go's logger skips formatting when its output is `io.Discard`, so the calls cost almost nothing.

**Does anything read those lines?** No. The legacy Node client (`src/index.ts`, the child's `stdout` listener) prints stdout to the console unless a chunk contains `WS_AUTH_TOKEN=`. It does not parse a token out of stdout: it makes the token itself and passes it to the child in the environment. The only Go code that prints `WS_AUTH_TOKEN=` is the `main()` in `cycletls/index.go` that never runs. The default client does not read stdout at all.

**Tests.** `TestDebugOutputFollowsEnv` and `TestDebugLoggerSilentByDefault` in `cycletls/state/debug_logger_test.go`.

**What a user notices.** The legacy client no longer prints `DEBUG:` lines to the console. To get them back, set `CYCLETLS_DEBUG=1` in the environment of the Node process; the child inherits it.

**Left out.** Lines written with the standard `log` package still go to stderr: bad-message reports, the client-creation failure in V2, and the tracker clean-up notes. They are rare, and a parent that never reads stderr could in theory still fill that pipe. The TypeScript side drains both pipes in its own branch.

## Checks

Run on macOS arm64, Go 1.26.11, Node 22.18.0.

| Step | Exit |
|---|---|
| `go build ./...` in `cycletls/` | 0 |
| `go vet ./...` in `cycletls/` | 0 |
| `gofmt -l` on the changed files | 0, nothing listed |
| `go test -race ./...` in `cycletls/` | 0 |
| New tests, `-race -count=5` | 0 |
| `go test -race -tags=integration ./tests/integration/tlsfingerprint/...` | 0 |
| `npm ci --unsafe-perm` | 0 |
| `npx jest tests/respawn-limit.test.js tests/decompression.test.ts --forceExit` | 0 |
| `npx jest --testPathPattern=tlsfingerprint --forceExit --runInBand` | 0 |
| `npx jest --testPathPattern=tlsfingerprint --forceExit` (parallel workers) | 1 |

The last row fails the same way with the base branch's binary: 16 and 10 failed tests in two base runs, 12, 7 and 13 in three runs here, a different set each time, nearly all `Connection closed`. The four suites run in separate workers, all on port 9119, and run in band they pass. The likely cause is that one suite's `close()` stops the Go server the others are using; that was not confirmed. Either way the failure is the same with and without these changes.
