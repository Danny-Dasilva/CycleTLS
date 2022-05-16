import initCycleTLS from "../dist/index.js";

let ja3 =
  "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0";
let userAgent =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36";

var myDictionary = {
  "https://httpbin.org/user-agent": {
    ja3: ja3,
    userAgent: userAgent,
  },
  "http://httpbin.org/post": {
    body: '{"field":"POST-VAL"}',
    method: "POST",
  },
  "http://httpbin.org/put": {
    body: '{"field":"PUT-VAL"}',
    method: "PUT",
  },
  "http://httpbin.org/patch": {
    body: '{"field":"PATCH-VAL"}',
    method: "PATCH",
  },
  "http://httpbin.org/delete": {
    body: '{"field":"DELETE-VAL"}',
    method: "Delete",
  },
  "http://httpbin.org/headers": {
    //returns headers in body
    headers: { Authorization: "Bearer someexampletoken" },
  },
  "http://httpbin.org/cookies": {
    cookies: [
      {
        name: "example1",
        value: "aaaaaaa",
        expires: "Mon, 02-Jan-2022 15:04:05 EST",
      },
      {
        name: "example2",
        value: "bbbbbbbbb",
        expires: "Tue, 06-Jan-2022 2:03:05 EST",
      },
    ],
  },
  "http://httpbin.org/ip": {}, //log ip
  "http://httpbin.org/response-headers?ExampleResponseHeader=HeaderisPickedUpByServer":
    {}, //log response headers
  "http://httpbin.org/html": {}, //log html
  "http://httpbin.org/delay/3": {}, //this request will wait 3 s before returning so it should always show up last in the log
};

(async () => {
  const cycleTLS = await initCycleTLS();

  for (let key in myDictionary) {
    let value = myDictionary[key];

    let body = value.body ?? "";
    let method = value.method ?? "GET";
    let ja3Token = value.ja3 ?? ja3;
    let Agent = value.userAgent ?? userAgent;
    let cookies = value.cookies;
    const response = cycleTLS(
      key,
      {
        body: body,
        ja3: ja3Token,
        userAgent: Agent,
        headers: value.headers,
        cookies: cookies,
      },
      method
    );

    response.then((out) => {
      console.log(key, out);
    });
  }
})();
