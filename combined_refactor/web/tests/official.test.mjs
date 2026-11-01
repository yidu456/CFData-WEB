import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
const load = async () => import('data:text/javascript;base64,' + Buffer.from(await readFile(new URL('../features/official/model.js', import.meta.url))).toString('base64'));
const base = { ipType: '4', port: '443', threads: '100', delay: '500', scanMode: 'tcping' };

test('scan request validates existing families and ports, keeps zero delay', async () => {
  const { buildScanRequest } = await load();
  for (const port of [443,2053,2083,2087,2096,8443,80,8080,8880,2052,2082,2086,2095]) {
    assert.deepEqual(buildScanRequest({ ...base, port: String(port) }), { ipType: 4, port, threads: 100, delay: 500, scanMode: 'tcping', autoSpeed: false });
  }
  assert.equal(buildScanRequest({ ...base, ipType: '6', delay: '0', scanMode: 'httping' }).delay, 0);
  for (const values of [{ipType:'5'}, {port:'abc'}, {port:'0'}, {port:'22'}, {threads:'1.5'}, {threads:''}, {threads:'10001'}, {delay:'-1'}, {delay:'60001'}, {scanMode:'unknown'}]) {
    assert.throws(() => buildScanRequest({ ...base, ...values }));
  }
});

test('rows preserve full data, deduplicate endpoints and convert nanos for latency sorting', async () => {
  const { createOfficialState, reduceOfficial, visibleScanRows } = await load();
  let state = createOfficialState();
  state = reduceOfficial(state, {type:'scan_result',data:{IP:'192.0.2.1',Port:443,DataCenter:'HKG',LatencyStr:'12.00ms',TCPDuration:12000000}});
  state = reduceOfficial(state, {type:'scan_result',data:{IP:'192.0.2.2',Port:443,TCPDuration:5000000}});
  state = reduceOfficial(state, {type:'scan_result',data:{IP:'192.0.2.1',Port:443,DataCenter:'HKG',TCPDuration:8000000}});
  assert.equal(state.rows.length,2); assert.equal(state.rows[0].latencyMS,8);
  assert.equal(visibleScanRows(state)[0].IP,'192.0.2.2');
  state = reduceOfficial(state,{type:'scan_result',data:{IP:'2001:db8::1',Port:443}});
  assert.equal(state.rows[2].latencyMS,null);
  state = reduceOfficial(state,{type:'scan_result',data:{IP:'',Port:443}});
  assert.equal(state.rows.length,3);
});

test('completed DC summaries sort by latency and selection must match current summary', async () => {
  const { createOfficialState, reduceOfficial } = await load();
  let state = reduceOfficial(createOfficialState(), {type:'scan_complete_wait_dc', data:[{DataCenter:'NRT',IPCount:2,MinLatency:20},{DataCenter:'HKG',IPCount:3,MinLatency:5}]});
  assert.equal(state.centers[0].DataCenter,'HKG');
  state = reduceOfficial(state,{type:'select_dc',data:'HKG'});
  assert.equal(state.selectedDC,'HKG');
  state = reduceOfficial(state,{type:'select_dc',data:'../secret'});
  assert.equal(state.selectedDC,'HKG');
  const previous=state;
  assert.deepEqual(reduceOfficial(state,{type:'task_stopped'}),previous);
  state = reduceOfficial(state,{type:'scan_requested'});
  assert.equal(state.centers.length,0); assert.equal(state.rows.length,0); assert.equal(state.selectedDC,'');
});

test('live display is bounded without discarding stored rows', async () => {
  const { createOfficialState, reduceOfficial, visibleScanRows } = await load();
  let state=createOfficialState();
  for(let i=0;i<105;i++)state=reduceOfficial(state,{type:'scan_result',data:{IP:`192.0.2.${i}`,Port:443,TCPDuration:i*1000000}});
  assert.equal(state.rows.length,105); assert.equal(visibleScanRows(state).length,100);
});

test('following a different background task does not mix scan rows or DC selection', async () => {
  const { createOfficialState, reduceOfficial } = await load();
  let state=reduceOfficial(createOfficialState(),{type:'scan_requested',data:{startedAt:'first'}});
  state=reduceOfficial(state,{type:'scan_result',data:{IP:'192.0.2.1',Port:443,TCPDuration:1000000}});
  state=reduceOfficial(state,{type:'scan_complete_wait_dc',data:[{DataCenter:'HKG',IPCount:1,MinLatency:1}]});
  state=reduceOfficial(state,{type:'select_dc',data:'HKG'});
  state=reduceOfficial(state,{type:'background_task_following',data:{mode:'official',startedAt:'second'}});
  assert.equal(state.rows.length,0);assert.equal(state.centers.length,0);assert.equal(state.selectedDC,'');
});

test('large IPv6 result sets retain endpoints without copying the full collection per event', async () => {
  const { createOfficialState, reduceOfficial, visibleScanRows } = await load();
  let state=createOfficialState();
  const rows=state.rows;
  for(let i=0;i<65536;i++)state=reduceOfficial(state,{type:'scan_result',data:{IP:`2001:db8::${i.toString(16)}`,Port:443,TCPDuration:i*1000000}});
  assert.equal(state.rows,rows);assert.equal(state.rows.length,65536);assert.equal(visibleScanRows(state).length,100);
});

test('detailed request uses selected valid DC and preserves zero delay', async () => {
  const { buildTestRequest }=await load();
  assert.deepEqual(buildTestRequest({...base,delay:'0'},'HKG',[{DataCenter:'HKG'}]),{dc:'HKG',port:443,delay:0,scanMode:'tcping'});
  assert.throws(()=>buildTestRequest(base,'NRT',[{DataCenter:'HKG'}]));
});

test('detailed rows normalize units, preserve partial/retest state and accept authoritative final rows', async () => {
  const { createOfficialState,reduceOfficial,visibleTestRows }=await load();
  let state=reduceOfficial(createOfficialState(),{type:'test_requested',data:{dc:'HKG'}});
  const row={IP:'192.0.2.1',Port:443,DataCenter:'HKG',Region:'亚洲',City:'香港',MinLatency:12000000,MaxLatency:20000000,AvgLatency:15000000,LossRate:.05,Speed:'失败'};
  state=reduceOfficial(state,{type:'test_result',data:row});
  assert.equal(state.testRows[0].minMS,12);assert.equal(state.testRows[0].avgMS,15);assert.equal(state.testRows[0].maxMS,20);assert.equal(state.testRows[0].lossPercent,5);assert.equal(state.testRows[0].Speed,'失败');
  state=reduceOfficial(state,{type:'task_stopped'});
  assert.equal(state.testingDC,'');assert.equal(state.testRows.length,1);
  state=reduceOfficial(state,{type:'test_requested',data:{dc:'HKG'}});
  assert.equal(state.testRows.length,0);
  state=reduceOfficial(state,{type:'test_result',data:row});
  state=reduceOfficial(state,{type:'test_complete',data:[{...row,AvgLatency:10000000}, {...row,IP:'2001:db8::1',MinLatency:null,LossRate:null}]});
  assert.equal(state.testRows.length,2);assert.equal(state.testingDC,'');assert.equal(state.testedDCs.has('HKG'),true);
  assert.equal(visibleTestRows(state)[0].avgMS,10);assert.equal(state.testRows[1].minMS,null);assert.equal(state.testRows[1].lossPercent,null);
});

test('reconnect retains displayed rows but invalidates foreground scan candidates', async () => {
  const {createOfficialState,reduceOfficial}=await load();
  let state=reduceOfficial(createOfficialState(),{type:'scan_requested'});
  assert.equal(state.candidatesReady,true);
  state=reduceOfficial(state,{type:'test_requested',data:{dc:'HKG'}});
  state=reduceOfficial(state,{type:'connection_lost'});
  assert.equal(state.candidatesReady,false);assert.equal(state.testingDC,'');
  state=reduceOfficial(state,{type:'background_task_found',data:{mode:'official',startedAt:'one'}});
  assert.equal(state.candidatesReady,false);
  state=reduceOfficial(state,{type:'background_task_following',data:{mode:'official',startedAt:'one'}});
  assert.equal(state.candidatesReady,true);
});

test('single/batch/continue speed updates keep endpoint result metadata and tested progress',async()=>{
 const {createOfficialState,reduceOfficial}=await load();let state=createOfficialState();
 state=reduceOfficial(state,{type:'test_result',data:{IP:'192.0.2.1',Port:443,AvgLatency:12000000,Speed:'1MB/s'}});
 state=reduceOfficial(state,{type:'speed_requested',data:{continued:true,ip:'192.0.2.1'}});
 assert.equal(state.testRows[0].Speed,'1MB/s');
 state=reduceOfficial(state,{type:'speed_test_result',data:{ip:'192.0.2.1',port:443,speed:'连接错误'}});
 assert.equal(state.testRows[0].Speed,'连接错误');assert.equal(state.testRows[0].avgMS,12);
 state=reduceOfficial(state,{type:'official_speed_complete',data:{}});assert.equal(state.continueUnlocked,true);
 state=reduceOfficial(state,{type:'speed_requested',data:{continued:false}});assert.equal(state.testRows[0].Speed,'');
});

test('replayed completed batch enables continuing untested rows',async()=>{
 const {createOfficialState,reduceOfficial}=await load();let state=createOfficialState();
 state=reduceOfficial(state,{type:'test_complete',data:[{IP:'192.0.2.1',Port:443,Speed:'1MB/s'},{IP:'192.0.2.2',Port:443,Speed:''}]});
 assert.equal(state.continueUnlocked,true);
});
test('automatic pipeline marks its actual detailed-test data center without frontend stage commands',async()=>{
 const {createOfficialState,reduceOfficial}=await load();let state=reduceOfficial(createOfficialState(),{type:'scan_requested',data:{automatic:true}});
 state=reduceOfficial(state,{type:'test_result',data:{IP:'192.0.2.1',Port:443,DataCenter:'HKG',AvgLatency:12000000}});
 assert.equal(state.testingDC,'HKG');assert.equal(state.selectedDC,'HKG');
 state=reduceOfficial(state,{type:'test_complete',data:[{IP:'192.0.2.1',Port:443,DataCenter:'HKG',AvgLatency:12000000}]});
 assert.equal(state.testedDCs.has('HKG'),true);assert.equal(state.testingDC,'');
});
