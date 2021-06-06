const initCycleTLS = require("cycletls");
// Typescript: import initCycleTLS from 'cycletls';

let ja3 = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0";
let userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36";
var requestDict = { //three urls used as an examle
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

    const cycleTLS = await initCycleTLS({port: 9117});

    for (const url in requestDict) {
      const params = requestDict[url]; //get request params

      const response = cycleTLS(
        url, {
          body: params.body ?? "", //?? is just setting defaults in this case
          ja3: params.ja3 ?? ja3,
          userAgent: params.userAgent ?? userAgent,
          headers: params.headers,
          cookies: params.cookies,
        }, params.method ?? "GET");

      response.then((out) => {
        expect(out.status).toBe(200); //Process request
      });

      promises.push(response); //Add request to promises array
    }

    Promise.all(promises).then(async () => {
      await cycleTLS.exit();
    }); //Check for all requests to process then exit
});
