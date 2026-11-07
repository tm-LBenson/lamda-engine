/* Deterministic animation samples shared by layout previews and running tests. */
(function(root){
 'use strict';
 const keys=['duration','easing','cycle','alpha','x','y','scaleX','scaleY','rotation','color'];
 const phases={start:'Start',main:'Loop',finish:'Finish'},easings={linear:'Linear',in:'Ease in',out:'Ease out',inOut:'Ease in / out'};
 const presets={
  start:{fade:{label:'Fade in',alpha:0},grow:{label:'Grow',alpha:0,scaleX:0,scaleY:0},shrink:{label:'Shrink',alpha:0,scaleX:1.6,scaleY:1.6},left:{label:'Slide from left',alpha:0,x:-80},right:{label:'Slide from right',alpha:0,x:80},spin:{label:'Spin in',alpha:0,rotation:-180,scaleX:.3,scaleY:.3}},
  finish:{fade:{label:'Fade out',alpha:0},shrink:{label:'Shrink',alpha:0,scaleX:0,scaleY:0},grow:{label:'Grow',alpha:0,scaleX:1.6,scaleY:1.6},left:{label:'Slide left',alpha:0,x:-80},right:{label:'Slide right',alpha:0,x:80},spin:{label:'Spin out',alpha:0,rotation:180,scaleX:.3,scaleY:.3}},
  main:{pulse:{label:'Pulse',alpha:50,scaleX:1.12,scaleY:1.12},bob:{label:'Bounce',y:-24},spin:{label:'Spin',rotation:360,cycle:'repeat'},color:{label:'Color shift',color:'#EF9B9B'}}
 };
 const clamp=n=>Math.max(0,Math.min(1,n));
 function valid(m,colorValue){
  if(m==null)return true;if(typeof m!=='object'||Array.isArray(m)||Object.keys(m).some(k=>!Object.hasOwn(phases,k)))return false;
  return Object.keys(phases).every(phase=>{const e=m[phase];if(e==null)return true;if(typeof e!=='object'||Array.isArray(e)||Object.keys(e).some(k=>!keys.includes(k)))return false;
   const range=(k,a,b)=>e[k]===undefined||Number.isFinite(e[k])&&e[k]>=a&&e[k]<=b;
   return Number.isFinite(e.duration)&&e.duration>=.05&&e.duration<=(phase==='finish'?30:300)&&[undefined,'',...Object.keys(easings)].includes(e.easing)&&[undefined,'','repeat','pingpong'].includes(e.cycle)&&range('alpha',0,100)&&range('x',-2048,2048)&&range('y',-2048,2048)&&range('scaleX',0,4)&&range('scaleY',0,4)&&range('rotation',-3600,3600)&&(e.color===undefined||typeof e.color==='string'&&(!e.color||!!colorValue(e.color)));});
 }
 function preset(phase,name){if(name==='none')return null;const {label,...p}=presets[phase][name]||{};return {duration:phase==='main'?1.5:.4,easing:name==='spin'?'linear':phase==='main'?'inOut':phase==='start'?'out':'in',...(phase==='main'?{cycle:'pingpong'}:{}),...p};}
 function presetName(phase,e){if(!e)return 'none';const endpoint=v=>JSON.stringify(Object.fromEntries(keys.filter(k=>!['duration','easing'].includes(k)&&v[k]!==undefined).map(k=>[k,v[k]])));return Object.keys(presets[phase]).find(k=>endpoint(preset(phase,k))===endpoint(e))||'custom';}
 function ease(t,kind){t=clamp(t);if(kind==='in')return t*t;if(kind==='out')return 1-(1-t)*(1-t);if(kind==='inOut')return t<.5?2*t*t:1-2*(1-t)*(1-t);return t;}
 function colorBlend(a,b,w){const rgb=x=>[1,3,5].map(i=>parseInt(x.slice(i,i+2),16));const x=rgb(a),y=rgb(b);return '#'+x.map((v,i)=>Math.floor(v+(y[i]-v)*w+.5).toString(16).padStart(2,'0')).join('').toUpperCase();}
 function sample(m,elapsed,color,colorValue,finishAt=null){
  const result={alpha:1,x:0,y:0,scaleX:1,scaleY:1,rotation:0,color};
  if(!m||elapsed==null)return result;
  elapsed=Math.max(0,elapsed);
  if(m.finish&&finishAt!==null){
   const from=sample(m,finishAt,color,colorValue),e=m.finish,w=ease((elapsed-finishAt)/e.duration,e.easing);
   from.alpha*=1+((e.alpha??100)/100-1)*w;from.x+=(e.x||0)*w;from.y+=(e.y||0)*w;from.scaleX*=1+((e.scaleX??1)-1)*w;from.scaleY*=1+((e.scaleY??1)-1)*w;from.rotation+=(e.rotation||0)*w;if(e.color)from.color=colorBlend(from.color,colorValue(e.color),w);return from;
  }
  let e,w;
  if(m.start&&elapsed<m.start.duration){e=m.start;w=1-ease(elapsed/e.duration,e.easing);}
  else if(m.main){e=m.main;const t=((elapsed-(m.start?.duration||0))/e.duration)%1;w=ease(e.cycle==='repeat'?t:t<.5?t*2:2-t*2,e.easing);}
  else return result;
  result.alpha=1+((e.alpha??100)/100-1)*w;result.x=(e.x||0)*w;result.y=(e.y||0)*w;result.scaleX=1+((e.scaleX??1)-1)*w;result.scaleY=1+((e.scaleY??1)-1)*w;result.rotation=(e.rotation||0)*w;
  if(e.color)result.color=colorBlend(color,colorValue(e.color),w);return result;
 }
 function changes(e){return e&&((e.alpha??100)!==100||e.x||e.y||(e.scaleX??1)!==1||(e.scaleY??1)!==1||e.rotation||e.color);}
 function active(m,elapsed,finishAt=null){if(m?.finish&&finishAt!==null)return elapsed-finishAt<m.finish.duration&&!!changes(m.finish);return !!m&&(m.start&&elapsed<m.start.duration?!!changes(m.start):!!changes(m.main));}
 function portable(m){return Object.fromEntries(Object.keys(phases).filter(p=>m[p]).map(p=>[p,Object.fromEntries(keys.filter(k=>Object.hasOwn(m[p],k)).map(k=>[k,m[p][k]]))]));}
 const api={keys,phases,easings,presets,valid,preset,presetName,ease,sample,active,portable};root.OverlayMotion=api;if(typeof module!=='undefined')module.exports=api;
})(typeof window!=='undefined'?window:globalThis);
