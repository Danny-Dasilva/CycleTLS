const initCycleTLS = require("../dist/index.js");
var fs = require("fs");
let ja3 =
  "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
let userAgent =
  "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0";
const writeImage = (filename, data) => {
  let writeStream = fs.createWriteStream(filename);

  // write some data with a base64 encoding
  writeStream.write(data, "base64");
  writeStream.on("finish", () => {
    console.log(`wrote to file ${filename}`);
  });

  // close the stream
  writeStream.end();
};
(async () => {
  const cycleTLS = await initCycleTLS({ port: 1111 });
  // try {

  const jpegImage = await cycleTLS("http://httpbin.org/image/jpeg", {
    body: "",
    ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
    userAgent:
      "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
    proxy: "",
  });
  writeImage("test.jpeg", jpegImage.body);

  const pngImage = await cycleTLS('https://httpbin.org/image/png', {
    body: '',
    ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0',
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36'
  }, 'get');

  pngImage.body.pipe(fs.createWriteStream('image.png'));

  const svgImage = await cycleTLS(
    "http://httpbin.org/image/svg",
    {
      ja3: ja3,
      userAgent: userAgent,
    },
    "get"
  );

  writeImage("test.svg", svgImage.body);

  const webpImage = await cycleTLS(
    "http://httpbin.org/image/webp",
    {
      ja3: ja3,
      userAgent: userAgent,
    },
    "get"
  );

  writeImage("test.webp", webpImage.body);

  await cycleTLS.exit();
})();
