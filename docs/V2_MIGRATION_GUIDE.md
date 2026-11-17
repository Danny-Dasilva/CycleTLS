# CycleTLS V2 Migration Guide

> **Note:** This file has been superseded by [V3_MIGRATION_GUIDE.md](./V3_MIGRATION_GUIDE.md). This version is retained for reference but all version references below refer to the pre-v3.0.0 naming scheme.

This guide helps you migrate from the legacy CycleTLS API (`initCycleTLS`) to the new streaming API (`new CycleTLS()`).

## Table of Contents

- [Quick Comparison](#quick-comparison)
- [API Initialization](#api-initialization)
- [Request Methods](#request-methods)
- [Response Object Differences](#response-object-differences)
- [Response Body Parsing](#response-body-parsing)
- [WebSocket API](#websocket-api)
- [Server-Sent Events (SSE)](#server-sent-events-sse)
- [Migration Examples](#migration-examples)
- [Breaking Changes Summary](#breaking-changes-summary)

## Quick Comparison

### Legacy API (v2.x and earlier)
```typescript
import initCycleTLS from 'cycletls';

const cycleTLS = await initCycleTLS();
const response = await cycleTLS('https://example.com', {
  ja3: '771,4865-4867-4866...',
  userAgent: 'Mozilla/5.0...'
});

console.log(response.status);    // HTTP status code
console.log(response.body);      // Full response body (string)
console.log(response.data);      // Same as body (parsed if JSON)
await cycleTLS.exit();
```

### Streaming API (v3.0.0)
```typescript
import CycleTLS from 'cycletls';

const client = new CycleTLS();
const response = await client.get('https://example.com', {
  ja3: '771,4865-4867-4866...',
  userAgent: 'Mozilla/5.0...'
});

console.log(response.statusCode); // HTTP status code
console.log(await response.text()); // Get full body as text
console.log(await response.json()); // Parse body as JSON
await client.close();
```

## API Initialization

### Legacy API
```typescript
import initCycleTLS from 'cycletls';

const cycleTLS = await initCycleTLS({
  port: 9119,
  debug: false,
  timeout: 20000,
  executablePath: '/path/to/binary'
});
```

### V2 API
```typescript
import CycleTLS from 'cycletls';

const client = new CycleTLS({
  port: 9119,
  debug: false,
  timeout: 30000,
  executablePath: '/path/to/binary',
  autoSpawn: true,           // NEW: Auto-start server
  initialWindow: 65536,      // NEW: Credit window size
  creditThreshold: 32768     // NEW: Credit replenishment
});
```

**Key Differences:**
- **No `await` needed**: V2 constructor is synchronous
- **New options**: `autoSpawn`, `initialWindow`, `creditThreshold` for flow control
- **Timeout unit**: Legacy uses milliseconds; V2 also uses milliseconds (both)

## Request Methods

### Legacy API

#### Method 1: Callable interface
```typescript
const response = await cycleTLS(url, options);
const response = await cycleTLS(url, options, 'get');
```

#### Method 2: Named methods
```typescript
const response = await cycleTLS.get(url, options);
const response = await cycleTLS.post(url, options);
```

### V2 API

#### Method 1: `.request()` method
```typescript
const response = await client.request({
  url: 'https://example.com',
  method: 'GET',
  headers: { 'Accept': 'application/json' }
});
```

#### Method 2: Convenience methods
```typescript
const response = await client.get(url, options);
const response = await client.post(url, body, options);
```

**Key Differences:**
- **No callable interface**: V2 doesn't support `client(url, options)`
- **Options object**: V2 `.request()` takes a single options object with `url` and `method` inside
- **POST body**: V2 `post(url, body, options)` has explicit `body` parameter

## Response Object Differences

### Property Mapping Table

| Legacy Property | V2 Property | Notes |
|----------------|-------------|-------|
| `response.status` | `response.statusCode` | V2 standardizes on `statusCode` |
| `response.body` | `await response.text()` | V2 body is async method, not property |
| `response.data` | `await response.json()` | V2 uses method instead of property |
| `response.headers` | `response.headers` | Same name, different format (see below) |
| `response.finalUrl` | `response.finalUrl` | Unchanged |
| N/A | `response.requestId` | NEW: Unique request identifier |

### Headers Format

**Legacy API:**
```typescript
response.headers: Record<string, string | string[]>
// Example:
{
  'content-type': 'application/json',
  'set-cookie': ['cookie1=value1', 'cookie2=value2']
}
```

**V2 API:**
```typescript
response.headers: Record<string, string[]>
// All values are arrays:
{
  'content-type': ['application/json'],
  'set-cookie': ['cookie1=value1', 'cookie2=value2']
}
```

**Migration:**
```typescript
// Legacy
const contentType = response.headers['content-type']; // string or string[]

// V2
const contentType = response.headers['content-type'][0]; // Always an array
```

### Response Body

**Legacy API:**
```typescript
interface CycleTLSResponse {
  status: number;
  body: string;              // Entire response buffered in memory
  data: ResponseData;        // Parsed according to responseType
  headers: HttpHeaders;
  finalUrl: string;
}

// Direct access (already parsed):
console.log(response.data);  // JSON object if responseType: 'json'
console.log(response.body);  // Raw string
```

**V2 API:**
```typescript
interface Response {
  requestId: string;
  statusCode: number;
  finalUrl: string;
  headers: Record<string, string[]>;
  body: Readable;            // Stream (not buffered)
}

// Must call methods to parse:
const json = await response.json();
const text = await response.text();
const buffer = await response.buffer();
```

## Response Body Parsing

### Legacy API (Axios-style `responseType`)

```typescript
// JSON (default)
const response = await cycleTLS.get(url, { responseType: 'json' });
console.log(response.data); // Parsed JSON object

// Text
const response = await cycleTLS.get(url, { responseType: 'text' });
console.log(response.data); // String

// ArrayBuffer
const response = await cycleTLS.get(url, { responseType: 'arraybuffer' });
console.log(response.data); // ArrayBuffer

// Blob
const response = await cycleTLS.get(url, { responseType: 'blob' });
console.log(response.data); // Blob

// Stream
const response = await cycleTLS.get(url, { responseType: 'stream' });
for await (const chunk of response.data) {
  process(chunk);
}
```

### V2 API (Fetch-style methods)

```typescript
const response = await client.get(url, options);

// JSON
const data = await response.json();

// Text
const text = await response.text();

// ArrayBuffer
const buffer = await response.arrayBuffer();

// Blob
const blob = await response.blob();

// Stream (default - body is already a stream)
for await (const chunk of response.body) {
  process(chunk);
}
```

**Key Differences:**
- **No `responseType` option**: V2 always returns a stream in `response.body`
- **Call methods to parse**: Use `.json()`, `.text()`, etc. when you need the full body
- **Streaming by default**: V2 is streaming-first; legacy was buffering-first

### Migration: Response Type Handling

```typescript
// LEGACY: responseType: 'json'
const response = await cycleTLS.get(url, { responseType: 'json' });
const data = response.data;

// V2: Use .json() method
const response = await client.get(url);
const data = await response.json();
```

```typescript
// LEGACY: responseType: 'text'
const response = await cycleTLS.get(url, { responseType: 'text' });
const text = response.data;

// V2: Use .text() method
const response = await client.get(url);
const text = await response.text();
```

```typescript
// LEGACY: responseType: 'stream'
const response = await cycleTLS.get(url, { responseType: 'stream' });
for await (const chunk of response.data) {
  process(chunk);
}

// V2: Body is already a stream
const response = await client.get(url);
for await (const chunk of response.body) {
  process(chunk);
}
```

## WebSocket API

### Legacy API

```typescript
const ws = await cycleTLS.ws('wss://example.com/socket', {
  ja3: '771,4865-4867-4866...',
});

// Send message
await ws.send(JSON.stringify({ type: 'hello' }), false); // text
await ws.send(buffer, true); // binary

// Receive messages (callback-based)
ws.onMessage((msg) => {
  console.log(msg.type); // 'text' | 'binary'
  console.log(msg.data); // string | Buffer
});

ws.onClose((code, reason) => {
  console.log(`Closed: ${code} ${reason}`);
});

ws.onError((error) => {
  console.error(error);
});

// Close connection
await ws.close(1000, 'Done');
```

### V2 API (ws-library compatible)

```typescript
const ws = await client.ws('wss://example.com/socket', {
  ja3: '771,4865-4867-4866...',
});

// Send message (ws-library style)
ws.send('hello', (err) => {
  if (err) console.error(err);
});
ws.send(buffer, { binary: true });

// Receive messages (EventEmitter pattern)
ws.on('open', () => {
  console.log('Connected');
});

ws.on('message', (data, isBinary) => {
  if (isBinary) {
    console.log('Binary:', data); // Buffer
  } else {
    console.log('Text:', data.toString());
  }
});

ws.on('close', (code, reason) => {
  console.log(`Closed: ${code} ${reason}`);
});

ws.on('error', (error) => {
  console.error(error);
});

// Close connection
ws.close(1000, 'Done');
```

**Key Differences:**
- **EventEmitter pattern**: V2 uses `.on()` instead of `.onMessage()`
- **ws-library compatibility**: V2 matches the popular `ws` library API
- **Binary detection**: V2 provides `isBinary` flag in message callback
- **No await on send**: V2 `send()` is synchronous with optional callback
- **Additional methods**: V2 adds `.ping()`, `.pong()`, `.terminate()`

### WebSocket Properties

| Legacy | V2 | Notes |
|--------|----|----|
| N/A | `ws.url` | NEW: WebSocket URL |
| N/A | `ws.readyState` | NEW: 0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED |
| N/A | `ws.protocol` | NEW: Negotiated subprotocol |
| N/A | `ws.extensions` | NEW: Negotiated extensions |
| N/A | `ws.bufferedAmount` | NEW: Bytes queued but not sent |
| N/A | `ws.binaryType` | NEW: 'nodebuffer', 'arraybuffer', or 'fragments' |

## Server-Sent Events (SSE)

### Legacy API

```typescript
const sse = await cycleTLS.sse('https://example.com/events', {});

sse.onEvent((event) => {
  console.log(event.id);
  console.log(event.event);
  console.log(event.data);
  console.log(event.retry);
});

sse.onError((error) => {
  console.error(error);
});

await sse.close();
```

### V2 API

```typescript
const sse = await client.sse('https://example.com/events', {});

// Method 1: Async iterator
for await (const event of sse.events()) {
  console.log(event.id);
  console.log(event.event);
  console.log(event.data);
  console.log(event.retry);
}

// Method 2: Callback (same as legacy)
sse.onEvent((event) => {
  console.log(event);
});

sse.onError((error) => {
  console.error(error);
});

await sse.close();
```

**Key Differences:**
- **Async iterator**: V2 adds `sse.events()` for async iteration
- **Both patterns supported**: V2 supports both callbacks and async iterators

## Migration Examples

### Example 1: Simple GET Request

**Legacy:**
```typescript
import initCycleTLS from 'cycletls';

const cycleTLS = await initCycleTLS();
const response = await cycleTLS.get('https://api.example.com/data', {
  headers: { 'Authorization': 'Bearer token' }
});

console.log(response.status);
console.log(response.data); // Parsed JSON
await cycleTLS.exit();
```

**V2:**
```typescript
import CycleTLS from 'cycletls';

const client = new CycleTLS();
const response = await client.get('https://api.example.com/data', {
  headers: { 'Authorization': 'Bearer token' }
});

console.log(response.statusCode);
console.log(await response.json()); // Parsed JSON
await client.close();
```

### Example 2: POST Request with Body

**Legacy:**
```typescript
const response = await cycleTLS.post('https://api.example.com/users', {
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ name: 'Alice' })
});
```

**V2:**
```typescript
const response = await client.post(
  'https://api.example.com/users',
  JSON.stringify({ name: 'Alice' }),
  { headers: { 'Content-Type': 'application/json' } }
);
```

### Example 3: Streaming Large Files

**Legacy (buffered - risky for large files):**
```typescript
const response = await cycleTLS.get('https://example.com/large-file.zip', {
  responseType: 'stream'
});

// Still buffers entire response in memory first
for await (const chunk of response.data) {
  await writeToFile(chunk);
}
```

**V2 (true streaming with backpressure):**
```typescript
const response = await client.get('https://example.com/large-file.zip');

// Streams with flow control - memory bounded
for await (const chunk of response.body) {
  await writeToFile(chunk);
  // Server waits for client to consume before sending more
}
```

### Example 4: JA3 Fingerprinting

**Legacy:**
```typescript
const response = await cycleTLS('https://tls.peet.ws/api/all', {
  ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-34-51-43-13-45-28-21,29-23-24-25-256-257,0',
  userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
});
```

**V2:**
```typescript
const response = await client.request({
  url: 'https://tls.peet.ws/api/all',
  ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-34-51-43-13-45-28-21,29-23-24-25-256-257,0',
  userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
});
```

### Example 5: Handling Errors

**Legacy:**
```typescript
try {
  const response = await cycleTLS.get('https://example.com/404');
  if (response.status >= 400) {
    console.error(`HTTP ${response.status}: ${response.body}`);
  }
} catch (error) {
  console.error('Request failed:', error.message);
}
```

**V2:**
```typescript
try {
  const response = await client.get('https://example.com/404');
  if (response.statusCode >= 400) {
    console.error(`HTTP ${response.statusCode}: ${await response.text()}`);
  }
} catch (error) {
  if (error instanceof CycleTLSError) {
    console.error(`Error ${error.statusCode}: ${error.message}`);
  } else {
    console.error('Request failed:', error.message);
  }
}
```

## Breaking Changes Summary

### Imports
- **Changed**: `import initCycleTLS from 'cycletls'` → `import CycleTLS from 'cycletls'`

### Initialization
- **Changed**: `await initCycleTLS()` → `new CycleTLS()`
- **Added**: New options `autoSpawn`, `initialWindow`, `creditThreshold`

### Request Methods
- **Removed**: Callable interface `cycleTLS(url, options)`
- **Changed**: `.request()` takes single options object with `url` inside
- **Changed**: `.post(url, body, options)` has explicit `body` parameter

### Response Object
- **Changed**: `response.status` → `response.statusCode`
- **Changed**: `response.body` (string) → `response.body` (Readable stream)
- **Changed**: `response.data` (parsed) → `await response.json()` / `await response.text()`
- **Changed**: `response.headers` now always `Record<string, string[]>` (all values are arrays)
- **Added**: `response.requestId`

### Response Parsing
- **Removed**: `responseType` option
- **Changed**: Use methods `.json()`, `.text()`, `.arrayBuffer()`, `.blob()` instead
- **Changed**: Default is streaming (`.body` is a stream), not buffered

### WebSocket
- **Changed**: `.onMessage()` → `.on('message')`
- **Changed**: `.onClose()` → `.on('close')`
- **Changed**: `.onError()` → `.on('error')`
- **Added**: `.on('open')`, `.ping()`, `.pong()`, `.terminate()`
- **Added**: Properties: `.url`, `.readyState`, `.protocol`, `.extensions`

### SSE
- **Added**: `sse.events()` async iterator
- **Kept**: Callback-based `.onEvent()` and `.onError()` still work

### Cleanup
- **Changed**: `cycleTLS.exit()` → `client.close()`

## Migrating Step-by-Step

1. **Update import:**
   ```typescript
   // Before
   import initCycleTLS from 'cycletls';

   // After
   import CycleTLS from 'cycletls';
   ```

2. **Change initialization:**
   ```typescript
   // Before
   const cycleTLS = await initCycleTLS();

   // After
   const client = new CycleTLS();
   ```

3. **Update request calls:**
   ```typescript
   // Before
   const response = await cycleTLS.get(url, options);

   // After
   const response = await client.get(url, options);
   ```

4. **Fix response property names:**
   ```typescript
   // Before
   console.log(response.status);

   // After
   console.log(response.statusCode);
   ```

5. **Parse response body:**
   ```typescript
   // Before
   console.log(response.data);

   // After
   console.log(await response.json());
   ```

6. **Update cleanup:**
   ```typescript
   // Before
   await cycleTLS.exit();

   // After
   await client.close();
   ```

## Why Migrate?

The V2 API provides several advantages:

1. **Memory Efficiency**: Streaming with backpressure prevents OOM on large files
2. **Better Performance**: Credit-based flow control optimizes throughput
3. **Modern API**: Fetch-style response methods (`.json()`, `.text()`)
4. **Standards Compliance**: WebSocket API matches the popular `ws` library
5. **Future-Proof**: V2 is the foundation for future features

## Need Help?

- **Legacy API still works**: Use `?v=1` to connect to legacy protocol
- **Documentation**: See [FLOW_CONTROL.md](./FLOW_CONTROL.md) for V2 details
- **Issues**: Report migration problems at [GitHub Issues](https://github.com/Danny-Dasilva/CycleTLS/issues)
