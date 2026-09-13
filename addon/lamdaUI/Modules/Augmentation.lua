local _, Addon = ...

-- Emergency recovery after live-client errors in 0.19.2.
-- No combat events, update loop, frame creation, charge queries, aura queries,
-- native Cooldown Manager edits, or timer callbacks run in this module.
local function suspend()
    local settings = Addon:GetCustomModuleSettings("augmentation")
    if not settings then return end
    if settings.enabledBeforeRecovery == nil then settings.enabledBeforeRecovery = settings.enabled ~= false end
    settings.enabled = false
    settings.suspended = true
end

Addon:RegisterCustomModule({
    id = "augmentation",
    name = "Augmentation icons",
    description = "Temporarily disabled after UI errors. Your size, position, and display preferences are saved.",
    defaults = { enabled = false },
    initialize = suspend,
    changed = function()
        suspend()
        print("|cff65d9fflamdaUI:|r Augmentation icons are suspended while the UI errors are investigated.")
    end,
    buildOptions = function(parent)
        Addon:CustomHubText(parent,
            "The combat tracker is not running.\nOther lamdaUI modules remain available.\n\nEnabling this checkbox will not restart the failed tracker.",
            0, 0, 505)
    end,
})

function Addon:AugmentationCommand(command)
    if not self.db or self.legacyConflict then return end
    if command == "status" then
        print("|cff65d9fflamdaUI:|r Augmentation tracker suspended for recovery; no combat tracking is running.")
        return
    end
    self:Show("custom")
    self:SelectCustomModule("augmentation")
end

SLASH_LAMDAUIAUGMENTATION1 = "/aughelper"
SLASH_LAMDAUIAUGMENTATION2 = "/ebonalert"
SlashCmdList.LAMDAUIAUGMENTATION = function(command) Addon:AugmentationCommand(command) end
