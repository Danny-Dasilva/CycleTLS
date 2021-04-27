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
(function () { return __awaiter(void 0, void 0, void 0, function () {
    var cycleTLS, i, before, _loop_1, t0, out_i_1, i_1;
    return __generator(this, function (_a) {
        switch (_a.label) {
            case 0: return [4 /*yield*/, index_js_1["default"]()];
            case 1:
                cycleTLS = _a.sent();
                i = 0;
                before = Date.now();
                _loop_1 = function (i_1) {
                    t0 = performance.now();
                    var response = cycleTLS('http://httpbin.org/cookies', {
                        body: '',
                        ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0',
                        userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36',
                        cookies: [
                            {
                                "name": "Standard",
                                "value": "aaaaaaa",
                                "time": "Mon, 02-Jan-2006 15:04:05 MST"
                            },
                            {
                                "name": "AHHH",
                                "value": "kill me",
                                "time": "Tue, 06-Jan-2019 2:03:05 MST"
                            }
                        ]
                    });
                    response.then(function (out) {
                        console.log(out);
                        i_1++;
                        var t1 = performance.now();
                        console.log("Call to doSomething took " + (t1 - t0) + " milliseconds.", i_1);
                    });
                    out_i_1 = i_1;
                };
                for (i_1 = 0; i_1 < 1; i_1++) {
                    _loop_1(i_1);
                    i_1 = out_i_1;
                }
                return [2 /*return*/];
        }
    });
}); })();
