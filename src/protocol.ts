/**
 * Binary Protocol Helpers for CycleTLS V2 Flow Control
 *
 * This module implements the binary protocol used for communication
 * between the TypeScript client and Go server in flow control mode.
 */

/**
 * Maximum string length for protocol encoding (uint16 length prefix = max 65535).
 */
const MAX_STRING_LEN = 65535;

/**
 * BufferWriter simplifies building binary packets.
 * Issue #2: All write methods include range validation to prevent overflows.
 */
class BufferWriter {
  private parts: Buffer[] = [];

  writeString(s: string): void {
    const strBuf = Buffer.from(s, "utf8");
    if (strBuf.length > MAX_STRING_LEN) {
      throw new Error(
        `String byte length ${strBuf.length} exceeds maximum ${MAX_STRING_LEN} for protocol encoding`
      );
    }
    const len = Buffer.alloc(2);
    len.writeUInt16BE(strBuf.length, 0);
    this.parts.push(len);
    this.parts.push(strBuf);
  }

  writeU16(v: number): void {
    if (v < 0 || v > 65535) {
      throw new RangeError(`U16 value ${v} out of range [0, 65535]`);
    }
    const buf = Buffer.alloc(2);
    buf.writeUInt16BE(v, 0);
    this.parts.push(buf);
  }

  writeU32(v: number): void {
    if (v < 0 || v > 4294967295) {
      throw new RangeError(`U32 value ${v} out of range [0, 4294967295]`);
    }
    const buf = Buffer.alloc(4);
    buf.writeUInt32BE(v, 0);
    this.parts.push(buf);
  }

  toBuffer(): Buffer {
    return Buffer.concat(this.parts);
  }
}

/**
 * Build an "init" packet to initialize a request.
 */
export function buildInitPacket(
  requestId: string,
  options: Record<string, unknown>,
  initialWindow: number
): Buffer {
  const w = new BufferWriter();
  w.writeString(requestId);
  w.writeString("init");
  w.writeU32(initialWindow);
  w.writeString(JSON.stringify(options));
  return w.toBuffer();
}

/**
 * Build a "credit" packet to replenish the server's credit window.
 */
export function buildCreditPacket(requestId: string, credits: number): Buffer {
  const w = new BufferWriter();
  w.writeString(requestId);
  w.writeString("credit");
  w.writeU32(credits);
  return w.toBuffer();
}

/**
 * BufferReader simplifies parsing binary packets.
 * Issue #2: All read methods include bounds checking to prevent underflows.
 */
export class BufferReader {
  private offset = 0;

  constructor(private data: Buffer) {}

  readU8(): number {
    if (this.offset + 1 > this.data.length) {
      throw new Error(
        `Buffer underflow: need 1 byte at offset ${this.offset}, but only ${this.data.length - this.offset} remaining`
      );
    }
    const v = this.data.readUInt8(this.offset);
    this.offset += 1;
    return v;
  }

  readU16(): number {
    if (this.offset + 2 > this.data.length) {
      throw new Error(
        `Buffer underflow: need 2 bytes at offset ${this.offset}, but only ${this.data.length - this.offset} remaining`
      );
    }
    const v = this.data.readUInt16BE(this.offset);
    this.offset += 2;
    return v;
  }

  readU32(): number {
    if (this.offset + 4 > this.data.length) {
      throw new Error(
        `Buffer underflow: need 4 bytes at offset ${this.offset}, but only ${this.data.length - this.offset} remaining`
      );
    }
    const v = this.data.readUInt32BE(this.offset);
    this.offset += 4;
    return v;
  }

  readString(): string {
    const len = this.readU16();
    if (this.offset + len > this.data.length) {
      throw new Error(
        `String length ${len} exceeds remaining buffer ${this.data.length - this.offset}`
      );
    }
    const s = this.data.toString("utf8", this.offset, this.offset + len);
    this.offset += len;
    return s;
  }

  remaining(): Buffer {
    return this.data.subarray(this.offset);
  }
}

/**
 * Parsed frame from server.
 */
export interface ParsedFrame {
  requestId: string;
  method: string;
  payload: Buffer;
}

/**
 * Minimum frame size: 2 bytes (requestId length prefix) + 2 bytes (method length prefix) = 4 bytes.
 * An empty requestId and empty method still require 4 bytes for their length prefixes.
 */
const MIN_FRAME_HEADER_SIZE = 4;

export function parseFrame(data: Buffer): ParsedFrame {
  if (data.length < MIN_FRAME_HEADER_SIZE) {
    throw new Error(
      `Invalid frame: buffer length ${data.length} is less than minimum header size ${MIN_FRAME_HEADER_SIZE}`
    );
  }
  const r = new BufferReader(data);
  const requestId = r.readString();
  const method = r.readString();
  const payload = r.remaining();
  return { requestId, method, payload };
}

/**
 * Parsed response payload.
 */
export interface ResponsePayload {
  statusCode: number;
  finalUrl: string;
  headers: Record<string, string[]>;
}

export function parseResponsePayload(payload: Buffer): ResponsePayload {
  const r = new BufferReader(payload);
  const statusCode = r.readU16();
  const finalUrl = r.readString();

  const headerCount = r.readU16();
  const headers: Record<string, string[]> = {};

  for (let i = 0; i < headerCount; i++) {
    const name = r.readString();
    const valueCount = r.readU16();
    const values: string[] = [];
    for (let j = 0; j < valueCount; j++) {
      values.push(r.readString());
    }
    headers[name] = values;
  }

  return { statusCode, finalUrl, headers };
}

/**
 * Parse a "data" frame payload.
 */
export function parseDataPayload(payload: Buffer): Buffer {
  const r = new BufferReader(payload);
  const length = r.readU32();
  if (4 + length > payload.length) {
    throw new Error(
      `Invalid data frame: length ${length} exceeds payload size ${payload.length - 4}`
    );
  }
  return payload.subarray(4, 4 + length);
}

/**
 * Parsed error payload.
 */
export interface ErrorPayload {
  statusCode: number;
  message: string;
}

export function parseErrorPayload(payload: Buffer): ErrorPayload {
  const r = new BufferReader(payload);
  const statusCode = r.readU16();
  const message = r.readString();
  return { statusCode, message };
}

// -----------------------------------------------------------------------------
// WebSocket Frame Parsers
// -----------------------------------------------------------------------------

/**
 * WebSocket open payload with negotiated protocol and extensions.
 */
export interface WebSocketOpenPayload {
  protocol?: string;
  extensions?: string;
}

/**
 * Parse a "ws_open" frame payload.
 * Format: length (4 bytes) + JSON data
 */
export function parseWebSocketOpenPayload(payload: Buffer): WebSocketOpenPayload {
  // Read length prefix (4 bytes)
  const length = payload.readUInt32BE(0);
  if (4 + length > payload.length) {
    throw new Error(
      `Invalid ws_open frame: length ${length} exceeds payload size ${payload.length - 4}`
    );
  }
  // Extract JSON data after the length prefix
  const jsonData = payload.subarray(4, 4 + length);
  const json = jsonData.toString("utf8");
  if (!json || json.length === 0) {
    return {};
  }
  try {
    return JSON.parse(json) as WebSocketOpenPayload;
  } catch {
    return {};
  }
}

/**
 * WebSocket message payload.
 */
export interface WebSocketMessagePayload {
  /** 1 = text, 2 = binary */
  messageType: number;
  data: Buffer;
}

/** Known WebSocket message types: 1 = text, 2 = binary */
const KNOWN_WS_MESSAGE_TYPES = new Set([1, 2]);

/**
 * Parse a "ws_message" frame payload.
 * Format: messageType (1 byte) + length (4 bytes) + data
 */
export function parseWebSocketMessagePayload(payload: Buffer): WebSocketMessagePayload {
  if (payload.length < 5) {
    throw new Error(
      `Invalid ws_message frame: payload length ${payload.length} is less than minimum 5 bytes`
    );
  }
  const messageType = payload.readUInt8(0);
  if (!KNOWN_WS_MESSAGE_TYPES.has(messageType)) {
    throw new Error(
      `Invalid ws_message frame: unknown messageType ${messageType} (expected 1=text or 2=binary)`
    );
  }
  const length = payload.readUInt32BE(1);
  if (5 + length > payload.length) {
    throw new Error(
      `Invalid ws_message frame: length ${length} exceeds payload size ${payload.length - 5}`
    );
  }
  const data = payload.subarray(5, 5 + length);
  return { messageType, data };
}

/**
 * WebSocket close payload.
 */
export interface WebSocketClosePayload {
  code: number;
  reason: string;
}

/**
 * Parse a "ws_close" frame payload.
 * Format: length (4 bytes) + JSON data
 */
export function parseWebSocketClosePayload(payload: Buffer): WebSocketClosePayload {
  // Read length prefix (4 bytes)
  const length = payload.readUInt32BE(0);
  if (4 + length > payload.length) {
    throw new Error(
      `Invalid ws_close frame: length ${length} exceeds payload size ${payload.length - 4}`
    );
  }
  // Extract JSON data after the length prefix
  const jsonData = payload.subarray(4, 4 + length);
  const json = jsonData.toString("utf8");
  if (!json || json.length === 0) {
    return { code: 1000, reason: "" };
  }
  try {
    const parsed = JSON.parse(json) as { code?: number; reason?: string };
    return {
      code: parsed.code ?? 1000,
      reason: parsed.reason ?? "",
    };
  } catch {
    return { code: 1000, reason: "" };
  }
}

// -----------------------------------------------------------------------------
// WebSocket Frame Builders (client to server)
// -----------------------------------------------------------------------------

/**
 * Build a WebSocket send packet to send data to the server.
 * Format: requestId (length-prefixed) + "ws_send" (length-prefixed) + messageType (1 byte) + dataLength (4 bytes) + data
 */
export function buildWebSocketSendPacket(
  requestId: string,
  data: Buffer,
  isBinary: boolean
): Buffer {
  const w = new BufferWriter();
  w.writeString(requestId);
  w.writeString("ws_send");
  // Message type: 1 = text, 2 = binary
  const messageType = isBinary ? 2 : 1;
  const typeBuf = Buffer.alloc(1);
  typeBuf.writeUInt8(messageType, 0);
  // Data length (4 bytes, big-endian)
  const lenBuf = Buffer.alloc(4);
  lenBuf.writeUInt32BE(data.length, 0);
  return Buffer.concat([w.toBuffer(), typeBuf, lenBuf, data]);
}

/**
 * Build a WebSocket close packet.
 */
export function buildWebSocketClosePacket(
  requestId: string,
  code: number,
  reason: string
): Buffer {
  const w = new BufferWriter();
  w.writeString(requestId);
  w.writeString("ws_close");
  w.writeU32(code);
  w.writeString(reason);
  return w.toBuffer();
}
