-- Timestamped, data-only planner sync. Desktop updates are read once per reload.
local _,LUI=...
local K=LUI.KBB
local S={};K.Sync=S
local ARRAY={}
local function arr(t)return setmetatable(t or {},ARRAY)end
local function copy(t)local n={};for k,v in pairs(t or {})do n[k]=type(v)=="table"and copy(v)or v end;return n end
local function json(v)
    local kind=type(v)
    if kind=="nil"then return "null"end
    if kind=="boolean"then return v and "true"or "false"end
    if kind=="number"then assert(v==v and math.abs(v)<math.huge,"Invalid planner number");return string.format("%.17g",v)end
    if kind=="string"then return '"'..v:gsub('[%z\1-\31\\"]',function(c)
        if c=='"'or c=='\\'then return '\\'..c end;return string.format('\\u%04x',c:byte())end)..'"'end
    assert(kind=="table","Invalid planner value")
    local out={}
    if getmetatable(v)==ARRAY then for _,x in ipairs(v)do out[#out+1]=json(x)end;return '['..table.concat(out,',')..']'end
    local keys={};for k in pairs(v)do assert(type(k)=="string");keys[#keys+1]=k end;table.sort(keys)
    for _,k in ipairs(keys)do out[#out+1]=json(k)..':'..json(v[k])end;return '{'..table.concat(out,',')..'}'
end
local function fields(source,names,target)
    target=target or {};for name in names:gmatch('%S+')do if source[name]~=nil then target[name]=copy(type(source[name])=="table"and source[name]or {})
        if type(source[name])~="table"then target[name]=source[name]end end end;return target
end
local function boardData(b,order)
    local v={id=b.id,name=b.name,order=order,columns=arr(),fields=arr(),questColumn=b.questColumn,doneColumn=b.doneColumn}
    for _,name in ipairs({"autoWatch","autoDone","hideQuestTracker"})do if b[name]then v[name]=true end end
    for _,c in ipairs(b.columns)do
        local d={id=c.id,name=c.name}
        fields(c,"uiSize uiPosition",d)
        if type(c.uiVisibility)=="table"and next(c.uiVisibility)then d.uiVisibility=copy(c.uiVisibility)end
        if c.uiPlacement and c.uiPlacement~=""then d.uiPlacement=c.uiPlacement end
        for _,name in ipairs({"showOnUI","uiLocked","uiTransparent","uiCompact","uiCollapsed"})do if c[name]then d[name]=true end end
        v.columns[#v.columns+1]=d
    end
    for _,f in ipairs(b.fields or {})do local d={id=f.id,name=f.name,type=f.type};if f.hidden then d.hidden=true end
        if f.options and #f.options>0 then d.options=arr(copy(f.options))end;v.fields[#v.fields+1]=d end
    return v
end
local function cardData(c,order)
    local v={id=c.id,title=c.title,notes=c.notes or "",column=c.column,kind=c.kind or "Note",priority=c.priority or "Normal",values=copy(c.values),order=order}
    if c.archived then v.archived=true end
    for _,name in ipairs({"created","updated","questID"})do if c[name]and c[name]~=0 then v[name]=c[name]end end
    for _,name in ipairs({"questStatus","questProgress"})do if c[name]and c[name]~=""then v[name]=c[name]end end
    return v
end
local function normalized(r)
    if r.deleted then return {key=r.key,stamp=copy(r.stamp),deleted=true}end
    local b=r.data;local d=r.key:sub(1,2)=="b/"and boardData(b,b.order)or cardData(b,b.order)
    return {key=r.key,stamp=copy(r.stamp),data=d}
end
local function compare(a,b)
    if not b then return 1 end
    for _,k in ipairs({"at","seq","actor"})do if a[k]~=b[k]then return a[k]>b[k]and 1 or -1 end end;return 0
end
local function wall()return (GetServerTime and GetServerTime()or time())*1000 end
local function nextStamp(sync)
    local at=wall();local old=sync.clock or {at=0,seq=0};local seq=0
    if old.at>=at then at=old.at;seq=old.seq+1 end
    sync.clock={at=at,seq=seq};return {at=at,seq=seq,actor="game"}
end
local function observe(sync,stamp)
    local old=sync.clock or {at=0,seq=0}
    if stamp.at>old.at or(stamp.at==old.at and stamp.seq>old.seq)then sync.clock={at=stamp.at,seq=stamp.seq}end
end
function S:Client()return select(4,GetBuildInfo())==16001 and "forever"or "retail"end
function S:Capture(initial)
    local collection=K:Collection();if not collection then return end
    local sync=collection.sync
    if not sync then sync={schema=1,records={},outbox={}};collection.sync=sync end
    assert(sync.schema==1 and type(sync.records)=="table","Unsupported planner sync data")
    sync.outbox=sync.outbox or {}
    local current={}
    for i,b in ipairs(collection.boards)do
        current['b/'..b.id]=boardData(b,i)
        local previous=0
        for j,c in ipairs(b.cards)do
            if c.order==nil then c.order=previous+1 end
            current['c/'..b.id..'/'..c.id]=cardData(c,c.order);previous=c.order
        end
    end
    local changed=false
    for key,data in pairs(current)do
        local old=sync.records[key]
        if not old or old.deleted or json(normalized(old).data)~=json(data)then
            local stamp
            if initial and not old then stamp={at=math.min(wall(),(data.updated or 0)*1000),seq=0,actor="game"};observe(sync,stamp)
            else stamp=nextStamp(sync)end
            sync.records[key]={key=key,stamp=stamp,data=data};changed=true
        end
    end
    for key in pairs(self.present or {})do
        local old=sync.records[key]
        if not current[key]and old and not old.deleted then sync.records[key]={key=key,stamp=nextStamp(sync),deleted=true};changed=true end
    end
    self.present=current
    if changed then self.dirty=true end
    return sync
end
-- Keep stable fractional positions: moving one card does not stamp every
-- shifted card with a newer version of its old notes.
function S:Moved(board,card)
    if not self.ready then return end
    for i,c in ipairs(board.cards)do if c==card then
        local before=board.cards[i-1];local after=board.cards[i+1]
        local low=before and before.order or 0
        card.order=after and (low+(after.order or low+2))/2 or low+1
        return
    end end
end
function S:Changed()
    if not self.ready or self.applying then return end
    local ok,err=pcall(self.Capture,self,false)
    if not ok then self.error=tostring(err)end
end
local function validStamp(s)
    return type(s)=="table"and type(s.at)=="number"and s.at>=0 and s.at<=wall()+300000 and s.at%1==0 and
        type(s.seq)=="number"and s.seq>=0 and s.seq<=2147483647 and s.seq%1==0 and(s.actor=="game"or s.actor=="desktop")
end
function S:Apply(sync,snapshot)
    assert(snapshot.schema==1 and snapshot.client==self:Client()and snapshot.guid==K:CharacterKey()and type(snapshot.records)=="table"and #snapshot.records<=21000,"Invalid desktop planner identity")
    local incoming={}
    for _,r in ipairs(snapshot.records)do
        assert(type(r.key)=="string"and validStamp(r.stamp)and not incoming[r.key],"Invalid desktop planner record")
        local boardID,cardID=r.key:match('^c/([%w_-]+)/([%w_-]+)$');boardID=boardID or r.key:match('^b/([%w_-]+)$')
        assert(boardID and #boardID<=64 and(not cardID or #cardID<=64),"Invalid desktop planner ID")
        local n=normalized(r)
        if n.data then assert(n.data.id==(cardID or boardID)and type(n.data.order)=="number","Invalid desktop planner data")end
        incoming[r.key]=n
    end
    for key,r in pairs(incoming)do
        observe(sync,r.stamp);local old=sync.records[key]
        if not old or compare(r.stamp,old.stamp)>0 then
            if old and json(normalized(old).data)~=json(r.data)then
                sync.outbox[#sync.outbox+1]=old;while #sync.outbox>200 do table.remove(sync.outbox,1)end
            end
            sync.records[key]=r
        elseif compare(r.stamp,old.stamp)==0 then
            assert(json(normalized(old))==json(r),"Conflicting desktop planner timestamp")
        end
    end
    local collection=K:Collection();local boards,map={},{}
    local serials={};for _,b in ipairs(collection.boards)do serials[b.id]=b.serial end
    for key,r in pairs(sync.records)do if key:sub(1,2)=='b/'and not r.deleted then
        local b=copy(r.data);b.cards={};b.serial=serials[b.id]or 0
        for _,col in ipairs(b.columns)do b.serial=math.max(b.serial,tonumber(col.id:match('^c(%d+)$'))or 0)end
        for _,f in ipairs(b.fields)do f.options=f.options or {};b.serial=math.max(b.serial,tonumber(f.id:match('^f(%d+)$'))or 0)end
        boards[#boards+1]=b;map[b.id]=b
        collection.serial=math.max(collection.serial,tonumber(b.id:match('^b(%d+)$'))or 0)
    end end
    for key,r in pairs(sync.records)do if key:sub(1,2)=='c/'and not r.deleted then
        local boardID=key:match('^c/([^/]+)/');local b=map[boardID]
        if b then
            local c=copy(r.data);local found=false;for _,col in ipairs(b.columns)do if col.id==c.column then found=true end end
            -- Concurrent column removal keeps the newer card in the first list.
            if not found then c.column=b.columns[1].id end
            b.cards[#b.cards+1]=c;b.serial=math.max(b.serial,tonumber(c.id:match('^t(%d+)$'))or 0)
        end
    end end
    local function ordered(a,b)if a.order==b.order then return a.id<b.id end;return a.order<b.order end
    assert(#boards>=1 and #boards<=20,"Invalid desktop board count")
    table.sort(boards,ordered);for _,b in ipairs(boards)do assert(#b.columns>=1 and #b.columns<=12 and #b.cards<=1000,"Invalid desktop board size");table.sort(b.cards,ordered)end
    collection.boards=boards;if not map[collection.active]then collection.active=boards[1].id end
end
function S:Initialize()
    if self.ready then return end
    local data=type(LamdaKBBData)=="table"and LamdaKBBData
    local snapshot=data and data.schema==1 and data.client==self:Client()and type(data.characters)=="table"and data.characters[K:CharacterKey()]
    if not K:Collection()then if snapshot then K:Collection(true)else self.ready=true;return end end
    self.applying=true
    local ok,err=pcall(function()
        local sync=self:Capture(true)
        if snapshot then
            -- Roll back the whole import if validation/application fails.
            local collection=K:Collection();local backup=copy(collection)
            local applied,why=pcall(self.Apply,self,sync,snapshot)
            if not applied then LamdaUIDB.kbb.characters[K:CharacterKey()]=backup;error(why,0)end
        end
        self:Capture(false)
    end)
    self.applying=false;self.ready=true
    if not ok then self.error=tostring(err)end
    if K.UI then K.UI:BoardChanged()end
    self.dirty=true
end
function S:Batches()
    local collection=K:Collection();if not collection or not collection.sync then return {}end
    local sync=collection.sync;local all={};local keys={};for key in pairs(sync.records)do keys[#keys+1]=key end;table.sort(keys)
    -- Send current state first. Recovery copies use separate batches so a key
    -- never appears twice within one snapshot.
    local batch={};local size=0
    for _,key in ipairs(keys)do local r=normalized(sync.records[key]);local n=#json(r)+1
        if size+n>110000 then all[#all+1]=batch;batch={};size=0 end
        batch[#batch+1]=r;size=size+n
    end
    if #batch>0 then all[#all+1]=batch end
    for _,r in ipairs(sync.outbox)do all[#all+1]={normalized(r)}end
    local out={}
    for _,records in ipairs(all)do out[#out+1]=json({schema=1,client=self:Client(),guid=K:CharacterKey(),name=UnitName("player")or "",realm=GetRealmName()or "",records=arr(records)})end
    return out
end
function S:CreateStrip()
    if self.strip then return end
    self.bridge=LUI.LookupEvents:GetStrip("kbb");self.strip=self.bridge.frame;self.cells=self.bridge.cells
end
function S:Tick()
    if LUI.overlayHidden then if self.strip then self.strip:Hide()end;self.shownAt=nil;return end
    if not self.ready or self.error or not PixelUtil or not LUI.LookupEvents then return end
    if not K:Collection()then return end
    if not LUI:DB().kbbEnabled and not LUI.LookupEvents.strips.kbb then return end
    local bridge=LUI.LookupEvents:GetStrip("kbb")
    if bridge.moving then self.shownAt=nil;return end
    local now=GetTime()
    if LamdaEngineDB and LamdaEngineDB.kbbEnabled==false then
        if self.strip then self.strip:Hide()end;self.shownAt=nil;return
    end
    if self.dirty and self.batches and not self.packets then self.batches=nil end
    if not self.batches then
        if not self.dirty and now<(self.retryAt or 0)then return end
        self:Capture(false);self.batches=self:Batches();self.batchIndex=1;self.dirty=false
    end
    local data=self.batches[self.batchIndex]
    if not data then self.batches=nil;self.retryAt=now+30;if self.strip then self.strip:Hide()end;return end
    if not self.packets or now-(self.transferAt or 0)>160 then
        self:CreateStrip();self.sequence=(self.sequence or 0)+1
        local at=GetServerTime and GetServerTime()or time();local id=at..'_'..math.floor(now*1000)..'_'..self.sequence
        self.packets={};local count=math.ceil(#data/5600);local crc=LUI.LookupEvents:Checksum(data)
        for i=1,count do local header=table.concat({'2',id,self:Client(),at,#data,i,count,string.format('%.0f',crc)},'|')
            self.packets[i]=assert(LUI.LookupEvents:Packet(header..'\n'..data:sub((i-1)*5600+1,i*5600),'LUK2'))end
        self.packetIndex=1;self.passes=0;self.shownAt=nil;self.transferAt=now
    end
    if self.shownAt then
        if now-self.shownAt<.4 then return end
        self.packetIndex=self.packetIndex+1
        if self.packetIndex>#self.packets then self.packetIndex=1;self.passes=self.passes+1 end
        if self.passes>=3 then self.batchIndex=self.batchIndex+1;self.packets=nil;self.shownAt=nil;self.strip:Hide();return end
    end
    self.bridge:ApplyPosition()
    local packet=self.packets[self.packetIndex]
    for i,cell in ipairs(self.cells)do local offset=(i-1)*3;cell:SetColorTexture(packet:byte(offset+1)/255,packet:byte(offset+2)/255,packet:byte(offset+3)/255,1)end
    self.shownAt=now;self.strip:Show()
end
S.JSON=json -- Used by the offline wire-format regression tests.
local events=CreateFrame("Frame");S.events=events;events:RegisterEvent("PLAYER_ENTERING_WORLD");events:RegisterEvent("PLAYER_LOGOUT")
events:SetScript("OnEvent",function(_,event)
    if event=="PLAYER_LOGOUT"then S:Changed();return end
    S:Initialize()
end)
local elapsed=0
events:SetScript("OnUpdate",function(_,dt)
    elapsed=elapsed+dt;if elapsed<.1 then return end;elapsed=0
    local ok,err=pcall(S.Tick,S);if not ok then S.error=tostring(err);if S.strip then S.strip:Hide()end end
end)
