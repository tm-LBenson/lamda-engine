-- Player cooldowns stay in native UI widgets attached to the player's unit frame.
-- Duration objects and charge values are never unpacked for the external engine.
local _,LUI=...
local C={bindings={},layoutSamples={},spells={},aliases={},dirty=true,elapsed=0}
LUI.SelfCooldowns=C
local candidates=LUI.SpellLists.cooldowns
local function public(value,kind)
    if issecretvalue and issecretvalue(value) then return nil end
    if kind and type(value)~=kind then return nil end
    return value
end
local function call(fn,kind,...)
    if type(fn)~="function" then return nil end
    local ok,value=pcall(fn,...);if ok then return public(value,kind)end
end
local function isSelf(unit)
    return unit and (unit=="player" or call(UnitIsUnit,"boolean",unit,"player")==true)
end
local function frameUnit(frame)
    local F=LUI.FrameAuras
    return F and F.UnitForFrame and F:UnitForFrame(frame)
end
function C:RefreshSpells()
    if InCombatLockdown() then return end
    local found,seen,aliases={},{},{}
    for _,base in ipairs(LUI.SpellLists.Parse(LUI:DB().selfCDSpells))do
        if call(C_SpellBook and C_SpellBook.IsSpellKnown,"boolean",base)==true then
            local spell=call(C_Spell and C_Spell.GetOverrideSpell,"number",base) or base
            if spell<1 or spell>10000000 or spell~=math.floor(spell) then spell=base end
            aliases[spell]=aliases[spell] or {};aliases[spell][base]=true;aliases[spell][spell]=true
            if not seen[spell] then found[#found+1]=spell;seen[spell]=true end
        end
    end
    self.spells=found;self.aliases=aliases;self.dirty=false;self.selection=LUI:DB().selfCDSpells
end
function C:LayoutOffsetX(db)
    -- Old left-side rows filled from the far edge of the full-capacity box.
    -- Preserve the occupied row's near edge when switching its growth direction.
    if db.selfCDLayoutVersion<2 and not self.dirty and db.selfCDAnchor==3 and db.selfCDGrowth==1 and #self.spells>0 then
        local columns=math.min(db.selfCDPerRow,db.selfCDMax)
        local occupied=math.min(columns,#self.spells)
        return math.max(-200,math.min(200,db.selfCDX-(columns-occupied)*(db.selfCDSize+db.selfCDSpacing)))
    end
    return db.selfCDX
end
function C:UpgradeLayout(db,preview)
    if db.selfCDLayoutVersion>=2 or preview or InCombatLockdown() or self.dirty then return end
    db.selfCDX=self:LayoutOffsetX(db)
    db.selfCDLayoutVersion=2
end
function C:EngineFallback(spell,db)
    if not (db.engineTracking and db.engineFrameEstimates and db.lookupBridge and db.estimateSelf) then return false end
    if self.engineSelection~=db.engineFrameSpells then
        self.engineSelection=db.engineFrameSpells;self.engineSpells=LUI.SpellLists.Map(db.engineFrameSpells)
    end
    for id in pairs(self.aliases[spell] or {})do if self.engineSpells[id] then return true end end
    return false
end
-- Own the public IDs actually displayed by working native widgets. A queued
-- spellbook refresh, settings edit or delayed Lua tick does not remove those
-- widgets. Keep ownership until Render hides/replaces them or data fails.
-- Never inspect duration contents, timer visibility, charges, or ready state.
function C:NativeSpellsForFrame(target,db)
    local binding=self.bindings[target]
    if not (db.cdEnabled and db.nativeFrames) or not binding or binding.preview
        or (db.selfCDFrame~=4 and LUI.FrameAuras.cooldownFrame~=target)
        or not binding.root:IsShown() or not isSelf(frameUnit(target)) or not binding.renderedAt then return "" end
    local covered={}
    for _,icon in ipairs(binding.icons)do
        if icon:IsShown() and icon.nativeAvailable then
            for id in pairs(icon.nativeSpells or {})do covered[id]=true end
        end
    end
    local ids={};for id in pairs(covered)do ids[#ids+1]=id end;table.sort(ids)
    return table.concat(ids,",")
end
-- Public spell IDs and the layout already assigned to this native row. These
-- are presentation slots, not cooldown values. A failed native timer retains
-- its slot so an engine estimate can replace it in place.
function C:EngineSlotsForFrame(target,db)
    if not (db.selfCooldowns and db.nativeFrames) then return nil end
    local binding=self.bindings[target]
    local result={slots={}}
    if not binding or binding.preview or not binding.renderedAt or not binding.layout
        or not binding.root:IsShown() or not isSelf(frameUnit(target)) then return result end
    result.layout=binding.layout
    for index,icon in ipairs(binding.icons)do
        if icon.renderedSpells then
            local ids={};for id in pairs(icon.renderedSpells)do ids[#ids+1]=id end;table.sort(ids)
            if #ids>0 then result.slots[#result.slots+1]={index=index,ids=ids}end
        end
    end
    return result
end
function C:Configure(binding,db)
    db=db or LUI:DB();local size,gap,perRow=db.selfCDSize,db.selfCDSpacing,db.selfCDPerRow
    local count=db.selfCDMax
    local cols=math.min(perRow,count);local rows=math.ceil(count/perRow)
    local reverse=db.selfCDGrowth==3 or (db.selfCDGrowth==1 and db.selfCDAnchor==3)
    binding.root:SetSize(cols*size+(cols-1)*gap,rows*size+(rows-1)*gap)
    binding.root:SetAlpha(db.selfCDOpacity/100)
    local anchors={{"TOPLEFT","BOTTOMLEFT",0,-1},{"BOTTOMLEFT","TOPLEFT",0,1},{"TOPRIGHT","TOPLEFT",-1,0},{"TOPLEFT","TOPRIGHT",1,0}}
    local a=anchors[db.selfCDAnchor]
    binding.root:ClearAllPoints();binding.root:SetPoint(a[1],binding.target,a[2],a[3]*db.selfCDGap+self:LayoutOffsetX(db),a[4]*db.selfCDGap+db.selfCDY)
    binding.layout={size=size,spacing=gap,perRow=perRow,maximum=count,opacity=db.selfCDOpacity,
        anchor=db.selfCDAnchor,gap=db.selfCDGap,x=self:LayoutOffsetX(db),y=db.selfCDY,reverse=reverse}
    for i,icon in ipairs(binding.icons)do
        icon:SetSize(size,size);icon:ClearAllPoints()
        local column=(i-1)%perRow;if reverse then column=cols-1-column end
        icon:SetPoint("TOPLEFT",binding.root,"TOPLEFT",column*(size+gap),-math.floor((i-1)/perRow)*(size+gap))
        icon.cooldown:SetHideCountdownNumbers(not db.selfCDTimers)
        icon.count:SetFont(STANDARD_TEXT_FONT,math.max(9,math.floor(size*.4)),"OUTLINE")
    end
end
function C:Create(target)
    local root=CreateFrame("Frame",nil,UIParent);root:SetFrameStrata("MEDIUM")
    local binding={root=root,target=target,icons={}};root:Hide()
    binding.title=root:CreateFontString(nil,"OVERLAY","GameFontNormalSmall")
    binding.title:SetPoint("TOPLEFT",root,"BOTTOMLEFT",0,-2)
    for i=1,12 do
        local icon=CreateFrame("Frame",nil,root,"BackdropTemplate");binding.icons[i]=icon
        icon:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
        icon:SetBackdropColor(.05,.08,.12,1);icon:SetBackdropBorderColor(.15,.6,.8,1)
        icon.texture=icon:CreateTexture(nil,"ARTWORK");icon.texture:SetAllPoints();icon.texture:SetTexCoord(.07,.93,.07,.93)
        icon.cooldown=CreateFrame("Cooldown",nil,icon,"CooldownFrameTemplate");icon.cooldown:SetAllPoints()
        icon.cooldown:SetDrawEdge(false);icon.cooldown:SetSwipeColor(0,0,0,.7)
        icon.count=icon:CreateFontString(nil,"OVERLAY","GameFontNormalSmall");icon.count:SetPoint("BOTTOMRIGHT",-1,1)
        icon.unknown=icon:CreateFontString(nil,"OVERLAY","GameFontNormalLarge");icon.unknown:SetPoint("CENTER");icon.unknown:SetText("?")
        icon:SetMouseClickEnabled(false);icon:SetMouseMotionEnabled(true)
        icon:SetScript("OnEnter",function(self)
            if not self.spell then return end
            GameTooltip:SetOwner(self,"ANCHOR_RIGHT");GameTooltip:SetSpellByID(self.spell)
            GameTooltip:AddLine(binding.preview and "Cooldown preview" or "Your cooldown from WoW",.5,.8,1,true)
            if self.unavailable then GameTooltip:AddLine("Cooldown data unavailable.",1,.7,.3,true)end
            GameTooltip:Show()
        end)
        icon:SetScript("OnLeave",function()GameTooltip:Hide()end)
    end
    return binding
end
function C:Render(binding,db,selected)
    binding.renderedAt=nil
    local F=LUI.FrameAuras;local target=binding.target;local unit=frameUnit(target)
    db=db or LUI:DB()
    if not (db.cdEnabled and db.nativeFrames and db.selfCooldowns) or not F or not F:ContextEnabled(db) or not isSelf(unit) or not F:AllowsUnit(unit,db)
        or (db.selfCDFrame~=4 and target~=selected)
        or call(function()return target:IsForbidden()end,"boolean")~=false
        or call(function()return target:IsVisible()end,"boolean")~=true
        or call(UnitExists,"boolean",unit)~=true or call(UnitIsVisible,"boolean",unit)~=true then
        binding.root:Hide();return
    end
    local count=binding.preview and db.selfCDMax or math.min(db.selfCDMax,#self.spells)
    if db.selfCDSpells=="" then count=0 end
    binding.title:SetText(binding.preview and "My cooldowns - preview" or "")
    for i,icon in ipairs(binding.icons)do
        icon.nativeAvailable=false;icon.nativeSpells=nil;icon.renderedSpells=nil
        icon:SetShown(i<=count)
        if i<=count then
            local spell=self.spells[i] or self.spells[(i-1)%math.max(1,#self.spells)+1] or candidates[(i-1)%#candidates+1]
            icon.spell=spell;icon.texture:SetTexture(call(C_Spell and C_Spell.GetSpellTexture,nil,spell) or 134400)
            icon.count:SetText("");icon.unknown:Hide();icon.unavailable=false
            if binding.preview then
                icon.cooldown:SetCooldown(GetTime()-2,10);icon.count:SetText(db.selfCDCharges and i==2 and "2" or "")
            else
                icon.renderedSpells=self.aliases[spell]
                local ok=pcall(function()
                    local duration=call(C_Spell and C_Spell.GetSpellChargeDuration,nil,spell)
                    if not duration then duration=call(C_Spell and C_Spell.GetSpellCooldownDuration,nil,spell,true)end
                    if not duration then error("missing cooldown duration")end
                    -- The object stays opaque, even when it contains secret timings.
                    icon.cooldown:SetCooldownFromDurationObject(duration,true)
                end)
                icon.nativeAvailable=ok
                -- RefreshSpells replaces alias tables; retain the table belonging
                -- to this rendered spell until the widget itself is refreshed.
                if ok then icon.nativeSpells=self.aliases[spell]end
                if ok and db.selfCDCharges then
                    -- A missing charge label does not invalidate a working timer.
                    local chargesOK=pcall(function()
                        local info=call(C_Spell and C_Spell.GetSpellCharges,"table",spell)
                        if info then
                            -- FontString is a native presentation sink; do not compare,
                            -- format in Lua, serialize or infer from a secret charge count.
                            local value=info.currentCharges
                            if issecretvalue and issecretvalue(value) then icon.count:SetText(value)
                            elseif type(value)=="number" then icon.count:SetText(value)end
                        end
                    end)
                    if not chargesOK then icon.count:SetText("")end
                end
                if not ok then
                    icon.cooldown:Clear();icon.count:SetText("");icon.unknown:Show();icon.unavailable=true
                    self.lastError="Player cooldown data unavailable"
                    -- Leave this spell to the selected engine source. With the
                    -- engine path disabled, retain the native '?' placeholder.
                    if self:EngineFallback(spell,db) then icon:Hide()end
                end
            end
        end
    end
    binding.root:SetShown(count>0)
    binding.renderedAt=GetTime()
end
function C:Sync(targets,preview)
    local timing=LUI.Performance and LUI.Performance:BeginWork()
    local db=LUI:DB();local F=LUI.FrameAuras
    local selected=F:SelectOwnCooldownFrame(targets,db)
    if self.selection~=db.selfCDSpells then self.dirty=true end
    if self.dirty then self:RefreshSpells()end
    self:UpgradeLayout(db,preview)
    local keep={};self.lastError=nil
    for _,target in ipairs(targets)do
        if db.cdEnabled and db.nativeFrames and db.selfCooldowns and isSelf(frameUnit(target.frame))
            and (db.selfCDFrame==4 or target.frame==selected)
            and not (preview and not F.layoutPreview and F.PreviewEnabled and not F:PreviewEnabled("Cooldowns")) then
            local binding=self.bindings[target.frame]
            if not binding and not InCombatLockdown() then binding=self:Create(target.frame);self.bindings[target.frame]=binding end
            if binding then
                keep[target.frame]=true;binding.preview=preview and not InCombatLockdown()
                if not InCombatLockdown() then self:Configure(binding,db)end
                self:Render(binding,db,selected)
            end
        end
    end
    for target,binding in pairs(self.bindings)do if not keep[target] then binding.root:Hide()end end
    if timing then LUI.Performance:EndWork("Own cooldown refreshes",timing)end
end
function C:HideLayoutSamples()
    for _,binding in pairs(self.layoutSamples)do binding.root:Hide()end
end
-- Positioning samples stay entirely in WoW. They never become observations or
-- native spell ownership, and work even when the companion is not running.
function C:SyncLayoutSamples(targets,enabled)
    self:HideLayoutSamples()
    if not enabled or InCombatLockdown() then return end
    local db=LUI:DB();local F=LUI.FrameAuras
    if not F:ContextEnabled(db)
        or not (db.engineTracking and db.engineFrameEstimates and db.lookupBridge) or db.engineFrameSpells=="" then return end
    local spells=LUI.SpellLists.Parse(db.engineFrameSpells)
    local selected=F:SelectOwnCooldownFrame(targets,db)
    for _,target in ipairs(targets)do
        local unit=frameUnit(target.frame);local own=isSelf(unit)
        local member=unit and (unit:match("^party[1-4]$") or unit:match("^raid%d+$"))
        if (own and db.estimateSelf and not db.selfCooldowns and (db.selfCDFrame==4 or target.frame==selected))
            or (not own and member and db.estimateParty) then
            local binding=self.layoutSamples[target.frame]
            if not binding then binding=self:Create(target.frame);self.layoutSamples[target.frame]=binding end
            local matching=not own and db.engineFrameMatchSelf
            local prefix=matching and "selfCD" or "engineFrame"
            local options={selfCDLayoutVersion=2,selfCDTimers=true}
            for _,suffix in ipairs({"Size","Spacing","PerRow","Max","Opacity","Anchor","Gap","X","Y","Growth"})do
                options["selfCD"..suffix]=db[prefix..suffix]
            end
            -- Match the engine's legacy party layout until the native row has
            -- completed its own migration; previewing must not save that migration.
            if matching and db.selfCDLayoutVersion<2 then options.selfCDGrowth=2 end
            self:Configure(binding,options)
            binding.preview=true;binding.title:SetText("Cooldowns - preview ("..options.selfCDMax..")")
            for i,icon in ipairs(binding.icons)do
                icon:SetShown(i<=options.selfCDMax)
                if i<=options.selfCDMax then
                    local spell=spells[(i-1)%#spells+1]
                    icon.spell=spell;icon.nativeAvailable=false;icon.nativeSpells=nil;icon.renderedSpells=nil
                    icon.texture:SetTexture(call(C_Spell and C_Spell.GetSpellTexture,nil,spell) or 134400)
                    icon.count:SetText("");icon.unknown:Hide();icon.unavailable=false
                    icon.cooldown:SetCooldown(GetTime()-2,10)
                end
            end
            binding.root:Show()
        end
    end
end
function C:Tick()
    local timing=LUI.Performance and LUI.Performance:BeginWork()
    local db=LUI:DB();local F=LUI.FrameAuras
    if InCombatLockdown() or not F or not F.layoutPreview or not F:ContextEnabled(db) then self:HideLayoutSamples()end
    local selected=F and F:SelectOwnCooldownFrame(F.discovered,db)
    for _,binding in pairs(self.bindings)do
        if binding.root:IsShown() then
            if InCombatLockdown() then binding.preview=false end
            self:Render(binding,db,selected)
        end
    end
    if timing then LUI.Performance:EndWork("Own cooldown refreshes",timing)end
end
local events=CreateFrame("Frame")
for _,event in ipairs({"SPELLS_CHANGED","PLAYER_SPECIALIZATION_CHANGED","TRAIT_CONFIG_UPDATED","SPELL_UPDATE_COOLDOWN","SPELL_UPDATE_CHARGES","SPELL_UPDATE_USES","PLAYER_REGEN_ENABLED"})do events:RegisterEvent(event)end
events:SetScript("OnEvent",function(_,event)
    if event=="SPELLS_CHANGED" or event=="PLAYER_SPECIALIZATION_CHANGED" or event=="TRAIT_CONFIG_UPDATED" then
        C.dirty=true;if LUI.FrameAuras then LUI.FrameAuras:RequestRefresh()end
    elseif event=="PLAYER_REGEN_ENABLED" and C.dirty then
        if LUI.FrameAuras then LUI.FrameAuras:RequestRefresh()end
    end
    C:Tick()
end)
events:SetScript("OnUpdate",function(_,delta)
    C.elapsed=C.elapsed+delta;if C.elapsed<.2 then return end;C.elapsed=0;C:Tick()
end)
