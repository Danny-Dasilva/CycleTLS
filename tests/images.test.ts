import CycleTLS from "../dist/index.js";
import { withCycleTLS, withUpstreamRetry, UPSTREAM_FLAKE_STATUSES } from "./test-utils.js";
import fs from "fs";
jest.setTimeout(60000);

// Skip-on-flake tracker — if httpbin is rate-limiting we won't have written
// fresh fixtures and the byte-compare in "Files should be the same" would
// fail spuriously, so we propagate the skip into the second test too.
let imageFlake = false;

let ja3 =
  "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
let userAgent =
  "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0";

async function fetchAndWrite(
  client: CycleTLS,
  url: string,
  outPath: string
): Promise<boolean> {
  const resp = await withUpstreamRetry(() =>
    client.get(url, { ja3: ja3, userAgent: userAgent })
  );
  if (UPSTREAM_FLAKE_STATUSES.has(resp.status)) {
    console.log(`Skipped ${url}: httpbin upstream flake (status ${resp.status})`);
    return false;
  }
  const buf = await resp.arrayBuffer();
  fs.writeFileSync(outPath, Buffer.from(buf));
  return true;
}

test("Should Write all Image types to file", async () => {
  await withCycleTLS({ port: 1111, timeout: 10000 }, async (client) => {
    const ok =
      (await fetchAndWrite(client, "http://httpbin.org/image/jpeg", "./tests/images/output.jpeg")) &&
      (await fetchAndWrite(client, "http://httpbin.org/image/png", "./tests/images/output.png")) &&
      (await fetchAndWrite(client, "http://httpbin.org/image/svg", "./tests/images/output.svg")) &&
      (await fetchAndWrite(client, "http://httpbin.org/image/webp", "./tests/images/output.webp"));
    if (!ok) {
      imageFlake = true;
    }
  });
});

test("Files should be the same", async () => {
  if (imageFlake) {
    console.log("Skipped: prior image-fetch flaked, skipping byte-compare");
    return;
  }
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
