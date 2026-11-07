/* Named action declarations expand into bounded call-site sequences. */
(function(root){
 'use strict';
 const compiled=Symbol('execution'),limit=256;
 const actionID=(rule,path,source)=>{let h=0xcbf29ce484222325n;for(const b of new TextEncoder().encode(rule+'|'+path+'|'+source))h=BigInt.asUintN(64,(h^BigInt(b))*0x100000001b3n);return 'a'+h.toString(16).padStart(16,'0');};
 const hasCalls=ns=>(ns||[]).some(n=>n.type==='call'||hasCalls(n.children));
 const definitions=rule=>(rule.roots||[]).filter(n=>n.type==='action');
 function compile(rule){
  if(rule[compiled])return rule;
  const ds=definitions(rule),defs=new Map(ds.map(n=>[n.id,n]));
  if(!ds.length&&!hasCalls(rule.roots))return rule;
  if(ds.length>16)throw Error('Keep at most 16 actions per overlay.');
  function expand(ns,path,visiting,depth,counter,inEach=false){
   return ns.map(source=>{
    if(++counter.n>limit||depth>8)throw Error('Action calls exceed the block or nesting limit.');
    if(['action','sequence'].includes(source.type))throw Error('Actions must be declared at the top level.');
    if(source.type==='each'&&inEach)throw Error('Calls cannot nest FOR EACH blocks.');
    const n={...source,id:path?actionID(rule.id,path,source.id):source.id};
    Object.defineProperty(n,'sourceID',{value:source.id});
    if(n.type==='call'){
     const key=n.props.action,def=defs.get(key);if(!def)throw Error('Choose an existing action.');
     if(visiting.has(key))throw Error('An action cannot call itself, directly or indirectly.');
     visiting.add(key);n.type='sequence';n.props={};n.children=expand(def.children,path+'/'+source.id+'/'+key,visiting,depth+1,counter,inEach);visiting.delete(key);
    }else if(source.children)n.children=expand(source.children,path,visiting,depth+1,counter,inEach||n.type==='each');
    return n;
   });
  }
  for(const def of ds)expand(def.children,'/'+def.id,new Set([def.id]),1,{n:0});
  const out={...rule,roots:expand(rule.roots.filter(n=>n.type!=='action'),'',new Set(),0,{n:0})};Object.defineProperty(out,compiled,{value:true});return out;
 }
 function executionRule(rule){try{return compile(rule);}catch(_){const out={...rule,roots:[]};Object.defineProperty(out,compiled,{value:true});return out;}}
 function valid(rule){try{compile(rule);return true;}catch(_){return false;}}
 function rules(rules){return rules.map(executionRule);}
 function targets(rule,from){
  const ds=definitions(rule),defs=new Map(ds.map(d=>[d.id,d]));
  const calls=ns=>ns.flatMap(n=>[...(n.type==='call'?[n.props.action]:[]),...calls(n.children||[])]);
  function reaches(key,target,seen=new Set()){if(key===target)return true;if(seen.has(key))return false;seen.add(key);return calls(defs.get(key)?.children||[]).some(k=>reaches(k,target,seen));}
  return ds.filter(d=>!from||!reaches(d.id,from));
 }
 function copies(roots,id){const ids=new Map();function walk(ns){return ns.flatMap(n=>[n,...walk(n.children||[])]);}
  const out=JSON.parse(JSON.stringify(roots));for(const n of walk(out)){const old=n.id;n.id=id();ids.set(old,n.id);}for(const n of walk(out))if(n.type==='call'&&ids.has(n.props.action))n.props.action=ids.get(n.props.action);return out;
 }
 const api={limit,actionID,compile,valid,executionRule,rules,definitions,copies,targets};root.OverlayActions=api;if(typeof module!=='undefined')module.exports=api;
})(typeof window!=='undefined'?window:globalThis);
