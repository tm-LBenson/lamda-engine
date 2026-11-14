/* Shared, dependency-free manager rule model. No game input or external I/O. */
(function (root) {
  'use strict';
  const limits=root.OverlayLimits||(typeof require==='function'?require('./limits.js'):null);
  const Actions=root.OverlayActions||(typeof require==='function'?require('./actions.js'):null);
  const Packs=root.OverlayPacks||(typeof require==='function'?require('./packs.js'):null);
  const Values=root.OverlayValues||(typeof require==='function'?require('./values.js'):null);
  const Motion=root.OverlayMotion||(typeof require==='function'?require('./motion.js'):null);
  let serial = 0;
  const id = () => 'b' + Date.now().toString(36) + (++serial).toString(36) + Math.random().toString(36).slice(2, 6);
  const clone = value => JSON.parse(JSON.stringify(value));
  const resources = {primary:'Current primary',0:'Mana',1:'Rage',2:'Focus',3:'Energy',4:'Combo points',5:'Runes',6:'Runic power',7:'Soul shards',8:'Astral power',9:'Holy power',10:'Alternate power',11:'Maelstrom',12:'Chi',13:'Insanity',14:'Burning embers',15:'Demonic fury',16:'Arcane charges',17:'Fury',18:'Pain',19:'Essence',20:'Blood runes',21:'Frost runes',22:'Unholy runes',23:'Quest power',24:'Encounter power',25:'Vigor / mount power',26:'Balance',27:'Happiness',28:'Shadow orbs',29:'Chromatic runes'};
  const palette={gold:'#F4C879',teal:'#65DDC7',violet:'#C6A5EE',red:'#EF9B9B'};
  const colorValue=value=>palette[value]||(/^#[a-fA-F0-9]{6}$/.test(value)?value.toUpperCase():undefined);
  const compare = (a,b,op='lt') => ({lt:a<b,lte:a<=b,eq:a===b,ne:a!==b,gt:a>b,gte:a>=b})[op] ?? false;
  const definitions = {
    action: {title:'ACTION',caption:'A reusable sequence',family:'timing',container:true,help:'Define a named sequence here, then use CALL inside WHEN or IF. Changes apply to every call in this overlay.',defaults:{name:'My action'}},
    call: {title:'CALL',caption:'Run a saved action',family:'action',help:'Run a named sequence using this caller’s conditions, event and unit. Each call has its own display and timing.',defaults:{action:''}},
    when: { title: 'WHEN', caption: 'Something happens', family: 'when', container: true, help: 'Starts when this event happens. Nested WHEN blocks watch for later events. Leaving an instance or resetting the test clears its watchers.', defaults: { event: 'state' } },
    if: { title: 'IF', caption: 'Health, resources, and buffs', family: 'condition', container: true, help: 'Only run the blocks inside when this condition is true. Check a buff or debuff by spell ID: active, missing, stacks, or time left. Nest IF blocks to require both conditions. Cooldowns support spells, items, equipped slots and the global cooldown. Ready ignores the global cooldown for spells; Usable also checks resources and spell requirements, but does not guarantee range or a valid target. Charges and recharge time are separate from normal cooldown time. Save and reload after adding a new watched cooldown.', defaults: { condition: 'role', value: 'tank' } },
    each: { title: 'FOR EACH', caption: 'Repeat for units', family: 'condition', container: true, help: 'Run the blocks once for each unit in the selected group. Current unit follows that unit; explicit unit choices keep their own target.', defaults: {scope:'party'} },
    show: { title: 'SHOW', caption: 'An icon, text, or bar', family: 'action', help: 'Choose a timer or While conditions are true. The latter stays visible after its WHEN fires until any enclosing IF stops matching. With no IF, it stays until hidden or unloaded. Use Appearance to style it, then drag it in the preview to move it.', defaults: { display: 'icon', text: 'Defensive used', duration: 6, visibility: 'timed' } },
    glow: { title: 'GLOW', caption: 'Make it stand out', family: 'action', help: 'Add a glow to the display for this overlay. Place SHOW before GLOW to give it a display to change.', defaults: { color: 'gold' } },
    sound: { title: 'PLAY', caption: 'A notification sound', family: 'action', help: 'Play one short sound. Test audio stays off until you enable it in Test scene.', defaults: { sound: 'chime' } },
    wait: { title: 'WAIT', caption: 'Pause before the next step', family: 'timing', help: 'Delay the blocks that follow in this container. A new test cancels delays from the previous test.', defaults: { seconds: 1 } },
    hide: { title: 'HIDE', caption: 'Clear this display', family: 'action', help: 'Hide this overlay. Place a WAIT before it if the display should stay visible for a while.', defaults: {} }
  };
  function node(type, props = {}, children = []) {
    if (!definitions[type]) throw Error('Unknown block type.');
    const n = { id: id(), type, props: { ...definitions[type].defaults, ...props } };
    if (definitions[type].container) n.children = children;
    return n;
  }
  function newRule(name = 'My new overlay', roots = [node('when')]) {
    return { id: id(), name, enabled: true, roots, load: {}, style: { size: 46, opacity: 100, color: 'gold', font: 14, transparent: true, x: 48, y: 40 } };
  }
  function auraExample() {
    const aura = { condition: 'buff', aura: '124273', unit:'player' };
    return newRule('Buff stack reminder', [node('when', { event: 'aura' }, [
      node('if', { ...aura, check: 'stacks', operator: 'gte', value: 3 }, [
        node('if', { ...aura, check: 'remaining', operator: 'lt', value: 5 }, [
          node('show', { display: 'text', text: 'Use your stacks!', duration: 5 }), node('glow')
        ])
      ])
    ])]);
  }
  function manaExample() {
    const r = newRule('Low mana alert', [node('when', { event: 'resource' }, [
      node('if', { condition: 'resource', resource: '0', operator: 'lt', measure: 'percent', value: 20 }, [
        node('show', { display: 'text', text: 'Low mana!', duration: 6, visibility: 'conditions' }),
        node('sound', { sound: 'alert' })
      ])
    ])]);
    r.style.font = 24; r.style.color = 'teal'; r.load.alive = 'yes';
    return r;
  }
  function capExample() {
    const r=manaExample();r.name='Resource cap alert';const c=r.roots[0].children[0];
    c.props={condition:'resource',resource:'primary',operator:'cap',measure:'amount',value:0};
    c.children[0].props.text='Resource capped!';return r;
  }
  function liveExample(kind='resource') {
    if(kind==='buff')return newRule('Buff timer',[node('when',{event:'state'},[node('if',{condition:'buff',aura:'436336',unit:'player',check:'active'},[node('show',{display:'bar',text:'{name} · {remaining}s',visibility:'conditions',fill:'source'})])])]);
    return newRule('Resource bar',[node('when',{event:'state'},[node('show',{display:'bar',text:'{name} {current}/{max}',visibility:'conditions',fill:'source',dataSource:'resource',dataResource:'primary'})])]);
  }
  function logicExample(){const r=manaExample();r.name='Health or mana';r.roots[0].props.event='state';const n=r.roots[0].children[0];n.props={condition:'group',mode:'any',tests:[{condition:'health',operator:'lt',value:35},n.props]};n.children[0].props.text='Health or mana is low';return r;}
  function nestedExample() {
    const r = auraExample();
    r.name = 'Instance buff reminder';
    r.roots = [node('when', { event: 'enter' }, r.roots)];
    return r;
  }
  function examples() {
    return [
      newRule('Tank defensive', [node('when', {}, [node('if', {}, [node('show'), node('glow'), node('sound')])])]),
      newRule('Low health warning', [node('when', { event: 'health' }, [node('if', { condition: 'health', value: 35 }, [node('show', { display: 'text', text: 'Health is low!', duration: 5 }), node('glow', { color: 'red' })])])]),
      newRule('Dungeon reminder', [node('when', { event: 'enter' }, [node('if', { condition: 'location', value: 'dungeon' }, [node('show', { display: 'text', text: 'Check your consumables', duration: 8 })])])]),
      auraExample()
    ];
  }
  function find(roots, target, parent = null) {
    for (let index = 0; index < roots.length; index++) {
      const n = roots[index];
      if (n.id === target) return { node: n, list: roots, index, parent };
      if (n.children) { const hit = find(n.children, target, n); if (hit) return hit; }
    }
    return null;
  }
  function flatten(roots) { return roots.flatMap(n => [n, ...flatten(n.children || [])]); }
  function copyNode(n) { const result = clone(n); for (const c of flatten([result])) c.id = id(); return result; }
  function callChoices(rule,nodeID){let hit=find(rule.roots,nodeID);while(hit&&hit.node.type!=='action')hit=hit.parent?find(rule.roots,hit.parent.id):null;return Actions.targets(rule,hit?.node.id);}
  function insert(rule, spec, parentId, index) {
    const destination = parentId ? find(rule.roots, parentId) : null;
    if (parentId && (!destination || !destination.node.children)) throw Error('Choose a container for this block.');
    const list = destination ? destination.node.children : rule.roots;
    const source = spec.id ? find(rule.roots, spec.id) : null;
    if (spec.id && !source) throw Error('That block is no longer available.');
    const n = source ? source.node : node(spec.type);
    if(!source&&n.type==='call'){const def=callChoices(rule,parentId)[0];if(!def)throw Error('Add another ACTION first.');n.props.action=def.id;}
    if (n.type==='action'&&parentId)throw Error('Declare an ACTION at the top level.');
    if(!source&&n.type==='action'){const names=new Set(Actions.definitions(rule).map(d=>d.props.name));let i=2;while(names.has(n.props.name))n.props.name='My action '+i++;}
    if (!['when','action'].includes(n.type) && !parentId) throw Error('Drop this block inside a WHEN, IF, or FOR EACH opening.');
    if (source && parentId && flatten([n]).some(child => child.id === parentId)) throw Error('A block cannot go inside itself.');
    if (!source && flatten(rule.roots).length >= 80) throw Error('This release supports up to 80 blocks per overlay.');
    let depth = 0, ancestor = destination;
    while (ancestor) { depth++; ancestor = ancestor.parent ? find(rule.roots, ancestor.parent.id) : null; }
    function height(x) { return 1 + Math.max(0, ...(x.children || []).map(height)); }
    if (depth + height(n) > 8) throw Error('Keep nesting to eight levels or fewer.');
    let at = Math.max(0, Math.min(Number(index) || 0, list.length));
    if (source) { if (source.list === list && source.index < at) at--; source.list.splice(source.index, 1); }
    list.splice(at, 0, n);
    return n.id;
  }
  function validUnit(unit){return [undefined,'','current'].includes(unit)||Object.hasOwn(anchorUnits,unit);}
  function unitContext(sample,unit){
    if(unit==='current'&&sample.cloneActor)return sample;
    const root=sample.playerContext||sample,wanted=unit==='current'?(sample.unit||'player'):unit||'player';
    const member={...(root.units||root.party)?.find(m=>m.unit===wanted),...root.unitSamples?.[wanted]},player=wanted==='player';
    return {...root,...member,unit:wanted,name:member?.name||(player?root.name:undefined),role:member?.role||(player?root.role:null),health:member?.health??(player?root.health:null),resources:member?.resources??(player?root.resources:null),primaryPower:member?.primaryPower??(player?root.primaryPower:null),auras:(root.auraUnit||'player')===wanted?root.auras:member?.auras,auraUnit:wanted};
  }
  function predicateList(p){return [p,...(p.condition==='group'&&Array.isArray(p.tests)?p.tests.flatMap(predicateList):[])];}
  function condition(n,context){
    if(n.props.condition==='value')return Values.condition(n.props,valueEvaluation(context.overlayLogic,context));
    const p=n.props;if(p.condition!=='group')return conditionLeaf(n,context);
    if(!p.tests?.length)return null;
    const values=p.tests.map(props=>condition({props},context));
    if(p.mode==='not')return values[0]===null?null:!values[0];
    if(p.mode==='all'&&values.includes(false))return false;
    if(p.mode==='any'&&values.includes(true))return true;
    if(values.includes(null))return null;
    return p.mode==='all';
  }
  function conditionLeaf(n, context) {
    if(['bossTimer','bossStage'].includes(n.props.condition))return bossCondition(n.props,context);
    if(n.props.condition==='encounter')return encounterCondition(n.props,context);
    if(n.props.condition==='pet')return petCondition(n.props,context);
    if(n.props.condition==='eventValue'){const v=context.combatEvent?.values?.[n.props.field];return Number.isFinite(v)?compare(v,Number(n.props.value),n.props.operator):null;}
    if(n.props.condition==='casting')return castCondition(n.props,context);
    if(n.props.condition==='equipment')return equipmentCondition(n.props,context);
    if (['talent','learned'].includes(n.props.condition))return talentCondition(n.props,context);
    if (n.props.condition === 'cooldown') return cooldownCondition(n.props, context);
    if (n.props.condition === 'role') return ['tank','healer','damage'].includes(context.role)?context.role===n.props.value:null;
    if (n.props.condition === 'location') return ['world','dungeon','raid','arena','pvp'].includes(context.location)?context.location===n.props.value:null;
    if (n.props.condition === 'health') {context=unitContext(context,n.props.unit||'current');return Number.isFinite(context.health)&&context.health>=0&&context.health<=100?compare(context.health,Number(n.props.value),n.props.operator):null;}
    if (n.props.condition === 'resource') {
      context=unitContext(context,n.props.unit||'current');
      const p=n.props,key=p.resource==='primary'?context.primaryPower:p.resource,v=context.resources?.[key];
      if(!v || !Number.isFinite(v.current) || !Number.isFinite(v.max) || v.max<=0 || v.current<0 || v.current>v.max)return null;
      return p.operator==='cap'?v.current>=v.max:compare(p.measure==='percent'?100*v.current/v.max:v.current,Number(p.value),p.operator);
    }
    if (['buff', 'debuff'].includes(n.props.condition)) {
      const p = n.props;
      context=unitContext(context,p.unit||'player');const wanted=p.unit==='current'?context.unit||'player':p.unit||'player';
      // An absent snapshot is unknown, not proof that the aura is missing.
      if (!Array.isArray(context.auras)||wanted!==(context.auraUnit||'player')) return null;
      const aura = context.auras.find(a => a.kind === p.condition && typeof a.name === 'string' && a.name.trim().toLowerCase() === p.aura.trim().toLowerCase());
      if (p.check === 'missing') return !aura;
      if (p.check === 'active') return !!aura;
      if (!aura) return false;
      if (p.check === 'remaining' && aura.permanent === true) return null;
      const value = p.check === 'stacks' ? aura.stacks : aura.remaining;
      if (!Number.isFinite(value) || value < 0) return null;
      const threshold = Number(p.value);
      return { eq: value === threshold, ne: value !== threshold, lt: value < threshold, lte: value <= threshold, gt: value > threshold, gte: value >= threshold }[p.operator] ?? false;
    }
    return false;
  }
  function displayState(rule, blockId, sample) {
    rule=Actions.executionRule(rule);
    sample={...sample,overlayLogic:rule.logic,overlayMemory:sample.overlayMemories?.get(rule.id)||sample.overlayMemory};
    function walk(nodes, checks, context) {
      for(const n of nodes) {
        if(n.id===blockId){const values=checks.map(c=>condition(c.node,c.context));return values.some(v=>v===null)?null:values.every(v=>v===true);}
        if(n.children){const next=n.type==='each'?sample:context,matched=walk(n.children,n.type==='if'?[...checks,{node:n,context}]:checks,next);if(matched!==undefined)return matched;}
      }
    }
    return rule.enabled ? walk(rule.roots,[],(sample.playerContext?{...sample.playerContext,overlayLogic:rule.logic,overlayMemory:sample.overlayMemory,combatEvent:sample.combatEvent,encounterEvent:sample.encounterEvent,bossEvent:sample.bossEvent}:sample))??null:false;
  }
  function displayMatches(rule,blockId,sample){return displayState(rule,blockId,sample)===true;}
  function createSession() { return { ruleId: null, active: new Map(), location: null, memory:Values.memoryState() }; }
  function evaluate(rule, sample, session = createSession()) {
    rule=Actions.executionRule(rule);
    sample={...sample,overlayLogic:rule.logic,overlayMemory:sample.overlayMemories?.get(rule.id)||sample.overlayMemory};
    const trace = [], actions = [], waiting = [];
    const now = Number.isFinite(sample.now) ? sample.now : Date.now();
    const playerEvent = event => ['ability', 'health', 'resource', 'aura', 'cooldown'].includes(event);
    function removePath(path) {
      for (const [key, record] of session.active) if (path.every((p, i) => record.path[i] === p)) session.active.delete(key);
    }
    if (session.ruleId !== rule.id || (session.location !== null && session.location !== sample.location) || ['enter', 'leave'].includes(sample.event)) {session.active.clear();session.memory=Values.memoryState();}
    session.ruleId = rule.id; session.location = sample.location;
    if (['key', 'keyend'].includes(sample.event)) for (const record of [...session.active.values()]) if (record.event === 'key') removePath(record.path);
    if (!rule.enabled) { session.active.clear(); session.memory=Values.memoryState(); return { trace, actions, waiting, reason: 'This overlay is disabled.' }; }
    session.memory||=Values.memoryState();sample.overlayMemory=session.memory;
    sample.overlayMemories?.set(rule.id,session.memory);
    for(const record of [...session.active.values()])if(record.actor&&record.actor!==sample.actor&&!cloneContext(sample,record.actor))removePath(record.path);
    Values.remember(rule,sample.event,session.memory,{matches:p=>triggerMatches(p,sample,sample),check:p=>condition({props:p},sample),evaluation:()=>valueEvaluation(rule.logic,sample)});
    function actorContext(context, actor) {
      if (!actor || actor === context.actor) return { ...context, boundActor: actor || context.boundActor };
      return cloneContext(sample,actor)||{...sample,actor,boundActor:actor,role:null,health:null,resources:null,auras:null};
    }
    function run(nodes, context, offset, execute, path, since, triggerContext=null) {
      context=triggerSample(context,triggerContext);
      let elapsed = offset;
      for (const n of nodes) {
        if(n.type==='sequence'){trace.push({id:n.sourceID||n.id,matched:true,actor:context.actor});elapsed=run(n.children,context,elapsed,execute,path,since,triggerContext);}
        else if (n.type === 'when') {
          const scopedActor = context.boundActor || (playerEvent(n.props.event) ? context.actor : null);
          const key = n.id + '|' + (scopedActor || '*');
          const ownPath = [...path, key];
          const fired = !execute && (!session.active.has(key)||['talents','equipment'].includes(sample.event)||Object.hasOwn(castEvents,sample.event)||sample.event==='combatlog'||Object.hasOwn(encounterEvents,sample.event)||Object.hasOwn(bossEvents,sample.event)) && triggerMatches(n.props,sample,context) && now >= since + elapsed && sample.event === n.props.event && (!['enter', 'key'].includes(n.props.event) || ['dungeon', 'raid'].includes(sample.location));
          if (fired) {
            removePath(ownPath);
            if (flatten(n.children).some(child => child.type === 'when')) session.active.set(key, { id:n.id, event:n.props.event, actor:scopedActor, path:ownPath, started:now,triggerContext:eventContext(sample)||triggerContext });
            trace.push({ id:n.sourceID||n.id, matched:true, actor:scopedActor });
            run(n.children, actorContext(context, scopedActor), 0, true, ownPath, now,eventContext(sample)||triggerContext);
          }
          const records = [...session.active.values()].filter(r => r.id === n.id && (!context.boundActor || r.actor === context.boundActor) && r.path.length === path.length + 1 && path.every((p,i) => r.path[i] === p));
          for (const record of records) {
            if (fired && record.path.join('/') === ownPath.join('/')) continue;
            trace.push({ id:n.sourceID||n.id, matched:true, watching:true, actor:record.actor });
            run(n.children, actorContext(context, record.actor), 0, false, record.path, record.started,record.triggerContext);
          }
          if (!fired && !records.length) {
            trace.push({ id:n.sourceID||n.id, matched:false, waiting:path.length > 0 });
            if (path.length) waiting.push({ id:n.id, event:n.props.event });
          }
        } else if (n.type === 'if') {
          const matched = condition(n, context);
          trace.push({ id: n.sourceID||n.id, matched, actor: context.actor });
          if (matched) elapsed = run(n.children, context, elapsed, execute, path, since,triggerContext);
          else {
            const ids = new Set(flatten(n.children).map(child => child.id));
            for (const record of [...session.active.values()]) if (ids.has(record.id) && (!context.boundActor || record.actor === context.boundActor) && path.every((p,i) => record.path[i] === p)) removePath(record.path);
          }
        } else if (n.type === 'each') {
          trace.push({ id: n.sourceID||n.id, matched: true });
          const ends = cloneMembers(sample,n.props.scope||'party').map(member => run(n.children, cloneContext(sample,member.actor), elapsed, execute, path, since,triggerContext));
          elapsed = Math.max(elapsed, ...ends);
        } else if (n.type === 'wait') {
          trace.push({ id: n.sourceID||n.id, matched: true });
          elapsed += Math.max(0, Math.min(15, Number(n.props.seconds) || 0)) * 1000;
        } else if (execute) {
          trace.push({ id: n.sourceID||n.id, matched: true, actor: context.actor });
          actions.push({ triggerContext,cloneActor:context.cloneActor||null,type: n.type, props: clone(n.props), actor: context.actor, role: context.role, at: elapsed, blockId: n.id });
        }
      }
      return elapsed;
    }
    run(rule.roots, sample, 0, false, [], now);
    actions.sort((a, b) => a.at - b.at);
    const reason = !rule.roots.length ? 'Add a WHEN block to start this overlay.' : !trace.some(t => t.matched) ? 'The test event does not match this WHEN block.' : !actions.length ? trace.some(t => t.matched === null) ? 'No actions ran. Required data is unknown.' : waiting.length ? 'Waiting for the next event.' : 'No actions ran. Check the condition marked in amber, or add an action.' : '';
    return { trace, actions, waiting, reason };
  }
  const combatKinds={damage:'Damage',heal:'Healing',interrupt:'Interrupt',dispel:'Dispel / steal',dispelfailed:'Dispel fails',auraapply:'Aura applied',auraremove:'Aura removed',aurarefresh:'Aura refreshed',aurastacks:'Aura stacks change',logcaststart:'Cast starts (log)',logcastsuccess:'Cast succeeds (log)',logcastfailed:'Cast fails (log)',energize:'Resource gained',miss:'Miss / avoid',death:'Death',despawn:'Unit removed',summon:'Summon',resurrect:'Resurrect'};
  const combatValues={amount:'Amount',effective:'Effective amount',baseAmount:'Base amount',absorbed:'Absorbed',overheal:'Overheal',overkill:'Overkill',blocked:'Blocked',resisted:'Resisted',stacks:'Stacks',wasted:'Wasted resource',power:'Resource type',critical:'Critical'};
  function eventContext(sample){return ['talents','equipment'].includes(sample.event)?{loadout:sample.event}:Object.hasOwn(bossEvents,sample.event)&&sample.bossEvent?{boss:sample.bossEvent}:Object.hasOwn(encounterEvents,sample.event)&&sample.encounterEvent?{encounter:sample.encounterEvent}:sample.event==='combatlog'&&sample.combatEvent?{combat:sample.combatEvent}:Object.hasOwn(castEvents,sample.event)?sample.castEvent:null;}
  function triggerSample(sample,trigger){if(!trigger)return sample;const data={combatEvent:trigger.combat||null,encounterEvent:trigger.encounter||null,bossEvent:trigger.boss||null,castEvent:trigger.combat||trigger.encounter||trigger.boss||trigger.loadout?null:trigger};return {...sample,...data,...(sample.playerContext?{playerContext:{...sample.playerContext,...data}}:{})};}
  function validWhen(p){return p&&['talents','equipment','ability','health','resource','enter','key','aura','leave','keyend','state','login','combat','combatend','cooldown','casting','combatlog',...Object.keys(castEvents),...Object.keys(encounterEvents),...Object.keys(bossEvents)].includes(p.event)&&(!Object.hasOwn(castEvents,p.event)&&p.event!=='casting'||validUnit(p.unit)&&validCastSpell(p.spell))&&(!Object.hasOwn(bossEvents,p.event)||validBossFields(p))&&(!Object.hasOwn(encounterEvents,p.event)||validEncounterWhen(p))&&(p.event!=='combatlog'||validCombatWhen(p));}
  function validEventValue(p){return p.condition==='eventValue'&&Object.hasOwn(combatValues,p.field)&&['lt','lte','eq','ne','gt','gte'].includes(p.operator)&&Number.isFinite(p.value)&&p.value>=0&&p.value<=1e12;}
  function validCombatWhen(p){
    const unit=u=>[undefined,'','any'].includes(u)||validUnit(u),any=v=>[undefined,'','any'].includes(v);
    return Object.hasOwn(combatKinds,p.combatEvent)&&unit(p.sourceUnit)&&unit(p.targetUnit)&&validCastSpell(p.spell)&&validCastSpell(p.extraSpell)&&['critical','periodic'].every(k=>any(p[k])||['yes','no'].includes(p[k]))&&(any(p.auraKind)||['buff','debuff'].includes(p.auraKind))&&(any(p.miss)||['ABSORB','BLOCK','DEFLECT','DODGE','EVADE','IMMUNE','MISS','PARRY','REFLECT','RESIST'].includes(p.miss))&&(any(p.school)||typeof p.school==='string'&&/^[1-9][0-9]{0,2}$/.test(p.school)&&+p.school<=127);
  }
  function combatCount(rules){rules=Actions.rules(rules);
    const units=new Set();function walk(ns,repeated){for(const n of ns){if(n.type==='when'&&n.props.event==='combatlog'){units.add('player');for(const k of ['sourceUnit','targetUnit'])if(![undefined,'','any'].includes(n.props[k]))for(const u of cloneUnits(repeated,n.props[k]))units.add(u);}walk(n.children||[],childScope(n,repeated));}}
    for(const r of rules)if(r.enabled){walk(r.roots,0);walk(Values.inputs(r),ruleScopes(r.roots)|playerScope);}return units.size;
  }
  function triggerMatches(p,sample,context){
    if(Object.hasOwn(bossEvents,p.event))return bossEventMatches(p,sample);
    if(Object.hasOwn(encounterEvents,p.event))return encounterMatches(p,sample.encounterEvent);
    if(p.event!=='combatlog')return castEventMatches(p,sample,context);
    const e=sample.combatEvent;if(!e||e.kind!==p.combatEvent)return false;
    for(const k of ['source','target']){let unit=p[k+'Unit'];if(![undefined,'','any'].includes(unit)){if(unit==='current')unit=context.unit||'player';const id=(sample.identities||{})[unit];if(!id||id!==e[k]?.id)return false;}}
    if(p.spell&&+p.spell!==e.spell||p.extraSpell&&+p.extraSpell!==e.extraSpell)return false;
    if(![undefined,'','any'].includes(p.school)&&((e.school||0)&+p.school)!==+p.school)return false;
    if(![undefined,'','any'].includes(p.critical)&&(typeof e.critical!=='boolean'||e.critical!==(p.critical==='yes')))return false;
    if(![undefined,'','any'].includes(p.periodic)&&!!e.periodic!==(p.periodic==='yes'))return false;
    if(![undefined,'','any'].includes(p.auraKind)&&e.auraKind!==p.auraKind||![undefined,'','any'].includes(p.miss)&&e.miss!==p.miss)return false;
    return true;
  }
  function combatExample(){return newRule('My interrupt',[node('when',{event:'combatlog',combatEvent:'interrupt',sourceUnit:'player',targetUnit:'any'},[node('show',{display:'text',text:'Interrupted {extraName}',duration:4}),node('sound',{sound:'chime'})])]);}
  function buildLoadMatches(rule,sample){const p=rule.load?.conditions;return !p||p.condition==='group'&&p.tests.length===0||condition({props:p},sample)===true;}
  const equipmentKinds={item:'Item',slot:'Slot',set:'Item set',saved:'Saved outfit',type:'Item type'};
  const equipmentSlots={1:'Head',2:'Neck',3:'Shoulders',4:'Shirt',5:'Chest',6:'Waist',7:'Legs',8:'Feet',9:'Wrists',10:'Hands',11:'First ring',12:'Second ring',13:'Upper trinket',14:'Lower trinket',15:'Back',16:'Main hand',17:'Off hand',18:'Ranged',19:'Tabard'};
  const equipmentTypes={2:{'-1':'Any weapon',0:'One-handed axe',1:'Two-handed axe',2:'Bow',3:'Gun',4:'One-handed mace',5:'Two-handed mace',6:'Polearm',7:'One-handed sword',8:'Two-handed sword',9:'Warglaive',10:'Staff',13:'Fist weapon',15:'Dagger',16:'Thrown',18:'Crossbow',19:'Wand',20:'Fishing pole'},4:{'-1':'Any armor',0:'Miscellaneous',1:'Cloth',2:'Leather',3:'Mail',4:'Plate',6:'Shield',7:'Libram',8:'Idol',9:'Totem',10:'Sigil',11:'Relic'}};
  function validEquipment(p){
    if(p.condition!=='equipment'||!Object.hasOwn(equipmentKinds,p.source)||!Number.isInteger(p.slot??0)||(p.slot??0)<0||(p.slot??0)>19)return false;
    const id=typeof p.id==='string'&&/^[1-9][0-9]{0,6}$/.test(p.id),comparison=['lt','lte','eq','ne','gt','gte'].includes(p.operator),presence=['equipped','missing'].includes(p.check);
    switch(p.source){
      case 'item':return id&&presence;
      case 'slot':return p.slot>0&&(['occupied','empty'].includes(p.check)||p.check==='level'&&comparison&&Number.isFinite(p.value)&&p.value>=0&&p.value<=10000);
      case 'set':return id&&p.check==='count'&&comparison&&Number.isInteger(p.value)&&p.value>=0&&p.value<=19;
      case 'saved':return typeof p.name==='string'&&!!p.name.trim()&&new TextEncoder().encode(p.name).length<=120&&!/[\r\n\0]/.test(p.name)&&presence;
      case 'type':return Number.isInteger(p.itemClass)&&p.itemClass>=0&&p.itemClass<=100&&Number.isInteger(p.itemSubclass)&&p.itemSubclass>=-1&&p.itemSubclass<=100&&presence;
    }return false;
  }
  function equipmentSetNames(rules){rules=Actions.rules(rules);const result=new Set();for(const r of rules)if(r.enabled){const tests=[...(r.load?.conditions?[r.load.conditions]:[]),...flatten([...r.roots,...Values.inputs(r)]).filter(n=>n.type==='if').map(n=>n.props),...(r.style?.modifiers||[]).map(m=>m.condition)];for(const p of tests)for(const q of predicateList(p))if(q.condition==='equipment'&&q.source==='saved')result.add(q.name);}return [...result];}
  function equipmentCondition(p,sample){
    const gear=(sample.playerContext||sample).equipment;if(!gear)return null;
    if(p.source==='saved'){const s=gear.sets?.find(s=>s.name===p.name);return typeof s?.equipped==='boolean'?s.equipped===(p.check==='equipped'):null;}
    let count=0,uncertain=false;const start=p.slot>0&&p.source!=='set'?p.slot:1,end=p.slot>0&&p.source!=='set'?p.slot:19;
    for(let i=start;i<=end;i++){
      const s=gear.slots?.find(s=>s.slot===i);if(!Number.isInteger(s?.item)){uncertain=true;continue;}
      if(p.source==='slot'){if(p.check==='occupied')return s.item>0;if(p.check==='empty')return s.item===0;if(s.item===0)return false;return Number.isFinite(s.level)?compare(s.level,p.value,p.operator):null;}
      if(s.item===0)continue;let match=false;
      if(p.source==='item')match=s.item===+p.id;
      if(p.source==='set'){if(!Number.isInteger(s.set)){uncertain=true;continue;}match=s.set===+p.id;}
      if(p.source==='type'){if(!Number.isInteger(s.class)){uncertain=true;continue;}if(s.class!==p.itemClass)continue;if(p.itemSubclass>=0){if(!Number.isInteger(s.subclass)){uncertain=true;continue;}match=s.subclass===p.itemSubclass;}else match=true;}
      if(match){count++;if(p.source!=='set')return p.check==='equipped';}
    }
    if(uncertain)return null;return p.source==='set'?compare(count,p.value,p.operator):p.check==='missing';
  }
  const talentKinds={spell:'Talent',pvp:'PvP talent',hero:'Hero tree',entry:'Talent entry'};
  function talentWatch(p){return {kind:p.condition==='learned'?'known':p.source,id:+p.id};}
  function validTalent(p){if(!['talent','learned'].includes(p.condition)||typeof p.id!=='string'||!/^[1-9][0-9]{0,6}$/.test(p.id)||p.condition==='talent'&&!Object.hasOwn(talentKinds,p.source))return false;return ['selected','missing'].includes(p.check)||p.condition==='talent'&&p.check==='rank'&&['lt','lte','eq','ne','gt','gte'].includes(p.operator)&&Number.isInteger(p.value)&&p.value>=0&&p.value<=100;}
  function talentReading(w,sample){return (sample.playerContext||sample).talents?.find(t=>t.kind===w.kind&&t.id===w.id);}
  function talentCondition(p,sample){const t=talentReading(talentWatch(p),sample);if(!Number.isInteger(t?.rank))return null;return p.check==='selected'?t.rank>0:p.check==='missing'?t.rank===0:compare(t.rank,p.value,p.operator);}
  function talentWatches(rules){rules=Actions.rules(rules);const result=new Map(),add=p=>{for(const q of predicateList(p)){if(['talent','learned'].includes(q.condition)){const w=talentWatch(q);result.set(w.kind+':'+w.id,w);}}};for(const r of rules)if(r.enabled){if(r.load?.conditions)add(r.load.conditions);for(const n of flatten([...r.roots,...Values.inputs(r)])){if(n.type==='if')add(n.props);if(n.type==='show'&&['talent','learned'].includes(n.props.dataSource)){const w={kind:n.props.dataSource==='learned'?'known':n.props.dataTalentKind,id:+n.props.dataID};result.set(w.kind+':'+w.id,w);}}for(const m of r.style?.modifiers||[])add(m.condition);}return [...result.values()];}
  const bossEvents={bossstart:'Boss timer starts',bossupdate:'Boss timer changes',bosspause:'Boss timer pauses',bossresume:'Boss timer resumes',bossstop:'Boss timer stops',bossexpire:'Boss timer expires',bosswarning:'Boss warning',bossstage:'Boss stage changes'};
  const bossProviders={auto:'Automatic',dbm:'DBM',bigwigs:'BigWigs',blizzard:'Blizzard'};
  const timelineSources={any:'Any source',encounter:'Encounter',script:'Script'};
  const bossKinds={any:'Any',timer:'Timer',cast:'Cast',pull:'Pull',break:'Break'};
  function validBossFields(p){return [undefined,'','any','encounter','script'].includes(p.timelineSource)&&[undefined,'','auto','dbm','bigwigs','blizzard'].includes(p.provider)&&validCastSpell(p.spell)&&validCastSpell(p.encounterID)&&[undefined,'','any','timer','cast','pull','break'].includes(p.timerKind)&&(p.textFilter===undefined||typeof p.textFilter==='string'&&[...p.textFilter].length<=80);}
  function validBossCondition(p){return ['bossTimer','bossStage'].includes(p.condition)&&validBossFields(p)&&(p.condition==='bossStage'?['lt','lte','eq','ne','gt','gte'].includes(p.operator)&&Number.isFinite(p.value)&&p.value>=0&&p.value<=10000:['active','missing','paused','running'].includes(p.check)||['remaining','count'].includes(p.check)&&['lt','lte','eq','ne','gt','gte'].includes(p.operator)&&Number.isFinite(p.value)&&p.value>=0&&p.value<=(p.check==='count'?1e6:86400));}
  function bossProvider(sample,name){const data=(sample.playerContext||sample).bosses;if(!data)return null;if([undefined,'','auto'].includes(name))return ['dbm','bigwigs','blizzard'].map(n=>data.providers?.find(p=>p.name===n&&p.available)).find(Boolean)||null;return data.providers?.find(p=>p.name===name)||null;}
  function bossQuery(p){return {provider:p.provider||'auto',spell:+p.spell||0,text:p.textFilter||'',kind:p.timerKind||'any',source:p.timelineSource||'any',check:p.check||'',operator:p.operator||'',value:Number.isFinite(p.value)?p.value:0};}
  function bossTimerMatches(q,t){let matched=true;if(q.source&&q.source!=='any'){if(!t.source)matched=null;else if(q.source!==t.source)return false;}if(q.spell>0){if(!(t.spell>0))matched=null;else if(q.spell!==t.spell)return false;}if(q.text){if(!t.name)matched=null;else if(!t.name.toLowerCase().includes(q.text.toLowerCase()))return false;}if(q.kind&&q.kind!=='any'){if(!t.kind)matched=null;else if(q.kind!==t.kind)return false;}return matched;}
  function bossTimerCheck(q,t){if(!q.check||q.check==='active')return true;if(q.check==='missing')return false;if(['paused','running'].includes(q.check))return typeof t.paused==='boolean'?t.paused===(q.check==='paused'):null;const v=t[q.check];return Number.isFinite(v)?compare(v,q.value,q.operator):null;}
  function bossCondition(p,sample){
    const provider=bossProvider(sample,p.provider);if(!provider?.available)return null;
    if(p.condition==='bossStage'){if(+p.encounterID>0){if(!(provider.encounterID>0))return null;if(+p.encounterID!==provider.encounterID)return false;}return Number.isFinite(provider.stage)?compare(provider.stage,p.value,p.operator):null;}
    let uncertain=!provider.complete;const q=bossQuery(p);
    for(const t of (sample.playerContext||sample).bosses.timers||[]){if(t.provider!==provider.name)continue;const match=bossTimerMatches(q,t);if(match===false)continue;if(match===null){uncertain=true;continue;}if(p.check==='missing')return false;const value=bossTimerCheck(q,t);if(value===true)return true;if(value===null)uncertain=true;}
    return uncertain?null:p.check==='missing';
  }
  function bossTimerSource(q,sample){const provider=bossProvider(sample,q.provider);if(!provider?.available)return null;return ((sample.playerContext||sample).bosses.timers||[]).filter(t=>t.provider===provider.name&&bossTimerMatches(q,t)===true&&bossTimerCheck(q,t)===true).sort((a,b)=>(a.remaining??1e9)-(b.remaining??1e9)||a.id-b.id)[0]||null;}
  function bossEventMatches(p,sample){const e=sample.bossEvent,provider=bossProvider(sample,p.provider);return !!e&&e.kind===p.event&&provider?.available&&provider.name===e.timer.provider&&(!p.encounterID||+p.encounterID===e.encounterID)&&bossTimerMatches(bossQuery(p),e.timer)===true;}
  function bossExample(){return newRule('Boss timer soon',[node('when',{event:'state'},[node('if',{condition:'bossTimer',provider:'auto',spell:'',textFilter:'',timerKind:'any',check:'remaining',operator:'lt',value:5},[node('show',{display:'bar',text:'{name} · {remaining}s',visibility:'conditions',fill:'source'}),node('sound',{sound:'chime'})])])]);}

  const encounterEvents={encounterstart:'Encounter starts',encounterend:'Encounter ends',bosskill:'Boss defeated'};
  const difficulties={'':'Any',1:'Normal dungeon',2:'Heroic dungeon',8:'Mythic+',14:'Normal raid',15:'Heroic raid',16:'Mythic raid',17:'Raid Finder',23:'Mythic dungeon',24:'Timewalking dungeon',33:'Timewalking raid',233:'Mythic raid (flex)'};
  function validEncounterFields(p){return validCastSpell(p.encounterID)&&([undefined,''].includes(p.difficulty)||typeof p.difficulty==='string'&&/^[1-9][0-9]{0,3}$/.test(p.difficulty)&&+p.difficulty<=1000);}
  function validEncounterWhen(p){return validEncounterFields(p)&&[undefined,'','any','victory','wipe'].includes(p.outcome)&&(p.event==='encounterend'||[undefined,'','any'].includes(p.outcome));}
  function validEncounterCondition(p){return p.condition==='encounter'&&validEncounterFields(p)&&(['active','idle'].includes(p.check)||p.check==='elapsed'&&['lt','lte','eq','ne','gt','gte'].includes(p.operator)&&Number.isFinite(p.value)&&p.value>=0&&p.value<=86400);}
  function encounterMatches(p,e){return !!e&&e.kind===p.event&&(!p.encounterID||+p.encounterID===e.id)&&(!p.difficulty||+p.difficulty===e.difficulty)&&([undefined,'','any'].includes(p.outcome)||typeof e.success==='boolean'&&e.success===(p.outcome==='victory'));}
  function encounterCondition(p,sample){
    const e=(sample.playerContext||sample).encounter;if(!e||typeof e.active!=='boolean')return null;
    if(!e.active)return p.check==='idle';
    for(const [key,value]of [['encounterID','id'],['difficulty','difficulty']])if(+p[key]>0){if(!(e[value]>0))return null;if(+p[key]!==e[value])return p.check==='idle';}
    if(p.check==='active')return true;if(p.check==='idle')return false;
    return Number.isFinite(e.elapsed)?compare(e.elapsed,p.value,p.operator):null;
  }
  function validPetCondition(p){return p.condition==='pet'&&(['present','missing','alive','dead','unavailable'].includes(p.check)||['family','specialization','creatureType'].includes(p.check)&&typeof p.value==='string'&&p.value.trim()!==''&&new TextEncoder().encode(p.value).length<=160&&!/[\x00-\x1f]/.test(p.value));}
  function petCondition(p,sample){
    const context=sample.playerContext||sample,pet=context.pet;
    if(context.petVersion!==1||!pet||typeof pet.exists!=='boolean')return null;
    if(!pet.exists)return ['missing','unavailable'].includes(p.check);
    if(p.check==='present')return true;if(p.check==='missing')return false;
    if(['alive','dead','unavailable'].includes(p.check))return typeof pet.alive==='boolean'?pet.alive===(p.check==='alive'):null;
    const v=pet[p.check];return typeof v==='string'&&v.trim()!==''?v.trim().toLowerCase()===String(p.value).trim().toLowerCase():null;
  }
  function encounterExample(){return newRule('Fight timer',[node('when',{event:'state'},[node('if',{condition:'encounter',check:'active',encounterID:'',difficulty:'',operator:'gte',value:60},[node('show',{display:'text',text:'{name} · {elapsed}s',visibility:'conditions'})])])]);}
  const castEvents={caststart:'Cast starts',castsuccess:'Cast succeeds',caststop:'Cast stops',castfailed:'Cast fails',castinterrupt:'Cast interrupted',channelstart:'Channel starts',channelstop:'Channel stops',empowerstart:'Empower starts',empowerstop:'Empower stops'};
  function validCastSpell(v){return v===undefined||v===''||typeof v==='string'&&/^[1-9][0-9]{0,6}$/.test(v);}
  function validCasting(p){return p.condition==='casting'&&validUnit(p.unit)&&validCastSpell(p.spell)&&(['active','idle','cast','channel','empower','interruptible','protected'].includes(p.check)||p.check==='remaining'&&['lt','lte','eq','ne','gt','gte'].includes(p.operator)&&Number.isFinite(p.value)&&p.value>=0&&p.value<=86400);}
  function castCount(rules){rules=Actions.rules(rules);
    const units=new Set(),add=(unit,repeated)=>cloneUnits(repeated,unit).forEach(u=>units.add(u));
    function pred(p,repeated){for(const q of predicateList(p))if(q.condition==='casting')add(q.unit,repeated);}
    function walk(ns,repeated){for(const n of ns){if(n.type==='if')pred(n.props,repeated);if(n.type==='when'&&(Object.hasOwn(castEvents,n.props.event)||n.props.event==='casting'))add(n.props.unit,repeated);if(n.type==='show'&&n.props.dataSource==='casting')add(n.props.dataUnit,repeated);walk(n.children||[],childScope(n,repeated));}}
    for(const r of rules){if(r.enabled){walk(r.roots,0);walk(Values.inputs(r),ruleScopes(r.roots)|playerScope);for(const m of r.style.modifiers||[])pred(m.condition,ruleScopes(r.roots));}}
    return units.size;
  }
  function castCondition(p,sample){
    const unit=p.unit==='current'?sample.unit||'player':p.unit||'player',c=(sample.playerContext||sample).casts?.find(c=>c.unit===unit);
    if(!c?.available||!Number.isInteger(c.mode)||!Number.isInteger(c.spell))return null;
    if(c.mode===0)return p.check==='idle';if(p.check==='idle')return +p.spell>0&&c.spell!==+p.spell;
    if(+p.spell>0&&c.spell!==+p.spell)return false;
    if(p.check==='active')return true;
    if(['cast','channel','empower'].includes(p.check))return c.mode==={cast:1,channel:2,empower:3}[p.check];
    if(['interruptible','protected'].includes(p.check))return typeof c.interruptible==='boolean'?c.interruptible===(p.check==='interruptible'):null;
    return Number.isFinite(c.remaining)?compare(c.remaining,p.value,p.operator):null;
  }
  function castEventMatches(p,sample,context){
    if(!Object.hasOwn(castEvents,p.event))return true;
    const e=sample.castEvent,unit=p.unit==='current'?context.unit||'player':p.unit||'player';
    return !!e&&e.unit===unit&&Number.isInteger(e.spell)&&e.spell>0&&(!p.spell||+p.spell===e.spell);
  }
  function castExample(){return newRule('Target cast',[node('when',{event:'state'},[node('if',{condition:'casting',unit:'target',check:'interruptible',spell:'',operator:'lt',value:1},[node('show',{display:'bar',text:'{name} · {remaining}s',visibility:'conditions',fill:'source'})])])]);}

  const partyTokens=['player','party1','party2','party3','party4'];
  const cloneScopes={party:'Party',group:'Party or raid',raid:'Raid',boss:'Bosses',arena:'Arena opponents',nameplates:'Nameplates'},scopeNames=Object.keys(cloneScopes),playerScope=128;
  const scopeMask=s=>{const i=scopeNames.indexOf(s||'party');return i<0?0:1<<i;};
  function scopeTokens(scope){if(!scope||scope==='party')return [...partyTokens];if(scope==='group')return [...partyTokens,...scopeTokens('raid')];return Array.from({length:({raid:40,boss:8,arena:5,nameplates:40})[scope]||0},(_,i)=>(scope==='nameplates'?'nameplate':scope)+(i+1));}
  function scopeContains(scope,unit,group){if(scope==='group')scope=group==='raid'?'raid':'party';return scopeTokens(scope).includes(unit);}
  function childScope(n,inherited){return n.type==='each'?scopeMask(n.props.scope):inherited;}
  function hasEach(nodes){return nodes.some(n=>n.type==='each'||hasEach(n.children||[]));}
  function ruleScopes(nodes){let scopes=0;function walk(ns,current){for(const n of ns){if(n.type==='each')scopes|=scopeMask(n.props.scope);if(!current&&['show','glow','sound','hide'].includes(n.type))scopes|=playerScope;walk(n.children||[],childScope(n,current));}}walk(nodes,0);return scopes;}
  function cloneUnits(scopes,unit){if(unit!=='current')return [unit||'player'];if(!scopes)return ['player'];return [...new Set([...(scopes&playerScope?['player']:[]),...scopeNames.flatMap((s,i)=>scopes&(1<<i)?scopeTokens(s):[])])];}
  function cloneActor(scope,member){scope||='party';return scope==='party'&&!/^(Slot|Creature|Pet|Vehicle)-/.test(member.actor)?member.actor:scope+'@'+member.actor;}
  function cloneMembers(sample,scope='party'){
    const roster=sample.units||sample.party||[],seen=new Set(),actual=scope==='group'?(sample.group==='raid'?'raid':'party'):scope;
    return scopeTokens(actual).flatMap(unit=>{const m=roster.find(m=>m.unit===unit);if(!m||seen.has(m.actor))return [];seen.add(m.actor);return [{...m,actor:cloneActor(scope,m)}];});
  }
  function cloneContext(sample,actor){
    if(!actor)return sample;let scope='party',identity=actor;const split=actor.indexOf('@');if(split>=0){scope=actor.slice(0,split);identity=actor.slice(split+1);if(!Object.hasOwn(cloneScopes,scope))return null;}
    const m=(sample.units||sample.party||[]).find(m=>m.actor===identity&&scopeContains(scope,m.unit,sample.group));if(!m)return null;
    return {...sample,...m,actor,unit:m.unit||'player',role:m.role??null,health:m.health??(m.unit==='player'?sample.health:null),auraUnit:m.unit||'player',auras:m.auras,resources:m.resources,primaryPower:m.primaryPower,boundActor:actor,cloneActor:actor,playerContext:sample};
  }
  function nodeScope(rule,id){rule=Actions.executionRule(rule);let hit=find(rule.roots,id);while(hit){if(hit.node.type==='each')return hit.node.props.scope||'party';hit=hit.parent?find(rule.roots,hit.parent.id):null;}return '';}
  function repeatedNode(rule,id){return !!nodeScope(rule,id);}
  function watchCount(rules){rules=Actions.rules(rules);
    const watches=new Set(),add=(id,kind,unit,repeated)=>{for(const u of cloneUnits(repeated,unit))watches.add(u+':'+kind+':'+Number(id));};
    function walk(ns,repeated){for(const n of ns){const p=n.props;if(n.type==='show'&&['buff','debuff'].includes(p.dataSource))add(p.dataID,p.dataSource,p.dataUnit,repeated);if(n.type==='if')for(const q of predicateList(p))if(['buff','debuff'].includes(q.condition))add(q.aura,q.condition,q.unit,repeated);walk(n.children||[],childScope(n,repeated));}}
    for(const r of rules){if(!r.enabled)continue;walk(r.roots,0);walk(Values.inputs(r),ruleScopes(r.roots)|playerScope);const repeated=ruleScopes(r.roots);for(const m of r.style.modifiers||[])walk([{type:'if',props:m.condition}],repeated);const a=r.style.arc;if(a?.repeat==='stacks'&&a.aura>0&&flatten(r.roots).some(n=>n.type==='show'&&['arc','ring'].includes(n.props.display)))add(a.aura,a.kind,a.unit,repeated);}
    return watches.size;
  }
  function measurementCount(rules){rules=Actions.rules(rules);
    const seen=new Set(),add=(unit,kind,resource,repeated)=>{for(const u of cloneUnits(repeated,unit))if(u!=='player')seen.add(u+':'+kind+':'+(kind==='health'?'':resource||'primary'));};
    function pred(p,repeated){for(const q of predicateList(p))if(['health','resource'].includes(q.condition))add(q.unit||'current',q.condition,q.resource,repeated);}
    function walk(ns,repeated){for(const n of ns){if(n.type==='if')pred(n.props,repeated);if(n.type==='show'&&['health','resource'].includes(n.props.dataSource))add(n.props.dataUnit,n.props.dataSource,n.props.dataResource,repeated);walk(n.children||[],childScope(n,repeated));}}
    for(const r of rules){if(!r.enabled)continue;walk(r.roots,0);walk(Values.inputs(r),ruleScopes(r.roots)|playerScope);const repeated=ruleScopes(r.roots);for(const m of r.style.modifiers||[])pred(m.condition,repeated);const a=r.style.arc;if(a&&['health','resource'].includes(a.fill)&&flatten(r.roots).some(n=>n.type==='show'&&['arc','ring'].includes(n.props.display)))add(a.fillUnit,a.fill,a.resource,repeated);}
    return seen.size;
  }
  function clonesExample(){return newRule('Party buff',[node('when',{event:'state'},[node('each',{scope:'party'},[node('if',{condition:'buff',aura:'436336',unit:'current',check:'active'},[node('show',{display:'text',text:'{unitName}: {stacks}',visibility:'conditions'})])])])]);}
  function validCooldown(p) {
    if(p.condition!=='cooldown'||typeof p.id!=='string'||!/^[1-9][0-9]{0,6}$/.test(p.id))return false;
    if(!['spell','item','slot','gcd'].includes(p.source)||(p.source==='slot'&&+p.id>19)||(p.source==='gcd'&&p.id!=='61304'))return false;
    if(['carried','missing','cooldownReady'].includes(p.check))return p.source==='item';
    if(['ready','cooldown','usable','unusable'].includes(p.check))return true;
    if(['charges','full','recharge'].includes(p.check)&&p.source!=='spell')return false;
    if(p.check==='full')return true;
    return ['remaining','charges','recharge'].includes(p.check)&&['lt','lte','eq','ne','gt','gte'].includes(p.operator)&&Number.isFinite(p.value)&&p.value>=0&&p.value<=(p.check==='charges'?999:86400)&&(p.check!=='charges'||Number.isInteger(p.value));
  }
  function cooldownCount(rules) {rules=Actions.rules(rules);
    return new Set(rules.filter(r=>r.enabled).flatMap(r=>[
      ...flatten([...r.roots,...Values.inputs(r)]).flatMap(n=>n.type==='if'?predicateList(n.props).filter(p=>p.condition==='cooldown').map(p=>p.source+':'+Number(p.id)):n.type==='show'&&['spell','item','slot','gcd'].includes(n.props.dataSource)?[n.props.dataSource+':'+Number(n.props.dataID)]:[]),
      ...(r.style.modifiers||[]).flatMap(m=>predicateList(m.condition)).filter(p=>p.condition==='cooldown').map(p=>p.source+':'+Number(p.id))
    ])).size;
  }
  function cooldownCondition(p, context) {
    const c=context.cooldowns?.find(c=>c.kind===p.source&&c.id===Number(p.id));
    if(p.check==='cooldownReady')return p.source==='item'&&c?.available&&Number.isFinite(c.remaining)?c.remaining===0:null;
    if(['carried','missing'].includes(p.check)) {
      if(p.source!=='item'||!c)return null;
      const carried=typeof c.carried==='boolean'?c.carried:c.available?true:null;
      return carried===null?null:carried===(p.check==='carried');
    }
    if(!c||!c.available)return null;
    if(c.kind==='item'&&c.carried===false)return null;
    if(['ready','cooldown','usable','unusable'].includes(p.check)) {
      if(typeof c.enabled!=='boolean'||!Number.isFinite(c.remaining)||!Number.isFinite(c.maxCharges))return null;
      if(!c.enabled)return false;
      if(c.maxCharges>0&&!Number.isFinite(c.charges))return null;
      const ready=c.remaining===0&&(c.maxCharges===0||c.charges>0);
      if(p.check==='ready')return ready;if(p.check==='cooldown')return !ready;
      if(typeof c.usable!=='boolean')return null;
      return p.check==='usable'?ready&&c.usable:!(ready&&c.usable);
    }
    if(['full','charges','recharge'].includes(p.check)&&(!Number.isFinite(c.maxCharges)||c.maxCharges===0))return null;
    if(p.check==='full')return Number.isFinite(c.charges)?c.charges===c.maxCharges:null;
    const value=c[p.check];return Number.isFinite(value)?compare(value,p.value,p.operator):null;
  }
  function cooldownExample() {
    return newRule('Cooldown ready',[node('when',{event:'cooldown'},[node('if',{condition:'cooldown',source:'spell',id:'363916',check:'ready',operator:'gte',value:1},[node('show',{display:'text',text:'Ready!',visibility:'conditions'}),node('sound')])])]);
  }
    function validAura(p) {
      if (!['buff', 'debuff'].includes(p.condition) || typeof p.aura !== 'string' || !/^[1-9][0-9]{0,6}$/.test(p.aura)) return false;
      if (!validUnit(p.unit)) return false;
      if (['active', 'missing'].includes(p.check)) return true;
      if (!['stacks', 'remaining'].includes(p.check) || !['eq', 'ne', 'lt', 'lte', 'gt', 'gte'].includes(p.operator) || typeof p.value !== 'number' || !Number.isFinite(p.value) || p.value < 0) return false;
      return p.check === 'stacks' ? Number.isInteger(p.value) && p.value <= 999 : p.value <= 3600;
    }
    function validPredicate(p,depth=0,budget={count:0}) {
      if(!p||typeof p!=='object'||Array.isArray(p)||depth>4||++budget.count>32)return false;
      if(Object.entries(p).some(([k,v])=>k!=='tests'&&!['string','number','boolean'].includes(typeof v)))return false;
      if(p.condition==='group')return ['all','any','not'].includes(p.mode)&&Array.isArray(p.tests)&&(p.mode!=='not'||p.tests.length<=1)&&p.tests.every(c=>validPredicate(c,depth+1,budget));
      return validPetCondition(p) || (p.condition==='value'&&Values.validCondition(p)) || (p.condition === 'role' && ['tank','healer','damage'].includes(p.value)) || (p.condition === 'location' && ['dungeon','raid','world'].includes(p.value)) || (p.condition === 'health' && validUnit(p.unit) && Number.isFinite(p.value) && +p.value >= 0 && +p.value <= 100 && [undefined,'lt','lte','eq','ne','gt','gte'].includes(p.operator)) || (p.condition === 'resource' && validUnit(p.unit) && Object.hasOwn(resources,p.resource) && ['amount','percent'].includes(p.measure) && (p.operator==='cap' || (['lt','lte','eq','ne','gt','gte'].includes(p.operator) && Number.isFinite(p.value) && +p.value>=0 && +p.value<=(p.measure==='percent'?100:16777214)))) || validAura(p) || validCooldown(p) || validCasting(p) || validEventValue(p) || validEncounterCondition(p) || validBossCondition(p) || validTalent(p) || validEquipment(p);
    }
  function usage(rules){return [
    ['Overlays',rules.length,limits.rules],['Aura sources',watchCount(rules),limits.auras],['Timer slots',cooldownCount(rules)+2*castCount(rules),limits.timers],['Health / resource sources',measurementCount(rules),limits.measurements],['Attachments',anchorCount(rules),limits.anchors],['Combat units',combatCount(rules),limits.combat],['Talents / learned spells',talentWatches(rules).length,limits.talents],['Saved outfits',equipmentSetNames(rules).length,limits.outfits]
  ];}
  function validRules(rules, allowEmpty = false) {
    if (!Array.isArray(rules) || (!allowEmpty && !rules.length) || rules.length > limits.rules) return false;
    const ids = new Set();
    const validId = v => typeof v === 'string' && /^[a-zA-Z0-9_-]{1,80}$/.test(v) && !ids.has(v) && !!ids.add(v);
    const colors = ['gold', 'teal', 'violet', 'red'];
    const object = v => v !== null && typeof v === 'object' && !Array.isArray(v);
    function validLoad(load) {
      if (load === undefined || load === null) return true;
      if (!object(load)) return false;
      if(load.conditions!==undefined&&(!validPredicate(load.conditions)||predicateList(load.conditions).some(p=>!['group','talent','learned','equipment'].includes(p.condition))))return false;
      for (const key of ['character','realm','class','race','faction','role','location','zone']) {
        const value = load[key];
        if (value !== undefined && (typeof value !== 'string' || new TextEncoder().encode(value).length > 120 || /[\r\n\0]/.test(value))) return false;
      }
      for (const [key,max] of [['spec',10000],['minLevel',1000]]) {
        if (load[key] !== undefined && (!Number.isInteger(load[key]) || load[key]<0 || load[key]>max)) return false;
      }
      return ['combat','mounted','alive'].every(k => [undefined,'','yes','no'].includes(load[k])) && [undefined,'','solo','party','raid'].includes(load.group);
    }
    let count = 0;
    function check(n, depth, inEach=false) {
      if (++count > 80 || !object(n) || typeof n.type !== 'string' || depth > 8 || !Object.hasOwn(definitions,n.type) || !validId(n.id) || !object(n.props)) return false;
      if (Object.entries(n.props).some(([k,v]) => !(n.type==='if'&&k==='tests')&&!['string','number','boolean'].includes(typeof v))) return false;
      if (depth === 0 && !['when','action'].includes(n.type)) return false;
      if(n.type==='action'&&(depth!==0||typeof n.props.name!=='string'||!n.props.name.trim()||[...n.props.name].length>60||/[\0\r\n]/.test(n.props.name)||Object.keys(n.props).length!==1))return false;
      if(n.type==='call'&&(typeof n.props.action!=='string'||!n.props.action||Object.keys(n.props).length!==1))return false;
      const p = n.props;
      if(n.type==='each'&&(inEach||!(p.scope===undefined||Object.hasOwn(cloneScopes,p.scope))))return false;
      if (n.type === 'when' && !['talents','equipment','ability', 'health', 'resource', 'enter', 'key', 'aura', 'leave', 'keyend','state','login','combat','combatend','cooldown','casting','combatlog',...Object.keys(castEvents),...Object.keys(encounterEvents),...Object.keys(bossEvents)].includes(p.event)) return false;
      if(n.type==='when'&&(Object.hasOwn(castEvents,p.event)||p.event==='casting')&&(!validUnit(p.unit)||!validCastSpell(p.spell)))return false;
      if(n.type==='when'&&Object.hasOwn(bossEvents,p.event)&&!validBossFields(p))return false;
      if(n.type==='when'&&Object.hasOwn(encounterEvents,p.event)&&!validEncounterWhen(p))return false;
      if(n.type==='when'&&p.event==='combatlog'&&!validCombatWhen(p))return false;
      if (n.type === 'if' && !validPredicate(p))return false;
      if (n.type === 'show' && (!['icon','text','bar','arc','ring','shape','image'].includes(p.display) || typeof p.text !== 'string' || p.text.length > 90 || ![undefined,'timed','conditions'].includes(p.visibility) || (p.visibility!=='conditions' && (!Number.isFinite(p.duration) || p.duration < 1 || p.duration > 120)))) return false;
      if (n.type === 'show' && !validShowData(p)) return false;
      if (n.type === 'glow' && !colorValue(p.color)) return false;
      if (n.type === 'sound' && !validSound(p.sound)) return false;
      if (n.type === 'wait' && (!Number.isFinite(p.seconds) || p.seconds < 0 || p.seconds > 15)) return false;
      return definitions[n.type].container ? Array.isArray(n.children) && n.children.every(c => check(c, depth + 1,inEach||n.type==='each')) : !n.children;
    }
    const valid = rules.every(r => {
      count = 0;
      if (!r || !validId(r.id) || typeof r.name !== 'string' || !r.name.trim() || r.name.length > 60 || typeof r.enabled !== 'boolean' || !Array.isArray(r.roots) || !r.roots.every(n => check(n, 0))) return false;
      if (!object(r.style) || !validLoad(r.load)) return false;
      if(!Values.validRule(r,valueAdapters)||!Actions.valid(r))return false;
      const s = r.style;
      if(['onShow','onHide'].some(k=>!validSound(s[k],true)))return false;
      if(s.modifiers!==undefined&&(!Array.isArray(s.modifiers)||s.modifiers.length>16||!s.modifiers.every(m=>{
        if(!object(m)||!validPredicate(m.condition)||m.color!==undefined&&!colorValue(m.color)||!validSound(m.sound,true))return false;
        if(['glow','visible'].some(k=>m[k]!==undefined&&typeof m[k]!=='boolean'))return false;
        if(m.text!==undefined&&(typeof m.text!=='string'||m.text.length>90))return false;
        return [['size',16,400],['font',10,72],['opacity',0,100]].every(([k,lo,hi])=>m[k]===undefined||Number.isFinite(m[k])&&m[k]>=lo&&m[k]<=hi);
      })))return false;
      if(!validAnchor(s.anchor)||!validVisual(s.visual))return false;
      if(s.arc !== undefined && !validArc(s.arc))return false;
      return Number.isFinite(s.size) && s.size >= 16 && s.size <= 400 && Number.isFinite(s.opacity) && s.opacity >= 25 && s.opacity <= 100 && !!colorValue(s.color) && Number.isFinite(s.font) && s.font>=10 && s.font<=72 && typeof s.transparent === 'boolean' && Number.isFinite(s.x) && Number.isFinite(s.y) && s.x >= 0 && s.x <= 100 && s.y >= 0 && s.y <= 100;
    });
    return valid && Packs.valid(rules) && anchorCount(rules)<=limits.anchors && validGroups(rules) && watchCount(rules) <= limits.auras && cooldownCount(rules)+2*castCount(rules) <= limits.timers && measurementCount(rules)<=limits.measurements && combatCount(rules)<=8 && talentWatches(rules).length<=16 && equipmentSetNames(rules).length<=8;
  }
  const dataKinds={value:'Value',talent:'Talent',learned:'Learned spell',auto:'Automatic',trigger:'Trigger',encounter:'Encounter',bossTimer:'Boss timer',bossStage:'Boss stage',affected:'Affected spell',casting:'Cast',health:'Health',resource:'Resource',buff:'Buff',debuff:'Debuff',spell:'Spell cooldown',item:'Item cooldown',slot:'Equipped cooldown',gcd:'Global cooldown',none:'None'};
  const dataTokens={rank:'Talent rank',name:'Name',remaining:'Time left',duration:'Duration',stacks:'Stacks',charges:'Charges',maxCharges:'Max charges',recharge:'Recharge',current:'Amount',max:'Maximum',percent:'Percent',character:'Character',realm:'Realm',unit:'Unit',unitName:'Unit name',role:'Role',sourceName:'Source name',targetName:'Target name',...combatValues,extraName:'Affected spell',spellID:'Spell ID',extraSpellID:'Affected spell ID',event:'Event',miss:'Miss outcome',encounterName:'Encounter name',encounterID:'Encounter ID',difficulty:'Difficulty ID',groupSize:'Group size',outcome:'Fight outcome',elapsed:'Elapsed seconds',provider:'Boss provider',timerID:'Timer ID',count:'Timer count',stage:'Stage',stageTotal:'Stage changes'};
  function dataSource(rule,id) {
    rule=Actions.executionRule(rule);
    function walk(nodes,inherited={kind:'none'},repeated=false) {
      for(const n of nodes) {
        let source=inherited;const p=n.props;
        if(n.type==='when'&&['none','castEvent','combatEvent','encounterEvent','bossEvent'].includes(source.kind)&&Object.hasOwn(castEvents,p.event))source={kind:'castEvent',unit:p.unit||'player'};
        if(n.type==='when'&&['none','castEvent','combatEvent','encounterEvent','bossEvent'].includes(source.kind)&&p.event==='combatlog')source={kind:'combatEvent'};
        if(n.type==='when'&&['none','castEvent','combatEvent','encounterEvent','bossEvent'].includes(source.kind)&&Object.hasOwn(encounterEvents,p.event))source={kind:'encounterEvent'};
        if(n.type==='when'&&['none','castEvent','combatEvent','encounterEvent','bossEvent'].includes(source.kind)&&Object.hasOwn(bossEvents,p.event))source={kind:'bossEvent'};
        if(n.type==='if') {
          const choices=predicateList(p).flatMap(p=>p.condition==='value'?[{kind:'value',value:p.key,maximum:100}]:['talent','learned'].includes(p.condition)?[{kind:p.condition,unit:'player',id:+p.id,talentKind:talentWatch(p).kind}]:['bossTimer','bossStage'].includes(p.condition)?[{kind:p.condition,unit:'player',boss:p.condition==='bossStage'?{provider:p.provider||'auto'}:bossQuery(p)}]:['health','resource','buff','debuff','casting','encounter'].includes(p.condition)?[{kind:p.condition,id:+p.aura||0,resource:p.condition==='resource'?p.resource:'',unit:p.unit||(['health','resource'].includes(p.condition)?'current':'player')}]:p.condition==='cooldown'?[{kind:p.source,id:+p.id,resource:'',unit:'player'}]:[]);
          const unique=[...new Set(choices.map(c=>JSON.stringify(c)))];
          if(unique.length===1){source=JSON.parse(unique[0]);if(!repeated&&source.unit==='current')source.unit='player';}
        }
        if(n.id===id)return !p.dataSource||p.dataSource==='auto'?source:explicitDataSource(p);
        const found=walk(n.children||[],source,repeated||n.type==='each');if(found)return found;
      }
    }
    return walk(rule.roots)||{kind:'none'};
  }
  function explicitDataSource(p){return {kind:p.dataSource,id:+p.dataID,talentKind:p.dataSource==='learned'?'known':p.dataTalentKind,resource:p.dataResource||'primary',unit:p.dataUnit||'player',...(['bossTimer','bossStage'].includes(p.dataSource)?{boss:bossQuery({provider:p.dataProvider,spell:p.dataID,textFilter:p.dataText,timerKind:p.dataKind,timelineSource:p.dataTimelineSource})}:{}),...(p.dataSource==='value'?{value:p.dataValue,maxValue:p.dataMaxValue,maximum:p.dataMaximum??100}:{})};}
  function validShowData(p) {
    if(![undefined,'','timer','full','source','charges','recharge'].includes(p.fill))return false;
    if(![undefined,'','shield','auto'].includes(p.icon)&&!(typeof p.icon==='string'&&/^[1-9][0-9]{0,6}$/.test(p.icon)))return false;
    const kind=p.dataSource||'auto';if(!Object.hasOwn(dataKinds,kind))return false;
    if(kind==='value')return typeof p.dataValue==='string'&&Values.keyPattern.test(p.dataValue)&&(!p.dataMaxValue||Values.keyPattern.test(p.dataMaxValue))&&(p.dataMaxValue===undefined||typeof p.dataMaxValue==='string')&&Number.isFinite(p.dataMaximum??100)&&(p.dataMaximum??100)>=.000000001&&(p.dataMaximum??100)<=Values.limit;
    if(['talent','learned'].includes(kind))return validTalent({condition:kind,source:p.dataTalentKind,id:p.dataID,check:'selected'});
    if(['bossTimer','bossStage'].includes(kind))return validBossFields({provider:p.dataProvider,spell:p.dataID,textFilter:p.dataText,timerKind:p.dataKind,timelineSource:p.dataTimelineSource});
    if(['auto','none','trigger','affected','encounter'].includes(kind))return true;if(['health','casting'].includes(kind))return validUnit(p.dataUnit);
    if(kind==='resource')return validUnit(p.dataUnit)&&Object.hasOwn(resources,p.dataResource||'primary');
    if(typeof p.dataID!=='string'||!/^[1-9][0-9]{0,6}$/.test(p.dataID))return false;
    if(['buff','debuff'].includes(kind))return validUnit(p.dataUnit);
    return kind==='slot'?+p.dataID<=19:kind==='gcd'?p.dataID==='61304':true;
  }
  function resolvedData(rawSource,sample,names={}) {
    const s={...rawSource,unit:rawSource.unit==='current'?sample.unit||'player':rawSource.unit},known=n=>Number.isFinite(n)&&n>=0&&n<=16777214;
    const displayUnit=sample.cloneActor?sample.unit:s.unit||'player',identity=sample.cloneActor?sample:unitContext(sample,displayUnit);
    const text={character:sample.character||'Player',realm:sample.realm||'Realm',unit:displayUnit,unitName:identity.name||(displayUnit==='player'?sample.character||'Player':''),role:identity.role||''},number=n=>known(n)?String(Math.round(n*10)/10):'—';
    const owner=unitContext(sample,rawSource.unit||'player');
    const numbers={};let fraction=null,charges=null,recharge=null,spell=0;
    const set=(key,n)=>{if(known(n)){text[key]=number(n);numbers[key]=n;}},ratio=(a,b)=>known(a)&&known(b)&&b>0&&a<=b?a/b:null;
    if(s.kind==='value'){const e=valueEvaluation(sample.overlayLogic,sample,names),v=e.value(s.value),max=s.maxValue?e.value(s.maxValue):s.maximum??100;text.name=Values.named(sample.overlayLogic,s.value)?.name;if(v!==null)text.current=Values.display(v);if(typeof max==='number'){text.max=Values.display(max);numbers.max=max;}if(typeof v==='number'){numbers.current=v;if(typeof max==='number'&&max>0)fraction=Math.max(0,Math.min(1,v/max));}}
    if(['buff','debuff','spell','gcd'].includes(s.kind)){spell=s.id;text.name=names[s.id]||`Spell ${s.id}`;}
    if(s.kind==='casting'){const c=(sample.playerContext||sample).casts?.find(c=>c.unit===s.unit);if(c?.available&&c.mode>0&&Number.isInteger(c.spell)){spell=c.spell;text.name=names[spell]||`Spell ${spell}`;set('remaining',c.remaining);set('duration',c.duration);fraction=ratio(c.remaining,c.duration);}}
    if(['castEvent','trigger'].includes(s.kind)&&sample.castEvent){spell=sample.castEvent.spell;text.name=names[spell]||`Spell ${spell}`;text.unit=sample.castEvent.unit;}
    if(s.kind==='health'){text.name='Health';set('percent',owner.health);fraction=ratio(owner.health,100);}
    if(s.kind==='resource'){
      const key=s.resource==='primary'?owner.primaryPower:s.resource,r=owner.resources?.[key];text.name=resources[key];
      if(r&&known(r.max)&&r.max>0&&known(r.current)&&r.current<=r.max){set('current',r.current);set('max',r.max);fraction=ratio(r.current,r.max);}
    }
    if(['buff','debuff'].includes(s.kind)&&Array.isArray(owner.auras)&&(s.unit||'player')===(owner.auraUnit||'player')){
      const a=owner.auras.find(a=>a.kind===s.kind&&String(a.name)===String(s.id)&&(a.unit||'player')===(s.unit||'player'));
      if(!a){set('remaining',0);set('stacks',0);fraction=0;}
      else {set('stacks',a.stacks);set('remaining',a.remaining);set('duration',a.duration);fraction=ratio(a.remaining,a.duration);if(a.permanent){text.remaining=text.duration='∞';delete numbers.remaining;delete numbers.duration;fraction=1;}}
    }
    if(['spell','item','slot','gcd'].includes(s.kind)){
      if(s.kind==='item')text.name=`Item ${s.id}`;if(s.kind==='slot')text.name=`Slot ${s.id}`;if(s.kind==='gcd')text.name='Global cooldown';
      const c=owner.cooldowns?.find(c=>c.kind===s.kind&&c.id===s.id&&c.available);
      if(c){set('remaining',c.remaining);set('duration',c.duration);fraction=ratio(c.remaining,c.duration);if(c.remaining===0&&c.duration===0)fraction=0;
        if(known(c.maxCharges)&&c.maxCharges>0){set('charges',c.charges);set('maxCharges',c.maxCharges);set('recharge',c.recharge);charges=ratio(c.charges,c.maxCharges);recharge=ratio(c.recharge,c.rechargeDuration);if(c.charges===c.maxCharges){recharge=0;set('recharge',0);}}
      }
    }
    if(['talent','learned'].includes(s.kind)){const t=talentReading({kind:s.talentKind,id:s.id},sample);if(t){set('rank',t.rank);if(['known','pvp','hero'].includes(t.kind)&&Number.isInteger(t.rank))fraction=t.rank;if(t.spell>0){spell=t.spell;text.name=names[spell]||`Spell ${spell}`;text.spellID=String(spell);}}}
    const bossValues=t=>{if(!t)return;Object.assign(text,{name:t.name,provider:{dbm:'DBM',bigwigs:'BigWigs',blizzard:'Blizzard'}[t.provider]});if(t.id>0)text.timerID=String(t.id);if(t.spell>0){spell=t.spell;text.spellID=String(t.spell);if(!text.name)text.name=names[t.spell]||`Spell ${t.spell}`;}for(const k of ['duration','remaining','count'])set(k,t[k]);fraction=ratio(t.remaining,t.duration);if(t.remaining===0&&t.duration===0)fraction=0;};
    if(s.kind==='bossTimer')bossValues(bossTimerSource(s.boss,sample));
    if(s.kind==='bossStage'){const provider=bossProvider(sample,s.boss.provider);if(provider?.available){text.name='Stage';text.provider={dbm:'DBM',bigwigs:'BigWigs',blizzard:'Blizzard'}[provider.name];set('stage',provider.stage);set('stageTotal',provider.stageTotal);}}
    if(sample.bossEvent&&!['bossTimer','bossStage'].includes(s.kind)){
      const e=sample.bossEvent;Object.assign(text,{provider:{dbm:'DBM',bigwigs:'BigWigs',blizzard:'Blizzard'}[e.timer.provider],event:e.kind});if(e.timer.id>0)text.timerID=String(e.timer.id);set('count',e.timer.count);set('stage',e.stage);set('stageTotal',e.stageTotal);
      if(['bossEvent','trigger'].includes(s.kind)){let t=e.timer;if(s.kind==='bossEvent'&&t.id>0){const provider=bossProvider(sample,t.provider),current=provider?.available?(sample.playerContext||sample).bosses.timers?.find(b=>b.id===t.id&&b.provider===t.provider):null;t=current||{...t,remaining:provider?.available&&provider.complete?0:null};}bossValues(t);}
    }

    const liveEncounter=(sample.playerContext||sample).encounter;
    const encounter=s.kind==='encounter'?(liveEncounter?.active===true?liveEncounter:null):sample.encounterEvent;
    if(encounter){const e=encounter;Object.assign(text,{encounterName:e.name});for(const k of ['id','difficulty','groupSize'])if(e[k]>0)text[k==='id'?'encounterID':k]=String(e[k]);
      if(s.kind!=='encounter'){text.event=e.kind;if(typeof e.success==='boolean')text.outcome=e.success?'Victory':'Wipe';}
      if(['encounter','encounterEvent','trigger'].includes(s.kind)){text.name=e.name;set('elapsed',e.elapsed);}
    }
    if(sample.combatEvent){const e=sample.combatEvent;Object.assign(text,{sourceName:e.source?.name,targetName:e.target?.name,extraName:e.extraName,event:e.kind,miss:e.miss});if(e.spell)text.spellID=String(e.spell);if(e.extraSpell)text.extraSpellID=String(e.extraSpell);
      for(const [k,v]of Object.entries(e.values||{}))if(Number.isFinite(v)&&v>=0&&v<=1e12&&(k!=='stacks'||['combatEvent','trigger','affected'].includes(s.kind))){text[k]=String(Math.round(v*10)/10);numbers[k]=v;}
      if(['combatEvent','trigger','affected'].includes(s.kind)){spell=s.kind==='affected'?e.extraSpell:e.spell;text.name=s.kind==='affected'?e.extraName:e.name;}
    }
    if(fraction!==null)set('percent',fraction*100);
    return {text,numbers,fraction,charges,recharge,spell};
  }
  function resolveShow(rule,id,props,sample,names={}) {
    rule=Actions.executionRule(rule);
    sample={...sample,overlayLogic:rule.logic,overlayMemory:sample.overlayMemories?.get(rule.id)||sample.overlayMemory};
    const source=dataSource(rule,id),{text,numbers,fraction,charges,recharge,spell}=resolvedData(source,sample,names);
    const output={...props,text:String(props.text||'').replace(/\{(name|remaining|duration|stacks|charges|maxCharges|recharge|current|max|percent|character|realm|unit|unitName|role|sourceName|targetName|amount|effective|baseAmount|absorbed|overheal|overkill|blocked|resisted|wasted|power|critical|extraName|spellID|extraSpellID|event|miss|encounterName|encounterID|difficulty|groupSize|outcome|elapsed|provider|timerID|count|stage|stageTotal|rank)\}/g,(_,key)=>text[key]!==undefined&&(text[key]!==''||source.kind==='value'&&key==='current')?text[key]:'—'),spell:props.icon==='auto'?spell:/^[1-9][0-9]{0,6}$/.test(props.icon)?+props.icon:0};
    if(output.text.includes('{value:'))output.text=valueEvaluation(rule.logic,sample,names).text(output.text);
    const numeric=k=>{const v=Number(text[k]);return text[k]!==undefined&&text[k]!==''&&Number.isFinite(v)?v:undefined;};output.remaining=numeric('remaining');output.stacks=numeric('stacks')??numeric('charges');
    output.fraction=props.fill==='full'?1:props.fill==='source'?fraction:props.fill==='charges'?charges:props.fill==='recharge'?recharge:undefined;
    output.visible=!['bar','arc','ring','shape','image'].includes(props.display)||output.fraction!==null;
    return output;
  }
  function resolveAppearance(rule,sample,base=rule.style){
    sample={...sample,overlayLogic:rule.logic,overlayMemory:sample.overlayMemories?.get(rule.id)||sample.overlayMemory};
    const result={style:{...base},visible:true};
    for(const m of rule.style.modifiers||[]){
      if(condition({props:m.condition},sample)!==true)continue;
      for(const k of ['color','size','font','opacity'])if(m[k]!==undefined)result.style[k]=m[k];
      for(const k of ['text','glow','visible'])if(m[k]!==undefined)result[k]=m[k];
      if(m.color!==undefined)result.color=m.color;
    }
    return result;
  }
  function arcDefaults(style,kind='arc') {
    return {start:-135,sweep:kind==='ring'?360:270,thickness:Math.max(3,style.size*.06),gap:6,reverse:false,fill:'timer',resource:'primary',repeat:'single',layout:'nested',pair:'sides',aura:0,kind:'buff',unit:'player',limit:2};
  }
  function validArc(a) {
    const range=(v,lo,hi)=>Number.isFinite(v)&&v>=lo&&v<=hi;
    return a && typeof a==='object' && !Array.isArray(a) && range(a.start,-360,360) && range(a.sweep,1,360) && range(a.thickness,1,40) && range(a.gap,0,40) && typeof a.reverse==='boolean' && ['full','timer','health','resource'].includes(a.fill) && typeof a.resource==='string' && (a.resource==='primary'||/^(?:[0-9]|[12][0-9]|3[01])$/.test(a.resource)) && ['single','stacks'].includes(a.repeat) && ['around','nested'].includes(a.layout) && ['sides','vertical'].includes(a.pair) && Number.isInteger(a.aura) && range(a.aura,0,9999999) && ['buff','debuff'].includes(a.kind) && validUnit(a.unit) && validUnit(a.fillUnit) && Number.isInteger(a.limit) && range(a.limit,1,20);
  }
  function arcExample() {
    const r=newRule('Stack arches',[node('when',{event:'state'},[node('show',{display:'arc',text:'',visibility:'conditions'})])]);
    r.style.size=220;r.style.arc={...arcDefaults(r.style),start:0,sweep:160,thickness:8,fill:'full',repeat:'stacks',layout:'around',aura:124273};return r;
  }
  function arcState(style,kind,timerFraction,sample) {
    const a=style.arc||arcDefaults(style,kind),capacity=a.repeat==='stacks'?a.limit:1,fillSample=unitContext(sample,a.fillUnit||'player');
    let count=1,fraction=a.fill==='timer'?timerFraction:1;
    if(a.fill==='health'){if(!Number.isFinite(fillSample.health)||fillSample.health<0||fillSample.health>100)return null;fraction=fillSample.health/100;}
    if(a.fill==='resource'){
      const r=fillSample.resources?.[a.resource==='primary'?fillSample.primaryPower:a.resource];
      if(!r||!Number.isFinite(r.max)||!Number.isFinite(r.current)||r.max<=0||r.current<0||r.current>r.max)return null;
      fraction=r.current/r.max;
    }
    if(a.repeat==='stacks'){
      const unit=a.unit==='current'?sample.unit||'player':a.unit;const source=unitContext(sample,a.unit||'player');if(unit!==(source.auraUnit||'player')||!Array.isArray(source.auras))return null;const w=source.auras.find(w=>String(w.name)===String(a.aura)&&w.kind===a.kind);
      if(!w)return {count:0,fraction};
      if(!Number.isFinite(w.stacks)||w.stacks<0)return null;
      count=Math.min(capacity,Math.floor(w.stacks));
    }
    return {count,fraction};
  }
  function arcPaths(style,kind,timerFraction,sample) {
    const state=arcState(style,kind,timerFraction,sample);if(!state||state.fraction<=0||!state.count)return [];
    const {count,fraction}=state,a=style.arc||arcDefaults(style,kind),capacity=a.repeat==='stacks'?a.limit:1;
    const around=a.layout==='around',slots=around?Math.min(4,capacity):1,layers=Math.ceil(capacity/slots);
    const radius=style.size*.43,center=style.size/2,fit=Math.min(1,(radius-1)/(a.thickness/2+(layers-1)*(a.thickness+a.gap))),thickness=a.thickness*fit,gap=a.gap*fit;
    const span=Math.min(359.9,around?(capacity<=2?180:90):360,a.sweep),sweep=span*Math.min(1,fraction),direction=a.reverse?-1:1;
    return Array.from({length:count},(_,i)=>{
      const r=radius-Math.floor(i/slots)*(thickness+gap),anchor=around?(a.pair==='vertical'?[270,90,180,0]:[180,0,270,90])[i%slots]+a.start:a.start+span/2;
      const start=(anchor-direction*span/2)*Math.PI/180,end=(anchor-direction*span/2+direction*sweep)*Math.PI/180;
      return {thickness,d:`M ${center+r*Math.cos(start)} ${center+r*Math.sin(start)} A ${r} ${r} 0 ${sweep>180?1:0} ${a.reverse?0:1} ${center+r*Math.cos(end)} ${center+r*Math.sin(end)}`};
    });
  }
  // Match the native overlay renderer's pixel bounds and top-left placement.
  const anchorKeys=['kind','target','point','selfPoint','x','y'];
  const anchorPoints={topleft:'Top left',top:'Top',topright:'Top right',left:'Left',center:'Center',right:'Right',bottomleft:'Bottom left',bottom:'Bottom',bottomright:'Bottom right'};
  const anchorUnits={current:'Current unit',player:'Player',pet:'Pet',target:'Target',focus:'Focus',...Object.fromEntries(Array.from({length:4},(_,i)=>['party'+(i+1),'Party '+(i+1)])),...Object.fromEntries(Array.from({length:40},(_,i)=>['raid'+(i+1),'Raid '+(i+1)])),...Object.fromEntries(Array.from({length:8},(_,i)=>['boss'+(i+1),'Boss '+(i+1)])),...Object.fromEntries(Array.from({length:5},(_,i)=>['arena'+(i+1),'Arena '+(i+1)])),...Object.fromEntries(Array.from({length:40},(_,i)=>['nameplate'+(i+1),'Nameplate '+(i+1)]))};
  function validAnchor(a){return a===undefined||a&&typeof a==='object'&&(a.kind==='frame'?typeof a.target==='string'&&/^[A-Za-z_][A-Za-z0-9_]{0,79}$/.test(a.target):['unit','nameplate'].includes(a.kind)&&(a.target==='current'||Object.hasOwn(anchorUnits,a.target)))&&Object.hasOwn(anchorPoints,a.point)&&Object.hasOwn(anchorPoints,a.selfPoint)&&[a.x,a.y].every(v=>Number.isFinite(v)&&v>=-4000&&v<=4000);}
  function anchorCount(rules){rules=Actions.rules(rules);return new Set(rules.filter(r=>r.enabled&&r.style.anchor).flatMap(r=>cloneUnits(ruleScopes(r.roots),r.style.anchor.target).map(u=>r.style.anchor.kind+':'+u))).size;}
  function anchorPoint(name){return {topleft:[0,0],top:[.5,0],topright:[1,0],left:[0,.5],center:[.5,.5],right:[1,.5],bottomleft:[0,1],bottom:[.5,1],bottomright:[1,1]}[name];}
  function placeAnchor(points,a,rect){
    if(!a)return points;if(!rect?.available||!points.length)return [];
    const x=Math.min(...points.map(p=>p.x)),y=Math.min(...points.map(p=>p.y)),w=Math.max(...points.map(p=>p.x+p.width))-x,h=Math.max(...points.map(p=>p.y+p.height))-y;
    const [px,py]=anchorPoint(a.point),[sx,sy]=anchorPoint(a.selfPoint),dx=rect.x+rect.width*px-w*sx+a.x-x,dy=rect.y+rect.height*py-h*sy+a.y-y;
    return points.map(p=>({...p,x:p.x+dx,y:p.y+dy}));
  }
  const groupKeys=['id','name','mode','sort','reverse','gap','columns','x','y'];
  function newGroup(name='My group',style={x:48,y:40}){return {id:id(),name,mode:'row',sort:'order',reverse:false,gap:8,columns:4,x:style.x,y:style.y};}
  function validGroups(rules){
    const groups=new Map(),anchors=new Map(),range=(v,a,b)=>Number.isFinite(v)&&v>=a&&v<=b;
    for(const r of rules){const g=r.style?.group;if(g===undefined)continue;
      if(!g||typeof g!=='object'||!/^[a-zA-Z0-9_-]{1,80}$/.test(g.id)||typeof g.name!=='string'||!g.name.trim()||g.name.length>60||!['static','row','column','grid','circle'].includes(g.mode)||!['order','name','remaining','stacks'].includes(g.sort)||typeof g.reverse!=='boolean'||!range(g.gap,0,200)||!Number.isInteger(g.columns)||!range(g.columns,1,20)||!range(g.x,0,100)||!range(g.y,0,100))return false;
      const old=groups.get(g.id);if(old&&groupKeys.some(k=>old[k]!==g[k]))return false;if(anchors.has(g.id)){const a=anchors.get(g.id),b=r.style.anchor;if(!!a!==!!b||a&&anchorKeys.some(k=>a[k]!==b[k]))return false;}anchors.set(g.id,r.style.anchor);groups.set(g.id,g);
    }return true;
  }
  function groupMembers(rules,rule){return rule?.style.group?rules.filter(r=>r.style.group?.id===rule.style.group.id):rule?[rule]:[];}
  function updateGroup(rules,rule,patch){for(const r of groupMembers(rules,rule))r.style.group={...r.style.group,...patch};}
  function groupLayout(items,g,width,height){
    let points=items.map((item,order)=>({...item,order:item.order??order,...displayGeometry(item.props,item.style),x:0,y:0}));if(!points.length)return points;
    points.sort((a,b)=>{let result=0;if(g.sort==='name')result=a.name.toLowerCase()<b.name.toLowerCase()?-1:a.name.toLowerCase()>b.name.toLowerCase()?1:0;
      if(['remaining','stacks'].includes(g.sort)){const va=a[g.sort],vb=b[g.sort];if(!Number.isFinite(va)&&Number.isFinite(vb))return 1;if(Number.isFinite(va)&&!Number.isFinite(vb))return -1;if(Number.isFinite(va)&&Number.isFinite(vb))result=va-vb;}
      result||=a.order-b.order||(String(a.id)<String(b.id)?-1:String(a.id)>String(b.id)?1:0);return g.reverse?-result:result;});
    const maxW=Math.max(...points.map(p=>p.width)),maxH=Math.max(...points.map(p=>p.height));let x=0,y=0;
    points.forEach((p,j)=>{if(g.mode==='row'){p.x=x;x+=p.width+g.gap;}else if(g.mode==='column'){p.y=y;y+=p.height+g.gap;}else if(g.mode==='grid'){p.x=j%g.columns*(maxW+g.gap);p.y=Math.floor(j/g.columns)*(maxH+g.gap);}else if(g.mode==='circle'&&points.length>1){const radius=(Math.hypot(maxW,maxH)+g.gap)/(2*Math.sin(Math.PI/points.length)),angle=j*2*Math.PI/points.length-Math.PI/2;p.x=radius*Math.cos(angle)-p.width/2;p.y=radius*Math.sin(angle)-p.height/2;}else if(g.mode==='static'){p.x=p.style.x*width/100;p.y=p.style.y*height/100;}});
    const minX=Math.min(...points.map(p=>p.x)),minY=Math.min(...points.map(p=>p.y)),w=Math.max(...points.map(p=>p.x+p.width))-minX,h=Math.max(...points.map(p=>p.y+p.height))-minY;
    const originX=g.mode==='static'?minX:Math.max(0,Math.min(width-w,g.x*width/100)),originY=g.mode==='static'?minY:Math.max(0,Math.min(height-h,g.y*height/100));
    return points.map(p=>({...p,x:p.x-minX+originX,y:p.y-minY+originY}));
  }
  function validSound(name,empty=false){return empty&&[undefined,''].includes(name)||['chime','alert','soft','sonar'].includes(name)||typeof name==='string'&&/^media:[a-f0-9]{64}$/.test(name);}
  function mediaReferences(rules){
    const ids=new Set(),sound=value=>{if(typeof value==='string'&&value.startsWith('media:'))ids.add(value.slice(6));};
    for(const r of rules){if(r.style.visual?.image)ids.add(r.style.visual.image);sound(r.style.onShow);sound(r.style.onHide);for(const m of r.style.modifiers||[])sound(m.sound);for(const n of flatten([...r.roots,...Values.inputs(r)]))if(n.type==='sound')sound(n.props.sound);}
    return [...ids].sort();
  }
  const visualKeys=['motion','sprite','fontFamily','bold','italic','textEffect','image','fit','tint','width','height','rotation','flipX','flipY','layer','shape','direction','trackColor','trackOpacity','fillOpacity','textAlign','barIcon'];
  const spriteKeys=['columns','rows','frames','start','fps','mode','reverse'];
  const spriteModes={loop:'Loop',once:'Play once',bounce:'Back and forth',progress:'Follow progress'};
  function validSprite(s){
    return s==null||typeof s==='object'&&!Array.isArray(s)&&['columns','rows','frames','start'].every(k=>Number.isInteger(s[k]))&&s.columns>=1&&s.columns<=64&&s.rows>=1&&s.rows<=64&&s.frames>=1&&s.frames<=256&&s.start>=0&&s.start+s.frames<=s.columns*s.rows&&Number.isFinite(s.fps)&&s.fps>=1&&s.fps<=60&&Object.hasOwn(spriteModes,s.mode)&&(s.reverse===undefined||typeof s.reverse==='boolean');
  }
  function spriteFits(s,asset){return !s||validSprite(s)&&!!asset&&s.columns<=asset.width&&s.rows<=asset.height;}
  function spriteFrame(s,elapsed,fraction=1){
    let frame=Math.floor(Math.max(0,elapsed)*s.fps);
    if(s.mode==='progress')frame=Math.min(s.frames-1,Math.floor(Math.max(0,Math.min(1,fraction))*s.frames));
    else if(s.mode==='once')frame=Math.min(s.frames-1,frame);
    else if(s.mode==='bounce'){const period=Math.max(1,s.frames*2-2);frame%=period;if(frame>=s.frames)frame=period-frame;}
    else frame%=s.frames;
    return s.start+(s.reverse?s.frames-1-frame:frame);
  }
  function spriteRect(s,asset,elapsed,fraction){
    if(!spriteFits(s,asset))return null;
    const frame=spriteFrame(s,elapsed,fraction),col=frame%s.columns,row=Math.floor(frame/s.columns),x=Math.floor(col*asset.width/s.columns),y=Math.floor(row*asset.height/s.rows);
    return {frame,x,y,width:Math.floor((col+1)*asset.width/s.columns)-x,height:Math.floor((row+1)*asset.height/s.rows)-y};
  }
  const fontFamilies={ui:'Segoe UI',arial:'Arial',georgia:'Georgia',consolas:'Consolas'};
  const shapes={rectangle:'Rectangle',circle:'Circle',diamond:'Diamond',triangle:'Triangle',arrow:'Arrow',cross:'Cross',star:'Star'};
  // The same 100-unit geometry is used in the native renderer; progress clips it.
  const shapePaths={rectangle:'M0,0 L100,0 100,100 0,100 Z',circle:'M50,0 A50,50 0 1 1 50,100 A50,50 0 1 1 50,0 Z',diamond:'M50,0 L100,50 50,100 0,50 Z',triangle:'M50,0 L100,100 0,100 Z',arrow:'M0,30 L60,30 60,0 100,50 60,100 60,70 0,70 Z',cross:'M35,0 L65,0 65,35 100,35 100,65 65,65 65,100 35,100 35,65 0,65 0,35 35,35 Z',star:'M50,0 L62,35 100,38 70,61 81,100 50,77 19,100 30,61 0,38 38,35 Z'};
  function validVisual(v){
    if(v===undefined||v===null)return true;
    if(typeof v!=='object'||Array.isArray(v)||!validSprite(v.sprite)||!Motion.valid(v.motion,colorValue))return false;
    const range=(k,lo,hi)=>v[k]===undefined||Number.isFinite(v[k])&&v[k]>=lo&&v[k]<=hi;
    return [undefined,'',...Object.keys(fontFamilies)].includes(v.fontFamily)&&[undefined,'','shadow','outline','none'].includes(v.textEffect)&&['width','height'].every(k=>v[k]===0||range(k,8,2048))&&range('rotation',-360,360)&&range('layer',-100,100)&&(v.layer===undefined||Number.isInteger(v.layer))&&(v.image===undefined||v.image===''||typeof v.image==='string'&&/^[a-f0-9]{64}$/.test(v.image))&&[undefined,'','contain','cover','stretch'].includes(v.fit)&&['flipX','flipY','tint','bold','italic'].every(k=>v[k]===undefined||typeof v[k]==='boolean')&&[undefined,'',...Object.keys(shapes)].includes(v.shape)&&[undefined,'','right','left','up','down'].includes(v.direction)&&(v.trackColor===undefined||typeof v.trackColor==='string'&&(!v.trackColor||!!colorValue(v.trackColor)))&&range('trackOpacity',0,100)&&range('fillOpacity',0,100)&&[undefined,'','left','center','right'].includes(v.textAlign)&&[undefined,'','none','left','right','top','bottom'].includes(v.barIcon);
  }
  function visualProgress(fraction,direction='right',width=100,height=100){
    const f=Math.max(0,Math.min(1,fraction));
    if(direction==='up')return {x:0,y:height*(1-f),width,height:height*f};
    if(direction==='down')return {x:0,y:0,width,height:height*f};
    return {x:direction==='left'?width*(1-f):0,y:0,width:width*f,height};
  }
  function visualBarLayout(geometry,visual={}){
    const {width,height}=geometry,side=visual.barIcon||'none',vertical=['top','bottom'].includes(side),size=side==='none'?0:Math.min(width,height,(vertical?height:width)/2);
    return {icon:size,side,left:side==='left'?size:0,top:side==='top'?size:0,right:side==='right'?size:0,bottom:side==='bottom'?size:0};
  }
  function displayGeometry(props,style) {
    const g=['text','bar'].includes(props.display)?{width:Math.max(style.size*3,String(props.text||'').length*style.font),height:Math.max(32,style.font*2)}:{width:style.size,height:style.size};
    if(style.visual?.width)g.width=style.visual.width;if(style.visual?.height)g.height=style.visual.height;return g;
  }
  function previewPosition(style,geometry,width,height) {
    return {x:Math.max(0,Math.min(width-geometry.width,style.x*width/100)),y:Math.max(0,Math.min(height-geometry.height,style.y*height/100))};
  }

  const valueSourceKeys=['dataSource','dataID','dataUnit','dataResource','dataProvider','dataText','dataKind','dataTimelineSource','dataTalentKind'];
  const valueAdapters={validWhen,validData:(p,field)=>p&&typeof p==='object'&&!Array.isArray(p)&&Object.keys(p).every(k=>valueSourceKeys.includes(k))&&!['auto','value',undefined,''].includes(p.dataSource)&&validShowData(p)&&Object.hasOwn(dataTokens,field),validCheck:p=>validPredicate(p)&&predicateList(p).every(q=>q.condition!=='value')};
  function valueEvaluation(logic,sample,names={}){
    return Values.evaluation(logic,{memory:sample.overlayMemory,check:p=>condition({props:p},sample),data:(props,field)=>{
      const d=resolvedData(explicitDataSource(props),sample,names);if(Object.hasOwn(d.numbers,field))return d.numbers[field];
      if(['name','character','realm','unit','unitName','role','sourceName','targetName','extraName','event','miss','encounterName','outcome','provider'].includes(field))return d.text[field]&&d.text[field]!=='—'?d.text[field]:null;
      if(['spellID','extraSpellID','encounterID','difficulty','groupSize','timerID','rank'].includes(field)&&d.text[field]!==undefined&&d.text[field]!=='')return Values.scalar(Number(d.text[field]));return null;
    }});
  }
  const api = { limits,usage,Packs,callChoices,Actions,executionRule:Actions.executionRule,copyRoots:roots=>Actions.copies(roots,id),validWhen,Values,valueAdapters,valueEvaluation,explicitDataSource,validPredicate, displayState,anchorPoint,Motion,spriteKeys,spriteModes,validSprite,spriteFits,spriteFrame,spriteRect,fontFamilies, validSound, mediaReferences, visualKeys, shapes, shapePaths, validVisual, visualProgress, visualBarLayout, buildLoadMatches,equipmentKinds,equipmentSlots,equipmentTypes,validEquipment,equipmentSetNames,equipmentCondition,talentKinds,talentWatch,validTalent,talentReading,talentCondition,talentWatches,bossEvents,bossProviders,timelineSources,bossKinds,bossCondition,bossExample,bossQuery,bossTimerSource,encounterEvents,difficulties,encounterExample,encounterCondition,combatKinds,combatValues,combatCount,combatExample,triggerSample,castEvents, castCount, castCondition, castExample, validUnit, unitContext, measurementCount, unitChoices:anchorUnits, cloneScopes,scopeTokens,ruleScopes,scopeMask,cloneMembers,nodeScope,partyTokens, hasEach, cloneUnits, cloneContext, repeatedNode, clonesExample, anchorKeys, anchorPoints, anchorUnits, validAnchor, anchorCount, placeAnchor, newGroup, validGroups, groupMembers, updateGroup, groupLayout, groupKeys, palette, colorValue, resolveAppearance, logicExample, predicateList, condition, liveExample, dataKinds, dataTokens, dataSource, validShowData, resolveShow, cooldownCount, cooldownCondition, cooldownExample, arcDefaults, validArc, arcExample, arcState, arcPaths, displayGeometry, previewPosition, displayMatches, resources, id, clone, definitions, node, newRule, examples, auraExample, manaExample, capExample, nestedExample, createSession, find, flatten, copyNode, insert, evaluate, validRules, watchCount };
  root.OverlayModel = api;
  if (typeof module !== 'undefined') module.exports = api;
})(typeof window !== 'undefined' ? window : globalThis);
