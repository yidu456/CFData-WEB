import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {webcrypto} from 'node:crypto';
import test from 'node:test';
const sourceURL=async path=>'data:text/javascript;base64,'+Buffer.from(await readFile(new URL(path,import.meta.url))).toString('base64');
const secrets=async()=>import(await sourceURL('../features/publishing/secrets.js'));
const model=async()=>{
 const dependency=await sourceURL('../features/publishing/secrets.js');
 const source=(await readFile(new URL('../features/publishing/targets.js',import.meta.url),'utf8')).replace('./secrets.js',dependency);
 return import('data:text/javascript;base64,'+Buffer.from(source).toString('base64'));
};
function memory(initial={}){const values=new Map(Object.entries(initial));return{values,getItem:key=>values.get(key)||null,setItem:(key,value)=>values.set(key,value)};}
const github={provider:'github',name:'list',repo:'owner/repo',branch:'main',path:'results/ip.txt',message:'update',format:'txt'};
test('targets validate provider destination without persisting credential inputs',async()=>{
 const {validateTarget}=await model();
 const target=validateTarget({...github,secret:'test-token',remember:true},'id');
 assert.equal(target.id,'id');assert.equal(target.secret,undefined);assert.equal(target.repo,'owner/repo');
 for(const values of [{...github,repo:'invalid'},{...github,path:'../secret'},{...github,provider:'dns'},{provider:'edgetunnel',name:'edge',host:'https://user:pass@example.org',mode:'append'},{provider:'edgetunnel',name:'edge',host:'https://example.org/?token=x',mode:'append'},{provider:'edgetunnel',name:'edge',host:'example.org',mode:'delete'}])assert.throws(()=>validateTarget(values,'id'));
 assert.equal(validateTarget({provider:'edgetunnel',name:'edge',host:'example.org',mode:'append'},'edge').host,'https://example.org');
});
test('legacy import keeps keys intact, credentials session-only and repeated/deleted imports do not return',async()=>{
 const {createTargetStore}=await model();let i=0;
 const original=JSON.stringify([{name:'old',repo:'owner/repo',branch:'dev',path:'ip.csv',token:'fake-legacy-token'}]);
 const storage=memory({'cfdata-github-accounts':original,'cfdata-edgetunnel-accounts':JSON.stringify([{name:'edge',host:'edge.example.org',mode:'append',passwordCipher:'iv:cipher'}]),theme:'dark'});
 const options={storage,crypto:webcrypto,origin:'http://127.0.0.1:13336',createID:()=>`id-${++i}`};
 let store=createTargetStore(options);assert.equal(store.list().length,2);assert.equal(await store.credential('id-1'),'fake-legacy-token');
 assert.equal(storage.getItem('cfdata-github-accounts'),original);assert.equal(storage.getItem('theme'),'dark');
 assert.doesNotMatch(storage.getItem('cfdata-publishing-targets'),/fake-legacy-token|"token"|"password"/);
 store=createTargetStore(options);assert.equal(store.list().length,2);assert.equal(await store.credential('id-1'),'fake-legacy-token');
 store.remove('id-1');store=createTargetStore(options);assert.equal(store.list().length,1);assert.equal(await store.credential('id-1'),'');
});
test('session and encrypted secrets follow stable IDs through rename and explicit forget',async()=>{
 const {createTargetStore}=await model();const storage=memory();let i=0;
 const options={storage,crypto:webcrypto,origin:'http://localhost',createID:()=>`id-${++i}`};
 let store=createTargetStore(options);
 const first=await store.save({...github,secret:'fake-session',remember:false});assert.equal(await store.credential(first.id),'fake-session');
 assert.doesNotMatch(storage.getItem('cfdata-publishing-targets'),/fake-session/);
 const second=await store.save({...github,name:'list',secret:'fake-remembered',remember:true});assert.notEqual(first.id,second.id);
 const renamed=await store.save({...second,name:'new-name'});assert.equal(renamed.id,second.id);assert.equal(await store.credential(second.id),'fake-remembered');
 store=createTargetStore(options);assert.equal(await store.credential(second.id),'fake-remembered');assert.equal(await store.credential(first.id),'');
 await store.save({...store.list().find(t=>t.id===second.id),forget:true});assert.equal(await store.credential(second.id),'');
 store.remove(first.id);assert.equal(store.list().length,1);assert.throws(()=>store.remove('missing'),/目标/);
});
test('bad/blocked/quota storage never destroys legacy or writes plaintext and stays usable',async()=>{
 const {createTargetStore}=await model();
 for(const storage of [memory({'cfdata-publishing-targets':'broken'}),{getItem(){throw Error('blocked')},setItem(){throw Error('blocked')}}]){
  const store=createTargetStore({storage,crypto:undefined,origin:'local',createID:()=> 'session'});
  const target=await store.save({...github,secret:'fake-token',remember:true});assert.equal(await store.credential(target.id),'fake-token');assert.match(store.warning,/保存|存储|加密/);
 }
 const storage=memory({'cfdata-github-accounts':'{broken'});const original=storage.getItem('cfdata-github-accounts');
 const store=createTargetStore({storage,origin:'local',createID:()=> 'session'});assert.match(store.warning,/旧|读取/);assert.equal(storage.getItem('cfdata-github-accounts'),original);
 const quota=memory();quota.setItem=()=>{throw Error('quota');};
 const fallback=createTargetStore({storage:quota,origin:'local',createID:()=> 'quota'});await fallback.save({...github,secret:'fake-token'});assert.equal(await fallback.credential('quota'),'fake-token');assert.match(fallback.warning,/保存|存储/);
});
test('forgetting legacy plaintext prevents later rehydration without deleting the old key',async()=>{
 const {createTargetStore}=await model();const storage=memory({'cfdata-github-accounts':JSON.stringify([{...github,token:'fake-old'}])});
 const options={storage,crypto:webcrypto,origin:'http://localhost',createID:()=> 'legacy'};
 let store=createTargetStore(options);await store.save({...store.list()[0],forget:true});
 store=createTargetStore(options);assert.equal(await store.credential('legacy'),'');assert.match(storage.getItem('cfdata-github-accounts'),/fake-old/);
});
test('saving or decrypting an old snapshot does not resurrect a concurrently deleted target',async()=>{
 const {createTargetStore}=await model();const storage=memory();let release;
 const delayed={getRandomValues:value=>webcrypto.getRandomValues(value),subtle:{importKey:(...args)=>webcrypto.subtle.importKey(...args),deriveKey:(...args)=>new Promise(resolve=>{release=()=>resolve(webcrypto.subtle.deriveKey(...args));}),encrypt:(...args)=>webcrypto.subtle.encrypt(...args)}};
 const store=createTargetStore({storage,crypto:delayed,origin:'http://localhost',createID:()=> 'id'});
 const created=await store.save({...github,secret:'session'});
 const pending=store.save({...created,secret:'new-secret',remember:true});while(!release)await new Promise(resolve=>setTimeout(resolve,1));
 store.remove(created.id);release();await assert.rejects(pending,/目标/);assert.equal(store.list().length,0);
});
test('unsupported crypto retains previous stored cipher while replacement remains session-only',async()=>{
 const {createTargetStore}=await model();const storage=memory();const options={storage,origin:'http://localhost',createID:()=> 'id'};
 let store=createTargetStore({...options,crypto:webcrypto});await store.save({...github,secret:'old-secret',remember:true});
 const cipher=JSON.parse(storage.getItem('cfdata-publishing-targets')).targets[0].credential;
 store=createTargetStore(options);await store.save({...store.list()[0],secret:'new-session',remember:true});
 assert.deepEqual(JSON.parse(storage.getItem('cfdata-publishing-targets')).targets[0].credential,cipher);
 assert.equal(await store.credential('id'),'new-session');assert.match(store.warning,/加密/);
});
test('real crypto roundtrip rejects tamper/context change and decrypts both legacy formats',async()=>{
 const {encryptSecret,decryptSecret}=await secrets();const origin='http://localhost';
 const encrypted=await encryptSecret('fake-token','id',origin,webcrypto);
 assert.equal(await decryptSecret(encrypted,'id',origin,webcrypto),'fake-token');assert.equal(await decryptSecret(encrypted,'other',origin,webcrypto),'');
 assert.equal(await decryptSecret({...encrypted,value:encrypted.value.slice(0,-4)+'xxxx'},'id',origin,webcrypto),'');
 const legacySource=await readFile(new URL('../../index.html',import.meta.url),'utf8');
 const githubSalt=/const githubTokenCryptoSalt = '([^']+)'/.exec(legacySource)[1];
 for(const [provider,salt,separator] of [['github',githubSalt,'.'],['edgetunnel','cfdata-edgetunnel-token-v1',':']]){
  const material=await webcrypto.subtle.importKey('raw',new TextEncoder().encode(`${origin}|old|${salt}`),'PBKDF2',false,['deriveKey']);
  const key=await webcrypto.subtle.deriveKey({name:'PBKDF2',salt:new TextEncoder().encode(salt),iterations:120000,hash:'SHA-256'},material,{name:'AES-GCM',length:256},false,['encrypt']);
  const iv=webcrypto.getRandomValues(new Uint8Array(12));const cipher=await webcrypto.subtle.encrypt({name:'AES-GCM',iv},key,new TextEncoder().encode('fake-old'));
  const value=Buffer.from(iv).toString('base64')+separator+Buffer.from(cipher).toString('base64');
  assert.equal(await decryptSecret({scheme:`legacy-${provider}`,context:'old',value},'new-id',origin,webcrypto),'fake-old');
 }
});
