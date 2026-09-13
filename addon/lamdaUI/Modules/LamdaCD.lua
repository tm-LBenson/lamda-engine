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
    LUI:Choice(parent,"Unit frames","nativeProvider",{"Automatic","DandersFrames","Blizzard"},0)
    LUI:Checkbox(parent,"Player frame","nativePlayer",-54)
    LUI:Checkbox(parent,"Party frames","nativeParty",-92)
    LUI:Checkbox(parent,"Raid frames","nativeRaid",-130)
    LUI:Checkbox(parent,"Pet frames","nativePets",-168)
    previewButton(parent,4,-270)
    LUI:Label(parent,"Attach icons to each frame",314,0)
    local positions={"Top left","Top","Top right","Left","Center","Right","Bottom left","Bottom","Bottom right"}
    local buttons={}
    for i,label in ipairs(positions) do
        local anchor=i
        buttons[i]=LUI:Button(parent,label,314+((i-1)%3)*88,-28-math.floor((i-1)/3)*30,84,function()
            local db=LUI:DB();db.nativeAnchor=anchor;db.nativeOffsetX=0;db.nativeOffsetY=0;LUI:RefreshUI()
        end)
    end
    LUI:OnRefresh(parent,function()
        for i,button in ipairs(buttons) do button:SetEnabled(i~=LUI:DB().nativeAnchor) end
    end)
    LUI:Slider(parent,"Distance from frame","nativeGap",-136,0,40,1,310)
    LUI:Slider(parent,"Horizontal adjustment","nativeOffsetX",-198,-200,200,1,310)
    LUI:Slider(parent,"Vertical adjustment","nativeOffsetY",-260,-200,200,1,310)
end
local function appearance(parent)
    local presets={{"Compact",20,1,6,10},{"Standard",26,2,6,12},{"Large",34,3,5,15}}
    for i,preset in ipairs(presets) do
        local values=preset
        LUI:Button(parent,values[1],4+(i-1)*88,0,84,function()
            local db=LUI:DB();db.nativeSize=values[2];db.nativeSpacing=values[3]
            db.nativePerRow=values[4];db.nativeFontSize=values[5];LUI:RefreshUI()
        end)
    end
    LUI:Slider(parent,"Icon size","nativeSize",-54,12,64,1)
    LUI:Slider(parent,"Space between icons","nativeSpacing",-112,0,16,1)
    LUI:Slider(parent,"Icons per row","nativePerRow",-170,1,12,1)
    LUI:Slider(parent,"Text size","nativeFontSize",-228,8,28,1)
    LUI:Checkbox(parent,"Timers","nativeTimers",0,310)
    LUI:Checkbox(parent,"Stack counts","nativeStacks",-36,310)
    LUI:Checkbox(parent,"Colored borders","nativeBorders",-72,310)
    LUI:Checkbox(parent,"Glow","nativeGlow",-108,310)
    LUI:Checkbox(parent,"Cooldown swipe","nativeSwipe",-144,310)
    LUI:Checkbox(parent,"Reverse swipe","nativeReverse",-180,310)
    previewButton(parent,314,-270)
end
local function content(parent)
    LUI:Checkbox(parent,"Open world","nativeWorld",0)
    LUI:Checkbox(parent,"Dungeons & follower dungeons","nativeDungeon",-44)
    LUI:Checkbox(parent,"Raids","nativeRaidContext",-88)
    LUI:Checkbox(parent,"Arenas","nativeArena",0,310)
    LUI:Checkbox(parent,"Battlegrounds","nativeBattleground",-44,310)
    LUI:Checkbox(parent,"Delves","nativeDelve",-88,310)
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
