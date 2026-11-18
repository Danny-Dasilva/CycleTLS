// Header and body values that are not strings (issue 338). Before 3.0.1 a
// number header value made that request and every later one on the client time
// out. Real binary, local server only.
const http = require("http");
const { initCycleTLS } = require("../dist/index.js");

jest.setTimeout(30000);

describe("legacy client: non-string header and body values", () => {
  let server;
  let url;
  let cycleTLS;
  let seen;

  beforeAll(async () => {
    server = http.createServer((req, res) => {
      const chunks = [];
      req.on("data", (chunk) => chunks.push(chunk));
      req.on("end", () => {
        seen.push({ rawHeaders: req.rawHeaders, headers: req.headers, body: Buffer.concat(chunks) });
        res.writeHead(200, { "Content-Type": "application/json" });
        res.end('{"ok":true}');
      });
    });
    await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
    url = `http://127.0.0.1:${server.address().port}/`;
    cycleTLS = await initCycleTLS({ port: 9745 });
  });

  afterAll(async () => {
    await cycleTLS.exit();
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(resolve));
  });

  beforeEach(() => {
    seen = [];
  });

  // Every value the server got for one header name, in order
  const valuesOf = (request, name) => {
    const values = [];
    for (let i = 0; i < request.rawHeaders.length; i += 2) {
      if (request.rawHeaders[i].toLowerCase() === name) values.push(request.rawHeaders[i + 1]);
    }
    return values;
  };

  test("number, boolean and bigint values arrive as their string form", async () => {
    const response = await cycleTLS.get(url, {
      timeout: 5,
      headers: { "x-int": 1, "x-float": 1.5, "x-zero": 0, "x-bool": false, "x-big": 10n, "x-str": "plain" },
    });
    expect(response.status).toBe(200);
    expect(seen).toHaveLength(1);
    expect(seen[0].headers["x-int"]).toBe("1");
    expect(seen[0].headers["x-float"]).toBe("1.5");
    expect(seen[0].headers["x-zero"]).toBe("0");
    expect(seen[0].headers["x-bool"]).toBe("false");
    expect(seen[0].headers["x-big"]).toBe("10");
    expect(seen[0].headers["x-str"]).toBe("plain");
  });

  test("array values are converted element by element and stay multi-value", async () => {
    const response = await cycleTLS.get(url, {
      timeout: 5,
      headers: { "x-multi": ["one", 2, true], "x-strings": ["a", "b"] },
    });
    expect(response.status).toBe(200);
    expect(valuesOf(seen[0], "x-multi")).toEqual(["one", "2", "true"]);
    expect(valuesOf(seen[0], "x-strings")).toEqual(["a", "b"]);
  });

  test.each([
    ["an object", { a: 1 }, /Invalid value for header "x-bad".*got object/],
    ["undefined", undefined, /Invalid value for header "x-bad".*got undefined/],
    ["null", null, /Invalid value for header "x-bad".*got null/],
    ["an array holding an object", ["ok", {}], /Invalid value for header "x-bad".*got object/],
    ["a nested array", [["a"]], /Invalid value for header "x-bad".*got a nested array/],
  ])("%s as a header value rejects before anything is sent", async (_label, value, pattern) => {
    const startedAt = Date.now();
    const error = await cycleTLS.get(url, { timeout: 5, headers: { "x-bad": value } }).then(
      () => null,
      (e) => e
    );
    expect(error).toBeInstanceOf(TypeError);
    expect(error.message).toMatch(pattern);
    expect(Date.now() - startedAt).toBeLessThan(1000);
    expect(seen).toHaveLength(0);

    // The client is still usable
    const response = await cycleTLS.get(url, { timeout: 5 });
    expect(response.status).toBe(200);
  });

  test("an object body rejects with a clear error and does not poison the client", async () => {
    await expect(cycleTLS.post(url, { timeout: 5, body: { a: 1 } })).rejects.toThrow(/Invalid body/);
    expect(seen).toHaveLength(0);
    const response = await cycleTLS.post(url, { timeout: 5, body: "still works" });
    expect(response.status).toBe(200);
    expect(seen[0].body.toString()).toBe("still works");
  });

  test("bodyBytes is sent as raw bytes", async () => {
    const bytes = Buffer.from([0x00, 0xff, 0x80, 0x0a, 0xc3, 0x28]);
    const response = await cycleTLS.post(url, { timeout: 5, bodyBytes: new Uint8Array(bytes) });
    expect(response.status).toBe(200);
    expect(seen[0].body.equals(bytes)).toBe(true);
  });
});
