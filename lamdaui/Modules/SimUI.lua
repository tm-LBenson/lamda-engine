-- Native Lamda Sim workspace. Calculations remain in the companion engine;
-- imported results are immutable until the user reloads the UI.
local _,LUI=...
local S=LUI.Sim
if not S then return end
local U={panels={},selected={},talentSelected={},crestRanks={},loadout={},page=1,crestPage=1,historyPage=1,upgradePage=1}
LUI.SimUI=U;S.GameUI=U
local backdrop={bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1}
local panelTabs={{"overview","Setup"},{"healing","Healing Lab"},{"meta","Builds"},{"advisor","Quick Advisor"},{"summary","Results"},{"loadout","Loadout"},{"crest","Crests"},{"gear","Bags"},{"talents","Talents"},{"vault","Vault"},{"upgrades","Drops"},{"history","Runs"}}
local mainTabs={{"overview","Setup"},{"summary","Results"},{"gear","Gear"},{"upgrades","Drops"},{"meta","Builds"},{"healing","Healing Lab"},{"history","Runs"}}
local gearTabs={{"gear","Bags"},{"vault","Vault"},{"crest","Crests"},{"talents","Talents"},{"loadout","Loadout"},{"advisor","Quick Advisor"}}
local primaryTabs={advisor="gear",loadout="gear",crest="gear",talents="gear",vault="gear"}
local slotNames={head="Head",neck="Neck",shoulder="Shoulders",back="Back",chest="Chest",wrist="Wrists",hands="Hands",waist="Waist",legs="Legs",feet="Feet",finger1="Rings",finger2="Rings",trinket1="Trinkets",trinket2="Trinkets",main_hand="Main hand",off_hand="Off hand"}
local sheetSlotNames={head="Head",neck="Neck",shoulder="Shoulders",back="Back",chest="Chest",shirt="Shirt",tabard="Tabard",wrist="Wrists",hands="Hands",waist="Waist",legs="Legs",feet="Feet",finger1="Ring 1",finger2="Ring 2",trinket1="Trinket 1",trinket2="Trinket 2",main_hand="Main hand",off_hand="Off hand"}
local sheetLeft={"head","neck","shoulder","back","chest","wrist"}
local sheetRight={"hands","waist","legs","feet","finger1","finger2","trinket1","trinket2"}
local slotOptions={"All slots","Head","Neck","Shoulders","Back","Chest","Wrists","Hands","Waist","Legs","Feet","Rings","Trinkets","Main hand","Off hand"}
local raidNames={[17]="Raid Finder",[14]="Normal",[15]="Heroic",[16]="Mythic"}
local raidTiers={[17]=0,[14]=1,[15]=2,[16]=3}
local buildSpecs={
    [250]={class="DEATHKNIGHT",className="Death Knight",name="Blood",healer=false},
    [251]={class="DEATHKNIGHT",className="Death Knight",name="Frost",healer=false},
    [252]={class="DEATHKNIGHT",className="Death Knight",name="Unholy",healer=false},
    [577]={class="DEMONHUNTER",className="Demon Hunter",name="Havoc",healer=false},
    [581]={class="DEMONHUNTER",className="Demon Hunter",name="Vengeance",healer=false},
    [1480]={class="DEMONHUNTER",className="Demon Hunter",name="Devourer",healer=false},
    [102]={class="DRUID",className="Druid",name="Balance",healer=false},
    [103]={class="DRUID",className="Druid",name="Feral",healer=false},
    [104]={class="DRUID",className="Druid",name="Guardian",healer=false},
    [105]={class="DRUID",className="Druid",name="Restoration",healer=true},
    [1467]={class="EVOKER",className="Evoker",name="Devastation",healer=false},
    [1468]={class="EVOKER",className="Evoker",name="Preservation",healer=true},
    [1473]={class="EVOKER",className="Evoker",name="Augmentation",healer=false},
    [253]={class="HUNTER",className="Hunter",name="Beast Mastery",healer=false},
    [254]={class="HUNTER",className="Hunter",name="Marksmanship",healer=false},
    [255]={class="HUNTER",className="Hunter",name="Survival",healer=false},
    [62]={class="MAGE",className="Mage",name="Arcane",healer=false},
    [63]={class="MAGE",className="Mage",name="Fire",healer=false},
    [64]={class="MAGE",className="Mage",name="Frost",healer=false},
    [268]={class="MONK",className="Monk",name="Brewmaster",healer=false},
    [270]={class="MONK",className="Monk",name="Mistweaver",healer=true},
    [269]={class="MONK",className="Monk",name="Windwalker",healer=false},
    [65]={class="PALADIN",className="Paladin",name="Holy",healer=true},
    [66]={class="PALADIN",className="Paladin",name="Protection",healer=false},
    [70]={class="PALADIN",className="Paladin",name="Retribution",healer=false},
    [256]={class="PRIEST",className="Priest",name="Discipline",healer=true},
    [257]={class="PRIEST",className="Priest",name="Holy",healer=true},
    [258]={class="PRIEST",className="Priest",name="Shadow",healer=false},
    [259]={class="ROGUE",className="Rogue",name="Assassination",healer=false},
    [260]={class="ROGUE",className="Rogue",name="Outlaw",healer=false},
    [261]={class="ROGUE",className="Rogue",name="Subtlety",healer=false},
    [262]={class="SHAMAN",className="Shaman",name="Elemental",healer=false},
    [263]={class="SHAMAN",className="Shaman",name="Enhancement",healer=false},
    [264]={class="SHAMAN",className="Shaman",name="Restoration",healer=true},
    [265]={class="WARLOCK",className="Warlock",name="Affliction",healer=false},
    [266]={class="WARLOCK",className="Warlock",name="Demonology",healer=false},
    [267]={class="WARLOCK",className="Warlock",name="Destruction",healer=false},
    [71]={class="WARRIOR",className="Warrior",name="Arms",healer=false},
    [72]={class="WARRIOR",className="Warrior",name="Fury",healer=false},
    [73]={class="WARRIOR",className="Warrior",name="Protection",healer=false},
}
local specs={};for id,spec in pairs(buildSpecs)do specs[id]=spec.name end
U.BuildSpecs=buildSpecs

-- A deliberately small, readable gearing model. These priorities are a
-- maintained starting point for Patch 12.1 rather than hidden pseudo-HPS/DPS.
-- Primary stats retain a large lead; secondaries express current guide order.
-- The UI shows the inputs, date, source and every limitation beside the answer.
local advisorProfiles={
    [62]={name="Arcane",primary="intellect",raid={"haste","vers","crit","mastery"},dungeon={"haste","vers","crit","mastery"}},
    [63]={name="Fire",primary="intellect",raid={"haste","vers","mastery","crit"},dungeon={"haste","mastery","vers","crit"}},
    [64]={name="Frost",primary="intellect",raid={"mastery","crit","haste","vers"},dungeon={"mastery","crit","haste","vers"}},
    [65]={name="Holy Paladin",primary="intellect",raid={"mastery","haste","crit","vers"},dungeon={"haste","vers","crit","mastery"},note="Mastery loses value when dungeon healing happens at range."},
    [66]={name="Protection Paladin",primary="strength",tank=true,balanced={"haste","crit","vers","mastery"},survival={"haste","vers","crit","mastery"},damage={"haste","crit","vers","mastery"}},
    [70]={name="Retribution",primary="strength",raid={"mastery","haste","crit","vers"},dungeon={"mastery","haste","crit","vers"}},
    [71]={name="Arms",primary="strength",weapon=true,raid={"crit","haste","mastery","vers"},dungeon={"crit","haste","mastery","vers"}},
    [72]={name="Fury",primary="strength",weapon=true,raid={"mastery","haste","crit","vers"},dungeon={"mastery","haste","crit","vers"}},
    [73]={name="Protection Warrior",primary="strength",weapon=true,tank=true,balanced={"haste","crit","vers","mastery"},survival={"haste","vers","crit","mastery"},damage={"haste","crit","vers","mastery"}},
    [102]={name="Balance",primary="intellect",raid={"mastery","haste","crit","vers"},dungeon={"mastery","haste","crit","vers"}},
    [103]={name="Feral",primary="agility",weapon=true,raid={"crit","haste","mastery","vers"},dungeon={"crit","haste","mastery","vers"},equal=true,note="All four secondaries are close; use the combat sim for final DPS choices."},
    [104]={name="Guardian",primary="agility",weapon=true,tank=true,balanced={"haste","vers","crit","mastery"},survival={"vers","haste","mastery","crit"},damage={"haste","vers","crit","mastery"}},
    [105]={name="Restoration Druid",primary="intellect",raid={"haste","mastery","crit","vers"},dungeon={"haste","mastery","vers","crit"}},
    [250]={name="Blood",primary="strength",weapon=true,tank=true,balanced={"crit","vers","mastery","haste"},survival={"vers","mastery","crit","haste"},damage={"crit","mastery","vers","haste"},note="Blood's best spread changes sharply between Deathbringer and San'layn; treat close items as unresolved."},
    [251]={name="Frost",primary="strength",weapon=true,raid={"crit","mastery","haste","vers"},dungeon={"crit","mastery","haste","vers"}},
    [252]={name="Unholy",primary="strength",weapon=true,raid={"crit","mastery","haste","vers"},dungeon={"crit","mastery","haste","vers"}},
    [253]={name="Beast Mastery",primary="agility",weapon=true,raid={"mastery","haste","crit","vers"},dungeon={"mastery","crit","vers","haste"}},
    [254]={name="Marksmanship",primary="agility",weapon=true,raid={"crit","mastery","vers","haste"},dungeon={"crit","mastery","vers","haste"}},
    [255]={name="Survival",primary="agility",weapon=true,raid={"mastery","crit","haste","vers"},dungeon={"mastery","haste","crit","vers"}},
    [256]={name="Discipline",primary="intellect",raid={"haste","mastery","crit","vers"},dungeon={"haste","mastery","crit","vers"},note="The current guide gives build-specific Haste rating targets; this model does not guess your hero build."},
    [257]={name="Holy Priest",primary="intellect",raid={"crit","mastery","vers","haste"},dungeon={"haste","vers","crit","mastery"}},
    [258]={name="Shadow",primary="intellect",raid={"haste","mastery","crit","vers"},dungeon={"haste","mastery","crit","vers"}},
    [259]={name="Assassination",primary="agility",weapon=true,raid={"crit","haste","mastery","vers"},dungeon={"crit","haste","mastery","vers"}},
    [260]={name="Outlaw",primary="agility",weapon=true,raid={"haste","crit","vers","mastery"},dungeon={"haste","crit","vers","mastery"},target="About 25% Haste is a comfort point, not a cap."},
    [261]={name="Subtlety",primary="agility",weapon=true,raid={"mastery","haste","crit","vers"},dungeon={"mastery","haste","crit","vers"},note="Deathstalker and Trickster use different Haste targets; close items need a build-specific sim."},
    [262]={name="Elemental",primary="intellect",raid={"mastery","haste","crit","vers"},dungeon={"mastery","haste","crit","vers"}},
    [263]={name="Enhancement",primary="agility",weapon=true,raid={"mastery","haste","crit","vers"},dungeon={"mastery","haste","crit","vers"}},
    [264]={name="Restoration Shaman",primary="intellect",raid={"crit","vers","haste","mastery"},dungeon={"crit","vers","haste","mastery"}},
    [265]={name="Affliction",primary="intellect",raid={"crit","haste","mastery","vers"},dungeon={"haste","crit","mastery","vers"}},
    [266]={name="Demonology",primary="intellect",raid={"crit","haste","mastery","vers"},dungeon={"crit","haste","mastery","vers"}},
    [267]={name="Destruction",primary="intellect",raid={"haste","crit","mastery","vers"},dungeon={"haste","crit","mastery","vers"}},
    [268]={name="Brewmaster",primary="agility",weapon=true,tank=true,balanced={"crit","vers","mastery","haste"},survival={"vers","crit","mastery","haste"},damage={"crit","vers","mastery","haste"}},
    [269]={name="Windwalker",primary="agility",weapon=true,raid={"haste","mastery","crit","vers"},dungeon={"haste","mastery","crit","vers"}},
    [270]={name="Mistweaver",primary="intellect",raid={"haste","crit","vers","mastery"},dungeon={"haste","crit","vers","mastery"}},
    [577]={name="Havoc",primary="agility",weapon=true,raid={"crit","mastery","haste","vers"},dungeon={"crit","mastery","haste","vers"}},
    [581]={name="Vengeance",primary="agility",weapon=true,tank=true,balanced={"haste","crit","vers","mastery"},survival={"haste","vers","crit","mastery"},damage={"haste","crit","mastery","vers"}},
    [1480]={name="Devourer",primary="intellect",weapon=true,raid={"mastery","haste","crit","vers"},dungeon={"mastery","haste","crit","vers"}},
    [1467]={name="Devastation",primary="intellect",raid={"crit","haste","mastery","vers"},dungeon={"crit","haste","mastery","vers"}},
    [1468]={name="Preservation",primary="intellect",raid={"mastery","crit","haste","vers"},dungeon={"mastery","crit","haste","vers"},target="Around 20% Haste is a comfort point. Mastery starts diminishing near 1,380 rating; neither is a hard cap.",note="Raid favors Leech and dungeons favor Avoidance when otherwise close."},
    [1473]={name="Augmentation",primary="intellect",raid={"mastery","haste","crit","vers"},dungeon={"mastery","haste","crit","vers"}},
}
local advisorStatKeys={
    intellect={"ITEM_MOD_INTELLECT_SHORT","ITEM_MOD_INTELLECT"},agility={"ITEM_MOD_AGILITY_SHORT","ITEM_MOD_AGILITY"},strength={"ITEM_MOD_STRENGTH_SHORT","ITEM_MOD_STRENGTH"},
    crit={"ITEM_MOD_CRIT_RATING_SHORT","ITEM_MOD_CRIT_RATING"},haste={"ITEM_MOD_HASTE_RATING_SHORT","ITEM_MOD_HASTE_RATING"},mastery={"ITEM_MOD_MASTERY_RATING_SHORT","ITEM_MOD_MASTERY_RATING"},
    vers={"ITEM_MOD_VERSATILITY","ITEM_MOD_VERSATILITY_SHORT"},stamina={"ITEM_MOD_STAMINA_SHORT","ITEM_MOD_STAMINA"},armor={"ITEM_MOD_ARMOR_SHORT","ITEM_MOD_ARMOR"},
    weapon={"ITEM_MOD_DAMAGE_PER_SECOND_SHORT","ITEM_MOD_DAMAGE_PER_SECOND"},leech={"ITEM_MOD_CR_LIFESTEAL_SHORT","ITEM_MOD_LIFESTEAL_SHORT"},avoidance={"ITEM_MOD_CR_AVOIDANCE_SHORT","ITEM_MOD_AVOIDANCE_SHORT"},
    socket={"EMPTY_SOCKET_PRISMATIC","EMPTY_SOCKET_RED","EMPTY_SOCKET_YELLOW","EMPTY_SOCKET_BLUE","EMPTY_SOCKET_META"},
}
local advisorStatNames={intellect="Intellect",agility="Agility",strength="Strength",crit="Crit",haste="Haste",mastery="Mastery",vers="Versatility",stamina="Stamina",armor="Armor",weapon="weapon DPS",leech="Leech",avoidance="Avoidance",socket="socket"}
local function clean(value,max)
    if type(value)~="string" then return "" end
    return value:gsub("|","||"):gsub("[%c]"," "):sub(1,max or 160)
end
local function friendlyStatus(value,max)
    local text=clean(value,max)
    return text:gsub("Check for results","Load finished results"):gsub("check for results","load finished results")
end
local function safe(fn,...)
    if type(fn)~="function" then return end
    local ok,a,b,c,d,e,f,g,h,i,j,k=pcall(fn,...)
    if ok then return a,b,c,d,e,f,g,h,i,j,k end
end
local function num(value)
    return type(value)=="number" and value==value and value~=math.huge and value~=-math.huge and value or nil
end
local function fmt(value)
    value=num(value);if not value then return "—" end
    local text=string.format(math.abs(value)>=1000 and "%.0f" or "%.1f",value)
    return BreakUpLargeNumbers and BreakUpLargeNumbers(text) or text
end
local function gain(value)
    value=num(value);if not value then return "—" end
    if value~=0 and math.abs(value)<.01 then return value>0 and "+<0.01%" or "-<0.01%" end
    return string.format("%+.2f%%",value)
end
local function metric(result)
    return result and ({group_dps="Group DPS",healing_score="Healing score",healing_hps="Effective HPS",build_reference="Build reference",item_stats="Item stats"})[result.metric] or "DPS"
end
local function throughputUnit(result)
    if result and result.metric=="group_dps" then return "group DPS" end
    if result and result.metric=="healing_score" then return "score" end
    if result and result.metric=="healing_hps" then return "HPS" end
    if result and result.metric=="build_reference" then return "reference" end
    return "DPS"
end
local function signed(value)
    value=num(value);if not value then return "—" end
    return (value>0 and "+" or "")..fmt(value)
end
local function comparisonDecision(reference,comparison)
    local baseline=reference and num(reference.mean);local mean=comparison and num(comparison.mean)
    if not baseline or baseline==0 or not mean then return end
    local absolute=mean-baseline;local percent=absolute/baseline*100
    local baseError=num(reference.meanError)or 0;local comparisonError=num(comparison.meanError)or 0
    local uncertain=baseError>0 and comparisonError>0 and math.abs(absolute)<=baseError+comparisonError
    local verdict=uncertain and "Too close to call" or absolute>0 and "Gain" or absolute<0 and "Loss" or "No change"
    return {absolute=absolute,percent=percent,mean=mean,uncertain=uncertain,verdict=verdict}
end
local function decisionColor(widget,value,estimate,modelDependent)
    if not widget or not widget.SetTextColor then return end
    if not value then widget:SetTextColor(.58,.7,.78)
    elseif modelDependent or value.uncertain or value.percent==0 then widget:SetTextColor(1,.82,.26)
    elseif value.percent>0 then widget:SetTextColor(.3,1,.5)
    else widget:SetTextColor(1,.28,.28) end
end
local function decisionBorder(widget,value,estimate,modelDependent)
    if not widget or not widget.SetBackdropBorderColor then return end
    if not value then widget:SetBackdropBorderColor(.12,.2,.25,.5)
    elseif modelDependent or value.uncertain or value.percent==0 then widget:SetBackdropBorderColor(.72,.52,.12,.9)
    elseif value.percent>0 then widget:SetBackdropBorderColor(.16,.8,.42,.9)
    else widget:SetBackdropBorderColor(.82,.18,.18,.9) end
end
local function decision(result,comparison)return comparisonDecision(result and result.baseline,comparison)end
local function decisionText(result,comparison)
    local value=decision(result,comparison);if not value then return "Comparison result unavailable",nil end
    local text=value.verdict.." · "..signed(value.absolute).." "..throughputUnit(result).." ("..gain(value.percent)..")"
    if value.uncertain then text=text.." · within simulation error" end
    return text.." · "..fmt(value.mean).." total",value
end
local function itemKey(item)
    if not item then return end
    local id=item.id or item.itemID
    if id then return "id:"..tostring(id) end
    local value=item.label or item.name
    if value then return "name:"..tostring(value) end
end
local function replacedItem(reference,comparison)
    if type(reference)~="table"or type(reference.items)~="table"or type(comparison)~="table"or type(comparison.items)~="table"then return end
    local remaining={}
    for _,item in ipairs(comparison.items)do local key=itemKey(item);if key then remaining[key]=(remaining[key]or 0)+1 end end
    for _,item in ipairs(reference.items)do
        local key=itemKey(item)
        if key and(remaining[key]or 0)>0 then remaining[key]=remaining[key]-1
        elseif key then return item end
    end
end
local function replacementText(reference,comparison)
    local item=replacedItem(reference,comparison);if not item then return end
    local text=clean(item.label or item.name or "equipped item",50);local level=item.level or item.itemLevel
    if level then text=text.." ("..tostring(level)..")" end
    return text
end
local function stamp(value)
    if type(value)=="number" and date then return date("%b %d, %H:%M",value) end
    if type(value)=="string" then
        local zone=value:match("Z$") and " UTC" or value:match("([+-]%d%d:%d%d)$")
        return clean(value:gsub("T"," "):sub(1,16))..(zone and(zone:sub(1,1)==" " and zone or " UTC"..zone)or "")
    end
    return "Unknown time"
end
local function settings()
    if type(LamdaUIDB)~="table" then LamdaUIDB={} end
    if type(LamdaUIDB.simUI)~="table" then LamdaUIDB.simUI={} end
    local db=LamdaUIDB.simUI
    if type(db.scenario)~="table" then db.scenario={} end
    if type(db.builds)~="table" then db.builds={} end
    return db
end
local function resultDisplayMode()
    return settings().resultDisplay=="absolute" and "absolute" or "percent"
end
local function resultGainText(result,value,estimate)
    if not value then return "\226\128\148" end
    local text
    if resultDisplayMode()=="absolute" and num(value.absolute) then text=signed(value.absolute).." "..throughputUnit(result)
    else text=gain(value.percent) end
    if value.uncertain then text="\226\137\136 "..text end
    if estimate then text=text.." est." end
    return text
end
local function specName(character)
    character=character or {};local id=character.spec or 0
    local api=C_SpecializationInfo and C_SpecializationInfo.GetSpecializationInfoByID or GetSpecializationInfoByID
    local _,name=safe(api,id)
    return clean(name or specs[id] or (id>0 and "Spec "..id or character.class or "Character"))
end
local function label(parent,text,x,y,width,large)
    local f=LUI:Description(parent,text,x,y,width);if large and f.SetFontObject then f:SetFontObject("GameFontNormalLarge") end
    f:SetTextColor(.86,.91,.95);return f
end
local function card(parent,x,y,w,h)
    local f=CreateFrame("Frame",nil,parent,"BackdropTemplate");f:SetPoint("TOPLEFT",x,y);f:SetSize(w,h)
    f:SetBackdrop(backdrop);f:SetBackdropColor(.045,.08,.115,.98);f:SetBackdropBorderColor(.16,.24,.30,1);return f
end
local function edit(parent,x,y,w,text,max)
    local f=CreateFrame("EditBox",nil,parent,"InputBoxTemplate");f:SetSize(w,26);f:SetPoint("TOPLEFT",x,y)
    f:SetAutoFocus(false);f:SetMaxLetters(max or 128);f:SetText(text or "")
    f:SetScript("OnEscapePressed",function(self)self:ClearFocus()end)
    f:SetScript("OnEnterPressed",function(self)self:ClearFocus()end);return f
end
local function button(parent,text,x,y,w,fn,help)
    local b=LUI:Button(parent,text,x,y,w,fn);if help then LUI:Explain(b,text,help) end;return b
end
local function resultUnits(panel)
    panel.units=button(panel,"Show: %",752,-3,178,function()
        settings().resultDisplay=resultDisplayMode()=="absolute"and"percent"or"absolute";U:Refresh()
    end,"Switches simulated gains between percent and raw DPS, Group DPS, HPS or score. It changes only how results are displayed.")
end
local function dropdown(parent,x,y,w,choices,get,set)
    return LUI:Dropdown(parent,x,y,w,function()return type(choices)=="function"and choices()or choices end,get,function(v)set(v);U:Refresh()end)
end
local function setText(widget,value)if widget then widget:SetText(value or "")end end
local function reloadPending()
    return type(S.IsReloadPending)=="function" and S:IsReloadPending() or false
end
local function transferBusy()
    return U.startToken~=nil or S.packets~=nil or S.scan~=nil or S.reloadAttemptToken~=nil or reloadPending()
end
local function identity(candidate)
    local i=candidate.identity or candidate
    return table.concat({tostring(i.source or candidate.source or ""),tostring(i.slot or candidate.slot or ""),tostring(i.value or candidate.value or ""),tostring(i.key or ""),tostring(candidate.id or "")},"\30")
end
local function normalize(view)
    if type(view)~="table" then return nil end
    if type(view.snapshot)=="table" then return view end
    return {snapshot=view,candidates=view.candidates or {},equipped=view.equipped or {},talents=view.talents or {},capability=view.capability}
end
function U:Envelope()
    return type(LamdaSimResults)=="table" and LamdaSimResults.schema==1 and LamdaSimResults or {snapshots={},jobs={}}
end
function U:SummaryFor(job)
    for _,summary in ipairs(self:Envelope().snapshots or {}) do if summary.snapshot and summary.snapshot.id==job.snapshotId then return summary end end
end
function U:Current(force)
    if force or not self.current then
        if InCombatLockdown and InCombatLockdown() then
            self.capturePending=true
            if force then
                self.captureProblem="Waiting for combat to end. Character data will refresh automatically."
                self.notice=self.captureProblem
            end
            return self.current
        end
        self.capturePending=nil
        local ok,view,problem=pcall(S.GameSnapshot or function()end,S,force)
        if ok and type(view)=="table" then
            self.current=normalize(view)
            if self.notice==self.captureProblem then self.notice=nil end
            self.captureProblem=nil
            if force then self.selected={};self.page=1 end
        elseif force then
            self.captureProblem=friendlyStatus(tostring(ok and problem or view):gsub("^.-%.lua:%d+:%s*",""),400)
            self.notice=self.captureProblem
        end
    end
    return self.current
end
function U:SameCharacter(summary)
    local a=self.current and self.current.snapshot.character;local b=summary and summary.snapshot.character
    if not a or not b then return false end
    return type(a.guid)=="string" and a.guid~="" and a.guid==b.guid and a.spec==b.spec
end
local function taskOf(job)
    if job.gameTask then return job.gameTask end
    local scenario=job.scenario or {}
    if scenario.task=="healing" or scenario.task=="builds" then return scenario.task end
    if scenario.task=="upgrades"or scenario.task=="analysis"then return scenario.task end
    if scenario.talents and #scenario.talents>0 then return "talents" end
    if job.selected and #job.selected>0 then return "compare" end
    return "baseline"
end
function U:Jobs(all)
    local jobs={}
    for _,job in ipairs(self:Envelope().jobs or {}) do
        if all or self:SameCharacter(self:SummaryFor(job)) then jobs[#jobs+1]=job end
    end
    table.sort(jobs,function(a,b)return tostring(a.createdAt or "")>tostring(b.createdAt or "")end)
    return jobs
end
function U:Job(task,source)
    local jobs=self:Jobs(false)
    local function matches(job)
        if task and taskOf(job)~=task then return false end
        if not source then return true end
        local summary=self:SummaryFor(job);local request=summary and summary.snapshot.gameRequest
        if request and request.selected then
            for _,item in ipairs(request.selected)do if item.source==source then return true end end
            return false
        end
        for _,id in ipairs(job.selected or {})do for _,item in ipairs(summary and summary.candidates or {})do if item.id==id and item.source==source then return true end end end
        return false
    end
    if task=="upgrades" then
        for _,selected in ipairs(jobs)do if selected.id==self.selectedJobID and selected.plan and selected.plan.jobID then
            for _,job in ipairs(jobs)do if job.id==selected.plan.jobID and matches(job)then return job end end
        end end
    end
    for _,job in ipairs(jobs) do if job.id==self.selectedJobID and matches(job) then return job end end
    local pending=LUI:DB().simPendingRequestID
    for _,job in ipairs(jobs)do if pending and pending~="" and job.requestID==pending and matches(job)then return job end end
    for _,job in ipairs(jobs) do if matches(job) then return job end end
end
local function advisorBuildReport(spec)
    for _,job in ipairs(U:Jobs(true))do
        local report=job.result and job.result.builds
        if report and tonumber(report.spec)==tonumber(spec)and report.mode=="m+"and type(report.stats)=="table"and#report.stats==4 then return report end
    end
end
function U:Capability()
    local current=self:Current();if not current then return false,"Update your character to begin." end
    if current.capability and current.capability.available==true then return true,current.capability.reason end
    for _,summary in ipairs(self:Envelope().snapshots or {}) do
        if self:SameCharacter(summary) and summary.capability and summary.capability.available==true then return true,summary.capability.reason end
    end
    if current.snapshot.client=="forever" then return true,"Item-stat comparisons only." end
    if current.snapshot.profile=="" then return false,"Enable Simulationcraft, then update your character." end
    return true,"The engine will check support for this character."
end
function U:Scenario()
    local scenario=settings().scenario
    return {targets=tonumber(scenario.targets) or 1,seconds=tonumber(scenario.seconds) or 300,iterations=tonumber(scenario.iterations) or 1000,
        augmentationMode=scenario.augmentationMode=="raid" and "raid" or "dungeon",allyItemLevel=tonumber(scenario.allyItemLevel) or 328,
        allySkill=tonumber(scenario.allySkill) or .9,healingMode=scenario.healingMode=="raid" and "raid" or "dungeon",
        fightStyle=scenario.fightStyle=="dungeon_slice"and"dungeon_slice"or"patchwerk",raidBuffs=scenario.raidBuffs=="full"and"full"or"none"}
end
function U:CopyWorkflow(value)
    if type(value)~="table"then return value end
    local copy={};for k,v in pairs(value)do copy[k]=self:CopyWorkflow(v)end;return copy
end
function U:WorkflowItemKey(item)
    local i=item.identity or item
    return table.concat({i.source or item.source or"",i.slot or item.slot or"",i.value or item.value or""},"\30")
end
function U:WorkflowSettings()
    local current=self:Current();local c=current and current.snapshot.character or{}
    local db=settings();db.workflows=db.workflows or{};local context=db.workflowContext=="raid"and"raid"or"m+"
    local key=table.concat({current and current.snapshot.client or"",c.guid or"",tostring(c.spec or 0),context},":")
    if not db.workflows[key]then
        local record={enabled=false,selected={},context=context}
        for _,entry in ipairs(self:Envelope().workflows or{})do
            if entry.characterGUID==c.guid and entry.spec==c.spec and entry.settings and entry.settings.context==context then
                record={enabled=true,selected=self:CopyWorkflow(entry.settings.selected or{}),context=context,encounters=self:CopyWorkflow(entry.settings.scenario and entry.settings.scenario.encounters)};break
            end
        end
        db.workflows[key]=record
    end
    return db.workflows[key],key
end
function U:WorkflowItems()
    local current=self:Current()or{};local out={}
    for _,items in ipairs({current.equipped or{},current.candidates or{}})do for _,item in ipairs(items)do out[#out+1]=item end end
    return out
end
function U:DefaultWorkflowEncounters()
    local aug=self:Current().snapshot.character.spec==1473;local s=self:Scenario()
    local buffs=aug and"full"or s.raidBuffs
    return{{id="raid",name="Raid boss",fightStyle="patchwerk",targets=1,seconds=aug and s.seconds or 300,raidBuffs=buffs,bloodlust=buffs=="full"and"on"or"off"},
        {id=aug and"aoe"or"dungeon",name=aug and"5 targets"or"Dungeon Slice",fightStyle=aug and"patchwerk"or"dungeon_slice",targets=aug and 5 or 1,seconds=aug and s.seconds or 360,raidBuffs=aug and"full"or"none",bloodlust=aug and"on"or"off"}}
end
function U:VenomstoneInfo(item)
    local snapshot=(self:Current()or{}).snapshot or{};local c=snapshot.character or{}
    if snapshot.client~="retail"or snapshot.interface~=120105 or c.role~="DAMAGER"then return{reason="Venomstone comparisons require the supported Retail 12.1.5 damage model."}end
    if not LUI.Sim.VenomstoneInfo then return{reason="Update LamdaSim to compare Venomstones."}end
    return LUI.Sim:VenomstoneInfo(item)
end
function U:VenomstoneSelected(record,item)
    for _,choice in ipairs(record.selected or{})do if self:WorkflowItemKey(choice)==self:WorkflowItemKey(item)then
        for _,option in ipairs(choice.options or{})do if option.venomstone then return true end end
    end end
    return false
end
function U:ToggleVenomstone(item,draft)
    local info=self:VenomstoneInfo(item);if not info.eligible then return end
    local record=draft or self:WorkflowSettings();record.selected=record.selected or{};record.autoVenomstone=record.autoVenomstone or{}
    local key=self:WorkflowItemKey(item);local choice,index
    for i,v in ipairs(record.selected)do if self:WorkflowItemKey(v)==key then choice=v;index=i;break end end
    if self:VenomstoneSelected(record,item)then
        for i=#choice.options,1,-1 do if choice.options[i].venomstone then table.remove(choice.options,i)end end
        if #choice.options==0 then choice.options=nil;if record.autoVenomstone[key]then table.remove(record.selected,index)end end
        record.autoVenomstone[key]=nil
        if #record.selected==0 and record.autoVenomstoneEnabled then record.enabled=false;record.autoVenomstoneEnabled=nil end
    else
        if not choice then
            if #record.selected>=8 then self.notice="Choose up to 8 items in Customize analysis.";if draft then setText(self.workflowPicker.note,self.notice)else self:Refresh()end;return end
            if item.source=="vault"then for _,v in ipairs(record.selected)do if v.source=="vault"then self.notice="Choose one Vault reward per comparison.";if draft then setText(self.workflowPicker.note,self.notice)else self:Refresh()end;return end end end
            local i=item.identity or item;choice={source=i.source or item.source,slot=i.slot or item.slot,value=i.value or item.value};record.selected[#record.selected+1]=choice;record.autoVenomstone[key]=true
        end
        choice.options=choice.options or{};if #choice.options>=8 then self.notice="Choose up to eight alternatives per item.";return end
        choice.options[#choice.options+1]={venomstone=true,label="Venomstones ("..info.level..")"}
        if not record.enabled then record.autoVenomstoneEnabled=true end
        record.enabled=true;record.encounters=record.encounters or self:DefaultWorkflowEncounters()
    end
    if draft then self:RefreshWorkflow()else self.notice="Choice saved. Compare selected items to calculate normal and Venomforged versions.";self:Refresh()end
end
function U:BuildVenomstoneButton(parent,x,y,width,draft)
    local b=button(parent,"",x,y,width,function()if parent.item then U:ToggleVenomstone(parent.item,draft and U.workflowDialog.draft)end end)
    b:SetScript("OnEnter",function(self)
        if not parent.item then return end
        local info=U:VenomstoneInfo(parent.item);GameTooltip:SetOwner(self,"ANCHOR_RIGHT");GameTooltip:SetText("Venomstones")
        GameTooltip:AddLine(info.eligible and("Compare this item normally and at max rank + Venomstones (ilvl "..info.level..").\n10 Venomstones per item. Normal rank costs, unlocks and your currencies are not checked.\nRun Compare selected items after choosing.")or info.reason,1,1,1,true);GameTooltip:Show()
    end)
    b:SetScript("OnLeave",function()U:HideItemTip()end);return b
end
function U:RefreshVenomstoneButton(row,record)
    local info=self:VenomstoneInfo(row.item);local on=self:VenomstoneSelected(record,row.item)
    row.venomstone:SetText(info.applied and"Applied"or info.eligible and(on and("[x] "..info.level)or"[ ] Off")or"—")
    -- Keep the button hoverable so disabled choices explain their eligibility.
    row.venomstone:SetEnabled(true)
end
function U:OpenWorkflow()
    self:Build()
    if not self.workflowDialog then
        local d=CreateFrame("Frame",nil,self.frame,"BackdropTemplate");self.workflowDialog=d;d:Hide();d:SetSize(900,590);d:SetPoint("CENTER");d:SetFrameLevel(self.frame:GetFrameLevel()+25)
        d:SetBackdrop(backdrop);d:SetBackdropColor(.025,.045,.065,1);d:SetBackdropBorderColor(.2,.65,.75,1);d:EnableMouse(true)
        label(d,"Customize analysis",18,-18,500,true)
        button(d,"Close",784,-12,98,function()d:Hide()end)
        d.context=dropdown(d,18,-50,390,{"Mythic+","Raid"},function()return settings().workflowContext=="raid"and"Raid"or"Mythic+"end,function(v)settings().workflowContext=v=="Raid"and"raid"or"m+";U:OpenWorkflow()end)
        label(d,"Compare the same choices under both encounters.",18,-86,400)
        d.encounters={}
        for index=1,2 do
            local y=-118-(index-1)*163;local box=card(d,18,y,400,150);local row={};d.encounters[index]=row
            row.preset=dropdown(box,10,-10,380,function()
                local choices={"Raid boss","7 targets / 3 min","1 target / 10 min","4 targets / 5 min","Custom"}
                local current=U:Current();if current and current.snapshot.character.spec~=1473 then table.insert(choices,2,"Dungeon Slice")end
                return choices
            end,function()return d.draft.encounters[index].name or"Custom"end,function(v)
                local e=d.draft.encounters[index];e.name=v;e.fightStyle="patchwerk"
                if v=="Dungeon Slice"then e.fightStyle="dungeon_slice";e.targets=1;e.seconds=360;e.raidBuffs="none";e.bloodlust="off"
                elseif v=="Raid boss"then e.targets=1;e.seconds=300
                elseif v=="7 targets / 3 min"then e.targets=7;e.seconds=180;e.bloodlust="off"
                elseif v=="1 target / 10 min"then e.targets=1;e.seconds=600;e.bloodlust="on"
                elseif v=="4 targets / 5 min"then e.targets=4;e.seconds=300 end
                U:RefreshWorkflow()
            end)
            label(box,"Targets",12,-51,70);row.targets=edit(box,16,-74,64,"",2)
            label(box,"Seconds",92,-51,72);row.seconds=edit(box,96,-74,64,"",3)
            label(box,"Raid buffs",174,-51,104)
            row.buffs=dropdown(box,170,-74,106,{"Self only","Full raid"},function()return d.draft.encounters[index].raidBuffs=="full"and"Full raid"or"Self only"end,function(v)d.draft.encounters[index].raidBuffs=v=="Full raid"and"full"or"none"end)
            label(box,"Bloodlust",286,-51,102)
            row.lust=dropdown(box,282,-74,106,{"On","Off"},function()return d.draft.encounters[index].bloodlust=="on"and"On"or"Off"end,function(v)d.draft.encounters[index].bloodlust=v=="On"and"on"or"off"end)
            label(box,"Stationary targets; Dungeon Slice uses its fixed model.",12,-115,375)
            local function retainFields()
                local e=d.draft and d.draft.encounters[index];if not e then return end
                local targets=tonumber(row.targets:GetText());local seconds=tonumber(row.seconds:GetText())
                if targets and targets%1==0 and targets>=1 and targets<=20 then e.targets=targets end
                if seconds and seconds%1==0 and seconds>=30 and seconds<=600 then e.seconds=seconds end
            end
            row.targets:SetScript("OnEditFocusLost",retainFields);row.seconds:SetScript("OnEditFocusLost",retainFields)
        end
        label(d,"Equipment · click a slot to add alternatives",438,-51,440,true)
        d.sheet={}
        local columns={{"head","neck","shoulder","back","chest","wrist"},{"hands","waist","legs","feet","finger1","finger2","trinket1","trinket2"}}
        for column,slots in ipairs(columns)do for index,slot in ipairs(slots)do self:BuildWorkflowSlot(d,slot,438+(column-1)*226,-84-(index-1)*43)end end
        self:BuildWorkflowSlot(d,"main_hand",438,-431);self:BuildWorkflowSlot(d,"off_hand",664,-431)
        button(d,"Clear selections",438,-486,180,function()d.draft.selected={};U:RefreshWorkflow()end)
        label(d,"Equipped gear stays as the baseline. Add several choices to any slot; rings and trinkets share their pair's alternatives.",626,-482,256):SetFontObject("GameFontHighlightSmall")
        d.note=label(d,"",18,-461,394);d.note:SetMaxLines(3)
        button(d,"Use default analysis",18,-540,248,function()local record=U:WorkflowSettings();record.enabled=false;record.selected={};record.autoVenomstone=nil;record.autoVenomstoneEnabled=nil;d:Hide();U:Refresh()end)
        button(d,"Apply",634,-540,248,function()U:ApplyWorkflow()end)
        d:SetScript("OnHide",function()if U.workflowPicker then U.workflowPicker:Hide()end;U:HideItemTip()end)
    end
    local record,key=self:WorkflowSettings();local d=self.workflowDialog;d.draft=self:CopyWorkflow(record);d.key=key
    if type(d.draft.encounters)~="table"or#d.draft.encounters~=2 then
        d.draft.encounters=self:DefaultWorkflowEncounters()
    end
    self:RefreshWorkflow();d:Show()
end
function U:BuildWorkflowSlot(dialog,slot,x,y)
    local row=CreateFrame("Button",nil,dialog,"BackdropTemplate");row:SetPoint("TOPLEFT",x,y);row:SetSize(216,39)
    row:SetBackdrop(backdrop);row:SetBackdropColor(.04,.055,.07,1);row:SetBackdropBorderColor(.25,.3,.34,1)
    row.icon=row:CreateTexture(nil,"ARTWORK");row.icon:SetSize(32,32);row.icon:SetPoint("LEFT",3,0);row.icon:SetTexCoord(.08,.92,.08,.92)
    row.title=label(row,sheetSlotNames[slot],42,-4,169);row.title:SetFontObject("GameFontHighlightSmall");row.title:SetMaxLines(1)
    row.name=label(row,"",42,-20,169);row.name:SetFontObject("GameFontHighlightSmall");row.name:SetMaxLines(1)
    row:SetScript("OnClick",function()U:OpenWorkflowSlot(slot)end)
    row:SetScript("OnEnter",function(self)if self.item then U:ItemTip(self,self.item)else GameTooltip:SetOwner(self,"ANCHOR_RIGHT");GameTooltip:SetText((sheetSlotNames[slot]or slot).." · Empty");GameTooltip:AddLine("Click to choose alternatives.");GameTooltip:Show()end end)
    row:SetScript("OnLeave",function()U:HideItemTip()end);dialog.sheet[slot]=row
end
function U:ToggleWorkflowChoice(item)
    local d=self.workflowDialog;local wanted=self:WorkflowItemKey(item);local selected=d.draft.selected
    for i,choice in ipairs(selected)do if self:WorkflowItemKey(choice)==wanted then table.remove(selected,i);self:RefreshWorkflow();return end end
    if #selected>=8 then setText(self.workflowPicker.note,"Choose up to 8 alternatives across your equipment.");return end
    if item.source=="vault"then for _,choice in ipairs(selected)do if choice.source=="vault"then setText(self.workflowPicker.note,"Choose one Vault reward per comparison.");return end end end
    local i=item.identity or item;selected[#selected+1]={source=i.source or item.source,slot=i.slot or item.slot,value=i.value or item.value};self:RefreshWorkflow()
end
function U:OpenWorkflowSlot(slot)
    if not self.workflowPicker then
        local p=CreateFrame("Frame",nil,self.workflowDialog,"BackdropTemplate");self.workflowPicker=p;p:Hide();p:SetSize(680,550);p:SetPoint("CENTER");p:SetFrameLevel(self.workflowDialog:GetFrameLevel()+10)
        p:SetBackdrop(backdrop);p:SetBackdropColor(.025,.045,.065,1);p:SetBackdropBorderColor(.2,.65,.75,1);p:EnableMouse(true)
        p.title=label(p,"",18,-16,530,true);button(p,"Done",554,-11,108,function()p:Hide();U:HideItemTip()end)
        p.base=CreateFrame("Button",nil,p);p.base:SetPoint("TOPLEFT",18,-49);p.base:SetSize(640,48)
        p.base.icon=p.base:CreateTexture(nil,"ARTWORK");p.base.icon:SetSize(36,36);p.base.icon:SetPoint("LEFT",0,0)
        p.base.name=label(p.base,"",46,-2,590);p.base.detail=label(p.base,"Equipped baseline · always included",46,-23,590)
        p.base:SetScript("OnEnter",function(self)if self.item then U:ItemTip(self,self.item)end end);p.base:SetScript("OnLeave",function()U:HideItemTip()end)
        label(p,"Venomstones",553,-91,106):SetFontObject("GameFontHighlightSmall")
        p.rows={}
        for index=1,10 do
            local row=CreateFrame("Button",nil,p,"BackdropTemplate");row:SetPoint("TOPLEFT",18,-108-(index-1)*34);row:SetSize(644,31)
            row:SetBackdrop(backdrop);row:SetBackdropColor(.04,.065,.08,.95);row:SetBackdropBorderColor(.13,.22,.27,1)
            row.check=label(row,"[ ]",7,-8,27);row.icon=row:CreateTexture(nil,"ARTWORK");row.icon:SetSize(27,27);row.icon:SetPoint("LEFT",34,0)
            row.name=label(row,"",69,-4,455);row.name:SetMaxLines(1);row.source=label(row,"",69,-18,455);row.source:SetFontObject("GameFontHighlightSmall")
            row.venomstone=U:BuildVenomstoneButton(row,535,-3,104,true)
            row:SetScript("OnClick",function(self)if self.item then U:ToggleWorkflowChoice(self.item)end end)
            row:SetScript("OnEnter",function(self)if self.item then U:ItemTip(self,self.item)end end);row:SetScript("OnLeave",function()U:HideItemTip()end);p.rows[index]=row
        end
        p.empty=label(p,"No captured alternatives for this slot.",18,-121,640)
        p.previous=button(p,"Previous",18,-458,112,function()p.page=math.max(1,p.page-1);U:RefreshWorkflowPicker()end)
        p.next=button(p,"Next",140,-458,112,function()p.page=p.page+1;U:RefreshWorkflowPicker()end)
        p.count=label(p,"",276,-464,386);p.note=label(p,"Tick multiple items for this slot. LSim compares legal sets; it does not equip anything.",18,-503,644)
    end
    local p=self.workflowPicker;p.slot=slot;p.page=1;self:RefreshWorkflowPicker();p:Show()
end
function U:RefreshWorkflowPicker()
    local p=self.workflowPicker;if not p or not p.slot then return end
    local items={};for _,item in ipairs((self:Current()or{}).candidates or{})do if self:WorkflowSlotMatches(p.slot,item)then items[#items+1]=item end end
    local selected={};for _,item in ipairs(self.workflowDialog.draft.selected or{})do selected[self:WorkflowItemKey(item)]=true end
    local base;for _,item in ipairs((self:Current()or{}).equipped or{})do if item.slot==p.slot then base=item;break end end
    setText(p.title,(sheetSlotNames[p.slot]or p.slot).." · choose alternatives");p.base.item=base;p.base.icon:SetTexture(self:ItemTexture(base,p.slot));setText(p.base.name,base and clean(base.label or p.slot)or"Empty slot")
    local pages=math.max(1,math.ceil(#items/10));p.page=math.min(p.page,pages)
    for index,row in ipairs(p.rows)do local item=items[(p.page-1)*10+index];row.item=item;row:SetShown(item~=nil)
        if item then
            local checked=selected[self:WorkflowItemKey(item)];setText(row.check,checked and"[x]"or"[ ]");row.icon:SetTexture(self:ItemTexture(item));setText(row.name,clean(item.label or item.slot));setText(row.source,(item.source=="vault"and"Vault"or"Bags").." · "..(sheetSlotNames[item.slot]or item.slot));row:SetBackdropBorderColor(checked and .2 or .13,checked and .8 or .22,checked and .45 or .27,1)
            self:RefreshVenomstoneButton(row,self.workflowDialog.draft)
            local swap=self:SwapInfo(nil,{items={{id=self:ItemID(item),slot=p.slot,key=item.value,label=item.label,link=item.link,source=item.source}}});row.compareText=swap.tooltip
        else row.compareText=nil end
    end
    p.empty:SetShown(#items==0);p.previous:SetEnabled(p.page>1);p.next:SetEnabled(p.page<pages);setText(p.count,"Page "..p.page.." / "..pages.." · "..#self.workflowDialog.draft.selected.." selected overall")
    setText(p.note,"Tick multiple alternatives for this slot. The sim selects legal sets; no equipment is changed.")
end
function U:RefreshWorkflow()
    local d=self.workflowDialog;if not d or not d.draft then return end
    d.context:SetText(d.draft.context=="raid"and"Raid"or"Mythic+")
    for index,row in ipairs(d.encounters)do local e=d.draft.encounters[index];row.preset:SetText(e.name);row.targets:SetText(tostring(e.targets));row.seconds:SetText(tostring(e.seconds));row.buffs:SetText(e.raidBuffs=="full"and"Full raid"or"Self only");row.lust:SetText(e.bloodlust=="on"and"On"or"Off")end
    local items=self:WorkflowItems()
    local selected={};for _,item in ipairs(d.draft.selected or{})do selected[self:WorkflowItemKey(item)]=true end
    local equipped={};for _,item in ipairs((self:Current()or{}).equipped or{})do equipped[item.slot]=item end
    for slot,row in pairs(d.sheet)do
        local count=0;for _,item in ipairs(d.draft.selected or{})do if self:WorkflowSlotMatches(slot,item)then count=count+1 end end
        row.item=equipped[slot];row.icon:SetTexture(self:ItemTexture(row.item,slot));setText(row.name,row.item and clean(row.item.label or slot,60)or"Empty");setText(row.title,(sheetSlotNames[slot]or slot)..(count>0 and(" · "..count.." alternatives")or""));row:SetBackdropBorderColor(count>0 and .2 or .25,count>0 and .8 or .3,count>0 and .45 or .34,1)
    end
    if self.workflowPicker and self.workflowPicker:IsShown()then self:RefreshWorkflowPicker()end
    local missing=0;for key in pairs(selected)do local found=false;for _,item in ipairs(items)do if self:WorkflowItemKey(item)==key then found=true;break end end;if not found then missing=missing+1 end end
    setText(d.note,tostring(#(d.draft.selected or{})).." selected · saved for this character, spec and content."..(missing>0 and("\n"..missing.." changed or unavailable; clear/reselect before running.")or"\nNo selections runs the usual broad gear analysis."))
end
function U:ApplyWorkflow()
    local d=self.workflowDialog
    for index,row in ipairs(d.encounters)do
        local e=d.draft.encounters[index];local targets=tonumber(row.targets:GetText());local seconds=tonumber(row.seconds:GetText())
        if not targets or targets%1~=0 or targets<1 or targets>20 or not seconds or seconds%1~=0 or seconds<30 or seconds>600 then setText(d.note,"Use 1–20 targets and 30–600 whole seconds.");return end
        e.targets=targets;e.seconds=seconds
        if e.fightStyle=="dungeon_slice"then e.targets=1;e.seconds=360;e.raidBuffs="none";e.bloodlust="off"
        else e.name=tostring(targets)..(targets==1 and" target"or" targets").." / "..seconds.."s"end
    end
    local _,key=self:WorkflowSettings();if key~=d.key then setText(d.note,"Your character or content context changed. Close and reopen Customize analysis.");return end
    d.draft.enabled=true;d.draft.autoVenomstoneEnabled=nil;d.draft.autoVenomstone={};settings().workflows[key]=self:CopyWorkflow(d.draft);d:Hide();self:Refresh()
end
function U:PrepareWorkflow(options)
    local current=self:Current();local c=current and current.snapshot.character or{}
    if not current or current.snapshot.client~="retail"or c.role~="DAMAGER"then return true end
    local record=self:WorkflowSettings();if not record.enabled then return true end
    local available={};for _,item in ipairs(self:WorkflowItems())do available[self:WorkflowItemKey(item)]=true end
    for _,item in ipairs(record.selected or{})do if not available[self:WorkflowItemKey(item)]then self.notice="A saved item changed or is unavailable. Open Customize analysis to update your choices.";return false end end
    options.scenario.encounters=self:CopyWorkflow(record.encounters);options.selected=self:CopyWorkflow(record.selected or{});options.compareMode=#options.selected>0 and"topgear"or"individual"
    local revision=0;for _,entry in ipairs(self:Envelope().workflows or{})do if entry.characterGUID==c.guid and entry.spec==c.spec and entry.settings and entry.settings.context==record.context then revision=entry.revision or 0;break end end
    options.workflow={context=record.context,expectedRevision=revision,scenario=options.scenario,compareMode=options.compareMode,selected=self:CopyWorkflow(options.selected)}
    return true
end
function U:ConfirmSend(callback)
    self:Build()
    if not self.confirmation then
        local overlay=CreateFrame("Frame","LamdaSimSendConfirmation",self.frame,"BackdropTemplate")
        self.confirmation=overlay;overlay:Hide();overlay:SetAllPoints(self.frame);overlay:SetFrameLevel(self.frame:GetFrameLevel()+30)
        overlay:EnableMouse(true);overlay:SetBackdrop({bgFile=backdrop.bgFile});overlay:SetBackdropColor(0,0,0,.65)
        local dialog=card(overlay,0,0,460,180);dialog:ClearAllPoints();dialog:SetPoint("CENTER")
        label(dialog,"Send to Lamda Engine",18,-18,420,true)
        overlay.warning=label(dialog,"Keep WoW in front while Lamda prepares the comparison. You can stop it from the workspace.",18,-51,420)
        overlay.accept=button(dialog,"Start",18,-132,204,function()
            local run=overlay.run;overlay:Hide();if run then run()end
        end)
        overlay.cancel=button(dialog,"Cancel",238,-132,204,function()overlay:Hide()end)
        overlay:SetScript("OnHide",function()overlay.run=nil end)
        if UISpecialFrames then UISpecialFrames[#UISpecialFrames+1]="LamdaSimSendConfirmation" end
    end
    if LUI.openMenu then LUI.openMenu:Hide()end
    self.confirmation.run=callback;self.confirmation:Show()
end
function U:Send(task,extra,confirmed)
    if InCombatLockdown and InCombatLockdown() then self.notice="Finish combat before sending a request.";self:Refresh();return end
    if transferBusy() then self.notice="A simulation is already in progress.";self:Refresh();return end
    if task=="analysis" then confirmed=true end
    if not confirmed then self:ConfirmSend(function()U:Send(task,extra,true)end);return end
    if LUI.overlayHidden then
        if not LUI.LookupEvents or type(LUI.LookupEvents.SetOverlayHidden)~="function" then
            self.notice="Show color strips before starting the analysis.";self:Refresh();return
        end
        LUI.LookupEvents:SetOverlayHidden(false)
        if LUI.overlayHidden then self.notice="Color strips could not be shown. Show them, then try again.";self:Refresh();return end
    end
    local available,reason=self:Capability()
    if task=="healing" or task=="builds" then available,reason=self:EncounterCapability(task) end
    if not available and task~="cancel" then self.notice=reason;self:Refresh();return end
    local options={task=task,scenario=self:Scenario(),compareMode=settings().compareMode or "individual"}
    for k,v in pairs(extra or {}) do options[k]=v end
    local returnTab=options.returnTab;options.returnTab=nil
    if task=="compare" then
        options.selected={}
		local selectedSet=options.selectedSet or self.selected;options.selectedSet=nil
        local source=self.tab=="vault" and "vault" or "owned"
		for _,item in ipairs((self:Current() or {}).candidates or {}) do
			local plan=selectedSet[identity(item)]
			if plan and(options.compareMode=="topgear"or options.compareMode=="loadout"or item.source==source)then
				local i=item.identity or item
				options.selected[#options.selected+1]={source=i.source or item.source,slot=i.slot or item.slot,
					placement=options.compareMode=="loadout"and(type(plan)=="table"and plan.placement or plan)or nil,
					plannedLevel=options.compareMode=="loadout"and type(plan)=="table"and plan.plannedLevel or nil,
					value=i.value or item.value,key=i.key,link=i.link}
            end
        end
		if #options.selected==0 and not(options.compareMode=="loadout"and options.talents and #options.talents==1)then self.notice="Change at least one item or talent build.";self:Refresh();return end
    elseif task=="talents" then
        options.talents={}
        for _,build in ipairs(self:Builds()) do if self.talentSelected[build.code] then options.talents[#options.talents+1]={label=build.label,code=build.code} end end
        if #options.talents==0 then self.notice="Choose a saved build or add an import code.";self:Refresh();return end
    elseif task=="analysis" then
        options.talents={}
        local current=self:Current();local character=current and current.snapshot and current.snapshot.character or{}
        if character.role~="HEALER"then for _,build in ipairs(self:Builds())do
            if #options.talents<16 then options.talents[#options.talents+1]={label=build.label,code=build.code} end
        end end
    end
    if type(S.RunGameRequest)~="function" then self.notice="Update LamdaUI and Lamda Engine to use in-game requests.";self:Refresh();return end
    local current=self:Current();local character=current and current.snapshot.character or {}
    if character.spec~=1473 or current.snapshot.client~="retail" then options.scenario.augmentationMode=nil;options.scenario.allyItemLevel=nil;options.scenario.allySkill=nil end
    if character.role~="HEALER" then options.scenario.healingMode=nil end
    if task=="analysis"and not self:PrepareWorkflow(options)then self:Refresh();return end
    options.autoReload=true
    local requestTab=self.panels[returnTab]and returnTab or({analysis="overview",baseline="overview",healing="healing",builds="meta",upgrades="upgrades",plan="upgrades",talents="talents"})[task]
    if task=="compare" then requestTab=primaryTabs[self.tab] and self.tab or "gear" end
    self.notice=nil;self.status=nil;self.selectedJobID=nil;self.requestAt=GetTime();self.requestTask=task;LUI:DB().simCurrentTab=requestTab or "overview"
    local ok,accepted,problem=pcall(S.RunGameRequest,S,options)
	if not ok or not accepted then self.requestAt=nil;self.notice=clean(ok and problem or accepted,500)
	elseif options.compareMode=="loadout"then self.loadoutDirty=nil end
    self:Refresh()
end
function U:SelectTab(name)
    if name=="character" then name="loadout" end
    if not self.panels[name] then name="overview" end
    local current=self:Current();local character=current and current.snapshot and current.snapshot.character or {}
    if name=="healing" and character.role~="HEALER" then name="overview" end
    self.tab=name;LUI:DB().simCurrentTab=name
    for key,panel in pairs(self.panels) do panel:SetShown(key==name) end
    self:RefreshNavigation()
    if LUI.openMenu then LUI.openMenu:Hide() end
    self:Refresh()
end
function U:SetStatus(message)
    self.status=message
    if message=="Result loaded." or message=="Cancellation received." then self.openTabOnce=LUI:DB().simCurrentTab end
    if self.frame and self.frame:IsShown() then self:RefreshStatus()end
end
function U:OnSnapshot(view)self.current=normalize(view);self:Refresh()end
local busyMessages={
    healing={"Simulating health, casts and cooldowns…","Replaying the same damage for each policy…"},
    builds={"Checking the dated build cache…","Reading the selected build sample…"},
    capture={"Reading loot…","Checking eligible gear…"},
    sending={"Checking bags…","Preparing your comparison…"},
    baseline={"Running your simulation…","Checking your equipped gear…"},
    compare={"Comparing gear…","Checking item changes…","Comparing your selected items…"},
    talents={"Comparing builds…","Checking talent changes…"},
    upgrades={"Comparing eligible gear…","Checking upgrade gains…"},
    analysis={"Reading equipped gear…","Indexing bag items…","Checking Vault choices…","Matching items to equipment slots…",
        "Trying alternate ring slots…","Trying alternate trinket slots…","Preparing fully upgraded variants…","Reading saved talent builds…",
        "Building single-target comparisons…","Building five-target comparisons…","Ranking the next crest rank…","Comparing equipped items at maximum…",
        "Running gear comparisons…","Running talent comparisons…","Checking item-level upgrade paths…","Comparing results with equipped gear…","Checking simulation confidence…",
        "Sorting Bag gear…","Sorting Vault choices…","Preparing your result pages…","Finishing the strongest comparisons…"},
    plan={"Comparing sources…","Estimating upgrade value…"},
    cancel={"Requesting cancellation…","Waiting for confirmation…"},
    stats={"Comparing item stats…","Checking captured equipment…"},
}
local sendingMessages={
    baseline={"Checking bags…","Reading equipment…"},
    compare={"Checking selected items…","Preparing your comparison…"},
    talents={"Reading talent builds…","Preparing your comparison…"},
    upgrades={"Checking available gear…","Preparing your comparison…"},
    analysis={"Reading equipped gear…","Reading bag and Vault items…","Collecting saved talents…","Preparing raid boss and dungeon views…","Building crest and item variants…"},
    plan={"Checking loot sources…","Preparing your plan…"},
    cancel={"Sending cancellation…","Requesting cancellation…"},
    stats={"Reading item stats…","Preparing your comparison…"},
}
function U:BusyMessage(phase)
    if phase=="paused" then return "Paused · waiting for combat to end." end
    if phase=="ready" then return "Results are ready to check." end
    local poll=S.pollSession
    if phase=="waiting" and poll and (poll.missingChecks or 0)>0 then return "Waiting for Lamda Engine…" end
    local task=poll and poll.task or self.requestTask or "baseline"
    local client=self.current and self.current.snapshot.client
    local model=client=="forever" and task~="cancel" and "stats" or task
    local pool=phase=="sending" and (sendingMessages[model] or busyMessages.sending) or busyMessages[phase] or busyMessages[model] or busyMessages.baseline
    local key=phase..":"..tostring(client)..":"..task
    local now=GetTime();local prior=self.busyMessage
    if not prior or prior.key~=key or now-prior.at>=2.75 then
        local index=prior and prior.key==key and ((prior.index or 1)%#pool+1) or 1
        self.busyMessage={key=key,at=now,index=index,text=pool[index]}
    end
    return self.busyMessage.text
end
function U:RefreshProgress()
    if not self.progress then return end
    local progress=safe(S.TransferProgress,S)
    local phase=type(progress)=="table" and progress.phase
    if not ({capture=true,sending=true,waiting=true,paused=true,ready=true})[phase] then
        self.progress:Hide();self.busyMessage=nil;self.busyAnimationStart=nil;return
    end
    local now=GetTime();self.busyAnimationStart=self.busyAnimationStart or now
    local travel=math.max(0,self.progress:GetWidth()-147)
    local paused=phase=="paused" or phase=="ready"
    local offset=paused and travel/2 or travel*(1-math.cos((now-self.busyAnimationStart)*math.pi/4))/2
    local segment=self.progress.segment
    segment:ClearAllPoints();segment:SetPoint("TOPLEFT",self.progress,"TOPLEFT",1+offset,-1)
    if paused then segment:SetColorTexture(.8,.58,.18,.45)
    else segment:SetColorTexture(.10,.60,.68,.45)end
    local text=phase=="ready" and friendlyStatus(progress.label,240) or self:BusyMessage(phase)
    if text=="" then text="Preparing results…" end
    self.progress.label:SetText(text);self.progress:Show()
    return text,phase
end
function U:RefreshStatus()
    if not self.frame then return end
    local text=self.notice or self.status or S.status or "Ready."
    local pending=reloadPending();local busy,phase=self:RefreshProgress()
    if not self.notice then
        if busy then text=phase=="sending" and "Keep WoW in front while the request starts." or (phase=="paused" or phase=="ready") and busy or phase=="capture" and "Reading eligible loot…" or "Analysis is running. Results will open when they are ready."
            if phase=="waiting" and S.pollSession and (S.pollSession.missingChecks or 0)>0 then text=busy end
        elseif S.scan then text="Reading eligible loot…"
        elseif S.packets then text="Sending request. Keep WoW in front."
        elseif pending then text="Waiting for Lamda Engine…"
        elseif self.requestAt then text="Analysis is running. Results will open when they are ready." end
    end
    local manualReady=phase=="ready" or S.reloadManual==true
    setText(self.statusLabel,friendlyStatus(text,240));self.stop:SetShown(transferBusy() and not manualReady);self.stop:SetText("Stop analysis")
    local running=false;for _,job in ipairs(self:Jobs())do if job.state=="running" then running=true end end
    if self.cancel then self.cancel:SetShown(running and not transferBusy() and not manualReady)end
    if self.loadedLabel then self.loadedLabel:Hide()end
    self.reload:SetShown(manualReady)
    self.reload:SetEnabled(manualReady and not S.reloadAttemptToken and not (InCombatLockdown and InCombatLockdown()))
end
local function currentItemID(item)
	if type(item)~="table"then return end
    return tonumber(item.itemID or item.itemId) or tonumber(tostring(item.value or ""):match("[,%s]id=(%d+)")) or tonumber(tostring(item.value or ""):match("^id=(%d+)")) or tonumber(item.id)
end
local function itemLevel(item)
	if type(item)~="table"then return end
    return tonumber(tostring(item.value or ""):match("ilevel=(%d+)")) or tonumber(tostring(item.label or ""):match("%((%d+)%)$")) or tonumber(item.level)
end
local function minimumItemLevel()
	local value=tonumber(settings().minimumItemLevel)
	return value and math.max(0,math.floor(value))or 0
end
local function syncMinimumItemLevel(except)
	local value=tostring(minimumItemLevel())
	for _,name in ipairs({"gear","vault","loadout"})do
		local field=U.panels[name]and U.panels[name].minLevel
		local focused=field and safe(field.HasFocus,field)
		if field and field~=except and not focused and field:GetText()~=value then field:SetText(value)end
	end
end
local function commitMinimumItemLevel(field)
	local text=tostring(field:GetText()or""):match("^%s*(.-)%s*$")
	local value=text==""and 0 or tonumber(text)
	if not value or value%1~=0 or value<0 or value>9999 then
		field:SetText(tostring(minimumItemLevel()));U.notice="Minimum item level must be a whole number from 0 to 9999.";U:Refresh();return
	end
	value=math.floor(value);settings().minimumItemLevel=value;field:SetText(tostring(value));syncMinimumItemLevel(field)
	U.page=1;U.loadoutOptionPage=1;U.notice=nil;U:Refresh()
end
local function minimumItemLevelEdit(parent,x,y,width)
	local field=edit(parent,x,y,width,tostring(minimumItemLevel()),4)
	field:SetScript("OnEnterPressed",function(self)commitMinimumItemLevel(self);self:ClearFocus()end)
	field:SetScript("OnEditFocusLost",function(self)commitMinimumItemLevel(self)end)
	return field
end
local function itemRank(item)
	local rank,maxRank=tonumber(item and(item.plannedRank or item.currentRank)),tonumber(item and item.maxRank)
	if not rank or not maxRank then return end
	return ((item.track and item.track~="")and(clean(item.track,18).." ")or"Rank ")..rank.."/"..maxRank
end
local function rankPath(item)
	local path={};for _,level in ipairs(item and item.rankLevels or{})do if num(level)then path[#path+1]=level end end
	local current,maxRank=tonumber(item and item.currentRank),tonumber(item and item.maxRank);local level=item and itemLevel(item)
	if current and maxRank and maxRank>current and #path==maxRank-current+1 and path[1]==level and path[#path]==item.maxLevel then return path,current,maxRank end
	return {},current,maxRank
end
local function plannedRank(item,level)
	local path,current=rankPath(item);for index,value in ipairs(path)do if value==level then return current+index-1 end end
end
local function sheetItemDetail(item,source)
	local parts={};local rank=itemRank(item);if rank then parts[#parts+1]=rank end
	local level=item and(tonumber(item.plannedLevel)or itemLevel(item));if level then parts[#parts+1]="ilvl "..level end
	parts[#parts+1]=source;return table.concat(parts," · ")
end

local function advisorStat(stats,name)
    local total=0
    for _,key in ipairs(advisorStatKeys[name]or{})do total=total+(tonumber(stats and stats[key])or 0)end
    return total
end
local function advisorMode(profile,spec)
    local saved=settings().advisorModes
    local wanted=type(saved)=="table"and saved[tostring(spec or"")]or settings().advisorMode
    if profile and profile.tank then
        if wanted~="balanced"and wanted~="survival"and wanted~="damage"then wanted="balanced"end
    elseif wanted~="raid"and wanted~="dungeon"then wanted="dungeon"end
    return wanted
end
local function advisorModeName(profile,mode)
    if profile and profile.tank then return ({balanced="Balanced",survival="Survival",damage="Damage"})[mode]or"Balanced"end
    return mode=="raid"and"Raid"or"Mythic+"
end
local function advisorWeights(profile,mode,metaOrder)
    if not profile then return end
    local order=metaOrder or profile[mode]or profile.dungeon or profile.raid
    if type(order)~="table"then return end
    local weights={[profile.primary]=2.55,weapon=profile.weapon and 9 or 0,leech=.18,avoidance=.16}
    local steps=profile.equal and{.9,.9,.9,.9}or{1,.91,.82,.73}
    for index,name in ipairs(order)do weights[name]=steps[index]or.65 end
    -- A current flawless gem carries 20 secondary rating (14/6 split). Empty
    -- sockets get that transparent opportunity value; filled gems already
    -- arrive as their real stats and do not receive it again.
    weights.socket=(weights[order[1]]or 0)*14+(weights[order[2]]or 0)*6
    if profile.tank then
        weights.armor=mode=="damage"and.18 or mode=="survival"and 1.15 or .72
        weights.stamina=mode=="damage"and.04 or mode=="survival"and.38 or .2
        weights.leech=mode=="damage"and.12 or.3;weights.avoidance=mode=="damage"and.1 or.28
        if mode=="survival"then weights[profile.primary]=2.75 end
    end
    return weights,order
end
local function advisorScore(stats,weights)
    local score=0
    for name,weight in pairs(weights or{})do score=score+advisorStat(stats,name)*weight end
    return score
end
local function advisorScale(fromLevel,toLevel)
    fromLevel,toLevel=tonumber(fromLevel),tonumber(toLevel)
    if not fromLevel or not toLevel or fromLevel<=0 or toLevel<=fromLevel then return 1 end
    return math.min(1.7,1.15^((toLevel-fromLevel)/15))
end
local function advisorTooltipFlags(link)
    if type(link)~="string"then return false,false end
    U.advisorTooltipCache=U.advisorTooltipCache or{}
    local cached=U.advisorTooltipCache[link];if cached then return cached[1],cached[2]end
    local effect,setBonus=false,false
    local api=C_TooltipInfo and C_TooltipInfo.GetHyperlink
    if type(api)=="function"then
        local ok,data=pcall(api,link)
        if ok and type(data)=="table"and type(data.lines)=="table"then
            for _,line in ipairs(data.lines)do
                for _,field in ipairs({"leftText","rightText"})do
                    local text=line and line[field]
                    if not(issecretvalue and issecretvalue(text))and type(text)=="string"then
                        text=text:gsub("|c%x%x%x%x%x%x%x%x",""):gsub("|r","")
                        local lower=text:lower()
                        if lower:match("^equip:")or lower:match("^use:")then effect=true end
                        if lower:find("set (",1,true)or lower:match("^set:")then setBonus=true end
                    end
                end
            end
        end
    end
    U.advisorTooltipCache[link]={effect,setBonus};return effect,setBonus
end
local function advisorEquipped(view)
    local bySlot={}
    for _,item in ipairs(view and view.equipped or{})do bySlot[item.slot]=item end
    return bySlot
end
local function advisorDualWields(view)
    local character=view and view.snapshot and view.snapshot.character or{}
    local class=tostring(character.class or""):upper();local spec=tonumber(character.spec)
    return class=="ROGUE"or class=="DEMONHUNTER"or(class=="WARRIOR"and spec==72)or(class=="DEATHKNIGHT"and spec==251)
        or(class=="SHAMAN"and spec==263)or(class=="MONK"and spec==269)
end
local function advisorReplacement(view,candidate,weights)
    local equipped=advisorEquipped(view);local slot=candidate.slot or""
    local choices={}
    if slot:match("^finger")then choices={equipped.finger1,equipped.finger2}
    elseif slot:match("^trinket")then choices={equipped.trinket1,equipped.trinket2}
    elseif slot=="off_hand"then
        if equipped.main_hand and equipped.main_hand.equipLoc=="INVTYPE_2HWEAPON"then return end
        if candidate.equipLoc=="INVTYPE_WEAPON"and not advisorDualWields(view)then return end
        choices={equipped.off_hand}
    elseif slot=="main_hand"and candidate.equipLoc=="INVTYPE_2HWEAPON"then
        local value=(equipped.main_hand and advisorScore(equipped.main_hand.stats,weights)or 0)+(equipped.off_hand and advisorScore(equipped.off_hand.stats,weights)or 0)
        return {items={equipped.main_hand,equipped.off_hand},score=value,label=clean((equipped.main_hand and equipped.main_hand.label)or"main hand",42)..(equipped.off_hand and(" + "..clean(equipped.off_hand.label or"off hand",28))or"")}
    else choices={equipped[slot]}end
    local best
    for _,item in ipairs(choices)do if item and item.statsReady~=false and type(item.stats)=="table"then
        local value=advisorScore(item.stats,weights)
        if not best or value<best.score then best={items={item},score=value,label=clean(item.label or slot,65)}end
    end end
    return best
end
local function advisorReason(candidate,replacement,profile,order)
    local before={}
    for _,item in ipairs(replacement and replacement.items or{})do
        for stat,value in pairs(item.stats or{})do before[stat]=(before[stat]or 0)+(tonumber(value)or 0)end
    end
    local after=candidate.stats or{};local values={profile.primary,"crit","haste","mastery","vers","socket"};local changes={}
    for _,name in ipairs(values)do
        local delta=advisorStat(after,name)-advisorStat(before,name)
        if delta~=0 then changes[#changes+1]={name=name,value=delta,importance=name==profile.primary and 10 or 5}end
    end
    table.sort(changes,function(a,b)return math.abs(a.value)*a.importance>math.abs(b.value)*b.importance end)
    local text={}
    for index=1,math.min(3,#changes)do local change=changes[index];text[#text+1]=string.format("%+d %s",change.value,advisorStatNames[change.name])end
    if #text==0 then return "Same scored stats; item effects decide."end
    return table.concat(text," · ")
end
local function advisorData(view)
    local character=view and view.snapshot and view.snapshot.character or{};local profile=advisorProfiles[character.spec]
    if not view or not view.snapshot or view.snapshot.client~="retail"then return nil,"The local gear advisor is available on Retail characters."end
    if not profile then return nil,"No maintained guide model for this specialization yet."end
    local mode=advisorMode(profile,character.spec);local report=advisorBuildReport(character.spec);local metaOrder
    if report and(mode=="dungeon"or profile.tank)then
        local stats={};for _,stat in ipairs(report.stats or{})do stats[#stats+1]=stat end
        table.sort(stats,function(a,b)return(tonumber(a.rank)or 99)<(tonumber(b.rank)or 99)end)
        local seen={};metaOrder={};for _,stat in ipairs(stats)do local key=stat.key;if advisorStatKeys[key]and not seen[key]then seen[key]=true;metaOrder[#metaOrder+1]=key end end
        if #metaOrder~=4 then metaOrder=nil end
    end
    local weights,order=advisorWeights(profile,mode,metaOrder)
    local total,totals,ready=0,{},0
    for _,item in ipairs(view.equipped or{})do if type(item.stats)=="table"and item.statsReady~=false then
        total=total+advisorScore(item.stats,weights);ready=ready+1
        for name in pairs(advisorStatKeys)do totals[name]=(totals[name]or 0)+advisorStat(item.stats,name)end
    end end
    if ready==0 or total<=0 then return nil,"Refresh character data in Setup so WoW can read the item stats."end
    local rows={};local reviewCount=0
    for _,candidate in ipairs(view.candidates or{})do
        if (candidate.source=="owned"or candidate.source=="vault")and candidate.equippable~=false and type(candidate.stats)=="table"and candidate.statsReady~=false then
            local replaced=advisorReplacement(view,candidate,weights)
            if replaced then
                local current=advisorScore(candidate.stats,weights)-replaced.score
                local level=tonumber(candidate.capturedLevel or itemLevel(candidate))or 0
                local maximum=tonumber(candidate.maxLevel)or level
                local replacedMaximum=0
                for _,item in ipairs(replaced.items or{})do
                    local itemLevelNow=tonumber(item.capturedLevel or itemLevel(item))or 0
                    local itemLevelMax=tonumber(item.maxLevel)or itemLevelNow
                    replacedMaximum=replacedMaximum+advisorScore(item.stats,weights)*advisorScale(itemLevelNow,itemLevelMax)
                end
                local maxDelta=advisorScore(candidate.stats,weights)*advisorScale(level,maximum)-replacedMaximum
                local effect,setBonus=advisorTooltipFlags(candidate.link);local removedEffect,removedSet=false,false
                for _,item in ipairs(replaced.items or{})do local a,b=advisorTooltipFlags(item and item.link);removedEffect=removedEffect or a;removedSet=removedSet or b end
                local special=effect or setBonus or removedEffect or removedSet or tostring(candidate.slot):match("^trinket")~=nil
                if current>0 or maxDelta>0 then
                    if special then reviewCount=reviewCount+1 end
                    local percent=current/total*100;local maxPercent=maxDelta/total*100
                    rows[#rows+1]={item=candidate,replaced=replaced,delta=current,maxDelta=maxDelta,percent=percent,maxPercent=maxPercent,
                        clear=current>0 and percent>=.25,close=current>0 and percent<.25,future=current<=0 and maxDelta>0,
                        special=special,effect=effect or removedEffect,setBonus=setBonus or removedSet,reason=advisorReason(candidate,replaced,profile,order),level=level,maxLevel=maximum}
                end
            end
        end
    end
    table.sort(rows,function(a,b)
        local function group(row)return row.special and 4 or row.clear and 1 or row.close and 2 or 3 end
        local ag,bg=group(a),group(b);if ag~=bg then return ag<bg end
        local av=(a.delta>0 and a.delta or a.maxDelta);local bv=(b.delta>0 and b.delta or b.maxDelta)
        if av~=bv then return av>bv end
        return tostring(a.item.label)<tostring(b.item.label)
    end)
    local secondary=(totals.crit or 0)+(totals.haste or 0)+(totals.mastery or 0)+(totals.vers or 0);local mix={}
    for _,name in ipairs({"crit","haste","mastery","vers"})do
        mix[#mix+1]=advisorStatNames[name].." "..(secondary>0 and string.format("%.0f%%",(totals[name]or 0)/secondary*100)or"—")
    end
    local crest
    local upgrades={};for _,item in ipairs(view.equipped or{})do upgrades[item.slot]=item end
    for _,item in ipairs(view.equipped or{})do
        local level=tonumber(item.capturedLevel or itemLevel(item));local nextLevel=tonumber(item.nextLevel)
        if level and nextLevel and nextLevel>level and type(item.stats)=="table"then
            local delta=advisorScore(item.stats,weights)*(advisorScale(level,nextLevel)-1)
            if not crest or delta>crest.delta then crest={item=item,delta=delta,percent=delta/total*100,nextLevel=nextLevel}end
        end
    end
    return {profile=profile,mode=mode,weights=weights,order=order,total=total,totals=totals,mix=table.concat(mix," · "),rows=rows,reviewCount=reviewCount,ready=ready,crest=crest,meta=metaOrder and report or nil}
end
function U:ItemTexture(item,slot)
    local id=currentItemID(item or{})
    local texture=item and item.icon or safe(C_Item and C_Item.GetItemIconByID or GetItemIcon,id)
    if not texture and slot and GetInventorySlotInfo then
        local names={head="HeadSlot",neck="NeckSlot",shoulder="ShoulderSlot",back="BackSlot",chest="ChestSlot",wrist="WristSlot",hands="HandsSlot",waist="WaistSlot",legs="LegsSlot",feet="FeetSlot",finger1="Finger0Slot",finger2="Finger1Slot",trinket1="Trinket0Slot",trinket2="Trinket1Slot",main_hand="MainHandSlot",off_hand="SecondaryHandSlot"}
        local _,icon=safe(GetInventorySlotInfo,names[slot]);texture=icon
    end
    return texture or 134400
end
function U:SwapInfo(job,comparison,reference)
    comparison=comparison or{};local summary=job and self:SummaryFor(job);local current=self:Current()or{}
    local base={};for _,item in ipairs(summary and summary.equipped or{})do base[item.slot]=item end
    for _,item in ipairs(reference and reference.items or{})do if item.slot and not base[item.slot]then base[item.slot]=item end end
    if not job then for _,item in ipairs(current.equipped or{})do base[item.slot]=item end end
    local changes=comparison.items
    if not changes or#changes==0 then changes={{slot=comparison.slot,candidateID=comparison.candidateID,id=comparison.id,label=comparison.label,key=comparison.key,link=comparison.link,level=comparison.level,source=comparison.source}}end
    local out={changes={}}
    for _,raw in ipairs(changes)do
        local item={};for k,v in pairs(raw)do item[k]=v end
        local key=raw.baseKey or raw.key or raw.value
        for _,items in ipairs({summary and summary.candidates or{},summary and summary.equipped or{},current.candidates or{},current.equipped or{}})do
            for _,candidate in ipairs(items)do
                if key and key~=""and(candidate.value==key or candidate.key==key)and(raw.source==nil or raw.source==""or raw.source==candidate.source)then
                    for k,v in pairs(candidate)do if item[k]==nil or item[k]==""or(k=="level"and item[k]==0)then item[k]=v end end
                end
            end
        end
        item.slot=raw.slot or comparison.slot or item.slot
        item.link=raw.link or(comparison.link and(#changes==1 or tonumber(raw.id)==tonumber(comparison.id))and comparison.link)or item.link
        item.itemID=tonumber(raw.id)or currentItemID(item)
        if item.itemID and item.itemID>0 then
            item.label=item.label or comparison.label or("Item "..item.itemID)
            item.level=raw.level and raw.level>0 and raw.level or comparison.level or item.level
            if(raw.variant and raw.variant~="current")or(raw.baseKey and raw.baseKey~=raw.key)or(comparison.capturedLevel and comparison.level~=comparison.capturedLevel)then
                item.previewNote="Tooltip shows the captured item; the sim includes planned changes"..(item.level and item.level>0 and(" at ilvl "..item.level)or"").."."
            end
            if(raw.venomstones or 0)>0 then item.previewNote=(item.previewNote or"").."\n"..raw.venomstones.." Venomstones; normal rank costs and currencies not checked."end
        end
        local old=base[item.slot]
        if not old and reference then old=replacedItem(reference,comparison)end
        local unchanged=old and key and(old.value or old.key)==(raw.key or raw.value)and not item.previewNote
        if not unchanged then
            local change={slot=item.slot,item=item.itemID and item.itemID>0 and item or nil,replaced=old}
            out.changes[#out.changes+1]=change
            if not out.item and change.item then out.item=change.item end
        end
    end
    local lines,tooltip={},{}
    for _,change in ipairs(out.changes)do
        local slot=sheetSlotNames[change.slot]or"Equipment";local old=change.replaced;local oldName=old and clean(old.label or old.name or"equipped item",64)
        if oldName and num(old.level)and old.level>0 and not oldName:find("(",1,true)then oldName=oldName.." ("..old.level..")"end
        local action=change.item and(oldName and("Replaces "..oldName)or"Captured slot was empty")or(oldName and("Remove "..oldName)or"Clear slot")
        if not old and job then action="Original equipped item unavailable"end
        lines[#lines+1]=slot.." · "..action
        tooltip[#tooltip+1]=slot..": "..(change.item and clean(change.item.label,100)or"Empty").."\n"..action
    end
    out.text=table.concat(lines,"\n");out.tooltip=table.concat(tooltip,"\n\n")
    return out
end
function U:HideItemTip()
    if TooltipComparisonManager and TooltipComparisonManager.Clear then safe(TooltipComparisonManager.Clear,TooltipComparisonManager,GameTooltip)end
    if GameTooltip then GameTooltip:Hide()end
    if ShoppingTooltip1 then ShoppingTooltip1:Hide()end;if ShoppingTooltip2 then ShoppingTooltip2:Hide()end
end
function U:ItemTip(owner,item)
    if not GameTooltip then return end
    GameTooltip:SetOwner(owner,"ANCHOR_RIGHT")
    local linked=item.link and pcall(GameTooltip.SetHyperlink,GameTooltip,item.link)
    if not linked then
        GameTooltip:SetText(clean(item.label or "Item"))
        if itemLevel(item) then GameTooltip:AddLine("Captured item level "..itemLevel(item),.8,.85,.9)end
        GameTooltip:AddLine("Refresh character data in Setup for this item's exact tooltip.",.65,.7,.75,true)
    end
    if item.source then GameTooltip:AddLine(item.source=="vault"and"Vault choice"or item.source=="equipped"and"Captured equipped item"or item.source=="owned"and"In your bags"or"Captured drop",.55,.8,.9)end
    if owner.compareText and owner.compareText~=""then GameTooltip:AddLine(owner.compareText,1,.82,.3,true)end
    if item.previewNote then GameTooltip:AddLine(item.previewNote,1,.82,.3,true)end
    GameTooltip:Show()
    if linked and type(GameTooltip_ShowCompareItem)=="function"then safe(GameTooltip_ShowCompareItem,GameTooltip,owner)end
end
function U:Candidates(source,values)
    local list={};local query=(self.search or ""):lower();local minimum=minimumItemLevel()
    for _,item in ipairs(values or (self:Current() or {}).candidates or {}) do
        local slot=slotNames[item.slot] or item.slot or ""
        local matches=item.source==source and (not self.slot or self.slot=="All slots" or slot==self.slot)
            and (query=="" or tostring(item.label or ""):lower():find(query,1,true) or tostring(currentItemID(item) or ""):find(query,1,true))
        if matches and(minimum==0 or(itemLevel(item)or 0)>=minimum)and(not self.onlySelected or self.selected[identity(item)])then list[#list+1]=item end
    end
    table.sort(list,function(a,b)
        if (a.slot or "")~=(b.slot or "") then return (a.slot or "")<(b.slot or "")end
        if (itemLevel(a) or 0)~=(itemLevel(b) or 0) then return (itemLevel(a) or 0)>(itemLevel(b) or 0)end
        return (a.label or a.id or "")<(b.label or b.id or "")
    end)
    return list
end
function U:ToggleItem(item)
    local key=identity(item)
    if self.selected[key] then self.selected[key]=nil else
        local count=0;for _ in pairs(self.selected)do count=count+1 end
        if count>=8 then self.notice="Choose up to 8 items per comparison.";self:Refresh();return end
        self.selected[key]=true
    end
    self.notice=nil;self:Refresh()
end
local function talentLabel(job,comparison,summary)
    local index=tonumber(tostring(comparison.name or ""):match("^talent(%d+)$"))
    if not index or comparison.label and not comparison.label:match("^Talent build %d+$") then return comparison.label end
    local code=job.scenario and job.scenario.talents and job.scenario.talents[index]
    local request=summary and summary.snapshot and summary.snapshot.gameRequest
    for _,build in ipairs(request and request.talents or {})do if build.code==code and build.label then return build.label end end
    for _,build in ipairs(summary and summary.talents or {})do if build.code==code and build.label then return build.label end end
    return comparison.label
end
local function targetResult(result,targets)
    for _,entry in ipairs(result and result.targetResults or {})do if entry.targets==targets then return entry end end
end
local function analysisTargetLabel(job,targets)
    local entry=targetResult(job and job.result,targets)
    if entry and entry.encounterName and entry.encounterID~="raid"and entry.encounterID~="dungeon"and entry.encounterID~="aoe"then return clean(entry.encounterName,64)end
    if entry and entry.fightStyle=="dungeon_slice"then return "Dungeon Slice"end
    if entry and entry.fightStyle=="patchwerk"then return "Raid boss"end
    return tostring(targets or 1).." target"..((targets or 1)==1 and""or"s")
end
local function analysisTargetChoices()
    local job=U:Job("analysis")
    local first=job and job.result and(job.result.targetResults or{})[1]
    if first and first.encounterID and first.encounterID~="raid"and first.encounterID~="dungeon"and first.encounterID~="aoe"then
        local choices={};for _,entry in ipairs(job.result.targetResults)do choices[#choices+1]=analysisTargetLabel(job,entry.targets)end;return choices
    end
    if targetResult(job and job.result,5)and targetResult(job and job.result,5).fightStyle=="dungeon_slice"then return{"Raid boss","Dungeon Slice"}end
    return{"1 target","5 targets"}
end
local function chooseAnalysisTarget(value)
    local job=U:Job("analysis");for _,entry in ipairs(job and job.result and job.result.targetResults or{})do if analysisTargetLabel(job,entry.targets)==value then return entry.targets end end
    return(value=="Dungeon Slice"or value=="5 targets")and 5 or 1
end
local validationText
local function analysisView(job,targets,source)
    local result=job and job.result;if result and result.metric=="healing_score"then targets=1 end
    local target=targetResult(result,targets)
    if not target then return end
    local comparisons={}
    for _,comparison in ipairs(target.comparisons or {})do if not source or comparison.source==source then comparisons[#comparisons+1]=comparison end end
    local view={}
    for key,value in pairs(job)do view[key]=value end
    local scenario={};for key,value in pairs(job.scenario or {})do scenario[key]=value end
    scenario.targets=target.targetCount or(target.fightStyle=="dungeon_slice"and 1 or targets)
    scenario.bloodlust=target.bloodlust;scenario.encounters=nil
    scenario.fightStyle=target.fightStyle or scenario.fightStyle;scenario.raidBuffs=target.raidBuffs or scenario.raidBuffs;scenario.seconds=target.seconds or scenario.seconds
    view.scenario=scenario
    view.result={metric=result.metric,model=result.model,provider=result.provider,providerVersion=result.providerVersion,validation=result.validation,
        warnings=result.warnings,modelInputs=result.modelInputs,baseline=target.baseline,comparisons=comparisons,rejectedComparisons=target.rejectedComparisons,
        tested=target.tested,requested=target.requested,refined=target.refined,elapsedSeconds=target.elapsedSeconds,complete=target.complete}
    return view
end
function U:AnalysisJob(source,targets)
    local job=self:Job("analysis");if job and job.result and job.result.metric=="healing_score"then targets=1 end
    return analysisView(job,targets or self.analysisTarget or 1,source)
end
local function resultRows(job,summary)
    local rows={};local result=job and job.result;if not result then return rows end
    if result.metric=="item_stats" then
        for _,comparison in ipairs(result.comparisons or {}) do
            local changes={}
            local keys={};for stat in pairs(result.baseline.stats or {})do keys[stat]=true end;for stat in pairs(comparison.stats or {})do keys[stat]=true end
            for stat in pairs(keys) do
                local before=(result.baseline.stats or {})[stat] or 0;local delta=((comparison.stats or {})[stat] or 0)-before
                if delta~=0 then changes[#changes+1]=stat:gsub("ITEM_MOD_",""):gsub("_SHORT",""):gsub("_"," "):lower().." "..string.format("%+g",delta) end
            end
            table.sort(changes);rows[#rows+1]={name=comparison.label or comparison.name,detail=table.concat(changes," · "),comparison=comparison}
        end
    else
        for _,comparison in ipairs(result.comparisons or {}) do
            local name=talentLabel(job,comparison,summary)
            if not name then for _,item in ipairs(summary and summary.candidates or {})do if item.id==comparison.name then name=item.label;break end end end
            if comparison.variant=="max" then name=(name or comparison.label or comparison.name).." · ilvl "..tostring(comparison.level or comparison.maxLevel or "?").." fully upgraded"
            elseif comparison.source~="talent" and comparison.level then name=(name or comparison.label or comparison.name).." · ilvl "..tostring(comparison.level)end
            local detail,value=decisionText(result,comparison)
            rows[#rows+1]={name=name or comparison.name,detail=detail,gain=value and value.percent,decision=value,comparison=comparison}
        end
        table.sort(rows,function(a,b)return (a.gain or -math.huge)>(b.gain or -math.huge)end)
    end
    return rows
end
function U:ComparisonText(job,limit)
    if not job then return "No result yet. Run the full analysis first." end
    if not job.result then return clean(job.error or ("Last loaded status: "..tostring(job.state)),400) end
    local result=job.result;local rows=resultRows(job,self:SummaryFor(job));local lines={}
    for i=1,math.min(limit or 8,#rows)do lines[#lines+1]=clean(rows[i].name,66).."  "..rows[i].detail end
    if #lines==0 then return "Baseline ready: "..fmt(result.baseline and result.baseline.mean).." "..metric(result)..". This total is only a reference; select changes below to get an equip-or-keep recommendation." end
    return table.concat(lines,"\n")
end
local function summaryRows(job,targets,source,variant)
    local view=analysisView(job,targets,source);local rows={}
    if not view then return rows end
    for _,row in ipairs(resultRows(view,U:SummaryFor(job)))do
        local comparison=row.comparison
        if comparison and (not variant or (comparison.variant or "current")==variant)then rows[#rows+1]=row end
    end
    return rows,view
end
local function bestSummaryRow(job,targets,source,variant)
    local rows=summaryRows(job,targets,source,variant)
    return rows[1]
end
function U:BuildSummary(panel)
    panel.heading=label(panel,"",4,-4,550,true);panel.heading:SetMaxLines(1)
    panel.target=dropdown(panel,564,-3,178,analysisTargetChoices,function()return analysisTargetLabel(U:Job("analysis"),U.analysisTarget or 1)end,function(v)U.analysisTarget=chooseAnalysisTarget(v)end)
    resultUnits(panel)
    panel.context=label(panel,"",4,-32,928);panel.context:SetTextColor(.55,.78,.82);panel.context:SetMaxLines(1)
    local owned=card(panel,0,-61,594,386);panel.owned=owned
    panel.ownedTitle=label(owned,"Best upgrades you own now",14,-14,562,true)
    panel.rows={}
    for i=1,6 do
        local row=CreateFrame("Button",nil,owned,"BackdropTemplate");row:SetPoint("TOPLEFT",14,-53-(i-1)*54);row:SetSize(566,50)
        row:SetBackdrop(backdrop);row:SetBackdropColor(.025,.045,.065,i%2==0 and .85 or .55);row:SetBackdropBorderColor(.12,.2,.25,.5)
        row.icon=row:CreateTexture(nil,"ARTWORK");row.icon:SetSize(32,32);row.icon:SetPoint("TOPLEFT",8,-7);row.icon:SetTexCoord(.08,.92,.08,.92)
        row.name=label(row,"",48,-5,300);row.name:SetMaxLines(1)
        row.gain=label(row,"",354,-5,200);row.gain:SetJustifyH("RIGHT");row.gain:SetMaxLines(1)
        row.detail=label(row,"",48,-23,506);row.detail:SetTextColor(.58,.7,.78);row.detail:SetFontObject("GameFontHighlightSmall");row.detail:SetMaxLines(2)
        row:SetScript("OnEnter",function(self)if self.item then U:ItemTip(self,self.item)end end)
        row:SetScript("OnLeave",function()U:HideItemTip()end)
        panel.rows[i]=row
    end
    panel.empty=label(owned,"",16,-65,558);panel.empty:SetSpacing(7)
    local quality=card(panel,612,-61,328,386);panel.quality=quality
    label(quality,"Run quality",14,-14,300,true)
    panel.coverage=label(quality,"",14,-53,300);panel.coverage:SetSpacing(7);panel.coverage:SetMaxLines(11)
    label(quality,"Other useful results",14,-214,300,true)
    panel.other=label(quality,"",14,-251,300);panel.other:SetSpacing(6);panel.other:SetMaxLines(10)
end
function U:RefreshWorkflowDecisions(panel,job,result)
    if not job or not result or result.metric=="healing_score"then return false end
    local report;for _,entry in ipairs(self:Envelope().decisions or{})do for _,view in ipairs(entry.views or{})do if view.jobID==job.id then report=entry;break end end;if report then break end end
    if not report then return false end
    local target=targetResult(result,self.analysisTarget or 1)or(result.targetResults or{})[1];if not target then return false end
    local view;for _,entry in ipairs(report.views or{})do if entry.jobID==job.id and entry.encounter and entry.encounter.id==target.encounterID then view=entry;break end end
    if not view then return false end
    local current=self:Current()or{};local summary=self:SummaryFor(job);local same=summary~=nil
    local equipped={};for _,item in ipairs(current.equipped or{})do equipped[item.slot]=item.value or(item.identity and item.identity.value)end
    local count=0;for _,item in ipairs(summary and summary.equipped or{})do count=count+1;if equipped[item.slot]~=item.value then same=false end end
    if count~=#(current.equipped or{})then same=false end
    local profile=current.snapshot and current.snapshot.profile or"";local talents=profile:match("\ntalents=([^\r\n]+)")or profile:match("^talents=([^\r\n]+)")
    if summary and summary.activeTalents and summary.activeTalents~=talents then same=false end
    setText(panel.heading,same and(view.complete and"Your next upgrades"or"Partial comparison results")or"Your build changed")
    local e=view.encounter;setText(panel.context,clean(e.name).." · "..e.targets.." targets · "..e.seconds.."s · Bloodlust "..(e.bloodlust or"default").." · "..validationText(result.validation))
    panel.target:Show();panel.target:SetText(analysisTargetLabel(job,target.targets));setText(panel.ownedTitle,"Equip · Upgrade · Farm")
    local available,owned={},{ };for _,item in ipairs(self:WorkflowItems())do local key=(item.source or"")..":"..(item.value or(item.identity and item.identity.value)or"");available[key]=(available[key]or 0)+1;if item.source~="vault"then local id=currentItemID(item);if id then owned[id]=true end end end
    local actions={}
    if same then
        for _,group in ipairs({{"Equip",view.equip},{"Upgrade",view.upgrade}})do
            local added=0;for _,choice in ipairs(group[2]or{})do
                local ready=true;local used={};for _,item in ipairs(choice.items or{})do if(item.id or 0)>0 then local key=(item.source or"")..":"..(item.baseKey or item.key or"");used[key]=(used[key]or 0)+1;if used[key]>(available[key]or 0)then ready=false end end end
                if ready and added<3 then actions[#actions+1]={category=group[1],name=choice.label or choice.comparison,percent=choice.percent,absolute=choice.absolute,uncertain=choice.withinMargin,choice=choice,detail=(choice.withinMargin and"Within the sim margin"or"Modeled improvement")..(choice.investment and" · costs not checked"or"")};added=added+1 end
            end
        end
        local context=report.context~=""and report.context or(self:WorkflowSettings()).context;local added=0
        for _,choice in ipairs(view.farm or{})do local item=choice.item
            local kind=string.lower(item.kind or"")
            if added<2 and not owned[item.id]and(context=="raid"and kind=="raid"or context=="m+"and kind=="dungeon")then
                actions[#actions+1]={category="Farm",name=item.label,percent=item.gain,choice=item,detail=clean(item.source or"")..(item.poolComplete and""or" · incomplete source pool")};added=added+1
            end
        end
    end
    panel.empty:SetShown(#actions==0);setText(panel.empty,same and"No positive, available gear comparison completed for this encounter. Use Gear or Drops to test additional choices."or"Run Full analysis again before using recommendations for your new gear or talents.")
    for index,row in ipairs(panel.rows)do local action=actions[index];row:SetShown(action~=nil)
        if action then
            local swap=self:SwapInfo(job,action.choice,target.baseline);row.item=swap.item;row.compareText=swap.tooltip;row.icon:SetTexture(self:ItemTexture(swap.item));row.icon:SetShown(swap.item~=nil)
            setText(row.name,action.category.." · "..clean(action.name,65));setText(row.gain,(action.uncertain and"≈ "or"")..(resultDisplayMode()=="absolute"and action.absolute and(signed(action.absolute).." "..throughputUnit(result))or gain(action.percent)));setText(row.detail,swap.text~=""and swap.text or action.detail);row.gain:SetTextColor(action.uncertain and 1 or .4,action.uncertain and .82 or 1,.4)
        else row.item=nil;row.compareText=nil end
    end
    local coverage={};for _,entry in ipairs(result.targetResults or{})do coverage[#coverage+1]=analysisTargetLabel(job,entry.targets)..": "..entry.tested.." / "..entry.requested.." tested"..(entry.complete and""or" · partial")end
    setText(panel.coverage,table.concat(coverage,"\n").."\n\n"..validationText(result.validation))
    setText(panel.other,"Gains describe each complete tested change.\n\nCommunity Build trends are supporting evidence, not personal BiS.\n\nRaid/boss usage is not connected.")
    return true
end
function U:RefreshSummary(panel)
    local job=self:Job("analysis");local result=job and job.result
    if self:RefreshWorkflowDecisions(panel,job,result)then return end
    if not result or not result.targetResults then
        setText(panel.heading,"No full analysis result yet")
        local baseline=self:Job("baseline")
        setText(panel.context,baseline and "The last run measured equipped gear only; it did not compare the items in your bags." or "Run a full analysis from Setup to compare your captured gear and talents.")
        panel.target:Hide();for _,row in ipairs(panel.rows)do row:Hide()end
		panel.empty:Show();panel.empty:SetTextColor(.86,.91,.95);setText(panel.empty,"Open Setup and run Full analysis. This page will then show the strongest improvements from items you already own.")
        setText(panel.coverage,"No comparison coverage is available.\n\nA baseline number alone cannot identify an upgrade.")
        setText(panel.other,"Bag, talent, Vault and upgrade-potential results will appear after a full analysis.")
        return
    end
    local healer=result.metric=="healing_score";local targets=healer and 1 or self.analysisTarget or 1;local target=targetResult(result,targets)or {}
    local complete=target.complete==true or (target.requested or 0)>0 and target.tested==target.requested
    setText(panel.heading,complete and "Full analysis complete" or "Partial analysis available")
    local modelDependent=healer and result.validation~="reviewed"
    local encounter=analysisTargetLabel(job,targets)
    setText(panel.context,healer and (((job.scenario and job.scenario.healingMode=="dungeon")and"Dungeon"or"Raid").." healing preset · QE relative score, not predicted HPS · "..validationText(result.validation))or(encounter.." · compared with the gear equipped at "..stamp(job.createdAt).." · "..validationText(result.validation)))
    panel.target:SetShown(not healer);panel.target:SetText(encounter)
    local rows=summaryRows(job,targets,"owned","current");local clear={}
    for _,row in ipairs(rows)do if row.decision and row.decision.absolute>0 and not row.decision.uncertain and row.comparison and row.comparison.refined==true then clear[#clear+1]=row end end
    setText(panel.ownedTitle,modelDependent and(#clear>0 and("QE model favors these swaps · verify before equipping · "..#clear.." found")or"No bag item favored by the QE model")or(#clear>0 and ("Best upgrades you own now · "..#clear.." found") or "No clear bag upgrade found"))
    panel.ownedTitle:SetTextColor(modelDependent and 1 or .86,modelDependent and .82 or .91,modelDependent and .26 or .95)
    panel.empty:SetShown(#clear==0)
    if #clear==0 then
        local closest=rows[1]
        setText(panel.empty,closest and ((modelDependent and"The QE model did not favor a bag swap. Closest result:\n\n"or"Nothing tested clearly beat your equipped gear. Closest result:\n\n")..clean(closest.name,72).."  "..closest.detail..(closest.comparison and closest.comparison.refined~=true and" · screening estimate"or"")) or "No compatible bag items completed for this view.")
		decisionColor(panel.empty,closest and closest.decision,false,modelDependent)
    end
    for i,row in ipairs(panel.rows)do
        local item=clear[i];row:SetShown(item~=nil)
        if item then
            local swap=self:SwapInfo(job,item.comparison,target.baseline);row.item=swap.item;row.compareText=swap.tooltip;row.icon:SetTexture(self:ItemTexture(swap.item));row.icon:SetShown(swap.item~=nil)
            setText(row.name,clean(item.name,62));setText(row.gain,(modelDependent and"model "or"")..resultGainText(result,item.decision));decisionColor(row.gain,item.decision,false,modelDependent)
            local replacement=replacementText(target.baseline,item.comparison)
            setText(row.detail,modelDependent and((replacement and("Replaces "..replacement.." · ")or"")..signed(item.decision.absolute).." relative score")or(signed(item.decision.absolute).." "..throughputUnit(result).." · "..fmt(item.decision.mean).." total"))
            if swap.text~=""then setText(row.detail,swap.text..(modelDependent and(" · "..signed(item.decision.absolute).." relative score")or""))end
            if modelDependent then row:SetBackdropBorderColor(.72,.52,.12,.9)else row:SetBackdropBorderColor(.16,.7,.42,.9)end
        else row.item=nil;row.compareText=nil end
    end
    local coverage={}
    for _,count in ipairs(healer and{1}or{1,5})do
        local value=targetResult(result,count)
        if value then
            local done=value.complete==true or (value.requested or 0)>0 and value.tested==value.requested
            local total=not healer and value.baseline and num(value.baseline.mean)
            coverage[#coverage+1]=(healer and"Healing gear"or analysisTargetLabel(job,count))..": "..(total and(fmt(total).." "..throughputUnit(result).." · ")or"")..(done and "Complete" or "Partial")
            coverage[#coverage+1]="  "..tostring(value.tested or 0).." / "..tostring(value.requested or 0).." variants tested"
            coverage[#coverage+1]="  "..tostring(value.refined or 0)..(healer and" gear changes scored · "or" strongest results refined · ")..string.format("%.1fs",tonumber(value.elapsedSeconds)or 0)
        end
    end
    local rejected=#(target.rejectedComparisons or {})
    if rejected>0 then coverage[#coverage+1]=rejected.." incompatible variant"..(rejected==1 and "" or "s").." skipped" end
    setText(panel.coverage,table.concat(coverage,"\n"))
    local other={}
    local potential=bestSummaryRow(job,targets,"owned","max")
    local crest=bestSummaryRow(job,targets,"equipped","next")
    local talent=bestSummaryRow(job,targets,"talent","talent")
    local vault=bestSummaryRow(job,targets,"vault","current")
    local function add(prefix,row)
        if row and row.decision then
			local estimate=row.comparison and row.comparison.refined~=true
			other[#other+1]=prefix..": "..clean(row.name,34).."  "..resultGainText(result,row.decision,estimate)
		end
    end
    add("Best next crest rank",crest);add("Fully upgraded potential",potential);add("Saved talent",talent);add("Vault choice",vault)
    if #other==0 then other[1]="No additional upgrade, talent or Vault result completed for this view." end
    setText(panel.other,table.concat(other,"\n"))
end
function U:LoadoutJob()
	for _,job in ipairs(self:Jobs(false))do
		if taskOf(job)=="compare"and job.scenario and job.scenario.compareMode=="loadout"then return job end
	end
end
local function copyItem(item)
	if type(item)~="table"then return end
	local out={};for key,value in pairs(item)do out[key]=value end;return out
end
local function loadoutBase(ui,job)
	local summary=job and ui:SummaryFor(job);local source=summary and summary.equipped or (ui:Current()or{}).equipped or{}
	local out={};for _,item in ipairs(source)do out[item.slot]=copyItem(item)end;return out,summary
end
local function dualWields(ui)
	local current=ui:Current();local character=current and current.snapshot and current.snapshot.character or{}
	local class=tostring(character.class or""):upper();local spec=tonumber(character.spec)
	return class=="ROGUE"or class=="DEMONHUNTER"or(class=="WARRIOR"and spec==72)or(class=="DEATHKNIGHT"and spec==251)or(class=="SHAMAN"and spec==263)or(class=="MONK"and spec==269)
end
local function sameFamily(ui,slot,item)
	local itemSlot=item and item.slot
	if slot==itemSlot then return true end
	for _,family in ipairs({"finger","trinket"})do
		if tostring(slot):match("^"..family)and tostring(itemSlot):match("^"..family)then return true end
	end
	if slot=="off_hand"and itemSlot=="main_hand"and item.inventoryType=="INVTYPE_WEAPON"and dualWields(ui)then return true end
	return false
end
function U:WorkflowSlotMatches(slot,item)
    if not item.inventoryType then
        for _,candidate in ipairs(self:WorkflowItems())do if candidate.source==item.source and candidate.slot==item.slot and candidate.value==item.value then item=candidate;break end end
    end
    return sameFamily(self,slot,item)
end
function U:ItemID(item)return currentItemID(item or{})end
local function loadoutCandidate(summary,current,resultItem)
	if not resultItem or resultItem.key==""then return end
	for _,group in ipairs({summary and summary.candidates or{},current and current.candidates or{}})do
		for _,item in ipairs(group)do
			if(resultItem.candidateID and resultItem.candidateID==item.id or item.value==resultItem.key)and(not resultItem.source or resultItem.source==""or item.source==resultItem.source)then
				local found=copyItem(item);local level=tonumber(resultItem.level)
				if level and level>0 and level~=itemLevel(found)then found.plannedLevel=level;found.plannedRank=plannedRank(found,level)end
				return found
			end
		end
	end
	return {id=resultItem.candidateID or resultItem.id,label=resultItem.label,source=resultItem.source,slot=resultItem.slot,value=resultItem.key,level=resultItem.level,itemID=resultItem.id}
end
function U:LoadoutState()
	local savedJob=self:LoadoutJob();local useResult=savedJob and savedJob.result and not self.loadoutDirty
	local base,summary=loadoutBase(self,useResult and savedJob or nil);local gear={};for slot,item in pairs(base)do gear[slot]=copyItem(item)end
	local modified={};local comparison,value
	if useResult then
		comparison=savedJob.result.comparisons and savedJob.result.comparisons[1];value=decision(savedJob.result,comparison)
		for _,resultItem in ipairs(comparison and comparison.items or{})do
			modified[resultItem.slot]=true;gear[resultItem.slot]=loadoutCandidate(summary,self:Current(),resultItem)
		end
	else
		for slot,item in pairs(self.loadout)do modified[slot]=true;gear[slot]=item and copyItem(item)or nil end
		local main=gear.main_hand
		if main and main.inventoryType=="INVTYPE_2HWEAPON"then if gear.off_hand then modified.off_hand=true end;gear.off_hand=nil end
	end
	return {base=base,gear=gear,modified=modified,job=useResult and savedJob or nil,comparison=comparison,value=value,summary=summary}
end
function U:MaterializeLoadout(state)
	state=state or self:LoadoutState();if not state.job then return end
	self.loadout={};for slot in pairs(state.modified)do self.loadout[slot]=state.gear[slot]and copyItem(state.gear[slot])or false end
	local request=state.summary and state.summary.snapshot and state.summary.snapshot.gameRequest
	local talent=request and request.talents and request.talents[1];settings().loadoutTalent=talent and talent.code or nil
	self.loadoutTarget=self.loadoutTarget or(state.job.scenario and state.job.scenario.targets)or 1
	self.loadoutDirty=true
end
local function loadoutChanged(ui)
	for _,item in pairs(ui.loadout)do if item~=nil then return true end end
	return settings().loadoutTalent~=nil and settings().loadoutTalent~=""
end
local function exactLoadoutAvailable(ui)
	local current=ui:Current();local snapshot=current and current.snapshot;local character=snapshot and snapshot.character
	return snapshot and snapshot.client=="retail"and character and character.role=="DAMAGER"
end
function U:ChooseLoadout(slot,item)
	local state=self:LoadoutState();if state.job then self:MaterializeLoadout(state);state=self:LoadoutState()end
	if slot=="off_hand"and(item or state.base[slot])and state.gear.main_hand and state.gear.main_hand.inventoryType=="INVTYPE_2HWEAPON"then
		self.loadout.main_hand=false
	end
	if item then
		if item.source=="vault"then
			for other,selected in pairs(self.loadout)do
				if other~=slot and type(selected)=="table"and selected.source=="vault"then
					self.notice="A planned loadout can include one Great Vault choice.";self:Refresh();return
				end
			end
		end
		local wanted=identity(item)
		for other,selected in pairs(self.loadout)do if other~=slot and selected and identity(selected)==wanted then self.loadout[other]=nil end end
		local old=self.loadout[slot];self.loadout[slot]=type(old)=="table"and identity(old)==identity(item)and old or copyItem(item)
	else self.loadout[slot]=nil end
	if slot=="main_hand"then
		local main=item or state.base.main_hand
		if main and main.inventoryType=="INVTYPE_2HWEAPON"then self.loadout.off_hand=(state.gear.off_hand or state.base.off_hand)and false or nil
		elseif self.loadout.off_hand==false then self.loadout.off_hand=nil end
	end
	self.loadoutDirty=true;self.loadoutSlot=slot;self.loadoutOptionPage=1;self.notice=nil;self:Refresh()
end
function U:ShiftLoadoutRank(slot,item,delta)
	if type(item)~="table"then return end
	local selected=self.loadout[slot]
	if type(selected)~="table"or identity(selected)~=identity(item)then
		self:ChooseLoadout(slot,item);selected=self.loadout[slot]
	end
	if type(selected)~="table"then return end
	local path,current=rankPath(selected);if #path<2 then return end
	local level=tonumber(selected.plannedLevel)or itemLevel(selected);local index=1
	for i,value in ipairs(path)do if value==level then index=i;break end end
	index=math.max(1,math.min(#path,index+delta));selected.plannedLevel=index==1 and nil or path[index];selected.plannedRank=index==1 and nil or current+index-1
	self.loadoutDirty=true;self.notice=nil;self:Refresh()
end
function U:LoadoutOptions(slot,state)
	local out={{current=true,item=state.base[slot]}};local seen={};local minimum=minimumItemLevel()
	for _,item in ipairs((self:Current()or{}).candidates or{})do
		local weaponOK=true
		if slot=="main_hand"and self.weaponFilter=="One-handed"then weaponOK=item.inventoryType=="INVTYPE_WEAPON"or item.inventoryType=="INVTYPE_WEAPONMAINHAND"or item.inventoryType=="INVTYPE_RANGEDRIGHT"
		elseif slot=="main_hand"and self.weaponFilter=="Two-handed"then weaponOK=item.inventoryType=="INVTYPE_2HWEAPON"end
		if weaponOK and(item.source=="owned"or item.source=="vault")and sameFamily(self,slot,item)and(minimum==0 or(itemLevel(item)or 0)>=minimum)then
			local key=identity(item);if not seen[key]then seen[key]=true;out[#out+1]={item=item}end
		end
	end
	table.sort(out,function(a,b)
		if a.current~=b.current then return a.current==true end
		local ai,bi=a.item or{},b.item or{};if ai.source~=bi.source then return ai.source=="owned"end
		if(itemLevel(ai)or 0)~=(itemLevel(bi)or 0)then return(itemLevel(ai)or 0)>(itemLevel(bi)or 0)end
		return(ai.label or"")<(bi.label or"")
	end)
	return out
end
local function loadoutReady(ui,state)
	local main,off=state.gear.main_hand,state.gear.off_hand
	if off and(not main or main.inventoryType=="INVTYPE_2HWEAPON")then return false end
	if off and off.slot=="main_hand"and off.inventoryType=="INVTYPE_WEAPON"and not dualWields(ui)then return false end
	return true
end
local function buildSheetSlot(panel,slot,x,y)
	local row=CreateFrame("Button",nil,panel,"BackdropTemplate");row:SetPoint("TOPLEFT",x,y);row:SetSize(290,34);row:SetBackdrop(backdrop);row:SetBackdropColor(.025,.045,.065,.9);row:SetBackdropBorderColor(.12,.2,.25,.7)
	row.slotKey=slot;row.icon=row:CreateTexture(nil,"ARTWORK");row.icon:SetSize(30,30);row.icon:SetPoint("LEFT",2,0);row.icon:SetTexCoord(.08,.92,.08,.92)
	row.slotLabel=label(row,sheetSlotNames[slot]or slot,38,-3,67);row.slotLabel:SetFontObject("GameFontHighlightSmall");row.slotLabel:SetTextColor(.56,.7,.78)
	row.nameLabel=label(row,"",102,-3,180);row.nameLabel:SetMaxLines(1);row.detail=label(row,"",102,-18,180);row.detail:SetFontObject("GameFontHighlightSmall");row.detail:SetMaxLines(1)
	row:SetScript("OnClick",function(self)U.loadoutSlot=self.slotKey;U.loadoutOptionPage=1;U:Refresh()end)
	row:SetScript("OnEnter",function(self)if self.item then U:ItemTip(self,self.item)end end);row:SetScript("OnLeave",function()if GameTooltip then GameTooltip:Hide()end end)
	panel.sheet[slot]=row
end
function U:BuildLoadout(panel)
	panel.context=label(panel,"",4,-3,550);panel.context:SetTextColor(.55,.78,.82);panel.context:SetMaxLines(1)
	panel.target=dropdown(panel,564,-3,178,analysisTargetChoices,function()return analysisTargetLabel(U:Job("analysis"),U.loadoutTarget or 1)end,function(v)U:MaterializeLoadout();U.loadoutTarget=chooseAnalysisTarget(v)end)
	resultUnits(panel)
	panel.result=label(panel,"",4,-27,928,true);panel.result:SetMaxLines(1)
	label(panel,"Simulated equipment",4,-54,590,true);panel.sheet={}
	for i,slot in ipairs(sheetLeft)do buildSheetSlot(panel,slot,0,-81-(i-1)*37)end
	for i,slot in ipairs(sheetRight)do buildSheetSlot(panel,slot,300,-81-(i-1)*37)end
	buildSheetSlot(panel,"main_hand",0,-385);buildSheetSlot(panel,"off_hand",300,-385)
	panel.talent=CreateFrame("Button",nil,panel,"BackdropTemplate");panel.talent:SetPoint("TOPLEFT",0,-423);panel.talent:SetSize(590,34);panel.talent:SetBackdrop(backdrop);panel.talent:SetBackdropColor(.025,.045,.065,.9);panel.talent:SetBackdropBorderColor(.12,.2,.25,.7)
	panel.talentLabel=label(panel.talent,"Talents",10,-8,74);panel.talentName=label(panel.talent,"",88,-8,492);panel.talentName:SetMaxLines(1);panel.talent:SetScript("OnClick",function()U:SelectTab("talents")end)
	local choices=card(panel,610,-48,330,409);panel.choices=choices
	panel.choiceTitle=label(choices,"Choose Head",12,-12,142,true)
	panel.weaponFilter=dropdown(choices,160,-7,160,{"All weapons","One-handed","Two-handed"},function()return U.weaponFilter or"All weapons"end,function(v)U.weaponFilter=v;U.loadoutOptionPage=1 end)
	panel.pairing=label(choices,"",12,-39,140);panel.pairing:SetTextColor(.6,.75,.82);panel.pairing:SetMaxLines(2)
	panel.minLevelLabel=label(choices,"Min ilvl (0 = all)",160,-42,88);panel.minLevelLabel:SetFontObject("GameFontHighlightSmall")
	panel.minLevel=minimumItemLevelEdit(choices,250,-37,70)
	panel.options={}
	for i=1,6 do
		local row=CreateFrame("Button",nil,choices,"BackdropTemplate");row:SetPoint("TOPLEFT",10,-82-(i-1)*42);row:SetSize(310,38);row:SetBackdrop(backdrop);row:SetBackdropColor(.025,.045,.065,i%2==0 and .9 or .65);row:SetBackdropBorderColor(.12,.2,.25,.6)
		row.icon=row:CreateTexture(nil,"ARTWORK");row.icon:SetSize(32,32);row.icon:SetPoint("LEFT",3,0);row.icon:SetTexCoord(.08,.92,.08,.92)
		row.nameLabel=label(row,"",40,-4,198);row.nameLabel:SetMaxLines(1);row.detail=label(row,"",40,-21,198);row.detail:SetFontObject("GameFontHighlightSmall");row.detail:SetMaxLines(1)
		row.minus=button(row,"−",246,-6,28,function()if row.choice and not row.choice.current then U:ShiftLoadoutRank(U.loadoutSlot,row.choice.item,-1)end end)
		row.plus=button(row,"+",278,-6,28,function()if row.choice and not row.choice.current then U:ShiftLoadoutRank(U.loadoutSlot,row.choice.item,1)end end)
		row:SetScript("OnClick",function(self)if self.choice then U:ChooseLoadout(U.loadoutSlot,self.choice.current and nil or self.choice.item)end end)
		row:SetScript("OnEnter",function(self)if self.choice and self.choice.item then U:ItemTip(self,self.choice.item)end end);row:SetScript("OnLeave",function()if GameTooltip then GameTooltip:Hide()end end);panel.options[i]=row
	end
	panel.optionPrev=button(choices,"Previous",10,-337,86,function()U.loadoutOptionPage=math.max(1,(U.loadoutOptionPage or 1)-1);U:Refresh()end)
	panel.optionNext=button(choices,"Next",102,-337,70,function()U.loadoutOptionPage=(U.loadoutOptionPage or 1)+1;U:Refresh()end)
	panel.optionCount=label(choices,"",182,-343,138);panel.optionCount:SetFontObject("GameFontHighlightSmall")
	panel.reset=button(choices,"Reset all",10,-371,92,function()U.loadout={};U.loadoutDirty=true;settings().loadoutTalent=nil;U.notice=nil;U:Refresh()end)
	panel.run=button(choices,"Sim this loadout",108,-371,212,function()U:RunLoadout()end)
end
function U:RunLoadout()
	local state=self:LoadoutState();if state.job then self:MaterializeLoadout(state)end
	local selected={};for slot,item in pairs(self.loadout)do if type(item)=="table"then selected[identity(item)]={placement=slot,plannedLevel=item.plannedLevel}end end
	local talents={};local code=settings().loadoutTalent
	if code and code~=""then for _,build in ipairs(self:Builds())do if build.code==code then talents[1]={label=build.label,code=build.code};break end end end
	local scenario=self:Scenario();local target=self.loadoutTarget or 1
	local reference=targetResult((self:Job("analysis")or{}).result,target)
	if reference and reference.fightStyle=="dungeon_slice"then
		scenario.targets=1;scenario.seconds=360;scenario.fightStyle="dungeon_slice";scenario.raidBuffs="none"
	else scenario.targets=target;scenario.fightStyle="patchwerk"end
	self:Send("compare",{compareMode="loadout",selectedSet=selected,talents=talents,scenario=scenario})
end
function U:RefreshLoadout(panel)
	syncMinimumItemLevel()
	local state=self:LoadoutState();local changed=0;for _ in pairs(state.modified)do changed=changed+1 end
	local talentCode=settings().loadoutTalent;local talentName="Captured equipped build"
	for _,build in ipairs(self:Builds())do if build.code==talentCode then talentName=build.label;break end end
	local talentChanged=talentCode~=nil and talentCode~=""
	local targets=self.loadoutTarget or(state.job and state.job.scenario and state.job.scenario.targets)or 1
	local available=exactLoadoutAvailable(self)
	local current=self:Current();local healer=current and current.snapshot and current.snapshot.character and current.snapshot.character.role=="HEALER"
	setText(panel.context,state.job and(analysisTargetLabel(self:Job("analysis"),targets).." · exact simulated placement · "..stamp(state.job.createdAt))or available and"Build the exact set you want to test. Cyan outlines are not currently equipped."or healer and"This healing model can rank one gear change at a time, but cannot apply an exact combined loadout."or"Exact combined loadouts require a supported Retail damage specialization.")
	if state.value then setText(panel.result,state.value.verdict.." · "..resultGainText(state.job.result,state.value));decisionColor(panel.result,state.value,false)
	elseif changed>0 or talentChanged then setText(panel.result,"Planned loadout · "..changed.." changed equipment slot"..(changed==1 and""or"s")..(talentChanged and" · saved talent build"or"").." · not simulated yet");panel.result:SetTextColor(.45,.85,.95)
	else setText(panel.result,"Captured equipment and talents · baseline");panel.result:SetTextColor(.7,.78,.84)end
	panel.target:SetText(analysisTargetLabel(self:Job("analysis"),targets))
	for slot,row in pairs(panel.sheet)do
		local item=state.gear[slot];row.item=item;local modified=state.modified[slot]
		row.icon:SetTexture(item and(item.icon or safe(C_Item and C_Item.GetItemIconByID or GetItemIcon,currentItemID(item)))or 134400)
		setText(row.nameLabel,item and clean(item.label or tostring(currentItemID(item)or"Item"),34)or"Empty")
		local source=modified and item and(item.source=="vault"and"VAULT"or"BAG")or modified and"REMOVED"or item and"EQUIPPED"or"EMPTY"
		setText(row.detail,sheetItemDetail(item,source))
		if modified then row:SetBackdropBorderColor(.1,.85,1,1);row.detail:SetTextColor(.2,.9,1)
		elseif self.loadoutSlot==slot then row:SetBackdropBorderColor(.85,.7,.2,1);row.detail:SetTextColor(.58,.7,.78)
		else row:SetBackdropBorderColor(.12,.2,.25,.7);row.detail:SetTextColor(.58,.7,.78)end
	end
	setText(panel.talentName,talentName..(talentChanged and" · selected for this loadout"or""));panel.talent:SetBackdropBorderColor(talentChanged and .1 or .12,talentChanged and .85 or .2,talentChanged and 1 or .25,talentChanged and 1 or .7)
	local main,off=state.gear.main_hand,state.gear.off_hand
	local weaponText="Main: "..clean(main and main.label or"Empty",25).."\nOff-hand: "..clean(off and off.label or"Empty",25)
	setText(panel.pairing,weaponText)
	local slot=self.loadoutSlot or"head";self.loadoutSlot=slot;setText(panel.choiceTitle,"Choose "..(sheetSlotNames[slot]or slot));panel.weaponFilter:SetShown(slot=="main_hand")
	local options=self:LoadoutOptions(slot,state);local pages=math.max(1,math.ceil(#options/6));self.loadoutOptionPage=math.min(math.max(1,self.loadoutOptionPage or 1),pages)
	for i,row in ipairs(panel.options)do local choice=options[(self.loadoutOptionPage-1)*6+i];row.choice=choice;row:SetShown(choice~=nil)
		if choice then
			local item=choice.item;setText(row.nameLabel,choice.current and(item and clean(item.label,32)or"Empty")or clean(item.label or"Item",32))
			local selected=not choice.current and type(self.loadout[slot])=="table"and identity(self.loadout[slot])==identity(item);local shown=selected and self.loadout[slot]or item
			local detail=sheetItemDetail(shown,choice.current and"Equipped"or item.source=="vault"and"Vault"or"Bag")
			setText(row.detail,detail);local id=item and currentItemID(item);row.icon:SetTexture(item and(item.icon or safe(C_Item and C_Item.GetItemIconByID or GetItemIcon,id))or 134400)
			local path=not choice.current and rankPath(item)or{};local level=selected and(tonumber(shown.plannedLevel)or itemLevel(shown))or itemLevel(item);local position=1
			for index,value in ipairs(path)do if value==level then position=index;break end end
			row.minus:SetShown(not choice.current and #path>1);row.plus:SetShown(not choice.current and #path>1);row.minus:SetEnabled(selected and position>1);row.plus:SetEnabled(#path>1 and position<#path)
			row:SetBackdropBorderColor(selected and .1 or .12,selected and .85 or .2,selected and 1 or .25,selected and 1 or .6)
		end
	end
	panel.optionPrev:SetEnabled(self.loadoutOptionPage>1);panel.optionNext:SetEnabled(self.loadoutOptionPage<pages);setText(panel.optionCount,self.loadoutOptionPage.." / "..pages)
	local ready=loadoutReady(self,state);if not ready then setText(panel.result,"Choose a compatible one-handed main hand for the selected off-hand.");panel.result:SetTextColor(1,.82,.26)end
	panel.run:SetText(state.job and"Run this loadout again"or"Sim this loadout");panel.run:SetEnabled(available and ready and not transferBusy()and(state.job~=nil or loadoutChanged(self)));panel.reset:SetEnabled(changed>0 or talentChanged or state.job~=nil)
end
function U:BuildGear(panel,source)
    panel.resultContext=label(panel,"",4,-3,550);panel.resultContext:SetTextColor(.55,.78,.82);panel.resultContext:SetMaxLines(1);panel.resultContext:SetWordWrap(false)
	panel.target=dropdown(panel,564,-3,178,analysisTargetChoices,function()return analysisTargetLabel(U:Job("analysis"),U.analysisTarget or 1)end,function(v)U.analysisTarget=chooseAnalysisTarget(v)end)
    resultUnits(panel)
    panel.summary=label(panel,"",4,-27,934);panel.summary:SetSpacing(2);panel.summary:SetFontObject("GameFontHighlightSmall");panel.summary:SetMaxLines(3)
    panel.search=edit(panel,4,-84,260,"",80);panel.search:SetScript("OnTextChanged",function(self)U.search=self:GetText();U.page=1;U:RefreshGear(panel,source)end)
    panel.slot=dropdown(panel,278,-84,172,slotOptions,function()return U.slot or "All slots"end,function(v)U.slot=v;U.page=1 end)
	panel.minLevelLabel=label(panel,"Min ilvl",470,-90,62);panel.minLevelLabel:SetFontObject("GameFontHighlightSmall")
	panel.minLevel=minimumItemLevelEdit(panel,536,-84,78)
    panel.compareSelected=button(panel,"Compare selected items",624,-84,180,function()U:Send("analysis",{returnTab="summary"})end)
    panel.allResults=button(panel,"All results",812,-84,118,function()U:ShowAllResults(panel.displayJob)end)
    label(panel,"Item",42,-121,320,true);label(panel,"Equip now",375,-121,134,true)
    label(panel,"Maxed vs today",519,-121,138,true);label(panel,"Maxed vs maxed",664,-121,162,true)
    label(panel,"Venomstones",836,-121,92,true)
    panel.rows={}
    for index=1,10 do
        local row=CreateFrame("Button",nil,panel,"BackdropTemplate");row:SetPoint("TOPLEFT",4,-149-(index-1)*29);row:SetSize(926,27)
        row:SetBackdrop(backdrop);row:SetBackdropColor(.025,.045,.065,index%2==0 and .85 or .55);row:SetBackdropBorderColor(.12,.2,.25,.5)
        row.icon=row:CreateTexture(nil,"ARTWORK");row.icon:SetSize(24,24);row.icon:SetPoint("LEFT",3,0);row.icon:SetTexCoord(.08,.92,.08,.92)
        row.nameLabel=label(row,"",34,-5,225);row.nameLabel:SetMaxLines(1)
        row.slot=label(row,"",266,-5,100);row.slot:SetTextColor(.58,.7,.78)
        row.current=label(row,"",371,-5,135);row.current:SetMaxLines(1)
        row.maximum=label(row,"",515,-5,140);row.maximum:SetMaxLines(1)
        row.normalized=label(row,"",660,-5,164);row.normalized:SetMaxLines(1)
        row.venomstone=U:BuildVenomstoneButton(row,832,-1,90)
        row:SetScript("OnEnter",function(self)if self.item then U:ItemTip(self,self.item)end end)
        row:SetScript("OnLeave",function()if GameTooltip then GameTooltip:Hide()end end);panel.rows[index]=row
    end
    panel:EnableMouseWheel(true);panel:SetScript("OnMouseWheel",function(_,delta)U.page=math.max(1,(U.page or 1)-(delta>0 and 1 or -1));U:Refresh()end)
    panel.empty=label(panel,"",8,-210,900)
    panel.prev=button(panel,"Previous",0,-455,100,function()U.page=math.max(1,U.page-1);U:Refresh()end)
    panel.next=button(panel,"Next",110,-455,90,function()U.page=U.page+1;U:Refresh()end)
    panel.count=label(panel,"",214,-461,705)
end
function U:RefreshGear(panel,source)
	syncMinimumItemLevel()
    local analysis=self:AnalysisJob(source,self.analysisTarget or 1);local job=analysis or self:Job("compare",source)
    local displayResult=job and job.result
    local healer=displayResult and displayResult.metric=="healing_score"
    local modelDependent=healer and displayResult.validation~="reviewed"
    panel.displayJob=job
    if analysis and job==analysis then
        local target=targetResult(self:Job("analysis").result,healer and 1 or self.analysisTarget or 1)or {};local coverage=tostring(target.tested or 0).." / "..tostring(target.requested or 0).." variants"
        setText(panel.resultContext,(healer and(modelDependent and"QE model estimate"or"Healing preset")or analysisTargetLabel(self:Job("analysis"),self.analysisTarget or 1)).." · "..coverage.." tested · "..tostring(target.refined or 0).." scored · "..stamp(analysis.createdAt))
    else
        setText(panel.resultContext,job and ((job.result and "Saved comparison" or "Request "..tostring(job.state)).." · "..stamp(job.createdAt)) or "Run a full analysis to rank every captured item.")
    end
    panel.target:SetShown(not healer and self:AnalysisJob(source,self.analysisTarget or 1)~=nil);panel.target:SetText(analysisTargetLabel(self:Job("analysis"),self.analysisTarget or 1))
    panel.allResults:SetEnabled(job and job.result~=nil)
    local workflow=self:WorkflowSettings();panel.compareSelected:SetEnabled(workflow.enabled and#workflow.selected>0 and not transferBusy()and not(InCombatLockdown and InCombatLockdown()))
    local metrics,equippedMax={},{};for _,comparison in ipairs(displayResult and displayResult.comparisons or {})do
        local id=comparison.candidateID or comparison.name;if id then metrics[id]=metrics[id]or {};metrics[id][comparison.variant or "current"]=comparison end
    end
    local equippedAnalysis=analysis and self:AnalysisJob("equipped",self.analysisTarget or 1)
    for _,comparison in ipairs(equippedAnalysis and equippedAnalysis.result and equippedAnalysis.result.comparisons or {})do
        if comparison.variant~="all_max" and comparison.slot then
            local old=equippedMax[comparison.slot];if not old or (comparison.level or 0)>(old.level or 0)then equippedMax[comparison.slot]=comparison end
        end
    end
    local summary=job and self:SummaryFor(analysis and self:Job("analysis")or job);local captured={}
    for _,item in ipairs(summary and summary.candidates or{})do captured[self:WorkflowItemKey(item)]=item.id end
    local items=self:Candidates(source);local ranked={}
    local live={};for _,item in ipairs((self:Current()or{}).candidates or {})do live[identity(item)]=item end
    for _,item in ipairs(items)do
        local current=live[identity(item)];if current then for _,key in ipairs({"link","icon","quality","itemID"})do if current[key]~=nil then item[key]=current[key]end end end
        local savedID=summary and captured[self:WorkflowItemKey(item)]or(not summary and item.id)
        local pair=metrics[savedID]or {};local best=-math.huge;local result=analysis and analysis.result
        local currentValue=decision(result,pair.current);if currentValue then best=math.max(best,currentValue.percent)end
        local maxReference=pair.max and (equippedMax[pair.max.slot]or(result and result.baseline))
        local fairValue=maxReference and comparisonDecision(maxReference,pair.max)
        if fairValue then best=math.max(best,fairValue.percent)end
        ranked[#ranked+1]={item=item,pair=pair,maxReference=maxReference,best=best}
    end
    table.sort(ranked,function(a,b)if a.best~=b.best then return a.best>b.best end return (a.item.label or "")<(b.item.label or "")end)
	local gains,close,losses,estimates=0,0,0,0
	for _,entry in ipairs(ranked)do
		local comparison=entry.pair.current;local value=decision(displayResult,comparison)
		if comparison and value then
			if analysis and comparison.refined~=true then estimates=estimates+1
			elseif value.uncertain or value.percent==0 then close=close+1
			elseif value.percent>0 then gains=gains+1 else losses=losses+1 end
		end
	end
	if displayResult then
		local first=(modelDependent and"QE model favors: "or"Equip now: ")..gains..(modelDependent and" item"or" clear gain")..(gains==1 and "" or "s").." · "..close.." close · "..losses.." worse"
		if estimates>0 then first=first.." · "..estimates.." screening estimate"..(estimates==1 and "" or "s") end
		local second=healer and"Each row changes one item. +/− is a relative QE score estimate, not predicted HPS."or"Each row changes one item. +/− is the change in "..throughputUnit(displayResult).."; ≈ is within the sim margin."
		local third="Maxed vs maxed compares the bag item and your equipped item at their highest captured ranks."
		setText(panel.summary,first.."\n"..second.."\n"..third)
	else setText(panel.summary,self:ComparisonText(job,3)) end
    local pages=math.max(1,math.ceil(#ranked/10));self.page=math.min(math.max(1,self.page),pages)
    local function outcome(comparison,prefix,reference)
        if not comparison or not displayResult then return prefix.."Not tested",nil end
        local value=reference and comparisonDecision(reference,comparison)or decision(displayResult,comparison);if not value then return prefix.."Unavailable",nil end
		local estimate=analysis and comparison.refined~=true
        local text=resultGainText(displayResult,value,estimate)
		return prefix..text,value
    end
	local function color(field,value,comparison)
		decisionColor(field,value,analysis and comparison and comparison.refined~=true,modelDependent)
	end
    for index,row in ipairs(panel.rows) do
        local entry=ranked[(self.page-1)*10+index];local item=entry and entry.item;row.item=item;row:SetShown(item~=nil)
        if item then
            local id=currentItemID(item);local api=C_Item or {};local icon=item.icon or safe(api.GetItemIconByID or GetItemIcon,id)
            if not icon and api.GetItemInfoInstant then local _,_,_,_,texture=safe(api.GetItemInfoInstant,id);icon=texture end
            row.icon:SetTexture(icon or 134400);setText(row.nameLabel,clean(item.label or tostring(id or "Item"),70));setText(row.slot,(slotNames[item.slot]or item.slot or "").."  "..tostring(itemLevel(item)or ""))
            local current,currentValue=outcome(entry.pair.current,"")
            local maximum,maxValue=outcome(entry.pair.max,(item.maxLevel and "ilvl "..item.maxLevel.."  " or ""))
            local normalized,fairValue=outcome(entry.pair.max,"",entry.maxReference)
            if not item.maxLevel then maximum="Not upgradeable";normalized="—" end
            setText(row.current,current);setText(row.maximum,maximum);setText(row.normalized,normalized)
			self:RefreshVenomstoneButton(row,workflow)
			color(row.current,currentValue,entry.pair.current);color(row.maximum,maxValue,entry.pair.max);color(row.normalized,fairValue,entry.pair.max)
			local value=currentValue or fairValue or maxValue
			local comparison=entry.pair.current or entry.pair.max
			decisionBorder(row,value,analysis and comparison and comparison.refined~=true,modelDependent)
        end
    end
    panel.empty:SetShown(#ranked==0)
    setText(panel.empty,not analysis and (job and "This saved comparison has no matching item rows. Run Full analysis from Setup to test everything you own." or "No full analysis has tested these items. Open Setup and run Full analysis.") or source=="vault" and "No matching Vault choices. Open your Vault, then refresh character data in Setup." or "No matching bag equipment. Clear filters or refresh character data in Setup.")
    panel.prev:SetEnabled(self.page>1);panel.next:SetEnabled(self.page<pages)
    setText(panel.count,#ranked.." items · "..#workflow.selected.." selected · page "..self.page.." / "..pages.." · Venomstones: compare normal vs max rank + 10 stones")
    panel.slot:SetText(self.slot or "All slots")
end
local function crestPath(item)
    return rankPath(item)
end
local function sameCrestEquipment(current,saved)
    local bySlot,count={},0
    for _,item in ipairs(current or {})do
        if item.slot~="shirt" and item.slot~="tabard" then bySlot[item.slot]=item;count=count+1 end
    end
    for _,item in ipairs(saved or {})do
        if item.slot~="shirt" and item.slot~="tabard" then
            local live=bySlot[item.slot]
            if not live or live.value~=item.value then return false end
            count=count-1
        end
    end
    return count==0
end
local function crestRankText(result,item,rank,maxRank,level,value,current,comparison,screening)
    local track=clean(item.track or "",40);local text=(track~=""and(track.." ")or"Rank ")..rank.."/"..maxRank.." · "..tostring(level or "?")
	if rank==current then return text.." · "..(value and(resultGainText(result,value).." vs equipped")or"Equipped")end
	if not value then return text.." · Not tested"end
	return text.." · "..resultGainText(result,value,screening and comparison and comparison.refined~=true)
end
function U:ShiftCrest(row,delta)
    local entry=row and row.crestEntry;if not entry then return end
    local selected=tonumber(self.crestRanks[entry.key])or entry.currentRank
    self.crestRanks[entry.key]=math.max(entry.currentRank,math.min(entry.maxRank,selected+delta));self:Refresh()
end
function U:BuildCrest(panel)
    panel.resultContext=label(panel,"",4,-3,550);panel.resultContext:SetTextColor(.55,.78,.82);panel.resultContext:SetMaxLines(1);panel.resultContext:SetWordWrap(false)
    panel.target=dropdown(panel,564,-3,178,analysisTargetChoices,function()return analysisTargetLabel(U:Job("analysis"),U.analysisTarget or 1)end,function(v)U.analysisTarget=chooseAnalysisTarget(v)end)
    resultUnits(panel)
    local total=card(panel,4,-35,926,70);panel.total=total
    panel.totalTitle=label(total,"All upgradeable equipped gear at maximum",14,-12,520,true)
    panel.totalValue=label(total,"",548,-14,360,true);panel.totalValue:SetJustifyH("RIGHT")
    panel.totalDetail=label(total,"",14,-41,890);panel.totalDetail:SetTextColor(.58,.7,.78)
    label(panel,"Equipped item",42,-120,238,true);label(panel,"Equipped baseline",290,-120,164,true)
    label(panel,"Planned vs equipped",496,-120,174,true);label(panel,"Maximum vs equipped",710,-120,210,true)
    panel.rows={}
    for index=1,10 do
        local row=CreateFrame("Button",nil,panel,"BackdropTemplate");row:SetPoint("TOPLEFT",4,-148-(index-1)*29);row:SetSize(926,27)
        row:SetBackdrop(backdrop);row:SetBackdropColor(.025,.045,.065,index%2==0 and .85 or .55);row:SetBackdropBorderColor(.12,.2,.25,.5)
        row.icon=row:CreateTexture(nil,"ARTWORK");row.icon:SetSize(24,24);row.icon:SetPoint("LEFT",3,0);row.icon:SetTexCoord(.08,.92,.08,.92)
        row.nameLabel=label(row,"",34,-5,242);row.nameLabel:SetMaxLines(1)
        row.currentRank=label(row,"",282,-5,172);row.currentRank:SetMaxLines(1);row.currentRank:SetTextColor(.58,.7,.78)
        row.minus=button(row,"−",458,-1,28,function()U:ShiftCrest(row,-1)end)
        row.plannedRank=label(row,"",492,-5,174);row.plannedRank:SetMaxLines(1)
        row.plus=button(row,"+",672,-1,28,function()U:ShiftCrest(row,1)end)
        row.maximum=label(row,"",706,-5,214);row.maximum:SetMaxLines(1)
        row:SetScript("OnEnter",function(self)if self.item then U:ItemTip(self,self.item)end end)
        row:SetScript("OnLeave",function()if GameTooltip then GameTooltip:Hide()end end);panel.rows[index]=row
    end
    panel:EnableMouseWheel(true);panel:SetScript("OnMouseWheel",function(_,delta)U.crestPage=math.max(1,(U.crestPage or 1)-(delta>0 and 1 or -1));U:Refresh()end)
    panel.empty=label(panel,"",8,-205,900);panel.empty:SetSpacing(7)
    panel.prev=button(panel,"Previous",0,-455,100,function()U.crestPage=math.max(1,U.crestPage-1);U:Refresh()end)
    panel.next=button(panel,"Next",110,-455,90,function()U.crestPage=U.crestPage+1;U:Refresh()end)
    panel.count=label(panel,"",214,-461,705)
end
function U:RefreshCrest(panel)
    local analysis=self:AnalysisJob("equipped",self.analysisTarget or 1);local job=self:Job("analysis")
    local result=analysis and analysis.result;local summary=job and self:SummaryFor(job)
    local current=self:Current()
    local changed=analysis and current and not sameCrestEquipment(current.equipped,summary and summary.equipped)
    local healer=result and result.metric=="healing_score"
    local modelDependent=healer and result.validation~="reviewed"
    setText(panel.resultContext,analysis and ((healer and(modelDependent and"QE model estimate"or"Healing preset")or analysisTargetLabel(job,self.analysisTarget or 1)).." · every value is vs equipped"..(healer and""or" · ≈ within sim margin").." · "..stamp(job.createdAt)) or "Run a full analysis to compare crest spending on equipped gear.")
    if changed then
        setText(panel.resultContext,"Gear changed since this analysis. Current upgrade ranks shown; run a new analysis in Setup.")
        panel.resultContext:SetTextColor(1,.75,.3)
        result=nil -- Every gain depends on the whole equipped baseline, not only this slot.
    else panel.resultContext:SetTextColor(.55,.78,.82)end
    panel.target:SetShown(analysis~=nil and not healer);panel.target:SetText(analysisTargetLabel(job,self.analysisTarget or 1))
    local metrics,allMax={},nil
    for _,comparison in ipairs(result and result.comparisons or {})do
        if comparison.variant=="all_max" then allMax=comparison
        else local id=comparison.candidateID or comparison.name;if id then
            metrics[id]=metrics[id]or{byLevel={}};metrics[id][comparison.variant or"current"]=comparison
            if comparison.level then metrics[id].byLevel[comparison.level]=comparison end
        end end
    end
    local totalDecision=decision(result,allMax)
	local totalEstimate=analysis and allMax and allMax.refined~=true
    setText(panel.totalValue,totalDecision and resultGainText(result,totalDecision,totalEstimate)or "Not tested")
	decisionColor(panel.totalValue,totalDecision,false,modelDependent)
    setText(panel.totalDetail,totalDecision and (signed(totalDecision.absolute).." "..throughputUnit(result).." vs today's full set · "..fmt(totalDecision.mean).." total · rows below change one item")or "The full analysis includes one maximum-rank set after the individual item comparisons.")
    if changed then setText(panel.totalDetail,"The saved result used different equipment. New gains need a completed analysis of this set.")end
	decisionBorder(panel.total,totalDecision,totalEstimate,modelDependent)
    local items={};for _,item in ipairs(current and current.equipped or summary and summary.equipped or {})do
		local path,currentRank,maxRank=crestPath(item)
		if #path>1 and maxRank>currentRank and(num(item.maxLevel)or 0)>(itemLevel(item)or 0)then items[#items+1]=item end
	end
    local ranked={}
    for _,item in ipairs(items)do
        local pair=metrics[item.id]or{byLevel={}};pair.byLevel=pair.byLevel or{}
        local path,currentRank,maxRank=crestPath(item);local nextMetric=pair.byLevel[path[2]]or pair.next;local maxMetric=pair.byLevel[path[#path]]or pair.max
        local nextValue=decision(result,nextMetric);local maxValue=decision(result,maxMetric)
        ranked[#ranked+1]={item=item,pair=pair,path=path,currentRank=currentRank,maxRank=maxRank,nextValue=nextValue,maxValue=maxValue,maxMetric=maxMetric,best=nextValue and nextValue.percent or -math.huge}
    end
    table.sort(ranked,function(a,b)if a.best~=b.best then return a.best>b.best end return (a.item.label or "")<(b.item.label or "")end)
    local pages=math.max(1,math.ceil(#ranked/10));self.crestPage=math.min(math.max(1,self.crestPage),pages)
    for index,row in ipairs(panel.rows)do
        local entry=ranked[(self.crestPage-1)*10+index];local item=entry and entry.item;row.item=item;row:SetShown(item~=nil)
        if item then
            local id=currentItemID(item);row.icon:SetTexture(item.icon or safe(C_Item and C_Item.GetItemIconByID or GetItemIcon,id)or 134400)
            setText(row.nameLabel,clean(item.label or tostring(id or "Item"),45).." · "..clean(slotNames[item.slot]or item.slot or "",16))
            local key=tostring(job and job.id or "").."\30"..identity(item);local selected=tonumber(self.crestRanks[key])or entry.currentRank
            selected=math.max(entry.currentRank,math.min(entry.maxRank,selected));self.crestRanks[key]=selected
            local pathIndex=selected-entry.currentRank+1;local selectedLevel=entry.path[pathIndex];local selectedMetric=entry.pair.byLevel[selectedLevel]
            local selectedValue=selected==entry.currentRank and{percent=0,absolute=0,mean=result and result.baseline and result.baseline.mean}or decision(result,selectedMetric)
            entry.key=key;row.crestEntry=entry
			setText(row.currentRank,crestRankText(result,item,entry.currentRank,entry.maxRank,entry.path[1],nil,entry.currentRank,nil,false))
            setText(row.plannedRank,crestRankText(result,item,selected,entry.maxRank,selectedLevel,selectedValue,entry.currentRank,selectedMetric,analysis~=nil))
            setText(row.maximum,crestRankText(result,item,entry.maxRank,entry.maxRank,entry.path[#entry.path],entry.maxValue,entry.currentRank,entry.maxMetric,analysis~=nil))
			if selected==entry.currentRank then row.plannedRank:SetTextColor(.55,.78,.82)else decisionColor(row.plannedRank,selectedValue,false,modelDependent)end;decisionColor(row.maximum,entry.maxValue,false,modelDependent)
            row.minus:SetEnabled(selected>entry.currentRank);row.plus:SetEnabled(selected<entry.maxRank)
			local value=selectedValue
			decisionBorder(row,value,analysis and selectedMetric and selectedMetric.refined~=true,modelDependent)
        end
    end
    panel.empty:SetShown(#ranked==0);setText(panel.empty,analysis and "No equipped upgrade paths completed. Update the character and run the analysis again." or "Run Full analysis from Setup. This page ranks the benefit of one more rank on each equipped item and shows each item's maximum result.")
    panel.prev:SetEnabled(self.crestPage>1);panel.next:SetEnabled(self.crestPage<pages)
    setText(panel.count,#ranked.." upgradeable equipped items · "..((changed or not analysis)and"current ranks; results not tested"or healer and"preset-scored ranks"or"simulated ranks; est. marks screening results").." · page "..self.crestPage.." / "..pages)
end
function U:Builds()
    local out,seen={},{}
    for _,group in ipairs({(self:Current() or {}).talents or {},settings().builds}) do
        for _,build in ipairs(group)do if type(build.code)=="string" and #build.code<=2048 and not seen[build.code]then
            seen[build.code]=true;out[#out+1]={label=clean(build.label~="" and build.label or "Saved build",60),code=build.code,changes=build.changes}
        end end
    end
    return out
end
function U:ChooseTalent(build)
	settings().loadoutTalent=build and build.code or nil;self.loadoutDirty=true;self.notice=nil;self:Refresh()
end
function U:BuildTalents(panel)
    panel.resultContext=label(panel,"",4,-3,550);panel.resultContext:SetTextColor(.55,.78,.82);panel.resultContext:SetMaxLines(1);panel.resultContext:SetWordWrap(false)
	panel.target=dropdown(panel,564,-3,178,analysisTargetChoices,function()return analysisTargetLabel(U:Job("analysis"),U.analysisTarget or 1)end,function(v)U.analysisTarget=chooseAnalysisTarget(v)end)
	resultUnits(panel)
	panel.help=label(panel,"Every result is vs equipped talents with the same gear. ≈ is within the sim margin; est. is a screening result. Click a row to use it in Loadout.",4,-28,926);panel.help:SetTextColor(.62,.74,.82)
	panel.current=CreateFrame("Button",nil,panel,"BackdropTemplate");panel.current:SetPoint("TOPLEFT",4,-55);panel.current:SetSize(926,43);panel.current:SetBackdrop(backdrop);panel.current:SetBackdropColor(.035,.065,.09,.95);panel.current:SetBackdropBorderColor(.12,.2,.25,.7)
	panel.currentName=label(panel.current,"Captured equipped talent build",12,-7,510,true);panel.currentGain=label(panel.current,"0.00% baseline",532,-8,170);panel.currentGain:SetJustifyH("RIGHT");panel.currentUse=label(panel.current,"",716,-8,194);panel.currentUse:SetJustifyH("RIGHT")
	panel.current:SetScript("OnClick",function()U:ChooseTalent(nil)end)
	label(panel,"Saved talent build",14,-111,485,true);label(panel,"vs equipped",548,-111,130,true);label(panel,"Use in Loadout",750,-111,166,true)
	panel.builds={}
	for i=1,6 do
		local row=CreateFrame("Button",nil,panel,"BackdropTemplate");row:SetPoint("TOPLEFT",4,-139-(i-1)*39);row:SetSize(926,36);row:SetBackdrop(backdrop);row:SetBackdropColor(.025,.045,.065,i%2==0 and .9 or .65);row:SetBackdropBorderColor(.12,.2,.25,.6)
		row.nameLabel=label(row,"",10,-9,505);row.nameLabel:SetMaxLines(1);row.gain=label(row,"",530,-9,145);row.gain:SetJustifyH("RIGHT");row.state=label(row,"",690,-9,224);row.state:SetJustifyH("RIGHT");row.state:SetMaxLines(1)
		row:SetScript("OnClick",function(self)if self.build then U:ChooseTalent(self.build)end end);panel.builds[i]=row
	end
	panel.prev=button(panel,"Previous",4,-377,96,function()U.talentPage=math.max(1,(U.talentPage or 1)-1);U:Refresh()end)
	panel.next=button(panel,"Next",106,-377,80,function()U.talentPage=(U.talentPage or 1)+1;U:Refresh()end);panel.count=label(panel,"",198,-383,350)
	label(panel,"Build name",4,-416,180);panel.name=edit(panel,8,-437,230,"",60)
	label(panel,"Talent import code",250,-416,525);panel.code=edit(panel,254,-437,520,"",2048)
	panel.add=button(panel,"Save build",790,-437,140,function()
        local name=panel.name:GetText():match("^%s*(.-)%s*$");local code=panel.code:GetText():match("^%s*(.-)%s*$")
        if name=="" or not code:match("^[A-Za-z0-9+/=]+$") then U.notice="Enter a build name and a valid talent import code.";U:Refresh();return end
        if #settings().builds>=12 then U.notice="Keep up to 12 imported builds.";U:Refresh();return end
        settings().builds[#settings().builds+1]={label=name,code=code};settings().loadoutTalent=code;U.loadoutDirty=true;panel.name:SetText("");panel.code:SetText("");U.notice=nil;U:Refresh()
    end)
end
function U:RefreshTalents(panel)
    local current=self:Current();local healer=current and current.snapshot.character and current.snapshot.character.role=="HEALER"
    local stats=current and current.snapshot.client=="forever"
	local supported=not healer and not stats
	setText(panel.help,supported and"Every result is vs equipped talents with the same gear. ≈ is within the sim margin; est. is a screening result. Click a row to use it in Loadout."or healer and"This healing model uses preset talents; custom talent comparisons are unavailable."or"Forever compares item stats only; talent comparisons are unavailable.")
	local analysis=self:AnalysisJob("talent",self.analysisTarget or 1);local job=analysis or self:Job("talents");panel.displayJob=job
	setText(panel.resultContext,job and((analysis and"Full analysis"or job.result and"Saved comparison"or"Request "..tostring(job.state)).." · compared with captured talents · "..stamp(job.createdAt))or"Run Full analysis to compare every saved build.")
	panel.target:SetShown(analysis~=nil and supported);panel.target:SetText(analysisTargetLabel(self:Job("analysis"),self.analysisTarget or 1))
	local metrics,metricsByLabel={},{};for _,comparison in ipairs(job and job.result and job.result.comparisons or{})do
		local index=tonumber(tostring(comparison.name or""):match("^talent(%d+)$"));local code=index and job.scenario and job.scenario.talents and job.scenario.talents[index]
		if code then metrics[code]=comparison end
		if comparison.label and comparison.label~=""then metricsByLabel[comparison.label]=comparison end
	end
	local builds=self:Builds();for _,build in ipairs(builds)do
		build.comparison=metrics[build.code]or metricsByLabel[build.label];build.decision=decision(job and job.result,build.comparison)
	end
	table.sort(builds,function(a,b)if(a.decision and a.decision.percent or-math.huge)~=(b.decision and b.decision.percent or-math.huge)then return(a.decision and a.decision.percent or-math.huge)>(b.decision and b.decision.percent or-math.huge)end return a.label<b.label end)
	local count=#builds;local selected=settings().loadoutTalent
	setText(panel.currentGain,(resultDisplayMode()=="absolute"and("0.0 "..throughputUnit(job and job.result))or"0.00%").." baseline")
	local capturedSelected=not selected or selected=="";panel.current:SetEnabled(supported);panel.current:SetBackdropBorderColor(capturedSelected and .1 or .12,capturedSelected and .85 or .2,capturedSelected and 1 or .25,capturedSelected and 1 or .7);setText(panel.currentUse,not supported and"Unavailable"or capturedSelected and"Selected in Loadout"or"Click to use")
    local pages=math.max(1,math.ceil(#builds/6));self.talentPage=math.min(self.talentPage or 1,pages)
    panel.prev:SetEnabled(self.talentPage>1);panel.next:SetEnabled(self.talentPage<pages)
	setText(panel.count,count.." saved build"..(count==1 and""or"s").." · page "..self.talentPage.." / "..pages)
	for i,row in ipairs(panel.builds)do local build=builds[(self.talentPage-1)*6+i];row.build=build;row:SetShown(build~=nil)
		if build then
			row:SetEnabled(supported)
			setText(row.nameLabel,build.label);local value=build.decision;local estimate=analysis and build.comparison and build.comparison.refined~=true
			setText(row.gain,value and resultGainText(job and job.result,value,estimate)or"Not tested");decisionColor(row.gain,value,estimate)
			local chosen=supported and selected==build.code;setText(row.state,not supported and"Unavailable"or chosen and"Selected in Loadout"or"Click to use");row.state:SetTextColor(chosen and .2 or .58,chosen and .9 or .7,chosen and 1 or .78)
			if chosen then row:SetBackdropBorderColor(.1,.85,1,1)else decisionBorder(row,value,estimate,false)end
		end
    end
	panel.add:SetEnabled(not healer and not stats)
end
local function numberSetting(parent,name,key,x,y,w,minimum,maximum,default,percent)
    local caption=label(parent,name,x,y,w)
    local value=tonumber(settings().scenario[key]) or default
    local field=edit(parent,x+4,y-21,w-8,tostring(percent and value*100 or value),8)
    local function save(self)
        local value=tonumber(self:GetText())
        if not value or value~=value or value<minimum or value>maximum or value%1~=0 then
            U.notice=name.." must be a whole number from "..minimum.." to "..maximum..".";U:RefreshStatus();return false
        end
        settings().scenario[key]=percent and value/100 or value;U.notice=nil;return true
    end
    field:SetScript("OnEnterPressed",function(self)if save(self)then self:ClearFocus()end end)
    field:SetScript("OnEditFocusLost",save);field.saveValue=save;field.caption=caption;return field
end
local function supportsAnalysis(current)
    local snapshot=current and current.snapshot;local character=snapshot and snapshot.character
    return snapshot and snapshot.client=="retail" and character and(character.role=="DAMAGER"or character.role=="HEALER")
end
function U:BuildAdvisor(panel)
    panel.heading=label(panel,"What should I equip?",4,-4,570,true)
    panel.source=label(panel,"",4,-31,700);panel.source:SetTextColor(.55,.78,.82);panel.source:SetMaxLines(1)
    panel.mode=LUI:Dropdown(panel,718,-2,214,function()
        local current=U:Current();local character=current and current.snapshot.character or{};local profile=advisorProfiles[character.spec]
        return profile and profile.tank and{"Balanced","Survival","Damage"}or{"Mythic+","Raid"}
    end,function()
        local current=U:Current();local character=current and current.snapshot.character or{};local profile=advisorProfiles[character.spec]
        return advisorModeName(profile,advisorMode(profile,character.spec))
    end,function(value)
        local current=U:Current();local character=current and current.snapshot.character or{}
        settings().advisorModes=settings().advisorModes or{}
        settings().advisorModes[tostring(character.spec or"")]=({Balanced="balanced",Survival="survival",Damage="damage",Raid="raid",["Mythic+"]="dungeon"})[value]
        U:Refresh()
    end)
    local left=card(panel,4,-57,570,102);local right=card(panel,586,-57,346,102)
    panel.answerTitle=label(left,"",14,-12,542,true);panel.answer=label(left,"",14,-42,542);panel.answer:SetMaxLines(3)
    panel.mixTitle=label(right,"Your secondary mix",14,-12,318,true);panel.mix=label(right,"",14,-42,318);panel.mix:SetMaxLines(3)
    label(panel,"Best bag changes and Great Vault choices",4,-173,420,true)
    panel.crest=label(panel,"",438,-177,494);panel.crest:SetJustifyH("RIGHT");panel.crest:SetTextColor(.55,.78,.82)
    panel.rows={}
    for index=1,6 do
        local row=CreateFrame("Button",nil,panel,"BackdropTemplate");row:SetPoint("TOPLEFT",4,-205-(index-1)*42);row:SetSize(928,39)
        row:SetBackdrop(backdrop);row:SetBackdropColor(.035,.065,.09,index%2==0 and.82 or.45);row:SetBackdropBorderColor(.12,.22,.28,.7)
        row.icon=row:CreateTexture(nil,"ARTWORK");row.icon:SetSize(32,32);row.icon:SetPoint("LEFT",4,0)
        row.nameLabel=label(row,"",43,-4,430);row.nameLabel:SetMaxLines(1)
        row.detail=label(row,"",43,-21,600);row.detail:SetFontObject("GameFontHighlightSmall");row.detail:SetTextColor(.68,.76,.82);row.detail:SetMaxLines(1)
        row.value=label(row,"",650,-5,264);row.value:SetJustifyH("RIGHT");row.value:SetMaxLines(1)
        row.confidence=label(row,"",650,-22,264);row.confidence:SetFontObject("GameFontHighlightSmall");row.confidence:SetJustifyH("RIGHT");row.confidence:SetMaxLines(1)
        row:SetScript("OnClick",function(self)
            if not self.choice then return end
            U.search=self.choice.item.label or"";U.slot=slotNames[self.choice.item.slot]or self.choice.item.slot;U.page=1
            U:SelectTab(self.choice.item.source=="vault"and"vault"or"gear")
        end)
        row:SetScript("OnEnter",function(self)if self.choice then U:ItemTip(self,self.choice.item)end end)
        row:SetScript("OnLeave",function()if GameTooltip then GameTooltip:Hide()end end)
        panel.rows[index]=row
    end
    panel.empty=label(panel,"",18,-221,895);panel.empty:SetMaxLines(5)
    panel.notes=button(panel,"How this decides",4,-455,174,function()
        local current=U:Current();local data,problem=advisorData(current)
        if not data then U:ShowDetail("Gear advisor",{problem or"No model is available."});return end
        local weights={};local primaryWeight=data.weights[data.profile.primary]or 1
        weights[#weights+1]=advisorStatNames[data.profile.primary].." 100%"
        for _,name in ipairs(data.order or{})do weights[#weights+1]=advisorStatNames[name].." "..string.format("%.0f%%",(data.weights[name]or 0)/primaryWeight*100)end
        local lines={
            (data.meta and"Current top-player direction: "or"Maintained guide direction: ")..advisorStatNames[data.profile.primary].." > "..table.concat((function()local out={};for _,name in ipairs(data.order or{})do out[#out+1]=advisorStatNames[name]end;return out end)()," > "),
            "Relative item values per rating: "..table.concat(weights," · ")..". These percentages are comparison weights, not character-sheet caps.",
            "Your secondary-stat budget: "..data.mix..".",
            "The advisor replaces one item at a time and compares the stats printed by WoW. Higher item level wins naturally through more primary stats, armor, Stamina, and weapon damage; it is not awarded points merely for its item-level number.",
            "Green rows are ordinary stats-only comparisons. Amber rows involve a trinket, Equip/Use effect, or set bonus. Their visible stats are included, but the effect is not assigned a made-up value.",
            "Maximum-rank values are estimates from the captured upgrade path. Use the Crest and Loadout pages for the existing detailed rank tools.",
            "Sockets, enchants and gems count only when WoW includes their stats in the captured hyperlink. Talent interactions, mana use, encounter mechanics, tier bonuses, embellishments and proc timing can change the answer.",
            "For another specialization, switch specs and refresh character data in Setup. This prevents dynamic primary stats, eligibility, tier and weapon rules from leaking across specs.",
        }
        if data.meta then lines[#lines+1]="Source: Murlok.io "..clean(data.meta.specName or data.profile.name,80).." Mythic+ top-player sample · updated "..stamp(data.meta.updatedAt)..". The priority is refreshed through Builds and is not a personalized stat-weight calculation."
        else lines[#lines+1]="Source: maintained Patch 12.1 guide fallback. Refresh M+ meta to use the current top-player stat order for this specialization." end
        if data.profile.target then lines[#lines+1]=data.profile.target end
        if data.profile.note then lines[#lines+1]=data.profile.note end
        U:ShowDetail(data.profile.name.." · "..advisorModeName(data.profile,data.mode),lines)
    end)
    panel.loadout=button(panel,"Open character sheet",190,-455,198,function()U:SelectTab("loadout")end)
    panel.refreshMeta=button(panel,"Refresh M+ meta",400,-455,162,function()
        local current=U:Current();local character=current and current.snapshot.character or{}
        U:Send("builds",{returnTab="advisor",compareMode="individual",scenario={targets=1,seconds=300,iterations=100,buildMode="m+",buildSpec=character.spec}})
    end,"Refreshes the current specialization's top-player Mythic+ stat priority. The result returns to Quick Advisor.")
    panel.specNote=label(panel,"Switch specs, then refresh character data in Setup for off-spec advice.",574,-461,358);panel.specNote:SetJustifyH("RIGHT")
end
function U:RefreshAdvisor(panel)
    local current=self:Current();local character=current and current.snapshot.character or{};local profile=advisorProfiles[character.spec]
    local data,problem=advisorData(current)
    panel.mode:SetText(advisorModeName(profile,profile and advisorMode(profile,character.spec)));panel.mode:SetEnabled(profile~=nil)
    local mode=data and data.mode or profile and advisorMode(profile,character.spec)
    local source=data and data.meta and("Community trend, not your stat weights · updated "..stamp(data.meta.updatedAt)..(data.meta.stale and" · refresh recommended"or""))or profile and("Guide fallback, not your stat weights · "..advisorModeName(profile,mode))or"No maintained model for this specialization"
    setText(panel.source,source)
    panel.empty:SetShown(not data or #data.rows==0)
    if not data then
        setText(panel.answerTitle,"Gear data needed");setText(panel.answer,problem or"Refresh character data in Setup to begin.");setText(panel.mix,"—");setText(panel.crest,"")
    else
        local clear,bestClear,bestClose,bestFuture,bestSpecial=0
        for _,row in ipairs(data.rows)do
            if not row.special and row.clear then clear=clear+1;bestClear=bestClear or row
            elseif not row.special and row.close then bestClose=bestClose or row
            elseif not row.special and row.future then bestFuture=bestFuture or row
            else bestSpecial=bestSpecial or row end
        end
        local best=bestClear or bestClose or bestFuture or bestSpecial
        if bestClear then
            setText(panel.answerTitle,clear.." likely visible-stat upgrade"..(clear==1 and""or"s").." available")
            local source=best.item.source=="vault"and"Great Vault choice"or"Bag item"
            setText(panel.answer,source..": "..clean(best.item.label,48).." over "..clean(best.replaced.label,44).." · "..gain(best.percent).." gear fit")
        elseif bestClose then
            setText(panel.answerTitle,"No meaningful current-rank upgrade")
            setText(panel.answer,(best.item.source=="vault"and"Closest Vault trade: "or"Closest bag trade: ")..clean(best.item.label,48).." over "..clean(best.replaced.label,42).." · "..gain(best.percent).." gear fit. Treat it as equal.")
        elseif bestFuture then
            setText(panel.answerTitle,"No upgrade to equip at its current rank")
            setText(panel.answer,clean(best.item.label,52).." becomes a possible "..gain(best.maxPercent).." stats win only after both items are compared at maximum rank.")
        elseif bestSpecial then
            setText(panel.answerTitle,"No safe stats-only recommendation")
            setText(panel.answer,"Best remaining option needs its effect or set bonus reviewed: "..clean(best.item.label,54)..".")
        else
            setText(panel.answerTitle,"Your bags do not improve this stat model")
            setText(panel.answer,"Keep the equipped set unless a special effect, tier bonus, or encounter need changes the choice.")
        end
        setText(panel.mix,data.mix..(data.profile.target and("\n"..data.profile.target)or""))
        if data.crest then
            local rank=plannedRank(data.crest.item,data.crest.nextLevel);local rankText=rank and data.crest.item.maxRank and((data.crest.item.track and(clean(data.crest.item.track,14).." ")or"Rank ")..rank.."/"..data.crest.item.maxRank.." · ")or""
            setText(panel.crest,"Best next crest: "..clean(data.crest.item.label,35).." · "..rankText.."ilvl "..data.crest.nextLevel.." · "..gain(data.crest.percent).." est.")
        else setText(panel.crest,"No equipped next-rank path captured")end
        setText(panel.empty,#data.rows==0 and"No positive individual swap was found from the readable bag or Great Vault stats. Open the Vault and refresh character data in Setup if choices are missing. Trinkets and other special effects may still be better; the advisor refuses to guess their value."or"")
    end
    for index,row in ipairs(panel.rows)do local choice=data and data.rows[index];row.choice=choice;row:SetShown(choice~=nil)
        if choice then
            local id=currentItemID(choice.item);row.icon:SetTexture(safe(C_Item and C_Item.GetItemIconByID or GetItemIcon,id)or 134400)
            local rank=itemRank(choice.item)
            local source=choice.item.source=="vault"and"VAULT"or"BAG"
            setText(row.nameLabel,source.."  ·  "..clean(choice.item.label or"Item",48)..(rank and("  ·  "..rank)or"")..(choice.level>0 and("  ·  ilvl "..choice.level)or""))
            setText(row.detail,(choice.item.source=="vault"and"Would replace "or"Replaces ")..clean(choice.replaced.label,54).." · "..choice.reason)
            local value=choice.delta>0 and(gain(choice.percent).." now")or"No gain now"
            local rankDetail=""
            if choice.maxLevel>choice.level and choice.maxDelta>0 then
                local maxRank=choice.item.track and choice.item.maxRank and(clean(choice.item.track,16).." "..choice.item.maxRank.."/"..choice.item.maxRank.." · ")or""
                value=value.." · "..gain(choice.maxPercent).." max-vs-max"
                rankDetail=maxRank.."ilvl "..choice.maxLevel.." estimate"
            end
            setText(row.value,value)
            local confidence=choice.special and(choice.setBonus and"Review set/effect"or"Review item effect")
                or choice.clear and"Likely stats upgrade"or choice.close and"Very close stat trade"or"Only better at max rank"
            setText(row.confidence,confidence..(rankDetail~=""and(" · "..rankDetail)or""))
            if choice.special then row.value:SetTextColor(1,.82,.26);row.confidence:SetTextColor(1,.72,.25);row:SetBackdropBorderColor(.65,.48,.12,.9)
            elseif choice.clear then row.value:SetTextColor(.3,1,.5);row.confidence:SetTextColor(.45,.86,.64);row:SetBackdropBorderColor(.12,.6,.38,.9)
            elseif choice.close then row.value:SetTextColor(1,.82,.26);row.confidence:SetTextColor(1,.82,.26);row:SetBackdropBorderColor(.55,.43,.16,.9)
            else row.value:SetTextColor(.58,.78,.92);row.confidence:SetTextColor(.58,.78,.92);row:SetBackdropBorderColor(.18,.4,.58,.9)end
        end
    end
    local buildsAvailable=self:EncounterCapability("builds");panel.refreshMeta:SetEnabled(profile~=nil and(mode~="raid"or profile.tank)and buildsAvailable and not transferBusy())
    panel.notes:SetEnabled(profile~=nil);panel.loadout:SetEnabled(current~=nil)
end
function U:BuildOverview(panel)
    local left=card(panel,0,0,412,420);local right=card(panel,430,0,502,420)
    label(left,"Test conditions",14,-14,382,true)
    panel.modelNote=label(left,"",14,-47,382)
    panel.damage=CreateFrame("Frame",nil,left);panel.damage:SetPoint("TOPLEFT",14,-92);panel.damage:SetSize(384,155)
    panel.fields={}
    panel.fields.targets=numberSetting(panel.damage,"Targets","targets",0,0,115,1,8,1)
    panel.fixedTargets=label(panel.damage,"Views",0,0,115);panel.fixedTargetValue=label(panel.damage,"Raid boss + Dungeon",4,-22,238,true)
    panel.fields.seconds=numberSetting(panel.damage,"Duration (s)","seconds",128,0,115,30,600,300)
    panel.fields.iterations=numberSetting(panel.damage,"Iterations","iterations",256,0,115,100,20000,1000)
    panel.raidBuffLabel=label(panel.damage,"Boss buffs",256,0,115)
    panel.raidBuffs=dropdown(panel.damage,252,-22,132,{"Self only","Full raid"},function()return U:Scenario().raidBuffs=="full"and"Full raid"or"Self only"end,function(v)settings().scenario.raidBuffs=v=="Full raid"and"full"or"none"end)
    LUI:Explain(panel.raidBuffs,"Boss buffs","Self only matches the no-buffs Raidbots preset. Dungeon Slice always uses self buffs because SimulationCraft disables optimal raid buffs for dungeon-style simulations.")
    panel.aug=CreateFrame("Frame",nil,left);panel.aug:SetPoint("TOPLEFT",14,-178);panel.aug:SetSize(384,170)
    label(panel.aug,"Group",0,0,380)
    panel.group=dropdown(panel.aug,0,-22,370,{"Dungeon","Raid"},function()return U:Scenario().augmentationMode=="raid" and "Raid" or "Dungeon"end,function(v)settings().scenario.augmentationMode=v:lower()end)
    panel.fields.allyItemLevel=numberSetting(panel.aug,"Ally item level","allyItemLevel",0,-67,180,1,1000,328)
    panel.fields.allySkill=numberSetting(panel.aug,"Ally performance (%)","allySkill",194,-67,180,1,100,.9,true)
    LUI:Explain(panel.group,"Simulated group","Fixed allies, not your current party. Keep this model the same when comparing gear and talents. Dungeon is a party model, not a timed Mythic+ route.")
    panel.healer=CreateFrame("Frame",nil,left);panel.healer:SetPoint("TOPLEFT",14,-102);panel.healer:SetSize(384,88)
    label(panel.healer,"Healing preset",0,0,370)
    panel.healingMode=dropdown(panel.healer,0,-25,370,{"Dungeon","Raid"},function()return U:Scenario().healingMode=="raid" and "Raid" or "Dungeon"end,function(v)settings().scenario.healingMode=v:lower()end)
    panel.customize=button(left,"Customize analysis",14,-319,370,function()U:OpenWorkflow()end)
    panel.run=button(left,"Run full analysis",14,-365,370,function()
        local task=panel.runTask or "baseline";local analysis=task=="analysis"
        for key,field in pairs(panel.fields)do if (not analysis or key~="targets") and (key~="allySkill" and key~="allyItemLevel" or panel.aug:IsShown()) and panel.damage:IsShown() and not field.saveValue(field)then return end end
        if transferBusy() then U.notice="A simulation is already in progress.";U:Refresh();return end
        local token={};U.startToken=token;U.notice=nil;U.status="Reading current character…"
        panel.run:SetText(analysis and "Starting analysis…" or "Starting…");panel.run:SetEnabled(false)
        setText(panel.readiness,"Reading current character…");U:RefreshStatus()
        C_Timer.After(0,function()
            if U.startToken~=token then return end
            U.startToken=nil;U:Send(task,nil,analysis)
        end)
    end)
    label(right,"Included in this run",14,-14,472,true)
    panel.scope=label(right,"",14,-53,470);panel.scope:SetSpacing(10);panel.scope:SetMaxLines(12)
    panel.readiness=label(right,"",14,-284,470);panel.readiness:SetTextColor(.55,.78,.82);panel.readiness:SetMaxLines(4)
    panel.refresh=button(right,"Refresh character data",14,-365,210,function()U.notice=nil;U:Current(true);U:Refresh()end,
        "Refreshes the Setup counts without starting a simulation. Full analysis always captures fresh gear, bags, talents and open Vault choices before it runs.")
    panel.refreshNote=label(right,"Optional preview refresh. Running the analysis refreshes this data automatically.",238,-361,244);panel.refreshNote:SetTextColor(.58,.7,.78);panel.refreshNote:SetMaxLines(3)
    label(panel,"Run once. LSim refreshes your character, performs the analysis, and opens Results when it finishes.",4,-442,916)
end
local function scenarioText(job)
    local scenario=job and job.scenario or {};local result=job and job.result or {};local parts={}
    if result.healing then local r=result.healing;return (r.replay and r.replay.encounter or r.options.scenario).." · "..tostring(r.replay and r.replay.seconds or r.options.seconds).."s · "..r.groupSize.." allies · "..r.options.runs.." runs · "..r.options.intensity.."% incoming damage" end
    if result.builds and(result.metric=="build_reference"or taskOf(job)=="builds")then return "Build reference · "..clean(result.builds.mode).." · sample "..tostring(result.builds.sample or 0).." · updated "..stamp(result.builds.updatedAt) end
    if result.metric=="group_dps" then
        parts[#parts+1]=(scenario.augmentationMode=="raid" and "Raid" or "Dungeon").." · allies ilvl "..tostring(scenario.allyItemLevel and scenario.allyItemLevel>0 and scenario.allyItemLevel or 328).." · "..tostring(math.floor((scenario.allySkill and scenario.allySkill>0 and scenario.allySkill or .9)*100+.5)).."% performance"
    elseif result.metric=="healing_score" then parts[#parts+1]=(scenario.healingMode=="dungeon" and "Dungeon" or "Raid").." healing preset" end
    if taskOf(job)=="analysis" and result.metric~="healing_score" then
        local dungeon=targetResult(result,5)
        if dungeon and dungeon.fightStyle=="dungeon_slice"then parts[#parts+1]="Raid boss · 5 min\nDungeon Slice · 6 min"
        else
            local seconds=tonumber(scenario.seconds)or 0;local duration=seconds>0 and seconds%60==0 and tostring(seconds/60).." min" or tostring(seconds).."s"
            parts[#parts+1]="1 and 5 stationary targets · "..duration
        end
    elseif result.metric~="healing_score" and result.metric~="item_stats" and scenario.targets then
        local seconds=tonumber(scenario.seconds)or 0;local duration=seconds>0 and seconds%60==0 and tostring(seconds/60).." min" or tostring(seconds).."s"
        if scenario.fightStyle=="dungeon_slice"then parts[#parts+1]="Dungeon Slice · 6 min · self buffs · "..fmt(scenario.iterations).." iterations"
        else
            local targets=tostring(scenario.targets).." stationary target"..(scenario.targets==1 and "" or "s")
            parts[#parts+1]=targets.." · "..duration.." · "..fmt(scenario.iterations).." iterations · "..(scenario.raidBuffs=="none"and"self buffs"or"full raid buffs")
        end
    end
    return table.concat(parts,"\n")
end
validationText=function(value)
    if value=="reviewed" then return "Model checked for this build and spec" end
    if value=="review-required" or value=="unreviewed" then return "Model not yet checked for this build and spec" end
    return clean(value or "Model review status unavailable")
end
local function overviewSummary(job)
    local result=job and job.result;if not result then return end
    if taskOf(job)=="analysis" and result.targetResults then
        local lines={}
        local healer=result.metric=="healing_score"
        for _,targets in ipairs(healer and{1}or{1,5})do
            lines[#lines+1]=healer and"HEALING PRESET"or analysisTargetLabel(job,targets):upper()
            for _,source in ipairs({"owned","vault"})do
                local view=analysisView(job,targets,source);local rows=resultRows(view,U:SummaryFor(job));local best=rows[1]
                local prefix=source=="owned" and "Bag" or "Vault"
                if not best then lines[#lines+1]=prefix.."  No captured items"
                elseif not best.decision or best.decision.absolute<=0 or best.decision.uncertain then lines[#lines+1]=prefix.."  Keep equipped · best: "..clean(best.name,45).." ("..gain(best.decision and best.decision.percent)..")"
                else lines[#lines+1]=prefix.."  "..clean(best.name,45).." · "..signed(best.decision.absolute).." "..throughputUnit(view.result).." ("..gain(best.decision.percent)..")" end
            end
        end
        return "Bag + Vault analysis",table.concat(lines,"\n")
    end
    if result.metric=="item_stats" then
        return "Stats captured — no winner calculated","This client can show exact stat changes, but it cannot turn them into a damage or healing recommendation. Compare the listed stats with your class priorities."
    end
    local rows=resultRows(job,U:SummaryFor(job))
    if #rows>0 then
        local best=rows[1];local value=best.decision;local task=taskOf(job)
        local title=value and value.uncertain and "No clear winner" or value and value.absolute<=0 and "Keep your current setup"
            or task=="talents" and "Best tested talent build" or task=="compare" and "Best tested gear change" or "Best tested result"
        local guidance=value and value.uncertain and "The difference is within simulation uncertainty; choose for utility or preference."
            or value and value.absolute<=0 and "Every tested option was equal or worse in this run."
            or "Use this change if the shown test conditions match what you care about."
        return title,clean(best.name,72).."\n"..best.detail.."\n\n"..guidance
    end
    local upgrades=result.upgrades or {}
    if #upgrades>0 then
        local best=upgrades[1]
        for _,item in ipairs(upgrades)do if num(item.gain)and (not num(best.gain)or item.gain>best.gain)then best=item end end
        local baseline=result.baseline and num(result.baseline.mean);local absolute=baseline and num(best.mean)and best.mean-baseline
        local amount=absolute and signed(absolute).." "..throughputUnit(result).." ("..gain(best.gain)..")" or gain(best.gain)
        local title=(best.gain or 0)>0 and "Best upgrade found" or "No upgrade found in captured loot"
        return title,clean(best.label or "Item",72).."\n"..amount.."\n\nOpen Upgrades for its source, item level, and other ranked options."
    end
    local baseline=result.baseline or {};local mean=num(baseline.mean)
    local body=fmt(mean).." "..metric(result).."\n"
    if result.metric=="group_dps" then
        body=body.."This includes fixed simulated allies. Judge gear and talents by the change in Group DPS, never by this total alone."
    else
        body=body.."This is a controlled simulation average, not a target for your damage meter. It becomes useful only when another setup is tested under the same conditions."
    end
    local error=num(baseline.meanError)
    if error and error>0 and mean and mean~=0 then body=body.."\n\nBaseline uncertainty: ±"..fmt(error).." ("..string.format("%.2f%%",math.abs(error/mean*100))..")." end
    return "Baseline ready — compare a change",body
end
function U:RefreshOverview(panel)
    local combat=InCombatLockdown and InCombatLockdown()
    local current=self:Current();local character=current and current.snapshot.character or {}
    local stats=current and current.snapshot.client=="forever";local healer=character.role=="HEALER";local aug=character.spec==1473
    local analysis=supportsAnalysis(current)
    panel.customize:SetShown(analysis and not healer and not stats);panel.customize:SetEnabled(not transferBusy()and not combat)
    panel.damage:SetShown(not stats and not healer);panel.aug:SetShown(aug and not stats);panel.healer:SetShown(healer and not stats)
    panel.fields.targets:SetShown(not analysis);panel.fields.targets.caption:SetShown(not analysis);panel.fixedTargets:SetShown(analysis and not healer);panel.fixedTargetValue:SetShown(analysis and not healer)
    panel.fields.seconds:SetShown(not analysis or aug);panel.fields.seconds.caption:SetShown(not analysis or aug)
    panel.fields.iterations:SetShown(not analysis);panel.fields.iterations.caption:SetShown(not analysis)
    panel.raidBuffLabel:SetShown(analysis and not healer and not aug);panel.raidBuffs:SetShown(analysis and not healer and not aug)
    setText(panel.fixedTargetValue,aug and"1 and 5 targets"or"Raid boss + Dungeon")
    panel.raidBuffs:SetText(self:Scenario().raidBuffs=="full"and"Full raid"or"Self only")
    setText(panel.modelNote,stats and "Captured equipment stats." or healer and "Questionably Epic healing gear score · preset talents and encounter model." or aug and "SimulationCraft Group DPS · fixed party model." or "SimulationCraft direct gear and talent comparisons.")
    panel.group:SetText(self:Scenario().augmentationMode=="raid" and "Raid" or "Dungeon");panel.healingMode:SetText(self:Scenario().healingMode=="raid" and "Raid" or "Dungeon")
    panel.runTask=analysis and "analysis" or "baseline"
    panel.run:SetText(self.startToken and (analysis and "Starting analysis…" or "Starting…") or healer and analysis and"Analyze healing gear"or analysis and "Run full analysis" or stats and "Capture current item stats" or healer and "Score current healing gear" or "Sim current setup")
    local available,reason=self:Capability();panel.run:SetEnabled(not transferBusy() and not combat);panel.refresh:SetEnabled(not transferBusy() and not combat)
    local currentView=self:Current() or {};local candidates=currentView.candidates or {};local bag,vault=0,0
    for _,item in ipairs(candidates)do if item.source=="owned"then bag=bag+1 elseif item.source=="vault"then vault=vault+1 end end
    local talents=#self:Builds();local upgradeable=0
    for _,item in ipairs(candidates)do if (item.maxLevel or 0)>(item.level or itemLevel(item)or 0)then upgradeable=upgradeable+1 end end
    local equipped=#(currentView.equipped or {})
    local damageViews=aug and"Every option is checked at 1 and 5 stationary targets."or("Every option is checked for a 5-minute raid boss and a 6-minute Dungeon Slice. Boss: "..(self:Scenario().raidBuffs=="full"and"full raid buffs."or"self buffs only."))
    local workflow=analysis and not healer and self:WorkflowSettings()
    if workflow and workflow.enabled then
        local names={};for _,e in ipairs(workflow.encounters or{})do names[#names+1]=clean(e.name).." · "..e.targets.." targets · "..e.seconds.."s · Bloodlust "..(e.bloodlust or"off")end
        damageViews=table.concat(names,"\n").."\n"..(#(workflow.selected or{})>0 and(#workflow.selected.." selected items compared as legal sets.")or"Broad gear screening; no item selections.")
        setText(panel.fixedTargetValue,"Custom encounters");panel.customize:SetText("Customize analysis · "..#(workflow.selected or{}).." selected")
    else panel.customize:SetText("Customize analysis")end
    setText(panel.scope,analysis and ("Equipped crest paths  "..equipped.." items\nBag gear  "..bag.." items\nVault  "..vault.." choices\n"..(healer and"Talents  Provider preset (not changeable)"or"Saved talents  "..talents.." builds").."\nBag upgrade potential  "..upgradeable.." items\n\n"..(healer and"Each compatible gear change is scored with the selected healing preset."or damageViews)) or "This calculation uses the conditions on the left.")
    setText(panel.readiness,combat and "Finish combat before refreshing your character or starting an analysis." or self.startToken and "Reading current character…" or self.notice or (available and (analysis and "Ready to analyze the captured character." or "Ready to run.") or reason))
end
function U:BuildUpgrades(panel)
    panel.headline=label(panel,"",4,-4,735);panel.headline:SetMaxLines(1);panel.context=label(panel,"",4,-34,928)
    resultUnits(panel)
    panel.run=button(panel,"Find upgrades",4,-80,150,function()
        local key=tonumber(panel.lootKey:GetText())
        if not key or key%1~=0 or key<0 or key>30 or key==1 then U.notice="Choose dungeon level 0 or 2–30.";U:Refresh();return end
        S.lootKey=key;settings().lootKey=key;U:Send("upgrades")
    end,"Scans eligible journal equipment, then asks Lamda Engine to compare it. Results open when they are ready.")
    panel.notes=button(panel,"Model notes",162,-80,118,function()U:ShowNotes(U:Job("upgrades"))end)
    panel.rank=dropdown(panel,288,-80,155,{"Item gain","Farm value","Reroll value"},function()return U.rank or "Item gain"end,function(v)U.rank=v;U.upgradePage=1 end)
    panel.view=dropdown(panel,451,-80,145,{"Drop now","Max upgraded","Bonus reward","Crafted"},function()return U.upgradeView or "Drop now"end,function(v)U.upgradeView=v;U.upgradePage=1 end)
    panel.filter=dropdown(panel,604,-80,145,{"All","Upgrades","BiS"},function()return U.upgradeFilter or "All"end,function(v)U.upgradeFilter=v;U.upgradePage=1 end)
    panel.plan=button(panel,"Calculate plan",757,-80,173,function()U:RequestPlan()end)
    panel.loot=CreateFrame("Frame",nil,panel);panel.loot:SetPoint("TOPLEFT",4,-121);panel.loot:SetSize(924,70)
    label(panel.loot,"Dungeon key (0 = Mythic)",0,0,240);panel.lootKey=edit(panel.loot,4,-23,90,tostring(settings().lootKey or S.lootKey or 10),2)
    label(panel.loot,"Raid difficulty",278,0,260)
    panel.lootRaid=dropdown(panel.loot,278,-23,210,{"Raid Finder","Normal","Heroic","Mythic"},function()return raidNames[S.lootRaid or settings().lootRaid or 15]or "Heroic"end,function(v)for id,name in pairs(raidNames)do if name==v then S.lootRaid=id;settings().lootRaid=id;U.lootDifficultyChanged=true;U.upgradePage=1 end end end)
    S.lootRaid=settings().lootRaid or S.lootRaid or 15
    label(panel.loot,"Only captured, eligible equipment is ranked.",510,-28,410)
    panel.assumptions=CreateFrame("Frame",nil,panel);panel.assumptions:SetPoint("TOPLEFT",4,-121);panel.assumptions:SetSize(924,72)
    label(panel.assumptions,"Gear reward chance (%)",0,0,220);panel.chance=edit(panel.assumptions,4,-22,112,"40",5)
    panel.cost=edit(panel.assumptions,146,-22,112,"35",6)
    panel.costLabel=panel.assumptions:CreateFontString(nil,"OVERLAY","GameFontHighlight");panel.costLabel:SetPoint("TOPLEFT",142,0);panel.costLabel:SetText("")
    panel.assumption=button(panel.assumptions,"Reroll pool: not confirmed",280,-22,290,function()U.bonusPool=not U.bonusPool;U:Refresh()end,"An estimate only: assumes bonus rewards use the captured journal's equipment pool, with equal item odds.")
    panel.assumptionNote=label(panel.assumptions,"Equal item odds. Equipment rewards only.",588,-24,330)
    panel.rows={}
    for i=1,6 do
        local row=CreateFrame("Button",nil,panel,"BackdropTemplate");row:SetPoint("TOPLEFT",4,-205-(i-1)*39);row:SetSize(924,37)
        row:SetBackdrop(backdrop);row:SetBackdropColor(.035,.065,.09,i%2==0 and .8 or .25);row:SetBackdropBorderColor(.1,.18,.23,.35)
        row.icon=row:CreateTexture(nil,"ARTWORK");row.icon:SetSize(32,32);row.icon:SetPoint("LEFT",3,0);row.icon:SetTexCoord(.08,.92,.08,.92)
        row.nameLabel=label(row,"",42,-3,350);row.nameLabel:SetMaxLines(1)
        row.farm=label(row,"",400,-3,302);row.farm:SetMaxLines(1);row.farm:SetFontObject("GameFontHighlightSmall")
        row.source=label(row,"",42,-21,660);row.source:SetMaxLines(1);row.source:SetFontObject("GameFontHighlightSmall")
        row.value=label(row,"",710,-4,204);row.value:SetJustifyH("RIGHT")
        row:SetScript("OnEnter",function(self)if self.item then U:ItemTip(self,self.item)end end)
        row:SetScript("OnLeave",function()U:HideItemTip()end);panel.rows[i]=row
    end
    panel.empty=label(panel,"",4,-208,918)
    panel.prev=button(panel,"Previous",4,-455,100,function()U.upgradePage=math.max(1,U.upgradePage-1);U:Refresh()end)
    panel.next=button(panel,"Next",112,-455,86,function()U.upgradePage=U.upgradePage+1;U:Refresh()end)
    panel.count=label(panel,"",214,-461,520)
    panel.target=button(panel,"Best tested set",738,-455,192,function()U:ShowTarget()end)
end
function U:PlanFor(upgrade)
    if not upgrade then return end
    for _,job in ipairs(self:Jobs())do if taskOf(job)=="plan" then
        local summary=self:SummaryFor(job);local request=summary and summary.snapshot.gameRequest
        local sourceID=job.plan and job.plan.jobID or request and request.jobID
        if sourceID==upgrade.id then return job end
    end end
end
function U:RequestPlan()
    local job=self:Job("upgrades");if not job or not job.result then self.notice="Run Find upgrades first.";self:Refresh();return end
    local panel=self.panels.upgrades;local chance=tonumber(panel.chance:GetText());local cost=tonumber(panel.cost:GetText())
    if not chance or chance<0 or chance>100 or not cost or cost<=0 or cost>1000 then self.notice="Enter a reward chance from 0–100 and a positive cost.";self:Refresh();return end
    local reroll=self.rank=="Reroll value"
    if reroll and not self.bonusPool then self.notice="Confirm the journal-pool assumption before estimating rerolls.";self:Refresh();return end
    local views={["Drop now"]="drop",["Max upgraded"]="max",Upgraded="max",["Bonus reward"]="bonus",Crafted="crafted"}
    self:Send("plan",{jobID=job.id,planOptions={mode=reroll and "reroll" or "farm",view=reroll and "bonus" or views[self.upgradeView or "Drop now"],chance=chance/100,cost=cost,exclude={},currency=0,assumeBonusPool=reroll and self.bonusPool or false,historyConfirmed=false,weights={}}})
end
local function upgradeText(item,result)
    local baseline=result and result.baseline and num(result.baseline.mean);local mean=item and num(item.mean)
    if baseline and mean then return resultGainText(result,{absolute=mean-baseline,percent=num(item.gain),uncertain=false}) end
    return gain(item and item.gain)
end
function U:RefreshUpgrades(panel)
    local current=self:Current();local job=self:Job("upgrades");local result=job and job.result
	local selectedRaid=S.lootRaid or settings().lootRaid or 15;local selectedRaidName=raidNames[selectedRaid]or"Heroic"
	local savedRaid=job and job.scenario and job.scenario.raidDifficulty
	local knownMismatch=savedRaid~=nil and tonumber(savedRaid)~=raidTiers[selectedRaid]
	if savedRaid~=nil and not knownMismatch then self.lootDifficultyChanged=nil end
	local difficultyChanged=result and(knownMismatch or self.lootDifficultyChanged==true)
	if difficultyChanged then result=nil end
	if self.upgradeView=="Upgraded"then self.upgradeView="Max upgraded"end
    local available=self:Capability();local stats=current and current.snapshot.client=="forever"
    panel.run:SetText(difficultyChanged and("Scan "..selectedRaidName.." drops")or"Find upgrades")
    panel.run:SetEnabled(available and not stats and not transferBusy());panel.notes:SetEnabled(result~=nil)
    local reroll=self.rank=="Reroll value";local planning=self.rank and self.rank~="Item gain"
    panel.assumptions:SetShown(planning==true);panel.loot:SetShown(not planning);panel.assumption:SetShown(reroll);panel.plan:SetShown(planning==true);panel.plan:SetEnabled(result~=nil and not transferBusy());panel.filter:SetShown(true);panel.filter:SetEnabled(result~=nil)
    panel.rank:SetText(self.rank or "Item gain");panel.view:SetText(reroll and "Bonus reward" or self.upgradeView or "Drop now");panel.view:SetEnabled(not reroll)
    panel.filter:SetText(self.upgradeFilter or "All")
    panel.assumption:SetText(self.bonusPool and "Using journal pool (assumed)" or "Reroll pool: not confirmed")
    setText(panel.costLabel,reroll and "Currency per roll" or "Minutes / run")
    local rows={}
    if difficultyChanged then
		setText(panel.headline,"Raid difficulty changed to "..selectedRaidName)
		setText(panel.context,"Run the new drop scan to rebuild the loot pool. Results from the previous difficulty are hidden.")
	elseif result then
        setText(panel.headline,"Ranked against your captured equipped setup · "..validationText(result.validation))
        setText(panel.context,scenarioText(job).." · run "..stamp(job.createdAt)..(result.metric=="group_dps" and " · simulated allies; gains use total Group DPS" or ""))
        if planning then
            local planJob=self:PlanFor(job);local plan=planJob and planJob.plan
            if planJob and planJob.error then setText(panel.context,"Plan failed: "..clean(planJob.error,240))end
            if plan then
                for _,source in ipairs(plan.sources or {})do
					local filter=self.upgradeFilter or"All";local upgrades=tonumber(source.upgradeCount)or 0;local bis=tonumber(source.bisCount)or 0
					if filter=="All"or filter=="Upgrades"and upgrades>0 or filter=="BiS"and bis>0 then
					local facts={};local status=bis>0 and(upgrades>0 and"BiS + Upgrade"or"BiS")or upgrades>0 and"Upgrade"or"No upgrade"
					facts[#facts+1]=status;if num(source.pool)then facts[#facts+1]=tostring(source.pool).." modeled item"..(source.pool==1 and""or"s")end
					if num(source.upgradeChance)then facts[#facts+1]=string.format("%.0f%% upgrade chance",source.upgradeChance*100)end
					if source.attempts50 then facts[#facts+1]="50% by about "..tostring(source.attempts50).." runs"end
					local prefix=tostring(source.id or""):match("^([^:]+)");prefix=prefix=="dungeon"and"Dungeon"or prefix=="raid"and"Raid"or"Source"
					rows[#rows+1]={label=prefix.." · "..source.name,source=table.concat(facts," · "),detail="Expected "..gain(source.expected).." · "..gain(source.rate)..(reroll and" / currency"or" / hour")}
					end
				end
                local options=plan.options or {}
                setText(panel.context,"Saved plan: "..clean(options.view or "").." · "..tostring(options.chance and options.chance*100 or "?").."% gear chance · "..tostring(options.cost or "?")..(options.mode=="reroll" and " currency / roll" or " minutes / run")..". Recalculate after changing assumptions.")
            end
        else
            local kinds={["Drop now"]="drop",["Max upgraded"]="max",["Bonus reward"]="bonus",Crafted="crafted"};local kind=kinds[self.upgradeView or "Drop now"]
            local filter=self.upgradeFilter or"All"
            for _,item in ipairs(result.upgrades or {})do if item.type==kind and(filter=="All"or filter=="Upgrades"and(num(item.gain)or 0)>0 or filter=="BiS"and item.bis==true)then rows[#rows+1]=item end end
            table.sort(rows,function(a,b)return (a.gain or -math.huge)>(b.gain or -math.huge)end)
			local meta=result.builds and(" · BiS: "..clean(result.builds.specName or"current spec",70).." top-player M+ · updated "..stamp(result.builds.updatedAt)..(result.builds.stale and" (refresh recommended)"or""))or" · BiS reference unavailable; run a new scan"
			setText(panel.context,scenarioText(job).." · "..(kind=="max"and"each drop shown at its captured maximum rank"or kind=="drop"and"each item shown at the selected difficulty's drop level"or"selected reward view")..meta.." · run "..stamp(job.createdAt))
        end
    else
        setText(panel.headline,stats and "Upgrade rankings are unavailable on this client." or "Find equipment worth comparing.")
        setText(panel.context,job and clean(job.error or ("Last loaded status: "..tostring(job.state)),220) or "The engine compares eligible journal loot. Keep WoW in front while the request is sent.")
    end
    local pages=math.max(1,math.ceil(#rows/6));self.upgradePage=math.min(math.max(1,self.upgradePage),pages)
    for i,row in ipairs(panel.rows)do local item=rows[(self.upgradePage-1)*6+i];row.item=nil;row.compareText=nil;row:SetShown(item~=nil)
        if item then
            row.icon:SetTexture(self:ItemTexture(item));row.icon:SetShown(not planning)
            if planning then
                setText(row.nameLabel,clean(item.label or item.name,70));setText(row.source,clean(item.source,100));setText(row.farm,"");row.nameLabel:SetTextColor(.86,.91,.95);row.source:SetTextColor(.86,.91,.95)
            else
                local farm=item.kind=="Dungeon"and("Dungeon · "..clean(item.source,52))or item.kind=="Raid"and("Raid · "..clean(item.source,55))or clean(item.source,62)
                local isUpgrade=(num(item.gain)or 0)>0;local status=item.bis and(isUpgrade and"BiS + Upgrade"or"BiS")or isUpgrade and"Upgrade"or"Not an upgrade"
                local swap=self:SwapInfo(job,item,result and result.baseline);row.item=swap.item or item;row.compareText=swap.tooltip
                setText(row.nameLabel,status.." · "..clean(item.label or item.name,55)..(item.level and" ("..item.level..")"or""));setText(row.farm,farm);setText(row.source,swap.text~=""and swap.text:gsub("\n"," · ")or(sheetSlotNames[item.slot]or"Slot unavailable"));row.nameLabel:SetTextColor(.86,.91,.95)
                if item.bis then row.source:SetTextColor(1,.82,.26)elseif isUpgrade then row.source:SetTextColor(.3,1,.5)else row.source:SetTextColor(.68,.76,.82)end
            end
            setText(row.value,item.detail or upgradeText(item,result))
			if planning then
                row.value:SetTextColor(.86,.91,.95);decisionBorder(row,nil,false,false)
            else
                local baseline=result and result.baseline and num(result.baseline.mean);local mean=num(item.mean)
                local value=num(item.gain)and{percent=item.gain,absolute=baseline and mean and mean-baseline,uncertain=false}or nil
                decisionColor(row.value,value,false);decisionBorder(row,value,false,false)
            end
        end
    end
    panel.empty:SetShown(#rows==0);setText(panel.empty,difficultyChanged and("Scan "..selectedRaidName.." drops to see results for this difficulty.")or planning and "Calculate a plan, then check for results. Only complete, modeled pools can be ranked." or result and self.upgradeFilter=="BiS"and not result.builds and"This older scan has no current BiS reference. Run Find upgrades again."or result and("No "..string.lower(self.upgradeFilter or"matching").." outcomes for this view. Choose another filter or view.")or"No upgrade result loaded yet.")
    panel.prev:SetEnabled(self.upgradePage>1);panel.next:SetEnabled(self.upgradePage<pages);setText(panel.count,#rows.." "..string.lower(self.upgradeFilter or"all").." outcomes · page "..self.upgradePage.." / "..pages)
    panel.target:SetEnabled(result and result.targetSet~=nil)
end
function U:BuildHistory(panel)
    panel.heading=label(panel,"Past runs for this character and specialization. Selecting a run never changes your gear or talents.",4,-4,924)
    panel.rows={}
    for i=1,10 do
        local row=button(panel,"",4,-45-(i-1)*38,928,function()end);row:SetHeight(33)
        row:SetScript("OnClick",function(self)
            if not self.job then return end
            U.selectedJobID=self.job.id
            local scenario=self.job.scenario or {};for _,key in ipairs({"targets","seconds","iterations","augmentationMode","allyItemLevel","allySkill","healingMode","fightStyle","raidBuffs"})do if scenario[key]~=nil then settings().scenario[key]=scenario[key]end end
            local overview=U.panels.overview
            for key,field in pairs(overview.fields)do local value=U:Scenario()[key];field:SetText(tostring(key=="allySkill" and value*100 or value))end
            local task=taskOf(self.job)
            if task=="builds" then settings().buildMode=scenario.buildMode or"m+";settings().buildSpec=scenario.buildSpec or(self.job.result and self.job.result.builds and self.job.result.builds.spec)end
            U:SelectTab(task=="healing" and "healing" or task=="builds" and "meta" or task=="analysis" and "summary" or (task=="upgrades" or task=="plan") and "upgrades" or task=="talents" and "talents" or task=="compare" and "gear" or "overview")
        end);panel.rows[i]=row
    end
    panel.prev=button(panel,"Previous",4,-455,100,function()U.historyPage=math.max(1,U.historyPage-1);U:Refresh()end)
    panel.next=button(panel,"Next",112,-455,90,function()U.historyPage=U.historyPage+1;U:Refresh()end);panel.count=label(panel,"",218,-461,712)
end
function U:RefreshHistory(panel)
    local jobs=self:Jobs();local pages=math.max(1,math.ceil(#jobs/10));self.historyPage=math.min(math.max(1,self.historyPage),pages)
    for i,row in ipairs(panel.rows)do local job=jobs[(self.historyPage-1)*10+i];row.job=job;row:SetShown(job~=nil)
        if job then local result=job.result;local task=taskOf(job);local taskName=({baseline="Reference",healing="Healing encounter",builds="Build reference",analysis="Bag + Vault analysis",compare="Gear comparison",talents="Talent builds",upgrades="Upgrades",plan="Farm / reroll plan",cancel="Cancellation"})[task]or task
            row:SetText(stamp(job.createdAt).."  ·  "..taskName.."  ·  "..clean(job.state)..(result and task~="builds" and "  ·  "..fmt(result.baseline and result.baseline.mean).." "..metric(result) or ""))
        end
    end
    panel.prev:SetEnabled(self.historyPage>1);panel.next:SetEnabled(self.historyPage<pages)
    local omitted=self:Envelope().omittedJobs or 0
    setText(panel.count,#jobs.." saved runs · page "..self.historyPage.." / "..pages..(omitted>0 and " · older runs remain in Lamda Engine" or ""))
end
function U:ShowDetail(title,lines,copyURL)
    if not self.details then
        local frame=card(self.frame,60,-74,880,554);self.details=frame;frame:SetFrameLevel(self.frame:GetFrameLevel()+20);frame:EnableMouse(true)
        frame.title=label(frame,"",18,-16,795,true)
        button(frame,"Close",752,-510,108,function()frame:Hide()end)
        local scroll=CreateFrame("ScrollFrame",nil,frame,"UIPanelScrollFrameTemplate");scroll:SetPoint("TOPLEFT",18,-52);scroll:SetPoint("BOTTOMRIGHT",-36,58)
        local child=CreateFrame("Frame",nil,scroll);child:SetSize(802,450);scroll:SetScrollChild(child)
        frame.body=label(child,"",0,0,796);frame.body:SetSpacing(6);frame.child=child;frame.scroll=scroll;frame:Hide()
        frame.copyURL=edit(frame,24,-470,828,"",2048)
        frame.copyHint=label(frame,"Select the link and press Ctrl+C to copy it into your browser.",18,-513,714)
        frame:SetScript("OnHide",function()frame.copyURL:ClearFocus()end)
    end
    local frame=self.details;local hasURL=type(copyURL)=="string" and copyURL~=""
    frame.copyURL:ClearFocus();frame.copyURL:SetShown(hasURL);frame.copyHint:SetShown(hasURL)
    frame.copyURL:SetText(hasURL and clean(copyURL,2048)or "")
    frame.scroll:SetPoint("BOTTOMRIGHT",-36,hasURL and 116 or 58)
    setText(frame.title,title);setText(frame.body,table.concat(lines,"\n\n"));frame.child:SetHeight(math.max(430,frame.body:GetStringHeight()+20));frame.scroll:SetVerticalScroll(0);frame:Show()
    if hasURL then frame.copyURL:SetFocus();frame.copyURL:HighlightText()end
end
function U:ShowAllResults(job)
    if not job or not job.result then return end
    local result=job.result;local baseline=result.baseline or {}
    local lines={"Saved run: "..stamp(job.createdAt),scenarioText(job)}
    if result.metric=="item_stats" then lines[#lines+1]="Changes in captured item stats, compared with equipped gear. These are not throughput rankings."
    else lines[#lines+1]="Baseline used for comparison (not a rating): "..fmt(baseline.mean).." "..metric(result)..((baseline.meanError or 0)>0 and " ± "..fmt(baseline.meanError) or "") end
    for _,row in ipairs(resultRows(job,self:SummaryFor(job)))do
        local detail=clean(row.name,160).."\n"..row.detail
        if row.comparison and (row.comparison.meanError or 0)>0 then detail=detail.." · uncertainty ± "..fmt(row.comparison.meanError)end
        lines[#lines+1]=detail
    end
    if result.validation=="review-required" then lines[#lines+1]="Needs model review. See Model notes for limitations."end
    self:ShowDetail("All comparison results",lines)
end
function U:ShowNotes(job)
    if not job then return end
    local result=job.result or {};local lines={"Captured run: "..stamp(job.createdAt),scenarioText(job)}
    local healing=result.healing;local training=healing and healing.training
    if training then
        lines[#lines+1]="Encounter learning: "..clean(training.encounter).."\n"..clean(training.cohort).."\n"..#(training.trainingIds or{}).." training fights; one separate, latest fight held out. "..clean(training.method)
        lines[#lines+1]=string.format("Workload validation against the held-out fight: incoming DPS %+.1f%%; largest five-second damage burst %+.1f%%. These errors measure the damage model, not the accuracy of your simulated healing.",training.damageErrorPercent or 0,training.burstErrorPercent or 0)
        for _,out in ipairs(healing.holdoutOutcomes or{})do lines[#lines+1]=string.format("Held-out fight · %s: %.0f effective HPS, %.0f absorbed/s, %.2f deaths/run, %.1f%% mana remaining.",out.policy,out.hps,out.absorbPs or 0,out.deaths,out.manaEnd)end
    end
    if healing and healing.stats then
        local included={};for _,id in ipairs(healing.modeledTalents or{})do included[id]=true end
        local missing={};for id in pairs(healing.stats.talents or{})do if not included[tonumber(id)]then missing[#missing+1]=tonumber(id)end end;table.sort(missing)
        local names={};for _,id in ipairs(missing)do
            local name=C_Spell and C_Spell.GetSpellName and C_Spell.GetSpellName(id)
            names[#names+1]=clean(name or ("Spell "..id))
        end
        lines[#lines+1]="Selected talents not evaluated by this model (includes utility talents): "..table.concat(names,", ")..". Modeled talents can still have omitted interactions; this is not a throughput-coverage percentage."
    end
    for _,warning in ipairs(result.warnings or {})do lines[#lines+1]=clean(warning,1600)end
    local keys={};for key in pairs(result.modelInputs or {})do keys[#keys+1]=key end;table.sort(keys)
    for _,key in ipairs(keys)do lines[#lines+1]=clean(key)..": "..clean(result.modelInputs[key],1600)end
    for _,rejected in ipairs(result.rejectedComparisons or {})do lines[#lines+1]="Skipped "..clean(rejected.label or rejected.id)..": "..clean(rejected.reason,800)end
    local planJob=self.tab=="upgrades" and self.rank and self.rank~="Item gain" and self:PlanFor(job)
    for _,warning in ipairs(planJob and planJob.plan and planJob.plan.warnings or {})do lines[#lines+1]=clean(warning,1600)end
    self:ShowDetail("Model notes",lines)
end
function U:ShowTarget()
    local job=self:Job("upgrades");local result=job and job.result;local target=result and result.targetSet;if not target then return end
    local baseline=result.baseline and num(result.baseline.mean);local absolute=baseline and num(target.mean)and target.mean-baseline
    local lines={"Best tested set, not a global BiS guarantee.",(absolute and signed(absolute).." "..throughputUnit(result).." ("..gain(absolute/baseline*100)..")" or "Gain unavailable").." · "..tostring(result.searchTests or 0).." tested combinations"}
    for _,item in ipairs(target.items or {})do lines[#lines+1]=(slotNames[item.slot]or item.slot or "")..": "..clean(item.label or "Equipped item")..(item.level and " ("..item.level..")" or "")end
    self:ShowDetail("Best tested set",lines)
end
function U:BuildReferenceSpec()
    local selected=settings().buildSpec
    if selected and buildSpecs[selected]then return selected end
    local current=self:Current();local character=current and current.snapshot and current.snapshot.character or{}
    return buildSpecs[character.spec] and character.spec or 1468
end
function U:EncounterCapability(task)
    local current=self:Current();local snapshot=current and current.snapshot;local character=snapshot and snapshot.character or{}
    if not snapshot or snapshot.client~="retail" then return false,"Capture a Retail character to use this page." end
    if task=="builds" then return buildSpecs[self:BuildReferenceSpec()]~=nil end
    local spec=buildSpecs[character.spec]
    if not spec or not spec.healer then return false,"Healing Lab supports all seven healer specializations. Switch to a healer and refresh character data in Setup." end
    if character.level~=90 or snapshot.interface and (snapshot.interface<120100 or snapshot.interface>=120200)then return false,"Healing Lab models level 90 on Patch 12.1." end
    if not snapshot.healingStats then return false,"Refresh character data in Setup outside combat to capture healing stats, spells and talents." end
    return true
end
local healingModes={"Learned local encounters","Tank burst","Dungeon group","Raid assignment","Imported log replay"}
local healingModeKeys={["Learned local encounters"]="learned",["Tank burst"]="tank",["Dungeon group"]="dungeon",["Raid assignment"]="raid",["Imported log replay"]="replay"}
local function healingMode()return settings().healingEncounter or "Learned local encounters" end
local function learnedLabel(row,index)
    local difficulty=(row.cohort or ""):match("^[^/]+/[^/]+/(.+)/[^/]+$")or ""
    return tostring(index)..". "..clean(row.encounter).." · "..difficulty
end
local buildModes={"Mythic+","Solo Shuffle","2v2","3v3","Blitz","RBG"}
local buildModeKeys={["Mythic+"]="m+",["Solo Shuffle"]="solo",["2v2"]="2v2",["3v3"]="3v3",Blitz="blitz",RBG="rbg"}
local function buildModeName(key)for name,value in pairs(buildModeKeys)do if value==key then return name end end;return "Mythic+" end
function U:BuildHealing(panel)
    panel.title=label(panel,"Healing Lab",4,-2,922,true)
    panel.scope=label(panel,"Learn encounter damage from local logs and simulate healing with your captured build. Talent and gear effects are still incomplete; see Model details.",4,-30,922)
    dropdown(panel,4,-74,188,healingModes,healingMode,function(v)settings().healingEncounter=v end)
    panel.seconds=dropdown(panel,200,-74,130,{"60 seconds","180 seconds","300 seconds"},function()return (healingMode()=="Imported log replay" or healingMode()=="Learned local encounters") and "From logs" or(settings().healingSeconds or 180).." seconds"end,function(v)settings().healingSeconds=tonumber(v:match("%d+"))end)
    panel.intensity=dropdown(panel,338,-74,144,{"10% damage","25% damage","50% damage","75% damage","100% damage","125% damage","150% damage","200% damage"},function()return(settings().healingIntensity or 100).."% damage"end,function(v)settings().healingIntensity=tonumber(v:match("%d+"))end)
    LUI:Explain(panel.seconds,"Encounter duration","Scripted encounters use the selected duration. Imported log replays always use the full recorded encounter.")
    LUI:Explain(panel.intensity,"Incoming damage","Scales incoming damage without changing your healing spells or stats. Lower this for a lighter workload; raise it to explore burst survival. 100% is the scenario's defined damage, not a dungeon difficulty rating.")
    dropdown(panel,490,-74,130,{"10 runs","25 runs","50 runs"},function()return(settings().healingRuns or 25).." runs"end,function(v)settings().healingRuns=tonumber(v:match("%d+"))end)
    panel.run=button(panel,"Run encounter",630,-74,150,function()
        U:Send("healing",{compareMode="individual",scenario={targets=1,iterations=100,seconds=settings().healingSeconds or 180,
            healingScenario=healingModeKeys[healingMode()],healingCohort=healingMode()=="Learned local encounters" and settings().healingCohort or nil,healingIntensity=settings().healingIntensity or 100,healingRuns=settings().healingRuns or 25,healingPolicy="compare"}})
    end,"Learns completed local fights in the companion and caches parsed logs. Needs three comparable fights; the latest is held out. Both policies receive identical sampled encounters. Uses freshly captured stats and modeled talents, starting with full resources and ready cooldowns.")
    button(panel,"Model details",788,-74,140,function()
        local j=U:Job("healing");if j then U:ShowNotes(j);return end
        U:ShowDetail("Healing models",{"All seven healers have separate experimental core-spell policies. Run your active healer to see its exact coverage and missing mechanics.","Discipline uses Atonement and shields; Holy Priest uses Holy Words and Echo of Light; Paladin generates/spends Holy Power; Druid uses HoTs; Shaman uses Deep Healing and totems; Mistweaver currently models caster healing; Preservation uses Echo and Essence.","Hero/apex talents, many optional talents and gear effects are still incomplete. These models cannot yet rank complete builds or healer classes.","Compare effective healing, consumed absorbs, deaths and mana against identical damage. Low incoming damage can cap useful HPS."})
    end)

    panel.context=label(panel,"",4,-112,924)
    label(panel,"Absorb/s",810,-148,112);label(panel,"Policy",12,-148,210);label(panel,"Effective HPS",230,-148,150);label(panel,"Overheal",388,-148,125);label(panel,"Deaths / run",520,-148,150);label(panel,"Mana left",680,-148,115)
    panel.outcomes={}
    for i=1,2 do local row=card(panel,4,-174-(i-1)*35,924,32);row.name=label(row,"",8,-8,210);row.hps=label(row,"",226,-8,150);row.over=label(row,"",384,-8,125);row.deaths=label(row,"",516,-8,150);row.mana=label(row,"",676,-8,120);row.absorb=label(row,"",806,-8,112);panel.outcomes[i]=row end
    panel.trace=label(panel,"First run · drag the timeline to inspect health and resources",4,-253,924)
    panel.timeline=CreateFrame("Slider",nil,panel,"OptionsSliderTemplate");panel.timeline:SetPoint("TOPLEFT",10,-286);panel.timeline:SetSize(910,16);panel.timeline:SetMinMaxValues(1,2);panel.timeline:SetValueStep(1);panel.timeline:SetObeyStepOnDrag(true)
    if panel.timeline.Low then panel.timeline.Low:SetText("")end;if panel.timeline.High then panel.timeline.High:SetText("")end
    panel.timeline:SetScript("OnValueChanged",function(_,value)if not panel.settingTimeline then panel.point=math.floor(value+.5);U:RefreshHealingTrace(panel)end end)
    panel.bars={}
    for i=1,20 do local bar=CreateFrame("StatusBar",nil,panel);bar:SetPoint("TOPLEFT",4+((i-1)%5)*186,-316-math.floor((i-1)/5)*24);bar:SetSize(174,19);bar:SetStatusBarTexture("Interface\\Buttons\\WHITE8X8");bar:SetMinMaxValues(0,100);bar.text=label(bar,"",4,-3,166);bar.text:SetFontObject("GameFontHighlightSmall");panel.bars[i]=bar end
    panel.footer=label(panel,"",4,-423,924)
    button(panel,"Spell breakdown",4,-459,172,function()U:ShowHealingTrace(false)end)
    button(panel,"Cast timeline",184,-459,155,function()U:ShowHealingTrace(true)end)
    panel.guides=button(panel,"Guide consensus",347,-459,170,function()settings().buildSpec=nil;U:SelectTab("meta")end)
    panel.encounter=LUI:Dropdown(panel,347,-459,432,function()
        local values={"Latest eligible encounter"};for i,row in ipairs(panel.report and panel.report.learnedEncounters or{})do values[#values+1]=learnedLabel(row,i)end;return values
    end,function()
        for i,row in ipairs(panel.report and panel.report.learnedEncounters or{})do if row.cohort==settings().healingCohort then return learnedLabel(row,i)end end
        return settings().healingCohort and "Previously selected encounter" or "Latest eligible encounter"
    end,function(v)
        settings().healingCohort=nil;for i,row in ipairs(panel.report and panel.report.learnedEncounters or{})do if learnedLabel(row,i)==v then settings().healingCohort=row.cohort;break end end;U:Refresh()
    end)
    LUI:Explain(panel.encounter,"Learned encounters","Run once to build the local encounter library. Choose a learned encounter here, then Run encounter. Difficulty, key level, affixes and group roles stay separate. No log data leaves your computer.")
    button(panel,"More allies",788,-459,140,function()panel.allyOffset=(panel.allyOffset or 0)==0 and 20 or 0;U:RefreshHealingTrace(panel)end)
end
function U:RefreshHealingTrace(panel)
    local report=panel.report;local outcome=report and report.outcomes and report.outcomes[1];local points=outcome and outcome.timeline or{}
    local point=points[math.max(1,math.min(panel.point or 1,#points))]
    if point and (panel.allyOffset or 0)>=#point.health then panel.allyOffset=0 end
    for index,bar in ipairs(panel.bars)do local i=index+(panel.allyOffset or 0);local hp=point and point.health and point.health[i];bar:SetShown(hp~=nil);if hp then bar:SetValue(hp);bar:SetStatusBarColor(hp==0 and .4 or hp<30 and .9 or .08,hp<30 and .12 or .58,.22,1);local member=report and report.replay and report.replay.members[i];setText(bar.text,(member and clean(member.label)or i==1 and"You"or i==2 and"Tank"or"Ally "..i).." · "..(hp==0 and"Dead"or string.format("%.0f%%",hp)))end end
    local resource="";if point then
        local name=point.resourceName or (report and report.spec==nil and "Essence")
        if name and name~=""then resource=" · "..name.." "..string.format("%.1f",name=="Essence"and(point.essence or 0)or(point.resource or 0))end
    end
    setText(panel.trace,point and string.format("First seed · preparation · %.1fs · mana %.0f%%",point.time,point.mana)..resource or"Run an encounter to inspect the health timeline.")
end
function U:RefreshHealing(panel)
    local current=self:Current();local character=current and current.snapshot and current.snapshot.character or{};local spec=buildSpecs[character.spec]
    setText(panel.title,"Healing Lab · "..(spec and(spec.name.." "..spec.className)or"choose a healer").." · experimental")
    local available,reason=self:EncounterCapability("healing");panel.run:SetEnabled(available and not transferBusy())
    panel.seconds:SetEnabled(healingMode()~="Imported log replay" and healingMode()~="Learned local encounters")
    panel.encounter:SetShown(healingMode()=="Learned local encounters");panel.guides:SetShown(healingMode()~="Learned local encounters")
    local job=self:Job("healing");local report=job and job.result and job.result.healing;panel.report=report
    setText(panel.context,job and("Captured "..stamp(job.createdAt).." · "..clean(job.error or report and((report.replay and report.replay.encounter or report.options.scenario).." · "..(report.replay and report.replay.seconds or report.options.seconds).."s · "..report.options.intensity.."% damage · "..report.options.runs.." runs")or job.state,500))or reason or"Choose an encounter. The same damage script is used for both policies.")
    for i,row in ipairs(panel.outcomes)do local out=report and report.outcomes[i];row:SetShown(out~=nil);if out then setText(row.name,out.policy=="consensus"and"Guide-informed preparation"or"Reactive comparison");setText(row.hps,fmt(out.hps).." ± "..fmt(out.meanError));setText(row.over,string.format("%.1f%%",out.overhealPercent));setText(row.deaths,string.format("%.2f",out.deaths));setText(row.mana,string.format("%.1f%%",out.manaEnd));setText(row.absorb,fmt(out.absorbPs or 0))end end
    local points=report and report.outcomes[1] and report.outcomes[1].timeline or{};panel.timeline:SetEnabled(#points>0)
    panel.settingTimeline=true;panel.timeline:SetMinMaxValues(1,math.max(2,#points));panel.point=math.min(panel.point or 1,math.max(1,#points));panel.timeline:SetValue(panel.point);panel.settingTimeline=false
    self:RefreshHealingTrace(panel)
    setText(panel.footer,"± is sampling uncertainty, not model accuracy. Details list omitted mechanics. A raid assignment is one healer's workload, without other healers.")
    if report and report.training then
        local t=report.training
        setText(panel.context,"Captured "..stamp(job.createdAt).." · "..clean(t.encounter).." · "..#(t.trainingIds or{}).." training fights + 1 held out · "..string.format("%.0f–%.0fs",t.minSeconds,t.maxSeconds).." · "..report.options.runs.." runs/policy")
        setText(panel.footer,string.format("Held-out workload error: incoming DPS %+.1f%% · five-second burst %+.1f%%. Model details show held-out healing results and missing build effects.",t.damageErrorPercent,t.burstErrorPercent))
    end
end
function U:ShowHealingTrace(casts)
    local job=self:Job("healing");local report=job and job.result and job.result.healing;if not report then self.notice="Run an encounter first.";self:Refresh();return end
    local lines={"Captured "..stamp(job.createdAt)}
    for _,out in ipairs(report.outcomes)do
        lines[#lines+1]="";lines[#lines+1]=out.policy.." · "..fmt(out.hps).." effective HPS · "..fmt(out.rawHps).." raw HPS · "..fmt(out.absorbPs or 0).." absorbed/s"
        lines[#lines+1]=string.format("No-death runs: %.0f%% · critical health: %.1fs · mana-starved: %.1fs",out.survivalPercent,out.criticalSeconds,out.manaStarvedSeconds)
        if casts then for _,cast in ipairs(out.casts or{})do
            local origin=cast.origin=="stasis" and " [Stasis replay]" or cast.origin=="automatic" and " [automatic]" or ""
            lines[#lines+1]=string.format("%6.2fs  %s%s → ally %d",cast.time,cast.spell,origin,cast.target)
        end
        else for _,sp in ipairs(out.spells or{})do
            local replay=(sp.replays or 0)>0 and (" + "..sp.replays.." Stasis replays") or ""
            lines[#lines+1]=clean(sp.name)..": "..sp.casts.." casts"..replay.." across all runs · "..fmt(sp.healing).." effective · "..fmt(sp.overheal).." overheal · "..fmt(sp.absorbed or 0).." absorbed"
        end end
    end
    self:ShowDetail(casts and"Cast timeline · first seed"or"Spell totals · all seeds",lines)
end
function U:BuildMeta(panel)
    panel.title=label(panel,"What top players use",4,-2,924,true)
    local classes={};local seen={};for _,entry in pairs(buildSpecs)do if not seen[entry.className]then seen[entry.className]=true;classes[#classes+1]=entry.className end end;table.sort(classes)
    panel.class=LUI:Dropdown(panel,4,-36,186,function()return classes end,function()return buildSpecs[U:BuildReferenceSpec()].className end,function(value)
        local first;for id,entry in pairs(buildSpecs)do if entry.className==value and(not first or id<first)then first=id end end;settings().buildSpec=first;U.metaPage=1;U:Refresh()
    end)
    panel.spec=LUI:Dropdown(panel,198,-36,184,function()
        local values={};local class=buildSpecs[U:BuildReferenceSpec()].class;for _,entry in pairs(buildSpecs)do if entry.class==class then values[#values+1]=entry.name end end;table.sort(values);return values
    end,function()return buildSpecs[U:BuildReferenceSpec()].name end,function(value)
        local class=buildSpecs[U:BuildReferenceSpec()].class;for id,entry in pairs(buildSpecs)do if entry.class==class and entry.name==value then settings().buildSpec=id;break end end;U.metaPage=1;U:Refresh()
    end)
    panel.mode=dropdown(panel,390,-36,180,buildModes,function()return buildModeName(settings().buildMode or"m+")end,function(v)settings().buildMode=buildModeKeys[v];U.metaPage=1 end)
    panel.mine=button(panel,"My specialization",578,-36,170,function()settings().buildSpec=nil;U.metaPage=1;U:Refresh();LUI:RefreshUI()end)
    panel.refresh=button(panel,"Load builds",756,-36,172,function()U:Send("builds",{compareMode="individual",scenario={targets=1,seconds=300,iterations=100,buildMode=settings().buildMode or"m+",buildSpec=U:BuildReferenceSpec()}})end,"Loads the selected specialization and mode. Data is cached locally for eight hours.")
    panel.talentRates=button(panel,"Talent pick rates",4,-72,172,function()local r=panel.report;if not r then return end;local lines={(r.specName or "").." · "..r.source.." · "..buildModeName(r.mode).." · sample "..r.sample.." · updated "..stamp(r.updatedAt)};local choices={};for _,t in ipairs(r.talents or{})do choices[#choices+1]=t end;table.sort(choices,function(a,b)if a.count~=b.count then return a.count>b.count end;return a.name<b.name end);for _,t in ipairs(choices)do lines[#lines+1]=clean(t.name).." · "..t.count.."/"..r.sample.." ("..string.format("%.0f%%",t.percent)..")"end;U:ShowDetail("Talent popularity · not a complete build",lines)end)
    panel.sources=button(panel,"Sources & consensus",184,-72,192,function()U:ShowBuildSources(panel.report)end)
    panel.gear=button(panel,"Stats & BiS gear",384,-72,178,function()
        local r=panel.report;if not r then return end;local lines={clean(r.specName).." · "..buildModeName(r.mode).." · updated "..stamp(r.updatedAt)}
        local priorities={};local stats={};for _,stat in ipairs(r.stats or{})do stats[#stats+1]=stat end;table.sort(stats,function(a,b)return(a.rank or 99)<(b.rank or 99)end)
        for _,stat in ipairs(stats)do priorities[#priorities+1]=clean(stat.name).." "..string.format("%.0f%%",stat.percent or 0).." ("..fmt(stat.rating).." rating)"end
        lines[#lines+1]="Current top-player stat priority: "..table.concat(priorities," > ")
        for _,item in ipairs(r.gear or{})do if item.bis then lines[#lines+1]=clean(item.slot)..": "..clean(item.name)..(item.rank and item.rank>1 and(" · #"..item.rank)or"")end end
        lines[#lines+1]="BiS here means most frequently equipped by this top-player sample. Personal Upgrade labels come from your completed simulation."
        U:ShowDetail("Current stats & BiS reference",lines,r.url)
    end)
    panel.context=label(panel,"",4,-106,924)
    panel.heroes=label(panel,"",4,-131,924,true)
    panel.headers={label(panel,"Rank / player",12,-150,215),label(panel,"Realm",245,-150,220),label(panel,"Hero / gear",475,-150,330),label(panel,"Rating",834,-150,90)}
    panel.empty=label(panel,"No build data loaded.\n\nChoose a specialization and activity, then click Load builds. This page shows what top players use; it does not simulate your character.",20,-196,884,true);panel.empty:SetSpacing(9)
    panel.rows={}
    for i=1,7 do local row=CreateFrame("Button",nil,panel,"BackdropTemplate");row:SetPoint("TOPLEFT",4,-175-(i-1)*34);row:SetSize(924,31);row:SetBackdrop(backdrop);row:SetBackdropColor(.035,.065,.09,.9);row:SetBackdropBorderColor(.12,.2,.25,.7);row.name=label(row,"",8,-7,225);row.realm=label(row,"",240,-7,225);row.details=label(row,"",470,-7,345);row.rating=label(row,"",830,-7,84)
        row:SetScript("OnClick",function(self)if self.player then U:ShowDetail("Player build source",{clean(self.player.name).." · "..clean(self.player.realm),clean(self.player.details),"Open this page in your browser for the player's complete build and import code. Individual talent popularity cannot safely be combined into a valid build.","You can save a copied import code in the Talents tab. Applying talents to your character remains a manual choice."},self.player.url)end end);panel.rows[i]=row end
    panel.note=label(panel,"",4,-424,924)
    panel.prev=button(panel,"Previous",4,-466,95,function()U.metaPage=math.max(1,(U.metaPage or 1)-1);U:Refresh()end)
    panel.next=button(panel,"Next",107,-466,75,function()U.metaPage=(U.metaPage or 1)+1;U:Refresh()end)
    panel.count=label(panel,"",198,-471,720)
end
function U:RefreshMeta(panel)
    local available,reason=self:EncounterCapability("builds");panel.refresh:SetEnabled(available and not transferBusy())
    local mode=settings().buildMode or"m+";local specID=self:BuildReferenceSpec();local job
    local entry=buildSpecs[specID];setText(panel.title,"What top players use · "..entry.name.." "..entry.className)
    local function matches(candidate)
        local r=candidate.result and candidate.result.builds;local summary=U:SummaryFor(candidate);local captured=summary and summary.snapshot and summary.snapshot.character.spec
        return taskOf(candidate)=="builds" and ((candidate.scenario or{}).buildMode or"m+")==mode and (r and r.spec or (candidate.scenario or{}).buildSpec or captured)==specID
    end
    for _,candidate in ipairs(self:Jobs())do if matches(candidate) then job=candidate;break end end
    local report=job and job.result and job.result.builds
    -- A failed refresh keeps the last completed snapshot visible, with its date.
    if not report then for _,candidate in ipairs(self:Jobs())do if matches(candidate) and candidate.result and candidate.result.builds then report=candidate.result.builds;break end end end
    panel.report=report
    panel.refresh:SetText(report and"Refresh builds"or"Load builds");panel.talentRates:SetEnabled(report~=nil);panel.gear:SetEnabled(report~=nil and#(report.gear or{})>0)
    setText(panel.context,report and(buildModeName(mode).." · "..tostring(report.sample or 0).." ranked players · "..(report.stale and"data may be old; refresh recommended"or"loaded locally"))or job and job.error and("Load failed: "..clean(job.error,420))or reason or"Choose a specialization and activity, then load its build sample.")
    local heroes={};for _,h in ipairs(report and report.heroes or{})do heroes[#heroes+1]=clean(h.name).." "..h.count.."/"..report.sample.." ("..string.format("%.0f%%",h.percent)..")"end
    setText(panel.heroes,table.concat(heroes,"   ·   "));panel.heroes:SetShown(report~=nil)
    panel.empty:SetShown(report==nil);for _,header in ipairs(panel.headers)do header:SetShown(report~=nil)end
    local players=report and report.players or{};local pages=math.max(1,math.ceil(#players/7));self.metaPage=math.min(self.metaPage or 1,pages)
    for i,row in ipairs(panel.rows)do local p=players[(self.metaPage-1)*7+i];row.player=p;row:SetShown(p~=nil);if p then setText(row.name,p.rank..". "..clean(p.name));setText(row.realm,clean(p.realm));setText(row.details,clean(p.details));setText(row.rating,fmt(p.rating))end end
    panel.note:SetShown(report~=nil);panel.prev:SetShown(report~=nil);panel.next:SetShown(report~=nil);panel.count:SetShown(report~=nil)
    setText(panel.note,job and job.error and("Refresh failed; the last completed sample is still shown. "..clean(job.error,300))or"Popularity is a reference, not simulated performance. Click a player for its source.")
    setText(panel.count,"Page "..self.metaPage.." / "..pages.." · top 10 shown")
end
function U:ShowBuildSources(report)
    if not report then self:ShowDetail("Build sources",{"Refresh builds to load the versioned source review.","Each specialization has its own source links. Healers include reviewed Method and Icy Veins priorities; Preservation also includes its existing QE, Wowhead and Spiritbloom review.","Shared authors count once. PvP popularity is not mixed into PvE strategy."});return end
    local lines={"Guide review: September 29, 2026 · Patch 12.1","Agreement is averaged across applicable independent authors. Context-dependent alternatives remain separate; talent strings and HPS are never averaged into a made-up build."}
    for _,r in ipairs(report.consensus or{})do lines[#lines+1]="";lines[#lines+1]=clean(r.context)..": "..clean(r.text,500).." · "..r.opinions.." independent sources" end
    for _,s in ipairs(report.sources or{})do lines[#lines+1]="";lines[#lines+1]=clean(s.name).." · reviewed "..clean(s.reviewed)..(s.updated~=""and" · source updated "..clean(s.updated)or" · source date unavailable");lines[#lines+1]=clean(s.use,500);lines[#lines+1]=s.url end
    for _,w in ipairs(report.warnings or{})do lines[#lines+1]=clean(w,700)end
    self:ShowDetail("Sources & guide consensus",lines)
end
function U:RefreshNavigation()
    if not self.nav then return end
    local current=self:Current();local character=current and current.snapshot and current.snapshot.character or {}
    local healer=character.role=="HEALER";local visible=healer and #mainTabs or #mainTabs-1
    local width=math.floor(940/visible);local x=20;local primary=primaryTabs[self.tab]or self.tab or"overview"
    for _,tab in ipairs(mainTabs)do
        local key=tab[1];local nav=self.nav[key];local shown=key~="healing"or healer
        nav:SetShown(shown)
        if shown then
            nav:ClearAllPoints();nav:SetPoint("TOPLEFT",x,-105);nav:SetSize(width-8,26)
            if nav.label then nav.label:SetWidth(width-20)end
            x=x+width
        end
        nav:SetSelected(shown and primary==key)
    end
    local gearShown=primary=="gear";self.gearNavFrame:SetShown(gearShown)
    for _,tab in ipairs(gearTabs)do self.gearNav[tab[1]]:SetSelected(gearShown and self.tab==tab[1])end
end
function U:Build()
    if self.frame then return end
    local frame=CreateFrame("Frame","LamdaSimWorkspace",UIParent,"BackdropTemplate");self.frame=frame;frame:Hide()
    frame:SetSize(980,735);frame:SetPoint("CENTER");frame:SetFrameStrata("DIALOG");frame:SetClampedToScreen(true)
    frame:SetBackdrop(backdrop);frame:SetBackdropColor(.025,.045,.065,.98);frame:SetBackdropBorderColor(.2,.65,.75,1)
    frame:EnableMouse(true);frame:SetMovable(true);frame:RegisterForDrag("LeftButton");frame:SetScript("OnDragStart",function(self)self:StartMoving()end);frame:SetScript("OnDragStop",function(self)self:StopMovingOrSizing()end)
    local width,height=UIParent:GetWidth(),UIParent:GetHeight();if width>0 and height>0 then frame:SetScale(math.min(1,(width-35)/980,(height-35)/735))end
    if UISpecialFrames then UISpecialFrames[#UISpecialFrames+1]="LamdaSimWorkspace" end
    label(frame,"LSim",20,-18,530,true);self.character=label(frame,"",20,-47,710)
    button(frame,"Close",858,-14,100,function()frame:Hide()end)
    self.identityNote=label(frame,"",20,-72,710)
    self.nav={}
    for _,tab in ipairs(mainTabs)do
        local key,name=tab[1],tab[2]
        self.nav[key]=LUI:Navigation(frame,name,20,-105,120,function()U:SelectTab(key)end)
    end
    self.gearNavFrame=CreateFrame("Frame",nil,frame);self.gearNavFrame:SetPoint("TOPLEFT",20,-136);self.gearNavFrame:SetSize(940,26);self.gearNavFrame:Hide();self.gearNav={}
    local gearWidth=math.floor(940/#gearTabs)
    for index,tab in ipairs(gearTabs)do
        local key,name=tab[1],tab[2]
        self.gearNav[key]=LUI:Navigation(self.gearNavFrame,name,(index-1)*gearWidth,0,gearWidth-8,function()U:SelectTab(key)end)
    end
    for _,tab in ipairs(panelTabs)do
        local key=tab[1]
        local panel=CreateFrame("Frame",nil,frame);panel:SetPoint("TOPLEFT",20,-168);panel:SetSize(940,475);panel:Hide();self.panels[key]=panel
    end
    self:BuildAdvisor(self.panels.advisor);self:BuildOverview(self.panels.overview);self:BuildSummary(self.panels.summary);self:BuildLoadout(self.panels.loadout);self:BuildCrest(self.panels.crest);self:BuildGear(self.panels.gear,"owned");self:BuildGear(self.panels.vault,"vault")
    self:BuildTalents(self.panels.talents);self:BuildUpgrades(self.panels.upgrades);self:BuildHistory(self.panels.history)
    self:BuildHealing(self.panels.healing);self:BuildMeta(self.panels.meta)
    self.statusLabel=label(frame,"",20,-655,705);self.statusLabel:SetMaxLines(2)
    self.progress=card(frame,20,-686,705,14);self.progress:Hide()
    self.progress.segment=self.progress:CreateTexture(nil,"ARTWORK");self.progress.segment:SetSize(145,12)
    self.progress.label=self.progress:CreateFontString(nil,"OVERLAY","GameFontHighlightSmall");self.progress.label:SetPoint("CENTER");self.progress.label:SetWidth(695);self.progress.label:SetMaxLines(1)
    self.stop=button(frame,"Stop analysis",742,-648,216,function()
        U.startToken=nil
        if reloadPending() and type(S.CancelAutoReload)=="function" then S:CancelAutoReload()else S:Stop()end
        U.requestAt=nil;U.status=nil;U:Refresh()
    end)
    self.cancel=button(frame,"Cancel calculation",742,-648,216,function()
        for _,job in ipairs(U:Jobs())do if job.state=="running" then U:Send("cancel",{jobID=job.id});return end end
    end,"Asks the engine to cancel the last loaded running job. Its response appears with the next result check.")
    self.reload=button(frame,"Load finished results",742,-648,216,function()
        if type(S.ReloadResults)=="function" then S:ReloadResults()
        elseif not InCombatLockdown or not InCombatLockdown()then ReloadUI()end
    end,"Appears only when the automatic result load needs help. It reloads the UI once to import the finished result.")
    local elapsed,progressElapsed=0,0
    frame:SetScript("OnUpdate",function(_,delta)
        elapsed=elapsed+delta;progressElapsed=progressElapsed+delta
        if elapsed>=.5 then elapsed=0;progressElapsed=0;U:RefreshStatus()
        elseif progressElapsed>=.05 then progressElapsed=0;U:RefreshProgress()end
    end)
    frame:RegisterEvent("PLAYER_REGEN_DISABLED");frame:RegisterEvent("PLAYER_REGEN_ENABLED")
    frame:SetScript("OnEvent",function(_,event)
        if not frame:IsShown() then return end
        if event=="PLAYER_REGEN_ENABLED" and U.capturePending and not transferBusy() then U:Current(true) end
        U:Refresh()
    end)
    frame:SetScript("OnHide",function()if LUI.openMenu then LUI.openMenu:Hide()end;if U.details then U.details:Hide()end;if U.confirmation then U.confirmation:Hide()end end)
end
function U:Refresh()
    if self.refreshing or not self.frame then return end
    self.refreshing=true
    if not InCombatLockdown or not InCombatLockdown() then
        local view=safe(S.GameSnapshot,S,false);if type(view)=="table" then self.current=normalize(view)end
    end
    local current=self:Current();local character=current and current.snapshot.character or {}
    setText(self.character,clean(character.name or "No character loaded").." · "..specName(character).." · level "..tostring(character.level or "?"))
    setText(self.identityNote,current and "Full analysis captures fresh gear, bags, talents and open Vault choices before it runs." or "Open Setup to capture this character and run an analysis.")
    local unit=character.role=="HEALER"and"score"or character.spec==1473 and"Group DPS"or"DPS"
    for key,panel in pairs(self.panels)do if panel.units then
        local applicable=not(key=="upgrades"and self.rank and self.rank~="Item gain")and not(key=="talents"and character.role=="HEALER")
        panel.units:SetShown(current~=nil and current.snapshot.client~="forever"and applicable)
        panel.units:SetText("Show: "..(resultDisplayMode()=="absolute"and unit or"%"))
    end end
    local pending=LUI:DB().simPendingRequestID
    if pending and pending~="" then for _,job in ipairs(self:Jobs())do if job.requestID==pending and (job.state=="complete" or job.state=="failed" or job.state=="cancelled" or job.state=="interrupted")then self.requestAt=nil;self.status=self.status or (job.state=="complete" and "Result loaded." or clean(job.error or job.state,250));break end end end
    local tab=self.tab or "overview"
    if tab=="healing"and character.role~="HEALER"then
        tab="overview";self.tab=tab;LUI:DB().simCurrentTab=tab
        for key,panel in pairs(self.panels)do panel:SetShown(key==tab)end
    end
    self:RefreshNavigation()
    if tab=="advisor"then self:RefreshAdvisor(self.panels.advisor)
    elseif tab=="healing"then self:RefreshHealing(self.panels.healing)
    elseif tab=="meta"then self:RefreshMeta(self.panels.meta)
    elseif tab=="overview" then self:RefreshOverview(self.panels.overview)
    elseif tab=="summary" then self:RefreshSummary(self.panels.summary)
	elseif tab=="loadout"then self:RefreshLoadout(self.panels.loadout)
    elseif tab=="crest" then self:RefreshCrest(self.panels.crest)
    elseif tab=="gear" then self:RefreshGear(self.panels.gear,"owned")
    elseif tab=="vault" then self:RefreshGear(self.panels.vault,"vault")
    elseif tab=="talents" then self:RefreshTalents(self.panels.talents)
    elseif tab=="upgrades" then self:RefreshUpgrades(self.panels.upgrades)
    elseif tab=="history" then self:RefreshHistory(self.panels.history)end
    self:RefreshStatus();self.refreshing=false
end
function U:Open()
    if not S:RequireEnabled() then return end
    self:Build();self:Current(true);local tab=self.openTabOnce or"overview";self.openTabOnce=nil
    self:SelectTab(tab);self.frame:Show();self:Refresh()
end
function S:OpenGameUI()U:Open()end
-- Expose small, deterministic view helpers for native regression tests.
U.Identity=identity;U.ResultRows=resultRows;U.ScenarioText=scenarioText;U.AdvisorData=advisorData
