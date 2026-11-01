import { encryptSecret, decryptSecret } from './secrets.js';
const storageKey = 'cfdata-publishing-targets';
const clean = value => String(value || '').trim();

export function validateTarget(values, id) {
  const target = {id,name:clean(values.name),provider:values.provider};
  if (!id || !target.name || target.name.length > 120) throw new Error('目标名称不能为空，最多120个字符');
  if (values.provider === 'github') {
    const repo = clean(values.repo), branch = clean(values.branch) || 'main', path = clean(values.path), format = values.format || 'txt';
    if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repo)) throw new Error('仓库格式为 owner/repo');
    if (!path || path.startsWith('/') || path.split('/').some(part => !part || part === '..' || part === '.') || /[\\\x00-\x1f]/.test(path)) throw new Error('请填写有效的仓库相对文件路径');
    if (/[\x00-\x20~^:?*\[\\]/.test(branch) || branch.includes('..') || branch.startsWith('/') || branch.endsWith('/') || branch.includes('//') || branch.endsWith('.lock')) throw new Error('分支名称无效');
    if (!['txt','csv'].includes(format)) throw new Error('结果格式仅支持TXT/CSV');
    return {...target,repo,branch,path,message:clean(values.message) || 'update cfdata results',format};
  }
  if (values.provider === 'edgetunnel') {
    let url; const host = clean(values.host);
    try { url = new URL(host.includes('://') ? host : `https://${host}`); } catch { throw new Error('edgetunnel 域名无效'); }
    if (!['http:','https:'].includes(url.protocol) || !url.hostname || url.username || url.password || url.search || url.hash || url.pathname !== '/') throw new Error('edgetunnel 仅填写HTTP/HTTPS服务域名，不含账号、路径或查询参数');
    if (!['append','overwrite'].includes(values.mode)) throw new Error('请选择追加或覆盖');
    return {...target,host:url.origin,mode:values.mode,format:'txt'};
  }
  throw new Error('该发布平台尚未接入');
}
function storedRecord(target) {
  const result = validateTarget(target,target.id);
  if (typeof target.legacySource === 'string') result.legacySource = target.legacySource;
  if (target.credential && typeof target.credential.value === 'string' && ['v1','legacy-github','legacy-edgetunnel'].includes(target.credential.scheme)) {
    result.credential = {scheme:target.credential.scheme,context:String(target.credential.context || ''),value:target.credential.value};
  }
  return result;
}
export function saveTargets(storage, targets, imported = []) {
  try { storage.setItem(storageKey,JSON.stringify({version:1,targets:targets.map(storedRecord),imported})); return ''; }
  catch { return '浏览器存储不可用或空间不足，目标仅保留在本页；刷新后未保存的更改会丢失'; }
}
export function loadTargets(storage, createID) {
  let targets = [], imported = [], warning = '', writable = true;
  const credentials = new Map();
  try {
    const raw = storage?.getItem(storageKey);
    if (raw) {
      const data = JSON.parse(raw);
      if (data.version !== 1 || !Array.isArray(data.targets) || !Array.isArray(data.imported)) throw Error('schema');
      targets = data.targets.map(storedRecord); imported = data.imported.filter(value => typeof value === 'string');
      if (new Set(targets.map(target => target.id)).size !== targets.length) throw Error('duplicate');
    }
  } catch { warning = '目标存储读取失败，原数据未改动；本页更改无法持久保存'; writable = false; targets = []; imported = []; }
  let changed = false;
  for (const provider of ['github','edgetunnel']) {
    try {
      const raw = storage?.getItem(`cfdata-${provider}-accounts`);
      if (!raw) continue;
      const accounts = JSON.parse(raw); if (!Array.isArray(accounts)) throw Error('array');
      accounts.forEach((account,index) => {
        if (!account || typeof account !== 'object') return;
        const identity = JSON.stringify([provider,index,account.name || '',account.repo || account.host || '']);
        const plain = provider === 'github' ? account.token : account.password;
        if (imported.includes(identity)) {
          const target = targets.find(item => item.legacySource === identity);
          if (target && plain) credentials.set(target.id,String(plain));
          return;
        }
        const id = createID(), name = account.name || account.repo || account.host || '旧发布目标';
        let target;
        try { target = validateTarget({...account,provider,name,format:account.path?.endsWith('.csv') ? 'csv' : 'txt',path:account.path || 'results/ip.txt',mode:account.mode || 'overwrite'},id); }
        catch { warning = '部分旧账号目的地无效，未导入；原账号仍完整保留在旧版'; return; }
        target.legacySource = identity;
        const cipher = provider === 'github' ? account.tokenCipher : account.passwordCipher;
        if (cipher) target.credential = {scheme:`legacy-${provider}`,context:account.name || account.repo || account.host || '',value:String(cipher)};
        if (plain) credentials.set(id,String(plain));
        targets.push(target); imported.push(identity); changed = true;
      });
    } catch { warning = '旧账号存储读取失败，原数据未改动；可继续使用旧版检查配置'; }
  }
  if (changed && writable) warning = saveTargets(storage,targets,imported) || warning;
  return {targets,imported,credentials,warning,writable};
}

export function createTargetStore({storage,crypto,origin,createID}) {
  const loaded = loadTargets(storage,createID), session = loaded.credentials;
  let targets = loaded.targets, warning = loaded.warning;
  const persist = () => {
    const message = loaded.writable ? saveTargets(storage,targets,loaded.imported) : '目标存储无法保存，原数据未改动；目标仅保留在本页';
    if (message) warning = message;
  };
  return {
    get warning() { return warning; },
    list: () => targets.map(target => ({...target,credential:target.credential && {...target.credential},credentialState:session.has(target.id) ? 'session' : target.credential ? 'stored' : 'missing'})),
    async save(values) {
      const existing = values.id ? targets.find(target => target.id === values.id) : undefined;
      if (values.id && !existing) throw new Error('目标已删除，请重新选择');
      const id = existing?.id || createID(), target = validateTarget(values,id);
      let secret = clean(values.secret), cipher = existing?.credential;
      if (existing?.legacySource && !values.forget && !secret) target.legacySource = existing.legacySource;
      if (values.forget) { secret = ''; cipher = undefined; }
      else if (secret) {
        if (values.remember) {
          let replacement;
          try { replacement = await encryptSecret(secret,id,origin,crypto); } catch { replacement = null; }
          if (replacement) cipher = replacement;
          else {
            if (existing?.legacySource) target.legacySource = existing.legacySource;
            warning = '浏览器不支持加密或加密失败，新凭据仅本页可用；原有持久凭据未改动，刷新后仍使用旧凭据';
          }
        } else cipher = undefined;
      }
      // Do not resurrect a target deleted while native crypto was pending.
      if (existing && !targets.includes(existing)) throw new Error('目标已改变，请重新选择后保存');
      if (cipher) target.credential = cipher;
      if (values.forget) session.delete(id); else if (secret) session.set(id,secret);
      targets = existing ? targets.map(item => item.id === id ? target : item) : [...targets,target];
      persist(); return {...target,credentialState:session.has(id) ? 'session' : cipher ? 'stored' : 'missing'};
    },
    remove(id) {
      if (!targets.some(target => target.id === id)) throw new Error('目标不存在或已删除');
      targets = targets.filter(target => target.id !== id); session.delete(id); persist();
    },
    async credential(id) {
      const target = targets.find(target => target.id === id);
      if (!target) return '';
      if (session.has(id)) return session.get(id);
      const secret = await decryptSecret(target.credential,id,origin,crypto);
      if (secret && targets.includes(target)) { session.set(id,secret); return secret; }
      return '';
    },
  };
}
