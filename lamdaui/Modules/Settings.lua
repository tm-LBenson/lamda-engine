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
function LUI:BuildGeneralSettings(parent)
    self:Label(parent,"Engine updates",4,-44,true)
    self:Checkbox(parent,"Check for updates","checkUpdates",-80,0,"Lamda Engine checks the public release version while it is running. This never downloads or installs an update automatically.")
    local notify=self:Checkbox(parent,"Notify me","notifyUpdates",-116,0,"Show a Windows notification when the running engine finds a newer release. Turning this off keeps version checks but suppresses the notification.")
    self:Label(parent,"Check frequency",4,-164)
    local frequency=self:Dropdown(parent,4,-188,260,function()return {"Daily","Weekly"}end,
        function()return LUI:DB().checkDays==1 and "Daily" or "Weekly"end,
        function(value)LUI:DB().checkDays=value=="Daily" and 1 or 7 end)
    self:Explain(frequency,"Check frequency","How often the running engine checks for a newer release. The engine also checks when it starts.")
    self:OnRefresh(frequency,function()
        local db=LUI:DB()
        frequency:SetEnabled(db.checkUpdates);notify:SetEnabled(db.checkUpdates)
    end)
    self:Description(parent,"Update checks require Lamda Engine to be running.",4,-236,285)
    self:Label(parent,"Overlay visibility",4,-284,true)
    self:Checkbox(parent,"Keep UI visible when tabbed out","keepUIWhenUnfocused",-322,0,
        "Keep Lamda Engine's cooldown and kick overlays visible when you switch to another app. Timers continue at the last known frame positions and may appear over other windows. Live screen readings resume when WoW regains focus. The overlay still hides if WoW is minimized or closed. Off by default; applies live while the engine can read the event strip.")
    self:Checkbox(parent,"Allow changes during combat","allowCombatManagement",-366,0,
        "Keep LamdaUI settings and non-secure movement controls available during combat. Protected frame layouts, secure bindings, addon loading, and restricted game data still wait until combat ends, as required by WoW.")
    local resetGeneral=self:ResetButton(parent,"General settings",function()LUI:ResetGeneral()end,4,-414)
    resetGeneral:SetText("Reset General")
    local resetAll=self:ResetButton(parent,"General and all modules in this profile",function()LUI:ResetAllSettings()end,4,-454)
    resetAll:SetText("Reset all settings")
    self:Explain(resetAll,"Reset all settings","Restore General, shared Performance settings, and the current profile's modules. Other named profiles are preserved. A confirmation appears first.")
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

function LUI:BuildColorStripSettings(parent)
    local function bridge()return LUI.LookupEvents end
    local move=self:Button(parent,"Move all",4,0,112,function()bridge():MoveAll();LUI:RefreshUI()end)
    local lock=self:Button(parent,"Lock all",124,0,112,function()bridge():LockStrips();LUI:RefreshUI()end)
    local arrange=self:Dropdown(parent,244,0,140,function()return {"Row","Column"}end,function()return "Arrange…"end,function(value)
        if not bridge():Arrange(value)then print("LamdaUI: This layout does not fit on screen. Resize the Overlays strip first.")end
    end)
    local together=CreateFrame("CheckButton",nil,parent,"UICheckButtonTemplate")
    together:SetSize(26,26);together:SetPoint("TOPLEFT",408,0);together:SetHitRectInsets(0,-132,0,0)
    local label=self:Label(together,"Move together",30,-6);label:SetWidth(132)
    together:SetScript("OnClick",function()
        bridge():FinishGroupDrag();bridge().moveTogether=not not together:GetChecked()
    end)
    self:Help(parent,"Color strips","Move all unlocks enabled strips. Drag the colors to move them. Move together keeps unlocked strips in the same arrangement. Row and Column arrange enabled or unlocked strips with room for their extra data bands. Use the gold grip beside Overlays to resize it; other strips keep their capture size. Lock all, then Apply & Reload when finished. Module switches remain under Modules.",748,0)
    self:OnRefresh(move,function()
        local available=bridge()~=nil and not LUI:CombatManagementBlocked()
        move:SetEnabled(available);lock:SetEnabled(available);arrange:SetEnabled(available);together:SetEnabled(available)
        together:SetChecked(bridge()and bridge().moveTogether or false)
    end)
    parent.stripRows={}
    for index,definition in ipairs({{"cd","LamdaCD"},{"kick","LamdaKick"},{"gladius","LamdaGladius"},{"kbb","LamdaKBB"},{"sim","LamdaSim"},{"overlays","LamdaOverlays"},{"augmentation","LamdaMeters"}})do
        local id,name=definition[1],definition[2];local y=-60-(index-1)*52
        self:Label(parent,name,4,y-6)
        local status=self:Label(parent,"",174,y-6)
        local rowMove,rowReset=self:ColorStripControls(parent,id,270,y)
        parent.stripRows[id]={move=rowMove,reset=rowReset,status=status}
        self:OnRefresh(status,function()
            local registered=false
            for _,module in ipairs(LUI.modules)do if module.id==id then registered=true;break end end
            local active=registered and bridge()and bridge():GetStrip(id):Enabled()
            status:SetText(active and "Active"or "Off");status:SetTextColor(active and .4 or .55,active and .9 or .6,active and .75 or .65)
            rowMove:SetEnabled(registered and bridge()~=nil and not LUI:CombatManagementBlocked())
            rowReset:SetEnabled(registered and bridge()~=nil and not LUI:CombatManagementBlocked())
        end)
        if id=="overlays"then
            local widths={Horizontal=512,Vertical=64,Square=144}
            local shape=self:Dropdown(parent,490,y,140,function()
                local options={};local w,h=GetPhysicalScreenSize()
                for _,name in ipairs({"Horizontal","Vertical","Square"})do if widths[name]<=w and 18432/widths[name]<=h then options[#options+1]=name end end
                return options
            end,function()return "Shape…"end,function(value)
                local strip=bridge():GetStrip(id);strip:FinishResize();strip:FinishDrag();strip:SetMoving(true)
                LUI:DB().overlaysBridgeWidth=widths[value];strip:ApplyPosition()
            end)
            self:OnRefresh(shape,function()shape:SetEnabled(LUI:DefaultValue("overlaysBridgeWidth")~=nil and bridge()~=nil and not LUI:CombatManagementBlocked())end)
        end
    end
    self:Checkbox(parent,"CD event strip","lookupBridge",-434,0,"Send cooldown observations and frame positions to Lamda Engine. Keep the strip and its buff band visible. Apply & Reload after changes.")
    local reset=self:ResetButton(parent,"color strip positions and shape",function()
        bridge():LockStrips();for _,strip in ipairs(bridge():ManagedStrips())do strip:ResetPosition()end;LUI:RefreshUI()
    end,490,-434)
    reset:SetText("Reset all")
    self:OnRefresh(reset,function()reset:SetEnabled(bridge()~=nil and not LUI:CombatManagementBlocked())end)
    self:Checkbox(parent,"Start hidden","startHidden",-482,0,"Start each login or reload with the color strips and engine displays hidden. Screen-fed tracking and transfers pause until you click Hidden to show them again.")
    local visibility=self:Button(parent,"Hide",330,-482,112,function()bridge():SetOverlayHidden(not LUI.overlayHidden);LUI:RefreshUI()end)
    self:OnRefresh(visibility,function()visibility:SetText(LUI.overlayHidden and "Show" or "Hide");visibility:SetEnabled(bridge()~=nil)end)
    if self.MeterOverlay and self.MeterOverlay:Enabled()then self.MeterOverlay:BuildStripControls(parent)end
    parent:HookScript("OnHide",function()if bridge()then bridge():LockStrips()end end)
end

function LUI:BuildTrackingSettings(parent)
    local modes={"Once per group","Group + key start","Key start only","Periodically"}
    self:Label(parent,"Talent scans",4,0,true)
    local timing=self:Dropdown(parent,4,-36,280,function()return modes end,
        function()return modes[LUI:DB().scanTiming]end,function(value)
            for i,name in ipairs(modes)do if name==value then LUI:DB().scanTiming=i end end
        end)
    self:Explain(timing,"Talent scans","Shared by enabled CD and Kick talent tracking. Once per group scans each player when they join. Group + key start also refreshes when a Mythic+ key begins. Key start only waits for that event. Periodically also repeats while grouped. Scans run outside combat and may take several seconds; unavailable players get up to three attempts. Your own talents update locally. Applies across characters and profiles. Apply & Reload also updates web fallback timing.")
    local interval=self:Dropdown(parent,300,-36,150,function()return {"1 minute","3 minutes","5 minutes"}end,
        function()return tostring(LUI:DB().scanMinutes)..(LUI:DB().scanMinutes==1 and " minute" or " minutes")end,
        function(value)LUI:DB().scanMinutes=tonumber(value:match("^%d+"))end)
    self:OnRefresh(interval,function()interval:SetShown(LUI:DB().scanTiming==4)end)
    local scan=self:Button(parent,"Scan now",4,-88,150,function()LUI.InspectTalents:RequestRefresh();LUI:RefreshUI()end)
    self:Explain(scan,"Scan now","Request one fresh scan of the current party. Requires talent inspection in CD or Kick and under Performance. Inspection waits while Blizzard's inspect window is open.")
    self:OnRefresh(scan,function()scan:SetEnabled(LUI.InspectTalents and LUI.InspectTalents:Enabled() and not InCombatLockdown())end)
    self:Help(parent,"Tracking","Default: scan on joining a group and at key start. Use Once per group to avoid repeated scans, or Key start only to wait until the run begins. A key-start scan starts during the countdown; combat or unavailable players can delay it. Last confirmed builds remain in use until refreshed. Performance and individual module switches still apply.",470,-36)
    self:ResetButton(parent,"tracking settings",function()LUI:ResetKeys({"scanTiming","scanMinutes"})end,4,-150)
end

function LUI:BuildResourceSettings(parent)
    local advanced=self:Checkbox(parent,"Advanced","resourceAdvanced",0,0,
        "Replace the simple switch with individual options. Your custom choices are remembered. These settings apply across characters and profiles; module switches still decide what is enabled.")
    advanced:SetScript("OnClick",function(button)LUI:SetResourceAdvanced(button:GetChecked())end)
    local simple=CreateFrame("Frame",nil,parent);simple:SetPoint("TOPLEFT",0,-52);simple:SetSize(760,80)
    self:Checkbox(simple,"Low resource mode","resourceLow",0,0,
        "Reduce optional work: skip party talent inspection, Raider.IO lookups, extra frame discovery and CPU diagnostics. Check inactive strips less often and use fewer display updates. Active capture, cooldowns, alerts and sounds keep running. Module choices are preserved. Apply & Reload to update the engine.")
    local options=CreateFrame("Frame",nil,parent);options:SetPoint("TOPLEFT",0,-52);options:SetSize(760,290)
    local choices={
        {"Party talent inspection","resourceInspect","Allow enabled CD/Kick modules to inspect party talents. Off reduces work when joining parties, but talent-dependent cooldowns may be less precise."},
        {"Raider.IO lookups","resourceLookups","Allow LamdaCD's optional public talent lookups. Off avoids these web requests. Requires Apply & Reload."},
        {"Extra frame discovery","resourceDiscovery","Find unit frames from addons without a known integration. Off keeps Blizzard, LamdaFrames, DandersFrames and oUF/ElvUI discovery; other unit-frame addons may lose attached indicators."},
        {"Fast idle checks","resourceFastIdle","Check inactive or unreadable strips five times per second. Off backs off to once per second after five failed readings; detecting a newly enabled strip can take up to one second. Valid active strips always keep five readings per second. Requires Apply & Reload."},
        {"Smooth display updates","resourceSmooth","Use normal engine display updates: 10 per second, or 30 during animations. Off uses 5 or 10. Timing, triggers and sounds are unchanged. Requires Apply & Reload."},
        {"CPU diagnostics","resourceCPU","Allow addon CPU and task measurements when the Performance module requests them. Off keeps its frame-time and optional network measurements available."},
    }
    for index,choice in ipairs(choices)do self:Checkbox(options,choice[1],choice[2],-(index-1)*44,0,choice[3])end
    self:Label(parent,"Active tracking: required",4,-350)
    self:Help(parent,"Performance options","These options limit optional work without enabling disabled modules. Active color capture, cooldown and aura tracking, rule evaluation, alerts, sounds and item protections remain available. Module-specific options still apply. Apply & Reload saves engine settings; your existing choices are retained when switching modes.",270,-350)
    self:ResetButton(parent,"performance options",function()LUI:ResetResourceSettings()end,4,-400)
    parent.simple=simple;parent.advancedOptions=options
    self:OnRefresh(advanced,function()
        local custom=LUI:DB().resourceAdvanced
        simple:SetShown(not custom);options:SetShown(custom)
    end)
end

function LUI:BuildGeneral(parent)
    local settings=CreateFrame("Frame",nil,parent);settings:SetPoint("TOPLEFT",0,-40);settings:SetSize(800,490);settings:Hide()
    local strips=CreateFrame("Frame",nil,parent);strips:SetPoint("TOPLEFT",0,-50);strips:SetSize(800,480);strips:Hide()
    local performance=CreateFrame("Frame",nil,parent);performance:SetPoint("TOPLEFT",0,-50);performance:SetSize(800,480);performance:Hide()
    local tracking=CreateFrame("Frame",nil,parent);tracking:SetPoint("TOPLEFT",0,-50);tracking:SetSize(800,480);tracking:Hide()
    self:BuildGeneralSettings(settings);self:BuildColorStripSettings(strips);self:BuildResourceSettings(performance);self:BuildTrackingSettings(tracking)
    parent.trackingSettings=tracking
    parent.profileName=settings.profileName;parent.colorStrips=strips;parent.performanceSettings=performance
    parent:HookScript("OnHide",function()if LUI.LookupEvents then LUI.LookupEvents:LockStrips()end end)
    local settingsButton,stripsButton,performanceButton,trackingButton
    local function select(page)
        if LUI.openMenu then LUI.openMenu:Hide()end
        settings:SetShown(page=="settings");strips:SetShown(page=="strips");performance:SetShown(page=="performance")
        tracking:SetShown(page=="tracking");trackingButton:SetSelected(page=="tracking")
        settingsButton:SetSelected(page=="settings");stripsButton:SetSelected(page=="strips");performanceButton:SetSelected(page=="performance");LUI:RefreshUI()
    end
    settingsButton=self:Navigation(parent,"Settings",4,0,112,function()select("settings")end)
    stripsButton=self:Navigation(parent,"Color strips",124,0,130,function()select("strips")end)
    performanceButton=self:Navigation(parent,"Performance",262,0,130,function()select("performance")end)
    trackingButton=self:Navigation(parent,"Tracking",400,0,120,function()select("tracking")end)
    select("settings")
end
