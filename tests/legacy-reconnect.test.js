// Reconnect paths of the legacy client that a real crash cannot reach on demand.
// The Go process is a fake child and the control socket is a local
// WebSocketServer, so each test decides when the socket drops and what the
// "Go process" receives.
const fs = require("fs");
const { EventEmitter } = require("events");
const { WebSocketServer } = require("ws");

jest.mock("child_process", () => ({
  ...jest.requireActual("child_process"),
  spawn: jest.fn(),
}));

const { spawn } = require("child_process");
const { initCycleTLS } = require("../dist/index.js");

jest.setTimeout(30000);

const executablePath = fs.realpathSync(process.execPath);
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// No pid: the client then signals the child through child.kill only.
const makeChild = () => {
  const child = new EventEmitter();
  child.stdout = new EventEmitter();
  child.stderr = new EventEmitter();
  child.pid = undefined;
  child.kill = jest.fn();
  return child;
};

const waitFor = async (predicate, timeoutMs = 5000) => {
  const deadline = Date.now() + timeoutMs;
  while (!predicate() && Date.now() < deadline) await sleep(20);
  if (!predicate()) throw new Error("waitFor timed out");
};

// Frames of the legacy protocol, as the Go process writes them.
const str = (s) => {
  const b = Buffer.from(s);
  const len = Buffer.alloc(2);
  len.writeUInt16BE(b.length);
  return Buffer.concat([len, b]);
};
const u16 = (n) => {
  const b = Buffer.alloc(2);
  b.writeUInt16BE(n);
  return b;
};
const responseFrame = (requestId, status, url) =>
  Buffer.concat([str(requestId), str("response"), u16(status), str(url), u16(0)]);
const endFrame = (requestId) => Buffer.concat([str(requestId), str("end")]);

describe("legacy client: control socket reconnect", () => {
  let wss;
  let received;
  let children;
  let logSpy;
  let errorSpy;
  let killSpy;

  // A stand-in Go server: records each request and answers 200 with no body,
  // except for URLs containing "hang".
  const startServer = (port) => {
    wss = new WebSocketServer({ port });
    wss.on("connection", (socket) => {
      socket.on("message", (raw) => {
        const message = JSON.parse(raw.toString());
        received.push(message);
        if (!message.options || message.options.url.includes("hang")) return;
        socket.send(responseFrame(message.requestId, 200, message.options.url));
        socket.send(endFrame(message.requestId));
      });
    });
    return new Promise((resolve) => wss.on("listening", resolve));
  };
  const stopServer = () =>
    new Promise((resolve) => {
      if (!wss) return resolve();
      for (const socket of wss.clients) socket.terminate();
      wss.close(() => resolve());
      wss = null;
    });

  beforeEach(() => {
    received = [];
    children = [];
    spawn.mockReset();
    spawn.mockImplementation(() => {
      const child = makeChild();
      children.push(child);
      return child;
    });
    killSpy = jest.spyOn(process, "kill").mockImplementation(() => true);
    logSpy = jest.spyOn(console, "log").mockImplementation(() => {});
    errorSpy = jest.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(async () => {
    await stopServer();
    killSpy.mockRestore();
    logSpy.mockRestore();
    errorSpy.mockRestore();
  });

  test("socket close with the process still alive: in-flight rejects, reconnects without a respawn", async () => {
    const port = 9751;
    spawn.mockImplementation(() => {
      const child = makeChild();
      children.push(child);
      if (!wss) startServer(port);
      return child;
    });

    const cycleTLS = await initCycleTLS({ port, timeout: 5000, executablePath });
    try {
      const first = await cycleTLS.get("http://example.test/ok", {});
      expect(first.status).toBe(200);

      const pending = cycleTLS.get("http://example.test/hang", { timeout: -1 }).then(
        () => null,
        (error) => error
      );
      await waitFor(() => received.length === 2);

      // The server drops the connection but keeps listening
      for (const socket of wss.clients) socket.terminate();

      const error = await pending;
      expect(error).toBeInstanceOf(Error);
      expect(error.message).toMatch(/lost the connection to its Go process/);

      await waitFor(() => wss.clients.size === 1);
      await sleep(50);
      const after = await cycleTLS.get("http://example.test/ok", {});
      expect(after.status).toBe(200);
      expect(spawn).toHaveBeenCalledTimes(1);
    } finally {
      await cycleTLS.exit();
    }
  });

  test("gives up after the init timeout, rejects with a clear Error, and a new init starts over", async () => {
    const port = 9752;
    let serve = true;
    spawn.mockImplementation(() => {
      const child = makeChild();
      children.push(child);
      if (serve && !wss) startServer(port);
      return child;
    });

    const cycleTLS = await initCycleTLS({ port, timeout: 600, executablePath });
    await cycleTLS.get("http://example.test/ok", {});

    // The server goes away for good; the fake child never exits, so nothing respawns
    serve = false;
    await stopServer();
    await sleep(100); // let the client see the close

    await expect(cycleTLS.get("http://example.test/ok", {})).rejects.toThrow(/reconnecting to its Go process/);
    await sleep(1000);
    await expect(cycleTLS.get("http://example.test/ok", {})).rejects.toThrow(
      /could not reconnect .*Call exit\(\) and initCycleTLS\(\) again/
    );
    expect(spawn).toHaveBeenCalledTimes(1);

    // A new init on the same port does not hand back the dead instance
    serve = true;
    const fresh = await initCycleTLS({ port, timeout: 5000, executablePath });
    try {
      expect(spawn).toHaveBeenCalledTimes(2);
      const response = await fresh.get("http://example.test/ok", {});
      expect(response.status).toBe(200);

      // exit() on the old client must not tear down the new instance
      await cycleTLS.exit();
      await sleep(100);
      const still = await fresh.get("http://example.test/ok", {});
      expect(still.status).toBe(200);
    } finally {
      await fresh.exit();
    }
  });

  test("a process that exits with no stderr output is restarted, up to the respawn limit", async () => {
    const port = 9753;
    spawn.mockImplementation(() => {
      const child = makeChild();
      children.push(child);
      if (!wss) startServer(port);
      return child;
    });

    const cycleTLS = await initCycleTLS({ port, timeout: 20000, executablePath });
    try {
      await cycleTLS.get("http://example.test/ok", {});

      // Each child dies quietly as soon as it is the current one: 1 start + 3 restarts
      for (let spawned = 1; spawned <= 4; spawned++) {
        await waitFor(() => children.length === spawned, 6000);
        children[spawned - 1].emit("exit", null, "SIGKILL");
        children[spawned - 1].emit("close", null, "SIGKILL");
      }
      await sleep(2500);
      expect(spawn).toHaveBeenCalledTimes(4);

      await expect(cycleTLS.get("http://example.test/ok", {})).rejects.toThrow(/kept crashing after 3 restarts/);
    } finally {
      await cycleTLS.exit();
    }
  });

  test("new 3.0.1 options reach the Go process unchanged", async () => {
    const port = 9754;
    spawn.mockImplementation(() => {
      const child = makeChild();
      children.push(child);
      if (!wss) startServer(port);
      return child;
    });

    const cycleTLS = await initCycleTLS({ port, timeout: 5000, executablePath });
    try {
      const options = {
        tlsProfile: "chrome_133",
        shuffleExtensions: true,
        clientHello: "16030100",
        sessionResumption: true,
        caCerts: ["-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----"],
        certPins: ["sha256/47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU="],
        http2Priority: { weight: 256, exclusive: true, streamDep: 0 },
        proxyHeaders: { "X-Session": "abc" },
        http2Fingerprint: "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p",
        tls13AutoRetry: true,
      };
      await cycleTLS.get("http://example.test/ok", { ...options, readTimeout: 500 });
      const sent = received[0].options;
      for (const [key, value] of Object.entries(options)) {
        expect(sent[key]).toEqual(value);
      }
      // A full profile replaces the default JA3; readTimeout is client-side only
      expect(sent.ja3).toBeUndefined();
      expect(sent).not.toHaveProperty("readTimeout");

      await cycleTLS.get("http://example.test/ok", { http2Priority: false, bodyBytes: new Uint8Array([0, 255, 1]) });
      expect(received[1].options.http2Priority).toBe(false);
      expect(received[1].options.bodyBytes).toBe(Buffer.from([0, 255, 1]).toString("base64"));
      // Without a profile the default JA3 is still added
      expect(typeof received[1].options.ja3).toBe("string");
    } finally {
      await cycleTLS.exit();
    }
  });
});
