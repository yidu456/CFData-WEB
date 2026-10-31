// Real embedded functions with an offline request-correlated WebSocket transport.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const { webcrypto } = require('node:crypto');
const html = fs.readFileSync(`${__dirname}/index.html`, 'utf8');
function extract(name) {
  const match = new RegExp(`        (?:async )?function ${name}\\(`).exec(html);
  assert.ok(match, `missing ${name}`);
  const next = /\n        (?:async )?function /.exec(html.slice(match.index + 1));
  return html.slice(match.index, next ? match.index + 1 + next.index : html.indexOf('</script>', match.index));
}
function extractConst(name) {
  const start = html.indexOf(`        const ${name}`); assert.ok(start >= 0, `missing ${name}`);
  return html.slice(start, html.indexOf(';', start) + 1);
}
function fixture() {
  const requests = [], notices = [], legacy = new Map();
  let saved = {}, revision = 0, fail = '', hold = false, dropped = [], privateFileStatus = '', form = { official: { threads: '20' }, nsb: {} };
  const elements = { webStateStatus: {}, configAutoLoad: {}, webStateCleanup: {} };
  const state = vm.createContext({
    webStateMemory: {}, webStateRevision: 0, webStateLoaded: false, webStateDirty: new Set(), webStatePending: new Map(), webStateSequence: 0,
    webStateQueue: Promise.resolve(), webStateMigrationReceipt: null, pendingSavedWebDCs: new Set(), webStateMetadata: {},
    webStateGenerations: {}, serverConfigLoadIntents: [], webBatchRunning: false, isStoppingTask: false,
    document: { getElementById: id => elements[id] }, selectedWebDCs: new Set(), dcList: [], isTaskRunning: false,
    setTimeout, clearTimeout, crypto: webcrypto, TextEncoder, JSON, Set, Map,
    githubSessionTokens: new Map(), edgetunnelSessionPasswords: new Map(),
    applyTheme(t) { state.appliedTheme = t; }, applyConfigValues(c) { state.appliedConfig = c; form = JSON.parse(JSON.stringify(c)); },
    mapCLIConfigToForm: c => c, readFormValues: () => JSON.parse(JSON.stringify(form)), renderGitHubAccounts() {}, renderEdgetunnelAccounts() {},
    resetLoadedPublishOptions() { state.resetCount = (state.resetCount || 0) + 1; },
    handleHuaweiMessage() { return false; },
    showNoticeCard: m => notices.push(m), log() {}, recordWebDebugError() {},
    localStorage: { getItem: k => legacy.get(k) ?? null, setItem() { throw Error('new browser persistence'); }, removeItem: k => legacy.delete(k) },
    githubAccountStorageKey: 'cfdata-github-accounts', edgetunnelAccountStorageKey: 'cfdata-edgetunnel-accounts',
    wsSend(request) {
      requests.push(request);
      if (!request.type.startsWith('web_state_')) return true;
      if (hold) { dropped.push(request); return true; }
      const { requestId } = request.data;
      let response = { action: request.type, requestId, ok: !fail, code: fail };
      if (!fail) {
        if (request.type === 'web_state_get') response = { ...response, metadata: { revision, privateFileStatus: privateFileStatus || (revision ? 'encrypted' : 'absent'), encryptedSaveSupported: true }, sections: saved };
        if (request.type === 'web_state_patch' || request.type === 'web_state_migrate') {
          if (request.data.revision !== revision || (request.type === 'web_state_migrate' && revision)) response = { ...response, ok: false, code: 'stale' };
          else { saved = { ...saved, ...JSON.parse(JSON.stringify(request.data.sections)) }; response.revision = ++revision; response.migrationToken = 'mock-receipt'; }
        }
        if (request.type === 'web_state_legacy_cleanup_ready') response.cleanupReady = true;
      }
      setImmediate(() => state.handleWebStateMessage({ type: 'web_state_response', data: response }));
      return true;
    },
  });
  vm.runInContext(extractConst('githubTokenCryptoSalt'), state);
  for (const name of ['webStateError','setWebStateStatus','webStateRequest','handleWebStateMessage','saveWebStateSections','loadWebState','loadConfigFromLS','loadConfigFromServer','handleMessage','autoLoadConfig','getGitHubAccounts','saveGitHubAccounts','normalizeGitHubAccountKey','migrateLegacyGitHubToken','getGitHubTokenCryptoKey','encryptGitHubToken','getEdgetunnelAccounts','saveEdgetunnelAccounts','normalizeEdgetunnelAccountKey','deleteSelectedEdgetunnelAccount','legacyWebStateSnapshot','legacyWebStateDigest','sanitizeLegacyWebConfig','importLegacyBrowserState','cleanupLegacyBrowserState','sha256Hex','bytesToBase64']) vm.runInContext(extract(name), state);
  return { state, requests, notices, legacy, dropped, form: value => { form = JSON.parse(JSON.stringify(value)); }, currentForm: () => form, saved: value => { saved = value; }, fail: v => { fail = v; }, hold: v => { hold = v; }, privateStatus: value => { privateFileStatus = value; } };
}
(async () => {
  const f = fixture();
  await f.state.loadWebState();
  await f.state.saveWebStateSections({ theme: 'dark' });
  await f.state.saveGitHubAccounts([{ repo: 'owner/repo', token: 'FIXTURE_PRIVATE_TOKEN' }]);
  assert.equal(f.state.webStateMemory.theme, 'dark');
  assert.match(f.state.document.getElementById('webStateStatus').textContent, /应用加密/);
  assert.match(f.state.document.getElementById('webStateStatus').textContent, /支持平台/);
  assert.match(f.state.webStateError('unavailable').message, /当前后端/);
  assert.doesNotMatch(f.state.webStateError('unavailable').message, /仅 Windows/);
  const copy = f.state.getGitHubAccounts(); copy[0].token = 'changed';
  assert.equal(f.state.getGitHubAccounts()[0].token, 'FIXTURE_PRIVATE_TOKEN', 'caller mutated saved accounts before acknowledgment');
  f.hold(true);
  const oldLoad = f.state.loadWebState();
  const oldRequest = f.requests.at(-1);
  f.state.handleWebStateMessage({ type: 'web_state_response', data: { action: oldRequest.type, requestId: oldRequest.data.requestId, ok: true, metadata: { revision: 0 }, sections: { theme: 'light' } } });
  await oldLoad;
  assert.equal(f.state.webStateMemory.theme, 'dark', 'late stale load reverted acknowledged state');
  f.hold(false);
  const concurrent = fixture();
  await concurrent.state.loadWebState();
  const first = concurrent.state.saveGitHubAccounts([{ repo: 'owner/first' }]);
  const second = concurrent.state.saveGitHubAccounts([{ repo: 'owner/second' }]);
  await first;
  await assert.rejects(second, /版本|changed/);
  assert.equal(concurrent.state.getGitHubAccounts()[0].repo, 'owner/first', 'queued stale account copy erased acknowledged account');
  f.fail('save_failed');
  await assert.rejects(f.state.saveGitHubAccounts([]), /encrypted|保存/);
  assert.equal(f.state.getGitHubAccounts().length, 1, 'failed save replaced good memory');
  const d = fixture();
  d.saved({ edgetunnelAccounts: [{ name: 'old', host: 'https://old.example', mode: 'overwrite' }] });
  await d.state.loadWebState();
  d.state.document.getElementById = id => id === 'edgetunnelAccountSelect' ? { value: '0' } : {};
  d.state.edgetunnelSessionPasswords.set('old', 'OLD_PASSWORD');
  d.hold(true);
  const deleting = d.state.deleteSelectedEdgetunnelAccount();
  await new Promise(resolve => setImmediate(resolve));
  d.state.handleWebStateMessage({ type: 'web_state_response', data: { action: 'web_state_patch', requestId: d.dropped[0].data.requestId, ok: false, code: 'stale' } });
  await deleting;
  assert.equal(d.state.getEdgetunnelAccounts()[0].name, 'old', 'failed delete removed the existing account from memory');
  assert.equal(d.state.edgetunnelSessionPasswords.get('old'), 'OLD_PASSWORD');
  const g = fixture();
  g.saved({ githubAccounts: [{ name: 'old', repo: 'owner/repo', token: 'LEGACY_PRIVATE_TOKEN' }] });
  await g.state.loadWebState();
  g.fail('save_failed');
  await assert.rejects(g.state.migrateLegacyGitHubToken(g.state.getGitHubAccounts()[0], 0));
  assert.equal(g.state.getGitHubAccounts()[0].token, 'LEGACY_PRIVATE_TOKEN', 'failed token migration lost account content');
  assert.equal(g.state.githubSessionTokens.has('old'), false, 'failed token migration granted session secret state');
  const delayed = fixture();
  delayed.saved({ config: { official: { threads: '20' }, nsb: {} } });
  await delayed.state.loadWebState();
  delayed.hold(true);
  const explicitLoad = delayed.state.loadConfigFromLS();
  await new Promise(resolve => setImmediate(resolve));
  const staleGet = delayed.dropped.shift();
  delayed.form({ official: { threads: '77' }, nsb: {} });
  delayed.state.webStateDirty.add('config'); delayed.state.webStateGenerations.config = (delayed.state.webStateGenerations.config || 0) + 1;
  const savedNewer = delayed.state.saveWebStateSections({ config: delayed.state.readFormValues() });
  await new Promise(resolve => setImmediate(resolve));
  const saveRequest = delayed.dropped.shift();
  delayed.state.handleWebStateMessage({ type: 'web_state_response', data: { action: saveRequest.type, requestId: saveRequest.data.requestId, ok: true, revision: 2 } });
  await savedNewer;
  delayed.state.handleWebStateMessage({ type: 'web_state_response', data: { action: staleGet.type, requestId: staleGet.data.requestId, ok: true, metadata: { revision: 1, privateFileStatus: 'encrypted' }, sections: { config: { official: { threads: '20' }, nsb: {} } } } });
  await explicitLoad;
  assert.equal(delayed.currentForm().official.threads, '77', 'late explicit saved load overwrote newer edit/save');
  assert.equal(delayed.state.webStateRevision, 2, 'late explicit saved load moved revision backwards');
  const server = fixture();
  server.state.webStateDirty.add('config'); server.state.webStateGenerations.config = 1;
  server.form({ official: { threads: '77' }, nsb: {} });
  server.state.loadConfigFromServer();
  server.state.handleMessage({ type: 'config_data', data: { official: { threads: '20' }, nsb: {} } });
  assert.equal(server.currentForm().official.threads, '20', 'explicit server sync did not replace pre-request dirty input');
  server.form({ official: { threads: '77' }, nsb: {} }); server.state.webStateDirty.add('config'); server.state.webStateGenerations.config++;
  server.state.loadConfigFromServer();
  server.form({ official: { threads: '88' }, nsb: {} }); server.state.webStateGenerations.config++;
  server.state.handleMessage({ type: 'config_data', data: { official: { threads: '20' }, nsb: {} } });
  assert.equal(server.currentForm().official.threads, '88', 'late explicit server sync overwrote post-request edit');
  const ordered = fixture();
  ordered.state.webStateMemory.autoload = 'configFile'; ordered.state.autoLoadConfig();
  ordered.form({ official: { threads: '77' }, nsb: {} }); ordered.state.webStateDirty.add('config'); ordered.state.webStateGenerations.config = 1;
  ordered.state.loadConfigFromServer();
  ordered.state.handleMessage({ type: 'config_data', data: { official: { threads: '10' }, nsb: {} } });
  assert.equal(ordered.currentForm().official.threads, '77', 'older automatic server response consumed explicit intent');
  ordered.state.handleMessage({ type: 'config_data', data: { official: { threads: '20' }, nsb: {} } });
  assert.equal(ordered.currentForm().official.threads, '20', 'ordered explicit server response was not applied');
  const failed = fixture();
  failed.state.webStateMemory.autoload = 'configFile'; failed.state.autoLoadConfig();
  try { failed.state.handleMessage({ type: 'error', data: '读取配置文件失败: offline fixture' }); } catch (_) {}
  assert.equal(failed.state.serverConfigLoadIntents.length, 0, 'failed server config read stranded response intent');
  f.fail('');
  f.state.webStateDirty.add('config'); f.state.webStateDirty.add('theme');
  f.saved({ theme: 'light', config: { official: { threads: '9' } }, autoload: 'backend' });
  await f.state.loadWebState();
  assert.equal(f.state.webStateMemory.theme, 'dark', 'reconnect erased unsaved theme');
  assert.equal(f.state.appliedConfig, undefined, 'reconnect erased dirty fields');
  assert.ok(f.state.resetCount > 0);
  const m = fixture();
  m.legacy.set('cfdata-config', JSON.stringify({ official: { threads: '30', autoPublish: true }, selectedDatacenters: ['NRT'], taskSnapshot: { complete: true } }));
  m.legacy.set('cfdata-autoload', 'localStorage'); m.legacy.set('cfdata-theme', 'dark'); m.legacy.set('unrelated', 'keep');
  await m.state.loadWebState();
  assert.equal(m.state.webStateMemory.theme, undefined, 'normal load silently imported browser storage');
  await m.state.importLegacyBrowserState();
  assert.equal(m.state.webStateMemory.autoload, 'backend');
  assert.equal(m.state.webStateMemory.config.official.autoPublish, undefined);
  assert.equal(m.state.webStateMemory.config.taskSnapshot, undefined);
  assert.ok(m.legacy.has('cfdata-config'), 'import deleted source without explicit cleanup');
  m.legacy.set('cfdata-theme', 'light');
  await m.state.cleanupLegacyBrowserState();
  assert.ok(m.legacy.has('cfdata-config'), 'changed source was cleaned');
  m.legacy.set('cfdata-theme', 'dark');
  await m.state.cleanupLegacyBrowserState();
  assert.equal(m.legacy.has('cfdata-config'), false); assert.equal(m.legacy.get('unrelated'), 'keep');
  assert.ok(m.requests.every(r => r.type.startsWith('web_state_')), 'persistence granted publishing authority');
  const oldDPAPI = fixture();
  oldDPAPI.privateStatus('legacy-dpapi');
  oldDPAPI.saved({ theme: 'light', config: { official: { threads: '20' }, nsb: {} } });
  await oldDPAPI.state.loadWebState();
  assert.match(oldDPAPI.state.document.getElementById('webStateStatus').textContent, /旧版 Windows 当前用户 DPAPI/);
  assert.match(oldDPAPI.state.document.getElementById('webStateStatus').textContent, /明确保存.*转换/);
  assert.match(oldDPAPI.state.document.getElementById('webStateStatus').textContent, /复制/);
  await oldDPAPI.state.loadConfigFromLS();
  assert.match(oldDPAPI.state.document.getElementById('webStateStatus').textContent, /旧版 Windows 当前用户 DPAPI/);
  assert.match(oldDPAPI.state.document.getElementById('webStateStatus').textContent, /明确保存.*转换/);
  await oldDPAPI.state.saveWebStateSections({ config: { official: { threads: '30' }, nsb: {} } });
  oldDPAPI.privateStatus('encrypted');
  await oldDPAPI.state.loadConfigFromLS();
  assert.match(oldDPAPI.state.document.getElementById('webStateStatus').textContent, /应用加密/);
  assert.doesNotMatch(oldDPAPI.state.document.getElementById('webStateStatus').textContent, /DPAPI/);
  assert.equal(oldDPAPI.state.webStateMetadata.privateFileStatus, 'encrypted');
  console.log('web_state_ui_test: passed');
})().catch(e => { console.error(e); process.exitCode = 1; });
