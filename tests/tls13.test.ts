import initCycleTLS from '../dist/index.js'


test('Should return a 200 response from tls 1.3', async () => {
  const cycleTLS = await initCycleTLS({ port: 9111 });

  const ja3 = '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0'
  const userAgent = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36'


  const redirectResponse = await cycleTLS("https://tls13.1d.pw", {
    body: "",
    ja3: ja3,
    userAgent: userAgent,
  }, "get");
  expect(redirectResponse.status).toBe(200)


  cycleTLS.exit()

});

