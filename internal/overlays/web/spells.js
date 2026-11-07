/* One offline spell picker shared by rule, arch and preview fields. */
(function(root){
  'use strict';
  class SpellPicker {
    constructor(){
      const get=id=>document.getElementById(id);
      this.dialog=get('spell-dialog');this.input=get('spell-query');this.kind=get('spell-kind');this.classFilter=get('spell-class');
      this.list=get('spell-results');this.status=get('spell-status');this.more=get('spell-more');this.items=[];this.version=0;this.active=-1;
      this.input.addEventListener('input',()=>this.schedule());
      for(const el of [this.kind,this.classFilter])el.addEventListener('change',()=>this.search());
      for(const el of [this.input,this.list])el.addEventListener('keydown',e=>{
        if(e.key==='ArrowDown'||e.key==='ArrowUp'){e.preventDefault();this.highlight(this.active+(e.key==='ArrowDown'?1:-1));}
        if(e.key==='Enter'){e.preventDefault();this.choose(this.active);}
      });
      this.list.addEventListener('click',e=>{const el=e.target.closest('[data-spell-index]');if(el)this.choose(+el.dataset.spellIndex);});
      this.more.addEventListener('click',()=>this.search(true));
      get('spell-close').addEventListener('click',()=>this.dialog.close());
      this.dialog.addEventListener('close',()=>{this.stop();this.chooseSpell=null;this.returnFocus?.()?.focus();});
    }
    stop(){clearTimeout(this.timer);this.controller?.abort();this.version++;}
    open({id,prefer='',kind='auras',choose,focus}){
      this.stop();this.chooseSpell=choose;this.returnFocus=focus;this.prefer=prefer;
      this.input.value=id?String(id):'';this.classFilter.value='';this.kind.value=kind;this.clear();
      this.dialog.showModal();this.input.focus();this.input.select();this.search();
    }
    clear(){this.items=[];this.list.replaceChildren();this.more.hidden=true;this.highlight(-1);this.status.textContent='';}
    schedule(){this.stop();this.clear();this.timer=setTimeout(()=>this.search(),150);}
    async search(append=false){
      this.stop();const version=this.version;this.controller=new AbortController();
      const offset=append?this.items.length:0;if(!append)this.clear();
      this.more.disabled=true;this.input.setAttribute('aria-busy','true');
      try{
        const params=new URLSearchParams({q:this.input.value.trim(),kind:this.kind.value,class:this.classFilter.value,prefer:this.prefer,offset:String(offset)});
        const response=await fetch('/api/overlays/spells?'+params,{signal:this.controller.signal});const result=await response.json();
        if(!response.ok)throw Error(result.error||'Spell search unavailable.');
        if(version!==this.version||!this.dialog.open)return;
        if(this.classFilter.options.length===1)for(const c of [...result.classes].sort((a,b)=>a.name.localeCompare(b.name)))this.classFilter.add(new Option(c.name,c.id));
        this.items.push(...result.spells);this.render(offset);
        this.more.hidden=this.items.length>=result.total;
        this.status.textContent=result.total?`${this.items.length} of ${result.total}`:this.input.value.trim().length>=2?'No matches. Try All spells or All classes.':'';
      }catch(error){if(error.name!=='AbortError'&&version===this.version)this.status.textContent=error.message;}
      finally{if(version===this.version){this.more.disabled=false;this.input.setAttribute('aria-busy','false');}}
    }
    render(active=0){
      this.list.replaceChildren();
      for(const [index,spell]of this.items.entries()){
        const row=document.createElement('button');row.type='button';row.className='spell-result';row.id='spell-result-'+index;row.dataset.spellIndex=index;
        row.setAttribute('role','option');row.tabIndex=-1;
        const title=document.createElement('strong');title.textContent=spell.name;
        const id=document.createElement('span');id.className='spell-id';id.textContent=spell.id;
        const meta=document.createElement('small');meta.textContent=[...spell.classes,spell.aura?'Aura':'Spell'].join(' · ');
        row.append(title,id,meta);
        if(spell.description){const text=document.createElement('span');text.className='spell-description';text.textContent=spell.description;row.append(text);}
        this.list.append(row);
      }
      this.highlight(this.items.length?active:-1);
    }
    highlight(index){
      this.active=this.items.length?Math.max(0,Math.min(this.items.length-1,index)):-1;
      for(const [i,row]of [...this.list.children].entries())row.setAttribute('aria-selected',String(i===this.active));
      if(this.active>=0){const row=this.list.children[this.active];this.input.setAttribute('aria-activedescendant',row.id);this.list.setAttribute('aria-activedescendant',row.id);row.scrollIntoView({block:'nearest'});}
      else{this.input.removeAttribute('aria-activedescendant');this.list.removeAttribute('aria-activedescendant');}
    }
    choose(index){const spell=this.items[index];if(!spell||!this.chooseSpell)return;if(this.chooseSpell(spell)!==false)this.dialog.close();}
  }
  root.OverlaySpellPicker=SpellPicker;
})(window);
