import http from "http";
import zlib from "zlib";
import crypto from "crypto";
import { withCycleTLS } from "./test-utils.js";

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

const contentEncodingOf = (headers: { [key: string]: any }) =>
  Object.keys(headers).filter((name) => name.toLowerCase() === "content-encoding");

describe("Response decompression (issue #414)", () => {
  let server: http.Server;
  let base: string;

  beforeAll((done) => {
    server = http.createServer((req, res) => {
      const path = (req.url || "").split("?")[0];
      const match = cases.find((c) => c.path === path);
      if (match) {
        res.writeHead(200, { "Content-Type": "application/json", ...match.headers });
        res.end(req.method === "HEAD" ? undefined : match.body);
        return;
      }
      if (path === "/identity") {
        res.writeHead(200, { "Content-Type": "application/json" });
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
    server.listen(0, () => {
      base = `http://127.0.0.1:${(server.address() as any).port}`;
      done();
    });
  });

  afterAll((done) => {
    server.close(done);
  });

  test("json(), text() and data are decoded for every supported encoding", async () => {
    await withCycleTLS({ port: 9414 }, async (cycleTLS) => {
      for (const c of cases) {
        const response = await cycleTLS(`${base}${c.path}`, { headers: requestHeaders }, "get");

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual(c.expected);
        expect(await response.text()).toBe(JSON.stringify(c.expected));
        expect(response.data).toEqual(c.expected);
        // The body is no longer encoded, so the header must not say it is.
        expect(contentEncodingOf(response.headers)).toEqual([]);
      }
    });
  });

  test("decodes when the caller sets no Accept-Encoding header", async () => {
    await withCycleTLS({ port: 9415 }, async (cycleTLS) => {
      const response = await cycleTLS(`${base}/gzip`, {}, "get");
      expect(await response.json()).toEqual(payload);
    });
  });

  test("decodes a gzip body that arrives in several chunks", async () => {
    await withCycleTLS({ port: 9416 }, async (cycleTLS) => {
      const response = await cycleTLS(`${base}/gzip-chunked`, { headers: requestHeaders }, "get");
      expect(await response.json()).toEqual(bigPayload);

      const streamed = await cycleTLS(
        `${base}/gzip-chunked`,
        { headers: requestHeaders, responseType: "stream" },
        "get"
      );
      const chunks: Buffer[] = [];
      for await (const chunk of streamed.data) chunks.push(chunk as Buffer);
      expect(JSON.parse(Buffer.concat(chunks).toString("utf8"))).toEqual(bigPayload);
    });
  });

  test("leaves an unencoded body and a HEAD response alone", async () => {
    await withCycleTLS({ port: 9417 }, async (cycleTLS) => {
      const plain = await cycleTLS(`${base}/identity`, { headers: requestHeaders }, "get");
      expect(await plain.json()).toEqual(payload);

      // HEAD has no body to decode, so its headers pass through as sent.
      const head = await cycleTLS(`${base}/gzip`, { headers: requestHeaders }, "head");
      expect(head.status).toBe(200);
      expect(contentEncodingOf(head.headers)).toHaveLength(1);
    });
  });
});
