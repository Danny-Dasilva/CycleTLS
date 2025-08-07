# CycleTLS Response API (Axios-Style)

CycleTLS now uses an Axios-compatible response API, making it familiar and easy to use. The response data is automatically parsed based on the `responseType` option and made available via `response.data`.

## Basic Usage

### JSON Responses (Default)
```typescript
const response = await cycleTLS.get('https://api.example.com/data');
console.log(response.data); // Parsed JSON object
console.log(response.status); // 200
console.log(response.headers); // Response headers
```

### Text Responses
```typescript
const response = await cycleTLS.get('https://example.com', { 
  responseType: 'text' 
});
console.log(response.data); // String content
```

### Binary Data as ArrayBuffer
```typescript
const response = await cycleTLS.get('https://example.com/file.pdf', { 
  responseType: 'arraybuffer' 
});
console.log(response.data); // ArrayBuffer
const uint8Array = new Uint8Array(response.data);
```

### Binary Data as Blob
```typescript
const response = await cycleTLS.get('https://example.com/image.png', { 
  responseType: 'blob' 
});
console.log(response.data); // Blob
console.log(response.data.type); // "image/png"
```

## Response Types

### Available responseType Options
- `'json'` (default) - Auto-parse JSON, fallback to raw Buffer for compressed data
- `'text'` - Return as UTF-8 string
- `'arraybuffer'` - Return as ArrayBuffer
- `'blob'` - Return as Blob with content-type
- `'stream'` - Return as Node.js Readable stream

### Examples for Each Type

```typescript
// JSON (default)
const jsonResponse = await cycleTLS.get('/api/users');
console.log(jsonResponse.data.users); // Parsed JSON

// Text
const textResponse = await cycleTLS.get('/api/info', { responseType: 'text' });
console.log(typeof textResponse.data); // "string"

// ArrayBuffer for binary processing
const binaryResponse = await cycleTLS.get('/api/download', { 
  responseType: 'arraybuffer' 
});
const bytes = new Uint8Array(binaryResponse.data);

// Blob for file downloads
const fileResponse = await cycleTLS.get('/api/file.zip', { 
  responseType: 'blob' 
});
// Save or process the blob

// Stream for large files
const streamResponse = await cycleTLS.get('/api/large-file', { 
  responseType: 'stream' 
});
streamResponse.data.pipe(fs.createWriteStream('output.dat'));
```

## Handling Compressed Responses

When servers send compressed data (gzip, deflate, brotli), CycleTLS returns the raw compressed bytes. This keeps the library lightweight and lets you choose your decompression library.

### Detecting Compressed Responses
```typescript
const response = await cycleTLS.get('https://httpbin.org/gzip', {
  headers: { 'Accept-Encoding': 'gzip, deflate, br' }
});

const isCompressed = response.headers['content-encoding'];
if (isCompressed) {
  console.log(`Response is ${isCompressed} compressed`);
  console.log(response.data); // Raw compressed Buffer
}
```

### Decompressing with Popular Libraries

#### Using Pako (gzip/deflate)
```typescript
import * as pako from 'pako';

const response = await cycleTLS.get('https://httpbin.org/gzip');
const encoding = response.headers['content-encoding'];

if (encoding === 'gzip') {
  const decompressed = pako.ungzip(response.data, { to: 'string' });
  const data = JSON.parse(decompressed);
  console.log(data);
}
```

#### Using Brotli
```typescript
import * as brotli from 'brotli';

const response = await cycleTLS.get('https://httpbin.org/brotli');
if (response.headers['content-encoding'] === 'br') {
  const decompressed = brotli.decompress(response.data);
  const text = Buffer.from(decompressed).toString('utf8');
  const data = JSON.parse(text);
  console.log(data);
}
```

#### Generic Helper Function
```typescript
import * as pako from 'pako';
import * as brotli from 'brotli';

function decompressResponse(response) {
  const encoding = response.headers['content-encoding'];
  const compressed = response.data;
  
  switch(encoding) {
    case 'gzip':
      return pako.ungzip(compressed, { to: 'string' });
    case 'deflate':
      return pako.inflate(compressed, { to: 'string' });
    case 'br':
    case 'brotli':
      const buffer = brotli.decompress(compressed);
      return Buffer.from(buffer).toString('utf8');
    default:
      return compressed.toString('utf8');
  }
}

// Usage
const response = await cycleTLS.get('https://httpbin.org/gzip');
const decompressed = decompressResponse(response);
const data = JSON.parse(decompressed);
```

## Error Handling

```typescript
try {
  const response = await cycleTLS.get('https://api.example.com/data');
  
  if (response.status >= 400) {
    console.error('HTTP Error:', response.status);
    console.error('Error data:', response.data);
  } else {
    console.log('Success:', response.data);
  }
} catch (error) {
  console.error('Request failed:', error.message);
}
```

## Response Object Properties

Every response object contains:

```typescript
interface CycleTLSResponse {
  status: number;        // HTTP status code (200, 404, etc.)
  headers: object;       // Response headers
  data: any;            // Parsed response data (based on responseType)
  finalUrl: string;     // Final URL after redirects
}
```

## Migration Guide

### From Previous CycleTLS Versions
**Before:**
```typescript
const response = await cycleTLS.get(url);
const data = await response.json();
const text = await response.text();
const buffer = await response.raw();
```

**After:**
```typescript
// JSON (default)
const response = await cycleTLS.get(url);
console.log(response.data); // Parsed JSON or raw Buffer if compressed

// Text
const textResponse = await cycleTLS.get(url, { responseType: 'text' });
console.log(textResponse.data); // String

// Raw bytes
const binaryResponse = await cycleTLS.get(url, { responseType: 'arraybuffer' });
console.log(binaryResponse.data); // ArrayBuffer
```

### From Axios
The API is now nearly identical to Axios:

```typescript
// Axios
const response = await axios.get(url, { responseType: 'arraybuffer' });
console.log(response.data);

// CycleTLS (same API!)
const response = await cycleTLS.get(url, { responseType: 'arraybuffer' });
console.log(response.data);
```

## TypeScript Support

```typescript
interface User {
  id: number;
  name: string;
}

// Type the response data
const response = await cycleTLS.get<User>('/api/user/1');
console.log(response.data.name); // TypeScript knows this is a string
```