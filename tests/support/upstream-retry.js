// Test-only wrapper around the built CycleTLS client.
//
// jest.config.js maps every `../dist/index.js` import in tests/ to this file,
// so each test gets a client that treats rate limiting on shared public hosts
// as "upstream unavailable" instead of a product failure:
//
//   - a request to a host in UPSTREAM_HOSTS that comes back 429, 495 or 5xx is
//     retried with backoff;
//   - if every attempt is refused, the client throws UpstreamUnavailableError,
//     which tests/support/jest.setup.js turns into a logged skip.
//
// Requests to any other host (localhost echo servers, badssl, tls.peet.ws)
// pass straight through, so their assertions are untouched.
//
// It also moves each Jest worker onto its own client ports; see workerPort.

const realInitCycleTLS = require("../../dist/index.js");

// Public hosts that rate-limit CI runners. httpbin.org answers 429 when
// several workflow runs share an egress address.
const UPSTREAM_HOSTS = new Set(["httpbin.org", "www.httpbin.org"]);

const BACKOFFS_MS = [1000, 2000, 4000];
const MAX_ATTEMPTS = BACKOFFS_MS.length + 1;

const REQUEST_METHODS = [
  "head",
  "get",
  "post",
  "put",
  "delete",
  "trace",
  "options",
  "connect",
  "patch",
];

class UpstreamUnavailableError extends Error {
  constructor(message) {
    super(message);
    this.name = "UpstreamUnavailableError";
  }
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// 429 = rate limited, 495 = CycleTLS's status for a failed connection or TLS
// handshake, 5xx = upstream fault.
const isFlakeStatus = (status) =>
  status === 429 || status === 495 || (status >= 500 && status < 600);

const isGuardedUrl = (url) => {
  let parsed;
  try {
    parsed = new URL(url);
  } catch (e) {
    return false;
  }
  if (!UPSTREAM_HOSTS.has(parsed.hostname)) return false;
  // httpbin's /status/<code> returns the requested code on purpose.
  return !parsed.pathname.startsWith("/status/");
};

// A stream body (form-data) is consumed by the first attempt and cannot be
// sent again.
const isReplayable = (options) =>
  !(options && options.body && typeof options.body.pipe === "function");

const requestWithRetry = async (send, url, options, label, wait = sleep) => {
  if (typeof url !== "string" || !isGuardedUrl(url)) {
    return send(options);
  }

  const attempts = isReplayable(options) ? MAX_ATTEMPTS : 1;
  let status;
  for (let attempt = 1; attempt <= attempts; attempt++) {
    // The client mutates the options object, so each attempt gets its own copy.
    const response = await send(options ? { ...options } : options);
    status = response && response.status;
    if (!isFlakeStatus(status)) return response;

    if (response.data && typeof response.data.destroy === "function") {
      response.data.destroy();
    }
    if (attempt < attempts) {
      await wait(BACKOFFS_MS[attempt - 1]);
    }
  }

  throw new UpstreamUnavailableError(
    `${label} ${url} returned ${status} on ${attempts} attempt(s)`
  );
};

const wrapClient = (client) => {
  const wrapped = (url, options, method) =>
    requestWithRetry(
      (opts) => client(url, opts, method),
      url,
      options,
      (method || "get").toUpperCase()
    );

  // Copy every property (ws, sse, exit, ...) and then replace the HTTP verbs.
  Object.assign(wrapped, client);
  for (const method of REQUEST_METHODS) {
    if (typeof client[method] !== "function") continue;
    wrapped[method] = (url, options) =>
      requestWithRetry(
        (opts) => client[method](url, opts),
        url,
        options,
        method.toUpperCase()
      );
  }
  return wrapped;
};

// Jest runs test files in parallel worker processes, and many files ask for
// the same client port (several use the default 9119). Two workers on one port
// share a single Go process: the second connects to the first one's server,
// and its requests hang when the first file finishes and stops that process.
// Give every worker its own block of 1000 ports so files cannot collide.
const DEFAULT_PORT = 9119;
const workerPort = (port) => {
  const worker = Number(process.env.JEST_WORKER_ID || 1);
  return 10000 + (worker - 1) * 1000 + ((port || DEFAULT_PORT) % 1000);
};

const initCycleTLS = async (initOptions = {}) =>
  wrapClient(
    await realInitCycleTLS({ ...initOptions, port: workerPort(initOptions.port) })
  );

module.exports = initCycleTLS;
module.exports.default = initCycleTLS;
module.exports.__esModule = true;
module.exports.UpstreamUnavailableError = UpstreamUnavailableError;
module.exports.requestWithRetry = requestWithRetry;
module.exports.MAX_ATTEMPTS = MAX_ATTEMPTS;
