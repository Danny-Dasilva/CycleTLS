import initCycleTLS from "../dist/index.js";
import { withCycleTLS } from "./test-utils.js";
import * as http2 from "node:http2";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { execSync } from "node:child_process";

const TMP_KEY = path.join(os.tmpdir(), "h2-test-key.pem");
const TMP_CERT = path.join(os.tmpdir(), "h2-test-cert.pem");

/**
 * Tests for HTTP/2 POST body transmission on servers with strict flow control.
 *
 * These tests use a local HTTPS server configured with strict HTTP/2 settings
 * (initialWindowSize: 16384) to verify that request bodies are correctly
 * transmitted as HTTP/2 DATA frames.
 *
 * Background: fhttp's AutoUpdate() was not setting InitialWindowSize for
 * Chrome/Firefox navigators, causing connection-level flow control to start
 * at 0. This blocked all body writes in awaitFlowControl(), so POST/PUT/PATCH
 * DATA frames were never sent. Servers with strict flow control would
 * RST_STREAM with CANCEL; lenient servers happened to work.
 */

jest.setTimeout(30000);

// Generate self-signed cert for local HTTP/2 server. Use os.tmpdir() so this
// works on Windows runners (where /tmp/ doesn't exist).
beforeAll(() => {
  // 2>/dev/null is a POSIX shellism; on Windows openssl prints to stderr
  // regardless and the redirect breaks the command. Drop it cross-platform —
  // execSync still throws on non-zero exit, which is what we care about.
  execSync(
    `openssl req -x509 -newkey rsa:2048 -keyout "${TMP_KEY}" -out "${TMP_CERT}" -days 1 -nodes -subj "/CN=localhost"`,
    { stdio: "ignore" }
  );
});

/** Start a local HTTP/2 server with strict flow control settings. */
function startStrictH2Server(): Promise<{
  port: number;
  close: () => Promise<void>;
}> {
  return new Promise((resolve) => {
    const server = http2.createSecureServer({
      key: fs.readFileSync(TMP_KEY),
      cert: fs.readFileSync(TMP_CERT),
      settings: {
        initialWindowSize: 16384,
        maxFrameSize: 16384,
      },
    });

    server.on("stream", (stream, headers) => {
      const method = headers[":method"];
      let body = "";
      stream.on("data", (chunk: Buffer) => (body += chunk.toString()));
      // Without the fhttp fix, the client sends RST_STREAM CANCEL which destroys
      // the stream before 'end' fires. The stream error prevents respond() from
      // being called, so CycleTLS times out and returns status 408. The test then
      // fails via the expect(status).toBe(200) assertion.
      stream.on("error", () => {});
      stream.on("end", () => {
        if (!stream.destroyed) {
          stream.respond({ ":status": 200, "content-type": "application/json" });
          stream.end(JSON.stringify({ method, bodyLength: body.length, body }));
        }
      });
    });

    server.listen(0, "127.0.0.1", () => {
      const port = (server.address() as any).port;
      resolve({
        port,
        close: () => new Promise<void>((res) => server.close(() => res())),
      });
    });
  });
}

test("Should send form-encoded POST body over HTTP/2 with strict flow control", async () => {
  const h2 = await startStrictH2Server();
  try {
    await withCycleTLS({ port: 9220 }, async (cycleTLS) => {
      const response = await cycleTLS(
        `https://localhost:${h2.port}/test`,
        {
          body: "key1=value1&key2=value2",
          headers: { "Content-Type": "application/x-www-form-urlencoded" },
          insecureSkipVerify: true,
        },
        "post"
      );

      expect(response.status).toBe(200);
      const responseBody = await response.json();
      expect(responseBody.method).toBe("POST");
      expect(responseBody.body).toBe("key1=value1&key2=value2");
    });
  } finally {
    await h2.close();
  }
});

test("Should send JSON POST body over HTTP/2 with strict flow control", async () => {
  const h2 = await startStrictH2Server();
  try {
    await withCycleTLS({ port: 9221 }, async (cycleTLS) => {
      const jsonBody = JSON.stringify({ message: "hello", number: 42 });
      const response = await cycleTLS(
        `https://localhost:${h2.port}/test`,
        {
          body: jsonBody,
          headers: { "Content-Type": "application/json" },
          insecureSkipVerify: true,
        },
        "post"
      );

      expect(response.status).toBe(200);
      const responseBody = await response.json();
      expect(responseBody.method).toBe("POST");
      expect(JSON.parse(responseBody.body)).toEqual({
        message: "hello",
        number: 42,
      });
    });
  } finally {
    await h2.close();
  }
});

test("Should send PUT body over HTTP/2 with strict flow control", async () => {
  const h2 = await startStrictH2Server();
  try {
    await withCycleTLS({ port: 9222 }, async (cycleTLS) => {
      const response = await cycleTLS(
        `https://localhost:${h2.port}/test`,
        {
          body: "updated=true",
          headers: { "Content-Type": "application/x-www-form-urlencoded" },
          insecureSkipVerify: true,
        },
        "put"
      );

      expect(response.status).toBe(200);
      const responseBody = await response.json();
      expect(responseBody.method).toBe("PUT");
      expect(responseBody.body).toBe("updated=true");
    });
  } finally {
    await h2.close();
  }
});

test("Should send PATCH body over HTTP/2 with strict flow control", async () => {
  const h2 = await startStrictH2Server();
  try {
    await withCycleTLS({ port: 9223 }, async (cycleTLS) => {
      const response = await cycleTLS(
        `https://localhost:${h2.port}/test`,
        {
          body: JSON.stringify({ field: "patched" }),
          headers: { "Content-Type": "application/json" },
          insecureSkipVerify: true,
        },
        "patch"
      );

      expect(response.status).toBe(200);
      const responseBody = await response.json();
      expect(responseBody.method).toBe("PATCH");
      expect(JSON.parse(responseBody.body)).toEqual({ field: "patched" });
    });
  } finally {
    await h2.close();
  }
});
