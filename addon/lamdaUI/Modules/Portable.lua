local _, Addon = ...

local formatPrefix = "LAMDAUI"
local currentFormatVersion = 6
local currentFormatHeader = formatPrefix .. tostring(currentFormatVersion)
local maximumImportLength = 1000000
local maximumRecords = 50000
local maximumProfileChanges = 10
local maximumDestinations = 512
local maximumActions = 4000
local maximumRolesPerAction = 32
local maximumWeight = 1000000
local maximumImportFingerprints = 1000
local maximumImportOrigins = 1000

local function encode(value)
    return tostring(value == nil and "" or value):gsub("([^%w%-%._:])", function(character)
        return string.format("%%%02X", string.byte(character))
    end)
end

local function decode(value)
    return (value or ""):gsub("%%(%x%x)", function(hex)
        return string.char(tonumber(hex, 16))
    end)
end

local function split(value, delimiter)
    local result = {}
    local startIndex = 1
    while true do
        local delimiterIndex = value:find(delimiter, startIndex, true)
        if not delimiterIndex then
            table.insert(result, value:sub(startIndex))
            return result
        end
        table.insert(result, value:sub(startIndex, delimiterIndex - 1))
        startIndex = delimiterIndex + #delimiter
    end
end

local function legacyChecksum(value)
    local result = 0
    for index = 1, #value do
        result = (result + (value:byte(index) * index)) % 2147483647
    end
    return result
end

local function adler32(value)
    local first, second = 1, 0
    for index = 1, #value do
        first = (first + value:byte(index)) % 65521
        second = (second + first) % 65521
    end
    return (second * 65536) + first
end

local checksumByVersion = {
    [1] = legacyChecksum,
    [2] = adler32,
    [3] = adler32,
    [4] = adler32,
    [5] = adler32,
    [6] = adler32,
}

local function validPortableOriginID(value)
    return type(value) == "string" and value ~= "" and #value <= 100
        and value:match("^[%w:_%-]+$") ~= nil
end

local function ensurePortableOriginID(profile)
    if validPortableOriginID(profile.portableOriginID) then
        return profile.portableOriginID
    end
    local entropy = table.concat({
        tostring(time and time() or 0),
        tostring(GetTimePreciseSec and GetTimePreciseSec() or 0),
        tostring(profile.id or "profile"),
        tostring(math.random and math.random() or 0),
        tostring({}),
    }, ":")
    local words = { "origin" }
    for index = 1, 4 do
        local randomPart = math.random and math.random() or index
        table.insert(words, tostring(adler32(entropy .. ":" .. tostring(index) .. ":" .. tostring(randomPart))))
    end
    profile.portableOriginID = table.concat(words, "-")
    return profile.portableOriginID
end

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

local function sortedKeys(value)
    local result = {}
    if type(value) ~= "table" then
        return result
    end
    for key in pairs(value) do
        table.insert(result, key)
    end
    table.sort(result, function(left, right) return tostring(left) < tostring(right) end)
    return result
end

local function countLabel(count, singular, plural)
    count = tonumber(count) or 0
    return tostring(count) .. " " .. (count == 1 and singular or (plural or (singular .. "s")))
end

local function addRecord(records, recordType, ...)
    local fields = { recordType }
    for index = 1, select("#", ...) do
        local value = select(index, ...)
        local encoded = encode(value)
        table.insert(fields, encoded)
    end
    table.insert(records, table.concat(fields, "|"))
end

local function joinRoles(roles)
    local result = {}
    if type(roles) ~= "table" then
        return ""
    end
    for _, role in ipairs(roles) do
        if type(role) == "string" and role ~= "" and #role <= 50 then
            table.insert(result, role)
        end
    end
    table.sort(result)
    return table.concat(result, ",")
end

local function addWeightRecords(records, recordType, actionKey, parent, allowedDestinations)
    if type(actionKey) ~= "string" or type(parent) ~= "table" then
        return
    end
    for _, firstKey in ipairs(sortedKeys(parent)) do
        local value = parent[firstKey]
        if type(firstKey) == "string" and type(value) == "table" then
            for _, destination in ipairs(sortedKeys(value)) do
                local weight = value[destination]
                if type(destination) == "string" and allowedDestinations[destination]
                    and type(weight) == "number"
                    and weight == weight and math.abs(weight) <= maximumWeight
                then
                    addRecord(records, recordType, actionKey, firstKey, destination, weight)
                end
            end
        elseif type(firstKey) == "string" and allowedDestinations[firstKey]
            and type(value) == "number"
            and value == value and math.abs(value) <= maximumWeight
        then
            addRecord(records, recordType, actionKey, firstKey, value)
        end
    end
end

local function addPortableContextRecords(records, actionKey, parent, profileID, allowedDestinations)
    if type(actionKey) ~= "string" or type(parent) ~= "table" then
        return
    end
    local suffix = "|" .. tostring(profileID or "")
    for _, contextKey in ipairs(sortedKeys(parent)) do
        local portableContext
        if type(contextKey) == "string" then
            if suffix ~= "|" and contextKey:sub(-#suffix) == suffix then
                portableContext = contextKey:sub(1, #contextKey - #suffix)
            elseif not contextKey:find("|", 1, true) then
                -- Pre-named-profile evidence belongs to the profile being exported.
                portableContext = contextKey
            end
        end
        local contextWeights = parent[contextKey]
        if portableContext and type(contextWeights) == "table" then
            for _, destination in ipairs(sortedKeys(contextWeights)) do
                local weight = contextWeights[destination]
                if type(destination) == "string" and allowedDestinations[destination]
                    and type(weight) == "number"
                    and weight == weight and math.abs(weight) <= maximumWeight
                then
                    addRecord(records, "C", actionKey, portableContext, destination, weight)
                end
            end
        end
    end
end

function Addon:ExportPortableProfile()
    local profile = self.destinationProfile or self:InitializeDestinationProfile()
    local model = self:InitializeTrainingModel()
    local profileEvidence = model.profileEvidence and model.profileEvidence[profile.id] or {}
    local profileRoleWeights = model.roleWeightsByProfile and model.roleWeightsByProfile[profile.id]
    profileRoleWeights = type(profileRoleWeights) == "table" and profileRoleWeights or {}
    local profilePreferredRules = model.preferredRulesByProfile and model.preferredRulesByProfile[profile.id]
    profilePreferredRules = type(profilePreferredRules) == "table" and profilePreferredRules or {}
    local profileDestinationWeights = model.destinationWeightsByProfile
        and model.destinationWeightsByProfile[profile.id] or {}
    local records = { currentFormatHeader }
    addRecord(records, "V", currentFormatVersion, self.version)
    addRecord(records, "N", profile.name or "Action Profile")
    addRecord(records, "O", ensurePortableOriginID(profile))
    addRecord(records, "M", profileEvidence.confirmedSetups or 0, profileEvidence.placementChoices or 0)
    local uiProfile = profile.ui or {}
    addRecord(records, "U",
        uiProfile.editModeLayoutName,
        uiProfile.editModeLayoutString,
        uiProfile.multiActionBars,
        uiProfile.cooldownViewerLayout,
        uiProfile.enableMouseoverCast,
        uiProfile.mouseoverCastModifier,
        uiProfile.autoSelfCast,
        uiProfile.selfCastModifier,
        uiProfile.focusCastModifier)

    local profileDestinations = {}
    for _, destination in ipairs(type(profile.destinations) == "table" and profile.destinations or {}) do
        if type(destination) == "table" and type(destination.id) == "string"
            and destination.id ~= "" and type(destination.inputKey) == "string"
            and destination.inputKey ~= ""
        then
            profileDestinations[destination.id] = true
            addRecord(records, "D",
                destination.id,
                destination.inputKey,
                type(destination.label) == "string" and destination.label or nil,
                type(destination.category) == "string" and destination.category or nil,
                type(destination.bindingCommand) == "string" and destination.bindingCommand or nil,
                type(destination.legacyButton) == "number" and destination.legacyButton or nil,
                type(destination.legacyLayer) == "string" and destination.legacyLayer or nil)
        end
    end

    for _, actionKey in ipairs(sortedKeys(model.actionWeights)) do
        local node = model.actionWeights[actionKey]
        -- Macro indexes and bodies are account-local. Shared role weights still
        -- carry their useful placement evidence without exporting the macro.
        if type(actionKey) == "string" and type(node) == "table"
            and node.actionType == "spell" and type(node.actionID) == "number"
            and type(node.actionName) == "string" and node.actionName ~= ""
        then
            addRecord(records, "A",
                actionKey,
                node.actionType,
                node.actionID,
                node.actionName,
                joinRoles(node.roles),
                node.confirmations or 0,
                node.rejections or 0)
            addWeightRecords(records, "G", actionKey, node.global, profileDestinations)
            addWeightRecords(records, "S", actionKey, node.bySpec, profileDestinations)
            addPortableContextRecords(records, actionKey, node.byContext, profile.id, profileDestinations)
        end
    end

    for _, destination in ipairs(sortedKeys(profileDestinationWeights)) do
        local weight = profileDestinationWeights[destination]
        if type(destination) == "string" and profileDestinations[destination]
            and type(weight) == "number"
            and weight == weight and math.abs(weight) <= maximumWeight
        then
            addRecord(records, "W", destination, weight)
        end
    end

    for _, role in ipairs(sortedKeys(profileRoleWeights)) do
        local roleWeights = profileRoleWeights[role]
        if type(role) == "string" and role ~= "" and #role <= 50 and type(roleWeights) == "table" then
            for _, destination in ipairs(sortedKeys(roleWeights)) do
                local weight = roleWeights[destination]
                if type(destination) == "string" and profileDestinations[destination]
                    and type(weight) == "number"
                    and weight == weight and math.abs(weight) <= maximumWeight
                then
                    addRecord(records, "R", role, destination, weight)
                end
            end
        end
    end
    for _, role in ipairs(sortedKeys(profilePreferredRules)) do
        local rule = profilePreferredRules[role]
        if type(role) == "string" and role ~= "" and #role <= 50 and type(rule) == "table"
            and type(rule.destination) == "string" and rule.destination ~= ""
            and profileDestinations[rule.destination]
        then
            addRecord(records, "P", role, rule.destination,
                type(rule.label) == "string" and rule.label or nil,
                type(rule.source) == "string" and rule.source or nil,
                type(rule.weight) == "number" and rule.weight or 0)
        end
    end

    local body = table.concat(records, ";")
    return body .. ";Z|" .. tostring(checksumByVersion[currentFormatVersion](body))
end

local function limitedNumber(value, default)
    local number = tonumber(value)
    if number == nil then
        return default
    end
    if number ~= number or math.abs(number) > maximumWeight then
        return nil
    end
    return number
end

local function ensureAction(model, actionKey)
    model.actionWeights[actionKey] = model.actionWeights[actionKey] or {
        key = actionKey,
        global = {},
        bySpec = {},
        byContext = {},
        roles = {},
        confirmations = 0,
        rejections = 0,
    }
    return model.actionWeights[actionKey]
end

function Addon:InitializeProfileChangeHistory()
    self.db.profileChangeHistory = type(self.db.profileChangeHistory) == "table"
        and self.db.profileChangeHistory or {}

    -- Migrate the one-step import backup used by earlier releases.
    if type(self.db.profileImportBackup) == "table" then
        local backup = self.db.profileImportBackup
        table.insert(self.db.profileChangeHistory, {
            kind = "import",
            capturedAt = backup.capturedAt,
            capturedAtText = backup.capturedAtText,
            destinationProfile = copyValue(backup.destinationProfile),
            trainingModel = copyValue(backup.trainingModel),
        })
        self.db.profileImportBackup = nil
    elseif self.db.profileImportBackup ~= nil then
        self.db.profileImportBackup = nil
    end

    local validHistory = {}
    for _, backup in ipairs(self.db.profileChangeHistory) do
        if type(backup) == "table"
            and (type(backup.destinationProfiles) == "table"
                or type(backup.destinationProfile) == "table")
            and (backup.trainingModel == nil or type(backup.trainingModel) == "table")
            and (backup.portableEvidenceImports == nil
                or type(backup.portableEvidenceImports) == "table")
            and (backup.portableEvidenceOrigins == nil
                or type(backup.portableEvidenceOrigins) == "table")
        then
            table.insert(validHistory, backup)
        end
    end
    while #validHistory > maximumProfileChanges do table.remove(validHistory, 1) end
    self.db.profileChangeHistory = validHistory
    return validHistory
end

function Addon:PushProfileChange(kind, includeTrainingModel)
    local history = self:InitializeProfileChangeHistory()
    table.insert(history, {
        kind = kind or "profile",
        capturedAt = time(),
        capturedAtText = date("%Y-%m-%d %H:%M:%S"),
        destinationProfiles = self.db.destinationProfiles and copyValue(self.db.destinationProfiles) or nil,
        activeProfile = self.db.activeProfile,
        destinationProfile = not self.db.destinationProfiles and copyValue(self.db.destinationProfile) or nil,
        trainingModel = includeTrainingModel and copyValue(self.db.trainingModel) or nil,
        portableEvidenceImports = includeTrainingModel
            and copyValue(self.db.portableEvidenceImports) or nil,
        portableEvidenceOrigins = includeTrainingModel
            and copyValue(self.db.portableEvidenceOrigins) or nil,
    })
    while #history > maximumProfileChanges do
        table.remove(history, 1)
    end
    return history[#history]
end

local function requiredString(fields, index, maximumLength)
    local value = fields[index]
    if type(value) ~= "string" or value == "" or #value > maximumLength then
        return nil
    end
    return value
end

local function optionalString(fields, index, maximumLength)
    local value = fields[index]
    if value == nil or value == "" then
        return nil, true
    end
    if type(value) ~= "string" or #value > maximumLength then
        return nil, false
    end
    return value, true
end

local function optionalNumber(fields, index)
    local value = fields[index]
    if value == nil or value == "" then
        return nil, true
    end
    local number = limitedNumber(value)
    return number, number ~= nil
end

local function isNonnegativeInteger(value)
    return type(value) == "number" and value >= 0 and value % 1 == 0
end

local function isSafeText(value, allowPipe)
    return value == nil or (type(value) == "string"
        and not value:find("%c")
        and (allowPipe or not value:find("|", 1, true)))
end

local function hasAtMostFields(fields, maximum)
    return #fields <= maximum
end

local function isManagedBindingCommand(command)
    if command == nil then return true end
    if not isSafeText(command) or #command > 500 then return false end
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

local function isValidClickBindingInput(inputKey)
    if type(inputKey) ~= "string" or inputKey:sub(1, 10) ~= "CLICKCAST:" then
        return true
    end
    local modifiers, button = inputKey:match("^CLICKCAST:(%d+):([%a]+%d*)$")
    modifiers = tonumber(modifiers)
    if not modifiers or modifiers < 0 or modifiers > 31 or modifiers % 1 ~= 0 then
        return false
    end
    if button == "LeftButton" or button == "RightButton" or button == "MiddleButton" then
        return true
    end
    local buttonNumber = tonumber(button and button:match("^Button(%d+)$"))
    return buttonNumber ~= nil and buttonNumber >= 1 and buttonNumber <= 31
        and buttonNumber % 1 == 0
end

local function parsePortableProfile(value)
    if type(value) ~= "string" or value == "" then
        return nil, "Paste a lamdaUI profile string first"
    end
    if #value > maximumImportLength then
        return nil, "The profile string is too large"
    end
    local records = split(value, ";")
    local formatVersionText = records[1] and records[1]:match("^" .. formatPrefix .. "(%d+)$")
    local formatVersion = tonumber(formatVersionText)
    if not formatVersion or not checksumByVersion[formatVersion] then
        if formatVersion and formatVersion > currentFormatVersion then
            return nil, "This profile was created by a newer lamdaUI format"
        end
        return nil, "This is not a supported lamdaUI profile"
    end
    if #records < 2 then return nil, "This is not a supported lamdaUI profile" end
    if #records > maximumRecords then
        return nil, "The profile contains too many records"
    end
    local checksumFields = split(records[#records], "|")
    if checksumFields[1] ~= "Z" or #checksumFields ~= 2 then
        return nil, "The profile checksum is missing"
    end
    local body = table.concat(records, ";", 1, #records - 1)
    local suppliedChecksum = tonumber(checksumFields[2])
    if not isNonnegativeInteger(suppliedChecksum)
        or suppliedChecksum ~= checksumByVersion[formatVersion](body)
    then
        return nil, "The profile is incomplete or was changed while copying"
    end

    local result = {
        destinationProfile = {
            version = 1,
            id = "local-action-map",
            name = "Imported Profile",
            source = "imported",
            destinations = {},
            ui = {},
        },
        trainingModel = {
            version = 3,
            actionWeights = {},
            roleWeights = {},
            roleWeightsByProfile = {},
            destinationWeightsByProfile = {},
            profileEvidence = {},
            preferredRulesByProfile = {},
            preferredRules = {},
            contextSetups = {},
            sessions = {},
            totalConfirmedSetups = 0,
            totalPlacementChoices = 0,
        },
        portable = {
            sourceVersion = formatVersion,
            destinationWeights = {},
            evidenceFingerprint = table.concat({
                tostring(formatVersion),
                tostring(suppliedChecksum),
                tostring(legacyChecksum(body)),
                tostring(#body),
            }, ":"),
        },
    }
    local seenDestinations, seenInputs, seenActions, referencedDestinations = {}, {}, {}, {}
    local seenActionBarCommands = {}
    local destinationCount, actionCount = 0, 0
    local sawVersionRecord, sawNameRecord, sawOriginRecord = false, false, false
    local sawTotalsRecord, sawUIRecord = false, false

    for recordIndex = 2, #records - 1 do
        local fields = split(records[recordIndex], "|")
        local recordType = fields[1]
        for fieldIndex = 2, #fields do
            fields[fieldIndex] = decode(fields[fieldIndex])
        end
        if recordType == "V" then
            local declaredVersion = limitedNumber(fields[2])
            local sourceAddonVersion, validAddonVersion = optionalString(fields, 3, 50)
            if formatVersion < 2 or recordIndex ~= 2 or sawVersionRecord
                or not hasAtMostFields(fields, 3)
                or declaredVersion ~= formatVersion or not validAddonVersion
                or not isSafeText(sourceAddonVersion)
            then
                return nil, "The profile version metadata is invalid"
            end
            sawVersionRecord = true
            result.portable.sourceAddonVersion = sourceAddonVersion
        elseif recordType == "N" then
            local profileName = requiredString(fields, 2, 48)
            if formatVersion < 3 or sawNameRecord or not hasAtMostFields(fields, 2) or not profileName
                or profileName:find("[%c|]")
            then
                return nil, "The profile name is invalid"
            end
            sawNameRecord = true
            result.destinationProfile.name = profileName
        elseif recordType == "O" then
            local originID = requiredString(fields, 2, 100)
            if formatVersion < 5 or sawOriginRecord or not hasAtMostFields(fields, 2)
                or not validPortableOriginID(originID)
            then
                return nil, "The profile source identity is invalid"
            end
            sawOriginRecord = true
            result.portable.originID = originID
            result.destinationProfile.portableOriginID = originID
        elseif recordType == "M" then
            local setups = limitedNumber(fields[2], 0)
            local placements = limitedNumber(fields[3], 0)
            if sawTotalsRecord or not hasAtMostFields(fields, 3)
                or not isNonnegativeInteger(setups) or not isNonnegativeInteger(placements)
            then
                return nil, "Profile totals are invalid"
            end
            sawTotalsRecord = true
            result.trainingModel.totalConfirmedSetups = setups
            result.trainingModel.totalPlacementChoices = placements
            result.portable.confirmedSetups = setups
            result.portable.placementChoices = placements
        elseif recordType == "U" then
            local layoutName, validLayoutName = optionalString(fields, 2, 200)
            local layoutString, validLayoutString = optionalString(fields, 3, 750000)
            local actionBars, validActionBars = optionalString(fields, 4, 3)
            local cooldownViewerLayout, validCooldownViewerLayout = optionalString(fields, 5, 200000)
            local enableMouseoverCast, validEnableMouseoverCast = optionalString(fields, 6, 1)
            local mouseoverCastModifier, validMouseoverCastModifier = optionalString(fields, 7, 5)
            local autoSelfCast, validAutoSelfCast = optionalString(fields, 8, 1)
            local selfCastModifier, validSelfCastModifier = optionalString(fields, 9, 5)
            local focusCastModifier, validFocusCastModifier = optionalString(fields, 10, 5)
            local actionBarNumber = actionBars and tonumber(actionBars)
            local maximumUIFields = formatVersion >= 6 and 10 or 5
            local function validBinary(value)
                return value == nil or value == "0" or value == "1"
            end
            local function validCastModifier(value)
                return value == nil or value == "NONE" or value == "ALT"
                    or value == "CTRL" or value == "SHIFT"
            end
            if sawUIRecord or not hasAtMostFields(fields, maximumUIFields)
                or not validLayoutName or not validLayoutString or not validActionBars or not validCooldownViewerLayout
                or not validEnableMouseoverCast or not validMouseoverCastModifier
                or not validAutoSelfCast or not validSelfCastModifier or not validFocusCastModifier
                or not isSafeText(layoutName)
                or (actionBars and (not actionBarNumber
                    or actionBarNumber < 0
                    or actionBarNumber > 255
                    or actionBarNumber % 1 ~= 0))
                or not validBinary(enableMouseoverCast) or not validBinary(autoSelfCast)
                or not validCastModifier(mouseoverCastModifier)
                or not validCastModifier(selfCastModifier)
                or not validCastModifier(focusCastModifier)
            then
                return nil, "The profile contains invalid UI preferences"
            end
            sawUIRecord = true
            result.destinationProfile.ui = {
                editModeLayoutName = layoutName,
                editModeLayoutString = layoutString,
                multiActionBars = actionBars,
                cooldownViewerLayout = cooldownViewerLayout,
                enableMouseoverCast = enableMouseoverCast,
                mouseoverCastModifier = mouseoverCastModifier,
                autoSelfCast = autoSelfCast,
                selfCastModifier = selfCastModifier,
                focusCastModifier = focusCastModifier,
            }
        elseif recordType == "D" then
            local id = requiredString(fields, 2, 200)
            local inputKey = requiredString(fields, 3, 100)
            local label, validLabel = optionalString(fields, 4, 200)
            local category, validCategory = optionalString(fields, 5, 200)
            local bindingCommand, validBinding = optionalString(fields, 6, 500)
            local legacyButton, validLegacyButton = optionalNumber(fields, 7)
            local legacyLayer, validLegacyLayer = optionalString(fields, 8, 50)
            if not id or not inputKey then
                return nil, "A destination is missing its identity"
            end
            if not hasAtMostFields(fields, 8)
                or not validLabel or not validCategory or not validBinding
                or not validLegacyButton or not validLegacyLayer
                or not isSafeText(id, true) or not isSafeText(inputKey, true)
                or not isValidClickBindingInput(inputKey)
                or not isSafeText(label) or not isSafeText(category)
                or not isManagedBindingCommand(bindingCommand) or not isSafeText(legacyLayer)
                or (legacyButton ~= nil and (legacyButton <= 0 or legacyButton % 1 ~= 0))
                or seenDestinations[id] or seenInputs[inputKey]
                or (isActionBarBindingCommand(bindingCommand)
                    and seenActionBarCommands[bindingCommand])
            then
                return nil, "The profile contains an invalid or duplicate destination"
            end
            seenDestinations[id], seenInputs[inputKey] = true, true
            if isActionBarBindingCommand(bindingCommand) then
                seenActionBarCommands[bindingCommand] = true
            end
            destinationCount = destinationCount + 1
            if destinationCount > maximumDestinations then
                return nil, "The profile contains too many inputs"
            end
            table.insert(result.destinationProfile.destinations, {
                id = id,
                key = id,
                inputKey = inputKey,
                label = label or inputKey,
                category = category,
                bindingCommand = bindingCommand,
                legacyButton = legacyButton,
                legacyLayer = legacyLayer,
                source = "imported",
            })
        elseif recordType == "A" then
            local actionKey = requiredString(fields, 2, 200)
            local actionType = requiredString(fields, 3, 20)
            local actionID = limitedNumber(fields[4])
            local actionName = requiredString(fields, 5, 300)
            local roleList, validRoles = optionalString(fields, 6, 500)
            if not hasAtMostFields(fields, 8) or not actionKey or actionType ~= "spell"
                or not isNonnegativeInteger(actionID) or actionID == 0
                or actionKey ~= "spell:" .. tostring(actionID)
                or not actionName or not validRoles or not isSafeText(actionName, true)
                or seenActions[actionKey]
            then
                return nil, "The profile contains an invalid action"
            end
            seenActions[actionKey] = true
            actionCount = actionCount + 1
            if actionCount > maximumActions then
                return nil, "The profile contains too many learned actions"
            end
            local node = ensureAction(result.trainingModel, actionKey)
            node.actionType = actionType
            node.actionID = actionID
            node.actionName = actionName
            node.roles = roleList and split(roleList, ",") or {}
            local seenRoles = {}
            if #node.roles > maximumRolesPerAction then
                return nil, "The profile contains too many roles for one action"
            end
            for _, role in ipairs(node.roles) do
                if role == "" or #role > 50 or not isSafeText(role)
                    or seenRoles[role]
                then
                    return nil, "The profile contains an invalid action role"
                end
                seenRoles[role] = true
            end
            node.confirmations = limitedNumber(fields[7], 0)
            node.rejections = limitedNumber(fields[8], 0)
            if not isNonnegativeInteger(node.confirmations) or not isNonnegativeInteger(node.rejections) then
                return nil, "Action totals are invalid"
            end
        elseif recordType == "G" then
            local actionKey = requiredString(fields, 2, 200)
            local destination = requiredString(fields, 3, 200)
            local weight = limitedNumber(fields[4])
            if not hasAtMostFields(fields, 4) or not actionKey or not destination or weight == nil then
                return nil, "A global action weight is invalid"
            end
            local node = ensureAction(result.trainingModel, actionKey)
            if node.global[destination] ~= nil then return nil, "A global action weight is duplicated" end
            node.global[destination] = weight
            referencedDestinations[destination] = true
        elseif recordType == "S" or recordType == "C" then
            local actionKey = requiredString(fields, 2, 200)
            local contextKey = requiredString(fields, 3, 500)
            local destination = requiredString(fields, 4, 200)
            local weight = limitedNumber(fields[5])
            if not hasAtMostFields(fields, 5) or not actionKey or not contextKey or not destination or weight == nil
                or not isSafeText(contextKey, true)
            then
                return nil, "A contextual action weight is invalid"
            end
            local node = ensureAction(result.trainingModel, actionKey)
            local parent = recordType == "S" and node.bySpec or node.byContext
            parent[contextKey] = parent[contextKey] or {}
            if parent[contextKey][destination] ~= nil then
                return nil, "A contextual action weight is duplicated"
            end
            parent[contextKey][destination] = weight
            referencedDestinations[destination] = true
        elseif recordType == "R" then
            local role = requiredString(fields, 2, 50)
            local destination = requiredString(fields, 3, 200)
            local weight = limitedNumber(fields[4])
            if not hasAtMostFields(fields, 4) or not role or not destination or weight == nil
                or not isSafeText(role)
            then
                return nil, "A role weight is invalid"
            end
            result.trainingModel.roleWeights[role] = result.trainingModel.roleWeights[role] or {}
            if result.trainingModel.roleWeights[role][destination] ~= nil then
                return nil, "A role weight is duplicated"
            end
            result.trainingModel.roleWeights[role][destination] = weight
            referencedDestinations[destination] = true
        elseif recordType == "W" then
            local destination = requiredString(fields, 2, 200)
            local weight = limitedNumber(fields[3])
            if formatVersion < 4 or not hasAtMostFields(fields, 3) or not destination or weight == nil
                or result.portable.destinationWeights[destination] ~= nil
            then
                return nil, "A profile input weight is invalid"
            end
            result.portable.destinationWeights[destination] = weight
            referencedDestinations[destination] = true
        elseif recordType == "P" then
            local role = requiredString(fields, 2, 50)
            local destination = requiredString(fields, 3, 200)
            local label, validLabel = optionalString(fields, 4, 200)
            local source, validSource = optionalString(fields, 5, 100)
            local weight = limitedNumber(fields[6], 0)
            if not hasAtMostFields(fields, 6) or not role or not destination
                or not validLabel or not validSource or weight == nil
                or not isSafeText(role) or not isSafeText(label) or not isSafeText(source)
                or result.trainingModel.preferredRules[role] ~= nil
                or weight < 0
            then
                return nil, "A preferred rule is invalid"
            end
            result.trainingModel.preferredRules[role] = {
                destination = destination,
                label = label,
                source = source,
                weight = weight,
            }
            referencedDestinations[destination] = true
        else
            return nil, "The profile contains an unknown record type"
        end
    end

    if formatVersion >= 2 and not sawVersionRecord then
        return nil, "The profile version metadata is missing"
    end
    if formatVersion >= 3 and not sawNameRecord then
        return nil, "The profile name is missing"
    end
    if formatVersion >= 5 and not sawOriginRecord then
        return nil, "The profile source identity is missing"
    end

    if #result.destinationProfile.destinations == 0 then
        return nil, "The profile has no action-map destinations"
    end
    for _, node in pairs(result.trainingModel.actionWeights) do
        if node.actionType ~= "spell" or not node.actionID then
            return nil, "The profile contains weights for an undefined action"
        end
    end
    for destination in pairs(referencedDestinations) do
        if not seenDestinations[destination] then
            return nil, "The profile contains learning for an unknown input"
        end
    end
    return result
end

local function mergeWeightMap(target, source)
    for key, weight in pairs(type(source) == "table" and source or {}) do
        if type(key) == "string" and type(weight) == "number" then
            local merged = (tonumber(target[key]) or 0) + weight
            target[key] = math.max(-maximumWeight, math.min(maximumWeight, merged))
        end
    end
end

local function mergeNestedWeightMap(target, source, keyTransform)
    for key, weights in pairs(type(source) == "table" and source or {}) do
        local targetKey = type(key) == "string" and (keyTransform and keyTransform(key) or key)
        if targetKey and type(weights) == "table" then
            target[targetKey] = type(target[targetKey]) == "table" and target[targetKey] or {}
            mergeWeightMap(target[targetKey], weights)
        end
    end
end

local function mergeRoleList(target, source)
    local seen = {}
    for _, role in ipairs(type(target) == "table" and target or {}) do
        if type(role) == "string" then seen[role] = true end
    end
    target = type(target) == "table" and target or {}
    for _, role in ipairs(type(source) == "table" and source or {}) do
        if type(role) == "string" and not seen[role] then
            seen[role] = true
            table.insert(target, role)
        end
    end
    return target
end

local function scopePortableContext(contextKey, profileID)
    local suffix = "|" .. tostring(profileID)
    if contextKey:sub(-#suffix) == suffix then
        return contextKey
    end
    return contextKey .. suffix
end

local function mergeTrainingModels(existing, imported, profileID, portable, mergeSharedEvidence)
    existing = type(existing) == "table" and existing or {}
    existing.version = math.max(tonumber(existing.version) or 1, tonumber(imported.version) or 1)
    existing.actionWeights = type(existing.actionWeights) == "table" and existing.actionWeights or {}
    existing.roleWeights = type(existing.roleWeights) == "table" and existing.roleWeights or {}
    existing.roleWeightsByProfile = type(existing.roleWeightsByProfile) == "table" and existing.roleWeightsByProfile or {}
    existing.destinationWeightsByProfile = type(existing.destinationWeightsByProfile) == "table" and existing.destinationWeightsByProfile or {}
    existing.profileEvidence = type(existing.profileEvidence) == "table" and existing.profileEvidence or {}
    existing.preferredRulesByProfile = type(existing.preferredRulesByProfile) == "table" and existing.preferredRulesByProfile or {}
    existing.preferredRules = type(existing.preferredRules) == "table" and existing.preferredRules or {}
    existing.contextSetups = type(existing.contextSetups) == "table" and existing.contextSetups or {}
    existing.sessions = type(existing.sessions) == "table" and existing.sessions or {}
    existing.totalConfirmedSetups = tonumber(existing.totalConfirmedSetups) or 0
    existing.totalPlacementChoices = tonumber(existing.totalPlacementChoices) or 0

    for actionKey, importedNode in pairs(imported.actionWeights or {}) do
        if type(actionKey) == "string" and type(importedNode) == "table" then
            local node = existing.actionWeights[actionKey]
            if type(node) ~= "table" then
                node = {
                    key = actionKey,
                    actionType = importedNode.actionType,
                    actionID = importedNode.actionID,
                    actionName = importedNode.actionName,
                    roles = {},
                    global = {},
                    bySpec = {},
                    byContext = {},
                    confirmations = 0,
                    rejections = 0,
                }
                existing.actionWeights[actionKey] = node
            end
            node.actionType = node.actionType or importedNode.actionType
            node.actionID = node.actionID or importedNode.actionID
            node.actionName = node.actionName or importedNode.actionName
            node.roles = mergeRoleList(node.roles, importedNode.roles)
            node.global = type(node.global) == "table" and node.global or {}
            node.bySpec = type(node.bySpec) == "table" and node.bySpec or {}
            node.byContext = type(node.byContext) == "table" and node.byContext or {}
            if mergeSharedEvidence then
                mergeWeightMap(node.global, importedNode.global)
                mergeNestedWeightMap(node.bySpec, importedNode.bySpec)
            end
            mergeNestedWeightMap(node.byContext, importedNode.byContext, function(contextKey)
                return scopePortableContext(contextKey, profileID)
            end)
            if mergeSharedEvidence then
                node.confirmations = math.min(maximumWeight,
                    (tonumber(node.confirmations) or 0) + (tonumber(importedNode.confirmations) or 0))
                node.rejections = math.min(maximumWeight,
                    (tonumber(node.rejections) or 0) + (tonumber(importedNode.rejections) or 0))
            end
        end
    end

    local profileRoles = {}
    existing.roleWeightsByProfile[profileID] = profileRoles
    for role, destinations in pairs(imported.roleWeights or {}) do
        if type(role) == "string" and type(destinations) == "table" then
            existing.roleWeights[role] = type(existing.roleWeights[role]) == "table" and existing.roleWeights[role] or {}
            profileRoles[role] = {}
            if mergeSharedEvidence then mergeWeightMap(existing.roleWeights[role], destinations) end
            mergeWeightMap(profileRoles[role], destinations)
        end
    end

    local destinationWeights = {}
    existing.destinationWeightsByProfile[profileID] = destinationWeights
    mergeWeightMap(destinationWeights, portable.destinationWeights)
    existing.profileEvidence[profileID] = {
        confirmedSetups = portable.confirmedSetups or 0,
        placementChoices = portable.placementChoices or 0,
    }
    if mergeSharedEvidence then
        existing.totalConfirmedSetups = math.min(maximumWeight,
            existing.totalConfirmedSetups + (portable.confirmedSetups or 0))
        existing.totalPlacementChoices = math.min(maximumWeight,
            existing.totalPlacementChoices + (portable.placementChoices or 0))
    end

    local profilePreferredRules = {}
    existing.preferredRulesByProfile[profileID] = profilePreferredRules
    for role, rule in pairs(imported.preferredRules or {}) do
        if type(role) == "string" and type(rule) == "table" then
            profilePreferredRules[role] = copyValue(rule)
        end
    end
    return existing
end

local function initializeImportFingerprints(addon)
    local stored = type(addon.db.portableEvidenceImports) == "table"
        and addon.db.portableEvidenceImports or {}
    local clean, ranked = {}, {}
    for fingerprint, record in pairs(stored) do
        if type(fingerprint) == "string" and #fingerprint <= 100
            and type(record) == "table"
        then
            local importedAt = tonumber(record.importedAt) or 0
            clean[fingerprint] = {
                importedAt = importedAt,
                profileID = type(record.profileID) == "string" and record.profileID or nil,
            }
            table.insert(ranked, { fingerprint = fingerprint, importedAt = importedAt })
        end
    end
    table.sort(ranked, function(left, right)
        if left.importedAt == right.importedAt then return left.fingerprint < right.fingerprint end
        return left.importedAt > right.importedAt
    end)
    for index = maximumImportFingerprints + 1, #ranked do
        clean[ranked[index].fingerprint] = nil
    end
    addon.db.portableEvidenceImports = clean
    return clean
end

local function initializeImportOrigins(addon)
    local stored = type(addon.db.portableEvidenceOrigins) == "table"
        and addon.db.portableEvidenceOrigins or {}
    local clean, ranked = {}, {}
    for originID, record in pairs(stored) do
        if validPortableOriginID(originID) and type(record) == "table" then
            local importedAt = tonumber(record.importedAt) or 0
            clean[originID] = {
                importedAt = importedAt,
                profileID = type(record.profileID) == "string" and record.profileID or nil,
                evidenceFingerprint = type(record.evidenceFingerprint) == "string"
                    and #record.evidenceFingerprint <= 100 and record.evidenceFingerprint or nil,
            }
            table.insert(ranked, { originID = originID, importedAt = importedAt })
        end
    end
    table.sort(ranked, function(left, right)
        if left.importedAt == right.importedAt then return left.originID < right.originID end
        return left.importedAt > right.importedAt
    end)
    for index = maximumImportOrigins + 1, #ranked do
        clean[ranked[index].originID] = nil
    end
    addon.db.portableEvidenceOrigins = clean
    return clean
end

local function captureImportState(addon)
    local history = addon:InitializeProfileChangeHistory()
    return {
        destinationProfiles = copyValue(addon.db.destinationProfiles),
        destinationProfile = copyValue(addon.db.destinationProfile),
        destinationProfileSequence = addon.db.destinationProfileSequence,
        activeProfile = addon.db.activeProfile,
        trainingModel = copyValue(addon.db.trainingModel),
        portableEvidenceImports = copyValue(addon.db.portableEvidenceImports),
        portableEvidenceOrigins = copyValue(addon.db.portableEvidenceOrigins),
        characters = copyValue(addon.db.characters),
        profileChangeHistory = copyValue(history),
        profileImportBackup = copyValue(addon.db.profileImportBackup),
        currentSnapshot = addon.currentSnapshot,
        trainingRecommendations = addon.trainingRecommendations,
        trainingCoverage = addon.trainingCoverage,
        layoutPlan = addon.layoutPlan,
    }
end

local function restoreImportState(addon, state)
    addon.db.destinationProfiles = copyValue(state.destinationProfiles)
    addon.db.destinationProfile = copyValue(state.destinationProfile)
    addon.db.destinationProfileSequence = state.destinationProfileSequence
    addon.db.activeProfile = state.activeProfile
    addon.db.trainingModel = copyValue(state.trainingModel)
    addon.db.portableEvidenceImports = copyValue(state.portableEvidenceImports)
    addon.db.portableEvidenceOrigins = copyValue(state.portableEvidenceOrigins)
    addon.db.characters = copyValue(state.characters)
    addon.db.profileChangeHistory = copyValue(state.profileChangeHistory)
    addon.db.profileImportBackup = copyValue(state.profileImportBackup)
    addon.destinationProfile = type(addon.db.destinationProfiles) == "table"
        and addon.db.destinationProfiles[addon.db.activeProfile]
        or addon.db.destinationProfile
    addon.currentSnapshot = state.currentSnapshot
    addon.trainingRecommendations = state.trainingRecommendations
    addon.trainingCoverage = state.trainingCoverage
    addon.layoutPlan = state.layoutPlan
end

function Addon:ImportPortableProfile(value)
    if InCombatLockdown and InCombatLockdown() then
        self:SetStatus("Cannot import a profile during combat")
        return false
    end
    local parsed, parseError = parsePortableProfile(value)
    if not parsed then
        self:SetStatus(parseError)
        return false, parseError
    end
    local stateBefore = captureImportState(self)
    local importFingerprints = initializeImportFingerprints(self)
    local importOrigins = initializeImportOrigins(self)
    local evidenceFingerprint = parsed.portable.evidenceFingerprint
    local originID = parsed.portable.originID
    local duplicateFingerprint = importFingerprints[evidenceFingerprint] ~= nil
    local duplicateOrigin = originID and importOrigins[originID] ~= nil or false
    local alreadyMerged = duplicateFingerprint or duplicateOrigin
    local imported, installedProfile = pcall(function()
        local existingModel = self.InitializeTrainingModel and self:InitializeTrainingModel()
            or (type(self.db.trainingModel) == "table" and self.db.trainingModel or {})
        self:PushProfileChange("import", true)
        parsed.destinationProfile.importedAt = time()
        parsed.destinationProfile.importedAtText = date("%Y-%m-%d %H:%M:%S")
        parsed.destinationProfile.importedFormatVersion = parsed.portable.sourceVersion
        parsed.destinationProfile.importedAddonVersion = parsed.portable.sourceAddonVersion
        local profile
        if self.InstallDestinationProfile then
            profile = self:InstallDestinationProfile(parsed.destinationProfile, parsed.destinationProfile.name)
        else
            self.db.destinationProfile = parsed.destinationProfile
            self.destinationProfile = parsed.destinationProfile
            profile = parsed.destinationProfile
        end
        local importedProfileID = profile.id
        self.db.trainingModel = mergeTrainingModels(existingModel, parsed.trainingModel, importedProfileID,
            parsed.portable, not alreadyMerged)
        importFingerprints[evidenceFingerprint] = {
            importedAt = time(),
            profileID = importedProfileID,
        }
        if originID then
            importOrigins[originID] = {
                importedAt = time(),
                profileID = importedProfileID,
                evidenceFingerprint = evidenceFingerprint,
            }
        end
        self.trainingRecommendations = nil
        self.trainingCoverage = nil
        self.layoutPlan = nil
        if self.CaptureSnapshot then self:CaptureSnapshot("profile imported") end
        return profile
    end)
    if not imported then
        restoreImportState(self, stateBefore)
        self:SetStatus("Profile import failed • Local profiles and learning were restored")
        if self.RefreshUI then pcall(self.RefreshUI, self) end
        return false, "Profile import failed"
    end
    local migrationText = parsed.portable.sourceVersion < currentFormatVersion and " • Older format updated" or ""
    local learningText
    if duplicateFingerprint then
        learningText = "Reusable learning was already merged"
    elseif duplicateOrigin then
        learningText = "Updated this source without recounting shared learning"
    else
        learningText = "Merged " .. countLabel(#sortedKeys(parsed.trainingModel.actionWeights), "learned spell")
    end
    self:SetStatus("Imported " .. installedProfile.name .. " • "
        .. countLabel(#installedProfile.destinations, "input") .. " • " .. learningText
        .. " • Local learning kept" .. migrationText)
    return true
end

function Addon:CanUndoLastProfileChange()
    local history = self.db and self:InitializeProfileChangeHistory()
    local backup = history and history[#history]
    if not backup then
        return false, "There is no profile change to undo"
    end
    local transaction = self.db.layoutTransactions and self.db.layoutTransactions.active
    local transactionProfileID = type(transaction) == "table" and type(transaction.context) == "table"
        and transaction.context.profileID
    if transactionProfileID then
        local profileWillRemain = backup.destinationProfiles and backup.destinationProfiles[transactionProfileID]
            or (backup.destinationProfile and backup.destinationProfile.id == transactionProfileID)
        if not profileWillRemain then
            return false, "Undo the active layout before removing its profile"
        end
    end
    return true, nil, backup
end

function Addon:UndoLastProfileChange()
    local canUndo, blockedReason, backup = self:CanUndoLastProfileChange()
    if not canUndo then
        self:SetStatus(blockedReason)
        return false
    end
    local history = self:InitializeProfileChangeHistory()
    table.remove(history)
    if backup.destinationProfiles then
        self.db.destinationProfiles = copyValue(backup.destinationProfiles)
        self.db.activeProfile = backup.activeProfile
        self.db.destinationProfile = nil
        self.destinationProfile = nil
        if self.InitializeDestinationProfile then self:InitializeDestinationProfile() end
    elseif backup.destinationProfile then
        self.db.destinationProfiles = nil
        self.db.activeProfile = backup.destinationProfile.id
        self.db.destinationProfile = copyValue(backup.destinationProfile)
        self.destinationProfile = self.db.destinationProfile
        if self.InitializeDestinationProfile then self:InitializeDestinationProfile() end
    end
    if backup.trainingModel then
        self.db.trainingModel = copyValue(backup.trainingModel)
        self.db.portableEvidenceImports = copyValue(backup.portableEvidenceImports) or {}
        self.db.portableEvidenceOrigins = copyValue(backup.portableEvidenceOrigins) or {}
    end
    self.trainingRecommendations = nil
    self.trainingCoverage = nil
    self.layoutPlan = nil
    self:CaptureSnapshot("profile change undone")
    self:SetStatus("Undid the last profile change")
    return true
end

-- Retain the old method name for slash-command and SavedVariables migration
-- compatibility while presenting one generic undo action in the interface.
function Addon:RestoreProfileImportBackup()
    return self:UndoLastProfileChange()
end

function Addon:InitializePortablePopups()
    if not StaticPopupDialogs then return end
    StaticPopupDialogs.LAMDAUI_EXPORT_PROFILE = {
        text = "Copy this portable profile. It includes the profile name, controls, UI settings, and reusable learning. Character names, snapshots, macro text, and restore history are excluded.",
        button1 = DONE,
        hasEditBox = true,
        OnShow = function(dialog)
            dialog.editBox:SetMaxLetters(0)
            dialog.editBox:SetText(Addon:ExportPortableProfile())
            dialog.editBox:HighlightText()
            dialog.editBox:SetFocus()
        end,
        EditBoxOnEscapePressed = function(editBox) editBox:GetParent():Hide() end,
        timeout = 0,
        whileDead = true,
        hideOnEscape = true,
        preferredIndex = 3,
    }
    StaticPopupDialogs.LAMDAUI_IMPORT_PROFILE = {
        text = "Paste a lamdaUI profile. It will be added and selected, and its reusable learning will be merged with your local data. Undo Change rolls back the complete import.",
        button1 = "Import",
        button2 = CANCEL,
        hasEditBox = true,
        OnShow = function(dialog)
            dialog.editBox:SetMaxLetters(0)
            dialog.editBox:SetText("")
            dialog.editBox:SetFocus()
        end,
        OnAccept = function(dialog)
            Addon:ImportPortableProfile(dialog.editBox:GetText())
        end,
        EditBoxOnEnterPressed = function(editBox)
            Addon:ImportPortableProfile(editBox:GetText())
            editBox:GetParent():Hide()
        end,
        EditBoxOnEscapePressed = function(editBox) editBox:GetParent():Hide() end,
        timeout = 0,
        whileDead = true,
        hideOnEscape = true,
        preferredIndex = 3,
    }
end

function Addon:ShowProfileExport()
    local profile = self.GetActiveDestinationProfile and self:GetActiveDestinationProfile() or self.destinationProfile
    if not profile or #(profile.destinations or {}) == 0 then
        self:SetStatus("Capture at least one bound input before exporting this profile")
        return false
    end
    self:InitializePortablePopups()
    if StaticPopup_Show then StaticPopup_Show("LAMDAUI_EXPORT_PROFILE") end
    return true
end

function Addon:ShowProfileImport()
    self:InitializePortablePopups()
    if StaticPopup_Show then StaticPopup_Show("LAMDAUI_IMPORT_PROFILE") end
end
