export function buildSpeedConfig(values) {
  const port = Number(values.port), speedMin = Number(values.speedMin), speedLimit = Number(values.speedLimit);
  if (!Number.isInteger(port) || port < 1 || port > 65535) throw new Error('测速端口无效');
  if (!Number.isFinite(speedMin) || speedMin <= 0) throw new Error('合格速度必须大于 0');
  if (!/^\d+$/.test(String(values.speedLimit)) || !Number.isSafeInteger(speedLimit)) throw new Error('测速结果数量必须为非负整数');
  const url = String(values.url || '').trim();
  if (url !== 'auto') {
    let parsed;
    try { parsed = new URL(url.includes('://') ? url : `https://${url}`); }
    catch { throw new Error('测速网址无效'); }
    if (!['http:', 'https:'].includes(parsed.protocol) || !parsed.hostname || parsed.username || parsed.password) throw new Error('测速网址须为无账号密码的 HTTP/HTTPS 地址');
  }
  return { port, url, speedMin, speedLimit };
}
export function speedStatus(text, min) {
  if (!text || text === '未测速') return 'untested';
  const match = /^\s*(\d+(?:\.\d+)?)\s*MB\/s\s*$/i.exec(text);
  if (!match || Number(match[1]) === 0) return 'failed';
  return Number(match[1]) >= min ? 'qualified' : 'unqualified';
}
export function buildSpeedRequest(config, rows, ip) {
  const row = rows.find(row => row.IP === ip);
  if (!row) throw new Error('只能对已有详细结果测速');
  return { ip, port: config.port, resultPort: row.Port, url: config.url };
}
export function buildBatchRequest(config, rows, continued) {
  if (config.speedLimit <= 0) throw new Error('测速已关闭，请将结果数量设为大于 0');
  if (!rows.length) throw new Error('请先取得详细测试结果');
  const results = rows.map(row => {
    const result = {};
    for (const key of ['IP', 'Port', 'DataCenter', 'DCCountry', 'Region', 'City', 'MinLatency', 'MaxLatency', 'AvgLatency', 'LossRate', 'Speed']) result[key] = row[key] ?? (['Port', 'MinLatency', 'MaxLatency', 'AvgLatency', 'LossRate'].includes(key) ? 0 : '');
    return result;
  });
  return { ...config, results, skipTested: continued };
}
