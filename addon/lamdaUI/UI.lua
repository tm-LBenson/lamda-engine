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
    if self.inlinePreview then
        local w,h=self:PreviewGeometry();local f=math.min(0.8,300/w,135/h)
        self:DrawSamples(self.inlinePreview,f)
    end
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
    self.cdGuide:Hide();self.moveControls:Hide();self.frame:Show();self:RefreshCooldownPreview()
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

local _,LUI=...
local inputs={}
function LUI:Button(parent,text,x,y,width,callback)
    local b=CreateFrame("Button",nil,parent,"UIPanelButtonTemplate")
    b:SetSize(width,26);b:SetPoint("TOPLEFT",x,y);b:SetText(text)
    b:SetScript("OnClick",callback);return b
end
function LUI:Checkbox(parent,text,key,y,x)
    local b=CreateFrame("CheckButton",nil,parent,"UICheckButtonTemplate")
    b:SetSize(26,26);b:SetPoint("TOPLEFT",x or 0,y)
    local label=b:CreateFontString(nil,"OVERLAY","GameFontHighlight")
    label:SetPoint("LEFT",b,"RIGHT",4,0);label:SetText(text)
    b:SetScript("OnShow",function(self)self:SetChecked(LUI:DB()[key])end)
    b:SetScript("OnClick",function(self)LUI:DB()[key]=not not self:GetChecked();LUI:RefreshCooldownPreview()end)
    return b
end
function LUI:Number(parent,text,key,y,min,max,x)
    x=x or 0
    local label=parent:CreateFontString(nil,"OVERLAY","GameFontHighlight")
    label:SetPoint("TOPLEFT",x+4,y-5);label:SetText(text)
    local b=CreateFrame("EditBox",nil,parent,"InputBoxTemplate")
    b:SetSize(100,24);b:SetPoint("TOPLEFT",x+155,y);b:SetAutoFocus(false);b:SetMaxLetters(6)
    local function commit()
        local v=tonumber(b:GetText())
        if v and v>=min and v<=max then LUI:DB()[key]=key=="overlayScale" and v or math.floor(v) end
        b:SetText(tostring(LUI:DB()[key]))
    end
    table.insert(inputs,commit)
    b:SetScript("OnShow",function(self)self:SetText(tostring(LUI:DB()[key]))end)
    b:SetScript("OnEnterPressed",function(self)commit();self:ClearFocus()end)
    b:SetScript("OnEditFocusLost",commit)
    b:SetScript("OnEscapePressed",function(self)self:SetText(tostring(LUI:DB()[key]));self:ClearFocus()end)
end
function LUI:BuildUI()
    if self.frame then return end
    local f=CreateFrame("Frame","LamdaUIFrame",UIParent,"BackdropTemplate")
    self.frame=f;f:SetSize(660,520);f:SetPoint("CENTER");f:SetFrameStrata("DIALOG")
    f:SetClampedToScreen(true);f:SetMovable(true);f:EnableMouse(true);f:RegisterForDrag("LeftButton")
    f:SetScript("OnDragStart",function(self)self:StartMoving()end)
    f:SetScript("OnDragStop",function(self)self:StopMovingOrSizing()end)
    f:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
    f:SetBackdropColor(0.04,0.07,0.11,0.97);f:SetBackdropBorderColor(0.18,0.26,0.32,1)
    local title=f:CreateFontString(nil,"OVERLAY","GameFontNormalLarge")
    title:SetPoint("TOPLEFT",18,-16);title:SetText("LamdaUI")
    local close=CreateFrame("Button",nil,f,"UIPanelCloseButton");close:SetPoint("TOPRIGHT",-3,-3)
    local panels,buttons={},{}
    local function selectTab(index)
        for i,p in ipairs(panels)do p:SetShown(i==index);buttons[i]:SetEnabled(i~=index)end
    end
    for i,m in ipairs(self.modules)do
        local p=CreateFrame("Frame",nil,f);p:SetPoint("TOPLEFT",20,-90);p:SetSize(620,370);p:Hide()
        panels[i]=p;m.build(p)
        buttons[i]=self:Button(f,m.name,18+(i-1)*124,-48,118,function()selectTab(i)end)
    end
    self.commitInputs=function()for _,commit in ipairs(inputs)do commit()end end
    local save=self:Button(f,"Apply & Reload",18,-478,150,function()LUI:Save()end)
    local function refresh()save:SetEnabled(not InCombatLockdown())end
    f:RegisterEvent("PLAYER_REGEN_DISABLED");f:RegisterEvent("PLAYER_REGEN_ENABLED")
    f:SetScript("OnEvent",refresh);f:SetScript("OnShow",function()refresh();LUI:RefreshCooldownPreview()end)
    selectTab(1);f:Hide()
    UISpecialFrames=UISpecialFrames or {};table.insert(UISpecialFrames,"LamdaUIFrame")
end

function LUI:Choice(parent,text,key,choices,y,x)
    x=x or 0
    local label=parent:CreateFontString(nil,"OVERLAY","GameFontHighlight")
    label:SetPoint("TOPLEFT",x+4,y-5);label:SetText(text)
    local b=self:Button(parent,"",x+145,y,145,function(self)
        local db=LUI:DB();db[key]=db[key]%#choices+1;self:SetText(choices[db[key]]);LUI:RefreshCooldownPreview()
    end)
    b:SetScript("OnShow",function(self)self:SetText(choices[LUI:DB()[key]] or choices[1])end)
end

function LUI:Slider(parent,text,key,y,min,max,step,x)
    x=x or 0
    local label=parent:CreateFontString(nil,"OVERLAY","GameFontHighlight")
    label:SetPoint("TOPLEFT",x+4,y);label:SetText(text)
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
    slider:SetScript("OnShow",function(self)refreshing=true;self:SetValue(LUI:DB()[key]);refreshing=false;value:SetText(tostring(LUI:DB()[key]))end)
    return slider
end
