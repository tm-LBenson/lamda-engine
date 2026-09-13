local _, Addon = ...

local bindingFrames = {
    ["1"] = "MultiBarBottomLeftButton",
    ["2"] = "MultiBarBottomRightButton",
    ["3"] = "MultiBarRightButton",
    ["4"] = "MultiBarLeftButton",
    ["5"] = "MultiBar5Button",
    ["6"] = "MultiBar6Button",
    ["7"] = "MultiBar7Button",
}

local fixedMultiBarSlotOffsets = {
    ["1"] = 60,
    ["2"] = 48,
    ["3"] = 24,
    ["4"] = 36,
    ["5"] = 144,
    ["6"] = 156,
    ["7"] = 168,
}

local function isManagedBindingCommand(command)
    if type(command) ~= "string" or command == "" or #command > 500 or command:find("%c") then
        return false
    end
    local mainButton = tonumber(command:match("^ACTIONBUTTON(%d+)$"))
    if mainButton then return mainButton >= 1 and mainButton <= 12 end
    local bar, button = command:match("^MULTIACTIONBAR(%d+)BUTTON(%d+)$")
    if bar and button then
        bar, button = tonumber(bar), tonumber(button)
        return fixedMultiBarSlotOffsets[tostring(bar)] ~= nil and button >= 1 and button <= 12
    end
    if command:match("^SPELL .+") then return not command:find("|", 1, true) end
    if command:match("^CLICK [%w_]+:[^%s]+$") then return true end
    return false
end

function Addon:IsManagedBindingCommand(command)
    return isManagedBindingCommand(command)
end

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

local function getSpellName(spellID)
    if not spellID then
        return nil
    end
    if C_Spell and C_Spell.GetSpellName then
        return C_Spell.GetSpellName(spellID)
    end
    if GetSpellInfo then
        return GetSpellInfo(spellID)
    end
end

local function getItemName(itemID)
    if not itemID then
        return nil
    end
    if C_Item and C_Item.GetItemNameByID then
        return C_Item.GetItemNameByID(itemID)
    end
    if GetItemInfo then
        return GetItemInfo(itemID)
    end
end

local function clickBindingType(name, fallback)
    return Enum and Enum.ClickBindingType and Enum.ClickBindingType[name] or fallback
end

local function validClickButton(button)
    if button == "LeftButton" or button == "RightButton" or button == "MiddleButton" then
        return true
    end
    if type(button) ~= "string" then return false end
    local number = tonumber(button:match("^Button(%d+)$"))
    return number ~= nil and number >= 1 and number <= 31 and number % 1 == 0
end

local function validClickModifiers(modifiers)
    return type(modifiers) == "number" and modifiers >= 0 and modifiers <= 31
        and modifiers % 1 == 0
end

local function parseClickBindingInputKey(inputKey)
    if type(inputKey) ~= "string" then return nil end
    local modifiers, button = inputKey:match("^CLICKCAST:(%d+):([%a]+%d*)$")
    modifiers = tonumber(modifiers)
    if not validClickModifiers(modifiers) or not validClickButton(button) then return nil end
    return button, modifiers
end

local function clickButtonLabel(button)
    local labels = {
        LeftButton = _G.LEFT_BUTTON_STRING,
        Button1 = _G.LEFT_BUTTON_STRING,
        RightButton = _G.RIGHT_BUTTON_STRING,
        Button2 = _G.RIGHT_BUTTON_STRING,
        MiddleButton = _G.MIDDLE_BUTTON_STRING,
        Button3 = _G.MIDDLE_BUTTON_STRING,
    }
    if labels[button] then return labels[button] end
    local number = button and button:match("^Button(%d+)$")
    local globalLabel = number and _G["BUTTON_" .. number .. "_STRING"]
    return globalLabel or (number and ("Mouse Button " .. number)) or tostring(button or "Mouse Button")
end

function Addon:GetClickBindingInputKey(button, modifiers)
    if not validClickButton(button) or not validClickModifiers(modifiers) then return nil end
    return "CLICKCAST:" .. tostring(modifiers) .. ":" .. button
end

function Addon:ParseClickBindingInputKey(inputKey)
    return parseClickBindingInputKey(inputKey)
end

function Addon:FormatClickBindingInput(button, modifiers)
    if not validClickButton(button) or not validClickModifiers(modifiers) then return nil end
    local modifierText = type(GetStringFromModifiers) == "function"
        and GetStringFromModifiers(modifiers) or nil
    if type(modifierText) ~= "string" then modifierText = "" end
    modifierText = modifierText:gsub("%-$", "")
    local buttonLabel = clickButtonLabel(button)
    return modifierText ~= "" and (modifierText .. "+" .. buttonLabel) or buttonLabel
end

function Addon:GetClickBindingProfile()
    if not C_ClickBindings or type(C_ClickBindings.GetProfileInfo) ~= "function" then
        return nil, "Blizzard Click Casting APIs are unavailable"
    end
    local ok, profile = pcall(C_ClickBindings.GetProfileInfo)
    if not ok or type(profile) ~= "table" then
        return nil, "The current Click Casting profile could not be read"
    end
    local recordCount = 0
    for key in pairs(profile) do
        if type(key) ~= "number" or key < 1 or key % 1 ~= 0 then
            return nil, "The current Click Casting profile has an unsupported structure"
        end
        recordCount = recordCount + 1
    end
    for index = 1, recordCount do
        if profile[index] == nil then
            return nil, "The current Click Casting profile is incomplete"
        end
    end
    local knownTypes = {
        [clickBindingType("Spell", 1)] = true,
        [clickBindingType("Macro", 2)] = true,
        [clickBindingType("Interaction", 3)] = true,
        [clickBindingType("PetAction", 4)] = true,
    }
    local knownFields = { type = true, actionID = true, button = true, modifiers = true }
    local result, seenInputs = {}, {}
    for _, binding in ipairs(profile) do
        if type(binding) ~= "table" then
            return nil, "The current Click Casting profile contains an unreadable record"
        end
        for key in pairs(binding) do
            if not knownFields[key] then
                return nil, "The current Click Casting profile contains fields this version cannot preserve"
            end
        end
        local bindingType = safeScalar(binding and binding.type)
        local actionID = safeScalar(binding and binding.actionID)
        local button = safeScalar(binding and binding.button)
        local modifiers = safeScalar(binding and binding.modifiers)
        local inputKey = self:GetClickBindingInputKey(button, modifiers)
        if inputKey and not seenInputs[inputKey]
            and knownTypes[bindingType]
            and type(actionID) == "number" and actionID > 0 and actionID % 1 == 0
        then
            seenInputs[inputKey] = true
            table.insert(result, {
                type = bindingType,
                actionID = actionID,
                button = button,
                modifiers = modifiers,
            })
        else
            return nil, "The current Click Casting profile contains a binding this version cannot preserve"
        end
    end
    return result
end

function Addon:InspectClickBinding(inputKey)
    local button, modifiers = parseClickBindingInputKey(inputKey)
    if not button then return { key = inputKey, actionName = "Invalid click input" } end
    local result = {
        key = inputKey,
        inputKey = inputKey,
        clickButton = button,
        clickModifiers = modifiers,
        inputLabel = self:FormatClickBindingInput(button, modifiers),
        actionName = "Unbound",
    }
    for _, binding in ipairs(self:GetClickBindingProfile() or {}) do
        if binding.button == button and binding.modifiers == modifiers then
            result.clickBindingType = binding.type
            result.actionID = binding.actionID
            if binding.type == clickBindingType("Spell", 1) then
                result.actionType = "spell"
                result.actionName = getSpellName(binding.actionID)
                    or ("Spell " .. tostring(binding.actionID))
            elseif binding.type == clickBindingType("Macro", 2) then
                result.actionType = "macro"
                local macroName, macroIcon, macroBody
                if GetMacroInfo then
                    macroName, macroIcon, macroBody = GetMacroInfo(binding.actionID)
                end
                result.actionName = macroName and ("Macro: " .. macroName)
                    or ("Macro " .. tostring(binding.actionID))
                result.macroName = safeScalar(macroName)
                result.macroBody = safeScalar(macroBody)
                result.icon = safeScalar(macroIcon)
            elseif binding.type == clickBindingType("PetAction", 4) then
                result.actionType = "petaction"
                result.actionName = getSpellName(binding.actionID)
                    or ("Pet action " .. tostring(binding.actionID))
            elseif binding.type == clickBindingType("Interaction", 3) then
                result.actionType = "interaction"
                local target = Enum and Enum.ClickBindingInteraction
                    and Enum.ClickBindingInteraction.Target or 1
                result.actionName = binding.actionID == target and "Target unit" or "Open unit menu"
            end
            break
        end
    end
    return result
end

local function actionBarValue(methodName, legacyFunction, ...)
    local modern = C_ActionBar and C_ActionBar[methodName]
    if modern then return modern(...) end
    if legacyFunction then return legacyFunction(...) end
end

local function actionBarPredicate(methodName, legacyFunction, ...)
    local predicate = C_ActionBar and C_ActionBar[methodName] or legacyFunction
    if type(predicate) ~= "function" then return nil end
    local ok, value = pcall(predicate, ...)
    if not ok or isSecret(value) or type(value) ~= "boolean" then return nil end
    return value
end

local function getFrameForBinding(command)
    local buttonNumber = command and command:match("^ACTIONBUTTON(%d+)$")
    if buttonNumber then
        local frameName = "ActionButton" .. buttonNumber
        return _G[frameName], frameName
    end

    local barNumber, multiButton
    if command then
        barNumber, multiButton = command:match("^MULTIACTIONBAR(%d+)BUTTON(%d+)$")
    end
    if barNumber and multiButton then
        local prefix = bindingFrames[barNumber]
        if prefix then
            local frameName = prefix .. multiButton
            return _G[frameName], frameName
        end
    end

    local clickFrame = command and command:match("^CLICK ([^:]+):")
    if clickFrame then
        return _G[clickFrame], clickFrame
    end

    return nil, nil
end

local function getFixedSlotForBinding(command)
    local buttonNumber = command and command:match("^ACTIONBUTTON(%d+)$")
    if buttonNumber then
        return tonumber(buttonNumber), "main"
    end

    local barNumber, multiButton
    if command then
        barNumber, multiButton = command:match("^MULTIACTIONBAR(%d+)BUTTON(%d+)$")
    end
    local offset = barNumber and fixedMultiBarSlotOffsets[barNumber]
    if offset and multiButton then
        return offset + tonumber(multiButton), "multi"
    end

    return nil, nil
end

function Addon:GetFixedActionSlotForBinding(command)
    return getFixedSlotForBinding(command)
end

function Addon:GetEffectiveActionSlotForBinding(command)
    local fixedSlot, slotKind = getFixedSlotForBinding(command)
    if slotKind == "multi" then return fixedSlot, slotKind end
    if slotKind == "main" then
        local frame = getFrameForBinding(command)
        local effectiveSlot = frame and safeScalar(frame.action)
        return effectiveSlot, slotKind
    end
    return fixedSlot, slotKind
end


local function isOverrideBarActive()
    return actionBarValue("HasOverrideActionBar", HasOverrideActionBar)
        or actionBarValue("HasVehicleActionBar", HasVehicleActionBar)
        or actionBarValue("HasTempShapeshiftActionBar", HasTempShapeshiftActionBar)
        or false
end

local function describeAction(slot)
    if not slot then
        return { actionName = "No action slot" }
    end
    if isSecret(slot) then
        return { protected = true, actionName = "Protected during combat" }
    end

    local actionType, actionID, actionSubType = GetActionInfo(slot)
    if isSecret(actionType) or isSecret(actionID) or isSecret(actionSubType) then
        return { protected = true, actionName = "Protected during combat" }
    end

    local result = {
        actionSlot = safeScalar(slot),
        actionType = safeScalar(actionType),
        actionID = safeScalar(actionID),
        actionSubType = safeScalar(actionSubType),
        icon = safeScalar(actionBarValue("GetActionTexture", GetActionTexture, slot)),
        isInterrupt = actionBarPredicate("IsInterruptAction", IsInterruptAction, slot),
    }

    if not actionType then
        result.actionName = "Empty"
    elseif actionType == "spell" then
        result.actionName = getSpellName(actionID) or ("Spell " .. tostring(actionID or "?"))
    elseif actionType == "macro" then
        local macroName, macroIcon, macroBody = GetMacroInfo(actionID)
        result.actionName = macroName and ("Macro: " .. macroName) or ("Macro " .. tostring(actionID or "?"))
        result.macroName = safeScalar(macroName)
        result.macroBody = safeScalar(macroBody)
        result.icon = safeScalar(macroIcon) or result.icon
    elseif actionType == "item" then
        result.actionName = getItemName(actionID) or ("Item " .. tostring(actionID or "?"))
    elseif actionType == "equipmentset" then
        result.actionName = tostring(actionID or "Equipment set")
    elseif actionType == "flyout" then
        local flyoutName
        if C_SpellBook and C_SpellBook.GetFlyoutInfo then
            local flyoutInfo = C_SpellBook.GetFlyoutInfo(actionID)
            flyoutName = flyoutInfo and flyoutInfo.name
        elseif GetFlyoutInfo then
            flyoutName = GetFlyoutInfo(actionID)
        end
        result.actionName = flyoutName or ("Flyout " .. tostring(actionID or "?"))
    else
        result.actionName = actionType:gsub("^%l", string.upper) .. (actionID and (" " .. tostring(actionID)) or "")
    end

    return result
end

function Addon:DescribeActionSlot(slot)
    return describeAction(slot)
end

function Addon:InspectKey(key)
    local command = GetBindingAction(key) or ""
    local frame, frameName = getFrameForBinding(command)
    local fixedSlot, slotKind = getFixedSlotForBinding(command)
    local effectiveSlot = frame and frame.action
    local slot = effectiveSlot or fixedSlot
    local overrideAction

    -- Vehicle, Skyriding, possess, and temporary override bars replace the
    -- visible main bar. Preserve the override for context, but show page 1 as
    -- the normal character bar while the override is active.
    if slotKind == "main" and effectiveSlot and isOverrideBarActive() then
        overrideAction = describeAction(effectiveSlot)
        slot = fixedSlot
    elseif slotKind == "multi" and fixedSlot then
        -- Multi bars have stable server-side slots even when their frames are
        -- hidden or have not been created.
        slot = fixedSlot
    end

    local directSpellName = not slot and command and command:match("^SPELL (.+)$")
    local directSpellID
    if directSpellName and C_Spell and C_Spell.GetSpellInfo then
        local info = C_Spell.GetSpellInfo(directSpellName)
        directSpellID = type(info) == "table" and safeScalar(info.spellID) or nil
    elseif directSpellName and GetSpellInfo then
        directSpellID = safeScalar(select(7, GetSpellInfo(directSpellName)))
    end
    local result = directSpellName and {
        actionType = "spell",
        actionID = directSpellID,
        actionName = directSpellName,
    } or describeAction(slot)

    result.key = key
    result.bindingCommand = safeScalar(command)
    result.frameName = safeScalar(frameName)
    result.effectiveActionSlot = safeScalar(effectiveSlot)
    result.overrideAction = overrideAction

    if command == "" then
        result.actionName = "Unbound"
    elseif not result.actionSlot and not result.actionType then
        result.actionName = "Binding has no readable action slot"
    end

    return result
end


local function getActiveFormInfo()
    local formIndex = GetShapeshiftForm and GetShapeshiftForm() or 0
    local formName = formIndex == 0 and "Caster" or nil
    local formSpellID

    if formIndex > 0 and GetShapeshiftFormInfo then
        local _, _, _, spellID = GetShapeshiftFormInfo(formIndex)
        formSpellID = safeScalar(spellID)
        formName = getSpellName(formSpellID)
    end

    return safeScalar(formIndex) or 0, formName or ("Form " .. tostring(formIndex)), formSpellID
end

function Addon:GetActionBarState()
    local formIndex, formName, formSpellID = getActiveFormInfo()
    local effectiveFirstSlot = ActionButton1 and safeScalar(ActionButton1.action)
    local effectivePage = effectiveFirstSlot and math.floor((effectiveFirstSlot - 1) / 12) + 1 or nil
    local overrideActive = isOverrideBarActive() and true or false
    local stateName = overrideActive and "Temporary override" or formName
    local stateID
    if overrideActive then
        stateID = "temporary-override"
    elseif formSpellID then
        stateID = "form:" .. tostring(formSpellID)
    elseif formIndex == 0 then
        stateID = "caster"
    else
        stateID = "form-index:" .. tostring(formIndex)
    end
    local pageSuffix = "-page" .. tostring(effectivePage or "unknown")

    return {
        overrideActive = overrideActive,
        actionBarPage = safeScalar(actionBarValue("GetActionBarPage", GetActionBarPage)),
        bonusBarOffset = safeScalar(actionBarValue("GetBonusBarOffset", GetBonusBarOffset)),
        formIndex = formIndex,
        formName = formName,
        formSpellID = formSpellID,
        effectiveFirstSlot = effectiveFirstSlot,
        effectivePage = effectivePage,
        -- stateID is locale-independent and is used for learned/apply context.
        -- stateKey remains the readable legacy key for migration and display.
        stateID = stateID .. pageSuffix,
        stateKey = stateName .. pageSuffix,
    }
end

function Addon:GetAllActionSlots()
    local result = {}
    local maximumSlot = MAX_ACTION_BUTTONS or 180
    for slot = 1, maximumSlot do
        local action = describeAction(slot)
        if action.actionType then
            result[slot] = action
        end
    end
    return result
end

function Addon:GetBindingAudit()
    local desiredBindings = {}
    if self.GetManagedDestinations then
        for _, destination in ipairs(self:GetManagedDestinations()) do
            if destination.bindingCommand and not destination.bindingCommand:match("^SPELL ") then
                table.insert(desiredBindings, {
                    key = destination.inputKey,
                    command = destination.bindingCommand,
                    destination = destination.id,
                })
            end
        end
    else
        desiredBindings = self.profile.bindings or {}
    end
    local audit = {
        matches = 0,
        total = #desiredBindings,
        mismatches = {},
        scopeID = GetCurrentBindingSet and GetCurrentBindingSet() or 1,
    }
    audit.scope = audit.scopeID == 2 and "Character-specific" or "Account-wide"

    for _, desired in ipairs(desiredBindings) do
        local current = GetBindingAction(desired.key) or ""
        if current == desired.command then
            audit.matches = audit.matches + 1
        else
            table.insert(audit.mismatches, {
                key = desired.key,
                destination = desired.destination,
                current = current,
                desired = desired.command,
            })
        end
    end

    audit.matchesProfile = audit.matches == audit.total
    return audit
end
