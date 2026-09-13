local _, Addon = ...
local macroDB, statusLabel

local TARGET_CHARACTER = "Devouring"
local TARGET_REALM = "Gurubashi"
local DEVASTATION_SPEC_ID = 1467
local INSTALL_VERSION = 2
local QUESTION_MARK_ICON = "INV_MISC_QUESTIONMARK"

-- Preheat's 12.1 recommendation is to cancel Deep Breath with whichever
-- rotational action comes next. Only spells already present on the current
-- Devastation action bars are converted, so this does not consume unnecessary
-- character-macro slots or alter the Preservation layout.
local MACROS = {
    { macroName = "DB Dragonrage", spellName = "Dragonrage" },
    { macroName = "DB Disintegrate", spellName = "Disintegrate", autoTarget = true },
    { macroName = "DB Pyre", spellName = "Pyre", autoTarget = true },
    { macroName = "DB Eternity", spellName = "Eternity Surge", autoTarget = true },
    { macroName = "DB Fire Breath", spellName = "Fire Breath", autoTarget = true },
    { macroName = "DB Living Flame", spellName = "Living Flame", autoTarget = true },
    { macroName = "DB Azure Strike", spellName = "Azure Strike", autoTarget = true },
    { macroName = "DB Unbound", spellName = "Unbound Flame", autoTarget = true },
    { macroName = "DB Shatter Star", spellName = "Shattering Star", autoTarget = true },
    { macroName = "DB Azure Sweep", spellName = "Azure Sweep", autoTarget = true },
    { macroName = "DB Tip Scales", spellName = "Tip the Scales" },
}

local macroBySpell = {}
local ourMacroNames = {}
for _, definition in ipairs(MACROS) do
    macroBySpell[definition.spellName] = definition
    ourMacroNames[definition.macroName] = true
end

local function printMessage(message)
    print("|cff65d9fflamdaUI macros:|r " .. tostring(message))
    if statusLabel then statusLabel:SetText(message) end
end

local function isTargetCharacter()
    local character = UnitName and UnitName("player")
    local realm = GetRealmName and GetRealmName()
    return character == TARGET_CHARACTER and realm == TARGET_REALM
end

local function getCurrentSpecID()
    if not GetSpecialization or not GetSpecializationInfo then
        return nil
    end
    local specializationIndex = GetSpecialization()
    if not specializationIndex then
        return nil
    end
    return GetSpecializationInfo(specializationIndex)
end

local function spellNameForID(spellID)
    if C_Spell and C_Spell.GetSpellName then
        return C_Spell.GetSpellName(spellID)
    end
    if GetSpellInfo then
        return GetSpellInfo(spellID)
    end
end

local function macroBody(definition)
    local body = "#showtooltip " .. definition.spellName
        .. "\n/cancelaura Deep Breath"

    if definition.autoTarget then
        body = body
            .. "\n/targetenemy [noharm][dead]"
            .. "\n/cast [harm,nodead] " .. definition.spellName
    else
        body = body .. "\n/cast " .. definition.spellName
    end

    return body
end

local function ensureMacro(definition)
    local index = GetMacroIndexByName and GetMacroIndexByName(definition.macroName) or 0
    local body = macroBody(definition)

    if index and index > 0 then
        local _, _, currentBody = GetMacroInfo(index)
        if currentBody ~= body then
            index = EditMacro(index, definition.macroName, QUESTION_MARK_ICON, body, 1)
        end
        return index
    end

    return CreateMacro(definition.macroName, QUESTION_MARK_ICON, body, true)
end

local function refreshExistingMacros()
    if not isTargetCharacter() or getCurrentSpecID() ~= DEVASTATION_SPEC_ID then
        return false, "Switch to Devastation before refreshing macros."
    end
    if InCombatLockdown and InCombatLockdown() then
        return false, "Macro refresh is waiting until combat ends."
    end
    if not GetMacroInfo or not GetMacroIndexByName or not EditMacro then
        return false, "Required macro APIs are unavailable."
    end

    local found = 0
    local updated = 0
    for _, definition in ipairs(MACROS) do
        local index = GetMacroIndexByName(definition.macroName)
        if index and index > 0 then
            found = found + 1
            local _, _, currentBody = GetMacroInfo(index)
            if currentBody ~= macroBody(definition) then
                ensureMacro(definition)
                updated = updated + 1
            end
        end
    end

    macroDB = macroDB or {}
    macroDB.installVersion = INSTALL_VERSION
    macroDB.disabled = nil

    return true, "Refreshed " .. tostring(updated) .. " of " .. tostring(found)
        .. " existing Devastation macros."
end

local function putCursorInSlot(slot)
    if C_ActionBar and C_ActionBar.PutActionInSlot then
        return C_ActionBar.PutActionInSlot(slot)
    end
    if PlaceAction then
        return PlaceAction(slot)
    end
end

local function collectSpellSlots()
    local slots = {}
    for slot = 1, 180 do
        local actionType, actionID = GetActionInfo(slot)
        if actionType == "spell" and actionID then
            local spellName = spellNameForID(actionID)
            if spellName and macroBySpell[spellName] then
                slots[#slots + 1] = {
                    slot = slot,
                    actionID = actionID,
                    spellName = spellName,
                }
            end
        end
    end
    return slots
end

local function install()
    if not isTargetCharacter() then
        return false, "This installer is restricted to Devouring-Gurubashi."
    end
    if getCurrentSpecID() ~= DEVASTATION_SPEC_ID then
        return false, "Switch to Devastation; installation will run automatically."
    end
    if InCombatLockdown and InCombatLockdown() then
        return false, "Installation is waiting until combat ends."
    end
    if not GetActionInfo or not GetMacroInfo or not GetMacroIndexByName
        or not CreateMacro or not EditMacro or not PickupMacro
    then
        return false, "Required macro APIs are unavailable."
    end
    if not ((C_ActionBar and C_ActionBar.PutActionInSlot) or PlaceAction) then
        return false, "Required action-bar APIs are unavailable."
    end

    local spellSlots = collectSpellSlots()
    if #spellSlots == 0 then
        return false, "No supported Devastation spells were found on the action bars."
    end

    macroDB = macroDB or {}
    macroDB.replacedSlots = macroDB.replacedSlots or {}

    local macroIndices = {}
    for _, slotInfo in ipairs(spellSlots) do
        local definition = macroBySpell[slotInfo.spellName]
        if not macroIndices[definition.macroName] then
            local index = ensureMacro(definition)
            if not index or index == 0 then
                return false, "There is not enough character-macro space to finish safely."
            end
            macroIndices[definition.macroName] = true
        end
    end

    -- Creating a character macro can renumber the other character macros.
    -- Resolve every final index only after all required macros exist.
    for macroName in pairs(macroIndices) do
        local index = GetMacroIndexByName(macroName)
        if not index or index == 0 then
            return false, "Macro " .. macroName .. " could not be resolved after creation."
        end
        macroIndices[macroName] = index
    end

    local replaced = 0
    for _, slotInfo in ipairs(spellSlots) do
        local definition = macroBySpell[slotInfo.spellName]
        local macroIndex = macroIndices[definition.macroName]

        if not macroDB.replacedSlots[slotInfo.slot] then
            macroDB.replacedSlots[slotInfo.slot] = {
                actionID = slotInfo.actionID,
                spellName = slotInfo.spellName,
                macroName = definition.macroName,
            }
        end

        if ClearCursor then ClearCursor() end
        PickupMacro(macroIndex)
        putCursorInSlot(slotInfo.slot)
        if ClearCursor then ClearCursor() end

        local actionType, actionID = GetActionInfo(slotInfo.slot)
        if actionType == "macro" and actionID == macroIndex then
            replaced = replaced + 1
        else
            return false, "Action slot " .. tostring(slotInfo.slot) .. " could not be replaced."
        end
    end

    macroDB.installVersion = INSTALL_VERSION
    macroDB.disabled = nil
    macroDB.installedAt = date and date("%Y-%m-%d %H:%M:%S") or nil
    macroDB.replacedCount = replaced
    return true, "Installed Deep Breath cancel macros on " .. tostring(replaced)
        .. " action-bar button" .. (replaced == 1 and "." or "s.")
end

local function undo()
    if not isTargetCharacter() then
        return false, "Undo is restricted to Devouring-Gurubashi."
    end
    if getCurrentSpecID() ~= DEVASTATION_SPEC_ID then
        return false, "Switch to Devastation before using undo."
    end
    if InCombatLockdown and InCombatLockdown() then
        return false, "Undo is waiting until combat ends."
    end
    if not macroDB or not macroDB.replacedSlots then
        return false, "There is no saved setup to undo."
    end
    if not C_Spell or not C_Spell.PickupSpell then
        return false, "Spell placement is unavailable."
    end

    local restored = 0
    local remaining = {}
    for slot, prior in pairs(macroDB.replacedSlots) do
        local actionType, actionID = GetActionInfo(slot)
        local currentMacroName = actionType == "macro" and GetMacroInfo(actionID) or nil
        local currentSpellName = actionType == "spell" and spellNameForID(actionID) or nil
        if currentSpellName == prior.spellName then
            restored = restored + 1
        elseif currentMacroName and ourMacroNames[currentMacroName] then
            if ClearCursor then ClearCursor() end
            C_Spell.PickupSpell(prior.actionID)
            putCursorInSlot(slot)
            if ClearCursor then ClearCursor() end

            local restoredType, restoredID = GetActionInfo(slot)
            if restoredType == "spell" and spellNameForID(restoredID) == prior.spellName then
                restored = restored + 1
            else
                remaining[slot] = prior
            end
        else
            remaining[slot] = prior
        end
    end

    if next(remaining) then
        macroDB.replacedSlots = remaining
        return false, "Restored " .. tostring(restored)
            .. " buttons, but some changed slots were left untouched."
    end

    macroDB.installVersion = nil
    macroDB.replacedSlots = nil
    macroDB.replacedCount = nil
    macroDB.disabled = true
    return true, "Restored " .. tostring(restored) .. " original spell button"
        .. (restored == 1 and "." or "s.")
end

local function status()
    if macroDB and macroDB.disabled then
        return true, "Automatic installation is disabled; the original spell buttons are active."
    end
    if macroDB and macroDB.installVersion == INSTALL_VERSION then
        return true, "Installed on " .. tostring(macroDB.replacedCount or 0)
            .. " action-bar buttons. Use /devamacros undo to restore the original spells."
    end
    return true, "Not installed yet. Switch to Devastation or use /devamacros install."
end

local function runAndReport(operation)
    local ok, message = operation()
    printMessage(message)
    return ok
end

SLASH_DEVOURINGDEVMACROS1 = "/devamacros"
SlashCmdList.DEVOURINGDEVMACROS = function(message)
    if not Addon.db or Addon.legacyConflict then return end
    if Addon.customMigrationPending then
        printMessage("Legacy macro data is waiting to be imported. Leave combat and reload before changing macros.")
        return
    end
    message = strtrim((message or ""):lower())
    if message ~= "undo" and message ~= "status" then Addon:SetCustomModuleEnabled("devastationMacros", true) end
    if message == "undo" then
        runAndReport(undo)
    elseif message == "status" then
        runAndReport(status)
    else
        runAndReport(install)
    end
end

local frame = CreateFrame("Frame")
frame:RegisterEvent("PLAYER_LOGIN")
frame:RegisterEvent("PLAYER_ENTERING_WORLD")
frame:RegisterEvent("PLAYER_SPECIALIZATION_CHANGED")
frame:RegisterEvent("PLAYER_REGEN_ENABLED")
frame:SetScript("OnEvent", function(_, event, unit)
    if event == "PLAYER_SPECIALIZATION_CHANGED" and unit ~= "player" then
        return
    end
    if not isTargetCharacter() or not Addon:IsCustomModuleEnabled("devastationMacros") or Addon.customMigrationPending then
        return
    end
    C_Timer.After(2, function()
        if Addon:IsCustomModuleEnabled("devastationMacros") and not Addon.customMigrationPending and not (macroDB and macroDB.disabled)
            and getCurrentSpecID() == DEVASTATION_SPEC_ID
            and not (InCombatLockdown and InCombatLockdown())
        then
            local ok, message = refreshExistingMacros()
            if ok and not message:find("Refreshed 0 ", 1, true) then
                printMessage(message)
            end
        end
    end)
end)


Addon:RegisterCustomModule({
    id = "devastationMacros", name = "Devastation macros",
    description = "Your existing Deep Breath cancel macros. Automatic refresh applies only to the original character and Devastation specialization.",
    defaults = { enabled = true },
    initialize = function()
        local settings = Addon:GetCustomModuleSettings("devastationMacros")
        settings.state = type(settings.state) == "table" and settings.state or {}
        macroDB = settings.state
    end,
    buildOptions = function(parent)
        Addon:CustomHubText(parent, "Turning this module off stops automatic macro refresh.\nExisting macros and buttons remain usable. Restore spells explicitly\nwith Undo on the original character in Devastation, outside combat.", 0, 0, 505)
        Addon:CustomHubButton(parent, "Install / refresh", 0, -78, 145, function() SlashCmdList.DEVOURINGDEVMACROS("install") end)
        Addon:CustomHubButton(parent, "Undo macros", 155, -78, 135, function() SlashCmdList.DEVOURINGDEVMACROS("undo") end)
        statusLabel = Addon:CustomHubText(parent, "", 0, -124, 505)
        local _, text = status()
        statusLabel:SetText(text)
    end,
})
