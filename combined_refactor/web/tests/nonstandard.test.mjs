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

test('NSB auto speed links public probe and preserves custom execution fields',async()=>{
 const {buildNSBExecution}=await load();const cfg={...values,autoSpeed:'on',speedTest:'5',speedURL:'auto',speedMin:'.1',speedLimit:'5',relinkPreset:'off',relinkHost:''};
 const data=buildNSBExecution(cfg);assert.equal(data.speedTest,5);assert.equal(data.relinkHost,'probe.609909.xyz');
 assert.equal(buildNSBExecution({...cfg,autoSpeed:'off'}).speedTest,0);
 assert.throws(()=>buildNSBExecution({...cfg,relinkPreset:'custom',relinkHost:'javascript:alert(1)'}));
 assert.equal(buildNSBExecution({...cfg,relinkPreset:'custom',relinkHost:'https://probe.example.test:8443/path?key=x'}).relinkHost,'probe.example.test');
});

test('NSB final CSV endpoint lookup is indexed for later speed events',async()=>{
 const {createNSBState,reduceNSB}=await load();
 const state=reduceNSB(createNSBState(),{type:'nsb_csv_complete',data:{headers:['IP地址','端口号','下载速度'],rows:[['192.0.2.1','443','']]}});
 assert.equal(state.csvIndex.get('192.0.2.1:443'),0);
});
test('NSB CSV endpoint mapping supports batch/continue and speed updates retain headers',async()=>{
 const {createNSBState,reduceNSB,getNSBResultRows,buildNSBFollowup}=await load();
 const headers=['IP地址','端口号','TLS','丢包率','扫描方式','网络延迟','下载速度','出站IP','IP类型','原始输入','数据中心','源IP位置','地区','城市','ASN号码','ASN组织','访问协议','TLS版本','SNI','HTTP版本','WARP','Gateway','RBI','密钥交换','时间戳'];
 const cells=['192.0.2.1','443','true','0%','TCPing','12ms','1MB/s','198.51.100.1','IPv4','example.test','HKG','HK','亚洲','香港','1','org','https','TLSv1.3','example.test','h2','off','off','off','X25519','now'];
 let state=reduceNSB(createNSBState(),{type:'nsb_csv_complete',data:{headers,rows:[cells],status:'complete'}});
 assert.equal(getNSBResultRows(state)[0].originalInput,'example.test');assert.equal(getNSBResultRows(state)[0].kex,'X25519');
 const cfg={...values,autoSpeed:'on',speedTest:'5',speedURL:'auto',speedMin:'.1',speedLimit:'5',relinkPreset:'off',relinkHost:''};
 assert.equal(buildNSBFollowup(cfg,state,'speed',true).skipTested,true);
 state=reduceNSB(state,{type:'nsb_scan_result',data:{ip:'192.0.2.1',port:'443',speed:'2MB/s'}});
 assert.equal(state.csvRows[0][6],'2MB/s');assert.equal(state.csvRows[0][23],'X25519');
});
