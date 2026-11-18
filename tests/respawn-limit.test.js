// Regression tests for https://github.com/Danny-Dasilva/CycleTLS/issues/428
// The Go process is replaced by a fake child so each test controls what it
// writes to stderr and can count how many times it was spawned.
const { EventEmitter } = require("events");
const { WebSocketServer } = require("ws");

jest.mock("child_process", () => ({
  ...jest.requireActual("child_process"),
  spawn: jest.fn(),
}));

const { spawn } = require("child_process");
// Load the built client by absolute path. jest.config.js maps "../dist/index.js"
// to a test wrapper that moves each Jest worker onto its own ports; these tests
// assert on the exact port, so they need the unwrapped client.
const initCycleTLS = require(require("path").join(__dirname, "..", "dist", "index.js"));

jest.setTimeout(30000);

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const makeChild = () => {
  const child = new EventEmitter();
  child.stdout = new EventEmitter();
  child.stderr = new EventEmitter();
  child.pid = 999999;
  child.kill = jest.fn();
  return child;
};

// Every spawned child writes `line` to stderr right after it starts.
const spawnFailingWith = (line) => {
  spawn.mockImplementation(() => {
    const child = makeChild();
    setImmediate(() => child.stderr.emit("data", Buffer.from(line)));
    return child;
  });
};

const waitForSpawnCount = async (count, timeoutMs) => {
  const deadline = Date.now() + timeoutMs;
  while (spawn.mock.calls.length < count && Date.now() < deadline) {
    await sleep(25);
  }
};

describe("Go process respawn limits", () => {
  let killSpy;
  let logSpy;
  let errorSpy;

  beforeEach(() => {
    spawn.mockReset();
    // The fake pid must never reach a real process group.
    killSpy = jest.spyOn(process, "kill").mockImplementation(() => true);
    logSpy = jest.spyOn(console, "log").mockImplementation(() => {});
    errorSpy = jest.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    killSpy.mockRestore();
    logSpy.mockRestore();
    errorSpy.mockRestore();
  });

  test("does not respawn on 'address already in use' and reports the port", async () => {
    const port = 9731;
    spawnFailingWith(`2026/10/01 01:35:46 listen tcp :${port}: bind: address already in use\n`);

    await expect(
      initCycleTLS({ port, timeout: 1000, executablePath: process.execPath })
    ).rejects.toThrow(`Port ${port} is already in use`);

    expect(spawn).toHaveBeenCalledTimes(1);
    await sleep(1000);
    expect(spawn).toHaveBeenCalledTimes(1);
  });

  test("gives up after 3 restarts when the process keeps crashing", async () => {
    spawnFailingWith("panic: boom\n");

    const startedAt = Date.now();
    await expect(
      initCycleTLS({ port: 9732, timeout: 20000, executablePath: process.execPath })
    ).rejects.toThrow(/kept crashing after 3 restarts/);

    // 1 start + 3 restarts, spaced 500ms, 1000ms and 2000ms apart
    expect(spawn).toHaveBeenCalledTimes(4);
    expect(Date.now() - startedAt).toBeGreaterThanOrEqual(3400);

    await sleep(1000);
    expect(spawn).toHaveBeenCalledTimes(4);
  });

  test("does not respawn after initialization has timed out", async () => {
    spawnFailingWith("panic: boom\n");

    await expect(
      initCycleTLS({ port: 9733, timeout: 200, executablePath: process.execPath })
    ).rejects.toThrow(/Could not connect to the CycleTLS instance within 200ms/);

    // The first restart was due at 500ms; it must not happen.
    await sleep(1500);
    expect(spawn).toHaveBeenCalledTimes(1);
  });

  test("a crash after a long healthy run gets a fresh set of restarts", async () => {
    const port = 9734;
    const children = [];
    let wss;
    spawn.mockImplementation(() => {
      const child = makeChild();
      children.push(child);
      // The first child "binds" the port so initialization succeeds.
      if (!wss) wss = new WebSocketServer({ port });
      return child;
    });
    const crash = () => children[children.length - 1].stderr.emit("data", Buffer.from("panic: boom\n"));

    const cycleTLS = await initCycleTLS({ port, timeout: 5000, executablePath: process.execPath });
    const nowSpy = jest.spyOn(Date, "now");
    try {
      // Use up all three restarts with quick crashes.
      for (let restarts = 1; restarts <= 3; restarts++) {
        crash();
        await waitForSpawnCount(1 + restarts, 5000);
        expect(spawn).toHaveBeenCalledTimes(1 + restarts);
      }

      // The fourth child stays up for over 30 seconds, then crashes.
      const realNow = Date.now();
      nowSpy.mockImplementation(() => realNow + 31000);
      crash();
      nowSpy.mockRestore();

      await waitForSpawnCount(5, 2000);
      expect(spawn).toHaveBeenCalledTimes(5);
    } finally {
      nowSpy.mockRestore();
      await cycleTLS.exit();
      await new Promise((resolve) => wss.close(resolve));
    }
  });
});
