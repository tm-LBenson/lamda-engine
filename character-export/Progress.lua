local _, E = ...
local call,array=E.call,E.array

-- Some classic lists expose only expanded rows. Expand within this synchronous
-- read, then restore precisely the headers we changed, including on failure.
-- No saved list preferences or selected quest/faction are deliberately changed.
function E.ReadExpandedList(read,expand,collapse)
    local changed,blocked={},{}
    local function annotate(rows)
        local counts={}
        for _,row in ipairs(rows) do
            if row.isHeader then
                local name=tostring(row.name or row.title or "")
                counts[name]=(counts[name] or 0)+1
                row._headerKey=name.."#"..counts[name]
            end
        end
        return rows
    end
    local rows,complete={},true
    local ok=pcall(function()
        for pass=1,128 do
            rows=annotate(read())
            local target
            for _,row in ipairs(rows) do
                if row.isHeader and row.isCollapsed and not blocked[row._headerKey] then target=row;break end
            end
            if not target then return end
            if not expand or not collapse or call("InCombatLockdown")==true then complete=false;return end
            local key=target._headerKey
            changed[key]=true
            E.invoke(expand,target._index)
            local stillCollapsed=false
            for _,row in ipairs(annotate(read())) do if row._headerKey==key and row.isCollapsed then stillCollapsed=true end end
            if stillCollapsed then blocked[key]=true;changed[key]=nil;complete=false end
            if pass==128 then complete=false end
        end
    end)
    if not ok then complete=false end
    local restoreOK,current=pcall(function()return annotate(read())end)
    if restoreOK then
        for index=#current,1,-1 do
            local row=current[index]
            if row.isHeader and changed[row._headerKey] then E.invoke(collapse,row._index) end
        end
    elseif next(changed) then E.note("A game list refresh prevented complete header restoration.") end
    for _,row in ipairs(rows) do row._headerKey=nil;row._index=nil end
    return rows,complete and ok
end

local function questRows()
    local rows={}
    local count=call("C_QuestLog.GetNumQuestLogEntries") or call("GetNumQuestLogEntries")
    for index=1,E.count(count,2000) do
        local info=call("C_QuestLog.GetInfo",index)
        if not info then
            local title,level,group,header,collapsed,complete,frequency,id=call("GetQuestLogTitle",index)
            if title then info={title=title,level=level,suggestedGroup=group,isHeader=header,isCollapsed=collapsed,isComplete=complete,frequency=frequency,questID=id} end
        end
        if info then info._index=index;info.logIndex=index;rows[#rows+1]=info end
    end
    return rows
end
E.register("quests",function()
    if not E.has("C_QuestLog.GetNumQuestLogEntries") and not E.has("GetNumQuestLogEntries") then return E.unavailable("Quest log APIs are unavailable.") end
    local expand=E.resolve("C_QuestLog.ExpandQuestHeader") or E.resolve("ExpandQuestHeader")
    local collapse=E.resolve("C_QuestLog.CollapseQuestHeader") or E.resolve("CollapseQuestHeader")
    local rows,complete=E.ReadExpandedList(questRows,expand,collapse)
    local active,header=array(),nil
    for _,info in ipairs(rows) do
        if info.isHeader then header=info.title
        else
            local row=E.pick(info,{"questID","title","level","suggestedGroup","frequency","isComplete","isTask","isBounty","isStory","isScaling","isHidden","questType"})
            row.title,row.header=E.text(row.title),E.text(header)
            local objectives=info.questID and call("C_QuestLog.GetQuestObjectives",info.questID)
            row.objectives=array()
            for _,objective in pairs(objectives or {}) do
                if type(objective)=="table" then
                    local value=E.pick(objective,{"type","finished","numFulfilled","numRequired","objectiveType","questObjectiveID"})
                    value.text=E.text(objective.text)
                    row.objectives[#row.objectives+1]=value
                end
            end
            if not objectives and info.logIndex then
                for objective=1,E.count(call("GetNumQuestLeaderBoards",info.logIndex),100) do
                    local text,kind,finished=call("GetQuestLogLeaderBoard",objective,info.logIndex)
                    if text then row.objectives[#row.objectives+1]={text=E.text(text),type=kind,finished=finished} end
                end
            end
            if info.questID then
                row.readyForTurnIn=call("C_QuestLog.IsComplete",info.questID)
                row.failed=call("C_QuestLog.IsFailed",info.questID)
                row.description=E.text(call("C_QuestLog.GetQuestDescription",info.questID))
            end
            active[#active+1]=row
        end
    end
    local completed=call("C_QuestLog.GetAllCompletedQuestIDs")
    local completedStatus="complete"
    if not completed and E.has("GetQuestsCompleted") then
        local raw={};call("GetQuestsCompleted",raw);raw=E.clean(raw)
        completed={}
        for id,done in pairs(raw or {}) do if done and type(id)=="number" then completed[#completed+1]=id end end
        completedStatus=E.completedReady and "complete" or "cached"
    end
    local ids=array()
    for _,id in pairs(completed or {}) do if type(id)=="number" then ids[#ids+1]=id end end
    table.sort(ids)
    local _,total=call("C_QuestLog.GetNumQuestLogEntries")
    if total and total>#active then complete=false end
    return E.section(complete and "complete" or "partial",{active=active,activeCount=#active,completedQuestIDs=ids,
        completedCount=#ids,completedStatus=completed and completedStatus or "unavailable",dailyResetSeconds=call("GetQuestResetTime"),
        note=not complete and "Some quest headers could not be expanded safely; this is not proof that no other quests exist." or nil})
end)

local function factionRows()
    local rows={}
    local count=call("C_Reputation.GetNumFactions") or call("GetNumFactions")
    for index=1,E.count(count,4000) do
        local info=call("C_Reputation.GetFactionDataByIndex",index)
        if not info then
            local name,description,reaction,minimum,maximum,value,atWar,_,header,collapsed,hasRep,watched,child,id=call("GetFactionInfo",index)
            if name then info={name=name,description=description,reaction=reaction,currentReactionThreshold=minimum,nextReactionThreshold=maximum,currentStanding=value,atWarWith=atWar,isHeader=header,isCollapsed=collapsed,isHeaderWithRep=hasRep,isWatched=watched,isChild=child,factionID=id} end
        end
        if info then info._index=index;rows[#rows+1]=info end
    end
    return rows
end
E.register("reputations",function()
    if not E.has("C_Reputation.GetNumFactions") and not E.has("GetNumFactions") then return E.unavailable("Reputation APIs are unavailable.") end
    local rows,complete=E.ReadExpandedList(factionRows,E.resolve("C_Reputation.ExpandFactionHeader") or E.resolve("ExpandFactionHeader"),E.resolve("C_Reputation.CollapseFactionHeader") or E.resolve("CollapseFactionHeader"))
    local result=array()
    for _,info in ipairs(rows) do
        if not info.isHeader or info.isHeaderWithRep then
            local value=E.pick(info,{"factionID","name","reaction","currentStanding","currentReactionThreshold","nextReactionThreshold","atWarWith","isAccountWide","isWatched"})
            value.name=E.text(value.name);result[#result+1]=value
        end
    end
    return E.section(complete and "complete" or "partial",{items=result})
end)

local function currencyRows()
    local rows={}
    local count=call("C_CurrencyInfo.GetCurrencyListSize") or call("GetCurrencyListSize")
    for index=1,E.count(count,4000) do
        local info=call("C_CurrencyInfo.GetCurrencyListInfo",index)
        if not info then
            local name,header,expanded,unused,watched,quantity,_,maximum,weekly,earned,id=call("GetCurrencyListInfo",index)
            if name then info={name=name,isHeader=header,isHeaderExpanded=expanded,isTypeUnused=unused,isShowInBackpack=watched,quantity=quantity,maxQuantity=maximum,canEarnPerWeek=weekly,quantityEarnedThisWeek=earned,currencyID=id} end
        end
        if info then info._index=index;info.isCollapsed=info.isHeader and not info.isHeaderExpanded;rows[#rows+1]=info end
    end
    return rows
end
E.register("currencies",function()
    if not E.has("C_CurrencyInfo.GetCurrencyListSize") and not E.has("GetCurrencyListSize") then return E.unavailable("This client has no currency-list API; money is under character.moneyCopper.") end
    local toggle=E.resolve("C_CurrencyInfo.ExpandCurrencyList") or E.resolve("ExpandCurrencyList")
    local rows,complete=E.ReadExpandedList(currencyRows,toggle and function(i)toggle(i,true)end,toggle and function(i)toggle(i,false)end)
    local result=array()
    for _,info in ipairs(rows) do
        if not info.isHeader then
            local value=E.pick(info,{"name","quantity","maxQuantity","quantityEarnedThisWeek","maxWeeklyQuantity","totalEarned","isAccountWide","isAccountTransferable","currencyID","currencyTypesID"})
            value.name=E.text(value.name);result[#result+1]=value
        end
    end
    return E.section(complete and "complete" or "partial",{items=result})
end)

E.register("lockouts",function()
    local count=call("GetNumSavedInstances")
    if count==nil then return E.unavailable("Instance lockout APIs are unavailable.") end
    local rows=array()
    for index=1,E.count(count,500) do
        local name,id,reset,difficulty,locked,extended,_,raid,maxPlayers,difficultyName,total,defeated=call("GetSavedInstanceInfo",index)
        if name then
            local row={name=E.text(name),lockoutID=id,resetSeconds=reset,difficultyID=difficulty,locked=locked,extended=extended,isRaid=raid,maxPlayers=maxPlayers,difficulty=E.text(difficultyName),bossesTotal=total,bossesDefeated=defeated,encounters=array()}
            for boss=1,E.count(total,100) do
                local bossName,_,killed=call("GetSavedInstanceEncounterInfo",index,boss)
                if bossName then row.encounters[#row.encounters+1]={name=E.text(bossName),killed=killed} end
            end
            rows[#rows+1]=row
        end
    end
    return E.section(E.raidInfoReady and "complete" or "cached",{items=rows,capturedAt=E.now()})
end)

function E.CaptureRecipes()
    if not E.db or not E.tradeOpen then return end
    local info=call("C_TradeSkillUI.GetBaseProfessionInfo")
    local ids=call("C_TradeSkillUI.GetAllRecipeIDs")
    local recipes=array()
    if type(info)=="table" and (info.professionID or info.skillLineID) and ids then
        for _,id in pairs(ids) do
            local recipe=call("C_TradeSkillUI.GetRecipeInfo",id)
            if recipe and recipe.learned then
                local row=E.pick(recipe,{"recipeID","name","relativeDifficulty","numAvailable","skillLineAbilityID","learned","canSkillUp"})
                row.recipeID=row.recipeID or id
                recipes[#recipes+1]=row
            end
        end
        E.sort(recipes,"recipeID")
        local key=tostring(info.skillLineID or info.professionID)
        E.db.recipes=E.db.recipes or {}
        E.db.recipes[key]={capturedAt=E.now(),profession=info,items=recipes,coverage="all learned recipes reported by this profession"}
        return
    end
    local name,rank,maximum=call("GetTradeSkillLine")
    if name and name~="" and name~=(UNKNOWN or "Unknown") and name~=(TRADE_SKILLS or "Trade Skills") then
        for index=1,E.count(call("GetNumTradeSkills"),5000) do
            local title,kind,available=call("GetTradeSkillInfo",index)
            if title and kind~="header" then
                local link=call("GetTradeSkillRecipeLink",index)
                recipes[#recipes+1]={name=E.text(title),difficulty=kind,availableCrafts=available,recipeID=type(link)=="string" and tonumber(link:match("enchant:(%d+)")) or nil}
            end
        end
        E.db.recipes=E.db.recipes or {}
        E.db.recipes[name]={capturedAt=E.now(),profession={name=E.text(name),rank=rank,maxRank=maximum},items=recipes,coverage="visible recipes at the last profession-window visit; filters can hide others"}
    end
end
E.register("recipes",function(snapshot)
    E.CaptureRecipes()
    local result=array()
    local learned={}
    for _,p in ipairs(snapshot.professions and snapshot.professions.items or {}) do
        if p.skillID then learned[tostring(p.skillID)]=true end
        for _,line in ipairs(p.skillLines or {}) do
            if line.skillID then learned[tostring(line.skillID)]=true end
            if line.name then learned[line.name]=true end
        end
        if p.name then learned[p.name]=true end
    end
    for key,value in pairs(E.db and E.db.recipes or {}) do
        local profession=value.profession or {}
        if learned[key] or learned[profession.name or profession.professionName] or not snapshot.professions or snapshot.professions.status~="complete" then
            local row=E.clean(value);array(row.items);result[#result+1]=row
        end
    end
    return E.section(#result>0 and "cached" or "unavailable",{professions=result,note="Recipe lists are captured when their profession window is opened. Other profession levels are still exported without opening them."})
end)

E.register("group",function()
    local raid=call("IsInRaid")
    local count=call("GetNumGroupMembers") or 0
    local rows=array()
    for index=1,E.count(count,40) do
        local unit=raid and "raid"..index or (index==count and "player" or "party"..index)
        local name,realm=call("UnitName",unit)
        local className,class=call("UnitClass",unit)
        if name then rows[#rows+1]={name=name,realm=realm,class=class,className=className,role=call("UnitGroupRolesAssigned",unit),level=call("UnitLevel",unit)} end
    end
    return E.section("complete",{isRaid=raid or false,count=count,items=rows})
end)
