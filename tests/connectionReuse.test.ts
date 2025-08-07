import initCycleTLS from "../dist/index.js";
import * as http from "http";
import * as https from "https";
import * as fs from "fs";
import * as path from "path";

// Set longer timeout for the test
jest.setTimeout(30000);

describe("Connection Reuse Tests", () => {
  // Connection counter server
  let server: https.Server;
  let handshakeCount = 0;
  const PORT = 8443;
  
  // Setup a test HTTPS server that counts TLS handshakes
  beforeAll((done) => {
    // Generate self-signed certificate for testing
    const certPath = path.join(__dirname, "cert.pem");
    const keyPath = path.join(__dirname, "key.pem");
    
    let cert: string;
    let key: string;
    
    cert = fs.readFileSync(certPath, "utf8");
    key = fs.readFileSync(keyPath, "utf8");
    
    const options = {
      key: key,
      cert: cert
    };
    
    // Create the HTTPS server
    server = https.createServer(options, (req, res) => {
      if (req.url === "/handshake-count") {
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.end(handshakeCount.toString());
        return;
      }
      
      res.writeHead(200, { "Content-Type": "text/plain" });
      res.end("OK");
    });
    
    // Track TLS handshakes by hooking into the connection event
    server.on('newSession', () => {
      handshakeCount++;
    });
    
    server.listen(PORT, () => {
      console.log(`Test HTTPS server running on port ${PORT}`);
      done();
    });
  });
  
  // Clean up the test server
  afterAll((done) => {
    if (server) {
      server.close(() => {
        console.log('Test HTTPS server closed');
        done();
      });
    } else {
      done();
    }
  });
  
  test("Should reuse connections for multiple requests to same host", async () => {
    // Initialize CycleTLS
    const cycleTLS = await initCycleTLS();
    
    // Options for requests
    const options = {
      ja3: "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
      userAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
      insecureSkipVerify: true, // Accept self-signed certificate
    };
    
    // Reset handshake counter
    handshakeCount = 0;
    
    // Make the first request - should establish a connection
    const url = `https://localhost:${PORT}`;
    const response1 = await cycleTLS.get(`${url}/first`, options);
    expect(response1.status).toBe(200);
    
    // Make a second request - should reuse the connection
    const response2 = await cycleTLS.get(`${url}/second`, options);
    expect(response2.status).toBe(200);
    
    // Make a third request to get the handshake count
    const countResponse = await cycleTLS.get(`${url}/handshake-count`, options);
    const count = parseInt(await countResponse.text(), 10);
    
    // If connection reuse is working, we expect only 1 handshake
    // But we'll allow up to 2 since the count request itself might trigger a new connection
    expect(count).toBeLessThanOrEqual(2);
    
    // Exit CycleTLS to clean up
    await cycleTLS.exit();
  });
});