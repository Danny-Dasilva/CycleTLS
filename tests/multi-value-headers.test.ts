import CycleTLS from "../dist/index.js";
import { createSuiteInstance } from "./test-utils.js";
import * as https from "https";
import * as fs from "fs";
import * as path from "path";

jest.setTimeout(30000);

// Companion to the Go-side multi-value header support (#423 equivalent): request
// headers may be `string | string[]`, array values are sent as repeated headers,
// and exact header key casing is preserved on the wire.
//
// A local HTTPS server captures req.rawHeaders, which preserves both original
// casing and repeated values as a flat [name, value, name, value, ...] array.
// HTTP/1.1 is forced because HTTP/2 lower-cases all header field names by spec,
// so casing preservation is only observable over HTTP/1.1.
describe("Multi-value request headers", () => {
  let client: CycleTLS;
  let cleanup: () => Promise<void>;
  let server: https.Server;
  let capturedRawHeaders: string[] = [];
  let url: string;

  beforeAll(async () => {
    ({ instance: client, cleanup } = await createSuiteInstance({ port: 9410 }));

    const cert = fs.readFileSync(path.join(__dirname, "cert.pem"), "utf8");
    const key = fs.readFileSync(path.join(__dirname, "key.pem"), "utf8");

    await new Promise<void>((resolve) => {
      server = https.createServer({ key, cert }, (req, res) => {
        capturedRawHeaders = req.rawHeaders;
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.end("OK");
      });
      server.listen(0, () => {
        const addr = server.address();
        if (typeof addr === "object" && addr && typeof addr.port === "number") {
          url = `https://127.0.0.1:${addr.port}/`;
          resolve();
        } else {
          throw new Error("Failed to acquire test server port");
        }
      });
    });
  });

  afterAll(async () => {
    await cleanup();
    if (server) {
      await new Promise<void>((resolve) => server.close(() => resolve()));
    }
  });

  // Walk rawHeaders pairs and return every value sent under a given key (case-sensitive).
  const valuesForKey = (raw: string[], key: string): string[] => {
    const out: string[] = [];
    for (let i = 0; i < raw.length; i += 2) {
      if (raw[i] === key) out.push(raw[i + 1]);
    }
    return out;
  };

  const keysIn = (raw: string[]): string[] => {
    const out: string[] = [];
    for (let i = 0; i < raw.length; i += 2) out.push(raw[i]);
    return out;
  };

  test("Should send an array header value as repeated headers", async () => {
    const resp = await client.get(url, {
      insecureSkipVerify: true,
      forceHTTP1: true,
      headers: {
        "X-Multi": ["one", "two"],
      },
    });

    expect(resp.status).toBe(200);
    expect(valuesForKey(capturedRawHeaders, "X-Multi")).toEqual(["one", "two"]);
  });

  test("Should preserve exact header key casing", async () => {
    const resp = await client.get(url, {
      insecureSkipVerify: true,
      forceHTTP1: true,
      headers: {
        "x-CUSTOM-case": "value",
      },
    });

    expect(resp.status).toBe(200);
    const keys = keysIn(capturedRawHeaders);
    expect(keys).toContain("x-CUSTOM-case");
    expect(keys).not.toContain("X-Custom-Case");
  });

  test("Should accept a plain string header value (backwards compatible)", async () => {
    const resp = await client.get(url, {
      insecureSkipVerify: true,
      forceHTTP1: true,
      headers: {
        "X-Single": "solo",
      },
    });

    expect(resp.status).toBe(200);
    expect(valuesForKey(capturedRawHeaders, "X-Single")).toEqual(["solo"]);
  });
});
