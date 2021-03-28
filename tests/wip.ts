// const initMyTls = require('./src/index');
import initMyTls from '../dist/index.js'
// Typescript: import initMyTls from 'mytls';

(async () => {
  const myTls = await initMyTls();

  const response = await myTls('https://api.ipify.org/?format=json', {
    body: '',
    headers: {
      'user-agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36',
    },
    ja3: '771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0',
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36',
    proxy: '',
  });
  console.log(response)
})();
