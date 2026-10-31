const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const html = fs.readFileSync(`${__dirname}/index.html`, 'utf8');
function extract(name) {
  const match = new RegExp(`        (?:async )?function ${name}\\(`).exec(html);
  assert.ok(match, `missing ${name}`);
  const next = /\n        (?:async )?function /.exec(html.slice(match.index + 1));
  const end = next ? match.index + 1 + next.index : -1;
  return html.slice(match.index, end === -1 ? html.indexOf('</script>', match.index) : end);
}
function scenario() {
  const sent = [];
  const elements = Object.fromEntries(['resultFormat','githubRepo','githubToken','githubBranch','githubPath','githubMessage','webBatchSummary','currentTestDC','officialTestPort','officialDelay','test'].map(id=>[id,{value:'',textContent:'',classList:{contains:()=>true}}]));
  elements.resultFormat.value='txt';elements.githubRepo.value='owner/repo';elements.githubToken.value='fake-token';
  const state=vm.createContext({document:{getElementById:id=>elements[id],querySelector:()=>({innerHTML:''})},currentMode:'official',resultModalMode:'test',testData:[],scanData:[],completedWebBatch:null,webBatchDerivedResults:false,isStoppingTask:false,isTaskRunning:false,webBatchRunning:false,
    webStatePending:new Map(),webStateLoaded:false,webStateDirty:new Set(),handleWebStateMessage:()=>false,resetLoadedPublishOptions(){},loadWebState:async()=>{},setWebStateStatus(){},
    wsSend:p=>sent.push(p),sendGitHubUpload:p=>sent.push({type:'github_upload',data:p}),readQuickUploadFile:async()=>null,saveCurrentGitHubAccount:async()=>{},showNoticeCard(){},log(){},escapeHTML:s=>s,
    renderAllTestResults(){},updateProgress(){},clearHuaweiPreview(){},handleHuaweiDisconnectFeedback(){},testResultToObject:r=>({ip:r.IP}),buildSelectedResultContent:()=>state.testData.map(r=>r.IP).join('\n'),testedDCs:new Set(),
    clearStopWatchdog(){},resetCurrentSpeedButton(){},finishRunningTask(){state.isTaskRunning=false;},updateBatchSpeedButton(){},updateNSBSpeedButtons(){},finishCompactIPv4(){},showDCSelection(){},renderCurrentScanResults(){},dcList:[],runningTaskMode:'official',progressBar:{style:{}},
    backgroundTaskActive:false,backgroundTaskFollowing:false,officialBatchSpeedRunning:false,window:{location:{protocol:'http:',host:'127.0.0.1'}},WebSocket:class { constructor(){} },setTimeout(){},updateStatus(){},
    requireProxyWarning:()=>false,setOfficialTestQualifiedFilter(){},updateBadge(){},setStartLoading(){},getRunningButtonText:()=>'',resetProgress(){},progressBox:{style:{}},getScanMode:()=>'',switchTab(){},
    handleHuaweiMessage:()=>false,onTestComplete(){},autoUploadOfficialResults(){},testResultsCount:0,officialTestQualifiedFilter:'all',getFilteredOfficialTestData:()=>state.testData,matchesOfficialQualifiedFilter:()=>true,renderTestRow(){},officialAutoSpeedPending:false,officialAutoStage:'',
  });
  for(const name of ['uploadSelectedResultsToGitHub','handleWebBatchMessage','handleMessage','addTestResult','getCurrentResultRows','selectedGitHubSettings','resetStoppedTaskUI','connect','selectDC'])vm.runInContext(extract(name),state);
  vm.runInContext(extract('autoUploadOfficialResults'),state);
  state.autoUploadResults=async()=>sent.push({type:'legacy_auto_upload'});
  return {state,sent};
}
(async()=>{
  for(const name of ['incomplete','failed','cancelled','disconnected','retest-invalidated']) {
    const {state,sent}=scenario();
    vm.runInContext(`handleWebBatchMessage({type:'web_batch_started',data:{policy:{Datacenters:['NRT']},candidateCount:1}});handleWebBatchMessage({type:'web_batch_result',data:{result:{IP:'192.0.2.2'},qualified:false}})`,state);
    if(name==='failed'||name==='cancelled')vm.runInContext(`handleWebBatchMessage({type:'web_batch_failed',data:{status:'${name}',message:'fixture'}})`,state);
    if(name==='cancelled')vm.runInContext(`resetStoppedTaskUI('cancelled')`,state);
    if(name==='retest-invalidated'||name==='disconnected') {
      vm.runInContext(`handleWebBatchMessage({type:'web_batch_complete',data:{id:7,results:[{IP:'192.0.2.2'}],policy:{Datacenters:['NRT'],Limit:10},measuredCount:1,qualifiedCount:1}})`,state);
      if(name==='disconnected')vm.runInContext('connect();ws.onclose()',state);else vm.runInContext(`handleWebBatchMessage({type:'web_batch_invalidated',data:null})`,state);
    }
    assert.equal(state.testData.length,1,'fixture must retain inspectable batch rows');
    await vm.runInContext('uploadSelectedResultsToGitHub()',state);
    assert.deepEqual(sent,[],`${name} Web-batch rows fell through legacy uploader`);
  }
  {
    const {state,sent}=scenario();
    vm.runInContext(`handleWebBatchMessage({type:'web_batch_complete',data:{id:7,results:[{IP:'192.0.2.2'}],policy:{Datacenters:['NRT'],Limit:10},measuredCount:1,qualifiedCount:1}})`,state);
    await vm.runInContext('uploadSelectedResultsToGitHub()',state);
    assert.equal(sent[0].type,'web_github_publish');assert.equal(sent[0].data.batchId,7);assert.equal(sent[0].data.content,undefined);
  }
  for(const mode of ['legacy','quick-file','nsb']) {
    const {state,sent}=scenario();state.testData=[{IP:'192.0.2.3'}];
    if(mode==='quick-file'){state.webBatchDerivedResults=true;state.readQuickUploadFile=async()=>({name:'file.txt',content:'explicit file'});}
    if(mode==='nsb'){state.currentMode='nsb';state.resultModalMode='nsb';state.webBatchDerivedResults=true;}
    await vm.runInContext('uploadSelectedResultsToGitHub()',state);
    assert.equal(sent[0].type,'github_upload',`${mode} compatibility changed`);
  }
  {
    const {state,sent}=scenario();state.webBatchDerivedResults=true;state.testData=[{IP:'192.0.2.2'}];
    vm.runInContext(`selectDC('NRT')`,state);
    assert.equal(state.webBatchDerivedResults,false,'genuine legacy detailed test did not replace batch provenance');
    state.testData=[{IP:'192.0.2.3'}];state.isTaskRunning=false;sent.length=0;
    await vm.runInContext('uploadSelectedResultsToGitHub()',state);assert.equal(sent[0].type,'github_upload');
  }
  for(const page of ['original-tab','fresh-follower']) {
    const original=scenario();
    vm.runInContext(`handleWebBatchMessage({type:'web_batch_complete',data:{id:7,results:[{IP:'192.0.2.2',WebBatchDerived:true}],policy:{Datacenters:['NRT'],Limit:10},measuredCount:1,qualifiedCount:1}});handleWebBatchMessage({type:'web_batch_invalidated',data:null});connect();ws.onclose()`,original.state);
    const {state,sent}=page==='original-tab'?original:scenario();
    // Actual fallback messages sent after background individual retest completes without webBatch authority.
    vm.runInContext(`handleMessage({type:'test_result',data:{IP:'192.0.2.2',Speed:'URL解析错误',WebBatchDerived:true}})`,state);
    assert.equal(state.webBatchDerivedResults,true,`${page} current replay result did not retain row provenance`);
    await vm.runInContext('uploadSelectedResultsToGitHub()',state);assert.deepEqual(sent,[]);
    vm.runInContext(`handleMessage({type:'test_complete',data:[{IP:'192.0.2.2',Speed:'URL解析错误',WebBatchDerived:true}]})`,state);
    assert.equal(state.testData.length,1,'replayed rows must remain inspectable');
    assert.equal(state.completedWebBatch,null,'retest replay must not invent completion authority');
    await vm.runInContext('uploadSelectedResultsToGitHub()',state);
    assert.deepEqual(sent,[],`${page} background retest replay resurrected legacy upload`);
    // A genuinely legacy replacement collection restores the existing upload route.
    vm.runInContext(`handleMessage({type:'test_complete',data:[{IP:'192.0.2.3',Speed:'2MB/s'}]})`,state);
    assert.equal(sent[0].type,'legacy_auto_upload',`${page} genuine legacy auto route changed`);
    sent.length=0;
    await vm.runInContext('uploadSelectedResultsToGitHub()',state);
    assert.equal(sent[0].type,'github_upload',`${page} genuine legacy replay did not restore route`);
  }
  console.log('Actual manual-upload caller rejects invalid Web-batch provenance; completed backend, quick-file, NSB and legacy routes pass (offline)');
})().catch(err=>{console.error(err);process.exitCode=1;});
