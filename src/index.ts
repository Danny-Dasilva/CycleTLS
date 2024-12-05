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
  /** How long should we wait on a request response before giving up */
  requestTimeout: number,
  /** How long should we wait before giving up on the request received handshake */
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
  ja3?: string;
  userAgent?: string;
  proxy?: string;
  timeout?: number;
  disableRedirect?: boolean;
  headerOrder?: string[];
  insecureSkipVerify?: boolean;
  forceHTTP1?: boolean;
}

export interface CycleTLSResponse {
  status: number;
  body: string | {
    [key: string]: any;
  };
  headers: {
    [key: string]: any;
  };
  finalUrl: string;
}


let child: ChildProcessWithoutNullStreams;
let lastRequestID: string

const cleanExit = async (message?: string | Error, exit?: boolean) => {
  if (message) console.log(message);
  exit = exit ?? true;

  if (child) {

    if (process.platform == "win32") {
      child?.kill();
    } else {
      process.kill(-child.pid);
    }
    if (exit) process.exit();

  }
};
process.on("SIGINT", () => cleanExit());
process.on("SIGTERM", () => cleanExit());

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
          `Error Processing Request (please open an issue https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose) -> ${errorMessage}`,
          false
        ).then(() => handleSpawn(debug, fileName, port));
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
    const len   = is64 ? this.readU64() : this.readU32();
    const bytes = this._data.subarray(this._index, this._index + len);

    this._index += len;

    return bytes;
  }

  readString(encoding?: BufferEncoding): string {
    const len   = this.readU16();
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
    let server = http.createServer();

    server.listen(this.port)
      .on('listening', () => {
        server.close(() => {
          this.spawnServer();
          this.host = true;
        })
      })
      .on('error', () => {
        this.createClient();
        this.host = false;
      });
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
    // In-line function that represents a connection attempt
    const attemptConnection = () => {
      const server = new WebSocket(`ws://localhost:${this.port}`);

      server.on("open", () => {
        // WebSocket connected - set server and emit ready
        this.server = server;

        this.server.on("message", (data: Buffer) => {
          const packetBuffer  = new PacketBuffer(data);
          const requestID     = packetBuffer.readString();
          const method        = packetBuffer.readString();

          if (method === "response") {
            const statusCode    = packetBuffer.readU16();
            const headers       = [];
            const headersLength = packetBuffer.readU16();

            for (let i = 0; i < headersLength; i++) {
              const headerValues  = [];
              const headerName    = packetBuffer.readString();
              const valuesLength  = packetBuffer.readU16();

              for (let j = 0; j < valuesLength; j++)
                headerValues.push(packetBuffer.readString());

              headers.push([ headerName, headerValues ]);
            }

            this.emit(requestID, {
              method,
              data: {
                statusCode,
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

          // @TODO: Finish this
          if (method === "error") {
            console.log(packetBuffer);
            // this.emit(requestID, {
            //   method,
            //   data: packetBuffer.readBytes(),
            // });
          }

          if (method === "end") {
            this.emit(requestID, { method });
          }
        });

        this.emit("ready");
      })

      server.on("error", (err) => {
        // Connection error - retry in .1s
        server.removeAllListeners();

        setTimeout(() => {
          // If we've failed to initialize, stop the loop
          if (this.failedInitialization) {
            return;
          }

          attemptConnection();
        }, 100)
      })
    }
    attemptConnection();

    // Set a timeout representing the initialization timeout that'll reject the promise if not initialized within the timeout
    setTimeout(() => {
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
    return new Promise((resolve, reject) => {
      this.server.close();
      if (this.host) {
        if (process.platform == "win32") {
          child?.kill();
        } else {
          process.kill(-child.pid);
        }
        resolve(null);
      } else {
        resolve(null);
      }
    });
  }
}
export interface CycleTLSClient {
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
  exit(): Promise<undefined>;
}
const initCycleTLS = async (
  initOptions: {
    port?: number;
    debug?: boolean;
    timeout?: number;
    executablePath?: string;
  } = {}
): Promise<CycleTLSClient> => {
  return new Promise((resolveReady, reject) => {
    let { port, debug, timeout, executablePath } = initOptions;

    if (!port) port = 9119;
    if (!debug) debug = false;
    if (!timeout) timeout = 20000;

    const instance = new Golang(port, debug, timeout, executablePath);

    instance.on("ready", () => {
      const CycleTLS = (() => {
        const CycleTLS = async (
          url: string,
          options: CycleTLSRequestOptions,
          method:
            | "head"
            | "get"
            | "post"
            | "put"
            | "delete"
            | "trace"
            | "options"
            | "connect"
            | "patch" = "get"
        ): Promise<any> => {
          return new Promise((resolveRequest, rejectRequest) => {
            const requestId = `${url}#${Date.now()}-${Math.floor(1000 * Math.random())}`;

            //set default options
            options ??= {}

            //set default ja3, user agent, body and proxy
            if (!options?.ja3)
              options.ja3 = "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
            if (!options?.userAgent)
              options.userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36";

            if (!options?.body) options.body = "";
            if (!options?.proxy) options.proxy = "";
            if (!options?.insecureSkipVerify) options.insecureSkipVerify = false;
            if (!options?.forceHTTP1) options.forceHTTP1 = false;


            //convert simple cookies
            const cookies = options?.cookies;

            if (typeof cookies === "object" && !Array.isArray(cookies) && cookies !== null) {
              const tempArr: { [key: string]: any; } = [];

              for (const [key, value] of Object.entries(options.cookies)) {
                tempArr.push({ name: key, value: value });
              }

              options.cookies = tempArr;
            }

            instance.request(requestId, {
              url,
              ...options,
              method,
            });

            instance.once(requestId, (response) => {
              if (response.method === "error") {
                rejectRequest(response.data);

              } else {
                const stream = new Readable({ read() {} });

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

                resolveRequest({
                  code    : response.data.statusCode,
                  headers : response.data.headers,
                  stream,
                  raw: () => {
                    const chunks: Buffer[] = [];
                    return new Promise<Buffer>((resolve, reject) => {
                      stream
                        .on("data", (chunk) => chunks.push(chunk))
                        .on("end", () => resolve(Buffer.concat(chunks)))
                        .on("error", (err) => reject(err));
                    });
                  },
                  json: () =>
                    new Promise((resolve, reject) => {
                      let data = "";
                      stream
                        .on("data", (chunk) => (data += chunk.toString()))
                        .on("end", () => resolve(JSON.parse(data)))
                        .on("error", (err) => reject(err));
                    }),
                  text: (enc: BufferEncoding) =>
                    new Promise((resolve, reject) => {
                      let data = "";
                      stream
                        .on("data", (chunk) => (data += chunk.toString(enc)))
                        .on("end", () => resolve(data))
                        .on("error", (err) => reject(err));
                    }),
                });
              }
            });
          });
        };

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

        CycleTLS.exit = async (): Promise<undefined> => {
          return instance.exit();
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

// CommonJS support for default export
module.exports = initCycleTLS;
module.exports.default = initCycleTLS;
module.exports.__esModule = true;
