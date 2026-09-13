local _, Addon = ...

-- Optional integrations register themselves here. The base addon does not
-- assume that a player uses any particular third-party UI package.
Addon.trackedAddons = Addon.trackedAddons or {}

function Addon:RegisterAddonAdapter(name, label, adapter)
    if type(name) ~= "string" or name == "" then
        return false
    end
    table.insert(self.trackedAddons, {
        name = name,
        label = label or name,
        adapter = adapter,
    })
    self.trackedAddonNames[name] = true
    return true
end

Addon.trackedAddonNames = {}
for _, tracked in ipairs(Addon.trackedAddons) do
    Addon.trackedAddonNames[tracked.name] = true
end

local function getAddonInfo(addonName)
    if not C_AddOns or not C_AddOns.GetAddOnInfo then
        return nil
    end

    local ok, info = pcall(C_AddOns.GetAddOnInfo, addonName)
    if not ok then
        return nil
    end
    return info
end

function Addon:GetAddonAudit()
    local result = {}
    for _, tracked in ipairs(self.trackedAddons) do
        local info = getAddonInfo(tracked.name)
        local loaded = C_AddOns and C_AddOns.IsAddOnLoaded and C_AddOns.IsAddOnLoaded(tracked.name) or false
        local installed = info ~= nil

        result[tracked.name] = {
            name = tracked.name,
            label = tracked.label,
            installed = installed,
            loaded = loaded and true or false,
            adapter = tracked.adapter,
            status = loaded and "Loaded" or (installed and "Installed, not loaded" or "Not installed"),
        }
    end
    return result
end
