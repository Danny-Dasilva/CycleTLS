const initCycleTLS = require("../dist/index.js");

describe("JA4 Fingerprinting Tests", () => {

  test("Firefox JA4_r fingerprint exact match", async () => {
    const firefoxJA4r = "t13d1717h2_002f,0035,009c,009d,1301,1302,1303,c009,c00a,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0005,000a,000b,000d,0012,0017,001b,001c,0022,0023,002b,002d,0033,fe0d,ff01_0403,0503,0603,0804,0805,0806,0401,0501,0601,0203,0201";    
    const cycleTLS = await initCycleTLS({ port: 9120 });
    
    try {
      const response = await cycleTLS.get('https://tls.peet.ws/api/all', {
        ja4r: firefoxJA4r,
        disableGrease: false,
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0'
      });
      
      expect(response.status).toBe(200);
      const result = await response.json();
      
      // Validate JA4_r response and HTTP/2 negotiation
      expect(result.tls).toBeDefined();
      expect(result.tls.ja4_r).toBeDefined();
      expect(result.http_version).toBe("h2");
      // Check for ECH extension (fe0d) preservation
      expect(result.tls.ja4_r).toContain("fe0d");
      // Check for Delegated Credentials (0022) - now fixed
      expect(result.tls.ja4_r).toContain("0022");
      // Check header format - should remain t13d1717h2 (17 extensions, ALPN auto-removed)
      expect(result.tls.ja4_r).toMatch(/^t13d1717h2/);
      // Verify expected output (ALPN auto-removed since h2 in header, so 17->17 extensions but 0010 removed)
      expect(result.tls.ja4_r).toBe(firefoxJA4r);
    } finally {
      await cycleTLS.exit();
    }
  }, 15000);

  test("Chrome JA4_r fingerprint exact match", async () => {
    const chromeJA4r = "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601";    
    const cycleTLS = await initCycleTLS({ port: 9121 });
    
    try {
      const response = await cycleTLS.get('https://tls.peet.ws/api/all', {
        ja4r: chromeJA4r,
        disableGrease: false,
        userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
      });

      expect(response.status).toBe(200);
      const result = await response.json();
      
      // Validate JA4_r response and HTTP/2 negotiation  
      expect(result.tls).toBeDefined();
      expect(result.tls.ja4_r).toBeDefined();
      expect(result.http_version).toBe("h2"); // Confirms ALPN worked
      expect(result.tls.ja4_r).toContain("44cd"); // New ALPS extension
      expect(result.tls.ja4_r).toContain("fe0d"); // ECH extension
      expect(result.tls.ja4_r).toMatch(/^t13d1516h2/); // Correct header format
      // Verify exact match (ALPN is auto-handled with h2, so it's removed from extensions)
      expect(result.tls.ja4_r).toBe(chromeJA4r);
    } finally {
      await cycleTLS.exit();
    }
  }, 15000);

  test("Chrome 138 JA4_r exact match test", async () => {
    const chrome138JA4r = "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601";    
    const cycleTLS = await initCycleTLS({ port: 9122 });
    
    try {
      const response = await cycleTLS.get('https://tls.peet.ws/api/all', {
        ja4r: chrome138JA4r,
        disableGrease: false,
        userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
      });

      expect(response.status).toBe(200);
      const result = await response.json();
      
      // Validate JA4_r response and HTTP/2 negotiation
      expect(result.tls).toBeDefined();
      expect(result.tls.ja4_r).toBeDefined();
      expect(result.http_version).toBe("h2");
      expect(result.tls.ja4_r).toContain("44cd"); // ALPS extension
      expect(result.tls.ja4_r).toContain("fe0d"); // ECH extension
      expect(result.tls.ja4_r).toMatch(/^t13d1516h2/);
      expect(result.tls.ja4_r).toBe(chrome138JA4r);
    } finally {
      await cycleTLS.exit();
    }
  }, 15000);

  test("Chrome 139 JA4_r exact match test", async () => {
    const chrome139JA4r = "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601";    
    const cycleTLS = await initCycleTLS({ port: 9123 });
    
    try {
      const response = await cycleTLS.get('https://tls.peet.ws/api/all', {
        ja4r: chrome139JA4r,
        disableGrease: false,
        userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36'
      });

      expect(response.status).toBe(200);
      const result = await response.json();
      
      // Validate JA4_r response and HTTP/2 negotiation
      expect(result.tls).toBeDefined();
      expect(result.tls.ja4_r).toBeDefined();
      expect(result.http_version).toBe("h2");
      expect(result.tls.ja4_r).toContain("44cd"); // ALPS extension
      expect(result.tls.ja4_r).toContain("fe0d"); // ECH extension
      expect(result.tls.ja4_r).toMatch(/^t13d1516h2/);
      expect(result.tls.ja4_r).toBe(chrome139JA4r);
    } finally {
      await cycleTLS.exit();
    }
  }, 15000);

  test("TLS 1.2 JA4_r fingerprint exact match", async () => {
    const tls12JA4r = "t12d128h2_002f,0035,009c,009d,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0005,000a,000b,000d,0017,0023,ff01_0403,0804,0401,0503,0805,0501,0806,0601,0201";
    
    const cycleTLS = await initCycleTLS({ port: 9124 });
    
    try {
      const response = await cycleTLS.get('https://tls.peet.ws/api/all', {
        ja4r: tls12JA4r,
        disableGrease: false,
        userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
      });

      expect(response.status).toBe(200);
      const result = await response.json();
      
      // Validate TLS 1.2 negotiation and JA4_r
      expect(result.tls).toBeDefined();
      expect(result.tls.ja4_r).toBeDefined();
      expect(result.http_version).toBe("h2");
      // TLS 1.2 response should be t12d128h2 (8 extensions with h2, ALPN auto-handled, single-digit format)
      expect(result.tls.ja4_r).toMatch(/^t12d128h2/);
      expect(result.tls.ja4_r).toBe(tls12JA4r);
    } finally {
      await cycleTLS.exit();
    }
  }, 15000);
});