/**
 * Unit tests for protocol.ts buffer validation
 *
 * Tests that parse functions properly validate length prefixes against
 * actual buffer sizes to prevent silent truncation on malformed data.
 */

import {
  parseDataPayload,
  parseWebSocketOpenPayload,
  parseWebSocketMessagePayload,
  parseWebSocketClosePayload,
  BufferReader,
} from "../src/protocol";

describe("parseDataPayload", () => {
  test("parses valid data payload", () => {
    const content = Buffer.from("hello");
    const buf = Buffer.alloc(4 + content.length);
    buf.writeUInt32BE(content.length, 0);
    content.copy(buf, 4);
    const result = parseDataPayload(buf);
    expect(result.toString("utf8")).toBe("hello");
  });

  test("rejects truncated buffer", () => {
    const buf = Buffer.alloc(8);
    buf.writeUInt32BE(1000, 0);
    expect(() => parseDataPayload(buf)).toThrow(/exceeds/);
  });

  test("rejects zero-length buffer with nonzero length prefix", () => {
    const buf = Buffer.alloc(4);
    buf.writeUInt32BE(1, 0);
    expect(() => parseDataPayload(buf)).toThrow(/exceeds/);
  });
});

describe("parseWebSocketOpenPayload", () => {
  test("rejects truncated buffer", () => {
    const buf = Buffer.alloc(4);
    buf.writeUInt32BE(500, 0);
    expect(() => parseWebSocketOpenPayload(buf)).toThrow(/exceeds/);
  });

  test("parses valid payload", () => {
    const json = JSON.stringify({ protocol: "graphql-ws" });
    const jsonBuf = Buffer.from(json, "utf8");
    const buf = Buffer.alloc(4 + jsonBuf.length);
    buf.writeUInt32BE(jsonBuf.length, 0);
    jsonBuf.copy(buf, 4);
    const result = parseWebSocketOpenPayload(buf);
    expect(result.protocol).toBe("graphql-ws");
  });
});

describe("parseWebSocketMessagePayload", () => {
  test("rejects truncated buffer", () => {
    const buf = Buffer.alloc(5);
    buf.writeUInt8(1, 0);
    buf.writeUInt32BE(100, 1);
    expect(() => parseWebSocketMessagePayload(buf)).toThrow(/exceeds/);
  });

  test("parses valid payload", () => {
    const data = Buffer.from("test message");
    const buf = Buffer.alloc(5 + data.length);
    buf.writeUInt8(1, 0);
    buf.writeUInt32BE(data.length, 1);
    data.copy(buf, 5);
    const result = parseWebSocketMessagePayload(buf);
    expect(result.messageType).toBe(1);
    expect(result.data.toString("utf8")).toBe("test message");
  });
});

describe("parseWebSocketClosePayload", () => {
  test("rejects truncated buffer", () => {
    const buf = Buffer.alloc(4);
    buf.writeUInt32BE(200, 0);
    expect(() => parseWebSocketClosePayload(buf)).toThrow(/exceeds/);
  });

  test("parses valid payload", () => {
    const json = JSON.stringify({ code: 1001, reason: "going away" });
    const jsonBuf = Buffer.from(json, "utf8");
    const buf = Buffer.alloc(4 + jsonBuf.length);
    buf.writeUInt32BE(jsonBuf.length, 0);
    jsonBuf.copy(buf, 4);
    const result = parseWebSocketClosePayload(buf);
    expect(result.code).toBe(1001);
    expect(result.reason).toBe("going away");
  });
});

describe("BufferReader.readString", () => {
  test("rejects overflow length", () => {
    const buf = Buffer.alloc(4);
    buf.writeUInt16BE(500, 0);
    const r = new BufferReader(buf);
    expect(() => r.readString()).toThrow(/exceeds/);
  });

  test("reads valid string", () => {
    const str = "hello";
    const buf = Buffer.alloc(2 + str.length);
    buf.writeUInt16BE(str.length, 0);
    buf.write(str, 2, "utf8");
    const r = new BufferReader(buf);
    expect(r.readString()).toBe("hello");
  });

  test("reads empty string", () => {
    const buf = Buffer.alloc(2);
    buf.writeUInt16BE(0, 0);
    const r = new BufferReader(buf);
    expect(r.readString()).toBe("");
  });
});
