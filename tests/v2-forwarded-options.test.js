/**
 * Default client: every request option reaches the Go server.
 *
 * The client copies options by a list (FORWARDED_REQUEST_OPTIONS). These
 * tests check the list against the RequestOptions type and check what is
 * actually put on the wire, using a stand-in WebSocket server that records
 * the init packet. Nothing leaves the machine.
 */
const fs = require("fs");
const path = require("path");
const WebSocket = require("ws");
const { loadPackage, loadClientModule, parseInitPacket } = require("./v2-helpers");

const CycleTLS = loadPackage();
const clientModule = loadClientModule();

/** Property names declared directly on `export interface <name>` in the source. */
function interfaceKeys(name) {
  const source = fs.readFileSync(
    path.join(__dirname, "..", "src", "flow-control-client.ts"),
    "utf8"
  );
  const start = source.indexOf(`export interface ${name} {`);
  expect(start).toBeGreaterThan(-1);
  const end = source.indexOf("\n}\n", start);
  const body = source.slice(start, end);
  // Direct members are indented by exactly two spaces.
  return [...body.matchAll(/^ {2}(\w+)\??:/gm)].map((m) => m[1]);
}

describe("forwarded option list", () => {
  const forwarded = clientModule.FORWARDED_REQUEST_OPTIONS;
  const clientOnly = clientModule.CLIENT_ONLY_REQUEST_OPTIONS;

  test("the lists are exported", () => {
    expect(Array.isArray(forwarded)).toBe(true);
    expect(Array.isArray(clientOnly)).toBe(true);
  });

  test("every RequestOptions key is forwarded or marked client-only", () => {
    const keys = interfaceKeys("RequestOptions");
    expect(keys.length).toBeGreaterThan(20);
    const listed = new Set([...forwarded, ...clientOnly]);
    expect(keys.filter((k) => !listed.has(k))).toEqual([]);
  });

  test("the lists name nothing that RequestOptions lacks, and do not overlap", () => {
    const keys = new Set(interfaceKeys("RequestOptions"));
    expect([...forwarded, ...clientOnly].filter((k) => !keys.has(k))).toEqual([]);
    expect(forwarded.filter((k) => clientOnly.includes(k))).toEqual([]);
    expect(new Set(forwarded).size).toBe(forwarded.length);
  });

  test.each([
    "tlsProfile",
    "shuffleExtensions",
    "clientHello",
    "sessionResumption",
    "caCerts",
    "certPins",
    "http2Priority",
    "proxyHeaders",
    "tls13AutoRetry",
    "bodyBytes",
    "serverName",
    "http2Fingerprint",
    "quicFingerprint",
  ])("%s is forwarded", (name) => {
    expect(forwarded).toContain(name);
  });
});

describe("options on the wire", () => {
  const PORT = 39541;
  let wss;
  let packets;
  let client;

  // One value for every forwarded option, in the shape Go expects.
  const everything = {
    url: "https://example.test/path",
    method: "POST",
    headers: { "X-One": "1", "x-many": ["a", "b"] },
    body: "payload",
    ja3: "771,4865-4866,0-23,29-23,0",
    ja4r: "t13d1516h2_002f,0035_0005,000a_0403",
    userAgent: "agent/1.0",
    proxy: "http://127.0.0.1:1",
    timeout: 4321,
    disableRedirect: true,
    insecureSkipVerify: true,
    proxyInsecureSkipVerify: false,
    forceHTTP1: true,
    forceHTTP3: false,
    protocol: "http3",
    http2Fingerprint: "1:65536;2:0|15663105|0|m,a,s,p",
    quicFingerprint: "quic-fp",
    disableGrease: true,
    serverName: "sni.example.test",
    headerOrder: ["x-one", "x-many"],
    orderAsProvided: true,
    cookies: [{ name: "c", value: "v", path: "/", domain: "example.test" }],
    tls13AutoRetry: false,
    enableConnectionReuse: true,
    tlsProfile: "chrome_133",
    shuffleExtensions: true,
    clientHello: "16030100",
    sessionResumption: true,
    caCerts: ["-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----"],
    certPins: ["sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="],
    http2Priority: { weight: 256, exclusive: true, streamDep: 0 },
    proxyHeaders: { "Proxy-Authorization": "Basic eDp5" },
  };

  /** Run one call against the stand-in server and return the init packet it got. */
  async function capture(call) {
    packets.length = 0;
    await call().catch(() => {}); // the stand-in closes without answering
    expect(packets).toHaveLength(1);
    return parseInitPacket(packets[0]);
  }

  beforeAll(async () => {
    packets = [];
    wss = new WebSocket.Server({ port: PORT, host: "127.0.0.1" });
    await new Promise((r) => wss.on("listening", r));
    wss.on("connection", (socket) => {
      socket.on("message", (data) => {
        packets.push(Buffer.from(data));
        socket.close();
      });
    });
    client = new CycleTLS({ port: PORT, timeout: 5000 });
  });

  afterAll(async () => {
    await client.close();
    for (const socket of wss.clients) socket.terminate();
    await new Promise((r) => wss.close(r));
  });

  test("the test covers every forwarded option", () => {
    const covered = new Set([...Object.keys(everything), "bodyBytes"]);
    expect(
      clientModule.FORWARDED_REQUEST_OPTIONS.filter((k) => !covered.has(k))
    ).toEqual([]);
  });

  test("request() sends every option unchanged", async () => {
    const { options, method } = await capture(() => client.request(everything));
    expect(method).toBe("init");
    expect(options).toEqual(everything);
  });

  test("bodyBytes is sent as base64", async () => {
    const bytes = Uint8Array.from([0, 1, 2, 250, 255]);
    const { options } = await capture(() =>
      client.request({ url: everything.url, method: "PUT", bodyBytes: bytes })
    );
    expect(options.bodyBytes).toBe(Buffer.from(bytes).toString("base64"));
    expect(options.body).toBe("");
  });

  test("bodyBytes that is a view into a larger buffer sends only the view", async () => {
    const backing = Buffer.from("xxHELLOxx");
    const view = new Uint8Array(backing.buffer, backing.byteOffset + 2, 5);
    const { options } = await capture(() =>
      client.request({ url: everything.url, bodyBytes: view })
    );
    expect(Buffer.from(options.bodyBytes, "base64").toString()).toBe("HELLO");
  });

  test("unset options are left out and defaults are filled in", async () => {
    const { options } = await capture(() => client.request({ url: everything.url }));
    expect(options).toEqual({
      url: everything.url,
      method: "GET",
      headers: {},
      body: "",
      timeout: 5000,
    });
  });

  test("client-only options are not sent", async () => {
    const { options } = await capture(() =>
      client.request({ url: everything.url, readTimeout: 111, sseTimeout: 222 })
    );
    expect(options).not.toHaveProperty("readTimeout");
    expect(options).not.toHaveProperty("sseTimeout");
  });

  test("timeout is sent as whole milliseconds, -1 for none", async () => {
    const none = await capture(() => client.request({ url: everything.url, timeout: -1 }));
    expect(none.options.timeout).toBe(-1);
    const fractional = await capture(() =>
      client.request({ url: everything.url, timeout: 1500.4 })
    );
    expect(fractional.options.timeout).toBe(1501);
    const zero = await capture(() => client.request({ url: everything.url, timeout: 0 }));
    expect(zero.options.timeout).toBe(30000);
  });

  test("ws() forwards the fingerprint options and fixes method, body and protocol", async () => {
    const { url, ...rest } = everything;
    const { options } = await capture(() => client.ws("wss://example.test/socket", rest));
    expect(options).toEqual({
      ...everything,
      url: "wss://example.test/socket",
      method: "GET",
      body: "",
      protocol: "websocket",
    });
  });

  test("sse() forwards the options, adds its headers and sets the protocol", async () => {
    const { url, ...rest } = everything;
    const { options } = await capture(() => client.sse("https://example.test/events", rest));
    expect(options).toEqual({
      ...everything,
      url: "https://example.test/events",
      protocol: "sse",
      headers: {
        Accept: "text/event-stream",
        "Cache-Control": "no-cache",
        ...everything.headers,
      },
    });
  });
});
