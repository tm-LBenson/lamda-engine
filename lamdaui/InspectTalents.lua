local _,LUI=...
local inspect={cache={},diagnostics={},nextRequest=0,retry={},revision=0,refresh={},members={},attempts={},cycles={}}
LUI.InspectTalents=inspect
local obs=LUI.Observations
local num,str,public=obs.Number,obs.String,obs.Public
local function playerGUID(unit)
    local guid=str(UnitGUID(unit))
    if guid and guid:match("^Player%-")then return guid end
end
local function gap(code)inspect.diagnostics[code]=(inspect.diagnostics[code] or 0)+1 end
local function call(fn,...)
    if type(fn)~="function"then return nil end
    local ok,result=pcall(fn,...);if ok and public(result)then return result end
end
local function tableValue(value)if public(value) and type(value)=="table"then return value end end
local function showing(frame)return frame and call(frame.IsShown,frame)==true end
local function busy()
    return InCombatLockdown() or showing(InspectFrame) or (PlayerSpellsFrame and call(PlayerSpellsFrame.IsInspecting,PlayerSpellsFrame)==true)
end
function inspect:Enabled()return LUI:OptionalWork("resourceInspect") and obs:Enabled() and ((LUI:CDTrackingEnabled() and LamdaEngineDB.inspectTalents) or (LamdaEngineDB.kickEnabled and LamdaEngineDB.kickInspectTalents)) and not IsInRaid()end
function inspect:Read(unit)
    if InCombatLockdown() then return nil end
    local guid=playerGUID(unit);if not guid then return nil end
    local own=unit=="player"
    if not own and call(C_Traits and C_Traits.HasValidInspectData)~=true then return nil end
    local config
    if own then config=num(call(C_ClassTalents and C_ClassTalents.GetActiveConfigID))
    else config=Constants and Constants.TraitConsts and num(Constants.TraitConsts.INSPECT_TRAIT_CONFIG_ID)end
    if not config then return nil end
    if own and C_Traits and call(C_Traits.ConfigHasStagedChanges,config)==true then return nil end
    local spec
    if own then
        local index=num(call(C_SpecializationInfo and C_SpecializationInfo.GetSpecialization or GetSpecialization))
        if index then spec=num(call(C_SpecializationInfo and C_SpecializationInfo.GetSpecializationInfo or GetSpecializationInfo,index))end
    else spec=num(call(C_SpecializationInfo and C_SpecializationInfo.GetInspectSpecialization or GetInspectSpecialization,unit))end
    if not spec or spec<=0 then return nil end
    local info=tableValue(call(C_Traits.GetConfigInfo,config));local trees=info and tableValue(info.treeIDs)
    if not trees or #trees<1 or #trees>8 then return nil end
    local selected={};local scanned={};local count=0
    for _,tree in ipairs(trees)do
        tree=num(tree);if not tree then return nil end
        local nodes=tableValue(call(C_Traits.GetTreeNodes,tree));if not nodes or #nodes<1 or #nodes>500 then return nil end
        for _,nodeID in ipairs(nodes)do
            nodeID=num(nodeID);if not nodeID then return nil end
            if not scanned[nodeID]then
                scanned[nodeID]=true;count=count+1;if count>600 then return nil end
                local node=tableValue(call(C_Traits.GetNodeInfo,config,nodeID));if not node then return nil end
                local rank=num(node.activeRank);if rank==nil or rank<0 then return nil end
                if not public(node.subTreeActive)then return nil end
                if node.subTreeActive==false then rank=0 end
                if rank>0 then
                    local active=tableValue(node.activeEntry);local entryID=active and num(active.entryID)
                    if not entryID then return nil end
                    local entry=tableValue(call(C_Traits.GetEntryInfo,config,entryID));if not entry then return nil end
                    local definitionID=num(entry.definitionID)
                    -- Subtree selectors have no spell; ordinary selected entries must resolve.
                    if definitionID and definitionID>0 then
                        local definition=tableValue(call(C_Traits.GetDefinitionInfo,definitionID));if not definition then return nil end
                        if not public(definition.spellID)then return nil end
                        local spell=num(definition.spellID)
                        if definition.spellID~=nil and spell==nil then return nil end
                        spell=spell or 0
                        if LUI.ObservationSpells[spell] then
                            if rank>10 or rank~=math.floor(rank)then return nil end
                            selected[#selected+1]=table.concat({nodeID,entryID,rank,spell},",")
                        end
                    elseif not (num(entry.subTreeID) and num(entry.subTreeID)>0) then return nil end
                end
            end
        end
    end
    if guid~=str(UnitGUID(unit))then return nil end
    local at=num(GetServerTime());local game=str(call(GetBuildInfo));if not at or not game then return nil end
    table.sort(selected)
    if #selected>128 then return nil end
    self.revision=self.revision+1
    -- Split on entry boundaries; every part carries identity and build time.
    local chunks={};local chunk=""
    for _,entry in ipairs(selected)do
        if #chunk+#entry+1>180 then chunks[#chunks+1]=chunk;chunk=""end
        chunk=chunk=="" and entry or chunk..";"..entry
    end
    chunks[#chunks+1]=chunk
    if #chunks>24 then return nil end
    return {guid=guid,spec=spec,at=at,time=GetTime(),game=game,
        id=tostring(at)..":"..tostring(self.revision),chunks=chunks}
end
function inspect:Ready(guid)
    guid=str(guid);if not guid or not self:Enabled() or InCombatLockdown() then return end
    -- The event GUID must still identify the current unit, including manual inspections.
    local unit=obs:Unit(guid);if not unit or unit=="player"then return end
    if self.pending and self.pending.guid==guid then self.pending=nil end
    local result=self:Read(unit)
    if result then self.cache[guid]=result;self.refresh[guid]=nil;gap("completed")else gap("failed");self.retry[guid]=GetTime()+15 end
end
function inspect:RequestRefresh(reason)
    if not self:Enabled() then return end
    local mode=LUI:DB().scanTiming
    if reason=="key" and mode==1 then return end
    -- Retain the last complete result while a fresh query is in flight.
    for _,unit in ipairs({"player","party1","party2","party3","party4"})do
        if UnitExists(unit)then
            local guid=playerGUID(unit)
            if guid then self.refresh[guid]=true;self.retry[guid]=nil;self.attempts[guid]=0;self.cycles[guid]=GetTime() end
        end
    end
end
function inspect:Tick()
    if not self:Enabled()then self.pending=nil;self.cache={};self.refresh={};self.members={};self.attempts={};self.cycles={};self.retry={};return end
    local now=GetTime()
    local db=LUI:DB();local mode=db.scanTiming
    for guid in pairs(self.members)do
        if not obs:Unit(guid) then
            self.members[guid]=nil;self.cache[guid]=nil;self.retry[guid]=nil;self.refresh[guid]=nil;self.attempts[guid]=nil;self.cycles[guid]=nil
        end
    end
    for _,unit in ipairs({"player","party1","party2","party3","party4"})do
        local guid=UnitExists(unit) and playerGUID(unit)
        if guid and not self.members[guid] then
            self.members[guid]=true;self.attempts[guid]=0;self.cycles[guid]=now
            if mode~=3 or unit=="player"then self.refresh[guid]=true end
        end
        if guid and mode==4 and now-(self.cycles[guid] or now)>=db.scanMinutes*60 then
            self.refresh[guid]=true;self.attempts[guid]=0;self.cycles[guid]=now
        end
    end
    if LUI.overlayHidden or busy()then return end
    if self.pending then
        if not obs:Unit(self.pending.guid) or now-self.pending.at>5 then
            self.retry[self.pending.guid]=now+15;self.pending=nil;gap("failed")
        else return end
    end
    if now<self.nextRequest then return end
    for _,unit in ipairs({"player","party1","party2","party3","party4"})do
        if UnitExists(unit)then
            local guid=playerGUID(unit)
            if guid and self.refresh[guid] and (self.attempts[guid] or 0)<3 and now>=(self.retry[guid] or 0)then
                self.attempts[guid]=(self.attempts[guid] or 0)+1
                self.nextRequest=now+3;self.retry[guid]=now+15
                if unit=="player"then
                    local result=self:Read(unit)
                    if result then self.cache[guid]=result;self.refresh[guid]=nil;gap("completed")else gap("failed")end
                elseif call(CanInspect,unit,false)==true and call(UnitIsConnected,unit)==true then
                    self.pending={guid=guid,at=now}
                    local ok=pcall(NotifyInspect,unit);if not ok then self.pending=nil;gap("failed")end
                end
                return
            end
        end
    end
end
function inspect:BridgeLine(page,budget)
    if not self:Enabled()then return nil end
    local pages={};local guids={}
    for guid,cached in pairs(self.cache)do
        if obs:Unit(guid) and GetTime()-cached.time<=604800 then guids[#guids+1]=guid end
    end
    table.sort(guids)
    for _,guid in ipairs(guids)do
        local b=self.cache[guid]
        for part,chunk in ipairs(b.chunks)do
            pages[#pages+1]=table.concat({"B",(GetTime()-b.time>300 and "Held:" or "")..b.id,b.guid,b.at,b.spec,b.game,part,#b.chunks,chunk},"|")
        end
    end
    if #pages==0 then return nil end
    local line=pages[page%#pages+1];if #line<=budget then return line end
end
function inspect:Invalidate(unit)
    unit=str(unit)
    if unit and UnitExists(unit)then local guid=playerGUID(unit);if guid then
        self.cache[guid]=nil;self.retry[guid]=nil
        -- Own talents are local reads, not inspection requests. Remote scans
        -- retain the chosen timing; a known change discards stale results.
        if unit=="player"then self.refresh[guid]=true;self.attempts[guid]=0;self.cycles[guid]=GetTime() end
    end end
end
-- Cooperate with Blizzard's Inspect UI and other inspectors. Never clear their data.
if hooksecurefunc and NotifyInspect then
    hooksecurefunc("NotifyInspect",function()inspect.nextRequest=math.max(inspect.nextRequest,GetTime()+3)end)
end
local frame=CreateFrame("Frame")
for _,event in ipairs({"INSPECT_READY","PLAYER_SPECIALIZATION_CHANGED","TRAIT_CONFIG_UPDATED","PLAYER_ENTERING_WORLD"})do frame:RegisterEvent(event)end
frame:SetScript("OnEvent",function(_,event,arg)
    local timing=LUI.Performance and LUI.Performance:BeginWork()
    local ok=pcall(function()
        if event=="INSPECT_READY"then inspect:Ready(arg)
        elseif event=="PLAYER_SPECIALIZATION_CHANGED"then inspect:Invalidate(arg)
        elseif event=="TRAIT_CONFIG_UPDATED"then inspect:Invalidate("player")
        else inspect.pending=nil end
    end)
    if not ok then gap("failed")end
    if timing then LUI.Performance:EndWork("Talent inspection",timing)end
end)
local elapsed=0
frame:SetScript("OnUpdate",function(_,delta)
    elapsed=elapsed+delta;if elapsed<0.5 then return end;elapsed=0
    local timing=LUI.Performance and LUI.Performance:BeginWork()
    local ok=pcall(inspect.Tick,inspect);if not ok then gap("failed");inspect.nextRequest=GetTime()+3 end
    if timing then LUI.Performance:EndWork("Talent inspection",timing)end
end)
