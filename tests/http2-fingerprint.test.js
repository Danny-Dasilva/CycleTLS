const initCycleTLS = require("../dist/index.js");

describe("HTTP/2 Fingerprinting Tests", () => {
  let cycleTLS;

  beforeAll(async () => {
    cycleTLS = await initCycleTLS({ port: 9121 });
  });

  afterAll(async () => {
    await cycleTLS.exit();
  });

  test("Firefox HTTP/2 fingerprint with peet.ws", async () => {
    const firefoxHTTP2 = "1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s";
    
    const response = await cycleTLS.get('https://tls.peet.ws/api/all', {
      http2Fingerprint: firefoxHTTP2,
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0'
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
    
    const response = await cycleTLS.get('https://tls.peet.ws/api/all', {
      http2Fingerprint: chromeHTTP2,
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
    });

    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate HTTP/2 is being used and we got data
    expect(result.http_version).toBe("h2");
    expect(result.http2).toBeDefined();
    expect(result.http2.akamai_fingerprint).toBeDefined();
    
  });

  test("Firefox HTTP/2 fingerprint with scrapfly.io", async () => {
    const firefoxHTTP2 = "1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s";
    
    const response = await cycleTLS.get('https://tools.scrapfly.io/api/fp/anything', {
      http2Fingerprint: firefoxHTTP2,
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0'
    });
    
    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate HTTP/2 is being used and we got data
    expect(result.http_protocol_version).toBe("HTTP/2.0");
    expect(result.http2).toBeDefined();
    expect(result.http2.fingerprint).toBeDefined();
    
  });

  test("Chrome HTTP/2 fingerprint with scrapfly.io", async () => {
    const chromeHTTP2 = "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p";
    
    const response = await cycleTLS.get('https://tools.scrapfly.io/api/fp/anything', {
      http2Fingerprint: chromeHTTP2,
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
    });

    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate HTTP/2 is being used and we got data
    expect(result.http_protocol_version).toBe("HTTP/2.0");
    expect(result.http2).toBeDefined();
    expect(result.http2.fingerprint).toBeDefined();
    
  });
});