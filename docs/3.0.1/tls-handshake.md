# 3.0.1: TLS handshake options

Five changes to the TCP TLS handshake in `cycletls/roundtripper.go` (`dialTLS` and its two retry dials). The new code is in `cycletls/tls_handshake.go`, `cycletls/cert_pinning.go` and `cycletls/session_cache.go`; the tests are in `cycletls/tls_handshake_test.go`. utls stays at v1.8.2.

| Option (JSON) | Go field (`Options` and `Browser`) | Type | Default |
|---|---|---|---|
| `sessionResumption` | `SessionResumption` | `bool` | `false` |
| `certPins` | `CertPins` | `[]string` | none |
| `caCerts` | `CACerts` | `[]string` | none |

All tests run against local servers (`httptest`, `tls.Listen`) with a CA generated in the test. None uses the network.

## 1. TLS session resumption (`sessionResumption`)

**What was missing.** The three `utls.Config` literals in `roundtripper.go` (first dial and both retries) set `ServerName`, `OmitEmptyPsk` and `InsecureSkipVerify` and nothing else. No `ClientSessionCache` existed anywhere in non-test code, so every connection was a full handshake. A JA3 that lists extension 41 (`pre_shared_key`) never produced it: with no session to offer, utls drops the extension.

**What changed.** With `sessionResumption: true`:

- `handshakeConfig` (`tls_handshake.go`) sets `ClientSessionCache` and `PreferSkipResumptionOnNilExtension: true`. Without the second flag utls panics on a spec that has neither a session-ticket nor a pre-shared-key extension.
- `finishHandshakeSpec` calls `ensurePreSharedKeyLast`, which appends `&utls.UtlsPreSharedKeyExtension{}` as the last extension when the spec has none, and moves an existing one to the end. utls keeps it off the wire while there is no session (`OmitEmptyPsk`), so the first hello to a host is byte-for-byte what it was. A resumed TLS 1.3 hello is the same list plus 41 at the end.
- A spec that offers TLS 1.2 only (a `771,` JA3 without `tls13AutoRetry`) resumes with its session ticket, extension 35. Extension 41 never appears there.

It is off by default because a resumed hello carries extension 41, so its JA3 and JA4 differ from the first hello's. Real browsers behave the same way.

**Where the cache lives.** With connection reuse off, `createNewClient` builds a new `roundTripper` for every request. A cache on the roundTripper would always be empty, and the client pool is skipped in that case too. So the caches sit in a package-level registry (`sessionCaches` in `session_cache.go`), and `createNewClient` hands the right one to the roundTripper through the unexported `Browser.sessionCache`.

The registry key (`sessionCacheKey`) is `generateClientKey` with cookies, header order, HTTP/2 fingerprint, timeout and redirect policy blanked. What is left decides whether two requests may share a session: JA3/JA4R/QUIC fingerprint, user agent, SNI, proxy, `insecureSkipVerify`, `forceHTTP1`, `certPins`, `caCerts`. A session is never offered through a different proxy or under a different fingerprint. A field added to `generateClientKey` later is covered without further work.

The registry holds at most 100 caches (least recently used goes first) and each cache holds 64 server names. A roundTripper built directly (`NewTransport` and the other constructors) gets its own cache, which lives as long as it does.

**Usage.**

```js
await client.get(url, { sessionResumption: true });
```

```go
client.Do(url, cycletls.Options{SessionResumption: true}, "GET")
```

**Tests.** `TestSessionResumption` runs two requests with reuse off against a server that reports `DidResume`, for the default Chrome hello on HTTP/2 and HTTP/1.1, a JA3 upgraded to TLS 1.3, a TLS 1.2 JA3, and the Firefox default. It checks that the second request resumed, that the second ClientHello has 41 as its last extension and is otherwise the first hello, and that with the option off neither hello has 41 and nothing resumes. `TestSessionResumptionKeepsFirstHello` compares the first hello with the option on and off. `TestSessionResumptionPerRoundTripper` covers the cache of a directly built roundTripper. `TestSessionResumptionConcurrent` runs 32 parallel requests on one cache under `-race`. `TestSessionCacheKey` checks which fields split the cache and that the registry stays bounded. `TestDoCarriesTLSHandshakeOptions` goes through `CycleTLS.Do`.

**Left out.**

- 0-RTT. utls v1.8.2 only sets early data on its QUIC path.
- Single-use tickets. Chrome uses a TLS 1.3 ticket once; this cache, like Go's, offers the newest ticket until the server sends another.
- HTTP/3 and WebSocket. Neither goes through `dialTLS`.
- The cache key inside one cache is the SNI, as in utls and Go. Two servers reached under the same SNI on different ports share an entry; the server that did not issue the ticket ignores it and does a full handshake.
- Not run against a live site in this change. The research prototype did that with the same three settings.

## 2. Certificate pinning (`certPins`)

**What was missing.** `Options.CertPins` was declared in `types.go` and read nowhere. A caller who set it got no check at all.

**What changed.** After every TLS handshake, `verifyHandshake` compares the certificates the server presented with the pins. If none matches, the connection is closed and the request fails with an error that wraps `ErrCertPinMismatch` and names the leaf's two pin values.

The field keeps its type (`[]string`). An entry has one of two forms, and the form says what was hashed:

| Form | Matches |
|---|---|
| 64 hex characters | SHA-256 of the whole certificate (DER). This is the meaning the field's comment and `CertPinConfig` already documented. |
| `sha256/<base64>` | SHA-256 of the certificate's SubjectPublicKeyInfo, the HPKP `pin-sha256` value. |

A hex pin is never compared with a key hash and a `sha256/` pin is never compared with a certificate hash. Hex entries are checked with the existing `CertPinConfig.Validate`. Any other entry, bare base64 included, fails the request before anything is dialed, with an error that names the entry.

Rules:

- Any certificate in the chain the server sent may match, so an intermediate can be pinned.
- The list applies to every TLS connection of the request, redirect hops to other hosts included. There is no per-host syntax.
- Pins are checked with `insecureSkipVerify` on as well. That flag turns off chain verification only.
- On a resumed session the pins are checked against the chain stored with the session.
- The check runs in `dialTLS` and in both retry dials before the connection is cached, so no HTTP/1.1 request and no HTTP/2 preface is written to a server that failed it. HTTP/2 connections are opened through the same function.
- With a proxy the pins apply to the target's certificate, not the proxy's.

Three paths cannot check pins, and each refuses the request (`checkTLSHandshakeOptions`, called at the top of `RoundTrip`, and `dispatchWebSocketRequest`):

- HTTP/3. The QUIC handshake is done by quic-go with its own TLS config. A `VerifyConnection` hook there could enforce the pins; that code belongs to the HTTP/3 work, so for now `certPins` with `forceHTTP3` returns an error.
- WebSocket. gorilla/websocket dials with `crypto/tls`.
- Plain `http://`, including a redirect from https to http. There is no certificate to check.

**Usage.**

```js
await client.get(url, {
  certPins: [
    "sha256/47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=", // SPKI
    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // whole certificate
  ],
});
```

To get the values for a host:

```sh
# SPKI pin
openssl s_client -connect example.com:443 </dev/null 2>/dev/null \
  | openssl x509 -pubkey -noout | openssl pkey -pubin -outform der \
  | openssl dgst -sha256 -binary | base64
# certificate pin
openssl s_client -connect example.com:443 </dev/null 2>/dev/null \
  | openssl x509 -outform der | openssl dgst -sha256 -hex
```

**Tests.** `TestParseCertPin` covers the accepted and rejected forms. `TestCertPinFormsDoNotCross` checks that each form only matches its own hash. `TestCertPins` runs on HTTP/1.1 and HTTP/2: leaf and issuer pins in both forms pass; a wrong pin fails with `ErrCertPinMismatch` and the handler is never called; the same with `insecureSkipVerify`; and a resumed session is still checked. `TestCertPinMismatchSendsNoRequestBytes` uses a raw `tls.Listen` server that counts application bytes after the handshake: zero on both protocols. `TestCertPinsOnRetryDial` and `TestCustomSNIOnCurveRetry` cover both retry dials. `TestCertPinsRejectedWhereTheyCannotBeChecked` covers an invalid pin (the server sees no ClientHello), HTTP/3, WebSocket and plain http.

**Left out.** Enforcement on HTTP/3 and WebSocket (refused instead). Per-host pin lists. Backup-pin or expiry policy. `Browser.WebSocketConnect`, the Go-only WebSocket helper, still ignores pins.

## 3. Custom root CAs (`caCerts`)

**What was missing.** The handshake config had no `RootCAs`, so only the system roots were trusted. A server signed by a private CA could only be reached with `insecureSkipVerify`.

**What changed.** `caCerts` is a list of PEM strings; one string may hold several certificates. `buildRootCAs` starts from the system pool and adds them, and `handshakeConfig` sets the result as `RootCAs` on the first dial and both retries. The system roots stay trusted.

Invalid input fails the request before anything is dialed, with an error that names the entry (`invalid caCerts[1]: ...`). `appendPEMCerts` is stricter than Go's `AppendCertsFromPEM`, which skips what it cannot read: text that is not PEM, a block that is not a `CERTIFICATE`, a certificate that does not parse, trailing text and an empty string are all errors.

The pool is also copied into the roundTripper's `TLSConfig` when the caller did not set roots there, because the HTTP/3 dials build their TLS config from that field. This part is inferred from reading `ConvertUtlsConfig` and `http3.go`; no HTTP/3 test runs it.

**Usage.**

```js
await client.get(url, { caCerts: [fs.readFileSync("corp-root.pem", "utf8")] });
```

**Tests.** `TestCACerts`, on HTTP/1.1 and HTTP/2: without the option the request fails verification and the handler is never called; another private CA does not help; with the right CA the request succeeds. `invalid_pem` covers six kinds of bad input and checks the server saw no ClientHello. `system_roots_kept` checks that the pool is more than the custom CA.

**Left out.** Client certificates (mTLS). A switch to trust only the given CAs. WebSocket does not use `caCerts`; it fails closed, as an untrusted certificate.

## 4. Custom SNI dropped on retry

**What was missing.** `dialTLS` works out the SNI (`serverName`, the custom name when set) but passed `host` to `retryWithTLS13CompatibleCurves` and `retryWithOriginalTLS12JA3`, and both put it in `ServerName`. A request with `serverName` set sent the custom name on the first dial and the URL's host on the retry.

**What changed.** Both calls pass `serverName`, the parameter is renamed to match, and all three dials build their config with `handshakeConfig(serverName)`.

The curve retry's spec rebuild now sits in one function, `tls13RetrySpec` (`roundtripper.go`). It rebuilds from the QUIC fingerprint, the JA3, or the default Chrome JA3, as before. A new fingerprint source in the spec selection needs a case there, or the retry sends a different hello than the first dial.

**Tests.** `TestCustomSNIOnTLS12FallbackRetry`: a `771,` JA3 with `tls13AutoRetry` goes out upgraded to TLS 1.3; the server refuses the first handshake, which triggers the TLS 1.2 fallback dial. Its `GetConfigForClient` must see the custom name twice. Before the fix it saw `["custom.test", ""]`. `TestCustomSNIOnCurveRetry` covers the other retry. No JA3 or JA4_r string reaches the unsupported-curve error at utls v1.8.2, so the test's dialer makes the first connection fail with that error text at its first write; the retry then runs as in production.

## 5. Firefox default path ignored `forceHTTP1`

**What was missing.** With a Firefox user agent and no fingerprint, `dialTLS` sets `helloID = utls.HelloFirefox_Auto` and no spec. The parrot's ALPN list (`h2`, `http/1.1`) went out unchanged, so `forceHTTP1` still negotiated HTTP/2.

**What changed.** Nothing in the spec-selection lines. `finishHandshakeSpec`, which runs after them, expands a built-in parrot into its spec with `utls.UTLSIdToSpec` when `forceHTTP1` (or `sessionResumption`) is on, and `forceHTTP1ALPN` sets ALPN to `http/1.1` and drops ALPS, the same rule as `ModernChromeSpec(true)`. utls applies a built-in parrot the same way, so the hello is otherwise identical. When neither option is on, the parrot is used untouched.

**Tests.** `TestFirefoxDefaultHonoursForceHTTP1`: with `forceHTTP1` the server sees ALPN `[http/1.1]` and serves HTTP/1.1; without it the hello still offers `[h2 http/1.1]` and negotiates HTTP/2.

## Found on the way, not fixed

- A JA3 whose version token is not 769 to 772 (for example `773,...`), or is 769, panics inside utls `ApplyPreset`: `StringToSpec` ignores the error from `createTlsVersion` and puts a nil extension in the list (`utils.go`). Seen while probing for the curve retry.
- `go vet ./...` reports six "cancel function is not used on all paths" findings in `index.go` and `ws_handler_v2.go`. They predate this change.
- `dialTLS` does not close the raw connection when building the spec fails. The paths added here close it; the earlier returns in the spec-selection lines do not.
- `CycleTLS.Do` builds its `Browser` by hand and leaves out `ServerName`, `DisableGrease` and `TLS13AutoRetry`.
