local _, ns = ...
local U, M, W = ns.UI, ns.Model, ns.UI.Widgets
local text, button, field, tooltip = W.text, W.button, W.field, W.tooltip
local pageNames = { tasks = "Quests & Tasks", categories = "Categories", settings = "Settings", appearance = "Appearance" }
local pageOrder = { "tasks", "categories", "settings", "appearance" }

local function list(parent, y, height)
    local scroll = CreateFrame("ScrollFrame", nil, parent)
    scroll:SetPoint("TOPLEFT", 24, y)
    scroll:SetSize(548, height)
    local content = CreateFrame("Frame", nil, scroll)
    content:SetSize(540, 1)
    scroll:SetScrollChild(content)
    scroll:EnableMouseWheel(true)
    scroll:SetScript("OnMouseWheel", function(self, delta)
        self:SetVerticalScroll(math.max(0, math.min(self:GetVerticalScrollRange(), self:GetVerticalScroll() - delta * 42)))
    end)
    scroll:HookScript("OnVerticalScroll", function() U.CloseEntryMenu() end)
    W.scrollRail(scroll)
    return { scroll = scroll, content = content, rows = {} }
end

local function finishList(view, height)
    view.content:SetHeight(math.max(1, height))
    view.scroll:UpdateScrollChildRect()
    view.scroll:SetVerticalScroll(math.min(view.scroll:GetVerticalScroll(), view.scroll:GetVerticalScrollRange()))
    view.scroll.lqlRail:Sync()
end

local function refreshTasks()
    local page = U.pages.tasks
    local settings = {}
    for key, value in pairs(ns.db.settings) do settings[key] = value end
    settings.group, settings.sort, settings.scope, settings.showCompleted, settings.quickSort = "none", "title", "all", true, ""
    settings.quickTurnIn = false
    local viewDB = { settings = settings, custom = ns.db.custom, questMeta = ns.db.questMeta,
        recipeMeta = ns.db.recipeMeta, categories = ns.db.categories, collapsedGroups = {},
        filters = { types = {}, zones = {}, categories = {} } }
    local groups, count = M.Build(viewDB, ns.entries or ns.quests or {}, U.managerQuery)
    page.count:SetText(count .. " entries")
    local entries = groups[1] and groups[1].entries or {}
    local view = page.list
    for index, entry in ipairs(entries) do
        local row = view.rows[index]
        if not row then
            row = CreateFrame("Frame", nil, view.content)
            row:SetSize(540, 54)
            local background = row:CreateTexture(nil, "BACKGROUND")
            background:SetAllPoints(); background:SetColorTexture(0.06, 0.075, 0.1, 1)
            row.title = text(row, "GameFontNormal", "", 38, -8)
            row.title:SetWidth(456)
            row.title:SetWordWrap(false)
            row.detail = text(row, "GameFontHighlightSmall", "", 38, -30)
            row.detail:SetTextColor(unpack(W.theme.muted))
            row.detail:SetWidth(456)
            row.detail:SetWordWrap(false)
            row.check = CreateFrame("CheckButton", nil, row, "UICheckButtonTemplate")
            row.check:SetSize(24, 24); row.check:SetPoint("TOPLEFT", 5, -4)
            row.status = U.CreateStatusMarker(row, 8, -6)
            row.recipeIcon = row:CreateTexture(nil, "ARTWORK")
            row.recipeIcon:SetSize(20, 20); row.recipeIcon:SetPoint("TOPLEFT", 8, -6)
            row.menu = button(row, "", 28, 0, 0, function()
                if row.entry then U.EntryMenu(row.menu, row.entry) end
            end)
            row.menu:ClearAllPoints(); row.menu:SetSize(28, 34); row.menu:SetPoint("TOPRIGHT", -6, -6)
            row.menu:SetFrameLevel(row:GetFrameLevel() + 2)
            row.menu:EnableMouse(true)
            if row.menu.EnableMouseClicks then row.menu:EnableMouseClicks(true) end
            if row.menu.EnableMouseMotion then row.menu:EnableMouseMotion(true) end
            row.menu:RegisterForClicks("LeftButtonUp")
            row.menu:SetHighlightTexture("Interface\\Buttons\\ButtonHilight-Square", "ADD")
            for _, offset in ipairs({ 5, 0, -5 }) do
                local dot = row.menu:CreateTexture(nil, "ARTWORK")
                dot:SetPoint("CENTER", 0, offset); dot:SetSize(2, 2)
                dot:SetColorTexture(0.78, 0.83, 0.92, 1)
            end
            row.menu:SetScript("OnClick", function()
                if row.entry then U.EntryMenu(row.menu, row.entry) end
            end)
            row:SetScript("OnHide", function() U.CloseEntryMenu(row.menu) end)
            view.rows[index] = row
        end
        local previous = row.entry
        if previous and (previous.id ~= entry.id or previous.watched ~= entry.watched
            or previous.category ~= entry.category or previous.completed ~= entry.completed) then
            U.CloseEntryMenu(row.menu)
            if previous.id ~= entry.id then row:Hide() end
        end
        row.entry = entry
        row:ClearAllPoints(); row:SetPoint("TOPLEFT", 0, -(index - 1) * 58)
        row.title:SetText(U.EntryTitle(entry))
        U.SetEntryTitleColor(row.title, entry)
        local kind = entry.custom and "Personal task" or (entry.recipe and "Tracked recipe" or (entry.watched and "Tracked quest" or "Quest"))
        row.detail:SetText(kind .. " - " .. (entry.zone ~= "" and entry.zone or "Other quests") .. " - " .. M.CategoryName(ns.db, entry.category))
        row.check:SetShown(entry.custom)
        U.UpdateStatusMarker(row.status, entry)
        row.recipeIcon:SetShown(entry.recipe == true)
        if entry.custom then
            row.check:SetChecked(entry.completed)
            row.check:SetScript("OnClick", function(self)
                local record = ns.db.custom[entry.id]
                if record then record.completed = self:GetChecked() == true end
                ns.RequestRefresh()
            end)
        elseif entry.recipe then row.recipeIcon:SetTexture(entry.icon) end
        tooltip(row.menu, "Actions: " .. entry.title, "Change category, manage tracking or edit this entry.")
        row:Show()
    end
    for index = #entries + 1, #view.rows do view.rows[index]:Hide(); view.rows[index].entry = nil end
    page.empty:SetShown(#entries == 0)
    finishList(view, #entries * 58)
end

local function refreshCategories()
    local view = U.pages.categories.list
    for _, row in ipairs(view.rows) do row:Hide() end
    for index, category in ipairs(ns.db.categories) do
        local row = view.rows[index]
        if not row then
            row = CreateFrame("Frame", nil, view.content)
            row:SetSize(540, 52)
            local background = row:CreateTexture(nil, "BACKGROUND")
            background:SetAllPoints(); background:SetColorTexture(0.06, 0.075, 0.1, 1)
            row.title = text(row, "GameFontHighlight", "", 12, -6)
            row.title:SetWidth(320); row.title:SetWordWrap(false)
            row.count = text(row, "GameFontHighlightSmall", "", 12, -28)
            row.count:SetTextColor(unpack(W.theme.muted))
            button(row, "Rename", 80, 356, -10, function() U.CategoryEditor(row.categoryID) end)
            button(row, "Remove", 80, 446, -10, function()
                M.DeleteCategory(ns.db, row.categoryID)
                ns.RequestRefresh()
            end)
            view.rows[index] = row
        end
        row.categoryID = category.id
        row:ClearAllPoints(); row:SetPoint("TOPLEFT", 0, -(index - 1) * 56)
        row.title:SetText(category.name)
        local count = 0
        for _, custom in pairs(ns.db.custom) do if custom.category == category.id then count = count + 1 end end
        for _, quest in ipairs(ns.entries or ns.quests or {}) do
            local meta = quest.recipe and ns.db.recipeMeta[quest.id] or ns.db.questMeta[tostring(quest.questID)]
            if meta and meta.category == category.id then count = count + 1 end
        end
        row.count:SetText(count .. " quests, recipes and tasks")
        row:Show()
    end
    U.pages.categories.empty:SetShown(#ns.db.categories == 0)
    finishList(view, #ns.db.categories * 56)
end

local function settingCheck(page, label, key, y)
    local control = CreateFrame("CheckButton", nil, page, "UICheckButtonTemplate")
    control:SetSize(24, 24); control:SetPoint("TOPLEFT", 21, y)
    text(page, "GameFontHighlight", label, 50, y - 6)
    page.checks[key] = control
    control:SetScript("OnClick", function(self)
        if key == "locked" then U.SetLocked(self:GetChecked())
        else ns.db.settings[key] = self:GetChecked() == true; ns.RequestRefresh() end
    end)
end

local function settingCycle(page, label, key, values, y)
    text(page, "GameFontHighlight", label, 24, y - 8)
    page.cycles[key] = W.dropdown(page, 295, 274, y, label, function()
        local choices = {}
        for _, value in ipairs(values) do choices[#choices + 1] = { value = value, label = W.labels[value] } end
        return choices
    end, function() return ns.db.settings[key] end, function(value)
        ns.db.settings[key] = value
        if key == "group" or key == "sort" then
            ns.db.settings.quickSort, ns.db.settings.quickTurnIn = "", false
        end
        ns.RequestRefresh()
    end)
end

local function slider(page, label, key, y, low, high, suffix)
    text(page, "GameFontNormal", label, 0, y)
    local control = CreateFrame("Slider", nil, page)
    control:SetSize(380, 18); control:SetPoint("TOPLEFT", 8, y - 28)
    control:SetOrientation("HORIZONTAL")
    control:SetMinMaxValues(low, high); control:SetValueStep(1); control:SetObeyStepOnDrag(true)
    local track = control:CreateTexture(nil, "BACKGROUND")
    track:SetAllPoints(); track:SetTexture("Interface\\Buttons\\UI-SliderBar-Background")
    control:SetThumbTexture("Interface\\Buttons\\UI-SliderBar-Button-Horizontal")
    local valueText = text(page, "GameFontHighlight", "", 420, y - 29)
    text(page, "GameFontHighlightSmall", tostring(low) .. suffix, 8, y - 53)
    text(page, "GameFontHighlightSmall", tostring(high) .. suffix, 360, y - 53)
    local function sync()
        control.lqlUpdating = true
        control:SetValue(ns.db.settings[key])
        valueText:SetText(tostring(ns.db.settings[key]) .. suffix)
        control.lqlUpdating = false
    end
    control:SetScript("OnValueChanged", function(_, value)
        if control.lqlUpdating then return end
        value = math.max(low, math.min(high, math.floor(value + 0.5)))
        ns.db.settings[key] = value
        valueText:SetText(value .. suffix)
        ns.RequestRefresh()
    end)
    page.sliders[key] = control
    page.sync[key] = sync
    sync()
end

local function createManager()
    local frame = W.dialog("LamdaQuestLogOptions", "Lamda Quest Log", 840, 660)
    U.options, U.pages, U.nav = frame, {}, {}
    frame:HookScript("OnHide", function() U.CloseEntryMenu() end)
    local separator = frame:CreateTexture(nil, "BACKGROUND")
    separator:SetColorTexture(0.19, 0.23, 0.3, 1)
    separator:SetSize(1, 510); separator:SetPoint("TOPLEFT", 208, -78)
    local icons = {
        tasks = { nil, "Interface\\GossipFrame\\AvailableQuestIcon" },
        categories = { nil, "Interface\\Icons\\INV_Misc_Note_06" },
        settings = { "questlog-icon-setting", "Interface\\Buttons\\UI-OptionsButton" },
        appearance = { nil, "Interface\\Icons\\INV_Misc_Dye_01" },
    }
    for index, key in ipairs(pageOrder) do
        local pageKey = key
        local page = CreateFrame("Frame", nil, frame)
        page:SetPoint("TOPLEFT", 224, -78); page:SetSize(592, 510)
        U.pages[key] = page
        U.nav[key] = button(frame, pageNames[key], 176, 16, -78 - (index - 1) * 44, function() U.Options(pageKey) end)
        U.nav[key]:SetHeight(36)
        W.buttonIcon(U.nav[key], icons[key][1], icons[key][2])
    end
    local tasks = U.pages.tasks
    local addTask = W.primaryButton(button(tasks, "Add personal task", 178, 24, 0, function() U.EditEntry() end))
    W.plusIcon(addTask)
    tasks.count = text(tasks, "GameFontHighlightSmall", "", 222, -8)
    tasks.count:SetTextColor(unpack(W.theme.muted))
    tasks.search = field(tasks, "Search quests, recipes, tasks, zones and categories", -52, 548)
    tasks.search:SetScript("OnTextChanged", function(self)
        U.managerQuery = self:GetText(); refreshTasks()
    end)
    tasks.list = list(tasks, -122, 366)
    tasks.empty = text(tasks.list.content, "GameFontHighlightSmall", "No entries match your search.", 12, -12)
    local categories = U.pages.categories
    local addCategory = W.primaryButton(button(categories, "New category", 172, 24, 0, function() U.CategoryEditor() end))
    W.plusIcon(addCategory)
    local hint = text(categories, "GameFontHighlightSmall", "Assign categories from an entry's actions menu.\nRemoving a category keeps its quests and tasks.", 24, -48)
    hint:SetTextColor(unpack(W.theme.muted)); hint:SetWidth(548)
    categories.list = list(categories, -112, 376)
    categories.empty = text(categories.list.content, "GameFontHighlightSmall", "No categories yet. Click New category to create one.", 0, -6)
    local settings = U.pages.settings
    settings.cycles, settings.checks = {}, {}
    settingCycle(settings, "Group by", "group", M.groups, 0)
    settingCycle(settings, "Sort within groups", "sort", M.sorts, -36)
    settingCycle(settings, "Show game quests", "scope", M.scopes, -72)
    settingCheck(settings, "Enable Lamda tracker", "enabled", -116)
    settingCheck(settings, "Replace Blizzard quest / recipe sections", "replaceNative", -148)
    settingCheck(settings, "Show completed quests and tasks", "showCompleted", -180)
    settingCheck(settings, "Lock position and size", "locked", -212)
    settingCheck(settings, "Show tracked recipes", "showRecipes", -244)
    settings.search = field(settings, "Filter entries shown in the on-screen tracker", -282, 500)
    settings.search:SetScript("OnTextChanged", function(self)
        U.query = self:GetText(); ns.RequestRefresh()
    end)
    button(settings, "Reset tracker position", 195, 0, -352, function()
        U.StopMotion(); ns.db.settings.position = nil
        ns.db.settings.width, ns.db.settings.height = 300, 480
        ns.RequestRefresh()
    end)
    button(settings, "Use Blizzard tracker", 195, 210, -352, function() ns.SetEnabled(false) end)
    local help = text(settings, "GameFontHighlightSmall", "The settings icon stays available when the tracker is hidden.\nSettings and edits are available in combat. Movement, resizing and tracker layout changes apply after combat.", 0, -384)
    help:SetWidth(515)
    button(settings, "Quest filters...", 150, 0, -444, function() U.Filters() end)
    local appearance = U.pages.appearance
    appearance.sliders, appearance.sync = {}, {}
    slider(appearance, "Background opacity", "backgroundOpacity", 0, 0, 100, "%")
    slider(appearance, "Font size", "fontSize", -112, 12, 20, "")
    local description = text(appearance, "GameFontHighlightSmall", "Opacity changes the background, as in Blizzard's default UI.\nText stays readable and the management button stays visible.\nFont size applies to this tracker; section headers are two points larger.", 0, -218)
    description:SetWidth(515)
    button(appearance, "Reset appearance", 180, 0, -300, function()
        ns.db.settings.backgroundOpacity, ns.db.settings.fontSize = 0, 12
        for _, sync in pairs(appearance.sync) do sync() end
        ns.RequestRefresh()
    end)
    U.managerStatus = text(frame, "GameFontHighlightSmall", "Settings are saved for this character.", 248, -622)
    U.managerStatus:SetTextColor(unpack(W.theme.muted))
    U.managerStatus:SetWidth(420)
    button(frame, "Close", 100, 716, -618, function() frame:Hide() end)
end

function U.RefreshManager()
    if not U.options or not U.options:IsShown() then return end
    U.managerStatus:SetText(ns.InCombat() and "Settings available; tracker layout updates after combat." or "Settings are saved for this character.")
    local settings = U.pages.settings
    for key, control in pairs(settings.checks) do control:SetChecked(ns.db.settings[key]) end
    for key, control in pairs(settings.cycles) do
        local label = W.labels[ns.db.settings[key]]
        local active, turnIn = M.QuickSorting(ns.db.settings), ns.db.settings.quickTurnIn
        if active and key == "sort" then
            local _, sort = M.View(ns.db.settings)
            label = ns.db.settings.quickSort == "" and "Ready for turn-in (quick view)"
                or ((turnIn and "Turn-in first; " or "") .. W.labels[sort] .. " (quick view)")
        elseif active and key == "group" then label = "No groups (quick view)" end
        control:SetText(label)
    end
    if U.managerPage == "tasks" then refreshTasks()
    elseif U.managerPage == "categories" then refreshCategories()
    elseif U.managerPage == "appearance" then for _, sync in pairs(U.pages.appearance.sync) do sync() end end
end

function U.Options(page)
    if not U.options then createManager() end
    local nextPage = pageNames[page] and page or U.managerPage or "tasks"
    if U.managerPage ~= nextPage then U.CloseEntryMenu() end
    U.managerPage = nextPage
    for key, content in pairs(U.pages) do
        content:SetShown(key == U.managerPage)
        U.nav[key]:SetText(pageNames[key])
        W.selectButton(U.nav[key], key == U.managerPage)
    end
    U.options:Show(); U.options:Raise()
    U.RefreshManager()
end
