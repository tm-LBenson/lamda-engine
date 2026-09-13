local _, Addon = ...

local colors = {
    accent = { 0.35, 0.82, 1.00 },
    ok = { 0.35, 0.95, 0.55 },
    change = { 1.00, 0.78, 0.25 },
    warning = { 1.00, 0.88, 0.55 },
    neutral = { 0.75, 0.82, 0.90 },
    muted = { 0.55, 0.61, 0.69 },
}

local function displayText(value)
    return tostring(value == nil and "" or value):gsub("|", "||")
end

local function setText(fontString, value, kind)
    local color = colors[kind or "neutral"]
    fontString:SetText(displayText(value))
    fontString:SetTextColor(color[1], color[2], color[3])
end

local function createSectionTitle(parent, text, y)
    local title = parent:CreateFontString(nil, "OVERLAY", "GameFontNormalLarge")
    title:SetPoint("TOPLEFT", 18, y)
    title:SetText(text)
    title:SetTextColor(unpack(colors.accent))
    return title
end

local function createSummaryRow(parent, y, label)
    local row = CreateFrame("Frame", nil, parent, "BackdropTemplate")
    row:SetPoint("TOPLEFT", 18, y)
    row:SetPoint("RIGHT", -18, 0)
    row:SetHeight(36)
    row:SetBackdrop({ bgFile = "Interface\\Buttons\\WHITE8X8" })
    row:SetBackdropColor(0.075, 0.09, 0.12, 0.90)

    local labelText = row:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    labelText:SetPoint("LEFT", 11, 0)
    labelText:SetWidth(168)
    labelText:SetJustifyH("LEFT")
    labelText:SetText(label)
    labelText:SetTextColor(unpack(colors.muted))

    local value = row:CreateFontString(nil, "OVERLAY", "GameFontHighlight")
    value:SetPoint("LEFT", labelText, "RIGHT", 8, 0)
    value:SetPoint("RIGHT", -11, 0)
    value:SetJustifyH("LEFT")
    return value
end

local function createActionButton(parent, text, width)
    local button = CreateFrame("Button", nil, parent, "UIPanelButtonTemplate")
    button:SetSize(width, 28)
    button:SetText(text)
    return button
end

local function addButtonTooltip(button, title, detail)
    button:SetScript("OnEnter", function(current)
        GameTooltip:SetOwner(current, "ANCHOR_RIGHT")
        GameTooltip:AddLine(title, unpack(colors.accent))
        if detail then GameTooltip:AddLine(detail, unpack(colors.muted), true) end
        GameTooltip:Show()
    end)
    button:SetScript("OnLeave", GameTooltip_Hide)
end

local function countEntries(value)
    local count = 0
    for _ in pairs(value or {}) do
        count = count + 1
    end
    return count
end

local function countLabel(count, singular, plural)
    count = tonumber(count) or 0
    return tostring(count) .. " " .. (count == 1 and singular or (plural or (singular .. "s")))
end

local function describePlacementCount(addon, snapshot)
    if not snapshot or not addon.GetCurrentTrainingPlacements then
        return 0
    end
    return countEntries(addon:GetCurrentTrainingPlacements(snapshot))
end

local function addActionTooltip(label, data)
    GameTooltip:AddLine(" ")
    GameTooltip:AddLine(displayText(label), unpack(colors.accent))
    if not data then
        GameTooltip:AddLine("Not scanned", unpack(colors.muted))
        return
    end
    GameTooltip:AddDoubleLine("Key", displayText(data.key or "—"), 0.7, 0.75, 0.82, 1, 1, 1)
    GameTooltip:AddDoubleLine("Action", displayText(data.actionName or "Empty"), 0.7, 0.75, 0.82, 1, 1, 1)
    if data.bindingCommand and data.bindingCommand ~= "" then
        GameTooltip:AddDoubleLine("Binding", displayText(data.bindingCommand), 0.7, 0.75, 0.82, 1, 1, 1)
    end
    if data.actionSlot then
        GameTooltip:AddDoubleLine("Action slot", tostring(data.actionSlot), 0.7, 0.75, 0.82, 1, 1, 1)
    end
end

function Addon:CreateSetupPanel(parent)
    local panel = CreateFrame("Frame", nil, parent)
    panel:SetAllPoints()
    self.setupPanel = panel

    createSectionTitle(panel, "Overview", -8)
    local description = panel:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    description:SetPoint("TOPLEFT", 18, -35)
    description:SetPoint("RIGHT", -18, 0)
    description:SetJustifyH("LEFT")
    description:SetText("Teach lamdaUI a setup you like, then use it to arrange another specialization or character.")
    description:SetTextColor(unpack(colors.muted))

    self.setupValues = {
        context = createSummaryRow(panel, -62, "Specialization"),
        profile = createSummaryRow(panel, -102, "Active profile"),
        learned = createSummaryRow(panel, -142, "Current setup"),
        plan = createSummaryRow(panel, -182, "Preview"),
    }

    createSectionTitle(panel, "Next step", -236)
    local nextStep = CreateFrame("Frame", nil, panel, "BackdropTemplate")
    nextStep:SetPoint("TOPLEFT", 18, -264)
    nextStep:SetPoint("RIGHT", -18, 0)
    nextStep:SetHeight(92)
    nextStep:SetBackdrop({ bgFile = "Interface\\Buttons\\WHITE8X8" })
    nextStep:SetBackdropColor(0.075, 0.09, 0.12, 0.90)

    local nextTitle = nextStep:CreateFontString(nil, "OVERLAY", "GameFontNormal")
    nextTitle:SetPoint("TOPLEFT", 12, -12)
    nextTitle:SetPoint("RIGHT", -12, 0)
    nextTitle:SetJustifyH("LEFT")
    self.setupNextTitle = nextTitle

    local nextDetail = nextStep:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    nextDetail:SetPoint("TOPLEFT", nextTitle, "BOTTOMLEFT", 0, -7)
    nextDetail:SetPoint("RIGHT", -12, 0)
    nextDetail:SetHeight(42)
    nextDetail:SetJustifyH("LEFT")
    nextDetail:SetJustifyV("TOP")
    nextDetail:SetWordWrap(true)
    nextDetail:SetTextColor(unpack(colors.muted))
    self.setupNextDetail = nextDetail

    local profilesButton = createActionButton(panel, "Profiles", 94)
    profilesButton:SetPoint("BOTTOMLEFT", 18, 12)
    profilesButton:SetScript("OnClick", function()
        self:SelectTab("map")
    end)
    addButtonTooltip(profilesButton, "Manage profiles", "Capture, name, share, or switch the bound inputs and Blizzard UI settings lamdaUI should manage.")
    self.setupProfilesButton = profilesButton

    local learnButton = createActionButton(panel, "Learn Current", 126)
    learnButton:SetPoint("LEFT", profilesButton, "RIGHT", 10, 0)
    learnButton:SetScript("OnClick", function()
        self:ConfirmLearnCurrentSetup()
    end)
    addButtonTooltip(learnButton, "Learn the current setup", "Refreshes and remembers the current specialization, form, talent loadout, and managed placements in one click. Nothing is moved.")
    self.setupLearnButton = learnButton

    local previewButton = createActionButton(panel, "Build Preview", 126)
    previewButton:SetPoint("LEFT", learnButton, "RIGHT", 10, 0)
    previewButton:SetScript("OnClick", function()
        self:SelectTab("layout")
        if not self.layoutPlan then self:PreviewLayoutPlan() end
    end)
    addButtonTooltip(previewButton, "Build a preview", "Creates a read-only before-and-after layout for the current specialization and form.")
    self.setupPreviewButton = previewButton

    local safety = panel:CreateFontString(nil, "OVERLAY", "GameFontDisableSmall")
    safety:SetPoint("LEFT", previewButton, "RIGHT", 14, 0)
    safety:SetPoint("RIGHT", -18, 0)
    safety:SetJustifyH("LEFT")
    safety:SetText("Review before Apply • Undo is saved locally")
    self.setupSafety = safety
end

local function createMapRow(addon, container, index)
    local row = CreateFrame("Frame", nil, container, "BackdropTemplate")
    row:SetSize(710, 34)
    row:SetPoint("TOPLEFT", 0, -((index - 1) * 38))
    row:SetBackdrop({ bgFile = "Interface\\Buttons\\WHITE8X8" })
    row:SetBackdropColor(0.075, 0.09, 0.12, 0.90)

    local input = row:CreateFontString(nil, "OVERLAY", "GameFontNormal")
    input:SetPoint("LEFT", 10, 0)
    input:SetWidth(132)
    input:SetJustifyH("LEFT")
    row.inputText = input

    local purpose = row:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    purpose:SetPoint("LEFT", input, "RIGHT", 8, 0)
    purpose:SetWidth(190)
    purpose:SetJustifyH("LEFT")
    purpose:SetWordWrap(false)
    row.purposeText = purpose

    local action = row:CreateFontString(nil, "OVERLAY", "GameFontHighlight")
    action:SetPoint("LEFT", purpose, "RIGHT", 8, 0)
    action:SetPoint("RIGHT", -10, 0)
    action:SetJustifyH("LEFT")
    action:SetWordWrap(false)
    row.actionText = action

    row:EnableMouse(true)
    row:SetScript("OnEnter", function(current)
        local snapshot = addon.currentSnapshot
        local destination = current.destination
        local data = destination and snapshot and snapshot.destinations and snapshot.destinations[destination.id]
        GameTooltip:SetOwner(current, "ANCHOR_RIGHT")
        GameTooltip:AddLine(displayText(destination and destination.label or "Action destination"), unpack(colors.accent))
        if destination and destination.category then
            GameTooltip:AddLine(displayText(destination.category), unpack(colors.muted))
        end
        addActionTooltip("Current action", data)
        GameTooltip:Show()
    end)
    row:SetScript("OnLeave", GameTooltip_Hide)
    return row
end

function Addon:CreateActionMapPanel(parent)
    local panel = CreateFrame("Frame", nil, parent)
    panel:SetAllPoints()
    panel:Hide()
    self.actionMapPanel = panel

    createSectionTitle(panel, "Profiles", -8)
    local description = panel:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    description:SetPoint("TOPLEFT", 18, -35)
    description:SetPoint("RIGHT", -18, 0)
    description:SetJustifyH("LEFT")
    description:SetText("A profile stores the bound inputs and Blizzard UI settings lamdaUI should reuse.")
    description:SetTextColor(unpack(colors.muted))

    local profileRow = CreateFrame("Frame", nil, panel, "BackdropTemplate")
    profileRow:SetPoint("TOPLEFT", 18, -62)
    profileRow:SetPoint("RIGHT", -18, 0)
    profileRow:SetHeight(76)
    profileRow:SetBackdrop({ bgFile = "Interface\\Buttons\\WHITE8X8" })
    profileRow:SetBackdropColor(0.075, 0.09, 0.12, 0.90)

    local profileLabel = profileRow:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    profileLabel:SetPoint("TOPLEFT", 10, -15)
    profileLabel:SetText("ACTIVE PROFILE")
    profileLabel:SetTextColor(unpack(colors.muted))

    local profileName = profileRow:CreateFontString(nil, "OVERLAY", "GameFontHighlight")
    profileName:SetPoint("LEFT", profileLabel, "RIGHT", 10, 0)
    profileName:SetWidth(260)
    profileName:SetJustifyH("LEFT")
    profileName:SetWordWrap(false)
    self.actionMapProfileName = profileName

    local previousButton = createActionButton(profileRow, "<", 30)
    previousButton:SetPoint("TOPRIGHT", -154, -8)
    previousButton:SetScript("OnClick", function() self:CycleDestinationProfile(-1) end)
    addButtonTooltip(previousButton, "Previous profile", "Switches the active input and UI profile without changing the current bars.")
    self.actionMapPreviousProfileButton = previousButton

    local nextButton = createActionButton(profileRow, ">", 30)
    nextButton:SetPoint("LEFT", previousButton, "RIGHT", 4, 0)
    nextButton:SetScript("OnClick", function() self:CycleDestinationProfile(1) end)
    addButtonTooltip(nextButton, "Next profile", "Switches the active input and UI profile without changing the current bars.")
    self.actionMapNextProfileButton = nextButton

    local updateButton = createActionButton(profileRow, "Capture Current", 112)
    updateButton:SetPoint("TOPRIGHT", -8, -8)
    updateButton:SetScript("OnClick", function() self:ConfirmCaptureDestinationProfile() end)
    addButtonTooltip(updateButton, "Capture current setup", "Replaces this profile's inputs and Blizzard UI settings with the current setup. Learned placements stay intact.")
    self.actionMapCaptureButton = updateButton

    local newButton = createActionButton(profileRow, "New", 56)
    newButton:SetPoint("BOTTOMLEFT", 8, 8)
    newButton:SetScript("OnClick", function() self:ShowNewDestinationProfile() end)
    addButtonTooltip(newButton, "New profile", "Captures the current bound inputs and UI into a separate named profile.")
    self.actionMapNewProfileButton = newButton

    local renameButton = createActionButton(profileRow, "Rename", 68)
    renameButton:SetPoint("LEFT", newButton, "RIGHT", 6, 0)
    renameButton:SetScript("OnClick", function() self:ShowRenameDestinationProfile() end)
    addButtonTooltip(renameButton, "Rename profile", "Changes only the local display name.")
    self.actionMapRenameProfileButton = renameButton

    local deleteButton = createActionButton(profileRow, "Delete", 62)
    deleteButton:SetPoint("LEFT", renameButton, "RIGHT", 6, 0)
    deleteButton:SetScript("OnClick", function() self:ConfirmDeleteDestinationProfile() end)
    addButtonTooltip(deleteButton, "Delete profile", "Keeps shared learning. Undo Change can restore the profile unless an active layout needs it.")
    self.actionMapDeleteProfileButton = deleteButton

    local restoreButton = createActionButton(profileRow, "Undo Change", 94)
    restoreButton:SetPoint("LEFT", deleteButton, "RIGHT", 6, 0)
    restoreButton:SetScript("OnClick", function() self:UndoLastProfileChange() end)
    addButtonTooltip(restoreButton, "Undo the last profile change", "Restores the state before the latest create, rename, delete, capture, or import.")
    self.actionMapRestoreButton = restoreButton

    local importButton = createActionButton(profileRow, "Import", 70)
    importButton:SetPoint("BOTTOMRIGHT", -8, 8)
    importButton:SetScript("OnClick", function() self:ShowProfileImport() end)
    addButtonTooltip(importButton, "Import a profile", "Adds a profile and merges its portable learning with your local data. Undo Change rolls back the import.")
    self.actionMapImportButton = importButton

    local exportButton = createActionButton(profileRow, "Export", 70)
    exportButton:SetPoint("RIGHT", importButton, "LEFT", -6, 0)
    exportButton:SetScript("OnClick", function() self:ShowProfileExport() end)
    addButtonTooltip(exportButton, "Export this profile", "Copies this profile and its portable learning without character names, snapshots, or macro text.")
    self.actionMapExportButton = exportButton

    local inputHeader = panel:CreateFontString(nil, "OVERLAY", "GameFontDisableSmall")
    inputHeader:SetPoint("TOPLEFT", 28, -154)
    inputHeader:SetText("KEY")
    local purposeHeader = panel:CreateFontString(nil, "OVERLAY", "GameFontDisableSmall")
    purposeHeader:SetPoint("TOPLEFT", 168, -154)
    purposeHeader:SetText("LEARNED USE")
    local actionHeader = panel:CreateFontString(nil, "OVERLAY", "GameFontDisableSmall")
    actionHeader:SetPoint("TOPLEFT", 366, -154)
    actionHeader:SetText("CURRENT ACTION")

    local scroll = CreateFrame("ScrollFrame", "LamdaUIActionMapScrollFrame", panel, "UIPanelScrollFrameTemplate")
    scroll:SetPoint("TOPLEFT", 18, -174)
    scroll:SetPoint("BOTTOMRIGHT", -30, 8)
    local scrollChild = CreateFrame("Frame", nil, scroll)
    scrollChild:SetSize(710, 1)
    scroll:SetScrollChild(scrollChild)
    self.actionMapContainer = scrollChild
    self.actionMapRows = {}

    local emptyText = panel:CreateFontString(nil, "OVERLAY", "GameFontDisable")
    emptyText:SetPoint("TOPLEFT", 28, -196)
    emptyText:SetPoint("RIGHT", -28, 0)
    emptyText:SetJustifyH("LEFT")
    emptyText:SetText("No bound inputs are saved in this profile. Select Capture Current to add them.")
    emptyText:Hide()
    self.actionMapEmptyText = emptyText
end

function Addon:EnsureActionMapRows(count)
    if not self.actionMapContainer then
        return
    end
    for index = #self.actionMapRows + 1, count do
        self.actionMapRows[index] = createMapRow(self, self.actionMapContainer, index)
    end
    self.actionMapContainer:SetHeight(math.max(1, count) * 38)
end

function Addon:CreateLearningPanel(parent)
    local panel = CreateFrame("Frame", nil, parent)
    panel:SetAllPoints()
    panel:Hide()
    self.learningPanel = panel

    createSectionTitle(panel, "Preview", -8)
    local description = panel:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    description:SetPoint("TOPLEFT", 18, -35)
    description:SetPoint("RIGHT", -18, 0)
    description:SetJustifyH("LEFT")
    description:SetText("Build a layout, review every proposed change, then apply it with a saved Undo point.")
    description:SetTextColor(unpack(colors.muted))

    self.learningValues = {
        context = createSummaryRow(panel, -62, "Current context"),
        setups = createSummaryRow(panel, -102, "Learning"),
        state = createSummaryRow(panel, -142, "Preview status"),
    }

    local previewButton = createActionButton(panel, "Build Preview", 130)
    previewButton:SetPoint("TOPLEFT", 18, -196)
    previewButton:SetScript("OnClick", function()
        self:PreviewLayoutPlan()
    end)
    addButtonTooltip(previewButton, "Build a preview", "Uses your learned preferences to propose actions and Blizzard UI settings without changing anything.")
    self.learningPreviewButton = previewButton

    local applyButton = createActionButton(panel, "Apply", 92)
    applyButton:SetPoint("LEFT", previewButton, "RIGHT", 10, 0)
    applyButton:SetScript("OnClick", function()
        self:ConfirmApplyLayout()
    end)
    addButtonTooltip(applyButton, "Apply the preview", "Applies only a complete, restorable preview after saving its exact before-state.")
    self.learningApplyButton = applyButton

    local undoButton = createActionButton(panel, "Undo", 82)
    undoButton:SetPoint("LEFT", applyButton, "RIGHT", 10, 0)
    undoButton:SetScript("OnClick", function()
        self:UndoLayoutTransaction()
    end)
    addButtonTooltip(undoButton, "Undo the applied layout", "Restores the most recent applied layout when doing so will not overwrite newer manual changes.")
    self.learningUndoButton = undoButton

    local historyButton = createActionButton(panel, "History", 92)
    historyButton:SetPoint("LEFT", undoButton, "RIGHT", 10, 0)
    historyButton:SetScript("OnClick", function()
        self:ShowLayoutHistory()
    end)
    addButtonTooltip(historyButton, "Layout history", "Shows recent Apply, Undo, and recovery results. History is informational and does not change the UI.")
    self.learningHistoryButton = historyButton

    local coverageFrame = CreateFrame("Frame", nil, panel)
    coverageFrame:SetPoint("LEFT", historyButton, "RIGHT", 10, 0)
    coverageFrame:SetPoint("RIGHT", -18, 0)
    coverageFrame:SetHeight(28)
    coverageFrame:EnableMouse(true)
    local coverage = coverageFrame:CreateFontString(nil, "OVERLAY", "GameFontDisableSmall")
    coverage:SetAllPoints()
    coverage:SetJustifyH("LEFT")
    self.learningCoverageText = coverage
    coverageFrame:SetScript("OnEnter", function(current)
        local summary = self.trainingCoverage or (self.layoutPlan and self.layoutPlan.coverage)
        GameTooltip:SetOwner(current, "ANCHOR_RIGHT")
        GameTooltip:AddLine("Preview coverage", unpack(colors.accent))
        if not summary or summary.targetPlacements == 0 then
            GameTooltip:AddLine("Build a preview to inspect placement coverage.", unpack(colors.muted), true)
        else
            GameTooltip:AddDoubleLine("Eligible abilities", tostring(summary.eligibleActions or 0), 0.7, 0.75, 0.82, 1, 1, 1)
            GameTooltip:AddDoubleLine("Managed inputs", tostring(summary.destinationCount or 0), 0.7, 0.75, 0.82, 1, 1, 1)
            GameTooltip:AddDoubleLine("Proposed placements", tostring(summary.recommendedPlacements or 0), 0.7, 0.75, 0.82, 1, 1, 1)
            if (summary.fallbackPlacements or 0) > 0 then
                GameTooltip:AddDoubleLine("Placements to review", tostring(summary.fallbackPlacements), 0.7, 0.75, 0.82, 1, 0.78, 0.25)
                GameTooltip:AddLine("These placements complete the profile when no stronger match exists. Review them before Apply.", unpack(colors.muted), true)
            end
            if (summary.broadRoleFallbacks or 0) > 0 then
                GameTooltip:AddDoubleLine("General ability matches", tostring(summary.broadRoleFallbacks), 0.7, 0.75, 0.82, 1, 0.78, 0.25)
                GameTooltip:AddLine("Teach a similar setup to give these abilities a more specific placement.", unpack(colors.muted), true)
            end
            local shown = 0
            for _, missing in ipairs(summary.unassignedActions or {}) do
                if shown < 8 then
                    local name = missing.action and missing.action.actionName or "Unknown ability"
                    local roles = missing.action and missing.action.roles and table.concat(missing.action.roles, ", ")
                    local detail = tostring(missing.reason or "Not assigned") .. (roles and (" • " .. roles) or "")
                    GameTooltip:AddLine(name .. " — " .. detail, unpack(colors.muted), true)
                    shown = shown + 1
                end
            end
            local remaining = #(summary.unassignedActions or {}) - shown
            if remaining > 0 then
                GameTooltip:AddLine("+" .. remaining .. " more unassigned abilities", unpack(colors.muted))
            end
        end
        GameTooltip:Show()
    end)
    coverageFrame:SetScript("OnLeave", GameTooltip_Hide)
    self.learningCoverageFrame = coverageFrame

    createSectionTitle(panel, "Changes", -246)
    local scroll = CreateFrame("ScrollFrame", "LamdaUILayoutScrollFrame", panel, "UIPanelScrollFrameTemplate")
    scroll:SetPoint("TOPLEFT", 18, -270)
    scroll:SetPoint("BOTTOMRIGHT", -30, 8)
    local scrollChild = CreateFrame("Frame", nil, scroll)
    scrollChild:SetSize(710, 1)
    scroll:SetScrollChild(scrollChild)
    self.learningScrollFrame = scroll
    self.learningRecommendationContainer = scrollChild
    self.learningRecommendationLines = {}
end

function Addon:EnsureLearningRecommendationLines(count)
    if not self.learningRecommendationContainer then
        return
    end
    for index = #self.learningRecommendationLines + 1, math.max(1, count) do
        local row = CreateFrame("Frame", nil, self.learningRecommendationContainer)
        row:SetPoint("TOPLEFT", 0, -((index - 1) * 27))
        row:SetSize(710, 25)
        row:EnableMouse(true)
        local line = row:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
        line:SetPoint("LEFT", 7, 0)
        line:SetPoint("RIGHT", -7, 0)
        line:SetJustifyH("LEFT")
        line:SetWordWrap(false)
        line.hoverRow = row
        row:SetScript("OnEnter", function(current)
            local entry = current.entry
            if not entry then return end
            local isUIEntry = entry.kind == "edit-mode" or entry.kind == "cvar"
                or entry.kind == "modified-click" or entry.kind == "cooldown-viewer"
            local label = isUIEntry and entry.label or entry.destination.label
            GameTooltip:SetOwner(current, "ANCHOR_RIGHT")
            GameTooltip:AddLine(displayText(label), unpack(colors.accent))
            if entry.blockedReason then
                GameTooltip:AddLine(displayText(entry.blockedReason), unpack(colors.warning))
            elseif isUIEntry then
                GameTooltip:AddLine("Blizzard UI setting", unpack(colors.muted))
                local before = entry.before and (entry.before.display or entry.before.name or entry.before.value)
                local desired = entry.desired and (entry.desired.display or entry.desired.name or entry.desired.value)
                GameTooltip:AddDoubleLine("Current", displayText(before or "Unavailable"),
                    0.7, 0.75, 0.82, 1, 1, 1)
                GameTooltip:AddDoubleLine("After Apply", displayText(desired or "Unavailable"),
                    0.7, 0.75, 0.82, 1, 1, 1)
                if entry.kind == "edit-mode" and entry.desired and entry.desired.install then
                    GameTooltip:AddLine("Apply will install this portable layout. Undo removes it only while its imported settings remain unchanged.",
                        unpack(colors.muted), true)
                end
            else
                GameTooltip:AddDoubleLine("Confidence", entry.confidence or "—", 0.7, 0.75, 0.82, 1, 1, 1)
                local semanticAction = entry.action or entry.desired
                if semanticAction and semanticAction.roles then
                    GameTooltip:AddDoubleLine("Ability type", table.concat(semanticAction.roles, ", "), 0.7, 0.75, 0.82, 1, 1, 1)
                end
                if semanticAction and semanticAction.roleSources and #semanticAction.roleSources > 0 then
                    GameTooltip:AddDoubleLine("Inferred from", table.concat(semanticAction.roleSources, ", "), 0.7, 0.75, 0.82, 1, 1, 1)
                end
                if entry.reason and entry.reason ~= "" then
                    GameTooltip:AddLine(displayText(entry.reason), unpack(colors.muted), true)
                end
            end
            GameTooltip:Show()
        end)
        row:SetScript("OnLeave", GameTooltip_Hide)
        self.learningRecommendationLines[index] = line
    end
end

function Addon:CreateUI()
    if self.frame then
        return
    end

    local frame = CreateFrame("Frame", "LamdaUIFrame", UIParent, "BackdropTemplate")
    frame:SetSize(800, 600)
    frame:SetPoint("CENTER")
    frame:SetFrameStrata("DIALOG")
    frame:SetClampedToScreen(true)
    frame:SetMovable(true)
    frame:EnableMouse(true)
    frame:RegisterForDrag("LeftButton")
    frame:SetScript("OnDragStart", frame.StartMoving)
    frame:SetScript("OnDragStop", frame.StopMovingOrSizing)
    frame:SetBackdrop({
        bgFile = "Interface\\DialogFrame\\UI-DialogBox-Background-Dark",
        edgeFile = "Interface\\DialogFrame\\UI-DialogBox-Border",
        tile = true,
        tileSize = 32,
        edgeSize = 24,
        insets = { left = 7, right = 7, top = 7, bottom = 7 },
    })
    frame:SetBackdropColor(0.03, 0.04, 0.055, 0.98)
    frame:Hide()
    self.frame = frame

    if UISpecialFrames then
        table.insert(UISpecialFrames, frame:GetName())
    end

    local title = frame:CreateFontString(nil, "OVERLAY", "GameFontNormalHuge")
    title:SetPoint("TOPLEFT", 22, -18)
    title:SetText("lamdaUI")
    title:SetTextColor(unpack(colors.accent))

    local closeButton = CreateFrame("Button", nil, frame, "UIPanelCloseButton")
    closeButton:SetPoint("TOPRIGHT", -5, -5)

    local scanButton = createActionButton(frame, "Scan", 84)
    scanButton:SetPoint("TOPRIGHT", -44, -18)
    scanButton:SetScript("OnClick", function()
        self:CaptureSnapshot("manual scan")
    end)
    addButtonTooltip(scanButton, "Scan current setup", "Refreshes the active specialization, form, talents, action bars, bindings, and spellbook without changing them.")

    local subtitle = frame:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    subtitle:SetPoint("TOPLEFT", title, "BOTTOMLEFT", 0, -5)
    subtitle:SetPoint("RIGHT", scanButton, "LEFT", -12, 0)
    subtitle:SetJustifyH("LEFT")
    self.subtitle = subtitle

    self.tabs = {}
    local tabNames = {
        { id = "custom", label = "Custom Addons" },
        { id = "setup", label = "Overview" },
        { id = "map", label = "Profiles" },
        { id = "layout", label = "Preview" },
    }
    for index, tabInfo in ipairs(tabNames) do
        local tab = createActionButton(frame, tabInfo.label, 112)
        tab:SetPoint("TOPLEFT", 22 + ((index - 1) * 118), -66)
        tab:SetScript("OnClick", function()
            self:SelectTab(tabInfo.id)
        end)
        self.tabs[tabInfo.id] = tab
    end

    local content = CreateFrame("Frame", nil, frame)
    content:SetPoint("TOPLEFT", 14, -98)
    content:SetPoint("BOTTOMRIGHT", -14, 42)
    self.content = content
    if self.CreateCustomHubPanel then self:CreateCustomHubPanel(content) end
    self:CreateSetupPanel(content)
    self:CreateActionMapPanel(content)
    self:CreateLearningPanel(content)

    local status = frame:CreateFontString(nil, "OVERLAY", "GameFontDisableSmall")
    status:SetPoint("BOTTOMLEFT", 22, 15)
    status:SetPoint("RIGHT", -22, 0)
    status:SetJustifyH("LEFT")
    self.statusText = status

    self:InitializeLayoutPopups()
    self:InitializeDestinationPopups()
    self:InitializePortablePopups()

    self:SelectTab("custom")
    self:RefreshUI()
end

function Addon:SelectTab(tabID)
    self.selectedTab = tabID
    if self.customHubPanel then self.customHubPanel:SetShown(tabID == "custom") end
    if self.setupPanel then
        self.setupPanel:SetShown(tabID == "setup")
    end
    if self.actionMapPanel then
        self.actionMapPanel:SetShown(tabID == "map")
    end
    if self.learningPanel then
        self.learningPanel:SetShown(tabID == "layout")
    end
    for id, button in pairs(self.tabs or {}) do
        button:SetEnabled(id ~= tabID)
    end
end

function Addon:RefreshSetup()
    local snapshot = self.currentSnapshot
    if not self.setupValues then
        return
    end

    if not snapshot then
        setText(self.setupValues.context, "Not scanned", "neutral")
        setText(self.setupValues.profile,
            self.destinationProfile and self.destinationProfile.name or "No active profile",
            self.destinationProfile and "neutral" or "warning")
        setText(self.setupValues.learned, "Not scanned", "neutral")
        setText(self.setupValues.plan, "Not built yet", "neutral")
        setText(self.setupNextTitle, "Scan the current character", "change")
        self.setupNextDetail:SetText("Select Scan to read the active specialization, abilities, bars, and bindings.")
        self.setupLearnButton:SetEnabled(false)
        self.setupPreviewButton:SetEnabled(false)
        self.setupPreviewButton:SetText("Build Preview")
        self.setupSafety:SetText("Nothing changes during a scan")
        return
    end

    local summary = self.GetTrainingSummary and self:GetTrainingSummary() or {}
    local learned, placementCount, context = false, describePlacementCount(self, snapshot), nil
    if self.GetTrainingSetupState then
        learned, placementCount, context = self:GetTrainingSetupState(snapshot)
    end

    local formName = snapshot.actionBarState and snapshot.actionBarState.formName
    local contextText = snapshot.specialization or "No specialization"
    if formName and formName ~= "Caster" then
        contextText = contextText .. " • " .. formName
    end
    if self.destinationProfile and self.destinationProfile.name then
        setText(self.setupValues.profile, self.destinationProfile.name, "neutral")
    else
        setText(self.setupValues.profile, "No active profile", "warning")
    end
    setText(self.setupValues.context, contextText, "neutral")

    if context and context.overrideActive then
        setText(self.setupValues.learned, "Temporary action bar active", "warning")
    elseif learned then
        setText(self.setupValues.learned, "Learned for this specialization and form", "ok")
    elseif placementCount > 0 then
        setText(self.setupValues.learned, "Ready to learn", "change")
    else
        setText(self.setupValues.learned, "No readable actions yet", "warning")
    end

    local plan, transaction = self:GetLayoutTransactionState()
    if plan ~= self.lastRenderedLayoutPlan then
        self.lastRenderedLayoutPlan = plan
        if self.learningScrollFrame and self.learningScrollFrame.SetVerticalScroll then
            self.learningScrollFrame:SetVerticalScroll(0)
        end
    end
    local transactionIncomplete = transaction and (transaction.status == "undo-incomplete"
        or transaction.status == "recovery-incomplete"
        or transaction.status == "apply-failed-restore-incomplete")
    if transaction and transaction.status == "applied" then
        setText(self.setupValues.plan, "Applied • Undo available", "ok")
    elseif transactionIncomplete then
        setText(self.setupValues.plan, "Recovery needed", "warning")
    elseif transaction and (transaction.status == "staged" or transaction.status == "applying") then
        setText(self.setupValues.plan, "Waiting to apply", "change")
    elseif plan and plan.ready then
        setText(self.setupValues.plan, countLabel(plan.changeCount, "change") .. " ready to review", "ok")
    elseif plan and plan.blockedCount > 0 then
        setText(self.setupValues.plan, countLabel(plan.blockedCount, "blocked change"), "warning")
    elseif plan then
        setText(self.setupValues.plan, "No changes proposed", "neutral")
    else
        setText(self.setupValues.plan, "Not built yet", "neutral")
    end

    local inCombat = InCombatLockdown and InCombatLockdown()
    local overrideActive = context and context.overrideActive
    local destinationCount = #(self.GetManagedDestinations and self:GetManagedDestinations() or {})
    self.setupLearnButton:SetEnabled(not inCombat and not overrideActive and placementCount > 0)
    self.setupPreviewButton:SetEnabled(not inCombat and destinationCount > 0)
    self.setupPreviewButton:SetText(plan and "Review Preview" or "Build Preview")

    local nextTitle, nextDetail, nextKind
    if transactionIncomplete then
        nextTitle = "Recovery needs attention"
        nextDetail = "Open Preview and use Undo to restore the saved layout."
        nextKind = "warning"
    elseif transaction and transaction.status == "applied" then
        nextTitle = "Layout applied"
        nextDetail = "The previous layout is still available from Undo in Preview."
        nextKind = "ok"
    elseif plan and plan.ready then
        nextTitle = "Preview ready"
        nextDetail = "Review the exact changes in Preview before applying them."
        nextKind = "ok"
    elseif plan and plan.blockedCount > 0 then
        nextTitle = "Preview needs attention"
        nextDetail = "Open Preview to see which changes cannot be restored safely."
        nextKind = "warning"
    elseif overrideActive then
        nextTitle = "Temporary action bar active"
        nextDetail = "Leave the vehicle or temporary state, then Scan again."
        nextKind = "warning"
    elseif placementCount == 0 then
        nextTitle = "No actions found"
        nextDetail = "Put abilities on bound action bars or Click Casting, then Scan again."
        nextKind = "warning"
    elseif learned then
        nextTitle = "This setup is learned"
        nextDetail = "Build a preview whenever you want lamdaUI to arrange this context."
        nextKind = "ok"
    else
        nextTitle = "Teach this setup"
        nextDetail = "If these placements feel right, select Learn Current."
        nextKind = "change"
    end
    setText(self.setupNextTitle, nextTitle, nextKind)
    self.setupNextDetail:SetText(nextDetail)

    if inCombat then
        self.setupSafety:SetText("Changes are paused during combat")
    else
        self.setupSafety:SetText("Review before Apply • Undo is saved locally")
    end
end

function Addon:RefreshActionMap()
    local snapshot = self.currentSnapshot
    local destinations = self.GetManagedDestinations and self:GetManagedDestinations() or {}
    local profiles = self.GetDestinationProfiles and self:GetDestinationProfiles() or {}
    local profile = self.GetActiveDestinationProfile and self:GetActiveDestinationProfile() or self.destinationProfile
    if self.actionMapProfileName then
        self.actionMapProfileName:SetText(profile
            and (profile.name .. " • " .. #destinations .. " inputs")
            or "No active profile")
    end
    if self.actionMapEmptyText then
        self.actionMapEmptyText:SetShown(#destinations == 0)
    end
    self:EnsureActionMapRows(#destinations)
    for index, row in ipairs(self.actionMapRows or {}) do
        local destination = destinations[index]
        if destination then
            local data = snapshot and snapshot.destinations and snapshot.destinations[destination.id]
            local learnedPurpose = self.GetDestinationRoleSummary
                and self:GetDestinationRoleSummary(destination.id, 2)
            row.destination = destination
            row.inputText:SetText(displayText(destination.label or destination.inputKey))
            row.purposeText:SetText(displayText(learnedPurpose
                or destination.category
                or (destination.kind == "click-binding" and "Click Casting")
                or (data and data.actionSlot and "Action bar" or "Direct spell")))
            row.actionText:SetText(displayText(data and data.actionName or "Not scanned"))
            row:Show()
        else
            row.destination = nil
            row:Hide()
        end
    end
    local inCombat = InCombatLockdown and InCombatLockdown()
    if self.actionMapCaptureButton then
        self.actionMapCaptureButton:SetEnabled(not inCombat and profile ~= nil)
    end
    if self.actionMapImportButton then
        self.actionMapImportButton:SetEnabled(not inCombat)
    end
    if self.actionMapExportButton then
        self.actionMapExportButton:SetEnabled(profile ~= nil and #destinations > 0)
    end
    if self.actionMapNewProfileButton then self.actionMapNewProfileButton:SetEnabled(not inCombat) end
    if self.actionMapRenameProfileButton then self.actionMapRenameProfileButton:SetEnabled(not inCombat and profile ~= nil) end
    local transaction = self.db and self.db.layoutTransactions and self.db.layoutTransactions.active
    local profileInTransaction = type(transaction) == "table" and type(transaction.context) == "table" and profile
        and transaction.context.profileID == profile.id
    if self.actionMapDeleteProfileButton then
        self.actionMapDeleteProfileButton:SetEnabled(not inCombat and #profiles > 1 and not profileInTransaction)
    end
    if self.actionMapPreviousProfileButton then self.actionMapPreviousProfileButton:SetEnabled(not inCombat and #profiles > 1) end
    if self.actionMapNextProfileButton then self.actionMapNextProfileButton:SetEnabled(not inCombat and #profiles > 1) end
    if self.actionMapRestoreButton then
        local canUndo = self.CanUndoLastProfileChange and self:CanUndoLastProfileChange()
        if not self.CanUndoLastProfileChange then
            local history = self.db and self.db.profileChangeHistory
            canUndo = self.db and ((history and #history > 0) or self.db.profileImportBackup ~= nil)
        end
        self.actionMapRestoreButton:SetEnabled(not inCombat and canUndo and true or false)
    end
end

function Addon:RefreshLearning()
    if not self.learningValues or not self.GetTrainingSummary then
        return
    end

    local summary = self:GetTrainingSummary()
    local snapshot = self.currentSnapshot
    local context = snapshot and self:GetTrainingContext(snapshot)
    if context then
        local label = (snapshot.specialization or context.specKey) .. " • "
            .. (context.stateLabel or context.stateKey)
        if self.destinationProfile and self.destinationProfile.name then
            label = label .. " • " .. self.destinationProfile.name
        end
        setText(self.learningValues.context, label, context.overrideActive and "warning" or "neutral")
    else
        setText(self.learningValues.context, "Scan a character to begin", "neutral")
    end
    local activeSetups = summary.profileConfirmedSetups or summary.confirmedSetups or 0
    local learnedAbilities = tonumber(summary.learnedActions) or 0
    setText(self.learningValues.setups,
        countLabel(activeSetups, "setup") .. " • " .. countLabel(learnedAbilities, "ability", "abilities"),
        activeSetups > 0 and "ok" or "neutral")

    local plan, transaction = self:GetLayoutTransactionState()
    local transactionIncomplete = transaction and (transaction.status == "undo-incomplete"
        or transaction.status == "recovery-incomplete"
        or transaction.status == "apply-failed-restore-incomplete")
    if transactionIncomplete then
        setText(self.learningValues.state, "Recovery needed • use Undo", "warning")
    elseif transaction and transaction.status == "applied" then
        setText(self.learningValues.state, "Applied • Undo available", "ok")
    elseif transaction and (transaction.status == "staged" or transaction.status == "applying") then
        setText(self.learningValues.state, "Waiting to apply", "change")
    elseif plan and plan.ready then
        setText(self.learningValues.state, countLabel(plan.changeCount, "change") .. " ready", "ok")
    elseif plan and plan.blockedCount > 0 then
        setText(self.learningValues.state, countLabel(plan.blockedCount, "blocked change"), "warning")
    elseif plan then
        setText(self.learningValues.state, "No changes proposed", "neutral")
    else
        setText(self.learningValues.state, "Not built yet", "neutral")
    end

    local proposed = {}
    for _, entry in ipairs(plan and plan.uiEntries or {}) do
        table.insert(proposed, entry)
    end
    for _, entry in ipairs(plan and plan.entries or {}) do
        if entry.blockedReason or not entry.unchanged then
            table.insert(proposed, entry)
        end
    end
    self:EnsureLearningRecommendationLines(#proposed)
    if self.learningRecommendationContainer then
        self.learningRecommendationContainer:SetHeight(math.max(1, #proposed) * 27)
    end
    for index, line in ipairs(self.learningRecommendationLines or {}) do
        local entry = proposed[index]
        local hoverRow = line.hoverRow
        if hoverRow then hoverRow.entry = entry end
        if entry then
            local isUIEntry = entry.kind == "edit-mode" or entry.kind == "cvar"
                or entry.kind == "modified-click" or entry.kind == "cooldown-viewer"
            local label = displayText(isUIEntry and entry.label or entry.destination.label)
            local desired = displayText(isUIEntry and (entry.desired.display or entry.desired.name or entry.desired.value) or entry.desired.actionName)
            local before = displayText(isUIEntry and (entry.before.display or entry.before.name or entry.before.value)
                or (entry.before and (entry.before.actionName or entry.before.command))
                or "Empty")
            if entry.blockedReason then
                line:SetText(string.format("%d. %s  ->  %s   |cffffe08cBlocked: %s|r", index, label,
                    tostring(desired or "Unavailable"), displayText(entry.blockedReason)))
                line:SetTextColor(unpack(colors.warning))
            else
                local detail = isUIEntry and "UI" or (entry.confidence or "")
                line:SetText(string.format("%d. %s   %s  ->  %s   |cff8998a8%s|r", index, label, tostring(before or "Unavailable"), tostring(desired or "Unavailable"), detail))
                line:SetTextColor(1, 1, 1)
            end
            line:Show()
            if hoverRow then hoverRow:Show() end
        elseif index == 1 then
            line:SetText("Build a preview to see every proposed action and UI change.")
            line:SetTextColor(unpack(colors.muted))
            line:Show()
            if hoverRow then hoverRow:Show() end
        else
            line:SetText("")
            line:Hide()
            if hoverRow then hoverRow:Hide() end
        end
    end

    local inCombat = InCombatLockdown and InCombatLockdown()
    self.learningPreviewButton:SetEnabled(not inCombat)
    self.learningApplyButton:SetEnabled(not inCombat and plan and plan.ready and (not transaction or transaction.status == "applied"))
    self.learningUndoButton:SetEnabled(not inCombat and transaction and (transaction.status == "applied" or transactionIncomplete))
    if self.learningHistoryButton then
        local history = self.GetLayoutTransactionHistory and self:GetLayoutTransactionHistory(1) or {}
        self.learningHistoryButton:SetEnabled(#history > 0)
        self.learningHistoryButton:SetText(transactionIncomplete and "Recovery" or "History")
    end
    if self.learningCoverageText then
        local coverage = self.trainingCoverage or (plan and plan.coverage)
        if coverage and coverage.targetPlacements > 0 then
            local kind = coverage.complete and "ok" or "warning"
            local suffix = coverage.complete and "mapped" or ("mapped • " .. coverage.unfilledTargetCount .. " unmapped")
            setText(self.learningCoverageText,
                coverage.recommendedPlacements .. "/" .. coverage.targetPlacements .. " " .. suffix,
                kind)
        else
            setText(self.learningCoverageText, "Build a preview to measure coverage", "neutral")
        end
    end
end

-- Training predates the production UI names. Keep one compatibility entry
-- point so learning and preview events refresh both relevant views.
function Addon:RefreshTraining()
    self:RefreshSetup()
    self:RefreshLearning()
end

function Addon:RefreshUI()
    if not self.frame then
        return
    end

    local snapshot = self.currentSnapshot
    if snapshot then
        local barState = snapshot.actionBarState
        local formText = barState and barState.formName and barState.formName ~= "Caster" and (" • " .. barState.formName) or ""
        local overrideText = barState and barState.overrideActive and " • temporary bar active" or ""
        local profileText = self.destinationProfile and self.destinationProfile.name
            and (" • " .. self.destinationProfile.name) or ""
        self.subtitle:SetText(displayText(snapshot.character .. "–" .. snapshot.realm .. " • " .. snapshot.specialization
            .. formText .. profileText .. overrideText))
    else
        self.subtitle:SetText("Scan a character to begin")
    end

    if self.RefreshCustomHub then self:RefreshCustomHub() end
    self:RefreshSetup()
    self:RefreshActionMap()
    self:RefreshLearning()
    self:UpdateStatusText()
end

function Addon:UpdateStatusText()
    if self.statusText then
        self.statusText:SetText(displayText(self.status or "Ready"))
    end
end

function Addon:Show(tabID)
    if not self.frame then
        return
    end
    self:CaptureSnapshot("window opened")
    self:SelectTab(tabID or self.selectedTab or "setup")
    self.frame:Show()
end

function Addon:Hide()
    if self.frame then
        self.frame:Hide()
    end
end

function Addon:Toggle()
    if not self.frame then
        return
    end
    if self.frame:IsShown() then
        self:Hide()
    else
        self:Show("setup")
    end
end
