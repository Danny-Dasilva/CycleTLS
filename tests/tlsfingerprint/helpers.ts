/**
 * Test helpers for tlsfingerprint.com integration tests
 *
 * Mirrors the Go test helpers in cycletls/tests/integration/tlsfingerprint/helpers_test.go
 *
 * IMPORTANT: These tests require the Go binary to be rebuilt with V2 protocol support.
 * Run: npm run build:go:mac:arm64 (or appropriate platform variant)
 *
 * The Go binary must include the ws_handler_v2.go implementation.
 */

import * as zlib from "zlib";
import { promisify } from "util";
import * as https from "https";

// Promisified decompression functions
const gunzip = promisify(zlib.gunzip);
const inflate = promisify(zlib.inflate);
const inflateRaw = promisify(zlib.inflateRaw);
const brotliDecompress = promisify(zlib.brotliDecompress);

// Target server for all tests
export const TEST_SERVER_URL = "https://tlsfingerprint.com";

// Cache for service availability check
let serviceAvailable: boolean | null = null;

/**
 * Check if tlsfingerprint.com is available
 * Returns true if service returns 200, false otherwise (e.g., 521 Cloudflare error)
 */
export async function isServiceAvailable(): Promise<boolean> {
  if (serviceAvailable !== null) {
    return serviceAvailable;
  }

  return new Promise((resolve) => {
    let resolved = false;
    const complete = (result: boolean) => {
      if (!resolved) {
        resolved = true;
        serviceAvailable = result;
        resolve(result);
      }
    };

    const timeout = setTimeout(() => {
      complete(false);
    }, 5000); // Shorter timeout

    // Allow self-signed certificates for the test server
    const req = https.get(`${TEST_SERVER_URL}/get`, { rejectUnauthorized: false }, (res) => {
      clearTimeout(timeout);
      const result = res.statusCode === 200;
      // Consume response data to properly close connection
      res.on('data', () => {});
      res.on('end', () => {
        complete(result);
      });
      res.on('error', () => {
        complete(false);
      });
    });

    req.on('error', () => {
      clearTimeout(timeout);
      complete(false);
    });

    req.on('timeout', () => {
      req.destroy();
      complete(false);
    });

    req.setTimeout(5000);
  });
}

/**
 * Helper to skip test if service is unavailable
 */
export function skipIfServiceUnavailable(): void {
  if (serviceAvailable === false) {
    throw new Error('tlsfingerprint.com is unavailable (received 521 or timeout)');
  }
}

// Common JA3 fingerprint for all tests (Chrome 120)
export const DEFAULT_JA3 =
  "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0";

// Default User-Agent (Chrome 120 on Windows)
export const DEFAULT_USER_AGENT =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36";

/**
 * Common TLS fingerprint fields present in all responses from tlsfingerprint.com
 */
export interface TLSFingerprintResponse {
  ja3: string;
  ja3_hash: string;
  ja4: string;
  ja4_r?: string;
  akamai?: string;
  akamai_hash?: string;
  peetprint: string;
  peetprint_hash: string;
  http_version: string;
}

/**
 * Echo response (httpbin-style) from /get, /post, /put, etc.
 */
export interface EchoResponse extends TLSFingerprintResponse {
  args?: Record<string, unknown>;
  headers?: Record<string, string>;
  origin?: string;
  url?: string;
  method?: string;
  data?: string;
  form?: Record<string, unknown>;
  files?: Record<string, unknown>;
  json?: unknown;
}

/**
 * Compression response from /gzip, /deflate, /brotli
 */
export interface CompressionResponse extends TLSFingerprintResponse {
  gzipped?: boolean;
  deflated?: boolean;
  brotli?: boolean;
}

/**
 * Cookies response from /cookies
 */
export interface CookiesResponse extends TLSFingerprintResponse {
  cookies?: Record<string, string>;
}

/**
 * Redirect response from /redirect/N
 */
export interface RedirectResponse extends TLSFingerprintResponse {
  redirect_count?: number;
  location?: string;
}

/**
 * Status response from /status/N
 */
export interface StatusResponse extends TLSFingerprintResponse {
  status_code?: number;
}

/**
 * Delay response from /delay/N
 */
export interface DelayResponse extends EchoResponse {
  delay?: number;
}

/**
 * Get default request options for tests
 */
export function getDefaultOptions() {
  return {
    ja3: DEFAULT_JA3,
    userAgent: DEFAULT_USER_AGENT,
    // tlsfingerprint.com uses a self-signed certificate
    insecureSkipVerify: true,
  };
}

/**
 * Required TLS fields that must be present in responses
 */
export const REQUIRED_TLS_FIELDS = [
  "ja3",
  "ja3_hash",
  "ja4",
  "peetprint",
  "peetprint_hash",
];

/**
 * Assert that all required TLS fingerprint fields are present and non-empty
 * @param body - Parsed JSON response body
 */
export function assertTLSFieldsPresent(body: Record<string, unknown>): void {
  for (const field of REQUIRED_TLS_FIELDS) {
    const value = body[field];
    if (value === undefined || value === null || value === "") {
      throw new Error(`Missing or empty required TLS field: ${field}`);
    }
  }
}

/**
 * Helper to consume stream body and return Buffer
 */
export async function consumeBody(body: AsyncIterable<Buffer>): Promise<Buffer> {
  const chunks: Buffer[] = [];
  for await (const chunk of body) {
    chunks.push(chunk);
  }
  return Buffer.concat(chunks);
}

/**
 * Helper to consume stream body and parse as JSON
 */
export async function consumeBodyAsJson<T>(body: AsyncIterable<Buffer>): Promise<T> {
  const buffer = await consumeBody(body);
  return JSON.parse(buffer.toString("utf8")) as T;
}

/**
 * Check if buffer looks like JSON (starts with { or [)
 */
function looksLikeJson(buffer: Buffer): boolean {
  const firstChar = buffer.toString("utf8", 0, 1).trim();
  return firstChar === "{" || firstChar === "[";
}

/**
 * Decompress gzip data and parse as JSON
 * Falls back to parsing as plain JSON if decompression fails
 */
export async function decompressGzip<T>(body: AsyncIterable<Buffer>): Promise<T> {
  const buffer = await consumeBody(body);

  // If already looks like JSON, the server didn't compress
  if (looksLikeJson(buffer)) {
    return JSON.parse(buffer.toString("utf8")) as T;
  }

  try {
    const decompressed = await gunzip(buffer);
    return JSON.parse(decompressed.toString("utf8")) as T;
  } catch {
    // Maybe it wasn't compressed after all
    return JSON.parse(buffer.toString("utf8")) as T;
  }
}

/**
 * Decompress deflate data and parse as JSON
 * Note: deflate can be either raw deflate or zlib-wrapped, try both
 * Falls back to parsing as plain JSON if decompression fails
 */
export async function decompressDeflate<T>(body: AsyncIterable<Buffer>): Promise<T> {
  const buffer = await consumeBody(body);

  // If already looks like JSON, the server didn't compress
  if (looksLikeJson(buffer)) {
    return JSON.parse(buffer.toString("utf8")) as T;
  }

  // Try zlib-wrapped deflate first (more common)
  try {
    const decompressed = await inflate(buffer);
    return JSON.parse(decompressed.toString("utf8")) as T;
  } catch {
    // Try raw deflate
    try {
      const decompressed = await inflateRaw(buffer);
      return JSON.parse(decompressed.toString("utf8")) as T;
    } catch {
      // Maybe it wasn't compressed after all
      return JSON.parse(buffer.toString("utf8")) as T;
    }
  }
}

/**
 * Decompress brotli data and parse as JSON
 * Falls back to parsing as plain JSON if decompression fails
 */
export async function decompressBrotli<T>(body: AsyncIterable<Buffer>): Promise<T> {
  const buffer = await consumeBody(body);

  // If already looks like JSON, the server didn't compress
  if (looksLikeJson(buffer)) {
    return JSON.parse(buffer.toString("utf8")) as T;
  }

  try {
    const decompressed = await brotliDecompress(buffer);
    return JSON.parse(decompressed.toString("utf8")) as T;
  } catch {
    // Maybe it wasn't compressed after all
    return JSON.parse(buffer.toString("utf8")) as T;
  }
}

/**
 * Auto-detect compression type from Content-Encoding header and decompress
 */
export async function decompressAuto<T>(
  body: AsyncIterable<Buffer>,
  contentEncoding?: string
): Promise<T> {
  const encoding = contentEncoding?.toLowerCase() || "";

  if (encoding.includes("gzip")) {
    return decompressGzip<T>(body);
  } else if (encoding.includes("br")) {
    return decompressBrotli<T>(body);
  } else if (encoding.includes("deflate")) {
    return decompressDeflate<T>(body);
  } else {
    // No compression or unknown, try parsing as-is
    return consumeBodyAsJson<T>(body);
  }
}
