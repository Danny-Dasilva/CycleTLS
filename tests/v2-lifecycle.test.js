/**
 * Default client: server startup, long runs and shutdown.
 * Local servers only.
 */
const childProcess = require("child_process");
const fs = require("fs");
const os = require("os");
const path = require("path");
const WebSocket = require("ws");
const {
  binaryPath,
  loadPackage,
  startHttpServer,
  sleep,
  isAlive,
  waitForExit,
  forceKill,
} = require("./v2-helpers");

const CycleTLS = loadPackage();
const posixOnly = process.platform === "win32" ? test.skip : test;

jest.setTimeout(120000);

describe("default client lifecycle", () => {
  let server;
  let spawnSpy;
  const clients = [];
  const tempDirs = [];

  /** Go server processes spawned through the client since the spy was set. */
  const serverSpawns = () =>
    spawnSpy.mock.results
      .map((r, i) => ({ child: r.value, args: spawnSpy.mock.calls[i] }))
      .filter(({ args }) => args[0] !== "taskkill");

  const newClient = (options) => {
    const client = new CycleTLS(options);
    clients.push(client);
    return client;
  };

  beforeAll(async () => {
    server = await startHttpServer((req, res) => res.end("ok"));
  });

  afterAll(async () => {
    await server.close();
    for (const dir of tempDirs) {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  beforeEach(() => {
    spawnSpy = jest.spyOn(childProcess, "spawn");
  });

  afterEach(async () => {
    const pids = serverSpawns()
      .map(({ child }) => child && child.pid)
      .filter(Boolean);
    for (const client of clients.splice(0)) {
      await client.close().catch(() => {});
    }
    pids.filter(isAlive).forEach(forceKill);
    spawnSpy.mockRestore();
  });

  test("500 sequential GETs complete (the server's output is drained)", async () => {
    const client = newClient({ port: 39501, timeout: 5000 });
    let completed = 0;
    for (let i = 0; i < 500; i++) {
      const res = await client.get(server.url);
      expect(await res.text()).toBe("ok");
      completed++;
    }
    expect(completed).toBe(500);
  });

  test("10 concurrent first requests spawn exactly one Go process", async () => {
    const client = newClient({ port: 39502, timeout: 10000 });
    const bodies = await Promise.all(
      Array.from({ length: 10 }, () =>
        client.get(server.url).then((r) => r.text())
      )
    );
    expect(bodies).toEqual(Array(10).fill("ok"));

    const spawns = serverSpawns();
    expect(spawns).toHaveLength(1);
    expect(isAlive(spawns[0].child.pid)).toBe(true);
  });

  test("the server is spawned without a shell", async () => {
    const client = newClient({ port: 39503, timeout: 10000 });
    await (await client.get(server.url)).text();

    const [{ args }] = serverSpawns();
    expect(args[0]).toBe(binaryPath());
    expect(args[2].shell).toBe(false);
  });

  test("the child is gone after close()", async () => {
    const client = newClient({ port: 39504, timeout: 10000 });
    await (await client.get(server.url)).text();

    const [{ child }] = serverSpawns();
    expect(isAlive(child.pid)).toBe(true);

    await client.close();
    // No waiting here: close() itself waits for the process to exit.
    expect(isAlive(child.pid)).toBe(false);
  });

  test("the client starts a new server for a request after close()", async () => {
    const client = newClient({ port: 39505, timeout: 10000 });
    await (await client.get(server.url)).text();
    await client.close();

    const res = await client.get(server.url);
    expect(await res.text()).toBe("ok");
    expect(serverSpawns()).toHaveLength(2);
  });

  test("a request after the server was killed starts a new one", async () => {
    const client = newClient({ port: 39506, timeout: 10000 });
    await (await client.get(server.url)).text();

    const [{ child }] = serverSpawns();
    forceKill(child.pid);
    expect(await waitForExit(child.pid)).toBe(true);
    await sleep(50); // let Node deliver the child's 'exit' event

    const res = await client.get(server.url);
    expect(await res.text()).toBe("ok");
    expect(serverSpawns()).toHaveLength(2);
  });

  test("an executable path with a space works", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "cycletls v2 space "));
    tempDirs.push(dir);
    const copy = path.join(dir, path.basename(binaryPath()));
    fs.copyFileSync(binaryPath(), copy);
    fs.chmodSync(copy, 0o755);

    const client = newClient({
      port: 39507,
      timeout: 10000,
      executablePath: copy,
    });
    const res = await client.get(server.url);
    expect(await res.text()).toBe("ok");
    expect(serverSpawns()[0].args[0]).toBe(copy);
  });

  test("one WebSocket connection per request once the server is known", async () => {
    // A stand-in server that only counts connections.
    let connections = 0;
    const wss = new WebSocket.Server({ port: 39508, host: "127.0.0.1" });
    await new Promise((r) => wss.on("listening", r));
    wss.on("connection", (socket) => {
      connections++;
      socket.on("message", () => socket.close());
    });

    try {
      const client = newClient({ port: 39508, timeout: 2000 });
      await client.get(server.url).catch(() => {});
      const afterFirst = connections; // one probe + one request
      await client.get(server.url).catch(() => {});
      await client.get(server.url).catch(() => {});

      expect(afterFirst).toBe(2);
      expect(connections - afterFirst).toBe(2);
      expect(serverSpawns()).toHaveLength(0);
    } finally {
      for (const socket of wss.clients) socket.terminate();
      await new Promise((r) => wss.close(r));
    }
  });

  posixOnly("a server that needs more than 5 s to start is waited for", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "cycletls-v2-slow-"));
    tempDirs.push(dir);
    const script = path.join(dir, "slow.sh");
    fs.writeFileSync(script, `#!/bin/sh\nsleep 6\nexec "${binaryPath()}"\n`, {
      mode: 0o755,
    });

    const client = newClient({
      port: 39509,
      timeout: 10000,
      executablePath: script,
    });
    const started = Date.now();
    const res = await client.get(server.url);
    expect(await res.text()).toBe("ok");
    expect(Date.now() - started).toBeGreaterThanOrEqual(5500);
  });

  posixOnly("startupTimeout bounds the wait and names itself in the error", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "cycletls-v2-never-"));
    tempDirs.push(dir);
    const script = path.join(dir, "never.sh");
    fs.writeFileSync(script, "#!/bin/sh\nexec sleep 30\n", { mode: 0o755 });

    const client = newClient({
      port: 39510,
      timeout: 10000,
      startupTimeout: 600,
      executablePath: script,
    });
    const started = Date.now();
    await expect(client.get(server.url)).rejects.toThrow(/startupTimeout/);
    expect(Date.now() - started).toBeLessThan(4000);

    // The process that never came up is not left behind.
    const [{ child }] = serverSpawns();
    expect(await waitForExit(child.pid)).toBe(true);
  });

  posixOnly("a server that exits at startup fails fast with its output", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "cycletls-v2-dead-"));
    tempDirs.push(dir);
    const script = path.join(dir, "dead.sh");
    fs.writeFileSync(script, "#!/bin/sh\necho 'cannot bind port' >&2\nexit 3\n", {
      mode: 0o755,
    });

    const client = newClient({
      port: 39511,
      timeout: 10000,
      executablePath: script,
    });
    const started = Date.now();
    await expect(client.get(server.url)).rejects.toThrow(
      /exited during startup \(code 3.*cannot bind port/s
    );
    expect(Date.now() - started).toBeLessThan(4000);
  });

  describe("port: 0", () => {
    const portOf = ({ args }) => Number(args[2].env.WS_PORT);

    test("picks a free port, starts one server on it and uses it", async () => {
      const client = newClient({ port: 0, timeout: 10000 });
      const bodies = await Promise.all(
        Array.from({ length: 5 }, () => client.get(server.url).then((r) => r.text()))
      );
      expect(bodies).toEqual(Array(5).fill("ok"));
      await (await client.get(server.url)).text();

      const spawns = serverSpawns();
      expect(spawns).toHaveLength(1);
      expect(portOf(spawns[0])).toBeGreaterThan(0);
    });

    test("two clients get a server each, and close() leaves none behind", async () => {
      const a = newClient({ port: 0, timeout: 10000 });
      const b = newClient({ port: 0, timeout: 10000 });
      await Promise.all([a.get(server.url), b.get(server.url)].map((p) => p.then((r) => r.text())));

      const spawns = serverSpawns();
      expect(spawns).toHaveLength(2);
      expect(portOf(spawns[0])).not.toBe(portOf(spawns[1]));

      // Closing one must not disturb the other.
      await a.close();
      expect(await (await b.get(server.url)).text()).toBe("ok");
      await b.close();
      expect(spawns.map(({ child }) => isAlive(child.pid))).toEqual([false, false]);
    });

    test("is refused with autoSpawn: false, where there is no server to place", () => {
      expect(() => new CycleTLS({ port: 0, autoSpawn: false })).toThrow(/port: 0/);
    });

    test.each([-1, 65536, 1.5, "abc"])("port %p is refused", (port) => {
      expect(() => new CycleTLS({ port })).toThrow(/Invalid port/);
    });
  });

  test("a missing executable rejects and does not crash", async () => {
    const client = newClient({
      port: 39512,
      executablePath: path.join(os.tmpdir(), "cycletls-v2-does-not-exist"),
    });
    await expect(client.get(server.url)).rejects.toThrow(/Executable not found/);
  });
});
