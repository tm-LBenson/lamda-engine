local _,LUI=...
local module={id="cd",name="LamdaCD",enabledKey="cdEnabled",
    defaults={cdEnabled=true,overlayX=60,overlayY=240,overlayScale=1,autoLog=true,companions=true,
        preview=false,anchor=1,grow=1,rowWidth=340,rowHeight=34,rowGap=3,fontSize=14,opacity=95,
        columns=1,maxRows=12,accent=1,showNames=true,showSpells=true,showTimers=true,border=true,bars=true},
    limits={overlayX={-16000,16000,integer=true},overlayY={-16000,16000,integer=true},overlayScale={0.5,3},
        anchor={1,9,integer=true},grow={1,2,integer=true},rowWidth={120,900,integer=true},rowHeight={20,100,integer=true},
        rowGap={0,40,integer=true},fontSize={8,32,integer=true},opacity={20,100,integer=true},columns={1,4,integer=true},
        maxRows={1,40,integer=true},accent={1,4,integer=true}}}
local function preview(parent)
    LUI:Label(parent,"Cooldown preview",310,0)
    local frame=CreateFrame("Frame",nil,parent);frame:SetPoint("TOPLEFT",310,-32);frame:SetSize(300,145)
    LUI.cdPreviews=LUI.cdPreviews or {};table.insert(LUI.cdPreviews,frame)
    if not LUI.inlinePreview then LUI.inlinePreview=frame end
end
local function appearance(parent)
    preview(parent)
    local presets={{"Compact",260,28,12,2},{"Standard",340,34,14,3},{"Large",420,44,18,5}}
    for i,preset in ipairs(presets) do
        LUI:Button(parent,preset[1],4+(i-1)*88,0,84,function()
            local db=LUI:DB();db.rowWidth=preset[2];db.rowHeight=preset[3];db.fontSize=preset[4];db.rowGap=preset[5];db.overlayScale=1
            LUI:RefreshUI()
        end)
    end
    LUI:Slider(parent,"Row width","rowWidth",-50,120,900,1)
    LUI:Slider(parent,"Row height","rowHeight",-104,20,100,1)
    LUI:Slider(parent,"Text size","fontSize",-158,8,32,1)
    LUI:Choice(parent,"Color","accent",{"Teal","Blue","Purple","Orange"},-216)
    LUI:Slider(parent,"Opacity","opacity",-268,20,100,5)
    LUI:Checkbox(parent,"Player names","showNames",-150,310)
    LUI:Checkbox(parent,"Spell names","showSpells",-184,310)
    LUI:Checkbox(parent,"Timers","showTimers",-218,310)
    LUI:Checkbox(parent,"Progress bars","bars",-252,310)
    LUI:Checkbox(parent,"Borders","border",-286,310)
end
local function placement(parent)
    preview(parent)
    local move=LUI:Button(parent,"Move & resize cooldowns",4,0,260,function()LUI:MoveCooldowns()end)
    parent:RegisterEvent("PLAYER_REGEN_DISABLED");parent:RegisterEvent("PLAYER_REGEN_ENABLED")
    local function refresh()move:SetEnabled(not InCombatLockdown())end
    parent:SetScript("OnEvent",refresh);LUI:OnRefresh(move,refresh)
    LUI:Label(parent,"Pin cooldowns to screen",4,-44)
    local anchors={"Top left","Top","Top right","Left","Center","Right","Bottom left","Bottom","Bottom right"}
    local buttons={}
    for i,label in ipairs(anchors) do
        buttons[i]=LUI:Button(parent,label,4+((i-1)%3)*88,-70-math.floor((i-1)/3)*30,84,function()
            local db=LUI:DB();db.anchor=i;db.overlayX=0;db.overlayY=0;LUI:RefreshUI()
        end)
    end
    LUI:OnRefresh(parent,function()for i,button in ipairs(buttons)do button:SetEnabled(i~=LUI:DB().anchor)end end)
    LUI:Slider(parent,"Overall scale","overlayScale",-180,0.5,3,0.05)
    LUI:Choice(parent,"Columns","columns",{"1","2","3","4"},-240)
    LUI:Choice(parent,"Grow","grow",{"Down","Up"},-282)
    LUI:Slider(parent,"Space between cooldowns","rowGap",-180,0,40,1,310)
    LUI:Slider(parent,"Maximum shown","maxRows",-250,1,40,1,310)
end
local function tracking(parent)
    LUI:Checkbox(parent,"Include companions","companions",0)
    LUI:Checkbox(parent,"Show preview in game","preview",-42)
    LUI:Checkbox(parent,"Enable combat logging in dungeons & delves","autoLog",-84)
    local reset
    reset=LUI:Button(parent,"Reset LamdaCD settings",4,-282,260,function()
        if reset.confirming then reset.confirming=false;LUI:ResetModule("cd")
        else reset.confirming=true;reset:SetText("Confirm reset") end
    end)
    LUI:OnRefresh(reset,function()reset.confirming=false;reset:SetText("Reset LamdaCD settings")end)
end
module.build=function(parent)
    LUI:Tabs(parent,{{id="appearance",name="Appearance",build=appearance},
        {id="placement",name="Placement",build=placement},{id="tracking",name="Tracking",build=tracking}})
end
LUI:RegisterModule(module)
