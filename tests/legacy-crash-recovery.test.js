// The legacy client (initCycleTLS) must survive its Go process dying after
// initialization: in-flight requests reject, the process is restarted through
// the bounded respawn logic, and later requests succeed.
// Uses the real binary and kills it. Local servers only.
const http = require("http");

// Pass every spawn through to the real one, but keep the child objects so the
// tests can find the Go process without guessing at pids.
jest.mock("child_process", () => {
  const actual = jest.requireActual("child_process");
  return { ...actual, spawn: jest.fn((...args) => actual.spawn(...args)) };
});

const { spawn } = require("child_process");
const { initCycleTLS } = require("../dist/index.js");

jest.setTimeout(60000);

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const isWindows = process.platform === "win32";

// The Go children this file started (not taskkill and the like).
const goChildren = () =>
  spawn.mock.results.map((r) => r.value).filter((c) => c && c.spawnfile && !/taskkill/i.test(c.spawnfile));

// Kill the newest Go process the hard way, as the OOM killer would.
const killGoProcess = () => {
  const children = goChildren();
  const child = children[children.length - 1];
  if (isWindows) {
    child.kill();
  } else {
    // The child is its own process group leader (detached), so this also
    // covers a shell that did not exec the binary.
    process.kill(-child.pid, "SIGKILL");
  }
  return child;
};

// Retry until the restarted process answers. Requests made while it is away
// reject at once, so this loop is cheap.
const getUntilOk = async (cycleTLS, url, deadlineMs = 15000) => {
  const deadline = Date.now() + deadlineMs;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const response = await cycleTLS.get(url, { timeout: 5 });
      if (response.status === 200) return response;
      lastError = new Error(`status ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await sleep(100);
  }
  throw lastError;
};

describe("legacy client: Go process exit after init", () => {
  let server;
  let base;
  let logSpy;
  let errorSpy;
  const openResponses = new Set();

  beforeAll((done) => {
    server = http.createServer((req, res) => {
      if (req.url === "/hang") {
        // Never answers
        openResponses.add(res);
        return;
      }
      if (req.url === "/stall-body") {
        // Headers and one chunk, then nothing
        openResponses.add(res);
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.write("first chunk");
        return;
      }
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end('{"ok":true}');
    });
    server.listen(0, "127.0.0.1", () => {
      base = `http://127.0.0.1:${server.address().port}`;
      done();
    });
  });

  afterAll((done) => {
    for (const res of openResponses) res.destroy();
    server.closeAllConnections?.();
    server.close(() => done());
  });

  beforeEach(() => {
    spawn.mockClear();
    logSpy = jest.spyOn(console, "log").mockImplementation(() => {});
    errorSpy = jest.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    logSpy.mockRestore();
    errorSpy.mockRestore();
  });

  test("in-flight requests reject with an Error and a later request succeeds", async () => {
    const cycleTLS = await initCycleTLS({ port: 9741 });
    try {
      const before = await cycleTLS.get(`${base}/`, {});
      expect(before.status).toBe(200);
      expect(goChildren()).toHaveLength(1);

      // Two requests waiting for headers; neither has a timeout that could
      // reject it for us.
      const startedAt = Date.now();
      const inFlight = [
        cycleTLS.get(`${base}/hang`, { timeout: -1 }),
        cycleTLS.get(`${base}/hang`, { timeout: 120 }),
      ];
      const settled = Promise.allSettled(inFlight);
      await sleep(300);

      killGoProcess();

      const results = await settled;
      expect(Date.now() - startedAt).toBeLessThan(10000);
      for (const result of results) {
        expect(result.status).toBe("rejected");
        expect(result.reason).toBeInstanceOf(Error);
        expect(result.reason.message).toMatch(/Go process/);
      }

      const after = await getUntilOk(cycleTLS, `${base}/`);
      expect(after.status).toBe(200);
      // Exactly one restart for one crash
      expect(goChildren()).toHaveLength(2);

      // And it keeps working
      const again = await cycleTLS.get(`${base}/`, {});
      expect(again.status).toBe(200);
    } finally {
      await cycleTLS.exit();
    }
  });

  test("a request made while the process is away rejects at once, not after its timeout", async () => {
    const cycleTLS = await initCycleTLS({ port: 9742 });
    try {
      await cycleTLS.get(`${base}/`, {});
      const child = killGoProcess();
      // Wait until Node has seen the exit; the restart is 500ms away.
      await new Promise((resolve) => (child.exitCode !== null || child.signalCode ? resolve() : child.once("exit", resolve)));
      await sleep(300);

      const startedAt = Date.now();
      await expect(cycleTLS.get(`${base}/`, { timeout: 30 })).rejects.toThrow(/reconnecting to its Go process/);
      expect(Date.now() - startedAt).toBeLessThan(1000);

      const after = await getUntilOk(cycleTLS, `${base}/`);
      expect(after.status).toBe(200);
    } finally {
      await cycleTLS.exit();
    }
  });

  test("a body that is still streaming fails instead of ending early", async () => {
    const cycleTLS = await initCycleTLS({ port: 9743 });
    try {
      // Buffered response: the promise is still pending while the body stalls
      const buffered = cycleTLS.get(`${base}/stall-body`, { timeout: 5, responseType: "text" });
      const bufferedResult = buffered.then(
        (r) => ({ ok: true, r }),
        (error) => ({ ok: false, error })
      );

      // Stream response: the caller already holds the stream
      const streamed = await cycleTLS.get(`${base}/stall-body`, { timeout: 5, responseType: "stream" });
      expect(streamed.status).toBe(200);
      const streamError = new Promise((resolve) => streamed.data.once("error", resolve));
      streamed.data.resume();

      await sleep(300);
      killGoProcess();

      const result = await bufferedResult;
      expect(result.ok).toBe(false);
      expect(result.error).toBeInstanceOf(Error);
      expect(result.error.message).toMatch(/Go process/);

      const error = await streamError;
      expect(error).toBeInstanceOf(Error);
      expect(error.message).toMatch(/Go process/);

      const after = await getUntilOk(cycleTLS, `${base}/`);
      expect(after.status).toBe(200);
    } finally {
      await cycleTLS.exit();
    }
  });

  test("no restart after exit()", async () => {
    const cycleTLS = await initCycleTLS({ port: 9744 });
    await cycleTLS.get(`${base}/`, {});
    expect(goChildren()).toHaveLength(1);
    const child = goChildren()[0];

    // A request still waiting when exit() is called settles with an Error
    const pending = cycleTLS.get(`${base}/hang`, { timeout: 120 }).then(
      () => null,
      (error) => error
    );
    await sleep(200);
    await cycleTLS.exit();

    const error = await pending;
    expect(error).toBeInstanceOf(Error);
    expect(error.message).toMatch(/exited before the request finished|closed before the request finished/);

    // exit() kills the process; that exit must not be treated as a crash
    await new Promise((resolve) => (child.exitCode !== null || child.signalCode ? resolve() : child.once("exit", resolve)));
    await sleep(1500);
    expect(goChildren()).toHaveLength(1);
  });
});
