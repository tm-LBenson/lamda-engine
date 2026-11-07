-- Player-authored plans live outside CD profiles. Quest state never owns notes.
local _,LUI=...
local K={kinds={"Note","Dungeon","Achievement","Recipe","Rotation","Quest"},priorities={"Normal","High","Low"}}
LUI.KBB=K
K.displayContexts={{"world","Outdoors"},{"party","Dungeons (no active key)"},{"key","Mythic+ (key active)"},
    {"raid","Raids"},{"scenario","Delves / scenarios"},{"arena","Arenas"},{"pvp","Battlegrounds"}}
local function public(v)return not issecretvalue or not issecretvalue(v)end
local function call(fn,...)
    if type(fn)~="function" then return nil end
    local ok,value=pcall(fn,...);if ok and public(value) then return value end
end
local function text(v,max,blank)
    if not public(v) or type(v)~="string" or #v>max or v:find("[%z\1-\8\11\12\14-\31]") then return nil end
    v=v:match("^%s*(.-)%s*$");if not blank and v=="" then return nil end
    return v
end
local function includes(list,value)for _,v in ipairs(list)do if v==value then return true end end end
local function find(list,id)for i,v in ipairs(list)do if v.id==id then return v,i end end end
local function copy(t)local n={};for k,v in pairs(t or {})do n[k]=v end;return n end
local function id(b,prefix)b.serial=b.serial+1;return prefix..b.serial end
local function stamp()return type(time)=="function" and time() or 0 end
function K:CharacterKey()
    local guid=call(UnitGUID,"player")
    if type(guid)=="string" and guid:match("^Player%-%w[%w%-]*$") then return guid end
end
local function newBoard()
    return {serial=4,columns={{id="c1",name="Backlog"},{id="c2",name="Next"},{id="c3",name="Doing"},{id="c4",name="Done"}},fields={},cards={},questColumn="c3",doneColumn="c4",autoWatch=false,autoDone=false}
end
local function readable(b)
    return type(b)=="table" and type(b.columns)=="table" and #b.columns>0 and type(b.cards)=="table" and type(b.fields)=="table" and type(b.serial)=="number"
end
function K:Collection(create)
    local key=self:CharacterKey();if not key then return nil,"Log into a character to open a board." end
    if type(LamdaUIDB)~="table" then if not create then return end;LamdaUIDB={} end
    local db=LamdaUIDB.kbb
    if db==nil and create then db={schema=2,characters={}};LamdaUIDB.kbb=db end
    if db==nil then return end
    if type(db)~="table" or (db.schema~=1 and db.schema~=2) or type(db.characters)~="table" then return nil,"This planner data version cannot be opened. Your saved data was kept." end
    if db.schema==1 then
        -- Validate every character before migrating any of them. Keep the actual
        -- board objects, including notes, hidden fields, quest rules and positions.
        for _,b in pairs(db.characters)do if not readable(b)then return nil,"A saved board could not be read. Your saved data was kept." end end
        local characters={}
        for guid,b in pairs(db.characters)do
            b.id="b1";b.name="My board";characters[guid]={serial=1,active="b1",boards={b}}
        end
        db.characters=characters;db.schema=2
    end
    local collection=db.characters[key]
    if collection==nil and create then
        local b=newBoard();b.id="b1";b.name="My board"
        collection={serial=1,active=b.id,boards={b}};db.characters[key]=collection
    end
    if collection and (type(collection)~="table" or type(collection.boards)~="table" or type(collection.serial)~="number" or not find(collection.boards,collection.active))then
        return nil,"The board list could not be read. Your saved data was kept."
    end
    return collection
end
function K:Board(create)
    local collection,err=self:Collection(create);if not collection then return nil,err end
    local b=find(collection.boards,collection.active)
    if not readable(b)then return nil,"The board could not be read. Your saved data was kept." end
    return b
end
function K:CanSwitchBoard()
    if self.UI and self.UI.editor and self.UI.editor:IsShown()then return nil,"Save or cancel the open card before switching boards." end
    return true
end
function K:SelectBoard(boardID)
    local collection,err=self:Collection();if not collection then return nil,err end
    local b=find(collection.boards,boardID);if not readable(b)then return nil,"Choose a saved board." end
    if collection.active==boardID then return true end
    local ok,why=self:CanSwitchBoard();if not ok then return nil,why end
    collection.active=boardID
    if self.UI then self.UI:BoardChanged()end
    if b.autoWatch then self:ImportWatched()end
    self:RefreshQuests();self:Changed();return true
end
local function boardName(collection,name,except)
    name=text(name,60);if not name then return nil,"Enter a board name (1–60 characters)." end
    for _,b in ipairs(collection.boards)do if b~=except and b.name:lower()==name:lower()then return nil,"That board name is already used." end end
    return name
end
function K:NewBoard(name)
    local ok,err=self:CanSwitchBoard();if not ok then return nil,err end
    local collection,why=self:Collection(true);if not collection then return nil,why end
    if #collection.boards>=20 then return nil,"A character can have up to 20 boards." end
    name,err=boardName(collection,name);if not name then return nil,err end
    local b=newBoard();b.name=name;b.id=id(collection,"b");collection.boards[#collection.boards+1]=b
    self:SelectBoard(b.id);return b
end
function K:RenameBoard(name)
    local collection,err=self:Collection();if not collection then return nil,err end
    local b=self:Board();name,err=boardName(collection,name,b);if not name then return nil,err end
    b.name=name;self:Changed();return true
end
function K:Changed()
    if self.batch then return end
    if self.UI then self.UI:Refresh()end
    if self.Sync then self.Sync:Changed()end
end
function K:Column(column)
    local b=self:Board();return b and find(b.columns,column)
end
function K:SetColumnDisplay(column,key,value)
    local c=self:Column(column)
    if not c or not includes({"showOnUI","uiLocked","uiTransparent","uiCompact","uiCollapsed"},key) or type(value)~="boolean" then return nil,"Choose a column and a display option." end
    c[key]=value;self:Changed();return true
end
function K:SetColumnPlacement(column,value)
    local c=self:Column(column)
    if not c or not includes({"floating","tracker","both"},value)then return nil,"Choose where to display this column." end
    c.uiPlacement=value;self:Changed();return true
end
function K:SetColumnVisibility(column,context,shown)
    local c=self:Column(column);local valid=false
    for _,entry in ipairs(self.displayContexts)do if entry[1]==context then valid=true end end
    if not c or not valid or type(shown)~="boolean"then return nil,"Choose a column and a visibility rule." end
    if type(c.uiVisibility)~="table"then c.uiVisibility={}end
    c.uiVisibility[context]=shown;self:Changed();return true
end
function K:DisplayContext()
    local kind=call(function()local inside,value=IsInInstance();return inside and value or "none" end)or "none"
    if kind=="party"then
        local active=self.keyActive
        if active==nil then active=C_ChallengeMode and call(C_ChallengeMode.IsChallengeModeActive)end
        if active==true then return "key"end
    end
    if C_ScenarioInfo and call(C_ScenarioInfo.IsTieredEntranceScenario)==true then return "scenario"end
    if kind=="none"then return "world"end
    for _,entry in ipairs(self.displayContexts)do if entry[1]==kind then return kind end end
    return "world"
end
function K:ColumnVisible(column,context)
    return column.showOnUI==true and (type(column.uiVisibility)~="table" or column.uiVisibility[context or self:DisplayContext()]~=false)
end
function K:SetDisplaysLocked(locked)
    local b=self:Board();if not b then return 0 end
    local count=0
    for _,c in ipairs(b.columns)do if c.showOnUI==true then
        c.uiLocked=locked==true;count=count+1
    end end
    if count>0 then self:Changed()end
    return count
end
local function finite(v)return type(v)=="number" and v==v and v~=math.huge and v~=-math.huge end
function K:SetColumnSize(column,width,height)
    local c=self:Column(column);if not c then return nil,"Choose a column." end
    if width==nil and height==nil then c.uiSize=nil
    elseif finite(width)and finite(height)and width>=220 and width<=640 and height>=150 and height<=4096 then
        c.uiSize={width=math.floor(width+.5),height=math.floor(height+.5)}
    else return nil,"Width must be 220–640 and height 150–4096 (displayed within the screen)." end
    self:Changed();return true
end
function K:SetColumnPosition(column,x,y)
    local c=self:Column(column);if not c then return nil,"Choose a column." end
    if x==nil and y==nil then c.uiPosition=nil
    elseif finite(x) and finite(y) then c.uiPosition={x=math.max(0,math.min(1,x)),y=math.max(0,math.min(1,y))}
    else return nil,"The column position could not be saved." end
    self:Changed();return true
end
function K:SetHideQuestTracker(hidden)
    local b,err=self:Board(true);if not b then return nil,err end
    b.hideQuestTracker=hidden==true;self:Changed();return true
end
function K:ResetDisplays()
    local collection=self:Collection();if not collection then return end
    for _,b in ipairs(collection.boards)do
        b.hideQuestTracker=false
        for _,c in ipairs(b.columns)do
            c.showOnUI=false;c.uiLocked=false;c.uiTransparent=false;c.uiPosition=nil
            c.uiCompact=nil;c.uiCollapsed=nil;c.uiSize=nil;c.uiVisibility=nil;c.uiPlacement=nil
        end
    end
    self:Changed()
end
function K:AddColumn(name)
    local b,err=self:Board(true);if not b then return nil,err end
    name=text(name,40);if not name then return nil,"Enter a column name (1–40 characters)." end
    if #b.columns>=12 then return nil,"A board can have up to 12 columns." end
    for _,c in ipairs(b.columns)do if c.name:lower()==name:lower() then return nil,"That column name is already used." end end
    local c={id=id(b,"c"),name=name};b.columns[#b.columns+1]=c;self:Changed();return c
end
function K:RenameColumn(column,name)
    local b=self:Board();local c=b and find(b.columns,column);name=text(name,40)
    if not c or not name then return nil,"Enter a column name (1–40 characters)." end
    for _,other in ipairs(b.columns)do if other~=c and other.name:lower()==name:lower() then return nil,"That column name is already used." end end
    c.name=name;self:Changed();return true
end
function K:ReorderColumn(column,delta)
    local b=self:Board();local c,i
    if b then c,i=find(b.columns,column)end
    if not c then return nil,"Choose a column." end
    local target=math.max(1,math.min(#b.columns,i+delta));table.remove(b.columns,i);table.insert(b.columns,target,c);self:Changed();return true
end
function K:RemoveColumn(column,destination)
    local b=self:Board();if not b then return nil,"Open a board first." end
    local c,i=find(b.columns,column)
    if not c or column==destination or not find(b.columns,destination) then return nil,"Choose a different column to receive the cards." end
    for _,card in ipairs(b.cards)do if card.column==column then card.column=destination end end
    if b.questColumn==column then b.questColumn=destination end
    if b.doneColumn==column then b.doneColumn=destination end
    table.remove(b.columns,i);self:Changed();return true
end
function K:AddField(name,kind,options)
    local b,err=self:Board(true);if not b then return nil,err end
    name=text(name,40)
    if not name or not includes({"Text","Number","Checkbox","Choice"},kind) then return nil,"Enter a field name and choose a field type." end
    if #b.fields>=16 then return nil,"A board can have up to 16 custom fields, including hidden fields." end
    for _,f in ipairs(b.fields)do if f.name:lower()==name:lower()then return nil,"That field name is already used." end end
    local choices={}
    if kind=="Choice" then
        if type(options)~="string" or #options>800 then return nil,"Enter comma-separated choices (up to 12)." end
        for part in (options..","):gmatch("(.-),")do
            local choice=text(part,60);if not choice or includes(choices,choice) then return nil,"Choices must be unique, nonempty, and at most 60 characters." end
            choices[#choices+1]=choice
        end
        if #choices<2 or #choices>12 then return nil,"Enter 2–12 comma-separated choices." end
    end
    local f={id=id(b,"f"),name=name,type=kind,options=choices};b.fields[#b.fields+1]=f;self:Changed();return f
end
function K:UpdateField(field,name,hidden)
    local b=self:Board();local f=b and find(b.fields,field);name=text(name,40)
    if not f or not name then return nil,"Enter a field name (1–40 characters)." end
    for _,other in ipairs(b.fields)do if other~=f and other.name:lower()==name:lower()then return nil,"That field name is already used." end end
    f.name=name;f.hidden=not not hidden;self:Changed();return true
end
function K:SaveCard(cardID,draft)
    local b,err=self:Board(true);if not b then return nil,err end
    local existing=cardID and find(b.cards,cardID)
    if cardID and not existing then return nil,"That card no longer exists." end
    if not existing and #b.cards>=1000 then return nil,"This board has reached its 1,000-card limit." end
    local title=text(draft.title,160);local notes=text(draft.notes or "",8000,true)
    if not title or not notes then return nil,"Enter a title (up to 160 characters) and notes of up to 8,000 characters." end
    if not find(b.columns,draft.column) then return nil,"Choose a column." end
    if not includes(self.kinds,draft.kind) or not includes(self.priorities,draft.priority) then return nil,"Choose a category and priority." end
    local values=copy(existing and existing.values)
    for _,f in ipairs(b.fields)do if not f.hidden then
        local value=(draft.values or {})[f.id]
        if value==nil or value=="" then values[f.id]=nil
        elseif f.type=="Text" then
            value=text(value,500,true);if value==nil then return nil,f.name..": enter up to 500 characters." end;values[f.id]=value
        elseif f.type=="Number" then
            local number=tonumber(value)
            if not number or number~=number or number==math.huge or number==-math.huge then return nil,f.name..": enter a finite number." end
            values[f.id]=number
        elseif f.type=="Checkbox" then
            if type(value)~="boolean" then return nil,f.name..": choose checked or unchecked." end;values[f.id]=value
        elseif f.type=="Choice" then
            if not includes(f.options,value) then return nil,f.name..": choose a listed value." end;values[f.id]=value
        end
    end end
    local c=existing or {id=id(b,"t"),created=stamp()}
    c.title=title;c.notes=notes;c.kind=draft.kind;c.priority=draft.priority;c.column=draft.column;c.values=values;c.updated=stamp()
    if not existing then b.cards[#b.cards+1]=c end
    self:Changed();return c
end
function K:MoveCard(cardID,column,beforeID)
    local b=self:Board();if not b or not find(b.columns,column) then return nil,"Choose a column." end
    local c,i=find(b.cards,cardID);if not c then return nil,"Choose a card." end
    if beforeID==cardID then return true end
    c.column=column;table.remove(b.cards,i)
    local target,position=find(b.cards,beforeID)
    if target and target.column==column then table.insert(b.cards,position,c)else b.cards[#b.cards+1]=c end
    if self.Sync then self.Sync:Moved(b,c)end
    self:Changed();return true
end
function K:ArchiveCard(cardID,archived)
    local b=self:Board();local c=b and find(b.cards,cardID);if not c then return nil,"Choose a card." end
    c.archived=not not archived;self:Changed();return true
end
function K:Cards(column,query,kind,archived)
    local b=self:Board();local rows={};if not b then return rows end
    query=(query or ""):lower()
    for _,c in ipairs(b.cards)do
        if (not column or c.column==column) and (not kind or kind=="All categories" or c.kind==kind) and (not not c.archived)==(not not archived) then
            if query=="" then rows[#rows+1]=c
            else
                local haystack=c.title.."\n"..c.notes
                for _,f in ipairs(b.fields)do if not f.hidden and c.values[f.id]~=nil then haystack=haystack.."\n"..tostring(c.values[f.id])end end
                if haystack:lower():find(query,1,true) then rows[#rows+1]=c end
            end
        end
    end
    return rows
end

-- Versioned, bounded data-only sharing. Length-prefixed UTF-8 tokens are hex
-- encoded for copying through chat/web pages. No Lua evaluation or decompressor.
-- IDs are local references by index; character identity, live quest progress,
-- automatic quest actions and screen preferences never leave the character.
local shareBytes=1048576
K.shareLimit=shareBytes*2+14
local function checksum(raw)
    local a,b=1,0
    for i=1,#raw do a=(a+raw:byte(i))%65521;b=(b+a)%65521 end
    return string.format("%08x",b*65536+a)
end
function K:ExportBoard()
    local b,err=self:Board();if not b then return nil,err end
    local ok,result=pcall(function()
        local tokens,size={},0
        local function put(v)
            local s=type(v)=="number"and string.format("%.17g",v)or tostring(v);local token=#s..":"..s;size=size+#token
            if size>shareBytes then error("This board is too large to share (1 MiB of board data maximum).",0)end
            tokens[#tokens+1]=token
        end
        put(b.name);put(#b.columns);local columns={}
        for i,c in ipairs(b.columns)do columns[c.id]=i;put(c.name)end
        put(columns[b.questColumn]or 1);put(columns[b.doneColumn]or #b.columns)
        put(#b.fields)
        for _,f in ipairs(b.fields)do
            put(f.name);put(f.type);put(f.hidden and 1 or 0);put(#f.options)
            for _,option in ipairs(f.options)do put(option)end
        end
        put(#b.cards)
        for _,c in ipairs(b.cards)do
            put(c.title);put(c.notes);put(columns[c.column]);put(c.kind);put(c.priority);put(c.archived and 1 or 0);put(c.questID or "")
            for _,f in ipairs(b.fields)do
                local value=c.values[f.id];put(value~=nil and 1 or 0)
                if value~=nil then put(f.type=="Checkbox" and (value and "1" or "0")or value)end
            end
        end
        local raw=table.concat(tokens)
        return "LKB1:"..checksum(raw)..":"..(raw:gsub(".",function(c)return string.format("%02x",c:byte())end))
    end)
    if ok then return result end
    return nil,result
end
local function decodeBoard(encoded)
    if type(encoded)~="string" or #encoded>K.shareLimit+4096 then error("Use a board export string of up to 2 MiB.",0)end
    encoded=encoded:gsub("%s","")
    if #encoded>K.shareLimit or encoded:sub(1,5)~="LKB1:" then error("Use a supported LamdaKBB board export (LKB1).",0)end
    local sum,hex=encoded:match("^LKB1:(%x%x%x%x%x%x%x%x):(%x+)$")
    if not hex or #hex%2~=0 then error("The board string is incomplete or damaged. Copy the entire export again.",0)end
    local raw=hex:gsub("..",function(pair)return string.char(tonumber(pair,16))end)
    if checksum(raw)~=sum:lower()then error("The board string is incomplete or damaged. Copy the entire export again.",0)end
    local pos=1
    local function read(max)
        local n=raw:sub(pos,pos+8):match("^(%d+):");local size=n and tonumber(n)
        if not size or size>max or #n>7 then error("Invalid board data length.",0)end
        pos=pos+#n+1;local last=pos+size-1
        if last>#raw then error("The board data is incomplete.",0)end
        local value=raw:sub(pos,last);pos=last+1;return value
    end
    local function number(min,max)
        local s=read(12);local n=tonumber(s)
        if not s:match("^%d+$") or not n or n<min or n>max then error("Invalid board count or reference.",0)end
        return n
    end
    local function word(max,blank)
        local value=text(read(max),max,blank)
        if not value then error("Invalid board text.",0)end
        return value
    end
    local function uniqueName(seen,max,exact)
        local value=word(max);local key=exact and value or value:lower()
        if seen[key]then error("Repeated column, field, or choice name.",0)end
        seen[key]=true;return value
    end
    local b={name=word(60),serial=0,columns={},fields={},cards={},autoWatch=false,autoDone=false}
    local names={}
    for _=1,number(1,12)do b.columns[#b.columns+1]={id=id(b,"c"),name=uniqueName(names,40)}end
    b.questColumn=b.columns[number(1,#b.columns)].id;b.doneColumn=b.columns[number(1,#b.columns)].id
    names={}
    for _=1,number(0,16)do
        local f={id=id(b,"f"),name=uniqueName(names,40),type=word(20),hidden=number(0,1)==1,options={}}
        if not includes({"Text","Number","Checkbox","Choice"},f.type)then error("Unsupported field type.",0)end
        local count=number(0,12)
        if (f.type=="Choice" and count<2)or(f.type~="Choice" and count~=0)then error("Invalid field choices.",0)end
        local choices={};for _=1,count do f.options[#f.options+1]=uniqueName(choices,60,true)end
        b.fields[#b.fields+1]=f
    end
    local quests={}
    for _=1,number(0,1000)do
        local c={id=id(b,"t"),title=word(160),notes=word(8000,true),column=b.columns[number(1,#b.columns)].id,
            kind=word(20),priority=word(20),archived=number(0,1)==1,values={},created=stamp(),updated=stamp()}
        if not includes(K.kinds,c.kind)or not includes(K.priorities,c.priority)then error("Unsupported card category or priority.",0)end
        local quest=read(10)
        if quest~="" then
            local n=tonumber(quest)
            if not quest:match("^%d+$")or not n or n<1 or n>2147483647 or quests[n]then error("Invalid or repeated quest link.",0)end
            c.questID=n;quests[n]=true
        end
        for _,f in ipairs(b.fields)do if number(0,1)==1 then
            local value
            if f.type=="Text" then value=word(500,true)
            elseif f.type=="Checkbox" then value=number(0,1)==1
            elseif f.type=="Number" then value=tonumber(read(40));if not finite(value)then error("Invalid number field.",0)end
            else value=word(60);if not includes(f.options,value)then error("Invalid choice value.",0)end end
            c.values[f.id]=value
        end end
        b.cards[#b.cards+1]=c
    end
    if pos~=#raw+1 then error("Unexpected data at the end of the board string.",0)end
    return b
end
function K:ImportBoard(encoded)
    local allowed,why=self:CanSwitchBoard();if not allowed then return nil,why end
    local ok,b=pcall(decodeBoard,encoded);if not ok then return nil,b end
    local collection,err=self:Collection(true);if not collection then return nil,err end
    if #collection.boards>=20 then return nil,"A character can have up to 20 boards." end
    local original=b.name;local suffix=1
    while not boardName(collection,b.name)do
        suffix=suffix+1;local ending=" ("..suffix..")";local length=60-#ending
        -- Do not split a multibyte character when making room for the suffix.
        while length>0 and original:byte(length+1)and original:byte(length+1)>=128 and original:byte(length+1)<192 do length=length-1 end
        b.name=original:sub(1,length)..ending
    end
    b.id=id(collection,"b");collection.boards[#collection.boards+1]=b
    self:SelectBoard(b.id);return b
end
function K:QuestAvailable()return C_QuestLog and type(C_QuestLog.IsOnQuest)=="function" and type(C_QuestLog.GetQuestObjectives)=="function" end
function K:QuestList()
    local rows,seen={},{};local api=C_QuestLog;if not self:QuestAvailable() then return rows end
    local function add(qid,title)
        if not public(qid) or type(qid)~="number" or qid<=0 or seen[qid] then return end
        if call(api.IsOnQuest,qid)~=true then return end
        seen[qid]=true;title=text(title or call(api.GetTitleForQuestID,qid),160) or ("Quest "..qid)
        rows[#rows+1]={id=qid,title=title,watched=call(api.GetQuestWatchType,qid)~=nil}
    end
    local count=call(api.GetNumQuestLogEntries)or 0
    for i=1,math.min(count,500)do
        local info=call(api.GetInfo,i)
        if type(info)=="table" and public(info.isHeader) and not info.isHeader and public(info.isHidden) and not info.isHidden then add(info.questID,info.title)end
    end
    -- Watch IDs remain available when a quest log header is collapsed.
    for i=1,math.min(call(api.GetNumQuestWatches)or 0,500)do add(call(api.GetQuestIDForQuestWatchIndex,i))end
    table.sort(rows,function(a,b)return a.title==b.title and a.id<b.id or a.title<b.title end);return rows
end
function K:AddQuest(questID)
    if not public(questID) or type(questID)~="number" or not self:QuestAvailable() or call(C_QuestLog.IsOnQuest,questID)~=true then return nil,"Choose an active quest on this character." end
    local b,err=self:Board(true);if not b then return nil,err end
    for _,c in ipairs(b.cards)do if c.questID==questID then return c,"This quest is already on the board (possibly archived)." end end
    local c,why=self:SaveCard(nil,{title=text(call(C_QuestLog.GetTitleForQuestID,questID),160)or("Quest "..questID),notes="",kind="Quest",priority="Normal",column=b.questColumn,values={}})
    if c then c.questID=questID;if not self.batch then self:RefreshQuests();self:Changed()end end
    return c,why
end
function K:ImportWatched()
    local count=0;local prior=self.batch;self.batch=true
    for _,q in ipairs(self:QuestList())do if q.watched then
        local c,why=self:AddQuest(q.id);if c and not why then count=count+1 end
    end end
    self.batch=prior;if count>0 then self:RefreshQuests();self:Changed()end;return count
end
function K:RefreshQuests(turnedIn,board)
    local b=board or self:Board();if not b or not self:QuestAvailable() then return end
    for _,c in ipairs(b.cards)do if c.questID then
        local old=c.questStatus;local onQuest=call(C_QuestLog.IsOnQuest,c.questID)
        if c.questID==turnedIn or type(turnedIn)=="table" and turnedIn[c.questID] then c.questStatus="Turned in";c.questProgress=""
        elseif onQuest==true then
            c.questStatus=call(C_QuestLog.IsComplete,c.questID) and "Ready to turn in" or "In progress"
            local objectives=call(C_QuestLog.GetQuestObjectives,c.questID);local lines={}
            for _,objective in ipairs(type(objectives)=="table" and objectives or {})do
                local line=text(objective.text,1000,true);if line then lines[#lines+1]=line end
            end
            c.questProgress=table.concat(lines,"\n")
        elseif onQuest==false then
            if c.questStatus=="Turned in" or (call(C_QuestLog.IsRepeatableQuest,c.questID)~=true and call(C_QuestLog.IsQuestFlaggedCompleted,c.questID)==true) then c.questStatus="Turned in"
            else c.questStatus="Not in quest log" end
            c.questProgress=""
        else c.questStatus="Quest data unavailable" end
        if b.autoDone and c.questStatus=="Turned in" and old~="Turned in" and find(b.columns,b.doneColumn)then c.column=b.doneColumn end
    end end
end
local events=CreateFrame("Frame");K.events=events
for _,event in ipairs({"PLAYER_ENTERING_WORLD","QUEST_LOG_UPDATE","QUEST_WATCH_UPDATE","QUEST_WATCH_LIST_CHANGED","QUEST_TURNED_IN","PLAYER_REGEN_ENABLED","ADDON_LOADED","DISPLAY_SIZE_CHANGED","UI_SCALE_CHANGED"})do events:RegisterEvent(event)end
-- Challenge events are absent on some supported clients. Manual boards still work.
for _,event in ipairs({"ZONE_CHANGED_NEW_AREA","GROUP_ROSTER_UPDATE","SCENARIO_UPDATE","CHALLENGE_MODE_START","CHALLENGE_MODE_COMPLETED","CHALLENGE_MODE_RESET"})do pcall(events.RegisterEvent,events,event)end
local pending=false;local turned={}
events:SetScript("OnEvent",function(_,event,questID)
    if event=="CHALLENGE_MODE_START"then K.keyActive=true
    elseif event=="CHALLENGE_MODE_COMPLETED"or event=="CHALLENGE_MODE_RESET"then K.keyActive=false
    elseif event=="PLAYER_ENTERING_WORLD"or event=="ZONE_CHANGED_NEW_AREA"then K.keyActive=nil end
    if event=="ZONE_CHANGED_NEW_AREA"or event=="GROUP_ROSTER_UPDATE"or event=="SCENARIO_UPDATE"or event:match("^CHALLENGE_MODE_")then
        if K.UI then K.UI:RefreshDisplays()end;return
    end
    if event=="ADDON_LOADED" then
        if questID=="Blizzard_ObjectiveTracker" and K.UI then K.UI:RefreshDisplays()end
        return
    end
    if event=="PLAYER_REGEN_ENABLED" or event=="DISPLAY_SIZE_CHANGED" or event=="UI_SCALE_CHANGED" then
        if K.UI then K.UI:RefreshDisplays()end;return
    end
    if event=="PLAYER_ENTERING_WORLD" and K.UI then K.UI:RefreshDisplays()end
    -- Login/setting changes validate the hub DB. Quest bursts should not rerun
    -- that validation for every event, especially before a board is ever used.
    if type(LamdaEngineDB)~="table" or LamdaEngineDB.kbbEnabled==false or not K:Board()then return end
    if event=="QUEST_TURNED_IN" and public(questID) and type(questID)=="number" then turned[questID]=true end
    if pending then return end;pending=true
    C_Timer.After(.5,function()
        pending=false;if type(LamdaEngineDB)~="table" or LamdaEngineDB.kbbEnabled==false then turned={};return end
        local b=K:Board();if not b then return end
        if b.autoWatch then K:ImportWatched()end
        -- A turn-in must reach saved quest cards even on an inactive board.
        for _,board in ipairs(K:Collection().boards)do K:RefreshQuests(turned,board)end
        turned={};K:Changed()
    end)
end)
LUI:RegisterModule({id="kbb",name="LamdaKBB",enabledKey="kbbEnabled",settingsScope="global",defaults={kbbEnabled=false,kbbBridgeX=0,kbbBridgeY=224},reloadKeys={"kbbEnabled","kbbBridgeX","kbbBridgeY"},limits={kbbBridgeX={0,16000,integer=true},kbbBridgeY={0,16000,integer=true}},onReset=function()K:ResetDisplays()end,build=function(parent)
    if K.UI then K.UI:BuildSettings(parent)end
end})
