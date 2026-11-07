local addonName,E=...
local call,array=E.call,E.array

function E.Initialize()
    if type(LamdaExportDB)~="table" then LamdaExportDB={} end
    E.db=LamdaExportDB
    E.db.schema=E.db.schema or 1
end
function E.CancelCapture()
    E.capture=nil
end
local function safeSection(collector,snapshot)
    local issues=E.readIssues or 0
    local ok,value=pcall(collector.collect,snapshot)
    if not ok or type(value)~="table" then
        E.note("The "..collector.name.." section could not be read on this client.")
        return E.unavailable("A game API read failed for this section.")
    end
    if (E.readIssues or 0)>issues and value.status=="complete" then value.status="partial" end
    value.observedAt=E.now()
    return value
end
local function finish(capture)
    if E.capture~=capture then return end
    local missing=0
    for _,item in ipairs(capture.itemRecords) do
        if item.metadataStatus=="pending" then item.metadataStatus="unavailable";missing=missing+1 end
    end
    if missing>0 then E.note("Item IDs and full item strings are preserved where item-name/stat metadata was unavailable.") end
    local snapshot=capture.snapshot
    snapshot.finishedAt=E.now()
    snapshot.durationMilliseconds=math.floor(((call("GetTime") or capture.started)-capture.started)*1000+.5)
    snapshot.availability={notes=array(E.notes),itemMetadataUnavailable=missing,sections={}}
    for _,collector in ipairs(E.collectors) do
        snapshot.availability.sections[collector.name]=snapshot[collector.name] and snapshot[collector.name].status or "unavailable"
    end
    local ok,text=pcall(E.json,snapshot)
    if not ok then
        E.capture=nil
        E.window.status:SetText("The snapshot could not be formatted. Try /export again.")
        E.print("The snapshot could not be formatted; no partial text was offered as a full export.")
        return
    end
    E.db.lastSnapshot=E.clean(snapshot)
    E.db.lastExportAt=snapshot.capturedAt
    E.capture=nil
    E.ShowExport(text,snapshot,capture.generation)
end
local function refreshPending(capture,final)
    local pending=0
    for id in pairs(capture.pendingItems) do
        if final or capture.receivedItems[id] then
            local stillMissing=false
            for _,row in ipairs(capture.itemsByID[id] or {}) do E.FillItem(row);if row.metadataStatus=="pending" then stillMissing=true end end
            if not stillMissing then capture.pendingItems[id]=nil else pending=pending+1 end
            capture.receivedItems[id]=nil
        else pending=pending+1 end
    end
    return pending
end
local function enrich(capture)
    if E.capture~=capture then return end
    local now=call("GetTime") or 0
    local final=now>=capture.deadline
    local pending=refreshPending(capture,final)
    if final or (pending==0 and not capture.raidPending and not capture.completedPending) then
        -- Server replies may have arrived after these sections were first read.
        for _,collector in ipairs(E.collectors) do
            if collector.name=="lockouts" or collector.name=="quests" then capture.snapshot[collector.name]=safeSection(collector,capture.snapshot) end
        end
        finish(capture)
    else C_Timer.After(.1,function()enrich(capture)end) end
end
local function requestMetadata(capture)
    capture.itemsByID={}
    for _,item in ipairs(capture.itemRecords) do
        if item.itemID then
            capture.itemsByID[item.itemID]=capture.itemsByID[item.itemID] or {}
            table.insert(capture.itemsByID[item.itemID],item)
        end
    end
    for id in pairs(capture.pendingItems) do
        capture.receivedItems[id]=true
    end
    refreshPending(capture,false)
    local ids={};for id in pairs(capture.pendingItems) do ids[#ids+1]=id end
    local index=1
    local function batch()
        if E.capture~=capture then return end
        for _=1,20 do
            local id=ids[index];if not id then enrich(capture);return end
            index=index+1
            if E.has("C_Item.RequestLoadItemDataByID") then call("C_Item.RequestLoadItemDataByID",id) end
        end
        C_Timer.After(0,batch)
    end
    batch()
end
function E.StartCapture()
    E.Initialize()
    E.CancelCapture()
    if BankFrame and E.invoke(BankFrame.IsShown,BankFrame)==true then E.bankOpen=true end
    for _,name in ipairs({"ProfessionsFrame","TradeSkillFrame","CraftFrame"}) do
        local frame=_G[name]
        if frame and E.invoke(frame.IsShown,frame)==true then E.tradeOpen=true end
    end
    E.notes,E.noteSet,E.readIssues={},{},0
    local generation=E.ShowProgress("Collecting character data...")
    local version,build,buildDate,interface=call("GetBuildInfo")
    local snapshot={format="LamdaExport",schema=1,addonVersion=E.version,
        client=interface==16001 and "forever" or (type(interface)=="number" and interface>=100000 and "retail" or "unknown"),
        game={version=version,build=build,buildDate=buildDate,interface=interface,locale=call("GetLocale")},
        capturedAt=E.now(),capturedAtUTC=call("date","!%Y-%m-%dT%H:%M:%SZ",E.now()),
    }
    local capture={snapshot=snapshot,generation=generation,index=1,started=call("GetTime") or 0,
        pendingItems={},receivedItems={},itemRecords={}}
    capture.deadline=capture.started+1.5
    E.capture=capture
    if E.has("RequestRaidInfo") and E.has("GetNumSavedInstances") then
        E.raidInfoReady=false;capture.raidPending=true;call("RequestRaidInfo")
    end
    if not E.has("C_QuestLog.GetAllCompletedQuestIDs") and E.has("QueryQuestsCompleted") then
        E.completedReady=false;capture.completedPending=true;call("QueryQuestsCompleted")
    end
    local function step()
        if E.capture~=capture then return end
        local collector=E.collectors[capture.index]
        if not collector then requestMetadata(capture);return end
        E.window.status:SetText("Collecting "..collector.name.."...")
        snapshot[collector.name]=safeSection(collector,snapshot)
        capture.index=capture.index+1
        C_Timer.After(0,step)
    end
    C_Timer.After(0,step)
end

local frame=CreateFrame("Frame")
E.eventFrame=frame
local scheduled={}
local function later(key,fn)
    if scheduled[key] then return end
    scheduled[key]=true
    C_Timer.After(.2,function()scheduled[key]=nil;fn()end)
end
frame:SetScript("OnEvent",function(_,event,...)
    local value=...
    if E.secret(value) then return end
    if event=="ADDON_LOADED" then
        if value==addonName then E.Initialize() end
    elseif event=="BANKFRAME_OPENED" then
        E.bankOpen=true
        later("bank",E.CaptureBank)
    elseif event=="BANKFRAME_CLOSED" then E.bankOpen=false
    elseif event=="BAG_UPDATE_DELAYED" or event=="PLAYERBANKSLOTS_CHANGED" then
        if E.bankOpen then later("bank",E.CaptureBank) end
    elseif event=="TRADE_SKILL_SHOW" or event=="CRAFT_SHOW" then
        E.tradeOpen=true;later("recipes",E.CaptureRecipes)
    elseif event=="TRADE_SKILL_CLOSE" or event=="CRAFT_CLOSE" then E.tradeOpen=false
    elseif event=="TRADE_SKILL_LIST_UPDATE" then
        if E.tradeOpen then later("recipes",E.CaptureRecipes) end
    elseif event=="UPDATE_INSTANCE_INFO" then
        E.raidInfoReady=true;if E.capture then E.capture.raidPending=false end
    elseif event=="QUEST_QUERY_COMPLETE" then
        E.completedReady=true;if E.capture then E.capture.completedPending=false end
    elseif event=="GET_ITEM_INFO_RECEIVED" or event=="ITEM_DATA_LOAD_RESULT" then
        if E.capture and type(value)=="number" and E.capture.pendingItems[value] then E.capture.receivedItems[value]=true end
    end
end)
for _,event in ipairs({"ADDON_LOADED","BANKFRAME_OPENED","BANKFRAME_CLOSED","BAG_UPDATE_DELAYED","PLAYERBANKSLOTS_CHANGED","TRADE_SKILL_SHOW","TRADE_SKILL_CLOSE","TRADE_SKILL_LIST_UPDATE","CRAFT_SHOW","CRAFT_CLOSE","UPDATE_INSTANCE_INFO","QUEST_QUERY_COMPLETE","GET_ITEM_INFO_RECEIVED","ITEM_DATA_LOAD_RESULT"}) do
    -- Some classic/retail event names differ. One missing optional event must
    -- not prevent the addon or its slash command from loading.
    pcall(frame.RegisterEvent,frame,event)
end
SLASH_LAMDAEXPORT1="/export"
SLASH_LAMDAEXPORT2="/lexport"
SlashCmdList.LAMDAEXPORT=function(message)
    if message and message:lower():match("^%s*status%s*$") then
        E.Initialize()
        E.print("Version "..E.version.."; last completed snapshot: "..tostring(E.db.lastExportAt or "none")..". Use /export to copy a fresh one.")
    else E.StartCapture() end
end
