# CycleTLS



<div align="center">
	<img src="docs/media/Banner.png" alt="CycleTLS"/>
	<br>
	
Accepting Community Support and PR's

	
	

![build](https://github.com/Danny-Dasilva/CycleTLS/actions/workflows/test_golang.yml/badge.svg)
[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg)](http://godoc.org/github.com/Danny-Dasilva/CycleTLS/cycletls) 
[![license](https://img.shields.io/github/license/Danny-Dasilva/CycleTLS.svg)](https://github.com/Danny-Dasilva/CycleTLS/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Danny-Dasilva/CycleTLS/cycletls)](https://goreportcard.com/report/github.com/Danny-Dasilva/CycleTLS/cycletls)
[![npm version](https://img.shields.io/npm/v/cycletls.svg)](https://www.npmjs.org/package/cycletls)
 <a href="https://discord.gg/gsmxMHrwhu">
  <img src="https://img.shields.io/discord/1100945880888115200?logo=discord"
	  alt="chat on Discord"></a>
</div>

If you have a API change or feature request feel free to open an [Issue](https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose)



# 🚀 Features

- [High-performance](#-performance) Built-in goroutine pool used for handling asynchronous requests
- Custom header ordering via [fhttp](https://github.com/useflyent/fhttp)
- Proxy support | Socks4, Socks5, Socks5h
- Ja3 Token configuration
- HTTP/3 and QUIC support
- WebSocket client
- Server-Sent Events (SSE)
- Connection reuse
- JA4 fingerprinting
- Flow Control / Backpressure for large downloads


Table of contents
=================


* [Table of contents](#table-of-contents)
* [Installation](#installation)
* [Usage](#usage)
	* [QuickStart JS](#example-cycletls-request-for-typescript-and-javascript)
	* [Quickstart Golang](#example-cycletls-request-for-golang)
	* [Initializing CycleTLS](#creating-an-instance)
	* [API/Methods](#cycletls-alias-methods)
	* [Request Config](#cycletls-request-config)
	* [Response Schema](#cycletls-response-schema)
	* [Multiple Requests Example](#multiple-requests-example-for-typescript-and-javascript)
	* [Streaming Responses](#streaming-responses-axios-style)
	* [Flow Control (V2 Protocol)](#flow-control-v2-protocol)
* [Local Setup](#dev-setup)
* [QA](#questions)
* [LICENSE](#license)

## Dependencies

```
node ^v18.0
golang ^v1.21x
```

## Installation

Node Js

```bash
$ npm install cycletls
```

Golang

```bash
$ go get github.com/Danny-Dasilva/CycleTLS/cycletls 
```

# Usage 

## Example CycleTLS Request for Typescript and Javascript

You can run this test in `tests/simple.test.ts`

```js
const CycleTLS = require('cycletls').default;
// Typescript: import CycleTLS from 'cycletls';

(async () => {
  // Create CycleTLS client
  const client = new CycleTLS();

  // Send request
  const response = await client.get('https://ja3er.com/json', {
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    proxy: 'http://username:password@hostname.com:443'
  });

  // Parse response using built-in helper methods
  const data = await response.json();
  console.log(data);

  // Cleanly close CycleTLS
  await client.close();
})();
```

## JA4R (Raw) TLS Fingerprinting

> **Important:** Pass `ja4r` to configure the TLS ClientHello. JA4 (hash) is a report-only value; configuring with a JA4 hash will not change your fingerprint.

JA4R is the raw format of JA4 fingerprinting that allows explicit configuration of cipher suites, extensions, and signature algorithms:


### JavaScript Example
```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  // Chrome JA4R fingerprint (raw format)
  const response = await client.get('https://tls.peet.ws/api/all', {
    ja4r: 't13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601'
  });

  // Parse response using built-in helper
  const data = await response.json();
  console.log('JA4:', data.tls.ja4);
  console.log('JA4_r:', data.tls.ja4_r);
  console.log('TLS Version:', data.tls.tls_version_negotiated);

  await client.close();
})();
```

### Golang JA4R Example
```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()

	// Chrome JA4R fingerprint (raw format)
	response, err := client.Do("https://tls.peet.ws/api/all", cycletls.Options{
		Ja4r: "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36",
	}, "GET")
	
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Response with JA4R:", response.Status)
}
```

## HTTP/2 Fingerprinting

HTTP/2 fingerprinting allows you to mimic specific browser HTTP/2 implementations:

### JavaScript Example
```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  // Firefox HTTP/2 fingerprint
  const response = await client.get('https://tls.peet.ws/api/all', {
    http2Fingerprint: '1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s',
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0'
  });

  // Parse response using built-in helper
  const data = await response.json();
  console.log('HTTP/2 Fingerprint:', data.http2.akamai_fingerprint);
  console.log('Settings:', data.http2.sent_frames[0].settings);

  await client.close();
})();
```

### Golang HTTP/2 Example
```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	// Firefox HTTP/2 fingerprint
	response, err := client.Do("https://tls.peet.ws/api/all", cycletls.Options{
		HTTP2Fingerprint: "1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0",
	}, "GET")
	
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Response with HTTP/2 fingerprint:", response.Status)
}
```

### Common Browser HTTP/2 Fingerprints

| Browser | HTTP/2 Fingerprint | Description |
|---------|-------------------|-------------|
| Firefox | `1:65536;2:0;4:131072;5:16384\|12517377\|0\|m,p,a,s` | Smaller window size, MPAS priority |
| Chrome | `1:65536;2:0;4:6291456;6:262144\|15663105\|0\|m,a,s,p` | Larger window size, MASP priority |

### Combined Fingerprinting Example
```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  // Complete Chrome browser fingerprint with JA4R
  const response = await client.get('https://tls.peet.ws/api/all', {
    ja4r: 't13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601',
    http2Fingerprint: '1:65536;2:0;4:131072;5:16384|12517377|0|m,p,a,s',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0'
  });

  // Parse response using built-in helper
  const data = await response.json();
  console.log('Complete fingerprint applied successfully');
  console.log('JA4:', data.tls.ja4);
  console.log('HTTP/2:', data.http2.akamai_fingerprint);

  await client.close();
})();
```

## Streaming Responses (Axios-style)

CycleTLS supports axios-compatible streaming responses for real-time data processing. In the streaming client, `response.data` is always a Readable stream (alias of `response.body`), and `responseType: 'stream'` is optional for compatibility.

### Basic Streaming Example

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  // Get streaming response
  const response = await client.get('https://httpbin.org/stream/3', {
    responseType: 'stream',
    headers: { Authorization: 'Bearer your_token_here' },
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0'
  });

  console.log('Status:', response.statusCode);

  // Stream body chunks as they arrive
  for await (const chunk of response.data) {
    console.log('Received chunk:', chunk.toString());
  }

  console.log('Stream done');
  await client.close();
})();
```

### Advanced Streaming with Error Handling

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  try {
    const response = await client.get('https://httpbin.org/drip?numbytes=100&duration=2', {
      responseType: 'stream',
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    });

    console.log('Status:', response.statusCode);
    console.log('Headers:', response.headers);

    const chunks = [];

    for await (const chunk of response.data) {
      chunks.push(chunk);
      console.log(`Received ${chunk.length} bytes`);
    }

    console.log('Stream complete');
    const fullData = Buffer.concat(chunks);
    console.log('Total received:', fullData.length, 'bytes');

  } catch (error) {
    console.error('Request failed:', error);
  } finally {
    await client.close();
  }
})();
```

### Consuming Response Bodies

With the streaming API, response bodies are consumed as streams. Here are common patterns:

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

// Helper to consume response body as string
async function consumeBody(body) {
  const chunks = [];
  for await (const chunk of body) chunks.push(chunk);
  return Buffer.concat(chunks).toString();
}

(async () => {
  const client = new CycleTLS();

  // JSON response
  const jsonResponse = await client.get('https://httpbin.org/json');
  const jsonText = await consumeBody(jsonResponse.body);
  const jsonData = JSON.parse(jsonText);
  console.log(jsonData); // Parsed JSON object

  // Text response
  const textResponse = await client.get('https://httpbin.org/html');
  const textData = await consumeBody(textResponse.body);
  console.log(textData); // String content

  await client.close();
})();
```

## Streaming & Backpressure (Default)

CycleTLS v3.0.0 uses a modern streaming protocol by default. This provides memory-efficient large file downloads with credit-based backpressure.

### Basic Usage

```typescript
import CycleTLS from 'cycletls';

const client = new CycleTLS();

// Streaming download with backpressure
const response = await client.get('https://example.com/large-file.zip');

console.log(`Status: ${response.statusCode}`);
console.log(`URL: ${response.finalUrl}`);

// Stream body - memory stays bounded regardless of file size
for await (const chunk of response.body) {
  await processChunk(chunk);
}

await client.close();
```

### POST Request

```typescript
import CycleTLS from 'cycletls';

const client = new CycleTLS();

const response = await client.post('https://api.example.com/upload',
  JSON.stringify({ data: 'value' }),
  {
    headers: { 'Content-Type': 'application/json' },
    ja3: '771,4865-4867-4866-49195...',
    userAgent: 'Mozilla/5.0...',
  }
);

for await (const chunk of response.body) {
  console.log(chunk.toString());
}

await client.close();
```

### Migrating from v2.x

If you're upgrading from v2.x or earlier, the API has changed. See the full [v3.0.0 Migration Guide](./docs/V3_MIGRATION_GUIDE.md) for details.

```typescript
// OLD (v2.x and earlier) - legacy API (still available as named export)
import { initCycleTLS } from 'cycletls';
const cycleTLS = await initCycleTLS();
const response = await cycleTLS('https://example.com', { ja3: '...' });
console.log(response.body); // Entire body buffered as string
await cycleTLS.exit();

// NEW (v3.0.0) - streaming with backpressure
import CycleTLS from 'cycletls';
const client = new CycleTLS();
const response = await client.get('https://example.com', { ja3: '...' });
for await (const chunk of response.body) {
  console.log(chunk.toString()); // Stream chunks as they arrive
}
await client.close();
```

**Key differences:**
- `new CycleTLS()` instead of `await initCycleTLS()`
- Response body is a stream, not a buffered string
- Use `client.get()`, `client.post()`, or `client.request()` methods
- Memory stays bounded regardless of response size

### CycleTLS Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `port` | number | `9119` | Server port |
| `initialWindow` | number | `65536` | Initial credit window (bytes) |
| `creditThreshold` | number | `initialWindow/2` | When to replenish credits |
| `autoSpawn` | boolean | `true` | Auto-start server |
| `debug` | boolean | `false` | Enable debug logging |
| `timeout` | number | `30000` | Request timeout (ms) |

See [FLOW_CONTROL.md](./docs/FLOW_CONTROL.md) for detailed protocol documentation.

## Example CycleTLS Request for Golang

```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	response, err := client.Do("https://ja3er.com/json", cycletls.Options{
		Body: "",
		Ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		EnableConnectionReuse: true, // Enable connection reuse for better performance
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response)
}

```
#### Example using your own custom http.Client

<details>

```go
import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	http "github.com/Danny-Dasilva/fhttp" // note this is a drop-in replacement for net/http
)

func main() {
	ja3 := "771,52393-52392-52244-52243-49195-49199-49196-49200-49171-49172-156-157-47-53-10,65281-0-23-35-13-5-18-16-30032-11-10,29-23-24,0"
	ua := "Chrome Version 57.0.2987.110 (64-bit) Linux"

	 cycleClient := &http.Client{
		Transport:     cycletls.NewTransport(ja3, ua),
	 }

	resp, err := cycleClient.Get("https://tls.peet.ws/")
	...
}
```
</details>

#### Performance Enhancement: Raw Bytes Option

The default `Init()` method provides the standard v1 API with `chan Response`. For performance-critical applications that can handle raw bytes, use the `WithRawBytes()` option:

```go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	// Use WithRawBytes() option for performance enhancement
	client := cycletls.Init(cycletls.WithRawBytes())
	defer client.Close()
	
	// Queue a request
	go func() {
		client.Queue("https://ja3er.com/json", cycletls.Options{
			Ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
			UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		}, "GET")
	}()
	
	// Performance pattern: receive raw bytes from RespChanV2
	select {
	case responseBytes := <-client.RespChanV2:
		var response cycletls.Response
		json.Unmarshal(responseBytes, &response)
		fmt.Printf("Status: %d\n", response.Status)
		fmt.Printf("Body: %s\n", response.Body)
	// Alternative: still supports v1 pattern via RespChan
	case response := <-client.RespChan:
		fmt.Printf("Status: %d\n", response.Status)
		fmt.Printf("Body: %s\n", response.Body)
	}
}
```

**Note:** Use `Init()` for standard compatibility with `chan Response`. Use `Init(cycletls.WithRawBytes())` when you need the performance benefits of handling raw `[]byte` responses directly.

## Creating an instance

In order to create a `CycleTLS` instance, you can run the following:

#### JavaScript

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

// Basic instantiation - spawns a Golang process that handles all requests concurrently
const client = new CycleTLS();

// With optional configuration
const client = new CycleTLS({ port: 9118, timeout: 30000, debug: false });
```
#### Golang

```go
import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

//The `Init` function initializes golang channels to process requests. 
client := cycletls.Init()
```


## API Reference

### TypeScript/JavaScript APIs

CycleTLS provides two APIs for TypeScript/JavaScript:

#### 1. Streaming API (v3.0.0) - Default Export

The modern streaming API with backpressure support for memory-efficient large downloads.

```typescript
import CycleTLS from 'cycletls';

// Create client
const client = new CycleTLS(options?: CycleTLSOptions);

// HTTP Methods
client.request(options: RequestOptions): Promise<Response>
client.get(url: string, options?: Omit<RequestOptions, "url" | "method">): Promise<Response>
client.post(url: string, body: string, options?: Omit<RequestOptions, "url" | "method" | "body">): Promise<Response>
client.put(url: string, body?: string, options?: Omit<RequestOptions, "url" | "method" | "body">): Promise<Response>
client.delete(url: string, options?: Omit<RequestOptions, "url" | "method">): Promise<Response>
client.patch(url: string, body?: string, options?: Omit<RequestOptions, "url" | "method" | "body">): Promise<Response>
client.head(url: string, options?: Omit<RequestOptions, "url" | "method">): Promise<Response>
client.options(url: string, options?: Omit<RequestOptions, "url" | "method">): Promise<Response>
client.trace(url: string, options?: Omit<RequestOptions, "url" | "method">): Promise<Response>
client.connect(url: string, options?: Omit<RequestOptions, "url" | "method">): Promise<Response>

// WebSocket (returns event-emitter matching 'ws' library API)
client.ws(url: string, options?: Omit<RequestOptions, "url">): Promise<CycleTLSWebSocketV2>
client.webSocket(url: string, options?: Omit<RequestOptions, "url">): Promise<CycleTLSWebSocketV2>

// Server-Sent Events (returns async iterator for events)
client.sse(url: string, options?: Omit<RequestOptions, "url">): Promise<SSEResponse>
client.eventSource(url: string, options?: Omit<RequestOptions, "url">): Promise<SSEResponse>

// Cleanup
client.close(): Promise<void>
```

**WebSocket Events (`CycleTLSWebSocketV2`):**

The WebSocket object returned by `client.ws()` uses the EventEmitter pattern, compatible with the popular `ws` library:

```typescript
const ws = await client.ws('wss://example.com/socket', options);

// Events
ws.on('open', () => { /* connection established */ });
ws.on('message', (data: Buffer, isBinary: boolean) => { /* message received */ });
ws.on('close', (code: number, reason: Buffer) => { /* connection closed */ });
ws.on('error', (error: Error) => { /* error occurred */ });
ws.on('ping', (data: Buffer) => { /* ping received */ });
ws.on('pong', (data: Buffer) => { /* pong received */ });

// Methods
ws.send(data: string | Buffer, options?: { binary?: boolean }, cb?: (err?: Error) => void): void;
ws.close(code?: number, reason?: string): void;
ws.ping(data?: Buffer): void;
ws.pong(data?: Buffer): void;
ws.terminate(): void;

// Properties
ws.readyState;    // 0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED
ws.url;           // WebSocket URL
ws.protocol;      // Negotiated subprotocol
ws.extensions;    // Negotiated extensions
ws.bufferedAmount; // Bytes queued but not sent
ws.binaryType;    // 'nodebuffer' | 'arraybuffer'
```

**SSE Response (`SSEResponse`):**

The SSE response returned by `client.sse()` supports both async iteration and callbacks:

```typescript
const sse = await client.sse('https://example.com/events', options);

// Method 1: Async iterator (recommended)
for await (const event of sse.events()) {
  console.log(event.id);     // Event ID (string)
  console.log(event.event);  // Event type (string)
  console.log(event.data);   // Event data (string)
  console.log(event.retry);  // Retry interval (number, optional)
}

// Method 2: Callback-based
sse.onEvent((event: SSEEvent) => { /* handle event */ });
sse.onError((error: Error) => { /* handle error */ });

// Cleanup
await sse.close();
```

**CycleTLSOptions:**
| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `port` | number | `9119` | Server port |
| `debug` | boolean | `false` | Enable debug logging |
| `timeout` | number | `30000` | Request timeout (ms) |
| `executablePath` | string | auto | Path to CycleTLS binary |
| `autoSpawn` | boolean | `true` | Auto-start server |
| `initialWindow` | number | `65536` | Initial credit window (bytes) |
| `creditThreshold` | number | `initialWindow/2` | Credit replenishment threshold |

**RequestOptions:**
| Option | Type | Description |
|--------|------|-------------|
| `url` | string | Request URL (required) |
| `method` | string | HTTP method |
| `headers` | Record<string, string> | Request headers |
| `body` | string | Request body (UTF-8) |
| `bodyBytes` | Uint8Array | Binary request body |
| `ja3` | string | JA3 fingerprint |
| `ja4r` | string | JA4 raw fingerprint |
| `userAgent` | string | User agent string |
| `proxy` | string | Proxy URL |
| `timeout` | number | Connection timeout until headers arrive (ms) |
| `readTimeout` | number | Body stream idle timeout (ms) |
| `disableRedirect` | boolean | Disable redirect following |
| `insecureSkipVerify` | boolean | Skip TLS verification |
| `forceHTTP1` | boolean | Force HTTP/1.1 |
| `forceHTTP3` | boolean | Force HTTP/3 |
| `http2Fingerprint` | string | HTTP/2 fingerprint |
| `quicFingerprint` | string | QUIC fingerprint |
| `disableGrease` | boolean | Disable GREASE for exact JA4 matching |
| `serverName` | string | Override TLS SNI |
| `headerOrder` | string[] | Custom header order |
| `orderAsProvided` | boolean | Preserve header insertion order |
| `cookies` | RequestCookie[] | Cookies to send |
| `tls13AutoRetry` | boolean | Auto retry TLS 1.3 handshake |
| `enableConnectionReuse` | boolean | Reuse connections (default: true) |

**RequestCookie:**
| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Cookie name |
| `value` | string | Cookie value |
| `path` | string | Cookie path (optional) |
| `domain` | string | Cookie domain (optional) |

**Response:**
| Property/Method | Type | Description |
|-----------------|------|-------------|
| `requestId` | string | Unique request identifier |
| `statusCode` | number | HTTP status code |
| `status` | number | Alias for `statusCode` (axios-style) |
| `finalUrl` | string | Final URL after redirects |
| `headers` | Record<string, string[]> | Response headers |
| `body` | Readable | Streaming response body |
| `data` | Readable | Alias for `body` (stream) |
| `json<T>()` | Promise<T> | Parse body as JSON |
| `text()` | Promise<string> | Get body as text |
| `buffer()` | Promise<Buffer> | Get body as Buffer |
| `arrayBuffer()` | Promise<ArrayBuffer> | Get body as ArrayBuffer |
| `blob()` | Promise<Blob> | Get body as Blob |

#### 2. Legacy API - Named Export

The classic API with buffered responses, still available for backward compatibility.

```typescript
import { initCycleTLS } from 'cycletls';

// Initialize client
const cycleTLS = await initCycleTLS(options?: InitOptions);

// Make requests
cycleTLS(url: string, options: RequestOptions, method?: string): Promise<Response>
cycleTLS.get(url: string, options?: RequestOptions): Promise<Response>
cycleTLS.post(url: string, options?: RequestOptions): Promise<Response>
cycleTLS.put(url: string, options?: RequestOptions): Promise<Response>
cycleTLS.delete(url: string, options?: RequestOptions): Promise<Response>
cycleTLS.head(url: string, options?: RequestOptions): Promise<Response>
cycleTLS.patch(url: string, options?: RequestOptions): Promise<Response>
cycleTLS.options(url: string, options?: RequestOptions): Promise<Response>
cycleTLS.trace(url: string, options?: RequestOptions): Promise<Response>
cycleTLS.connect(url: string, options?: RequestOptions): Promise<Response>

// WebSocket
cycleTLS.ws(url: string, options?: RequestOptions): Promise<WebSocket>
cycleTLS.webSocket(url: string, options?: RequestOptions): Promise<WebSocket>

// Server-Sent Events
cycleTLS.sse(url: string, options?: RequestOptions): Promise<SSEResponse>
cycleTLS.eventSource(url: string, options?: RequestOptions): Promise<SSEResponse>

// Cleanup
cycleTLS.exit(): Promise<void>
```

**InitOptions:**
| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `port` | number | `9119` | Server port |
| `debug` | boolean | `false` | Enable debug logging |
| `timeout` | number | `20000` | Connection timeout (ms) |
| `executablePath` | string | auto | Path to CycleTLS binary |
| `autoExit` | boolean | `true` | Auto cleanup on process exit |

**Legacy Response:**
| Property | Type | Description |
|----------|------|-------------|
| `status` | number | HTTP status code |
| `body` | string | Response body (buffered) |
| `headers` | object | Response headers |
| `finalUrl` | string | Final URL after redirects |
| `json()` | function | Parse body as JSON |
| `text()` | function | Get body as text |

---

### Golang API

```go
import "github.com/Danny-Dasilva/CycleTLS/cycletls"

// Initialize client
client := cycletls.Init(opts ...Option)

// With raw bytes for performance
client := cycletls.Init(cycletls.WithRawBytes())

// Methods
client.Do(url string, options Options, method string) (Response, error)
client.Queue(url string, options Options, method string)
client.Close()

// Response channels
client.RespChan    // chan Response (v1 default)
client.RespChanV2  // chan []byte (with WithRawBytes())
```

**Options struct:**
```go
type Options struct {
    URL                   string
    Method                string
    Headers               map[string]string
    Body                  string
    BodyBytes             []byte            // Binary request body

    // TLS Fingerprinting
    Ja3                   string            // JA3 fingerprint
    Ja4r                  string            // JA4 raw fingerprint
    HTTP2Fingerprint      string            // HTTP/2 fingerprint
    QUICFingerprint       string            // QUIC fingerprint
    DisableGrease         bool              // Disable GREASE

    // Browser identification
    UserAgent             string

    // Connection options
    Proxy                 string
    ServerName            string            // Custom SNI
    Cookies               []Cookie
    Timeout               int               // Seconds
    DisableRedirect       bool
    HeaderOrder           []string
    InsecureSkipVerify    bool

    // Protocol options
    ForceHTTP1            bool
    ForceHTTP3            bool
    Protocol              string            // "http1", "http2", "http3", "websocket", "sse"

    // TLS 1.3 options
    TLS13AutoRetry        bool              // Auto retry with TLS 1.3 curves

    // Connection reuse
    EnableConnectionReuse bool              // Default: true
}
```

**Response struct:**
```go
type Response struct {
    RequestID string
    Status    int
    Body      string
    BodyBytes []byte              // Binary response body
    Headers   map[string]string
    Cookies   []*http.Cookie
    FinalUrl  string
}

// Helper method
response.JSONBody() map[string]interface{}
```

**Custom Transport (for use with standard http.Client):**
```go
// Create transport with JA3
transport := cycletls.NewTransport(ja3, userAgent)

// Create transport with JA4
transport := cycletls.NewTransportWithJA4(ja4, userAgent)

// Create transport with HTTP/2 fingerprint
transport := cycletls.NewTransportWithHTTP2Fingerprint(http2fp, userAgent)

// Create transport with proxy
transport := cycletls.NewTransportWithProxy(ja3, userAgent, proxyDialer)

// Use with standard http.Client
client := &http.Client{Transport: transport}
```

---

### Quick Comparison

| Feature | Streaming API (`CycleTLS`) | Legacy API (`initCycleTLS`) |
|---------|---------------------------|----------------------------|
| Import | `import CycleTLS from 'cycletls'` | `import { initCycleTLS } from 'cycletls'` |
| Init | `new CycleTLS()` | `await initCycleTLS()` |
| Response body | Stream (`body`, alias `data`) + helpers | Buffered (`string`) |
| Status property | `statusCode` (alias `status`) | `status` |
| Memory usage | Bounded (backpressure) | Unbounded for large files |
| Cleanup | `client.close()` | `cycleTLS.exit()` |
| WebSocket | ✅ `client.ws()` (EventEmitter API) | `cycleTLS.ws()` |
| SSE | ✅ `client.sse()` (async iterator) | `cycleTLS.sse()` |
| HTTP Methods | All 9 methods | All 9 methods |
| Response helpers | `.json()`, `.text()`, `.buffer()`, `.arrayBuffer()`, `.blob()` | `.json()`, `.text()`, `.arrayBuffer()`, `.blob()` |

### Exports

CycleTLS provides the following exports:

```typescript
// Default export: CycleTLS class (streaming API)
import CycleTLS from 'cycletls';

// Named exports
import {
  CycleTLS,              // Streaming client class
  CycleTLSOptions,       // Client configuration interface
  RequestOptions,        // Per-request options interface
  Response,              // Streaming response interface
  CycleTLSError,         // Error class with statusCode
  CycleTLSWebSocketV2,   // Streaming WebSocket class
  SSEResponse,           // SSE response interface
  CreditManager,         // Credit manager for flow control
  Protocol,              // Binary protocol helpers (namespace)
  StreamingWebSocket,    // Alias for CycleTLSWebSocketV2
  initCycleTLS,          // Legacy API (backward compatibility)
  CycleTLSWebSocket,     // Legacy WebSocket class
} from 'cycletls';
```

**CommonJS:**
```js
const CycleTLS = require('cycletls').default;          // Streaming client
const { initCycleTLS } = require('cycletls');           // Legacy API
```

### Importable Types

The following TypeScript types/interfaces are available for import:

| Type | Description |
|------|-------------|
| `CycleTLSOptions` | Client configuration (`port`, `debug`, `timeout`, etc.) |
| `RequestOptions` | Per-request options (`url`, `method`, `headers`, `ja3`, etc.) |
| `RequestCookie` | Simple cookie for requests (`name`, `value`, `path?`, `domain?`) |
| `Response` | Streaming response with helper methods |
| `SSEResponse` | SSE response with `events()` async iterator |
| `SSEEvent` | Individual SSE event (`id`, `event`, `data`, `retry`) |
| `CycleTLSError` | Error class with `statusCode` property |
| `CycleTLSWebSocketV2` | Streaming WebSocket (EventEmitter-based) |

### Cookie Types

CycleTLS supports two cookie formats:

**`RequestCookie`** (v3.0.0 streaming API - simple):
```typescript
interface RequestCookie {
  name: string;
  value: string;
  path?: string;
  domain?: string;
}
```

**`Cookie`** (Legacy API - full Go struct):
```typescript
interface Cookie {
  name: string;
  value: string;
  path?: string;
  domain?: string;
  expires?: string;
  rawExpires?: string;
  maxAge?: number;
  secure?: boolean;
  httpOnly?: boolean;
  sameSite?: string;
  unparsed?: string;
}
```

### Legacy Response Object

The legacy API (`initCycleTLS`) returns a different response object than the streaming API:

```typescript
// Legacy Response (from initCycleTLS)
interface CycleTLSResponse {
  status: number;              // HTTP status code
  body: string;                // Full response body (buffered string)
  headers: Record<string, string | string[]>;  // Headers (mixed types)
  finalUrl: string;            // Final URL after redirects

  // Helper methods
  json(): any;                 // Parse body as JSON (synchronous)
  text(): string;              // Get body as text (synchronous)
  arrayBuffer(): ArrayBuffer;  // Get body as ArrayBuffer
  blob(): Blob;                // Get body as Blob
}

// Streaming Response (from CycleTLS - v3.0.0)
interface Response {
  requestId: string;           // Unique request identifier
  statusCode: number;          // HTTP status code
  status: number;              // Alias for statusCode
  finalUrl: string;            // Final URL after redirects
  headers: Record<string, string[]>;  // Headers (always string arrays)
  body: Readable;              // Streaming body
  data: Readable;              // Alias for body

  // Helper methods (async - buffers entire response)
  json<T>(): Promise<T>;       // Parse body as JSON
  text(): Promise<string>;     // Get body as text
  buffer(): Promise<Buffer>;   // Get body as Buffer
  arrayBuffer(): Promise<ArrayBuffer>;
  blob(): Promise<Blob>;
}
```

**Key differences:**
- Legacy `response.body` is a `string`; streaming `response.body` is a `Readable` stream
- Legacy `response.headers` values can be `string | string[]`; streaming headers are always `string[]`
- Legacy helpers are synchronous; streaming helpers are async (return Promises)
- Streaming response adds `requestId` and `data` (alias for `body`)

## CycleTLS Request Config

```js
{
  // URL for the request (required if not specified as an argument)
  url: "https://example.com",
  // Method for the request ("head" | "get" | "post" | "put" | "delete" | "trace" | "options" | "connect" | "patch")
  method: "get", // Default method
  // Custom headers to send
  headers: { "Authorization": "Bearer someexampletoken" },
  // Cookies to send
  cookies: [
    { name: "key", value: "val", path: "/docs", domain: "google.com" }
  ],
  // Body to send with request (string)
  body: "",
  // Binary body for non-UTF8 payloads
  bodyBytes: new Uint8Array([0x00, 0x01]),
  // JA3 token to send with request
  ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
  // JA4R token for enhanced fingerprinting (raw format)
  ja4r: 't13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601',
  // User agent for request
  userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
  // Proxy to send request through (supports http, socks4, socks5, socks5h)
  proxy: 'http://username:password@hostname.com:443',
  // Timeout before headers arrive (ms)
  timeout: 2000,
  // Body stream idle timeout (ms)
  readTimeout: 500,
  // Toggle if CycleTLS should follow redirects
  disableRedirect: true,
  // Custom header order to send with request (This value will overwrite default header order)
  headerOrder: ["cache-control", "connection", "host"],
  // Preserve header insertion order exactly as provided
  orderAsProvided: true,
  // Toggle if CycleTLS should skip verify certificate (If InsecureSkipVerify is true, TLS accepts any certificate presented by the server and any host name in that certificate.)
  insecureSkipVerify: false,
  // Forces CycleTLS to do a http1 handshake
  forceHTTP1: false,
  // Forces HTTP/3 protocol
  forceHTTP3: false,
  // Enable connection reuse across requests
  enableConnectionReuse: true,
  // HTTP/2 fingerprint
  http2Fingerprint: '1:65536;4:131072;5:16384|12517377|3:0:0:201,5:0:0:101,7:0:0:1,9:0:7:1,11:0:3:1,13:0:0:241|m,p,a,s',
  // QUIC fingerprint for HTTP/3
  quicFingerprint: '16030106f2010006ee03039a2b98d81139db0e128ea09eff...',
  // Disable GREASE for exact JA4 matching
  disableGrease: false,
  // Override TLS SNI
  serverName: "example.com",
  // Auto retry TLS 1.3 handshake failures
  tls13AutoRetry: true
}

```

## Response Decompression

CycleTLS automatically handles response decompression for compressed content. No additional configuration is needed.

**Supported Compression Formats**
* `gzip` - Automatically decompressed
* `deflate` - Automatically decompressed  
* `brotli` - Automatically decompressed

### JavaScript Decompression Example
```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  // CycleTLS automatically handles compressed responses
  const response = await client.get('https://httpbin.org/gzip', {
    headers: {
      'Accept-Encoding': 'gzip, deflate, br' // Optional - CycleTLS sets this automatically
    }
  });

  // Response is automatically decompressed
  const data = await response.json();
  console.log('Decompressed data:', data);

  await client.close();
})();
```

### Golang Decompression Example
```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	// CycleTLS automatically handles compressed responses
	response, err := client.Do("https://httpbin.org/gzip", cycletls.Options{
		Headers: map[string]string{
			"Accept-Encoding": "gzip, deflate, br", // Optional - set automatically
		},
	}, "GET")
	
	if err != nil {
		log.Fatal(err)
	}
	
	// Response body is automatically decompressed
	log.Println("Decompressed response:", response.Body)
	
	// Parse as JSON if needed
	jsonData := response.JSONBody()
	log.Println("Parsed JSON:", jsonData)
}
```

**Note:** Decompression happens automatically based on the `Content-Encoding` header. You don't need to manually decompress responses.

## Timeout and Error Handling

CycleTLS provides comprehensive timeout handling and error responses for failed requests.

### Timeout Configuration

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

const client = new CycleTLS();

// JavaScript timeout example
const response = await client.get('https://httpbin.org/delay/10', {
  timeout: 5000, // 5 seconds timeout (ms)
  ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
  userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0'
});

await client.close();
```

```go
// Golang timeout example
response, err := client.Do("https://httpbin.org/delay/10", cycletls.Options{
	Timeout:   5, // 5 seconds timeout
	UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
}, "GET")
```

### Timeout Error Response

When a request times out, the streaming client rejects with `CycleTLSError` (status code `408`). The legacy client returns a response with:
- **Status Code**: `408` (Request Timeout)
- **Body**: Contains error message describing the timeout
- **Error**: Go will have `err != nil`

### JavaScript Timeout Error Handling

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  try {
    const response = await client.get('https://httpbin.org/delay/10', {
      timeout: 2000, // Will timeout after 2 seconds (ms)
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0'
    });

    // Parse response
    const data = await response.json();
    console.log('Success:', data);
  } catch (error) {
    if (error && error.statusCode === 408) {
      console.error('Request timed out:', error.message);
    } else {
      console.error('Request failed:', error);
    }
  } finally {
    await client.close();
  }
})();
```

### Golang Timeout Error Handling

```go
package main

import (
	"log"
	"strings"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	response, err := client.Do("https://httpbin.org/delay/10", cycletls.Options{
		Timeout: 2, // Will timeout after 2 seconds
	}, "GET")

	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}

	// Check for timeout response
	if response.Status == 408 {
		log.Printf("Request timed out: %s", response.Body)
		return
	}

	// Check for other error conditions
	if strings.Contains(response.Body, "timeout") {
		log.Printf("Timeout detected in response: %s", response.Body)
		return
	}

	// Success case
	log.Printf("Request succeeded: %d", response.Status)
}
```

### Common Error Status Codes

- **408**: Request timeout
- **502**: Bad gateway (proxy/connection issues)
- **503**: Service unavailable
- **0**: Connection failed (network errors)

## Proxy Support

CycleTLS supports multiple proxy protocols for routing requests through intermediary servers.

### Supported Proxy Types

- **HTTP Proxy**: `http://proxy.example.com:8080`
- **HTTPS Proxy**: `https://proxy.example.com:8080`
- **SOCKS4**: `socks4://proxy.example.com:1080`
- **SOCKS5**: `socks5://proxy.example.com:1080`
- **SOCKS5h**: `socks5h://proxy.example.com:1080` (hostname resolution through proxy)

### JavaScript Proxy Examples

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';


(async () => {
  const client = new CycleTLS();

  // HTTP Proxy with authentication
  const httpResponse = await client.get('https://httpbin.org/ip', {
    proxy: 'http://username:password@proxy.example.com:8080'
  });

  // SOCKS5 Proxy
  const socksResponse = await client.get('https://httpbin.org/ip', {
    proxy: 'socks5://proxy.example.com:1080'
  });

  // SOCKS5h (hostname resolution through proxy)
  const socks5hResponse = await client.get('https://httpbin.org/ip', {
    proxy: 'socks5h://proxy.example.com:1080'
  });

  console.log('HTTP Proxy IP:', await consumeJSON(httpResponse.body));
  console.log('SOCKS5 IP:', await consumeJSON(socksResponse.body));
  console.log('SOCKS5h IP:', await consumeJSON(socks5hResponse.body));

  await client.close();
})();
```

### Golang Proxy Examples

```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	// HTTP Proxy with authentication
	httpResponse, err := client.Do("https://httpbin.org/ip", cycletls.Options{
		Proxy: "http://username:password@proxy.example.com:8080",
	}, "GET")
	
	if err != nil {
		log.Printf("HTTP proxy request failed: %v", err)
	} else {
		log.Printf("HTTP Proxy Response: %s", httpResponse.Body)
	}

	// SOCKS4 Proxy
	socks4Response, err := client.Do("https://httpbin.org/ip", cycletls.Options{
		Proxy: "socks4://proxy.example.com:1080",
	}, "GET")
	
	if err != nil {
		log.Printf("SOCKS4 proxy request failed: %v", err)
	} else {
		log.Printf("SOCKS4 Response: %s", socks4Response.Body)
	}

	// SOCKS5 Proxy
	socks5Response, err := client.Do("https://httpbin.org/ip", cycletls.Options{
		Proxy: "socks5://proxy.example.com:1080",
	}, "GET")
	
	if err != nil {
		log.Printf("SOCKS5 proxy request failed: %v", err)
	} else {
		log.Printf("SOCKS5 Response: %s", socks5Response.Body)
	}

	// SOCKS5h (hostname resolved through proxy)
	socks5hResponse, err := client.Do("https://httpbin.org/ip", cycletls.Options{
		Proxy: "socks5h://proxy.example.com:1080",
	}, "GET")
	
	if err != nil {
		log.Printf("SOCKS5h proxy request failed: %v", err)
	} else {
		log.Printf("SOCKS5h Response: %s", socks5hResponse.Body)
	}
}
```

### Proxy Error Handling

```go
// Check for proxy connection errors
response, err := client.Do("https://example.com", cycletls.Options{
	Proxy: "socks5://proxy.example.com:1080",
	Timeout: 10,
}, "GET")

if err != nil {
	log.Printf("Proxy connection failed: %v", err)
	return
}

// Check for proxy authentication errors
if response.Status == 407 {
	log.Printf("Proxy authentication required")
	return
}

// Check for proxy server errors
if response.Status == 502 {
	log.Printf("Bad gateway - proxy server error")
	return
}
```

**Note**: SOCKS5h resolves hostnames through the proxy server, providing better privacy and allowing access to internal networks through the proxy.

## CycleTLS Response Schema

```typescript
interface Response {
  // Unique request identifier (String)
  requestId: string;
  // Status code returned from server (Number)
  statusCode: number;
  // Alias for statusCode (Number)
  status: number;
  // Final URL after redirects (String)
  finalUrl: string;
  // Headers returned from the server (Object)
  headers: Record<string, string[]>;
  // Body as a readable stream (for large responses)
  body: Readable;
  // Alias for body (Readable)
  data: Readable;

  // Helper methods (buffer entire response)
  json<T>(): Promise<T>;       // Parse as JSON
  text(): Promise<string>;     // Get as text
  buffer(): Promise<Buffer>;   // Get as Buffer
  arrayBuffer(): Promise<ArrayBuffer>;
  blob(): Promise<Blob>;
}
```

### Response Helper Methods

The response includes convenient methods for parsing the body:

```js
const CycleTLS = require('cycletls').default;

(async () => {
  const client = new CycleTLS();

  const response = await client.get('https://api.example.com/data');

  // Parse as JSON (most common)
  const data = await response.json();

  // Parse as JSON with TypeScript type parameter
  // const typed = await response.json<{ id: number; name: string }>();

  // Get as text
  const text = await response.text();

  // Get as Buffer (for binary data)
  const buffer = await response.buffer();

  // Access response metadata
  console.log('Status:', response.statusCode);
  console.log('Final URL:', response.finalUrl);
  console.log('Headers:', response.headers);

  await client.close();
})();
```

### Streaming Large Responses

For large files, use the stream directly to avoid buffering:

```js
const fs = require('fs');

const response = await client.get('https://example.com/large-file.zip');

// Pipe to file without loading into memory
const writeStream = fs.createWriteStream('download.zip');
response.body.pipe(writeStream);

// Or process chunks manually
for await (const chunk of response.body) {
  process(chunk);
}
```



## Multiple Requests Example for Typescript and Javascript

If CycleTLS is being used in a JavaScript environment, CycleTLS will spawn a Golang process to handle requests. This Golang process handles requests `concurrently` in a worker pool. Due to this, CycleTLS returns response objects as soon as they are made available
(in other terms, CycleTLS processes requests as they are received, but responses are returned asynchronously so they will NOT be returned in the order requested)

If you are using CycleTLS in JavaScript, it is necessary to close the instance to prevent zombie processes. The example below shows one way to approach cleanly closing CycleTLS if you need to process multiple requests (note: keep in mind that calling the `close()` function will kill any requests in progress). If your workflow requires requests running the entire time the process runs, modules such as [exit-hook](https://www.npmjs.com/package/exit-hook) could serve as an alternative solution to cleanly closing CycleTLS.

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

// Defining JA3 token and user agent
const ja3 = "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
const userAgent = "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0";


// Defining multiple requests
const requestDict = {
  "https://httpbin.org/user-agent": {
    ja3: ja3,
    userAgent: userAgent,
    method: "GET",
  },
  "http://httpbin.org/post": {
    body: '{"field":"POST-VAL"}',
    method: "POST",
  },
  "http://httpbin.org/cookies": {
    cookies: [
      {
        name: "example1",
        value: "aaaaaaa",
        expires: "Mon, 02-Jan-2022 15:04:05 EST",
      },
    ],
    method: "GET",
  },
};

// Anonymous async function
(async () => {
  // Create CycleTLS client
  const client = new CycleTLS();

  // Create promises for all requests
  const promises = Object.entries(requestDict).map(async ([url, params]) => {
    const response = await client.request({
      url,
      method: params.method ?? "GET",
      body: params.body ?? "",
      ja3: params.ja3 ?? ja3,
      userAgent: params.userAgent ?? userAgent,
      headers: params.headers,
      cookies: params.cookies,
    });

    // Parse response body as JSON
    const data = await consumeJSON(response.body);
    console.log(url, data);
    return { url, data };
  });

  // Wait for all requests to complete
  await Promise.all(promises);

  // Cleanly close CycleTLS
  await client.close();
})();
```

## Multiple Requests Example for Golang

The general expectation for golang packages is to expect the user to implement a worker pool or any other form of goroutine/asynchronous processing. This package includes a built in Queue method that leverages a worker pool/channels for long running asynchronous requests against a set of urls.

```go
package main

import (
	"log"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

// Static variables
var (
	ja3       = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0"
	userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36"
)

// RequestConfig holds the configuration for each request.
type RequestConfig struct {
	URL     string
	Method  string
	Options cycletls.Options
}

func main() {
	client := cycletls.Init(true) // Initialize with worker pool

	// Define the requests
	requests := []RequestConfig{
		{
			URL:    "http://httpbin.org/delay/4",
			Method: "GET",
			Options: cycletls.Options{
				Ja3:       ja3,
				UserAgent: userAgent,
			},
		},
		{
			URL:    "http://httpbin.org/post",
			Method: "POST",
			Options: cycletls.Options{
				Body:      `{"field":"POST-VAL"}`,
				Ja3:       ja3,
				UserAgent: userAgent,
			},
		},
		{
			URL:    "http://httpbin.org/cookies",
			Method: "GET",
			Options: cycletls.Options{
				Ja3:       ja3,
				UserAgent: userAgent,
				Cookies: []cycletls.Cookie{
					{
						Name:  "example1",
						Value: "aaaaaaa",
					},
				},
			},
		},
	}

	// Queue the requests
	for _, req := range requests {
		client.Queue(req.URL, req.Options, req.Method)
	}

	// Asynchronously read responses as soon as they are available
	// They will return as soon as they are processed
	// e.g. Delay 3 will be returned last
	for i := 0; i < len(requests); i++ {
		response := <-client.RespChan
		log.Println("Response:", response)
	}

	// Close the client
	client.Close()
}

```

# Dev Setup

If you would like to compile CycleTLS on your own, use the following commands:

Set module-aware mode
`go env -w  GO111MODULE=off`

Install golang dependencies
`go get github.com/Danny-Dasilva/CycleTLS/cycletls`

install npm packages (this command handles the above)

`npm install`

### To recompile index.ts in the src folder

`npm run build`

### To recompile Golang files in the golang folder

All

`npm run build:go`

Windows

`npm run build:go:windows:amd64`

Linux

`npm run build:go:linux:amd64`

Mac

`npm run build:go:mac:arm64`


You can view the available compile options within the `package.json`

## Questions

### How do I set Cookies
<details>

There are two simple ways to interface with cookies
### Javascript Simple Cookie Configuration

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  // Create CycleTLS client
  const client = new CycleTLS();
  const response = await client.get("https://httpbin.org/cookies", {
    cookies: {
      cookie1: "value1",
      cookie2: "value2",
    },
  });

  // Parse response using built-in helper
  const data = await response.json();
  console.log(data);
  /* Expected
  {
    "cookies": {
      "cookie1": "value1",
      "cookie2": "value2"
    }
  }
  */
  await client.close();
})();
```

In this simple example you can set the cookie `name` and `value` within an object


### Javascript Complex Cookie Configuration

If you wish to have more fine grained control over cookie parameters you have access to the full underlying Go struct

here are the following values you can set

```ts
export interface Cookie {
  name: string;
  value: string;
  path?: string;
  domain?: string;
  expires?: string;
  rawExpires?: string;
  maxAge?: number;
  secure?: boolean;
  httpOnly?: boolean;
  sameSite?: string;
  unparsed?: string;
}
```

you can use them in a request as follows

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  // Create CycleTLS client
  const client = new CycleTLS();
  const complexCookies = [
    {
      name: "cookie1",
      value: "value1",
      domain: "httpbin.org",
    },
    {
      name: "cookie2",
      value: "value2",
      domain: "httpbin.org",
    },
  ];

  const response = await client.get("https://httpbin.org/cookies", {
    cookies: complexCookies,
  });

  // Parse response using built-in helper
  const data = await response.json();
  console.log(data);
  /* Expected
  {
    "cookies": {
      "cookie1": "value1",
      "cookie2": "value2"
    }
  }
  */
  await client.close();
})();
```


### Golang Configure Cookies
```golang
package main

import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	resp, err := client.Do("https://httpbin.org/cookies", cycletls.Options{
		Body:      "",
		Ja3:       "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		UserAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
		Cookies: []cycletls.Cookie{{Name: "cookie1", Value: "value1"},
			{Name: "cookie2", Value: "value2"}},
	}, "GET")
	if err != nil {
	  log.Print("Request Failed: " + err.Error())
	}
	log.Println(resp.Body)
	/* Expected
	{
	  "cookies": {
		"cookie1": "value1", 
		"cookie2": "value2"
	  }
	  }
	*/
	
	// Alternatively if you want access to values within a map
	log.Println(resp.JSONBody())
	/* Expected
	map[cookies:map[cookie1:value1 cookie2:value2]]
	*/
}

```


Feel free to open an [Issue](https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose) with a feature request for specific file type support. 
</details>


### How do I use CookieJar in CycleTLS?

<details>


### CookieJar in JS

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

const tough = require("tough-cookie");

// Helper to consume response body
async function consumeBody(body) {
  const chunks = [];
  for await (const chunk of body) chunks.push(chunk);
  return Buffer.concat(chunks).toString();
}

(async () => {
  // Create CycleTLS client and CookieJar
  const client = new CycleTLS();
  const cookieJar = new tough.CookieJar();

  // Capture a set cookie
  const firstResponse = await client.get(
    "https://httpbin.org/cookies/set?freeform=test",
    {
      disableRedirect: true,
    }
  );
  // Consume first response body (not needed but good practice)
  await consumeBody(firstResponse.body);

  // Now use the processCookies function to add the cookies from the response headers to the cookie jar
  await processCookies(
    firstResponse,
    "https://httpbin.org/cookies/set?freeform=test",
    cookieJar
  );
  // Now send a second to verify we have our cookies
  const secondResponse = await client.get("https://httpbin.org/cookies", {
    headers: {
      cookie: await cookieJar.getCookieString("https://httpbin.org/cookies"),
    },
  });

  // Verify cookies were set
  const bodyText = await consumeBody(secondResponse.body);
  const data = JSON.parse(bodyText);
  console.log(data);
  /* Expected
  {
    "cookies": {
      "freeform": "test"
    }
  }
  */
  await client.close();
})();

async function processCookies(response, url, cookieJar) {
  const setCookie = response.headers["Set-Cookie"] || response.headers["set-cookie"];
  if (setCookie instanceof Array) {
    for (const cookieString of setCookie) {
      await cookieJar.setCookie(cookieString, url);
    }
  } else if (setCookie) {
    await cookieJar.setCookie(setCookie, url);
  }
}
```


### CookieJar in Golang

```go
package main

import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"log"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

func main() {
	client := cycletls.Init()
	jar, err := cookiejar.New(nil)
  if err != nil {
	  log.Fatal(err)
  }
	// First request to set cookie
	firstResponse, err := client.Do("https://httpbin.org/cookies/set?a=1&b=2&c=3", cycletls.Options{
		Body: "",
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		DisableRedirect: true,
	},
		 "GET")
	if err != nil {
		log.Fatal(err)
	}
	firstURL, _ := url.Parse(firstResponse.FinalUrl)
  jar.SetCookies( firstURL, firstResponse.Cookies)


	// Second request to verify cookies, including the cookies from the first response
	secondResponse, err := client.Do("https://httpbin.org/cookies", cycletls.Options{
	Body: "",
	Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
	UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
	Headers: map[string]string{
		"Cookie": getHeadersFromJar(jar, firstURL),
	},
	}, "GET")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Second Response body:", secondResponse.Body)
}


func getHeadersFromJar(jar *cookiejar.Jar, url *url.URL) string {
	cookies := jar.Cookies(url)
	var cookieStrs []string
	for _, cookie := range cookies {
		cookieStrs = append(cookieStrs, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(cookieStrs, "; ")
}

```

</details>

### How do I send multipart/form-data in CycleTLS

<details>

### Javascript Text form-data
```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';
const FormData = require('form-data');

(async () => {
  const client = new CycleTLS();

  const formData = new FormData();
  formData.append("key1", "value1");
  formData.append("key2", "value2");

  const response = await client.post('http://httpbin.org/post', formData.getBuffer().toString(), {
    headers: formData.getHeaders(), // Use formData.getHeaders() for proper content-type
  });

  // Parse response using built-in helper
  const data = await response.json();
  console.log(data);

  await client.close();
})();
```


### Javascript File form-data
```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';
const FormData = require('form-data');
const fs = require('fs');

(async () => {
  const client = new CycleTLS();

  const formData = new FormData();
  const fileContent = fs.readFileSync("../go.mod");
  formData.append('file', fileContent, 'go.mod');

  const response = await client.post('http://httpbin.org/post', formData.getBuffer().toString(), {
    headers: formData.getHeaders(), // Use formData.getHeaders() for proper content-type
  });

  // Parse response using built-in helper
  const data = await response.json();
  console.log(data);

  await client.close();
})();
```

### Golang Text form-data
```golang
package main

import (
	"bytes"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"log"
	"mime/multipart"
)

func main() {
	client := cycletls.Init()

	// Prepare a buffer to write our multipart form
	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Add form fields
	multipartWriter.WriteField("key1", "value1")
	multipartWriter.WriteField("key2", "value2")

	contentType := multipartWriter.FormDataContentType()
	// Close the writer before making the request
	multipartWriter.Close()

	response, err := client.Do("http://httpbin.org/post", cycletls.Options{
		Body: requestBody.String(),
		Headers: map[string]string{
			"Content-Type": contentType,
		},
	}, "POST")

	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}

	log.Println(response.Body)
}
```


### Golang file upload form-data
```golang
package main

import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"os"
)

func main() {
  client := cycletls.Init()

  // Prepare a buffer to write our multipart form
  var requestBody bytes.Buffer
  multipartWriter := multipart.NewWriter(&requestBody)

  // Add a file
  fileWriter, err := multipartWriter.CreateFormFile("fieldname", "filename")
  if err != nil {
	  log.Fatal("CreateFormFile Error: ", err)
  }

  // Open the file that you want to upload
  file, err := os.Open("path/to/your/file")
  if err != nil {
	  log.Fatal("File Open Error: ", err)
  }
  defer file.Close()

  // Copy the file to the multipart writer
  _, err = io.Copy(fileWriter, file)
  if err != nil {
	  log.Fatal("File Copy Error: ", err)
  }

  // Close the writer before making the request
  contentType := multipartWriter.FormDataContentType()
  multipartWriter.Close()

  response, err := client.Do("http://httpbin.org/post", cycletls.Options{
	  Body: requestBody.String(),
	  Headers: map[string]string{
		  "Content-Type": contentType,
	  },
  }, "POST")

  if err != nil {
	  log.Print("Request Failed: " + err.Error())
  }

  log.Println(response.Body)
}
```


If requested encoding helpers can be added to the repo for golang 
</details>

### How do I send a application/x-www-form-urlencoded Post request

<details>

### Javascript application/x-www-form-urlencoded form
```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  const urlEncodedData = new URLSearchParams();
  urlEncodedData.append('key1', 'value1');
  urlEncodedData.append('key2', 'value2');

  const response = await client.post('http://httpbin.org/post', urlEncodedData.toString(), {
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded',
    },
  });

  // Parse response using built-in helper
  const data = await response.json();
  console.log(data);

  await client.close();
})();
```
### Golang application/x-www-form-urlencoded form
```golang
package main

import (
	"log"
	  "net/url"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {

	client := cycletls.Init()

	// Prepare form data
	form := url.Values{}
	form.Add("key1", "value1")
	form.Add("key2", "value2")

	response, err := client.Do("http://httpbin.org/post", cycletls.Options{
		Body: form.Encode(),
		Headers: map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		},
	}, "POST")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response.Body)
}

```
</details>

### How do I download images and videos?

<details>

Images and videos with supported `Content-Type` headers are returned as raw binary data stored in a string format.

**Supported Media Types**
* `image/svg+xml`
* `image/webp`
* `image/jpeg`
* `image/png`
* `image/gif`
* `application/pdf`
* `video/mp4`
* `video/webm`
* `video/avi`
* `video/quicktime`

**Important:** The media data is NOT base64 encoded. It is raw binary data converted to a string format.

To write them to a file you can use the below methods

### Javascript Media Download Example
```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';
const fs = require("fs");

// Helper to consume response body as Buffer
async function consumeBuffer(body) {
  const chunks = [];
  for await (const chunk of body) chunks.push(chunk);
  return Buffer.concat(chunks);
}

(async () => {
  const client = new CycleTLS();

  // Download JPEG image
  const jpegResponse = await client.get("http://httpbin.org/image/jpeg", {
    ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
    userAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
  });
  const jpegBuffer = await consumeBuffer(jpegResponse.body);
  fs.writeFileSync('./images/output.jpeg', jpegBuffer);
  console.log('JPEG image downloaded');

  // Download PNG
  const pngResponse = await client.get("http://httpbin.org/image/png", {
    ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
    userAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
  });
  const pngBuffer = await consumeBuffer(pngResponse.body);
  fs.writeFileSync('./images/output.png', pngBuffer);
  console.log('PNG image downloaded');

  // Download WebP
  const webpResponse = await client.get("http://httpbin.org/image/webp", {
    ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
    userAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
  });
  const webpBuffer = await consumeBuffer(webpResponse.body);
  fs.writeFileSync('./images/output.webp', webpBuffer);
  console.log('WebP image downloaded');

  // Download video
  const videoResponse = await client.get("https://sample-videos.com/zip/10/mp4/SampleVideo_360x240_1mb.mp4", {
    ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
    userAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
  });
  const videoBuffer = await consumeBuffer(videoResponse.body);
  fs.writeFileSync('./videos/sample_video.mp4', videoBuffer);
  console.log('Video downloaded');

  await client.close();
})();
```

### Streaming Binary Data Example

For large files, the streaming API provides memory-efficient downloads:

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';
const fs = require("fs");

(async () => {
  const client = new CycleTLS();

  try {
    // Stream large video file
    const response = await client.get("https://sample-videos.com/zip/25/mp4/SampleVideo_1280x720_5mb.mp4", {
      ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
      userAgent: "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
    });

    const writeStream = fs.createWriteStream('./videos/large_video.mp4');
    let totalSize = 0;

    // Stream body chunks directly to file
    for await (const chunk of response.body) {
      totalSize += chunk.length;
      console.log(`Downloaded ${totalSize} bytes`);
      writeStream.write(chunk);
    }

    writeStream.end();
    console.log(`Stream complete. Total size: ${totalSize} bytes`);
  } catch (error) {
    console.error('Download error:', error);
  } finally {
    await client.close();
  }
})();
```

### Golang Media Download Example
```golang
package main

import (
	"log"
	"os"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func writeMedia(filepath string, data string) error {
	// Convert string body to bytes (raw binary data)
	bodyBytes := []byte(data)
	
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()
	
	if _, err := f.Write(bodyBytes); err != nil {
		return err
	}
	
	return f.Sync()
}

func main() {
	client := cycletls.Init()
	defer client.Close()
	
	// Download image
	response, err := client.Do("http://httpbin.org/image/jpeg", cycletls.Options{
	  Body:      "",
	  Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
	  UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
	}, "GET")
	
	if err != nil {
		log.Fatal("Image download failed: ", err)
	}
	
	if err := writeMedia("test.jpeg", response.Body); err != nil {
		log.Fatal("Image write failed: ", err)
	}
	log.Println("Image downloaded successfully")
	
	// Download video
	videoResponse, err := client.Do("https://sample-videos.com/zip/10/mp4/SampleVideo_360x240_1mb.mp4", cycletls.Options{
	  Body:      "",
	  Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
	  UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
	}, "GET")
	
	if err != nil {
		log.Fatal("Video download failed: ", err)
	}
	
	if err := writeMedia("sample_video.mp4", videoResponse.Body); err != nil {
		log.Fatal("Video write failed: ", err)
	}
	log.Println("Video downloaded successfully")
}

```


Additional file type support is planned.

Feel free to open an [Issue](https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose) with a feature request for specific file type support. 
</details>

### How do I use Connection Reuse?

<details>

Connection reuse allows you to reuse TLS connections across multiple requests to the same host, reducing handshake overhead and improving performance.

### Golang Connection Reuse

```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	// Initialize without worker pool for better connection management
	client := cycletls.Init(false)
	defer client.Close()

	// Enable connection reuse in the options
	options := cycletls.Options{
		Ja3:                   "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		UserAgent:             "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
		EnableConnectionReuse: true, // Enable connection reuse
	}

	// First request - establishes connection
	resp1, err := client.Do("https://httpbin.org/get", options, "GET")
	if err != nil {
		log.Fatal("First request failed: ", err)
	}
	log.Println("First request status:", resp1.Status)

	// Second request - reuses connection
	resp2, err := client.Do("https://httpbin.org/headers", options, "GET")
	if err != nil {
		log.Fatal("Second request failed: ", err)
	}
	log.Println("Second request status:", resp2.Status)

	// Connection is reused for requests to the same host
}
```

</details>

### How do I use HTTP/3 and QUIC?

<details>

CycleTLS now supports HTTP/3 over QUIC protocol with custom QUIC fingerprinting.

### Golang HTTP/3 Basic Usage

```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	// Force HTTP/3
	response, err := client.Do("https://cloudflare-quic.com/", cycletls.Options{
		ForceHTTP3:         true,
		UserAgent:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		InsecureSkipVerify: true,
	}, "GET")

	if err != nil {
		log.Fatal("Request failed: ", err)
	}

	log.Println("Response over HTTP/3:", response.Status)
}
```

### Golang QUIC Fingerprinting

```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	// Custom QUIC fingerprint
	quicFingerprint := "16030106f2010006ee03039a2b98d81139db0e128ea09eff6874549c219b543fb6dbaa7e4dbfe9e31602c620ce04c4026f019442affade7fed8ba66e022e186f77f1c670fd992f33c0143f120020aaaa130113021303c02bc02fc02cc030cca9cca8c013c014009c009d002f0035010006851a1a00000010000e000c02683208687474702f312e31002b000706dada03040303..."

	response, err := client.Do("https://cloudflare-quic.com/", cycletls.Options{
		QUICFingerprint:    quicFingerprint,
		ForceHTTP3:         true,
		UserAgent:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		InsecureSkipVerify: true,
	}, "GET")

	if err != nil {
		log.Fatal("Request failed: ", err)
	}

	log.Println("Response with QUIC fingerprint:", response.Status)
}
```

### Golang HTTP/3 Transport Direct Usage

```go
package main

import (
	"crypto/tls"
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	http "github.com/Danny-Dasilva/fhttp"
)

func main() {
	// Create TLS config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	// Create HTTP/3 transport
	transport := cycletls.NewHTTP3Transport(tlsConfig)

	// Create request
	req, err := http.NewRequest("GET", "https://cloudflare-quic.com/", nil)
	if err != nil {
		log.Fatal("Failed to create request: ", err)
	}

	// Send request
	resp, err := transport.RoundTrip(req)
	if err != nil {
		log.Fatal("Request failed: ", err)
	}
	defer resp.Body.Close()

	log.Println("Direct HTTP/3 response:", resp.Status)
}
```

</details>

### How do I use WebSocket support?

<details>

CycleTLS provides a full-featured WebSocket client with TLS fingerprinting support. The API matches the popular `ws` library.

### JavaScript WebSocket Example (Streaming API)

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  // Connect to WebSocket with TLS fingerprinting
  const ws = await client.ws('wss://echo.websocket.org', {
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    headers: {
      'Sec-WebSocket-Protocol': 'echo-protocol'
    }
  });

  // Handle connection open
  ws.on('open', () => {
    console.log('WebSocket connected!');
    console.log('Protocol:', ws.protocol);
    console.log('Extensions:', ws.extensions);
    ws.send('Hello, WebSocket!');
  });

  // Handle incoming messages
  ws.on('message', (data, isBinary) => {
    console.log('Received:', data.toString());
  });

  // Handle connection close
  ws.on('close', (code, reason) => {
    console.log('Closed:', code, reason);
    client.close();
  });

  // Handle errors
  ws.on('error', (err) => {
    console.error('WebSocket error:', err);
  });

  // Send messages (text or binary)
  ws.send('text message');
  ws.send(Buffer.from([0x01, 0x02, 0x03]));

  // Close connection gracefully
  ws.close(1000, 'Normal closure');
})();
```

### WebSocket Properties and Methods

```typescript
interface CycleTLSWebSocketV2 {
  // Properties
  URL: string;                              // WebSocket URL
  readyState: number;                       // 0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED
  protocol: string;                         // Negotiated subprotocol
  extensions: string;                       // Negotiated extensions
  binaryType: 'nodebuffer' | 'arraybuffer'; // Binary data format

  // Methods
  send(data: string | Buffer, callback?: (err?: Error) => void): void;
  close(code?: number, reason?: string): void;
  ping(data?: Buffer | string): void;
  pong(data?: Buffer | string): void;
  terminate(): void;  // Immediate close without handshake

  // Events
  on('open', () => void): void;
  on('message', (data: Buffer, isBinary: boolean) => void): void;
  on('close', (code: number, reason: string) => void): void;
  on('error', (err: Error) => void): void;
  on('ping', (data: Buffer) => void): void;
  on('pong', (data: Buffer) => void): void;
}
```

### Golang WebSocket Example

```go
package main

import (
	"log"
	"net/http"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
)

func main() {
	// Create TLS config
	tlsConfig := &utls.Config{
		InsecureSkipVerify: true,
	}

	// Create headers
	headers := make(http.Header)
	headers.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")

	// Create WebSocket client
	wsClient := cycletls.NewWebSocketClient(tlsConfig, headers)

	// Connect to WebSocket server
	conn, resp, err := wsClient.Connect("wss://echo.websocket.org/")
	if err != nil {
		log.Fatal("WebSocket connection failed: ", err)
	}
	defer conn.Close()

	log.Println("WebSocket connected, status:", resp.StatusCode)

	// Send message
	testMessage := "Hello, WebSocket!"
	if err := conn.WriteMessage(websocket.TextMessage, []byte(testMessage)); err != nil {
		log.Fatal("Failed to send message: ", err)
	}

	// Read response
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		log.Fatal("Failed to read message: ", err)
	}

	log.Printf("Received message type %d: %s\n", messageType, string(message))
}
```

### Golang WebSocket Response Wrapper

```go
package main

import (
	"log"
	"net/http"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
)

func main() {
	// Setup WebSocket client
	tlsConfig := &utls.Config{
		InsecureSkipVerify: true,
	}

	headers := make(http.Header)
	headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	wsClient := cycletls.NewWebSocketClient(tlsConfig, headers)

	// Connect
	conn, _, err := wsClient.Connect("wss://echo.websocket.org/")
	if err != nil {
		log.Fatal("Connection failed: ", err)
	}

	// Create response wrapper
	wsResponse := &cycletls.WebSocketResponse{
		Conn: conn,
	}
	defer wsResponse.Close()

	// Send message using wrapper
	if err := wsResponse.Send(websocket.TextMessage, []byte("Hello!")); err != nil {
		log.Fatal("Send failed: ", err)
	}

	// Receive message using wrapper
	messageType, message, err := wsResponse.Receive()
	if err != nil {
		log.Fatal("Receive failed: ", err)
	}

	log.Printf("Received: %s (type: %d)
", string(message), messageType)
}
```


</details>

### How do I use Server-Sent Events (SSE)?

<details>

CycleTLS provides first-class SSE support with async iteration for real-time event processing.

### JavaScript SSE Example (Streaming API)

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  // Connect to SSE endpoint with TLS fingerprinting
  const sse = await client.sse('https://example.com/events', {
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0'
  });

  // Option 1: Async iteration (recommended)
  for await (const event of sse.events()) {
    console.log('Event ID:', event.id);
    console.log('Event Type:', event.event);
    console.log('Event Data:', event.data);

    if (event.event === 'done') {
      break;
    }
  }

  // Option 2: Callback-based
  sse.onEvent((event) => {
    console.log('Received:', event.event, event.data);
  });

  sse.onError((err) => {
    console.error('SSE error:', err);
  });

  // Close the connection
  await sse.close();
  await client.close();
})();
```

### SSE Response Interface

```typescript
interface SSEEvent {
  id?: string;      // Event ID (for resuming)
  event?: string;   // Event type (default: 'message')
  data: string;     // Event payload
  retry?: number;   // Reconnection time (ms)
}

interface SSEResponse extends Response {
  events(): AsyncIterableIterator<SSEEvent>;
  onEvent(callback: (event: SSEEvent) => void): void;
  onError(callback: (error: Error) => void): void;
  close(): Promise<void>;
}
```

### JavaScript SSE with Raw Streaming

You can also use the streaming API to process SSE events manually:

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  try {
    // SSE with streaming for real-time processing
    const response = await client.get('https://example.com/events', {
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      headers: {
        'Accept': 'text/event-stream',
        'Cache-Control': 'no-cache'
      }
    });

    // Process SSE stream in real-time
    let buffer = '';

    for await (const chunk of response.body) {
      buffer += chunk.toString();
      const lines = buffer.split('\n');

      // Process complete lines, keep incomplete line in buffer
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('data:')) {
          const eventData = line.substring(5).trim();
          console.log('Event data:', eventData);
        } else if (line.startsWith('event:')) {
          const eventType = line.substring(6).trim();
          console.log('Event type:', eventType);
        }
      }
    }

    console.log('SSE stream ended');
  } catch (error) {
    console.error('SSE stream error:', error);
  } finally {
    await client.close();
  }
})();
```

### Golang SSE Client Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	fhttp "github.com/Danny-Dasilva/fhttp"
)

func main() {
	// Create HTTP client
	httpClient := &fhttp.Client{
		Timeout: 30 * time.Second,
	}

	// Create headers
	headers := make(fhttp.Header)
	headers.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	headers.Set("Accept", "text/event-stream")

	// Create SSE client
	sseClient := cycletls.NewSSEClient(httpClient, headers)

	// Connect to SSE server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sseResp, err := sseClient.Connect(ctx, "http://localhost:3333/events")
	if err != nil {
		log.Fatal("SSE connection failed: ", err)
	}
	defer sseResp.Close()

	// Read events
	eventCount := 0
	for eventCount < 5 {
		event, err := sseResp.NextEvent()
		if err != nil {
			log.Printf("Error reading event: %v\n", err)
			break
		}
		
		if event != nil {
			eventCount++
			fmt.Printf("Event #%d:\n", eventCount)
			fmt.Printf("  Type: %s\n", event.Event)
			fmt.Printf("  Data: %s\n", event.Data)
			fmt.Printf("  ID: %s\n", event.ID)
		}
	}
}
```

### Golang SSE with Browser Configuration

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	// Create browser configuration with TLS fingerprinting
	browser := cycletls.Browser{
		UserAgent:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		JA3:                "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		InsecureSkipVerify: true,
	}

	// Connect to SSE endpoint with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := browser.SSEConnect(ctx, "http://127.0.0.1:3333/events")
	if err != nil {
		log.Fatal("SSE connection failed: ", err)
	}
	defer response.Close()

	// Process events with detailed parsing
	for {
		event, err := response.NextEvent()
		if err != nil {
			log.Printf("Event stream ended: %v\n", err)
			break
		}
		
		if event != nil && event.Data != "" {
			fmt.Printf("Event Type: %s\n", event.Event)
			fmt.Printf("Event ID: %s\n", event.ID)
			fmt.Printf("Event Data: %s\n", event.Data)
			
			// Break after receiving specific event
			if event.Data == "done" {
				break
			}
		}
	}
}
```

### Browser.SSEConnect Method

The `Browser.SSEConnect` method provides SSE connections with TLS fingerprinting support:

```go
type Browser struct {
	UserAgent          string
	JA3                string
	JA4r               string
	JA4r               string
	HTTP2Fingerprint   string
	QUICFingerprint    string
	InsecureSkipVerify bool
	ForceHTTP1         bool
	ForceHTTP3         bool
}

// SSEConnect establishes an SSE connection with browser fingerprinting
func (b *Browser) SSEConnect(ctx context.Context, url string) (*SSEResponse, error)
```

### SSE Event Structure

```go
type SSEEvent struct {
	ID    string  // Event ID from server
	Event string  // Event type (custom event names)
	Data  string  // Event data payload
	Retry int64   // Reconnection time in milliseconds
}
```

</details>

### How do I use JA4R fingerprinting?
### How do I use JA4R fingerprinting?

<details>

> **Note:** Pass `ja4r` (raw format) to configure fingerprints. JA4 hashes are for observation only.
> **Note:** Pass `ja4r` (raw format) to configure fingerprints. JA4 hashes are for observation only.

JA4R is the raw format for configuring TLS fingerprints with explicit cipher suites and extensions.

### Golang JA4R Fingerprinting
JA4R is the raw format for configuring TLS fingerprints with explicit cipher suites and extensions.

### Golang JA4R Fingerprinting

```go
package main

import (
	"log"
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
)

func main() {
	client := cycletls.Init()
	defer client.Close()

	// Use both JA3 and JA4R fingerprints
	// Use both JA3 and JA4R fingerprints
	response, err := client.Do("https://tls.peet.ws/api/clean", cycletls.Options{
		Ja3:       "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
		Ja4r:      "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601", // JA4R fingerprint (raw format)
		Ja4r:      "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601", // JA4R fingerprint (raw format)
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36",
	}, "GET")

	if err != nil {
		log.Fatal("Request failed: ", err)
	}

	log.Println("Response with JA4R:", response.Status)
	log.Println("Response with JA4R:", response.Status)
}
```

</details>

### How do I set a custom SNI (domain fronting)?

<details>

You can override the TLS Server Name Indication (SNI) independently from the HTTP `Host` header. This enables domain fronting scenarios where the handshake SNI differs from the request host.

JavaScript/TypeScript:

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();
  const response = await client.get('https://127.0.0.1:8443', {
    serverName: 'front.example',            // TLS SNI used in handshake
    headers: { Host: 'real.example' },      // HTTP Host header inside the request
    insecureSkipVerify: true,               // for local/self-signed testing
    ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0',
    userAgent: 'Mozilla/5.0 ... Chrome/101.0.4951.54 Safari/537.36'
  });

  // Parse response
  const text = await response.text();
  console.log(text);

  await client.close();
})();
```

Golang:

```go
client := cycletls.Init()
response, err := client.Do("https://127.0.0.1:8443", cycletls.Options{
    ServerName: "front.example",                  // TLS SNI
    Headers:    map[string]string{"Host": "real.example"}, // HTTP Host
    InsecureSkipVerify: true,
    Ja3: "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
    UserAgent: "Mozilla/5.0 ... Chrome/101.0.4951.54 Safari/537.36",
}, "GET")
```

Notes:
- When `serverName` is provided, it is used for the TLS handshake; the library will not overwrite your `Host` header.
- JA4R fingerprints that include SNI (extension 0x0000) will be constructed using the provided `serverName`.
 - Protocol support: `serverName` works with HTTP/1.1, HTTP/2, HTTP/3, WebSocket (`wss://`), and SSE (`https://`).

WebSocket (wss) with custom SNI:

```ts
import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();
  const ws = await client.ws('wss://127.0.0.1:8443/socket', {
    serverName: 'front.example',
    headers: { Host: 'real.example' },
    insecureSkipVerify: true,
  });
  ws.on('message', (data) => console.log('message:', data.toString()));
  ws.on('close', () => client.close());
  ws.close();
})();
```

SSE with custom SNI:

```ts
import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();
  const sse = await client.sse('https://127.0.0.1:8443/events', {
    serverName: 'front.example',
    headers: { Host: 'real.example' },
    insecureSkipVerify: true,
  });
  for await (const event of sse.events()) {
    console.log('event:', event.data);
  }
  await sse.close();
  await client.close();
})();
```

</details>

### How do I set/force HTTP1 or HTTP3

<details>

In golang set `ForceHTTP1` in Options

```go
package main

import (
	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	"log"
)

func main() {
	client := cycletls.Init()
	response, err := client.Do("https://tls.peet.ws/api/all", cycletls.Options{
		ForceHTTP1: true,
	}, "GET")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response.Body,) //You can verify the HTTP_Version in the response

}

```


In JS/TS set `forceHTTP1` in Options

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  const response = await client.get('https://ja3er.com/json', {
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    forceHTTP1: true, // Set this field to force HTTP/1.1
  });

  // Parse response using built-in helper
  const data = await response.json();
  console.log(data);
  // You can verify the HTTP_Version in the response

  await client.close();
})();
```



### Forcing HTTP/3

Similarly, you can force HTTP/3 protocol usage:

In JS/TS set `forceHTTP3` in Options

```js
const CycleTLS = require('cycletls').default;
// TypeScript: import CycleTLS from 'cycletls';

(async () => {
  const client = new CycleTLS();

  const response = await client.get('https://www.google.com/', {
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    forceHTTP3: true, // Set this field to force HTTP/3 via QUIC
  });

  // Parse response
  const text = await response.text();
  console.log(text);
  // HTTP/3 requests use QUIC protocol

  await client.close();
})();
```

</details>

### Cross Compiling for other platforms
<details>

Natively the 3 Operating System types `linux`, `darwin` , `windows`  should cover most use cases.
	
You can use the built in Golang cross compiling commands `go build` to compile for another operating system. 

As an example for linux arm you need to pass in the `GOOS` and `GOARCH` arguments

```bash
$ GOOS=linux GOARCH=arm go build -o ./dist/index ./golang && chmod +x ./dist/index
```

With the above command you can simply run `./index` and CycleTLS should function as intended.

Use this [gist](https://gist.github.com/asukakenji/f15ba7e588ac42795f421b48b8aede63) for different Operating Systems that support cross-compilation and feel free to open an [Issue](https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose) with a feature request for your specific operating system use case. 

</details>

## LICENSE
### GPL3 LICENSE SYNOPSIS

**_TL;DR_*** Here's what the GPL3 license entails:

```markdown
1. Anyone can copy, modify and distribute this software.
2. You have to include the license and copyright notice with each and every distribution.
3. You can use this software privately.
4. You can use this software for commercial purposes.
5. Source code MUST be made available when the software is distributed.
6. Any modifications of this code base MUST be distributed with the same license, GPLv3.
7. This software is provided without warranty.
8. The software author or license can not be held liable for any damages inflicted by the software.
```

More information on about the [LICENSE can be found here](http://choosealicense.com/licenses/gpl-3.0/)
