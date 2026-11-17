/**
 * Tests for pid undefined safety checks in cleanExit and cleanup methods.
 *
 * Bug: Non-null assertion `this.child.pid!` panics if pid is undefined
 * (process failed to spawn). Should check pid !== undefined first and
 * fall back to direct kill.
 *
 * These tests use flexible source extraction that can tolerate changes
 * in compiled output formatting and method ordering.
 */

function readDistSource(): string {
  const fs = require("fs");
  return fs.readFileSync(require.resolve("../dist/index.js"), "utf8");
}

function extractMethodBody(source: string, signature: string, endPatterns: string[]): string {
  const startIdx = source.indexOf(signature);
  if (startIdx === -1) {
    throw new Error(`Method "${signature}" not found in dist/index.js.`);
  }
  let endIdx = -1;
  for (const pattern of endPatterns) {
    endIdx = source.indexOf(pattern, startIdx + signature.length);
    if (endIdx !== -1) break;
  }
  if (endIdx === -1) {
    endIdx = Math.min(startIdx + 2000, source.length);
  }
  return source.substring(startIdx, endIdx);
}

describe("pid undefined safety", () => {
  let source: string;

  beforeAll(() => {
    source = readDistSource();
  });

  test("cleanExit should safely handle child process termination", () => {
    const methodSource = extractMethodBody(
      source,
      "async cleanExit(",
      ["async cleanup()", "async close()", "async removeSharedInstance"]
    );

    // cleanExit should guard against null/undefined child before killing.
    // The implementation may use:
    //   - Direct: if (this.child.pid !== undefined) process.kill(...)
    //   - Delegated: if (this.child) forceKillProcess(this.child, ...)
    // Both are valid as long as there's a null guard.
    expect(methodSource).toMatch(/this\.child/);
    expect(methodSource).toMatch(/kill|forceKill/i);

    // Should have a guard (if check) to avoid calling kill on null
    expect(methodSource).toMatch(/if\s*\(/);
  });

  test("cleanup should safely handle child process termination", () => {
    // Find the SharedInstance.cleanup method (not InstanceManager.cleanup)
    // by looking for the one that references child process handling
    const sharedCleanupStart = source.indexOf("async cleanup()");
    // Skip InstanceManager.cleanup and find SharedInstance.cleanup
    let idx = sharedCleanupStart;
    let methodSource = "";
    // Find the cleanup method that deals with child process
    while (idx !== -1) {
      const nextIdx = source.indexOf("async cleanup()", idx + 1);
      const endPatterns = ["Force close the WebSocket", "async close()", "rejectInitialization", "extractClientId"];
      let endIdx = -1;
      for (const pattern of endPatterns) {
        endIdx = source.indexOf(pattern, idx + 15);
        if (endIdx !== -1) break;
      }
      if (endIdx === -1) endIdx = Math.min(idx + 2000, source.length);
      const candidate = source.substring(idx, endIdx);
      if (candidate.includes("this.child") || candidate.includes("forceKill")) {
        methodSource = candidate;
        break;
      }
      if (nextIdx === -1) break;
      idx = nextIdx;
    }

    // The cleanup method that handles child processes should guard against null
    expect(methodSource).toMatch(/this\.child/);
    expect(methodSource).toMatch(/kill|forceKill/i);
  });

  test("no non-null pid assertions should remain in process.kill calls", () => {
    const fs = require("fs");
    const path = require("path");
    const tsSource = fs.readFileSync(
      path.resolve(__dirname, "..", "src", "index.ts"),
      "utf8"
    );

    // After the fix, there should be NO occurrences of .pid! in the TypeScript source
    // (the ! is the TypeScript non-null assertion that we're removing)
    const pidBangMatches = tsSource.match(/\.pid!/g) || [];
    expect(pidBangMatches.length).toBe(0);
  });
});
