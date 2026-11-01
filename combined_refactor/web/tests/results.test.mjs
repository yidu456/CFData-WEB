import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
const load=async()=>import('data:text/javascript;base64,'+Buffer.from(await readFile(new URL('../features/results/browser.js',import.meta.url))).toString('base64'));
test('result filters are strict and pagination keeps source intact',async()=>{
 const {browseResultRows}=await load();const rows=Array.from({length:105},(_,i)=>({IP:`192.0.2.${i}`,Port:443,minMS:i,maxMS:i,avgMS:i,lossPercent:0,Speed:'1MB/s'}));
 const old=rows.slice();const page=browseResultRows(rows,{page:2},'detail');assert.equal(page.total,105);assert.deepEqual(page.indexes,[100,101,102,103,104]);assert.deepEqual(rows,old);
 assert.equal(browseResultRows(rows,{latency:5},'detail').total,5);
 assert.equal(browseResultRows([{IP:'2001:db8::1',avgMS:null,lossPercent:null}],{latency:5},'detail').total,0);
 assert.equal(browseResultRows(rows,{loss:0},'detail').total,0);
});
test('three-level sorting deduplicates priorities, orders speed states and is stable',async()=>{
 const {browseResultRows}=await load();const rows=[{IP:'a',minMS:1,maxMS:2,avgMS:3,lossPercent:0,Speed:'失败'},{IP:'b',minMS:1,maxMS:2,avgMS:3,lossPercent:0,Speed:''},{IP:'c',minMS:1,maxMS:2,avgMS:3,lossPercent:0,Speed:'2MB/s'},{IP:'d',minMS:1,maxMS:2,avgMS:3,lossPercent:0,Speed:'2MB/s'}];
 assert.deepEqual(browseResultRows(rows,{sort:['speed','speed','loss']},'detail').indexes,[2,3,1,0]);
 assert.equal(browseResultRows(rows,{qualified:'qualified',speedMin:2},'detail').total,2);
 assert.equal(browseResultRows(rows,{qualified:'unqualified',speedMin:2},'detail').total,1);
});
test('NSB CSV string metrics and IPv6 type filters preserve source indexing',async()=>{
 const {browseResultRows}=await load();const headers=['IP地址','网络延迟','丢包率','下载速度'];const rows=[['192.0.2.1','12ms','5%','0.09MB/s'],['2001:db8::1','5ms','0%','1MB/s']];
 const result=browseResultRows(rows,{ipType:'6',latency:10,loss:1,sort:['latency']},'nsb',headers);assert.deepEqual(result.indexes,[1]);
});
test('numeric zero speed still sorts before untested and nonnumeric failures',async()=>{
 const {browseResultRows}=await load();const rows=[{IP:'a',Speed:'失败'},{IP:'b',Speed:''},{IP:'c',Speed:'0MB/s'}];
 assert.deepEqual(browseResultRows(rows,{sort:['speed']},'detail').indexes,[2,1,0]);
});
