'use strict';
const $ = id => document.getElementById(id);
const text = (id,value) => { $(id).textContent=value; };
let planSequence=0, lastPlan=null, planTimer, upgradeSignature='';
let state, selectedID=new URLSearchParams(location.search).get('snapshot')||'', signature='', busy=false, selectedJobID='', disconnected=false;
const checked = new Set();
// Stable specialization IDs, including names whose calculator is unavailable.
const specNames={62:'Arcane',63:'Fire',64:'Frost',65:'Holy',66:'Protection',70:'Retribution',71:'Arms',72:'Fury',73:'Protection',102:'Balance',103:'Feral',104:'Guardian',105:'Restoration',250:'Blood',251:'Frost',252:'Unholy',253:'Beast Mastery',254:'Marksmanship',255:'Survival',256:'Discipline',257:'Holy',258:'Shadow',259:'Assassination',260:'Outlaw',261:'Subtlety',262:'Elemental',263:'Enhancement',264:'Restoration',265:'Affliction',266:'Demonology',267:'Destruction',268:'Brewmaster',269:'Windwalker',270:'Mistweaver',577:'Havoc',581:'Vengeance',1480:'Devourer',1467:'Devastation',1468:'Preservation',1473:'Augmentation'};
const specName=snapshot=>specNames[snapshot?.character.spec]||(snapshot?.character.spec?'Spec '+snapshot.character.spec:'No specialization');
const className=value=>({DEATHKNIGHT:'Death Knight',DEMONHUNTER:'Demon Hunter'}[value]||(value?value[0]+value.slice(1).toLowerCase():''));
const validationName=value=>({'review-required':'Needs review',unreviewed:'Not reviewed',reviewed:'Reviewed','item-stats':'Item stats'}[value]||value||'Not reviewed');
const captureWarning=warning=>/^Loot capture[: ]/i.test(warning)||warning==='Some loot tables were unavailable. Run Sim again to retry.';
const encounterDescription=scenario=>scenario?.targets>0&&scenario?.seconds>0&&scenario?.iterations>0?scenario.targets+' target'+(scenario.targets===1?'':'s')+' · '+scenario.seconds+'s · '+scenario.iterations+' iterations':'Encounter settings unavailable';
const groupDescription=scenario=>(scenario?.augmentationMode==='raid'?'Raid':'Dungeon')+' · allies ilvl '+(scenario?.allyItemLevel||328)+' · '+Math.round((scenario?.allySkill||0.9)*100)+'% performance · '+encounterDescription(scenario);
function unavailableMessage(summary){
 if(!summary)return 'Import a character from WoW to begin.';
 if(summary.snapshot.character.spec===1473&&summary.capability.reason?.includes('Augmentation sims'))return 'Your character was imported. Group-damage comparisons are not supported by this engine yet.';
 return summary.capability.reason||'No calculator is available for this snapshot.';
}
function selectResult(job){selectedID=job.snapshotId;selectedJobID=job.id;signature='';$('augmentation-controls').dataset.snapshot='';for(const id of ['targets','seconds','iterations'])if(job.scenario?.[id]>0)$(id).value=job.scenario[id];render();tab('overview');}
function renderSupport(summary){
 const blocked=!!summary&&!summary.capability.available;
 $('support-panel').hidden=!blocked||$('character-panel').hidden;
 text('support-title',specName(summary?.snapshot)+' calculation unavailable');
 text('support-message',unavailableMessage(summary));
 const links=$('related-results');links.replaceChildren();
 if(!blocked)return;
 const character=summary.snapshot.character,seen=new Set();
 for(const job of state.jobs){
  const saved=state.snapshots.find(s=>s.snapshot.id===job.snapshotId),other=saved?.snapshot.character;
  if(!job.result||job.state!=='complete'||!other||job.snapshotId===selectedID||other.name!==character.name||other.realm!==character.realm||other.class!==character.class||seen.has(other.spec))continue;
  seen.add(other.spec);const button=node('button','View '+specName(saved.snapshot)+' result','quiet');button.addEventListener('click',()=>selectResult(job));links.append(button);if(seen.size===3)break;
 }
}
function renderWarnings(snapshot,result){
 const notes=$('warnings'),capture=$('capture-warnings');notes.replaceChildren();capture.replaceChildren();
 const warnings=new Set([...(snapshot?.warnings||[]),...(result?.warnings||[])]);
 if(snapshot&&Date.now()/1000-snapshot.capturedAt>86400)warnings.add('Snapshot is over a day old. Export again to update it.');
 for(const warning of warnings){
  const technical=captureWarning(warning);
  (technical?capture:notes).append(node('li',warning));
 }
 $('warnings-panel').hidden=!notes.children.length;$('capture-details').hidden=!capture.children.length;
 const loot=current(),complete=(loot?.lootPreviews||[]).filter(p=>p.complete).length;
 text('capture-summary',loot?.lootItems?'Loot capture · '+number(loot.lootItems)+' items · '+complete+' of '+loot.lootPools+' sources complete':'Capture details');
}
function appendRunNotes(holder,result){
 const notes=node('ul',undefined,'run-notes'),capture=node('ul');
 for(const warning of new Set(result.warnings||[]))(captureWarning(warning)?capture:notes).append(node('li',warning));
 if(notes.children.length)holder.append(notes);
 if(capture.children.length){const details=node('details');details.append(node('summary','Capture details'),capture);holder.append(details);}
 if(result.modelInputs){const details=node('details');details.append(node('summary','Model inputs'),table(['Input','Source'],Object.entries(result.modelInputs)));holder.append(details);}
}

const date = value => value ? new Date(value).toLocaleString() : 'Not checked';
const number = value => Number(value).toLocaleString(undefined,{maximumFractionDigits:1});
const percent = value => value!==0&&Math.abs(value)<0.01?(value<0?'−':'')+'<0.01%':Number(value).toLocaleString(undefined,{maximumFractionDigits:2})+'%';
const slotName = value => (value[0].toUpperCase()+value.slice(1)).replaceAll('_',' ').replace(/(\d)/g,' $1');
function node(tag,value,cls){const el=document.createElement(tag);if(value!==undefined)el.textContent=value;if(cls)el.className=cls;return el;}
async function api(path,data){const response=await fetch('/api/'+path,data===undefined?{}:{method:'POST',headers:{'Content-Type':'application/json','X-Lamda-Sim':'1'},body:JSON.stringify(data)});const body=await response.json();if(!response.ok)throw Error(body.error||'Request failed');return body;}
function message(error){text('message',error instanceof Error?error.message:String(error||''));}
function tab(name){document.querySelectorAll('.tab').forEach(el=>el.hidden=el.id!==name);document.querySelectorAll('[data-tab]').forEach(el=>el.classList.toggle('active',el.dataset.tab===name));text('title',document.querySelector('[data-tab="'+name+'"]').textContent);$('character-panel').hidden=['audit','about'].includes(name);$('import-panel').hidden=true;if(state)renderSupport(current());}
document.querySelectorAll('[data-tab]').forEach(el=>el.addEventListener('click',()=>tab(el.dataset.tab)));
function current(){return state?.snapshots.find(s=>s.snapshot.id===selectedID);}
function table(headers,rows){const t=node('table'),head=node('tr');headers.forEach(h=>head.append(node('th',h)));t.append(head);rows.forEach(cells=>{const tr=node('tr');cells.forEach(v=>{const td=node('td');td.append(v instanceof Node?v:document.createTextNode(String(v)));tr.append(td);});t.append(tr);});const wrap=node('div',undefined,'table-wrap');wrap.append(t);return wrap;}
function renderCandidates(summary){for(const source of ['owned','vault']){const list=$(source==='owned'?'gear-list':'vault-list');list.replaceChildren();const items=summary?.candidates.filter(c=>c.source===source)||[];if(!items.length){list.append(node('p','No '+(source==='vault'?'Vault choices':'bag items')+' in this snapshot.'));continue;}items.forEach(c=>{const label=node('label',undefined,'item'),input=node('input');input.type='checkbox';input.checked=checked.has(c.id);input.disabled=!summary?.capability.available;input.addEventListener('change',()=>{if(input.checked){if(checked.size>=8){input.checked=false;message('Choose up to 8 items.');return;}checked.add(c.id);}else checked.delete(c.id);});const description=node('span');description.append(node('strong',c.label||c.slot),node('small',slotName(c.slot)));label.append(input,description);list.append(label);});}}
function render(){const summary=current(),snapshot=summary?.snapshot;const identity=state.snapshots.map(s=>s.snapshot.id).join('|');if($('snapshot').dataset.identity!==identity){$('snapshot').replaceChildren(...state.snapshots.map(s=>{const o=node('option',s.snapshot.character.name+' · '+specName(s.snapshot)+' · '+date(s.snapshot.capturedAt*1000));o.value=s.snapshot.id;return o;}));if(!state.snapshots.length){const empty=node('option','No snapshots');empty.value='';$('snapshot').append(empty);}$('snapshot').dataset.identity=identity;}$('snapshot').value=selectedID;
text('client',state.client==='retail'?'Retail':'Forever');text('connection','Engine connected');text('character',snapshot?snapshot.character.name+' · '+snapshot.character.realm:'No character loaded');text('character-info',snapshot?'Level '+snapshot.character.level+' · '+specName(snapshot)+' '+className(snapshot.character.class):'In WoW: /lsim → Run Sim.');text('import-status',snapshot?'Imported '+date(snapshot.capturedAt*1000):'');renderSupport(summary);
const available=summary?.capability.available;const active=state.jobs.find(j=>['running','cancelling'].includes(j.state));$('run').disabled=!available||!!active||busy;document.querySelectorAll('.compare').forEach(b=>b.disabled=!available||!!active||busy);$('cancel').hidden=!active;$('cancel').disabled=active?.state==='cancelling';text('capability',available?(summary.capability.reason||'Ready to calculate.'):'');text('progress',active?(active.state==='cancelling'?'Cancelling':'Running')+' · '+Math.floor((Date.now()-Date.parse(active.createdAt))/1000)+'s':'');
const sig=selectedID+':'+(summary?.candidates.length||0)+':'+!!available;if(signature!==sig){checked.clear();renderCandidates(summary);signature=sig;}
const latest=state.jobs.find(j=>j.id===selectedJobID&&j.snapshotId===selectedID)||state.jobs.find(j=>j.snapshotId===selectedID),result=$('result');result.replaceChildren();if(latest?.result?.metric==='item_stats'){renderStats(result,latest.result);}else if(latest?.result){const r=latest.result,m=node('div',number(r.baseline.mean), 'metric');m.append(node('small',metricName(r)+' · '+validationName(r.validation)),node('small',r.baseline.meanError>0?'Uncertainty: ±'+number(r.baseline.meanError)+' '+metricName(r):(r.metric==='healing_score'?'Preset model':'Uncertainty unavailable')));result.append(m,node('p',(r.provider||'SimulationCraft')+' '+r.providerVersion+' · '+(r.model||latest.scenario.targets+' target(s), '+latest.scenario.seconds+'s, '+latest.scenario.iterations+' iterations.')));const rows=r.comparisons.map(c=>{const item=summary.candidates.find(x=>x.id===c.name);const overlap=r.baseline.meanError>0&&c.meanError>0&&Math.abs(c.mean-r.baseline.mean)<=r.baseline.meanError+c.meanError;return [c.label||item?.label||c.name,number(c.mean)+(c.meanError>0?' ± '+number(c.meanError):''),((c.mean/r.baseline.mean-1)*100).toFixed(2)+'%'+(overlap?' · uncertainty overlaps':'')];});if(rows.length)result.append(table(['Comparison',metricName(r),r.metric==='group_dps'?'Group gain':'Change'],rows));if(r.metric==='group_dps')result.append(node('p',groupDescription(latest.scenario),'group-scenario'),node('p','You + fixed simulated allies. Gains are a percentage of total Group DPS.','group-note'));}else if(latest){result.append(node('h2',latest.state),node('p',latest.error||'Waiting for results…'));if(latest.state==='failed')result.append(logLink(latest.id));}else result.append(node('p',!summary?'Import your character to begin.':!available?'Select an available snapshot or view a saved result.':summary.capability.id==='forever-stats'?'Choose bag items to compare their stats.':'Ready. Run a calculation or choose gear to compare.'));
if(latest?.result?.modelInputs){const details=node('details'),summary=node('summary','Model inputs');details.append(summary,table(['Input','Source'],Object.entries(latest.result.modelInputs)));result.append(details);}
if(latest?.result?.provider==='SimulationCraft'&&latest.result.inputRevision!==2){result.prepend(node('p','Older item export format. Rerun before using this result.','note'));}
if(latest?.result?.rejectedComparisons?.length){const details=node('details');details.append(node('summary','Skipped comparisons'),table(['Set','Reason'],latest.result.rejectedComparisons.map(c=>[c.label||c.id,c.reason])));result.append(details);}
renderWarnings(snapshot,latest?.result);
const audit=$('audit-results');audit.replaceChildren(node('p','Last checked: '+date(state.audit.checkedAt)));audit.append(table(['Source','Status','Checked'],(state.audit.sources||[]).map(s=>[s.name,s.status,date(s.lastSuccess)])));text('provider',state.provider.name+' · '+(state.provider.version||'Not installed')+(state.provider.reason?' · '+state.provider.reason:''));
const history=$('history-list');history.replaceChildren(table(['Character','Started','Status','Scenario','Result','Actions'],state.jobs.map(j=>{const saved=state.snapshots.find(s=>s.snapshot.id===j.snapshotId);const b=node('button',saved?saved.snapshot.character.name+' · '+specName(saved.snapshot):'Archived snapshot','quiet');b.addEventListener('click',()=>{if(!state.snapshots.some(s=>s.snapshot.id===j.snapshotId)){message('Snapshot archived.');return;}selectResult(j);});const actions=node('div');const rerun=node('button','Rerun','quiet');rerun.disabled=!!active||!saved?.capability.available||busy;rerun.addEventListener('click',async()=>{try{const job=await api('run',{id:j.snapshotId,scenario:j.scenario,selected:j.selected||[]});selectedID=j.snapshotId;selectedJobID=job.id;state=await api('state');render();tab('overview');}catch(e){message(e);}});const download=node('a','Save');download.href='/api/result?id='+encodeURIComponent(j.id);actions.append(rerun,download);return[b,date(j.createdAt),j.state,j.result?.metric==='group_dps'?groupDescription(j.scenario):j.result?.model||j.scenario.targets+' targets / '+j.scenario.seconds+'s',j.result?(j.result.metric==='item_stats'?'Item stats':number(j.result.baseline.mean)+' '+metricName(j.result)):j.error||'—',actions];})));if(!state.jobs.length)history.replaceChildren(node('p','No runs yet.'));
renderFeatures(summary,latest,active);
$('save-review').disabled=!summary;text('provider',(summary?.capability.name||state.provider.name)+' · '+(summary?.capability.version||state.provider.version||'Not installed'));
$('review-list').replaceChildren(table(['Date','Calculator','Build / spec','Notes'],(state.reviews||[]).map(r=>[date(r.at),r.provider+' '+r.version,r.build+' / '+(specNames[r.spec]||r.spec),r.notes])));
}
function logLink(id){const a=node('a','Open worker log');a.href='/api/log?id='+encodeURIComponent(id);a.target='_blank';a.rel='noopener';return a;}
async function refresh(){try{state=await api('state');if(disconnected){message('');disconnected=false;}if(!state.snapshots.some(s=>s.snapshot.id===selectedID))selectedID=state.snapshots[0]?.snapshot.id||'';render();}catch(e){disconnected=true;text('connection','Engine disconnected');['run','run-talents','find-upgrades','open-healing'].forEach(id=>$(id).disabled=true);document.querySelectorAll('.compare').forEach(b=>b.disabled=true);message(e);}finally{setTimeout(refresh,2500);}}
async function run(source,extra={}){
 if(busy)return;const summary=current();if(!summary?.capability.available){message(unavailableMessage(summary));return;}
 busy=true;message('');try{
  const selected=source?[...checked].filter(id=>summary.candidates.some(c=>c.id===id&&($('compare-mode').value==='topgear'||c.source===source))):[];
  if(source&&!selected.length)throw Error('Choose an item first.');
  const scenario={targets:Number($('targets').value),seconds:Number($('seconds').value),iterations:Number($('iterations').value),healingMode:summary.capability.id==='qe'?$('healing-mode').value:'',compareMode:source?$('compare-mode').value:'individual',...extra};
  if(summary.snapshot.character.spec===1473&&summary.snapshot.client==='retail'){
   for(const id of ['ally-item-level','ally-skill'])if(!$(id).checkValidity()){$(id).closest('details').open=true;$(id).reportValidity();throw Error('Check the simulated ally settings.');}
   scenario.augmentationMode=$('augmentation-mode').value;scenario.allyItemLevel=Number($('ally-item-level').value);scenario.allySkill=Number($('ally-skill').value)/100;
  }
  const job=await api('run',{id:selectedID,scenario,selected});selectedJobID=job.id;tab(extra.task==='upgrades'?'plan':'overview');state=await api('state');render();
 }catch(e){message(e);}finally{busy=false;render();}
}
$('snapshot').addEventListener('change',()=>{selectedID=$('snapshot').value;selectedJobID='';signature='';render();});$('run').addEventListener('click',()=>run());document.querySelectorAll('.compare').forEach(b=>b.addEventListener('click',()=>run(b.dataset.source)));
$('cancel').addEventListener('click',async()=>{try{const j=state.jobs.find(j=>j.state==='running');if(j)await api('cancel',{id:j.id});}catch(e){message(e);}});
$('audit-now').addEventListener('click',async()=>{try{await api('audit',{});message('Checking for updates…');}catch(e){message(e);}});
$('show-import').addEventListener('click',()=>{$('import-panel').hidden=false;$('import-text').focus();});$('hide-import').addEventListener('click',()=>{$('import-panel').hidden=true;});$('import').addEventListener('click',async()=>{try{const result=await api('import',JSON.parse($('import-text').value));selectedID=result.id;$('import-panel').hidden=true;$('import-text').value='';message('');state=await api('state');render();}catch(e){message(e);}});
function metricName(r){return r.metric==='group_dps'?'Group DPS':r.metric==='healing_score'?'Healing score':r.metric==='item_stats'?'Item stats':r.metric||'DPS';}
function statName(value){return value.replace(/^ITEM_MOD_|_SHORT$/g,'').replaceAll('_',' ').toLowerCase().replace(/^./,c=>c.toUpperCase());}
function renderStats(holder,r){const names=new Set(Object.keys(r.baseline.stats||{}));for(const c of r.comparisons||[])Object.keys(c.stats||{}).forEach(k=>names.add(k));holder.append(node('p','Captured item stats'),table(['Stat','Equipped',...(r.comparisons||[]).map(c=>c.label||c.name)],[...names].sort().map(key=>[statName(key),number(r.baseline.stats[key]||0),...(r.comparisons||[]).map(c=>{const delta=(c.stats[key]||0)-(r.baseline.stats[key]||0);return number(c.stats[key]||0)+' ('+(delta>0?'+':'')+number(delta)+')';})])));}
function itemTable(items){return table(['Slot','Item','Level'],items.map(i=>[i.slot,(i.label||i.id)+(i.craftedStats?' · '+i.craftedStats:''),i.level||'—']));}
function renderFeatures(summary,latest,active){
 const healer=summary?.capability.id==='qe',stats=summary?.capability.id==='forever-stats',available=!!summary?.capability.available;
 const aug=summary?.snapshot.client==='retail'&&summary.snapshot.character.spec===1473,blocked=unavailableMessage(summary);
 $('healer-controls').hidden=!healer||!available;$('damage-controls').hidden=healer||stats||!available;$('augmentation-controls').hidden=!aug||!available;
 text('scenario-note',!available?'':stats?'Captured equipment stats':healer?'Preset healing score':aug?'You + simulated allies. Compare changes within this group.':'Stationary targets · Full raid buffs');
 text('run',stats?'Compare stats':healer?'Calculate healing score':aug?'Run group sim':'Run sim');
 const modelKey=selectedID+':'+selectedJobID;
 if($('augmentation-controls').dataset.snapshot!==modelKey){
  $('augmentation-controls').dataset.snapshot=modelKey;const scenario=latest?.scenario;
  $('augmentation-mode').value=scenario?.augmentationMode||'dungeon';$('ally-item-level').value=scenario?.allyItemLevel||328;$('ally-skill').value=Math.round((scenario?.allySkill||0.9)*100);
 }
 for(const id of ['targets','seconds','iterations','healing-mode','augmentation-mode','ally-item-level','ally-skill'])$(id).disabled=!available||!!active||busy;
 text('gear-status',available?'Choose up to 8 items.':blocked);
 text('vault-status',available?'Missing choices? Open the Vault in WoW, then export again.':blocked);
 text('healing-status',healer?(available?'Compare gear with the Raid or Dungeon preset. Results are healing scores.':blocked):'Select a healer snapshot to compare healing gear.');
 $('open-healing').disabled=!healer||!available;
 const talentsAvailable=available&&!healer&&!stats;
 $('run-talents').disabled=!talentsAvailable||!!active||busy;$('talent-code').disabled=!talentsAvailable||!!active||busy;
 text('talent-note',!available?blocked:healer?'This healer model uses preset talents.':stats?'Talent calculations are unavailable.':aug?'Compare builds with the same simulated allies.':'Compare against equipped gear.');
 $('compare-mode').disabled=stats||!available;const sets=$('compare-mode').querySelector('[value=topgear]');sets.disabled=stats||(!healer&&!summary?.itemRules);sets.textContent=sets.disabled&&!stats?'Complete sets (export again)':'Complete sets';if(sets.disabled)$('compare-mode').value='individual';
 const talents=$('talent-list');if(talents.dataset.snapshot!==selectedID){talents.dataset.snapshot=selectedID;talents.replaceChildren();for(const t of summary?.talents||[]){const label=node('label',undefined,'item'),input=node('input');input.type='checkbox';input.value=t.code;label.append(input,node('span',t.label||'Saved build'));talents.append(label);}}
 talents.querySelectorAll('input').forEach(input=>input.disabled=!talentsAvailable||!!active||busy);
 const healing=$('healing-result');healing.replaceChildren();if(latest?.result?.metric==='healing_score'){healing.append(node('p',number(latest.result.baseline.mean)+' healing score · '+latest.result.model));if(latest.result.comparisons[0]?.items)healing.append(itemTable(latest.result.comparisons[0].items));}
 $('find-upgrades').disabled=stats||!available||!!active||busy||(!healer&&!summary?.lootItems);$('upgrade-settings').hidden=!healer||!available;
 const raidNames={17:'Raid Finder',14:'Normal',15:'Heroic',16:'Mythic'},previews=summary?.lootPreviews||[];
 if($('upgrade-settings').dataset.snapshot!==selectedID){$('upgrade-settings').dataset.snapshot=selectedID;const raid=previews.find(p=>p.type==='Raid'),dungeon=previews.find(p=>p.type==='Dungeon');if(raid&&[17,14,15,16].includes(raid.difficultyID))$('raid-difficulty').value=[17,14,15,16].indexOf(raid.difficultyID);if(dungeon){const key=dungeon.keyLevel||0;const bucket=key>=10?7:key>=8?6:key>=7?5:key>=6?4:key>=5?3:key>=4?2:key>=2?1:0;$('key-level').value=bucket;}}
 const levels=[...new Set(previews.map(p=>p.type==='Raid'?(raidNames[p.difficultyID]||'Raid '+p.difficultyID):p.keyLevel?'M+'+p.keyLevel:'M0'))].join(' / '),complete=previews.filter(p=>p.complete).length;
 text('upgrade-status',!available?blocked:stats?'Combat-based upgrade rankings are unavailable.':summary?.lootItems?number(summary.lootItems)+' items · '+complete+' of '+summary.lootPools+' loot sources complete'+(levels?' · '+levels:''):'Export again in WoW to capture loot for this specialization.');
 renderPlanningInputs(summary);renderUpgrades();
}
function upgradeRun(){return state?.jobs.find(j=>j.id===selectedJobID&&j.snapshotId===selectedID&&j.result?.upgrades)||state?.jobs.find(j=>j.snapshotId===selectedID&&j.result?.upgrades);}
function renderPlanningInputs(summary){
 const select=$('roll-history'), currency=$('roll-currency'), run=upgradeRun();
 const identity=selectedID+':'+(run?.id||'');if(select.dataset.identity===identity)return;select.dataset.identity=identity;$('history-confirmed').checked=false;
 currency.replaceChildren();for(const c of summary?.planning?.currencies||[]){const o=node('option','Currency '+c.id+' · '+c.quantity+' available');o.value=c.id;currency.append(o);}if(!currency.options.length){const o=node('option','Balance unavailable');o.value='0';currency.append(o);}
 select.replaceChildren(node('option','No recorded pool'));select.options[0].value='';const seen=new Set();
 for(const r of summary?.planning?.rolls||[]){if(r.spec!==summary.snapshot.character.spec)continue;const sourceID=run?.result?.rollSources?.[String(r.source)];if(!sourceID)continue;const source=run.result.upgrades.find(i=>i.sourceID===sourceID)?.source||r.source;const key=[r.currency,r.source,r.context,r.keyLevel,r.spec].join(':');if(seen.has(key))continue;seen.add(key);const o=node('option',source+' · context '+r.context+(r.keyLevel?' · +'+r.keyLevel:'')+' · currency '+r.currency);o.value=JSON.stringify(r);select.append(o);}
}
function renderUpgrades(){
 const run=upgradeRun(),result=run?.result;
 const key=JSON.stringify([selectedID,current()?.capability.available,run?.id,run?.finishedAt,...['upgrade-sort','upgrade-view','drop-chance','run-minutes','reroll-chance','reroll-cost','reroll-exclude','roll-currency','roll-history','item-weights','bonus-pool-assumption','history-confirmed'].map(id=>$(id).type==='checkbox'?$(id).checked:$(id).value)]);
 if(upgradeSignature===key)return;upgradeSignature=key;
 ++planSequence;clearTimeout(planTimer);lastPlan=null;$('save-plan').hidden=true;
 for(const id of ['upgrade-views','planning-controls','target-title','target-set'])$(id).hidden=!result;
 if(!result){const summary=current(),note=!summary?.capability.available?unavailableMessage(summary):summary.capability.id==='forever-stats'?'Item-stat comparisons are available under Gear.':summary.lootItems||summary.capability.id==='qe'?'Choose Find upgrades to compare the available loot.':'No loot captured for this specialization yet.';$('upgrade-results').replaceChildren(node('p',note));$('target-set').replaceChildren();return;}
 const sort=$('upgrade-sort').value,types=new Set(result.upgrades.map(i=>i.type));
 for(const option of $('upgrade-view').options)option.disabled=!types.has(option.value);
 if(sort==='reroll')$('upgrade-view').value='bonus';else if(!types.has($('upgrade-view').value))$('upgrade-view').value=types.has('drop')?'drop':[...types][0];
 $('upgrade-view').disabled=sort==='reroll';const view=$('upgrade-view').value;
 const items=result.upgrades.filter(i=>i.type===view);
 const targetIDs=new Set((result.targetSet?.items||[]).map(i=>i.id));
 const holder=$('upgrade-results');holder.replaceChildren(node('p',result.model+' · '+validationName(result.validation)));if(result.metric==='group_dps')holder.append(node('p',groupDescription(run.scenario)+' · gains are a percentage of total Group DPS.','group-note'),node('p','You + fixed simulated allies. Compare changes within this group.','group-note'));appendRunNotes(holder,result);
 if(sort==='gain')holder.append(table(['Item','Source','Level','Gain now','Target'],[...items].sort((a,b)=>b.gain-a.gain).map(i=>[i.label+(i.craftedStats?' · '+i.craftedStats:''),i.source,i.level,percent(i.gain),targetIDs.has(i.id)?'Yes':''])));
 else {
  const output=node('div');holder.append(output);
  if(!types.has(view)){output.append(node('p','No '+view+' reward outcomes in this run.'));}
  else {
   output.append(node('p','Calculating plan…'));const sequence=planSequence;
   planTimer=setTimeout(async()=>{try{
    const weights={};for(const line of $('item-weights').value.split('\n').map(x=>x.trim()).filter(Boolean)){const parts=line.split('=');if(parts.length!==2||!parts[0]||!Number.isFinite(Number(parts[1])))throw Error('Use source/itemID=weight, one per line.');weights[parts[0]]=Number(parts[1]);}
    const options={mode:sort,view,chance:Number($(sort==='farm'?'drop-chance':'reroll-chance').value)/100,cost:Number($(sort==='farm'?'run-minutes':'reroll-cost').value),exclude:$('reroll-exclude').value.split(/[^0-9]+/).filter(Boolean).map(Number),currency:Number($('roll-currency').value),assumeBonusPool:$('bonus-pool-assumption').checked,historyConfirmed:$('history-confirmed').checked,weights};
    if(sort==='reroll'&&$('roll-history').value)options.history=JSON.parse($('roll-history').value);
    if(sort==='reroll'&&!options.assumeBonusPool)throw Error('Select the journal-pool assumption to estimate rerolls.');if(options.history&&!options.historyConfirmed)throw Error('Confirm that the recorded history matches this reward pool.');
    const plan=await api('plan',{id:run.id,options});if(sequence!==planSequence)return;
    output.replaceChildren();if(plan.balance!==undefined)output.append(node('p',plan.balance+' currency available · '+plan.affordable+' rolls at this cost'));
    output.append(table(['Source','Pool','Expected gain',sort==='farm'?'Gain / hour':'Gain / currency','Upgrade chance'],plan.sources.map(r=>[r.name,r.pool,percent(r.expected),percent(r.rate),percent(r.upgradeChance*100)])));
    if(!plan.sources.length)output.append(node('p','No complete, verified loot pools for this plan.'));
    const notes=node('details');notes.append(node('summary','Assumptions and data'));for(const warning of plan.warnings)notes.append(node('p',warning));
    notes.append(table(['Source ID','Item ID','Item'],items.filter((item,i)=>items.findIndex(other=>other.id===item.id&&other.sourceID===item.sourceID)===i).map(item=>[item.sourceID,item.id,item.label])));output.append(notes);
    lastPlan={...plan,result:run.result,snapshotId:run.snapshotId,scenario:run.scenario};$('save-plan').hidden=false;
   }catch(e){if(sequence===planSequence)output.replaceChildren(node('p',e.message));}},150);
  }
 }
 const target=$('target-set');target.replaceChildren();if(result.targetSet){target.append(node('p','Best tested set · '+percent((result.targetSet.mean/result.baseline.mean-1)*100)+' · '+result.searchTests+' comparisons'),itemTable(result.targetSet.items));}else target.append(node('p','No target set available.'));
}
$('roll-history').addEventListener('change',()=>{$('history-confirmed').checked=false;renderUpgrades();});
$('save-plan').addEventListener('click',()=>{if(!lastPlan)return;const url=URL.createObjectURL(new Blob([JSON.stringify(lastPlan,null,2)],{type:'application/json'}));const link=node('a');link.href=url;link.download='lamda-plan-'+lastPlan.jobID+'.json';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);});
$('save-review').addEventListener('click',async()=>{try{await api('review',{id:selectedID,notes:$('review-notes').value});$('review-notes').value='';state=await api('state');render();message('Review note saved.');}catch(e){message(e);}});
$('open-healing').addEventListener('click',()=>tab('overview'));
$('run-talents').addEventListener('click',()=>{const talents=[...$('talent-list').querySelectorAll('input:checked')].map(i=>i.value);const code=$('talent-code').value.trim();if(code)talents.push(code);if(!talents.length){message('Choose or paste a talent build.');return;}run(undefined,{talents});});
$('find-upgrades').addEventListener('click',()=>run(undefined,{task:'upgrades',keyLevel:Number($('key-level').value),raidDifficulty:Number($('raid-difficulty').value)}));
for(const id of ['upgrade-view','upgrade-sort','drop-chance','run-minutes','reroll-chance','reroll-cost','reroll-exclude','roll-currency','roll-history','item-weights','bonus-pool-assumption','history-confirmed'])$(id).addEventListener('input',renderUpgrades);
refresh();
