local _, Addon = ...

-- Each custom feature registers here; this is independent of action profiles.
Addon.customModules = {}
local byID = {}

function Addon:RegisterCustomModule(module)
    assert(type(module.id) == "string" and not byID[module.id], "Duplicate custom module")
    byID[module.id] = module
    table.insert(self.customModules, module)
end

function Addon:GetCustomModuleSettings(id)
    if not self.db then return nil end
    local root = self.db.customModules
    if type(root) ~= "table" then root = {}; self.db.customModules = root end
    if type(root[id]) ~= "table" then root[id] = {} end
    return root[id]
end

function Addon:IsCustomModuleEnabled(id)
    if self.legacyConflict or not self.db then return false end
    local state = self:GetCustomModuleSettings(id)
    return state.enabled ~= false
end

function Addon:SetCustomModuleEnabled(id, enabled)
    local state = self:GetCustomModuleSettings(id)
    if not state then return end
    state.enabled = enabled and true or false
    local module = byID[id]
    if module and module.changed then module.changed() end
    self:RefreshCustomHub()
end

local function copy(value)
    if type(value) ~= "table" then return value end
    local result = {}
    for k, v in pairs(value) do result[k] = copy(v) end
    return result
end

local function loadLegacyData()
    if type(DevouringDevMacrosDB) == "table" then return true end
    if not C_AddOns or not C_AddOns.GetAddOnMetadata then return true end
    local ok, version = pcall(C_AddOns.GetAddOnMetadata, "DevouringDevMacros", "Version")
    if not ok or not version or version == "" then return true end
    -- Only load our data-only bridge, never reactivate the retired installer.
    if version ~= "1.2.0-lamdaui-bridge" or InCombatLockdown() then return false end
    local character = UnitName("player")
    if not character or not C_AddOns.GetAddOnEnableState or not C_AddOns.LoadAddOn
        or not C_AddOns.EnableAddOn or not C_AddOns.DisableAddOn then return false end
    local state = C_AddOns.GetAddOnEnableState("DevouringDevMacros", character)
    if state == 0 then C_AddOns.EnableAddOn("DevouringDevMacros", character) end
    local loadedOK, loaded = pcall(C_AddOns.LoadAddOn, "DevouringDevMacros")
    if state == 0 then C_AddOns.DisableAddOn("DevouringDevMacros", character) end
    return loadedOK and loaded == true
end

function Addon:InitializeCustomModules()
    local macro = self:GetCustomModuleSettings("devastationMacros")
    if not macro.migrated then
        self.customMigrationPending = not loadLegacyData()
    end
    if not macro.migrated and not self.customMigrationPending then
        local previous = type(DevouringDevMacrosDB) == "table" and DevouringDevMacrosDB or {}
        macro.state = type(macro.state) == "table" and macro.state or {}
        for _, key in ipairs({"installVersion", "disabled", "replacedSlots", "replacedCount"}) do
            macro.state[key] = copy(previous[key])
        end
        macro.migrated = true
        local aug = self:GetCustomModuleSettings("augmentation")
        if aug.enabled == nil and type(previous.ebonMightMissingAlert) == "table" then
            aug.enabled = previous.ebonMightMissingAlert.enabled
        end
    end
    for _, module in ipairs(self.customModules) do
        local db = self:GetCustomModuleSettings(module.id)
        for key, value in pairs(module.defaults or {}) do
            if db[key] == nil then db[key] = copy(value) end
        end
        if module.initialize then module.initialize() end
    end
end

local migrationDriver = CreateFrame("Frame")
migrationDriver:RegisterEvent("PLAYER_REGEN_ENABLED")
migrationDriver:RegisterEvent("PLAYER_LOGIN")
migrationDriver:SetScript("OnEvent", function()
    if Addon.db and not Addon.legacyConflict and Addon.customMigrationPending then
        Addon:InitializeCustomModules()
    end
end)

function Addon:CustomHubText(parent, text, x, y, width, large)
    local label = parent:CreateFontString(nil, "OVERLAY", large and "GameFontNormalLarge" or "GameFontHighlight")
    label:SetPoint("TOPLEFT", x, y)
    label:SetWidth(width or 500)
    label:SetJustifyH("LEFT")
    label:SetText(text)
    return label
end

function Addon:CustomHubButton(parent, text, x, y, width, callback)
    local button = CreateFrame("Button", nil, parent, "UIPanelButtonTemplate")
    button:SetSize(width or 125, 28)
    button:SetPoint("TOPLEFT", x, y)
    button:SetText(text)
    button:SetScript("OnClick", callback)
    return button
end

function Addon:CustomHubCheckbox(parent, text, y, getValue, setValue)
    local button = CreateFrame("CheckButton", nil, parent, "UICheckButtonTemplate")
    button:SetSize(26, 26)
    button:SetPoint("TOPLEFT", 0, y)
    local label = button:CreateFontString(nil, "OVERLAY", "GameFontHighlight")
    label:SetPoint("LEFT", button, "RIGHT", 4, 0)
    label:SetText(text)
    button.refresh = function() button:SetChecked(getValue()) end
    button:SetScript("OnClick", function() setValue(button:GetChecked()); button.refresh() end)
    button.refresh()
    return button
end

function Addon:CreateCustomHubPanel(parent)
    local panel = CreateFrame("Frame", nil, parent)
    panel:SetAllPoints()
    panel:Hide()
    self.customHubPanel = panel
    self:CustomHubText(panel, "Custom Addons", 18, -8, 710, true)
    self.customHubButtons = {}
    for index, module in ipairs(self.customModules) do
        local button = self:CustomHubButton(panel, module.name, 18, -70 - (index - 1) * 38, 188,
            function() self:SelectCustomModule(module.id) end)
        self.customHubButtons[module.id] = button
        local detail = CreateFrame("Frame", nil, panel)
        detail:SetPoint("TOPLEFT", 225, -70)
        detail:SetPoint("BOTTOMRIGHT", -18, 0)
        detail:Hide()
        module.panel = detail
        self:CustomHubText(detail, module.name, 0, 0, 510, true)
        module.enable = self:CustomHubCheckbox(detail, "Enabled", -38,
            function() return self:IsCustomModuleEnabled(module.id) end,
            function(value) self:SetCustomModuleEnabled(module.id, value) end)
        if module.noEnable then module.enable:Hide() end
        local controls = CreateFrame("Frame", nil, detail)
        controls:SetPoint("TOPLEFT", 0, module.noEnable and -38 or -80)
        controls:SetPoint("BOTTOMRIGHT")
        if module.buildOptions then module.buildOptions(controls) end
    end
    panel:SetScript("OnShow", function() self:RefreshCustomHub() end)
    self:SelectCustomModule(self.selectedCustomModule or "lamdaCD")
end

function Addon:SelectCustomModule(id)
    if not byID[id] then id = self.customModules[1] and self.customModules[1].id end
    self.selectedCustomModule = id
    self:RefreshCustomHub()
end

function Addon:RefreshCustomHub()
    for _, module in ipairs(self.customModules) do
        if module.panel then
            local selected = self.selectedCustomModule == module.id
            module.panel:SetShown(selected)
            module.enable.refresh()
            self.customHubButtons[module.id]:SetEnabled(not selected)
            if module.refreshOptions then module.refreshOptions() end
        end
    end
end
