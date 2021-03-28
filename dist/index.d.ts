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
declare const initMyTls: (initOptions?: {
    port?: number;
    debug?: boolean;
}) => Promise<{
    (url: string, options: MyTlsRequestOptions, method?: 'head' | 'get' | 'post' | 'put' | 'delete' | 'trace' | 'options' | 'connect' | 'patch'): Promise<MyTlsResponse>;
    head(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
    get(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
    post(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
    put(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
    delete(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
    trace(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
    options(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
    connect(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
    patch(url: string, options: MyTlsRequestOptions): Promise<MyTlsResponse>;
}>;
export default initMyTls;
