local _,LUI=...
local colors={{0.21,0.75,0.65},{0.20,0.52,0.84},{0.58,0.40,0.85},{0.85,0.54,0.20}}
local samples={{"Tank","Shield Wall","~60s",0.5},{"Healer","Pain Suppression","~90s*",0.7},{"Damage","Blur","~25s*",0.4}}
local keys={"anchor","overlayX","overlayY","rowWidth","rowHeight","overlayScale"}
local function clamp(v,lo,hi)return math.max(lo,math.min(hi,v))end
local function round(v)return math.floor(v+0.5)end
function LUI:PreviewGeometry()
    local d=self:DB();local count=math.min(3,d.maxRows);local cols=math.min(d.columns,count)
    local rows=math.ceil(count/cols);local scale=d.overlayScale
    return cols*d.rowWidth*scale+(cols-1)*d.rowGap*scale,rows*d.rowHeight*scale+(rows-1)*d.rowGap*scale,count,cols,rows
end
function LUI:DrawSamples(frame,factor)
    frame.sampleRows=frame.sampleRows or {}
    local d=self:DB();local _,_,count,cols,totalRows=self:PreviewGeometry()
    local color=colors[d.accent] or colors[1]
    for i=1,3 do
        local row=frame.sampleRows[i]
        if not row then
            row=CreateFrame("Frame",nil,frame,"BackdropTemplate");frame.sampleRows[i]=row
            row:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
            row.fill=row:CreateTexture(nil,"BACKGROUND");row.fill:SetColorTexture(color[1],color[2],color[3],0.25)
            row.text=row:CreateFontString(nil,"OVERLAY","GameFontHighlight")
            row.timer=row:CreateFontString(nil,"OVERLAY","GameFontHighlight")
        end
        row:SetShown(i<=count)
        if i<=count then
            local width=d.rowWidth*d.overlayScale*factor;local height=d.rowHeight*d.overlayScale*factor
            local r=math.floor((i-1)/cols);if d.grow==2 then r=totalRows-1-r end
            local c=(i-1)%cols
            row:ClearAllPoints();row:SetPoint("TOPLEFT",c*(width+d.rowGap*d.overlayScale*factor),-r*(height+d.rowGap*d.overlayScale*factor))
            row:SetSize(width,height);row:SetBackdropColor(0.08,0.14,0.19,d.opacity/100)
            row:SetBackdropBorderColor(color[1],color[2],color[3],d.border and 1 or 0)
            row.fill:ClearAllPoints();row.fill:SetPoint("TOPLEFT",1,-1);row.fill:SetSize(math.max(1,(width-2)*samples[i][4]),math.max(1,height-2));row.fill:SetShown(d.bars)
            row.fill:SetColorTexture(color[1],color[2],color[3],0.25)
            local fontSize=math.max(6,math.min(d.fontSize*d.overlayScale*factor,height-4))
            row.text:SetFont(STANDARD_TEXT_FONT,fontSize);row.timer:SetFont(STANDARD_TEXT_FONT,fontSize)
            row.timer:ClearAllPoints();row.timer:SetPoint("RIGHT",-8*factor,0);row.timer:SetText(d.showTimers and samples[i][3] or "")
            row.text:ClearAllPoints();row.text:SetPoint("LEFT",8*factor,0);row.text:SetJustifyH("LEFT");row.text:SetWordWrap(false)
            row.text:SetWidth(math.max(1,width-(d.showTimers and 74 or 16)*factor))
            local parts={};if d.showNames then table.insert(parts,samples[i][1])end;if d.showSpells then table.insert(parts,samples[i][2])end
            row.text:SetText(table.concat(parts,"  "))
        end
    end
end
function LUI:RefreshCooldownPreview()
    local w,h=self:PreviewGeometry();local factor=math.min(0.8,300/w,105/h)
    for _,preview in ipairs(self.cdPreviews or {}) do self:DrawSamples(preview,factor) end
    if self.cdGuide and self.cdGuide:IsShown() and not self.resizingGuide then self:RefreshGuide() end
end
local function screenUnits()
    local physicalWidth,physicalHeight=GetPhysicalScreenSize()
    return physicalWidth,physicalHeight,UIParent:GetWidth()/physicalWidth
end
function LUI:GuideOffsets(left,top,width,height,screenWidth,screenHeight,anchor)
    local ax=((anchor-1)%3)/2;local ay=math.floor((anchor-1)/3)/2
    return round(left-(screenWidth-width)*ax),round(top-(screenHeight-height)*ay)
end
function LUI:RefreshGuide()
    local sw,sh,factor=screenUnits();local w,h=self:PreviewGeometry();local d=self:DB()
    local ax=((d.anchor-1)%3)/2;local ay=math.floor((d.anchor-1)/3)/2
    local f=self.cdGuide;f:ClearAllPoints();f:SetSize(w*factor,h*factor)
    f:SetPoint("TOPLEFT",UIParent,"TOPLEFT",((sw-w)*ax+d.overlayX)*factor,-((sh-h)*ay+d.overlayY)*factor)
    self:DrawSamples(f,factor)
end
function LUI:RecordGuide()
    local sw,sh,factor=screenUnits();local f=self.cdGuide;local d=self:DB()
    local left=(f:GetLeft()-(UIParent:GetLeft() or 0))/factor
    local top=((UIParent:GetTop() or UIParent:GetHeight())-f:GetTop())/factor
    local w,h=self:PreviewGeometry()
    d.overlayX,d.overlayY=self:GuideOffsets(left,top,w,h,sw,sh,d.anchor)
    d.overlayX=clamp(d.overlayX,-16000,16000);d.overlayY=clamp(d.overlayY,-16000,16000)
end
function LUI:FinishMove(cancel)
    if not self.cdGuide or not self.cdGuide:IsShown() then return end
    self.cdGuide:StopMovingOrSizing();self.resizingGuide=false
    if cancel then for k,v in pairs(self.moveSnapshot or {})do self:DB()[k]=v end else self:RecordGuide() end
    self.cdGuide:Hide();self.moveControls:Hide();self.frame:Show();self:RefreshUI()
end
function LUI:MoveCooldowns()
    if InCombatLockdown() then return end
    if self.commitInputs then self.commitInputs() end
    self.moveSnapshot={};for _,k in ipairs(keys)do self.moveSnapshot[k]=self:DB()[k]end
    if not self.cdGuide then
        local f=CreateFrame("Frame","LamdaUICooldownGuide",UIParent,"BackdropTemplate");self.cdGuide=f
        f:SetFrameStrata("DIALOG");f:SetMovable(true);f:SetResizable(true);f:EnableMouse(true);f:RegisterForDrag("LeftButton");f:SetClampedToScreen(true)
        f:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=2})
        f:SetBackdropColor(0.03,0.06,0.09,0.8);f:SetBackdropBorderColor(0.2,0.85,0.7,1)
        local title=f:CreateFontString(nil,"OVERLAY","GameFontNormal")
        title:SetPoint("BOTTOMLEFT",f,"TOPLEFT",0,8);title:SetText("Team cooldowns · Preview — drag to move")
        f:SetScript("OnDragStart",function(self)if not InCombatLockdown()then self:StartMoving()end end)
        f:SetScript("OnDragStop",function(self)self:StopMovingOrSizing();LUI:RecordGuide()end)
        local grip=CreateFrame("Button",nil,f,"UIPanelButtonTemplate");grip:SetSize(22,22);grip:SetPoint("BOTTOMRIGHT",6,-6);grip:SetText("//")
        grip:SetScript("OnEnter",function(self)GameTooltip:SetOwner(self,"ANCHOR_RIGHT");GameTooltip:SetText("Drag to resize");GameTooltip:Show()end)
        grip:SetScript("OnLeave",function()GameTooltip:Hide()end)
        grip:SetScript("OnMouseDown",function(_,button)
            if button=="LeftButton" and not InCombatLockdown()then
                LUI.resizingGuide=true;f:StartSizing("BOTTOMRIGHT")
            end
        end)
        grip:SetScript("OnMouseUp",function()
            if not LUI.resizingGuide then return end
            f:StopMovingOrSizing();local _,_,factor=screenUnits();local d=LUI:DB();local _,_,_,cols,rows=LUI:PreviewGeometry()
            d.rowWidth=clamp(round((f:GetWidth()/factor-(cols-1)*d.rowGap*d.overlayScale)/cols/d.overlayScale),120,900)
            d.rowHeight=clamp(round((f:GetHeight()/factor-(rows-1)*d.rowGap*d.overlayScale)/rows/d.overlayScale),20,100)
            LUI.resizingGuide=false;LUI:RecordGuide();LUI:RefreshGuide()
        end)
        local controls=CreateFrame("Frame",nil,UIParent,"BackdropTemplate");self.moveControls=controls
        controls:SetSize(310,54);controls:SetPoint("BOTTOM",0,80);controls:SetFrameStrata("DIALOG")
        controls:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8"});controls:SetBackdropColor(0.03,0.06,0.09,0.95)
        LUI:Button(controls,"Apply & Reload",12,-14,160,function()if not InCombatLockdown()then LUI:FinishMove(false);LUI:Save()end end)
        LUI:Button(controls,"Cancel",184,-14,110,function()LUI:FinishMove(true)end)
        f:RegisterEvent("PLAYER_REGEN_DISABLED");f:SetScript("OnEvent",function()LUI:FinishMove(true)end)
        f:Hide();controls:Hide()
    end
    self.frame:Hide();self.cdGuide:Show();self.moveControls:Show();self:RefreshGuide()
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
function LUI:Checkbox(parent,text,key,y,x)
    local button=CreateFrame("CheckButton",nil,parent,"UICheckButtonTemplate")
    button:SetSize(26,26);button:SetPoint("TOPLEFT",x or 0,y)
    local label=button:CreateFontString(nil,"OVERLAY","GameFontHighlight")
    label:SetPoint("LEFT",button,"RIGHT",4,0);label:SetText(text)
    self:OnRefresh(button,function()button:SetChecked(LUI:DB()[key])end)
    button:SetScript("OnClick",function(self)LUI:DB()[key]=not not self:GetChecked();LUI:RefreshUI()end)
    return button
end
function LUI:Choice(parent,text,key,choices,y,x)
    x=x or 0;self:Label(parent,text,x+4,y-5)
    local button=self:Button(parent,"",x+125,y,135,function(self)
        local db=LUI:DB();db[key]=db[key]%#choices+1;LUI:RefreshUI()
    end)
    self:OnRefresh(button,function()button:SetText(choices[LUI:DB()[key]] or choices[1])end)
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
        v=math.max(min,math.min(max,math.floor(v/step+0.5)*step));LUI:DB()[key]=v
        value:SetText(step<1 and string.format("%.2f",v) or tostring(v));LUI:RefreshCooldownPreview()
    end)
    self:OnRefresh(slider,function()
        refreshing=true;slider:SetValue(LUI:DB()[key]);refreshing=false
        value:SetText(step<1 and string.format("%.2f",LUI:DB()[key]) or tostring(LUI:DB()[key]))
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
    self:OnRefresh(button,function()button:SetText(selected().."  ▾")end)
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
        general:SetShown(isGeneral);modules:SetShown(not isGeneral)
        generalButton:SetEnabled(not isGeneral);modulesButton:SetEnabled(isGeneral);self:RefreshUI()
    end
    generalButton=self:Button(frame,"General",18,-52,118,function()LUI:SelectPage("general")end)
    modulesButton=self:Button(frame,"Modules",142,-52,118,function()LUI:SelectPage("modules")end)
    if self.modules[1] then self:SelectModule(self.modules[1].id) end
    local save=self:Button(frame,"Apply & Reload",18,-548,150,function()LUI:Save()end)
    local function refresh()save:SetEnabled(not InCombatLockdown())end
    frame:RegisterEvent("PLAYER_REGEN_DISABLED");frame:RegisterEvent("PLAYER_REGEN_ENABLED")
    frame:SetScript("OnEvent",refresh);frame:SetScript("OnShow",function()refresh();LUI:RefreshUI()end)
    frame:SetScript("OnHide",function()if LUI.profileDialog then LUI.profileDialog:Hide()end end)
    self:SelectPage("general");frame:Hide()
    UISpecialFrames=UISpecialFrames or {};table.insert(UISpecialFrames,"LamdaUIFrame")
end
function LUI:OpenUI()
    -- Reopening the hub always ends temporary placement mode first.
    self:FinishMove(true);self:SelectPage("general");self.frame:Show()
end
