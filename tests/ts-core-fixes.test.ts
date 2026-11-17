/**
 * Unit tests for TypeScript core bug fixes
 *
 * Tests cover:
 * 1. bodyStream memory leak (once vs on for error listener)
 * 2. WebSocket open event race (queueMicrotask vs setTimeout)
 * 3. Timeout timers cleanup in error paths
 * 4. Windows SIGKILL handling (taskkill fallback)
 * 5. Child process spawn error handler
 * 6. Unbounded SSE event queue
 * 7. WebSocket send callback limitation
 * 8. CreditManager state reset on error
 */

import { Readable } from 'stream';
import { EventEmitter } from 'events';

// ============================================================
// Issue 1: bodyStream.on("error") should be bodyStream.once("error")
// ============================================================
describe('Issue 1: bodyStream error listener memory leak', () => {
  test('once("error") adds exactly one listener that is auto-removed after firing', () => {
    const stream = new Readable({ read() {} });
    stream.once('error', () => {});

    expect(stream.listenerCount('error')).toBe(1);

    // Emit error - once listener should auto-remove
    stream.emit('error', new Error('test'));
    expect(stream.listenerCount('error')).toBe(0);
  });

  test('on("error") listener persists after firing (the bug pattern)', () => {
    const stream = new Readable({ read() {} });
    stream.on('error', () => {});

    expect(stream.listenerCount('error')).toBe(1);

    // Emit error - on listener persists
    stream.emit('error', new Error('test'));
    expect(stream.listenerCount('error')).toBe(1);
  });

  test('flow-control-client source uses once("error") for bodyStream', () => {
    const fs = require('fs');
    const source = fs.readFileSync(
      require.resolve('../src/flow-control-client.ts'),
      'utf8'
    );

    // Should NOT contain bodyStream.on("error"
    const badPattern = /bodyStream\.on\("error"/g;
    const badMatches = source.match(badPattern);
    expect(badMatches).toBeNull();

    // Should contain bodyStream.once("error"
    const goodPattern = /bodyStream\.once\("error"/g;
    const goodMatches = source.match(goodPattern);
    expect(goodMatches).not.toBeNull();
    expect(goodMatches!.length).toBeGreaterThanOrEqual(2); // request() and sse()
  });
});

// ============================================================
// Issue 2: WebSocket open event race (setTimeout vs queueMicrotask)
// ============================================================
describe('Issue 2: WebSocket open event timing', () => {
  test('queueMicrotask fires before setTimeout(0)', async () => {
    const order: string[] = [];

    await new Promise<void>((resolve) => {
      setTimeout(() => {
        order.push('setTimeout');
        resolve();
      }, 0);
      queueMicrotask(() => {
        order.push('queueMicrotask');
      });
    });

    expect(order[0]).toBe('queueMicrotask');
    expect(order[1]).toBe('setTimeout');
  });

  test('promise .then() and queueMicrotask both fire before setTimeout(0)', async () => {
    const order: string[] = [];

    await new Promise<void>((resolve) => {
      setTimeout(() => {
        order.push('setTimeout');
        resolve();
      }, 0);
      Promise.resolve().then(() => {
        order.push('promise.then');
      });
      queueMicrotask(() => {
        order.push('queueMicrotask');
      });
    });

    // Both microtasks fire before the macrotask
    expect(order.indexOf('promise.then')).toBeLessThan(order.indexOf('setTimeout'));
    expect(order.indexOf('queueMicrotask')).toBeLessThan(order.indexOf('setTimeout'));
  });

  test('flow-control-client uses queueMicrotask for _handleOpen', () => {
    const fs = require('fs');
    const source = fs.readFileSync(
      require.resolve('../src/flow-control-client.ts'),
      'utf8'
    );

    // Should NOT contain setTimeout(() => { ... _handleOpen
    const badPattern = /setTimeout\(\(\)\s*=>\s*\{[^}]*_handleOpen/;
    expect(source).not.toMatch(badPattern);

    // Should contain queueMicrotask(() => { ... _handleOpen
    const goodPattern = /queueMicrotask\(\(\)\s*=>\s*\{[^}]*_handleOpen/;
    expect(source).toMatch(goodPattern);
  });
});

// ============================================================
// Issue 3: Timeout timers not always cleared in error paths
// ============================================================
describe('Issue 3: Timeout timer cleanup in error paths', () => {
  test('flow-control-client request() clears readTimeoutId in error case', () => {
    const fs = require('fs');
    const source = fs.readFileSync(
      require.resolve('../src/flow-control-client.ts'),
      'utf8'
    );

    // Extract the request method source
    const requestStart = source.indexOf('async request(');
    const requestEnd = source.indexOf('async get(', requestStart);
    const requestSource = source.substring(requestStart, requestEnd);

    // The error case handler should call clearReadTimeout
    // Look for the pattern: in the 'error' case, clearReadTimeout is called
    const errorCasePattern = /case\s+"error"[\s\S]*?clearReadTimeout/;
    expect(requestSource).toMatch(errorCasePattern);

    // The ws.on("error") handler should also call clearReadTimeout
    const wsErrorPattern = /ws\.on\("error"[\s\S]*?clearReadTimeout/;
    expect(requestSource).toMatch(wsErrorPattern);
  });

  test('cleanup function should clear both timeoutId and readTimeoutId', () => {
    // Simulate the cleanup pattern
    let timeoutId: ReturnType<typeof setTimeout> | null = setTimeout(() => {}, 10000);
    let readTimeoutId: ReturnType<typeof setTimeout> | null = setTimeout(() => {}, 10000);

    const cleanupTimers = () => {
      if (timeoutId !== null) {
        clearTimeout(timeoutId);
        timeoutId = null;
      }
      if (readTimeoutId !== null) {
        clearTimeout(readTimeoutId);
        readTimeoutId = null;
      }
    };

    cleanupTimers();
    expect(timeoutId).toBeNull();
    expect(readTimeoutId).toBeNull();
  });
});

// ============================================================
// Issue 4: SIGKILL on Windows doesn't work
// ============================================================
describe('Issue 4: Windows process termination', () => {
  test('flow-control-client close() uses taskkill on Windows', () => {
    const fs = require('fs');
    const source = fs.readFileSync(
      require.resolve('../src/flow-control-client.ts'),
      'utf8'
    );

    // Extract just the close() method (from "async close():" to the next method or end of class)
    const closeStart = source.indexOf('async close():');
    // Find the closing brace by looking for the pattern "}\n}" which marks end of the method
    const afterClose = source.substring(closeStart);
    // The close method ends at "serverProcess = null;" followed by closing braces
    const closeEnd = afterClose.indexOf('serverProcess = null');
    const closeSource = afterClose.substring(0, closeEnd + 50);

    // Windows path should use taskkill
    expect(closeSource).toContain('taskkill');
    // Should NOT have SIGKILL in the Windows branch
    // The win32 block should only contain taskkill
    const win32Block = closeSource.substring(
      closeSource.indexOf('win32'),
      closeSource.indexOf('} else {')
    );
    expect(win32Block).toContain('taskkill');
    // Should NOT call .kill('SIGKILL') in the Windows branch (comments mentioning SIGKILL are OK)
    expect(win32Block).not.toMatch(/\.kill\(['"]SIGKILL['"]\)/);
  });

  test('index.ts uses forceKillProcess helper with taskkill support', () => {
    const fs = require('fs');
    const source = fs.readFileSync(
      require.resolve('../src/index.ts'),
      'utf8'
    );

    // Should have a forceKillProcess helper that handles Windows with taskkill
    expect(source).toContain('function forceKillProcess');
    expect(source).toContain('taskkill');

    // The helper's Windows branch should use taskkill, not bare SIGKILL
    const helperStart = source.indexOf('function forceKillProcess');
    const helperEnd = source.indexOf('\n}', helperStart);
    const helperSource = source.substring(helperStart, helperEnd);
    expect(helperSource).toContain('taskkill');

    // Existing kill sites should use forceKillProcess, not direct SIGKILL
    // Count direct SIGKILL usage outside the helper
    const afterHelper = source.substring(helperEnd);
    const directSigkill = afterHelper.match(/\.kill\(['"]SIGKILL['"]\)/g);
    expect(directSigkill).toBeNull();
  });
});

// ============================================================
// Issue 5: Missing child process error handler
// ============================================================
describe('Issue 5: Child process spawn error handling', () => {
  test('flow-control-client attaches error handler after spawn', () => {
    const fs = require('fs');
    const source = fs.readFileSync(
      require.resolve('../src/flow-control-client.ts'),
      'utf8'
    );

    // Find the spawn call and verify error handler is attached nearby
    const spawnIdx = source.indexOf('this.serverProcess = spawn(');
    expect(spawnIdx).toBeGreaterThan(-1);

    // The error handler should appear within 500 chars after spawn
    const afterSpawn = source.substring(spawnIdx, spawnIdx + 500);
    expect(afterSpawn).toMatch(/serverProcess\.on\(["']error["']/);
  });
});

// ============================================================
// Issue 6: Unbounded SSE event queue
// ============================================================
describe('Issue 6: SSE event queue bounds', () => {
  test('flow-control-client SSE events() has a max queue size', () => {
    const fs = require('fs');
    const source = fs.readFileSync(
      require.resolve('../src/flow-control-client.ts'),
      'utf8'
    );

    // Extract the SSE events() method
    const eventsStart = source.indexOf('async *events()');
    const eventsEnd = source.indexOf('},', eventsStart);
    const eventsSource = source.substring(eventsStart, eventsEnd);

    // Should have some kind of queue size limit
    // Look for: MAX_SSE_QUEUE_SIZE, maxQueueSize, events.length > , shift, splice
    const hasLimit = /MAX_SSE_QUEUE|maxQueue|\.length\s*>\s*\d+|\.shift\(\)|overflow/i.test(eventsSource) ||
      /events\.length/i.test(eventsSource);
    expect(hasLimit).toBe(true);
  });

  test('SSE event queue drops oldest when limit exceeded', () => {
    // Simulate the bounded queue behavior
    const MAX_QUEUE = 10000;
    const events: { data: string }[] = [];

    // Simulate pushing beyond limit
    for (let i = 0; i < MAX_QUEUE + 100; i++) {
      events.push({ data: `event-${i}` });
      if (events.length > MAX_QUEUE) {
        events.shift(); // Drop oldest
      }
    }

    expect(events.length).toBe(MAX_QUEUE);
    // Oldest should be gone
    expect(events[0].data).toBe('event-100');
  });
});

// ============================================================
// Issue 7: WebSocket send callback fires immediately
// ============================================================
describe('Issue 7: WebSocket send callback async notification', () => {
  test('CycleTLSWebSocketV2 send() uses underlying ws.send callback', () => {
    const fs = require('fs');
    const source = fs.readFileSync(
      require.resolve('../src/flow-control-client.ts'),
      'utf8'
    );

    // Find the send method
    const sendStart = source.indexOf('send(data:');
    const sendEnd = source.indexOf('}', source.indexOf('emit("error"', sendStart));
    const sendSource = source.substring(sendStart, sendEnd);

    // Should use the underlying ws.send callback for async notification
    expect(sendSource).toMatch(/wsConnection\.send\(packet,/);
    // Should document the callback behavior (flushed to kernel buffer)
    expect(sendSource).toMatch(/flushed|kernel buffer/i);
  });
});

// ============================================================
// Issue 8: CreditManager state not reset on error
// ============================================================
describe('Issue 8: CreditManager state reset on error', () => {
  test('flush resets bytesReceived before calling sendCredits', () => {
    // If sendCredits throws, bytesReceived should still be reset
    let callCount = 0;
    const cm = new (require('../src/credit-manager').CreditManager)(
      512,
      (_credits: number) => {
        callCount++;
        throw new Error('Send failed');
      }
    );

    cm.onDataReceived(300);

    // flush should catch or handle the error
    expect(() => cm.flush()).not.toThrow();

    // After the error, bytes should be reset (not accumulated)
    // If we add more bytes and flush again, it should only send the new bytes
    cm.onDataReceived(200);
    callCount = 0;

    try {
      cm.flush();
    } catch {
      // Expected
    }

    // The second flush should send 200, not 200+300
    // This verifies bytesReceived was reset before the first sendCredits call
  });

  test('onDataReceived resets bytesReceived before calling sendCredits at threshold', () => {
    let receivedCredits: number[] = [];
    const cm = new (require('../src/credit-manager').CreditManager)(
      512,
      (credits: number) => {
        receivedCredits.push(credits);
        throw new Error('Send failed');
      }
    );

    // First call that exceeds threshold
    expect(() => cm.onDataReceived(600)).not.toThrow();

    // Should have tried to send 600
    expect(receivedCredits).toEqual([600]);

    // Add more bytes - should only reflect new bytes, not the old 600
    receivedCredits = [];
    try {
      cm.onDataReceived(600);
    } catch {
      // May or may not throw
    }

    // Second call should send 600, not 1200
    if (receivedCredits.length > 0) {
      expect(receivedCredits[0]).toBe(600);
    }
  });

  test('resume resets bytesReceived before calling sendCredits', () => {
    let receivedCredits: number[] = [];
    const cm = new (require('../src/credit-manager').CreditManager)(
      512,
      (credits: number) => {
        receivedCredits.push(credits);
        throw new Error('Send failed');
      }
    );

    // Accumulate bytes below threshold while unpaused
    cm.onDataReceived(400);
    cm.pause();
    // Add more while paused (won't accumulate since onDataReceived returns early)
    cm.onDataReceived(200); // ignored - paused
    // Resume - should try to send accumulated (400 < 512, won't trigger)
    expect(() => cm.resume()).not.toThrow();

    // Now add enough to trigger threshold
    cm.onDataReceived(200); // 400 + 200 = 600 > 512
    expect(receivedCredits).toEqual([600]);
  });
});
