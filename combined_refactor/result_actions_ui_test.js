// Offline navigation checks; the Chrome smoke verifies real DOM visibility and focus.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const html = fs.readFileSync(`${__dirname}/index.html`, 'utf8');
function extract(name) {
  const start = html.indexOf(`        function ${name}(`);
  assert.notEqual(start, -1, `missing ${name}`);
  const end = html.indexOf('\n        function ', start + 1);
  return html.slice(start, end === -1 ? html.indexOf('</script>', start) : end);
}
const elements = {};
let focused;
for (const id of ['resultModalMask', 'resultModalTitle', 'resultFormat', 'resultFileName', 'resultIPTypeFilterField', 'resultIPTypeFilter', 'resultQualifiedFilterField', 'resultQualifiedFilter', 'resultExportChoice', 'resultGitHubChoice', 'resultEdgetunnelChoice', 'resultHuaweiChoice', 'resultBack', 'githubAccountSelect', 'settingsMenu', 'settingsToggle', 'huaweiModalMask', 'huaweiAK', 'huaweiSK', 'huaweiBack', 'huaweiConfigStatus', 'huaweiSaveEncrypted', 'huaweiSaveStatus', 'noticeCardMask', 'huaweiDiscoveryStatus', 'huaweiDNSStatus', 'huaweiConfirm', 'huaweiRegion', 'huaweiProjectID', 'huaweiPrivatePath', 'huaweiLegacyStatus', 'huaweiLegacyMigrate', 'huaweiLegacyCleanup']) {
  const classes = new Set();
  elements[id] = { value: '', style: {}, dataset: {}, textContent: '', focus() { focused = id; }, classList: { add: c => classes.add(c), remove: c => classes.delete(c), contains: c => classes.has(c) } };
}
const sent = [], notices = [];
const opener = { focus() { focused = 'opener'; } };
const ctx = vm.createContext({
  document: { activeElement: opener, getElementById: id => elements[id] },
  currentMode: 'official', resultModalMode: 'scan', resultFieldOrder: ['ipport'], resultReturnFocus: null,
  huaweiFromResult: false, resultHuaweiLoaded: false, resultHuaweiLoading: false,
  getCurrentResultRows: () => [], getDefaultResultFieldOrder: () => ['ipport'],
  renderGitHubAccounts() {}, renderResultFieldOptions() {},
  showNoticeCard: text => notices.push(text), wsSend: payload => { sent.push(payload); return true; }, escapeHTML: text => text,
});
vm.runInContext(extract('openResultModal'), ctx);
if (html.includes('        function showResultActionView(')) vm.runInContext(extract('showResultActionView'), ctx);
vm.runInContext('openResultModal("test")', ctx);
assert.equal(elements.resultModalMask.classList.contains('active'), true, 'empty results must still open the destination chooser');
assert.equal(ctx.resultModalMode, 'test');
for (const name of ['showResultActionView', 'closeResultModal', 'openHuaweiPanel', 'closeHuaweiPanel', 'loadHuaweiConfig', 'handleResultDialogKey', 'handleHuaweiMessage']) vm.runInContext(extract(name), ctx);
vm.runInContext('showResultActionView("github")', ctx);
assert.equal(elements.resultModalMask.dataset.view, 'github');
elements.resultFileName.value = 'keep-name';
elements.resultFormat.value = 'csv';
vm.runInContext('showResultActionView("chooser");showResultActionView("edgetunnel");showResultActionView("chooser")', ctx);
assert.equal(elements.resultFileName.value, 'keep-name');
assert.equal(elements.resultFormat.value, 'csv');
assert.equal(focused, 'resultEdgetunnelChoice');
ctx.wsSend = () => false;
vm.runInContext('showResultActionView("huawei");closeHuaweiPanel()', ctx);
assert.equal(ctx.resultHuaweiLoaded, false, 'failed config send cannot claim a successful read');
ctx.wsSend = payload => { sent.push(payload); return true; };
vm.runInContext('showResultActionView("huawei")', ctx);
assert.equal(elements.resultModalMask.classList.contains('active'), false);
assert.equal(elements.huaweiModalMask.classList.contains('active'), true);
assert.deepEqual(sent.map(p => p.type), ['huawei_config_get'], 'navigation must only read basic config');
assert.equal(ctx.resultHuaweiLoaded, false, 'sending alone must not claim loaded configuration');
vm.runInContext('handleHuaweiMessage({type:"huawei_response",data:{action:"huawei_config_get",data:{regions:["cn-north-4"],projectId:"0123456789abcdef0123456789abcdef"}}})', ctx);
assert.equal(ctx.resultHuaweiLoaded, true);
assert.equal(ctx.resultHuaweiLoading, false);
elements.huaweiAK.value = 'unsaved-key';
elements.huaweiSK.value = 'unsaved-secret';
vm.runInContext('closeHuaweiPanel();showResultActionView("huawei")', ctx);
assert.equal(elements.huaweiAK.value, 'unsaved-key', 'back navigation must retain unsaved config in its original node');
assert.equal(elements.huaweiSK.value, 'unsaved-secret');
assert.equal(sent.length, 1, 're-entering must not reload over unsaved config');
vm.runInContext('closeHuaweiPanel()', ctx);
assert.equal(focused, 'resultHuaweiChoice');
vm.runInContext('showResultActionView("export");handleResultDialogKey({key:"Escape",preventDefault(){}})', ctx);
assert.equal(elements.resultModalMask.dataset.view, 'chooser');
assert.equal(focused, 'resultExportChoice');
vm.runInContext('handleResultDialogKey({key:"Escape",preventDefault(){}})', ctx);
assert.equal(elements.resultModalMask.classList.contains('active'), false);
assert.equal(focused, 'opener');
assert.equal(elements.huaweiAK.value, '');
assert.equal(elements.huaweiSK.value, '');
assert.deepEqual(sent.map(p => p.type), ['huawei_config_get']);
assert.deepEqual(notices, []);
ctx.currentMode = 'nsb';
elements.githubAccountSelect.value = '2';
vm.runInContext('openResultModal("scan")', ctx);
assert.equal(ctx.resultModalMode, 'nsb', 'legacy NSB entry must retain its own rows/format');
assert.equal(elements.githubAccountSelect.value, '2');
assert.equal(elements.resultIPTypeFilterField.style.display, '');
console.log('Result destination chooser/back/config preservation and read-only navigation pass (offline)');
