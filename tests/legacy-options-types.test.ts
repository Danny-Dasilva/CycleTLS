/**
 * Type-level test for the options added in 3.0.1. ts-jest type-checks this
 * file, so it fails to compile when an option is missing from either client's
 * request option type.
 */
import type { CycleTLSRequestOptions, RequestOptions, HTTP2Priority, RequestHeaders } from "../dist/index.js";

const pem = "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----";
const akamai = "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p";
const pin = "sha256/47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=";

describe("3.0.1 request option types", () => {
  test("legacy client options (initCycleTLS)", () => {
    const priority: HTTP2Priority = { weight: 256, exclusive: true, streamDep: 0 };
    const headers: RequestHeaders = { "x-int": 1, "x-bool": true, "x-multi": ["a", 2], accept: "*/*" };

    const options: CycleTLSRequestOptions = {
      headers,
      tlsProfile: "chrome_133",
      shuffleExtensions: true,
      clientHello: "16030100",
      sessionResumption: true,
      caCerts: [pem],
      certPins: [pin],
      http2Priority: priority,
      proxyHeaders: { "X-Session": "abc" },
      http2Fingerprint: akamai,
      tls13AutoRetry: true,
      bodyBytes: new Uint8Array([1, 2, 3]),
      timeout: -1,
      readTimeout: 5000,
    };
    const noPriority: CycleTLSRequestOptions = { http2Priority: false };
    const partialPriority: CycleTLSRequestOptions = { http2Priority: { weight: 42 } };
    const bufferBody: CycleTLSRequestOptions = { bodyBytes: Buffer.from("abc") };

    expect(Object.keys(options)).toHaveLength(14);
    expect(noPriority.http2Priority).toBe(false);
    expect(partialPriority.http2Priority).toEqual({ weight: 42 });
    expect(bufferBody.bodyBytes).toBeInstanceOf(Uint8Array);
  });

  test("default client options (new CycleTLS())", () => {
    const options: RequestOptions = {
      url: "https://example.com",
      tlsProfile: "chrome_133",
      shuffleExtensions: true,
      clientHello: "16030100",
      sessionResumption: true,
      caCerts: [pem],
      certPins: [pin],
      http2Priority: { weight: 256, exclusive: true, streamDep: 0 },
      proxyHeaders: { "X-Session": "abc" },
      http2Fingerprint: akamai,
      tls13AutoRetry: true,
      bodyBytes: new Uint8Array([1, 2, 3]),
    };
    const noPriority: RequestOptions = { url: "https://example.com", http2Priority: false };

    expect(Object.keys(options)).toHaveLength(12);
    expect(noPriority.http2Priority).toBe(false);
  });
});
