import initCycleTLS from "../dist/index.js";
jest.setTimeout(30000);
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

test("Response bodies should be decoded", async () => {
  const cycleTLS = await initCycleTLS({ port: 9115 });

  for (let request of myRequests) {
    const response = await cycleTLS(
      request.url,
      {
        ja3: ja3,
        userAgent: userAgent,
      },
      "get"
    );
    const Body = JSON.parse(response.body);
    //Remove origin for comparison
    delete Body.origin;
    delete Body.headers["X-Amzn-Trace-Id"];

    expect(response.status).toBe(200);
    expect(Body).toStrictEqual(request.response);
  }
  cycleTLS.exit();
});
