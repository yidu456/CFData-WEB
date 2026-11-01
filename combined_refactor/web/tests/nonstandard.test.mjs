import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
const load=async()=>import('data:text/javascript;base64,'+Buffer.from(await readFile(new URL('../features/nonstandard/model.js',import.meta.url))).toString('base64'));
const values={maxThreads:'100',fallbackPort:'443',delay:'0',resultLimit:'1000',enableTLS:'true',scanMode:'tcping',compact:'false',outFile:'ip.csv',dc:''};
test('NSB input validates mutually exclusive source and scan values',async()=>{
 const {buildNSBRequest}=await load();const data=buildNSBRequest(values,{fileName:'test.csv',fileContent:'192.0.2.1'});
 assert.equal(data.maxThreads,100);assert.equal(data.delay,0);assert.equal(data.resultLimit,1000);assert.equal(data.enableTLS,true);assert.equal(data.speedTest,0);assert.equal(data.compact,false);
 assert.throws(()=>buildNSBRequest(values,{}));assert.throws(()=>buildNSBRequest(values,{fileContent:'a',sourceURL:'https://example.test/a'}));
 assert.throws(()=>buildNSBRequest({...values,resultLimit:'0'},{fileContent:'a'}));
 assert.throws(()=>buildNSBRequest(values,{sourceURL:'ftp://example.test/a'}));
 assert.equal(buildNSBRequest(values,{sourceURL:'https://example.test/list'}).sourceURL,'https://example.test/list');
});
test('NSB live endpoints deduplicate while final CSV retains complete fields and partial status',async()=>{
 const {createNSBState,reduceNSB}=await load();let state=createNSBState();
 state=reduceNSB(state,{type:'nsb_scan_result',data:{ip:'192.0.2.1',port:'443',originalInput:'example.test',latency:'12ms',speed:''}});
 state=reduceNSB(state,{type:'nsb_scan_result',data:{ip:'192.0.2.1',port:'443',originalInput:'example.test',latency:'12ms',speed:'1MB/s'}});
 assert.equal(state.rows.length,1);assert.equal(state.rows[0].speed,'1MB/s');
 const headers=Array.from({length:25},(_,i)=>'字段'+i),rows=[headers.map((_,i)=>String(i))];
 state=reduceNSB(state,{type:'nsb_csv_complete',data:{headers,rows,status:'partial',file:'ip.csv'}});
 assert.equal(state.headers.length,25);assert.equal(state.csvRows[0].length,25);assert.equal(state.status,'partial');
 state=reduceNSB(state,{type:'task_stopped'});assert.equal(state.csvRows.length,1);
 state=reduceNSB(state,{type:'nsb_requested',data:{startedAt:'new'}});assert.equal(state.rows.length,0);assert.equal(state.csvRows.length,0);
});
test('NSB sorted/follow updates isolate task identity and preserve endpoint strings',async()=>{
 const {createNSBState,reduceNSB}=await load();let state=reduceNSB(createNSBState(),{type:'nsb_requested',data:{startedAt:'one'}});
 state=reduceNSB(state,{type:'nsb_scan_sorted',data:[{ip:'2001:db8::1',port:'8443',tls:'true',latency:'20ms'}]});
 assert.equal(state.rows[0].ip,'2001:db8::1');
 state=reduceNSB(state,{type:'background_task_following',data:{mode:'nsb',startedAt:'two'}});assert.equal(state.rows.length,0);
});
