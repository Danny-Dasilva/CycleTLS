# CycleTLS v2.0.0 Breaking Changes

This document outlines the breaking changes introduced in CycleTLS v2.0.0 and provides migration guidance for existing users.

## Overview

Version 2.0.0 introduces significant improvements to the CycleTLS API, making it more consistent with modern JavaScript fetch APIs while maintaining backward compatibility where possible. The main changes focus on response handling, streaming support, real-time communication protocols (WebSocket & SSE), and initialization options.

### New Protocol Support

CycleTLS v2.0.0 expands beyond HTTP to support real-time communication protocols:

- **WebSocket Support**: Full WebSocket client implementation with TLS fingerprinting
- **Server-Sent Events (SSE)**: Native SSE support for real-time event streaming
- **HTTP/3 & QUIC**: Enhanced HTTP/3 protocol support with custom QUIC fingerprinting
- **Enhanced Streaming**: Improved streaming response handling for all protocols

## Major Breaking Changes

### 1. Response Object API Changes

#### Before (v1.x)
```javascript
const response = await cycleTLS('https://httpbin.org/json', options);
console.log(response.body); // Direct access to body
console.log(response.status); // Status code
```

#### After (v2.0.0)
```javascript
const response = await cycleTLS('https://httpbin.org/json', options);
const data = await response.json(); // Parse as JSON
// OR
const text = await response.text(); // Get as text
// OR
const buffer = await response.arrayBuffer(); // Get as ArrayBuffer
// OR
const blob = await response.blob(); // Get as Blob

console.log(response.status); // Status code (unchanged)
```

**Migration**: Replace direct `response.body` access with appropriate response methods.

### 2. New Response Methods

Version 2.0.0 introduces Fetch API-compatible response methods:

- `response.json()` - Parse response as JSON
- `response.text()` - Get response as text string
- `response.arrayBuffer()` - Get response as ArrayBuffer
- `response.blob()` - Get response as Blob

All methods can be called multiple times on the same response object.

### 3. Streaming Support

#### New Feature (v2.0.0)
```javascript
const response = await cycleTLS('https://httpbin.org/stream/3', {
  responseType: 'stream'
});

const stream = response.data;
stream.on('data', chunk => {
  console.log('Received:', chunk.toString());
});

stream.on('end', () => {
  console.log('Stream complete');
  cycleTLS.exit();
});
```

**New Options**:
- `responseType: 'stream'` - Returns a Node.js readable stream
- `responseType: 'json'` - Pre-parses response as JSON (default behavior)
- `responseType: 'text'` - Returns response as text

### 4. Initialization Changes

#### Before (v1.x)
```javascript
const cycleTLS = await initCycleTLS();
```

#### After (v2.0.0)
```javascript
// Basic initialization (unchanged)
const cycleTLS = await initCycleTLS();

// With configuration options (new)
const cycleTLS = await initCycleTLS({
  port: 9118,
  timeout: 30000
});
```

**New initialization options**:
- `port` - Specify port for internal communication
- `timeout` - Set timeout for requests

### 5. Method Shortcuts

Version 2.0.0 introduces method shortcuts similar to axios:

```javascript
// New method shortcuts
const response = await cycleTLS.get(url, options);
const response = await cycleTLS.post(url, options);
const response = await cycleTLS.put(url, options);
const response = await cycleTLS.delete(url, options);
const response = await cycleTLS.head(url, options);
const response = await cycleTLS.options(url, options);
const response = await cycleTLS.patch(url, options);

// Traditional method still works
const response = await cycleTLS(url, options, 'GET');
```

### 6. WebSocket Support

Version 2.0.0 introduces native WebSocket support with TLS fingerprinting:

```javascript
// New WebSocket connection methods
const wsResponse = await cycleTLS.ws('wss://echo.websocket.org', {
  ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
  userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0'
});

// Alternative method name
const wsResponse = await cycleTLS.webSocket(url, options);

// WebSocket operations (planned API)
wsResponse.onMessage((message) => {
  if (message.type === 'text') {
    console.log('Received:', message.data.toString());
  }
});

wsResponse.onClose(() => {
  console.log('Connection closed');
});

wsResponse.onError((error) => {
  console.error('WebSocket error:', error);
});

// Send messages
await wsResponse.send('Hello, WebSocket!');
await wsResponse.send(Buffer.from('binary data'));

// Close connection
await wsResponse.close();
```

### 7. Server-Sent Events (SSE) Support

Version 2.0.0 introduces Server-Sent Events support for real-time data streaming:

```javascript
// New SSE connection methods
const sseResponse = await cycleTLS.sse('https://example.com/events', {
  ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
  userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
  responseType: 'stream' // For streaming events
});

// Alternative method name
const sseResponse = await cycleTLS.eventSource(url, options);

// Event-based approach (planned API)
sseResponse.onEvent((event) => {
  console.log('Event type:', event.event);
  console.log('Event data:', event.data);
  console.log('Event ID:', event.id);
});

sseResponse.onError((error) => {
  console.error('SSE error:', error);
});

// Async iterator approach (planned API)
for await (const event of sseResponse.events()) {
  console.log('Event:', event.data);
  if (event.data === 'done') break;
}

// Close connection
await sseResponse.close();

// Basic streaming approach (currently available)
const response = await cycleTLS('https://example.com/events', {
  responseType: 'stream',
  headers: {
    'Accept': 'text/event-stream',
    'Cache-Control': 'no-cache'
  }
});

const stream = response.data;
stream.on('data', (chunk) => {
  const lines = chunk.toString().split('\n');
  // Parse SSE format manually
  lines.forEach(line => {
    if (line.startsWith('data:')) {
      console.log('Event data:', line.substring(5).trim());
    }
  });
});
```

### 6. Form Data Handling

#### Before (v1.x)
```javascript
const response = await cycleTLS('http://httpbin.org/post', {
  body: formData,
  headers: {
    'Content-Type': 'multipart/form-data',
  },
}, 'post');
```

#### After (v2.0.0)
```javascript
const response = await cycleTLS('http://httpbin.org/post', {
  body: formData,
  headers: formData.getHeaders(), // Use formData.getHeaders()
}, 'post');

const data = await response.json(); // Parse response
```

**Migration**: Use `formData.getHeaders()` instead of manual content-type headers.

### 8. New Protocol APIs

#### WebSocket Connections (New in v2.0.0)
```javascript
// No equivalent in v1.x - completely new feature
const wsResponse = await cycleTLS.ws('wss://echo.websocket.org', {
  ja3: 'your_ja3_string',
  userAgent: 'your_user_agent'
});

// Check connection status
console.log('WebSocket status:', wsResponse.status); // Should be 101 for successful upgrade
```

#### Server-Sent Events (New in v2.0.0)
```javascript
// No equivalent in v1.x - completely new feature
const sseResponse = await cycleTLS.sse('https://example.com/events', {
  ja3: 'your_ja3_string',
  userAgent: 'your_user_agent',
  responseType: 'stream'
});

// Parse SSE events from response
const eventData = await sseResponse.text();
console.log('SSE events:', eventData);
```

## New API Methods (v2.0.0)

### Complete List of New Methods

#### HTTP Method Shortcuts
```javascript
// All return standard CycleTLS response objects
const response = await cycleTLS.get(url, options);
const response = await cycleTLS.post(url, options);
const response = await cycleTLS.put(url, options);
const response = await cycleTLS.delete(url, options);
const response = await cycleTLS.head(url, options);
const response = await cycleTLS.options(url, options);
const response = await cycleTLS.patch(url, options);

// Remember: Still need to parse responses!
const data = await response.json();
```

#### WebSocket Methods
```javascript
// Both methods are equivalent
const wsResponse = await cycleTLS.ws(url, options);
const wsResponse = await cycleTLS.webSocket(url, options);

// Returns response with status 101 for successful WebSocket upgrade
console.log(wsResponse.status); // 101
console.log(wsResponse.headers); // WebSocket headers
```

#### Server-Sent Events Methods
```javascript
// Both methods are equivalent
const sseResponse = await cycleTLS.sse(url, options);
const sseResponse = await cycleTLS.eventSource(url, options);

// Use with responseType: 'stream' for real-time events
const sseResponse = await cycleTLS.sse(url, {
  ...options,
  responseType: 'stream'
});
```

#### Response Methods (Apply to All Requests)
```javascript
// All response objects now have these methods
const jsonData = await response.json();
const textData = await response.text();
const arrayBuffer = await response.arrayBuffer();
const blob = await response.blob();

// For streaming responses
if (options.responseType === 'stream') {
  const stream = response.data;
  stream.on('data', chunk => { /* handle chunk */ });
  stream.on('end', () => { /* stream complete */ });
}
```

#### Enhanced Initialization
```javascript
// New configuration options
const cycleTLS = await initCycleTLS({
  port: 9118,           // Custom port for internal communication
  timeout: 30000,       // Default timeout for requests
  debug: false          // Enable debug logging
});
```

### API Usage Examples

#### Complete WebSocket Example
```javascript
const initCycleTLS = require('cycletls');

(async () => {
  const cycleTLS = await initCycleTLS();
  
  try {
    // Establish WebSocket connection
    const wsResponse = await cycleTLS.ws('wss://echo.websocket.org', {
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    });
    
    // Verify successful connection
    if (wsResponse.status === 101) {
      console.log('WebSocket connected successfully');
      console.log('Connection headers:', wsResponse.headers);
    }
    
  } catch (error) {
    console.error('WebSocket connection failed:', error.message);
  }
  
  cycleTLS.exit();
})();
```

#### Complete SSE Example
```javascript
const initCycleTLS = require('cycletls');

(async () => {
  const cycleTLS = await initCycleTLS();
  
  try {
    // Connect to SSE endpoint
    const sseResponse = await cycleTLS.sse('https://example.com/events', {
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    });
    
    // Parse SSE data
    const eventData = await sseResponse.text();
    console.log('Received SSE events:');
    
    // Basic SSE parser
    const lines = eventData.split('\n');
    let currentEvent = {};
    
    lines.forEach(line => {
      if (line.startsWith('event:')) {
        currentEvent.event = line.substring(6).trim();
      } else if (line.startsWith('data:')) {
        currentEvent.data = line.substring(5).trim();
      } else if (line.startsWith('id:')) {
        currentEvent.id = line.substring(3).trim();
      } else if (line === '') {
        // End of event
        if (currentEvent.data) {
          console.log('Event:', currentEvent);
          currentEvent = {};
        }
      }
    });
    
  } catch (error) {
    console.error('SSE connection failed:', error.message);
  }
  
  cycleTLS.exit();
})();
```

## Non-Breaking Changes

### Backward Compatibility

The following features remain unchanged and fully backward compatible:

- Basic request syntax: `cycleTLS(url, options, method)`
- All existing request options (ja3, userAgent, proxy, etc.)
- Response properties: `status`, `headers`, `finalUrl`
- Exit method: `cycleTLS.exit()`

### Enhanced Features

- **Connection Reuse**: Better connection reuse for improved performance
- **Error Handling**: Improved error messages and handling
- **TypeScript Support**: Enhanced TypeScript definitions
- **Testing**: Comprehensive test coverage

## Migration Guide

### Step 1: Update Response Handling

Replace all instances of `response.body` with appropriate response methods:

```javascript
// Old
console.log(response.body);

// New - for JSON responses
const data = await response.json();
console.log(data);

// New - for text responses
const text = await response.text();
console.log(text);
```

### Step 2: Update Form Data

Replace manual content-type headers with `formData.getHeaders()`:

```javascript
// Old
headers: {
  'Content-Type': 'multipart/form-data',
}

// New
headers: formData.getHeaders()
```

### Step 3: Leverage New Features

Consider using new features for improved functionality:

```javascript
// Use method shortcuts for cleaner code
const response = await cycleTLS.get(url, options);
const data = await response.json(); // Don't forget!

// Use streaming for large responses
const response = await cycleTLS.get(url, { responseType: 'stream' });

// Configure initialization for your needs
const cycleTLS = await initCycleTLS({ port: 9118, timeout: 30000 });

// Use WebSocket connections
const wsResponse = await cycleTLS.ws('wss://echo.websocket.org', options);

// Use Server-Sent Events
const sseResponse = await cycleTLS.sse('https://example.com/events', {
  ...options,
  responseType: 'stream'
});
```

### Step 4: Update Error Handling

Response method calls are async and can throw errors:

```javascript
try {
  const data = await response.json();
  console.log(data);
} catch (error) {
  console.error('Failed to parse JSON:', error.message);
}
```

## Testing Your Migration

1. **Functional Tests**: Ensure all existing functionality works with new response methods
2. **Performance Tests**: Verify that streaming and connection reuse provide expected benefits
3. **Error Handling**: Test error scenarios with new async response methods
4. **TypeScript**: Verify TypeScript compilation if using TypeScript

## Benefits of Migration

- **Modern API**: Consistent with fetch API standards
- **Real-time Communications**: Native WebSocket and SSE support with TLS fingerprinting
- **Streaming Support**: Handle large responses efficiently
- **Better Performance**: Improved connection reuse
- **Enhanced Developer Experience**: Method shortcuts and better error handling
- **Protocol Versatility**: HTTP/1.1, HTTP/2, HTTP/3, WebSocket, and SSE in one library
- **Future-Proof**: Aligns with modern JavaScript patterns

## Support

For questions about migration or issues with v2.0.0, please:

1. Check the updated examples in README.md
2. Review the test files in `/tests` directory for usage patterns
3. Open an issue on GitHub with specific migration questions

## Conclusion

CycleTLS v2.0.0 represents a significant improvement in API design and functionality while maintaining backward compatibility for core features. The migration primarily involves updating response handling patterns, which provides better alignment with modern JavaScript standards and enhanced capabilities like streaming support.