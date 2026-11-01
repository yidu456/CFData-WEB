const numeric = value => typeof value === 'number' && Number.isFinite(value) ? value : null;
const parsed = value => { const match = /^\s*(\d+(?:\.\d+)?)\s*(?:ms|%|MB\/s)?\s*$/i.exec(String(value ?? '')); return match ? Number(match[1]) : null; };

export function resultMetrics(row, type, headers = []) {
  if (type === 'nsb') {
    const get = key => Array.isArray(row) ? row[headers.indexOf(key)] : row[({ 'IP地址':'ip','网络延迟':'latency','丢包率':'lossRate','下载速度':'speed' })[key]];
    return { ip: get('IP地址') || '', latency: [parsed(get('网络延迟'))], loss: parsed(get('丢包率')), speed: get('下载速度') || '' };
  }
  return { ip: row.IP || '', latency: type === 'scan' ? [numeric(row.latencyMS)] : [numeric(row.minMS), numeric(row.maxMS), numeric(row.avgMS)], loss: type === 'scan' ? null : numeric(row.lossPercent), speed: row.Speed || '' };
}
function speedValue(text) {
  const match = /^\s*(\d+(?:\.\d+)?)\s*MB\/s\s*$/i.exec(text);
  if (match) return Number(match[1]);
  return !text || text === '未测速' || text === '-' ? -1 : -2;
}
export function browseResultRows(rows, options = {}, type = 'detail', headers = []) {
  const speedMin = Number(options.speedMin) || .1, latency = Number(options.latency) || 0;
  const loss = options.loss === undefined ? 100 : Number(options.loss);
  const priority = [...new Set(options.sort || ['latency'])].filter(value => ['loss','latency','speed'].includes(value));
  const entries = [];
  rows.forEach((row, index) => {
    const metrics = resultMetrics(row, type, headers), speed = speedValue(metrics.speed);
    if (options.ipType === '4' && String(metrics.ip).includes(':') || options.ipType === '6' && !String(metrics.ip).includes(':')) return;
    if (options.qualified === 'qualified' && speed < speedMin) return;
    if (options.qualified === 'unqualified' && (speed >= speedMin || speed === -1)) return;
    const filterLatency = type === 'detail' ? metrics.latency[2] : metrics.latency[0];
    if (latency > 0 && (filterLatency === null || filterLatency >= latency)) return;
    if (loss < 100 && (metrics.loss === null || metrics.loss >= loss)) return;
    entries.push({ index, ...metrics, speed });
  });
  entries.sort((a,b) => {
    for (const key of priority) {
      if (key === 'speed' && a.speed !== b.speed) return b.speed-a.speed;
      if (key === 'loss') { const delta=(a.loss??Infinity)-(b.loss??Infinity); if (delta && !Number.isNaN(delta)) return delta; }
      if (key === 'latency') for(let i=0;i<a.latency.length;i++){const delta=(a.latency[i]??Infinity)-(b.latency[i]??Infinity);if(delta&&!Number.isNaN(delta))return delta;}
    }
    return a.index-b.index;
  });
  if(options.allIndexes)return{indexes:entries.map(entry=>entry.index),total:entries.length};
  const pages = Math.max(1,Math.ceil(entries.length/100)), page = Math.max(1,Math.min(pages,Math.floor(Number(options.page)||1)));
  return { indexes: entries.slice((page-1)*100,page*100).map(entry=>entry.index), total: entries.length, page, pages };
}
