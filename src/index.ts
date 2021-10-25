import { spawn, exec, ChildProcessWithoutNullStreams } from "child_process";
import path from "path";
import { EventEmitter } from "events";
import { Server } from "ws";
export interface CycleTLSRequestOptions {
  headers?: {
    [key: string]: any;
  };
  cookies?: {
    [key: string]: any;
  };
  body?: string;
  ja3?: string;
  userAgent?: string;
  proxy?: string;
  timeout?: number;
  disableRedirect?: boolean;
}

export interface CycleTLSResponse {
  status: number;
  body: string;
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
  } else {
    //linux/darwin os
    new Promise((resolve, reject) => {
      process.kill(-child.pid);
      if (exit) process.exit();
    });
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
  server: Server;
  queue: Array<string>;
  queueId: NodeJS.Timeout;
  constructor(port: number, debug: boolean) {
    super();
    this.server = new Server({ port });
    this.queue = [];
    let executableFilename;

    if (process.platform == "win32") {
      executableFilename = "index.exe";
    } else if (process.platform == "linux") {
      executableFilename = "index";
    } else if (process.platform == "darwin") {
      executableFilename = "index-mac";
    } else {
      cleanExit(new Error("Operating system not supported"));
    }
    handleSpawn(debug, executableFilename, port);

    this.server.on("connection", (ws) => {
      this.emit("ready");
      ws.on("message", (data: string) => {
        const message = JSON.parse(data);
        this.emit(message.RequestID, message.Response);
      });

      ws.on("close", (data: string) => {
        this.emit(lastRequestID, { error: new Error(`Error Occured on URL: ${lastRequestID} Go Process is restarting`) });
      });
    });

  }

  request(
    requestId: string,
    options: {
      [key: string]: any;
    }
  ) {
    lastRequestID = requestId

    let client = [...this.server.clients][0]
    if (client) {
      client.send(JSON.stringify({ requestId, options }));
    } else {
      this.queue.push(JSON.stringify({ requestId, options }))

      if (this.queueId == null) {
        this.queueId = setInterval(() => {
          let client = [...this.server.clients][0]
          if (client) {
            for (let request of this.queue) {
              client.send(request);
            }
            this.queue = [];
            clearInterval(this.queueId);
            this.queueId = null
          }
        }, 100)
      }
    }
  }

  exit() {
    this.server.close();
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

            if (!options.ja3)
              options.ja3 = "771,255-49195-49199-49196-49200-49171-49172-156-157-47-53,0-10-11-13,23-24,0";
            if (!options.body) options.body = "";
            if (!options.proxy) options.proxy = "";

            instance.request(requestId, {
              url,
              ...options,
              method,
            });

            instance.once(requestId, (response) => {
              if (response.error) return rejectRequest(response.error);

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
          if (process.platform == "win32") {
            return new Promise((resolve, reject) => {
              exec(
                "taskkill /pid " + child.pid + " /T /F",
                (error: any, stdout: any, stderr: any) => {
                  if (error) {
                    console.warn(error);
                  }
                  instance.exit();
                  resolve(stdout ? stdout : stderr);
                }
              );
            });
          } else {
            return new Promise((resolve, reject) => {
              process.kill(-child.pid);
              instance.exit();
            });
          }
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
