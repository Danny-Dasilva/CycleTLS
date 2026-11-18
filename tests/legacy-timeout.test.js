// Timeouts of the legacy client: a negative `timeout` means no timeout, and the
// opt-in `readTimeout` bounds a body that stalls after the headers.
// Real binary, local server only.
const http = require("http");
const { initCycleTLS } = require("../dist/index.js");

jest.setTimeout(30000);

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

describe("legacy client: timeout and readTimeout", () => {
  let server;
  let base;
  let cycleTLS;
  const open = new Set();

  beforeAll(async () => {
    server = http.createServer((req, res) => {
      open.add(res);
      res.on("close", () => open.delete(res));
      if (req.url === "/slow-headers") {
        setTimeout(() => {
          res.writeHead(200, { "Content-Type": "text/plain" });
          res.end("late");
        }, 1500);
      } else if (req.url === "/stall-body") {
        // Headers and one chunk, then silence
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.write("first chunk");
      } else if (req.url === "/drip") {
        // A chunk every 150ms for about a second
        res.writeHead(200, { "Content-Type": "text/plain" });
        let sent = 0;
        const timer = setInterval(() => {
          res.write("x");
          if (++sent === 7) {
            clearInterval(timer);
            res.end();
          }
        }, 150);
        res.on("close", () => clearInterval(timer));
      } else {
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.end("ok");
      }
    });
    await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
    base = `http://127.0.0.1:${server.address().port}`;
    cycleTLS = await initCycleTLS({ port: 9746 });
  });

  afterAll(async () => {
    await cycleTLS.exit();
    for (const res of open) res.destroy();
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(resolve));
  });

  test("timeout: -1 is not rejected at once", async () => {
    const response = await cycleTLS.get(`${base}/`, { timeout: -1, responseType: "text" });
    expect(response.status).toBe(200);
    expect(response.data).toBe("ok");
  });

  test("timeout: -1 waits for headers that a 1 second timeout gives up on", async () => {
    await expect(cycleTLS.get(`${base}/slow-headers`, { timeout: 1 })).rejects.toThrow(/Request timeout after 1000ms/);
    const response = await cycleTLS.get(`${base}/slow-headers`, { timeout: -1, responseType: "text" });
    expect(response.status).toBe(200);
    expect(response.data).toBe("late");
  });

  test("without readTimeout a stalled body is not bounded by timeout", async () => {
    let settled = false;
    const request = cycleTLS.get(`${base}/stall-body`, { timeout: 1, responseType: "text" });
    request.then(
      () => (settled = true),
      () => (settled = true)
    );
    // Three times the request timeout later it is still waiting for the body
    await sleep(3000);
    expect(settled).toBe(false);
  });

  test("readTimeout rejects a buffered request whose body stalls", async () => {
    const startedAt = Date.now();
    await expect(
      cycleTLS.get(`${base}/stall-body`, { timeout: 5, readTimeout: 400, responseType: "text" })
    ).rejects.toThrow(/Read timeout: no response data for 400ms/);
    expect(Date.now() - startedAt).toBeLessThan(3000);
  });

  test("readTimeout errors a stream response whose body stalls", async () => {
    const response = await cycleTLS.get(`${base}/stall-body`, { timeout: 5, readTimeout: 400, responseType: "stream" });
    expect(response.status).toBe(200);
    const chunks = [];
    const error = await new Promise((resolve) => {
      response.data.on("data", (chunk) => chunks.push(chunk));
      response.data.once("error", resolve);
    });
    expect(Buffer.concat(chunks).toString()).toBe("first chunk");
    expect(error.message).toMatch(/Read timeout/);
  });

  test("readTimeout measures idle time: a slow but steady body completes", async () => {
    // The whole body takes about 1050ms, longer than readTimeout, but no gap is
    const response = await cycleTLS.get(`${base}/drip`, { timeout: 5, readTimeout: 600, responseType: "text" });
    expect(response.status).toBe(200);
    expect(response.data).toBe("xxxxxxx");
  });

  test("the client still works after read timeouts", async () => {
    const response = await cycleTLS.get(`${base}/`, { timeout: 5, responseType: "text" });
    expect(response.status).toBe(200);
  });
});
