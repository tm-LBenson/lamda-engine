local _,LUI=...
local module={id="cd",name="LamdaCD",enabledKey="cdEnabled",
    defaults={cdEnabled=true,nativeFrames=true,
        nativeProvider=1,nativePlayer=true,nativeParty=true,nativeRaid=true,nativePets=false,
        nativeCC=true,nativeDebuffs=true,nativeDefensives=true,nativeBuffs=true,
        nativeMaxCC=3,nativeMaxDebuffs=4,nativeMaxDefensives=3,nativeMaxBuffs=2,
        nativeAnchor=6,nativeGap=6,nativeOffsetX=0,nativeOffsetY=0,
        nativeSize=26,nativeSpacing=2,nativePerRow=6,nativeFontSize=12,
        nativeTimers=true,nativeStacks=true,nativeBorders=true,nativeGlow=false,nativeSwipe=true,nativeReverse=false,
        nativeWorld=true,nativeDungeon=true,nativeRaidContext=true,nativeArena=true,nativeBattleground=true,nativeDelve=true,
        -- Preserve existing profile values and the engine's configuration contract.
        -- The frame module supersedes the detached display when nativeFrames is true.
        overlayX=60,overlayY=240,overlayScale=1,autoLog=true,companions=true,
        preview=false,anchor=1,grow=1,rowWidth=340,rowHeight=34,rowGap=3,fontSize=14,opacity=95,
        columns=1,maxRows=12,accent=1,showNames=true,showSpells=true,showTimers=true,border=true,bars=true},
    limits={nativeProvider={1,3,integer=true},nativeMaxCC={1,12,integer=true},nativeMaxDebuffs={1,12,integer=true},
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
local regionNames={CC="Crowd control",Debuffs="Debuffs",Defensives="Defensive buffs",Buffs="Important buffs"}
local regionDefaults={
    CC={Party={32,6,2,1},Raid={20,5,2,1}},
    Debuffs={Party={24,8,2,1},Raid={18,8,2,1}},
    Defensives={Party={30,5,0,1},Raid={25,5,0,1}},
    Buffs={Party={26,2,2,1},Raid={20,2,2,1}},
}
local layoutLimits={Anchor={1,9},Gap={0,40},OffsetX={-200,200},OffsetY={-200,200},
    Size={12,64},Spacing={0,16},PerRow={1,12},FontSize={8,28},Growth={1,8}}
for _,region in ipairs(regions)do
    for _,context in ipairs({"Party","Raid"})do
        local value=regionDefaults[region][context];local prefix="native"..region..context
        local settings={Size=value[1],Anchor=value[2],Gap=value[3],Growth=value[4],
            OffsetX=0,OffsetY=0,Spacing=2,PerRow=6,FontSize=context=="Raid" and 10 or 12,
            Timers=true,Stacks=true,Borders=true,Glow=false,Swipe=true,Reverse=false,Tooltips=true}
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
    LUI:Dropdown(parent,4,0,260,function()
        local names={};for _,region in ipairs(regions)do names[#names+1]=regionNames[region]end;return names
    end,function()return regionNames[LUI.frameLayoutEdit.region]end,function(name)
        for _,region in ipairs(regions)do if regionNames[region]==name then LUI.frameLayoutEdit.region=region;break end end
    end)
    LUI:Dropdown(parent,314,0,260,function()return {"Party & player frames","Raid frames"}end,
        function()return LUI.frameLayoutEdit.context=="Raid" and "Raid frames" or "Party & player frames"end,
        function(name)LUI.frameLayoutEdit.context=name=="Raid frames" and "Raid" or "Party" end)
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
local function auras(parent)
    LUI:Checkbox(parent,"Crowd control","nativeCC",0)
    LUI:Slider(parent,"Maximum CC icons","nativeMaxCC",-42,1,12,1)
    LUI:Checkbox(parent,"Debuffs","nativeDebuffs",-122)
    LUI:Slider(parent,"Maximum debuff icons","nativeMaxDebuffs",-164,1,12,1)
    LUI:Checkbox(parent,"Defensive buffs","nativeDefensives",0,310)
    LUI:Slider(parent,"Maximum per defensive group","nativeMaxDefensives",-42,1,12,1,310)
    LUI:Checkbox(parent,"Important buffs","nativeBuffs",-122,310)
    LUI:Slider(parent,"Maximum buff icons","nativeMaxBuffs",-164,1,12,1,310)
    previewButton(parent,4,-270)
end
local function frames(parent)
    layoutSelectors(parent)
    LUI:Choice(parent,"Growth",layoutKey("Growth"),
        {"Right / down","Left / down","Right / up","Left / up","Down / right","Up / right","Down / left","Up / left"},-48)
    LUI:Slider(parent,"Icon size",layoutKey("Size"),-98,12,64,1)
    LUI:Slider(parent,"Space between icons",layoutKey("Spacing"),-156,0,16,1)
    LUI:Slider(parent,"Icons per line",layoutKey("PerRow"),-214,1,12,1)
    previewButton(parent,4,-290)
    local positions={"Top left","Top","Top right","Left","Center","Right","Bottom left","Bottom","Bottom right"}
    local buttons={}
    for i,label in ipairs(positions) do
        local anchor=i
        buttons[i]=LUI:Button(parent,label,314+((i-1)%3)*88,-48-math.floor((i-1)/3)*30,84,function()
            local db=LUI:DB();local prefix=layoutPrefix()
            db[prefix.."Anchor"]=anchor;db[prefix.."OffsetX"]=0;db[prefix.."OffsetY"]=0;LUI:RefreshUI()
        end)
    end
    LUI:OnRefresh(parent,function()
        for i,button in ipairs(buttons) do button:SetEnabled(i~=LUI:DB()[layoutPrefix().."Anchor"]) end
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
    previewButton(parent,314,-290)
end
local function content(parent)
    LUI:Checkbox(parent,"Open world","nativeWorld",0)
    LUI:Checkbox(parent,"Dungeons & follower dungeons","nativeDungeon",-44)
    LUI:Checkbox(parent,"Raids","nativeRaidContext",-88)
    LUI:Checkbox(parent,"Arenas","nativeArena",0,310)
    LUI:Checkbox(parent,"Battlegrounds","nativeBattleground",-44,310)
    LUI:Checkbox(parent,"Delves","nativeDelve",-88,310)
    LUI:Choice(parent,"Unit frames","nativeProvider",{"Automatic","DandersFrames","Blizzard"},-142)
    LUI:Checkbox(parent,"Player frame","nativePlayer",-194)
    LUI:Checkbox(parent,"Party frames","nativeParty",-230)
    LUI:Checkbox(parent,"Raid frames","nativeRaid",-194,310)
    LUI:Checkbox(parent,"Pet frames","nativePets",-230,310)
    local reset
    reset=LUI:Button(parent,"Reset LamdaCD settings",4,-270,260,function()
        if reset.confirming then reset.confirming=false;LUI:ResetModule("cd")
        else reset.confirming=true;reset:SetText("Confirm reset") end
    end)
    LUI:OnRefresh(reset,function()reset.confirming=false;reset:SetText("Reset LamdaCD settings")end)
end
module.build=function(parent)
    LUI:Tabs(parent,{{id="auras",name="Auras",build=auras},{id="frames",name="Frames",build=frames},
        {id="appearance",name="Appearance",build=appearance},{id="content",name="Content",build=content}})
end
LUI:RegisterModule(module)
