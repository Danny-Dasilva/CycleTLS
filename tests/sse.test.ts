import initCycleTLS, { CycleTLSSSEResponse, SSEEvent } from '../src/index';

describe('Server-Sent Events Tests', () => {
  let cycleTLS: any;

  beforeAll(async () => {
    cycleTLS = await initCycleTLS();
  });

  afterAll(async () => {
    await cycleTLS.exit();
  });

  // Skip this test for now until the SSE functionality is fully implemented
  test.skip('should connect to an SSE endpoint and receive events', async () => {
    // Connect to a public SSE server
    const sseResponse: CycleTLSSSEResponse = await cycleTLS.sse('https://example.com/events', {
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    });

    // Prepare for receiving events
    const events: SSEEvent[] = [];
    
    // Set up event handler
    sseResponse.onEvent((event) => {
      events.push(event);
    });

    // Wait for a few events
    await new Promise(resolve => setTimeout(resolve, 5000));

    // Close the connection
    await sseResponse.close();

    // Check that we received at least one event
    expect(events.length).toBeGreaterThan(0);
  });

  // Alternative implementation using async iterator
  test.skip('should read SSE events using async iterator', async () => {
    // Connect to a public SSE server
    const sseResponse: CycleTLSSSEResponse = await cycleTLS.sse('https://example.com/events', {
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    });

    // Read events using async iterator
    const events: SSEEvent[] = [];
    for await (const event of sseResponse.events()) {
      events.push(event);
      
      // Break after receiving a few events
      if (events.length >= 3) {
        break;
      }
    }

    // Close the connection
    await sseResponse.close();

    // Check that we received the expected number of events
    expect(events.length).toBe(3);
  });
});