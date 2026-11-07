(function(root){
 'use strict';
 const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 root.OverlaySavedMemory=class {
  constructor(context,notify){this.context=context;this.notify=notify;this.identity='';this.profile='';this.open=false;this.serial=0;this.data=null;this.busy=false;}
  mount(host,rule,key){
   this.serial++;this.busy=false;this.host=host;if(!host)return;
   const id=rule+'\0'+key;if(id!==this.identity){this.identity=id;this.rule=rule;this.key=key;this.data=null;this.profile='';this.open=false;}
   host.open=this.open;host.addEventListener('toggle',()=>{if(this.host!==host)return;this.open=host.open;if(this.open)this.load();});
   host.addEventListener('change',e=>{if(e.target.matches('[data-history-character]')){this.profile=e.target.value;this.data=null;this.load();}});
   host.addEventListener('click',e=>{const b=e.target.closest('button');if(!b)return;if(b.hasAttribute('data-history-refresh'))this.load();if(b.hasAttribute('data-history-reset'))this.reset(false);if(b.hasAttribute('data-history-recover'))this.reset(true);});
   this.render();if(this.open)this.load();
  }
  entries(){
   const c=this.context(),directory=c.characters,found=new Map();
   for(const p of [...(directory?.list()||[]),...(this.data?.characters||[])])found.set(directory.key(p),p);
   return [...found.values()];
  }
  selected(){const c=this.context();return this.entries().find(p=>c.characters.key(p)===this.profile);}
  render(){
   if(!this.host)return;
   const c=this.context(),body=this.host.querySelector('[data-history-body]'),entries=this.entries();
   if(!c.saved){body.innerHTML='<p>Save changes to manage history.</p>';return;}
   if(!this.profile){const current=c.current;this.profile=current?.character&&current?.realm?c.characters.key(current):entries[0]?c.characters.key(entries[0]):'';}
   const p=this.selected(),data=this.data;
   body.innerHTML=`<label>Character<select data-history-character aria-label="History character"><option value="">Choose character…</option>${entries.map(p=>`<option value="${esc(c.characters.key(p))}" ${c.characters.key(p)===this.profile?'selected':''}>${esc(c.characters.label(p))}</option>`).join('')}</select></label><div class="history-value"><output data-history-value>${p&&data?esc(root.OverlayModel.Values.display(data.value)):'—'}</output><span>${p&&data?(data.captured?'Captured':'Initial value'):''}</span></div>${data?.error?`<p class="history-error" role="status">${esc(data.error)}</p>`:''}<div class="history-actions"><button class="button" data-history-refresh ${this.busy?'disabled':''}>Refresh</button><button class="button" data-history-reset ${!p||!data?.captured||this.busy||data?.recoverable?'disabled':''}>Reset value</button>${data?.recoverable?`<button class="button danger" data-history-recover ${this.busy?'disabled':''}>Reset saved history</button>`:''}</div>`;
  }
  async request(path,edit){const r=await fetch('/api/overlays/memory'+path,edit?{method:'POST',headers:{'Content-Type':'application/json','X-Lamda-Sim':'1'},body:JSON.stringify(edit)}:{});const data=await r.json();if(!r.ok)throw Error(data.error||'Saved history unavailable');return data;}
  async load(){
   if(!this.host?.open||!this.context().saved)return;
   const serial=++this.serial;this.busy=true;this.render();
   const requestedProfile=this.profile,p=this.selected()||{},query=new URLSearchParams({rule:this.rule,key:this.key,character:p.character||'',realm:p.realm||''});
   try{const data=await this.request('?'+query);if(serial!==this.serial)return;this.data=data;}
   catch(error){if(serial===this.serial){this.data=null;this.notify(error.message);}}
   finally{if(serial===this.serial){this.busy=false;this.render();if(this.profile!==requestedProfile)this.load();}}
  }
  async reset(recover){
   const c=this.context(),p=this.selected();if(this.busy||!c.saved||!this.data||(!recover&&!p))return;
   if(this.data.revision!==c.revision){this.notify('The rules changed in another window. Reload this page.');return;}
   if(!root.confirm(recover?'Reset all saved history? The unreadable file will be kept as a backup.':`Reset this value for ${c.characters.label(p)}?`))return;
   this.busy=true;this.render();const serial=++this.serial;
   try{await this.request(recover?'/recover':'/reset',{revision:c.revision,rule:this.rule,key:this.key,...(recover?{}:p)});if(serial===this.serial){this.data=null;await this.load();}}
   catch(error){if(serial===this.serial)this.notify(error.message);}
   finally{if(serial===this.serial){this.busy=false;this.render();}}
  }
 };
})(window);
