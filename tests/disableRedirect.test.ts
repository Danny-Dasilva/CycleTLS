
import initCycleTLS from '../dist/index.js'

let ja3 = '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0'
let userAgent = 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0'

test('Should return a 301 redirect', async () => {
    const cycleTLS = await initCycleTLS({ port: 9114 });

    const response = await cycleTLS("https://google.com", {
        body: "",
        ja3: ja3,
        userAgent: userAgent,
        disableRedirect: true,

    }, "get");
    expect(response.status).toBe(301)

    cycleTLS.exit()

});


test('Should return a 200 response with no redirect', async () => {
    const cycleTLS = await initCycleTLS({ port: 9113 });

    const response = await cycleTLS("https://google.com", {
        body: "",
        ja3: ja3,
        userAgent: userAgent,
        disableRedirect: false,

    }, "get");

    expect(response.status).toBe(200)

    cycleTLS.exit()

});

