local _, ns = ...
local N, M = ns.Navigation, ns.Model

-- /way #<mapID> x y label, /way <mapID> x y label, or /way x y label.
-- Named zones are left to the user's /way provider; never guess a zone ID.
function N.Parse(command, currentMapID)
    command = M.Trim(command)
    local verb, rest = command:match("^(/%S+)%s+(.+)$")
    if not verb or (verb:lower() ~= "/way" and verb:lower() ~= "/waypoint") then
        return nil, "Use /way #mapID x y label, or attach an existing macro."
    end
    local mapToken, x, y, label = rest:match("^#(%d+)%s+([%d%.,]+)%s+([%d%.,]+)%s*(.*)$")
    if not mapToken then
        mapToken, x, y, label = rest:match("^(%d+)%s+([%d%.,]+)%s+([%d%.,]+)%s*(.*)$")
    end
    if not mapToken then
        x, y, label = rest:match("^([%d%.,]+)%s+([%d%.,]+)%s*(.*)$")
    end
    x, y = tonumber(((x or ""):gsub(",", "."))), tonumber(((y or ""):gsub(",", ".")))
    local mapID = tonumber(mapToken) or currentMapID
    if not x or not y or x < 0 or x > 100 or y < 0 or y > 100 then
        return nil, "Coordinates must be numbers from 0 to 100. Named zones need a /way provider."
    end
    if not mapID or mapID <= 0 or mapID ~= math.floor(mapID) then return nil, "Include a valid #mapID." end
    return { mapID = mapID, x = x / 100, y = y / 100, title = label or "" }
end

function N.GetCurrentMap()
    return C_Map and C_Map.GetBestMapForUnit and C_Map.GetBestMapForUnit("player")
end

local function slashProvider(command)
    local verb, rest = command:match("^(/%S+)%s*(.*)$")
    if not verb then return end
    -- Only delegate navigation commands, never arbitrary slash handlers or /run.
    if verb:lower() ~= "/way" and verb:lower() ~= "/waypoint" then return end
    for key, handler in pairs(SlashCmdList or {}) do
        for index = 1, 20 do
            local alias = _G["SLASH_" .. key .. index]
            if not alias then break end
            if alias:lower() == verb:lower() and type(handler) == "function" then
                local ok, err = pcall(handler, rest)
                if not ok then return false, "Waypoint provider failed: " .. tostring(err) end
                return true
            end
        end
    end
end

function N.Run(command, title)
    command = M.Trim(command)
    if command:find("[\r\n]") then return false, "Use one /way command per entry, or attach an existing macro." end
    local verb = command:match("^(/%S+)")
    if not verb or (verb:lower() ~= "/way" and verb:lower() ~= "/waypoint") then
        return false, "Use a /way command or attach an existing macro."
    end
    local point, err = N.Parse(command, N.GetCurrentMap())
    -- Prefer the installed /way handler (WaypointUI or TomTom), including zone names.
    local provided, providerError = slashProvider(command)
    if provided ~= nil then return provided, providerError end
    if not point then return false, err end
    if C_Map and C_Map.SetUserWaypoint and UiMapPoint and UiMapPoint.CreateFromCoordinates then
        if C_Map.CanSetUserWaypointOnMap and not C_Map.CanSetUserWaypointOnMap(point.mapID) then
            return false, "This map does not support a native waypoint. Enable a /way provider."
        end
        local mapPoint = UiMapPoint.CreateFromCoordinates(point.mapID, point.x, point.y)
        local ok, failure = pcall(C_Map.SetUserWaypoint, mapPoint)
        if not ok then return false, "Waypoint unavailable: " .. tostring(failure) end
        if C_SuperTrack and C_SuperTrack.SetSuperTrackedUserWaypoint then
            pcall(C_SuperTrack.SetSuperTrackedUserWaypoint, true)
        end
        return true
    end
    return false, "Enable WaypointUI or TomTom for waypoint navigation on this client."
end

function N.Copy(command)
    if ChatFrame_OpenChat then ChatFrame_OpenChat(command) end
end
