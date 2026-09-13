local _, LUI = ...
LUI.modules = {}
local defaults = {schema=1,checkUpdates=true,notifyUpdates=true,checkDays=1}
local limits = {checkDays={values={1,7}}}
local moduleKeys = {}
local function valid(value, default, rule)
    if type(value)~=type(default) then return false end
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
    for key,default in pairs(defaults) do
        if not valid(LamdaEngineDB[key],default,limits[key]) then LamdaEngineDB[key]=default end
    end
    LamdaEngineDB.schema=1
    return LamdaEngineDB
end
function LUI:RegisterModule(module)
    assert(type(module.id)=="string" and type(module.name)=="string" and type(module.build)=="function")
    for _,m in ipairs(self.modules) do assert(m.id~=module.id,"Duplicate module") end
    module.defaults=module.defaults or {}
    if module.enabledKey then
        assert(type(module.enabledKey)=="string","Module enabledKey must be a string")
        if module.defaults[module.enabledKey]==nil then module.defaults[module.enabledKey]=true end
    end
    for key,default in pairs(module.defaults) do
        assert(type(key)=="string" and key:match("^[%a][%w_]*$") and defaults[key]==nil,"Duplicate or invalid setting")
        assert(type(default)=="number" or type(default)=="boolean" or type(default)=="string","Unsupported setting type")
        defaults[key]=default;moduleKeys[key]=true
        limits[key]=module.limits and module.limits[key]
    end
    table.insert(self.modules,module)
end
local function snapshot()
    local values={};local db=LUI:DB()
    for key in pairs(moduleKeys) do values[key]=db[key] end
    return values
end
local function normalized(values)
    local result={}
    for key in pairs(moduleKeys) do
        result[key]=valid(values[key],defaults[key],limits[key]) and values[key] or defaults[key]
        -- Lua's and/or idiom must preserve a valid false value.
        if valid(values[key],defaults[key],limits[key]) then result[key]=values[key] end
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
    for key,value in pairs(normalized(hub.profiles[name])) do self:DB()[key]=value end
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
    for key,value in pairs(normalized(hub.profiles[hub.active])) do self:DB()[key]=value end
    if self.RefreshUI then self:RefreshUI() end
    return true
end
function LUI:ResetModule(id)
    for _,module in ipairs(self.modules) do if module.id==id then
        for key,value in pairs(module.defaults) do self:DB()[key]=value end
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
    for key,value in pairs(hub.profiles[name]) do self:DB()[key]=value end
    if self.RefreshUI then self:RefreshUI() end
    return true
end
-- Legacy SavedVariables stay declared only to preserve existing users' data.
function LUI:Save()
    if InCombatLockdown() then return end
    if self.commitInputs then self.commitInputs() end
    self:SaveActiveProfile();ReloadUI()
end
function LUI:EnsureLogging()
    if not self:DB().autoLog or not self:DB().cdEnabled then return end
    local inside,kind=IsInInstance()
    if not inside or (kind~="party" and kind~="scenario") then return end
    if type(LoggingCombat)=="function" then pcall(LoggingCombat,true) end
end
local events=CreateFrame("Frame")
events:RegisterEvent("PLAYER_LOGIN")
events:RegisterEvent("PLAYER_ENTERING_WORLD")
events:SetScript("OnEvent",function(_,event)
    LUI:DB();LUI:ProfileDB()
    if event=="PLAYER_LOGIN" then
        LUI:BuildUI()
        SLASH_LAMDAUI1="/lui";SLASH_LAMDAUI2="/lamdaui"
        SlashCmdList.LAMDAUI=function()LUI:OpenUI()end
    end
    LUI:EnsureLogging();C_Timer.After(11,function()LUI:EnsureLogging()end)
end)
