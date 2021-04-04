// const initCycleTLS = require('./src/index');
import initCycleTLS from '../dist/index.js'
const {performance} = require('perf_hooks');
// Typescript: import initCycleTLS from 'cycletls';

// (async () => {
//   const cycleTLS = await initCycleTLS();

//   const response = await cycleTLS('http://localhost:8080', {
//     body: '',
//     ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0',
//     userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36',

//   });
//   console.log(response)
// })();



function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}
async function delayedLog(item, cycleTLS) {

  const response = await cycleTLS('http://localhost:8080', {
    body: '',
    ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0',
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36',

  });
  console.log(item)
}
async function processArray(array) {
  var t0 = performance.now()
  const cycleTLS = await initCycleTLS();

  for (const item of array) {
    await delayedLog(item, cycleTLS);

  }
  var t1 = performance.now()

  console.log("Call to processArray took " + (t1 - t0) + " milliseconds.")
  console.log('Done!');
  // return 
}

processArray(Array(1000).fill(null).map((_, i) => i));

