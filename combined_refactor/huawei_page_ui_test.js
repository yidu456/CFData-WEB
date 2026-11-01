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

assert.equal((html.match(/id="resultHuaweiChoice"/g) || []).length, 1, 'Huawei must have one chooser entry');
assert.doesNotMatch(html, /onclick="openHuaweiPanel\(\)"/, 'settings menu Huawei shortcut must be removed');
assert.doesNotMatch(html, />多中心结果发布<\/div>/, 'orphan settings heading must be removed');
for (const section of ['huaweiConnectionSection', 'huaweiTargetSection', 'huaweiUpdateSection']) {
  assert.equal((html.match(new RegExp(`id="${section}"`, 'g')) || []).length, 1, `${section} must exist once`);
}
for (const field of ['huaweiAK', 'huaweiSK', 'huaweiRegion', 'huaweiProjectID']) {
  assert.equal((html.match(new RegExp(`id="${field}"`, 'g')) || []).length, 1, `${field} must not be duplicated`);
}
for (const url of [
  'https://support.huaweicloud.com/usermanual-ca/ca_05_0006.html',
  'https://developer.huaweicloud.com/endpoint',
  'https://console.huaweicloud.com/dns/?locale=zh-cn',
]) {
  assert.match(html, new RegExp(`href="${url.replace(/[?./]/g, '\\$&')}"[^>]*target="_blank"[^>]*rel="noopener noreferrer"`));
}
assert.match(html, /<details[^>]*id="huaweiHelp"/);
assert.match(html, /id="huaweiLegacyMigrate"/);
assert.match(html, /id="huaweiLegacyCleanup"/);
assert.match(html, /AES-256-GCM/);
assert.match(html, /随机 nonce/);
assert.match(html, /固定程序密钥/);
assert.match(html, /不能防止逆向/);
assert.match(html, /DPAPI[^<]*保存加密配置[^<]*转换[^<]*复制/);

const ids = ['huaweiAK', 'huaweiSK', 'huaweiRegion', 'huaweiProjectID', 'huaweiSaveEncrypted', 'huaweiSaveStatus', 'huaweiConfigStatus', 'huaweiDiscoveryStatus', 'huaweiDNSStatus', 'huaweiZone', 'huaweiRecord', 'huaweiTarget', 'huaweiPreview', 'huaweiConfirm', 'huaweiAuto', 'huaweiLegacyMigrate', 'huaweiLegacyCleanup', 'huaweiLegacyStatus', 'huaweiPrivatePath'];
const elements = Object.fromEntries(ids.map(id => [id, { value: '', checked: false, disabled: false, textContent: '', innerHTML: '', className: '' }]));
elements.huaweiUpdateMode={value:'overwrite'};elements.huaweiNewIPs={value:''};
const sent = [];
const notices = [];
const confirmations = [];
let selectedIPs = ['192.0.2.9', '192.0.2.3'];
const ctx = vm.createContext({
  document: { getElementById: id => elements[id] },
  wsSend: payload => { sent.push(payload); return true; },
  showNoticeCard: text => notices.push(text),
  confirm: text => { confirmations.push(text); return true; },
  escapeHTML: String,
  huaweiDraftEdited:false,huaweiTargetRecords:[],getHuaweiEditedIPs:()=>elements.huaweiNewIPs.value.split(/\s+/).filter(Boolean),reloadHuaweiDraft(){},completedWebBatch: null, isTaskRunning: false, getSelectedHuaweiIPs: () => selectedIPs,
  huaweiPreviewToken: '', huaweiPreviewBatchID: 0, huaweiPreviewSelection: '', huaweiPreviewRequestID: '', huaweiPreviewSequence: 0,
  huaweiUpdatePending: false, resultHuaweiLoading: false, resultHuaweiLoaded: false,
});
for (const name of ['clearHuaweiPreview', 'handleHuaweiDisconnectFeedback', 'discoverHuaweiZones', 'discoverHuaweiRecords', 'selectHuaweiRecord', 'previewHuaweiDNS', 'confirmHuaweiDNS', 'manageHuaweiLegacy', 'handleHuaweiMessage']) vm.runInContext(extract(name), ctx);

ctx.wsSend = () => false;
vm.runInContext('discoverHuaweiZones(true)', ctx);
assert.match(elements.huaweiDiscoveryStatus.textContent, /未发送/);
vm.runInContext('discoverHuaweiRecords()', ctx);
assert.match(elements.huaweiDiscoveryStatus.textContent, /未发送/);
vm.runInContext('previewHuaweiDNS()', ctx);
assert.match(elements.huaweiDNSStatus.textContent, /未发送/);
ctx.huaweiPreviewToken = 'token'; ctx.huaweiPreviewBatchID = 0;ctx.huaweiPreviewSelection=JSON.stringify({mode:'overwrite',ips:[]});
vm.runInContext('confirmHuaweiDNS()', ctx);
assert.match(elements.huaweiDNSStatus.textContent, /未发送/);
assert.equal(ctx.huaweiPreviewToken, 'token', 'failed update send must retain preview authority');
assert.match(elements.huaweiDNSStatus.textContent, /重新预览当前结果/);
vm.runInContext('clearHuaweiPreview()', ctx);
assert.equal(ctx.huaweiPreviewToken, '', 'disconnect lifecycle must invalidate failed-send preview authority');
assert.equal(ctx.huaweiPreviewRequestID, '', 'disconnect lifecycle must reject outstanding preview replies');

ctx.wsSend = payload => { sent.push(payload); return true; };
vm.runInContext('previewHuaweiDNS()', ctx);
assert.equal(sent.at(-1).data.batchId, 0, 'manual preview must not require a completed measurement batch');
assert.deepEqual(sent.at(-1).data.ips, ['192.0.2.9', '192.0.2.3'], 'manual preview must use the selected range in its current order');
const firstRequestID = sent.at(-1).data.requestId;
selectedIPs = ['192.0.2.10'];
vm.runInContext('clearHuaweiPreview(); previewHuaweiDNS()', ctx);
const currentRequestID = sent.at(-1).data.requestId;
assert.notEqual(currentRequestID, firstRequestID, 'new selections need distinct preview correlation');
function previewReply(requestId, token, changed = true, ips = ['192.0.2.10']) {
  ctx.previewReply = { type: 'huawei_response', data: { action: 'huawei_preview', data: { requestId, token, batchId: 0, target: { name: 'edge.example.com.', type: 'A', line: 'default_view', ttl: 300 }, old: ['192.0.2.1'], new: ips, changed } } };
  vm.runInContext('handleHuaweiMessage(previewReply)', ctx);
}
previewReply(firstRequestID, 'stale-token');
assert.equal(ctx.huaweiPreviewToken, '', 'out-of-order reply must not install a token for an older range');
assert.equal(elements.huaweiConfirm.disabled, true);
previewReply(currentRequestID, 'sent-token');
assert.equal(ctx.huaweiPreviewToken, 'sent-token');
assert.equal(elements.huaweiConfirm.disabled, false);
const currentPreview = elements.huaweiPreview.textContent;
previewReply(firstRequestID, 'stale-again');
assert.equal(ctx.huaweiPreviewToken, 'sent-token', 'stale reply must not replace current preview authority');
assert.equal(elements.huaweiPreview.textContent, currentPreview);
vm.runInContext('confirmHuaweiDNS()', ctx);
assert.equal(ctx.huaweiUpdatePending, true);
assert.equal(sent.at(-1).type, 'huawei_update');
assert.equal(sent.at(-1).data.token, 'sent-token');
assert.equal(sent.at(-1).data.batchId, 0);
vm.runInContext(`handleHuaweiMessage({type:'web_publish_status',data:{destination:'dns',batchId:0,status:'running',message:'正在更新'}})`, ctx);
assert.equal(ctx.huaweiUpdatePending, true, 'running status must keep the update pending until a terminal result');
vm.runInContext('handleHuaweiDisconnectFeedback(); clearHuaweiPreview()', ctx);
assert.equal(ctx.huaweiUpdatePending, false);
assert.match(elements.huaweiDNSStatus.textContent, /云端结果未知/);
assert.match(elements.huaweiDNSStatus.textContent, /先只读检查云端记录/);
previewReply(currentRequestID, 'post-disconnect-token');
assert.equal(ctx.huaweiPreviewToken, '', 'disconnected preview replies must not restore authority');
vm.runInContext('previewHuaweiDNS()', ctx);
const noFamilyRequestID = sent.at(-1).data.requestId;
const beforeNoFamily = sent.length;
previewReply(noFamilyRequestID, 'no-family-token', false, []);
assert.equal(elements.huaweiConfirm.disabled, true, 'no matching address family must not expose a mutation action');
assert.match(elements.huaweiPreview.textContent, /无匹配地址族，保持旧记录/);
assert.equal(sent.length, beforeNoFamily, 'no-family reply must not send a DNS mutation');
ctx.huaweiDraftEdited=false;vm.runInContext('previewHuaweiDNS()', ctx);
previewReply(sent.at(-1).data.requestId, 'selection-token');
const beforeChangedSelection = sent.length;
selectedIPs = ['192.0.2.11']; elements.huaweiNewIPs.value='192.0.2.11';
vm.runInContext('confirmHuaweiDNS()', ctx);
assert.equal(sent.length, beforeChangedSelection, 'changed row selection must not confirm the old preview');
assert.match(notices.at(-1), /IP 列表或模式已变化/);
selectedIPs = ['192.0.2.10'];
ctx.isTaskRunning = true;
vm.runInContext('confirmHuaweiDNS()', ctx);
assert.equal(sent.length, beforeChangedSelection, 'active task must not confirm an old preview');
ctx.isTaskRunning = false;
vm.runInContext('manageHuaweiLegacy("migrate")', ctx);
assert.equal(sent.at(-1).type, 'huawei_legacy_migrate');
assert.equal(sent.at(-1).data.confirmed, true);
vm.runInContext('manageHuaweiLegacy("cleanup")', ctx);
assert.equal(sent.at(-1).type, 'huawei_legacy_cleanup');
assert.match(confirmations.at(-1), /删除/);

vm.runInContext(`handleHuaweiMessage({type:'huawei_response',data:{action:'huawei_config_get',data:{maskedAK:'configured (ending 6789)',hasSecret:true,region:'cn-north-4',projectId:'0123456789abcdef0123456789abcdef',persisted:true,encryptedSaveSupported:true,privatePath:'C:/Program Files/CFData/huawei-dns.private',privateFileStatus:'encrypted',legacyPrivatePath:'C:/Users/me/AppData/Roaming/cfdata-web/huawei-dns.private',legacyFileStatus:'available',legacyMigrationSupported:true,legacyCleanupReady:false,regions:['cn-north-4']}}})`, ctx);
assert.match(elements.huaweiConfigStatus.textContent, /6789/);
assert.doesNotMatch(elements.huaweiConfigStatus.textContent, /C:\//);
assert.match(elements.huaweiLegacyStatus.textContent, /C:\//);
assert.equal(elements.huaweiLegacyMigrate.disabled, false);
assert.equal(elements.huaweiLegacyCleanup.disabled, true);
assert.match(elements.huaweiLegacyStatus.textContent, /迁移不会自动删除/);

vm.runInContext(`handleHuaweiMessage({type:'huawei_response',data:{action:'huawei_legacy_migrate',data:{hasSecret:true,region:'cn-north-4',projectId:'0123456789abcdef0123456789abcdef',persisted:true,encryptedSaveSupported:true,privatePath:'C:/Program Files/CFData/huawei-dns.private',privateFileStatus:'encrypted',legacyPrivatePath:'C:/Users/me/AppData/Roaming/cfdata-web/huawei-dns.private',legacyFileStatus:'available',legacyMigrationSupported:false,legacyCleanupReady:true,regions:['cn-north-4']}}})`, ctx);
assert.equal(elements.huaweiLegacyCleanup.disabled, false);
assert.match(elements.huaweiSaveStatus.textContent, /迁移完成/);

console.log('Huawei page structure/help/migration/status/disconnected-send assertions passed');
