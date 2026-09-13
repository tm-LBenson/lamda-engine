local addonName, Addon = ...

Addon.name = addonName
Addon.version = "0.20.0-alpha.2"

Addon.profile = {
    id = "local-action-map",
    name = "Default Action Profile",
    bindings = {},
}

local legacyAddonName = "LewisUI"
local legacyMigrationFormat = 1
local legacyConflictMessage = "An earlier lamdaUI prototype is also enabled. Use the migration prompt, or disable the older addon and reload. No saved data has been changed."
local legacyMigrationPrompt = "An earlier lamdaUI prototype is also enabled. Copy its local setup data into this version, disable the older addon, and reload now? The older SavedVariables file is kept as a rollback copy."

local function copySavedValue(value, state, depth)
    local valueType = type(value)
    if valueType == "nil" or valueType == "boolean" or valueType == "string" then
        return value
    end
    if valueType == "number" then
        if value ~= value or value == math.huge or value == -math.huge then
            return nil, "non-finite number"
        end
        return value
    end
    if valueType ~= "table" then
        return nil, "unsupported " .. valueType .. " value"
    end

    state = state or { active = {}, copies = {}, entries = 0, tables = 0 }
    depth = depth or 0
    if depth > 100 then return nil, "table nesting is too deep" end
    if state.active[value] then return nil, "cyclic table" end
    if state.copies[value] then return state.copies[value] end

    state.tables = state.tables + 1
    if state.tables > 50000 then return nil, "too many tables" end
    local copied = {}
    state.copies[value] = copied
    state.active[value] = true
    for key, nestedValue in pairs(value) do
        state.entries = state.entries + 1
        if state.entries > 1000000 then
            state.active[value] = nil
            return nil, "too many saved values"
        end
        local keyType = type(key)
        if keyType ~= "string" and keyType ~= "number" and keyType ~= "boolean" then
            state.active[value] = nil
            return nil, "unsupported table key"
        end
        if keyType == "number" and (key ~= key or key == math.huge or key == -math.huge) then
            state.active[value] = nil
            return nil, "non-finite table key"
        end
        local nestedCopy, copyError = copySavedValue(nestedValue, state, depth + 1)
        if copyError then
            state.active[value] = nil
            return nil, copyError
        end
        copied[key] = nestedCopy
    end
    state.active[value] = nil
    return copied
end

local function isAddonLoaded(name)
    if C_AddOns and C_AddOns.IsAddOnLoaded then
        return C_AddOns.IsAddOnLoaded(name) and true or false
    end
    if IsAddOnLoaded then return IsAddOnLoaded(name) and true or false end
    return false
end

local function reportLegacyConflict()
    Addon.legacyConflict = true
    print("|cffff6b6blamdaUI|r " .. legacyConflictMessage)
end

local function legacyMigrationFailure(message)
    local status = "Migration did not run • " .. tostring(message or "unknown error")
    Addon.legacyMigrationPromptShown = nil
    if Addon.SetStatus then Addon:SetStatus(status) end
    print("|cffff6b6blamdaUI|r " .. status)
    return false
end

function Addon:BeginLegacyMigration()
    if not self.legacyConflict then
        return legacyMigrationFailure("the older addon is not loaded")
    end
    if InCombatLockdown and InCombatLockdown() then
        return legacyMigrationFailure("leave combat and try again")
    end
    if type(LamdaUIDB) ~= "table" then
        return legacyMigrationFailure("the older saved data could not be read")
    end

    local disableAddOn = C_AddOns and C_AddOns.DisableAddOn or DisableAddOn
    local enableAddOn = C_AddOns and C_AddOns.EnableAddOn or EnableAddOn
    if type(disableAddOn) ~= "function" or type(ReloadUI) ~= "function" then
        return legacyMigrationFailure("the required client APIs are unavailable")
    end

    local copiedDatabase, copyError = copySavedValue(LamdaUIDB)
    if not copiedDatabase then
        return legacyMigrationFailure("the older saved data is not safe to copy (" .. tostring(copyError) .. ")")
    end

    local priorStage = LamdaUIMigrationDB
    LamdaUIMigrationDB = {
        formatVersion = legacyMigrationFormat,
        addonVersion = self.version,
        database = copiedDatabase,
    }

    local disabled, disableError = pcall(disableAddOn, legacyAddonName)
    if not disabled then
        LamdaUIMigrationDB = priorStage
        return legacyMigrationFailure("the older addon could not be disabled (" .. tostring(disableError) .. ")")
    end

    self:SetStatus("Migration prepared • Reloading the interface")
    local reloaded, reloadError = pcall(ReloadUI)
    if not reloaded then
        if type(enableAddOn) == "function" then pcall(enableAddOn, legacyAddonName) end
        LamdaUIMigrationDB = priorStage
        return legacyMigrationFailure("the interface could not reload (" .. tostring(reloadError) .. ")")
    end
    return true
end

function Addon:ShowLegacyMigrationPrompt()
    if self.legacyMigrationPromptShown then return true end
    if type(StaticPopupDialogs) ~= "table" or type(StaticPopup_Show) ~= "function" then
        return false
    end
    StaticPopupDialogs.LAMDAUI_LEGACY_MIGRATION = {
        text = legacyMigrationPrompt,
        button1 = "Migrate and Reload",
        button2 = CANCEL or "Not Now",
        OnAccept = function() Addon:BeginLegacyMigration() end,
        OnCancel = function()
            Addon.legacyMigrationPromptShown = nil
            if Addon.SetStatus then Addon:SetStatus("Migration postponed • No changes were made") end
        end,
        timeout = 0,
        whileDead = 1,
        hideOnEscape = 1,
        preferredIndex = 3,
    }
    self.legacyMigrationPromptShown = true
    StaticPopup_Show("LAMDAUI_LEGACY_MIGRATION")
    return true
end

local maximumContextSnapshotsPerCharacter = 256

local function compactStoredSnapshot(snapshot)
    local spellbook = type(snapshot.spellbook) == "table" and snapshot.spellbook or {}
    return {
        addonVersion = snapshot.addonVersion,
        capturedAt = snapshot.capturedAt,
        capturedAtText = snapshot.capturedAtText,
        reason = snapshot.reason,
        character = snapshot.character,
        realm = snapshot.realm,
        class = snapshot.class,
        specializationID = snapshot.specializationID,
        specialization = snapshot.specialization,
        profile = snapshot.profile,
        destinationProfile = snapshot.destinationProfile,
        destinationProfileName = snapshot.destinationProfileName,
        gameBuild = snapshot.gameBuild,
        destinations = snapshot.destinations,
        actionBarState = snapshot.actionBarState,
        spellbook = {
            available = spellbook.available,
            status = spellbook.status,
            activeConfigID = spellbook.activeConfigID,
            activeHeroSpecID = spellbook.activeHeroSpecID,
            numItems = spellbook.numItems,
        },
        audit = snapshot.audit,
    }
end

local function pruneContextSnapshots(value)
    local clean, ranked = {}, {}
    for contextKey, context in pairs(type(value) == "table" and value or {}) do
        if type(contextKey) == "string" and #contextKey <= 700 and type(context) == "table" then
            clean[contextKey] = context
            table.insert(ranked, {
                key = contextKey,
                capturedAt = tonumber(context.capturedAt) or 0,
            })
        end
    end
    table.sort(ranked, function(left, right)
        if left.capturedAt == right.capturedAt then return left.key < right.key end
        return left.capturedAt > right.capturedAt
    end)
    for index = maximumContextSnapshotsPerCharacter + 1, #ranked do
        clean[ranked[index].key] = nil
    end
    return clean
end

function Addon:GetCharacterIdentity()
    local characterName = UnitName("player") or "Unknown"
    local realmName = GetNormalizedRealmName and GetNormalizedRealmName() or GetRealmName() or "UnknownRealm"
    local characterKey = characterName .. "-" .. realmName
    local getSpecialization = C_SpecializationInfo and C_SpecializationInfo.GetSpecialization
        or GetSpecialization
    local getSpecializationInfo = C_SpecializationInfo and C_SpecializationInfo.GetSpecializationInfo
        or GetSpecializationInfo
    local specIndex = getSpecialization and getSpecialization()
    local specID, specName

    if specIndex and getSpecializationInfo then
        specID, specName = getSpecializationInfo(specIndex)
    end

    return {
        key = characterKey,
        character = characterName,
        realm = realmName,
        class = select(2, UnitClass("player")),
        specializationID = specID or 0,
        specialization = specName or "No specialization",
    }
end

function Addon:InitializeDatabase()
    local migrationStagePresent = LamdaUIMigrationDB ~= nil
    local migrationStageValid = type(LamdaUIMigrationDB) == "table"
        and LamdaUIMigrationDB.formatVersion == legacyMigrationFormat
        and type(LamdaUIMigrationDB.database) == "table"
    local currentSchema = type(LamdaUIDB) == "table" and tonumber(LamdaUIDB.schemaVersion) or 0
    local usedMigrationStage = false
    if migrationStageValid and currentSchema < 6 then
        local copiedDatabase = copySavedValue(LamdaUIMigrationDB.database)
        if copiedDatabase then
            LamdaUIDB = copiedDatabase
            usedMigrationStage = true
        end
    end

    LamdaUIDB = type(LamdaUIDB) == "table" and LamdaUIDB or {}
    LamdaUIDB.schemaVersion = 6
    LamdaUIDB.addonVersion = self.version
    LamdaUIDB.activeProfile = LamdaUIDB.activeProfile or self.profile.id
    LamdaUIDB.characters = type(LamdaUIDB.characters) == "table" and LamdaUIDB.characters or {}
    LamdaUIDB.history = type(LamdaUIDB.history) == "table" and LamdaUIDB.history or {}
    LamdaUIDB.options = type(LamdaUIDB.options) == "table" and LamdaUIDB.options or {}
    self.db = LamdaUIDB
    if self.InitializeCustomModules then self:InitializeCustomModules() end
    if self.InitializeDestinationProfile then
        self:InitializeDestinationProfile()
        LamdaUIDB.activeProfile = self.destinationProfile.id
    end
    if self.InitializeTrainingModel then
        self:InitializeTrainingModel()
    end
    if self.InitializeProfileChangeHistory then
        self:InitializeProfileChangeHistory()
    end
    if migrationStagePresent then
        LamdaUIMigrationDB = nil
        self.legacyMigrationCompleted = usedMigrationStage
    end
end

function Addon:SetStatus(message)
    self.status = message
    if self.UpdateStatusText then
        self:UpdateStatusText()
    end
end

function Addon:CaptureSnapshot(reason)
    if self.legacyConflict then
        self:SetStatus(legacyConflictMessage)
        return nil
    end
    if InCombatLockdown and InCombatLockdown() then
        self.scanAfterCombat = true
        self:SetStatus("Scan queued until combat ends")
        return nil
    end

    if not self.db then
        self:InitializeDatabase()
    end

    local identity = self:GetCharacterIdentity()
    local snapshot = {
        addonVersion = self.version,
        capturedAt = time(),
        capturedAtText = date("%Y-%m-%d %H:%M:%S"),
        reason = reason or "manual",
        character = identity.character,
        realm = identity.realm,
        class = identity.class,
        specializationID = identity.specializationID,
        specialization = identity.specialization,
        profile = self.destinationProfile and self.destinationProfile.id or self.profile.id,
        gameBuild = select(4, GetBuildInfo()),
        destinationProfile = self.destinationProfile and self.destinationProfile.id,
        destinationProfileName = self.destinationProfile and self.destinationProfile.name,
        destinations = self.GetDestinationSnapshot and self:GetDestinationSnapshot() or nil,
        actionBarState = self:GetActionBarState(),
        actionSlots = self:GetAllActionSlots(),
        spellbook = self.GetSpellbookSnapshot and self:GetSpellbookSnapshot() or nil,
    }

    if self.trainingCoverage and self.GetTrainingContext then
        local trainingContext = self:GetTrainingContext(snapshot)
        if trainingContext and self.trainingCoverage.contextKey
            and trainingContext.contextKey ~= self.trainingCoverage.contextKey
        then
            self.trainingCoverage = nil
            self.trainingRecommendations = nil
        end
    end

    snapshot.audit = self:BuildAudit(snapshot)
    local storedCharacter = self.db.characters[identity.key]
    local characterData = type(storedCharacter) == "table" and storedCharacter or {
        character = identity.character,
        realm = identity.realm,
        specs = {},
    }
    characterData.character = identity.character
    characterData.realm = identity.realm
    characterData.class = identity.class
    characterData.latestSpecID = identity.specializationID
    characterData.latestSnapshot = compactStoredSnapshot(snapshot)
    local formKey = snapshot.actionBarState
        and (snapshot.actionBarState.stateID or snapshot.actionBarState.stateKey) or "Unknown"
    local profileID = snapshot.destinationProfile or snapshot.profile or "default"
    local loadoutKey = tostring(snapshot.spellbook and snapshot.spellbook.activeConfigID or 0)
        .. ":" .. tostring(snapshot.spellbook and snapshot.spellbook.activeHeroSpecID or 0)
    local contextKey = table.concat({
        tostring(profileID),
        tostring(identity.specializationID),
        loadoutKey,
        formKey,
    }, "|")
    characterData.contextSnapshots = pruneContextSnapshots(characterData.contextSnapshots)
    characterData.contextSnapshots[contextKey] = {
        capturedAt = snapshot.capturedAt,
        capturedAtText = snapshot.capturedAtText,
        specializationID = snapshot.specializationID,
        specialization = snapshot.specialization,
        stateKey = formKey,
        stateLabel = snapshot.actionBarState and snapshot.actionBarState.stateKey,
        profileID = profileID,
        activeConfigID = snapshot.spellbook and snapshot.spellbook.activeConfigID or 0,
        activeHeroSpecID = snapshot.spellbook and snapshot.spellbook.activeHeroSpecID or 0,
    }
    characterData.contextSnapshots = pruneContextSnapshots(characterData.contextSnapshots)
    -- Versions before schema 5 stored the same full scan in several indexes.
    -- Learning lives in trainingModel, so these redundant copies can be dropped.
    characterData.specs = nil
    characterData.formSnapshots = nil
    characterData.profileSnapshots = nil
    self.db.characters[identity.key] = characterData

    self.currentSnapshot = snapshot
    if self.layoutPlan and self.IsLayoutPlanCurrent and not self:IsLayoutPlanCurrent(snapshot) then
        self.layoutPlan = nil
    end
    self:SetStatus("Scan complete • Saved on reload or logout")

    if self.RefreshUI then
        self:RefreshUI()
    end

    return snapshot
end

function Addon:ScheduleScan(reason, delay)
    self.scheduledReason = reason or self.scheduledReason or "event"
    self.scanScheduleToken = (self.scanScheduleToken or 0) + 1
    local token = self.scanScheduleToken
    if self.scanTimer and self.scanTimer.Cancel then
        pcall(self.scanTimer.Cancel, self.scanTimer)
    end
    self.scanScheduled = true
    local function runScan()
        if token ~= self.scanScheduleToken then return end
        self.scanTimer = nil
        self.scanScheduled = false
        local scheduledReason = self.scheduledReason
        self.scheduledReason = nil
        self:CaptureSnapshot(scheduledReason)
    end
    if C_Timer and C_Timer.NewTimer then
        self.scanTimer = C_Timer.NewTimer(delay or 0.5, runScan)
    elseif C_Timer and C_Timer.After then
        C_Timer.After(delay or 0.5, runScan)
    else
        runScan()
    end
end

function Addon:PrintHelp()
    print("|cff65d9fflamdaUI|r commands:")
    print("  |cffffffff/lamdaui|r or |cffffffff/lui|r — open or close the dashboard")
    print("  |cffffffff/lui hub|r — open Custom Addons; /aughelper — Augmentation settings")
    print("  |cffffffff/lamdaui scan|r — refresh the current setup")
    print("  |cffffffff/lamdaui keys|r — open Profiles")
    print("  |cffffffff/lamdaui layout|r — open Preview")
    print("  |cffffffff/lamdaui learn|r — remember the current setup as a preference")
    print("  |cffffffff/lamdaui preview|r — build a layout preview")
    print("  |cffffffff/lamdaui apply|r — apply the reviewed layout")
    print("  |cffffffff/lamdaui undo|r — restore the layout saved before the latest Apply")
    print("  |cffffffff/lamdaui history|r — show recent Apply, Undo, and recovery activity")
    print("  |cffffffff/lamdaui export|r — copy a portable profile")
    print("  |cffffffff/lamdaui import|r — import a portable profile")
    print("  |cffffffff/lamdaui profileundo|r — undo the latest profile change")
    print("  |cffffffff/lamdaui doctor|r — print a privacy-safe readiness report")
end

local function availability(value)
    return value and "ready" or "unavailable"
end

local function apiFunction(owner, name)
    return type(owner and owner[name]) == "function"
end

function Addon:GetDiagnosticsReport()
    local inCombat = InCombatLockdown and InCombatLockdown() or false
    local snapshot = self.currentSnapshot
    if not snapshot and not inCombat and self.CaptureSnapshot then
        snapshot = self:CaptureSnapshot("diagnostics")
    end
    local identity = self:GetCharacterIdentity()
    local gameVersion, gameBuild, _, interfaceVersion = GetBuildInfo()
    local barState = snapshot and snapshot.actionBarState or {}
    local spellbook = snapshot and snapshot.spellbook or {}
    local profileSummary = self.GetDestinationProfileSummary
        and self:GetDestinationProfileSummary() or {}
    local trainingSummary = self.GetTrainingSummary and self:GetTrainingSummary() or {}
    local transaction
    if self.GetLayoutTransactionState then
        local _, activeTransaction = self:GetLayoutTransactionState()
        transaction = activeTransaction
    end
    local coverage = self.trainingCoverage
    local plan = self.layoutPlan
    local actionMutationReady = apiFunction(C_Spell, "PickupSpell")
        and (apiFunction(C_ActionBar, "PutActionInSlot") or type(PlaceAction) == "function")
        and type(PickupAction) == "function"
        and type(GetActionInfo) == "function"
        and type(GetCursorInfo) == "function"
        and type(ClearCursor) == "function"
    local bindingMutationReady = type(GetCurrentBindingSet) == "function"
        and type(GetBindingAction) == "function"
        and type(SetBinding) == "function"
        and type(SetBindingSpell) == "function"
        and type(SaveBindings) == "function"
    local clickCastingReady = apiFunction(C_ClickBindings, "GetProfileInfo")
        and apiFunction(C_ClickBindings, "SetProfileByInfo")
        and apiFunction(C_ClickBindings, "CanSpellBeClickBound")
    local editModeReady = apiFunction(C_EditMode, "GetLayouts")
        and apiFunction(C_EditMode, "SetActiveLayout")
    local portableEditModeReady = editModeReady
        and apiFunction(C_EditMode, "ConvertLayoutInfoToString")
        and apiFunction(C_EditMode, "ConvertStringToLayoutInfo")
        and apiFunction(C_EditMode, "IsValidLayoutName")
        and apiFunction(C_EditMode, "SaveLayouts")
        and apiFunction(C_EditMode, "OnLayoutAdded")
        and apiFunction(C_EditMode, "OnLayoutDeleted")
    local cooldownManagerReady = apiFunction(C_CooldownViewer, "GetLayoutData")
        and apiFunction(C_CooldownViewer, "SetLayoutData")
    local castSettingsReady = type(GetCVar) == "function"
        and (apiFunction(C_CVar, "SetCVar") or type(SetCVar) == "function")
        and type(GetModifiedClick) == "function"
        and type(SetModifiedClick) == "function"
        and type(SaveBindings) == "function"
    local bindingAudit = snapshot and snapshot.audit and snapshot.audit.bindings or {}
    local recentLayout
    if self.GetLayoutTransactionHistory then
        local recent = self:GetLayoutTransactionHistory(1)
        recentLayout = recent[1] and recent[1].transaction
    end
    local blockers = {}
    if inCombat then table.insert(blockers, "combat") end
    if barState.overrideActive then table.insert(blockers, "temporary action bar") end
    if (profileSummary.destinations or 0) == 0 then table.insert(blockers, "no captured inputs") end
    if not spellbook.available then table.insert(blockers, "spellbook unavailable") end
    if (trainingSummary.profileConfirmedSetups or 0) == 0 then
        table.insert(blockers, "active profile has no confirmed setup")
    end

    local lines = {
        "lamdaUI diagnostics",
        "Addon: " .. tostring(self.version) .. " • database schema "
            .. tostring(self.db and self.db.schemaVersion or "not loaded"),
        "Client: " .. tostring(gameVersion or "unknown") .. " • build "
            .. tostring(gameBuild or "unknown") .. " • interface "
            .. tostring(interfaceVersion or "unknown") .. " • locale "
            .. tostring(GetLocale and GetLocale() or "unknown"),
        "Context: " .. tostring(identity.class or "unknown") .. " • "
            .. tostring(identity.specialization or "No specialization") .. " ("
            .. tostring(identity.specializationID or 0) .. ") • "
            .. tostring(barState.stateID or barState.stateKey or "not scanned")
            .. " • loadout " .. tostring(spellbook.activeConfigID or 0) .. ":"
            .. tostring(spellbook.activeHeroSpecID or 0),
        "Profile: " .. (profileSummary.name and "active" or "not loaded") .. " • "
            .. tostring(profileSummary.readable or 0) .. "/"
            .. tostring(profileSummary.destinations or 0) .. " readable inputs • "
            .. tostring(trainingSummary.profileConfirmedSetups or 0) .. " confirmed setups",
        "Bindings: " .. tostring(bindingAudit.scope or "unknown scope") .. " ("
            .. tostring(bindingAudit.scopeID or "?") .. ") • "
            .. tostring(bindingAudit.matches or 0) .. "/"
            .. tostring(bindingAudit.total or 0) .. " profile commands active",
        "Spellbook: " .. availability(spellbook.available) .. " • "
            .. tostring(spellbook.numItems or 0) .. " items • Cooldown Manager categories "
            .. availability(spellbook.cooldownViewerAvailable),
        "Core mutation APIs: action bars " .. availability(actionMutationReady)
            .. " • bindings " .. availability(bindingMutationReady)
            .. " • Click Casting " .. availability(clickCastingReady),
        "UI mutation APIs: Edit Mode " .. availability(editModeReady)
            .. " • portable Edit Mode " .. availability(portableEditModeReady)
            .. " • Cooldown Manager " .. availability(cooldownManagerReady)
            .. " • cast settings " .. availability(castSettingsReady),
        "Recovery: " .. tostring(transaction and transaction.status or "none"),
        "Latest layout result: " .. tostring(recentLayout and recentLayout.status or "none"),
        "Preview: " .. tostring(plan and (plan.ready and "ready" or "needs attention") or "not built")
            .. (coverage and (" • " .. tostring(coverage.recommendedPlacements or 0) .. "/"
                .. tostring(coverage.targetPlacements or 0) .. " placements") or ""),
        "Blockers: " .. (#blockers > 0 and table.concat(blockers, ", ") or "none"),
        "Character and realm names are intentionally omitted.",
    }
    return table.concat(lines, "\n")
end

function Addon:PrintDiagnostics()
    local report = self:GetDiagnosticsReport()
    for line in report:gmatch("[^\n]+") do
        print("|cff65d9fflamdaUI|r " .. line:gsub("|", "||"))
    end
    return report
end

local function handleSlash(message)
    if Addon.legacyConflict then
        reportLegacyConflict()
        return
    end
    message = strtrim((message or ""):lower())
    if message == "" or message == "hub" or message == "addons" or message == "custom" then
        Addon:Show("custom")
    elseif message == "aug" then
        Addon:AugmentationCommand("")
    elseif message == "scan" then
        Addon:CaptureSnapshot("slash command")
        Addon:Show("setup")
    elseif message == "apply" then
        Addon:Show("layout")
        Addon:ConfirmApplyLayout()
    elseif message == "undo" then
        Addon:UndoLayoutTransaction()
        Addon:Show("layout")
    elseif message == "history" or message == "recovery" then
        Addon:Show("layout")
        Addon:ShowLayoutHistory()
    elseif message == "keys" or message == "keybinds" then
        Addon:Show("map")
    elseif message == "layout" or message == "train" or message == "training" then
        Addon:Show("layout")
    elseif message == "learn" or message == "learn setup" then
        Addon:ConfirmLearnCurrentSetup()
    elseif message == "suggest" or message == "recommend" or message == "preview" then
        Addon:Show("layout")
        Addon:PreviewLayoutPlan()
    elseif message == "export" then
        Addon:Show("map")
        Addon:ShowProfileExport()
    elseif message == "import" then
        Addon:Show("map")
        Addon:ShowProfileImport()
    elseif message == "profileundo" or message == "profile undo"
        or message == "importundo" or message == "import undo"
    then
        Addon:UndoLastProfileChange()
        Addon:Show("map")
    elseif message == "doctor" or message == "diagnostics" then
        Addon:PrintDiagnostics()
    elseif message == "show" then
        Addon:Show("setup")
    elseif message == "hide" then
        Addon:Hide()
    elseif message == "help" then
        Addon:PrintHelp()
    elseif message == "" then
        Addon:Toggle()
    else
        Addon:PrintHelp()
    end
end

local eventFrame = CreateFrame("Frame")
eventFrame:RegisterEvent("ADDON_LOADED")
eventFrame:SetScript("OnEvent", function(_, event, ...)
    if event == "ADDON_LOADED" then
        local loadedAddon = ...
        if loadedAddon == addonName then
            if addonName ~= legacyAddonName and isAddonLoaded(legacyAddonName) then
                reportLegacyConflict()
                Addon:ShowLegacyMigrationPrompt()
                return
            end
            Addon:InitializeDatabase()
            Addon:CreateUI()

            SLASH_LAMDAUI1 = "/lamdaui"
            SLASH_LAMDAUI2 = "/lui"
            SlashCmdList.LAMDAUI = handleSlash

            eventFrame:RegisterEvent("PLAYER_LOGIN")
            eventFrame:RegisterEvent("PLAYER_ENTERING_WORLD")
            eventFrame:RegisterEvent("UPDATE_BINDINGS")
            eventFrame:RegisterEvent("ACTIONBAR_SLOT_CHANGED")
            eventFrame:RegisterEvent("PLAYER_SPECIALIZATION_CHANGED")
            eventFrame:RegisterEvent("PLAYER_TALENT_UPDATE")
            eventFrame:RegisterEvent("ACTIVE_COMBAT_CONFIG_CHANGED")
            eventFrame:RegisterEvent("SELECTED_LOADOUT_CHANGED")
            eventFrame:RegisterEvent("SPELLS_CHANGED")
            eventFrame:RegisterEvent("SPELL_TEXT_UPDATE")
            eventFrame:RegisterEvent("COOLDOWN_VIEWER_DATA_LOADED")
            eventFrame:RegisterEvent("COOLDOWN_VIEWER_TABLE_HOTFIXED")
            eventFrame:RegisterEvent("COOLDOWN_VIEWER_SPELL_OVERRIDE_UPDATED")
            eventFrame:RegisterEvent("PLAYER_REGEN_ENABLED")
            eventFrame:RegisterEvent("EDIT_MODE_LAYOUTS_UPDATED")
            eventFrame:RegisterEvent("CVAR_UPDATE")
            eventFrame:RegisterEvent("UPDATE_SHAPESHIFT_FORM")
            eventFrame:RegisterEvent("UPDATE_BONUS_ACTIONBAR")
            eventFrame:RegisterEvent("ACTIONBAR_PAGE_CHANGED")
            eventFrame:RegisterEvent("UPDATE_STEALTH")
        elseif loadedAddon == legacyAddonName and addonName ~= legacyAddonName then
            reportLegacyConflict()
            if Addon.SetStatus then Addon:SetStatus(legacyConflictMessage) end
            Addon:ShowLegacyMigrationPrompt()
        elseif Addon.db and Addon.trackedAddonNames and Addon.trackedAddonNames[loadedAddon] then
            Addon:ScheduleScan("addon loaded")
        end
        return
    end

    if event == "PLAYER_LOGIN" then
        Addon:ScheduleScan("login", 2)
        C_Timer.After(2.5, function()
            Addon:ResumeLayoutTransaction()
        end)
        if not Addon.db.options.seenWelcome then
            Addon.db.options.seenWelcome = true
            C_Timer.After(2.2, function()
                Addon:Show("custom")
            end)
        end
    elseif event == "PLAYER_ENTERING_WORLD" then
        Addon:ScheduleScan("entering world", 1)
    elseif event == "UPDATE_BINDINGS" then
        Addon:ScheduleScan("bindings changed")
    elseif event == "ACTIONBAR_SLOT_CHANGED" then
        Addon:ScheduleScan("action bar changed")
    elseif event == "PLAYER_SPECIALIZATION_CHANGED" then
        local unit = ...
        if unit == "player" then
            Addon:ScheduleScan("specialization changed", 1)
        end
    elseif event == "PLAYER_TALENT_UPDATE"
        or event == "ACTIVE_COMBAT_CONFIG_CHANGED"
        or event == "SELECTED_LOADOUT_CHANGED"
    then
        Addon:ScheduleScan("talents changed", 1)
    elseif event == "SPELLS_CHANGED" then
        Addon:ScheduleScan("spellbook changed", 1)
    elseif event == "SPELL_TEXT_UPDATE" then
        Addon:ScheduleScan("spell details loaded", 0.8)
    elseif event == "COOLDOWN_VIEWER_DATA_LOADED"
        or event == "COOLDOWN_VIEWER_TABLE_HOTFIXED"
        or event == "COOLDOWN_VIEWER_SPELL_OVERRIDE_UPDATED"
    then
        Addon:ScheduleScan("specialization spell categories updated", 0.8)
    elseif event == "EDIT_MODE_LAYOUTS_UPDATED" then
        Addon:ScheduleScan("Edit Mode layout changed", 1)
    elseif event == "UPDATE_SHAPESHIFT_FORM" or event == "UPDATE_BONUS_ACTIONBAR" or event == "ACTIONBAR_PAGE_CHANGED" or event == "UPDATE_STEALTH" then
        Addon:ScheduleScan("form or action page changed", 0.8)
        C_Timer.After(1, function()
            Addon:ResumeLayoutTransaction()
        end)
    elseif event == "CVAR_UPDATE" then
        local cvarName = ...
        if cvarName == "enableMultiActionBars" then
            Addon:ScheduleScan("action bars changed")
        end
    elseif event == "PLAYER_REGEN_ENABLED" then
        if Addon.scanAfterCombat then
            Addon.scanAfterCombat = nil
            Addon:ScheduleScan("after combat")
        end
        C_Timer.After(0.8, function()
            Addon:ResumeLayoutTransaction()
        end)
    end
end)

function LamdaUI_OnAddonCompartmentClick()
    if Addon.legacyConflict then
        reportLegacyConflict()
        Addon:ShowLegacyMigrationPrompt()
        return
    end
    Addon:Toggle()
end
