import initCycleTLS, { CycleTLSClient } from "../dist/index.js";

describe("CycleTLS HTTP Version Tests", () => {
  let cycleTLS: CycleTLSClient;
  let ja3 = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0";
  let userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36";

  beforeAll(async () => {
    cycleTLS = await initCycleTLS({ port: 9134 });
  });

  afterAll(() => {
    cycleTLS.exit();
  });

  test("Should use HTTP/2 by default", async () => {
    const response = await cycleTLS(
      "https://tls.peet.ws/api/all",
      {
        body: "",
        ja3: ja3,
        userAgent: userAgent,
        forceHTTP1: false,
      },
      "get"
    );

    expect(response.status).toBe(200);
    let fullResp = response.body as { http_version: string };
    expect(fullResp.http_version).toBe("h2");
  });

  test("Should force HTTP/1.1 when specified", async () => {
    const response = await cycleTLS(
      "https://tls.peet.ws/api/all",
      {
        body: "",
        ja3: ja3,
        userAgent: userAgent,
        forceHTTP1: true,
      },
      "get"
    );

    expect(response.status).toBe(200);
    let fullResp = response.body as { http_version: string };
    expect(fullResp.http_version).toBe("HTTP/1.1");
  });
});
