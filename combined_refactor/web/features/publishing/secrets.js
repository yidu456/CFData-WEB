const currentSalt = 'cfdata-publishing-target-v1';
const legacyGitHubSalt = 'cfdata-github-token-v1';
const encode = bytes => btoa(String.fromCharCode(...bytes));
const decode = text => Uint8Array.from(atob(text),char => char.charCodeAt(0));
async function keyFor(context, origin, salt, crypto) {
  const encoder = new TextEncoder();
  const material = await crypto.subtle.importKey('raw',encoder.encode(`${origin}|${context}|${salt}`),'PBKDF2',false,['deriveKey']);
  return crypto.subtle.deriveKey({name:'PBKDF2',salt:encoder.encode(salt),iterations:120000,hash:'SHA-256'},material,{name:'AES-GCM',length:256},false,['encrypt','decrypt']);
}
export async function encryptSecret(secret, id, origin, crypto) {
  if (!crypto?.subtle) return null;
  const key = await keyFor(id,origin,currentSalt,crypto), iv = crypto.getRandomValues(new Uint8Array(12));
  const encrypted = await crypto.subtle.encrypt({name:'AES-GCM',iv},key,new TextEncoder().encode(secret));
  return {scheme:'v1',context:id,value:encode(iv) + '.' + encode(new Uint8Array(encrypted))};
}
export async function decryptSecret(stored, id, origin, crypto) {
  if (!stored || !crypto?.subtle) return '';
  const schemes = {v1:[currentSalt,'.'], 'legacy-github':[legacyGitHubSalt,'.'], 'legacy-edgetunnel':['cfdata-edgetunnel-token-v1',':']};
  const scheme = schemes[stored.scheme];
  if (!scheme || stored.scheme === 'v1' && stored.context !== id) return '';
  try {
    const parts = stored.value.split(scheme[1]);
    if (parts.length !== 2) return '';
    const key = await keyFor(stored.context,origin,scheme[0],crypto);
    const decrypted = await crypto.subtle.decrypt({name:'AES-GCM',iv:decode(parts[0])},key,decode(parts[1]));
    return new TextDecoder().decode(decrypted);
  } catch { return ''; }
}
