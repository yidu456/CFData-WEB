import { createTargetStore } from './targets.js';

export function mountTargets({root, storage}) {
  const window = root.defaultView;
  const store = createTargetStore({storage,crypto:window.crypto,origin:window.location.origin,createID:()=>window.crypto?.randomUUID?.() || `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`});
  const form = root.querySelector('[data-target-form]'), select = root.querySelector('[data-target-select]');
  const status = root.querySelector('[data-target-status]'), summary = root.querySelector('[data-target-summary]');
  const remove = root.querySelector('[data-target-delete]'), save = root.querySelector('[data-target-save]');
  const dialog = root.querySelector('[data-target-delete-dialog]'); let selected = '', revision = 0, deleting;
  const fields = ['provider','name','repo','branch','path','message','format','host','mode'];
  const defaults = {provider:'github',name:'',repo:'',branch:'main',path:'results/ip.txt',message:'update cfdata results',format:'txt',host:'',mode:'overwrite'};
  function providerChanged() {
    for (const field of root.querySelectorAll('[data-target-provider]')) {
      field.hidden = field.dataset.targetProvider !== form.elements.provider.value;
      for (const input of field.querySelectorAll('input,select')) input.disabled = field.hidden;
    }
  }
  function renderList() {
    select.replaceChildren(new Option('新建目标',''));
    for (const target of store.list()) select.append(new Option(`${target.name} · ${target.provider} · ${target.provider === 'github' ? target.repo : target.host}`,target.id));
    select.value = selected;
    root.querySelector('[data-target-storage-warning]').textContent = store.warning || '目标存储于当前浏览器，不跨设备同步；旧账号原数据完整保留。';
  }
  function load(id) {
    revision++; selected = id;
    const target = store.list().find(item => item.id === id);
    for (const name of fields) form.elements[name].value = target?.[name] || defaults[name];
    form.elements.secret.value = ''; form.elements.remember.checked = !!target?.credential; form.elements.forget.checked = false;
    form.elements.provider.disabled = !!target;
    summary.textContent = target ? `${target.provider === 'github' ? `${target.repo} / ${target.branch} / ${target.path}` : `${target.host} / ${target.mode === 'append' ? '追加' : '覆盖'}`}；${({session:'凭据本页可用',stored:'已存加密凭据，执行时解密',missing:'未配置凭据'})[target.credentialState]}` : '选择已保存目标编辑，或填写新目标。此处只保存配置，不执行上传。';
    remove.disabled = !target; save.disabled = false; status.textContent = '';
    providerChanged(); renderList();
  }
  select.addEventListener('change',()=>load(select.value));
  form.elements.provider.addEventListener('change',providerChanged);
  root.querySelector('[data-target-new]').addEventListener('click',()=>{load('');form.elements.name.focus();});
  root.querySelector('[data-target-cancel]').addEventListener('click',()=>load(selected));
  form.addEventListener('submit',async event => {
    event.preventDefault(); if (save.disabled) return;
    const operation = revision, id = selected;
    const values = Object.fromEntries(new FormData(form));
    values.provider = form.elements.provider.value; values.id = id || undefined;
    values.remember = form.elements.remember.checked; values.forget = form.elements.forget.checked;
    save.disabled = true;
    try {
      const target = await store.save(values);
      if (operation !== revision) { renderList(); return; }
      load(target.id); status.textContent = store.warning || '目标已保存。上传时无需重复填写目的地；密钥未显示在列表中。'; save.focus();
    } catch (error) { if (operation === revision) status.textContent = error.message; }
    finally { if (operation === revision) save.disabled = false; }
  });
  remove.addEventListener('click',()=>{deleting={id:selected,revision};dialog.showModal();});
  root.querySelector('[data-target-delete-cancel]').addEventListener('click',()=>dialog.close());
  root.querySelector('[data-target-delete-ok]').addEventListener('click',()=>{
    dialog.close(); if (!deleting || deleting.revision !== revision || deleting.id !== selected) return;
    try { store.remove(selected);load('');status.textContent=store.warning || '目标已删除；旧版原账号数据未删除。';select.focus(); }
    catch(error){status.textContent=error.message;}
  });
  load(''); return store;
}
