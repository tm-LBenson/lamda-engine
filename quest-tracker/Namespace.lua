local addonName, ns = ...
ns.name = addonName
ns.version = "1.0.0"
ns.Model = {}
ns.Navigation = {}
ns.Compat = {}
ns.Recipes = {}
ns.UI = {}

function ns.Print(message)
    if DEFAULT_CHAT_FRAME then
        DEFAULT_CHAT_FRAME:AddMessage("|cffffd100Lamda Quest Log:|r " .. tostring(message))
    end
end

function ns.InCombat()
    return InCombatLockdown and InCombatLockdown() or false
end

