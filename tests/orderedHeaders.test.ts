import initCycleTLS from "../dist/index.js";
const jsdom = require("jsdom");

const ja3 =
  "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0";
const userAgent =
  "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36";

function parseBody(html: any) {
  if (typeof html === "string") {
    const dom = new jsdom.JSDOM(html);
    const query = dom.window.document.querySelectorAll(".headername nobr"); // content
    let queryValues: string[] = [];
    for (let i = 0; i < query.length; i++) {
      const inner = query[i].innerHTML.split(":")[0];
      queryValues.push(inner);
    }
    return queryValues;
  }
  return null;
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
  if (typeof headerRequest.body === "object") {
    expect(headerRequest.body?.headers?.Foo).toStrictEqual("bar");
  }
  cycleTLS.exit();
});
