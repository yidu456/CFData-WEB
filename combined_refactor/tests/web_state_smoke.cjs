// Installed Chrome, embedded HTML and offline WebSocket responses; no application/provider server.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');
const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
const origin = 'http://127.0.0.1:13335/';
const fixtureToken = 'OFFLINE_PRIVATE_TOKEN';

(async () => {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROME_PATH });
  try {
    const context = await browser.newContext({ acceptDownloads: true, serviceWorkers: 'block' });
    context.setDefaultTimeout(5000);
    await context.route('**/*', route => route.request().url() === origin || route.request().url() === 'http://localhost:13335/'
      ? route.fulfill({ contentType: 'text/html; charset=utf-8', body: html }) : route.abort());
    await context.addInitScript(({ fixtureToken }) => {
      const source = {
        'cfdata-theme': 'dark', 'cfdata-autoload': 'localStorage',
        'cfdata-config': JSON.stringify({ official: { threads: '23', autoPublish: true }, nsb: {}, selectedDatacenters: ['NRT', 'GRU'], completedBatch: { id: 7 } }),
        'cfdata-github-accounts': JSON.stringify([{ name: 'legacy', repo: 'owner/repo', token: fixtureToken }]),
        'cfdata-edgetunnel-accounts': JSON.stringify([{ name: 'edge', host: 'example.invalid', mode: 'overwrite', password: 'OFFLINE_PRIVATE_PASSWORD' }]),
        unrelated: 'keep',
      };
      const originalGet = Storage.prototype.getItem, originalSet = Storage.prototype.setItem;
      if (!sessionStorage.getItem('seeded')) {
        for (const [key, value] of Object.entries(source)) originalSet.call(localStorage, key, value);
        sessionStorage.setItem('seeded', 'yes');
      }
      window.storageReads = []; window.storageWrites = [];
      Storage.prototype.getItem = function(key) { if (this === localStorage) storageReads.push(key); return originalGet.call(this, key); };
      Storage.prototype.setItem = function(key, value) { if (this === localStorage) storageWrites.push(key); return originalSet.call(this, key, value); };
      window.fixtureRead = key => originalGet.call(localStorage, key);
      window.fixtureWrite = (key, value) => originalSet.call(localStorage, key, value);
    }, { fixtureToken });
    let sections = {}, revision = 0, failure = '', supported = true, holdSave = false, holdGet = false, held, heldGet, lastSocket;
    const requests = [], errors = [], consoles = [];
    await context.routeWebSocket('**/*', socket => {
      lastSocket = socket;
      const send = data => socket.send(JSON.stringify({ type: 'web_state_response', data }));
      socket.send(JSON.stringify({ type: 'init_config', data: { speedTestURL: 'auto', skipGeoCheck: true, geoCheckOK: true } }));
      socket.onMessage(raw => {
        const request = JSON.parse(String(raw)); requests.push(request);
        const { requestId } = request.data || {};
        if (!request.type.startsWith('web_state_')) return;
        const respond = () => {
          let data = { action: request.type, requestId, ok: !failure, code: failure };
          if (!failure) {
            if (request.type === 'web_state_get') data = { ...data, metadata: { revision, encryptedSaveSupported: supported, privateFileStatus: revision ? 'encrypted' : 'absent', privatePath: 'C:/Offline EXE/web-state.private' }, sections };
            else if (request.type === 'web_state_patch' || request.type === 'web_state_migrate') {
              if (!supported) data = { ...data, ok: false, code: 'unavailable' };
              else if (request.data.revision !== revision) data = { ...data, ok: false, code: 'stale' };
              else if (request.type === 'web_state_migrate' && revision) data = { ...data, ok: false, code: 'exists' };
              else { sections = { ...sections, ...JSON.parse(JSON.stringify(request.data.sections)) }; data.revision = ++revision; data.migrationToken = 'offline-receipt'; }
            } else data.cleanupReady = request.data.revision === revision;
          }
          send(data);
        };
        if (holdGet && request.type === 'web_state_get') heldGet = respond;
        else if (holdSave && request.type === 'web_state_patch') held = respond;
        else respond();
      });
    });
    const page = await context.newPage();
    page.on('pageerror', error => errors.push(error.message));
    page.on('console', msg => consoles.push(msg.text()));
    await page.goto(origin, { waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => webStateLoaded);
    assert.equal(await page.evaluate(() => storageReads.length), 0, 'normal startup read old browser persistence');
    assert.equal(await page.evaluate(() => getGitHubAccounts().length), 0, 'automatic browser account import');
    // Failed import preserves source and gives no cleanup authority.
    failure = 'save_failed';
    await page.evaluate(() => importLegacyBrowserState());
    assert.equal(revision, 0);
    assert.ok(await page.evaluate(() => fixtureRead('cfdata-config')));
    assert.equal(await page.locator('#webStateCleanup').isDisabled(), true);
    failure = '';
    await page.evaluate(() => importLegacyBrowserState());
    assert.equal(revision, 1);
    assert.equal(sections.config.official.autoPublish, undefined);
    assert.equal(sections.config.completedBatch, undefined);
    assert.equal(await page.locator('#officialThreads').inputValue(), '23');
    assert.deepEqual(await page.evaluate(() => [...selectedWebDCs]), [], 'undiscovered selection became active');
    await page.evaluate(() => showDCSelection([{ DataCenter: 'NRT', City: 'Tokyo', MinLatency: 1000000 }, { DataCenter: 'GRU', City: 'Sao Paulo', MinLatency: 1000000 }]));
    assert.deepEqual((await page.evaluate(() => [...selectedWebDCs])).sort(), ['GRU', 'NRT']);
    assert.equal(await page.locator('#huaweiAuto').isChecked(), false);
    assert.equal(await page.locator('#githubAutoSelected').isChecked(), false);
    await page.evaluate(() => fixtureWrite('cfdata-theme', 'light'));
    await page.evaluate(() => cleanupLegacyBrowserState());
    assert.ok(await page.evaluate(() => fixtureRead('cfdata-config')), 'changed source cleaned');
    await page.evaluate(() => fixtureWrite('cfdata-theme', 'dark'));
    await page.evaluate(() => cleanupLegacyBrowserState());
    assert.equal(await page.evaluate(() => fixtureRead('cfdata-config')), null);
    assert.equal(await page.evaluate(() => fixtureRead('unrelated')), 'keep');
    await page.evaluate(() => closeNoticeCard());
    // Real original account crypto/unlock, save/delete, failed writes and memory-only use.
    await page.evaluate(async () => {
      document.getElementById('githubAccountName').value = 'crypto';
      document.getElementById('githubSaveToken').checked = true;
      await saveCurrentGitHubAccount({ owner: 'owner', repo: 'crypto', token: 'OFFLINE_PRIVATE_TOKEN', branch: 'main', path: 'results/ip.txt', message: 'offline fixture' });
      document.getElementById('edgetunnelAccountName').value = 'crypto-edge';
      document.getElementById('edgetunnelHost').value = 'https://example.invalid';
      document.getElementById('edgetunnelPassword').value = 'OFFLINE_PRIVATE_PASSWORD';
      document.getElementById('edgetunnelSavePassword').checked = true;
      await saveCurrentEdgetunnelAccount();
    });
    assert.match(sections.githubAccounts[1].tokenCipher, /^[^.]+\.[^.]+$/);
    assert.match(sections.edgetunnelAccounts[1].passwordCipher, /^[^:]+:[^:]+$/);
    const cryptoCipher = sections.githubAccounts[1].tokenCipher;
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => webStateLoaded);
    assert.equal(await page.evaluate(() => storageReads.length), 0, 'reload normally reads localStorage');
    await page.evaluate(() => { document.getElementById('githubAccountSelect').value = '1'; loadSelectedGitHubAccount(); });
    await page.waitForFunction(() => document.getElementById('githubToken').value === 'OFFLINE_PRIVATE_TOKEN');
    await page.evaluate(() => { document.getElementById('edgetunnelAccountSelect').value = '1'; loadSelectedEdgetunnelAccount(); });
    await page.waitForFunction(() => document.getElementById('edgetunnelPassword').value === 'OFFLINE_PRIVATE_PASSWORD');
    failure = 'save_failed';
    await page.evaluate(() => deleteSelectedGitHubAccount());
    assert.equal(await page.evaluate(() => getGitHubAccounts().length), 2);
    assert.equal(sections.githubAccounts.length, 2);
    failure = '';
    await page.evaluate(() => deleteSelectedGitHubAccount());
    assert.equal(sections.githubAccounts.length, 1);
    await page.evaluate(() => deleteSelectedEdgetunnelAccount());
    assert.equal(sections.edgetunnelAccounts.length, 1);
    // Theme/autoload persist without browser storage; other sections survive.
    await page.evaluate(async () => { await setTheme('light'); await saveAutoLoadPreference('backend'); });
    assert.equal(sections.theme, 'light'); assert.equal(sections.githubAccounts.length, 1);
    holdSave = true;
    await page.evaluate(() => { window.pendingTheme = setTheme('dark'); });
    await page.waitForTimeout(30);
    await page.evaluate(() => { webStateMemory.theme = 'auto'; webStateDirty.add('theme'); webStateGenerations.theme++; applyTheme('auto'); });
    holdSave = false; held(); held = null;
    await page.evaluate(() => window.pendingTheme);
    assert.equal(await page.evaluate(() => webStateMemory.theme), 'auto', 'late save acknowledgment erased newer edit');
    // Explicit saved load may replace pre-request dirty input, but never newer edits/saves.
    await page.locator('#officialThreads').fill('31');
    holdGet = true;
    await page.evaluate(() => { window.pendingConfigLoad = loadConfigFromLS(); });
    await page.waitForTimeout(30);
    await page.locator('#officialThreads').fill('77');
    holdGet = false; heldGet(); heldGet = null;
    await page.evaluate(() => window.pendingConfigLoad);
    assert.equal(await page.locator('#officialThreads').inputValue(), '77', 'late explicit saved load overwrote post-request edit');
    await page.evaluate(() => saveConfigToLS());
    const savedRevision = revision;
    holdGet = true;
    await page.evaluate(() => { window.pendingConfigLoad = loadConfigFromLS(); });
    await page.waitForTimeout(30);
    await page.locator('#officialThreads').fill('88');
    await page.evaluate(() => saveConfigToLS());
    holdGet = false; heldGet(); heldGet = null;
    await page.evaluate(() => window.pendingConfigLoad);
    assert.equal(await page.locator('#officialThreads').inputValue(), '88', 'late explicit saved load overwrote acknowledged save');
    assert.ok(revision > savedRevision); assert.equal(await page.evaluate(() => webStateRevision), revision);
    // Dirty form survives actual socket reconnection and server defaults.
    await page.locator('#officialThreads').fill('77');
    await page.evaluate(() => { document.getElementById('huaweiAuto').checked = true; document.getElementById('githubAutoSelected').checked = true; });
    const readsBefore = requests.filter(r => r.type === 'web_state_get').length;
    lastSocket.close();
    await page.waitForFunction(() => ws.readyState === WebSocket.OPEN && webStatePending.size === 0);
    await page.waitForTimeout(3200);
    assert.ok(requests.filter(r => r.type === 'web_state_get').length > readsBefore);
    assert.equal(await page.locator('#officialThreads').inputValue(), '77');
    assert.equal(await page.locator('#huaweiAuto').isChecked(), false);
    assert.equal(await page.locator('#githubAutoSelected').isChecked(), false);
    supported = false;
    const oldRevision = revision;
    await page.evaluate(() => saveConfigToLS());
    assert.equal(revision, oldRevision); assert.equal(await page.locator('#officialThreads').inputValue(), '77');
    assert.match(await page.locator('#webStateStatus').textContent(), /不可用/);
    failure = 'unauthorized';
    await page.evaluate(async () => { try { await loadWebState(); } catch (_) {} });
    assert.equal(await page.locator('#officialThreads').inputValue(), '77');
    failure = ''; supported = true;
    await page.evaluate(() => { document.getElementById('nsbSourceUrl').value = ''; });
    await page.evaluate(() => saveConfigToLS());
    await page.evaluate(() => { document.getElementById('nsbSourceUrl').value = 'https://example.invalid/unsaved'; markWebConfigDirty(); });
    await page.evaluate(() => loadConfigFromLS());
    assert.equal(await page.locator('#nsbSourceUrl').inputValue(), '', 'explicit saved load did not restore empty form control');
    // Existing Blob download bytes, filename and browser event are untouched.
    const downloadPromise = page.waitForEvent('download');
    await page.evaluate(() => downloadFile('198.51.100.1:443#NRT\n', 'offline-state', 'txt'));
    const download = await downloadPromise;
    assert.match(download.suggestedFilename(), /^offline-state_\d+\.txt$/);
    assert.equal(fs.readFileSync(await download.path(), 'utf8'), '\ufeff198.51.100.1:443#NRT\n');
    // Same DPAPI account blob at another origin does not unlock origin-bound inner cipher.
    const other = await context.newPage();
    await other.goto('http://localhost:13335/', { waitUntil: 'domcontentloaded' });
    await other.waitForFunction(() => webStateLoaded);
    assert.equal(await other.evaluate(cipher => decryptStoredGitHubToken({ name: 'crypto', repo: 'owner/crypto', tokenCipher: cipher }), cryptoCipher), '');
    assert.deepEqual(await page.evaluate(() => storageWrites), [], 'normal Web state wrote new persistent browser data');
    assert.ok(!requests.some(r => ['github_upload','edgetunnel_upload','web_github_publish','web_publish_options','huawei_update','start_web_batch','start_task'].includes(r.type)), 'persistence/loading granted cloud-write/scan authority');
    assert.deepEqual(errors, []);
    assert.ok(!consoles.join('\n').includes(fixtureToken), 'secret in browser console');
    console.log('Offline Chrome Web state passed: explicit migration/failure/unchanged cleanup, preferences/config/account crypto save/delete/reload, dirty reconnect/delayed save, unsupported/authorization failure, discovered-only DC membership, origin-bound unlock, no cloud/scans/new browser persistence, unchanged Blob download.');
    await context.close();
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
