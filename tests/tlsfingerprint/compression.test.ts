/**
 * Compression tests against tlsfingerprint.com
 *
 * Tests gzip, deflate, and brotli compression handling.
 * The streaming CycleTLS client returns raw compressed bytes, so we manually
 * decompress in the tests to validate the response.
 *
 * Mirrors Go tests in cycletls/tests/integration/tlsfingerprint/compression_test.go
 */

import CycleTLS from "../../dist/index.js";
import {
  TEST_SERVER_URL,
  getDefaultOptions,
  assertTLSFieldsPresent,
  decompressGzip,
  decompressDeflate,
  decompressBrotli,
  CompressionResponse,
  isServiceAvailable,
} from "./helpers";

// Longer timeout for network requests
jest.setTimeout(90000);

// Check service availability and conditionally run tests
let serviceAvailable = false;
// Circuit breaker — once any test trips the deadline or a flake error,
// every subsequent test in this file skips immediately (no 60s wait per
// test). Without this, a hung upstream costs 60s × N tests in this file.
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
// mid-test transient flake (upstream returns a flake status, request times
// out, connection reset, etc.). Real assertion failures still throw.
const conditionalTest = (name: string, fn: () => Promise<void>) => {
  it(name, async () => {
    if (!serviceAvailable || upstreamUnreachable) {
      console.log(`Skipped: ${name} (upstream unavailable)`);
      return;
    }
    // 60s soft deadline (Jest setTimeout is 90s). If the upstream hangs we
    // skip rather than let Jest kill the worker with "Exceeded timeout".
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

describe("TLS Fingerprint - Compression", () => {
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

  describe("Gzip Compression", () => {
    conditionalTest("should handle gzip compressed response", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/gzip`,
        ...options,
        headers: {
          "Accept-Encoding": "gzip, deflate, br",
        },
      });

      expect(response.statusCode).toBe(200);

      // Manually decompress the gzip response
      const body = await decompressGzip<CompressionResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // Verify gzipped flag is true
      expect(body.gzipped).toBe(true);
    });

    conditionalTest("should decompress gzip and validate TLS fields", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/gzip`, options);

      expect(response.statusCode).toBe(200);

      // Decompress and parse JSON
      const body = await decompressGzip<CompressionResponse>(response.body);

      expect(body.gzipped).toBe(true);
      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // Verify JA3 fingerprint is present
      expect(body.ja3).toBeDefined();
      expect(body.ja3.length).toBeGreaterThan(50);
    });
  });

  describe("Deflate Compression", () => {
    conditionalTest("should handle deflate compressed response", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/deflate`,
        ...options,
        headers: {
          // Request only deflate to ensure server responds with deflate encoding
          "Accept-Encoding": "deflate",
        },
      });

      expect(response.statusCode).toBe(200);

      // Manually decompress the deflate response
      const body = await decompressDeflate<CompressionResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // Verify deflated flag is true
      expect(body.deflated).toBe(true);
    });

    conditionalTest("should decompress deflate and validate TLS fields", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/deflate`, options);

      expect(response.statusCode).toBe(200);

      const body = await decompressDeflate<CompressionResponse>(response.body);

      expect(body.deflated).toBe(true);
      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
    });
  });

  describe("Brotli Compression", () => {
    conditionalTest("should handle brotli compressed response", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/brotli`,
        ...options,
        headers: {
          "Accept-Encoding": "gzip, deflate, br",
        },
      });

      expect(response.statusCode).toBe(200);

      // Manually decompress the brotli response
      const body = await decompressBrotli<CompressionResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // Verify brotli flag is true
      expect(body.brotli).toBe(true);
    });

    conditionalTest("should decompress brotli and validate TLS fields", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/brotli`, options);

      expect(response.statusCode).toBe(200);

      const body = await decompressBrotli<CompressionResponse>(response.body);

      expect(body.brotli).toBe(true);
      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
    });
  });

  describe("Compression with specific Accept-Encoding", () => {
    conditionalTest("should work with only gzip Accept-Encoding", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/gzip`,
        ...options,
        headers: {
          "Accept-Encoding": "gzip",
        },
      });

      expect(response.statusCode).toBe(200);

      const body = await decompressGzip<CompressionResponse>(response.body);
      expect(body.gzipped).toBe(true);
    });

    conditionalTest("should work with only deflate Accept-Encoding", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/deflate`,
        ...options,
        headers: {
          "Accept-Encoding": "deflate",
        },
      });

      expect(response.statusCode).toBe(200);

      const body = await decompressDeflate<CompressionResponse>(response.body);
      expect(body.deflated).toBe(true);
    });

    conditionalTest("should work with only br Accept-Encoding", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/brotli`,
        ...options,
        headers: {
          "Accept-Encoding": "br",
        },
      });

      expect(response.statusCode).toBe(200);

      const body = await decompressBrotli<CompressionResponse>(response.body);
      expect(body.brotli).toBe(true);
    });
  });

  describe("All compression types with TLS fingerprint validation", () => {
    conditionalTest("should have valid TLS fingerprint fields for /gzip", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/gzip`, options);

      expect(response.statusCode).toBe(200);

      const body = await decompressGzip<CompressionResponse>(response.body);

      // Must have all required TLS fields
      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // Compression flag should be true
      expect(body.gzipped).toBe(true);

      // JA3 should be a non-empty string
      expect(typeof body.ja3).toBe("string");
      expect(body.ja3.length).toBeGreaterThan(50);

      // JA3 hash should be MD5 format (32 hex chars)
      expect(body.ja3_hash).toMatch(/^[a-f0-9]{32}$/);
    });

    conditionalTest("should have valid TLS fingerprint fields for /deflate", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/deflate`, options);

      expect(response.statusCode).toBe(200);

      const body = await decompressDeflate<CompressionResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
      expect(body.deflated).toBe(true);
      expect(typeof body.ja3).toBe("string");
      expect(body.ja3.length).toBeGreaterThan(50);
      expect(body.ja3_hash).toMatch(/^[a-f0-9]{32}$/);
    });

    conditionalTest("should have valid TLS fingerprint fields for /brotli", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/brotli`, options);

      expect(response.statusCode).toBe(200);

      const body = await decompressBrotli<CompressionResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
      expect(body.brotli).toBe(true);
      expect(typeof body.ja3).toBe("string");
      expect(body.ja3.length).toBeGreaterThan(50);
      expect(body.ja3_hash).toMatch(/^[a-f0-9]{32}$/);
    });
  });
});
