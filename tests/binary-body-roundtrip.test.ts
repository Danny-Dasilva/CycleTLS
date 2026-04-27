import initCycleTLS from "../dist/index.js";
import http from "http";
import crypto from "crypto";
import { withCycleTLS } from "./test-utils.js";

jest.setTimeout(30000);

// Regression test for binary body corruption (PR #389 follow-up).
// Previously, bytes > 0x7F passed in `options.body` were UTF-8 mangled when
// traversing the TS → WebSocket → Go transport: `JSON.stringify` emits them
// as \uXXXX escapes and Go's decoder stores them as runes that re-emit as
// multi-byte UTF-8. Fix: route Buffer/Uint8Array/ArrayBuffer through
// base64-encoded `bodyBytes` so the Go server sees exact bytes.

describe("Binary request body round-trip", () => {
  let server: http.Server;
  let serverPort: number;
  let received: Buffer = Buffer.alloc(0);

  beforeAll((done) => {
    server = http.createServer((req, res) => {
      const chunks: Buffer[] = [];
      req.on("data", (c: Buffer) => chunks.push(c));
      req.on("end", () => {
        received = Buffer.concat(chunks);
        res.writeHead(200, { "Content-Type": "application/octet-stream" });
        res.end(received);
      });
    });
    server.listen(0, () => {
      serverPort = (server.address() as any).port;
      done();
    });
  });

  afterAll((done) => {
    server.close(done);
  });

  test("Buffer body with high bytes is delivered byte-for-byte", async () => {
    await withCycleTLS({ port: 9311 }, async (cycleTLS) => {
      const original = Buffer.from("deadbeefabcdef123456", "hex");
      const response = await cycleTLS(
        `http://localhost:${serverPort}/echo`,
        { body: original as any, headers: { "Content-Type": "application/octet-stream" } },
        "post"
      );
      expect(response.status).toBe(200);
      expect(received.equals(original)).toBe(true);
    });
  });

  test("All 256 byte values survive round-trip", async () => {
    await withCycleTLS({ port: 9312 }, async (cycleTLS) => {
      const original = Buffer.from(Array.from({ length: 256 }, (_, i) => i));
      const response = await cycleTLS(
        `http://localhost:${serverPort}/echo`,
        { body: original as any, headers: { "Content-Type": "application/octet-stream" } },
        "post"
      );
      expect(response.status).toBe(200);
      expect(received.length).toBe(256);
      expect(crypto.createHash("sha256").update(received).digest("hex")).toBe(
        crypto.createHash("sha256").update(original).digest("hex")
      );
    });
  });

  test("Uint8Array body is routed through bodyBytes", async () => {
    await withCycleTLS({ port: 9313 }, async (cycleTLS) => {
      const original = new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 0x80, 0x81, 0x00, 0x7f, 0xfe]);
      const response = await cycleTLS(
        `http://localhost:${serverPort}/echo`,
        { body: original as any, headers: { "Content-Type": "application/octet-stream" } },
        "post"
      );
      expect(response.status).toBe(200);
      expect(received.equals(Buffer.from(original))).toBe(true);
    });
  });

  test("Explicit bodyBytes option preserves exact bytes", async () => {
    await withCycleTLS({ port: 9314 }, async (cycleTLS) => {
      const original = Buffer.from([0xde, 0xad, 0xbe, 0xef, 0xab, 0xcd, 0xef]);
      const response = await cycleTLS(
        `http://localhost:${serverPort}/echo`,
        { bodyBytes: original as any, headers: { "Content-Type": "application/octet-stream" } } as any,
        "post"
      );
      expect(response.status).toBe(200);
      expect(received.equals(original)).toBe(true);
    });
  });

  test("String body still flows through the `body` field unchanged", async () => {
    await withCycleTLS({ port: 9315 }, async (cycleTLS) => {
      const original = "hello world — plain ASCII text";
      const response = await cycleTLS(
        `http://localhost:${serverPort}/echo`,
        { body: original, headers: { "Content-Type": "text/plain" } },
        "post"
      );
      expect(response.status).toBe(200);
      expect(received.toString("utf8")).toBe(original);
    });
  });
});
