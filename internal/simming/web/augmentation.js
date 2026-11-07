'use strict';
(() => {
  const $ = id => document.getElementById(id);
  const spells = {'395152':'Ebon Might casts','409311':'Prescience casts','360827':'Blistering Scales casts','403631':'Breath of Eons casts','395160':'Eruption casts','396286':'Upheaval casts'};
  let state = {reports:[],live:[]};
  const records = () => [...(state.live || []).map(r=>({...r,live:true})), ...(state.reports || [])];
  const percent = r => 100*(r.ebonSeconds || 0)/Math.max(1,r.seconds);
  function choices(element, rows, fallback) {
    const previous=element.value;
    element.replaceChildren(...rows.map(([value,label]) => { const option=document.createElement('option');option.value=value;option.textContent=label;return option; }));
    element.value=rows.some(r=>r[0]===previous)?previous:(fallback || rows[0]?.[0] || '');
  }
  function render() {
    $('status').textContent=state.status || 'Waiting for combat log';
    const valid=state.lastEventAt && !state.lastEventAt.startsWith('0001-');
    const age=valid?Math.max(0,(Date.now()-Date.parse(state.lastEventAt))/1000):null;const offline=(state.status || '').startsWith('Offline');
    $('freshness').textContent=offline?'Offline log replay · live delivery latency was not measured':valid?`Last log event: ${new Date(state.lastEventAt).toLocaleString()} · Receipt delay ${Math.round(state.delaySeconds || 0)}s · Event age ${Math.round(age)}s`:'No log events received in this session.';
    const rows=records(), players=new Map(rows.map(r=>[r.guid,r.player || r.guid]));
    choices($('character'),[...players]);
    const mine=rows.filter(r=>r.guid===$('character').value);
    choices($('session'),mine.map(r=>[r.id,`${r.live?'In progress · ':''}${r.name} · ${new Date(r.startedAt).toLocaleString()}`]));
    const r=mine.find(r=>r.id===$('session').value);
    $('review').hidden=!r;$('empty').hidden=!!r;if(!r)return;
    $('name').textContent=r.name;
    const meter=(state.damageMeters || []).find(m=>m.owner===r.guid && m.kind===r.kind && m.startedAt===r.startedAt);
    $('damage-meter').hidden=!meter;
    if(meter){
      $('damage-status').textContent=!meter.valid?'Attribution incomplete — waiting for usable damage/ownership data.':meter.partial?`Partial log attribution — ${meter.rejectedHooks} invalid hooks excluded.`:'Damage reassigned from recorded support events. Blizzard hook omissions can still affect the split.';
      $('damage-bars').replaceChildren();$('damage-rows').replaceChildren();
      const maximum=Math.max(1,...meter.rows.map(row=>row.adjusted));
      for(const row of meter.rows){
        const own=row.guid===meter.owner,bar=document.createElement('div'),fill=document.createElement('div'),label=document.createElement('div');
        bar.className='damage-bar'+(own?' own':'');fill.className='damage-fill';fill.style.width=`${meter.valid?Math.max(0,row.adjusted)/maximum*100:0}%`;
        label.className='damage-label';const name=document.createElement('span'),value=document.createElement('span');name.textContent=row.name+(own?' (you)':'');value.textContent=meter.valid?`${Number(row.adjusted).toLocaleString()} · ${Math.round(row.dps).toLocaleString()} DPS`:'pending';label.append(name,value);bar.append(fill,label);$('damage-bars').append(bar);
        const tr=document.createElement('tr');if(own)tr.className='own';
        for(const value of [row.name,row.raw,row.removed,row.added,meter.valid?row.adjusted:'pending',meter.valid?Math.round(row.dps):'pending']){const td=document.createElement('td');td.textContent=typeof value==='number'?value.toLocaleString():value;tr.append(td);}$('damage-rows').append(tr);
      }
      $('damage-total').textContent=`Group total: ${Number(meter.rawTotal).toLocaleString()} recorded = ${Number(meter.adjustedTotal).toLocaleString()} reattributed. Unassigned pet damage: ${Number(meter.unassignedPetDamage).toLocaleString()}.`;
    }
    $('context').textContent=`${r.player || r.guid} · ${r.kind==='key'?'Whole key':'Boss attempt'} · ${Math.round(r.seconds)}s · ${r.live?'In progress':r.success?'Completed':'Ended / wipe'}`;
    $('ebon').textContent=`${percent(r).toFixed(1)}%`;
    $('prescience').textContent=((r.prescienceTargetSeconds || 0)/Math.max(1,r.seconds)).toFixed(2);
    $('eons').textContent=r.casts?.['403631'] || 0;
    $('damage').textContent=Number(r.recordedSupportDamage || 0).toLocaleString();
    $('detail').replaceChildren();
    const details=[...Object.entries(spells).map(([id,label])=>[label,r.casts?.[id] || 0]),['Deaths',r.deaths || 0],['Blistering Scales average targets',((r.scalesTargetSeconds || 0)/Math.max(1,r.seconds)).toFixed(2)],['Other recorded support events',r.unknownSupportEvents || 0]];
    for(const [label,value] of details){const dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=label;dd.textContent=value;$('detail').append(dt,dd);}
    $('note').textContent=r.note;
    const old=mine.find(p=>!p.live && p.id!==r.id && p.name===r.name && p.kind===r.kind && Date.parse(p.startedAt)<Date.parse(r.startedAt));
    $('comparison').textContent=old?`Previous matching session: Ebon ${percent(old).toFixed(1)}%, Prescience ${((old.prescienceTargetSeconds || 0)/Math.max(1,old.seconds)).toFixed(2)} targets. Compare fight length, downtime and missing records before judging progress.`:'';
  }
  async function poll(){
    try{const response=await fetch('/api/augmentation',{signal:AbortSignal.timeout(6000)});if(!response.ok)throw new Error('unavailable');state=await response.json();render();}
    catch(_){$('status').textContent='Engine connection unavailable';$('freshness').textContent='Displayed reports are from the last successful update.';}
    finally{setTimeout(poll,2000);}
  }
  $('character').addEventListener('change',render);$('session').addEventListener('change',render);poll();
})();
