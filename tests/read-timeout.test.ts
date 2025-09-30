import initCycleTLS from "../dist/index.js";
import http from "http";
import { withCycleTLS } from "./test-utils.js";

jest.setTimeout(30000);

describe("Read timeout handling", () => {
  let server: http.Server;
  let serverPort: number;

  beforeAll((done) => {
    // Create a test server that delays the response body
    server = http.createServer((req, res) => {
      if (req.url === "/slow-body") {
        // Send headers immediately
        res.writeHead(200, { "Content-Type": "text/plain" });
        
        // Start sending body but then delay indefinitely
        res.write("Start of response...");
        
        // Never end the response - this will trigger a read timeout
        // The connection stays open but no more data is sent
      } else if (req.url === "/delayed-body") {
        // Send headers immediately
        res.writeHead(200, { "Content-Type": "text/plain" });
        
        // Send partial body after a delay
        setTimeout(() => {
          res.write("Delayed chunk...");
          // Still don't end - will timeout during body read
        }, 2000);
      } else {
        res.writeHead(404);
        res.end("Not found");
      }
    });

    server.listen(0, () => {
      serverPort = (server.address() as any).port;
      done();
    });
  });

  afterAll((done) => {
    server.close(done);
  });

  test("Should handle read timeout with proper error response", async () => {
    await withCycleTLS({ port: 9117 }, async (cycleTLS) => {
      // Test with a server that starts sending but then stalls
      const response = await cycleTLS(
        `http://localhost:${serverPort}/slow-body`,
        {
          body: "",
          timeout: 3, // 3 second timeout
        },
        "get"
      );

      // Should receive a 408 timeout status
      expect(response.status).toBe(408);
      const data = await response.text();
      expect(data).toContain("deadline exceeded");
    });
  });

  test("Should handle mid-stream timeout without hanging", async () => {
    await withCycleTLS({ port: 9118 }, async (cycleTLS) => {
      // Test with a server that delays mid-body
      const response = await cycleTLS(
        `http://localhost:${serverPort}/delayed-body`,
        {
          body: "",
          timeout: 1, // 1 second timeout - will trigger before delayed chunk
        },
        "get"
      );

      // Should receive a 408 timeout status
      expect(response.status).toBe(408);
      const data = await response.text();
      expect(data).toContain("deadline exceeded");
    });
  });

  test("Process should not restart on read timeout errors", async () => {
    await withCycleTLS({ port: 9119, debug: true }, async (cycleTLS) => {
      // Capture console output to verify no restart messages
      const originalLog = console.log;
      const originalError = console.error;
      const logs: string[] = [];

      console.log = (msg: any) => {
        logs.push(String(msg));
        originalLog(msg);
      };
      console.error = (msg: any) => {
        logs.push(String(msg));
        originalError(msg);
      };

      try {
        // Make request that will timeout
        const response = await cycleTLS(
          `http://localhost:${serverPort}/slow-body`,
          {
            body: "",
            timeout: 1,
          },
          "get"
        );

        expect(response.status).toBe(408);

        // Verify no fatal error or restart messages in logs
        const hasRestart = logs.some(log =>
          log.includes("Fatal error detected") ||
          log.includes("restarting") ||
          log.includes("cleanExit")
        );

        expect(hasRestart).toBe(false);

      } finally {
        // Restore console
        console.log = originalLog;
        console.error = originalError;
      }
    });
  });
});