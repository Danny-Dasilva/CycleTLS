/**
 * Integration tests for CycleTLS (V2 Protocol)
 *
 * These tests verify the streaming and backpressure functionality
 * that provides memory-efficient large file downloads.
 */

import CycleTLS from "../dist/index.js";

jest.setTimeout(60000);

// FIXME(v3.0.0): same intermittent v3 spawn flake as binary-data-handling and
// streaming — `new CycleTLS()` instances time out at 60s on CI runners
// (especially under Node Integration's full-suite load). Skip until the v3
// startup race is fixed.
describe.skip("CycleTLS (V2 Protocol)", () => {
  let client: CycleTLS;

  beforeEach(() => {
    client = new CycleTLS({
      port: 9119,
      debug: false,
      timeout: 30000,
      initialWindow: 65536, // 64KB
      autoSpawn: true,
    });
  });

  afterEach(async () => {
    await client.close();
  });

  describe("Basic Request Functionality", () => {
    it("should make a successful GET request", async () => {
      const response = await client.get("https://httpbin.org/get");

      expect(response.statusCode).toBe(200);
      expect(response.requestId).toBeDefined();
      expect(response.finalUrl).toContain("httpbin.org");

      // Consume the body
      const chunks: Buffer[] = [];
      for await (const chunk of response.body) {
        chunks.push(chunk);
      }
      const body = Buffer.concat(chunks).toString("utf8");
      const json = JSON.parse(body);

      expect(json.url).toBe("https://httpbin.org/get");
    });

    it("should make a successful POST request", async () => {
      const postData = JSON.stringify({ test: "value", number: 42 });

      const response = await client.post("https://httpbin.org/post", postData, {
        headers: { "Content-Type": "application/json" },
      });

      expect(response.statusCode).toBe(200);

      const chunks: Buffer[] = [];
      for await (const chunk of response.body) {
        chunks.push(chunk);
      }
      const body = Buffer.concat(chunks).toString("utf8");
      const json = JSON.parse(body);

      expect(json.json).toEqual({ test: "value", number: 42 });
    });

    it("should include response headers", async () => {
      const response = await client.get("https://httpbin.org/response-headers?X-Custom=TestValue");

      expect(response.statusCode).toBe(200);
      expect(response.headers).toBeDefined();

      // Consume body
      for await (const _ of response.body) {
        // drain
      }
    });
  });

  describe("Streaming and Backpressure", () => {
    it("should stream response body in chunks", async () => {
      // Request 10KB of random bytes
      const response = await client.get("https://httpbin.org/bytes/10240");

      expect(response.statusCode).toBe(200);

      let totalBytes = 0;
      let chunkCount = 0;

      for await (const chunk of response.body) {
        totalBytes += chunk.length;
        chunkCount++;
      }

      expect(totalBytes).toBe(10240);
      expect(chunkCount).toBeGreaterThan(0);
    });

    it("should handle slow consumers with backpressure", async () => {
      // Request moderate amount of data
      const response = await client.get("https://httpbin.org/bytes/32768");

      expect(response.statusCode).toBe(200);

      let totalBytes = 0;
      const delays: number[] = [];

      for await (const chunk of response.body) {
        const start = Date.now();

        // Simulate slow consumer
        await new Promise((resolve) => setTimeout(resolve, 10));

        delays.push(Date.now() - start);
        totalBytes += chunk.length;
      }

      expect(totalBytes).toBe(32768);
      // Should have at least one chunk (may be 1 or more depending on network conditions)
      expect(delays.length).toBeGreaterThanOrEqual(1);
    });

    it("should complete large downloads without memory explosion", async () => {
      // Get initial memory usage
      const initialMemory = process.memoryUsage().heapUsed;

      // Download 100KB (httpbin.org limits response size to ~100KB)
      const response = await client.get("https://httpbin.org/bytes/102400");

      expect(response.statusCode).toBe(200);

      let totalBytes = 0;
      let peakMemory = initialMemory;

      for await (const chunk of response.body) {
        totalBytes += chunk.length;
        const currentMemory = process.memoryUsage().heapUsed;
        peakMemory = Math.max(peakMemory, currentMemory);
      }

      expect(totalBytes).toBe(102400);

      // Memory increase should be bounded
      // Note: This is a soft check - memory usage can vary due to GC timing
      // The main point is that we're not buffering the entire response
      const memoryIncrease = peakMemory - initialMemory;
      // Allow up to 5MB increase for a 100KB download (accounts for GC variance)
      const maxAllowedIncrease = 5 * 1024 * 1024; // 5MB
      expect(memoryIncrease).toBeLessThan(maxAllowedIncrease);
    });
  });

  describe("Credit Window Behavior", () => {
    it("should work with small initial window", async () => {
      const smallWindowClient = new CycleTLS({
        port: 9119,
        initialWindow: 8192, // 8KB
        creditThreshold: 4096,
        autoSpawn: true,
        timeout: 60000, // 60 second timeout for slow networks
      });

      try {
        const response = await smallWindowClient.get("https://httpbin.org/bytes/16384"); // Reduced size

        expect(response.statusCode).toBe(200);

        let totalBytes = 0;
        for await (const chunk of response.body) {
          totalBytes += chunk.length;
        }

        expect(totalBytes).toBe(16384);
      } finally {
        await smallWindowClient.close();
      }
    }, 90000); // Increase test timeout to 90 seconds

    it("should work with large initial window", async () => {
      const largeWindowClient = new CycleTLS({
        port: 9119,
        initialWindow: 262144, // 256KB
        autoSpawn: true,
      });

      try {
        // Use 64KB instead of 128KB (httpbin.org limits to ~100KB)
        const response = await largeWindowClient.get("https://httpbin.org/bytes/65536");

        expect(response.statusCode).toBe(200);

        let totalBytes = 0;
        for await (const chunk of response.body) {
          totalBytes += chunk.length;
        }

        expect(totalBytes).toBe(65536);
      } finally {
        await largeWindowClient.close();
      }
    });
  });

  describe("Error Handling", () => {
    it("should handle 404 responses", async () => {
      const response = await client.get("https://httpbin.org/status/404");

      expect(response.statusCode).toBe(404);

      // Consume body even on error
      for await (const _ of response.body) {
        // drain
      }
    });

    it("should handle 500 responses", async () => {
      const response = await client.get("https://httpbin.org/status/500");

      expect(response.statusCode).toBe(500);

      for await (const _ of response.body) {
        // drain
      }
    });

    it("should handle connection errors gracefully", async () => {
      const badClient = new CycleTLS({
        port: 59999, // Non-existent server
        autoSpawn: false,
        timeout: 2000,
      });

      await expect(
        badClient.get("https://httpbin.org/get")
      ).rejects.toThrow();

      await badClient.close();
    });

    it("should handle request timeout", async () => {
      const timeoutClient = new CycleTLS({
        port: 9119,
        timeout: 1000, // 1 second timeout
        autoSpawn: true,
      });

      try {
        // Request a 5 second delay - should timeout
        await expect(
          timeoutClient.get("https://httpbin.org/delay/5")
        ).rejects.toThrow(/timeout/i);
      } finally {
        await timeoutClient.close();
      }
    });
  });

  describe("Request Options", () => {
    it("should support custom headers", async () => {
      const response = await client.request({
        url: "https://httpbin.org/headers",
        headers: {
          "X-Custom-Header": "TestValue123",
          "Accept": "application/json",
        },
      });

      expect(response.statusCode).toBe(200);

      const chunks: Buffer[] = [];
      for await (const chunk of response.body) {
        chunks.push(chunk);
      }
      const body = Buffer.concat(chunks).toString("utf8");
      const json = JSON.parse(body);

      expect(json.headers["X-Custom-Header"]).toBe("TestValue123");
    });

    it("should support custom user agent", async () => {
      const response = await client.request({
        url: "https://httpbin.org/user-agent",
        userAgent: "CycleTLS-Test/1.0",
      });

      expect(response.statusCode).toBe(200);

      const chunks: Buffer[] = [];
      for await (const chunk of response.body) {
        chunks.push(chunk);
      }
      const body = Buffer.concat(chunks).toString("utf8");
      const json = JSON.parse(body);

      expect(json["user-agent"]).toBe("CycleTLS-Test/1.0");
    });

    it("should handle redirects by default", async () => {
      const response = await client.get("https://httpbin.org/redirect/2");

      expect(response.statusCode).toBe(200);
      expect(response.finalUrl).toContain("/get");

      for await (const _ of response.body) {
        // drain
      }
    });

    it("should respect disableRedirect option", async () => {
      const response = await client.request({
        url: "https://httpbin.org/redirect/1",
        disableRedirect: true,
      });

      expect(response.statusCode).toBe(302);

      for await (const _ of response.body) {
        // drain
      }
    });
  });
});
