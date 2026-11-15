local _, ns = ...
local U, M, W = ns.UI, ns.Model, ns.UI.Widgets

function U.CloseEntryMenu(owner)
    if owner and U.entryMenuOwner ~= owner then return end
    local menu = U.entryMenuFrame
    U.entryMenuFrame, U.entryMenuOwner = nil, nil
    if not menu then return end
    if menu.Close then pcall(menu.Close, menu) else menu:Hide() end
end

local function actionsFor(entry, group)
    local actions, categories = {}, {}
    local function action(label, run)
        return { label = label, enabled = true, run = run }
    end
    local function categoryChoice(id, name)
        return { label = name, selected = function()
            local meta = entry.custom and ns.db.custom[entry.id]
                or (entry.recipe and ns.db.recipeMeta[entry.id]) or ns.db.questMeta[tostring(entry.questID)]
            return (meta and meta.category) == id
        end, run = function()
            local meta = M.Meta(ns.db, entry)
            if meta then meta.category = id; ns.RequestRefresh() end
        end }
    end
    categories[#categories + 1] = categoryChoice(nil, "Uncategorized")
    for _, category in ipairs(ns.db.categories) do
        categories[#categories + 1] = categoryChoice(category.id, category.name)
    end
    categories[#categories + 1] = { divider = true }
    categories[#categories + 1] = action("New category...", function() U.CategoryEditor() end)
    actions[#actions + 1] = { label = "Change category", children = categories }
    actions[#actions + 1] = { divider = true }
    if entry.custom then
        actions[#actions + 1] = action("Edit personal task", function() U.EditEntry(entry) end)
        actions[#actions + 1] = action(entry.completed and "Mark incomplete" or "Mark complete", function()
            local record = ns.db.custom[entry.id]
            if record then record.completed = not record.completed; ns.RequestRefresh() end
        end)
        if entry.command and entry.command ~= "" then
            actions[#actions + 1] = action("Navigate to waypoint", function()
                local ok, err = ns.Navigation.Run(entry.command, entry.title)
                if not ok and err then ns.Print(err) end
            end)
            actions[#actions + 1] = action("Copy waypoint to chat", function() ns.Navigation.Copy(entry.command) end)
        end
        actions[#actions + 1] = { divider = true }
        actions[#actions + 1] = action("Delete personal task", function()
            ns.db.custom[entry.id] = nil; ns.RequestRefresh()
        end)
    elseif entry.recipe then
        actions[#actions + 1] = action("Open recipe", function() ns.Recipes.Open(entry) end)
        actions[#actions + 1] = action("Untrack recipe", function() ns.Recipes.Untrack(entry) end)
    else
        actions[#actions + 1] = action("Open quest details", function()
            U.CloseDialogs(); ns.Compat.OpenQuestDetails(entry)
        end)
        actions[#actions + 1] = action(entry.watched and "Untrack quest" or "Track quest", function()
            ns.Compat.Watch(entry, not entry.watched)
        end)
    end
    if group then
        actions[#actions + 1] = { divider = true }
        for _, move in ipairs({ { "Move up", -1 }, { "Move down", 1 } }) do
            local label, delta = move[1], move[2]
            actions[#actions + 1] = action(label, function()
                M.Move(ns.db, group, entry.id, delta); ns.RequestRefresh()
            end)
        end
    end
    return actions
end

local function activate(node)
    if node.enabled == false then return end
    if node.checkbox or node.keepOpen then node.run(); return end
    local menu = U.entryMenuFrame
    U.entryMenuFrame, U.entryMenuOwner = nil, nil
    if menu and menu == U.entryMenuFallback then menu:Hide() end
    -- Native menu responses close their own frames after the callback returns.
    node.run()
end

local function populate(description, nodes)
    if description.SetScrollMode then description:SetScrollMode(300) end
    for _, node in ipairs(nodes) do
        if node.divider then description:CreateDivider()
        elseif node.children then populate(description:CreateButton(node.label), node.children)
        else
            local item
            if node.checkbox then item = description:CreateCheckbox(node.label, node.selected, function() activate(node) end)
            elseif node.selected then item = description:CreateRadio(node.label, node.selected, function() activate(node) end)
            else item = description:CreateButton(node.label, function() activate(node) end) end
            if node.keepOpen and item.SetResponse and MenuResponse and MenuResponse.Refresh then item:SetResponse(MenuResponse.Refresh) end
            item:SetEnabled(node.enabled ~= false)
            if node.enabled == false and item.SetTooltip then
                item:SetTooltip(function(tip) tip:SetText(node.disabledReason or "Unavailable on this client.") end)
            end
        end
    end
end

local function fallbackMenu(owner, title, actions)
    local frame = U.entryMenuFallback
    if not frame then
        frame = CreateFrame("Frame", "LamdaQuestLogEntryMenu", UIParent, "BackdropTemplate")
        frame:Hide()
        U.entryMenuFallback = frame
        frame:SetFrameStrata("FULLSCREEN_DIALOG"); frame:SetFrameLevel(230)
        frame:SetClampedToScreen(true); frame:EnableMouse(true)
        frame:SetBackdrop({ bgFile = "Interface\\Buttons\\WHITE8X8", edgeFile = "Interface\\Tooltips\\UI-Tooltip-Border", edgeSize = 12 })
        frame:SetBackdropColor(0.045, 0.045, 0.05, 1)
        frame.title = W.text(frame, "GameFontNormal", "", 15, -13)
        frame.title:SetWidth(220); frame.title:SetWordWrap(false)
        frame.scroll = CreateFrame("ScrollFrame", nil, frame, "UIPanelScrollFrameTemplate")
        frame.scroll:SetPoint("TOPLEFT", 12, -38)
        frame.scroll:EnableMouseWheel(true)
        frame.scroll:SetScript("OnMouseWheel", function(self, delta)
            self:SetVerticalScroll(math.max(0, math.min(self:GetVerticalScrollRange(), self:GetVerticalScroll() - delta * 24)))
        end)
        frame.content = CreateFrame("Frame", nil, frame.scroll)
        frame.content:SetWidth(220); frame.scroll:SetScrollChild(frame.content)
        frame.rows = {}
        frame:SetScript("OnHide", function(self)
            if U.entryMenuFrame == self then U.entryMenuFrame, U.entryMenuOwner = nil, nil end
        end)
        pcall(frame.RegisterEvent, frame, "GLOBAL_MOUSE_DOWN")
        frame:SetScript("OnEvent", function(self)
            local owner = U.entryMenuOwner
            if self:IsShown() and MouseIsOver and not MouseIsOver(self) and (not owner or not MouseIsOver(owner)) then U.CloseEntryMenu() end
        end)
        if UISpecialFrames then UISpecialFrames[#UISpecialFrames + 1] = "LamdaQuestLogEntryMenu" end
    end
    local show
    show = function(nodes, categoryPage)
        for _, row in ipairs(frame.rows) do row:Hide() end
        local options = {}
        if categoryPage then options[#options + 1] = { label = "< Back", navigate = function() show(actions) end } end
        for _, node in ipairs(nodes) do options[#options + 1] = node end
        options[#options + 1] = { label = "Close", run = function() end }
        local y = 0
        for index, node in ipairs(options) do
            local row = frame.rows[index]
            if not row then
                row = CreateFrame("Button", nil, frame.content)
                row:SetWidth(220); row:EnableMouse(true)
                if row.EnableMouseClicks then row:EnableMouseClicks(true) end
                row:RegisterForClicks("LeftButtonUp")
                row:SetHighlightTexture("Interface\\QuestFrame\\UI-QuestTitleHighlight", "ADD")
                row.label = W.text(row, "GameFontHighlight", "", 22, -5)
                row.label:SetWidth(188); row.label:SetWordWrap(false)
                row.check = row:CreateTexture(nil, "ARTWORK")
                row.check:SetTexture("Interface\\Buttons\\UI-CheckBox-Check")
                row.check:SetSize(18, 18); row.check:SetPoint("LEFT")
                row.line = row:CreateTexture(nil, "BACKGROUND")
                row.line:SetColorTexture(0.6, 0.5, 0.3, 0.4)
                row.line:SetSize(208, 1); row.line:SetPoint("CENTER")
                frame.rows[index] = row
            end
            row.node = node
            row:ClearAllPoints(); row:SetPoint("TOPLEFT", 0, -y)
            row:SetHeight(node.divider and 10 or 26)
            row.label:SetText((node.label or "") .. (node.children and " >" or ""))
            row.label:SetTextColor(node.enabled == false and 0.45 or 1, node.enabled == false and 0.45 or 1, node.enabled == false and 0.45 or 1)
            row.label:SetShown(not node.divider)
            row.check:SetShown(node.selected and node.selected() or false)
            row.line:SetShown(node.divider == true)
            row:SetScript("OnClick", function()
                if node.divider or node.enabled == false then return end
                if node.children then show(node.children, true)
                elseif node.navigate then node.navigate()
                else
                    activate(node)
                    if (node.checkbox or node.keepOpen) and frame:IsShown() then show(nodes, categoryPage) end
                end
            end)
            W.tooltip(row, node.label or "", node.enabled == false and (node.disabledReason or "Unavailable on this client.") or nil)
            row:Show(); y = y + row:GetHeight()
        end
        local extent = math.min(300, y)
        frame:SetSize(260, extent + 50)
        frame.scroll:SetSize(220, extent)
        frame.content:SetHeight(y)
        frame.scroll:UpdateScrollChildRect(); frame.scroll:SetVerticalScroll(0)
        frame.title:SetText(categoryPage and "Change category" or title)
    end
    frame:ClearAllPoints(); frame:SetPoint("TOPRIGHT", owner, "BOTTOMRIGHT", 0, 2)
    U.entryMenuFrame, U.entryMenuOwner = frame, owner
    show(actions); frame:Show()
end

function U.ActionMenu(owner, title, actions)
    if U.entryMenuOwner == owner then U.CloseEntryMenu(); return end
    U.CloseEntryMenu()
    if GameTooltip then GameTooltip:Hide() end
    if MenuUtil and MenuUtil.CreateContextMenu then
        local ok, menu = pcall(MenuUtil.CreateContextMenu, owner, function(_, root)
            root:CreateTitle(title)
            if root.AddMenuReleasedCallback then root:AddMenuReleasedCallback(function(released)
                if U.entryMenuFrame == released then U.entryMenuFrame, U.entryMenuOwner = nil, nil end
            end) end
            populate(root, actions)
        end)
        if ok and menu then U.entryMenuFrame, U.entryMenuOwner = menu, owner; return end
    end
    fallbackMenu(owner, title, actions)
end

function U.EntryMenu(owner, entry, group)
    U.ActionMenu(owner, entry.title, actionsFor(entry, group))
end

function U.SortMenu()
    local settings = ns.db.settings
    local actions = {
        { label = "Ready for turn-in first", checkbox = true,
            selected = function() return settings.quickTurnIn end,
            run = U.ToggleTurnInSort },
        { divider = true },
    }
    for _, value in ipairs(M.quickSorts) do
        local sort = value
        actions[#actions + 1] = { label = W.labels[sort], keepOpen = true,
            enabled = sort ~= "distance" or ns.Compat.HasQuestDistance() or settings.quickSort == "distance",
            disabledReason = "Quest distances are unavailable on this client.",
            selected = function() return settings.quickSort == sort end,
            run = function() U.SetQuickSort(sort) end }
    end
    actions[#actions + 1] = { divider = true }
    actions[#actions + 1] = { label = "Restore saved grouping and sort", run = U.RestoreQuickSort }
    U.ActionMenu(U.nearButton, "Quick sort", actions)
end
