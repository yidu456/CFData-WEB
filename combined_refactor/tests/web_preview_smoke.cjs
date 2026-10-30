// Run with installed Playwright via NODE_PATH; no server or live provider is used.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { chromium } = require('playwright');

const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
const output = process.env.QA_OUTPUT_DIR || path.join(os.tmpdir(), 'cfdata-web-preview-qa');
const origin = 'http://127.0.0.1:13335/';
const target = { id: 'record1', zoneId: 'zone1', name: 'edge.example.com.', type: 'A', line: 'default_view', ttl: 300, records: ['192.0.2.1'] };
const row = (n) => ({ IP: `198.51.100.${n}`, Port: 443, DataCenter: n % 2 ? 'FRA' : 'SJC', DCCountry: n % 2 ? 'DE' : 'US', City: n % 2 ? 'Frankfurt' : 'San Jose', AvgLatency: (100 + n) * 1000000, MinLatency: 90000000, MaxLatency: 150000000, LossRate: 0, Speed: `${21 - n}.00MB/s`, WebBatchDerived: true });

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const browser = await chromium.launch({ headless: true, ...(process.env.CHROME_PATH ? { executablePath: process.env.CHROME_PATH } : {}) });
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1050 }, serviceWorkers: 'block' });
    context.setDefaultTimeout(5000);
    const errors = [], requests = [];
    // Fulfill only the application HTML; every asset/provider request is blocked.
    await context.route('**/*', route => route.request().url() === origin
      ? route.fulfill({ contentType: 'text/html; charset=utf-8', body: html }) : route.abort());
    context.on('page', page => page.on('pageerror', error => errors.push(error.message)));
    const page = await context.newPage();
    let socket, connections = 0, batchID = 0, resultCount = 10, holdBatch = false, previewError = false, previewChanged = true;
    const send = (type, data) => socket.send(JSON.stringify({ type, data }));
    const response = (action, data) => send('huawei_response', { action, data });
    await page.routeWebSocket('**/*', routed => {
      socket = routed;
      connections++;
      send('init_config', { version: 'offline-web-preview', skipGeoCheck: true, geoCheckOK: true, speedTestURL: 'auto' });
      routed.onMessage(raw => {
        const request = JSON.parse(String(raw));
        requests.push(request);
        switch (request.type) {
          case 'check_proxy_country': send('proxy_country_result', { cfCountry: 'SKIPPED', proxyWarning: false, geoCheckOK: true, skipGeoCheck: true }); break;
          case 'web_publish_options': response(request.type, { dns: request.data.dns, github: request.data.github }); break;
          case 'start_web_batch': {
            batchID++;
            const policy = { Datacenters: request.data.datacenters, MaxLatencyMS: request.data.maxLatencyMS, MinSpeedMB: request.data.minSpeedMB, Limit: request.data.limit };
            send('web_batch_started', { id: batchID, candidateCount: 14, policy });
            send('web_batch_progress', { current: 14, total: 14, qualified: resultCount, phase: 'speed' });
            if (holdBatch) { send('web_batch_result', { result: row(1), qualified: true }); break; }
            send('web_batch_complete', { id: batchID, results: Array.from({ length: resultCount }, (_, i) => row(i + 1)), candidateCount: 14, measuredCount: 14, qualifiedCount: resultCount === 10 ? 14 : resultCount, missingDatacenters: resultCount === 2 ? ['SJC'] : [], policy });
            send('task_complete', null);
            break;
          }
          case 'stop_task': send('web_batch_failed', { status: 'cancelled', message: 'Mock cancellation' }); send('task_stopped', null); break;
          case 'huawei_config_get': case 'huawei_config_set': response(request.type, { maskedAK: 'configured (ending 6789)', hasSecret: true, region: 'cn-north-4', projectId: '0123456789abcdef0123456789abcdef', persisted: request.data.persist || false, regions: ['cn-north-4', 'cn-east-3'] }); break;
          case 'huawei_test': case 'huawei_zones': response(request.type, [{ id: 'zone1', name: 'example.com.' }]); break;
          case 'huawei_records': response(request.type, [target]); break;
          case 'huawei_select': response(request.type, target); break;
          case 'huawei_preview':
            if (previewError) send('huawei_error', { action: request.type, message: 'Preview stale; select and preview again' });
            else response(request.type, { token: 'mock-preview-token', batchId: request.data.batchId, target, old: ['192.0.2.1'], new: previewChanged ? ['198.51.100.1'] : [], changed: previewChanged });
            break;
          case 'huawei_update': send('web_publish_status', { destination: 'dns', batchId: request.data.batchId, status: 'updated', message: 'Mock DNS update' }); break;
          case 'web_github_publish': send('web_publish_status', { destination: 'github', batchId: request.data.batchId, status: 'uploaded', message: 'Mock GitHub upload' }); break;
        }
      });
    });
    await page.goto(origin, { waitUntil: 'domcontentloaded' });
    await page.waitForFunction(() => ws && ws.readyState === WebSocket.OPEN);
    const summary = page.locator('#webDCSelector summary');
    await summary.focus();
    await summary.press('Enter');
    assert.equal(await page.locator('#webDCSelector').evaluate(el => el.open), true);
    assert.equal(await page.locator('#webDCEmpty').isVisible(), true);
    assert.equal(await page.locator('#officialAutoTargetSection').isVisible(), false);
    await page.evaluate(() => showDCSelection([
      { DataCenter: 'FRA', DCCountry: 'DE', City: 'Frankfurt', IPCount: 5, MinLatency: 100 },
      { DataCenter: 'SJC', DCCountry: 'US', City: 'San Jose', IPCount: 9, MinLatency: 150 },
      { DataCenter: 'GRU', DCCountry: 'BR', City: 'Sao Paulo', IPCount: 2, MinLatency: 90 },
    ]));
    const fra = page.locator('#webDCChoices input[data-dc="FRA"]');
    await fra.focus();
    await fra.press('Space');
    assert.equal(await fra.isChecked(), true);
    assert.equal(await fra.evaluate(el => el === document.activeElement), true, 'selection must preserve keyboard focus');
    assert.equal(await page.locator('#dcTable input[data-dc="FRA"]').isChecked(), true);
    await fra.press('Escape');
    assert.equal(await page.locator('#webDCSelector').evaluate(el => el.open), false);
    assert.equal(await summary.evaluate(el => el === document.activeElement), true);
    await page.locator('#dcTable input[data-dc="SJC"]').check();
    await page.locator('#dcTable input[data-dc="GRU"]').check();
    assert.deepEqual(await page.evaluate(() => getWebBatchRequest().datacenters), ['FRA', 'GRU', 'SJC']);
    await page.locator('#dcTable input[data-dc="GRU"]').uncheck();
    await page.locator('#officialAutoSpeed').selectOption('on');
    assert.equal(await page.locator('#officialAutoTargetSection').isVisible(), true);
    await page.locator('#officialTargetDC').fill('GRU');
    assert.equal(await page.evaluate(() => readFormValues().official.targetDC), 'GRU');
    await page.locator('#officialAutoSpeed').selectOption('off');
    assert.equal(await page.locator('#officialAutoTargetSection').isVisible(), false);
    assert.equal(await page.locator('#webBatchLimit').inputValue(), '10');

    const runBatch = async count => {
      resultCount = count;
      const expectedID = batchID + 1;
      await page.locator('.tab[onclick="switchTab(\'scan\')"]').click();
      await page.locator('#btnWebBatch').click();
      await page.waitForFunction(id => completedWebBatch?.id === id && !isTaskRunning, expectedID);
      assert.equal(await page.locator('#testTable tbody tr').count(), count);
    };
    await runBatch(10);
    assert.deepEqual(requests.find(r => r.type === 'start_web_batch').data.datacenters, ['FRA', 'SJC']);
    assert.match(await page.locator('#webBatchSummary').textContent(), /14.*14.*10\/10/);
    assert.deepEqual(await page.evaluate(() => testData.map(r => r.BatchRank)), [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
    await page.screenshot({ path: path.join(output, 'web-desktop.png'), fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    await summary.scrollIntoViewIfNeeded();
    await summary.click();
    const dropdownBounds = await page.locator('#webDCSelector .web-dc-dropdown').boundingBox();
    assert.ok(dropdownBounds && dropdownBounds.x >= 0 && dropdownBounds.x + dropdownBounds.width <= 391, 'datacenter dropdown overflows mobile viewport');
    await page.screenshot({ path: path.join(output, 'web-mobile.png'), fullPage: true });
    await summary.press('Escape');
    await page.setViewportSize({ width: 1440, height: 1050 });
    await runBatch(2);
    assert.match(await page.locator('#webBatchSummary').textContent(), /2\/10.*SJC/);
    await runBatch(0);
    const previewCount = requests.filter(r => r.type === 'huawei_preview').length;
    await page.evaluate(() => previewHuaweiDNS());
    assert.equal(requests.filter(r => r.type === 'huawei_preview').length, previewCount, 'empty batch attempted DNS preview');
    await page.evaluate(() => closeNoticeCard());
    holdBatch = true;
    await page.locator('.tab[onclick="switchTab(\'scan\')"]').click();
    await page.locator('#btnWebBatch').click();
    await page.waitForFunction(() => testData.length === 1 && isTaskRunning);
    // Exercise the same stop control as the normal Web flow, without a scan.
    await page.locator('#btnStart').click();
    await page.locator('#noticeCardActions button').last().click();
    await page.waitForFunction(() => !isTaskRunning && !completedWebBatch);
    assert.equal(requests.filter(r => r.type === 'stop_task').length, 1);
    await page.evaluate(() => previewHuaweiDNS());
    assert.equal(requests.filter(r => r.type === 'huawei_preview').length, previewCount, 'cancelled batch attempted DNS preview');
    await page.evaluate(() => closeNoticeCard());
    holdBatch = false;
    await runBatch(10);

    await page.evaluate(() => openHuaweiPanel());
    await page.waitForFunction(() => document.getElementById('huaweiConfigStatus').textContent.includes('6789'));
    assert.equal(await page.locator('#huaweiAK').inputValue(), '');
    assert.equal(await page.locator('#huaweiSK').inputValue(), '');
    assert.equal(await page.locator('#huaweiAK').evaluate(el => el === document.activeElement), true);
    assert.equal(await page.locator('#huaweiPersist').isChecked(), false);
    await page.locator('#huaweiAK').fill('MOCK_AK_PRIVATE_123456789');
    await page.locator('#huaweiSK').fill('MOCK_SK_PRIVATE_123456789');
    await page.evaluate(() => saveHuaweiConfig());
    await page.waitForFunction(() => document.getElementById('huaweiAK').value === '' && document.getElementById('huaweiSK').value === '');
    await page.locator('#huaweiPersist').check();
    await page.evaluate(() => saveHuaweiConfig());
    await page.waitForFunction(() => document.getElementById('huaweiConfigStatus').textContent.includes('6789'));
    await page.evaluate(() => discoverHuaweiZones(true));
    await page.locator('#huaweiZone').selectOption('zone1');
    await page.locator('#huaweiRecord').selectOption('record1');
    await page.waitForFunction(() => document.getElementById('huaweiTarget').textContent.includes('TTL: 300s'));
    assert.match(await page.locator('#huaweiTarget').textContent(), /default_view/);
    await page.evaluate(() => previewHuaweiDNS());
    await page.waitForFunction(() => !document.getElementById('huaweiConfirm').disabled);
    assert.match(await page.locator('#huaweiPreview').textContent(), /192\.0\.2\.1/);
    assert.match(await page.locator('#huaweiPreview').textContent(), /198\.51\.100\.1/);
    assert.equal(requests.filter(r => r.type === 'huawei_update').length, 0, 'preview must not update DNS');
    await page.locator('#huaweiAuto').check();
    await page.screenshot({ path: path.join(output, 'dns-desktop.png'), fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    const modalBounds = await page.locator('#huaweiModalMask .result-modal').boundingBox();
    assert.ok(modalBounds && modalBounds.x >= 0 && modalBounds.x + modalBounds.width <= 391, 'Huawei panel overflows mobile viewport');
    assert.equal(await page.locator('#huaweiModalMask').evaluate(el => el.scrollWidth <= el.clientWidth + 1), true);
    await page.screenshot({ path: path.join(output, 'dns-mobile.png'), fullPage: true });
    await page.locator('#huaweiPreview').scrollIntoViewIfNeeded();
    await page.screenshot({ path: path.join(output, 'dns-mobile-preview.png') });
    const firstControl = page.locator('#huaweiModalMask button').first();
    await page.locator('#huaweiConfirm').focus();
    await page.keyboard.press('Tab');
    assert.equal(await firstControl.evaluate(el => el === document.activeElement), true, 'DNS dialog must trap keyboard focus');
    await page.keyboard.press('Shift+Tab');
    assert.equal(await page.locator('#huaweiConfirm').evaluate(el => el === document.activeElement), true);
    page.once('dialog', dialog => dialog.accept());
    await page.locator('#huaweiConfirm').click();
    await page.waitForFunction(() => document.getElementById('huaweiDNSStatus').textContent.includes('Mock DNS update'));
    const update = requests.find(r => r.type === 'huawei_update');
    assert.equal(update.data.confirmed, true);
    assert.equal(update.data.token, 'mock-preview-token');
    assert.equal(requests.find(r => r.type === 'huawei_preview').data.results, undefined, 'browser must not author DNS addresses');
    const blankConfig = requests.filter(r => r.type === 'huawei_config_set').at(-1).data;
    assert.equal(blankConfig.ak, '');
    assert.equal(blankConfig.sk, '');
    assert.equal(blankConfig.persist, true, 'private save must be explicitly requested');
    previewChanged = false;
    await page.evaluate(() => previewHuaweiDNS());
    await page.waitForFunction(() => huaweiPreviewToken === 'mock-preview-token');
    assert.equal(await page.locator('#huaweiConfirm').isDisabled(), true, 'empty/no-change DNS preview enabled mutation');
    previewChanged = true;
    previewError = true;
    await page.evaluate(() => previewHuaweiDNS());
    await page.waitForFunction(() => document.getElementById('huaweiDNSStatus').textContent.includes('Preview stale'));
    assert.equal(await page.locator('#huaweiConfirm').isDisabled(), true);
    previewError = false;
    await page.evaluate(() => previewHuaweiDNS());
    await page.waitForFunction(() => !document.getElementById('huaweiConfirm').disabled);
    await page.evaluate(async () => {
      resultModalMode = 'test'; document.getElementById('githubRepo').value = 'owner/repo'; document.getElementById('githubToken').value = 'mock-token';
      document.getElementById('githubAutoSelected').checked = true; saveWebPublishOptions();
      await uploadSelectedResultsToGitHub();
    });
    await page.waitForFunction(() => document.getElementById('githubUploadStatus').textContent.includes('Mock GitHub upload'));
    const options = requests.filter(r => r.type === 'web_publish_options').at(-1).data;
    assert.equal(options.dns, true);
    assert.equal(options.github, true);
    const upload = requests.find(r => r.type === 'web_github_publish');
    assert.equal(upload.data.batchId, batchID);
    assert.equal(upload.data.content, undefined, 'browser must not author completed batch uploads');
    send('web_publish_status', { destination: 'dns', batchId: batchID, status: 'failed', message: 'Mock provider unavailable' });
    send('web_publish_status', { destination: 'github', batchId: batchID, status: 'uploaded', message: 'Mock GitHub success' });
    await page.waitForFunction(() => document.getElementById('githubUploadStatus').textContent.includes('Mock GitHub success'));
    assert.match(await page.locator('#huaweiDNSStatus').textContent(), /Mock provider unavailable/);
    assert.match(await page.locator('#huaweiDNSStatus').getAttribute('class'), /error/);
    assert.doesNotMatch(await page.locator('#githubUploadStatus').getAttribute('class'), /error/);
    const exported = await page.evaluate(() => JSON.stringify(readFormValues()));
    const storage = await page.evaluate(() => JSON.stringify({ ...localStorage }));
    assert.ok(!/MOCK_[AS]K/.test(exported + storage), 'Huawei keys escaped into browser export/storage');
    const connectionCount = connections;
    socket.close();
    await page.waitForFunction(() => completedWebBatch === null && document.getElementById('huaweiConfirm').disabled);
    await page.waitForFunction(() => ws && ws.readyState === WebSocket.OPEN);
    assert.equal(connections, connectionCount + 1, 'WebSocket did not reconnect');
    assert.equal(await page.evaluate(() => completedWebBatch), null, 'reconnect resurrected stale completion authority');
    await page.locator('#huaweiAK').focus();
    await page.keyboard.press('Escape');
    assert.equal(await page.locator('#huaweiModalMask').isVisible(), false);
    assert.equal(await page.locator('#settingsToggle').evaluate(el => el === document.activeElement), true);

    const replayRequests = [];
    let replayRows = [row(1)], backgrounded = false;
    const openReplayPage = async () => {
      const replayPage = await context.newPage();
      await replayPage.routeWebSocket('**/*', routed => {
        const emit = (type, data) => routed.send(JSON.stringify({ type, data }));
        emit('init_config', { version: 'offline-replay', skipGeoCheck: true, geoCheckOK: true, speedTestURL: 'auto' });
        routed.onMessage(raw => {
          const request = JSON.parse(String(raw)); replayRequests.push(request);
          if (request.type === 'check_proxy_country') emit('proxy_country_result', { cfCountry: 'SKIPPED', proxyWarning: false, geoCheckOK: true, skipGeoCheck: true });
          else if (request.type === 'start_speed_test') { replayRows[0].Speed = 'unmeasured'; emit('web_batch_invalidated', null); }
          else if (request.type === 'run_in_background') backgrounded = true;
          else if (request.type === 'follow_background_task') {
            assert.ok(backgrounded, 'fixture must background the retest before replay');
            emit('background_task_following', { running: false, mode: 'official', phase: 'complete', label: 'individual retest', resultCount: 1, speedCount: 1 });
            replayRows.forEach(row => emit('test_result', row)); emit('test_complete', replayRows);
          }
        });
      });
      await replayPage.goto(origin, { waitUntil: 'domcontentloaded' });
      await replayPage.waitForFunction(() => ws && ws.readyState === WebSocket.OPEN);
      return replayPage;
    };
    const original = await openReplayPage();
    await original.evaluate(row => {
      handleMessage({ type: 'web_batch_complete', data: { id: 7, results: [row], policy: { Datacenters: ['FRA'], Limit: 10 }, measuredCount: 1, qualifiedCount: 1 } });
      handleMessage({ type: 'task_complete', data: null });
      runSpeedTest(row.IP);
    }, replayRows[0]);
    await original.waitForFunction(() => completedWebBatch === null);
    await original.evaluate(() => wsSend({ type: 'run_in_background', data: {} }));
    await original.waitForTimeout(30);
    await original.close();
    const follower = await openReplayPage();
    assert.equal(await follower.evaluate(() => webBatchDerivedResults), false, 'fresh page must not inherit old tab state');
    await follower.evaluate(() => followBackgroundTask());
    await follower.waitForFunction(() => testData.length === 1 && webBatchDerivedResults);
    await follower.evaluate(async () => {
      resultModalMode = 'test'; document.getElementById('githubRepo').value = 'owner/repo'; document.getElementById('githubToken').value = 'mock-token';
      await uploadSelectedResultsToGitHub();
    });
    await follower.waitForTimeout(30);
    assert.ok(replayRequests.every(r => r.type !== 'github_upload' && r.type !== 'web_github_publish'), 'fresh background follower published invalid retest rows');
    replayRows = [{ ...replayRows[0], IP: '192.0.2.3', WebBatchDerived: false }];
    await follower.evaluate(() => { closeNoticeCard(); followBackgroundTask(); });
    await follower.waitForFunction(() => testData[0]?.IP === '192.0.2.3' && !webBatchDerivedResults);
    await follower.evaluate(() => uploadSelectedResultsToGitHub());
    await follower.waitForTimeout(30);
    assert.equal(replayRequests.filter(r => r.type === 'github_upload').length, 1, 'genuine legacy replay lost manual upload compatibility');
    assert.ok([...requests, ...replayRequests].every(r => r.type !== 'start_task' && r.type !== 'edgetunnel_upload'), 'smoke unexpectedly started scan/edgetunnel flow');
    assert.deepEqual(errors, []);
    console.log(`Offline Chrome smoke passed: arbitrary DC/keyboard, global top10/fewer/empty/cancel, DNS config/discovery/preview/confirmation/errors, independent statuses, reconnect and fresh legacy/provenance replay. Screenshots: ${output}`);
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
