local _,LUI=...
function LUI:BuildGeneral(parent)
    local title=parent:CreateFontString(nil,"OVERLAY","GameFontNormalLarge")
    title:SetPoint("TOPLEFT",4,0);title:SetText("General")
    local settings=CreateFrame("Frame",nil,parent)
    settings:SetPoint("TOPLEFT",0,-40);settings:SetSize(620,300)
    parent=settings
    LUI:Checkbox(parent,"Check for updates","checkUpdates",0)
    LUI:Checkbox(parent,"Notify me","notifyUpdates",-36)
    local frequency=LUI:Button(parent,"",0,-86,140,function(self)
        local db=LUI:DB();db.checkDays=db.checkDays==1 and 7 or 1
        self:SetText(db.checkDays==1 and "Daily" or "Weekly")
    end)
    frequency:SetScript("OnShow",function(self)
        self:SetText(LUI:DB().checkDays==1 and "Daily" or "Weekly")
    end)
end
