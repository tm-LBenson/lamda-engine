local _, ns = ...
local U, M, W = ns.UI, ns.Model, ns.UI.Widgets
local dimensions = { "types", "zones", "categories" }
local titles = { types = "Quest types", zones = "Zones", categories = "Custom categories" }

local function changed()
    U.UpdateQuickControls()
    U.RefreshFilters()
    ns.RequestRefresh()
end

local function setSection(dimension, include)
    local choices = M.FilterChoices(ns.db, ns.entries or ns.quests or {})
    for _, choice in ipairs(choices[dimension]) do
        if include then ns.db.filters[dimension][choice.key] = nil
        else ns.db.filters[dimension][choice.key] = true end
    end
    changed()
end

local function createPanel()
    local frame = W.dialog("LamdaQuestLogFilters", "Quest filters", 500, 600)
    U.filterPanel = frame
    frame.rows, frame.headers = {}, {}
    local help = W.text(frame, "GameFontHighlightSmall", "Checked choices are shown. Uncheck Dungeon, a zone or a category to hide it. Layout changes apply after combat.", 20, -51)
    help:SetWidth(450)
    local scroll = CreateFrame("ScrollFrame", nil, frame, "UIPanelScrollFrameTemplate")
    scroll:SetPoint("TOPLEFT", 20, -99)
    scroll:SetSize(430, 422)
    scroll:EnableMouseWheel(true)
    scroll:SetScript("OnMouseWheel", function(self, delta)
        self:SetVerticalScroll(math.max(0, math.min(self:GetVerticalScrollRange(), self:GetVerticalScroll() - delta * 42)))
    end)
    local content = CreateFrame("Frame", nil, scroll)
    content:SetSize(425, 1)
    scroll:SetScrollChild(content)
    frame.scroll, frame.content = scroll, content
    frame.empty = W.text(content, "GameFontHighlightSmall", "No filter categories are available yet.", 0, -6)
    frame.summary = W.text(frame, "GameFontHighlightSmall", "", 20, -535)
    W.button(frame, "Reset filters", 130, 20, -559, function()
        for _, dimension in ipairs(dimensions) do ns.db.filters[dimension] = {} end
        changed()
    end)
    W.button(frame, "Close", 100, 370, -559, function() frame:Hide() end)
end

local function headerFor(frame, dimension)
    local header = frame.headers[dimension]
    if header then return header end
    header = CreateFrame("Frame", nil, frame.content)
    header:SetSize(425, 28)
    W.text(header, "GameFontNormal", titles[dimension], 0, -5)
    W.button(header, "All", 45, 312, 0, function() setSection(dimension, true) end)
    W.button(header, "None", 55, 365, 0, function() setSection(dimension, false) end)
    frame.headers[dimension] = header
    return header
end

local function rowFor(frame, index)
    local row = frame.rows[index]
    if row then return row end
    row = CreateFrame("Frame", nil, frame.content)
    row:SetSize(425, 27)
    row.check = CreateFrame("CheckButton", nil, row, "UICheckButtonTemplate")
    row.check:SetSize(24, 24); row.check:SetPoint("TOPLEFT", -3, 1)
    row.label = W.text(row, "GameFontHighlight", "", 27, -5)
    row.label:SetWidth(390); row.label:SetWordWrap(false)
    row.check:SetScript("OnClick", function(self)
        if self:GetChecked() then ns.db.filters[row.dimension][row.key] = nil
        else ns.db.filters[row.dimension][row.key] = true end
        changed()
    end)
    frame.rows[index] = row
    return row
end

function U.RefreshFilters()
    local frame = U.filterPanel
    if not frame or not frame:IsShown() then return end
    local choices = M.FilterChoices(ns.db, ns.entries or ns.quests or {})
    for _, row in ipairs(frame.rows) do row:Hide() end
    for _, header in pairs(frame.headers) do header:Hide() end
    local index, y = 0, 0
    for _, dimension in ipairs(dimensions) do
        if #choices[dimension] > 0 then
            local header = headerFor(frame, dimension)
            header:ClearAllPoints(); header:SetPoint("TOPLEFT", 0, -y); header:Show()
            y = y + 30
            for _, choice in ipairs(choices[dimension]) do
                index = index + 1
                local row = rowFor(frame, index)
                row.dimension, row.key = dimension, choice.key
                row:ClearAllPoints(); row:SetPoint("TOPLEFT", 0, -y)
                row.label:SetText(choice.label)
                row.check:SetChecked(not ns.db.filters[dimension][choice.key])
                W.tooltip(row.check, choice.label, "Checked: show matching entries. Unchecked: hide them.")
                row:Show(); y = y + 27
            end
            y = y + 12
        end
    end
    frame.empty:SetShown(index == 0)
    frame.content:SetHeight(math.max(48, y))
    frame.scroll:UpdateScrollChildRect()
    frame.scroll:SetVerticalScroll(math.min(frame.scroll:GetVerticalScroll(), frame.scroll:GetVerticalScrollRange()))
    frame.summary:SetText(M.FilterCount(ns.db) .. " exclusions. Choices come from your current quest log and tasks.")
end

function U.Filters()
    if not U.filterPanel then createPanel() end
    U.filterPanel:Show()
    U.RefreshFilters()
end
