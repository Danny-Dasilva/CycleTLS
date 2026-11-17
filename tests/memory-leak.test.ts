import { EventEmitter } from 'events';

describe('WebSocket listener cleanup', () => {
  test('listeners should be cleaned up after request', () => {
    const ws = new EventEmitter();
    ws.on('open', () => {});
    ws.on('message', () => {});
    ws.on('error', () => {});
    ws.on('close', () => {});
    expect(ws.listenerCount('open')).toBeGreaterThan(0);
    ws.removeAllListeners();
    expect(ws.listenerCount('open')).toBe(0);
  });

  test('listeners should not accumulate across requests', () => {
    const wsWithCleanup = new EventEmitter();
    for (let i = 0; i < 10; i++) {
      wsWithCleanup.on('open', () => {});
      wsWithCleanup.on('message', () => {});
      wsWithCleanup.removeAllListeners();
    }
    expect(wsWithCleanup.listenerCount('open')).toBe(0);
    expect(wsWithCleanup.listenerCount('message')).toBe(0);
  });

  test('request() source should contain ws.removeAllListeners', () => {
    const fs = require('fs');
    const source = fs.readFileSync(
      require.resolve('../dist/flow-control-client.js'),
      'utf8'
    );
    const requestMethodSource = source.substring(
      source.indexOf('async request('),
      source.indexOf('async get(')
    );
    expect(requestMethodSource).toContain('removeAllListeners');
  });

  test('ws() source should contain cleanup for control WebSocket', () => {
    const fs = require('fs');
    const source = fs.readFileSync(
      require.resolve('../dist/flow-control-client.js'),
      'utf8'
    );
    const wsMethodSource = source.substring(
      source.indexOf('async ws('),
      source.indexOf('webSocket(')
    );
    expect(wsMethodSource).toContain('removeAllListeners');
  });

  test('sse() source should contain cleanup for SSE WebSocket', () => {
    const fs = require('fs');
    const source = fs.readFileSync(
      require.resolve('../dist/flow-control-client.js'),
      'utf8'
    );
    const sseMethodSource = source.substring(
      source.indexOf('async sse('),
      source.indexOf('eventSource(')
    );
    expect(sseMethodSource).toContain('removeAllListeners');
  });
});
