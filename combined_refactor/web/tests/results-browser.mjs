import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
const {chromium}=createRequire(import.meta.url)(process.env.PLAYWRIGHT_MODULE||'playwright');
const base=process.argv[2]||'http://127.0.0.1:13336';const browser=await chromium.launch({headless:true,channel:process.env.BROWSER_CHANNEL||'msedge'});
try{
 const context=await browser.newContext({viewport:{width:1366,height:950}});const commands=[];let socket;const emit=(type,data)=>socket.send(JSON.stringify({type,data}));
 await context.routeWebSocket('**/ws',ws=>{socket=ws;ws.onMessage(raw=>{const m=JSON.parse(raw);commands.push(m);if(m.type==='get_background_task_status')emit('background_task_missing',null);});emit('init_config',{version:'browse-test'});});
 const page=await context.newPage();await page.goto(base+'/workbench');await page.locator('[data-mode="nonstandard"]').click();
 const headers=['IP地址','端口号','网络延迟','丢包率','下载速度'];const rows=Array.from({length:105},(_,i)=>[`192.0.2.${i}`,'443',i+'ms','0%','未测速']);
 emit('nsb_csv_complete',{headers,rows,status:'complete'});emit('task_complete',null);
 await page.waitForFunction(()=>document.querySelector('[data-browse="nsb"] [data-page-status]').textContent.includes('105'));
 assert.equal(await page.locator('[data-nsb-rows] tr').count(),100);
 await page.locator('[data-browse="nsb"] [data-page="next"]').click();assert.equal(await page.locator('[data-nsb-rows] tr').count(),5);
 await page.locator('[data-browse="nsb"] [data-browse-field="latency"]').fill('1');await page.locator('[data-browse="nsb"] [data-browse-field="latency"]').dispatchEvent('change');
 assert.equal(await page.locator('[data-nsb-rows] tr').count(),1);
 await page.locator('[data-nsb-speed]').click();assert.equal(commands.find(m=>m.type==='start_nsb_speed_batch').data.results.length,105);
 emit('task_stopped',null);
 await page.locator('[data-mode="official"]').click();await page.locator('[data-mode="nonstandard"]').click();assert.equal(await page.locator('[data-nsb-rows] tr').count(),1);
 emit('nsb_csv_complete',{headers,rows:[['192.0.2.1','443','0ms','0%','1MB/s']],status:'complete'});
 await page.locator('[data-browse="nsb"] [data-browse-field="qualified"]').selectOption('qualified');
 await page.waitForFunction(()=>document.querySelectorAll('[data-nsb-rows] tr').length===1);
 await page.locator('[data-nsb-form] [name="speedMin"]').fill('2');
 await page.waitForFunction(()=>document.querySelectorAll('[data-nsb-rows] tr').length===0);
 for(const width of [320,375,768,1366]){await page.setViewportSize({width,height:950});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth),true);}
 await context.close();console.log('PASS: all105 rows paginated,strict view filter,full105 measurement scope,mode persistence,responsive result controls.');
}finally{await browser.close();}
