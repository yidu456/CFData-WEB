import { buildNSBRequest, buildNSBExecution, hasNSBPendingRows, buildNSBFollowup, createNSBState, reduceNSB, liveFields, getNSBResultRows } from './model.js';
import { safeTaskText } from '../tasks/state.js';
import { mountResultBrowser } from '../results/controls.js';
import { appendAddressActions } from '../results/output-view.js';

export function mountNSB({ root, monitor }) {
  let state = createNSBState(), input = {}, readVersion = 0, reading = false, frame;
  const form = root.querySelector('[data-nsb-form]'), file = root.querySelector('[data-nsb-file]'), source = root.querySelector('[data-nsb-source]');
  const inputStatus = root.querySelector('[data-nsb-input-status]'), status = root.querySelector('[data-nsb-validation]');
  const start = root.querySelector('[data-nsb-start]');
  const dialog = root.querySelector('[data-nsb-confirm]');
  let confirmRows;
  const browser=mountResultBrowser({root,name:'nsb',type:'nsb',onChange:render});
  function ready() {
    start.disabled = !monitor.isReady() || reading;
    const available = state.headers.length ? state.csvRows.length : state.rows.length;
    root.querySelector('[data-nsb-speed]').disabled = !monitor.isReady() || !available || Number(form.elements.speedLimit.value) <= 0;
    root.querySelector('[data-nsb-continue]').disabled = !monitor.isReady() || !available || !hasNSBPendingRows(state) || Number(form.elements.speedLimit.value) <= 0;
    root.querySelector('[data-nsb-relink]').disabled = !monitor.isReady() || !available;
  }
  function render() {
    ready();
    const headers = state.headers.length ? state.headers : liveFields;
    const count = state.headers.length ? state.csvRows.length : state.rows.length;
    const sourceRows=state.headers.length?state.csvRows:state.rows;
    const page=browser.select(sourceRows,state,state.headers,Number(form.elements.speedMin.value)||.1);
    const rows = page.indexes.map(index=>state.headers.length?state.csvRows[index]:liveFields.map(key=>state.rows[index][key]));
    const head = root.createDocumentFragment();
    for (const value of headers) { const th = root.createElement('th'); th.scope = 'col'; th.textContent = value; head.append(th); }
    root.querySelector('[data-nsb-headers]').replaceChildren(head);
    const body = root.createDocumentFragment();
    const fields = state.headers.length ? state.csvFields : liveFields;
    for (const row of rows) {
      const tr = root.createElement('tr');
      for (const [index,value] of row.entries()) {
        const td = root.createElement('td');
        if (fields[index] === 'ip' || fields[index] === 'port') appendAddressActions(td,String(value ?? ''),row[fields.indexOf('originalInput')],fields[index] === 'ip');
        else td.textContent = safeTaskText(String(value ?? ''));
        tr.append(td);
      }
      body.append(tr);
    }
    root.querySelector('[data-nsb-rows]').replaceChildren(body);
    root.querySelector('[data-nsb-count]').textContent = `已保存 ${count} 条，筛选 ${page.total} 条，每页100条${state.file ? '；后端输出 ' + safeTaskText(state.file) : ''}${state.status ? '；' + ({partial:'部分完成',failed:'失败',complete:'完成'})[state.status] : ''}`;
  }
  function sourceChanged() {
    readVersion++; reading = false; input = {};
    file.value = ''; form.elements.sourceURL.value = '';
    file.disabled = source.value !== 'file'; form.elements.sourceURL.disabled = source.value !== 'url';
    inputStatus.textContent = source.value === 'file' ? '请选择 TXT/CSV 文件' : 'URL 由后端读取，不在浏览器获取';
    ready();
  }
  source.addEventListener('change', sourceChanged);
  file.addEventListener('change', async () => {
    const version = ++readVersion, selected = file.files[0]; input = {}; reading = false;
    if (!selected) { reading = false; ready(); return; }
    if (selected.size > 16 * 1024 * 1024) { inputStatus.textContent = '文件超过 16MiB，请用 CLI 处理'; file.value = ''; ready(); return; }
    reading = true; inputStatus.textContent = '正在读取文件'; ready();
    try {
      const content = await selected.text();
      if (version !== readVersion || source.value !== 'file') return;
      input = { fileName: selected.name, fileContent: content };
      inputStatus.textContent = `已读取 ${selected.name}，${selected.size} 字节`;
    } catch {
      if (version !== readVersion) return;
      inputStatus.textContent = '文件读取失败，请重新选择';
    } finally { if (version === readVersion) { reading = false; ready(); } }
  });
  form.elements.enableTLS.addEventListener('change', () => {
    const value = form.elements.fallbackPort.value;
    if (value === '443' || value === '80') form.elements.fallbackPort.value = form.elements.enableTLS.value === 'true' ? '443' : '80';
  });
  form.addEventListener('submit', event => {
    event.preventDefault(); if (!monitor.isReady() || reading) return;
    try {
      const values = Object.fromEntries(new FormData(form));
      const data = { ...buildNSBRequest(values, source.value === 'url' ? { sourceURL: values.sourceURL } : input), ...buildNSBExecution(values) };
      if (!monitor.startTask('start_nsb_task', data, '非标优选', 'nsb')) return;
      state = reduceNSB(state, { type: 'nsb_requested', data: { startedAt: monitor.getState().startedAt } });
      status.textContent = '已提交非标扫描；后端将解析/去重/解析域名并写入指定结果文件';
      render();
    } catch (error) { status.textContent = error.message; }
  });
  form.elements.autoSpeed.addEventListener('change', () => {
    if (form.elements.autoSpeed.value === 'on' && Number(form.elements.speedTest.value) <= 0) form.elements.speedTest.value = '5';
  });
  form.elements.nsbSpeedPreset.addEventListener('change', () => { if (form.elements.nsbSpeedPreset.value !== 'custom') form.elements.speedURL.value = form.elements.nsbSpeedPreset.value; });
  form.elements.speedURL.addEventListener('input', () => { form.elements.nsbSpeedPreset.value = 'custom'; });
  form.addEventListener('input', ready);
  form.elements.speedMin.addEventListener('input', render);
  function followup(action, continued) {
    if (!monitor.isReady()) return;
    try {
      const values = Object.fromEntries(new FormData(form));
      const data = buildNSBFollowup(values, state, action, continued);
      if (!monitor.startTask(action === 'speed' ? 'start_nsb_speed_batch' : 'start_nsb_reallink', data, action === 'speed' ? '非标批量测速' : '链路复验', 'nsb')) return;
      state = reduceNSB(state, { type: 'nsb_followup_requested', data: { action, continued } });
      status.textContent = action === 'speed' ? '已提交非标测速，更新原端点结果；继续测速跳过已测试行' : '已提交链路复验，不可用候选将被剔除';
      render();
    } catch (error) { status.textContent = error.message; }
  }
  root.querySelector('[data-nsb-speed]').addEventListener('click', () => followup('speed', false));
  root.querySelector('[data-nsb-continue]').addEventListener('click', () => followup('speed', true));
  root.querySelector('[data-nsb-relink]').addEventListener('click', () => { confirmRows = state.headers.length ? state.csvRows : state.rows; dialog.showModal(); });
  root.querySelector('[data-nsb-confirm-cancel]').addEventListener('click', () => dialog.close());
  root.querySelector('[data-nsb-confirm-ok]').addEventListener('click', () => { dialog.close(); if (confirmRows === (state.headers.length ? state.csvRows : state.rows)) followup('relink', false); });
  sourceChanged(); render();
  return { getOutputRows: () => getNSBResultRows(state), getSpeedMin: () => Number(form.elements.speedMin.value) || .1,
    onState(value) { if (value !== 'connected') { confirmRows = undefined; if (dialog.open) dialog.close(); } ready(); }, onMessage(message) {
    if (message.type === 'init_config' || message.type === 'speed_url_status') {
      const payload = message.type === 'init_config' ? message.data?.speedURLStatus : message.data;
      for (const entry of payload?.urls || []) {
        const option = [...form.elements.nsbSpeedPreset.options].find(option => option.value === entry.url);
        if (option) option.textContent = `${entry.label}${entry.tested && !entry.available ? '（不可用）' : entry.tested ? '（可用）' : '（未检测）'}`;
      }
    }
    const next = reduceNSB(state, message);
    if (next !== state) { state = next; if (!frame) frame = requestAnimationFrame(() => { frame = undefined; render(); }); }
    else ready();
  } };
}
