local _, Addon = ...

-- MiniAuras is an optional dependency, so its SavedVariables are available
-- before this file loads. Apply the confirmed local setup before MiniAuras'
-- PLAYER_ENTERING_WORLD refresh; this makes the change safe while WoW is open
-- and prevents /reload from overwriting an offline SavedVariables edit.
local devastationSpecId = 1467
local imminentDestructionId = 411055
local strafingRunId = 1266165
local dragonrageId = 375087
local essenceBurstId = 359618
local setupVersion = 2

local function copyTable(value)
    if type(value) ~= "table" then return value end

    local copied = {}
    for key, nestedValue in pairs(value) do
        copied[copyTable(key)] = copyTable(nestedValue)
    end
    return copied
end

local function groupTracksSpell(group, spellId)
    if type(group) ~= "table" or type(group.Spells) ~= "table" then
        return false
    end

    for _, trackedId in ipairs(group.Spells) do
        if trackedId == spellId then return true end
    end
    return false
end

local function findGroup(options, spellId)
    if type(options) ~= "table" or type(options.Groups) ~= "table" then
        return nil
    end

    for _, group in ipairs(options.Groups) do
        if groupTracksSpell(group, spellId) then return group end
    end
end

local function setScreenPosition(group, x)
    if type(group) ~= "table" then return end

    group.Position = type(group.Position) == "table" and group.Position or {}
    group.Position.Point = "CENTER"
    group.Position.RelativePoint = "CENTER"
    group.Position.X = x
    group.Position.Y = 220
end

local function nextGroupNumber(...)
    local nextId = 1

    for index = 1, select("#", ...) do
        local options = select(index, ...)
        if type(options) == "table" then
            nextId = math.max(nextId, tonumber(options.NextId) or 1)
            for _, group in ipairs(type(options.Groups) == "table" and options.Groups or {}) do
                local number = type(group.Id) == "string" and tonumber(group.Id:match("^g(%d+)$"))
                if number then nextId = math.max(nextId, number + 1) end
            end
        end
    end

    return nextId
end

local function configureImminentGroup(group, id)
    group.Id = id
    group.Name = "Imminent Destruction"
    group.Enabled = true
    group.Icon = ""
    group.Unit = "player"
    group.Anchor = "SCREEN"
    group.AuraType = "HELPFUL"
    group.TrackingMode = "SPELLS"
    group.Spells = { imminentDestructionId }
    group.Specs = { [devastationSpecId] = true }
    group.Caster = "ANY"
    group.Sort = "OLDEST"
    group.Grow = "CENTER"
    group.Strata = "AUTO"
    group.ShowWhen = "INCOMBAT"
    group.Filters = {}
    group.Candidates = {}
    group.Offset = type(group.Offset) == "table" and group.Offset or { X = 0, Y = 40 }
    setScreenPosition(group, 36)

    group.Icons = type(group.Icons) == "table" and group.Icons or {}
    group.Icons.Display = "ICON"
    group.Icons.Size = 64
    group.Icons.FontScale = 1.2
    group.Icons.Glow = true
    group.Icons.Border = true
    group.Icons.EnableSwipe = true
    group.Icons.EnableNumbers = true
    group.Icons.ReverseCooldown = true
    group.Icons.CenterStacks = false
    group.Icons.UseGroupIcon = false
    group.Icons.ShowTooltips = false
    group.Icons.Color = { R = 0.35, G = 0.85, B = 1, A = 1 }

    group.Sound = type(group.Sound) == "table" and group.Sound or {}
    group.Sound.Applied = ""
    group.Sound.Removed = ""
    group.Sound.Stacks = ""
    group.Sound.Channel = "Master"
end

local function configureStrafingGroup(group, id)
    group.Id = id
    group.Name = "Strafing Run"
    group.Enabled = true
    group.Icon = ""
    group.Unit = "player"
    group.Anchor = "SCREEN"
    group.AuraType = "HELPFUL"
    group.TrackingMode = "SPELLS"
    group.Spells = { strafingRunId }
    group.Specs = { [devastationSpecId] = true }
    group.Caster = "ANY"
    group.Sort = "OLDEST"
    group.Grow = "CENTER"
    group.Strata = "AUTO"
    group.ShowWhen = "INCOMBAT"
    group.Filters = {}
    group.Candidates = {}
    group.Offset = type(group.Offset) == "table" and group.Offset or { X = 0, Y = 40 }
    setScreenPosition(group, -36)

    group.Icons = type(group.Icons) == "table" and group.Icons or {}
    group.Icons.Display = "ICON"
    group.Icons.Size = 64
    group.Icons.FontScale = 1.2
    group.Icons.Glow = true
    group.Icons.Border = true
    group.Icons.EnableSwipe = true
    group.Icons.EnableNumbers = true
    group.Icons.ReverseCooldown = true
    group.Icons.CenterStacks = false
    group.Icons.UseGroupIcon = false
    group.Icons.ShowTooltips = false
    group.Icons.Color = { R = 0.35, G = 1, B = 0.55, A = 1 }

    group.Sound = type(group.Sound) == "table" and group.Sound or {}
    group.Sound.Applied = ""
    group.Sound.Removed = ""
    group.Sound.Stacks = ""
    group.Sound.Channel = "Master"
end

local function upsertImminentGroup(options, id, template)
    if type(options) ~= "table" or type(options.Groups) ~= "table" then
        return nil
    end

    local group = findGroup(options, imminentDestructionId)
    if not group then
        group = copyTable(template or {})
        table.insert(options.Groups, group)
    end

    configureImminentGroup(group, id)
    local idNumber = tonumber(id:match("^g(%d+)$")) or 0
    options.NextId = math.max(tonumber(options.NextId) or 1, idNumber + 1)
    return group
end

local function upsertStrafingGroup(options, id, template)
    if type(options) ~= "table" or type(options.Groups) ~= "table" then
        return nil
    end

    local group = findGroup(options, strafingRunId)
    if not group then
        group = copyTable(template or {})
        table.insert(options.Groups, group)
    end

    configureStrafingGroup(group, id)
    local idNumber = tonumber(id:match("^g(%d+)$")) or 0
    options.NextId = math.max(tonumber(options.NextId) or 1, idNumber + 1)
    return group
end

local function configureOptions(options)
    if type(options) ~= "table" then return end
    setScreenPosition(findGroup(options, dragonrageId), -108)
    setScreenPosition(findGroup(options, strafingRunId), -36)
    setScreenPosition(findGroup(options, imminentDestructionId), 36)
    setScreenPosition(findGroup(options, essenceBurstId), 108)
end

local function configureMiniAuras()
    local database = MiniAurasDB
    local liveOptions = database and database.Modules and database.Modules.PersonalAuras
    if type(liveOptions) ~= "table" or type(liveOptions.Groups) ~= "table" then
        return false
    end

    local activeProfile = database.Profiles and database.Profiles[database.ActiveProfile]
    local profileOptions = activeProfile and activeProfile.Modules
        and activeProfile.Modules.PersonalAuras
    local liveGroup = findGroup(liveOptions, imminentDestructionId)
    local profileGroup = findGroup(profileOptions, imminentDestructionId)
    local id = liveGroup and liveGroup.Id or profileGroup and profileGroup.Id

    if type(id) ~= "string" or not id:match("^g%d+$") then
        id = "g" .. nextGroupNumber(liveOptions, profileOptions)
    end

    local template = findGroup(liveOptions, essenceBurstId)
        or findGroup(liveOptions, dragonrageId)
    if not template then return false end
    liveGroup = upsertImminentGroup(liveOptions, id, template)
    if not liveGroup then return false end

    local liveStrafing = findGroup(liveOptions, strafingRunId)
    local profileStrafing = findGroup(profileOptions, strafingRunId)
    local strafingId = liveStrafing and liveStrafing.Id
        or profileStrafing and profileStrafing.Id
    if type(strafingId) ~= "string" or not strafingId:match("^g%d+$") then
        strafingId = "g" .. nextGroupNumber(liveOptions, profileOptions)
    end
    if not upsertStrafingGroup(liveOptions, strafingId, template) then
        return false
    end

    configureOptions(liveOptions)

    -- MiniAuras profiles retain their own Personal Auras snapshot. Keep the
    -- active one byte-for-byte equivalent to the newly configured live table
    -- so a later profile switch cannot restore the old two-icon row.
    if activeProfile and type(activeProfile.Modules) == "table" then
        activeProfile.Modules.PersonalAuras = copyTable(liveOptions)
    end
    return true
end

local eventFrame = CreateFrame("Frame")
eventFrame:RegisterEvent("PLAYER_LOGIN")
eventFrame:SetScript("OnEvent", function(self)
    self:UnregisterAllEvents()
    if not Addon:IsCustomModuleEnabled("miniAurasSetup") then return end

    if Addon.db and type(Addon.db.options) == "table"
        and (tonumber(Addon.db.options.miniAurasSetupVersion) or 0) >= setupVersion
    then
        return
    end

    local configured = configureMiniAuras()
    if configured then
        if Addon.db and type(Addon.db.options) == "table" then
            Addon.db.options.miniAurasSetupVersion = setupVersion
        end
        print("|cff65d9fflamdaUI|r MiniAuras now tracks Imminent Destruction and Strafing Run.")
    else
        Addon.miniAurasSetupPending = true
    end
end)

Addon:RegisterCustomModule({
    id = "miniAurasSetup", name = "MiniAuras setup",
    description = "Maintains the existing Devastation aura setup in MiniAuras. MiniAuras remains a separate addon with its own display settings.",
    defaults = { enabled = true },
    buildOptions = function(parent)
        Addon:CustomHubText(parent, "This toggle controls future setup on login. Existing MiniAuras groups\nremain as you configured them. Reload after enabling it.\n\nUse MiniAuras settings to change or remove those aura displays.", 0, 0, 505)
    end,
})
