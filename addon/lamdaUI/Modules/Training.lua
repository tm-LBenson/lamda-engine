local _, Addon = ...

local modelVersion = 3
local maximumContextSetups = 2048
local positiveWeights = { context = 5, spec = 3, global = 1, role = 1, profileRole = 2, destination = 0.5 }
local negativeWeights = { context = -4, spec = -2, global = -1, role = -0.5, profileRole = -1, destination = -0.5 }
local destinationUsageScore = 0.2
local completionFallbackScore = 0.01

local function countLabel(count, singular, plural)
    count = tonumber(count) or 0
    return tostring(count) .. " " .. (count == 1 and singular or (plural or (singular .. "s")))
end

local roleLabels = {
    healing = "healing",
    defensive = "defensive",
    cooldown = "cooldown",
    builder = "builder",
    spender = "spender",
    aoe = "AoE",
    utility = "utility",
    interrupt = "interrupt",
    movement = "movement",
    cc = "crowd control",
    dispel = "dispel",
    rotation = "core rotation",
    maintenance = "maintenance",
    offensive = "offensive",
    supportive = "supportive",
}

local cooldownCategoryRoles = {
    ["essential"] = { "cooldown" },
    ["utility"] = { "utility" },
    ["tracked-buff"] = { "maintenance" },
    ["tracked-bar"] = { "maintenance" },
    ["group-buff"] = { "supportive", "maintenance" },
    ["spec-essential"] = { "cooldown" },
    ["spec-tracked"] = { "maintenance" },
}

local spenderPowerTokens = {
    RAGE = true,
    COMBO_POINTS = true,
    RUNIC_POWER = true,
    SOUL_SHARDS = true,
    LUNAR_POWER = true,
    HOLY_POWER = true,
    MAELSTROM = true,
    CHI = true,
    INSANITY = true,
    ARCANE_CHARGES = true,
    FURY = true,
    PAIN = true,
    ESSENCE = true,
}

local function isSecret(value)
    return value ~= nil and issecretvalue and issecretvalue(value)
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

local function addUnique(result, seen, role)
    if role and not seen[role] then
        seen[role] = true
        table.insert(result, role)
        return true
    end
    return false
end

local function activeProfileID(addon)
    return addon.destinationProfile and addon.destinationProfile.id
        or (addon.db and addon.db.activeProfile)
        or "default"
end

local function inferSpellRoles(spellID, spellName, metadata, action)
    local result = {}
    local seen = {}
    local sources, seenSources = {}, {}
    local function addSource(source)
        if source and not seenSources[source] then
            seenSources[source] = true
            table.insert(sources, source)
        end
    end
    local roleCount = #result
    for _, category in ipairs(metadata and metadata.cooldownViewerCategories or {}) do
        for _, role in ipairs(cooldownCategoryRoles[category] or {}) do
            addUnique(result, seen, role)
        end
    end
    if #result > roleCount then addSource("Blizzard cooldown category") end
    roleCount = #result
    for _, powerCost in ipairs(metadata and metadata.powerCosts or {}) do
        local token = type(powerCost.name) == "string" and powerCost.name:upper() or nil
        local cost = math.max(tonumber(powerCost.cost) or 0, tonumber(powerCost.minCost) or 0)
        if token and spenderPowerTokens[token] and cost > 0 then
            addUnique(result, seen, "spender")
        end
    end
    if #result > roleCount then addSource("resource cost") end
    roleCount = #result

    if (action and action.isInterrupt == true)
        or (metadata and metadata.isInterrupt == true)
    then
        addUnique(result, seen, "interrupt")
    end
    if #result > roleCount then addSource("action-bar semantic flag") end
    roleCount = #result

    local isCrowdControl = metadata and metadata.isCrowdControl
    if isCrowdControl == nil then
        isCrowdControl = safeCall(C_Spell and C_Spell.IsSpellCrowdControl, spellID)
    end
    if isCrowdControl == true then
        addUnique(result, seen, "cc")
    end
    if #result > roleCount then addSource("spell semantic flag") end
    roleCount = #result

    -- Blizzard categories and spell flags are locale-independent. Tooltip
    -- language is used only when its vocabulary is known; other locales fall
    -- back to those APIs instead of silently applying English assumptions.
    local locale = safeCall(GetLocale)
    local description = ""
    if locale == "enUS" or locale == "enGB" then
        description = safeCall(C_Spell and C_Spell.GetSpellDescription, spellID)
        description = type(description) == "string" and description:lower() or ""
    end
    if description:find("interrupt", 1, true) or description:find("spellcasting", 1, true) then
        addUnique(result, seen, "interrupt")
    end
    if description:find("restores", 1, true) and description:find("health", 1, true)
        or description:find("heals", 1, true)
        or description:find("heal ", 1, true)
    then
        addUnique(result, seen, "healing")
    end
    if description:find("damage taken", 1, true)
        or description:find("absorbs", 1, true)
        or description:find("immune", 1, true)
        or description:find("armor", 1, true)
    then
        addUnique(result, seen, "defensive")
    end
    if description:find("movement speed", 1, true)
        or description:find("leap", 1, true)
        or description:find("charge to", 1, true)
        or description:find("dash", 1, true)
    then
        addUnique(result, seen, "movement")
    end
    if description:find("removes", 1, true)
        and (description:find("magic", 1, true)
            or description:find("curse", 1, true)
            or description:find("poison", 1, true)
            or description:find("disease", 1, true)
            or description:find("enrage", 1, true))
    then
        addUnique(result, seen, "dispel")
    end
    if description:find("stun", 1, true)
        or description:find("incapacitat", 1, true)
        or description:find("disorient", 1, true)
        or description:find("root", 1, true)
        or description:find("fear", 1, true)
        or description:find("knock", 1, true)
    then
        addUnique(result, seen, "cc")
    end
    if description:find("nearby enemies", 1, true)
        or description:find("all enemies", 1, true)
        or description:find("target area", 1, true)
        or description:find("in a cone", 1, true)
    then
        addUnique(result, seen, "aoe")
    end
    if description:find("combo point", 1, true) then
        if description:find("per combo point", 1, true) or description:find("finishing move", 1, true) then
            addUnique(result, seen, "spender")
        else
            addUnique(result, seen, "builder")
        end
    end
    if description ~= "" and #result > roleCount then addSource("localized tooltip") end
    roleCount = #result

    local isHarmful = metadata and metadata.isHarmful
    if isHarmful == nil then isHarmful = safeCall(C_Spell and C_Spell.IsSpellHarmful, spellID) end
    if isHarmful == true then
        addUnique(result, seen, "offensive")
    end
    local isHelpful = metadata and metadata.isHelpful
    if isHelpful == nil then isHelpful = safeCall(C_Spell and C_Spell.IsSpellHelpful, spellID) end
    if isHelpful == true then
        addUnique(result, seen, "supportive")
    end
    if #result > roleCount then addSource("spell targeting flag") end

    if #result == 0 then
        addUnique(result, seen, "utility")
        addSource("generic fallback")
    end
    return result, sources
end

local function getDestinationDefinitions(addon)
    local result = {}
    if not addon.GetManagedDestinations then
        return result
    end
    for _, destination in ipairs(addon:GetManagedDestinations()) do
        table.insert(result, {
            key = destination.id,
            label = destination.label,
            inputKey = destination.inputKey,
            category = destination.category,
            bindingCommand = destination.bindingCommand,
        })
    end
    return result
end

local function destinationLookup(addon)
    local result = {}
    for _, destination in ipairs(getDestinationDefinitions(addon)) do
        result[destination.key] = destination
    end
    return result
end

local function actionKey(action)
    if not action or not action.actionType or not action.actionID then
        return nil
    end
    if action.actionSubType == "assistedcombat" then
        return nil
    end
    if action.actionType == "macro" then
        return "macro:" .. tostring(action.macroName or action.actionName or action.actionID)
    end
    return action.actionType .. ":" .. tostring(action.actionID)
end

local function normalizeAction(action, metadata)
    local key = actionKey(action)
    if not key then
        return nil
    end
    local roles, roleSources = action.roles, action.roleSources
    if not roles and action.actionType == "spell" then
        roles, roleSources = inferSpellRoles(action.actionID, action.actionName, metadata, action)
    end
    roles = roles or { "utility" }
    return {
        key = key,
        actionType = action.actionType,
        actionID = action.actionID,
        baseSpellID = action.baseSpellID
            or (metadata and (metadata.baseSpellID or metadata.actionID or metadata.spellID)),
        actionSubType = action.actionSubType,
        actionName = action.actionName,
        macroName = action.macroName,
        roles = roles,
        roleSources = roleSources,
    }
end

local function buildSpellMetadataLookup(snapshot)
    local result = {}
    for _, spell in ipairs(snapshot and snapshot.spellbook and snapshot.spellbook.items or {}) do
        if spell.actionID then result["id:" .. tostring(spell.actionID)] = spell end
        if spell.spellID then result["id:" .. tostring(spell.spellID)] = spell end
        if spell.overrideSpellID then result["id:" .. tostring(spell.overrideSpellID)] = spell end
        if spell.name then result["name:" .. spell.name:lower()] = spell end
    end
    return result
end

local function getActionMetadata(lookup, action)
    return action and ((action.actionID and lookup["id:" .. tostring(action.actionID)])
        or (action.actionName and lookup["name:" .. action.actionName:lower()]))
end

local function currentContext(addon, snapshot)
    local identity = addon:GetCharacterIdentity()
    local state = snapshot and snapshot.actionBarState or {}
    local spellbook = snapshot and snapshot.spellbook or {}
    local stateLabel = state.stateKey
        or ((state.formName or "Default") .. "-page" .. tostring(state.effectivePage or "unknown"))
    local stateKey = state.stateID or stateLabel
    local specKey = identity.class .. ":" .. tostring(identity.specializationID)
    local profileID = activeProfileID(addon)
    local baseContextKey = specKey .. ":" .. stateKey
    local legacyContextKey = specKey .. ":" .. stateLabel
    local activeConfigID = spellbook.activeConfigID or 0
    local activeHeroSpecID = spellbook.activeHeroSpecID or 0
    local loadoutContextKey = baseContextKey .. "|loadout:"
        .. tostring(activeConfigID) .. ":" .. tostring(activeHeroSpecID)
    local legacyLoadoutContextKey = legacyContextKey .. "|loadout:"
        .. tostring(activeConfigID) .. ":" .. tostring(activeHeroSpecID)
    local profileLegacyContextKey = legacyContextKey .. "|" .. profileID
    local contextKey = loadoutContextKey .. "|" .. profileID
    local legacyProfileContextKey = legacyLoadoutContextKey .. "|" .. profileID
    return {
        characterKey = identity.key,
        class = identity.class,
        specializationID = identity.specializationID,
        specialization = identity.specialization,
        stateKey = stateKey,
        stateLabel = stateLabel,
        specKey = specKey,
        profileID = profileID,
        activeConfigID = activeConfigID,
        activeHeroSpecID = activeHeroSpecID,
        contextKey = contextKey,
        loadoutContextKey = loadoutContextKey,
        legacyLoadoutContextKey = legacyLoadoutContextKey,
        legacyProfileContextKey = legacyProfileContextKey,
        profileLegacyContextKey = profileLegacyContextKey,
        legacyContextKey = legacyContextKey,
        setupKey = identity.key .. "|" .. contextKey,
        legacySetupKey = identity.key .. "|" .. legacyContextKey,
        overrideActive = state.overrideActive and true or false,
    }
end

local function migrateLegacyContextEvidence(model, context)
    if not context.legacyContextKey or context.legacyContextKey == context.contextKey then return end
    local candidateKeys = {
        context.legacyProfileContextKey,
        context.legacyLoadoutContextKey,
        context.loadoutContextKey,
        context.profileLegacyContextKey,
        context.legacyContextKey,
    }
    for _, node in pairs(model.actionWeights or {}) do
        if node.byContext and not node.byContext[context.contextKey] then
            for _, candidateKey in ipairs(candidateKeys) do
                if candidateKey and candidateKey ~= context.contextKey and node.byContext[candidateKey] then
                    node.byContext[context.contextKey] = node.byContext[candidateKey]
                    node.byContext[candidateKey] = nil
                    break
                end
            end
        end
    end
    if model.contextSetups and not model.contextSetups[context.setupKey] then
        local setupCandidateKeys = {
            context.characterKey .. "|" .. tostring(context.legacyProfileContextKey),
            context.characterKey .. "|" .. tostring(context.legacyLoadoutContextKey),
            context.characterKey .. "|" .. tostring(context.loadoutContextKey),
            context.characterKey .. "|" .. tostring(context.profileLegacyContextKey),
            context.legacySetupKey,
        }
        for _, setupKey in ipairs(setupCandidateKeys) do
            local legacySetup = model.contextSetups[setupKey]
            if legacySetup then
                legacySetup.contextKey = context.contextKey
                legacySetup.profileID = context.profileID
                legacySetup.activeConfigID = context.activeConfigID
                legacySetup.activeHeroSpecID = context.activeHeroSpecID
                model.contextSetups[context.setupKey] = legacySetup
                model.contextSetups[setupKey] = nil
                break
            end
        end
    end
end

local function collectCurrentPlacements(addon, snapshot)
    local placements = {}
    local spellMetadata = buildSpellMetadataLookup(snapshot)
    for _, destination in ipairs(getDestinationDefinitions(addon)) do
        local actionData = snapshot.destinations and snapshot.destinations[destination.key]
        if not actionData and destination.inputKey and addon.InspectKey then
            actionData = addon:InspectKey(destination.inputKey)
        end
        local action = normalizeAction(actionData, getActionMetadata(spellMetadata, actionData))
        if action then
            placements[destination.key] = action
        end
    end

    return placements
end

local function placementFingerprint(placements)
    local parts = {}
    for destination, action in pairs(placements) do
        table.insert(parts, destination .. "=" .. action.key)
    end
    table.sort(parts)
    return table.concat(parts, "|")
end

local function adjust(map, key, amount)
    map[key] = (map[key] or 0) + amount
end

local function nestedMap(parent, key)
    parent[key] = parent[key] or {}
    return parent[key]
end

local function mergeRoles(existing, incoming)
    local result, seen, added = {}, {}, {}
    for _, role in ipairs(existing or {}) do
        if not seen[role] then
            seen[role] = true
            table.insert(result, role)
        end
    end
    for _, role in ipairs(incoming or {}) do
        if not seen[role] then
            seen[role] = true
            table.insert(result, role)
            table.insert(added, role)
        end
    end
    return result, added
end

local function registerAction(model, action)
    local node = model.actionWeights[action.key] or {
        key = action.key,
        actionType = action.actionType,
        actionID = action.actionID,
        actionName = action.actionName,
        roles = action.roles,
        global = {},
        bySpec = {},
        byContext = {},
        confirmations = 0,
        rejections = 0,
    }
    node.actionName = action.actionName or node.actionName
    node.roles = mergeRoles(node.roles, action.roles)
    model.actionWeights[action.key] = node
    return node
end

local function enrichExistingPlacement(model, context, destination, action)
    local node = model.actionWeights[action.key]
    if not node then return 0 end
    local merged, added = mergeRoles(node.roles, action.roles)
    node.roles = merged
    local profileRoles = nestedMap(model.roleWeightsByProfile, context.profileID)
    for _, role in ipairs(added) do
        adjust(nestedMap(model.roleWeights, role), destination, positiveWeights.role)
        adjust(nestedMap(profileRoles, role), destination, positiveWeights.profileRole)
    end
    return #added
end

local function applyPlacementEvidence(model, context, destination, action, positive)
    local node = registerAction(model, action)
    local weights = positive and positiveWeights or negativeWeights
    adjust(node.global, destination, weights.global)
    adjust(nestedMap(node.bySpec, context.specKey), destination, weights.spec)
    adjust(nestedMap(node.byContext, context.contextKey), destination, weights.context)
    if positive then
        node.confirmations = node.confirmations + 1
        model.totalPlacementChoices = model.totalPlacementChoices + 1
    else
        node.rejections = node.rejections + 1
    end

    local profileRoles = nestedMap(model.roleWeightsByProfile, context.profileID)
    for _, role in ipairs(node.roles or {}) do
        local roleMap = nestedMap(model.roleWeights, role)
        adjust(roleMap, destination, weights.role)
        adjust(nestedMap(profileRoles, role), destination, weights.profileRole)
    end
    adjust(nestedMap(model.destinationWeightsByProfile, context.profileID), destination, weights.destination)
end

local function copyPlacementKeys(placements)
    local result = {}
    for destination, action in pairs(placements) do
        result[destination] = action.key
    end
    return result
end

local function countTableEntries(value)
    local count = 0
    for _ in pairs(value or {}) do
        count = count + 1
    end
    return count
end

local function destinationRoleScore(model, destination, roles, profileID)
    local score = 0
    local reason
    local profileRules = model.preferredRulesByProfile[profileID] or {}
    for _, role in ipairs(roles or {}) do
        local preference = profileRules[role]
        if preference and preference.destination == destination.key then
            score = score + (preference.weight or 12)
            -- Provenance is stored for import compatibility, but it may contain
            -- an old device label or another person's profile name. Explanations
            -- describe the evidence without exposing that private metadata.
            reason = "Saved " .. role .. " preference for this profile"
        end
    end
    return score, reason
end

local function scoreCandidate(model, action, destination, context, explain)
    local score = 0
    local reasons = explain and {} or nil
    local function addReason(reason)
        if reasons then table.insert(reasons, reason) end
    end
    local hasNegativeEvidence = false
    local node = model.actionWeights[action.key]
    if node then
        local contextWeights = node.byContext[context.contextKey]
            or (context.legacyContextKey and node.byContext[context.legacyContextKey])
        local contextScore = contextWeights and contextWeights[destination.key] or 0
        local specScore = node.bySpec[context.specKey] and node.bySpec[context.specKey][destination.key] or 0
        local globalScore = node.global[destination.key] or 0
        score = score + contextScore + specScore + globalScore
        hasNegativeEvidence = contextScore < 0 or specScore < 0 or globalScore < 0
        if contextScore > 0 then
            addReason("same form/spec history")
        elseif specScore > 0 then
            addReason("same-spec history")
        elseif globalScore > 0 then
            addReason("cross-character history")
        end
    end

    local profileRoleWeights = model.roleWeightsByProfile[context.profileID] or {}
    for _, role in ipairs(action.roles or {}) do
        local profileRoleScore = profileRoleWeights[role] and profileRoleWeights[role][destination.key] or 0
        local learnedRoleScore = model.roleWeights[role] and model.roleWeights[role][destination.key] or 0
        score = score + profileRoleScore + learnedRoleScore
        hasNegativeEvidence = hasNegativeEvidence or profileRoleScore < 0 or learnedRoleScore < 0
        if profileRoleScore > 0 then
            addReason("this profile's learned " .. (roleLabels[role] or role) .. " pattern")
        elseif learnedRoleScore > 0 then
            addReason("learned " .. (roleLabels[role] or role) .. " pattern")
        end
    end

    local defaultScore, defaultReason = destinationRoleScore(model, destination, action.roles, context.profileID)
    score = score + defaultScore
    if defaultReason then
        addReason(defaultReason)
    end

    local destinationWeights = model.destinationWeightsByProfile[context.profileID] or {}
    local destinationWeight = destinationWeights[destination.key] or 0
    if destinationWeight > 0 then
        score = score + (destinationWeight * destinationUsageScore)
        addReason("this profile uses this input")
    elseif destinationWeight < 0 then
        hasNegativeEvidence = true
    end

    local evidence = model.profileEvidence[context.profileID]
    local completionFallback = false
    if score == 0 and not hasNegativeEvidence and evidence and (evidence.confirmedSetups or 0) > 0 then
        score = completionFallbackScore
        completionFallback = true
        addReason("low-confidence profile completion fallback")
    end
    return score, reasons and table.concat(reasons, ", ") or "", completionFallback
end

-- Finds the globally best one-action-per-destination layout. A greedy picker
-- can waste a destination needed by another spell. Keeping the smaller side
-- as the Hungarian algorithm's row set avoids padding the common rectangular
-- spellbook/input matrix to a much larger square.
local function maximumWeightAssignment(rowCount, columnCount, weights)
    if rowCount == 0 or columnCount == 0 then
        return {}
    end
    local transposed = rowCount > columnCount
    local rowSize = transposed and columnCount or rowCount
    local columnSize = transposed and rowCount or columnCount
    local function getWeight(row, column)
        if transposed then
            return weights[column] and (weights[column][row] or 0) or 0
        end
        return weights[row] and (weights[row][column] or 0) or 0
    end
    local u, v, p, way = {}, {}, {}, {}
    for index = 0, rowSize do u[index] = 0 end
    for index = 0, columnSize do
        v[index], p[index], way[index] = 0, 0, 0
    end

    for row = 1, rowSize do
        p[0] = row
        local column = 0
        local minimums, used = {}, {}
        for index = 0, columnSize do
            minimums[index] = math.huge
            used[index] = false
        end
        repeat
            used[column] = true
            local activeRow = p[column]
            local delta, nextColumn = math.huge, 0
            for candidateColumn = 1, columnSize do
                if not used[candidateColumn] then
                    local weight = getWeight(activeRow, candidateColumn)
                    local reducedCost = -weight - u[activeRow] - v[candidateColumn]
                    if reducedCost < minimums[candidateColumn] then
                        minimums[candidateColumn] = reducedCost
                        way[candidateColumn] = column
                    end
                    if minimums[candidateColumn] < delta then
                        delta = minimums[candidateColumn]
                        nextColumn = candidateColumn
                    end
                end
            end
            for candidateColumn = 0, columnSize do
                if used[candidateColumn] then
                    u[p[candidateColumn]] = u[p[candidateColumn]] + delta
                    v[candidateColumn] = v[candidateColumn] - delta
                else
                    minimums[candidateColumn] = minimums[candidateColumn] - delta
                end
            end
            column = nextColumn
        until p[column] == 0

        repeat
            local previousColumn = way[column]
            p[column] = p[previousColumn]
            column = previousColumn
        until column == 0
    end

    local result = {}
    for column = 1, columnSize do
        local assignedRow = p[column]
        if assignedRow > 0 and getWeight(assignedRow, column) > 0 then
            local actionIndex = transposed and column or assignedRow
            local destinationIndex = transposed and assignedRow or column
            result[actionIndex] = destinationIndex
        end
    end
    return result
end

local function isFiniteWeight(value)
    return type(value) == "number" and value == value and math.abs(value) <= 1000000
end

local function safeCounter(value)
    local number = tonumber(value)
    return isFiniteWeight(number) and math.max(0, number) or 0
end

local function sanitizeWeightMap(value)
    if type(value) ~= "table" then
        return {}
    end
    for key, weight in pairs(value) do
        if type(key) ~= "string" or not isFiniteWeight(weight) then
            value[key] = nil
        end
    end
    return value
end

local function sanitizeNestedWeightMap(value)
    if type(value) ~= "table" then
        return {}
    end
    for key, weights in pairs(value) do
        if type(key) ~= "string" or type(weights) ~= "table" then
            value[key] = nil
        else
            sanitizeWeightMap(weights)
        end
    end
    return value
end

local function sanitizeRoleWeights(value)
    return sanitizeNestedWeightMap(value)
end

local function sanitizePreferredRules(value)
    if type(value) ~= "table" then return {} end
    for role, rule in pairs(value) do
        if type(role) ~= "string" or type(rule) ~= "table"
            or type(rule.destination) ~= "string" or rule.destination == ""
        then
            value[role] = nil
        else
            rule.weight = isFiniteWeight(tonumber(rule.weight)) and tonumber(rule.weight) or nil
            rule.label = type(rule.label) == "string" and rule.label or nil
            rule.source = type(rule.source) == "string" and rule.source or nil
            if rule.source and rule.source:match("^%a[%a'-]*%s+[%w%-]+%s+starter$") then
                rule.source = "imported"
            end
        end
    end
    return value
end

local function sanitizeRoles(value)
    local result, seen = {}, {}
    if type(value) == "table" then
        for _, role in ipairs(value) do
            if type(role) == "string" and role ~= "" and #role <= 50 and not seen[role] then
                seen[role] = true
                table.insert(result, role)
            end
        end
    end
    return result
end

local function sanitizeModelContents(model)
    for actionKey, node in pairs(model.actionWeights) do
        if type(actionKey) ~= "string" or type(node) ~= "table" then
            model.actionWeights[actionKey] = nil
        else
            node.key = actionKey
            node.global = sanitizeWeightMap(node.global)
            node.bySpec = sanitizeNestedWeightMap(node.bySpec)
            node.byContext = sanitizeNestedWeightMap(node.byContext)
            node.roles = sanitizeRoles(node.roles)
            node.confirmations = safeCounter(node.confirmations)
            node.rejections = safeCounter(node.rejections)
        end
    end
    model.roleWeights = sanitizeRoleWeights(model.roleWeights)
    for profileID, roles in pairs(model.roleWeightsByProfile) do
        if type(profileID) ~= "string" or type(roles) ~= "table" then
            model.roleWeightsByProfile[profileID] = nil
        else
            sanitizeRoleWeights(roles)
        end
    end
    for profileID, destinations in pairs(model.destinationWeightsByProfile) do
        if type(profileID) ~= "string" or type(destinations) ~= "table" then
            model.destinationWeightsByProfile[profileID] = nil
        else
            sanitizeWeightMap(destinations)
        end
    end
    for profileID, evidence in pairs(model.profileEvidence) do
        if type(profileID) ~= "string" or type(evidence) ~= "table" then
            model.profileEvidence[profileID] = nil
        else
            evidence.confirmedSetups = safeCounter(evidence.confirmedSetups)
            evidence.placementChoices = safeCounter(evidence.placementChoices)
        end
    end
    local rankedSetups = {}
    for setupKey, setup in pairs(model.contextSetups) do
        if type(setupKey) ~= "string" or #setupKey > 1000 or setupKey:find("%c")
            or type(setup) ~= "table"
        then
            model.contextSetups[setupKey] = nil
        elseif type(setup.placements) ~= "table" then
            setup.placements = {}
        else
            for destination, learnedActionKey in pairs(setup.placements) do
                if type(destination) ~= "string" or type(learnedActionKey) ~= "string" then
                    setup.placements[destination] = nil
                end
            end
        end
        if model.contextSetups[setupKey] then
            table.insert(rankedSetups, {
                key = setupKey,
                confirmedAt = tonumber(setup.confirmedAt) or 0,
            })
        end
    end
    table.sort(rankedSetups, function(left, right)
        if left.confirmedAt == right.confirmedAt then return left.key < right.key end
        return left.confirmedAt > right.confirmedAt
    end)
    for index = maximumContextSetups + 1, #rankedSetups do
        model.contextSetups[rankedSetups[index].key] = nil
    end
    model.preferredRules = sanitizePreferredRules(model.preferredRules)
    for profileID, rules in pairs(model.preferredRulesByProfile) do
        if type(profileID) ~= "string" or type(rules) ~= "table" then
            model.preferredRulesByProfile[profileID] = nil
        else
            sanitizePreferredRules(rules)
        end
    end
    model.totalConfirmedSetups = safeCounter(model.totalConfirmedSetups)
    model.totalPlacementChoices = safeCounter(model.totalPlacementChoices)
end

local function migrateTransferEvidence(addon, model, priorVersion)
    if priorVersion < 2 then
        local migratedSetups = 0
        for _, setup in pairs(model.contextSetups) do
            if type(setup) == "table" then
                local profileID = setup.profileID or activeProfileID(addon)
                local evidence = nestedMap(model.profileEvidence, profileID)
                evidence.confirmedSetups = (tonumber(evidence.confirmedSetups) or 0) + 1
                local profileDestinations = nestedMap(model.destinationWeightsByProfile, profileID)
                local profileRoles = nestedMap(model.roleWeightsByProfile, profileID)
                local placementCount = 0
                for destination, learnedActionKey in pairs(type(setup.placements) == "table" and setup.placements or {}) do
                    if type(destination) == "string" then
                        placementCount = placementCount + 1
                        adjust(profileDestinations, destination, positiveWeights.destination)
                        local node = model.actionWeights[learnedActionKey]
                        for _, role in ipairs(type(node) == "table" and type(node.roles) == "table" and node.roles or {}) do
                            if type(role) == "string" then
                                adjust(nestedMap(profileRoles, role), destination, positiveWeights.profileRole)
                            end
                        end
                    end
                end
                evidence.placementChoices = (tonumber(evidence.placementChoices) or 0) + placementCount
                migratedSetups = migratedSetups + 1
            end
        end

        -- Portable profiles from the earlier model intentionally omitted local
        -- context records. Preserve their aggregate learning and infer an input
        -- usage prior from positive shared action evidence for the selected profile.
        if migratedSetups == 0 and model.totalConfirmedSetups > 0 then
            local profileID = activeProfileID(addon)
            local evidence = nestedMap(model.profileEvidence, profileID)
            evidence.confirmedSetups = model.totalConfirmedSetups
            evidence.placementChoices = model.totalPlacementChoices
            local profileDestinations = nestedMap(model.destinationWeightsByProfile, profileID)
            local profileRoles = nestedMap(model.roleWeightsByProfile, profileID)
            for role, destinations in pairs(model.roleWeights) do
                if type(role) == "string" and type(destinations) == "table" then
                    for destination, weight in pairs(destinations) do
                        if type(destination) == "string" and type(weight) == "number" and weight > 0 then
                            adjust(nestedMap(profileRoles, role), destination, weight)
                        end
                    end
                end
            end
            for _, node in pairs(model.actionWeights) do
                for destination, weight in pairs(type(node) == "table" and type(node.global) == "table" and node.global or {}) do
                    if type(destination) == "string" and type(weight) == "number" and weight > 0 then
                        adjust(profileDestinations, destination, weight * positiveWeights.destination)
                    end
                end
            end
        end
    end

    if priorVersion < 3 and next(model.preferredRules) ~= nil then
        local profileRules = nestedMap(model.preferredRulesByProfile, activeProfileID(addon))
        for role, rule in pairs(model.preferredRules) do
            if profileRules[role] == nil then profileRules[role] = rule end
        end
        model.preferredRules = {}
    end
end

function Addon:InitializeTrainingModel()
    local model = type(self.db.trainingModel) == "table" and self.db.trainingModel or {}
    local priorVersion = tonumber(model.version) or 0
    model.actionWeights = type(model.actionWeights) == "table" and model.actionWeights or {}
    model.roleWeights = type(model.roleWeights) == "table" and model.roleWeights or {}
    model.roleWeightsByProfile = type(model.roleWeightsByProfile) == "table" and model.roleWeightsByProfile or {}
    model.destinationWeightsByProfile = type(model.destinationWeightsByProfile) == "table" and model.destinationWeightsByProfile or {}
    model.profileEvidence = type(model.profileEvidence) == "table" and model.profileEvidence or {}
    model.preferredRulesByProfile = type(model.preferredRulesByProfile) == "table" and model.preferredRulesByProfile or {}
    model.contextSetups = type(model.contextSetups) == "table" and model.contextSetups or {}
    model.sessions = type(model.sessions) == "table" and model.sessions or {}
    model.totalConfirmedSetups = tonumber(model.totalConfirmedSetups) or 0
    model.totalPlacementChoices = tonumber(model.totalPlacementChoices) or 0
    model.preferredRules = type(model.preferredRules) == "table" and model.preferredRules
        or (type(model.hardRules) == "table" and model.hardRules or {})
    model.hardRules = nil
    if self.trainingModelInitialized ~= model then
        sanitizeModelContents(model)
        migrateTransferEvidence(self, model, priorVersion)
        self.trainingModelInitialized = model
    end
    model.version = modelVersion
    self.db.trainingModel = model
    return model
end

function Addon:RemoveTrainingProfileEvidence(profileID)
    if type(profileID) ~= "string" or profileID == "" then return false end
    local model = self:InitializeTrainingModel()
    model.roleWeightsByProfile[profileID] = nil
    model.destinationWeightsByProfile[profileID] = nil
    model.profileEvidence[profileID] = nil
    model.preferredRulesByProfile[profileID] = nil

    local suffix = "|" .. profileID
    for setupKey, setup in pairs(model.contextSetups) do
        if (type(setup) == "table" and setup.profileID == profileID)
            or (type(setupKey) == "string" and setupKey:sub(-#suffix) == suffix)
        then
            model.contextSetups[setupKey] = nil
        end
    end
    for _, node in pairs(model.actionWeights) do
        if type(node) == "table" and type(node.byContext) == "table" then
            for contextKey in pairs(node.byContext) do
                if type(contextKey) == "string" and contextKey:sub(-#suffix) == suffix then
                    node.byContext[contextKey] = nil
                end
            end
        end
    end
    for index = #model.sessions, 1, -1 do
        local session = model.sessions[index]
        if type(session) == "table" and session.profileID == profileID then
            table.remove(model.sessions, index)
        end
    end
    self.trainingRecommendations = nil
    self.trainingCoverage = nil
    self.layoutPlan = nil
    return true
end

function Addon:GetTrainingContext(snapshot)
    return currentContext(self, snapshot or self.currentSnapshot)
end

function Addon:GetTrainingSummary()
    local model = self:InitializeTrainingModel()
    local profileID = activeProfileID(self)
    local profileEvidence = model.profileEvidence[profileID] or {}
    return {
        confirmedSetups = model.totalConfirmedSetups,
        profileConfirmedSetups = tonumber(profileEvidence.confirmedSetups) or 0,
        profilePlacementChoices = tonumber(profileEvidence.placementChoices) or 0,
        placementChoices = model.totalPlacementChoices,
        learnedActions = countTableEntries(model.actionWeights),
        sessions = #model.sessions,
        lastSession = model.sessions[#model.sessions],
    }
end

function Addon:GetDestinationRoleSummary(destinationKey, maximumRoles)
    local model = self:InitializeTrainingModel()
    local profileID = activeProfileID(self)
    local profileRoleWeights = model.roleWeightsByProfile[profileID] or {}
    local profilePreferredRules = model.preferredRulesByProfile[profileID] or {}
    local ranked = {}
    local roles = {}
    for role in pairs(model.roleWeights or {}) do roles[role] = true end
    for role in pairs(profileRoleWeights) do roles[role] = true end
    for role in pairs(profilePreferredRules) do roles[role] = true end
    for role in pairs(roles) do
        local destinations = model.roleWeights[role] or {}
        local profileDestinations = profileRoleWeights[role] or {}
        local score = (destinations[destinationKey] or 0) + (profileDestinations[destinationKey] or 0)
        local preference = profilePreferredRules[role]
        if preference and preference.destination == destinationKey then
            score = score + (preference.weight or 12)
        end
        if score > 0 then
            table.insert(ranked, {
                role = role,
                label = roleLabels[role] or role,
                score = score,
            })
        end
    end
    table.sort(ranked, function(left, right)
        if left.score == right.score then return left.label < right.label end
        return left.score > right.score
    end)
    local labels = {}
    for index = 1, math.min(maximumRoles or 2, #ranked) do
        table.insert(labels, ranked[index].label)
    end
    return #labels > 0 and table.concat(labels, " / ") or nil, ranked
end

function Addon:GetCurrentTrainingPlacements(snapshot)
    snapshot = snapshot or self.currentSnapshot
    if not snapshot then
        return {}
    end
    return collectCurrentPlacements(self, snapshot)
end

function Addon:GetTrainingSetupState(snapshot)
    snapshot = snapshot or self.currentSnapshot
    if not snapshot then
        return false, 0
    end
    local model = self:InitializeTrainingModel()
    local context = currentContext(self, snapshot)
    migrateLegacyContextEvidence(model, context)
    local placements = collectCurrentPlacements(self, snapshot)
    local prior = model.contextSetups[context.setupKey]
        or (context.legacySetupKey and model.contextSetups[context.legacySetupKey])
    local learned = prior and prior.fingerprint == placementFingerprint(placements) or false
    return learned, countTableEntries(placements), context
end

function Addon:LearnCurrentSetup()
    if InCombatLockdown and InCombatLockdown() then
        self:SetStatus("Cannot learn a setup during combat")
        return false
    end
    local snapshot = self:CaptureSnapshot("confirmed training setup")
    if not snapshot then
        return false
    end
    local context = currentContext(self, snapshot)
    if context.overrideActive then
        self:SetStatus("Leave the vehicle or temporary action bar before learning this setup")
        return false
    end

    local model = self:InitializeTrainingModel()
    local placements = collectCurrentPlacements(self, snapshot)
    local fingerprint = placementFingerprint(placements)
    migrateLegacyContextEvidence(model, context)
    local prior = model.contextSetups[context.setupKey]
        or (context.legacySetupKey and model.contextSetups[context.legacySetupKey])
    if prior and prior.fingerprint == fingerprint then
        local enriched = 0
        for destination, action in pairs(placements) do
            enriched = enriched + enrichExistingPlacement(model, context, destination, action)
        end
        if enriched > 0 then
            self:SetStatus("This setup was already learned • Updated " .. countLabel(enriched, "ability description"))
        else
            self:SetStatus("This setup is already learned • Nothing changed")
        end
        return true, { duplicate = true, learned = 0, rejected = 0, enriched = enriched }
    end

    local destinations = {}
    for destination in pairs(placements) do
        destinations[destination] = true
    end
    for destination in pairs(prior and prior.placements or {}) do
        destinations[destination] = true
    end

    local learned = 0
    local rejected = 0
    for destination in pairs(destinations) do
        local newAction = placements[destination]
        local oldKey = prior and prior.placements[destination]
        local newKey = newAction and newAction.key
        if oldKey ~= newKey then
            if oldKey and model.actionWeights[oldKey] then
                local oldNode = model.actionWeights[oldKey]
                applyPlacementEvidence(model, context, destination, {
                    key = oldKey,
                    actionType = oldNode.actionType,
                    actionID = oldNode.actionID,
                    actionName = oldNode.actionName,
                    roles = oldNode.roles,
                }, false)
                rejected = rejected + 1
            end
            if newAction then
                applyPlacementEvidence(model, context, destination, newAction, true)
                learned = learned + 1
            end
        end
    end

    model.totalConfirmedSetups = model.totalConfirmedSetups + 1
    local profileEvidence = nestedMap(model.profileEvidence, context.profileID)
    profileEvidence.confirmedSetups = (tonumber(profileEvidence.confirmedSetups) or 0) + 1
    profileEvidence.placementChoices = (tonumber(profileEvidence.placementChoices) or 0) + learned
    model.contextSetups[context.setupKey] = {
        fingerprint = fingerprint,
        contextKey = context.contextKey,
        specKey = context.specKey,
        stateKey = context.stateKey,
        profileID = context.profileID,
        activeConfigID = context.activeConfigID,
        activeHeroSpecID = context.activeHeroSpecID,
        placements = copyPlacementKeys(placements),
        confirmedAt = time(),
        confirmedAtText = date("%Y-%m-%d %H:%M:%S"),
    }
    if context.legacySetupKey and context.legacySetupKey ~= context.setupKey then
        model.contextSetups[context.legacySetupKey] = nil
    end
    local session = {
        confirmedAt = time(),
        confirmedAtText = date("%Y-%m-%d %H:%M:%S"),
        characterKey = context.characterKey,
        class = context.class,
        specializationID = context.specializationID,
        stateKey = context.stateKey,
        profileID = context.profileID,
        activeConfigID = context.activeConfigID,
        activeHeroSpecID = context.activeHeroSpecID,
        learned = learned,
        rejected = rejected,
        placements = countTableEntries(placements),
    }
    table.insert(model.sessions, session)
    while #model.sessions > 100 do
        table.remove(model.sessions, 1)
    end

    self.trainingRecommendations = nil
    self:SetStatus("Learned " .. countLabel(learned, "placement")
        .. " • Replaced " .. countLabel(rejected, "older preference"))
    if self.RefreshTraining then
        self:RefreshTraining()
    end
    return true, session
end

function Addon:BuildTrainingRecommendations()
    local snapshot = self.currentSnapshot or self:CaptureSnapshot("training recommendation")
    if not snapshot or not snapshot.spellbook or not snapshot.spellbook.available then
        self.trainingCoverage = {
            eligibleActions = 0,
            destinationCount = 0,
            targetPlacements = 0,
            recommendedPlacements = 0,
            unassignedActions = {},
            complete = false,
            blockedReason = "The active spellbook is unavailable",
        }
        return {}, self.trainingCoverage
    end
    local model = self:InitializeTrainingModel()
    local context = currentContext(self, snapshot)
    migrateLegacyContextEvidence(model, context)
    local destinations = getDestinationDefinitions(self)
    local actions = {}
    local seenActions = {}
    local seenActionNames = {}
    local currentPlacements = collectCurrentPlacements(self, snapshot)

    local spellItemType = Enum and Enum.SpellBookItemType and Enum.SpellBookItemType.Spell
    for _, spell in ipairs(snapshot.spellbook.items or {}) do
        local isUsableSpellEntry = spellItemType == nil or spell.itemType == spellItemType
        local effectiveSpellID = spell.overrideSpellID or spell.spellID or spell.actionID
        local baseSpellID = spell.baseSpellID or spell.actionID or spell.spellID
        local validIdentity = type(effectiveSpellID) == "number" and effectiveSpellID > 0
            and effectiveSpellID % 1 == 0 and type(baseSpellID) == "number"
            and baseSpellID > 0 and baseSpellID % 1 == 0
        local validName = type(spell.name) == "string" and spell.name ~= ""
            and #spell.name <= 300 and not spell.name:find("%c")
        if isUsableSpellEntry and validIdentity and validName
            and not spell.isPassive and not spell.isOffSpec
        then
            local action = normalizeAction({
                actionType = "spell",
                actionID = effectiveSpellID,
                baseSpellID = baseSpellID,
                actionName = spell.name,
            }, spell)
            local normalizedName = action and action.actionName and action.actionName:lower()
            if action and not seenActions[action.key] and not (normalizedName and seenActionNames[normalizedName]) then
                seenActions[action.key] = true
                if normalizedName then seenActionNames[normalizedName] = true end
                table.insert(actions, action)
            end
        end
    end

    table.sort(actions, function(left, right)
        if left.actionName == right.actionName then
            return left.key < right.key
        end
        return left.actionName < right.actionName
    end)

    local weights = {}
    for actionIndex, action in ipairs(actions) do
        weights[actionIndex] = {}
        for destinationIndex, destination in ipairs(destinations) do
            local score = scoreCandidate(model, action, destination, context, false)
            local current = currentPlacements[destination.key]
            if score > 0 and current and current.key == action.key then
                score = score + 0.25
            end
            weights[actionIndex][destinationIndex] = math.max(0, score)
        end
    end

    local assignment = maximumWeightAssignment(#actions, #destinations, weights)
    local result = {}
    local assignedActions = {}
    local fallbackPlacements = 0
    for actionIndex, destinationIndex in pairs(assignment) do
        local action = actions[actionIndex]
        local destination = destinations[destinationIndex]
        local score, reason, completionFallback = scoreCandidate(
            model, action, destination, context, true)
        local current = currentPlacements[destination.key]
        if score > 0 and current and current.key == action.key then
            score = score + 0.25
            reason = reason ~= "" and (reason .. ", already on this input") or "already on this input"
        end
        if score > 0 then
            local candidate = {
                action = action,
                destination = destination,
                score = score,
                reason = reason,
                completionFallback = completionFallback,
                destinationOrder = destinationIndex,
            }
            candidate.confidence = candidate.score >= 12 and "high" or (candidate.score >= 6 and "medium" or "low")
            table.insert(result, candidate)
            assignedActions[actionIndex] = true
            if candidate.completionFallback then
                fallbackPlacements = fallbackPlacements + 1
            end
        end
    end
    table.sort(result, function(left, right)
        if left.destinationOrder == right.destinationOrder then
            return left.action.actionName < right.action.actionName
        end
        return left.destinationOrder < right.destinationOrder
    end)

    local unassignedActions = {}
    local needsEvidence = 0
    local broadRoleFallbacks = 0
    for _, action in ipairs(actions) do
        for _, source in ipairs(action.roleSources or {}) do
            if source == "generic fallback" then
                broadRoleFallbacks = broadRoleFallbacks + 1
                break
            end
        end
    end
    for actionIndex, action in ipairs(actions) do
        if not assignedActions[actionIndex] then
            local bestScore = 0
            for destinationIndex = 1, #destinations do
                bestScore = math.max(bestScore, weights[actionIndex][destinationIndex] or 0)
            end
            local reason
            if #result >= #destinations and #destinations > 0 then
                reason = "No free managed input after stronger matches"
            elseif bestScore > 0 then
                reason = "A stronger complete assignment used the matching inputs"
            else
                reason = "No matching preference learned yet"
                needsEvidence = needsEvidence + 1
            end
            table.insert(unassignedActions, {
                action = action,
                bestScore = bestScore,
                reason = reason,
            })
        end
    end

    local targetPlacements = math.min(#actions, #destinations)
    self.trainingCoverage = {
        contextKey = context.contextKey,
        eligibleActions = #actions,
        destinationCount = #destinations,
        targetPlacements = targetPlacements,
        recommendedPlacements = #result,
        unfilledTargetCount = math.max(0, targetPlacements - #result),
        unfilledDestinations = math.max(0, #destinations - #result),
        capacityLimited = math.max(0, #actions - #destinations),
        needsEvidence = needsEvidence,
        broadRoleFallbacks = broadRoleFallbacks,
        fallbackPlacements = fallbackPlacements,
        unassignedActions = unassignedActions,
        complete = targetPlacements > 0 and #result == targetPlacements,
    }
    return result, self.trainingCoverage
end

function Addon:PreviewTrainingRecommendations()
    self:CaptureSnapshot("training recommendation preview")
    self.trainingRecommendations, self.trainingCoverage = self:BuildTrainingRecommendations()
    local count = #self.trainingRecommendations
    if self.trainingCoverage.complete then
        self:SetStatus("Built a complete preview with " .. countLabel(count, "placement") .. " • Nothing changed")
    else
        self:SetStatus("Built " .. count .. " of " .. self.trainingCoverage.targetPlacements
            .. " possible placements • Learn another setup to improve it")
    end
    if self.RefreshTraining then
        self:RefreshTraining()
    end
    return self.trainingRecommendations, self.trainingCoverage
end

function Addon:ConfirmLearnCurrentSetup()
    return self:LearnCurrentSetup()
end

function Addon:GetTrainingDestinationLookup()
    return destinationLookup(self)
end
