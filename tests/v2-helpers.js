/**
 * Shared helpers for the v2-*.test.js suites (default, flow-control client).
 *
 * Everything here is local: plain http servers on 127.0.0.1 and the Go binary
 * from dist/. Set CYCLETLS_DIST to run the suites against another build.
 */
const path = require("path");
const http = require("http");
const os = require("os");

const DIST = process.env.CYCLETLS_DIST
  ? path.resolve(process.env.CYCLETLS_DIST)
  : path.join(__dirname, "..", "dist");

const PLATFORM_BINARIES = {
  win32: { x64: "index.exe" },
  linux: { arm: "index-arm", arm64: "index-arm64", x64: "index" },
  darwin: { x64: "index-mac", arm: "index-mac-arm", arm64: "index-mac-arm64" },
  freebsd: { x64: "index-freebsd" },
};

/** Path of the Go binary the client would pick on this machine. */
function binaryPath() {
  const name = (PLATFORM_BINARIES[process.platform] || {})[os.arch()];
  if (!name) {
    throw new Error(`no binary for ${process.platform}/${os.arch()}`);
  }
  return path.join(DIST, name);
}

/** The package entry point, as `require('cycletls')` returns it. */
function loadPackage() {
  return require(path.join(DIST, "index.js"));
}

/** The flow-control client module on its own. */
function loadClientModule() {
  return require(path.join(DIST, "flow-control-client.js"));
}

function loadProtocol() {
  return require(path.join(DIST, "protocol.js"));
}

/**
 * Start a local HTTP server. `handler(req, res, body)` is called with the
 * whole request body. Resolves with { url, port, close, requests }.
 */
function startHttpServer(handler) {
  return new Promise((resolve, reject) => {
    const state = { requests: 0 };
    const server = http.createServer((req, res) => {
      state.requests++;
      const chunks = [];
      req.on("data", (c) => chunks.push(c));
      req.on("end", () => handler(req, res, Buffer.concat(chunks)));
    });
    server.on("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      resolve({
        url: `http://127.0.0.1:${port}/`,
        port,
        state,
        close: () =>
          new Promise((r) => {
            if (server.closeAllConnections) server.closeAllConnections();
            server.close(() => r());
          }),
      });
    });
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/** True while a process with this pid exists. */
function isAlive(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch (err) {
    return err.code === "EPERM";
  }
}

/** Wait until the pid is gone. Resolves true if it went within the time. */
async function waitForExit(pid, timeoutMs = 5000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (!isAlive(pid)) return true;
    await sleep(25);
  }
  return !isAlive(pid);
}

/** Best-effort kill, for cleaning up after a failed assertion. */
function forceKill(pid) {
  if (!pid) return;
  try {
    process.kill(-pid, "SIGKILL");
  } catch {
    // not a group leader, or already gone
  }
  try {
    process.kill(pid, "SIGKILL");
  } catch {
    // already gone
  }
}

/**
 * Split an init packet into its fields. Understands both layouts:
 * "init" (u16 options length) and "init2" (u32 options length).
 */
function parseInitPacket(buf) {
  let o = 0;
  const idLen = buf.readUInt16BE(o);
  o += 2;
  const requestId = buf.toString("utf8", o, o + idLen);
  o += idLen;
  const mLen = buf.readUInt16BE(o);
  o += 2;
  const method = buf.toString("utf8", o, o + mLen);
  o += mLen;
  const initialWindow = buf.readUInt32BE(o);
  o += 4;
  let jsonLen;
  if (method === "init2") {
    jsonLen = buf.readUInt32BE(o);
    o += 4;
  } else {
    jsonLen = buf.readUInt16BE(o);
    o += 2;
  }
  const json = buf.subarray(o, o + jsonLen);
  return {
    requestId,
    method,
    initialWindow,
    optionsLength: jsonLen,
    trailing: buf.length - (o + jsonLen),
    options: JSON.parse(json.toString("utf8")),
  };
}

module.exports = {
  DIST,
  binaryPath,
  loadPackage,
  loadClientModule,
  loadProtocol,
  startHttpServer,
  sleep,
  isAlive,
  waitForExit,
  forceKill,
  parseInitPacket,
};
