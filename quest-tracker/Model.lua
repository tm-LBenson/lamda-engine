local _, ns = ...
local M = ns.Model

M.groups = { "zone", "category", "type", "none" }
M.sorts = { "log", "title", "title_desc", "level_asc", "level", "progress", "campaign", "completion", "distance", "manual" }
M.quickSorts = { "distance", "level_asc", "level", "title", "title_desc", "progress", "campaign", "log" }
M.scopes = { "watched", "all" }
local defaults = {
    enabled = true, replaceNative = true, group = "zone", sort = "log",
    scope = "watched", showCompleted = true, locked = true,
    width = 300, height = 480, collapsed = false,
    backgroundOpacity = 0, fontSize = 12,
    showRecipes = true,
    quickSort = "",
    quickTurnIn = false,
}

function M.Trim(value)
    return tostring(value or ""):match("^%s*(.-)%s*$")
end

local function contains(values, value)
    for _, item in ipairs(values) do if item == value then return true end end
end

function M.Initialize(db)
    db = type(db) == "table" and db or {}
    if (tonumber(db.schema) or 0) > 1 then return nil, "Saved data is from a newer addon version." end
    db.schema = 1
    for _, key in ipairs({ "settings", "categories", "custom", "questMeta", "recipeMeta", "collapsedGroups", "filters" }) do
        if type(db[key]) ~= "table" then db[key] = {} end
    end
    for _, dimension in ipairs({ "types", "zones", "categories" }) do
        if type(db.filters[dimension]) ~= "table" then db.filters[dimension] = {} end
    end
    if db.settings.quickSort == nil and db.settings.quickDistance == true then db.settings.quickSort = "distance" end
    db.settings.quickDistance = nil
    for key, value in pairs(defaults) do
        if type(db.settings[key]) ~= type(value) then db.settings[key] = value end
    end
    if not contains(M.groups, db.settings.group) then db.settings.group = defaults.group end
    if not contains(M.sorts, db.settings.sort) then db.settings.sort = defaults.sort end
    if not contains(M.quickSorts, db.settings.quickSort) then db.settings.quickSort = "" end
    if not contains(M.scopes, db.settings.scope) then db.settings.scope = defaults.scope end
    db.settings.width = math.max(240, math.min(600, db.settings.width))
    db.settings.height = math.max(180, math.min(1000, db.settings.height))
    db.settings.backgroundOpacity = math.max(0, math.min(100, math.floor(db.settings.backgroundOpacity + 0.5)))
    db.settings.fontSize = math.max(12, math.min(20, math.floor(db.settings.fontSize + 0.5)))
    db.nextID = math.max(1, math.floor(tonumber(db.nextID) or 1))
    return db
end

function M.CategoryName(db, id)
    for _, category in ipairs(db.categories) do
        if category.id == id then return category.name end
    end
    return "Uncategorized"
end

function M.SaveCategory(db, id, name)
    name = M.Trim(name)
    if name == "" then return nil, "Enter a category name." end
    for _, category in ipairs(db.categories) do
        if category.id ~= id and category.name:lower() == name:lower() then
            return nil, "A category with that name already exists."
        end
    end
    if id then
        for _, category in ipairs(db.categories) do
            if category.id == id then category.name = name; return id end
        end
        return nil, "Category no longer exists."
    end
    id = "category:" .. db.nextID
    db.nextID = db.nextID + 1
    db.categories[#db.categories + 1] = { id = id, name = name }
    return id
end

function M.DeleteCategory(db, id)
    for index, category in ipairs(db.categories) do
        if category.id == id then table.remove(db.categories, index); break end
    end
    for _, entry in pairs(db.custom) do if entry.category == id then entry.category = nil end end
    for _, meta in pairs(db.questMeta) do if meta.category == id then meta.category = nil end end
    for _, meta in pairs(db.recipeMeta) do if meta.category == id then meta.category = nil end end
    db.collapsedGroups["category:" .. tostring(id)] = nil
end

function M.SaveCustom(db, id, values)
    local title = M.Trim(values.title)
    if title == "" then return nil, "Enter a title." end
    if id and not db.custom[id] then return nil, "Entry no longer exists." end
    if not id then
        id = "custom:" .. db.nextID
        db.nextID = db.nextID + 1
    end
    local previous = db.custom[id] or {}
    db.custom[id] = {
        id = id, title = title, notes = M.Trim(values.notes), zone = M.Trim(values.zone),
        command = M.Trim(values.command), macro = M.Trim(values.macro),
        category = values.category, completed = values.completed == true,
        order = tonumber(values.order) or previous.order or db.nextID,
    }
    return id
end

function M.Meta(db, entry)
    if entry.custom then return db.custom[entry.id] end
    if entry.recipe then
        db.recipeMeta[entry.id] = db.recipeMeta[entry.id] or {}
        return db.recipeMeta[entry.id]
    end
    local key = tostring(entry.questID)
    db.questMeta[key] = db.questMeta[key] or {}
    return db.questMeta[key]
end

function M.View(settings)
    if contains(M.quickSorts, settings.quickSort) then return "none", settings.quickSort, settings.quickTurnIn == true end
    if settings.quickTurnIn then return "none", "log", true end
    return settings.group, settings.sort, false
end

function M.QuickSorting(settings)
    return contains(M.quickSorts, settings.quickSort) == true or settings.quickTurnIn == true
end

function M.ReadyForTurnIn(entry)
    return entry.questID ~= nil and not entry.custom and not entry.recipe
        and entry.completed == true and not entry.failed
end

function M.Kind(entry)
    if entry.custom then return "custom", "Personal tasks" end
    if entry.recipe then return "recipe", "Recipes" end
    if entry.questType and entry.questTypeLabel then return entry.questType, entry.questTypeLabel end
    if entry.campaign then return "campaign", "Campaign" end
    return "quest", "Quests"
end

local function zoneName(entry)
    local zone = M.Trim(entry.zone)
    return zone ~= "" and zone or "Other quests"
end

function M.PassesFilters(db, entry)
    return not db.filters.types[M.Kind(entry)] and not db.filters.zones[zoneName(entry)]
        and not db.filters.categories[entry.category or "none"]
end

function M.FilterCount(db)
    local count = 0
    for _, dimension in ipairs({ "types", "zones", "categories" }) do
        for _, excluded in pairs(db.filters[dimension]) do if excluded then count = count + 1 end end
    end
    return count
end

function M.FilterChoices(db, inputs)
    local choices, seen = { types = {}, zones = {}, categories = {} }, { types = {}, zones = {}, categories = {} }
    local function add(dimension, key, label)
        if seen[dimension][key] then return end
        seen[dimension][key] = true
        choices[dimension][#choices[dimension] + 1] = { key = key, label = label }
    end
    local function entryChoices(entry)
        local kind, label = M.Kind(entry)
        add("types", kind, label)
        local zone = zoneName(entry)
        add("zones", zone, zone)
        local category = entry.category
        if not entry.custom then
            local meta = entry.recipe and db.recipeMeta[entry.id] or db.questMeta[tostring(entry.questID)]
            category = meta and meta.category
        end
        add("categories", category or "none", M.CategoryName(db, category))
    end
    for _, entry in ipairs(inputs or {}) do entryChoices(entry) end
    for _, custom in pairs(db.custom) do
        entryChoices({ custom = true, zone = custom.zone, category = custom.category })
    end
    for _, category in ipairs(db.categories) do add("categories", category.id, category.name) end
    for _, values in pairs(choices) do
        table.sort(values, function(a, b)
            if a.label:lower() == b.label:lower() then return a.key < b.key end
            return a.label:lower() < b.label:lower()
        end)
    end
    return choices
end

local function distanceRank(entry)
    local value = entry.distanceSq
    if type(value) == "number" and value == value and value >= 0 and value < math.huge then return value end
    return math.huge
end

local function finiteNumber(value)
    if issecretvalue and issecretvalue(value) then return false end
    return type(value) == "number" and value == value and value > -math.huge and value < math.huge
end

local function questLevel(entry)
    if entry.questID and not entry.custom and not entry.recipe and finiteNumber(entry.level) and entry.level > 0 then
        return entry.level
    end
end

function M.QuestProgress(entry)
    if not entry.questID or entry.custom or entry.recipe or entry.failed then return end
    if entry.completed then return 1 end
    local progress, count = 0, 0
    for _, objective in ipairs(entry.objectives or {}) do
        local value
        if objective.finished == true then value = 1
        elseif finiteNumber(objective.numFulfilled) and finiteNumber(objective.numRequired) and objective.numRequired > 0 then
            value = math.max(0, math.min(1, objective.numFulfilled / objective.numRequired))
        elseif objective.finished == false then value = 0 end
        if value then progress, count = progress + value, count + 1 end
    end
    if count > 0 then return progress / count end
end

function M.Build(db, quests, query)
    local entries, groups, groupByKey = {}, {}, {}
    local groupMode, sortMode, turnInFirst = M.View(db.settings)
    query = M.Trim(query):lower()
    for index, quest in ipairs(quests) do
        if (not quest.recipe or db.settings.showRecipes) and (db.settings.scope == "all" or quest.watched) then
            local entry = {}
            for key, value in pairs(quest) do entry[key] = value end
            entry.id = quest.recipe and quest.id or ("quest:" .. quest.questID)
            entry.logOrder = index
            local meta = quest.recipe and (db.recipeMeta[entry.id] or {}) or (db.questMeta[tostring(quest.questID)] or {})
            entry.category, entry.order = meta.category, meta.order or index
            entries[#entries + 1] = entry
        end
    end
    for _, custom in pairs(db.custom) do
        local entry = {}
        for key, value in pairs(custom) do entry[key] = value end
        entry.custom, entry.level = true, 0
        entry.logOrder = 100000 + (tonumber(entry.order) or 0)
        entries[#entries + 1] = entry
    end
    local function less(a, b)
        local sort = sortMode
        if turnInFirst and M.ReadyForTurnIn(a) ~= M.ReadyForTurnIn(b) then return M.ReadyForTurnIn(a) end
        if sort == "distance" then
            local aQuest = a.questID ~= nil and not a.custom and not a.recipe
            local bQuest = b.questID ~= nil and not b.custom and not b.recipe
            if aQuest ~= bQuest then return aQuest end
            if aQuest and distanceRank(a) ~= distanceRank(b) then return distanceRank(a) < distanceRank(b) end
        end
        if sort == "completion" and (a.completed == true) ~= (b.completed == true) then return a.completed ~= true end
        if sort == "level" or sort == "level_asc" then
            if (a.sortLevel ~= nil) ~= (b.sortLevel ~= nil) then return a.sortLevel ~= nil end
            if a.sortLevel and a.sortLevel ~= b.sortLevel then
                if sort == "level_asc" then return a.sortLevel < b.sortLevel end
                return a.sortLevel > b.sortLevel
            end
        end
        if sort == "progress" then
            if (a.progress ~= nil) ~= (b.progress ~= nil) then return a.progress ~= nil end
            if a.progress and a.progress ~= b.progress then return a.progress > b.progress end
        end
        if sort == "campaign" then
            local aCampaign = a.campaign == true and not a.custom and not a.recipe
            local bCampaign = b.campaign == true and not b.custom and not b.recipe
            if aCampaign ~= bCampaign then return aCampaign end
        end
        if sort == "manual" and (a.order or 0) ~= (b.order or 0) then return (a.order or 0) < (b.order or 0) end
        if (sort == "log" or sort == "campaign") and a.logOrder ~= b.logOrder then return a.logOrder < b.logOrder end
        if a.title:lower() ~= b.title:lower() then
            if sort == "title_desc" then return a.title:lower() > b.title:lower() end
            return a.title:lower() < b.title:lower()
        end
        return a.id < b.id
    end
    local visibleCount = 0
    for _, entry in ipairs(entries) do
        local categoryName = M.CategoryName(db, entry.category)
        local zone = zoneName(entry)
        local haystack = table.concat({ entry.title, zone, categoryName, entry.notes or "" }, " "):lower()
        if M.PassesFilters(db, entry) and (db.settings.showCompleted or not entry.completed) and (query == "" or haystack:find(query, 1, true)) then
            if sortMode == "level" or sortMode == "level_asc" then entry.sortLevel = questLevel(entry) end
            if sortMode == "progress" then entry.progress = M.QuestProgress(entry) end
            local mode, key, label = groupMode
            if mode == "zone" then key, label = "zone:" .. zone, zone
            elseif mode == "category" then key, label = "category:" .. (entry.category or "none"), categoryName
            elseif mode == "type" then
                local _, kindLabel = M.Kind(entry)
                label = kindLabel
                key = "type:" .. label
            else key, label = "all", "Quests" end
            local group = groupByKey[key]
            if not group then
                group = { key = key, label = label, entries = {}, collapsed = db.collapsedGroups[key] == true }
                groups[#groups + 1], groupByKey[key] = group, group
            end
            group.entries[#group.entries + 1] = entry
            visibleCount = visibleCount + 1
        end
    end
    table.sort(groups, function(a, b)
        if a.label:lower() == b.label:lower() then return a.key < b.key end
        return a.label:lower() < b.label:lower()
    end)
    for _, group in ipairs(groups) do table.sort(group.entries, less) end
    return groups, visibleCount
end

-- A move affects just the displayed group and normalizes its ranks, avoiding ties.
function M.Move(db, group, entryID, delta)
    for index, entry in ipairs(group.entries) do
        if entry.id == entryID then
            local target = math.max(1, math.min(#group.entries, index + delta))
            table.remove(group.entries, index)
            table.insert(group.entries, target, entry)
            for rank, item in ipairs(group.entries) do M.Meta(db, item).order = rank end
            db.settings.sort = "manual"
            db.settings.quickSort = ""
            db.settings.quickTurnIn = false
            return
        end
    end
end
