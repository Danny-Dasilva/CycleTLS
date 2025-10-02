import initCycleTLS from "../dist/index.js";
import * as https from "https";
import * as tls from "tls";
import * as fs from "fs";
import * as path from "path";

jest.setTimeout(30000);

describe("Custom SNI (serverName) support", () => {
  let server: https.Server;
  let capturedServerName: string | undefined;
  let capturedHostHeader: string | undefined;
  let port: number;

  beforeAll((done) => {
    const certPath = path.join(__dirname, "cert.pem");
    const keyPath = path.join(__dirname, "key.pem");

    const cert = fs.readFileSync(certPath, "utf8");
    const key = fs.readFileSync(keyPath, "utf8");

    const options: https.ServerOptions = {
      key,
      cert,
      // SNICallback allows us to capture the SNI server name during handshake
      SNICallback: (servername, cb) => {
        capturedServerName = servername;
        const ctx = tls.createSecureContext({ key, cert });
        cb(null, ctx);
      },
    };

    server = https.createServer(options, (req, res) => {
      capturedHostHeader = req.headers["host"] as string | undefined;
      res.writeHead(200, { "Content-Type": "text/plain" });
      res.end("OK");
    });

    server.listen(0, () => {
      const addressInfo = server.address();
      if (typeof addressInfo === "object" && addressInfo && typeof addressInfo.port === "number") {
        port = addressInfo.port;
      } else {
        throw new Error("Failed to acquire test server port");
      }
      done();
    });
  });

  afterAll((done) => {
    if (server) {
      server.close(() => done());
    } else {
      done();
    }
  });

  test("Should send custom SNI while keeping custom Host header", async () => {
    const cycleTLS = await initCycleTLS();

    const url = `https://127.0.0.1:${port}/`; // connect by IP to ensure SNI matters
    const options = {
      insecureSkipVerify: true,
      // provide a custom serverName for TLS SNI
      serverName: "front.example",
      // and a different HTTP Host header for domain fronting
      headers: { Host: "real.example" },
      // keep defaults for fingerprinting
      ja3: "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
      userAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
    } as any;

    const resp = await cycleTLS.get(url, options);
    expect(resp.status).toBe(200);

    // Validate that SNI and Host header were as intended
    expect(capturedServerName).toBe("front.example");
    expect(capturedHostHeader).toBe("real.example");

    await cycleTLS.exit();
  });
});

