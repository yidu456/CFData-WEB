const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { chromium } = require('playwright');
const origin = 'http://127.0.0.1:13335/';
const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
const scan = (n, dc) => ({ IP: `192.0.2.${n}`, Port: 443, DataCenter: dc, City: dc, TCPDuration: n * 1000000, LatencyStr: `${n}ms`, RecordedScanMode: 'tcping' });
const test = (n, dc) => ({ ...scan(n, dc), AvgLatency: n * 1000000, MinLatency: n * 1000000, MaxLatency: n * 1000000, LossRate: 0, Speed: '4MB/s' });
(async () => {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROME_PATH });
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
    context.setDefaultTimeout(5000);
    const page = await context.newPage(), errors = [], requests = [];
    let socket, view = { activeCenters: ['HKG'], selectedCenters: ['HKG'], activeTab: 'test' };
    const send = (type, data) => socket.send(JSON.stringify({ type, data }));
    page.on('pageerror', error => errors.push(error.message));
    page.on('dialog', dialog => dialog.accept());
    await context.route('**/*', r => r.request().url() === origin ? r.fulfill({ contentType: 'text/html', body: html }) : r.abort());
    await page.routeWebSocket('**/*', routed => {
      socket = routed;
      send('init_config', { skipGeoCheck: true, geoCheckOK: true, speedTestURL: 'auto' });
      routed.onMessage(raw => {
        const r = JSON.parse(String(raw)); requests.push(r);
        switch (r.type) {
          case 'web_state_get': send('web_state_response', { action: r.type, requestId: r.data.requestId, ok: true, metadata: { revision: 0 }, sections: {} }); break;
          case 'retained_results_get': send('retained_results', { generation: 1, mode: 'official', scans: [scan(1, 'HKG'), scan(2, 'NRT')], tests: [test(1, 'HKG')], nsbLive: [], centers: [{ DataCenter: 'HKG', IPCount: 1, MinLatency: 1 }, { DataCenter: 'NRT', IPCount: 1, MinLatency: 2 }], params: { scanMode: 'tcping' }, view, saved: true }); break;
          case 'retained_results_view': view = r.data.view; send('retained_results_saved', { generation: 1, saved: true }); break;
          case 'start_test': send('test_result', { ...test(2, 'NRT'), Speed: '' }); send('test_complete', [{ ...test(2, 'NRT'), Speed: '' }]); send('task_complete', null); break;
          case 'start_official_speed_batch': send('speed_test_result', { ip: '192.0.2.2', port: '443', speed: '5MB/s' }); send('official_speed_complete', { qualified: 1, totalQualified: 1, limit: 10 }); send('task_complete', null); break;
          case 'huawei_config_get': send('huawei_response', { action: r.type, data: { regions: ['cn-north-4'], region: 'cn-north-4', encryptedSaveSupported: true } }); break;
          case 'huawei_preview': {
            const old = ['192.0.2.9'];
            const values = r.data.mode === 'append' && !r.data.final ? [...new Set([...old, ...r.data.ips])] : [...new Set(r.data.ips)];
            send('huawei_response', { action: r.type, data: { token: r.data.requestId, requestId: r.data.requestId, batchId: 0, old, new: values, changed: true, target: { name: 'edge.example.', type: 'A', ttl: 300, line: 'default_view' } } }); break;
          }
          case 'huawei_update': send('web_publish_status', { destination: 'dns', batchId: 0, status: 'updated', message: 'offline' }); break;
          case 'check_proxy_country': send('proxy_country_result', { skipGeoCheck: true, geoCheckOK: true, proxyWarning: false }); break;
        }
      });
    });
    await page.goto(origin);
    await page.waitForFunction(() => testData.length === 1);
    const before = requests.length;
    await page.evaluate(() => { selectedWebDCs = new Set(['NRT']); startWebBatch(); });
    assert.equal(await page.locator('#testTable tbody tr').count(), 1);
    assert.match(await page.locator('#testTable tbody').innerText(), /192\.0\.2\.2/);
    assert.doesNotMatch(await page.locator('#testTable tbody').innerText(), /192\.0\.2\.1/);
    assert.equal(await page.evaluate(() => testData[0].Speed), '4MB/s');
    assert.deepEqual(requests.slice(before).filter(r => /start_|publish|upload|huawei/.test(r.type)), []);
    await page.locator('#btnDetailLatency').click();
    await page.waitForFunction(() => !isTaskRunning && testData.length === 2);
    assert.deepEqual(requests.find(r => r.type === 'start_test').data.datacenters, ['NRT']);
    assert.equal(requests.some(r => r.type === 'start_official_speed_batch'), false, 'latency completion cannot start speed');
    await page.evaluate(() => { window.autoUploadTriggered = false; autoUploadResults = async () => { window.autoUploadTriggered = true; }; document.getElementById('officialAutoUploadAccount').innerHTML='<option value="0:txt">test</option>'; document.getElementById('officialAutoUploadAccount').value='0:txt'; });
    await page.evaluate(() => { document.getElementById('officialSpeedLimit').value = '10'; requireProxyWarning = () => false; startOfficialBatchSpeed(); });
    await page.waitForFunction(() => !isTaskRunning && testData.find(r => r.DataCenter === 'NRT').Speed === '5MB/s');
    assert.deepEqual(requests.find(r => r.type === 'start_official_speed_batch').data.datacenters, ['NRT']);
    assert.equal(await page.evaluate(() => testData.find(r => r.DataCenter === 'HKG').Speed), '4MB/s');
    assert.equal(await page.evaluate(() => window.autoUploadTriggered), false, 'manual speed cannot trigger configured automatic uploads');
    await page.evaluate(() => { selectedWebDCs = new Set(['HKG']); startWebBatch(); });
    assert.match(await page.locator('#testTable tbody').innerText(), /4MB\/s/);
    await page.waitForTimeout(200);
    await page.reload();
    await page.waitForFunction(() => activeDetailCenters?.has('HKG'));
    assert.match(await page.locator('#testTable tbody').innerText(), /192\.0\.2\.1/);
    await page.evaluate(() => { openResultModal('test'); showResultActionView('huawei'); });
    await page.locator('#huaweiUpdateMode').selectOption('append');
    await page.evaluate(() => previewHuaweiDNS());
    await page.waitForFunction(() => !!huaweiPreviewToken);
    assert.equal(await page.locator('#huaweiNewIPs').inputValue(), '192.0.2.9\n192.0.2.1');
    await page.locator('#huaweiNewIPs').fill('192.0.2.8\n192.0.2.7\n192.0.2.8');
    assert.equal(await page.locator('#huaweiConfirm').isDisabled(), true);
    await page.evaluate(() => previewHuaweiDNS());
    await page.waitForFunction(() => !!huaweiPreviewToken);
    const last = requests.findLast(r => r.type === 'huawei_preview');
    assert.equal(last.data.final, true);
    assert.deepEqual(last.data.ips, ['192.0.2.8', '192.0.2.7']);
    assert.doesNotMatch(await page.locator('#huaweiNewIPs').inputValue(), /192\.0\.2\.9/);
    await page.evaluate(() => { window.copyText = ''; copyToClipboard = text => { window.copyText = text; }; copyHuaweiIPs({}); });
    assert.equal(await page.evaluate(() => window.copyText), '192.0.2.8\n192.0.2.7');
    await page.locator('#huaweiConfirm').click();
    await page.waitForFunction(() => !huaweiUpdatePending);
    assert.equal(requests.findLast(r => r.type === 'huawei_update').data.token, last.data.requestId);
    await page.locator('#huaweiNewIPs').fill('');
    const count = requests.filter(r => r.type === 'huawei_preview').length;
    await page.evaluate(() => previewHuaweiDNS());
    assert.equal(requests.filter(r => r.type === 'huawei_preview').length, count, 'empty draft must not clear DNS');
    await page.evaluate(() => closeNoticeCard());
    for (const width of [320, 390, 1440]) {
      await page.setViewportSize({ width, height: 1000 });
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), true);
      await page.screenshot({ path: path.join(os.tmpdir(), `cfdata-manual-dns-${width}.png`), fullPage: true });
    }
    assert.deepEqual(errors, []);
    console.log('Chrome manual detail scope/manual latency+speed/cache restore and editable append DNS PASS');
  } finally { await browser.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
