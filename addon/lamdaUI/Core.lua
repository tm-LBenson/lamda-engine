local _, LUI = ...
LUI.modules = {}
local defaults = {schema=1,cdEnabled=true,checkUpdates=true,notifyUpdates=true,
    checkDays=1,overlayX=60,overlayY=240,overlayScale=1,autoLog=true}
function LUI:DB()
    if type(LamdaEngineDB)~="table" then LamdaEngineDB={} end
    for k,v in pairs(defaults) do
        if type(LamdaEngineDB[k])~=type(v) then LamdaEngineDB[k]=v end
    end
    LamdaEngineDB.schema=1
    return LamdaEngineDB
end
function LUI:RegisterModule(module)
    assert(type(module.id)=="string" and type(module.build)=="function")
    for _,m in ipairs(self.modules) do assert(m.id~=module.id,"Duplicate module") end
    table.insert(self.modules,module)
end
-- Old SavedVariables are declared only to preserve existing users' data.
-- No legacy helpers, bindings, profiles, layouts or macros are loaded or modified.
function LUI:Save()
    if InCombatLockdown() then return end
    if self.commitInputs then self.commitInputs() end
    ReloadUI()
end
function LUI:EnsureLogging()
    if not self:DB().autoLog or not self:DB().cdEnabled then return end
    local inside,kind=IsInInstance()
    if not inside or kind~="party" then return end
    if type(LoggingCombat)=="function" then pcall(LoggingCombat,true) end
end
local events=CreateFrame("Frame")
events:RegisterEvent("PLAYER_LOGIN")
events:RegisterEvent("PLAYER_ENTERING_WORLD")
events:SetScript("OnEvent",function(_,event)
    LUI:DB()
    if event=="PLAYER_LOGIN" then
        LUI:BuildUI()
        SLASH_LAMDAUI1="/lui"
        SLASH_LAMDAUI2="/lamdaui"
        SlashCmdList.LAMDAUI=function() LUI.frame:Show() end
    end
    LUI:EnsureLogging()
    C_Timer.After(11,function() LUI:EnsureLogging() end)
end)
