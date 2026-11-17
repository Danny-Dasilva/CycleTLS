module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  // Global per-test deadline. Prevents the Node.js Integration job from hanging
  // for the full 6h GitHub-runner kill when a test forgets jest.setTimeout and
  // its httpbin.org request hangs forever (e.g., Cloudflare-dropped TCP, no
  // server-side timeout). Tests that legitimately need longer can override
  // with their own jest.setTimeout(ms) call.
  testTimeout: 60000,
  moduleNameMapper: {
    // Map test-utils.js to test-utils (TypeScript file)
    '^(.*)/test-utils\\.js$': '$1/test-utils',
    // Map ./helpers.js to ./helpers for tlsfingerprint tests
    '^(.*)/helpers\\.js$': '$1/helpers'
  }
};

global.performance = {
  now: () => Date.now(),
};