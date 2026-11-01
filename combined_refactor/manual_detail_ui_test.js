const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const html = fs.readFileSync(`${__dirname}/index.html`, 'utf8');
function extract(name) {
  const start = html.indexOf(`        function ${name}(`);
  assert.ok(start >= 0, `missing ${name}`);
  const end = html.indexOf('\n        function ', start + 1);
  return html.slice(start, end < 0 ? html.indexOf('</script>', start) : end);
}
const elements = { webBatchSummary: {}, huaweiNewIPs: { value: '192.0.2.1\n192.0.2.2' }, huaweiUpdateMode: { value: 'append' }, huaweiConfirm: {} };
const s = vm.createContext({
  currentMode: 'official', isTaskRunning: false, activeDetailCenters: null,
  selectedWebDCs: new Set(['HKG']), dcList: [{ DataCenter: 'HKG' }, { DataCenter: 'NRT' }],
  scanData: [{ IP: '192.0.2.1', Port: 443, DataCenter: 'HKG', TCPDuration: 1000000 }, { IP: '192.0.2.2', Port: 443, DataCenter: 'NRT', TCPDuration: 2000000 }],
  testData: [{ IP: '192.0.2.1', Port: 443, DataCenter: 'HKG', AvgLatency: 3000000, Speed: '4MB/s' }],
  officialAutoSpeedPending: false, officialAutoStage: '', manualDetailRunning: false,
  retainedSort: { official: [], nsb: [] },
  document: { getElementById: id => elements[id] }, showNoticeCard() {}, clearHuaweiPreview() {},
  renderAllTestResults() {}, switchTab() {}, scheduleRetainedView() {},
  wsSend() { throw Error('adding scope must not start network tasks'); },
});
for (const name of ['getDetailScopeRows', 'startWebBatch', 'getHuaweiEditedIPs']) vm.runInContext(extract(name), s);
vm.runInContext('startWebBatch()', s);
assert.deepEqual([...s.activeDetailCenters], ['HKG']);
assert.equal(s.getDetailScopeRows()[0].Speed, '4MB/s');
vm.runInContext(`selectedWebDCs=new Set(['NRT']);startWebBatch()`, s);
assert.equal(s.getDetailScopeRows().length, 1);
assert.equal(s.getDetailScopeRows()[0].DataCenter, 'NRT');
assert.equal(s.getDetailScopeRows()[0].CandidateOnly, true);
assert.equal(s.testData[0].Speed, '4MB/s', 'removing scope cannot delete cached test');
vm.runInContext(`selectedWebDCs=new Set(['HKG']);startWebBatch()`, s);
assert.equal(s.getDetailScopeRows()[0].Speed, '4MB/s', 're-added center must restore previous test');
assert.deepEqual([...s.getHuaweiEditedIPs()], ['192.0.2.1', '192.0.2.2']);
elements.huaweiNewIPs.value = '192.0.2.8\n192.0.2.8\n\n192.0.2.9';
assert.deepEqual([...s.getHuaweiEditedIPs()], ['192.0.2.8', '192.0.2.9']);
assert.match(html, /id="huaweiNewIPs"[^>]*oninput="editHuaweiIPs\(\)"/);
assert.doesNotMatch(html, /id="huaweiNewIPs"[^>]*(readonly|disabled)/);
console.log('Manual scope replacement/cache restoration/no-network and editable DNS draft PASS');
