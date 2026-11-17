import http from "http";
import zlib from "zlib";
import crypto from "crypto";
import CycleTLS from "../dist/index.js";
import { createSuiteInstance, streamToBuffer } from "./test-utils.js";

jest.setTimeout(30000);

// Regression test for issue #414: a response with a Content-Encoding header
// reached the Node client still compressed, so json() threw and text()
// returned binary. The Go side now decodes the body as it streams it.
// All responses come from a local server, so the test needs no network.

const payload = { ok: true, message: "hello from cycletls", filler: "x".repeat(40000) };
const raw = Buffer.from(JSON.stringify(payload));

// zstd frame of {"ok":true,"message":"hello from cycletls"}. Stored as a
// fixture because zlib only gained zstd in Node 22.15.
const zstdPayload = { ok: true, message: "hello from cycletls" };
const zstdBody = Buffer.from(
  "KLUv/SArWQEAeyJvayI6dHJ1ZSwibWVzc2FnZSI6ImhlbGxvIGZyb20gY3ljbGV0bHMifQ==",
  "base64"
);

// Random data does not compress, so this body spans many 8 KB read chunks.
const bigPayload = { ok: true, noise: crypto.randomBytes(60000).toString("hex") };
const bigGzip = zlib.gzipSync(Buffer.from(JSON.stringify(bigPayload)));

interface Case {
  path: string;
  headers: { [key: string]: string };
  body: Buffer;
  expected: any;
}

const cases: Case[] = [
  { path: "/gzip", headers: { "Content-Encoding": "gzip" }, body: zlib.gzipSync(raw), expected: payload },
  { path: "/deflate", headers: { "Content-Encoding": "deflate" }, body: zlib.deflateSync(raw), expected: payload },
  { path: "/deflate-raw", headers: { "Content-Encoding": "deflate" }, body: zlib.deflateRawSync(raw), expected: payload },
  { path: "/br", headers: { "Content-Encoding": "br" }, body: zlib.brotliCompressSync(raw), expected: payload },
  { path: "/zstd", headers: { "Content-Encoding": "zstd" }, body: zstdBody, expected: zstdPayload },
  { path: "/mixed-case", headers: { "content-ENCODING": "GZip" }, body: zlib.gzipSync(raw), expected: payload },
  {
    path: "/gzip-then-br",
    headers: { "Content-Encoding": "gzip, br" },
    body: zlib.brotliCompressSync(zlib.gzipSync(raw)),
    expected: payload,
  },
];

const requestHeaders = { "Accept-Encoding": "gzip, deflate, br, zstd" };

const headerNamed = (headers: { [key: string]: any }, wanted: string) =>
  Object.keys(headers).filter((name) => name.toLowerCase() === wanted);

describe("Response decompression (issue #414)", () => {
  let client: CycleTLS;
  let cleanup: () => Promise<void>;
  let server: http.Server;
  let base: string;

  beforeAll(async () => {
    ({ instance: client, cleanup } = await createSuiteInstance({ port: 9414 }));

    server = http.createServer((req, res) => {
      const path = (req.url || "").split("?")[0];
      const match = cases.find((c) => c.path === path);
      if (match) {
        res.writeHead(200, {
          "Content-Type": "application/json",
          "Content-Length": String(match.body.length),
          ...match.headers,
        });
        res.end(req.method === "HEAD" ? undefined : match.body);
        return;
      }
      if (path === "/identity") {
        res.writeHead(200, { "Content-Type": "application/json", "Content-Length": String(raw.length) });
        res.end(raw);
        return;
      }
      if (path === "/gzip-chunked") {
        // No Content-Length: the body goes out in five separate writes.
        res.writeHead(200, { "Content-Type": "application/json", "Content-Encoding": "gzip" });
        const parts = 5;
        const step = Math.ceil(bigGzip.length / parts);
        let i = 0;
        const writeNext = () => {
          res.write(bigGzip.subarray(i * step, (i + 1) * step));
          i++;
          if (i < parts) setTimeout(writeNext, 20);
          else res.end();
        };
        writeNext();
        return;
      }
      res.writeHead(404);
      res.end();
    });

    await new Promise<void>((resolve) => {
      server.listen(0, "127.0.0.1", () => {
        base = `http://127.0.0.1:${(server.address() as any).port}`;
        resolve();
      });
    });
  });

  afterAll(async () => {
    await cleanup();
    if (server) {
      await new Promise<void>((resolve) => server.close(() => resolve()));
    }
  });

  test("json() and text() are decoded for every supported encoding", async () => {
    for (const c of cases) {
      const response = await client.get(`${base}${c.path}`, { headers: requestHeaders });

      expect(response.status).toBe(200);
      expect(await response.json()).toEqual(c.expected);
      // The body is no longer encoded, so the headers must not say it is, and
      // the compressed length no longer describes it.
      expect(headerNamed(response.headers, "content-encoding")).toEqual([]);
      expect(headerNamed(response.headers, "content-length")).toEqual([]);

      // A body can be read once, so text() needs a request of its own.
      const again = await client.get(`${base}${c.path}`, { headers: requestHeaders });
      expect(await again.text()).toBe(JSON.stringify(c.expected));
    }
  });

  test("decodes when the caller sets no Accept-Encoding header", async () => {
    const response = await client.get(`${base}/gzip`);
    expect(await response.json()).toEqual(payload);
  });

  test("decodes a gzip body that arrives in several chunks", async () => {
    const response = await client.get(`${base}/gzip-chunked`, { headers: requestHeaders });
    expect(await response.json()).toEqual(bigPayload);

    // The same body read from the stream.
    const streamed = await client.get(`${base}/gzip-chunked`, { headers: requestHeaders });
    const body = await streamToBuffer(streamed.body);
    expect(JSON.parse(body.toString("utf8"))).toEqual(bigPayload);
  });

  test("leaves an unencoded body and a HEAD response alone", async () => {
    const plain = await client.get(`${base}/identity`, { headers: requestHeaders });
    expect(await plain.json()).toEqual(payload);
    expect(headerNamed(plain.headers, "content-length")).toHaveLength(1);

    // HEAD has no body to decode, so its headers pass through as sent.
    const head = await client.head(`${base}/gzip`, { headers: requestHeaders });
    expect(head.status).toBe(200);
    expect(headerNamed(head.headers, "content-encoding")).toHaveLength(1);
  });
});
