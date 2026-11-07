(function(root){
 'use strict';
 const M=root.OverlayModel,U=root.OverlayPackUpdates,esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 const rowLabels={added:'New · disabled',changed:'Updated',unchanged:'Unchanged',removed:'Removed',restored:'Restored'};
 function rows(plan){return plan.rows.map(r=>`<li><span>${esc(r.name)}${r.previous&&r.previous!==r.name?`<small>Previously ${esc(r.previous)}</small>`:''}</span><small>${r.kind==='removed'&&plan.keepRemoved?'Kept separately · disabled':rowLabels[r.kind]}</small></li>`).join('');}
 function notes(plan){return `${plan.resetSettings.length?`<p>${plan.resetSettings.length} setting${plan.resetSettings.length===1?'':'s'} will use the pack value.</p><details><summary>Settings</summary><ul>${plan.resetSettings.map(s=>`<li>${esc(s.overlay)} · ${esc(s.name)}</li>`).join('')}</ul></details>`:''}${plan.sharedGroups?.length?`<p>Uses the current layout for ${plan.sharedGroups.map(esc).join(', ')}.</p>`:''}`;}
 root.OverlayPackImport=class {
  constructor(host,changed){this.host=host;this.changed=changed;this.reset();host.addEventListener('change',e=>{const el=e.target;if(el.hasAttribute('data-pack-import-mode'))this.mode=el.value;else if(el.hasAttribute('data-pack-update-target'))this.target=el.value;else if(el.dataset.packKeep)this.options[el.dataset.packKeep]=el.checked;this.changed();});}
  reset(){this.mode='copy';this.target='';this.options={layout:true,sounds:true,load:true,settings:true,keepRemoved:true,allowOlder:false};this.host.hidden=true;this.host.replaceChildren();}
  selection(){return {mode:this.mode,instance:this.target,options:{...this.options}};}
  preview(existing,incoming){
   let targets=[];try{targets=U.targets(existing,incoming);}catch(_){}
   this.host.hidden=!targets.length;if(!targets.length){this.mode='copy';return null;}
   if(!targets.some(p=>p.instance===this.target))this.target=targets[0].instance;
   const source=U.sourcePack(incoming),target=targets.find(p=>p.instance===this.target),older=source.version<=target.version;
   const check=(key,label)=>`<label class="check-label"><input type="checkbox" data-pack-keep="${key}" ${this.options[key]?'checked':''}> ${label}</label>`;
   this.host.innerHTML=`<label>Import as<select data-pack-import-mode aria-label="Import mode"><option value="copy" ${this.mode==='copy'?'selected':''}>New copy</option><option value="update" ${this.mode==='update'?'selected':''}>Update pack</option></select></label>${this.mode==='update'?`<label>Pack<select data-pack-update-target>${targets.map(p=>`<option value="${p.instance}" ${p.instance===this.target?'selected':''}>${esc(p.name)} · v${p.version}</option>`).join('')}</select></label><p class="pack-version">v${target.version} → v${source.version}</p><details class="pack-update-options"><summary>Keep my choices</summary>${check('layout','Layout and colors')}${check('sounds','Sounds')}${check('load','Load filters')}${check('settings','Setting overrides')}</details>${check('keepRemoved','Keep removed members separately')}${older?check('allowOlder','Allow same or older version'):''}`:''}`;
   if(this.mode!=='update')return null;
   const plan=U.plan(existing,incoming,this.target,{...this.options,allowOlder:true});return {plan,ready:!older||this.options.allowOlder};
  }
 };
 root.OverlayPackHistory=class {
  constructor(options){Object.assign(this,options);this.dialog=document.querySelector('#pack-history-dialog');this.body=this.dialog.querySelector('[data-pack-history-body]');this.serial=0;this.data={entries:[],revision:0};this.snapshot=null;this.busy=false;this.keepRemoved=true;this.instance='';this.error='';
   this.dialog.addEventListener('close',()=>{this.serial++;this.snapshot=null;});
   this.dialog.addEventListener('click',e=>this.click(e));this.dialog.addEventListener('change',e=>{if(e.target.hasAttribute('data-backup-keep')){this.keepRemoved=e.target.checked;this.render();}if(e.target.hasAttribute('data-backup-scope')){this.instance=e.target.value;this.snapshot=null;this.render();}});
  }
  async request(path='',edit){const response=await fetch('/api/overlays/pack-history'+path,edit?{method:'POST',headers:{'Content-Type':'application/json','X-Lamda-Sim':'1'},body:JSON.stringify(edit)}:{}),data=await response.json();if(!response.ok)throw Error(data.error||'Pack backups unavailable');return data;}
  async open(instance=''){this.instance=instance;this.snapshot=null;this.keepRemoved=true;this.error='';this.dialog.showModal();await this.load();}
  async load(){const serial=++this.serial;this.busy=true;this.render();try{const data=await this.request();if(serial===this.serial)this.data=data;}catch(error){if(serial===this.serial)this.error=error.message;}finally{if(serial===this.serial){this.busy=false;this.render();}}}
  render(){
   const entries=this.data.entries||[],packs=new Map(entries.map(e=>[e.instance,e.name]));if(!this.busy&&this.instance&&!packs.has(this.instance))this.instance='';
   let plan=null,problem=this.error||this.data.error||'';if(this.snapshot)try{plan=U.restore(this.rules(),this.snapshot,{keepRemoved:this.keepRemoved});}catch(error){problem=error.message;}
   const shown=entries.filter(e=>!this.instance||e.instance===this.instance);
   this.body.innerHTML=`${packs.size>1?`<label>Pack<select data-backup-scope><option value="">All packs</option>${[...packs].map(([id,name])=>`<option value="${id}" ${id===this.instance?'selected':''}>${esc(name)}</option>`).join('')}</select></label>`:''}${!entries.length&&!problem?`<p>${this.busy?'Loading…':'No backups yet.'}</p>`:''}<div class="pack-backup-list">${shown.map(e=>`<button class="button ${this.snapshot?.id===e.id?'primary':''}" data-backup-select="${e.id}" ${this.busy?'disabled':''}><span>${esc(e.name)} · v${e.version}</span><small>${esc(new Date(e.createdAt).toLocaleString())}</small></button>`).join('')}</div>${this.snapshot?`<section class="pack-restore-review"><h3>Restore v${this.snapshot.rules[0].pack.version}</h3><p>Restores this backup’s rules and settings.</p><label class="check-label"><input type="checkbox" data-backup-keep ${this.keepRemoved?'checked':''}> Keep extra members separately</label>${plan?`<ul class="pack-review-list">${rows(plan)}</ul>${notes(plan)}`:''}<div class="history-actions"><button class="button primary" data-backup-restore ${!plan||this.busy?'disabled':''}>Restore</button><button class="button danger" data-backup-delete ${this.busy?'disabled':''}>Delete backup</button></div></section>`:''}${problem?`<p class="history-error" role="status">${esc(problem)}</p>`:''}${this.data.recoverable?'<button class="button danger" data-backup-recover>Reset backups</button>':''}`;
  }
  async select(id){const serial=++this.serial;this.busy=true;this.error='';this.render();try{const snapshot=await this.request('?id='+encodeURIComponent(id));if(serial===this.serial)this.snapshot=snapshot;}catch(error){if(serial===this.serial)this.error=error.message;}finally{if(serial===this.serial){this.busy=false;this.render();}}}
  async click(event){const b=event.target.closest('button');if(!b)return;
   if(b.hasAttribute('data-backup-close')){this.dialog.close();return;}
   if(this.busy)return;
   if(b.dataset.backupSelect){await this.select(b.dataset.backupSelect);return;}
   if(b.hasAttribute('data-backup-restore')){try{const plan=U.restore(this.rules(),this.snapshot,{keepRemoved:this.keepRemoved});if(this.apply(plan)){this.dialog.close();this.notify('Restored locally. Save changes when ready.');}}catch(error){this.error=error.message;this.render();}return;}
   const recover=b.hasAttribute('data-backup-recover'),remove=b.hasAttribute('data-backup-delete');if(!recover&&!remove)return;
   if(!root.confirm(recover?'Reset pack backups? The unreadable file will be kept as a backup.':'Delete this saved pack version?'))return;
   const serial=++this.serial;this.busy=true;this.error='';this.render();
   try{const data=await this.request(recover?'/recover':'/delete',{revision:this.data.revision,...(recover?{}:{id:this.snapshot.id})});if(serial===this.serial){this.data=data;this.snapshot=null;}}catch(error){if(serial===this.serial)this.error=error.message;}finally{if(serial===this.serial){this.busy=false;this.render();}}
  }
 };
 root.OverlayPackReview={rows,notes};
})(window);
