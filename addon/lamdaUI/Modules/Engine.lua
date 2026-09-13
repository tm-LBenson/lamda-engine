local _, Addon = ...
local defaults = {schema=1, cdEnabled=true, checkUpdates=true, notifyUpdates=true,
    checkDays=1, overlayX=60, overlayY=240, overlayScale=1}
local function db()
    if type(LamdaEngineDB) ~= "table" then LamdaEngineDB = {} end
    for k,v in pairs(defaults) do
        if type(LamdaEngineDB[k]) ~= type(v) then LamdaEngineDB[k] = v end
    end
    LamdaEngineDB.schema = 1
    return LamdaEngineDB
end
local function saveButton(parent)
    local button = Addon:CustomHubButton(parent, "Save & Reload", 0, -224, 150, function()
        if not InCombatLockdown() then
            for _, box in ipairs(parent.engineInputs or {}) do box:GetScript("OnEditFocusLost")(box) end
            ReloadUI()
        end
    end)
    parent:RegisterEvent("PLAYER_REGEN_DISABLED")
    parent:RegisterEvent("PLAYER_REGEN_ENABLED")
    local refresh = function() button:SetEnabled(not InCombatLockdown()) end
    parent:SetScript("OnEvent", refresh)
    parent:HookScript("OnShow", refresh)
    refresh()
end
local function input(parent, label, key, y, min, max)
    Addon:CustomHubText(parent, label, 0, y, 150)
    local box=CreateFrame("EditBox",nil,parent,"InputBoxTemplate")
    box:SetSize(90,24);box:SetPoint("TOPLEFT",180,y+4);box:SetAutoFocus(false)
    parent.engineInputs = parent.engineInputs or {}
    table.insert(parent.engineInputs, box)
    box:SetMaxLetters(6)
    local function commit(self)
        local value=tonumber(self:GetText())
        if value and value>=min and value<=max then
            db()[key]=key=="overlayScale" and value or math.floor(value)
        end
        self:SetText(tostring(db()[key]));self:ClearFocus()
    end
    box:SetScript("OnEnterPressed",commit)
    box:SetScript("OnEditFocusLost",function(self)
        local value=tonumber(self:GetText())
        if value and value>=min and value<=max then db()[key]=key=="overlayScale" and value or math.floor(value) end
        self:SetText(tostring(db()[key]))
    end)
    box:SetScript("OnEscapePressed",function(self)self:SetText(tostring(db()[key]));self:ClearFocus()end)
    box:SetScript("OnShow",function(self)self:SetText(tostring(db()[key]))end)
end
Addon:RegisterCustomModule({id="lamdaCD",name="LamdaCD",description="",defaults={enabled=true},
    initialize=function()
        db()
        Addon:GetCustomModuleSettings("lamdaCD").enabled=db().cdEnabled
    end,
    changed=function()db().cdEnabled=Addon:IsCustomModuleEnabled("lamdaCD")end,
    buildOptions=function(parent)
        input(parent,"Left", "overlayX",0,0,16000)
        input(parent,"Top", "overlayY",-40,0,16000)
        input(parent,"Scale", "overlayScale",-80,0.5,3)
        saveButton(parent)
    end})
Addon:RegisterCustomModule({id="engineSettings",name="Settings",description="",noEnable=true,
    buildOptions=function(parent)
        Addon:CustomHubCheckbox(parent,"Check for updates",0,function()return db().checkUpdates end,function(v)db().checkUpdates=not not v end)
        Addon:CustomHubCheckbox(parent,"Notify me",-36,function()return db().notifyUpdates end,function(v)db().notifyUpdates=not not v end)
        local frequency=Addon:CustomHubButton(parent,"",0,-82,150,function(self)
            db().checkDays=db().checkDays==1 and 7 or 1
            self:SetText(db().checkDays==1 and "Daily" or "Weekly")
        end)
        frequency:SetScript("OnShow",function(self)self:SetText(db().checkDays==1 and "Daily" or "Weekly")end)
        saveButton(parent)
    end})
