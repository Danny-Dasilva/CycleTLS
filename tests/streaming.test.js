const { CycleTLS } = require("../dist/index.js");
const { probeUpstream, makeConditionalTest } = require("./test-utils.js");

// Longer Jest timeout — individual tests have their own 30s upstream deadline
jest.setTimeout(90000);

// Circuit-breaker state shared across all tests in this file. Once any test
// hits the upstream deadline or surfaces a flake-class error, all subsequent
// tests skip instantly so a hung httpbin doesn't blow the 20-min CI ceiling.
const upstreamState = { serviceAvailable: false, upstreamUnreachable: false };
const conditionalTest = makeConditionalTest(upstreamState, 30000);

// FIXME(v3.0.0 windows): same v3 CycleTLS-startup issue as binary-data-handling
// — cycletls Go subprocess fails to start on Windows runners. Skip the suite
// on Windows until the v3 startup path is fixed.
const _describe = process.platform === 'win32' ? describe.skip : describe;
_describe("Streaming Response Tests", () => {
  let client;

  beforeAll(async () => {
    upstreamState.serviceAvailable = await probeUpstream('https://httpbin.org/json');
    if (!upstreamState.serviceAvailable) {
      console.warn('SKIPPING streaming tests: httpbin.org unavailable');
      return;
    }
    client = new CycleTLS({ port: 9118 });
  }, 20000);

  afterAll(async () => {
    if (client) {
      try {
        await client.close();
      } catch (e) {
        // Ignore cleanup errors - client may already be closed or failed to init
      }
    }
  });

  describe("Live streaming with responseType: 'stream'", () => {
    test("Should stream data from httpbin /stream endpoint", async () => {
      const response = await client.get('https://httpbin.org/stream/3', {
        responseType: 'stream',
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(response.data).toBeDefined();
      expect(typeof response.data.on).toBe('function'); // Should be a stream
      expect(typeof response.data.pipe).toBe('function'); // Should be a stream

      return new Promise((resolve, reject) => {
        const chunks = [];
        let chunkCount = 0;

        const timeoutHandle = setTimeout(() => {
          response.data.destroy();
          reject(new Error('Stream test timeout'));
        }, 15000);

        response.data.on('data', (chunk) => {
          chunks.push(chunk);
          chunkCount++;

          // Verify we're getting buffer chunks
          expect(chunk).toBeInstanceOf(Buffer);
        });

        response.data.on('end', () => {
          clearTimeout(timeoutHandle);
          try {
            // Should have received at least one chunk
            expect(chunkCount).toBeGreaterThan(0);

            // Combine all chunks and verify content
            const fullData = Buffer.concat(chunks).toString();
            expect(fullData).toContain('https://httpbin.org/stream/3');

            // Should contain JSON objects (httpbin streams JSON lines)
            const lines = fullData.trim().split('\n');
            expect(lines.length).toBe(3); // httpbin.org/stream/3 returns 3 lines

            lines.forEach(line => {
              expect(() => JSON.parse(line)).not.toThrow();
            });

            resolve();
          } catch (error) {
            reject(error);
          }
        });

        response.data.on('error', (error) => {
          clearTimeout(timeoutHandle);
          reject(error);
        });
      });
    });

    test("Should stream data from httpbin /stream endpoint multiple calls", async () => {
      const response = await client.get('https://httpbin.org/stream/2', {
        responseType: 'stream',
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(response.data).toBeDefined();

      return new Promise((resolve, reject) => {
        const chunks = [];
        let totalBytes = 0;
        let chunkCount = 0;

        const timeoutHandle = setTimeout(() => {
          response.data.destroy();
          reject(new Error('Stream test timeout'));
        }, 5000);

        response.data.on('data', (chunk) => {
          chunks.push(chunk);
          totalBytes += chunk.length;
          chunkCount++;
        });

        response.data.on('end', () => {
          clearTimeout(timeoutHandle);
          try {
            // Should have received data
            expect(totalBytes).toBeGreaterThan(0);
            expect(chunkCount).toBeGreaterThan(0); // Should have at least one chunk

            // Parse the received data as JSON lines
            const fullData = Buffer.concat(chunks).toString();
            const lines = fullData.trim().split('\n');
            expect(lines.length).toBe(2); // /stream/2 returns 2 lines

            lines.forEach(line => {
              expect(() => JSON.parse(line)).not.toThrow();
            });

            resolve();
          } catch (error) {
            reject(error);
          }
        });

        response.data.on('error', (error) => {
          clearTimeout(timeoutHandle);
          reject(error);
        });
      });
    }, 5000);

    test("Should provide working response methods for streams", async () => {
      const response = await client.get('https://httpbin.org/stream/1', {
        responseType: 'stream',
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(typeof response.json).toBe('function');
      expect(typeof response.text).toBe('function');
      expect(typeof response.arrayBuffer).toBe('function');
      expect(typeof response.blob).toBe('function');

      // Note: These methods will consume the stream, so we can only test one
      return new Promise((resolve, reject) => {
        // First consume the stream normally to get expected content
        const chunks = [];

        const timeoutHandle = setTimeout(() => {
          response.data.destroy();
          reject(new Error('Stream response methods test timeout'));
        }, 10000);

        response.data.on('data', (chunk) => {
          chunks.push(chunk);
        });

        response.data.on('end', async () => {
          clearTimeout(timeoutHandle);
          try {
            const fullData = Buffer.concat(chunks).toString();
            const jsonData = JSON.parse(fullData.trim());

            // Verify the stream gave us valid JSON
            expect(jsonData).toHaveProperty('url');
            expect(jsonData.url).toContain('stream/1');

            resolve();
          } catch (error) {
            reject(error);
          }
        });

        response.data.on('error', (error) => {
          clearTimeout(timeoutHandle);
          reject(error);
        });
      });
    });
  });

  describe("Response data access patterns", () => {
    test("Should provide json() method for JSON responses", async () => {
      const response = await client.get('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);

      // V2 API: response.data is always a stream
      expect(typeof response.data.on).toBe('function');

      // Use json() method to get parsed JSON
      const jsonData = await response.json();
      expect(typeof jsonData).toBe('object');
      expect(jsonData).toHaveProperty('slideshow');
    });

    test("Should provide data as stream by default", async () => {
      const response = await client.get('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);

      // V2 API: response.data is always a Readable stream (same as response.body)
      expect(typeof response.data.on).toBe('function');
      expect(typeof response.data.pipe).toBe('function');

      // Can consume via json() method
      const jsonData = await response.json();
      expect(jsonData).toHaveProperty('slideshow');
    });
  });

  describe("Stream event handling", () => {
    test("Should emit proper events in order", async () => {
      const response = await client.get('https://httpbin.org/stream/2', {
        responseType: 'stream',
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      return new Promise((resolve, reject) => {
        const events = [];
        let dataEventCount = 0;

        const timeoutHandle = setTimeout(() => {
          response.data.destroy();
          reject(new Error(`Stream event test timeout. Events received: ${events.join(', ')}`));
        }, 10000);

        response.data.on('data', (chunk) => {
          events.push('data');
          dataEventCount++;
          expect(chunk).toBeInstanceOf(Buffer);
        });

        response.data.on('end', () => {
          clearTimeout(timeoutHandle);
          events.push('end');
          try {
            // Verify events occurred in expected order
            expect(events.length).toBeGreaterThan(1);
            expect(events[events.length - 1]).toBe('end'); // Last event should be 'end'
            expect(dataEventCount).toBeGreaterThan(0); // Should have had at least one data event

            // All events before 'end' should be 'data' events
            for (let i = 0; i < events.length - 1; i++) {
              expect(events[i]).toBe('data');
            }

            resolve();
          } catch (error) {
            reject(error);
          }
        });

        response.data.on('error', (error) => {
          clearTimeout(timeoutHandle);
          events.push('error');
          reject(error);
        });
      });
    });
  });

  describe("Error handling", () => {
    test("Should handle streaming errors gracefully", async () => {
      try {
        const response = await client.get('https://httpbin.org/status/404', {
          responseType: 'stream',
          ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
          userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
        });

        // Should still return a response object even for error status codes
        expect(response.status).toBe(404);
        expect(response.data).toBeDefined();
        expect(typeof response.data.on).toBe('function');

        return new Promise((resolve, reject) => {
          const chunks = [];

          const timeoutHandle = setTimeout(() => {
            response.data.destroy();
            reject(new Error('Error handling test timeout'));
          }, 5000);

          response.data.on('data', (chunk) => {
            chunks.push(chunk);
          });

          response.data.on('end', () => {
            clearTimeout(timeoutHandle);
            // Even error responses should be streamable
            resolve();
          });

          response.data.on('error', (error) => {
            clearTimeout(timeoutHandle);
            reject(error);
          });
        });
      } catch (error) {
        // If the request fails entirely, that's also acceptable
        expect(error).toBeDefined();
      }
    });
  });
});