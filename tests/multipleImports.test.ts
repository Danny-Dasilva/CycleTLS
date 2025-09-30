import initCycleTLS from "../dist/index.js";
import { createSafeCycleTLS, cleanupCycleTLS } from "./test-utils.js";

jest.setTimeout(30000);

test("Test Multiple Imports, should return 200", async () => {
  const url = "https://httpbin.org/get";
  const options = {
    body: "",
    ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
    userAgent:
      "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
  };

  // Test multiple imports by using the same port - second should connect as client
  const cycleTLS = await createSafeCycleTLS({ port: 9161, timeout: 30000 });
  const cycleTLS2 = await createSafeCycleTLS({ port: 9161, timeout: 30000 });

  try {
    // Both instances should work
    const resp1 = await cycleTLS.get(url, options);
    expect(resp1.status).toBe(200);

    const resp2 = await cycleTLS2.get(url, options);
    expect(resp2.status).toBe(200);
  } finally {
    await cleanupCycleTLS(cycleTLS2);
    await cleanupCycleTLS(cycleTLS);
  }
});
