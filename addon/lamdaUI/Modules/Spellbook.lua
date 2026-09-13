local _, Addon = ...

local function isSecret(value)
    return value ~= nil and issecretvalue and issecretvalue(value)
end

local function safeScalar(value)
    if value == nil or isSecret(value) then
        return nil
    end

    local valueType = type(value)
    if valueType == "string" or valueType == "number" or valueType == "boolean" then
        return value
    end

    return nil
end

local function safeCall(func, ...)
    if not func then
        return nil
    end

    local ok, value = pcall(func, ...)
    if not ok or isSecret(value) then
        return nil
    end
    return value
end

local function getTalentFlag(func, slotIndex, spellBank)
    local value = safeCall(func, slotIndex, spellBank)
    return value == true
end

local function getSpellPowerCosts(spellID)
    local result = {}
    local costs = safeCall(C_Spell and C_Spell.GetSpellPowerCost, spellID)
    if type(costs) ~= "table" then return result end
    for _, cost in ipairs(costs) do
        if type(cost) == "table" then
            local powerType = safeScalar(cost.type)
            local powerToken = safeScalar(cost.name)
            if type(powerType) == "number" and type(powerToken) == "string" then
                table.insert(result, {
                    type = powerType,
                    name = powerToken,
                    cost = safeScalar(cost.cost),
                    minCost = safeScalar(cost.minCost),
                    costPercent = safeScalar(cost.costPercent),
                    costPerSec = safeScalar(cost.costPerSec),
                    requiredAuraID = safeScalar(cost.requiredAuraID),
                })
            end
        end
    end
    return result
end

local cooldownCategoryDefinitions = {
    { enumName = "Essential", key = "essential" },
    { enumName = "Utility", key = "utility" },
    { enumName = "TrackedBuff", key = "tracked-buff" },
    { enumName = "TrackedBar", key = "tracked-bar" },
    { enumName = "GroupBuff", key = "group-buff" },
    { enumName = "SpecAgnosticEssential", key = "spec-essential" },
    { enumName = "SpecAgnosticTracked", key = "spec-tracked" },
}

local function addCooldownCategory(lookup, spellID, category)
    if type(spellID) ~= "number" then
        return
    end
    lookup[spellID] = lookup[spellID] or {}
    lookup[spellID][category] = true
end

local function getCooldownViewerLookup()
    local lookup = {}
    if not C_CooldownViewer
        or not C_CooldownViewer.GetCooldownViewerCategorySet
        or not C_CooldownViewer.GetCooldownViewerCooldownInfo
        or not Enum
        or not Enum.CooldownViewerCategory
    then
        return lookup, false
    end
    if C_CooldownViewer.IsCooldownViewerAvailable
        and safeCall(C_CooldownViewer.IsCooldownViewerAvailable) == false
    then
        return lookup, false
    end

    for _, definition in ipairs(cooldownCategoryDefinitions) do
        local categoryValue = Enum.CooldownViewerCategory[definition.enumName]
        if categoryValue ~= nil then
            local cooldownIDs = safeCall(C_CooldownViewer.GetCooldownViewerCategorySet, categoryValue, false)
            if type(cooldownIDs) == "table" then
                for _, cooldownID in ipairs(cooldownIDs) do
                    local info = safeCall(C_CooldownViewer.GetCooldownViewerCooldownInfo, cooldownID)
                    if type(info) == "table" and info.isKnown ~= false then
                        addCooldownCategory(lookup, safeScalar(info.spellID), definition.key)
                        addCooldownCategory(lookup, safeScalar(info.overrideSpellID), definition.key)
                        addCooldownCategory(lookup, safeScalar(info.overrideTooltipSpellID), definition.key)
                        for _, linkedSpellID in ipairs(info.linkedSpellIDs or {}) do
                            addCooldownCategory(lookup, safeScalar(linkedSpellID), definition.key)
                        end
                    end
                end
            end
        end
    end
    return lookup, true
end

local function getSpellCategories(lookup, ...)
    local seen, result = {}, {}
    for index = 1, select("#", ...) do
        local spellID = select(index, ...)
        for category in pairs(lookup[spellID] or {}) do
            seen[category] = true
        end
    end
    for _, definition in ipairs(cooldownCategoryDefinitions) do
        if seen[definition.key] then
            table.insert(result, definition.key)
        end
    end
    return result
end

function Addon:GetSpellbookSnapshot()
    local snapshot = {
        available = false,
        activeConfigID = safeCall(C_ClassTalents and C_ClassTalents.GetActiveConfigID),
        activeHeroSpecID = safeCall(C_ClassTalents and C_ClassTalents.GetActiveHeroTalentSpec),
        skillLines = {},
        items = {},
    }

    if not C_SpellBook
        or not C_SpellBook.GetNumSpellBookSkillLines
        or not C_SpellBook.GetSpellBookSkillLineInfo
        or not C_SpellBook.GetSpellBookItemInfo
        or not Enum
        or not Enum.SpellBookSpellBank
    then
        snapshot.status = "Spellbook API unavailable"
        return snapshot
    end

    local playerBank = Enum.SpellBookSpellBank.Player
    local numSkillLines = safeCall(C_SpellBook.GetNumSpellBookSkillLines)
    if type(numSkillLines) ~= "number" then
        snapshot.status = "Unable to read spellbook skill lines"
        return snapshot
    end

    snapshot.available = true
    snapshot.numSkillLines = numSkillLines
    local cooldownLookup, cooldownViewerAvailable = getCooldownViewerLookup()
    snapshot.cooldownViewerAvailable = cooldownViewerAvailable

    for skillLineIndex = 1, numSkillLines do
        local lineInfo = safeCall(C_SpellBook.GetSpellBookSkillLineInfo, skillLineIndex)
        if type(lineInfo) == "table" then
            local line = {
                index = skillLineIndex,
                name = safeScalar(lineInfo.name),
                iconID = safeScalar(lineInfo.iconID),
                itemIndexOffset = safeScalar(lineInfo.itemIndexOffset),
                numSpellBookItems = safeScalar(lineInfo.numSpellBookItems),
                isGuild = safeScalar(lineInfo.isGuild),
                shouldHide = safeScalar(lineInfo.shouldHide),
                specID = safeScalar(lineInfo.specID),
                offSpecID = safeScalar(lineInfo.offSpecID),
            }
            table.insert(snapshot.skillLines, line)

            local offset = line.itemIndexOffset
            local itemCount = line.numSpellBookItems
            if type(offset) == "number" and type(itemCount) == "number" then
                for slotIndex = offset + 1, offset + itemCount do
                    local itemInfo = safeCall(C_SpellBook.GetSpellBookItemInfo, slotIndex, playerBank)
                    if type(itemInfo) == "table" then
                        local actionID = safeScalar(itemInfo.actionID)
                        local spellID = safeScalar(itemInfo.spellID)
                        local overrideSpellID = safeCall(C_Spell and C_Spell.GetOverrideSpell, actionID or spellID)
                        local item = {
                            slotIndex = slotIndex,
                            skillLineIndex = skillLineIndex,
                            skillLineName = line.name,
                            skillLineSpecID = line.specID,
                            skillLineOffSpecID = line.offSpecID,
                            actionID = actionID,
                            spellID = spellID,
                            overrideSpellID = safeScalar(overrideSpellID),
                            itemType = safeScalar(itemInfo.itemType),
                            name = safeScalar(itemInfo.name),
                            subName = safeScalar(itemInfo.subName),
                            iconID = safeScalar(itemInfo.iconID),
                            isPassive = safeScalar(itemInfo.isPassive),
                            isOffSpec = safeScalar(itemInfo.isOffSpec),
                            isClassTalent = getTalentFlag(C_SpellBook.IsClassTalentSpellBookItem, slotIndex, playerBank),
                            isPvPTalent = getTalentFlag(C_SpellBook.IsPvPTalentSpellBookItem, slotIndex, playerBank),
                        }
                        -- Retail exposes actionID as the stable base spell and
                        -- spellID as the current override when one is active.
                        -- Keep both identities so a learned placement remains
                        -- portable across forms and talent override states.
                        if type(actionID) == "number" and actionID > 0
                            and actionID % 1 == 0
                        then
                            item.baseSpellID = actionID
                        end
                        local effectiveSpellID = item.overrideSpellID or spellID or actionID
                        item.powerCosts = getSpellPowerCosts(effectiveSpellID)
                        item.isHarmful = safeScalar(safeCall(C_Spell and C_Spell.IsSpellHarmful, effectiveSpellID))
                        item.isHelpful = safeScalar(safeCall(C_Spell and C_Spell.IsSpellHelpful, effectiveSpellID))
                        item.isCrowdControl = safeScalar(safeCall(
                            C_Spell and C_Spell.IsSpellCrowdControl, effectiveSpellID))
                        item.cooldownViewerCategories = getSpellCategories(
                            cooldownLookup, actionID, spellID, item.overrideSpellID)
                        table.insert(snapshot.items, item)
                    end
                end
            end
        end
    end

    snapshot.numItems = #snapshot.items
    snapshot.status = "Captured " .. #snapshot.items .. " spellbook item(s)"
    return snapshot
end
