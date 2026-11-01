export const liveFields = ['ip', 'port', 'originalInput', 'tls', 'dc', 'loc', 'region', 'city', 'latency', 'lossRate', 'speed', 'outboundIP', 'ipType', 'asnNumber', 'asnOrg', 'visitScheme', 'tlsVersion', 'sni', 'httpVersion', 'warp', 'gateway', 'rbi', 'kex', 'timestamp'];

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
export function createNSBState() { return { rows: [], index: new Map(), headers: [], csvRows: [], status: '', file: '', startedAt: '' }; }
export function reduceNSB(state, { type, data }) {
  if (type === 'nsb_requested') return { ...createNSBState(), startedAt: data?.startedAt || '' };
  if (type === 'background_task_enabled' && data?.mode === 'nsb') return { ...state, startedAt: data.startedAt || state.startedAt };
  if (type === 'background_task_following' && data?.mode === 'nsb' && data.startedAt !== state.startedAt) return { ...createNSBState(), startedAt: data.startedAt || '' };
  if (type === 'nsb_scan_result') {
    if (!data || typeof data.ip !== 'string' || !data.ip || !/^\d+$/.test(String(data.port)) || Number(data.port) < 1 || Number(data.port) > 65535) return state;
    const row = {};
    for (const key of liveFields) row[key] = typeof data[key] === 'string' ? data[key] : '';
    row.port = String(data.port); row.speedQualified = data.speedQualified === true;
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
    return { ...state, headers: data.headers.map(String), csvRows: data.rows.filter(Array.isArray).map(row => row.map(cell => String(cell ?? ''))), status: typeof data.status === 'string' ? data.status : 'complete', file: typeof data.file === 'string' ? data.file : '' };
  }
  return state;
}
