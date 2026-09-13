local _, Addon = ...

local profileVersion = 1
local maximumHistory = 10
local maximumProfileNameLength = 48
local maximumDestinations = 512

local function copyValue(value, seen)
    if type(value) ~= "table" then return value end
    seen = seen or {}
    if seen[value] then return seen[value] end
    local result = {}
    seen[value] = result
    for key, child in pairs(value) do
        result[copyValue(key, seen)] = copyValue(child, seen)
    end
    return result
end

local function trim(value)
    value = tostring(value or "")
    if strtrim then return strtrim(value) end
    return value:match("^%s*(.-)%s*$") or ""
end

local function normalizeProfileName(value)
    value = tostring(value or "")
    if value:find("|", 1, true) then return nil end
    local name = trim(value):gsub("[%c]", " "):gsub("%s+", " ")
    if #name > maximumProfileNameLength then return nil end
    return name ~= "" and name or nil
end

local function sortedProfiles(db)
    local result = {}
    for _, profile in pairs(db.destinationProfiles or {}) do
        if type(profile) == "table" then table.insert(result, profile) end
    end
    table.sort(result, function(left, right)
        local leftName = tostring(left.name or left.id or ""):lower()
        local rightName = tostring(right.name or right.id or ""):lower()
        if leftName == rightName then return tostring(left.id) < tostring(right.id) end
        return leftName < rightName
    end)
    return result
end

local function profileNameExists(db, name, exceptID)
    local normalized = tostring(name or ""):lower()
    for id, profile in pairs(db.destinationProfiles or {}) do
        if id ~= exceptID and tostring(profile.name or ""):lower() == normalized then
            return true
        end
    end
    return false
end

local function uniqueProfileName(db, preferred, exceptID)
    local base = normalizeProfileName(preferred) or "Action Profile"
    if not profileNameExists(db, base, exceptID) then return base end
    local suffix = 2
    while profileNameExists(db, base .. " (" .. suffix .. ")", exceptID) do
        suffix = suffix + 1
    end
    return base .. " (" .. suffix .. ")"
end

local function nextProfileID(db)
    local sequence = tonumber(db.destinationProfileSequence) or 0
    local id
    repeat
        sequence = sequence + 1
        id = "profile:" .. sequence
    until not (db.destinationProfiles and db.destinationProfiles[id])
    db.destinationProfileSequence = sequence
    return id
end

local function copyScalar(value)
    local valueType = type(value)
    if valueType == "string" or valueType == "number" or valueType == "boolean" then
        return value
    end
end

local function validString(value, maximumLength, allowPipe)
    return type(value) == "string" and value ~= "" and #value <= maximumLength
        and not value:find("%c") and (allowPipe or not value:find("|", 1, true))
end

local function validPortableOriginID(value)
    return type(value) == "string" and value ~= "" and #value <= 100
        and value:match("^[%w:_%-]+$") ~= nil
end

local displayKey

local function managedBindingCommand(addon, command)
    if addon.IsManagedBindingCommand then return addon:IsManagedBindingCommand(command) end
    if not validString(command, 500) then return false end
    local mainButton = tonumber(command:match("^ACTIONBUTTON(%d+)$"))
    if mainButton then return mainButton >= 1 and mainButton <= 12 end
    local bar, button = command:match("^MULTIACTIONBAR(%d+)BUTTON(%d+)$")
    if bar and button then
        bar, button = tonumber(bar), tonumber(button)
        return bar >= 1 and bar <= 7 and button >= 1 and button <= 12
    end
    return command:match("^SPELL .+") ~= nil or command:match("^CLICK [%w_]+:[^%s]+$") ~= nil
end

local function isActionBarBindingCommand(command)
    return type(command) == "string" and (
        command:match("^ACTIONBUTTON%d+$") ~= nil
        or command:match("^MULTIACTIONBAR%d+BUTTON%d+$") ~= nil
    )
end

local function clickDestinationInfo(addon, inputKey)
    if addon.ParseClickBindingInputKey then
        local button, modifiers = addon:ParseClickBindingInputKey(inputKey)
        if button then return button, modifiers end
    end
end

local function inspectDestination(addon, destination)
    local button = destination.clickButton
    if destination.kind == "click-binding" or button then
        return addon.InspectClickBinding and addon:InspectClickBinding(destination.inputKey) or nil
    end
    return addon:InspectKey(destination.inputKey)
end

local function sanitizeUIProfile(value)
    value = type(value) == "table" and value or {}
    local actionBars = tonumber(value.multiActionBars)
    if not actionBars or actionBars < 0 or actionBars > 255 or actionBars % 1 ~= 0 then
        actionBars = nil
    end
    local function binaryCVar(setting)
        setting = setting ~= nil and tostring(setting) or nil
        return (setting == "0" or setting == "1") and setting or nil
    end
    local function castModifier(setting)
        return (setting == "NONE" or setting == "ALT" or setting == "CTRL"
            or setting == "SHIFT") and setting or nil
    end
    return {
        editModeLayoutName = validString(value.editModeLayoutName, 200)
            and value.editModeLayoutName or nil,
        editModeLayoutString = type(value.editModeLayoutString) == "string"
            and #value.editModeLayoutString <= 750000 and value.editModeLayoutString or nil,
        multiActionBars = actionBars and tostring(actionBars) or nil,
        cooldownViewerLayout = type(value.cooldownViewerLayout) == "string"
            and #value.cooldownViewerLayout <= 200000 and value.cooldownViewerLayout or nil,
        enableMouseoverCast = binaryCVar(value.enableMouseoverCast),
        mouseoverCastModifier = castModifier(value.mouseoverCastModifier),
        autoSelfCast = binaryCVar(value.autoSelfCast),
        selfCastModifier = castModifier(value.selfCastModifier),
        focusCastModifier = castModifier(value.focusCastModifier),
    }
end

local function sanitizeDestinations(addon, value)
    local result, seenIDs, seenInputs, seenActionBarCommands = {}, {}, {}, {}
    for _, destination in ipairs(type(value) == "table" and value or {}) do
        if #result >= maximumDestinations then break end
        if type(destination) == "table"
            and validString(destination.id, 200, true)
            and validString(destination.inputKey, 100, true)
            and not seenIDs[destination.id] and not seenInputs[destination.inputKey]
        then
            local clickButton, clickModifiers = clickDestinationInfo(addon, destination.inputKey)
            local malformedClickInput = destination.inputKey:sub(1, 10) == "CLICKCAST:"
                and not clickButton
            local bindingCommand = destination.bindingCommand
            if clickButton then bindingCommand = nil end
            if bindingCommand ~= nil and not managedBindingCommand(addon, bindingCommand) then
                bindingCommand = nil
            end
            local duplicateActionBarCommand = isActionBarBindingCommand(bindingCommand)
                and seenActionBarCommands[bindingCommand]
            if not malformedClickInput and not duplicateActionBarCommand then
                seenIDs[destination.id], seenInputs[destination.inputKey] = true, true
                if isActionBarBindingCommand(bindingCommand) then
                    seenActionBarCommands[bindingCommand] = true
                end
                table.insert(result, {
                    id = destination.id,
                    key = destination.id,
                    inputKey = destination.inputKey,
                    label = validString(destination.label, 200) and destination.label
                        or (clickButton and addon:FormatClickBindingInput(clickButton, clickModifiers))
                        or displayKey(destination.inputKey),
                    category = validString(destination.category, 200) and destination.category or nil,
                    bindingCommand = bindingCommand,
                    kind = clickButton and "click-binding" or nil,
                    clickButton = clickButton,
                    clickModifiers = clickModifiers,
                    bindingCategory = validString(destination.bindingCategory, 200)
                        and destination.bindingCategory or nil,
                    source = validString(destination.source, 50) and destination.source or nil,
                    legacyButton = type(destination.legacyButton) == "number"
                        and destination.legacyButton > 0 and destination.legacyButton % 1 == 0
                        and destination.legacyButton or nil,
                    legacyLayer = validString(destination.legacyLayer, 50)
                        and destination.legacyLayer or nil,
                    order = #result + 1,
                })
            end
        end
    end
    return result
end

displayKey = function(key)
    local result = tostring(key or "")
    result = result:gsub("CTRL%-", "Ctrl+")
    result = result:gsub("SHIFT%-", "Shift+")
    result = result:gsub("ALT%-", "Alt+")
    result = result:gsub("META%-", "Meta+")
    return result
end

local function countEntries(value)
    local count = 0
    for _ in pairs(value or {}) do
        count = count + 1
    end
    return count
end

local function newestStoredSnapshot(db)
    local newest
    for _, character in pairs(type(db.characters) == "table" and db.characters or {}) do
        local snapshot = type(character) == "table" and character.latestSnapshot
        if snapshot and (not newest or (snapshot.capturedAt or 0) > (newest.capturedAt or 0)) then
            newest = snapshot
        end
    end
    return newest
end

local function addDestination(result, seenInputs, destination)
    local inputKey = destination.inputKey
    if type(inputKey) ~= "string" or inputKey == "" or seenInputs[inputKey] then
        return
    end
    seenInputs[inputKey] = true
    destination.id = destination.id or ("input:" .. inputKey)
    destination.key = destination.id
    destination.label = destination.label or displayKey(inputKey)
    destination.order = #result + 1
    table.insert(result, destination)
end

local function migrateLegacySnapshot(addon, snapshot)
    local result, seenInputs = {}, {}
    local buttonNumbers = {}
    for buttonNumber in pairs(snapshot and snapshot.buttons or {}) do
        table.insert(buttonNumbers, buttonNumber)
    end
    table.sort(buttonNumbers, function(left, right) return tonumber(left) < tonumber(right) end)
    for _, buttonNumber in ipairs(buttonNumbers) do
        local buttonData = snapshot.buttons[buttonNumber]
        local number = tonumber(buttonNumber) or buttonData.number
        for _, layer in ipairs({ "base", "shift" }) do
            if layer == "base" or (number and number >= 9) then
                local action = buttonData and buttonData[layer]
                if action and action.key then
                    addDestination(result, seenInputs, {
                        id = layer .. ":" .. tostring(number),
                        inputKey = action.key,
                        label = layer == "shift" and ("Shift+" .. tostring(number)) or tostring(number),
                        category = buttonData.role,
                        bindingCommand = action.bindingCommand,
                        source = "legacy-import",
                        legacyButton = number,
                        legacyLayer = layer,
                    })
                end
            end
        end
    end

    local model = addon.db and addon.db.trainingModel
    local destinationIDs = {}
    for _, rule in pairs(model and model.preferredRules or {}) do
        if rule.destination then
            destinationIDs[rule.destination] = true
        end
    end
    for _, node in pairs(model and model.actionWeights or {}) do
        for destinationID in pairs(node.global or {}) do
            destinationIDs[destinationID] = true
        end
    end
    for destinationID in pairs(destinationIDs) do
        local inputKey = destinationID:match("^special:(.+)$")
        if inputKey then
            addDestination(result, seenInputs, {
                id = destinationID,
                inputKey = inputKey,
                label = displayKey(inputKey),
                bindingCommand = GetBindingAction and GetBindingAction(inputKey) or nil,
                source = "legacy-import",
            })
        end
    end
    return result
end

local function getBindingValues(index)
    if not GetBinding then
        return nil
    end
    local values = { GetBinding(index) }
    if type(values[1]) ~= "string" then
        return nil
    end
    return values
end

local function discoverBoundDestinations(addon)
    local result, seenInputs, seenActionBarCommands = {}, {}, {}
    local bindingCount = GetNumBindings and GetNumBindings() or 0
    for index = 1, bindingCount do
        local values = getBindingValues(index)
        if values then
            local command = values[1]
            local category = values[2]
            for valueIndex = 3, #values do
                local inputKey = values[valueIndex]
                if type(inputKey) == "string" and inputKey ~= "" and not seenInputs[inputKey] then
                    local inspected = addon:InspectKey(inputKey)
                    if managedBindingCommand(addon, command)
                        and inspected and (inspected.actionSlot or inspected.actionType == "spell")
                        and not (isActionBarBindingCommand(command)
                            and seenActionBarCommands[command])
                    then
                        addDestination(result, seenInputs, {
                            inputKey = inputKey,
                            label = displayKey(inputKey),
                            bindingCommand = command,
                            bindingCategory = copyScalar(category),
                            source = "detected",
                        })
                        if isActionBarBindingCommand(command) then
                            seenActionBarCommands[command] = true
                        end
                    end
                end
            end
        end
    end
    local spellClickType = Enum and Enum.ClickBindingType and Enum.ClickBindingType.Spell or 1
    local clickProfile, clickProfileError
    if addon.GetClickBindingProfile then
        clickProfile, clickProfileError = addon:GetClickBindingProfile()
    end
    for _, binding in ipairs(clickProfile or {}) do
        if binding.type == spellClickType then
            local inputKey = addon:GetClickBindingInputKey(binding.button, binding.modifiers)
            if inputKey and not seenInputs[inputKey] then
                addDestination(result, seenInputs, {
                    inputKey = inputKey,
                    label = addon:FormatClickBindingInput(binding.button, binding.modifiers),
                    kind = "click-binding",
                    clickButton = binding.button,
                    clickModifiers = binding.modifiers,
                    source = "click-bindings",
                })
            end
        end
    end
    table.sort(result, function(left, right)
        local leftAction = inspectDestination(addon, left)
        local rightAction = inspectDestination(addon, right)
        local leftSlot = leftAction and leftAction.actionSlot
        local rightSlot = rightAction and rightAction.actionSlot
        if leftSlot and rightSlot and leftSlot ~= rightSlot then
            return leftSlot < rightSlot
        elseif leftSlot ~= rightSlot then
            return leftSlot ~= nil
        end
        return left.label < right.label
    end)
    for index, destination in ipairs(result) do
        destination.order = index
    end
    return result, clickProfileError
end

local function captureUIProfile(addon, snapshot)
    local editMode = snapshot and snapshot.audit and snapshot.audit.editMode
        or (addon.GetEditModeAudit and addon:GetEditModeAudit())
    local actionBars = snapshot and snapshot.audit and snapshot.audit.actionBars
        or (addon.GetActionBarAudit and addon:GetActionBarAudit())
    local layoutName = editMode and editMode.activeName
    if layoutName == "Unknown" or (editMode and editMode.available == false) then
        layoutName = nil
    end
    local result = {
        editModeLayoutName = layoutName,
        multiActionBars = actionBars and actionBars.current,
    }
    local function readCVar(name)
        if type(GetCVar) ~= "function" then return nil end
        local ok, value = pcall(GetCVar, name)
        value = ok and value ~= nil and tostring(value) or nil
        return (value == "0" or value == "1") and value or nil
    end
    local function readModifiedClick(name, defaultValue)
        if type(GetModifiedClick) ~= "function" then return nil end
        local ok, value = pcall(GetModifiedClick, name)
        if not ok then return nil end
        value = value or defaultValue
        if value == "NONE" or value == "ALT" or value == "CTRL" or value == "SHIFT" then
            return value
        end
    end
    result.enableMouseoverCast = readCVar("enableMouseoverCast")
    result.mouseoverCastModifier = readModifiedClick("MOUSEOVERCAST", "NONE")
    result.autoSelfCast = readCVar("autoSelfCast")
    result.selfCastModifier = readModifiedClick("SELFCAST", "ALT")
    result.focusCastModifier = readModifiedClick("FOCUSCAST", "NONE")
    if C_CooldownViewer and C_CooldownViewer.GetLayoutData then
        local ok, layoutData = pcall(C_CooldownViewer.GetLayoutData)
        if ok and type(layoutData) == "string" then
            result.cooldownViewerLayout = layoutData
        end
    end
    if result.editModeLayoutName and C_EditMode and C_EditMode.ConvertLayoutInfoToString and C_EditMode.GetLayouts then
        local ok, layouts = pcall(C_EditMode.GetLayouts)
        if ok and layouts and layouts.layouts then
            for _, layout in ipairs(layouts.layouts) do
                if layout.layoutName == result.editModeLayoutName then
                    local converted, serialized = pcall(C_EditMode.ConvertLayoutInfoToString, layout)
                    if converted and type(serialized) == "string" then
                        result.editModeLayoutString = serialized
                    end
                    break
                end
            end
        end
    end
    return result
end

local function newProfile(destinations, source, uiProfile, id, name, portableOriginID)
    return {
        version = profileVersion,
        id = id,
        name = name,
        source = source,
        capturedAt = time(),
        capturedAtText = date("%Y-%m-%d %H:%M:%S"),
        destinations = destinations,
        ui = uiProfile,
        portableOriginID = validPortableOriginID(portableOriginID) and portableOriginID or nil,
    }
end

function Addon:InitializeDestinationProfile()
    self.db.destinationProfiles = type(self.db.destinationProfiles) == "table"
        and self.db.destinationProfiles or {}

    -- Convert the pre-0.13 single profile without changing its destinations or
    -- learned destination identifiers.
    if type(self.db.destinationProfile) == "table" and next(self.db.destinationProfiles) == nil then
        local legacy = self.db.destinationProfile
        legacy.id = legacy.id or "local-action-map"
        legacy.name = normalizeProfileName(legacy.name) or "Default"
        legacy.version = profileVersion
        legacy.destinations = sanitizeDestinations(self, legacy.destinations)
        legacy.ui = sanitizeUIProfile(legacy.ui or captureUIProfile(self))
        self.db.destinationProfiles[legacy.id] = legacy
        self.db.activeProfile = legacy.id
    end
    self.db.destinationProfile = nil

    for id, profile in pairs(self.db.destinationProfiles) do
        if type(id) ~= "string" or type(profile) ~= "table" then
            self.db.destinationProfiles[id] = nil
        else
            profile.id = id
            profile.version = profileVersion
            profile.name = uniqueProfileName(self.db, profile.name or "Action Profile", id)
            profile.destinations = sanitizeDestinations(self, profile.destinations)
            profile.ui = type(profile.ui) == "table" and sanitizeUIProfile(profile.ui) or nil
            profile.portableOriginID = validPortableOriginID(profile.portableOriginID)
                and profile.portableOriginID or nil
        end
    end

    if next(self.db.destinationProfiles) == nil then
        local storedSnapshot = newestStoredSnapshot(self.db)
        local destinations = storedSnapshot and migrateLegacySnapshot(self, storedSnapshot) or {}
        local source = "legacy-import"
        if #destinations == 0 then
            destinations = discoverBoundDestinations(self)
            source = "detected"
        end
        local legacyLayoutName = storedSnapshot and storedSnapshot.audit
            and storedSnapshot.audit.editMode and storedSnapshot.audit.editMode.activeName
        if legacyLayoutName == "Unknown" then legacyLayoutName = nil end
        local legacyUI = storedSnapshot and storedSnapshot.audit and {
            editModeLayoutName = legacyLayoutName,
            multiActionBars = storedSnapshot.audit.actionBars and storedSnapshot.audit.actionBars.current,
        } or nil
        local id = nextProfileID(self.db)
        self.db.destinationProfiles[id] = newProfile(
            destinations, source, legacyUI or captureUIProfile(self), id, "Default")
        self.db.activeProfile = id
    end

    local profiles = sortedProfiles(self.db)
    local active = self.db.destinationProfiles[self.db.activeProfile]
    if not active then
        active = profiles[1]
        self.db.activeProfile = active and active.id or nil
    end
    active.version = profileVersion
    active.name = normalizeProfileName(active.name) or "Default"
    active.destinations = sanitizeDestinations(self, active.destinations)
    if not active.ui then
        active.ui = sanitizeUIProfile(captureUIProfile(self))
    else
        active.ui = sanitizeUIProfile(active.ui)
    end
    self.db.destinationProfileHistory = type(self.db.destinationProfileHistory) == "table"
        and self.db.destinationProfileHistory or {}
    self.destinationProfile = active
    return self.destinationProfile
end

function Addon:GetDestinationProfiles()
    self:InitializeDestinationProfile()
    return sortedProfiles(self.db)
end

function Addon:GetActiveDestinationProfile()
    return self.destinationProfile or self:InitializeDestinationProfile()
end

function Addon:GetUniqueDestinationProfileName(preferred)
    self:InitializeDestinationProfile()
    return uniqueProfileName(self.db, preferred)
end

local function clearProfileDerivedState(addon)
    addon.trainingRecommendations = nil
    addon.trainingCoverage = nil
    addon.layoutPlan = nil
end

local function refreshAfterProfileChange(addon, reason)
    clearProfileDerivedState(addon)
    if addon.CaptureSnapshot then
        addon:CaptureSnapshot(reason)
    elseif addon.RefreshUI then
        addon:RefreshUI()
    end
end

function Addon:ActivateDestinationProfile(profileID)
    if InCombatLockdown and InCombatLockdown() then
        self:SetStatus("Cannot switch profiles during combat")
        return false
    end
    self:InitializeDestinationProfile()
    local profile = self.db.destinationProfiles[profileID]
    if not profile then
        self:SetStatus("That profile is no longer available")
        return false
    end
    if self.db.activeProfile == profileID then return true, profile end
    self.db.activeProfile = profileID
    self.destinationProfile = profile
    refreshAfterProfileChange(self, "action profile selected")
    self:SetStatus("Selected profile: " .. profile.name)
    return true, profile
end

function Addon:CycleDestinationProfile(direction)
    local profiles = self:GetDestinationProfiles()
    if #profiles < 2 then return true, profiles[1] end
    local activeIndex = 1
    for index, profile in ipairs(profiles) do
        if profile.id == self.db.activeProfile then activeIndex = index break end
    end
    local nextIndex = ((activeIndex - 1 + (direction or 1)) % #profiles) + 1
    return self:ActivateDestinationProfile(profiles[nextIndex].id)
end

local function recordProfileChange(addon, kind, includeTrainingModel)
    if addon.PushProfileChange then
        addon:PushProfileChange(kind, includeTrainingModel and true or false)
        return
    end
    addon.db.destinationProfileHistory = addon.db.destinationProfileHistory or {}
    table.insert(addon.db.destinationProfileHistory, {
        kind = kind,
        destinationProfiles = copyValue(addon.db.destinationProfiles),
        activeProfile = addon.db.activeProfile,
    })
    while #addon.db.destinationProfileHistory > maximumHistory do
        table.remove(addon.db.destinationProfileHistory, 1)
    end
end

function Addon:InstallDestinationProfile(profile, preferredName)
    self:InitializeDestinationProfile()
    local id = nextProfileID(self.db)
    profile.id = id
    profile.name = uniqueProfileName(self.db, preferredName or profile.name)
    profile.version = profileVersion
    profile.destinations = sanitizeDestinations(self, profile.destinations)
    profile.ui = sanitizeUIProfile(profile.ui)
    profile.portableOriginID = validPortableOriginID(profile.portableOriginID)
        and profile.portableOriginID or nil
    self.db.destinationProfiles[id] = profile
    self.db.activeProfile = id
    self.destinationProfile = profile
    clearProfileDerivedState(self)
    return profile
end

function Addon:CreateDestinationProfile(name)
    if InCombatLockdown and InCombatLockdown() then
        self:SetStatus("Cannot create a profile during combat")
        return false
    end
    self:InitializeDestinationProfile()
    name = normalizeProfileName(name)
    if not name then
        self:SetStatus("Enter a profile name")
        return false
    end
    if profileNameExists(self.db, name) then
        self:SetStatus("A profile with that name already exists")
        return false
    end
    local destinations, discoveryError = discoverBoundDestinations(self)
    if discoveryError and C_ClickBindings and type(C_ClickBindings.GetProfileInfo) == "function" then
        self:SetStatus(discoveryError .. " • Profile capture stopped without changes")
        return false
    end
    if #destinations == 0 then
        self:SetStatus("No action-bar, direct-spell, or Click Casting inputs were detected")
        return false
    end
    recordProfileChange(self, "create")
    local id = nextProfileID(self.db)
    local profile = newProfile(destinations, "captured", captureUIProfile(self), id, name)
    self.db.destinationProfiles[id] = profile
    self.db.activeProfile = id
    self.destinationProfile = profile
    refreshAfterProfileChange(self, "action profile created")
    self:SetStatus("Created " .. name .. " with " .. #destinations .. " inputs")
    return true, profile
end

function Addon:RenameDestinationProfile(name)
    if InCombatLockdown and InCombatLockdown() then
        self:SetStatus("Cannot rename a profile during combat")
        return false
    end
    local profile = self:GetActiveDestinationProfile()
    name = normalizeProfileName(name)
    if not name then self:SetStatus("Enter a profile name") return false end
    if profileNameExists(self.db, name, profile.id) then
        self:SetStatus("A profile with that name already exists")
        return false
    end
    if profile.name == name then return true, profile end
    recordProfileChange(self, "rename")
    profile.name = name
    if self.RefreshUI then self:RefreshUI() end
    self:SetStatus("Renamed profile to " .. name)
    return true, profile
end

function Addon:DeleteDestinationProfile()
    if InCombatLockdown and InCombatLockdown() then
        self:SetStatus("Cannot delete a profile during combat")
        return false
    end
    local profiles = self:GetDestinationProfiles()
    if #profiles <= 1 then
        self:SetStatus("Keep at least one profile")
        return false
    end
    local removed = self:GetActiveDestinationProfile()
    local transaction = self.db.layoutTransactions and self.db.layoutTransactions.active
    if transaction and transaction.context and transaction.context.profileID == removed.id then
        self:SetStatus("Undo or finish the active layout before deleting this profile")
        return false
    end
    recordProfileChange(self, "delete", true)
    self.db.destinationProfiles[removed.id] = nil
    if self.RemoveTrainingProfileEvidence then
        self:RemoveTrainingProfileEvidence(removed.id)
    end
    local remaining = sortedProfiles(self.db)[1]
    self.db.activeProfile = remaining.id
    self.destinationProfile = remaining
    refreshAfterProfileChange(self, "action profile deleted")
    self:SetStatus("Deleted profile: " .. removed.name)
    return true, removed
end

function Addon:GetManagedDestinations()
    local profile = self.destinationProfile or self:InitializeDestinationProfile()
    return profile.destinations or {}
end

function Addon:GetDestinationSnapshot()
    local result = {}
    for _, destination in ipairs(self:GetManagedDestinations()) do
        local action = inspectDestination(self, destination) or { actionName = "Unavailable" }
        action.id = destination.id
        action.destinationKey = destination.id
        action.destinationLabel = destination.label
        action.inputKey = destination.inputKey
        action.category = destination.category
        action.expectedBindingCommand = destination.bindingCommand
        result[destination.id] = action
    end
    return result
end

function Addon:GetDestinationProfileSummary()
    local profile = self.destinationProfile or self:InitializeDestinationProfile()
    local readable = 0
    for _, destination in ipairs(profile.destinations or {}) do
        local action = inspectDestination(self, destination)
        if action and (action.actionSlot or action.actionType) then
            readable = readable + 1
        end
    end
    return {
        name = profile.name,
        source = profile.source,
        destinations = #(profile.destinations or {}),
        readable = readable,
        learnedContexts = countEntries(self.db.trainingModel and self.db.trainingModel.contextSetups),
    }
end

function Addon:CaptureCurrentDestinationProfile()
    if InCombatLockdown and InCombatLockdown() then
        self:SetStatus("Cannot capture a profile during combat")
        return false
    end
    local destinations, discoveryError = discoverBoundDestinations(self)
    if discoveryError and C_ClickBindings and type(C_ClickBindings.GetProfileInfo) == "function" then
        self:SetStatus(discoveryError .. " • Profile capture stopped without changes")
        return false
    end
    if #destinations == 0 then
        self:SetStatus("No action-bar, direct-spell, or Click Casting inputs were detected")
        return false
    end

    local previous = self:GetActiveDestinationProfile()
    local previousByInput = {}
    for _, destination in ipairs(previous and previous.destinations or {}) do
        previousByInput[destination.inputKey] = destination
    end
    for _, destination in ipairs(destinations) do
        local prior = previousByInput[destination.inputKey]
        if prior then
            destination.id = prior.id
            destination.key = prior.id
            destination.label = prior.label
            destination.category = prior.category
            destination.legacyButton = prior.legacyButton
            destination.legacyLayer = prior.legacyLayer
        end
    end
    recordProfileChange(self, "update")
    local profile = newProfile(destinations, "captured", captureUIProfile(self), previous.id,
        previous.name, previous.portableOriginID)
    self.db.destinationProfiles[profile.id] = profile
    self.destinationProfile = profile
    clearProfileDerivedState(self)
    self:CaptureSnapshot("profile captured")
    self:SetStatus("Updated " .. profile.name .. " with " .. #destinations .. " inputs")
    return true, profile
end

function Addon:InitializeDestinationPopups()
    if not StaticPopupDialogs or StaticPopupDialogs.LAMDAUI_CAPTURE_ACTION_MAP then
        return
    end
    StaticPopupDialogs.LAMDAUI_CAPTURE_ACTION_MAP = {
        text = "Capture the current controls and Blizzard UI settings in %s?\n\nThis replaces the profile's saved inputs, Edit Mode layout, visible bars, and Cooldown Manager setup. Learned placements stay intact.",
        button1 = "Capture",
        button2 = CANCEL,
        OnAccept = function()
            Addon:CaptureCurrentDestinationProfile()
        end,
        timeout = 0,
        whileDead = true,
        hideOnEscape = true,
        preferredIndex = 3,
    }
    StaticPopupDialogs.LAMDAUI_NEW_ACTION_PROFILE = {
        text = "Create a profile from the current controls and Blizzard UI settings.\n\nEnter a name:",
        button1 = "Create",
        button2 = CANCEL,
        hasEditBox = true,
        OnShow = function(dialog)
            dialog.editBox:SetMaxLetters(maximumProfileNameLength)
            dialog.editBox:SetText("")
            dialog.editBox:SetFocus()
        end,
        OnAccept = function(dialog) Addon:CreateDestinationProfile(dialog.editBox:GetText()) end,
        EditBoxOnEnterPressed = function(editBox)
            if Addon:CreateDestinationProfile(editBox:GetText()) then editBox:GetParent():Hide() end
        end,
        EditBoxOnEscapePressed = function(editBox) editBox:GetParent():Hide() end,
        timeout = 0,
        whileDead = true,
        hideOnEscape = true,
        preferredIndex = 3,
    }
    StaticPopupDialogs.LAMDAUI_RENAME_ACTION_PROFILE = {
        text = "Rename the active profile:\n\nEnter a name:",
        button1 = "Rename",
        button2 = CANCEL,
        hasEditBox = true,
        OnShow = function(dialog)
            local profile = Addon:GetActiveDestinationProfile()
            dialog.editBox:SetMaxLetters(maximumProfileNameLength)
            dialog.editBox:SetText(profile and profile.name or "")
            dialog.editBox:HighlightText()
            dialog.editBox:SetFocus()
        end,
        OnAccept = function(dialog) Addon:RenameDestinationProfile(dialog.editBox:GetText()) end,
        EditBoxOnEnterPressed = function(editBox)
            if Addon:RenameDestinationProfile(editBox:GetText()) then editBox:GetParent():Hide() end
        end,
        EditBoxOnEscapePressed = function(editBox) editBox:GetParent():Hide() end,
        timeout = 0,
        whileDead = true,
        hideOnEscape = true,
        preferredIndex = 3,
    }
    StaticPopupDialogs.LAMDAUI_DELETE_ACTION_PROFILE = {
        text = "Delete %s?\n\nShared learning stays available, but this profile's controls and UI settings will be removed. Undo Change can restore it.",
        button1 = DELETE,
        button2 = CANCEL,
        OnAccept = function() Addon:DeleteDestinationProfile() end,
        timeout = 0,
        whileDead = true,
        hideOnEscape = true,
        preferredIndex = 3,
    }
end

function Addon:ConfirmCaptureDestinationProfile()
    if InCombatLockdown and InCombatLockdown() then
        self:SetStatus("Cannot capture a profile during combat")
        return
    end
    self:InitializeDestinationPopups()
    if StaticPopup_Show then
        local profile = self:GetActiveDestinationProfile()
        StaticPopup_Show("LAMDAUI_CAPTURE_ACTION_MAP", profile and profile.name or "this profile")
    end
end

function Addon:ShowNewDestinationProfile()
    self:InitializeDestinationPopups()
    if StaticPopup_Show then StaticPopup_Show("LAMDAUI_NEW_ACTION_PROFILE") end
end

function Addon:ShowRenameDestinationProfile()
    self:InitializeDestinationPopups()
    if StaticPopup_Show then StaticPopup_Show("LAMDAUI_RENAME_ACTION_PROFILE") end
end

function Addon:ConfirmDeleteDestinationProfile()
    self:InitializeDestinationPopups()
    local profile = self:GetActiveDestinationProfile()
    if StaticPopup_Show and profile then
        StaticPopup_Show("LAMDAUI_DELETE_ACTION_PROFILE", profile.name)
    end
end
