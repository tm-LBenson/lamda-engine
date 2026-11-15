-- Observations only: no purchases, postings, crafting, input or protected actions.
local _,LUI=...
local M={queue={},serial=0,recipeOpen=false,status="Open an auction house and choose Scan auctions. Character exports run automatically."}
local BULK_WIDTH,BULK_HEIGHT,BULK_CHUNK=512,64,24000
local FRAME_SECONDS,PASSES=1.1,3
LUI.Market=M
local ARRAY={}
local function arr(t)return setmetatable(t or {},ARRAY)end
local function plain(v,kind)
    if issecretvalue and issecretvalue(v)then return nil end
    if type(v)~=kind then return nil end
    if kind=="number"and(v~=v or math.abs(v)==math.huge)then return nil end
    return v
end
local function call(fn,...)
    if type(fn)~="function"then return nil end
    local v={pcall(fn,...)};if v[1]then return unpack(v,2,24)end
end
local function json(v)
    local kind=type(v)
    if kind=="nil"then return "null"end
    if kind=="boolean"then return v and "true"or "false"end
    if kind=="number"then assert(plain(v,"number"));return string.format("%.17g",v)end
    if kind=="string"then return '"'..v:gsub('[%z\1-\31\\"]',function(c)if c=='"'or c=='\\'then return '\\'..c end;return string.format('\\u%04x',c:byte())end)..'"'end
    assert(kind=="table","Invalid market value")
    local out={};if getmetatable(v)==ARRAY then for _,x in ipairs(v)do out[#out+1]=json(x)end;return '['..table.concat(out,',')..']'end
    local keys={};for k in pairs(v)do assert(type(k)=="string");keys[#keys+1]=k end;table.sort(keys)
    for _,k in ipairs(keys)do out[#out+1]=json(k)..':'..json(v[k])end;return '{'..table.concat(out,',')..'}'
end
function M:Status(text)
    if self.status==text then return end
    self.status=text
    for _,label in ipairs(self.labels or {})do label:SetText(text)end
end
function M:Link(parent,x,y,width)
    local port=LUI.ForeverEngine and 47642 or 47641
    local url="http://127.0.0.1:"..port.."/market/"
    local box=CreateFrame("EditBox",nil,parent,"InputBoxTemplate")
    box:SetSize(width or 380,24);box:SetPoint("TOPLEFT",x,y);box:SetAutoFocus(false);box:SetText(url);box:SetCursorPosition(0)
    box:SetScript("OnEditFocusGained",function(self)self:HighlightText()end)
    box:SetScript("OnMouseUp",function(self)self:HighlightText()end)
    box:SetScript("OnTextChanged",function(self,input)if input then self:SetText(url);self:HighlightText()end end)
    box:SetScript("OnEscapePressed",function(self)self:ClearFocus()end)
    box:SetScript("OnEnterPressed",function(self)self:ClearFocus()end)
    return box
end
function M:Panel()
    if self.panel then self.panel:Show();return end
    local p=CreateFrame("Frame","LamdaMarketCapture",UIParent,"BackdropTemplate")
    self.panel=p;p:SetSize(365,225);p:SetPoint("CENTER",UIParent,"CENTER",0,100);p:SetClampedToScreen(true);p:SetMovable(true);p:EnableMouse(true);p:RegisterForDrag("LeftButton")
    p:SetScript("OnDragStart",function(self)self:StartMoving()end);p:SetScript("OnDragStop",function(self)
        self:StopMovingOrSizing();local point,_,relative,x,y=self:GetPoint()
        LamdaMarketDB=type(LamdaMarketDB)=="table"and LamdaMarketDB or {};LamdaMarketDB.panel={point=point,relative=relative,x=x,y=y}
    end)
    local saved=type(LamdaMarketDB)=="table"and LamdaMarketDB.panel
    local anchors={TOP=true,TOPLEFT=true,TOPRIGHT=true,CENTER=true,LEFT=true,RIGHT=true,BOTTOM=true,BOTTOMLEFT=true,BOTTOMRIGHT=true}
    if type(saved)=="table"and anchors[saved.point]and anchors[saved.relative]and plain(saved.x,"number")and plain(saved.y,"number")then
        p:ClearAllPoints();p:SetPoint(saved.point,UIParent,saved.relative,math.max(-16000,math.min(16000,saved.x)),math.max(-16000,math.min(16000,saved.y)))
    end
    p:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Tooltips\\UI-Tooltip-Border",edgeSize=16,insets={left=4,right=4,top=4,bottom=4}})
    p:SetBackdropColor(.04,.07,.09,.98)
    LUI:Label(p,"LamdaMarket",14,-12,true)
    local close=CreateFrame("Button",nil,p,"UIPanelCloseButton");close:SetPoint("TOPRIGHT",0,0);close:SetScript("OnClick",function()p:Hide()end)
    LUI:Button(p,"Scan auctions",14,-43,150,function()M:Scan()end)
    LUI:Button(p,"Export character",181,-43,165,function()M:Character()end)
    self:Link(p,18,-84,325)
    LUI:Button(p,"Resend export",14,-116,150,function()M:Resend()end)
    LUI:Button(p,"Settings",181,-116,165,function()LUI:OpenUI();LUI:SelectPage("modules");LUI:SelectModule("market")end)
    local label=LUI:Description(p,self.status,16,-155,330);self.labels=self.labels or {};self.labels[#self.labels+1]=label
    local bar=CreateFrame("StatusBar",nil,p);bar:SetSize(330,6);bar:SetPoint("BOTTOMLEFT",16,20);bar:SetStatusBarTexture("Interface\\Buttons\\WHITE8X8");bar:SetStatusBarColor(.25,.8,.6,1);bar:SetMinMaxValues(0,100);bar:SetValue(0)
    self.progress=bar
end
function M:Enabled()return self.ready and LUI:DB().marketEnabled end
function M:Identity()
    local realm=plain(call(GetNormalizedRealmName),"string")or plain(call(GetRealmName),"string")
    local faction=plain(call(UnitFactionGroup,"player"),"string")
    local region=({[1]="us",[2]="kr",[3]="eu",[4]="tw"})[plain(call(GetCurrentRegion),"number")or 0]
    if not realm or not faction or not region then return nil end
    -- Neutral auction houses must be selected explicitly; never mix faction books.
    if LUI:DB().marketNeutral then faction="Neutral"end
    return {client=LUI.ForeverEngine and "forever"or "retail",region=region,realm=realm,faction=faction}
end
function M:Event(kind,at)
    local market=self:Identity();if not market then self:Status("Waiting for character/realm identity.");return end
    self.serial=self.serial+1;self.session=self.session or tostring(GetServerTime()).."_"..math.floor(GetTime()*1000)
    return {version=1,id=self.session.."_"..self.serial,market=market,at=at or GetServerTime(),kind=kind}
end
function M:Record(event)
    if not event then return end
    local raw=json(event)
    if #raw>262144 then self:Status("Export exceeds the event limit; no data was truncated.");return end
    local data=raw;local version=1
    if LUI.MarketCodec then
        local compressed=LUI.MarketCodec:CompressDeflate(raw,{level=1})
        if compressed and #compressed<#raw then data=compressed;version=2 end
    end
    return {id=event.id,data=data,version=version,decoded=#raw}
end
-- Expand only Market's strip; the shared core and other strips keep their geometry.
function M:TransferBridge()
    self.bridge=self.bridge or LUI.LookupEvents:GetStrip("market")
    local w,h=GetPhysicalScreenSize()
    local width,height=BULK_WIDTH,BULK_HEIGHT
    if w<width or h<height then width,height=256,32 end
    if self.packets then
        if self.transferBulk and width~=BULK_WIDTH then self.shownAt=nil;self:Status("Transfer paused. Enlarge the game window to show the transfer panel.");return end
        if not self.transferBulk then width,height=256,32 end
    end
    local b=self.bridge
    if b.width~=width or b.height~=height then
        b.width,b.height=width,height;b.frame:SetSize(width,height)
        b.cells=b.cells or {};b.marketCells=b.marketCells or {}
        for i=1,width*height/4 do
            local cell=b.marketCells[i]or b.cells[i]or b.frame:CreateTexture(nil,"OVERLAY")
            b.cells[i]=cell;b.marketCells[i]=cell;cell:SetSize(2,2);cell:ClearAllPoints()
            cell:SetPoint("TOPLEFT",b.frame,"TOPLEFT",((i-1)%(width/2))*2,-math.floor((i-1)/(width/2))*2);cell:Show()
        end
        for i=#b.cells,width*height/4+1,-1 do b.cells[i]:Hide();b.cells[i]=nil end
        b.rendered=false
    end
    return b,width==BULK_WIDTH
end
local function bytes(value,count)
    local out={};for i=count-1,0,-1 do out[#out+1]=string.char(math.floor(value/256^i)%256)end
    return table.concat(out)
end
function M:TransferPacket(record,index,count,bulk)
    local chunk=bulk and BULK_CHUNK or 5600
    local version=(record.version or 1)+(bulk and 2 or 0)
    local header={version,record.id,index,count,#record.data}
    if record.version==2 then header[#header+1]=record.decoded end
    local body=table.concat(header,"|").."\n"..record.data:sub((index-1)*chunk+1,index*chunk)
    if not bulk then return LUI.LookupEvents:Packet(body,"LMK1")end
    local capacity=BULK_WIDTH*BULK_HEIGHT/4*3-10
    assert(#body<=capacity,"Market transfer frame exceeds capacity")
    return "LMK2"..bytes(#body,2)..bytes(LUI.LookupEvents:Checksum(body),4)..body..string.rep("\0",capacity-#body)
end
function M:TransferProgress()
    local total,before=0,0
    for i,record in ipairs(self.queue)do
        local frames=math.ceil(#record.data/self.transferChunk)*PASSES
        total=total+frames;if i<(self.queueIndex or 1)then before=before+frames end
    end
    local done=before+(self.passes or 0)*#self.packets+(self.packetIndex or 1)-1
    local remaining=math.ceil((total-done)*FRAME_SECONDS)
    return math.floor(100*done/math.max(1,total)),math.floor(remaining/60),remaining%60
end
function M:AddBatch(records)
    if self.queueIndex and self.queueIndex>#self.queue then self.lastExport=self.queue;self.queue={};self.queueBytes=0;self.queueIndex=nil end
    local bytes=0;for _,record in ipairs(records)do bytes=bytes+#record.data end
    if #self.queue+#records>4096 or (self.queueBytes or 0)+bytes>32*1024*1024 then self:Status("Export queue is full. Finish the pending transfer, then retry.");return false end
    for _,record in ipairs(records)do self.queue[#self.queue+1]=record end
    self.queueBytes=(self.queueBytes or 0)+bytes;return true
end
function M:Enqueue(event)
    local record=self:Record(event);if not record then return false end
    return self:AddBatch({record})
end
-- Preserve real suffix/bonus variants; plain materials share their numeric item ID.
function M:ItemKey(link,fallback)
    local pet=plain(link,"string")and link:match("battlepet:([^|]+)")
    if pet then local fields={};for value in (pet..":"):gmatch("(.-):")do fields[#fields+1]=value;if #fields==6 then break end end;if #fields==6 then return "pet:"..table.concat(fields,":")end end
    local body=plain(link,"string")and link:match("item:([^|]+)")
    if not body then return fallback and tostring(fallback)end
    local fields={};for s in (body..":"):gmatch("(.-):")do fields[#fields+1]=s end
    local id=tonumber(fields[1]);if not id then return end
    -- 3524 is the generic recipe-preview bonus, not a distinct output variant.
    if tonumber(fields[13])==1 and tonumber(fields[14])==3524 then fields[13]="0";table.remove(fields,14)end
    if (tonumber(fields[7])or 0)~=0 or (tonumber(fields[13])or 0)~=0 then
        -- Strip player-dependent link level/spec and unique auction instance.
        fields[8]="0";fields[9]="0";fields[10]="0"
        return "item:"..table.concat(fields,":")
    end
    return tostring(id)
end
function M:Item(link,id,name,refreshing)
    local key=self:ItemKey(link,id);if not key then return end
    local numeric=id or tonumber(key)or (plain(link,"string")and tonumber(link:match("item:(%d+)")))
    -- GetItemInfo itself requests uncached data. Avoid thousands of implicit
    -- requests while walking a catalog; the bounded loader owns those requests.
    local cached=not(C_Item and C_Item.IsItemDataCachedByID)or call(C_Item.IsItemDataCachedByID,numeric)==true
    local info={}
    if cached then info={call(C_Item and C_Item.GetItemInfo or GetItemInfo,link or id)};if not info[1]and GetItemInfo then info={call(GetItemInfo,link or id)}end end
    name=plain(name,"string")
    if name==key or name==tostring(numeric)then name=nil end
    local function usable(value)value=plain(value,"string");if value and value~=key and value~=tostring(numeric)then return value end end
    local remembered=self.itemNameCache and self.itemNameCache[key]
    name=usable(name)or usable(info[1])or usable(plain(link,"string")and link:match("|h%[([^]]+)%]|h"))or(cached and usable(call(C_Item and C_Item.GetItemNameByID,numeric)))or(remembered and remembered.name)
    -- Bind status is explicit; unknown items cannot be assumed transferable.
    local bind=plain(info[14],"number")
    local item={id=key,name=name or key,transferable=bind==0 or bind==2 or bind==3 or key:sub(1,4)=="pet:",bindingKnown=bind~=nil or key:sub(1,4)=="pet:"}
    if not item.bindingKnown and remembered and remembered.bindingKnown then item.bindingKnown=true;item.transferable=remembered.transferable end
    -- Instant metadata does not request uncached item data from the server.
    local instantAPI=C_Item and C_Item.GetItemInfoInstant or GetItemInfoInstant
    local instant={call(instantAPI,link or numeric)}
    local function categoryID(value)value=plain(value,"number");if value and value>=0 and value<=255 and value==math.floor(value)then return value end end
    item.classID=categoryID(info[12])or categoryID(instant[6])
    if item.classID then
        item.className=plain(info[6],"string")or plain(instant[2],"string")or plain(call(C_Item and C_Item.GetItemClassInfo or GetItemClassInfo,item.classID),"string")
        item.subclassID=categoryID(info[13])or categoryID(instant[7])
        if item.subclassID then item.subclassName=plain(info[7],"string")or plain(instant[3],"string")or plain(call(C_Item and C_Item.GetItemSubClassInfo or GetItemSubClassInfo,item.classID,item.subclassID),"string")end
    elseif remembered then
        item.classID,item.subclassID,item.className,item.subclassName=remembered.classID,remembered.subclassID,remembered.className,remembered.subclassName
    end
    if numeric and not refreshing and(not name or(type(instantAPI)=="function"and item.classID==nil))then
        self:QueueItemName(key,{id=numeric,link=link,category=type(instantAPI)=="function"})
    end
    if name then self.itemNameCache=self.itemNameCache or {};self.itemNameCache[key]=item end
    return item,plain(info[11],"number")
end
function M:QueueItemName(key,ref)
    self.uncachedItems=self.uncachedItems or {}
    if not self.uncachedItems[key]then
        self.uncachedItems[key]=ref;self.nameQueue=self.nameQueue or {};self.nameQueue[#self.nameQueue+1]=key
        self.nameKeysByID=self.nameKeysByID or {};self.nameKeysByID[ref.id]=self.nameKeysByID[ref.id]or {};self.nameKeysByID[ref.id][key]=true
    end
    self.itemNamesAt=self.itemNamesAt or GetTime()+.2
end
-- The game cache can evict a loaded item before a large round-robin returns to
-- it. Read just-loaded IDs first, while retaining a bounded amount of work/tick.
function M:ItemDataLoaded(id)
    local request=self.itemDataRequested and self.itemDataRequested[id]
    if request and request.pending then request.pending=false;self.nameLoadsInFlight=math.max(0,(self.nameLoadsInFlight or 0)-1)end
    local keys=self.nameKeysByID and self.nameKeysByID[id];if not keys then return end
    self.loadedNameQueue=self.loadedNameQueue or {};self.loadedNameSet=self.loadedNameSet or {}
    for key in pairs(keys)do if not self.loadedNameSet[key]then self.loadedNameSet[key]=true;self.loadedNameQueue[#self.loadedNameQueue+1]=key end end
    self.itemNamesAt=GetTime()
end
function M:FlushItemNames()
    if not self.metadataItems or #self.metadataItems==0 then return true end
    local e=self:Event("items");if not e then return false end
    e.items=self.metadataItems
    if not self:Enqueue(e)then return false end
    self.metadataItems=arr();self.metadataBytes=0;return true
end
function M:RefreshItemNames()
    if not self.uncachedItems or not self:Enabled()then return end
    self.metadataItems=self.metadataItems or arr();local checked,requests=0,0;local now=GetTime()
    if not self.nameQueue then self.nameQueue={};for key in pairs(self.uncachedItems)do self.nameQueue[#self.nameQueue+1]=key end end
    self.nameQueueHead=self.nameQueueHead or 1;local stop=#self.nameQueue
    self.itemDataRequested=self.itemDataRequested or {}
    self.loadedNameHead=self.loadedNameHead or 1
    while checked<100 do
        local key,urgent
        if self.loadedNameQueue and self.loadedNameHead<=#self.loadedNameQueue then
            key=self.loadedNameQueue[self.loadedNameHead];self.loadedNameHead=self.loadedNameHead+1;self.loadedNameSet[key]=nil;urgent=true
        elseif self.nameQueueHead<=stop then key=self.nameQueue[self.nameQueueHead];self.nameQueueHead=self.nameQueueHead+1
        else break end
        local ref=self.uncachedItems[key]
        if ref then
        local item=self:Item(ref.link,ref.id,nil,true)
        if item and item.name~=item.id and(not ref.category or item.classID~=nil)then
            item.id=key;local size=#json(item)+1
            if (self.metadataBytes or 0)+size>180000 and not self:FlushItemNames()then self.nameQueue[#self.nameQueue+1]=key;self.itemNamesAt=GetTime()+.2;return end
            self.metadataItems[#self.metadataItems+1]=item;self.metadataBytes=(self.metadataBytes or 0)+size;self.uncachedItems[key]=nil
            if self.nameKeysByID and self.nameKeysByID[ref.id]then self.nameKeysByID[ref.id][key]=nil end
        else
            local request=self.itemDataRequested[ref.id]
            if request and request.pending and now-request.at>=15 then request.pending=false;self.nameLoadsInFlight=math.max(0,(self.nameLoadsInFlight or 0)-1)end
            if request and request.tries>=12 and now-request.at>=15 then self.uncachedItems[key]=nil
            else
            if (not request or now-request.at>=15)and requests<20 and(self.nameLoadsInFlight or 0)<32 then
                request={at=now,tries=(request and request.tries or 0)+1,pending=true};self.itemDataRequested[ref.id]=request;self.nameLoadsInFlight=(self.nameLoadsInFlight or 0)+1
                if C_Item and C_Item.RequestLoadItemDataByID then call(C_Item.RequestLoadItemDataByID,ref.id)else call(GetItemInfo,ref.link or ref.id)end;requests=requests+1
            end
            if not urgent then self.nameQueue[#self.nameQueue+1]=key end
            end
        end
        checked=checked+1
        end
    end
    if self.loadedNameQueue and self.loadedNameHead>#self.loadedNameQueue then self.loadedNameQueue=nil;self.loadedNameSet=nil;self.loadedNameHead=nil end
    if self.nameQueueHead>4096 then local queue={};for i=self.nameQueueHead,#self.nameQueue do queue[#queue+1]=self.nameQueue[i]end;self.nameQueue=queue;self.nameQueueHead=1 end
    if #self.metadataItems>=1500 or not next(self.uncachedItems)then self:FlushItemNames()end
    if next(self.uncachedItems)or #self.metadataItems>0 then self.itemNamesAt=GetTime()+.2 end
end
function M:Scan()
    if not self:Enabled()then self:Status("Enable LamdaMarket first.");return end
    if not self.auctionOpen then self:Status("Open an auction house first.");return end
    if self.failed then
        -- A manual retry replaces failed jobs; complete queued events remain intact.
        self.failed=nil;self.pending=nil;self.job=nil;self.recipeJob=nil;self.depthJob=nil;self.readAt=nil;self.browseMoreAt=nil
    end
    if self.pending or self.job or self.depthJob then self:Status("A scan is already running.");return end
    if #self.queue>0 then
        if (self.queueIndex or 1)<=#self.queue then self:Status("Finish or cancel the pending export before another scan.");return end
        self:Clear()
    end
    local modern=C_AuctionHouse
    -- Forever exposes replication even when its server never returns a snapshot.
    -- Browse pages supply indicative prices; only search results supply buyable lots.
    if LUI.ForeverEngine and modern and type(modern.SendBrowseQuery)=="function"then
        self:StartBrowse();return
    elseif modern and type(modern.ReplicateItems)=="function"then
        if modern.IsThrottledMessageSystemReady and not call(modern.IsThrottledMessageSystemReady)then self:Status("Auction queries are throttled. Try again when ready.");return end
        self.readAttempts=0;self.pending="replicate";self.requestAt=GetTime();local ok,err=pcall(modern.ReplicateItems)
        if not ok then self.pending=nil;self:Status("Scan request unavailable: "..tostring(err));return end
    elseif type(QueryAuctionItems)=="function"then
        local canQuery,canAll=call(CanSendAuctionQuery)
        if not canQuery or not canAll then self:Status("The server is not ready for a full scan. Try again later.");return end
        self.readAttempts=0;self.pending="legacy";self.requestAt=GetTime();local ok,err=pcall(QueryAuctionItems,"",nil,nil,0,false,nil,true,false,nil)
        if not ok then self.pending=nil;self:Status("Scan request unavailable: "..tostring(err));return end
    else self:Status("This client does not expose a full-scan API.");return end
    self:Status("Scan requested. Waiting for the auction server…")
end
function M:HookAuctionQueries()
    if not hooksecurefunc or not C_AuctionHouse then return end
    self.queryHooks=self.queryHooks or {}
    for _,name in ipairs({"SendBrowseQuery","SearchForItemKeys","SearchForFavorites","SendSearchQuery","SendSellSearchQuery"})do
        if not self.queryHooks[name]and type(C_AuctionHouse[name])=="function"then
            local ok=pcall(hooksecurefunc,C_AuctionHouse,name,function()
                if M.pending=="browse"and not M.ownBrowseQuery then
                    M.pending=nil;M.browseMoreAt=nil
                    M:Status("Auction search changed; scan cancelled. Choose Scan auctions to start again.")
                end
            end)
            self.queryHooks[name]=ok
        end
    end
end
function M:StartBrowse()
    local api=C_AuctionHouse
    if api.IsThrottledMessageSystemReady and not call(api.IsThrottledMessageSystemReady)then self:Status("Auction queries are throttled. Try again when ready.");return end
    self:HookAuctionQueries();self.pending="browse";self.browseCount=0;self.browseStalls=0;self.browseMoreAt=nil;self.requestAt=GetTime()
    self.ownBrowseQuery=true
    local ok,err=pcall(api.SendBrowseQuery,{searchString="",sorts={},filters={},itemClassFilters={}})
    self.ownBrowseQuery=nil
    if not ok then self.pending=nil;self:Status("Browse request unavailable: "..tostring(err));return end
    self:Status("Requesting auction browse pages…")
end
function M:QueueAuctions(lots,items,coverage,stamp,market,retain)
    local pages={};local page={lots=arr(),items=arr(),seen={},bytes=0}
    for index,lot in ipairs(lots)do
        local needed=#json(lot)+1
        if not page.seen[lot.item]then needed=needed+#json(items[lot.item])+1 end
        if page.bytes+needed>180000 and #page.lots>0 then pages[#pages+1]=page;page={lots=arr(),items=arr(),seen={},bytes=0};needed=#json(lot)+#json(items[lot.item])+2 end
        page.lots[#page.lots+1]=lot;page.bytes=page.bytes+needed
        if not page.seen[lot.item]then page.seen[lot.item]=true;page.items[#page.items+1]=items[lot.item]end
        if index%200==0 then coroutine.yield()end
    end
    if #page.lots>0 or #pages==0 then pages[#pages+1]=page end
    local scan=self:Event("auction",stamp);if not scan then return end
    local records={}
    for index,data in ipairs(pages)do
        self:Status("Preparing export page: "..index.." / "..#pages)
        local e=self:Event("auction",stamp);e.market=market;e.scan=scan.id;e.page=index;e.pages=#pages;e.coverage=coverage;e.lots=data.lots;e.items=data.items
        local record=self:Record(e);if not record then return end;records[#records+1]=record;coroutine.yield()
    end
    if not self:AddBatch(records)then return end
    if retain then self.auctionExport=records end
    self:Status(#lots.." price levels queued ("..coverage.."). Keep the color strip visible.")
end
function M:BrowseResults()
    if self.pending~="browse"or not self.auctionOpen then return end
    local rows=call(C_AuctionHouse.GetBrowseResults)
    if type(rows)~="table"then return end
    local count=#rows
    if count>200000 then self.pending=nil;self:Status("Browse scan exceeds 200000 item keys; nothing exported.");return end
    if not call(C_AuctionHouse.HasFullBrowseResults)then
        self.browseStalls=count<=(self.browseCount or 0)and(self.browseStalls or 0)+1 or 0
        if self.browseStalls>3 then self.pending=nil;self:Status("Auction paging stopped advancing. Close and reopen the auction house, then retry.");return end
        self.browseCount=count;self.requestAt=GetTime();self.browseMoreAt=GetTime()+.5
        self:Status("Reading auction catalog: "..count.." item keys; waiting for the next page…");return
    end
    self.pending=nil;self.browseMoreAt=nil
    local stamp=GetServerTime();local market=self:Identity();if not market then return end
    self.job=coroutine.create(function()
        local lots,items={},{}
        for index,row in ipairs(rows)do
            if not self.auctionOpen then self:Status("Auction house closed; scan cancelled.");return end
            local key=row.itemKey;local id=type(key)=="table"and plain(key.itemID,"number")
            local price,qty=plain(row.minPrice,"number"),plain(row.totalQuantity,"number")
            if id and price and price>0 and price<=1000000000000 and qty and qty>0 and qty<=10000000 then
                local info=call(C_AuctionHouse.GetItemKeyInfo,key)or {}
                local link=plain(row.appearanceLink,"string")or plain(info.appearanceLink,"string")or plain(call(C_AuctionHouse.GetItemKeySampleLink,key),"string")
                local item=self:Item(link or("item:"..id),id,info.itemName)
                if item then
                    if not link and((plain(key.itemSuffix,"number")or 0)~=0 or(plain(key.battlePetSpeciesID,"number")or 0)~=0 or info.isEquipment)then
                        local unresolved=self.uncachedItems and self.uncachedItems[item.id]
                        item.id=string.format("browse:%d:%d:%d:%d",id,plain(key.itemLevel,"number")or 0,plain(key.itemSuffix,"number")or 0,plain(key.battlePetSpeciesID,"number")or 0)
                        if unresolved then self:QueueItemName(item.id,{id=unresolved.id,link=unresolved.link});item.name=item.id end
                    end
                    items[item.id]=item
                    -- Total listed supply is not the quantity offered at the minimum.
                    -- Existing summary coverage excludes these rows from acquisitions.
                    lots[#lots+1]={item=item.id,price=price,quantity=1,count=qty,divisible=false}
                end
            end
            if index%200==0 then coroutine.yield()end
        end
        self:QueueAuctions(lots,items,"summary",stamp,market,true)
    end)
end
function M:SearchResults(key,commodity)
    if not self:Enabled()or not self.auctionOpen or self.pending or self.job then return end
    local api=C_AuctionHouse;local id=commodity and plain(key,"number")or type(key)=="table"and plain(key.itemID,"number")
    if not id then return end
    local count=plain(call(commodity and api.GetNumCommoditySearchResults or api.GetNumItemSearchResults,key),"number")
    if not count or count<1 or count>200000 then return end
    local stamp=GetServerTime();local market=self:Identity();if not market then return end
    self.depthJob=coroutine.create(function()
        local lots,items={},{}
        for index=1,count do
            if not self.auctionOpen then return end
            local row=call(commodity and api.GetCommoditySearchResultInfo or api.GetItemSearchResultInfo,key,index)
            if type(row)=="table"then
                local price=plain(commodity and row.unitPrice or row.buyoutAmount,"number");local qty=plain(row.quantity,"number")
                if price and price>0 and price<=1000000000000 and qty and qty>0 and qty<=10000000 then
                    local link=plain(row.itemLink,"string")
                    if commodity or link then
                        local item=self:Item(link or("item:"..id),id)
                        if item then items[item.id]=item;lots[#lots+1]={item=item.id,price=price,quantity=commodity and 1 or qty,count=commodity and qty or 1,divisible=commodity==true}end
                    end
                end
            end
            if index%200==0 then coroutine.yield()end
        end
        if #lots>0 then self:QueueAuctions(lots,items,"partial",stamp,market,false)end
    end)
end
function M:ReadAuctions(mode)
    if not self.pending or self.pending~=mode or not self.auctionOpen then return end
    self.pending=nil
    local modern=mode=="replicate";local count,total
    if modern then count=plain(call(C_AuctionHouse.GetNumReplicateItems),"number");total=count
    else count,total=call(GetNumAuctionItems,"list");count=plain(count,"number");total=plain(total,"number")end
    if not count or count>200000 or count<0 then self:Status("Scan count unavailable or over 200000; nothing exported.");return end
    local stamp=GetServerTime();local market=self:Identity();if not market then return end
    self.job=coroutine.create(function()
        local grouped,items={},{};local missing=0;local requested={}
        for n=1,count do
            if not self.auctionOpen then self:Status("Auction house closed; scan cancelled.");return end
            local index=modern and n-1 or n
            local info=modern and {call(C_AuctionHouse.GetReplicateItemInfo,index)}or {call(GetAuctionItemInfo,"list",index)}
            local link=modern and call(C_AuctionHouse.GetReplicateItemLink,index)or call(GetAuctionItemLink,"list",index)
            local price,qty=plain(info[10],"number"),plain(info[3],"number")
            if info[18]==false then missing=missing+1 end
            if not price then missing=missing+1
            elseif price>0 then
                local item=plain(link,"string")and self:Item(link,plain(info[17],"number"),info[1])
                if item and qty and qty>0 and qty<=10000000 and price<=1000000000000 then
                    items[item.id]=item;local key=item.id.."/"..price.."/"..qty
                    local lot=grouped[key];if lot then lot.count=lot.count+1 else grouped[key]={item=item.id,price=price,quantity=qty,count=1,divisible=false}end
                else
                    missing=missing+1
                    local itemID=plain(info[17],"number")
                    if itemID and not requested[itemID]then requested[itemID]=true;call(C_Item and C_Item.RequestLoadItemDataByID,itemID)end
                end
            end
            if n%200==0 then self:Status("Reading auction rows: "..n.." / "..count);coroutine.yield()end
        end
        if missing>0 and (self.readAttempts or 0)<3 then
            self.readAttempts=(self.readAttempts or 0)+1;self.pending=mode;self.requestAt=GetTime();self.readAt=GetTime()+2
            self:Status("Waiting for auction item data (retry "..self.readAttempts.." / 3)…");return
        end
        local lots={};for _,lot in pairs(grouped)do lots[#lots+1]=lot end
        table.sort(lots,function(a,b)if a.item~=b.item then return a.item<b.item end;if a.price~=b.price then return a.price<b.price end;return a.quantity<b.quantity end)
        self:QueueAuctions(lots,items,(missing==0 and count==total)and "full"or "partial",stamp,market,true)
    end)
end
function M:Professions()
    local out=arr();local indices={call(GetProfessions)}
    for _,i in pairs(indices)do if type(i)=="number"then local name,_,rank,maxRank,_,_,id=call(GetProfessionInfo,i)
        if plain(id,"number")then out[#out+1]={id=tostring(id),name=plain(name,"string")or tostring(id),rank=plain(rank,"number")or 0,max=plain(maxRank,"number")or 0}end
    end end
    return out
end
function M:Character()
    if not self:Enabled()then return end
    local id=plain(call(UnitGUID,"player"),"string");if not id then return end
    LamdaMarketDB=type(LamdaMarketDB)=="table"and LamdaMarketDB or {};LamdaMarketDB.known=LamdaMarketDB.known or {}
    local e=self:Event("character");if not e then return end
    local name,surname=call(UnitFullName,"player")
    local given,family=call(UnitName,"player")
    name=plain(given,"string")or plain(name,"string")
    -- Forever's second UnitName value is a surname, not an economy/realm ID.
    surname=plain(family,"string")or plain(surname,"string")
    if not LUI.ForeverEngine then surname=surname or plain(call(GetRealmName),"string")end
    local _,class=call(UnitClass,"player")
    local c={id=id,name=plain(name,"string")or id,surname=surname,class=plain(class,"string"),professions=self:Professions(),known=arr(),knownByProfession={},recipeTimes={},inventory={},at=e.at,recipesObserved=next(LamdaMarketDB.known[id]or {})~=nil}
    for pid,known in pairs(LamdaMarketDB.known[id]or {})do c.knownByProfession[pid]=arr(known);c.recipeTimes[pid]=LamdaMarketDB.recipeTimes and LamdaMarketDB.recipeTimes[id]and LamdaMarketDB.recipeTimes[id][pid]or 0;for _,recipe in ipairs(known)do c.known[#c.known+1]=recipe end end
    local container=C_Container or _G
    for bag=0,(NUM_TOTAL_EQUIPPED_BAG_SLOTS or NUM_BAG_SLOTS or 4)do
        local count=plain(call(container.GetContainerNumSlots,bag),"number")or 0
        for slot=1,count do
            local info=call(container.GetContainerItemInfo,bag,slot);local link,qty
            if type(info)=="table"then link=info.hyperlink;qty=plain(info.stackCount,"number")else local _,q,_,_,_,_,l=call(container.GetContainerItemInfo,bag,slot);link=l;qty=plain(q,"number")end
            local key=self:ItemKey(link);if key and qty then c.inventory[key]=(c.inventory[key]or 0)+qty end
        end
    end
    e.character=c;self:Enqueue(e)
end
function M:Merchant()
    if not self:Enabled()then return end
    local count=plain(call(GetMerchantNumItems),"number");if not count then return end
    local e=self:Event("vendor");if not e then return end;e.items=arr();e.vendors=arr()
    local guid=plain(call(UnitGUID,"npc"),"string")or "unknown";local name=plain(call(UnitName,"npc"),"string")or "Merchant"
    local map=call(C_Map and C_Map.GetBestMapForUnit,"player");local position=map and call(C_Map.GetPlayerMapPosition,map,"player");local location=tostring(map or "unknown")
    if position then local x,y=call(position.GetXY,position);if x and y then location=location..string.format(" %.1f, %.1f",x*100,y*100)end end
    for index=1,math.min(count,1000)do
        local v=call(C_MerchantFrame and C_MerchantFrame.GetItemInfo,index)
        if not v then local n,_,price,bundle,stock,purchasable,usable,extended=call(GetMerchantItemInfo,index);v={name=n,price=price,stackCount=bundle,numAvailable=stock,isPurchasable=purchasable,isUsable=usable,hasExtendedCost=extended}end
        local link=call(GetMerchantItemLink,index);local item,sell=self:Item(link,nil,v.name)
        local price,bundle,stock=plain(v.price,"number"),plain(v.stackCount,"number"),plain(v.numAvailable,"number")
        if item and price and bundle and bundle>0 and stock then
            e.items[#e.items+1]=item;local requirement=""
            if v.hasExtendedCost then requirement="Extended currency/item cost (not priced)"elseif v.isPurchasable==false then requirement="Merchant purchase requirement not met"end
            e.vendors[#e.vendors+1]={id=guid.."/"..index,item=item.id,name=name,buy=price,sell=sell or 0,bundle=bundle,stock=stock,requirement=requirement,location=location}
        end
    end
    self:Enqueue(e)
end
function M:Recipes(force)
    if not self:Enabled()or self.recipeJob then return end
    if not self.recipeOpen then self:Status("Open a profession window first.");return end
    local guid=plain(call(UnitGUID,"player"),"string");if not guid then return end
    local modern=C_TradeSkillUI and type(C_TradeSkillUI.GetAllRecipeIDs)=="function"
    if modern and (call(C_TradeSkillUI.IsTradeSkillLinked)or call(C_TradeSkillUI.IsTradeSkillGuild)or call(C_TradeSkillUI.IsNPCCrafting))then return end
    if not modern and call(IsTradeSkillLinked)then return end
    local professions=self:Professions();local pid,pname
    if modern then local p=call(C_TradeSkillUI.GetBaseProfessionInfo);if type(p)=="table"then pid=plain(p.professionID,"number");pname=p.professionName end
    else pname=call(GetTradeSkillLine)end
    for _,p in ipairs(professions)do if p.name==pname then pid=tonumber(p.id)end end
    if not pid then self:Status("Profession identity unavailable; recipe export skipped.");return end
    local ids=modern and call(C_TradeSkillUI.GetAllRecipeIDs)or nil
    if modern and type(ids)~="table"then return end
    if modern and #ids==0 then self.recipeAt=GetTime()+1;self:Status("Waiting for profession data…");return end
    force=force or self.forceRecipes;self.forceRecipes=nil
    self.recipeJob=coroutine.create(function()
        local known=arr();local recipes=arr();local items=arr();local batchBytes=0;local count=modern and #ids or (plain(call(GetNumTradeSkills),"number")or 0)
        self.recipeSnapshots=self.recipeSnapshots or {};local previous=not force and self.recipeSnapshots[pid]or {};local signatures={};local changed,allSent=false,true
        local function flush()if #recipes>0 then
            local signature=json({recipes=recipes,items=items});signatures[#signatures+1]=signature
            if previous[#signatures]~=signature then local e=self:Event("recipes");e.recipes=recipes;e.items=items;if self:Enqueue(e)then changed=true else allSent=false end end
            recipes=arr();items=arr();batchBytes=0
        end end
        for index=1,count do
            local recipeID,recipeName,learned,output,inputs,minYield,cooldown,requirement
            local rowItems=arr();inputs=arr();minYield=1;requirement="";cooldown=""
            if modern then
                recipeID=ids[index];local info=call(C_TradeSkillUI.GetRecipeInfo,recipeID)
                if type(info)=="table"then
                    recipeName=info.name;learned=info.learned==true
                    local link=call(C_TradeSkillUI.GetRecipeItemLink,recipeID);output=self:Item(link)
                    local schematic=call(C_TradeSkillUI.GetRecipeSchematic,recipeID,false)
                    if type(schematic)=="table"then
                        minYield=plain(schematic.quantityMin,"number")or 1
                        output=output or (schematic.outputItemID and self:Item(nil,schematic.outputItemID))
                        for _,slot in ipairs(schematic.reagentSlotSchematics or {})do
                            if slot.required then
                                local reagents=slot.reagents or {}
                                if #reagents~=1 or not reagents[1].itemID then requirement="Selectable or quality reagent; explicit recipe model needed"
                                else local item=self:Item(nil,reagents[1].itemID);if item then rowItems[#rowItems+1]=item;inputs[#inputs+1]={item=item.id,quantity=slot.quantityRequired}end end
                            end
                        end
                        if schematic.hasCraftingOperationInfo then requirement="Quality/crafting operation requires an explicit recipe model"end
                    else requirement="Recipe schematic unavailable"end
                    local remaining,day=call(C_TradeSkillUI.GetRecipeCooldown,recipeID);remaining=plain(remaining,"number");if (remaining and remaining>0)or day==true then cooldown="Observed cooldown"end
                end
            else
                local difficulty;recipeName,difficulty=call(GetTradeSkillInfo,index)
                if difficulty~="header"and recipeName then
                    learned=true;local recipeLink=call(GetTradeSkillRecipeLink,index);recipeID=type(recipeLink)=="string"and (recipeLink:match("enchant:(%d+)")or recipeLink:match("spell:(%d+)"))
                    output=self:Item(call(GetTradeSkillItemLink,index));minYield=plain(call(GetTradeSkillNumMade,index),"number")or 1
                    for reagent=1,(plain(call(GetTradeSkillNumReagents,index),"number")or 0)do local reagentName,_,qty=call(GetTradeSkillReagentInfo,index,reagent);local item=self:Item(call(GetTradeSkillReagentItemLink,index,reagent),nil,reagentName);if item then rowItems[#rowItems+1]=item;inputs[#inputs+1]={item=item.id,quantity=qty}else requirement="Reagent identity missing"end end
                    local remaining,day=call(GetTradeSkillCooldown,index);remaining=plain(remaining,"number");if (remaining and remaining>0)or day==true then cooldown="Observed cooldown"end
                end
            end
            if recipeID then
                if learned then known[#known+1]=tostring(recipeID)end
                if output and #inputs>0 then
                    rowItems[#rowItems+1]=output
                    local recipe={id=tostring(recipeID),name=recipeName or tostring(recipeID),profession=tostring(pid),rank=0,inputs=inputs,output={item=output.id,quantity=minYield},cooldown=cooldown,requirement=requirement,source="Observed profession window; minimum yield; rank requirement unknown"}
                    local size=#json(recipe)+#json(rowItems)+2
                    if batchBytes+size>180000 or #recipes>=500 or #items+#rowItems>4000 then flush()end
                    for _,item in ipairs(rowItems)do items[#items+1]=item end
                    recipes[#recipes+1]=recipe;batchBytes=batchBytes+size
                end
            end
            if index%50==0 then coroutine.yield()end
        end
        flush();LamdaMarketDB=type(LamdaMarketDB)=="table"and LamdaMarketDB or {};LamdaMarketDB.known=LamdaMarketDB.known or {};LamdaMarketDB.known[guid]=LamdaMarketDB.known[guid]or {};LamdaMarketDB.known[guid][tostring(pid)]=known
        LamdaMarketDB.recipeTimes=LamdaMarketDB.recipeTimes or {};LamdaMarketDB.recipeTimes[guid]=LamdaMarketDB.recipeTimes[guid]or {};LamdaMarketDB.recipeTimes[guid][tostring(pid)]=GetServerTime()
        signatures.known=json(known);if allSent then self.recipeSnapshots[pid]=signatures end
        if not allSent then self:Status("Export queue is full. Clear it or finish sending, then export the profession again.")
        elseif changed or previous.known~=signatures.known then self:Character();self:Status("Profession exported. Only simple, fully priced recipes enter automatic estimates.")
        else self:Status("Profession data is already up to date.")end
    end)
end
function M:TransferFinished()
    if self.pending or self.job or self.depthJob or self.recipeJob or next(self.uncachedItems or {})then
        if self.progress then self.progress:Hide()end
        if self.pending=="browse"then self:Status("Reading auction catalog: "..(self.browseCount or 0).." item keys; waiting for the server…")
        elseif self.pending then self:Status("Waiting for the auction server…")
        elseif self.job or self.depthJob then self:Status("Preparing auction export…")
        elseif self.recipeJob then self:Status("Reading profession recipes…")
        else self.nameStatusAt=0;self:Status("Loading item names. Keep WoW visible.")end
    else
        if self.progress then self.progress:SetValue(100)end
        self:Status("Export passes finished. Check the browser for received pages; use Resend if incomplete.")
    end
end
function M:Tick()
    if self.failed or not self:Enabled()or LUI.overlayHidden then if self.bridge then self.bridge.frame:Hide()end;self.shownAt=nil;return end
    if self.readAt and GetTime()>=self.readAt then self.readAt=nil;self:ReadAuctions(self.pending)end
    if self.pending=="browse"and self.browseMoreAt and GetTime()>=self.browseMoreAt then
        local api=C_AuctionHouse
        if not api.IsThrottledMessageSystemReady or call(api.IsThrottledMessageSystemReady)then
            self.browseMoreAt=nil;self.requestAt=GetTime()
            local ok,err=pcall(api.RequestMoreBrowseResults)
            if not ok then self.pending=nil;self:Status("Auction paging unavailable: "..tostring(err))end
        end
    end
    if self.pending and GetTime()-(self.requestAt or 0)>90 then self.pending=nil;self:Status("Auction request timed out. Try again when the server permits it.")end
    for _,key in ipairs({"job","recipeJob","depthJob"})do local job=self[key];if job then local ok,err=coroutine.resume(job);if not ok then self[key]=nil;self.failed=true;self:Status("Capture stopped: "..tostring(err));return elseif coroutine.status(job)=="dead"then self[key]=nil end end end
    if self.characterAt and GetTime()>=self.characterAt then self.characterAt=nil;self:Character()end
    if self.recipeAt and GetTime()>=self.recipeAt then self.recipeAt=nil;self:Recipes()end
    if self.merchantAt and GetTime()>=self.merchantAt then self.merchantAt=nil;self:Merchant()end
    if self.itemNamesAt and GetTime()>=self.itemNamesAt then self.itemNamesAt=nil;self:RefreshItemNames()end
    local event=self.queue[self.queueIndex or 1];if not event then
        if self.bridge then self.bridge.frame:Hide()end
        if not self.pending and not self.job and next(self.uncachedItems or {})then
            if not self.nameStatusAt or GetTime()-self.nameStatusAt>=1 then
                self.nameStatusAt=GetTime();local count=0;for _ in pairs(self.uncachedItems)do count=count+1 end
                if self.progress then self.progress:Hide()end
                self:Status("Loading item names: "..count.." remaining. Keep WoW visible.")
            end
        elseif self.nameStatusAt and not self.pending and not self.job and not self.recipeJob then
            self.nameStatusAt=nil;self:Status("Export passes finished. Check the browser for received pages; use Resend if incomplete.")
        end
        return
    end
    if not PixelUtil or not LUI.LookupEvents then return end
    local bridge,bulk=self:TransferBridge();if not bridge then return end;if bridge.moving then self.shownAt=nil;self:Status("Export paused. Lock the LamdaMarket color strip to continue.");return end
    local now=GetTime()
    if not self.packets then
        self.transferBulk=bulk
        self.transferChunk=bulk and BULK_CHUNK or 5600
        self.packets={};local count=math.ceil(#event.data/self.transferChunk)
        for i=1,count do self.packets[i]=assert(self:TransferPacket(event,i,count,bulk))end
        self.packetIndex=1;self.passes=0
    end
    if self.shownAt then
        if now-self.shownAt<FRAME_SECONDS then return end
        self.packetIndex=self.packetIndex+1;if self.packetIndex>#self.packets then self.packetIndex=1;self.passes=self.passes+1 end
        if self.passes>=PASSES then self.queueIndex=(self.queueIndex or 1)+1;self.packets=nil;self.shownAt=nil
            if self.queueIndex>#self.queue then self:TransferFinished()end
            return
        end
    end
    self.bridge:ApplyPosition();self.bridge:Paint(self.packets[self.packetIndex]);self.shownAt=now
    local percent,minutes,seconds=self:TransferProgress()
    if self.progress then self.progress:Show();self.progress:SetValue(percent)end
    self:Status(string.format("Sending %d%% · about %d:%02d left\nExport %d / %d · packet %d / %d · pass %d / 3",percent,minutes,seconds,self.queueIndex or 1,#self.queue,self.packetIndex,#self.packets,self.passes+1))
end
function M:Resend()
    if self.auctionExport and not self.job and not self.recipeJob and not self.pending and (#self.queue==0 or (self.queueIndex or 1)>#self.queue)then
        self.queue=self.auctionExport
    elseif #self.queue==0 and self.lastExport then self.queue=self.lastExport end
    self.queueBytes=0;for _,event in ipairs(self.queue)do self.queueBytes=self.queueBytes+#event.data end
    self.queueIndex=1;self.packets=nil;self.shownAt=nil;self:Status("Replaying the retained export. Duplicate observations are safe.")
end
function M:Clear()
    self.failed=nil;self.lastExport=nil;self.readAt=nil;self.browseMoreAt=nil;self.pending=nil;self.job=nil;self.recipeJob=nil;self.recipeAt=nil;self.forceRecipes=nil;self.depthJob=nil
    self.queue={};self.queueBytes=0;self.queueIndex=nil;self.packets=nil;self.shownAt=nil
    self.uncachedItems=nil;self.nameQueue=nil;self.nameQueueHead=nil;self.metadataItems=nil;self.metadataBytes=0;self.itemNamesAt=nil;self.itemDataRequested=nil;self.nameStatusAt=nil
    self.nameKeysByID=nil;self.loadedNameQueue=nil;self.loadedNameSet=nil;self.loadedNameHead=nil
    self.nameLoadsInFlight=0
    if self.bridge then self.bridge.frame:Hide()end
    if self.progress then self.progress:SetValue(0)end
    self:Status("Export queue cleared. Incomplete scans stay out of price history.")
end

local module={id="market",name="LamdaMarket",enabledKey="marketEnabled",settingsScope="global",enabledHelp="Observe auction listings, merchants and character professions for the local market workspace.",defaults={marketEnabled=true,marketNeutral=false,marketBridgeX=0,marketBridgeY=416},limits={marketBridgeX={0,16000,integer=true},marketBridgeY={0,16000,integer=true}}}
module.build=function(parent)
    LUI:Description(parent,"Market research in Lamda Engine. Scan manually at an auction house; open merchants and professions to add their data. Nothing buys, sells or crafts for you.",4,0,650)
    LUI:Button(parent,"Scan auctions",4,-75,155,function()M:Scan()end)
    LUI:Button(parent,"Export character",175,-75,155,function()M:Character()end)
    LUI:Button(parent,"Export profession",346,-75,160,function()M:Recipes(true)end)
    LUI:Button(parent,"Resend export",4,-125,155,function()M:Resend()end)
    LUI:Button(parent,"Clear queue",175,-125,155,function()M:Clear()end)
    LUI:Checkbox(parent,"Neutral auction house (separate market)","marketNeutral",-185,0)
    LUI:Description(parent,"Forever scans browse pages for price charts. Summary prices do not prove purchase depth; opening an item's auction results captures actual lots. Scans respect server throttling. Retail replication covers its realm, while commodity searches capture observed regional prices. Keep the color strip uncovered; missed pages can be resent.",4,-240,650)
    M.label=LUI:Description(parent,M.status,4,-340,650);M.labels=M.labels or {};M.labels[#M.labels+1]=M.label
    LUI:Button(parent,"Capture panel",4,-385,155,function()M:Panel()end)
    M:Link(parent,180,-385,350)
    if LUI.ColorStripControls then LUI:ColorStripControls(parent,"market",4,-410)end
end
LUI:RegisterModule(module)
local events=CreateFrame("Frame");M.events=events
for _,event in ipairs({"PLAYER_LOGIN","PLAYER_ENTERING_WORLD","SKILL_LINES_CHANGED","AUCTION_HOUSE_SHOW","AUCTION_HOUSE_CLOSED","AUCTION_ITEM_LIST_UPDATE","REPLICATE_ITEM_LIST_UPDATE","AUCTION_HOUSE_BROWSE_RESULTS_UPDATED","AUCTION_HOUSE_BROWSE_RESULTS_ADDED","COMMODITY_SEARCH_RESULTS_UPDATED","COMMODITY_SEARCH_RESULTS_ADDED","ITEM_SEARCH_RESULTS_UPDATED","ITEM_SEARCH_RESULTS_ADDED","ITEM_DATA_LOAD_RESULT","GET_ITEM_INFO_RECEIVED","MERCHANT_SHOW","MERCHANT_UPDATE","MERCHANT_CLOSED","TRADE_SKILL_SHOW","TRADE_SKILL_LIST_UPDATE","TRADE_SKILL_DATA_SOURCE_CHANGED","TRADE_SKILL_CLOSE"})do pcall(events.RegisterEvent,events,event)end
events:SetScript("OnEvent",function(_,event,key)
    if event=="PLAYER_LOGIN"then M.ready=true;M:Character()
    elseif event=="PLAYER_ENTERING_WORLD"or event=="SKILL_LINES_CHANGED"then M.characterAt=GetTime()+1
    elseif event=="AUCTION_HOUSE_SHOW"then M.auctionOpen=true;M:HookAuctionQueries();if M:Enabled()then M:Panel()end
    elseif event=="AUCTION_HOUSE_CLOSED"then M.auctionOpen=false;M.pending=nil;M.browseMoreAt=nil;M.depthJob=nil
    elseif event=="REPLICATE_ITEM_LIST_UPDATE"then M:ReadAuctions("replicate")
    elseif event=="AUCTION_ITEM_LIST_UPDATE"then M:ReadAuctions("legacy")
    elseif event=="AUCTION_HOUSE_BROWSE_RESULTS_UPDATED"or event=="AUCTION_HOUSE_BROWSE_RESULTS_ADDED"then M:BrowseResults()
    elseif event=="COMMODITY_SEARCH_RESULTS_UPDATED"or event=="COMMODITY_SEARCH_RESULTS_ADDED"then M:SearchResults(key,true)
    elseif event=="ITEM_SEARCH_RESULTS_UPDATED"or event=="ITEM_SEARCH_RESULTS_ADDED"then M:SearchResults(key,false)
    elseif event=="ITEM_DATA_LOAD_RESULT"or event=="GET_ITEM_INFO_RECEIVED"then M:ItemDataLoaded(key)
    elseif event=="MERCHANT_SHOW"or event=="MERCHANT_UPDATE"then M.merchantAt=GetTime()+.75
    elseif event=="MERCHANT_CLOSED"then M.merchantAt=nil
    elseif event=="TRADE_SKILL_CLOSE"then M.recipeOpen=false;M.forceRecipes=nil;M.recipeAt=nil;M.recipeJob=nil
    elseif event=="TRADE_SKILL_SHOW"then M.recipeOpen=true;M.forceRecipes=true;M.recipeAt=GetTime()+1
    elseif event=="TRADE_SKILL_DATA_SOURCE_CHANGED"then M.recipeJob=nil;if M.recipeOpen then M.forceRecipes=true;M.recipeAt=GetTime()+1 end
    elseif M.recipeOpen then M.recipeAt=M.recipeAt or GetTime()+1 end
end)
local elapsed=0;events:SetScript("OnUpdate",function(_,dt)elapsed=elapsed+dt;if elapsed<.1 then return end;elapsed=0;local ok,err=pcall(M.Tick,M);if not ok then M.failed=true;if M.bridge then M.bridge.frame:Hide()end;M:Status("Market paused after an error: "..tostring(err)..". Clear queue to retry.")end end)
M.JSON=json;M.Array=arr -- Wire regression harness.
