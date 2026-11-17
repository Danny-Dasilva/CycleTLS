import CycleTLS from "../dist/index.js";
import { withCycleTLS } from "./test-utils.js";
import http from 'http';

// Check if httpbin.org is reachable before running tests
let httpbinAvailable = true;
beforeAll(async () => {
  try {
    await new Promise<void>((resolve, reject) => {
      const req = http.get('http://httpbin.org/get', { timeout: 5000 }, (res) => {
        res.resume();
        resolve();
      });
      req.on('error', () => reject());
      req.on('timeout', () => { req.destroy(); reject(); });
    });
  } catch {
    httpbinAvailable = false;
    console.warn('httpbin.org is not reachable - skipping cookie tests');
  }
});

test("Should Return 200", async () => {
  if (!httpbinAvailable) {
    console.warn('Skipping: httpbin.org is not reachable');
    return;
  }
  await withCycleTLS({ port: 9094 }, async (client) => {
    const cookies = {
      cookie1: "value1",
      cookie2: "value2",
    };

    const response = await client.get('https://httpbin.org/cookies', {
      cookies: [
        { name: 'cookie1', value: 'value1' },
        { name: 'cookie2', value: 'value2' },
      ],
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    });

    const result = await response.json() as { cookies: Record<string, string> };
    expect(result.cookies).toEqual(cookies);
  });
});

test("Complex Cookie test", async () => {
  if (!httpbinAvailable) {
    console.warn('Skipping: httpbin.org is not reachable');
    return;
  }
  await withCycleTLS({ port: 9092 }, async (client) => {
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

    const response = await client.get('https://httpbin.org/cookies', {
      cookies: complexCookies,
    });

    const result = await response.json() as { cookies: Record<string, string> };
    expect(result.cookies).toEqual(cookies);
  });
});
