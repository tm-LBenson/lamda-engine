local _,LUI=...
LUI:RegisterModule({id="cd",name="LamdaCD",build=function(parent)
    local sections={
        {name="Tracking",build=function(p)
            LUI:Checkbox(p,"Enabled","cdEnabled",0)
            LUI:Checkbox(p,"Companions","companions",-36)
            LUI:Checkbox(p,"Preview","preview",-72)
        end},
        {name="Layout",build=function(p)
            LUI:Choice(p,"Anchor","anchor",{"Top left","Top","Top right","Left","Center","Right","Bottom left","Bottom","Bottom right"},0)
            LUI:Number(p,"Offset X","overlayX",-42,-16000,16000)
            LUI:Number(p,"Offset Y","overlayY",-84,-16000,16000)
            LUI:Number(p,"Scale","overlayScale",-126,0.5,3)
            LUI:Number(p,"Columns","columns",-168,1,4)
            LUI:Choice(p,"Grow","grow",{"Down","Up"},0,310)
            LUI:Number(p,"Width","rowWidth",-42,120,900,310)
            LUI:Number(p,"Height","rowHeight",-84,20,100,310)
            LUI:Number(p,"Spacing","rowGap",-126,0,40,310)
            LUI:Number(p,"Max rows","maxRows",-168,1,40,310)
            LUI:Button(p,"Reset layout",4,-220,145,function()
                if LUI.commitInputs then LUI.commitInputs() end
                local db=LUI:DB()
                for k,v in pairs({anchor=1,grow=1,overlayX=60,overlayY=240,overlayScale=1,columns=1,rowWidth=340,rowHeight=34,rowGap=3,maxRows=12}) do db[k]=v end
                p:Hide();p:Show()
            end)
        end},
        {name="Style",build=function(p)
            LUI:Number(p,"Font size","fontSize",0,8,32)
            LUI:Number(p,"Opacity (%)","opacity",-42,20,100)
            LUI:Choice(p,"Color","accent",{"Teal","Blue","Purple","Orange"},-84)
            LUI:Checkbox(p,"Border","border",-130)
            LUI:Checkbox(p,"Player name","showNames",0,310)
            LUI:Checkbox(p,"Spell name","showSpells",-36,310)
            LUI:Checkbox(p,"Timer","showTimers",-72,310)
            LUI:Checkbox(p,"Progress bars","bars",-108,310)
        end},
    }
    local panels,buttons={},{}
    local function select(index)
        for i,p in ipairs(panels) do p:SetShown(i==index);buttons[i]:SetEnabled(i~=index) end
    end
    for i,s in ipairs(sections) do
        local p=CreateFrame("Frame",nil,parent);p:SetPoint("TOPLEFT",0,-42);p:SetSize(620,310);p:Hide()
        panels[i]=p;s.build(p)
        buttons[i]=LUI:Button(parent,s.name,(i-1)*108,0,102,function()select(i)end)
    end
    select(1)
end})
