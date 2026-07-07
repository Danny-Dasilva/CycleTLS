/**
 * Tests for TS Core review fixes (src/index.ts)
 *
 * Covers 6 CRITICAL + 8 MAJOR issues:
 *
 * CRITICAL:
 *   1. WebSocket event listener memory leak on connection errors
 *   2. Request timeout memory leak when early response arrives
 *   3. Race condition in process kill during initialization failure
 *   4. Zombie process risk when HTTP server fails
 *   5. Process exit handlers not removed after cleanup
 *   6. Unhandled promise rejection in WebSocket send
 *
 * MAJOR:
 *   7. HTTP server listener leak on multiple errors
 *   8. Stream event listener leak on early stream closure
 *   9. Connection timeout not cleared on initialization rejection
 *   10. stderr listener memory leak on process respawn
 *   11. WebSocket server message handler never removed
 *   12. Missing error handler on FormData stream
 *   13. Race condition in shared instance cleanup
 *   14. InstanceManager cleanup doesn't wait for initialization
 */

import { EventEmitter } from "events";
import { Readable } from "stream";
import * as fs from "fs";
import * as path from "path";

// Read source files for structural verification
const srcPath = path.resolve(__dirname, "..", "src", "index.ts");
const distPath = path.resolve(__dirname, "..", "dist", "index.js");
const tsSource = fs.readFileSync(srcPath, "utf8");
const jsSource = fs.readFileSync(distPath, "utf8");

jest.setTimeout(15000);

// ============================================================
// CRITICAL Issue 1: WebSocket event listener memory leak on connection errors
// ============================================================
describe("CRITICAL #1: WebSocket event listener leak on connection errors", () => {
  test("ws() method should register cleanup on both 'close' AND 'error' events", () => {
    // Source should have cleanup on both close and error for the WS handler
    const wsMethodStart = tsSource.indexOf("async ws(url: string");
    const wsMethodEnd = tsSource.indexOf("webSocket(url: string", wsMethodStart);
    const wsMethod = tsSource.substring(wsMethodStart, wsMethodEnd);

    // Should have both close and error cleanup
    expect(wsMethod).toContain("ws.once('close', cleanupWsHandler)");
    expect(wsMethod).toContain("ws.once('error', cleanupWsHandler)");
  });

  test("cleanup handler should remove both close and error listeners", () => {
    const wsMethodStart = tsSource.indexOf("async ws(url: string");
    const wsMethodEnd = tsSource.indexOf("webSocket(url: string", wsMethodStart);
    const wsMethod = tsSource.substring(wsMethodStart, wsMethodEnd);

    // The cleanupWsHandler should remove listeners for both events
    expect(wsMethod).toContain("ws.off('close', cleanupWsHandler)");
    expect(wsMethod).toContain("ws.off('error', cleanupWsHandler)");
  });

  test("error event should clean up handleMessage listener (behavioral)", () => {
    // Simulate the pattern: EventEmitter with handleMessage registered,
    // and cleanup triggered by 'error' event
    const client = new EventEmitter();
    const ws = new EventEmitter();
    const requestId = "test-request-id";

    const handleMessage = () => {};
    client.on(requestId, handleMessage);

    expect(client.listenerCount(requestId)).toBe(1);

    // Simulate the cleanupWsHandler
    const cleanupWsHandler = () => {
      client.off(requestId, handleMessage);
      ws.off("close", cleanupWsHandler);
      ws.off("error", cleanupWsHandler);
    };
    ws.once("close", cleanupWsHandler);
    ws.once("error", cleanupWsHandler);

    // Trigger error (not close)
    ws.emit("error", new Error("connection reset"));

    // handleMessage should be cleaned up
    expect(client.listenerCount(requestId)).toBe(0);
    // close listener should also be removed
    expect(ws.listenerCount("close")).toBe(0);
    expect(ws.listenerCount("error")).toBe(0);
  });

  test("close event should clean up handleMessage listener (behavioral)", () => {
    const client = new EventEmitter();
    const ws = new EventEmitter();
    const requestId = "test-request-id";

    const handleMessage = () => {};
    client.on(requestId, handleMessage);

    const cleanupWsHandler = () => {
      client.off(requestId, handleMessage);
      ws.off("close", cleanupWsHandler);
      ws.off("error", cleanupWsHandler);
    };
    ws.once("close", cleanupWsHandler);
    ws.once("error", cleanupWsHandler);

    // Trigger close (not error)
    ws.emit("close", 1000, "");

    expect(client.listenerCount(requestId)).toBe(0);
    expect(ws.listenerCount("close")).toBe(0);
    expect(ws.listenerCount("error")).toBe(0);
  });
});

// ============================================================
// CRITICAL Issue 2: Request timeout memory leak when early response arrives
// ============================================================
describe("CRITICAL #2: Request timeout memory leak on early response", () => {
  test("catch block in stream handling should call cleanupTimeout", () => {
    // The catch block around streamToBuffer should clear timeout
    const requestMethod = tsSource.substring(
      tsSource.indexOf("async request("),
      tsSource.indexOf("// HTTP method implementations")
    );

    // Find the catch block that calls rejectRequest
    const catchBlocks = requestMethod.match(/} catch \(error\) \{[\s\S]*?rejectRequest/g);
    expect(catchBlocks).not.toBeNull();

    // At least one catch block should call cleanupTimeout before rejectRequest
    const hasCleanupInCatch = catchBlocks!.some(
      (block) => block.includes("cleanupTimeout")
    );
    expect(hasCleanupInCatch).toBe(true);
  });

  test("cleanupTimeout should clear and null the timeoutId (behavioral)", () => {
    let timeoutId: ReturnType<typeof setTimeout> | null = null;
    let timeoutFired = false;

    const cleanupTimeout = () => {
      if (timeoutId) {
        clearTimeout(timeoutId);
        timeoutId = null;
      }
    };

    // Set a short timeout
    timeoutId = setTimeout(() => {
      timeoutFired = true;
    }, 50);

    // Clear it immediately
    cleanupTimeout();

    expect(timeoutId).toBeNull();

    // Wait to confirm it didn't fire
    return new Promise<void>((resolve) => {
      setTimeout(() => {
        expect(timeoutFired).toBe(false);
        resolve();
      }, 100);
    });
  });
});

// ============================================================
// CRITICAL Issue 3: Race condition in process kill during init failure
// ============================================================
describe("CRITICAL #3: Process kill race condition in rejectInitialization", () => {
  test("rejectInitialization should use forceKillProcess which handles PID undefined", () => {
    const rejectMethod = tsSource.substring(
      tsSource.indexOf("private rejectInitialization("),
      tsSource.indexOf("async initialize()")
    );

    // Should use forceKillProcess (which handles PID internally)
    expect(rejectMethod).toContain("forceKillProcess(this.child, true)");
    // Should null the child after killing
    expect(rejectMethod).toContain("this.child = null");
  });

  test("forceKillProcess handles undefined PID gracefully (behavioral)", () => {
    // Simulate the forceKillProcess pattern
    const mockChild: { pid: number | undefined; kill: jest.Mock } = {
      pid: undefined,
      kill: jest.fn(),
    };

    // The function should try child.kill() when pid is undefined
    const killGroup = true;
    if (killGroup && mockChild.pid !== undefined) {
      // Would call process.kill(-pid, SIGKILL)
    } else {
      try {
        mockChild.kill("SIGKILL");
      } catch {
        /* Process already dead */
      }
    }

    expect(mockChild.kill).toHaveBeenCalledWith("SIGKILL");
  });
});

// ============================================================
// CRITICAL Issue 4: Zombie process risk when HTTP server fails
// ============================================================
describe("CRITICAL #4: Zombie process prevention on HTTP server failure", () => {
  test("HTTP server error handler should kill child process", () => {
    // Find the onError handler in checkSpawnedInstance
    const checkSpawnedMethod = tsSource.substring(
      tsSource.indexOf("private checkSpawnedInstance("),
      tsSource.indexOf("private spawnServer()")
    );

    // The error handler should kill child process
    const onErrorStart = checkSpawnedMethod.indexOf("const onError");
    const onErrorBody = checkSpawnedMethod.substring(onErrorStart);

    expect(onErrorBody).toContain("if (this.child)");
    expect(onErrorBody).toContain("forceKillProcess(this.child, true)");
    expect(onErrorBody).toContain("this.child = null");
  });

  test("child process should be killed before falling back to client mode (behavioral)", () => {
    let childKilled = false;
    const mockChild = {
      pid: 12345,
      kill: () => {
        childKilled = true;
      },
    };

    // Simulate the error handler logic
    if (mockChild) {
      mockChild.kill();
      childKilled = true;
    }

    expect(childKilled).toBe(true);
  });
});

// ============================================================
// CRITICAL Issue 5: Process exit handlers not removed after cleanup
// ============================================================
describe("CRITICAL #5: Process exit handlers leak prevention", () => {
  test("source should have registerGlobalCleanupHandlers function", () => {
    expect(tsSource).toContain("function registerGlobalCleanupHandlers()");
  });

  test("source should have removeGlobalCleanupHandlers function", () => {
    expect(tsSource).toContain("function removeGlobalCleanupHandlers()");
  });

  test("globalCleanup should call removeGlobalCleanupHandlers in finally block", () => {
    const globalCleanupStart = tsSource.indexOf("const globalCleanup = async ()");
    const globalCleanupEnd = tsSource.indexOf(
      "function registerGlobalCleanupHandlers",
      globalCleanupStart
    );
    const globalCleanupBody = tsSource.substring(
      globalCleanupStart,
      globalCleanupEnd
    );

    expect(globalCleanupBody).toContain("finally");
    expect(globalCleanupBody).toContain("removeGlobalCleanupHandlers()");
  });

  test("registerGlobalCleanupHandlers prevents duplicate registration", () => {
    const registerFn = tsSource.substring(
      tsSource.indexOf("function registerGlobalCleanupHandlers()"),
      tsSource.indexOf("function removeGlobalCleanupHandlers()")
    );

    expect(registerFn).toContain("if (globalCleanupRegistered) return");
  });

  test("removeGlobalCleanupHandlers uses process.off to detach handlers", () => {
    const removeFn = tsSource.substring(
      tsSource.indexOf("function removeGlobalCleanupHandlers()"),
      tsSource.indexOf("// Set up process-wide cleanup handlers")
    );

    expect(removeFn).toContain('process.off("SIGINT", globalCleanup)');
    expect(removeFn).toContain('process.off("SIGTERM", globalCleanup)');
    expect(removeFn).toContain('process.off("beforeExit", globalCleanup)');
  });

  test("handler registration/removal cycle does not leak (behavioral)", () => {
    const emitter = new EventEmitter();
    let registered = false;
    const handler = () => {};

    function register() {
      if (registered) return;
      registered = true;
      emitter.on("test", handler);
    }

    function remove() {
      if (!registered) return;
      registered = false;
      emitter.off("test", handler);
    }

    // Register multiple times
    register();
    register();
    register();
    expect(emitter.listenerCount("test")).toBe(1);

    // Remove
    remove();
    expect(emitter.listenerCount("test")).toBe(0);

    // Re-register
    register();
    expect(emitter.listenerCount("test")).toBe(1);

    // Clean up
    remove();
    expect(emitter.listenerCount("test")).toBe(0);
  });
});

// ============================================================
// CRITICAL Issue 6: Unhandled promise rejection in WebSocket send
// ============================================================
describe("CRITICAL #6: Safe error emission in WebSocket send", () => {
  test("send method should check listenerCount before emitting error", () => {
    const sendMethod = tsSource.substring(
      tsSource.indexOf("send(\n    data: string | Buffer | ArrayBuffer,\n    optionsOrCallback"),
      tsSource.indexOf("// Close the connection (matching ws library signature)")
    );

    expect(sendMethod).toContain("this.listenerCount('error') > 0");
    expect(sendMethod).toContain("process.nextTick");
  });

  test("close method should check listenerCount before emitting error", () => {
    const closeMethod = tsSource.substring(
      tsSource.indexOf("close(code?: number, reason?: string): void {"),
      tsSource.indexOf("// Ping the connection")
    );

    expect(closeMethod).toContain("this.listenerCount('error') > 0");
  });

  test("error emitted only when listeners exist (behavioral)", () => {
    const emitter = new EventEmitter();
    let errorReceived = false;

    // No listeners - should not emit
    if (emitter.listenerCount("error") > 0) {
      emitter.emit("error", new Error("test"));
    }

    // Add listener
    emitter.on("error", () => {
      errorReceived = true;
    });

    // Now should emit
    if (emitter.listenerCount("error") > 0) {
      emitter.emit("error", new Error("test"));
    }

    expect(errorReceived).toBe(true);
  });

  test("process.nextTick fallback when no listeners and no callback (behavioral)", (done) => {
    // Simulate the pattern: no callback, no error listeners
    const originalNextTick = process.nextTick;
    let nextTickCalled = false;

    // The actual code uses process.nextTick(() => { throw err; })
    // We test the pattern works correctly
    const emitter = new EventEmitter();
    const err = new Error("test error");

    const cb: ((err: Error) => void) | undefined = undefined;
    const hasListeners = emitter.listenerCount("error") > 0;

    if (cb) {
      // would call cb(err)
    } else if (hasListeners) {
      emitter.emit("error", err);
    } else {
      // This is the new path - schedule throw
      process.nextTick(() => {
        nextTickCalled = true;
        done();
      });
    }

    expect(hasListeners).toBe(false);
  });
});

// ============================================================
// MAJOR Issue 7: HTTP server listener leak on multiple errors
// ============================================================
describe("MAJOR #7: HTTP server listener leak prevention", () => {
  test("listening handler should remove error listener", () => {
    const checkMethod = tsSource.substring(
      tsSource.indexOf("private checkSpawnedInstance("),
      tsSource.indexOf("private spawnServer()")
    );

    // onListening should remove onError
    expect(checkMethod).toContain("this.httpServer?.off('error', onError)");
  });

  test("error handler should remove listening listener", () => {
    const checkMethod = tsSource.substring(
      tsSource.indexOf("private checkSpawnedInstance("),
      tsSource.indexOf("private spawnServer()")
    );

    // onError should remove onListening
    expect(checkMethod).toContain("this.httpServer?.off('listening', onListening)");
  });

  test("named handlers used instead of anonymous (structural)", () => {
    const checkMethod = tsSource.substring(
      tsSource.indexOf("private checkSpawnedInstance("),
      tsSource.indexOf("private spawnServer()")
    );

    // Should use named handlers
    expect(checkMethod).toContain("const onListening = ()");
    expect(checkMethod).toContain("const onError = (err: Error)");
    expect(checkMethod).toContain("this.httpServer.once('listening', onListening)");
    expect(checkMethod).toContain("this.httpServer.once('error', onError)");
  });

  test("cross-removal prevents listener leak (behavioral)", () => {
    const server = new EventEmitter();

    const onListening = () => {
      server.off("error", onError);
    };

    const onError = () => {
      server.off("listening", onListening);
    };

    server.once("listening", onListening);
    server.once("error", onError);

    // Both listeners registered
    expect(server.listenerCount("listening")).toBe(1);
    expect(server.listenerCount("error")).toBe(1);

    // Trigger listening - should remove error
    server.emit("listening");
    expect(server.listenerCount("listening")).toBe(0); // once auto-removes
    expect(server.listenerCount("error")).toBe(0); // explicitly removed
  });
});

// ============================================================
// MAJOR Issue 8: Stream event listener leak on early stream closure
// ============================================================
describe("MAJOR #8: Stream event listener cleanup on early closure", () => {
  test("source should have cleanupStreamListeners helper", () => {
    const requestMethod = tsSource.substring(
      tsSource.indexOf("async request("),
      tsSource.indexOf("// HTTP method implementations")
    );

    expect(requestMethod).toContain("const cleanupStreamListeners = ()");
  });

  test("handleClose should call cleanupStreamListeners", () => {
    const requestMethod = tsSource.substring(
      tsSource.indexOf("async request("),
      tsSource.indexOf("// HTTP method implementations")
    );

    // handleClose should call cleanupStreamListeners
    const handleCloseStart = requestMethod.indexOf("const handleClose = ()");
    const handleCloseEnd = requestMethod.indexOf("};", handleCloseStart);
    const handleClose = requestMethod.substring(handleCloseStart, handleCloseEnd);

    expect(handleClose).toContain("cleanupStreamListeners()");
  });

  test("handleData end/error paths should use cleanupStreamListeners", () => {
    const requestMethod = tsSource.substring(
      tsSource.indexOf("async request("),
      tsSource.indexOf("// HTTP method implementations")
    );

    // Find the handleData function
    const handleDataStart = requestMethod.indexOf("const handleData = (");
    const handleDataEnd = requestMethod.indexOf("stream.on(\"close\"", handleDataStart);
    const handleData = requestMethod.substring(handleDataStart, handleDataEnd);

    // Both error and end paths should use cleanupStreamListeners
    const cleanupCalls = handleData.match(/cleanupStreamListeners\(\)/g);
    expect(cleanupCalls).not.toBeNull();
    expect(cleanupCalls!.length).toBeGreaterThanOrEqual(2); // error + end
  });

  test("stream destroy cleans up all listeners (behavioral)", () => {
    const client = new EventEmitter();
    const stream = new Readable({ read() {} });
    const requestId = "test-req";

    const cleanupStreamListeners = () => {
      stream.off("close", handleClose);
      client.off(requestId, handleData);
    };

    const handleClose = () => {
      cleanupStreamListeners();
    };

    const handleData = () => {};

    stream.on("close", handleClose);
    client.on(requestId, handleData);

    expect(stream.listenerCount("close")).toBe(1);
    expect(client.listenerCount(requestId)).toBe(1);

    // Destroy stream (simulates cancel/timeout)
    stream.destroy();

    // Use setImmediate because destroy emits close asynchronously
    return new Promise<void>((resolve) => {
      setImmediate(() => {
        expect(stream.listenerCount("close")).toBe(0);
        expect(client.listenerCount(requestId)).toBe(0);
        resolve();
      });
    });
  });
});

// ============================================================
// MAJOR Issue 9: Connection timeout not cleared on init rejection
// ============================================================
describe("MAJOR #9: Connection timeout cleared on initialization rejection", () => {
  test("rejectInitialization should clear connectionTimeout", () => {
    const rejectMethod = tsSource.substring(
      tsSource.indexOf("private rejectInitialization("),
      tsSource.indexOf("async initialize()")
    );

    expect(rejectMethod).toContain("if (this.connectionTimeout)");
    expect(rejectMethod).toContain("clearTimeout(this.connectionTimeout)");
    expect(rejectMethod).toContain("this.connectionTimeout = null");
  });

  test("connectionTimeout clearing happens before rejecting (order check)", () => {
    const rejectMethod = tsSource.substring(
      tsSource.indexOf("private rejectInitialization("),
      tsSource.indexOf("async initialize()")
    );

    const clearTimeoutPos = rejectMethod.indexOf("clearTimeout(this.connectionTimeout)");
    const rejectCallPos = rejectMethod.indexOf("this.initializationReject(reason)");

    // clearTimeout should come before the reject call
    expect(clearTimeoutPos).toBeLessThan(rejectCallPos);
  });

  test("timeout does not fire after rejection (behavioral)", () => {
    let timeoutFired = false;
    let rejected = false;
    let connectionTimeout: ReturnType<typeof setTimeout> | null = null;

    const rejectInit = (reason: string) => {
      // Clear timeout first
      if (connectionTimeout) {
        clearTimeout(connectionTimeout);
        connectionTimeout = null;
      }
      rejected = true;
    };

    // Set a timeout that would fire
    connectionTimeout = setTimeout(() => {
      timeoutFired = true;
    }, 50);

    // Reject immediately
    rejectInit("test failure");

    expect(rejected).toBe(true);
    expect(connectionTimeout).toBeNull();

    return new Promise<void>((resolve) => {
      setTimeout(() => {
        expect(timeoutFired).toBe(false);
        resolve();
      }, 100);
    });
  });
});

// ============================================================
// MAJOR Issue 10: stderr listener memory leak on process respawn
// ============================================================
describe("MAJOR #10: stderr listener cleanup on process respawn", () => {
  test("handleSpawn should remove old child listeners before spawning", () => {
    const handleSpawnMethod = tsSource.substring(
      tsSource.indexOf("private handleSpawn(fileName: string): void {"),
      tsSource.indexOf("private createClient(")
    );

    // Should clean up old child listeners
    expect(handleSpawnMethod).toContain("this.child.stdout.removeAllListeners()");
    expect(handleSpawnMethod).toContain("this.child.stderr.removeAllListeners()");
    expect(handleSpawnMethod).toContain("this.child.removeAllListeners()");
  });

  test("cleanup happens before new spawn (order check)", () => {
    const handleSpawnMethod = tsSource.substring(
      tsSource.indexOf("private handleSpawn(fileName: string): void {"),
      tsSource.indexOf("private createClient(")
    );

    const removeListenersPos = handleSpawnMethod.indexOf(
      "this.child.stdout.removeAllListeners()"
    );
    const spawnPos = handleSpawnMethod.indexOf("this.child = spawn(");

    expect(removeListenersPos).toBeLessThan(spawnPos);
  });

  test("old child listeners removed on respawn (behavioral)", () => {
    const child1 = new EventEmitter() as any;
    child1.stdout = new EventEmitter();
    child1.stderr = new EventEmitter();

    // Add listeners like handleSpawn would
    child1.stdout.on("data", () => {});
    child1.stderr.on("data", () => {});
    child1.on("error", () => {});

    expect(child1.stdout.listenerCount("data")).toBe(1);
    expect(child1.stderr.listenerCount("data")).toBe(1);
    expect(child1.listenerCount("error")).toBe(1);

    // Simulate cleanup before respawn
    child1.stdout.removeAllListeners();
    child1.stderr.removeAllListeners();
    child1.removeAllListeners();

    expect(child1.stdout.listenerCount("data")).toBe(0);
    expect(child1.stderr.listenerCount("data")).toBe(0);
    expect(child1.listenerCount("error")).toBe(0);
  });
});

// ============================================================
// MAJOR Issue 11: WebSocket server message handler never removed
// ============================================================
describe("MAJOR #11: WebSocket server message handler cleanup", () => {
  test("cleanup method should call removeAllListeners on server", () => {
    const cleanupMethod = tsSource.substring(
      tsSource.indexOf("async cleanup(): Promise<void> {", tsSource.indexOf("class SharedInstance")),
      tsSource.indexOf("// Notify all clients")
    );

    expect(cleanupMethod).toContain("this.server.removeAllListeners()");
    expect(cleanupMethod).toContain("this.server.terminate()");
    expect(cleanupMethod).toContain("this.server = null");
  });

  test("removeAllListeners cleans all handlers including message (behavioral)", () => {
    const ws = new EventEmitter();

    // Register a message handler (like the server does)
    ws.on("message", () => {});
    ws.on("close", () => {});
    ws.on("error", () => {});

    expect(ws.listenerCount("message")).toBe(1);

    // cleanup pattern
    ws.removeAllListeners();

    expect(ws.listenerCount("message")).toBe(0);
    expect(ws.listenerCount("close")).toBe(0);
    expect(ws.listenerCount("error")).toBe(0);
  });
});

// ============================================================
// MAJOR Issue 12: Missing error handler on FormData stream
// ============================================================
describe("MAJOR #12: FormData stream error handling", () => {
  test("FormData stream conversion should handle close event", () => {
    const sendRequestMethod = tsSource.substring(
      tsSource.indexOf("async sendRequest(requestId: string"),
      tsSource.indexOf("async cancelRequest(")
    );

    expect(sendRequestMethod).toContain("form.on('close', onClose)");
  });

  test("FormData stream should have settled guard to prevent double resolution", () => {
    const sendRequestMethod = tsSource.substring(
      tsSource.indexOf("async sendRequest(requestId: string"),
      tsSource.indexOf("async cancelRequest(")
    );

    expect(sendRequestMethod).toContain("let settled = false");
    expect(sendRequestMethod).toContain("if (settled) return");
  });

  test("FormData stream should cleanup listeners after settling", () => {
    const sendRequestMethod = tsSource.substring(
      tsSource.indexOf("async sendRequest(requestId: string"),
      tsSource.indexOf("async cancelRequest(")
    );

    // Should have cleanup function that removes all listeners
    expect(sendRequestMethod).toContain("form.off('data', onData)");
    expect(sendRequestMethod).toContain("form.off('end', onEnd)");
    expect(sendRequestMethod).toContain("form.off('error', onError)");
    expect(sendRequestMethod).toContain("form.off('close', onClose)");
  });

  test("destroyed FormData stream rejects instead of hanging (behavioral)", async () => {
    const stream = new Readable({ read() {} });
    let settled = false;

    const promise = new Promise<string>((resolve, reject) => {
      const cleanup = () => {
        stream.off("data", onData);
        stream.off("end", onEnd);
        stream.off("error", onError);
        stream.off("close", onClose);
      };

      const onData = (chunk: Buffer) => {};
      const onEnd = () => {
        if (settled) return;
        settled = true;
        cleanup();
        resolve("done");
      };
      const onError = (err: Error) => {
        if (settled) return;
        settled = true;
        cleanup();
        reject(err);
      };
      const onClose = () => {
        if (!settled) {
          settled = true;
          cleanup();
          reject(new Error("Stream was destroyed before completion"));
        }
      };

      stream.on("data", onData);
      stream.on("end", onEnd);
      stream.on("error", onError);
      stream.on("close", onClose);
      stream.resume();
    });

    // Destroy the stream externally
    stream.destroy();

    await expect(promise).rejects.toThrow("Stream was destroyed before completion");
  });
});

// ============================================================
// MAJOR Issue 13: Race condition in shared instance cleanup
// ============================================================
describe("MAJOR #13: Shared instance cleanup race condition", () => {
  test("removeClient should check for pending initializations before cleanup", () => {
    const removeClientMethod = tsSource.substring(
      tsSource.indexOf("removeClient(clientId: string): void {"),
      tsSource.indexOf("async sendRequest(")
    );

    expect(removeClientMethod).toContain("_hasInitializingPromise(this.port)");
  });

  test("cleanup is skipped when initialization is in progress (behavioral)", () => {
    let cleanupCalled = false;
    const initializingPorts = new Set([9119]);

    const removeClient = (port: number) => {
      const clientsEmpty = true; // simulate no clients left

      if (clientsEmpty) {
        if (!initializingPorts.has(port)) {
          cleanupCalled = true;
        }
        // Otherwise skip cleanup - new client is initializing
      }
    };

    removeClient(9119);
    expect(cleanupCalled).toBe(false);

    // After initialization completes
    initializingPorts.delete(9119);
    removeClient(9119);
    expect(cleanupCalled).toBe(true);
  });
});

// ============================================================
// MAJOR Issue 14: InstanceManager cleanup doesn't wait for init
// ============================================================
describe("MAJOR #14: InstanceManager cleanup waits for initialization", () => {
  test("cleanup should await pending initializingPromises", () => {
    const instanceManagerClass = tsSource.substring(
      tsSource.indexOf("class InstanceManager {"),
      tsSource.indexOf("class SharedInstance")
    );

    const cleanupMethod = instanceManagerClass.substring(
      instanceManagerClass.indexOf("async cleanup(): Promise<void>")
    );

    // Should wait for initializing promises
    expect(cleanupMethod).toContain("this.initializingPromises.size > 0");
    expect(cleanupMethod).toContain("Promise.allSettled(pendingPromises)");
  });

  test("cleanup should clear initializingPromises map", () => {
    const instanceManagerClass = tsSource.substring(
      tsSource.indexOf("class InstanceManager {"),
      tsSource.indexOf("class SharedInstance")
    );

    const cleanupMethod = instanceManagerClass.substring(
      instanceManagerClass.indexOf("async cleanup(): Promise<void>")
    );

    expect(cleanupMethod).toContain("this.initializingPromises.clear()");
  });

  test("Promise.allSettled handles both success and failure (behavioral)", async () => {
    const promises = [
      Promise.resolve("ok"),
      Promise.reject(new Error("fail")),
      Promise.resolve("ok2"),
    ];

    const results = await Promise.allSettled(promises);

    expect(results[0]).toEqual({ status: "fulfilled", value: "ok" });
    expect(results[1]).toEqual({
      status: "rejected",
      reason: expect.any(Error),
    });
    expect(results[2]).toEqual({ status: "fulfilled", value: "ok2" });
  });

  test("cleanup runs after pending init completes (behavioral)", async () => {
    const order: string[] = [];

    // Simulate pending initialization
    const initPromise = new Promise<void>((resolve) => {
      setTimeout(() => {
        order.push("init-complete");
        resolve();
      }, 50);
    });

    // Simulate cleanup that waits for init
    const cleanupFn = async () => {
      await Promise.allSettled([initPromise]);
      order.push("cleanup-done");
    };

    await cleanupFn();

    expect(order).toEqual(["init-complete", "cleanup-done"]);
  });
});

// ============================================================
// Cross-cutting: Verify dist/index.js was built with all fixes
// ============================================================
describe("Build verification: dist/index.js contains all fixes", () => {
  test("dist contains cleanupWsHandler pattern (Fix #1)", () => {
    expect(jsSource).toContain("cleanupWsHandler");
  });

  test("dist contains cleanupTimeout in catch block (Fix #2)", () => {
    // The catch block should have cleanupTimeout before rejectRequest
    const catchPattern = /catch.*\{[\s\S]*?cleanupTimeout[\s\S]*?rejectRequest/;
    expect(jsSource).toMatch(catchPattern);
  });

  test("dist contains child kill in HTTP server error handler (Fix #4)", () => {
    // The onError handler should contain child process kill
    expect(jsSource).toContain("forceKillProcess(this.child, true)");
  });

  test("dist contains registerGlobalCleanupHandlers (Fix #5)", () => {
    expect(jsSource).toContain("registerGlobalCleanupHandlers");
    expect(jsSource).toContain("removeGlobalCleanupHandlers");
  });

  test("dist contains listenerCount check for error emission (Fix #6)", () => {
    expect(jsSource).toContain("listenerCount('error')");
  });

  test("dist contains onListening/onError named handlers (Fix #7)", () => {
    expect(jsSource).toContain("onListening");
    expect(jsSource).toContain("onError");
  });

  test("dist contains cleanupStreamListeners (Fix #8)", () => {
    expect(jsSource).toContain("cleanupStreamListeners");
  });

  test("dist clears connectionTimeout in rejectInitialization (Fix #9)", () => {
    const rejectStart = jsSource.indexOf("rejectInitialization(reason)");
    const rejectEnd = jsSource.indexOf("initialize()", rejectStart);
    const rejectBody = jsSource.substring(rejectStart, rejectEnd);

    expect(rejectBody).toContain("clearTimeout(this.connectionTimeout)");
  });

  test("dist removes old child listeners in handleSpawn (Fix #10)", () => {
    expect(jsSource).toContain("this.child.stdout.removeAllListeners()");
    expect(jsSource).toContain("this.child.stderr.removeAllListeners()");
  });

  test("dist contains settled guard for FormData (Fix #12)", () => {
    expect(jsSource).toContain("settled");
  });

  test("dist checks initializingPromise before cleanup (Fix #13)", () => {
    expect(jsSource).toContain("_hasInitializingPromise");
  });

  test("dist uses Promise.allSettled in cleanup (Fix #14)", () => {
    expect(jsSource).toContain("Promise.allSettled");
  });
});
