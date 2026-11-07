-- LamdaUI's addon-button drawer. Original implementation; LibDBIcon owns the
-- buttons, artwork, tooltips and click handlers. No HidingBar runtime is copied.
local _, LUI = ...
local D = {owned={}, hooked={}, ready=false}
LUI.Drawer = D
local backdrop={bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1}
local function safe(button)
    return button and not button:IsForbidden() and not button:IsProtected()
end
local function oldBarLoaded()
    local check=C_AddOns and C_AddOns.IsAddOnLoaded or IsAddOnLoaded
    return check and check("HidingBar")
end
function D:RequestScan(delay)
    if self.failure then return end
    local at=GetTime()+(delay or 0)
    self.scanAt=math.min(self.scanAt or at,at)
end
function D:Dirty()
    if not self.adjusting then self.dirty=true end
end
function D:Status(text)
    self.status=text
    if self.statusLabel then self.statusLabel:SetText(text) end
end
function D:Library()
    local lib=LibStub and LibStub("LibDBIcon-1.0",true)
    if not lib or not lib.GetButtonList or not lib.GetMinimapButton then return end
    if self.lib~=lib then
        self.lib=lib
        lib.RegisterCallback(self,"LibDBIcon_IconCreated",function()D:RequestScan()end)
        -- The library may enable minimap fading after a button was collected.
        -- Preserve that preference for release, but the drawer controls fading.
        if lib.ShowOnEnter then hooksecurefunc(lib,"ShowOnEnter",function(_,name,value)
            local record=D.owned[lib:GetMinimapButton(name)]
            if record then record.mouseover=not not value;D:Dirty() end
        end) end
    end
    return lib
end
function D:Excluded(name)
    for entry in LUI:DB().drawerExclude:gmatch("[^,\r\n]+") do
        entry=entry:match("^%s*(.-)%s*$"):gsub("^LibDBIcon10_","")
        if entry==name then return true end
    end
    return name=="HidingBar"
end
function D:Capture(button,name)
    if self.owned[button] or not safe(button) or InCombatLockdown() then return end
    local width,height=button:GetSize()
    if not width or not height or width<=0 or height<=0 then return end
    local points={}
    for i=1,button:GetNumPoints() do points[i]={button:GetPoint(i)} end
    self.owned[button]={name=name,parent=button:GetParent(),points=points,
        width=width,height=height,scale=button:GetScale(),alpha=button:GetAlpha(),
        strata=button:GetFrameStrata(),level=button:GetFrameLevel(),
        dragStart=button:GetScript("OnDragStart"),dragStop=button:GetScript("OnDragStop"),
        mouseover=button.showOnMouseover}
    if not self.hooked[button] then
        self.hooked[button]=true
        local function changed()if D.owned[button] then D:Dirty() end end
        for _,method in ipairs({"SetPoint","ClearAllPoints","SetParent","SetScale","SetSize"}) do
            hooksecurefunc(button,method,changed)
        end
        button:HookScript("OnShow",changed);button:HookScript("OnHide",changed)
        hooksecurefunc(button,"SetScript",function(_,kind,fn)
            local record=D.owned[button]
            if not record or D.adjusting then return end
            local key=kind=="OnDragStart" and "dragStart" or kind=="OnDragStop" and "dragStop"
            if key then
                record[key]=fn
                if not InCombatLockdown() and safe(button) then
                    D.adjusting=true;button:SetScript(kind,nil);D.adjusting=false
                end
                D:Dirty()
            end
        end)
    end
end
function D:Release(button)
    local record=self.owned[button]
    if not record or InCombatLockdown() or not safe(button) then return false end
    self.owned[button]=nil
    self.adjusting=true
    button:SetParent(record.parent);button:ClearAllPoints()
    for _,point in ipairs(record.points) do button:SetPoint(unpack(point)) end
    button:SetSize(record.width,record.height);button:SetScale(record.scale)
    button:SetFrameStrata(record.strata);button:SetFrameLevel(record.level)
    button:SetAlpha(record.alpha);button.showOnMouseover=record.mouseover
    button:SetScript("OnDragStart",record.dragStart);button:SetScript("OnDragStop",record.dragStop)
    -- Honor the owner's CURRENT hide/position/lock settings, not a login snapshot.
    if self.lib and self.lib:GetMinimapButton(record.name)==button then
        self.lib:Refresh(record.name)
        if self.lib.ShowOnEnter then self.lib:ShowOnEnter(record.name,record.mouseover) end
    end
    self.adjusting=false
    return true
end
function D:Create()
    if self.root then return end
    local root=CreateFrame("Frame","LamdaUIAddonDrawer",UIParent)
    self.root=root;root:Hide();root:SetFrameStrata("MEDIUM")
    local handle=CreateFrame("Button",nil,root,"BackdropTemplate")
    self.handle=handle;handle:SetSize(76,16);handle:SetPoint("TOP",root,"TOP")
    handle:SetBackdrop(backdrop);handle:SetBackdropColor(.035,.075,.10,1)
    handle:SetBackdropBorderColor(.20,.65,.75,1)
    local label=handle:CreateFontString(nil,"OVERLAY","GameFontHighlightSmall")
    label:SetPoint("CENTER");label:SetText("Addons")
    handle:RegisterForClicks("LeftButtonUp","RightButtonUp")
    handle:RegisterForDrag("LeftButton")
    handle:SetScript("OnEnter",function()D:Open()end)
    handle:SetScript("OnClick",function(_,button)
        if D.dragged then D.dragged=false;return end
        if button=="RightButton" then
            LUI:OpenUI();LUI:SelectPage("modules");LUI:SelectModule("drawer")
        else D.pinned=not D.pinned;D:Open() end
    end)
    handle:SetScript("OnDragStart",function()
        if IsShiftKeyDown() and not InCombatLockdown() then D.dragging=true;D.dragged=true;D:Open() end
    end)
    handle:SetScript("OnDragStop",function()D.dragging=false end)
    local panel=CreateFrame("Frame",nil,root,"BackdropTemplate")
    self.panel=panel;panel:Hide();panel:EnableMouse(true)
    panel:SetBackdrop(backdrop);panel:SetBackdropColor(.025,.045,.065,.96)
    panel:SetBackdropBorderColor(.18,.40,.46,1)
    panel:SetScript("OnEnter",function()D:Open()end)
    self.empty=panel:CreateFontString(nil,"OVERLAY","GameFontHighlightSmall")
    self.empty:SetPoint("CENTER");self.empty:SetText("No addon icons")
end
function D:Position()
    local db=LUI:DB();local root=self.root
    local scale=root:GetEffectiveScale()
    local screen=UIParent:GetWidth()*scale
    local half=math.min(screen/2,math.max(38,(self.panel:GetWidth() or 0)/2)*scale)
    local x=db.drawerPosition<0 and screen/2 or db.drawerPosition
    x=math.max(half,math.min(screen-half,x))
    root:ClearAllPoints();self.panel:ClearAllPoints()
    if db.drawerEdge==1 then
        root:SetPoint("TOP",UIParent,"TOPLEFT",x/scale,0)
        self.panel:SetPoint("TOP",root,"BOTTOM",0,0)
    else
        root:SetPoint("BOTTOM",UIParent,"BOTTOMLEFT",x/scale,0)
        self.panel:SetPoint("BOTTOM",root,"TOP",0,0)
    end
end
function D:Layout()
    if InCombatLockdown() then self.dirty=true;return end
    self.dirty=false
    if not self.active then return end
    local db=LUI:DB();local visible={}
    for button,record in pairs(self.owned) do
        if safe(button) and button:IsShown() then visible[#visible+1]={button=button,record=record} end
    end
    table.sort(visible,function(a,b)return a.record.name:lower()<b.record.name:lower()end)
    local size=db.drawerSize
    local columns=math.min(db.drawerColumns,math.max(1,math.floor((UIParent:GetWidth()-12)/size)))
    local width=math.max(120,math.min(columns,#visible)*size+12)
    local height=math.max(1,math.ceil(#visible/columns))*size+12
    self.root:SetSize(76,16);self.panel:SetSize(width,height)
    self.adjusting=true
    -- Hidden icons remain children too; an owner's later Show() schedules layout.
    for button in pairs(self.owned) do if safe(button) then
        button:SetParent(self.panel)
        button.showOnMouseover=false
        if button.fadeOut then button.fadeOut:Stop() end
        button:SetAlpha(1)
        button:SetScript("OnDragStart",nil);button:SetScript("OnDragStop",nil)
    end end
    for i,entry in ipairs(visible) do
        local button,record=entry.button,entry.record
        local factor=size/math.max(record.width,record.height)
        button:SetScale(factor);button:ClearAllPoints()
        button:SetPoint("CENTER",self.panel,"TOPLEFT",
            (6+((i-1)%columns+.5)*size)/factor,(-6-(math.floor((i-1)/columns)+.5)*size)/factor)
        button:SetFrameStrata("MEDIUM");button:SetFrameLevel(self.panel:GetFrameLevel()+1)
    end
    self.adjusting=false
    self.empty:SetShown(#visible==0);self:Position()
    self:Status(string.format("%d addon icons. Hover to open; click to pin. Shift-drag the handle to move it.",#visible))
    if db.drawerAlwaysOpen then self:Open() end
end
function D:Scan()
    if not self.ready or InCombatLockdown() then return end
    self.scanAt=nil
    local db=LUI:DB();local conflict=oldBarLoaded()
    self.active=db.drawerEnabled and not conflict
    if not self.active then
        for button in pairs(self.owned) do self:Release(button) end
        if self.root then self.root:Hide() end
        self.pinned=false
        self:Status(conflict and "HidingBar is still loaded. Disable it and restart WoW to use LamdaDrawer."
            or "Disabled. Your addon buttons are back on the minimap.")
        return
    end
    self:Create()
    local lib=self:Library();local wanted={}
    if lib then for _,name in ipairs(lib:GetButtonList()) do
        local button=lib:GetMinimapButton(name)
        if not self:Excluded(name) and safe(button) then
            wanted[button]=true;self:Capture(button,name)
        end
    end end
    for button in pairs(self.owned) do if not wanted[button] then self:Release(button) end end
    self:Layout()
    self.root:SetShown(not self.petBattle)
    if not lib then self:Status("Waiting for addons to register minimap icons.") end
end
function D:Open()
    if not self.active or not self.panel or self.petBattle then return end
    self.panel:Show();self.handle:SetAlpha(1);self.leaveAt=nil
end
function D:Tick()
    if not self.ready then return end
    local now=GetTime()
    if not InCombatLockdown() then
        if self.scanAt and now>=self.scanAt then self:Scan()
        elseif self.dirty then self:Layout() end
    end
    if not self.active or not self.root or not self.root:IsShown() then return end
    local db=LUI:DB()
    if self.dragging and not InCombatLockdown() then
        local x=GetCursorPosition();db.drawerPosition=math.max(0,x);self:Position()
    end
    local hovered=self.handle:IsMouseOver() or (self.panel:IsShown() and self.panel:IsMouseOver())
    if hovered or self.dragging or self.pinned or db.drawerAlwaysOpen then self:Open()
    elseif self.panel:IsShown() then
        self.leaveAt=self.leaveAt or now
        if now-self.leaveAt>=db.drawerHideDelay then self.panel:Hide();self.handle:SetAlpha(db.drawerFade/100) end
    else self.handle:SetAlpha(db.drawerFade/100) end
end
function D:RestoreAfterError()
    if InCombatLockdown() then self.restorePending=true;return end
    self.restorePending=false;self.adjusting=false
    for button in pairs(self.owned) do pcall(self.Release,self,button) end
    if self.root then self.root:Hide() end
end
function D:PauseAfterError(message)
    if self.failure then return end
    self.failure=tostring(message);self.active=false;self.scanAt=nil
    self.events:SetScript("OnUpdate",nil)
    self:RestoreAfterError()
    self:Status("Drawer paused after an error. Buttons return to the minimap after combat. Reload after updating LamdaUI.")
    print("LamdaUI: LamdaDrawer paused after an error; details are in WoW's log.")
    -- Report once through the normal handler, keeping the actual file/line.
    -- Never leave a failed timer running twenty times per second.
    if geterrorhandler then pcall(geterrorhandler(),self.failure) end
end
local module={id="drawer",name="LamdaDrawer",enabledKey="drawerEnabled",settingsScope="global",
    enabledHelp="Collect addon minimap buttons into a LamdaUI drawer. Disabling returns them to the minimap. Changes wait until combat ends.",
    -- Position is HidingBar's saved top-edge center, in screen pixels. Other
    -- defaults retain Lewis's current 31px / 10 per row / 0.75s / 20% preferences.
    defaults={drawerEnabled=false,drawerEdge=1,drawerPosition=682.6666870117188,drawerSize=31,
        drawerColumns=10,drawerHideDelay=.75,drawerFade=20,drawerAlwaysOpen=false,drawerExclude=""},
    limits={drawerEdge={1,2,integer=true},drawerPosition={-1,32768},drawerSize={20,48,integer=true},
        drawerColumns={1,20,integer=true},drawerHideDelay={0,3},drawerFade={5,100,integer=true}}}
module.build=function(parent)
    LUI:Choice(parent,"Screen edge","drawerEdge",{"Top","Bottom"},0)
    LUI:Checkbox(parent,"Keep drawer open","drawerAlwaysOpen",-40,0,"Keep addon buttons visible instead of closing after the mouse leaves. A left click on the handle can also pin it open for this session.")
    for _,setting in ipairs({{"Icon size","drawerSize",-90,20,48,1,0},
        {"Icons per row","drawerColumns",-90,1,20,1,300},
        {"Hide delay (seconds)","drawerHideDelay",-158,0,3,.05,0},
        {"Idle handle opacity (%)","drawerFade",-158,5,100,1,300}}) do
        local slider=LUI:Slider(parent,unpack(setting))
        slider:HookScript("OnValueChanged",function()D:RequestScan()end)
    end
    LUI:Label(parent,"Leave these icons on the minimap (names separated by commas)",4,-227)
    local edit=CreateFrame("EditBox",nil,parent,"InputBoxTemplate")
    edit:SetSize(570,26);edit:SetPoint("TOPLEFT",8,-254);edit:SetAutoFocus(false);edit:SetMaxLetters(2000)
    edit:SetScript("OnEnterPressed",function(self)
        LUI:DB().drawerExclude=self:GetText();self:ClearFocus();D:RequestScan()
    end)
    edit:SetScript("OnEscapePressed",function(self)self:SetText(LUI:DB().drawerExclude);self:ClearFocus()end)
    LUI:OnRefresh(edit,function()
        if not edit:HasFocus() then edit:SetText(LUI:DB().drawerExclude) end
        D:RequestScan()
    end)
    LUI:Button(parent,"Center on screen",4,-296,160,function()LUI:DB().drawerPosition=-1;D:RequestScan()end)
    LUI:Button(parent,"Show drawer",180,-296,140,function()D:Open()end)
    D.statusLabel=LUI:Description(parent,D.status or "Waiting for login...",4,-341,585)
    LUI:Description(parent,"Right-click the handle for these settings. Addon clicks, right-clicks and tooltips work as before. An icon hidden in its own addon stays hidden. No companion required.",4,-389,585)
end
LUI:RegisterModule(module)
local events=CreateFrame("Frame");D.events=events
for _,event in ipairs({"PLAYER_LOGIN","PLAYER_ENTERING_WORLD","ADDON_LOADED","PLAYER_REGEN_ENABLED",
    "PLAYER_REGEN_DISABLED","UI_SCALE_CHANGED","DISPLAY_SIZE_CHANGED","PET_BATTLE_OPENING_START","PET_BATTLE_CLOSE"}) do
    pcall(events.RegisterEvent,events,event)
end
events:SetScript("OnEvent",function(_,event)
    if D.failure then
        if event=="PLAYER_REGEN_ENABLED" and D.restorePending then D:RestoreAfterError() end
        return
    end
    if event=="PLAYER_LOGIN" then D.ready=true end
    if event=="PLAYER_REGEN_DISABLED" then D.dragging=false end
    if event=="PET_BATTLE_OPENING_START" then D.petBattle=true;if D.root then D.root:Hide() end
    elseif event=="PET_BATTLE_CLOSE" then D.petBattle=false end
    D:RequestScan(event=="PLAYER_LOGIN" and 1 or .1)
end)
local elapsed=0
events:SetScript("OnUpdate",function(_,delta)
    elapsed=elapsed+delta;if elapsed<.05 then return end;elapsed=0
    local ok,message=pcall(D.Tick,D)
    if not ok then D:PauseAfterError(message) end
end)
