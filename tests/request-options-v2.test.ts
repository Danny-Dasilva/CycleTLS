/**
 * Tests for V2 RequestOptions interface completeness
 * Verifies all new options are accepted by the type system
 */

import { CycleTLS, RequestOptions, RequestCookie } from "../src/flow-control-client";

describe("RequestOptions V2 Interface", () => {
  // Type-level test: ensure all new options are accepted
  const fullOptions: RequestOptions = {
    // Original options
    url: "https://example.com",
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: '{"test": true}',
    ja3: "771,4865-4866-4867-49195-49199,0-23-65281-10-11-35-16-5-13-18-51-45-43,29-23-24,0",
    ja4r: "t13d1516h2_8daaf6152771_e5627efa2ab1",
    userAgent: "Mozilla/5.0",
    proxy: "http://proxy:8080",
    timeout: 30000,
    disableRedirect: true,
    insecureSkipVerify: false,
    forceHTTP1: false,
    forceHTTP3: false,

    // NEW options for Go parity
    bodyBytes: new Uint8Array([0x01, 0x02, 0x03]),
    http2Fingerprint: "1:65536,4:131072,5:16384|12517377|3:0:0:201,5:0:0:101,7:0:0:1,9:0:7:1,11:0:3:1,13:0:0:241|m,p,a,s",
    quicFingerprint: "custom-quic-fingerprint",
    disableGrease: true,
    serverName: "custom.example.com",
    headerOrder: ["Host", "User-Agent", "Accept", "Accept-Language"],
    orderAsProvided: true,
    cookies: [
      { name: "session", value: "abc123" },
      { name: "tracking", value: "xyz789", path: "/", domain: ".example.com" },
    ],
    tls13AutoRetry: true,
    enableConnectionReuse: true,
  };

  it("should accept all RequestOptions fields", () => {
    // This test is primarily a compile-time check
    // If the interface is incorrect, TypeScript will fail to compile
    expect(fullOptions.url).toBe("https://example.com");
    expect(fullOptions.bodyBytes).toBeInstanceOf(Uint8Array);
    expect(fullOptions.http2Fingerprint).toBeDefined();
    expect(fullOptions.quicFingerprint).toBeDefined();
    expect(fullOptions.disableGrease).toBe(true);
    expect(fullOptions.serverName).toBe("custom.example.com");
    expect(fullOptions.headerOrder).toHaveLength(4);
    expect(fullOptions.orderAsProvided).toBe(true);
    expect(fullOptions.cookies).toHaveLength(2);
    expect(fullOptions.tls13AutoRetry).toBe(true);
    expect(fullOptions.enableConnectionReuse).toBe(true);
  });

  it("should accept RequestCookie interface", () => {
    const cookie: RequestCookie = {
      name: "test",
      value: "value",
      path: "/api",
      domain: ".test.com",
    };
    expect(cookie.name).toBe("test");
    expect(cookie.path).toBe("/api");
  });

  it("should allow partial options (only url required)", () => {
    const minimalOptions: RequestOptions = {
      url: "https://example.com",
    };
    expect(minimalOptions.url).toBeDefined();
    expect(minimalOptions.bodyBytes).toBeUndefined();
    expect(minimalOptions.http2Fingerprint).toBeUndefined();
  });

  it("should export CycleTLS class", () => {
    expect(CycleTLS).toBeDefined();
    // Don't instantiate to avoid starting server, just check export
  });
});
