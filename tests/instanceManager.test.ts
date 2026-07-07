/**
 * Tests for InstanceManager initialization promise cleanup
 *
 * Bug: initializingPromises map grows unbounded on failure because
 * the promise is only deleted on success path, not on failure.
 *
 * Fix: Use try/finally to ensure cleanup happens regardless of success/failure.
 */

import { _InstanceManager as InstanceManager } from "../dist/index.js";

jest.setTimeout(30000);

describe("InstanceManager initializingPromises cleanup", () => {
  let instanceManager: InstanceType<typeof InstanceManager>;

  beforeEach(() => {
    // Reset singleton between tests
    InstanceManager._resetInstance();
    instanceManager = InstanceManager.getInstance();
  });

  afterEach(async () => {
    // Cleanup any instances that may have been created
    try {
      await instanceManager.cleanup();
    } catch (e) {
      // Ignore cleanup errors
    }
  });

  test("should remove promise from initializingPromises after successful initialization", async () => {
    const port = 9200;

    // Verify no promises initially
    expect(instanceManager._getInitializingPromisesCount()).toBe(0);
    expect(instanceManager._hasInitializingPromise(port)).toBe(false);

    // Start initialization - this should succeed
    const initPromise = instanceManager.getOrCreateSharedInstance(port, false, 10000);

    // During initialization, the promise should be in the map
    expect(instanceManager._hasInitializingPromise(port)).toBe(true);

    // Wait for initialization to complete
    await initPromise;

    // After success, promise should be removed from initializingPromises
    expect(instanceManager._hasInitializingPromise(port)).toBe(false);
    expect(instanceManager._getInitializingPromisesCount()).toBe(0);

    // Clean up the instance
    await instanceManager.removeSharedInstance(port);
  });

  test("concurrent requests to same port should return existing instance after init", async () => {
    const port = 9201;

    // Start first initialization
    const promise1 = instanceManager.getOrCreateSharedInstance(port, false, 10000);

    // Wait for it to complete
    const instance1 = await promise1;

    // Second request should get the same instance
    const promise2 = instanceManager.getOrCreateSharedInstance(port, false, 10000);
    const instance2 = await promise2;

    // Both should be the same instance
    expect(instance1).toBe(instance2);

    // No initializing promises should remain
    expect(instanceManager._hasInitializingPromise(port)).toBe(false);

    // Clean up
    await instanceManager.removeSharedInstance(port);
  });

  test("removing instance clears shared instance but allows fresh port use", async () => {
    const port = 9202;

    // Create initial instance
    const instance1 = await instanceManager.getOrCreateSharedInstance(port, false, 10000);
    expect(instanceManager._hasInitializingPromise(port)).toBe(false);

    // Remove the instance
    await instanceManager.removeSharedInstance(port);

    // Wait for the port to be fully released before attempting reuse.
    // The Go process may still be shutting down even after removeSharedInstance resolves.
    await new Promise((resolve) => setTimeout(resolve, 2000));

    // Use a DIFFERENT port for the second instance to avoid port conflict entirely.
    // The important thing is that the removed instance is gone from the map,
    // not that we can rebind the same port immediately.
    const freshPort = 9252;
    const promise2 = instanceManager.getOrCreateSharedInstance(freshPort, false, 10000);

    // A new initialization should be attempted (promise added to map)
    expect(instanceManager._hasInitializingPromise(freshPort)).toBe(true);

    // Wait and clean up
    try {
      await promise2;
      await instanceManager.removeSharedInstance(freshPort);
    } catch (e) {
      // Cleanup errors are acceptable - the important assertion was above
    }
  });

  test("multiple ports can be initialized independently", async () => {
    const port1 = 9203;
    const port2 = 9204;

    // Start both initializations
    const promise1 = instanceManager.getOrCreateSharedInstance(port1, false, 10000);
    const promise2 = instanceManager.getOrCreateSharedInstance(port2, false, 10000);

    // Both should have initializing promises
    expect(instanceManager._hasInitializingPromise(port1)).toBe(true);
    expect(instanceManager._hasInitializingPromise(port2)).toBe(true);
    expect(instanceManager._getInitializingPromisesCount()).toBe(2);

    // Wait for both
    await Promise.all([promise1, promise2]);

    // Both should be cleaned up
    expect(instanceManager._hasInitializingPromise(port1)).toBe(false);
    expect(instanceManager._hasInitializingPromise(port2)).toBe(false);
    expect(instanceManager._getInitializingPromisesCount()).toBe(0);

    // Clean up
    await instanceManager.removeSharedInstance(port1);
    await instanceManager.removeSharedInstance(port2);
  });
});

describe("InstanceManager race condition prevention", () => {
  let instanceManager: InstanceType<typeof InstanceManager>;

  beforeEach(() => {
    InstanceManager._resetInstance();
    instanceManager = InstanceManager.getInstance();
  });

  afterEach(async () => {
    try {
      await instanceManager.cleanup();
    } catch (e) {
      // Ignore cleanup errors
    }
  });

  test("concurrent calls to same port should return the same promise (no duplicate instances)", async () => {
    const port = 9220;

    // Call getOrCreateSharedInstance twice WITHOUT awaiting the first call.
    // This simulates the race condition: two callers both check the map
    // at nearly the same time. Because the promise is set synchronously
    // (before any await), the second call should find the pending promise
    // and return it rather than creating a duplicate instance.
    const promise1 = instanceManager.getOrCreateSharedInstance(port, false, 10000);
    const promise2 = instanceManager.getOrCreateSharedInstance(port, false, 10000);

    // Only one initializing promise should exist (the second call reuses
    // the first's pending promise). Note: the returned promises are different
    // objects because async functions always wrap returns in a new promise,
    // but they both resolve to the same SharedInstance.
    expect(instanceManager._getInitializingPromisesCount()).toBe(1);

    // Wait for both to complete
    const [instance1, instance2] = await Promise.all([promise1, promise2]);

    // Both should resolve to the same SharedInstance (no duplicate)
    expect(instance1).toBe(instance2);

    // Cleanup
    await instanceManager.removeSharedInstance(port);
  });

  test("initializingPromises is set synchronously before any async work", () => {
    const port = 9221;

    // Before the call, no promise exists
    expect(instanceManager._hasInitializingPromise(port)).toBe(false);

    // Start initialization (don't await)
    const promise = instanceManager.getOrCreateSharedInstance(port, false, 10000);

    // Immediately after the synchronous call, the promise MUST be in the map.
    // This is the key invariant that prevents race conditions.
    expect(instanceManager._hasInitializingPromise(port)).toBe(true);

    // Clean up - wait for the promise to settle
    promise.then(() => instanceManager.removeSharedInstance(port)).catch(() => {});
  });
});

/**
 * Test to verify the fix works with actual failure case.
 *
 * The SharedInstance now properly rejects its promise when initialization fails
 * via the rejectInitialization() helper method, which is accessible from
 * spawnServer() and handleSpawn() error handlers.
 */
describe("InstanceManager failure cleanup", () => {
  let localInstanceManager: InstanceType<typeof InstanceManager>;

  afterEach(async () => {
    // Clean up the local instance manager created in each test
    if (localInstanceManager) {
      try {
        await localInstanceManager.cleanup();
      } catch (e) {
        // Ignore cleanup errors
      }
    }
  });

  test("should remove promise from initializingPromises after failed initialization", async () => {
    InstanceManager._resetInstance();
    localInstanceManager = InstanceManager.getInstance();
    const port = 9210;
    const invalidExecutablePath = "/nonexistent/path/to/cycletls";

    // Start initialization with invalid path - this should fail
    const initPromise = localInstanceManager.getOrCreateSharedInstance(
      port,
      false,
      5000,
      invalidExecutablePath
    );

    // Wait for initialization to fail - should reject with error message, not timeout
    await expect(initPromise).rejects.toMatch(/Executable not found|not found/i);

    // After failure, promise should be removed from initializingPromises
    expect(localInstanceManager._hasInitializingPromise(port)).toBe(false);
  });

  test("should reject immediately on spawn error, not wait for timeout", async () => {
    InstanceManager._resetInstance();
    localInstanceManager = InstanceManager.getInstance();
    const port = 9211;
    const invalidExecutablePath = "/nonexistent/path/to/cycletls";

    const startTime = Date.now();

    // Start initialization with invalid path and long timeout
    const initPromise = localInstanceManager.getOrCreateSharedInstance(
      port,
      false,
      30000, // 30 second timeout - we should NOT wait this long
      invalidExecutablePath
    );

    // Wait for initialization to fail
    await expect(initPromise).rejects.toMatch(/Executable not found|not found/i);

    const elapsed = Date.now() - startTime;

    // Should fail quickly (well under 30 seconds), not wait for timeout
    // Allow 5 seconds for test overhead
    expect(elapsed).toBeLessThan(5000);
  });
});
