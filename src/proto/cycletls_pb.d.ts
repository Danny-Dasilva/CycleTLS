// package: cyclestream
// file: proto/cycletls.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class CycleTLSRequest extends jspb.Message { 
    getRequestid(): string;
    setRequestid(value: string): CycleTLSRequest;
    getUrl(): string;
    setUrl(value: string): CycleTLSRequest;
    getMethod(): string;
    setMethod(value: string): CycleTLSRequest;

    getHeadersMap(): jspb.Map<string, string>;
    clearHeadersMap(): void;
    getBody(): string;
    setBody(value: string): CycleTLSRequest;
    getJa3(): string;
    setJa3(value: string): CycleTLSRequest;
    getUseragent(): string;
    setUseragent(value: string): CycleTLSRequest;
    getProxy(): string;
    setProxy(value: string): CycleTLSRequest;

    getCookiesMap(): jspb.Map<string, string>;
    clearCookiesMap(): void;
    getTimeout(): number;
    setTimeout(value: number): CycleTLSRequest;
    getDisableredirect(): boolean;
    setDisableredirect(value: boolean): CycleTLSRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CycleTLSRequest.AsObject;
    static toObject(includeInstance: boolean, msg: CycleTLSRequest): CycleTLSRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CycleTLSRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CycleTLSRequest;
    static deserializeBinaryFromReader(message: CycleTLSRequest, reader: jspb.BinaryReader): CycleTLSRequest;
}

export namespace CycleTLSRequest {
    export type AsObject = {
        requestid: string,
        url: string,
        method: string,

        headersMap: Array<[string, string]>,
        body: string,
        ja3: string,
        useragent: string,
        proxy: string,

        cookiesMap: Array<[string, string]>,
        timeout: number,
        disableredirect: boolean,
    }
}

export class Response extends jspb.Message { 
    getRequestid(): string;
    setRequestid(value: string): Response;
    getStatus(): number;
    setStatus(value: number): Response;
    getBody(): string;
    setBody(value: string): Response;

    getHeadersMap(): jspb.Map<string, string>;
    clearHeadersMap(): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Response.AsObject;
    static toObject(includeInstance: boolean, msg: Response): Response.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Response, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Response;
    static deserializeBinaryFromReader(message: Response, reader: jspb.BinaryReader): Response;
}

export namespace Response {
    export type AsObject = {
        requestid: string,
        status: number,
        body: string,

        headersMap: Array<[string, string]>,
    }
}
