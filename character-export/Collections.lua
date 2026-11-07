local _,E=...
local call,array=E.call,E.array

E.register("mounts",function()
    local rows,seen=array(),{}
    local ids=call("C_MountJournal.GetMountIDs")
    local available=type(ids)=="table"
    for _,id in pairs(ids or {}) do
        local name,spell,_,active,usable,source,favorite,_,faction,_,owned=call("C_MountJournal.GetMountInfoByID",id)
        if owned==true then
            rows[#rows+1]={mountID=id,name=E.text(name),spellID=spell,active=active,usable=usable,sourceType=source,favorite=favorite,faction=faction}
            if spell then seen[spell]=true end
        end
    end
    local count=call("GetNumCompanions","MOUNT")
    if count~=nil then available=true end
    for index=1,E.count(count,10000) do
        local creature,name,spell,_,active,mountType=call("GetCompanionInfo","MOUNT",index)
        if name and (not spell or not seen[spell]) then
            rows[#rows+1]={name=E.text(name),spellID=spell,creatureID=creature,active=active,mountType=mountType,source="companion API"}
            if spell then seen[spell]=true end
        end
    end
    E.sort(rows,"name")
    return E.section(available and "complete" or "unavailable",{items=rows,count=#rows,scope="collected mounts; item-based mounts still in bags are also under bags"})
end)

local function toyFilterState()
    local state={sources={},expansions={}}
    local booleanMethods={"CollectedShown","UncollectedShown","UnusableShown"}
    for _,key in ipairs(booleanMethods) do
        if not E.has("C_ToyBox.Get"..key) or not E.has("C_ToyBox.Set"..key) then return nil end
        state[key]=call("C_ToyBox.Get"..key)
        if type(state[key])~="boolean" then return nil end
    end
    if E.has("C_ToyBox.GetFilterString") then state.search=call("C_ToyBox.GetFilterString")
    elseif ToyBox and not E.secret(ToyBox) then state.search=E.clean(ToyBox.searchString or "") end
    if type(state.search)~="string" or not E.has("C_ToyBox.SetFilterString") then return nil end
    local sourceCount=call("C_PetJournal.GetNumPetSources")
    local expansionCount=call("GetNumExpansions")
    if not sourceCount or not expansionCount then return nil end
    for _,path in ipairs({"C_ToyBox.IsSourceTypeFilterChecked","C_ToyBox.SetSourceTypeFilter","C_ToyBox.IsExpansionTypeFilterChecked","C_ToyBox.SetExpansionTypeFilter"}) do
        if not E.has(path) then return nil end
    end
    for id=1,E.count(sourceCount,100) do
        local valid=call("C_ToyBoxInfo.IsToySourceValid",id)
        if valid~=false then
            local checked=call("C_ToyBox.IsSourceTypeFilterChecked",id)
            if type(checked)~="boolean" then return nil end
            state.sources[id]=checked
        end
    end
    for id=1,E.count(expansionCount,100) do
        local checked=call("C_ToyBox.IsExpansionTypeFilterChecked",id)
        if type(checked)~="boolean" then return nil end
        state.expansions[id]=checked
    end
    return state
end
local function setToyFilters(state,restore)
    if restore then
        call("C_ToyBox.SetCollectedShown",state.CollectedShown)
        call("C_ToyBox.SetUncollectedShown",state.UncollectedShown)
        call("C_ToyBox.SetUnusableShown",state.UnusableShown)
    else
        call("C_ToyBox.SetCollectedShown",true)
        call("C_ToyBox.SetUncollectedShown",false)
        call("C_ToyBox.SetUnusableShown",true)
    end
    for id,checked in pairs(state.sources) do
        if restore then call("C_ToyBox.SetSourceTypeFilter",id,checked) else call("C_ToyBox.SetSourceTypeFilter",id,true) end
    end
    for id,checked in pairs(state.expansions) do
        if restore then call("C_ToyBox.SetExpansionTypeFilter",id,checked) else call("C_ToyBox.SetExpansionTypeFilter",id,true) end
    end
    call("C_ToyBox.SetFilterString",restore and state.search or "")
end
E.register("toys",function(snapshot)
    if not E.has("C_ToyBox.GetToyFromIndex") then
        return E.unavailable("This client has no toy-collection enumeration API. Owned toy items and other usable items in bags/bank are exported there, including their use spell when available.")
    end
    local rows,seen=array(),{}
    local function add(id)
        if type(id)~="number" or seen[id] then return end
        seen[id]=true
        local owned=call(E.has("PlayerHasToy") and "PlayerHasToy" or "C_ToyBox.PlayerHasToy",id)
        if owned==true then
            local toy,name=call("C_ToyBox.GetToyInfo",id)
            rows[#rows+1]={itemID=toy or id,name=E.text(name)}
        end
    end
    local state=toyFilterState()
    local full=state~=nil and call("InCombatLockdown")~=true
    local ok=pcall(function()
        if full then setToyFilters(state,false) end
        local count=call("C_ToyBox.GetNumFilteredToys") or call("C_ToyBox.GetNumToys")
        for index=1,E.count(count,20000) do add(call("C_ToyBox.GetToyFromIndex",index)) end
    end)
    if full then setToyFilters(state,true) end
    for _,id in pairs(E.db and E.db.toyIDs or {}) do add(id) end
    -- Direct ownership checks also find bag toys when the journal is filtered.
    for _,bag in ipairs(snapshot.bags and snapshot.bags.containers or {}) do
        for _,item in ipairs(bag.items or {}) do add(item.itemID) end
    end
    E.sort(rows,"itemID")
    if E.db then E.db.toyIDs={};for _,row in ipairs(rows) do E.db.toyIDs[#E.db.toyIDs+1]=row.itemID end end
    local expected=call("C_ToyBox.GetNumLearnedDisplayedToys")
    local complete=ok and (full or (expected~=nil and #rows==expected))
    return E.section(complete and "complete" or "partial",{items=rows,count=#rows,reportedOwnedCount=expected,filtersRestored=full,
        scope=full and "unfiltered owned collection" or "visible collection plus previously observed and bag-owned toys",
        note=not complete and "The client did not expose enough filter state for a complete, safely reversible collection read." or nil})
end)

E.register("companions",function()
    local rows,seen=array(),{}
    local count=call("GetNumCompanions","CRITTER")
    local available=count~=nil
    for index=1,E.count(count,10000) do
        local creature,name,spell,_,active=call("GetCompanionInfo","CRITTER",index)
        if name then
            rows[#rows+1]={name=E.text(name),spellID=spell,creatureID=creature,active=active,source="companion API"}
            if spell then seen[spell]=true end
        end
    end
    local total,ownedTotal=call("C_PetJournal.GetNumPets")
    local journalCount=0
    if total~=nil then available=true end
    for index=1,E.count(total,30000) do
        local petID,species,owned,customName,level,favorite,revoked,name,_,petType,creature,source,description,wild,canBattle=call("C_PetJournal.GetPetInfoByIndex",index)
        if owned and petID then
            rows[#rows+1]={petID=petID,speciesID=species,name=E.text(name),customName=E.text(customName),level=level,favorite=favorite,revoked=revoked,petType=petType,creatureID=creature,sourceText=E.text(source),canBattle=canBattle}
            journalCount=journalCount+1
        end
    end
    E.sort(rows,"name")
    local partial=ownedTotal~=nil and journalCount<ownedTotal
    return E.section(not available and "unavailable" or (partial and "partial" or "complete"),{items=rows,count=#rows,reportedOwnedBattlePets=ownedTotal,
        note=partial and "Battle-pet journal filters hide some owned pets; collection preferences were left unchanged." or nil})
end)
