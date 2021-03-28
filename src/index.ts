import { spawn, ChildProcessWithoutNullStreams } from 'child_process';
import path from 'path';
import { EventEmitter } from 'events';
import { Server } from 'ws';
export interface MyTlsRequestOptions {
  headers?: {
    [key: string]: any;
  };
  body?: string;
  ja3?: string;
  userAgent?: string;
  proxy?: string;
}

export interface MyTlsResponse {
  status: number;
  body: string;
  headers: {
    [key: string]: any;
  };
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

const initMyTls = async (
  initOptions: {
    port?: number;
    debug?: boolean;
  } = {}
): Promise<{
  (
    url: string,
    options: MyTlsRequestOptions,
    method?: 'head' | 'get' | 'post' | 'put' | 'delete' | 'trace' | 'options' | 'connect' | 'patch'
  ): Promise<MyTlsResponse>;
  head(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
  get(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
  post(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
  put(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
  delete(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
  trace(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
  options(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
  connect(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
  patch(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
}> => {
  return new Promise((resolveReady) => {
    let { port, debug } = initOptions;

    if (!port) port = 9119;
    if (!debug) debug = false;

    const instance = new Golang(port, debug);

    instance.on('ready', () => {
      const mytls = (() => {
        const MyTls = async (
          url: string,
          options: MyTlsRequestOptions,
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
        ): Promise<MyTlsResponse> => {
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

              if (headers['Set-Cookie']) headers['Set-Cookie'] = headers['Set-Cookie'].split('/,/');

              resolveRequest({
                status,
                body,
                headers,
              });
            });
          });
        };
        MyTls.head = (url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse> => {
          return MyTls(url, options, 'head');
        };
        MyTls.get = (url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse> => {
          return MyTls(url, options, 'get');
        };
        MyTls.post = (url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse> => {
          return MyTls(url, options, 'post');
        };
        MyTls.put = (url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse> => {
          return MyTls(url, options, 'put');
        };
        MyTls.delete = (url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse> => {
          return MyTls(url, options, 'delete');
        };
        MyTls.trace = (url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse> => {
          return MyTls(url, options, 'trace');
        };
        MyTls.options = (url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse> => {
          return MyTls(url, options, 'options');
        };
        MyTls.connect = (url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse> => {
          return MyTls(url, options, 'options');
        };
        MyTls.patch = (url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse> => {
          return MyTls(url, options, 'patch');
        };

        return MyTls;
      })();
      resolveReady(mytls);
    });
  });
};

export default initMyTls;

// CommonJS support for default export
module.exports = initMyTls;
module.exports.default = initMyTls;
module.exports.__esModule = true;
