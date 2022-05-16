
import initCycleTLS from '../dist/index.js'
jest.setTimeout(30000);


let ja3 = '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0'
let userAgent = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36'

interface Request {
    url: string;
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
    method?: 'head' | 'get' | 'post' | 'put' | 'delete' | 'trace' | 'options' | 'connect' | 'patch'
}

const myRequests: Request[] = [
    {
        url: "https://httpbin.org/user-agent",
        ja3: ja3,
        userAgent: userAgent
    },

    {
        url: "http://httpbin.org/post",
        body: '{"field":"POST-VAL"}',
        method: 'post'
    },

    {
        url: "http://httpbin.org/put",
        body: '{"field":"PUT-VAL"}',
        method: 'put'
    },

    {
        url: "http://httpbin.org/patch",
        body: '{"field":"PATCH-VAL"}',
        method: 'patch'
    },

    {
        url: "http://httpbin.org/delete",
        body: '{"field":"DELETE-VAL"}',
        method: 'delete'
    },

    {
        url: "http://httpbin.org/headers",
        headers: { "Authorization": "Bearer someexampletoken", }
    },
    {
        url: "http://httpbin.org/cookies",
        cookies: [
            {
                "name": "example1",
                "value": "aaaaaaa",
                "expires": "Mon, 02-Jan-2022 15:04:05 EST"
            },
            {
                "name": "example2",
                "value": "bbbbbbbbb",
                "expires": "Tue, 06-Jan-2022 2:03:05 EST"
            }]
    },
    { url: "http://httpbin.org/ip" },  //log ip 
    { url: "http://httpbin.org/response-headers?ExampleResponseHeader=HeaderisPickedUpByServer" },  //log response headers
    { url: "http://httpbin.org/html" }, //log html
    { url: "https://example.com" }, //Test extension 43

];


test('Should Return 200 for all responses', async () => {
    const cycleTLS = await initCycleTLS();

    for (let request of myRequests) {
        const response = await cycleTLS(request.url, {
            body: request.body,
            ja3: request.ja3,
            userAgent: request.userAgent,
            headers: request.headers,
            cookies: request.cookies,
        }, request.method);


        expect(response.status).toBe(200)
    }
    cycleTLS.exit()

});

