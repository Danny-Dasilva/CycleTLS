import initCycleTLS from "../dist/index.js";
jest.setTimeout(30000);

let ja3 =
  "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
let userAgent =
  "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0";
var fs = require("fs");
const writeImage = (filename: string, data: string) => {
  let writeStream = fs.createWriteStream(filename);

  // write some data with a base64 encoding
  writeStream.write(data, "base64");
  writeStream.on("finish", () => {});

  // close the stream
  writeStream.end();
};

test("Should Write all Image types to file", async () => {
  const cycleTLS = await initCycleTLS({ port: 1111 });

  const jpegImage = await cycleTLS(
    "http://httpbin.org/image/jpeg",
    {
      ja3: ja3,
      userAgent: userAgent,
    },
    "get"
  );
  writeImage("./tests/images/output.jpeg", jpegImage.body);

  const pngImage = await cycleTLS(
    "http://httpbin.org/image/png",
    {
      ja3: ja3,
      userAgent: userAgent,
    },
    "get"
  );

  writeImage("./tests/images/output.png", pngImage.body);

  const svgImage = await cycleTLS(
    "http://httpbin.org/image/svg",
    {
      ja3: ja3,
      userAgent: userAgent,
    },
    "get"
  );

  writeImage("./tests/images/output.svg", svgImage.body);

  const webpImage = await cycleTLS(
    "http://httpbin.org/image/webp",
    {
      ja3: ja3,
      userAgent: userAgent,
    },
    "get"
  );

  await writeImage("./tests/images/output.webp", webpImage.body);

  cycleTLS.exit();
});

test("Files should be the same", async () => {
  //Wait for files to write, probably a better way to do this
  await new Promise((r) => setTimeout(r, 1000));
  const compareFiles = (file1: string, file2: string) => {
    const tmpBuf = fs.readFileSync(`./tests/images/${file1}`);
    const testBuf = fs.readFileSync(`./tests/images/${file2}`);
    return tmpBuf.equals(testBuf);
  };

  expect(compareFiles("test.jpeg", "output.jpeg")).toBe(true);
  expect(compareFiles("test.png", "output.png")).toBe(true);
  if (process.platform != "win32") {
    expect(compareFiles("test.svg", "output.svg")).toBe(true);
  }
  expect(compareFiles("test.webp", "output.webp")).toBe(true);
});
