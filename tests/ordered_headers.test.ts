import initCycleTLS from "../dist/index.js";
const jsdom = require("jsdom");

const ja3 =
  "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
const userAgent =
  "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0";

function parseBody(html: string) {
  const dom = new jsdom.JSDOM(html);
  const query = dom.window.document.querySelectorAll(".headername nobr"); // content
  let queryValues: string[] = [];
  for (let i = 0; i < query.length; i++) {
    const inner = query[i].innerHTML.split(":")[0];
    queryValues.push(inner);
  }
  return queryValues;
}

test("Should correctly set header order", async () => {
  const cycleTLS = await initCycleTLS({ port: 9120 });

  const headerOrder = ["User-Agent", "Cache-Control", "Connection", "Host"];
  const defaultHeaders = await cycleTLS(
    "https://pgl.yoyo.org/http/browser-headers.php",
    {
      ja3: ja3,
      userAgent: userAgent,
      headers: {
        host: "pgl.yoyo.org",
        connection: "keep-alive",
        "cache-control": "no-cache",
      },
      proxy: "",
    }
  );
  let responseOrder = parseBody(defaultHeaders.body);
  expect(responseOrder).toStrictEqual(headerOrder);

  //Test custom header order
  const customHeaderOrder = [
    "User-Agent",
    "Cache-Control",
    "Connection",
    "Host",
  ];
  const customHeaders = await cycleTLS(
    "https://pgl.yoyo.org/http/browser-headers.php",
    {
      ja3: ja3,
      userAgent: userAgent,
      headers: {
        host: "pgl.yoyo.org",
        connection: "keep-alive",
        "cache-control": "no-cache",
      },
      headerOrder: customHeaderOrder,
      proxy: "",
    }
  );
  responseOrder = parseBody(customHeaders.body);
  const expectedresponseOrder = [
    "User-Agent",
    "Host",
    "Connection",
    "Cache-Control",
  ];

  expect(responseOrder).toStrictEqual(expectedresponseOrder);

  //Test Setting headers
  const headerRequest = await cycleTLS("https://httpbin.org/headers", {
    ja3: ja3,
    userAgent: userAgent,
    headers: { foo: "bar" },
    proxy: "",
  });
  expect(JSON.parse(headerRequest.body).headers.Foo).toStrictEqual("bar");

  cycleTLS.exit();
});
