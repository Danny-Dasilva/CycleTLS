/**
 * Default client: the `timeout` option. Negative means no timeout, as it does
 * in the Go server; 0 means the default. Local servers only.
 */
const { loadPackage, loadClientModule, startHttpServer } = require("./v2-helpers");

const CycleTLS = loadPackage();

jest.setTimeout(60000);

describe("normalizeTimeout", () => {
  const { normalizeTimeout } = loadClientModule();

  test.each([
    [undefined, 30000, 30000],
    [null, 30000, 30000],
    [NaN, 30000, 30000],
    [undefined, null, null],
    [-1, 30000, null],
    [-5000, 30000, null],
    [0, 1234, 30000],
    [1500, 30000, 1500],
    [Infinity, 30000, null],
    [2147483648, 30000, null],
    [2147483647, 30000, 2147483647],
  ])("normalizeTimeout(%p, %p) is %p", (value, fallback, expected) => {
    expect(normalizeTimeout(value, fallback)).toBe(expected);
  });
});

describe("default client timeout", () => {
  let slow;
  let client;
  const DELAY = 1500;

  beforeAll(async () => {
    slow = await startHttpServer((req, res) => {
      setTimeout(() => res.end("slow ok"), DELAY);
    });
    client = new CycleTLS({ port: 39531, timeout: 10000 });
  });

  afterAll(async () => {
    await client.close();
    await slow.close();
  });

  test("timeout: -1 on a slow endpoint succeeds", async () => {
    const started = Date.now();
    const res = await client.get(slow.url, { timeout: -1 });
    expect(res.status).toBe(200);
    expect(await res.text()).toBe("slow ok");
    expect(Date.now() - started).toBeGreaterThanOrEqual(DELAY - 100);
  });

  test("timeout: 0 means the default, not an immediate failure", async () => {
    const res = await client.get(slow.url, { timeout: 0 });
    expect(await res.text()).toBe("slow ok");
  });

  test("timeout: Infinity means no timeout", async () => {
    const res = await client.get(slow.url, { timeout: Infinity });
    expect(await res.text()).toBe("slow ok");
  });

  test("a positive timeout shorter than the response still rejects", async () => {
    const started = Date.now();
    await expect(client.get(slow.url, { timeout: 300 })).rejects.toMatchObject({
      name: "CycleTLSError",
      statusCode: 408,
    });
    expect(Date.now() - started).toBeLessThan(DELAY);
  });

  test("a client-level timeout of -1 applies to its requests", async () => {
    const noTimeout = new CycleTLS({ port: 39531, timeout: -1 });
    try {
      const res = await noTimeout.get(slow.url);
      expect(await res.text()).toBe("slow ok");
    } finally {
      await noTimeout.close();
    }
  });
});
