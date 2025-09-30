const initCycleTLS = require("../dist/index.js");

// Track all active instances for emergency cleanup
const activeInstances = new Set();

/**
 * Wrapper function that ensures CycleTLS instance cleanup even if test fails.
 * Use this for individual test cases that need a single CycleTLS instance.
 *
 * @param {number|object} portOrOptions - Port number or full options object
 * @param {Function} testFn - Test function that receives the CycleTLS instance
 * @returns {Promise} Promise that resolves with test function result
 *
 * @example
 * test("Should handle timeout", async () => {
 *   await withCycleTLS(9117, async (cycleTLS) => {
 *     const response = await cycleTLS('https://example.com');
 *     expect(response.status).toBe(200);
 *   });
 * });
 */
async function withCycleTLS(portOrOptions, testFn) {
  const options = typeof portOrOptions === 'number'
    ? { port: portOrOptions }
    : portOrOptions;

  const cycleTLS = await initCycleTLS(options);
  activeInstances.add(cycleTLS);

  try {
    return await testFn(cycleTLS);
  } finally {
    activeInstances.delete(cycleTLS);
    await cycleTLS.exit();
  }
}

/**
 * Alternative approach for tests that need multiple CycleTLS instances
 * or want more control over instance lifecycle.
 *
 * @param {object} options - CycleTLS initialization options
 * @returns {Promise} CycleTLS instance that's tracked for cleanup
 *
 * @example
 * test("Multiple instances", async () => {
 *   const cycleTLS1 = await createSafeCycleTLS({ port: 9001 });
 *   const cycleTLS2 = await createSafeCycleTLS({ port: 9002 });
 *
 *   try {
 *     // Test logic here
 *   } finally {
 *     await cleanupCycleTLS(cycleTLS1);
 *     await cleanupCycleTLS(cycleTLS2);
 *   }
 * });
 */
async function createSafeCycleTLS(options) {
  const cycleTLS = await initCycleTLS(options);
  activeInstances.add(cycleTLS);
  return cycleTLS;
}

/**
 * Manual cleanup function for instances created with createSafeCycleTLS
 *
 * @param {object} instance - CycleTLS instance to cleanup
 */
async function cleanupCycleTLS(instance) {
  if (activeInstances.has(instance)) {
    activeInstances.delete(instance);
    await instance.exit();
  }
}

/**
 * Wrapper for test suites that share a single CycleTLS instance
 * Use this with Jest's beforeAll/afterAll hooks
 *
 * @param {object} options - CycleTLS initialization options
 * @returns {object} Object with instance and cleanup function
 *
 * @example
 * describe("Test Suite", () => {
 *   let cycleTLS;
 *   let cleanup;
 *
 *   beforeAll(async () => {
 *     ({ instance: cycleTLS, cleanup } = await createSuiteInstance({ port: 9001 }));
 *   });
 *
 *   afterAll(async () => {
 *     await cleanup();
 *   });
 *
 *   test("test 1", async () => {
 *     // Use cycleTLS here
 *   });
 * });
 */
async function createSuiteInstance(options) {
  const instance = await createSafeCycleTLS(options);

  const cleanup = async () => {
    await cleanupCycleTLS(instance);
  };

  return { instance, cleanup };
}

/**
 * Get count of active CycleTLS instances (useful for debugging)
 */
function getActiveInstanceCount() {
  return activeInstances.size;
}

/**
 * Emergency cleanup of all active instances
 * This is called automatically on process exit
 */
async function cleanupAll() {
  if (activeInstances.size > 0) {
    console.warn(`⚠️  Cleaning up ${activeInstances.size} orphaned CycleTLS instances`);
    const instances = [...activeInstances];
    await Promise.all(instances.map(async (instance) => {
      try {
        await instance.exit();
      } catch (error) {
        console.error('Error cleaning up CycleTLS instance:', error);
      }
    }));
    activeInstances.clear();
  }
}

// Global cleanup hooks to prevent process hanging
// These will catch instances that weren't properly cleaned up
process.on('beforeExit', async () => {
  await cleanupAll();
});

process.on('SIGTERM', async () => {
  await cleanupAll();
  process.exit(0);
});

process.on('SIGINT', async () => {
  await cleanupAll();
  process.exit(0);
});

// Jest global cleanup hook
if (typeof afterAll !== 'undefined') {
  afterAll(async () => {
    await cleanupAll();
  });
}

module.exports = {
  withCycleTLS,
  createSafeCycleTLS,
  cleanupCycleTLS,
  createSuiteInstance,
  getActiveInstanceCount
};