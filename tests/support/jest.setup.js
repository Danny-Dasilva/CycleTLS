// Loaded by jest.config.js (setupFilesAfterEnv).
//
// Jest cannot skip a test once it is running. When the retrying client in
// ./upstream-retry.js gives up on a rate-limited public host it throws
// UpstreamUnavailableError; the wrapper below catches that one error, logs why
// the test did not run to the end, and lets it finish without a failure. Every
// other error, including failed assertions, is rethrown unchanged.

const guard = (fn) => {
  // Leave callback-style tests (fn takes `done`) alone.
  if (typeof fn !== "function" || fn.length > 0) return fn;

  return async function guardedTest() {
    try {
      return await fn.call(this);
    } catch (error) {
      if (error && error.name === "UpstreamUnavailableError") {
        const name = expect.getState().currentTestName;
        console.warn(`SKIPPED (upstream unavailable): ${name}\n  ${error.message}`);
        return undefined;
      }
      throw error;
    }
  };
};

const patch = (base) => {
  const wrapped = (name, fn, timeout) => base(name, guard(fn), timeout);
  Object.assign(wrapped, base);
  wrapped.only = (name, fn, timeout) => base.only(name, guard(fn), timeout);
  Object.assign(wrapped.only, base.only);
  return wrapped;
};

global.test = patch(global.test);
global.it = patch(global.it);
