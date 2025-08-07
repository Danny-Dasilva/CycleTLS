# CycleTLS Changelog

## 2.0.0 - (8-5-2025)
### Release Highlights
⚠️ **MAJOR BREAKING CHANGES** ⚠️

This is a major release with **breaking changes** to the JavaScript/TypeScript API. Please review the migration guide below before upgrading.

New features include HTTP/3, WebSocket, Server-Sent Events, JA4 fingerprinting, and binary response support. Significant performance improvements through connection reuse and enhanced protocol support.

### New Features
- **HTTP/3 Support** - Full HTTP/3 protocol implementation with QUIC transport, custom QUIC configuration options, and connection pooling
- **WebSocket Support** - Native WebSocket client implementation with event-based API (onMessage, onClose, onError callbacks) and TLS fingerprinting integration [#34](https://github.com/Danny-Dasilva/CycleTLS/issues/34)
- **Server-Sent Events (SSE)** - SSE client implementation for real-time event streaming with automatic reconnection and AsyncIterableIterator API [#274](https://github.com/Danny-Dasilva/CycleTLS/issues/274)
- **JA4 Fingerprinting** - Implementation of JA4 TLS fingerprinting (successor to JA3) combining TLS version, cipher suites, extensions, HTTP headers, and User-Agent [#112](https://github.com/Danny-Dasilva/CycleTLS/issues/112)
- **HTTP/2 Fingerprinting** - Enhanced HTTP/2 client fingerprinting with custom settings, stream dependencies, and priority orders [#112](https://github.com/Danny-Dasilva/CycleTLS/issues/112)
- **Binary Response Support** - Streaming support for binary responses with async dispatcher and improved memory efficiency [#371](https://github.com/Danny-Dasilva/CycleTLS/pull/371), [#274](https://github.com/Danny-Dasilva/CycleTLS/issues/274), [#380](https://github.com/Danny-Dasilva/CycleTLS/issues/380)
- **Connection Reuse** - Persistent connection pooling to reduce TLS handshakes and improve performance [#281](https://github.com/Danny-Dasilva/CycleTLS/issues/281)

### API Enhancements

#### JavaScript/TypeScript API Changes
- **Fetch-like Response Methods** - New response methods: `response.json()`, `response.text()`, `response.arrayBuffer()`, `response.blob()` for consistent data handling
- **HTTP Method Shortcuts** - Added convenient methods: `cycleTLS.get()`, `cycleTLS.post()`, `cycleTLS.put()`, `cycleTLS.delete()`, `cycleTLS.head()`, `cycleTLS.options()`, `cycleTLS.patch()`
- **WebSocket Support** - New `cycleTLS.ws()` and `cycleTLS.webSocket()` methods for WebSocket connections with event-based API
- **Server-Sent Events** - New `cycleTLS.sse()` and `cycleTLS.eventSource()` methods for SSE connections with async iterator support
- **Enhanced Initialization** - Optional configuration support: `initCycleTLS({ port: 9118, timeout: 30000 })`
- **Streaming Response Support** - New `responseType: 'stream'` option for efficient handling of large responses
- **Improved Form Data Handling** - Better multipart form data support with `formData.getHeaders()`

#### Protocol-Specific Methods
- **WebSocket Support** - `cycleTLS.ws()` / `cycleTLS.webSocket()` for WebSocket connections with event-based API
- **Server-Sent Events** - `cycleTLS.sse()` / `cycleTLS.eventSource()` for SSE connections with async iterator support
- **HTTP/3 Protocol** - `forceHTTP3` option to explicitly use HTTP/3 protocol
- **Protocol Selection** - `protocol` parameter to specify connection type: "http1", "http2", "http3", "websocket", "sse"

### ⚠️ BREAKING CHANGES ⚠️

```
🚨🚨🚨 CRITICAL BREAKING CHANGES 🚨🚨🚨

Your JavaScript/TypeScript code WILL BREAK if you don't update it!
Do NOT upgrade to v2.0.0 without reading the migration guide below.

❌ response.body is REMOVED
✅ Use response.json(), response.text(), etc. instead
```

---

#### 🚨 CRITICAL CHANGES (WILL BREAK YOUR CODE)

| Change Type | Old (v1.x) | New (v2.0.0) | Status |
|-------------|------------|--------------|--------|
| **Response Data Access** | `response.body` | `response.json()` / `response.text()` | ❌ **REMOVED** |
| **Form Data Headers** | Manual `Content-Type` | `formData.getHeaders()` | ⚠️ **DEPRECATED** |
| **Response Methods** | Synchronous | Asynchronous (returns Promises) | ⚠️ **CHANGED** |

---

#### 📋 DETAILED BREAKING CHANGES

**1. ⚠️ Response Body Access (CRITICAL)**
- Direct `response.body` access **REMOVED**
- Must use: `response.json()`, `response.text()`, `response.arrayBuffer()`, or `response.blob()` methods
- Impact: **ALL existing code that accesses response.body will break**

**2. ⚠️ Form Data Headers (RECOMMENDED)**
- Manual `Content-Type: multipart/form-data` headers **DEPRECATED**
- Must use: `formData.getHeaders()` instead
- Impact: **Form uploads may fail without proper boundary headers**

**3. ⚠️ Async Response Methods (BEHAVIORAL CHANGE)**
- All response data access is now asynchronous and returns Promises
- Must use `await` or `.then()` with response methods
- Impact: **Synchronous response handling will not work**

**4. ✨ NEW API Methods (ADDITIONS - NON-BREAKING)**
- Added `cycleTLS.ws()`, `cycleTLS.webSocket()` for WebSocket connections
- Added `cycleTLS.sse()`, `cycleTLS.eventSource()` for Server-Sent Events
- Added HTTP method shortcuts: `cycleTLS.get()`, `cycleTLS.post()`, etc.
- Impact: **No breaking changes - these are new features**

---

### 📝 JavaScript/TypeScript Examples

```
📚 EXAMPLE LIBRARY - All New v2.0.0 API Patterns

Use these examples as templates for updating your code!
Each example shows the complete, working v2.0.0 syntax.
```

#### ✅ Basic Request with New Response API
```javascript
const initCycleTLS = require('cycletls');

(async () => {
  const cycleTLS = await initCycleTLS();
  
  // ✅ Using new response methods (REQUIRED in v2.0.0)
  const response = await cycleTLS('https://httpbin.org/json', {
    ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
  });
  
  const data = await response.json(); // ✅ REQUIRED: Parse as JSON
  console.log(data);
  
  cycleTLS.exit();
})();
```

#### ✨ NEW: Streaming Response
```javascript
// ✨ NEW FEATURE in v2.0.0
const response = await cycleTLS('https://httpbin.org/stream/3', {
  responseType: 'stream' // ✨ NEW option
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

#### ✨ NEW: WebSocket Connection
```javascript
// ✨ BRAND NEW FEATURE in v2.0.0
const wsResponse = await cycleTLS.ws('wss://echo.websocket.org', {
  ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
  userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
});

// ✨ NEW: Set up message handler
wsResponse.onMessage((message) => {
  if (message.type === 'text') {
    console.log('Received:', message.data.toString());
  }
});

// ✨ NEW: Send a message
const testMessage = 'Hello, WebSocket!';
await wsResponse.send(testMessage);

// ✨ NEW: Close connection
await wsResponse.close();
```

#### ✨ NEW: Server-Sent Events (SSE)
```javascript
// ✨ BRAND NEW FEATURE in v2.0.0
// Event-based approach
const sseResponse = await cycleTLS.sse('https://example.com/events', {
  ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
  userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
});

// ✨ NEW: Event handler
sseResponse.onEvent((event) => {
  console.log('SSE Event:', event.data);
});

// ✨ NEW: Async iterator approach
for await (const event of sseResponse.events()) {
  console.log('Event:', event.data);
  if (event.data === 'done') break;
}

await sseResponse.close();
```

#### ✨ NEW: HTTP Method Shortcuts
```javascript
// ✨ NEW FEATURE: Convenient method shortcuts (similar to axios)
const getResponse = await cycleTLS.get('https://httpbin.org/get', options);
const getData = await getResponse.json(); // ⚠️ Don't forget this!

const postResponse = await cycleTLS.post('https://httpbin.org/post', {
  body: JSON.stringify({ key: 'value' }),
  headers: { 'Content-Type': 'application/json' }
});
const postData = await postResponse.json(); // ⚠️ Don't forget this!
```

---

### 🔄 MIGRATION GUIDE (v1.x → v2.0.0)

```
🛠️ STEP-BY-STEP MIGRATION CHECKLIST

□ 1. Update all response.body references
□ 2. Fix form data headers
□ 3. Add error handling for async methods
□ 4. (Optional) Use new API features
```

**⚠️ REQUIRED CHANGES - Your code will break without these updates:**

---

#### ⚠️ STEP 1: Update Response Handling (REQUIRED)

**🚨 CRITICAL: This change affects ALL existing code**

```javascript
// ❌ OLD (v1.x) - THIS WILL NO LONGER WORK
const response = await cycleTLS(url, options);
console.log(response.body); // ❌ response.body is REMOVED

// ✅ NEW (v2.0.0) - CHOOSE THE RIGHT METHOD
const response = await cycleTLS(url, options);

// For JSON APIs (most common)
const data = await response.json();
console.log(data);

// For HTML/plain text
const text = await response.text();
console.log(text);

// For binary data (images, files)
const buffer = await response.arrayBuffer();
console.log(buffer);

// For blob data
const blob = await response.blob();
console.log(blob);
```

**📝 Quick Fix Pattern:**
```javascript
// Find and replace in your codebase:
// OLD: response.body
// NEW: await response.json()  (for JSON)
// NEW: await response.text()  (for text)
```

---

#### ⚠️ STEP 2: Update Form Data Headers (STRONGLY RECOMMENDED)

**🔧 ISSUE: Manual Content-Type headers break multipart boundaries**

```javascript
// ❌ OLD (v1.x) - WILL CAUSE UPLOAD FAILURES
const formData = new FormData();
formData.append('file', fileStream);

const response = await cycleTLS(url, {
  body: formData,
  headers: {
    'Content-Type': 'multipart/form-data' // ❌ Missing boundary!
  }
});

// ✅ NEW (v2.0.0) - PROPER BOUNDARY HANDLING
const formData = new FormData();
formData.append('file', fileStream);

const response = await cycleTLS(url, {
  body: formData,
  headers: formData.getHeaders() // ✅ Includes proper boundary
});
const result = await response.json(); // ✅ Parse response too!
```

**📝 Why This Matters:**
Multipart form data requires unique boundaries. Manual headers miss this critical detail.

---

#### ⚠️ STEP 3: Add Error Handling (RECOMMENDED)

**🛡️ PROTECTION: Response parsing can now fail**

```javascript
// ✅ ROBUST ERROR HANDLING
try {
  const response = await cycleTLS(url, options);
  
  // Check if request was successful
  if (response.status >= 400) {
    throw new Error(`Request failed: ${response.status}`);
  }
  
  // Parse response (can throw parsing errors)
  const data = await response.json();
  console.log(data);
  
} catch (error) {
  if (error.message.includes('JSON')) {
    console.error('Response is not valid JSON:', error.message);
  } else {
    console.error('Request failed:', error.message);
  }
}
```

**🔍 Common Errors to Handle:**
- **JSON parsing errors**: Server returned non-JSON data
- **Network errors**: Connection failed
- **HTTP errors**: 4xx/5xx status codes

---

#### ✨ STEP 4: Explore New Features (OPTIONAL)

**🎉 BONUS: Take advantage of new capabilities**

```javascript
// ✨ HTTP METHOD SHORTCUTS (cleaner code)
const getResponse = await cycleTLS.get(url, options);
const postResponse = await cycleTLS.post(url, options);
const putResponse = await cycleTLS.put(url, options);
const deleteResponse = await cycleTLS.delete(url, options);
// ... and more!

// ⚠️ Still need to parse responses!
const data = await getResponse.json();
```

```javascript
// ✨ WEBSOCKET CONNECTIONS
const wsResponse = await cycleTLS.ws('wss://echo.websocket.org', {
  ja3: 'your_ja3_string',
  userAgent: 'your_user_agent'
});

// Check connection status
if (wsResponse.status === 101) {
  console.log('WebSocket connected!');
}
```

```javascript
// ✨ SERVER-SENT EVENTS
const sseResponse = await cycleTLS.sse('https://example.com/events', {
  ja3: 'your_ja3_string',
  userAgent: 'your_user_agent'
});

// Parse real-time events
const eventData = await sseResponse.text();
console.log('SSE events:', eventData);
```

```javascript
// ✨ STREAMING RESPONSES (for large data)
const response = await cycleTLS('https://example.com/large-file', {
  responseType: 'stream'
});

const stream = response.data;
stream.on('data', chunk => {
  console.log('Received:', chunk.length, 'bytes');
});
stream.on('end', () => {
  console.log('Download complete!');
});
```

---

### 🐛 Bug Fixes
- Fix uncaught `ESRCH` on `SIGINT`/`SIGTERM` signals [#370](https://github.com/Danny-Dasilva/CycleTLS/issues/370)
- Improved error handling for Windows systems
- Fixed syntax issues with redirects
- Resolved deadlock issues on Linux
- Better proxy header handling
- General test fixes and stability improvements

---

### ✅ What Remains Unchanged (Backward Compatible)

**🎯 GOOD NEWS: These parts of your code don't need changes**

| Category | Details | Status |
|----------|---------|--------|
| **Core Syntax** | `cycleTLS(url, options, method)` | ✅ **Unchanged** |
| **Request Options** | `ja3`, `userAgent`, `proxy`, `timeout`, etc. | ✅ **Unchanged** |
| **Response Properties** | `response.status`, `response.headers`, `response.finalUrl` | ✅ **Unchanged** |
| **Lifecycle Methods** | `cycleTLS.exit()`, `initCycleTLS()` | ✅ **Unchanged** |
| **Golang API** | All Go package methods and types | ✅ **100% Compatible** |

**📋 What This Means:**
- Your request configuration stays the same
- Your TLS fingerprinting setup works unchanged
- Your proxy and authentication logic is preserved
- Only response parsing needs updates

## 1.0.26 - (2-16-2024)
### Release Highlights
Fix illegal parameter error and location url error

### Enhancements
- [Default JA3 and UA](https://github.com/Danny-Dasilva/CycleTLS/issues/322)
- [Clearer Docs around HTTP1.1 support](https://github.com/Danny-Dasilva/CycleTLS/issues/321)
### Bug Fixes
-  [302 Location Url Error](https://github.com/Danny-Dasilva/CycleTLS/issues/323) thanks to @lif0
- Fix error: remote error: tls: illegal parameter based on incorrect TLS version


## 1.0.25 - (1-08-2024)
### Release Highlights
Support extension 65037 as well as socks5h and socks4 proxies

### Enhancements
- Support Socks4
- [Support Socks5H](https://github.com/Danny-Dasilva/CycleTLS/issues/301)
### Bug Fixes
-  [Add support for Extension 65037](https://github.com/Danny-Dasilva/CycleTLS/issues/138)

## 1.0.24 - (11-27-2023)
### Release Highlights
Small bugfixes and ReadMe updates

### Enhancements
- Update docs
- Add tests for forceHTTP1
- update to using io instead of ioutil
### Bug Fixes
- Fix issue with keepalives on closed connections when http1 is forced

## 1.0.23 - (11-27-2023)
### Release Highlights
Switch to UTLS
Add Insecure Skip Verify for proxy bypass
Add Force HTTP1
### Enhancements
-   [Add forceHTTP1 param to configure http version](https://github.com/Danny-Dasilva/CycleTLS/issues/268)
-  add InsecureSkipVerify configuration option enabling bypassing the SSL certificate verification when making HTTP requests addressed by @lif0 in [this PR](https://github.com/Danny-Dasilva/CycleTLS/pull/279)
- [Add finalUrl response to track redirects ](https://github.com/Danny-Dasilva/CycleTLS/issues/121) @lif0 in [this PR](https://github.com/Danny-Dasilva/CycleTLS/pull/283)
- [Add support for application/x-www-form-urlencoded](https://github.com/Danny-Dasilva/CycleTLS/issues/155)
- [Add support for multipart/form-data](https://github.com/Danny-Dasilva/CycleTLS/issues/89)
- [Add CycleTLS Queue Example and test](https://github.com/Danny-Dasilva/CycleTLS/issues/128)
- [Add CookieJar Support in Golang](https://github.com/Danny-Dasilva/CycleTLS/issues/260), [duplicate](https://github.com/Danny-Dasilva/CycleTLS/issues/146), [other duplicate](https://github.com/Danny-Dasilva/CycleTLS/issues/222)
- [tag submodule cycletls](https://github.com/Danny-Dasilva/CycleTLS/issues/232)
- [Export transport for http client](https://github.com/Danny-Dasilva/CycleTLS/issues/151), addressed by @deoxykev in [this PR](https://github.com/Danny-Dasilva/CycleTLS/pull/286)
- Add support for ja3 `24` FakeTokenBindingExtension
- Add support for ja3 `34` DelegatedCredentialsExtension
- Add support for ja3 `41` UtlsPreSharedKeyExtension
- Add support for ja3 `50` SignatureAlgorithmsCertExtension
- Add support for ja3 `57` QUICTransportParametersExtension

## 1.0.22 - (10-24-2023)
### Release Highlights
Support for additional filetypes and memory leak fix
Update Golang version to 17 
[New Discord Community](https://discord.gg/gsmxMHrwhu) 

### Enhancements
- [Missing Gif/Avif/... support in Decoding ](https://github.com/Danny-Dasilva/CycleTLS/issues/262)
- [Discord Community](https://github.com/Danny-Dasilva/CycleTLS/issues/206)
### Bug Fixes
- [Memory leaks builded package on Docker container](https://github.com/Danny-Dasilva/CycleTLS/issues/157)
- [There are lot's of memory leaks, DDoS and infinite loop if any error in NodeJS](https://github.com/Danny-Dasilva/CycleTLS/issues/264) 
  * golang request closing memory leak issue [addressed by](https://github.com/Danny-Dasilva/CycleTLS/pull/265) @abler98 
  * [npm memory leak issue addressed](https://github.com/Danny-Dasilva/CycleTLS/pull/266/commits/7251f03aa0fec15093ff29e21a5aae4064c1aa0b) by @Stephen-PP
- [is there a way to support socks5 proxy?](https://github.com/Danny-Dasilva/CycleTLS/issues/263) addressed by @ChengHoward
- [Support Socks5/4](https://github.com/Danny-Dasilva/CycleTLS/issues/77) addressed by @ChengHoward


## 1.0.21 - (6-20-2022)
### Release Highlights
Json Parse body in response and Cookie examples
### Enhancements
- Return Json body instead of string if response is type Json 🚨 BREAKING CHANGE (users no longer need to `JSON.parse` the response request body)
- Changed basic api to not require CycleTLS `config`
- Add Cookie examples in readme and update Cookie API
- WIP golang release tags and dependabot alerts 
### Bug Fixes
- [Added support for TLS Certificate Compression (causing cloudflare handshake issues)](https://github.com/Danny-Dasilva/CycleTLS/issues/103)
- [Send Correct Frame headers](https://github.com/Danny-Dasilva/CycleTLS/issues/101)
- [Add Temporary support for extension 17](https://github.com/Danny-Dasilva/CycleTLS/issues/100)

## 0.0.20 - (5-16-2022)
### Release Highlights
Add support for extension 17513 and repo cleanup
### Enhancements
- Changed default request timeout to 15 seconds
- [remove duplicate sources](https://github.com/Danny-Dasilva/CycleTLS/issues/85)
- [Added support for extension 17513](https://github.com/Danny-Dasilva/CycleTLS/issues/70)
- add support for `application/pdf`, 
### Bug Fixes
- [properly add grease](https://github.com/Danny-Dasilva/CycleTLS/issues/53)


## 0.0.19 - (4-20-2022)
### Release Highlights
Added image and Arm support
### Enhancements
- [Arm and Arm64 compilation support](https://github.com/Danny-Dasilva/CycleTLS/issues/86)
- [Images are now Base64 encoded for Javascript](https://github.com/Danny-Dasilva/CycleTLS/issues/76)
### Bug Fixes
- [Fix installation dependencies](https://github.com/Danny-Dasilva/CycleTLS/issues/84)
- Don't crash if golang websocket server receives a normal http request
- Update npm dependencies
- Added site support from updates to [fhttp](https://github.com/Danny-Dasilva/fhttp) and [utls](https://github.com/Danny-Dasilva/utls)

## 0.0.18 - (3-26-2022)
### Release Highlights
Swap Golang and TS server
### Enhancements
- Golang is now handles incoming requests, temporary solution till [this](https://github.com/Danny-Dasilva/CycleTLS/issues/55) is addressed 
### Bug Fixes
- [Don't kill process if it does not exist](https://github.com/Danny-Dasilva/CycleTLS/pull/71)

## 0.0.17 - (3-14-2022)
### Release Highlights
Fix fhttp handshake 
### Bug Fixes
- Added utls to the fhttp package and fix imports

## 0.0.16 - (2-15-2022)
### Release Highlights
Fix setting request headers
### Bug Fixes
- [Fix request Headers not being set](https://github.com/Danny-Dasilva/CycleTLS/issues/60)
### Enhancements
- Add request header integration test

## 0.0.15 - (2-11-2022)
### Release Highlights
Support Ordered Request Headers
### Bug Fixes
- Properly form Grease Headers based on UserAgent 
- Refactor error handling 
### Enhancements
- Ordered Request Headers
- [Gzip/brotly/deflate decoding](https://github.com/Danny-Dasilva/CycleTLS/issues/48)
- [Queue requests in case of Process restart](https://github.com/Danny-Dasilva/CycleTLS/issues/50)

## 0.0.14 - (10-06-2021)
### Release Highlights
Bugfix Ja3 Token Parsing and better error handling
### Bug Fixes
- Fix Stack overlflow if token is unsuported [Some Ja3 strings causing stackoverflow](https://github.com/Danny-Dasilva/CycleTLS/issues/51)
- Address Ticket [Spoofed TLS/JA3 fingerprint is detected by some sites](https://github.com/Danny-Dasilva/CycleTLS/issues/47)
- Address Ticket [Set User-Agent for the CONNECT request from the config](https://github.com/Danny-Dasilva/CycleTLS/issues/46)

## 0.0.13 - (09-16-2021)
### Release Highlights
Performance improvements and Better Ja3 Token Parsing 
### Bug Fixes
- Fix Performance issue relating to worker pool [Related Ticket](https://github.com/Danny-Dasilva/CycleTLS/issues/41)
- Address Ticket (fix extension 43) [error: tls: server selected unsupported group](https://github.com/Danny-Dasilva/CycleTLS/issues/43)
- Address Ticket [ fatal error: stack overflow (added support for extensions 22 49 50) ](https://github.com/Danny-Dasilva/CycleTLS/issues/42)
### Enhancements
- Updated Documentation
- Updated Integration tests for new token fixes
- Add new issue template

## 0.0.12 - (08-14-2021)
### Release Highlights
Timeout and Disable redirect integration
### Bug Fixes
- Change to Stable node version V 14 for release
- Address Ticket [Golang cmd window not hiding](https://github.com/Danny-Dasilva/CycleTLS/issues/30)
- Address Ticket [ Method in CycleTLS.Do isn't used ](https://github.com/Danny-Dasilva/CycleTLS/issues/33)
### Enhancements
- Add timeout [ Add a timeout feature ](https://github.com/Danny-Dasilva/CycleTLS/issues/31)
- Optionally disable auto redirect [ Disable auto-redirect ](https://github.com/Danny-Dasilva/CycleTLS/issues/32)
- Add integration tests for timeout and redirect
- Make client exportable [ Make client interface exportable for easy use ](https://github.com/Danny-Dasilva/CycleTLS/pull/37)
- Update CI release with stable node version

## 0.0.11 - (06-6-2021)
### Release Highlights
Memory leak fixes and Clean Exit 
### Bug Fixes
- Address Ticket [Memory Leak on Exit ](https://github.com/Danny-Dasilva/CycleTLS/issues/28) with added unit test
- General Linting on yaml files

### Enhancements
- CI/CD changes for js tests
- New Jest test for clean exit 
- Update ReadMe

## 0.0.10 - (06-4-2021)
### Release Highlights
New Error Handling and bufixes
### Bug Fixes
- Address Ticket [initCycleTLS is not a function](https://github.com/Danny-Dasilva/CycleTLS/issues/26) with added unit test
- Add type hints in TLS transport map

### Enhancements
- New Errors file for checking proxy errors
- Timeout now hard coded to 6 seconds

## 0.0.9 - (05-28-2021)
### Release Highlights
New Jest CI Tests and general bugfixes

### Bug Fixes
- Fix breaking error with file path spaces
- Fix Proxy connection refused and set it to error -> 407
- Fix incorrect proxy url error

### Enhancements
- Add Jest integration tests
- Add Github actions for the above integration test
- Add Cycletls exit api once requests are finished
- CI fixes
- Add Changelog 

## 0.0.8 - (05-24-2021)
### Release Highlights
add fix for crashing js instance

### Bug Fixes
- Fmt or Log printing in golang was causing crashing issues


## 0.0.7 - (05-18-2020)
### Release Highlights
New error logging addition

### Bug Fixes
- Failed timeout requests will now return a 408 instead of crashing the instance, this will help CycleTLS run through Proxy Lists without crashing

