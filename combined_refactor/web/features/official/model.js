export const officialPorts = [443, 2053, 2083, 2087, 2096, 8443, 80, 8080, 8880, 2052, 2082, 2086, 2095];

function integer(value, min, max, label) {
  if (!/^\d+$/.test(String(value))) throw new Error(`${label}必须是整数`);
  const number = Number(value);
  if (!Number.isSafeInteger(number) || number < min || number > max) throw new Error(`${label}范围为 ${min}–${max}`);
  return number;
}
export function buildScanRequest(values) {
  const ipType = integer(values.ipType, 4, 6, '地址族');
  if (![4, 6].includes(ipType)) throw new Error('仅支持 IPv4 或 IPv6');
  const port = integer(values.port, 1, 65535, '端口');
  if (!officialPorts.includes(port)) throw new Error('请选择官方支持的端口');
  if (!['tcping', 'httping'].includes(values.scanMode)) throw new Error('请选择 TCPing 或 HTTPing');
  return { ipType, port, threads: integer(values.threads, 1, 10000, '扫描并发'), delay: integer(values.delay, 0, 60000, '合格延迟'), scanMode: values.scanMode, autoSpeed: false };
}

export function createOfficialState() { return { rows: [], rowIndex: new Map(), centers: [], selectedDC: '', startedAt: '' }; }

export function reduceOfficial(state, { type, data }) {
  if (type === 'scan_requested') return { ...createOfficialState(), startedAt: data?.startedAt || '' };
  if (type === 'background_task_enabled') return { ...state, startedAt: data?.startedAt || state.startedAt };
  if (type === 'background_task_following' && data?.mode === 'official') {
    return data.startedAt && data.startedAt !== state.startedAt ? { ...createOfficialState(), startedAt: data.startedAt } : state;
  }
  if (type === 'select_dc') return state.centers.some(center => center.DataCenter === data) ? { ...state, selectedDC: data } : state;
  if (type === 'scan_result') {
    if (!data || typeof data.IP !== 'string' || !data.IP || !Number.isInteger(data.Port) || data.Port < 1 || data.Port > 65535) return state;
    const row = {};
    for (const key of ['IP', 'DataCenter', 'DCCountry', 'Region', 'City', 'LatencyStr']) row[key] = typeof data[key] === 'string' ? data[key] : '';
    row.Port = data.Port;
    row.latencyMS = Number.isFinite(data.TCPDuration) && data.TCPDuration >= 0 ? data.TCPDuration / 1000000 : null;
    const key = `${row.IP}:${row.Port}`;
    const index = state.rowIndex.get(key);
    // Keep the collection append-only/indexed so a large IPv6 scan stays linear.
    if (index === undefined) { state.rowIndex.set(key, state.rows.length); state.rows.push(row); }
    else state.rows[index] = row;
    return { ...state };
  }
  if (type === 'scan_complete_wait_dc') {
    if (!Array.isArray(data)) return state;
    const centers = data.filter(item => typeof item?.DataCenter === 'string' && item.DataCenter).map(item => ({
      DataCenter: item.DataCenter, DCCountry: typeof item.DCCountry === 'string' ? item.DCCountry : '', City: typeof item.City === 'string' ? item.City : '',
      IPCount: Number.isInteger(item.IPCount) && item.IPCount >= 0 ? item.IPCount : 0,
      MinLatency: Number.isFinite(item.MinLatency) && item.MinLatency >= 0 ? item.MinLatency : null,
    })).sort((a, b) => (a.MinLatency ?? Infinity) - (b.MinLatency ?? Infinity) || a.DataCenter.localeCompare(b.DataCenter));
    return { ...state, centers, selectedDC: centers.some(item => item.DataCenter === state.selectedDC) ? state.selectedDC : '' };
  }
  return state;
}

export function visibleScanRows(state) {
  return state.rows.slice(-100).sort((a, b) => (a.latencyMS ?? Infinity) - (b.latencyMS ?? Infinity));
}
