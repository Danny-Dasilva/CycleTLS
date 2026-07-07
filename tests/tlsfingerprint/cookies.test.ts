/**
 * Cookie tests against tlsfingerprint.com
 *
 * Tests sending and receiving cookies.
 *
 * Mirrors Go tests in cycletls/tests/integration/tlsfingerprint/cookies_test.go
 *
 * PREREQUISITES:
 * - Go binary must be rebuilt with V2 protocol support
 * - Run: npm run build:go (or platform-specific variant)
 */

import CycleTLS from "../../dist/index.js";
import {
  TEST_SERVER_URL,
  getDefaultOptions,
  assertTLSFieldsPresent,
  consumeBodyAsJson,
  CookiesResponse,
  isServiceAvailable,
} from "./helpers";

// Longer timeout for network requests
jest.setTimeout(90000);

// Check service availability and conditionally run tests
let serviceAvailable = false;
// Circuit breaker — once any test trips the deadline or a flake error,
// every subsequent test in this file skips immediately.
let upstreamUnreachable = false;

beforeAll(async () => {
  serviceAvailable = await isServiceAvailable();
  if (!serviceAvailable) {
    console.warn('SKIPPING tlsfingerprint tests: Service unavailable (received 521 or timeout)');
  }
});

// Status codes from upstream that we treat as transient flake.
const FLAKE_STATUSES = new Set<number>([408, 421, 502, 503, 504, 521, 522, 523, 524, 525]);

// Helper to conditionally run test. Skips on initial unavailability OR on
// mid-test transient upstream flake. Real assertion failures still throw.
const conditionalTest = (name: string, fn: () => Promise<void>) => {
  it(name, async () => {
    if (!serviceAvailable || upstreamUnreachable) {
      console.log(`Skipped: ${name} (upstream unavailable)`);
      return;
    }
    const deadline = new Promise<"timeout">((res) => setTimeout(() => res("timeout"), 60000));
    try {
      const result = await Promise.race([fn().then(() => "ok" as const), deadline]);
      if (result === "timeout") {
        upstreamUnreachable = true;
        console.log(`Skipped: ${name} (upstream hung past 60s; tripping circuit breaker)`);
        return;
      }
    } catch (e) {
      const msg = (e as Error)?.message ?? String(e);
      const isFlakeStatus = [...FLAKE_STATUSES].some((c) => msg.includes(`${c}`) && msg.includes("statusCode"));
      const isNetworkErr = /timeout|timed out|ECONNRESET|ECONNREFUSED|EAI_AGAIN|ENOTFOUND|socket hang up/i.test(msg);
      if (isFlakeStatus || isNetworkErr) {
        upstreamUnreachable = true;
        console.log(`Skipped: ${name} (upstream flake; tripping circuit breaker: ${msg.slice(0, 200)})`);
        return;
      }
      throw e;
    }
  });
};

describe("TLS Fingerprint - Cookies", () => {
  let client: CycleTLS;

  beforeEach(() => {
    if (!serviceAvailable) return;
    client = new CycleTLS({
      port: 9119,
      debug: false,
      timeout: 30000,
      autoSpawn: true,
    });
  });

  afterEach(async () => {
    if (client) {
      await client.close();
    }
  });

  describe("Cookie Reading", () => {
    conditionalTest("should send cookies and have them echoed back", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/cookies`,
        ...options,
        headers: {
          Cookie: "test=value1; session=abc123",
        },
      });

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<CookiesResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // Verify cookies are echoed back
      expect(body.cookies).toBeDefined();
      expect(body.cookies).not.toBeNull();

      if (body.cookies) {
        expect(body.cookies["test"]).toBe("value1");
        expect(body.cookies["session"]).toBe("abc123");
      }
    });

    conditionalTest("should send single cookie", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/cookies`,
        ...options,
        headers: {
          Cookie: "single_cookie=single_value",
        },
      });

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<CookiesResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      expect(body.cookies).toBeDefined();
      if (body.cookies) {
        expect(body.cookies["single_cookie"]).toBe("single_value");
      }
    });

    conditionalTest("should handle cookies with special characters", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/cookies`,
        ...options,
        headers: {
          // URL-encode special characters in cookie value
          Cookie: "encoded=hello%20world",
        },
      });

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<CookiesResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
      expect(body.cookies).toBeDefined();
    });

    conditionalTest("should handle multiple cookies", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/cookies`,
        ...options,
        headers: {
          Cookie: "cookie1=value1; cookie2=value2; cookie3=value3",
        },
      });

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<CookiesResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      expect(body.cookies).toBeDefined();
      if (body.cookies) {
        expect(body.cookies["cookie1"]).toBe("value1");
        expect(body.cookies["cookie2"]).toBe("value2");
        expect(body.cookies["cookie3"]).toBe("value3");
      }
    });
  });

  describe("Cookie Setting", () => {
    conditionalTest("should receive cookies set via /cookies/set", async () => {
      const options = getDefaultOptions();
      const response = await client.get(
        `${TEST_SERVER_URL}/cookies/set?name=value&foo=bar`,
        options
      );

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<CookiesResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // Verify the cookies were set and echoed back
      expect(body.cookies).toBeDefined();
      if (body.cookies) {
        expect(body.cookies["name"]).toBe("value");
        expect(body.cookies["foo"]).toBe("bar");
      }
    });

    conditionalTest("should receive single cookie set via query param", async () => {
      const options = getDefaultOptions();
      const response = await client.get(
        `${TEST_SERVER_URL}/cookies/set?session_id=xyz789`,
        options
      );

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<CookiesResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      expect(body.cookies).toBeDefined();
      if (body.cookies) {
        expect(body.cookies["session_id"]).toBe("xyz789");
      }
    });
  });

  describe("Cookie Delete", () => {
    conditionalTest("should handle cookie deletion endpoint", async () => {
      const options = getDefaultOptions();

      // First set a cookie
      await client.get(
        `${TEST_SERVER_URL}/cookies/set?deleteme=value`,
        options
      );

      // Then request deletion (this endpoint may redirect to /cookies)
      const deleteResponse = await client.get(
        `${TEST_SERVER_URL}/cookies/delete?deleteme`,
        options
      );

      expect(deleteResponse.statusCode).toBe(200);

      const body = await consumeBodyAsJson<CookiesResponse>(deleteResponse.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
    });
  });

  describe("Cookies with TLS Fingerprint", () => {
    conditionalTest("should have valid TLS fingerprint when sending cookies", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/cookies`,
        ...options,
        headers: {
          Cookie: "auth=token123",
        },
      });

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<CookiesResponse>(response.body);

      // Verify all required TLS fields
      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // JA3 should be present and valid
      expect(body.ja3).toBeDefined();
      expect(body.ja3.length).toBeGreaterThan(50);

      // JA3 hash should be MD5 format
      expect(body.ja3_hash).toMatch(/^[a-f0-9]{32}$/);

      // JA4 should be present
      expect(body.ja4).toBeDefined();
      expect(body.ja4.length).toBeGreaterThan(0);

      // Peetprint should be present
      expect(body.peetprint).toBeDefined();
      expect(body.peetprint_hash).toBeDefined();
    });
  });

  describe("No Cookies", () => {
    conditionalTest("should return empty cookies when none sent", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/cookies`, options);

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<CookiesResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // Cookies object should exist but be empty
      expect(body.cookies).toBeDefined();
      expect(Object.keys(body.cookies || {}).length).toBe(0);
    });
  });
});
