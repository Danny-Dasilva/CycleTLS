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
    
    console.log('Firefox peet.ws JA4:', result.tls.ja4);
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
    
    console.log('Chrome peet.ws JA4:', result.tls.ja4);
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
    
    console.log('Firefox scrapfly.io TLS data:', result.tls);
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
    
    console.log('Chrome scrapfly.io TLS data:', result.tls);
  });
});