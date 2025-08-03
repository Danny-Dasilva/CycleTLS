import initCycleTLS, { CycleTLSWebSocketResponse } from '../src/index';

describe('WebSocket Tests', () => {
  let cycleTLS: any;

  beforeAll(async () => {
    cycleTLS = await initCycleTLS();
  });

  afterAll(async () => {
    await cycleTLS.exit();
  });

  // Skip this test for now until the WebSocket functionality is fully implemented
  test.skip('should connect to a WebSocket server and exchange messages', async () => {
    // Connect to the WebSocket echo server
    const wsResponse: CycleTLSWebSocketResponse = await cycleTLS.ws('wss://echo.websocket.org', {
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    });

    // Prepare for receiving messages
    const messages: string[] = [];
    
    // Set up message handler
    wsResponse.onMessage((message) => {
      if (message.type === 'text') {
        messages.push(message.data.toString());
      }
    });

    // Send a test message
    const testMessage = 'Hello, WebSocket!';
    await wsResponse.send(testMessage);

    // Wait for the response
    await new Promise(resolve => setTimeout(resolve, 1000));

    // Check that we received the echo
    expect(messages).toContain(testMessage);

    // Close the connection
    await wsResponse.close();
  });
});