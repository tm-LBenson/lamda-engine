local _, ns = ...
local R = ns.Recipes

local function call(fn, ...)
    if type(fn) ~= "function" then return end
    local ok, a, b = pcall(fn, ...)
    if ok then return a, b end
end

function R.Key(recipeID, recraft)
    return "recipe:" .. recipeID .. (recraft and ":recraft" or ":craft")
end

local function reagentName(reagent)
    if reagent.itemID then
        local name = call(C_Item and C_Item.GetItemInfo or GetItemInfo, reagent.itemID)
        if name then return name end
        call(C_Item and C_Item.RequestLoadItemDataByID, reagent.itemID)
        return "Item #" .. reagent.itemID
    elseif reagent.currencyID then
        local info = call(C_CurrencyInfo and C_CurrencyInfo.GetCurrencyInfo, reagent.currencyID)
        return info and info.name or ("Currency #" .. reagent.currencyID)
    end
end

local function quantityOwned(reagents)
    local native = call(ProfessionsUtil and ProfessionsUtil.AccumulateReagentsInPossession, reagents)
    if type(native) == "number" then return native end
    local count = 0
    for _, reagent in ipairs(reagents) do
        if reagent.itemID then
            local owned = call(C_Item and C_Item.GetItemCount or GetItemCount, reagent.itemID, true, false, true)
            if type(owned) ~= "number" then return end
            count = count + owned
        elseif reagent.currencyID then
            local info = call(C_CurrencyInfo and C_CurrencyInfo.GetCurrencyInfo, reagent.currencyID)
            if not info or type(info.quantity) ~= "number" then return end
            count = count + info.quantity
        end
    end
    return count
end

local function objectivesFor(schematic)
    local objectives, ready = {}, true
    if not schematic or not schematic.reagentSlotSchematics then
        return { { text = "Loading recipe reagents...", finished = false } }, false
    end
    for _, slot in ipairs(schematic.reagentSlotSchematics) do
        local required = call(ProfessionsUtil and ProfessionsUtil.IsReagentSlotRequired, slot)
        if required == nil then required = slot.required == true end
        if required then
            local reagents = slot.reagents or {}
            local reagent = reagents[1]
            if reagent then
                local basic = call(ProfessionsUtil and ProfessionsUtil.IsReagentSlotBasicRequired, slot)
                if basic == nil then basic = not slot.slotInfo or (Enum and Enum.CraftingReagentType and slot.reagentType == Enum.CraftingReagentType.Basic) end
                local name = (not basic and slot.slotInfo and slot.slotInfo.slotText) or reagentName(reagent) or "Reagent"
                local amount = call(slot.GetQuantityRequired, slot, reagent) or slot.quantityRequired
                local owned = quantityOwned(reagents)
                local variable = call(slot.IsVariableQuantityReagent, slot, reagent)
                local min, max = call(slot.GetVariableQuantityRange, slot, reagent)
                if variable == nil and slot.variableQuantities and #slot.variableQuantities > 0 then
                    variable = true
                    for _, choice in ipairs(slot.variableQuantities) do
                        if type(choice.quantity) == "number" then
                            min = min and math.min(min, choice.quantity) or choice.quantity
                            max = max and math.max(max, choice.quantity) or choice.quantity
                        end
                    end
                end
                local text, finished
                if type(owned) ~= "number" then
                    text, finished = name .. " (owned quantity unavailable)", false
                elseif variable and min and max then
                    text, finished = string.format("%d / %d-%d %s", owned, min, max, name), false
                elseif variable then
                    text, finished = string.format("%d %s (variable quantity required)", owned, name), false
                elseif type(amount) == "number" and amount > 0 then
                    text, finished = string.format("%d / %d %s", owned, amount, name), owned >= amount
                else
                    text, finished = name .. " (required quantity unavailable)", false
                end
                objectives[#objectives + 1] = { text = text, finished = finished }
                if not finished then ready = false end
            else
                objectives[#objectives + 1] = { text = "Loading required reagent...", finished = false }
                ready = false
            end
        end
    end
    return objectives, ready
end

function R.Collect()
    local api = C_TradeSkillUI
    if not api or not api.GetRecipesTracked or (not api.GetRecipeSchematic and not (ProfessionsUtil and ProfessionsUtil.GetRecipeSchematic)) then return {}, false end
    local regular = call(api.GetRecipesTracked, false)
    if type(regular) ~= "table" then return {}, false end
    local recrafts = call(api.GetRecipesTracked, true)
    if type(recrafts) ~= "table" then return {}, false end
    local entries = {}
    local function add(recipes, recraft)
        for _, recipeID in ipairs(recipes) do
            local schematic = call(ProfessionsUtil and ProfessionsUtil.GetRecipeSchematic, recipeID, recraft)
                or call(api.GetRecipeSchematic, recipeID, recraft)
            local info = call(api.GetRecipeInfo, recipeID)
            local name = schematic and schematic.name or info and info.name or ("Recipe #" .. recipeID)
            if recraft then name = (PROFESSIONS_CRAFTING_FORM_RECRAFTING_HEADER or "Recraft: %s"):format(name) end
            local objectives, ready = objectivesFor(schematic)
            entries[#entries + 1] = { id = R.Key(recipeID, recraft), recipe = true, recipeID = recipeID,
                recraft = recraft, title = name, zone = "Professions", level = 0, watched = true,
                completed = false, ready = ready, objectives = objectives,
                icon = schematic and schematic.icon or info and info.icon or "Interface\\Icons\\INV_Misc_Note_01" }
        end
    end
    add(regular, false); add(recrafts, true)
    return entries, true
end

function R.Untrack(entry)
    if C_TradeSkillUI and C_TradeSkillUI.SetRecipeTracked then
        local ok, err = pcall(C_TradeSkillUI.SetRecipeTracked, entry.recipeID, false, entry.recraft == true)
        if not ok then ns.Print("Unable to stop recipe tracking: " .. tostring(err)); return end
        ns.RequestRefresh()
    end
end

function R.Open(entry)
    if IsShiftKeyDown and IsShiftKeyDown() then R.Untrack(entry); return end
    if ProfessionsFrame_LoadUI and not ProfessionsFrame then call(ProfessionsFrame_LoadUI) end
    local api = C_TradeSkillUI
    if not api then return end
    local learned = call(api.IsRecipeProfessionLearned, entry.recipeID)
    if learned == false and Professions and Professions.InspectRecipe then
        Professions.InspectRecipe(entry.recipeID)
    elseif api.OpenRecipe then
        local ok, err = pcall(api.OpenRecipe, entry.recipeID)
        if not ok then ns.Print("Unable to open recipe: " .. tostring(err)) end
    else ns.Print("Open your profession window to view this recipe.") end
end
