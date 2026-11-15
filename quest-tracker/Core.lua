local _, ns = ...
local events = CreateFrame("Frame")
local scheduled, warnedNative

function ns.Refresh()
    scheduled = false
    if not ns.db then return end
    local settings = ns.db.settings
    ns.quests = ns.Compat.Collect()
    ns.recipes, ns.recipesAvailable = ns.Recipes.Collect()
    ns.entries = {}
    for _, entry in ipairs(ns.quests) do ns.entries[#ns.entries + 1] = entry end
    for _, entry in ipairs(ns.recipes) do ns.entries[#ns.entries + 1] = entry end
    if ns.InCombat() then
        ns.dirty = true
        ns.UI.UpdateQuestFocus(ns.quests)
        ns.UI.ApplyLiveAppearance(); ns.UI.UpdateQuickControls(); ns.UI.RefreshManager(); ns.UI.RefreshFilters()
        return
    end
    ns.dirty = false
    ns.UI.Render(ns.entries)
    ns.UI.RefreshManager()
    ns.UI.RefreshFilters()
    local replace = settings.enabled and settings.replaceNative
    local nativeReady = ns.Compat.ReplaceNative(replace, replace and settings.showRecipes and ns.recipesAvailable)
    if settings.enabled and settings.replaceNative and not nativeReady and not warnedNative then
        warnedNative = true
        ns.Print("Native tracker is not available yet. Your existing tracker remains available.")
    end
end

function ns.RequestRefresh()
    if not ns.db then return end
    if ns.InCombat() then
        ns.Refresh(); return
    end
    if scheduled then return end
    scheduled = true
    if C_Timer and C_Timer.After then C_Timer.After(0.1, ns.Refresh) else ns.Refresh() end
end

function ns.RefreshDistances()
    if not ns.db or not ns.db.settings.enabled or ns.InCombat() or ns.UI.moving then return false end
    local _, sort = ns.Model.View(ns.db.settings)
    if sort ~= "distance" then return false end
    local changed = ns.Compat.RefreshDistances(ns.quests or {})
    if changed then ns.UI.Render(ns.entries or ns.quests or {}) end
    return changed
end

local distanceElapsed = 0
events:SetScript("OnUpdate", function(_, elapsed)
    if not ns.db or not ns.db.settings.enabled then distanceElapsed = 0; return end
    local _, sort = ns.Model.View(ns.db.settings)
    if sort ~= "distance" or ns.InCombat() then distanceElapsed = 0; return end
    distanceElapsed = distanceElapsed + elapsed
    if distanceElapsed >= 1 then distanceElapsed = 0; ns.RefreshDistances() end
end)

function ns.SetEnabled(enabled)
    if not ns.db then return end
    ns.db.settings.enabled = enabled
    ns.Refresh()
end

function LamdaQuestLog_Toggle()
    if ns.db then ns.SetEnabled(not ns.db.settings.enabled) end
end

BINDING_HEADER_LAMDAQUESTLOG = "Lamda Quest Log"
BINDING_NAME_LAMDAQUESTLOG_TOGGLE = "Toggle quest tracker"
SLASH_LAMDAQUESTLOG1 = "/lql"
SLASH_LAMDAQUESTLOG2 = "/lamdaquestlog"
SlashCmdList.LAMDAQUESTLOG = function(message)
    if not ns.db then return end
    local command = ns.Model.Trim(message):lower()
    if command == "add" then ns.UI.EditEntry()
    elseif command == "options" or command == "settings" then ns.UI.Options()
    elseif command == "restore" then
        ns.SetEnabled(false)
        ns.Print(ns.InCombat() and "Blizzard quest tracking will return after combat." or "Blizzard quest tracking restored. /lql show enables Lamda again.")
    elseif command == "show" then ns.SetEnabled(true)
    elseif command == "hide" then ns.SetEnabled(false)
    elseif command == "unlock" or command == "lock" then
        ns.UI.SetLocked(command == "lock")
    elseif command == "resetposition" then
        ns.db.settings.position = nil
        ns.db.settings.width, ns.db.settings.height = 300, 480
        ns.RequestRefresh()
    elseif command == "" then ns.UI.Options()
    else ns.Print("/lql [add | options | show | hide | restore | lock | unlock | resetposition]") end
end

events:RegisterEvent("ADDON_LOADED")
events:RegisterEvent("PLAYER_LOGIN")
events:RegisterEvent("PLAYER_ENTERING_WORLD")
events:RegisterEvent("PLAYER_LEVEL_UP")
events:RegisterEvent("UNIT_LEVEL")
pcall(events.RegisterEvent, events, "UNIT_EFFECTIVE_LEVEL")
events:RegisterEvent("QUEST_LOG_UPDATE")
events:RegisterEvent("QUEST_AUTOCOMPLETE")
events:RegisterEvent("QUEST_WATCH_UPDATE")
events:RegisterEvent("QUEST_WATCH_LIST_CHANGED")
events:RegisterEvent("QUEST_POI_UPDATE")
events:RegisterEvent("SUPER_TRACKING_CHANGED")
events:RegisterEvent("ZONE_CHANGED_NEW_AREA")
events:RegisterEvent("PLAYER_REGEN_ENABLED")
events:RegisterEvent("PLAYER_REGEN_DISABLED")
events:RegisterEvent("UPDATE_MACROS")
events:RegisterEvent("BAG_UPDATE_DELAYED")
events:RegisterEvent("BAG_UPDATE_COOLDOWN")
for _, event in ipairs({ "TRACKED_RECIPE_UPDATE", "TRADE_SKILL_LIST_UPDATE", "TRADE_SKILL_DATA_SOURCE_CHANGED",
    "CURRENCY_DISPLAY_UPDATE", "GET_ITEM_INFO_RECEIVED" }) do
    pcall(events.RegisterEvent, events, event)
end
events:SetScript("OnEvent", function(_, event, name)
    if (event == "UNIT_LEVEL" or event == "UNIT_EFFECTIVE_LEVEL") and name ~= "player" then return end
    if event == "BAG_UPDATE_COOLDOWN" then ns.UI.UpdateCooldowns(); return end
    if event == "ADDON_LOADED" and name == ns.name then
        local db, err = ns.Model.Initialize(LamdaQuestLogDB)
        if not db then ns.Print(err); return end
        LamdaQuestLogDB, ns.db = db, db
    elseif event == "PLAYER_REGEN_DISABLED" then
        ns.UI.StopMotion()
        ns.Refresh()
    end
    if event == "PLAYER_REGEN_DISABLED" then return end
    ns.RequestRefresh()
end)
