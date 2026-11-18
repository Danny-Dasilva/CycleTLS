# 3.0.1: the legacy Node client (`initCycleTLS`) and the Jest setup

This covers the changes to `src/index.ts`, the option types of both Node clients, and the non-fingerprint Jest job. Each section says what was wrong, what changed, how it is tested, and what a user will notice.

"Run" means I ran it on macOS arm64 with Node against the binary built from this branch. "Read" means I read the code and did not run it.

## 1. The client never recovered when the Go process died

**What was wrong.** `SharedInstance` had no `exit` listener on the child process and no `close` listener on the control WebSocket. A restart only happened when stderr matched `panic|fatal error|runtime error|address already in use`. A process killed by a signal, by the OOM killer or by `log.Fatal` writes none of those.

Run, before the change (`crashprobe.js`: init, one request, `SIGKILL` the Go process, two more requests):

```
1_before:                 resolved status=200 in 4ms
serverListeningAfterKill: no (not respawned)
2_after_sigkill:          rejected "Request timeout after 4000ms" in 4002ms
3_after_sigkill_again:    rejected "Request timeout after 4000ms" in 4000ms
```

After the change the same probe prints `yes (respawned)` and both later requests resolve with 200.

**What changed.**

- `handleSpawn` listens for the child's `close` event, and for `exit` with a 250 ms fallback timer. `close` fires after stderr has been read, so a fatal line (for example "address already in use") is still handled by the stderr path first and keeps its own rules. Both lead to the new `handleChildExit`, which ignores an exit we asked for (shutdown, a child already replaced, a failed initialization) and passes any other exit to `handleFatalError`.
- `handleFatalError` is the restart mechanism added for issue 428 and is unchanged in its limits: 3 restarts in a row, delays of 500, 1000 and 2000 ms, a fresh set after 30 s of healthy running, no restart on "address already in use". There is no second mechanism. The one addition: after initialization it calls `handleConnectionLost` before deciding on a restart.
- `handleConnectionLost` (new) drops the dead socket, rejects everything in flight, and reconnects through the same code as initialization: the host calls `createClient` and waits for its restarted child; a client that was sharing another process's server calls `checkSpawnedInstance`, so it becomes the host when the port turns out to be free. The reconnect is bounded by the init `timeout` (default 20 s) and by the respawn limit. It reads the auth token on each attempt, so it picks up the token of the restarted child.
- `createClient` now registers a `close` listener on the socket once it is open, which calls `handleConnectionLost`. This covers a socket that closes while the process stays alive.
- Each `CycleTLSClientImpl` keeps a `pending` map from request id to a function that fails that request. `SharedInstance.failInFlight` calls them. A request waiting for headers rejects. A buffered response whose body is still arriving rejects. A `responseType: 'stream'` response has its stream destroyed with the error. A WebSocket from `ws()` gets a `close` event with code 1006.
- `sendRequest` checks that the socket is open and throws at once when it is not. While a reconnect is running the message is `CycleTLS is reconnecting to its Go process after it stopped; retry the request shortly`.
- When the reconnect fails for good, later requests reject with `CycleTLS lost its Go process and could not reconnect (<reason>). Call exit() and initCycleTLS() again.` `InstanceManager.getOrCreateSharedInstance` no longer hands back such an instance: a new `initCycleTLS()` on the same port starts a new process. `removeSharedInstance` takes the instance as a second argument so an old client's `exit()` cannot remove the new instance.
- `cleanup()` and `exit()` set the shutdown flag before killing the child, so that exit is never treated as a crash. They also settle requests still in flight.

**Tests.** `tests/legacy-crash-recovery.test.js` uses the real binary and kills its process group with `SIGKILL`:

- two requests in flight (one with `timeout: -1`) reject with an `Error` naming the Go process, a later request succeeds, and exactly one restart happened;
- a request made while the process is away rejects in under a second with the "reconnecting" message, not after its 30 s timeout;
- a buffered body and a stream body that are mid-transfer both fail with an error;
- after `exit()` the pending request rejects and no new process is spawned.

`tests/legacy-reconnect.test.js` uses a fake child and a local `WebSocketServer` (the pattern from `tests/respawn-limit.test.js`):

- the socket closes while the process stays alive: the in-flight request rejects, the client reconnects, no respawn;
- the server goes away for good: requests reject with "reconnecting", then with "could not reconnect"; a new `initCycleTLS()` spawns again and survives the old client's `exit()`;
- a child that exits with no stderr output is restarted 3 times and no more.

All of these fail on the unchanged source (run). `tests/respawn-limit.test.js` still passes unchanged.

**What a user will notice.** Requests in flight when the process dies now reject within milliseconds with an `Error` instead of timing out. Requests during the restart window (about 0.6 s for a first crash) reject at once and should be retried. A stream response emits `error` on its stream; a stream with no `error` listener will throw, as any Node stream does. The log line `Fatal error detected ..., restarting in 500ms (attempt 1 of 3) -> Go process exited unexpectedly (signal SIGKILL)` appears for exits that used to be silent. `exit()` now rejects requests that were still in flight; before, they sat until their timeout.

## 2. Init rejections were strings

**What was wrong.** `SharedInstance.rejectInitialization(reason: string)` and the connection timeout in `createClient` rejected with plain strings. `initCycleTLS` wrapped them, so the public API already threw an `Error`; only callers of the internal `_InstanceManager` saw a string (read).

**What changed.** `rejectInitialization` accepts an `Error` or a string and always rejects with an `Error`. The timeout path rejects with `new Error(...)`. The `reject` parameters are typed `(reason: Error) => void`. `initCycleTLS` keeps its message (`Failed to initialize CycleTLS: <message>`) and sets the original error as `cause`.

**Tests.** `tests/instanceManager.test.ts` matched with `rejects.toMatch`, which only works on strings. It now asserts `rejects.toBeInstanceOf(Error)` and `rejects.toThrow(/Executable not found|not found/i)`. With the unchanged source those two assertions fail.

**What a user will notice.** Nothing through `initCycleTLS`. Code that used `_InstanceManager` directly and compared the rejection to a string must read `.message`.

## 3. Non-string header values poisoned the connection

**What was wrong.** Run, before the change (`poison.js`):

```
1_good:          resolved status=200 in 7ms
2_number_header: rejected "Request timeout after 4000ms" in 4003ms
3_good_again:    rejected "Request timeout after 4000ms" in 4002ms
```

The Go `Options.UnmarshalJSON` accepts a string or a string array per header. A number fails decoding, and the Go handler stops reading that socket without closing it, so every later request goes unanswered (read; the Go half is fixed by another change).

**What changed.** `normalizeRequestHeaders` runs at the start of every request and `ws()` call (from `applyDefaultOptions`). Strings pass through; numbers, booleans and bigints go through `String()`; arrays are converted element by element and stay arrays, so `string[]` multi-value headers work as before. `undefined`, `null`, objects, functions and nested arrays throw a `TypeError` that names the header, before anything is sent. The option type is now `RequestHeaders` (`string | number | boolean`, or an array of those); response headers keep the `HttpHeaders` type.

`sendRequest` also rejects a `body` that is not a string after the `URLSearchParams` and `FormData` conversions. An object body poisoned the connection the same way (issue 337).

**Tests.** `tests/legacy-headers.test.js`, real binary and a local HTTP server that records `rawHeaders`: number, float, zero, boolean and bigint values arrive as strings; `["one", 2, true]` arrives as three header lines; an object, `undefined`, `null`, an array holding an object and a nested array each reject with a `TypeError` in under a second, nothing reaches the server, and the next request works; an object body rejects and the next request works. All fail on the unchanged source.

**What a user will notice.** `headers: { 'x-int': 1 }` works. A bad value fails that one request with a clear message instead of freezing the client.

## 4. Timeouts

**What was wrong.** `timeout: -1` rejected in 1 ms with `Request timeout after -1000ms` (run, `poison.js`), because the timer was set to `options.timeout * 1000`. The Go side treats a negative timeout as "no timeout" (`cycletls/request_timeout.go`).

The client timer is cleared when the headers arrive, and Go bounds only the wait for headers. I confirmed by running that a stalled body is unbounded: against a local server that sends headers and one chunk and then nothing, a request with `timeout: 1` was still pending after 3 s. That check is kept as a test.

**What changed.**

- A negative `timeout` sets no client timer. `0` or unset still means 30 s.
- New opt-in `readTimeout` in milliseconds. It starts when the headers arrive and restarts on every body chunk, so it measures idle time. When it fires the request fails with `Read timeout: no response data for <n>ms` (a buffered request rejects, a stream response emits `error`) and the request is cancelled in the Go process. It is not sent to Go. `timeout` on this client is in seconds and `readTimeout` in milliseconds; the name and unit match the default client's `readTimeout`.

**Tests.** `tests/legacy-timeout.test.js`, real binary and a local server: `timeout: -1` resolves; `timeout: -1` waits for headers that `timeout: 1` gives up on; a stalled body without `readTimeout` stays pending; `readTimeout: 400` rejects a buffered request and errors a stream after delivering the first chunk; a body that drips for longer than `readTimeout` in total, but with no long gap, completes. Five of the seven fail on the unchanged source; the two that pass there are the stalled-body check and a plain request.

**What a user will notice.** `timeout: -1` works. Nothing else changes unless `readTimeout` is set.

## 5. Types and docs for the 3.0.1 options

**What changed.** Added to `CycleTLSRequestOptions` in `src/index.ts`, with doc comments: `tlsProfile`, `clientHello`, `shuffleExtensions`, `sessionResumption`, `caCerts`, `certPins`, `http2Priority` (with an exported `HTTP2Priority` interface), `proxyHeaders`, plus `tls13AutoRetry`, `bodyBytes` and `readTimeout`, which the default client had and this one did not. The comments on `http2Fingerprint`, `quicFingerprint`, `forceHTTP3` and `timeout` were updated.

`certPins` did not exist in either TypeScript type before; it existed only in the Go struct.

The legacy client spreads the options into the JSON it sends, so the new fields reach Go unchanged. Two exceptions, both deliberate: `bodyBytes` (a `Uint8Array`) is turned into the base64 string Go expects for a `[]byte`, and `readTimeout` is removed. When `tlsProfile` or `clientHello` is set the client no longer adds its default JA3; before, a default JA3 was added whenever no fingerprint option was present.

The same eight new fields were added to `RequestOptions` in `src/flow-control-client.ts`, in one marked block at the end of the interface. That file belongs to another change. I added types only: the default client builds its request field by field, and forwarding the new fields is not done here.

README has a new section "New in 3.0.1" before "Usage", with the option table, the precedence order and the behaviour changes, including `proxy` with `forceHTTP3` now returning an error and `enableConnectionReuse` left out now meaning true. The README passages that described `quicFingerprint` as working now say it has no effect. The option semantics in the README and the doc comments are the ones reported by the Go changes; I did not run them.

**Tests.** `tests/legacy-options-types.test.ts` assigns every new option on both types and is type-checked by ts-jest; on the unchanged source it fails to compile with TS2353 on `tlsProfile`, `http2Priority` and `bodyBytes`. In `tests/legacy-reconnect.test.js` a fake Go server records the JSON it receives and asserts that each new option arrives unchanged, that `http2Priority: false` arrives as `false`, that `bodyBytes` arrives as base64 and that `readTimeout` is not sent. `tests/legacy-headers.test.js` checks that `bodyBytes` reaches a local server as the exact bytes.

## 6. The Jest setup

Job: `npm test -- --testPathIgnorePatterns=tlsfingerprint --forceExit` (the job that is `if: false` in `.github/workflows/test_npm.yml`).

**Before** (run): 7 suites failed, 25 passed, 13 skipped; 10 tests failed, 215 passed, 67 skipped.

| Suite | Failure | Cause |
| --- | --- | --- |
| `images`, `multipartFormData`, `forceHTTP1`, `multipleImports`, `urlencoded` | `withUpstreamRetry is not a function` | `tests/test-utils.js` shadowed `tests/test-utils.ts` |
| `insecureSkipVerify` | TS2348, value of type `typeof CycleTLS` is not callable | the default export is the `CycleTLS` class since 3.0.0 |
| `async-close-race` | `Server failed to start` in 2 of 4 tests | the tests passed `port: 0` |

**`test-utils.js`.** `jest.config.js` maps `./test-utils.js` to `./test-utils`, and Jest tries `.js` before `.ts`, so every import got the `.js` file. It was not simply stale. Git history shows the two files drifting apart: `withUpstreamRetry` was added to the `.ts` only (commit `0faa370`), and `probeUpstream`, `makeConditionalTest` and a wider `UPSTREAM_FLAKE_STATUSES` (with 429 and the Cloudflare 52x codes) were added to the `.js` only (commit `29f3ea5`). `tests/streaming.test.js` needs the second set. I moved those three into `tests/test-utils.ts` with types and removed `tests/test-utils.js`. Nothing loads the `.js` on purpose: every `require("./test-utils.js")` goes through the same mapper. One difference remains and is harmless: `createSuiteInstance` and `createSafeCycleTLS` were `async` in the `.js` and are not in the `.ts`; every caller uses `await`, which works for both.

`withUpstreamRetry` now also retries on 429, since it reads the shared status set. It still returns the last response, so a test that asserts 200 still fails if every attempt is throttled.

**`insecureSkipVerify.test.ts`.** Now imports the named `initCycleTLS`. Nothing else changed in it.

**`async-close-race.test.ts`.** This was not a startup race. The tests construct `new CycleTLS({ port: 0 })`. `flow-control-client.ts` keeps the 0 (`options.port ?? 9119`), starts the Go process with `WS_PORT=0`, and Go listens on `:0`, a random port. The client then dials `ws://localhost:0` fifty times and throws `Server failed to start`. The two tests that passed never made a successful request. Each failed attempt also leaves a Go process listening on a random port. I gave each test its own port (9761 to 9764) and pointed the requests at a local HTTP server with the same `/delay/2` and `/get` paths instead of httpbin.org. The assertions are unchanged. `tests/multipleRequests.test.js` and `tests/binary-data-handling.test.js` also pass `port: 0`; they are skipped today and will hit the same problem when enabled. Whether the default client should reject port 0 or pick a free port is for the owner of `flow-control-client.ts`.

**After** (run): 0 suites failed, 37 passed, 13 skipped; 0 tests failed, 255 passed, 67 skipped. Exit code 0. The count rose by five suites and 30 tests: the new `legacy-*` suites. The 13 skipped suites are the ones marked `describe.skip` with the "v3 spawn race" comment; I did not touch them.

No test was skipped at run time for an upstream problem (no "Skipped:" line in the log), and I saw no httpbin 429 and no port collision in this job: the suites that run in it use distinct ports.

One thing to know when changing `src/index.ts`: `tests/ts-core-review-fixes.test.ts` and `tests/zombie-process.test.ts` assert on the text of the source and of `dist/index.js` (for example that the method is spelled `rejectInitialization(reason)` and that its body has no `initialize()` in it). Two earlier drafts of this change failed those tests for that reason only, so the names in those spots are kept.

**Can the workflow's `if: false` be flipped?** On this machine the job is green, so the reasons given in the workflow comment (the TypeScript compile break and "Server failed to start") are gone for the suites that run. I would still not make it a required check yet:

- about a dozen suites that run call httpbin.org, tls.peet.ws or badssl.com, so a 429 or an outage can fail the job. No failure of that kind showed up in my runs;
- the job is `continue-on-error: true`, so flipping it cannot block a PR either way, which makes flipping it low risk;
- I ran macOS only. The workflow matrix also has Ubuntu and Windows. `legacy-crash-recovery` kills a process group on Unix and calls `child.kill()` on Windows; the Windows path is not run.

I did not change the workflow.

## Left out, and why

- **Signal handling (`SIGINT`/`SIGTERM` swallowed by the import-time listener in `src/index.ts`).** The author of the default-client change asked for the legacy handler to re-send the signal after cleanup. That is gap G21, marked "needs care", and was not in this brief. Not done.
- **Forwarding the new options in the default client.** Types only, see section 5.
- **Waiting for the reconnect inside a request.** A request made while the Go process is restarting fails at once. Making it wait would hide the crash from the caller but needs its own bound against the request timeout. The error says to retry.
- **`Buffer` or `FormData` with binary parts as `body`.** `FormData` is still read as UTF-8 text. Use `bodyBytes` for binary data.
- **Other options with the wrong type** (for example `timeout: "5"` or `timeout: 0.5`, which Go cannot decode into an int). These still depend on the Go-side fix that answers a decode error with an error frame.
- **The 13 skipped suites and the two other `port: 0` suites.** They belong to the default client.
- **Windows and Linux runs.** Not available here.

## Checks

All run on macOS arm64 after the last source change.

| Check | Exit code | Result |
| --- | --- | --- |
| `npm ci --unsafe-perm` | 0 | `src/go.mod` unchanged |
| `npx tsc` (the repo's `npm run build`) | 0 | no errors |
| `npx jest tests/legacy- --forceExit` | 0 | 5 suites, 26 tests passed (3 runs in a row) |
| `npm test -- --testPathIgnorePatterns=tlsfingerprint --forceExit` | 0 | 37 suites passed, 13 skipped |
| `npx jest tests/respawn-limit.test.js tests/decompression.test.ts tests/instanceManager.test.ts --forceExit` | 0 | 3 suites, 16 tests passed |
| `npm test -- --testPathPattern=tlsfingerprint --forceExit` | 1 | 4 suites failed, 34 tests, all with `CycleTLSError: Connection closed` |
| the same with `--runInBand` | 0 | 4 suites, 43 tests passed, 2 skipped |
| `npm publish --dry-run` | 0 | `src/go.mod` unchanged |

The fingerprint job fails as written for a reason outside this change. All four suites under `tests/tlsfingerprint/` create the default client on port 9119. Jest runs them in parallel workers, they end up sharing one Go process, and when the first suite finishes its `close()` kills that process under the others. Run one at a time they all pass. The compiled `dist/flow-control-client.js` is byte-for-byte the same with and without this change (the edit there adds interface fields only), so this change cannot be the cause. The fix is a distinct port per suite, in files this change does not own.
