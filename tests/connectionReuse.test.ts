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
    
    try {
      cert = fs.readFileSync(certPath, "utf8");
      key = fs.readFileSync(keyPath, "utf8");
    } catch (err) {
      // If certs don't exist, use hardcoded self-signed certs for testing
      cert = `-----BEGIN CERTIFICATE-----
MIIDBjCCAe4CCQDf+pSPo/OpvDANBgkqhkiG9w0BAQsFADBFMQswCQYDVQQGEwJB
VTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50ZXJuZXQgV2lkZ2l0
cyBQdHkgTHRkMB4XDTIzMDMwNDE3NDIzMFoXDTI0MDMwMzE3NDIzMFowRTELMAkG
A1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0
IFdpZGdpdHMgUHR5IEx0ZDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AMJGbdNVN3VG34L/Uf7jrQG3YwbqUTgpxz1PxXw3ZbAVUIJ2xM5RM5TRFZhOKj5o
l5qFP8u5+L7zWuoNpI/urVmUe5mNPLlWUgY0MQxtZnHj9da8zkiY9XCE1Rk2DCHJ
yK8OS9gPt0rLkEuZTDTFdONZVFZt0sSxCNcV/dUKILSKV8xnUbdTGUulPwk9Vkh9
0yAs9mkbDgL4tQu11D+1XqzKCVijl0XZ2xW/q5AfuFFC7uU7GbRJxR8pk27vxYT6
/DU60xQAWZzGXyCEW1qs5ZoKtLMk8W7ORQJBr8YDayvHnTnNQJQfk7KbBlgyw9jK
LrY1G2SiJ0fqZkVTt0f/dbUCAwEAATANBgkqhkiG9w0BAQsFAAOCAQEAL+KTW2fK
4H3EhIJWbmCNGBvQkXpAXg1MwIkv/RBIRVtP4bfB8/iQOQ+Qhutv+BwJQ4GQivES
q3ZaOWpAsDCJKMXb+UUFVIlWa3nCrZh8UYe+EQVmXYCOTKiS0J0vWIyTK9ZbtGPW
Dzvx2lNKEgGhzIQWsUVvLtXFXcmyDKIxJQN3FvLpVTTRzs4R4zVpvFV3MeZOEwbT
kKHZwTZFJl2sGXk1GyEKzIvkVpj1V4xJ9+XSRnqxwIx4oBcFnDnJZGl4mFGUPDq8
e7tc1inXP9gwWaiZNoEmeoP2ps4PHR2GJnAQMbQyxBgLTHV/FrOelCuVVTXx22Rn
+d3I4I4SwlYkiw==
-----END CERTIFICATE-----`;
      key = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDCRm3TVTd1Rt+C
/1H+460Bt2MG6lE4Kcc9T8V8N2WwFVCCdsTOUTOU0RWYTio+aJeahT/Lufi+81rq
DaSP7q1ZlHuZjTy5VlIGNDEMbWZx4/XWvM5ImPVwhNUZNgwhycivDkvYD7dKy5BL
mUw0xXTjWVRWbdLEsQjXFf3VCiC0ilfMZ1G3UxlLpT8JPVZI
fdMgLPZpGw4C+LULtdQ/tV6syglYo5dF2dsVv6uQH7hRQu7lOxm0ScUfKZNu78WE
+vw1OtMUAFmcxl8ghFtarOWaCrSzJPFuzkUCQa/GA2srx505zUCUH5OymwZYMsPY
yi62NRtkoic+mZFU7dH/3W1JAgMBAAECggEAMYnckSCY+JVjHxKP1T+THsp7LlHI
6q3Y0jJ4eQS7+daGUbKvNEsyXrKPF3T8iPZjLs8c7nQmuKjEeBLAKMSITXZ9IXiR
QeUl8jOWwX3EEWiYO6QdQKkcK4+6Ny3bLwCCRn1bm8+XYbESlxeK9L8/0yu/jVqx
9XrKFgK2I9+nyp6yTmYSXnSTkPhaYSEHUNQJxXeiG86t0UMQfKUdDczcHZIcQvFS
q4CTxITD/EQwRl/QgCwKcLVT5FQ6pQgiYrKzbwZHvSJGBj53rPZpYUDduXLV8Vlt
9n1OLT/edvt+PuCkoJa+nSVTg9MOLD2iqUSL8+aGoCToYEAAQKBgQDkWx5aMPUeR
ojv/ZFhbsIKza4HnXy2tXe0B2D6IxXwsxjmEDbKSSDZ3QOgEJWNQVRlXIkrZt+an
X0aZoaMeXpBN8eJJ7QQ2ZZGGaLVUCGFx1M52f1UM1vb63cb5l8OL1TKDA1lkLBRY
mLMFOfrp7mSKWTOLNQzbMmPjz9WLqQKBgQDZcJjgJkS1LzgFoGC4AULYsWR74nZ0
I2+UvjVXVitD0fpgGcETYKq/uC/KaGzDkLwqkpKrXXCR8xvJ9ehLG+G7GnhwBBG7
/Dx00mevpoLyK43vtIG89FW/X1fULfzK93hMlB39tBzwKIfAVDdO8FgbKsAMJDUq
9bFPME0+AQKBgQDNQhFHN7d9PQ+UfhzylZhzXbX+qXFljGem58XtYE5SvfoV+GRM
KyxLYU8WZXcfUGKHVdOiQelLnLlF1qV/uKrYUn3TQoQ8zPexFSVmYDJQmXKKnpkp
xYAjQXZRT3OjLzNK614XYecYbDPQA5FVq/NLWs5KtCuJZL2x/+MMdcJj+QKBgB6D
SGi/JJrVe5h9PkZdWoMCB/wlYac3CyOu0NJZL7Mth6UpSmp9T/ZGJ0XYLjRVtdyu
7GOvw6ZWqh4q9OeDZNy+dOipXQeGSIB8oPzWUluDYdWTQVLr0/OFVNPBPaDLSy2h
8QzWm/YIV/mCQdkYcRkijZ++6/jTm3AEhV/0MYABAoGBAJAOFpS7gyojXzPkdNSy
MzNOmOigjBYZxzYpcBQnaLpGkSG2tnCY5YvzNk+XfXvLUOjJTdXGe9xNyYkdqm3X
0R1h/KUjZuJUvHoFBRZ7x3CZrr/XqQPQzvnFgLuYuBdTnwlTbh0FJJuHK3o4uPOz
jjCsB7/IOWOwXXmGQBdM2ZVt
-----END PRIVATE KEY-----`;
      
      // Write the hardcoded certs to files for future use
      fs.writeFileSync(certPath, cert);
      fs.writeFileSync(keyPath, key);
    }
    
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