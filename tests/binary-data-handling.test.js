const initCycleTLS = require('../dist/index.js');
const fs = require('fs');
const http = require('http');
const path = require('path');
const crypto = require('crypto');

jest.setTimeout(60000);

const ja3 = '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0';
const userAgent = 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0';

const jpegFixture = fs.readFileSync(path.join(__dirname, 'images', 'test.jpeg'));
const pngFixture = fs.readFileSync(path.join(__dirname, 'images', 'test.png'));

// These tests assert byte-exact behaviour, so they run against a local server
// that records the request body and echoes it back unchanged. httpbin.org is
// not usable for this: it re-encodes the body into a JSON `data` field (a
// UTF-8 string or a base64 data URL, depending on the bytes) and rate-limits
// CI runners.
describe('Binary Data Handling - Issue #297 Fix', () => {
  let cycleTLS;
  let server;
  let baseUrl;
  // Body of the most recent POST exactly as it arrived on the wire.
  let receivedBody;

  beforeAll((done) => {
    server = http.createServer((req, res) => {
      if (req.method === 'GET' && req.url === '/image/jpeg') {
        res.writeHead(200, { 'Content-Type': 'image/jpeg', 'Content-Length': jpegFixture.length });
        res.end(jpegFixture);
        return;
      }
      if (req.method === 'GET' && req.url === '/image/png') {
        res.writeHead(200, { 'Content-Type': 'image/png', 'Content-Length': pngFixture.length });
        res.end(pngFixture);
        return;
      }
      if (req.method === 'POST' && req.url === '/post') {
        const chunks = [];
        req.on('data', (chunk) => chunks.push(chunk));
        req.on('end', () => {
          receivedBody = Buffer.concat(chunks);
          res.writeHead(200, {
            'Content-Type': req.headers['content-type'] || 'application/octet-stream',
            'Content-Length': receivedBody.length,
          });
          res.end(receivedBody);
        });
        return;
      }
      res.writeHead(404);
      res.end();
    });
    server.listen(0, '127.0.0.1', () => {
      baseUrl = `http://127.0.0.1:${server.address().port}`;
      done();
    });
  });

  afterAll((done) => {
    server.close(done);
  });

  beforeEach(async () => {
    receivedBody = undefined;
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

    const response = await cycleTLS(`${baseUrl}/post`, {
      body: problematicData, // A Buffer body is sent byte-for-byte
      headers: {
        'Content-Type': 'application/octet-stream',
        'Content-Length': problematicData.length.toString()
      },
      ja3,
      userAgent
    }, 'POST');

    expect(response.status).toBe(200);

    // What the server received on the wire
    expect(receivedBody.equals(problematicData)).toBe(true);

    // What the server echoed back
    const echoed = Buffer.from(await response.arrayBuffer());
    const receivedHash = crypto.createHash('md5').update(echoed).digest('hex');

    expect(receivedHash).toBe(originalHash);
    expect(echoed.equals(problematicData)).toBe(true);
  });

  test('Binary download using arrayBuffer() should preserve data integrity', async () => {
    const response = await cycleTLS(`${baseUrl}/image/jpeg`, {
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

    // The download must match the served file byte-for-byte
    const buffer = Buffer.from(binaryData);
    const hash = crypto.createHash('md5').update(buffer).digest('hex');
    expect(hash).toBe(crypto.createHash('md5').update(jpegFixture).digest('hex'));
    expect(buffer.length).toBe(jpegFixture.length);
  });

  test('Binary image upload and download round-trip should preserve data', async () => {
    const testImageData = jpegFixture;
    const originalHash = crypto.createHash('sha256').update(testImageData).digest('hex');

    // Upload the image
    const uploadResponse = await cycleTLS(`${baseUrl}/post`, {
      body: testImageData,
      headers: {
        'Content-Type': 'image/jpeg'
      },
      ja3,
      userAgent
    }, 'POST');

    expect(uploadResponse.status).toBe(200);
    expect(crypto.createHash('sha256').update(receivedBody).digest('hex')).toBe(originalHash);

    // Download the same image
    const downloadResponse = await cycleTLS(`${baseUrl}/image/jpeg`, {
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

    // Upload and download must both match the original file
    const downloadedHash = crypto.createHash('sha256').update(downloadedBuffer).digest('hex');
    expect(downloadedHash).toBe(originalHash);
  });

  test('Mixed binary and text data should both work correctly', async () => {
    const binaryData = Buffer.from([0xFF, 0x00, 0x01, 0x02, 0x80, 0x81, 0xFE, 0xFF]);
    const textData = JSON.stringify({ message: 'Hello, world!', binary: true });

    // Upload binary data
    const binaryResponse = await cycleTLS(`${baseUrl}/post`, {
      body: binaryData,
      headers: { 'Content-Type': 'application/octet-stream' },
      ja3,
      userAgent
    }, 'POST');

    expect(binaryResponse.status).toBe(200);
    expect(receivedBody.equals(binaryData)).toBe(true);

    // Upload text data
    const textResponse = await cycleTLS(`${baseUrl}/post`, {
      body: textData,
      headers: { 'Content-Type': 'application/json' },
      ja3,
      userAgent
    }, 'POST');

    expect(textResponse.status).toBe(200);
    expect(receivedBody.toString('utf8')).toBe(textData);

    // Verify the echoed text can be parsed as JSON
    const textResponseData = JSON.parse(await textResponse.text());
    expect(textResponseData.message).toBe('Hello, world!');
    expect(textResponseData.binary).toBe(true);

    // Verify the echoed binary body is intact
    const echoedBinary = Buffer.from(await binaryResponse.arrayBuffer());
    expect(echoedBinary.equals(binaryData)).toBe(true);
  });

  test('Large binary file upload should not corrupt data', async () => {
    // Create a larger binary file with repeating problematic sequences
    const pattern = Buffer.from([0xFF, 0x00, 0x80, 0x81, 0xC0, 0xC1, 0xFE, 0xFF]);
    const repetitions = 1000; // 8KB of binary data
    const largeData = Buffer.concat(Array(repetitions).fill(pattern));

    const originalHash = crypto.createHash('md5').update(largeData).digest('hex');

    const response = await cycleTLS(`${baseUrl}/post`, {
      body: largeData,
      headers: {
        'Content-Type': 'application/octet-stream',
        'Content-Length': largeData.length.toString()
      },
      ja3,
      userAgent
    }, 'POST');

    expect(response.status).toBe(200);
    expect(receivedBody.equals(largeData)).toBe(true);

    const receivedData = Buffer.from(await response.arrayBuffer());
    const receivedHash = crypto.createHash('md5').update(receivedData).digest('hex');

    expect(receivedHash).toBe(originalHash);
    expect(receivedData.length).toBe(largeData.length);
  });

  test('Binary data with all possible byte values should be preserved', async () => {
    // Create data with all possible byte values (0-255)
    const allBytesData = Buffer.from(Array.from({ length: 256 }, (_, i) => i));
    const originalHash = crypto.createHash('sha256').update(allBytesData).digest('hex');

    const response = await cycleTLS(`${baseUrl}/post`, {
      body: allBytesData,
      headers: { 'Content-Type': 'application/octet-stream' },
      ja3,
      userAgent
    }, 'POST');

    expect(response.status).toBe(200);
    expect(receivedBody.equals(allBytesData)).toBe(true);

    const receivedData = Buffer.from(await response.arrayBuffer());
    const receivedHash = crypto.createHash('sha256').update(receivedData).digest('hex');

    expect(receivedHash).toBe(originalHash);
    expect(receivedData.length).toBe(256);

    // Verify all byte values are preserved
    for (let i = 0; i < 256; i++) {
      expect(receivedData[i]).toBe(i);
    }
  });

  test('String body is sent as UTF-8 text, not as raw bytes', async () => {
    // A string is text: each character above 0x7F becomes a multi-byte UTF-8
    // sequence on the wire. A latin1 "binary string" therefore does not carry
    // raw bytes; binary payloads must be passed as Buffer/Uint8Array.
    const bytes = Buffer.from([0x41, 0x80, 0xFF, 0x42]);
    const binaryString = bytes.toString('binary');

    const response = await cycleTLS(`${baseUrl}/post`, {
      body: binaryString,
      headers: { 'Content-Type': 'text/plain; charset=utf-8' },
      ja3,
      userAgent
    }, 'POST');

    expect(response.status).toBe(200);
    expect(receivedBody.equals(Buffer.from(binaryString, 'utf8'))).toBe(true);
    expect(receivedBody.toString('utf8')).toBe(binaryString);
  });

  test('arrayBuffer() provides clean binary data access', async () => {
    const response = await cycleTLS(`${baseUrl}/image/png`, {
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

    // The download must match the served file byte-for-byte
    const hash = crypto.createHash('md5').update(bufferFromArray).digest('hex');
    expect(hash).toBe(crypto.createHash('md5').update(pngFixture).digest('hex'));
  });

  test('Empty binary data should be handled correctly', async () => {
    const emptyData = Buffer.alloc(0);

    const response = await cycleTLS(`${baseUrl}/post`, {
      body: emptyData,
      headers: { 'Content-Type': 'application/octet-stream' },
      ja3,
      userAgent
    }, 'POST');

    expect(response.status).toBe(200);

    // The server receives an empty body and echoes an empty body
    expect(receivedBody.length).toBe(0);
    expect((await response.arrayBuffer()).byteLength).toBe(0);
  });
});
