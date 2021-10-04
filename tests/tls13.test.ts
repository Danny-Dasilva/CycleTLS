import initCycleTLS from '../dist/index.js'


test('Should return a 200 response from tls 1.3', async () => {
  const cycleTLS = await initCycleTLS({ port: 9111 });

  const ja3 = '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0'
  const userAgent = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:92.0) Gecko/20100101 Firefox/92.0'


  const redirectResponse = await cycleTLS("https://tls13.1d.pw", {
    body: "",
    ja3: ja3,
    userAgent: userAgent,
  }, "get");
  expect(redirectResponse.status).toBe(200)


  cycleTLS.exit()

});

