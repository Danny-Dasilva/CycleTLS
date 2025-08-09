const initCycleTLS = require("../dist/index.js");

describe("JA4H Fingerprinting Tests", () => {
  let cycleTLS;

  beforeAll(async () => {
    cycleTLS = await initCycleTLS({ port: 9122 });
  });

  afterAll(async () => {
    await cycleTLS.exit();
  });

  test("GET HTTP/1.1 JA4H fingerprint with httpbin.org", async () => {
    const ja4hFingerprint = "ge11_73a4f1e_8b3fce7";
    
    const response = await cycleTLS.get('https://httpbin.org/get', {
      ja4h: ja4hFingerprint,
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0'
    });

    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate we got data back
    expect(result.url).toBeDefined();
    expect(result.headers).toBeDefined();
    expect(result.headers['User-Agent']).toContain('Firefox');
  });

  test("POST HTTP/2.0 JA4H fingerprint with httpbin.org", async () => {
    const ja4hFingerprint = "po20_ab123cd_ef456gh";
    
    const response = await cycleTLS('https://httpbin.org/post', {
      method: 'POST',
      ja4h: ja4hFingerprint,
      body: JSON.stringify({ test: 'data' }),
      headers: {
        'Content-Type': 'application/json'
      },
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
    });

    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate we got data back
    expect(result.url).toBeDefined();
    expect(result.headers).toBeDefined();
    expect(result.json).toBeDefined();
    expect(result.json.test).toBe('data');
  });

  test("PUT HTTP/1.1 JA4H fingerprint with httpbin.org", async () => {
    const ja4hFingerprint = "pu11_def789_abc123";
    
    const response = await cycleTLS('https://httpbin.org/put', {
      method: 'PUT', 
      ja4h: ja4hFingerprint,
      body: JSON.stringify({ updated: 'value' }),
      headers: {
        'Content-Type': 'application/json'
      },
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
    });

    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate we got data back
    expect(result.url).toBeDefined();
    expect(result.json).toBeDefined();
    expect(result.json.updated).toBe('value');
  });

  test("JA4H fingerprint with custom headers", async () => {
    const ja4hFingerprint = "ge11_custom1_custom2";
    
    const response = await cycleTLS.get('https://httpbin.org/headers', {
      ja4h: ja4hFingerprint,
      headers: {
        'X-Custom-Header': 'test-value',
        'X-Another-Header': 'another-value'
      },
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0'
    });

    expect(response.status).toBe(200);
    const result = await response.json();
    
    // Validate custom headers were sent
    expect(result.headers).toBeDefined();
    expect(result.headers['X-Custom-Header']).toBe('test-value');
    expect(result.headers['X-Another-Header']).toBe('another-value');
  });
});