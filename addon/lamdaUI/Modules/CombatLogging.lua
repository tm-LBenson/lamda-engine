local _, Addon = ...

local evaluationSerial = 0
local retryDelays = { 0, 11 }
local loggingIndicator
local statusTicker
local nextLoggingAttempt = 0

local function getCombatLoggingState()
    if C_ChatInfo and type(C_ChatInfo.IsLoggingCombat) == "function" then
        local succeeded, enabled, advanced = pcall(C_ChatInfo.IsLoggingCombat)
        if succeeded then
            return enabled == true, advanced == true
        end
    end
    return false, false
end

local function createLoggingIndicator()
    if loggingIndicator or type(CreateFrame) ~= "function" or not Minimap then
        return loggingIndicator
    end

    local frame = CreateFrame("Frame", "LamdaUICombatLoggingIndicator", Minimap)
    frame:SetSize(31, 31)
    frame:SetPoint("TOPRIGHT", Minimap, "TOPLEFT", -4, -2)
    frame:SetFrameStrata(Minimap:GetFrameStrata() or "MEDIUM")
    frame:SetFrameLevel((Minimap:GetFrameLevel() or 0) + 8)
    frame:SetClampedToScreen(true)
    frame:EnableMouse(true)

    local background = frame:CreateTexture(nil, "BACKGROUND")
    background:SetSize(24, 24)
    background:SetPoint("CENTER")
    background:SetTexture(136467) -- Interface\Minimap\UI-Minimap-Background

    local icon = frame:CreateTexture(nil, "ARTWORK")
    icon:SetSize(18, 18)
    icon:SetPoint("CENTER")
    icon:SetTexture("Interface\\Icons\\INV_Misc_Book_09")
    icon:SetTexCoord(0.08, 0.92, 0.08, 0.92)

    local border = frame:CreateTexture(nil, "OVERLAY")
    border:SetTexture(136430) -- Interface\Minimap\MiniMap-TrackingBorder
    border:SetSize(50, 50)
    border:SetPoint("TOPLEFT")

    frame:SetScript("OnEnter", function(self)
        GameTooltip:SetOwner(self, "ANCHOR_LEFT")
        GameTooltip:SetText("Combat logging active", 0.40, 0.85, 1)
        local _, advanced = getCombatLoggingState()
        GameTooltip:AddLine(
            advanced and "Advanced combat data is being recorded."
                or "Basic combat data is being recorded.",
            1, 1, 1, true
        )
        GameTooltip:AddLine(
            "lamdaUI enables logging automatically when you enter an instance.",
            0.75, 0.75, 0.75, true
        )
        GameTooltip:Show()
    end)
    frame:SetScript("OnLeave", function()
        GameTooltip:Hide()
    end)
    frame:Hide()
    loggingIndicator = frame
    return frame
end

function Addon:UpdateCombatLoggingIndicator()
    if not self:IsCustomModuleEnabled("combatLogging") then
        if loggingIndicator then loggingIndicator:Hide() end
        return
    end
    local frame = createLoggingIndicator()
    if not frame then
        return
    end
    local active = getCombatLoggingState()
    if active then
        frame:Show()
    else
        if GameTooltip and GameTooltip:IsOwned(frame) then
            GameTooltip:Hide()
        end
        frame:Hide()
    end
end

local function startStatusTicker()
    if statusTicker or not C_Timer or type(C_Timer.NewTicker) ~= "function" then
        return
    end
    statusTicker = C_Timer.NewTicker(1, function()
        Addon:UpdateCombatLoggingIndicator()
    end)
end

local function enableAdvancedCombatLogging()
    if type(GetCVar) == "function" and GetCVar("advancedCombatLogging") == "1" then
        return
    end

    local setCVar = C_CVar and C_CVar.SetCVar or SetCVar
    if type(setCVar) == "function" then
        pcall(setCVar, "advancedCombatLogging", "1")
    end
end

function Addon:EnsureInstanceCombatLogging()
    if not self:IsCustomModuleEnabled("combatLogging") then return false end
    if type(IsInInstance) ~= "function" or type(LoggingCombat) ~= "function" then
        return false
    end

    local inInstance, instanceType = IsInInstance()
    if not inInstance or instanceType == "none" then
        return false
    end

    enableAdvancedCombatLogging()
    if getCombatLoggingState() then
        self:UpdateCombatLoggingIndicator()
        return true
    end

    local now = type(GetTime) == "function" and GetTime() or 0
    if now < nextLoggingAttempt then
        return false
    end
    nextLoggingAttempt = now + 10.5

    local changed, enabled = pcall(LoggingCombat, true)
    if changed and (enabled == true or getCombatLoggingState()) then
        self:UpdateCombatLoggingIndicator()
        print("|cff65d9fflamdaUI|r Combat logging enabled for this instance.")
        return true
    end

    return false
end

function Addon:ScheduleInstanceCombatLogging()
    evaluationSerial = evaluationSerial + 1
    local scheduledSerial = evaluationSerial

    for _, delay in ipairs(retryDelays) do
        C_Timer.After(delay, function()
            if scheduledSerial == evaluationSerial then
                Addon:EnsureInstanceCombatLogging()
            end
        end)
    end
end

local eventFrame = CreateFrame("Frame")
eventFrame:RegisterEvent("PLAYER_LOGIN")
eventFrame:RegisterEvent("PLAYER_ENTERING_WORLD")
eventFrame:RegisterEvent("ZONE_CHANGED_NEW_AREA")
eventFrame:SetScript("OnEvent", function()
    startStatusTicker()
    Addon:UpdateCombatLoggingIndicator()
    Addon:ScheduleInstanceCombatLogging()
end)

Addon:RegisterCustomModule({
    id = "combatLogging", name = "Combat logging",
    description = "Enables advanced combat logging on entering an instance. The book by the minimap shows when recording is active.",
    defaults = { enabled = true },
    changed = function()
        Addon:UpdateCombatLoggingIndicator()
        if Addon:IsCustomModuleEnabled("combatLogging") then Addon:ScheduleInstanceCombatLogging() end
    end,
    buildOptions = function(parent)
        Addon:CustomHubText(parent, "Disabling this module stops automatic starts and hides its indicator.\nAn existing recording continues until you stop it with /combatlog.\n\nYour logs stay on this computer for later review.", 0, 0, 505)
    end,
})
