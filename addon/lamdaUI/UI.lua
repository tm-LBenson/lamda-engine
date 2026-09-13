local _,LUI=...
function LUI:RefreshCooldownPreview()
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
    for _,refresh in ipairs(self.refreshers) do refresh() end
    self:RefreshCooldownPreview()
end
function LUI:OnRefresh(frame,callback)
    table.insert(self.refreshers,callback);frame:SetScript("OnShow",callback)
end
function LUI:Label(parent,text,x,y,large)
    local label=parent:CreateFontString(nil,"OVERLAY",large and "GameFontNormalLarge" or "GameFontHighlight")
    label:SetPoint("TOPLEFT",x,y);label:SetText(text);return label
end
function LUI:Button(parent,text,x,y,width,callback)
    local button=CreateFrame("Button",nil,parent,"UIPanelButtonTemplate")
    button:SetSize(width,26);button:SetPoint("TOPLEFT",x,y);button:SetText(text)
    button:SetScript("OnClick",callback);return button
end
local function settingKey(key)
    return type(key)=="function" and key() or key
end
function LUI:Checkbox(parent,text,key,y,x)
    local button=CreateFrame("CheckButton",nil,parent,"UICheckButtonTemplate")
    button:SetSize(26,26);button:SetPoint("TOPLEFT",x or 0,y)
    local label=button:CreateFontString(nil,"OVERLAY","GameFontHighlight")
    label:SetPoint("LEFT",button,"RIGHT",4,0);label:SetText(text)
    self:OnRefresh(button,function()button:SetChecked(LUI:DB()[settingKey(key)])end)
    button:SetScript("OnClick",function(self)LUI:DB()[settingKey(key)]=not not self:GetChecked();LUI:RefreshUI()end)
    return button
end
function LUI:Choice(parent,text,key,choices,y,x)
    x=x or 0;self:Label(parent,text,x+4,y-5)
    local button=self:Button(parent,"",x+125,y,135,function(self)
        local db=LUI:DB();local current=settingKey(key);db[current]=db[current]%#choices+1;LUI:RefreshUI()
    end)
    self:OnRefresh(button,function()button:SetText(choices[LUI:DB()[settingKey(key)]] or choices[1])end)
    return button
end
function LUI:Slider(parent,text,key,y,min,max,step,x)
    x=x or 0
    self:Label(parent,text,x+4,y)
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
    return slider
end
-- A compact native menu. Entries are rebuilt when opened so added profiles appear immediately.
function LUI:Dropdown(parent,x,y,width,items,selected,onSelect)
    local button=self:Button(parent,"",x,y,width,function()end)
    local menu=CreateFrame("Frame",nil,button,"BackdropTemplate")
    menu:SetPoint("TOPLEFT",button,"BOTTOMLEFT",0,-2);menu:SetSize(width,190);menu:SetFrameStrata("TOOLTIP")
    menu:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
    menu:SetBackdropColor(0.04,0.07,0.11,1);menu:SetBackdropBorderColor(0.18,0.26,0.32,1)
    local offset=0;local entries={}
    local function draw()
        local values=items();offset=math.max(0,math.min(offset,math.max(0,#values-6)))
        menu:SetHeight(math.min(6,#values)*30+8)
        for i=1,6 do
            if not entries[i] then entries[i]=LUI:Button(menu,"",4,-4-(i-1)*30,width-8,function()end) end
            local entry=entries[i];local value=values[offset+i];entry:SetShown(value~=nil)
            if value then entry:SetText(value);entry:SetScript("OnClick",function()menu:Hide();onSelect(value);LUI:RefreshUI()end) end
        end
    end
    menu:EnableMouse(true);menu:EnableMouseWheel(true)
    menu:SetScript("OnMouseWheel",function(_,delta)offset=offset-delta;draw()end)
    button:SetScript("OnClick",function()if menu:IsShown()then menu:Hide()else offset=0;draw();menu:Show()end end)
    button:SetScript("OnHide",function()menu:Hide()end)
    self:OnRefresh(button,function()button:SetText(selected().."  v")end)
    menu:Hide();return button
end
function LUI:Tabs(parent,definitions)
    local pages,buttons={},{}
    local function select(index)
        for i,page in ipairs(pages) do page:SetShown(i==index);buttons[i]:SetEnabled(i~=index) end
        parent.selectedTab=definitions[index].id;LUI:RefreshUI()
    end
    for i,definition in ipairs(definitions) do
        local page=CreateFrame("Frame",nil,parent);page:SetPoint("TOPLEFT",0,-44);page:SetSize(620,326)
        pages[i]=page;definition.build(page)
        buttons[i]=self:Button(parent,definition.name,(i-1)*130,0,122,function()select(i)end)
    end
    parent.tabPages=pages;parent.selectTab=select;select(1)
end
function LUI:BuildUI()
    if self.frame then return end
    local frame=CreateFrame("Frame","LamdaUIFrame",UIParent,"BackdropTemplate")
    self.frame=frame;frame:SetSize(820,590);frame:SetPoint("CENTER");frame:SetFrameStrata("DIALOG")
    frame:SetClampedToScreen(true);frame:SetMovable(true);frame:EnableMouse(true);frame:RegisterForDrag("LeftButton")
    frame:SetScript("OnDragStart",function(self)if not InCombatLockdown()then self:StartMoving()end end)
    frame:SetScript("OnDragStop",function(self)self:StopMovingOrSizing()end)
    frame:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
    frame:SetBackdropColor(0.04,0.07,0.11,0.97);frame:SetBackdropBorderColor(0.18,0.26,0.32,1)
    self:Label(frame,"LamdaUI",18,-16,true)
    local close=CreateFrame("Button",nil,frame,"UIPanelCloseButton");close:SetPoint("TOPRIGHT",-3,-3)
    local general=CreateFrame("Frame",nil,frame);general:SetPoint("TOPLEFT",20,-98);general:SetSize(780,430)
    self:BuildGeneral(general)
    local modules=CreateFrame("Frame",nil,frame);modules:SetPoint("TOPLEFT",20,-98);modules:SetSize(780,430)
    self.generalPage=general;self.modulesPage=modules
    local scroll=CreateFrame("ScrollFrame",nil,modules,"UIPanelScrollFrameTemplate")
    scroll:SetPoint("TOPLEFT",0,0);scroll:SetSize(130,416)
    local list=CreateFrame("Frame",nil,scroll);list:SetSize(130,math.max(416,#self.modules*36));scroll:SetScrollChild(list)
    local panels,buttons={},{};self.modulePanels=panels
    function self:SelectModule(id)
        if id~="cd" then self:StopFramePreview() end
        for i,module in ipairs(self.modules) do
            local selected=module.id==id;panels[i]:SetShown(selected);buttons[i]:SetEnabled(not selected)
            if selected then self.selectedModule=id end
        end
        self:RefreshUI()
    end
    for i,module in ipairs(self.modules) do
        local detail=CreateFrame("Frame",nil,modules);detail:SetPoint("TOPLEFT",160,0);detail:SetSize(620,430)
        panels[i]=detail;self:Label(detail,module.name,4,0,true)
        if module.enabledKey then self:Checkbox(detail,"Enabled",module.enabledKey,-32) end
        local content=CreateFrame("Frame",nil,detail);content:SetPoint("TOPLEFT",0,-76);content:SetSize(620,370)
        detail.content=content;module.build(content)
        buttons[i]=self:Button(list,module.name,0,-(i-1)*36,130,function()LUI:SelectModule(module.id)end)
    end
    local generalButton,modulesButton
    function self:SelectPage(page)
        if self.profileDialog then self.profileDialog:Hide() end
        local isGeneral=page~="modules";self.selectedPage=isGeneral and "general" or "modules"
        if isGeneral then self:StopFramePreview() end
        general:SetShown(isGeneral);modules:SetShown(not isGeneral)
        generalButton:SetEnabled(not isGeneral);modulesButton:SetEnabled(isGeneral);self:RefreshUI()
    end
    generalButton=self:Button(frame,"General",18,-52,118,function()LUI:SelectPage("general")end)
    modulesButton=self:Button(frame,"Modules",142,-52,118,function()LUI:SelectPage("modules")end)
    if self.modules[1] then self:SelectModule(self.modules[1].id) end
    local save=self:Button(frame,"Apply & Reload",18,-548,150,function()LUI:Save()end)
    local function refresh()save:SetEnabled(not InCombatLockdown())end
    frame:RegisterEvent("PLAYER_REGEN_DISABLED");frame:RegisterEvent("PLAYER_REGEN_ENABLED")
    frame:SetScript("OnEvent",function()refresh();LUI:RefreshUI()end)
    frame:SetScript("OnShow",function()refresh();LUI:RefreshUI()end)
    frame:SetScript("OnHide",function()
        LUI:StopFramePreview()
        if LUI.profileDialog then LUI.profileDialog:Hide()end
    end)
    self:SelectPage("general");frame:Hide()
    UISpecialFrames=UISpecialFrames or {};table.insert(UISpecialFrames,"LamdaUIFrame")
end
function LUI:OpenUI()
    self:SelectPage("general");self.frame:Show()
end
