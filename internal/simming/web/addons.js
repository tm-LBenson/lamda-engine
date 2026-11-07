'use strict';
const $=id=>document.getElementById(id);
let state={modules:[]},tab='browse',busy=false;
async function api(options){
 const response=await fetch('/api/addons',options);let body={};try{body=await response.json();}catch{}
 if(!response.ok)throw Error(body.error||'Browse Addons is unavailable.');return body;
}
function message(value,error=false){$('message').textContent=value||'';$('message').className=error?'error':'success';}
function statusName(module){return module.status==='update'?'Update available':module.installed?'Installed':'Available';}
function visible(module){
 if(tab==='installed'&&!module.installed)return false;if(tab==='updates'&&module.status!=='update')return false;
 const query=$('search').value.trim().toLowerCase();return !query||(module.name+' '+module.summary).toLowerCase().includes(query);
}
function button(text,kind,handler){const value=document.createElement('button');value.textContent=text;if(kind)value.className=kind;value.disabled=busy;value.addEventListener('click',handler);return value;}
async function apply(module,action){
 if(action==='uninstall'&&!confirm('Uninstall '+module.name+'? Its settings will be kept.'))return;
 busy=true;message(action==='uninstall'?'Uninstalling…':action==='update'?'Updating…':'Installing…');render();
 try{const result=await api({method:'POST',headers:{'Content-Type':'application/json','X-Lamda-Sim':'1'},body:JSON.stringify({id:module.id,action})});state=result.state;message(module.name+' is '+(action==='uninstall'?'uninstalled.':'ready. Use /reload in WoW.'));}
 catch(error){message(error.message,true);}finally{busy=false;render();}
}
function render(){
 const updates=state.modules.filter(module=>module.status==='update');$('update-count').textContent=updates.length?updates.length:'';$('update-all').hidden=tab!=='updates'||!updates.length;$('update-all').disabled=busy;
 $('client').textContent=(state.client==='forever'?'Forever':'Retail')+' · LamdaUI '+(state.addonVersion||'');
 const holder=$('addons');holder.replaceChildren();
 for(const module of state.modules.filter(visible)){
  const card=document.createElement('article');card.className='addon-card';
  const header=document.createElement('header'),title=document.createElement('h2');title.textContent=module.name;header.append(title);
  const meta=document.createElement('div');meta.className='addon-meta';const status=document.createElement('span');status.className='status';status.textContent=statusName(module);meta.append(status);
  if(module.experimental){const badge=document.createElement('span');badge.className='experimental';badge.textContent='Experimental';meta.append(badge);}header.append(meta);
  const summary=document.createElement('p');summary.textContent=module.summary;
  const footer=document.createElement('footer');
  if(!module.installed)footer.append(button('Install','',()=>apply(module,'install')));
  else if(module.status==='update')footer.append(button('Update','',()=>apply(module,'update')));
  else{const version=document.createElement('span');version.className='addon-meta';version.textContent='Version '+module.version;footer.append(version);}
  if(module.installed)footer.append(button('Uninstall','quiet',()=>apply(module,'uninstall')));
  card.append(header,summary,footer);holder.append(card);
 }
 if(!holder.children.length){const empty=document.createElement('div');empty.className='empty';empty.textContent=tab==='updates'?'Your addons are up to date.':tab==='installed'?'No addons installed.':'No matching addons.';holder.append(empty);}
}
document.querySelectorAll('[data-tab]').forEach(control=>control.addEventListener('click',()=>{tab=control.dataset.tab;document.querySelectorAll('[data-tab]').forEach(item=>item.classList.toggle('active',item===control));render();}));
$('search').addEventListener('input',render);
$('update-all').addEventListener('click',async()=>{for(const module of state.modules.filter(item=>item.status==='update')){await apply(module,'update');if(busy)break;}});
api().then(value=>{state=value;$('connection').textContent='Local engine';render();}).catch(error=>{message(error.message,true);$('connection').textContent='Unavailable';});
