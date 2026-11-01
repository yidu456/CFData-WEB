import { buildNSBRequest, createNSBState, reduceNSB, liveFields } from './model.js';
import { safeTaskText } from '../tasks/state.js';

export function mountNSB({ root, monitor }) {
  let state = createNSBState(), input = {}, readVersion = 0, reading = false, frame;
  const form = root.querySelector('[data-nsb-form]'), file = root.querySelector('[data-nsb-file]'), source = root.querySelector('[data-nsb-source]');
  const inputStatus = root.querySelector('[data-nsb-input-status]'), status = root.querySelector('[data-nsb-validation]');
  const start = root.querySelector('[data-nsb-start]');
  function ready() { start.disabled = !monitor.isReady() || reading; }
  function render() {
    ready();
    const headers = state.headers.length ? state.headers : liveFields;
    const count = state.headers.length ? state.csvRows.length : state.rows.length;
    const rows = state.headers.length ? state.csvRows.slice(-100) : state.rows.slice(-100).map(row => liveFields.map(key => row[key]));
    const head = root.createDocumentFragment();
    for (const value of headers) { const th = root.createElement('th'); th.scope = 'col'; th.textContent = value; head.append(th); }
    root.querySelector('[data-nsb-headers]').replaceChildren(head);
    const body = root.createDocumentFragment();
    for (const row of rows) {
      const tr = root.createElement('tr');
      for (const value of row) { const td = root.createElement('td'); td.textContent = safeTaskText(String(value ?? '')); tr.append(td); }
      body.append(tr);
    }
    root.querySelector('[data-nsb-rows]').replaceChildren(body);
    root.querySelector('[data-nsb-count]').textContent = `已保存 ${count} 条，显示最近 ${Math.min(100, count)} 条${state.file ? '；后端输出 ' + safeTaskText(state.file) : ''}${state.status ? '；' + ({partial:'部分完成',failed:'失败',complete:'完成'})[state.status] : ''}`;
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
      const data = buildNSBRequest(values, source.value === 'url' ? { sourceURL: values.sourceURL } : input);
      if (!monitor.startTask('start_nsb_task', data, '非标优选', 'nsb')) return;
      state = reduceNSB(state, { type: 'nsb_requested', data: { startedAt: monitor.getState().startedAt } });
      status.textContent = '已提交非标扫描；后端将解析/去重/解析域名并写入指定结果文件';
      render();
    } catch (error) { status.textContent = error.message; }
  });
  sourceChanged(); render();
  return { onState: ready, onMessage(message) {
    const next = reduceNSB(state, message);
    if (next !== state) { state = next; if (!frame) frame = requestAnimationFrame(() => { frame = undefined; render(); }); }
    else ready();
  } };
}
