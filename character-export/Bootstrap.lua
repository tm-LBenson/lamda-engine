local _, E = ...
_G.LamdaExport = E
E.version = "0.1.1"
E.collectors = {}
E.notes = {}
E.noteSet = {}
local unpack = unpack or table.unpack
local function pack(...) return {n = select("#", ...), ...} end

function E.secret(value)
    return type(issecretvalue) == "function" and issecretvalue(value)
end
function E.note(message)
    if not E.noteSet[message] and #E.notes < 64 then
        E.noteSet[message] = true
        E.notes[#E.notes + 1] = message
    end
end
function E.array(value)
    return setmetatable(value or {}, {__lamdaArray = true})
end
function E.clean(value, depth, seen)
    if E.secret(value) then E.readIssues=(E.readIssues or 0)+1;E.note("Restricted game values were omitted."); return nil end
    local kind = type(value)
    if kind == "nil" or kind == "boolean" then return value end
    if kind == "number" then
        if value == value and value ~= math.huge and value ~= -math.huge then return value end
        return nil
    end
    if kind == "string" then return value end
    if kind ~= "table" then return nil end
    depth, seen = depth or 0, seen or {}
    if depth > 12 or seen[value] then E.note("Unsupported nested data was omitted."); return nil end
    seen[value] = true
    local result, count = {}, 0
    for key, item in pairs(value) do
        count = count + 1
        if count > 100000 then E.note("An API list exceeded the export limit."); break end
        if not E.secret(key) and (type(key) == "string" or type(key) == "number") then
            result[key] = E.clean(item, depth + 1, seen)
        end
    end
    seen[value] = nil
    local meta = getmetatable(value)
    if type(meta) == "table" and meta.__lamdaArray then E.array(result) end
    return result
end
function E.resolve(path)
    local value = _G
    for key in path:gmatch("[^.]+") do
        if type(value) ~= "table" then return nil end
        value = value[key]
    end
    return type(value) == "function" and value or nil
end
function E.has(path) return E.resolve(path) ~= nil end
function E.invoke(fn, ...)
    if type(fn) ~= "function" then return nil end
    local values = pack(pcall(fn, ...))
    if not values[1] then E.readIssues=(E.readIssues or 0)+1;E.note("Some game API reads failed; affected data is marked unavailable."); return nil end
    for index = 2, values.n do values[index] = E.clean(values[index]) end
    return unpack(values, 2, values.n)
end
function E.call(path, ...) return E.invoke(E.resolve(path), ...) end
function E.pick(value, keys)
    local result = {}
    if type(value) == "table" then
        for _, key in ipairs(keys) do result[key] = E.clean(value[key]) end
    end
    return result
end
function E.text(value)
    if E.secret(value) or type(value) ~= "string" then return nil end
    local text = value:gsub("|H.-|h(.-)|h", "%1"):gsub("|c%x%x%x%x%x%x%x%x", "")
        :gsub("|r", ""):gsub("|T.-|t", ""):gsub("|A.-|a", ""):gsub("|n", "\n")
    return text
end
function E.count(value, maximum)
    if type(value) ~= "number" or value < 0 then return 0 end
    return math.min(math.floor(value), maximum or 100000)
end
function E.now() return E.call("GetServerTime") or E.call("time") or 0 end
function E.sort(rows, key)
    table.sort(rows, function(a, b)
        local x, y = a[key], b[key]
        if type(x) == "number" and type(y) == "number" then return x < y end
        return tostring(x or "") < tostring(y or "")
    end)
    return rows
end
function E.section(status, fields)
    local result = fields or {}
    result.status = status
    return result
end
function E.unavailable(reason) return E.section("unavailable", {reason = reason}) end
function E.register(name, fn) E.collectors[#E.collectors + 1] = {name = name, collect = fn} end
function E.print(message)
    if DEFAULT_CHAT_FRAME and DEFAULT_CHAT_FRAME.AddMessage then
        DEFAULT_CHAT_FRAME:AddMessage("|cff57d6b1LamdaExport:|r " .. message)
    end
end
