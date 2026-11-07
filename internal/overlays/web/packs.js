(function(root){
 'use strict';
 const limits=root.OverlayLimits||(typeof require==='function'?require('./limits.js'):null);
 const clone=v=>JSON.parse(JSON.stringify(v)),id=/^[a-zA-Z0-9_-]{1,80}$/;
 const keys=['nodes','id','instance','member','members','name','version'];
 const canonical=v=>JSON.stringify(v===undefined?null:stable(v));
 function stable(v){if(Array.isArray(v))return v.map(stable);if(v&&typeof v==='object')return Object.fromEntries(Object.keys(v).sort().map(k=>[k,stable(v[k])]));return v;}
 const signature=p=>canonical({...p,nodes:null,member:'',members:[...p.members].sort()});
 const options=r=>r.logic?.options||[];
 const option=(r,k)=>options(r).find(o=>o.key===k);
 const members=(rules,rule)=>rule?.pack?rules.filter(r=>r.pack?.instance===rule.pack.instance):rule?[rule]:[];
 function list(rules){const found=new Map();for(const r of rules)if(r.pack)found.set(r.pack.instance,r.pack);return [...found.values()];}
 function valid(rules){
  const groups=new Map();
  for(const r of rules){const p=r.pack;if(p===undefined||p===null){if(options(r).some(o=>o.shared))return false;continue;}
   if(typeof p!=='object'||Array.isArray(p)||Object.keys(p).some(k=>!keys.includes(k))||!['id','instance','member'].every(k=>typeof p[k]==='string'&&id.test(p[k]))||typeof p.name!=='string'||!p.name.trim()||[...p.name].length>60||/[\x00-\x1f\x7f]/.test(p.name)||!Number.isInteger(p.version)||p.version<1||p.version>1e6||!Array.isArray(p.members)||!p.members.length||p.members.length>limits.rules||p.members.some(v=>typeof v!=='string'||!id.test(v))||new Set(p.members).size!==p.members.length||!p.members.includes(p.member))return false;
   if(!validNodes(r))return false;
   if(!groups.has(p.instance))groups.set(p.instance,[]);groups.get(p.instance).push(r);
  }
  for(const rs of groups.values()){const p=rs[0].pack,shared=new Map(),seen=new Set();if(rs.length!==p.members.length)return false;
   for(const r of rs){if(signature(r.pack)!==signature(p)||seen.has(r.pack.member))return false;seen.add(r.pack.member);for(const o of options(r))if(o.shared){if(shared.has(o.key)&&shared.get(o.key)!==canonical(o))return false;shared.set(o.key,canonical(o));}}
   for(const r of rs)for(const [key,value] of shared)if(canonical(option(r,key))!==value)return false;
  }
  return true;
 }
 function walk(ns){return ns.flatMap(n=>[n,...walk(n.children||[])]);}
 function validNodes(r){const p=r.pack,nodes=walk(r.roots),mapping=p.nodes;if(!mapping||typeof mapping!=='object'||Array.isArray(mapping)||Object.keys(mapping).length!==nodes.length||nodes.length>80)return false;const wanted=new Set(nodes.map(n=>n.id)),seen=new Set();for(const [source,local]of Object.entries(mapping)){if(!id.test(source)||typeof local!=='string'||!wanted.has(local)||seen.has(local))return false;seen.add(local);}return true;}
 function syncNodes(rule){if(!rule.pack)return;const nodes=walk(rule.roots),wanted=new Set(nodes.map(n=>n.id)),mapping=Object.create(null);for(const [source,local]of Object.entries(rule.pack.nodes||{}))if(wanted.has(local))mapping[source]=local;const known=new Set(Object.values(mapping));for(const n of nodes)if(!known.has(n.id)){let source=n.id,i=1;while(Object.hasOwn(mapping,source))source='source'+i++;mapping[source]=n.id;}rule.pack.nodes=mapping;}
 function refresh(rules,instance){const rs=rules.filter(r=>r.pack?.instance===instance),ids=rs.map(r=>r.pack.member).sort();for(const r of rs)r.pack.members=[...ids];}
 function detach(rules,rule){const old=rule.pack?.instance;delete rule.pack;for(const o of options(rule))delete o.shared;if(old)refresh(rules,old);}
 function detachPartial(rules){const out=clone(rules);for(const p of list(out)){const rs=out.filter(r=>r.pack?.instance===p.instance);if(rs.length!==p.members.length||rs.some(r=>!p.members.includes(r.pack.member)))for(const r of rs){delete r.pack;for(const o of options(r))delete o.shared;}}return out;}
 function create(rules,rule,fresh){detach(rules,rule);const member=fresh();rule.pack={id:fresh(),instance:fresh(),member,members:[member],name:rule.name,version:1};syncNodes(rule);return rule.pack;}
 function freeKey(rules,prefix='packSetting'){let n=1,key;do{key=prefix+n++;}while(rules.some(r=>option(r,key)));return key;}
 function renameOption(rule,old,key){
  const expr=e=>{if(e.kind==='option'&&e.key===old)e.key=key;for(const a of e.args||[])expr(a);};
  for(const v of rule.logic?.values||[])expr(v.expr);
  const check=p=>{if(p.condition==='value'&&p.compareTo==='option'&&p.other===old)p.other=key;for(const c of p.tests||[])check(c);};
  const walk=ns=>{for(const n of ns){if(n.type==='if')check(n.props);if(n.children)walk(n.children);}};walk(rule.roots);
  for(const m of rule.style.modifiers||[])check(m.condition);if(rule.load?.conditions)check(rule.load.conditions);
  const o=option(rule,old);if(o)o.key=key;
 }
 function join(rules,rule,instance,fresh){
  if(rule.pack?.instance===instance)return;
  const source=rules.find(r=>r.pack?.instance===instance);if(!source)throw Error('That pack no longer exists.');
  const p=clone(source.pack),shared=options(source).filter(o=>o.shared).map(clone);
  if(options(rule).length+shared.length>16)throw Error('This overlay needs room for the pack’s settings (16 settings maximum).');
  detach(rules,rule);rule.pack={...p,member:fresh()};
  for(const o of shared){if(option(rule,o.key))renameOption(rule,o.key,freeKey([...members(rules,source),rule],'localSetting'));((rule.logic||={}).options||=[]).push(o);}
  refresh(rules,instance);syncNodes(rule);
 }
 function shareOption(rules,rule,key,shared){
  if(!rule.pack)throw Error('Choose a pack first.');const rs=members(rules,rule),o=option(rule,key);if(!o)throw Error('Setting no longer exists.');
  if(!shared){for(const r of rs){const setting=option(r,key);if(setting)delete setting.shared;}return key;}
  if(o.shared)return key;
  if(rs.some(r=>r!==rule&&options(r).length>=16))throw Error('A pack member already has 16 settings.');
  if(rs.some(r=>r!==rule&&option(r,key))){const next=freeKey(rs);renameOption(rule,key,next);key=next;}
  o.shared=true;for(const r of rs)if(r!==rule)((r.logic||={}).options||=[]).push(clone(o));return key;
 }
 // Editor mutations change one copy. Fan out an intentional shared-setting edit
 // before validation; reject contradictory simultaneous edits instead of picking one.
 function reconcile(before,after){
  for(const p of list(after)){const rs=after.filter(r=>r.pack?.instance===p.instance),changes=new Map();
   for(const r of rs){const old=before.find(b=>b.id===r.id&&b.pack?.instance===p.instance);if(!old)continue;
    for(const key of new Set([...options(old),...options(r)].filter(o=>o.shared).map(o=>o.key))){const a=option(old,key),b=option(r,key);if(canonical(a)===canonical(b))continue;
     if(changes.has(key)&&canonical(changes.get(key))!==canonical(b))throw Error('Shared settings must match across the pack.');changes.set(key,b?clone(b):null);
    }
   }
   for(const [key,value] of changes)for(const r of rs){const l=r.logic||={};l.options=(l.options||[]).filter(o=>o.key!==key);if(value)l.options.push(clone(value));}
  }
  for(const r of after)syncNodes(r);
 }
 function edit(rules,rule,key,value){if(!rule.pack)return;for(const r of members(rules,rule))r.pack[key]=value;}
 const api={valid,syncNodes,list,members,refresh,detach,detachPartial,create,join,shareOption,reconcile,edit,renameOption,keys};
 root.OverlayPacks=api;if(typeof module!=='undefined')module.exports=api;
})(typeof window!=='undefined'?window:globalThis);
