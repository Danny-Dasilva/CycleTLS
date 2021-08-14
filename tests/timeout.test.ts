
import initCycleTLS from '../dist/index.js'

let ja3 = '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0'
let userAgent = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36'


test('Should return a timeout error', async () => {
    const cycleTLS = await initCycleTLS({ port: 9116 });

    const response = await cycleTLS("http://httpbin.org/delay/2", {
        body: "",
        ja3: ja3,
        userAgent: userAgent,
        timeout: 1,

    }, "get");

    expect(response.status).toBe(408)

    cycleTLS.exit()

});


test('Should return a 200 response', async () => {
    const cycleTLS = await initCycleTLS({ port: 9116 });

    const response = await cycleTLS("http://httpbin.org/delay/1", {
        body: "",
        ja3: ja3,
        userAgent: userAgent,
        timeout: 2,

    }, "get");

    expect(response.status).toBe(200)

    cycleTLS.exit()

});

