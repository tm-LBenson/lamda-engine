/* Local media library. Rule screenshots are never read by this module. */
(function(root){
  'use strict';
  class OverlayMedia {
    constructor({getRules,onChange,notify}){
      this.getRules=getRules;this.onChange=onChange;this.notify=notify;this.library={revision:0,assets:[]};this.selection=null;this.serial=0;
      const dialog=document.createElement('dialog');dialog.id='media-dialog';dialog.className='media-dialog';dialog.innerHTML='<div class="media-heading"><h2>Media</h2><button id="media-close" class="button">Close</button></div><div id="media-list"></div><p id="media-status" role="status"></p><button id="media-upload" class="button">Upload…</button><input id="media-file" type="file" accept="image/png,image/jpeg,audio/wav,.wav" hidden>';
      document.body.append(dialog);this.dialog=dialog;
      dialog.querySelector('#media-close').onclick=()=>dialog.close();dialog.addEventListener('close',()=>{this.serial++;this.selection=null;});
      dialog.querySelector('#media-upload').onclick=()=>dialog.querySelector('#media-file').click();
      dialog.querySelector('#media-file').onchange=e=>this.upload(e);
      dialog.querySelector('#media-list').addEventListener('click',e=>this.action(e));
      dialog.querySelector('#media-list').addEventListener('change',e=>this.rename(e));
    }
    async request(path='',data){
      const response=await fetch('/api/overlays/media'+path,data?{method:'POST',headers:{'Content-Type':'application/json','X-Lamda-Sim':'1'},body:JSON.stringify(data)}:{}),value=await response.json();if(!response.ok)throw Error(value.error||'Media unavailable');return value;
    }
    async refresh(){this.library=await this.request();this.onChange(this.library);if(this.dialog.open)this.render();return this.library;}
    async open(kind=null,select=null){
      this.selection=select;this.kind=kind;this.serial++;this.dialog.querySelector('#media-status').textContent='';this.dialog.querySelector('#media-file').accept=kind==='image'?'image/png,image/jpeg':kind==='sound'?'audio/wav,.wav':'image/png,image/jpeg,audio/wav,.wav';
      this.render();if(!this.dialog.open)this.dialog.showModal();try{await this.refresh();}catch(e){this.status(e.message);}
    }
    status(text){this.dialog.querySelector('#media-status').textContent=text;}
    render(){
      const host=this.dialog.querySelector('#media-list');host.replaceChildren();
      for(const asset of this.library.assets.filter(a=>!this.kind||a.kind===this.kind)){
        const row=document.createElement('div');row.className='media-row';row.dataset.id=asset.id;
        if(asset.kind==='image'&&asset.available){const img=document.createElement('img');img.src='/api/overlays/media/file?id='+asset.id;img.alt='';row.append(img);}else{const icon=document.createElement('span');icon.className='media-symbol';icon.textContent=asset.kind==='sound'?'♪':'▧';row.append(icon);}
        const name=document.createElement('input');name.value=asset.name;name.maxLength=80;name.dataset.rename=asset.id;name.setAttribute('aria-label','Media name');row.append(name);
        if(this.selection){const choose=document.createElement('button');choose.className='button';choose.textContent=asset.available?'Use':'Missing';choose.disabled=!asset.available;choose.dataset.choose=asset.id;row.append(choose);}
        const remove=document.createElement('button');remove.className='button media-delete';remove.textContent='Delete';remove.dataset.remove=asset.id;remove.disabled=root.OverlayModel.mediaReferences(this.getRules()).includes(asset.id);row.append(remove);host.append(row);
      }
      if(!host.children.length){const empty=document.createElement('p');empty.textContent='No media yet.';host.append(empty);}
      if(this.library.error)this.status(this.library.error);
    }
    async upload(event){
      const file=event.target.files[0];event.target.value='';if(!file)return;const serial=this.serial;this.status('Uploading…');
      try{
        if(file.size>8*1024*1024)throw Error('Choose a file under 8 MB.');
        const mime=/\.wav$/i.test(file.name)?'audio/wav':file.type;if(!['image/png','image/jpeg','audio/wav'].includes(mime))throw Error('Choose a PNG, JPG or WAV.');
        const data=await file.arrayBuffer(),hash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',data))].map(b=>b.toString(16).padStart(2,'0')).join('');
        if(serial!==this.serial)return;
        const response=await fetch('/api/overlays/media?'+new URLSearchParams({revision:this.library.revision,name:file.name.replace(/\.[^.]+$/,'').slice(0,80)}),{method:'POST',headers:{'Content-Type':mime,'X-Lamda-Sim':'1'},body:data});
        const lib=await response.json();if(!response.ok)throw Error(lib.error||'Upload failed');this.library=lib;this.onChange(lib);
        if(serial!==this.serial)return;this.status('');this.render();const asset=lib.assets.find(a=>a.id===hash);
        if(this.selection&&asset&&(!this.kind||asset.kind===this.kind)){this.selection(asset);this.dialog.close();}
      }catch(e){if(serial===this.serial)this.status(e.message);}
    }
    async action(event){
      const button=event.target.closest('button');if(!button)return;
      if(button.dataset.choose){const asset=this.library.assets.find(a=>a.id===button.dataset.choose);if(asset&&this.selection){this.selection(asset);this.dialog.close();}return;}
      if(!button.dataset.remove)return;
      if(button.dataset.confirm!=='yes'){button.dataset.confirm='yes';button.textContent='Confirm';return;}
      button.disabled=true;
      try{this.library=await this.request('/edit',{revision:this.library.revision,id:button.dataset.remove,remove:true});this.onChange(this.library);this.status('');this.render();}catch(e){this.status(e.message);button.disabled=false;}
    }
    async rename(event){
      const el=event.target,id=el.dataset.rename;if(!id)return;
      try{this.library=await this.request('/edit',{revision:this.library.revision,id,name:el.value});this.onChange(this.library);this.status('');this.render();}catch(e){this.status(e.message);}
    }
    async exportAssets(rules){
      const ids=root.OverlayModel.mediaReferences(rules);if(!ids.length)return [];
      await this.refresh();const out=[];let bytes=0;
      for(const id of ids){
        const asset=this.library.assets.find(a=>a.id===id&&a.available);if(!asset)throw Error('This overlay is missing media. Add the file before sharing.');
        const response=await fetch('/api/overlays/media/file?id='+id);if(!response.ok)throw Error('Media unavailable: '+asset.name);
        const buffer=await response.arrayBuffer();bytes+=buffer.byteLength;if(bytes>12*1024*1024)throw Error('Keep exported media under 12 MB.');
        const hash=[...new Uint8Array(await crypto.subtle.digest('SHA-256',buffer))].map(b=>b.toString(16).padStart(2,'0')).join('');if(hash!==id)throw Error('Media changed on disk: '+asset.name);
        const data=new Uint8Array(buffer);let binary='';for(let i=0;i<data.length;i+=8192)binary+=String.fromCharCode(...data.subarray(i,i+8192));out.push({id,name:asset.name,data:btoa(binary)});
      }
      return out;
    }
    async importAssets(assets){
      if(!assets.length)return;
      await this.refresh();this.library=await this.request('/import',{revision:this.library.revision,assets});this.onChange(this.library);
    }
  }
  root.OverlayMedia=OverlayMedia;
})(window);
