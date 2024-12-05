const fs = require("fs");
const initCycleTLS = require('../dist/index.js');
// import initCycleTLS from '../dist/index.js'
// Typescript: import initCycleTLS from 'cycletls';

(async () => {
  const cycleTLS = await initCycleTLS();

  {
    const response = await cycleTLS('https://localhost:8080/music.mp3', {
      body: '',
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      proxy: ''
    });

    response.stream.pipe(fs.createWriteStream("output_cancel.mp3"));
    response.stream.destroy();
  }

  {
    const response = await cycleTLS('https://localhost:8080/music.mp3', {
      body: '',
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
      proxy: ''
    });

    response.stream.pipe(fs.createWriteStream("output.mp3"));
  }
})();