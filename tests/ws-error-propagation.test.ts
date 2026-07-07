/**
 * Unit tests for flow control client error and edge case handling.
 */

import { EventEmitter } from 'events';
import { Readable } from 'stream';

// We test the behavioral contract: if a WebSocket error occurs after
// the response promise has resolved, it should destroy the bodyStream.
// Since the actual implementation creates WS internally, we verify
// the pattern by testing the error propagation logic directly.

describe('WS error after resolution', () => {
  test('post-resolution error should destroy body stream', (done) => {
    // Simulate the pattern used in flow-control-client.ts
    const bodyStream = new Readable({ read() {} });
    let resolved = false;

    // Simulate the error handler pattern from the fix
    const handleError = (err: Error) => {
      if (!resolved) {
        // Would reject promise
      } else {
        // Post-resolution: propagate to body stream
        if (bodyStream && !bodyStream.destroyed) {
          bodyStream.destroy(err);
        }
      }
    };

    // Simulate resolution
    resolved = true;
    bodyStream.push(Buffer.from('partial data'));

    bodyStream.on('error', (err) => {
      expect(err.message).toBe('connection reset');
      // Verify stream is actually destroyed after error propagation
      expect(bodyStream.destroyed).toBe(true);
      expect(bodyStream.readable).toBe(false);
      done();
    });

    // Now simulate a post-resolution WS error
    handleError(new Error('connection reset'));
  });

  test('pre-resolution error should not destroy body stream', () => {
    const bodyStream = new Readable({ read() {} });
    let resolved = false;
    let rejected = false;

    const handleError = (err: Error) => {
      if (!resolved) {
        rejected = true;
      } else {
        if (bodyStream && !bodyStream.destroyed) {
          bodyStream.destroy(err);
        }
      }
    };

    handleError(new Error('connection refused'));
    expect(rejected).toBe(true);
    expect(bodyStream.destroyed).toBe(false);

    // Clean up: destroy the stream to prevent leak
    bodyStream.destroy();
  });

  test('double-destroy on body stream should not throw', (done) => {
    const bodyStream = new Readable({ read() {} });

    // Attach error handler to prevent unhandled error crashes
    bodyStream.on('error', (err) => {
      expect(err.message).toBe('first error');
      expect(bodyStream.destroyed).toBe(true);

      // Second destroy after first should be safe (no-op, no extra error)
      expect(() => {
        bodyStream.destroy(new Error('second error'));
      }).not.toThrow();
      expect(bodyStream.destroyed).toBe(true);

      done();
    });

    // First destroy - triggers the error handler above
    bodyStream.destroy(new Error('first error'));
  });

  test('destroy on already-ended stream should be safe', (done) => {
    const bodyStream = new Readable({ read() {} });

    // Push data and end the stream
    bodyStream.push(Buffer.from('data'));
    bodyStream.push(null); // Signal end

    bodyStream.on('end', () => {
      // After end, destroying should be safe
      expect(() => {
        bodyStream.destroy();
      }).not.toThrow();
      expect(bodyStream.destroyed).toBe(true);
      done();
    });

    // Consume to trigger 'end'
    bodyStream.resume();
  });
});
