/**
 * Default client: request bodies over 64 KB (the "init2" packet) and the
 * upper bound. Local servers only.
 */
const crypto = require("crypto");
const {
  loadPackage,
  loadProtocol,
  startHttpServer,
  parseInitPacket,
} = require("./v2-helpers");

const CycleTLS = loadPackage();
const Protocol = loadProtocol();
const MAX = 32 * 1024 * 1024;

jest.setTimeout(120000);

describe("init packet layout", () => {
  test("a request that fits in 65535 bytes uses the original init packet", () => {
    const packet = Protocol.buildInitPacket("req-1", { url: "http://x/", body: "a".repeat(60000) }, 65536);
    const parsed = parseInitPacket(packet);
    expect(parsed.method).toBe("init");
    expect(parsed.requestId).toBe("req-1");
    expect(parsed.initialWindow).toBe(65536);
    expect(parsed.trailing).toBe(0);
    expect(parsed.options.body).toHaveLength(60000);
  });

  test("a larger request uses init2 with a 32-bit options length", () => {
    const body = "b".repeat(100000);
    const packet = Protocol.buildInitPacket("req-2", { url: "http://x/", body }, 65536);
    const parsed = parseInitPacket(packet);
    expect(parsed.method).toBe("init2");
    expect(parsed.optionsLength).toBeGreaterThan(65535);
    expect(parsed.trailing).toBe(0);
    expect(parsed.options.body).toBe(body);
  });

  test("the switch happens exactly above 65535 bytes of options JSON", () => {
    const overhead = Buffer.byteLength(JSON.stringify({ body: "" }));
    const fits = Protocol.buildInitPacket("r", { body: "c".repeat(65535 - overhead) }, 65536);
    const tooBig = Protocol.buildInitPacket("r", { body: "c".repeat(65536 - overhead) }, 65536);
    expect(parseInitPacket(fits)).toMatchObject({ method: "init", optionsLength: 65535 });
    expect(parseInitPacket(tooBig)).toMatchObject({ method: "init2", optionsLength: 65536 });
  });

  test("the limit is 32 MiB and is exported", () => {
    expect(Protocol.MAX_INIT_OPTIONS_BYTES).toBe(MAX);
  });
});

describe("default client with large request bodies", () => {
  let echo;
  let client;

  beforeAll(async () => {
    // Echoes the request body byte for byte.
    echo = await startHttpServer((req, res, body) => {
      res.setHeader("content-type", "application/octet-stream");
      res.setHeader("x-received-bytes", String(body.length));
      res.end(body);
    });
    client = new CycleTLS({ port: 39521, timeout: 30000 });
  });

  afterAll(async () => {
    await client.close();
    await echo.close();
  });

  test("a 100 KB POST works (it crashed Node before)", async () => {
    const body = "a".repeat(100000);
    const res = await client.post(echo.url, body);
    expect(res.status).toBe(200);
    expect(res.headers["X-Received-Bytes"] || res.headers["x-received-bytes"]).toEqual(["100000"]);
    expect(await res.text()).toBe(body);
  });

  test("a 1 MB text body round-trips byte-exact", async () => {
    // Mixed widths, so UTF-8 length and string length differ.
    const unit = "abc-éß-日本-🙂-\"\\\n\t|";
    const body = unit.repeat(Math.ceil((1 << 20) / Buffer.byteLength(unit)));
    const sent = Buffer.from(body, "utf8");
    expect(sent.length).toBeGreaterThanOrEqual(1 << 20);

    const res = await client.post(echo.url, body);
    expect(res.status).toBe(200);
    const received = await res.buffer();
    expect(received.length).toBe(sent.length);
    expect(received.equals(sent)).toBe(true);
  });

  test("a 1 MB binary body (bodyBytes) round-trips byte-exact", async () => {
    const sent = crypto.randomBytes(1 << 20);
    const res = await client.request({
      url: echo.url,
      method: "POST",
      bodyBytes: new Uint8Array(sent.buffer, sent.byteOffset, sent.byteLength),
    });
    expect(res.status).toBe(200);
    const received = await res.buffer();
    expect(received.length).toBe(sent.length);
    expect(received.equals(sent)).toBe(true);
  });

  test("an 8 MB body round-trips byte-exact", async () => {
    const sent = crypto.randomBytes(8 << 20);
    const res = await client.request({ url: echo.url, method: "PUT", bodyBytes: sent });
    expect(res.status).toBe(200);
    const received = await res.buffer();
    expect(received.equals(sent)).toBe(true);
  });

  describe("over the limit", () => {
    const uncaught = [];
    const onUncaught = (err) => uncaught.push(err);

    beforeAll(() => {
      process.on("uncaughtException", onUncaught);
      process.on("unhandledRejection", onUncaught);
    });
    afterAll(() => {
      process.off("uncaughtException", onUncaught);
      process.off("unhandledRejection", onUncaught);
    });

    test("a body over 32 MiB rejects with a clear error", async () => {
      const before = echo.state.requests;
      const body = "a".repeat(MAX + 1);

      const err = await client.post(echo.url, body).then(
        () => null,
        (e) => e
      );
      expect(err).toBeInstanceOf(CycleTLS.CycleTLSError);
      expect(err.message).toMatch(/Request too large/);
      expect(err.message).toMatch(/33554432 bytes \(32 MiB\)/);
      expect(err.statusCode).toBe(413);
      expect(echo.state.requests).toBe(before); // nothing was sent
    });

    test("binary data whose base64 form is over the limit rejects", async () => {
      const bytes = new Uint8Array(25 * 1024 * 1024 + 3); // 33.3 MiB as base64
      await expect(
        client.request({ url: echo.url, method: "POST", bodyBytes: bytes })
      ).rejects.toThrow(/Request too large/);
    });

    test("a body just under the limit that the JSON wrapper pushes over rejects", async () => {
      const body = "a".repeat(MAX - 8);
      await expect(client.post(echo.url, body)).rejects.toMatchObject({
        name: "CycleTLSError",
        statusCode: 413,
      });
    });

    test("options that cannot be encoded reject and are not thrown", async () => {
      const headers = {};
      headers.self = headers; // circular: JSON.stringify throws
      await expect(client.request({ url: echo.url, headers })).rejects.toThrow(
        /Failed to encode request/
      );
    });

    test("the process did not crash and the client still works", async () => {
      await new Promise((r) => setImmediate(r));
      expect(uncaught).toEqual([]);
      const res = await client.post(echo.url, "still here");
      expect(await res.text()).toBe("still here");
    });
  });
});
