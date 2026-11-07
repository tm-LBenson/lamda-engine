/* Bounded typed calculations shared by the editor and sample preview. */
(function(root){
 'use strict';
 const limit=1e12,budgetLimit=1024,keyPattern=/^[A-Za-z][A-Za-z0-9_]{0,31}$/;
 const tokens=/\{value:([A-Za-z][A-Za-z0-9_]{0,31})\}/g;
 const exprKeys=['kind','value','key','source','field','op','args','check'];
 const operators={add:'+',sub:'−',mul:'×',div:'÷',mod:'Remainder',min:'Minimum',max:'Maximum',clamp:'Clamp',abs:'Absolute',neg:'Negate',floor:'Round down',ceil:'Round up',round:'Round',eq:'=',ne:'≠',lt:'<',lte:'≤',gt:'>',gte:'≥',all:'All',any:'Any',not:'Not',choose:'Choose',concat:'Join text'};
 const scalar=v=>typeof v==='number'?Number.isFinite(v)&&Math.abs(v)<=limit?Object.is(v,-0)?0:v:null:typeof v==='boolean'?v:typeof v==='string'&&[...v].length<=512&&!v.includes('\0')?v:null;
 const display=v=>{
  if(v===null||v===undefined)return '—';const text=String(v);if(typeof v!=='number'||!/[eE]/.test(text))return text;
  const [mantissa,exponent]=text.split(/e/i),negative=mantissa.startsWith('-'),unsigned=negative?mantissa.slice(1):mantissa,parts=unsigned.split('.'),digits=parts.join(''),at=parts[0].length+Number(exponent);
  return (negative?'-':'')+(at<=0?'0.'+'0'.repeat(-at)+digits:at>=digits.length?digits+'0'.repeat(at-digits.length):digits.slice(0,at)+'.'+digits.slice(at));
 };
 const literal=v=>({kind:'literal',value:v});
 const object=v=>v!==null&&typeof v==='object'&&!Array.isArray(v);
 const label=v=>typeof v==='string'&&v.trim()&&[...v].length<=60&&!/[\0\r\n]/.test(v);
 const clean=(o,keys)=>object(o)&&Object.keys(o).every(k=>keys.includes(k));
 const optionKeys=['shared','key','name','type','default','value','min','max','choices'];
 const valueKeys=['key','name','expr','remember'];
 const named=(l,k)=>l?.values?.find(v=>v.key===k),option=(l,k)=>l?.options?.find(o=>o.key===k);
 function accepts(o,v){v=scalar(v);if(v===null)return false;
  if(o.type==='number')return typeof v==='number'&&(o.min===undefined||v>=o.min)&&(o.max===undefined||v<=o.max);
  if(o.type==='boolean')return typeof v==='boolean';if(o.type==='text')return typeof v==='string'&&[...v].length<=120;
  return o.type==='choice'&&typeof v==='string'&&o.choices.includes(v);
 }
 function arity(op){if(['abs','floor','ceil','round','neg','not'].includes(op))return [1,1];if(['clamp','choose'].includes(op))return [3,3];if(['min','max','all','any','concat'].includes(op))return [2,8];return [2,2];}
 function validExpr(e,adapters,depth=1,counter={n:0}){
  if(!clean(e,exprKeys)||depth>8||++counter.n>64)return false;
  const no=(...keys)=>keys.every(k=>e[k]===undefined);
  if(e.kind==='literal')return scalar(e.value)!==null&&no('key','source','field','op','args','check');
  if(['value','option'].includes(e.kind))return typeof e.key==='string'&&keyPattern.test(e.key)&&no('value','source','field','op','args','check');
  if(e.kind==='data')return no('value','key','op','args','check')&&adapters.validData(e.source,e.field);
  if(e.kind==='predicate')return no('value','key','source','field','op','args')&&adapters.validCheck(e.check);
  if(e.kind!=='op'||!Object.hasOwn(operators,e.op)||!no('value','key','source','field','check')||!Array.isArray(e.args))return false;
  const [min,max]=arity(e.op);return e.args.length>=min&&e.args.length<=max&&e.args.every(a=>validExpr(a,adapters,depth+1,counter));
 }
 function validGraph(e,l,visiting=new Set(),depth=0,counter={n:0}){
  if(++counter.n>budgetLimit||depth>32)return false;
  if(e.kind==='option')return !!option(l,e.key);
  if(e.kind==='value'){const v=named(l,e.key);if(v?.remember)return true;if(!v||visiting.has(e.key))return false;visiting.add(e.key);const ok=validGraph(v.expr,l,visiting,depth+1,counter);visiting.delete(e.key);return ok;}
  return (e.args||[]).every(a=>validGraph(a,l,visiting,depth+1,counter));
 }
 function valid(l,adapters){
  if(l===undefined||l===null)return true;if(!clean(l,['values','options']))return false;
  if(l.values!==undefined&&!Array.isArray(l.values)||l.options!==undefined&&!Array.isArray(l.options))return false;
  const values=l.values||[],options=l.options||[],keys=new Set();if(values.length>16||options.length>16)return false;
  for(const o of options){if(!clean(o,optionKeys)||o.shared!==undefined&&typeof o.shared!=='boolean'||typeof o.key!=='string'||!keyPattern.test(o.key)||keys.has('o:'+o.key)||!label(o.name)||!['number','boolean','text','choice'].includes(o.type))return false;keys.add('o:'+o.key);
   if([o.min,o.max].some(v=>v!==undefined&&(typeof v!=='number'||scalar(v)===null))||o.min!==undefined&&o.max!==undefined&&o.min>o.max)return false;
   if(o.type!=='number'&&(o.min!==undefined||o.max!==undefined)||o.type!=='choice'&&o.choices!==undefined)return false;
   if(o.type==='choice'&&(!Array.isArray(o.choices)||!o.choices.length||o.choices.length>20||o.choices.some(v=>!label(v))||new Set(o.choices).size!==o.choices.length))return false;
   if(!accepts(o,o.default)||o.value!==undefined&&o.value!==null&&!accepts(o,o.value))return false;
  }
  for(const v of values){if(!clean(v,valueKeys)||typeof v.key!=='string'||!keyPattern.test(v.key)||keys.has('v:'+v.key)||!label(v.name)||!validExpr(v.expr,adapters)||!validRemember(v.remember,adapters))return false;keys.add('v:'+v.key);}
  return values.every(v=>validGraph(v.expr,l,new Set([v.key])));
 }
 function apply(op,args){
  if(!args.length||args.some(v=>v===null))return null;
  if(['eq','ne','lt','lte','gt','gte'].includes(op)&&args.length===2){const [a,b]=args;if(typeof a!==typeof b)return null;if(!['eq','ne'].includes(op)&&typeof a!=='number')return null;return {eq:a===b,ne:a!==b,lt:a<b,lte:a<=b,gt:a>b,gte:a>=b}[op];}
  if(op==='concat')return scalar(args.map(display).join(''));
  if(['all','any','not'].includes(op)){if(args.some(v=>typeof v!=='boolean'))return null;return op==='all'?args.every(Boolean):op==='any'?args.some(Boolean):!args[0];}
  if(args.some(v=>typeof v!=='number'))return null;
  const [a,b,c]=args;
  switch(op){case 'add':return scalar(a+b);case 'sub':return scalar(a-b);case 'mul':return scalar(a*b);case 'div':return b===0?null:scalar(a/b);case 'mod':return b===0?null:scalar(a%b);case 'min':return Math.min(...args);case 'max':return Math.max(...args);case 'abs':return Math.abs(a);case 'neg':return scalar(-a);case 'floor':return Math.floor(a);case 'ceil':return scalar(Math.ceil(a));case 'round':return scalar(Math.round(a));case 'clamp':return b<=c?Math.max(b,Math.min(c,a)):null;default:return null;}
 }
 function evaluation(l,env){
  let budget=budgetLimit;const memo=new Map(),visiting=new Set();
  function value(key){if(memo.has(key))return memo.get(key);if(visiting.has(key))return null;const v=named(l,key);if(!v)return null;if(v.remember)return scalar(env.memory?.values?.has(key)?env.memory.values.get(key):v.remember.initial);visiting.add(key);const result=expr(v.expr);visiting.delete(key);memo.set(key,result);return result;}
  function setting(key){const o=option(l,key);return o?scalar(o.value??o.default):null;}
  function expr(e,depth=0){
   if(--budget<0||depth>32||!e)return null;
   if(e.kind==='literal')return scalar(e.value);if(e.kind==='value')return value(e.key);if(e.kind==='option')return setting(e.key);
   if(e.kind==='data')return scalar(env.data(e.source,e.field));if(e.kind==='predicate'){const p=env.check(e.check);return typeof p==='boolean'?p:null;}
   if(e.kind!=='op'||!Array.isArray(e.args))return null;
   if(e.op==='choose'&&e.args.length===3){const c=expr(e.args[0],depth+1);return typeof c==='boolean'?expr(e.args[c?1:2],depth+1):null;}
   const args=[];for(const a of e.args){const v=expr(a,depth+1);if(e.op==='all'&&v===false)return false;if(e.op==='any'&&v===true)return true;args.push(v);}return apply(e.op,args);
  }
  return {value,option:setting,expr,text:s=>String(s).replace(tokens,(_,k)=>display(value(k)))};
 }
 function refs(rule){
  const values=new Set(),options=new Set();
  const text=s=>{for(const m of String(s||'').matchAll(tokens))values.add(m[1]);};
  const check=q=>{if(q.condition==='value'){values.add(q.key);if(!['true','false'].includes(q.operator)){if(q.compareTo==='value')values.add(q.other);if(q.compareTo==='option')options.add(q.other);}}for(const child of q.tests||[])check(child);};
  const walk=ns=>{for(const n of ns){if(n.type==='if')check(n.props);if(n.type==='show'){text(n.props.text);if(n.props.dataSource==='value'){values.add(n.props.dataValue);if(n.props.dataMaxValue)values.add(n.props.dataMaxValue);}}walk(n.children||[]);}};walk(rule.roots);
  for(const m of rule.style.modifiers||[]){check(m.condition);text(m.text);}return {values,options};
 }
 function validRule(rule,adapters){if(!valid(rule.logic,adapters))return false;const r=refs(rule);return [...r.values].every(k=>named(rule.logic,k))&&[...r.options].every(k=>option(rule.logic,k));}
 function inputs(rule){
  if(!rule.logic)return [];
  const seen=new Set(),out=[];
  const value=key=>{if(seen.has(key))return;seen.add(key);const v=named(rule.logic,key);if(v){if(v.remember){out.push({type:'when',props:v.remember.when});if(v.remember.reset==='encounter')out.push({type:'when',props:{event:'encounterstart'}});if(v.remember.check)out.push({type:'if',props:v.remember.check});}expr(v.expr);}};
  const expr=e=>{if(e.kind==='value')value(e.key);if(e.kind==='predicate')out.push({type:'if',props:e.check});if(e.kind==='data')out.push({type:'show',props:{...e.source,display:'text',text:'{'+e.field+'}'}});for(const a of e.args||[])expr(a);};
  for(const key of refs(rule).values)value(key);return out;
 }
 const whenKeys=['event','unit','spell','combatEvent','sourceUnit','targetUnit','extraSpell','school','critical','periodic','auraKind','miss','provider','timelineSource','textFilter','timerKind','encounterID','difficulty','outcome'];
 function validRemember(m,a){return m===undefined||m===null||clean(m,['initial','when','check','reset'])&&scalar(m.initial)!==null&&['loaded','combat','encounter','saved'].includes(m.reset)&&clean(m.when,whenKeys)&&a.validWhen(m.when)&&(m.check===undefined||a.validCheck(m.check));}
 function memoryState(){return {values:new Map(),latched:new Set()};}
 function remembered(rule){
  const seen=new Set();const expr=e=>{if(e.kind==='value')visit(e.key);for(const a of e.args||[])expr(a);};
  const visit=key=>{if(seen.has(key))return;seen.add(key);const v=named(rule.logic,key);if(v)expr(v.expr);};
  for(const key of refs(rule).values)visit(key);return (rule.logic?.values||[]).filter(v=>v.remember&&seen.has(v.key));
 }
 function remember(rule,event,m,adapters){
  const vs=remembered(rule),writes=new Map();
  for(const v of vs)if(v.remember.reset==='combat'&&event==='combat'||v.remember.reset==='encounter'&&event==='encounterstart'){m.values.delete(v.key);m.latched.delete(v.key);}
  for(const v of vs){const c=v.remember;if(c.when.event!==event)continue;
   const matches=adapters.matches(c.when)&&(!c.check||adapters.check(c.check)===true);
   if(!matches){m.latched.delete(v.key);continue;}
   if(['state','aura','cooldown','casting'].includes(event)&&m.latched.has(v.key))continue;
   const result=adapters.evaluation().expr(v.expr);if(result===null||typeof result!==typeof c.initial)continue;writes.set(v.key,result);m.latched.add(v.key);
  }
  for(const [key,v]of writes)m.values.set(key,v);
 }
 function validCondition(p){if(typeof p.key!=='string'||!keyPattern.test(p.key)||!['true','false','eq','ne','lt','lte','gt','gte'].includes(p.operator))return false;if(['true','false'].includes(p.operator))return true;if(!p.compareTo||p.compareTo==='literal')return scalar(p.value)!==null;return ['value','option'].includes(p.compareTo)&&typeof p.other==='string'&&keyPattern.test(p.other);}
 function condition(p,e){const a=e.value(p.key);if(a===null)return null;if(['true','false'].includes(p.operator))return typeof a==='boolean'?a===(p.operator==='true'):null;const b=p.compareTo==='value'?e.value(p.other):p.compareTo==='option'?e.option(p.other):scalar(p.value);return apply(p.operator,[a,b]);}
 const api={validRemember,memoryState,remembered,remember,limit,keyPattern,operators,exprKeys,optionKeys,valueKeys,scalar,display,literal,arity,valid,validExpr,validRule,validCondition,condition,refs,inputs,named,option,evaluation,accepts,apply};
 root.OverlayValues=api;if(typeof module!=='undefined')module.exports=api;
})(typeof window!=='undefined'?window:globalThis);
