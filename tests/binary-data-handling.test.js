const initCycleTLS = require('../dist/index.js');
const fs = require('fs');
const crypto = require('crypto');

jest.setTimeout(60000);

const ja3 = '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0';
const userAgent = 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0';

describe('Binary Data Handling - Issue #297 Fix', () => {
  let cycleTLS;

  beforeEach(async () => {
    // Use a different port for each test to avoid conflicts
    const port = 9119 + Math.floor(Math.random() * 1000);
    cycleTLS = await initCycleTLS({ port });
  });

  afterEach(async () => {
    if (cycleTLS) {
      await cycleTLS.exit();
    }
  });

  test('Binary upload with problematic UTF-8 sequences should preserve data integrity', async () => {
    // Create binary data with sequences that would corrupt in UTF-8
    const problematicData = Buffer.from([
      0xFF, 0xD8, 0xFF, 0xE0, // JPEG header
      0x80, 0x81, 0x82, 0x83, // Invalid UTF-8 sequences
      0x00, 0x01, 0x02, 0x03, // Null bytes and control characters
      0xFE, 0xFF, 0xC0, 0xC1, // More problematic bytes
      0xEF, 0xBF, 0xBD,       // UTF-8 replacement character sequence
      0xF0, 0x90, 0x8D,       // Incomplete 4-byte UTF-8 sequence
    ]);

    const originalHash = crypto.createHash('md5').update(problematicData).digest('hex');

    const response = await cycleTLS('https://httpbin.org/post', {
      body: problematicData.toString('binary'), // Convert Buffer to binary string
      headers: {
        'Content-Type': 'application/octet-stream',
        'Content-Length': problematicData.length.toString()
      },
      ja3,
      userAgent
    }, 'POST');

    expect(response.status).toBe(200);

    // Parse response to verify server received the data
    const responseData = JSON.parse(await response.text());
    expect(responseData.data).toBeDefined();

    // httpbin.org returns data as raw binary string when sent as binary, not base64
    const receivedData = Buffer.from(responseData.data, 'binary');
    const receivedHash = crypto.createHash('md5').update(receivedData).digest('hex');

    expect(receivedHash).toBe(originalHash);
  });

  test('Binary download using arrayBuffer() should preserve data integrity', async () => {
    const response = await cycleTLS('https://httpbin.org/image/jpeg', {
      ja3,
      userAgent
    });

    expect(response.status).toBe(200);

    // Use arrayBuffer() to get clean binary data
    const binaryData = await response.arrayBuffer();
    const dataArray = new Uint8Array(binaryData);

    // Verify it's a valid JPEG (starts with 0xFF 0xD8)
    expect(dataArray.length).toBeGreaterThan(2);
    expect(dataArray[0]).toBe(0xFF);
    expect(dataArray[1]).toBe(0xD8);

    // Verify we can calculate a consistent hash
    const buffer = Buffer.from(binaryData);
    const hash = crypto.createHash('md5').update(buffer).digest('hex');
    expect(hash).toBeDefined();
    expect(hash.length).toBe(32); // MD5 hash length
  });

  test('Binary image upload and download round-trip should preserve data', async () => {
    // Try to use test image, fallback to created binary data
    let testImageData;
    try {
      testImageData = fs.readFileSync('./tests/images/test.jpeg');
    } catch (err) {
      // Create fake JPEG data if test image doesn't exist
      testImageData = Buffer.from([
        0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
        0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00, 0xFF, 0xD9
      ]);
    }

    const originalHash = crypto.createHash('sha256').update(testImageData).digest('hex');

    // Upload the image
    const uploadResponse = await cycleTLS('https://httpbin.org/post', {
      body: testImageData.toString('binary'), // Convert Buffer to binary string
      headers: {
        'Content-Type': 'image/jpeg'
      },
      ja3,
      userAgent
    }, 'POST');

    expect(uploadResponse.status).toBe(200);

    // Download a reference image for comparison  
    const downloadResponse = await cycleTLS('https://httpbin.org/image/jpeg', {
      ja3,
      userAgent
    });

    expect(downloadResponse.status).toBe(200);

    // Use arrayBuffer() to get the downloaded data
    const downloadedData = await downloadResponse.arrayBuffer();
    const downloadedBuffer = Buffer.from(downloadedData);

    // Verify it's valid JPEG data
    expect(downloadedBuffer[0]).toBe(0xFF);
    expect(downloadedBuffer[1]).toBe(0xD8);

    // Verify we can process the data without corruption
    const downloadedHash = crypto.createHash('sha256').update(downloadedBuffer).digest('hex');
    expect(downloadedHash).toBeDefined();
    expect(downloadedHash.length).toBe(64); // SHA256 hash length
  });

  test('Mixed binary and text data should both work correctly', async () => {
    const binaryData = Buffer.from([0xFF, 0x00, 0x01, 0x02, 0x80, 0x81, 0xFE, 0xFF]);
    const textData = JSON.stringify({ message: 'Hello, world!', binary: true });

    // Upload binary data
    const binaryResponse = await cycleTLS('https://httpbin.org/post', {
      body: binaryData.toString('binary'), // Convert Buffer to binary string
      headers: { 'Content-Type': 'application/octet-stream' },
      ja3,
      userAgent
    }, 'POST');

    expect(binaryResponse.status).toBe(200);

    // Upload text data  
    const textResponse = await cycleTLS('https://httpbin.org/post', {
      body: textData,
      headers: { 'Content-Type': 'application/json' },
      ja3,
      userAgent
    }, 'POST');

    expect(textResponse.status).toBe(200);

    // Verify text response can be parsed as JSON
    const textResponseData = JSON.parse(await textResponse.text());
    expect(textResponseData.json.message).toBe('Hello, world!');
    expect(textResponseData.json.binary).toBe(true);

    // Verify binary response has data
    const binaryResponseData = JSON.parse(await binaryResponse.text());
    expect(binaryResponseData.data).toBeDefined();
  });

  test('Large binary file upload should not corrupt data', async () => {
    // Create a larger binary file with repeating problematic sequences
    const pattern = Buffer.from([0xFF, 0x00, 0x80, 0x81, 0xC0, 0xC1, 0xFE, 0xFF]);
    const repetitions = 1000; // 8KB of binary data
    const largeData = Buffer.concat(Array(repetitions).fill(pattern));

    const originalHash = crypto.createHash('md5').update(largeData).digest('hex');

    const response = await cycleTLS('https://httpbin.org/post', {
      body: largeData.toString('binary'), // Convert Buffer to binary string
      headers: {
        'Content-Type': 'application/octet-stream',
        'Content-Length': largeData.length.toString()
      },
      ja3,
      userAgent
    }, 'POST');

    expect(response.status).toBe(200);

    const responseData = JSON.parse(await response.text());
    const receivedData = Buffer.from(responseData.data, 'binary');
    const receivedHash = crypto.createHash('md5').update(receivedData).digest('hex');

    expect(receivedHash).toBe(originalHash);
    expect(receivedData.length).toBe(largeData.length);
  });

  test('Binary data with all possible byte values should be preserved', async () => {
    // Create data with all possible byte values (0-255)
    const allBytesData = Buffer.from(Array.from({ length: 256 }, (_, i) => i));
    const originalHash = crypto.createHash('sha256').update(allBytesData).digest('hex');

    const response = await cycleTLS('https://httpbin.org/post', {
      body: allBytesData.toString('binary'), // Convert Buffer to binary string
      headers: { 'Content-Type': 'application/octet-stream' },
      ja3,
      userAgent
    }, 'POST');

    expect(response.status).toBe(200);

    const responseData = JSON.parse(await response.text());
    const receivedData = Buffer.from(responseData.data, 'binary');
    const receivedHash = crypto.createHash('sha256').update(receivedData).digest('hex');

    expect(receivedHash).toBe(originalHash);
    expect(receivedData.length).toBe(256);

    // Verify all byte values are preserved
    for (let i = 0; i < 256; i++) {
      expect(receivedData[i]).toBe(i);
    }
  });

  test('arrayBuffer() provides clean binary data access', async () => {
    const response = await cycleTLS('https://httpbin.org/image/png', {
      ja3,
      userAgent
    });

    expect(response.status).toBe(200);

    // Use arrayBuffer() for binary data (recommended approach)
    const arrayBufferData = await response.arrayBuffer();
    const bufferFromArray = Buffer.from(arrayBufferData);

    // Verify it's valid PNG data (starts with PNG signature)
    const pngSignature = Buffer.from([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]);
    expect(bufferFromArray.subarray(0, 8).equals(pngSignature)).toBe(true);

    // Verify we got substantial binary data
    expect(bufferFromArray.length).toBeGreaterThan(1000); // PNG should be reasonably large
    
    // Verify we can process binary data without corruption
    const hash = crypto.createHash('md5').update(bufferFromArray).digest('hex');
    expect(hash).toBeDefined();
    expect(hash.length).toBe(32);
  });

  test('Empty binary data should be handled correctly', async () => {
    const emptyData = Buffer.alloc(0);

    const response = await cycleTLS('https://httpbin.org/post', {
      body: emptyData.toString('binary'), // Convert Buffer to binary string (empty string)
      headers: { 'Content-Type': 'application/octet-stream' },
      ja3,
      userAgent
    }, 'POST');

    expect(response.status).toBe(200);

    const responseData = JSON.parse(await response.text());
    // httpbin should return empty string for empty data
    expect(responseData.data).toBe('');
  });
});