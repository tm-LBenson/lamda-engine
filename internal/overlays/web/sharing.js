/* Portable rules and explicitly referenced media. Never character screenshots or code. */
(function (root) {
  'use strict';
  const M = root.OverlayModel || require('./model.js');
  const prefix = 'LAMDAOVERLAYS:', maxBytes = 16 * 1024 * 1024;
  const loadKeys = ['character','realm','class','spec','race','faction','role','location','combat','group','mounted','alive','minLevel','zone'];
  const props = {
    action:['name'],call:['action'],
    when:['provider','timelineSource','textFilter','timerKind','encounterID','difficulty','outcome','event','unit','spell','combatEvent','sourceUnit','targetUnit','extraSpell','school','critical','periodic','auraKind','miss'], each:['scope'], if:['key','other','compareTo','slot','name','itemClass','itemSubclass','provider','timelineSource','textFilter','timerKind','encounterID','difficulty','condition','value','operator','resource','measure','aura','unit','check','source','id','mode','tests','spell','field'],
    show:['dataValue','dataMaxValue','dataMaximum','display','text','duration','visibility','dataSource','dataID','dataUnit','dataResource','dataProvider','dataText','dataKind','dataTimelineSource','dataTalentKind','icon','fill'], glow:['color'], sound:['sound'], wait:['seconds'], hide:[]
  };
  function pick(value, keys) { return Object.fromEntries(keys.filter(k => Object.hasOwn(value,k)).map(k => [k,value[k]])); }
  function broaderClones(rules){return M.anchorCount(rules)>40||M.cooldownCount(rules)+2*M.castCount(rules)>64||rules.some(r=>M.flatten(r.roots).some(n=>n.type==='each'&&n.props.scope&&n.props.scope!=='party'));}
  function expandedCapture(rules){return M.watchCount(rules)>16||M.cooldownCount(rules)+2*M.castCount(rules)>16||M.measurementCount(rules)>16||M.anchorCount(rules)>8;}
  function bundle(rules, includeCharacter = false, assets = []) {
    rules=M.Packs.detachPartial(rules);
    if (!M.validRules(rules)) throw Error('These overlays cannot be exported. Check their settings.');
    function predicate(p){const out=pick(p,props.if);if(p.condition==='group')out.tests=p.tests.map(predicate);else delete out.tests;return out;}
    function block(n) {
      const out = {id:n.id,type:n.type,props:n.type==='if'?predicate(n.props):pick(n.props,props[n.type])};
      if (n.children) out.children = n.children.map(block);
      return out;
    }
    assets=validateAssets(rules,assets);
    return {version:broaderClones(rules)?12:expandedCapture(rules)?11:rules.length>40?10:rules.some(r=>r.pack)?9:rules.some(r=>r.logic?.values?.some(v=>v.remember?.reset==='saved'))?8:rules.some(r=>M.flatten(r.roots).some(n=>['action','call'].includes(n.type)))?7:rules.some(r=>r.logic?.values?.some(v=>v.remember))?6:rules.some(r=>r.logic)?5:rules.some(r=>r.style.visual?.motion)?4:rules.some(r=>r.style.visual&&['sprite','fontFamily','bold','italic','textEffect'].some(k=>Object.hasOwn(r.style.visual,k)))?3:assets.length||rules.some(r=>r.style.visual)?2:1,...(assets.length?{assets}:{}),rules:rules.map(r => ({
      id:r.id,name:r.name,enabled:r.enabled,...(r.pack?{pack:pick(r.pack,M.Packs.keys)}:{}),roots:r.roots.map(block),...(r.logic?{logic:M.clone(r.logic)}:{}),
      load:{...pick(r.load || {},loadKeys.filter(k => includeCharacter || !['character','realm'].includes(k))),...(r.load?.conditions?{conditions:predicate(r.load.conditions)}:{})},
      style:{...pick(r.style,['size','opacity','color','font','transparent','x','y','onShow','onHide']),...(r.style.visual?{visual:{...pick(r.style.visual,M.visualKeys),...(r.style.visual.motion?{motion:M.Motion.portable(r.style.visual.motion)}:{}),...(r.style.visual.sprite?{sprite:pick(r.style.visual.sprite,M.spriteKeys)}:{})}}:{}),...(r.style.anchor?{anchor:pick(r.style.anchor,M.anchorKeys)}:{}),...(r.style.group?{group:pick(r.style.group,M.groupKeys)}:{}),...(r.style.modifiers?{modifiers:r.style.modifiers.map(m=>({...pick(m,['color','size','font','opacity','text','glow','visible','sound']),condition:predicate(m.condition)}))}:{}),...(r.style.arc?{arc:pick(r.style.arc,['start','sweep','thickness','gap','reverse','fill','fillUnit','resource','repeat','layout','pair','aura','kind','unit','limit'])}:{})}
    }))};
  }
  function encode(data) {
    const bytes = new TextEncoder().encode(JSON.stringify(data));
    if (bytes.length > maxBytes) throw Error('Keep overlay files under 16 MB.');
    let binary = '';
    for (let offset=0;offset<bytes.length;offset+=8192) binary += String.fromCharCode(...bytes.subarray(offset,offset+8192));
    return prefix + data.version + ':' + btoa(binary);
  }
  function decodeBundle(text) {
    if (typeof text !== 'string' || text.length > Math.ceil(maxBytes*4/3)+1024) throw Error('Keep overlay files under 16 MB.');
    text = text.trim();
    if (!text) throw Error('Paste a share code or choose a file.');
    const header=text.match(/^LAMDAOVERLAYS:([1-9]|10|11|12):/);
    if (text.startsWith(prefix) && !header) throw Error('Unsupported share version. Update Lamda Engine first.');
    const codeVersion=text.startsWith(prefix)?Number(header[1]):null;
    try {
      if (text.startsWith(prefix)) {
        const binary = atob(text.slice(header[0].length).replace(/\s/g,''));
        if (binary.length > maxBytes) throw Error();
        text = new TextDecoder('utf-8',{fatal:true}).decode(Uint8Array.from(binary,c => c.charCodeAt(0)));
      }
    } catch (_) { throw Error('Invalid share code. Copy the entire code and try again.'); }
    if (new TextEncoder().encode(text).length > maxBytes) throw Error('Keep overlay files under 16 MB.');
    let data;
    try { data = JSON.parse(text); } catch (_) { throw Error('Invalid overlay data. Paste a share code or choose an exported JSON file.'); }
    if (data && data.version !== undefined && ![1,2,3,4,5,6,7,8,9,10,11,12].includes(data.version)) throw Error('Unsupported share version. Update Lamda Engine first.');
    if (!data || codeVersion!==null&&codeVersion!==data.version || ![1,2,3,4,5,6,7,8,9,10,11,12].includes(data.version) || !M.validRules(data.rules)) throw Error('Invalid overlay data or unsupported settings.');
    // Retain explicit filters in older exports, and discard unrelated metadata.
    if(data.version<12&&broaderClones(data.rules))throw Error('Broader clones require share version 12.');
    if(data.version<11&&expandedCapture(data.rules))throw Error('Expanded tracking requires share version 11.');
    if(data.version<10&&data.rules.length>40)throw Error('More than 40 overlays requires share version 10.');
    if(data.version===1&&data.assets?.length)throw Error('Media requires share version 2.');
    if(data.version<9&&data.rules.some(r=>r.pack))throw Error('Packs require share version 9.');
    if(data.version<7&&data.rules.some(r=>M.flatten(r.roots).some(n=>['action','call'].includes(n.type))))throw Error('Actions require share version 7.');
    if(data.version<8&&data.rules.some(r=>r.logic?.values?.some(v=>v.remember?.reset==='saved')))throw Error('Saved history setup requires share version 8.');
    if(data.version<6&&data.rules.some(r=>r.logic?.values?.some(v=>v.remember)))throw Error('Remembered values require share version 6.');
    if(data.version<5&&data.rules.some(r=>r.logic))throw Error('Values require share version 5.');
    const assets=validateAssets(data.rules,data.assets||[]);return {rules:bundle(data.rules,true,assets).rules,assets};
  }
  function decode(text){return decodeBundle(text).rules;}
  function validateAssets(rules,assets){
    if(!Array.isArray(assets)||assets.length>64)throw Error('Invalid media library.');
    const refs=new Set(M.mediaReferences(rules)),ids=new Set();let bytes=0;
    const out=assets.map(a=>{
      if(!a||typeof a!=='object'||!refs.has(a.id)||ids.has(a.id)||typeof a.name!=='string'||!a.name.trim()||[...a.name].length>80||/[\x00-\x1f\x7f]/.test(a.name)||typeof a.data!=='string'||!a.data.length||a.data.length%4!==0||!/^[A-Za-z0-9+/]*={0,2}$/.test(a.data)||a.data.length>Math.ceil(8*1024*1024/3)*4)throw Error('Invalid shared media.');
      ids.add(a.id);bytes+=a.data.length;if(bytes>maxBytes)throw Error('Keep shared media under 16 MB.');return {id:a.id,name:a.name,data:a.data};
    });
    if([...refs].some(id=>!ids.has(id)))throw Error('This overlay is missing media. Add the file before sharing.');
    return out;
  }
  function prepareImport(existing, incoming, enabled = false) {
    if (!M.validRules(incoming)) throw Error('Invalid overlay data or unsupported settings.');
    if (existing.length + incoming.length > M.limits.rules) throw Error(`There is room for ${Math.max(0,M.limits.rules-existing.length)} more overlays (${M.limits.rules} maximum).`);
    const ids = new Set(existing.flatMap(r => [r.id,...(r.pack?[r.pack.instance]:[]),...(r.style.group?[r.style.group.id]:[]),...M.flatten(r.roots).map(n => n.id)]));
    const names = new Set(existing.map(r => r.name.toLowerCase()));
    const fresh = () => { let key; do { key = M.id(); } while(ids.has(key)); ids.add(key); return key; };
    const groupIDs=new Map(),packIDs=new Map(),packNames=new Map();
    const added = incoming.map(source => {
      const rule = M.clone(source); rule.id = fresh(); rule.enabled = enabled;
      rule.roots=M.Actions.copies(rule.roots,fresh);
      if(rule.pack){const oldNodes=M.flatten(source.roots),newNodes=M.flatten(rule.roots),mapping=new Map(oldNodes.map((n,i)=>[n.id,newNodes[i].id]));rule.pack.nodes=Object.fromEntries(Object.entries(source.pack.nodes).map(([original,local])=>[original,mapping.get(local)]));}
      if(rule.pack){const old=rule.pack.instance;if(!packIDs.has(old)){packIDs.set(old,fresh());let name=rule.pack.name,n=2;const names=new Set([...existing.map(r=>r.pack?.name),...packNames.values()]);while(names.has(name)){const suffix=' ('+(n++)+')';name=rule.pack.name.slice(0,60-suffix.length)+suffix;}packNames.set(old,name);}rule.pack.instance=packIDs.get(old);rule.pack.name=packNames.get(old);}
      if(rule.style.group){const old=rule.style.group.id;if(!groupIDs.has(old))groupIDs.set(old,fresh());rule.style.group.id=groupIDs.get(old);}
      let name = rule.name, i = 2;
      while (names.has(name.toLowerCase())) { const suffix = ` (${i++})`; name = rule.name.slice(0,60-suffix.length)+suffix; }
      rule.name = name; names.add(name.toLowerCase()); return rule;
    });
    const combined = [...existing,...added];
    if (M.watchCount(combined) > M.limits.auras) throw Error(`Up to ${M.limits.auras} auras can be watched. Import with Enable after import unchecked.`);
    if (!M.validRules(combined)) throw Error('These overlays exceed the supported limits.');
    return added;
  }
  const api = {maxBytes,bundle,encode,decode,decodeBundle,prepareImport};
  root.OverlaySharing = api;
  if (typeof module !== 'undefined') module.exports = api;
})(typeof window !== 'undefined' ? window : globalThis);
