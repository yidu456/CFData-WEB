export const liveFields = ['ip', 'port', 'originalInput', 'tls', 'dc', 'loc', 'region', 'city', 'latency', 'lossRate', 'speed', 'outboundIP', 'ipType', 'asnNumber', 'asnOrg', 'visitScheme', 'tlsVersion', 'sni', 'httpVersion', 'warp', 'gateway', 'rbi', 'kex', 'timestamp'];
const csvFields = { 'IP地址':'ip', '端口号':'port', 'TLS':'tls', '丢包率':'lossRate', '网络延迟':'latency', '下载速度':'speed', '出站IP':'outboundIP', 'IP类型':'ipType', '原始输入':'originalInput', 'OriginalInput':'originalInput', '数据中心':'dc', '源IP位置':'loc', '地区':'region', '城市':'city', 'ASN号码':'asnNumber', 'ASN组织':'asnOrg', '访问协议':'visitScheme', 'TLS版本':'tlsVersion', 'SNI':'sni', 'HTTP版本':'httpVersion', 'WARP':'warp', 'Gateway':'gateway', 'RBI':'rbi', '密钥交换':'kex', '时间戳':'timestamp' };

function integer(value, min, max, name) {
  if (!/^\d+$/.test(String(value)) || !Number.isSafeInteger(Number(value)) || Number(value) < min || Number(value) > max) throw new Error(`${name}范围为 ${min}–${max} 的整数`);
  return Number(value);
}
export function buildNSBRequest(values, input) {
  const fileContent = String(input.fileContent || ''), sourceURL = String(input.sourceURL || '').trim();
  if (!!fileContent.trim() === !!sourceURL) throw new Error('请选择文件或网络 URL，必须二选一');
  if (sourceURL) {
    let url;
    try { url = new URL(sourceURL); } catch { throw new Error('网络 URL 无效'); }
    if (!['http:', 'https:'].includes(url.protocol) || !url.hostname) throw new Error('网络 URL 必须为 HTTP/HTTPS');
  }
  if (fileContent.length > 16 * 1024 * 1024) throw new Error('文件超过 16MiB，请使用 CLI 处理');
  if (!['tcping', 'httping'].includes(values.scanMode)) throw new Error('请选择扫描方式');
  if (!['true', 'false'].includes(values.enableTLS) || !['true', 'false'].includes(values.compact)) throw new Error('TLS/结果格式选择无效');
  return { fileName: String(input.fileName || ''), fileContent, sourceURL, outFile: String(values.outFile || '').trim() || 'ip.csv',
    maxThreads: integer(values.maxThreads, 1, 10000, '并发'), fallbackPort: integer(values.fallbackPort, 1, 65535, '备用端口'),
    delay: integer(values.delay, 0, 60000, '延迟'), resultLimit: integer(values.resultLimit, 1, 1000000, '扫描合格数量'),
    enableTLS: values.enableTLS === 'true', compact: values.compact === 'true', scanMode: values.scanMode, dc: String(values.dc || '').trim(), speedTest: 0, speedURL: 'auto', speedMin: .1, speedLimit: 5, relinkHost: '' };
}
export function createNSBState() { return { rows: [], index: new Map(), headers: [], csvRows: [], csvIndex: new Map(), csvFields: [], status: '', file: '', startedAt: '' }; }
export function buildNSBExecution(values) {
  const speedTest = values.autoSpeed === 'on' ? integer(values.speedTest || '5', 1, 1000, '测速并发') : 0;
  const speedMin = Number(values.speedMin ?? .1), speedLimit = integer(values.speedLimit ?? '5', 0, 1000000, '测速数量');
  if (!Number.isFinite(speedMin) || speedMin <= 0) throw new Error('测速合格速度必须大于0');
  const speedURL = String(values.speedURL || 'auto').trim();
  if (speedURL !== 'auto') {
    let url; try { url = new URL(speedURL.includes('://') ? speedURL : `https://${speedURL}`); } catch { throw new Error('测速网址无效'); }
    if (!['http:', 'https:'].includes(url.protocol) || !url.hostname || url.username || url.password) throw new Error('测速网址须为无账号密码的HTTP/HTTPS地址');
  }
  let relinkHost = values.relinkPreset === 'custom' ? String(values.relinkHost || '').trim() : values.relinkPreset === 'default' || speedTest > 0 ? 'probe.609909.xyz' : '';
  if (relinkHost) {
    let url; try { url = new URL(relinkHost.includes('://') ? relinkHost : `https://${relinkHost}`); } catch { throw new Error('探针网址无效'); }
    if (!['http:', 'https:'].includes(url.protocol) || !url.hostname || url.username || url.password) throw new Error('探针须为无账号密码的HTTP/HTTPS域名');
    relinkHost = url.hostname;
  } else if (values.relinkPreset === 'custom') throw new Error('请填写自定义探针');
  return { speedTest, speedURL, speedMin, speedLimit, relinkHost };
}
export function getNSBResultRows(state) {
  if (!state.headers.length) return state.rows;
  return state.csvRows.map(cells => {
    const row = {};
    state.csvFields.forEach((field, index) => { if (field) row[field] = cells[index] || ''; });
    return row;
  }).filter(row => row.ip && row.port);
}
export function hasNSBPendingRows(state) {
  if (!state.headers.length) return state.rows.some(row => !row.speed || row.speed === '未测速' || row.speed === '-');
  const column = state.csvFields.indexOf('speed');
  return state.csvRows.some(row => !row[column] || row[column] === '未测速' || row[column] === '-');
}
export function buildNSBFollowup(values, state, action, continued) {
  const execution = buildNSBExecution({ ...values, autoSpeed: action === 'speed' ? 'on' : values.autoSpeed });
  const results = getNSBResultRows(state).map(row => ({ ...row, ...(action === 'speed' && !continued ? { speed: '' } : {}) }));
  if (!results.length) throw new Error('没有可执行的非标结果');
  if (action === 'speed' && execution.speedLimit <= 0) throw new Error('测速数量0，当前批量测速关闭');
  if (action === 'relink' && !execution.relinkHost) execution.relinkHost = 'probe.609909.xyz';
  return { results, ...execution, enableTLS: values.enableTLS === 'true', compact: values.compact === 'true', ...(action === 'speed' ? { skipTested: !!continued } : { outFile: String(values.outFile || 'ip.csv'), scanMode: values.scanMode }) };
}
export function reduceNSB(state, { type, data }) {
  if (type === 'nsb_requested') return { ...createNSBState(), startedAt: data?.startedAt || '' };
  if (type === 'background_task_enabled' && data?.mode === 'nsb') return { ...state, startedAt: data.startedAt || state.startedAt };
  if (type === 'background_task_following' && data?.mode === 'nsb' && data.startedAt !== state.startedAt) return { ...createNSBState(), startedAt: data.startedAt || '' };
  if (type === 'nsb_followup_requested') {
    if (data.action === 'speed' && !data.continued) {
      const speedColumn = state.headers.indexOf('下载速度');
      if (speedColumn >= 0) for (const cells of state.csvRows) cells[speedColumn] = '';
      for (const row of state.rows) row.speed = '';
    }
    return { ...state, status: '' };
  }
  if (type === 'nsb_scan_result') {
    if (!data || typeof data.ip !== 'string' || !data.ip || !/^\d+$/.test(String(data.port)) || Number(data.port) < 1 || Number(data.port) > 65535) return state;
    const row = {};
    for (const key of liveFields) row[key] = typeof data[key] === 'string' ? data[key] : '';
    row.port = String(data.port); row.speedQualified = data.speedQualified === true;
    if (state.headers.length) {
      const speedColumn = state.csvFields.indexOf('speed');
      const cells = state.csvRows[state.csvIndex.get(`${row.ip}:${row.port}`)];
      if (cells && speedColumn >= 0 && typeof data.speed === 'string') cells[speedColumn] = data.speed;
    }
    const key = `${row.ip}:${row.port}`, index = state.index.get(key);
    if (index === undefined) { state.index.set(key, state.rows.length); state.rows.push(row); } else state.rows[index] = row;
    return { ...state };
  }
  if (type === 'nsb_scan_sorted') {
    if (!Array.isArray(data)) return state;
    let next = { ...state, rows: [], index: new Map() };
    for (const row of data) next = reduceNSB(next, { type: 'nsb_scan_result', data: row });
    return next;
  }
  if (type === 'nsb_csv_complete') {
    if (!Array.isArray(data?.headers) || !Array.isArray(data.rows)) return state;
    const headers = data.headers.map(String), rows = data.rows.filter(Array.isArray).map(row => row.map(cell => String(cell ?? '')));
    const fields = headers.map(header => csvFields[header] || '');
    const ipColumn = fields.indexOf('ip'), portColumn = fields.indexOf('port'), csvIndex = new Map();
    rows.forEach((row, index) => { if (ipColumn >= 0 && portColumn >= 0) csvIndex.set(`${row[ipColumn]}:${row[portColumn]}`, index); });
    return { ...state, headers, csvRows: rows, csvIndex, csvFields: fields, status: typeof data.status === 'string' ? data.status : 'complete', file: typeof data.file === 'string' ? data.file : '' };
  }
  return state;
}
