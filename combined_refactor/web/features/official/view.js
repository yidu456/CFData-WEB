import { buildScanRequest, buildTestRequest, createOfficialState, reduceOfficial } from './model.js';
import { buildSpeedConfig, buildSpeedRequest, buildBatchRequest, buildAutoScanRequest, speedStatus } from '../measurement/model.js';
import { mountResultBrowser } from '../results/controls.js';

export function mountOfficial({ root, monitor }) {
  let state = createOfficialState(), frame;
  const form = root.querySelector('[data-official-form]');
  const start = root.querySelector('[data-official-start]');
  const status = root.querySelector('[data-official-validation]');
  const rows = root.querySelector('[data-official-rows]');
  const centers = root.querySelector('[data-official-centers]');
  const testButton = root.querySelector('[data-official-test]');
  const speedForm = root.querySelector('[data-speed-form]');
  const speedConfig = () => buildSpeedConfig({ ...Object.fromEntries(new FormData(speedForm)), port: form.elements.port.value });
  const scanBrowser=mountResultBrowser({root,name:'scan',type:'scan',onChange:render});
  const testBrowser=mountResultBrowser({root,name:'detail',type:'detail',onChange:render});
  function updateReady() {
    start.disabled = !monitor.isReady();
    testButton.disabled = !monitor.isReady() || !state.selectedDC || !state.candidatesReady;
    root.querySelector('[data-speed-batch]').disabled = !monitor.isReady() || !state.testRows.length || Number(speedForm.elements.speedLimit.value) <= 0;
    root.querySelector('[data-speed-continue]').disabled = !monitor.isReady() || !state.continueUnlocked || !state.testRows.some(row => !row.Speed || row.Speed === '未测速') || Number(speedForm.elements.speedLimit.value) <= 0;
    for (const button of root.querySelectorAll('[data-speed-ip]')) button.disabled = !monitor.isReady();
  }
  function render() {
    updateReady();
    const scanPage=scanBrowser.select(state.rows,state);
    root.querySelector('[data-official-count]').textContent = `已保存 ${state.rows.length} 条扫描结果；当前筛选 ${scanPage.total} 条，每页100条`;
    const fragment = root.createDocumentFragment();
    for (const index of scanPage.indexes) {
      const row=state.rows[index];
      const tr = root.createElement('tr');
      for (const value of [row.IP, row.Port, row.DataCenter || '—', [row.DCCountry, row.Region, row.City].filter(Boolean).join(' / ') || '—', row.LatencyStr || (row.latencyMS === null ? '未知' : row.latencyMS.toFixed(2) + 'ms')]) {
        const cell = root.createElement('td'); cell.textContent = String(value); tr.append(cell);
      }
      fragment.append(tr);
    }
    rows.replaceChildren(fragment);
    const dcFragment = root.createDocumentFragment();
    for (const center of state.centers) {
      const button = root.createElement('button');
      button.type = 'button'; button.className = 'dc-choice'; button.dataset.dc = center.DataCenter;
      button.setAttribute('aria-pressed', String(state.selectedDC === center.DataCenter));
      button.textContent = `${center.DataCenter} ${center.City || center.DCCountry} · ${center.IPCount} 个 IP · 最低 ${center.MinLatency ?? '未知'}ms${state.testingDC === center.DataCenter ? ' · 测试中' : state.testedDCs.has(center.DataCenter) ? ' · 已测试' : ''}`;
      button.addEventListener('click', () => { state = reduceOfficial(state, { type: 'select_dc', data: center.DataCenter }); render(); });
      dcFragment.append(button);
    }
    centers.replaceChildren(dcFragment);
    root.querySelector('[data-official-selection]').textContent = state.testingDC ? `正在测试 ${state.testingDC}` : state.selectedDC ? `已选择 ${state.selectedDC}${state.testedDCs.has(state.selectedDC) ? ' · 已测试，可重新测试' : ''}` : '扫描完成后选择数据中心';
    testButton.textContent = state.selectedDC && state.testedDCs.has(state.selectedDC) ? '重新详细测试' : '开始详细测试';
    const detailPage=testBrowser.select(state.testRows,state,undefined,Number(speedForm.elements.speedMin.value)||.1);
    root.querySelector('[data-official-test-count]').textContent = `已保存 ${state.testRows.length} 条详细结果；当前筛选 ${detailPage.total} 条，每页100条`;
    const detail = root.createDocumentFragment();
    const milliseconds = value => value === null ? '未知' : value.toFixed(2) + 'ms';
    for (const index of detailPage.indexes) {
      const row=state.testRows[index];
      const tr = root.createElement('tr');
      for (const value of [row.IP, row.Port, row.DataCenter || '—', [row.DCCountry, row.Region, row.City].filter(Boolean).join(' / ') || '—', milliseconds(row.minMS), milliseconds(row.maxMS), milliseconds(row.avgMS), row.lossPercent === null ? '未知' : row.lossPercent.toFixed(2) + '%', row.Speed || '未测速']) {
        const cell = root.createElement('td'); cell.textContent = String(value); tr.append(cell);
      }
      const speed = root.createElement('td');
      const tag = root.createElement('span');
      tag.textContent = ({ untested: '未测速', qualified: '达标', unqualified: '未达标', failed: '失败' })[speedStatus(row.Speed, Number(speedForm.elements.speedMin.value) || .1)];
      speed.append(tag);
      const button = root.createElement('button'); button.className = 'button'; button.type = 'button'; button.dataset.speedIp = row.IP;
      button.textContent = row.Speed ? '重新测速' : '测速'; button.disabled = !monitor.isReady();
      button.addEventListener('click', () => startSpeed(false, row.IP));
      speed.append(button); tr.append(speed);
      detail.append(tr);
    }
    root.querySelector('[data-official-test-rows]').replaceChildren(detail);
  }
  form.addEventListener('submit', event => {
    event.preventDefault();
    if (!monitor.isReady()) return;
    try {
      let data = buildScanRequest(Object.fromEntries(new FormData(form)));
      if (form.elements.automatic.value === 'on') data = buildAutoScanRequest(data, speedConfig(), form.elements.targetDC.value);
      if (!monitor.startTask('start_task', data, '官方优选扫描')) { status.textContent = '任务未发送，请检查连接或当前运行任务'; return; }
      state = reduceOfficial(state, { type: 'scan_requested', data: { startedAt: monitor.getState().startedAt, automatic: data.serverAutoFlow } });
      status.textContent = data.serverAutoFlow ? '已提交自动流程，后端串行完成扫描/详细测试/测速；转后台后继续，0 达标数量跳过测速' : '已提交官方扫描；停止和后台操作位于全局任务区域';
      render();
    } catch (error) { status.textContent = error.message; }
  });
  form.elements.scanMode.addEventListener('change', () => {
    root.querySelector('[data-official-measurement]').textContent = form.elements.scanMode.value === 'httping'
      ? 'HTTPing 测量 HTTP 首字节响应；TLS/非 TLS 的通过阈值口径不同，不能直接与 TCPing 数值比较。'
      : 'TCPing 测量 TCP 建连延迟；合格延迟 0 表示不以该阈值筛除。';
  });
  testButton.addEventListener('click', () => {
    if (!monitor.isReady() || !state.candidatesReady) return;
    try {
      const data = buildTestRequest(Object.fromEntries(new FormData(form)), state.selectedDC, state.centers);
      if (!monitor.startTask('start_test', data, `官方详细测试 ${data.dc}`)) return;
      state = reduceOfficial(state, { type: 'test_requested', data });
      status.textContent = `已提交 ${data.dc} 详细测试，沿用当前端口和测量参数`;
      render();
    } catch (error) { status.textContent = error.message; }
  });
  function startSpeed(continued, ip) {
    if (!monitor.isReady()) return;
    try {
      const config = speedConfig();
      const data = ip ? buildSpeedRequest(config, state.testRows, ip) : buildBatchRequest(config, state.testRows, continued);
      const type = ip ? 'start_speed_test' : 'start_official_speed_batch';
      if (!monitor.startTask(type, data, ip ? `单 IP 测速 ${ip}` : continued ? '官方继续测速' : '官方批量测速')) return;
      state = reduceOfficial(state, { type: 'speed_requested', data: { ip, continued } });
      status.textContent = '测速已提交；达标目标与尝试数量不同，停止和限流保留已取得结果';
      render();
    } catch (error) { status.textContent = error.message; }
  }
  root.querySelector('[data-speed-batch]').addEventListener('click', () => startSpeed(false));
  root.querySelector('[data-speed-continue]').addEventListener('click', () => startSpeed(true));
  speedForm.addEventListener('input', render);
  speedForm.addEventListener('submit', event => event.preventDefault());
  speedForm.elements.preset.addEventListener('change', () => {
    if (speedForm.elements.preset.value !== 'custom') speedForm.elements.url.value = speedForm.elements.preset.value;
  });
  speedForm.elements.url.addEventListener('input', () => { speedForm.elements.preset.value = 'custom'; });
  render();
  return {
    onState(value) {
      if (value !== 'connected') {
        state = reduceOfficial(state, { type: 'connection_lost' });
        status.textContent = '连接已中断，显示结果保留；详细测试需重新扫描或跟随后台任务恢复候选';
        render();
      } else updateReady();
    },
    onMessage(message) {
      if (message.type === 'init_config' || message.type === 'speed_url_status') {
        const payload = message.type === 'init_config' ? message.data?.speedURLStatus : message.data;
        for (const entry of payload?.urls || []) {
          const option = [...speedForm.elements.preset.options].find(option => option.value === entry.url);
          if (option) option.textContent = `${entry.label}${entry.tested && !entry.available ? '（不可用）' : entry.tested ? '（可用）' : '（未检测）'}`;
        }
      }
      const next = reduceOfficial(state, message);
      if (next !== state) {
        state = next;
        if (!frame) frame = requestAnimationFrame(() => { frame = undefined; render(); });
      }
      else updateReady();
    },
  };
}
