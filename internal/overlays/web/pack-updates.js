(function(root){
 'use strict';
 const M=root.OverlayModel||(typeof require==='function'?require('./model.js'):null),V=M.Values;
 const clone=M.clone,own=(o,k)=>Object.hasOwn(o||{},k);
 const stable=v=>Array.isArray(v)?v.map(stable):v&&typeof v==='object'?Object.fromEntries(Object.keys(v).sort().map(k=>[k,stable(v[k])])):v;
 const same=(a,b)=>JSON.stringify(stable(a))===JSON.stringify(stable(b));
 const content=r=>{const c=clone(r);delete c.pack;return c;};
 const layoutKeys=['x','y','size','font','opacity','color','transparent','anchor','group'];
 const visualLayoutKeys=['width','height','rotation','flipX','flipY','layer','fit','trackColor','trackOpacity','fillOpacity','fontFamily','bold','italic','textEffect','textAlign'];
 function preserveFields(old,next,keys){for(const key of keys)if(own(old,key)&&old[key]!==undefined)next[key]=clone(old[key]);else delete next[key];}
 function sourcePack(incoming){
  if(!M.validRules(incoming))throw Error('Invalid overlay data or unsupported settings.');
  const packs=M.Packs.list(incoming);
  if(packs.length!==1||incoming.some(r=>r.pack?.instance!==packs[0].instance))throw Error('Export one complete pack to update it.');
  return packs[0];
 }
 function targets(existing,incoming){const source=sourcePack(incoming);return M.Packs.list(existing).filter(p=>p.id===source.id);}
 function plan(existing,incoming,instance,options={}){
  if(!M.validRules(existing,true))throw Error('Current overlays have invalid settings.');
  const config={layout:true,sounds:true,load:true,settings:true,keepRemoved:true,allowOlder:false,...options};
  const source=sourcePack(incoming),oldPack=M.Packs.list(existing).find(p=>p.instance===instance&&p.id===source.id);
  if(!oldPack)throw Error('Choose an installed copy of this pack.');
  if(source.version<=oldPack.version&&!config.allowOlder)throw Error('Allow the same or an older version to continue.');
  const oldRules=existing.filter(r=>r.pack?.instance===instance),oldMembers=new Map(oldRules.map(r=>[r.pack.member,r]));
  const reserved=new Set(existing.flatMap(r=>[r.id,...(r.pack?[r.pack.instance]:[]),...(r.style.group?[r.style.group.id]:[]),...M.flatten(r.roots).map(n=>n.id)]));
  const fresh=()=>{let id;do{id=M.id();}while(reserved.has(id));reserved.add(id);return id;};
  const groups=new Map(),resetSettings=[],resetKeys=new Set(),rows=[],sharedOverrides=new Map();
  for(const r of oldRules)for(const o of r.logic?.options||[])if(o.shared&&own(o,'value'))sharedOverrides.set(o.key,o);
  const reset=(r,o,old)=>{const key=(o.shared?'shared':r.pack.member)+'\0'+o.key;if(!resetKeys.has(key)){resetKeys.add(key);resetSettings.push({member:r.pack.member,overlay:r.name,key:o.key,name:o.name,previous:old.value});}};
  const updated=incoming.map(original=>{
   const r=clone(original),old=oldMembers.get(r.pack.member),incomingNodes=M.flatten(r.roots),oldNodes=new Map((old?M.flatten(old.roots):[]).map(n=>[n.id,n])),canonical=new Map(Object.entries(r.pack.nodes).map(([key,local])=>[local,key]));
   r.id=old?.id||fresh();r.enabled=old?.enabled??false;
   const nodeIDs=new Map(incomingNodes.map(n=>[n.id,own(old?.pack.nodes,canonical.get(n.id))?old.pack.nodes[canonical.get(n.id)]:fresh()]));
   if(config.sounds&&old)for(const n of incomingNodes){const previous=oldNodes.get(nodeIDs.get(n.id));if(n.type==='sound'&&previous?.type==='sound')n.props.sound=previous.props.sound;}
   for(const n of incomingNodes){n.id=nodeIDs.get(n.id);if(n.type==='call')n.props.action=nodeIDs.get(n.props.action);}
   r.pack.nodes=Object.fromEntries(Object.entries(r.pack.nodes).map(([key,local])=>[key,nodeIDs.get(local)]));
   r.pack.instance=instance;r.pack.name=oldPack.name;
   if(r.style.group){const sourceID=r.style.group.id;if(!groups.has(sourceID))groups.set(sourceID,fresh());r.style.group.id=groups.get(sourceID);}
   if(old&&config.layout){preserveFields(old.style,r.style,layoutKeys);if(visualLayoutKeys.some(k=>own(old.style.visual,k))||visualLayoutKeys.some(k=>own(r.style.visual,k))){r.style.visual||={};preserveFields(old.style.visual||{},r.style.visual,visualLayoutKeys);if(!Object.keys(r.style.visual).length)delete r.style.visual;}}
   if(old&&config.layout&&old.style.arc&&r.style.arc)preserveFields(old.style.arc,r.style.arc,['pair','start','sweep','thickness','gap','layout']);
   if(old&&config.load)r.load=clone(old.load||{});
   if(old&&config.sounds){
    preserveFields(old.style,r.style,['onShow','onHide']);
    for(const modifier of r.style.modifiers||[]){const matches=(old.style.modifiers||[]).filter(m=>same(m.condition,modifier.condition));if(matches.length===1)preserveFields(matches[0],modifier,['sound']);}
   }
   if(config.settings)for(const o of r.logic?.options||[]){
    const previous=o.shared?sharedOverrides.get(o.key):V.option(old?.logic,o.key);
    if(previous&&own(previous,'value')){if(previous.type===o.type&&V.accepts(o,previous.value))o.value=clone(previous.value);else reset(r,o,previous);}
    else if(o.shared){const local=V.option(old?.logic,o.key);if(local&&!local.shared&&own(local,'value'))reset(r,o,local);}
   }
   rows.push({member:r.pack.member,id:r.id,name:r.name,previous:old?.name,kind:!old?'added':same(content(old),content(r))?'unchanged':'changed'});
   return r;
  });
  const replacements=new Map(updated.map(r=>[r.pack.member,r])),oldIDs=new Set(oldRules.map(r=>r.id)),last=Math.max(...existing.map((r,i)=>oldIDs.has(r.id)?i:-1)),result=[];
  const added=updated.filter(r=>!oldMembers.has(r.pack.member));
  existing.forEach((original,i)=>{
   if(original.pack?.instance!==instance)result.push(clone(original));
   else {const r=replacements.get(original.pack.member);if(r)result.push(r);else{rows.push({member:original.pack.member,id:original.id,name:original.name,kind:'removed'});if(config.keepRemoved){const detached=clone(original);M.Packs.detach([],detached);detached.enabled=false;result.push(detached);}}}
   if(i===last)result.push(...added);
  });
  if(result.length>M.limits.rules)throw Error(`This update exceeds ${M.limits.rules} overlays. Remove obsolete members or other overlays first.`);
  if(!M.validRules(result,true))throw Error('This update exceeds the supported limits or has incompatible settings.');
  return {rules:result,instance,packID:source.id,name:oldPack.name,fromVersion:oldPack.version,toVersion:source.version,older:source.version<=oldPack.version,rows,resetSettings,keepRemoved:config.keepRemoved,first:updated[0]?.id};
 }
 function restore(existing,snapshot,options={}){
  if(!M.validRules(existing,true))throw Error('Current overlays have invalid settings.');
  const saved=clone(snapshot.rules),source=sourcePack(saved),keepRemoved=options.keepRemoved!==false,current=M.Packs.list(existing).find(p=>p.instance===source.instance);
  if(current&&current.id!==source.id)throw Error('This backup belongs to a different pack.');
  const wanted=new Map(saved.map(r=>[r.id,r])),replacements=new Map(wanted),rows=[],result=[];
  for(const r of existing){
   if(wanted.has(r.id)){
    if(r.pack&&r.pack.instance!==source.instance)throw Error('A backup member now belongs to another pack. Detach it before restoring.');
    const replacement=wanted.get(r.id);result.push(replacement);replacements.delete(r.id);rows.push({id:r.id,name:replacement.name,previous:r.name,kind:'restored'});
   }else if(r.pack?.instance===source.instance){
    rows.push({id:r.id,name:r.name,kind:'removed'});if(keepRemoved){const detached=clone(r);M.Packs.detach([],detached);detached.enabled=false;result.push(detached);}
   }else result.push(clone(r));
  }
  for(const r of replacements.values()){result.push(r);rows.push({id:r.id,name:r.name,kind:'restored'});}
  // A pack restore cannot roll back a shared group's unrelated members. Use
  // that group's current arrangement and report it in the review.
  const externalGroups=new Map(result.filter(r=>r.pack?.instance!==source.instance&&r.style.group).map(r=>[r.style.group.id,r.style])),sharedGroups=new Set();
  for(const r of saved){const other=externalGroups.get(r.style.group?.id);if(other&&(!same(other.group,r.style.group)||!same(other.anchor,r.style.anchor))){preserveFields(other,r.style,['group','anchor']);sharedGroups.add(other.group.name);}}
  if(result.length>M.limits.rules)throw Error(`This restore exceeds ${M.limits.rules} overlays. Remove extra members or other overlays first.`);
  if(!M.validRules(result,true))throw Error('This backup conflicts with current blocks, groups or capture limits.');
  return {rules:result,instance:source.instance,packID:source.id,name:source.name,fromVersion:current?.version??null,toVersion:source.version,rows,resetSettings:[],keepRemoved,first:saved[0]?.id,restoring:true,sharedGroups:[...sharedGroups],needsBackup:!!current};
 }
 const api={plan,restore,targets,sourcePack};root.OverlayPackUpdates=api;if(typeof module!=='undefined')module.exports=api;
})(typeof window!=='undefined'?window:globalThis);
