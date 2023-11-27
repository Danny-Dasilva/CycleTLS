const initCycleTLS = require("../dist/index.js");
const FormData = require('form-data');
const fs = require('fs');

(async () => {
  const cycleTLS = await initCycleTLS();

  const formData = new FormData();
  const fileStream = fs.createReadStream("../go.mod");
  formData.append('file', fileStream);

  
  const response = await cycleTLS('http://httpbin.org/post', {
      body: formData,
      headers: {
          'Content-Type': 'multipart/form-data',
      },
  }, 'post');

  console.log(response);

  cycleTLS.exit();
})();