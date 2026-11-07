-- Public, player-owned execution measurements. Restricted values stay unknown.
-- Disk-only support damage is reviewed separately by the engine.
local _,LUI=...
local A={views={},segments={},seen={},sampleClock=0}
LUI.Augmentation=A
local spells={[395152]="Ebon Might",[409311]="Prescience",[360827]="Blistering Scales",
    [403631]="Breath of Eons",[395160]="Eruption",[396286]="Upheaval"}
A.spells=spells
local function public(value)return not (issecretvalue and issecretvalue(value))end
local function readable(value)return public(value) and not (issecrettable and type(value)=="table" and issecrettable(value))end
local function number(value)return public(value) and type(value)=="number" and value==value and value>=0 and value<math.huge and value or nil end
local function text(value,fallback)
    if not public(value) or type(value)~="string" then return fallback or "Encounter" end
    value=value:gsub("|"," "):gsub("[%c]"," ")
    local last=70
    while #value>last and value:byte(last+1)>=128 and value:byte(last+1)<192 do last=last-1 end
    return value:sub(1,last)
end
function A:Eligible()
    if WOW_PROJECT_ID~=WOW_PROJECT_MAINLINE then return false end
    local index=GetSpecialization and GetSpecialization()
    local spec=index and GetSpecializationInfo(index)
    return public(spec) and spec==1473
end
function A:History()
    if type(LamdaUIDB)~="table" then LamdaUIDB={} end
    if type(LamdaUIDB.augmentation)~="table" then LamdaUIDB.augmentation={} end
    local guid=UnitGUID("player")
    if not public(guid) or type(guid)~="string" then return {} end
    local db=LamdaUIDB.augmentation
    if type(db[guid])~="table" then db[guid]={} end
    return db[guid]
end
-- Query a single spell only after Blizzard's per-spell guard says it is public.
-- A nil aura is absence only on a visible unit after that guard succeeds.
local function aura(unit,id)
    if not C_Secrets or type(C_Secrets.ShouldSpellAuraBeSecret)~="function" or not C_UnitAuras then return nil end
    local ok,secret=pcall(C_Secrets.ShouldSpellAuraBeSecret,id)
    if not ok or not public(secret) or secret~=false then return nil end
    if UnitIsVisible then
        local visible=UnitIsVisible(unit);if not public(visible) or not visible then return nil end
    end
    local fn=C_UnitAuras.GetUnitAuraBySpellID
    if type(fn)~="function" then return nil end
    local success,value=pcall(fn,unit,id)
    if not success or not readable(value) then return nil end
    if value==nil then return false end
    local source=value.sourceUnit
    if not public(source) or type(source)~="string" then return nil end
    if source=="player" then return true end
    -- The API returns the first matching aura. A different Aug can hide ours.
    return nil
end
function A:Observation()
    local ebon=aura("player",395296)
    local count,known=0,true
    local raid=IsInRaid and IsInRaid()
    local n=GetNumGroupMembers and GetNumGroupMembers() or 0
    if not public(raid) or not number(n) then return {ebon=ebon} end
    local units={}
    if raid then for i=1,math.min(40,n)do units[#units+1]="raid"..i end
    else units[1]="player";for i=1,math.max(0,math.min(4,n-1))do units[#units+1]="party"..i end end
    for _,unit in ipairs(units)do
        local present=aura(unit,410089)
        if present==nil then known=false elseif present then count=count+1 end
    end
    return {ebon=ebon,prescience=known and count or nil}
end
function A:Sample(now)
    now=now or GetTime()
    local dt=self.previousAt and now-self.previousAt or 0
    for _,s in pairs(self.segments)do
        if dt>0 and dt<=1.5 and self.previous then
            if self.previous.ebon~=nil then
                s.ebonKnown=s.ebonKnown+dt
                if self.previous.ebon then s.ebonUp=s.ebonUp+dt end
            end
            if self.previous.prescience~=nil then
                s.prescienceKnown=s.prescienceKnown+dt
                s.prescienceTargets=s.prescienceTargets+dt*self.previous.prescience
            end
        end
    end
    self.previousAt=now;self.previous=self:Observation()
end
function A:Start(kind,id,name)
    if not self.enabled or self.segments[kind] then return end
    -- Do not integrate the previous segment's sample across a new boundary.
    self:Sample()
    self.segments[kind]={kind=kind,encounterID=id,name=text(name),guid=self.guid,started=GetTime(),
        startedAt=time(),casts={},ebonKnown=0,ebonUp=0,prescienceKnown=0,prescienceTargets=0,deaths=0}
    self:Draw()
end
function A:Finish(kind,id,success)
    local s=self.segments[kind]
    if not s or (id and id~=s.encounterID) then return end
    self:Sample();self.segments[kind]=nil
    s.seconds=math.max(0,GetTime()-s.started);s.endedAt=time();s.success=success==true;s.groupVersion=self.groupVersion or 0
    if s.seconds<1 then return end
    local history=self:History();table.insert(history,1,s);while #history>20 do table.remove(history)end
    local db=LUI:DB()
    if (kind=="boss" and db.augAfterBoss) or (kind=="key" and db.augAfterKey) then
        if db.augPrivateSummary then self:Print(s)end
        if db.augGroupSummary and not(LUI.Meters and LUI.Meters.QueueSummary)then self:Share(s,true)end
    end
    self:Draw()
end
function A:Cast(castGUID,id)
    if not self.enabled or not public(id) or not spells[id] then return end
    if public(castGUID) and type(castGUID)=="string" then
        if self.seen[castGUID] then return end
        self.seen[castGUID]=GetTime()
    end
    for _,s in pairs(self.segments)do s.casts[id]=(s.casts[id] or 0)+1 end
end
local function uptime(s)
    local seconds=s.seconds or math.max(0,GetTime()-s.started)
    local coverage=seconds>0 and math.min(100,100*s.ebonKnown/seconds) or 0
    if s.ebonKnown<1 then return "Ebon uptime unavailable" end
    return string.format("Ebon %.0f%% (%.0f%% observed)",100*s.ebonUp/s.ebonKnown,coverage)
end
function A:Summary(s)
    s=s or self:History()[1];if not s then return "LamdaMeters: no completed boss or key yet." end
    return string.format("LamdaMeters: %s %s | %s | Prescience %d casts, Eons %d",
        text(s.name),s.success and "complete" or "ended",uptime(s),s.casts[409311] or 0,s.casts[403631] or 0)
end
function A:Print(s)
    local message=self:Summary(s)
    if DEFAULT_CHAT_FRAME then DEFAULT_CHAT_FRAME:AddMessage(message)end
    return message
end
function A:Share(s,automatic)
    s=s or self:History()[1]
    if not automatic then
        local meters=LUI.Meters
        return meters and meters.BeginSummary and not not meters:BeginSummary(s and s.kind or "key")or false
    end
    if not self.enabled or not s or s.guid~=self.guid or not self:Eligible() then return false end
    -- Sharing old reports after joining another group would mislead that group.
    if time()-(s.endedAt or 0)>120 or s.groupVersion~=(self.groupVersion or 0) then return false end
    local key=tostring(s.startedAt)..":"..s.kind
    local keyCompletion=automatic and s.kind=="key" and self.lastShareKind=="boss"
    if self.shared==key or (self.lastShare and GetTime()-self.lastShare<15 and not keyCompletion)then return false end
    local channel
    if IsInGroup and IsInGroup(LE_PARTY_CATEGORY_INSTANCE)then channel="INSTANCE_CHAT"
    elseif IsInRaid and IsInRaid()then channel="RAID"
    elseif IsInGroup and IsInGroup()then channel="PARTY" end
    if not channel then return false end
    if automatic and not LUI:DB().augGroupSummary then return false end
    -- Group output comes from the adjusted damage ledger, never cast counts.
    local meters=LUI.Meters
    if not meters or not meters.BeginSummary then return false end
    local opened=meters:BeginSummary(s.kind)
    if opened then self.shared=key;self.lastShare=GetTime();self.lastShareKind=s.kind end
    return not not opened
end
function A:Review()
    local data=LamdaAugResults
    if type(data)~="table" or data.schema~=1 or data.client~="retail" or type(data.reports)~="table" then return nil end
    for _,r in ipairs(data.reports)do if r.guid==self.guid then return r end end
end
function A:Draw()
    local s=self.segments.key or self.segments.boss or self:History()[1]
    local line=s and (text(s.name).." | "..uptime(s)) or "Waiting for a boss or Mythic+ key"
    local casts=s and string.format("Prescience %d  |  Ebon %d  |  Eons %d",s.casts[409311] or 0,s.casts[395152] or 0,s.casts[403631] or 0) or "No session collected yet"
    for _,draw in ipairs(self.views)do draw(s,line,casts)end
end
function A:Configure()
    local db=LUI:DB();local guid=UnitGUID("player");if not public(guid) or type(guid)~="string" then guid=nil end
    local enabled=not not (db.augEnabled and self:Eligible() and guid)
    if self.enabled~=enabled or self.guid~=guid or self.profile~=(LUI:ProfileDB().active) then
        self.segments={};self.previous=nil;self.previousAt=nil;self.seen={}
    end
    self.enabled=not not enabled;self.guid=public(guid) and guid or nil;self.profile=LUI:ProfileDB().active
    if self.enabled then
        local width,height=number(UIParent:GetWidth()),number(UIParent:GetHeight())
        if width and width>=1 then db.augUIWidth=math.min(16000,math.floor(width+.5))end
        if height and height>=1 then db.augUIHeight=math.min(16000,math.floor(height+.5))end
    end
    if db.augEnabled and WOW_PROJECT_ID==WOW_PROJECT_MAINLINE and not LUI.ForeverEngine and guid and db.augAutoLog and IsInInstance then
        local inside,kind=IsInInstance()
        if inside and (kind=="party" or kind=="raid" or kind=="scenario") and LoggingCombat then
            if not LoggingCombat()then LoggingCombat(true)end
        end
    end
    self:Draw()
end
function A:InitializeMeter()
    if LUI.Meters then LUI.Meters:Initialize() end
end
local events=CreateFrame("Frame");A.events=events
for _,event in ipairs({"PLAYER_LOGIN","PLAYER_ENTERING_WORLD","PLAYER_SPECIALIZATION_CHANGED","GROUP_ROSTER_UPDATE",
    "UNIT_SPELLCAST_SUCCEEDED","ENCOUNTER_START","ENCOUNTER_END","CHALLENGE_MODE_START","CHALLENGE_MODE_COMPLETED","CHALLENGE_MODE_RESET","PLAYER_DEAD"})do events:RegisterEvent(event)end
events:SetScript("OnEvent",function(_,event,...)
    if event=="PLAYER_LOGIN" then A:InitializeMeter();A:Configure()
    elseif event=="PLAYER_ENTERING_WORLD" then A.segments={};A.previous=nil;A.previousAt=nil;A:Configure()
    elseif event=="PLAYER_SPECIALIZATION_CHANGED" then A:Configure()
    elseif event=="GROUP_ROSTER_UPDATE" then A.previous=nil;A.previousAt=nil;A.groupVersion=(A.groupVersion or 0)+1
    elseif not A.enabled then return
    elseif event=="UNIT_SPELLCAST_SUCCEEDED" then local unit,guid,id=...;if public(unit) and unit=="player" then A:Cast(guid,id)end
    elseif event=="ENCOUNTER_START" then local id,name=...;if public(id) then A:Start("boss",id,name)end
    elseif event=="ENCOUNTER_END" then local id,_,_,_,success=...;if public(id) and public(success) then A:Finish("boss",id,success==1)end
    elseif event=="CHALLENGE_MODE_START" then
        local id=C_ChallengeMode and C_ChallengeMode.GetActiveChallengeMapID and C_ChallengeMode.GetActiveChallengeMapID()
        local name=number(id) and C_ChallengeMode.GetMapUIInfo(id)
        A:Start("key",nil,text(name,"Mythic+"))
    elseif event=="CHALLENGE_MODE_COMPLETED" then A:Finish("key",nil,true)
    elseif event=="CHALLENGE_MODE_RESET" then A.segments.key=nil
    elseif event=="PLAYER_DEAD" then for _,s in pairs(A.segments)do s.deaths=s.deaths+1 end end
end)
events:SetScript("OnUpdate",function(_,elapsed)
    A.sampleClock=A.sampleClock+elapsed;if A.sampleClock<.5 then return end;A.sampleClock=0
    A:Configure()
    if A.enabled and next(A.segments)then A:Sample();A:Draw()end
    for id,at in pairs(A.seen)do if GetTime()-at>10 then A.seen[id]=nil end end
end)
SLASH_LAMDAAUG1="/laug"
SlashCmdList.LAMDAAUG=function(command)
    command=(command or ""):lower():match("^%s*(.-)%s*$")
    if command=="share" then A:Share()
    elseif command=="summary" then A:Print()
    else LUI:OpenUI();LUI:SelectPage("modules");LUI:SelectModule("augmentation")end
end
