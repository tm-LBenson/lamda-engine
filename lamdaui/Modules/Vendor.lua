-- LamdaUI's merchant queue. Inventory acknowledgements advance it, not a fixed
-- number of attempted sales. No item deletion or confirmation-popup automation.
local _, LUI = ...
local V = {status="Visit a merchant to sell junk and repair."}
LUI.Vendor = V
local function public(v)
    if issecretvalue and issecretvalue(v) then return nil end
    return v
end
local function number(v)
    v=public(v)
    if type(v)=="number" and v==v and v~=math.huge and v~=-math.huge then return v end
end
local function call(fn,...)
    if type(fn)~="function" then return nil end
    local result={pcall(fn,...)}
    if result[1] then return unpack(result,2,20) end
end
local function money(n)
    return string.format("%dg %ds %dc",math.floor(n/10000),math.floor(n/100)%100,n%100)
end
function V:SetStatus(text)
    self.status=text
    if self.label then self.label:SetText(text) end
end
local function bags()
    local result={0,1,2,3,4}
    local reagent=Enum and Enum.BagIndex and number(Enum.BagIndex.ReagentBag)
    if reagent and reagent>4 and reagent<=5 then result[#result+1]=reagent end
    return result
end
local function protectedItem(db,id)
    for token in db.vendorKeepItems:gmatch("%d+") do
        if tonumber(token)==id then return true end
    end
    return false
end
local tracks={"Explorer","Adventurer","Veteran","Champion","Hero","Myth"}
local qualities={"Common","Uncommon","Rare","Epic"}
local gearSlots={INVTYPE_HEAD=true,INVTYPE_NECK=true,INVTYPE_SHOULDER=true,INVTYPE_CHEST=true,
    INVTYPE_ROBE=true,INVTYPE_WAIST=true,INVTYPE_LEGS=true,INVTYPE_FEET=true,INVTYPE_WRIST=true,
    INVTYPE_HAND=true,INVTYPE_FINGER=true,INVTYPE_TRINKET=true,INVTYPE_CLOAK=true,INVTYPE_WEAPON=true,
    INVTYPE_SHIELD=true,INVTYPE_2HWEAPON=true,INVTYPE_WEAPONMAINHAND=true,INVTYPE_WEAPONOFFHAND=true,
    INVTYPE_HOLDABLE=true,INVTYPE_RANGED=true,INVTYPE_RANGEDRIGHT=true,INVTYPE_THROWN=true,INVTYPE_RELIC=true}
local function boolean(value)
    value=public(value);if type(value)=="boolean" then return value end
end
local function text(value)
    value=public(value);if type(value)=="string" then return value end
end
-- Bag items have equipment types, not a chosen destination in a paired slot.
-- Keep rings/trinkets together and never guess the hand for flexible weapons.
local slots={
    {"head","Head",{"INVTYPE_HEAD"}}, {"neck","Neck",{"INVTYPE_NECK"}},
    {"shoulder","Shoulders",{"INVTYPE_SHOULDER"}}, {"back","Back",{"INVTYPE_CLOAK"}},
    {"chest","Chest",{"INVTYPE_CHEST","INVTYPE_ROBE"}}, {"wrist","Wrists",{"INVTYPE_WRIST"}},
    {"hands","Hands",{"INVTYPE_HAND"}}, {"waist","Waist",{"INVTYPE_WAIST"}},
    {"legs","Legs",{"INVTYPE_LEGS"}}, {"feet","Feet",{"INVTYPE_FEET"}},
    {"rings","Rings (both slots)",{"INVTYPE_FINGER"}}, {"trinkets","Trinkets (both slots)",{"INVTYPE_TRINKET"}},
    {"onehand","One-hand weapons",{"INVTYPE_WEAPON"}}, {"mainhand","Main-hand only",{"INVTYPE_WEAPONMAINHAND"}},
    {"twohand","Two-hand weapons",{"INVTYPE_2HWEAPON"}},
    {"offhand","Off-hand / shields",{"INVTYPE_WEAPONOFFHAND","INVTYPE_HOLDABLE","INVTYPE_SHIELD"}},
    {"ranged","Ranged / relics",{"INVTYPE_RANGED","INVTYPE_RANGEDRIGHT","INVTYPE_THROWN","INVTYPE_RELIC"}},
}
local slotsByEquip,slotsByID={},{}
for _,slot in ipairs(slots) do
    slotsByID[slot[1]]=slot
    for _,equip in ipairs(slot[3]) do slotsByEquip[equip]=slot end
end
local slotDefaults={Mode=1,Track=true,MaxTrack=4,Level=false,MaxLevel=300,Quality=false,MaxQuality=4,Match=1}
local slotRanges={Mode={1,4},MaxTrack={1,6},MaxLevel={1,2000},MaxQuality={1,4},Match={1,2}}
local slotModes={"Follow the Gear rule","Keep everything","Sell all eligible gear","Custom conditions"}
local function copySlotDefaults()
    local copy={};for key,value in pairs(slotDefaults) do copy[key]=value end;return copy
end
local function validSlotValue(key,value)
    local default=slotDefaults[key]
    if default==nil or type(value)~=type(default) then return false end
    if type(value)=="boolean" then return true end
    local range=slotRanges[key];value=number(value)
    return value~=nil and value%1==0 and value>=range[1] and value<=range[2]
end
local function characterID()
    local guid=text(call(UnitGUID,"player"))
    if guid and guid:match("^Player%-%w[%w%-]*$") then return guid end
end
local function slotStore(create)
    if type(LamdaUIDB)~="table" then return nil,false end
    local store=LamdaUIDB.vendorSlots
    if store==nil then
        if not create then return nil,true end
        store={schema=1,characters={}};LamdaUIDB.vendorSlots=store
    end
    if type(store)~="table" or store.schema~=1 or type(store.characters)~="table" then return nil,false end
    return store,true
end
function V:SlotRule(id)
    if not slotsByID[id] then return nil,"Unknown equipment slot" end
    local guid=characterID();if not guid then return nil,"Character slot rules unavailable" end
    local store,known=slotStore(false)
    if not known then return nil,"Slot settings unavailable" end
    local saved=store and store.characters[guid]
    if saved==nil then return copySlotDefaults() end
    if type(saved)~="table" then return nil,"Reset this character's slot rules" end
    local rule=saved[id]
    if rule==nil then return copySlotDefaults() end
    if type(rule)~="table" then return nil,"Reset this slot's rule" end
    for key in pairs(slotDefaults) do
        if not validSlotValue(key,rule[key]) then return nil,"Reset this slot's rule" end
    end
    return rule
end
function V:SetSlotValue(id,key,value)
    if not slotsByID[id] or not validSlotValue(key,value) then return false end
    local guid=characterID();if not guid then return false end
    local store,known=slotStore(true);if not known then return false end
    local rule=self:SlotRule(id)
    -- Corrupt data must be explicitly reset, never normalized into selling.
    if not rule then return false end
    if not store.characters[guid] then store.characters[guid]={} end
    rule[key]=value;store.characters[guid][id]=rule
    return true
end
function V:ResetSlots(id)
    if id~=nil and not slotsByID[id] then return false end
    local guid=characterID();if not guid then return false end
    local store,known=slotStore(false);if not known then return false end
    if not store then return true end
    if id then
        local saved=store.characters[guid]
        if type(saved)~="table" then return saved==nil end
        saved[id]=nil
    else store.characters[guid]=nil end
    return true
end
local function trackNumber(label)
    label=text(label);if not label then return end
    label=label:gsub("|c%x%x%x%x%x%x%x%x",""):gsub("|r","")
    -- Only explicit names/ranks from the client are accepted. Never infer a
    -- selling track from item level, rarity, season-specific bonus IDs or caps.
    for i,name in ipairs(tracks) do
        if label:match("%f[%a]"..name.."%s+%d+%s*/%s*%d+%f[%D]")
            or label:match("^%s*"..name.."%s*$") then return i end
    end
end
function V:Track(item)
    local info=public(call(C_Item and C_Item.GetItemUpgradeInfo,item.link))
    local track=type(info)=="table" and trackNumber(info.trackString)
    if track then return track end
    local tip=public(call(C_TooltipInfo and C_TooltipInfo.GetBagItem,item.bag,item.slot))
    local lines=type(tip)=="table" and public(tip.lines)
    if type(lines)=="table" then
        for _,line in ipairs(lines) do
            if type(public(line))=="table" then
                track=trackNumber(line.leftText)
                if track then return track end
            end
        end
    end
end
function V:RuleSummary(db,prefix)
    local clauses={}
    if db[prefix.."Track"] then clauses[#clauses+1]=tracks[db[prefix.."MaxTrack"]].." track or below" end
    if db[prefix.."Level"] then clauses[#clauses+1]="item level "..db[prefix.."MaxLevel"].." or below" end
    if db[prefix.."Quality"] then clauses[#clauses+1]=qualities[db[prefix.."MaxQuality"]].." quality or below" end
    if #clauses==0 then return "No conditions selected: this rule sells nothing." end
    return "Sell when "..table.concat(clauses,db[prefix.."Match"]==2 and " OR " or " AND ").."."
end
function V:Matches(item,db,prefix)
    local results,matched,missed={},{},{}
    if db[prefix.."Track"] then
        if not item.track then return nil,"No readable upgrade track" end
        results[#results+1]=item.track<=db[prefix.."MaxTrack"]
        if results[#results] then matched[#matched+1]=tracks[item.track].." track"
        else missed[#missed+1]=tracks[item.track].." is above "..tracks[db[prefix.."MaxTrack"]] end
    end
    if db[prefix.."Level"] then
        if not item.level or item.level<=0 then return nil,"Item level unavailable" end
        results[#results+1]=item.level<=db[prefix.."MaxLevel"]
        if results[#results] then matched[#matched+1]="item level "..item.level
        else missed[#missed+1]="Item level "..item.level.." is above "..db[prefix.."MaxLevel"] end
    end
    if db[prefix.."Quality"] then
        if not item.quality then return nil,"Item quality unavailable" end
        results[#results+1]=item.quality<=db[prefix.."MaxQuality"]
        if results[#results] then matched[#matched+1]=(qualities[item.quality] or "Poor").." quality"
        else missed[#missed+1]=qualities[item.quality].." is above "..qualities[db[prefix.."MaxQuality"]] end
    end
    if #results==0 then return false,"No selling conditions selected" end
    local matches=db[prefix.."Match"]==1
    for _,value in ipairs(results) do
        if db[prefix.."Match"]==1 then matches=matches and value else matches=matches or value end
    end
    return matches,matches and table.concat(matched," + ") or table.concat(missed,"; ")
end
function V:Read(bag,slot)
    local C=C_Container
    if not C or type(C.GetContainerItemInfo)~="function" then return nil,false end
    local ok,info=pcall(C.GetContainerItemInfo,bag,slot)
    if not ok or (issecretvalue and issecretvalue(info)) then return nil,false end
    if info==nil then return nil,true end
    if type(info)~="table" then return nil,false end
    local id,count=number(info.itemID),number(info.stackCount)
    if not id or not count or count<1 then return nil,false end
    local link=public(info.hyperlink)
    if type(link)~="string" or link=="" then return nil,false end
    local item={bag=bag,slot=slot,id=id,count=count,quality=number(info.quality),
        locked=public(info.isLocked)~=false,noValue=public(info.hasNoValue),loot=public(info.hasLoot),
        bound=boolean(info.isBound),icon=number(info.iconFileID),name=text(info.itemName),
        link=link}
    item.location=call(ItemLocation and ItemLocation.CreateFromBagAndSlot,ItemLocation,bag,slot)
    item.guid=item.location and text(call(C_Item and C_Item.GetItemGUID,item.location))
    -- Counts use the exact variant link, which must be readable. A temporarily
    -- unavailable GUID must not look like a removed item/confirmed sale.
    item.identity=item.link
    item.key=bag..":"..slot..":"..item.link..":"..(item.guid or "")..":"..count
    return item,true
end
function V:CheckSale(item)
    if item.noValue~=false or not item.price or item.price<=0 then return false,"No vendor value" end
    if item.loot~=false then return false,"Loot container" end
    local C=C_Container
    local quest=public(call(C and C.GetContainerItemQuestInfo,item.bag,item.slot))
    if type(quest)~="table" or boolean(quest.isQuestItem)==nil
        or (issecretvalue and issecretvalue(quest.questID)) then return nil,"Quest information unavailable" end
    if quest.isQuestItem or (quest.questID~=nil and number(quest.questID)~=0) then return false,"Quest item" end
    if type(C.GetContainerItemPurchaseInfo)~="function" then return nil,"Refund information unavailable" end
    local ok,purchase=pcall(C.GetContainerItemPurchaseInfo,item.bag,item.slot,false)
    if not ok or (issecretvalue and issecretvalue(purchase)) then return nil,"Refund information unavailable" end
    if purchase~=nil then
        if type(purchase)~="table" or not number(purchase.refundSeconds) then return nil,"Refund information unavailable" end
        if purchase.refundSeconds>0 then return false,"Refundable purchase" end
    end
    return true
end
function V:Evaluate(item,db,now,preview)
    if db.vendorSettingsUnavailable then return nil,"Character selling settings unavailable" end
    if protectedItem(db,item.id) then return false,"On your keep list" end
    local info={call(C_Item and C_Item.GetItemInfo or GetItemInfo,item.link)}
    item.name=text(info[1]) or item.name;item.price=number(info[11]);item.quality=item.quality or number(info[3])
    item.icon=item.icon or number(info[10]);item.equip=text(info[9]);item.class=number(info[12])
    if not item.price or item.quality==nil then
        self.requests=self.requests or {}
        if now>=(self.requests[item.id] or 0) then
            self.requests[item.id]=now+2;call(C_Item and C_Item.RequestLoadItemDataByID,item.id)
        end
        return nil,"Waiting for item data"
    end
    if item.quality>=5 then return false,"Legendary, artifact or heirloom: always kept" end
    local valid,reason=self:CheckSale(item)
    if not valid then return valid,reason end
    if not item.class or not item.equip then return nil,"Equipment type unavailable" end
    item.gear=(item.class==2 or item.class==4) and gearSlots[item.equip] or false
    -- Gray non-equipment keeps its existing independent junk policy.
    if not item.gear then
        if item.quality==0 then return db.vendorSellJunk,db.vendorSellJunk and "Gray junk" or "Gray selling is off" end
        return false,"Not combat equipment"
    end
    local slot=slotsByEquip[item.equip]
    if not slot then return nil,"Equipment slot unavailable" end
    item.slotID=slot[1];item.slotName=slot[2]
    local slotRule,slotError=self:SlotRule(slot[1])
    if not slotRule then return nil,slotError end
    if slotRule.Mode==2 then return false,slot[2]..": keep everything" end
    if item.quality>0 and not preview and not db.vendorSellGear then return false,"Gear selling is off" end
    if db.vendorKeepTrinkets and item.equip=="INVTYPE_TRINKET" then return false,"Keep trinkets" end
    if db.vendorKeepSets then
        local inSet=boolean(call(C_Container.GetContainerItemEquipmentSetInfo,item.bag,item.slot))
        if inSet==nil then return nil,"Equipment-set information unavailable" end
        if inSet then return false,"Saved equipment set" end
    end
    local account=boolean(call(C_Item and C_Item.IsItemBindToAccount,item.link))
    local untilEquip=item.location and boolean(call(C_Item and C_Item.IsBoundToAccountUntilEquip,item.location))
    if account==nil or untilEquip==nil then return nil,"Warbound status unavailable" end
    item.warbound=account or untilEquip
    local prefix="vendorGear"
    if item.warbound then
        if db.vendorWarboundMode==1 then return false,"Keep all warbound gear" end
        if db.vendorWarboundMode==3 then prefix="vendorWarbound" end
    elseif db.vendorKeepBoE then
        local binding=number(info[14])
        if not binding then return nil,"Binding information unavailable" end
        if binding==2 then
            if item.bound==nil then return nil,"Binding information unavailable" end
            if not item.bound then return false,"Unbound bind-on-equip gear" end
        end
    end
    local useSlot=slotRule.Mode~=1 and prefix~="vendorWarbound"
    if item.quality==0 then
        -- Warbound equipment still needs explicit gear authorization and a rule;
        -- the junk toggle must never bypass a warbound retention policy.
        if not item.warbound and not useSlot then return db.vendorSellJunk,db.vendorSellJunk and "Gray junk" or "Gray selling is off" end
        if not preview and not db.vendorSellGear then return false,"Gear selling is off" end
    end
    local ruleName=prefix=="vendorWarbound" and "Warbound rule" or "Gear rule"
    if useSlot then
        ruleName=slot[2].." rule"
        if slotRule.Mode==3 then return true,ruleName..": sell all eligible gear" end
        db=slotRule;prefix=""
    end
    if db[prefix.."Track"] then item.track=self:Track(item) end
    if db[prefix.."Level"] then item.level=number(call(C_Item and C_Item.GetDetailedItemLevelInfo,item.link)) end
    local matches,why=self:Matches(item,db,prefix)
    if matches or useSlot then why=ruleName..": "..why end
    return matches,why
end
function V:Scan(now,db,preview)
    local items,counts,complete,waiting,rows={},{},true,0,{}
    local C=C_Container
    for _,bag in ipairs(bags()) do
        local slots=number(call(C and C.GetContainerNumSlots,bag))
        if not slots then complete=false else
            for slot=1,slots do
                local item,known=self:Read(bag,slot)
                if not known then complete=false;waiting=waiting+1 end
                if item then
                    counts[item.identity]=(counts[item.identity] or 0)+item.count
                    if preview or db.vendorSellGear or item.quality==nil or item.quality==0 then
                        local eligible,reason=self:Evaluate(item,db,now,preview)
                        item.decision=eligible==nil and "Waiting" or (eligible and "Sell" or "Keep")
                        item.reason=reason;rows[#rows+1]=item
                        if eligible then items[#items+1]=item elseif eligible==nil then waiting=waiting+1 end
                    end
                end
            end
        end
    end
    return items,counts,complete,waiting+(complete and 0 or 1),rows
end
function V:CanAct()
    if not self.open or self.bypass or not LUI:DB().vendorEnabled then return false end
    local _,known=self:Settings()
    if not known then self:SetStatus("Character selling settings unavailable. Automation paused.");return false end
    if not MerchantFrame or not MerchantFrame:IsShown() then return false end
    if self.settings and self.settings:IsVisible() then self:SetStatus("Selling and repairs paused while editing LamdaVendor.");return false end
    if InCombatLockdown and InCombatLockdown() then self:SetStatus("Paused during combat.");return false end
    if public(MerchantFrame.selectedTab)==2 then self:SetStatus("Paused while viewing buyback.");return false end
    local cursor=call(GetCursorInfo)
    if (issecretvalue and issecretvalue(cursor)) or cursor~=nil then self:SetStatus("Paused while holding an item.");return false end
    if StaticPopup_Visible and StaticPopup_Visible("CONFIRM_MERCHANT_TRADE_TIMER_REMOVAL") then
        self:SetStatus("Waiting for your merchant confirmation.");return false
    end
    return true
end
function V:Retry(item,now)
    local failed=self.failures[item.key] or {count=0}
    failed.count=failed.count+1;failed.after=now+math.min(8,2^failed.count)
    self.failures[item.key]=failed
end
function V:Repair(now,db)
    if self.repairDone or public(call(CanMerchantRepair))~=true then return end
    local cost,needed=call(GetRepairAllCost);cost=number(cost);needed=public(needed)
    if not cost or type(needed)~="boolean" then return end
    if self.repairPending then
        local pending=self.repairPending
        if cost==0 or not needed then
            self.repairDone=true;self.repairPending=nil;self.repaired=pending.cost
            self.repairSource=pending.guild and "guild" or "personal"
            self:SetStatus("Repaired for "..money(pending.cost).." ("..self.repairSource..").")
            return
        end
        if now<pending.deadline then return end
        self.repairPending=nil
        self.nextRepair=now+1
    end
    if cost<=0 or not needed then self.repairDone=true;return end
    if not db.vendorAutoRepair then return end
    if now<(self.nextRepair or 0) then return end
    local guild=false
    if db.vendorGuildRepair and not self.guildTried and public(call(CanGuildBankRepair))==true then
        local available=number(call(GetGuildBankWithdrawMoney))
        local funds=number(call(GetGuildBankMoney))
        guild=available~=nil and funds~=nil and funds>=cost and (available==-1 or available>=cost)
    end
    if not guild and (number(call(GetMoney)) or 0)<cost then
        self:SetStatus("Not enough gold to repair ("..money(cost).."). Selling continues.");return
    end
    if not guild and self.personalRepairs>=2 then self:SetStatus("Repair was not confirmed; try the merchant's repair button.");return end
    if guild then self.guildTried=true else self.personalRepairs=self.personalRepairs+1 end
    self.repairPending={cost=cost,guild=guild,deadline=now+math.max(5,self.timeout)}
    call(RepairAllItems,guild)
end
function V:Tick(now)
    if not self.open then return end
    local db=V:Settings()
    if not self:CanAct() then return end
    if now<(self.nextPoll or 0) then return end
    self.nextPoll=now+0.2
    local items,counts,complete,waiting=self:Scan(now,db)
    if self.pending then
        local p=self.pending
        if complete and (counts[p.identity] or 0)<=p.total-p.count then
            self.soldStacks=self.soldStacks+1;self.soldItems=self.soldItems+p.count
            self.proceeds=self.proceeds+p.count*p.price
            self.pending=nil
        else
            local current,known=self:Read(p.bag,p.slot)
            if known and (not current or current.key~=p.key) then
                -- A bag sort/stack move is not a confirmed sale. Scan its new position.
                self.pending=nil
            elseif now>=p.deadline then self:Retry(p,now);self.pending=nil
            else return end
        end
    end
    -- Repair acknowledgement and sale acknowledgement must not overlap.
    if self.repairPending then self:Repair(now,db);return end
    local remaining,blocked=0,0
    if (db.vendorSellJunk or db.vendorSellGear) and not self.refusesSales then
        for _,item in ipairs(items) do
            remaining=remaining+1
            local failed=self.failures[item.key]
            if item.locked then -- Revisit locked stacks; they never consume retries.
            elseif failed and failed.count>=3 then blocked=blocked+1
            elseif (not failed or now>=failed.after) and now>=(self.nextAction or 0) then
                -- Binding, sets, refund timers and settings can change between
                -- scans. Re-evaluate the exact item immediately before dispatch.
                local current=self:Read(item.bag,item.slot)
                if current and current.key==item.key and not current.locked
                    and self:Evaluate(current,V:Settings(),now)==true and self:CanAct() then
                    current.total=counts[item.identity];current.deadline=now+self.timeout
                    self.pending=current;self.nextAction=now+self.pace
                    self:SetStatus(string.format("Selling matched loot: %d stacks sold; %d remaining.",self.soldStacks,#items))
                    local ok=pcall(C_Container.UseContainerItem,item.bag,item.slot)
                    if not ok then self.pending=nil;self:Retry(item,now) end
                    return
                end
            end
        end
    end
    if not db.vendorSellJunk and not db.vendorSellGear then self:SetStatus("Auto-selling is off.")
    elseif remaining>0 or waiting>0 then
        self:SetStatus(string.format("%d stacks sold; %d waiting for bags, locks or retries%s.",self.soldStacks,remaining+waiting,
            blocked>0 and ("; "..blocked.." need manual attention") or ""))
    elseif self.refusesSales then self:SetStatus("This merchant does not buy items.")
    else self:SetStatus(string.format("Matching loot clear. Sold %d stacks (%s).",self.soldStacks,money(self.proceeds))) end
    self:Repair(now,db)
end
function V:Open()
    if self.open then return end
    self.open=true;self.bypass=public(call(IsShiftKeyDown))==true
    self.pending=nil;self.failures={};self.requests={};self.refusesSales=false
    self.soldStacks=0;self.soldItems=0;self.proceeds=0;self.repaired=nil
    self.repairDone=false;self.repairPending=nil;self.guildTried=false;self.personalRepairs=0;self.nextRepair=0
    local _,_,home,world=call(GetNetStats)
    local latency=math.max(number(home) or 0,number(world) or 0)/1000
    self.pace=math.max(0.2,math.min(1,latency));self.timeout=math.max(3,math.min(12,latency*4+2))
    self.nextAction=GetTime()+0.3;self.nextPoll=self.nextAction
    self:SetStatus(self.bypass and "Skipped this visit because Shift was held." or "Checking bags and repairs...")
    self.frame:Show()
end
function V:Close()
    if not self.open then return end
    self.open=false;self.pending=nil;self.repairPending=nil;self.frame:Hide()
    if V:Settings().vendorSummary and (self.soldStacks>0 or self.repaired) then
        print("LamdaUI Vendor: sold "..self.soldStacks.." stacks ("..money(self.proceeds)..")."
            ..(self.repaired and (" Repaired for "..money(self.repaired).." ("..self.repairSource..").") or ""))
    end
end
-- Preview uses the same evaluator as the merchant, with the gear-enable switch
-- ignored deliberately. It never opens a merchant, queues a sale or repairs.
function V:Preview()
    local items,_,complete,waiting,rows=self:Scan(GetTime(),V:Settings(),true)
    local total=0
    for _,item in ipairs(items) do total=total+item.count*item.price end
    table.sort(rows,function(a,b)
        local order={Sell=1,Waiting=2,Keep=3}
        if a.decision~=b.decision then return order[a.decision]<order[b.decision] end
        if a.bag~=b.bag then return a.bag<b.bag end
        return a.slot<b.slot
    end)
    return {rows=rows,sell=#items,value=total,waiting=waiting,complete=complete}
end
function V:SetKeepItems(value)
    if type(value)~="string" or #value>1000 or value:find("[^%d%s,]") then
        return false,"Use item IDs separated by spaces or commas (up to 1,000 characters)."
    end
    local ids,seen={},{}
    for token in value:gmatch("%d+") do
        local id=tonumber(token)
        if not id or id<1 or id>2147483647 then return false,"Enter positive item IDs." end
        if not seen[id] then seen[id]=true;ids[#ids+1]=string.format("%d",id) end
    end
    if not self:SetSetting("vendorKeepItems",table.concat(ids,", ")) then return false,"Character settings unavailable." end
    return true,"Keep list saved. Applies to every copy of these item IDs."
end
function V:ToggleKeep(id)
    local db=V:Settings();local ids={};local found=false
    for token in db.vendorKeepItems:gmatch("%d+") do
        if tonumber(token)==id then found=true else ids[#ids+1]=token end
    end
    if not found then ids[#ids+1]=tostring(id) end
    return self:SetKeepItems(table.concat(ids,", "))
end
local module={id="vendor",name="LamdaVendor",enabledKey="vendorEnabled",settingsScope="global",
    enabledHelp="Sell matching loot and repair at merchants. Selling rules, keep lists and repair preferences are saved per character, across specs and profiles. Gear selling starts off on every character until you enable it there. Hold Shift when opening a merchant to skip that visit.",
    onReset=function()V:ResetSettings();V:ResetSlots()end,
    defaults={vendorEnabled=false,vendorSellJunk=true,vendorSellGear=false,vendorAutoRepair=true,
        vendorGuildRepair=false,vendorSummary=true,vendorKeepItems="",vendorKeepTrinkets=true,
        vendorKeepBoE=true,vendorKeepSets=true,vendorWarboundMode=1},
    limits={vendorWarboundMode={1,3,integer=true}}}
for _,prefix in ipairs({"vendorGear","vendorWarbound"}) do
    local values={Track=true,MaxTrack=prefix=="vendorGear" and 4 or 3,Level=false,MaxLevel=300,
        Quality=false,MaxQuality=4,Match=1}
    for key,value in pairs(values) do module.defaults[prefix..key]=value end
    module.limits[prefix.."MaxTrack"]={1,6,integer=true}
    module.limits[prefix.."MaxLevel"]={1,2000,integer=true}
    module.limits[prefix.."MaxQuality"]={1,4,integer=true}
    module.limits[prefix.."Match"]={1,2,integer=true}
end
-- Legacy preferences did not record who enabled gear selling. Preserve their
-- rules as an immutable migration seed, but never copy the selling permission.
-- Character records are independent of CD profiles and of one another.
local function validSetting(key,value)
    value=public(value)
    local default=module.defaults[key]
    if key=="vendorEnabled" or default==nil or type(value)~=type(default) then return false end
    if type(value)=="number" then
        local range=module.limits[key]
        return number(value)~=nil and (not range or value%1==0 and value>=range[1] and value<=range[2])
    end
    return type(value)~="string" or #value<=8192
end
local function newSettings(source)
    local result={}
    for key,default in pairs(module.defaults) do
        if key~="vendorEnabled" then
            if source and validSetting(key,source[key]) then result[key]=source[key]
            else result[key]=default end
        end
    end
    result.vendorSellGear=false
    return result
end
local function settingsStore(create)
    if type(LamdaUIDB)~="table" then return nil end
    local store=LamdaUIDB.vendorSettings
    if store==nil and create then
        local legacy=LUI:DB()
        store={schema=1,legacy=newSettings(legacy),characters={}}
        LamdaUIDB.vendorSettings=store
        -- An older addon restored later must not resume account-wide gear sales.
        legacy.vendorSellGear=false
    end
    if type(store)~="table" or store.schema~=1 or type(store.characters)~="table" or type(store.legacy)~="table" then return nil end
    return store
end
function V:Settings()
    local guid=characterID()
    local store=guid and settingsStore(true)
    if store then
        local saved=store.characters[guid]
        if saved==nil then saved=newSettings(store.legacy);store.characters[guid]=saved end
        local known=type(saved)=="table"
        if known then
            for key in pairs(module.defaults) do
                if key~="vendorEnabled" and not validSetting(key,saved[key]) then known=false;break end
            end
        end
        if known then return saved,true end
    end
    local disabled=newSettings()
    disabled.vendorSellJunk=false;disabled.vendorAutoRepair=false;disabled.vendorGuildRepair=false
    disabled.vendorSettingsUnavailable=true
    return disabled,false
end
function V:SetSetting(key,value)
    if not validSetting(key,value) then return false end
    local db,known=self:Settings();if not known then return false end
    db[key]=value
    return true
end
function V:ResetSettings(keys)
    local guid=characterID();local store=guid and settingsStore(true)
    if not store then return false end
    if keys then
        local db,known=self:Settings();if not known then return false end
        for _,key in ipairs(keys) do
            if key~="vendorEnabled" and module.defaults[key]~=nil then db[key]=module.defaults[key] end
        end
    else
        -- Preserve table identity for open controls; a corrupt character record
        -- is replaced only by this explicit reset, never during a normal read.
        local saved=store.characters[guid]
        if type(saved)~="table" then saved={};store.characters[guid]=saved end
        for key in pairs(saved) do saved[key]=nil end
        for key,value in pairs(newSettings()) do saved[key]=value end
    end
    if LUI.RefreshUI then LUI:RefreshUI()end
    return true
end
local function ruleKeys(prefix)
    local keys={}
    for _,suffix in ipairs({"Track","MaxTrack","Level","MaxLevel","Quality","MaxQuality","Match"}) do keys[#keys+1]=prefix..suffix end
    return keys
end
local function readSettings()return V:Settings()end
local function writeSetting(key,value)V:SetSetting(key,value)end
local function choice(parent,key,values,x,y,width,read,write)
    read=read or readSettings;write=write or writeSetting
    return LUI:Dropdown(parent,x,y,width or 280,function()return values end,
        function()return values[read()[key]]end,function(value)
            for i,label in ipairs(values) do if label==value then write(key,i);break end end
        end)
end
local function ruleCheck(parent,label,key,y,help,read,write)
    read=read or readSettings;write=write or writeSetting
    local button=CreateFrame("CheckButton",nil,parent,"UICheckButtonTemplate")
    button:SetSize(26,26);button:SetPoint("TOPLEFT",0,y);button:SetHitRectInsets(0,-210,0,0)
    local caption=button:CreateFontString(nil,"OVERLAY","GameFontHighlight")
    caption:SetPoint("LEFT",button,"RIGHT",4,0);caption:SetText(label)
    button:SetScript("OnClick",function(self)write(key,not not self:GetChecked());LUI:RefreshUI()end)
    LUI:OnRefresh(button,function()button:SetChecked(read()[key])end)
    LUI:Explain(button,label,help);LUI:Help(parent,label,help,270,y)
    return button
end
local function buildRule(parent,prefix,read,write)
    read=read or readSettings;write=write or writeSetting
    LUI:Label(parent,"Sell equipment matching",4,-5)
    choice(parent,prefix.."Match",{"ALL conditions (AND)","ANY condition (OR)"},320,0,nil,read,write)
    local track=ruleCheck(parent,"Upgrade track at or below",prefix.."Track",-44,
        "Match the named upgrade track, including its maximum rank. Champion includes Explorer, Adventurer and Veteran. Gear with an unreadable or unrecognized track is kept whenever this condition is selected.",read,write)
    local names={};for i,name in ipairs(tracks) do names[i]=name.." and below" end
    local trackValue=choice(parent,prefix.."MaxTrack",names,320,-44,nil,read,write)
    ruleCheck(parent,"Item level at or below",prefix.."Level",-88,
        "Compare this exact item's current item level, not its possible fully upgraded level. Type a whole number and press Enter or leave the field to save it.",read,write)
    local level=CreateFrame("EditBox",nil,parent,"InputBoxTemplate")
    level:SetPoint("TOPLEFT",328,-90);level:SetSize(100,26);level:SetAutoFocus(false);level:SetMaxLetters(4);level:SetNumeric(true)
    local function commit(self)
        local value=tonumber(self:GetText())
        if value and value>=1 and value<=2000 and value%1==0 then write(prefix.."MaxLevel",value) end
        self:SetText(tostring(read()[prefix.."MaxLevel"]));LUI:RefreshUI()
    end
    level:SetScript("OnEnterPressed",function(self)commit(self);self:ClearFocus()end)
    level:SetScript("OnEditFocusLost",commit)
    level:SetScript("OnEscapePressed",function(self)self:SetText(tostring(read()[prefix.."MaxLevel"]));self:ClearFocus()end)
    parent.commitRuleInput=function()if level:HasFocus() then commit(level);level:ClearFocus()end end
    LUI:Explain(level,"Maximum item level","Whole number from 1 to 2,000. The threshold is inclusive. Changes save on Enter or when you leave the field; an invalid value restores the last saved value.")
    ruleCheck(parent,"Quality at or below",prefix.."Quality",-132,
        "Quality is rarity (Common, Uncommon, Rare or Epic), not upgrade track. Legendary, artifact and heirloom items are always kept.",read,write)
    local qualityNames={};for i,name in ipairs(qualities) do qualityNames[i]=name.." and below" end
    local qualityValue=choice(parent,prefix.."MaxQuality",qualityNames,320,-132,nil,read,write)
    local summary=LUI:Description(parent,"",4,-182,596)
    LUI:Description(parent,"Missing data for a selected condition keeps the item. Selecting no conditions sells nothing. The Keep list always takes priority.",4,-230,596)
    LUI:OnRefresh(parent,function()
        local db=read()
        if not level:HasFocus() then level:SetText(tostring(db[prefix.."MaxLevel"])) end
        trackValue:SetEnabled(db[prefix.."Track"]);qualityValue:SetEnabled(db[prefix.."Quality"])
        level:SetEnabled(db[prefix.."Level"]);level:SetAlpha(db[prefix.."Level"] and 1 or .45)
        summary:SetText(V:RuleSummary(db,prefix))
    end)
    return track
end
local function buildSelling(parent)
    ruleCheck(parent,"Sell gray junk","vendorSellJunk",0,
        "Sell poor-quality items with a vendor value. Equipment exclusions and the item keep list still apply. There is no twelve-item stop; older sales can leave the merchant's buyback list.")
    ruleCheck(parent,"Auto-sell matching gear","vendorSellGear",-42,
        "Enable gear selling for this character only, using its Gear rule, slot overrides and Warbound policy. Review the Preview tab first. Ordinary gray junk remains controlled separately.")
    local state=LUI:Description(parent,"",4,-86,596)
    ruleCheck(parent,"Repair automatically","vendorAutoRepair",-146,"Repair after selling so sale proceeds can cover the cost. Repair requires enough available gold.")
    ruleCheck(parent,"Try guild repairs first","vendorGuildRepair",-188,"Use permitted guild repair funds when they cover the repair; otherwise use personal gold. Unconfirmed guild repairs fall back to personal gold.")
    ruleCheck(parent,"Print visit summary","vendorSummary",-230,"Report confirmed sales and repairs when you close the merchant.")
    LUI:Description(parent,"Hold Shift when opening a merchant to skip that visit. Selling pauses while editing this module. Settings save live; no reload is needed.",4,-278,596)
    V.label=LUI:Description(parent,V.status,4,-330,596)
    LUI:ResetButton(parent,"LamdaVendor",function()LUI:ResetModule("vendor")end,4,-378)
    LUI:OnRefresh(parent,function()
        state:SetText(V:Settings().vendorSellGear and "Gear selling ON for this character. Review matching items in Preview." or "Gear selling OFF for this character. Review your rules in Preview before enabling it.")
        state:SetTextColor(V:Settings().vendorSellGear and 1 or .75,.85,.6)
    end)
end
local function buildGear(parent)
    buildRule(parent,"vendorGear")
    LUI:Description(parent,"Use Slots to override this rule for individual equipment slots. Warbound has its own policy; the Keep list protects trinkets, BoEs, sets and individual items.",4,-288,596)
    LUI:ResetButton(parent,"this character’s gear conditions",function()V:ResetSettings(ruleKeys("vendorGear"))end,4,-378)
end
local function buildSlots(parent)
    local selected="head"
    LUI:Label(parent,"Equipment slot",4,0);LUI:Label(parent,"What to do",324,0)
    local editor=CreateFrame("Frame",nil,parent);editor:SetPoint("TOPLEFT",0,-100);editor:SetSize(620,272)
    local function read()return V:SlotRule(selected) or copySlotDefaults()end
    local function write(key,value)V:SetSlotValue(selected,key,value)end
    buildRule(editor,"",read,write)
    local shortModes={"Default","Keep","Sell all","Custom"}
    local function slotLabel(slot)
        local rule=V:SlotRule(slot[1])
        return slot[2].." - "..(rule and shortModes[rule.Mode] or "Unavailable")
    end
    local picker=LUI:Dropdown(parent,4,-24,296,function()
        local values={};for _,slot in ipairs(slots) do values[#values+1]=slotLabel(slot) end;return values
    end,function()return slotLabel(slotsByID[selected])end,function(value)
        for _,slot in ipairs(slots) do
            if value==slotLabel(slot) then editor.commitRuleInput();selected=slot[1];break end
        end
    end)
    LUI:Explain(picker,"Equipment slot","Rules are saved for this character. Rings and trinkets each cover both slots. One-hand and two-hand weapons have their own groups because a bag weapon may be usable in either hand. Off-hand / shields covers explicit off-hand weapons, shields and held items.")
    local mode=choice(parent,"Mode",slotModes,320,-24,280,read,function(key,value)
        editor.commitRuleInput();write(key,value)
    end)
    LUI:Explain(mode,"Slot rule","Follow the Gear rule leaves the normal behavior. Keep everything protects this slot, including gray and warbound gear. Sell all eligible gear ignores track, item level and rarity conditions, while every keep/unsellable protection still applies. Custom conditions replace the Gear rule only for this slot.")
    local scope=LUI:Description(parent,"",4,-64,596)
    local details=LUI:Description(parent,"",4,-112,596)
    local count=LUI:Description(parent,"",4,-240,596)
    local reminders=LUI:Description(parent,"Bag gear only. Auto-sell matching gear must be on for slot selling. Use Preview to review matches; equipped items are never sold.",4,-304,596)
    local reset=LUI:ResetButton(parent,"the selected slot for this character",function()
        V:ResetSlots(selected);LUI:RefreshUI()
    end,4,-378);reset:SetText("Reset this slot")
    local resetAll=LUI:ResetButton(parent,"all slot rules for this character",function()
        V:ResetSlots();LUI:RefreshUI()
    end,172,-378);resetAll:SetText("Reset all slots")
    LUI:OnRefresh(parent,function()
        local rule,why=V:SlotRule(selected);local custom=rule and rule.Mode==4
        editor:SetShown(custom);details:SetShown(not custom);count:SetShown(not custom);reminders:SetShown(not custom)
        local guid=characterID();mode:SetEnabled(rule~=nil);picker:SetEnabled(guid~=nil)
        reset:SetEnabled(guid~=nil);resetAll:SetEnabled(guid~=nil)
        scope:SetText(guid and ("This character | Gear selling "..(V:Settings().vendorSellGear and "ON" or "OFF")..". Keep exceptions win. Custom warbound rules stay separate.") or "Character unavailable. Slot editing and equipment sales are paused.")
        local label=slotsByID[selected][2]
        if not rule then details:SetText(why..". No items in this slot will be sold.")
        elseif rule.Mode==1 then details:SetText(label..": follow your Gear rule and Warbound policy. Gray junk keeps its normal setting.\n\nChoose Sell all eligible gear once you no longer need upgrades here, or Custom conditions to retain higher-track off-spec pieces.")
        elseif rule.Mode==2 then details:SetText(label..": keep every item in this slot, including gray and warbound gear. This takes priority over your other selling rules.")
        elseif rule.Mode==3 then details:SetText(label..": sell all eligible bag gear, including Hero/Myth and gear with no upgrade track. Your Keep list, equipment-set protection and Warbound policy still apply.") end
        local changed=0
        for _,slot in ipairs(slots) do local current=V:SlotRule(slot[1]);if current and current.Mode~=1 then changed=changed+1 end end
        count:SetText(changed.." customized slots on this character. Each slot's dropdown entry shows its current mode.")
    end)
    parent.slotEditor=editor;parent.slotPicker=picker;parent.slotMode=mode
end
local function buildWarbound(parent)
    LUI:Label(parent,"Warbound gear",4,-5)
    choice(parent,"vendorWarboundMode",{"Keep all warbound gear","Follow the Gear rule","Custom warbound rule"},290,0,310)
    local explanation=LUI:Description(parent,"",4,-42,596)
    local rule=CreateFrame("Frame",nil,parent);rule:SetPoint("TOPLEFT",0,-100);rule:SetSize(620,272)
    buildRule(rule,"vendorWarbound")
    local keys=ruleKeys("vendorWarbound");keys[#keys+1]="vendorWarboundMode"
    LUI:ResetButton(parent,"this character’s warbound policy",function()V:ResetSettings(keys)end,4,-378)
    LUI:OnRefresh(parent,function()
        local mode=V:Settings().vendorWarboundMode;rule:SetShown(mode==3)
        explanation:SetText(mode==1 and "Keep all warbound and warbound-until-equipped gear, regardless of track or item level." or
            (mode==2 and "Apply the Gear rule, including this character's slot overrides, to warbound gear. Keep exceptions still win." or
            "These conditions replace the Gear rule and slot selling rules for warbound gear. Keep-list and keep-slot protections still win."))
    end)
end
local function buildKeep(parent)
    ruleCheck(parent,"Keep trinkets","vendorKeepTrinkets",0,"Omit all trinkets from both gear rules and gray sales, regardless of their track or item level.")
    ruleCheck(parent,"Keep unbound BoE gear","vendorKeepBoE",-44,"Keep bind-on-equip gear that can still be traded. A previously equipped, now soulbound copy follows your selling rule. Warbound equipment uses the separate Warbound policy.")
    ruleCheck(parent,"Keep saved equipment sets","vendorKeepSets",-88,"Keep bag items used in any of this character's saved equipment sets, including off-spec sets. If set membership cannot be read, keep the item.")
    LUI:Label(parent,"Always keep these item IDs",4,-144)
    local edit=CreateFrame("EditBox",nil,parent,"InputBoxTemplate")
    edit:SetSize(442,28);edit:SetPoint("TOPLEFT",8,-168);edit:SetAutoFocus(false);edit:SetMaxLetters(1000)
    local hint=LUI:Description(parent,"Separate IDs with spaces or commas, or use Keep beside an item in Preview.",4,-210,596)
    local function save()
        local ok,message=V:SetKeepItems(edit:GetText());hint:SetText(message)
        if ok then edit:SetText(V:Settings().vendorKeepItems);edit:ClearFocus();LUI:RefreshUI()end
    end
    edit:SetScript("OnEnterPressed",save)
    edit:SetScript("OnEscapePressed",function(self)self:SetText(V:Settings().vendorKeepItems);self:ClearFocus()end)
    LUI:Button(parent,"Save keep list",468,-169,132,save)
    LUI:OnRefresh(edit,function()if not edit:HasFocus() then edit:SetText(V:Settings().vendorKeepItems)end end)
    LUI:Description(parent,"Always kept: legendary, artifact and heirloom items; quest items; refundable purchases; loot containers; and items without vendor value. Gear rules only cover combat equipment in your bags.",4,-266,596)
    LUI:ResetButton(parent,"this character’s keep exceptions",function()V:ResetSettings({"vendorKeepItems","vendorKeepTrinkets","vendorKeepBoE","vendorKeepSets"})end,4,-378)
end
local function buildPreview(parent)
    LUI:Description(parent,"Read-only preview of your rules, including gear selling while it is OFF. No items are sold by this page. Refresh after changing bags or conditions.",4,0,596)
    local filter,page,snapshot="All items",1,nil
    local rows,filtered={},{}
    local summary=LUI:Description(parent,"",320,-49,282)
    local pager=LUI:Label(parent,"",225,-358)
    local notice=LUI:Description(parent,"",4,-391,596)
    local previous,nextPage,draw
    for i=1,6 do
        local row=CreateFrame("Frame",nil,parent);row:SetSize(600,43);row:SetPoint("TOPLEFT",0,-91-(i-1)*43)
        local icon=row:CreateTexture(nil,"ARTWORK");icon:SetSize(30,30);icon:SetPoint("TOPLEFT",4,-3)
        local name=LUI:Label(row,"",42,-1);name:SetWidth(355);name:SetMaxLines(1);name:SetWordWrap(false)
        local reason=LUI:Label(row,"",42,-22);reason:SetWidth(438);reason:SetMaxLines(1);reason:SetWordWrap(false)
        reason:SetTextColor(.72,.78,.82)
        local result=LUI:Label(row,"",410,-1)
        local keep=LUI:Button(row,"Keep",500,-4,100,function()
            if not row.item then return end
            local _,message=V:ToggleKeep(row.item.id);notice:SetText(message)
            snapshot=V:Preview();draw();LUI:RefreshUI()
        end)
        row:EnableMouse(true)
        row:SetScript("OnEnter",function(self)
            if not self.item or not GameTooltip then return end
            GameTooltip:SetOwner(self,"ANCHOR_RIGHT")
            GameTooltip:SetHyperlink(self.item.link)
            GameTooltip:AddLine(self.item.decision..": "..self.item.reason,1,.85,.4,true);GameTooltip:Show()
        end)
        row:SetScript("OnLeave",function()if GameTooltip then GameTooltip:Hide()end end)
        LUI:Explain(keep,"Item keep list","Keep adds this item ID to your keep list, protecting every copy. Unkeep removes only that exception; other protections still apply.")
        rows[i]={frame=row,icon=icon,name=name,reason=reason,result=result,keep=keep}
    end
    draw=function()
        if not snapshot then return end
        filtered={}
        for _,item in ipairs(snapshot.rows) do
            if filter=="All items" or filter=="Would sell" and item.decision=="Sell"
                or filter=="Kept" and item.decision=="Keep" or filter=="Waiting" and item.decision=="Waiting" then filtered[#filtered+1]=item end
        end
        local pages=math.max(1,math.ceil(#filtered/6));page=math.max(1,math.min(page,pages))
        for i,row in ipairs(rows) do
            local item=filtered[(page-1)*6+i];row.frame.item=item;row.frame:SetShown(item~=nil)
            if item then
                row.icon:SetTexture(item.icon or 134400)
                row.name:SetText((item.name or item.link)..(item.count>1 and (" x"..item.count) or ""))
                local r,g,b=call(C_Item and C_Item.GetItemQualityColor or GetItemQualityColor,item.quality or 1)
                row.name:SetTextColor(number(r) or 1,number(g) or 1,number(b) or 1)
                row.reason:SetText((item.slotName and (item.slotName.." | ") or "")..item.reason..(item.locked and " (locked; waits)" or ""))
                row.result:SetText(item.decision=="Sell" and "Sell" or item.decision)
                row.result:SetTextColor(item.decision=="Sell" and 1 or .55,item.decision=="Waiting" and .8 or .9,.65)
                row.keep:SetText(protectedItem(V:Settings(),item.id) and "Unkeep" or "Keep")
            end
        end
        summary:SetText(snapshot.sell.." stacks would sell: "..money(snapshot.value)..(snapshot.waiting>0 and (" | "..snapshot.waiting.." waiting") or ""))
        pager:SetText(#filtered.." items | "..page.." / "..pages)
        previous:SetEnabled(page>1);nextPage:SetEnabled(page<pages)
        if #snapshot.rows==0 then notice:SetText(snapshot.complete and "Your bags are empty." or "Bag information unavailable; try refreshing outside combat.")end
    end
    local function refresh()
        snapshot=V:Preview();notice:SetText("Preview updated. Hover an item for its tooltip and full reason.");draw()
    end
    LUI:Button(parent,"Refresh bags",4,-47,132,refresh)
    LUI:Dropdown(parent,150,-47,154,function()return {"All items","Would sell","Kept","Waiting"}end,function()return filter end,
        function(value)filter=value;page=1;draw()end)
    previous=LUI:Button(parent,"Previous",4,-351,100,function()page=page-1;draw()end)
    nextPage=LUI:Button(parent,"Next",500,-351,100,function()page=page+1;draw()end)
    parent:SetScript("OnShow",function()if not LUI.buildingUI then refresh()end end)
    -- A hidden preview does no inventory work during normal UI refreshes.
    parent.refreshPreview=refresh;parent.previewRows=rows
end
module.build=function(parent)
    V.settings=parent
    LUI:Tabs(parent,{{id="selling",name="Selling",build=buildSelling},
        {id="gear",name="Gear rule",build=buildGear},{id="slots",name="Slots",build=buildSlots},{id="warbound",name="Warbound",build=buildWarbound},
        {id="keep",name="Keep list",build=buildKeep},{id="preview",name="Preview",build=buildPreview}})
end
LUI:RegisterModule(module)
local frame=CreateFrame("Frame");V.frame=frame;frame:Hide()
for _,event in ipairs({"MERCHANT_SHOW","MERCHANT_CLOSED","BAG_UPDATE_DELAYED","BAG_UPDATE","ITEM_LOCK_CHANGED",
    "ITEM_DATA_LOAD_RESULT","GET_ITEM_INFO_RECEIVED","MERCHANT_UPDATE","PLAYER_MONEY","UPDATE_INVENTORY_DURABILITY","UI_ERROR_MESSAGE"}) do
    pcall(frame.RegisterEvent,frame,event)
end
frame:SetScript("OnEvent",function(_,event,_,message)
    if event=="MERCHANT_SHOW" then V:Open()
    elseif event=="MERCHANT_CLOSED" then V:Close()
    elseif V.open then
        if event=="UI_ERROR_MESSAGE" then
            message=public(message)
            if message and ERR_VENDOR_DOESNT_BUY and message==ERR_VENDOR_DOESNT_BUY then
                V.refusesSales=true;V.pending=nil
            else V.nextAction=math.max(V.nextAction or 0,GetTime()+1) end
        end
        V.nextPoll=math.min(V.nextPoll or 0,GetTime()+0.1)
    end
end)
local elapsed=0
frame:SetScript("OnUpdate",function(_,delta)
    elapsed=elapsed+delta;if elapsed<0.1 then return end;elapsed=0
    V:Tick(GetTime())
end)
