local _,LUI=...
local inputs={}
function LUI:Button(parent,text,x,y,width,callback)
    local b=CreateFrame("Button",nil,parent,"UIPanelButtonTemplate")
    b:SetSize(width,26);b:SetPoint("TOPLEFT",x,y);b:SetText(text)
    b:SetScript("OnClick",callback);return b
end
function LUI:Checkbox(parent,text,key,y)
    local b=CreateFrame("CheckButton",nil,parent,"UICheckButtonTemplate")
    b:SetSize(26,26);b:SetPoint("TOPLEFT",0,y)
    local label=b:CreateFontString(nil,"OVERLAY","GameFontHighlight")
    label:SetPoint("LEFT",b,"RIGHT",4,0);label:SetText(text)
    b:SetScript("OnShow",function(self)self:SetChecked(LUI:DB()[key])end)
    b:SetScript("OnClick",function(self)LUI:DB()[key]=not not self:GetChecked()end)
    return b
end
function LUI:Number(parent,text,key,y,min,max)
    local label=parent:CreateFontString(nil,"OVERLAY","GameFontHighlight")
    label:SetPoint("TOPLEFT",4,y-5);label:SetText(text)
    local b=CreateFrame("EditBox",nil,parent,"InputBoxTemplate")
    b:SetSize(100,24);b:SetPoint("TOPLEFT",155,y);b:SetAutoFocus(false);b:SetMaxLetters(6)
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
    self.frame=f;f:SetSize(410,400);f:SetPoint("CENTER");f:SetFrameStrata("DIALOG")
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
        local p=CreateFrame("Frame",nil,f);p:SetPoint("TOPLEFT",20,-90);p:SetSize(370,250);p:Hide()
        panels[i]=p;m.build(p)
        buttons[i]=self:Button(f,m.name,18+(i-1)*124,-48,118,function()selectTab(i)end)
    end
    self.commitInputs=function()for _,commit in ipairs(inputs)do commit()end end
    local save=self:Button(f,"Save & Reload",18,-358,150,function()LUI:Save()end)
    local function refresh()save:SetEnabled(not InCombatLockdown())end
    f:RegisterEvent("PLAYER_REGEN_DISABLED");f:RegisterEvent("PLAYER_REGEN_ENABLED")
    f:SetScript("OnEvent",refresh);f:SetScript("OnShow",refresh)
    selectTab(1);f:Hide()
    UISpecialFrames=UISpecialFrames or {};table.insert(UISpecialFrames,"LamdaUIFrame")
end
