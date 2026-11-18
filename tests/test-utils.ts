import CycleTLS from "../dist/index.js";
import { Readable } from "stream";
import https from "https";

export { CycleTLS };

// Track all active instances for emergency cleanup
const activeInstances = new Set<CycleTLS>();

export interface CycleTLSOptions {
  port?: number;
  timeout?: number;
  debug?: boolean;
  executablePath?: string;
  autoSpawn?: boolean;
  initialWindow?: number;
  creditThreshold?: number;
}

/**
 * Helper to consume a Readable stream into a Buffer
 */
export async function streamToBuffer(stream: Readable): Promise<Buffer> {
  const chunks: Buffer[] = [];
  for await (const chunk of stream) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
  }
  return Buffer.concat(chunks);
}

/**
 * Helper to consume a Readable stream into a string
 */
export async function streamToText(stream: Readable): Promise<string> {
  const buffer = await streamToBuffer(stream);
  return buffer.toString("utf8");
}

/**
 * Helper to consume a Readable stream and parse as JSON
 */
export async function streamToJson<T = unknown>(stream: Readable): Promise<T> {
  const text = await streamToText(stream);
  return JSON.parse(text) as T;
}

/**
 * Wrapper function that ensures CycleTLS instance cleanup even if test fails.
 * Use this for individual test cases that need a single CycleTLS instance.
 *
 * @param portOrOptions - Port number or full options object
 * @param testFn - Test function that receives the CycleTLS instance
 * @returns Promise that resolves with test function result
 *
 * @example
 * test("Should handle timeout", async () => {
 *   await withCycleTLS(9117, async (client) => {
 *     const response = await client.get('https://example.com');
 *     expect(response.status).toBe(200);
 *   });
 * });
 */
export async function withCycleTLS<T>(
  portOrOptions: number | CycleTLSOptions,
  testFn: (client: CycleTLS) => Promise<T>
): Promise<T> {
  const options = typeof portOrOptions === 'number'
    ? { port: portOrOptions }
    : portOrOptions;

  const client = new CycleTLS(options);
  activeInstances.add(client);

  try {
    return await testFn(client);
  } finally {
    activeInstances.delete(client);
    await client.close();
  }
}

/**
 * Alternative approach for tests that need multiple CycleTLS instances
 * or want more control over instance lifecycle.
 *
 * @param options - CycleTLS initialization options
 * @returns CycleTLS instance that's tracked for cleanup
 *
 * @example
 * test("Multiple instances", async () => {
 *   const client1 = createSafeCycleTLS({ port: 9001 });
 *   const client2 = createSafeCycleTLS({ port: 9002 });
 *
 *   try {
 *     // Test logic here
 *   } finally {
 *     await cleanupCycleTLS(client1);
 *     await cleanupCycleTLS(client2);
 *   }
 * });
 */
export function createSafeCycleTLS(
  options?: CycleTLSOptions
): CycleTLS {
  const client = new CycleTLS(options);
  activeInstances.add(client);
  return client;
}

/**
 * Manual cleanup function for instances created with createSafeCycleTLS
 *
 * @param instance - CycleTLS instance to cleanup
 */
export async function cleanupCycleTLS(instance: CycleTLS): Promise<void> {
  if (activeInstances.has(instance)) {
    activeInstances.delete(instance);
    await instance.close();
  }
}

/**
 * Wrapper for test suites that share a single CycleTLS instance
 * Use this with Jest's beforeAll/afterAll hooks
 *
 * @param options - CycleTLS initialization options
 * @returns Object with instance and cleanup function
 *
 * @example
 * describe("Test Suite", () => {
 *   let client: CycleTLS;
 *   let cleanup: () => Promise<void>;
 *
 *   beforeAll(() => {
 *     ({ instance: client, cleanup } = createSuiteInstance({ port: 9001 }));
 *   });
 *
 *   afterAll(async () => {
 *     await cleanup();
 *   });
 *
 *   test("test 1", async () => {
 *     // Use client here
 *   });
 * });
 */
export function createSuiteInstance(
  options?: CycleTLSOptions
): { instance: CycleTLS; cleanup: () => Promise<void> } {
  const instance = createSafeCycleTLS(options);

  const cleanup = async () => {
    await cleanupCycleTLS(instance);
  };

  return { instance, cleanup };
}

/**
 * Get count of active CycleTLS instances (useful for debugging)
 */
export function getActiveInstanceCount(): number {
  return activeInstances.size;
}

/**
 * Status codes from upstream treated as transient flake (rate limit, gateway
 * timeout, Cloudflare origin errors). When httpbin.org or tlsfingerprint.com are
 * being squeezed these surface now and then and should not fail the suite.
 */
export const UPSTREAM_FLAKE_STATUSES: ReadonlySet<number> = new Set([408, 421, 429, 502, 503, 504, 521, 522, 523, 524, 525]);

/**
 * Probe an upstream URL with a short timeout. Resolves true when the probe gets
 * a 200 within `timeoutMs`. Used in beforeAll to gate a whole suite when the
 * upstream is down.
 */
export function probeUpstream(url: string, timeoutMs = 8000): Promise<boolean> {
  return new Promise((resolve) => {
    let resolved = false;
    const finish = (ok: boolean) => {
      if (resolved) return;
      resolved = true;
      resolve(ok);
    };
    const t = setTimeout(() => finish(false), timeoutMs);
    const req = https.get(url, { timeout: timeoutMs }, (res) => {
      const ok = res.statusCode === 200;
      res.on('data', () => {});
      res.on('end', () => { clearTimeout(t); finish(ok); });
      res.on('error', () => { clearTimeout(t); finish(false); });
    });
    req.on('error', () => { clearTimeout(t); finish(false); });
    req.on('timeout', () => { req.destroy(); clearTimeout(t); finish(false); });
  });
}

export interface UpstreamState {
  serviceAvailable: boolean;
  upstreamUnreachable: boolean;
}

/**
 * Build a `conditionalTest` helper for a suite. The returned function takes
 * (name, fn) like Jest's `test`, but:
 *  - If `state.serviceAvailable` is false (pre-suite probe failed), the test
 *    body logs "Skipped" and returns.
 *  - Each test body races against `deadlineMs` (default 30s). On a deadline hit
 *    it sets `state.upstreamUnreachable` so later tests skip at once.
 *  - Flake statuses and network errors become a skip and trip the breaker.
 *    Real assertion failures still throw.
 *
 * Use:
 *   const state = { serviceAvailable: false, upstreamUnreachable: false };
 *   const conditionalTest = makeConditionalTest(state, 30000);
 *   beforeAll(async () => { state.serviceAvailable = await probeUpstream('https://httpbin.org/json'); });
 *   conditionalTest('should...', async () => { ... });
 */
export function makeConditionalTest(state: UpstreamState, deadlineMs = 30000) {
  return function conditionalTest(name: string, fn: () => Promise<unknown>): void {
    test(name, async () => {
      if (!state.serviceAvailable || state.upstreamUnreachable) {
        console.log(`Skipped: ${name} (upstream unavailable)`);
        return;
      }
      const deadline = new Promise<'timeout'>((res) => setTimeout(() => res('timeout'), deadlineMs));
      try {
        const result = await Promise.race([fn().then(() => 'ok' as const), deadline]);
        if (result === 'timeout') {
          state.upstreamUnreachable = true;
          console.log(`Skipped: ${name} (upstream hung past ${deadlineMs}ms; tripping circuit breaker)`);
          return;
        }
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        const isFlakeStatus = [...UPSTREAM_FLAKE_STATUSES].some((c) => msg.includes(`${c}`) && (msg.includes('statusCode') || msg.includes('status')));
        const isNetworkErr = /timeout|timed out|ECONNRESET|ECONNREFUSED|EAI_AGAIN|ENOTFOUND|socket hang up|aborted/i.test(msg);
        if (isFlakeStatus || isNetworkErr) {
          state.upstreamUnreachable = true;
          console.log(`Skipped: ${name} (upstream flake; tripping circuit breaker: ${msg.slice(0, 200)})`);
          return;
        }
        throw e;
      }
    });
  };
}

/**
 * Retry a request-producing function on upstream-flake statuses (see UPSTREAM_FLAKE_STATUSES).
 * The fn is expected to return an object that has a `.status` (number) field — both
 * the legacy `.do()` shape and the modern `.get/.request()` Response work, since
 * `Response.status` is a getter.
 *
 * Returns the *last* response so the caller can still inspect the final status (and
 * choose to skip the test if all retries flake out).
 */
export async function withUpstreamRetry<T extends { status: number }>(
  fn: () => Promise<T>,
  attempts = 3
): Promise<T> {
  // Short backoff (500ms, 1s, 2s) — keeps total retry budget under ~3.5s so we
  // don't blow the Jest test timeout in CI when an upstream is flaking hard.
  const backoffsMs = [500, 1000, 2000];
  let last!: T;
  for (let i = 0; i < attempts; i++) {
    try {
      last = await fn();
      if (last.status >= 200 && last.status < 400) return last;
      if (!UPSTREAM_FLAKE_STATUSES.has(last.status)) return last;
      // Flake — retry after backoff
    } catch (e) {
      // Network error counts as flake; retry
      if (i === attempts - 1) throw e;
    }
    if (i < backoffsMs.length) {
      await new Promise<void>((res) => setTimeout(res, backoffsMs[i]));
    }
  }
  return last;
}

/**
 * Emergency cleanup of all active instances
 * This is called automatically on process exit
 */
async function cleanupAll(): Promise<void> {
  if (activeInstances.size > 0) {
    console.warn(`Warning: Cleaning up ${activeInstances.size} orphaned CycleTLS instances`);
    const instances = [...activeInstances];
    await Promise.all(instances.map(async (instance) => {
      try {
        await instance.close();
      } catch (error) {
        console.error('Error cleaning up CycleTLS instance:', error);
      }
    }));
    activeInstances.clear();
  }
}

// Global cleanup hooks to prevent process hanging
// These will catch instances that weren't properly cleaned up
process.on('beforeExit', async () => {
  await cleanupAll();
});

process.on('SIGTERM', async () => {
  await cleanupAll();
  process.exit(0);
});

process.on('SIGINT', async () => {
  await cleanupAll();
  process.exit(0);
});

// Jest global cleanup hook
if (typeof afterAll !== 'undefined') {
  afterAll(async () => {
    await cleanupAll();
  });
}
