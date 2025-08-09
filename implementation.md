# CycleTLS npm Test Implementation Plan

## REQUIRED CHANGES TO src/index.ts

Before implementing the comprehensive test suite, several parameters need to be added to the TypeScript interface to support the new Go functionality:

### Interface Updates Required

#### 1. CycleTLSRequestOptions Interface (Lines 32-67)
**Current Missing Parameters:**
```typescript
export interface CycleTLSRequestOptions {
  // ... existing parameters ...
  
  // MISSING - Connection Management
  enableConnectionReuse?: boolean;    // Enable/disable connection reuse (default: true)
  
  // MISSING - UQuic Integration  
  useUQuic?: boolean;                // Enable UQuic transport for HTTP/3
  uquicConfig?: {                    // UQuic configuration options
    dialTimeout?: number;
    config?: any;                    // uquic.Config equivalent
  };
  
  // MISSING - Enhanced Protocol Options
  quicSpec?: string;                 // QUIC specification for UQuic
  browserFingerprint?: 'chrome' | 'firefox'; // Browser-specific fingerprinting
  
  // MISSING - Advanced Fingerprinting
  akamaiFingerprint?: string;        // Akamai HTTP/2 fingerprint
  akamaiHash?: string;              // Akamai fingerprint hash
  
  // MISSING - Protocol Configuration
  http3Transport?: 'standard' | 'uquic'; // HTTP/3 transport type
  fallbackProtocol?: 'http2' | 'http1';  // Fallback when HTTP/3 fails
  
  // MISSING - Connection Pool Options
  maxConnections?: number;           // Maximum connections per host
  connectionTimeout?: number;        // Connection establishment timeout
  keepAlive?: boolean;              // TCP keep-alive
}
```

#### 2. Enhanced Response Interface
**Add to CycleTLSResponse (Lines 69-81):**
```typescript
export interface CycleTLSResponse {
  // ... existing properties ...
  
  // MISSING - Enhanced Response Data
  protocol?: string;                 // Actual protocol used ("http/1.1", "h2", "h3")
  connectionReused?: boolean;        // Whether connection was reused
  tlsVersion?: string;              // TLS version used
  quicVersion?: string;             // QUIC version if HTTP/3
  serverPush?: any[];               // HTTP/2 server push data
  
  // MISSING - Timing Information
  timings?: {
    dnsLookup?: number;
    tcpConnection?: number;
    tlsHandshake?: number;
    quicHandshake?: number;
    firstByte?: number;
    total?: number;
  };
}
```

#### 3. New Interface for Advanced Options
**Add new interfaces:**
```typescript
export interface QUICFingerprint {
  chromeVersion?: string;            // Chrome version for QUIC fingerprint
  firefoxVersion?: string;           // Firefox version for QUIC fingerprint  
  customSpec?: string;               // Custom QUIC specification
}

export interface ConnectionPool {
  maxIdleConns?: number;            // Max idle connections
  maxIdleConnsPerHost?: number;     // Max idle connections per host
  idleConnTimeout?: number;         // Idle connection timeout
  expectContinueTimeout?: number;   // 100-continue timeout
}

export interface FingerprintOptions {
  ja3?: string;                     // JA3 TLS fingerprint
  ja4?: string;                     // JA4 TLS fingerprint  
  http2?: string;                   // HTTP/2 fingerprint
  quic?: string | QUICFingerprint;  // QUIC fingerprint
  akamai?: string;                  // Akamai fingerprint
}
```

### Default Value Updates Required

#### In initCycleTLS function (around line 754):
```typescript
// Add new default options
if (!options?.enableConnectionReuse) options.enableConnectionReuse = true;
if (!options?.useUQuic) options.useUQuic = false;
if (!options?.http3Transport) options.http3Transport = 'standard';
if (!options?.maxConnections) options.maxConnections = 10;
if (!options?.connectionTimeout) options.connectionTimeout = 30000;
if (!options?.keepAlive) options.keepAlive = true;
```

## COMPREHENSIVE TEST ANALYSIS RESULTS

Based on analysis of both the npm test suite and Go integration tests, this document outlines the comprehensive plan to map all Go test features to equivalent npm tests.

## CRITICAL FINDINGS

### Current npm Test Coverage: 14 Active Test Files
- **connectionReuse.test.ts** - Basic connection reuse (missing parameter testing)
- **encoding.test.ts** - Response compression handling
- **images.test.ts** - Image download/streaming
- **frameHeader.test.ts** - HTTP/2 frame headers
- **integration.test.ts** - Basic HTTP methods
- **timeout.test.ts** - Request timeout handling
- **cookie.test.ts** - Cookie handling
- **cookiejar.test.ts** - Advanced cookie jar management
- And 6 others covering basic functionality...

### MAJOR GAPS IDENTIFIED

#### 1. **JA4 FINGERPRINTING** ❌ COMPLETELY MISSING
- **Go Implementation**: Full JA4 support in `multiple_requests_test.go`
  ```go
  ja4 = "t13d_8a21_3269_e1c9" // Example JA4 fingerprint
  Options: cycletls.Options{
      Ja4: ja4,
      UserAgent: userAgent,
  }
  ```
- **npm Status**: NO tests exist for JA4 parameter
- **Impact**: Major fingerprinting feature completely untested in npm

#### 2. **QUIC/HTTP3 FINGERPRINTING** ❌ MISSING
- **Go Implementation**: Comprehensive QUIC support in `quic_test.go`
  ```go
  const TestQUICFingerprint = "16030106f2010006ee..." // Long QUIC fingerprint
  QUICFingerprint: TestQUICFingerprint,
  ```
- **npm Status**: No QUIC fingerprint tests
- **Impact**: Advanced HTTP/3 evasion capabilities not tested

#### 3. **UQUIC INTEGRATION** ❌ COMPLETELY MISSING
- **Go Implementation**: Advanced UQuic transport with fingerprinting
- **Features**: `UQuicHTTP3Transport`, browser-specific QUIC specs
- **npm Status**: No UQuic integration tests
- **Impact**: Cutting-edge QUIC fingerprinting untested

#### 4. **CONNECTION REUSE PARAMETER** ⚠️ PARTIALLY MISSING
- **Go Implementation**: Explicit `EnableConnectionReuse: true/false` parameter
  ```go
  EnableConnectionReuse: true,  // Enable connection reuse for the test
  EnableConnectionReuse: false, // DISABLE connection reuse for the negative test
  ```
- **npm Status**: Basic connection reuse test exists but missing parameter control
- **Impact**: Cannot test connection reuse disable scenarios

#### 5. **COMMENTED OUT/SKIPPED TESTS** ❌ NEED IMPLEMENTATION
- **websocket.test.ts**: Uses `test.skip()` - "Skip this test for now until the WebSocket functionality is fully implemented"
- **sse.test.ts**: Uses `test.skip()` - "Skip this test for now until the SSE functionality is fully implemented"  
- **multipartFormData.test.ts**: ENTIRE FILE commented out with `//`
- **multipleImports.test.ts**: Complete test function commented out

## IMPLEMENTATION PLAN

### Phase 1: Core New Feature Tests (HIGH PRIORITY)

#### 1.1 JA4 Fingerprinting Test Suite
**File**: `tests/ja4.test.ts`
```typescript
import cycleTLS from '../dist/index.js';

const JA4_FINGERPRINT = 't13d_8a21_3269_e1c9'; // From Go tests
const USER_AGENT = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36';

test('JA4 fingerprint parameter usage', async () => {
  const response = await cycleTLS('https://httpbin.org/get', {
    ja4: JA4_FINGERPRINT,
    userAgent: USER_AGENT,
    timeout: 10000
  });
  
  expect(response.status).toBe(200);
  expect(response.body).toContain('httpbin.org');
});

test('JA4 vs JA3 comparison', async () => {
  const ja3Response = await cycleTLS('https://httpbin.org/get', {
    ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0',
    userAgent: USER_AGENT
  });
  
  const ja4Response = await cycleTLS('https://httpbin.org/get', {
    ja4: JA4_FINGERPRINT,
    userAgent: USER_AGENT
  });
  
  expect(ja3Response.status).toBe(200);
  expect(ja4Response.status).toBe(200);
  // Both should work but potentially with different fingerprinting
});
```

#### 1.2 QUIC/HTTP3 Fingerprinting Test Suite  
**File**: `tests/quic.test.ts`
```typescript
import cycleTLS from '../dist/index.js';

// From Go tests - TestQUICFingerprint (truncated for readability)
const QUIC_FINGERPRINT = '16030106f2010006ee03039a2b98d81139db0e128ea09eff6874549c219b543fb6dbaa7e4dbfe9e31602c620ce04c4026f019442affade7fed8ba66e022e186f77f1c670fd992f33c0143f120020aaaa130113021303c02bc02fc02cc030cca9cca8c013c014009c009d002f0035010006851a1a00000010000e000c02683208687474702f312e31002b000706dada03040303002d00020101000d0012001004030804040105030805050108060601001b0003020002ff0100010000230000000a000c000afafa11ec001d001700180000000e000c0000096c6f63616c686f7374003304ef04edfafa00010011ec04c0...';

test('QUIC fingerprint for HTTP/3', async () => {
  const response = await cycleTLS('https://cloudflare-quic.com/b/test', {
    quicFingerprint: QUIC_FINGERPRINT,
    forceHTTP3: true,
    timeout: 15000
  });
  
  // HTTP/3 might not be available in all environments
  if (response.status === 0) {
    console.log('QUIC HTTP/3 not available in test environment - skipping');
    return;
  }
  
  expect(response.status).toBeGreaterThanOrEqual(200);
});

test('HTTP/3 without QUIC fingerprint', async () => {
  const response = await cycleTLS('https://cloudflare-quic.com/b/test', {
    forceHTTP3: true,
    timeout: 15000
  });
  
  // Should still work without fingerprint
  if (response.status === 0) {
    console.log('HTTP/3 not available in test environment - skipping');
    return;
  }
  
  expect(response.status).toBeGreaterThanOrEqual(200);
});
```

#### 1.3 UQuic Integration Test Suite
**File**: `tests/uquic.test.ts`
```typescript
import cycleTLS from '../dist/index.js';

const CHROME_QUIC_FINGERPRINT = 'QUICChrome_115'; // From Go implementation
const FIREFOX_QUIC_FINGERPRINT = 'QUICFirefox_116'; // From Go implementation

test('UQuic Chrome fingerprint', async () => {
  const response = await cycleTLS('https://cloudflare-quic.com/b/test', {
    quicFingerprint: CHROME_QUIC_FINGERPRINT,
    userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36',
    forceHTTP3: true,
    timeout: 15000
  });
  
  if (response.status === 0) {
    console.log('UQuic Chrome not available - skipping');
    return;
  }
  
  expect(response.status).toBeGreaterThanOrEqual(200);
});

test('UQuic Firefox fingerprint', async () => {
  const response = await cycleTLS('https://cloudflare-quic.com/b/test', {
    quicFingerprint: FIREFOX_QUIC_FINGERPRINT,
    userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:116.0) Gecko/20100101 Firefox/116.0',
    forceHTTP3: true,
    timeout: 15000
  });
  
  if (response.status === 0) {
    console.log('UQuic Firefox not available - skipping');
    return;
  }
  
  expect(response.status).toBeGreaterThanOrEqual(200);
});
```

### Phase 2: Parameter Enhancement Tests (MEDIUM PRIORITY)

#### 2.1 Enhanced Connection Reuse Tests
**File**: `tests/connectionReuseEnhanced.test.ts`
```typescript
import cycleTLS from '../dist/index.js';

test('EnableConnectionReuse: false parameter', async () => {
  const cycleTLSInstance = await cycleTLS.initCycleTLS();
  
  const response = await cycleTLSInstance('https://httpbin.org/get', {
    enableConnectionReuse: false,
    ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0',
    userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36'
  });
  
  expect(response.status).toBe(200);
  
  cycleTLSInstance.exit();
});

test('Connection reuse with different fingerprints', async () => {
  const cycleTLSInstance = await cycleTLS.initCycleTLS();
  
  const response1 = await cycleTLSInstance('https://httpbin.org/get', {
    enableConnectionReuse: true,
    ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0'
  });
  
  const response2 = await cycleTLSInstance('https://httpbin.org/get', {
    enableConnectionReuse: true,
    ja4: 't13d_8a21_3269_e1c9'
  });
  
  expect(response1.status).toBe(200);
  expect(response2.status).toBe(200);
  
  cycleTLSInstance.exit();
});
```

### Phase 3: Activate Commented/Skipped Tests (MEDIUM PRIORITY)

#### 3.1 WebSocket Test Implementation
**File**: `tests/websocket.test.ts` - Remove `test.skip()` and implement:
```typescript
import cycleTLS from '../dist/index.js';

// Remove the test.skip() and implement:
test('WebSocket connection', async () => {
  const response = await cycleTLS('wss://echo.websocket.org/', {
    websocket: true,
    ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0',
    timeout: 10000
  });
  
  expect(response.status).toBe(101); // WebSocket upgrade status
});

test('WebSocket with JA4 fingerprint', async () => {
  const response = await cycleTLS('wss://echo.websocket.org/', {
    websocket: true,
    ja4: 't13d_8a21_3269_e1c9',
    timeout: 10000
  });
  
  expect(response.status).toBe(101);
});
```

#### 3.2 SSE Test Implementation  
**File**: `tests/sse.test.ts` - Remove `test.skip()` and implement:
```typescript
import cycleTLS from '../dist/index.js';

// Remove the test.skip() and implement:
test('Server-Sent Events connection', async () => {
  const response = await cycleTLS('https://httpbin.org/stream/5', {
    sse: true,
    ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0',
    timeout: 15000
  });
  
  expect(response.status).toBe(200);
  expect(response.body).toContain('data:');
});

test('SSE with async iterator', async () => {
  // Test SSE async iterator pattern from Go tests
  const response = await cycleTLS('https://httpbin.org/stream/3', {
    sse: true,
    ja4: 't13d_8a21_3269_e1c9',
    timeout: 15000
  });
  
  expect(response.status).toBe(200);
});
```

#### 3.3 Multipart Form Data Tests
**File**: `tests/multipartFormData.test.ts` - Uncomment all tests:
```typescript
// Remove all the // comment markers and fix the tests
import cycleTLS from '../dist/index.js';

test('Multipart form data submission', async () => {
  const formData = {
    field1: 'value1',
    field2: 'value2'
  };
  
  const response = await cycleTLS('https://httpbin.org/post', {
    body: formData,
    multipart: true,
    ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0'
  }, 'POST');
  
  expect(response.status).toBe(200);
});

test('File upload with multipart', async () => {
  // Implement file upload test
  const response = await cycleTLS('https://httpbin.org/post', {
    files: [{
      name: 'testfile',
      filename: 'test.txt', 
      content: 'test file content'
    }],
    multipart: true,
    ja4: 't13d_8a21_3269_e1c9'
  }, 'POST');
  
  expect(response.status).toBe(200);
});
```

### Phase 4: Integration & Edge Cases (LOW PRIORITY)

#### 4.1 Cross-Feature Integration Tests
**File**: `tests/integration.advanced.test.ts`
```typescript
import cycleTLS from '../dist/index.js';

test('JA4 + QUIC fingerprint combination', async () => {
  const response = await cycleTLS('https://cloudflare-quic.com/b/test', {
    ja4: 't13d_8a21_3269_e1c9',
    quicFingerprint: 'QUICChrome_115',
    forceHTTP3: true,
    enableConnectionReuse: true,
    timeout: 15000
  });
  
  if (response.status === 0) {
    console.log('Advanced fingerprinting not available - skipping');
    return;
  }
  
  expect(response.status).toBeGreaterThanOrEqual(200);
});

test('Protocol fallback testing', async () => {
  // Test HTTP/3 to HTTP/2 fallback
  const response = await cycleTLS('https://httpbin.org/get', {
    forceHTTP3: true,
    ja4: 't13d_8a21_3269_e1c9',
    timeout: 10000
  });
  
  // Should fallback to HTTP/2 or HTTP/1.1 if HTTP/3 not available
  expect(response.status).toBe(200);
});
```

## IMPLEMENTATION CHECKLIST

### Phase 1: Core New Feature Tests (HIGH PRIORITY)
- [ ] **JA4 Parameter Test** - Create `tests/ja4.test.ts`
- [ ] **JA4 vs JA3 Comparison** - Test both fingerprinting methods
- [ ] **QUIC Fingerprint Test** - Create `tests/quic.test.ts`
- [ ] **HTTP/3 QUIC Integration** - Test HTTP/3 requests with QUIC fingerprints
- [ ] **UQuic Chrome/Firefox Tests** - Create `tests/uquic.test.ts`

### Phase 2: Parameter Enhancement Tests (MEDIUM PRIORITY)  
- [ ] **EnableConnectionReuse: false Test** - Create `tests/connectionReuseEnhanced.test.ts`
- [ ] **Connection Reuse Combinations** - Test with different fingerprints
- [ ] **Worker Pool Disable Test** - Test `initCycleTLS(false)` parameter usage

### Phase 3: Activate Commented/Skipped Tests (MEDIUM PRIORITY)
- [ ] **WebSocket Implementation** - Remove `test.skip()` from `tests/websocket.test.ts`
- [ ] **SSE Implementation** - Remove `test.skip()` from `tests/sse.test.ts`  
- [ ] **Multipart Form Tests** - Uncomment all tests in `tests/multipartFormData.test.ts`
- [ ] **Multiple Instance Test** - Uncomment and fix `tests/multipleImports.test.ts`

### Phase 4: Integration & Edge Cases (LOW PRIORITY)
- [ ] **JA4 + QUIC Combination** - Test using both fingerprinting methods
- [ ] **Error Handling** - Test invalid fingerprint parameters
- [ ] **Browser Simulation** - Test Chrome vs Firefox specific behaviors
- [ ] **Protocol Fallback** - Test HTTP/3 to HTTP/2 fallback scenarios

## TESTING INFRASTRUCTURE REQUIREMENTS

### New Test Dependencies
- May need to add test endpoints that support HTTP/3/QUIC
- Consider using Cloudflare QUIC test endpoints
- Ensure test environments support advanced fingerprinting

### Test Environment Considerations
- QUIC/HTTP3 tests may fail in environments without support
- Should include graceful skipping for unsupported features
- Consider adding environment detection for feature availability

### Performance Considerations  
- QUIC tests may be slower due to protocol overhead
- Consider increasing timeouts for HTTP/3 tests
- Connection reuse tests should verify actual connection behavior

## EXPECTED OUTCOMES

After implementing this plan:
1. **Complete Feature Parity**: npm tests will cover all Go functionality
2. **Enhanced Test Coverage**: All new fingerprinting features tested
3. **Regression Prevention**: Comprehensive test suite prevents feature breaks
4. **Documentation**: Tests serve as usage examples for new features

This implementation plan ensures that the npm test suite provides comprehensive coverage for all the advanced features implemented in the Go backend, particularly the new QUIC/HTTP3 fingerprinting and JA4 support that are currently completely missing from the npm tests.