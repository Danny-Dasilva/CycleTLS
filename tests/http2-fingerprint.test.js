const { CycleTLS } = require("../dist/index.js");

jest.setTimeout(process.env.CI ? 60000 : 30000);

describe("HTTP/2 Fingerprinting Tests", () => {
  let client;

  beforeAll(async () => {
    client = new CycleTLS({ port: 9121 });
  });

  afterAll(async () => {
    try {
      if (client) {
        await client.close();
      }
    } catch (e) {
      // Ignore cleanup errors - client may have failed to initialize
    }
  });

  test("Firefox HTTP/2 fingerprint with peet.ws", async () => {
    const firefoxHTTP2 = "1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s";
    
    const response = await client.get('https://tls.peet.ws/api/all', {
      http2Fingerprint: firefoxHTTP2,
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0',
      insecureSkipVerify: true, // tls.peet.ws fixture cert is rotation-prone
    });

    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate HTTP/2 is being used and we got data
    expect(result.http_version).toBe("h2");
    expect(result.http2).toBeDefined();
    expect(result.http2.akamai_fingerprint).toBeDefined();
    
  });

  test("Chrome HTTP/2 fingerprint with peet.ws", async () => {
    const chromeHTTP2 = "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p";

    const response = await client.get('https://tls.peet.ws/api/all', {
      http2Fingerprint: chromeHTTP2,
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36',
      insecureSkipVerify: true, // tls.peet.ws fixture cert is rotation-prone
    });

    expect(response.status).toBe(200);
    const result = await response.json();

    // Validate HTTP/2 is being used and we got data
    expect(result.http_version).toBe("h2");
    expect(result.http2).toBeDefined();
    expect(result.http2.akamai_fingerprint).toBeDefined();

  });

  // Skip scrapfly.io tests - service is unreliable and causes CI flakiness
  // The peet.ws tests above provide equivalent coverage
  test.skip("Firefox HTTP/2 fingerprint with scrapfly.io", async () => {
    const firefoxHTTP2 = "1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s";

    const response = await client.get('https://tools.scrapfly.io/api/fp/anything', {
      http2Fingerprint: firefoxHTTP2,
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0'
    });

    expect(response.status).toBe(200);
    const result = await response.json();

    // Validate HTTP/2 is being used and we got data
    expect(result.http_protocol_version).toBe("HTTP/2.0");
    expect(result.http2).toBeDefined();
    expect(result.http2.fingerprint).toBeDefined();

  }, 60000);

  // Skip scrapfly.io tests - service is unreliable and causes CI flakiness
  // The peet.ws tests above provide equivalent coverage
  test.skip("Chrome HTTP/2 fingerprint with scrapfly.io", async () => {
    const chromeHTTP2 = "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p";

    const response = await client.get('https://tools.scrapfly.io/api/fp/anything', {
      http2Fingerprint: chromeHTTP2,
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
    });

    expect(response.status).toBe(200);
    const result = await response.json();

    // Validate HTTP/2 is being used and we got data
    expect(result.http_protocol_version).toBe("HTTP/2.0");
    expect(result.http2).toBeDefined();
    expect(result.http2.fingerprint).toBeDefined();

  }, 60000);
});