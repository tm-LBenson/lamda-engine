local _,LUI=...
function LUI:ShowProfileDialog(importing)
    if not self.profileDialog then
        local dialog=CreateFrame("Frame","LamdaUIProfileTransfer",self.frame,"BackdropTemplate")
        self.profileDialog=dialog;dialog:SetSize(690,390);dialog:SetPoint("CENTER");dialog:SetFrameStrata("FULLSCREEN_DIALOG")
        dialog:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
        dialog:SetBackdropColor(0.04,0.07,0.11,1);dialog:SetBackdropBorderColor(0.2,0.75,0.65,1)
        dialog.title=self:Label(dialog,"",18,-18,true)
        dialog.nameLabel=self:Label(dialog,"Name",18,-57)
        dialog.name=CreateFrame("EditBox",nil,dialog,"InputBoxTemplate");dialog.name:SetSize(300,24);dialog.name:SetPoint("TOPLEFT",80,-50)
        dialog.name:SetAutoFocus(false);dialog.name:SetMaxLetters(40)
        local scroll=CreateFrame("ScrollFrame",nil,dialog,"UIPanelScrollFrameTemplate")
        scroll:SetPoint("TOPLEFT",20,-94);scroll:SetSize(630,226)
        local edit=CreateFrame("EditBox",nil,scroll);dialog.edit=edit
        edit:SetMultiLine(true);edit:SetFontObject(ChatFontNormal);edit:SetSize(630,226);edit:SetAutoFocus(false);edit:SetMaxLetters(65536)
        edit:SetScript("OnTextChanged",function(self)
            local _,lines=self:GetText():gsub("\n","");self:SetHeight(math.max(226,(lines+2)*15))
        end)
        edit:SetScript("OnEscapePressed",function()dialog:Hide()end);scroll:SetScrollChild(edit)
        dialog.error=self:Label(dialog,"",18,-328)
        dialog.action=self:Button(dialog,"Import",18,-350,140,function()
            local ok,err=LUI:ImportProfile(dialog.name:GetText(),edit:GetText())
            if ok then dialog:Hide()else dialog.error:SetText(err)end
        end)
        self:Button(dialog,"Close",530,-350,140,function()dialog:Hide()end)
        table.insert(UISpecialFrames,"LamdaUIProfileTransfer")
    end
    local dialog=self.profileDialog
    dialog.title:SetText(importing and "Import profile" or "Export profile")
    dialog.nameLabel:SetShown(importing);dialog.name:SetShown(importing);dialog.action:SetShown(importing)
    dialog.error:SetText("");dialog.name:SetText("");dialog.edit:SetText(importing and "" or self:ExportProfile())
    dialog:Show()
    if importing then dialog.name:SetFocus()else dialog.edit:SetFocus();dialog.edit:HighlightText()end
end
function LUI:BuildGeneral(parent)
    self:Label(parent,"General",4,0,true)
    self:Label(parent,"Engine updates",4,-44,true)
    self:Checkbox(parent,"Check for updates","checkUpdates",-80)
    local notify=self:Checkbox(parent,"Notify me","notifyUpdates",-116)
    self:Label(parent,"Check frequency",4,-164)
    local frequency=self:Button(parent,"",4,-188,260,function()
        local db=LUI:DB();db.checkDays=db.checkDays==1 and 7 or 1;LUI:RefreshUI()
    end)
    self:OnRefresh(frequency,function()
        local db=LUI:DB();frequency:SetText(db.checkDays==1 and "Daily" or "Weekly")
        frequency:SetEnabled(db.checkUpdates);notify:SetEnabled(db.checkUpdates)
    end)
    self:Label(parent,"Profiles",360,-44,true)
    self:Label(parent,"Module settings",360,-72)
    local status=self:Label(parent,"",360,-336)
    self:Dropdown(parent,360,-100,360,function()return LUI:ProfileNames()end,function()return LUI:ProfileDB().active end,function(name)
        local ok,err=LUI:SelectProfile(name);status:SetText(ok and "" or err)
    end)
    self:Label(parent,"Profile name",360,-144)
    local name=CreateFrame("EditBox",nil,parent,"InputBoxTemplate")
    name:SetSize(350,24);name:SetPoint("TOPLEFT",366,-166);name:SetAutoFocus(false);name:SetMaxLetters(40)
    parent.profileName=name
    local shownProfile
    self:OnRefresh(name,function()
        local active=LUI:ProfileDB().active
        if active~=shownProfile then shownProfile=active;name:SetText(active) end
    end)
    local function action(fn)
        local ok,err=fn();status:SetText(ok and "" or err)
    end
    self:Button(parent,"Save as new",360,-206,174,function()action(function()return LUI:CreateProfile(name:GetText())end)end)
    self:Button(parent,"Rename",546,-206,174,function()action(function()return LUI:RenameProfile(name:GetText())end)end)
    local delete
    delete=self:Button(parent,"Delete profile",360,-246,174,function()
        if delete.confirming~=LUI:ProfileDB().active then
            delete.confirming=LUI:ProfileDB().active;delete:SetText("Confirm delete")
        else delete.confirming=nil;action(function()return LUI:DeleteProfile()end);delete:SetText("Delete profile")end
    end)
    self:OnRefresh(delete,function()delete.confirming=nil;delete:SetText("Delete profile");delete:SetEnabled(#LUI:ProfileNames()>1)end)
    self:Button(parent,"Export",360,-290,174,function()LUI:ShowProfileDialog(false)end)
    self:Button(parent,"Import",546,-290,174,function()LUI:ShowProfileDialog(true)end)
end
