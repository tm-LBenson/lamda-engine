local _, E = ...
local replacements = {['"']='\\"', ['\\']='\\\\', ['\b']='\\b', ['\f']='\\f', ['\n']='\\n', ['\r']='\\r', ['\t']='\\t', ['|']='\\u007c'}
local function quote(value)
    -- Literal pipes can activate WoW text markup in the copy box. JSON's
    -- equivalent Unicode escape preserves the data without native formatting.
    return '"' .. value:gsub('[%z\1-\31\\"|]', function(char)
        return replacements[char] or string.format("\\u%04x", char:byte())
    end) .. '"'
end
function E.json(value)
    local active = {}
    local function encode(v, depth)
        if E.secret(v) then return "null" end
        local kind = type(v)
        if kind == "nil" then return "null" end
        if kind == "boolean" then return v and "true" or "false" end
        if kind == "number" then
            assert(v == v and v ~= math.huge and v ~= -math.huge, "Invalid JSON number")
            return (string.format("%.17g", v):gsub(",", "."))
        end
        if kind == "string" then return quote(v) end
        assert(kind == "table" and not active[v] and depth < 24, "Invalid JSON structure")
        active[v] = true
        local keys, numeric, largest = {}, true, 0
        for key in pairs(v) do
            assert(type(key) == "string" or type(key) == "number", "Invalid JSON key")
            keys[#keys + 1] = key
            if type(key) ~= "number" or key < 1 or key % 1 ~= 0 then numeric = false
            else largest = math.max(largest, key) end
        end
        local meta = getmetatable(v)
        local array = (#keys > 0 and numeric and largest == #keys) or (type(meta) == "table" and meta.__lamdaArray)
        local rows = {}
        if array then
            assert(numeric and largest == #keys, "Sparse JSON array")
            for index = 1, largest do rows[#rows + 1] = encode(v[index], depth + 1) end
            active[v] = nil
            return "[" .. table.concat(rows, ",") .. "]"
        end
        table.sort(keys, function(a,b) return tostring(a) < tostring(b) end)
        local padding, child = string.rep("  ", depth), string.rep("  ", depth + 1)
        for _, key in ipairs(keys) do rows[#rows + 1] = child .. quote(tostring(key)) .. ": " .. encode(v[key], depth + 1) end
        active[v] = nil
        if #rows == 0 then return "{}" end
        return "{\n" .. table.concat(rows, ",\n") .. "\n" .. padding .. "}"
    end
    return encode(value, 0)
end
