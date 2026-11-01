import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
const load=async()=>import('data:text/javascript;base64,'+Buffer.from(await readFile(new URL('../features/measurement/model.js',import.meta.url))).toString('base64'));
const row={IP:'192.0.2.1',Port:443,DataCenter:'HKG',MinLatency:12000000,MaxLatency:20000000,AvgLatency:15000000,LossRate:.05,Speed:'',minMS:12};
test('measurement config validates URL/numbers and zero disables batch',async()=>{
 const {buildSpeedConfig}=await load();
 assert.deepEqual(buildSpeedConfig({port:'443',url:'auto',speedMin:'0.1',speedLimit:'5'}),{port:443,url:'auto',speedMin:.1,speedLimit:5});
 assert.equal(buildSpeedConfig({port:'443',url:'example.test/file',speedMin:'.1',speedLimit:'0'}).speedLimit,0);
 for(const url of ['ftp://example.test/a','javascript:alert(1)','https://','https://user:pass@example.test'])assert.throws(()=>buildSpeedConfig({port:443,url,speedMin:.1,speedLimit:5}));
 assert.throws(()=>buildSpeedConfig({port:443,url:'auto',speedMin:0,speedLimit:5}));
});
test('speed states preserve failure text and exact threshold equality',async()=>{
 const {speedStatus}=await load();
 assert.equal(speedStatus('',.1),'untested');assert.equal(speedStatus('未测速',.1),'untested');
 assert.equal(speedStatus('0.10MB/s',.1),'qualified');assert.equal(speedStatus('0.09MB/s',.1),'unqualified');
 assert.equal(speedStatus('连接错误',.1),'failed');assert.equal(speedStatus('0MB/s',.1),'failed');
});
test('speed requests retain all raw row fields, not visible/display conversions',async()=>{
 const {buildSpeedRequest,buildBatchRequest}=await load();const config={port:443,url:'auto',speedMin:.1,speedLimit:5};
 assert.deepEqual(buildSpeedRequest(config,[row],'192.0.2.1'),{ip:'192.0.2.1',port:443,resultPort:443,url:'auto'});
 assert.throws(()=>buildSpeedRequest(config,[row],'192.0.2.9'));
 const rows=Array.from({length:105},(_,i)=>({...row,IP:`192.0.2.${i}`}));
 const request=buildBatchRequest(config,rows,true);assert.equal(request.results.length,105);assert.equal(request.skipTested,true);
 assert.equal(request.results[0].MinLatency,12000000);assert.equal(request.results[0].LossRate,.05);assert.equal(request.results[0].minMS,undefined);
 assert.throws(()=>buildBatchRequest({...config,speedLimit:0},rows,false));
});
test('row identity remains distinct from the selected measurement port',async()=>{
 const {buildSpeedRequest}=await load();
 assert.deepEqual(buildSpeedRequest({port:8443,url:'auto'},[row],'192.0.2.1'),{ip:'192.0.2.1',port:8443,resultPort:443,url:'auto'});
});
test('automatic request is server-owned and zero speed limit keeps detailed stage',async()=>{
 const {buildAutoScanRequest}=await load();const scan={ipType:4,port:443,delay:0,threads:100,scanMode:'tcping',autoSpeed:false};
 const data=buildAutoScanRequest(scan,{port:443,url:'auto',speedMin:.1,speedLimit:0},' hkg ');
 assert.equal(data.serverAutoFlow,true);assert.equal(data.autoSpeed,true);assert.equal(data.officialTargetDC,'HKG');assert.equal(data.officialSpeedLimit,0);
 assert.throws(()=>buildAutoScanRequest(scan,{port:443,url:'auto',speedMin:.1,speedLimit:5},'../x'));
});
