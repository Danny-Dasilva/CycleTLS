import CycleTLS from "../dist/index.js";
import { withCycleTLS, withUpstreamRetry, UPSTREAM_FLAKE_STATUSES } from "./test-utils.js";
jest.setTimeout(60000);
let ja3 =
  "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0";
let userAgent =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36";

interface Request {
  url: string;
  headers?: {
    [key: string]: any;
  };
  cookies?: {
    [key: string]: any;
  };
  body?: string;
  response?: {
    [key: string]: any;
  };
  ja3?: string;
  userAgent?: string;
  proxy?: string;
  method?:
    | "head"
    | "get"
    | "post"
    | "put"
    | "delete"
    | "trace"
    | "options"
    | "connect"
    | "patch";
}

const myRequests: Request[] = [
  {
    url: "https://httpbin.org/brotli",
    ja3: ja3,
    userAgent: userAgent,
    response: {
      brotli: true,
      headers: {
        "Accept-Encoding": "gzip, deflate, br",
        Host: "httpbin.org",
        "User-Agent":
          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
      },
      method: "GET",
    },
  },

  {
    url: "https://httpbin.org/deflate",
    response: {
      deflated: true,
      headers: {
        "Accept-Encoding": "gzip, deflate, br",
        Host: "httpbin.org",
        "User-Agent":
          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
      },
      method: "GET",
    },
  },

  {
    url: "https://httpbin.org/gzip",
    response: {
      gzipped: true,
      headers: {
        "Accept-Encoding": "gzip, deflate, br",
        Host: "httpbin.org",
        "User-Agent":
          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
      },
      method: "GET",
    },
  },
];

test("Response data contains raw compressed data (Axios-style)", async () => {
  await withCycleTLS({ port: 9115, timeout: 10000 }, async (client) => {
    for (let request of myRequests) {
      // Test with default responseType (json) - should return raw buffer for compressed data
      const response = await withUpstreamRetry(() =>
        client.get(request.url, {
          ja3: ja3,
          userAgent: userAgent,
          headers: { 'Accept-Encoding': 'gzip, deflate, br' },
        })
      );

      if (UPSTREAM_FLAKE_STATUSES.has(response.status)) {
        console.log(`Skipped ${request.url}: httpbin upstream flake (status ${response.status})`);
        continue;
      }
      expect(response.status).toBe(200);

      // New API returns a Readable stream as data - consume it to get buffer
      const buffer = await response.buffer();
      expect(buffer).toBeInstanceOf(Buffer);
      expect(buffer.length).toBeGreaterThan(0);

      // Test with explicit arraybuffer method
      const arrayBufferResponse = await withUpstreamRetry(() =>
        client.get(request.url, {
          ja3: ja3,
          userAgent: userAgent,
          headers: { 'Accept-Encoding': 'gzip, deflate, br' },
        })
      );

      if (UPSTREAM_FLAKE_STATUSES.has(arrayBufferResponse.status)) {
        console.log(`Skipped ${request.url} (arrayBuffer): httpbin upstream flake (status ${arrayBufferResponse.status})`);
        continue;
      }

      const arrayBuffer = await arrayBufferResponse.arrayBuffer();
      expect(arrayBuffer).toBeInstanceOf(ArrayBuffer);
      expect(arrayBuffer.byteLength).toBeGreaterThan(0);
    }
  });
});
