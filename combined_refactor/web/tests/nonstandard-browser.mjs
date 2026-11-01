import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir} from 'node:fs/promises';
import path from 'node:path';
const {chromium}=createRequire(import.meta.url)(process.env.PLAYWRIGHT_MODULE||'playwright');
const base=process.argv[2]||'http://127.0.0.1:13336';
const output=path.resolve('../build/workbench-nsb');await mkdir(output,{recursive:true});
const browser=await chromium.launch({headless:true,channel:process.env.BROWSER_CHANNEL||'msedge'});
try{
 const context=await browser.newContext({viewport:{width:1366,height:950}});const commands=[],errors=[];let socket;
 const emit=(type,data)=>socket.send(JSON.stringify({type,data}));
 await context.routeWebSocket('**/ws',ws=>{socket=ws;ws.onMessage(raw=>{const m=JSON.parse(raw);commands.push(m);if(m.type==='get_background_task_status')emit('background_task_missing',null);});emit('init_config',{version:'nsb-test'});});
 const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));await page.goto(base+'/workbench');
 await page.locator('[data-mode="nonstandard"]').click();
 await page.evaluate(()=>{
   window.resolveNSBFile=null;
   const input=document.querySelector('[data-nsb-file]');
   Object.defineProperty(input,'files',{configurable:true,value:[{name:'pending.csv',size:1,text:()=>new Promise(resolve=>window.resolveNSBFile=resolve)}]});input.dispatchEvent(new Event('change'));
   Object.defineProperty(input,'files',{configurable:true,value:[{name:'oversize.csv',size:17*1024*1024}]});input.dispatchEvent(new Event('change'));
   window.resolveNSBFile('stale input');delete input.files;
 });
 await page.waitForFunction(()=>document.querySelector('[data-nsb-input-status]').textContent.includes('16MiB'));
 await page.waitForFunction(()=>!document.querySelector('[data-nsb-start]').disabled);
 await page.locator('[data-nsb-file]').setInputFiles({name:'ips.csv',mimeType:'text/csv',buffer:Buffer.from('192.0.2.1,443')});
 await page.waitForFunction(()=>document.querySelector('[data-nsb-input-status]').textContent.includes('ips.csv'));
 await page.locator('[data-nsb-form] [name="enableTLS"]').selectOption('false');
 assert.equal(await page.locator('[data-nsb-form] [name="fallbackPort"]').inputValue(),'80');
 await page.locator('[data-nsb-start]').click();
 const request=commands.find(m=>m.type==='start_nsb_task').data;assert.equal(request.fileName,'ips.csv');assert.equal(request.fileContent,'192.0.2.1,443');assert.equal(request.fallbackPort,80);assert.equal(request.speedTest,0);
 emit('nsb_progress',{phase:'scan',current:1,total:2,text:'扫描中'});emit('nsb_scan_result',{ip:'192.0.2.1',port:'80',city:'<img src=x>',latency:'12ms'});emit('nsb_scan_result',{ip:'192.0.2.1',port:'80',city:'<img src=x>',speed:'1MB/s'});
 await page.waitForFunction(()=>document.querySelectorAll('[data-nsb-rows] tr').length===1);assert.equal(await page.locator('[data-nsb-rows] img').count(),0);
 const headers=Array.from({length:25},(_,i)=>'列'+i);emit('nsb_csv_complete',{headers,rows:[headers.map((_,i)=>String(i))],status:'partial',message:'已整理结果',file:'ip.csv'});emit('task_complete',null);
 await page.waitForFunction(()=>document.querySelectorAll('[data-nsb-headers] th').length===25);assert.equal(await page.locator('[data-nsb-rows] td').count(),25);
 await page.locator('[data-mode="official"]').click();await page.locator('[data-mode="nonstandard"]').click();assert.equal(await page.locator('[data-nsb-headers] th').count(),25);
 for(const width of [320,375,768,1366]){await page.setViewportSize({width,height:950});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth),true);if(width===375||width===1366)await page.screenshot({path:path.join(output,width===375?'mobile.png':'desktop.png'),fullPage:true});}
 await page.locator('[data-nsb-source]').selectOption('url');await page.locator('[data-nsb-form] [name="sourceURL"]').fill('https://example.test/list');await page.locator('[data-nsb-start]').click();
 const second=commands.filter(m=>m.type==='start_nsb_task').at(-1).data;assert.equal(second.fileContent,'');assert.equal(second.sourceURL,'https://example.test/list');
 emit('error','无有效输入');emit('task_complete',null);await page.waitForFunction(()=>!document.querySelector('[data-nsb-start]').disabled);
 assert.deepEqual(errors,[]);await context.close();console.log('PASS: NSB file/URL exclusivity,TLS defaults,validated request,live dedupe/safe text,25-column final,partial/mode persistence,responsive,error.');
}finally{await browser.close();}
