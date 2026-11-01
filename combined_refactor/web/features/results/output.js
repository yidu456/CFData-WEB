import { browseResultRows } from './browser.js';

export const outputFields = [
  ['ipport','IP:PORT'], ['ip','IP地址'], ['port','端口号'], ['tls','TLS'], ['lossRate','丢包率'], ['latency','网络延迟'], ['speed','下载速度'],
  ['outboundIP','出站IP'], ['ipType','IP类型'], ['originalInput','原始输入'], ['dc','数据中心'], ['dcCountry','落地区域'], ['loc','源IP位置'],
  ['region','地区'], ['city','城市'], ['asnNumber','ASN号码'], ['asnOrg','ASN组织'], ['visitScheme','访问协议'], ['tlsVersion','TLS版本'],
  ['sni','SNI'], ['httpVersion','HTTP版本'], ['warp','WARP'], ['gateway','Gateway'], ['rbi','RBI'], ['kex','密钥交换'], ['timestamp','时间戳'],
].map(([key,label]) => ({key,label}));

export function defaultOutputFields(type, format = 'txt') {
  if (format === 'txt') return type === 'nsb' ? ['ipport','dc','loc'] : ['ipport','dc','dcCountry'];
  if (type === 'nsb') return ['ip','port','tls','lossRate','latency','speed','outboundIP','ipType','originalInput','dc','loc','region','city','asnNumber','asnOrg'];
  return ['ip','port','latency',...(type === 'detail' ? ['speed'] : []),'dc','dcCountry','region','city'];
}
export function createOutputConfig(type, kind) {
  return { type, kind, format:'txt', fields:defaultOutputFields(type), constants:[], custom:'', start:1, end:20, ipType:'all', qualified:'all',
    latency:0, loss:100, separator:'-', separatorPreset:'-', brackets:true, name:`Cloudflare_${type === 'nsb' ? 'NSB' : type === 'detail' ? 'SpeedTest' : 'Scan'}_Results` };
}
export function normalizeOutputRows(rows, type) {
  if (type === 'nsb') return rows.map(row => ({...row}));
  return rows.map(row => {
    const average = Number.isFinite(row.AvgLatency) ? row.AvgLatency / 1000000 : row.avgMS;
    return { ...row, ip:row.IP || '', port:String(row.Port ?? ''), dc:row.DataCenter || '', dcCountry:row.DCCountry || '', region:row.Region || '', city:row.City || '',
      originalInput:row.OriginalInput || row.originalInput || '',
      latency:type === 'detail' ? Number.isFinite(average) ? `${average.toFixed(0)}ms` : '' : row.LatencyStr || (Number.isFinite(row.latencyMS) ? `${row.latencyMS.toFixed(0)}ms` : ''),
      lossRate:Number.isFinite(row.LossRate) ? `${(row.LossRate * 100).toFixed(0)}%` : '', speed:row.Speed || '' };
  });
}
export function parseCustomFields(spec) {
  const used = new Set([...outputFields.map(field => field.key), '__proto__', 'constructor', 'prototype']);
  return String(spec || '').split(',').filter(part => part.trim()).map(part => {
    let item = part.trim(), key = '';
    const colon = item.indexOf(':'), equal = item.indexOf('=');
    if (equal >= 0 && equal < colon) { key = item.slice(0,equal).trim().toLowerCase(); item = item.slice(equal+1).trim(); }
    const split = item.indexOf(':'), label = item.slice(0,split).trim(), value = item.slice(split+1).trim();
    if (split < 1 || !label || !value) throw new Error('常量格式为 标题:内容 或 key=标题:内容，逗号分隔');
    const base = (key || label.toLowerCase()).replace(/[\s,:#]/g,'_');
    key = base; let suffix = 1;
    while (used.has(key)) key = base + suffix++;
    used.add(key); return {key,label,value};
  });
}
export function availableOutputFields(rows, constants = [], selected = []) {
  return outputFields.filter(field => field.key === 'ipport' || selected.includes(field.key) || rows.some(row => String(row[field.key] ?? '').trim())).concat(constants);
}
export function formatIPPort(ip, port, brackets = true) {
  const address = String(ip || '').replace(/^\[(.*)\]$/, '$1');
  return `${brackets && address.includes(':') ? `[${address}]` : address}:${port ?? ''}`;
}
const csvCell = value => /[",\r\n]/.test(String(value ?? '')) ? `"${String(value).replaceAll('"','""')}"` : String(value ?? '');

export function buildOutput(rows, config, speedMin = .1) {
  const start = Number(config.start), end = Number(config.end);
  if (!Number.isSafeInteger(start) || start < 1 || !Number.isSafeInteger(end) || end < start) throw new Error('开始行/结束行必须为正整数，结束行不小于开始行');
  if (!['txt','csv'].includes(config.format)) throw new Error('输出格式无效');
  const definitions = new Map(outputFields.concat(config.constants || []).map(field => [field.key,field]));
  const fields = [...new Set(config.fields)];
  if (fields.some(key => !definitions.has(key)) || config.format === 'csv' && !fields.length) throw new Error('请选择有效输出字段');
  const selection = browseResultRows(rows, {ipType:config.ipType,qualified:config.qualified,speedMin,
    latency:config.kind === 'export' ? config.latency : 0, loss:config.kind === 'export' ? config.loss : 100, sort:[], allIndexes:true}, 'nsb');
  const selected = selection.indexes.slice(start-1,end).map(index => {
    const row = {...rows[index],ipport:formatIPPort(rows[index].ip,rows[index].port,config.brackets)};
    for (const field of config.constants || []) row[field.key] = field.value;
    return row;
  });
  let content = '';
  if (selected.length && config.format === 'txt') content = selected.map(row => {
    const extras = fields.filter(key => key !== 'ipport').map(key => row[key]).filter(value => String(value ?? '').trim());
    return row.ipport + (extras.length ? '#' + extras.join(config.separator) : '');
  }).join('\n') + '\n';
  if (selected.length && config.format === 'csv') content = fields.map(key => csvCell(definitions.get(key).label)).join(',') + '\n' +
    selected.map(row => fields.map(key => csvCell(row[key])).join(',')).join('\n') + '\n';
  return {content,total:selection.total,count:selected.length,fields};
}
