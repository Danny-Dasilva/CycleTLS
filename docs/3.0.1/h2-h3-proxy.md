# 3.0.1: HTTP/2, HTTP/3 and proxy fixes

Six changes to the Go side of CycleTLS. Each section says what was wrong, what changed, how it is tested and what was left out. Paths are relative to `cycletls/`.

New options:

| JSON | Go (`Options` and `Browser`) | Type |
|---|---|---|
| `http2Priority` | `HTTP2Priority` | `*HTTP2PriorityOptions` |
| `proxyHeaders` | `ProxyHeaders` | `map[string]string` |

New exported names: `HTTP2PriorityOptions`, `ErrHTTP3ProxyUnsupported`, `HTTP2Fingerprint.PriorityFrames`, `HTTP2Fingerprint.NoPriorityFrames`, `HTTP2Fingerprint.PseudoHeaderOrder()`.

Every "failed before" below was run against the unchanged `version/3.0.1` code, not read off the source.

## 1. Akamai-format `http2Fingerprint`

### What was wrong

`NewHTTP2Fingerprint` read the third part of the string with `Sscanf("%d")` as a 0/1 flag and stored the fourth part without using it. The README's own example,

```
1:65536;4:131072;5:16384|12517377|3:0:0:201,5:0:0:101,7:0:0:1,9:0:7:1,11:0:3:1,13:0:0:241|m,p,a,s
```

parsed as "exclusive = true" (the leading `3`) and no PRIORITY frames. With a Chrome User-Agent the wire showed zero PRIORITY frames, first stream 1, pseudo-header order `m,a,s,p` and an exclusive HEADERS priority. The pseudo-header order always came from the User-Agent (`parseUserAgent` in `utils.go`), so a Safari string with a Chrome User-Agent still sent Chrome's order.

There was a second trap. With part 3 `0` and a Firefox User-Agent, fhttp wrote Firefox's six legacy PRIORITY frames anyway and opened stream 15, because `AutoWriteFrames` falls back to the browser preset when `PriorityFrames` is nil. Current Firefox sends no PRIORITY frames.

### What changed

`http2.go`: `NewHTTP2Fingerprint`, `parsePriorityEntry`, `pseudoHeaderOrder`, `HTTP2Fingerprint.PseudoHeaderOrder`, `String`, `Apply`, and three `roundTripper` methods: `parsedHTTP2Fingerprint`, `configureHTTP2Transport`, `applyPseudoHeaderOrder`.

Accepted grammar:

```
fingerprint = settings "|" window "|" priority "|" pseudo
settings    = id ":" value *( ("," / ";") id ":" value )   ; id 0-65535, value 0-4294967295
window      = 0-4294967295                                  ; connection WINDOW_UPDATE increment
priority    = "0"                                           ; no PRIORITY frames
            / "1"                                           ; 3.0.0 meaning, see below
            / entry *( "," entry )
entry       = streamID ":" exclusive ":" dependsOn ":" weight
              ; streamID 1 to 2^31-1, exclusive 0 or 1, dependsOn 0 to 2^31-1, weight 1-256
pseudo      = ""                                            ; keep the User-Agent's order
            / a permutation of m,a,s,p                      ; :method :authority :scheme :path
```

- **Settings** take `,` or `;`, also mixed. tls-client and tls.peet.ws print `;`, azuretls prints `,`. Any setting ID is sent, known or not.
- **Priority entries** fill `HTTP2Settings.PriorityFrames` in fhttp. Weight in the string is the RFC 7540 weight (1-256); the wire byte is weight minus one, the same convention azuretls and tls.peet.ws use. The first request goes out on the stream after the last entry (13 gives 15).
- **`0`** means no PRIORITY frames, for every User-Agent. For a Firefox User-Agent `Apply` sets the transport's `Navigator` to Chrome. That is the only way to make this fhttp version write no PRIORITY frames and open stream 1; an empty frame list would open stream 0. SETTINGS and WINDOW_UPDATE still come from the string, so nothing else on the wire changes. One side effect: fhttp then caps response headers at 262144 bytes, as it does for Chrome, where the Firefox preset had no cap of its own.
- **`1`** (or any other bare non-zero number) keeps its 3.0.0 meaning: exclusive HEADERS priority, and the User-Agent's own PRIORITY frames (none for Chrome, six for Firefox).
- **Pseudo-header order** from part 4 replaces the User-Agent's guess. `roundTripper.RoundTrip` sets fhttp's `PHeader-Order:` key from the fingerprint just before sending, so it applies to every caller (`Do`, the v1 dispatcher, the v2 handler, `NewTransportWithHTTP2Fingerprint`) without touching `utils.go`.
- `String()` writes the frame list back and keeps the settings separator it was parsed with.

```js
// Node
await cycleTLS.get(url, {
  userAgent: firefoxUA,
  http2Fingerprint: '1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s',
});
```

```go
// Go
client.Do(url, cycletls.Options{
    UserAgent:        chromeUA,
    HTTP2Fingerprint: "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p",
}, "GET")
```

### Behaviour changes to know about

- A string with a frame list no longer sets the exclusive bit on HEADERS (it did only because `Sscanf` read the first stream ID as a flag). Use `http2Priority` to set it.
- Part 3 `0` with a Firefox User-Agent no longer sends six PRIORITY frames.
- Part 4 now has an effect, and a non-empty part 4 that is not a permutation of `m,a,s,p` is an error. An order that left one out would make fhttp drop that pseudo-header and send a broken request.
- Settings and window values are parsed strictly. `1:65536abc` was accepted by `Sscanf` and is now an error.

### Tests

`http2_wire_test.go` starts a local TLS listener that speaks HTTP/2 with a raw `golang.org/x/net/http2` framer and records SETTINGS, the connection WINDOW_UPDATE, PRIORITY frames, the HEADERS stream ID and priority, and the pseudo-header order. Requests go through `Do`.

- `TestHTTP2FingerprintREADMEExampleOnWire` (Chrome and Firefox User-Agent): SETTINGS `1:65536,4:131072,5:16384`, window 12517377, six PRIORITY frames with wire weights 200, 100, 0, 0, 0, 240, first stream 15, order `m,p,a,s`. Both subtests failed before.
- `TestHTTP2FingerprintChromeStyleOnWire`: `1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p` gives those four settings, window 15663105, no PRIORITY frames, stream 1, order `m,a,s,p`. The Firefox subtest failed before; the Chrome subtest passed and guards against regressions.
- `TestHTTP2FingerprintPseudoHeaderOrderOnWire`: a Safari-style string with a Chrome User-Agent sends `m,s,a,p`. Failed before.
- `TestHTTP2FingerprintLegacyExclusiveFlagOnWire` and `TestHTTP2DefaultsUnchangedOnWire` passed before and after: bare `1` and requests without a fingerprint send the same frames as 3.0.0.
- `http2_priority_test.go`: `TestNewHTTP2FingerprintParsesAkamaiFormat`, `...LegacyFlagAndSeparators`, `...RejectsBadInput`, `TestHTTP2FingerprintApplyFirefoxNoPriorityFrames`. New API or new errors, so they could not pass before.

### Left out

- New default fingerprints. Without `http2Fingerprint`, fhttp's presets still decide the bytes, and they are older than current Chrome and Firefox. Changing defaults for every HTTP/2 user is not a patch-release change.
- An `InitialStreamID` field in fhttp would replace the Navigator switch. The fork cannot be changed from this repository.
- The expected values for real browsers come from tls-client's profiles. They were not checked against a browser capture.

## 2. `http2Priority`

### What was wrong

With `http2Fingerprint` set, the HEADERS frame always carried weight 256 (wire 255), dependency 0, and exclusive only for a bare `1`. Without it, HEADERS carried no priority at all. Neither could be changed, so Chrome (exclusive, weight 256), Firefox (weight 42) and Safari (no priority) could not all be matched.

### What changed

`http2.go`: type `HTTP2PriorityOptions` with `UnmarshalJSON`, `MarshalJSON`, `priorityParam`, `key`; function `applyHTTP2Priority`, called from `roundTripper.configureHTTP2Transport`.

```go
type HTTP2PriorityOptions struct {
    Weight    int    `json:"weight"`    // RFC 7540 weight 1-256; 0 means 256. Wire byte is Weight-1.
    Exclusive bool   `json:"exclusive"`
    StreamDep uint32 `json:"streamDep"` // 0 means none
    Disabled  bool   `json:"-"`         // JSON false sets it
}
```

| `http2Priority` | HEADERS frame |
|---|---|
| absent or `null` | as 3.0.0: weight 256 with a fingerprint, no priority without one |
| `{"weight":42,"exclusive":false,"streamDep":0}` | those values |
| `false` | no priority fields |
| `true` | read as `{}`: weight 256, not exclusive |

Weight uses the same scale as the fingerprint's priority entries: the number a browser trace shows, one more than the wire byte. A weight outside 1-256 is refused when the JSON is decoded, and from Go when the HTTP/2 transport is built.

The option works without `http2Fingerprint`. In that case only the HEADERS priority changes; SETTINGS, WINDOW_UPDATE and the browser's PRIORITY frames stay as the preset sends them.

```js
// Chrome
{ http2Fingerprint: '1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p',
  http2Priority: { weight: 256, exclusive: true, streamDep: 0 } }
// Safari
{ http2Fingerprint: '2:0;3:100;4:2097152;9:1|10420225|0|m,s,a,p', http2Priority: false }
```

```go
cycletls.Options{HTTP2Priority: &cycletls.HTTP2PriorityOptions{Weight: 42}}
cycletls.Options{HTTP2Priority: &cycletls.HTTP2PriorityOptions{Disabled: true}}
```

### Tests

`http2_priority_test.go`, against the same local HTTP/2 server:

- `TestHTTP2PriorityOptionOnWire`: eight cases. Exclusive 256, weight 42, zero weight, a stream dependency on top of the README frame list, `false` with a fingerprint, the option without a fingerprint for Chrome and Firefox, and `false` without a fingerprint. Each asserts the HEADERS priority, the PRIORITY frames and the stream ID.
- `TestHTTP2PriorityInvalidWeightFailsRequest`, `TestHTTP2PriorityJSON` (object, `false`, `null`, absent, bad values, marshalling), `TestClientKeyCoversHTTP2PriorityAndProxyHeaders`.

These use the new field, so they did not compile before.

### Left out

The Node types. Another change adds `http2Priority` and `proxyHeaders` to the TypeScript options; the JSON shape above is what the Go side reads.

## 3. HTTP/3 ignored the proxy

### What was wrong

`proxy` together with `forceHTTP3` or `protocol: "http3"` sent the request straight from the machine's own address. The HTTP/3 branch of `roundTripper.RoundTrip` never used `rt.dialer`, and `http3Dial` opened a plain UDP socket. Run against a local UDP listener with a proxy configured, the unchanged code delivered 16 QUIC packets to the target.

### What changed

The request now fails with `ErrHTTP3ProxyUnsupported` before a socket is opened. There is no fallback to a direct connection and none to TCP.

- `http3.go`: `ErrHTTP3ProxyUnsupported`, `roundTripper.usesProxy`.
- `roundtripper.go`, `RoundTrip`: the first thing the HTTP/3 branch does is return the error when the dialer is not `proxy.Direct`. This covers every caller, including the v2 handler and a custom dialer passed to `NewTransportWithProxy`.
- `index.go`, `dispatchHTTP3Request`: a check at the top returns the error through `fullRequest.err`, which the dispatcher turns into an error frame, before a client is built.
- `index.go`, `Do`: returns the error as a Go `error`. Without this, `Do` would wrap it into a `Response` with a status code like other transport failures.

The message is `cycletls: HTTP/3 cannot be sent through a proxy; remove the proxy or drop forceHTTP3/protocol "http3"`. Go callers can test for it with `errors.Is`.

`Do` only treats `ForceHTTP3` as HTTP/3; it never read `Protocol`. A `Do` call with `Protocol: "http3"` and a proxy goes over TCP through the proxy, as before, so nothing leaks there.

### Tests

`http3_fixes_test.go`, `TestHTTP3WithProxyReturnsErrorAndSendsNoPackets`: a local UDP listener counts datagrams while the request is made three ways (`processRequest` with `forceHTTP3` and with `protocol: "http3"`, `Do`, and a `roundTripper` with a proxy dialer). Each must return `ErrHTTP3ProxyUnsupported`, and the listener must have seen zero packets. All four parts failed before.

### Left out

HTTP/3 through a proxy. It needs SOCKS5 UDP ASSOCIATE or CONNECT-UDP (MASQUE), a new client and a `net.PacketConn` wrapper, and few proxies offer either.

## 4. HTTP/3 small fixes

### What was wrong

- `headerOrder` with HTTP/3 failed every request. `RoundTrip` puts fhttp's `Header-Order:` key in the header map; quic-go checks header names and answered `http3: invalid http header name "Header-Order:"`. Confirmed by running, not only by reading.
- `insecureSkipVerify` was ignored on HTTP/3. A self-signed server gave status 495.

### What changed

- `http3.go`, `http3RequestHeader`: copies the headers for quic-go without `Header-Order:` and `PHeader-Order:`. `makeHTTP3RequestWithTransport` in `roundtripper.go` uses it. The caller's header map is not modified.
- `InsecureSkipVerify` is set on the TLS config in `ghttp3Dial` and `uhttp3Dial` (`http3.go`) and in `makeHTTP3Request` (`roundtripper.go`), which builds the config for later dials on the same transport.

`headerOrder` does not order headers on HTTP/3. quic-go writes the header map in Go's map order and has no option for it. The fix makes the request succeed; it does not add ordering.

### Tests

`http3_fixes_test.go`, against a local quic-go `http3.Server` with a self-signed certificate:

- `TestHTTP3InsecureSkipVerifyOption`: 200 with the option, refused without it. Failed before (495).
- `TestHTTP3HeaderOrderDoesNotBreakRequest`: sets `Browser.TLSConfig` to skip verification, which already worked, so that before the fix it failed for the header reason alone. Asserts the server saw the request headers and no ordering key.
- `TestHTTP3HeaderOrderThroughDo`: the same through `Do` with both options. Failed before.

### Left out

Header order, pseudo-header order and SETTINGS order on HTTP/3. They need a forked or replaced HTTP/3 stack.

## 5. `quicFingerprint`

### What was wrong

The option did nothing for HTTP/3 and something wrong for TCP. `dialTLS` was its only reader: when set, it built the TCP ClientHello from `QUICStringToSpec`, which ignores the string beyond a length check and offers ALPN `h3`. A local TLS listener saw `[h3 h3-29 h3-28 h3-27]` from a plain HTTPS request, which no TCP server speaks.

On the HTTP/3 side nothing fills `Browser.USpec` from the option, and even with `USpec` set from Go the uquic connection is closed and quic-go dials again without it.

### What changed

`quicFingerprint` is now a no-op, and says so.

- `roundtripper.go`: removed the `QUICFingerprint` branch from `dialTLS` and the matching case from `retryWithTLS13CompatibleCurves`. TCP requests pick their ClientHello from `ja3`, `ja4r` or the User-Agent as if the option were absent.
- `http3.go`, `logQUICFingerprintNoOp`: called from `newRoundTripper`, writes one debug-log line when the option is set: `quicFingerprint is set but has no effect: HTTP/3 requests use quic-go's own QUIC Initial, and TCP requests ignore it`.

The option is still accepted and still part of the client pool key, so existing calls keep working.

### Tests

`TestQUICFingerprintLeavesTCPClientHelloAlone` in `http3_fixes_test.go`: a local TLS listener records the ALPN list of a request made with `quicFingerprint`. It must be `[h2 http/1.1]`. Failed before with the `h3` list.

### Left out

Wiring `USpec` through to uquic, so that the QUIC Initial packet really changes. It is not small. quic-go's `http3.Transport` cannot take a uquic connection, so it means moving HTTP/3 requests to uquic's own `http3` package, an old quic-go fork with different types and without request trailers. uquic's last tag (v0.0.6) only has Chrome 115 and Firefox 116 parrots, and the brief ruled out bumping it. The existing `USpec` path (`uhttp3Dial`, then a second plain dial in `makeHTTP3Request`) is unchanged.

`QUICStringToSpec` and `CreateUQuicSpecFromFingerprint` stay in `utils.go`; nothing on the request path calls them now. The README still lists `quicFingerprint` as "QUIC fingerprint" and should say it has no effect yet.

## 6. `proxyHeaders`

### What was wrong

There was no way to add headers to the CONNECT request. Proxy providers use them for session, country and sticky-IP control. `connectDialer` had a `DefaultHeader` map and a context hook (`ContextKeyHeader`), and nothing set either from options.

### What changed

- `connect.go`: `setProxyHeaders` copies the option into `connectDialer.DefaultHeader`; `proxyHeadersKey` feeds the pool key. `createNewClient` in `client.go` calls `setProxyHeaders` right after it builds the dialer.
- Header names keep the casing given. A header with the same name as a default (`User-Agent`, and `Proxy-Authorization` from the proxy URL's credentials) replaces it, whatever the casing.
- The headers go only on the CONNECT request, for `http://` and `https://` proxies, over HTTP/1.1 or HTTP/2. They are not sent to the target. SOCKS proxies have no CONNECT request, so the option does nothing there.
- Request `headers` are not sent to the proxy, as before.

```js
{ proxy: 'http://user:pass@proxy.example:8080',
  proxyHeaders: { 'X-Proxy-Session': 'abc123', 'X-Proxy-Country': 'de' } }
```

```go
cycletls.Options{
    Proxy:        "http://user:pass@proxy.example:8080",
    ProxyHeaders: map[string]string{"X-Proxy-Session": "abc123"},
}
```

### Tests

`connect_proxy_headers_test.go`: a local TCP listener records the raw CONNECT request, answers 200, then plays the origin inside the tunnel and records that request too.

- `TestProxyHeadersSentOnConnect`: the headers are on CONNECT with their casing, the defaults are still there, the request's own headers are not on CONNECT, and the proxy headers are not on the tunnelled request.
- `TestProxyHeadersOverrideDefaults`: one `user-agent` and one `Proxy-Authorization` line, with the caller's values.
- `TestNoProxyHeadersLeavesConnectUnchanged`: without the option the CONNECT request is byte-for-byte what 3.0.0 sent. Passed before and after.
- `TestProxyHeadersJSONAndSocks`: JSON decoding, `browserFromOptions`, and that SOCKS dialers are left alone.

### Left out

- A test against an HTTPS proxy that negotiates HTTP/2. The headers travel in the same `DefaultHeader` map on that path; it was read, not run.
- Multi-value proxy headers. The option is `map[string]string`, one value per name.
- The proxy's response headers and body on a failed CONNECT are still not returned to the caller.

## Shared plumbing

Both new options are threaded the same way:

- `types.go`: two fields at the end of `Options`.
- `client.go`: two fields on `Browser`; `generateClientKey` adds both to the pool key, so clients that differ only in these options do not share connections; one line in `createNewClient`.
- `index.go`: `browserFromOptions` and the `Browser` literal in `Do` copy both fields.
- `roundtripper.go`: `roundTripper.HTTP2Priority` plus the cached parsed fingerprint; `newRoundTripper` copies the field.
