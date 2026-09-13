local _, Addon = ...

local transactionVersion = 5
local oldestSupportedTransactionVersion = 1
local maximumHistory = 20

local transactionStatusLabels = {
    staged = "Waiting for reload",
    applying = "Applying",
    applied = "Applied; Undo available",
    undone = "Undone",
    recovered = "Recovered",
    superseded = "Replaced by a newer Apply",
    ["apply-failed-restored"] = "Apply failed; original restored",
    ["binding-save-failed-restored"] = "Binding save failed; original restored",
    ["undo-incomplete"] = "Undo needs attention",
    ["recovery-incomplete"] = "Recovery needs attention",
    ["apply-failed-restore-incomplete"] = "Apply recovery needs attention",
    invalid = "Invalid recovery data",
}

local function countLabel(count, singular, plural)
    count = tonumber(count) or 0
    return tostring(count) .. " " .. (count == 1 and singular or (plural or (singular .. "s")))
end

local function escapeMarkup(value)
    return tostring(value or ""):gsub("|", "||")
end

local function cursorIsEmpty()
    return not GetCursorInfo or GetCursorInfo() == nil
end

local function clearCursor()
    if ClearCursor then
        ClearCursor()
    end
end

local function putActionInSlot(slot)
    if C_ActionBar and C_ActionBar.PutActionInSlot then
        local ok, result = pcall(C_ActionBar.PutActionInSlot, slot)
        return ok and result ~= false
    end
    if PlaceAction then
        local ok, result = pcall(PlaceAction, slot)
        return ok and result ~= false
    end
    return false
end

local function actionSlotMutationError()
    if not C_Spell or type(C_Spell.PickupSpell) ~= "function"
        or (not (C_ActionBar and type(C_ActionBar.PutActionInSlot) == "function")
            and type(PlaceAction) ~= "function")
        or type(PickupAction) ~= "function"
        or type(GetActionInfo) ~= "function"
        or type(GetCursorInfo) ~= "function"
        or type(ClearCursor) ~= "function"
    then
        return "Action-bar mutation and rollback APIs are unavailable"
    end
end

local function actionLabel(action)
    if not action or not action.actionType then
        return "Empty"
    end
    return action.actionName or (action.actionType .. " " .. tostring(action.actionID or ""))
end

local function copyAction(action)
    action = action or {}
    return {
        actionType = action.actionType,
        actionID = action.actionID,
        baseSpellID = action.baseSpellID,
        actionSubType = action.actionSubType,
        actionName = action.actionName,
        macroName = action.macroName,
        macroBody = action.macroBody,
        icon = action.icon,
        bindingCommand = action.bindingCommand,
    }
end

local function actionFingerprint(action)
    if not action or not action.actionType then
        return "empty"
    end
    return table.concat({
        tostring(action.actionType or ""),
        tostring(action.actionID or ""),
        tostring(action.actionSubType or ""),
        tostring(action.macroName or ""),
        tostring(action.macroBody or ""),
    }, "\031")
end

local function addChecksumValue(parts, value)
    local encoded = value == nil and "nil" or (type(value) .. ":" .. tostring(value))
    table.insert(parts, tostring(#encoded) .. ":" .. encoded)
end

local function addChecksumAction(parts, action, extended)
    action = type(action) == "table" and action or {}
    addChecksumValue(parts, action.actionType)
    addChecksumValue(parts, action.actionID)
    if extended then
        addChecksumValue(parts, action.baseSpellID)
        addChecksumValue(parts, action.clickBindingType)
    end
    addChecksumValue(parts, action.actionSubType)
    addChecksumValue(parts, action.actionName)
    addChecksumValue(parts, action.macroName)
    addChecksumValue(parts, action.macroBody)
    addChecksumValue(parts, action.bindingCommand)
    addChecksumValue(parts, action.command)
end

local function transactionPayloadChecksum(transaction)
    local parts = {}
    local context = type(transaction.context) == "table" and transaction.context or {}
    addChecksumValue(parts, transaction.version)
    addChecksumValue(parts, transaction.id)
    addChecksumValue(parts, transaction.bindingSet)
    addChecksumValue(parts, transaction.changesBindings)
    addChecksumValue(parts, context.characterKey)
    addChecksumValue(parts, context.class)
    addChecksumValue(parts, context.specializationID)
    addChecksumValue(parts, context.specialization)
    addChecksumValue(parts, context.stateKey)
    addChecksumValue(parts, context.formName)
    addChecksumValue(parts, context.activeConfigID)
    addChecksumValue(parts, context.activeHeroSpecID)
    addChecksumValue(parts, context.profileID)
    for _, entry in ipairs(type(transaction.entries) == "table" and transaction.entries or {}) do
        local target = type(entry.target) == "table" and entry.target or {}
        addChecksumValue(parts, target.kind)
        addChecksumValue(parts, target.slot)
        addChecksumValue(parts, target.key)
        addChecksumValue(parts, target.inputKey)
        addChecksumValue(parts, target.expectedBindingCommand)
        local extended = (tonumber(transaction.version) or 0) >= 3
        if extended then
            addChecksumValue(parts, target.button)
            addChecksumValue(parts, target.modifiers)
        end
        if (tonumber(transaction.version) or 0) >= 5 then
            addChecksumValue(parts, target.preserveClickIdentity)
        end
        addChecksumAction(parts, entry.before, extended)
        addChecksumAction(parts, entry.desired, extended)
        addChecksumValue(parts, entry.beforeFingerprint)
    end
    for _, entry in ipairs(type(transaction.uiEntries) == "table" and transaction.uiEntries or {}) do
        local before = type(entry.before) == "table" and entry.before or {}
        local desired = type(entry.desired) == "table" and entry.desired or {}
        addChecksumValue(parts, entry.kind)
        addChecksumValue(parts, entry.cvar)
        -- Edit Mode indices are resolved again by layout name and may change
        -- after staging, so names are the immutable preference identifiers.
        addChecksumValue(parts, before.name)
        addChecksumValue(parts, before.value)
        addChecksumValue(parts, desired.name)
        addChecksumValue(parts, desired.value)
        if (tonumber(transaction.version) or 0) >= 2 then
            addChecksumValue(parts, before.layoutType)
            addChecksumValue(parts, desired.layoutType)
            addChecksumValue(parts, desired.install)
        end
        if (tonumber(transaction.version) or 0) >= 4 then
            addChecksumValue(parts, entry.modifiedClick)
        end
    end
    local payload = table.concat(parts, "|")
    local first, second = 1, 0
    for index = 1, #payload do
        first = (first + payload:byte(index)) % 65521
        second = (second + first) % 65521
    end
    return (second * 65536) + first
end

local function copyTransactionEntry(entry)
    local target = entry.target or {}
    local storedTarget = {
        kind = target.kind,
        slot = target.slot,
        key = target.key,
        inputKey = target.inputKey,
        expectedBindingCommand = target.expectedBindingCommand,
        button = target.button,
        modifiers = target.modifiers,
        preserveClickIdentity = target.preserveClickIdentity,
    }
    local before
    if target.kind == "click-binding" then
        before = {
            clickBindingType = entry.before and entry.before.clickBindingType,
            actionID = entry.before and entry.before.actionID,
            actionName = entry.before and entry.before.actionName,
            macroName = entry.before and entry.before.macroName,
            macroBody = entry.before and entry.before.macroBody,
        }
    elseif target.kind == "binding" then
        before = { command = tostring(entry.before and entry.before.command or "") }
    else
        before = copyAction(entry.before)
    end
    return {
        target = storedTarget,
        before = before,
        desired = copyAction(entry.desired),
        beforeFingerprint = entry.beforeFingerprint,
    }
end

local function copyTransactionUIEntry(entry)
    local before = entry.before or {}
    local desired = entry.desired or {}
    return {
        kind = entry.kind,
        cvar = entry.cvar,
        modifiedClick = entry.modifiedClick,
        before = {
            index = before.index,
            name = before.name,
            value = before.value,
            layoutType = before.layoutType,
        },
        desired = {
            index = desired.index,
            name = desired.name,
            value = desired.value,
            layoutType = desired.layoutType,
            install = desired.install,
        },
    }
end

local function desiredMatches(current, desired)
    if not current or current.actionType ~= desired.actionType then
        return false
    end
    if desired.actionType == "spell" then
        return current.actionID == desired.actionID
            or (current.actionName and desired.actionName and current.actionName == desired.actionName)
    end
    return actionFingerprint(current) == actionFingerprint(desired)
end

local function canRestoreAction(action, snapshot)
    if not action or not action.actionType then
        return true
    end
    if action.actionSubType == "assistedcombat" then
        return false, "Assisted Combat actions cannot be restored exactly"
    end
    if action.actionType == "spell" then
        if not C_Spell or type(C_Spell.PickupSpell) ~= "function" then
            return false, "Spell placement is unavailable"
        end
        local spellbook = snapshot and snapshot.spellbook
        if spellbook and spellbook.available then
            for _, spell in ipairs(spellbook.items or {}) do
                if not spell.isPassive and not spell.isOffSpec
                    and (spell.spellID == action.actionID
                        or spell.actionID == action.actionID
                        or spell.overrideSpellID == action.actionID
                        or (spell.name and action.actionName and spell.name == action.actionName))
                then
                    action.baseSpellID = action.baseSpellID or spell.baseSpellID
                        or spell.actionID or spell.spellID
                    return true
                end
            end
            return false, actionLabel(action) .. " is not available in the active spellbook"
        end
        return true
    end
    if action.actionType == "item" then
        if not C_Item or type(C_Item.PickupItem) ~= "function" then
            return false, "Item placement is unavailable"
        end
        local getItemCount = C_Item.GetItemCount or GetItemCount
        if getItemCount then
            local ok, count = pcall(getItemCount, action.actionID)
            if ok and type(count) == "number" and count < 1 then
                return false, actionLabel(action) .. " is not currently available to restore"
            end
        end
        return true
    end
    if action.actionType == "macro" then
        if not action.macroName or not action.actionID then
            return false, "This spell-backed macro has no restorable macro identity"
        end
        if type(PickupMacro) ~= "function" then
            return false, "Macro placement is unavailable"
        end
        if GetMacroInfo then
            local name = GetMacroInfo(action.actionID)
            if not name or name ~= action.macroName then
                return false, "Macro " .. tostring(action.macroName) .. " is no longer available"
            end
        end
        return true
    end
    if action.actionType == "equipmentset" then
        local pickup = C_EquipmentSet and C_EquipmentSet.PickupEquipmentSet or PickupEquipmentSet
        return type(pickup) == "function", "Equipment-set placement is unavailable"
    end
    return false, "Action type " .. tostring(action.actionType) .. " is not supported"
end

local function pickupAction(action)
    if action.actionType == "spell" then
        return pcall(C_Spell.PickupSpell, action.baseSpellID or action.actionID)
    elseif action.actionType == "item" then
        return pcall(C_Item.PickupItem, action.actionID)
    elseif action.actionType == "macro" then
        return pcall(PickupMacro, action.actionID)
    elseif action.actionType == "equipmentset" then
        local pickup = C_EquipmentSet and C_EquipmentSet.PickupEquipmentSet or PickupEquipmentSet
        return pcall(pickup, action.actionID)
    end
    return false
end

local function clearActionSlot(slot)
    local actionType = GetActionInfo(slot)
    if not actionType then
        return true
    end
    local ok = pcall(PickupAction, slot)
    if not ok or cursorIsEmpty() then
        clearCursor()
        return false
    end
    clearCursor()
    return GetActionInfo(slot) == nil
end

local function restoreSlot(addon, slot, action)
    if not action.actionType then
        if clearActionSlot(slot) then
            return true
        end
        return false, "slot " .. tostring(slot) .. " could not be cleared"
    end
    clearCursor()
    local ok = pickupAction(action)
    if not ok or cursorIsEmpty() then
        clearCursor()
        return false, actionLabel(action) .. " could not be picked up"
    end
    local placed = putActionInSlot(slot)
    clearCursor()
    if not placed then
        return false, "slot " .. tostring(slot) .. " could not be restored"
    end
    local current = copyAction(addon:DescribeActionSlot(slot))
    if actionFingerprint(current) ~= actionFingerprint(action) then
        return false, "slot " .. tostring(slot) .. " did not match its saved state"
    end
    return true
end

local function placeSpell(addon, slot, desired)
    clearCursor()
    local ok = pcall(C_Spell.PickupSpell, desired.baseSpellID or desired.actionID)
    if not ok or cursorIsEmpty() then
        clearCursor()
        return false, desired.actionName .. " could not be picked up"
    end
    local placed = putActionInSlot(slot)
    clearCursor()
    if not placed then
        return false, "slot " .. tostring(slot) .. " could not be changed"
    end
    local current = copyAction(addon:DescribeActionSlot(slot))
    if not desiredMatches(current, desired) then
        return false, "slot " .. tostring(slot) .. " did not receive " .. desired.actionName
    end
    return true
end

local validActiveStatuses = {
    staged = true,
    applying = true,
    applied = true,
    ["undo-incomplete"] = true,
    ["recovery-incomplete"] = true,
    ["apply-failed-restore-incomplete"] = true,
}

local function validStoredString(value, maximumLength)
    return type(value) == "string" and value ~= "" and #value <= maximumLength
        and not value:find("%c")
end

local function validOptionalStoredString(value, maximumLength)
    return value == nil or validStoredString(value, maximumLength)
end

local function validOptionalStoredText(value, maximumLength)
    return value == nil or (type(value) == "string" and #value <= maximumLength
        and not value:find("%c"))
end

local function validOptionalStoredBlob(value, maximumLength)
    return value == nil or (type(value) == "string" and #value <= maximumLength)
end

local function validDenseArray(value)
    if type(value) ~= "table" then return false end
    local count = 0
    for key in pairs(value) do
        if type(key) ~= "number" or key < 1 or key % 1 ~= 0 then return false end
        count = count + 1
    end
    return count == #value
end

local function validStoredAction(action, allowEmpty)
    if type(action) ~= "table" then return false end
    if action.actionType == nil then
        return allowEmpty and action.actionID == nil and action.actionSubType == nil
            and validOptionalStoredString(action.actionName, 300)
            and validOptionalStoredText(action.bindingCommand, 500)
    end
    if action.baseSpellID ~= nil
        and (type(action.baseSpellID) ~= "number" or action.baseSpellID <= 0
            or action.baseSpellID % 1 ~= 0)
    then
        return false
    end
    if not validOptionalStoredString(action.actionSubType, 100)
        or not validStoredString(action.actionName, 300)
        or not validOptionalStoredText(action.bindingCommand, 500)
    then
        return false
    end
    if action.actionType == "spell" or action.actionType == "item" or action.actionType == "macro" then
        if type(action.actionID) ~= "number" or action.actionID <= 0 or action.actionID % 1 ~= 0 then
            return false
        end
        if action.actionType == "macro" then
            return validStoredString(action.macroName, 300)
                and validOptionalStoredBlob(action.macroBody, 10000)
        end
        return action.macroName == nil and action.macroBody == nil
    elseif action.actionType == "equipmentset" then
        return (type(action.actionID) == "number" and action.actionID > 0 and action.actionID % 1 == 0)
            or validStoredString(action.actionID, 300)
    end
    return false
end

local function validManagedBindingCommand(command)
    if command == nil then return true end
    if not validStoredString(command, 500) then return false end
    local mainButton = tonumber(command:match("^ACTIONBUTTON(%d+)$"))
    if mainButton then return mainButton >= 1 and mainButton <= 12 end
    local bar, button = command:match("^MULTIACTIONBAR(%d+)BUTTON(%d+)$")
    if bar and button then
        bar, button = tonumber(bar), tonumber(button)
        return bar >= 1 and bar <= 7 and button >= 1 and button <= 12
    end
    return command:match("^SPELL .+") ~= nil or command:match("^CLICK [%w_]+:[^%s]+$") ~= nil
end

local function validClickBindingButton(button)
    if button == "LeftButton" or button == "RightButton" or button == "MiddleButton" then
        return true
    end
    if type(button) ~= "string" then return false end
    local buttonNumber = tonumber(button:match("^Button(%d+)$"))
    return buttonNumber ~= nil and buttonNumber >= 1 and buttonNumber <= 31
        and buttonNumber % 1 == 0
end

local function validateStoredTransaction(transaction)
    if type(transaction) ~= "table"
        or type(transaction.version) ~= "number"
        or transaction.version < oldestSupportedTransactionVersion
        or transaction.version > transactionVersion
        or transaction.version % 1 ~= 0
        or not validActiveStatuses[transaction.status]
        or not validStoredString(transaction.id, 300)
        or (transaction.bindingSet ~= 1 and transaction.bindingSet ~= 2)
        or type(transaction.context) ~= "table"
        or not validStoredString(transaction.context.characterKey, 200)
        or not validStoredString(transaction.context.class, 30)
        or type(transaction.context.specializationID) ~= "number"
        or transaction.context.specializationID < 0
        or transaction.context.specializationID % 1 ~= 0
        or not validOptionalStoredString(transaction.context.specialization, 200)
        or not validStoredString(transaction.context.stateKey, 200)
        or not validOptionalStoredString(transaction.context.formName, 200)
        or (transaction.context.activeConfigID ~= nil
            and (type(transaction.context.activeConfigID) ~= "number"
                or transaction.context.activeConfigID < 0
                or transaction.context.activeConfigID % 1 ~= 0))
        or (transaction.context.activeHeroSpecID ~= nil
            and (type(transaction.context.activeHeroSpecID) ~= "number"
                or transaction.context.activeHeroSpecID < 0
                or transaction.context.activeHeroSpecID % 1 ~= 0))
        or not validStoredString(transaction.context.profileID, 200)
        or type(transaction.changesBindings) ~= "boolean"
        or not validDenseArray(transaction.entries)
        or not validDenseArray(transaction.uiEntries)
        or #transaction.entries + #transaction.uiEntries > 1024
    then
        return false, "The saved transaction header is invalid"
    end

    local seenTargets = {}
    for _, entry in ipairs(transaction.entries) do
        if type(entry) ~= "table" or type(entry.target) ~= "table"
            or type(entry.before) ~= "table" or type(entry.desired) ~= "table"
            or type(entry.beforeFingerprint) ~= "string"
            or not validStoredAction(entry.desired, false)
            or entry.desired.actionType ~= "spell"
        then
            return false, "A saved action change is invalid"
        end
        local target = entry.target
        local key
        if target.kind == "slot" then
            local maximumSlot = tonumber(MAX_ACTION_BUTTONS) or 180
            if type(target.slot) ~= "number" or target.slot % 1 ~= 0
                or target.slot < 1 or target.slot > maximumSlot
                or (target.inputKey ~= nil and not validStoredString(target.inputKey, 100))
                or not validManagedBindingCommand(target.expectedBindingCommand)
                or not validStoredAction(entry.before, true)
            then
                return false, "A saved action slot is invalid"
            end
            local expectedBeforeFingerprint = actionFingerprint(entry.before)
                .. "\030binding:" .. tostring(entry.before.bindingCommand or "")
            if entry.beforeFingerprint ~= expectedBeforeFingerprint then
                return false, "A saved action before-state is inconsistent"
            end
            key = "slot:" .. tostring(target.slot)
        elseif target.kind == "binding" then
            if not validStoredString(target.key, 100)
                or type(entry.before.command) ~= "string" or #entry.before.command > 500
                or entry.before.command:find("%c")
                or entry.beforeFingerprint ~= "binding:" .. entry.before.command
            then
                return false, "A saved direct binding is invalid"
            end
            key = "binding:" .. target.key
        elseif target.kind == "click-binding" and transaction.version >= 3 then
            local beforeType = entry.before.clickBindingType
            local beforeActionID = entry.before.actionID
            local beforeEmpty = beforeType == nil and beforeActionID == nil
            if not validStoredString(target.button, 30)
                or target.inputKey ~= "CLICKCAST:" .. tostring(target.modifiers)
                    .. ":" .. tostring(target.button)
                or not validClickBindingButton(target.button)
                or type(target.modifiers) ~= "number" or target.modifiers < 0
                or target.modifiers > 31 or target.modifiers % 1 ~= 0
                or not validOptionalStoredString(entry.before.actionName, 300)
                or (transaction.version >= 5 and target.preserveClickIdentity ~= true)
                or (transaction.version < 5 and target.preserveClickIdentity ~= nil)
                or (beforeType == (Enum and Enum.ClickBindingType
                        and Enum.ClickBindingType.Macro or 2) and transaction.version >= 5
                    and (not validStoredString(entry.before.macroName, 300)
                        or type(entry.before.macroBody) ~= "string"
                        or #entry.before.macroBody > 10000))
                or (transaction.version >= 5
                    and beforeType ~= (Enum and Enum.ClickBindingType
                        and Enum.ClickBindingType.Macro or 2)
                    and (entry.before.macroName ~= nil or entry.before.macroBody ~= nil))
                or (not beforeEmpty and (
                    type(beforeType) ~= "number" or beforeType < 1 or beforeType > 4
                    or beforeType % 1 ~= 0 or type(beforeActionID) ~= "number"
                    or beforeActionID <= 0 or beforeActionID % 1 ~= 0))
                or entry.beforeFingerprint ~= "click:" .. tostring(beforeType or "")
                    .. ":" .. tostring(beforeActionID or "")
                    .. (target.preserveClickIdentity and beforeType == (Enum
                        and Enum.ClickBindingType and Enum.ClickBindingType.Macro or 2)
                        and ("\031" .. tostring(entry.before.macroName or "")
                            .. "\031" .. tostring(entry.before.macroBody or "")) or "")
            then
                return false, "A saved Click Casting change is invalid"
            end
            key = "click:" .. tostring(target.modifiers) .. ":" .. target.button
        else
            return false, "A saved action target is invalid"
        end
        if seenTargets[key] then return false, "A saved action target is duplicated" end
        seenTargets[key] = true
    end

    local seenUI = {}
    for _, entry in ipairs(transaction.uiEntries) do
        if type(entry) ~= "table" or type(entry.before) ~= "table"
            or type(entry.desired) ~= "table"
        then
            return false, "A saved UI change is invalid"
        end
        local key = entry.kind
        if entry.kind == "edit-mode" then
            local installsLayout = transaction.version >= 2 and entry.desired.install == true
            if type(entry.before.index) ~= "number" or entry.before.index % 1 ~= 0
                or entry.before.index < 1 or entry.before.index > 100
                or not validStoredString(entry.before.name, 200)
                or not validStoredString(entry.desired.name, 200)
                or (installsLayout and (
                    (entry.desired.index ~= nil and (
                        type(entry.desired.index) ~= "number" or entry.desired.index % 1 ~= 0
                        or entry.desired.index < 1 or entry.desired.index > 100))
                    or (entry.desired.layoutType ~= 1 and entry.desired.layoutType ~= 2)
                    or not validStoredString(entry.desired.value, 750000)))
                or (not installsLayout and (
                    type(entry.desired.index) ~= "number" or entry.desired.index % 1 ~= 0
                    or entry.desired.index < 1 or entry.desired.index > 100))
                or (transaction.version >= 2 and entry.desired.install ~= nil
                    and type(entry.desired.install) ~= "boolean")
                or (transaction.version >= 2 and not installsLayout
                    and (entry.desired.layoutType ~= nil or entry.desired.value ~= nil))
            then
                return false, "A saved Edit Mode change is invalid"
            end
        elseif entry.kind == "cvar" then
            local beforeValue = tonumber(entry.before.value)
            local desiredValue = tonumber(entry.desired.value)
            local maximumValue = entry.cvar == "enableMultiActionBars" and 255
                or ((transaction.version >= 4 and (entry.cvar == "enableMouseoverCast"
                    or entry.cvar == "autoSelfCast")) and 1 or nil)
            if not maximumValue
                or not beforeValue or beforeValue < 0 or beforeValue > maximumValue
                or beforeValue % 1 ~= 0
                or not desiredValue or desiredValue < 0 or desiredValue > maximumValue
                or desiredValue % 1 ~= 0
            then
                return false, "A saved interface setting is invalid"
            end
            key = entry.kind .. ":" .. entry.cvar
        elseif entry.kind == "modified-click" and transaction.version >= 4 then
            local function validModifier(value)
                return value == "NONE" or value == "ALT" or value == "CTRL" or value == "SHIFT"
            end
            if (entry.modifiedClick ~= "MOUSEOVERCAST" and entry.modifiedClick ~= "SELFCAST"
                    and entry.modifiedClick ~= "FOCUSCAST")
                or not validModifier(entry.before.value)
                or not validModifier(entry.desired.value)
            then
                return false, "A saved cast modifier is invalid"
            end
            key = entry.kind .. ":" .. entry.modifiedClick
        elseif entry.kind == "cooldown-viewer" then
            if type(entry.before.value) ~= "string" or #entry.before.value > 200000
                or type(entry.desired.value) ~= "string" or #entry.desired.value > 200000
            then
                return false, "A saved Cooldown Manager change is invalid"
            end
        else
            return false, "A saved UI target is invalid"
        end
        if seenUI[key] then return false, "A saved UI target is duplicated" end
        seenUI[key] = true
    end
    local expectedChecksum = transactionPayloadChecksum(transaction)
    if transaction.payloadChecksum == nil then
        -- Transactions staged by earlier releases gain integrity protection
        -- only after their complete legacy structure has passed validation.
        transaction.payloadChecksum = expectedChecksum
    elseif type(transaction.payloadChecksum) ~= "number"
        or transaction.payloadChecksum ~= expectedChecksum
    then
        return false, "The saved transaction integrity check failed"
    end
    return true
end

local function initializeStorage(addon)
    if type(addon.db.layoutTransactions) ~= "table" then
        addon.db.layoutTransactions = {
            version = transactionVersion,
            history = {},
        }
    end
    addon.db.layoutTransactions = addon.db.layoutTransactions or {
        version = transactionVersion,
        history = {},
    }
    local storage = addon.db.layoutTransactions
    storage.version = transactionVersion
    storage.history = type(storage.history) == "table" and storage.history or {}
    local cleanHistory = {}
    for _, transaction in ipairs(storage.history) do
        if type(transaction) == "table" then table.insert(cleanHistory, transaction) end
    end
    while #cleanHistory > maximumHistory do table.remove(cleanHistory, 1) end
    storage.history = cleanHistory
    return storage
end

local function quarantineInvalidActive(addon, storage, reason)
    table.insert(storage.history, {
        version = transactionVersion,
        status = "invalid",
        error = reason,
        context = {},
        entries = {},
        uiEntries = {},
        closedAt = time(),
        closedAtText = date("%Y-%m-%d %H:%M:%S"),
    })
    while #storage.history > maximumHistory do table.remove(storage.history, 1) end
    storage.active = nil
    addon:SetStatus("Invalid layout recovery data was quarantined • No changes were made")
end

local function archiveActive(addon, status)
    local storage = initializeStorage(addon)
    local transaction = storage.active
    if not transaction then
        return
    end
    transaction.status = status or transaction.status
    transaction.closedAt = time()
    transaction.closedAtText = date("%Y-%m-%d %H:%M:%S")
    table.insert(storage.history, transaction)
    while #storage.history > maximumHistory do
        table.remove(storage.history, 1)
    end
    storage.active = nil
end

local function getContext(addon, snapshot)
    local identity = addon:GetCharacterIdentity()
    local barState = snapshot and snapshot.actionBarState or {}
    local spellbook = snapshot and snapshot.spellbook or {}
    return {
        characterKey = identity.key,
        class = identity.class,
        specializationID = identity.specializationID,
        specialization = identity.specialization,
        stateKey = barState.stateID or barState.stateKey,
        formName = barState.formName,
        overrideActive = barState.overrideActive and true or false,
        activeConfigID = spellbook.activeConfigID,
        activeHeroSpecID = spellbook.activeHeroSpecID,
        profileID = addon.destinationProfile and addon.destinationProfile.id
            or (addon.db and addon.db.activeProfile),
    }
end

local function sameContext(left, right)
    return left and right
        and left.characterKey == right.characterKey
        and left.class == right.class
        and left.specializationID == right.specializationID
        and left.stateKey == right.stateKey
        and left.activeConfigID == right.activeConfigID
        and left.activeHeroSpecID == right.activeHeroSpecID
        and left.profileID == right.profileID
end

local function validActionSlot(slot)
    local maximumSlot = tonumber(MAX_ACTION_BUTTONS) or 180
    return type(slot) == "number" and slot % 1 == 0
        and slot >= 1 and slot <= maximumSlot
end

local function destinationTarget(addon, snapshot, destination)
    local clickButton, clickModifiers
    if destination.kind == "click-binding" and destination.clickButton ~= nil then
        clickButton, clickModifiers = destination.clickButton, destination.clickModifiers
    elseif addon.ParseClickBindingInputKey then
        clickButton, clickModifiers = addon:ParseClickBindingInputKey(destination.inputKey)
    end
    if clickButton then
        if not C_ClickBindings or type(C_ClickBindings.GetProfileInfo) ~= "function"
            or type(C_ClickBindings.SetProfileByInfo) ~= "function"
            or type(C_ClickBindings.CanSpellBeClickBound) ~= "function"
        then
            return nil, destination.label .. " requires Blizzard Click Casting APIs"
        end
        if not addon.GetClickBindingProfile or type(addon:GetClickBindingProfile()) ~= "table" then
            return nil, destination.label .. " could not read the current Click Casting profile"
        end
        return {
            kind = "click-binding",
            button = clickButton,
            modifiers = clickModifiers,
            inputKey = destination.inputKey,
            preserveClickIdentity = true,
        }
    end
    if destination.inputKey then
        local inspected = snapshot.destinations and snapshot.destinations[destination.key]
            or addon:InspectKey(destination.inputKey)
        local expectedCommand = destination.bindingCommand
        local fixedSlot, fixedSlotKind
        local effectiveExpectedSlot
        if expectedCommand and addon.GetFixedActionSlotForBinding then
            fixedSlot, fixedSlotKind = addon:GetFixedActionSlotForBinding(expectedCommand)
        end
        if expectedCommand and addon.GetEffectiveActionSlotForBinding then
            effectiveExpectedSlot = addon:GetEffectiveActionSlotForBinding(expectedCommand)
        end
        -- Main-bar buttons page with stances and shapeshift forms, so their
        -- visible frame action is authoritative. Multi-bars remain fixed even
        -- while hidden or before their frames are created.
        if fixedSlot and fixedSlotKind ~= "main" then
            return {
                kind = "slot",
                slot = fixedSlot,
                inputKey = destination.inputKey,
                expectedBindingCommand = expectedCommand,
            }
        end
        if fixedSlotKind == "main" then
            local buttonNumber = tonumber(expectedCommand and expectedCommand:match("^ACTIONBUTTON(%d+)$"))
            local firstSlot = snapshot.actionBarState and snapshot.actionBarState.effectiveFirstSlot
            local inferredPageSlot = type(firstSlot) == "number" and buttonNumber
                and (firstSlot + buttonNumber - 1) or nil
            local activeSlot = effectiveExpectedSlot or inferredPageSlot
            if type(activeSlot) == "number" then
                return {
                    kind = "slot",
                    slot = activeSlot,
                    inputKey = destination.inputKey,
                    expectedBindingCommand = expectedCommand,
                }
            end
        end
        if inspected and type(inspected.actionSlot) == "number" then
            return {
                kind = "slot",
                slot = inspected.actionSlot,
                inputKey = destination.inputKey,
                expectedBindingCommand = expectedCommand or inspected.bindingCommand,
            }
        end
        if fixedSlot then
            return {
                kind = "slot",
                slot = fixedSlot,
                inputKey = destination.inputKey,
                expectedBindingCommand = expectedCommand,
            }
        end
        if expectedCommand and not expectedCommand:match("^SPELL ") then
            return nil, destination.label .. " no longer resolves to its captured action-bar slot"
        end
        if type(SetBindingSpell) ~= "function" then
            return nil, destination.label .. " has no action slot and direct spell binding is unavailable"
        end
        return { kind = "binding", key = destination.inputKey }
    end

    if destination.button and destination.layer then
        local buttonData = snapshot.buttons and snapshot.buttons[destination.button]
        local action = buttonData and buttonData[destination.layer]
        if not action or type(action.actionSlot) ~= "number" then
            return nil, "Destination " .. destination.label .. " has no readable action slot"
        end
        return { kind = "slot", slot = action.actionSlot }
    end

    local key = destination.key and destination.key:match("^special:(.+)$")
    if not key then
        return nil, "Destination " .. tostring(destination.label) .. " is not supported"
    end
    local inspected = addon:InspectKey(key)
    if inspected and type(inspected.actionSlot) == "number" then
        return { kind = "slot", slot = inspected.actionSlot, key = key }
    end
    if type(SetBindingSpell) ~= "function" then
        return nil, key .. " has no action slot and direct spell binding is unavailable"
    end
    return { kind = "binding", key = key }
end

local function targetKey(target)
    if target.kind == "click-binding" then
        return target.kind .. ":" .. tostring(target.modifiers) .. ":" .. tostring(target.button)
    end
    return target.kind .. ":" .. tostring(target.slot or target.key)
end

local function findClickBinding(addon, target)
    local profile = addon.GetClickBindingProfile and addon:GetClickBindingProfile()
    if type(profile) ~= "table" then return nil, nil end
    for index, binding in ipairs(profile) do
        if binding.button == target.button and binding.modifiers == target.modifiers then
            return binding, index, profile
        end
    end
    return nil, nil, profile
end

local function clickBindingFingerprint(binding, macroName, macroBody)
    local fingerprint = "click:" .. tostring(binding and binding.type or "")
        .. ":" .. tostring(binding and binding.actionID or "")
    local macroType = Enum and Enum.ClickBindingType and Enum.ClickBindingType.Macro or 2
    if binding and binding.type == macroType and macroName ~= nil then
        fingerprint = fingerprint .. "\031" .. tostring(macroName)
            .. "\031" .. tostring(macroBody or "")
    end
    return fingerprint
end

local function captureTarget(addon, target)
    if target.kind == "slot" then
        local action = copyAction(addon:DescribeActionSlot(target.slot))
        if target.inputKey then
            action.bindingCommand = GetBindingAction(target.inputKey) or ""
        end
        return action, actionFingerprint(action) .. "\030binding:" .. tostring(action.bindingCommand or "")
    end
    if target.kind == "click-binding" then
        local binding = findClickBinding(addon, target)
        local inspected = addon.InspectClickBinding
            and addon:InspectClickBinding(target.inputKey) or {}
        return {
            clickBindingType = binding and binding.type,
            actionID = binding and binding.actionID,
            actionName = inspected and inspected.actionName,
            macroName = target.preserveClickIdentity and inspected and inspected.macroName or nil,
            macroBody = target.preserveClickIdentity and inspected and inspected.macroBody or nil,
        }, clickBindingFingerprint(binding,
            target.preserveClickIdentity and inspected and inspected.macroName or nil,
            target.preserveClickIdentity and inspected and inspected.macroBody or nil)
    end
    local command = GetBindingAction(target.key) or ""
    return { command = command }, "binding:" .. command
end

local function targetMatchesBefore(addon, entry)
    local _, fingerprint = captureTarget(addon, entry.target)
    return fingerprint == entry.beforeFingerprint
end

local function targetMatchesDesired(addon, entry)
    if entry.target.kind == "slot" then
        local actionMatches = desiredMatches(copyAction(addon:DescribeActionSlot(entry.target.slot)), entry.desired)
        local bindingMatches = not entry.target.expectedBindingCommand
            or (GetBindingAction(entry.target.inputKey) or "") == entry.target.expectedBindingCommand
        return actionMatches and bindingMatches
    end
    if entry.target.kind == "click-binding" then
        local binding = findClickBinding(addon, entry.target)
        local spellType = Enum and Enum.ClickBindingType and Enum.ClickBindingType.Spell or 1
        local desiredSpellID = entry.desired.baseSpellID or entry.desired.actionID
        return binding and binding.type == spellType and binding.actionID == desiredSpellID
    end
    local command = GetBindingAction(entry.target.key) or ""
    return command == entry.appliedBindingCommand
        or command == "SPELL " .. tostring(entry.desired.actionName)
end

local function setClickBindingTarget(addon, target, replacement)
    if not C_ClickBindings or type(C_ClickBindings.SetProfileByInfo) ~= "function" then
        return false, "Click Casting profile updates are unavailable"
    end
    local _, index, profile = findClickBinding(addon, target)
    if type(profile) ~= "table" then
        return false, "The current Click Casting profile could not be read"
    end
    if index and replacement then
        profile[index] = replacement
    elseif index then
        table.remove(profile, index)
    elseif replacement then
        table.insert(profile, replacement)
    end
    local updated, updateResult = pcall(C_ClickBindings.SetProfileByInfo, profile)
    if not updated or updateResult == false then
        return false, "The Click Casting profile could not be changed"
    end
    local current = findClickBinding(addon, target)
    if clickBindingFingerprint(current) ~= clickBindingFingerprint(replacement) then
        return false, "The Click Casting input did not match its requested state"
    end
    return true
end

local function applyEntry(addon, entry)
    if entry.target.kind == "slot" then
        if entry.target.expectedBindingCommand
            and (GetBindingAction(entry.target.inputKey) or "") ~= entry.target.expectedBindingCommand
        then
            local bound, bindingResult = pcall(SetBinding, entry.target.inputKey, entry.target.expectedBindingCommand)
            if not bound or bindingResult == false then
                return false, entry.target.inputKey .. " could not be restored to its captured action-bar binding"
            end
        end
        local current = copyAction(addon:DescribeActionSlot(entry.target.slot))
        if desiredMatches(current, entry.desired) then
            return true
        end
        return placeSpell(addon, entry.target.slot, entry.desired)
    end
    if entry.target.kind == "click-binding" then
        local desiredSpellID = entry.desired.baseSpellID or entry.desired.actionID
        local bindable, canBind = pcall(C_ClickBindings.CanSpellBeClickBound, desiredSpellID)
        if not bindable or not canBind then
            return false, tostring(entry.desired.actionName) .. " cannot be used for Click Casting"
        end
        local spellType = Enum and Enum.ClickBindingType and Enum.ClickBindingType.Spell or 1
        return setClickBindingTarget(addon, entry.target, {
            type = spellType,
            actionID = desiredSpellID,
            button = entry.target.button,
            modifiers = entry.target.modifiers,
        })
    end
    local ok, result = pcall(SetBindingSpell, entry.target.key, entry.desired.actionName)
    if not ok or result == false then
        return false, entry.target.key .. " could not be bound to " .. entry.desired.actionName
    end
    entry.appliedBindingCommand = GetBindingAction(entry.target.key) or ""
    if entry.appliedBindingCommand == "" then
        return false, entry.target.key .. " did not receive a spell binding"
    end
    return true
end

local function restoreEntry(addon, entry)
    if entry.target.kind == "slot" then
        local current = copyAction(addon:DescribeActionSlot(entry.target.slot))
        if actionFingerprint(current) ~= actionFingerprint(entry.before) then
            local restored, restoreError = restoreSlot(addon, entry.target.slot, entry.before)
            if not restored then
                return false, restoreError
            end
        end
        if entry.target.inputKey
            and (GetBindingAction(entry.target.inputKey) or "") ~= entry.before.bindingCommand
        then
            local command = entry.before.bindingCommand ~= "" and entry.before.bindingCommand or nil
            local bound, bindingResult = pcall(SetBinding, entry.target.inputKey, command)
            if not bound or bindingResult == false then
                return false, entry.target.inputKey .. " could not be restored"
            end
        end
        return true
    end
    if entry.target.kind == "click-binding" then
        local macroType = Enum and Enum.ClickBindingType and Enum.ClickBindingType.Macro or 2
        if entry.target.preserveClickIdentity
            and entry.before.clickBindingType == macroType
        then
            local macroName, macroBody
            if GetMacroInfo then
                macroName, _, macroBody = GetMacroInfo(entry.before.actionID)
            end
            if macroName ~= entry.before.macroName or macroBody ~= entry.before.macroBody then
                return false, "The previous Click Casting macro changed and cannot be restored exactly"
            end
        end
        local replacement
        if entry.before.clickBindingType then
            replacement = {
                type = entry.before.clickBindingType,
                actionID = entry.before.actionID,
                button = entry.target.button,
                modifiers = entry.target.modifiers,
            }
        end
        local restored, restoreError = setClickBindingTarget(addon, entry.target, replacement)
        if not restored then return false, restoreError end
        if not targetMatchesBefore(addon, entry) then
            return false, "The previous Click Casting input could not be verified"
        end
        return true
    end
    local command = entry.before.command ~= "" and entry.before.command or nil
    local ok, result = pcall(SetBinding, entry.target.key, command)
    if not ok or result == false then
        return false, entry.target.key .. " could not be restored"
    end
    if (GetBindingAction(entry.target.key) or "") ~= entry.before.command then
        return false, entry.target.key .. " did not match its saved binding"
    end
    return true
end

local function saveBindings(transaction)
    if not transaction.changesBindings then
        return true
    end
    local ok, result = pcall(SaveBindings, transaction.bindingSet)
    return ok and result ~= false
end

local function getEditModeState(addon)
    local audit = addon.GetEditModeAudit and addon:GetEditModeAudit() or {}
    local result = {
        index = audit.activeIndex,
        name = audit.activeName,
        layouts = {},
    }
    if C_EditMode and C_EditMode.GetLayouts then
        local ok, layoutInfo = pcall(C_EditMode.GetLayouts)
        if ok and layoutInfo and layoutInfo.layouts then
            result.index = layoutInfo.activeLayout or result.index
            result.layouts = layoutInfo.layouts
            local active = result.index and layoutInfo.layouts[result.index]
            result.name = active and active.layoutName or result.name
        end
    end
    return result
end

local function copyTableValue(value, seen)
    if type(value) ~= "table" then return value end
    seen = seen or {}
    if seen[value] then return seen[value] end
    local copied = {}
    seen[value] = copied
    for key, nested in pairs(value) do
        copied[copyTableValue(key, seen)] = copyTableValue(nested, seen)
    end
    return copied
end

local function getCVarValue(name)
    if type(GetCVar) ~= "function" then return nil end
    local ok, value = pcall(GetCVar, name)
    return ok and value or nil
end

local function getModifiedClickValue(name, defaultValue)
    if type(GetModifiedClick) ~= "function" then return nil end
    local ok, value = pcall(GetModifiedClick, name)
    if not ok then return nil end
    local defaults = { MOUSEOVERCAST = "NONE", SELFCAST = "ALT", FOCUSCAST = "NONE" }
    value = value or defaultValue or defaults[name]
    if value == "NONE" or value == "ALT" or value == "CTRL" or value == "SHIFT" then
        return value
    end
end

local function binaryDisplay(value)
    return tostring(value) == "1" and "Enabled" or "Disabled"
end

local function modifierDisplay(value)
    return value == "NONE" and "None"
        or (value and value:sub(1, 1) .. value:sub(2):lower()) or "Unavailable"
end

local function uiEntryLabel(entry)
    if entry.label then return entry.label end
    local labels = {
        enableMultiActionBars = "Visible action bars",
        enableMouseoverCast = "Mouseover casting",
        autoSelfCast = "Automatic self cast",
        MOUSEOVERCAST = "Mouseover cast modifier",
        SELFCAST = "Self-cast modifier",
        FOCUSCAST = "Focus-cast modifier",
    }
    return labels[entry.cvar or entry.modifiedClick] or "Interface preference"
end

local function multiBarDisplay(value)
    local numeric = tonumber(value)
    if not numeric then return value ~= nil and tostring(value) or "Unavailable" end
    if numeric == 0 then return "Main bar only" end
    local count = 0
    while numeric > 0 do
        if numeric % 2 == 1 then count = count + 1 end
        numeric = math.floor(numeric / 2)
    end
    return countLabel(count, "extra action bar")
end

local function findEditModeLayoutIndex(state, layoutName)
    for index, layout in ipairs(state and state.layouts or {}) do
        if layout.layoutName == layoutName then
            return index, layout
        end
    end
end

local function accountEditModeLayoutType()
    return Enum and Enum.EditModeLayoutType and Enum.EditModeLayoutType.Account or 1
end

local function characterEditModeLayoutType()
    return Enum and Enum.EditModeLayoutType and Enum.EditModeLayoutType.Character or 2
end

local function isUserEditModeLayoutType(layoutType)
    return layoutType == accountEditModeLayoutType() or layoutType == characterEditModeLayoutType()
end

local function maximumEditModeLayoutsPerType()
    local maximum = Constants and Constants.EditModeConsts
        and tonumber(Constants.EditModeConsts.EditModeMaxLayoutsPerType)
    return maximum and maximum > 0 and maximum or 5
end

local function countEditModeLayoutsOfType(state, layoutType)
    local count = 0
    for _, layout in ipairs(state and state.layouts or {}) do
        if layout.layoutType == layoutType then count = count + 1 end
    end
    return count
end

local function editModeImportAPIsAvailable()
    return C_EditMode
        and type(C_EditMode.ConvertStringToLayoutInfo) == "function"
        and type(C_EditMode.ConvertLayoutInfoToString) == "function"
        and type(C_EditMode.GetLayouts) == "function"
        and type(C_EditMode.IsValidLayoutName) == "function"
        and type(C_EditMode.SaveLayouts) == "function"
        and type(C_EditMode.OnLayoutAdded) == "function"
        and type(C_EditMode.OnLayoutDeleted) == "function"
        and type(C_EditMode.SetActiveLayout) == "function"
end

local function serializeEditModeLayout(layout)
    if not C_EditMode or type(C_EditMode.ConvertLayoutInfoToString) ~= "function" then return nil end
    local ok, serialized = pcall(C_EditMode.ConvertLayoutInfoToString, layout)
    if not ok or type(serialized) ~= "string" or serialized == "" or #serialized > 750000 then
        return nil
    end
    return serialized
end

local function parseEditModeLayout(layoutString)
    if not C_EditMode or type(C_EditMode.ConvertStringToLayoutInfo) ~= "function"
        or type(layoutString) ~= "string" or layoutString == "" or #layoutString > 750000
    then
        return nil
    end
    local ok, layout = pcall(C_EditMode.ConvertStringToLayoutInfo, layoutString)
    if not ok or type(layout) ~= "table" or type(layout.systems) ~= "table" then return nil end
    return layout
end

local function prepareEditModeImport(state, layoutName, layoutString)
    if not editModeImportAPIsAvailable() then
        return nil, "Edit Mode layout installation is unavailable"
    end
    if type(layoutString) ~= "string" or layoutString == "" then
        return nil, "The captured Edit Mode layout is not installed and this profile has no portable layout data"
    end
    local validNameCall, validName = pcall(C_EditMode.IsValidLayoutName, layoutName)
    if not validNameCall or not validName then
        return nil, "The captured Edit Mode layout name is not accepted by this client"
    end
    local layout = parseEditModeLayout(layoutString)
    if not layout then
        return nil, "The captured Edit Mode layout data is invalid for this client"
    end
    local layoutType = isUserEditModeLayoutType(layout.layoutType)
        and layout.layoutType or accountEditModeLayoutType()
    if countEditModeLayoutsOfType(state, layoutType) >= maximumEditModeLayoutsPerType() then
        local scope = layoutType == characterEditModeLayoutType() and "character" or "account"
        return nil, "The " .. scope .. " Edit Mode layout limit is full"
    end
    layout.layoutName = layoutName
    layout.layoutType = layoutType
    local canonical = serializeEditModeLayout(layout)
    if not canonical then
        return nil, "The captured Edit Mode layout could not be prepared safely"
    end
    local roundTrip = parseEditModeLayout(canonical)
    if not roundTrip or roundTrip.layoutName ~= layoutName or roundTrip.layoutType ~= layoutType then
        return nil, "The captured Edit Mode layout failed validation"
    end
    return {
        index = nil,
        name = layoutName,
        value = canonical,
        layoutType = layoutType,
        install = true,
        display = "Install and use " .. layoutName,
    }
end

local function setCVarValue(name, value)
    if C_CVar and C_CVar.SetCVar then
        return C_CVar.SetCVar(name, value)
    end
    if SetCVar then
        return SetCVar(name, value)
    end
end

local function getCooldownViewerLayout()
    if not C_CooldownViewer or not C_CooldownViewer.GetLayoutData then return nil end
    local ok, value = pcall(C_CooldownViewer.GetLayoutData)
    return ok and type(value) == "string" and value or nil
end

local function buildUIEntries(addon)
    local result = {}
    local uiProfile = addon.destinationProfile and addon.destinationProfile.ui or {}
    if uiProfile.editModeLayoutName then
        local current = getEditModeState(addon)
        local desiredIndex, desiredLayout = findEditModeLayoutIndex(current, uiProfile.editModeLayoutName)
        local installedLayoutError
        if desiredLayout and uiProfile.editModeLayoutString then
            local expectedLayout = parseEditModeLayout(uiProfile.editModeLayoutString)
            if not expectedLayout then
                installedLayoutError = "The captured Edit Mode layout data is invalid for this client"
            else
                expectedLayout.layoutName = uiProfile.editModeLayoutName
                expectedLayout.layoutType = isUserEditModeLayoutType(expectedLayout.layoutType)
                    and expectedLayout.layoutType or accountEditModeLayoutType()
                local expectedString = serializeEditModeLayout(expectedLayout)
                if not expectedString or expectedLayout.layoutType ~= desiredLayout.layoutType
                    or expectedString ~= serializeEditModeLayout(desiredLayout)
                then
                    installedLayoutError = "An installed Edit Mode layout with this name has different settings"
                end
            end
        end
        if current.name ~= uiProfile.editModeLayoutName or installedLayoutError then
            local entry = {
                kind = "edit-mode",
                label = "Edit Mode layout",
                before = { index = current.index, name = current.name },
                desired = {
                    index = desiredIndex,
                    name = uiProfile.editModeLayoutName,
                    install = false,
                },
            }
            if installedLayoutError then
                entry.blockedReason = installedLayoutError
            elseif type(current.index) ~= "number" or type(current.name) ~= "string"
                or current.name == "" then
                entry.blockedReason = "The current Edit Mode layout cannot be saved for Undo"
            elseif not C_EditMode or not C_EditMode.SetActiveLayout then
                entry.blockedReason = "Edit Mode layout switching is unavailable"
            elseif not desiredIndex then
                local prepared, prepareError = prepareEditModeImport(
                    current, uiProfile.editModeLayoutName, uiProfile.editModeLayoutString)
                if prepared then
                    entry.desired = prepared
                else
                    entry.blockedReason = prepareError
                end
            end
            table.insert(result, entry)
        end
    end
    if uiProfile.multiActionBars ~= nil then
        local current = getCVarValue("enableMultiActionBars")
        if tostring(current) ~= tostring(uiProfile.multiActionBars) then
            local entry = {
                kind = "cvar",
                label = "Visible action bars",
                cvar = "enableMultiActionBars",
                before = { value = current, display = multiBarDisplay(current) },
                desired = { value = uiProfile.multiActionBars, display = multiBarDisplay(uiProfile.multiActionBars) },
            }
            if not (C_CVar and C_CVar.SetCVar) and not SetCVar then
                entry.blockedReason = "Action-bar visibility settings are unavailable"
            elseif not tonumber(current) or tonumber(current) < 0 or tonumber(current) > 255
                or tonumber(current) % 1 ~= 0
            then
                entry.blockedReason = "The current action-bar visibility cannot be saved for Undo"
            end
            table.insert(result, entry)
        end
    end
    local function addBinaryCVar(field, cvar, label)
        local desired = uiProfile[field]
        if desired == nil then return end
        local current = getCVarValue(cvar)
        if tostring(current) == tostring(desired) then return end
        local entry = {
            kind = "cvar",
            label = label,
            cvar = cvar,
            before = { value = current, display = current ~= nil and binaryDisplay(current) or "Unavailable" },
            desired = { value = desired, display = binaryDisplay(desired) },
        }
        if (tostring(current) ~= "0" and tostring(current) ~= "1")
            or (not (C_CVar and C_CVar.SetCVar) and not SetCVar)
        then
            entry.blockedReason = label .. " is unavailable"
        end
        table.insert(result, entry)
    end
    local function addModifiedClick(field, modifiedClick, defaultValue, label)
        local desired = uiProfile[field]
        if desired == nil then return end
        local current = getModifiedClickValue(modifiedClick, defaultValue)
        if current == desired then return end
        local entry = {
            kind = "modified-click",
            label = label,
            modifiedClick = modifiedClick,
            before = { value = current, display = modifierDisplay(current) },
            desired = { value = desired, display = modifierDisplay(desired) },
        }
        if current == nil or type(SetModifiedClick) ~= "function"
            or type(SaveBindings) ~= "function"
        then
            entry.blockedReason = label .. " is unavailable"
        end
        table.insert(result, entry)
    end
    addBinaryCVar("enableMouseoverCast", "enableMouseoverCast", "Mouseover casting")
    addModifiedClick("mouseoverCastModifier", "MOUSEOVERCAST", "NONE", "Mouseover cast modifier")
    addBinaryCVar("autoSelfCast", "autoSelfCast", "Automatic self cast")
    addModifiedClick("selfCastModifier", "SELFCAST", "ALT", "Self-cast modifier")
    addModifiedClick("focusCastModifier", "FOCUSCAST", "NONE", "Focus-cast modifier")
    if uiProfile.cooldownViewerLayout ~= nil then
        local current = getCooldownViewerLayout()
        if current ~= uiProfile.cooldownViewerLayout then
            local entry = {
                kind = "cooldown-viewer",
                label = "Cooldown Manager setup",
                before = { value = current, display = current and "Current setup" or "Unavailable" },
                desired = { value = uiProfile.cooldownViewerLayout, display = "Profile setup" },
            }
            if not C_CooldownViewer
                or not C_CooldownViewer.GetLayoutData
                or not C_CooldownViewer.SetLayoutData
                or current == nil
            then
                entry.blockedReason = "Cooldown Manager layout settings are unavailable"
            end
            table.insert(result, entry)
        end
    end
    return result
end

local function editModeLayoutInsertionIndex(state, layoutType)
    local highestPreset, highestAccount, highestCharacter = 0, nil, nil
    for index, layout in ipairs(state and state.layouts or {}) do
        if layout.layoutType == accountEditModeLayoutType() then
            highestAccount = index
        elseif layout.layoutType == characterEditModeLayoutType() then
            highestCharacter = index
        elseif not isUserEditModeLayoutType(layout.layoutType) then
            highestPreset = index
        end
    end
    if layoutType == characterEditModeLayoutType() then
        return (highestCharacter or highestAccount or highestPreset) + 1
    end
    return (highestAccount or highestPreset) + 1
end

local function installEditModeLayout(addon, entry)
    if not editModeImportAPIsAvailable() then
        return false, "Edit Mode layout installation is unavailable"
    end
    local current = getEditModeState(addon)
    local existingIndex, existingLayout = findEditModeLayoutIndex(current, entry.desired.name)
    if existingIndex then
        local existingString = serializeEditModeLayout(existingLayout)
        if existingString ~= entry.desired.value
            or existingLayout.layoutType ~= entry.desired.layoutType
        then
            return false, "An Edit Mode layout with that name now contains different settings"
        end
        entry.desired.index = existingIndex
    else
        local layout = parseEditModeLayout(entry.desired.value)
        if not layout or layout.layoutName ~= entry.desired.name
            or layout.layoutType ~= entry.desired.layoutType
            or serializeEditModeLayout(layout) ~= entry.desired.value
        then
            return false, "The saved Edit Mode layout failed its integrity check"
        end
        if countEditModeLayoutsOfType(current, entry.desired.layoutType)
            >= maximumEditModeLayoutsPerType()
        then
            return false, "The Edit Mode layout limit became full after preview"
        end
        local insertionIndex = editModeLayoutInsertionIndex(current, entry.desired.layoutType)
        local saveInfo = {
            activeLayout = current.index,
            layouts = copyTableValue(current.layouts),
        }
        if insertionIndex <= saveInfo.activeLayout then
            saveInfo.activeLayout = saveInfo.activeLayout + 1
        end
        table.insert(saveInfo.layouts, insertionIndex, layout)
        local saved, saveResult = pcall(C_EditMode.SaveLayouts, saveInfo)
        if not saved or saveResult == false then
            return false, "The Edit Mode layout could not be installed"
        end
        pcall(C_EditMode.OnLayoutAdded, insertionIndex, false, true)
        current = getEditModeState(addon)
        existingIndex, existingLayout = findEditModeLayoutIndex(current, entry.desired.name)
        if not existingIndex or existingLayout.layoutType ~= entry.desired.layoutType
            or serializeEditModeLayout(existingLayout) ~= entry.desired.value
        then
            return false, "The installed Edit Mode layout did not match the profile"
        end
        entry.desired.index = existingIndex
    end

    local activated, activationResult = pcall(C_EditMode.SetActiveLayout, entry.desired.index)
    if not activated or activationResult == false then
        return false, "The installed Edit Mode layout could not be activated"
    end
    current = getEditModeState(addon)
    local activeIndex, activeLayout = findEditModeLayoutIndex(current, entry.desired.name)
    if current.name ~= entry.desired.name or activeIndex ~= current.index
        or not activeLayout or activeLayout.layoutType ~= entry.desired.layoutType
        or serializeEditModeLayout(activeLayout) ~= entry.desired.value
    then
        return false, "The installed Edit Mode layout could not be verified"
    end
    entry.desired.index = activeIndex
    return true
end

local function removeInstalledEditModeLayout(addon, entry)
    if not editModeImportAPIsAvailable() then
        return false, "Edit Mode layout removal is unavailable"
    end
    local current = getEditModeState(addon)
    local previousIndex = findEditModeLayoutIndex(current, entry.before.name)
    if not previousIndex then
        return false, "The previous Edit Mode layout is no longer available"
    end
    if current.name ~= entry.before.name then
        local activated, activationResult = pcall(C_EditMode.SetActiveLayout, previousIndex)
        if not activated or activationResult == false then
            return false, "The previous Edit Mode layout could not be restored"
        end
    end

    current = getEditModeState(addon)
    previousIndex = findEditModeLayoutIndex(current, entry.before.name)
    local installedIndex, installedLayout = findEditModeLayoutIndex(current, entry.desired.name)
    if not installedIndex then
        if current.name == entry.before.name then return true end
        return false, "The previous Edit Mode layout could not be verified"
    end
    if current.name ~= entry.before.name or installedIndex == current.index
        or not installedLayout or installedLayout.layoutType ~= entry.desired.layoutType
        or serializeEditModeLayout(installedLayout) ~= entry.desired.value
    then
        return false, "The installed Edit Mode layout changed and was not removed"
    end

    local saveInfo = {
        activeLayout = current.index,
        layouts = copyTableValue(current.layouts),
    }
    table.remove(saveInfo.layouts, installedIndex)
    if installedIndex < saveInfo.activeLayout then
        saveInfo.activeLayout = saveInfo.activeLayout - 1
    end
    local saved, saveResult = pcall(C_EditMode.SaveLayouts, saveInfo)
    if not saved or saveResult == false then
        return false, "The installed Edit Mode layout could not be removed"
    end
    pcall(C_EditMode.OnLayoutDeleted, installedIndex)
    current = getEditModeState(addon)
    entry.before.index = findEditModeLayoutIndex(current, entry.before.name)
    if current.name ~= entry.before.name
        or findEditModeLayoutIndex(current, entry.desired.name) ~= nil
    then
        return false, "The previous Edit Mode layout could not be verified"
    end
    return true
end

local function uiEntryMatches(addon, entry, state)
    if entry.kind == "edit-mode" then
        local current = getEditModeState(addon)
        if current.name ~= state.name then return false end
        if entry.desired.install and state == entry.before then
            return findEditModeLayoutIndex(current, entry.desired.name) == nil
        end
        if state.install then
            local index, layout = findEditModeLayoutIndex(current, state.name)
            return index == current.index and layout
                and layout.layoutType == state.layoutType
                and serializeEditModeLayout(layout) == state.value
        end
        return true
    elseif entry.kind == "cvar" then
        return tostring(getCVarValue(entry.cvar)) == tostring(state.value)
    elseif entry.kind == "modified-click" then
        return getModifiedClickValue(entry.modifiedClick) == state.value
    elseif entry.kind == "cooldown-viewer" then
        return getCooldownViewerLayout() == state.value
    end
    return false
end

local function applyUIEntry(addon, entry)
    if entry.kind == "edit-mode" then
        if entry.desired.install then
            return installEditModeLayout(addon, entry)
        end
        local current = getEditModeState(addon)
        local desiredIndex = findEditModeLayoutIndex(current, entry.desired.name)
        if not desiredIndex then
            return false, "Edit Mode layout " .. tostring(entry.desired.name) .. " is no longer installed"
        end
        entry.desired.index = desiredIndex
        local ok = pcall(C_EditMode.SetActiveLayout, desiredIndex)
        if not ok or not uiEntryMatches(addon, entry, entry.desired) then
            return false, "Edit Mode layout could not be changed to " .. tostring(entry.desired.name)
        end
        return true
    elseif entry.kind == "cvar" then
        local ok = pcall(setCVarValue, entry.cvar, entry.desired.value)
        if not ok or not uiEntryMatches(addon, entry, entry.desired) then
            return false, uiEntryLabel(entry) .. " could not be changed"
        end
        return true
    elseif entry.kind == "modified-click" then
        if type(SetModifiedClick) ~= "function" then
            return false, uiEntryLabel(entry) .. " is unavailable"
        end
        local ok, result = pcall(SetModifiedClick, entry.modifiedClick, entry.desired.value)
        if not ok or result == false or not uiEntryMatches(addon, entry, entry.desired) then
            return false, uiEntryLabel(entry) .. " could not be changed"
        end
        return true
    elseif entry.kind == "cooldown-viewer" then
        if not C_CooldownViewer or not C_CooldownViewer.SetLayoutData then
            return false, "Cooldown Manager layout settings are unavailable"
        end
        local ok = pcall(C_CooldownViewer.SetLayoutData, entry.desired.value)
        if not ok or not uiEntryMatches(addon, entry, entry.desired) then
            return false, "Cooldown Manager setup could not be changed"
        end
        return true
    end
    return false, "Unknown UI preference"
end

local function restoreUIEntry(addon, entry)
    if entry.kind == "edit-mode" then
        if entry.desired.install then
            return removeInstalledEditModeLayout(addon, entry)
        end
        local current = getEditModeState(addon)
        local previousIndex = findEditModeLayoutIndex(current, entry.before.name)
        if not previousIndex then
            return false, "The previous Edit Mode layout is no longer available"
        end
        entry.before.index = previousIndex
        local ok = pcall(C_EditMode.SetActiveLayout, previousIndex)
        if not ok or not uiEntryMatches(addon, entry, entry.before) then
            return false, "The previous Edit Mode layout could not be restored"
        end
        return true
    elseif entry.kind == "cvar" then
        local ok = pcall(setCVarValue, entry.cvar, entry.before.value)
        if not ok or not uiEntryMatches(addon, entry, entry.before) then
            return false, "The previous " .. uiEntryLabel(entry):lower() .. " could not be restored"
        end
        return true
    elseif entry.kind == "modified-click" then
        if type(SetModifiedClick) ~= "function" then
            return false, "The previous " .. uiEntryLabel(entry):lower() .. " is unavailable"
        end
        local ok, result = pcall(SetModifiedClick, entry.modifiedClick, entry.before.value)
        if not ok or result == false or not uiEntryMatches(addon, entry, entry.before) then
            return false, "The previous " .. uiEntryLabel(entry):lower() .. " could not be restored"
        end
        return true
    elseif entry.kind == "cooldown-viewer" then
        if entry.before.value == nil then
            return false, "The previous Cooldown Manager setup is unavailable"
        end
        if not C_CooldownViewer or not C_CooldownViewer.SetLayoutData then
            return false, "Cooldown Manager layout settings are unavailable"
        end
        local ok = pcall(C_CooldownViewer.SetLayoutData, entry.before.value)
        if not ok or not uiEntryMatches(addon, entry, entry.before) then
            return false, "The previous Cooldown Manager setup could not be restored"
        end
        return true
    end
    return false, "Unknown UI preference"
end

local function preflightTransactionRestore(addon, transaction, snapshot)
    if transaction.changesBindings and type(SaveBindings) ~= "function" then
        return false, "Binding persistence is unavailable"
    end
    for _, entry in ipairs(transaction.entries or {}) do
        if entry.target.kind == "slot" then
            if not targetMatchesBefore(addon, entry) then
                local mutationError = actionSlotMutationError()
                if mutationError then return false, mutationError end
                local restorable, restoreError = canRestoreAction(copyAction(entry.before), snapshot)
                if not restorable then return false, restoreError end
            end
            if entry.target.inputKey
                and (GetBindingAction(entry.target.inputKey) or "") ~= entry.before.bindingCommand
                and type(SetBinding) ~= "function"
            then
                return false, "Binding restore APIs are unavailable"
            end
        elseif entry.target.kind == "binding" then
            if type(SetBinding) ~= "function" then
                return false, "Binding restore APIs are unavailable"
            end
        elseif entry.target.kind == "click-binding" then
            if not C_ClickBindings or type(C_ClickBindings.SetProfileByInfo) ~= "function"
                or not addon.GetClickBindingProfile
                or type(addon:GetClickBindingProfile()) ~= "table"
            then
                return false, "Click Casting profile restore is unavailable"
            end
            local macroType = Enum and Enum.ClickBindingType
                and Enum.ClickBindingType.Macro or 2
            if entry.target.preserveClickIdentity
                and entry.before.clickBindingType == macroType
            then
                local macroName, macroBody
                if GetMacroInfo then
                    macroName, _, macroBody = GetMacroInfo(entry.before.actionID)
                end
                if macroName ~= entry.before.macroName or macroBody ~= entry.before.macroBody then
                    return false, "The previous Click Casting macro changed and cannot be restored exactly"
                end
            end
        end
    end
    for _, entry in ipairs(transaction.uiEntries or {}) do
        if not uiEntryMatches(addon, entry, entry.before) then
            if entry.kind == "edit-mode" then
                local current = getEditModeState(addon)
                if not findEditModeLayoutIndex(current, entry.before.name) then
                    return false, "The previous Edit Mode layout is no longer available"
                end
                if entry.desired.install then
                    if not editModeImportAPIsAvailable() then
                        return false, "Edit Mode layout removal is unavailable"
                    end
                elseif not C_EditMode or type(C_EditMode.SetActiveLayout) ~= "function" then
                    return false, "Edit Mode layout switching is unavailable"
                end
            elseif entry.kind == "cvar" then
                if not (C_CVar and type(C_CVar.SetCVar) == "function")
                    and type(SetCVar) ~= "function"
                then
                    return false, uiEntryLabel(entry) .. " is unavailable"
                end
            elseif entry.kind == "modified-click" then
                if type(SetModifiedClick) ~= "function" then
                    return false, uiEntryLabel(entry) .. " is unavailable"
                end
            elseif entry.kind == "cooldown-viewer" then
                if entry.before.value == nil or not C_CooldownViewer
                    or type(C_CooldownViewer.SetLayoutData) ~= "function"
                then
                    return false, "Cooldown Manager layout settings are unavailable"
                end
            else
                return false, "Unknown UI preference"
            end
        end
    end
    return true
end

local function preflightTransactionApply(addon, transaction, snapshot)
    local restorable, restoreError = preflightTransactionRestore(addon, transaction, snapshot)
    if not restorable then return false, restoreError end

    for _, entry in ipairs(transaction.entries or {}) do
        if entry.target.kind == "slot" then
            local mutationError = actionSlotMutationError()
            if mutationError then return false, mutationError end
            local canRestore, actionError = canRestoreAction(copyAction(entry.before), snapshot)
            if not canRestore then return false, actionError end
            if entry.target.expectedBindingCommand
                and (GetBindingAction(entry.target.inputKey) or "")
                    ~= entry.target.expectedBindingCommand
                and type(SetBinding) ~= "function"
            then
                return false, "Binding changes are unavailable"
            end
        elseif entry.target.kind == "binding" then
            if type(SetBindingSpell) ~= "function" then
                return false, "Direct spell binding is unavailable"
            end
        elseif entry.target.kind == "click-binding" then
            if not C_ClickBindings
                or type(C_ClickBindings.GetProfileInfo) ~= "function"
                or type(C_ClickBindings.SetProfileByInfo) ~= "function"
                or type(C_ClickBindings.CanSpellBeClickBound) ~= "function"
            then
                return false, "Click Casting profile updates are unavailable"
            end
            local desiredSpellID = entry.desired.baseSpellID or entry.desired.actionID
            local checked, canBind = pcall(C_ClickBindings.CanSpellBeClickBound, desiredSpellID)
            if not checked or not canBind then
                return false, tostring(entry.desired.actionName) .. " cannot be used for Click Casting"
            end
        else
            return false, "Unknown saved action target"
        end
    end

    for _, entry in ipairs(transaction.uiEntries or {}) do
        if entry.kind == "edit-mode" then
            local current = getEditModeState(addon)
            if not findEditModeLayoutIndex(current, entry.before.name) then
                return false, "The previous Edit Mode layout is no longer available"
            end
            if entry.desired.install then
                if not editModeImportAPIsAvailable() then
                    return false, "Edit Mode layout installation is unavailable"
                end
            elseif not C_EditMode or type(C_EditMode.SetActiveLayout) ~= "function" then
                return false, "Edit Mode layout switching is unavailable"
            end
        elseif entry.kind == "cvar" then
            if not (C_CVar and type(C_CVar.SetCVar) == "function")
                and type(SetCVar) ~= "function"
            then
                return false, uiEntryLabel(entry) .. " is unavailable"
            end
        elseif entry.kind == "modified-click" then
            if type(SetModifiedClick) ~= "function" then
                return false, uiEntryLabel(entry) .. " is unavailable"
            end
        elseif entry.kind == "cooldown-viewer" then
            if not C_CooldownViewer
                or type(C_CooldownViewer.GetLayoutData) ~= "function"
                or type(C_CooldownViewer.SetLayoutData) ~= "function"
            then
                return false, "Cooldown Manager layout settings are unavailable"
            end
        else
            return false, "Unknown UI preference"
        end
    end
    return true
end

local function restoreTransaction(addon, transaction)
    local errors = {}
    clearCursor()
    for _, entry in ipairs(transaction.entries or {}) do
        if not targetMatchesBefore(addon, entry) then
            local restored, restoreError = restoreEntry(addon, entry)
            if not restored then
                table.insert(errors, restoreError)
            end
        end
    end
    for _, entry in ipairs(transaction.uiEntries or {}) do
        if not uiEntryMatches(addon, entry, entry.before) then
            local restored, restoreError = restoreUIEntry(addon, entry)
            if not restored then table.insert(errors, restoreError) end
        end
    end
    if not saveBindings(transaction) then
        table.insert(errors, "bindings could not be saved")
    end
    return #errors == 0, table.concat(errors, "; ")
end

function Addon:GetLayoutTransactionState()
    local storage = self.db and initializeStorage(self)
    if storage and storage.active then
        local valid, validationError = validateStoredTransaction(storage.active)
        if not valid then
            quarantineInvalidActive(self, storage, validationError)
            return self.layoutPlan, nil
        end
    end
    return self.layoutPlan, storage and storage.active
end

function Addon:GetLayoutTransactionHistory(maximumEntries)
    if not self.db then return {} end
    local storage = initializeStorage(self)
    local result = {}
    if storage.active then
        table.insert(result, { transaction = storage.active, active = true })
    end
    for index = #storage.history, 1, -1 do
        if not maximumEntries or #result < maximumEntries then
            table.insert(result, { transaction = storage.history[index], active = false })
        end
    end
    return result
end

function Addon:FormatLayoutTransactionHistory(maximumEntries)
    local records = self:GetLayoutTransactionHistory(maximumEntries or 6)
    if #records == 0 then
        return "Layout activity\n\nNo layout changes have been applied yet."
    end

    local lines = { "Layout activity" }
    for _, record in ipairs(records) do
        local transaction = record.transaction
        local context = type(transaction.context) == "table" and transaction.context or {}
        local contextLabel = escapeMarkup(context.specialization or context.class or "Unknown context")
        if context.formName and context.formName ~= "Default" and context.formName ~= "Caster" then
            contextLabel = contextLabel .. " / " .. escapeMarkup(context.formName)
        end
        local timestamp = escapeMarkup(transaction.closedAtText or transaction.appliedAtText
            or transaction.stagedAtText or "Unknown time")
        local entries = type(transaction.entries) == "table" and transaction.entries or {}
        local uiEntries = type(transaction.uiEntries) == "table" and transaction.uiEntries or {}
        local changeCount = #entries + #uiEntries
        local status = escapeMarkup(transactionStatusLabels[transaction.status]
            or tostring(transaction.status or "Unknown"))
        local prefix = record.active and "Current" or "Past"
        table.insert(lines, string.format("\n%s • %s", prefix, status))
        table.insert(lines, string.format("%s • %s • %s", contextLabel, countLabel(changeCount, "change"), timestamp))
        if transaction.error and transaction.error ~= "" then
            local errorText = escapeMarkup(transaction.error)
            if #errorText > 180 then errorText = errorText:sub(1, 177) .. "..." end
            table.insert(lines, "Attention: " .. errorText)
        end
    end
    return table.concat(lines, "\n")
end

function Addon:ShowLayoutHistory()
    self:InitializeLayoutPopups()
    if StaticPopup_Show then
        StaticPopup_Show("LAMDAUI_LAYOUT_HISTORY", self:FormatLayoutTransactionHistory(6))
    end
end

function Addon:IsLayoutPlanCurrent(snapshot)
    return not self.layoutPlan or sameContext(self.layoutPlan.context, getContext(self, snapshot))
end

function Addon:BuildLayoutPlan(recommendations, snapshot)
    snapshot = snapshot or self.currentSnapshot
    recommendations = recommendations or self.trainingRecommendations or {}
    local plan = {
        version = transactionVersion,
        createdAt = time(),
        createdAtText = date("%Y-%m-%d %H:%M:%S"),
        context = getContext(self, snapshot),
        entries = {},
        uiEntries = {},
        changeCount = 0,
        unchangedCount = 0,
        blockedCount = 0,
        bindingSet = GetCurrentBindingSet and GetCurrentBindingSet() or 1,
    }

    if not snapshot then
        plan.blockedCount = 1
        plan.blockedReason = "Scan the current character before building a layout"
        plan.ready = false
        return plan
    end
    if plan.context.overrideActive then
        plan.blockedCount = 1
        plan.blockedReason = "Leave the vehicle or temporary action bar before building a layout"
        plan.ready = false
        return plan
    end
    if plan.bindingSet ~= 1 and plan.bindingSet ~= 2 then
        plan.blockedCount = 1
        plan.blockedReason = "The active binding set is unavailable"
        plan.ready = false
        return plan
    end

    local seenTargets = {}
    for _, recommendation in ipairs(recommendations) do
        local target, targetError = destinationTarget(self, snapshot, recommendation.destination)
        if target and target.kind == "slot" and not validActionSlot(target.slot) then
            target = nil
            targetError = tostring(recommendation.destination.label)
                .. " resolved outside the supported action bars"
        end
        local desired = copyAction(recommendation.action)
        local entry = {
            destination = recommendation.destination,
            target = target,
            desired = desired,
            score = recommendation.score,
            confidence = recommendation.confidence,
            reason = recommendation.reason,
        }
        if not target then
            entry.blockedReason = targetError
        else
            local key = targetKey(target)
            if seenTargets[key] then
                entry.blockedReason = "Two destinations resolve to the same managed input"
            else
                seenTargets[key] = true
                entry.before, entry.beforeFingerprint = captureTarget(self, target)
                local bindingMatches = target.kind ~= "slot"
                    or not target.expectedBindingCommand
                    or entry.before.bindingCommand == target.expectedBindingCommand
                if target.kind == "slot" and desiredMatches(entry.before, desired) and bindingMatches then
                    entry.unchanged = true
                    plan.unchangedCount = plan.unchangedCount + 1
                elseif target.kind == "binding"
                    and ((entry.before.command == "SPELL " .. tostring(desired.actionName)))
                then
                    entry.unchanged = true
                    plan.unchangedCount = plan.unchangedCount + 1
                elseif target.kind == "click-binding" and targetMatchesDesired(self, entry) then
                    entry.unchanged = true
                    plan.unchangedCount = plan.unchangedCount + 1
                else
                    local restorable, restoreError = true, nil
                    local changesBinding = target.kind == "binding"
                        or (target.kind == "slot" and target.expectedBindingCommand
                            and entry.before.bindingCommand ~= target.expectedBindingCommand)
                    if changesBinding and (type(SetBinding) ~= "function"
                        or type(SaveBindings) ~= "function"
                        or (target.kind == "binding" and type(SetBindingSpell) ~= "function"))
                    then
                        restorable = false
                        restoreError = "Binding changes are unavailable"
                    elseif target.kind == "slot" then
                        restoreError = actionSlotMutationError()
                        if restoreError then
                            restorable = false
                        else
                            restorable, restoreError = canRestoreAction(entry.before, snapshot)
                        end
                    end
                    if not restorable then
                        entry.blockedReason = restoreError
                    elseif target.kind == "click-binding" then
                        local desiredSpellID = desired.baseSpellID or desired.actionID
                        local macroType = Enum and Enum.ClickBindingType
                            and Enum.ClickBindingType.Macro or 2
                        if entry.before.clickBindingType == macroType
                            and (not entry.before.macroName or entry.before.macroBody == nil)
                        then
                            entry.blockedReason = "The current Click Casting macro cannot be saved for exact Undo"
                        else
                            local bindable, canBind = pcall(C_ClickBindings.CanSpellBeClickBound, desiredSpellID)
                            if not bindable or not canBind then
                                entry.blockedReason = tostring(desired.actionName)
                                    .. " cannot be used for Click Casting"
                            else
                                plan.changeCount = plan.changeCount + 1
                            end
                        end
                    else
                        plan.changeCount = plan.changeCount + 1
                    end
                end
            end
        end
        if entry.blockedReason then
            plan.blockedCount = plan.blockedCount + 1
        end
        table.insert(plan.entries, entry)
    end

    plan.uiEntries = buildUIEntries(self)
    for _, entry in ipairs(plan.uiEntries) do
        if entry.blockedReason then
            plan.blockedCount = plan.blockedCount + 1
        else
            plan.changeCount = plan.changeCount + 1
        end
    end

    plan.ready = plan.changeCount > 0 and plan.blockedCount == 0
    if #recommendations == 0 and plan.changeCount == 0 then
        plan.blockedReason = "No matching placements are available for this specialization"
    end
    return plan
end

function Addon:PreviewLayoutPlan()
    local snapshot = self:CaptureSnapshot("layout preview")
    if not snapshot then
        return nil
    end
    self.trainingRecommendations, self.trainingCoverage = self:BuildTrainingRecommendations()
    self.layoutPlan = self:BuildLayoutPlan(self.trainingRecommendations, snapshot)
    self.layoutPlan.coverage = self.trainingCoverage
    local plan = self.layoutPlan
    if plan.ready then
        local coverageText = self.trainingCoverage and not self.trainingCoverage.complete
            and ("; mapped " .. self.trainingCoverage.recommendedPlacements .. " of "
                .. self.trainingCoverage.targetPlacements .. " possible placements") or ""
        self:SetStatus("Preview ready • " .. countLabel(plan.changeCount, "change") .. " • "
            .. countLabel(plan.unchangedCount, "ability", "abilities") .. " already in place" .. coverageText)
    elseif plan.blockedCount > 0 then
        self:SetStatus("Preview needs attention • " .. countLabel(plan.blockedCount, "change") .. " cannot be applied safely")
    elseif self.trainingCoverage and not self.trainingCoverage.complete then
        self:SetStatus("Preview mapped " .. self.trainingCoverage.recommendedPlacements .. " of "
            .. self.trainingCoverage.targetPlacements .. " possible placements • Learn another setup to improve it")
    else
        self:SetStatus(plan.blockedReason or "No layout changes were proposed")
    end
    if self.RefreshUI then
        self:RefreshUI()
    end
    return plan
end

function Addon:StageLayoutPlan()
    if InCombatLockdown and InCombatLockdown() then
        self:SetStatus("Cannot apply a layout during combat")
        return false
    end
    local plan = self.layoutPlan
    if not plan or not plan.ready then
        self:SetStatus("Build a safe layout preview before applying")
        return false
    end
    local currentBindingSet = GetCurrentBindingSet and GetCurrentBindingSet() or 1
    if currentBindingSet ~= plan.bindingSet then
        self.layoutPlan = nil
        self:SetStatus("The active binding set changed • Build a new preview")
        return false
    end
    local snapshot = self:CaptureSnapshot("layout apply preflight")
    if not snapshot or not sameContext(plan.context, getContext(self, snapshot)) then
        self.layoutPlan = nil
        self:SetStatus("The specialization, form, talents, or profile changed • Build a new preview")
        return false
    end
    if not cursorIsEmpty() then
        self:SetStatus("Clear the mouse cursor before applying the layout")
        return false
    end

    local transaction = {
        version = transactionVersion,
        id = tostring(time()) .. ":" .. plan.context.characterKey .. ":" .. tostring(plan.context.specializationID),
        status = "staged",
        stagedAt = time(),
        stagedAtText = date("%Y-%m-%d %H:%M:%S"),
        context = plan.context,
        bindingSet = plan.bindingSet,
        entries = {},
        uiEntries = {},
        changesBindings = false,
    }
    for _, entry in ipairs(plan.entries) do
        if not entry.unchanged and not entry.blockedReason then
            if not targetMatchesBefore(self, entry) then
                self.layoutPlan = nil
                self:SetStatus("The profile inputs changed after preview • Build a new preview")
                return false
            end
            table.insert(transaction.entries, copyTransactionEntry(entry))
            if entry.target.kind == "binding"
                or (entry.target.expectedBindingCommand
                    and entry.before.bindingCommand ~= entry.target.expectedBindingCommand)
            then
                transaction.changesBindings = true
            end
        end
    end
    for _, entry in ipairs(plan.uiEntries or {}) do
        if not entry.blockedReason then
            if not uiEntryMatches(self, entry, entry.before) then
                self.layoutPlan = nil
                self:SetStatus("The UI changed after preview • Build a new preview")
                return false
            end
            table.insert(transaction.uiEntries, copyTransactionUIEntry(entry))
            if entry.kind == "modified-click" then
                transaction.changesBindings = true
            end
        end
    end

    transaction.payloadChecksum = transactionPayloadChecksum(transaction)
    local transactionValid, transactionError = validateStoredTransaction(transaction)
    if not transactionValid then
        self.layoutPlan = nil
        self:SetStatus("The preview could not be saved safely: " .. transactionError)
        return false
    end

    local storage = initializeStorage(self)
    if storage.active then
        local valid, validationError = validateStoredTransaction(storage.active)
        if not valid then
            quarantineInvalidActive(self, storage, validationError)
            return false
        end
        if storage.active.status == "staged" or storage.active.status == "applying" then
            self:SetStatus("A layout is already waiting to apply")
            return false
        elseif storage.active.status ~= "applied" then
            self:SetStatus("Finish layout recovery before applying another preview")
            return false
        end
        archiveActive(self, "superseded")
        storage = initializeStorage(self)
    end
    storage.active = transaction
    local transactionChanges = #transaction.entries + #transaction.uiEntries
    self:SetStatus("Preview saved • Reloading to apply " .. countLabel(transactionChanges, "change"))
    if ReloadUI then
        ReloadUI()
        return true
    end
    self:SetStatus("Layout is staged; reload the UI to apply it")
    return true
end

function Addon:ResumeLayoutTransaction()
    if not self.db or (InCombatLockdown and InCombatLockdown()) then
        return false
    end
    local storage = initializeStorage(self)
    local transaction = storage.active
    if not transaction then
        return false
    end
    local valid, validationError = validateStoredTransaction(transaction)
    if not valid then
        quarantineInvalidActive(self, storage, validationError)
        if self.RefreshUI then self:RefreshUI() end
        return false
    end
    if transaction.status ~= "staged" and transaction.status ~= "applying" then return false end
    local currentBindingSet = GetCurrentBindingSet and GetCurrentBindingSet() or 1
    if currentBindingSet ~= transaction.bindingSet then
        self:SetStatus("This preview is waiting for its original account or character binding set")
        if self.RefreshUI then self:RefreshUI() end
        return false
    end
    local snapshot = self.currentSnapshot or self:CaptureSnapshot("layout recovery")
    if not snapshot or not sameContext(transaction.context, getContext(self, snapshot)) then
        self:SetStatus("This preview is waiting for its original specialization, form, talents, and profile")
        if self.RefreshUI then self:RefreshUI() end
        return false
    end
    if not cursorIsEmpty() then
        self:SetStatus("Clear the mouse cursor to continue the staged layout")
        return false
    end

    local beforeCount, desiredCount = 0, 0
    local transactionChanges = #transaction.entries + #(transaction.uiEntries or {})
    for _, entry in ipairs(transaction.entries) do
        if targetMatchesBefore(self, entry) then
            beforeCount = beforeCount + 1
        elseif targetMatchesDesired(self, entry) then
            desiredCount = desiredCount + 1
        end
    end
    for _, entry in ipairs(transaction.uiEntries or {}) do
        if uiEntryMatches(self, entry, entry.before) then
            beforeCount = beforeCount + 1
        elseif uiEntryMatches(self, entry, entry.desired) then
            desiredCount = desiredCount + 1
        end
    end
    if desiredCount == transactionChanges then
        transaction.status = "applied"
        transaction.appliedAt = transaction.appliedAt or time()
        transaction.appliedAtText = transaction.appliedAtText or date("%Y-%m-%d %H:%M:%S")
        self:SetStatus("Layout is applied; Undo is available")
        if self.RefreshUI then self:RefreshUI() end
        return true
    elseif beforeCount ~= transactionChanges then
        local canRestore, preflightError = preflightTransactionRestore(self, transaction, snapshot)
        if not canRestore then
            transaction.status = "recovery-incomplete"
            transaction.error = preflightError
            self:SetStatus("Layout recovery stopped before making more changes: " .. preflightError)
            if self.RefreshUI then self:RefreshUI() end
            return false
        end
        local restored, restoreError = restoreTransaction(self, transaction)
        if restored then
            archiveActive(self, "recovered")
            self:SetStatus("An interrupted layout was restored to its saved before-state")
        else
            transaction.status = "recovery-incomplete"
            transaction.error = restoreError
            self:SetStatus("Layout recovery needs attention: " .. restoreError)
        end
        if self.ScheduleScan then self:ScheduleScan("layout recovery", 0.8) end
        if self.RefreshUI then self:RefreshUI() end
        return restored
    end


    local canApply, preflightError = preflightTransactionApply(self, transaction, snapshot)
    if not canApply then
        transaction.status = "staged"
        transaction.error = preflightError
        self:SetStatus("Layout apply stopped before making changes: " .. preflightError)
        if self.RefreshUI then self:RefreshUI() end
        return false
    end

    transaction.status = "applying"
    transaction.error = nil
    for _, entry in ipairs(transaction.entries) do
        local applied, applyError = applyEntry(self, entry)
        if not applied then
            local restored, restoreError = restoreTransaction(self, transaction)
            if restored then
                archiveActive(self, "apply-failed-restored")
                self:SetStatus("Layout was not applied: " .. applyError .. "; the original setup was restored")
            else
                transaction.status = "apply-failed-restore-incomplete"
                transaction.error = applyError .. "; " .. restoreError
                self:SetStatus("Layout recovery needs attention: " .. transaction.error)
            end
            if self.ScheduleScan then self:ScheduleScan("layout apply failed", 0.8) end
            if self.RefreshUI then self:RefreshUI() end
            return false
        end
    end
    for _, entry in ipairs(transaction.uiEntries or {}) do
        local applied, applyError = applyUIEntry(self, entry)
        if not applied then
            local restored, restoreError = restoreTransaction(self, transaction)
            if restored then
                archiveActive(self, "apply-failed-restored")
                self:SetStatus("Layout was not applied: " .. applyError .. "; the original setup was restored")
            else
                transaction.status = "apply-failed-restore-incomplete"
                transaction.error = applyError .. "; " .. restoreError
                self:SetStatus("Layout recovery needs attention: " .. transaction.error)
            end
            if self.ScheduleScan then self:ScheduleScan("UI apply failed", 0.8) end
            if self.RefreshUI then self:RefreshUI() end
            return false
        end
    end
    if not saveBindings(transaction) then
        local restored, restoreError = restoreTransaction(self, transaction)
        if restored then
            archiveActive(self, "binding-save-failed-restored")
            self:SetStatus("Bindings could not be saved; the original setup was restored")
        else
            transaction.status = "apply-failed-restore-incomplete"
            transaction.error = restoreError
            self:SetStatus("Bindings and layout recovery need attention: " .. restoreError)
        end
        return false
    end

    transaction.status = "applied"
    transaction.appliedAt = time()
    transaction.appliedAtText = date("%Y-%m-%d %H:%M:%S")
    self.layoutPlan = nil
    self:SetStatus("Applied " .. countLabel(transactionChanges, "change") .. " • Undo is available")
    if self.ScheduleScan then self:ScheduleScan("layout applied", 0.8) end
    if self.RefreshUI then self:RefreshUI() end
    return true
end

function Addon:UndoLayoutTransaction()
    if InCombatLockdown and InCombatLockdown() then
        self:SetStatus("Cannot undo a layout during combat")
        return false
    end
    local storage = self.db and initializeStorage(self)
    local transaction = storage and storage.active
    if not transaction then
        self:SetStatus("There is no applied layout to undo")
        return false
    end
    local valid, validationError = validateStoredTransaction(transaction)
    if not valid then
        quarantineInvalidActive(self, storage, validationError)
        if self.RefreshUI then self:RefreshUI() end
        return false
    end
    local incomplete = transaction.status == "undo-incomplete"
        or transaction.status == "recovery-incomplete"
        or transaction.status == "apply-failed-restore-incomplete"
    if transaction.status ~= "applied" and not incomplete then
        self:SetStatus("There is no applied layout to undo")
        return false
    end
    local currentBindingSet = GetCurrentBindingSet and GetCurrentBindingSet() or 1
    if currentBindingSet ~= transaction.bindingSet then
        self:SetStatus("Return to the account or character binding set used for Apply")
        return false
    end
    local snapshot = self:CaptureSnapshot("layout undo preflight")
    if not snapshot or not sameContext(transaction.context, getContext(self, snapshot)) then
        self:SetStatus("Return to the character, specialization, form, talents, and profile used for Apply")
        return false
    end
    if not cursorIsEmpty() then
        self:SetStatus("Clear the mouse cursor before undoing the layout")
        return false
    end
    if not incomplete then
        for _, entry in ipairs(transaction.entries) do
            if not targetMatchesDesired(self, entry) then
                self:SetStatus("A managed action changed after Apply; Undo stopped to protect the newer setup")
                return false
            end
        end
        for _, entry in ipairs(transaction.uiEntries or {}) do
            if not uiEntryMatches(self, entry, entry.desired) then
                self:SetStatus("A managed UI setting changed after Apply; Undo stopped to protect the newer setup")
                return false
            end
        end
    end
    local canRestore, preflightError = preflightTransactionRestore(self, transaction, snapshot)
    if not canRestore then
        self:SetStatus("Undo stopped before making changes: " .. preflightError)
        return false
    end
    local restored, restoreError = restoreTransaction(self, transaction)
    if not restored then
        transaction.status = "undo-incomplete"
        transaction.error = restoreError
        self:SetStatus("Undo needs attention: " .. restoreError)
        return false
    end
    archiveActive(self, incomplete and "recovered" or "undone")
    self:SetStatus("The previous layout was restored")
    if self.ScheduleScan then self:ScheduleScan("layout undone", 0.8) end
    if self.RefreshUI then self:RefreshUI() end
    return true
end

function Addon:InitializeLayoutPopups()
    if not StaticPopupDialogs then
        return
    end
    StaticPopupDialogs.LAMDAUI_APPLY_LAYOUT = {
        text = "Apply %s?\n\nlamdaUI will save the current setup, reload the interface, and apply only the changes in this preview. Undo remains available afterward.",
        button1 = "Apply",
        button2 = CANCEL,
        OnAccept = function()
            Addon:StageLayoutPlan()
        end,
        timeout = 0,
        whileDead = true,
        hideOnEscape = true,
        preferredIndex = 3,
    }
    StaticPopupDialogs.LAMDAUI_LAYOUT_HISTORY = {
        text = "%s",
        button1 = DONE,
        timeout = 0,
        whileDead = true,
        hideOnEscape = true,
        preferredIndex = 3,
    }
end

function Addon:ConfirmApplyLayout()
    local plan = self.layoutPlan
    if not plan or not plan.ready then
        self:SetStatus("Build a safe layout preview before applying")
        return
    end
    self:InitializeLayoutPopups()
    if StaticPopup_Show then
        StaticPopup_Show("LAMDAUI_APPLY_LAYOUT", countLabel(plan.changeCount, "change"))
    end
end
