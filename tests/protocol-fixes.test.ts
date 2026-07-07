/**
 * Tests for protocol.ts fixes from TS Flow Control/Protocol review.
 *
 * Covers:
 * - Issue #3 (CRITICAL): writeString byte length bug with multi-byte UTF-8
 * - Issue #4 (CRITICAL): Empty payload / minimum length validation in parseFrame
 * - Issue #6 (MAJOR): BufferReader bounds checks for readU8, readU16, readU32
 * - Issue #10 (MAJOR): parseWebSocketMessagePayload messageType validation
 */

import {
  BufferReader,
  parseFrame,
  parseWebSocketMessagePayload,
  buildInitPacket,
  buildCreditPacket,
  buildWebSocketSendPacket,
  buildWebSocketClosePacket,
} from "../src/protocol";

// ============================================================================
// Issue #3 (CRITICAL): writeString uses Buffer.byteLength, not string.length
// ============================================================================

describe("Issue #3: writeString multi-byte UTF-8 encoding", () => {
  test("buildInitPacket with ASCII-only requestId produces correct frame", () => {
    const packet = buildInitPacket("req-1", { url: "http://test.com" }, 65536);
    const r = new BufferReader(packet);
    const requestId = r.readString();
    expect(requestId).toBe("req-1");
    const method = r.readString();
    expect(method).toBe("init");
  });

  test("buildInitPacket with multi-byte UTF-8 characters produces correct frame", () => {
    // Japanese characters: each is 3 bytes in UTF-8
    const unicodeId = "\u6D4B\u8BD5"; // 2 chars, 6 bytes in UTF-8
    const packet = buildInitPacket(unicodeId, { url: "http://test.com" }, 1024);
    const r = new BufferReader(packet);
    const requestId = r.readString();
    expect(requestId).toBe(unicodeId);
    const method = r.readString();
    expect(method).toBe("init");
  });

  test("buildInitPacket with emoji characters produces correct frame", () => {
    // Emoji: 4 bytes each in UTF-8
    const emojiId = "req-\u{1F600}";
    const packet = buildInitPacket(emojiId, {}, 1024);
    const r = new BufferReader(packet);
    const requestId = r.readString();
    expect(requestId).toBe(emojiId);
  });

  test("length prefix matches byte length, not character count", () => {
    // Build a packet with a known multi-byte string
    const testStr = "\u00E9\u00E8\u00EA"; // e-acute, e-grave, e-circumflex: 2 bytes each = 6 bytes
    expect(testStr.length).toBe(3); // 3 characters
    expect(Buffer.byteLength(testStr, "utf8")).toBe(6); // 6 bytes

    const packet = buildCreditPacket(testStr, 100);
    // Read the length prefix manually
    const lengthPrefix = packet.readUInt16BE(0);
    expect(lengthPrefix).toBe(6); // Should be byte length, not char count
  });

  test("round-trip multi-byte strings through buildCreditPacket + parseFrame", () => {
    const multiByteId = "req-\u00FC\u00E4\u00F6"; // German umlauts
    const packet = buildCreditPacket(multiByteId, 512);
    const frame = parseFrame(packet);
    expect(frame.requestId).toBe(multiByteId);
    expect(frame.method).toBe("credit");
  });

  test("buildWebSocketSendPacket with multi-byte requestId", () => {
    const unicodeId = "\u4E16\u754C"; // Chinese "world", 6 bytes UTF-8
    const data = Buffer.from("hello");
    const packet = buildWebSocketSendPacket(unicodeId, data, false);
    const r = new BufferReader(packet);
    const requestId = r.readString();
    expect(requestId).toBe(unicodeId);
  });

  test("buildWebSocketClosePacket with multi-byte reason", () => {
    const reason = "fermeture anormale \u00E0 cause d'erreur"; // French with accent
    const packet = buildWebSocketClosePacket("req-1", 1006, reason);
    const frame = parseFrame(packet);
    expect(frame.requestId).toBe("req-1");
    expect(frame.method).toBe("ws_close");
  });
});

// ============================================================================
// Issue #4 (CRITICAL): parseFrame minimum length validation
// ============================================================================

describe("Issue #4: parseFrame empty payload / minimum length validation", () => {
  test("rejects empty buffer", () => {
    expect(() => parseFrame(Buffer.alloc(0))).toThrow(/minimum header size/);
  });

  test("rejects buffer with 1 byte", () => {
    expect(() => parseFrame(Buffer.alloc(1))).toThrow(/minimum header size/);
  });

  test("rejects buffer with 2 bytes (only requestId length prefix)", () => {
    const buf = Buffer.alloc(2);
    buf.writeUInt16BE(0, 0); // empty requestId length prefix
    expect(() => parseFrame(buf)).toThrow(/minimum header size/);
  });

  test("rejects buffer with 3 bytes", () => {
    expect(() => parseFrame(Buffer.alloc(3))).toThrow(/minimum header size/);
  });

  test("accepts minimal valid frame (4 bytes: two empty strings)", () => {
    // 2 bytes for empty requestId + 2 bytes for empty method
    const buf = Buffer.alloc(4);
    buf.writeUInt16BE(0, 0); // requestId length = 0
    buf.writeUInt16BE(0, 2); // method length = 0
    const frame = parseFrame(buf);
    expect(frame.requestId).toBe("");
    expect(frame.method).toBe("");
    expect(frame.payload.length).toBe(0);
  });

  test("rejects buffer with valid requestId but truncated method", () => {
    // 2 bytes for "ab" length prefix + 2 bytes "ab" = 4 bytes, no room for method length
    const buf = Buffer.alloc(4);
    buf.writeUInt16BE(2, 0); // requestId = 2 chars
    buf.write("ab", 2, "utf8");
    // Now the reader needs 2 more bytes for method length prefix, but buffer is only 4 bytes
    expect(() => parseFrame(buf)).toThrow(/underflow|exceeds/);
  });
});

// ============================================================================
// Issue #6 (MAJOR): BufferReader bounds checks
// ============================================================================

describe("Issue #6: BufferReader bounds checks", () => {
  test("readU8 throws on empty buffer", () => {
    const r = new BufferReader(Buffer.alloc(0));
    expect(() => r.readU8()).toThrow(/Buffer underflow/);
  });

  test("readU8 reads valid single byte", () => {
    const buf = Buffer.alloc(1);
    buf.writeUInt8(42, 0);
    const r = new BufferReader(buf);
    expect(r.readU8()).toBe(42);
  });

  test("readU8 throws on second read when only 1 byte available", () => {
    const buf = Buffer.alloc(1);
    buf.writeUInt8(1, 0);
    const r = new BufferReader(buf);
    r.readU8(); // first read succeeds
    expect(() => r.readU8()).toThrow(/Buffer underflow/);
  });

  test("readU16 throws on empty buffer", () => {
    const r = new BufferReader(Buffer.alloc(0));
    expect(() => r.readU16()).toThrow(/Buffer underflow/);
  });

  test("readU16 throws on 1-byte buffer", () => {
    const r = new BufferReader(Buffer.alloc(1));
    expect(() => r.readU16()).toThrow(/Buffer underflow/);
  });

  test("readU16 reads valid 2 bytes", () => {
    const buf = Buffer.alloc(2);
    buf.writeUInt16BE(12345, 0);
    const r = new BufferReader(buf);
    expect(r.readU16()).toBe(12345);
  });

  test("readU32 throws on empty buffer", () => {
    const r = new BufferReader(Buffer.alloc(0));
    expect(() => r.readU32()).toThrow(/Buffer underflow/);
  });

  test("readU32 throws on 3-byte buffer", () => {
    const r = new BufferReader(Buffer.alloc(3));
    expect(() => r.readU32()).toThrow(/Buffer underflow/);
  });

  test("readU32 reads valid 4 bytes", () => {
    const buf = Buffer.alloc(4);
    buf.writeUInt32BE(999999, 0);
    const r = new BufferReader(buf);
    expect(r.readU32()).toBe(999999);
  });

  test("sequential reads exhaust buffer correctly", () => {
    // 1 + 2 + 4 = 7 bytes
    const buf = Buffer.alloc(7);
    buf.writeUInt8(0xff, 0);
    buf.writeUInt16BE(0xabcd, 1);
    buf.writeUInt32BE(0x12345678, 3);
    const r = new BufferReader(buf);
    expect(r.readU8()).toBe(0xff);
    expect(r.readU16()).toBe(0xabcd);
    expect(r.readU32()).toBe(0x12345678);
    // Now buffer is exhausted
    expect(() => r.readU8()).toThrow(/Buffer underflow/);
  });

  test("readString after partial read with insufficient remaining bytes", () => {
    // 2 bytes for U16 + 2 bytes for string length prefix, but string says 100 bytes
    const buf = Buffer.alloc(6);
    buf.writeUInt16BE(42, 0);
    buf.writeUInt16BE(100, 2); // string claims 100 bytes, only 2 remain
    const r = new BufferReader(buf);
    r.readU16(); // consume first 2 bytes
    expect(() => r.readString()).toThrow(/exceeds/);
  });
});

// ============================================================================
// Issue #10 (MAJOR): parseWebSocketMessagePayload messageType validation
// ============================================================================

describe("Issue #10: parseWebSocketMessagePayload messageType validation", () => {
  function buildWsMessagePayload(messageType: number, data: Buffer): Buffer {
    const buf = Buffer.alloc(5 + data.length);
    buf.writeUInt8(messageType, 0);
    buf.writeUInt32BE(data.length, 1);
    data.copy(buf, 5);
    return buf;
  }

  test("accepts messageType 1 (text)", () => {
    const data = Buffer.from("hello");
    const payload = buildWsMessagePayload(1, data);
    const result = parseWebSocketMessagePayload(payload);
    expect(result.messageType).toBe(1);
    expect(result.data.toString("utf8")).toBe("hello");
  });

  test("accepts messageType 2 (binary)", () => {
    const data = Buffer.from([0x00, 0xff, 0x42]);
    const payload = buildWsMessagePayload(2, data);
    const result = parseWebSocketMessagePayload(payload);
    expect(result.messageType).toBe(2);
    expect(result.data).toEqual(data);
  });

  test("rejects messageType 0 (unknown)", () => {
    const payload = buildWsMessagePayload(0, Buffer.from("test"));
    expect(() => parseWebSocketMessagePayload(payload)).toThrow(/unknown messageType 0/);
  });

  test("rejects messageType 3 (unknown)", () => {
    const payload = buildWsMessagePayload(3, Buffer.from("test"));
    expect(() => parseWebSocketMessagePayload(payload)).toThrow(/unknown messageType 3/);
  });

  test("rejects messageType 255 (unknown)", () => {
    const payload = buildWsMessagePayload(255, Buffer.from("test"));
    expect(() => parseWebSocketMessagePayload(payload)).toThrow(/unknown messageType 255/);
  });

  test("rejects payload shorter than minimum 5 bytes", () => {
    expect(() => parseWebSocketMessagePayload(Buffer.alloc(0))).toThrow(/minimum 5 bytes/);
    expect(() => parseWebSocketMessagePayload(Buffer.alloc(4))).toThrow(/minimum 5 bytes/);
  });

  test("rejects truncated data", () => {
    const buf = Buffer.alloc(5);
    buf.writeUInt8(1, 0);
    buf.writeUInt32BE(100, 1); // claims 100 bytes of data, but none present
    expect(() => parseWebSocketMessagePayload(buf)).toThrow(/exceeds/);
  });
});
