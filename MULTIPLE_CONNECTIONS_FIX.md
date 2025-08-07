# Multiple Connections Architecture Fix

## Overview

This document explains the architectural changes made to CycleTLS to enable multiple `initCycleTLS` instances to share the same port without conflicts. The solution implements a **Shared Instance Manager** pattern that allows multiple TypeScript clients to connect to a single Go server process.

## Problem Statement

### Original Issue
- Multiple `initCycleTLS({ port: 9998 })` calls would fail
- Second instance would hang indefinitely, never completing requests
- Global variables (`child`, `lastRequestID`, `isShuttingDown`) caused state conflicts
- No proper coordination between instances

### Expected Behavior
- Multiple Node.js instances should share one Go server per port
- First instance spawns the server, subsequent instances connect as clients
- When the host instance exits, proper cleanup coordination should occur
- Each client should operate independently with isolated state

## Root Cause Analysis

The original architecture had several critical flaws:

### 1. **Global State Conflicts**
```typescript
// OLD: Shared across all instances
let child: ChildProcessWithoutNullStreams;
let lastRequestID: string;
let isShuttingDown = false;
```

### 2. **Event Emitter Interference**
- All instances shared the same `Golang` class with a single EventEmitter
- Messages from the Go server were broadcast to all instances
- Request IDs could collide between instances

### 3. **Process Management Issues**
- Only one instance could "own" the child process
- Cleanup was uncoordinated, leading to race conditions
- No reference counting for proper lifecycle management

## Solution Architecture

### 1. **Shared Instance Manager (Singleton)**

The `InstanceManager` class tracks shared instances by port:

```typescript
class InstanceManager {
  private sharedInstances: Map<number, SharedInstance> = new Map();
  
  async getOrCreateSharedInstance(port: number): Promise<SharedInstance> {
    let sharedInstance = this.sharedInstances.get(port);
    
    if (!sharedInstance) {
      sharedInstance = new SharedInstance(port, debug, timeout, executablePath);
      this.sharedInstances.set(port, sharedInstance);
      await sharedInstance.initialize();
    }
    
    return sharedInstance;
  }
}
```

**Benefits:**
- One `SharedInstance` per port across the entire process
- Automatic creation and reuse of shared instances
- Centralized lifecycle management

### 2. **SharedInstance (Go Server Manager)**

The `SharedInstance` class manages one Go server and multiple client connections:

```typescript
class SharedInstance extends EventEmitter {
  private child: ChildProcessWithoutNullStreams | null = null;
  private server: WebSocket | null = null;
  private clients: Map<string, CycleTLSClient> = new Map();
  private isHost: boolean = false;
  
  // Port binding logic determines host vs client
  private checkSpawnedInstance(): void {
    this.httpServer = http.createServer();
    
    this.httpServer.once('listening', () => {
      // Successfully bound to port -> become host
      this.spawnServer();
      this.isHost = true;
    });
    
    this.httpServer.once('error', (err) => {
      // Port already in use -> become client
      this.createClient();
      this.isHost = false;
    });
  }
}
```

**Key Features:**
- **Host Detection**: Uses HTTP server binding to determine if it should spawn Go server or connect as client
- **Message Routing**: Routes WebSocket messages to appropriate clients based on request ID
- **Reference Counting**: Tracks connected clients for proper cleanup timing
- **Process Isolation**: Only the host instance manages the Go child process

### 3. **CycleTLSClient (Individual Connection)**

Each `CycleTLSClient` represents one connection to the shared instance:

```typescript
class CycleTLSClient extends EventEmitter {
  private sharedInstance: SharedInstance;
  private clientId: string;
  
  constructor(sharedInstance: SharedInstance) {
    super();
    this.clientId = `client-${Date.now()}-${Math.floor(10000 * Math.random())}`;
    this.sharedInstance.addClient(this.clientId, this);
  }
  
  async request(url: string, options: CycleTLSRequestOptions, method: string): Promise<CycleTLSResponse> {
    const requestId = `${this.clientId}#${url}#${Date.now()}-${Math.floor(1000 * Math.random())}`;
    // Send request through shared instance...
  }
}
```

**Benefits:**
- **Unique Client IDs**: Each client gets a unique identifier for request isolation
- **Independent State**: Each client maintains its own connection tracking and event handling  
- **Backward Compatibility**: Provides the exact same API as the original implementation

## Port Sharing Mechanism

### 1. **Host/Client Determination**

```typescript
// SharedInstance.checkSpawnedInstance()
this.httpServer = http.createServer();

this.httpServer.once('listening', () => {
  // Port is free -> I am the host
  this.httpServer.close();
  this.spawnServer();  // Start Go process
  this.isHost = true;
});

this.httpServer.once('error', (err) => {
  // Port is busy -> I am a client  
  this.createClient(); // Connect to existing Go server
  this.isHost = false;
});

this.httpServer.listen(this.port);
```

### 2. **Message Routing**

```typescript
// In SharedInstance.createClient()
this.server.on("message", (data: Buffer) => {
  const requestID = packetBuffer.readString();
  
  // Extract client ID from request ID
  const clientId = this.extractClientIdFromRequestId(requestID);
  const client = this.clients.get(clientId);
  
  if (client) {
    // Route message to the specific client
    client.emit(requestID, responseData);
  }
});

// Request ID format: ${clientId}#${url}#${timestamp}-${random}
private extractClientIdFromRequestId(requestId: string): string {
  return requestId.split('#')[0];
}
```

### 3. **Reference Counting Cleanup**

```typescript
removeClient(clientId: string): void {
  this.clients.delete(clientId);
  
  // If no more clients, cleanup the shared instance
  if (this.clients.size === 0) {
    InstanceManager.getInstance().removeSharedInstance(this.port);
  }
}
```

## Benefits of the New Architecture

### ✅ **True Port Sharing**
- Multiple `initCycleTLS` calls on the same port now work seamlessly
- Only one Go server process per port, regardless of how many TypeScript clients connect

### ✅ **Instance Isolation** 
- Each client operates independently with its own event emitters
- Request IDs are globally unique: `${clientId}#${url}#${timestamp}-${random}`
- No shared global state between instances

### ✅ **Proper Lifecycle Management**
- Reference counting ensures Go server only shuts down when last client disconnects
- Host instance coordinates cleanup of the shared Go process
- Graceful handling of client disconnections

### ✅ **Backward Compatibility**
- Public API remains exactly the same
- All existing tests pass without modification
- Drop-in replacement for existing code

### ✅ **Resource Efficiency**
- One Go process per port instead of one per TypeScript instance
- Reduced memory and CPU usage for multiple connections
- Better connection reuse within the shared Go server

## Testing Results

### Multiple Connection Test
```javascript
// This now works perfectly:
const cycleTLS1 = await initCycleTLS({ port: 9998 });
const cycleTLS2 = await initCycleTLS({ port: 9998 }); // Reuses existing server

const response1 = await cycleTLS1.get("https://httpbin.org/get");
const response2 = await cycleTLS2.get("https://httpbin.org/get");

// Both requests complete successfully
console.log(response1.status); // 200 ✅
console.log(response2.status); // 200 ✅
```

### Backward Compatibility
All existing functionality continues to work:
- ✅ `cookiejar.test.js` - Cookie handling and response methods
- ✅ `images.test.ts` - Binary data and arrayBuffer support  
- ✅ `integration.test.ts` - Complex request scenarios
- ✅ `simple-connection.test.js` - Basic HTTP requests

## Implementation Details

### Request ID Generation
```typescript
// Format: ${clientId}#${url}#${timestamp}-${random}
const requestId = `${this.clientId}#${url}#${Date.now()}-${Math.floor(1000 * Math.random())}`;
```

This ensures:
- **Global Uniqueness**: No collisions between clients or requests
- **Client Association**: Easy routing of responses back to the correct client
- **Temporal Ordering**: Timestamp helps with debugging and logging

### Memory Management
```typescript
// Automatic cleanup when clients disconnect
removeClient(clientId: string): void {
  this.clients.delete(clientId);
  
  // Last client triggers shared instance cleanup
  if (this.clients.size === 0) {
    InstanceManager.getInstance().removeSharedInstance(this.port);
  }
}
```

### Error Handling
- Connection failures are isolated to individual clients
- Shared instance remains stable even if one client encounters errors
- Graceful degradation when Go server becomes unavailable

## Migration Guide

### For Existing Code
**No changes required!** The new architecture is fully backward compatible:

```typescript
// This code works exactly the same as before
const cycleTLS = await initCycleTLS({ port: 9119 });
const response = await cycleTLS.get("https://example.com");
await cycleTLS.exit();
```

### For New Multi-Instance Code
```typescript
// Now this works perfectly:
const instance1 = await initCycleTLS({ port: 9119 });
const instance2 = await initCycleTLS({ port: 9119 }); // Shares the same Go server

// Both can make requests independently
const [response1, response2] = await Promise.all([
  instance1.get("https://httpbin.org/get"),
  instance2.get("https://httpbin.org/json")
]);

// Cleanup is coordinated automatically
await instance1.exit();
await instance2.exit(); // Go server shuts down after last client
```

## Future Considerations

### Potential Enhancements
1. **Connection Pooling**: Could add intelligent connection pooling within the shared Go server
2. **Load Balancing**: Could distribute requests across multiple Go processes for high throughput scenarios  
3. **Health Monitoring**: Could add health checks for the shared Go server instance
4. **Metrics**: Could add per-client metrics and monitoring

### Performance Impact
- **Positive**: Reduced memory usage (fewer Go processes)
- **Positive**: Better connection reuse within shared server
- **Minimal**: Small overhead for message routing (~microseconds)
- **Positive**: More efficient resource utilization

This architecture successfully solves the multiple connection problem while maintaining full backward compatibility and improving resource efficiency.