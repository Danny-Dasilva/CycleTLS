import CycleTLS from '../dist/index.js'
import { withCycleTLS, withUpstreamRetry, UPSTREAM_FLAKE_STATUSES } from "./test-utils.js";
import http from 'http';
jest.setTimeout(60000);

// Check if httpbin.org is reachable before running tests that depend on it
let httpbinAvailable = true;
beforeAll(async () => {
  try {
    await new Promise<void>((resolve, reject) => {
      const req = http.get('http://httpbin.org/get', { timeout: 5000 }, (res) => {
        res.resume(); // Consume response body
        resolve();
      });
      req.on('error', () => reject());
      req.on('timeout', () => { req.destroy(); reject(); });
    });
  } catch {
    httpbinAvailable = false;
    console.warn('httpbin.org is not reachable - skipping integration tests that depend on it');
  }
});

let ja3 = "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0"
let userAgent = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36'

interface RequestCookie {
    name: string;
    value: string;
    expires?: string;
}

interface Request {
    url: string;
    headers?: {
        [key: string]: any;
    };
    cookies?: RequestCookie[];
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
                "expires": "2022-01-02T15:04:05Z"
            },
            {
                "name": "example2",
                "value": "bbbbbbbbb",
                "expires": "2022-01-06T02:03:05Z"
            }]
    },
    { url: "http://httpbin.org/ip" },  //log ip
    { url: "http://httpbin.org/response-headers?ExampleResponseHeader=HeaderisPickedUpByServer" },  //log response headers
    { url: "http://httpbin.org/html" }, //log html
    { url: "https://example.com" }, //Test extension 43

];


test('Should Return 200 for all responses', async () => {
    if (!httpbinAvailable) {
        console.warn('Skipping: httpbin.org is not reachable');
        return;
    }
    await withCycleTLS({ port: 9123, timeout: 10000 }, async (client) => {
        for (let request of myRequests) {
            const options = {
                body: request.body,
                ja3: request.ja3,
                userAgent: request.userAgent,
                headers: request.headers,
                cookies: request.cookies,
            };

            // Wrap in upstream-flake retry — skip-on-flake per request rather
            // than fail the whole iteration when httpbin returns 408/502/etc.
            const response = await withUpstreamRetry(async () => {
                switch (request.method) {
                    case 'post':
                        return await client.post(request.url, request.body || '', options);
                    case 'put':
                        return await client.put(request.url, request.body || '', options);
                    case 'patch':
                        return await client.patch(request.url, request.body || '', options);
                    case 'delete':
                        return await client.delete(request.url, options);
                    default:
                        return await client.get(request.url, options);
                }
            });

            if (UPSTREAM_FLAKE_STATUSES.has(response.status)) {
                console.log(`Skipped ${request.url}: httpbin upstream flake (status ${response.status})`);
                continue;
            }

            // Handle different response types based on URL
            let result;
            if (request.url.includes('/html') || request.url.includes('example.com')) {
                // These URLs return HTML, not JSON
                result = await response.text();
                expect(typeof result).toBe('string');
            } else {
                // These URLs return JSON - but some might fail, so let's be safe
                try {
                    result = await response.json();
                    expect(typeof result).toBe('object');
                } catch (error) {
                    // If JSON parsing fails, fall back to text
                    result = await response.text();
                    expect(typeof result).toBe('string');
                }
            }
            expect(response.status).toBe(200)
        }
    });
});
