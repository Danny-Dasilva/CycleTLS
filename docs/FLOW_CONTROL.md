# CycleTLS Streaming Protocol

## Overview

CycleTLS v3.0.0 uses a modern streaming protocol by default that provides credit-based backpressure for HTTP requests. This prevents memory explosion when downloading large files or handling high-throughput workloads.

## The Problem

The legacy protocol uses a single multiplexed WebSocket with no flow control:

```
Client ──── 1 WebSocket ────→ Server
            JSON messages       reads at max speed
            multiplexed         buffers fill up
                               → OOM crash
```

When downloading large files, the server reads and sends data as fast as possible. If the client can't keep up (slow disk, network bottleneck, processing delay), data buffers grow unbounded until the process runs out of memory.

## The Solution: Credit-Based Flow Control

The modern CycleTLS protocol introduces TCP-like flow control at the application layer:

```
Client ──── 1 WebSocket/Request ────→ Server
            Binary protocol            waits for credits
            Credit flow control        before sending
                                      → bounded memory
```

**How it works:**

1. Client opens WebSocket with initial credit window (e.g., 64KB)
2. Server sends data up to the credit limit, then **blocks**
3. Client consumes data and sends `credit` packets to replenish
4. Server resumes sending when credits are available

This ensures the server never sends more data than the client can handle.

## Usage

### TypeScript / JavaScript

```typescript
import CycleTLS from 'cycletls';

// Create a CycleTLS client
const client = new CycleTLS({
  port: 9119,              // Server port (default: 9119)
  initialWindow: 65536,    // Initial credit window in bytes (default: 64KB)
  creditThreshold: 32768,  // Replenish credits when this many bytes consumed
  autoSpawn: true,         // Auto-start server if not running
  debug: false,            // Enable debug logging
  timeout: 30000,          // Request timeout in ms
});

// Make a request with flow control
const response = await client.request({
  url: 'https://example.com/large-file.zip',
  method: 'GET',
  headers: { 'User-Agent': 'MyApp/1.0' },
  ja3: '771,4865-4866-4867...',  // Optional JA3 fingerprint
});

console.log(`Status: ${response.statusCode}`);
console.log(`Final URL: ${response.finalUrl}`);

// Stream the response body with backpressure
for await (const chunk of response.body) {
  await processChunk(chunk);  // Slow processing is OK!
  // Credits are automatically replenished as you consume data
}

// Clean up when done
await client.close();
```

### Convenience Methods

```typescript
// GET request
const response = await client.get('https://example.com/data');

// POST request
const response = await client.post('https://api.example.com/upload', jsonBody, {
  headers: { 'Content-Type': 'application/json' }
});
```

### Request Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `url` | string | required | Target URL |
| `method` | string | `"GET"` | HTTP method |
| `headers` | object | `{}` | Request headers |
| `body` | string | `""` | Request body |
| `ja3` | string | - | JA3 fingerprint |
| `ja4r` | string | - | JA4R raw fingerprint |
| `userAgent` | string | - | User-Agent header |
| `proxy` | string | - | Proxy URL |
| `timeout` | number | - | Request timeout (ms) |
| `disableRedirect` | boolean | `false` | Don't follow redirects |
| `insecureSkipVerify` | boolean | `false` | Skip TLS verification |
| `forceHTTP1` | boolean | `false` | Force HTTP/1.1 |
| `forceHTTP3` | boolean | `false` | Force HTTP/3 (QUIC) |

### Client Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `port` | number | `9119` | Server port |
| `initialWindow` | number | `65536` | Initial credit window (bytes) |
| `creditThreshold` | number | `initialWindow/2` | When to replenish credits |
| `autoSpawn` | boolean | `true` | Auto-start server |
| `executablePath` | string | - | Custom server executable path |
| `debug` | boolean | `false` | Enable debug logging |
| `timeout` | number | `30000` | Default request timeout (ms) |

## Protocol Details

### Version Routing

The Go server uses a query parameter for version routing:

| URL | Protocol | Description |
|-----|----------|-------------|
| `ws://localhost:9119` | Modern (default) | Binary, flow control, 1 WS/request |
| `ws://localhost:9119?v=2` | Modern | Explicit modern selection |
| `ws://localhost:9119?v=1` | Legacy | JSON, multiplexed, no flow control |

### Binary Frame Format

The modern protocol uses a length-prefixed binary format:

**Init Packet (Client → Server):**
```
[2B: requestId.length][requestId string]
[2B: "init".length]["init"]
[4B: initial credit window]
[2B: options.length][JSON options]
```

**Credit Packet (Client → Server):**
```
[2B: requestId.length][requestId string]
[2B: "credit".length]["credit"]
[4B: credit amount]
```

**Response Frame (Server → Client):**
```
[2B: requestId.length][requestId string]
[2B: "response".length]["response"]
[4B: payload.length][JSON: {statusCode, finalUrl, headers}]
```

**Data Frame (Server → Client):**
```
[2B: requestId.length][requestId string]
[2B: "data".length]["data"]
[4B: chunk.length][chunk bytes]
```

**End Frame (Server → Client):**
```
[2B: requestId.length][requestId string]
[2B: "end".length]["end"]
```

**Error Frame (Server → Client):**
```
[2B: requestId.length][requestId string]
[2B: "error".length]["error"]
[4B: payload.length][JSON: {statusCode, message}]
```

## Migrating from v2.x

The API has changed significantly in v3.0.0. Here's how to migrate:

### Before (v2.x - Buffered)

```typescript
import initCycleTLS from 'cycletls';

const cycleTLS = await initCycleTLS();
const response = await cycleTLS('https://example.com/large-file.zip', {
  ja3: '771,4865-4867-4866-49195...',
  userAgent: 'Mozilla/5.0...',
});

// Entire body buffered in memory - may OOM on large files!
console.log(response.body);
await cycleTLS.exit();
```

### After (v3.0.0 - Streaming)

```typescript
import CycleTLS from 'cycletls';

const client = new CycleTLS();
const response = await client.request({
  url: 'https://example.com/large-file.zip',
  ja3: '771,4865-4867-4866-49195...',
  userAgent: 'Mozilla/5.0...',
});

// Stream chunks - memory stays bounded
for await (const chunk of response.body) {
  process(chunk);
}
await client.close();
```

### Key Migration Points

| Aspect | v2.x (Old) | v3.0.0 (New) |
|--------|--------------|--------------|
| Import | `import initCycleTLS from 'cycletls'` | `import CycleTLS from 'cycletls'` |
| Initialize | `await initCycleTLS()` | `new CycleTLS()` |
| Request | `cycleTLS(url, options)` | `client.request({ url, ...options })` |
| Response body | `response.body` (string) | `response.body` (stream) |
| Cleanup | `cycleTLS.exit()` | `client.close()` |
| Memory | Unbounded (entire response) | Bounded (credit window) |

## Architecture

```
TypeScript new CycleTLS()
    │
    └── WebSocket (per request)
            │
            ▼
        Go Server (handleWSRequestV2)
            │
            ├── errgroup coordination
            │
            ├── creditWindow semaphore
            │
            └── context cancellation
```

## Performance Considerations

- **WebSocket overhead**: ~1-3ms per request for localhost connections
- **Credit window size**: Larger = higher throughput, smaller = less memory
- **Credit threshold**: Lower = more frequent credit packets, smoother flow

**Recommended settings:**
- Standard downloads: `initialWindow: 65536` (64KB)
- Large file streaming: `initialWindow: 262144` (256KB)
- Memory-constrained: `initialWindow: 16384` (16KB)

## Troubleshooting

### Server not responding
```typescript
const client = new CycleTLS({ debug: true });
// Check console for connection logs
```

### Request timeout
```typescript
const client = new CycleTLS({ timeout: 60000 }); // 60 seconds
```

### Memory still growing
- Ensure you're consuming the response body stream
- Use smaller `initialWindow` if client processing is slow
- Check that `creditThreshold` triggers replenishment

## Files Reference

### Go Server
| File | Purpose |
|------|---------|
| `cycletls/flow_control.go` | Credit window semaphore with context support |
| `cycletls/packet_builder.go` | Binary frame encoding |
| `cycletls/packet_reader.go` | Binary frame decoding |
| `cycletls/ws_handler_v2.go` | V2 request handler with errgroup |
| `cycletls/index.go:1480-1499` | Version routing logic |

### TypeScript Client
| File | Purpose |
|------|---------|
| `src/flow-control-client.ts` | CycleTLS class |
| `src/protocol.ts` | Binary protocol helpers |
| `src/credit-manager.ts` | Credit replenishment logic |

### Tests
| File | Coverage |
|------|----------|
| `cycletls/flow_control_test.go` | Credit window unit tests |
| `cycletls/packet_test.go` | Protocol encoding/decoding |
| `tests/flow-control.test.ts` | TypeScript integration tests |

## WebSocket Command Buffering

### Buffer Size: 32 Messages

The WebSocket V2 handler uses a buffered channel for command messages:

```go
// cycletls/ws_handler_v2.go
wsCommandCh = make(chan WebSocketCommandV2, 32)
```

**Rationale:**
- **32 messages** provides backpressure while allowing burst handling
- Balances memory usage vs. throughput for typical request patterns
- When buffer fills, sender blocks (natural backpressure)
- Sized for burst scenarios (e.g., rapid WebSocket message sends)

**Behavior when full:**
- Sender goroutine blocks until space available
- This is intentional backpressure, not an error condition
- Prevents unbounded memory growth from slow consumers

**Tuning considerations:**
- Increase for high-throughput WebSocket applications with bursty sends
- Decrease for memory-constrained environments
- Current value is suitable for most use cases

**Related:** See `initialWindow` and `creditThreshold` in CycleTLSOptions for response body flow control.
