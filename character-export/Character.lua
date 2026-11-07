local _, E = ...
local call, array = E.call, E.array

E.register("character", function()
    local name, realm = call("UnitFullName", "player")
    if not name then name, realm = call("UnitName", "player") end
    local className, class, classID = call("UnitClass", "player")
    local raceName, race, raceID = call("UnitRace", "player")
    local faction, factionName = call("UnitFactionGroup", "player")
    local guild, guildRank, guildRankIndex = call("GetGuildInfo", "player")
    local specIndex = call("GetSpecialization")
    local specID, specName, _, _, specRole
    if specIndex then specID, specName, _, _, specRole = call("GetSpecializationInfo", specIndex) end
    return E.section(name and "complete" or "partial", {
        name=name, realm=realm or call("GetRealmName"), class=class, className=className, classID=classID,
        race=race, raceName=raceName, raceID=raceID, faction=faction, factionName=factionName,
        level=call("UnitLevel", "player"), sex=call("UnitSex", "player"),
        specialization=specName, specializationID=specID, role=specRole,
        xp=call("UnitXP", "player"), xpMax=call("UnitXPMax", "player"), restedXP=call("GetXPExhaustion"),
        moneyCopper=call("GetMoney"), bindLocation=call("GetBindLocation"),
        guild=guild, guildRank=guildRank, guildRankIndex=guildRankIndex,
        dead=call("UnitIsDeadOrGhost", "player"), mounted=call("IsMounted"),
        inCombat=call("InCombatLockdown"), pvp=call("UnitIsPVP", "player"),
        honorLevel=call("UnitHonorLevel", "player"), pvpRank=call("UnitPVPRank", "player"),
        hasNewMail=call("HasNewMail"), locale=call("GetLocale"), region=call("GetCurrentRegion"),
    })
end)

E.register("location", function()
    local mapID = call("C_Map.GetBestMapForUnit", "player")
    local mapInfo = mapID and call("C_Map.GetMapInfo", mapID)
    local x, y
    if mapID and E.has("C_Map.GetPlayerMapPosition") then
        x, y = E.invoke(function()
            local point = C_Map.GetPlayerMapPosition(mapID, "player")
            if point and not E.secret(point) then return point:GetXY() end
        end)
    end
    local instance, instanceType, difficultyID, difficultyName, maxPlayers, _, _, instanceID = call("GetInstanceInfo")
    local inside = call("IsInInstance")
    return E.section(mapID and "complete" or "partial", {
        zone=call("GetRealZoneText") or call("GetZoneText"), subzone=call("GetSubZoneText"),
        minimapZone=call("GetMinimapZoneText"), mapID=mapID, mapName=mapInfo and mapInfo.name,
        xPercent=x and math.floor(x*10000+.5)/100, yPercent=y and math.floor(y*10000+.5)/100,
        coordinateStatus=x and y and "available" or "unavailable",
        facingRadians=call("GetPlayerFacing"), inInstance=inside,
        instance=instance, instanceType=instanceType, instanceID=instanceID,
        difficultyID=difficultyID, difficulty=difficultyName, maxPlayers=maxPlayers,
        hearthLocation=call("GetBindLocation"), warMode=call("C_PvP.IsWarModeDesired"),
    })
end)

E.register("stats", function()
    local result = E.section("complete", {primary={}, resistances=array(), powers=array()})
    for index, label in ipairs({"strength", "agility", "stamina", "intellect", "spirit"}) do
        local base, effective, positive, negative = call("UnitStat", "player", index)
        if base or effective then result.primary[label] = {base=base, effective=effective, positive=positive, negative=negative} end
    end
    result.health, result.maxHealth = call("UnitHealth", "player"), call("UnitHealthMax", "player")
    local primaryPower, powerToken = call("UnitPowerType", "player")
    result.primaryPower, result.primaryPowerToken = primaryPower, powerToken
    local powerIDs, used = {primaryPower or 0, 0}, {}
    for _, id in ipairs(powerIDs) do
        if not used[id] then
            used[id] = true
            local maximum = call("UnitPowerMax", "player", id)
            if maximum then result.powers[#result.powers+1] = {type=id, current=call("UnitPower", "player", id), maximum=maximum} end
        end
    end
    local baseArmor, effectiveArmor, armor, positiveArmor, negativeArmor = call("UnitArmor", "player")
    result.armor = {base=baseArmor, effective=effectiveArmor, raw=armor, positive=positiveArmor, negative=negativeArmor}
    local ap, positiveAP, negativeAP = call("UnitAttackPower", "player")
    local rangedAP, positiveRanged, negativeRanged = call("UnitRangedAttackPower", "player")
    result.attackPower = {base=ap, positive=positiveAP, negative=negativeAP}
    result.rangedAttackPower = {base=rangedAP, positive=positiveRanged, negative=negativeRanged}
    result.critPercent, result.rangedCritPercent = call("GetCritChance"), call("GetRangedCritChance")
    result.hastePercent, result.spellHastePercent = call("GetHaste"), call("UnitSpellHaste", "player")
    result.masteryPercent, result.dodgePercent, result.parryPercent = call("GetMasteryEffect"), call("GetDodgeChance"), call("GetParryChance")
    result.blockPercent, result.healingBonus = call("GetBlockChance"), call("GetSpellBonusHealing")
    result.manaRegen, result.castingManaRegen = call("GetManaRegen")
    result.mainHandSpeed, result.offHandSpeed = call("UnitAttackSpeed", "player")
    local low, high, offLow, offHigh, positive, negative, multiplier = call("UnitDamage", "player")
    result.weaponDamage = {minimum=low, maximum=high, offHandMinimum=offLow, offHandMaximum=offHigh, positive=positive, negative=negative, multiplier=multiplier}
    local schoolNames={"Holy","Fire","Nature","Frost","Shadow","Arcane"}
    for school=2,7 do
        local resistanceIndex=school-1
        local base, real, effective, bonus = call("UnitResistance", "player", resistanceIndex)
        result.resistances[#result.resistances+1] = {school=school, schoolName=schoolNames[resistanceIndex],resistanceIndex=resistanceIndex,
            base=base, real=real, effective=effective, bonus=bonus,
            spellDamage=call("GetSpellBonusDamage", school), spellCritPercent=call("GetSpellCritChance", school)}
    end
    result.ratings = array()
    local seen = {}
    for _, name in ipairs({"CR_DEFENSE_SKILL", "CR_DODGE", "CR_PARRY", "CR_BLOCK", "CR_HIT_MELEE", "CR_HIT_RANGED", "CR_HIT_SPELL", "CR_CRIT_MELEE", "CR_CRIT_RANGED", "CR_CRIT_SPELL", "CR_HASTE_MELEE", "CR_HASTE_RANGED", "CR_HASTE_SPELL", "CR_EXPERTISE", "CR_MASTERY", "CR_VERSATILITY_DAMAGE_DONE", "CR_RESILIENCE_CRIT_TAKEN"}) do
        local id = _G[name]
        if type(id) == "number" and not seen[id] then
            seen[id] = true
            local value = call("GetCombatRating", id)
            if value then result.ratings[#result.ratings+1] = {type=name, id=id, value=value, bonus=call("GetCombatRatingBonus", id)} end
        end
    end
    if not result.maxHealth then result.status="partial" end
    return result
end)

local professionIDs = {129,164,165,171,182,185,186,197,202,333,356,393,755,773,794}
local knownProfessions = {}; for _, id in ipairs(professionIDs) do knownProfessions[id]=true end
E.register("professions", function()
    local rows, skills, seen, candidates = array(), array(), {}, {}
    local flat,byID=array(),{}
    local fields={"name","skillID","rank","maxRank","modifier","minLevel","parentSkillLineID"}
    local function add(row)
        if not row or not row.name or row.isHeader then return end
        local key = row.skillID or row.name
        row.name = E.text(row.name)
        if seen[key] then
            for field,value in pairs(row) do seen[key][field]=value end
            return
        end
        seen[key] = row
        flat[#flat+1] = row
        if row.skillID then byID[row.skillID]=row end
    end
    if E.has("GetProfessions") and E.has("GetProfessionInfo") then
        local indices={call("GetProfessions")}
        for _, index in pairs(indices) do
            local name, _, rank, maximum, _, _, skillID, modifier = call("GetProfessionInfo", index)
            if name then add({name=name, rank=rank, maxRank=maximum, skillID=skillID, modifier=modifier}) end
            if skillID then candidates[skillID]=true end
        end
    end
    for _, id in ipairs(professionIDs) do candidates[id]=true end
    local tradeLines = call("C_TradeSkillUI.GetAllProfessionTradeSkillLines")
    for _, id in pairs(tradeLines or {}) do if type(id)=="number" then candidates[id]=true end end
    local collapsed = false
    local count = call("C_SkillInfo.GetNumSkillLines") or call("GetNumSkillLines")
    for index=1,E.count(count,2000) do
        local info=call("C_SkillInfo.GetSkillLineInfo",index)
        if not info then
            local name, header, expanded, rank, temporary, modifier, maximum = call("GetSkillLineInfo",index)
            if name then info={name=name,isHeader=header,isCollapsed=header and not expanded,rank=rank,tempPoints=temporary,modifier=modifier,maxRank=maximum} end
        end
        if info then
            if info.isHeader and info.isCollapsed then collapsed=true end
            if not info.isHeader then
                skills[#skills+1]=E.pick(info,{"name","skillID","rank","maxRank","modifier","tempPoints","skillLineCategoryID","parentSkillLineID","minLevel"})
                if knownProfessions[info.skillID] or info.skillLineCategoryID==9 or info.skillLineCategoryID==11 then add(E.pick(info,fields)) end
            end
        end
    end
    for id in pairs(candidates) do
        local info=call("C_SkillInfo.GetSkillLineInfoByID",id)
        if info and info.name and info.maxRank and info.maxRank>0 then
            info.skillID=info.skillID or id
            add(E.pick(info,fields))
        end
    end
    local function rootID(row)
        local original=row.skillID or row.name
        local visited={}
        for _=1,10 do
            local id=row.skillID or row.name
            if visited[id] then return original end
            visited[id]=true
            local parent=row.parentSkillLineID
            if type(parent)~="number" or parent<=0 or parent==id then return id end
            if not byID[parent] then
                local info=call("C_SkillInfo.GetSkillLineInfoByID",parent)
                if info then info.skillID=info.skillID or parent;add(E.pick(info,fields)) end
            end
            if not byID[parent] then return parent end
            row=byID[parent]
        end
        return original
    end
    -- Group parent/child skill lines while preserving every expansion's rank.
    -- Names alone cannot distinguish Retail profession tiers reliably.
    local roots={}
    for index=1,#flat do roots[flat[index]]=rootID(flat[index]) end
    local grouped={}
    for _,row in ipairs(flat) do
        local root=roots[row] or rootID(row)
        local group=grouped[root]
        if not group then
            group=E.pick(byID[root] or row,fields)
            if type(root)=="number" then group.skillID=root end
            group.skillLines=array()
            grouped[root]=group;rows[#rows+1]=group
        end
        group.skillLines[#group.skillLines+1]=E.pick(row,fields)
    end
    for _,row in ipairs(rows) do
        if #row.skillLines>1 then E.sort(row.skillLines,"skillID") else row.skillLines=nil end
    end
    E.sort(rows,"name")
    local status=(E.has("GetProfessions") or E.has("C_SkillInfo.GetSkillLineInfoByID")) and "complete" or (count and "partial" or "unavailable")
    return E.section(status,{items=rows,otherSkills=skills,otherSkillsStatus=collapsed and "partial" or "complete",
        note=collapsed and "Collapsed skill headers may hide non-profession skill rows; direct profession-ID reads are included." or nil})
end)

E.register("talents",function()
    local config=call("C_ClassTalents.GetActiveConfigID")
    if type(config)=="number" and config>0 then
        local info=call("C_Traits.GetConfigInfo",config)
        local rows,seen=array(),{}
        for _,tree in pairs(info and info.treeIDs or {}) do
            for _,id in pairs(call("C_Traits.GetTreeNodes",tree) or {}) do
                if type(id)=="number" and not seen[id] then
                    seen[id]=true
                    local node=call("C_Traits.GetNodeInfo",config,id)
                    if node and node.subTreeActive~=false and (node.activeRank or 0)>0 then
                        local entryID=node.activeEntry and node.activeEntry.entryID
                        local entry=entryID and call("C_Traits.GetEntryInfo",config,entryID)
                        local definition=entry and entry.definitionID and call("C_Traits.GetDefinitionInfo",entry.definitionID)
                        local spellID=definition and definition.spellID
                        local spell=spellID and call("C_Spell.GetSpellInfo",spellID)
                        rows[#rows+1]={nodeID=id,entryID=entryID,spellID=spellID,name=spell and spell.name,rank=node.activeRank,subTreeID=entry and entry.subTreeID}
                    end
                end
            end
        end
        return E.section(info and "complete" or "partial",{system="traits",configID=config,items=rows,
            hasStagedChanges=call("C_Traits.ConfigHasStagedChanges",config),pvpTalents=call("C_SpecializationInfo.GetAllSelectedPvpTalentIDs")})
    end
    local tabs=call("GetNumTalentTabs")
    if tabs==nil then return E.unavailable("The client did not expose a talent API for this character.") end
    local rows=array()
    for tab=1,E.count(tabs,10) do
        local tabName=call("GetTalentTabInfo",tab,false,false)
        for index=1,E.count(call("GetNumTalents",tab),200) do
            local name,_,tier,column,rank,maximum = call("GetTalentInfo",tab,index,false,false)
            if name then rows[#rows+1]={name=E.text(name),tree=tabName,treeIndex=tab,index=index,tier=tier,column=column,rank=rank,maxRank=maximum} end
        end
    end
    return E.section("complete",{system="legacy",items=rows,unspentPoints=call("UnitCharacterPoints","player"),activeGroup=call("GetActiveTalentGroup")})
end)

local function spellName(id)
    local info=call("C_Spell.GetSpellInfo",id)
    return info and info.name or call("GetSpellInfo",id)
end
E.spellName=spellName
function E.ScanSpellBank(pet)
    local rows,seen=array(),{}
    local bank=Enum and Enum.SpellBookSpellBank and (pet and Enum.SpellBookSpellBank.Pet or Enum.SpellBookSpellBank.Player)
    local legacy=pet and (BOOKTYPE_PET or "pet") or (BOOKTYPE_SPELL or "spell")
    local function add(id,known,name,passive)
        if type(id)~="number" or id<=0 or seen[id] or known==false then return end
        seen[id]=true
        rows[#rows+1]={spellID=id,name=E.text(name or spellName(id)),known=known,passive=passive}
    end
    local function entry(index)
        local info=bank and call("C_SpellBook.GetSpellBookItemInfo",index,bank)
        if info then
            local known=call("C_SpellBook.IsSpellKnown",info.spellID or 0,bank)
            if info.spellID and not info.isOffSpec and not info.isUnlearned then add(info.spellID,known,info.name,info.isPassive) end
            if info.actionID and Enum and Enum.SpellBookItemType and info.itemType==Enum.SpellBookItemType.Flyout then
                local _,_,slots=call("GetFlyoutInfo",info.actionID)
                for slot=1,E.count(slots,100) do local id,override,learned,name=call("GetFlyoutSlotInfo",info.actionID,slot);if learned then add(override or id,true,name) end end
            end
        else
            local kind,id=call("GetSpellBookItemInfo",index,legacy)
            if kind=="SPELL" or kind=="spell" then add(id,true,call("GetSpellBookItemName",index,legacy)) end
            if kind=="FLYOUT" or kind=="flyout" then
                local _,_,slots=call("GetFlyoutInfo",id)
                for slot=1,E.count(slots,100) do local spell,override,learned,name=call("GetFlyoutSlotInfo",id,slot);if learned then add(override or spell,true,name) end end
            end
        end
    end
    local available=false
    if pet then
        local count=call("C_SpellBook.HasPetSpells") or call("HasPetSpells")
        available=count~=nil
        for index=1,E.count(count,1000) do entry(index) end
    else
        local count=call("C_SpellBook.GetNumSpellBookSkillLines")
        if count~=nil then
            available=true
            for line=1,E.count(count,100) do
                local info=call("C_SpellBook.GetSpellBookSkillLineInfo",line)
                if info and (not info.offSpecID or info.offSpecID==0) then
                    for index=(info.itemIndexOffset or 0)+1,(info.itemIndexOffset or 0)+E.count(info.numSpellBookItems,2000) do entry(index) end
                end
            end
        else
            count=call("GetNumSpellTabs");available=count~=nil
            for line=1,E.count(count,100) do
                local _,_,offset,numSlots,_,offSpec=call("GetSpellTabInfo",line)
                if offset and (not offSpec or offSpec==0) then for index=offset+1,offset+E.count(numSlots,2000) do entry(index) end end
            end
        end
    end
    E.sort(rows,"spellID")
    return E.section(available and "complete" or "unavailable",{items=rows,count=#rows,bank=pet and "pet" or "player"})
end
E.register("spells",function() return E.ScanSpellBank(false) end)

E.register("pet",function()
    local exists=call("UnitExists","pet")
    if exists~=true then return E.section(exists==false and "complete" or "unavailable",{exists=exists}) end
    local name=call("UnitName","pet")
    return E.section("complete",{exists=true,name=name,family=call("UnitCreatureFamily","pet"),
        creatureType=call("UnitCreatureType","pet"),specialization=call("C_PetInfo.GetPetTalentTree"),
        level=call("UnitLevel","pet"),dead=call("UnitIsDeadOrGhost","pet"),
        health=call("UnitHealth","pet"),maxHealth=call("UnitHealthMax","pet"),
        happiness=call("GetPetHappiness"),spells=E.ScanSpellBank(true)})
end)

E.register("actions",function()
    if not E.has("GetActionInfo") then return E.unavailable("Action bar APIs are unavailable.") end
    local rows=array()
    for slot=1,math.max(180,math.min(MAX_ACTION_BUTTONS or 180,240)) do
        local kind,id,subtype=call("GetActionInfo",slot)
        if kind then
            local row={slot=slot,type=kind,id=id,subtype=subtype}
            if kind=="spell" and id then row.name=spellName(id) end
            if kind=="macro" and id then
                row.name=call("GetMacroInfo",id)
                row.spellID=call("GetMacroSpell",id)
                local item=call("GetMacroItem",id)
                row.item=item and E.text(item)
                -- Macro bodies can contain private chat text. Export their
                -- names and resolved actions, not arbitrary command contents.
            end
            rows[#rows+1]=row
        end
    end
    return E.section("complete",{items=rows})
end)

E.register("keybindings",function()
    local count=call("GetNumBindings")
    if not count then return E.unavailable("Keybinding APIs are unavailable.") end
    local rows=array()
    for index=1,E.count(count,3000) do
        local command,category,key1,key2=call("GetBinding",index)
        if command and (key1 or key2) then rows[#rows+1]={command=command,category=category,key1=key1,key2=key2} end
    end
    return E.section("complete",{items=rows,macroBodiesIncluded=false})
end)

E.register("addons",function()
    local modern=E.has("C_AddOns.GetNumAddOns")
    local prefix=modern and "C_AddOns." or ""
    local count=call(prefix.."GetNumAddOns")
    if not count then return E.unavailable("Addon metadata APIs are unavailable.") end
    local rows=array()
    for index=1,E.count(count,1000) do
        local name,title=call(prefix.."GetAddOnInfo",index)
        if name then rows[#rows+1]={name=name,title=E.text(title),version=call(prefix.."GetAddOnMetadata",name,"Version"),loaded=call(prefix.."IsAddOnLoaded",name)} end
    end
    E.sort(rows,"name")
    return E.section("complete",{items=rows})
end)

E.register("buffs",function()
    local modern=E.has("C_UnitAuras.GetAuraDataByIndex")
    if not modern and not E.has("UnitBuff") then return E.unavailable("Aura enumeration is unavailable; exported stats are still the actual current character stats.") end
    local rows=array()
    for index=1,255 do
        local info
        if modern then info=call("C_UnitAuras.GetAuraDataByIndex","player",index,"HELPFUL")
        else
            local name,_,applications,dispel,duration,expires,source,_,_,spellID=call("UnitBuff","player",index)
            if name then info={name=name,applications=applications,dispelName=dispel,duration=duration,expirationTime=expires,sourceUnit=source,spellId=spellID} end
        end
        if not info then break end
        local row=E.pick(info,{"name","spellId","applications","duration","expirationTime","sourceUnit","isFromPlayerOrPlayerPet"})
        row.name=E.text(row.name);rows[#rows+1]=row
    end
    return E.section("complete",{items=rows,note="Current buffs help explain temporary stat increases."})
end)
