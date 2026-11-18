/**
 * Default client: the Go server must not outlive the Node process, and
 * SIGINT/SIGTERM must still end a process that has no listener of its own.
 *
 * Each case runs tests/v2-fixture-child.js in a child Node process and
 * watches both that process and the Go server it spawned. Local only.
 */
const { spawn } = require("child_process");
const path = require("path");
const { DIST, sleep, isAlive, waitForExit, forceKill } = require("./v2-helpers");

const FIXTURE = path.join(__dirname, "v2-fixture-child.js");
const posixOnly = process.platform === "win32" ? describe.skip : describe;

jest.setTimeout(60000);

/** Start the fixture and wait for it to report the Go server's pid. */
function startFixture(moduleName, port, mode) {
  const child = spawn(process.execPath, [FIXTURE, DIST, moduleName, String(port), mode], {
    stdio: ["ignore", "pipe", "pipe"],
  });
  const run = { child, stdout: "", stderr: "", exit: null, serverPid: null };
  child.stdout.on("data", (d) => (run.stdout += d));
  child.stderr.on("data", (d) => (run.stderr += d));
  run.exited = new Promise((resolve) => {
    child.on("exit", (code, signal) => {
      run.exit = { code, signal };
      resolve(run.exit);
    });
  });
  run.ready = (async () => {
    const deadline = Date.now() + 30000;
    while (Date.now() < deadline) {
      const line = run.stdout.split("\n").find((l) => l.startsWith("{"));
      if (line) {
        run.serverPid = JSON.parse(line).serverPid;
        return run;
      }
      if (run.exit) break;
      await sleep(25);
    }
    throw new Error(`fixture did not start: ${run.stderr || run.stdout}`);
  })();
  return run;
}

/** Wait for the fixture to exit, up to a limit. Resolves null if it is still running. */
function exitWithin(run, ms) {
  return Promise.race([run.exited, sleep(ms).then(() => null)]);
}

describe("Go server cleanup when Node ends", () => {
  const runs = [];
  const start = (...args) => {
    const run = startFixture(...args);
    runs.push(run);
    return run.ready;
  };

  afterEach(() => {
    for (const run of runs.splice(0)) {
      if (!run.exit) run.child.kill("SIGKILL");
      forceKill(run.serverPid);
    }
  });

  test("process.exit() without close() kills the server", async () => {
    const run = await start("index", 39551, "exit");
    expect(await run.exited).toEqual({ code: 0, signal: null });
    expect(run.serverPid).toBeGreaterThan(0);
    expect(await waitForExit(run.serverPid)).toBe(true);
  });

  test("an uncaught exception kills the server", async () => {
    const run = await start("index", 39552, "throw");
    const exit = await run.exited;
    expect(exit.code).not.toBe(0);
    expect(run.stderr).toMatch(/uncaught on purpose/);
    expect(await waitForExit(run.serverPid)).toBe(true);
  });

  posixOnly("signals", () => {
    test.each(["SIGINT", "SIGTERM"])(
      "%s ends a process that has no listener of its own, and kills the server",
      async (signal) => {
        // The client module alone, so the only signal listeners are its own.
        const run = await start("client", signal === "SIGINT" ? 39553 : 39554, "idle");
        expect(isAlive(run.serverPid)).toBe(true);

        run.child.kill(signal);
        const exit = await exitWithin(run, 5000);

        // Ended by the signal itself, the way it would without the client.
        expect(exit).toEqual({ code: null, signal });
        expect(await waitForExit(run.serverPid)).toBe(true);
      }
    );

    test("SIGINT with the application's own listener: server killed, the application decides", async () => {
      const run = await start("client", 39555, "listener");

      run.child.kill("SIGINT");
      const exit = await exitWithin(run, 5000);

      expect(run.stdout).toMatch(/user-listener/);
      // The fixture's listener exits with 42; the client did not end the process first.
      expect(exit).toEqual({ code: 42, signal: null });
      expect(await waitForExit(run.serverPid)).toBe(true);
    });

    test.each(["SIGINT", "SIGTERM"])(
      "%s through the package entry point kills the server",
      async (signal) => {
        const run = await start("index", signal === "SIGINT" ? 39556 : 39557, "idle");
        expect(isAlive(run.serverPid)).toBe(true);

        run.child.kill(signal);
        expect(await waitForExit(run.serverPid)).toBe(true);
      }
    );
  });
});
