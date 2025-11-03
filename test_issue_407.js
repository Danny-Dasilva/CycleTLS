// Test script for Issue #407: Connection Reuse with Concurrent Requests
// This reproduces the exact scenario from the issue report

import initCycleTLS from './dist/index.js';
import chalk from 'chalk';

// ========== Configuration ==========
const ENABLE_CONNECTION_REUSE = true; // Toggle connection reuse for testing
const IS_CONCURRENT = true; // true: Concurrent mode | false: Sequential mode
const DELAY_BETWEEN_REQUESTS_MS = 100; // Delay between requests in milliseconds
const TEST_URL = 'https://www.google.com';
const NUM_INSTANCES = 5; // Number of CycleTLS instances (different ports)
const NUM_REQUESTS_PER_INSTANCE = 2; // Number of requests each instance will make
// ===================================

// Helper function: Delay
const delay = (ms) => new Promise(resolve => setTimeout(resolve, ms));

// Logging function with timestamp (millisecond precision)
function log(message, color = 'white') {
    const now = new Date();
    const timestamp = `${now.toLocaleDateString('en-US')} ${now.toLocaleTimeString('en-US', { hour12: false })}.${String(now.getMilliseconds()).padStart(3, '0')}`;
    const coloredMessage = color !== 'white' ? chalk[color](message) : message;
    console.log(`${chalk.dim(`[${timestamp}]`)} ${coloredMessage}`);
}

// Display configuration status
log('========================================', 'yellow');
log('  CycleTLS Connection Reuse Test', 'yellow');
log('  Issue #407 Reproduction', 'yellow');
log('========================================', 'yellow');
log(`Connection Reuse: ${ENABLE_CONNECTION_REUSE ? chalk.green('ENABLED ✓') : chalk.red('DISABLED ✗')}`, 'white');
log(`Execution Mode: ${IS_CONCURRENT ? chalk.cyan('Concurrent') : chalk.magenta('Sequential')}`, 'white');
log(`Delay Between Requests: ${DELAY_BETWEEN_REQUESTS_MS}ms`, 'white');
log(`Test URL: ${TEST_URL}`, 'white');
log(`Instances: ${NUM_INSTANCES}`, 'white');
log(`Requests per Instance: ${NUM_REQUESTS_PER_INSTANCE}`, 'white');
log('========================================\n', 'yellow');

// Array to hold all CycleTLS instances
const instances = [];

// Track errors and panics
let hadPanic = false;
let hadError = false;
const errors = [];

try {
    // Initialize instances outside the loop
    log('Initializing CycleTLS instances...', 'cyan');
    for (let i = 0; i < NUM_INSTANCES; i++) {
        const port = 9119 + i; // Ports: 9119, 9120, ..., up to NUM_INSTANCES
        try {
            const cycleTLS = await initCycleTLS({ port: port, timeout: 30000 });
            instances.push(cycleTLS);
            log(`  ✓ Instance ${i + 1} initialized on port ${port}`, 'gray');
        } catch (err) {
            log(`  ✗ Failed to initialize instance ${i + 1} on port ${port}: ${err.message}`, 'red');
            hadError = true;
            errors.push({ type: 'initialization', instance: i + 1, error: err.message });
        }
    }
    log(`✅ ${instances.length}/${NUM_INSTANCES} instances initialized\n`, 'green');

    if (instances.length === 0) {
        throw new Error('No instances could be initialized');
    }

    // Function to perform a single request
    async function performRequest(instanceIndex, requestIndex, cycleTLS) {
        const instancePort = 9119 + instanceIndex;
        log(`Instance ${instanceIndex + 1} (Port ${instancePort}) - Request ${requestIndex + 1}:`, 'cyan');

        // Start time
        const startTime = performance.now();

        try {
            // Send request
            const response = await cycleTLS(TEST_URL, {
                ja3: '771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0',
                userAgent: 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0',
                disableRedirect: true,
                enableConnectionReuse: ENABLE_CONNECTION_REUSE,
                http2Fingerprint: '1:65536;4:131072;5:16384|12517377|3:0:0:201,5:0:0:101,7:0:0:1,9:0:7:1,11:0:3:1,13:0:0:241|m,p,a,s',
                ja4h: 'ge11_73a4f1e_8b3fce7',
            }, 'get');

            // End time
            const endTime = performance.now();
            const executionTime = endTime - startTime;

            // Check for panic in response
            if (response.body && typeof response.body === 'string' && response.body.includes('panic:')) {
                hadPanic = true;
                const statusColor = 'red';
                log(`  ⚠️  Instance ${instanceIndex + 1}, Request ${requestIndex + 1}: ${executionTime.toFixed(2)} ms - PANIC DETECTED!`, statusColor);
                errors.push({
                    type: 'panic',
                    instance: instanceIndex + 1,
                    request: requestIndex + 1,
                    error: response.body.substring(0, 200)
                });
            } else {
                // Display execution time and status with instance and request info
                const statusColor = response.status === 200 ? 'green' : 'yellow';
                log(`  ↳ Instance ${instanceIndex + 1}, Request ${requestIndex + 1}: ${executionTime.toFixed(2)} ms, Status: ${response.status}`, statusColor);
            }

            // Return execution time with instance and request info
            return { instanceIndex, requestIndex, executionTime, status: response.status };
        } catch (err) {
            const endTime = performance.now();
            const executionTime = endTime - startTime;

            hadError = true;
            log(`  ✗ Instance ${instanceIndex + 1}, Request ${requestIndex + 1}: FAILED after ${executionTime.toFixed(2)} ms`, 'red');
            log(`    Error: ${err.message}`, 'red');
            errors.push({
                type: 'request_error',
                instance: instanceIndex + 1,
                request: requestIndex + 1,
                error: err.message
            });

            return { instanceIndex, requestIndex, executionTime, error: err.message };
        }
    }

    // Run requests based on mode
    const results = [];
    if (IS_CONCURRENT) {
        // Concurrent execution for all instances and their requests (default)
        const promises = [];
        for (let i = 0; i < instances.length; i++) {
            for (let j = 0; j < NUM_REQUESTS_PER_INSTANCE; j++) {
                // Add a small delay between each request to avoid overwhelming the server
                promises.push(
                    (async () => {
                        if (DELAY_BETWEEN_REQUESTS_MS > 0) {
                            await delay(DELAY_BETWEEN_REQUESTS_MS * (i * NUM_REQUESTS_PER_INSTANCE + j));
                        }
                        return await performRequest(i, j, instances[i]);
                    })()
                );
            }
        }
        const allResults = await Promise.all(promises);
        results.push(...allResults);
    } else {
        // Fully sequential execution: one instance at a time, one request at a time
        for (let i = 0; i < instances.length; i++) {
            for (let j = 0; j < NUM_REQUESTS_PER_INSTANCE; j++) {
                const result = await performRequest(i, j, instances[i]);
                results.push(result);

                // Add delay between sequential requests if configured
                if (DELAY_BETWEEN_REQUESTS_MS > 0 && (i < instances.length - 1 || j < NUM_REQUESTS_PER_INSTANCE - 1)) {
                    await delay(DELAY_BETWEEN_REQUESTS_MS);
                }
            }
        }
    }

    // Group results by instance
    const instanceStats = {};
    for (const result of results) {
        if (result.error) continue; // Skip failed requests

        const instanceKey = result.instanceIndex;
        if (!instanceStats[instanceKey]) {
            instanceStats[instanceKey] = [];
        }
        instanceStats[instanceKey].push(result.executionTime);
    }

    // Display statistics per instance
    log('\n=== Statistics Per Instance ===', 'yellow');
    for (let i = 0; i < instances.length; i++) {
        const times = instanceStats[i] || [];
        if (times.length === 0) {
            log(`\nInstance ${i + 1} (Port ${9119 + i}): No successful requests`, 'red');
            continue;
        }

        const instancePort = 9119 + i;
        const totalTime = times.reduce((sum, time) => sum + time, 0);
        const avgTime = totalTime / times.length;
        const minTime = Math.min(...times);
        const maxTime = Math.max(...times);

        log(`\nInstance ${i + 1} (Port ${instancePort}):`, 'cyan');
        log(`  Requests: ${times.length}`, 'white');
        log(`  Average: ${avgTime.toFixed(2)} ms`, 'blue');
        log(`  Minimum: ${minTime.toFixed(2)} ms`, 'green');
        log(`  Maximum: ${maxTime.toFixed(2)} ms`, 'magenta');
    }

    // Overall statistics
    const allTimes = results.filter(r => !r.error).map(r => r.executionTime);
    if (allTimes.length > 0) {
        const overallTotal = allTimes.reduce((sum, time) => sum + time, 0);
        const overallAvg = overallTotal / allTimes.length;
        const overallMin = Math.min(...allTimes);
        const overallMax = Math.max(...allTimes);

        log('\n=== Overall Statistics ===', 'yellow');
        log(`Total Requests: ${allTimes.length}`, 'white');
        log(`Average: ${overallAvg.toFixed(2)} ms`, 'blue');
        log(`Minimum: ${overallMin.toFixed(2)} ms`, 'green');
        log(`Maximum: ${overallMax.toFixed(2)} ms`, 'magenta');
    }

    // Error summary
    if (errors.length > 0) {
        log('\n=== Error Summary ===', 'red');
        log(`Total Errors: ${errors.length}`, 'red');

        const errorsByType = {};
        for (const err of errors) {
            errorsByType[err.type] = (errorsByType[err.type] || 0) + 1;
        }

        for (const [type, count] of Object.entries(errorsByType)) {
            log(`  ${type}: ${count}`, 'red');
        }

        log('\nFirst 3 errors:', 'red');
        for (const err of errors.slice(0, 3)) {
            log(`  [${err.type}] Instance ${err.instance}, Request ${err.request || 'N/A'}`, 'red');
            log(`    ${err.error.substring(0, 100)}`, 'gray');
        }
    }

} catch (err) {
    log('\n=== FATAL ERROR ===', 'red');
    log(err.message, 'red');
    log(err.stack, 'gray');
    hadError = true;
} finally {
    // Cleanup: Exit all CycleTLS instances
    log('\nCleaning up CycleTLS instances...', 'gray');
    for (const cycleTLS of instances) {
        try {
            await cycleTLS.exit();
        } catch (err) {
            log(`  Warning: Error closing instance: ${err.message}`, 'yellow');
        }
    }
    log('✅ All instances closed successfully', 'green');

    // Final verdict
    log('\n========================================', 'yellow');
    if (hadPanic) {
        log('❌ TEST FAILED: Panics detected!', 'red');
        log('Issue #407 is NOT fixed', 'red');
        process.exit(1);
    } else if (hadError) {
        log('⚠️  TEST COMPLETED WITH ERRORS', 'yellow');
        log('Some requests failed but no panics occurred', 'yellow');
        process.exit(1);
    } else {
        log('✅ TEST PASSED: No panics or errors!', 'green');
        log('Issue #407 appears to be fixed', 'green');
        process.exit(0);
    }
}
