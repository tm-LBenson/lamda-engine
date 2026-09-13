local _, LUI = ...
if LUI.NativeAuras then return end

-- Native aura rendering for WoW 12.1. Blizzard owns aura selection, secret
-- values, icon assignment, duration updates and visibility. This file supplies
-- only display settings and fresh regions inside its initializeFrame callback.
-- API reference: Blizzard_AuraContainer/{Blizzard_CustomAuraContainer.lua,
-- Blizzard_CustomAuraButton.lua,Blizzard_AuraButton.lua} in Blizzard's UI source.
-- No aura tables, spell identities, active-button counts or durations are read.
local Native = { handles = {}, pending = {}, supported = nil }
LUI.NativeAuras = Native
local Handle = {}
Handle.__index = Handle
local secret = issecretvalue or function() return false end
local function inCombat() return InCombatLockdown and InCombatLockdown() end
local function bounded(value, fallback, minimum, maximum)
    if secret(value) or type(value) ~= "number" or value ~= value then return fallback end
    return math.max(minimum, math.min(maximum, math.floor(value + 0.5)))
end
local points = { TOPLEFT=true, TOP=true, TOPRIGHT=true, LEFT=true, CENTER=true, RIGHT=true,
    BOTTOMLEFT=true, BOTTOM=true, BOTTOMRIGHT=true }
-- {vertical primary axis, grow left, grow up}; indexes are persisted settings.
local growthDirections={
    {false,false,false}, -- Right, then down.
    {false,true,false},  -- Left, then down.
    {false,false,true},  -- Right, then up.
    {false,true,true},   -- Left, then up.
    {true,false,false},  -- Down, then right.
    {true,false,true},   -- Up, then right.
    {true,true,false},   -- Down, then left.
    {true,true,true},    -- Up, then left.
}
local function anchor(value, fallback)
    if secret(value) or type(value) ~= "string" or not points[value] then return fallback end
    return value
end
local function flag(value, fallback)
    if secret(value) or type(value) ~= "boolean" then return fallback end
    return value
end
local function safeFrame(frame)
    if secret(frame) then return false end
    local kind = type(frame)
    if kind ~= "table" and kind ~= "userdata" then return false end
    if type(frame.IsForbidden) ~= "function" then return false end
    local ok, forbidden = pcall(frame.IsForbidden, frame)
    return ok and not secret(forbidden) and not forbidden
end
local function normalize(options, parent)
    options = type(options) == "table" and options or {}
    local result = {
        size=bounded(options.size,26,12,80), spacing=bounded(options.spacing,2,0,24),
        perRow=bounded(options.perRow,6,1,20), fontSize=bounded(options.fontSize,12,8,32),
        growth=bounded(options.growth,0,0,8),
        offsetX=bounded(options.offsetX,6,-2000,2000), offsetY=bounded(options.offsetY,0,-2000,2000),
        anchorPoint=anchor(options.anchorPoint,"LEFT"), relativePoint=anchor(options.relativePoint,"RIGHT"),
        relativeFrame=safeFrame(options.relativeFrame) and options.relativeFrame or parent,
        timers=flag(options.timers,true), stacks=flag(options.stacks,true),
        borders=flag(options.borders,true), glow=flag(options.glow,true),
        swipe=flag(options.swipe,true), reverse=flag(options.reverse,false),
        tooltips=flag(options.tooltips,true), groups={},
    }
    local groups = type(options.groups) == "table" and options.groups or {}
    for _, category in ipairs({"cc","debuffs","defensives","buffs"}) do
        local group = type(groups[category]) == "table" and groups[category] or {}
        result.groups[category] = {enabled=flag(group.enabled,true), max=bounded(group.max,category=="debuffs" and 4 or 3,0,20)}
    end
    return result
end
local placementKeys = {relativeFrame=true,anchorPoint=true,relativePoint=true,offsetX=true,offsetY=true}
local function sameOptions(a, b, ignorePlacement)
    if not a or not b then return false end
    for key, value in pairs(a) do
        if key == "groups" then
            for category, group in pairs(value) do
                local other=b.groups[category]
                if group.enabled~=other.enabled or group.max~=other.max then return false end
            end
        elseif not (ignorePlacement and placementKeys[key]) and value ~= b[key] then return false end
    end
    return true
end

function Native:IsSupported()
    if self.supported ~= nil then return self.supported end
    if inCombat() then return false, "combat" end
    local _,_,_,toc=GetBuildInfo()
    if type(toc)~="number" or toc<120100 then self.supported=false;return false,"version" end
    -- Loading a Blizzard module is safe only outside combat. Template/API probes
    -- are kept outside combat even when pcall could catch the Lua exception.
    if not AuraContainerSortMethod and C_AddOns and C_AddOns.LoadAddOn then
        pcall(C_AddOns.LoadAddOn,"Blizzard_AuraContainer")
    end
    if not AuraContainerSortMethod or not AnchorUtil or not AnchorUtil.FlowDirection then
        return false,"unavailable"
    end
    local ok, container = pcall(CreateFrame,"AuraContainer",nil,UIParent,"CustomAuraContainerTemplate")
    if not ok or not container then return false,"unavailable" end
    pcall(container.Hide,container)
    if type(container.AddAuraGroup)~="function" or type(container.SetUnit)~="function"
        or type(container.SetFlowLayoutMaximumLineSize)~="function" then
        self.supported=false;return false,"unavailable"
    end
    self.spare=container;self.supported=true
    return true
end

local function edge(button, point, relativePoint, x, y, width, height, color)
    local texture=button:CreateTexture(nil,"OVERLAY")
    texture:SetColorTexture(color[1],color[2],color[3],color[4] or 1)
    texture:SetPoint(point,button,relativePoint,x,y)
    texture:SetSize(width,height)
    return texture
end

local highlightColors={
    cc={1,0.3,0.15,0.8}, debuffs={0.75,0.35,1,0.8},
    defensives={0.2,0.65,1,0.8}, buffs={1,0.78,0.12,0.8},
}

local function initializeButton(button, options, category)
    -- This callback runs before access restrictions are applied. Never install
    -- addon scripts here, reparent an existing region, or keep aura button data.
    button:SetSize(options.size,options.size)
    if button.SetMouseClickEnabled then button:SetMouseClickEnabled(false) end
    if button.SetMouseMotionEnabled then button:SetMouseMotionEnabled(options.tooltips) end
    if button.SetTooltipAnchorPoint then button:SetTooltipAnchorPoint("ANCHOR_RIGHT",2,0) end
    if button.SetHideTooltipInCombat then button:SetHideTooltipInCombat(true) end

    local icon=button:CreateTexture(nil,"ARTWORK")
    icon:SetAllPoints(button)
    icon:SetTexCoord(0.07,0.93,0.07,0.93)
    button:SetIcon(icon)

    if options.swipe then
        -- A fresh Cooldown has no addon OnUpdate/OnCooldownDone script.
        local cooldown=CreateFrame("Cooldown",nil,button)
        cooldown:SetAllPoints(button)
        cooldown:SetDrawEdge(false)
        cooldown:SetDrawSwipe(true)
        cooldown:SetReverse(options.reverse)
        cooldown:SetHideCountdownNumbers(true)
        if cooldown.SetSwipeColor then cooldown:SetSwipeColor(0,0,0,0.62) end
        button:SetDurationCooldown(cooldown)
    end
    if options.timers then
        local duration=button:CreateFontString(nil,"OVERLAY")
        duration:SetFont(STANDARD_TEXT_FONT,math.min(options.fontSize,options.size-3),"OUTLINE")
        duration:SetPoint("CENTER",button,"CENTER",0,0)
        duration:SetTextColor(1,1,1,1)
        -- The native default formatter handles secret durations itself.
        button:SetDurationText(duration,{})
    end
    if options.stacks then
        local count=button:CreateFontString(nil,"OVERLAY")
        count:SetFont(STANDARD_TEXT_FONT,math.max(8,math.min(options.fontSize-1,options.size-3)),"OUTLINE")
        count:SetPoint("BOTTOMRIGHT",button,"BOTTOMRIGHT",1,-1)
        count:SetTextColor(1,1,1,1)
        button:SetApplicationCount(count,{})
    end
    if options.borders then
        local dark={0.04,0.05,0.07,1}
        edge(button,"TOPLEFT","TOPLEFT",-1,1,options.size+2,1,dark)
        edge(button,"BOTTOMLEFT","BOTTOMLEFT",-1,-1,options.size+2,1,dark)
        edge(button,"TOPLEFT","TOPLEFT",-1,1,1,options.size+2,dark)
        edge(button,"TOPRIGHT","TOPRIGHT",1,1,1,options.size+2,dark)
        if (category=="cc" or category=="debuffs") and type(button.AddDispelTypeTexture)=="function"
            and Enum and Enum.CustomAuraButtonDispelTypeTextureStyle then
            local border=button:CreateTexture(nil,"OVERLAY")
            border:SetPoint("TOPLEFT",button,"TOPLEFT",-1,1)
            border:SetPoint("BOTTOMRIGHT",button,"BOTTOMRIGHT",1,-1)
            button:AddDispelTypeTexture(border,{
                style=Enum.CustomAuraButtonDispelTypeTextureStyle.Border,
                showWhenHarmful=true,showWhenHelpful=false,showWithoutDispelType=true,
            })
        end
    end
    if options.glow then
        -- A fixed highlight inherits the native aura button's visibility. No
        -- secret-dependent animation, visibility query, or timer is required.
        local color=highlightColors[category]
        edge(button,"TOPLEFT","TOPLEFT",-2,2,options.size+4,2,color)
        edge(button,"BOTTOMLEFT","BOTTOMLEFT",-2,-2,options.size+4,2,color)
        edge(button,"TOPLEFT","TOPLEFT",-2,2,2,options.size+4,color)
        edge(button,"TOPRIGHT","TOPRIGHT",2,2,2,options.size+4,color)
    end
end

function Handle:GetFrame() return self.frame end
function Handle:GetError() return self.error end
function Handle:_Suspend()
    -- These are inbound engine setters, never protected-unit-frame mutations.
    -- Disabling also stops aura subscriptions and clears native assignments.
    if self.container then
        pcall(self.container.SetEnabled,self.container,false)
        pcall(self.container.Hide,self.container)
    end
    if self.frame then pcall(self.frame.Hide,self.frame) end
end
function Handle:_Place()
    local options=self.options
    if not safeFrame(options.relativeFrame) then error("Unavailable unit frame") end
    self.frame:ClearAllPoints()
    self.frame:SetPoint("CENTER",options.relativeFrame,options.relativePoint,options.offsetX,options.offsetY)
    self.container:ClearAllPoints()
    self.container:SetPoint(options.anchorPoint,self.frame,"CENTER",0,0)
    local direction=AnchorUtil.FlowDirection
    -- SetFlowLayoutGrowthDirection always takes physical horizontal, vertical
    -- directions. On the Vertical axis the vertical direction becomes primary
    -- and horizontal becomes the direction of the next column (AnchorUtil).
    local selected=growthDirections[options.growth]
    local vertical,fromRight,fromBottom
    if selected then
        vertical,fromRight,fromBottom=selected[1],selected[2],selected[3]
    else
        vertical=false
        fromRight=options.anchorPoint:find("RIGHT")~=nil
        fromBottom=options.anchorPoint:find("BOTTOM")~=nil
    end
    self.container:SetFlowLayoutAxis(vertical and AnchorUtil.FlowLayoutAxis.Vertical or AnchorUtil.FlowLayoutAxis.Horizontal)
    -- Flow uses a corner even when the whole group attaches by LEFT/RIGHT/CENTER.
    -- Starting flow at CENTER would put later rows outside its measured bounds.
    self.container:SetFlowLayoutAnchorPoint((fromBottom and "BOTTOM" or "TOP")..(fromRight and "RIGHT" or "LEFT"))
    self.container:SetFlowLayoutGrowthDirection(fromRight and direction.Left or direction.Right,
        fromBottom and direction.Up or direction.Down)
end
function Handle:_Build()
    self:_Suspend()
    local container=Native.spare
    Native.spare=nil
    if not container then container=CreateFrame("AuraContainer",nil,UIParent,"CustomAuraContainerTemplate") end
    self.container=container
    container:Hide()
    container:SetEnabled(false)
    container:SetUnit(self.unit)
    if container.SetMouseClickEnabled then container:SetMouseClickEnabled(false) end
    if container.SetMouseMotionEnabled then container:SetMouseMotionEnabled(false) end
    container:SetFrameStrata("MEDIUM")
    container:SetFrameLevel(40)
    container:SetFlowLayoutPadding(0,0,0,0)
    local options=self.options
    container:SetFlowLayoutMaximumLineSize(options.perRow*options.size+(options.perRow-1)*options.spacing)
    self:_Place()
    local order={
        {"cc","HARMFUL|CROWD_CONTROL","Default"},
        {"debuffs","HARMFUL|!CROWD_CONTROL","UnitFrameDebuff"},
        {"defensives","HELPFUL|BIG_DEFENSIVE","BigDefensive"},
        {"external","HELPFUL|EXTERNAL_DEFENSIVE|!BIG_DEFENSIVE","BigDefensive"},
        {"buffs","HELPFUL|IMPORTANT|!BIG_DEFENSIVE|!EXTERNAL_DEFENSIVE","ImportantOnly"},
    }
    for index, entry in ipairs(order) do
        local key,filter,sort=entry[1],entry[2],entry[3]
        local category=key=="external" and "defensives" or key
        local group=options.groups[category]
        if group.enabled and group.max>0 then
            if AuraUtil and AuraUtil.IsValidFilterString and not AuraUtil.IsValidFilterString(filter) then
                error("Unsupported native aura category")
            end
            container:AddAuraGroup(key,filter,{
                maxFrameCount=group.max,
                sortMethod=AuraContainerSortMethod[sort],
                sortDirection=AuraContainerSortDirection.Normal,
                layout={elementWidth=options.size,elementHeight=options.size,
                    elementSpacing=options.spacing,lineSpacing=options.spacing,
                    groupSpacing=0,layoutIndex=index},
                initializeFrame=function(button)initializeButton(button,options,category)end,
            })
        end
    end
    -- SetEnabled must be last: group topology and every initializer are ready.
    self.error=nil
    self:_ApplyShown()
end
function Handle:_ApplyShown()
    if self.destroyed then return end
    if self.shown and not self.error and not Native.pending[self] then
        self.frame:Show();self.container:Show();self.container:SetEnabled(true)
    else self:_Suspend() end
end
local function guarded(handle, action)
    local ok,reason=pcall(action)
    if not ok then
        if not secret(reason) and type(reason)=="string" then
            handle.error=reason:gsub("[%c]"," "):sub(1,400)
        else handle.error="Native aura display unavailable." end
        handle:_Suspend()
        return false,handle.error
    end
    return true
end
function Handle:Configure(options)
    if self.destroyed then return false,"destroyed" end
    local nextOptions=normalize(options,self.parent)
    if sameOptions(self.pendingOptions or self.options,nextOptions) and not self.error then return true end
    if inCombat() then
        self.pendingOptions=nextOptions;Native.pending[self]=true;self:_Suspend()
        return false,"combat"
    end
    local placementOnly=sameOptions(self.options,nextOptions,true)
    self.options=nextOptions
    return guarded(self,function()
        if placementOnly and self.container and not self.error then self:_Place();self:_ApplyShown()
        else self:_Build() end
    end)
end
function Handle:SetUnit(unit)
    if self.destroyed then return false,"destroyed" end
    if secret(unit) or type(unit)~="string" or not unit:match("^[%a]+%d*$") then
        self.error="Invalid unit binding.";self:_Suspend();return false,"unit"
    end
    if unit==self.unit and not self.pendingUnit and not self.error then return true end
    if inCombat() then
        self.pendingUnit=unit;Native.pending[self]=true;self:_Suspend();return false,"combat"
    end
    self.unit=unit;self.pendingUnit=nil
    return guarded(self,function()self.container:SetUnit(unit);self.error=nil;self:_ApplyShown()end)
end
function Handle:SetShown(shown)
    if self.destroyed then return false,"destroyed" end
    self.shown=shown==true
    if not self.shown then self:_Suspend();return true end
    if inCombat() and Native.pending[self] then return false,"combat" end
    return guarded(self,function()self:_ApplyShown()end)
end
function Handle:Show() return self:SetShown(true) end
function Handle:Hide() return self:SetShown(false) end
function Handle:Destroy()
    self.shown=false;self:_Suspend();self.destroyed=true
    Native.pending[self]=nil;Native.handles[self]=nil
end
function Handle:Refresh()
    if self.destroyed or self.error or Native.pending[self] then return false end
    return guarded(self,function()self.container:UpdateAllAuras()end)
end
function Native:Create(parent,unit,options)
    if inCombat() then return nil,"combat" end
    if not safeFrame(parent) then return nil,"parent" end
    if secret(unit) or type(unit)~="string" or not unit:match("^[%a]+%d*$") then return nil,"unit" end
    local supported,reason=self:IsSupported()
    if not supported then return nil,reason or "unavailable" end
    local handle=setmetatable({parent=parent,unit=unit,shown=true,options=normalize(options,parent)},Handle)
    handle.frame=CreateFrame("Frame",nil,UIParent)
    handle.frame:SetSize(1,1)
    handle.frame:EnableMouse(false)
    handle.frame:Hide()
    local ok,error=guarded(handle,function()handle:_Build()end)
    if not ok then handle:Destroy();return nil,error end
    self.handles[handle]=true
    return handle
end
function Native:FlushPending()
    if inCombat() then return end
    for handle in pairs(self.pending) do
        self.pending[handle]=nil
        if not handle.destroyed then
            local rebuild=handle.pendingOptions and not sameOptions(handle.options,handle.pendingOptions,true)
            if handle.pendingOptions then handle.options=handle.pendingOptions;handle.pendingOptions=nil end
            if handle.pendingUnit then handle.unit=handle.pendingUnit;handle.pendingUnit=nil end
            guarded(handle,function()
                if rebuild or handle.error then handle:_Build()
                else handle:_Place();handle.container:SetUnit(handle.unit);handle:_ApplyShown() end
            end)
        end
    end
end
local events=CreateFrame("Frame")
events:RegisterEvent("PLAYER_REGEN_ENABLED")
events:SetScript("OnEvent",function()Native:FlushPending()end)
