'use strict';
const $=id=>document.getElementById(id),clone=v=>structuredClone(v);
let state,guid='',boardID='',draft=null,busy=false;
const uid=()=>`d${crypto.randomUUID().replaceAll('-','')}`;
const stamp=s=>s?(s.at===0?'Initial game version':`${new Date(s.at).toLocaleString()} · ${s.actor==='game'?'WoW':'Desktop'}`):'New';
const equalStamp=(a,b)=>a&&b&&a.at===b.at&&a.seq===b.seq&&a.actor===b.actor;
const el=(tag,text,parent)=>{const n=document.createElement(tag);if(text!==undefined)n.textContent=text;if(parent)parent.append(n);return n;};
const button=(text,parent,fn)=>{const n=el('button',text,parent);n.type='button';n.addEventListener('click',()=>Promise.resolve().then(fn).catch(error));return n;};
function error(e){$('message').className='error';$('message').textContent=e.message;if($('history-dialog').open)$('history-error').textContent=e.message;}
function char(){return state?.characters[guid];}
function records(){return Object.values(char()?.records||{});}
function boards(){return records().filter(r=>r.key.startsWith('b/')&&!r.deleted).sort((a,b)=>a.data.order-b.data.order||a.key.localeCompare(b.key));}
function board(){return char()?.records[`b/${boardID}`];}
function cards(){return records().filter(r=>r.key.startsWith(`c/${boardID}/`)&&!r.deleted).sort((a,b)=>a.data.order-b.data.order||a.key.localeCompare(b.key));}
function options(select,choices,value){select.replaceChildren();for(const[key,name]of choices){const o=el('option',name,select);o.value=key;}select.value=value;}
async function request(path,value){const r=await fetch(path,value?{method:'POST',headers:{'Content-Type':'application/json','X-Lamda-Sim':'1'},body:JSON.stringify(value)}:undefined);const out=await r.json();if(!r.ok)throw new Error(out.error||'Planner request failed');return out;}
async function refresh(){if(busy)return;try{state=await request('/api/kbb');render();}catch(e){error(e);}}
async function saveChanges(changes){busy=true;try{state=await request('/api/kbb/edit',{guid,changes});$('message').className='success';$('message').textContent='Saved. Your changes will load on your next /reload.';render();}finally{busy=false;}}
const change=(r,data,deleted=false)=>({key:r.key,base:r.stamp||null,...(deleted?{deleted:true}:{data})});
function render(){
 const chars=Object.values(state.characters);if(!state.characters[guid])guid=chars[0]?.guid||'';
 options($('character'),chars.map(c=>[c.guid,`${c.name||c.guid} — ${c.realm}`]),guid);
 const bs=boards();if(!bs.some(b=>b.data.id===boardID))boardID=bs[0]?.data.id||'';
 options($('board'),bs.map(b=>[b.data.id,b.data.name]),boardID);
 $('client').textContent=state.client==='retail'?'Retail':'Forever';$('empty').hidden=!!board();
 for(const id of ['new-board','history'])$(id).disabled=!char();$('settings').disabled=!board();
 let pending=0;for(const r of records())if(!equalStamp(r.stamp,char().gameSeen[r.key]))pending++;
 const status=$('sync');status.className=pending?'pending':'synced';
 if(state.publishError){status.textContent=`Saved locally. ${state.publishError}`;status.className='pending';}
 else if(!state.published&&pending)status.textContent='Saved locally. Preparing changes for WoW…';
 else if(pending)status.textContent=`${pending} change${pending===1?'':'s'} ready for your next /reload.`;
 else if(char())status.textContent=`Synced with WoW · last received ${new Date(char().lastGame*1000).toLocaleTimeString()}`;
 else status.textContent='Waiting for your first planner from WoW.';
 $('columns').replaceChildren();const b=board()?.data;if(!b)return;
 const query=$('search').value.toLocaleLowerCase(),archived=$('archived').checked;
 for(const col of b.columns){const lane=el('section',undefined,$('columns'));lane.className='lane';const head=el('header',undefined,lane);el('h2',col.name,head);const add=button('+',head,()=>editCard(null,col.id));add.className='add';add.setAttribute('aria-label',`Add card to ${col.name}`);
  for(const r of cards().filter(r=>r.data.column===col.id&&!!r.data.archived===archived)){
   const c=r.data;if(query&&!`${c.title}\n${c.notes}\n${Object.values(c.values).join('\n')}`.toLocaleLowerCase().includes(query))continue;
   const node=el('article',undefined,lane);node.className='planner-card';el('h3',c.title,node);if(c.notes)el('p',c.notes,node);
   const info=el('div',stamp(r.stamp),node);info.className='stamp';if(!equalStamp(r.stamp,char().gameSeen[r.key])){info.classList.add('pending');info.textContent+=' · waiting for reload';}
   const actions=el('div',undefined,node);actions.className='actions';button('Edit',actions,()=>editCard(r));button(c.archived?'Restore':'Archive',actions,()=>saveChanges([change(r,{...c,archived:!c.archived})]));
   button('Move up',actions,async()=>{const peers=cards().filter(x=>x.data.column===c.column&&!!x.data.archived===!!c.archived);const i=peers.findIndex(x=>x.key===r.key);if(i<1)return;const prev=peers[i-1];await saveChanges([change(r,{...c,order:prev.data.order}),change(prev,{...prev.data,order:c.order})]);});
  }
 }
}
function input(parent,label,type,value,opts){const wrap=el('label',undefined,parent);el('span',label,wrap);const n=el(type==='textarea'?'textarea':type==='select'?'select':'input',undefined,wrap);if(type==='select')options(n,opts,value);else if(type==='checkbox'){n.type=type;n.checked=!!value;wrap.className='check';}else{if(type!=='textarea')n.type=type;n.value=value??'';}return n;}
function dialog(title,record){if($('editor').open)throw new Error('Save or cancel the open editor first.');draft={record:record&&clone(record),guid,boardID};$('edit-title').textContent=title;$('edit-fields').replaceChildren();$('edit-error').textContent='';$('editor').showModal();return $('edit-fields');}
function editCard(record,col){
 const b=clone(board().data),parent=dialog(record?'Edit card':'New card',record);
 const c=record?clone(record.data):{id:uid(),title:'',notes:'',column:col||b.columns[0].id,kind:'Note',priority:'Normal',values:{},order:Math.max(0,...cards().map(r=>r.data.order))+1,created:Math.floor(Date.now()/1000)};
 const title=input(parent,'Title','text',c.title);title.required=true;title.maxLength=160;const notes=input(parent,'Notes','textarea',c.notes);notes.rows=8;notes.maxLength=8000;
 const grid=el('div',undefined,parent);grid.className='grid';const column=input(grid,'List','select',c.column,b.columns.map(c=>[c.id,c.name])),kind=input(grid,'Category','select',c.kind,['Note','Dungeon','Achievement','Recipe','Rotation','Quest'].map(s=>[s,s])),priority=input(grid,'Priority','select',c.priority,['Normal','High','Low'].map(s=>[s,s])),archived=input(grid,'Archived','checkbox',c.archived),fields=[];
 for(const f of b.fields){if(f.hidden)continue;const type={Text:'text',Number:'number',Checkbox:'checkbox',Choice:'select'}[f.type];const n=input(parent,f.name,type,c.values[f.id],f.type==='Choice'?[['','—'],...f.options.map(s=>[s,s])]:undefined);if(type==='number')n.step='any';if(type==='text')n.maxLength=500;fields.push([f,n]);}
 draft.save=async()=>{Object.assign(c,{title:title.value.trim(),notes:notes.value,column:column.value,kind:kind.value,priority:priority.value,archived:archived.checked,updated:Math.floor(Date.now()/1000)});for(const[f,n]of fields){if(f.type==='Checkbox')c.values[f.id]=n.checked;else if(n.value==='')delete c.values[f.id];else c.values[f.id]=f.type==='Number'?Number(n.value):n.value;}await saveChanges([change(draft.record||{key:`c/${draft.boardID}/${c.id}`},c)]);};title.focus();
}
function editBoard(isNew=false){
 const record=isNew?null:clone(board()),parent=dialog(isNew?'New board':'Board settings',record);
 const b=record?clone(record.data):{id:uid(),name:'',order:Math.max(0,...boards().map(r=>r.data.order))+1,columns:[{id:uid(),name:'Backlog'},{id:uid(),name:'Doing'},{id:uid(),name:'Done'}],fields:[]};if(isNew){b.questColumn=b.columns[1].id;b.doneColumn=b.columns[2].id;}
 const originalCards=isNew?[]:clone(cards()),name=input(parent,'Board name','text',b.name);name.required=true;name.maxLength=60;
 const autoWatch=input(parent,'Create cards for watched quests','checkbox',b.autoWatch),autoDone=input(parent,'Move completed quests to Done','checkbox',b.autoDone),hide=input(parent,'Hide the WoW quest tracker','checkbox',b.hideQuestTracker),q=input(parent,'Quest list','select',b.questColumn,b.columns.map(c=>[c.id,c.name])),done=input(parent,'Done list','select',b.doneColumn,b.columns.map(c=>[c.id,c.name]));
 const columns=el('div',undefined,parent),fields=el('div',undefined,parent),removed=new Map();
 function destinations(){options(q,b.columns.map(c=>[c.id,c.name]),b.columns.some(c=>c.id===q.value)?q.value:b.columns[0].id);options(done,b.columns.map(c=>[c.id,c.name]),b.columns.some(c=>c.id===done.value)?done.value:b.columns.at(-1).id);}
 function drawColumns(){columns.replaceChildren();el('h3','Lists',columns);b.columns.forEach((c,i)=>{const box=el('fieldset',undefined,columns);el('legend',`List ${i+1}`,box);const n=input(box,'Name','text',c.name);n.required=true;n.maxLength=40;n.addEventListener('input',()=>{c.name=n.value;destinations();});const grid=el('div',undefined,box);grid.className='grid';
  for(const[key,label]of [['showOnUI','Display on screen'],['uiTransparent','Transparent'],['uiLocked','Lock position'],['uiCollapsed','Minimized'],['uiCompact','Compact']]){const ctl=input(grid,label,'checkbox',c[key]);ctl.addEventListener('change',()=>c[key]=ctl.checked);}
  const placement=input(grid,'Placement','select',c.uiPlacement||'floating',[['floating','Floating'],['tracker','Quest tracker'],['both','Both']]);placement.addEventListener('change',()=>c.uiPlacement=placement.value);
  const size=el('details',undefined,box);el('summary','Size and visibility',size);const width=input(size,'Width (220–640)','number',c.uiSize?.width||320),height=input(size,'Height (150–4096)','number',c.uiSize?.height||420);width.min=220;width.max=640;height.min=150;height.max=4096;const sizing=()=>c.uiSize={width:Number(width.value),height:Number(height.value)};width.addEventListener('change',sizing);height.addEventListener('change',sizing);
  for(const[key,label]of [['world','Outdoors'],['party','Dungeons'],['key','Active Mythic+ key'],['raid','Raids'],['scenario','Delves / scenarios'],['arena','Arenas'],['pvp','Battlegrounds']]){const ctl=input(size,label,'checkbox',c.uiVisibility?.[key]!==false);ctl.addEventListener('change',()=>{c.uiVisibility||={};c.uiVisibility[key]=ctl.checked;});}
  if(i>0)button('Move left',box,()=>{[b.columns[i-1],b.columns[i]]=[b.columns[i],b.columns[i-1]];drawColumns();});if(b.columns.length>1)button('Remove list',box,()=>{const dest=b.columns.find(x=>x.id!==c.id);removed.set(c.id,dest.id);for(const[key,val]of removed)if(val===c.id)removed.set(key,dest.id);b.columns.splice(i,1);destinations();drawColumns();});
 });if(b.columns.length<12)button('Add list',columns,()=>{b.columns.push({id:uid(),name:`List ${b.columns.length+1}`});drawColumns();destinations();});}
 function drawFields(){fields.replaceChildren();el('h3','Custom fields',fields);for(const f of b.fields){const box=el('fieldset',undefined,fields),n=input(box,'Field name','text',f.name);n.maxLength=40;n.required=true;n.addEventListener('input',()=>f.name=n.value);el('p',f.type,box);const hidden=input(box,'Hide field','checkbox',f.hidden);hidden.addEventListener('change',()=>f.hidden=hidden.checked);if(f.type==='Choice'){const choices=input(box,'Choices (one per line)','textarea',(f.options||[]).join('\n'));choices.rows=3;choices.addEventListener('input',()=>f.options=choices.value.split('\n').map(s=>s.trim()).filter(Boolean));}}
 if(b.fields.length<16){const type=input(fields,'New field type','select','Text',['Text','Number','Checkbox','Choice'].map(s=>[s,s]));button('Add field',fields,()=>{b.fields.push({id:uid(),name:`Field ${b.fields.length+1}`,type:type.value,...(type.value==='Choice'?{options:['Option 1','Option 2']}:{})});drawFields();});}}
 drawColumns();drawFields();draft.save=async()=>{Object.assign(b,{name:name.value.trim(),questColumn:q.value,doneColumn:done.value,autoWatch:autoWatch.checked,autoDone:autoDone.checked,hideQuestTracker:hide.checked});const changes=[change(record||{key:`b/${b.id}`},b)];for(const r of originalCards)if(removed.has(r.data.column))changes.push(change(r,{...r.data,column:removed.get(r.data.column)}));await saveChanges(changes);boardID=b.id;render();};name.focus();
}
function showHistory(){
 const list=$('history-list');list.replaceChildren();$('history-error').textContent='';
 for(const h of char().history){
  const section=el('article',undefined,list);el('h3',h.data?.title||h.data?.name||'Deleted item',section);el('p',stamp(h.stamp),section);
  const details=el('details',undefined,section);el('summary','View previous content',details);
  if(h.deleted)el('p','Deleted',details);
  else if(h.data.title){
   const d=h.data,b=char().records[`b/${h.key.split('/')[1]}`]?.data;
   el('p',`${d.kind} · ${d.priority} priority${d.archived?' · Archived':''}`,details);
   el('pre',d.notes||'No notes',details);
   for(const[key,value]of Object.entries(d.values||{})){const f=b?.fields.find(f=>f.id===key);el('p',`${f?.name||'Former custom field'}: ${value}`,details);}
  }else{
   for(const col of h.data.columns){el('p',`${col.name} · ${col.showOnUI?'Displayed':'Hidden'}${col.uiTransparent?' · Transparent':''}${col.uiCollapsed?' · Minimized':''}`,details);}
   if(h.data.fields.length)el('p',`Custom fields: ${h.data.fields.map(f=>f.name).join(', ')}`,details);
  }
  button('Restore this version',section,async()=>{const current=char().records[h.key];await saveChanges([change(current||{key:h.key},h.data,h.deleted)]);$('history-dialog').close();});
 }
 if(!char().history.length)el('p','No previous versions yet.',list);$('history-dialog').showModal();
}
$('edit-form').addEventListener('submit',async event=>{event.preventDefault();if(busy||!draft)return;$('save').disabled=true;try{if(guid!==draft.guid)throw new Error('Return to the character you were editing.');await draft.save();$('editor').close();draft=null;}catch(e){$('edit-error').textContent=e.message;}finally{$('save').disabled=false;}});
$('cancel').addEventListener('click',()=>{$('editor').close();draft=null;});$('editor').addEventListener('cancel',()=>draft=null);$('new-board').addEventListener('click',()=>editBoard(true));$('settings').addEventListener('click',()=>editBoard());$('history').addEventListener('click',showHistory);$('history-close').addEventListener('click',()=>$('history-dialog').close());
$('character').addEventListener('change',()=>{guid=$('character').value;boardID='';render();});$('board').addEventListener('change',()=>{boardID=$('board').value;render();});$('search').addEventListener('input',render);$('archived').addEventListener('change',render);window.addEventListener('beforeunload',e=>{if(draft){e.preventDefault();e.returnValue='';}});refresh();setInterval(refresh,2500);
