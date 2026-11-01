import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const load = async () => import('data:text/javascript;base64,' + Buffer.from(await readFile(new URL('../api/socket.js', import.meta.url))).toString('base64'));
const flush = () => new Promise(resolve => setImmediate(resolve));

async function harness(fetchImpl = async () => ({ status: 200, redirected: false, url: 'https://example.test/workbench' })) {
  const { createSocketClient } = await load();
  const sockets = [], states = [], messages = [], errors = [], timers = new Map();
  let nextTimer = 0;
  class FakeSocket {
    constructor(url) { this.url = url; this.readyState = 0; this.sent = []; sockets.push(this); }
    open() { this.readyState = 1; this.onopen?.(); }
    message(value) { this.onmessage?.({ data: value }); }
    send(value) { this.sent.push(JSON.parse(value)); }
    close() { this.readyState = 3; this.onclose?.(); }
  }
  const client = createSocketClient({
    url: 'wss://example.test/ws', WebSocketImpl: FakeSocket, fetchImpl,
    onState: value => states.push(value), onMessage: value => messages.push(value), onError: value => errors.push(value),
    setTimer: (fn, delay) => { const id = ++nextTimer; timers.set(id, { fn, delay }); return id; },
    clearTimer: id => timers.delete(id),
  });
  return { client, sockets, states, messages, errors, timers };
}

test('socket validates envelopes, uses WSS and never queues disconnected commands', async () => {
  const h = await harness();
  h.client.start();
  assert.equal(h.sockets[0].url, 'wss://example.test/ws');
  assert.equal(h.client.send('stop_task'), false);
  h.sockets[0].open();
  assert.equal(h.client.send('get_background_task_status'), true);
  assert.deepEqual(h.sockets[0].sent, [{ type: 'get_background_task_status', data: {} }]);
  for (const frame of ['bad JSON', 'null', '[]', '{"type":12}', '{"type":""}']) h.sockets[0].message(frame);
  assert.equal(h.errors.length, 5);
  h.sockets[0].message('{"type":"init_config","data":{"version":"test"}}');
  assert.deepEqual(h.messages, [{ type: 'init_config', data: { version: 'test' } }]);
  h.sockets[0].close();
  await flush();
  assert.equal(h.client.send('stop_task'), false);
  assert.equal([...h.timers.values()][0].delay, 3000);
  [...h.timers.values()][0].fn();
  h.sockets[1].open();
  assert.deepEqual(h.sockets[1].sent, []);
  h.client.close();
});

test('expired authentication stops reconnect attempts and probes same-origin entry', async () => {
  let probe;
  const h = await harness(async (url, options) => {
    probe = { url, options };
    return { status: 200, redirected: true, url: 'https://example.test/auth/login' };
  });
  h.client.start();
  h.sockets[0].close();
  await flush();
  assert.equal(h.states.at(-1), 'auth-required');
  assert.equal(h.timers.size, 0);
  assert.equal(probe.url, 'https://example.test/workbench');
  assert.equal(probe.options.credentials, 'same-origin');
  h.client.close();
});

test('network failures retry, duplicate start does not create a second live connection', async () => {
  const h = await harness(async () => { throw new Error('offline'); });
  h.client.start(); h.client.start();
  assert.equal(h.sockets.length, 1);
  h.sockets[0].close();
  await flush();
  assert.equal(h.states.at(-1), 'disconnected');
  assert.equal(h.timers.size, 1);
  h.client.close();
  assert.equal(h.timers.size, 0);
});

test('close invalidates a pending auth probe and stale socket messages', async () => {
  let resolve;
  const h = await harness(() => new Promise(done => { resolve = done; }));
  h.client.start();
  h.sockets[0].close();
  h.client.close();
  resolve({ status: 401 });
  await flush();
  assert.equal(h.timers.size, 0);
  assert.equal(h.states.at(-1), 'disconnected');
  h.sockets[0].message('{"type":"log","data":"stale"}');
  assert.equal(h.messages.length, 0);
});

test('send exceptions do not claim successful command delivery', async () => {
  const h = await harness(); h.client.start(); h.sockets[0].open();
  h.sockets[0].send = () => { throw new Error('write failed'); };
  assert.equal(h.client.send('stop_task'), false);
  assert.equal(h.errors.length, 1);
  h.client.close();
});

test('closed socket callbacks are ignored while the authentication probe is pending', async () => {
  let resolve;
  const h = await harness(() => new Promise(done => { resolve = done; }));
  h.client.start(); h.sockets[0].open(); h.sockets[0].close();
  h.sockets[0].message('{"type":"log","data":"late"}');
  h.sockets[0].open();
  assert.equal(h.messages.length, 0);
  assert.equal(h.states.at(-1), 'disconnected');
  resolve({ status: 200, redirected: false });
  await flush();
  assert.equal(h.timers.size, 1);
  h.client.close();
});

test('an unresponsive authentication probe times out and permits reconnect', async () => {
  const h = await harness(() => new Promise(() => {}));
  h.client.start(); h.sockets[0].close();
  const timeout = [...h.timers.values()].find(timer => timer.delay === 5000);
  assert(timeout, 'authentication probe needs a finite timeout');
  timeout.fn(); await flush();
  assert([...h.timers.values()].some(timer => timer.delay === 3000));
  h.client.close();
  assert.equal(h.timers.size, 0);
});
