import {spawn, exec, ChildProcessWithoutNullStreams} from "child_process";
import path from "path";
import { EventEmitter } from "events";
import WebSocket from "ws";
import * as http from "http";
import os from 'os';
import util from "util";

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
export interface CycleTLSRequestOptions {
  headers?: {
    [key: string]: any;
  };
  cookies?:
     Array<object>
    | {
        [key: string]: string;
      };
  body?: string;
  ja3?: string;
  userAgent?: string;
  proxy?: string;
  timeout?: number;
  disableRedirect?: boolean;
  headerOrder?: string[];
}

export interface CycleTLSResponse {
  status: number;
  body: string | {
    [key: string]: any;
  };
  headers: {
    [key: string]: any;
  };
}

let child: ChildProcessWithoutNullStreams;
let lastRequestID: string

const cleanExit = async (message?: string | Error, exit?: boolean) => {
  if (message) console.log(message);
  exit = exit ?? true

  if (process.platform == "win32") {
    if(child) {
      new Promise((resolve, reject) => {
        exec(
            "taskkill /pid " + child.pid + " /T /F",
            (error: any, stdout: any, stderr: any) => {
              if (error) {
                console.warn(error);
              }
              if (exit) process.exit();
            }
        );
      });
    }
  } else {
    if(child) {
      //linux/darwin os
      new Promise((resolve, reject) => {
        process.kill(-child.pid);
        if (exit) process.exit();
      });
    }
  }
};
process.on("SIGINT", () => cleanExit());
process.on("SIGTERM", () => cleanExit());

const handleSpawn = (debug: boolean, fileName: string, port: number) => {
  child = spawn(path.join(`"${__dirname}"`, fileName), {
    env: { WS_PORT: port.toString() },
    shell: true,
    windowsHide: true,
    detached: process.platform !== "win32"
  });
  child.stderr.on("data", (stderr) => {
    if (stderr.toString().includes("Request_Id_On_The_Left")) {
      const splitRequestIdAndError = stderr.toString().split("Request_Id_On_The_Left");
      const [requestId, error] = splitRequestIdAndError;
      //TODO Correctly parse logging messages
      // this.emit(requestId, { error: new Error(error) });
    } else {
      debug
        ? cleanExit(new Error(stderr))
        //TODO add Correct error logging url request/ response/
        : cleanExit(`Error Processing Request (please open an issue https://github.com/Danny-Dasilva/CycleTLS/issues/new/choose) -> ${stderr}`, false).then(() => handleSpawn(debug, fileName, port));
    }
  });
}


class Golang extends EventEmitter {
  server: WebSocket;
  queue: Array<string>;
  host: boolean;
  queueId: NodeJS.Timeout;
  constructor(port: number, debug: boolean) {
    super();
    let server = http.createServer();

    server.listen(port)
        .on('listening', () => {
          server.close(() => {
            this.spawnServer(port, debug);
            this.host = true;
          })
        })
        .on('error', () => {
          this.createClient(port, debug);
          this.host = false;
        });
  }

  spawnServer(
      port: number,
      debug: boolean
  ){
    let executableFilename;

    if (process.platform == "win32") {
      executableFilename = "index.exe";
    } else if (process.platform == "linux") {

      //build arm 
      if (os.arch() == "arm") {
        executableFilename = "index-arm";
      } else if (os.arch() == "arm64") {
        executableFilename = "index-arm64";
      } else {
        //default
        executableFilename = "index";
      }
  
    } else if (process.platform == "darwin") {
      executableFilename = "index-mac";
    } else {
      cleanExit(new Error("Operating system not supported"));
    }
    handleSpawn(debug, executableFilename, port);

    this.createClient(port, debug);
  }

  createClient(
      port: number,
      debug: boolean
  ){
    const server = new WebSocket(`ws://localhost:${port}`);

    server.on("open", () => {
      // WebSocket connected - set server and emit ready
      this.server = server;

      this.server.on("message", (data: string) => {
        const message = JSON.parse(data);
        this.emit(message.RequestID, message);
      });

      this.emit("ready");
    })

    server.on("error", (err) => {
      // Connection error - retry in .1s
      server.removeAllListeners();

      setTimeout(() => {
        this.createClient(port, debug)
      }, 100)
    })
  }

  request(
    requestId: string,
    options: {
      [key: string]: any;
    }
  ) {
    lastRequestID = requestId

    if (this.server) {
      this.server.send(JSON.stringify({ requestId, options }));
    } else {
      if(this.queue == null){
        this.queue = [];
      }
      this.queue.push(JSON.stringify({ requestId, options }))

      if (this.queueId == null) {
        this.queueId = setInterval(() => {
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
    if (process.platform == "win32") {
      return new Promise((resolve, reject) => {
        this.server.close();
        if (this.host) {
          exec(
            "taskkill /pid " + child.pid + " /T /F",
            (error: any, stdout: any, stderr: any) => {
              if (error) {
                console.warn(error);
              }
              resolve(stdout ? stdout : stderr);
            }
          );
        } else {
          resolve(null);
        }
      });
    } else {
      return new Promise((resolve, reject) => {
        this.server.close();
        if (this.host) {
          process.kill(-child.pid);
          resolve(null);
        } else {
          resolve(null);
        }
      });
    }
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
  } = {}
): Promise<CycleTLSClient> => {
  return new Promise((resolveReady) => {
    let { port, debug } = initOptions;

    if (!port) port = 9119;
    if (!debug) debug = false;

    const instance = new Golang(port, debug);
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
        ): Promise<CycleTLSResponse> => {
          return new Promise((resolveRequest, rejectRequest) => {
            const requestId = `${url}${Math.floor(Date.now() * Math.random())}`;
            //set default options
            options = options ?? {}
            
            //set default ja3, user agent, body and proxy
            if (!options?.ja3)
              options.ja3 = "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
              if (!options?.userAgent)
              options.userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36";
            if (!options?.body) options.body = "";
            if (!options?.proxy) options.proxy = "";
            
            //convert simple cookies
            const cookies = options?.cookies;
            if (
              typeof cookies === "object" &&
              !Array.isArray(cookies) &&
              cookies !== null
             ) {
              const tempArr: {
                [key: string]: any;
              } = [];
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
              if (response.error) return rejectRequest(response.error);
              try {
                //parse json responses
                response.Body = JSON.parse(response.Body);
                //override console.log full repl to display full body
                response.Body[util.inspect.custom] = function(){ return JSON.stringify( this, undefined, 2); }
              } catch (e) {}

              const { Status: status, Body: body, Headers: headers } = response;
              
              if (headers["Set-Cookie"])
                headers["Set-Cookie"] = headers["Set-Cookie"].split("/,/");
              resolveRequest({
                status,
                body,
                headers,
              });
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
  });
};

export default initCycleTLS;

// CommonJS support for default export
module.exports = initCycleTLS;
module.exports.default = initCycleTLS;
module.exports.__esModule = true;
