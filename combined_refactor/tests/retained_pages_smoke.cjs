// Offline browser regression: original workbench, durable results and provider pages.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { chromium } = require('playwright');

const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
const origin = 'http://127.0.0.1:13335/';
const output = path.join(os.tmpdir(), 'cfdata-retained-pages-qa');
const row = n => ({ IP: `198.51.100.${n}`, Port: 443, DataCenter: 'HKG', City: 'Hong Kong', AvgLatency: n * 1000000, MinLatency: n * 1000000, MaxLatency: n * 1000000, LossRate: 0, Speed: `${30 - n}MB/s` });

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROME_PATH });
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
    context.setDefaultTimeout(5000);
    const errors = [], requests = [];
    let socket, rejectStart = true, rejectClear = true;
    let saved = { generation: 1, mode: 'official', scans: [row(1)], tests: Array.from({ length: 18 }, (_, i) => row(i + 1)), centers: [{ DataCenter: 'HKG', City: 'Hong Kong', IPCount: 1, MinLatency: 1 }], nsbLive: [], params: {}, view: { selectedCenters: ['HKG'], testedCenters: ['HKG'], activeTab: 'test' }, saved: true };
    await context.route('**/*', route => route.request().url() === origin
      ? route.fulfill({ contentType: 'text/html', body: html }) : route.abort());
    const page = await context.newPage();
    page.on('pageerror', e => errors.push(e.message));
    page.on('dialog', dialog => dialog.accept());
    const send = (type, data) => socket.send(JSON.stringify({ type, data }));
    await page.routeWebSocket('**/*', routed => {
      socket = routed;
      send('init_config', { skipGeoCheck: true, geoCheckOK: true, speedTestURL: 'auto' });
      routed.onMessage(raw => {
        const r = JSON.parse(String(raw)); requests.push(r);
        switch (r.type) {
          case 'web_state_get': send('web_state_response', { action: r.type, requestId: r.data.requestId, ok: true, metadata: { revision: 0 }, sections: {} }); break;
          case 'retained_results_get': send('retained_results', saved); break;
          case 'retained_results_view': saved.view = r.data.view; break;
          case 'retained_results_clear':
            if (rejectClear) send('retained_results_error', { action: r.type, message: 'disk save failed, old results retained' });
            else { saved = { generation: saved.generation + 1, mode: 'official', scans: [], tests: [], nsbLive: [], centers: [], view: {}, saved: true }; send('retained_results', saved); }
            break;
          case 'start_task':
            if (rejectStart) send('retained_results_error', { message: '已有任务正在运行，旧结果保留', code: 'busy' });
            else { saved = { generation: saved.generation + 1, mode: 'official', scans: [], tests: [], nsbLive: [], centers: [], view: {}, saved: true }; send('retained_results_started', saved); send('task_complete', null); }
            break;
          case 'huawei_config_get': send('huawei_response', { action: r.type, data: { regions: ['cn-north-4'], region: 'cn-north-4', encryptedSaveSupported: true } }); break;
          case 'huawei_preview': send('huawei_response', { action: r.type, data: { token: 'manual-token', batchId: 0, requestId: r.data.requestId, target: { name: 'edge.example.', type: 'A', line: 'default_view', ttl: 300 }, old: ['192.0.2.1'], new: r.data.ips, changed: true } }); break;
          case 'huawei_update': send('web_publish_status', { destination: 'dns', batchId: 0, status: 'updated', message: 'offline update' }); break;
          case 'check_proxy_country': send('proxy_country_result', { skipGeoCheck: true, geoCheckOK: true, proxyWarning: false }); break;
          case 'web_publish_options': send('huawei_response', { action: r.type, data: { dns: false, github: false } }); break;
        }
      });
    });
    await page.goto(origin);
    await page.waitForFunction(() => testData.length === 18);
    await page.evaluate(() => { window.savedSend = wsSend; });
    assert.equal(await page.evaluate(() => selectedWebDCs.has('HKG')), true);
    await page.evaluate(() => { openResultModal('test'); showResultActionView('github'); });
    assert.equal(await page.locator('#resultModalMask').getAttribute('role'), 'region');
    assert.equal(await page.locator('body > .container').isVisible(), false);
    await page.locator('#githubPath').fill('keep/results.txt');
    await page.locator('#resultEndRow').fill('7');
    await page.locator('#resultBack').click();
    await page.locator('#resultEdgetunnelChoice').click();
    assert.equal(await page.locator('#resultEndRow').inputValue(), '20', 'provider output drafts are independent');
    await page.locator('#resultEndRow').fill('12');
    await page.locator('#resultBack').click();
    await page.locator('#resultGitHubChoice').click();
    assert.equal(await page.locator('#githubPath').inputValue(), 'keep/results.txt');
    assert.equal(await page.locator('#resultEndRow').inputValue(), '7');
    await page.locator('#resultBack').click();
    await page.locator('#resultHuaweiChoice').click();
    assert.equal(await page.locator('#huaweiModalMask').getAttribute('role'), 'region');
    await page.locator('#huaweiEndRow').fill('12');
    await page.evaluate(() => previewHuaweiDNS());
    await page.waitForFunction(() => huaweiPreviewToken === 'manual-token');
    const preview = requests.findLast(r => r.type === 'huawei_preview');
    assert.equal(preview.data.batchId, 0);
    assert.deepEqual(preview.data.ips, Array.from({ length: 12 }, (_, i) => row(i + 1).IP));
    await page.locator('#huaweiEndRow').fill('10');
    assert.equal(await page.locator('#huaweiConfirm').isDisabled(), true, 'changing selection invalidates server preview');
    await page.evaluate(() => previewHuaweiDNS());
    await page.waitForFunction(() => huaweiPreviewToken === 'manual-token');
    await page.evaluate(() => confirmHuaweiDNS());
    await page.waitForFunction(() => document.getElementById('huaweiDNSStatus').textContent.includes('offline update'));
    assert.equal(requests.findLast(r => r.type === 'huawei_update').data.batchId, 0);
    await page.locator('#huaweiBack').click();
    await page.evaluate(() => closeResultModal());
    assert.equal(await page.evaluate(() => testData.length), 18, 'navigation never discards results');
    assert.equal(await page.evaluate(() => {
      document.getElementById('officialSpeedLimit').value = '10';
      requireProxyWarning = () => false;
      let attempted = false;
      wsSend = () => { attempted = true; return false; };
      startOfficialBatchSpeed();
      return attempted;
    }), true, 'speed retry reaches transport');
    assert.equal(await page.evaluate(() => testData[0].Speed), row(1).Speed, 'disconnected speed retry preserves measurement');
    await page.evaluate(() => { closeNoticeCard(); wsSend = window.savedSend; });
    await page.evaluate(() => startOfficialTask());
    await page.waitForFunction(() => !pendingInitialScan);
    assert.equal(await page.evaluate(() => testData.length), 18, 'rejected starts preserve old data');
    await page.evaluate(() => closeNoticeCard());
    await page.reload();
    await page.waitForFunction(() => testData.length === 18);
    assert.equal(requests.filter(r => r.type === 'start_task').length, 1, 'restore must not start work');
    await page.evaluate(() => {
      mergeRetainedTests([{ ...testData[0], Port: 8443, Speed: '7MB/s', ExactSpeed: { measured: true, mb: 7 } }]);
      renderAllTestResults();
    });
    assert.equal(await page.locator('#btn-speed-198_51_100_1_443').count(), 1);
    assert.equal(await page.locator('#btn-speed-198_51_100_1_8443').count(), 1);
    send('speed_test_result', { ip: row(1).IP, port: '8443', speed: 'failed' });
    await page.waitForFunction(() => testData.find(row => row.Port === 8443).Speed === 'failed');
    assert.equal(await page.evaluate(() => testData.find(row => row.IP === '198.51.100.1' && row.Port === 443).Speed), row(1).Speed);
    assert.equal(await page.evaluate(() => matchesOfficialQualifiedFilter(testData.find(row => row.Port === 8443), 'qualified')), false);
    for (const width of [320, 390, 736, 1024, 1440]) {
      await page.setViewportSize({ width, height: 1000 });
      for (const provider of ['github', 'edgetunnel', 'huawei']) {
        await page.evaluate(p => { openResultModal('test'); showResultActionView(p); }, provider);
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), true, `${provider} overflow at ${width}`);
        await page.screenshot({ path: path.join(output, `${provider}-${width}.png`), fullPage: true });
        await page.evaluate(p => p === 'huawei' ? closeHuaweiPanel() : showResultActionView('chooser'), provider);
        await page.evaluate(() => closeResultModal());
      }
    }
    rejectStart = false;
    await page.evaluate(() => startOfficialTask());
    await page.waitForFunction(() => retainedGeneration === 2);
    assert.equal(await page.evaluate(() => testData.length), 0, 'accepted new scan replaces old results');
    saved.tests = [row(1)];
    send('retained_results', saved);
    await page.waitForFunction(() => testData.length === 1);
    await page.evaluate(() => { clearRetainedResults(); noticeConfirmAction(); closeNoticeCard(); });
    await page.waitForFunction(() => document.getElementById('retainedStatus').textContent.includes('disk save failed'));
    assert.equal(await page.evaluate(() => testData.length), 1, 'failed clear cannot erase current or durable results');
    rejectClear = false;
    await page.evaluate(() => { clearRetainedResults(); noticeConfirmAction(); closeNoticeCard(); });
    await page.waitForFunction(() => retainedGeneration === 3 && testData.length === 0);
    assert.deepEqual(errors, []);
    console.log(`Retained results/provider pages/DNS selection/failed starts/mobile PASS; screenshots: ${output}`);
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
