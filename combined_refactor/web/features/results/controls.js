import { browseResultRows } from './browser.js';
const emptyHeaders = [];

export function mountResultBrowser({ root, name, type, onChange }) {
  const container = root.querySelector(`[data-browse="${name}"]`), controls = {}, options = {page:1};
  let source, cachedRows, cachedHeaders, cachedOptions, selected;
  function field(key,label,values) {
    const wrapper=root.createElement('label');wrapper.textContent=label;
    const element=root.createElement(values?'select':'input');element.dataset.browseField=key;
    if(values)for(const [value,text] of values){const option=root.createElement('option');option.value=value;option.textContent=text;element.append(option);}
    else{element.type='number';element.min='0';element.value=key==='loss'?'100':'0';}
    wrapper.append(element);container.append(wrapper);controls[key]=element;
    element.addEventListener('change',()=>{options.page=1;cachedOptions=undefined;onChange();});
  }
  if(type==='nsb')field('ipType','地址族',[['all','全部'],['4','IPv4'],['6','IPv6']]);
  if(type!=='scan')field('qualified','测速状态',[['all','全部'],['qualified','达标'],['unqualified','未达标/失败']]);
  field('latency','延迟低于（ms，0不限制）');
  if(type!=='scan')field('loss','丢包低于（%，100不限制）');
  for(let i=1;i<=3;i++)field('sort'+i,'排序优先级'+i,[['none','无'],['latency','延迟升序'],['loss','丢包升序'],['speed','速度降序']]);
  controls.sort1.value='latency';
  const pager=root.createElement('div');pager.className='result-pager';
  const previous=root.createElement('button'),next=root.createElement('button'),status=root.createElement('span');
  for(const button of [previous,next]){button.type='button';button.className='button';}
  previous.textContent='上一页';next.textContent='下一页';previous.dataset.page='previous';next.dataset.page='next';status.dataset.pageStatus='';
  pager.append(previous,status,next);container.append(pager);
  previous.addEventListener('click',()=>{options.page--;onChange();});next.addEventListener('click',()=>{options.page++;onChange();});
  return {
    select(rows,revision,headers=emptyHeaders,speedMin=.1) {
      const values={...options,speedMin,sort:[controls.sort1.value,controls.sort2.value,controls.sort3.value]};
      for(const key of ['ipType','qualified','latency','loss'])if(controls[key])values[key]=controls[key].value;
      const serialized=JSON.stringify({...values,page:undefined});
      if(source!==revision||cachedRows!==rows||cachedHeaders!==headers||cachedOptions!==serialized){selected=browseResultRows(rows,{...values,allIndexes:true},type,headers);source=revision;cachedRows=rows;cachedHeaders=headers;cachedOptions=serialized;}
      const pages=Math.max(1,Math.ceil(selected.total/100)),page=Math.max(1,Math.min(pages,options.page));
      options.page=page;previous.disabled=page<=1;next.disabled=page>=pages;
      status.textContent=`第 ${page}/${pages} 页 · 筛选 ${selected.total}/${rows.length} 条`;
      return{indexes:selected.indexes.slice((page-1)*100,page*100),total:selected.total,page,pages};
    },
  };
}
