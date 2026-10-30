// Offline assertions against the embedded frontend, with no browser/network startup.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const html = fs.readFileSync(`${__dirname}/index.html`, 'utf8');
const scripts = [...html.matchAll(/<script(?:\s[^>]*)?>([\s\S]*?)<\/script>/g)].map(m => m[1]);
scripts.forEach(script => new vm.Script(script));

function extractFunction(name) {
  const start = html.indexOf(`        function ${name}(`);
  assert.notEqual(start, -1, `missing ${name}`);
  const end = html.indexOf('\n        function ', start + 1);
  return html.slice(start, end === -1 ? html.indexOf('</script>', start) : end);
}

assert.match(html, /id="webBatchLimit"[^>]*value="10"/);
assert.match(html, /id="btnWebBatch"[^>]*onclick="startWebBatch\(\)"/);
assert.match(html, /id="webBatchSummary"[^>]*aria-live="polite"/);
assert.ok(html.includes('<details id="webDCSelector"'), 'missing top discovered-DC dropdown');
assert.ok(html.includes('id="officialAutoTargetSection"'), 'legacy single-center target not separated');
const elements = Object.fromEntries([
  ['officialTestPort', '443'], ['officialDelay', '500'], ['officialSpeedMin', '1'], ['webBatchLimit', '10'],
].map(([id, value]) => [id, { value }]));
elements.btnWebBatch = {};
elements.webBatchSelection = {};
elements.webBatchSummary = {};
elements.webDCSummary = {};
elements.webDCEmpty = {};
elements.webDCChoices = { dataset: {} };
elements.officialAutoSpeed = { value: 'off' };
elements.officialTargetDC = { value: '' };
elements.officialAutoTargetSection = { style: {} };
const checkboxes = ['NRT', 'LHR', 'GRU'].flatMap(dc => [{ dataset: { dc } }, { dataset: { dc } }]);
const tbody = { innerHTML: '', insertAdjacentHTML(_where, row) { this.innerHTML += row; } };
elements.dcSection = { style: {} };
const state = vm.createContext({
  document: { getElementById: id => elements[id], querySelector: () => tbody, querySelectorAll: () => checkboxes },
  dcList: [{ DataCenter: 'NRT', City: 'Tokyo' }, { DataCenter: 'LHR', City: 'London' }, { DataCenter: 'GRU', City: 'Sao Paulo' }],
  selectedWebDCs: new Set(['NRT', 'GRU']), testedDCs: new Set(), currentTestingDC: '',
  isTaskRunning: false, webBatchRunning: false, currentMode: 'official',
  resolveSpeedTestUrl: () => 'speed.cloudflare.com/__down', getScanMode: () => 'tcping',
  escapeHTML: value => String(value ?? '').replaceAll('<', '&lt;'),
  getLatencyColorFactor: () => 1, getLatencyColor: () => 'inherit', updateBadge() {},
});
for (const name of ['getWebBatchRequest', 'updateWebBatchSelection', 'toggleWebDatacenter', 'showDCSelection']) {
  vm.runInContext(extractFunction(name), state);
}
let request = JSON.parse(vm.runInContext('JSON.stringify(getWebBatchRequest())', state));
assert.deepEqual(request.datacenters, ['GRU', 'NRT']);
assert.equal(request.limit, 10);
assert.equal(request.minSpeedMB, 1);
vm.runInContext('showDCSelection(dcList)', state);
assert.equal((tbody.innerHTML.match(/type="checkbox"/g) || []).length, 3);
assert.match(tbody.innerHTML, /aria-label="[^"\n]*GRU/);
assert.match(tbody.innerHTML, /selectDC\('LHR'\)/); // Legacy row action remains.
assert.equal(state.selectedWebDCs.size, 2); // Rendering does not erase selection.
vm.runInContext("toggleWebDatacenter('LHR', true)", state);
assert.equal(state.selectedWebDCs.size, 3);
assert.ok(checkboxes.every(input => input.checked), 'top/table checkbox state not synchronized');
assert.equal((elements.webDCChoices.innerHTML.match(/type="checkbox"/g) || []).length, 3);
vm.runInContext(extractFunction('updateOfficialAutoSpeedControls'), state);
vm.runInContext('updateOfficialAutoSpeedControls()', state);
assert.equal(elements.officialAutoTargetSection.style.display, 'none');
assert.equal(state.selectedWebDCs.size, 3, 'auto-speed off erased multi-selection');
elements.officialAutoSpeed.value = 'on';
elements.officialTargetDC.value = 'LHR';
vm.runInContext('updateOfficialAutoSpeedControls()', state);
assert.equal(elements.officialAutoTargetSection.style.display, 'flex');
assert.equal(elements.officialTargetDC.disabled, false);
vm.runInContext(extractFunction('readFormValues'), state);
assert.equal(JSON.parse(vm.runInContext('JSON.stringify(readFormValues())', state)).official.targetDC, 'LHR');
Object.assign(state, { officialAutoSpeedPending: true, officialAutoStage: 'scan', selectDC(dc) { state.legacySelectedDC = dc; } });
vm.runInContext(extractFunction('maybeStartOfficialAutoTest'), state);
vm.runInContext('maybeStartOfficialAutoTest()', state);
assert.equal(state.legacySelectedDC, 'LHR', 'legacy auto target changed to multi-select');
state.isTaskRunning = true;
vm.runInContext('updateWebBatchSelection()', state);
assert.ok(checkboxes.every(input => input.disabled), 'running task permits checkbox changes');
state.isTaskRunning = false;
elements.webBatchLimit.value = '0';
assert.throws(() => vm.runInContext('getWebBatchRequest()', state), /./);
elements.webBatchLimit.value = '10';
elements.officialSpeedMin.value = 'not a number';
assert.throws(() => vm.runInContext('getWebBatchRequest()', state), /./);
assert.match(html, /case 'web_batch_complete'/);
assert.match(html, /case 'web_batch_failed'/);
assert.match(html, /case 'web_batch_progress'/);
Object.assign(state, {
  isStoppingTask: false, completedWebBatch: null, testData: [],
  renderAllTestResults() {}, updateProgress() {},
});
vm.runInContext(extractFunction('handleWebBatchMessage'), state);
vm.runInContext(`handleWebBatchMessage({ type: 'web_batch_result', data: { result: { IP: '192.0.2.1', Speed: '1.00MB/s' }, qualified: false } })`, state);
assert.equal(state.testData[0].BatchQualified, false); // 0.999 rounding must not qualify.
vm.runInContext(`handleWebBatchMessage({ type: 'web_batch_complete', data: { id: 7, results: [{ IP: '192.0.2.9' }, { IP: '192.0.2.1' }], policy: { Datacenters: ['NRT', 'GRU'], Limit: 10 }, measuredCount: 20, qualifiedCount: 12 } })`, state);
assert.equal(state.testData.length, 2);
assert.equal(state.testData[0].IP, '192.0.2.9');
assert.equal(state.testData[0].BatchRank, 1);
assert.equal(state.completedWebBatch.id, 7);
vm.runInContext('showDCSelection(dcList)', state);
assert.equal(state.completedWebBatch.id, 7); // Rendering cannot clear verified batch ID.
Object.assign(state, {
  webBatchRunning: true, isTaskRunning: true, currentMode: 'official',
  finishRunningTask() { state.isTaskRunning = false; },
  autoUploadOfficialResults() { throw new Error('legacy upload callback must not control batch completion'); },
});
vm.runInContext(extractFunction('handleMessage'), state);
vm.runInContext(`handleMessage({ type: 'task_complete', data: null })`, state);
assert.equal(state.webBatchRunning, false);
assert.equal(state.isTaskRunning, false);
assert.equal(state.completedWebBatch.id, 7);
vm.runInContext(`handleWebBatchMessage({ type: 'web_batch_failed', data: { status: 'cancelled', message: 'cancelled' } })`, state);
assert.equal(state.completedWebBatch, null);
assert.equal(state.testData.length, 2); // Inspecting partial rows is allowed, publishing them is not.
state.scanData = [{ IP: '192.0.2.1', DataCenter: 'FRA', City: 'wrong scan row' }];
vm.runInContext(extractFunction('testResultToObject'), state);
const metadata = JSON.parse(vm.runInContext(`JSON.stringify(testResultToObject({ IP: '192.0.2.1', DataCenter: 'NRT', City: 'Tokyo', AvgLatency: 100000000 }))`, state));
assert.equal(metadata.dc, 'NRT');
assert.equal(metadata.city, 'Tokyo');
Object.assign(state, { syncOfficialSpeedConfigState() {}, syncNSBSpeedConfigState() {}, updateNSBSpeedControls() {}, updateNSBCompactToggle() {} });
vm.runInContext(extractFunction('setValue'), state);
vm.runInContext(extractFunction('applyConfigValues'), state);
vm.runInContext(`applyConfigValues({ official: { autoSpeed: 'on', targetDC: 'NRT' } })`, state);
assert.equal(elements.officialTargetDC.value, 'NRT');
assert.equal(elements.officialAutoTargetSection.style.display, 'flex');
vm.runInContext(extractFunction('applyBackgroundParams'), state);
vm.runInContext(`applyBackgroundParams({ datacenters: ['NRT', 'GRU'], finalLimit: 7 })`, state);
assert.deepEqual([...state.selectedWebDCs].sort(), ['GRU', 'NRT']);
assert.equal(elements.webBatchLimit.value, 7);
vm.runInContext('showDCSelection([])', state);
assert.equal(elements.webDCEmpty.hidden, false);
assert.equal(elements.webDCChoices.hidden, true);
assert.equal(state.selectedWebDCs.size, 0);
for (const id of ['backgroundStatusCard', 'backgroundStatusTitle', 'backgroundSource', 'backgroundPhase', 'backgroundProgressText', 'backgroundResultCount', 'backgroundSpeedCount', 'btnFollowBackground']) {
  elements[id] = { classList: { toggle() {} } };
}
state.backgroundTaskFollowing = false;
vm.runInContext(extractFunction('renderBackgroundStatusCard'), state);
vm.runInContext(`renderBackgroundStatusCard({ webBatchStatus: 'failed', phase: '测试失败', running: false })`, state);
assert.ok(!elements.backgroundStatusTitle.textContent.startsWith('✅'), 'failed background batch presented as successful');
vm.runInContext(`renderBackgroundStatusCard({ webBatchStatus: 'cancelled', phase: '已停止', running: false })`, state);
assert.ok(!elements.backgroundStatusTitle.textContent.startsWith('✅'), 'cancelled background batch presented as successful');
console.log('Embedded scripts syntax and multi-DC UI assertions passed');
