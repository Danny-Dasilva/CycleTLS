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
import { Blob } from 'buffer';
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
  ja4r?: string;         // JA4 raw format (JA4R) with explicit cipher/extension values. Pass raw JA4 (JA4R) values. The JA4 hash is not accepted for configuration.
  http2Fingerprint?: string;
  quicFingerprint?: string;
  disableGrease?: boolean; // Disable GREASE for exact JA4 matching
  
  // Browser identification
  userAgent?: string;
  
  // Connection options
  serverName?: string;     // Overrides TLS Server Name Indication (SNI)
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

// Global instance manager for tracking shared instances by port
class InstanceManager {
  private static instance: InstanceManager;
  private sharedInstances: Map<number, SharedInstance> = new Map();

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
    let sharedInstance = this.sharedInstances.get(port);
    
    if (!sharedInstance) {
      sharedInstance = new SharedInstance(port, debug, timeout, executablePath);
      this.sharedInstances.set(port, sharedInstance);
      
      // Initialize the shared instance
      await sharedInstance.initialize();
    }
    
    return sharedInstance;
  }

  async removeSharedInstance(port: number): Promise<void> {
    const sharedInstance = this.sharedInstances.get(port);
    if (sharedInstance) {
      await sharedInstance.cleanup();
      this.sharedInstances.delete(port);
    }
  }

  async cleanup(): Promise<void> {
    const cleanupPromises = Array.from(this.sharedInstances.values()).map(instance => instance.cleanup());
    await Promise.all(cleanupPromises);
    this.sharedInstances.clear();
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

  constructor(port: number, debug: boolean, timeout: number, executablePath?: string) {
    super();
    this.port = port;
    this.debug = debug;
    this.timeout = timeout;
    this.executablePath = executablePath;
  }

  async initialize(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.checkSpawnedInstance(resolve, reject);
    });
  }

  private checkSpawnedInstance(resolve: () => void, reject: (reason: string) => void): void {
    this.httpServer = http.createServer();

    this.httpServer.once('listening', () => {
      // Close the HTTP server immediately after it starts listening
      this.httpServer!.close(() => {
        // Ensure all listeners are removed and server is nulled
        this.httpServer!.removeAllListeners();
        this.httpServer = null;
        this.spawnServer();
        this.isHost = true;
        this.createClient(resolve, reject);
      });
    });

    this.httpServer.once('error', (err) => {
      // Ensure the HTTP server is closed if an error occurs
      if (this.httpServer) {
        try {
          this.httpServer.close(() => {
            this.httpServer!.removeAllListeners();
            this.httpServer = null;
          });
        } catch (e) {
          console.error("Error closing server on error:", e);
          this.httpServer = null;
        }
      }
      this.createClient(resolve, reject);
      this.isHost = false;
    });

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
      this.cleanExit(new Error(`Unsupported architecture ${os.arch()} for ${process.platform}`));
      return;
    }

    this.handleSpawn(executableFilename);
  }

  private handleSpawn(fileName: string): void {
    try {
      // Determine the executable path
      let execPath: string;

      if (this.executablePath) {
        // If filePath is provided, use it directly
        execPath = this.executablePath;
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
        env: { WS_PORT: this.port.toString() },
        shell: process.platform !== "win32", // false for Windows, true for others
        windowsHide: true,
        detached: process.platform !== "win32",
        // Add cwd option to ensure proper working directory
        cwd: path.dirname(execPath)
      };
      
      this.child = spawn(execPath, [], spawnOptions);
      
      this.child.stdout.on("data", (stdout) => {
        console.log(stdout.toString());
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
      this.child.on('error', (error: any) => {
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
  }

  private createClient(resolve: () => void, reject: (reason: string) => void): void {
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
    if (this.clients.size === 0) {
      InstanceManager.getInstance().removeSharedInstance(this.port);
    }
  }

  async sendRequest(requestId: string, options: { [key: string]: any }): Promise<void> {
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
        
        form.on('data', (chunk) => {
          chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
        });
        
        form.on('end', () => {
          const result = Buffer.concat(chunks).toString('utf8');
          resolve(result);
        });
        
        form.on('error', reject);
        
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

  private async cleanExit(message?: string | Error): Promise<void> {
    if (message) console.log(message);
    if (this.isShuttingDown) return;

    this.isShuttingDown = true;

    if (this.child) {
      if (process.platform === "win32") {
        try {
          this.child.kill('SIGKILL');
        } catch (error) {
          console.error("Error killing Windows process:", error);
        }
      } else {
        try {
          process.kill(-this.child.pid!, 'SIGKILL');
        } catch (error) {
          if ((error as any).code !== "ESRCH") {
            console.error("Error killing process:", error);
          }
        }
      }
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
      if (process.platform === "win32") {
        try {
          this.child.kill('SIGKILL');
        } catch (error) {
          console.error("Error killing Windows process:", error);
        }
      } else {
        try {
          process.kill(-this.child.pid!, 'SIGKILL');
        } catch (error) {
          if ((error as any).code !== "ESRCH") {
            console.error("Error killing process:", error);
          }
        }
      }
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

    //set default options
    options ??= {}

    // Set default fingerprinting options - prefer JA3 if multiple options are provided
    if (!options?.ja3 && !options?.ja4r && !options?.http2Fingerprint && !options?.quicFingerprint) {
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
      const tempArr: { [key: string]: any } = [];

      for (const [key, value] of Object.entries(options.cookies!)) {
        tempArr.push({ name: key, value: value });
      }

      options.cookies = tempArr;
    }
    


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
      let responseMetadata: any = null;

      const handleMessage = async (response: any) => {
        if (response.method === "error") {
          // Handle error before or during body read
          // If we already have response metadata (headers sent successfully but body read failed),
          // return the error with empty headers
          const errorResponse = {
            status: response.data.statusCode,
            headers: responseMetadata ? responseMetadata.headers : {},
            finalUrl: responseMetadata ? responseMetadata.finalUrl : url,
            data: response.data.message,
            json: async () => Promise.resolve({}),
            text: async () => Promise.resolve(response.data.message),
            arrayBuffer: async () => Promise.resolve(new ArrayBuffer(0)),
            blob: async () => Promise.resolve(new Blob([response.data.message], { type: 'text/plain' }))
          };
          this.off(requestId, handleMessage);
          resolveRequest(errorResponse);
        } else if (response.method === "response") {
          // Store response metadata but don't resolve yet
          responseMetadata = response.data;
        } else if (response.method === "data" || response.method === "end") {
          // Now we have response metadata, set up stream handling
          if (!responseMetadata) return;

          // Remove the message handler and set up stream handling
          this.off(requestId, handleMessage);

          const stream = new Readable({ read() { } });

          const handleClose = () => {
            this.sharedInstance.cancelRequest(requestId);
          };

          let bodyReadError: any = null;

          const handleData = (response: any) => {
            if (response.method === "data") {
              stream.push(Buffer.from(response.data));
            } else if (response.method === "error") {
              // Handle error that occurred during body read - store it and close the stream
              bodyReadError = {
                statusCode: response.data.statusCode,
                message: response.data.message
              };
              stream.push(null); // Close stream gracefully
              stream.off("close", handleClose);
              this.off(requestId, handleData);
            } else if (response.method === "end") {
              stream.push(null);
              stream.off("close", handleClose);
              this.off(requestId, handleData);
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
                  headers: {},
                  finalUrl: url,
                  data: bodyReadError.message,
                  json: async () => Promise.resolve({}),
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
  ws(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSWebSocketResponse> {
    // Set WebSocket protocol
    options.protocol = "websocket";
    return this.request(url, options, "get") as Promise<CycleTLSWebSocketResponse>;
  }

  webSocket(url: string, options: CycleTLSRequestOptions): Promise<CycleTLSWebSocketResponse> {
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

const globalCleanup = async () => {
  if (globalShuttingDown) return;
  globalShuttingDown = true;
  
  try {
    await InstanceManager.getInstance().cleanup();
  } catch (error) {
    console.error('Error during global cleanup:', error);
  }
};

// Set up process-wide cleanup handlers
process.once("SIGINT", globalCleanup);
process.once("SIGTERM", globalCleanup);
process.once("beforeExit", globalCleanup);

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
      return rawBuffer.buffer.slice(rawBuffer.byteOffset, rawBuffer.byteOffset + rawBuffer.byteLength) as ArrayBuffer;
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
}

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
      CycleTLS.ws = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSWebSocketResponse> => {
        return client.ws(url, options);
      };
      CycleTLS.webSocket = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSWebSocketResponse> => {
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

export default initCycleTLS;
module.exports = initCycleTLS;
module.exports.default = initCycleTLS;
module.exports.__esModule = true;
