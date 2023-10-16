const fs = require('fs');

const formData = {
  fields: [
    { name: 'field1', value: 'value1' },
    { name: 'field2', value: 'value2' }
  ],
  files: [
    {
      name: 'file',
      filename: 'file.txt',
      contentType: 'text/plain',
      data: fs.readFileSync('main.ts', { encoding: 'base64' })
    }
  ]
};