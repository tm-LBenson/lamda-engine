local _, ns = ...
local C = ns.Compat

local function call(fn, ...)
    if type(fn) ~= "function" then return end
    local ok, a, b, c = pcall(fn, ...)
    if ok then return a, b, c end
end

local function difficultyColor(fn, ...)
    local color = call(fn, ...)
    if type(color) ~= "table" then return end
    for _, key in ipairs({ "r", "g", "b" }) do
        local value = color[key]
        if issecretvalue and issecretvalue(value) then return end
        if type(value) ~= "number" or value ~= value or value < 0 or value > 1 then return end
    end
    return color
end

function C.QuestDifficultyColor(entry)
    if not entry or entry.custom or entry.recipe or not entry.questID then return end
    -- Retail's quest log uses content difficulty, including scaled quests.
    local difficulty = C_PlayerInfo and call(C_PlayerInfo.GetContentDifficultyQuestForPlayer, entry.questID)
    if issecretvalue and issecretvalue(difficulty) then return end
    if difficulty ~= nil then
        local color = difficultyColor(DifficultyUtil and DifficultyUtil.GetDifficultyColor or GetDifficultyColor, difficulty)
        if color then return color end
    end
    -- Forever and older UIs expose the native level-based palette instead.
    local level = entry.difficultyLevel or entry.level
    if type(level) == "number" and level > 0 then
        return difficultyColor(DifficultyUtil and DifficultyUtil.GetQuestDifficultyColor or GetQuestDifficultyColor,
            level, entry.isScaling, entry.questID)
    end
end

function C.HasQuestDistance()
    return (C_QuestLog and type(C_QuestLog.GetDistanceSqToQuest) == "function") or type(GetQuestLogDistanceSq) == "function"
end

function C.QuestDistance(questID, logIndex)
    local distance, onContinent
    if C_QuestLog and C_QuestLog.GetDistanceSqToQuest then
        distance, onContinent = call(C_QuestLog.GetDistanceSqToQuest, questID)
    elseif GetQuestLogDistanceSq and logIndex then
        distance, onContinent = call(GetQuestLogDistanceSq, logIndex)
    end
    if issecretvalue and (issecretvalue(distance) or issecretvalue(onContinent)) then return end
    if onContinent == false or onContinent == 0 then return end
    if type(distance) == "number" and distance == distance and distance >= 0 and distance < math.huge then return distance end
end

function C.RefreshDistances(quests)
    local changed = false
    for _, entry in ipairs(quests) do
        local distance = C.QuestDistance(entry.questID, entry.logIndex)
        if distance ~= entry.distanceSq then entry.distanceSq = distance; changed = true end
    end
    return changed
end

function C.Collect()
    local quests, zone = {}, "Other quests"
    local modern = C_QuestLog and C_QuestLog.GetInfo and C_QuestLog.GetNumQuestLogEntries
    local count = call(modern and C_QuestLog.GetNumQuestLogEntries or GetNumQuestLogEntries) or 0
    for index = 1, count do
        local info
        if modern then
            info = call(C_QuestLog.GetInfo, index)
        elseif GetQuestLogTitle then
            local title, level, _, isHeader, _, complete, _, questID = GetQuestLogTitle(index)
            info = { title = title, level = level, isHeader = isHeader, questID = questID, isComplete = complete }
        end
        if info and info.isHeader then
            zone = info.title or zone
        elseif info and info.title and info.questID and info.questID > 0 and not info.isHidden then
            local watched = true
            if C_QuestLog and C_QuestLog.GetQuestWatchType then
                watched = call(C_QuestLog.GetQuestWatchType, info.questID) ~= nil
            elseif IsQuestWatched then
                local value = call(IsQuestWatched, index)
                watched = value == true or value == 1
            end
            local completed = info.isComplete == true or info.isComplete == 1
            if C_QuestLog and C_QuestLog.IsComplete then
                local status = call(C_QuestLog.IsComplete, info.questID)
                if status ~= nil then completed = status == true or status == 1 end
            end
            local objectives = C_QuestLog and call(C_QuestLog.GetQuestObjectives, info.questID)
            if not objectives then
                objectives = {}
                for objectiveIndex = 1, call(GetNumQuestLeaderBoards, index) or 0 do
                    local text, kind, finished = call(GetQuestLogLeaderBoard, objectiveIndex, index)
                    objectives[#objectives + 1] = { text = text or "", type = kind, finished = finished == true or finished == 1 }
                end
            end
            local itemLink, itemTexture, charges = call(GetQuestLogSpecialItemInfo, index)
            local tag = C_QuestLog and call(C_QuestLog.GetQuestTagInfo, info.questID)
            quests[#quests + 1] = {
                questID = info.questID, title = info.title, level = info.level or info.difficultyLevel or 0,
                difficultyLevel = info.difficultyLevel or info.level, isScaling = info.isScaling == true,
                zone = zone, watched = watched, completed = completed,
                failed = info.isComplete == -1, campaign = (info.campaignID or 0) > 0,
                objectives = objectives, logIndex = index, itemLink = itemLink, itemTexture = itemTexture, charges = charges,
                distanceSq = C.QuestDistance(info.questID, index),
                questType = tag and tag.tagID and ("tag:" .. tag.tagID), questTypeLabel = tag and tag.tagName,
            }
        end
    end
    return quests
end

function C.OpenQuestDetails(entry)
    if not entry or entry.custom or entry.recipe or not entry.questID then return end
    local questID = entry.questID
    local popup = QuestLogPopupDetailFrame
    -- Blizzard's Details action opens the standalone popup, independently of
    -- the map's quest panel (which can be hidden on Forever).
    if popup and call(popup.IsShowingQuest, popup, questID) then return true end
    if QuestUtil and type(QuestUtil.OpenQuestDetails) == "function" then
        local ok = pcall(QuestUtil.OpenQuestDetails, questID)
        if ok then return true end
    end
    if popup and type(popup.ShowQuest) == "function" then
        local ok = pcall(popup.ShowQuest, popup, questID)
        if ok then return true end
    end
    if type(QuestMapFrame_OpenToQuestDetails) == "function" then
        local ok = pcall(QuestMapFrame_OpenToQuestDetails, questID)
        if ok then return true end
    end
    if QuestLogFrame and ShowUIPanel then
        local index = C_QuestLog and call(C_QuestLog.GetLogIndexForQuestID, questID) or entry.logIndex
        if index and index > 0 then
            call(SelectQuestLogEntry, index)
            call(QuestLog_SetSelection, index)
            local ok = pcall(ShowUIPanel, QuestLogFrame)
            if ok then return true end
        end
    end
    ns.Print("Quest details could not be opened on this client.")
end

function C.OpenQuest(entry)
    if IsModifiedClick and IsModifiedClick("CHATLINK") then
        local link = call(GetQuestLink, entry.logIndex)
        if link and ChatEdit_InsertLink and ChatEdit_InsertLink(link) then return end
    end
    if IsShiftKeyDown and IsShiftKeyDown() then return C.Watch(entry, not entry.watched) end
    if entry.completed and C_QuestLog and call(C_QuestLog.IsAutoComplete, entry.questID) and ShowQuestComplete then
        ShowQuestComplete(entry.questID)
    else
        return C.OpenQuestDetails(entry)
    end
end

function C.ItemCooldown(index)
    return call(GetQuestLogSpecialItemCooldown, index)
end

function C.IsFocusedQuest(questID)
    return C_SuperTrack and call(C_SuperTrack.GetSuperTrackedQuestID) == questID or false
end

function C.FocusQuest(entry)
    if not entry or not entry.questID then return end
    if IsModifiedClick and IsModifiedClick("CHATLINK") then
        local link = call(GetQuestLink, entry.logIndex)
        if link and ChatEdit_InsertLink and ChatEdit_InsertLink(link) then return end
    end
    if IsShiftKeyDown and IsShiftKeyDown() then C.Watch(entry, false); return end
    if not C_SuperTrack or not C_SuperTrack.SetSuperTrackedQuestID then
        ns.Print("Quest navigation is unavailable on this client."); return
    end
    if C.IsFocusedQuest(entry.questID) and C_SuperTrack.ClearAllSuperTracked then
        C_SuperTrack.ClearAllSuperTracked()
    else
        if not entry.watched then C.Watch(entry, true) end
        C_SuperTrack.SetSuperTrackedQuestID(entry.questID)
    end
    ns.RequestRefresh()
end

local highlightedQuestID, highlightedOwner, highlightedMap
function C.HighlightQuest(questID, owner)
    -- Map areas and POI icons have separate highlight channels in Blizzard's UI.
    if WorldMapFrame and WorldMapFrame.SetHighlightedQuestID then
        WorldMapFrame:SetHighlightedQuestID(questID)
        highlightedMap = WorldMapFrame
    end
    local manager = POIButtonHighlightManager
    if manager and manager.SetHighlight then
        manager:SetHighlight(questID)
    elseif EventRegistry and EventRegistry.TriggerEvent then
        EventRegistry:TriggerEvent("SetHighlightedQuestPOI", questID)
    end
    highlightedQuestID = questID
    highlightedOwner = owner
end

function C.ClearQuestHighlight(questID, owner)
    if highlightedQuestID ~= questID or (owner and highlightedOwner ~= owner) then return end
    local manager = POIButtonHighlightManager
    local stillOwned = not manager or not manager.GetQuestID or manager:GetQuestID() == questID
    if stillOwned and highlightedMap and highlightedMap.ClearHighlightedQuestID then
        highlightedMap:ClearHighlightedQuestID()
    end
    if manager and manager.ClearHighlight then
        if not manager.GetQuestID or manager:GetQuestID() == questID then manager:ClearHighlight() end
    elseif EventRegistry and EventRegistry.TriggerEvent then
        EventRegistry:TriggerEvent("ClearHighlightedQuestPOI", questID)
    end
    highlightedQuestID = nil
    highlightedOwner, highlightedMap = nil, nil
end

function C.Watch(entry, watch)
    if C_QuestLog then
        if watch and C_QuestLog.AddQuestWatch then
            C_QuestLog.AddQuestWatch(entry.questID, Enum and Enum.QuestWatchType and Enum.QuestWatchType.Manual or 1)
        elseif not watch and C_QuestLog.RemoveQuestWatch then
            C_QuestLog.RemoveQuestWatch(entry.questID)
        end
    elseif watch and AddQuestWatch then AddQuestWatch(entry.logIndex)
    elseif not watch and RemoveQuestWatch then RemoveQuestWatch(entry.logIndex) end
    ns.RequestRefresh()
end

-- Replace quests and supported recipe tracking through Blizzard's container API.
-- Scenario, achievement, dungeon and world-quest modules remain with Blizzard.
-- Do not replace native methods or unregister their events.
local detached, hooked = {}, {}
function C.ReplaceNative(replace, replaceRecipes)
    if ns.InCombat() then return false end
    local container = ObjectiveTrackerFrame
    if container and container.HasModule and container.RemoveModule and container.AddModule then
        for _, pair in ipairs({ { QuestObjectiveTracker, replace }, { CampaignQuestObjectiveTracker, replace },
            { ProfessionsRecipeTracker, replaceRecipes == true } }) do
            local module, replaceModule = pair[1], pair[2]
            if module then
                if replaceModule and container:HasModule(module) then
                    detached[module] = detached[module] or { container = container, shown = module:IsShown() }
                    container:RemoveModule(module)
                    module:Hide()
                elseif not replaceModule and detached[module] then
                    local state = detached[module]
                    state.container:AddModule(module)
                    detached[module] = nil
                    if state.shown then module:Show() end
                end
            end
        end
        return QuestObjectiveTracker ~= nil or CampaignQuestObjectiveTracker ~= nil
    end
    local legacy = QuestWatchFrame or WatchFrame
    if legacy then
        if replace and not detached[legacy] then
            detached[legacy] = { shown = legacy:IsShown() }
            if not hooked[legacy] then
                hooked[legacy] = true
                legacy:HookScript("OnShow", function(self)
                    if detached[self] and not ns.InCombat() then self:Hide() end
                end)
            end
            legacy:Hide()
        elseif not replace and detached[legacy] then
            local state = detached[legacy]
            detached[legacy] = nil
            if state.shown then legacy:Show() end
        end
        return true
    end
    -- Unknown UI: leave it intact and allow the addon tracker to be previewed.
    return false
end

function C.CopyNativeStyle(text, kind)
    local font = kind == "header" and ObjectiveTrackerHeaderFont or ObjectiveTrackerLineFont
    if font then text:SetFontObject(font) end
    if ns.db and text.GetFont then
        local face, _, flags = text:GetFont()
        if face then text:SetFont(face, ns.db.settings.fontSize + (kind == "header" and 2 or 0), flags) end
    end
    local palette = OBJECTIVE_TRACKER_COLOR
    local color
    if kind == "header" then color = NORMAL_FONT_COLOR or (palette and palette.Header)
    else color = palette and palette[kind == "title" and "Header" or "Normal"] end
    if color then text:SetTextColor(color.r, color.g, color.b) end
end
