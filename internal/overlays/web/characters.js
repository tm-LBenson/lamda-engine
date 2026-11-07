/* Character identities are local to this engine/client. Never guess among realms. */
window.OverlayCharacters = class {
  constructor(context, notify) { this.context=context; this.notify=notify; this.known=[]; }
  key(c) { return JSON.stringify([String(c.character||'').trim().toLowerCase(),String(c.realm||'').trim().toLowerCase().replace(/[\s-]/g,'')]); }
  label(c) { return c.character ? `${c.character} — ${c.realm||'Any realm'}` : c.realm ? `Any character — ${c.realm}` : 'Shared'; }
  list() {
    const {character,images=[],rules=[]}=this.context(), found=new Map();
    for(const c of [...rules.map(r=>r.load),...this.known,...images,character]) {
      if(c?.character?.trim()&&c?.realm?.trim()) found.set(this.key(c),{character:c.character,realm:c.realm});
    }
    return [...found.values()].sort((a,b)=>(a.character+' '+a.realm).localeCompare(b.character+' '+b.realm));
  }
  async load() {
    try { const r=await fetch('/api/overlays/characters');if(r.ok){const data=await r.json();this.known=Array.isArray(data.characters)?data.characters:[];} }
    catch (_) {} // Manual entry and the current character remain available offline.
  }
  mount(host, prefix, value, changed, allowAny=false) {
    const directory=this;
    host.innerHTML=`<div class="character-choice"><label>Character<select id="${prefix}-pick" aria-label="${allowAny?'Load character':'Screenshot character'}"></select></label><button id="${prefix}-current" class="button" type="button">Use current character</button></div><div id="${prefix}-manual" class="two-fields character-manual" hidden><label>Name<input id="${prefix}-character" maxlength="80" autocomplete="off"></label><label>Realm<input id="${prefix}-realm" maxlength="80" autocomplete="off"></label></div>`;
    const select=host.querySelector('select'), name=host.querySelector(`#${prefix}-character`), realm=host.querySelector(`#${prefix}-realm`), manual=host.querySelector('.character-manual');
    let entries=[];
    const get=()=>({character:name.value.trim(),realm:realm.value.trim()});
    function options() {
      entries=directory.list();
      select.replaceChildren(new Option(allowAny?'Any character':'Choose character…',''),...entries.map(c=>new Option(`${c.character} — ${c.realm}`,directory.key(c))),new Option('Enter manually…','manual'));
    }
    function set(c) {
      name.value=c.character||'';realm.value=c.realm||'';
      const matches=entries.filter(i=>i.character.toLowerCase()===name.value.trim().toLowerCase()&&(!realm.value||directory.key(i)===directory.key(c)));
      if(matches.length===1 && (!allowAny||realm.value)) {name.value=matches[0].character;realm.value=matches[0].realm;select.value=directory.key(matches[0]);}
      else select.value=name.value||realm.value?'manual':'';
      manual.hidden=select.value!=='manual';
    }
    select.addEventListener('change',()=>{
      manual.hidden=select.value!=='manual';
      if(select.value==='manual'){name.focus();return;}
      const c=entries.find(i=>directory.key(i)===select.value)||{};name.value=c.character||'';realm.value=c.realm||'';changed(get());
    });
    const currentButton=host.querySelector('button');
    const availability=()=>{const c=directory.context().character;currentButton.disabled=!(c?.character&&c?.realm);};
    currentButton.addEventListener('click',()=>{
      const c=directory.context().character;
      if(!c?.character||!c?.realm){directory.notify('Open WoW and enable LamdaOverlays to use the current character.');return;}
      options();set(c);changed(get());
    });
    name.addEventListener('change',()=>changed(get()));
    realm.addEventListener('change',()=>changed(get()));
    options();set(value||{});availability();
    return {get,set,availability,editing:()=>select.value==='manual',edit(){select.value='manual';manual.hidden=false;},refresh(){availability();const c=get(),editing=select.value==='manual';options();if(editing){select.value='manual';manual.hidden=false;}else set(c);}};
  }
};
