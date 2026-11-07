local _,LUI=...
local O={seq=0,cast=0,castSeq=0,clips={},containers={}}
LUI.Overlays=O
local function public(v) return not issecretvalue or not issecretvalue(v) end
local function value(v,fallback) if public(v)and v~=nil then return v end;return fallback end
local function text(v) v=value(v,"");return type(v)=="string"and v or ""end
local function json(v)
    local kind=type(v)
    if kind=="string"then return '"'..v:gsub('[%z\1-\31\\"]',function(c)if c=='"'or c=='\\'then return '\\'..c end;return string.format('\\u%04x',c:byte())end)..'"'end
    if kind=="number"then return tostring(v)end
    if kind=="boolean"then return v and "true"or "false"end
    local out={};for k,x in pairs(v)do out[#out+1]=json(k)..":"..json(x)end;return "{"..table.concat(out,",").."}"
end
local function place(frame,x,y)
    O.points=O.points or {};O.points[#O.points+1]={frame,x,y}
    local px,py=O.strip:MapPixel(x,y+32)
    frame:SetPoint("TOPLEFT",O.root,"TOPLEFT",px,-py)
end
function O:Layout()
    if not self.root then return end
    self.root:SetSize(self.strip.width,self.strip.height)
    for _,point in ipairs(self.points or {})do
        local x,y=self.strip:MapPixel(point[2],point[3]+32)
        point[1]:ClearAllPoints();point[1]:SetPoint("TOPLEFT",self.root,"TOPLEFT",x,-y)
    end
end
-- Compact patches retain the legacy sampled inner 2x2 pixels. Their fixed
-- banks fit the same native panel; ordinary configurations keep the old layout.
local function densePoint(cell)return (cell%128)*2,math.floor(cell/128)*2 end
local function auraPoint(index,copy,field)
    if O.compact then return densePoint(144+(index-1)*8+copy*4+field)end
    return ((index-1)%8)*32+copy*16+field*4,8+math.floor((index-1)/8)*16
end
local function timerPoint(index,field,copy)
    local cell=index*16+field*2+copy
    if O.compact then return densePoint(656+cell)end
    return (cell%64)*4,({4,16,20,32})[math.floor(cell/64)+1]
end
local function healthPoint(copy)
    if O.compact then return densePoint(8+copy)end
    return 40+copy*4,0
end
local function resourcePoint(power,field)
    if O.compact then return densePoint(16+power*4+field)end
    return (power%16)*16+field*4,12+math.floor(power/16)*16
end
local function patch(parent,x,y,native)
    local size=O.compact and 2 or 4
    local clip=CreateFrame("ScrollFrame",nil,parent);clip:SetSize(size,size);place(clip,x,y)
    clip:SetFrameStrata("TOOLTIP");clip:SetFrameLevel(9002);clip:SetClipsChildren(true);clip:EnableMouse(false)
    local canvas=CreateFrame("Frame",nil,clip);canvas:SetSize(96,96);canvas:SetFrameLevel(9003);clip:SetScrollChild(canvas)
    local font=canvas:CreateFontString(nil,"OVERLAY");font:SetFont("Interface\\AddOns\\LamdaFrames\\Fonts\\RobotoMono-Bold.ttf",64,"")
    font:SetPoint("CENTER");font:SetSize(96,96);font:SetJustifyH("CENTER");font:SetJustifyV("MIDDLE");font:SetTextColor(1,1,1,1);font:SetShadowOffset(0,0);font:SetWordWrap(false);font:SetNonSpaceWrap(false)
    O.clips[#O.clips+1]=clip
    local function scroll(self)self:UpdateScrollChildRect();self:SetHorizontalScroll(48+(4-size)/2);self:SetVerticalScroll(46+(4-size)/2)end
    -- Native aura descendants forbid addon visibility scripts. Their geometry
    -- is initialized once, before Blizzard seals the button's descendants.
    scroll(clip)
    if not native then clip:SetScript("OnShow",scroll)end
    return font
end
local function tile(parent,x,y,color)
    local t=parent:CreateTexture(nil,"OVERLAY");local size=O.compact and 2 or 4;t:SetSize(size,size);place(t,x,y)
    t:SetColorTexture(math.floor(color/65536)%256/255,math.floor(color/256)%256/255,color%256/255,1)
end
-- Attachment coordinates are public presentation data, separate from native
-- health/aura sinks. An unavailable frame never reuses its previous rectangle.
local function anchorValue(fn,kind,...)
    if type(fn)~="function"then return nil end
    local ok,v=pcall(fn,...)
    if not ok or not public(v)or type(v)~=kind then return nil end
    if kind=="number"and(v~=v or v==math.huge or v==-math.huge)then return nil end
    return v
end
-- Only the player's active main pet, not guardians or Animal Companion. Public
-- identity/context queries must not turn a restricted/error result into absence.
function O:PetData(class)
    local p={exists=anchorValue(UnitExists,"boolean","pet")}
    if p.exists==nil then return p end
    local dead=anchorValue(UnitIsDeadOrGhost,"boolean","pet")
    -- A dead hunter pet can lose its unit when the corpse disappears. Remember
    -- only a public, confirmed death until a live main pet returns. Dismissing a
    -- living pet never sets this flag; demons/ghouls still need a new summon.
    if class=="HUNTER"then
        if dead==true then self.petNeedsRevive=true
        elseif p.exists and dead==false then self.petNeedsRevive=nil end
        if self.petNeedsRevive and (not p.exists or dead==true)then p.revivable=true end
    else self.petNeedsRevive=nil end
    if p.exists~=true then return p end
    if dead~=nil then p.alive=not dead end
    local function label(fn,...)
        local s=anchorValue(fn,"string",...)
        if s and #s<=160 and not s:find('[%z\1-\31]')then return s end
    end
    p.name=label(UnitName,"pet")
    p.family=label(UnitCreatureFamily,"pet")
    p.creatureType=label(UnitCreatureType,"pet")
    p.specialization=label(C_PetInfo and C_PetInfo.GetPetTalentTree)
    return p
end
function O:AnchorRows(width,height)
    local watches=(LamdaOverlaysData and LamdaOverlaysData.anchors)or {}
    local needUnits=false;for _,w in ipairs(watches)do if w.kind=="unit"then needUnits=true end end
    local F=LUI.FrameAuras
    if needUnits and F and (not self.anchorFrames or GetTime()-(self.anchorScanAt or 0)>=1)then
        local ok,frames=pcall(F.Discover,F,{nativePlayer=true,nativeParty=true,nativeRaid=true,nativePets=true})
        self.anchorFrames=ok and frames or {};self.anchorScanAt=GetTime()
    end
    local function matches(frame,unit)
        local found=F and F:UnitForFrame(frame)
        return found and (found==unit or anchorValue(UnitIsUnit,"boolean",found,unit)==true)
    end
    local function read(w)
        local frame
        if w.kind=="frame"then frame=_G[w.target]
        elseif w.kind=="nameplate"then frame=C_NamePlate and C_NamePlate.GetNamePlateForUnit(w.target)
        elseif w.kind=="unit"then
            if anchorValue(UnitExists,"boolean",w.target)~=true or anchorValue(UnitIsVisible,"boolean",w.target)~=true then return end
            for _,candidate in ipairs(self.anchorFrames or {})do
                if matches(candidate.frame,w.target)and anchorValue(candidate.frame.IsVisible,"boolean",candidate.frame)==true then frame=candidate.frame;break end
            end
            if not frame then frame=({player=PlayerFrame,pet=PetFrame,target=TargetFrame,focus=FocusFrame})[w.target]end
            if frame and not matches(frame,w.target)then return end
        end
        if not public(frame)or not frame or anchorValue(frame.IsForbidden,"boolean",frame)~=false or anchorValue(frame.IsVisible,"boolean",frame)~=true then return end
        local factor=PixelUtil and anchorValue(PixelUtil.GetPixelToUIUnitFactor,"number")
        local scale=anchorValue(UIParent.GetEffectiveScale,"number",UIParent)
        local parentLeft=anchorValue(UIParent.GetLeft,"number",UIParent);local parentTop=anchorValue(UIParent.GetTop,"number",UIParent)
        local effective=anchorValue(frame.GetEffectiveScale,"number",frame)
        local left=anchorValue(frame.GetLeft,"number",frame);local top=anchorValue(frame.GetTop,"number",frame)
        local wdt=anchorValue(frame.GetWidth,"number",frame);local hgt=anchorValue(frame.GetHeight,"number",frame)
        if not factor or factor<=0 or not scale or scale<=0 or not parentLeft or not parentTop or not effective or effective<=0 or not left or not top or not wdt or not hgt then return end
        local x=left*effective/factor-parentLeft*scale/factor;local y=parentTop*scale/factor-top*effective/factor
        wdt=wdt*effective/factor;hgt=hgt*effective/factor
        if x<0 or y<0 or wdt<=0 or hgt<=0 or x+wdt>width+1 or y+hgt>height+1 then return end
        if w.kind=="unit"and not matches(frame,w.target)then return end
        return {kind=w.kind,target=w.target,available=true,x=math.floor(x+.5),y=math.floor(y+.5),width=math.floor(wdt+.5),height=math.floor(hgt+.5)}
    end
    local rows={}
    for i,w in ipairs(watches)do
        if i>64 then break end
        local ok,result=pcall(read,w)
        rows[#rows+1]=json(ok and result or {kind=w.kind,target=w.target,available=false})
    end
    return table.concat(rows,",")
end

function O:UnitRows(includeNames)
    if not (LamdaOverlaysData and LamdaOverlaysData.units)then return ""end
    if type(LamdaOverlaysData.unitTokens)=="table"then
        local rows={}
        for i,unit in ipairs(LamdaOverlaysData.unitTokens)do
            if i>98 then break end
            local generation=self:UnitGeneration(unit)
            if anchorValue(UnitExists,"boolean",unit)==true then
                local id=anchorValue(UnitGUID,"string",unit)
                local kind=id and id:match("^(%a+)%-")
                if not id or #id>122 or not id:match("^%a+%-%w[%w%-]*$")or not ({Player=true,Creature=true,Pet=true,Vehicle=true})[kind or ""]then id="Slot-"..unit.."-"..generation end
                local name=includeNames and anchorValue(UnitFullName,"string",unit)or ""
                if name and #name>120 then name=""end
                local role=anchorValue(UnitGroupRolesAssigned,"string",unit);role=role and role:lower()or ""
                if role=="damager"then role="damage"end
                if role~="damage"and role~="tank"and role~="healer"then role=""end
                rows[#rows+1]=json({unit=unit,id=id,g=generation,name=name or "",role=role})
            end
        end
        return table.concat(rows,",")
    end
    local rows,seen={},{}
    for _,unit in ipairs({"player","party1","party2","party3","party4"})do
        if anchorValue(UnitExists,"boolean",unit)==true then
            local id=anchorValue(UnitGUID,"string",unit)
            if id and #id<=108 and id:match("^Player%-%w[%w%-]*$")and not seen[id]then
                local name=includeNames and anchorValue(UnitFullName,"string",unit)or ""
                if name and #name>120 then name=""end
                local role=anchorValue(UnitGroupRolesAssigned,"string",unit);role=role and role:lower()or ""
                if role=="damager"then role="damage"end
                if role~="damage"and role~="tank"and role~="healer"then role=""end
                rows[#rows+1]=json({unit=unit,id=id,name=name or "",role=role});seen[id]=true
            end
        end
    end
    return table.concat(rows,",")
end

function O:UnitGeneration(unit)
    self.unitGenerations=self.unitGenerations or {}
    local exists=anchorValue(UnitExists,"boolean",unit)==true
    local id=exists and (anchorValue(UnitGUID,"string",unit)or "present")or "missing"
    local state=self.unitGenerations[unit]
    if not state then state={id=id,generation=1};self.unitGenerations[unit]=state
    elseif state.id~=id then state.id=id;state.generation=state.generation+1 end
    return state.generation
end
function O:UnitChanged(unit)
    local state=self.unitGenerations and self.unitGenerations[unit]
    if state then state.generation=state.generation+1 end
end

function O:IdentityRows()
    if not (LamdaOverlaysData and LamdaOverlaysData.combat)then return ""end
    local rows={}
    for i,unit in ipairs(LamdaOverlaysData.identities or {})do
        if i>8 then break end
        local id=anchorValue(UnitExists,"boolean",unit)==true and anchorValue(UnitGUID,"string",unit)or ""
        local kind=id and id:match("^(%a+)%-")
        if not id or #id>122 or not id:match("^%a+%-%w[%w%-]*$")or not ({Player=true,Creature=true,Pet=true,Vehicle=true,GameObject=true})[kind or ""]then id=""end
        rows[#rows+1]="["..json(unit)..","..json(id)..","..self:UnitGeneration(unit).."]"
    end
    return table.concat(rows,",")
end

local function encounterInteger(v,lo,hi)
    if public(v)and type(v)=="number"and v==v and v>=lo and v<=hi and v==math.floor(v)then return v end
    return 0
end
local function encounterName(v)
    v=text(v);return #v<=160 and v or ""
end
function O:EncounterState()
    if not self.encounter then self.encounter={id=0,name="",difficulty=0,groupSize=0,seq=0,events={}}end
    return self.encounter
end
function O:EncounterEvent(event,id,name,difficulty,groupSize,success)
    if event=="PLAYER_ENTERING_WORLD"then self.encounter=nil;return end
    if not (LUI:DB().overlaysEnabled and LamdaOverlaysData and LamdaOverlaysData.encounters)then return end
    local kinds={ENCOUNTER_START="encounterstart",ENCOUNTER_END="encounterend",BOSS_KILL="bosskill"}
    if not kinds[event]and event~="ENCOUNTER_STATE_CHANGED"then return end
    local e=self:EncounterState();local now=GetTime()
    if event=="ENCOUNTER_STATE_CHANGED"then
        if public(id)and type(id)=="boolean"then
            if id and e.active~=true then e.id=0;e.name="";e.started=nil;e.groupSize=0 end
            e.active=id;e.changedAt=now;e.publicEvent=true
            if not id then e.started=nil end
        end
        return
    end
    local info={id=encounterInteger(id,1,9999999),name=encounterName(name),difficulty=encounterInteger(difficulty,1,1000),groupSize=encounterInteger(groupSize,1,100)}
    if event=="BOSS_KILL"then
        if info.id>0 and info.id==e.id then info.difficulty=e.difficulty;info.groupSize=e.groupSize end
        local d=anchorValue(function()if type(GetInstanceInfo)=="function"then local _,_,v=GetInstanceInfo();return v end end,"number")
        if d then info.difficulty=encounterInteger(d,1,1000)end
    end
    if event=="ENCOUNTER_START"then info.elapsed=0
    elseif e.started and e.id>0 and e.id==info.id and now>=e.started and now-e.started<=86400 then info.elapsed=now-e.started end
    if event=="ENCOUNTER_START"or event=="ENCOUNTER_END"then
        e.id=info.id;e.name=info.name;e.difficulty=info.difficulty;e.groupSize=info.groupSize
        e.active=event=="ENCOUNTER_START";e.started=e.active and now or nil;e.changedAt=now;e.publicEvent=true
        if event=="ENCOUNTER_END"and public(success)and type(success)=="number"and(success==0 or success==1)then info.success=success==1 end
    end
    e.seq=e.seq+1;info.seq=e.seq;info.kind=kinds[event];info.at=now
    e.events[#e.events+1]=info;if #e.events>4 then table.remove(e.events,1)end
end
function O:EncounterData()
    if not (LamdaOverlaysData and LamdaOverlaysData.encounters)then return nil end
    local e=self:EncounterState();local now=GetTime()
    local fn=C_InstanceEncounter and C_InstanceEncounter.IsEncounterInProgress or IsEncounterInProgress
    local active=anchorValue(fn,"boolean")
    if active~=nil then
        if active==e.active then e.changedAt=nil
        elseif not e.changedAt or now-e.changedAt>=2 then
            e.active=active;e.id=0;e.name="";e.started=nil;e.groupSize=0;e.changedAt=nil;e.publicEvent=nil
        end
    elseif not e.publicEvent then e.active=nil;e.id=0;e.name="";e.started=nil;e.groupSize=0
    end
    local diff=anchorValue(function()if type(GetInstanceInfo)=="function"then local _,_,d=GetInstanceInfo();return d end end,"number")
    if diff and diff>=1 and diff<=1000 then e.difficulty=math.floor(diff)end
    local elapsed=e.active and e.started and now-e.started or nil
    if elapsed and(elapsed<0 or elapsed>86400)then elapsed=nil end
    local rows,kept={},{}
    for _,v in ipairs(e.events)do
        local age=math.floor((now-v.at)*1000)
        if age>=0 and age<=3000 then
            kept[#kept+1]=v
            rows[#rows+1]=json({seq=v.seq,kind=v.kind,id=v.id,name=v.name,difficulty=v.difficulty,groupSize=v.groupSize,success=v.success,elapsed=v.elapsed,age=age})
        end
    end
    e.events=kept
    local body=json({active=e.active,id=e.id,name=e.name,difficulty=e.difficulty,groupSize=e.groupSize,elapsed=elapsed,seq=e.seq})
    return body:sub(1,-2)..',"events":['..table.concat(rows,",")..']}'
end

-- Boss-mod callbacks carry public observations only. Opaque callback arguments
-- never become keys or arithmetic inputs. Native timeline sources are separate.
local function bossNumber(v,maximum)
    if public(v)and type(v)=="number"and v==v and v>=0 and v<=(maximum or 86400)then return v end
end
local function bossText(v,limit)
    v=text(v);if #v<=(limit or 160)then return v end;return ""
end
local function bossName(v)
    v=bossText(v,512):gsub("|c%x%x%x%x%x%x%x%x",""):gsub("|r",""):gsub("|T.-|t",""):gsub("|A.-|a",""):gsub("|H.-|h(.-)|h","%1"):gsub("||","|")
    return #v<=160 and v or ""
end
local function bossOwner(v)
    if public(v)and type(v)=="number"and bossNumber(v,9999999)then return tostring(v)end
    return bossText(v,160)
end
local function bossSpell(v)
    if public(v)and type(v)=="string"then v=tonumber(v)end
    v=bossNumber(v,9999999);if v and v>0 and v==math.floor(v)then return v end
end
local function bossBoolean(v)if public(v)and type(v)=="boolean"then return v end end
local function bossEnabled()return LUI:DB().overlaysEnabled and LamdaOverlaysData and LamdaOverlaysData.bosses end
local function bossKind(v)
    if not public(v)or v==nil then return ""end
    v=bossText(v,32)
    if v=="cast"or v=="castnp"then return "cast"end
    if v=="pull"or v=="break"then return v end
    return "timer"
end
function O:BossState()
    if not self.bosses then
        local progress=anchorValue(C_InstanceEncounter and C_InstanceEncounter.IsEncounterInProgress or IsEncounterInProgress,"boolean")
        self.bosses={providers={dbm={name="dbm",available=false,complete=progress==false},bigwigs={name="bigwigs",available=false,complete=progress==false},blizzard={name="blizzard",available=false,complete=false}},timers={},events={},modules={},moduleSerial=0}
    end
    return self.bosses
end
function O:BossKey(provider,key,owner)
    key=bossText(key,512);if key==""then self:BossState().providers[provider].complete=false;return nil end
    if provider=="bigwigs"then
        if not public(owner)then self:BossState().providers[provider].complete=false;return nil end
        local s=self:BossState();local ownerID=0
        if owner~=nil then
            if not s.modules[owner]then s.moduleSerial=s.moduleSerial+1;s.modules[owner]=s.moduleSerial end
            ownerID=s.modules[owner]
        end
        key=ownerID..":"..key
    end
    return provider..":"..key
end
function O:BossSnapshot(t,now)
    local remaining=t.remaining
    if not t.paused and t.ends then remaining=math.max(0,t.ends-now)end
    return {provider=t.provider,id=t.id,source=t.source,name=t.name,spell=t.spell,kind=t.kind,count=t.count,duration=t.duration,remaining=remaining,paused=t.paused}
end
function O:BossEmit(kind,t,extra)
    local s=self:BossState();self.bossSeq=(self.bossSeq or 0)+1
    local e={seq=self.bossSeq,kind=kind,at=GetTime(),timer=self:BossSnapshot(t,GetTime())}
    if extra then e.stage=extra.stage;e.stageTotal=extra.stageTotal;e.encounterID=extra.encounterID end
    s.events[#s.events+1]=e;if #s.events>8 then table.remove(s.events,1)end
end
function O:BossStart(provider,key,owner,name,spell,duration,kind,count,keep,silent)
    local s=self:BossState();key=self:BossKey(provider,key,owner);if not key then return end
    if s.timers[key]then self:BossEmit("bossstop",s.timers[key]);s.timers[key]=nil end
    local total=0;for _ in pairs(s.timers)do total=total+1 end
    if total>=128 then s.providers[provider].complete=false;s.dropped=true;return end
    self.bossTimerID=(self.bossTimerID or 0)+1
    if not public(keep)then s.providers[provider].complete=false end
    local d=bossNumber(duration)
    local t={provider=provider,id=self.bossTimerID,name=bossName(name),spell=bossSpell(spell),kind=bossKind(kind),count=bossNumber(count,1000000),duration=d,ends=d and GetTime()+d or nil,paused=false,keep=bossBoolean(keep)==true,owner=owner}
    s.timers[key]=t;if not silent then self:BossEmit("bossstart",t)end;return t
end
function O:BossChange(provider,key,owner,kind,elapsed,duration)
    local s=self:BossState();key=self:BossKey(provider,key,owner);if not key then return end
    local t=s.timers[key];if not t then return end
    local now=GetTime()
    if kind=="bossstop"then
        if not t.expired and not t.paused and t.ends and t.ends<=now then self:BossEmit("bossexpire",t);t.expired=true end
        self:BossEmit(kind,t);s.timers[key]=nil;return
    elseif kind=="bosspause"then
        if t.paused then return end
        t.remaining=t.ends and math.max(0,t.ends-now)or nil;t.ends=nil;t.paused=true
    elseif kind=="bossresume"then
        if not t.paused then return end
        t.ends=t.remaining and now+t.remaining or nil;t.remaining=nil;t.paused=false
    elseif kind=="bossupdate"then
        local d,e=bossNumber(duration),bossNumber(elapsed)
        t.duration=d;local remaining=d and e and math.max(0,d-e)or nil
        t.remaining=t.paused and remaining or nil;t.ends=not t.paused and remaining and now+remaining or nil;t.expired=false
    end
    self:BossEmit(kind,t)
end
function O:BossClear(provider,owner)
    local s=self:BossState()
    for key,t in pairs(s.timers)do
        if t.provider==provider and(owner==nil or rawequal(t.owner,owner))then self:BossEmit("bossstop",t);s.timers[key]=nil end
    end
end
function O:BossStage(provider,stage,total,encounterID,emit)
    local p=self:BossState().providers[provider]
    p.stage=bossNumber(stage,10000);p.stageTotal=bossNumber(total,1000000);p.encounterID=bossSpell(encounterID)
    if emit then self:BossEmit("bossstage",{provider=provider,id=0},p)end
end
function O:DBMBossCallback(event,...)
    if not bossEnabled()then return end
    local s=self:BossState();if not s.providers.dbm.available then return end
    if event=="DBM_TimerBegin"then
        local id,name,duration,_,kind,spell,_,owner,keep,_,_,_,count=...
        self:BossStart("dbm",id,bossOwner(owner),name,spell,duration,kind,count,keep)
    elseif event=="DBM_TimerUpdate"then
        local id,elapsed,duration=...;self:BossChange("dbm",id,nil,"bossupdate",elapsed,duration)
    elseif event=="DBM_TimerStop"or event=="DBM_TimerPause"or event=="DBM_TimerResume"then
        self:BossChange("dbm",...,nil,({DBM_TimerStop="bossstop",DBM_TimerPause="bosspause",DBM_TimerResume="bossresume"})[event])
    elseif event=="DBM_TimerUpdateName"then
        local id,name=...;local key=self:BossKey("dbm",id);local t=key and s.timers[key];if t then t.name=bossName(name);self:BossEmit("bossupdate",t)end
    elseif event=="DBM_Announce"then
        local name,_,_,spell,_,_,count=...;self:BossEmit("bosswarning",{provider="dbm",id=0,name=bossName(name),spell=bossSpell(spell),count=bossNumber(count,1000000)})
    elseif event=="DBM_SetStage"then
        local _,_,stage,encounterID,total=...;self:BossStage("dbm",stage,total,encounterID,true)
    elseif event=="DBM_Pull"then
        s.providers.dbm.complete=not s.dropped
    elseif event=="DBM_Wipe"or event=="DBM_Kill"then
        local mod=...;local owner
        if public(mod)and type(mod)=="table"then owner=bossOwner(mod.id)end
        -- If the module cannot be identified, clear this provider on its explicit end.
        self:BossClear("dbm",owner~=""and owner or nil);s.providers.dbm.complete=not s.dropped
        self:BossStage("dbm",0,0,nil,true)
    end
end
function O:BigWigsBossCallback(event,...)
    if not bossEnabled()then return end
    local s=self:BossState();if not s.providers.bigwigs.available then return end
    if event=="BigWigs_Timer"or event=="BigWigs_TargetTimer"or event=="BigWigs_CastTimer"then
        local owner,spell,duration,_,name,count=...;self:BossStart("bigwigs",name,owner,name,spell,duration,event=="BigWigs_Timer"and "timer"or "cast",count,false)
    elseif event=="BigWigs_StartPull"or event=="BigWigs_StartBreak"then
        local owner,duration,_,name,_,breakName=...;if event=="BigWigs_StartBreak"then name=breakName end
        self:BossStart("bigwigs",name,owner,name,nil,duration,event=="BigWigs_StartPull"and "pull"or "break",nil,false)
    elseif event=="BigWigs_StopBar"or event=="BigWigs_PauseBar"or event=="BigWigs_ResumeBar"then
        local owner,name=...;self:BossChange("bigwigs",name,owner,({BigWigs_StopBar="bossstop",BigWigs_PauseBar="bosspause",BigWigs_ResumeBar="bossresume"})[event])
    elseif event=="BigWigs_StopBars"or event=="BigWigs_OnBossDisable"or event=="BigWigs_OnPluginDisable"or event=="BigWigs_OnBossWipe"or event=="BigWigs_OnBossWin"then
        local owner=...;if public(owner)then self:BossClear("bigwigs",owner)else s.providers.bigwigs.complete=false end
        if event=="BigWigs_OnBossWipe"or event=="BigWigs_OnBossWin"then self:BossStage("bigwigs",0,0,nil,true);s.providers.bigwigs.complete=not s.dropped end
    elseif event=="BigWigs_OnBossEngage"then
        s.providers.bigwigs.complete=not s.dropped
    elseif event=="BigWigs_Message"then
        local _,spell,name=...;self:BossEmit("bosswarning",{provider="bigwigs",id=0,name=bossName(name),spell=bossSpell(spell)})
    elseif event=="BigWigs_SetStage"then
        local _,stage=...;self:BossStage("bigwigs",stage,nil,nil,true)
    end
end
local dbmBossEvents={"DBM_TimerBegin","DBM_TimerUpdate","DBM_TimerStop","DBM_TimerPause","DBM_TimerResume","DBM_TimerUpdateName","DBM_Announce","DBM_SetStage","DBM_Pull","DBM_Wipe","DBM_Kill"}
local bigWigsBossEvents={"BigWigs_Timer","BigWigs_TargetTimer","BigWigs_CastTimer","BigWigs_StartPull","BigWigs_StartBreak","BigWigs_StopBar","BigWigs_PauseBar","BigWigs_ResumeBar","BigWigs_StopBars","BigWigs_OnBossDisable","BigWigs_OnPluginDisable","BigWigs_OnBossWipe","BigWigs_OnBossWin","BigWigs_OnBossEngage","BigWigs_Message","BigWigs_SetStage"}
function O:BossRegister()
    local s=self:BossState();self.bossHooks=self.bossHooks or {}
    for _,name in ipairs({"dbm","bigwigs"})do
        local object,method
        if name=="dbm"then object=DBM;method=object and object.RegisterCallback
        else object=BigWigsLoader;method=object and object.RegisterMessage end
        local available=type(method)=="function"
        if available then
            local hooks=self.bossHooks[name]
            if not hooks or hooks.object~=object then hooks={object=object};self.bossHooks[name]=hooks end
            for _,event in ipairs(name=="dbm"and dbmBossEvents or bigWigsBossEvents)do
                if not hooks[event]then
                    local fn=function(...)if name=="dbm"then self:DBMBossCallback(...)else self:BigWigsBossCallback(...)end end
                    local ok=pcall(method,name=="dbm"and object or self,event,fn)
                    if ok then hooks[event]=fn else available=false;s.providers[name].complete=false end
                end
            end
        end
        s.providers[name].available=available
        if not available then s.providers[name].stage=nil;s.providers[name].stageTotal=nil;s.providers[name].encounterID=nil end
    end
end
-- Blizzard owns this clock and its final states. A zero countdown can remain
-- queued; only a Finished state means expiry. No changes to Blizzard's UI.
local function timelineCall(name,...)
    local fn=C_EncounterTimeline and C_EncounterTimeline[name]
    if type(fn)~="function"then return nil end
    local ok,v=pcall(fn,...);if ok and public(v)then return v end
end
local function timelineID(v)
    v=bossNumber(v,2147483647);if v and v>0 and v==math.floor(v)then return v end
end
local function timelineEnums()
    local s=Enum and Enum.EncounterTimelineEventState
    if s and type(s.Active)=="number"and type(s.Paused)=="number"and type(s.Finished)=="number"and type(s.Canceled)=="number"then return s end
end
function O:TimelineAvailable()
    local api=C_EncounterTimeline
    return api and type(api.GetEventList)=="function"and type(api.GetEventInfo)=="function"and type(api.GetEventState)=="function"and type(api.GetEventTimeRemaining)=="function"and timelineEnums()and Enum.EncounterTimelineEventSource and timelineCall("IsFeatureEnabled")==true or false
end
function O:TimelineTimer(id,info,emit)
    id=timelineID(id);if not id then return nil end
    local s=self:BossState();local key="blizzard:"..id;local t=s.timers[key]
    if not public(info)or type(info)~="table"then info=timelineCall("GetEventInfo",id)end
    if type(info)~="table"then
        s.providers.blizzard.complete=false
        if t then t.suppressed=true;t.remaining=nil;t.duration=nil;t.name="";t.spell=nil end
        return nil
    end
    local sources=Enum and Enum.EncounterTimelineEventSource
    local source=public(info.source)and info.source or nil
    if sources and source==sources.EditMode then s.timers[key]=nil;return true end
    if not sources or source~=sources.Encounter and source~=sources.Script then
        s.providers.blizzard.complete=false;if t then t.suppressed=true end;return nil
    end
    local state=timelineCall("GetEventState",id);local states=timelineEnums()
    if not states then return nil end
    local terminal=state==states.Finished or state==states.Canceled
    if terminal then
        if t then
            t.remaining=bossNumber(timelineCall("GetEventTimeRemaining",id))
            if emit then self:BossEmit(state==states.Finished and "bossexpire"or "bossstop",t)end
            s.timers[key]=nil
        end
        return true
    end
    local validState=state==states.Active or state==states.Paused
    local created=not t
    if not t then
        t=self:BossStart("blizzard",tostring(id),nil,info.spellName,info.spellID,info.duration,"timer",nil,true,true)
        if not t then return nil end
    end
    local oldState,oldRemaining,oldDuration,readAt=t.timelineState,t.remaining,t.duration,t.readAt
    local now=GetTime();t.readAt=now
    t.ends=nil;t.duration=bossNumber(info.duration);t.remaining=bossNumber(timelineCall("GetEventTimeRemaining",id))
    t.name=bossName(info.spellName);t.spell=bossSpell(info.spellID);t.source=source==sources.Encounter and "encounter"or "script"
    t.timelineState=state;t.suppressed=not validState;t.paused=nil
    if validState then t.paused=state==states.Paused else s.providers.blizzard.complete=false end
    if emit and validState then
        if created then self:BossEmit("bossstart",t)
        elseif oldState~=state then
            local kind=state==states.Paused and "bosspause"or oldState==states.Paused and "bossresume"or "bossupdate"
            self:BossEmit(kind,t)
        elseif oldDuration~=t.duration or oldRemaining and t.remaining and readAt and math.abs(t.remaining-math.max(0,oldRemaining-(t.paused and 0 or now-readAt)))>0.25 then
            self:BossEmit("bossupdate",t)
        end
    end
    return validState
end
function O:TimelineData()
    local s=self:BossState();local p=s.providers.blizzard
    local warningEnabled=anchorValue(C_EncounterWarnings and C_EncounterWarnings.IsFeatureEnabled,"boolean")==true
    local available=self:TimelineAvailable();p.available=available or warningEnabled;p.complete=false
    if not available then
        for key,t in pairs(s.timers)do if t.provider=="blizzard"then s.timers[key]=nil end end
        s.timelineSeeded=false;return
    end
    local ids=timelineCall("GetEventList")
    if type(ids)~="table"then
        for _,t in pairs(s.timers)do if t.provider=="blizzard"then t.suppressed=true;t.remaining=nil end end
        return
    end
    p.complete=true
    local seen,count={},0
    for _,raw in ipairs(ids)do
        count=count+1;if count>128 then p.complete=false;break end
        local id=timelineID(raw)
        if id then seen["blizzard:"..id]=true;self:TimelineTimer(id,nil,s.timelineSeeded==true)
        else p.complete=false end
    end
    for key,t in pairs(s.timers)do
        if t.provider=="blizzard"and not seen[key]then
            if p.complete then
                -- A disappearance alone never proves the spell happened.
                if s.timelineSeeded then self:BossEmit("bossstop",t)end
                s.timers[key]=nil
            else t.suppressed=true;t.remaining=nil end
        end
    end
    s.timelineSeeded=true
end
function O:TimelineEvent(event,arg)
    if not bossEnabled()then return end
    local s=self:BossState();local p=s.providers.blizzard
    if event=="ENCOUNTER_WARNING"then
        if anchorValue(C_EncounterWarnings and C_EncounterWarnings.IsFeatureEnabled,"boolean")~=true then return end
        if not public(arg)or type(arg)~="table"then return end
        p.available=true
        self:BossEmit("bosswarning",{provider="blizzard",id=0,name=bossName(arg.text),spell=bossSpell(arg.tooltipSpellID),duration=bossNumber(arg.duration)})
        return
    end
    if not self:TimelineAvailable()then return end
    p.available=true
    if event=="ENCOUNTER_TIMELINE_EVENT_ADDED"then
        if public(arg)and type(arg)=="table"then self:TimelineTimer(arg.id,arg,true)else p.complete=false end
    elseif event=="ENCOUNTER_TIMELINE_EVENT_STATE_CHANGED"then
        self:TimelineTimer(arg,nil,true)
    elseif event=="ENCOUNTER_TIMELINE_EVENT_REMOVED"then
        local id=timelineID(arg);local key=id and "blizzard:"..id;local t=key and s.timers[key]
        if t then self:BossEmit("bossstop",t);s.timers[key]=nil end
    elseif event=="ENCOUNTER_TIMELINE_VIEW_DEACTIVATED"then
        for key,t in pairs(s.timers)do if t.provider=="blizzard"then s.timers[key]=nil end end
        s.timelineSeeded=false;p.complete=false
    elseif event=="ENCOUNTER_TIMELINE_VIEW_ACTIVATED"or event=="ENCOUNTER_TIMELINE_STATE_UPDATED"then
        self:TimelineData()
    end
end

function O:BossData()
    if not bossEnabled()then self.bosses=nil;return nil end
    self:BossRegister();self:TimelineData();local s=self:BossState();local now=GetTime()
    if s.providers.dbm.available and type(DBM.GetStage)=="function"then
        local ok,stage,total,id=pcall(DBM.GetStage,DBM)
        if ok then self:BossStage("dbm",stage,total,id,false)else self:BossStage("dbm",nil,nil,nil,false)end
    end
    if s.providers.bigwigs.available and BigWigs and type(BigWigs.IterateBossModules)=="function"then
        local ok,stage=pcall(function()
            local found,current=0,nil
            for _,mod in BigWigs:IterateBossModules()do
                if not public(mod)or type(mod)~="table"or type(mod.IsEngaged)~="function"then return nil end
                local engaged=mod:IsEngaged();if not public(engaged)then return nil end
                if engaged then found=found+1;current=type(mod.GetStage)=="function"and bossNumber(mod:GetStage(),10000)or nil end
            end
            if found==0 then return 0 elseif found==1 then return current end
        end)
        self:BossStage("bigwigs",ok and stage or nil,nil,nil,false)
    end
    local timers={}
    for key,t in pairs(s.timers)do
        if not s.providers[t.provider].available then s.timers[key]=nil
        else
            if not t.expired and not t.paused and t.ends and t.ends<=now then self:BossEmit("bossexpire",t);t.expired=true;if not t.keep then s.timers[key]=nil end end
            if s.timers[key]and not t.suppressed then timers[#timers+1]=self:BossSnapshot(t,now)end
        end
    end
    table.sort(timers,function(a,b)local ar,br=a.remaining or 100000,b.remaining or 100000;if ar==br then return a.id<b.id end;return ar<br end)
    local timerRows={};local overflow=#timers>16 or s.dropped==true
    local omitted={};for i,t in ipairs(timers)do if i<=16 then timerRows[#timerRows+1]=json(t)else omitted[t.provider]=true end end
    local providers={}
    for _,name in ipairs({"dbm","bigwigs","blizzard"})do
        local p=s.providers[name]
        providers[#providers+1]=json({name=name,available=p.available,complete=p.available and p.complete and not omitted[name],stage=p.stage,stageTotal=p.stageTotal,encounterID=p.encounterID})
    end
    local events,kept={},{}
    for _,e in ipairs(s.events)do
        local age=math.floor((now-e.at)*1000)
        if age>=0 and age<=3000 and s.providers[e.timer.provider].available then
            kept[#kept+1]=e;events[#events+1]=json({seq=e.seq,kind=e.kind,age=age,timer=e.timer,stage=e.stage,stageTotal=e.stageTotal,encounterID=e.encounterID})
        end
    end
    s.events=kept
    return json({seq=self.bossSeq or 0,overflow=overflow}):sub(1,-2)..',"providers":['..table.concat(providers,",")..'],"timers":['..table.concat(timerRows,",")..'],"events":['..table.concat(events,",")..']}'
end

-- Cache tree traversal; the published watch rows stay independent of Sim exports.
local function talentCall(fn,...)
    if type(fn)~="function"then return nil,false end
    local ok,v=pcall(fn,...);if not ok or not public(v)then return nil,false end
    return v,true
end
local function talentInteger(v,zero)
    if not public(v)or type(v)~="number"or v~=v or v< (zero and 0 or 1)or v>=10000000 or v~=math.floor(v)then return nil end
    return v
end
local function talentRank(v)
    local n=talentInteger(v,true);if n and n<=100 then return n end
end
function O:TalentTree(config)
    local result={spells={},entries={},complete=false};local api=C_Traits
    if not api then return result end
    local staged,ok=talentCall(api.ConfigHasStagedChanges,config)
    if not ok or staged~=false then return result end
    local success=pcall(function()
        local info=talentCall(api.GetConfigInfo,config);assert(type(info)=="table")
        local trees=info.treeIDs;assert(public(trees)and type(trees)=="table"and #trees>0 and #trees<=8)
        local seen,count={},0
        for _,tree in ipairs(trees)do
            tree=talentInteger(tree);assert(tree)
            local nodes=talentCall(api.GetTreeNodes,tree);assert(type(nodes)=="table"and #nodes>0 and #nodes<=500)
            for _,id in ipairs(nodes)do
                id=talentInteger(id);assert(id)
                if not seen[id]then
                    seen[id]=true;count=count+1;assert(count<=600)
                    local node=talentCall(api.GetNodeInfo,config,id);assert(type(node)=="table"and talentInteger(node.ID))
                    local entries=node.entryIDs;assert(public(entries)and type(entries)=="table"and #entries<=8)
                    local active=node.activeEntry;assert(public(active)and (active==nil or type(active)=="table"))
                    local rank=talentRank(node.activeRank);assert(rank)
                    local activeID,activeRank
                    if active then activeID=talentInteger(active.entryID,true);activeRank=talentRank(active.rank);assert(activeID and activeRank and (activeID>0 or activeRank==0 and rank==0))end
                    if #entries==0 then assert(rank==0)end
                    local subTree=node.subTreeID;assert(public(subTree))
                    local subtreeActive=true
                    if subTree~=nil then
                        subTree=talentInteger(subTree);assert(subTree)
                        local subtree=talentCall(api.GetSubTreeInfo,config,subTree);assert(type(subtree)=="table"and public(subtree.isActive)and type(subtree.isActive)=="boolean")
                        subtreeActive=subtree.isActive
                    end
                    for _,entryID in ipairs(entries)do
                        entryID=talentInteger(entryID);assert(entryID)
                        local entry=talentCall(api.GetEntryInfo,config,entryID);assert(type(entry)=="table")
                        local n=rank
                        if active then n=activeID==entryID and activeRank or 0
                        elseif #entries>1 then assert(rank==0)end
                        if not subtreeActive then n=0 end
                        local definition=entry.definitionID;assert(public(definition))
                        if definition~=nil then
                            definition=talentInteger(definition);assert(definition)
                            local d=talentCall(api.GetDefinitionInfo,definition);assert(type(d)=="table"and public(d.spellID))
                            local spell=talentInteger(d.spellID)
                            -- Some non-spell entries legitimately have no spell ID.
                            assert(d.spellID==nil or spell)
                            result.entries[entryID]={rank=n,spell=spell}
                            if spell then result.spells[spell]=math.max(result.spells[spell]or 0,n)end
                        else
                            assert(talentInteger(entry.subTreeID));result.entries[entryID]={rank=n}
                        end
                    end
                end
            end
        end
    end)
    result.complete=success
    if not success then result.spells={};result.entries={}
    else
        local entries={};for id,entry in pairs(result.entries)do entries[#entries+1]=id..":"..entry.rank end
        table.sort(entries);result.signature=table.concat(entries,",")
    end
    return result
end
-- Change events describe committed snapshots. Initial/unknown snapshots do not
-- invent transitions, and signatures never leave the addon.
function O:RecordLoadoutChange(kind,signature)
    self.loadoutChanges=self.loadoutChanges or {}
    local s=self.loadoutChanges[kind]
    if not s then s={seq=0};self.loadoutChanges[kind]=s end
    if signature then
        if s.signature and s.signature~=signature then
            s.seq=s.seq<2147483647 and s.seq+1 or 1;s.at=GetTime()
        end
        s.signature=signature
    end
end
function O:LoadoutChangeData()
    local rows={}
    for _,kind in ipairs({'talents','equipment'})do
        local enabled=LamdaOverlaysData and LamdaOverlaysData[kind=='talents'and 'talentChanges'or 'equipmentChanges']
        if enabled then
            self:RecordLoadoutChange(kind,nil);local s=self.loadoutChanges[kind]
            local age=s.at and math.floor((GetTime()-s.at)*1000)
            if age and (age<0 or age>3000)then age=nil end
            rows[#rows+1]=json({kind=kind,seq=s.seq,age=age})
        elseif self.loadoutChanges then self.loadoutChanges[kind]=nil end
    end
    return table.concat(rows,',')
end

function O:TalentData()
    local watches=LamdaOverlaysData and LamdaOverlaysData.talents or {}
    local changes=LamdaOverlaysData and LamdaOverlaysData.talentChanges
    if #watches==0 and not changes then self.talentTree=nil;return nil end
    local config,configKnown=talentCall(C_ClassTalents and C_ClassTalents.GetActiveConfigID)
    if config~=nil then config=talentInteger(config);if not config then configKnown=false end end
    local needTree=changes;for _,w in ipairs(watches)do if w.kind=="spell"or w.kind=="entry"then needTree=true end end
    local now=GetTime()
    local staged,stagedKnown=talentCall(C_Traits and C_Traits.ConfigHasStagedChanges,config)
    if config and (not stagedKnown or staged~=false or self.talentStaged)then self.talentDirty=true end
    self.talentStaged=staged==true
    if needTree and (not self.talentTree or self.talentConfig~=config or self.talentDirty or not self.talentTree.complete and now-(self.talentReadAt or 0)>=2)then
        self.talentConfig=config;self.talentReadAt=now;self.talentDirty=false
        if configKnown and config==nil then self.talentTree={spells={},entries={},complete=true,signature=""}
        elseif configKnown then self.talentTree=self:TalentTree(config)
        else self.talentTree={spells={},entries={},complete=false}end
    end
    local treeReady=configKnown and (config==nil or stagedKnown and staged==false)and self.talentTree and self.talentTree.complete
    local pvp,pvpReady={},true
    local needPvp=changes;for _,w in ipairs(watches)do if w.kind=="pvp"then needPvp=true end end
    if needPvp then
        local selected=talentCall(C_SpecializationInfo and C_SpecializationInfo.GetAllSelectedPvpTalentIDs)
        if type(selected)~="table"or #selected>8 then pvpReady=false
        else
            for _,id in ipairs(selected)do
                id=talentInteger(id)
                local info=id and talentCall(C_SpecializationInfo.GetPvpTalentInfo,id)
                local spell=type(info)=="table"and talentInteger(info.spellID)
                if spell then pvp[spell]=1 else pvpReady=false end
            end
        end
    end
    local hero,heroKnown=talentCall(C_ClassTalents and C_ClassTalents.GetActiveHeroTalentSpec)
    if hero~=nil then hero=talentInteger(hero);if not hero then heroKnown=false end end
    if changes and treeReady and pvpReady and heroKnown then
        local pvpIDs={}
        for id in pairs(pvp)do pvpIDs[#pvpIDs+1]=tostring(id)end
        table.sort(pvpIDs)
        self:RecordLoadoutChange("talents",self.talentTree.signature.."|"..table.concat(pvpIDs,",").."|"..(hero or 0))
    end
    local rows={}
    for i,w in ipairs(watches)do
        if i>16 then break end
        local rank,spell
        if (w.kind=="spell"or w.kind=="entry")and treeReady then
            if w.kind=="spell"then rank=self.talentTree.spells[w.id]or 0;spell=w.id
            else local entry=self.talentTree.entries[w.id];rank=entry and entry.rank or 0;spell=entry and entry.spell end
        elseif w.kind=="pvp"and pvpReady then rank=pvp[w.id]or 0;spell=w.id
        elseif w.kind=="hero"and heroKnown and (configKnown and config==nil or stagedKnown and staged==false)then rank=hero==w.id and 1 or 0
        elseif w.kind=="known"then
            local known,ok=talentCall(C_SpellBook and C_SpellBook.IsSpellKnown,w.id)
            if ok and type(known)=="boolean"then rank=known and 1 or 0 end;spell=w.id
        end
        rows[#rows+1]=json({kind=w.kind,id=w.id,rank=rank,spell=spell})
    end
    return table.concat(rows,",")
end

-- Personal equipped gear uses only public item facts. Missing cache data is unknown.
function O:EquipmentData(names)
    if not LamdaOverlaysData or not LamdaOverlaysData.equipment then self.equipment=nil;return nil end
    local now=GetTime()
    if not self.equipment or self.equipmentDirty or not self.equipment.complete and now-(self.equipmentReadAt or 0)>=2 then
        self.equipmentDirty=false;self.equipmentReadAt=now
        local data={slots={},sets={},complete=true}
        for slot=1,19 do
            local row={slot=slot};data.slots[#data.slots+1]=row
            local loc=ItemLocation and talentCall(ItemLocation.CreateFromEquipmentSlot,ItemLocation,slot)
            local exists=loc and talentCall(C_Item and C_Item.DoesItemExist,loc)
            if exists==false then row.item=0
            elseif exists==true then
                row.item=talentInteger(talentCall(C_Item and C_Item.GetItemID,loc))
                if row.item then
                    row.level=bossNumber(talentCall(C_Item and C_Item.GetCurrentItemLevel,loc),10000)
                    if C_Item and type(C_Item.GetItemInfo)=="function"then
                        local info={pcall(C_Item.GetItemInfo,row.item)}
                        if info[1] and public(info[2])and type(info[2])=="string"then
                            row.name=bossName(info[2]);if #row.name>80 then row.name=nil end
                            row.class=talentInteger(info[13],true);row.subclass=talentInteger(info[14],true)
                            if row.class and row.class>100 then row.class=nil end
                            if row.subclass and row.subclass>100 then row.subclass=nil end
                            if public(info[17])then row.set=info[17]==nil and 0 or talentInteger(info[17],true)end
                        end
                    end
                    if not row.level or not row.class or not row.subclass or not row.set then data.complete=false end
                else data.complete=false end
            else data.complete=false end
        end
        for i,w in ipairs(LamdaOverlaysData.equipmentSets or {})do
            if i>8 then break end
            local row={key=w.key};data.sets[#data.sets+1]=row
            local id,known=talentCall(C_EquipmentSet and C_EquipmentSet.GetEquipmentSetID,w.name)
            if known and id==nil then row.equipped=false
            elseif known and talentInteger(id,true)and C_EquipmentSet and type(C_EquipmentSet.GetEquipmentSetInfo)=="function"then
                local ok,_,_,_,equipped=pcall(C_EquipmentSet.GetEquipmentSetInfo,id)
                if ok and public(equipped)and type(equipped)=="boolean"then row.equipped=equipped end
            end
            if row.equipped==nil then data.complete=false end
        end
        self.equipment=data
    end
    if LamdaOverlaysData.equipmentChanges then
        local signature,complete={},true
        for _,r in ipairs(self.equipment.slots)do
            if r.item==nil or r.item>0 and (r.level==nil or r.set==nil)then complete=false;break end
            signature[#signature+1]=r.slot..":"..r.item..":"..(r.level or 0)..":"..(r.set or 0)
        end
        for _,r in ipairs(self.equipment.sets)do
            if r.equipped==nil then complete=false;break end
            signature[#signature+1]=r.key..":"..tostring(r.equipped)
        end
        if complete then self:RecordLoadoutChange("equipment",table.concat(signature,"|"))end
    end
    local slots,sets={},{}
    for _,r in ipairs(self.equipment.slots)do
        slots[#slots+1]=json({slot=r.slot,item=r.item,level=r.level,set=r.set,class=r.class,subclass=r.subclass,name=names and r.name or nil})
    end
    for _,r in ipairs(self.equipment.sets)do sets[#sets+1]=json(r)end
    return '{"slots":['..table.concat(slots,",")..'],"sets":['..table.concat(sets,",")..']}'
end

local function measurementPoint(index,field,copy)
    local cell=(index-1)*6+copy*3+field
    if O.compact then return densePoint((O.nativeVersion==3 and 2096 or 1680)+cell)end
    if cell<52 then return 48+cell*4,0 end
    return (cell-52)*4,36
end
function O:MeasurementTick()
    local rows={}
    for i,w in ipairs(self.measurementWatches or {})do
        local fonts=self.measurementFonts and self.measurementFonts[i]
        local power,mod,available=-1,1,false
        local ok=pcall(function()
            if not self.native or not fonts or anchorValue(UnitExists,"boolean",w.unit)~=true or anchorValue(UnitIsVisible,"boolean",w.unit)~=true then return end
            local current,maximum
            if w.kind=="health"then current=UnitHealthPercent(w.unit,true,CurveConstants.ScaleTo100);maximum=100
            else
                power=w.resource=="primary"and anchorValue(UnitPowerType,"number",w.unit)or tonumber(w.resource)
                if not power or power<0 or power>=32 or power==Enum.PowerType.Runes then power=-1;return end
                mod=UnitPowerDisplayMod and anchorValue(UnitPowerDisplayMod,"number",power)or 1
                if not mod or mod<=0 or mod>1000 then mod=0;return end
                current,maximum=UnitPower(w.unit,power,true),UnitPowerMax(w.unit,power,true)
            end
            for copy=0,1 do
                fonts[copy*3+1]:SetFormattedText("|cff%06xI|r",0x28c6a1)
                fonts[copy*3+2]:SetFormattedText("|cff%06xI|r",current)
                fonts[copy*3+3]:SetFormattedText("|cff%06xI|r",maximum)
            end
            available=true
        end)
        if not ok or not available then
            available=false
            for _,f in ipairs(fonts or {})do f:SetFormattedText("|cff%06xI|r",0xffffff)end
        end
        rows[#rows+1]="["..json(w.unit)..","..json(w.kind)..","..json(w.resource or "")..","..tostring(power or -1)..","..tostring(mod or 0)..","..json(available)..","..self:UnitGeneration(w.unit).."]"
    end
    return table.concat(rows,",")
end

function O:AuraFilter(watch)
    if watch.kind~="debuff"then return "HELPFUL"end
    if watch.unit=="target"and self.targetDebuffsMine then return "HARMFUL|PLAYER"end
    return "HARMFUL"
end
function O:Create()
    if InCombatLockdown()then return false end
    self.targetDebuffsMine=LUI:DB().overlaysOwnTargetDebuffs==true
    self.nativeVersion=#self.cooldownWatches+2*#self.castWatches>64 and 3 or nil
    self.compact=#self.watches>16 or #self.cooldownWatches+2*#self.castWatches>16 or #self.measurementWatches>16
    self.nativeVersion=self.nativeVersion or (self.compact and 2 or nil)
    assert(#self.watches<=64 and #self.cooldownWatches+2*#self.castWatches<=90 and #self.measurementWatches<=64,"Native source limit exceeded")
    self.strip=LUI.LookupEvents:GetStrip("overlays")
    if not self.root then
        self.root=CreateFrame("Frame",nil,self.strip.frame);self.root:SetSize(self.strip.width,self.strip.height);self.root:SetFrameStrata("TOOLTIP");self.root:SetFrameLevel(9000);self.root:EnableMouse(false)
        self.root:SetPoint("TOPLEFT",self.strip.frame,"TOPLEFT",0,0)
        self.strip.frame:Show();self.root:Show()
        -- Keep the backing below the native glyphs (9002/9003) and packet
        -- tiles (10000). A BACKGROUND on the packet frame still covers glyphs
        -- belonging to lower-level frames, turning every reading black.
        local bg=self.root:CreateTexture(nil,"BACKGROUND");bg:SetAllPoints();bg:SetColorTexture(0,0,0,1)
        for i,c in ipairs({0,255,65280,16711680,16777215,0x123456,0xabcdef,0x808080})do
            local x,y=(i-1)*4,0;if self.compact then x,y=densePoint(i-1)end
            patch(self.root,x,y):SetFormattedText("|cff%06xI|r",c)
        end
        self.cooldownFonts={}
        for i=1,#self.cooldownWatches do
            local fonts={};self.cooldownFonts[i]=fonts
            for field=0,7 do
                fonts[field+1]={}
                for copy=0,1 do
                    local x,y=timerPoint(i-1,field,copy)
                    fonts[field+1][copy+1]=patch(self.root,x,y)
                end
            end
        end
        self.castFonts={}
        for i=1,#self.castWatches do
            local fonts={};self.castFonts[i]=fonts
            for field=0,15 do
                fonts[field+1]={}
                for copy=0,1 do
                    local x,y=timerPoint(#self.cooldownWatches+(i-1)*2,field,copy)
                    fonts[field+1][copy+1]=patch(self.root,x,y)
                end
            end
        end
        self.measurementFonts={}
        for i=1,#self.measurementWatches do
            local fonts={};self.measurementFonts[i]=fonts
            for copy=0,1 do for field=0,2 do local x,y=measurementPoint(i,field,copy);fonts[#fonts+1]=patch(self.root,x,y)end end
        end
        self.health={};for copy=0,1 do local x,y=healthPoint(copy);self.health[#self.health+1]=patch(self.root,x,y)end
        self.resources={};self.resourceMods={}
        for _,power in pairs(Enum.PowerType or {})do
            if type(power)=="number"and power>=0 and power<32 and not self.resources[power]then
                local ok,mod=pcall(function()return UnitPowerDisplayMod and UnitPowerDisplayMod(power)or 1 end)
                mod=ok and value(mod,0)or 0
                if type(mod)~="number"then mod=0 end
                self.resourceMods[tostring(power)]=power==Enum.PowerType.Runes and 1 or mod
                self.resources[power]={};for field=0,3 do local x,y=resourcePoint(power,field);self.resources[power][field+1]=patch(self.root,x,y)end
            end
        end
    end
    if not C_StringUtil or not C_StringUtil.CreateNumericRuleFormatter then self.nativeError="Native aura support unavailable";return true end
    if not AuraContainerSortMethod then C_AddOns.LoadAddOn("Blizzard_AuraContainer")end
    local count=C_StringUtil.CreateNumericRuleFormatter();count:AddBreakpoint({threshold=0,format="|cff%06xI|r",step=1})
    local duration=C_StringUtil.CreateNumericRuleFormatter();duration:AddBreakpoint({threshold=0,format="|cff%06xI|r",components={{div=.1,step=1,rounding=Enum.NumericRuleFormatRounding.Down}}})
    -- Round remaining cooldowns up: a positive fraction must never read ready.
    self.cooldownFormatter=C_StringUtil.CreateNumericRuleFormatter()
    self.cooldownFormatter:AddBreakpoint({threshold=0,format="|cff1%05xI|r",components={{div=.1,step=1,rounding=Enum.NumericRuleFormatRounding.Up}}})
    self:CreateCasts()
    for i,w in ipairs(self.watches)do
        for copy=0,1 do
            local x,y=auraPoint(i,copy,0)
            tile(self.root,x,y,0x19354b)
            local c=CreateFrame("AuraContainer",nil,self.root,"CustomAuraContainerTemplate")
            c:SetEnabled(false)
            c:SetSize(1,1);c:SetPoint("TOPLEFT");c:SetUnit(w.unit);c:SetFrameStrata("TOOLTIP");c:SetFrameLevel(9001)
            local initialized=false
            c:AddAuraSlot("watch",self:AuraFilter(w),{
                candidateFilters={includeSpellIDs={[w.spell]=true}},sortMethod=AuraContainerSortMethod.Default,sortDirection=AuraContainerSortDirection.Normal,
                initializeFrame=function(button)
                    -- Blizzard invokes this through securecallfunction, which
                    -- otherwise swallows failures before our outer pcall sees them.
                    local ok,err=pcall(function()
                    button:SetSize(self.compact and 6 or 12,self.compact and 2 or 4);place(button,x,y);button:SetMouseClickEnabled(false);button:SetMouseMotionEnabled(false)
                    button:SetFrameStrata("TOOLTIP");button:SetFrameLevel(9002)
                    tile(button,x,y,0x37d6a0)
                    -- Binding a sink can hide a button with no aura. Finish
                    -- both scroll regions first while their geometry is visible.
                    local sx,sy=auraPoint(i,copy,1);local rx,ry=auraPoint(i,copy,2)
                    local stacks=patch(button,sx,sy,true)
                    local remaining=patch(button,rx,ry,true)
                    button:SetApplicationCount(stacks,{formatter=count})
                    local binding=C_DurationUtil.CreateDurationTextBinding();binding:SetFormatter(duration);binding:SetUpdateInterval(.1)
                    binding:SetZeroDurationText("|cffffffffI|r");binding:SetExpiredText("|cff000000I|r")
                    button:SetDurationText(remaining,{binding=binding})
                    end)
                    initialized=ok
                    if not ok then self.captureError=text(err):sub(1,220);if self.captureError==""then self.captureError="Aura initialization failed"end end
                end,
            })
            if not initialized and not self.captureError then self.captureError="Aura initializer did not complete"end
            c:Show();c:SetEnabled(initialized);self.containers[#self.containers+1]=c
        end
    end
    -- Each native button owns a single duration text binding. Separate paired
    -- slots supply totals without replacing the established countdown sinks.
    self.durationAvailable={}
    for i,w in ipairs(self.watches)do
        local complete=true
        for copy=0,1 do
            local c
            local ok=pcall(function()
                assert(Enum.DurationTextBindingProperty and Enum.DurationTextBindingProperty.TotalDuration)
                local x,y=auraPoint(i,copy,3)
                tile(self.root,x,y,0xffffff)
                c=CreateFrame("AuraContainer",nil,self.root,"CustomAuraContainerTemplate")
                c:SetEnabled(false);c:SetSize(1,1);c:SetPoint("TOPLEFT");c:SetUnit(w.unit)
                c:SetFrameStrata("TOOLTIP");c:SetFrameLevel(9001)
                local initialized=false
                c:AddAuraSlot("duration",self:AuraFilter(w),{
                    candidateFilters={includeSpellIDs={[w.spell]=true}},sortMethod=AuraContainerSortMethod.Default,sortDirection=AuraContainerSortDirection.Normal,
                    initializeFrame=function(button)
                        initialized=pcall(function()
                            local size=self.compact and 2 or 4;button:SetSize(size,size);place(button,x,y);button:SetMouseClickEnabled(false);button:SetMouseMotionEnabled(false)
                            button:SetFrameStrata("TOOLTIP");button:SetFrameLevel(9002)
                            local total=patch(button,x,y,true)
                            local binding=C_DurationUtil.CreateDurationTextBinding()
                            binding:SetTextFormat("{}",{{property=Enum.DurationTextBindingProperty.TotalDuration,formatter=self.cooldownFormatter}})
                            binding:SetUpdateInterval(.1);binding:SetZeroDurationText("|cffffffffI|r");binding:SetExpiredText("|cffffffffI|r")
                            button:SetDurationText(total,{binding=binding})
                        end)
                    end,
                })
                assert(initialized);c:Show();c:SetEnabled(true)
            end)
            if c then self.containers[#self.containers+1]=c;if not ok then c:SetEnabled(false)end end
            if not ok then complete=false end
        end
        self.durationAvailable[i]=complete
    end
    self.native=true;return true
end

-- Values go directly from Blizzard APIs/formatters to native font sinks.
-- Lua never branches on or serializes cooldown, charge, or usability values.
function O:CooldownTick()
    local rows={}
    for i,w in ipairs(self.cooldownWatches or {})do
        local fonts=self.cooldownFonts[i]
        local function number(field,n)for _,f in ipairs(fonts[field])do f:SetFormattedText("|cff1%05xI|r",n)end end
        local function boolean(field,b)
            if public(b)then if type(b)~="boolean"then error("Missing cooldown state")end;number(field,b and 1 or 0)
            else number(field,C_CurveUtil.EvaluateColorValueFromBoolean(b,1,0))end
        end
        local function timer(field,d)
            for _,f in ipairs(fonts[field])do f:SetText(d:FormatRemainingDuration(self.cooldownFormatter))end
            for _,f in ipairs(fonts[field+1])do f:SetText(d:FormatTotalDuration(self.cooldownFormatter))end
        end
        local available=false
        -- Failed reads erase every sink in this slot, so stale ready values
        -- cannot survive an unlearned spell, equipment swap or API failure.
        local ok=pcall(function()
            if not self.cooldownFormatter then error("Cooldown formatter unavailable")end
            local id=w.id
            if w.kind=="spell"or w.kind=="gcd"then
                if w.kind=="spell"then
                    local known=C_SpellBook.IsSpellKnownOrOverridesKnown or C_SpellBook.IsSpellKnown
                    available=value(known(id),false)==true
                    -- Pet abilities can be absent from the player's spell bank.
                    -- Query the active pet's actual spellbook instead of inferring
                    -- availability from a pet family or specialization label.
                    if not available and C_SpellBook.IsSpellKnown and Enum.SpellBookSpellBank and Enum.SpellBookSpellBank.Pet then
                        available=value(C_SpellBook.IsSpellKnown(id,Enum.SpellBookSpellBank.Pet),false)==true
                    end
                    if not available then return end
                    id=value(C_Spell.GetOverrideSpell(id),id)
                else available=true end
                local cd=C_Spell.GetSpellCooldown(id)
                if not public(cd)or type(cd)~="table"then error("Cooldown unavailable")end
                timer(1,C_Spell.GetSpellCooldownDuration(id,w.kind~="gcd"))
                boolean(8,cd.isEnabled)
                boolean(7,C_Spell.IsSpellUsable(id))
                local charges=C_Spell.GetSpellCharges(id)
                if not public(charges)then error("Charges unavailable")end
                if charges then
                    number(3,charges.currentCharges);number(4,charges.maxCharges)
                    local recharge=C_Spell.GetSpellChargeDuration(id)
                    if recharge then timer(5,recharge)
                    else
                        -- A full charge pool can have no active recharge object.
                        -- Retain count/readiness; do not invent a recharge time.
                        for field=5,6 do for _,f in ipairs(fonts[field])do f:SetFormattedText("|cff%06xI|r",0xffffff)end end
                    end
                else number(3,0);number(4,0);number(5,0);number(6,0)end
            else
                local start,span,enabled
                if w.kind=="slot"then
                    id=value(GetInventoryItemID("player",id),nil)
                    if not id then return end
                    start,span,enabled=GetInventoryItemCooldown("player",w.id)
                    number(8,enabled)
                else
                    local count=value(C_Item.GetItemCount(id,false),0)
                    if type(count)~="number"or count<=0 then return end
                    start,span,enabled=C_Item.GetItemCooldown(id)
                    boolean(8,enabled)
                end
                available=true
                local d=C_DurationUtil.CreateDuration();d:SetTimeFromStart(start,span)
                timer(1,d);boolean(7,C_Item.IsUsableItem(id))
                number(3,0);number(4,0);number(5,0);number(6,0)
            end
        end)
        if not ok or not available then
            for _,pair in ipairs(fonts)do for _,f in ipairs(pair)do f:SetFormattedText("|cff%06xI|r",0xffffff)end end
        end
        rows[#rows+1]=json({kind=w.kind,id=w.id,available=ok and available})
    end
    return table.concat(rows,",")
end
-- A unit event closure retains the configured public token; event arguments may
-- themselves be restricted. Spell IDs only enter native font sinks.
local castEvents={UNIT_SPELLCAST_START=1,UNIT_SPELLCAST_SUCCEEDED=2,UNIT_SPELLCAST_STOP=3,UNIT_SPELLCAST_FAILED=4,UNIT_SPELLCAST_INTERRUPTED=5,UNIT_SPELLCAST_CHANNEL_START=6,UNIT_SPELLCAST_CHANNEL_STOP=7,UNIT_SPELLCAST_EMPOWER_START=8,UNIT_SPELLCAST_EMPOWER_STOP=9}
local function castNumber(s,field,n,marked)
    for _,f in ipairs(s.fonts[field])do f:SetFormattedText(marked and "|cff1%05xI|r"or "|cff%06xI|r",n)end
end
function O:CastGeneration(s)
    local g=self:UnitGeneration(s.unit)
    if s.generation~=g then
        s.generation=g;s.events={}
        for field=9,16 do castNumber(s,field,0xffffff)end
    end
    castNumber(s,6,g)
end
function O:CastEvent(s,event,_,_,spell,...)
    if not LUI:DB().overlaysEnabled then return end
    self:CastGeneration(s)
    local kind=castEvents[event];if not kind then return end
    local bar=select(event=="UNIT_SPELLCAST_EMPOWER_STOP"and 3 or (event=="UNIT_SPELLCAST_INTERRUPTED"or event=="UNIT_SPELLCAST_CHANNEL_STOP")and 2 or 1,...)
    bar=value(bar,0);if type(bar)~="number"or bar<0 or bar>2147483647 then bar=0 end
    -- Every outcome owns its own slot. A generic STOP cannot change the
    -- meaning of a preceding success/interrupt, and remains an event itself.
    if public(spell)and(type(spell)~="number"or spell<1 or spell>=10000000)then return end
    if s.seq>=0xfffffe then return end -- new UI session resets sequence
    s.seq=s.seq+1;local slot=(s.seq-1)%4;local field=9+slot*2
    castNumber(s,field,0xffffff);castNumber(s,field+1,0xffffff)
    local ok=pcall(castNumber,s,field,spell)
    if not ok then return end
    castNumber(s,field+1,s.seq)
    s.events[slot+1]={seq=s.seq,kind=kind,bar=bar,at=GetTime()}
end
function O:CreateCasts()
    if self.castSources then return end
    self.castSources={}
    for i,unit in ipairs(self.castWatches or {})do
        local s={unit=unit,seq=0,events={},fonts=self.castFonts[i],bindings={}}
        self.castSources[i]=s
        -- A duration object may itself be secret. Pass it to native bindings;
        -- never call a method on, compare, or serialize that object in Lua.
        pcall(function()
            for field=1,2 do for _,f in ipairs(s.fonts[field])do
                local b=C_DurationUtil.CreateDurationTextBinding()
                b:SetFontString(f);b:SetUpdateInterval(.05)
                b:SetZeroDurationText("|cff100000I|r");b:SetExpiredText("|cff100000I|r")
                if field==1 then b:SetFormatter(self.cooldownFormatter)
                else b:SetTextFormat("{}",{{property=Enum.DurationTextBindingProperty.TotalDuration,formatter=self.cooldownFormatter}})end
                b:SetEnabled(false);s.bindings[#s.bindings+1]=b
            end end
        end)
        local f=CreateFrame("Frame");s.frame=f
        for event in pairs(castEvents)do f:RegisterUnitEvent(event,unit)end
        f:SetScript("OnEvent",function(_,event,...)pcall(O.CastEvent,O,s,event,...)end)
        self:CastGeneration(s)
    end
end
function O:CastTick()
    local rows={}
    for _,s in ipairs(self.castSources or {})do
        self:CastGeneration(s)
        for _,b in ipairs(s.bindings)do b:SetEnabled(false)end
        for field=1,5 do castNumber(s,field,0xffffff)end
        local available=false
        local ok=pcall(function()
            if not self.native or anchorValue(UnitExists,"boolean",s.unit)~=true or anchorValue(UnitIsVisible,"boolean",s.unit)~=true then return end
            local _,_,_,_,_,_,_,notInterruptible,spell,castBar=UnitCastingInfo(s.unit)
            local function hasBar(id)return public(id)and type(id)=="number"and id>=0 end
            local function knownSpell(id)return public(id)and type(id)=="number"and id>0 end
            local mode,duration=1,UnitCastingDuration
            if not hasBar(castBar)and not knownSpell(spell)then
                local _,_,_,_,_,_,ni,id,empowered,_,channelBar=UnitChannelInfo(s.unit)
                if hasBar(channelBar)or knownSpell(id)then
                    if not public(empowered)then error("Cast kind unavailable")end
                    notInterruptible,spell=ni,id
                    mode=empowered==true and 3 or 2
                    duration=mode==3 and UnitEmpoweredChannelDuration or UnitChannelDuration
                elseif public(spell)and spell==nil and public(id)and id==nil then
                    castNumber(s,3,0);castNumber(s,4,0,true);available=true;return
                else error("Cast identity unavailable")end
            end
            castNumber(s,3,spell);castNumber(s,4,mode,true);available=true
            pcall(function()
                if public(notInterruptible)then
                    if type(notInterruptible)=="boolean"then castNumber(s,5,notInterruptible and 0 or 1,true)end
                else castNumber(s,5,C_CurveUtil.EvaluateColorValueFromBoolean(notInterruptible,0,1),true)end
            end)
            local timerOK=pcall(function()
                if #s.bindings~=4 then error("Cast timer unavailable")end
                local d=duration(s.unit)
                for _,b in ipairs(s.bindings)do b:SetDuration(d);b:SetEnabled(true)end
            end)
            if not timerOK then
                for _,b in ipairs(s.bindings)do b:SetEnabled(false)end
                castNumber(s,1,0xffffff);castNumber(s,2,0xffffff)
            end
        end)
        if not ok then available=false;for field=1,5 do castNumber(s,field,0xffffff)end end
        local events={}
        for _,e in pairs(s.events)do
            local age=math.floor((GetTime()-e.at)*1000+.5)
            if age>=0 and age<=3000 then events[#events+1]="["..e.seq..","..e.kind..","..e.bar..","..age.."]"end
        end
        rows[#rows+1]="["..json(s.unit)..","..s.generation..","..json(available)..","..s.seq..",["..table.concat(events,",").."]]"
    end
    return table.concat(rows,",")
end

function O:MetadataPacket(body,level)
    if #body>65536 then return nil end
    if #body>6134 then
        local library=LibStub and LibStub("LibDeflate",true)
        if library and type(library.CompressDeflate)=="function"then
            local ok,compressed=pcall(library.CompressDeflate,library,body,{level=level or 1})
            if ok and type(compressed)=="string"and #compressed+2<=6134 then body="Z1"..compressed end
        end
    end
    if #body>6134 then return nil end
    return LUI.LookupEvents:Packet(body,"LUO1")
end

function O:Tick()
    if LUI.overlayHidden then if self.root then self.root:Hide()end;if self.strip then self.strip.frame:Hide()end;return end
    if not LUI:DB().overlaysEnabled then self.bosses=nil;self.loadoutChanges=nil;self.talentTree=nil;self.equipment=nil;if self.root then self.root:Hide()end;if self.strip then self.strip.frame:Hide()end;return end
    if not self.ready then
        if InCombatLockdown()then return end
        self.watches=(LamdaOverlaysData and LamdaOverlaysData.watches)or {}
        self.castWatches=(LamdaOverlaysData and LamdaOverlaysData.casts)or {}
        self.cooldownWatches=(LamdaOverlaysData and LamdaOverlaysData.cooldowns)or {}
        self.measurementWatches=(LamdaOverlaysData and LamdaOverlaysData.measurements)or {}
        local ok,result=pcall(self.Create,self)
        if not ok then self.nativeError="Native aura setup failed";self.captureError=text(result):sub(1,220);self.ready=true;self.native=false
        else self.ready=result end
        if not self.ready or not self.strip then return end
    end
    if self.strip.moving then self.root:Hide();return end
    local width,height=GetPhysicalScreenSize();local offset=32
    self.root:ClearAllPoints();self.root:SetPoint("TOPLEFT",self.strip.frame,"TOPLEFT",0,0);self.root:Show()
    if self.native and UnitHealthPercent then
        local ok=pcall(function()local health=UnitHealthPercent("player",true,CurveConstants.ScaleTo100);for _,f in ipairs(self.health)do f:SetFormattedText("|cff%06xI|r",health)end end)
        if not ok then for _,f in ipairs(self.health)do f:SetFormattedText("|cff%06xI|r",0xffffff)end end
    end
    if self.native then
        -- Pass resource values straight to native sinks, without Lua arithmetic.
        for power,fonts in pairs(self.resources)do
            local ok=pcall(function()
                local current,maximum
                if power==Enum.PowerType.Runes then
                    local _,class=UnitClass("player");current=0;maximum=0
                    if class=="DEATHKNIGHT"then
                        maximum=6
                        for i=1,6 do
                            local _,_,ready=GetRuneCooldown(i)
                            if not public(ready)or ready==nil then error("Rune state unavailable")end
                            if ready then current=current+1 end
                        end
                    end
                else current,maximum=UnitPower("player",power,true),UnitPowerMax("player",power,true)end
                fonts[1]:SetFormattedText("|cff%06xI|r",current);fonts[2]:SetFormattedText("|cff%06xI|r",current)
                fonts[3]:SetFormattedText("|cff%06xI|r",maximum);fonts[4]:SetFormattedText("|cff%06xI|r",maximum)
            end)
            if not ok then for _,f in ipairs(fonts)do f:SetFormattedText("|cff%06xI|r",0xffffff)end end
        end
    end
    local equipment=self:EquipmentData(true)
    local talents=self:TalentData()
    local changes=self:LoadoutChangeData()
    local bosses=self:BossData()
    local encounter=self:EncounterData()
    local casts=self:CastTick()
    local cooldowns=self:CooldownTick()
    local measurements=self:MeasurementTick()
    local name,realm=UnitFullName("player");local _,class=UnitClass("player");local _,race=UnitRace("player")
    local spec=GetSpecialization and GetSpecialization();spec=spec and select(1,GetSpecializationInfo(spec))or 0
    local inside,kind=IsInInstance();local location=inside and kind or "world";if location=="party"or location=="scenario"then location="dungeon"end
    local role=text(UnitGroupRolesAssigned("player")):lower();if role=="damager"then role="damage"end
    if role=="none"or role==""then local s=GetSpecialization and GetSpecialization();role=(s and text(GetSpecializationRole(s)):lower())or "damage";if role=="damager"then role="damage"end end
    self.seq=self.seq+1;self.epoch=self.epoch or tostring(GetServerTime())..":"..tostring(math.floor(GetTime()*1000))
    local p={epoch=self.epoch,seq=self.seq,at=GetServerTime(),hash=(LamdaOverlaysData and LamdaOverlaysData.hash)or "4f53cda18c2baa0c",
        loadoutChangesVersion=1,castsVersion=1,castHash=(LamdaOverlaysData and LamdaOverlaysData.castHash)or "4f53cda18c2baa0c",
        measurementsVersion=1,measurementHash=(LamdaOverlaysData and LamdaOverlaysData.measurementHash)or "4f53cda18c2baa0c",
        unitsVersion=(LamdaOverlaysData and LamdaOverlaysData.units)and 1 or 0,anchorsVersion=1,auraDurationsVersion=1,cooldownsVersion=1,cooldownHash=(LamdaOverlaysData and LamdaOverlaysData.cooldownHash)or "4f53cda18c2baa0c",
        nativeVersion=self.nativeVersion,captureError=self.captureError,stripWidth=self.strip.width,resourcesVersion=1,resourceMods=self.resourceMods or {},primaryPower=value(UnitPowerType and UnitPowerType("player"),-1),width=width,height=height,offset=offset,native=self.native==true,character=text(name),realm=text(realm)~=""and text(realm)or GetNormalizedRealmName(),class=text(class),race=text(race),spec=value(spec,0),faction=text(UnitFactionGroup("player")),role=role,location=text(location),zone=text(GetZoneText()),
        level=value(UnitLevel("player"),0),combat=InCombatLockdown(),mounted=value(IsMounted(),false),alive=not value(UnitIsDeadOrGhost("player"),true),group=IsInRaid()and "raid"or IsInGroup()and "party"or "solo",key=C_ChallengeMode and C_ChallengeMode.IsChallengeModeActive and value(C_ChallengeMode.IsChallengeModeActive(),false)or false,cast=self.cast,castSeq=self.castSeq}
    if LamdaOverlaysData and LamdaOverlaysData.combat then p.combatVersion=1;p.identityHash=LamdaOverlaysData.identityHash;p.logging=anchorValue(LoggingCombat,"boolean")end
    p.petVersion=1;p.pet=self:PetData(text(class))
    if encounter then p.encountersVersion=1 end
    if bosses then p.bossesVersion=1 end
    if equipment then p.equipmentVersion=1 end
    if talents then p.talentsVersion=1;p.talentHash=LamdaOverlaysData.talentHash end
    local unitTokens
    if LamdaOverlaysData and LamdaOverlaysData.units and type(LamdaOverlaysData.unitTokens)=="table"then
        p.unitsVersion=2;p.unitHash=LamdaOverlaysData.unitHash
        local tokens={};for i,unit in ipairs(LamdaOverlaysData.unitTokens)do if i>98 then break end;tokens[#tokens+1]=json(unit)end
        unitTokens=table.concat(tokens,",")
    end
    p.heroSpec=talentInteger(anchorValue(C_ClassTalents and C_ClassTalents.GetActiveHeroTalentSpec,"number"))
    local rows={};for i,w in ipairs(self.watches)do rows[#rows+1]=json({g=self:UnitGeneration(w.unit),spell=w.spell,kind=w.kind,unit=w.unit,durationAvailable=self.durationAvailable and self.durationAvailable[i]==true,available=value(UnitExists(w.unit),false)and value(UnitIsVisible(w.unit),false)})end
    local function header(gear)return json(p):sub(1,-2)..(unitTokens and ',"unitTokens":['..unitTokens..']' or '')..',"loadoutChanges":['..changes..']'..(gear and ',"equipment":'..gear or '')..(talents and ',"talents":['..talents..']' or '')..(bosses and ',"bosses":'..bosses or '')..(encounter and ',"encounter":'..encounter or '')..',"watches":[' end
    local head=header(equipment)
    local tail='],"cooldowns":['..cooldowns..'],"casts":['..casts..'],"anchors":['..self:AnchorRows(width,height)..'],"measurements":['..measurements..'],"identities":['..self:IdentityRows()..'],"units":['
    local body=head..table.concat(rows,",")..tail..self:UnitRows(true)..']}'
    local packet=self:MetadataPacket(body)
    if packet then self.strip:Paint(packet);return end
    if equipment then
        head=header(self:EquipmentData(false));body=head..table.concat(rows,",")..tail..self:UnitRows(true)..']}'
        packet=self:MetadataPacket(body);if packet then self.strip:Paint(packet);return end
    end
    if #body>6134 then body=head..table.concat(rows,",")..tail..self:UnitRows(false)..']}'end
    if #body>6134 then
        local compact={}
        for i,w in ipairs(self.watches)do compact[#compact+1]="["..w.spell..","..json(w.kind)..","..json(w.unit)..","..self:UnitGeneration(w.unit)..","..json(value(UnitExists(w.unit),false)and value(UnitIsVisible(w.unit),false))..","..json(self.durationAvailable and self.durationAvailable[i]==true or false).."]"end
        body=head..table.concat(compact,",")..tail..self:UnitRows(false)..']}'
    end
    packet=self:MetadataPacket(body,5)
    if not packet then
        -- Publish a current unavailable sample; stale metadata must never be
        -- paired with the new native pixels after a serialization failure.
        packet=self:MetadataPacket(json({epoch=self.epoch,seq=self.seq,at=p.at,hash=p.hash,width=width,height=height,offset=offset,stripWidth=self.strip.width,nativeVersion=p.nativeVersion,native=false,captureError="Capture packet too large"}))
    end
    if packet then self.strip:Paint(packet)end
end
local module={id="overlays",name="LamdaOverlays",enabledKey="overlaysEnabled",experimental=true,defaults={overlaysEnabled=false,overlaysBridgeX=0,overlaysBridgeY=320,overlaysBridgeWidth=256,overlaysOwnTargetDebuffs=not not LUI.ForeverEngine},limits={overlaysBridgeX={0,16000},overlaysBridgeY={0,16000},overlaysBridgeWidth={values={16,24,32,36,48,64,72,96,128,144,192,256,288,384,512,576,768,1152}}},reloadKeys={"overlaysEnabled","overlaysBridgeX","overlaysBridgeY","overlaysBridgeWidth","overlaysOwnTargetDebuffs"}}
module.build=function(parent)
    LUI:Label(parent,"Manager",4,-7)
    local url="http://127.0.0.1:"..(LUI.ForeverEngine and "47642"or "47641").."/overlays/"
    local link=CreateFrame("EditBox",nil,parent,"InputBoxTemplate");parent.managerLink=link
    link:SetSize(360,28);link:SetPoint("TOPLEFT",84,0);link:SetAutoFocus(false);link:SetText(url);link:SetCursorPosition(0)
    link:SetScript("OnEditFocusGained",function(self)self:HighlightText()end)
    link:SetScript("OnMouseUp",function(self)self:HighlightText()end)
    link:SetScript("OnTextChanged",function(self,userInput)if userInput then self:SetText(url);self:HighlightText()end end)
    link:SetScript("OnEscapePressed",function(self)self:ClearFocus()end)
    link:SetScript("OnEnterPressed",function(self)self:ClearFocus()end)
    link:SetScript("OnHide",function(self)self:ClearFocus()end)
    LUI:Checkbox(parent,"Target debuffs: only mine","overlaysOwnTargetDebuffs",-42,0,
        "Count only your own debuffs in target-based overlay rules. This applies to presence and countdowns together. Enabled by default on Forever. Apply & Reload after changing it.")
    LUI:Button(parent,"Help",4,-88,96,function()
        if parent.help then parent.help:Hide();parent.help=nil;return end
        parent.help=LUI:Description(parent,"Click the Manager field, press Ctrl+C, and open the link in your browser. Keep Lamda Engine running. Save your rules there. Reload WoW after adding or changing a watched aura. Preview tests stay in the browser. General → Color strips holds the movement controls. Drag the gold grip to resize, then Lock all and Apply & Reload.",4,-130,540)
    end)
end
LUI:RegisterModule(module)
function O:Event(event,unit,cast,spell,...)
    if event=="UNIT_FLAGS"then
        if LUI:DB().overlaysEnabled and public(unit)and unit=="pet"then
            local _,class=UnitClass("player");self:PetData(text(class))
        end
        return
    end
    if event=="PLAYER_LOGIN"or event=="PLAYER_ENTERING_WORLD"or event=="PLAYER_EQUIPMENT_CHANGED"or event=="GET_ITEM_INFO_RECEIVED"or event=="EQUIPMENT_SETS_CHANGED"or event=="EQUIPMENT_SWAP_FINISHED"or event=="UNIT_INVENTORY_CHANGED"and public(unit)and unit=="player"then self.equipmentDirty=true end
    if event=="PLAYER_LOGIN"or event=="PLAYER_ENTERING_WORLD"or event=="PLAYER_TALENT_UPDATE"or event=="TRAIT_CONFIG_UPDATED"or event=="TRAIT_CONFIG_LIST_UPDATED"or event=="TRAIT_NODE_CHANGED"or event=="PLAYER_SPECIALIZATION_CHANGED"and public(unit)and unit=="player"then self.talentDirty=true end
    if event=="ENCOUNTER_WARNING"or event:match("^ENCOUNTER_TIMELINE_")then self:TimelineEvent(event,unit);return end
    if event=="PLAYER_ENTERING_WORLD"then self.bosses=nil;self.loadoutChanges=nil end
    if event=="ADDON_LOADED"and bossEnabled()then self:BossRegister()end
    self:EncounterEvent(event,unit,cast,spell,...)
    if event=="PLAYER_LOGIN"then C_Timer.NewTicker(.2,function()local ok=pcall(O.Tick,O);if not ok then if O.root then O.root:Hide()end;if O.strip then O.strip.frame:Hide()end end end)
    elseif event=="UNIT_SPELLCAST_SUCCEEDED"then
        if LUI:DB().overlaysEnabled and public(spell)and type(spell)=="number"then O.cast=spell;O.castSeq=O.castSeq+1 end
    elseif event=="PLAYER_TARGET_CHANGED"then self:UnitChanged("target")
    elseif event=="PLAYER_FOCUS_CHANGED"then self:UnitChanged("focus")
    elseif event=="UNIT_PET"then if public(unit)and unit=="player"then self:UnitChanged("pet")end
    elseif event=="NAME_PLATE_UNIT_ADDED"or event=="NAME_PLATE_UNIT_REMOVED"or event=="ARENA_OPPONENT_UPDATE"then
        if public(unit)and type(unit)=="string"then self:UnitChanged(unit)end
    else
        for token in pairs(self.unitGenerations or {})do
            if event=="PLAYER_ENTERING_WORLD"or event=="GROUP_ROSTER_UPDATE"and (token:match("^party")or token:match("^raid"))or event=="INSTANCE_ENCOUNTER_ENGAGE_UNIT"and token:match("^boss")then self:UnitChanged(token)end
        end
    end
end
local frame=CreateFrame("Frame")
for _,event in ipairs({"ADDON_LOADED","ENCOUNTER_START","ENCOUNTER_END","ENCOUNTER_STATE_CHANGED","BOSS_KILL","PLAYER_LOGIN","PLAYER_TARGET_CHANGED","PLAYER_FOCUS_CHANGED","UNIT_PET","NAME_PLATE_UNIT_ADDED","NAME_PLATE_UNIT_REMOVED","ARENA_OPPONENT_UPDATE","GROUP_ROSTER_UPDATE","INSTANCE_ENCOUNTER_ENGAGE_UNIT","PLAYER_ENTERING_WORLD"})do frame:RegisterEvent(event)end
frame:RegisterUnitEvent("UNIT_SPELLCAST_SUCCEEDED","player")
frame:RegisterUnitEvent("UNIT_FLAGS","pet")
frame:SetScript("OnEvent",function(_,event,...)O:Event(event,...)end)

for _,event in ipairs({"ENCOUNTER_WARNING","ENCOUNTER_TIMELINE_EVENT_ADDED","ENCOUNTER_TIMELINE_EVENT_STATE_CHANGED","ENCOUNTER_TIMELINE_EVENT_REMOVED","ENCOUNTER_TIMELINE_VIEW_ACTIVATED","ENCOUNTER_TIMELINE_VIEW_DEACTIVATED","ENCOUNTER_TIMELINE_STATE_UPDATED"})do pcall(frame.RegisterEvent,frame,event)end

for _,event in ipairs({"PLAYER_TALENT_UPDATE","TRAIT_CONFIG_UPDATED","TRAIT_CONFIG_LIST_UPDATED","TRAIT_NODE_CHANGED","PLAYER_SPECIALIZATION_CHANGED"})do pcall(frame.RegisterEvent,frame,event)end

for _,event in ipairs({"PLAYER_EQUIPMENT_CHANGED","GET_ITEM_INFO_RECEIVED","EQUIPMENT_SETS_CHANGED","EQUIPMENT_SWAP_FINISHED","UNIT_INVENTORY_CHANGED"})do pcall(frame.RegisterEvent,frame,event)end
