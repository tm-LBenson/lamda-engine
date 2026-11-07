(function(root){
 'use strict';
 const M=root.OverlayModel,esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 root.OverlayPackEditor=class {
  constructor(options){Object.assign(this,options);this.host.addEventListener('change',e=>this.change(e));this.host.addEventListener('click',e=>this.click(e));}
  render(){const r=this.current();this.host.parentElement.hidden=false;if(!r){this.host.innerHTML='<button class="button" data-pack-backups>Backups</button>';return;}
   const p=r.pack;this.host.innerHTML=`<select data-pack-select aria-label="Overlay pack"><option value="">None</option>${M.Packs.list(this.rules()).map(pack=>`<option value="${pack.instance}" ${p?.instance===pack.instance?'selected':''}>${esc(pack.name)}</option>`).join('')}</select><button class="button" data-pack-create>New pack</button>${p?`<label>Name<input data-pack-edit="name" maxlength="60" value="${esc(p.name)}"></label><label>Version<input data-pack-edit="version" type="number" min="1" max="1000000" step="1" value="${p.version}"></label><div class="pack-members">${M.Packs.members(this.rules(),r).map(m=>`<button class="text-button" data-pack-member="${m.id}" ${m===r?'disabled':''}>${esc(m.name)}</button>`).join('')}</div><button class="button" data-pack-export>Export pack</button>`:''}<button class="button" data-pack-backups>Backups</button>`;
  }
  change(event){const el=event.target;if(!el.checkValidity()){el.reportValidity();return;}
   if(el.hasAttribute('data-pack-select'))this.mutate(()=>el.value?M.Packs.join(this.rules(),this.current(),el.value,M.id):M.Packs.detach(this.rules(),this.current()));
   if(el.dataset.packEdit)this.mutate(()=>M.Packs.edit(this.rules(),this.current(),el.dataset.packEdit,el.type==='number'?+el.value:el.value.trim()));
  }
  click(event){const b=event.target.closest('button');if(!b)return;
   if(b.hasAttribute('data-pack-create'))this.mutate(()=>M.Packs.create(this.rules(),this.current(),M.id));
   if(b.dataset.packMember)this.select(b.dataset.packMember);
   if(b.hasAttribute('data-pack-export'))this.export();
   if(b.hasAttribute('data-pack-backups'))this.backups();
  }
 };
})(window);
