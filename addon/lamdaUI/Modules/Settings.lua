local _,LUI=...
LUI:RegisterModule({id="settings",name="Settings",build=function(parent)
    LUI:Checkbox(parent,"Check for updates","checkUpdates",0)
    LUI:Checkbox(parent,"Notify me","notifyUpdates",-36)
    LUI:Checkbox(parent,"Combat logging","autoLog",-72)
    local frequency=LUI:Button(parent,"",0,-122,140,function(self)
        local db=LUI:DB();db.checkDays=db.checkDays==1 and 7 or 1
        self:SetText(db.checkDays==1 and "Daily" or "Weekly")
    end)
    frequency:SetScript("OnShow",function(self)
        self:SetText(LUI:DB().checkDays==1 and "Daily" or "Weekly")
    end)
end})
