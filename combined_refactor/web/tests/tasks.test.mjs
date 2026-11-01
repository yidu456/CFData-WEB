import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
const load = async () => import('data:text/javascript;base64,' + Buffer.from(await readFile(new URL('../features/tasks/state.js', import.meta.url))).toString('base64'));

test('progress clamps the meter, validates counts and separates official/NSB phases', async () => {
  const { createTaskState, reduceTask } = await load();
  let state = reduceTask(createTaskState(), { type: 'scan_progress', data: { current: 12, total: 10 } });
  assert.equal(state.running, true); assert.equal(state.percent, 100); assert.equal(state.phase, '扫描中');
  state = reduceTask(state, { type: 'test_progress', data: { current: -5, total: 'NaN' } });
  assert.equal(state.current, 0); assert.equal(state.total, 0); assert.equal(state.percent, 0);
  state = reduceTask(state, { type: 'nsb_progress', data: { phase: 'reallink', current: 2, total: 9, text: '链路复验中' } });
  assert.equal(state.mode, 'nsb'); assert.equal(state.phase, '链路复验中');
  state = reduceTask(state, { type: 'official_speed_progress', data: { qualified: 3, limit: 5 } });
  assert.equal(state.phase, '测速中'); assert.equal(state.percent, 60);
});

test('specific completion outcomes survive generic completion and stop acknowledgement', async () => {
  const { createTaskState, reduceTask } = await load();
  for (const outcome of ['partial', 'failed']) {
    let state = reduceTask(createTaskState(), { type: 'nsb_csv_complete', data: { status: outcome, message: outcome, rows: [] } });
    state = reduceTask(state, { type: 'task_complete', data: null });
    state = reduceTask(state, { type: 'task_stopped', data: null });
    assert.equal(state.outcome, outcome); assert.equal(state.message, outcome);
  }
  let state = reduceTask(createTaskState(), { type: 'official_speed_complete', data: { rateLimited: true, qualified: 1, limit: 5 } });
  state = reduceTask(state, { type: 'task_complete', data: null });
  assert.equal(state.outcome, 'rate-limited'); assert.equal(state.running, false);
});

test('new tasks clear old outcomes while same-background generic snapshots do not', async () => {
  const { createTaskState, reduceTask } = await load();
  const snapshot = { label: '非标任务', mode: 'nsb', phase: '测速中', running: true, startedAt: 'first', current: 2, total: 10 };
  let state = reduceTask(createTaskState(), { type: 'background_task_following', data: snapshot });
  assert.equal(state.following, true); assert.equal(state.background, true);
  state = reduceTask(state, { type: 'nsb_csv_complete', data: { status: 'failed', message: '无合格结果', rows: [] } });
  state = reduceTask(state, { type: 'background_task_update', data: { ...snapshot, running: false, phase: '完成' } });
  assert.equal(state.outcome, 'failed'); assert.equal(state.message, '无合格结果');
  state = reduceTask(state, { type: 'background_task_found', data: { ...snapshot, startedAt: 'second' } });
  assert.equal(state.outcome, ''); assert.equal(state.following, false); assert.equal(state.running, true);
});

test('stopping survives running snapshots and missing only clears background ownership', async () => {
  const { createTaskState, reduceTask } = await load();
  let state = reduceTask(createTaskState(), { type: 'background_task_found', data: { running: true, phase: '扫描中', startedAt: 'one' } });
  state = reduceTask(state, { type: 'request_stop', data: {} });
  state = reduceTask(state, { type: 'background_task_update', data: { running: true, phase: '扫描中', startedAt: 'one' } });
  assert.equal(state.stopping, true); assert.equal(state.phase, '正在停止');
  state = reduceTask(state, { type: 'task_stopped', data: null });
  state = reduceTask(state, { type: 'task_complete', data: null });
  assert.equal(state.outcome, 'stopped');
  state = reduceTask(state, { type: 'background_task_missing', data: null });
  assert.equal(state.background, false); assert.equal(state.outcome, 'stopped');
});

test('initial configuration is restricted to display fields, unknown events keep state', async () => {
  const { createTaskState, reduceTask } = await load();
  const initial = createTaskState();
  assert.equal(reduceTask(initial, { type: 'github_upload_success', data: { token: 'secret' } }), initial);
  const state = reduceTask(initial, { type: 'init_config', data: { version: 'test', speedTestDefault: 'auto', proxyWarning: true, token: 'secret' } });
  assert.equal(state.initialized, true); assert.equal(state.version, 'test'); assert.equal(state.proxyWarning, true);
  assert.equal(JSON.stringify(state).includes('secret'), false);
});

test('background summary and live row events do not double count the same result', async () => {
  const { createTaskState, reduceTask } = await load();
  let state = reduceTask(createTaskState(), { type: 'background_task_following', data: { running: true, startedAt: 'one', resultCount: 5, testCount: 4, speedCount: 2, scanSuccess: 5 } });
  for (const type of ['scan_result', 'test_result', 'speed_test_result', 'nsb_scan_result']) state = reduceTask(state, { type, data: {} });
  assert.equal(state.resultCount, 5); assert.equal(state.testCount, 4); assert.equal(state.speedCount, 2);
});

test('task errors stay failed after generic completion and missing task does not keep stopping UI', async () => {
  const { createTaskState, reduceTask } = await load();
  let state = reduceTask(createTaskState(), { type: 'scan_progress', data: { current: 0, total: 10 } });
  state = reduceTask(state, { type: 'task_error', data: '未发现有效 IP' });
  state = reduceTask(state, { type: 'task_complete' });
  assert.equal(state.outcome, 'failed'); assert.equal(state.message, '未发现有效 IP');
  state = reduceTask(createTaskState(), { type: 'background_task_found', data: { running: true, startedAt: 'one' } });
  state = reduceTask(state, { type: 'request_stop' });
  state = reduceTask(state, { type: 'background_task_missing' });
  assert.equal(state.running, false); assert.equal(state.stopping, false);
  assert.notEqual(state.phase, '正在停止');
});

test('display text strips credentials and signed query parameters from source URLs', async () => {
  const { safeTaskText, createTaskState, reduceTask } = await load();
  const raw = '下载失败 https://user:password@example.test/input.csv?token=secret#key';
  assert.equal(safeTaskText(raw), '下载失败 https://example.test/input.csv');
  const state = reduceTask(createTaskState(), { type: 'background_task_found', data: { running: true, message: raw, params: { sourceURL: 'https://example.test/input.csv?token=secret' } } });
  assert.equal(JSON.stringify(state).includes('secret'), false);
  assert.equal(JSON.stringify(state).includes('password'), false);
});

test('local start request owns the task before the first progress or error frame', async () => {
  const { createTaskState, reduceTask } = await load();
  let state = reduceTask(createTaskState(), {type:'task_error',data:'previous failure'});
  state = reduceTask(state, {type:'request_start',data:{label:'官方优选扫描',mode:'official',startedAt:'new-local-task'}});
  assert.equal(state.running,true); assert.equal(state.background,false); assert.equal(state.outcome,'');
  assert.equal(state.startedAt,'new-local-task'); assert.equal(state.label,'官方优选扫描');
  state = reduceTask(state,{type:'task_error',data:'地址库下载失败'});
  assert.equal(state.outcome,'failed'); assert.equal(state.running,false);
});

test('stopped background replay keeps terminal label while updating available result count', async () => {
  const {createTaskState,reduceTask}=await load();
  let state=reduceTask(createTaskState(),{type:'background_task_following',data:{running:false,phase:'已停止',mode:'official',startedAt:'stopped'}});
  state=reduceTask(state,{type:'scan_complete_wait_dc',data:[{DataCenter:'HKG'}]});
  state=reduceTask(state,{type:'test_complete',data:[{IP:'192.0.2.1'}]});
  assert.equal(state.outcome,'stopped');assert.equal(state.phase,'已停止');assert.equal(state.testCount,1);
});
