
const initCycleTLS = require('../dist/index.js');
// import initCycleTLS from '../dist/index.js'
// Typescript: import initCycleTLS from 'cycletls';


(async () => {
    const cycleTLS = await initCycleTLS();

    // const response = cycleTLS('https://ja3er.com/json', {
    //   body: '',
    //   ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
    //   userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    //   proxy: ''
    // });

    // response.then((out) => {
    //   console.log(out, "aaaa")
    // })


    // const timeoutResponse = await cycleTLS("http://httpbin.org/delay/2", {
    //     body: "",
    //     ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0',
    //     userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    //     timeout: 1,

    // }, "get");

    // console.log(timeoutResponse.status)


    // const normalResponse = await cycleTLS("http://httpbin.org/delay/1", {
    //     body: "",
    //     ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0',
    //     userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
    //     timeout: 2,

    // }, "get");

    // console.log(normalResponse.status)
    // cycleTLS.exit()

})();

