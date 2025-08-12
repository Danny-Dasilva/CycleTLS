const initCycleTLS = require("../dist/index.js");
jest.setTimeout(30000);

let ja3 =
  "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
let userAgent =
  "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0";

var requestDict = {
  "https://httpbin.org/user-agent": {
    ja3: ja3,
    userAgent: userAgent,
  },
  "http://httpbin.org/post": {
    body: '{"field":"POST-VAL"}',
    method: "POST",
  },
  "http://httpbin.org/cookies": {
    cookies: [
      {
        name: "example1",
        value: "aaaaaaa",
        expires: "2022-01-02T15:04:05Z",
      },
    ],
  },
};

test("Multiple concurrent requests should complete successfully", async () => {
  const cycleTLS = await initCycleTLS({ port: 9150, timeout: 30000 });
  
  try {
    const promises = Object.entries(requestDict).map(([url, params]) => {
      return cycleTLS(
        url,
        {
          body: params.body ?? "",
          ja3: params.ja3 ?? ja3,
          userAgent: params.userAgent ?? userAgent,
          headers: params.headers,
          cookies: params.cookies,
          timeout: 30,
        },
        params.method ?? "GET"
      );
    });

    // Wait for all promises to resolve and check their status
    const results = await Promise.all(promises);

    // Verify each response
    results.forEach((response) => {
      expect(response.status).toBe(200);
    });
  } finally {
    await cycleTLS.exit();
  }
});

test("Sequential requests to same host should reuse connection", async () => {
  const cycleTLS = await initCycleTLS({ port: 9151, timeout: 30000 });
  
  try {
    // Make multiple requests to same domain
    const url = "https://httpbin.org";
    
    // First request
    const response1 = await cycleTLS.get(`${url}/get`, {
      ja3: ja3,
      userAgent: userAgent,
    });
    expect(response1.status).toBe(200);
    
    // Second request - should reuse connection
    const response2 = await cycleTLS.get(`${url}/get?second=true`, {
      ja3: ja3,
      userAgent: userAgent,
    });
    expect(response2.status).toBe(200);
    
    // Third request with different path but same domain - should still reuse connection
    const response3 = await cycleTLS.get(`${url}/headers`, {
      ja3: ja3,
      userAgent: userAgent,
    });
    expect(response3.status).toBe(200);
    
    // The connection reuse is happening at the Go level, and we can't directly test it from JS
    // But we can verify that all requests completed successfully
  } finally {
    await cycleTLS.exit();
  }
});
