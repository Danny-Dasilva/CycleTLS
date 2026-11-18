/**
 * Child process for v2-exit-cleanup.test.js. Not a test suite.
 *
 * usage: node v2-fixture-child.js <dist> <module> <port> <mode>
 *   module: "index" (the package entry point) or "client" (flow-control-client)
 *   mode:   "exit"     make one request, then process.exit(0) without close()
 *           "throw"    make one request, then die on an uncaught exception
 *           "idle"     make one request, then stay alive on a timer
 *           "listener" as "idle", with the application's own SIGINT listener
 *
 * Prints one JSON line {"serverPid": n} once the request has completed.
 */
const path = require("path");
const http = require("http");
const childProcess = require("child_process");

const [dist, moduleName, port, mode] = process.argv.slice(2);

// Record the Go server's pid.
const realSpawn = childProcess.spawn;
let serverPid = null;
childProcess.spawn = function (...args) {
  const child = realSpawn.apply(this, args);
  if (serverPid === null) serverPid = child.pid;
  return child;
};

const loaded = require(
  path.join(dist, moduleName === "index" ? "index.js" : "flow-control-client.js")
);
const CycleTLS = loaded.CycleTLS;

const server = http.createServer((req, res) => res.end("ok"));
server.listen(0, "127.0.0.1", async () => {
  const client = new CycleTLS({ port: Number(port), timeout: 10000 });
  const res = await client.get(`http://127.0.0.1:${server.address().port}/`);
  await res.text();
  process.stdout.write(JSON.stringify({ serverPid }) + "\n");

  if (mode === "exit") {
    process.exit(0);
  }
  if (mode === "throw") {
    setTimeout(() => {
      throw new Error("fixture: uncaught on purpose");
    }, 10);
    return;
  }
  if (mode === "listener") {
    process.on("SIGINT", () => {
      process.stdout.write("user-listener\n");
      setTimeout(() => process.exit(42), 300);
    });
  }
  setInterval(() => {}, 1000);
});
