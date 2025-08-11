const initCycleTLS = require("../dist/index.js");

(async () => {
  const cycleTLS = await initCycleTLS({ port: 9118 });

  const tls12JA4r = "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601";
    const ja4rResponse = await cycleTLS.get('https://tls.peet.ws/api/all', {
      ja4r: tls12JA4r,
      disableGrease: false,
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
    });
    const response = await ja4rResponse.json()
  console.log(response, "hello")
 

  await cycleTLS.exit();
})();