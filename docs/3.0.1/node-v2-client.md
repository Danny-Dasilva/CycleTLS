# 3.0.1: the default Node client

This covers the client you get from `new CycleTLS()` (the default export, in `src/flow-control-client.ts`), its wire protocol (`src/protocol.ts`) and the Go packet reader (`cycletls/packet_reader.go`). The legacy `initCycleTLS` client is not part of it.

Every "before" result below was produced on this branch before the change, on macOS arm64 with Node 22.18 and Go 1.26.11, against local HTTP and WebSocket servers only. The probe scripts are `pipeprobe2.js` and `bigbody.js` from the gap research, plus `startprobe.js` written for this work.

## 1. The client stopped answering after about 240 requests

### What was wrong

`startServer` spawned the Go server with piped stdout and stderr and never read either pipe. The Go server writes debug lines to stdout on every request, about 372 bytes each by the gap research's count. A pipe holds 64 KB. Once it was full, the server blocked inside its next log write and no request got an answer.

Probe, before:

```
default client, unread server stdout: 240/1500 requests ok in 5667ms; first failure: Request timeout
```

After:

```
default client, unread server stdout: 1500/1500 requests ok in 756ms; first failure: none
```

### What changed

`startServer` now spawns with `stdio: ["ignore", "pipe", "pipe"]` and attaches a `data` listener to both pipes, so they are always read. With `debug: true` each chunk is printed with a `[CycleTLS server]` prefix; otherwise stdout is dropped. The last 2000 characters of stderr are kept and added to the error when the server fails to start.

The pipes stay open, so a later change can still read a line from the server (an auth token, a ready line). Nothing is parsed from them today.

This holds whether or not the Go logger is silenced: the test below runs against a binary that still logs on every request.

### Test

`tests/v2-lifecycle.test.js`, "500 sequential GETs complete". Before the change it failed at request 240.

## 2. A request over 64 KB crashed Node

### What was wrong

`buildInitPacket` wrote the whole options JSON, body included, behind a 16-bit length. Anything over 65535 bytes threw. The throw happened inside a WebSocket `open` listener, outside the promise, so Node died with an uncaught exception. Binary bodies travel as base64 inside the same JSON, so they hit the limit near 48 KB.

Probe, before:

```
"post60k": "status 200, server got 60000 bytes",
"uncaughtException": "String byte length 100087 exceeds maximum 65535 for protocol encoding"
```

After:

```
"post60k": "status 200, server got 60000 bytes",
"post100k": "status 200, server got 100000 bytes"
```

### The new packet: `init2`

There are now two packets that start a request. They carry the same fields and differ only in the width of one length. All integers are big-endian.

| Field | `init` | `init2` |
|---|---|---|
| request id | u16 length, then bytes | u16 length, then bytes |
| method name | u16 length, then `init` | u16 length, then `init2` |
| initial window | u32 | u32 |
| options JSON | **u16** length, then bytes | **u32** length, then bytes |

Nothing may follow the options JSON in either packet.

The client sends `init` when the options JSON is 65535 bytes or less and `init2` when it is longer. Small requests therefore still work against a Go binary older than 3.0.1. The Go reader accepts both, so a client older than 3.0.1 works against the new binary.

The options JSON of `init2` may be at most 32 MiB (33554432 bytes). Both sides enforce it: `MAX_INIT_OPTIONS_BYTES` in `src/protocol.ts` and `MaxInitOptionsLen` in `cycletls/packet_reader.go`. That allows a text body of about 32 MiB or a binary body of about 24 MiB. The Go reader checks the declared length before it reads the field.

### What changed

- `src/protocol.ts`: `buildInitPacket` picks the packet by size and throws `InitPacketTooLargeError` above the limit. New exports: `MAX_INIT_OPTIONS_BYTES`, `InitPacketTooLargeError`.
- `src/flow-control-client.ts`: the new `encodeInit` builds the packet at the top of `request()`, `ws()` and `sse()`, before any connection is opened. Those methods are `async`, so a failure there rejects the returned promise. An over-limit request rejects with `CycleTLSError`, status 413, message `Request too large: the encoded request is N bytes, the limit is 33554432 bytes (32 MiB) including the body`. A body that is over the limit by its length alone is refused before it is copied into a JSON string. Any other encoding failure (a circular object in `headers`, for example) rejects with `CycleTLSError`, status 400, `Failed to encode request: …`. The new `sendInit` routes a failed `send` into the same rejection path.
- `cycletls/packet_reader.go`: `parseInitMessage` accepts `init` and `init2`. New: `readLongBytes`, `MaxInitOptionsLen`, `ErrInitOptionsTooLarge`.
- `cycletls/ws_handler_v2.go`: not edited. `readInitPacketV2` already hands the whole message to `parseInitMessage`.

### Tests

- Go, `cycletls/packet_init2_test.go`: `init2` with bodies from empty to 8 MB; the old `init` still parses; both packets decode the same options to the same request; a declared length over the limit fails with `ErrInitOptionsTooLarge`; truncated, trailing-byte, bad-JSON, bad-window and unknown-method packets fail; a 16-bit layout sent under the name `init2`, and the reverse, fail.
- Jest, `tests/v2-large-body.test.js`: packet choice at exactly 65535 and 65536 bytes; 100 KB POST; 1 MB text body with multi-byte characters, byte-exact; 1 MB and 8 MB random binary bodies, byte-exact; three over-limit cases and one unencodable case reject; no uncaught exception or unhandled rejection was seen and the client still works afterwards.

## 3. Starting and stopping the server

Each finding was confirmed by running `startprobe.js` before any change.

| Finding | Before | After |
|---|---|---|
| A probe connection for every request | `one request opened 2 WebSocket connections`, every time | Two for the first request, one for each later request |
| Concurrent first requests | `10 first requests -> 10 spawns` | `10 first requests -> 1 spawns` |
| Start budget fixed at 5 s | Server that needs 6 s: `rejected after 5166ms: Server failed to start` | `ok 200 after 6273ms` |
| Path with a space | `rejected: Server failed to start` | `ok 200` |
| Node exits without `close()` | `go server pid 37159 alive: true` | `alive: false` |
| SIGINT, SIGTERM | Node keeps running, Go server alive | Go server killed. See below for Node itself |

### What changed, all in `src/flow-control-client.ts`

**One shared startup.** `ensureServerRunning` keeps the startup in `startPromise`. Calls that arrive while it is pending wait on the same promise. Once a server has answered, `serverReady` is set and later calls return at once with no probe. `serverReady` is cleared when the spawned server exits, and when a request cannot connect. In the second case the new `openSocket` runs the check again, which starts a server, and connects once more, so a request made after the server died still succeeds.

**Configurable start time.** New client option `startupTimeout`, in milliseconds, default 20000 (it was a fixed 5000). `startServer` polls from 20 ms up to 100 ms apart until the server accepts a connection. It stops early when the process cannot be spawned or exits, and says why: `CycleTLS server exited during startup (code 3, signal null, port 39511); server output: cannot bind port`. At the deadline it kills the process and rejects with `CycleTLS server did not accept connections on port N within Mms (startupTimeout)`. If the server exits because another process took the port first and that process is a working server, the client uses it, as it did before.

**No shell.** The server is spawned with `shell: false`. The path reaches the OS as one argument, so spaces and shell characters are safe, and the child is the server itself, not a shell around it.

**Cleanup at exit.** While a client owns a live server, the module listens for `exit`, `SIGINT` and `SIGTERM` on `process` and kills the server in each. The listeners are added when the first server starts and removed when the last one is gone, so importing the package installs nothing. `exit` also covers `process.exit()` and an uncaught exception.

**Signals are passed on.** After killing the servers the signal handler removes itself. If no other listener for that signal is left, it sends the signal to its own process again, so the process ends by that signal exactly as it would without CycleTLS. If the application has its own listener, the application decides.

**`close()` waits.** `close()` now resolves after the server process has exited (or after 2 s). It also settles a startup that is still in flight, so a server cannot appear just after `close()` returns. A client can be used again after `close()`; the next request starts a new server.

### One part is not fixed here: SIGINT through the package entry point

`src/index.ts` registers the legacy client's SIGINT and SIGTERM listeners when the package is imported, and those listeners return without ending the process. Because they are always present, the default client sees another listener and does not re-send the signal. Result today, measured:

```
signal SIGINT: node STILL RUNNING (signal swallowed); go server pid 75833 alive: false
```

The Go server is now killed, but a script that only idles still needs a second Ctrl-C. With the client module loaded alone the process ends by the signal (tested). `src/index.ts` belongs to the legacy-client work; the fix there is to re-send the signal after cleanup when no other listener is left, or to stop registering at import. That change was left to the legacy-client work on purpose.

### Tests

`tests/v2-lifecycle.test.js`: one spawn for 10 concurrent first requests (spawn spy); `shell: false`; the child is gone when `close()` resolves; a request after `close()` and a request after the server was killed both start a new server; a path with a space; one connection per request after the first; a server that takes 6 s to start; `startupTimeout` reached; a server that exits at startup; a missing executable.

`tests/v2-exit-cleanup.test.js` runs `tests/v2-fixture-child.js` in a child Node process: `process.exit()` without `close()`; an uncaught exception; SIGINT and SIGTERM with the client module alone (the process must end by that signal and the server must be gone); SIGINT with an application listener (the application's exit code wins, the server is gone); SIGINT and SIGTERM through the package entry point (the server is gone).

The shell scripts and signals make three lifecycle tests and the signal tests POSIX only; they are skipped on Windows.

## 4. `require('cycletls')` lacked three names

### What was wrong

`src/index.ts` ends with `module.exports = CycleTLS`, which replaces the object the named exports were put on, and then re-adds names by hand. `CycleTLSError`, `CreditManager` and `Protocol` were not re-added. Probe, before: `CreditManager: undefined`, `Protocol: undefined`, and the same for `CycleTLSError`. After: `function`, `function`, `object`.

### What changed

`src/index.ts`, two places: the import next to the existing `import CycleTLS from "./flow-control-client"` now also brings in `CycleTLSError` (as `CycleTLSErrorV2`, because that file has a local interface of the same name), `CreditManager` and `Protocol`; and three `module.exports.…` lines were added to the CommonJS block. Nothing else in that file was touched.

### Test

`tests/v2-exports.test.js`: the three names exist and work, the older names are unchanged, and every value named in an `export { … }` or `export * as` line of `dist/index.d.ts` exists on the required module.

## 5. `timeout: -1` failed at once

### What was wrong

The client passed the timeout straight to `setTimeout`, which fires a negative delay after 1 ms. The Go server reads a negative timeout as "no timeout" (`cycletls/request_timeout.go`). Probe, before: `timeout -1: rejected after 2ms: Request timeout`. After: `timeout -1: ok 200 after 1505ms` against an endpoint that answers after 1.5 s.

### What changed

`normalizeTimeout` in `src/flow-control-client.ts` now decides, for the per-request option and the client option alike:

| Value | Meaning |
|---|---|
| negative | no timeout: no client timer, and `-1` is sent to Go |
| `0` | the default, 30000 ms, which is what Go does with 0 |
| above 2147483647, or `Infinity` | no timeout (a Node timer cannot hold it and would fire at once) |
| not a number | the client's default |
| anything else | that many milliseconds |

The value sent to Go is rounded up to a whole number, because Go decodes it into an integer and a fraction made the whole request fail to parse.

The same rule applies to `ws()` and `sse()`.

### Test

`tests/v2-timeout.test.js`: a table for `normalizeTimeout`; `-1`, `0` and `Infinity` against an endpoint that takes 1.5 s; a 300 ms timeout still rejects with status 408; a client created with `timeout: -1`.

## 6. Options were forwarded by a hand-written list

### What was wrong

`request()` copied 23 named fields into the object sent to Go. `ws()` and `sse()` each copied 14. Any option not named was dropped without a word, so `serverName`, `http2Fingerprint`, `cookies` and others never reached Go from `ws()` or `sse()`, and a new option would have reached Go from nowhere.

### What changed

`src/flow-control-client.ts` now has two exported lists: `FORWARDED_REQUEST_OPTIONS` (sent to Go under the same name) and `CLIENT_ONLY_REQUEST_OPTIONS` (`readTimeout`, `sseTimeout`). A type check beside them fails the build when `RequestOptions` has a key that is in neither list. `buildWireOptions` copies every listed option that is set and is used by all three methods.

New fields on `RequestOptions`, all forwarded: `tlsProfile` (string), `shuffleExtensions` (boolean), `clientHello` (hex string), `sessionResumption` (boolean), `caCerts` (PEM strings), `certPins` (strings), `http2Priority` (`{ weight?, exclusive?, streamDep? }` or `false`), `proxyHeaders` (string to string), and two that Go already had: `proxyInsecureSkipVerify` and `protocol`.

`ws()` forces `method: "GET"`, an empty body and `protocol: "websocket"`. `sse()` forces `protocol: "sse"` and adds its `Accept` and `Cache-Control` headers under the caller's.

### Test

`tests/v2-forwarded-options.test.js`: the keys of `RequestOptions`, read from the source file, equal the two lists together; a stand-in WebSocket server records the init packet and the test compares the JSON with what was passed, for `request()`, `ws()` and `sse()`, with a value for every forwarded option.

## 7. `port: 0` never worked

### What was wrong

The client kept port 0 and passed `WS_PORT=0` to the Go server, which then listened on a random port. The client went on dialling port 0, so no request could succeed, and each attempt started another server. Probe (`port0.js`), before:

```
port 0, request 1: rejected (5160ms): Server failed to start
port 0, request 2: rejected (5157ms): Server failed to start
spawns: [{"pid":84837,"port":"0"},{"pid":84857,"port":"0"}]
go processes alive after close(): 1 of 2
```

After:

```
port 0, request 1: ok 200 (38ms)
port 0, request 2: ok 200 (1ms)
spawns: [{"pid":85896,"port":"51043"}]
go processes alive after close(): 0 of 1
```

### What changed

`port: 0` now means "a free port". On the first request `connectOrStart` asks the OS for one (`pickFreePort`: listen on 0, read the port, close), keeps it for the life of the client and starts the server on it. No probe runs first, since there is no known port to probe.

The constructor now throws a `RangeError` for a port that is not an integer from 0 to 65535, and for `port: 0` with `autoSpawn: false`, where there is no server to place. A numeric string is still accepted.

Another process can take the port between the pick and the server's bind. The server then exits and the request rejects with the startup error from item 3. This was not seen in any run.

### Test

`tests/v2-lifecycle.test.js`, "port: 0": five concurrent requests and one more use one server on a non-zero port; two clients get two servers on different ports, closing one leaves the other working, and none is left after both close; the two constructor errors.

## 8. The `tlsfingerprint` suites killed each other's server

### What was wrong

All four `tests/tlsfingerprint/*.test.ts` suites built the default client on port 9119. Jest runs them in parallel workers, so they shared one Go server, owned by whichever worker won the port. When that worker closed its client, the server died under the requests the other three had in flight. Seen once here before the fix: 5 tests failed with `CycleTLSError: Connection closed`, one run out of three.

### What changed

Each suite now passes `port: 0`, so every client gets its own server on its own free port. This was chosen over fixed per-suite ports because fixed ports can still collide with another checkout running the same suites on the same machine.

The client's own rule is unchanged and was already the safe one: `close()` only kills a server that this client spawned.

### Test

`npm test -- --testPathPattern=tlsfingerprint --forceExit`, without `--runInBand`, three runs in a row: 43 passed and 2 skipped each time, exit 0.

## What you will notice

- Long-running processes keep working past 240 requests.
- Bodies up to about 32 MiB of text or 24 MiB of binary can be sent. Above that the promise rejects with status 413.
- A request that cannot be encoded rejects its promise. It no longer ends the process.
- The first request is slightly faster and later requests open one local connection, not two.
- A slow machine gets 20 s for the server to start, and `startupTimeout` changes that. A server that cannot start fails at once with the server's own message; before, it failed after 5 s with `Server failed to start`.
- The Go server is gone when Node exits, however it exits, short of `SIGKILL`.
- `await client.close()` returns only when the server process has exited.
- `timeout: -1` waits for as long as it takes. `timeout: 0` means 30 s; it used to fail at once.
- With `debug: true` the server's log lines are printed, prefixed `[CycleTLS server]`.
- `ws()` and `sse()` now send every option you pass, among them `cookies`, `serverName`, `headerOrder`, `http2Fingerprint` and `enableConnectionReuse`. Whether Go acts on each of them for WebSockets and SSE is a separate matter on the Go side.
- An install path with a space works.
- `port: 0` gives the client its own server on a free port. An invalid port throws when the client is created.

## Left out, and why

- **SIGINT through the package entry point** still needs a second Ctrl-C when the application has no listener. The cause is in `src/index.ts`, described under item 3.
- **`SIGKILL` of Node** still orphans the Go server. Node gets no chance to run anything. Fixing it needs the server to notice that its parent is gone (a closed stdin, for example), which is a change to the Go entry point.
- **Request bodies are still sent in one packet**, so the whole body is in memory on both sides, several times over at the peak. 32 MiB keeps that bounded. Larger uploads need a streamed request body, which is a new set of packets and belongs in a minor release.
- **No read limit on the server's WebSocket.** The Go reader rejects a declared options length over 32 MiB, but gorilla has already buffered the message by then. Setting `SetReadLimit` means editing `handleWSRequestV2`, which another change owns.
- **No auth token is sent.** The shipped binary does not check one today. If that is switched on, the default client has to send the token in `dial` and `testConnection`.
- **`enableConnectionReuse` and `tls13AutoRetry` defaults** are unchanged: an omitted value still reaches Go as false.
- **README**: `startupTimeout`, the body limit and the timeout rule are not in the README yet.
- **Windows** was not run. The spawn options were already shell-free there; `close()` still uses `taskkill /t /f` and then a direct kill.
