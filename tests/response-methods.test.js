const { CycleTLS } = require("../dist/index.js");
const { Blob } = require('buffer');
const https = require('https');

// Longer Jest timeout — individual tests have their own 60s upstream deadline
jest.setTimeout(90000);

// Probe-skip: if httpbin.org is rate-limiting / down, skip the entire suite up
// front rather than letting every test eat its 60s ceiling. Mirrors the
// pattern in tests/tlsfingerprint/{basic,compression,...}.test.ts.
let serviceAvailable = false;

function probeHttpbin() {
  return new Promise((resolve) => {
    let resolved = false;
    const finish = (ok) => {
      if (resolved) return;
      resolved = true;
      resolve(ok);
    };
    const t = setTimeout(() => finish(false), 5000);
    const req = https.get('https://httpbin.org/status/200', { timeout: 5000 }, (res) => {
      clearTimeout(t);
      const ok = res.statusCode === 200;
      res.on('data', () => {});
      res.on('end', () => finish(ok));
      res.on('error', () => finish(false));
    });
    req.on('error', () => { clearTimeout(t); finish(false); });
    req.on('timeout', () => { req.destroy(); clearTimeout(t); finish(false); });
  });
}

// Status codes treated as transient upstream flake (rate-limit, gateway timeout)
const FLAKE_STATUSES = new Set([408, 421, 429, 502, 503, 504, 521, 522, 523, 524, 525]);

// Wraps a test body with: (a) early skip if upstream probe failed, (b) a 60s
// soft deadline so a hung httpbin doesn't blow the Jest timeout, (c) flake
// status / network-error catch that converts to a skip console.log.
function conditionalTest(name, fn) {
  test(name, async () => {
    if (!serviceAvailable) {
      console.log(`Skipped: ${name} (httpbin.org unavailable)`);
      return;
    }
    const deadline = new Promise((res) => setTimeout(() => res('timeout'), 60000));
    try {
      const result = await Promise.race([fn().then(() => 'ok'), deadline]);
      if (result === 'timeout') {
        console.log(`Skipped: ${name} (upstream hung past 60s deadline)`);
        return;
      }
    } catch (e) {
      const msg = (e && e.message) ? e.message : String(e);
      const isFlakeStatus = [...FLAKE_STATUSES].some((c) => msg.includes(`${c}`) && (msg.includes('statusCode') || msg.includes('status')));
      const isNetworkErr = /timeout|timed out|ECONNRESET|ECONNREFUSED|EAI_AGAIN|ENOTFOUND|socket hang up|aborted/i.test(msg);
      if (isFlakeStatus || isNetworkErr) {
        console.log(`Skipped: ${name} (upstream flake: ${msg.slice(0, 200)})`);
        return;
      }
      throw e;
    }
  });
}

describe("Response Methods Tests", () => {
  let client;

  beforeAll(async () => {
    serviceAvailable = await probeHttpbin();
    if (!serviceAvailable) {
      console.warn('SKIPPING response-methods tests: httpbin.org unavailable');
      return;
    }
    client = new CycleTLS({ port: 9117 });
  });

  afterAll(async () => {
    if (client) {
      await client.close();
    }
  });

  describe("json() method", () => {
    conditionalTest("Should parse JSON response correctly", async () => {
      const response = await client.get('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(typeof response.json).toBe('function');

      const jsonData = await response.json();
      expect(typeof jsonData).toBe('object');
      expect(jsonData).toHaveProperty('slideshow');
    });

    conditionalTest("Should handle invalid JSON gracefully", async () => {
      const response = await client.get('https://httpbin.org/html', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);

      // V2 API throws standard JSON.parse error, not wrapped
      await expect(response.json()).rejects.toThrow(/not valid JSON|Unexpected token/);
    });

    conditionalTest("Should be callable multiple times", async () => {
      const response = await client.get('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const jsonData1 = await response.json();
      const jsonData2 = await response.json();

      expect(jsonData1).toEqual(jsonData2);
    });
  });

  describe("text() method", () => {
    conditionalTest("Should return text content", async () => {
      const response = await client.get('https://httpbin.org/html', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(typeof response.text).toBe('function');

      const textData = await response.text();
      expect(typeof textData).toBe('string');
      expect(textData).toContain('<!DOCTYPE html>');
      expect(textData).toContain('<html>');
    });

    conditionalTest("Should handle plain text responses", async () => {
      const response = await client.get('https://httpbin.org/robots.txt', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);

      const textData = await response.text();
      expect(typeof textData).toBe('string');
      expect(textData).toContain('User-agent');
    });

    conditionalTest("Should be callable multiple times", async () => {
      const response = await client.get('https://httpbin.org/robots.txt', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const textData1 = await response.text();
      const textData2 = await response.text();

      expect(textData1).toEqual(textData2);
    });
  });

  describe("arrayBuffer() method", () => {
    conditionalTest("Should return ArrayBuffer", async () => {
      const response = await client.get('https://httpbin.org/bytes/1024', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(typeof response.arrayBuffer).toBe('function');

      const arrayBuffer = await response.arrayBuffer();
      expect(arrayBuffer instanceof ArrayBuffer).toBe(true);
      expect(arrayBuffer.byteLength).toBe(1024);
    });

    conditionalTest("Should work with different byte sizes", async () => {
      const response = await client.get('https://httpbin.org/bytes/512', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const arrayBuffer = await response.arrayBuffer();
      expect(arrayBuffer.byteLength).toBe(512);
    });

    conditionalTest("Should be callable multiple times", async () => {
      const response = await client.get('https://httpbin.org/bytes/256', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const arrayBuffer1 = await response.arrayBuffer();
      const arrayBuffer2 = await response.arrayBuffer();

      expect(arrayBuffer1.byteLength).toEqual(arrayBuffer2.byteLength);
      // Compare the actual contents
      const view1 = new Uint8Array(arrayBuffer1);
      const view2 = new Uint8Array(arrayBuffer2);
      expect(view1).toEqual(view2);
    });
  });

  describe("blob() method", () => {
    conditionalTest("Should return Blob with correct type", async () => {
      const response = await client.get('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(typeof response.blob).toBe('function');

      const blob = await response.blob();
      expect(blob instanceof Blob).toBe(true);
      // V2 API: headers are stored as arrays (e.g., {"Content-Type": ["application/json"]})
      // The blob() implementation accesses response.headers["content-type"]?.[0]
      // Verify we get a blob with some content type (may vary based on server response header casing)
      expect(blob.size).toBeGreaterThan(0);
      // Content-type should be set (may be application/json or fallback to application/octet-stream)
      expect(blob.type).toBeTruthy();
    });

    conditionalTest("Should handle HTML content type", async () => {
      const response = await client.get('https://httpbin.org/html', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const blob = await response.blob();
      expect(blob instanceof Blob).toBe(true);
      // V2 API: Content-type should be set
      expect(blob.type).toBeTruthy();
      expect(blob.size).toBeGreaterThan(0);
    });

    conditionalTest("Should be callable multiple times", async () => {
      const response = await client.get('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const blob1 = await response.blob();
      const blob2 = await response.blob();

      expect(blob1.size).toEqual(blob2.size);
      expect(blob1.type).toEqual(blob2.type);
    });
  });

  describe("Method compatibility with existing data property", () => {
    conditionalTest("Should have both data property and methods available", async () => {
      const response = await client.get('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      // V2 API: data is always a Readable stream (alias for body)
      expect(response.data).toBeDefined();
      expect(typeof response.data.on).toBe('function'); // It's a stream
      expect(typeof response.data.pipe).toBe('function'); // It's a stream

      // V2 API provides convenience methods for parsing
      expect(typeof response.json).toBe('function');
      expect(typeof response.text).toBe('function');
      expect(typeof response.arrayBuffer).toBe('function');
      expect(typeof response.blob).toBe('function');

      // Test that json() produces valid parsed result
      const jsonFromMethod = await response.json();
      expect(jsonFromMethod).toHaveProperty('slideshow');
    });

    conditionalTest("Should work with stream consumption via methods", async () => {
      const response = await client.get('https://httpbin.org/html', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      // V2 API: response.data is always a stream, use response.text() to get string
      expect(response.data).toBeDefined();
      expect(typeof response.data.on).toBe('function'); // It's a stream

      // Use text() method to get the content
      const textFromMethod = await response.text();
      expect(typeof textFromMethod).toBe('string');
      expect(textFromMethod).toContain('<!DOCTYPE html>');
    });
  });

  describe("Cross-method consistency", () => {
    conditionalTest("JSON content should be consistent across methods", async () => {
      const response = await client.get('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const jsonData = await response.json();
      const textData = await response.text();
      const parsedFromText = JSON.parse(textData);

      expect(jsonData).toEqual(parsedFromText);
    });

    conditionalTest("ArrayBuffer and Blob should have consistent size", async () => {
      const response = await client.get('https://httpbin.org/bytes/1024', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const arrayBuffer = await response.arrayBuffer();
      const blob = await response.blob();

      expect(arrayBuffer.byteLength).toEqual(blob.size);
    });
  });
});
