import { spawn, ChildProcessWithoutNullStreams, SpawnOptionsWithoutStdio } from "child_process";
import path from "path";
import { EventEmitter } from "events";
import WebSocket from "ws";
import * as http from "http";
import os from 'os';
import FormData from 'form-data';
import { Readable, Writable } from 'stream';
import { promisify } from 'util';
import stream from 'stream';
const pipeline = promisify(stream.pipeline);

export interface Cookie {
  name: string;
  value: string;
  path?: string;
  domain?: string;
  expires?: string;
  rawExpires?: string;
  maxAge?: number;
  secure?: boolean;
  httpOnly?: boolean;
  sameSite?: string;
  unparsed?: string;
}

export interface TimeoutOptions {
  requestTimeout: number,
  acknowledgementTimeout?: number
}

export interface CycleTLSRequestOptions {
  headers?: {
    [key: string]: any;
  };
  cookies?:
  Array<object>
  | {
    [key: string]: string;
  };
  body?: string | URLSearchParams | FormData;
  
  // Response type (like Axios)
  responseType?: 'json' | 'text' | 'arraybuffer' | 'blob' | 'stream';
  
  // TLS fingerprinting options
  ja3?: string;
  ja4?: string;
  http2Fingerprint?: string;
  quicFingerprint?: string;
  
  // Browser identification
  userAgent?: string;
  
  // Connection options
  proxy?: string;
  timeout?: number;
  disableRedirect?: boolean;
  headerOrder?: string[];
  orderAsProvided?: boolean;
  insecureSkipVerify?: boolean;
  
  // Protocol options
  forceHTTP1?: boolean;
  forceHTTP3?: boolean;
  protocol?: string; // "http1", "http2", "http3", "websocket", "sse"
}

export interface CycleTLSResponse {
  status: number;
  headers: {
    [key: string]: any;
  };
  data: any; // Axios-style data property
  finalUrl: string;
  // Axios/Fetch-like response methods
  json(): Promise<any>;
  text(): Promise<string>;
  arrayBuffer(): Promise<ArrayBuffer>;
  blob(): Promise<Blob>;
}

export interface WebSocketMessage {
  type: 'text' | 'binary' | 'close' | 'ping' | 'pong';
  data: string | Buffer;
}

export interface CycleTLSWebSocketResponse extends CycleTLSResponse {
  // WebSocket specific methods
  send(data: string | Buffer, isBinary?: boolean): Promise<void>;
  close(code?: number, reason?: string): Promise<void>;
  onMessage(callback: (message: WebSocketMessage) => void): void;
  onClose(callback: (code: number, reason: string) => void): void;
  onError(callback: (error: Error) => void): void;
}

export interface SSEEvent {
  id?: string;
  event?: string;
  data: string;
  retry?: number;
}

export interface CycleTLSSSEResponse extends CycleTLSResponse {
  // SSE specific methods
  events(): AsyncIterableIterator<SSEEvent>;
  onEvent(callback: (event: SSEEvent) => void): void;
  onError(callback: (error: Error) => void): void;
  close(): Promise<void>;
}

let child: ChildProcessWithoutNullStreams;
let lastRequestID: string;
let isShuttingDown = false;

const cleanExit = async (message?: string | Error) => {
  if (message) console.log(message);
  if (isShuttingDown) return;

  isShuttingDown = true;

  if (child) {
    if (process.platform === "win32") {
      try {
        child.kill('SIGKILL');
      } catch (error) {
        console.error("Error killing Windows process:", error);
      }
    } else {
      try {
        process.kill(-child.pid, 'SIGKILL');
      } catch (error) {
        if (error.code !== "ESRCH") {
          console.error("Error killing process:", error);
        }
      }
    }
  }
};

const close = () => cleanExit();


process
  .once("SIGINT", close)
  .once("SIGTERM", close);

const handleSpawn = (debug: boolean, fileName: string, port: number, filePath?: string) => {
  try {
    // Determine the executable path
    let execPath: string;

    if (filePath) {
      // If filePath is provided, use it directly
      execPath = filePath;
    } else {
      // Otherwise, construct path relative to __dirname
      execPath = path.join(__dirname, fileName);
    }

    // Remove quotes as they're not needed and can cause issues on Windows
    execPath = execPath.replace(/"/g, '');

    // Verify file exists before attempting to spawn
    if (!require('fs').existsSync(execPath)) {
      throw new Error(`Executable not found at path: ${execPath}`);
    }

    const spawnOptions: SpawnOptionsWithoutStdio = {
      env: { WS_PORT: port.toString() },
      shell: process.platform !== "win32", // false for Windows, true for others
      windowsHide: true,
      detached: process.platform !== "win32",
      // Add cwd option to ensure proper working directory
      cwd: path.dirname(execPath)
    };
    child = spawn(execPath, [], spawnOptions);
    child.stdout.on("data", (stdout) => {
      console.log(stdout.toString());
    });
    child.stderr.on("data", (stderr) => {
      const errorMessage = stderr.toString();
      if (errorMessage.includes("Request_Id_On_The_Left")) {
        const [requestId, error] = errorMessage.split("Request_Id_On_The_Left");
        // Handle request-specific error
      } else {
        if (debug) {
          cleanExit(new Error(errorMessage));
        } else {
          cleanExit(
            `Error Processing Request (please open an issue https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose) -> ${errorMessage}`
          )
          handleSpawn(debug, fileName, port);
        }
      }
    });

    // Add error handler for spawn errors
    child.on('error', (error: any) => {
      console.error(`Failed to start subprocess: ${error.message}`);
      if (error.code === 'ENOENT') {
        console.error(`Executable not found at: ${execPath}`);
        console.error('Please ensure the executable exists and has correct permissions');
      }
      throw error;
    });

  } catch (error) {
    console.error(`Error in handleSpawn: ${error.message}`);
    throw error;
  }
};

// Function to convert a stream into a string
async function streamToString(stream: Readable): Promise<string> {
  const chunks: Buffer[] = [];
  await pipeline(
    stream,
    new Writable({
      write(chunk: Buffer, encoding: BufferEncoding, callback: (error?: Error | null) => void) {
        chunks.push(chunk);
        callback();
      }
    })
  );
  return Buffer.concat(chunks).toString('utf8');
}

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
  headers: { [key: string]: any }
): Promise<any> {
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
        return JSON.parse(buffer.toString('utf8'));
      } catch (error) {
        // If JSON parsing fails, return raw buffer (could be compressed)
        return buffer;
      }
  }
}

// Helper functions to create response parsing methods
function createResponseMethods(rawBuffer: Buffer, headers: { [key: string]: any }) {
  return {
    json: async (): Promise<any> => {
      try {
        return JSON.parse(rawBuffer.toString('utf8'));
      } catch (error) {
        throw new Error(`Failed to parse response as JSON: ${error.message}`);
      }
    },
    
    text: async (): Promise<string> => {
      return rawBuffer.toString('utf8');
    },
    
    arrayBuffer: async (): Promise<ArrayBuffer> => {
      return rawBuffer.buffer.slice(rawBuffer.byteOffset, rawBuffer.byteOffset + rawBuffer.byteLength);
    },
    
    blob: async (): Promise<Blob> => {
      const contentType = headers['content-type'] || headers['Content-Type'] || 'application/octet-stream';
      return new Blob([rawBuffer], { type: Array.isArray(contentType) ? contentType[0] : contentType });
    }
  };
}
class PacketBuffer {

  private _data: Buffer;
  private _index: number;

  constructor(data: Buffer) {
    this._data = data;
    this._index = 0;
  }

  readU8(): number {
    return this._data[this._index++];
  }

  readU16(): number {
    return this.readU8() << 8
      | this.readU8();
  }

  readU32(): number {
    return this.readU8() << 24
      | this.readU8() << 16
      | this.readU8() << 8
      | this.readU8();
  }

  readU64(): number {
    return this.readU8() << 56
      | this.readU8() << 48
      | this.readU8() << 40
      | this.readU8() << 32
      | this.readU8() << 24
      | this.readU8() << 16
      | this.readU8() << 8
      | this.readU8();
  }

  readBytes(is64: boolean): Buffer {
    const len = is64 ? this.readU64() : this.readU32();
    const bytes = this._data.subarray(this._index, this._index + len);

    this._index += len;

    return bytes;
  }

  readString(encoding?: BufferEncoding): string {
    const len = this.readU16();
    const bytes = this._data.subarray(this._index, this._index + len);

    this._index += len;

    return bytes.toString(encoding);
  }
}

class Golang extends EventEmitter {
  server: WebSocket;
  queue: Array<string>;
  host: boolean;
  queueId: NodeJS.Timeout;
  private httpServer: http.Server; // Add reference to HTTP server
  private connectionTimeout: NodeJS.Timeout;
  private timeout: number;
  private port: number;
  private debug: boolean;
  private filePath?: string;
  private failedInitialization: boolean = false;

  constructor(port: number, debug: boolean, timeout: number, filePath?: string) {
    super();

    this.port = port;
    this.debug = debug;
    this.timeout = timeout;
    this.filePath = filePath;

    this.checkSpawnedInstance();
  }


  checkSpawnedInstance() {
    this.httpServer = http.createServer();

    this.httpServer.once('listening', () => {
      // Close the HTTP server immediately after it starts listening
      this.httpServer.close(() => {
        // Ensure all listeners are removed and server is nulled
        this.httpServer.removeAllListeners();
        this.httpServer = null;
        this.spawnServer();
        this.host = true;
      });
    });

    this.httpServer.once('error', (err) => {
      // Ensure the HTTP server is closed if an error occurs
      if (this.httpServer) {
        try {
          this.httpServer.close(() => {
            this.httpServer.removeAllListeners();
            this.httpServer = null;
          });
        } catch (e) {
          console.error("Error closing server on error:", e);
          this.httpServer = null;
        }
      }
      this.createClient();
      this.host = false;
    });

    // Start listening last so that the above listeners are in place
    this.httpServer.listen(this.port);
  }
  spawnServer() {
    const PLATFORM_BINARIES: { [platform: string]: { [arch: string]: string } } = {
      "win32": { "x64": "index.exe" },
      "linux": { "arm": "index-arm", "arm64": "index-arm64", "x64": "index" },
      "darwin": { "x64": "index-mac", "arm": "index-mac-arm", "arm64": "index-mac-arm64" },
      "freebsd": { "x64": "index-freebsd" }
    };

    const executableFilename = PLATFORM_BINARIES[process.platform]?.[os.arch()];
    if (!executableFilename) {
      cleanExit(new Error(`Unsupported architecture ${os.arch()} for ${process.platform}`));
    }

    handleSpawn(this.debug, executableFilename, this.port, this.filePath);

    this.createClient();
  }

  createClient() {
    const attemptConnection = () => {
      const server = new WebSocket(`ws://localhost:${this.port}`);

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

            this.emit(requestID, {
              method,
              data: {
                statusCode,
                finalUrl,
                headers: Object.fromEntries(headers),
              },
            });
          }

          if (method === "data") {
            this.emit(requestID, {
              method,
              data: packetBuffer.readBytes(false),
            });
          }

          if (method === "error") {
            console.log(packetBuffer);
          }

          if (method === "end") {
            this.emit(requestID, { method });
          }
        });

        this.emit("ready");
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
      this.emit("failure", `Could not connect to the CycleTLS instance within ${this.timeout}ms`);
    }, this.timeout);
  }

  async cancelRequest(requestId: string) {
    if (this.server) {
      this.server.send(JSON.stringify({ action: "cancel", requestId }));
    }
  }
  async request(
    requestId: string,
    options: {
      [key: string]: any;
    }
  ): Promise<void> {
    lastRequestID = requestId

    // Check if options.body is URLSearchParams and convert to string
    if (options.body instanceof URLSearchParams) {
      options.body = options.body.toString();
    }
    // Check if options.body is FormData and convert to string
    if (options.body instanceof FormData) {
      options.headers = { ...options.headers, ...options.body.getHeaders() };
      options.body = await streamToString(options.body as unknown as Readable);
    }

    if (this.server) {
      this.server.send(JSON.stringify({ requestId, options }), (err) => {
        // An error occurred sending the webhook to a server we already confirmed exists - let's get back up and running

        // First, we'll create a queue to store the failed request
        // Do a check to make sure server isn't null to prevent a race condition where multiple requests fail
        if (err) {
          if (this.server != null) {
            // Add failed request to queue
            this.server = null;

            // Just resend the request so that it adds to queue properly
            this.request(requestId, options);

            // Start process of client re-creation
            this.checkSpawnedInstance();
          } else {
            // Add to queue and hope server restarts properly
            this.queue.push(JSON.stringify({ requestId, options }));
          }
        }
      });
    } else {
      if (this.queue == null) {
        this.queue = [];
      }
      this.queue.push(JSON.stringify({ requestId, options }))

      if (this.queueId == null) {
        this.queueId = setInterval(() => {
          // If we failed to initialize - clear the queue
          if (this.failedInitialization) {
            clearInterval(this.queueId);
            this.queue = [];
            this.queueId = null;
            return;
          }

          // If the server is available - empty the queue into the server and delete the queue
          if (this.server) {
            for (let request of this.queue) {
              this.server.send(request);
            }
            this.queue = [];
            clearInterval(this.queueId);
            this.queueId = null
          }
        }, 100)
      }
    }
  }
  exit(): Promise<undefined> {
    return new Promise((resolve) => {
      const cleanup = () => {
        // Clear any pending intervals or timeouts
        if (this.queueId) {
          clearInterval(this.queueId);
          this.queueId = null;
        }
        if (this.connectionTimeout) {
          clearTimeout(this.connectionTimeout);
          this.connectionTimeout = null;
        }

        process.removeListener("SIGINT", close);
        process.removeListener("SIGTERM", close);

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
        if (this.host && child) {
          if (process.platform === "win32") {
            try {
              child.kill('SIGKILL');
            } catch (error) {
              console.error("Error killing Windows process:", error);
            }
          } else {
            try {
              process.kill(-child.pid, 'SIGKILL');
            } catch (error) {
              if (error.code !== "ESRCH") {
                console.error("Error killing process:", error);
              }
            }
          }
          child = null;
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

        this.removeAllListeners();
        resolve(undefined);
      };

      cleanup();
    });
  }
}

export interface CycleTLSClient {
  // Basic HTTP methods
  (
    url: string,
    options: CycleTLSRequestOptions,
    method?: "head" | "get" | "post" | "put" | "delete" | "trace" | "options" | "connect" | "patch"
  ): Promise<CycleTLSResponse>;
  head(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  get(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  post(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  put(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  delete(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  trace(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  options(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  connect(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  patch(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse>;
  
  // WebSocket methods
  ws(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSWebSocketResponse>;
  webSocket(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSWebSocketResponse>;
  
  // Server-Sent Events (SSE) methods
  sse(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSSSEResponse>;
  eventSource(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSSSEResponse>;
  
  // Utility methods
  exit(): Promise<undefined>;
}const initCycleTLS = async (
  initOptions: {
    port?: number;
    debug?: boolean;
    timeout?: number;
    executablePath?: string;
    autoExit?: boolean;
  } = {}
): Promise<CycleTLSClient> => {
  return new Promise((resolveReady, reject) => {
    let { port, debug, timeout, executablePath, autoExit = true } = initOptions;

    if (!port) port = 9119;
    if (!debug) debug = false;
    if (!timeout) timeout = 20000;

    const instance = new Golang(port, debug, timeout, executablePath);
    let exitTimeout: NodeJS.Timeout;

    const gracefulShutdown = async () => {
      if (isShuttingDown) return;
      isShuttingDown = true;

      try {
        if (exitTimeout) {
          clearTimeout(exitTimeout);
        }
        if (instance) {
          await instance.exit();
        }
      } catch (error) {
        console.error('Error during graceful shutdown:', error);
      }
    };

    if (autoExit) {
      process.on('beforeExit', gracefulShutdown);
      process.on('SIGINT', gracefulShutdown);
      process.on('SIGTERM', gracefulShutdown);
      process.on('uncaughtException', async (error) => {
        console.error('Uncaught Exception:', error);
        await gracefulShutdown();
      });
    }

    instance.on("ready", () => {
      const CycleTLS = (() => {
        // Track connections by host for connection reuse
        const connectionsByHost = new Map<string, boolean>();
        
        const CycleTLS = async (
          url: string,
          options: CycleTLSRequestOptions,
          method: "head" | "get" | "post" | "put" | "delete" | "trace" | "options" | "connect" | "patch" = "get"
        ): Promise<any> => {
          try {
            if (exitTimeout) {
              clearTimeout(exitTimeout);
            }
            
            // Track connection reuse by parsing the URL's host
            const urlObj = new URL(url);
            const hostKey = urlObj.host;

            const response = await new Promise((resolveRequest, rejectRequest) => {
              const requestId = `${url}#${Date.now()}-${Math.floor(1000 * Math.random())}`;

              //set default options
              options ??= {}

              // Set default fingerprinting options - prefer JA3 if multiple options are provided
              if (!options?.ja3 && !options?.ja4 && !options?.http2Fingerprint && !options?.quicFingerprint) {
                options.ja3 = "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
              }
              
              // Set default user agent
              if (!options?.userAgent) {
                options.userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36";
              }

              // Set default request options
              if (!options?.body) options.body = "";
              if (!options?.proxy) options.proxy = "";
              if (!options?.insecureSkipVerify) options.insecureSkipVerify = false;
              if (!options?.forceHTTP1) options.forceHTTP1 = false;
              if (!options?.forceHTTP3) options.forceHTTP3 = false;
              if (!options?.responseType) options.responseType = 'json';
              if (!options?.protocol) {
                // Default to standard HTTP protocol
                options.protocol = ""; // Empty string means standard HTTP/HTTPS
              }


              //convert simple cookies
              const cookies = options?.cookies;

              if (typeof cookies === "object" && !Array.isArray(cookies) && cookies !== null) {
                const tempArr: { [key: string]: any; } = [];

                for (const [key, value] of Object.entries(options.cookies)) {
                  tempArr.push({ name: key, value: value });
                }

                options.cookies = tempArr;
              }

              // Track if we've connected to this host before for connection reuse
              const hasExistingConnection = connectionsByHost.has(hostKey);
              
              // Set the connection as tracked for this host
              connectionsByHost.set(hostKey, true);
              
              // Add connection reuse hint in the options
              instance.request(requestId, {
                url,
                ...options,
                method,
                // Add metadata about connection reuse (will be ignored by Go if not implemented)
                _connectionReuse: hasExistingConnection ? "reuse" : "new",
                _hostKey: hostKey,
              });

              instance.once(requestId, async (response) => {
                if (response.method === "error") {
                  rejectRequest(response.data);

                } else {
                  const stream = new Readable({ read() { } });

                  const handleClose = () => {
                    instance.cancelRequest(requestId);
                  };

                  const handleData = (response: any) => {
                    if (response.method === "data") {
                      stream.push(Buffer.from(response.data));

                    } else if (response.method === "end") {
                      stream.push(null);
                      stream.off("close", handleClose);
                      instance.off(requestId, handleData);
                    }
                  };

                  stream.on("close", handleClose);
                  instance.on(requestId, handleData);
                  
                  try {
                    // For stream responses, return live stream immediately without buffering
                    if (options.responseType === 'stream') {
                      // Create response methods that collect data when called
                      const createStreamResponseMethods = (liveStream: Readable) => ({
                        json: async (): Promise<any> => {
                          const buffer = await streamToBuffer(liveStream);
                          return JSON.parse(buffer.toString('utf8'));
                        },
                        text: async (): Promise<string> => {
                          const buffer = await streamToBuffer(liveStream);
                          return buffer.toString('utf8');
                        },
                        arrayBuffer: async (): Promise<ArrayBuffer> => {
                          const buffer = await streamToBuffer(liveStream);
                          return buffer.buffer.slice(buffer.byteOffset, buffer.byteOffset + buffer.byteLength);
                        },
                        blob: async (): Promise<Blob> => {
                          const buffer = await streamToBuffer(liveStream);
                          const contentType = response.data.headers['content-type'] || response.data.headers['Content-Type'] || 'application/octet-stream';
                          return new Blob([buffer], { type: Array.isArray(contentType) ? contentType[0] : contentType });
                        }
                      });
                      
                      // Return response immediately with live stream
                      const streamMethods = createStreamResponseMethods(stream);
                      
                      resolveRequest({
                        status: response.data.statusCode,
                        headers: response.data.headers,
                        finalUrl: response.data.finalUrl,
                        data: stream, // Return live stream directly
                        ...streamMethods
                      });
                    } else {
                      // Get raw buffer first for response methods (existing behavior)
                      const rawBuffer = await streamToBuffer(stream);
                      
                      // Parse data based on responseType for backward compatibility
                      const parsedData = await parseResponseData(
                        new Readable({
                          read() {
                            this.push(rawBuffer);
                            this.push(null);
                          }
                        }), 
                        options.responseType, 
                        response.data.headers
                      );
                      
                      // Create response methods
                      const responseMethods = createResponseMethods(rawBuffer, response.data.headers);
                      
                      resolveRequest({
                        status: response.data.statusCode,
                        headers: response.data.headers,
                        finalUrl: response.data.finalUrl,
                        data: parsedData,
                        ...responseMethods
                      });
                    }
                  } catch (error) {
                    rejectRequest(error);
                  }
                }
              });
            });

            if (autoExit) {
              exitTimeout = setTimeout(() => {
                gracefulShutdown();
              }, 100);
            }

            return response;
          } catch (error) {
            if (autoExit) {
              exitTimeout = setTimeout(() => {
                gracefulShutdown();
              }, 100);
            }
            throw error;
          }
        };

        // HTTP method functions remain the same
        CycleTLS.head = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, "head");
        };
        CycleTLS.get = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, "get");
        };

        CycleTLS.post = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, "post");
        };

        CycleTLS.put = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, "put");
        };

        CycleTLS.delete = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, "delete");
        };

        CycleTLS.trace = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, "trace");
        };

        CycleTLS.options = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, "options");
        };

        CycleTLS.connect = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, "options");
        };

        CycleTLS.patch = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, "patch");
        };

        // WebSocket methods
        CycleTLS.ws = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSWebSocketResponse> => {
          // Set WebSocket protocol
          options.protocol = "websocket";
          return CycleTLS(url, options, "get") as Promise<CycleTLSWebSocketResponse>;
        };

        CycleTLS.webSocket = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSWebSocketResponse> => {
          return CycleTLS.ws(url, options);
        };

        // Server-Sent Events methods
        CycleTLS.sse = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSSSEResponse> => {
          // Set SSE protocol
          options.protocol = "sse";
          return CycleTLS(url, options, "get") as Promise<CycleTLSSSEResponse>;
        };

        CycleTLS.eventSource = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSSSEResponse> => {
          return CycleTLS.sse(url, options);
        };

        // Utility methods
        CycleTLS.exit = async (): Promise<undefined> => {
          if (exitTimeout) {
            clearTimeout(exitTimeout);
          }

          try {
            await instance.exit();
          } catch (error) {
            console.error('Error during exit:', error);
          }
          return Promise.resolve(undefined);
        };


        return CycleTLS;
      })();

      resolveReady(CycleTLS);
    });

    instance.on("failure", (reason: string) => {
      reject(reason);
    });
  });
};

export default initCycleTLS;
module.exports = initCycleTLS;
module.exports.default = initCycleTLS;
module.exports.__esModule = true;