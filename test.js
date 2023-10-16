const fs = require('fs');

const filePath = './test.go';

fs.readFile(filePath, (err, data) => {
  if (err) {
    // handle error
    return;
  }
  
  // encode the file contents as a base64 string
  const base64Data = Buffer.from(data).toString('base64');
  console.log(base64Data);
});

console.log(fs.readFileSync(filePath, { encoding: 'base64' }))