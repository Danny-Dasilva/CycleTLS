import initCycleTLS from "../dist/index.js";

test("Should return a timeout error", async () => {
    const cycleTLS = await initCycleTLS({ port: 9116 });
    const ja3 =
        "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0";
    const userAgent =
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36";

    const deflateResponse = await cycleTLS(
        "https://httpbin.org/deflate",
        {
            body: "",
            ja3: ja3,
            userAgent: userAgent,
        },
        "get"
    );

    expect(deflateResponse.body["deflated"]).toBe(true);

    const brotliResponse = await cycleTLS(
        "http://httpbin.org/brotli",
        {
            body: "",
            ja3: ja3,
            userAgent: userAgent,
        },
        "get"
    );

    expect(brotliResponse.body["brotli"]).toBe(true);
    

    const gzipResponse = await cycleTLS(
        "http://httpbin.org/gzip",
        {
            body: "",
            ja3: ja3,
            userAgent: userAgent,
        },
        "get"
    );

    expect(gzipResponse.body["gzipped"]).toBe(true);

    cycleTLS.exit();
});
