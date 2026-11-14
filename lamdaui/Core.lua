local _, LUI = ...
LUI.modules = {}
local defaults = {schema=1,colorStripVersion=2,checkUpdates=true,notifyUpdates=true,checkDays=1,keepUIWhenUnfocused=false,allowCombatManagement=true,scanTiming=2,scanMinutes=3,startHidden=false,instanceAutoLog=false}
local limits = {colorStripVersion={2,2,integer=true},checkDays={values={1,7}},scanTiming={values={1,2,3,4}},scanMinutes={values={1,3,5}}}
local moduleKeys = {}
-- Modules declare settings delivered through SavedVariables. Everything else
-- applies in the addon (or through live frame packets) without a reload.
local reloadKeys = {checkUpdates=true,notifyUpdates=true,checkDays=true,scanTiming=true,scanMinutes=true}
-- Machine-wide limits leave module/profile choices intact. Simple mode ignores
-- the remembered advanced choices; returning to Advanced restores them.
local resourceKeys={"resourceInspect","resourceLookups","resourceDiscovery","resourceFastIdle","resourceSmooth","resourceCPU"}
defaults.resourceAdvanced=false;defaults.resourceLow=false;defaults.resourceCustomized=false
reloadKeys.resourceAdvanced=true;reloadKeys.resourceLow=true
for _,key in ipairs(resourceKeys)do defaults[key]=true end
for _,key in ipairs({"resourceLookups","resourceFastIdle","resourceSmooth"})do reloadKeys[key]=true end
function LUI:OptionalWork(key,db)
    db=db or self:DB()
    if db.resourceAdvanced then return db[key]~=false end
    return not db.resourceLow
end
function LUI:CDTrackingEnabled(db)
    db=db or self:DB()
    return db.cdEnabled and db.engineTracking and db.engineFrameEstimates and db.lookupBridge
end
function LUI:SetResourceAdvanced(value)
    local db=self:DB()
    if value and not db.resourceCustomized then
        for _,key in ipairs(resourceKeys)do db[key]=not db.resourceLow end
        db.resourceCustomized=true
    end
    db.resourceAdvanced=not not value
    if self.RefreshUI then self:RefreshUI()end
end
function LUI:ResetResourceSettings()
    local keys={"resourceAdvanced","resourceLow","resourceCustomized"}
    for _,key in ipairs(resourceKeys)do keys[#keys+1]=key end
    self:ResetKeys(keys)
end
local function valid(value, default, rule)
    if type(value)~=type(default) then return false end
    if rule and rule.validate and not rule.validate(value) then return false end
    if type(value)=="number" then
        if value~=value or value==math.huge or value==-math.huge then return false end
        if rule then
            if rule[1] and value<rule[1] or rule[2] and value>rule[2] then return false end
            if rule.integer and value~=math.floor(value) then return false end
            if rule.values then
                for _,v in ipairs(rule.values) do if v==value then return true end end
                return false
            end
        end
    elseif type(value)=="string" and #value>8192 then return false end
    return true
end
function LUI:DB()
    if type(LamdaEngineDB)~="table" then LamdaEngineDB={} end
    -- Preserve existing users of the formerly always-available frame/sim modules.
    if self.FramesEnabledAtLoad~=nil and type(LamdaEngineDB.framesEnabled)~="boolean" then
        LamdaEngineDB.framesEnabled=self.FramesEnabledAtLoad
    end
    if LamdaEngineDB.simEnabled==nil and LamdaEngineDB.simCurrentTab~=nil then LamdaEngineDB.simEnabled=true end
    if self.SpellLists then self.SpellLists.MigrateDefaults(LamdaEngineDB)end
    for key,default in pairs(defaults) do
        if not valid(LamdaEngineDB[key],default,limits[key]) then LamdaEngineDB[key]=default end
    end
    LamdaEngineDB.schema=1
    if self.FramesEnabledAtLoad~=nil and type(LamdaFramesDB_v2)=="table" then
        LamdaFramesDB_v2.lamdaModuleEnabled=LamdaEngineDB.framesEnabled==true
    end
    return LamdaEngineDB
end
function LUI:RegisterModule(module)
    assert(type(module.id)=="string" and type(module.name)=="string" and type(module.build)=="function")
    assert(module.onReset==nil or type(module.onReset)=="function","Invalid module reset handler")
    assert(module.settingsScope==nil or module.settingsScope=="global","Unsupported settings scope")
    for _,m in ipairs(self.modules) do assert(m.id~=module.id,"Duplicate module") end
    module.defaults=module.defaults or {}
    if module.enabledKey then
        assert(type(module.enabledKey)=="string","Module enabledKey must be a string")
        if module.defaults[module.enabledKey]==nil then module.defaults[module.enabledKey]=false end
    end
    for key,default in pairs(module.defaults) do
        assert(type(key)=="string" and key:match("^[%a][%w_]*$") and defaults[key]==nil,"Duplicate or invalid setting")
        assert(type(default)=="number" or type(default)=="boolean" or type(default)=="string","Unsupported setting type")
        defaults[key]=default
        -- Diagnostics may remain global while owning their controls/defaults.
        -- Such settings are not copied when switching or exporting CD profiles.
        if module.settingsScope~="global" then moduleKeys[key]=true end
        limits[key]=module.limits and module.limits[key]
    end
    table.insert(self.modules,module)
    for _,key in ipairs(module.reloadKeys or {})do
        assert(defaults[key]~=nil,"Unknown reload setting");reloadKeys[key]=true
    end
end
local function snapshot()
    local values={};local db=LUI:DB()
    for key in pairs(moduleKeys) do values[key]=db[key] end
    return values
end
local function normalized(values)
    local result={}
    for key in pairs(moduleKeys) do
        -- Validate once and preserve a valid false value.
        if valid(values[key],defaults[key],limits[key]) then result[key]=values[key]
        else result[key]=defaults[key]end
    end
    if LUI.SpellLists and values.defensiveCatalogVersion~=1 then
        result.defensiveCatalogVersion=nil;LUI.SpellLists.MigrateDefaults(result)
    end
    return result
end
local function profileName(name)
    if type(name)~="string" then return nil end
    name=name:match("^%s*(.-)%s*$")
    if #name<1 or #name>40 or name:find("[%c|]") then return nil end
    return name
end
function LUI:ProfileDB()
    if type(LamdaUIDB)~="table" then LamdaUIDB={} end
    if type(LamdaUIDB.engineHub)~="table" then LamdaUIDB.engineHub={} end
    local hub=LamdaUIDB.engineHub
    if type(hub.profiles)~="table" then hub.profiles={} end
    for name,values in pairs(hub.profiles) do
        if profileName(name)~=name or type(values)~="table" then hub.profiles[name]=nil
        else hub.profiles[name]=normalized(values) end
    end
    if not hub.profiles[hub.active] then
        hub.active="Default";hub.profiles.Default=snapshot()
    end
    hub.schema=1
    return hub
end
function LUI:ProfileNames()
    local names={};for name in pairs(self:ProfileDB().profiles) do table.insert(names,name) end
    table.sort(names);return names
end
function LUI:SaveActiveProfile()
    local hub=self:ProfileDB();hub.profiles[hub.active]=snapshot()
end
function LUI:SelectProfile(name)
    local hub=self:ProfileDB();if not hub.profiles[name] then return false,"Choose a profile." end
    self:SaveActiveProfile();hub.active=name
    local db=self:DB();for key,value in pairs(normalized(hub.profiles[name])) do db[key]=value end
    if self.RefreshUI then self:RefreshUI() end
    return true
end
function LUI:CreateProfile(name)
    name=profileName(name);if not name then return false,"Use a name of 1–40 characters." end
    local hub=self:ProfileDB();if hub.profiles[name] then return false,"That profile already exists." end
    self:SaveActiveProfile();hub.profiles[name]=snapshot();hub.active=name
    if self.RefreshUI then self:RefreshUI() end
    return true
end
function LUI:RenameProfile(name)
    name=profileName(name);if not name then return false,"Use a name of 1–40 characters." end
    local hub=self:ProfileDB();if name==hub.active then return true end
    if hub.profiles[name] then return false,"That profile already exists." end
    hub.profiles[name]=snapshot();hub.profiles[hub.active]=nil;hub.active=name
    if self.RefreshUI then self:RefreshUI() end
    return true
end
function LUI:DeleteProfile()
    local hub=self:ProfileDB();local names=self:ProfileNames()
    if #names==1 then return false,"Keep at least one profile." end
    local old=hub.active;hub.profiles[old]=nil
    for _,name in ipairs(names) do if name~=old then hub.active=name;break end end
    local db=self:DB();for key,value in pairs(normalized(hub.profiles[hub.active])) do db[key]=value end
    if self.RefreshUI then self:RefreshUI() end
    return true
end
function LUI:ResetModule(id)
    for _,module in ipairs(self.modules) do if module.id==id then
        local db=self:DB();for key,value in pairs(module.defaults) do db[key]=value end
        if module.onReset then module.onReset() end
        if self.RefreshUI then self:RefreshUI() end
        return true
    end end
    return false
end
local function hex(text)return (text:gsub(".",function(c)return string.format("%02x",string.byte(c))end))end
function LUI:ExportProfile()
    local values=snapshot();local keys={};for key in pairs(values) do table.insert(keys,key) end;table.sort(keys)
    local lines={"LUI1"}
    for _,key in ipairs(keys) do
        local value=values[key];local encoded
        if type(value)=="boolean" then encoded=value and "b:1" or "b:0"
        elseif type(value)=="number" then encoded="n:"..tostring(value)
        else encoded="s:"..hex(value) end
        table.insert(lines,key.."="..encoded)
    end
    return table.concat(lines,"\n")
end
function LUI:ImportProfile(name,text)
    name=profileName(name);if not name then return false,"Use a name of 1–40 characters." end
    local hub=self:ProfileDB();if hub.profiles[name] then return false,"That profile already exists." end
    if type(text)~="string" or #text>65536 then return false,"Invalid profile." end
    text=text:gsub("\r","")
    if text:sub(1,5)~="LUI1\n" then return false,"Use a LamdaUI profile export." end
    local values={};local count=0
    for line in text:sub(6):gmatch("[^\n]+") do
        local key,kind,raw=line:match("^([%a][%w_]*)=([bns]):(.*)$")
        if not key or not moduleKeys[key] or values[key]~=nil then return false,"Unsupported or repeated setting." end
        local value
        if kind=="b" then if raw=="1" then value=true elseif raw=="0" then value=false end
        elseif kind=="n" then value=tonumber(raw)
        elseif #raw%2==0 and not raw:find("[^%x]") then value=(raw:gsub("..",function(pair)return string.char(tonumber(pair,16))end)) end
        if not valid(value,defaults[key],limits[key]) then return false,"Invalid value for "..key.."." end
        values[key]=value;count=count+1
    end
    if count==0 then return false,"The profile is empty." end
    self:SaveActiveProfile();hub.profiles[name]=normalized(values);hub.active=name
    local db=self:DB();for key,value in pairs(hub.profiles[name]) do db[key]=value end
    if self.RefreshUI then self:RefreshUI() end
    return true
end
function LUI:DefaultValue(key)return defaults[key]end
function LUI:IsModuleEnabledKey(key)
    for _,module in ipairs(self.modules)do if module.enabledKey==key then return true end end
    return false
end
function LUI:CombatManagementEnabled()
    return self:DB().allowCombatManagement~=false
end
function LUI:CombatManagementBlocked()
    return InCombatLockdown() and not self:CombatManagementEnabled()
end
local function callModule(target,method,...)
    if not target or type(target[method])~="function" then return false end
    return pcall(target[method],target,...)
end
function LUI:SetModulesUnlocked(unlocked)
    unlocked=unlocked==true
    local db=self:DB()
    callModule(self.LookupEvents,"SetMoving",unlocked,true)
    callModule(self.Kick,"SetMoving",unlocked)
    callModule(self.Performance,"SetMoving",unlocked)
    if self.Gladius and db.gladiusEnabled then
        db.gladiusLocked=not unlocked
        callModule(self.Gladius,"RefreshSettings",true)
    end
    callModule(self.KBB,"SetDisplaysLocked",not unlocked)
    local mover
    if LibStub then
        local ok,value=pcall(function()return LibStub("LamdaMover-1.0",true)end)
        if ok then mover=value end
    end
    callModule(mover,unlocked and "Unlock" or "Lock")
    callModule(self.FrameAuras,"SetLayoutPreview",unlocked)
    if self.RefreshUI then self:RefreshUI()end
    if unlocked then
        print("LamdaUI: Movable displays unlocked. Drag any visible handle; use /lamda lock when finished.")
        if InCombatLockdown()then print("LamdaUI: Protected frame layouts remain locked until combat ends.")end
    else
        print("LamdaUI: Movable displays locked.")
    end
end
function LUI:ResetKeys(keys)
    local db=self:DB()
    for _,key in ipairs(keys)do if defaults[key]~=nil then db[key]=defaults[key]end end
    self:SaveActiveProfile()
    if self.RefreshUI then self:RefreshUI()end
end
function LUI:ResetGeneral()
    local keys={"checkUpdates","notifyUpdates","checkDays","keepUIWhenUnfocused","allowCombatManagement","instanceAutoLog","resourceAdvanced","resourceLow","resourceCustomized","scanTiming","scanMinutes","startHidden"}
    for _,key in ipairs(resourceKeys)do keys[#keys+1]=key end
    self:ResetKeys(keys)
end
function LUI:ResetAllSettings()
    for _,module in ipairs(self.modules) do if module.onReset then module.onReset() end end
    local keys={};for key in pairs(defaults)do keys[#keys+1]=key end;self:ResetKeys(keys)
end
-- WoW writes SavedVariables on reload/logout. Apply keeps the active profile in
-- this session and updates safe native settings without forcing a reload.
function LUI:CaptureLoadedSettings()
    self.loadedSettings={};local db=self:DB()
    for key in pairs(reloadKeys)do self.loadedSettings[key]=db[key]end
    self.reloadRequired=false
end
function LUI:UpdateReloadRequired()
    local changed={};local db=self:DB()
    if self.loadedSettings then
        for key in pairs(reloadKeys)do if db[key]~=self.loadedSettings[key]then changed[#changed+1]=key end end
    end
    table.sort(changed);self.reloadChanges=changed;self.reloadRequired=#changed>0
    return self.reloadRequired
end
function LUI:Apply()
    if self.commitInputs then self.commitInputs()end
    self:SaveActiveProfile();self.applied=true;self:UpdateReloadRequired();self:EnsureLogging()
    if self.RefreshUI then self:RefreshUI()end
end
function LUI:Save()
    if self:CombatManagementBlocked() then return end
    self:Apply();ReloadUI()
end
function LUI:EnsureLogging()
    local db=self:DB()
    if not db.instanceAutoLog and not (self:CDTrackingEnabled(db) and db.autoLog) and not (db.kickEnabled and db.kickAutoLog) then return end
    local inside,kind=IsInInstance()
    if not inside or (kind~="party" and kind~="scenario" and kind~="raid" and kind~="arena" and not (kind=="pvp" and db.instanceAutoLog)) then return end
    if type(LoggingCombat)=="function" then pcall(LoggingCombat,true) end
end
local events=CreateFrame("Frame")
events:RegisterEvent("PLAYER_LOGIN")
events:RegisterEvent("PLAYER_ENTERING_WORLD")
events:SetScript("OnEvent",function(_,event)
    LUI:DB();LUI:ProfileDB()
    if event=="PLAYER_LOGIN" then
        LUI.overlayHidden=LamdaEngineDB.startHidden
        LUI:CaptureLoadedSettings()
        LUI:BuildUI()
        SLASH_LAMDAUI1="/lamda";SLASH_LAMDAUI2="/lamdaui"
        SlashCmdList.LAMDAUI=function(text)
            text=(text or ""):lower():match("^%s*(.-)%s*$")
            if text=="fps" and LUI.Performance then
                LUI.Performance:ToggleTracker()
            elseif text=="gladius" and LUI.Gladius then
                LUI.Gladius:Open()
            elseif (text=="kbb" or text=="planner") and LUI.KBB and LUI.KBB.UI then
                LUI.KBB.UI:Open()
            elseif text=="overlays" and LUI.Overlays then
                LUI:OpenUI();LUI:SelectPage("modules");LUI:SelectModule("overlays")
            elseif text=="kick" and LUI.Kick then
                LUI:OpenUI();LUI:SelectPage("modules");LUI:SelectModule("kick")
            elseif text=="performance" and LUI.Performance then
                LUI:OpenUI();LUI:SelectPage("modules");LUI:SelectModule("performance")
            elseif text=="perf" and LUI.Performance then LUI.Performance:Report()
            elseif text=="tracking" and LUI.Observations then LUI.Observations:Diagnostic()
            elseif text=="debug" and LUI.FrameAuras then LUI.FrameAuras:Diagnostic()
            elseif text=="unlock" then LUI:SetModulesUnlocked(true)
            elseif text=="lock" then LUI:SetModulesUnlocked(false)
            else LUI:OpenUI()end
        end
        if LUI.Performance then LUI.Performance:RefreshSettings()end
    end
    LUI:EnsureLogging();C_Timer.After(11,function()LUI:EnsureLogging()end)
end)
