-- Frame integration for native aura rendering. Never infer a unit from a frame index.
local _,LUI=...
if LUI.FrameAuras then return end
local F={bindings={},preview=false,samples={},pending=true,elapsed=0,revision=0,failures={}}
LUI.FrameAuras=F
local function public(value,kind)
    if issecretvalue and issecretvalue(value) then return nil end
    if kind and type(value)~=kind then return nil end
    return value
end
local function call(fn,kind,...)
    if type(fn)~="function" then return nil end
    local ok,value=pcall(fn,...)
    if ok then return public(value,kind) end
end
local function frameUnit(frame)
    local ok,attribute=pcall(function()return frame:GetAttribute("unit")end)
    if not ok or issecretvalue and issecretvalue(attribute) then return nil end
    local fieldOK,field=pcall(function()return frame.unit end)
    if not fieldOK or issecretvalue and issecretvalue(field) then return nil end
    attribute=public(attribute,"string");field=public(field,"string")
    if attribute and field and attribute~=field then return nil end
    return attribute or field
end
local function usable(frame)
    frame=public(frame)
    if not frame then return false end
    if call(function()return frame:IsForbidden()end,"boolean")~=false then return false end
    return call(function()return frame:IsVisible()end,"boolean")==true
end
function F:AllowsUnit(unit)
    local db=LUI:DB()
    if unit=="player" then return db.nativePlayer end
    if unit=="pet" then return db.nativePets and db.nativePlayer end
    local party=unit:match("^party([1-4])$")
    if party then return db.nativeParty end
    local raid=tonumber(unit:match("^raid(%d+)$"))
    if raid and raid>=1 and raid<=40 then return db.nativeRaid end
    if unit:match("^partypet[1-4]$") then return db.nativePets and db.nativeParty end
    local raidpet=tonumber(unit:match("^raidpet(%d+)$"))
    return not not (raidpet and raidpet>=1 and raidpet<=40 and db.nativePets and db.nativeRaid)
end
function F:ContextEnabled()
    local db=LUI:DB()
    if not db.cdEnabled or not db.nativeFrames then return false end
    if C_DelvesUI and call(C_DelvesUI.IsInDelve,"boolean")==true then return db.nativeDelve end
    local _,kind=IsInInstance()
    local key=({none="nativeWorld",party="nativeDungeon",raid="nativeRaidContext",arena="nativeArena",pvp="nativeBattleground",scenario="nativeDelve"})[kind or "none"]
    return key and db[key] or false
end
function F:Discover()
    local result,seen={},{}
    local function consider(frame,provider)
        if not usable(frame) then return end
        local unit=frameUnit(frame)
        if not unit or seen[unit] or not self:AllowsUnit(unit) then return end
        if call(UnitExists,"boolean",unit)~=true or call(UnitIsVisible,"boolean",unit)~=true then return end
        seen[unit]=true;result[#result+1]={frame=frame,unit=unit,provider=provider}
    end
    local provider=LUI:DB().nativeProvider
    if provider==1 or provider==2 then
        local all=call(DandersFrames_GetAllFrames,"table")
        if all then local count=0;for _,frame in pairs(all)do
            consider(frame,"DandersFrames");count=count+1;if count>=160 then break end
        end end
        if LUI:DB().nativePets and DandersFrames then
            for _,name in ipairs({"petFrames","partyPetFrames","raidPetFrames","arenaPetFrames"})do
                local frames=public(DandersFrames[name],"table")
                if frames then
                    local count=0;for _,frame in pairs(frames)do
                        consider(frame,"DandersFrames");count=count+1;if count>=80 then break end
                    end
                end
            end
        end
        local lookup=DandersFrames and DandersFrames.Api and DandersFrames.Api.GetFrameForUnit
        if type(lookup)=="function" then
            consider(call(lookup,nil,"player","party"),"DandersFrames")
            for i=1,4 do consider(call(lookup,nil,"party"..i,"party"),"DandersFrames")end
            if LUI:DB().nativeRaid then for i=1,40 do consider(call(lookup,nil,"raid"..i,"raid"),"DandersFrames")end end
        end
    end
    if provider==1 or provider==3 then
        consider(PlayerFrame,"Blizzard");consider(PetFrame,"Blizzard")
        for i=1,5 do consider(_G["CompactPartyFrameMember"..i],"Blizzard")end
        for i=1,4 do
            consider(PartyFrame and PartyFrame["MemberFrame"..i],"Blizzard")
            consider(_G["PartyMemberFrame"..i],"Blizzard")
            consider(_G["PartyMemberFrame"..i.."PetFrame"],"Blizzard")
        end
        for i=1,40 do consider(_G["CompactRaidFrame"..i],"Blizzard")end
        for group=1,8 do for member=1,5 do consider(_G["CompactRaidGroup"..group.."Member"..member],"Blizzard")end end
    end
    return result
end
local anchors={
    {"BOTTOMRIGHT","TOPLEFT",-1,1},{"BOTTOM","TOP",0,1},{"BOTTOMLEFT","TOPRIGHT",1,1},
    {"RIGHT","LEFT",-1,0},{"CENTER","CENTER",0,0},{"LEFT","RIGHT",1,0},
    {"TOPRIGHT","BOTTOMLEFT",-1,-1},{"TOP","BOTTOM",0,-1},{"TOPLEFT","BOTTOMRIGHT",1,-1},
}
local regions={
    {id="CC",key="cc",label="CC",maximum="nativeMaxCC",spells={118,408,853}},
    {id="Debuffs",key="debuffs",label="Debuffs",maximum="nativeMaxDebuffs",spells={589,172,980}},
    {id="Defensives",key="defensives",label="Defensives",maximum="nativeMaxDefensives",spells={871,33206,1022}},
    {id="Buffs",key="buffs",label="Important buffs",maximum="nativeMaxBuffs",spells={10060,2825,12042}},
}
local regionByID={};for _,region in ipairs(regions)do regionByID[region.id]=region end
local highlightColors={cc={1,.3,.15,.8},debuffs={.7,.3,1,.8},defensives={.2,.65,1,.8},buffs={1,.78,.12,.8}}
local growths={
    {vertical=false,left=false,up=false},{vertical=false,left=true,up=false},
    {vertical=false,left=false,up=true},{vertical=false,left=true,up=true},
    {vertical=true,left=false,up=false},{vertical=true,left=false,up=true},
    {vertical=true,left=true,up=false},{vertical=true,left=true,up=true},
}
function F:LayoutContext(unit)
    unit=public(unit,"string")
    if unit and (unit:match("^raid%d+$") or unit:match("^raidpet%d+$")) then return "Raid" end
    if (unit=="player" or unit=="pet") and call(IsInRaid,"boolean")==true then return "Raid" end
    return "Party"
end
local function contextName(value)
    return public(value,"string")=="Raid" and "Raid" or "Party"
end
local function selectedEditor()
    local selected=public(LUI.frameLayoutEdit,"table")
    if not selected then return nil end
    local region=public(selected.region,"string");local context=public(selected.context,"string")
    if not regionByID[region] or (context~="Party" and context~="Raid") then return nil end
    return {region=region,context=context}
end
function F:Options(target,regionID,context)
    local db=LUI:DB();local region=regionByID[public(regionID,"string")] or regions[1]
    context=context and contextName(context) or self:LayoutContext(frameUnit(target))
    local prefix="native"..region.id..context
    local function setting(name,fallback)
        local value=public(db[prefix..name]);if value~=nil then return value end
        value=public(db["native"..name]);if value~=nil then return value end
        return fallback
    end
    local anchor=anchors[setting("Anchor",6)] or anchors[6]
    local defaultGrowth=(anchor[1]:find("RIGHT") and 2 or 1)+(anchor[1]:find("BOTTOM") and 2 or 0)
    local growth=setting("Growth",defaultGrowth);if not growths[growth] then growth=defaultGrowth end
    local gap=setting("Gap",6)
    local options={relativeFrame=target,anchorPoint=anchor[1],relativePoint=anchor[2],
        offsetX=anchor[3]*gap+setting("OffsetX",0),offsetY=anchor[4]*gap+setting("OffsetY",0),
        size=setting("Size",26),spacing=setting("Spacing",2),perRow=setting("PerRow",6),fontSize=setting("FontSize",12),
        growth=growth,timers=setting("Timers",true),stacks=setting("Stacks",true),borders=setting("Borders",true),
        glow=setting("Glow",false),swipe=setting("Swipe",true),reverse=setting("Reverse",false),tooltips=setting("Tooltips",true),groups={}}
    for _,other in ipairs(regions)do
        options.groups[other.key]={enabled=other==region and db["native"..other.id]==true,max=other==region and db[other.maximum] or 0}
    end
    return options
end
function F:DrawSample(target,index,regionID,context)
    local region=regionByID[public(regionID,"string")] or regions[1]
    context=context and contextName(context) or self:LayoutContext(frameUnit(target))
    local options=self:Options(target,region.id,context);local sample=self.samples[index]
    if not sample then
        sample=CreateFrame("Frame",nil,UIParent);sample.icons={};self.samples[index]=sample
        sample.title=sample:CreateFontString(nil,"OVERLAY","GameFontNormal")
        sample.title:SetPoint("BOTTOMLEFT",sample,"TOPLEFT",0,4)
        sample:SetFrameStrata("DIALOG")
    end
    sample.region=region.id;sample.context=context;sample.target=target
    local editor=selectedEditor();local selected=editor and editor.region==region.id and editor.context==context
    sample.title:SetText("LamdaCD preview: "..region.label)
    sample.title:SetTextColor(selected and 1 or .8,selected and .82 or .85,selected and .25 or .9,1)
    local group=options.groups[region.key]
    -- Synthetic icons fill the configured capacity. Defensives has separate
    -- native personal/external groups, each with the same per-group maximum.
    local count=group.enabled and group.max*(region.key=="defensives" and 2 or 1) or 0
    local growth=growths[options.growth]
    local primary=math.min(options.perRow,math.max(1,count));local secondary=math.max(1,math.ceil(count/primary))
    local cols=growth.vertical and secondary or primary;local rows=growth.vertical and primary or secondary
    sample:ClearAllPoints();sample:SetPoint(options.anchorPoint,target,options.relativePoint,options.offsetX,options.offsetY)
    sample:SetSize(cols*options.size+(cols-1)*options.spacing,rows*options.size+(rows-1)*options.spacing)
    for i=1,math.max(count,#sample.icons)do
        local icon=sample.icons[i]
        if not icon then
            icon=CreateFrame("Frame",nil,sample,"BackdropTemplate");sample.icons[i]=icon
            icon:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
            icon.texture=icon:CreateTexture(nil,"ARTWORK");icon.texture:SetAllPoints();icon.texture:SetTexCoord(.07,.93,.07,.93)
            icon.cooldown=CreateFrame("Cooldown",nil,icon);icon.cooldown:SetAllPoints()
            icon.cooldown:SetHideCountdownNumbers(true);icon.cooldown:SetDrawEdge(false)
            icon.timer=icon:CreateFontString(nil,"OVERLAY","GameFontNormal")
            icon.timer:SetPoint("CENTER");icon.timer:SetText("8")
            icon.stack=icon:CreateFontString(nil,"OVERLAY","GameFontNormalSmall");icon.stack:SetPoint("BOTTOMRIGHT",1,-1);icon.stack:SetText("2")
        end
        icon:SetShown(i<=count)
        if i<=count then
            local along=(i-1)%primary;local across=math.floor((i-1)/primary)
            local col=growth.vertical and across or along;local row=growth.vertical and along or across
            if growth.left then col=cols-1-col end;if growth.up then row=rows-1-row end
            icon:ClearAllPoints();icon:SetPoint("TOPLEFT",col*(options.size+options.spacing),-row*(options.size+options.spacing))
            icon:SetSize(options.size,options.size)
            local spell=region.spells[(i-1)%#region.spells+1]
            icon.texture:SetTexture(C_Spell and call(C_Spell.GetSpellTexture,nil,spell) or 134400)
            local color=options.glow and highlightColors[region.key] or {.04,.05,.07,1}
            icon:SetBackdropBorderColor(color[1],color[2],color[3],(options.borders or options.glow) and color[4] or 0)
            icon.cooldown:SetShown(options.swipe);icon.cooldown:SetReverse(options.reverse);icon.cooldown:SetSwipeColor(0,0,0,.62)
            icon.cooldown:SetCooldown((GetTime and GetTime() or 0)-2,10)
            icon.timer:SetFont(STANDARD_TEXT_FONT,math.min(options.fontSize,options.size-3),"OUTLINE");icon.timer:SetShown(options.timers)
            icon.stack:SetFont(STANDARD_TEXT_FONT,math.max(8,math.min(options.fontSize-1,options.size-3)),"OUTLINE");icon.stack:SetShown(options.stacks and i==2)
        end
    end
    sample:SetShown(count>0)
end
function F:HideSamples()
    for _,sample in ipairs(self.samples)do sample:Hide()end
    if self.demo then self.demo:Hide()end
end
function F:DemoFrame(context)
    context=contextName(context)
    if not self.demo then
        local frame=CreateFrame("Frame",nil,UIParent,"BackdropTemplate");self.demo=frame
        frame:SetPoint("CENTER",-100,40);frame:SetFrameStrata("DIALOG")
        frame:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
        frame:SetBackdropColor(.08,.18,.22,.95);frame:SetBackdropBorderColor(.2,.75,.65,1)
        frame.title=frame:CreateFontString(nil,"OVERLAY","GameFontNormal");frame.title:SetPoint("CENTER")
    end
    self.demo.context=context
    self.demo:SetSize(context=="Raid" and 120 or 170,context=="Raid" and 42 or 54)
    self.demo.title:SetText(context=="Raid" and "Sample raid frame" or "Sample party frame")
    self.demo:Show();return self.demo
end
function F:RequestRefresh()
    self.pending=true;self.elapsed=0;self.debounce=.25;self.revision=self.revision+1
end
function F:Refresh()
    self:RequestRefresh()
end
function F:SetPreview(enabled)
    if InCombatLockdown() then return end
    self.preview=not not enabled;self:Update()
    if LUI.RefreshUI then LUI:RefreshUI()end
end
function F:AdoptStandaloneDisplay()
    if self.legacySuppressed or InCombatLockdown() then return end
    if type(lamdaCDDB)~="table" or not SlashCmdList or type(SlashCmdList.LAMDACD)~="function" then return end
    -- This is our own predecessor, not MiniAuras or the frame provider. Preserve
    -- its visibility preference before retiring the duplicate display.
    local hub=LUI:ProfileDB()
    if hub.standaloneCDVisible==nil then hub.standaloneCDVisible=lamdaCDDB.visible==true end
    self.legacySuppressed=pcall(SlashCmdList.LAMDACD,"hide")
    if self.legacySuppressed and C_AddOns and type(C_AddOns.DisableAddOn)=="function" then
        pcall(C_AddOns.DisableAddOn,"lamdaCD")
    end
end
local function hideBinding(binding)
    for _,entry in pairs(binding.regions)do entry.handle:SetShown(false)end
end
function F:Update()
    if not self.providerHook and DandersFrames and type(DandersFrames.RegisterCallback)=="function" then
        self.providerHook=pcall(DandersFrames.RegisterCallback,self,"OnFramesSorted",function()F:Update()end)
    end
    local combat=InCombatLockdown()
    if combat then self.preview=false end
    if not combat then self:HideSamples()end
    local enabled=self:ContextEnabled();local discovered=enabled and self:Discover() or {}
    local keep={};self.frameCount=#discovered;self.lastError=nil
    local db=LUI:DB()
    for _,target in ipairs(discovered)do
        local key=target.frame;local binding=self.bindings[key]
        local context=self:LayoutContext(target.unit)
        if not binding then
            binding={unit=target.unit,context=context,regions={}};self.bindings[key]=binding
        end
        keep[key]=true
        local allReady=true;local anyReady=false;local unitsMatch=true
        for _,region in ipairs(regions)do
            local active=db["native"..region.id]==true and db[region.maximum]>0
            local entry=binding.regions[region.id]
            if entry and entry.unit~=target.unit then
                entry.handle:SetShown(false)
                if not combat and entry.handle:SetUnit(target.unit) then entry.unit=target.unit end
            end
            self.failures[key]=self.failures[key] or {}
            local failure=self.failures[key][region.id]
            if active and not entry and not combat and LUI.NativeAuras and
                (not failure or failure.revision~=self.revision or failure.unit~=target.unit or failure.context~=context) then
                local handle,reason=LUI.NativeAuras:Create(UIParent,target.unit,self:Options(target.frame,region.id,context))
                if handle then
                    entry={handle=handle,unit=target.unit,context=context,attemptedContext=context,attemptedUnit=target.unit,revision=self.revision}
                    binding.regions[region.id]=entry;self.failures[key][region.id]=nil
                else self.failures[key][region.id]={revision=self.revision,unit=target.unit,context=context,error=reason} end
            end
            if entry then
                if active and not combat and (entry.revision~=self.revision or entry.attemptedContext~=context or entry.attemptedUnit~=target.unit) then
                    if entry.handle:Configure(self:Options(target.frame,region.id,context)) then entry.context=context end
                    entry.attemptedContext=context;entry.attemptedUnit=target.unit;entry.revision=self.revision
                end
                local error=entry.handle.GetError and entry.handle:GetError()
                if active and error then self.lastError=error end
                local ready=entry.unit==target.unit and entry.context==context and not error
                entry.handle:SetShown(active and not self.preview and ready)
                if entry.unit~=target.unit then unitsMatch=false end
                if active then anyReady=anyReady or ready;allReady=allReady and ready end
            elseif active then
                allReady=false
                local failed=self.failures[key][region.id]
                if failed then self.lastError=failed.error or self.lastError end
            end
        end
        if not combat and unitsMatch then binding.unit=target.unit;binding.context=context end
        if not combat and anyReady and allReady then self:AdoptStandaloneDisplay()end
    end
    for key,binding in pairs(self.bindings)do if not keep[key] then hideBinding(binding)end end
    if self.preview and not combat and enabled then
        local editor=selectedEditor();local previewTargets={}
        for _,target in ipairs(discovered)do
            local context=self:LayoutContext(target.unit)
            if not editor or editor.context==context then previewTargets[#previewTargets+1]={frame=target.frame,context=context}end
        end
        if #previewTargets==0 then
            local context=editor and editor.context or "Party"
            previewTargets[1]={frame=self:DemoFrame(context),context=context}
        end
        local index=0
        for _,target in ipairs(previewTargets)do for _,region in ipairs(regions)do
            if db["native"..region.id]==true and db[region.maximum]>0 then
                index=index+1;self:DrawSample(target.frame,index,region.id,target.context)
            end
        end end
    end
    if not combat then self.pending=false end
end
function F:Diagnostic()
    local ready,reason
    if LUI.NativeAuras then ready,reason=LUI.NativeAuras:IsSupported()end
    local status=ready and "available" or (reason=="combat" and "not checked during combat" or "unavailable")
    print("LamdaCD: "..(LUI:DB().cdEnabled and "enabled" or "disabled")..", "..(self.frameCount or 0).." unit frames, native auras "..status)
    if self.lastError then print("LamdaCD: "..tostring(self.lastError))end
end
local events=CreateFrame("Frame")
for _,event in ipairs({"PLAYER_LOGIN","PLAYER_ENTERING_WORLD","GROUP_ROSTER_UPDATE","PLAYER_REGEN_ENABLED","PLAYER_REGEN_DISABLED","UNIT_PET","ADDON_LOADED"})do events:RegisterEvent(event)end
events:SetScript("OnEvent",function(_,event)
    if event=="PLAYER_REGEN_DISABLED" then
        F.preview=false;F:HideSamples();F:Update()
    else F:RequestRefresh()end
end)
events:SetScript("OnUpdate",function(_,elapsed)
    F.elapsed=F.elapsed+elapsed
    if F.elapsed>=(F.pending and (F.debounce or .25) or 1)then F.elapsed=0;F:Update()end
end)
