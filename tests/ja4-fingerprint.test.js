const initCycleTLS = require("../dist/index.js");

describe("JA4 Fingerprinting Tests", () => {
  let cycleTLS;

  beforeAll(async () => {
    cycleTLS = await initCycleTLS({ port: 9120 });
  });

  afterAll(async () => {
    await cycleTLS.exit();
  });

  test("Firefox JA4 fingerprint with peet.ws", async () => {
    const firefoxJA4 = "t13d1717h2_5b57614c22b0_f2748d6cd58d";
    
    const response = await cycleTLS.get('https://tls.peet.ws/api/all', {
      ja4: firefoxJA4,
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0'
    });
    
    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate we got TLS data back and it includes JA4
    expect(result.tls).toBeDefined();
    expect(result.tls.ja4).toBeDefined();
    expect(result.http_version).toBe("h2");
    
  });

  test("Chrome JA4 fingerprint with peet.ws", async () => {
    const chromeJA4 = "t13d1517h2_8daaf6152771_7e51fdad25f2";
    
    const response = await cycleTLS.get('https://tls.peet.ws/api/all', {
      ja4: chromeJA4,
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
    });

    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate we got TLS data back and it includes JA4
    expect(result.tls).toBeDefined();
    expect(result.tls.ja4).toBeDefined();
    expect(result.http_version).toBe("h2");
    
  });

  test("Firefox JA4 fingerprint with scrapfly.io", async () => {
    const firefoxJA4 = "t13d1717h2_5b57614c22b0_f2748d6cd58d";
    
    const response = await cycleTLS.get('https://tools.scrapfly.io/api/fp/anything', {
      ja4: firefoxJA4,
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0'
    });
    
    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate we got TLS data back
    expect(result.tls).toBeDefined();
    expect(result.http_protocol_version).toBe("HTTP/2.0");
    
  });

  test("Chrome JA4 fingerprint with scrapfly.io", async () => {
    const chromeJA4 = "t13d1517h2_8daaf6152771_7e51fdad25f2";
    
    const response = await cycleTLS.get('https://tools.scrapfly.io/api/fp/anything', {
      ja4: chromeJA4,
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
    });

    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate we got TLS data back  
    expect(result.tls).toBeDefined();
    expect(result.http_protocol_version).toBe("HTTP/2.0");
    
  });

  test("TLS 1.2 JA4 fingerprint with peet.ws", async () => {
    // Real TLS 1.2 JA4 fingerprint from the provided data
    const tls12JA4 = "t12d1209h2_d34a8e72043a_b39be8c56a14";
    
    const response = await cycleTLS.get('https://tls.peet.ws/api/all', {
      ja4: tls12JA4,
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
    });
    
    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate TLS data
    expect(result.tls).toBeDefined();
    expect(result.tls.ja4).toBeDefined();
    expect(result.http_version).toBeDefined();
  });

  test("TLS 1.2 JA4 fingerprint with scrapfly.io", async () => {
    // Real TLS 1.2 JA4 fingerprint with JA3 matching
    const tls12JA4 = "t12d1209h2_d34a8e72043a_b39be8c56a14";
    
    const response = await cycleTLS.get('https://tools.scrapfly.io/api/fp/anything', {
      ja4: tls12JA4,
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
    });
    
    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate TLS data
    expect(result.tls).toBeDefined();
    expect(result.http_protocol_version).toBeDefined();
  });
});