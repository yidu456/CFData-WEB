import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
const load = async path => import('data:text/javascript;base64,' + Buffer.from(await readFile(new URL(path, import.meta.url))).toString('base64'));
const model = async () => {
  const browser = 'data:text/javascript;base64,' + Buffer.from(await readFile(new URL('../features/results/browser.js',import.meta.url))).toString('base64');
  const source = (await readFile(new URL('../features/results/output.js',import.meta.url),'utf8')).replace('./browser.js',browser);
  return import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));
};
const platform = () => load('../platform/output.js');
const rows = Array.from({length:105}, (_,i) => ({ip:`192.0.2.${i}`,port:'443',dc:'HKG',dcCountry:'HK',latency:`${i}ms`,lossRate:'0%',speed:i<5?'未测速':'1MB/s'}));

test('output normalizes official AVG and retains all NSB protocol metadata', async () => {
  const {normalizeOutputRows} = await model();
  const source = {IP:'2001:db8::1',Port:443,AvgLatency:12345678,MinLatency:1000000,DataCenter:'HKG',DCCountry:'HK',LossRate:.125,Speed:'1MB/s'};
  const [row] = normalizeOutputRows([source],'detail');
  assert.equal(row.latency,'12ms'); assert.equal(row.lossRate,'13%'); assert.equal(row.AvgLatency,12345678);
  assert.equal(row.ip,'2001:db8::1'); assert.equal(row.dcCountry,'HK'); assert.equal(source.ip,undefined);
  const nsb = {...rows[0],sni:'example.org',httpVersion:'HTTP/2',kex:'X25519',timestamp:'now'};
  assert.deepEqual(normalizeOutputRows([nsb],'nsb')[0],nsb);
});
test('copy defaults20 rows and independent filters precede range without mutating source', async () => {
  const {buildOutput,createOutputConfig} = await model();
  const copy = createOutputConfig('scan','copy'), file = createOutputConfig('scan','export');
  assert.equal(buildOutput(rows,copy).count,20); assert.equal(buildOutput(rows,copy).total,105);
  copy.qualified='qualified'; copy.start=2; copy.end=3;
  assert.equal(buildOutput(rows,copy,.1).content,'192.0.2.6:443#HKG-HK\n192.0.2.7:443#HKG-HK\n');
  assert.equal(buildOutput(rows,file).count,20); assert.equal(rows.length,105);
  assert.throws(()=>buildOutput(rows,{...copy,start:0}),/行/);
  assert.throws(()=>buildOutput(rows,{...copy,end:1}),/行/);
  assert.throws(()=>buildOutput(rows,{...copy,start:1.5}),/行/);
  assert.equal(buildOutput(rows,{...copy,start:1000,end:1001}).content,'');
});
test('export thresholds are strict, unknown metrics excluded; copy has no threshold', async () => {
  const {buildOutput,createOutputConfig} = await model();
  const sample=[rows[0],{...rows[1],latency:'5ms',lossRate:'5%'},{...rows[2],latency:'未知'}];
  const config={...createOutputConfig('detail','export'),latency:5,loss:100};
  assert.equal(buildOutput(sample,config).count,1);
  assert.equal(buildOutput(sample,{...config,latency:0,loss:5}).count,2);
  assert.equal(buildOutput(sample,{...config,kind:'copy',latency:5,loss:0}).count,3);
  assert.equal(buildOutput([{...rows[0],speed:'未测速'},{...rows[1],speed:'失败'},{...rows[2],speed:'0MB/s'}],{...config,latency:0,qualified:'unqualified'}).count,2);
});
test('TXT keeps IP:PORT prefix, ordered nonempty extras, actual space and IPv6 brackets once', async () => {
  const {buildOutput,createOutputConfig} = await model();
  const sample=[{ip:'[2001:db8::1]',port:'8443',dc:'HKG',loc:'HK',city:''}];
  const config={...createOutputConfig('nsb','copy'),fields:['loc','city','dc'],separator:' '};
  assert.equal(buildOutput(sample,config).content,'[2001:db8::1]:8443#HK HKG\n');
  assert.equal(buildOutput(sample,{...config,brackets:false,fields:['ipport']}).content,'2001:db8::1:8443\n');
  assert.deepEqual(createOutputConfig('nsb','copy').fields,['ipport','dc','loc']);
});
test('constants avoid source/reserved collisions and CSV quotes headers, quotes and CRLF', async () => {
  const {parseCustomFields,buildOutput,createOutputConfig} = await model();
  const constants=parseCustomFields('ip=IP:keep,tag=Title:one:two,tag=Other:three,__proto__=Proto:four');
  assert.equal(new Set(constants.map(field=>field.key)).size,4); assert.notEqual(constants[0].key,'ip');
  assert.notEqual(constants[3].key,'__proto__'); assert.equal(constants[1].value,'one:two');
  assert.throws(()=>parseCustomFields('missing-colon'),/常量/);
  const custom={key:'note',label:'a,"b"\r\nc',value:'v,"q"\r\nx'};
  const config={...createOutputConfig('nsb','export'),format:'csv',fields:['ip','note'],constants:[custom]};
  assert.equal(buildOutput([rows[0]],config).content,'IP地址,"a,""b""\r\nc"\n192.0.2.0,"v,""q""\r\nx"\n');
  assert.throws(()=>buildOutput(rows,{...config,fields:[]}),/字段/);
  assert.throws(()=>buildOutput(rows,{...config,format:'html'}),/格式/);
});
test('clipboard rejection falls back and restores focus; denied fallback reports failure', async () => {
  const {copyText} = await platform(); const events=[];
  const active={focus:()=>events.push('focus')};
  const textarea={style:{},select:()=>events.push('select'),remove:()=>events.push('remove')};
  const document={activeElement:active,createElement:()=>textarea,body:{append:()=>events.push('append')},execCommand:()=>true};
  await copyText('hello',{navigator:{clipboard:{writeText:async()=>{throw Error('denied');}}},document});
  assert.equal(textarea.value,'hello');assert.deepEqual(events,['append','select','remove','focus']);
  document.execCommand=()=>false;
  await assert.rejects(copyText('hello',{navigator:{},document}),/复制/);
  assert.equal(events.at(-1),'focus');
});
test('desktop download uses timestamp BOM MIME and revokes URL; Android gets plain content', async () => {
  const {downloadText} = await platform();let blob,revoked,link,scheduled;
  const environment={Blob,URL:{createObjectURL:value=>{blob=value;return 'blob:test';},revokeObjectURL:value=>{revoked=value;}},document:{createElement:()=>link={click(){},remove(){}}},setTimeout:callback=>{scheduled=callback;},now:new Date('2026-11-02T01:02:03Z')};
  downloadText('ip\n','my/file.csv','csv',environment);
  assert.equal(blob.type,'text/csv;charset=utf-8');assert.deepEqual([...new Uint8Array(await blob.arrayBuffer()).slice(0,3)],[239,187,191]);
  assert.match(link.download,/^my_file_20261102010203\.csv$/);scheduled();assert.equal(revoked,'blob:test');
  let native;
  downloadText('ip\n','native','txt',{...environment,CFDataAndroid:{saveTextFile:(...args)=>{native=args;}}});
  assert.deepEqual(native,['native_20261102010203.txt','ip\n']);
});
