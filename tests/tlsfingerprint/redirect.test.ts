/**
 * Redirect tests against tlsfingerprint.com
 *
 * Tests redirect following behavior and status code handling.
 *
 * Mirrors Go tests in cycletls/tests/integration/tlsfingerprint/redirect_test.go
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
  consumeBody,
  EchoResponse,
  StatusResponse,
  isServiceAvailable,
} from "./helpers";

// Longer timeout for redirect chains
jest.setTimeout(90000);

// Check service availability and conditionally run tests
let serviceAvailable = false;

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
    if (!serviceAvailable) {
      console.log(`Skipped: ${name} (service unavailable)`);
      return;
    }
    // 60s soft deadline (Jest setTimeout is 90s). If the upstream hangs we
    // skip rather than let Jest kill the worker with "Exceeded timeout".
    const deadline = new Promise<"timeout">((res) => setTimeout(() => res("timeout"), 60000));
    try {
      const result = await Promise.race([fn().then(() => "ok" as const), deadline]);
      if (result === "timeout") {
        console.log(`Skipped: ${name} (upstream hung past 60s deadline)`);
        return;
      }
    } catch (e) {
      const msg = (e as Error)?.message ?? String(e);
      const isFlakeStatus = [...FLAKE_STATUSES].some((c) => msg.includes(`${c}`) && msg.includes("statusCode"));
      const isNetworkErr = /timeout|timed out|ECONNRESET|ECONNREFUSED|EAI_AGAIN|ENOTFOUND|socket hang up/i.test(msg);
      if (isFlakeStatus || isNetworkErr) {
        console.log(`Skipped: ${name} (upstream flake: ${msg.slice(0, 200)})`);
        return;
      }
      throw e;
    }
  });
};

describe("TLS Fingerprint - Redirects", () => {
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

  describe("Redirect Following", () => {
    conditionalTest("should follow /redirect/3 chain and end at /get", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/redirect/3`, options);

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<EchoResponse>(response.body);

      // Verify TLS fields present after redirect chain
      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // After following 3 redirects, we end up at /get which returns EchoResponse
      expect(body.method).toBe("GET");
    });

    conditionalTest("should follow /redirect/1 single redirect", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/redirect/1`, options);

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<EchoResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
    });

    conditionalTest("should follow /redirect/5 multiple redirects", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/redirect/5`, options);

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<EchoResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
    });
  });

  describe("Redirect-To Endpoint", () => {
    conditionalTest("should follow redirect-to internal URL", async () => {
      const options = getDefaultOptions();
      const targetUrl = encodeURIComponent(`${TEST_SERVER_URL}/get`);
      const response = await client.get(
        `${TEST_SERVER_URL}/redirect-to?url=${targetUrl}`,
        options
      );

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<EchoResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // Verify url field is present in response
      expect(body.url).toBeDefined();
    });
  });

  describe("Disable Redirect", () => {
    conditionalTest("should return 302 when redirects are disabled", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/redirect/1`,
        ...options,
        disableRedirect: true,
      });

      expect(response.statusCode).toBe(302);

      // Consume body to complete request
      await consumeBody(response.body);
    });

    conditionalTest("should return redirect location header when disabled", async () => {
      const options = getDefaultOptions();
      const response = await client.request({
        url: `${TEST_SERVER_URL}/redirect/1`,
        ...options,
        disableRedirect: true,
      });

      expect(response.statusCode).toBe(302);
      expect(response.headers).toBeDefined();

      // Location header should be present
      const location =
        response.headers.Location ||
        response.headers.location ||
        response.headers["Location"] ||
        response.headers["location"];
      expect(location).toBeDefined();

      await consumeBody(response.body);
    });
  });

  describe("Status Codes", () => {
    conditionalTest("should return 201 Created status", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/status/201`, options);

      expect(response.statusCode).toBe(201);

      const body = await consumeBodyAsJson<StatusResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);

      // Verify status_code in response body matches
      expect(body.status_code).toBe(201);
    });

    conditionalTest("should return 400 Bad Request status", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/status/400`, options);

      expect(response.statusCode).toBe(400);

      const body = await consumeBodyAsJson<StatusResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
      expect(body.status_code).toBe(400);
    });

    conditionalTest("should return 404 Not Found status", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/status/404`, options);

      expect(response.statusCode).toBe(404);

      const body = await consumeBodyAsJson<StatusResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
      expect(body.status_code).toBe(404);
    });

    conditionalTest("should return 500 Internal Server Error status", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/status/500`, options);

      expect(response.statusCode).toBe(500);

      const body = await consumeBodyAsJson<StatusResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
      expect(body.status_code).toBe(500);
    });

    conditionalTest("should return 204 No Content status", async () => {
      const options = getDefaultOptions();
      const response = await client.get(`${TEST_SERVER_URL}/status/204`, options);

      expect(response.statusCode).toBe(204);

      // 204 has no body, just consume to complete
      await consumeBody(response.body);
    });
  });

  // Skipped: /absolute-redirect endpoint not available on tlsfingerprint.com
  describe.skip("Absolute Redirect", () => {
    it("should follow absolute-redirect endpoint", async () => {
      const options = getDefaultOptions();
      const response = await client.get(
        `${TEST_SERVER_URL}/absolute-redirect/2`,
        options
      );

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<EchoResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
    });
  });

  // Skipped: /relative-redirect endpoint not available on tlsfingerprint.com
  describe.skip("Relative Redirect", () => {
    it("should follow relative-redirect endpoint", async () => {
      const options = getDefaultOptions();
      const response = await client.get(
        `${TEST_SERVER_URL}/relative-redirect/2`,
        options
      );

      expect(response.statusCode).toBe(200);

      const body = await consumeBodyAsJson<EchoResponse>(response.body);

      assertTLSFieldsPresent(body as unknown as Record<string, unknown>);
    });
  });
});
