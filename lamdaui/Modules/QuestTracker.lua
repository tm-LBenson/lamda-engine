local _, LUI = ...
LUI:RegisterModule({id="quests",name="Lamda Quest Tracker",defaults={},build=function(parent)
    LUI:Description(parent,"Quests, personal tasks, categories and tracker appearance are managed in the quest tracker's settings. Settings are saved separately for each character.",4,0,588)
    local status=LUI:Label(parent,"",4,-72)
    local open=LUI:Button(parent,"Open tracker settings",4,-110,230,function()
        if SlashCmdList and SlashCmdList.LAMDAQUESTLOG then
            SlashCmdList.LAMDAQUESTLOG("settings")
        end
    end)
    LUI:OnRefresh(open,function()
        local ready=SlashCmdList and type(SlashCmdList.LAMDAQUESTLOG)=="function"
        open:SetEnabled(not not ready)
        status:SetText(ready and "Quest Tracker is loaded." or "Enable LamdaQuestLog in WoW's AddOns menu, then reload.")
    end)
end})
