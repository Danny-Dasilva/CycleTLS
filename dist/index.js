"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const child_process_1 = require("child_process");
const path_1 = __importDefault(require("path"));
const events_1 = require("events");
const ws_1 = require("ws");
let child;
const cleanExit = (message) => {
    if (message)
        console.log(message);
    child.kill();
    process.exit();
};
process.on('SIGINT', () => cleanExit());
process.on('SIGTERM', () => cleanExit());
class Golang extends events_1.EventEmitter {
    constructor(port, debug) {
        super();
        this.server = new ws_1.Server({ port });
        let executableFilename;
        if (process.platform == 'win32') {
            executableFilename = 'index.exe';
        }
        else if (process.platform == 'linux') {
            executableFilename = 'index';
        }
        else if (process.platform == 'darwin') {
            executableFilename = 'index-mac';
        }
        else {
            cleanExit(new Error('Operating system not supported'));
        }
        child = child_process_1.spawn(path_1.default.join(__dirname, executableFilename), {
            env: { WS_PORT: port.toString() },
            shell: true,
            windowsHide: true,
        });
        child.stderr.on('data', (stderr) => {
            if (stderr.toString().includes('Request_Id_On_The_Left')) {
                const splitRequestIdAndError = stderr.toString().split('Request_Id_On_The_Left');
                const [requestId, error] = splitRequestIdAndError;
                this.emit(requestId, { error: new Error(error) });
            }
            else {
                debug
                    ? cleanExit(new Error(stderr))
                    : cleanExit(new Error('Invalid JA3 hash. Exiting... (Golang wrapper exception)'));
            }
        });
        this.server.on('connection', (ws) => {
            this.emit('ready');
            ws.on('message', (data) => {
                const message = JSON.parse(data);
                this.emit(message.RequestID, message.Response);
            });
        });
    }
    request(requestId, options) {
        [...this.server.clients][0].send(JSON.stringify({ requestId, options }));
    }
}
const initMyTls = async (initOptions = {}) => {
    return new Promise((resolveReady) => {
        let { port, debug } = initOptions;
        if (!port)
            port = 9119;
        if (!debug)
            debug = false;
        const instance = new Golang(port, debug);
        instance.on('ready', () => {
            const mytls = (() => {
                const MyTls = async (url, options, method = 'get') => {
                    return new Promise((resolveRequest, rejectRequest) => {
                        const requestId = `${url}${Math.floor(Date.now() * Math.random())}`;
                        if (!options.ja3)
                            options.ja3 =
                                '771,255-49195-49199-49196-49200-49171-49172-156-157-47-53,0-10-11-13,23-24,0';
                        if (!options.body)
                            options.body = '';
                        if (!options.proxy)
                            options.proxy = '';
                        instance.request(requestId, {
                            url,
                            ...options,
                            method,
                        });
                        instance.once(requestId, (response) => {
                            if (response.error)
                                return rejectRequest(response.error);
                            const { Status: status, Body: body, Headers: headers } = response;
                            if (headers['Set-Cookie'])
                                headers['Set-Cookie'] = headers['Set-Cookie'].split('/,/');
                            resolveRequest({
                                status,
                                body,
                                headers,
                            });
                        });
                    });
                };
                MyTls.head = (url, options) => {
                    return MyTls(url, options, 'head');
                };
                MyTls.get = (url, options) => {
                    return MyTls(url, options, 'get');
                };
                MyTls.post = (url, options) => {
                    return MyTls(url, options, 'post');
                };
                MyTls.put = (url, options) => {
                    return MyTls(url, options, 'put');
                };
                MyTls.delete = (url, options) => {
                    return MyTls(url, options, 'delete');
                };
                MyTls.trace = (url, options) => {
                    return MyTls(url, options, 'trace');
                };
                MyTls.options = (url, options) => {
                    return MyTls(url, options, 'options');
                };
                MyTls.connect = (url, options) => {
                    return MyTls(url, options, 'options');
                };
                MyTls.patch = (url, options) => {
                    return MyTls(url, options, 'patch');
                };
                return MyTls;
            })();
            resolveReady(mytls);
        });
    });
};
exports.default = initMyTls;
// CommonJS support for default export
module.exports = initMyTls;
module.exports.default = initMyTls;
module.exports.__esModule = true;
