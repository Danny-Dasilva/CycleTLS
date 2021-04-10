"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __generator = (this && this.__generator) || function (thisArg, body) {
    var _ = { label: 0, sent: function() { if (t[0] & 1) throw t[1]; return t[1]; }, trys: [], ops: [] }, f, y, t, g;
    return g = { next: verb(0), "throw": verb(1), "return": verb(2) }, typeof Symbol === "function" && (g[Symbol.iterator] = function() { return this; }), g;
    function verb(n) { return function (v) { return step([n, v]); }; }
    function step(op) {
        if (f) throw new TypeError("Generator is already executing.");
        while (_) try {
            if (f = 1, y && (t = op[0] & 2 ? y["return"] : op[0] ? y["throw"] || ((t = y["return"]) && t.call(y), 0) : y.next) && !(t = t.call(y, op[1])).done) return t;
            if (y = 0, t) op = [op[0] & 2, t.value];
            switch (op[0]) {
                case 0: case 1: t = op; break;
                case 4: _.label++; return { value: op[1], done: false };
                case 5: _.label++; y = op[1]; op = [0]; continue;
                case 7: op = _.ops.pop(); _.trys.pop(); continue;
                default:
                    if (!(t = _.trys, t = t.length > 0 && t[t.length - 1]) && (op[0] === 6 || op[0] === 2)) { _ = 0; continue; }
                    if (op[0] === 3 && (!t || (op[1] > t[0] && op[1] < t[3]))) { _.label = op[1]; break; }
                    if (op[0] === 6 && _.label < t[1]) { _.label = t[1]; t = op; break; }
                    if (t && _.label < t[2]) { _.label = t[2]; _.ops.push(op); break; }
                    if (t[2]) _.ops.pop();
                    _.trys.pop(); continue;
            }
            op = body.call(thisArg, _);
        } catch (e) { op = [6, e]; y = 0; } finally { f = t = 0; }
        if (op[0] & 5) throw op[1]; return { value: op[0] ? op[1] : void 0, done: true };
    }
};
exports.__esModule = true;
// const initCycleTLS = require('./src/index');
var index_js_1 = require("../dist/index.js");
var performance = require('perf_hooks').performance;
// Typescript: import initCycleTLS from 'cycletls';
var ja3 = '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0';
var userAgent = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36';
var myDictionary = {
    'https://httpbin.org/user-agent': {
        'ja3': ja3,
        'userAgent': userAgent
    },
    'http://httpbin.org/post': {
        'body': '{"field":"POST-VAL"}',
        'method': 'POST'
    },
    'http://httpbin.org/put': {
        'body': '{"field":"PUT-VAL"}',
        'method': 'PUT'
    },
    'http://httpbin.org/patch': {
        'body': '{"field":"PATCH-VAL"}',
        'method': 'PATCH'
    },
    'http://httpbin.org/delete': {
        'body': '{"field":"DELETE-VAL"}',
        'method': 'Delete'
    },
    'http://httpbin.org/headers': {
        'headers': { "Authorization": "Bearer someexampletoken" }
    },
    // 'http://httpbin.org/cookies/set?name1=value1&name2=value2' : {
    // },
    'https://httpbin.org/ip': {},
    'https://httpbin.org/response-headers?ExampleResponseHeader=HeaderisPickedUpByServer': {},
    'https://httpbin.org/html': {},
    'https://httpbin.org/delay/3': {} //this request will wait 3 s before returning so it should always show up last in the console.log
};
(function () { return __awaiter(void 0, void 0, void 0, function () {
    var cycleTLS, _loop_1, key;
    var _a, _b, _c, _d;
    return __generator(this, function (_e) {
        switch (_e.label) {
            case 0: return [4 /*yield*/, index_js_1["default"]()];
            case 1:
                cycleTLS = _e.sent();
                _loop_1 = function (key) {
                    var value = myDictionary[key];
                    var body = (_a = value.body) !== null && _a !== void 0 ? _a : '';
                    var method = (_b = value.method) !== null && _b !== void 0 ? _b : 'GET';
                    var ja3Token = (_c = value.ja3) !== null && _c !== void 0 ? _c : ja3;
                    var Agent = (_d = value.userAgent) !== null && _d !== void 0 ? _d : userAgent;
                    var response = cycleTLS(key, {
                        body: body,
                        ja3: ja3Token,
                        userAgent: Agent,
                        headers: value.headers
                    }, method);
                    response.then(function (out) {
                        console.log(key, out);
                    });
                };
                for (key in myDictionary) {
                    _loop_1(key);
                }
                return [2 /*return*/];
        }
    });
}); })();
