/**
 * CommonJS surface of the package: `require('cycletls')` has to expose the
 * names the ESM build and the type declarations do.
 */
const fs = require("fs");
const path = require("path");
const { DIST, loadPackage } = require("./v2-helpers");

describe("require('cycletls')", () => {
  const pkg = loadPackage();

  test("exposes CycleTLSError, CreditManager and Protocol", () => {
    expect(typeof pkg.CycleTLSError).toBe("function");
    expect(typeof pkg.CreditManager).toBe("function");
    expect(typeof pkg.Protocol).toBe("object");
  });

  test("CycleTLSError is the class the client rejects with", () => {
    const err = new pkg.CycleTLSError("boom", 418, "req-1");
    expect(err).toBeInstanceOf(Error);
    expect(err.name).toBe("CycleTLSError");
    expect(err.statusCode).toBe(418);
    expect(err.requestId).toBe("req-1");
    expect(pkg.CycleTLSError).toBe(
      require(path.join(DIST, "flow-control-client.js")).CycleTLSError
    );
  });

  test("CreditManager and Protocol are usable", () => {
    const sent = [];
    const manager = new pkg.CreditManager(10, (n) => sent.push(n));
    manager.onDataReceived(12);
    expect(sent).toEqual([12]);

    expect(typeof pkg.Protocol.buildInitPacket).toBe("function");
    expect(typeof pkg.Protocol.parseFrame).toBe("function");
    expect(pkg.Protocol.buildCreditPacket("r", 5)).toBeInstanceOf(Buffer);
  });

  test("the names that were already there still are", () => {
    expect(pkg.default).toBe(pkg);
    expect(pkg.CycleTLS).toBe(pkg);
    expect(typeof pkg.initCycleTLS).toBe("function");
    expect(typeof pkg.CycleTLSWebSocketV2).toBe("function");
    expect(pkg.StreamingWebSocket).toBe(pkg.CycleTLSWebSocketV2);
  });

  test("every value the type declarations export exists at runtime", () => {
    const dts = fs.readFileSync(path.join(DIST, "index.d.ts"), "utf8");
    // Names that are types only, so have no runtime value.
    const typeOnly = new Set([
      "CycleTLSOptions",
      "RequestOptions",
      "Response",
      "SSEResponse",
    ]);
    const names = new Set();
    for (const m of dts.matchAll(/^export \{([^}]+)\}/gm)) {
      for (const part of m[1].split(",")) {
        const name = part.trim().split(/\s+as\s+/).pop().trim();
        if (name && !typeOnly.has(name)) names.add(name);
      }
    }
    for (const m of dts.matchAll(/^export \* as (\w+)/gm)) names.add(m[1]);

    expect(names.has("CycleTLSError")).toBe(true);
    const missing = [...names].filter((n) => pkg[n] === undefined);
    expect(missing).toEqual([]);
  });
});
