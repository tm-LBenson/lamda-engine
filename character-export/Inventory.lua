local _, E = ...
local call, array = E.call, E.array
local function itemCall(method, ...)
    return call(E.has("C_Item."..method) and "C_Item."..method or method, ...)
end
local function containerCall(method, ...)
    return call(E.has("C_Container."..method) and "C_Container."..method or method, ...)
end
local function validItemID(id)
    return type(id)=="number" and id>0 and id<math.huge and id==math.floor(id)
end

function E.FillItem(row)
    local query=row.itemString or row.itemID
    if not row.itemString and not validItemID(row.itemID) then return end
    local name,_,quality,level,required,class,subclass,stack,equip,_,price,classID,subclassID,binding,expansion,setID,reagent=itemCall("GetItemInfo",query)
    if name then
        row.name, row.quality, row.baseItemLevel = E.text(name), quality, level
        row.requiredLevel, row.class, row.subclass = required, class, subclass
        row.maxStack, row.equipLocation, row.sellCopper = stack, equip, price
        row.classID, row.subclassID, row.bindType = classID, subclassID, binding
        row.expansionID, row.setID, row.craftingReagent = expansion, setID, reagent
        row.metadataStatus="complete"
        row.itemLevel=itemCall("GetDetailedItemLevelInfo",query) or level
        if equip and equip~="" and equip~="INVTYPE_NON_EQUIP_IGNORE" then row.itemStats=itemCall("GetItemStats",query) end
        local useName,useID=itemCall("GetItemSpell",query)
        if useName or useID then row.useSpell={name=E.text(useName),spellID=useID} end
    else
        row.metadataStatus="pending"
        if E.capture and validItemID(row.itemID) then E.capture.pendingItems[row.itemID]=true end
    end
end
function E.Item(link, id)
    local row={itemID=validItemID(id) and id or nil}
    if type(link)=="string" then
        row.itemString=link:match("item:[^|]+")
        local linkedID=tonumber(link:match("item:(%d+)"))
        if validItemID(linkedID) then row.itemID=linkedID end
        row.name=E.text(link:match("|h%[(.-)%]|h"))
    end
    if not row.itemID then return nil end
    if row.itemString then
        local parts={}
        for value in (row.itemString..":"):gmatch("(.-):") do parts[#parts+1]=value end
        local enchant=tonumber(parts[3])
        if enchant and enchant>0 then row.enchantID=enchant end
        row.gems=array()
        for socket=1,4 do
            local gem=tonumber(parts[3+socket])
            if gem and gem>0 then row.gems[#row.gems+1]={socket=socket,itemID=gem} end
        end
    end
    E.FillItem(row)
    if E.capture then E.capture.itemRecords[#E.capture.itemRecords+1]=row end
    return row
end

E.register("equipment",function()
    if not E.has("GetInventoryItemLink") then return E.unavailable("Equipped item APIs are unavailable.") end
    local labels={[0]="ammo","head","neck","shoulder","shirt","chest","waist","legs","feet","wrist","hands","finger1","finger2","trinket1","trinket2","back","mainHand","offHand","ranged","tabard"}
    local rows=array()
    for slot=0,19 do
        local link=call("GetInventoryItemLink","player",slot)
        local id=call("GetInventoryItemID","player",slot)
        local row=E.Item(link,id)
        if row then
            row.slot, row.slotName=slot,labels[slot]
            row.durability,row.maxDurability=call("GetInventoryItemDurability",slot)
            rows[#rows+1]=row
        end
    end
    local overall,equipped,pvp=call("GetAverageItemLevel")
    return E.section("complete",{items=rows,averageItemLevel=overall,equippedItemLevel=equipped,pvpItemLevel=pvp})
end)

function E.ScanContainers(ids)
    local bags,available,readable=array(),false,true
    for _,bag in ipairs(ids) do
        local beforeSlots=E.readIssues or 0
        local count=containerCall("GetContainerNumSlots",bag)
        if (E.readIssues or 0)>beforeSlots then readable=false end
        if count~=nil then
            available=true
            local row={bagID=bag,slots=count,items=array(),name=E.text(containerCall("GetBagName",bag))}
            row.freeSlots,row.family=containerCall("GetContainerNumFreeSlots",bag)
            if bag~=0 then
                local inventorySlot=containerCall("ContainerIDToInventoryID",bag)
                if inventorySlot then row.container=E.Item(call("GetInventoryItemLink","player",inventorySlot),call("GetInventoryItemID","player",inventorySlot)) end
            else row.name=row.name or "Backpack" end
            for slot=1,E.count(count,1000) do
                local beforeItem=E.readIssues or 0
                local info
                if E.has("C_Container.GetContainerItemInfo") then info=call("C_Container.GetContainerItemInfo",bag,slot)
                else
                    local icon,stack,locked,quality,readable,lootable,link,filtered,noValue,id,bound=call("GetContainerItemInfo",bag,slot)
                    if icon or id or link then info={stackCount=stack,isLocked=locked,quality=quality,hyperlink=link,itemID=id,isBound=bound} end
                end
                local link=info and info.hyperlink or containerCall("GetContainerItemLink",bag,slot)
                if (E.readIssues or 0)>beforeItem then readable=false end
                local item=E.Item(link,info and info.itemID)
                if item then
                    item.slot,item.count,item.bound=slot,info and info.stackCount,info and info.isBound
                    item.quality=item.quality or (info and info.quality)
                    row.items[#row.items+1]=item
                end
            end
            bags[#bags+1]=row
        end
    end
    return bags,available,readable
end
local function carriedIDs()
    local total=NUM_TOTAL_EQUIPPED_BAG_SLOTS or NUM_BAG_SLOTS or 4
    local ids,seen={},{}
    for id=0,E.count(total,12) do ids[#ids+1]=id end
    local keyring=KEYRING_CONTAINER or (Enum and Enum.BagIndex and Enum.BagIndex.Keyring)
    for _,id in ipairs(ids) do seen[id]=true end
    if type(keyring)=="number" and not seen[keyring] then ids[#ids+1]=keyring;seen[keyring]=true end
    return ids,seen,keyring,E.count(total,12)
end
E.register("bags",function()
    local ids,_,keyring=carriedIDs()
    local bags,available=E.ScanContainers(ids)
    local free,specialized,keys=0,0,0
    for _,bag in ipairs(bags) do
        if bag.bagID==keyring then
            bag.kind="keyring";keys=keys+(bag.freeSlots or 0)
        elseif bag.family and bag.family~=0 then
            bag.kind="specialized";specialized=specialized+(bag.freeSlots or 0)
        else
            bag.kind="general";free=free+(bag.freeSlots or 0)
        end
    end
    return E.section(available and "complete" or "unavailable",{containers=bags,freeSlots=free,
        specializedFreeSlots=specialized,keyringFreeSlots=keys,totalCarriedFreeSlots=free+specialized})
end)

local function bankIDs(account)
    local ids,seen={},{}
    local _,carried,_,lastCarried=carriedIDs()
    local function add(id) if type(id)=="number" and not seen[id] and not carried[id] then seen[id]=true;ids[#ids+1]=id end end
    local indices=Enum and Enum.BagIndex or {}
    local tabbed=false
    for name,id in pairs(indices) do
        if type(name)=="string" then
            if name:match("^CharacterBankTab_") then tabbed=true end
            if (account and name:match("^AccountBankTab_")) or (not account and (name:match("^CharacterBankTab_") or name:match("^BankBag"))) then add(id) end
        end
    end
    if not account and not tabbed then
        -- Negative legacy bank IDs alias keyring/account storage on Forever.
        -- Modern clients enumerate tabs; use legacy offsets only without tabs.
        add(BANK_CONTAINER or indices.Bank)
        add(REAGENTBANK_CONTAINER or indices.Reagentbank)
        if not BANK_CONTAINER and not indices.Bank and not carried[-1] then add(-1) end
        for index=1,E.count(NUM_BANKBAGSLOTS or 7,20) do add(lastCarried+index) end
    end
    table.sort(ids)
    return ids
end
local function normalize(cache)
    local value=E.clean(cache)
    if not value then return nil end
    array(value.containers)
    for _,bag in ipairs(value.containers or {}) do array(bag.items) end
    return value
end
function E.CaptureBank()
    if not E.bankOpen or not E.db then return end
    local containers,available,readable=E.ScanContainers(bankIDs(false))
    local capacity=0;for _,bag in ipairs(containers) do capacity=capacity+(bag.slots or 0) end
    local characterUpdated=available and readable and capacity>0
    if characterUpdated then E.db.bank={status="cached",capturedAt=E.now(),containers=containers} end
    local accountUpdated=false
    local bankType=Enum and Enum.BankType and Enum.BankType.Account
    if bankType~=nil and call("C_Bank.CanViewBank",bankType)==true then
        local account,visible,accountReadable=E.ScanContainers(bankIDs(true))
        local accountCapacity=0;for _,bag in ipairs(account) do accountCapacity=accountCapacity+(bag.slots or 0) end
        if visible and accountReadable and accountCapacity>0 then
            E.db.accountBank={status="cached",capturedAt=E.now(),containers=account}
            accountUpdated=true
        end
    end
    return characterUpdated,accountUpdated
end
E.register("bank",function()
    local liveCharacter,liveAccount
    if E.bankOpen then liveCharacter,liveAccount=E.CaptureBank() end
    local character=E.db and normalize(E.db.bank)
    local account=E.db and normalize(E.db.accountBank)
    if character then character.status=liveCharacter and "live" or "cached" end
    if account then account.status=liveAccount and "live" or "cached" end
    return E.section(character and (liveCharacter and "live" or "cached") or "unavailable",{
        character=character or E.unavailable("Open your bank once with LamdaExport loaded to capture its contents."),
        account=account or E.unavailable("No account-bank contents have been captured on this character."),
        note="Cached storage is a dated observation, not a current inventory guarantee.",
    })
end)
