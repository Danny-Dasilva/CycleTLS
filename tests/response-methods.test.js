const initCycleTLS = require("../dist/index.js");

describe("Response Methods Tests", () => {
  let cycleTLS;

  beforeAll(async () => {
    cycleTLS = await initCycleTLS({ port: 9117 });
  });

  afterAll(async () => {
    if (cycleTLS) {
      await cycleTLS.exit();
    }
  });

  describe("json() method", () => {
    test("Should parse JSON response correctly", async () => {
      const response = await cycleTLS('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(typeof response.json).toBe('function');
      
      const jsonData = await response.json();
      expect(typeof jsonData).toBe('object');
      expect(jsonData).toHaveProperty('slideshow');
    });

    test("Should handle invalid JSON gracefully", async () => {
      const response = await cycleTLS('https://httpbin.org/html', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      
      await expect(response.json()).rejects.toThrow('Failed to parse response as JSON');
    });

    test("Should be callable multiple times", async () => {
      const response = await cycleTLS('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const jsonData1 = await response.json();
      const jsonData2 = await response.json();
      
      expect(jsonData1).toEqual(jsonData2);
    });
  });

  describe("text() method", () => {
    test("Should return text content", async () => {
      const response = await cycleTLS('https://httpbin.org/html', {
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

    test("Should handle plain text responses", async () => {
      const response = await cycleTLS('https://httpbin.org/robots.txt', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      
      const textData = await response.text();
      expect(typeof textData).toBe('string');
      expect(textData).toContain('User-agent');
    });

    test("Should be callable multiple times", async () => {
      const response = await cycleTLS('https://httpbin.org/robots.txt', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const textData1 = await response.text();
      const textData2 = await response.text();
      
      expect(textData1).toEqual(textData2);
    });
  });

  describe("arrayBuffer() method", () => {
    test("Should return ArrayBuffer", async () => {
      const response = await cycleTLS('https://httpbin.org/bytes/1024', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(typeof response.arrayBuffer).toBe('function');
      
      const arrayBuffer = await response.arrayBuffer();
      expect(arrayBuffer instanceof ArrayBuffer).toBe(true);
      expect(arrayBuffer.byteLength).toBe(1024);
    });

    test("Should work with different byte sizes", async () => {
      const response = await cycleTLS('https://httpbin.org/bytes/512', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const arrayBuffer = await response.arrayBuffer();
      expect(arrayBuffer.byteLength).toBe(512);
    });

    test("Should be callable multiple times", async () => {
      const response = await cycleTLS('https://httpbin.org/bytes/256', {
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
    test("Should return Blob with correct type", async () => {
      const response = await cycleTLS('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(typeof response.blob).toBe('function');
      
      const blob = await response.blob();
      expect(blob instanceof Blob).toBe(true);
      expect(blob.type).toContain('application/json');
      expect(blob.size).toBeGreaterThan(0);
    });

    test("Should handle HTML content type", async () => {
      const response = await cycleTLS('https://httpbin.org/html', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const blob = await response.blob();
      expect(blob instanceof Blob).toBe(true);
      expect(blob.type).toContain('text/html');
    });

    test("Should be callable multiple times", async () => {
      const response = await cycleTLS('https://httpbin.org/json', {
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
    test("Should have both data property and methods available", async () => {
      const response = await cycleTLS('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      // Check that both old and new APIs work
      expect(response.data).toBeDefined();
      expect(typeof response.json).toBe('function');
      expect(typeof response.text).toBe('function');
      expect(typeof response.arrayBuffer).toBe('function');
      expect(typeof response.blob).toBe('function');

      // Test that both produce consistent results
      const jsonFromMethod = await response.json();
      expect(response.data).toEqual(jsonFromMethod);
    });

    test("Should work with different response types", async () => {
      const response = await cycleTLS('https://httpbin.org/html', {
        responseType: 'text',
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      // data should be pre-parsed as text
      expect(typeof response.data).toBe('string');
      
      // methods should still work
      const textFromMethod = await response.text();
      expect(response.data).toEqual(textFromMethod);
    });
  });

  describe("Cross-method consistency", () => {
    test("JSON content should be consistent across methods", async () => {
      const response = await cycleTLS('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const jsonData = await response.json();
      const textData = await response.text();
      const parsedFromText = JSON.parse(textData);
      
      expect(jsonData).toEqual(parsedFromText);
    });

    test("ArrayBuffer and Blob should have consistent size", async () => {
      const response = await cycleTLS('https://httpbin.org/bytes/1024', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      const arrayBuffer = await response.arrayBuffer();
      const blob = await response.blob();
      
      expect(arrayBuffer.byteLength).toEqual(blob.size);
    });
  });
});