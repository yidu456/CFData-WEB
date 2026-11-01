import { availableOutputFields, buildOutput, createOutputConfig, defaultOutputFields, normalizeOutputRows, parseCustomFields } from './output.js';
import { copyText, downloadText } from '../../platform/output.js';
import { safeTaskText } from '../tasks/state.js';

export function appendAddressActions(cell, value, source = '', ip = false) {
  const root = cell.ownerDocument, window = root.defaultView;
  const button = root.createElement('button');
  button.type = 'button'; button.className = 'copy-address'; button.dataset.copyText = String(value);
  button.textContent = String(value); button.setAttribute('aria-label', `复制${ip ? ' IP ' : '端口 '}${value}`);
  button.addEventListener('click', async () => {
    const status = root.querySelector('[data-output-feedback]');
    try { await copyText(value,{navigator:window.navigator,document:root}); status.textContent = `已复制 ${value}`; }
    catch (error) { status.textContent = error.message; }
  });
  cell.append(button);
  if (!ip) return;
  if (source && source !== String(value) && /[a-z]/i.test(source)) {
    const hint = root.createElement('span'); hint.dataset.sourceHint = ''; hint.textContent = '域名';
    hint.title = `来源域名：${safeTaskText(source)}`; cell.append(hint);
  }
  const link = root.createElement('a');
  link.href = `https://ippure.com/?ip=${encodeURIComponent(value)}`; link.target = '_blank'; link.rel = 'noopener noreferrer';
  link.textContent = '详情'; link.setAttribute('aria-label', `在 ippure.com 查看 ${value} 的 IP 详情`); cell.append(link);
}

export function mountResultOutput({root, getRows, getSpeedMin}) {
  const window = root.defaultView;
  if (typeof window.downloadFile !== 'function') window.downloadFile = (content,name,format) => downloadText(content,name,format,window);
  const dialog = root.querySelector('[data-output-dialog]'), form = dialog.querySelector('form');
  const preview = dialog.querySelector('[data-output-preview]'), status = dialog.querySelector('[data-output-status]');
  const execute = dialog.querySelector('[data-output-execute]'), list = dialog.querySelector('[data-output-fields]');
  const configs = new Map(); let config, frame, fieldSignature = '', generation = 0, pendingGeneration;
  const inputNames = ['format','start','end','ipType','qualified','latency','loss','separatorPreset','separator','brackets','name','custom'];
  function focusField(key, direction) {
    const checkbox = [...list.querySelectorAll('[data-output-key]')].find(input => input.dataset.outputKey === key);
    const move = direction && checkbox?.parentElement.parentElement.querySelector(`[data-field-move="${direction}"]`);
    (move && !move.disabled ? move : checkbox)?.focus();
  }
  function readConfig() {
    for (const name of inputNames) config[name] = name === 'brackets' ? form.elements[name].checked : form.elements[name].value;
    if (config.kind === 'copy') config.format = 'txt';
    const previous = new Set(config.constants.map(field => field.key));
    const constants = parseCustomFields(config.custom), next = new Set(constants.map(field => field.key));
    config.fields = config.fields.filter(key => !previous.has(key) || next.has(key));
    for (const field of constants) if (!previous.has(field.key)) config.fields.push(field.key);
    config.constants = constants;
  }
  function renderFields(rows) {
    const available = availableOutputFields(rows,config.constants,config.fields);
    const byKey = new Map(available.map(field => [field.key,field]));
    const ordered = config.fields.map(key => byKey.get(key)).filter(Boolean).concat(available.filter(field => !config.fields.includes(field.key)));
    const signature = JSON.stringify([ordered,config.fields]);
    if (signature === fieldSignature) return;
    fieldSignature = signature;
    const fragment = root.createDocumentFragment();
    for (const field of ordered) {
      const item = root.createElement('div'); item.className = 'output-field';
      const label = root.createElement('label'), checkbox = root.createElement('input');
      checkbox.type = 'checkbox'; checkbox.dataset.outputKey = field.key; checkbox.checked = config.fields.includes(field.key);
      checkbox.addEventListener('change', () => {
        config.fields = checkbox.checked ? [...config.fields,field.key] : config.fields.filter(key => key !== field.key);
        refresh(); focusField(field.key);
      });
      label.append(checkbox,root.createTextNode(field.label)); item.append(label);
      for (const [direction,delta,labelText] of [['up',-1,'上移'],['down',1,'下移']]) {
        const button = root.createElement('button'); button.className = 'button'; button.type = 'button'; button.dataset.fieldMove = direction;
        const index = config.fields.indexOf(field.key);
        button.textContent = labelText; button.setAttribute('aria-label', `${labelText}${field.label}`);
        button.disabled = index < 0 || index + delta < 0 || index + delta >= config.fields.length;
        button.addEventListener('click', () => {
          const target = index + delta;
          [config.fields[index],config.fields[target]] = [config.fields[target],config.fields[index]];
          refresh(); focusField(field.key,direction);
        }); item.append(button);
      }
      fragment.append(item);
    }
    list.replaceChildren(fragment);
  }
  function build() {
    readConfig();
    const rows = normalizeOutputRows(getRows(config.type),config.type);
    renderFields(rows);
    return buildOutput(rows,config,getSpeedMin(config.type));
  }
  function refresh() {
    if (!config || !dialog.open) return;
    try {
      const result = build(); preview.value = result.content; execute.disabled = !result.count || pendingGeneration === generation;
      status.textContent = result.count ? `独立筛选 ${result.total} 条，本次输出 ${result.count} 条。预览与实际内容一致。` : '暂无可输出结果，请检查结果、筛选与行范围';
    } catch (error) { preview.value = ''; execute.disabled = true; status.textContent = error.message; }
  }
  for (const button of root.querySelectorAll('[data-output-type]')) button.addEventListener('click', () => {
    const type = button.dataset.outputType, kind = button.dataset.outputKind, key = `${type}:${kind}`;
    if (!configs.has(key)) configs.set(key,createOutputConfig(type,kind));
    config = configs.get(key); fieldSignature = ''; generation++;
    dialog.querySelector('h2').textContent = `${({scan:'官方扫描',detail:'官方详细',nsb:'非标'})[type]} · ${kind === 'copy' ? '批量复制' : '文件导出'}`;
    for (const element of dialog.querySelectorAll('[data-export-only]')) element.hidden = kind !== 'export';
    for (const name of inputNames) { if (name === 'brackets') form.elements[name].checked = config[name]; else form.elements[name].value = config[name]; }
    execute.textContent = kind === 'copy' ? '复制文本' : '导出文件'; dialog.showModal(); refresh();
  });
  form.addEventListener('submit',event => event.preventDefault());
  form.addEventListener('input',event => {
    if (event.target.name === 'separator') form.elements.separatorPreset.value = 'custom';
    refresh();
  });
  form.addEventListener('change',event => {
    if (event.target.name === 'separatorPreset' && event.target.value !== 'custom') form.elements.separator.value = event.target.value === 'space' ? ' ' : event.target.value;
    if (event.target.name === 'format') config.fields = defaultOutputFields(config.type,event.target.value);
    refresh();
  });
  for (const button of dialog.querySelectorAll('[data-output-preset]')) button.addEventListener('click', () => {
    const rows = normalizeOutputRows(getRows(config.type),config.type);
    config.fields = button.dataset.outputPreset === 'all' ? availableOutputFields(rows,config.constants).map(field => field.key)
      : button.dataset.outputPreset === 'ipport' ? ['ipport'] : defaultOutputFields(config.type,config.format);
    refresh();
  });
  dialog.querySelector('[data-output-close]').addEventListener('click', () => dialog.close());
  execute.addEventListener('click', async () => {
    if (pendingGeneration === generation) return;
    const operation = generation, kind = config.kind;
    try {
      const result = build(); preview.value = result.content;
      if (!result.count) { refresh(); return; }
      execute.disabled = true; pendingGeneration = operation;
      if (kind === 'copy') await copyText(result.content,{navigator:window.navigator,document:root});
      else window.downloadFile(result.content,config.name,config.format);
      if (operation === generation) status.textContent = kind === 'copy' ? `已复制 ${result.count} 条` : `已请求导出 ${result.count} 条，请完成浏览器/系统保存`;
    } catch (error) { if (operation === generation) status.textContent = error.message; }
    finally {
      if (pendingGeneration === operation) pendingGeneration = undefined;
      if (operation === generation) execute.disabled = !preview.value;
    }
  });
  return {onMessage() { if (dialog.open && !frame) frame = window.requestAnimationFrame(() => {frame = undefined; refresh();}); }};
}
