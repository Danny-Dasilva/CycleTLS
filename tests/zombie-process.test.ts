/**
 * Tests for zombie process prevention on early initialization rejection.
 *
 * Bug: When rejectInitialization() is called, the spawned child process
 * is not killed, leaving zombie processes.
 *
 * Fix: Kill the child process in rejectInitialization() before rejecting.
 *
 * These tests use a flexible source extraction approach that searches for
 * the method body using multiple candidate patterns, making them resilient
 * to minification, reformatting, or renaming in the compiled output.
 */

function readDistSource(): string {
  const fs = require("fs");
  return fs.readFileSync(require.resolve("../dist/index.js"), "utf8");
}

/**
 * Extract a method body from source code using flexible pattern matching.
 * Returns the source between the method signature and the next method/class boundary.
 */
function extractMethodBody(source: string, methodName: string, endPatterns: string[]): string {
  const startIdx = source.indexOf(methodName);
  if (startIdx === -1) {
    throw new Error(`Method "${methodName}" not found in dist/index.js. The method may have been renamed or removed.`);
  }

  // Find the end using the first matching pattern
  let endIdx = -1;
  for (const pattern of endPatterns) {
    endIdx = source.indexOf(pattern, startIdx + methodName.length);
    if (endIdx !== -1) break;
  }

  if (endIdx === -1) {
    // Fallback: take a generous 2000 char window
    endIdx = Math.min(startIdx + 2000, source.length);
  }

  return source.substring(startIdx, endIdx);
}

describe("Zombie process prevention", () => {
  let source: string;

  beforeAll(() => {
    source = readDistSource();
  });

  test("rejectInitialization source should kill child process", () => {
    const methodSource = extractMethodBody(
      source,
      "rejectInitialization",
      ["initialize()", "async initialize", "getOrCreate"]
    );

    // The method should reference the child process and kill it.
    // The implementation may use forceKillProcess() helper or direct process.kill().
    expect(methodSource).toMatch(/this\.child/);
    expect(methodSource).toMatch(/kill|forceKill/i);
  });

  test("rejectInitialization should handle missing child process gracefully", () => {
    const methodSource = extractMethodBody(
      source,
      "rejectInitialization",
      ["initialize()", "async initialize", "getOrCreate"]
    );

    // Should have a null/undefined check for child process (if guard)
    expect(methodSource).toMatch(/if\s*\(\s*this\.child/);
    // Should nullify child after killing to prevent double-kill
    expect(methodSource).toMatch(/this\.child\s*=\s*null/);
  });

  test("rejectInitialization should ensure process cleanup via forceKillProcess or process.kill", () => {
    // The process group kill logic may be in rejectInitialization directly
    // or delegated to a helper function like forceKillProcess.
    // Verify either approach is present in the codebase.
    const methodSource = extractMethodBody(
      source,
      "rejectInitialization",
      ["initialize()", "async initialize", "getOrCreate"]
    );

    // Should use forceKillProcess helper or inline process.kill
    const usesForceKill = /forceKillProcess/.test(methodSource);
    const usesProcessKill = /process\.kill/.test(methodSource);
    expect(usesForceKill || usesProcessKill).toBe(true);

    // If delegated to forceKillProcess, verify the helper handles platform differences
    if (usesForceKill) {
      const forceKillBody = extractMethodBody(
        source,
        "forceKillProcess",
        ["class ", "module.exports", "exports."]
      );
      // The helper should handle process group kill and platform differences
      expect(forceKillBody).toMatch(/kill/);
      expect(forceKillBody).toMatch(/win32|platform/);
    }
  });
});
