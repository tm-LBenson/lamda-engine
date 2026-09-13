local _, Addon = ...

function Addon:GetEditModeAudit()
    local result = {
        available = false,
        matchesProfile = true,
    }

    if not C_EditMode or not C_EditMode.GetLayouts then
        result.status = "Edit Mode API unavailable"
        return result
    end

    local ok, layoutInfo = pcall(C_EditMode.GetLayouts)
    if not ok or not layoutInfo or not layoutInfo.layouts then
        result.status = "Unable to read Edit Mode layouts"
        return result
    end

    result.available = true
    result.activeIndex = layoutInfo.activeLayout
    local activeLayout = result.activeIndex and layoutInfo.layouts[result.activeIndex]
    if not activeLayout and type(result.activeIndex) == "number" then
        activeLayout = layoutInfo.layouts[result.activeIndex + 1]
    end
    result.activeName = activeLayout and activeLayout.layoutName or "Unknown"

    result.status = "Observed"
    return result
end

function Addon:GetActionBarAudit()
    local current = GetCVar and GetCVar("enableMultiActionBars") or nil
    return {
        cvar = "enableMultiActionBars",
        current = current,
        matchesProfile = true,
    }
end

function Addon:GetCastTargetingAudit()
    local function readCVar(name)
        if type(GetCVar) ~= "function" then return nil end
        local ok, value = pcall(GetCVar, name)
        return ok and value or nil
    end
    local function readModifiedClick(name)
        if type(GetModifiedClick) ~= "function" then return nil end
        local ok, value = pcall(GetModifiedClick, name)
        return ok and value or nil
    end
    return {
        enableMouseoverCast = readCVar("enableMouseoverCast"),
        mouseoverCastModifier = readModifiedClick("MOUSEOVERCAST"),
        autoSelfCast = readCVar("autoSelfCast"),
        selfCastModifier = readModifiedClick("SELFCAST"),
        focusCastModifier = readModifiedClick("FOCUSCAST"),
    }
end

function Addon:BuildAudit()
    return {
        editMode = self:GetEditModeAudit(),
        actionBars = self:GetActionBarAudit(),
        castTargeting = self:GetCastTargetingAudit(),
        bindings = self:GetBindingAudit(),
        addons = self:GetAddonAudit(),
    }
end
