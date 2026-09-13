local _,LUI=...
LUI:RegisterModule({id="cd",name="LamdaCD",build=function(parent)
    local display=CreateFrame("Frame",nil,parent);display:SetAllPoints()
    local advanced=CreateFrame("Frame",nil,parent);advanced:SetAllPoints();advanced:Hide()
    local function label(p,text,x,y,large)
        local l=p:CreateFontString(nil,"OVERLAY",large and "GameFontNormalLarge" or "GameFontHighlight")
        l:SetPoint("TOPLEFT",x,y);l:SetText(text);return l
    end
    label(display,"Team cooldown display",4,0,true)
    LUI:Checkbox(display,"Show teammate cooldowns","cdEnabled",-28)
    LUI:Checkbox(display,"Include companions","companions",-64)
    local move=LUI:Button(display,"Move & resize",4,-108,260,function()LUI:MoveCooldowns()end)
    display:RegisterEvent("PLAYER_REGEN_DISABLED");display:RegisterEvent("PLAYER_REGEN_ENABLED")
    local function refresh()move:SetEnabled(not InCombatLockdown());LUI:RefreshCooldownPreview()end
    display:SetScript("OnEvent",refresh);display:SetScript("OnShow",refresh)
    local presets={
        {"Compact",260,28,12,2},{"Standard",340,34,14,3},{"Large",420,44,18,5},
    }
    for i,preset in ipairs(presets)do
        LUI:Button(display,preset[1],4+(i-1)*88,-152,84,function()
            local d=LUI:DB();d.rowWidth=preset[2];d.rowHeight=preset[3];d.fontSize=preset[4];d.rowGap=preset[5];d.overlayScale=1
            display:Hide();display:Show();LUI:RefreshCooldownPreview()
        end)
    end
    LUI:Slider(display,"Row width","rowWidth",-204,120,900,1)
    LUI:Slider(display,"Row height","rowHeight",-258,20,100,1)
    LUI:Slider(display,"Text size","fontSize",-312,8,32,1)
    label(display,"Preview",310,0)
    local sample=CreateFrame("Frame",nil,display);sample:SetPoint("TOPLEFT",310,-36);sample:SetSize(300,145)
    sample.sampleRows={};LUI.inlinePreview=sample
    LUI:Slider(display,"Space between cooldowns","rowGap",-204,0,40,1,310)
    LUI:Slider(display,"Opacity","opacity",-258,20,100,5,310)
    LUI:Button(display,"More options",310,-322,260,function()display:Hide();advanced:Show()end)

    label(advanced,"Cooldown display options",4,0,true)
    LUI:Button(advanced,"Back",510,0,90,function()advanced:Hide();display:Show()end)
    label(advanced,"Pin display to",4,-40)
    local anchors={"Top left","Top","Top right","Left","Center","Right","Bottom left","Bottom","Bottom right"}
    local anchorButtons={}
    local function anchorRefresh()for i,b in ipairs(anchorButtons)do b:SetEnabled(i~=LUI:DB().anchor)end end
    for i,name in ipairs(anchors)do
        anchorButtons[i]=LUI:Button(advanced,name,4+((i-1)%3)*88,-66-math.floor((i-1)/3)*30,84,function()
            local d=LUI:DB();d.anchor=i;d.overlayX=0;d.overlayY=0;anchorRefresh();LUI:RefreshCooldownPreview()
        end)
    end
    advanced:SetScript("OnShow",anchorRefresh)
    LUI:Choice(advanced,"Color","accent",{"Teal","Blue","Purple","Orange"},-180)
    LUI:Choice(advanced,"Grow","grow",{"Down","Up"},-222)
    LUI:Choice(advanced,"Columns","columns",{"1","2","3","4"},-264)
    LUI:Slider(advanced,"Maximum shown","maxRows",-310,1,40,1)
    LUI:Checkbox(advanced,"Player names","showNames",-44,310)
    LUI:Checkbox(advanced,"Spell names","showSpells",-80,310)
    LUI:Checkbox(advanced,"Timers","showTimers",-116,310)
    LUI:Checkbox(advanced,"Progress bars","bars",-152,310)
    LUI:Checkbox(advanced,"Borders","border",-188,310)
    LUI:Checkbox(advanced,"Show samples in game","preview",-234,310)
    LUI:Slider(advanced,"Overall scale","overlayScale",-300,0.5,3,0.05,310)
    LUI:RefreshCooldownPreview()
end})
