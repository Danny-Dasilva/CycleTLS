const initCycleTLS = require("../dist/index.js");

test("Test Multiple Imports, should return 200", async () => {
  const url = "https://ja3er.com/json";
  const options = {
    body: "",
    ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
    userAgent:
      "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
  };
  //Request the same CycleTLS instance
  const cycleTLS = await initCycleTLS({ port: 9110 });
  const cycleTLS2 = await initCycleTLS({ port: 9110 });

  let resp;

  resp = await cycleTLS.get(url, options);
  expect(resp.status).toBe(200);
  resp = await cycleTLS2.get(url, options);
  expect(resp.status).toBe(200);

  await cycleTLS2.exit();
  await cycleTLS.exit();
});
