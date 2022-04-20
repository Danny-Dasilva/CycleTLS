import initCycleTLS from "../dist/index.js";
jest.setTimeout(30000);

test("Should return a timeout error", async () => {
  const cycleTLS = await initCycleTLS({ port: 9116 });
  const ja3 =
    "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0";
  const userAgent =
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36";

  const timeoutResponse = await cycleTLS(
    "https://httpbin.org/delay/4",
    {
      body: "",
      ja3: ja3,
      userAgent: userAgent,
      timeout: 1,
    },
    "get"
  );

  expect(timeoutResponse.status).toBe(408);

  const normalResponse = await cycleTLS(
    "https://httpbin.org/delay/1",
    {
      body: "",
      ja3: ja3,
      userAgent: userAgent,
      timeout: 30,
    },
    "get"
  );

  expect(normalResponse.status).toBe(200);

  cycleTLS.exit();
});
