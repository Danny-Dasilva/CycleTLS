const initCycleTLS = require("../dist/index.js");

async function testJA4RRoundtrip(testName, inputJA4R, expectedExtensions = null) {
    console.log(`\n=== ${testName} ===`);
    console.log(`Input JA4R:  ${inputJA4R}`);
    
    const cycleTLS = await initCycleTLS({ port: 9118 });
    
    try {
        const response = await cycleTLS.get('https://tls.peet.ws/api/all', {
            ja4r: inputJA4R,
            disableGrease: false,
            userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36'
        });
        
        if (response.status !== 200) {
            console.log(await response.text())
            throw new Error(`HTTP ${response.status}: Request failed`);
        }
        
        const result = await response.json();
        
        if (!result.tls || !result.tls.ja4_r) {
            throw new Error(`Missing TLS data or JA4_r in response`);
        }
        
        const outputJA4R = result.tls.ja4_r;
        
        console.log(`Output JA4R: ${outputJA4R}`);
        
        // Parse input and output JA4R
        const inputParts = inputJA4R.split('_');
        const outputParts = outputJA4R.split('_');
        
        // Compare sections
        const inputHeader = inputParts[0];
        const outputHeader = outputParts[0];
        const inputExtensions = inputParts[2] ? inputParts[2].split(',') : [];
        const outputExtensions = outputParts[2] ? outputParts[2].split(',') : [];
        
        console.log(`Header:     ${inputHeader} → ${outputHeader} ${inputHeader === outputHeader ? '✅' : '❌'}`);
        console.log(`Ext Count:  ${inputExtensions.length} → ${outputExtensions.length} ${inputExtensions.length === outputExtensions.length ? '✅' : '❌'}`);
        
        if (expectedExtensions) {
            const missingExtensions = expectedExtensions.filter(ext => !outputExtensions.includes(ext));
            const extraExtensions = outputExtensions.filter(ext => !expectedExtensions.includes(ext) && !inputExtensions.includes(ext));
            
            if (missingExtensions.length > 0) {
                console.log(`Missing:    ${missingExtensions.join(', ')} ❌`);
            }
            if (extraExtensions.length > 0) {
                console.log(`Extra:      ${extraExtensions.join(', ')} ❌`);
            }
            if (missingExtensions.length === 0 && extraExtensions.length === 0) {
                console.log(`Extensions: All expected extensions present ✅`);
            }
        }
        
        // Check for specific extensions
        const hasNewALPS = outputExtensions.includes('44cd');
        const hasECH = outputExtensions.includes('fe0d');
        const hasOldALPS = outputExtensions.includes('4469');
        
        console.log(`New ALPS (44cd): ${hasNewALPS ? '✅' : '❌'}`);
        console.log(`ECH (fe0d):      ${hasECH ? '✅' : '❌'}`);
        console.log(`Old ALPS (4469): ${hasOldALPS ? (inputExtensions.includes('4469') ? '✅' : '⚠️ Unexpected') : '✅'}`);
        
        const isExactMatch = inputJA4R === outputJA4R;
        console.log(`Exact Match: ${isExactMatch ? '✅' : '❌'}`);
        
        return {
            success: isExactMatch,
            inputJA4R,
            outputJA4R,
            hasNewALPS,
            hasECH,
            hasOldALPS
        };
        
    } catch (error) {
        console.error(`Error: ${error.message} ❌`);
        return { success: false, error: error.message };
    } finally {
        await cycleTLS.exit();
    }
}

(async () => {
    console.log("🧪 JA4R Validation Test Suite");
    
    const tests = [
        {
            name: "Original Test Case (with h2)",
            ja4r: "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601",
            expectedExt: ['0000', '0005', '000a', '000b', '000d', '0012', '0017', '001b', '0023', '002b', '002d', '0033', '44cd', 'fe0d', 'ff01']
        },
        {
            name: "Modified Test Case (no h2)",  
            ja4r: "t13d1515_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601",
            expectedExt: ['0000', '0005', '000a', '000b', '000d', '0012', '0017', '001b', '0023', '002b', '002d', '0033', '44cd', 'fe0d', 'ff01']
        },
        {
            name: "Old ALPS Test",
            ja4r: "t13d1514_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,4469,ff01_0403,0804,0401,0503,0805,0501,0806,0601",
            expectedExt: ['0000', '0005', '000a', '000b', '000d', '0012', '0017', '001b', '0023', '002b', '002d', '0033', '4469', 'ff01']
        },
        {
            name: "Firefox JA4R Test",
            ja4r: "t13d1717h2_002f,0035,009c,009d,1301,1302,1303,c009,c00a,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,001c,0022,0023,002b,002d,0033,fe0d,ff01_0403,0503,0603,0804,0805,0806,0401,0501,0601,0203,0201",
            expectedExt: ['0000', '0005', '000a', '000b', '000d', '0012', '0017', '001b', '001c', '0022', '0023', '002b', '002d', '0033', 'fe0d', 'ff01']
        },
        {
            name: "TLS 1.2 JA4R Test", 
            ja4r: "t12d128h2_002f,0035,009c,009d,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0005,000a,000b,000d,0017,0023,ff01_0403,0804,0401,0503,0805,0501,0806,0601,0201",
            expectedExt: ['0005', '000a', '000b', '000d', '0017', '0023', 'ff01']
        }
    ];
    
    let passedTests = 0;
    
    for (const test of tests) {
        const result = await testJA4RRoundtrip(test.name, test.ja4r, test.expectedExt);
        if (result.success) {
            passedTests++;
        }
        
        // Wait between tests
        await new Promise(resolve => setTimeout(resolve, 1000));
    }
    
    console.log(`\n📊 Results: ${passedTests}/${tests.length} tests passed`);
    
    if (passedTests === tests.length) {
        console.log("🎉 All tests passed!");
    } else {
        console.log("❌ Some tests failed - JA4R mapping needs fixes");
    }
})();