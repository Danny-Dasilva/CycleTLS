// const initCycleTLS = require('./src/index');
import initCycleTLS from '../dist/index.js'
const { performance } = require('perf_hooks');
// Typescript: import initCycleTLS from 'cycletls';

(async () => {
  const cycleTLS = await initCycleTLS();
  let i = 0
  const before = Date.now();
  for (let i = 0; i < 1; i++) {
    var t0 = performance.now()

    const response = cycleTLS('http://httpbin.org/cookies', {
      body: '',
      ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0',
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36',
      headers:  {
        "Authorization": "Bearer someexampletoken",
      },
      cookies: [
        {
          "name": "Standard",
          "value": "aaaaaaa",
          "time" : "Mon, 02-Jan-2006 15:04:05 MST"
        },
        {
          "name": "AHHH",
          "value": "kill me",
          "time" : "Tue, 06-Jan-2019 2:03:05 MST"
        }]

    });

    response.then((out) => {
      console.log(out)
      i++
      var t1 = performance.now()
      console.log("Call to doSomething took " + (t1 - t0) + " milliseconds.", i)
    })
    
  }
 
})();