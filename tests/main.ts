// const initCycleTLS = require('./src/index');
import initCycleTLS from '../dist/index.js'
// Typescript: import initCycleTLS from 'cycletls';

(async () => {
  const cycleTLS = await initCycleTLS();
  for (let i = 0; i < 10; i++) {
    const response = await cycleTLS('http://localhost:8080', {
      id: i,
    });
    console.log(response)

  }
 
})();
