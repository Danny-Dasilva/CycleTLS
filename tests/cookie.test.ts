import initCycleTLS from "../dist/index.js";
import { withCycleTLS } from "./test-utils.js";

test("Should Return 200", async () => {
  await withCycleTLS({ port: 9094 }, async (cycleTLS) => {
    const cookies = {
      cookie1: "value1",
      cookie2: "value2",
    };

    const response = await cycleTLS('https://httpbin.org/cookies', {
      body: '',
      cookies: cookies,
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    });

    const result = await response.json();
    expect(result.cookies).toEqual(cookies);
  });
});

test("Complex Cookie test", async () => {
  await withCycleTLS({ port: 9092 }, async (cycleTLS) => {
    const cookies = {
      cookie1: "value1",
      cookie2: "value2",
    };
    const complexCookies = [
      {
        name: "cookie1",
        value: "value1",
      },
      {
        name: "cookie2",
        value: "value2",
      },
    ];

    const response = await cycleTLS('https://httpbin.org/cookies', {
      cookies: complexCookies,
    });

    const result = await response.json();
    expect(result.cookies).toEqual(cookies);
  });
});
