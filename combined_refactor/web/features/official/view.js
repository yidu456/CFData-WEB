import { buildScanRequest, buildTestRequest, createOfficialState, reduceOfficial, visibleScanRows, visibleTestRows } from './model.js';

export function mountOfficial({ root, monitor }) {
  let state = createOfficialState(), frame;
  const form = root.querySelector('[data-official-form]');
  const start = root.querySelector('[data-official-start]');
  const status = root.querySelector('[data-official-validation]');
  const rows = root.querySelector('[data-official-rows]');
  const centers = root.querySelector('[data-official-centers]');
  const testButton = root.querySelector('[data-official-test]');
  function updateReady() {
    start.disabled = !monitor.isReady();
    testButton.disabled = !monitor.isReady() || !state.selectedDC || !state.candidatesReady;
  }
  function render() {
    updateReady();
    root.querySelector('[data-official-count]').textContent = `已保存 ${state.rows.length} 条扫描结果；当前显示最近 ${Math.min(state.rows.length, 100)} 条，按延迟排序`;
    const fragment = root.createDocumentFragment();
    for (const row of visibleScanRows(state)) {
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
    root.querySelector('[data-official-test-count]').textContent = `已保存 ${state.testRows.length} 条详细结果；显示最近 ${Math.min(state.testRows.length, 100)} 条，按平均延迟排序`;
    const detail = root.createDocumentFragment();
    const milliseconds = value => value === null ? '未知' : value.toFixed(2) + 'ms';
    for (const row of visibleTestRows(state)) {
      const tr = root.createElement('tr');
      for (const value of [row.IP, row.Port, row.DataCenter || '—', [row.DCCountry, row.Region, row.City].filter(Boolean).join(' / ') || '—', milliseconds(row.minMS), milliseconds(row.maxMS), milliseconds(row.avgMS), row.lossPercent === null ? '未知' : row.lossPercent.toFixed(2) + '%', row.Speed || '未测速']) {
        const cell = root.createElement('td'); cell.textContent = String(value); tr.append(cell);
      }
      detail.append(tr);
    }
    root.querySelector('[data-official-test-rows]').replaceChildren(detail);
  }
  form.addEventListener('submit', event => {
    event.preventDefault();
    if (!monitor.isReady()) return;
    try {
      const data = buildScanRequest(Object.fromEntries(new FormData(form)));
      if (!monitor.startTask('start_task', data, '官方优选扫描')) { status.textContent = '任务未发送，请检查连接或当前运行任务'; return; }
      state = reduceOfficial(state, { type: 'scan_requested', data: { startedAt: monitor.getState().startedAt } });
      status.textContent = '已提交官方扫描；停止和后台操作位于全局任务区域';
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
      const next = reduceOfficial(state, message);
      if (next !== state) {
        state = next;
        if (!frame) frame = requestAnimationFrame(() => { frame = undefined; render(); });
      }
      else updateReady();
    },
  };
}
