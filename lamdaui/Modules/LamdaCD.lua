local _,LUI=...
local S=LUI.SpellLists
local module={id="cd",name="LamdaCD",enabledKey="cdEnabled",
    reloadKeys={"cdEnabled","nativeFrames","engineTracking","companions","publicLookups","lookupInterval",
        "lookupBridge","lookupBridgeX","lookupBridgeY","engineFrameEstimates","engineFrameSpells"},
    defaults={cdEnabled=false,defensiveCatalogVersion=2,nativeFrames=true,engineTracking=true,inspectTalents=true,liveCasts=true,auraStarts=true,publicLookups=true,lookupInterval=1,lookupBridge=true,lookupBridgeX=0,lookupBridgeY=0,
        engineFrameEstimates=true,engineFrameSpells=table.concat(S.cooldowns,","),engineFrameUnknown=true,engineFramePersistent=true,engineFrameMatchSelf=true,
        engineFrameSize=32,engineFrameSpacing=2,engineFramePerRow=6,engineFrameMax=6,engineFrameOpacity=95,
        engineFrameAnchor=2,engineFrameGap=20,engineFrameX=0,engineFrameY=0,engineFrameGrowth=1,
        nativeDefensivesSpells=table.concat(S.defensives,","),nativeBuffsSpells=table.concat(S.buffs,","),
        nativeDefensivesGameDefaults=false,nativeBuffsGameDefaults=false,nativeDebuffFilter=1,nativeHideSated=true,
        selfCDSpells=table.concat(S.cooldowns,","),selfCDOpacity=100,
        selfCooldowns=true,selfCDTimers=true,selfCDCharges=true,selfCDSize=26,selfCDMax=6,selfCDPerRow=6,
        selfCDSpacing=2,selfCDAnchor=1,selfCDGap=8,selfCDX=0,selfCDY=0,selfCDFrame=1,selfCDGrowth=1,selfCDLayoutVersion=1,
        estimatePanel=false,estimateSelf=true,estimateParty=true,estimateReady=false,estimateRows=8,estimateSize=2,estimateX=60,estimateY=200,
        nativeProvider=1,nativePlayer=true,nativeParty=true,nativeRaid=true,nativePets=false,
        nativeCC=true,nativeDebuffs=true,nativeDefensives=true,nativeBuffs=true,
        nativeMaxCC=3,nativeMaxDebuffs=4,nativeMaxDefensives=3,nativeMaxBuffs=2,
        nativeAnchor=6,nativeGap=6,nativeOffsetX=0,nativeOffsetY=0,
        nativeSize=26,nativeSpacing=2,nativePerRow=6,nativeFontSize=12,
        nativeTimers=true,nativeStacks=true,nativeBorders=true,nativeGlow=false,nativeSwipe=true,nativeReverse=false,
        nativeWorld=true,nativeDungeon=true,nativeRaidContext=true,nativeArena=true,nativeBattleground=true,nativeDelve=true,
        -- Preserve existing profile values and the engine's configuration contract.
        -- Detached display keys remain importable for older profiles only.
        overlayX=60,overlayY=240,overlayScale=1,autoLog=true,companions=true,
        preview=false,anchor=1,grow=1,rowWidth=340,rowHeight=34,rowGap=3,fontSize=14,opacity=95,
        columns=1,maxRows=12,accent=1,showNames=true,showSpells=true,showTimers=true,border=true,bars=true},
    limits={defensiveCatalogVersion={1,2,integer=true},lookupInterval={1,5,integer=true},lookupBridgeX={0,16000,integer=true},lookupBridgeY={0,16000,integer=true},nativeProvider={1,3,integer=true},nativeMaxCC={1,12,integer=true},nativeMaxDebuffs={1,12,integer=true},
        engineFrameSpells={validate=S.Valid},engineFrameSize={12,64,integer=true},engineFrameSpacing={0,16,integer=true},
        engineFramePerRow={1,12,integer=true},engineFrameMax={1,12,integer=true},engineFrameOpacity={10,100,integer=true},
        engineFrameAnchor={1,4,integer=true},engineFrameGap={0,40,integer=true},engineFrameX={-200,200,integer=true},engineFrameY={-200,200,integer=true},engineFrameGrowth={1,3,integer=true},
        nativeDefensivesSpells={validate=S.Valid},nativeBuffsSpells={validate=S.Valid},selfCDSpells={validate=S.Valid},
        nativeDebuffFilter={1,3,integer=true},selfCDOpacity={10,100,integer=true},
        selfCDSize={12,64,integer=true},selfCDMax={1,12,integer=true},selfCDPerRow={1,12,integer=true},
        selfCDSpacing={0,16,integer=true},selfCDAnchor={1,4,integer=true},selfCDGap={0,40,integer=true},
        selfCDX={-200,200,integer=true},selfCDY={-200,200,integer=true},selfCDFrame={1,4,integer=true},selfCDGrowth={1,3,integer=true},selfCDLayoutVersion={1,2,integer=true},
        estimateRows={1,12,integer=true},estimateSize={1,3,integer=true},estimateX={0,16000,integer=true},estimateY={0,16000,integer=true},
        nativeMaxDefensives={1,12,integer=true},nativeMaxBuffs={1,12,integer=true},nativeAnchor={1,9,integer=true},
        nativeGap={0,40,integer=true},nativeOffsetX={-200,200,integer=true},nativeOffsetY={-200,200,integer=true},
        nativeSize={12,64,integer=true},nativeSpacing={0,16,integer=true},nativePerRow={1,12,integer=true},nativeFontSize={8,28,integer=true},
        overlayX={-16000,16000,integer=true},overlayY={-16000,16000,integer=true},overlayScale={0.5,3},
        anchor={1,9,integer=true},grow={1,2,integer=true},rowWidth={120,900,integer=true},rowHeight={20,100,integer=true},
        rowGap={0,40,integer=true},fontSize={8,32,integer=true},opacity={20,100,integer=true},columns={1,4,integer=true},
        maxRows={1,40,integer=true},accent={1,4,integer=true}}}
-- Each effect region has its own party and raid layout. These remain flat,
-- validated profile values; selecting an editor is transient UI state only.
local regions={"CC","Debuffs","Defensives","Buffs"}
local regionNames={CC="Crowd control",Debuffs="Debuffs",Defensives="Active defensives",Buffs="Buffs"}
local regionDefaults={
    CC={Party={32,6,2,1},Raid={20,5,2,1}},
    Debuffs={Party={24,8,2,1},Raid={18,8,2,1}},
    Defensives={Party={30,5,0,1},Raid={25,5,0,1}},
    Buffs={Party={26,2,2,1},Raid={20,2,2,1}},
}
local layoutLimits={Opacity={10,100},Anchor={1,9},Gap={0,40},OffsetX={-200,200},OffsetY={-200,200},
    Size={12,64},Spacing={0,16},PerRow={1,12},FontSize={8,28},Growth={1,8}}
for _,region in ipairs(regions)do
    for _,context in ipairs({"Party","Raid"})do
        local value=regionDefaults[region][context];local prefix="native"..region..context
        local settings={Size=value[1],Anchor=value[2],Gap=value[3],Growth=value[4],
            OffsetX=0,OffsetY=0,Spacing=2,PerRow=6,FontSize=context=="Raid" and 10 or 12,
            Opacity=100,Timers=true,Stacks=true,Borders=true,Glow=false,Swipe=true,Reverse=false,Tooltips=true}
        for field,default in pairs(settings)do
            module.defaults[prefix..field]=default
            if layoutLimits[field] then
                module.limits[prefix..field]={layoutLimits[field][1],layoutLimits[field][2],integer=true}
            end
        end
    end
end
LUI.frameLayoutEdit={region="CC",context="Party"}
local function layoutPrefix()
    local edit=LUI.frameLayoutEdit
    return "native"..edit.region..edit.context
end
local function layoutKey(field)return function()return layoutPrefix()..field end end
local function layoutSelectors(parent)
    LUI:Dropdown(parent,4,0,260,function()return {"Party & player frames","Raid frames"}end,
        function()return LUI.frameLayoutEdit.context=="Raid" and "Raid frames" or "Party & player frames"end,
        function(name)LUI.frameLayoutEdit.context=name=="Raid frames" and "Raid" or "Party" end)
    local scope=LUI:Description(parent,"",314,0,282)
    LUI:OnRefresh(scope,function()scope:SetText(regionNames[LUI.frameLayoutEdit.region].." only. Other groups keep their positions.")end)
end
local function previewButton(parent,x,y)
    local button=LUI:Button(parent,"Preview on frames",x,y,260,function()
        if InCombatLockdown() then return end
        local auras=LUI.FrameAuras
        if auras and auras.SetPreview then auras:SetPreview(not auras.preview);LUI:RefreshUI() end
    end)
    LUI:OnRefresh(button,function()
        local auras=LUI.FrameAuras
        button:SetEnabled(auras and type(auras.SetPreview)=="function" and not InCombatLockdown())
        button:SetText(auras and auras.preview and "Stop preview" or "Preview on frames")
    end)
    return button
end
local function groupSelector(parent)
    LUI:Dropdown(parent,4,0,260,function()
        local names={};for _,region in ipairs(regions)do names[#names+1]=regionNames[region]end;return names
    end,function()return regionNames[LUI.frameLayoutEdit.region]end,function(name)
        for _,region in ipairs(regions)do if regionNames[region]==name then LUI.frameLayoutEdit.region=region;break end end
    end)
end
local descriptions={CC="Crowd control is an effect that limits a unit: stuns, fears, roots and similar effects.",
    Debuffs="Harmful effects currently on a unit. Choose how much to show; long lockouts can be hidden.",
    Defensives="Defensive buffs currently active on a unit. Their timers show effect duration, not ability recharge.",
    Buffs="Helpful effects currently active on a unit. Lamda's editable starter list includes major offensive and group buffs."}
local maxima={CC="nativeMaxCC",Debuffs="nativeMaxDebuffs",Defensives="nativeMaxDefensives",Buffs="nativeMaxBuffs"}
local function auras(parent)
    local description=LUI:Description(parent,"",314,0,282)
    LUI:OnRefresh(description,function()description:SetText(descriptions[LUI.frameLayoutEdit.region])end)
    LUI:Checkbox(parent,"Show this group",function()return "native"..LUI.frameLayoutEdit.region end,0,0,
        "Show active effects in this group on enabled unit frames. Use Cooldowns for time until your abilities recharge.")
    LUI:Slider(parent,"Maximum icons",function()return maxima[LUI.frameLayoutEdit.region]end,-46,1,12,1)
    local helpful=CreateFrame("Frame",nil,parent);helpful:SetAllPoints(parent)
    local useGame=LUI:Checkbox(helpful,"Use Blizzard's category",function()return "native"..LUI.frameLayoutEdit.region.."GameDefaults"end,-76,310,
        "Use all effects in Blizzard's category instead of the editable list below. Turn this off to use only your selected spells. Your custom list is kept when switching.")
    local picker=LUI:SpellPicker(helpful,function()return "native"..LUI.frameLayoutEdit.region.."Spells"end,0,-120)
    local automatic=LUI:Description(helpful,"Blizzard chooses this category. Turn off the switch above to use and edit your tracked list.",4,-156,570)
    LUI:OnRefresh(helpful,function()
        local region=LUI.frameLayoutEdit.region;local active=region=="Defensives" or region=="Buffs"
        helpful:SetShown(active)
        local auto=active and LUI:DB()["native"..region.."GameDefaults"]
        picker:SetShown(active and not auto);automatic:SetShown(active and auto)
    end)
    local harmful=CreateFrame("Frame",nil,parent);harmful:SetAllPoints(parent)
    local note=LUI:Description(harmful,"",4,-130,575)
    local debuffs=CreateFrame("Frame",nil,harmful);debuffs:SetAllPoints(harmful)
    LUI:Choice(debuffs,"Show","nativeDebuffFilter",{"Important","Dispellable","All"},-86,310)
    LUI:Checkbox(debuffs,"Hide Sated and similar lockouts","nativeHideSated",-250,0,
        "Hide Sated, Exhaustion and Temporal Displacement with WoW's permitted non-secret spell filter. This does not hide important encounter debuffs.")
    LUI:OnRefresh(harmful,function()
        local region=LUI.frameLayoutEdit.region;harmful:SetShown(region=="CC" or region=="Debuffs");debuffs:SetShown(region=="Debuffs")
        note:SetText(region=="CC" and "Tracked: the game's crowd-control category. WoW restricts individual spell selection for CC on friendly frames. You can enable this group and customize its position and style." or
            "Tracked: the selected debuff category. WoW restricts individual encounter-debuff selection on friendly frames. The Sated filter below is a supported exception.")
    end)
    LUI:ResetButton(parent,"this effect group's selection",function()
        local region=LUI.frameLayoutEdit.region
        local keys={"native"..region,maxima[region],"native"..region.."Spells","native"..region.."GameDefaults"}
        if region=="Debuffs"then keys[#keys+1]="nativeDebuffFilter";keys[#keys+1]="nativeHideSated"end
        LUI:ResetKeys(keys)
    end,4,-370)
end
local function frames(parent)
    layoutSelectors(parent)
    LUI:Choice(parent,"Growth",layoutKey("Growth"),
        {"Right / down","Left / down","Right / up","Left / up","Down / right","Up / right","Down / left","Up / left"},-48)
    LUI:Slider(parent,"Icon size",layoutKey("Size"),-98,12,64,1)
    LUI:Slider(parent,"Space between icons",layoutKey("Spacing"),-156,0,16,1)
    LUI:Slider(parent,"Icons per line",layoutKey("PerRow"),-214,1,12,1)
    LUI:Description(parent,"Position the selected effect group around its unit frame. Party and raid layouts are separate.",4,-314,590)
    LUI:ResetButton(parent,"this group and frame layout",function()
        local keys={};for _,field in ipairs({"Growth","Size","Spacing","PerRow","Anchor","Gap","OffsetX","OffsetY"})do keys[#keys+1]=layoutPrefix()..field end;LUI:ResetKeys(keys)
    end,4,-370)
    local positions={"Top left","Top","Top right","Left","Center","Right","Bottom left","Bottom","Bottom right"}
    local buttons={}
    for i,label in ipairs(positions) do
        local anchor=i
        buttons[i]=LUI:Navigation(parent,label,314+((i-1)%3)*88,-48-math.floor((i-1)/3)*30,84,function()
            local db=LUI:DB();local prefix=layoutPrefix()
            db[prefix.."Anchor"]=anchor;db[prefix.."OffsetX"]=0;db[prefix.."OffsetY"]=0;LUI:RefreshUI()
        end)
    end
    LUI:OnRefresh(parent,function()
        for i,button in ipairs(buttons) do button:SetSelected(i==LUI:DB()[layoutPrefix().."Anchor"]) end
    end)
    LUI:Slider(parent,"Distance from frame",layoutKey("Gap"),-152,0,40,1,310)
    LUI:Slider(parent,"Horizontal adjustment",layoutKey("OffsetX"),-210,-200,200,1,310)
    LUI:Slider(parent,"Vertical adjustment",layoutKey("OffsetY"),-268,-200,200,1,310)
end
local function appearance(parent)
    layoutSelectors(parent)
    local presets={{"Compact",20,1,6,10},{"Standard",26,2,6,12},{"Large",34,3,5,15}}
    for i,preset in ipairs(presets) do
        local values=preset
        LUI:Button(parent,values[1],4+(i-1)*88,-42,84,function()
            local db=LUI:DB();local prefix=layoutPrefix()
            db[prefix.."Size"]=values[2];db[prefix.."Spacing"]=values[3]
            db[prefix.."PerRow"]=values[4];db[prefix.."FontSize"]=values[5];LUI:RefreshUI()
        end)
    end
    LUI:Choice(parent,"Growth",layoutKey("Growth"),
        {"Right / down","Left / down","Right / up","Left / up","Down / right","Up / right","Down / left","Up / left"},-82)
    LUI:Slider(parent,"Icon size",layoutKey("Size"),-122,12,64,1)
    LUI:Slider(parent,"Space between icons",layoutKey("Spacing"),-170,0,16,1)
    LUI:Slider(parent,"Icons per line",layoutKey("PerRow"),-218,1,12,1)
    LUI:Slider(parent,"Text size",layoutKey("FontSize"),-266,8,28,1)
    LUI:Checkbox(parent,"Timers",layoutKey("Timers"),-42,310)
    LUI:Checkbox(parent,"Stack counts",layoutKey("Stacks"),-76,310)
    LUI:Checkbox(parent,"Colored borders",layoutKey("Borders"),-110,310)
    LUI:Checkbox(parent,"Highlight",layoutKey("Glow"),-144,310)
    LUI:Checkbox(parent,"Cooldown swipe",layoutKey("Swipe"),-178,310)
    LUI:Checkbox(parent,"Reverse swipe",layoutKey("Reverse"),-212,310)
    LUI:Checkbox(parent,"Tooltips",layoutKey("Tooltips"),-246,310)
    LUI:Slider(parent,"Opacity (%)",layoutKey("Opacity"),-316,10,100,1)
    LUI:ResetButton(parent,"this group and frame style",function()
        local keys={};for _,field in ipairs({"Growth","Size","Spacing","PerRow","FontSize","Opacity","Timers","Stacks","Borders","Glow","Swipe","Reverse","Tooltips"})do keys[#keys+1]=layoutPrefix()..field end;LUI:ResetKeys(keys)
    end,4,-370)
end
local function content(parent)
    LUI:Checkbox(parent,"Open world","nativeWorld",0)
    LUI:Checkbox(parent,"Dungeons & follower dungeons","nativeDungeon",-44)
    LUI:Checkbox(parent,"Raids","nativeRaidContext",-88)
    LUI:Checkbox(parent,"Arenas","nativeArena",0,310)
    LUI:Checkbox(parent,"Battlegrounds","nativeBattleground",-44,310)
    LUI:Checkbox(parent,"Delves","nativeDelve",-88,310)
    LUI:Description(parent,"Visible unit frames are detected automatically. Choose where these groups should appear.",4,-146,585)
    LUI:Checkbox(parent,"Player frame","nativePlayer",-194)
    LUI:Checkbox(parent,"Party frames","nativeParty",-230)
    LUI:Checkbox(parent,"Raid frames","nativeRaid",-194,310)
    LUI:Checkbox(parent,"Pet frames","nativePets",-230,310)
    LUI:ResetButton(parent,"visibility settings",{"nativeWorld","nativeDungeon","nativeRaidContext","nativeArena","nativeBattleground","nativeDelve","nativePlayer","nativeParty","nativeRaid","nativePets"},4,-312)
    local reset=LUI:ResetButton(parent,"all LamdaCD settings in this profile",function()LUI:ResetModule("cd")end,4,-370)
    reset:SetText("Reset LamdaCD settings");reset:SetWidth(240)
end
local function tracking(parent)
    LUI:Checkbox(parent,"Estimate cooldowns with Lamda Engine","engineTracking",0)
    LUI:Checkbox(parent,"Automatically log instances & arenas","autoLog",-48)
    LUI:Checkbox(parent,"Observe supported companions","companions",-96)
    LUI:Checkbox(parent,"Web fallback","publicLookups",-144,0,"Use Raider.IO when a party build is unavailable locally. Your own character and inspected builds are skipped; recent web results are reused. Apply & Reload after changing.")
    LUI:Slider(parent,"Seconds between requests","lookupInterval",-198,1,5,1)
    LUI:Checkbox(parent,"Inspect party talents","inspectTalents",0,310,
        "Choose scan timing under General → Tracking. Inspect nearby party members outside combat. Prefer inspected talents over public profiles.")
    LUI:Checkbox(parent,"Use readable live casts","liveCasts",-48,310,
        "Send readable player and party casts to Lamda Engine. Combat logs provide a fallback for restricted events.")
    LUI:Checkbox(parent,"Start estimates from active buffs","auraStarts",-96,310,
        "Observe selected defensive buffs through the small band beside this module's strip. These remain cooldown estimates until a cast confirms them.")
    LUI:Description(parent,"Talent lookups refresh when you join a party or start a key. Public profiles can lag behind in-game changes. Keep Lamda Engine running.",314,-152,280)
    LUI:ResetButton(parent,"engine tracking settings",{"engineTracking","inspectTalents","liveCasts","auraStarts","autoLog","companions","publicLookups","lookupInterval"},4,-350)
end
local function engineFrameLayout(parent)
    LUI:Checkbox(parent,"Engine estimates on unit frames","engineFrameEstimates",0)
    LUI:Checkbox(parent,"My estimates","estimateSelf",-38,0,
        "Use engine estimates when a native cooldown is unavailable. With your native cooldown row enabled, estimates use the same learned-spell slots and position; working native timers take priority. Your public profile cannot add another row or unselected spells. Turn this off to keep only teammates' estimates. Active buff durations stay separate. Applies live without reloading.")
    LUI:Checkbox(parent,"Party / raid estimates","estimateParty",-76,0,
        "Show engine estimates for other group members. Your own estimates have their own switch above, even when your frame is included in the party layout.")
    LUI:Checkbox(parent,"Keep defensive icons visible","engineFramePersistent",-114,0,
        "Keep spell icons when no timer is running. Available builds can identify selected abilities. If a teammate's build cannot be verified, their recorded specialization supplies possible abilities as dim question-mark icons; they may not have learned or selected them. A question mark never means ready. Observed abilities take priority over suggestions. Turn this off for observation-only display. Applies live without reloading.")
    LUI:Checkbox(parent,"Show unknown cooldown states","engineFrameUnknown",-152,0,
        "Show dim question-mark icons for unverified abilities or unknown cooldowns, including missing casts, late logs, resource-based abilities and uncertain charges. Turning this off also hides specialization suggestions. An estimated timer does not guarantee charges or usability. Applies live without reloading.")
    LUI:Slider(parent,"Icon size","engineFrameSize",-200,12,64,1)
    LUI:Slider(parent,"Maximum per frame","engineFrameMax",-252,1,12,1)
    LUI:Slider(parent,"Icons per line","engineFramePerRow",-304,1,12,1)
    LUI:Choice(parent,"Placement","engineFrameAnchor",{"Below","Above","Left","Right"},0,310)
    LUI:Slider(parent,"Distance from frame","engineFrameGap",-52,0,40,1,310)
    LUI:Slider(parent,"Horizontal adjustment","engineFrameX",-104,-200,200,1,310)
    LUI:Slider(parent,"Vertical adjustment","engineFrameY",-156,-200,200,1,310)
    LUI:Slider(parent,"Space between icons","engineFrameSpacing",-208,0,16,1,310)
    LUI:Slider(parent,"Opacity","engineFrameOpacity",-260,10,100,1,310)
    LUI:Checkbox(parent,"Match my cooldown layout","engineFrameMatchSelf",-310,310,
        "Teammates use Cooldowns > Position for size, placement, growth, spacing, offsets and capacity, plus its opacity. Turn this off to use the custom engine layout on this page. With your native cooldown row enabled, your own fallbacks always replace its missing timers in the same slots. Applies live without reloading.")
    local growth=LUI:Choice(parent,"Growth","engineFrameGrowth",{"Automatic","Left to right","Right to left"},-350)
    LUI:Explain(growth,"Custom engine growth","Automatic fills left-side rows from right to left, starting beside the frame. Other placements fill left to right. Wrapped rows start at the same edge. When matching is enabled, teammates use the growth setting under Cooldowns > Position instead.")
    LUI:Description(parent,"Matching teammates use Cooldowns > Position. Controls here set the custom engine layout.",314,-350,280)
    LUI:ResetButton(parent,"engine frame layout",{"engineFrameEstimates","estimateSelf","estimateParty","estimateReady","engineFrameUnknown","engineFramePersistent","engineFrameMatchSelf","engineFrameSize","engineFrameSpacing","engineFramePerRow","engineFrameMax","engineFrameOpacity","engineFrameAnchor","engineFrameGap","engineFrameX","engineFrameY","engineFrameGrowth"},4,-380)
end
local function engineSettings(parent)
    parent:SetScript("OnHide",function()if LUI.LookupEvents then LUI.LookupEvents:SetMoving(false)end end)
    LUI.enginePage=LUI.enginePage or "Tracking & sources"
    local names={"Tracking & sources","Estimates on frames","Tracked engine spells"};local pages={}
    for _,name in ipairs(names)do
        local body=CreateFrame("Frame",nil,parent);body:SetPoint("TOPLEFT",parent,"TOPLEFT",0,-46);body:SetSize(620,410);pages[name]=body
        if name=="Estimates on frames" then engineFrameLayout(body)
        elseif name=="Tracked engine spells" then
            LUI:Description(body,"Choose fallback cooldowns for you and your group. A working native cooldown takes priority on each frame. Estimates require an observation and engine rule. Apply & Reload saves this list for the engine.",4,0,590)
            LUI:SpellPicker(body,"engineFrameSpells",4,-62)
            LUI:ResetButton(body,"engine tracked spells",{"engineFrameSpells"},4,-370)
        else tracking(body)end
        body:SetShown(name==LUI.enginePage)
    end
    LUI:Dropdown(parent,4,0,280,function()return names end,function()return LUI.enginePage end,
        function(value)LUI.enginePage=value;for name,page in pairs(pages)do page:SetShown(name==value)end end)
end
local function cooldownLayout(parent)
    LUI:Description(parent,"Position your cooldown row. Teammates share this layout when Match my cooldown layout is on.",4,0,275)
    local choice=LUI:Choice(parent,"My frame","selfCDFrame",{"Automatic","Custom / group","Blizzard player","All my frames"},-60)
    LUI:Explain(choice,"My cooldown frame","Automatic uses one visible custom or group frame, falling back to the Blizzard player frame. Custom / group excludes the Blizzard player frame. All my frames allows duplicate rows. This choice also applies to your engine estimates; teammate cooldowns and active effects keep their own visibility settings.")
    LUI:Help(parent,choice.helpTitle,choice.helpText,270,-60)
    LUI:Slider(parent,"Icon size","selfCDSize",-112,12,64,1)
    LUI:Slider(parent,"Maximum icons","selfCDMax",-164,1,12,1)
    LUI:Slider(parent,"Icons per line","selfCDPerRow",-216,1,12,1)
    LUI:Slider(parent,"Space between icons","selfCDSpacing",-268,0,16,1)
    LUI:Choice(parent,"Placement","selfCDAnchor",{"Below","Above","Left","Right"},0,310)
    LUI:Slider(parent,"Distance from frame","selfCDGap",-50,0,40,1,310)
    LUI:Slider(parent,"Horizontal adjustment","selfCDX",-108,-200,200,1,310)
    LUI:Slider(parent,"Vertical adjustment","selfCDY",-166,-200,200,1,310)
    local growth=LUI:Choice(parent,"Growth","selfCDGrowth",{"Automatic","Left to right","Right to left"},-232,310)
    LUI:Explain(growth,"Cooldown growth","Automatic starts left-side rows beside the frame and grows outward to the left. Other placements grow left to right. Wrapped rows start at the same edge. Teammates share this setting when Match my cooldown layout is enabled.")
    LUI:Description(parent,"Automatic: left-side icons grow outward to the left. Teammates share this growth when matching is on.",314,-276,282)
    LUI:ResetButton(parent,"your cooldown layout",{"selfCDSize","selfCDMax","selfCDPerRow","selfCDSpacing","selfCDAnchor","selfCDGap","selfCDX","selfCDY","selfCDFrame","selfCDGrowth","selfCDLayoutVersion"},4,-370)
end

local function subpages(parent,definitions,x,width)
    local pages,buttons={},{};x=x or 4;width=width or 194
    local function select(index)
        if LUI.openMenu then LUI.openMenu:Hide()end
        for i,page in ipairs(pages)do page:SetShown(i==index);buttons[i]:SetSelected(i==index)end
        parent.selectedSection=definitions[index].name;LUI:RefreshUI()
    end
    for index,definition in ipairs(definitions)do
        local page=CreateFrame("Frame",nil,parent);page:SetPoint("TOPLEFT",0,-44);page:SetSize(620,408)
        page:Hide()
        pages[index]=page;definition.build(page)
        buttons[index]=LUI:Navigation(parent,definition.name,x+(index-1)*(width+4),0,width,function()select(index)end)
    end
    parent.sectionPages=pages;parent.selectSection=select;select(1)
end
local function effects(parent)
    groupSelector(parent)
    subpages(parent,{{name="Tracked spells",build=auras},{name="Position",build=frames},{name="Style",build=appearance}},280,103)
end
local function cooldownStyle(parent)
    LUI:Checkbox(parent,"Countdown numbers","selfCDTimers",0)
    LUI:Checkbox(parent,"Charge counts","selfCDCharges",-44)
    LUI:Slider(parent,"Opacity (%)","selfCDOpacity",-100,10,100,1)
    LUI:Description(parent,"Style for your own cooldown icons. Active defensive buffs and engine estimates have separate settings.",314,0,282)
    LUI:ResetButton(parent,"your cooldown style",{"selfCDTimers","selfCDCharges","selfCDOpacity"},4,-370)
end
local function cooldowns(parent)
    subpages(parent,{{name="Tracked spells",build=function(list)
        LUI:Checkbox(list,"My defensive cooldowns","selfCooldowns",0,0,
            "Show WoW's cooldowns for your selected learned abilities. When a native timer works, Lamda hides its duplicate engine estimate on that frame. If the timer is unavailable and this spell is also selected under Engine, that display takes over when it has an observation. Enable My estimates and the live strip for automatic fallback. Without an engine fallback, unavailable native data shows a question mark. Active defensive buff durations are independent.")
        LUI:Description(list,"WoW's cooldown takes priority. Selected engine estimates fill gaps when native data is unavailable. Use My build to find learned abilities.",314,0,282)
        LUI:SpellPicker(list,"selfCDSpells",0,-86)
        LUI:ResetButton(list,"your cooldown spell list",{"selfCDSpells","selfCooldowns"},4,-370)
    end},{name="Position",build=cooldownLayout},{name="Style",build=cooldownStyle}})
end
local function previews(parent)
    LUI:Description(parent,"Choose which groups to preview. These switches only affect sample icons; your real tracking settings are kept.",4,0,592)
    local names={CC="Crowd control",Debuffs="Debuffs",Defensives="Active defensives",Buffs="Buffs",Cooldowns="My cooldowns"}
    for index,id in ipairs({"CC","Debuffs","Defensives","Buffs","Cooldowns"})do
        local group=id;local button=CreateFrame("CheckButton",nil,parent,"UICheckButtonTemplate")
        button:SetSize(26,26);button:SetPoint("TOPLEFT",0,-58-(index-1)*40)
        local label=button:CreateFontString(nil,"OVERLAY","GameFontHighlight");label:SetPoint("LEFT",button,"RIGHT",4,0);label:SetText(names[id])
        button:SetHitRectInsets(0,-225,0,0)
        LUI:OnRefresh(button,function()
            local F=LUI.FrameAuras;button:SetChecked(not F or not F.PreviewEnabled or F:PreviewEnabled(group));button:SetEnabled(not InCombatLockdown())
        end)
        button:SetScript("OnClick",function(self)
            local F=LUI.FrameAuras;if F and F.SetPreviewGroup then F:SetPreviewGroup(group,self:GetChecked())end
        end)
        LUI:Explain(button,"Preview "..names[id],"Show or hide this sample group while previewing. Does not change which real effects are tracked.")
    end
    LUI:Description(parent,"Each effect group has its own Position and Style submenu. All selected previews stay at full capacity so you can check overlap.",314,-68,285)
    local reset=LUI:Button(parent,"Preview all groups",4,-292,260,function()
        local F=LUI.FrameAuras;if F and not InCombatLockdown()then F.previewGroups={};F:SetPreview(true)end
    end)
    LUI:OnRefresh(reset,function()reset:SetEnabled(not InCombatLockdown())end)
    previewButton(parent,314,-292)
end

module.build=function(parent)
    LUI:Tabs(parent,{{id="auras",name="Effects",build=effects},
        {id="cooldowns",name="Cooldowns",build=cooldowns},{id="content",name="Visibility",build=content},
        {id="tracking",name="Engine",build=engineSettings},
        {id="preview",name="Preview",build=previews}})
end
LUI:RegisterModule(module)
