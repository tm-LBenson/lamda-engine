local _,LUI=...
function LUI:RefreshCooldownPreview()
    self:UpdateReloadRequired()
    if self.RefreshApplyStatus then self:RefreshApplyStatus()end
    local auras=self.FrameAuras
    if auras and auras.RequestRefresh then auras:RequestRefresh() end
end
function LUI:StopFramePreview()
    local auras=self.FrameAuras
    if not auras or not auras.preview then return end
    -- SetPreview refreshes the hub; closing it must not recursively reopen or refresh it.
    auras.preview=false
    if auras.HideSamples then auras:HideSamples() end
    if auras.RequestRefresh then auras:RequestRefresh() end
end

LUI.refreshers={}
function LUI:RefreshUI()
    -- Nested tab builders request refreshes before the remaining modules exist.
    if self.buildingUI then return end
    local timing=self.Performance and self.Performance:BeginWork()
    if self.LookupEvents then self.LookupEvents:RefreshSettings()end
    if self.Kick then self.Kick:RefreshSettings()end
    if self.Performance then self.Performance:RefreshSettings()end
    if self.Gladius then self.Gladius:RefreshSettings()end
    for _,refresh in ipairs(self.refreshers) do refresh() end
    self:RefreshCooldownPreview()
    if timing then self.Performance:EndWork("Settings refresh",timing)end
end
function LUI:OnRefresh(frame,callback)
    table.insert(self.refreshers,callback)
    -- FontStrings participate in refreshes but cannot own frame scripts.
    if type(frame.HookScript)=="function"then frame:HookScript("OnShow",callback)end
end
function LUI:Label(parent,text,x,y,large)
    local label=parent:CreateFontString(nil,"OVERLAY",large and "GameFontNormalLarge" or "GameFontHighlight")
    label:SetPoint("TOPLEFT",x,y);label:SetText(text);return label
end
local background={bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1}
function LUI:Button(parent,text,x,y,width,callback)
    local button=CreateFrame("Button",nil,parent,"BackdropTemplate")
    button:SetSize(width,26);button:SetPoint("TOPLEFT",x,y);button:SetBackdrop(background)
    button:SetBackdropColor(.10,.16,.22,1);button:SetBackdropBorderColor(.28,.38,.47,1)
    local label=button:CreateFontString(nil,"OVERLAY","GameFontHighlight");label:SetPoint("CENTER")
    label:SetWidth(width-12);label:SetMaxLines(1);label:SetWordWrap(false)
    button.label=label
    local nativeEnabled=button.SetEnabled
    function button:SetEnabled(value)
        nativeEnabled(self,not not value)
        label:SetTextColor(value and .9 or .45,value and .95 or .5,value and 1 or .55)
    end
    function button:SetText(value)self.text=value;label:SetText(value)end
    button:SetText(text)
    button:SetHighlightTexture("Interface\\QuestFrame\\UI-QuestTitleHighlight","ADD")
    button:SetScript("OnClick",callback);return button
end
function LUI:Navigation(parent,text,x,y,width,callback)
    local tab=self:Button(parent,text,x,y,width,callback);tab.navigation=true
    local line=tab:CreateTexture(nil,"OVERLAY");line:SetColorTexture(.2,.8,.9,1)
    line:SetPoint("BOTTOMLEFT",0,0);line:SetPoint("BOTTOMRIGHT",0,0);line:SetHeight(3)
    function tab:SetSelected(selected)
        self.selected=selected;line:SetShown(selected)
        self:SetBackdropColor(selected and .11 or .045,selected and .23 or .08,selected and .30 or .12,1)
        self:SetBackdropBorderColor(selected and .2 or .10,selected and .65 or .15,selected and .75 or .20,1)
        self.label:SetTextColor(selected and 1 or .72,selected and 1 or .78,selected and 1 or .82)
    end
    tab:SetSelected(false);return tab
end
function LUI:Explain(frame,title,body)
    frame.helpTitle=title;frame.helpText=body
    frame.explain=function(self)
        if not GameTooltip then return end
        GameTooltip:SetOwner(self,"ANCHOR_RIGHT");GameTooltip:SetText(title)
        GameTooltip:AddLine(body,.85,.9,.95,true);GameTooltip:Show()
    end
    frame:HookScript("OnEnter",frame.explain)
    frame:HookScript("OnLeave",function(self)if not self.pinned and GameTooltip then GameTooltip:Hide()end end)
    frame:HookScript("OnHide",function(self)
        if LUI.pinnedHelp==self then LUI.pinnedHelp=nil;self.pinned=false;if GameTooltip then GameTooltip:Hide()end end
    end)
    return frame
end
function LUI:Help(parent,title,body,x,y)
    local info=self:Button(parent,"?",x,y,22,function(self)
        if self.pinned then self.pinned=false;LUI.pinnedHelp=nil;if GameTooltip then GameTooltip:Hide()end
        else
            if LUI.pinnedHelp then LUI.pinnedHelp.pinned=false end
            self.pinned=true;LUI.pinnedHelp=self;self.explain(self)
        end
    end)
    self:Explain(info,title,body);return info
end
function LUI:Description(parent,text,x,y,width)
    local label=self:Label(parent,text,x,y);label:SetWidth(width or 600);label:SetJustifyH("LEFT");label:SetSpacing(3)
    return label
end
function LUI:ResetButton(parent,title,keys,x,y)
    return self:Button(parent,"Restore defaults",x or 4,y or -354,150,function()
        if not LUI.resetDialog then
            local dialog=CreateFrame("Frame",nil,LUI.frame or UIParent,"BackdropTemplate")
            LUI.resetDialog=dialog;dialog:SetSize(440,155);dialog:SetPoint("CENTER");dialog:SetFrameStrata("FULLSCREEN_DIALOG")
            dialog:SetBackdrop(background);dialog:SetBackdropColor(.05,.08,.12,1);dialog:SetBackdropBorderColor(.2,.65,.75,1)
            dialog.prompt=LUI:Description(dialog,"",18,-18,404)
            LUI:Button(dialog,"Restore",18,-108,140,function()local action=dialog.action;dialog:Hide();action()end)
            LUI:Button(dialog,"Cancel",280,-108,140,function()dialog:Hide()end)
        end
        local dialog=LUI.resetDialog
        dialog.prompt:SetText("Restore "..title.." to defaults? Other profiles are kept.")
        dialog.action=function()
            if type(keys)=="function" then keys()else LUI:ResetKeys(keys)end
        end
        dialog:Show()
    end)
end
local function settingKey(key)
    return type(key)=="function" and key() or key
end
function LUI:Checkbox(parent,text,key,y,x,help)
    local button=CreateFrame("CheckButton",nil,parent,"UICheckButtonTemplate")
    button:SetSize(26,26);button:SetPoint("TOPLEFT",x or 0,y)
    local label=button:CreateFontString(nil,"OVERLAY","GameFontHighlight")
    label:SetPoint("LEFT",button,"RIGHT",4,0);label:SetWidth(232);label:SetHeight(30)
    label:SetJustifyH("LEFT");label:SetWordWrap(true);label:SetMaxLines(2);label:SetText(text);button.label=label
    self:OnRefresh(button,function()button:SetChecked(not not LUI:DB()[settingKey(key)])end)
    button:SetScript("OnClick",function(self)LUI:DB()[settingKey(key)]=not not self:GetChecked();LUI:RefreshUI()end)
    button:SetHitRectInsets(0,-210,0,0)
    local current=settingKey(key)
    local explanation=help or (LUI.settingHelp and LUI.settingHelp[current]) or ("Show "..text:lower().." for this display. Turning this off keeps the rest of the group settings.")
    self:Explain(button,text,explanation);self:Help(parent,text,explanation,(x or 0)+270,y)
    return button
end
function LUI:Choice(parent,text,key,choices,y,x)
    x=x or 0;local label=self:Label(parent,text,x+4,y)
    label:SetWidth(112);label:SetHeight(26);label:SetJustifyH("LEFT");label:SetWordWrap(true);label:SetMaxLines(2)
    return self:Dropdown(parent,x+125,y,135,function()return choices end,
        function()return choices[LUI:DB()[settingKey(key)]] or choices[1]end,function(value)
            for index,label in ipairs(choices)do if label==value then LUI:DB()[settingKey(key)]=index;break end end
        end)
end
function LUI:Slider(parent,text,key,y,min,max,step,x)
    x=x or 0
    local label=self:Label(parent,text,x+4,y)
    label:SetWidth(210);label:SetJustifyH("LEFT");label:SetWordWrap(false);label:SetMaxLines(1)
    local value=parent:CreateFontString(nil,"OVERLAY","GameFontNormal")
    value:SetPoint("TOPRIGHT",parent,"TOPLEFT",x+260,y)
    local slider=CreateFrame("Slider",nil,parent,"OptionsSliderTemplate")
    slider:SetPoint("TOPLEFT",x+8,y-24);slider:SetSize(248,16);slider:SetOrientation("HORIZONTAL")
    slider:SetMinMaxValues(min,max);slider:SetValueStep(step);slider:SetObeyStepOnDrag(true)
    if slider.Low then slider.Low:SetText("")end;if slider.High then slider.High:SetText("")end
    local refreshing=false
    slider:SetScript("OnValueChanged",function(_,v)
        if refreshing then return end
        v=math.max(min,math.min(max,math.floor(v/step+0.5)*step));LUI:DB()[settingKey(key)]=v
        value:SetText(step<1 and string.format("%.2f",v) or tostring(v));LUI:RefreshCooldownPreview()
    end)
    self:OnRefresh(slider,function()
        local current=settingKey(key)
        refreshing=true;slider:SetValue(LUI:DB()[current]);refreshing=false
        value:SetText(step<1 and string.format("%.2f",LUI:DB()[current]) or tostring(LUI:DB()[current]))
    end)
    self:Explain(slider,text,"Adjust "..text:lower().." for the selected group and frame layout. The preview updates as you change this value.")
    return slider
end
-- Every transmitting module owns its own origin in the shared strip manager.
function LUI:ColorStripControls(parent,id,x,y)
    x=x or 4;y=y or 0
    local function strip(create)
        local bridge=LUI.LookupEvents
        if not bridge then return end
        if create then return bridge:GetStrip(id)end
        if id=="cd" then return bridge.frame and bridge end
        return bridge.strips[id]
    end
    local move=self:Button(parent,"Move",x,y,96,function()
        local target=strip(true);if target then target:SetMoving(not target.moving);LUI:RefreshUI()end
    end)
    local reset=self:Button(parent,"Reset",x+104,y,96,function()
        local target=strip(true);if target then target:ResetPosition();LUI:RefreshUI()end
    end)
    self:OnRefresh(move,function()
        local target=strip();local unlocked=target and target.moving
        move:SetText(unlocked and "Lock" or "Move")
        move:SetEnabled(LUI.LookupEvents~=nil and not LUI:CombatManagementBlocked())
        reset:SetEnabled(LUI.LookupEvents~=nil and not LUI:CombatManagementBlocked())

    end)
    parent:HookScript("OnHide",function()local target=strip();if target then target:SetMoving(false)end end)
    return move,reset
end

-- A compact native menu. Entries are rebuilt when opened so added profiles appear immediately.
function LUI:Dropdown(parent,x,y,width,items,selected,onSelect)
    local button=self:Button(parent,"",x,y,width,function()end)
    button.selectControl=true
    button.label:ClearAllPoints();button.label:SetPoint("LEFT",10,0);button.label:SetJustifyH("LEFT");button.label:SetWidth(width-42)
    button:SetBackdropColor(.025,.045,.065,1)
    local arrow=button:CreateTexture(nil,"OVERLAY");arrow:SetSize(22,22);arrow:SetPoint("RIGHT",-2,0)
    arrow:SetTexture("Interface\\Buttons\\UI-ScrollBar-ScrollDownButton-Up")
    local menu=CreateFrame("Frame",nil,button,"BackdropTemplate")
    menu:SetPoint("TOPLEFT",button,"BOTTOMLEFT",0,-2);menu:SetSize(width,190);menu:SetFrameStrata("TOOLTIP")
    menu:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
    menu:SetBackdropColor(0.04,0.07,0.11,1);menu:SetBackdropBorderColor(0.18,0.26,0.32,1)
    local offset=0;local entries={}
    local more=self:Label(menu,"",8,-190)
    local function draw()
        local values=items();offset=math.max(0,math.min(offset,math.max(0,#values-6)))
        menu:SetHeight(math.min(6,#values)*30+8+(#values>6 and 22 or 0))
        more:SetShown(#values>6);more:SetText("Scroll for more ("..(offset+1).."-"..math.min(offset+6,#values).." / "..#values..")")
        for i=1,6 do
            if not entries[i] then entries[i]=LUI:Button(menu,"",4,-4-(i-1)*30,width-8,function()end) end
            local entry=entries[i];local value=values[offset+i];entry:SetShown(value~=nil)
            if value then entry:SetBackdrop(nil);entry.label:SetTextColor(value==selected() and .3 or .9,.9,.95);entry:SetText(value);entry:SetScript("OnClick",function()menu:Hide();onSelect(value);LUI:RefreshUI()end) end
        end
    end
    menu:EnableMouse(true);menu:EnableMouseWheel(true)
    menu:SetScript("OnMouseWheel",function(_,delta)offset=offset-delta;draw()end)
    menu:SetScript("OnHide",function()if LUI.openMenu==menu then LUI.openMenu=nil end end)
    button:SetScript("OnClick",function()if menu:IsShown()then menu:Hide()else if LUI.openMenu then LUI.openMenu:Hide()end;LUI.openMenu=menu;offset=0;draw();menu:Show()end end)
    button:SetScript("OnHide",function()menu:Hide()end)
    self:OnRefresh(button,function()button:SetText(selected())end)
    menu:Hide();return button
end
function LUI:Tabs(parent,definitions)
    local pages,buttons={},{}
    local step=math.min(130,math.floor(620/#definitions))
    local function select(index)
        if LUI.openMenu then LUI.openMenu:Hide()end
        for i,page in ipairs(pages) do page:SetShown(i==index);buttons[i]:SetSelected(i==index) end
        parent.selectedTab=definitions[index].id;LUI:RefreshUI()
    end
    for i,definition in ipairs(definitions) do
        local page=CreateFrame("Frame",nil,parent);page:SetPoint("TOPLEFT",0,-44);page:SetSize(620,408)
        page:Hide()
        pages[i]=page;definition.build(page)
        buttons[i]=self:Navigation(parent,definition.name,(i-1)*step,0,step-8,function()select(i)end)
    end
    parent.tabPages=pages;parent.tabButtons=buttons;parent.selectTab=select;select(1)
end
function LUI:BuildUI()
    if self.frame then return end
    local frame=CreateFrame("Frame","LamdaUIFrame",UIParent,"BackdropTemplate")
    -- New WoW frames are shown by default. Keep construction (and any failed
    -- construction) off screen until navigation has selected its active pages.
    frame:Hide();self.buildingUI=true
    self.frame=frame;frame:SetSize(880,740);frame:SetPoint("CENTER");frame:SetFrameStrata("DIALOG")
    frame:SetClampedToScreen(true);frame:SetMovable(true);frame:EnableMouse(true);frame:RegisterForDrag("LeftButton")
    frame:SetScript("OnDragStart",function(self)if not LUI:CombatManagementBlocked()then self:StartMoving()end end)
    frame:SetScript("OnDragStop",function(self)self:StopMovingOrSizing()end)
    frame:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
    frame:SetBackdropColor(0.04,0.07,0.11,0.97);frame:SetBackdropBorderColor(0.18,0.26,0.32,1)
    frame:SetScript("OnMouseDown",function()
        if LUI.openMenu then LUI.openMenu:Hide()end
        if LUI.pinnedHelp then LUI.pinnedHelp.pinned=false;LUI.pinnedHelp=nil;if GameTooltip then GameTooltip:Hide()end end
    end)
    self:Label(frame,"LamdaUI",18,-16,true)
    local metadata=C_AddOns and C_AddOns.GetAddOnMetadata or GetAddOnMetadata
    self:Label(frame,type(metadata)=="function" and (metadata("lamdaUI","Version") or "") or "",710,-20)
    local close=CreateFrame("Button",nil,frame,"UIPanelCloseButton");close:SetPoint("TOPRIGHT",-3,-3)
    local general=CreateFrame("Frame",nil,frame);general:SetPoint("TOPLEFT",20,-98);general:SetSize(800,530)
    general:Hide()
    self:BuildGeneral(general)
    local modules=CreateFrame("Frame",nil,frame);modules:SetPoint("TOPLEFT",20,-98);modules:SetSize(800,530)
    modules:Hide()
    self.generalPage=general;self.modulesPage=modules
    local scroll=CreateFrame("ScrollFrame",nil,modules,"UIPanelScrollFrameTemplate")
    scroll:SetPoint("TOPLEFT",0,0);scroll:SetSize(130,470)
    local list=CreateFrame("Frame",nil,scroll);list:SetSize(130,math.max(470,#self.modules*36));scroll:SetScrollChild(list)
    local panels,buttons={},{};self.modulePanels=panels
    function self:SelectModule(id)
        if self.openMenu then self.openMenu:Hide()end
        if id~="cd" then self:StopFramePreview() end
        if id~="kick" and self.Kick then self.Kick:StopPreview()end
        for i,module in ipairs(self.modules) do
            local selected=module.id==id;panels[i]:SetShown(selected);buttons[i]:SetSelected(selected)
            if selected then self.selectedModule=id end
        end
        self:RefreshUI()
    end
    for i,module in ipairs(self.modules) do
        local detail=CreateFrame("Frame",nil,modules);detail:SetPoint("TOPLEFT",160,0);detail:SetSize(620,530)
        detail:Hide()
        panels[i]=detail;self:Label(detail,module.name..(module.experimental and " |cffffcc66(Experimental)|r" or ""),4,0,true)
        if module.enabledKey then self:Checkbox(detail,"Enabled",module.enabledKey,-32,0,module.enabledHelp) end
        local content=CreateFrame("Frame",nil,detail);content:SetPoint("TOPLEFT",0,-76);content:SetSize(620,460)
        detail.content=content;module.build(content)
        buttons[i]=self:Navigation(list,module.name,0,-(i-1)*36,130,function()LUI:SelectModule(module.id)end)
        if module.experimental then
            local button=buttons[i];button:SetHeight(34)
            button.label:ClearAllPoints();button.label:SetPoint("TOP",0,-3)
            local status=button:CreateFontString(nil,"OVERLAY","GameFontHighlightSmall")
            status:SetPoint("BOTTOM",0,5);status:SetWidth(118);status:SetMaxLines(1);status:SetWordWrap(false)
            status:SetText("Experimental");status:SetTextColor(1,.8,.4)
        end
    end
    local generalButton,modulesButton
    function self:SelectPage(page)
        if self.openMenu then self.openMenu:Hide()end
        if self.profileDialog then self.profileDialog:Hide() end
        local isGeneral=page~="modules";self.selectedPage=isGeneral and "general" or "modules"
        if isGeneral then self:StopFramePreview();if self.Kick then self.Kick:StopPreview()end end
        general:SetShown(isGeneral);modules:SetShown(not isGeneral)
        generalButton:SetSelected(isGeneral);modulesButton:SetSelected(not isGeneral);self:RefreshUI()
    end
    generalButton=self:Navigation(frame,"General",18,-52,118,function()LUI:SelectPage("general")end)
    modulesButton=self:Navigation(frame,"Modules",142,-52,118,function()LUI:SelectPage("modules")end)
    modulesButton:SetShown(#self.modules>0)
    if self.modules[1] then self:SelectModule(self.modules[1].id) end
    local apply=self:Button(frame,"Apply",18,-690,94,function()LUI:Apply()end)
    self.applyButton=apply
    self:Explain(apply,"Apply without reloading","Keep your changes in the active profile for this session. You can close these settings and reload later. WoW writes profiles to disk on reload or logout; a crash before then can lose changes.")
    local save=self:Button(frame,"Apply & Reload",124,-690,150,function()LUI:Save()end)
    self.reloadButton=save
    self:Explain(save,"Apply and reload","Keep the active profile, then reload the UI to write settings to disk and update Lamda Engine. When Allow changes during combat is off, this waits until combat ends.")
    self.applyHelp=self:Help(frame,"Reload required","Apply & Reload sends these settings to the engine. You can also Apply now and reload later.",292,-690)
    function self:RefreshApplyStatus()
        save:SetShown(self.reloadRequired);save:SetEnabled(not self:CombatManagementBlocked())
        self.applyHelp:SetShown(self.reloadRequired)
    end
    self:OnRefresh(self.applyHelp,function()LUI:UpdateReloadRequired();LUI:RefreshApplyStatus()end)
    local function refresh()save:SetEnabled(not LUI:CombatManagementBlocked())end
    self:OnRefresh(save,refresh)
    if self.Sim and self.Sim.RefreshEnabled then self:OnRefresh(frame,function()LUI.Sim:RefreshEnabled()end)end
    frame:RegisterEvent("PLAYER_REGEN_DISABLED");frame:RegisterEvent("PLAYER_REGEN_ENABLED")
    frame:SetScript("OnEvent",function()refresh();LUI:RefreshUI()end)
    frame:SetScript("OnShow",function()refresh();LUI:RefreshUI()end)
    frame:SetScript("OnHide",function()
        LUI:StopFramePreview()
        if LUI.LookupEvents then LUI.LookupEvents:LockStrips()end
        if LUI.Kick then LUI.Kick:StopPreview()end
        if LUI.Performance then LUI.Performance:SetMoving(false)end
        if LUI.profileDialog then LUI.profileDialog:Hide()end
        if LUI.resetDialog then LUI.resetDialog:Hide()end
    end)
    local height=UIParent:GetHeight();if type(height)=="number" and height>0 then frame:SetScale(math.min(1,(height-30)/740))end
    self.buildingUI=false
    self:SelectPage("general")
    UISpecialFrames=UISpecialFrames or {};table.insert(UISpecialFrames,"LamdaUIFrame")
end
function LUI:OpenUI()
    self:SelectPage("general");self.frame:Show()
end
