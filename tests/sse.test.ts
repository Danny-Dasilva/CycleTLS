import initCycleTLS, { CycleTLSSSEResponse, SSEEvent } from '../dist/index.js';
import * as http from 'http';
import { AddressInfo } from 'net';

// Set longer timeout for tests
jest.setTimeout(30000);

describe('Server-Sent Events Tests', () => {
  let cycleTLS: any;
  let sseServer: http.Server;
  let serverUrl: string;

  beforeAll(async () => {
    cycleTLS = await initCycleTLS();
    
    // Create a local SSE server for testing (similar to Go tests)
    sseServer = http.createServer((req, res) => {
      if (req.url === '/events') {
        // Set headers for SSE
        res.writeHead(200, {
          'Content-Type': 'text/event-stream',
          'Cache-Control': 'no-cache',
          'Connection': 'keep-alive',
          'Access-Control-Allow-Origin': '*',
        });

        // Send a few test events
        let eventCount = 0;
        const sendEvent = () => {
          eventCount++;
          res.write(`event: message\n`);
          res.write(`id: ${eventCount}\n`);
          res.write(`data: Event ${eventCount}\n\n`);

          if (eventCount < 3) {
            setTimeout(sendEvent, 100);
          } else {
            res.end();
          }
        };

        sendEvent();
      } else {
        res.writeHead(404);
        res.end();
      }
    });

    // Start server on random port
    await new Promise<void>((resolve) => {
      sseServer.listen(0, () => {
        const address = sseServer.address() as AddressInfo;
        serverUrl = `http://localhost:${address.port}`;
        resolve();
      });
    });
  });

  afterAll(async () => {
    await cycleTLS.exit();
    
    // Close the test server
    await new Promise<void>((resolve) => {
      sseServer.close(() => resolve());
    });
  });

  test('should connect to an SSE endpoint using regular GET request', async () => {
    // First test with regular GET request to see if the protocol works at all
    const response = await cycleTLS.get(`${serverUrl}/events`, {
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      headers: {
        'Accept': 'text/event-stream'
      }
    });

    
    // Check that we got a successful response
    expect(response.status).toBe(200);
    
    // Check content-type with case insensitive approach
    const contentType = response.headers['content-type'] || response.headers['Content-Type'];
    expect(contentType).toBeDefined();
    if (contentType) {
      const contentTypeStr = Array.isArray(contentType) ? contentType[0] : contentType;
      expect(contentTypeStr).toContain('text/event-stream');
    }
    
    // Parse SSE events from the response
    const streamData = await response.text();
    
    // Basic SSE parser
    const lines = streamData.split('\n');
    let currentEvent: Partial<SSEEvent> = {};
    const events: SSEEvent[] = [];
    
    for (const line of lines) {
      if (line.startsWith('event:')) {
        currentEvent.event = line.substring(6).trim();
      } else if (line.startsWith('id:')) {
        currentEvent.id = line.substring(3).trim();
      } else if (line.startsWith('data:')) {
        currentEvent.data = line.substring(5).trim();
      } else if (line === '') {
        // Empty line indicates end of event
        if (currentEvent.data) {
          events.push(currentEvent as SSEEvent);
          currentEvent = {};
        }
      }
    }

    // Check that we received events
    expect(events.length).toBeGreaterThan(0);
    if (events.length > 0) {
      expect(events[0].event).toBe('message');
      expect(events[0].data).toContain('Event');
    }
  });

  test('should handle SSE stream response with streaming', async () => {
    // Test streaming response type
    const response = await cycleTLS.get(`${serverUrl}/events`, {
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      responseType: 'stream',
      headers: {
        'Accept': 'text/event-stream'
      }
    });

    // Check response properties
    expect(response.status).toBe(200);
    
    // The response should contain SSE formatted data
    const responseText = await response.text();
    
    // Basic checks for SSE format
    expect(responseText).toBeDefined();
    expect(responseText.length).toBeGreaterThan(0);
    expect(responseText).toContain('event: message');
    expect(responseText).toContain('data: Event');
  });
});