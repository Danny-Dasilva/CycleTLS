const {
  requestWithRetry,
  UpstreamUnavailableError,
  MAX_ATTEMPTS,
} = require("./support/upstream-retry.js");

// Covers the test harness itself: the retry wrapper every test client goes
// through (tests/support/upstream-retry.js) and the skip guard installed by
// tests/support/jest.setup.js. No network access.
describe("Upstream retry wrapper", () => {
  const noWait = () => Promise.resolve();
  const sender = (statuses) => {
    const calls = [];
    const send = async (options) => {
      calls.push(options);
      return { status: statuses[Math.min(calls.length - 1, statuses.length - 1)] };
    };
    return { send, calls };
  };

  test("returns the first response when the upstream answers normally", async () => {
    const { send, calls } = sender([200]);
    const response = await requestWithRetry(send, "https://httpbin.org/get", {}, "GET", noWait);
    expect(response.status).toBe(200);
    expect(calls.length).toBe(1);
  });

  test("retries 429 and returns the later success", async () => {
    const { send, calls } = sender([429, 503, 200]);
    const response = await requestWithRetry(send, "https://httpbin.org/get", {}, "GET", noWait);
    expect(response.status).toBe(200);
    expect(calls.length).toBe(3);
  });

  test("gives each attempt its own copy of the options", async () => {
    const { send, calls } = sender([429, 200]);
    const options = { body: "payload" };
    await requestWithRetry(send, "https://httpbin.org/post", options, "POST", noWait);
    expect(calls[0]).not.toBe(options);
    expect(calls[1]).not.toBe(calls[0]);
    expect(calls[1]).toEqual({ body: "payload" });
  });

  test("throws UpstreamUnavailableError when every attempt is refused", async () => {
    const { send, calls } = sender([429]);
    await expect(
      requestWithRetry(send, "https://httpbin.org/get", {}, "GET", noWait)
    ).rejects.toBeInstanceOf(UpstreamUnavailableError);
    expect(calls.length).toBe(MAX_ATTEMPTS);
  });

  test("does not retry statuses that are a real answer", async () => {
    for (const status of [404, 408, 301]) {
      const { send, calls } = sender([status]);
      const response = await requestWithRetry(send, "https://httpbin.org/get", {}, "GET", noWait);
      expect(response.status).toBe(status);
      expect(calls.length).toBe(1);
    }
  });

  test("passes other hosts through untouched, whatever the status", async () => {
    for (const url of [
      "http://127.0.0.1:8080/post",
      "https://expired.badssl.com",
      "https://httpbin.org/status/503",
    ]) {
      const { send, calls } = sender([495]);
      const options = {};
      const response = await requestWithRetry(send, url, options, "GET", noWait);
      expect(response.status).toBe(495);
      expect(calls.length).toBe(1);
    }
  });

  test("does not resend a stream body", async () => {
    const { send, calls } = sender([429]);
    const options = { body: { pipe: () => {} } };
    await expect(
      requestWithRetry(send, "https://httpbin.org/post", options, "POST", noWait)
    ).rejects.toBeInstanceOf(UpstreamUnavailableError);
    expect(calls.length).toBe(1);
  });

  test("a test that hits an unavailable upstream is skipped, not failed", async () => {
    // jest.setup.js must swallow this error; without the guard this test fails.
    throw new UpstreamUnavailableError("simulated outage (expected in this test)");
  });
});
