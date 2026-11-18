module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  // Room for the retry backoff in tests/support/upstream-retry.js. Test files
  // that call jest.setTimeout keep their own value.
  testTimeout: 30000,
  // Tests import the client from ../dist/index.js. Route that import through a
  // wrapper that retries rate-limited public hosts (httpbin.org); see
  // tests/support/upstream-retry.js.
  moduleNameMapper: {
    '^\\.\\./dist/index(\\.js)?$': '<rootDir>/tests/support/upstream-retry.js',
  },
  setupFilesAfterEnv: ['<rootDir>/tests/support/jest.setup.js'],
};

global.performance = {
  now: () => Date.now(),
};
