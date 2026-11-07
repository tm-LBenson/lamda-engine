-- Native arena frames with the classic Gladius arrangement. No combat-log polling.
-- Blizzard owns cast/DR state; we only relocate those widgets and restore them.
local _, LUI = ...
local G = {rows={}, previews={}, borrowed={}, defaults={
    gladiusEnabled=false, gladiusLocked=true, gladiusWidth=190, gladiusHealthHeight=30,
    gladiusPowerHeight=9, gladiusSpacing=32, gladiusScale=1, gladiusOpacity=100,
    gladiusX=720, gladiusY=-240, gladiusGrow=1, gladiusFontSize=12,
    gladiusClassIcon=true, gladiusSpecIcon=false, gladiusPower=true, gladiusHealthText=true,
    gladiusSpecText=true, gladiusCast=true, gladiusCastHeight=16, gladiusTrinket=true,
    gladiusTrinketSize=39, gladiusDR=true, gladiusDRSize=26, gladiusDRSide=1,
    gladiusTargetHighlight=true, gladiusFocusHighlight=true,
    gladiusCC=true, gladiusBuffs=true, gladiusDebuffs=false, gladiusAuraSize=24,
    gladiusAuraMax=3, gladiusAuraOpacity=100, gladiusPets=true,
    gladiusEngine=true, gladiusInterrupts=true, gladiusDefensives=true, gladiusRacials=true,
    gladiusCooldownSize=24, gladiusCooldownMax=6, gladiusCooldownSide=1,
    gladiusLockouts=true, gladiusAnnouncements=false,
}}
LUI.Gladius = G
local texture = "Interface\\TargetingFrame\\UI-StatusBar"
local question = "Interface\\Icons\\INV_Misc_QuestionMark"
local backdrop = {bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1}
local function public(v) return not issecretvalue or not issecretvalue(v) end
local function read(fn,...)
    if type(fn)~="function" then return nil end
    local ok,v=pcall(fn,...); if ok and public(v) then return v end
end
local function arena() local _,kind=IsInInstance();return kind=="arena" end
local function combat() return InCombatLockdown() end
local function font(parent,size)
    local t=parent:CreateFontString(nil,"OVERLAY")
    t:SetFont(STANDARD_TEXT_FONT,size,"OUTLINE");t:SetShadowOffset(1,-1);t:SetWordWrap(false)
    return t
end
local function bar(parent)
    local b=CreateFrame("StatusBar",nil,parent)
    b:SetStatusBarTexture(texture);b:SetMinMaxValues(0,1);b:SetValue(1)
    b.bg=b:CreateTexture(nil,"BACKGROUND");b.bg:SetAllPoints();b.bg:SetColorTexture(.045,.045,.045,.95)
    return b
end
local function icon(parent)
    local f=CreateFrame("Frame",nil,parent,"BackdropTemplate")
    f:SetBackdrop(backdrop);f:SetBackdropColor(0,0,0,1);f:SetBackdropBorderColor(0,0,0,1)
    f.icon=f:CreateTexture(nil,"ARTWORK");f.icon:SetPoint("TOPLEFT",1,-1);f.icon:SetPoint("BOTTOMRIGHT",-1,1)
    f.icon:SetTexture(question);f.icon:SetTexCoord(.07,.93,.07,.93)
    return f
end
local function remember(frame)
    local points={};for i=1,frame:GetNumPoints() do points[i]={frame:GetPoint(i)} end
    return {frame=frame,parent=frame:GetParent(),points=points,width=frame:GetWidth(),height=frame:GetHeight(),
        scale=frame:GetScale(),alpha=frame:GetAlpha(),level=frame:GetFrameLevel(),strata=frame:GetFrameStrata()}
end
local function restore(saved)
    local f=saved.frame;f:SetParent(saved.parent);f:ClearAllPoints()
    f:SetSize(saved.width,saved.height);f:SetScale(saved.scale);f:SetAlpha(saved.alpha)
    f:SetFrameStrata(saved.strata);f:SetFrameLevel(saved.level)
    for _,p in ipairs(saved.points) do f:SetPoint(unpack(p)) end
end
function G:Conflict()
    local loaded=C_AddOns and C_AddOns.IsAddOnLoaded or IsAddOnLoaded
    for _,name in ipairs({"sArena_Reloaded","sArena","Gladius","Gladius_Updated_by_sammers","GladiusEx"}) do
        if read(loaded,name) then return name end
    end
end
function G:CreateRow(index,preview)
    local row=CreateFrame(preview and "Frame" or "Button",preview and nil or "LamdaGladiusArena"..index,UIParent,
        preview and "BackdropTemplate" or "SecureActionButtonTemplate,BackdropTemplate")
    row:Hide();row.index=index;row.unit="arena"..index;row.preview=preview
    row:SetFrameStrata("MEDIUM")
    row:SetBackdrop(backdrop);row:SetBackdropColor(0,0,0,.9);row:SetBackdropBorderColor(0,0,0,1)
    row.health=bar(row);row.power=bar(row);row.class=icon(row);row.trinket=icon(row)
    row.trinket.cooldown=CreateFrame("Cooldown",nil,row.trinket,"CooldownFrameTemplate")
    row.trinket.cooldown:SetAllPoints();row.trinket.cooldown:SetDrawEdge(true)
    row.trinket.cooldown:SetHideCountdownNumbers(false)
    row.trinket.unknown=font(row.trinket,12);row.trinket.unknown:SetPoint("BOTTOMRIGHT",-2,2)
    row.nameText=font(row.health,12);row.nameText:SetPoint("LEFT",4,0);row.nameText:SetJustifyH("LEFT")
    row.healthText=font(row.health,12);row.healthText:SetPoint("RIGHT",-4,0);row.healthText:SetJustifyH("RIGHT")
    row.specText=font(row.power,9);row.specText:SetPoint("CENTER")
    -- Holders permit hiding one borrowed component without changing its own
    -- native event handlers or guessing when its restricted timer expires.
    row.castHost=CreateFrame("Frame",nil,row);row.drHost=CreateFrame("Frame",nil,row)
    row.castHost:SetSize(1,1);row.drHost:SetSize(1,1)
    row.pet=CreateFrame(preview and "Frame" or "Button",nil,row,
        preview and "BackdropTemplate" or "SecureActionButtonTemplate,BackdropTemplate")
    row.pet.unit="arenapet"..index;row.pet.health=bar(row.pet);row.pet.health:SetAllPoints()
    row.pet.health:SetStatusBarColor(.3,.55,.35)
    row.pet.text=font(row.pet.health,10);row.pet.text:SetPoint("LEFT",3,0)
    if not preview then
        row.pet:RegisterForClicks("AnyUp");row.pet:SetAttribute("unit",row.pet.unit)
        row.pet:SetAttribute("type1","target");row.pet:SetAttribute("type2","focus")
    end
    if preview then
        row.sampleCC=icon(row.class);row.sampleCC:SetAllPoints();row.sampleCC.icon:SetTexture(136071)
        row.sampleCC.label=font(row.sampleCC,13);row.sampleCC.label:SetPoint("CENTER");row.sampleCC.label:SetText("4")
        row.sampleAuras={};row.sampleCooldowns={}
        for i=1,12 do row.sampleAuras[i]=icon(row)end
        for i=1,8 do
            local f=icon(row);row.sampleCooldowns[i]=f;f.label=font(f,12);f.label:SetPoint("CENTER")
        end
        row.sampleCast=bar(row.castHost);row.sampleCast:SetStatusBarColor(1,.72,.15)
        row.sampleCast.text=font(row.sampleCast,11);row.sampleCast.text:SetPoint("CENTER")
        row.sampleDR={}
        for i=1,3 do row.sampleDR[i]=icon(row.drHost);row.sampleDR[i].label=font(row.sampleDR[i],10);row.sampleDR[i].label:SetPoint("BOTTOMRIGHT",0,0) end
    else
        row:RegisterForClicks("AnyUp");row:SetAttribute("unit",row.unit)
        row:SetAttribute("type1","target");row:SetAttribute("type2","focus")
        row:SetScript("OnEnter",function(self)
            if not GameTooltip then return end
            GameTooltip:SetOwner(self,"ANCHOR_RIGHT");GameTooltip:SetText("Arena "..self.index)
            GameTooltip:AddLine("Left-click: target   Right-click: focus",1,1,1);GameTooltip:Show()
        end)
        row:SetScript("OnLeave",function()if GameTooltip then GameTooltip:Hide()end end)
    end
    return row
end
function G:EnsureFrames()
    if self.anchor then return end
    self.sink=CreateFrame("Frame",nil,UIParent);self.sink:Hide()
    local a=CreateFrame("Frame",nil,UIParent,"BackdropTemplate");self.anchor=a
    a:SetSize(260,24);a:SetFrameStrata("DIALOG");a:SetClampedToScreen(true);a:SetMovable(true);a:EnableMouse(true)
    a:SetBackdrop(backdrop);a:SetBackdropColor(.06,.16,.22,.95);a:SetBackdropBorderColor(.2,.8,.9,1)
    a.text=font(a,12);a.text:SetPoint("CENTER");a.text:SetText("LamdaGladius — move frames")
    a:RegisterForDrag("LeftButton")
    a:SetScript("OnDragStart",function(self)if not combat() then self:StartMoving()end end)
    a:SetScript("OnDragStop",function(self)
        self:StopMovingOrSizing();if combat() then return end
        local db=LUI:DB();local scale=db.gladiusScale
        db.gladiusX=self:GetLeft()*scale;db.gladiusY=self:GetTop()*scale-UIParent:GetHeight()-2*scale
        G:RefreshSettings(true)
    end)
    a:Hide()
    for i=1,5 do self.rows[i]=self:CreateRow(i,false);self.previews[i]=self:CreateRow(i,true) end
end
function G:LayoutRow(row,db)
    local h=db.gladiusHealthHeight+(db.gladiusPower and db.gladiusPowerHeight+1 or 0)
    local step=h+(db.gladiusCast and db.gladiusCastHeight+3 or 0)+(db.gladiusPets and 16 or 0)
        +(db.gladiusEngine and db.gladiusCooldownSide==1 and (db.gladiusCooldownSize+3+((db.gladiusLockouts or db.gladiusAnnouncements) and 18 or 0)) or 0)
        +math.max(db.gladiusSpacing,(db.gladiusBuffs and db.gladiusAuraSize+5 or 0)+(db.gladiusDebuffs and db.gladiusAuraSize+3 or 0))
    row:SetSize(db.gladiusWidth,h);row:SetScale(db.gladiusScale);row:SetAlpha(db.gladiusOpacity/100)
    row:ClearAllPoints();row:SetPoint("TOPLEFT",UIParent,"TOPLEFT",db.gladiusX/db.gladiusScale,
        db.gladiusY/db.gladiusScale+(row.index-1)*step*(db.gladiusGrow==2 and 1 or -1))
    row.health:ClearAllPoints();row.health:SetPoint("TOPLEFT",1,-1);row.health:SetSize(db.gladiusWidth-2,db.gladiusHealthHeight-1)
    row.power:ClearAllPoints();row.power:SetPoint("TOPLEFT",row.health,"BOTTOMLEFT",0,-1);row.power:SetSize(db.gladiusWidth-2,db.gladiusPowerHeight-1)
    row.power:SetShown(db.gladiusPower)
    row.nameText:SetFont(STANDARD_TEXT_FONT,db.gladiusFontSize,"OUTLINE");row.nameText:SetWidth(db.gladiusWidth-(db.gladiusHealthText and 65 or 10))
    row.healthText:SetFont(STANDARD_TEXT_FONT,db.gladiusFontSize,"OUTLINE");row.healthText:SetShown(db.gladiusHealthText)
    row.specText:SetFont(STANDARD_TEXT_FONT,math.max(8,math.min(db.gladiusFontSize,db.gladiusPowerHeight)),"OUTLINE")
    row.specText:SetWidth(db.gladiusWidth-8);row.specText:SetShown(db.gladiusSpecText)
    row.class:ClearAllPoints();row.class:SetPoint("RIGHT",row,"LEFT",-2,0);row.class:SetSize(h,h);row.class:SetShown(db.gladiusClassIcon)
    row.trinket:ClearAllPoints();row.trinket:SetPoint("LEFT",row,"RIGHT",3,0)
    row.trinket:SetSize(db.gladiusTrinketSize,db.gladiusTrinketSize);row.trinket:SetShown(db.gladiusTrinket)
    row.castHost:ClearAllPoints();row.castHost:SetPoint("TOPLEFT",row,"BOTTOMLEFT",0,-3);row.castHost:SetSize(db.gladiusWidth,db.gladiusCastHeight);row.castHost:SetShown(db.gladiusCast)
    row.pet:ClearAllPoints();row.pet:SetPoint("TOPLEFT",row,"BOTTOMLEFT",0,-(db.gladiusCast and db.gladiusCastHeight+6 or 3))
    row.pet:SetSize(db.gladiusWidth*.65,13)
    if row.preview then row.pet:SetShown(db.gladiusPets);row.pet.text:SetText("Pet");row.pet.health:SetValue(.7) end
    row.drHost:ClearAllPoints();row.drHost:SetSize(3*(db.gladiusDRSize+2),db.gladiusDRSize)
    if db.gladiusDRSide==1 then row.drHost:SetPoint("RIGHT",row,"LEFT",-(db.gladiusClassIcon and h+5 or 3),0)
    else row.drHost:SetPoint("LEFT",row,"RIGHT",db.gladiusTrinket and db.gladiusTrinketSize+7 or 3,0)end
    row.drHost:SetShown(db.gladiusDR)
    if row.cast then
        row.cast:ClearAllPoints();row.cast:SetPoint("CENTER",row.castHost,"CENTER");row.cast:SetSize(db.gladiusWidth,db.gladiusCastHeight)
        row.cast:SetScale(1);row.cast:SetAlpha(1)
    end
    if row.dr then
        row.dr:ClearAllPoints();row.dr:SetScale(db.gladiusDRSize/26);row.dr:SetAlpha(1)
        row.dr:SetPoint(db.gladiusDRSide==1 and "RIGHT" or "LEFT",row.drHost,db.gladiusDRSide==1 and "RIGHT" or "LEFT",0,0)
    end
    if row.preview then
        row.sampleCC:SetShown(db.gladiusCC and row.index==2)
        for i,f in ipairs(row.sampleAuras)do
            local debuff=i>6;local n=(i-1)%6
            f:ClearAllPoints();f:SetSize(db.gladiusAuraSize,db.gladiusAuraSize);f:SetAlpha(db.gladiusAuraOpacity/100)
            f:SetPoint("BOTTOMLEFT",row,"TOPLEFT",n*(db.gladiusAuraSize+2),debuff and db.gladiusAuraSize+5 or 3)
            f.icon:SetTexture(debuff and 136207 or ({135936,135988,136120,135940,132351,136206})[n+1])
            f:SetShown(n<math.min(db.gladiusAuraMax,debuff and 2 or 3) and (debuff and db.gladiusDebuffs or not debuff and db.gladiusBuffs))
        end
        local examples={}
        if db.gladiusInterrupts then examples[#examples+1]={135856,"12"}end
        if db.gladiusDefensives then examples[#examples+1]={135841,"1:20"}end
        if db.gladiusRacials then examples[#examples+1]={136225,"40"}end
        for i,f in ipairs(row.sampleCooldowns)do
            f:ClearAllPoints();f:SetSize(db.gladiusCooldownSize,db.gladiusCooldownSize)
            if db.gladiusCooldownSide==1 then f:SetPoint("TOPLEFT",row,"BOTTOMLEFT",(i-1)*(db.gladiusCooldownSize+2),
                -((db.gladiusCast and db.gladiusCastHeight+3 or 0)+(db.gladiusPets and 16 or 0)+3))
            else f:SetPoint("TOPLEFT",row,"TOPRIGHT",(db.gladiusTrinket and db.gladiusTrinketSize+7 or 3)
                +(db.gladiusDR and db.gladiusDRSide==2 and 3*(db.gladiusDRSize+2)+4 or 0)+(i-1)*(db.gladiusCooldownSize+2),0)end
            f:SetShown(db.gladiusEngine and i<=#examples and i<=db.gladiusCooldownMax)
            if examples[i] then f.icon:SetTexture(examples[i][1]);f.label:SetText(examples[i][2])end
        end
        row.sampleCast:SetAllPoints(row.castHost)
        for i,f in ipairs(row.sampleDR)do f:ClearAllPoints();f:SetSize(db.gladiusDRSize,db.gladiusDRSize);f:SetPoint("LEFT",row.drHost,"LEFT",(i-1)*(db.gladiusDRSize+2),0)end
    end
end
function G:UpdatePet(row)
    if not read(UnitExists,row.pet.unit) then return end
    row.pet.health:SetMinMaxValues(0,UnitHealthMax(row.pet.unit));row.pet.health:SetValue(UnitHealth(row.pet.unit))
    row.pet.text:SetText(UnitName(row.pet.unit))
end
function G:HideAuras()
    for _,row in ipairs(self.rows)do for _,handle in pairs(row.auras or {})do handle:Hide()end end
end
function G:ConfigureAuras(row,shown)
    if not LUI.NativeAuras then return end
    local db=self.settings;row.auras=row.auras or {}
    for _,kind in ipairs({"cc","buffs","debuffs"})do
        local enabled=kind=="cc" and db.gladiusCC and db.gladiusClassIcon
            or kind=="buffs" and db.gladiusBuffs or kind=="debuffs" and db.gladiusDebuffs
        local groups={cc={enabled=false,max=0},buffs={enabled=false,max=0},defensives={enabled=false,max=0},debuffs={enabled=false,max=0}}
        groups[kind]={enabled=true,max=kind=="cc" and 1 or db.gladiusAuraMax}
        if kind=="buffs" then groups.defensives={enabled=true,max=db.gladiusAuraMax} end
        local cc=kind=="cc"
        local options={groups=groups,size=cc and row.class:GetHeight() or db.gladiusAuraSize,
            scalePercent=db.gladiusScale*100,opacity=cc and db.gladiusOpacity or db.gladiusAuraOpacity*db.gladiusOpacity/100,
            spacing=2,perRow=kind=="buffs" and db.gladiusAuraMax*3 or db.gladiusAuraMax,
            relativeFrame=cc and row.class or row,anchorPoint=cc and "CENTER" or "BOTTOMLEFT",
            relativePoint=cc and "CENTER" or "TOPLEFT",offsetX=0,
            offsetY=cc and 0 or (kind=="debuffs" and db.gladiusAuraSize+5 or 3),
            growth=3,timers=true,stacks=not cc,tooltips=true,debuffFilter=3,hideSated=true}
        if enabled then
            if row.auras[kind] then row.auras[kind]:Configure(options)
            else
                local handle,err=LUI.NativeAuras:Create(row,row.unit,options);row.auras[kind]=handle
                if not handle then self.auraError=public(err) and type(err)=="string" and err or "unavailable" end
            end
        end
        if row.auras[kind] then row.auras[kind]:SetShown(shown and enabled==true) end
    end
end
function G:SetClass(row,class,specIcon,db)
    if not public(class) then return end
    local c=class and RAID_CLASS_COLORS and RAID_CLASS_COLORS[class]
    row.health:SetStatusBarColor(c and c.r or .5,c and c.g or .5,c and c.b or .5)
    if db.gladiusSpecIcon and public(specIcon) and specIcon then
        row.class.icon:SetTexture(specIcon);row.class.icon:SetTexCoord(.07,.93,.07,.93)
    elseif class and CLASS_ICON_TCOORDS and CLASS_ICON_TCOORDS[class] then
        row.class.icon:SetTexture("Interface\\Glues\\CharacterCreate\\UI-CharacterCreate-Classes")
        row.class.icon:SetTexCoord(unpack(CLASS_ICON_TCOORDS[class]))
    else row.class.icon:SetTexture(question);row.class.icon:SetTexCoord(0,1,0,1) end
end
function G:UpdateIdentity(row)
    local db=self.settings;if not db then return end
    local spec=read(GetArenaOpponentSpec,row.index)
    if type(spec)=="number" and spec>0 and GetSpecializationInfoByID then
        local _,name,_,image,_,class=GetSpecializationInfoByID(spec)
        if public(class) then row.classToken=class end
        if public(name) then row.specName=name end
        if public(image) then row.specIcon=image end
    end
    if not row.classToken and UnitClass then
        local _,class=UnitClass(row.unit);if public(class) then row.classToken=class end
    end
    self:SetClass(row,row.classToken,row.specIcon,db);row.specText:SetText(row.specName or "")
    if read(UnitExists,row.unit) then local name=UnitName(row.unit);row.nameText:SetText(name)
    else row.nameText:SetText(row.specName or ("Arena "..row.index)) end
end
function G:UpdateVitals(row)
    -- Restricted values go straight into Blizzard widgets, never Lua arithmetic.
    if not read(UnitExists,row.unit) then return end
    row.health:SetMinMaxValues(0,UnitHealthMax(row.unit));row.health:SetValue(UnitHealth(row.unit))
    if type(UnitHealthPercent)=="function" and CurveConstants and CurveConstants.ScaleTo100 then
        row.healthText:SetFormattedText("%.0f%%",UnitHealthPercent(row.unit,true,CurveConstants.ScaleTo100))
    else row.healthText:SetText(UnitHealth(row.unit)) end
    row.power:SetMinMaxValues(0,UnitPowerMax(row.unit));row.power:SetValue(UnitPower(row.unit))
    local power=read(UnitPowerType,row.unit);local c=power and PowerBarColor and PowerBarColor[power]
    row.power:SetStatusBarColor(c and c.r or .1,c and c.g or .4,c and c.b or 1)
end
function G:UpdateTrinket(row,spellID)
    if not self.settings.gladiusTrinket then return end
    if public(spellID) and spellID==nil and C_PvP and C_PvP.GetArenaCrowdControlInfo then
        spellID=C_PvP.GetArenaCrowdControlInfo(row.unit)
    end
    if not public(spellID) or (type(spellID)=="number" and spellID>0) then
        -- GetSpellTexture accepts a secret spell identifier; SetTexture accepts
        -- its result. Do not compare/cache that identifier or use it as a key.
        local image=C_Spell.GetSpellTexture(spellID);row.trinket.icon:SetTexture(image)
        row.trinket.unknown:SetText("")
    end
    if C_PvP and C_PvP.GetArenaCrowdControlDuration and row.trinket.cooldown.SetCooldownFromDurationObject then
        local duration=C_PvP.GetArenaCrowdControlDuration(row.unit)
        if duration then row.trinket.cooldown:SetCooldownFromDurationObject(duration) end
    end
end
function G:UpdateHighlights()
    local db=self.settings;if not db then return end
    for _,row in ipairs(self.rows)do
        local target=db.gladiusTargetHighlight and read(UnitIsUnit,row.unit,"target")
        local focus=db.gladiusFocusHighlight and read(UnitIsUnit,row.unit,"focus")
        if target then row:SetBackdropBorderColor(1,.8,0,1)
        elseif focus then row:SetBackdropBorderColor(.2,.8,1,1)
        else row:SetBackdropBorderColor(0,0,0,1)end
    end
end
function G:DRSetting(enabled)
    if not C_CVar or not C_CVar.GetCVar or not C_CVar.SetCVar then return end
    local key="spellDiminishPVPEnemiesEnabled"
    -- Persist the original across /reload so disabling the module restores the
    -- user's setting, not the value we enabled in the previous session.
    LamdaUIDB=LamdaUIDB or {}
    if enabled then
        if LamdaUIDB.gladiusOriginalDR==nil then LamdaUIDB.gladiusOriginalDR=C_CVar.GetCVar(key)end
        if C_CVar.GetCVar(key)~="1" then C_CVar.SetCVar(key,"1")end
    elseif LamdaUIDB.gladiusOriginalDR~=nil then
        C_CVar.SetCVar(key,LamdaUIDB.gladiusOriginalDR);LamdaUIDB.gladiusOriginalDR=nil
    end
end
function G:BorrowNative()
    if not CompactArenaFrame_Generate then return end
    local native=CompactArenaFrame_Generate()
    if not native then return end
    if not self.native then
        self.native=remember(native)
        self.hooked=self.hooked or {}
        if hooksecurefunc and not self.hooked[native] then
            self.hooked[native]=true
            hooksecurefunc(native,"UpdateLayout",function()G:QueueLayout()end)
        end
    end
    for i,row in ipairs(self.rows) do
        local member=native.memberUnitFrames and native.memberUnitFrames[i]
        if member then
            for field,key in pairs({CastingBarFrame="cast",SpellDiminishStatusTray="dr"})do
                local widget=member[field]
                if widget and not row[key] then
                    self.borrowed[#self.borrowed+1]=remember(widget);row[key]=widget
                    widget:SetParent(key=="cast" and row.castHost or row.drHost)
                end
            end
        end
    end
    -- Borrow first; hiding the original container must not hide cast/DR widgets.
    native:SetParent(self.sink)
end
function G:RestoreNative()
    for _,saved in ipairs(self.borrowed)do restore(saved)end
    self.borrowed={}
    for _,row in ipairs(self.rows)do row.cast=nil;row.dr=nil end
    local saved=self.native;self.native=nil
    if saved then restore(saved);if saved.frame.UpdateLayout then saved.frame:UpdateLayout()end end
    self:DRSetting(false)
end
function G:QueueLayout()
    if not self.active or self.queued then return end
    if combat() then self.pending=true;return end
    self.queued=true
    C_Timer.After(0,function()G.queued=nil;if G.active then G:RefreshSettings(true)end end)
end
local samples={
    {"Warrior","WARRIOR","Arms",132355,.78,.65},
    {"Priest","PRIEST","Discipline",135940,.62,.83},
    {"Mage","MAGE","Frost",135846,.91,.52},
    {"Rogue","ROGUE","Subtlety",132320,.7,.5},
    {"Druid","DRUID","Restoration",136041,.9,.8},
}
function G:DrawPreview()
    local count=self.previewCount or 0;local db=self.settings
    for i,row in ipairs(self.previews)do
        row:SetShown(i<=count)
        if i<=count then
            local s=samples[i];self:LayoutRow(row,db);self:SetClass(row,s[2],s[4],db)
            row.nameText:SetText(s[1]);row.specText:SetText(s[3]);row.health:SetMinMaxValues(0,1);row.health:SetValue(s[5])
            row.healthText:SetFormattedText("%.0f%%",s[5]*100);row.power:SetValue(s[6]);row.power:SetStatusBarColor(.1,.45,1)
            row.trinket.icon:SetTexture("Interface\\Icons\\INV_Jewelry_TrinketPVP_01");row.trinket.unknown:SetText("")
            row.trinket.cooldown:SetCooldown(GetTime(),i==2 and 90 or 0)
            row.sampleCast:SetMinMaxValues(0,1);row.sampleCast:SetValue(.6);row.sampleCast.text:SetText("Preview cast")
            for j,f in ipairs(row.sampleDR)do
                f.icon:SetTexture(({132298,136071,136183})[j]);f.label:SetText(({"12","8","IMM"})[j])
            end
        end
    end
end
function G:SetPreview(count)
    if combat() then return false,"Arena previews are available after combat." end
    if count and count>0 and arena() then return false,"Leave the arena to show sample opponents." end
    if count and count~=0 and count~=2 and count~=3 and count~=5 then return false,"Choose 2, 3 or 5 opponents." end
    if not LUI:DB().gladiusEnabled then return false,"Enable LamdaGladius first." end
    if self:Conflict() then return false,"Disable the other arena addon and reload before previewing." end
    self.previewCount=count and count>0 and count or nil
    self:RefreshSettings(true);return true
end
function G:ResetMatch()
    if self.tracking then self.tracking:Reset() end
    for _,row in ipairs(self.rows)do
        row.classToken=nil;row.specName=nil;row.specIcon=nil;row.requestedTrinket=nil
        row.health:SetMinMaxValues(0,1);row.health:SetValue(1);row.healthText:SetText("")
        row.power:SetMinMaxValues(0,1);row.power:SetValue(1)
        row.trinket.icon:SetTexture(question);row.trinket.unknown:SetText("?");row.trinket.cooldown:Clear()
    end
end
function G:RefreshSettings(force)
    local db=LUI:DB();if db.gladiusEnabled==nil then return end
    local changed=force or not self.settings
    if self.settings then for key in pairs(self.defaults)do if self.settings[key]~=db[key]then changed=true;break end end end
    if not changed then return end
    if combat() then self.pending=true;return end
    self.pending=nil;self.settings={};for key in pairs(self.defaults)do self.settings[key]=db[key]end
    self.auraError=nil
    self.conflict=self:Conflict();self.active=db.gladiusEnabled and not self.conflict
    if not self.active then
        self:HideAuras()
        self.previewCount=nil
        if self.anchor then
            self.anchor:Hide();for _,r in ipairs(self.rows)do UnregisterStateDriver(r,"visibility");r.visibility=nil;r:Hide()end
            for _,r in ipairs(self.previews)do r:Hide()end
        end
        self:RestoreNative();self.failed=nil;self.restoreFailed=nil;return
    end
    self:EnsureFrames();self:BorrowNative()
    self:DRSetting(db.gladiusDR and self.native~=nil and C_SpellDiminish and read(C_SpellDiminish.IsSystemSupported)==true)
    local inArena=arena()
    if inArena then self.previewCount=nil end
    local size=read(GetNumArenaOpponentSpecs) or 0
    if type(size)~="number" then size=0 end
    for i,row in ipairs(self.rows)do
        self:LayoutRow(row,db)
        local condition=inArena and self.native and (i<=size and "show" or ("[@"..row.unit..",exists] show; hide")) or "hide"
        if row.visibility~=condition then RegisterStateDriver(row,"visibility",condition);row.visibility=condition end
        local petCondition=inArena and self.native and db.gladiusPets and ("[@"..row.pet.unit..",exists] show; hide") or "hide"
        if row.pet.visibility~=petCondition then RegisterStateDriver(row.pet,"visibility",petCondition);row.pet.visibility=petCondition end
        self:ConfigureAuras(row,inArena and self.native~=nil and i<=size)
        self:UpdateIdentity(row)
        if inArena and self.native then
            self:UpdateVitals(row)
            self:UpdatePet(row)
            if not row.requestedTrinket and C_PvP and C_PvP.RequestCrowdControlSpell then
                C_PvP.RequestCrowdControlSpell(row.unit);row.requestedTrinket=true
            end
            self:UpdateTrinket(row)
        end
    end
    self:DrawPreview();self:UpdateHighlights()
    self.anchor:SetScale(db.gladiusScale);self.anchor:ClearAllPoints()
    self.anchor:SetPoint("BOTTOMLEFT",UIParent,"TOPLEFT",db.gladiusX/db.gladiusScale,db.gladiusY/db.gladiusScale+2)
    self.anchor:SetShown(not db.gladiusLocked and (inArena or self.previewCount~=nil))
    self.failed=nil;self.restoreFailed=nil
end
function G:Status()
    if self.failed then return "LamdaGladius paused: "..self.failed..(self.pending and " Native frames restore after combat." or self.restoreFailed and " Restore needs a UI reload." or " WoW's arena frames were restored.")end
    if self.pending then return "Changes saved. Arena layout will apply when combat ends." end
    if self.conflict then return "LamdaGladius is paused while "..self.conflict.." is loaded. Disable it and reload." end
    if not self.active then return "LamdaGladius is off. WoW's arena frames are restored." end
    if not self.native then return "Preview is available. This client does not provide the Retail arena widgets; WoW's live arena frames stay enabled." end
    if self.previewCount then return "Preview: "..self.previewCount.." sample opponents. Drag the unlocked header to move them." end
    if self.auraError then return "Arena frames active. Native aura display unavailable: "..self.auraError end
    local screen=self.tracking and self.tracking.screen
    if arena() and self.settings.gladiusEngine then
        if not LUI:DB().lookupBridge then return "Native tracking active. Enable the event strip for engine cooldowns." end
        if screen and screen.failed then return "Native tracking active. Cast capture unavailable: "..screen.error end
        if screen and screen.waitingForFont then return "Native tracking active. Preparing cast text for Lamda Engine." end
    end
    return "Ready for arenas. Left-click an opponent to target; right-click to focus."
end
function G:Open()
    LUI:OpenUI();LUI:SelectPage("modules");LUI:SelectModule("gladius")
end
local events=CreateFrame("Frame");G.events=events
for _,event in ipairs({"PLAYER_LOGIN","PLAYER_ENTERING_WORLD","PLAYER_REGEN_ENABLED","PLAYER_REGEN_DISABLED",
    "ARENA_PREP_OPPONENT_SPECIALIZATIONS","ARENA_OPPONENT_UPDATE","ARENA_COOLDOWNS_UPDATE","ARENA_CROWD_CONTROL_SPELL_UPDATE",
    "PLAYER_TARGET_CHANGED","PLAYER_FOCUS_CHANGED","PVP_MATCH_STATE_CHANGED","ADDON_LOADED"})do pcall(events.RegisterEvent,events,event)end
for _,event in ipairs({"UNIT_HEALTH","UNIT_MAXHEALTH","UNIT_POWER_UPDATE","UNIT_MAXPOWER","UNIT_DISPLAYPOWER","UNIT_NAME_UPDATE"})do
    events:RegisterUnitEvent(event,"arena1","arena2","arena3","arena4","arena5","arenapet1","arenapet2","arenapet3","arenapet4","arenapet5")
end
local function onEvent(_,event,unit,spellID)
    if event=="PLAYER_LOGIN" then
        SLASH_LAMDAGLADIUS1="/lamdagladius";SlashCmdList.LAMDAGLADIUS=function()G:Open()end
        G:RefreshSettings(true);return
    elseif event=="PLAYER_REGEN_DISABLED" then
        G.previewCount=nil
        for _,r in ipairs(G.previews)do r:Hide()end
        if G.anchor then G.anchor:StopMovingOrSizing();G.anchor:Hide()end
        return
    elseif event=="PLAYER_REGEN_ENABLED" then G:RefreshSettings(true);return
    elseif event=="PLAYER_ENTERING_WORLD" or event=="ARENA_PREP_OPPONENT_SPECIALIZATIONS" then
        G:ResetMatch();G:RefreshSettings(true);return
    elseif event=="ADDON_LOADED" then
        if unit=="Blizzard_UnitFrame" then G:QueueLayout()end;return
    end
    if not G.active or not G.native or not arena() then return end
    if event=="PLAYER_TARGET_CHANGED" or event=="PLAYER_FOCUS_CHANGED" then G:UpdateHighlights();return end
    if event=="ARENA_OPPONENT_UPDATE" or event=="PVP_MATCH_STATE_CHANGED" then G:QueueLayout()end
    for _,row in ipairs(G.rows)do
        if public(unit) and unit==row.pet.unit then G:UpdatePet(row)end
        if event=="ARENA_COOLDOWNS_UPDATE" then G:UpdateTrinket(row)
        elseif public(unit) and unit==row.unit then
            if event=="ARENA_CROWD_CONTROL_SPELL_UPDATE" then G:UpdateTrinket(row,spellID)
            else
                if event=="UNIT_NAME_UPDATE" or event=="ARENA_OPPONENT_UPDATE" then G:UpdateIdentity(row)end
                G:UpdateVitals(row)
            end
        end
    end
end
function G:Fail(reason)
    -- A capability/runtime failure in this module must not break hub startup
    -- or the player's party frames, bindings, planner, and other modules.
    self.failed=public(reason) and type(reason)=="string" and reason:sub(1,220) or "Arena data is unavailable."
    self.active=false;self.previewCount=nil
    self:HideAuras()
    if self.lastError~=self.failed then
        self.lastError=self.failed;print("LamdaGladius: "..self.failed.." Open /lamda gladius for status.")
    end
    if combat() then self.pending=true;return end
    if self.anchor then self.anchor:Hide()end
    for _,r in ipairs(self.previews)do r:Hide()end
    for _,r in ipairs(self.rows)do UnregisterStateDriver(r,"visibility");r.visibility=nil;r:Hide()end
    local restored=pcall(self.RestoreNative,self)
    self.restoreFailed=not restored
end
local refresh=G.RefreshSettings
function G:RefreshSettings(force)
    local ok,err=pcall(refresh,self,force)
    if not ok then self:Fail(err)end
end
events:SetScript("OnEvent",function(...)
    local ok,err=pcall(onEvent,...);if not ok then G:Fail(err)end
end)
