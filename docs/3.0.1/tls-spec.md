# 3.0.1: TLS ClientHello changes

utls stays at v1.8.2. Everything here uses what that version already ships.

Three options are new, all off by default:

| Go (`Options` / `Browser`) | JSON | Type | What it does |
|---|---|---|---|
| `TLSProfile` | `tlsProfile` | `string` | Send the ClientHello of a named utls profile |
| `ShuffleExtensions` | `shuffleExtensions` | `bool` | Random extension order on each connection |
| `ClientHello` | `clientHello` | `string` | Replay a captured ClientHello given as hex |

Which option decides the ClientHello, first match wins:

1. `clientHello`
2. `tlsProfile`
3. `quicFingerprint`
4. `ja3`
5. `ja4r`
6. the User-Agent default (Chrome spec, or utls Firefox for a Firefox User-Agent)

So `tlsProfile` and `clientHello` override `ja3` and `ja4r` when both are set. Nothing is merged: the losing options are ignored for TLS.

The tests for all of this are Go unit tests in `cycletls/` that need no network. They build the ClientHello with a utls `UClient` on an in-memory pipe, or send it to an `httptest` TLS server on 127.0.0.1 that records what it receives, and read the result with the new JA3/JA4 helpers.

## 1. JA4R path sent empty extension bodies

**What was missing.** `CreateExtensionFromID` (`cycletls/extensions.go`) had no case for 0x0011, 0x0015, 0x0018, 0x0029, 0x002c, 0x0032 and 0x0039, so each became `GenericExtension{Id, Data: []}`. Run against the old code, the new tests reported:

```
extension 0x0011 has a zero-length body
extension 0x0018 has a zero-length body
extension 0x002c has a zero-length body
extension 0x0032 has a zero-length body
pre_shared_key is extension 8 of 11, want last
pre_shared_key has a zero-length body
```

An empty `signature_algorithms_cert` and a `pre_shared_key` that is empty or not last are invalid TLS. The JA3 path already had valid bodies for the same IDs in `genMap`.

**What changed.**

- `CreateExtensionFromID`: before falling back to an empty generic extension, it looks the ID up in `genMap`. That covers 0x0011 (`StatusRequestV2Extension`), 0x0018 (`FakeTokenBindingExtension`), 0x0029 (`UtlsPreSharedKeyExtension`), 0x002c (`CookieExtension`), 0x0032 (`SignatureAlgorithmsCertExtension`), 0x0039 (`QUICTransportParametersExtension`), and also 15, 13172, 30031 and 30032.
- 0x0015 is a `UtlsPaddingExtension` with the new `ja4rPaddingStyle`: the BoringSSL length when the unpadded hello is 256 to 511 bytes, one zero byte otherwise. BoringSSL sends no padding extension outside that range, but a JA4_r that lists 0x0015 counts and hashes it, so it is always sent.
- `JA4RStringToSpec` (`cycletls/utils.go`) calls the new `movePreSharedKeyLast`. JA4_r sorts extensions by ID, which put 0x0029 mid-list.

**JA4.** The IDs on the wire are the same as before, so the JA4 is the same. The one exception is 0x0029: with no session to resume there are no identities or binders to send, and utls leaves the extension out (`Config.OmitEmptyPsk`, which `dialTLS` sets). A JA4_r string that lists 0029 therefore produces a hello with one extension fewer than the string says. Before this change the hello carried `00290000`, which matched the count and was not valid TLS.

**Tests** (`cycletls/extension_bodies_test.go`): `TestJA4RExtensionBodiesAreNotEmpty` checks each body is non-empty, that 0x0011 is `000702000400000000`, that padding is zero bytes, and that the JA4 of the hello equals the JA4 of the input string. `TestJA4RPreSharedKeyIsLastOrAbsent` checks 0x0029 is last or absent and every other ID is still sent. Both failed before the change with the output above.

**Left out.**

- 0x0039 (`quic_transport_parameters`) is now the utls type but still has an empty parameter list. That is a well-formed encoding, and the extension has no meaning on a TCP connection whatever its body.
- 0x002c (`cookie`) is sent as an empty cookie (`0000`). A cookie belongs in the reply to a HelloRetryRequest, so there is no right value for a first hello.
- The hand-written delegated credentials body (0x0022, two algorithms where Firefox sends four) and `case 0x6399`, which treats a key-exchange group as an extension ID, are unchanged. Both were outside this item.
- The JA4R path still adds no GREASE (gap 11 in the research report). That changes bytes for every JA4R user and needs its own changelog line.

## 2. More JA3 extension IDs

**What was missing.** `StringToSpec` returns an error for any ID not in `genMap`. 15 and 30031 were not there. 17 was sent with an empty body and 30032 with one zero byte, which no client sends. Before the change `TestJA3ExtensionIDs` and `TestJA3NewExtensionIDsOneByOne` failed in `StringToSpec` with "Extension {{ 15 }} is not Supported".

**What changed** (`genMap` in `cycletls/utils.go`):

| ID | Before | Now | Body |
|---|---|---|---|
| 15 heartbeat | error | `GenericExtension{15, 01}` | `01` |
| 30031 channel_id (old) | error | `FakeChannelIDExtension{OldExtensionID: true}` | empty |
| 17 status_request_v2 | empty generic | `StatusRequestV2Extension{}` | `000702000400000000` |
| 30032 channel_id | generic, body `00` | `FakeChannelIDExtension{}` | empty |

17 and 30032 change bytes for JA3 strings that contain them. JA3 and JA4 hashes do not change, since neither covers extension bodies.

`supportedSignatureAlgorithmsExtensions` in `extensions.go` is removed. Its keys were names (`"PKCS1WithSHA256"`, `"rsa_pkcs1_sha256"`) and the only lookup used `fmt.Sprintf("0x%04x", rawSig)`, so no lookup could match; the raw-value fallback did the work and still does.

**Tests** (`cycletls/extension_bodies_test.go`): the JA3 read back from the hello equals the input string, which fixes each ID's position; bodies are compared byte for byte; 15 and 30031 are each tried first, in the middle and last; extension 42 is still refused.

**Left out.** 17613 stays a `GenericExtension`. `utls.ApplicationSettingsExtensionNew` writes the same five bytes, but `cycletls/tests/unit/issue427_test.go` asserts the generic type and that directory is not mine to edit. IDs with no known valid body (42, 47, 14, 58, 65486) still return the error.

## 3. Named TLS profiles: `tlsProfile`

**What was missing.** The only use of a utls parrot was `HelloFirefox_Auto` for a Firefox User-Agent with no fingerprint. A JA3 string cannot carry signature algorithms, key shares, supported versions, certificate compression, ALPS or padding; a utls profile carries all of them.

**What changed** (new file `cycletls/tls_profiles.go`):

- `TLSProfileNames() []string` returns the accepted names, sorted.
- `TLSProfileToSpec(name string, forceHTTP1 bool) (*utls.ClientHelloSpec, error)` maps the name to a `utls.ClientHelloID`, calls `utls.UTLSIdToSpec`, and applies the forceHTTP1 filter. An unknown name returns `unknown tlsProfile "x"; valid names: ...` with the full list. Names are matched in lower case with `.`, `-` and spaces read as `_`, so `Safari_16.0` works.
- `applyForceHTTP1` is the ALPN/ALPS filter that used to be inline in `ModernChromeSpec`, which now calls it.

The 39 names: `chrome`, `chrome_58`, `chrome_62`, `chrome_70`, `chrome_72`, `chrome_83`, `chrome_87`, `chrome_96`, `chrome_100`, `chrome_102`, `chrome_106`, `chrome_115_pq`, `chrome_120`, `chrome_120_pq`, `chrome_131`, `chrome_133`, `firefox`, `firefox_55`, `firefox_56`, `firefox_63`, `firefox_65`, `firefox_99`, `firefox_102`, `firefox_105`, `firefox_120`, `safari`, `safari_16_0`, `ios`, `ios_11_1`, `ios_12_1`, `ios_13`, `ios_14`, `edge`, `edge_85`, `edge_106`, `android_11_okhttp`, `360_7_5`, `360_11_0`, `qq_11_1`. The unversioned names follow the utls "Auto" IDs (today Chrome 133, Firefox 120, Safari 16.0, iOS 14, Edge 85) and will move when utls is upgraded.

```go
resp, err := client.Do("https://example.com", cycletls.Options{
    TLSProfile: "chrome_133",
    UserAgent:  "Mozilla/5.0 ... Chrome/133.0.0.0 Safari/537.36",
}, "GET")
```

```json
{ "url": "https://example.com", "tlsProfile": "firefox_120", "userAgent": "Mozilla/5.0 ... Firefox/120.0" }
```

A profile covers the TLS ClientHello only. HTTP/2 settings, pseudo-header order, header order and the User-Agent still come from `http2Fingerprint`, `headerOrder` and `userAgent`; set them to match the profile.

**Tests** (`cycletls/tls_spec_options_test.go`): `TestTLSProfilesHandshakeAndMatchUTLS` runs every name: it completes a handshake with the local server, then compares the JA4 and JA3 of the hello the server received with a hello utls builds from the same `ClientHelloID`. For profiles that shuffle (Chrome 106 and later) the JA3 comparison sorts the extension list. Padding is left out of both sides: BoringSSL-style padding depends on the hello length and the Chrome GREASE ECH extension has a random length, so two hellos from one utls spec can differ in whether they carry it. Other tests cover the name list and error text, forceHTTP1, a real request through `newRoundTripper` for three profiles with `ja3` and `ja4r` also set, and the error for an unknown name.

**Left out.**

- The PSK parrots (`HelloChrome_100_PSK`, `112_PSK_Shuf`, `114_Padding_PSK_Shuf`, `115_PQ_PSK`): without a session cache they send no PSK, so they would be duplicates of the plain profiles under a misleading name.
- `HelloRandomized*`: the research probe saw one of two handshakes fail.
- Chrome 155, Firefox 148 and Safari 26.3 exist only on untagged utls master, which needs Go 1.27.
- No HTTP/2 defaults per profile.
- HTTP/3 is not affected by the option.

## 4. Extension-order shuffle: `shuffleExtensions`

**What was missing.** `ModernChromeSpec` has one fixed extension order and the JA4R path sends extensions sorted by ID. Chrome has sent a new random order on every connection since version 106.

**What changed.** With `shuffleExtensions: true`, `shuffleSpecExtensions` (`cycletls/tls_profiles.go`) calls `utls.ShuffleChromeTLSExtensions` on the spec. GREASE, padding and pre_shared_key keep their positions. It applies on three paths:

- the default Chrome spec and the `ja4r` path, in `roundTripper.dialTLS`;
- a `tlsProfile`, in `roundTripper.optInSpec`.

It does nothing with an explicit `ja3` or an imported `clientHello`, where the caller gave the order, nor on the Firefox default path, since Firefox does not shuffle. The JA3 hash changes on every connection when this is on, as it does for Chrome. JA4 does not, because JA4 sorts.

```go
cycletls.Options{ShuffleExtensions: true, UserAgent: chromeUA}
```

```json
{ "url": "https://example.com", "ja4r": "t13d1516h2_...", "shuffleExtensions": true }
```

**Tests** (`TestShuffleExtensions`): each path sends real requests to the local server. With the option off, four hellos have the same order. With it on, six hellos have at least two different orders, the same extension set and the same JA4, and on the Chrome path GREASE stays first and last. With `ja3` set and the option on, the JA3 on the wire equals the input four times running; an imported hello keeps its order too.

**Left out.** Shuffling on by default for Chrome User-Agents. It would change the JA3 of every default-path user.

## 5. ClientHello importer: `clientHello`

**What was missing.** `utls.Fingerprinter` was not used anywhere, so there was no way to copy a client that JA3 or JA4_r cannot describe.

**What changed** (new file `cycletls/clienthello_import.go`): `ClientHelloToSpec(clientHelloHex string, forceHTTP1 bool) (*utls.ClientHelloSpec, error)`.

- Input is the hex of the TLS record as captured (`16 03 01 ...`) or of the bare handshake message (`01 ...`). Upper or lower case, an optional `0x`, and spaces, newlines or `:` between bytes are accepted. A hello split across several records is joined.
- It runs `utls.Fingerprinter{AllowBluntMimicry: true}.FingerprintClientHello`, so extensions utls has no type for are replayed byte for byte.
- The captured server name is not kept: the SNI extension stays and utls fills in the host of each connection.
- A captured `pre_shared_key` is dropped. Its ticket and binder belong to another session.
- utls discards captured key shares and makes new ones, and GREASE values are redrawn per connection.
- Bad hex returns `clientHello: not valid hex: ...`; a record that is not a ClientHello or is cut short returns an error that also starts with `clientHello:`.

```go
cycletls.Options{ClientHello: "16030106b2010006ae0303...", UserAgent: ua}
```

```json
{ "url": "https://example.com", "clientHello": "16030106b2010006ae0303..." }
```

**Tests**: `TestClientHelloImportRoundTrip` takes hellos produced by this code (default Chrome, `firefox_120`, `safari_16_0`, a JA3 with 15, 17 and 30031), imports each in four input forms, rebuilds, and checks JA3, JA4, JA4_r, ALPN and the bodies of the extensions that hold no per-connection value; the imported spec then completes a handshake with the local server. Further tests: the captured SNI is replaced, an appended `pre_shared_key` is dropped, forceHTTP1 works, five kinds of bad input return an error, and a real request with `clientHello`, `tlsProfile` and `ja3` all set sends the imported hello.

**Left out.** Base64 input, the tlsfingerprint.io JSON format (`UnmarshalJSONClientHello`), and real PSK resumption from a capture (`RealPSKResumption`), which needs a session cache. The padding extension of a capture pads to the captured hello length, which is what utls does.

## 6. JA3 and JA4 of a ClientHello

**What was missing.** `GenerateJA4` (`cycletls/ja4.go`) was not the FoxIO algorithm: no cipher or extension counts, no ALPN characters, hashes cut to 1 and 4 hex characters, HTTP headers and the User-Agent mixed in. A search of every `.go` file found no caller. Tests had to ask tls.peet.ws what was sent.

**What changed** (new file `cycletls/clienthello_fingerprint.go`):

- `ParseClientHello(raw []byte) (*ClientHelloInfo, error)` reads TLS records or a bare handshake message.
- `ClientHelloInfo` has methods `JA3()`, `JA3Hash()`, `JA4()`, `JA4R()` and `ExtensionBody(id)`.
- `JA3FromClientHello(raw) (ja3, hash string, err error)`, `JA4FromClientHello(raw) (string, error)`, `JA4RFromClientHello(raw) (string, error)`.

JA4 follows the FoxIO rules: version from `supported_versions` when present, `d` or `i` for SNI, counts without GREASE and capped at 99, first and last character of the first ALPN value (`00` when there is none, hex digits when not alphanumeric), sorted lists, SNI and ALPN counted but not hashed, signature algorithms in wire order, 12 hex characters per hash.

`GenerateJA4` keeps its signature and now returns the FoxIO format for what it is given. Its arguments carry no ALPN list and no signature algorithms, so the ALPN part is `00` and the last hash covers extension IDs only; `headers` and `userAgent` are ignored. It is marked deprecated in favour of `JA4FromClientHello`. Its unused helpers `getTLSVersionString`, `hashCipherSuites` and `hashExtensions` are gone. `GenerateJA4HTTP` and `GenerateJA4H2` are unchanged.

```go
uconn.BuildHandshakeState()
ja4, err := cycletls.JA4FromClientHello(uconn.HandshakeState.Hello.Raw)
```

**Tests** (`cycletls/clienthello_fingerprint_test.go`): `TestJA4MatchesFoxIOExample` reproduces the Chrome example from the FoxIO spec, `t13d1516h2_8daaf6152771_e5627efa2ab1`, and its JA4_r. Others cover the prefix rules, a hello built by utls from a known spec (as a message, one record, and split records), truncated input, and `GenerateJA4`.

**Left out.** `GenerateJA4`'s output changed. Any Go caller that stored the old string will see a different one; the old one was not a JA4. The fingerprint is not added to the response. JA4 for QUIC (`q` prefix) and JA4H are not covered. `GenerateJA4HTTP` is also not the FoxIO JA4H; it was outside this item.

## Shared-file edits

| File | Where | Edit |
|---|---|---|
| `cycletls/types.go` | end of `Options` | three fields under `// 3.0.1: TLS spec options` |
| `cycletls/client.go` | end of `Browser` | three fields |
| `cycletls/client.go` | `generateClientKey` | one statement appending the three options to the key string |
| `cycletls/roundtripper.go` | `roundTripper` struct, `newRoundTripper` | three fields and their copy from `Browser` |
| `cycletls/roundtripper.go` | `dialTLS`, spec selection | `rt.optInSpec()` ahead of the existing chain; shuffle on the JA4R and default-Chrome branches |
| `cycletls/index.go` | `browserFromOptions`, and the `Browser{}` literal in `CycleTLS.Do` | three fields each |

## For whoever owns the retry code

`retryWithTLS13CompatibleCurves` rebuilds the spec from `ja3` or the default Chrome JA3. If the first handshake of a `tlsProfile` or `clientHello` connection fails with "CurvePreferences includes unsupported curve" and `tls13AutoRetry` is on, the retry sends a different fingerprint from the one asked for. That function was outside the files this change could touch.
