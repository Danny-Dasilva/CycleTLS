const initCycleTLS = require("../dist/index.js");

describe("Streaming Response Tests", () => {
  let cycleTLS;

  beforeAll(async () => {
    cycleTLS = await initCycleTLS({ port: 9118 });
  }, 20000);

  afterAll(async () => {
    if (cycleTLS) {
      await cycleTLS.exit();
    }
  });

  describe("Live streaming with responseType: 'stream'", () => {
    test("Should stream data from httpbin /stream endpoint", async () => {
      const response = await cycleTLS('https://httpbin.org/stream/3', {
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

        response.data.on('data', (chunk) => {
          chunks.push(chunk);
          chunkCount++;
          
          // Verify we're getting buffer chunks
          expect(chunk).toBeInstanceOf(Buffer);
        });

        response.data.on('end', () => {
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
          reject(error);
        });

        // Timeout after 15 seconds
        setTimeout(() => {
          reject(new Error('Stream test timeout'));
        }, 15000);
      });
    });

    test("Should stream data from httpbin /stream endpoint multiple calls", async () => {
      const response = await cycleTLS('https://httpbin.org/stream/2', {
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

        response.data.on('data', (chunk) => {
          chunks.push(chunk);
          totalBytes += chunk.length;
          chunkCount++;
          console.log(`Stream chunk ${chunkCount}: ${chunk.length} bytes`);
        });

        response.data.on('end', () => {
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
            
            console.log(`Stream test completed: ${chunkCount} chunks, ${totalBytes} total bytes`);
            resolve();
          } catch (error) {
            reject(error);
          }
        });

        response.data.on('error', (error) => {
          reject(error);
        });

        // Timeout after 2 seconds
        setTimeout(() => {
          reject(new Error('Stream test timeout'));
        }, 2000);
      });
    }, 5000);

    test("Should provide working response methods for streams", async () => {
      const response = await cycleTLS('https://httpbin.org/stream/1', {
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
        
        response.data.on('data', (chunk) => {
          chunks.push(chunk);
        });

        response.data.on('end', async () => {
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

        response.data.on('error', reject);
      });
    });
  });

  describe("Backward compatibility - non-stream responses", () => {
    test("Should work normally with responseType: 'json'", async () => {
      const response = await cycleTLS('https://httpbin.org/json', {
        responseType: 'json',
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(typeof response.data).toBe('object');
      expect(response.data).toHaveProperty('slideshow');
      
      // Response methods should still work
      const jsonData = await response.json();
      expect(jsonData).toEqual(response.data);
    });

    test("Should work normally with default responseType", async () => {
      const response = await cycleTLS('https://httpbin.org/json', {
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      expect(response.status).toBe(200);
      expect(typeof response.data).toBe('object');
      expect(response.data).toHaveProperty('slideshow');
    });
  });

  describe("Stream event handling", () => {
    test("Should emit proper events in order", async () => {
      const response = await cycleTLS('https://httpbin.org/stream/2', {
        responseType: 'stream',
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      });

      return new Promise((resolve, reject) => {
        const events = [];
        let dataEventCount = 0;

        response.data.on('data', (chunk) => {
          events.push('data');
          dataEventCount++;
          expect(chunk).toBeInstanceOf(Buffer);
        });

        response.data.on('end', () => {
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
          events.push('error');
          reject(error);
        });

        // Timeout
        setTimeout(() => {
          reject(new Error(`Stream event test timeout. Events received: ${events.join(', ')}`));
        }, 10000);
      });
    });
  });

  describe("Error handling", () => {
    test("Should handle streaming errors gracefully", async () => {
      try {
        const response = await cycleTLS('https://httpbin.org/status/404', {
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

          response.data.on('data', (chunk) => {
            chunks.push(chunk);
          });

          response.data.on('end', () => {
            // Even error responses should be streamable
            resolve();
          });

          response.data.on('error', reject);

          setTimeout(() => {
            reject(new Error('Error handling test timeout'));
          }, 5000);
        });
      } catch (error) {
        // If the request fails entirely, that's also acceptable
        expect(error).toBeDefined();
      }
    });
  });
});