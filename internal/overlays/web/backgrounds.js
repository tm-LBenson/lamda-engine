/* Screenshots are local preview data, separate from rules and their Load filters. */
window.OverlayBackgrounds = class {
  constructor(context, changed, notify, characters) {
    this.context=context;this.changed=changed;this.notify=notify;this.characters=characters;
    this.library={revision:0,images:[]};this.choice='auto';this.key='';this.width=1920;this.height=1080;
    this.$=id=>document.getElementById(id);
    try {this.choice=localStorage.getItem('lamda-overlays-background')||'auto';}catch(_){}
    this.$('background-select').addEventListener('change',e=>{this.choice=e.target.value;this.remember();this.refresh();});
    this.$('screenshot-open').addEventListener('click',()=>this.open());
    this.$('screenshot-cancel').addEventListener('click',()=>this.$('screenshot-dialog').close());
    this.$('screenshot-form').addEventListener('submit',e=>{e.preventDefault();this.upload();});
    this.$('screenshot-remove').addEventListener('click',()=>this.remove());
    this.$('expand-preview').addEventListener('click',async()=>{
      try {if(document.fullscreenElement)await document.exitFullscreen();else await this.$('preview-stage').requestFullscreen();}
      catch(_){this.notify('Fullscreen is unavailable in this browser.');}
    });
    document.addEventListener('fullscreenchange',()=>{
      const b=this.$('expand-preview');b.textContent=document.fullscreenElement?'⤡':'⤢';b.setAttribute('aria-label',document.fullscreenElement?'Exit full screen':'Expand preview');this.changed();
    });
  }
  remember(){try{localStorage.setItem('lamda-overlays-background',this.choice);}catch(_){}}
  normalize(value){return String(value||'').trim().toLowerCase();}
  realm(value){return this.normalize(value).replace(/[\s-]/g,'');}
  match(character,realm){
    const matches=this.library.images.filter(i=>this.normalize(i.character)===this.normalize(character)&&(!realm||this.realm(i.realm)===this.realm(realm)));
    return matches.length===1?matches[0]:null;
  }
  identity(){const {load,character}=this.context();return load?.character?{character:load.character,realm:load.realm||''}:character||{};}
  selected(){if(this.choice==='sample')return null;if(this.choice==='auto'){const c=this.identity();return this.match(c.character,c.realm);}return this.library.images.find(i=>i.id===this.choice)||null;}
  async request(path,options){const r=await fetch('/api/overlays/'+path,options);const result=await r.json();if(!r.ok){if(r.status===409)await this.load();throw Error(result.error||'Screenshot unavailable.');}return result;}
  async load(){this.library=await this.request('backgrounds');this.options();this.refresh();}
  options(){
    if(!['auto','sample'].includes(this.choice)&&!this.library.images.some(i=>i.id===this.choice))this.choice='auto';
    this.$('background-select').replaceChildren(...[['auto','Current character'],['sample','Sample UI'],...this.library.images.map(i=>[i.id,`${i.character} — ${i.realm}`])].map(([value,label])=>new Option(label,value,false,value===this.choice)));
  }
  refresh(){
    const selected=this.selected(),key=selected?selected.id+selected.image:'sample';
    if(this.key===key)return;
    this.key=key;
    const image=this.$('scene-background'),scene=this.$('scene');
    image.hidden=true;scene.classList.remove('has-background');
    this.width=selected?.width||1920;this.height=selected?.height||1080;
    this.applyDimensions();
    if(!selected){image.removeAttribute('src');this.changed();return;}
    image.onload=()=>{if(this.key!==key)return;this.width=image.naturalWidth;this.height=image.naturalHeight;this.applyDimensions();image.hidden=false;scene.classList.add('has-background');this.changed();};
    image.onerror=()=>{if(this.key!==key)return;this.choice='sample';this.remember();this.options();this.refresh();this.notify('Screenshot unavailable. Upload it again.');};
    image.src='/api/overlays/background?'+new URLSearchParams({id:selected.id,version:selected.image});
    this.changed();
  }
  applyDimensions(){this.$('scene').style.aspectRatio=`${this.width} / ${this.height}`;this.$('preview-stage').style.setProperty('--screen-ratio',this.width/this.height);}
  open(){
    const c=this.selected()||this.identity();
    this.picker=this.characters.mount(this.$('screenshot-identity'),'screenshot',c,()=>this.updateRemove());
    this.characters.load().then(()=>{if(this.$('screenshot-dialog').open&&!this.uploading)this.picker.refresh();});
    this.$('screenshot-file').value='';this.$('screenshot-error').textContent='';this.updateRemove();this.$('screenshot-dialog').showModal();
  }
  updateRemove(){this.$('screenshot-remove').hidden=!this.match(this.$('screenshot-character').value,this.$('screenshot-realm').value)||!this.$('screenshot-realm').value.trim();}
  busy(value){this.uploading=value;for(const el of this.$('screenshot-form').elements)el.disabled=value;if(!value)this.picker?.availability();}
  async upload(){
    const file=this.$('screenshot-file').files[0];
    if(!file){this.$('screenshot-error').textContent='Choose a screenshot.';return;}
    if(!['image/png','image/jpeg'].includes(file.type)||file.size>32*1024*1024){this.$('screenshot-error').textContent='Choose a PNG or JPG screenshot smaller than 32 MB.';return;}
    const character=this.$('screenshot-character').value.trim(),realm=this.$('screenshot-realm').value.trim();
    if(!character||!realm){this.$('screenshot-error').textContent='Choose a character, or enter a name and realm.';return;}
    this.busy(true);
    try{
      this.library=await this.request('backgrounds?'+new URLSearchParams({character,realm,revision:this.library.revision}),{method:'POST',headers:{'Content-Type':file.type,'X-Lamda-Sim':'1'},body:file});
      this.choice=this.match(character,realm).id;this.remember();this.options();this.refresh();this.$('screenshot-dialog').close();this.notify('Screenshot saved.');
    }catch(error){this.$('screenshot-error').textContent=error.message;}finally{this.busy(false);this.updateRemove();}
  }
  async remove(){
    const item=this.match(this.$('screenshot-character').value,this.$('screenshot-realm').value);if(!item)return;
    this.busy(true);
    try{
      this.library=await this.request('backgrounds/remove',{method:'POST',headers:{'Content-Type':'application/json','X-Lamda-Sim':'1'},body:JSON.stringify({id:item.id,revision:this.library.revision})});
      this.options();this.remember();this.refresh();this.$('screenshot-dialog').close();this.notify('Screenshot removed.');
    }catch(error){this.$('screenshot-error').textContent=error.message;}finally{this.busy(false);this.updateRemove();}
  }
};
