import CycleTLS, { CycleTLSError } from "../dist/index.js";
import http from "http";
import net from "net";
import { withCycleTLS } from "./test-utils.js";

jest.setTimeout(30000);

/**
 * Read Timeout Tests for V2 API
 *
 * V2 API supports two timeout options:
 * - `timeout`: Connection timeout (until headers arrive)
 * - `readTimeout`: Body streaming timeout (resets on each chunk received)
 *
 * The `readTimeout` option prevents streams from hanging indefinitely if the
 * server stalls mid-body during transfer.
 */
// FIXME(v3.0.0): v3 CycleTLS spawn races on CI runners. Skipping until fixed.
describe.skip("Read timeout handling", () => {
  let server: http.Server;
  let serverPort: number;
  const sockets = new Set<net.Socket>();

  beforeAll((done) => {
    // Create a test server that delays the response body
    server = http.createServer((req, res) => {
      if (req.url === "/slow-headers") {
        // Don't send headers for a while - this WILL trigger timeout
        const timer = setTimeout(() => {
          res.writeHead(200, { "Content-Type": "text/plain" });
          res.end("Delayed headers response");
        }, 5000);
        const clearTimer = () => clearTimeout(timer);
        req.on("close", clearTimer);
        res.on("close", clearTimer);
      } else if (req.url === "/slow-body") {
        // Send headers immediately
        res.writeHead(200, { "Content-Type": "text/plain" });

        // Start sending body but then delay indefinitely
        res.write("Start of response...");

        // Never end the response - V2 does NOT timeout during body read
        // The connection stays open but no more data is sent
      } else if (req.url === "/quick-response") {
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.end("Quick response");
      } else {
        res.writeHead(404);
        res.end("Not found");
      }
    });

    server.on("connection", (socket) => {
      sockets.add(socket);
      socket.unref();
      socket.on("close", () => sockets.delete(socket));
    });

    server.listen(0, () => {
      serverPort = (server.address() as any).port;
      server.unref();
      done();
    });
  });

  afterAll((done) => {
    for (const socket of sockets) {
      socket.destroy();
    }
    server.close(done);
  });

  test("Should timeout when headers are delayed (connection timeout)", async () => {
    // V2 API: timeout applies until response headers are received
    await withCycleTLS({ port: 9117, timeout: 1000 }, async (client) => {
      // This endpoint delays headers for 5 seconds - should timeout at 1 second
      await expect(
        client.request({
          url: `http://localhost:${serverPort}/slow-headers`,
        })
      ).rejects.toThrow(/timeout/i);
    });
  });

  test("Without readTimeout: response resolves once headers arrive, body can hang", async () => {
    // Without readTimeout option, the connection timeout clears when headers arrive
    // The body stream could hang forever if the server stalls
    await withCycleTLS({ port: 9118, timeout: 1000 }, async (client) => {
      // This endpoint sends headers immediately, so the response resolves
      // even though the body never completes
      const response = await client.request({
        url: `http://localhost:${serverPort}/slow-body`,
      });

      // Response should be received (headers came quickly)
      expect(response.status).toBe(200);
      expect(response.body).toBeDefined();

      // Without readTimeout, the body stream would hang forever if consumed
      // Clean up: destroy the stream to prevent hanging
      response.body.destroy();
    });
  });

  test("With readTimeout: should timeout when body streaming stalls", async () => {
    // With readTimeout option, the stream will timeout if no data arrives
    await withCycleTLS({ port: 9120, timeout: 5000 }, async (client) => {
      const response = await client.request({
        url: `http://localhost:${serverPort}/slow-body`,
        readTimeout: 500, // 500ms read timeout - will fire since body stalls
      });

      // Response headers arrive immediately
      expect(response.status).toBe(200);

      // Now try to consume the body - should timeout after 500ms
      const chunks: Buffer[] = [];
      let readTimeoutError: Error | null = null;

      try {
        for await (const chunk of response.body) {
          chunks.push(chunk as Buffer);
        }
      } catch (err) {
        readTimeoutError = err as Error;
      }

      // Should have received at least the initial chunk
      expect(chunks.length).toBeGreaterThan(0);
      const receivedText = Buffer.concat(chunks).toString();
      expect(receivedText).toContain("Start of response");

      // Should have received a read timeout error
      expect(readTimeoutError).not.toBeNull();
      expect(readTimeoutError!.message).toMatch(/read timeout/i);
    });
  });

  test("Normal requests should work within timeout", async () => {
    await withCycleTLS({ port: 9119, timeout: 5000 }, async (client) => {
      const response = await client.request({
        url: `http://localhost:${serverPort}/quick-response`,
      });

      expect(response.status).toBe(200);
      const text = await response.text();
      expect(text).toBe("Quick response");
    });
  });

  test("Per-request timeout should override client-level timeout", async () => {
    // Client has 10 second timeout, but per-request timeout of 500ms should apply
    await withCycleTLS({ port: 9121, timeout: 10000 }, async (client) => {
      const startTime = Date.now();

      // Request with short per-request timeout (500ms) should timeout quickly
      // even though client timeout is 10 seconds
      await expect(
        client.request({
          url: `http://localhost:${serverPort}/slow-headers`,
          timeout: 500, // This should override the 10s client timeout
        })
      ).rejects.toThrow(/timeout/i);

      const elapsed = Date.now() - startTime;
      // Should timeout around 500ms, not 10000ms (allow some margin for execution)
      expect(elapsed).toBeLessThan(2000);
    });
  });

  test("Per-request timeout can be longer than client timeout", async () => {
    // Client has 500ms timeout, but per-request can override to be longer
    await withCycleTLS({ port: 9122, timeout: 500 }, async (client) => {
      // /quick-response should complete in <500ms, so this should work
      // even with a very short client timeout, because per-request overrides
      const response = await client.request({
        url: `http://localhost:${serverPort}/quick-response`,
        timeout: 5000, // Override to 5 seconds
      });

      expect(response.status).toBe(200);
      const text = await response.text();
      expect(text).toBe("Quick response");
    });
  });
});
