import { spawn, ChildProcessWithoutNullStreams } from 'child_process';
import path from 'path';
import { EventEmitter } from 'events';
import { Server } from 'ws';
export interface CycleTLSRequestOptions {
  body?: string;
  ja3?: string;
  userAgent?: string;
  id?: number;
  proxy?: string;
}

export interface CycleTLSResponse {
  status: number;
  body: string;
}
let child: ChildProcessWithoutNullStreams;

const cleanExit = (message?: string | Error) => {
  if (message) console.log(message);
  child.kill();
  process.exit();
};
process.on('SIGINT', () => cleanExit());
process.on('SIGTERM', () => cleanExit());

class Golang extends EventEmitter {
  server: Server;
  constructor(port: number, debug: boolean) {
    super();

    this.server = new Server({ port });

    let executableFilename;

    if (process.platform == 'win32') {
      executableFilename = 'index.exe';
    } else if (process.platform == 'linux') {
      executableFilename = 'index';
    } else if (process.platform == 'darwin') {
      executableFilename = 'index-mac';
    } else {
      cleanExit(new Error('Operating system not supported'));
    }

    child = spawn(path.join(__dirname, executableFilename), {
      env: { WS_PORT: port.toString() },
      shell: true,
      windowsHide: true,
    });

    child.stderr.on('data', (stderr) => {
      
      if (stderr.toString().includes('Request_Id_On_The_Left')) {
        const splitRequestIdAndError = stderr.toString().split('Request_Id_On_The_Left');
        const [requestId, error] = splitRequestIdAndError;
        this.emit(requestId, { error: new Error(error) });
      } else {
        console.log(stderr.toString())
        debug
          ? cleanExit(new Error(stderr))
          : cleanExit(new Error('Invalid JA3 hash. Exiting... (Golang wrapper exception)'));
      }
    });

    this.server.on('connection', (ws) => {
      this.emit('ready');

      ws.on('message', (data: string) => {
        const message = JSON.parse(data);
        this.emit(message.RequestID, message.Response);
      });
    });
  }

  request(
    requestId: string,
    options: {
      [key: string]: any;
    }
  ) {
    [...this.server.clients][0].send(JSON.stringify({ requestId, options }));
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
    method?: 'head' | 'get' | 'post' | 'put' | 'delete' | 'trace' | 'options' | 'connect' | 'patch'
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
}> => {
  return new Promise((resolveReady) => {
    let { port, debug } = initOptions;

    if (!port) port = 9119;
    if (!debug) debug = false;

    const instance = new Golang(port, debug);

    instance.on('ready', () => {
      const CycleTLS = (() => {
        const CycleTLS = async (
          url: string,
          options: CycleTLSRequestOptions,
          method:
            | 'head'
            | 'get'
            | 'post'
            | 'put'
            | 'delete'
            | 'trace'
            | 'options'
            | 'connect'
            | 'patch' = 'get'
        ): Promise<CycleTLSResponse> => {
          return new Promise((resolveRequest, rejectRequest) => {
            const requestId = `${url}${Math.floor(Date.now() * Math.random())}`;

            if (!options.ja3)
              options.ja3 =
                '771,255-49195-49199-49196-49200-49171-49172-156-157-47-53,0-10-11-13,23-24,0';
            if (!options.body) options.body = '';
            if (!options.proxy) options.proxy = '';

            instance.request(requestId, {
              url,
              ...options,
              method,
            });

            instance.once(requestId, (response) => {
              if (response.error) return rejectRequest(response.error);

              const { Status: status, Body: body, Headers: headers } = response;

              // if (headers['Set-Cookie']) headers['Set-Cookie'] = headers['Set-Cookie'].split('/,/');

              resolveRequest({
                status,
                body,
              });
            });
          });
        };
        CycleTLS.head = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, 'head');
        };
        CycleTLS.get = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, 'get');
        };
        CycleTLS.post = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, 'post');
        };
        CycleTLS.put = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, 'put');
        };
        CycleTLS.delete = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, 'delete');
        };
        CycleTLS.trace = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, 'trace');
        };
        CycleTLS.options = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, 'options');
        };
        CycleTLS.connect = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, 'options');
        };
        CycleTLS.patch = (url: string, options: CycleTLSRequestOptions): Promise<CycleTLSResponse> => {
          return CycleTLS(url, options, 'patch');
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
