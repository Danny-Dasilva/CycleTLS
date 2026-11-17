/**
 * Unit tests for CreditManager
 */

import { CreditManager } from '../src/credit-manager';

describe('CreditManager', () => {
  test('flush should not send credits when paused', () => {
    const sent: number[] = [];
    const cm = new CreditManager(512, (n) => sent.push(n));
    // Simulate receiving bytes (below threshold so onDataReceived won't auto-send)
    // We need to add bytes while not paused, then pause, then flush
    // addBytes doesn't exist - use onDataReceived but below threshold
    cm.onDataReceived(400);
    cm.pause();
    cm.flush();
    expect(sent).toHaveLength(0); // Nothing sent while paused
    cm.resume();
    cm.flush();
    expect(sent).toHaveLength(1); // Now sends
    expect(sent[0]).toBe(400);
  });

  test('flush sends credits when not paused', () => {
    const sent: number[] = [];
    const cm = new CreditManager(512, (n) => sent.push(n));
    cm.onDataReceived(300);
    cm.flush();
    expect(sent).toHaveLength(1);
    expect(sent[0]).toBe(300);
  });

  test('onDataReceived does not accumulate when paused', () => {
    const sent: number[] = [];
    const cm = new CreditManager(512, (n) => sent.push(n));
    cm.pause();
    cm.onDataReceived(600);
    expect(sent).toHaveLength(0); // Nothing sent while paused
  });

  test('resume sends accumulated credits above threshold', () => {
    const sent: number[] = [];
    const cm = new CreditManager(512, (n) => sent.push(n));
    cm.onDataReceived(400); // below threshold, accumulated
    cm.pause();
    // resume should check threshold
    cm.resume();
    // 400 < 512, so resume won't auto-send
    expect(sent).toHaveLength(0);
    // flush should send the 400
    cm.flush();
    expect(sent).toHaveLength(1);
    expect(sent[0]).toBe(400);
  });
});
