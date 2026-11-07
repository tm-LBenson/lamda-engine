(async function () {
  'use strict';
  const M = window.OverlayModel, $ = s => document.querySelector(s), $$ = s => [...document.querySelectorAll(s)];
  const esc = value => String(value).replace(/[&<>"']/g, c => ({ '&':'&amp;', '<':'&lt;', '>':'&gt;', '"':'&quot;', "'":'&#39;' }[c]));
  let backgrounds, characters, loadPicker, loadPickerRule, media;
  let revision=0,live=null,view='rules',connectionError='';
 const draft=()=>M.newRule('My overlay',[M.node('when',{event:'state'},[M.node('show',{display:'text',text:'My reminder',duration:6})])]);
  const eventNames = {talents:'Talents change',equipment:'Equipment changes',...M.bossEvents,...M.encounterEvents,combatlog:'Combat event',...M.castEvents,casting:'Cast changes', cooldown:'Cooldown changes',state:'Conditions become true',login:'Login / reload',combat:'Enter combat',combatend:'Leave combat', ability:'An ability is used', health:'Health changes', resource:'Resource changes', enter:'Enter an instance', key:'A Mythic+ key starts', aura:'A buff or debuff changes', leave:'Leave the instance', keyend:'The Mythic+ key ends' };
  const comparisons = { gte:'at least ≥', gt:'above >', eq:'exactly =', ne:'not equal ≠', lt:'below <', lte:'at most ≤' };
  const colors = { gold:'#f4c879', teal:'#65ddc7', violet:'#c6a5ee', red:'#ef9b9b' };
  let rules = [draft()], active, selected = null, history = [], future = [], drag = null, timerIds = [], runSerial = 0, running = false, displays = new Map(), lastTrace = [], saved = true, toastTimer, audioContext;
  const closedActions=new Set();
  let pendingPackUpdates=new Set();
  let testSession = M.createSession(), modifierEditor=null, loadEditor=null;
  const loadEditing=()=>loadEditor===active&&view==='load';
  const retiredDisplays=new Map(),renderedDisplays=new Map();
  const modifierSoundLatches=new Map();const previewVisibility=new Map(),groupTestSessions=new Map();let previewBounds=null,previewPlacement=[];
  const collapsedKey='lamda.overlays.collapsed-characters';
  let collapsedGroups=new Set(), librarySelection='';
  try { const stored=JSON.parse(localStorage.getItem(collapsedKey)||'[]');if(Array.isArray(stored))collapsedGroups=new Set(stored.filter(k=>typeof k==='string')); } catch (_) {}
  function saveCollapsedGroups(){try{localStorage.setItem(collapsedKey,JSON.stringify([...collapsedGroups]));}catch(_) {}}
  async function api(data) {
    const response=await fetch('/api/overlays',data?{method:'POST',headers:{'Content-Type':'application/json','X-Lamda-Sim':'1'},body:JSON.stringify(data)}:{});
    const result=await response.json();if(!response.ok)throw Error(result.error||'Engine unavailable');return result;
  }
  try { const state=await api();revision=state.revision;live=state;rules=state.rules; }
  catch(error){connectionError=error.message;saved=false;}

  for(const r of rules)for(const n of M.Actions.definitions(r))closedActions.add(n.id);
  active = rules[0]?.id||null;
  const current = () => rules.find(r => r.id === active) || rules[0];
  const snapshot = () => ({ rules:M.clone(rules), active, selected, packUpdates:[...pendingPackUpdates] });
  media=new window.OverlayMedia({getRules:()=>rules,notify:message=>notify(message),onChange:library=>{live||={};live.media=library;render();}});
  media.library=live?.media||{revision:0,assets:[]};
  function soundChoices(value){
    const choices={chime:'Chime',alert:'Alert',soft:'Soft tone',...(live?.sonar?{sonar:'Sonar'}:{})};
    for(const a of media?.library.assets||[])if(a.kind==='sound')choices['media:'+a.id]=a.name+(a.available?'':' (missing)');
    if(value&&!Object.hasOwn(choices,value))choices[value]=value==='sonar'?'Sonar':'Missing sound';return choices;
  }

  function notify(message) { clearTimeout(toastTimer); $('#toast').textContent = message; $('#toast').hidden = false; toastTimer = setTimeout(() => { $('#toast').hidden = true; }, 3400); }
  function status() { $('#save-status').textContent = connectionError || (saved ? 'Saved' : 'Unsaved changes'); $('#undo').disabled = !history.length; $('#redo').disabled = !future.length; }
  function remember() { history.push(snapshot()); if (history.length > 60) history.shift(); future = []; saved = false; }
  function mutate(fn) {
    const before = snapshot();
    try { fn(); M.Packs.reconcile(before.rules,rules); if (!M.validRules(rules,true)) throw Error('That value is outside the supported limits.'); }
    catch (error) { rules = before.rules; active = before.active; selected = before.selected; pendingPackUpdates=new Set(before.packUpdates||[]); notify(error.message); render(); return false; }
    for(const id of pendingPackUpdates)if(!before.packUpdates.includes(id))before.packUpdates.push(id);
    history.push(before); if (history.length > 60) history.shift(); future = []; saved = false; resetSession(); render(); return true;
  }
  function restore(s) { rules = s.rules; active = s.active; selected = s.selected; pendingPackUpdates=new Set(s.packUpdates||[]); saved = false; resetSession(); render(); }
  function stepHistory(from,to) { if(!from.length)return;const next=from.pop(),before=snapshot(),updates=[...new Set([...(next.packUpdates||[]),...before.packUpdates])];next.packUpdates=updates;before.packUpdates=[...updates];to.push(before);restore(next); }
  function undo() { stepHistory(history,future); }
  function redo() { stepHistory(future,history); }
  async function save() {
    const missing=rules.find(r=>r.enabled&&r.style.arc?.repeat==='stacks'&&!r.style.arc.aura&&M.flatten(r.roots).some(n=>n.type==='show'&&['arc','ring'].includes(n.props.display)));
    if(missing){active=missing.id;selected=M.flatten(missing.roots).find(n=>n.type==='show'&&['arc','ring'].includes(n.props.display)).id;render();$('#tab-style').click();$('#arch-aura').focus();$('#arch-aura').reportValidity();return;}
    const payload=M.clone(rules),packUpdates=[...pendingPackUpdates].filter(id=>live?.rules?.some(r=>r.pack?.instance===id)&&payload.some(r=>r.pack?.instance===id));$('#save').disabled=true;
    try {const state=await api({revision,rules:payload,packUpdates});revision=state.revision;live=state;saved=JSON.stringify(rules)===JSON.stringify(payload);for(const id of pendingPackUpdates)if(JSON.stringify(rules.filter(r=>r.pack?.instance===id))===JSON.stringify(payload.filter(r=>r.pack?.instance===id)))pendingPackUpdates.delete(id);connectionError='';status();valuesEditor.render();updateLive();notify(state.publishError||(state.live?.status?.startsWith('Reload WoW')?'Saved. '+state.live.status+'.':'Saved to Lamda Engine.'));}
    catch(error){connectionError=error.message;status();notify(error.message);}
    finally{$('#save').disabled=false;}
  }
  const savedMemory=new window.OverlaySavedMemory(()=>({revision,saved,current:live?.live?.character,characters}),notify);
  const packHistory=new window.OverlayPackHistory({rules:()=>rules,notify,apply:plan=>{const ok=mutate(()=>{rules=plan.rules;pendingPackUpdates.add(plan.instance);active=plan.first;selected=null;});if(ok){selectRuleSample();changeView('rules');}return ok;}});
  const packEditor=new window.OverlayPackEditor({host:$('#pack-panel'),current,rules:()=>rules,mutate,backups:()=>packHistory.open(current()?.pack?.instance||''),select:id=>{active=id;selected=null;resetSession();selectRuleSample();render();},export:()=>{$('#export-open').click();$('#export-scope').value='pack';updateExport();}});
  const valuesEditor=new window.OverlayValueEditor({host:$('#values-panel'),getRule:current,getPackMembers:()=>M.Packs.members(rules,current()),shareOption:(key,shared)=>{let next=key;mutate(()=>{next=M.Packs.shareOption(rules,current(),key,shared);valuesEditor.author=next;});return next;},onMemory:(host,rule,key)=>savedMemory.mount(host,rule,key),whenFields:rememberWhenFields,changeWhen:changeWhen,changeRule:mutate,sample:()=>sampleState(),notify,summary:predicateSummary,defaults:predicateDefaults,onCheck:()=>{renderAppearance();if(valuesEditor.check())$('#tab-style').click();},search:options=>spellPicker.open({...options,prefer:current()?.load?.class||live?.live?.character?.class||''})});
  function rememberWhenFields(n){const event=n.props.event;return `<div class="expression-fields">${fields(n).replace('A buff or debuff changes','Buff condition becomes true').replace('Cooldown changes','Cooldown condition becomes true').replace('Cast changes','Cast condition becomes true')}</div>`+(event==='combatlog'?`<details><summary>Filters</summary>${combatFilters(n)}</details>`:Object.hasOwn(M.bossEvents,event)?bossFilters(n):Object.hasOwn(M.encounterEvents,event)?encounterFilters(n):Object.hasOwn(M.castEvents,event)||event==='casting'?castSpell(n):'');}
  function changeWhen(p,key,value){
    if(key==='event'){for(const k of Object.keys(p))delete p[k];p.event=value;
      if(Object.hasOwn(M.bossEvents,value))Object.assign(p,{provider:'auto',spell:'',textFilter:'',timerKind:'any',timelineSource:'any',encounterID:''});
      if(Object.hasOwn(M.encounterEvents,value))Object.assign(p,{encounterID:'',difficulty:'',outcome:''});
      if(value==='combatlog')Object.assign(p,{combatEvent:'damage',sourceUnit:'player',targetUnit:'any',spell:'',extraSpell:'',school:'any',critical:'any',periodic:'any',auraKind:'any',miss:'any'});
      return;
    }
    p[key]=value.trim();if(key==='provider')delete p.timelineSource;
    if(key==='combatEvent')for(const k of ['extraSpell','school','critical','periodic','auraKind','miss'])delete p[k];
  }
  function valueCheckEditing(){return view==='values'&&!!valuesEditor.check();}
  function conditionTypes(){if(loadEditing())return {talent:'Talent',learned:'Learned spell',equipment:'Equipment',group:'Conditions'};const types={...predicateTypes};if(valueCheckEditing()||!current()?.logic?.values?.length)delete types.value;return types;}
  function valueConditionFields(n){
    const p=n.props,l=current().logic||{},values=Object.fromEntries((l.values||[]).map(v=>[v.key,v.name])),options=Object.fromEntries((l.options||[]).map(o=>[o.key,o.name]));
    let html=choice(n,'key',values,'Named value')+choice(n,'operator',{lt:'<',lte:'≤',eq:'=',ne:'≠',gt:'>',gte:'≥',true:'Is true',false:'Is false'},'Value comparison');
    if(!['true','false'].includes(p.operator)){const type=p.compareTo==='value'||p.compareTo==='option'?p.compareTo:typeof p.value==='string'?'text':typeof p.value==='boolean'?'boolean':'literal';html+=choice({...n,props:{...p,compareTo:type}},'compareTo',{literal:'Number',text:'Text',boolean:'True / false',value:'Value',...(Object.keys(options).length?{option:'Setting'}:{})},'Compare with');if(type==='value'||type==='option')html+=choice(n,'other',type==='value'?values:options,'Comparison value');else if(type==='boolean')html+=choice(n,'value',{true:'True',false:'False'},'Comparison value');else if(type==='text')html+=`<input data-prop="value" data-node="${n.id}" maxlength="512" value="${esc(p.value)}" aria-label="Comparison text">`;else html+=number(n,'value',-M.Values.limit,M.Values.limit,'Comparison number','any');}
    return html;
  }
  function changeValueCondition(p,key,value){
    if(p.condition!=='value')return false;
    if(key==='compareTo'){if(['literal','text','boolean'].includes(value)){p.compareTo='literal';p.value={literal:0,text:'',boolean:true}[value];}else{p.compareTo=value;p.other=(value==='option'?current().logic.options:current().logic.values)[0].key;}return true;}
    if(key==='value'&&typeof p.value==='boolean'){p.value=value==='true';return true;}return false;
  }
  function selectNode(nodeId) { loadEditor=null;if(selected!==nodeId)modifierEditor=null; selected = nodeId; $$('.block.selected').forEach(n => n.classList.remove('selected')); const el = document.getElementById(nodeId); if (el) el.classList.add('selected'); renderSelection(); renderAppearance(); renderPreview(); if(selectedShow()||groupSelected())$('#tab-style').click(); }
  function option(value, label, chosen) { return `<option value="${esc(value)}"${String(chosen) === String(value) ? ' selected' : ''}>${esc(label)}</option>`; }
  function choice(n, prop, entries, label) { return `<select data-prop="${prop}" data-node="${n.id}" aria-label="${esc(label)}">${Object.entries(entries).map(([v,l]) => option(v,l,n.props[prop])).join('')}</select>`; }
  function number(n, prop, lo, hi, label, step = 1) { return `<input type="number" data-prop="${prop}" data-node="${n.id}" value="${n.props[prop]}" min="${lo}" max="${hi}" step="${step}" aria-label="${esc(label)}">`; }
  function castSpell(n){return `<label class="cast-spell">Spell<span class="spell-field"><input type="text" maxlength="7" data-prop="spell" data-node="${n.id}" value="${esc(n.props.spell||'')}" placeholder="Any spell" aria-label="Cast spell filter"><button type="button" class="spell-find" data-spell-target="cast" data-spell-node="${n.id}">Search</button></span></label>`;}
  function combatFilters(n){
    const field=(k,label,entries)=>`<label class="cast-spell">${label}${choice({...n,props:{...n.props,[k]:n.props[k]||'any'}},k,entries,label)}</label>`;
    const spell=(k,label)=>`<label class="cast-spell">${label}<span class="spell-field"><input data-prop="${k}" data-node="${n.id}" maxlength="7" value="${esc(n.props[k]||'')}" placeholder="Any spell" aria-label="${label} filter"><button class="spell-find" data-spell-target="combat-${k}" data-spell-node="${n.id}">Search</button></span></label>`;
    const any={any:'Any'},p=n.props;
    return `<div id="combat-filters">${field('sourceUnit','Source',{...any,...M.unitChoices})}${field('targetUnit','Target',{...any,...M.unitChoices})}${spell('spell','Spell')}${['interrupt','dispel','dispelfailed'].includes(p.combatEvent)?spell('extraSpell','Affected spell'):''}<details><summary>Filters</summary>${field('school','School',{...any,1:'Physical',2:'Holy',4:'Fire',8:'Nature',16:'Frost',32:'Shadow',64:'Arcane'})}${['damage','heal'].includes(p.combatEvent)?field('critical','Critical',{...any,yes:'Yes',no:'No'}):''}${['damage','heal','energize','miss'].includes(p.combatEvent)?field('periodic','Periodic',{...any,yes:'Yes',no:'No'}):''}${p.combatEvent.startsWith('aura')||['dispel','dispelfailed'].includes(p.combatEvent)?field('auraKind','Aura type',{...any,buff:'Buff',debuff:'Debuff'}):''}${p.combatEvent==='miss'?field('miss','Outcome',{...any,ABSORB:'Absorb',BLOCK:'Block',DEFLECT:'Deflect',DODGE:'Dodge',EVADE:'Evade',IMMUNE:'Immune',MISS:'Miss',PARRY:'Parry',REFLECT:'Reflect',RESIST:'Resist'}):''}</details></div>`;
  }
  const bossStageProviders={auto:'Automatic',dbm:'DBM',bigwigs:'BigWigs'};
  function equipmentChange(p,key,value){
    if(['slot','itemClass','itemSubclass'].includes(key))p[key]=+value;
    if(key==='itemClass')p.itemSubclass=-1;
    if(key==='source'){
      Object.assign(p,{slot:value==='slot'?13:0,check:value==='slot'?'occupied':value==='set'?'count':'equipped',operator:'gte',value:value==='set'?2:600});
      p.id||='5512';p.name||='My outfit';p.itemClass=2;p.itemSubclass=-1;
    }
    if(key==='check'&&value==='level'){p.operator='gte';p.value=600;}
  }
  function equipmentOptions(n){
    const p=n.props,slots=live?.live?.character?.equipment?.slots||[];
    const choices=slots.filter(s=>p.source==='set'?s.set>0:s.item>0);
    return (['item','type'].includes(p.source)?`<label class="cast-spell">Slot${choice({...n,props:{...p,slot:p.slot||0}},'slot',{0:'Any slot',...M.equipmentSlots},'Equipment slot filter')}</label>`:'')+(p.source==='type'?`<label class="cast-spell">Type${choice(n,'itemSubclass',M.equipmentTypes[p.itemClass]||{'-1':'Any'},'Equipment subtype')}</label>`:'')+(['item','set'].includes(p.source)?`<label class="cast-spell">Equipped<select data-equipped="${n.id}" aria-label="Choose equipped item" ${choices.length?'':'disabled'}><option value="">${choices.length?'Choose an item…':'No equipment data'}</option>${choices.map(s=>option(s.slot,`${M.equipmentSlots[s.slot]} · ${s.name||'Item '+s.item}${p.source==='set'?' · Set '+s.set:''}`,'')).join('')}</select></label>`:'');
  }
  function talentOptions(n){return n.props.condition==='talent'?`<label class="cast-spell">Type${choice(n,'source',M.talentKinds,'Talent type')}</label>${n.props.source==='hero'?`<button class="button" data-current-hero="${n.id}" ${live?.live?.character?.heroSpec?'':'disabled'}>Use current hero tree</button>`:''}`:'';}
  function bossFilters(n){
    const p=n.props,stage=p.condition==='bossStage'||p.event==='bossstage',provider=p.provider||'auto';
    const field=(key,label,values)=>`<label class="cast-spell">${label}${choice({...n,props:{...p,[key]:p[key]||Object.keys(values)[0]}},key,values,label)}</label>`;
    return `<div class="boss-filters">${field('provider','Boss provider',stage?{...bossStageProviders,...(provider==='blizzard'?{blizzard:'Blizzard'}:{})}:M.bossProviders)}${stage?`<label class="cast-spell">Encounter ID<input data-prop="encounterID" data-node="${n.id}" maxlength="7" inputmode="numeric" value="${esc(p.encounterID||'')}" placeholder="Any encounter" aria-label="Boss encounter ID"></label>`:`<label class="cast-spell">Spell<span class="spell-field"><input data-prop="spell" data-node="${n.id}" maxlength="7" inputmode="numeric" value="${esc(p.spell||'')}" placeholder="Any spell" aria-label="Boss spell filter"><button class="spell-find" data-spell-target="boss" data-spell-node="${n.id}">Search</button></span></label><details><summary>Filters</summary><label class="cast-spell">Text contains<input data-prop="textFilter" data-node="${n.id}" maxlength="80" value="${esc(p.textFilter||'')}" aria-label="Boss text filter"></label>${p.event==='bosswarning'?'':field('timerKind','Timer kind',M.bossKinds)+(provider==='blizzard'?field('timelineSource','Timeline source',M.timelineSources):'')}</details>`}</div>`;
  }

  function encounterFilters(n){
    const p=n.props,difficulties={...M.difficulties,...(p.difficulty&&!Object.hasOwn(M.difficulties,p.difficulty)?{[p.difficulty]:'Difficulty '+p.difficulty}:{})};
    const field=(key,label,values)=>`<label class="cast-spell">${label}${choice({...n,props:{...p,[key]:p[key]||''}},key,values,label)}</label>`;
    return `<div class="encounter-filters"><label class="cast-spell">Encounter ID<input data-prop="encounterID" data-node="${n.id}" maxlength="7" inputmode="numeric" value="${esc(p.encounterID||'')}" placeholder="Any encounter" aria-label="Encounter ID filter"></label>${field('difficulty','Difficulty',difficulties)}${p.event==='encounterend'?field('outcome','Outcome',{'':'Any',victory:'Victory',wipe:'Wipe'}):''}</div>`;
  }
  function fields(n,expanded=false) {
    switch (n.type) {
      case 'action': return `<input data-prop="name" data-node="${n.id}" maxlength="60" value="${esc(n.props.name)}" aria-label="Action name">`;
      case 'call': return choice(n,'action',Object.fromEntries(M.callChoices(current(),n.id).map(d=>[d.id,d.props.name])),'Action to run');
      case 'when': return choice(n,'event',eventNames,'When event')+(n.props.event==='combatlog'?choice(n,'combatEvent',M.combatKinds,'Combat event'):'')+((Object.hasOwn(M.castEvents,n.props.event)||n.props.event==='casting')?choice({...n,props:{...n.props,unit:n.props.unit||'player'}},'unit',M.unitChoices,'Cast unit'):'');
      case 'if': {
        let value;
        if(n.props.condition==='value')value=valueConditionFields(n);
        else if(n.props.condition==='group')value=choice(n,'mode',{all:'All (AND)',any:'Any (OR)',not:'Not (NOT)'},'Match conditions')+`<span class="block-phrase">${n.props.tests.length} checks</span>`;
        else if (n.props.condition === 'role') value = choice(n,'value',{tank:'Tank',healer:'Healer',damage:'Damage'},'Required player role');
        else if (n.props.condition === 'location') value = choice(n,'value',{dungeon:'Dungeon',raid:'Raid',world:'Open world'},'Required location');
        else if(n.props.condition==='equipment'){
          const p=n.props;value=choice(n,'source',M.equipmentKinds,'Equipment source');
          if(['item','set'].includes(p.source))value+=`<input type="text" maxlength="7" value="${esc(p.id)}" data-prop="id" data-node="${n.id}" aria-label="${p.source==='set'?'Item set ID':'Equipped item ID'}" placeholder="ID">`;
          if(p.source==='saved')value+=`<input type="text" maxlength="120" value="${esc(p.name)}" data-prop="name" data-node="${n.id}" aria-label="Outfit name">`;
          if(p.source==='slot')value+=choice(n,'slot',M.equipmentSlots,'Equipment slot');
          if(p.source==='type')value+=choice(n,'itemClass',{2:'Weapon',4:'Armor'},'Equipment class');
          if(p.source!=='set')value+=choice(n,'check',p.source==='slot'?{occupied:'Occupied',empty:'Empty',level:'Item level'}:{equipped:'Equipped',missing:'Not equipped'},'Equipment check');
          if(['count','level'].includes(p.check))value+='<span class="block-measure">'+choice(n,'operator',{gte:'≥',gt:'>',eq:'=',ne:'≠',lt:'<',lte:'≤'},'Equipment comparison')+number(n,'value',0,p.check==='count'?19:10000,p.check==='count'?'Set pieces':'Item level',p.check==='count'?1:.1)+'</span>';
          if(expanded)value+=equipmentOptions(n);
        }
        else if(['talent','learned'].includes(n.props.condition)){
          const p=n.props,learned=p.condition==='learned',search=learned||['spell','pvp'].includes(p.source);
          value=`<span class="spell-field"><input type="text" maxlength="7" value="${esc(p.id)}" data-prop="id" data-node="${n.id}" aria-label="Talent ID" placeholder="${p.source==='hero'?'Tree ID':p.source==='entry'?'Entry ID':'Spell ID'}">${search?`<button class="spell-find" data-spell-target="talent" data-spell-node="${n.id}">Search</button>`:''}</span>`+choice(n,'check',{selected:learned?'Known':'Selected',missing:learned?'Not known':'Not selected',...(!learned?{rank:'Rank'}:{})},'Talent check');
          if(p.check==='rank')value+='<span class="block-measure">'+choice(n,'operator',comparisons,'Talent comparison')+number(n,'value',0,100,'Talent rank',1)+'</span>';
          if(expanded)value+=talentOptions(n);
        }
        else if(['bossTimer','bossStage'].includes(n.props.condition)){
          const p=n.props,stage=p.condition==='bossStage';
          value=stage?'':choice(n,'check',{active:'Active',missing:'Missing',running:'Running',paused:'Paused',remaining:'Time left',count:'Count'},'Boss timer check');
          if(stage||['remaining','count'].includes(p.check))value+='<span class="block-measure">'+choice(n,'operator',{lt:'<',lte:'≤',eq:'=',ne:'≠',gt:'>',gte:'≥'},'Boss comparison')+number(n,'value',0,stage?10000:p.check==='count'?1e6:86400,'Boss threshold',.1)+(p.check==='remaining'?'<span>sec</span>':'')+'</span>';
          if(expanded)value+=bossFilters(n);
        }
        else if(n.props.condition==='pet'){
          value=choice(n,'check',{unavailable:'Missing or dead',missing:'Missing',present:'Present',alive:'Alive',dead:'Dead',family:'Family',specialization:'Specialization',creatureType:'Creature type'},'Main pet check');
          if(['family','specialization','creatureType'].includes(n.props.check))value+=`<input type="text" maxlength="80" value="${esc(n.props.value)}" data-prop="value" data-node="${n.id}" aria-label="Pet type value" placeholder="${n.props.check==='specialization'?'Ferocity / Tenacity / Cunning':n.props.check==='family'?'Wolf / Spirit Beast / …':'Beast / Undead / …'}">`;
          if(expanded)value+='<p class="muted">Your active main pet only, not Animal Companion. Type names use your game language. Unknown readings never match.</p>';
        }
        else if(n.props.condition==='encounter'){
          value=choice(n,'check',{active:'Active',idle:'Inactive',elapsed:'Elapsed time'},'Encounter check');
          if(n.props.check==='elapsed')value+='<span class="block-measure">'+choice(n,'operator',{lt:'<',lte:'≤',eq:'=',ne:'≠',gt:'>',gte:'≥'},'Encounter comparison')+number(n,'value',0,86400,'Elapsed threshold',.1)+'<span>sec</span></span>';
          if(expanded)value+=encounterFilters(n);
        }
        else if(n.props.condition==='eventValue'){value=choice(n,'field',M.combatValues,'Event value')+(n.props.field==='critical'?choice(n,'value',{1:'Yes',0:'No'},'Critical outcome'):choice(n,'operator',{lt:'<',lte:'≤',eq:'=',ne:'≠',gt:'>',gte:'≥'},'Event comparison')+number(n,'value',0,1e12,'Event threshold',.1));}
        else if(n.props.condition==='casting'){
          value=choice({...n,props:{...n.props,unit:n.props.unit||'player'}},'unit',M.unitChoices,'Cast unit')+choice(n,'check',{active:'Active',idle:'Not casting',cast:'Casting',channel:'Channeling',empower:'Empowering',interruptible:'Interruptible',protected:'Uninterruptible',remaining:'Time left'},'Cast check');
          if(n.props.check==='remaining')value+='<span class="block-measure">'+choice(n,'operator',{lt:'<',lte:'≤',eq:'=',ne:'≠',gt:'>',gte:'≥'},'Cast comparison')+number(n,'value',0,86400,'Cast time',.1)+'<span>sec</span></span>';
          if(expanded)value+=castSpell(n);
        }
        else if(n.props.condition==='cooldown') {
          const p=n.props;
          value=choice(n,'source',{spell:'Spell',item:'Item',slot:'Equipped',gcd:'Global'},'Cooldown source');
          if(p.source==='slot')value+=choice(n,'id',{13:'Upper trinket',14:'Lower trinket',16:'Main hand',17:'Off hand'},'Equipment slot');
          else if(p.source!=='gcd')value+=`<span class="spell-field"><input type="text" maxlength="7" value="${esc(p.id)}" data-prop="id" data-node="${n.id}" aria-label="${p.source==='spell'?'Cooldown spell ID':'Item ID'}" placeholder="ID">${p.source==='spell'?`<button type="button" class="spell-find" data-spell-node="${n.id}" aria-label="Find cooldown spell">Search</button>`:''}</span>`;
          value+=choice(n,'check',{ready:'Ready',cooldown:'On cooldown',usable:'Usable',unusable:'Not usable',remaining:'Time left',...(p.source==='spell'?{charges:'Charges',full:'Full charges',recharge:'Recharge'}:{})},'Cooldown check');
          if(['remaining','charges','recharge'].includes(p.check))value+='<span class="block-measure">'+choice(n,'operator',{gte:'≥',gt:'>',eq:'=',ne:'≠',lt:'<',lte:'≤'},'Cooldown comparison')+number(n,'value',0,p.check==='charges'?999:86400,'Cooldown threshold',p.check==='charges'?1:.1)+(p.check==='charges'?'':'<span class="block-phrase">sec</span>')+'</span>';
        } else if (['buff','debuff'].includes(n.props.condition)) {
          value = choice(n,'unit',M.unitChoices,'Aura unit') + `<span class="spell-field"><input type="text" maxlength="80" value="${esc(n.props.aura)}" data-prop="aura" data-node="${n.id}" aria-label="Spell ID" placeholder="Spell ID"><button type="button" class="spell-find" data-spell-node="${n.id}" aria-label="Find condition spell">Search</button></span>` + choice(n,'check',{active:'is active',missing:'is missing',stacks:'has stacks',remaining:'has time left'},'Aura check');
          if (['stacks','remaining'].includes(n.props.check)) value += '<span class="block-measure">' + choice(n,'operator',{gte:'≥',gt:'>',eq:'=',ne:'≠',lt:'<',lte:'≤'},'Aura comparison') + number(n,'value',0,n.props.check==='stacks'?999:3600,n.props.check==='stacks'?'Stack threshold':'Time remaining threshold',n.props.check==='stacks'?1:0.1) + (n.props.check==='remaining'?'<span class="block-phrase">sec</span>':'') + '</span>';
        } else if(n.props.condition==='resource') {
          value=choice(n,'resource',M.resources,'Resource')+choice(n,'operator',{...comparisons,cap:'At cap'},'Resource comparison');
          if(n.props.operator!=='cap')value+=number(n,'value',0,n.props.measure==='percent'?100:16777214,'Resource threshold',0.1)+choice(n,'measure',{amount:'Amount',percent:'%'},'Resource measurement');
        } else value = choice({...n,props:{...n.props,operator:n.props.operator||'lt'}},'operator',comparisons,'Health comparison')+number(n,'value',0,100,'Health threshold',0.1) + '<span class="block-phrase">%</span>';
        if(['health','resource'].includes(n.props.condition))value=choice({...n,props:{...n.props,unit:n.props.unit||'current'}},'unit',M.unitChoices,'Condition unit')+value;
        return choice(n,'condition',conditionTypes(),'Condition type') + value;
      }
      case 'each': return choice({...n,props:{...n.props,scope:n.props.scope||'party'}},'scope',M.cloneScopes,'Units');
      case 'show': return choice(n,'display',{icon:'Icon',text:'Text',bar:'Progress bar',arc:'Arc',ring:'Ring',shape:'Shape',image:'Image'},'Display type') + (['text','bar'].includes(n.props.display) ? `<input type="text" maxlength="90" value="${esc(n.props.text)}" data-prop="text" data-node="${n.id}" aria-label="Display text">` : '<span class="block-phrase">on screen</span>') + choice({...n,props:{...n.props,visibility:n.props.visibility||'timed'}},'visibility',{timed:'For',conditions:'While conditions are true'},'Display lifetime') + (n.props.visibility==='conditions'?'':number(n,'duration',1,120,'Display duration') + '<span class="block-phrase">sec</span>');
      case 'glow': return '<span class="block-phrase">this display</span>' + choice(n,'color',{gold:'Gold',teal:'Teal',violet:'Violet',red:'Red'},'Glow color');
      case 'sound': return choice(n,'sound',soundChoices(n.props.sound),'Notification sound')+`<button class="button media-browse" data-media-sound="${n.id}">Browse…</button>`;
      case 'wait': return number(n,'seconds',0,15,'Wait duration',.1) + '<span class="block-phrase">seconds</span>';
      case 'hide': return '<span class="block-phrase">this player’s display</span>';
      default: return '';
    }
  }
  function slot(parent, index, empty, end = false) {
    const root = !parent;
    const label = root ? (empty ? '＋ WHEN' : end ? '＋ WHEN' : '') : empty ? '＋ Drop a block' : '';
    return `<div class="drop-slot ${empty?'empty ':''}${root&&empty?'root-empty ':''}${end?'root-end':''}" data-parent="${parent||''}" data-index="${index}" aria-label="${root?'Event':'Inside container'} insertion ${index+1}"${end?' role="button" tabindex="0"':''}>${label}</div>`;
  }
  function treeList(list, parent = null) {
    if (!list.length) return slot(parent,0,true,!parent);
    let html = slot(parent,0,false);
    list.forEach((n,index) => {
      const def = M.definitions[n.type];
      html += `<div class="block ${def.family} ${def.container?'container':''} ${n.id===selected?'selected':''} ${n.type==='if'&&['buff','debuff','resource','cooldown'].includes(n.props.condition)?'aura-condition':''} ${n.type==='if'&&['buff','debuff','cooldown'].includes(n.props.condition)?'spell-condition':''} ${n.props.condition==='cooldown'?'cooldown-condition':''}" id="${n.id}" data-block="${n.id}">
        <div class="block-head" tabindex="0" role="group" draggable="true" data-drag="${n.id}" aria-label="${def.title} block" data-select="${n.id}"><button class="grip" draggable="true" data-drag="${n.id}" aria-label="Move ${def.title} block" title="Drag this block">⠿</button><span class="block-label">${def.title}</span>${fields(n)}${def.container?`<span class="block-count">${n.children.length} inside</span>`:''}${n.type==='action'?`<button class="text-button action-toggle" data-action-toggle="${n.id}" aria-expanded="${!closedActions.has(n.id)}" aria-label="${closedActions.has(n.id)?'Expand':'Collapse'} action">${closedActions.has(n.id)?'▸':'▾'}</button>`:''}</div>
        ${def.container?`<div class="block-body" ${n.type==='action'&&closedActions.has(n.id)?'hidden':''}>${treeList(n.children,n.id)}</div><div class="block-foot" ${n.type==='action'&&closedActions.has(n.id)?'hidden':''}></div>`:''}</div>`;
      html += slot(parent,index+1,false,!parent&&index===list.length-1);
    });
    return html;
  }
  function renderLibrary() {
    $('#rule-count').textContent = rules.length;
    const groups=new Map(), shared=characters.key({});
    for(const r of rules){const c=r.load||{},key=characters.key(c);if(!groups.has(key))groups.set(key,{key,label:characters.label(c),rules:[]});groups.get(key).rules.push(r);}
    const activeGroup=characters.key(current()?.load||{}), selection=active+activeGroup, changed=selection!==librarySelection;
    if(changed){collapsedGroups.delete(activeGroup);librarySelection=selection;saveCollapsedGroups();}
    $('#rule-list').innerHTML = [...groups.values()].sort((a,b)=>a.key===shared?-1:b.key===shared?1:a.label.localeCompare(b.label)).map((g,i)=>{
      const closed=collapsedGroups.has(g.key);
      return `<section class="rule-group"><button class="rule-group-toggle" data-group="${esc(g.key)}" aria-expanded="${!closed}" aria-controls="rule-group-${i}"><span class="group-chevron" aria-hidden="true">${closed?'▸':'▾'}</span><span class="group-name">${esc(g.label)}</span><span class="group-count">${g.rules.length}</span></button><div id="rule-group-${i}"${closed?' hidden':''}>${g.rules.map(r => `<button class="rule-item ${r.id===active?'active':''}" data-rule="${r.id}" aria-pressed="${r.id===active}"><span class="rule-icon" aria-hidden="true">${['⬡','♥','▤'][rules.indexOf(r)%3]}</span><span><strong>${esc(r.name)}</strong><small>${r.enabled?'Enabled':'Disabled'} · ${M.flatten(r.roots).length} blocks</small></span></button>`).join('')}</div></section>`;
    }).join('');
    renderCharacterChoice();
    if(changed)requestAnimationFrame(()=>{
      const list=$('#rule-list'), item=list.querySelector('.rule-item.active');if(!item)return;
      const group=item.closest('.rule-group'), bounds=list.getBoundingClientRect(), target=(group.offsetHeight<=list.clientHeight?group:item).getBoundingClientRect();
      if(target.top<bounds.top)list.scrollTop+=target.top-bounds.top;
      else if(target.bottom>bounds.bottom)list.scrollTop+=target.bottom-bounds.bottom;
    });
  }
  function renderCharacterChoice(){
    if(!characters||!current())return;
    const picker=$('#rule-character'), c=current().load||{}, key=characters.key(c), shared=characters.key({});
    const entries=characters.list(), liveCharacter=live?.live?.character;
    if(key!==shared&&!entries.some(i=>characters.key(i)===key))entries.push({character:c.character,realm:c.realm});
    const options=option('shared','Shared',null)+`<option value="current"${liveCharacter?.character&&liveCharacter?.realm?'':' disabled'}>Use current character</option>`+entries.map(i=>option(characters.key(i),characters.label(i),null)).join('')+option('more','Choose another…',null);
    // Polling live data must not rebuild an open, unchanged native menu.
    if(picker.dataset.options!==options){picker.innerHTML=options;picker.dataset.options=options;}
    picker.value=key===shared?'shared':key;
  }
  function assignCharacter(rule,c){
    rule.load||={};
    for(const key of ['character','realm']){const value=String(c?.[key]||'').trim();if(value)rule.load[key]=value;else delete rule.load[key];}
  }
  function inheritCharacter(rule){assignCharacter(rule,current()?.load);return rule;}
  function renderPalette() {
    const search = $('#block-search').value.trim().toLowerCase();
    const entries = Object.entries(M.definitions).filter(([_,d]) => (d.title+' '+d.caption+' '+d.help).toLowerCase().includes(search));
    const open=$('#palette details')?.open||false;
    const button=([type,d])=>`<button class="palette-item ${d.family}" data-type="${type}" draggable="true" aria-label="Add ${d.title} block"><span class="palette-shape" aria-hidden="true"></span><span><strong>${d.title}</strong><small>${d.caption}</small></span><span class="palette-plus" aria-hidden="true">＋</span></button>`;
    const common=entries.filter(([type])=>!['action','call'].includes(type)),actions=entries.filter(([type])=>['action','call'].includes(type));
    $('#palette').innerHTML=common.map(button).join('')+(actions.length?search?actions.map(button).join(''):`<details class="palette-actions" ${open?'open':''}><summary>Reusable actions</summary>${actions.map(button).join('')}</details>`:'')||'<p class="no-blocks">No matching blocks.</p>';
  }
  function renderSelection() {
    const hit = M.find(current().roots,selected);
    if (!hit) { $('#selection-panel').innerHTML = ''; return; }
    const d = M.definitions[hit.node.type];
    $('#selection-panel').innerHTML = `<div class="selected-heading"><span class="block-pill ${d.family}">${d.title}</span><strong>Selected block</strong></div><div class="selection-actions"><button class="button" data-edit="up"${hit.index===0?' disabled':''} aria-label="Move selected block up">↑ Up</button><button class="button" data-edit="down"${hit.index===hit.list.length-1?' disabled':''} aria-label="Move selected block down">↓ Down</button><button class="button" data-edit="copy">Copy</button><button class="button delete-block" data-edit="delete">Delete</button></div>`;
    if(hit.node.props.condition==='equipment')$('#selection-panel').innerHTML+=equipmentOptions(hit.node);
    if(hit.node.props.condition==='talent')$('#selection-panel').innerHTML+=talentOptions(hit.node);
    if(['bossTimer','bossStage'].includes(hit.node.props.condition)||Object.hasOwn(M.bossEvents,hit.node.props.event))$('#selection-panel').innerHTML+=bossFilters(hit.node);
    if(hit.node.props.condition==='encounter'||Object.hasOwn(M.encounterEvents,hit.node.props.event))$('#selection-panel').innerHTML+=encounterFilters(hit.node);
    if(hit.node.type==='when'&&hit.node.props.event==='combatlog')$('#selection-panel').innerHTML+=combatFilters(hit.node);
    if(hit.node.props.condition==='casting'||Object.hasOwn(M.castEvents,hit.node.props.event))$('#selection-panel').innerHTML+=castSpell(hit.node);
  }
  function previewNode() { const n=M.find(current().roots,selected)?.node,compiled=M.flatten(M.executionRule(current()).roots);return n?.type==='show'?(compiled.find(c=>c.id===n.id||c.sourceID===n.id)||n):compiled.find(n=>n.type==='show'); }
  function textDisplay() { return previewNode()?.props.display==='text'; }
  function archSelected() { const n=selected?M.find(current().roots,selected)?.node:previewNode();return n?.type==='show'&&['arc','ring'].includes(n.props.display); }
  function renderArcControls() {
    $('#arch-controls').hidden=!archSelected();if(!archSelected())return;
    const a=current().style.arc||M.arcDefaults(current().style,previewNode().props.display);
    for(const el of $$('[data-arc]')){const v=a[el.dataset.arc]??(el.dataset.arc==='fillUnit'?'player':undefined);if(el.type==='checkbox')el.checked=v;else el.value=el.dataset.arc==='aura'&&!v?'':v;}
    if(previewNode()?.props.fill&&!['timer','full'].includes(previewNode().props.fill))$('#arch-fill').value=previewNode().props.fill;
    $('#arch-fill-unit-label').hidden=!['health','resource'].includes(a.fill);$('#arch-resource-field').hidden=a.fill!=='resource';$('#arch-stack-fields').hidden=a.repeat!=='stacks';$('#arch-pair-field').hidden=a.layout!=='around'||a.limit>2;
    $('#arch-gap-field').hidden=a.repeat!=='stacks'||(a.layout==='around'&&a.limit<=4);
    $('#arch-sweep').max=a.repeat==='stacks'&&a.layout==='around'?(a.limit<=2?180:90):360;
  }
  function selectedShow(){const n=M.find(current()?.roots||[],selected)?.node;return n?.type==='show'?n:null;}
  function renderVisualControls(){
    const host=$('#visual-controls'),n=selectedShow(),open=host.querySelector('details')?.open||false,fontOpen=host.querySelector('.text-effects')?.open||false,spriteOpen=host.querySelector('.sprite-effects')?.open||false,motionOpen=host.querySelector('.motion-effects')?.open||false,advancedMotion=new Set([...host.querySelectorAll('[data-motion-advanced][open]')].map(e=>e.dataset.motionAdvanced));
    host.hidden=!n;if(!n){host.replaceChildren();return;}
    const v=current().style.visual||{},kind=n.props.display,progress=['bar','shape','image'].includes(kind);
    const number=(key,label,min,max,value=v[key]??'')=>`<label>${label}<input data-visual="${key}" type="number" min="${min}" max="${max}" step="1" value="${value}" placeholder="Auto" aria-label="${label}"></label>`;
    const select=(key,label,values,fallback)=>`<label>${label}<select data-visual="${key}" aria-label="${label}">${Object.entries(values).map(([keyValue,text])=>option(keyValue,text,v[key]||fallback)).join('')}</select></label>`;
    host.innerHTML=`<details ${open?'open':''}><summary>Dimensions &amp; effects</summary><div class="two-fields">${number('width','Width',8,2048,v.width||'')}${number('height','Height',8,2048,v.height||'')}</div><div class="two-fields">${number('rotation','Rotation',-360,360,v.rotation||0)}${number('layer','Layer',-100,100,v.layer||0)}</div><div class="two-fields">${['X','Y'].map(axis=>`<label class="check-label"><input data-visual="flip${axis}" type="checkbox" ${v['flip'+axis]?'checked':''}> Mirror ${axis==='X'?'horizontal':'vertical'}</label>`).join('')}</div>
    ${kind==='shape'?select('shape','Shape',M.shapes,'rectangle'):''}
    ${kind==='image'?select('fit','Fit',{contain:'Fit inside',cover:'Crop to fill',stretch:'Stretch'},'contain')+`<label class="check-label"><input data-visual="tint" type="checkbox" ${v.tint?'checked':''}> Use accent color</label>`:''}
    ${progress?select('direction','Fill direction',{right:'Left to right',left:'Right to left',up:'Bottom to top',down:'Top to bottom'},'right')+(kind==='image'?'':`<div class="two-fields">${number('fillOpacity','Fill opacity',0,100,v.fillOpacity??(kind==='bar'?30:100))}${number('trackOpacity','Track opacity',0,100,v.trackOpacity??0)}</div><label>Track color<input data-visual="trackColor" type="color" aria-label="Track color" value="${M.colorValue(v.trackColor)||'#101923'}"></label>`):''}
    ${kind==='bar'?select('barIcon','Icon position',{none:'None',left:'Left',right:'Right',top:'Top',bottom:'Bottom'},'none'):''}
    ${['text','bar'].includes(kind)?`<details class="text-effects" ${fontOpen?'open':''}><summary>Font</summary>${select('fontFamily','Font',M.fontFamilies,'ui')}<div class="two-fields"><label class="check-label"><input data-visual="bold" type="checkbox" ${v.bold!==false?'checked':''}> Bold</label><label class="check-label"><input data-visual="italic" type="checkbox" ${v.italic?'checked':''}> Italic</label></div>${select('textEffect','Effect',{shadow:'Shadow',outline:'Outline',none:'None'},'shadow')}${select('textAlign','Text alignment',{left:'Left',center:'Center',right:'Right'},'center')}</details>`:''}${kind==='image'?spriteControls(v,spriteOpen):''}${motionControls(v,motionOpen,advancedMotion)}<button class="button" data-visual-reset>Reset effects</button></details>`;
  }
  function motionControls(v,open,advanced){
    const m=v.motion||{},field=(phase,e,key,label,min,max,step=1,fallback='')=>`<label>${label}<input data-motion="${key}" data-motion-phase="${phase}" aria-label="${M.Motion.phases[phase]} ${label}" type="number" min="${min}" max="${max}" step="${step}" value="${e[key]??fallback}" ${key==='duration'?'required':''}></label>`;
    return `<details class="motion-effects" ${open?'open':''}><summary>Animations</summary>${Object.entries(M.Motion.phases).map(([phase,label])=>{
      const e=m[phase],name=M.Motion.presetName(phase,e),select=(key,values,value)=>`<select data-motion="${key}" data-motion-phase="${phase}" aria-label="${label} ${key}">${Object.entries(values).map(([k,l])=>option(k,l,value)).join('')}</select>`;
      return `<label>${label}${select('preset',{none:'None',...Object.fromEntries(Object.entries(M.Motion.presets[phase]).map(([k,v])=>[k,v.label])),custom:'Custom'},name)}</label>${e?`${field(phase,e,'duration','Seconds',.05,phase==='finish'?30:300,.05)}<details data-motion-advanced="${phase}" ${advanced.has(phase)||name==='custom'?'open':''}><summary>Advanced</summary><label>Easing${select('easing',M.Motion.easings,e.easing||'linear')}</label>${phase==='main'?`<label>Cycle${select('cycle',{pingpong:'Back and forth',repeat:'Repeat'},e.cycle||'pingpong')}</label>`:''}${field(phase,e,'alpha','Opacity',0,100,1,100)}<div class="two-fields">${field(phase,e,'x','Offset X',-2048,2048,1,0)}${field(phase,e,'y','Offset Y',-2048,2048,1,0)}</div><div class="two-fields">${field(phase,e,'scaleX','Scale X',0,4,.05,1)}${field(phase,e,'scaleY','Scale Y',0,4,.05,1)}</div>${field(phase,e,'rotation','Rotation',-3600,3600,1,0)}<label class="check-label"><input type="checkbox" data-motion="useColor" data-motion-phase="${phase}" ${e.color?'checked':''}> Color</label>${e.color?`<input type="color" data-motion="color" data-motion-phase="${phase}" value="${M.colorValue(e.color)}" aria-label="${label} color">`:''}</details>`:''}`;
    }).join('')}</details>`;
  }
  function spriteControls(v,open){
    const s=v.sprite,asset=media.library.assets.find(a=>a.id===v.image&&a.available),number=(key,label,min,max,value=s?.[key])=>`<label>${label}<input data-sprite="${key}" type="number" min="${min}" max="${max}" step="1" value="${value}" aria-label="${label}"></label>`;
    return `<details class="sprite-effects" ${open?'open':''}><summary>Sprite sheet</summary><label class="check-label"><input type="checkbox" data-sprite="enabled" ${s?'checked':''}> Enabled</label>${s?`<div class="two-fields">${number('columns','Columns',1,64)}${number('rows','Rows',1,64)}</div><div class="two-fields">${number('start','First frame',1,s.columns*s.rows-s.frames+1,s.start+1)}${number('frames','Frames',1,Math.min(256,s.columns*s.rows-s.start))}</div><label>Playback<select data-sprite="mode" aria-label="Playback">${Object.entries(M.spriteModes).map(([k,l])=>option(k,l,s.mode)).join('')}</select></label>${s.mode!=='progress'?number('fps','Frames / second',1,60):''}<label class="check-label"><input data-sprite="reverse" type="checkbox" ${s.reverse?'checked':''}> Reverse</label>${asset&&!M.spriteFits(s,asset)?'<p class="media-missing">More cells than image pixels.</p>':''}`:''}</details>`;
  }
  $('#visual-controls').addEventListener('change',e=>{
    const el=e.target,phase=el.dataset.motionPhase;
    if(phase){if(!el.checkValidity()){el.reportValidity();return;}mutate(()=>{const v=current().style.visual||={},m=v.motion||={};if(el.dataset.motion==='preset'){const next=M.Motion.preset(phase,el.value);if(next)m[phase]=next;else delete m[phase];if(!Object.keys(m).length)delete v.motion;return;}const effect=m[phase],key=el.dataset.motion;if(key==='useColor'){if(el.checked)effect.color=current().style.color;else delete effect.color;}else if(el.type==='number'&&el.value==='')delete effect[key];else effect[key]=el.type==='number'?+el.value:el.value;});return;}
    const spriteKey=el.dataset.sprite;
    if(spriteKey){if(!el.checkValidity()){el.reportValidity();return;}mutate(()=>{const v=current().style.visual||={};if(spriteKey==='enabled'){if(el.checked){v.sprite={columns:1,rows:1,frames:1,start:0,fps:12,mode:'loop'};selectedShow().props.fill='full';}else delete v.sprite;return;}const s=v.sprite;s[spriteKey]=el.type==='checkbox'?el.checked:el.type==='number'?+el.value:el.value;if(spriteKey==='start')s.start--;if(['columns','rows'].includes(spriteKey)){s.start=Math.min(s.start,s.columns*s.rows-1);s.frames=Math.min(s.frames,s.columns*s.rows-s.start);}if(spriteKey==='mode'&&s.mode==='progress'&&selectedShow().props.fill==='full')selectedShow().props.fill='source';});return;}
    const key=el.dataset.visual;if(!key)return;if(!el.checkValidity()){el.reportValidity();return;}
    mutate(()=>{const v=current().style.visual||={};if(el.type==='number'&&el.value==='')delete v[key];else v[key]=el.type==='checkbox'?el.checked:el.type==='number'?+el.value:el.value;});
  });
  $('#visual-controls').addEventListener('click',e=>{if(e.target.closest('[data-visual-reset]'))mutate(()=>{const image=current().style.visual?.image;if(image)current().style.visual={image};else delete current().style.visual;});});
  function renderShowControls(){
    const host=$('#show-controls'),n=selectedShow(),wasOpen=host.dataset.block===n?.id&&host.querySelector('details')?.open;
    host.hidden=!n;if(!n){host.replaceChildren();return;}host.dataset.block=n.id;
    const p=n.props,source=p.dataSource||'auto',select=(key,values,value)=>`<select id="show-${key}" data-show="${key}">${Object.entries(values).map(([v,l])=>option(v,l,value)).join('')}</select>`;
    const spells=['buff','debuff','spell','bossTimer','learned'].includes(source)||source==='talent'&&['spell','pvp'].includes(p.dataTalentKind),numeric=['buff','debuff','spell','item','bossTimer','talent','learned'].includes(source);
    const icon=p.icon||'shield',dataUsed=icon==='auto'||['source','charges','recharge'].includes(p.fill)||/\{\w+\}/.test(p.text);
    const iconField=(p.display==='icon'||p.display==='bar'&&current().style.visual?.barIcon&&!['none'].includes(current().style.visual.barIcon))?`<label>Icon${select('icon',{shield:'Shield',auto:'From source',spell:'Choose spell…'},['shield','auto'].includes(icon)?icon:'spell')}</label>${!['shield','auto'].includes(icon)?`<label>Spell ID<span class="spell-field"><input type="text" id="show-icon-id" data-show="iconID" value="${esc(icon)}" inputmode="numeric" pattern="[1-9][0-9]{0,6}" required><button class="spell-find" data-spell-target="icon" data-spell-node="${n.id}">Search</button></span></label>`:''}`:'';
    const insert=['text','bar'].includes(p.display)?`<label>Insert value<select id="show-token"><option value="">Choose…</option>${Object.entries({...M.dataTokens,...Object.fromEntries((current().logic?.values||[]).map(v=>['value:'+v.key,v.name]))}).map(([k,v])=>option(k,v,'')).join('')}</select></label>`:'';
    const fill=['bar','shape','image'].includes(p.display)?`<label>Fill${select('fill',{timer:'SHOW timer',full:'Full',source:'Source progress',charges:'Charges',recharge:'Recharge'},p.fill||'timer')}</label>`:'';
    const imageID=current().style.visual?.image||'',images=media.library.assets.filter(a=>a.kind==='image'),missing=imageID&&!images.some(a=>a.id===imageID&&a.available);
    const imageField=p.display==='image'?`<label>Image<span class="media-picker"><select data-image-select aria-label="Image">${option('','Choose…',imageID)}${images.map(a=>option(a.id,a.name+(a.available?'':' (missing)'),imageID)).join('')}${missing&&!images.some(a=>a.id===imageID)?option(imageID,'Missing image',imageID):''}</select><button class="button" data-media-image>Browse…</button></span></label>${missing?'<p class="media-missing">Image unavailable.</p>':''}`:'';
    host.innerHTML=imageField+iconField+insert+fill+`<details class="aura-controls" ${wasOpen||dataUsed||p.dataSource?'open':''}><summary>Live data</summary><label>Source${select('dataSource',Object.fromEntries(Object.entries(M.dataKinds).filter(([k])=>k!=='value'||current().logic?.values?.length)),source)}</label>
      ${source==='value'?`<label>Value${select('dataValue',Object.fromEntries((current().logic?.values||[]).map(v=>[v.key,v.name])),p.dataValue)}</label><label>Maximum${select('dataMaxValue',{'':'Number',...Object.fromEntries((current().logic?.values||[]).map(v=>[v.key,v.name]))},p.dataMaxValue||'')}</label>${!p.dataMaxValue?`<input data-show="dataMaximum" type="number" step="any" min=".000000001" max="${M.Values.limit}" value="${p.dataMaximum??100}" aria-label="Maximum value">`:''}`:''}
      ${['bossTimer','bossStage'].includes(source)?`<label>Provider${select('dataProvider',source==='bossStage'?bossStageProviders:M.bossProviders,p.dataProvider||'auto')}</label>`:''}
      ${source==='bossTimer'?`<label>Text contains<input data-show="dataText" maxlength="80" value="${esc(p.dataText||'')}"></label><label>Kind${select('dataKind',M.bossKinds,p.dataKind||'any')}</label>${p.dataProvider==='blizzard'?`<label>Timeline source${select('dataTimelineSource',M.timelineSources,p.dataTimelineSource||'any')}</label>`:''}`:''}
      ${source==='talent'?`<label>Type${select('dataTalentKind',M.talentKinds,p.dataTalentKind||'spell')}</label>`:''}
      ${source==='resource'?`<label>Resource${select('dataResource',M.resources,p.dataResource||'primary')}</label>`:''}
      ${numeric?`<label>${source==='item'?'Item ID':source==='talent'&&p.dataTalentKind==='hero'?'Tree ID':source==='talent'&&p.dataTalentKind==='entry'?'Entry ID':'Spell ID'}<span class="spell-field"><input type="text" id="show-dataID" data-show="dataID" value="${esc(p.dataID||'')}" inputmode="numeric" pattern="[1-9][0-9]{0,6}" ${source==='bossTimer'?'placeholder="Any spell"':'required'}>${spells?`<button class="spell-find" data-spell-target="data" data-spell-node="${n.id}">Search</button>`:''}</span></label>`:''}
      ${source==='slot'?`<label>Slot${select('dataID',{'13':'Upper trinket','14':'Lower trinket','16':'Main hand','17':'Off hand'},p.dataID||'13')}</label>`:''}
      ${['buff','debuff','health','resource','casting'].includes(source)?`<label>Unit${select('dataUnit',M.unitChoices,p.dataUnit||'player')}</label>`:''}
    </details>`;
  }
  $('#show-controls').addEventListener('change',e=>{
    const n=selectedShow();if(!n)return;
    if(e.target.id==='show-token'){
      const token=e.target.value;if(!token)return;
      mutate(()=>{n.props.text=(n.props.text?n.props.text+' ':'')+'{'+token+'}';});return;
    }
    if(e.target.hasAttribute('data-image-select')){mutate(()=>{(current().style.visual||={}).image=e.target.value;});return;}
    const key=e.target.dataset.show;if(!key)return;
    if(!e.target.checkValidity()){e.target.reportValidity();return;}
    const value=e.target.value;
    mutate(()=>{
      const p=n.props;
      if(key==='icon'){p.icon=value==='spell'?String(M.dataSource(current(),n.id).id||363916):value;return;}
      p[key==='iconID'?'icon':key]=key==='dataMaximum'?+value:value;
      if(key==='dataProvider')delete p.dataTimelineSource;
      if(key==='dataSource'){
        if(value==='value'&&!M.Values.named(current().logic,p.dataValue))p.dataValue=current().logic.values[0].key;
        p.dataResource||='primary';p.dataUnit||='player';p.dataTalentKind||='spell';
        p.dataID=['bossTimer','bossStage'].includes(value)?'':value==='gcd'?'61304':value==='slot'?'13':value==='item'?'5512':p.dataID||'363916';
      }
    });
  });
  const spellNames={},requestedNames=new Set();
  function requestDisplayName(id){
    if(!id||requestedNames.has(id))return;requestedNames.add(id);
    fetch('/api/overlays/spells?q='+id).then(r=>r.ok?r.json():null).then(data=>{const found=data?.spells.find(s=>s.id===id);if(found){spellNames[id]=found.name;renderPreview();}}).catch(()=>{});
  }
  const predicateTypes={pet:'Main pet',value:'Value',equipment:'Equipment',talent:'Talent',learned:'Learned spell',bossTimer:'Boss timer',bossStage:'Boss stage',encounter:'Encounter',eventValue:'Event value',casting:'Casting',health:'Health',resource:'Resource',buff:'Buff',debuff:'Debuff',cooldown:'Cooldown',role:'Player role',location:'Location',group:'Condition group'};
  function predicateDefaults(kind){
    if(kind==='pet')return {condition:'pet',check:'unavailable',value:'Wolf'};
    if(kind==='value')return {condition:'value',key:current().logic.values[0].key,operator:'lt',compareTo:'literal',value:20};
    if(kind==='equipment')return {condition:kind,source:'item',id:'5512',slot:0,check:'equipped',operator:'gte',value:2};
    if(['talent','learned'].includes(kind))return {condition:kind,source:'spell',id:'363916',check:'selected',operator:'gte',value:1};
    if(kind==='group')return {condition:'group',mode:'all',tests:[]};
    if(['bossTimer','bossStage'].includes(kind))return {condition:kind,provider:'auto',spell:'',textFilter:'',timerKind:'any',timelineSource:'any',encounterID:'',check:'active',operator:'gte',value:kind==='bossStage'?2:5};
    if(kind==='encounter')return {condition:kind,check:'active',encounterID:'',difficulty:'',operator:'gte',value:60};
    if(kind==='eventValue')return {condition:kind,field:'amount',operator:'gte',value:1000};
    if(kind==='casting')return {condition:kind,unit:'player',check:'active',spell:'',operator:'lt',value:1};
    if(kind==='cooldown')return {condition:kind,source:'spell',id:'363916',check:'ready',operator:'gte',value:1};
    if(['buff','debuff'].includes(kind))return {condition:kind,aura:'124273',unit:'player',check:'active',operator:'gte',value:3};
    if(kind==='resource')return {condition:kind,resource:'primary',measure:'percent',operator:'lt',value:20};
    return {condition:kind,value:{role:'tank',location:'dungeon',health:35}[kind],...(kind==='health'?{operator:'lt'}:{})};
  }
  function groupSelected(){const n=M.find(current()?.roots||[],selected)?.node;return n?.type==='if'&&n.props.condition==='group'?n:null;}
  function editingModifier(){return selectedShow()&&modifierEditor?.rule===current()?.id?current().style.modifiers?.[modifierEditor.index]:null;}
  function predicateOwner(){if(valueCheckEditing())return valuesEditor.check();if(loadEditing()&&current()?.load?.conditions)return {id:'load-'+current().id,props:current().load.conditions};const m=editingModifier();return m?{id:'modifier-'+current().id+'-'+modifierEditor.index,props:m.condition}:groupSelected();}
  function predicateAt(path){let p=predicateOwner()?.props;if(path)for(const i of path.split('.'))p=p?.tests?.[+i];return p;}
  function predicateSummary(p){
    if(p.condition==='pet')return `Main pet · ${p.check}${['family','specialization','creatureType'].includes(p.check)?' '+p.value:''}`;
    if(p.condition==='value')return (M.Values.named(current()?.logic,p.key)?.name||'Value')+' '+({true:'is true',false:'is false',lt:'<',lte:'≤',eq:'=',ne:'≠',gt:'>',gte:'≥'}[p.operator]||'');
    if(p.condition==='group')return `${{all:'All',any:'Any',not:'Not'}[p.mode]} · ${p.tests.length}`;
    const op={lt:'<',lte:'≤',eq:'=',ne:'≠',gt:'>',gte:'≥',cap:'at cap'}[p.operator||'lt'];
    if(p.condition==='health')return `Health ${op} ${p.value}%`;
    if(p.condition==='resource')return `${M.resources[p.resource]} ${op}${p.operator==='cap'?'':' '+p.value+(p.measure==='percent'?'%':'')}`;
    if(['buff','debuff'].includes(p.condition))return `${predicateTypes[p.condition]} ${p.aura} · ${{active:'active',missing:'missing',stacks:'stacks',remaining:'time left'}[p.check]}`;
    if(p.condition==='equipment')return `${M.equipmentKinds[p.source]} · ${p.source==='saved'?p.name:p.source==='slot'?M.equipmentSlots[p.slot]:p.id||''} · ${p.check}${['count','level'].includes(p.check)?' '+op+' '+p.value:''}`;
    if(['talent','learned'].includes(p.condition))return `${p.condition==='learned'?'Learned spell':M.talentKinds[p.source]} · ${p.id} · ${p.check}${p.check==='rank'?' '+op+' '+p.value:''}`;
    if(p.condition==='bossStage')return `Boss stage ${op} ${p.value}`;
    if(p.condition==='bossTimer')return `Boss timer · ${['remaining','count'].includes(p.check)?p.check+' '+op+' '+p.value:p.check}`;
    if(p.condition==='encounter')return `Encounter · ${p.check==='elapsed'?'elapsed '+op+' '+p.value+'s':p.check}`;
    if(p.condition==='eventValue')return `${M.combatValues[p.field]} ${op} ${p.value}`;
    if(p.condition==='casting')return `${M.unitChoices[p.unit||'player']} · ${p.check}`;
    if(p.condition==='cooldown')return `${p.source==='gcd'?'Global':p.source==='slot'?'Slot '+p.id:'ID '+p.id} · ${{ready:'ready',cooldown:'on cooldown',usable:'usable',unusable:'not usable',remaining:'time left',charges:'charges',full:'full charges',recharge:'recharge'}[p.check]}`;
    return `${predicateTypes[p.condition]} · ${p.value}`;
  }
  function setPredicateMode(p,value){
    if(value==='not'&&p.tests.length>1)p.tests=[{condition:'group',mode:'all',tests:p.tests}];p.mode=value;
  }
  function renderPredicateControls(){
    const host=$('#condition-controls'),n=predicateOwner();host.hidden=!n;$('#display-settings').hidden=!!n;
    if(!n){host.replaceChildren();return;}
    const existing=host.dataset.block===n.id?new Set([...host.querySelectorAll('details')].map(e=>e.dataset.testPath)):new Set();
    const opened=host.dataset.block===n.id?new Set([...host.querySelectorAll('details[open]')].map(e=>e.dataset.testPath)):new Set();host.dataset.block=n.id;
    function group(p,path){
      return `<label>Match<select data-predicate-mode="${path}">${Object.entries({all:'All (AND)',any:'Any (OR)',not:'Not (NOT)'}).map(([k,v])=>option(k,v,p.mode)).join('')}</select></label><div class="predicate-children">${p.tests.map((child,i)=>row(child,path?path+'.'+i:String(i))).join('')}</div>${p.mode!=='not'||p.tests.length===0?`<select data-predicate-add="${path}" aria-label="Add condition"><option value="">+ Condition</option>${Object.entries(conditionTypes()).map(([k,v])=>option(k,v,'')).join('')}</select>`:''}`;
    }
    function row(p,path){
      const content=p.condition==='group'?group(p,path):`<div class="predicate-fields">${fields({type:'if',id:n.id,props:p},true)}</div>`;
      return `<details class="predicate-card" data-test-path="${path}" ${opened.has(path)||!existing.has(path)&&p.condition==='group'?'open':''}><summary>${esc(predicateSummary(p))}</summary>${content}<button class="text-button predicate-remove" data-predicate-remove="${path}" aria-label="Remove condition">Remove</button></details>`;
    }
    host.innerHTML=`${editingModifier()?'<button type="button" class="text-button" data-modifier-back>← Appearance changes</button>':''}<h3>${loadEditing()?'Build & gear':'Conditions'}</h3>${group(n.props,'')}${editingModifier()?modifierEffects():''}`;
  }
  $('#condition-controls').addEventListener('change',e=>{
    const target=e.target;if(target.closest('#modifier-effects')||target.matches('[data-equipped]'))return;const path=target.closest('[data-test-path]')?.dataset.testPath||'';
    mutate(()=>{
      if(target.dataset.predicateMode!==undefined){setPredicateMode(predicateAt(target.dataset.predicateMode),target.value);return;}
      if(target.dataset.predicateAdd!==undefined){if(target.value)predicateAt(target.dataset.predicateAdd).tests.push(predicateDefaults(target.value));return;}
      const p=predicateAt(path),key=target.dataset.prop;if(!p||!key)return;
      if(key==='condition'){const old={...p};for(const key of Object.keys(p))delete p[key];Object.assign(p,predicateDefaults(target.value));if(['buff','debuff'].includes(p.condition)&&old.aura)p.aura=old.aura;return;}
      if(changeValueCondition(p,key,target.value))return;
      let value=target.value;if(target.type==='number'){if(!target.checkValidity())throw Error('Enter a value within the shown range.');value=+value;}
      if(p.condition==='eventValue'&&key==='value')value=+value;
      p[key]=value;
      if(p.condition==='equipment')equipmentChange(p,key,value);
      if(key==='provider')delete p.timelineSource;
      if(p.condition==='eventValue'&&key==='field'&&value==='critical'){p.operator='eq';p.value=1;}
      if(key==='source'&&p.condition==='cooldown'){p.id=value==='gcd'?'61304':value==='slot'?'13':value==='item'?'5512':'363916';p.check='ready';}
      if(key==='check'&&['stacks','remaining','charges','recharge','count','rank'].includes(value)){p.operator=['stacks','charges'].includes(value)?'gte':'lt';p.value=value==='rank'?1:value==='stacks'?3:value==='charges'?1:5;}
      if(key==='measure'&&value==='percent')p.value=Math.min(100,p.value);
      if(['id','aura','spell','encounterID','difficulty'].includes(key))p[key]=value.trim();
    });
  });
  $('#condition-controls').addEventListener('click',e=>{
    if(e.target.closest('[data-modifier-back]')){modifierEditor=null;renderAppearance();return;}
    const remove=e.target.closest('[data-predicate-remove]');
    if(remove){const parts=remove.dataset.predicateRemove.split('.'),index=+parts.pop();mutate(()=>predicateAt(parts.join('.')).tests.splice(index,1));return;}
    const button=e.target.closest('.spell-find');if(!button)return;e.stopPropagation();
    const path=button.closest('[data-test-path]').dataset.testPath,n=predicateOwner(),p=predicateAt(path),ruleID=current().id,blockID=n.id;
    spellPicker.open({id:['casting','bossTimer','bossStage'].includes(p.condition)?p.spell:['cooldown','talent','learned'].includes(p.condition)?p.id:p.aura,kind:['casting','cooldown','talent','learned','bossTimer','bossStage'].includes(p.condition)?'all':'auras',prefer:current().load?.class||live?.live?.character?.class||'',focus:()=>$('#condition-controls').querySelector(`[data-test-path="${path}"] .spell-find`),choose:spell=>{
      if(current()?.id!==ruleID||predicateOwner()?.id!==blockID)return false;
      return mutate(()=>{const p=predicateAt(path);if(!p)throw Error('Choose the condition again.');p[['casting','bossTimer','bossStage'].includes(p.condition)?'spell':['cooldown','talent','learned'].includes(p.condition)?'id':'aura']=String(spell.id);});
    }});
  });
  const modifierProperties={color:'Color',size:'Display size',font:'Text size',opacity:'Opacity',text:'Text',glow:'Glow',visible:'Visibility',sound:'Sound'};
  const modifierDefaults={color:'#ef9b9b',size:90,font:24,opacity:100,text:'Alert!',glow:true,visible:true,sound:'alert'};
  function renderModifiers(){
    const host=$('#modifier-controls');host.hidden=!selectedShow();if(host.hidden)return;
    const open=host.querySelector('details')?.open||false,ms=current().style.modifiers||[];
    host.innerHTML=`<details ${open?'open':''}><summary>Appearance changes${ms.length?' · '+ms.length:''}</summary>${ms.map((m,i)=>`<div class="modifier-row"><button class="text-button modifier-edit" data-modifier-edit="${i}">${i+1}. ${esc(predicateSummary(m.condition))}</button><button class="text-button" data-modifier-up="${i}" aria-label="Move change up" ${i===0?'disabled':''}>↑</button><button class="text-button" data-modifier-remove="${i}" aria-label="Remove appearance change">×</button></div>`).join('')}<button class="button" data-modifier-new ${ms.length>=16?'disabled':''}>+ Change</button></details>`;
  }
  $('#modifier-controls').addEventListener('click',e=>{
    const b=e.target.closest('button');if(!b)return;
    if(b.hasAttribute('data-modifier-new')){mutate(()=>{const ms=current().style.modifiers||=[];ms.push({condition:{condition:'group',mode:'all',tests:[predicateDefaults('health')]},color:'#ef9b9b'});modifierEditor={rule:current().id,index:ms.length-1};});return;}
    const index=+(b.dataset.modifierEdit??b.dataset.modifierRemove??b.dataset.modifierUp);
    if(b.hasAttribute('data-modifier-edit')){modifierEditor={rule:current().id,index};const m=editingModifier();if(m.condition.condition!=='group')mutate(()=>{m.condition={condition:'group',mode:'all',tests:[m.condition]};});else renderAppearance();return;}
    mutate(()=>{const ms=current().style.modifiers;if(b.hasAttribute('data-modifier-remove'))ms.splice(index,1);else if(index>0)[ms[index-1],ms[index]]=[ms[index],ms[index-1]];});
  });
  for(const el of $$('[data-lifecycle]'))el.addEventListener('change',()=>mutate(()=>{current().style[el.dataset.lifecycle]=el.value;}));
  function modifierEffects(){
    const m=editingModifier();
    const control=(key,value)=>{
      if(key==='color')return `<input type="color" data-modifier-value="color" aria-label="Change color" value="${M.colorValue(value)}">`;
      if(['glow','visible'].includes(key))return `<select data-modifier-value="${key}" aria-label="Change ${modifierProperties[key]}">${option('true',key==='visible'?'Show':'On',String(value))}${option('false',key==='visible'?'Hide':'Off',String(value))}</select>`;
      if(key==='sound')return `<select data-modifier-value="sound" aria-label="Change sound">${Object.entries(soundChoices(value)).map(([k,v])=>option(k,v,value)).join('')}</select>`;
      const numeric=key!=='text',range={size:[16,400],font:[10,72],opacity:[0,100]}[key];
      return `<input type="${numeric?'number':'text'}" data-modifier-value="${key}" aria-label="Change ${modifierProperties[key]}" value="${esc(value)}" ${numeric?`min="${range[0]}" max="${range[1]}"`:'maxlength="90"'}>`;
    };
    return `<section id="modifier-effects"><h3>Change</h3>${Object.keys(modifierProperties).filter(k=>m[k]!==undefined).map(k=>`<div class="modifier-effect"><label>${modifierProperties[k]}${control(k,m[k])}</label><button class="text-button" data-effect-remove="${k}" aria-label="Remove ${modifierProperties[k]} change">×</button></div>`).join('')}<select data-effect-add aria-label="Add appearance property"><option value="">+ Property</option>${Object.entries(modifierProperties).filter(([k])=>m[k]===undefined).map(([k,v])=>option(k,v,'')).join('')}</select></section>`;
  }
  $('#condition-controls').addEventListener('change',e=>{
    const b=e.target;if(!b.closest('#modifier-effects'))return;
    mutate(()=>{const m=editingModifier();if(b.hasAttribute('data-effect-add')){if(b.value)m[b.value]=modifierDefaults[b.value];return;}const k=b.dataset.modifierValue;if(!k)return;if(!b.checkValidity())throw Error('Enter a value within the shown range.');m[k]=b.type==='number'?+b.value:['glow','visible'].includes(k)?b.value==='true':b.value;});
  });
  $('#condition-controls').addEventListener('click',e=>{const b=e.target.closest('[data-effect-remove]');if(b)mutate(()=>{delete editingModifier()[b.dataset.effectRemove];});});
  const attachmentTargets={screen:'Screen',unit:'Unit frame',nameplate:'Nameplate',PlayerFrame:'Blizzard player frame',TargetFrame:'Blizzard target frame',FocusFrame:'Blizzard focus frame',Minimap:'Minimap',ObjectiveTrackerFrame:'Quest tracker',MainActionBar:'Action bar',custom:'Named frame…'};
  function setAnchor(anchor){for(const r of M.groupMembers(rules,current())){if(anchor)r.style.anchor=M.clone(anchor);else delete r.style.anchor;}}
  function renderAnchorControls(){
    const host=$('#anchor-controls');host.hidden=!selectedShow();if(host.hidden)return;
    const a=current().style.anchor,open=host.querySelector('details')?.open||false,key=!a?'screen':a.kind==='frame'?(Object.hasOwn(attachmentTargets,a.target)?a.target:'custom'):a.kind;
    host.innerHTML=`<details ${open?'open':''}><summary>Attachment</summary><label>Attach to<select data-anchor-target>${Object.entries(attachmentTargets).map(([k,v])=>option(k,v,key)).join('')}</select></label>${a?`${key==='custom'?`<label>Frame name<input data-anchor="target" type="text" maxlength="80" pattern="[A-Za-z_][A-Za-z0-9_]*" value="${esc(a.target)}"></label>`:a.kind!=='frame'?`<label>Unit<select data-anchor="target">${Object.entries(M.anchorUnits).map(([k,v])=>option(k,v,a.target)).join('')}</select></label>`:''}<div class="two-fields"><label>Frame point<select data-anchor="point">${Object.entries(M.anchorPoints).map(([k,v])=>option(k,v,a.point)).join('')}</select></label><label>Display point<select data-anchor="selfPoint">${Object.entries(M.anchorPoints).map(([k,v])=>option(k,v,a.selfPoint)).join('')}</select></label></div><div class="two-fields"><label>X<input type="number" min="-4000" max="4000" step="0.1" data-anchor="x" value="${a.x}"></label><label>Y<input type="number" min="-4000" max="4000" step="0.1" data-anchor="y" value="${a.y}"></label></div>`:''}</details>`;
    $('#sample-anchor-controls').hidden=!a;
  }
  $('#anchor-controls').addEventListener('change',e=>{const el=e.target;mutate(()=>{
    if(el.hasAttribute('data-anchor-target')){if(el.value==='screen'){for(const r of M.groupMembers(rules,current())){const p=previewPlacement.find(p=>p.rule.id===r.id);if(p){r.style.x=Math.max(0,Math.min(100,p.x*100/(backgrounds?.width||1920)));r.style.y=Math.max(0,Math.min(100,p.y*100/(backgrounds?.height||1080)));}}if(current().style.group&&previewBounds)M.updateGroup(rules,current(),{x:Math.max(0,Math.min(100,previewBounds.x*100/(backgrounds?.width||1920))),y:Math.max(0,Math.min(100,previewBounds.y*100/(backgrounds?.height||1080)))});setAnchor(null);return;}setAnchor({kind:['unit','nameplate'].includes(el.value)?el.value:'frame',target:el.value==='unit'?'player':el.value==='nameplate'?'target':el.value==='custom'?'MyFrame':el.value,point:'center',selfPoint:'center',x:0,y:0});return;}
    if(!el.dataset.anchor)return;if(!el.checkValidity())throw Error('Enter a value within the shown range.');setAnchor({...current().style.anchor,[el.dataset.anchor]:el.type==='number'?+el.value:el.value.trim()});
  });});
  function previewAnchor(actor){
    const a=current()?.style.anchor;if(!a)return null;const unit=a.target==='current'?(M.cloneContext(sampleState(),actor)?.unit||'player'):a.target;
    if($('#sample-anchor-source').value==='live'){
      const c=live?.live?.character,bg=backgrounds?.selected();
      if(!c||c.anchorsVersion!==1||Math.abs(Date.now()/1000-c.at)>3||bg&&((bg.character||'').toLowerCase()!==(c.character||'').toLowerCase()||(bg.realm||'').toLowerCase()!==(c.realm||'').toLowerCase()))return null;
      const rect=c.anchors?.find(v=>v.kind===a.kind&&v.target===unit);if(!rect?.available)return null;
      const sx=(backgrounds?.width||1920)/c.width,sy=(backgrounds?.height||1080)/c.height;return {...rect,x:rect.x*sx,y:rect.y*sy,width:rect.width*sx,height:rect.height*sy};
    }
    return {available:$('#sample-anchor-visible').checked,x:+$('#sample-anchor-x').value,y:+$('#sample-anchor-y').value+(a.target==='current'?Math.max(0,M.partyTokens.includes(unit)?M.partyTokens.indexOf(unit):Number(unit.match(/\d+$/)?.[0]||1)-1)*70:0),width:+$('#sample-anchor-width').value,height:+$('#sample-anchor-height').value};
  }
  $('#sample-anchor-controls').addEventListener('change',()=>renderPreview());
  $('#sample-roster-controls').addEventListener('change',e=>{if(e.target.id==='sample-group')selectRuleSample();updateSample();});
  function renderGroupControls(){
    const host=$('#group-controls');host.hidden=!selectedShow();if(host.hidden)return;
    const group=current().style.group,open=host.querySelector('details')?.open||false,groups=new Map(rules.filter(r=>r.style.group).map(r=>[r.style.group.id,r.style.group]));
    host.innerHTML=`<details ${open?'open':''}><summary>Group${group?' · '+esc(group.name):''}</summary><label>Group<select data-group-choice aria-label="Display group">${option('','None',group?.id)}${[...groups.values()].map(g=>option(g.id,g.name,group?.id)).join('')}${option('new','New group…',group?.id)}</select></label>${group?`<label>Name<input type="text" maxlength="60" data-layout="name" value="${esc(group.name)}"></label><label>Layout<select data-layout="mode">${Object.entries({static:'Keep positions',row:'Row',column:'Column',grid:'Grid',circle:'Circle'}).map(([k,v])=>option(k,v,group.mode)).join('')}</select></label>${group.mode!=='static'?`<div class="two-fields"><label>Spacing<input type="number" data-layout="gap" value="${group.gap}" min="0" max="200"></label>${group.mode==='grid'?`<label>Columns<input type="number" data-layout="columns" value="${group.columns}" min="1" max="20"></label>`:''}</div><label>Sort<select data-layout="sort">${Object.entries({order:'Library order',name:'Name',remaining:'Time left',stacks:'Stacks / charges'}).map(([k,v])=>option(k,v,group.sort)).join('')}</select></label><label class="check-label"><input type="checkbox" data-layout="reverse" ${group.reverse?'checked':''}> Reverse</label>`:''}<div class="group-members">${M.groupMembers(rules,current()).map((r,i)=>`<div class="modifier-row"><button class="text-button modifier-edit" data-group-member="${r.id}">${esc(r.name)}</button><button class="text-button" data-group-move="${r.id}" aria-label="Move group member up" ${i===0?'disabled':''}>↑</button></div>`).join('')}</div>`:''}</details>`;
  }
  $('#group-controls').addEventListener('change',e=>{
    const el=e.target;
    mutate(()=>{
      if(el.hasAttribute('data-group-choice')){if(el.value==='new')current().style.group=M.newGroup('My group',current().style);else if(el.value){const member=rules.find(r=>r.style.group?.id===el.value);current().style.group=M.clone(member.style.group);if(member.style.anchor)current().style.anchor=M.clone(member.style.anchor);else delete current().style.anchor;}else {const placed=previewPlacement.find(p=>p.rule.id===current().id);if(placed){current().style.x=Math.max(0,Math.min(100,placed.x*100/(backgrounds?.width||1920)));current().style.y=Math.max(0,Math.min(100,placed.y*100/(backgrounds?.height||1080)));}delete current().style.group;}return;}
      const key=el.dataset.layout;if(!key)return;if(!el.checkValidity())throw Error('Enter a value within the shown range.');let value=el.type==='checkbox'?el.checked:el.type==='number'?+el.value:el.value;if(key==='name')value=value.trim();if(key==='mode'&&value==='static')for(const r of M.groupMembers(rules,current())){const placed=previewPlacement.find(p=>p.rule.id===r.id);if(placed){r.style.x=Math.max(0,Math.min(100,placed.x*100/(backgrounds?.width||1920)));r.style.y=Math.max(0,Math.min(100,placed.y*100/(backgrounds?.height||1080)));}}M.updateGroup(rules,current(),{[key]:value});
    });
  });
  $('#group-controls').addEventListener('click',e=>{const b=e.target.closest('button');if(!b)return;if(b.dataset.groupMember){active=b.dataset.groupMember;selected=M.flatten(current().roots).find(n=>n.type==='show')?.id;modifierEditor=null;resetSession();render();return;}if(b.dataset.groupMove)mutate(()=>{const ms=M.groupMembers(rules,current()),index=ms.findIndex(r=>r.id===b.dataset.groupMove);if(index>0){const a=rules.indexOf(ms[index]),b=rules.indexOf(ms[index-1]);[rules[a],rules[b]]=[rules[b],rules[a]];}});});
  function renderGroupPreview(override){
    const repeated=M.hasEach(M.executionRule(current()).roots),group=current().style.group||{mode:repeated?'row':'static',sort:'order',reverse:false,gap:8,columns:4,x:current().style.x,y:current().style.y},members=M.groupMembers(rules,current()),sample=sampleState(),width=backgrounds?.width||1920,height=backgrounds?.height||1080,now=Date.now();
    const entries=running?[...displays.values()].filter(d=>d.end>now).map(d=>({rule:rules.find(r=>r.id===d.ruleID)||current(),id:d.blockId,actor:d.actor,triggerContext:d.triggerContext,key:d.key,started:d.started,elapsed:(now-d.started)/1000,props:d.props,remaining:(d.end-now)/1000,glow:d.glow})):members.flatMap(rule=>{
      const n=rule.id===current().id?previewNode():M.flatten(M.executionRule(rule).roots).find(n=>n.type==='show');if(!n)return [];
      const base={rule,id:n.id,props:n.props,remaining:n.props.duration||6};return M.repeatedNode(rule,n.id)?M.cloneMembers(sample,M.nodeScope(rule,n.id)).map(m=>({...base,actor:m.actor})):[base];
    });
    const items=entries.flatMap(entry=>{
      if(!members.includes(entry.rule))return [];let context=entry.actor?M.cloneContext(sample,entry.actor):sample;if(!context)return [];context=M.triggerSample(context,entry.triggerContext);
      const original=entry.rule.id===current().id?override:entry.rule.style,base=entry.glow?{...original,color:entry.glow}:original,a=M.resolveAppearance(entry.rule,context,base),props=M.resolveShow(entry.rule,entry.id,{...entry.props,...(a.text!==undefined?{text:a.text}:{})},context,spellNames);
      let readable=props.visible;
      if(props.display==='image'&&!media.library.assets.some(item=>item.id===a.style.visual?.image&&item.kind==='image'&&item.available&&M.spriteFits(a.style.visual?.sprite,item)))readable=false;
      const arc=['arc','ring'].includes(props.display)?M.arcState(props.fraction!==undefined?{...a.style,arc:{...(a.style.arc||M.arcDefaults(a.style,props.display)),fill:'timer'}}:a.style,props.display,props.fraction??1,context):{count:1,fraction:1};
      readable=readable&&arc!==null;
      if(!readable||!a.visible||!arc.count||arc.fraction<=0){if(running&&entry.key)endPreview(entry.key,displays.get(entry.key),readable,true);return [];}
      if(entry.key)retiredDisplays.delete(entry.key);
      return [{...entry,order:rules.indexOf(entry.rule),name:entry.rule.name,style:a.style,context,rawProps:entry.props,glow:a.glow??!!entry.glow,props,remaining:props.remaining??(entry.props.visibility==='conditions'||M.dataSource(entry.rule,entry.id).kind!=='none'?undefined:entry.remaining),stacks:props.stacks,html:displayHTML(entry.props,entry.remaining,!!entry.glow,base,entry.id,entry.rule,context,entry.elapsed??null)}];
    });
    let placed=M.groupLayout(items,group,width,height);
    if(!current().style.group&&!current().style.anchor&&!repeated)placed=placed.map(p=>({...p,...M.previewPosition(p.style,p,width,height)}));
    if(current().style.anchor?.target==='current'){
      const batches=new Map();for(const p of placed){const key=p.actor||'';if(!batches.has(key))batches.set(key,[]);batches.get(key).push(p);}
      placed=[...batches].flatMap(([actor,ps])=>M.placeAnchor(ps,current().style.anchor,previewAnchor(actor)));
    }else placed=M.placeAnchor(placed,current().style.anchor,previewAnchor());
    const target=$('#overlay-preview');previewPlacement=[...placed];
    if(running){renderedDisplays.clear();for(const p of placed)if(p.key&&p.style.visual?.motion?.finish)renderedDisplays.set(p.key,{...p,screenWidth:width,screenHeight:height,anchor:previewFinishAnchor(p.rule,p.actor)});
      for(const [key,p]of retiredDisplays){
        const rule=rules.find(r=>r.id===p.rule.id),d={ruleID:p.rule.id,blockId:p.id,actor:p.actor,triggerContext:p.triggerContext,props:p.rawProps};
        if(!rule||p.screenWidth!==width||p.screenHeight!==height||!members.some(r=>r.id===rule.id)||p.end<=now||!previewFinishReadable(d)){retiredDisplays.delete(key);continue;}
        let x=p.x,y=p.y;if(p.anchor){const anchor=previewFinishAnchor(rule,p.actor);if(!anchor){retiredDisplays.delete(key);continue;}x+=anchor[0]-p.anchor[0];y+=anchor[1]-p.anchor[1];}
        if(placed.length>=64){retiredDisplays.delete(key);continue;}
        placed.push({...p,x,y,finishing:true,html:displayHTML(p.props,p.remaining,p.glow,p.style,p.id,p.rule,p.context,(now-p.started)/1000,(p.finishing-p.started)/1000,true)});
      }
    }
    const x=placed.length?Math.min(...placed.map(p=>p.x)):group.x*width/100,y=placed.length?Math.min(...placed.map(p=>p.y)):group.y*height/100;
    const w=placed.length?Math.max(...placed.map(p=>p.x+p.width))-x:1,h=placed.length?Math.max(...placed.map(p=>p.y+p.height))-y:1;previewBounds={x,y};
    target.style.width=w+'px';target.style.height=h+'px';target.innerHTML=[...placed].sort((a,b)=>(a.style.visual?.layer||0)-(b.style.visual?.layer||0)).map(p=>`<div data-preview-rule="${p.rule.id}" data-preview-actor="${esc(p.actor||'')}" ${p.finishing?'data-preview-finishing="true"':''} style="position:absolute;left:${p.x-x}px;top:${p.y-y}px;--accent:${M.colorValue(p.style.color)}">${p.html}</div>`).join('');
  }
  function movePreview(x,y,baseline=rules){
    const group=current().style.group;
    if(current().style.anchor){const dx=(x-(previewBounds?.x||0)*100/(backgrounds?.width||1920))*(backgrounds?.width||1920)/100,dy=(y-(previewBounds?.y||0)*100/(backgrounds?.height||1080))*(backgrounds?.height||1080)/100;setAnchor({...current().style.anchor,x:Math.max(-4000,Math.min(4000,Math.round((current().style.anchor.x+dx)*10)/10)),y:Math.max(-4000,Math.min(4000,Math.round((current().style.anchor.y+dy)*10)/10))});return;}
    if(group&&group.mode!=='static'){M.updateGroup(rules,current(),{x,y});return;}
    if(group){const original=baseline.find(r=>r.id===current().id),members=M.groupMembers(baseline,original),width=backgrounds?.width||1920,height=backgrounds?.height||1080,minX=Math.min(...members.map(r=>r.style.x)),minY=Math.min(...members.map(r=>r.style.y));for(const r of M.groupMembers(rules,current())){const old=baseline.find(v=>v.id===r.id);r.style.x=Math.max(0,Math.min(100,old.style.x+x-minX));r.style.y=Math.max(0,Math.min(100,old.style.y+y-minY));}M.updateGroup(rules,current(),{x,y});return;}
    current().style.x=x;current().style.y=y;
  }
  function renderAppearance() {
    const r=current();if(!r)return;
    const textSize = textDisplay(), size = textSize?r.style.font:r.style.size;
    $('#size-label').textContent=textSize?'Text size':'Display size';
    $('#appearance-size').setAttribute('aria-label',textSize?'Text size':'Display size');
    $('#appearance-size').min=textSize?10:16;$('#appearance-size').max=textSize?72:400;$('#appearance-size').step=textSize?1:2;
    $('#appearance-size').value=size;$('#size-value').textContent=size+' px';$('#font-field').hidden=textSize||['arc','ring','icon'].includes(previewNode()?.props.display);
    if(![...$('#appearance-font').options].some(o=>+o.value===r.style.font))$('#appearance-font').add(new Option(r.style.font+' px',r.style.font));
    $('#appearance-opacity').value = r.style.opacity; $('#opacity-value').textContent = r.style.opacity + '%';
    $('#appearance-color').value = Object.hasOwn(M.palette,r.style.color)?r.style.color:'custom';$('#appearance-custom-color').hidden=Object.hasOwn(M.palette,r.style.color);$('#appearance-custom-color').value=M.colorValue(r.style.color)||'#ffffff'; $('#appearance-font').value = r.style.font; $('#appearance-transparent').checked = r.style.transparent;
    renderArcControls(); renderShowControls(); renderVisualControls(); renderGroupControls(); renderAnchorControls(); renderModifiers(); renderPredicateControls();$('#lifecycle-controls').hidden=!selectedShow();for(const el of $$('[data-lifecycle]')){const value=r.style[el.dataset.lifecycle]||'';el.innerHTML=option('','None',value)+Object.entries(soundChoices(value)).map(([k,v])=>option(k,v,value)).join('');}
  }
  function render() {
    const r = current();
    packEditor.render();
    document.body.dataset.empty=String(!r);$('#empty-library').hidden=!!r;$('#export-open').disabled=!r;
    $$('button[data-view]').forEach(b=>b.disabled=!r);
    if(!r){$('#tree').replaceChildren();$('#selection-panel').replaceChildren();$('#load-panel').replaceChildren();loadPicker=null;loadPickerRule=null;renderLibrary();renderPalette();valuesEditor.render();renderPreview();status();return;}
    $('#rule-name').value = r.name; $('#enabled').checked = r.enabled;
    $('#tree').innerHTML = treeList(r.roots);
    $('#rule-summary').textContent = `${M.flatten(r.roots).length} blocks`;
    renderAppearance();valuesEditor.render();
    backgrounds?.refresh(); renderLibrary(); renderPalette(); renderSelection(); renderPreview(); renderLoad(); status();
  }
  function parentForAdd(type) {
    if(type==='action')return null;
    const hit = M.find(current().roots,selected);
    if (hit) return hit.node.children ? hit.node.id : hit.parent?.id;
    return ['when','action'].includes(type) ? null : current().roots.find(n=>n.type==='when')?.id;
  }
  function addBlock(type, root = false) {
    if(!current())return;
    mutate(() => {
      let parent = root ? null : parentForAdd(type);
      if (!['when','action'].includes(type) && !parent) { const root = M.node('when'); current().roots.push(root); parent = root.id; }
      const list = parent ? M.find(current().roots,parent).node.children : current().roots;
      selected = M.insert(current(),{type},parent,list.length);
    });
    document.getElementById(selected)?.scrollIntoView({block:'nearest',behavior:'smooth'});
  }
  function editSelected(action) {
    mutate(() => {
      const hit = M.find(current().roots,selected); if (!hit) return;
      if (action === 'delete') { if(hit.node.type==='action'&&M.flatten(current().roots).some(n=>n.type==='call'&&n.props.action===hit.node.id))throw Error('This action is still in use.');hit.list.splice(hit.index,1); selected = hit.parent?.id || null; }
      if (action === 'copy') { if (M.flatten(current().roots).length + M.flatten([hit.node]).length > 80) throw Error('This overlay has reached the block limit.'); const copy=M.copyNode(hit.node); hit.list.splice(hit.index+1,0,copy); selected=copy.id; }
      if (action === 'up' && hit.index > 0) [hit.list[hit.index-1],hit.list[hit.index]]=[hit.list[hit.index],hit.list[hit.index-1]];
      if (action === 'down' && hit.index < hit.list.length-1) [hit.list[hit.index+1],hit.list[hit.index]]=[hit.list[hit.index],hit.list[hit.index+1]];
    });
  }
  function resetTest() {
    timerIds.forEach(clearTimeout); timerIds=[]; runSerial++; running=false; displays.clear();retiredDisplays.clear();renderedDisplays.clear(); modifierSoundLatches.clear(); previewVisibility.clear(); lastTrace=[];
    $$('.block.matched,.block.failed,.block.waiting').forEach(el => el.classList.remove('matched','failed','waiting'));
    $('#test-result').className='test-result'; $('#test-result').textContent='';
    renderWatchStatus();
    $('#preview-state').textContent='Layout preview'; renderPreview();
  }
  function resetSession() { testSession=M.createSession();groupTestSessions.clear(); resetTest(); }
  function renderWatchStatus() {
    const count=testSession.active.size;
    $('#watch-status').textContent=count?`${count} active watcher${count===1?'':'s'}`:'';
    $('#watch-status').hidden=!count;
  }
  let spriteMaskSerial=0;
  const shield = '<svg viewBox="4 1 40 46" fill="none" aria-hidden="true"><path d="M24 3 42 10 42 22C42 33 32 40 24 45C16 40 6 33 6 22L6 10Z M24 13V33 M16 22H32" stroke="currentColor" stroke-width="2" vector-effect="non-scaling-stroke"/></svg>';
  function displayHTML(props, remaining, glowing, s=current().style,nodeID=previewNode()?.id,rule=current(),sample=sampleState(),elapsed=null,finishAt=null,frozen=false) {
    const source=M.dataSource(rule,nodeID);if(['spell','buff','debuff'].includes(source.kind))requestDisplayName(source.id);
    if(source.kind==='casting'){const unit=source.unit==='current'?sample.unit:source.unit;requestDisplayName((sample.playerContext||sample).casts?.find(c=>c.unit===unit)?.spell);}
    if(['castEvent','trigger'].includes(source.kind))requestDisplayName(sample.castEvent?.spell);
    if(sample.bossEvent)requestDisplayName(sample.bossEvent.timer?.spell);
    if(['bossTimer','bossEvent'].includes(source.kind))for(const t of (sample.playerContext||sample).bosses?.timers||[])requestDisplayName(t.spell);
    if(sample.combatEvent){requestDisplayName(sample.combatEvent.spell);requestDisplayName(sample.combatEvent.extraSpell);}
    const appearance=frozen?{style:s,visible:true}:M.resolveAppearance(rule,sample,s);s=appearance.style;glowing=appearance.glow??glowing;
    if(appearance.text!==undefined)props={...props,text:appearance.text};
    if(!frozen)props=M.resolveShow(rule,nodeID,props,sample,spellNames);
    const kind=props.display, fraction=props.fraction??(props.visibility==='conditions'?1:Math.max(0,Math.min(1,remaining/(+props.duration||6))));
    const size=s.size, font=s.font, geometry=M.displayGeometry(props,s),v=s.visual||{},motion=M.Motion.sample(v.motion,elapsed,M.colorValue(s.color),M.colorValue,finishAt),rect=M.visualProgress(fraction,v.direction),track=M.colorValue(v.trackColor)||'#101923';
    const progressStyle=`left:${rect.x}%;top:${rect.y}%;width:${rect.width}%;height:${rect.height}%;`;
    let content,visible=props.visible&&appearance.visible;
    const iconHTML=()=>props.spell?`<img class="spell-art" src="/api/overlays/icon?spell=${props.spell}" alt="Spell icon" draggable="false">`:props.icon==='auto'?'<span class="unknown-icon">?</span>':shield;
    if(['ring','arc'].includes(kind)){
      const archStyle=props.fraction!==undefined?{...s,arc:{...(s.arc||M.arcDefaults(s,kind)),fill:'timer'}}:s;
      const paths=visible?M.arcPaths(archStyle,kind,fraction,sample):[];visible=paths.length>0;
      content=`<svg class="preview-shape" width="${geometry.width}" height="${geometry.height}" viewBox="0 0 ${size} ${size}" preserveAspectRatio="none" aria-hidden="true">${paths.map(p=>`<path d="${p.d}" fill="none" stroke="currentColor" stroke-width="${p.thickness}" stroke-linecap="round"/>`).join('')}</svg>`;
    }else if(kind==='shape'){
      const path=M.shapePaths[v.shape]||M.shapePaths.rectangle,svg=(color,opacity,clip='')=>`<svg class="visual-shape" viewBox="0 0 100 100" preserveAspectRatio="none" style="color:${color};opacity:${opacity};${clip}" aria-hidden="true"><path d="${path}" fill="currentColor"/></svg>`;
      content=svg(track,(v.trackOpacity??0)/100)+svg('currentColor',(v.fillOpacity??100)/100,`clip-path:inset(${rect.y}% ${100-rect.x-rect.width}% ${100-rect.y-rect.height}% ${rect.x}%);`);
    }else if(kind==='image'){
      const asset=media.library.assets.find(a=>a.id===v.image&&a.kind==='image'&&a.available);visible=visible&&!!asset&&M.spriteFits(v.sprite,asset);
      const url=asset?'/api/overlays/media/file?id='+asset.id:'',fit=v.fit||'contain',clip=v.sprite?.mode==='progress'?'':`clip-path:inset(${rect.y}% ${100-rect.x-rect.width}% ${100-rect.y-rect.height}% ${rect.x}%);`;
      if(asset&&v.sprite){
        const crop=M.spriteRect(v.sprite,asset,elapsed??0,fraction),id='sprite-mask-'+(++spriteMaskSerial),preserve=fit==='stretch'?'none':'xMidYMid '+(fit==='cover'?'slice':'meet'),img=`<image href="${url}" width="${asset.width}" height="${asset.height}"/>`;
        content=!crop?'':`<svg class="media-image" data-sprite-frame="${crop.frame}" viewBox="${crop.x} ${crop.y} ${crop.width} ${crop.height}" preserveAspectRatio="${preserve}" style="overflow:hidden;${clip}" aria-hidden="true">${v.tint?`<defs><mask id="${id}" maskUnits="userSpaceOnUse" maskContentUnits="userSpaceOnUse" x="${crop.x}" y="${crop.y}" width="${crop.width}" height="${crop.height}" style="mask-type:alpha">${img}</mask></defs><rect x="${crop.x}" y="${crop.y}" width="${crop.width}" height="${crop.height}" fill="currentColor" mask="url(#${id})"/>`:`<defs><clipPath id="${id}-clip"><rect x="${crop.x}" y="${crop.y}" width="${crop.width}" height="${crop.height}"/></clipPath></defs><g clip-path="url(#${id}-clip)">${img}</g>`}</svg>`;
      }else content=!asset?'':v.tint?`<span class="media-image media-mask" style="mask-image:url('${url}');mask-size:${fit==='stretch'?'100% 100%':fit};${clip}"></span>`:`<img class="media-image" src="${url}" draggable="false" alt="" style="object-fit:${fit==='stretch'?'fill':fit};${clip}">`;
    }else if(kind==='bar'){
      const b=M.visualBarLayout(geometry,v),icon=b.icon?`<span class="bar-icon" style="width:${b.icon}px;height:${b.icon}px;${b.side==='right'?'right:0;':b.side==='bottom'?'bottom:0;':'left:0;top:0;'}">${iconHTML()}</span>`:'';
      content=icon+`<span class="bar-content" style="left:${b.left}px;right:${b.right}px;top:${b.top}px;bottom:${b.bottom}px;"><span class="bar-track" style="background:${track};opacity:${(v.trackOpacity??0)/100}"></span><span class="bar-fill" style="${progressStyle}opacity:${(v.fillOpacity??30)/100}"></span><span class="preview-label" style="text-align:${v.textAlign||'center'}">${esc(props.text)}</span></span>`;
    }else content=kind==='icon'?iconHTML():`<span class="preview-label" style="text-align:${v.textAlign||'center'}">${esc(props.text)}</span>`;
    return `<div class="preview-display ${kind} ${s.transparent?'transparent':''} ${glowing?'glowing':''} ${['text','bar'].includes(kind)?'text-effect-'+(v.textEffect||'shadow'):''}" style="--accent:${motion.color};opacity:${s.opacity/100*motion.alpha};width:${geometry.width}px;height:${geometry.height}px;font-size:${font}px;${['text','bar'].includes(kind)?`font-family:'${M.fontFamilies[v.fontFamily]||M.fontFamilies.ui}',sans-serif;font-weight:${v.bold===false?400:700};font-style:${v.italic?'italic':'normal'};`:''}visibility:${visible?'visible':'hidden'};transform:translate(${motion.x}px,${motion.y}px) rotate(${(v.rotation||0)+motion.rotation}deg) scale(${(v.flipX?-1:1)*motion.scaleX},${(v.flipY?-1:1)*motion.scaleY});">${content}</div>`;
  }

  function previewFinishAnchor(rule,actor){
    const a=rule.style.anchor;if(!a)return null;const rect=previewAnchor(actor);if(!rect?.available)return null;const [x,y]=M.anchorPoint(a.point);return [rect.x+rect.width*x,rect.y+rect.height*y];
  }
  function previewFinishReadable(d){
    const rule=rules.find(r=>r.id===d.ruleID);if(!rule?.enabled)return false;const sample=sampleState();let context=d.actor?M.cloneContext(sample,d.actor):sample;if(!context)return false;context=M.triggerSample(context,d.triggerContext);
    if(M.displayState(rule,d.blockId,context)===null||rule.style.anchor&&!previewFinishAnchor(rule,d.actor))return false;
    if(d.props.display==='image'&&!media.library.assets.some(a=>a.available&&a.id===rule.style.visual?.image))return false;
    if((rule.style.modifiers||[]).some(m=>m.visible!==undefined&&M.condition({props:m.condition},{...context,overlayLogic:rule.logic,overlayMemory:context.overlayMemories?.get(rule.id)})===null))return false;
    const props=M.resolveShow(rule,d.blockId,d.props,context,spellNames);
    if(['arc','ring'].includes(props.display)&&M.arcState(props.fraction!==undefined?{...rule.style,arc:{...(rule.style.arc||M.arcDefaults(rule.style,props.display)),fill:'timer'}}:rule.style,props.display,props.fraction??1,context)===null)return false;
    return M.resolveShow(rule,d.blockId,d.props,context,spellNames).visible;
  }
  function endPreview(key,d,confirmed,keepActive=false){
    if(!d)return;
    if(confirmed&&retiredDisplays.has(key)){if(!keepActive)displays.delete(key);return;}
    const p=renderedDisplays.get(key),now=Date.now(),effect=p?.style.visual?.motion?.finish;
    if(confirmed&&p&&effect&&previewFinishReadable(d)){
      const copy=M.clone(p);copy.started=d.started;copy.finishing=now;copy.end=now+effect.duration*1000;
      copy.remaining=d.end===Infinity?1:Math.max(0,(d.end-now)/1000);
      if(retiredDisplays.size>=64)retiredDisplays.delete(retiredDisplays.keys().next().value);retiredDisplays.set(key,copy);
    }else retiredDisplays.delete(key);
    if(!keepActive)displays.delete(key);renderedDisplays.delete(key);
  }
  function renderPreview(style=current()?.style) {
    valuesEditor.refresh();
    const target=$('#overlay-preview'), s=style;
    if(!s){target.replaceChildren();return;}
    target.style.opacity=1;  target.style.setProperty('--accent',M.colorValue(s.color));
    target.classList.toggle('ghost',!running);
    if(running){const now=Date.now();for(const [key,d]of displays)if(d.end<=now)endPreview(key,d,true);}
    if(current().style.group||current().style.anchor||M.hasEach(M.executionRule(current()).roots)||current().style.visual?.motion?.finish){renderGroupPreview(s);updateModifierSounds();clampPreview();return;}
    previewBounds=null;previewPlacement=[];target.style.width='';target.style.height='';
    if (!running) {
      const n=previewNode();
      target.innerHTML=displayHTML(n?.props||{display:'text',text:'Your display'},+(n?.props.duration||6),false,s);
    } else {
      const now=Date.now(), visible=[...displays.values()].filter(d=>d.end>now);
      target.innerHTML=visible.map(d=>`<div style="--accent:${M.colorValue(d.glow||s.color)}">${displayHTML(d.props,(d.end-now)/1000,!!d.glow,d.glow?{...s,color:d.glow}:s,d.blockId,current(),M.triggerSample(sampleState(),d.triggerContext),(now-d.started)/1000)}</div>`).join('');
      if (!visible.length && displays.size) $('#preview-state').textContent='Display finished';
    }
    updateModifierSounds();clampPreview();
  }
  function clampPreview() {
    if(!current())return;
    const target=$('#overlay-preview'), scene=$('#scene'), width=backgrounds?.width||1920,height=backgrounds?.height||1080, scale=scene.getBoundingClientRect().width/width;
    target.style.transform=`scale(${scale})`;
    const position=previewBounds||M.previewPosition(current().style,{width:target.offsetWidth,height:target.offsetHeight},width,height);
    target.style.left=position.x*scale+'px';target.style.top=position.y*scale+'px';
  }
  function playSound(name) { if(!$('#audio-enabled').checked)return;const audio=new Audio(name.startsWith('media:')?'/api/overlays/media/file?id='+name.slice(6):name==='sonar'?'/api/overlays/sound/sonar':'sounds/'+name+'.wav');audio.volume=.6;audio.play().catch(()=>notify('Sound unavailable.')); }

  function updateModifierSounds(){
    const sample=sampleState(),next=new Map();
    for(const rule of M.groupMembers(rules,current())){
      const actors=new Set(running?[...displays.values()].filter(d=>(d.ruleID||current().id)===rule.id&&d.end>Date.now()).map(d=>d.actor||''):[]);
      const visible=running&&[...$('#overlay-preview').querySelectorAll('.preview-display')].some(e=>e.style.visibility!=='hidden'&&!e.closest('[data-preview-finishing="true"]')&&(e.closest('[data-preview-rule]')?.dataset.previewRule||current().id)===rule.id);
      if(visible!==!!previewVisibility.get(rule.id)){const sound=rule.style[visible?'onShow':'onHide'];if(sound)playSound(sound);}previewVisibility.set(rule.id,visible);
      for(const actor of actors){const context=actor?M.cloneContext(sample,actor):sample;if(!context)continue;
        for(const [i,m]of (rule.style.modifiers||[]).entries()){
          const key=rule.id+'/'+i+'/'+actor,contexts=[...displays.values()].filter(d=>(d.ruleID||current().id)===rule.id&&(d.actor||'')===actor).map(d=>M.triggerSample(context,d.triggerContext)),matched=contexts.some(c=>M.condition({props:m.condition},{...c,overlayLogic:rule.logic,overlayMemory:c.overlayMemories?.get(rule.id)})===true);
          if(matched&&!modifierSoundLatches.get(key)&&m.sound)playSound(m.sound);next.set(key,matched);
        }
      }
    }
    modifierSoundLatches.clear();for(const [key,value]of next)modifierSoundLatches.set(key,value);
  }

  function sampleState(event=$('#sample-event').value) {
    const role=$('#sample-role').value, health=+$('#sample-health').value,healthUnit=$('#sample-health-unit').value,resourceUnit=$('#sample-resource-unit').value;
    const state=$('#sample-aura-state').value,auraUnit=$('#sample-aura-unit').value;
    const auras=state==='unknown'?null:state==='missing'?[]:[{unit:auraUnit,name:$('#sample-aura-name').value.trim(),kind:$('#sample-aura-kind').value,stacks:+$('#sample-aura-stacks').value,duration:+$('#sample-aura-duration').value,remaining:$('#sample-aura-permanent').checked?null:+$('#sample-aura-remaining').value,permanent:$('#sample-aura-permanent').checked}];
    const resource=$('#sample-resource').value;const resources={[resource]:{current:+$('#sample-resource-current').value,max:+$('#sample-resource-max').value}};
    const group=$('#sample-group').value,count=Math.max(1,Math.min(40,+$('#sample-members').value||3));
    const facts=m=>({...m,health:m.unit===healthUnit?health:null,resources:m.unit===resourceUnit?resources:null,primaryPower:m.unit===resourceUnit?resource:null,auras:state==='unknown'?null:m.unit===auraUnit?auras:[]});
    let slot=0;const baseParty=[{actor:'sample-tank',name:'Stoneguard',role:'tank'},{actor:'sample-healer',name:'Dreamweaver',role:'healer'},{actor:'sample-damage',name:'Emberfall',role:'damage'},{actor:'sample-four',name:'Bramble',role:'damage'},{actor:'sample-five',name:'Skylark',role:'damage'}];
    const playerActor='sample-'+role,others=baseParty.filter(m=>m.actor!==playerActor).slice(0,Math.min(5,count)-1);
    const party=baseParty.filter(m=>m.actor===playerActor||others.includes(m)).map(member=>facts({...member,unit:member.actor===playerActor?'player':'party'+(++slot)}));
    const units=[...party,...['raid','boss','arena','nameplates'].flatMap(scope=>M.scopeTokens(scope).slice(0,count).map((unit,i)=>facts({unit,actor:scope==='raid'&&i===0?playerActor:'sample-'+unit,name:scope==='raid'&&i===0?party.find(m=>m.unit==='player').name:M.unitChoices[unit],role:scope==='raid'?i===0?role:['tank','healer','damage'][i%3]:null})))];

    const cooldowns=$('#sample-cooldown-state').value==='unknown'?[]:[{kind:$('#sample-cooldown-kind').value,id:+$('#sample-cooldown-id').value,available:true,enabled:true,duration:+$('#sample-cooldown-duration').value,rechargeDuration:+$('#sample-cooldown-recharge-duration').value,remaining:+$('#sample-cooldown-remaining').value,charges:+$('#sample-cooldown-charges').value,maxCharges:+$('#sample-cooldown-max').value,recharge:+$('#sample-cooldown-recharge').value,usable:$('#sample-cooldown-usable').checked}];
    const castUnit=$('#sample-cast-unit').value,castMode=$('#sample-cast-state').value,castSpell=+$('#sample-cast-id').value;
    const casts=castMode==='unknown'?[]:[{unit:castUnit,available:true,mode:+castMode,spell:castMode==='0'?0:castSpell,remaining:+$('#sample-cast-remaining').value,duration:+$('#sample-cast-duration').value,interruptible:$('#sample-cast-interruptible').value==='unknown'?null:$('#sample-cast-interruptible').value==='yes'}];
    const castEvent=Object.hasOwn(M.castEvents,event)?{unit:castUnit,spell:castSpell,kind:event}:null;
    const identities=Object.fromEntries(Object.keys(M.unitChoices).filter(u=>u!=='current').map(u=>[u,'sample-'+u]));
    const actor=u=>({id:identities[u],name:party.find(m=>m.unit===u)?.name||M.unitChoices[u]||u});
    const logSpell=+$('#sample-log-spell').value,extraSpell=+$('#sample-log-extraSpell').value,critical=$('#sample-log-critical').value;
    const values={};if($('#sample-log-known').checked)values[$('#sample-log-field').value]=+$('#sample-log-value').value;if(critical!=='unknown')values.critical=critical==='yes'?1:0;
    const combatEvent={kind:$('#sample-log-kind').value,source:actor($('#sample-log-source').value),target:actor($('#sample-log-target').value),spell:logSpell,name:spellNames[logSpell]||`Spell ${logSpell}`,extraSpell,extraName:extraSpell?(spellNames[extraSpell]||`Spell ${extraSpell}`):'',values,critical:critical==='unknown'?null:critical==='yes',school:+$('#sample-log-school').value,periodic:$('#sample-log-periodic').checked,auraKind:$('#sample-log-aura').value,miss:$('#sample-log-miss').value};
    $('#sample-boss-source-field').hidden=$('#sample-boss-provider').value!=='blizzard';
    const equipment={slots:Array.from({length:19},(_,i)=>({slot:i+1,item:0})),sets:[{name:$('#sample-equipment-name').value,equipped:$('#sample-equipment-outfit').checked}]};
    const gearSlot=+$('#sample-equipment-slot').value;equipment.slots[gearSlot-1]=$('#sample-equipment-known').checked?{slot:gearSlot,item:+$('#sample-equipment-item').value,level:+$('#sample-equipment-level').value,set:+$('#sample-equipment-set').value,class:+$('#sample-equipment-class').value,subclass:+$('#sample-equipment-subclass').value}:{slot:gearSlot};
    const talentKind=$('#sample-talent-kind').value,talentID=+$('#sample-talent-id').value;
    const talents=[{kind:talentKind,id:talentID,rank:$('#sample-talent-known').checked?+$('#sample-talent-rank').value:null,spell:['spell','pvp','known'].includes(talentKind)?talentID:+$('#sample-talent-spell').value}];
    const bossProvider=$('#sample-boss-provider').value,bossState=$('#sample-boss-state').value,bossValue=id=>$('#'+id).value===''?null:+$('#'+id).value;
    const bossTimer={provider:bossProvider,id:1,source:bossProvider==='blizzard'?$('#sample-boss-source').value:undefined,name:$('#sample-boss-name').value,spell:bossValue('sample-boss-id'),kind:$('#sample-boss-kind').value,count:bossValue('sample-boss-count'),duration:bossValue('sample-boss-duration'),remaining:bossValue('sample-boss-remaining'),paused:bossState==='paused'};
    const bossProviderState={name:bossProvider,available:$('#sample-boss-available').checked,complete:bossState!=='unknown',stage:bossProvider==='blizzard'?null:bossValue('sample-boss-stage'),stageTotal:null,encounterID:bossProvider==='blizzard'?null:bossValue('sample-boss-encounter')};
    const bosses={providers:[bossProviderState],timers:['running','paused'].includes(bossState)?[bossTimer]:[]};
    const bossEvent=Object.hasOwn(M.bossEvents,event)?{kind:event,timer:{...bossTimer,id:['bosswarning','bossstage'].includes(event)?0:1},stage:event==='bossstage'?bossProviderState.stage:null,encounterID:bossProviderState.encounterID}:null;
    const encounter={active:$('#sample-encounter-state').value==='unknown'?null:$('#sample-encounter-state').value==='active',id:+$('#sample-encounter-id').value,name:$('#sample-encounter-name').value,difficulty:+$('#sample-encounter-difficulty').value,groupSize:+$('#sample-encounter-size').value,elapsed:$('#sample-encounter-elapsed').value===''?null:+$('#sample-encounter-elapsed').value};
    const encounterEvent=Object.hasOwn(M.encounterEvents,event)?{...encounter,kind:event,success:event==='encounterend'&&$('#sample-encounter-outcome').value!=='unknown'?$('#sample-encounter-outcome').value==='victory':null}:null;
    const unitSamples={[healthUnit]:{health}};unitSamples[resourceUnit]={...unitSamples[resourceUnit],resources,primaryPower:resource};
    const petState=$('#sample-pet-state').value,pet=petState==='unknown'?{}:petState==='missing'?{exists:false}:{exists:true,alive:petState==='alive'?true:petState==='dead'?false:null,family:$('#sample-pet-family').value,specialization:$('#sample-pet-specialization').value,creatureType:$('#sample-pet-type').value};
    return {petVersion:1,pet,overlayMemories:new Map([[current()?.id,testSession.memory],...[...groupTestSessions].map(([key,s])=>[key,s.memory])]),event,role,equipment,talents,casts,castEvent,encounter,encounterEvent,bosses,bossEvent,combatEvent:event==='combatlog'?combatEvent:null,identities,health:healthUnit==='player'?health:null,auraUnit,cooldowns,resources:resourceUnit==='player'?resources:null,unitSamples,primaryPower:resource,location:$('#sample-location').value,actor:'sample-'+role,unit:'player',name:party.find(m=>m.unit==='player').name,auras,party,units,group};
  }
  function updateSample() {
    if(running) {
      const sample=sampleState();
      for(const [key,d]of displays){let context=d.actor?M.cloneContext(sample,d.actor):sample;if(context)context=M.triggerSample(context,d.triggerContext);const state=context?M.displayState(rules.find(r=>r.id===d.ruleID)||current(),d.blockId,context):null;if(!context||d.props.visibility==='conditions'&&state!==true)endPreview(key,d,state===false);}
      renderPreview();if(!displays.size)$('#preview-state').textContent='Conditions not met';
    } else resetTest();
  }
  function runTest() {
    if(!current())return;
    for (const input of $$('#sample-aura-controls input')) {
      if (!input.disabled && !input.checkValidity()) { $('#sample-aura-controls').open=true; input.reportValidity(); return; }
    }
    resetTest(); running=true; const serial=runSerial;
    const event=$('#sample-event').value;
    if(event==='leave')$('#sample-location').value='world';
    const sample=sampleState(event);
    const results=M.groupMembers(rules,current()).map(rule=>{let session=testSession;if(rule.id!==current().id){if(!groupTestSessions.has(rule.id))groupTestSessions.set(rule.id,M.createSession());session=groupTestSessions.get(rule.id);}const result=M.evaluate(rule,sample,session);result.actions=result.actions.map(a=>({...a,ruleID:rule.id}));return result;});
    const result={actions:results.flatMap(r=>r.actions),trace:results.flatMap(r=>r.trace),waiting:results.flatMap(r=>r.waiting),reason:results.every(r=>!r.actions.length)?results[0]?.reason:''};lastTrace=result.trace;
    for(const t of result.trace){const el=document.getElementById(t.id);if(el){if(t.matched){el.classList.add('matched');el.classList.remove('failed','waiting');}else if(!el.classList.contains('matched'))el.classList.add(t.waiting?'waiting':'failed');}}
    renderWatchStatus();
    $('#test-result').className='test-result '+(result.actions.length?'success':result.waiting.length?'':'failure');
    const sounds=result.actions.filter(a=>a.type==='sound').length;
    $('#test-result').textContent=result.reason||`${result.actions.length} action${result.actions.length===1?'':'s'} matched.${sounds&&!$('#audio-enabled').checked?' Sound muted.':''}`;
    $('#preview-state').textContent=result.actions.length?'Test running':result.waiting.length?'Watching':'No matching display';
    let batching=true;
    for(const action of result.actions){
      const apply=()=>{
        if(serial!==runSerial)return;
        if(action.at){const rule=rules.find(r=>r.id===action.ruleID),sample=sampleState(),context=action.cloneActor?M.cloneContext(sample,action.cloneActor):sample;if(!rule||!context||M.displayState(rule,action.blockId,M.triggerSample(context,action.triggerContext))!==true)return;}
        const prefix=action.ruleID+'/'+action.actor+'/',key=prefix+action.blockId;
        if(action.type==='show'){retiredDisplays.delete(key);renderedDisplays.delete(key);displays.set(key,{key,started:Date.now(),ruleID:action.ruleID,blockId:action.blockId,actor:action.cloneActor,triggerContext:action.triggerContext,props:action.props,end:action.props.visibility==='conditions'?Infinity:Date.now()+Number(action.props.duration)*1000,glow:null});}
        if(action.type==='hide')for(const [id,d]of displays)if(id.startsWith(prefix))endPreview(id,d,true);
        if(action.type==='glow')for(const [id,d]of displays)if(id.startsWith(prefix))d.glow=action.props.color;
        if(action.type==='sound')playSound(action.props.sound);
        if(!batching)renderPreview();
      };
      if(action.at)timerIds.push(setTimeout(apply,action.at));else apply();
    }
    batching=false;renderPreview();
  }
  function selectRuleSample() {
    if(!current())return;
    $('#sample-event').value=current().roots[0]?.props.event||'ability';
    const each=M.flatten(M.executionRule(current()).roots).find(n=>n.type==='each'),scope=each?.props.scope||'party',actual=scope==='group'?($('#sample-group').value==='raid'?'raid':'party'):scope,sampleCurrent=M.scopeTokens(actual)[0]||'player';
    const gear=M.flatten(current().roots).filter(n=>n.type==='if').flatMap(n=>M.predicateList(n.props)).find(p=>p.condition==='equipment');
    if(gear){$('#sample-equipment-controls').open=true;$('#sample-equipment-slot').value=gear.slot||13;if(gear.source==='item')$('#sample-equipment-item').value=gear.id;if(gear.source==='set')$('#sample-equipment-set').value=gear.id;if(gear.source==='saved')$('#sample-equipment-name').value=gear.name;if(gear.source==='type'){$('#sample-equipment-class').value=gear.itemClass;$('#sample-equipment-subclass').value=Math.max(0,gear.itemSubclass);}}
    const talent=M.flatten(current().roots).filter(n=>n.type==='if').flatMap(n=>M.predicateList(n.props)).find(p=>['talent','learned'].includes(p.condition));
    if(talent){$('#sample-talent-controls').open=true;$('#sample-talent-kind').value=M.talentWatch(talent).kind;$('#sample-talent-id').value=talent.id;$('#sample-talent-rank').value=talent.check==='missing'?0:talent.check==='rank'?talent.value:1;}
    const boss=M.flatten(current().roots).filter(n=>n.type==='if').flatMap(n=>M.predicateList(n.props)).find(p=>['bossTimer','bossStage'].includes(p.condition))||M.flatten(current().roots).find(n=>Object.hasOwn(M.bossEvents,n.props.event))?.props;
    if(boss){$('#sample-boss-controls').open=true;$('#sample-boss-provider').value=!boss.provider||boss.provider==='auto'?'dbm':boss.provider;if(boss.timelineSource&&boss.timelineSource!=='any')$('#sample-boss-source').value=boss.timelineSource;if(boss.spell)$('#sample-boss-id').value=boss.spell;if(boss.textFilter)$('#sample-boss-name').value=boss.textFilter;if(boss.encounterID)$('#sample-boss-encounter').value=boss.encounterID;if(boss.timerKind&&boss.timerKind!=='any')$('#sample-boss-kind').value=boss.timerKind;$('#sample-boss-state').value=boss.check==='missing'?'missing':boss.check==='paused'?'paused':'running';if(boss.check==='remaining')$('#sample-boss-remaining').value=boss.value;if(boss.condition==='bossStage')$('#sample-boss-stage').value=boss.value;}
    const encounter=M.flatten(current().roots).filter(n=>n.type==='if').flatMap(n=>M.predicateList(n.props)).find(p=>p.condition==='encounter')||M.flatten(current().roots).find(n=>Object.hasOwn(M.encounterEvents,n.props.event))?.props;
    if(encounter){$('#sample-encounter-controls').open=true;$('#sample-encounter-state').value=encounter.check==='idle'?'idle':'active';if(encounter.encounterID)$('#sample-encounter-id').value=encounter.encounterID;if(encounter.difficulty)$('#sample-encounter-difficulty').value=encounter.difficulty;if(encounter.check==='elapsed')$('#sample-encounter-elapsed').value=encounter.value;$('#sample-encounter-outcome').value=encounter.outcome||'victory';}
    const combat=M.flatten(current().roots).find(n=>n.type==='when'&&n.props.event==='combatlog')?.props;
    if(combat){$('#sample-log-controls').open=true;$('#sample-log-kind').value=combat.combatEvent;for(const k of ['spell','extraSpell'])if(combat[k])$('#sample-log-'+k).value=combat[k];for(const k of ['source','target'])if(![undefined,'','any'].includes(combat[k+'Unit']))$('#sample-log-'+k).value=combat[k+'Unit']==='current'?'player':combat[k+'Unit'];}
    const cast=M.flatten(current().roots).filter(n=>n.type==='if').flatMap(n=>M.predicateList(n.props)).find(p=>p.condition==='casting')||M.flatten(current().roots).find(n=>Object.hasOwn(M.castEvents,n.props.event))?.props;
    if(cast){$('#sample-cast-controls').open=true;$('#sample-cast-unit').value=cast.unit==='current'?sampleCurrent:cast.unit||'player';if(cast.spell)$('#sample-cast-id').value=cast.spell;}
    const cooldown=M.flatten(current().roots).filter(n=>n.type==='if').flatMap(n=>M.predicateList(n.props)).find(p=>p.condition==='cooldown');
    if(cooldown){$('#sample-cooldown-controls').open=true;$('#sample-cooldown-id').value=cooldown.id;$('#sample-cooldown-kind').value=cooldown.source;}
    const aura=M.flatten(current().roots).filter(n=>n.type==='if').flatMap(n=>M.predicateList(n.props)).find(p=>['buff','debuff'].includes(p.condition));
    if(aura){$('#sample-aura-unit').value=aura.unit==='current'?sampleCurrent:aura.unit||'player';$('#sample-aura-name').value=aura.aura;$('#sample-aura-kind').value=aura.condition;}
    const a=current().style.arc;
    if(a?.repeat==='stacks'&&a.aura){$('#sample-aura-unit').value=a.unit==='current'?sampleCurrent:a.unit;$('#sample-aura-name').value=a.aura;$('#sample-aura-kind').value=a.kind;}
    if(a?.fill==='resource')$('#sample-resource').value=a.resource==='primary'?'0':a.resource;
    const resource=M.flatten(current().roots).filter(n=>n.type==='if').flatMap(n=>M.predicateList(n.props)).find(p=>p.condition==='resource');
    if(resource){$('#sample-resource-controls').open=true;$('#sample-resource').value=resource.resource==='primary'?'0':resource.resource;}
    const health=M.flatten(current().roots).filter(n=>n.type==='if').flatMap(n=>M.predicateList(n.props)).find(p=>p.condition==='health');
    const sampleUnit=unit=>unit&&unit!=='current'?unit:sampleCurrent;
    $('#sample-health-unit').value=sampleUnit(health?.unit);$('#sample-resource-unit').value=sampleUnit(resource?.unit);
    if(a?.fill==='health')$('#sample-health-unit').value=sampleUnit(a.fillUnit);
    if(a?.fill==='resource')$('#sample-resource-unit').value=sampleUnit(a.fillUnit);

    if(aura)$('#sample-aura-controls').open=true;
  }
  function updateAuraControls() {
    const active=$('#sample-aura-state').value==='active';
    $('#sample-aura-stacks').disabled=!active;
    $('#sample-aura-permanent').disabled=!active;
    $('#sample-aura-remaining').disabled=!active||$('#sample-aura-permanent').checked;
  }

  function updateLive(){
    const c=live?.live?.character,st=live?.live?.status||'Waiting for WoW';
    $('#live-status').textContent=' · '+(c?c.character+' · ':'')+st+(live?.memoryError?' · '+live.memoryError:'')+(live?.historyError?' · '+live.historyError:'');
    const pet=c?.pet,known=st==='Connected'&&c?.petVersion===1&&typeof pet?.exists==='boolean';
    $('#live-pet').textContent='Main pet: '+(!known?'Unknown · reload WoW after updating':!pet.exists?'Missing':[pet.name||'Present',pet.alive===false?'Dead':pet.alive===true?'Alive':'Life state unknown',pet.family,pet.specialization,pet.creatureType].filter(Boolean).join(' · '));
    backgrounds?.refresh();loadPicker?.availability();renderCharacterChoice();if(!backgrounds?.uploading)backgrounds?.picker?.availability();
  }
  function loadField(key,label,values){const chosen=current().load?.[key]??'';return `<label>${label}${values?`<select data-load="${key}">${Object.entries({'':'Any',...values}).map(([v,l])=>option(v,l,chosen)).join('')}</select>`:`<input data-load="${key}" value="${esc(chosen)}" maxlength="120">`}</label>`;}
  function renderLoad(){
    const manual=loadPickerRule===active&&loadPicker?.editing();
    const specs={62:'Arcane',63:'Fire',64:'Frost Mage',65:'Holy Paladin',66:'Protection Paladin',70:'Retribution',71:'Arms',72:'Fury',73:'Protection Warrior',102:'Balance',103:'Feral',104:'Guardian',105:'Restoration Druid',250:'Blood',251:'Frost DK',252:'Unholy',253:'Beast Mastery',254:'Marksmanship',255:'Survival',256:'Discipline',257:'Holy Priest',258:'Shadow',259:'Assassination',260:'Outlaw',261:'Subtlety',262:'Elemental',263:'Enhancement',264:'Restoration Shaman',265:'Affliction',266:'Demonology',267:'Destruction',268:'Brewmaster',269:'Windwalker',270:'Mistweaver',577:'Havoc',581:'Vengeance',1480:'Devourer',1467:'Devastation',1468:'Preservation',1473:'Augmentation'};
    $('#load-panel').innerHTML=`<div id="load-identity"></div><div class="load-grid">${loadField('class','Class',Object.fromEntries(['Death Knight','Demon Hunter','Druid','Evoker','Hunter','Mage','Monk','Paladin','Priest','Rogue','Shaman','Warlock','Warrior'].map(n=>[n.replaceAll(' ','').toUpperCase(),n])))}${loadField('spec','Specialization',specs)}${loadField('race','Race', {Human:'Human',Dwarf:'Dwarf',NightElf:'Night Elf',Gnome:'Gnome',Draenei:'Draenei',Worgen:'Worgen',Orc:'Orc',Scourge:'Undead',Tauren:'Tauren',Troll:'Troll',BloodElf:'Blood Elf',Goblin:'Goblin',Pandaren:'Pandaren',Dracthyr:'Dracthyr',VoidElf:'Void Elf',LightforgedDraenei:'Lightforged Draenei',DarkIronDwarf:'Dark Iron Dwarf',KulTiran:'Kul Tiran',Mechagnome:'Mechagnome',Nightborne:'Nightborne',HighmountainTauren:'Highmountain Tauren',MagharOrc:'Mag’har Orc',ZandalariTroll:'Zandalari Troll',Vulpera:'Vulpera',EarthenDwarf:'Earthen'})}${loadField('faction','Faction',{Alliance:'Alliance',Horde:'Horde',Neutral:'Neutral'})}${loadField('role','Role',{tank:'Tank',healer:'Healer',damage:'Damage'})}${loadField('location','Location',{world:'Open world',dungeon:'Dungeon',raid:'Raid',arena:'Arena',pvp:'Battleground'})}</div><details><summary>More conditions</summary><div class="load-grid">${loadField('combat','In combat',{yes:'Yes',no:'No'})}${loadField('group','Group',{solo:'Solo',party:'Party',raid:'Raid'})}${loadField('mounted','Mounted',{yes:'Yes',no:'No'})}${loadField('alive','Alive',{yes:'Yes',no:'No'})}${loadField('minLevel','Minimum level')}${loadField('zone','Zone')}</div></details><div class="load-build-actions"><button id="load-build-edit" class="button">Build &amp; gear${current().load?.conditions?' · '+(M.predicateList(current().load.conditions).filter(p=>p.condition!=='group').length):''}</button>${current().load?.conditions?'<button id="load-build-clear" class="text-button">Clear</button>':''}</div>`;
    if(characters){loadPicker=characters.mount($('#load-identity'),'load',current().load||{},c=>mutate(()=>{current().load={...(current().load||{}),...c};}),true);loadPickerRule=active;if(manual)loadPicker.edit();}
  }
  $('#load-panel').addEventListener('click',e=>{
    if(e.target.closest('#load-build-clear')){loadEditor=null;mutate(()=>{delete current().load.conditions;});return;}
    if(e.target.closest('#load-build-edit')){loadEditor=active;modifierEditor=null;selected=null;const p=current().load?.conditions;if(p?.condition==='group'){renderAppearance();renderSelection();}else mutate(()=>{current().load||={};current().load.conditions={condition:'group',mode:'all',tests:p?[p]:[]};});$('#tab-style').click();}
  });
  $('#load-panel').addEventListener('change',e=>{if(!e.target.dataset.load)return;const key=e.target.dataset.load,value=e.target.value;mutate(()=>{current().load||={};current().load[key]=['spec','minLevel'].includes(key)?Number(value)||0:value.trim();});});

  function changeView(v){if(v!=='load')loadEditor=null;if(v==='load')characters?.load().then(()=>loadPicker?.refresh());view=v;document.body.dataset.view=v;$('#workspace').hidden=v!=='rules';$('#load-panel').hidden=v!=='load';$('#values-panel').hidden=v!=='values';valuesEditor.render();$$('button[data-view]').forEach(b=>b.setAttribute('aria-selected',String(b.dataset.view===v)));renderAppearance();requestAnimationFrame(()=>renderPreview());}
  $$('button[data-view]').forEach(b=>b.addEventListener('click',()=>changeView(b.dataset.view)));changeView('rules');
  let deleteTarget=null;
  $('#delete-rule').addEventListener('click',()=>{
    const r=current();if(!r)return;deleteTarget=r.id;$('#delete-name').textContent=r.name;$('#delete-dialog').showModal();
  });
  $('#delete-cancel').addEventListener('click',()=>$('#delete-dialog').close());
  $('#delete-dialog').addEventListener('close',()=>{deleteTarget=null;});
  $('#delete-confirm').addEventListener('click',()=>{
    const id=deleteTarget;$('#delete-dialog').close();if(!id)return;
    if(mutate(()=>{
      const index=rules.findIndex(r=>r.id===id);if(index<0)throw Error('Overlay no longer exists.');
      const group=characters.key(rules[index].load||{});M.Packs.detach(rules,rules[index]);rules.splice(index,1);
      active=(rules.find(r=>characters.key(r.load||{})===group)||rules[Math.min(index,rules.length-1)])?.id||null;selected=null;
    })){selectRuleSample();notify('Deleted. Undo to restore; Save changes to apply.');}
  });
  $('#empty-new').addEventListener('click',()=>$('#new-rule').click());
  $('#media-open').addEventListener('click',()=>media.open());
  document.addEventListener('click',e=>{
    const image=e.target.closest('[data-media-image]'),sound=e.target.closest('[data-media-sound]');if(!image&&!sound)return;
    const ruleID=current()?.id,nodeID=sound?.dataset.mediaSound;
    media.open(image?'image':'sound',asset=>{const r=rules.find(r=>r.id===ruleID);if(!r)return;mutate(()=>{if(image)(r.style.visual||={}).image=asset.id;else{const n=M.find(r.roots,nodeID)?.node;if(n)n.props.sound='media:'+asset.id;}});});
  });
  const sharing = window.OverlaySharing;
  let exportData, importReady = [], importAssets = [], importRead = 0, exportRead=0, importBusy=false;
  const packImport=new window.OverlayPackImport($('#pack-update-controls'),()=>{importRead++;updateImport();});
  async function updateExport() {
    const read=++exportRead;exportData=null;$('#export-code').value='';$('#export-copy').disabled=$('#export-download').disabled=true;$('#export-status').textContent='';
    try {
      const selectedRules=M.clone($('#export-scope').value==='all'?rules:$('#export-scope').value==='pack'?M.Packs.members(rules,current()):$('#export-scope').value==='group'?M.groupMembers(rules,current()):[current()]),includeCharacter=$('#export-character').checked;
      const assets=await media.exportAssets(selectedRules);if(read!==exportRead)return;
      exportData=sharing.bundle(selectedRules,includeCharacter,assets);$('#export-code').value=sharing.encode(exportData);$('#export-status').textContent='';$('#export-copy').disabled=$('#export-download').disabled=false;
    }catch(error){if(read!==exportRead)return;exportData=null;$('#export-status').textContent=error.message;}
  }
  $('#export-open').addEventListener('click',()=>{
    $('#export-scope').value = 'selected';$('#export-scope option[value="group"]').disabled=!current().style.group;
    $('#export-scope option[value="pack"]').disabled=!current().pack;
    $('#export-scope option[value="selected"]').textContent = current().name;
    $('#export-character').checked = false; updateExport(); $('#export-dialog').showModal();
  });
  $('#export-scope').addEventListener('change',updateExport);
  $('#export-character').addEventListener('change',updateExport);
  $('#export-close').addEventListener('click',()=>$('#export-dialog').close());
  $('#export-dialog').addEventListener('close',()=>{exportRead++;});
  $('#export-code').addEventListener('click',e=>e.currentTarget.select());
  $('#export-copy').addEventListener('click',async()=>{
    const code = $('#export-code');
    try { await navigator.clipboard.writeText(code.value); $('#export-status').textContent = 'Copied.'; }
    catch (_) { code.focus(); code.select(); $('#export-status').textContent = 'Press Ctrl+C to copy the selected code.'; }
  });
  $('#export-download').addEventListener('click',()=>{
    if (!exportData) return;
    const url = URL.createObjectURL(new Blob([JSON.stringify(exportData,null,2)],{type:'application/json'}));
    const a = document.createElement('a'); a.href = url;
    const name = exportData.rules.length===1?exportData.rules[0].name:'lamda-overlays';
    a.download = (name.toLowerCase().replace(/[^a-z0-9_-]+/g,'-').replace(/^-|-$/g,'').slice(0,60)||'lamda-overlay')+'.json';
    a.click(); setTimeout(()=>URL.revokeObjectURL(url),1000);
  });
  function updateImport() {
    importReady = []; importAssets=[]; $('#import-confirm').disabled = true; $('#import-status').textContent = '';
    $('#import-preview').hidden = true; $('#import-list').replaceChildren();$('#pack-update-notes').replaceChildren();$('#import-enabled-field').hidden=false;$('#import-confirm').textContent='Import';
    const text = $('#import-code').value;
    if (!text.trim()) {packImport.reset();return;}
    try {
      const bundle=sharing.decodeBundle(text),incoming=bundle.rules;importAssets=bundle.assets;
      $('#import-preview').hidden = false;
      const update=packImport.preview(rules,incoming);
      if(update){importReady=update.plan.rules;$('#import-enabled-field').hidden=true;$('#import-confirm').textContent='Update pack';$('#import-list').innerHTML=window.OverlayPackReview.rows(update.plan);$('#pack-update-notes').innerHTML=window.OverlayPackReview.notes(update.plan);$('#import-confirm').disabled=importBusy||!update.ready;return;}
      importReady = sharing.prepareImport(rules,incoming,$('#import-enabled').checked);
      $('#import-list').innerHTML = importReady.map(r=>`<li><span>${esc(r.name)}</span><small>${M.flatten(r.roots).length} blocks</small></li>`).join('');
      $('#import-confirm').disabled = importBusy;
    } catch(error) { $('#import-status').textContent = error.message;const updating=packImport.selection().mode==='update';$('#import-enabled-field').hidden=updating;$('#import-confirm').textContent=updating?'Update pack':'Import'; }
  }
  $('#import-open').addEventListener('click',()=>{
    importRead++;packImport.reset(); $('#import-code').value = ''; $('#import-file').value = '';
    $('#import-enabled').checked = false; updateImport(); $('#import-dialog').showModal();
  });
  $('#import-code').addEventListener('input',()=>{importRead++;packImport.reset(); updateImport();});
  $('#import-enabled').addEventListener('change',()=>{importRead++;updateImport();});
  $('#import-file-open').addEventListener('click',()=>$('#import-file').click());
  $('#import-file').addEventListener('change',async e=>{
    const file = e.target.files[0], read = ++importRead; e.target.value = ''; if (!file) return;
    importReady = []; importAssets=[];packImport.reset(); $('#import-confirm').disabled = true; $('#import-code').value = '';
    $('#import-preview').hidden = true; $('#import-status').textContent = 'Reading…';
    try {
      if (file.size > Math.ceil(sharing.maxBytes*4/3)+1024) throw Error('Keep overlay files under 16 MB.');
      const text = await file.text();
      if (read !== importRead || !$('#import-dialog').open) return;
      $('#import-code').value = text;packImport.reset(); updateImport();
    } catch(error) { if (read===importRead) $('#import-status').textContent = error.message; }
  });
  $('#import-cancel').addEventListener('click',()=>$('#import-dialog').close());
  $('#import-dialog').addEventListener('close',()=>{importRead++; importReady=[];importAssets=[];});
  $('#import-confirm').addEventListener('click',async()=>{
    if(importBusy)return;updateImport();if(!importReady.length||$('#import-confirm').disabled)return;
    const read=importRead,assets=importAssets,source=sharing.decodeBundle($('#import-code').value).rules,enabled=$('#import-enabled').checked,selection=packImport.selection();
    importBusy=true;$('#import-confirm').disabled=true;$('#import-status').textContent=assets.length?'Importing media…':'';
    try{
      await media.importAssets(assets);if(read!==importRead||!$('#import-dialog').open)return;
      if(selection.mode==='update'){const plan=window.OverlayPackUpdates.plan(rules,source,selection.instance,selection.options);if(mutate(()=>{rules=plan.rules;pendingPackUpdates.add(plan.instance);active=plan.first;selected=null;})){$('#import-dialog').close();selectRuleSample();changeView('rules');notify('Updated locally. Save changes when ready.');}return;}
      const added=sharing.prepareImport(rules,source,enabled);
      if(mutate(()=>{rules.push(...added);active=added[0].id;selected=null;})){$('#import-dialog').close();selectRuleSample();changeView('rules');notify(`Imported ${added.length} overlay${added.length===1?'':'s'}. Save changes when ready.`);}
    }catch(error){if(read===importRead)$('#import-status').textContent=error.message;}
    finally{const message=$('#import-status').textContent;importBusy=false;if($('#import-dialog').open){updateImport();if(message)$('#import-status').textContent=message;}}
  });
  $('#rule-list').addEventListener('click',e=>{
    const group=e.target.closest('[data-group]');
    if(group){const key=group.dataset.group,closed=!collapsedGroups.has(key);if(closed)collapsedGroups.add(key);else collapsedGroups.delete(key);saveCollapsedGroups();group.setAttribute('aria-expanded',String(!closed));group.querySelector('.group-chevron').textContent=closed?'▸':'▾';document.getElementById(group.getAttribute('aria-controls')).hidden=closed;return;}
    const button=e.target.closest('[data-rule]');if(!button)return;active=button.dataset.rule;selected=null;resetSession();selectRuleSample();render();
  });
  $('#rule-character').addEventListener('change',e=>{
    const key=e.target.value;
    if(key==='more'){renderCharacterChoice();changeView('load');const pick=$('#load-pick');pick.value='manual';pick.dispatchEvent(new Event('change'));return;}
    const c=key==='shared'?{}:key==='current'?live?.live?.character:characters.list().find(c=>characters.key(c)===key);
    if(!c){renderCharacterChoice();return;}
    loadPickerRule=null;
    mutate(()=>assignCharacter(current(),c));
  });
  $('#palette').addEventListener('click',e=>{const item=e.target.closest('[data-type]');if(item)addBlock(item.dataset.type);});
  $('#block-search').addEventListener('input',renderPalette);
  $('#workspace').addEventListener('click',e=>{
    const head=e.target.closest('[data-select]');if(head)selectNode(head.dataset.select);
    const toggle=e.target.closest('[data-action-toggle]');if(toggle){const id=toggle.dataset.actionToggle;if(closedActions.has(id))closedActions.delete(id);else closedActions.add(id);render();return;}
    if(e.target.closest('.root-end'))addBlock('when',true);
  });
  $('#workspace').addEventListener('keydown',e=>{if((e.key==='Enter'||e.key===' ')&&e.target.matches('.block-head,.root-end')){e.preventDefault();if(e.target.matches('.root-end'))addBlock('when',true);else selectNode(e.target.dataset.select);}});
  $('#workspace').addEventListener('change',e=>{
    const input=e.target.closest('[data-prop]');if(!input)return;
    mutate(()=>{
      const n=M.find(current().roots,input.dataset.node).node;let value=input.value;
      if(input.type==='number'){value=Number(value);if(!input.value.trim()||!Number.isFinite(value))throw Error('Enter a valid number.');value=Math.max(+input.min,Math.min(+input.max,value));if(input.step==='1')value=Math.round(value);}
      if(changeValueCondition(n.props,input.dataset.prop,value))return;
      n.props[input.dataset.prop]=value;selected=n.id;
      if(input.dataset.prop==='provider')delete n.props.timelineSource;
      if(input.dataset.prop==='condition'){
        if(value!=='group'){delete n.props.tests;delete n.props.mode;}
        if(value==='group'){n.props={condition:'group',mode:'all',tests:[]};}
        else if(['pet','value','equipment','talent','learned','bossTimer','bossStage'].includes(value)){Object.assign(n.props,predicateDefaults(value));}
        else if(value==='encounter'){Object.assign(n.props,predicateDefaults('encounter'));}
        else if(value==='eventValue'){Object.assign(n.props,{field:'amount',operator:'gte',value:1000});}
        else if(value==='casting'){Object.assign(n.props,{unit:'player',spell:'',check:'active',operator:'lt',value:1});}
        else if(value==='cooldown'){Object.assign(n.props,{source:'spell',id:'363916',check:'ready',operator:'gte',value:1});}
        else if(['buff','debuff'].includes(value)){
          n.props.aura ||= '124273';
          n.props.check='active';n.props.operator='gte';n.props.value=3;
          $('#sample-aura-controls').open=true;
        }else if(value==='resource'){Object.assign(n.props,{resource:'primary',operator:'lt',measure:'percent',value:20});}
        else {n.props.value={role:'tank',location:'dungeon',health:35}[value];if(value==='health')n.props.operator='lt';}
      }
      if(n.props.condition==='equipment')equipmentChange(n.props,input.dataset.prop,value);
      if(n.props.condition==='eventValue'&&input.dataset.prop==='field'&&value==='critical'){n.props.operator='eq';n.props.value=1;}
      if(n.props.condition==='eventValue'&&input.dataset.prop==='value')n.props.value=+value;
      if(input.dataset.prop==='event'&&Object.hasOwn(M.bossEvents,value)){Object.assign(n.props,{provider:'auto',spell:'',textFilter:'',timerKind:'any',timelineSource:'any',encounterID:''});}
      if(input.dataset.prop==='event'&&Object.hasOwn(M.encounterEvents,value)){n.props.encounterID||='';n.props.difficulty||='';n.props.outcome='';}
      if(input.dataset.prop==='event'&&value==='combatlog'){Object.assign(n.props,{combatEvent:'damage',sourceUnit:'player',targetUnit:'any',spell:'',extraSpell:'',school:'any',critical:'any',periodic:'any',auraKind:'any',miss:'any'});}
      if(input.dataset.prop==='combatEvent'){for(const k of ['extraSpell','school','critical','periodic','auraKind','miss'])delete n.props[k];}
      if(input.dataset.prop==='mode'&&n.props.condition==='group')setPredicateMode(n.props,value);
      if(input.dataset.prop==='source'&&n.props.condition==='cooldown'){n.props.id=value==='gcd'?'61304':value==='slot'?'13':value==='item'?'5512':'363916';n.props.check='ready';}
      if(input.dataset.prop==='id')n.props.id=value.trim();
      if(input.dataset.prop==='check'&&n.props.condition==='cooldown'){n.props.operator=value==='charges'?'gte':'lt';n.props.value=value==='charges'?1:5;}
      if(input.dataset.prop==='measure'&&value==='percent')n.props.value=Math.min(100,n.props.value);
      if(input.dataset.prop==='check'&&['stacks','remaining','count'].includes(value)){n.props.operator=value==='stacks'?'gte':'lt';n.props.value=value==='stacks'?3:5;}
      if(input.dataset.prop==='aura'){n.props.aura=value.trim();if(!n.props.aura)throw Error("Enter the aura’s spell ID.");}
      if(input.dataset.prop==='event')$('#sample-event').value=value;
    });
    if(input.dataset.prop==='scope'){selectRuleSample();updateSample();}
    if(groupSelected())$('#tab-style').click();
  });
  $('#selection-panel').addEventListener('change',e=>{const el=e.target.closest('[data-prop]');if(el)mutate(()=>{const p=M.find(current().roots,el.dataset.node).node.props;p[el.dataset.prop]=el.value.trim();if(p.condition==='equipment')equipmentChange(p,el.dataset.prop,el.value);});});
  $('#selection-panel').addEventListener('click',e=>{const b=e.target.closest('[data-edit]');if(b)editSelected(b.dataset.edit);});
  document.addEventListener('change',e=>{
    const select=e.target.closest('[data-equipped]');if(!select||!select.value)return;
    const slot=live?.live?.character?.equipment?.slots?.find(s=>s.slot===+select.value);if(!slot)return;
    const path=select.closest('[data-test-path]')?.dataset.testPath;
    mutate(()=>{const p=path===undefined?M.find(current().roots,select.dataset.equipped)?.node.props:predicateAt(path);if(p?.condition==='equipment'){const id=p.source==='set'?slot.set:slot.item;if(id>0)p.id=String(id);}});
  });
  document.addEventListener('dragstart',e=>{
    if(e.target.closest('input,select')){e.preventDefault();return;}
    const source=e.target.closest('[data-drag],[data-type]');if(!source)return;
    drag=source.dataset.drag?{id:source.dataset.drag}:{type:source.dataset.type};
    e.dataTransfer.setData('application/x-lamda-block',JSON.stringify(drag));e.dataTransfer.setData('text/plain','Lamda block');e.dataTransfer.effectAllowed=drag.id?'move':'copy';
    const started=drag,block=drag.id?document.getElementById(drag.id):null;
    if(block){selectNode(drag.id);e.dataTransfer.setDragImage(block,25,20);}
    // Let the browser start the drag before expanding slots moves its source.
    setTimeout(()=>{if(drag!==started)return;document.body.classList.add('dragging');block?.classList.add('drag-source');},0);
  });
  function clearDrag(){document.body.classList.remove('dragging');$$('.over,.drag-source').forEach(el=>el.classList.remove('over','drag-source'));drag=null;}
  document.addEventListener('dragend',clearDrag);
  $('#workspace').addEventListener('dragover',e=>{const target=e.target.closest('.drop-slot');if(!target||!drag)return;e.preventDefault();e.dataTransfer.dropEffect=drag.id?'move':'copy';$$('.drop-slot.over').forEach(el=>{if(el!==target)el.classList.remove('over');});target.classList.add('over');});
  $('#workspace').addEventListener('dragleave',e=>{const target=e.target.closest('.drop-slot');if(target&&!target.contains(e.relatedTarget))target.classList.remove('over');});
  document.addEventListener('change',e=>{if(e.target.dataset.prop==='display'&&archSelected())$('#tab-style').click();});
  $('#workspace').addEventListener('drop',e=>{const target=e.target.closest('.drop-slot');if(!target||!drag)return;e.preventDefault();const spec=drag;const parent=target.dataset.parent||null,index=+target.dataset.index;clearDrag();mutate(()=>{selected=M.insert(current(),spec,parent,index);});});
  $('#save').addEventListener('click',save);$('#undo').addEventListener('click',undo);$('#redo').addEventListener('click',redo);
  $('#rule-name').addEventListener('change',e=>{const name=e.target.value.trim();if(!name){e.target.value=current().name;return;}mutate(()=>{current().name=name;});});
  $('#enabled').addEventListener('change',e=>{const enabled=e.target.checked;mutate(()=>{current().enabled=enabled;});});
  $('#new-rule').addEventListener('click',()=>mutate(()=>{if(rules.length>=M.limits.rules)throw Error(`This release supports up to ${M.limits.rules} overlays.`);const r=inheritCharacter(M.newRule());rules.push(r);active=r.id;selected=r.roots[0].id;$('#sample-event').value='state';}));
  $('#example-menu').addEventListener('change',e=>{
    const template=e.target.value;e.target.value='';if(!template)return;
    if(mutate(()=>{if(rules.length>=M.limits.rules)throw Error(`This release supports up to ${M.limits.rules} overlays.`);const r=inheritCharacter(template==='reminder'?draft():template==='nested'?M.nestedExample():template==='mana'?M.manaExample():template==='cap'?M.capExample():template==='clones'?M.clonesExample():template==='arches'?M.arcExample():template==='combatlog'?M.combatExample():template==='encounter'?M.encounterExample():template==='boss'?M.bossExample():template==='cast'?M.castExample():template==='cooldown'?M.cooldownExample():template==='logic'?M.logicExample():['resourcebar','bufftimer'].includes(template)?M.liveExample(template==='bufftimer'?'buff':'resource'):M.auraExample());rules.push(r);active=r.id;selected=r.roots[0].id;})){
      selectRuleSample();$('#sample-aura-state').value='active';$('#sample-aura-stacks').value=3;$('#sample-aura-remaining').value=4;$('#sample-aura-permanent').checked=false;updateAuraControls();$('#tab-test').click();
      if(template==='mana'){$('#sample-resource').value='0';$('#sample-resource-current').value=15;$('#sample-resource-max').value=100;}
      if(template==='cap'){$('#sample-resource-current').value=100;$('#sample-resource-max').value=100;}
      if(template==='nested')$('#sample-location').value='dungeon';
    }
  });
  $('#duplicate').addEventListener('click',()=>mutate(()=>{if(rules.length>=M.limits.rules)throw Error(`This release supports up to ${M.limits.rules} overlays.`);const r=M.clone(current());r.id=M.id();r.name=(r.name+' copy').slice(0,60);r.roots=M.copyRoots(r.roots);M.Packs.detach([],r);rules.push(r);active=r.id;selected=null;}));
  $('#test-event').addEventListener('click',runTest);$('#clear-preview').addEventListener('click',resetSession);
  $('#sample-health').addEventListener('input',e=>{$('#health-value').textContent=e.target.value+'%';$('.unit-frame.tank .health-track i').style.width=e.target.value+'%';$('.unit-frame.tank .health-number').textContent=e.target.value+'%';updateSample();});
  $('#sample-equipment-slot').innerHTML=Object.entries(M.equipmentSlots).map(([k,v])=>option(k,v,13)).join('');
  for(const input of $$('#sample-equipment-controls input,#sample-equipment-controls select'))input.addEventListener('change',updateSample);
  for(const input of $$('#sample-talent-controls input,#sample-talent-controls select'))input.addEventListener('change',updateSample);
  for(const input of $$('#sample-boss-controls input,#sample-boss-controls select'))input.addEventListener('change',updateSample);
  for(const input of $$('#sample-encounter-controls input,#sample-encounter-controls select'))input.addEventListener('change',updateSample);
  for(const input of $$('#sample-pet-controls input,#sample-pet-controls select'))input.addEventListener('change',updateSample);
  for(const input of $$('#sample-log-controls input,#sample-log-controls select'))input.addEventListener('change',updateSample);
  for(const input of $$('#sample-cast-controls input,#sample-cast-controls select'))input.addEventListener('change',updateSample);
  for(const input of $$('#sample-cooldown-controls input,#sample-cooldown-controls select'))input.addEventListener('change',updateSample);
  for(const key of ['sample-health-unit','sample-resource-unit','sample-resource','sample-resource-current','sample-resource-max'])$('#'+key).addEventListener('change',updateSample);
  for(const id of ['sample-log-source','sample-log-target','sample-cast-unit','sample-aura-unit','sample-health-unit','sample-resource-unit','arch-unit','arch-fill-unit'])$('#'+id).innerHTML=Object.entries(M.unitChoices).filter(([key])=>!id.startsWith('sample-')||key!=='current').map(([key,label])=>option(key,label,'player')).join('');
  $('#sample-log-target').value='target';
  $('#sample-log-kind').innerHTML=Object.entries(M.combatKinds).map(([k,v])=>option(k,v,'interrupt')).join('');
  $('#sample-log-field').innerHTML=Object.entries(M.combatValues).filter(([k])=>k!=='critical').map(([k,v])=>option(k,v,'amount')).join('');
  $('#sample-resource').innerHTML=Object.entries(M.resources).filter(([key])=>key!=='primary').map(([key,label])=>option(key,label,'0')).join('');
  $('#sample-event').innerHTML=Object.entries(eventNames).map(([k,v])=>option(k,v,'state')).join('');
  for(const key of ['sample-event','sample-role','sample-location'])$('#'+key).addEventListener('change',updateSample);
  $('#sample-aura-controls').addEventListener('change',()=>{updateAuraControls();updateSample();});
  $$('.settings-tabs button').forEach(button=>button.addEventListener('click',()=>{
    $$('.settings-tabs button').forEach(b=>b.setAttribute('aria-selected',String(b===button)));
    $('#test-panel').hidden=button.dataset.tab!=='test';$('#style-panel').hidden=button.dataset.tab!=='style';
  }));
  for(const [input,prop] of [['appearance-size','size'],['appearance-opacity','opacity'],['appearance-color','color'],['appearance-font','font'],['appearance-transparent','transparent']]){
    $('#'+input).addEventListener('change',e=>{const value=e.target.type==='checkbox'?e.target.checked:['size','opacity','font'].includes(prop)?+e.target.value:e.target.value;mutate(()=>{current().style[prop==='size'&&textDisplay()?'font':prop]=prop==='color'&&value==='custom'?M.colorValue(current().style.color):value;});});
    if(['size','opacity'].includes(prop))$('#'+input).addEventListener('input',e=>{const value=+e.target.value;$('#'+(prop==='size'?'size':'opacity')+'-value').textContent=value+(prop==='size'?' px':'%');renderPreview({...current().style,[prop==='size'&&textDisplay()?'font':prop]:value});});
  }
  $('#appearance-custom-color').addEventListener('change',e=>mutate(()=>{current().style.color=e.target.value;}));
  document.addEventListener('click',e=>{
    const button=e.target.closest('[data-current-hero]');if(!button)return;
    const id=live?.live?.character?.heroSpec;if(!id)return;
    const path=button.closest('[data-test-path]')?.dataset.testPath;
    mutate(()=>{const p=path===undefined?M.find(current().roots,button.dataset.currentHero)?.node.props:predicateAt(path);if(p?.condition==='talent'&&p.source==='hero')p.id=String(id);});
  });
  const spellPicker=new window.OverlaySpellPicker();
  document.addEventListener('click',e=>{
    const button=e.target.closest('.spell-find');if(!button||!current())return;
    const ruleID=current().id,nodeID=button.dataset.spellNode,target=button.dataset.spellTarget;
    const node=nodeID?M.find(current().roots,nodeID)?.node:null;
    const id=(target==='boss'?node?.props.spell:target==='sample-boss-id'?$('#sample-boss-id').value:target?.startsWith('combat-')?node?.props[target.slice(7)]:target?.startsWith('sample-log-')?$('#'+target).value:target==='sample-cast'?$('#sample-cast-id').value:target==='cast'?node?.props.spell:target==='icon'?node?.props.icon:target==='data'?node?.props.dataID:null)||(['cooldown','talent','learned'].includes(node?.props.condition)?node.props.id:node?.props.aura)||(target==='arch'?current().style.arc?.aura:$('#sample-aura-name').value);
    const classID=current().load?.class||live?.live?.character?.class||'';
    const prefer=['WARRIOR','PALADIN','HUNTER','ROGUE','PRIEST','DEATHKNIGHT','SHAMAN','MAGE','WARLOCK','MONK','DRUID','DEMONHUNTER','EVOKER'].includes(classID)?classID:'';
    spellPicker.open({id,prefer,kind:target==='boss'||target==='sample-boss-id'||target?.startsWith('combat-')||target?.startsWith('sample-log-')||target==='cast'||target==='sample-cast'||target==='icon'||target==='data'&&['spell','bossTimer','talent','learned'].includes(node?.props.dataSource)||['cooldown','talent','learned'].includes(node?.props.condition)?'all':'auras',focus:()=>nodeID?document.querySelector(`[data-spell-node="${nodeID}"]`):document.querySelector(`[data-spell-target="${target}"]`),choose:spell=>{
      if(current()?.id!==ruleID)return false;
      if(target==='sample-boss-id'){$('#sample-boss-id').value=spell.id;$('#sample-boss-name').value=spell.name;spellNames[spell.id]=spell.name;updateSample();return true;}
      if(target?.startsWith('sample-log-')){$('#'+target).value=spell.id;spellNames[spell.id]=spell.name;updateSample();return true;}
      if(target==='sample-cast'){$('#sample-cast-id').value=spell.id;updateSample();return true;}
      if(target==='sample'){$('#sample-aura-name').value=spell.id;updateSample();return true;}
      const ok=mutate(()=>{
        if(target==='arch')current().style.arc.aura=spell.id;
        else if(target==='icon'||target==='data'){const n=M.find(current().roots,nodeID)?.node;if(!n)throw Error('Choose the spell field again.');n.props[target==='icon'?'icon':'dataID']=String(spell.id);spellNames[spell.id]=spell.name;selected=n.id;}
        else{const n=M.find(current().roots,nodeID)?.node;if(!n)throw Error('Choose the spell field again.');n.props[target?.startsWith('combat-')?target.slice(7):['cast','boss'].includes(target)?'spell':['cooldown','talent','learned'].includes(n.props.condition)?'id':'aura']=String(spell.id);selected=n.id;}
      });
      if(ok){selectRuleSample();$('#sample-aura-name').value=spell.id;renderPreview();}
      return ok;
    }});
  });
  $('#arch-resource').innerHTML=Object.entries(M.resources).map(([key,label])=>option(key,label,'primary')).join('');
  for(const el of $$('[data-arc]'))el.addEventListener('change',e=>{
    const field=el.dataset.arc,value=el.type==='checkbox'?el.checked:el.type==='number'?+el.value:el.value;
    if(el.type==='number'&&!el.checkValidity()){el.reportValidity();return;}
    if(field==='fill'&&['source','charges','recharge'].includes(value)){mutate(()=>{previewNode().props.fill=value;});return;}
    if(mutate(()=>{
      if(field==='fill')delete previewNode().props.fill;
      const r=current(),a=r.style.arc||(r.style.arc=M.arcDefaults(r.style,previewNode()?.props.display));a[field]=value;
      if(field==='repeat'&&value==='stacks'){
        a.layout='around';a.start=0;a.sweep=a.limit<=2?160:70;a.fill='full';
        const n=M.flatten(r.roots).find(n=>n.type==='if'&&['buff','debuff'].includes(n.props.condition));
        if(n){a.aura=+n.props.aura;a.kind=n.props.condition;a.unit=n.props.unit||'player';}
      }
      if(field==='limit'&&a.layout==='around'){a.sweep=a.limit<=2?160:70;if(a.limit>2)a.pair='sides';}
      if(field==='layout'&&value==='around'){a.start=0;a.sweep=a.limit<=2?160:70;}
    })){selectRuleSample();renderPreview();}
  });
  $('#reset-position').addEventListener('click',()=>mutate(()=>movePreview(48,40,M.clone(rules))));
  let pointer;
  $('#overlay-preview').addEventListener('pointerdown',e=>{if(e.button!==0)return;const b=$('#overlay-preview').getBoundingClientRect();pointer={before:snapshot(),x:e.clientX,y:e.clientY,left:b.left,top:b.top,moved:false};e.currentTarget.focus({preventScroll:true});e.currentTarget.setPointerCapture(e.pointerId);e.preventDefault();});
  $('#overlay-preview').addEventListener('pointermove',e=>{if(!pointer)return;const scene=$('#scene').getBoundingClientRect(),bounds=$('#overlay-preview').getBoundingClientRect();const dx=e.clientX-pointer.x,dy=e.clientY-pointer.y;if(Math.abs(dx)+Math.abs(dy)>3)pointer.moved=true;if(!pointer.moved)return;movePreview(100*Math.max(0,Math.min(scene.width-bounds.width,pointer.left-scene.left+dx))/scene.width,100*Math.max(0,Math.min(scene.height-bounds.height,pointer.top-scene.top+dy))/scene.height,pointer.before.rules);renderPreview();});
  $('#overlay-preview').addEventListener('keydown',e=>{
    const direction={ArrowLeft:[-1,0],ArrowRight:[1,0],ArrowUp:[0,-1],ArrowDown:[0,1]}[e.key];if(!direction)return;e.preventDefault();
    mutate(()=>{const s=current().style,el=$('#overlay-preview'),width=backgrounds?.width||1920,height=backgrounds?.height||1080,p=previewBounds||M.previewPosition(s,{width:el.offsetWidth,height:el.offsetHeight},width,height),step=e.shiftKey?10:1;movePreview(100*Math.max(0,Math.min(width-el.offsetWidth,p.x+direction[0]*step))/width,100*Math.max(0,Math.min(height-el.offsetHeight,p.y+direction[1]*step))/height,M.clone(rules));});
  });
  function stopPointer(){if(pointer?.moved){history.push(pointer.before);future=[];saved=false;status();}pointer=null;}
  $('#overlay-preview').addEventListener('pointerup',stopPointer);$('#overlay-preview').addEventListener('pointercancel',stopPointer);
  $('#help').addEventListener('click',()=>{$('#capacity-usage').innerHTML='<table><tbody>'+M.usage(rules).map(([name,used,max])=>`<tr><th>${esc(name)}</th><td>${used} / ${max}</td></tr>`).join('')+'</tbody></table>';$('#help-dialog').showModal();});$('#close-help').addEventListener('click',()=>$('#help-dialog').close());
  $('#reset').addEventListener('click',()=>$('#reset-dialog').showModal());$('#cancel-reset').addEventListener('click',()=>$('#reset-dialog').close());$('#confirm-reset').addEventListener('click',()=>{$('#reset-dialog').close();mutate(()=>{rules=M.examples();active=rules[0].id;selected=null;$('#sample-event').value='state';});});
  document.addEventListener('keydown',e=>{
    if ($('dialog[open]')) { if ((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='s') e.preventDefault(); return; }
    const typing=e.target.matches('input,textarea,select');
    if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='s'){e.preventDefault();save();}
    if(!typing&&(e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='z'){e.preventDefault();e.shiftKey?redo():undo();}
    if(!typing&&e.key==='Delete'&&selected&&!$('dialog[open]')){e.preventDefault();editSelected('delete');}
  });
  new ResizeObserver(()=>clampPreview()).observe($('#scene'));
  window.addEventListener('beforeunload',e=>{if(!saved){e.preventDefault();e.returnValue='';}});
  let lastPreviewTick=0;
  setInterval(()=>{if(!running||pointer)return;const now=Date.now(),animated=retiredDisplays.size>0||[...displays.values()].some(d=>{if(d.end<=now)return false;const v=rules.find(r=>r.id===d.ruleID)?.style.visual;return !!v?.sprite||M.Motion.active(v?.motion,(now-d.started)/1000)});if(now-lastPreviewTick>=(animated?30:150)){lastPreviewTick=now;renderPreview();}},33);
  characters=new window.OverlayCharacters(()=>({character:live?.live?.character,images:backgrounds?.library.images||[],rules}),notify);
  await characters.load();
  backgrounds=new window.OverlayBackgrounds(()=>({load:current()?.load||{},character:live?.live?.character}),()=>renderPreview(),notify,characters);
  try{await backgrounds.load();}catch(error){notify(error.message);}
  render(); selectRuleSample(); updateAuraControls();updateLive();
  setInterval(async()=>{try {live=await api();if(live.media&&JSON.stringify(live.media)!==JSON.stringify(media.library)){media.library=live.media;render();}updateLive();if(current()?.style.anchor&&$('#sample-anchor-source').value==='live'&&!pointer)renderPreview();}catch(_){$('#live-status').textContent=' · Engine unavailable';}},2000);
})();
