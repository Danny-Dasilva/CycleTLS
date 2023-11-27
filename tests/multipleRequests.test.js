const initCycleTLS = require("../dist/index.js");
jest.setTimeout(30000);

let ja3 =
  "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
let userAgent =
  "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0";
var requestDict = {
  //three urls used as an examle
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
        expires: "Mon, 02-Jan-2022 15:04:05 EST",
      },
    ],
  },
};

const promises = [];

test("Multiple Request example", async () => {
  const cycleTLS = await initCycleTLS({ port: 9117 });

  for (const url in requestDict) {
    const params = requestDict[url]; //get request params

    const response = cycleTLS(
      url,
      {
        body: params.body ?? "", //?? is just setting defaults in this case
        ja3: params.ja3 ?? ja3,
        userAgent: params.userAgent ?? userAgent,
        headers: params.headers,
        cookies: params.cookies,
        timeout: 20,
      },
      params.method ?? "GET"
    );

    response.then((out) => {
      expect(out.status).toBe(200); //Process request
    });

    promises.push(response); //Add request to promises array
  }

  Promise.all(promises).then(async () => {
    await cycleTLS.exit();
  }); //Check for all requests to process then exit
});
