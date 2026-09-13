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
function F:Options(target)
    local db=LUI:DB();local anchor=anchors[db.nativeAnchor] or anchors[6]
    return {relativeFrame=target,anchorPoint=anchor[1],relativePoint=anchor[2],
        offsetX=anchor[3]*db.nativeGap+db.nativeOffsetX,offsetY=anchor[4]*db.nativeGap+db.nativeOffsetY,
        size=db.nativeSize,spacing=db.nativeSpacing,perRow=db.nativePerRow,fontSize=db.nativeFontSize,
        timers=db.nativeTimers,stacks=db.nativeStacks,borders=db.nativeBorders,glow=db.nativeGlow,
        swipe=db.nativeSwipe,reverse=db.nativeReverse,groups={
            cc={enabled=db.nativeCC,max=db.nativeMaxCC},debuffs={enabled=db.nativeDebuffs,max=db.nativeMaxDebuffs},
            defensives={enabled=db.nativeDefensives,max=db.nativeMaxDefensives},buffs={enabled=db.nativeBuffs,max=db.nativeMaxBuffs}}}
end
local sampleSpells={cc=118,debuffs=589,defensives=871,buffs=10060}
function F:DrawSample(target,index)
    local options=self:Options(target);local sample=self.samples[index]
    if not sample then
        sample=CreateFrame("Frame",nil,UIParent);sample.icons={};self.samples[index]=sample
        sample.title=sample:CreateFontString(nil,"OVERLAY","GameFontNormal")
        sample.title:SetPoint("BOTTOMLEFT",sample,"TOPLEFT",0,4);sample.title:SetText("LamdaCD preview")
        sample:SetFrameStrata("DIALOG")
    end
    local list={}
    for _,key in ipairs({"cc","debuffs","defensives","buffs"})do
        if options.groups[key].enabled and options.groups[key].max>0 then
            list[#list+1]={spell=sampleSpells[key],category=key}
            if key=="defensives" then list[#list+1]={spell=33206,category=key}end
        end
    end
    local cols=math.min(options.perRow,math.max(1,#list));local rows=math.max(1,math.ceil(#list/cols))
    sample:ClearAllPoints();sample:SetPoint(options.anchorPoint,target,options.relativePoint,options.offsetX,options.offsetY)
    sample:SetSize(cols*options.size+(cols-1)*options.spacing,rows*options.size+(rows-1)*options.spacing)
    for i=1,5 do
        local icon=sample.icons[i]
        if not icon then
            icon=CreateFrame("Frame",nil,sample,"BackdropTemplate");sample.icons[i]=icon
            icon:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
            icon.texture=icon:CreateTexture(nil,"ARTWORK");icon.texture:SetAllPoints();icon.texture:SetTexCoord(.08,.92,.08,.92)
            icon.cooldown=CreateFrame("Cooldown",nil,icon);icon.cooldown:SetAllPoints()
            icon.cooldown:SetHideCountdownNumbers(true);icon.cooldown:SetDrawEdge(false)
            icon.timer=icon:CreateFontString(nil,"OVERLAY","GameFontNormal")
            icon.timer:SetPoint("CENTER");icon.timer:SetText("8")
            icon.stack=icon:CreateFontString(nil,"OVERLAY","GameFontNormalSmall");icon.stack:SetPoint("BOTTOMRIGHT",-1,1);icon.stack:SetText("2")
        end
        icon:SetShown(list[i]~=nil)
        if list[i] then
            local col=(i-1)%cols;local row=math.floor((i-1)/cols)
            if options.anchorPoint:find("RIGHT") then col=cols-1-col end
            if options.anchorPoint:find("BOTTOM") then row=rows-1-row end
            icon:ClearAllPoints();icon:SetPoint("TOPLEFT",col*(options.size+options.spacing),-row*(options.size+options.spacing))
            icon:SetSize(options.size,options.size)
            icon.texture:SetTexture(C_Spell and call(C_Spell.GetSpellTexture,nil,list[i].spell) or 134400)
            local category=list[i].category
            local highlighted=options.glow and (category=="cc" or category=="buffs")
            icon:SetBackdropBorderColor(highlighted and 1 or .04,highlighted and (category=="cc" and .3 or .78) or .05,highlighted and (category=="cc" and .15 or .12) or .07,(options.borders or highlighted) and 1 or 0)
            icon.cooldown:SetShown(options.swipe);icon.cooldown:SetReverse(options.reverse);icon.cooldown:SetSwipeColor(0,0,0,.62)
            icon.cooldown:SetCooldown((GetTime and GetTime() or 0)-2,10)
            icon.timer:SetFont(STANDARD_TEXT_FONT,math.min(options.fontSize,options.size-3),"OUTLINE");icon.timer:SetShown(options.timers)
            icon.stack:SetFont(STANDARD_TEXT_FONT,math.max(8,math.min(options.fontSize-1,options.size-3)),"OUTLINE");icon.stack:SetShown(options.stacks and i==2)
        end
    end
    sample:SetShown(#list>0)
end
function F:HideSamples()
    for _,sample in ipairs(self.samples)do sample:Hide()end
    if self.demo then self.demo:Hide()end
end
function F:DemoFrame()
    if not self.demo then
        local frame=CreateFrame("Frame",nil,UIParent,"BackdropTemplate");self.demo=frame
        frame:SetSize(170,54);frame:SetPoint("CENTER",-100,40);frame:SetFrameStrata("DIALOG")
        frame:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
        frame:SetBackdropColor(.08,.18,.22,.95);frame:SetBackdropBorderColor(.2,.75,.65,1)
        local title=frame:CreateFontString(nil,"OVERLAY","GameFontNormal")
        title:SetPoint("CENTER");title:SetText("Sample party frame")
    end
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
function F:Update()
    if not self.providerHook and DandersFrames and type(DandersFrames.RegisterCallback)=="function" then
        self.providerHook=pcall(DandersFrames.RegisterCallback,self,"OnFramesSorted",function()F:Update()end)
    end
    local combat=InCombatLockdown()
    if combat then self.preview=false end
    if not combat then self:HideSamples()end
    local enabled=self:ContextEnabled();local discovered=enabled and self:Discover() or {}
    local keep={};self.frameCount=#discovered;self.lastError=nil
    for _,target in ipairs(discovered)do
        local key=target.frame;local binding=self.bindings[key]
        if binding and binding.unit~=target.unit then
            binding.handle:SetShown(false)
            if not combat and binding.handle:SetUnit(target.unit) then binding.unit=target.unit end
        end
        local failure=self.failures[key]
        if not binding and not combat and LUI.NativeAuras and (not failure or failure.revision~=self.revision) then
            local handle,reason=LUI.NativeAuras:Create(UIParent,target.unit,self:Options(target.frame))
            if handle then
                binding={unit=target.unit,handle=handle,revision=self.revision};self.bindings[key]=binding;self.failures[key]=nil;self:AdoptStandaloneDisplay()
            else self.failures[key]={revision=self.revision,error=reason} end
        end
        if binding then
            keep[key]=true
            if not combat and binding.revision~=self.revision then
                binding.handle:Configure(self:Options(target.frame));binding.revision=self.revision
            end
            binding.handle:SetShown(not self.preview and binding.unit==target.unit)
            if binding.handle.GetError then self.lastError=binding.handle:GetError() or self.lastError end
        elseif self.failures[key] then self.lastError=self.failures[key].error end
    end
    for key,binding in pairs(self.bindings)do
        if not keep[key] then binding.handle:SetShown(false)end
    end
    if self.preview and not combat and enabled then
        if #discovered==0 then self:DrawSample(self:DemoFrame(),1)
        else for i,target in ipairs(discovered)do self:DrawSample(target.frame,i)end end
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
