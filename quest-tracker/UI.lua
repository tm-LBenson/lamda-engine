local _, ns = ...
local U, M = ns.UI, ns.Model
local dialogs, rows, dialogCache = {}, {}, {}
local managerShield
local theme = {
    panel = { 0.045, 0.055, 0.073, 1 }, input = { 0.025, 0.033, 0.047, 1 },
    border = { 0.19, 0.23, 0.3, 1 }, accent = { 0.86, 0.69, 0.4, 1 },
    muted = { 0.61, 0.67, 0.76, 1 },
}
local function surface(frame, color, border)
    frame:SetBackdrop({ bgFile = "Interface\\Buttons\\WHITE8X8", edgeFile = "Interface\\Buttons\\WHITE8X8", edgeSize = 1 })
    frame:SetBackdropColor(unpack(color or theme.panel))
    frame:SetBackdropBorderColor(unpack(border or theme.border))
end

local function paintButton(control)
    local enabled = control:IsEnabled()
    local active = control.lqlPrimary or control.lqlActive
    control:SetBackdropColor(active and 0.19 or (control.lqlHover and 0.13 or 0.085),
        active and 0.16 or (control.lqlHover and 0.16 or 0.105), active and 0.105 or (control.lqlHover and 0.21 or 0.14), 1)
    control:SetBackdropBorderColor(unpack(active and theme.accent or theme.border))
    control.label:SetTextColor(enabled and 0.93 or 0.42, enabled and 0.95 or 0.47, enabled and 1 or 0.55)
    if control.icon then control.icon:SetAlpha(enabled and 1 or 0.3) end
    if control.chevron then control.chevron:SetAlpha(enabled and 0.85 or 0.3) end
end

local function makeButton(parent)
    local control = CreateFrame("Button", nil, parent, "BackdropTemplate")
    control.lqlStyled = true
    control:EnableMouse(true)
    if control.EnableMouseClicks then control:EnableMouseClicks(true) end
    if control.EnableMouseMotion then control:EnableMouseMotion(true) end
    surface(control)
    control.label = control:CreateFontString(nil, "OVERLAY", "GameFontHighlight")
    control.label:SetPoint("CENTER")
    control.label:SetFontObject(GameFontHighlight)
    control.label:SetFont("Fonts\\ARIALN.TTF", 14, "")
    control:SetFontString(control.label)
    control:RegisterForClicks("LeftButtonUp")
    control:SetScript("OnEnter", function(self) self.lqlHover = true; paintButton(self) end)
    control:SetScript("OnLeave", function(self) self.lqlHover = false; paintButton(self) end)
    return control
end
local labels = {
    zone = "Zone", category = "Custom category", type = "Quest type", none = "No groups",
    log = "Quest log order", title = "Name: A to Z", title_desc = "Name: Z to A",
    level = "Level: high to low", level_asc = "Level: low to high",
    progress = "Most progress first", campaign = "Campaign first",
    completion = "Incomplete first", manual = "Manual order", watched = "Tracked quests", all = "All quests",
    distance = "Closest first",
}

local function acquire(parent, kind, create)
    parent.lqlPools = parent.lqlPools or {}
    parent.lqlUsed = parent.lqlUsed or {}
    parent.lqlControls = parent.lqlControls or {}
    local pool = parent.lqlPools[kind] or {}
    parent.lqlPools[kind] = pool
    local index = (parent.lqlUsed[kind] or 0) + 1
    parent.lqlUsed[kind] = index
    local control = pool[index]
    if not control then
        control = create()
        pool[index] = control
        parent.lqlControls[#parent.lqlControls + 1] = control
    end
    control:ClearAllPoints()
    control:Show()
    return control
end

local function text(parent, template, value, x, y)
    local label = acquire(parent, "text", function() return parent:CreateFontString(nil, "OVERLAY", template or "GameFontHighlight") end)
    label:SetFontObject(_G[template or "GameFontHighlight"] or template or GameFontHighlight)
    label:SetFont("Fonts\\ARIALN.TTF", template and template:find("Large") and 18 or (template and template:find("Small") and 12 or 14), "")
    label:SetPoint("TOPLEFT", x, y)
    label:SetJustifyH("LEFT")
    label:SetText(value or "")
    return label
end

local function button(parent, label, width, x, y, callback)
    local control = acquire(parent, "button", function() return makeButton(parent) end)
    control.lqlHover, control.lqlPrimary, control.lqlActive = false, false, false
    control:SetEnabled(true)
    if control.icon then control.icon:Hide() end
    for _, art in ipairs(control.plus or {}) do art:Hide() end
    if control.chevron then control.chevron:Hide() end
    control.label:ClearAllPoints(); control.label:SetPoint("CENTER")
    control.label:SetWidth(0); control.label:SetJustifyH("CENTER")
    control:SetSize(width, 28)
    control:SetPoint("TOPLEFT", x, y)
    control:SetText(label)
    control:SetScript("OnClick", function() if control:IsEnabled() then callback(control) end end)
    paintButton(control)
    return control
end

local function buttonIcon(control, atlas, fallback, onlyIcon)
    control.icon = control.icon or control:CreateTexture(nil, "ARTWORK")
    local icon = control.icon
    if atlas and C_Texture and C_Texture.GetAtlasInfo and C_Texture.GetAtlasInfo(atlas) then icon:SetAtlas(atlas)
    else icon:SetTexture(fallback) end
    icon:ClearAllPoints(); icon:SetSize(18, 18)
    icon:SetPoint(onlyIcon and "CENTER" or "LEFT", onlyIcon and 0 or 10, 0); icon:Show()
    if not onlyIcon then
        control.label:ClearAllPoints(); control.label:SetPoint("LEFT", 36, 0)
    end
end

local function selectButton(control, selected)
    control.lqlActive = selected; paintButton(control)
end

local function primaryButton(control)
    control.lqlPrimary = true; paintButton(control); return control
end

local function plusIcon(control, onlyIcon)
    control.plus = control.plus or { control:CreateTexture(nil, "ARTWORK"), control:CreateTexture(nil, "ARTWORK") }
    for index, art in ipairs(control.plus) do
        art:ClearAllPoints(); art:SetPoint(onlyIcon and "CENTER" or "LEFT", onlyIcon and 0 or (index == 1 and 13 or 17), 0)
        art:SetSize(index == 1 and 10 or 2, index == 1 and 2 or 10)
        art:SetColorTexture(0.88, 0.76, 0.52, 1); art:Show()
    end
    if not onlyIcon then control.label:ClearAllPoints(); control.label:SetPoint("LEFT", 34, 0) end
end

local function tooltip(owner, title, detail)
    owner:SetScript("OnEnter", function(self)
        if self.lqlStyled then self.lqlHover = true; paintButton(self) end
        if not GameTooltip then return end
        GameTooltip:SetOwner(self, "ANCHOR_LEFT")
        GameTooltip:SetText(title)
        if detail then GameTooltip:AddLine(detail, 1, 1, 1, true) end
        GameTooltip:Show()
    end)
    owner:SetScript("OnLeave", function(self)
        if self.lqlStyled then self.lqlHover = false; paintButton(self) end
        if GameTooltip then GameTooltip:Hide() end
    end)
end

local function dropdown(parent, width, x, y, title, choices, current, choose)
    local control = button(parent, "", width, x, y, function(self)
        local actions = {}
        for _, option in ipairs(choices()) do
            local value = option.value
            actions[#actions + 1] = { label = option.label,
                selected = function() return current() == value end,
                run = function() choose(value); self:Refresh() end }
        end
        U.ActionMenu(self, title, actions)
    end)
    control.label:ClearAllPoints(); control.label:SetPoint("LEFT", 12, 0)
    control.label:SetWidth(width - 38); control.label:SetJustifyH("LEFT"); control.label:SetWordWrap(false)
    control.chevron = control.chevron or control:CreateTexture(nil, "ARTWORK")
    control.chevron:SetTexture("Interface\\ChatFrame\\UI-ChatIcon-ScrollDown-Up")
    control.chevron:ClearAllPoints(); control.chevron:SetSize(14, 14); control.chevron:SetPoint("RIGHT", -10, 0); control.chevron:Show()
    function control:Refresh()
        local options, value, label = choices(), current(), "No categories yet"
        for _, option in ipairs(options) do if option.value == value then label = option.label; break end end
        self:SetText(label); self:SetEnabled(#options > 0); paintButton(self)
    end
    control:SetScript("OnHide", function(self) if U.CloseEntryMenu then U.CloseEntryMenu(self) end end)
    control:Refresh()
    return control
end

-- Small, draggable rails for management forms. The protected tracker keeps its native scrollbar.
local function scrollRail(scroll)
    if scroll.lqlRail then return scroll.lqlRail end
    local rail = CreateFrame("Button", nil, scroll:GetParent())
    rail:EnableMouse(true)
    rail:SetWidth(6); rail:SetPoint("TOPLEFT", scroll, "TOPRIGHT", 8, 0); rail:SetPoint("BOTTOMLEFT", scroll, "BOTTOMRIGHT", 8, 0)
    local track = rail:CreateTexture(nil, "BACKGROUND"); track:SetAllPoints(); track:SetColorTexture(0.1, 0.13, 0.18, 1)
    local thumb = CreateFrame("Button", nil, rail); thumb:SetWidth(6)
    thumb:EnableMouse(true)
    local art = thumb:CreateTexture(nil, "ARTWORK"); art:SetAllPoints(); art:SetColorTexture(0.49, 0.55, 0.65, 1)
    rail.thumb, scroll.lqlRail = thumb, rail
    function rail:Sync()
        local range, height = scroll:GetVerticalScrollRange(), scroll:GetHeight()
        self:SetShown(range > 0 and height > 0)
        if range <= 0 or height <= 0 then return end
        local size = math.min(height, math.max(24, height * height / (height + range)))
        thumb:SetHeight(size); thumb:ClearAllPoints()
        thumb:SetPoint("TOPLEFT", 0, -(height - size) * scroll:GetVerticalScroll() / range)
    end
    local function move(grab)
        local _, y = GetCursorPosition()
        local height = scroll:GetHeight() - thumb:GetHeight()
        local offset = rail:GetTop() - y / rail:GetEffectiveScale() - grab
        if height > 0 then scroll:SetVerticalScroll(math.max(0, math.min(1, offset / height)) * scroll:GetVerticalScrollRange()) end
    end
    rail:SetScript("OnMouseDown", function(_, mouse) if mouse == "LeftButton" then move(thumb:GetHeight() / 2) end end)
    thumb:SetScript("OnMouseDown", function(_, mouse)
        if mouse ~= "LeftButton" then return end
        local _, y = GetCursorPosition()
        local grab = thumb:GetTop() - y / thumb:GetEffectiveScale()
        rail:SetScript("OnUpdate", function(self)
            if IsMouseButtonDown and not IsMouseButtonDown("LeftButton") then self:SetScript("OnUpdate", nil); return end
            move(grab)
        end)
    end)
    thumb:SetScript("OnMouseUp", function() rail:SetScript("OnUpdate", nil) end)
    rail:SetScript("OnHide", function(self) self:SetScript("OnUpdate", nil) end)
    scroll:HookScript("OnVerticalScroll", function() rail:Sync() end)
    scroll:HookScript("OnScrollRangeChanged", function() rail:Sync() end)
    scroll:HookScript("OnSizeChanged", function() rail:Sync() end)
    scroll:HookScript("OnShow", function() rail:Sync() end)
    rail:Sync()
    return rail
end

local function hasAtlas(name)
    return C_Texture and C_Texture.GetAtlasInfo and C_Texture.GetAtlasInfo(name) ~= nil
end

local function iconArt(texture, atlas, fallback)
    texture:SetTexCoord(0, 1, 0, 1)
    if hasAtlas(atlas) then texture:SetAtlas(atlas)
    else texture:SetTexture(fallback) end
end

-- Header controls have their own click targets above the drag handle and native UI.
local function iconButton(parent, atlas, fallback, callback, kind, name)
    local control = CreateFrame(kind or "Button", name, parent)
    control:SetSize(24, 24)
    control:SetFrameStrata("MEDIUM")
    control:SetFrameLevel(82)
    control:EnableMouse(true)
    if control.EnableMouseClicks then control:EnableMouseClicks(true) end
    if control.EnableMouseMotion then control:EnableMouseMotion(true) end
    control:RegisterForClicks("LeftButtonUp")
    control:SetHighlightTexture("Interface\\Buttons\\ButtonHilight-Square", "ADD")
    control.icon = control:CreateTexture(nil, "ARTWORK")
    control.icon:SetSize(18, 18)
    control.icon:SetPoint("CENTER")
    if atlas then iconArt(control.icon, atlas, fallback) end
    control.activeGlow = control:CreateTexture(nil, "BACKGROUND")
    control.activeGlow:SetAllPoints()
    control.activeGlow:SetTexture("Interface\\Buttons\\ButtonHilight-Square")
    control.activeGlow:SetBlendMode("ADD")
    control.activeGlow:SetAlpha(0.55)
    control.activeGlow:Hide()
    control:SetScript("OnClick", function(self) callback(self) end)
    control:SetScript("OnMouseDown", function(self) self.icon:SetPoint("CENTER", 1, -1) end)
    control:SetScript("OnMouseUp", function(self) self.icon:SetPoint("CENTER") end)
    return control
end

local function clearQuestHover(owner)
    if owner.highlightedQuestID then
        ns.Compat.ClearQuestHighlight(owner.highlightedQuestID, owner)
        owner.highlightedQuestID = nil
    end
    if GameTooltip and GameTooltip.GetOwner and GameTooltip:GetOwner() == owner then GameTooltip:Hide() end
end

local function questHover(owner, entry, title, detail)
    owner:EnableMouse(true)
    if owner.EnableMouseMotion then owner:EnableMouseMotion(true) end
    tooltip(owner, title, detail)
    local enter, leave = owner:GetScript("OnEnter"), owner:GetScript("OnLeave")
    owner:SetScript("OnEnter", function(self)
        if entry.questID and not entry.custom and not entry.recipe then
            self.highlightedQuestID = entry.questID
            ns.Compat.HighlightQuest(entry.questID, self)
        end
        if enter then enter(self) end
    end)
    owner:SetScript("OnLeave", function(self) clearQuestHover(self); if leave then leave(self) end end)
    owner:SetScript("OnHide", function(self)
        clearQuestHover(self)
        if U.CloseEntryMenu then U.CloseEntryMenu(self) end
    end)
end

local function syncDialogShield()
    local modal = U.activeModal
    if not modal or not modal:IsShown() or not U.options or not U.options:IsShown() then
        if managerShield then managerShield:Hide() end
        return
    end
    if not managerShield then
        managerShield = CreateFrame("Frame", nil, UIParent)
        managerShield:SetFrameStrata("FULLSCREEN_DIALOG")
        managerShield:SetFrameLevel(200)
        managerShield:EnableMouse(true)
        managerShield:EnableMouseWheel(true)
        managerShield:SetScript("OnMouseWheel", function() end)
        local shade = managerShield:CreateTexture(nil, "BACKGROUND")
        shade:SetAllPoints()
        shade:SetColorTexture(0, 0, 0, 0.45)
        U.managerShield = managerShield
    end
    managerShield:ClearAllPoints()
    managerShield:SetAllPoints(U.options)
    managerShield:Show()
    modal:SetFrameLevel(210)
end

local function dialog(name, title, width, height)
    if U.CloseEntryMenu then U.CloseEntryMenu() end
    local frame = dialogCache[name]
    local modal = name ~= "LamdaQuestLogOptions"
    if not frame then
        frame = CreateFrame("Frame", name, UIParent, "BackdropTemplate")
        frame:Hide()
        dialogCache[name] = frame
        local close = makeButton(frame)
        close:SetSize(28, 28); close:SetText("\195\151"); paintButton(close)
        close:SetPoint("TOPRIGHT", -14, -14)
        close:SetScript("OnClick", function() frame:Hide() end)
        tooltip(close, "Close")
        frame.close = close
        local header = frame:CreateTexture(nil, "BACKGROUND")
        header:SetPoint("TOPLEFT", 1, -1); header:SetPoint("TOPRIGHT", -1, -1)
        header:SetHeight(58); header:SetColorTexture(0.067, 0.081, 0.108, 1)
        local footer = frame:CreateTexture(nil, "BACKGROUND")
        footer:SetPoint("BOTTOMLEFT", 1, 1); footer:SetPoint("BOTTOMRIGHT", -1, 1)
        footer:SetHeight(54); footer:SetColorTexture(0.055, 0.066, 0.089, 1)
        if UISpecialFrames then UISpecialFrames[#UISpecialFrames + 1] = name end
        dialogs[#dialogs + 1] = frame
        frame:SetScript("OnShow", function(self)
            if modal then
                if U.activeModal and U.activeModal ~= self then U.activeModal:Hide() end
                U.activeModal = self
            end
            syncDialogShield()
        end)
        frame:SetScript("OnHide", function(self)
            if U.activeModal == self then U.activeModal = nil end
            syncDialogShield()
            local owner = self.lqlReturnTo
            self.lqlReturnTo = nil
            if owner and not U.closingDialogs then owner:Show(); owner:Raise() end
        end)
    end
    for _, control in ipairs(frame.lqlControls or {}) do
        control:Hide()
        if control.lqlBorder then control.lqlBorder:Hide() end
    end
    frame.lqlUsed = {}
    frame:SetSize(width, height)
    frame:ClearAllPoints(); frame:SetPoint("CENTER")
    frame:SetFrameStrata(modal and "FULLSCREEN_DIALOG" or "DIALOG")
    frame:SetToplevel(true)
    frame:SetAlpha(1)
    frame:SetClampedToScreen(true)
    frame:EnableMouse(true)
    frame:SetMovable(true)
    frame:RegisterForDrag("LeftButton")
    frame:SetScript("OnDragStart", function(self) self:StartMoving() end)
    frame:SetScript("OnDragStop", function(self) self:StopMovingOrSizing() end)
    surface(frame)
    frame:Raise()
    frame:SetFrameLevel(modal and 210 or 100)
    local heading = text(frame, "GameFontNormalLarge", title, 24, -20)
    heading:SetTextColor(0.94, 0.96, 1)
    return frame
end

local function field(parent, label, y, width, multiline, height)
    local caption = text(parent, "GameFontHighlight", label, 24, y)
    caption:SetTextColor(0.7, 0.76, 0.85)
    local input = acquire(parent, multiline and "multiline" or "input", function()
        return CreateFrame("EditBox", nil, parent)
    end)
    input:SetFontObject(GameFontHighlight)
    input:SetFont("Fonts\\ARIALN.TTF", 14, "")
    input:SetTextColor(0.94, 0.96, 1)
    if input.SetBlinkSpeed then input:SetBlinkSpeed(0.5) end
    input:SetAutoFocus(false)
    input:SetMultiLine(multiline == true)
    input:SetMaxLetters(multiline and 4000 or 1000)
    input:SetScript("OnEscapePressed", function(self) self:ClearFocus() end)
    input:SetScript("OnHide", function(self) self:ClearFocus() end)
    input:EnableMouse(true); input:EnableKeyboard(true)
    input:SetScript("OnEditFocusGained", function(self)
        self.lqlBorder:SetBackdropBorderColor(unpack(theme.accent))
    end)
    input:SetScript("OnEditFocusLost", function(self)
        self.lqlBorder:SetBackdropBorderColor(unpack(theme.border))
    end)
    local border = input.lqlBorder or CreateFrame("Frame", nil, parent, "BackdropTemplate")
    input.lqlBorder = border
    border:ClearAllPoints(); border:Show()
    surface(border, theme.input)
    border:SetFrameLevel(parent:GetFrameLevel() + 1)
    if multiline then
        local viewportHeight = height or 104
        border:SetSize(width, viewportHeight)
        border:SetPoint("TOPLEFT", 24, y - 20)
        local scroll = input.lqlScroll or CreateFrame("ScrollFrame", nil, border)
        input.lqlScroll = scroll
        scroll:ClearAllPoints()
        scroll:SetFrameLevel(border:GetFrameLevel() + 1)
        scroll:SetPoint("TOPLEFT", 12, -8)
        scroll:SetPoint("BOTTOMRIGHT", -24, 8)
        input:SetParent(scroll)
        input:SetFrameLevel(scroll:GetFrameLevel() + 1)
        input:SetPoint("TOPLEFT", 0, 0)
        input:SetWidth(width - 36)
        input:SetHeight(viewportHeight - 16)
        input:EnableMouse(true)
        if input.EnableMouseClicks then input:EnableMouseClicks(true) end
        input:EnableKeyboard(true)
        -- Keep the native caret inside the scroll viewport at the first column.
        input:SetTextInsets(2, 2, 2, 2)
        input:SetJustifyH("LEFT"); input:SetJustifyV("TOP")
        if input.SetAltArrowKeyMode then input:SetAltArrowKeyMode(false) end
        input:SetScript("OnEnterPressed", nil)
        input:SetScript("OnMouseDown", function(self) self:SetFocus() end)
        input:SetScript("OnHide", function(self) self:ClearFocus() end)
        local function focusNotes() input:SetFocus() end
        border:EnableMouse(true); border:SetScript("OnMouseDown", focusNotes)
        scroll:EnableMouse(true); scroll:SetScript("OnMouseDown", focusNotes)
        local function scrollNotes(_, delta)
            scroll:SetVerticalScroll(math.max(0, math.min(scroll:GetVerticalScrollRange(), scroll:GetVerticalScroll() - delta * 42)))
        end
        for _, control in ipairs({ border, scroll, input }) do
            control:EnableMouseWheel(true); control:SetScript("OnMouseWheel", scrollNotes)
        end
        scroll:SetScrollChild(input)
        input.lqlMeasure = input.lqlMeasure or border:CreateFontString(nil, "OVERLAY", "GameFontHighlight")
        input.lqlMeasure:SetFontObject(GameFontHighlight)
        input.lqlMeasure:SetFont("Fonts\\ARIALN.TTF", 14, "")
        input.lqlMeasure:SetWidth(width - 40)
        input.lqlMeasure:Hide()
        input:SetScript("OnTextChanged", function(self)
            self.lqlMeasure:SetText(self:GetText())
            self:SetHeight(math.max(viewportHeight - 16, self.lqlMeasure:GetStringHeight() + 20))
            scroll:UpdateScrollChildRect()
            scroll:SetVerticalScroll(math.min(scroll:GetVerticalScroll(), scroll:GetVerticalScrollRange()))
            scroll.lqlRail:Sync()
        end)
        input:SetScript("OnCursorChanged", function(self, _, offset, _, cursorHeight)
            local top = -offset
            local current, target = scroll:GetVerticalScroll(), scroll:GetVerticalScroll()
            if top < current then target = top
            elseif top + cursorHeight > current + scroll:GetHeight() then
                target = top + cursorHeight - scroll:GetHeight()
            end
            scroll:SetVerticalScroll(math.max(0, math.min(scroll:GetVerticalScrollRange(), target)))
        end)
        scrollRail(scroll)
    else
        border:SetSize(width, 30); border:SetPoint("TOPLEFT", 24, y - 20)
        input:SetFrameLevel(border:GetFrameLevel() + 1)
        input:SetSize(width - 24, 30)
        input:SetPoint("TOPLEFT", 36, y - 20)
        input:SetTextInsets(0, 0, 0, 0)
        input:SetScript("OnMouseDown", function(self) self:SetFocus() end)
        border:EnableMouse(true); border:SetScript("OnMouseDown", function() input:SetFocus() end)
        input:SetScript("OnEnterPressed", function(self) self:ClearFocus() end)
    end
    return input
end

function U.CloseDialogs()
    U.closingDialogs = true
    if U.CloseEntryMenu then U.CloseEntryMenu() end
    for _, frame in ipairs(dialogs) do frame:StopMovingOrSizing(); frame:Hide() end
    U.closingDialogs = false
end

function U.EditEntry(entry)
    if U.editor then U.editor:Hide() end
    local custom = not entry or entry.custom
    local frame = dialog("LamdaQuestLogEditor", custom and (entry and "Edit personal task" or "Add personal task") or (entry.recipe and "Recipe category" or "Quest category"), 500, custom and 654 or 266)
    U.editor = frame
    local record = entry and M.Meta(ns.db, entry) or {}
    local category = record.category
    local titleInput, zoneInput, notesInput, commandInput, macroInput, completed
    if custom then
        titleInput = field(frame, "Title", -74, 452)
        titleInput:SetText(record.title or "")
        zoneInput = field(frame, "Zone (used when grouping by zone)", -136, 452)
        zoneInput:SetText(record.zone or (GetRealZoneText and GetRealZoneText()) or "")
    else
        local label = text(frame, "GameFontHighlight", entry.title, 24, -78)
        label:SetWidth(452)
    end
    text(frame, "GameFontHighlight", "Custom category", 24, custom and -198 or -112)
    local categoryButton = dropdown(frame, 402, 24, custom and -218 or -132, "Choose category", function()
        if #ns.db.categories == 0 then return {} end
        local choices = { { label = "Uncategorized" } }
        for _, value in ipairs(ns.db.categories) do choices[#choices + 1] = { label = value.name, value = value.id } end
        return choices
    end, function() return category end, function(value) category = value end)
    categoryButton:SetHeight(30)
    tooltip(categoryButton, "Choose category", "Choose from the list. Use the + button to create a category without losing this draft.")
    local addCategory = button(frame, "", 38, 438, custom and -218 or -132, function()
        U.CategoryEditor(nil, function(id)
            category = id; categoryButton:Refresh()
        end, frame)
    end)
    plusIcon(addCategory, true)
    addCategory:SetHeight(30)
    tooltip(addCategory, "New category", "Create a category and select it for this task. Your unsaved fields are kept.")
    frame.categoryDropdown, frame.addCategory = categoryButton, addCategory
    if custom then
        notesInput = field(frame, "Notes / objectives", -262, 452, true)
        notesInput:SetText(record.notes or "")
        commandInput = field(frame, "Waypoint command (one /way line)", -402, 452)
        commandInput:SetText(record.command or "")
        tooltip(commandInput, "Waypoint", "/way #mapID x y label, or /way x y label for your current zone. Zone names use your installed /way provider.")
        macroInput = field(frame, "Existing WoW macro name (optional)", -468, 452)
        macroInput:SetText(record.macro or "")
        completed = acquire(frame, "check", function() return CreateFrame("CheckButton", nil, frame, "UICheckButtonTemplate") end)
        completed:SetSize(24, 24)
        completed:SetPoint("TOPLEFT", 21, -534)
        completed:SetChecked(record.completed == true)
        text(frame, "GameFontHighlight", "Completed", 49, -540)
        local inputs = { titleInput, zoneInput, notesInput, commandInput, macroInput }
        for index, input in ipairs(inputs) do
            input:SetScript("OnTabPressed", function()
                local delta = IsShiftKeyDown and IsShiftKeyDown() and -1 or 1
                inputs[(index - 1 + delta) % #inputs + 1]:SetFocus()
            end)
        end
    end
    local errorText = text(frame, "GameFontRedSmall", "", 24, custom and -570 or -178)
    errorText:SetWidth(452)
    primaryButton(button(frame, "Save", 100, 376, custom and -610 or -222, function()
        if custom then
            local macro = M.Trim(macroInput:GetText())
            if macro ~= "" and (not GetMacroIndexByName or GetMacroIndexByName(macro) == 0) then
                errorText:SetText("Create that macro in /macro first, or leave its name empty.")
                return
            end
            local command = M.Trim(commandInput:GetText())
            if command ~= "" and (command:find("[\r\n]") or not command:lower():match("^/way%s") and not command:lower():match("^/waypoint%s")) then
                errorText:SetText("Use one /way command here. Other commands belong in an existing macro.")
                return
            end
            -- Resolve current-zone coordinates when saving so travel cannot retarget the task.
            local point = ns.Navigation.Parse(command, ns.Navigation.GetCurrentMap())
            if point then command = string.format("/way #%d %.6f %.6f %s", point.mapID, point.x * 100, point.y * 100, point.title) end
            local id, err = M.SaveCustom(ns.db, entry and entry.id, {
                title = titleInput:GetText(), zone = zoneInput:GetText(), notes = notesInput:GetText(),
                command = command, macro = macro, category = category, completed = completed:GetChecked(),
            })
            if not id then errorText:SetText(err); return end
        else
            record.category = category
        end
        frame:Hide()
        ns.RequestRefresh()
    end))
    button(frame, "Cancel", 100, 264, custom and -610 or -222, function() frame:Hide() end)
    frame:Show()
end

function U.CategoryEditor(id, onSaved, owner)
    local frame = dialog("LamdaQuestLogCategory", id and "Rename category" or "New category", 420, 220)
    U.categoryEditor = frame
    frame.lqlReturnTo = owner
    local input = field(frame, "Category name", -78, 372)
    input:SetText(id and M.CategoryName(ns.db, id) or "")
    local errorText = text(frame, "GameFontRedSmall", "", 24, -136)
    errorText:SetWidth(372)
    primaryButton(button(frame, "Save", 92, 304, -176, function()
        local saved, err = M.SaveCategory(ns.db, id, input:GetText())
        if not saved then errorText:SetText(err); return end
        if onSaved then onSaved(saved) end
        frame:Hide()
        if not owner and U.options then U.Options("categories") end
        ns.RequestRefresh()
    end))
    button(frame, "Cancel", 92, 200, -176, function() frame:Hide() end)
    frame:Show()
    input:SetFocus()
end

function U.Context(entry, group, owner)
    U.EntryMenu(owner or U.frame, entry, group)
end

local function savePosition(frame)
    local point, _, relativePoint, x, y = frame:GetPoint(1)
    ns.db.settings.position = { point = point, relativePoint = relativePoint, x = x, y = y }
    ns.db.settings.width = frame:GetWidth()
    if not ns.db.settings.collapsed then ns.db.settings.height = frame:GetHeight() end
end

function U.StopMotion()
    if not U.motion then return end
    savePosition(U.frame)
    U.motion, U.moving = nil, false
    ns.dirty = true
    if not ns.InCombat() then ns.RequestRefresh() end
end

local function startMotion(mode)
    if ns.InCombat() or ns.db.settings.locked then return end
    local scale = UIParent:GetEffectiveScale()
    local x, y = GetCursorPosition()
    U.motion = { mode = mode, x = x / scale, y = y / scale,
        left = U.frame:GetLeft(), bottom = U.frame:GetBottom(),
        width = U.frame:GetWidth(), height = U.frame:GetHeight() }
    U.moving = true
end

-- Manual cursor following stops without protected-frame calls when combat starts.
-- Native StartMoving would keep following the cursor until an allowed StopMoving.
function U.UpdateMotion()
    local motion = U.motion
    if not motion then return end
    if ns.InCombat() then U.StopMotion(); return end
    local x, y = GetCursorPosition()
    local scale = UIParent:GetEffectiveScale()
    local dx, dy = x / scale - motion.x, y / scale - motion.y
    local frame = U.frame
    frame:ClearAllPoints()
    if motion.mode == "resize" then
        frame:SetPoint("TOPLEFT", UIParent, "BOTTOMLEFT", motion.left, motion.bottom + motion.height)
        frame:SetSize(math.max(240, math.min(600, motion.width + dx)), math.max(180, math.min(1000, motion.height - dy)))
    else
        frame:SetPoint("BOTTOMLEFT", UIParent, "BOTTOMLEFT", motion.left + dx, motion.bottom + dy)
    end
end

function U.SetLocked(locked)
    U.StopMotion()
    ns.db.settings.locked = locked == true
    U.UpdateLockButton()
    ns.RequestRefresh()
end

function U.UpdateLockButton()
    local control = U.lock
    if not control or not ns.db then return end
    local locked = ns.db.settings.locked
    control:SetChecked(locked)
    if control.lockState == locked then return end
    control.lockState = locked
    control.icon:SetTexture(locked and "Interface\\Buttons\\LockButton-Locked-Up" or "Interface\\Buttons\\LockButton-Unlocked-Up")
    tooltip(control, locked and "Locked - click to unlock" or "Unlocked - click to lock",
        locked and "Unlock to drag the header or resize the lower-right corner while out of combat." or "Drag the header to move, or the lower-right corner to resize, while out of combat. Click this padlock when finished.")
    if GameTooltip and GameTooltip.GetOwner and GameTooltip:GetOwner() == control then
        control:GetScript("OnEnter")(control)
    end
end

function U.SetQuickSort(sort)
    local valid
    for _, mode in ipairs(M.quickSorts) do if mode == sort then valid = true; break end end
    if not valid then return end
    if sort == "distance" and ns.db.settings.quickSort ~= "distance" and not ns.Compat.HasQuestDistance() then
        ns.Print("Quest distances are unavailable on this client."); return
    end
    ns.db.settings.quickSort = sort
    U.UpdateQuickControls()
    ns.RequestRefresh()
end

function U.ToggleTurnInSort()
    ns.db.settings.quickTurnIn = not ns.db.settings.quickTurnIn
    U.UpdateQuickControls()
    ns.RequestRefresh()
end

function U.RestoreQuickSort()
    ns.db.settings.quickSort, ns.db.settings.quickTurnIn = "", false
    U.UpdateQuickControls()
    ns.RequestRefresh()
end

function U.UpdateQuickControls()
    if not ns.db then return end
    if U.nearButton then
        local settings = ns.db.settings
        local _, sort = M.View(settings)
        U.nearButton.activeGlow:SetShown(M.QuickSorting(settings))
        tooltip(U.nearButton, "Quick sort",
            "Order: " .. (labels[sort] or sort) .. ".\nReady for turn-in first: " .. (settings.quickTurnIn and "On" or "Off") .. ".\nChoose distance, level, name, progress or campaign order. Turn-in priority combines with any order. Your filters still apply. Layout changes apply after combat.")
    end
    if U.filterButton then
        local count = M.FilterCount(ns.db)
        U.filterButton.activeGlow:SetShown(count > 0)
        tooltip(U.filterButton, count > 0 and ("Quest filters (" .. count .. " exclusions)") or "Quest filters",
            "Choose quest types, zones and custom categories from your current log. Uncheck Dungeon or a zone to hide those entries. Settings are available during combat; tracker layout updates afterward.")
    end
    if U.collapse then
        local collapsed = ns.db.settings.collapsed
        iconArt(U.collapse.icon, collapsed and "common-button-collapseexpand-down" or "common-button-collapseexpand-up",
            collapsed and "Interface\\Buttons\\UI-PlusButton-UP" or "Interface\\Buttons\\UI-MinusButton-UP")
        tooltip(U.collapse, collapsed and "Expand tracker" or "Collapse tracker", "The header controls stay available. Tracker layout changes apply after combat.")
    end
end

function U.RequestTrackerScroll(offset)
    local scroll = U.scroll
    offset = math.max(0, math.min(scroll:GetVerticalScrollRange(), offset))
    if ns.InCombat() then
        U.pendingScroll = offset
        ns.dirty = true
        return
    end
    U.pendingScroll = nil
    scroll:SetVerticalScroll(offset)
end

local function createTracker()
    local frame = CreateFrame("Frame", "LamdaQuestLogTracker", UIParent)
    U.frame = frame
    frame:SetFrameStrata("LOW")
    frame:SetClampedToScreen(true)
    frame:SetMovable(true)
    frame:SetResizable(true)
    frame:SetScript("OnUpdate", U.UpdateMotion)
    U.background = frame:CreateTexture(nil, "BACKGROUND")
    U.background:SetAllPoints()
    if frame.SetResizeBounds then frame:SetResizeBounds(240, 180, 600, 1000) end
    local chrome = CreateFrame("Frame", "LamdaQuestLogHeader", UIParent)
    U.chrome = chrome
    chrome:SetFrameStrata("MEDIUM")
    chrome:SetFrameLevel(80)
    chrome:SetPoint("TOPLEFT", frame, "TOPLEFT")
    chrome:SetPoint("TOPRIGHT", frame, "TOPRIGHT")
    chrome:SetHeight(56)
    local drag = CreateFrame("Button", nil, chrome)
    U.drag = drag
    drag:SetFrameLevel(81)
    drag:SetPoint("TOPLEFT")
    drag:SetPoint("TOPRIGHT", -34, 0)
    drag:SetHeight(28)
    drag:EnableMouse(true)
    if drag.EnableMouseClicks then drag:EnableMouseClicks(true) end
    drag:RegisterForDrag("LeftButton")
    drag:RegisterForClicks("RightButtonUp")
    drag:SetScript("OnClick", function() U.Options() end)
    drag:SetScript("OnDragStart", function()
        startMotion("move")
    end)
    drag:SetScript("OnDragStop", function()
        U.StopMotion()
    end)
    local background = drag:CreateTexture(nil, "BACKGROUND")
    background:SetPoint("LEFT", 0, 0)
    background:SetSize(260, 26)
    local native = QuestObjectiveTracker and QuestObjectiveTracker.Header and QuestObjectiveTracker.Header.Background
    local atlas = native and native.GetAtlas and native:GetAtlas()
    if not atlas and C_Texture and C_Texture.GetAtlasInfo and C_Texture.GetAtlasInfo("UI-QuestTracker-Secondary-Objective-Header") then
        atlas = "UI-QuestTracker-Secondary-Objective-Header"
    end
    if atlas then background:SetAtlas(atlas, true)
    elseif native and native:GetTexture() then
        background:SetTexture(native:GetTexture()); background:SetTexCoord(native:GetTexCoord())
    end
    U.header = text(drag, "GameFontNormal", "Quests", 7, -6)
    ns.Compat.CopyNativeStyle(U.header, "header")
    U.header:SetWordWrap(false)
    local divider = chrome:CreateTexture(nil, "BACKGROUND")
    divider:SetPoint("TOPLEFT", 7, -27)
    divider:SetPoint("TOPRIGHT", -5, -27)
    divider:SetHeight(1)
    divider:SetColorTexture(0.8, 0.65, 0.3, 0.35)
    local collapse = iconButton(chrome, nil, nil, function()
        ns.db.settings.collapsed = not ns.db.settings.collapsed; ns.RequestRefresh()
    end)
    collapse:ClearAllPoints(); collapse:SetPoint("TOPRIGHT", 0, -2)
    U.collapse = collapse
    U.quickBar = CreateFrame("Frame", nil, chrome)
    U.quickBar:SetPoint("TOPLEFT", 4, -28)
    U.quickBar:SetPoint("TOPRIGHT", -2, -28)
    U.quickBar:SetHeight(24)
    U.nearButton = iconButton(U.quickBar, "Waypoint-MapPin-Untracked", "Interface\\Minimap\\Tracking\\None", function() U.SortMenu() end)
    U.nearButton:SetPoint("TOPLEFT")
    U.nearButton:SetScript("OnHide", function(self) if U.CloseEntryMenu then U.CloseEntryMenu(self) end end)
    U.filterButton = iconButton(U.quickBar, "ui-questtrackerbutton-filter", "Interface\\Icons\\INV_Misc_Spyglass_03", function() U.Filters() end)
    U.filterButton:SetPoint("LEFT", U.nearButton, "RIGHT", 4, 0)
    U.manageButton = iconButton(UIParent, "questlog-icon-setting", "Interface\\Buttons\\UI-OptionsButton", function() U.Options() end)
    U.manageButton:SetPoint("TOPRIGHT", U.quickBar, "TOPRIGHT")
    tooltip(U.manageButton, "Manage Lamda Quest Log", "Quests & Tasks, Categories, Settings and Appearance. Settings are available during combat.")
    U.lock = iconButton(U.quickBar, nil, nil, function()
        U.SetLocked(not ns.db.settings.locked)
    end, "CheckButton", "LamdaQuestLogLockButton")
    U.lock:SetPoint("TOPRIGHT", -28, 0)
    U.lock.icon:SetTexCoord(0.1875, 0.78125, 0.21875, 0.78125)
    U.UpdateLockButton()
    U.addButton = iconButton(U.quickBar, "communities-icon-addchannelplus", "Interface\\Buttons\\UI-PlusButton-UP", function() U.EditEntry() end)
    U.addButton:SetPoint("TOPRIGHT", -56, 0)
    tooltip(U.addButton, "Add personal task", "Create a task with notes, a category, a waypoint and an existing macro. Available during combat.")
    local scroll = CreateFrame("ScrollFrame", "LamdaQuestLogScroll", frame, "UIPanelScrollFrameTemplate")
    scroll:SetPoint("TOPLEFT", 0, -58)
    scroll:SetPoint("BOTTOMRIGHT", -25, 16)
    scroll:EnableMouseWheel(true)
    scroll:HookScript("OnVerticalScroll", function() if U.CloseEntryMenu then U.CloseEntryMenu() end end)
    scroll:SetScript("OnMouseWheel", function(self, delta)
        U.RequestTrackerScroll((U.pendingScroll or self:GetVerticalScroll()) - delta * 42)
    end)
    local bar = scroll.ScrollBar or _G[(scroll:GetName() or "") .. "ScrollBar"]
    if bar then
        local changed = bar:GetScript("OnValueChanged")
        bar:SetScript("OnValueChanged", function(self, value, ...)
            -- Native arrow/slider callbacks otherwise bypass the wheel guard.
            if ns.InCombat() or not changed then U.RequestTrackerScroll(value); return end
            return changed(self, value, ...)
        end)
    end
    local content = CreateFrame("Frame", nil, scroll)
    content:SetSize(270, 1)
    scroll:SetScrollChild(content)
    U.scroll, U.content = scroll, content
    local resize = CreateFrame("Button", nil, frame)
    resize:SetSize(16, 16)
    resize:SetPoint("BOTTOMRIGHT")
    resize:SetNormalTexture("Interface\\ChatFrame\\UI-ChatIM-SizeGrabber-Up")
    resize:SetHighlightTexture("Interface\\ChatFrame\\UI-ChatIM-SizeGrabber-Highlight")
    resize:SetScript("OnMouseDown", function()
        startMotion("resize")
    end)
    resize:SetScript("OnMouseUp", function()
        U.StopMotion()
    end)
    U.resize = resize
    U.empty = text(content, "GameFontHighlightSmall", "", 8, -10)
    U.empty:SetWidth(240)
end

function U.CreateStatusMarker(parent, x, y)
    local marker = CreateFrame("Button", nil, parent)
    marker:SetSize(20, 20)
    marker:SetPoint("TOPLEFT", x, y)
    marker:RegisterForClicks("LeftButtonUp")
    marker:EnableMouse(true)
    if marker.EnableMouseMotion then marker:EnableMouseMotion(true) end
    if marker.SetHitRectInsets then marker:SetHitRectInsets(-6, -6, -6, -6) end
    marker.ring = marker:CreateTexture(nil, "BACKGROUND")
    marker.ring:SetPoint("CENTER")
    if hasAtlas("UI-QuestPoi-QuestNumber") then
        marker.ring:SetAtlas("UI-QuestPoi-QuestNumber", true)
    else
        marker.ring:SetTexture("Interface\\WorldMap\\UI-QuestPoi-NumberIcons")
        marker.ring:SetTexCoord(0.875, 1, 0.875, 1)
        marker.ring:SetSize(32, 32)
    end
    marker.icon = marker:CreateTexture(nil, "ARTWORK")
    marker.icon:SetPoint("CENTER")
    marker.label = marker:CreateFontString(nil, "OVERLAY", "GameFontNormal")
    marker.label:SetPoint("CENTER")
    marker.label:SetSize(18, 18)
    marker.label:SetJustifyH("CENTER")
    marker.label:SetJustifyV("MIDDLE")
    marker.hover = marker:CreateTexture(nil, "OVERLAY")
    marker.hover:SetPoint("CENTER")
    if hasAtlas("UI-QuestPoi-InnerGlow") then marker.hover:SetAtlas("UI-QuestPoi-InnerGlow", true)
    else
        marker.hover:SetTexture("Interface\\WorldMap\\UI-QuestPoi-NumberIcons")
        marker.hover:SetTexCoord(0.625, 0.75, 0.375, 0.5)
        marker.hover:SetSize(32, 32)
    end
    marker.hover:Hide()
    if marker.hover.SetBlendMode then marker.hover:SetBlendMode("ADD") end
    marker:SetScript("OnClick", function(self, mouse)
        if mouse == "LeftButton" then ns.Compat.FocusQuest(self.entry) end
    end)
    marker:SetScript("OnEnter", function(self)
        local entry = self.entry
        if not entry or not entry.questID then return end
        self.highlightedQuestID = entry.questID
        ns.Compat.HighlightQuest(entry.questID, self)
        self.hover:Show()
        if GameTooltip then
            GameTooltip:SetOwner(self, "ANCHOR_LEFT")
            GameTooltip:SetText(entry.title)
            GameTooltip:AddLine(ns.Compat.IsFocusedQuest(entry.questID) and "Click to stop navigating to this quest." or "Click to navigate to this quest.", 1, 1, 1, true)
            GameTooltip:AddLine("Shift-click to stop tracking.", 0.8, 0.8, 0.8)
            GameTooltip:Show()
        end
    end)
    local function leave(self)
        if self.highlightedQuestID then ns.Compat.ClearQuestHighlight(self.highlightedQuestID, self); self.highlightedQuestID = nil end
        self.hover:Hide()
        if GameTooltip and GameTooltip.GetOwner and GameTooltip:GetOwner() == self then GameTooltip:Hide() end
    end
    marker:SetScript("OnLeave", leave)
    marker:SetScript("OnHide", leave)
    return marker
end

local function updateStatusArt(marker, entry)
    marker.label:SetText(entry.failed and "!" or (entry.completed and "?" or "..."))
    marker.label:SetTextColor(1, entry.failed and 0.15 or 0.82, 0)
    local selected = ns.Compat.IsFocusedQuest(entry.questID)
    local ring = selected and "UI-QuestPoi-QuestNumber-SuperTracked" or "UI-QuestPoi-QuestNumber"
    if hasAtlas(ring) then marker.ring:SetAtlas(ring, true) end
    local atlas = entry.completed and "UI-QuestIcon-TurnIn-Normal" or (selected and "Quest-In-Progress-Icon-Brown" or "Quest-In-Progress-Icon-yellow")
    if not entry.failed and hasAtlas(atlas) then
        marker.icon:SetAtlas(atlas, true)
        marker.icon:Show()
        marker.label:Hide()
    else
        marker.icon:Hide()
        marker.label:Show()
    end
end

function U.UpdateStatusMarker(marker, entry)
    if entry.custom or entry.recipe then marker.entry = nil; marker:Hide(); return end
    if marker.entry and marker.entry.questID ~= entry.questID and marker.highlightedQuestID then
        ns.Compat.ClearQuestHighlight(marker.highlightedQuestID, marker); marker.highlightedQuestID = nil
        marker.hover:Hide()
    end
    marker.entry = entry
    updateStatusArt(marker, entry)
    marker:Show()
end

function U.UpdateQuestFocus(quests)
    local watched = {}
    for _, quest in ipairs(quests or {}) do watched[quest.questID] = quest.watched end
    -- Update existing click targets and textures without moving, hiding or
    -- rebinding any frame or secure item/macro button during combat.
    for _, row in ipairs(rows) do
        local entry = row.entry
        if entry and entry.questID and not entry.custom and not entry.recipe and row:IsShown() then
            if watched[entry.questID] ~= nil then entry.watched = watched[entry.questID] end
            updateStatusArt(row.status, entry)
        end
    end
end

local function getRow(index)
    if rows[index] then return rows[index] end
    local row = CreateFrame("Frame", nil, U.content)
    row.title = CreateFrame("Button", nil, row)
    row.title:SetPoint("TOPLEFT", 28, 0)
    row.title:RegisterForClicks("LeftButtonUp", "RightButtonUp")
    row.title:SetHighlightTexture("Interface\\QuestFrame\\UI-QuestTitleHighlight", "ADD")
    row.titleText = row.title:CreateFontString(nil, "ARTWORK", "GameFontNormal")
    row.titleText:SetPoint("TOPLEFT")
    row.titleText:SetJustifyH("LEFT")
    row.titleText:SetJustifyV("TOP")
    ns.Compat.CopyNativeStyle(row.titleText, "title")
    row.sectionLine = row.title:CreateTexture(nil, "BACKGROUND")
    row.sectionLine:SetPoint("BOTTOMLEFT")
    row.sectionLine:SetPoint("BOTTOMRIGHT")
    row.sectionLine:SetHeight(1)
    row.sectionLine:SetColorTexture(0.8, 0.65, 0.3, 0.5)
    row.sectionLine:Hide()
    row.body = text(row, "GameFontHighlightSmall", "", 28, -20)
    ns.Compat.CopyNativeStyle(row.body, "objective")
    row.check = CreateFrame("CheckButton", nil, row, "UICheckButtonTemplate")
    row.check:SetSize(20, 20)
    row.check:SetPoint("TOPLEFT", -1, 3)
    row.status = U.CreateStatusMarker(row, 0, 1)
    row.recipeIcon = row:CreateTexture(nil, "ARTWORK")
    row.recipeIcon:SetSize(20, 20); row.recipeIcon:SetPoint("TOPLEFT", 0, 1)
    row.way = button(row, "", 28, 0, 0, function()
        if not row.entry then return end
        local ok, err = ns.Navigation.Run(row.entry.command, row.entry.title)
        if not ok and err then ns.Print(err) end
    end)
    buttonIcon(row.way, "Waypoint-MapPin-Untracked", "Interface\\Minimap\\Tracking\\None", true)
    row.way:ClearAllPoints(); row.way:SetPoint("TOPRIGHT", 0, 2)
    row.action = CreateFrame("Button", nil, row, "UIPanelButtonTemplate,SecureActionButtonTemplate")
    row.action:SetSize(24, 24)
    row.action:SetPoint("TOPRIGHT", -34, 2)
    row.action:RegisterForClicks("LeftButtonUp", "LeftButtonDown")
    row.actionIcon = row.action:CreateTexture(nil, "ARTWORK")
    row.actionIcon:SetAllPoints()
    row.cooldown = CreateFrame("Cooldown", nil, row.action, "CooldownFrameTemplate")
    row.cooldown:SetAllPoints()
    row.charges = row.action:CreateFontString(nil, "OVERLAY", "NumberFontNormalSmall")
    row.charges:SetPoint("BOTTOMRIGHT", 0, 0)
    rows[index] = row
    return row
end

local function colored(value, key, fallback)
    local color = OBJECTIVE_TRACKER_COLOR and OBJECTIVE_TRACKER_COLOR[key]
    if color then
        return string.format("|cff%02x%02x%02x%s|r", color.r * 255, color.g * 255, color.b * 255, value)
    end
    return "|cff" .. fallback .. value .. "|r"
end

local function bodyText(entry)
    if entry.custom then return entry.notes or "" end
    if entry.failed then return colored(FAILED or "Failed", "Failed", "ff3333") end
    if entry.completed then return colored(QUEST_WATCH_QUEST_READY or "Ready for turn-in", "Complete", "999999") end
    local lines = {}
    for _, objective in ipairs(entry.objectives or {}) do
        local value = objective.text or ""
        if objective.finished then value = colored(value, "Complete", "999999") end
        lines[#lines + 1] = "- " .. value
    end
    return table.concat(lines, "\n")
end

local function bindRow(row, key)
    if row.boundKey ~= key then
        clearQuestHover(row.title)
        row.status:Hide()
        row:Hide()
        row.boundKey = key
    end
end

local function renderRow(row, entry, group, y, width)
    if U.CloseEntryMenu then U.CloseEntryMenu(row.title) end
    bindRow(row, entry.id)
    row.entry = entry
    row:SetPoint("TOPLEFT", 0, -y)
    row:SetWidth(width)
    row.sectionLine:Hide()
    row.title:ClearAllPoints(); row.title:SetPoint("TOPLEFT", 28, 0)
    row.action:Hide(); row.actionIcon:Hide(); row.cooldown:Hide(); row.charges:Hide(); row.way:Hide(); row.check:Hide(); row.recipeIcon:Hide()
    row.action:SetAttribute("type", nil)
    row.action:SetAttribute("macro", nil)
    row.action:SetAttribute("item", nil)
    local hasCommand = entry.command and entry.command ~= ""
    local macroIndex = entry.macro and entry.macro ~= "" and GetMacroIndexByName and GetMacroIndexByName(entry.macro) or 0
    local hasAction = macroIndex > 0 or entry.itemLink ~= nil
    local controlsWidth = (hasCommand and 36 or 0) + (hasAction and 28 or 0)
    row.title:SetWidth(width - 32 - controlsWidth)
    row.titleText:SetWidth(width - 32 - controlsWidth)
    row.titleText:SetText(U.EntryTitle(entry))
    ns.Compat.CopyNativeStyle(row.titleText, "title")
    U.SetEntryTitleColor(row.titleText, entry)
    if entry.macro and entry.macro ~= "" and macroIndex == 0 then
        row.titleText:SetText(U.EntryTitle(entry) .. " |cffff6666(macro missing)|r")
    end
    local titleHeight = math.max(16, row.titleText:GetStringHeight())
    row.title:SetHeight(titleHeight)
    row.body:ClearAllPoints(); row.body:SetPoint("TOPLEFT", 28, -titleHeight - 2)
    row.body:SetWidth(width - 32)
    ns.Compat.CopyNativeStyle(row.body, "objective")
    row.body:SetText(bodyText(entry))
    row.body:Show()
    local bodyHeight = row.body:GetText() ~= "" and row.body:GetStringHeight() + 4 or 0
    row:SetHeight(math.max(titleHeight + bodyHeight + 9, 28))
    row.title:SetScript("OnClick", function(self, mouse)
        if mouse == "RightButton" then U.Context(entry, group, self)
        elseif entry.custom then U.EditEntry(entry)
        elseif entry.recipe then ns.Recipes.Open(entry)
        else ns.Compat.OpenQuest(entry) end
    end)
    questHover(row.title, entry, entry.title, entry.custom and "Personal task. Click to edit; right-click for actions." or (entry.recipe and "Click to open this recipe. Shift-click to stop tracking. Right-click for categories and actions." or "Click for quest details. Use the circular marker to navigate. Shift-click toggles tracking. Right-click for categories and ordering."))
    if entry.custom then
        U.UpdateStatusMarker(row.status, entry)
        row.check:SetChecked(entry.completed)
        row.check:SetScript("OnClick", function(self)
            local record = ns.db.custom[entry.id]
            if record then record.completed = self:GetChecked() == true end
            ns.RequestRefresh()
        end)
        row.check:Show()
    elseif entry.recipe then
        U.UpdateStatusMarker(row.status, entry)
        row.recipeIcon:SetTexture(entry.icon); row.recipeIcon:Show()
    else
        U.UpdateStatusMarker(row.status, entry)
    end
    if hasCommand then
        tooltip(row.way, "Navigate to waypoint", entry.command)
        row.way:Show()
    end
    if hasAction then
        row.action:ClearAllPoints()
        row.action:SetPoint("TOPRIGHT", hasCommand and -36 or 0, 2)
        if macroIndex > 0 then
            row.action:SetText("M")
            row.action:SetAttribute("type", "macro")
            row.action:SetAttribute("macro", entry.macro)
            tooltip(row.action, "Run macro: " .. entry.macro, "Uses your existing WoW macro. Edit it with /macro.")
        else
            row.action:SetText("")
            row.actionIcon:SetTexture(entry.itemTexture or "Interface\\Icons\\INV_Misc_QuestionMark")
            row.actionIcon:Show()
            row.action:SetAttribute("type", "item")
            row.action:SetAttribute("item", entry.itemLink)
            tooltip(row.action, "Use quest item", entry.itemLink)
            if entry.charges and entry.charges > 1 then
                row.charges:SetText(entry.charges); row.charges:Show()
            end
        end
        row.action:Show()
    end
    row:Show()
    return row:GetHeight()
end

function U.UpdateCooldowns()
    for _, row in ipairs(rows) do
        local entry = row.entry
        if entry and entry.itemLink and row:IsShown() then
            local start, duration, enabled = ns.Compat.ItemCooldown(entry.logIndex)
            if start and CooldownFrame_Set then CooldownFrame_Set(row.cooldown, start, duration, enabled) end
        end
    end
end

function U.Render(quests)
    if ns.InCombat() then ns.dirty = true; return end
    if not U.frame then createTracker() end
    local frame, settings = U.frame, ns.db.settings
    if U.moving then ns.dirty = true; return end
    local height = settings.collapsed and 56 or settings.height
    if frame:GetWidth() ~= settings.width or frame:GetHeight() ~= height then frame:SetSize(settings.width, height) end
    local p = settings.position
    local point, relativePoint, x, y = "TOPRIGHT", "TOPRIGHT", -64, -240
    if p and p.point and p.relativePoint and type(p.x) == "number" and type(p.y) == "number" then
        point, relativePoint, x, y = p.point, p.relativePoint, p.x, p.y
    end
    local currentPoint, relativeTo, currentRelativePoint, currentX, currentY = frame:GetPoint(1)
    if currentPoint ~= point or relativeTo ~= UIParent or currentRelativePoint ~= relativePoint or currentX ~= x or currentY ~= y then
        frame:ClearAllPoints(); frame:SetPoint(point, UIParent, relativePoint, x, y)
    end
    U.resize:SetShown(not settings.locked and not settings.collapsed)
    U.UpdateLockButton()
    U.background:SetColorTexture(0, 0, 0, settings.backgroundOpacity / 100)
    ns.Compat.CopyNativeStyle(U.header, "header")
    U.header:SetWidth(settings.width - 42)
    U.manageButton:ClearAllPoints()
    if settings.enabled then U.manageButton:SetPoint("TOPRIGHT", U.quickBar, "TOPRIGHT")
    else U.manageButton:SetPoint("TOPRIGHT", frame, "TOPRIGHT", 0, -2) end
    U.manageButton:Show()
    U.scroll:SetShown(not settings.collapsed)
    U.UpdateQuickControls()
    local width = settings.width - 28
    U.content:SetWidth(width)
    local groups, count = M.Build(ns.db, quests, U.query)
    local groupMode = M.View(settings)
    U.groups = groups
    local header = TRACKER_HEADER_QUESTS or "Quests"
    if ns.db.settings.showRecipes and ns.recipes and #ns.recipes > 0 then header = TRACKER_ALL_OBJECTIVES or "Objectives" end
    U.header:SetText(header .. " (" .. count .. ")")
    local index, y = 0, 2
    for _, group in ipairs(groups) do
        if groupMode ~= "none" then
            index = index + 1
            local row = getRow(index)
            bindRow(row, "group:" .. group.key)
            row.entry = nil
            row.status.entry = nil
            row:SetPoint("TOPLEFT", 0, -y)
            row:SetWidth(width)
            row.body:Hide(); row.check:Hide(); row.action:Hide(); row.way:Hide(); row.status:Hide(); row.recipeIcon:Hide()
            row.title:ClearAllPoints(); row.title:SetPoint("TOPLEFT", 8, 0)
            row.title:SetWidth(width - 16)
            row.titleText:SetWidth(width - 16)
            row.titleText:SetText(group.label .. " |cff999999(" .. #group.entries .. ")|r")
            ns.Compat.CopyNativeStyle(row.titleText, "header")
            row.titleText:SetTextColor(1, 0.82, 0)
            row.title:SetHeight(math.max(18, row.titleText:GetStringHeight()) + 5)
            row:SetHeight(row.title:GetHeight() + 9)
            row.sectionLine:SetAlpha(group.collapsed and 0.4 or 1)
            row.sectionLine:Show()
            row.title:SetScript("OnClick", function()
                ns.db.collapsedGroups[group.key] = not group.collapsed; ns.RequestRefresh()
            end)
            tooltip(row.title, group.label, group.collapsed and "Click to expand this section." or "Click to collapse this section.")
            row:Show(); y = y + row:GetHeight()
        end
        if not group.collapsed or groupMode == "none" then
            for _, entry in ipairs(group.entries) do
                index = index + 1
                y = y + renderRow(getRow(index), entry, group, y, width)
            end
        end
    end
    for unused = index + 1, #rows do
        clearQuestHover(rows[unused].title)
        rows[unused].status:Hide()
        rows[unused]:Hide()
    end
    local emptyText = U.query and U.query ~= "" and "No matching quests, recipes or tasks." or "No tracked quests or recipes.\nUse the add icon for a personal task, or the settings icon to manage the tracker."
    if M.FilterCount(ns.db) > 0 then emptyText = "No entries match your filters or search.\nUse Filters to show more entries." end
    U.empty:SetText(count == 0 and emptyText or "")
    U.empty:SetShown(count == 0)
    U.content:SetHeight(math.max(y + 8, 72))
    U.scroll:UpdateScrollChildRect()
    U.RequestTrackerScroll(U.pendingScroll or U.scroll:GetVerticalScroll())
    U.UpdateCooldowns()
    ns.Compat.CopyNativeStyle(U.empty, "objective")
    frame:SetShown(settings.enabled)
    U.chrome:SetShown(settings.enabled)
end

function U.ApplyLiveAppearance()
    if U.background and ns.db then U.background:SetColorTexture(0, 0, 0, ns.db.settings.backgroundOpacity / 100) end
end

function U.EntryTitle(entry)
    if not entry.custom and not entry.recipe and entry.level and entry.level > 0 then
        return string.format("[%d] %s", entry.level, entry.title)
    end
    return entry.title
end

function U.SetEntryTitleColor(label, entry)
    local color = ns.Compat.QuestDifficultyColor(entry)
    if color then label:SetTextColor(color.r, color.g, color.b)
    elseif entry.custom and entry.completed then label:SetTextColor(0.5, 0.5, 0.5)
    else label:SetTextColor(1, 0.82, 0) end
end

U.Widgets = { text = text, button = button, tooltip = tooltip, dialog = dialog,
    field = field, acquire = acquire, labels = labels, questHover = questHover,
    dropdown = dropdown, buttonIcon = buttonIcon, primaryButton = primaryButton,
    selectButton = selectButton, scrollRail = scrollRail, surface = surface, theme = theme, plusIcon = plusIcon }
