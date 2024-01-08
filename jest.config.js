module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
};

global.performance = {
  now: () => Date.now(),
};