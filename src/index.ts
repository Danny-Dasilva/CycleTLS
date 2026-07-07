import { spawn, ChildProcessWithoutNullStreams, SpawnOptionsWithoutStdio } from "child_process";
import path from "path";
import fs from "fs";
import crypto from "crypto";
import { EventEmitter } from "events";
import WebSocket from "ws";
import * as http from "http";
import os from 'os';
import FormData from 'form-data';
import { Readable } from 'stream';
import { Blob } from 'buffer';

/**
 * Security: List of HTTP headers whose values should be redacted in debug logs.
 * Prevents leaking credentials, tokens, and session data.
 */
const SENSITIVE_HEADERS = new Set([
  'authorization', 'cookie', 'set-cookie', 'proxy-authorization',
  'x-api-key', 'x-auth-token'
]);

/**
 * Redact sensitive header values for safe logging.
 * Returns a copy of headers with sensitive values replaced by "[REDACTED]".
 */
function redactHeaders(headers: Record<string, string | string[]>): Record<string, string | string[]> {
  const redacted: Record<string, string | string[]> = {};
  for (const [key, value] of Object.entries(headers)) {
    if (SENSITIVE_HEADERS.has(key.toLowerCase())) {
      redacted[key] = '[REDACTED]';
    } else {
      redacted[key] = value;
    }
  }
  return redacted;
}

/**
 * Validate an executable path for security.
 * Prevents path traversal and symlink attacks.
 */
function validateExecutablePath(execPath: string): string {
  // Resolve to absolute path to prevent traversal
  const resolved = path.resolve(execPath);

  // Check the file exists
  if (!fs.existsSync(resolved)) {
    throw new Error(`Executable not found at path: ${resolved}`);
  }

  // Check it's a regular file (not a directory, symlink to unexpected location, etc.)
  const stats = fs.lstatSync(resolved);
  if (!stats.isFile()) {
    throw new Error(`Executable path is not a regular file: ${resolved}`);
  }

  // If it's a symlink, resolve and validate the target
  if (stats.isSymbolicLink()) {
    const realPath = fs.realpathSync(resolved);
    const realStats = fs.statSync(realPath);
    if (!realStats.isFile()) {
      throw new Error(`Executable symlink target is not a regular file: ${realPath}`);
    }
  }

  return resolved;
}

/**
 * Generate a cryptographically random authentication token.
 */
function generateAuthToken(): string {
  return crypto.randomBytes(32).toString('hex');
}

/**
 * Check if a process is still alive by sending signal 0.
 * Returns false if the process does not exist (ESRCH).
 */
function isProcessAlive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch (error) {
    return (error as NodeJS.ErrnoException).code !== "ESRCH";
  }
}

/**
 * Gracefully kill a child process with SIGTERM first, then SIGKILL after timeout.
 * On Windows, uses taskkill since SIGKILL/SIGTERM are not supported.
 * On Unix, kills the process group (negative PID) for detached processes.
 *
 * Security fixes applied:
 * - Issue #8: Sends SIGTERM first with a 5-second grace period before SIGKILL
 * - Issue #9: Validates PID is still alive before process group kill (TOCTOU mitigation)
 */
function forceKillProcess(
  child: ChildProcessWithoutNullStreams,
  killGroup: boolean = false
): void {
  if (process.platform === "win32") {
    // Windows: SIGKILL is not supported. Use taskkill for forceful termination
    if (child.pid !== undefined) {
      try {
        spawn("taskkill", ["/pid", String(child.pid), "/f", "/t"]);
      } catch {
        // Fallback: direct kill (Node translates to TerminateProcess on Windows)
        try { child.kill(); } catch { /* Process already dead */ }
      }
    } else {
      try { child.kill(); } catch { /* Process already dead */ }
    }
  } else {
    // Issue #8: Send SIGTERM first for graceful shutdown
    try {
      if (killGroup && child.pid !== undefined) {
        // Issue #9: Validate PID is still alive before group kill
        if (isProcessAlive(child.pid)) {
          process.kill(-child.pid, "SIGTERM");
        }
      } else {
        child.kill("SIGTERM");
      }
    } catch {
      // Process already dead - no need to continue
      return;
    }

    // Schedule SIGKILL after grace period if process hasn't exited
    const gracePeriodMs = 5000;
    setTimeout(() => {
      if (child.pid === undefined) return;

      // Check if process is still alive before sending SIGKILL
      if (!isProcessAlive(child.pid)) return;

      if (killGroup) {
        // Issue #9: Re-validate PID before group kill
        if (isProcessAlive(child.pid)) {
          try {
            process.kill(-child.pid, "SIGKILL");
          } catch (error) {
            if ((error as NodeJS.ErrnoException).code !== "ESRCH") {
              console.error("Error killing process group:", error);
            }
          }
        }
      } else {
        try { child.kill("SIGKILL"); } catch { /* Process already dead */ }
      }
    }, gracePeriodMs).unref(); // unref() so timer doesn't prevent Node from exiting
  }
}

/**
 * HTTP headers type - maps header names to string or string array values.
 * Some headers like Set-Cookie may have multiple values.
 */
export type HttpHeaders = Record<string, string | string[]>;

/**
 * Simple cookie object for basic name/value pairs.
 * For more control, use the full Cookie interface.
 */
export interface SimpleCookie {
  /** The name of the cookie */
  name: string;
  /** The value of the cookie */
  value: string;
}

// Internal response metadata from Go server
interface ResponseMetadata {
  statusCode: number;
  finalUrl: string;
  headers: HttpHeaders;
}

// Internal message types from Go server
interface GoResponseMessage {
  method: 'response';
  data: ResponseMetadata;
}

interface GoDataMessage {
  method: 'data';
  data: Buffer;
}

interface GoErrorMessage {
  method: 'error';
  data: {
    statusCode: number;
    message: string;
  };
}

interface GoEndMessage {
  method: 'end';
}

interface GoWebSocketOpenMessage {
  method: 'ws_open';
  data: {
    protocol?: string;
    extensions?: string;
  };
}

interface GoWebSocketMessage {
  method: 'ws_message';
  data: {
    messageType: number;
    data: Buffer;
  };
}

interface GoWebSocketCloseMessage {
  method: 'ws_close';
  data: {
    code?: number;
    reason?: string;
  };
}

interface GoWebSocketErrorMessage {
  method: 'ws_error';
  data: {
    statusCode: number;
    message: string;
  };
}

type GoMessage =
  | GoResponseMessage
  | GoDataMessage
  | GoErrorMessage
  | GoEndMessage
  | GoWebSocketOpenMessage
  | GoWebSocketMessage
  | GoWebSocketCloseMessage
  | GoWebSocketErrorMessage;

// Body read error during streaming
interface BodyReadError {
  statusCode: number;
  message: string;
}

// Request options for internal sendRequest
interface InternalRequestOptions extends CycleTLSRequestOptions {
  url: string;
  method: string;
  _connectionReuse?: string;
  _hostKey?: string;
}

// WebSocket send options (matching ws library)
interface WebSocketSendOptions {
  binary?: boolean;
  compress?: boolean;
  fin?: boolean;
  mask?: boolean;
}

// Error with optional status code
interface CycleTLSError extends Error {
  code?: number;
}

// Internal WebSocket message data union type
type WebSocketMessageData =
  | ResponseMetadata
  | { protocol?: string; extensions?: string }
  | { messageType: number; data: Buffer }
  | { code?: number; reason?: string }
  | { statusCode: number; message: string };

/**
 * Represents an HTTP cookie with all standard cookie attributes.
 * Used for both sending cookies with requests and receiving cookies from responses.
 *
 * @example
 * ```typescript
 * const cookie: Cookie = {
 *   name: 'session_id',
 *   value: 'abc123',
 *   domain: '.example.com',
 *   path: '/',
 *   secure: true,
 *   httpOnly: true,
 *   sameSite: 'Strict'
 * };
 * ```
 */
export interface Cookie {
  /** The name of the cookie */
  name: string;
  /** The value of the cookie */
  value: string;
  /** The URL path that must exist in the requested URL for the browser to send the Cookie header */
  path?: string;
  /** The domain for which the cookie is valid */
  domain?: string;
  /** The expiration date of the cookie as an ISO 8601 date string */
  expires?: string;
  /** The raw expires value as received from the server */
  rawExpires?: string;
  /** The maximum age of the cookie in seconds */
  maxAge?: number;
  /** Whether the cookie should only be sent over HTTPS */
  secure?: boolean;
  /** Whether the cookie is inaccessible to JavaScript */
  httpOnly?: boolean;
  /** Controls whether the cookie is sent with cross-site requests ('Strict', 'Lax', or 'None') */
  sameSite?: string;
  /** Any unparsed cookie attributes */
  unparsed?: string;
}

/**
 * Configuration options for request timeouts.
 */
export interface TimeoutOptions {
  /** The maximum time (in milliseconds) to wait for a request to complete */
  requestTimeout: number,
  /** The maximum time (in milliseconds) to wait for the server to acknowledge the request */
  acknowledgementTimeout?: number
}

/**
 * Configuration options for CycleTLS HTTP requests.
 * Includes TLS fingerprinting, proxy settings, headers, cookies, and more.
 *
 * @example
 * ```typescript
 * const options: CycleTLSRequestOptions = {
 *   headers: { 'Accept': 'application/json' },
 *   ja3: '771,4865-4867-4866-49195-49199...',
 *   userAgent: 'Mozilla/5.0...',
 *   proxy: 'http://user:pass@proxy.example.com:8080',
 *   timeout: 30
 * };
 * ```
 */
export interface CycleTLSRequestOptions {
  /**
   * HTTP headers to send with the request.
   *
   * Values may be a single string or an array of strings. Array values are sent
   * as repeated headers (multiple entries for the same field), and the exact
   * key casing you provide is preserved on the wire instead of being canonicalized.
   *
   * @example
   * headers: {
   *   "x-CUSTOM-case": ["one", "two"], // sent twice, casing preserved
   *   "Accept": "text/plain",
   * }
   */
  headers?: HttpHeaders;
  /**
   * Cookies to send with the request.
   * Can be an array of Cookie objects or a simple key-value object.
   */
  cookies?:
  Array<Cookie | SimpleCookie>
  | {
    [key: string]: string;
  };
  /** Request body for POST, PUT, PATCH requests */
  body?: string | URLSearchParams | FormData;

  /**
   * Response type (like Axios).
   * - 'json': Parse response as JSON (default)
   * - 'text': Return response as string
   * - 'arraybuffer': Return response as ArrayBuffer
   * - 'blob': Return response as Blob
   * - 'stream': Return response as readable stream
   */
  responseType?: 'json' | 'text' | 'arraybuffer' | 'blob' | 'stream';

  /**
   * JA3 fingerprint string for TLS fingerprinting.
   * Format: TLSVersion,Ciphers,Extensions,EllipticCurves,EllipticCurvePointFormats
   */
  ja3?: string;
  /**
   * JA4 raw format (JA4R) with explicit cipher/extension values.
   * Pass raw JA4 (JA4R) values. The JA4 hash is not accepted for configuration.
   */
  ja4r?: string;
  /** HTTP/2 fingerprint for protocol-level fingerprinting */
  http2Fingerprint?: string;
  /** QUIC/HTTP3 fingerprint for protocol-level fingerprinting */
  quicFingerprint?: string;
  /** Disable GREASE (Generate Random Extensions And Sustain Extensibility) for exact JA4 matching */
  disableGrease?: boolean;

  /** User-Agent header value */
  userAgent?: string;

  /** Overrides TLS Server Name Indication (SNI). Useful for accessing sites via IP with correct TLS handshake */
  serverName?: string;
  /**
   * Proxy URL in format: protocol://user:pass@host:port
   * Supports HTTP, HTTPS, SOCKS4, and SOCKS5 proxies
   */
  proxy?: string;
  /** Request timeout in seconds (default: 30) */
  timeout?: number;
  /** Disable automatic following of redirects */
  disableRedirect?: boolean;
  /** Order in which headers should be sent (for fingerprinting) */
  headerOrder?: string[];
  /** Send headers in the exact order they are provided */
  orderAsProvided?: boolean;
  /** Skip TLS certificate verification (use with caution) */
  insecureSkipVerify?: boolean;
  /**
   * Skip TLS certificate verification for proxy connections.
   * Defaults to true for backward compatibility since proxies commonly use self-signed certs.
   * Set to false to require valid proxy certificates.
   */
  proxyInsecureSkipVerify?: boolean;
  /** Controls whether connections are pooled and reused between requests (default: true) */
  enableConnectionReuse?: boolean;

  /** Force use of HTTP/1.1 instead of HTTP/2 */
  forceHTTP1?: boolean;
  /** Force use of HTTP/3 (QUIC) */
  forceHTTP3?: boolean;
  /** Protocol to use: "http1", "http2", "http3", "websocket", "sse" */
  protocol?: string;
}

/**
 * Response data can be various types depending on responseType option.
 */
export type ResponseData = unknown | string | ArrayBuffer | Blob | Readable | Buffer;

/**
 * Response object returned from CycleTLS HTTP requests.
 * Provides both direct data access (Axios-style) and method-based parsing (Fetch-style).
 *
 * @example
 * ```typescript
 * const response = await cycleTLS.get('https://api.example.com/data', {});
 *
 * // Direct data access (based on responseType)
 * console.log(response.status);  // 200
 * console.log(response.data);    // parsed JSON by default
 *
 * // Or use response methods
 * const json = await response.json();
 * const text = await response.text();
 * ```
 */
export interface CycleTLSResponse {
  /** HTTP status code (e.g., 200, 404, 500) */
  status: number;
  /** Response headers as key-value pairs */
  headers: HttpHeaders;
  /** Response body, parsed according to responseType option (default: JSON) */
  data: ResponseData;
  /** The final URL after any redirects */
  finalUrl: string;
  /**
   * Parse the response body as JSON.
   * @returns Promise resolving to the parsed JSON data
   * @throws Error if the response body is not valid JSON
   */
  json<T = unknown>(): Promise<T>;
  /**
   * Get the response body as a string.
   * @returns Promise resolving to the response text
   */
  text(): Promise<string>;
  /**
   * Get the response body as an ArrayBuffer.
   * @returns Promise resolving to the raw binary data
   */
  arrayBuffer(): Promise<ArrayBuffer>;
  /**
   * Get the response body as a Blob.
   * @returns Promise resolving to a Blob with the appropriate content type
   */
  blob(): Promise<Blob>;
}

/**
 * Represents a WebSocket message received from the server.
 */
export interface WebSocketMessage {
  /** The type of message: 'text', 'binary', 'close', 'ping', or 'pong' */
  type: 'text' | 'binary' | 'close' | 'ping' | 'pong';
  /** The message data as a string (for text) or Buffer (for binary) */
  data: string | Buffer;
}

/**
 * Response object for WebSocket connections.
 * Extends CycleTLSResponse with WebSocket-specific methods.
 */
export interface CycleTLSWebSocketResponse extends CycleTLSResponse {
  /** Send a message through the WebSocket connection */
  send(data: string | Buffer, isBinary?: boolean): Promise<void>;
  /** Close the WebSocket connection */
  close(code?: number, reason?: string): Promise<void>;
  /** Register a callback for incoming messages */
  onMessage(callback: (message: WebSocketMessage) => void): void;
  /** Register a callback for connection close events */
  onClose(callback: (code: number, reason: string) => void): void;
  /** Register a callback for error events */
  onError(callback: (error: Error) => void): void;
}

/**
 * Represents a Server-Sent Event (SSE) message.
 */
export interface SSEEvent {
  /** Optional event ID for resuming connections */
  id?: string;
  /** Event type/name */
  event?: string;
  /** Event data payload */
  data: string;
  /** Reconnection time in milliseconds */
  retry?: number;
}

/**
 * Response object for Server-Sent Events (SSE) connections.
 * Extends CycleTLSResponse with SSE-specific methods.
 */
export interface CycleTLSSSEResponse extends CycleTLSResponse {
  /** Async iterator for consuming SSE events */
  events(): AsyncIterableIterator<SSEEvent>;
  /** Register a callback for incoming events */
  onEvent(callback: (event: SSEEvent) => void): void;
  /** Register a callback for error events */
  onError(callback: (error: Error) => void): void;
  /** Close the SSE connection */
  close(): Promise<void>;
}

// Global instance manager for tracking shared instances by port
class InstanceManager {
  private static instance: InstanceManager;
  private sharedInstances: Map<number, SharedInstance> = new Map();
  private initializingPromises: Map<number, Promise<SharedInstance>> = new Map();

  static getInstance(): InstanceManager {
    if (!InstanceManager.instance) {
      InstanceManager.instance = new InstanceManager();
    }
    return InstanceManager.instance;
  }

  async getOrCreateSharedInstance(
    port: number,
    debug: boolean,
    timeout: number,
    executablePath?: string
  ): Promise<SharedInstance> {
    // Check if already initialized
    const existing = this.sharedInstances.get(port);
    if (existing) {
      return existing;
    }

    // Check if initialization is in progress
    const pending = this.initializingPromises.get(port);
    if (pending) {
      return pending;
    }

    // Start initialization - IMPORTANT: The promise is created and stored in
    // initializingPromises synchronously (before any await) to prevent race
    // conditions. Since JS is single-threaded for synchronous code, no other
    // call can interleave between the check above and the set below.
    const initPromise = (async () => {
      try {
        const sharedInstance = new SharedInstance(port, debug, timeout, executablePath);
        await sharedInstance.initialize();
        this.sharedInstances.set(port, sharedInstance);
        return sharedInstance;
      } finally {
        this.initializingPromises.delete(port);
      }
    })();

    this.initializingPromises.set(port, initPromise);
    return initPromise;
  }

  async removeSharedInstance(port: number): Promise<void> {
    const sharedInstance = this.sharedInstances.get(port);
    if (sharedInstance) {
      await sharedInstance.cleanup();
      this.sharedInstances.delete(port);
    }
  }

  async cleanup(): Promise<void> {
    // Wait for any pending initializations to complete (or fail) before cleanup
    // to avoid leaving half-initialized instances as zombies
    if (this.initializingPromises.size > 0) {
      const pendingPromises = Array.from(this.initializingPromises.values());
      await Promise.allSettled(pendingPromises);
    }

    const cleanupPromises = Array.from(this.sharedInstances.values()).map(instance => instance.cleanup());
    await Promise.all(cleanupPromises);
    this.sharedInstances.clear();
    this.initializingPromises.clear();
  }

  // Testing accessors - allows tests to verify internal state
  /** @internal For testing only - check if a port has a pending initialization promise */
  _hasInitializingPromise(port: number): boolean {
    return this.initializingPromises.has(port);
  }

  /** @internal For testing only - get the count of initializing promises */
  _getInitializingPromisesCount(): number {
    return this.initializingPromises.size;
  }

  /** @internal For testing only - reset the singleton instance */
  static _resetInstance(): void {
    InstanceManager.instance = undefined as unknown as InstanceManager;
  }
}

// Manages one Go server process and multiple TypeScript client connections
class SharedInstance extends EventEmitter {
  private child: ChildProcessWithoutNullStreams | null = null;
  private server: WebSocket | null = null;
  private clients: Map<string, CycleTLSClientImpl> = new Map();
  private port: number;
  private debug: boolean;
  private timeout: number;
  private executablePath?: string;
  private isHost: boolean = false;
  private connectionTimeout: NodeJS.Timeout | null = null;
  private failedInitialization: boolean = false;
  private isShuttingDown: boolean = false;
  private httpServer: http.Server | null = null;
  private initializationReject: ((reason: string) => void) | null = null;
  private authToken: string = ''; // Issue #4: WebSocket auth token

  constructor(port: number, debug: boolean, timeout: number, executablePath?: string) {
    super();
    this.port = port;
    this.debug = debug;
    this.timeout = timeout;
    this.executablePath = executablePath;
  }

  private rejectInitialization(reason: string): void {
    this.failedInitialization = true;

    // Clear connection timeout to prevent it from firing on an already-rejected promise
    if (this.connectionTimeout) {
      clearTimeout(this.connectionTimeout);
      this.connectionTimeout = null;
    }

    // Kill child process if it exists to prevent zombie processes
    if (this.child) {
      forceKillProcess(this.child, true);
      this.child = null;
    }

    if (this.initializationReject) {
      this.initializationReject(reason);
      this.initializationReject = null;
    }
  }

  async initialize(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.initializationReject = reject;
      this.checkSpawnedInstance(resolve, reject);
    });
  }

  private checkSpawnedInstance(resolve: () => void, reject: (reason: string) => void): void {
    this.httpServer = http.createServer();

    const onListening = () => {
      // Remove the error listener since we succeeded
      this.httpServer?.off('error', onError);
      // Close the HTTP server immediately after it starts listening
      this.httpServer!.close(() => {
        // Ensure all listeners are removed and server is nulled
        this.httpServer!.removeAllListeners();
        this.httpServer = null;
        this.spawnServer();
        // Only proceed with client creation if spawn didn't fail
        if (!this.failedInitialization && this.initializationReject) {
          this.isHost = true;
          this.createClient(resolve, reject);
        }
      });
    };

    const onError = (err: Error) => {
      // Remove the listening listener since we failed
      this.httpServer?.off('listening', onListening);

      // Ensure the HTTP server is closed if an error occurs.
      // IMPORTANT: createClient must be called INSIDE the close callback
      // to avoid a race where the server still holds the port.

      // Kill any spawned child process to prevent zombie processes
      // when HTTP server fails after process was spawned
      if (this.child) {
        forceKillProcess(this.child, true);
        this.child = null;
      }

      if (this.httpServer) {
        try {
          this.httpServer.close(() => {
            this.httpServer?.removeAllListeners();
            this.httpServer = null;
            this.createClient(resolve, reject);
            this.isHost = false;
          });
        } catch (e) {
          console.error("Error closing server on error:", e);
          this.httpServer = null;
          this.createClient(resolve, reject);
          this.isHost = false;
        }
      } else {
        this.createClient(resolve, reject);
        this.isHost = false;
      }
    };

    this.httpServer.once('listening', onListening);
    this.httpServer.once('error', onError);

    // Start listening last so that the above listeners are in place
    this.httpServer.listen(this.port);
  }

  private spawnServer(): void {
    const PLATFORM_BINARIES: { [platform: string]: { [arch: string]: string } } = {
      "win32": { "x64": "index.exe" },
      "linux": { "arm": "index-arm", "arm64": "index-arm64", "x64": "index" },
      "darwin": { "x64": "index-mac", "arm": "index-mac-arm", "arm64": "index-mac-arm64" },
      "freebsd": { "x64": "index-freebsd" }
    };

    const executableFilename = PLATFORM_BINARIES[process.platform]?.[os.arch()];
    if (!executableFilename) {
      this.rejectInitialization(`Unsupported architecture: ${process.platform}-${os.arch()}`);
      return;
    }

    this.handleSpawn(executableFilename);
  }

  private handleSpawn(fileName: string): void {
    try {
      // Clean up old child process listeners before respawning to prevent leaks
      if (this.child) {
        this.child.stdout.removeAllListeners();
        this.child.stderr.removeAllListeners();
        this.child.removeAllListeners();
      }

      // Determine the executable path
      let execPath: string;

      if (this.executablePath) {
        // Issue #1: Validate user-provided executable path
        execPath = validateExecutablePath(this.executablePath);
      } else {
        // Otherwise, construct path relative to __dirname
        execPath = path.join(__dirname, fileName);
      }

      // Remove quotes as they're not needed and can cause issues on Windows
      execPath = execPath.replace(/"/g, '');

      // Verify file exists before attempting to spawn
      if (!fs.existsSync(execPath)) {
        throw new Error(`Executable not found at path: ${execPath}`);
      }

      // Issue #4: Generate auth token for WebSocket authentication
      this.authToken = generateAuthToken();

      const spawnOptions: SpawnOptionsWithoutStdio = {
        env: {
          ...process.env,
          WS_PORT: this.port.toString(),
          WS_AUTH_TOKEN: this.authToken, // Issue #4: Pass token to child process
        },
        shell: process.platform !== "win32", // false for Windows, true for others
        windowsHide: true,
        detached: process.platform !== "win32",
        // Add cwd option to ensure proper working directory
        cwd: path.dirname(execPath)
      };

      this.child = spawn(execPath, [], spawnOptions);

      this.child.stdout.on("data", (stdout) => {
        // Issue #5: Don't log stdout if it contains auth token
        const output = stdout.toString();
        if (!output.includes('WS_AUTH_TOKEN=')) {
          console.log(output);
        }
      });

      this.child.stderr.on("data", (stderr) => {
        const errorMessage = stderr.toString();
        if (errorMessage.includes("Request_Id_On_The_Left")) {
          const [requestId, error] = errorMessage.split("Request_Id_On_The_Left");
          // Handle request-specific error
        } else {
          // Only restart on truly fatal errors
          const fatalErrorPattern = /panic|fatal error|runtime error|address already in use/i;
          
          if (fatalErrorPattern.test(errorMessage)) {
            // Critical error - restart the process
            this.cleanExit(
              `Fatal error detected (please open an issue https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose) -> ${errorMessage}`
            );
            this.handleSpawn(fileName);
          } else {
            // Non-fatal error - just log it for debugging
            if (this.debug) {
              console.log(`[DEBUG] stderr: ${errorMessage}`);
            }
            // Don't restart for non-fatal errors like read timeouts
          }
        }
      });

      // Add error handler for spawn errors
      this.child.on('error', (error: NodeJS.ErrnoException) => {
        console.error(`Failed to start subprocess: ${error.message}`);
        if (error.code === 'ENOENT') {
          console.error(`Executable not found at: ${execPath}`);
          console.error('Please ensure the executable exists and has correct permissions');
        }
        this.rejectInitialization(error.message);
      });

    } catch (error) {
      const err = error instanceof Error ? error : new Error(String(error));
      console.error(`Error in handleSpawn: ${err.message}`);
      this.rejectInitialization(err.message);
    }
  }

  private createClient(resolve: () => void, reject: (reason: string) => void): void {
    const attemptConnection = () => {
      // Use ?v=1 to request legacy JSON protocol (V2 flow-control is now default)
      // Issue #4: Include auth token in WebSocket connection for authentication
      const tokenParam = this.authToken ? `&token=${this.authToken}` : '';
      const server = new WebSocket(`ws://localhost:${this.port}?v=1${tokenParam}`);

      server.on("open", () => {
        // When connected, clear the connection timeout if it exists.
        if (this.connectionTimeout) {
          clearTimeout(this.connectionTimeout);
          this.connectionTimeout = null;
        }
        this.server = server;

        this.server.on("message", (data: Buffer) => {
          const packetBuffer = new PacketBuffer(data);
          const requestID = packetBuffer.readString();
          const method = packetBuffer.readString();

          // Route message to the appropriate client based on request ID
          const clientId = this.extractClientIdFromRequestId(requestID);
          const client = this.clients.get(clientId);

          if (client) {
            if (method === "response") {
              const statusCode = packetBuffer.readU16();
              const headers = [];
              const finalUrl = packetBuffer.readString();
              const headersLength = packetBuffer.readU16();

              for (let i = 0; i < headersLength; i++) {
                const headerValues = [];
                const headerName = packetBuffer.readString();
                const valuesLength = packetBuffer.readU16();

                for (let j = 0; j < valuesLength; j++) {
                  headerValues.push(packetBuffer.readString());
                }

                headers.push([headerName, headerValues]);
              }

              client.emit(requestID, {
                method,
                data: {
                  statusCode,
                  finalUrl,
                  headers: Object.fromEntries(headers),
                },
              });
            }

            if (method === "data") {
              client.emit(requestID, {
                method,
                data: packetBuffer.readBytes(false),
              });
            }

            if (method === "error") {
              const statusCode = packetBuffer.readU16();
              const errorMessage = packetBuffer.readString();
              client.emit(requestID, {
                method,
                data: {
                  statusCode,
                  message: errorMessage,
                },
              });
            }

            if (method === "end") {
              client.emit(requestID, { method });
            }

            // WebSocket-specific message types
            if (method === "ws_open") {
              const data = packetBuffer.readBytes(false);
              const parsedData = JSON.parse(data.toString());
              client.emit(requestID, {
                method,
                data: parsedData,
              });
            }

            if (method === "ws_message") {
              const messageType = packetBuffer.readU8();
              const messageData = packetBuffer.readBytes(false);
              client.emit(requestID, {
                method,
                data: {
                  messageType,
                  data: messageData,
                },
              });
            }

            if (method === "ws_close") {
              const data = packetBuffer.readBytes(false);
              const parsedData = JSON.parse(data.toString());
              client.emit(requestID, {
                method,
                data: parsedData,
              });
            }

            if (method === "ws_error") {
              const statusCode = packetBuffer.readU16();
              const errorMessage = packetBuffer.readString();
              client.emit(requestID, {
                method,
                data: {
                  statusCode,
                  message: errorMessage,
                },
              });
            }
          }
        });

        resolve();
      });

      server.on("error", (err) => {
        // On error, remove the listeners and try again after a short delay.
        server.removeAllListeners();
        setTimeout(() => {
          if (this.failedInitialization) {
            return;
          }
          attemptConnection();
        }, 100);
      });
    };

    attemptConnection();

    // Store the timeout so that we can clear it when the connection succeeds.
    this.connectionTimeout = setTimeout(() => {
      this.failedInitialization = true;
      reject(`Could not connect to the CycleTLS instance within ${this.timeout}ms`);
    }, this.timeout);
  }

  private extractClientIdFromRequestId(requestId: string): string {
    // Request ID format: ${clientId}#${url}#${timestamp}-${random}
    return requestId.split('#')[0];
  }

  addClient(clientId: string, client: CycleTLSClientImpl): void {
    this.clients.set(clientId, client);
  }

  removeClient(clientId: string): void {
    this.clients.delete(clientId);

    // If no more clients, cleanup the shared instance
    // But check that no new client is currently initializing to avoid
    // race condition where cleanup runs while a new client is connecting
    if (this.clients.size === 0) {
      const manager = InstanceManager.getInstance();
      if (!manager._hasInitializingPromise(this.port)) {
        manager.removeSharedInstance(this.port);
      }
    }
  }

  async sendRequest(requestId: string, options: InternalRequestOptions): Promise<void> {
    // Check if options.body is URLSearchParams and convert to string
    if (options.body instanceof URLSearchParams) {
      options.body = options.body.toString();
    }
    // Check if options.body is FormData and convert to multipart format
    if (options.body instanceof FormData) {
      // Get headers with boundary from FormData
      const formHeaders = options.body.getHeaders();
      options.headers = { ...options.headers, ...formHeaders };
      
      // Convert FormData to string while preserving multipart format
      const formDataString = await new Promise<string>((resolve, reject) => {
        const chunks: Buffer[] = [];
        const form = options.body as unknown as Readable;
        let settled = false;

        const onData = (chunk: Buffer | string) => {
          chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
        };

        const onEnd = () => {
          if (settled) return;
          settled = true;
          cleanup();
          const result = Buffer.concat(chunks).toString('utf8');
          resolve(result);
        };

        const onError = (err: Error) => {
          if (settled) return;
          settled = true;
          cleanup();
          reject(err);
        };

        const onClose = () => {
          // Stream destroyed externally before 'end' - reject to prevent hanging
          if (!settled) {
            settled = true;
            cleanup();
            reject(new Error('FormData stream was destroyed before completion'));
          }
        };

        const cleanup = () => {
          form.off('data', onData);
          form.off('end', onEnd);
          form.off('error', onError);
          form.off('close', onClose);
        };

        form.on('data', onData);
        form.on('end', onEnd);
        form.on('error', onError);
        form.on('close', onClose);

        // Force reading the stream
        form.resume();
      });
      
      options.body = formDataString;
    }

    if (this.server) {
      this.server.send(JSON.stringify({ requestId, options }));
    } else {
      throw new Error('WebSocket server not connected');
    }
  }

  async cancelRequest(requestId: string): Promise<void> {
    if (this.server) {
      this.server.send(JSON.stringify({ action: "cancel", requestId }));
    }
  }

  async sendWebSocketCommand(requestId: string, action: string, data?: Record<string, unknown>): Promise<void> {
    if (this.server) {
      this.server.send(JSON.stringify({
        action,
        requestId,
        ...data
      }));
    } else {
      throw new Error('WebSocket server not connected');
    }
  }

  private async cleanExit(message?: string | Error): Promise<void> {
    if (message) console.log(message);
    if (this.isShuttingDown) return;

    this.isShuttingDown = true;

    if (this.child) {
      forceKillProcess(this.child, true);
    }
  }

  async cleanup(): Promise<void> {
    // Clear any pending timeouts
    if (this.connectionTimeout) {
      clearTimeout(this.connectionTimeout);
      this.connectionTimeout = null;
    }

    // Close HTTP server if it exists
    if (this.httpServer) {
      try {
        this.httpServer.close();
        this.httpServer.removeAllListeners();
      } catch (error) {
        console.error("Error closing HTTP server:", error);
      }
      this.httpServer = null;
    }

    // Kill child process forcefully
    if (this.isHost && this.child) {
      forceKillProcess(this.child, true);
      this.child = null;
    }

    // Force close the WebSocket immediately
    if (this.server) {
      try {
        this.server.removeAllListeners();
        this.server.terminate();
        this.server = null;
      } catch (error) {
        console.error("Error closing WebSocket:", error);
      }
    }

    // Notify all clients that the shared instance is shutting down
    for (const client of this.clients.values()) {
      client.emit('sharedInstanceClosed');
    }
    this.clients.clear();
    
    this.removeAllListeners();
  }
}

// CycleTLSWebSocket class implementing EventEmitter pattern (matching ws library)
class CycleTLSWebSocket extends EventEmitter {
  private sharedInstance: SharedInstance;
  private requestId: string;
  private _url: string;
  private _readyState: number = 0; // 0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED
  private _protocol: string = '';
  private _extensions: string = '';
  private _bufferedAmount: number = 0;
  private _binaryType: 'nodebuffer' | 'arraybuffer' | 'fragments' = 'nodebuffer';
  private responseHeaders: HttpHeaders = {};
  private responseStatus: number = 0;

  constructor(
    sharedInstance: SharedInstance,
    requestId: string,
    url: string
  ) {
    super();
    this.sharedInstance = sharedInstance;
    this.requestId = requestId;
    this._url = url;
  }

  // Properties matching ws library
  get url(): string {
    return this._url;
  }

  get readyState(): number {
    return this._readyState;
  }

  get protocol(): string {
    return this._protocol;
  }

  get extensions(): string {
    return this._extensions;
  }

  get bufferedAmount(): number {
    return this._bufferedAmount;
  }

  get binaryType(): 'nodebuffer' | 'arraybuffer' | 'fragments' {
    return this._binaryType;
  }

  set binaryType(value: 'nodebuffer' | 'arraybuffer' | 'fragments') {
    this._binaryType = value;
  }

  // Send a message (matching ws library signature)
  send(data: string | Buffer | ArrayBuffer, callback?: (err?: Error) => void): void;
  send(data: string | Buffer | ArrayBuffer, options?: WebSocketSendOptions, callback?: (err?: Error) => void): void;
  send(
    data: string | Buffer | ArrayBuffer,
    optionsOrCallback?: WebSocketSendOptions | ((err?: Error) => void),
    callback?: (err?: Error) => void
  ): void {
    let options: WebSocketSendOptions = {};
    let cb: ((err?: Error) => void) | undefined;

    // Parse arguments
    if (typeof optionsOrCallback === 'function') {
      cb = optionsOrCallback;
    } else if (typeof optionsOrCallback === 'object') {
      options = optionsOrCallback;
      cb = callback;
    }

    if (this._readyState !== 1) {
      const err = new Error('WebSocket is not open: readyState ' + this._readyState);
      if (cb) {
        cb(err);
        return;
      }
      throw err;
    }

    try {
      let messageData: string;
      let isBinary = false;

      if (Buffer.isBuffer(data)) {
        messageData = data.toString('base64');
        isBinary = true;
      } else if (data instanceof ArrayBuffer) {
        messageData = Buffer.from(data).toString('base64');
        isBinary = true;
      } else {
        messageData = String(data);
      }

      if (options.binary !== undefined) {
        isBinary = options.binary;
      }

      this.sharedInstance.sendWebSocketCommand(this.requestId, 'ws_send', {
        data: messageData,
        isBinary
      }).then(() => {
        if (cb) cb();
      }).catch((err) => {
        if (cb) {
          cb(err);
        } else if (this.listenerCount('error') > 0) {
          this.emit('error', err);
        } else {
          // No callback and no error listeners - schedule throw to avoid
          // unhandled promise rejection silently swallowing the error
          process.nextTick(() => {
            throw err;
          });
        }
      });

    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));
      if (cb) {
        cb(error);
      } else {
        this.emit('error', error);
      }
    }
  }

  // Close the connection (matching ws library signature)
  close(code?: number, reason?: string): void {
    if (this._readyState === 2 || this._readyState === 3) {
      return; // Already closing or closed
    }

    this._readyState = 2; // CLOSING

    this.sharedInstance.sendWebSocketCommand(this.requestId, 'ws_close', {
      code: code || 1000,
      reason: reason || ''
    }).catch((err) => {
      if (this.listenerCount('error') > 0) {
        this.emit('error', err);
      }
      // If no error listeners, swallow close errors gracefully
    });
  }

  // Ping the connection (matching ws library signature)
  ping(data?: Buffer | string, mask?: boolean, callback?: (err?: Error) => void): void {
    if (this._readyState !== 1) {
      const err = new Error('WebSocket is not open: readyState ' + this._readyState);
      if (callback) {
        callback(err);
        return;
      }
      throw err;
    }

    const pingData = data ? (Buffer.isBuffer(data) ? data.toString('base64') : data) : '';

    this.sharedInstance.sendWebSocketCommand(this.requestId, 'ws_ping', {
      data: pingData
    }).then(() => {
      if (callback) callback();
    }).catch((err) => {
      if (callback) callback(err);
      else this.emit('error', err);
    });
  }

  // Pong the connection (matching ws library signature)
  pong(data?: Buffer | string, mask?: boolean, callback?: (err?: Error) => void): void {
    if (this._readyState !== 1) {
      const err = new Error('WebSocket is not open: readyState ' + this._readyState);
      if (callback) {
        callback(err);
        return;
      }
      throw err;
    }

    const pongData = data ? (Buffer.isBuffer(data) ? data.toString('base64') : data) : '';

    this.sharedInstance.sendWebSocketCommand(this.requestId, 'ws_pong', {
      data: pongData
    }).then(() => {
      if (callback) callback();
    }).catch((err) => {
      if (callback) callback(err);
      else this.emit('error', err);
    });
  }

  // Terminate the connection immediately (matching ws library)
  terminate(): void {
    this._readyState = 3; // CLOSED
    this.removeAllListeners();
  }

  // Internal method to handle incoming messages
  _handleMessage(method: string, data: WebSocketMessageData): void {
    switch (method) {
      case 'response': {
        const responseData = data as ResponseMetadata;
        this.responseStatus = responseData.statusCode;
        this.responseHeaders = responseData.headers;
        break;
      }
      case 'ws_open': {
        const openData = data as { protocol?: string; extensions?: string };
        this._readyState = 1; // OPEN
        this._protocol = openData.protocol || '';
        this._extensions = openData.extensions || '';
        this.emit('open');
        break;
      }
      case 'ws_message': {
        const msgData = data as { messageType: number; data: Buffer };
        this.emit('message', msgData.data, msgData.messageType === 2);
        break;
      }
      case 'ws_close': {
        const closeData = data as { code?: number; reason?: string };
        this._readyState = 3; // CLOSED
        this.emit('close', closeData.code || 1000, closeData.reason || '');
        break;
      }
      case 'ws_error': {
        const errorData = data as { statusCode: number; message: string };
        const error: CycleTLSError = new Error(errorData.message || 'WebSocket error');
        error.code = errorData.statusCode;
        this.emit('error', error);
        break;
      }
      case 'error': {
        const errorData = data as { statusCode: number; message: string };
        this._readyState = 3; // CLOSED
        const error: CycleTLSError = new Error(errorData.message || 'Connection error');
        error.code = errorData.statusCode;
        this.emit('error', error);
        break;
      }
      case 'end': {
        if (this._readyState !== 3) {
          this._readyState = 3; // CLOSED
          this.emit('close', 1000, 'Connection ended');
        }
        break;
      }
    }
  }

  // Response-like properties for backward compatibility
  get status(): number {
    return this.responseStatus;
  }

  get headers(): HttpHeaders {
    return this.responseHeaders;
  }
}

// Default JA3 fingerprint for Chrome
const DEFAULT_JA3 = "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";

// Default User-Agent string
const DEFAULT_USER_AGENT = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36";

// Apply default fingerprinting options to request options
function applyDefaultOptions(options: CycleTLSRequestOptions): void {
  // Set default fingerprinting options - prefer JA3 if multiple options are provided
  if (!options.ja3 && !options.ja4r && !options.http2Fingerprint && !options.quicFingerprint) {
    options.ja3 = DEFAULT_JA3;
  }

  // Set default user agent
  if (!options.userAgent) {
    options.userAgent = DEFAULT_USER_AGENT;
  }

  // Convert simple cookies to Cookie array
  const cookies = options.cookies;
  if (typeof cookies === "object" && !Array.isArray(cookies) && cookies !== null) {
    const tempArr: SimpleCookie[] = [];
    for (const [key, value] of Object.entries(cookies)) {
      tempArr.push({ name: key, value: value });
    }
    options.cookies = tempArr;
  }
}

// Represents an individual client connection to a SharedInstance
class CycleTLSClientImpl extends EventEmitter {
  private sharedInstance: SharedInstance;
  private clientId: string;
  private connectionsByHost: Map<string, boolean> = new Map();

  constructor(sharedInstance: SharedInstance) {
    super();
    this.sharedInstance = sharedInstance;
    this.clientId = `client-${Date.now()}-${Math.floor(10000 * Math.random())}`;

    // Register this client with the shared instance
    this.sharedInstance.addClient(this.clientId, this);

    // Listen for shared instance closure
    this.on('sharedInstanceClosed', () => {
      this.removeAllListeners();
    });
  }

  async request(
    url: string,
    options: CycleTLSRequestOptions,
    method: "head" | "get" | "post" | "put" | "delete" | "trace" | "options" | "connect" | "patch" = "get"
  ): Promise<CycleTLSResponse> {
    // Track connection reuse by parsing the URL's host
    const urlObj = new URL(url);
    const hostKey = urlObj.host;

    const requestId = `${this.clientId}#${url}#${Date.now()}-${Math.floor(1000 * Math.random())}`;

    // Ensure options object exists
    options ??= {};

    // Apply default fingerprinting and user agent options
    applyDefaultOptions(options);

    // Set default request options
    if (!options.body) options.body = "";
    if (!options.proxy) options.proxy = "";
    if (!options.insecureSkipVerify) options.insecureSkipVerify = false;
    if (options.enableConnectionReuse === undefined) options.enableConnectionReuse = true;
    if (!options.forceHTTP1) options.forceHTTP1 = false;
    if (!options.forceHTTP3) options.forceHTTP3 = false;
    if (!options.responseType) options.responseType = 'json';
    if (!options.protocol) options.protocol = "";

    // Track if we've connected to this host before for connection reuse
    const hasExistingConnection = this.connectionsByHost.has(hostKey);
    
    // Set the connection as tracked for this host
    this.connectionsByHost.set(hostKey, true);
    
    // Send request through shared instance
    await this.sharedInstance.sendRequest(requestId, {
      url,
      ...options,
      method,
      // Add metadata about connection reuse (will be ignored by Go if not implemented)
      _connectionReuse: hasExistingConnection ? "reuse" : "new",
      _hostKey: hostKey,
    });

    return new Promise((resolveRequest, rejectRequest) => {
      let responseMetadata: ResponseMetadata | null = null;
      let timeoutId: NodeJS.Timeout | null = null;

      // Default timeout of 30 seconds, configurable via options.timeout (in seconds)
      const timeoutMs = options.timeout ? options.timeout * 1000 : 30000;

      const cleanupTimeout = () => {
        if (timeoutId) {
          clearTimeout(timeoutId);
          timeoutId = null;
        }
      };

      // Set up client-side timeout to prevent hanging promises
      timeoutId = setTimeout(() => {
        cleanupTimeout();
        this.off(requestId, handleMessage);
        rejectRequest(new Error(`Request timeout after ${timeoutMs}ms`));
      }, timeoutMs);

      const handleMessage = async (response: GoMessage) => {
        if (response.method === "error") {
          // Handle error before or during body read
          // If we already have response metadata (headers sent successfully but body read failed),
          // return the error with empty headers
          cleanupTimeout();
          const errorResponse = {
            status: response.data.statusCode,
            headers: responseMetadata ? responseMetadata.headers : {},
            finalUrl: responseMetadata ? responseMetadata.finalUrl : url,
            data: response.data.message,
            json: async <T = unknown>(): Promise<T> => Promise.resolve({} as T),
            text: async () => Promise.resolve(response.data.message),
            arrayBuffer: async () => Promise.resolve(new ArrayBuffer(0)),
            blob: async () => Promise.resolve(new Blob([response.data.message], { type: 'text/plain' }))
          };
          this.off(requestId, handleMessage);
          resolveRequest(errorResponse);
        } else if (response.method === "response") {
          // Store response metadata but don't resolve yet
          // Clear timeout when we receive response headers (server is responding)
          cleanupTimeout();
          responseMetadata = response.data;
        } else if (response.method === "data" || response.method === "end") {
          // Now we have response metadata, set up stream handling
          if (!responseMetadata) return;

          // Remove the message handler and set up stream handling
          this.off(requestId, handleMessage);

          const stream = new Readable({ read() { } });

          let bodyReadError: BodyReadError | null = null;

          const cleanupStreamListeners = () => {
            stream.off("close", handleClose);
            this.off(requestId, handleData);
          };

          const handleClose = () => {
            // Stream was closed externally (cancel/timeout/destroy)
            cleanupStreamListeners();
            this.sharedInstance.cancelRequest(requestId);
          };

          const handleData = (dataResponse: GoDataMessage | GoErrorMessage | GoEndMessage) => {
            if (dataResponse.method === "data") {
              stream.push(Buffer.from(dataResponse.data));
            } else if (dataResponse.method === "error") {
              // Handle error that occurred during body read - store it and close the stream
              bodyReadError = {
                statusCode: dataResponse.data.statusCode,
                message: dataResponse.data.message
              };
              stream.push(null); // Close stream gracefully
              cleanupStreamListeners();
            } else if (dataResponse.method === "end") {
              stream.push(null);
              cleanupStreamListeners();
            }
          };

          stream.on("close", handleClose);
          this.on(requestId, handleData);

          // Push the current data/end message to the stream
          handleData(response);
          
          try {
            // For stream responses, return live stream immediately without buffering
            if (options.responseType === 'stream') {
              // Create response methods that collect data when called
              const createStreamResponseMethods = (liveStream: Readable) => ({
                json: async <T = unknown>(): Promise<T> => {
                  const buffer = await streamToBuffer(liveStream);
                  return JSON.parse(buffer.toString('utf8')) as T;
                },
                text: async (): Promise<string> => {
                  const buffer = await streamToBuffer(liveStream);
                  return buffer.toString('utf8');
                },
                arrayBuffer: async (): Promise<ArrayBuffer> => {
                  const buffer = await streamToBuffer(liveStream);
                  return buffer.buffer.slice(buffer.byteOffset, buffer.byteOffset + buffer.byteLength) as ArrayBuffer;
                },
                blob: async (): Promise<Blob> => {
                  const buffer = await streamToBuffer(liveStream);
                  const contentType = responseMetadata.headers['content-type'] || responseMetadata.headers['Content-Type'] || 'application/octet-stream';
                  return new Blob([buffer], { type: Array.isArray(contentType) ? contentType[0] : contentType });
                }
              });

                            // Return response immediately with live stream
              const streamMethods = createStreamResponseMethods(stream);

              resolveRequest({
                status: responseMetadata.statusCode,
                headers: responseMetadata.headers,
                finalUrl: responseMetadata.finalUrl,
                data: stream, // Return live stream directly
                ...streamMethods
              });
            } else {
              // Get raw buffer first for response methods (existing behavior)
              const rawBuffer = await streamToBuffer(stream);

              // Check if there was an error during body read
              if (bodyReadError) {
                // Return error response instead of successful response
                const errorResponse = {
                  status: bodyReadError.statusCode,
                  headers: {} as HttpHeaders,
                  finalUrl: url,
                  data: bodyReadError.message,
                  json: async <T = unknown>(): Promise<T> => Promise.resolve({} as T),
                  text: async () => Promise.resolve(bodyReadError.message),
                  arrayBuffer: async () => Promise.resolve(new ArrayBuffer(0)),
                  blob: async () => Promise.resolve(new Blob([bodyReadError.message], { type: 'text/plain' }))
                };
                resolveRequest(errorResponse);
                return;
              }

              // Parse data based on responseType for backward compatibility
              const parsedData = await parseResponseData(
                new Readable({
                  read() {
                    this.push(rawBuffer);
                    this.push(null);
                  }
                }),
                options.responseType,
                responseMetadata.headers
              );

                            // Create response methods
              const responseMethods = createResponseMethods(rawBuffer, responseMetadata.headers);

              resolveRequest({
                status: responseMetadata.statusCode,
                headers: responseMetadata.headers,
                finalUrl: responseMetadata.finalUrl,
                data: parsedData,
                ...responseMethods
              });
            }
          } catch (error) {
            cleanupTimeout();
            rejectRequest(error);
          }
        }
      };

      this.on(requestId, handleMessage);
    });
  }

  // HTTP method implementations
  head(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> {
    return this.request(url, options, "head");
  }
  
  get(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> {
    return this.request(url, options, "get");
  }

  post(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> {
    return this.request(url, options, "post");
  }

  put(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> {
    return this.request(url, options, "put");
  }

  delete(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> {
    return this.request(url, options, "delete");
  }

  trace(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> {
    return this.request(url, options, "trace");
  }

  options(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> {
    return this.request(url, options, "options");
  }

  connect(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> {
    return this.request(url, options, "connect");
  }

  patch(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> {
    return this.request(url, options, "patch");
  }

  // WebSocket methods
  async ws(url: string, options: CycleTLSRequestOptions = {}): Promise<CycleTLSWebSocket> {
    // Set WebSocket protocol
    options.protocol = "websocket";

    const requestId = `${this.clientId}#${url}#${Date.now()}-${Math.floor(1000 * Math.random())}`;

    // Apply default fingerprinting and user agent options
    applyDefaultOptions(options);

    // Create WebSocket instance
    const ws = new CycleTLSWebSocket(this.sharedInstance, requestId, url);

    // Set up message handler
    const handleMessage = (response: { method: string; data: WebSocketMessageData }) => {
      ws._handleMessage(response.method, response.data);
    };

    this.on(requestId, handleMessage);

    // Clean up handler when connection closes or errors
    const cleanupWsHandler = () => {
      this.off(requestId, handleMessage);
      ws.off('close', cleanupWsHandler);
      ws.off('error', cleanupWsHandler);
    };
    ws.once('close', cleanupWsHandler);
    ws.once('error', cleanupWsHandler);

    // Send WebSocket request
    await this.sharedInstance.sendRequest(requestId, {
      url,
      ...options,
      method: "get",
    });

    // Return WebSocket immediately (it will emit 'open' when connected)
    return ws;
  }

  webSocket(url: string, options: CycleTLSRequestOptions = {}): Promise<CycleTLSWebSocket> {
    return this.ws(url, options);
  }

  // Server-Sent Events methods
  sse(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSSSEResponse> {
    // Set SSE protocol
    options.protocol = "sse";
    return this.request(url, options, "get") as Promise<CycleTLSSSEResponse>;
  }

  eventSource(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSSSEResponse> {
    return this.sse(url, options);
  }

  async exit(): Promise<undefined> {
    // Remove this client from the shared instance
    this.sharedInstance.removeClient(this.clientId);
    
    // Clean up this client's event listeners
    this.removeAllListeners();
    
    return Promise.resolve(undefined);
  }
}

// Global cleanup handler for the entire process
let globalShuttingDown = false;
let globalCleanupRegistered = false;

const globalCleanup = async () => {
  if (globalShuttingDown) return;
  globalShuttingDown = true;

  try {
    await InstanceManager.getInstance().cleanup();
  } catch (error) {
    console.error('Error during global cleanup:', error);
  } finally {
    // Remove handlers after cleanup to prevent leaks on create/destroy cycles
    removeGlobalCleanupHandlers();
  }
};

function registerGlobalCleanupHandlers(): void {
  if (globalCleanupRegistered) return;
  globalCleanupRegistered = true;
  globalShuttingDown = false;
  process.on("SIGINT", globalCleanup);
  process.on("SIGTERM", globalCleanup);
  process.on("beforeExit", globalCleanup);
}

function removeGlobalCleanupHandlers(): void {
  if (!globalCleanupRegistered) return;
  globalCleanupRegistered = false;
  process.off("SIGINT", globalCleanup);
  process.off("SIGTERM", globalCleanup);
  process.off("beforeExit", globalCleanup);
}

// Set up process-wide cleanup handlers
registerGlobalCleanupHandlers();

// Utility function to convert stream to buffer
async function streamToBuffer(stream: Readable): Promise<Buffer> {
  const chunks: Buffer[] = [];
  return new Promise<Buffer>((resolve, reject) => {
    stream
      .on("data", (chunk) => chunks.push(chunk))
      .on("end", () => resolve(Buffer.concat(chunks)))
      .on("error", (err) => reject(err));
  });
}

// Parse response data based on responseType (Axios-style)
async function parseResponseData(
  stream: Readable,
  responseType: string = 'json',
  headers: HttpHeaders
): Promise<ResponseData> {
  const buffer = await streamToBuffer(stream);

  switch (responseType) {
    case 'arraybuffer':
      return buffer.buffer.slice(buffer.byteOffset, buffer.byteOffset + buffer.byteLength);

    case 'blob':
      const contentType = headers['content-type'] || headers['Content-Type'] || 'application/octet-stream';
      return new Blob([buffer], { type: Array.isArray(contentType) ? contentType[0] : contentType });

    case 'text':
      return buffer.toString('utf8');

    case 'stream':
      // Return the original stream (though it's already consumed)
      const newStream = new Readable({ read() {} });
      newStream.push(buffer);
      newStream.push(null);
      return newStream;

    case 'json':
    default:
      try {
        return JSON.parse(buffer.toString('utf8')) as unknown;
      } catch {
        // If JSON parsing fails, return raw buffer (could be compressed)
        return buffer;
      }
  }
}

// Helper functions to create response parsing methods
function createResponseMethods(rawBuffer: Buffer, headers: HttpHeaders) {
  return {
    json: async <T = unknown>(): Promise<T> => {
      try {
        return JSON.parse(rawBuffer.toString('utf8')) as T;
      } catch (error) {
        const err = error instanceof Error ? error : new Error(String(error));
        throw new Error(`Failed to parse response as JSON: ${err.message}`);
      }
    },
    
    text: async (): Promise<string> => {
      return rawBuffer.toString('utf8');
    },
    
    arrayBuffer: async (): Promise<ArrayBuffer> => {
      return rawBuffer.buffer.slice(rawBuffer.byteOffset, rawBuffer.byteOffset + rawBuffer.byteLength) as ArrayBuffer;
    },
    
    blob: async (): Promise<Blob> => {
      const contentType = headers['content-type'] || headers['Content-Type'] || 'application/octet-stream';
      return new Blob([rawBuffer], { type: Array.isArray(contentType) ? contentType[0] : contentType });
    }
  };
}
/**
 * PacketBuffer provides sequential reading of typed values from a binary buffer.
 * Issue #2: All read methods include bounds checking to prevent buffer overflows.
 */
class PacketBuffer {

  private _data: Buffer;
  private _index: number;

  constructor(data: Buffer) {
    this._data = data;
    this._index = 0;
  }

  private _remaining(): number {
    return this._data.length - this._index;
  }

  readU8(): number {
    if (this._remaining() < 1) {
      throw new Error(`PacketBuffer underflow: need 1 byte, have ${this._remaining()}`);
    }
    return this._data[this._index++];
  }

  readU16(): number {
    if (this._remaining() < 2) {
      throw new Error(`PacketBuffer underflow: need 2 bytes, have ${this._remaining()}`);
    }
    const v = (this._data[this._index] << 8) | this._data[this._index + 1];
    this._index += 2;
    return v >>> 0; // ensure unsigned
  }

  readU32(): number {
    if (this._remaining() < 4) {
      throw new Error(`PacketBuffer underflow: need 4 bytes, have ${this._remaining()}`);
    }
    const v = (this._data[this._index] << 24)
      | (this._data[this._index + 1] << 16)
      | (this._data[this._index + 2] << 8)
      | this._data[this._index + 3];
    this._index += 4;
    return v >>> 0; // ensure unsigned
  }

  readU64(): bigint {
    if (this._remaining() < 8) {
      throw new Error(`PacketBuffer underflow: need 8 bytes, have ${this._remaining()}`);
    }
    const high = (BigInt(this._data[this._index]) << 56n) |
      (BigInt(this._data[this._index + 1]) << 48n) |
      (BigInt(this._data[this._index + 2]) << 40n) |
      (BigInt(this._data[this._index + 3]) << 32n);
    const low = (BigInt(this._data[this._index + 4]) << 24n) |
      (BigInt(this._data[this._index + 5]) << 16n) |
      (BigInt(this._data[this._index + 6]) << 8n) |
      BigInt(this._data[this._index + 7]);
    this._index += 8;
    return high | low;
  }

  readBytes(is64: boolean): Buffer {
    const len = is64 ? Number(this.readU64()) : this.readU32();
    // Issue #2: Validate length does not exceed remaining buffer
    if (len < 0) {
      throw new Error(`PacketBuffer: negative length ${len}`);
    }
    if (len > this._remaining()) {
      throw new Error(`PacketBuffer underflow: need ${len} bytes, have ${this._remaining()}`);
    }
    // Issue #6: Enforce max read size (10MB)
    const MAX_READ_BYTES = 10 * 1024 * 1024;
    if (len > MAX_READ_BYTES) {
      throw new Error(`PacketBuffer: read size ${len} exceeds maximum ${MAX_READ_BYTES}`);
    }
    const bytes = this._data.subarray(this._index, this._index + len);
    this._index += len;
    return bytes;
  }

  readString(encoding?: BufferEncoding): string {
    const len = this.readU16();
    // Issue #2: Validate string length against remaining buffer
    if (len > this._remaining()) {
      throw new Error(`PacketBuffer underflow: string length ${len} exceeds remaining ${this._remaining()}`);
    }
    const bytes = this._data.subarray(this._index, this._index + len);
    this._index += len;
    return bytes.toString(encoding);
  }
}


/**
 * The CycleTLS client interface for making HTTP requests with TLS fingerprinting.
 * Supports all standard HTTP methods, WebSocket connections, and Server-Sent Events.
 *
 * @example
 * ```typescript
 * const cycleTLS = await initCycleTLS();
 *
 * // Make a GET request
 * const response = await cycleTLS.get('https://example.com', {
 *   ja3: '771,4865-4866-4867-49195...',
 *   userAgent: 'Mozilla/5.0...'
 * });
 *
 * // Or use the callable interface
 * const response = await cycleTLS('https://example.com', {}, 'get');
 *
 * // Don't forget to cleanup
 * await cycleTLS.exit();
 * ```
 */
export interface CycleTLSClient {
  /**
   * Make an HTTP request with the specified method.
   * @param url - The URL to request
   * @param options - Request configuration options
   * @param method - HTTP method (default: 'get')
   * @returns Promise resolving to the response
   */
  (
    url: string,
    options: CycleTLSRequestOptions,
    method?: "head" | "get" | "post" | "put" | "delete" | "trace" | "options" | "connect" | "patch"
  ): Promise<CycleTLSResponse>;
  /** Make a HEAD request */
  head(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  /** Make a GET request */
  get(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  /** Make a POST request */
  post(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  /** Make a PUT request */
  put(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  /** Make a DELETE request */
  delete(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  /** Make a TRACE request */
  trace(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  /** Make an OPTIONS request */
  options(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  /** Make a CONNECT request */
  connect(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  /** Make a PATCH request */
  patch(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSResponse>;

  /** Open a WebSocket connection */
  ws(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSWebSocket>;
  /** Open a WebSocket connection (alias for ws) */
  webSocket(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSWebSocket>;

  /** Open a Server-Sent Events (SSE) connection */
  sse(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSSSEResponse>;
  /** Open a Server-Sent Events connection (alias for sse) */
  eventSource(url: string, options?: CycleTLSRequestOptions): Promise<CycleTLSSSEResponse>;

  /**
   * Close the CycleTLS client and cleanup resources.
   * Should be called when done making requests.
   */
  exit(): Promise<undefined>;
}

/**
 * Initialize a CycleTLS client for making HTTP requests with TLS fingerprinting.
 *
 * @param initOptions - Configuration options for the client
 * @param initOptions.port - Port number for the Go backend (default: 9119)
 * @param initOptions.debug - Enable debug logging (default: false)
 * @param initOptions.timeout - Connection timeout in milliseconds (default: 20000)
 * @param initOptions.executablePath - Custom path to the CycleTLS binary
 * @param initOptions.autoExit - Automatically cleanup on process exit (default: true)
 * @returns Promise resolving to the CycleTLS client
 *
 * @example
 * ```typescript
 * // Basic usage
 * const cycleTLS = await initCycleTLS();
 * const response = await cycleTLS('https://example.com', {
 *   ja3: '771,4865-4866-4867-49195...',
 *   userAgent: 'Mozilla/5.0...'
 * });
 * console.log(response.body);
 * await cycleTLS.exit();
 *
 * // With custom options
 * const cycleTLS = await initCycleTLS({
 *   port: 9120,
 *   debug: true,
 *   timeout: 30000
 * });
 * ```
 */
const initCycleTLS = async (
  initOptions: {
    port?: number;
    debug?: boolean;
    timeout?: number;
    executablePath?: string;
    autoExit?: boolean;
  } = {}
): Promise<CycleTLSClient> => {
  let { port, debug, timeout, executablePath, autoExit = true } = initOptions;

  if (!port) port = 9119;
  if (!debug) debug = false;
  if (!timeout) timeout = 20000;

  try {
    // Get or create shared instance for this port
    const sharedInstance = await InstanceManager.getInstance().getOrCreateSharedInstance(
      port, debug, timeout, executablePath
    );

    // Create a new client connected to the shared instance
    const client = new CycleTLSClientImpl(sharedInstance);

    // Create the public interface that matches the expected API
    const publicInterface = (() => {
      const CycleTLS = async (
        url: string,
        options: CycleTLSRequestOptions,
        method: "head" | "get" | "post" | "put" | "delete" | "trace" | "options" | "connect" | "patch" = "get"
      ): Promise<CycleTLSResponse> => {
        return client.request(url, options, method);
      };

      // HTTP method functions
      CycleTLS.head = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
        return client.head(url, options);
      };
      CycleTLS.get = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
        return client.get(url, options);
      };
      CycleTLS.post = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
        return client.post(url, options);
      };
      CycleTLS.put = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
        return client.put(url, options);
      };
      CycleTLS.delete = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
        return client.delete(url, options);
      };
      CycleTLS.trace = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
        return client.trace(url, options);
      };
      CycleTLS.options = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
        return client.options(url, options);
      };
      CycleTLS.connect = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
        return client.connect(url, options);
      };
      CycleTLS.patch = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
        return client.patch(url, options);
      };

      // WebSocket methods
      CycleTLS.ws = (url: string, options: CycleTLSRequestOptions = {}): Promise<CycleTLSWebSocket> => {
        return client.ws(url, options);
      };
      CycleTLS.webSocket = (url: string, options: CycleTLSRequestOptions = {}): Promise<CycleTLSWebSocket> => {
        return client.webSocket(url, options);
      };

      // Server-Sent Events methods
      CycleTLS.sse = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSSSEResponse> => {
        return client.sse(url, options);
      };
      CycleTLS.eventSource = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSSSEResponse> => {
        return client.eventSource(url, options);
      };

      // Utility methods
      CycleTLS.exit = async (): Promise<undefined> => {
        return client.exit();
      };

      return CycleTLS;
    })();

    return publicInterface;
  } catch (error) {
    throw new Error(`Failed to initialize CycleTLS: ${error.message}`);
  }
};

// CycleTLS exports (streaming with backpressure)
export {
  CycleTLS,
  CycleTLSOptions,
  RequestOptions,
  Response,
  CycleTLSError,
  CycleTLSWebSocketV2,
  SSEResponse,
} from "./flow-control-client";
// Note: SSEEvent is already exported from the legacy API definitions above
export { CreditManager } from "./credit-manager";
export * as Protocol from "./protocol";

// Default export: CycleTLS
import CycleTLS from "./flow-control-client";
import { CycleTLSWebSocketV2 } from "./flow-control-client";
export default CycleTLS;

// Legacy WebSocket class (from legacy API)
export { CycleTLSWebSocket };

// Streaming API WebSocket (alias for CycleTLSWebSocketV2)
export { CycleTLSWebSocketV2 as StreamingWebSocket };

// Legacy API for backward compatibility
export { initCycleTLS };

// Internal exports for testing
export { InstanceManager as _InstanceManager };
export { registerGlobalCleanupHandlers as _registerGlobalCleanupHandlers };
export { removeGlobalCleanupHandlers as _removeGlobalCleanupHandlers };

// CommonJS compatibility
module.exports = CycleTLS;
module.exports.default = CycleTLS;
module.exports.CycleTLS = CycleTLS;
module.exports.CycleTLSWebSocket = CycleTLSWebSocket;
module.exports.CycleTLSWebSocketV2 = CycleTLSWebSocketV2;
module.exports.StreamingWebSocket = CycleTLSWebSocketV2;
module.exports.initCycleTLS = initCycleTLS;
module.exports._InstanceManager = InstanceManager;
module.exports._registerGlobalCleanupHandlers = registerGlobalCleanupHandlers;
module.exports._removeGlobalCleanupHandlers = removeGlobalCleanupHandlers;
module.exports.__esModule = true;
