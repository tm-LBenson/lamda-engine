local _,LUI=...
local G=LUI.Gladius
G.defaults.gladiusBridgeX=448;G.defaults.gladiusBridgeY=0
local module={id="gladius",name="LamdaGladius",experimental=true,enabledKey="gladiusEnabled",settingsScope="global",
    enabledHelp="Classic Gladius-style enemy arena frames. Includes class/spec icons, health, power, cast bars, trinkets and diminishing returns. Settings are independent of party cooldown layouts. Protected layout changes wait until combat ends.",
    reloadKeys={"gladiusBridgeX","gladiusBridgeY","gladiusEnabled","gladiusEngine"},
    defaults=G.defaults,limits={gladiusBridgeX={0,16000,integer=true},gladiusBridgeY={0,16000,integer=true},gladiusWidth={100,400},gladiusHealthHeight={20,60},gladiusPowerHeight={6,20},
        gladiusSpacing={0,100},gladiusScale={.5,2},gladiusOpacity={20,100},gladiusX={-10000,16000},gladiusY={-16000,10000},
        gladiusGrow={1,2,integer=true},gladiusFontSize={8,22},gladiusCastHeight={10,30},
        gladiusTrinketSize={20,70},gladiusDRSize={16,50},gladiusDRSide={1,2,integer=true},
        gladiusAuraSize={16,48},gladiusAuraMax={1,6,integer=true},gladiusAuraOpacity={20,100},
        gladiusCooldownSize={16,48},gladiusCooldownMax={1,8,integer=true},gladiusCooldownSide={1,2,integer=true}}}
local function slider(parent,title,key,y,min,max,step,x)
    local s=LUI:Slider(parent,title,key,y,min,max,step,x)
    s:HookScript("OnValueChanged",function()G:RefreshSettings()end)
end
local function auras(parent)
    LUI:Checkbox(parent,"Active CC over class icon","gladiusCC",0,0,"Replace the class icon with active crowd control and its remaining duration. WoW draws the aura directly; the engine is not needed.")
    LUI:Checkbox(parent,"Arena pets","gladiusPets",0,310,"Add a small pet health bar below the cast bar. Left-click targets the pet; right-click focuses it.")
    LUI:Checkbox(parent,"Important buffs / defensives","gladiusBuffs",-66,0,"Show WoW's important buffs, major personal defensives and external defensives above the opponent's frame, with native duration and stack counts.")
    LUI:Checkbox(parent,"Debuffs","gladiusDebuffs",-66,310,"Show harmful effects other than crowd control on a second row above buffs. Sated and similar exhaustion effects are omitted.")
    slider(parent,"Aura icon size","gladiusAuraSize",-140,16,48,1)
    slider(parent,"Maximum icons per aura group","gladiusAuraMax",-140,1,6,1,310)
    slider(parent,"Aura opacity","gladiusAuraOpacity",-214,20,100,5)
    LUI:Description(parent,"Active effects and cooldowns are separate: a defensive can be active while its cooldown is already counting down. CC uses the class-icon size; other auras use the size above.",4,-292,588)
    LUI:ResetButton(parent,"arena auras and pets",{"gladiusCC","gladiusPets","gladiusBuffs","gladiusDebuffs","gladiusAuraSize","gladiusAuraMax","gladiusAuraOpacity"},4,-334)
end
local function tracking(parent)
    LUI:Checkbox(parent,"Engine opponent tracking","gladiusEngine",0,0,"Read observed opponent casts through the optional event strip and attach cooldowns to these arena frames. Requires Lamda Engine, visible event strip and WoW in front for screen observations.")
    LUI:Checkbox(parent,"Interrupt cooldowns","gladiusInterrupts",-42,0,"Track observed interrupt casts, including attempts that miss. A cooldown does not prove a school lockout landed.")
    LUI:Checkbox(parent,"Defensive cooldowns","gladiusDefensives",-42,310,"Start an opponent's defensive cooldown when the cast is observed, while its active buff is still displayed.")
    LUI:Checkbox(parent,"Racial cooldowns","gladiusRacials",-84,0,"Track observed racial casts. Native arena trinkets stay in their own exact display.")
    LUI:Checkbox(parent,"Local cast announcements","gladiusAnnouncements",-84,310,"Briefly show the last observed tracked cast below that opponent's cooldowns. No chat messages are sent.")
    slider(parent,"Cooldown icon size","gladiusCooldownSize",-134,16,48,1)
    slider(parent,"Maximum cooldown icons","gladiusCooldownMax",-134,1,8,1,310)
    LUI:Choice(parent,"Position","gladiusCooldownSide",{"Below frame","Right of trinket / DR"},-192,0)
    LUI:Checkbox(parent,"Confirmed school lockouts","gladiusLockouts",-254,0,"Show a school-lockout estimate only when a successful interrupt log record matches this opponent's verified GUID. A kick cast alone cannot establish a lockout. Missing or restricted identity/log data leaves this blank.")
    LUI:Description(parent,"Cast times are observed; durations use baselines. Enemy talent changes and resets are unverified. A new arena round clears history.",4,-292,588)
    LUI:ResetButton(parent,"arena engine tracking",{"gladiusEngine","gladiusInterrupts","gladiusDefensives","gladiusRacials","gladiusCooldownSize","gladiusCooldownMax","gladiusCooldownSide","gladiusLockouts","gladiusAnnouncements"},4,-334)
end
local function message(ok,err) if not ok and err then print("LamdaGladius: "..err)end;LUI:RefreshUI()end
local function layout(parent)
    LUI:Description(parent,"The class icon sits to the left of each opponent, with health and power in the middle and their trinket on the right. Casts appear below; diminishing returns sit alongside.",4,0,596)
    LUI:Checkbox(parent,"Lock position","gladiusLocked",-66,0,"Unlock the header to move the whole arena group. Use Preview outside an arena to place it. Movement stops during combat.")
    LUI:Choice(parent,"Grow","gladiusGrow",{"Down","Up"},-66,310)
    slider(parent,"Health bar width","gladiusWidth",-120,100,400,5)
    slider(parent,"Health bar height","gladiusHealthHeight",-120,20,60,1,310)
    slider(parent,"Space between opponents","gladiusSpacing",-184,0,100,1)
    slider(parent,"Overall scale","gladiusScale",-184,.5,2,.05,310)
    slider(parent,"Opacity","gladiusOpacity",-248,20,100,5)
    slider(parent,"Text size","gladiusFontSize",-248,8,22,1,310)
    LUI:Button(parent,"Reset position",4,-316,150,function()LUI:ResetKeys({"gladiusX","gladiusY"});G:RefreshSettings(true)end)
    LUI:ResetButton(parent,"arena frame layout",{"gladiusLocked","gladiusWidth","gladiusHealthHeight","gladiusSpacing","gladiusScale","gladiusOpacity","gladiusGrow","gladiusFontSize","gladiusX","gladiusY"},170,-316)
end
local function bars(parent)
    LUI:Checkbox(parent,"Class icon","gladiusClassIcon",0,0,"Show the opponent's class icon beside the health bar.")
    LUI:Checkbox(parent,"Use spec icon","gladiusSpecIcon",0,310,"Use their specialization icon in place of the class icon when the arena provides a spec.")
    LUI:Checkbox(parent,"Health text","gladiusHealthText",-48,0,"Show health percentage when supported, or the client's health value. Restricted values are displayed by native widgets.")
    LUI:Checkbox(parent,"Power bar","gladiusPower",-48,310,"Show mana, energy, rage or the opponent's current power below their health.")
    LUI:Checkbox(parent,"Spec name in power bar","gladiusSpecText",-96,0,"Display specialization text over the power bar. Enable Power bar to see it.")
    slider(parent,"Power bar height","gladiusPowerHeight",-102,6,20,1,310)
    LUI:Checkbox(parent,"Cast bar","gladiusCast",-168,0,"Place WoW's arena cast bar below the opponent. Blizzard continues to handle cast, channel, empower and interruptibility state.")
    slider(parent,"Cast bar height","gladiusCastHeight",-174,10,30,1,310)
    LUI:Checkbox(parent,"Target highlight","gladiusTargetHighlight",-248,0,"A gold border marks your current target when the client exposes that match.")
    LUI:Checkbox(parent,"Focus highlight","gladiusFocusHighlight",-248,310,"A blue border marks your focus. Target color wins if both match.")
    LUI:ResetButton(parent,"arena bars and icons",{"gladiusClassIcon","gladiusSpecIcon","gladiusHealthText","gladiusPower","gladiusPowerHeight","gladiusSpecText","gladiusCast","gladiusCastHeight","gladiusTargetHighlight","gladiusFocusHighlight"},4,-318)
end
local function cooldowns(parent)
    LUI:Checkbox(parent,"Trinket / CC break","gladiusTrinket",0,0,"Show the arena crowd-control break spell and its cooldown from WoW. A question mark means the opponent's spell has not arrived yet; it is not a Ready claim.")
    slider(parent,"Trinket icon size","gladiusTrinketSize",-52,20,70,1)
    LUI:Description(parent,"The trinket stays on the right of the health bar. Its countdown comes from the arena API and does not need Lamda Engine.",314,-52,280)
    LUI:Checkbox(parent,"Diminishing returns","gladiusDR",-142,0,"Show WoW's arena diminishing-return icons and reset timers. This enables the game's enemy DR display while active and restores its previous setting when turned off. Categories follow WoW's DR settings.")
    slider(parent,"DR icon size","gladiusDRSize",-194,16,50,1)
    LUI:Choice(parent,"DR side","gladiusDRSide",{"Left","Right"},-194,310)
    LUI:Description(parent,"DR shows repeated crowd-control categories and their reset countdowns. Live icons appear when WoW reports a category. The preview uses labelled sample icons.",4,-272,588)
    LUI:ResetButton(parent,"arena trinkets and DR",{"gladiusTrinket","gladiusTrinketSize","gladiusDR","gladiusDRSize","gladiusDRSide"},4,-344)
end
local function preview(parent)
    LUI:Description(parent,"Place and size the classic arena layout before queueing. Sample opponents cannot be targeted. Real arenas and entering combat automatically stop the preview.",4,0,588)
    LUI:Button(parent,"Preview 2v2",4,-78,150,function()message(G:SetPreview(2))end)
    LUI:Button(parent,"Preview 3v3",166,-78,150,function()message(G:SetPreview(3))end)
    LUI:Button(parent,"Stop preview",328,-78,150,function()message(G:SetPreview(0))end)
    LUI:Checkbox(parent,"Lock position","gladiusLocked",-136,0,"Unlock to drag the LamdaGladius header. All opponents move together. Locking leaves target/focus clicks available in real arenas.")
    LUI:Description(parent,"Change Layout, Bars, or Trinkets & DR while the preview is visible. Each switch controls the same component in real arenas. Position and appearance are shared across your characters, separately from party-frame settings.",4,-194,588)
    LUI:Description(parent,"In a match: left-click a frame to target, right-click to focus. Your existing keyboard bindings and healing click profiles are unchanged.",4,-280,588)
end
module.build=function(parent)
    LUI:Tabs(parent,{{id="layout",name="Layout",build=layout},{id="bars",name="Bars",build=bars},
        {id="cooldowns",name="Trinkets & DR",build=cooldowns},{id="auras",name="Auras & pets",build=auras},
        {id="tracking",name="Tracking",build=tracking},{id="preview",name="Preview",build=preview}})
    local state=LUI:Description(parent,"",4,-418,600)
    LUI:OnRefresh(parent,function()state:SetText(G:Status())end)
    local reset=LUI:ResetButton(parent,"all LamdaGladius settings",function()LUI:ResetModule("gladius")end,4,-466)
    reset:SetText("Reset LamdaGladius")
end
LUI:RegisterModule(module)
