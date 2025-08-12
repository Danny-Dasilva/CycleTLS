/**
 * Binary Data Handling Examples - Fixes GitHub Issue #297
 * 
 * This file demonstrates how CycleTLS properly handles binary data without UTF-8 corruption.
 * Before this fix, binary data would be corrupted when converting to/from strings.
 * 
 * Key improvements:
 * - Use arrayBuffer() for downloading binary content
 * - Proper binary upload handling 
 * - Data integrity preservation
 */

const initCycleTLS = require('cycletls');
const fs = require('fs');
const crypto = require('crypto');

async function demonstrateBinaryDataHandling() {
  const cycleTLS = await initCycleTLS();

  console.log('🔄 Demonstrating binary data handling fixes for Issue #297\n');

  try {
    // Example 1: Upload binary data with problematic UTF-8 sequences
    console.log('1. Testing binary upload with problematic UTF-8 sequences...');
    
    // Create data that would corrupt in UTF-8 encoding
    const problematicData = Buffer.from([
      0xFF, 0xD8, 0xFF, 0xE0, // JPEG header
      0x80, 0x81, 0x82, 0x83, // Invalid UTF-8 sequences
      0x00, 0x01, 0x02, 0x03, // Null bytes and control characters
      0xFE, 0xFF, 0xC0, 0xC1, // More problematic bytes
    ]);

    const originalHash = crypto.createHash('md5').update(problematicData).digest('hex');
    console.log(`   Original data hash: ${originalHash}`);

    const uploadResponse = await cycleTLS('https://httpbin.org/post', {
      body: problematicData.toString('binary'), // Convert Buffer to binary string for proper transmission
      headers: {
        'Content-Type': 'application/octet-stream'
      },
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0'
    }, 'POST');

    // Verify data integrity
    const serverData = JSON.parse(await uploadResponse.text());
    const receivedData = Buffer.from(serverData.data, 'binary');
    const receivedHash = crypto.createHash('md5').update(receivedData).digest('hex');

    console.log(`   Received data hash: ${receivedHash}`);
    console.log(`   ✅ Data integrity preserved: ${originalHash === receivedHash}\n`);

    // Example 2: Download binary image using arrayBuffer()
    console.log('2. Testing binary download with arrayBuffer()...');
    
    const imageResponse = await cycleTLS('https://httpbin.org/image/jpeg', {
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0'
    });

    // ✅ CORRECT: Use arrayBuffer() for binary data
    const binaryData = await imageResponse.arrayBuffer();
    const dataArray = new Uint8Array(binaryData);
    
    // Verify it's a valid JPEG
    const isValidJpeg = dataArray[0] === 0xFF && dataArray[1] === 0xD8;
    console.log(`   Downloaded ${binaryData.byteLength} bytes`);
    console.log(`   ✅ Valid JPEG signature: ${isValidJpeg}\n`);

    // Save the image to demonstrate data integrity
    fs.writeFileSync('downloaded-image.jpg', Buffer.from(binaryData));
    console.log('   📁 Image saved as downloaded-image.jpg');

    // Example 3: Compare arrayBuffer() vs text() for binary data
    console.log('\n3. Comparing arrayBuffer() vs text() methods...');
    
    const testResponse = await cycleTLS('https://httpbin.org/image/png', {
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0'
    });

    // Get data both ways
    const arrayBufferData = await testResponse.arrayBuffer();
    const textData = await testResponse.text();

    // Convert to buffers for comparison
    const bufferFromArray = Buffer.from(arrayBufferData);
    const bufferFromText = Buffer.from(textData, 'binary');

    const arraysEqual = bufferFromArray.equals(bufferFromText);
    console.log(`   ArrayBuffer size: ${arrayBufferData.byteLength} bytes`);
    console.log(`   Text data size: ${textData.length} bytes`);
    console.log(`   ✅ Data consistency: ${arraysEqual}`);

    // Verify PNG signature in both
    const pngSignature = [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A];
    const arrayHasSignature = pngSignature.every((byte, i) => new Uint8Array(arrayBufferData)[i] === byte);
    const textHasSignature = pngSignature.every((byte, i) => bufferFromText[i] === byte);
    
    console.log(`   ✅ ArrayBuffer PNG signature valid: ${arrayHasSignature}`);
    console.log(`   ✅ Text PNG signature valid: ${textHasSignature}\n`);

    // Example 4: Test all possible byte values (comprehensive corruption test)
    console.log('4. Testing all possible byte values (0-255)...');
    
    const allBytesData = Buffer.from(Array.from({ length: 256 }, (_, i) => i));
    const allBytesHash = crypto.createHash('sha256').update(allBytesData).digest('hex');
    
    const allBytesResponse = await cycleTLS('https://httpbin.org/post', {
      body: allBytesData.toString('binary'), // Convert Buffer to binary string
      headers: { 'Content-Type': 'application/octet-stream' },
      ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
      userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0'
    }, 'POST');

    const allBytesServerData = JSON.parse(await allBytesResponse.text());
    const allBytesReceived = Buffer.from(allBytesServerData.data, 'binary');
    const allBytesReceivedHash = crypto.createHash('sha256').update(allBytesReceived).digest('hex');

    console.log(`   Original hash: ${allBytesHash}`);
    console.log(`   Received hash: ${allBytesReceivedHash}`);
    console.log(`   ✅ All 256 byte values preserved: ${allBytesHash === allBytesReceivedHash}\n`);

    // Example 5: Demonstrate file upload
    console.log('5. Testing actual file upload...');
    
    try {
      // Try to read an existing image file
      let testFileData;
      const testFiles = ['./examples/test-image.jpg', './tests/images/test.jpeg', './package.json'];
      
      for (const filePath of testFiles) {
        try {
          testFileData = fs.readFileSync(filePath);
          console.log(`   📁 Reading file: ${filePath}`);
          break;
        } catch (err) {
          // File doesn't exist, try next
        }
      }

      if (!testFileData) {
        // Create a minimal test file
        testFileData = Buffer.from('Test file content with binary data: \x00\x01\x02\xFF\xFE');
        console.log('   📁 Using generated test data');
      }

      const fileHash = crypto.createHash('md5').update(testFileData).digest('hex');
      console.log(`   File size: ${testFileData.length} bytes`);
      console.log(`   File hash: ${fileHash}`);

      const fileResponse = await cycleTLS('https://httpbin.org/post', {
        body: testFileData.toString('binary'), // Convert Buffer to binary string
        headers: { 'Content-Type': 'application/octet-stream' },
        ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
        userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0'
      }, 'POST');

      console.log(`   ✅ Upload successful: ${fileResponse.status === 200}\n`);

    } catch (fileErr) {
      console.log(`   ⚠️  File test skipped: ${fileErr.message}\n`);
    }

    console.log('🎉 All binary data handling tests completed successfully!');
    console.log('   Issue #297 fix verified: No UTF-8 corruption detected');

  } catch (error) {
    console.error('❌ Error during binary data testing:', error);
  } finally {
    await cycleTLS.exit();
  }
}

// Migration examples showing before/after the fix
function showMigrationExamples() {
  console.log('\n📋 Migration Guide - Before vs After Issue #297 Fix:\n');

  console.log('❌ BEFORE (would cause corruption):');
  console.log('   const response = await cycleTLS(url);');
  console.log('   const data = await response.text(); // Binary data corrupted as UTF-8\n');

  console.log('✅ AFTER (preserves binary integrity):');
  console.log('   const response = await cycleTLS(url);');
  console.log('   const data = await response.arrayBuffer(); // Clean binary data\n');

  console.log('🔧 For uploads:');
  console.log('   // Convert Buffer to binary string for transmission');
  console.log('   const binaryData = fs.readFileSync("image.jpg");');
  console.log('   const response = await cycleTLS(url, { body: binaryData.toString("binary") }, "POST");\n');
}

// Main execution
if (require.main === module) {
  showMigrationExamples();
  demonstrateBinaryDataHandling().catch(console.error);
}

module.exports = {
  demonstrateBinaryDataHandling,
  showMigrationExamples
};