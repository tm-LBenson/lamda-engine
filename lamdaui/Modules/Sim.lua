local _, LUI = ...
local S = {sequence=0,status="Ready. Requires Lamda Engine."}
LUI.Sim = S
function S:RequireEnabled()
    if LUI:DB().simEnabled then return true end
    self:SetStatus("Enable LamdaSim in /lamda > Modules first.")
    if LUI.frame then LUI:OpenUI();LUI:SelectPage("modules");LUI:SelectModule("sim")end
    return false
end
local SEND_INTERVAL,SEND_PASSES,RELOAD_BUFFER,RELOAD_MINIMUM=0.4,3,2,30
local POLL_INTERVAL,POLL_LIMIT,MISSING_LIMIT=30,3600,3
local SIM_CHUNK,SIM_WIDTH,SIM_HEIGHT,SIM_CELLS=5600,256,32,2048
local ENGINE_RELOAD_BINDING="CTRL-SHIFT-F12"
local array = {array=true}
local previewLevel
if hooksecurefunc and C_EncounterJournal and C_EncounterJournal.SetPreviewMythicPlusLevel then
    hooksecurefunc(C_EncounterJournal,"SetPreviewMythicPlusLevel",function(level)previewLevel=level end)
end
local function plain(value)
    if issecretvalue and issecretvalue(value) then error("Some character data is restricted. Try again outside combat.") end
    return value
end
local function quote(value)
    return '"'..value:gsub('[%z\1-\31\\"]',function(c)
        if c=='"' or c=='\\' then return '\\'..c end
        return string.format('\\u%04x',c:byte())
    end)..'"'
end
local function json(value,depth)
    plain(value);depth=(depth or 0)+1;assert(depth<12,"Snapshot nesting is invalid")
    if type(value)=="string" then return quote(value)
    elseif type(value)=="number" then assert(value==value and value~=math.huge and value~=-math.huge);return tostring(value)
    elseif type(value)=="boolean" then return value and "true" or "false"
    elseif type(value)=="table" then
        local out={}
        if getmetatable(value)==array then
            for i,v in ipairs(value) do out[i]=json(v,depth) end
            return "["..table.concat(out,",").."]"
        end
        local keys={};for k in pairs(value) do assert(type(k)=="string");keys[#keys+1]=k end;table.sort(keys)
        for _,k in ipairs(keys) do out[#out+1]=quote(k)..":"..json(value[k],depth) end
        return "{"..table.concat(out,",").."}"
    end
    error("Unreadable character data")
end
function S:SetStatus(message)
    self.status=message
    if self.label then self.label:SetText(message) end
    if self.GameUI then
        if self.GameUI.SetStatus then self.GameUI:SetStatus(message) end
        if self.GameUI.Refresh then self.GameUI:Refresh() end
    end
    if self.gameStatusCallback then pcall(self.gameStatusCallback,message,self.scan~=nil or self.packets~=nil) end
end
-- Read only item contributions reported by this client. An unloaded item is
-- distinct from an item with no stats; neither is a zero-throughput result.
local function readGearItem(api,key,id,link,slot)
    local item={key=key,id=id,link=link,slot=slot,stats={},statsReady=false}
    local ok=pcall(function()
        local name,_,_,level,_,_,_,_,equipLoc=api.GetItemInfo(link)
        if not equipLoc and api.GetItemInfoInstant then local _,_,_,instantLoc=api.GetItemInfoInstant(link);equipLoc=instantLoc end
        item.label=plain(name) or tostring(id);item.level=plain(level) or 0;item.equipLoc=plain(equipLoc) or ""
        local stats=plain(api.GetItemStats(link))
        if type(stats)=="table" and name and equipLoc then
            for stat,value in pairs(stats) do item.stats[plain(stat)]=plain(value) end
            item.statsReady=true
        end
    end)
    if not ok then item.stats={};item.statsReady=false end
    return item
end
function S:GearSnapshot()
    local gear={equipped=setmetatable({},array),bags=setmetatable({},array)}
    local api=C_Item
    if not api or not api.GetItemStats or not api.GetItemInfo then return nil end
    for slot=1,19 do
        local id=plain(GetInventoryItemID("player",slot))
        if id then
            local link=plain(GetInventoryItemLink("player",slot))
            if link then gear.equipped[#gear.equipped+1]=readGearItem(api,"equipped_"..slot,id,link,slot) end
        end
    end
    if C_Container and C_Container.GetContainerNumSlots and C_Container.GetContainerItemInfo then
        for bag=0,(NUM_BAG_SLOTS or 4) do
            for slot=1,plain(C_Container.GetContainerNumSlots(bag)) do
                local info=plain(C_Container.GetContainerItemInfo(bag,slot))
                if info and plain(info.itemID) and plain(info.hyperlink) then
                    local item=readGearItem(api,"bag_"..bag.."_"..slot,info.itemID,info.hyperlink,0)
                    if item.equipLoc and item.equipLoc~="" then gear.bags[#gear.bags+1]=item end
                end
            end
        end
    end
    return gear
end
local lootSlots={INVTYPE_HEAD="head",INVTYPE_NECK="neck",INVTYPE_SHOULDER="shoulder",INVTYPE_CLOAK="back",
    INVTYPE_CHEST="chest",INVTYPE_ROBE="chest",INVTYPE_WRIST="wrist",INVTYPE_HAND="hands",INVTYPE_WAIST="waist",
    INVTYPE_LEGS="legs",INVTYPE_FEET="feet",INVTYPE_FINGER="finger1",INVTYPE_TRINKET="trinket1",
    INVTYPE_WEAPON="main_hand",INVTYPE_WEAPONMAINHAND="main_hand",INVTYPE_2HWEAPON="main_hand",
    INVTYPE_RANGED="main_hand",INVTYPE_RANGEDRIGHT="main_hand",INVTYPE_WEAPONOFFHAND="off_hand",
    INVTYPE_SHIELD="off_hand",INVTYPE_HOLDABLE="off_hand"}
-- Item constraints are instance data: bonus IDs can add crafted limits. Read
-- both public APIs after the item cache resolves; absence is not an empty rule.
local function itemRule(link)
    local api=C_Item
    if not api or not api.GetItemInfo or not api.GetItemInfoInstant or not api.GetItemUniqueness or not api.GetItemUniquenessByID then return nil end
    local ok,result=pcall(function()
        link=plain(link);assert(type(link)=="string")
        local body=link:match("item:([%-?%d:]+)");assert(body)
        local fields={};for value in (body..":"):gmatch("(.-):") do fields[#fields+1]=value end
        assert(#fields>=13)
        local id=tonumber(fields[1]);local count=fields[13]=="" and 0 or tonumber(fields[13])
        assert(id and id>0 and id<=10000000 and count and count>=0 and count<=64 and count%1==0 and #fields>=13+count)
        local bonuses=setmetatable({},array);local seen={}
        for i=14,13+count do
            local value=tonumber(fields[i]);assert(value and value>0 and value<=10000000 and value%1==0)
            if not seen[value] then seen[value]=true;bonuses[#bonuses+1]=value end
        end
        table.sort(bonuses)
        local name,_,_,_,_,_,_,_,cachedLoc=api.GetItemInfo(link)
        local itemID,_,_,equipLoc=api.GetItemInfoInstant(link)
        assert(type(plain(name))=="string" and name~="" and plain(itemID)==id and plain(cachedLoc)==plain(equipLoc) and lootSlots[equipLoc])
        local category,maximum=api.GetItemUniqueness(link)
        local unique,_,staticMax,staticCategory=api.GetItemUniquenessByID(link)
        category=plain(category);maximum=plain(maximum);unique=plain(unique);staticMax=plain(staticMax);staticCategory=plain(staticCategory)
        assert(type(unique)=="boolean")
        category=category or 0;staticCategory=staticCategory or 0
        assert(type(category)=="number" and category>=0 and category<=10000000 and category%1==0 and type(staticCategory)=="number" and staticCategory>=0 and staticCategory<=10000000 and staticCategory%1==0)
        assert(category==0 or staticCategory==0 or category==staticCategory)
        if category==0 and staticCategory>0 then category=staticCategory;maximum=staticMax end
        if category>0 then assert(type(maximum)=="number" and maximum>=1 and maximum<=100 and maximum%1==0) else maximum=0 end
        return {id=id,bonusIDs=bonuses,inventoryType=equipLoc,unique=unique,limitCategory=category,limitMax=maximum}
    end)
    if ok then return result end
end
function S:RuleSnapshot()
    local api=C_Item
    if not api or not api.GetItemInfo or not api.GetItemInfoInstant or not api.GetItemUniqueness or not api.GetItemUniquenessByID then return nil end
    local byKey={};local unknown=false;local count=0
    local function add(link)
        if not link then return end
        local _,_,_,equipLoc=api.GetItemInfoInstant(link);equipLoc=plain(equipLoc)
        if type(equipLoc)=="string" and not lootSlots[equipLoc] then return end
        local rule=itemRule(link)
        if not rule then unknown=true;return end
        local key=rule.id..":"..table.concat(rule.bonusIDs,"/")
        local prior=byKey[key]
        if prior==false then return end
        if prior and json(prior)~=json(rule) then byKey[key]=false;unknown=true;return end
        if not prior then count=count+1;assert(count<=768,"Too many item rules.");byKey[key]=rule end
    end
    local ok=pcall(function()
        for slot=1,19 do
            if plain(GetInventoryItemID("player",slot)) then add(plain(GetInventoryItemLink("player",slot))) end
        end
        if C_Container and C_Container.GetContainerNumSlots and C_Container.GetContainerItemInfo then
            for bag=0,(NUM_BAG_SLOTS or 4) do
                local slots=plain(C_Container.GetContainerNumSlots(bag));assert(type(slots)=="number" and slots>=0 and slots<=200)
                for slot=1,slots do
                    local item=plain(C_Container.GetContainerItemInfo(bag,slot))
                    if item and plain(item.hyperlink) then
                        local _,_,_,equipLoc=api.GetItemInfoInstant(item.hyperlink)
                        if lootSlots[plain(equipLoc)] then add(item.hyperlink) end
                    end
                end
            end
        end
        local weekly=C_WeeklyRewards
        if weekly and weekly.GetActivities and weekly.GetItemHyperlink then
            for _,activity in ipairs(plain(weekly.GetActivities()) or {}) do
                for _,reward in ipairs(plain(activity.rewards) or {}) do
                    local id=plain(reward.itemDBID)
                    if id then
                        local link=plain(weekly.GetItemHyperlink(id))
                        if link then add(link) else unknown=true end
                    end
                end
            end
        end
    end)
    if not ok then return nil,"Item equip rules were unavailable. Gear combinations may be limited." end
    local keys={};for key in pairs(byKey) do keys[#keys+1]=key end;table.sort(keys)
    local rules=setmetatable({},array)
    for _,key in ipairs(keys) do if byKey[key] then rules[#rules+1]=byKey[key] end end
    return rules,unknown and "Some item equip rules were unavailable. Gear combinations may be limited." or nil
end
local function readableNumber(value)
    if issecretvalue and issecretvalue(value) then return end
    if type(value)=="number" and value==value and value>=0 and value<=2000 and value%1==0 then return value end
end
local function upgradeInfo(link)
    if not C_Item or type(C_Item.GetItemUpgradeInfo)~="function" then return end
    local ok,info=pcall(C_Item.GetItemUpgradeInfo,link)
    if not ok or (issecretvalue and issecretvalue(info)) or type(info)~="table" then return end
    return info
end
local function itemLinkFields(link)
    local prefix,body,suffix=tostring(link or ""):match("^(.-Hitem:)([^|]+)(|h.*)$")
    if not body then return end
    local fields={};for value in (body..":"):gmatch("(.-):")do fields[#fields+1]=value end
    local count=tonumber(fields[13]);if not count or count<1 or count>64 or #fields<13+count then return end
    return prefix,fields,suffix,count
end
local function replaceBonus(prefix,fields,suffix,index,value)
    local copy={};for i,field in ipairs(fields)do copy[i]=field end;copy[13+index]=tostring(value)
    return prefix..table.concat(copy,":")..suffix
end
-- Retail may protect item levels and may not cache a constructed rank link.
-- These are the exact 12.1 rank bonus levels. The game's readable current/max
-- ranks still choose the path; overlapping ranks are never guessed from the
-- bonus alone.
local upgradeRankLevels={
    [12815]=259,[12816]=263,[12817]=266,[12818]=269,[12819]=272,[12820]=276,
    [12821]=266,[12822]=269,[12823]=272,[12824]=276,[12825]=279,[12826]=282,
    [12827]=285,[12828]=289,[12829]=292,[12830]=295,
    [12833]=292,[12834]=295,[12835]=298,[12836]=302,[12837]=305,[12838]=308,
    [12841]=305,[12842]=308,[12843]=311,[12844]=315,[12845]=318,[12846]=321,
    [12849]=318,[12850]=321,[12851]=324,[12852]=328,[12853]=331,[12854]=334,
}
local function upgradeRanks(info)
    local current=info and readableNumber(info.currentLevel);local last=info and readableNumber(info.maxLevel)
    local label=info and info.trackString
    if issecretvalue and issecretvalue(label) then label=nil end
    local track,shownCurrent,shownLast
    if type(label)=="string" then
        track,shownCurrent,shownLast=label:match("^%s*(.-)%s+(%d+)%s*/%s*(%d+)")
        if track then track=track:match("^%s*(.-)%s*$");if track==""or#track>40 then track=nil end end
    end
    return current or readableNumber(tonumber(shownCurrent)),last or readableNumber(tonumber(shownLast)),track
end
local upgradeTrackByMaximum={[282]="Adventurer",[295]="Veteran",[308]="Champion",[321]="Hero",[334]="Myth"}
local function tooltipUpgradeTrack(link,current,last)
	local api=C_TooltipInfo and C_TooltipInfo.GetHyperlink;if type(api)~="function"then return end
	local ok,data=pcall(api,link);if not ok or type(data)~="table"or type(data.lines)~="table"then return end
	for _,line in ipairs(data.lines)do
		local text=line and line.leftText
		if not(issecretvalue and issecretvalue(text))and type(text)=="string"then
			text=text:gsub("|c%x%x%x%x%x%x%x%x",""):gsub("|r","")
			local track,shownCurrent,shownLast=text:match("([%a][%a'%-]*)%s+(%d+)%s*/%s*(%d+)")
			if track and tonumber(shownCurrent)==current and tonumber(shownLast)==last and #track<=40 then return track end
		end
	end
end
local function itemUpgradeLevels(link,level)
    level=readableNumber(level);if not level then return end
    local info=upgradeInfo(link);local maximum=info and info.maxItemLevel
    maximum=readableNumber(maximum)
    if maximum and maximum<=level then maximum=nil end
    local current,last,track=upgradeRanks(info);if not current or not last or last<=current then return nil,maximum end
	track=track or tooltipUpgradeTrack(link,current,last)
    local prefix,fields,suffix,count=itemLinkFields(link);if not prefix then return nil,maximum end
    local fallbackNext,fallbackMax=0,maximum or 0;local bestPath
    for index=1,count do
        local bonus=tonumber(fields[13+index])
        local path=setmetatable({level},array);local valid=bonus~=nil;local previous=level
        for rank=current+1,last do
            local target=bonus and bonus+(rank-current);local variant=target and replaceBonus(prefix,fields,suffix,index,target)
            local variantInfo=variant and upgradeInfo(variant)
            local variantCurrent=variantInfo and readableNumber(variantInfo.currentLevel)
            local variantMaximum=variantInfo and readableNumber(variantInfo.maxLevel)
            local knownLevel=target and upgradeRankLevels[target]
            local recognized=knownLevel or variantCurrent==rank and variantMaximum==last
            local value
            if recognized and C_Item and type(C_Item.GetDetailedItemLevelInfo)=="function" then
                local ok,result=pcall(C_Item.GetDetailedItemLevelInfo,variant);value=ok and readableNumber(result)
            end
            value=value or knownLevel
            if not value or value<=previous then valid=false;break end
            path[#path+1]=value;previous=value
            if rank==current+1 and (fallbackNext==0 or value<fallbackNext)then fallbackNext=value end
            if rank==last and value>fallbackMax then fallbackMax=value end
        end
        if valid and #path==last-current+1 and (not maximum or path[#path]==maximum)then bestPath=path;break end
    end
    if bestPath then return bestPath[2],bestPath[#bestPath],track or upgradeTrackByMaximum[bestPath[#bestPath]],current,last,bestPath end
    return fallbackNext>level and fallbackNext or nil,fallbackMax>level and fallbackMax or nil
end
local function itemMaxLevel(link,level)
    local _,maximum=itemUpgradeLevels(link,level);return maximum
end
-- Journal reads are asynchronous. Never label a partial cache as a complete
-- drop pool, and never silently omit uncached tokens/unsupported equipment.
local function releaseJournalFence(self)
    local fence=self.journalFence;if not fence then return end
    self.journalFence=nil
    if fence.frame:GetScript("OnEvent")==fence.handler then fence.frame:SetScript("OnEvent",fence.original) end
end
local function fenceJournalRestore(self,scan)
    local frame,old=scan.journalFrame,scan.old
    if not frame or not old.difficultyEvent then return end
    releaseJournalFence(self)
    local fence={frame=frame,original=frame:GetScript("OnEvent")}
    if type(fence.original)~="function" then return end
    fence.handler=function(owner,event,...)
        -- Another addon may wrap this function. Once released, an indirect
        -- reference must be transparent too, not keep suppressing old events.
        if self.journalFence~=fence then return fence.original(owner,event,...) end
        if event=="EJ_DIFFICULTY_UPDATE" then
            -- A native notification may arrive after registration is restored.
            -- The hidden view already has this state; rebuilding it would reset
            -- tabs/home subviews. No timer or assumed delivery order is needed.
            if not owner:IsShown() and EJ_GetDifficulty()==old.difficulty
                and owner.instanceID==old.instance and owner.encounterID==old.encounter then return end
            releaseJournalFence(self)
        end
        return fence.original(owner,event,...)
    end
    frame:SetScript("OnEvent",fence.handler);self.journalFence=fence
    if self.journalShowHookFrame~=frame then
        frame:HookScript("OnShow",function(owner)
            if self.journalFence and self.journalFence.frame==owner then releaseJournalFence(self) end
        end)
        self.journalShowHookFrame=frame
    end
end
function S:CancelLoot()
    local scan=self.scan;if not scan then return end
    self.scan=nil
    local old=scan.old
    -- Difficulty events refresh the journal even while it is hidden. Restore
    -- its Lua selection before native setters can trigger that refresh.
    if EncounterJournal then
        EncounterJournal.instanceID=old.instance
        EncounterJournal.encounterID=old.encounter
    end
    pcall(fenceJournalRestore,self,scan)
    -- One failed restore must not prevent the remaining filters being restored.
    if old.tier then pcall(EJ_SelectTier,old.tier) end
    if old.instance then pcall(EJ_SelectInstance,old.instance) end
    if old.encounter then pcall(EJ_SelectEncounter,old.encounter) end
    if old.difficulty then pcall(EJ_SetDifficulty,old.difficulty) end
    pcall(EJ_SetLootFilter,old.class or 0,old.spec or 0)
    if old.slot then pcall(C_EncounterJournal.SetSlotFilter,old.slot) end
    if old.preview then pcall(C_EncounterJournal.SetPreviewMythicPlusLevel,old.preview) end
    -- Do not call Refresh/DisplayInstance here: they reset tabs, instance
    -- widgets and home/Journeys subviews. Only refresh the loot/filter content
    -- touched by the scan, then restore this frame's original subscription.
    if type(EncounterJournal_LootUpdate)=="function" then pcall(EncounterJournal_LootUpdate) end
    if type(EncounterJournal_SetupDifficultyDropdown)=="function" and scan.journalFrame then
        pcall(EncounterJournal_SetupDifficultyDropdown,scan.journalFrame)
    end
    if scan.journalFrame and old.difficultyEvent then
        pcall(scan.journalFrame.RegisterEvent,scan.journalFrame,"EJ_DIFFICULTY_UPDATE")
    end
end
function S:CollectLoot(done)
    local ej=C_EncounterJournal
    local function unavailable(reason)
        self.loot=nil;self.lootWarnings={"Loot capture unavailable: "..(reason or "character specialization")..". Character data was exported without loot."}
        return false
    end
    if not ej then return unavailable("Encounter Journal API") end
    if not C_Item then return unavailable("item API") end
    if not C_Timer or type(C_Timer.After)~="function" then return unavailable("timer API") end
    for _,name in ipairs({"GetLootInfoByIndex","GetSlotFilter","SetSlotFilter","ResetSlotFilter"}) do
        if type(ej[name])~="function" then return unavailable("C_EncounterJournal."..name) end
    end
    for _,name in ipairs({"DoesItemContainSpec","GetItemInfoInstant","GetDetailedItemLevelInfo"}) do
        if type(C_Item[name])~="function" then return unavailable("C_Item."..name) end
    end
    for _,name in ipairs({"EJ_GetNumLoot","EJ_GetLootFilter","EJ_SetLootFilter","EJ_GetCurrentTier","EJ_SelectTier", "EJ_GetNumTiers",
        "EJ_GetDifficulty","EJ_SetDifficulty","EJ_SelectInstance","EJ_SelectEncounter","EJ_GetInstanceByIndex","EJ_GetEncounterInfo"}) do
        if type(_G[name])~="function" then return unavailable(name) end
    end
    assert(not InCombatLockdown(),"Finish combat before reading loot tables.")
    if C_AddOns and C_AddOns.LoadAddOn then C_AddOns.LoadAddOn("Blizzard_EncounterJournal") end
    assert(not EncounterJournal or not EncounterJournal:IsShown(),"Close the Adventure Guide, then start the analysis again.")
    local specAPI=C_SpecializationInfo
    if not specAPI or type(specAPI.GetSpecialization)~="function" or type(specAPI.GetSpecializationInfo)~="function" then return unavailable() end
    local index=plain(specAPI.GetSpecialization())
    local spec=index and plain(specAPI.GetSpecializationInfo(index))
    local _,_,classID=UnitClass("player");classID=plain(classID)
    if not spec or not classID then return unavailable() end
    local season=0
    if C_SeasonInfo and C_SeasonInfo.GetCurrentDisplaySeasonID then season=plain(C_SeasonInfo.GetCurrentDisplaySeasonID()) or 0 end
    local oldClass,oldSpec=EJ_GetLootFilter()
    local scan={spec=spec,classID=classID,season=season,tasks={},pools=setmetatable({},array),index=0,started=GetTime(),totalItems=0,reasons={},requestedItems={},
        old={class=oldClass,spec=oldSpec,tier=EJ_GetCurrentTier(),difficulty=EJ_GetDifficulty(),
            instance=EncounterJournal and EncounterJournal.instanceID,encounter=EncounterJournal and EncounterJournal.encounterID,
            slot=ej.GetSlotFilter(),preview=previewLevel}}
    self.scan=scan;self.loot=nil;self.lootWarnings=nil
    -- Keep only the final poll's failures. Retried cache misses must not become
    -- warnings after the item resolves. Samples identify the source/item without
    -- exporting another unbounded diagnostic payload.
    local reasonLabels={
        dungeonAPI="missing dungeon APIs",dungeonMaps="empty dungeon map list",mapping="unmapped dungeon maps",
        difficulty="unsupported instance/difficulty",validator="difficulty check unavailable",
        empty="empty loot tables",cache="uncached items",eligibility="unknown spec eligibility",
        equipment="unsupported eligible equipment",details="invalid item levels/IDs",
        bosses="missing boss mappings",variants="conflicting item variants",noItems="no eligible loot",
        preview="M+ preview level mismatch",selection="journal difficulty mismatch",
    }
    local function reason(reasons,key,id)
        local entry=reasons[key]
        if not entry then entry={count=0,ids={},seen={}};reasons[key]=entry end
        entry.count=entry.count+1
        if id~=nil then
            id=tostring(id)
            if #id<=32 and not entry.seen[id] and #entry.ids<6 then entry.ids[#entry.ids+1]=id;entry.seen[id]=true end
        end
    end
    local function mergeReasons(reasons)
        for key,entry in pairs(reasons) do
            local old=scan.reasons[key]
            if not old then scan.reasons[key]=entry
            else
                old.count=old.count+entry.count
                for _,id in ipairs(entry.ids) do
                    if not old.seen[id] and #old.ids<6 then old.ids[#old.ids+1]=id;old.seen[id]=true end
                end
            end
        end
    end
    local function bounded(ok,message)
        if not ok then scan.tooLarge=true;error(message) end
    end
    local function dropCatalog()
        self:CancelLoot();self.loot=nil
        self.lootWarnings={"Loot tables exceeded the capture limit. Character data was exported without loot."}
        done()
    end
    local ok,problem=pcall(function()
        -- Blizzard's hidden difficulty listener rebuilds the entire selected
        -- view. Suspend only it; other frames still receive the native event.
        -- Cancellation (including setup failures) restores its subscription.
        if EncounterJournal then
            releaseJournalFence(self)
            assert(type(EncounterJournal.IsEventRegistered)=="function" and type(EncounterJournal.UnregisterEvent)=="function"
                and type(EncounterJournal.RegisterEvent)=="function" and type(EncounterJournal.GetScript)=="function"
                and type(EncounterJournal.SetScript)=="function" and type(EncounterJournal.HookScript)=="function",
                "The journal event state is unavailable.")
            assert(type(EncounterJournal:GetScript("OnEvent"))=="function","The journal event handler is unavailable.")
            scan.journalFrame=EncounterJournal
            scan.old.difficultyEvent=plain(EncounterJournal:IsEventRegistered("EJ_DIFFICULTY_UPDATE"))
            assert(type(scan.old.difficultyEvent)=="boolean","The journal event state is unavailable.")
            EncounterJournal:UnregisterEvent("EJ_DIFFICULTY_UPDATE")
        end
        -- These requests populate the client's preview/reward caches. They do
        -- not authorize substituting Vault rewards for end-of-run loot.
        if C_MythicPlus then
            if type(C_MythicPlus.RequestMapInfo)=="function" then pcall(C_MythicPlus.RequestMapInfo) end
            if type(C_MythicPlus.RequestRewards)=="function" then pcall(C_MythicPlus.RequestRewards) end
        end
        local taskIDs={}
        local function task(value)
            local key=value.type..":"..value.instanceID..":"..(value.challengeMapID or 0)
            if taskIDs[key] then return end
            taskIDs[key]=true;bounded(#scan.tasks<64,"Too many loot sources. Capture stopped.")
            scan.tasks[#scan.tasks+1]=value
        end
        if C_ChallengeMode and C_ChallengeMode.GetMapTable and C_ChallengeMode.GetMapUIInfo and ej.GetInstanceForGameMap and ej.SetPreviewMythicPlusLevel then
            local maps=plain(C_ChallengeMode.GetMapTable()) or {}
            bounded(#maps<=64,"Too many active dungeon maps. Capture stopped.")
            if #maps==0 then scan.missing=true;reason(scan.reasons,"dungeonMaps") end
            for _,map in ipairs(maps) do
                local name,_,_,_,_,gameMap=C_ChallengeMode.GetMapUIInfo(map)
                local instance=gameMap and ej.GetInstanceForGameMap(plain(gameMap))
                if instance then task({instanceID=plain(instance),challengeMapID=plain(map),name=plain(name),type="Dungeon",difficultyID=self.lootKey==0 and 23 or 8,keyLevel=self.lootKey or 10})
                else scan.missing=true;reason(scan.reasons,"mapping",plain(map)) end
            end
        else scan.missing=true;reason(scan.reasons,"dungeonAPI") end
        EJ_SelectTier(EJ_GetNumTiers())
        for i=1,33 do
            local instance,name=EJ_GetInstanceByIndex(i,true)
            if not instance then break end
            bounded(i<=32,"Too many raid instances. Capture stopped.")
            task({instanceID=plain(instance),name=plain(name),type="Raid",difficultyID=self.lootRaid or 15,keyLevel=0})
        end
        scan.taskCount=#scan.tasks
        EJ_SetLootFilter(classID,spec);ej.ResetSlotFilter()
        scan.slot=plain(ej.GetSlotFilter())
    end)
    if not ok then
        if scan.tooLarge then dropCatalog();return true end
        self:CancelLoot();error(problem)
    end
    local function finish()
        if #scan.pools>0 then
            local catalog={schema=1,spec=spec,season=season,capturedAt=plain(GetServerTime()),pools=scan.pools}
            if #json(catalog)>110*1024 then dropCatalog();return end
            self.loot=catalog
        end
        self:CancelLoot()
        if scan.missing or #scan.pools==0 then
            self.lootWarnings={"Some loot tables were unavailable. Start the analysis again to retry."}
            local keys={};for key in pairs(scan.reasons) do keys[#keys+1]=key end;table.sort(keys)
            for _,key in ipairs(keys) do
                local entry=scan.reasons[key]
                local sample=#entry.ids>0 and " ["..table.concat(entry.ids,",").."]" or ""
                self.lootWarnings[#self.lootWarnings+1]="Loot capture: "..reasonLabels[key].." "..entry.count..sample.."."
            end
        end
        done()
    end
    local step
    step=function()
        if S.scan~=scan then return end
        local success,errorMessage=pcall(function()
            assert(not InCombatLockdown(),"Loot scan stopped for combat. Start the analysis again afterwards.")
            assert(not EncounterJournal or not EncounterJournal:IsShown(),"Loot scan stopped because the Adventure Guide opened.")
            assert(GetTime()-scan.started<55,"Loot scan timed out. Start the analysis again.")
            local current=scan.current
            if not current then
                scan.index=scan.index+1;current=scan.tasks[scan.index];scan.current=current
                if not current then finish();return end
                -- Mirror Blizzard's DisplayInstance sequence. A hidden journal
                -- still refreshes on EJ_DIFFICULTY_UPDATE and would otherwise
                -- reselect the user's old encounter during this delayed scan.
                if EncounterJournal then
                    EncounterJournal.instanceID=current.instanceID
                    EncounterJournal.encounterID=nil
                end
                EJ_SelectInstance(current.instanceID)
                -- Normal raid/Mythic difficulties use the journal mask. M+
                -- preview is different: the live mask rejects 8 even when its
                -- explicit preview works. Set its key first, select 8, then
                -- require the journal to report 8 before accepting any items.
                local validate=type(EJ_IsValidInstanceDifficulty)=="function" and EJ_IsValidInstanceDifficulty or ej.InstanceHasDifficultyID
                local plus=current.type=="Dungeon" and current.keyLevel>0
                if plus then ej.SetPreviewMythicPlusLevel(current.keyLevel) end
                current.journalDifficultyID=current.difficultyID
                local supported
                if plus then supported=true
                elseif type(validate)=="function" then supported=plain(validate(current.difficultyID)) end
                if type(supported)=="boolean" then
                    if not supported then
                        reason(scan.reasons,"difficulty",current.instanceID.."/"..current.difficultyID)
                        scan.missing=true;scan.current=nil;C_Timer.After(0.05,step);return
                    end
                else current.unverifiedDifficulty=true;reason(scan.reasons,"validator",current.instanceID) end
                EJ_SetDifficulty(current.journalDifficultyID)
                scan.waited=0;scan.signature=nil
                S:SetStatus("Reading loot "..scan.index.."/"..scan.taskCount.."...")
                C_Timer.After(0.25,step);return
            end
            -- Verify again after asynchronous refreshes, before accepting any
            -- rows under this source/spec. External filter changes stop the scan.
            local currentClass,currentSpec=EJ_GetLootFilter()
            assert((not EncounterJournal or (plain(EncounterJournal.instanceID)==current.instanceID
                    and plain(EncounterJournal.encounterID)==nil))
                and plain(currentClass)==classID and plain(currentSpec)==spec
                and type(scan.slot)=="number" and plain(ej.GetSlotFilter())==scan.slot,
                "Loot scan stopped because the journal selection changed. Start the analysis again.")
            local count=plain(EJ_GetNumLoot())
            bounded(type(count)=="number" and count>=0 and count<=2048 and count%1==0,"Loot table exceeds the capture limit.")
            local complete=count>0 and not current.unverifiedDifficulty;local items={};local signature={};local reasons={};local badBosses={}
            if count==0 then reason(reasons,"empty",current.instanceID) end
            local plus=current.type=="Dungeon" and current.keyLevel>0
            local rewardLevel;local previewSelected=true
            if plus then
                if C_MythicPlus and type(C_MythicPlus.GetRewardLevelForDifficultyLevel)=="function" then
                    local ok,_,value=pcall(C_MythicPlus.GetRewardLevelForDifficultyLevel,current.keyLevel)
                    if ok then value=plain(value);if type(value)=="number" and value>=1 and value<=2000 and value%1==0 then rewardLevel=value end end
                end
                if plain(EJ_GetDifficulty())~=current.journalDifficultyID then
                    complete=false;previewSelected=false;reason(reasons,"selection",current.instanceID)
                end
            end
            local function nonGear(info)
                if not info then return false end
                local value=plain(info.link) or plain(info.itemID)
                if not value then return false end
                for _,name in ipairs({"IsDecorItem","IsCosmeticItem"}) do
                    if type(C_Item[name])=="function" then
                        local ok,result=pcall(C_Item[name],value)
                        if ok and plain(result)==true then return true end
                    end
                end
                local id=plain(info.itemID)
                if type(id)=="number" and id>0 and C_ToyBox and type(C_ToyBox.GetToyInfo)=="function" then
                    local ok,toyID,toyName=pcall(C_ToyBox.GetToyInfo,id)
                    if ok then
                        toyID=plain(toyID);toyName=plain(toyName)
                        if toyID==id and type(toyName)=="string" and toyName~="" then return true end
                    end
                end
                local _,_,_,_,_,itemClass,itemSubclass=C_Item.GetItemInfoInstant(value)
                itemClass=plain(itemClass);itemSubclass=plain(itemSubclass)
                local classes=Enum and Enum.ItemClass
                local misc=Enum and Enum.ItemMiscellaneousSubclass
                -- Keep other miscellaneous items: they can be redeemable tier
                -- tokens. Only explicit collection categories are non-gear.
                return not not (classes and ((classes.Recipe and itemClass==classes.Recipe)
                    or (classes.Housing and itemClass==classes.Housing)
                    or (classes.Battlepet and itemClass==classes.Battlepet)
                    or (classes.Miscellaneous and itemClass==classes.Miscellaneous and misc
                        and ((misc.Mount and itemSubclass==misc.Mount) or (misc.CompanionPet and itemSubclass==misc.CompanionPet)))))
            end
            local function cached(id)
                if not id then return not plus end
                if type(C_Item.IsItemDataCachedByID)~="function" then return not plus end
                local ok,ready=pcall(C_Item.IsItemDataCachedByID,id)
                if ok and plain(ready)==true then return true end
                if not scan.requestedItems[id] and type(C_Item.RequestLoadItemDataByID)=="function" then
                    scan.requestedItems[id]=true;pcall(C_Item.RequestLoadItemDataByID,id)
                end
                return false
            end
            local function encountersFor(info,index,id)
                if current.type~="Raid" then return {0},true end
                local encounters={};local known=true
                local bosses=EJ_GetNumEncountersForLootByIndex and plain(EJ_GetNumEncountersForLootByIndex(index))
                if not bosses then known=false;bosses=1 end
                bounded(type(bosses)=="number" and bosses>=0 and bosses<=64 and bosses%1==0,"Loot boss list exceeds the capture limit.")
                if bosses==0 then known=false end
                for boss=1,bosses do
                    local row=boss==1 and info or plain(ej.GetLootInfoByIndex(index,boss))
                    local encounter=row and plain(row.encounterID)
                    if encounter and encounter>0 then encounters[#encounters+1]=encounter else known=false end
                end
                if not known then complete=false;reason(reasons,"bosses",id) end
                return encounters,known
            end
            local function failRow(key,id,encounters,known)
                reason(reasons,key,id)
                if current.type=="Raid" and known then
                    for _,encounter in ipairs(encounters) do badBosses[encounter]=true end
                else complete=false end
            end
            for i=1,count do
                local info=plain(ej.GetLootInfoByIndex(i))
                local itemSeason=info and plain(info.displaySeasonID)
                if info and itemSeason and itemSeason~=season then
                    -- A known older-season row cannot enter this pool, even if
                    -- its unrelated item cache has not loaded yet.
                elseif nonGear(info) then
                    -- Gear probabilities exclude explicitly identified recipes,
                    -- housing and appearance-only rewards. Unknown tokens stay.
                elseif not info or not plain(info.link) or not plain(info.name) then
                    local id=info and plain(info.itemID)
                    local encounters,known=encountersFor(info,i,id)
                    failRow("cache",id,encounters,known)
                else
                    local link=info.link
                    local eligible=plain(C_Item.DoesItemContainSpec(link,classID,spec))
                    if eligible==nil then
                        local id=plain(info.itemID);local encounters,known=encountersFor(info,i,id)
                        failRow("eligibility",id,encounters,known)
                    elseif eligible then
                        local _,_,_,equipLoc=C_Item.GetItemInfoInstant(link);equipLoc=plain(equipLoc)
                        local slot=lootSlots[equipLoc]
                        local level=plain(C_Item.GetDetailedItemLevelInfo(link))
                        local id=plain(info.itemID)
                        local encounters,known=encountersFor(info,i,id)
                        if not cached(id) then failRow("cache",id,encounters,known)
                        elseif not slot then failRow("equipment",id,encounters,known)
                        elseif type(level)~="number" or level<=0 or type(id)~="number" or id<=0 then failRow("details",id,encounters,known)
                        elseif plus and not previewSelected then
                            -- Failed selection must never relabel M0 as M+.
                        elseif plus and rewardLevel and level~=rewardLevel then failRow("preview",id,encounters,known)
                        else
                            local veryRare=plain(info.displayAsVeryRare)
                            local extremelyRare=plain(info.displayAsExtremelyRare)
                            local perPlayer=plain(info.displayAsPerPlayerLoot)
                            local item={id=id,label=info.name,slot=slot,link=link,level=level,inventoryType=equipLoc,
                                rare=not not (veryRare or extremelyRare or perPlayer),encounters=encounters}
                            item.maxLevel=itemMaxLevel(link,level)
                            local rule=itemRule(link)
                            if rule then
                                item.rulesKnown=true;item.unique=rule.unique;item.limitCategory=rule.limitCategory;item.limitMax=rule.limitMax
                            end
                            items[#items+1]=item
                            signature[#signature+1]=link..":"..level..":"..table.concat(encounters,",")
                        end
                    end
                end
            end
            local fingerprint=count..":"..(rewardLevel or 0)..":"..table.concat(signature,"|")
            scan.waited=scan.waited+0.25
            if (not complete or next(badBosses) or fingerprint~=scan.signature) and scan.waited<2 then
                scan.signature=fingerprint;C_Timer.After(0.25,step);return
            end
            local groups={}
            local function poolFor(encounter)
                local pool=groups[encounter]
                if not pool then
                    local name=current.name
                    if encounter>0 then name=plain(EJ_GetEncounterInfo(encounter)) or name end
                    pool={instanceID=current.instanceID,encounterID=encounter,challengeMapID=current.challengeMapID or 0,
                        name=name,type=current.type,difficultyID=current.difficultyID,keyLevel=current.keyLevel,items=setmetatable({},array),seen={}}
                    if plus then pool.journalDifficultyID=current.journalDifficultyID;pool.previewRewardLevel=rewardLevel end
                    groups[encounter]=pool
                end
                return pool
            end
            if current.type=="Dungeon" and (not plus or (#items>0 and previewSelected)) then poolFor(0) end
            for encounter in pairs(badBosses) do poolFor(encounter) end
            for _,item in ipairs(items) do
                for _,encounter in ipairs(item.encounters) do
                    local pool=poolFor(encounter)
                    if not pool.seen[item.id] then
                        pool.seen[item.id]=item.link
                        local copy={};for key,value in pairs(item) do if key~="encounters" then copy[key]=value end end
                        pool.items[#pool.items+1]=copy;scan.totalItems=scan.totalItems+1
                        bounded(scan.totalItems<=512,"Loot items exceed the capture limit.")
                    elseif pool.seen[item.id]~=item.link then
                        if current.type=="Raid" then badBosses[encounter]=true else complete=false end
                        reason(reasons,"variants",item.id)
                    end
                end
            end
            if not complete or next(badBosses) or not next(groups) then scan.missing=true end
            if not next(groups) and count>0 then reason(reasons,"noItems",current.instanceID) end
            mergeReasons(reasons)
            for _,pool in pairs(groups) do
                pool.seen=nil;pool.complete=complete and not badBosses[pool.encounterID]
                bounded(#scan.pools<64,"Loot sources exceed the capture limit.")
                scan.pools[#scan.pools+1]=pool
            end
            scan.current=nil
            C_Timer.After(0.05,step)
        end)
        if not success then
            if scan.tooLarge then dropCatalog() else S:CancelLoot();S:SetStatus(tostring(errorMessage)) end
        end
    end
    step();return true
end
-- Out-of-combat, public character-sheet capture for the experimental healing
-- model. A partial capture is omitted, never replaced by plausible zeroes.
local healerSpells={
    [105]={8936,774,33763,18562,48438,145205,740},
    [256]={2061,17,194509,585,47540},
    [257]={2061,596,2050,34861,33076,64843},
    [264]={77472,8004,61295,1064,73920,5394,108280},
    [270]={116670,115151,124682,115310},
    [65]={19750,82326,20473,85673,85222,31884,53563},
    [1468]={361469,364343,366155,355936,360995,355913,359816,363534,370537,373861,1291636},
}
function S:HealingStats(spec)
    spec=spec or 1468;assert(healerSpells[spec],"Choose a healer specialization.")
    assert(not InCombatLockdown(),"Finish combat before capturing healing stats.")
    local function number(value,positive)
        value=plain(value)
        assert(type(value)=="number" and value==value and value>=0 and value<math.huge and (not positive or value>0),"Healing stats are not ready.")
        return value
    end
    local _,regen=GetManaRegen()
    local vers=number(GetCombatRatingBonus(CR_VERSATILITY_DAMAGE_DONE or 29))
    if GetVersatilityBonus then vers=vers+number(GetVersatilityBonus(CR_VERSATILITY_DAMAGE_DONE or 29)) end
    local stats={spellPower=number(GetSpellBonusHealing(),true),health=number(UnitHealthMax("player"),true),
        mana=number(UnitPowerMax("player",0),true),manaRegen=number(regen),crit=number(GetSpellCritChance((spec==65 or spec==256 or spec==257) and 2 or 4))/100,
        haste=number(UnitSpellHaste("player"))/100,mastery=number(GetMasteryEffect())/100,versatility=vers/100,talents={}}
    assert(C_SpellBook and C_SpellBook.IsSpellKnown,"Spellbook is not ready.")
    stats.knownSpells={}
    for _,spell in ipairs(healerSpells[spec])do
        local known=plain(C_SpellBook.IsSpellKnown(spell));assert(type(known)=="boolean","Spellbook is not ready.")
        local override=C_Spell and C_Spell.GetOverrideSpell and C_Spell.GetOverrideSpell(spell)
        assert(not (issecretvalue and issecretvalue(override)),"Spell overrides are not ready.")
        assert(override==nil or override==false or type(override)=="number","Spell overrides are not ready.")
        -- A loaded Stasis changes its button to release; it is still the same
        -- learned cooldown, unlike an unmodeled talent replacing a spell.
        stats.knownSpells[tostring(spell)]=known and (not override or override==0 or override==spell or spell==370537 and override==370564)
    end
    -- Barrier is a supported replacement, captured under its own spell ID.
    -- Some spellbook views expose the learned base spell as well as its replacement.
    if spec==1468 and stats.knownSpells['1291636'] then stats.knownSpells['373861']=false end
    local config=number(C_ClassTalents.GetActiveConfigID(),true)
    assert(not plain(C_Traits.ConfigHasStagedChanges(config)),"Apply your staged talents before capturing.")
    local info=plain(C_Traits.GetConfigInfo(config));local trees=info and plain(info.treeIDs)
    assert(type(trees)=="table" and #trees>0 and #trees<=8,"Talent trees are not ready.")
    local seen,count={},0
    for _,tree in ipairs(trees)do
        local nodes=plain(C_Traits.GetTreeNodes(number(tree,true)))
        assert(type(nodes)=="table" and #nodes<=500,"Talent nodes are not ready.")
        for _,nodeID in ipairs(nodes)do
            nodeID=number(nodeID,true)
            if not seen[nodeID] then
                seen[nodeID]=true;count=count+1;assert(count<=600,"Too many talent nodes.")
                local node=plain(C_Traits.GetNodeInfo(config,nodeID));assert(type(node)=="table","Talent node is not ready.")
                local rank=number(node.activeRank)
                if plain(node.subTreeActive)~=false and rank>0 then
                    local active=plain(node.activeEntry);assert(type(active)=="table","Talent entry is not ready.")
                    local entry=plain(C_Traits.GetEntryInfo(config,number(active.entryID,true)))
                    assert(type(entry)=="table","Talent definition is not ready.")
                    local definitionID=plain(entry.definitionID)
                    if definitionID and definitionID>0 then
                        local definition=plain(C_Traits.GetDefinitionInfo(definitionID));assert(type(definition)=="table","Talent spell is not ready.")
                        local spell=plain(definition.spellID)
                        if spell then stats.talents[tostring(number(spell,true))]=rank end
                    else assert(plain(entry.subTreeID),"Talent definition is missing.") end
                end
            end
        end
    end
    return stats
end
function S:Snapshot()
    assert(not InCombatLockdown(),"Finish combat before exporting your character.")
    local _,build,_,interface=GetBuildInfo()
    interface=plain(interface)
    local client=interface==16001 and "forever" or "retail"
    assert(interface==16001 or interface>=100000,"This WoW client is not supported.")
    local guid=plain(UnitGUID("player"));assert(type(guid)=="string","Your character is not ready yet.")
    local name,realm=UnitFullName("player")
    local _,class=UnitClass("player")
    local spec,role=0,"UNKNOWN"
    local api=C_SpecializationInfo
    local getSpec=api and api.GetSpecialization or GetSpecialization
    local getInfo=api and api.GetSpecializationInfo or GetSpecializationInfo
    if getSpec and getInfo then
        local index=plain(getSpec())
        if index then
            -- Role is the fifth return; the sixth is the numeric primary stat.
            local id,_,_,_,r=getInfo(index)
            spec=plain(id) or 0;r=plain(r)
            if r=="DAMAGER" or r=="HEALER" or r=="TANK" then role=r end
        end
    end
    self.sequence=self.sequence+1
    local captured=plain(GetServerTime())
    local id=guid:gsub("[^%w_-]","").."_"..captured.."_"..math.floor(GetTime()*1000).."_"..self.sequence
    local warnings=setmetatable({},array)
    local items=setmetatable({},array)
    for slot=1,19 do
        local item=plain(GetInventoryItemID("player",slot))
        if item then
            local link=plain(GetInventoryItemLink("player",slot))
            assert(type(link)=="string","An equipped item is still loading. Start the analysis again in a moment.")
            items[#items+1]={slot=slot,link=link}
        end
    end
    local profile=""
    if client=="retail" and SimulationcraftAPI and type(SimulationcraftAPI.GetSimcProfile)=="function" then
        -- The existing exporter's public API supplies item-instance modifiers,
        -- bags, talents and Vault metadata. No exporter UI or macro is opened.
        local result,problem=SimulationcraftAPI:GetSimcProfile(false,false,false,false)
        assert(not problem,plain(problem) or "SimulationCraft could not export your character.")
        profile=plain(result)
        assert(type(profile)=="string" and #profile>0,"The SimulationCraft export is empty.")
    else
        warnings[#warnings+1]=client=="retail" and "Enable Simulationcraft, then export again."
            or "Item stats only. Combat calculations are unavailable."
    end
    local snapshot={schema=1,id=id,client=client,capturedAt=captured,build=tostring(plain(build)),interface=interface,
        character={guid=guid,name=plain(name),realm=plain(realm) or plain(GetRealmName()),class=plain(class),
            spec=spec,role=role,level=plain(UnitLevel("player"))},profile=profile,items=items,warnings=warnings}
    if client=="forever" then snapshot.gear=self:GearSnapshot() end
    if client=="retail" then
        if healerSpells[spec] then
            local ok,stats=pcall(self.HealingStats,self,spec)
            if ok then snapshot.healingStats=stats else warnings[#warnings+1]="Healing Lab needs a complete public stat/talent capture outside combat." end
        end
        local rules,problem=self:RuleSnapshot();snapshot.rules=rules
        if problem then warnings[#warnings+1]=problem end
        local upgrades=self:UpgradeSnapshot(profile)
        if upgrades and #upgrades>0 then snapshot.upgrades=upgrades end
    end
    if client=="retail" and self.loot and self.loot.spec==spec and captured-self.loot.capturedAt>=0 and captured-self.loot.capturedAt<300 then snapshot.loot=self.loot end
    for _,warning in ipairs(self.lootWarnings or {}) do warnings[#warnings+1]=warning end
    local data=json(snapshot)
    if #data>131072 and snapshot.loot then
        snapshot.loot=nil;warnings[#warnings+1]="Loot tables exceeded the transfer limit. Character data was exported without loot."
        data=json(snapshot)
    end
    assert(#data<=131072,"The character export is too large for this transfer. Use the browser import instead.")
    self.lastSnapshot=snapshot;self.gameSnapshot=nil
    return snapshot,data
end
local profileSlots={head=true,neck=true,shoulder=true,back=true,chest=true,shirt=true,tabard=true,wrist=true,
    hands=true,waist=true,legs=true,feet=true,finger1=true,finger2=true,trinket1=true,trinket2=true,main_hand=true,off_hand=true}
local function trim(value)return value:match("^%s*(.-)%s*$")end
local function identityKey(item)
    return table.concat({item.source or "",item.slot or "",item.value or "",item.key or "",item.link or ""},"\n")
end
local function candidateDetails(candidate)
    candidate.itemID=tonumber(candidate.value:match("^id=(%d+)") or candidate.value:match(",id=(%d+)"))
	if candidate.itemID and C_Item and C_Item.GetItemInfoInstant then
		local ok,_,_,_,inventoryType=pcall(C_Item.GetItemInfoInstant,candidate.itemID)
		if ok then candidate.inventoryType=plain(inventoryType) end
	end
    if candidate.itemID and C_Item and C_Item.GetItemInfo then
        -- ID metadata is useful for icons only. It is not the selected variant's
        -- item level or hyperlink; those modifiers remain in identity.value.
        local ok,name,_,quality,_,_,_,_,_,_,icon=pcall(C_Item.GetItemInfo,candidate.itemID)
        if ok then
            candidate.icon=plain(icon);candidate.quality=plain(quality)
            if candidate.label=="" then candidate.label=plain(name) or tostring(candidate.itemID) end
        end
    end
    if candidate.label=="" then candidate.label=tostring(candidate.itemID or candidate.id) end
end
local tooltipFields={id=true,enchant_id=true,gem_id=true,bonus_id=true,ilevel=true,content_tuning=true,redirected_base_stats=true}
local tooltipModifiers={[28]="content_tuning",[64]="redirected_base_stats"}
local function tooltipVariant(candidate,link)
    local wanted={}
    for part in candidate.value:gmatch("[^,]+") do
        local key,value=part:match("^([^=]+)=(.*)$")
        if key then
            -- More complicated exported modifiers remain valid calculations,
            -- but this bounded matcher does not guess their tooltip instance.
            if not tooltipFields[key] or wanted[key] then return end
            wanted[key]=value
        end
    end
    local body=link:match("item:([%-?%d:]+)");if not body then return end
    local fields={};for value in (body..":"):gmatch("(.-):") do fields[#fields+1]=tonumber(value) or 0 end
    if #fields<13 or fields[1]~=tonumber(wanted.id) or fields[2]~=(tonumber(wanted.enchant_id) or 0) then return end
    local gems={};for i=3,6 do gems[#gems+1]=fields[i] end
    while #gems>0 and gems[#gems]==0 do table.remove(gems) end
    if table.concat(gems,"/")~=(wanted.gem_id or "") then return end
    local count=fields[13];if count<0 or count>64 or count%1~=0 or #fields<13+count then return end
    local bonuses={};for i=14,13+count do bonuses[#bonuses+1]=fields[i] end
    if table.concat(bonuses,"/")~=(wanted.bonus_id or "") then return end
    local modifierCount=fields[14+count]or 0
    if modifierCount<0 or modifierCount>64 or modifierCount%1~=0 or #fields<14+count+modifierCount*2 then return end
    local found={}
    for index=1,modifierCount do
        local offset=14+count+(index-1)*2;local key=tooltipModifiers[fields[offset+1]]
        if key then found[key]=tostring(fields[offset+2]) end
    end
    for _,key in ipairs({"content_tuning","redirected_base_stats"})do if (wanted[key]or "")~=(found[key]or "")then return end end
    if wanted.ilevel then
        if not C_Item or not C_Item.GetDetailedItemLevelInfo or plain(C_Item.GetDetailedItemLevelInfo(link))~=tonumber(wanted.ilevel) then return end
    end
    return true
end
function S:GameItemLinks(view)
    local links={equipped={},owned={},vault={}}
    local function add(source,link)
        link=plain(link);if type(link)=="string" then links[source][link]=true end
    end
    local readOK=pcall(function()
        for slot=1,19 do
            local link=GetInventoryItemLink and GetInventoryItemLink("player",slot)
            if link then add("equipped",link) end
        end
        if C_Container and C_Container.GetContainerNumSlots and C_Container.GetContainerItemInfo then
            for bag=0,(NUM_BAG_SLOTS or 4) do
                for slot=1,plain(C_Container.GetContainerNumSlots(bag)) do
                    local item=plain(C_Container.GetContainerItemInfo(bag,slot))
                    if item then add("owned",item.hyperlink) end
                end
            end
        end
        if C_WeeklyRewards and C_WeeklyRewards.GetActivities and C_WeeklyRewards.GetItemHyperlink then
            for _,activity in ipairs(plain(C_WeeklyRewards.GetActivities()) or {}) do
                for _,reward in ipairs(plain(activity.rewards) or {}) do
                    local id=plain(reward.itemDBID);if id then add("vault",C_WeeklyRewards.GetItemHyperlink(id)) end
                end
            end
        end
    end)
    if not readOK then return end
    for _,candidate in ipairs(view.candidates) do
        local match,ambiguous
        for link in pairs(links[candidate.source] or {}) do
            local ok,same=pcall(tooltipVariant,candidate,link)
            if ok and same then
                if match and match~=link then ambiguous=true;break end
                match=link
            end
        end
        if match and not ambiguous then
            candidate.link=match
            if C_Item and C_Item.GetDetailedItemLevelInfo then
                local ok,level=pcall(C_Item.GetDetailedItemLevelInfo,match)
                if ok and type(level)=="number" and not (issecretvalue and issecretvalue(level)) then candidate.level=level end
            end
        end
    end
end
function S:UpgradeSnapshot(profile)
    if type(profile)~="string" or not C_Item or type(C_Item.GetItemUpgradeInfo)~="function" then return setmetatable({},array) end
    local view={candidates={}}
    local source,label="equipped",""
    for raw in (profile.."\n"):gmatch("(.-)\n") do
        local line=trim(raw)
        local comment=line:sub(1,1)=="#"
        if comment then
            if line:sub(1,3)=="###" then
                local lower=line:lower()
                if lower:find("weekly",1,true) or lower:find("vault",1,true) then source="vault"
                elseif lower:find("bags",1,true) then source="owned" else source="other" end
            end
            line=trim(line:gsub("^#+",""))
        end
        local key,value=line:match("^([^=]+)=(.*)$")
        if key and profileSlots and profileSlots[key] and ((comment and (source=="owned" or source=="vault"))or(not comment and source=="equipped"))and #view.candidates<256 then
                value=value:gsub("^,","")
                view.candidates[#view.candidates+1]={id="item"..(#view.candidates+1),label=label,source=source,slot=key,value=value}
        elseif comment and not key and #line>0 and #line<200 then label=line end
    end
    self:GameItemLinks(view)
    local out=setmetatable({},array)
    local order,byKey={},{}
    for _,candidate in ipairs(view.candidates) do
        local level=plain(candidate.level)
        local nextLevel,maximum,track,currentRank,maxRank,rankLevels
        if candidate.link then nextLevel,maximum,track,currentRank,maxRank,rankLevels=itemUpgradeLevels(candidate.link,level) end
        if type(level)=="number" and type(maximum)=="number" and maximum>level then
            local upgrade={source=candidate.source,slot=candidate.slot,value=candidate.value,
                level=level,nextLevel=nextLevel,maxLevel=maximum,track=rankLevels and track or nil,
                currentRank=rankLevels and currentRank or nil,maxRank=rankLevels and maxRank or nil,rankLevels=rankLevels}
            -- The profile retains every owned copy. Upgrade metadata describes
            -- each exact source/slot/value once, independently of display names.
            -- Conflicting cache reads omit only this optional metadata record.
            local key=table.concat({candidate.source,candidate.slot,candidate.value},"\30")
            local signature=json(upgrade);local prior=byKey[key]
            if prior==nil then
                upgrade.label=candidate.label
                byKey[key]={value=upgrade,signature=signature};order[#order+1]=key
            elseif prior and prior.signature~=signature then byKey[key]=false end
        end
    end
    for _,key in ipairs(order)do if byKey[key]then out[#out+1]=byKey[key].value end end
    return out
end
-- This view never enters the wire snapshot. Match ParseProfile's candidate
-- ordering for display, but transmit full identities, not positional itemN IDs.
function S:GameSnapshot(refresh)
    if not refresh and self.gameSnapshot then return self.gameSnapshot end
    local snapshot=self.lastSnapshot
    if refresh or not snapshot then
        local ok,result=pcall(self.Snapshot,self)
        if not ok then return nil,tostring(result) end
        snapshot=result
    end
    local view={};for key,value in pairs(snapshot) do view[key]=value end
    view.candidates={};view.equipped={};view.talents={}
    if snapshot.client=="forever" then
        local hasTwoHand=false
        for _,item in ipairs(snapshot.gear and snapshot.gear.equipped or {}) do
            if item.slot==16 and item.equipLoc=="INVTYPE_2HWEAPON" then hasTwoHand=true end
        end
        for _,item in ipairs(snapshot.gear and snapshot.gear.bags or {}) do
            local slot=lootSlots[item.equipLoc]
            if item.equipLoc=="INVTYPE_BODY" then slot="shirt" end
            if item.equipLoc=="INVTYPE_TABARD" then slot="tabard" end
            if item.equipLoc=="INVTYPE_RANGED" or item.equipLoc=="INVTYPE_RANGEDRIGHT" or item.equipLoc=="INVTYPE_THROWN" or item.equipLoc=="INVTYPE_RELIC" then slot="ranged" end
            if item.statsReady and slot and not (slot=="off_hand" and hasTwoHand) then
                local identity={source="owned",slot=slot,value="id="..item.id,key=item.key,link=item.link}
                local c={id=item.key,label=item.label,source=identity.source,slot=slot,value=identity.value,identity=identity,
                    itemID=item.id,link=item.link,level=item.level,stats=item.stats}
                view.candidates[#view.candidates+1]=c
            end
        end
    else
        local source,label="owned",""
        for raw in (snapshot.profile.."\n"):gmatch("(.-)\n") do
            local line=trim(raw)
			local comment=line:sub(1,1)=="#"
            if comment then
                if line:sub(1,3)=="###" then
                    local lower=line:lower()
                    if lower:find("weekly",1,true) or lower:find("vault",1,true) then source="vault"
                    elseif lower:find("bags",1,true) then source="owned" else source="other" end
                end
                line=trim(line:gsub("^#+",""))
                local key,value=line:match("^([^=]+)=(.*)$")
                if key=="talents" and #value<=2048 and value:match("^[%w+/=]+$") then
                    view.talents[#view.talents+1]={label=label:gsub("^Saved Loadout: ",""),code=value}
                elseif profileSlots[key] and (source=="owned" or source=="vault") and #view.candidates<256 then
                    value=value:gsub("^,","")
                    local c={id="item"..(#view.candidates+1),label=label,source=source,slot=key,value=value,
                        identity={source=source,slot=key,value=value}}
                    candidateDetails(c);view.candidates[#view.candidates+1]=c
                elseif not key and #line>0 and #line<200 then label=line end
			else
				local key,value=line:match("^([^=]+)=(.*)$")
				if profileSlots[key] and value~="" then
					value=value:gsub("^,","")
					local c={id="equipped_"..key,label="",source="equipped",slot=key,value=value,
						identity={source="equipped",slot=key,value=value}}
					candidateDetails(c);view.equipped[#view.equipped+1]=c
				end
            end
        end
        self:GameItemLinks(view)
        local upgradeByItem={}
        for _,upgrade in ipairs(snapshot.upgrades or {}) do
            upgradeByItem[table.concat({upgrade.source,upgrade.slot,upgrade.value},"\30")]=upgrade
        end
		for _,candidate in ipairs(view.equipped)do
			local upgrade=upgradeByItem[table.concat({candidate.source,candidate.slot,candidate.value},"\30")]
			if upgrade then
				candidate.label=upgrade.label or candidate.label
				for _,key in ipairs({"level","nextLevel","maxLevel","track","currentRank","maxRank","rankLevels"})do candidate[key]=upgrade[key] end
			end
		end
        for _,candidate in ipairs(view.candidates) do
            local upgrade=upgradeByItem[table.concat({candidate.source,candidate.slot,candidate.value},"\30")]
            if upgrade then
                for _,key in ipairs({"level","nextLevel","maxLevel","track","currentRank","maxRank","rankLevels"})do candidate[key]=upgrade[key] end
            end
        end
        self:GameItemLinks({candidates=view.equipped})

        -- Retail combat exports contain exact item-instance identities, while
        -- C_Item exposes the contribution printed on each live tooltip. Keep
        -- this richer table in the in-memory view only: the engine still gets
        -- the bounded, independently validated wire snapshot above. The native
        -- gear advisor can therefore react immediately without a reload or a
        -- second calculator, and no new untrusted fields enter saved jobs.
        view.gear=self:GearSnapshot()
        local byLink={}
        local function remember(item)
            if type(item.link)=="string"then
                local body=item.link:match("item:([^|]+)")or item.link
                local bucket=byLink[body]or{};bucket[#bucket+1]=item;byLink[body]=bucket
            end
        end
        for _,item in ipairs(view.gear and view.gear.equipped or{})do remember(item)end
        for _,item in ipairs(view.gear and view.gear.bags or{})do remember(item)end
        local used={}
        local function attach(candidate)
            if type(candidate.link)~="string"then return end
            local body=candidate.link:match("item:([^|]+)")or candidate.link
            local bucket=byLink[body]
            local item
            for _,value in ipairs(bucket or{})do if not used[value.key]then item=value;used[value.key]=true;break end end
            if not item and candidate.source=="vault"and C_Item and C_Item.GetItemStats and C_Item.GetItemInfo then
                item=readGearItem(C_Item,"vault_"..tostring(candidate.id),candidate.itemID,candidate.link,0)
            end
            if not item then return end
            candidate.gearKey=item.key;candidate.stats=item.stats;candidate.statsReady=item.statsReady
            candidate.equipLoc=item.equipLoc;candidate.capturedLevel=item.level
            local canEquip=C_Item and C_Item.IsEquippableItem or IsEquippableItem
            if type(canEquip)=="function"then
                local ok,value=pcall(canEquip,candidate.link)
                if ok and not(issecretvalue and issecretvalue(value))then candidate.equippable=not not value end
            end
        end
        for _,candidate in ipairs(view.equipped)do attach(candidate)end
        for _,candidate in ipairs(view.candidates)do if candidate.source=="owned"or candidate.source=="vault"then attach(candidate)end end
    end
    self.gameSnapshot=view;return view
end
local scenarioDefaults={targets=1,seconds=300,iterations=1000}
local scenarioNumbers={buildSpec={1,2000},targets={1,20},seconds={30,600},iterations={100,20000},keyLevel={0,7},raidDifficulty={0,3},allyItemLevel={0,1000},allySkill={0,1},healingIntensity={10,200},healingRuns={1,100}}
local scenarioWords={fightStyle={patchwerk=true,dungeon_slice=true},raidBuffs={none=true,full=true},bloodlust={on=true,off=true},healingMode={raid=true,dungeon=true},augmentationMode={raid=true,dungeon=true},healingScenario={tank=true,dungeon=true,raid=true,replay=true,learned=true},healingPolicy={consensus=true,reactive=true,compare=true},buildMode={["m+"]=true,solo=true,["2v2"]=true,["3v3"]=true,blitz=true,rbg=true}}
-- Keep these reviewed 12.1.5 recipes aligned with Engine/internal/simming/venomstones.go.
-- Exact captured bonuses establish eligibility; an item level alone never does.
function S:VenomstoneInfo(item)
    local value=item.value or(item.identity and item.identity.value)or""
    local bonuses={};local track,count=0,0
    for raw in (value:match("bonus_id=([^,]+)")or""):gmatch("%d+")do
        local id=tonumber(raw);bonuses[id]=true
        if id==12848 or id==12856 or id==13852 then return{applied=true,reason="Already Venomforged"}end
        if(id>=12841 and id<=12846)or(id>=12849 and id<=12854)then track=id;count=count+1 end
    end
    local slot=item.slot or(item.identity and item.identity.slot)
    if slot~="neck"and slot~="trinket1"and slot~="trinket2"and slot~="main_hand"and slot~="off_hand"then return{reason="Only eligible weapons, trinkets and necklaces use Venomstones."}end
    if bonuses[13751]then
        local quality=value:match("crafting_quality=([^,]+)")
        if count~=0 or(quality~="5"and not(not quality and bonuses[9627]))then return{reason="Maximum-quality Tidal crafted gear is required."}end
        if bonuses[13835]then return{reason="Hero-crafted Venomstone scaling still needs live-client verification."}end
        if bonuses[13836]then return{eligible=true,level=337}end
        return{reason="A verified maximum Tidal crafting cap is required."}
    end
    if count~=1 then return{reason="Requires a Season 2 Hero or Myth track. Elevated gear cannot be upgraded."}end
    return{eligible=true,level=track<=12846 and 328 or 341}
end
local function itemOptions(values)
    assert(type(values)=="table"and #values<=8,"Choose up to eight item alternatives.")
    local out=setmetatable({},array)
    for _,value in ipairs(values)do
        assert(type(value)=="table","Invalid item alternative.");local copy={}
        for key,v in pairs(value)do
            if key=="label"then assert(type(v)=="string"and #v<=80 and not v:find("[%z\r\n]"),"Use a shorter alternative name.");copy[key]=v
            elseif key=="venomstone"then assert(type(v)=="boolean","Invalid Venomstone option.");copy[key]=v
            elseif key=="level"or key=="enchantID"then
                assert(type(v)=="number"and v%1==0 and v>=0 and v<=(key=="level"and 2000 or 10000000),"Invalid item alternative.");copy[key]=v
            elseif key=="gemIDs"then
                assert(type(v)=="table"and #v<=4,"Invalid gem alternatives.");copy.gemIDs=setmetatable({},array)
                for _,id in ipairs(v)do assert(type(id)=="number"and id%1==0 and id>=0 and id<=10000000,"Invalid gem ID.");copy.gemIDs[#copy.gemIDs+1]=id end
            else error("Unknown item alternative field: "..tostring(key))end
        end
        assert(copy.venomstone or(copy.level or 0)>0 or copy.enchantID~=nil or #(copy.gemIDs or{})>0,"Item alternative has no change.")
        out[#out+1]=copy
    end
    return out
end
local function encounterOptions(values)
    assert(type(values)=="table"and #values<=4,"Choose up to four encounters.")
    local out=setmetatable({},array);local seen={}
    for _,value in ipairs(values)do
        assert(type(value)=="table","Invalid encounter preset.");local copy={}
        for key,v in pairs(value)do
            if key=="id"then assert(type(v)=="string"and #v<=64 and v:match("^[a-z][a-z0-9_]*$"),"Invalid encounter ID.")
            elseif key=="name"then assert(type(v)=="string"and #v>0 and #v<=80 and not v:find("[%z\r\n]"),"Use a short encounter name.")
            elseif key=="targets"or key=="seconds"then local range=scenarioNumbers[key];assert(type(v)=="number"and v%1==0 and v>=range[1]and v<=range[2],"Invalid encounter setting.")
            elseif key=="fightStyle"or key=="raidBuffs"or key=="bloodlust"then assert(scenarioWords[key][v],"Invalid encounter setting.")
            else error("Unknown encounter field: "..tostring(key))end
            copy[key]=v
        end
        assert(copy.id and copy.name and copy.targets and copy.seconds and copy.fightStyle and copy.raidBuffs and not seen[copy.id],"Complete each encounter and use distinct IDs.")
        seen[copy.id]=true;out[#out+1]=copy
    end
    return out
end
local function gameOptions(options)
    assert(type(options)=="table","Choose a calculation.")
    local tasks={baseline=true,compare=true,talents=true,upgrades=true,analysis=true,cancel=true,plan=true,healing=true,builds=true}
    assert(tasks[options.task],"Choose a calculation.")
    local request={schema=1,task=options.task,scenario={},selected=setmetatable({},array),talents=setmetatable({},array)}
    for key,value in pairs(scenarioDefaults) do request.scenario[key]=value end
    for key,value in pairs(options.scenario or {}) do
        local range=scenarioNumbers[key];local words=scenarioWords[key]
        if range then
            assert(type(value)=="number" and value>=range[1] and value<=range[2] and (key=="allySkill" or value%1==0),"Invalid encounter setting: "..key)
            assert(key~="allySkill" or value==0 or value>=0.01,"Ally performance must be 1–100%.")
            request.scenario[key]=value
        elseif key=="encounters"then request.scenario.encounters=encounterOptions(value)
        elseif key=="healingCohort" then
            assert(type(value)=="string" and #value<=256,"Invalid learned encounter selection.");request.scenario[key]=value
        elseif words then
            assert(words[value],"Invalid encounter setting: "..key);request.scenario[key]=value
        else error("Unknown encounter setting: "..tostring(key)) end
    end
    if options.task=="analysis" then request.scenario.budgetSeconds=60 end
    request.compareMode=options.compareMode or "individual"
	assert(request.compareMode=="individual" or request.compareMode=="topgear"or request.compareMode=="loadout","Choose a comparison mode.")
	local selectedLimit=request.compareMode=="loadout"and 18 or 8
    assert(type(options.selected or {})=="table" and #(options.selected or {})<=selectedLimit,"Choose fewer items.")
    for _,value in ipairs(options.selected or {}) do
        assert(type(value)=="table","Refresh the item list, then select your items.")
        local item=value.identity or value
        assert(type(item)=="table" and type(item.source)=="string" and type(item.slot)=="string" and type(item.value)=="string","Refresh the item list, then select your items.")
        local copy={source=item.source,slot=item.slot,value=item.value}
        for _,key in ipairs({"key","link"}) do
            if item[key]~=nil then assert(type(item[key])=="string","Invalid item identity.");copy[key]=item[key] end
        end
		if value.placement~=nil then
			assert(request.compareMode=="loadout"and type(value.placement)=="string"and profileSlots[value.placement],"Invalid planned item placement.")
			copy.placement=value.placement
		end
		if value.plannedLevel~=nil then
			assert(request.compareMode=="loadout"and type(value.plannedLevel)=="number"and value.plannedLevel%1==0 and value.plannedLevel>0 and value.plannedLevel<=2000,"Invalid planned item level.")
			copy.plannedLevel=value.plannedLevel
		end
        if value.options~=nil then
            assert(request.compareMode=="topgear"or request.compareMode=="loadout","Choose a gear combination for item alternatives.")
            copy.options=itemOptions(value.options)
        end
        request.selected[#request.selected+1]=copy
    end
    assert(type(options.talents or {})=="table" and #(options.talents or {})<=16,"Choose at most sixteen talent builds.")
    local seenTalents={}
    for i,value in ipairs(options.talents or {}) do
        if type(value)=="string" then value={label="Build "..i,code=value} end
        assert(type(value)=="table" and type(value.code)=="string" and #value.code>0 and #value.code<=2048 and value.code:match("^[%w+/=]+$"),"Paste a talent export code.")
        local label=value.label or "Build "..i
        assert(type(label)=="string" and #label<=200 and not label:find("[%z\r\n]"),"Use a shorter talent name.")
        assert(not seenTalents[value.code],"Select each talent build once.");seenTalents[value.code]=true
        request.talents[#request.talents+1]={label=label,code=value.code}
    end
	if options.task=="compare"then
		if request.compareMode=="loadout"then assert(#request.selected>0 or #request.talents==1,"Change at least one item or talent build.")
		else assert(#request.selected>0,"Select at least one item.")end
	end
    if options.task=="talents" then assert(#request.talents>0,"Select at least one talent build.") end
    assert(options.task=="compare"or(options.task=="analysis"and request.compareMode=="topgear")or #request.selected==0,"Use gear comparison for selected items.")
	assert(options.task=="talents"or options.task=="analysis"or(options.task=="compare"and request.compareMode=="loadout"and #request.talents<=1)or #request.talents==0,"Use talent comparison for selected builds.")
    if options.task=="cancel" or options.task=="plan" then
        assert(type(options.jobID)=="string" and #options.jobID<=96 and options.jobID:match("^[%w_-]+$"),"Choose a saved run.")
        request.jobID=options.jobID
    end
    if options.task=="plan" then
        assert(type(options.planOptions)=="table","Enter planning assumptions.")
        -- Preserve explicit native history/weights. The engine validates every
        -- field and checks the selected job's character before using them.
        local function copy(value,depth)
            assert(depth<5,"Invalid planning options.")
            if type(value)~="table" then assert(type(value)=="string" or type(value)=="number" or type(value)=="boolean","Invalid planning option.");return value end
            local out={};if #value>0 then setmetatable(out,array) end
            for key,item in pairs(value) do out[key]=copy(item,depth+1) end
            return out
        end
        request.planOptions=copy(options.planOptions,1)
        if request.planOptions.exclude then setmetatable(request.planOptions.exclude,array) end
    end
    if options.workflow~=nil then
        local draft=options.workflow
        assert(type(draft)=="table"and(draft.context=="raid"or draft.context=="m+")and type(draft.expectedRevision)=="number"and draft.expectedRevision>=0 and draft.expectedRevision%1==0,"Refresh the saved workflow before saving.")
        local mode=draft.compareMode or"individual"
        local task=mode=="loadout"and"compare"or #(draft.selected or{})>0 and"compare"or"baseline"
        local checked=gameOptions({task=task,scenario=draft.scenario,compareMode=mode,selected=draft.selected,talents=mode=="loadout"and draft.talents or nil})
        request.workflow={context=draft.context,expectedRevision=draft.expectedRevision,scenario=checked.scenario,compareMode=mode,selected=checked.selected,talents=checked.talents}
    end
    return request
end
function S:RunGameRequest(options)
    if not self:RequireEnabled() then return false,self.status end
    if self.scan or self.packets or self.resumeToken or self.reloadAttemptToken or self:IsReloadPending() then return false,"A request is already active. Cancel it before starting another." end
    local ok,request=pcall(gameOptions,options)
    if not ok then self:SetStatus(tostring(request));return false,tostring(request) end
    if InCombatLockdown() then self:SetStatus("Finish combat before starting a calculation.");return false,self.status end
    if request.task=="upgrades" then
        if request.scenario.keyLevel==nil then
            local key=self.lootKey or 10
            request.scenario.keyLevel=key==0 and 0 or key<=3 and 1 or key==4 and 2 or key==5 and 3 or key==6 and 4 or key==7 and 5 or key<=9 and 6 or 7
        end
        if request.scenario.raidDifficulty==nil then request.scenario.raidDifficulty=({[17]=0,[14]=1,[15]=2,[16]=3})[self.lootRaid or 15] end
    end
    self.gameStatusCallback=type(options.onStatus)=="function" and options.onStatus or nil
    local token={};self.gameRequestToken=token
    local function send()
        if S.gameRequestToken~=token then return false,"Capture cancelled." end
        local success,result=pcall(function()
            local snapshot=S:Snapshot()
            local view=S:GameSnapshot()
            local available={}
            for _,group in ipairs({view.candidates or{},view.equipped or{}})do for _,candidate in ipairs(group)do
                local key=identityKey(candidate.identity or candidate);available[key]=(available[key] or 0)+1
            end end
            for _,item in ipairs(request.selected) do
                local key=identityKey(item)
                assert((available[key] or 0)>0,"A selected item changed or disappeared. Refresh the list and select it again.")
                available[key]=available[key]-1
            end
            request.id=snapshot.id;snapshot.gameRequest=request
            local data=json(snapshot)
            assert(#data<=131072,"This request exceeds the transfer limit. Select fewer items or builds.")
            S:Transmit(snapshot,data,true,options.autoReload==true)
            LUI:DB().simPendingRequestID=request.id
            S:SetStatus("Sending to Lamda Engine. Keep WoW in front.")
            return true
        end)
        if not success then S:Stop(tostring(result));return false,tostring(result) end
        return true
    end
    if request.task=="upgrades" and select(4,GetBuildInfo())~=16001 then
        local success,collecting=pcall(self.CollectLoot,self,send)
        if not success then self:CancelLoot();self.gameRequestToken=nil;self:SetStatus(tostring(collecting));return false,tostring(collecting) end
        if collecting then return true end
    end
    return send()
end
function S:Stop(message)
    self:ClearEngineReload()
    self:CancelLoot()
    self.gameRequestToken=nil
    self.packets=nil;self.started=nil
    self.autoPackets=nil;self.transferRetries=nil;self.retryTransfer=nil
    self.autoReloadRequested=nil;self.reloadAt=nil;self.reloadExpires=nil;self.reloadCharacter=nil;self.waitingForCombat=nil
    self.pollSession=nil;self.reloadCountdownStarted=nil;self.resumeToken=nil;self.reloadManual=nil;self.reloadManualReason=nil;self.reloadAttemptToken=nil
    LUI:DB().simReloadResume=nil
    if self.reloadWatch then self.reloadWatch:Hide() end
    if self.strip then self.strip:Hide() end
    if self.GameUI then self.GameUI.requestAt=nil end
    self:SetStatus(message or "Transfer stopped.")
end
function S:IsReloadPending()
    return self.autoReloadRequested==true or self.reloadAt~=nil
end
function S:CancelAutoReload()
    self:Stop("Automatic checks stopped. Any calculation already received can keep running.")
end
-- One bounded diagnostic survives logout; no character profile or binding data.
function S:ReloadDiagnostic(stage,reason)
    local db=LUI:DB()
    local id=self.pollSession and self.pollSession.requestID or db.simPendingRequestID or ""
    local function short(value,limit)return tostring(value or ""):gsub("[%c]"," "):sub(1,limit)end
    id=short(id,96);stage=short(stage,32);reason=short(reason,180)
    local prior=db.simReloadDiagnostic
    if type(prior)=="table" and prior.requestID==id and prior.stage==stage and prior.reason==reason then return end
    db.simReloadDiagnostic={requestID=id,stage=stage,reason=reason,at=GetServerTime()}
end
local function reloadIdentity()
    local guid=plain(UnitGUID("player"));assert(type(guid)=="string" and guid~="","Character unavailable")
    local api=C_SpecializationInfo
    local getSpec=api and api.GetSpecialization or GetSpecialization
    local getInfo=api and api.GetSpecializationInfo or GetSpecializationInfo
    local index=getSpec and plain(getSpec())
    return guid,index and getInfo and (plain(getInfo(index)) or 0) or 0
end
function S:ReloadStatus()
    if not self:IsReloadPending() then return end
    if self.reloadManual then return self.reloadManualReason or "Click Check for results to continue." end
    if self.packets then
        return "Sending data · pass "..math.min(SEND_PASSES,(self.sendPasses or 0)+1).."/"..SEND_PASSES.." · packet "..tostring(self.rendered or 1).."/"..#self.packets..". Keep WoW in front."
    end
    if InCombatLockdown() or self.waitingForCombat then return "Analysis paused while you are in combat." end
    return self.pollSession and self.pollSession.checks>0 and "Finishing your comparisons…" or "Analysis started…"
end
-- This is transfer/countdown progress, never a percentage of worker completion.
function S:TransferProgress()
    local now=GetTime()
    if self.reloadManual then return {phase="ready",fraction=0,label=self.reloadManualReason or "Click Check for results to continue."} end
    if self.scan then
        local count=math.max(1,self.scan.taskCount or 1)
        local done=math.max(0,math.min(count,(self.scan.index or 1)-1))
        return {phase="capture",fraction=done/count,label="Reading loot · "..done.."/"..count}
    end
    if self.autoReloadRequested and self.packets then
        local count=#self.packets
        local dwell=self.packetSince and math.max(0,math.min(1,(now-self.packetSince)/SEND_INTERVAL)) or 0
        local done=(self.sendPasses or 0)*count+math.max(0,(self.rendered or 1)-1)+dwell
        local fraction=math.max(0,math.min(1,done/(SEND_PASSES*count)))
        return {phase="sending",fraction=fraction,label="Sending data · "..math.floor(fraction*100).."%"}
    end
    if self.reloadAt then
        if InCombatLockdown() or self.waitingForCombat then
            return {phase="paused",fraction=0,label="Analysis paused · waiting for combat to end"}
        end
        return {phase="waiting",fraction=0,label="Analysis in progress"}
    end
end
function S:ClearEngineReload()
    self.engineReloadRequest=nil;self.reloadInputPacket=nil
    if self.strip and not self.packets then self.strip:Hide() end
    if self.engineReloadButton and self.reloadKeyBound then
        if InCombatLockdown() then self.reloadBindingClearPending=true
        else
            local ok=pcall(ClearOverrideBindings,self.engineReloadButton)
            if ok then self.reloadKeyBound=nil;self.reloadBindingClearPending=nil end
        end
    end
end
function S:HoldForManualReload(message)
    self:ClearEngineReload()
    self.reloadAt=nil;self.reloadExpires=nil;self.reloadManual=true
    self.reloadManualReason=message or "Click Check for results to continue."
    if self.reloadWatch then self.reloadWatch:Hide() end
    self:SetStatus(self.reloadManualReason)
end
local function typingOrModified()
    if type(GetCurrentKeyBoardFocus)~="function" then return true,"Keyboard-focus API is unavailable." end
    if GetCurrentKeyBoardFocus()~=nil then return true,"Waiting for typing to finish." end
    if (IsShiftKeyDown and IsShiftKeyDown()) or (IsControlKeyDown and IsControlKeyDown()) or (IsAltKeyDown and IsAltKeyDown()) then
        return true,"Waiting for modifier keys to be released."
    end
end
function S:EngineReloadClick()
    local request=self.engineReloadRequest
    self:ReloadDiagnostic("click-received","Reload binding callback received.")
    local function rejected(reason)
        if request then request.rejectedReason=reason end
        S:ReloadDiagnostic("click-rejected",reason)
    end
    if not request then rejected("No active reload request.");return end
    if InCombatLockdown() then rejected("Reload callback arrived during combat.");return end
    local blocked,reason=typingOrModified()
    if blocked then rejected(reason);return end
    local ok,guid,spec=pcall(reloadIdentity)
    if not ok or not self.pollSession or self.pollSession.requestID~=request.id or LUI:DB().simPendingRequestID~=request.id
        or guid~=request.guid or spec~=request.spec then rejected("Reload request or character changed.");return end
    if GetTime()>request.untilTime or GetServerTime()>=request.expires then rejected("Reload request expired.");return end
    if GetBindingAction(ENGINE_RELOAD_BINDING,true)~="CLICK LamdaSimEngineReload:LeftButton" then rejected("Reload binding changed.");return end
    -- Called by a real binding event, never by a timer or a macro string.
    self:ReloadResults()
end
function S:RequestEngineReload()
    if self.engineReloadRequest then
        if GetTime()>self.engineReloadRequest.untilTime then
            local rejected=self.engineReloadRequest.rejectedReason
            if rejected then
                self:ReloadDiagnostic("click-rejected",rejected)
                self:HoldForManualReload("Automatic result check was rejected: "..rejected.." Click Check for results to continue.")
            elseif self.pollSession and self.pollSession.checks==0 and (self.transferRetries or 0)<1 and self:RetryAutoTransfer() then
                self:ReloadDiagnostic("transfer-retry","No reload input arrived; resending the original request once.")
            else
                self:ReloadDiagnostic("engine-timeout","No reload input arrived from the engine.")
                self:HoldForManualReload("The automatic result check did not start. Click Check for results to continue.")
            end
        end
        return
    end
    local poll=self.pollSession;local character=self.reloadCharacter
    if not poll or not character or type(GetBindingAction)~="function" or type(SetOverrideBindingClick)~="function"
        or type(ClearOverrideBindings)~="function" or type(GetCurrentKeyBoardFocus)~="function" then
        self:ReloadDiagnostic("unavailable","Reload request or required binding API is unavailable.")
        self:HoldForManualReload("Automatic result checks are unavailable. Click Check for results to continue.");return
    end
    local blocked,reason=typingOrModified()
    if blocked then self:ReloadDiagnostic("waiting-input",reason);return end
    local binding=GetBindingAction(ENGINE_RELOAD_BINDING,true)
    if binding and binding~="" then
        self:ReloadDiagnostic("binding-conflict","Ctrl+Shift+F12 is already bound.")
        self:HoldForManualReload("The automatic result-check key is already bound. Click Check for results to continue.");return
    end
    if not self.engineReloadButton then
        local button=CreateFrame("Button","LamdaSimEngineReload",UIParent);self.engineReloadButton=button
        button:SetSize(1,1);button:SetPoint("TOPLEFT",UIParent,"BOTTOMLEFT",-10,-10);button:EnableMouse(false)
        button:RegisterForClicks("AnyUp");button:SetScript("OnClick",function()S:EngineReloadClick()end)
    end
    local ok,problem=pcall(SetOverrideBindingClick,self.engineReloadButton,true,ENGINE_RELOAD_BINDING,"LamdaSimEngineReload","LeftButton")
    self.reloadKeyBound=ok
    if not ok or GetBindingAction(ENGINE_RELOAD_BINDING,true)~="CLICK LamdaSimEngineReload:LeftButton" then
        self:ReloadDiagnostic("binding-rejected",ok and "WoW did not accept the reload key." or "Binding API error: "..tostring(problem))
        self:HoldForManualReload("WoW did not accept the automatic result check. Click Check for results to continue.");return
    end
    local now=GetServerTime()
    self.engineReloadSequence=(self.engineReloadSequence or 0)+1
    local nonce=poll.requestID.."_"..poll.checks.."_"..math.floor(GetTime()*1000).."_"..self.engineReloadSequence
    local client=select(4,GetBuildInfo())==16001 and "forever" or "retail"
    local body=table.concat({"2",client,poll.requestID,character.guid,character.spec,nonce,now,now+15},"|")
    self.engineReloadRequest={id=poll.requestID,guid=character.guid,spec=character.spec,expires=now+15,untilTime=GetTime()+15}
    self.reloadInputPacket=assert(LUI.LookupEvents:Packet(body,"LUR2"))
    self:InitializeStrip()
    self.bridge:ApplyPosition()
    self:RenderPacket(self.reloadInputPacket);self.strip:Show()
    self:ReloadDiagnostic("lease-emitted","Reload input request displayed for the engine.")
end
function S:ReloadResults()
    if InCombatLockdown() or self.reloadAttemptToken then return false end
    local ok,guid,spec=pcall(reloadIdentity)
    local poll=self:IsReloadPending() and self.pollSession or nil
    if poll and (not ok or not self.reloadCharacter or guid~=self.reloadCharacter.guid or spec~=self.reloadCharacter.spec
        or poll.requestID~=LUI:DB().simPendingRequestID) then poll=nil end
    self:Stop("Reloading results...")
    local db=LUI:DB();db.simReloadResume=nil
    if ok then
        if poll then poll.checks=poll.checks+1 end
        db.simReloadResume={guid=guid,spec=spec,requestID=db.simPendingRequestID or "",tab=db.simCurrentTab or "overview",at=GetServerTime(),poll=poll}
    end
    local marker=db.simReloadResume;local attempt={};self.reloadAttemptToken=attempt
    self:ReloadDiagnostic("reload-requested","ReloadUI called.")
    local success,problem=pcall(ReloadUI)
    local function refused()
        if self.reloadAttemptToken~=attempt then return end
        self.reloadAttemptToken=nil
        if db.simReloadResume==marker then db.simReloadResume=nil end
        local same,currentGUID,currentSpec=pcall(reloadIdentity)
        if InCombatLockdown() or not same or currentGUID~=guid or currentSpec~=spec
            or (marker and marker.requestID~=(db.simPendingRequestID or "")) then
            self:Stop("Character changed. Reload cancelled.");return
        end
        self:ReloadDiagnostic("reload-refused",success and "ReloadUI returned without reloading." or "ReloadUI error: "..tostring(problem))
        if poll then
            poll.checks=math.max(0,poll.checks-1);self.pollSession=poll;self.autoReloadRequested=true
            self.reloadCharacter={guid=guid,spec=spec};self:HoldForManualReload("The automatic result check did not start. Click Check for results to continue.")
        else self:SetStatus("The automatic result check did not start. Click Check for results.") end
    end
    if not success then refused()
    else
        -- A protected call can return without throwing or reloading. Only a
        -- successful reload destroys this timer; never treat pcall as delivery.
        C_Timer.After(2,refused)
    end
    return success
end
function S:CheckAutoReload()
    if not self.reloadAt then return end
    local now=GetTime()
    if now>self.reloadExpires then self:Stop("The automatic result check expired. Click Check for results when ready.");return end
    if self.pollSession and GetServerTime()>=self.pollSession.deadline then self:Stop("Automatic checks stopped after an hour. Check Lamda Engine, then click Check for results.");return end
    local ok,guid,spec=pcall(reloadIdentity)
    if not ok or not self.reloadCharacter or guid~=self.reloadCharacter.guid or spec~=self.reloadCharacter.spec
        or (self.pollSession and self.pollSession.requestID~=LUI:DB().simPendingRequestID) then
        self:Stop("Character changed. Automatic reload cancelled.");return
    end
    if InCombatLockdown() then self:ClearEngineReload();self.waitingForCombat=true;return end
    if self.waitingForCombat then self.waitingForCombat=nil;self.reloadAt=math.max(self.reloadAt,now+POLL_INTERVAL);self.reloadCountdownStarted=now end
    if now>=self.reloadAt then self:RequestEngineReload() end
end
function S:ScheduleReload(at)
    local now=GetTime()
    self.reloadAt=at;self.reloadCountdownStarted=now
    self.reloadExpires=self.reloadAt+300
    if not self.reloadWatch then
        self.reloadWatch=CreateFrame("Frame",nil,UIParent)
        local elapsed=0
        self.reloadWatch:SetScript("OnUpdate",function(_,delta)
            elapsed=elapsed+delta
            if elapsed>=0.2 then
                elapsed=0
                local ok,problem=pcall(S.CheckAutoReload,S)
                if not ok then
                    S:ReloadDiagnostic("check-error",tostring(problem))
                    S:Stop("The automatic result check failed. Click Check for results; details were saved.")
                end
            end
        end)
    end
    self.reloadWatch:Show();self:SetStatus(self:ReloadStatus())
end
function S:FinishAutoTransfer()
    local at=self.retryTransfer and GetTime()+RELOAD_BUFFER or math.max(GetTime()+RELOAD_BUFFER,self.started+RELOAD_MINIMUM)
    self.retryTransfer=nil
    self.packets=nil;self.started=nil;self.strip:Hide()
    self:ScheduleReload(at)
end
function S:RetryAutoTransfer()
    if InCombatLockdown() or not self.autoPackets or #self.autoPackets==0 or not self.pollSession or not self.reloadCharacter then return false end
    self:ClearEngineReload();self.reloadAt=nil;self.reloadExpires=nil;self.reloadManual=nil;self.reloadManualReason=nil
    self.packets=self.autoPackets;self.started=GetTime();self.rendered=nil;self.sendPasses=0;self.packetSince=nil
    self.transferRetries=(self.transferRetries or 0)+1;self.retryTransfer=true
    self:Tick();self.strip:Show()
    self:SetStatus("The first transfer was missed. Sending the same request again…")
    return true
end
local function pollingJob(resume)
    local envelope=LamdaSimResults
    local client=select(4,GetBuildInfo())==16001 and "forever" or "retail"
    if type(envelope)~="table" or envelope.schema~=1 or envelope.client~=client then return end
    for _,job in ipairs(envelope.jobs or {}) do
        if job.requestID==resume.requestID then
            for _,summary in ipairs(envelope.snapshots or {}) do
                local snapshot=summary.snapshot;local character=snapshot and snapshot.character
                if snapshot and character and snapshot.id==job.snapshotId and snapshot.client==client
                    and character.guid==resume.guid and character.spec==resume.spec then
                    local poll=resume.poll
                    if not poll then return job end
                    local request=snapshot.gameRequest
                    if job.snapshotId==resume.requestID and job.gameTask==poll.task and request
                        and request.id==resume.requestID and request.task==poll.task then return job end
                end
            end
        end
    end
end
local function validPoll(poll,resume)
    local now=GetServerTime()
    return type(poll)=="table" and poll.requestID==resume.requestID
        and ({baseline=true,compare=true,talents=true,upgrades=true,analysis=true,plan=true,cancel=true,healing=true,builds=true})[poll.task]
        and type(poll.startedAt)=="number" and poll.startedAt<=now
        and type(poll.deadline)=="number" and poll.deadline>poll.startedAt and poll.deadline<=poll.startedAt+POLL_LIMIT
        and type(poll.checks)=="number" and poll.checks>=1 and poll.checks<=POLL_LIMIT/POLL_INTERVAL+1 and poll.checks%1==0
        and type(poll.missingChecks)=="number" and poll.missingChecks>=0 and poll.missingChecks<MISSING_LIMIT and poll.missingChecks%1==0
end
function S:ResumeAfterReload(isReloading)
    if not LUI:DB().simEnabled then return end
    self.reloadAttemptToken=nil
    local db=LUI:DB();local resume=db.simReloadResume;db.simReloadResume=nil
    if isReloading~=true or type(resume)~="table" or type(resume.at)~="number" or GetServerTime()-resume.at<0 or GetServerTime()-resume.at>180 then return end
    local ok,guid,spec=pcall(reloadIdentity)
    if not ok or resume.guid~=guid or resume.spec~=spec or resume.requestID~=(db.simPendingRequestID or "") then return end
    db.simCurrentTab=resume.tab
    local token={};self.resumeToken=token
    C_Timer.After(0.5,function()
        if S.resumeToken~=token then return end
        S.resumeToken=nil
        local same,currentGUID,currentSpec=pcall(reloadIdentity)
        if not same or currentGUID~=guid or currentSpec~=spec or resume.requestID~=db.simPendingRequestID then return end
        local status="No result loaded for this request. Check Lamda Engine before sending again."
        local job=pollingJob(resume);local poll=validPoll(resume.poll,resume) and resume.poll or nil
        local pending=not job or job.state=="queued" or job.state=="running" or job.state=="cancelling"
        if job then
            if job.state=="complete" then
                local task=poll and poll.task or job.gameTask
                local complete=task=="cancel" or (task=="plan" and type(job.plan)=="table" and (not poll or job.plan.jobID==poll.jobID))
                    or (task~="plan" and task~="cancel" and type(job.result)=="table")
                status=complete and (task=="cancel" and "Cancellation received." or "Result loaded.") or "The engine report is incomplete. Automatic checks stopped."
                if complete and task=="analysis" then db.simCurrentTab="summary" end
                if complete and task=="healing" then db.simCurrentTab="healing" end
                if complete and task=="builds" then db.simCurrentTab="meta" end
            elseif job.state=="failed" or job.state=="cancelled" or job.state=="interrupted" then
                status=job.error or ("Calculation "..job.state..".")
            elseif pending then status="Still calculating. Results will open when ready."
            else status="Unknown engine status. Automatic checks stopped." end
        end
        if poll and pending then
            poll.missingChecks=job and 0 or poll.missingChecks+1
            if GetServerTime()>=poll.deadline then
                status="Automatic checks stopped after an hour. Check Lamda Engine, then click Check for results."
            elseif poll.missingChecks>=MISSING_LIMIT then
                status="No response from Lamda Engine. Automatic checks stopped. Check the engine before sending again."
            else
                S.pollSession=poll;S.autoReloadRequested=true;S.reloadCharacter={guid=guid,spec=spec}
                S:ScheduleReload(GetTime()+POLL_INTERVAL)
                status=S:ReloadStatus()
            end
        end
        S:SetStatus(status)
        if not InCombatLockdown() and S.OpenGameUI then pcall(S.OpenGameUI,S) end
    end)
end
function S:InitializeStrip()
    if self.strip then return end
    self.bridge=LUI.LookupEvents:GetStrip("sim");self.strip=self.bridge.frame;self.cells=self.bridge.cells
    local strip=self.strip
    strip:SetScript("OnUpdate",function()
        local ok=pcall(S.Tick,S)
        if not ok then S:Stop("Transfer stopped after a display error. Try again after reloading.") end
    end)
end
function S:Tick()
    if LUI.overlayHidden then self:Stop("Color strips hidden. Show them before sending again.");return end
    if self.bridge and self.bridge.moving then return end
    if not self.packets then return end
    local elapsed=GetTime()-self.started
    if elapsed>=175 then self:Stop("Transfer ended. Open the simulator to resend.");return end
    local index
    if self.autoReloadRequested then
        -- Count actual displayed packets with a full dwell. A delayed frame
        -- advances once; elapsed wall time cannot invent unsent whole passes.
        if self.rendered then
            if GetTime()-self.packetSince<SEND_INTERVAL then return end
            index=self.rendered%#self.packets+1
            if index==1 then
                self.sendPasses=self.sendPasses+1
                if self.sendPasses>=SEND_PASSES then self:FinishAutoTransfer();return end
            end
        else index=1 end
        self.packetSince=GetTime()
    else index=math.floor(elapsed/SEND_INTERVAL)%#self.packets+1 end
    if index==self.rendered then return end
    self:RenderPacket(self.packets[index])
    self.rendered=index
end
function S:RenderPacket(packet)
    for i,cell in ipairs(self.cells) do
        local offset=(i-1)*3
        cell:SetColorTexture(packet:byte(offset+1)/255,packet:byte(offset+2)/255,packet:byte(offset+3)/255,1)
    end
end
function S:Start(skipLoot)
    if not self:RequireEnabled() then return false end
    self:Stop()
    if not skipLoot and select(4,GetBuildInfo())~=16001 and C_EncounterJournal and C_EncounterJournal.GetLootInfoByIndex then
        local ok,collecting=pcall(self.CollectLoot,self,function() S:Start(true) end)
        if not ok then self:CancelLoot();self:SetStatus(tostring(collecting));return false end
        if collecting then return true end
    end
    local ok,snapshot,data=pcall(self.Snapshot,self)
    if not ok then self:SetStatus(tostring(snapshot));return false end
    return self:Transmit(snapshot,data)
end
function S:Transmit(snapshot,data,inGame,autoReload)
    if not self:RequireEnabled() then return false end
    if LUI.overlayHidden then self:SetStatus("Show color strips before sending.");return false end
    self:InitializeStrip();self.bridge:SetMoving(false)
    assert(PixelUtil and LUI.LookupEvents,"The color bridge is unavailable.")
    -- Fixed physical dimensions; the module owns its saved origin.
    self.bridge:ApplyPosition()
    local packets={};local count=math.ceil(#data/SIM_CHUNK);local crc=LUI.LookupEvents:Checksum(data)
    for index=1,count do
        local header=table.concat({"2",snapshot.id,snapshot.client,snapshot.capturedAt,#data,index,count,string.format("%.0f",crc)},"|")
        packets[index]=assert(LUI.LookupEvents:Packet(header.."\n"..data:sub((index-1)*SIM_CHUNK+1,index*SIM_CHUNK),"LUS2"))
    end
    self.snapshotData=data;self.packets=packets;self.started=GetTime();self.rendered=nil
    self.sendPasses=0;self.packetSince=nil
    self.autoReloadRequested=inGame==true and autoReload==true
    self.autoPackets=self.autoReloadRequested and packets or nil;self.transferRetries=0;self.retryTransfer=nil
    self.reloadCharacter=self.autoReloadRequested and {guid=snapshot.character.guid,spec=snapshot.character.spec} or nil
    self.pollSession=self.autoReloadRequested and {requestID=snapshot.id,task=snapshot.gameRequest.task,jobID=snapshot.gameRequest.jobID,
        startedAt=GetServerTime(),deadline=GetServerTime()+POLL_LIMIT,checks=0,missingChecks=0} or nil
    self:Tick();self.strip:Show()
    self:SetStatus(inGame and "Sending to Lamda Engine. Keep WoW in front." or "Sending... Keep WoW in front until the browser opens, then stop transfer.")
    return true
end
local module={id="sim",name="LamdaSim",enabledKey="simEnabled",settingsScope="global",reloadKeys={"simEnabled","simBridgeX","simBridgeY"},limits={simBridgeX={0,16000,integer=true},simBridgeY={0,16000,integer=true}},defaults={simBridgeX=288,simBridgeY=224,simEnabled=false,simCurrentTab="character",simPendingRequestID=""}}
module.build=function(parent)
    LUI:Button(parent,"Open simulator",4,0,180,function()
        if S.OpenGameUI then
            if LUI.frame then LUI.frame:Hide()end
            S:OpenGameUI();return
        end
        local ok=pcall(S.RunGameRequest,S,{task="baseline"})
        if not ok then S:Stop("Unable to start the transfer. Check the color bridge and reload.") end
    end)
    S.label=LUI:Description(parent,"Encounter setup and all results are in the simulator workspace.",4,-48,570)
end
LUI:RegisterModule(module)
function S:RefreshEnabled()
    if LUI:DB().simEnabled then return end
    if self.packets or self.scan or self.resumeToken or self.reloadAttemptToken or self:IsReloadPending() then self:Stop("LamdaSim disabled.")end
    if self.GameUI and self.GameUI.frame then self.GameUI.frame:Hide()end
end
local events=CreateFrame("Frame")
for _,event in ipairs({"PLAYER_REGEN_DISABLED","PLAYER_REGEN_ENABLED","PLAYER_ENTERING_WORLD","PLAYER_SPECIALIZATION_CHANGED","UI_SCALE_CHANGED","DISPLAY_SIZE_CHANGED","PLAYER_LOGOUT","UPDATE_BINDINGS"}) do events:RegisterEvent(event) end
events:SetScript("OnEvent",function(_,event,arg1,arg2)
    -- A module disabled during combat must still release its temporary binding.
    if event=="PLAYER_REGEN_ENABLED" and S.reloadBindingClearPending then S:ClearEngineReload() end
    if not LUI:DB().simEnabled then S:RefreshEnabled();return end
    if event=="UPDATE_BINDINGS" and not S.engineReloadRequest then return end
    if event=="PLAYER_REGEN_ENABLED" then
        if S.reloadAt then S.waitingForCombat=true;S:CheckAutoReload() end
        return
    end
    if event=="PLAYER_SPECIALIZATION_CHANGED" and arg1~="player" then return end
    if S.reloadAttemptToken and event~="PLAYER_LOGOUT" then S:Stop("Character or display changed. Reload cancelled.");return end
    if event=="PLAYER_REGEN_DISABLED" and S.reloadAt then S:ClearEngineReload();S.waitingForCombat=true;return end
    if S.packets or S.scan or S:IsReloadPending() then S:Stop("Character or display state changed. Transfer and automatic reload cancelled.") end
    if event=="PLAYER_ENTERING_WORLD" then S:ResumeAfterReload(arg2) end
end)
SLASH_LAMDASIM1="/lsim"
SlashCmdList.LAMDASIM=function()
    if S.OpenGameUI then return S:OpenGameUI() end
    LUI:OpenUI();LUI:SelectPage("modules");LUI:SelectModule("sim")
end
