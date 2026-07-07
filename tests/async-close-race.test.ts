/**
 * Behavioral tests for HTTP server async close race condition.
 *
 * Instead of inspecting compiled source code for string patterns,
 * these tests verify the actual behavior: that calling close() while
 * requests are in-flight results in a clean shutdown without hanging
 * promises or unhandled errors.
 */

import CycleTLS from "../dist/index.js";

jest.setTimeout(30000);

describe("HTTP server async close race", () => {
  test("close() during in-flight request should not hang", async () => {
    const client = new CycleTLS({
      port: 0,
      autoSpawn: true,
      timeout: 10000,
    });

    // Start a request but don't await it yet
    const requestPromise = client.get("https://httpbin.org/delay/2");

    // Close immediately while request is in-flight
    // This should complete without hanging
    const closePromise = client.close();

    // Both should settle (resolve or reject) within the timeout
    const results = await Promise.allSettled([requestPromise, closePromise]);

    // close() should always resolve cleanly
    expect(results[1].status).toBe("fulfilled");

    // The request may resolve or reject (due to close), but should not hang
    // We don't care about the specific outcome, just that it settled
    expect(["fulfilled", "rejected"]).toContain(results[0].status);
  });

  test("close() after successful request should clean up without errors", async () => {
    const client = new CycleTLS({
      port: 0,
      autoSpawn: true,
      timeout: 10000,
    });

    // Complete a request first
    const response = await client.get("https://httpbin.org/get");
    expect(response.statusCode).toBe(200);

    // Consume body
    for await (const _ of response.body) {
      // drain
    }

    // Close should complete cleanly
    await expect(client.close()).resolves.not.toThrow();
  });

  test("multiple close() calls should not throw or hang", async () => {
    const client = new CycleTLS({
      port: 0,
      autoSpawn: true,
      timeout: 10000,
    });

    // Wait for client to be ready by making a request
    const response = await client.get("https://httpbin.org/get");
    expect(response.statusCode).toBe(200);
    for await (const _ of response.body) {
      // drain
    }

    // Call close() multiple times concurrently - should all resolve
    const closeResults = await Promise.allSettled([
      client.close(),
      client.close(),
      client.close(),
    ]);

    for (const result of closeResults) {
      expect(result.status).toBe("fulfilled");
    }
  });

  test("close() on never-used client should not hang", async () => {
    const client = new CycleTLS({
      port: 0,
      autoSpawn: true,
      timeout: 5000,
    });

    // Close without ever making a request
    // Should complete within a reasonable time
    await expect(client.close()).resolves.not.toThrow();
  });
});
