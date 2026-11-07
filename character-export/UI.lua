local _, E = ...

function E.CreateWindow()
    if E.window then return E.window end
    local frame = CreateFrame("Frame", "LamdaExportWindow", UIParent, "BasicFrameTemplateWithInset")
    frame:SetSize(780, 530)
    frame:SetPoint("CENTER")
    frame:SetFrameStrata("DIALOG")
    frame:SetClampedToScreen(true)
    frame:SetMovable(true)
    frame:EnableMouse(true)
    frame:RegisterForDrag("LeftButton")
    frame:SetScript("OnDragStart", frame.StartMoving)
    frame:SetScript("OnDragStop", frame.StopMovingOrSizing)
    frame.TitleText:SetText("LamdaExport")
    frame:Hide()
    local hint = frame:CreateFontString(nil, "OVERLAY", "GameFontHighlight")
    hint:SetPoint("TOPLEFT", 18, -38)
    hint:SetText("Ctrl+C to copy your character snapshot and close this window.")
    local status = frame:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
    status:SetPoint("TOPLEFT", 18, -62)
    status:SetText("Collecting character data...")
    local scroll = CreateFrame("ScrollFrame", nil, frame, "UIPanelScrollFrameTemplate")
    scroll:SetPoint("TOPLEFT", 18, -88)
    scroll:SetPoint("BOTTOMRIGHT", -38, 20)
    local edit = CreateFrame("EditBox", nil, scroll)
    edit:SetMultiLine(true)
    edit:SetAutoFocus(false)
    edit:SetMaxLetters(0)
    if edit.SetMaxBytes then edit:SetMaxBytes(0) end
    edit:SetFontObject(ChatFontNormal or GameFontHighlightSmall)
    edit:SetWidth(716)
    edit:SetHeight(1)
    scroll:SetScrollChild(edit)
    edit:SetScript("OnEscapePressed", function() frame:Hide() end)
    edit:SetScript("OnEditFocusGained", function(self)
        if frame.ready then self:HighlightText(0, -1) end
    end)
    edit:SetScript("OnTextChanged", function(self, userInput)
        if userInput and frame.ready and not frame.restoring then
            frame.restoring = true
            self:SetText(frame.exportText)
            self:HighlightText(0, -1)
            frame.restoring = nil
        end
    end)
    edit:SetScript("OnKeyDown", function(self, key)
        if key == "C" and frame.ready and (IsControlKeyDown() or (IsMetaKeyDown and IsMetaKeyDown())) then
            self:HighlightText(0, -1)
            frame.copyPending = frame.generation
        end
    end)
    edit:SetScript("OnKeyUp", function(_, key)
        if key ~= "C" or not frame.copyPending then return end
        local generation = frame.copyPending
        frame.copyPending = nil
        -- Native EditBox handles the actual clipboard copy. Hiding on key-up,
        -- with a short deferred close, leaves focus intact for that native action.
        C_Timer.After(0.1, function()
            if frame:IsShown() and frame.ready and frame.generation == generation then frame:Hide() end
        end)
    end)
    frame:SetScript("OnHide", function()
        frame.copyPending = nil
        edit:ClearFocus()
        E.CancelCapture()
    end)
    frame.edit, frame.status, frame.scroll = edit, status, scroll
    E.window = frame
    if UISpecialFrames then UISpecialFrames[#UISpecialFrames + 1] = "LamdaExportWindow" end
    return frame
end
function E.ShowProgress(message)
    local frame = E.CreateWindow()
    frame.generation = (frame.generation or 0) + 1
    frame.ready, frame.copyPending, frame.exportText = false, nil, nil
    frame.edit:ClearFocus()
    frame.edit:SetText("")
    frame.status:SetText(message or "Collecting character data...")
    frame:Show()
    return frame.generation
end
function E.ShowExport(text, snapshot, generation)
    local frame = E.CreateWindow()
    if generation and (frame.generation ~= generation or not frame:IsShown()) then return end
    frame.exportText, frame.ready = text, true
    frame.edit:SetText(text)
    frame.edit:SetCursorPosition(0)
    frame.edit:SetFocus()
    frame.edit:HighlightText(0, -1)
    frame.scroll:SetVerticalScroll(0)
    local name = snapshot.character and snapshot.character.name or "Character"
    frame.status:SetText(string.format("%s  ·  %s  ·  %s bytes", name, snapshot.client or "WoW", #text))
end
