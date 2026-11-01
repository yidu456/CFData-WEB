export async function copyText(text, {navigator, document}) {
  try {
    if (navigator?.clipboard?.writeText) { await navigator.clipboard.writeText(String(text)); return; }
  } catch {
    // Insecure contexts and denied clipboard permission can still allow a user-triggered copy.
  }
  const active = document.activeElement, selection = document.getSelection?.();
  const ranges = selection ? Array.from({length:selection.rangeCount},(_,i) => selection.getRangeAt(i).cloneRange()) : [];
  const textarea = document.createElement('textarea');
  textarea.value = String(text); textarea.style.position = 'fixed'; textarea.style.opacity = '0';
  const parent = document.querySelector?.('dialog[open]') || document.body;
  try {
    parent.append(textarea); textarea.select();
    if (!document.execCommand('copy')) throw new Error('复制失败，请在预览中手动选择并复制');
  } catch {
    throw new Error('复制失败，请在预览中手动选择并复制');
  } finally {
    textarea.remove(); active?.focus();
    if (selection) { selection.removeAllRanges(); for (const range of ranges) selection.addRange(range); }
  }
}
export function downloadText(content, name, format, environment) {
  if (!['txt','csv'].includes(format)) throw new Error('导出格式无效');
  const timestamp = (environment.now || new Date()).toISOString().replace(/[-:T]/g,'').split('.')[0];
  const base = String(name || 'cfdata-results').replace(/[\\/:*?"<>|\x00-\x1f]/g,'_').trim().replace(/\.(txt|csv)$/i,'') || 'cfdata-results';
  const filename = `${base}_${timestamp}.${format}`;
  const plain = String(content || '').replace(/^\uFEFF/,'');
  if (environment.CFDataAndroid?.saveTextFile) { environment.CFDataAndroid.saveTextFile(filename,plain); return; }
  const blob = new environment.Blob(['\uFEFF' + plain], {type:format === 'csv' ? 'text/csv;charset=utf-8' : 'text/plain;charset=utf-8'});
  const url = environment.URL.createObjectURL(blob), link = environment.document.createElement('a');
  try { link.href = url; link.download = filename; link.click(); }
  finally { link.remove(); environment.setTimeout(() => environment.URL.revokeObjectURL(url),30000); }
}
