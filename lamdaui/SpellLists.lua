-- Spell selection and a bounded browser of readable active buffs.
-- Browsing never supplies cooldown evidence or changes tracking automatically.
local _,LUI=...
local S={}
LUI.SpellLists=S
S.interrupts={1766,2139,6552,15487,19647,47528,57994,78675,89766,96231,106839,116705,147362,183752,187707,351338}
S.cooldowns={48707,51052,49028,48792,49039,55233,198589,196718,204021,187827,
    22812,102342,61336,363916,363534,357170,374227,374348,186265,109304,264735,110959,
    45438,55342,115203,116849,122470,31850,1022,6940,498,642,86659,633,19236,
    47585,47788,33206,62618,31224,5277,108271,98008,108416,104773,118038,184364,97462,871}
S.defensives={48707,51052,49028,48792,49039,55233,198589,196718,204021,187827,
    22812,102342,61336,363916,357170,374227,374348,186265,264735,110960,45438,55342,
    115203,116849,122470,31850,1022,6940,498,642,86659,19236,47585,47788,33206,
    62618,31224,5277,108271,98008,108416,104773,118038,184364,97462,871}
S.buffs={10060,2825,32182,80353,264667,390386,375087,12472,190319,12042,
    31884,1719,107574,51271,13750,121471,102560,102543,194223,137639,193526,114050,114051}
-- Migrate only the exact shipped starter selections. Empty or customized
-- lists remain authoritative, including deliberate removal of a defensive.
S.legacyCooldowns=table.concat(S.cooldowns,",")
S.legacyDefensives=table.concat(S.defensives,",")
local extraCooldowns={2565,12975,23920,202168,190456}
local extraDefensives={2565,12975,23920,190456}
for _,id in ipairs(extraCooldowns)do S.cooldowns[#S.cooldowns+1]=id end
for _,id in ipairs(extraDefensives)do S.defensives[#S.defensives+1]=id end
S.previousCooldowns=table.concat(S.cooldowns,",")
S.previousDefensives=table.concat(S.defensives,",")
S.cooldowns[#S.cooldowns+1]=342245
S.defensives[#S.defensives+1]=414658
S.defensives[#S.defensives+1]=342246
function S.MigrateDefaults(values)
    if values.defensiveCatalogVersion==2 then return end
    if values.defensiveCatalogVersion~=1 then
        for _,entry in ipairs({{"engineFrameSpells",S.legacyCooldowns,extraCooldowns},
            {"selfCDSpells",S.legacyCooldowns,extraCooldowns},
            {"nativeDefensivesSpells",S.legacyDefensives,extraDefensives}})do
            local key,old,added=entry[1],entry[2],entry[3]
            if values[key]==old or values[key]==old:gsub(",374348","")then
                values[key]=values[key]..","..table.concat(added,",")
            end
        end
    end
    for _,entry in ipairs({{"engineFrameSpells",S.previousCooldowns,"342245"},
        {"selfCDSpells",S.previousCooldowns,"342245"},
        {"nativeDefensivesSpells",S.previousDefensives,"414658,342246"}})do
        local key,old,added=entry[1],entry[2],entry[3]
        if values[key]==old or values[key]==old:gsub(",374348","")then values[key]=values[key]..","..added end
    end
    values.defensiveCatalogVersion=2
end
function S.Valid(value)
    if type(value)~="string" or #value>2048 then return false end
    if value=="" then return true end
    if value:find("[^%d,]") or value:sub(1,1)=="," or value:sub(-1)=="," or value:find(",,",1,true) then return false end
    local seen,count={},0
    for item in value:gmatch("[^,]+")do
        local id=tonumber(item);count=count+1
        if not id or id<1 or id>10000000 or seen[id] or count>128 then return false end
        seen[id]=true
    end
    return true
end
function S.Parse(value)
    local ids={};if not S.Valid(value)then return ids end
    for item in value:gmatch("%d+")do ids[#ids+1]=tonumber(item)end
    return ids
end
function S.Map(value)local map={};for _,id in ipairs(S.Parse(value))do map[id]=true end;return map end
local function publicCall(fn,...)
    if type(fn)~="function"then return end
    local ok,value=pcall(fn,...)
    if ok and not (issecretvalue and issecretvalue(value))then return value end
end
function S.Name(id)
    local name=publicCall(C_Spell and C_Spell.GetSpellName,id)
    return type(name)=="string" and name or "Spell "..id
end
function S.Resolve(query)
    query=query:match("^%s*(.-)%s*$")
    local info=publicCall(C_Spell and C_Spell.GetSpellInfo,tonumber(query) or query)
    if type(info)~="table" then return nil end
    local id=info.spellID
    if issecretvalue and issecretvalue(id)then return nil end
    return type(id)=="number" and id==math.floor(id) and id>0 and id<=10000000 and id or nil
end
function S.Change(key,id,add)
    local db=LUI:DB();local ids=S.Parse(db[key]);local found=false
    for index,value in ipairs(ids)do if value==id then found=true;if not add then table.remove(ids,index)end;break end end
    if add and not found then ids[#ids+1]=id end
    local value=table.concat(ids,",");if not S.Valid(value)then return false end
    db[key]=value;LUI:RefreshUI();return true
end
-- Class tags organize the existing starter catalog; they do not imply learned
-- talents, source ownership of an active aura, or cooldown availability.
S.classes={"Death Knight","Demon Hunter","Druid","Evoker","Hunter","Mage","Monk","Paladin","Priest","Rogue","Shaman","Warlock","Warrior"}
local classSpells={
    ["Death Knight"]={48707,51052,49028,48792,49039,55233,51271},
    ["Demon Hunter"]={198589,196718,204021,187827},
    Druid={22812,102342,61336,102560,102543,194223},
    Evoker={363916,363534,357170,374227,374348,390386,375087},
    Hunter={186265,109304,264735,264667,193526},
    Mage={110959,110960,45438,414658,342245,342246,55342,80353,12472,190319,12042},
    Monk={115203,116849,122470,137639},
    Paladin={31850,1022,6940,498,642,86659,633,31884},
    Priest={19236,47585,47788,33206,62618,10060},
    Rogue={31224,5277,13750,121471},
    Shaman={108271,98008,2825,32182,114050,114051},
    Warlock={108416,104773},Warrior={118038,184364,97462,871,1719,107574,2565,12975,23920,202168,190456},
}
S.metadata={}
for class,ids in pairs(classSpells)do for _,id in ipairs(ids)do S.metadata[id]={class=class}end end
local kickClasses={['Death Knight']={47528},['Demon Hunter']={183752},Druid={78675,106839},Evoker={351338},Hunter={147362,187707},Mage={2139},Monk={116705},Paladin={96231},Priest={15487},Rogue={1766},Shaman={57994},Warlock={19647,89766},Warrior={6552}}
for class,ids in pairs(kickClasses)do for _,id in ipairs(ids)do S.metadata[id]={class=class,Interrupt=true}end end
for tag,ids in pairs({Defensive=S.defensives,Buff=S.buffs,Cooldown=S.cooldowns})do
    for _,id in ipairs(ids)do local info=S.metadata[id] or {};S.metadata[id]=info;info[tag]=true end
end
local function validID(value)
    if issecretvalue and issecretvalue(value)then return nil end
    return type(value)=="number" and value==math.floor(value) and value>0 and value<=10000000 and value or nil
end
function S.Known(id)
    return publicCall(IsPlayerSpell,id)==true or publicCall(IsSpellKnownOrOverridesKnown,id)==true
end
-- Fresh, unsaved snapshots only. Restricted entries are never stringified,
-- compared, indexed or allowed into a list. Nil and unreadable are distinct.
function S.Active(source)
    local result,seen={},{};local incomplete=false
    local units=source=="Active on target" and {"target"} or {"player"}
    if source=="Active in party"then units={"player","party1","party2","party3","party4"}end
    if not C_UnitAuras or type(C_UnitAuras.GetAuraDataByIndex)~="function"then return {},true end
    for _,unit in ipairs(units)do
        local exists=publicCall(UnitExists,unit)
        if exists==nil then incomplete=true end
        if exists==true then
            for index=1,128 do
                local ok,aura=pcall(C_UnitAuras.GetAuraDataByIndex,unit,index,"HELPFUL")
                if not ok or (issecretvalue and issecretvalue(aura))then incomplete=true;break end
                if aura==nil then break end
                if type(aura)~="table"then incomplete=true;break end
                local id=publicCall(function()return validID(aura.spellId)end)
                if id then if not seen[id]then seen[id]=true;result[#result+1]=id end
                else incomplete=true end
                if index==128 then incomplete=true end
            end
        end
    end
    return result,incomplete
end
function S.Filter(ids,query,options)
    options=options or {};query=(query or ""):lower():match("^%s*(.-)%s*$")
    local results={}
    for _,id in ipairs(ids)do
        local meta=S.metadata[id] or {};local class=meta.class or "Unclassified"
        local matchesClass=not options.class or options.class=="All classes" or options.class==class
        local matchesType=not options.category or options.category=="All types" or meta[options.category]
            or (options.category=="Other" and not meta.Defensive and not meta.Buff and not meta.Cooldown and not meta.Interrupt)
        if matchesClass and matchesType and (not options.known or S.Known(id)) and
            (query=="" or tostring(id):find(query,1,true) or S.Name(id):lower():find(query,1,true))then results[#results+1]=id end
    end
    table.sort(results,function(a,b)
        if options.sort=="Spell ID"then return a<b end
        local left,right=S.Name(a):lower(),S.Name(b):lower()
        if left==right then return a<b end
        if options.sort=="Name Z-A"then return left>right end
        return left<right
    end)
    return results
end
function S.Available(key,query,options,active)
    local chosen=S.Map(LUI:DB()[key]);local candidates,seen={},{}
    local function add(id)if not chosen[id] and not seen[id]then candidates[#candidates+1]=id;seen[id]=true end end
    if active then for _,id in ipairs(active)do add(id)end
    else
        for _,list in ipairs(key=="kickSpells" and {S.interrupts} or {S.cooldowns,S.defensives,S.buffs})do for _,id in ipairs(list)do add(id)end end
        local resolved=query~="" and S.Resolve(query)
        if resolved then add(resolved)end
    end
    return S.Filter(candidates,query,options)
end
function LUI:SpellPicker(parent,key,x,y)
    x=x or 0;y=y or 0
    local panel=CreateFrame("Frame",nil,parent);panel:SetPoint("TOPLEFT",x,y);panel:SetSize(600,260)
    local function currentKey()return type(key)=="function" and key() or key end
    local options={class="All classes",category="All types",sort="Name A-Z",source="Suggested spells"}
    panel.filters=options
    self:Label(panel,"Search both lists: name or spell ID",4,0)
    self:Label(panel,"Available spell source",314,0)
    local search=CreateFrame("EditBox",nil,panel,"InputBoxTemplate")
    search:SetPoint("TOPLEFT",10,-21);search:SetSize(278,24);search:SetAutoFocus(false);search:SetMaxLetters(80)
    self:Explain(search,"Find a spell","Search tracked and available spells together. Enter an exact spell name or ID to add a spell outside the starter catalog. Active sources use buff IDs, which can differ from ability IDs.")
    local offset={0,0};local rows={{},{}};local bars={};local updating=false;local lastKey
    local counts={self:Label(panel,"",4,-89),self:Label(panel,"",314,-89)}
    local message=self:Description(panel,"",4,-236,592)
    local active,incomplete,dirty=nil,false,true
    local function activeSource()return options.source:sub(1,6)=="Active"end
    local function draw()
        local current=currentKey();if current~=lastKey then offset={0,0};lastKey=current end
        if dirty and activeSource() and panel:IsVisible()then active,incomplete=S.Active(options.source);dirty=false end
        options.known=options.source=="My build"
        local query=search:GetText() or "";local selected=S.Parse(LUI:DB()[current])
        local candidates=S.Available(current,query,options,activeSource() and (active or {}) or nil)
        local lists={S.Filter(selected,query,options),candidates}
        panel.filtered=lists
        counts[1]:SetText("Tracked: "..#lists[1].." / "..#selected.."  (- removes)")
        counts[2]:SetText("Available: "..#lists[2].."  (+ adds)")
        local notice="Filters only browse lists; your tracking stays unchanged."
        if activeSource()then notice=incomplete and "Some active buffs are unreadable; these results are incomplete." or "Active buffs update while this page is open. Filters do not change tracking."
        elseif options.source=="My build"then notice="My build: learned ability IDs for your current talents. Buff IDs can differ."
        elseif #selected==0 then notice="Nothing tracked: this group shows no icons."end
        message:SetText(notice)
        for column=1,2 do
            offset[column]=math.max(0,math.min(offset[column],math.max(0,#lists[column]-4)))
            updating=true;bars[column]:SetMinMaxValues(0,math.max(0,#lists[column]-4));bars[column]:SetValue(offset[column]);updating=false
            bars[column]:SetShown(#lists[column]>4)
            for index=1,4 do
                local row=rows[column][index];local id=lists[column][offset[column]+index]
                row:SetShown(id~=nil)
                if id then
                    row:SetText((column==1 and "- " or "+ ")..S.Name(id));row.spellID=id;row.idLabel:SetText(id)
                    row:SetScript("OnClick",function()S.Change(current,id,column==2)end)
                end
            end
        end
    end
    local function filter(field,value)
        options[field]=value;offset={0,0}
        if field=="source"then active=nil;incomplete=false;dirty=true end
        draw()
    end
    panel.source=self:Dropdown(panel,314,-20,282,function()return {"Suggested spells","My build","Active on me","Active on target","Active in party"}end,
        function()return options.source end,function(value)filter("source",value)end)
    self:Explain(panel.source,"Available spell source","Browse suggestions, learned abilities in your current build, or currently readable buffs. Active in party includes you and up to four party members. Unreadable buffs are reported as incomplete, not missing. Tracking is never changed by this filter.")
    panel.class=self:Dropdown(panel,4,-54,192,function()
        local names={"All classes"};for _,name in ipairs(S.classes)do names[#names+1]=name end;names[#names+1]="Unclassified";return names
    end,function()return options.class end,function(value)filter("class",value)end)
    self:Explain(panel.class,"Class filter","Filter starter spells by class. Added spells without a class tag remain under All classes or Unclassified. A buff's recipient is not necessarily its casting class.")
    panel.category=self:Dropdown(panel,204,-54,192,function()return {"All types","Defensive","Buff","Cooldown","Interrupt","Other"}end,
        function()return options.category end,function(value)filter("category",value)end)
    self:Explain(panel.category,"Spell type","Filter by membership in Lamda's defensive, helpful-buff or cooldown starter lists. Added spells without tags are Other. This does not change which effects are tracked.")
    panel.sort=self:Dropdown(panel,404,-54,192,function()return {"Name A-Z","Name Z-A","Spell ID"}end,
        function()return options.sort end,function(value)filter("sort",value)end)
    self:Explain(panel.sort,"Sort both lists","Sort these browsing lists only. Your saved spell priority and icon ordering stay unchanged.")
    for column=1,2 do
        local which=column
        local bar=CreateFrame("Slider",nil,panel,"BackdropTemplate");bars[column]=bar
        bar:SetPoint("TOPLEFT",(column-1)*310+276,-113);bar:SetSize(12,113);bar:SetOrientation("VERTICAL")
        bar:SetThumbTexture("Interface\\Buttons\\UI-ScrollBar-Knob");bar:SetValueStep(1);bar:SetObeyStepOnDrag(true)
        bar:SetScript("OnValueChanged",function(_,value)if not updating then offset[which]=math.floor(value+.5);draw()end end)
    end
    for column=1,2 do for index=1,4 do
        local which=column
        local row=self:Button(panel,"",(column-1)*310+4,-113-(index-1)*29,266,function()end)
        row.label:ClearAllPoints();row.label:SetPoint("LEFT",8,0);row.label:SetWidth(192);row.label:SetJustifyH("LEFT")
        row.idLabel=row:CreateFontString(nil,"OVERLAY","GameFontNormalSmall");row.idLabel:SetPoint("RIGHT",-6,0);row.idLabel:SetTextColor(.55,.68,.77)
        rows[column][index]=row
        row:EnableMouseWheel(true);row:SetScript("OnMouseWheel",function(_,delta)offset[which]=offset[which]-delta;draw()end)
        row:SetScript("OnEnter",function(self)
            if GameTooltip then GameTooltip:SetOwner(self,"ANCHOR_RIGHT");GameTooltip:SetSpellByID(self.spellID);GameTooltip:Show()end
        end)
        row:SetScript("OnLeave",function()if GameTooltip then GameTooltip:Hide()end end)
    end end
    search:SetScript("OnTextChanged",function()offset={0,0};draw()end)
    search:SetScript("OnEscapePressed",function(self)self:ClearFocus()end)
    for _,event in ipairs({"UNIT_AURA","PLAYER_TARGET_CHANGED","GROUP_ROSTER_UPDATE","PLAYER_ENTERING_WORLD","PLAYER_SPECIALIZATION_CHANGED","SPELLS_CHANGED","PLAYER_REGEN_ENABLED","PLAYER_REGEN_DISABLED"})do panel:RegisterEvent(event)end
    panel:SetScript("OnEvent",function()dirty=true end)
    local elapsed=0
    panel:SetScript("OnUpdate",function(_,delta)
        elapsed=elapsed+delta
        if elapsed>=.5 then elapsed=0
            if dirty and panel:IsVisible() and (activeSource() or options.source=="My build")then draw();dirty=false end
        end
    end)
    self:OnRefresh(panel,draw)
    panel:HookScript("OnShow",function()dirty=true;draw()end)
    panel:HookScript("OnHide",function()active=nil;incomplete=false;dirty=true end)
    panel.search=search;panel.rows=rows;panel.notice=message;panel.SetFilter=filter
    return panel
end
