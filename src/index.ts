import { spawn, exec, ChildProcessWithoutNullStreams } from "child_process";
import path from "path";
import { EventEmitter } from "events";
import { Server } from "ws";
import * as grpc from '@grpc/grpc-js';
import { CycleStreamClient } from './proto/cycletls_grpc_pb';
import { CycleTLSRequest, Response } from './proto/cycletls_pb';
import { Stream } from "stream";
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

const doBidirectionalStreamingCall = (client: Golang) => {
  const stream = client.server.stream();
  console.log("bidirectional streaming call")
  // Server stream
  stream.on('data', (serverMessage: Response) => {
    console.log(
      `(client) Got server message: ${serverMessage.getRequestid()}`
    );
    // console.log(
    //   `(client) Got server message: ${serverMessage.getStatus()}`
    // );
    // console.log(
    //   `(client) Got server message: ${serverMessage.getRequestid()}`
    // );
    // console.log(
    //   `(client) Got server message: ${serverMessage.getHeadersMap()}`
    // );
    client.emit(serverMessage.getRequestid(),
    {"Status": serverMessage.getStatus(), "Body": serverMessage.getBody(), "Headers": []}
    )

  });
 
}



const cleanExit = async (message?: string | Error) => {
  if (message) {
    console.log(message);
  }

  if (process.platform == "win32") {
    new Promise((resolve, reject) => {
      exec(
        "taskkill /pid " + child.pid + " /T /F",
        (error: any, stdout: any, stderr: any) => {
          if (error) {
            console.warn(error);
          }
          process.exit();
          resolve(stdout ? stdout : stderr);
        }
      );
    });
  } else {
    //linux/darwin os
    new Promise((resolve, reject) => {
      process.kill(-child.pid);
      process.exit();
    });
  }
};
process.on("SIGINT", () => cleanExit());
process.on("SIGTERM", () => cleanExit());

class Golang extends EventEmitter {
  server: CycleStreamClient;
  constructor(port: number, debug: boolean) {
    super();

    // this.server = new Server({ port });

    const host = '0.0.0.0:10000';
    const deadline = new Date();
    deadline.setSeconds(deadline.getSeconds() + 5);
    this.server = new CycleStreamClient(host, grpc.credentials.createInsecure());
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
    console.log("aaa")
    // child = spawn(path.join(`"${__dirname}"`, executableFilename), {
    //   env: { WS_PORT: port.toString() },
    //   shell: true,
    //   windowsHide: true,
    //   detached: true,
    // });

    // child.stderr.on("data", (stderr) => {
    //   if (stderr.toString().includes("Request_Id_On_The_Left")) {
    //     const splitRequestIdAndError = stderr.toString().split("Request_Id_On_The_Left");
    //     const [requestId, error] = splitRequestIdAndError;
    //     this.emit(requestId, { error: new Error(error) });
    //   } else {
    //     debug
    //       ? cleanExit(new Error(stderr))
    //       //TODO add Correct error logging url request/ response/ 
    //       : cleanExit(new Error("Error Exiting ... (Golang wrapper exception)"));
    //   }
    // });


    this.server.waitForReady(deadline, (error?: Error) => {
      if (error) {
        console.log(`Client connect error: ${error.message}`);
      } else {
        this.emit("ready");
        doBidirectionalStreamingCall(this);
      }
    });


    // this.server.on("connection", (ws) => {
    //   this.emit("ready");

    //   ws.on("message", (data: string) => {
    //     const message = JSON.parse(data);
    //     this.emit(message.RequestID, message.Response);
    //   });
      
    // });
  }

  request(
    requestId: string,
    options: {
      [key: string]: any;
    }
  ) {
     // Client stream
     console.log("request called")
     const stream = this.server.stream();
      const cycleTLSRequest = new CycleTLSRequest();
      cycleTLSRequest.setRequestid(requestId);
      cycleTLSRequest.setUrl(options.url);
      cycleTLSRequest.setMethod(options.method);
      cycleTLSRequest.setBody(options.body);
      cycleTLSRequest.setJa3(options.ja3);
      cycleTLSRequest.setUseragent(options.userAgent);

      cycleTLSRequest.setProxy(options.proxy);
      cycleTLSRequest.setTimeout(options.timeout);
      cycleTLSRequest.setDisableredirect(options.disableRedirect);


      stream.write(cycleTLSRequest);
  }

  exit() {
    const stream = this.server.stream();
    stream.end()
    // grpc.closeClient(this.server);
    // this.server.close();
  }
}



const initCycleTLS = async (
  initOptions: {
    port?: number;
    debug?: boolean;
  } = {}
): Promise<{
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
}> => {
  return new Promise((resolveReady) => {
    let { port, debug } = initOptions;

    if (!port) port = 9119;
    if (!debug) debug = true;

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
              console.log(response)
              if (response.error) return rejectRequest(response.error);

              const { Status: status, Body: body, Headers: headers } = response;

              // if (headers["Set-Cookie"])
              //   headers["Set-Cookie"] = headers["Set-Cookie"].split("/,/");

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
          
          return new Promise((resolve, reject) => {
                instance.exit();
              });
          // if (process.platform == "win32") {
          //   return new Promise((resolve, reject) => {
          //     exec(
          //       "taskkill /pid " + child.pid + " /T /F",
          //       (error: any, stdout: any, stderr: any) => {
          //         if (error) {
          //           console.warn(error);
          //         }
          //         instance.exit();
          //         resolve(stdout ? stdout : stderr);
          //       }
          //     );
          //   });
          // } else {
          //   return new Promise((resolve, reject) => {
          //     process.kill(-child.pid);
          //     instance.exit();
          //   });
          // }
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
