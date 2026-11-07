local _,LUI=...
local module={id="kick",name="LamdaKick",enabledKey="kickEnabled",
    enabledHelp="Your interrupt uses WoW's actual cooldown. Teammates use Lamda Engine: Calibrating means watching for a cast or waiting one normal cooldown, then Ready or a countdown. Teammate timers assume no missed use.",
    reloadKeys={"kickBridgeX","kickBridgeY","kickEnabled","kickSpells","castCaptureMode"},
    defaults={kickBridgeX=224,kickBridgeY=0,kickSpells=table.concat(LUI.SpellLists.interrupts,","),kickEnabled=false,kickMode=2,kickSelf=true,kickSolo=false,kickLocked=true,
        kickWorld=true,kickDungeon=true,kickArena=true,kickLiveCasts=true,kickScreenCasts=true,kickInspectTalents=true,kickAutoLog=true,castCaptureMode=1,
        kickX=420,kickY=260,kickWidth=240,kickHeight=28,kickOpacity=95,kickCounts=false,
        kickSize=26,kickAnchor=4,kickGap=4,kickOffsetX=0,kickOffsetY=0,kickSort=1},
    limits={kickBridgeX={0,16000,integer=true},kickBridgeY={0,16000,integer=true},castCaptureMode={1,2,integer=true},kickSpells={validate=LUI.SpellLists.Valid},kickMode={1,3,integer=true},kickX={0,16000,integer=true},kickY={0,16000,integer=true},
        kickWidth={160,400,integer=true},kickHeight={20,48,integer=true},kickOpacity={20,100,integer=true},
        kickSize={16,64,integer=true},kickAnchor={1,4,integer=true},kickGap={0,80,integer=true},
        kickOffsetX={-200,200,integer=true},kickOffsetY={-200,200,integer=true},kickSort={1,2,integer=true}}}
local function lockControl(parent,x,y)
    local lock=LUI:Checkbox(parent,"Locked","kickLocked",y,x,
        "Uncheck to drag the tracker or frame icons. Preview unlocks them. Closing settings locks them; combat also locks them when Allow changes during combat is off.")
    LUI:OnRefresh(lock,function()lock:SetEnabled(LUI:DB().kickEnabled and not LUI:CombatManagementBlocked())end)
    return lock
end
local function previewControl(parent,x,y)
    local preview=LUI:Button(parent,"Preview",x,y,180,function()
        LUI.Kick:SetPreview(not LUI.Kick.preview);LUI:RefreshUI()
    end)
    LUI:OnRefresh(preview,function()
        preview:SetText(LUI.Kick.preview and "Stop preview" or "Preview")
        preview:SetEnabled(LUI:DB().kickEnabled and not LUI:CombatManagementBlocked())
    end)
    LUI:Help(parent,"Preview","Shows sample timers and unlocks dragging. Check Locked to hold the position while previewing.",x+192,y)
end
local function display(parent)
    LUI:Choice(parent,"Display","kickMode",{"Frame icons","Classic bars","Both"},0)
    LUI:Help(parent,"Display","Frame icons follow teammates' frames. Classic bars use a separate tracker. Both shows each layout.",270,0)
    LUI:Checkbox(parent,"Include my interrupt","kickSelf",-44,0,"Show your actual interrupt cooldown from WoW. Your native row stays first; teammate rows follow the selected order.")
    LUI:Choice(parent,"Bar order","kickSort",{"Party order","Ready first"},-90)
    LUI:Help(parent,"Bar order","Your own native row stays first. Ready first sorts teammates who are ready ahead of cooldowns; Calibrating entries come last.",270,-90)
    LUI:Slider(parent,"Opacity","kickOpacity",-140,20,100,5)
    previewControl(parent,4,-220)
    lockControl(parent,310,-220)
end
local function bars(parent)
    LUI:Slider(parent,"Bar width","kickWidth",0,160,400,10)
    LUI:Slider(parent,"Bar height","kickHeight",0,20,48,1,310)
    LUI:Checkbox(parent,"Show kick success / attempts","kickCounts",-82,0,"Show landed kicks / observed attempts to the left of each bar: 3/4 means three landed and one had no confirmed interrupt. Each cast counts once, including kicks that hit multiple enemies. Delayed interrupt records can update the result later. Counts reset for a new instance or key and when the engine restarts; starting mid-run gives a partial count. Applies live.")
    previewControl(parent,4,-138);lockControl(parent,310,-138)
    LUI:Button(parent,"Reset position",4,-186,160,function()LUI.Kick:SetMoving(false);LUI:ResetKeys({"kickX","kickY"})end)
    LUI:ResetButton(parent,"LamdaKick bar layout",{"kickX","kickY","kickWidth","kickHeight","kickSort","kickCounts"},4,-266)
end
local function icons(parent)
    LUI:Choice(parent,"Anchor","kickAnchor",{"Below","Above","Left","Right"},0)
    LUI:Help(parent,"Frame icons","Drag any unlocked icon to adjust the shared offset. C means Calibrating for a teammate; your own icon uses WoW's actual cooldown.",270,0)
    LUI:Slider(parent,"Icon size","kickSize",-62,16,64,1)
    LUI:Slider(parent,"Gap from frame","kickGap",-62,0,80,1,310)
    LUI:Slider(parent,"Horizontal offset","kickOffsetX",-150,-200,200,1)
    LUI:Slider(parent,"Vertical offset","kickOffsetY",-150,-200,200,1,310)
    previewControl(parent,4,-230);lockControl(parent,310,-230)
    LUI:ResetButton(parent,"LamdaKick frame icons",{"kickAnchor","kickSize","kickGap","kickOffsetX","kickOffsetY"},4,-292)
end
local function visibility(parent)
    LUI:Checkbox(parent,"World / outdoors","kickWorld",0,0,"Show in a five-player party outdoors. Raid groups, raid instances and battlegrounds are hidden.")
    LUI:Checkbox(parent,"Dungeons / delves","kickDungeon",-44,0,"Show in dungeons, delves and scenarios.")
    LUI:Checkbox(parent,"Arenas","kickArena",-88,0,"Show friendly team interrupts in arenas. PvP cooldown changes may affect estimates.")
    LUI:Checkbox(parent,"Show while solo","kickSolo",-132,0,"Show your own interrupt while ungrouped. Include my interrupt must also be enabled.")
    LUI:ResetButton(parent,"LamdaKick visibility",{"kickSelf","kickSolo","kickWorld","kickDungeon","kickArena"},4,-210)
end
local function tracking(parent)
    LUI:Checkbox(parent,"Read cast events","kickLiveCasts",0,0,"Use readable interrupt casts. Restricted events use the combat-log fallback. An interrupt cast starts its cooldown even when it misses.")
    LUI:Checkbox(parent,"Inspect party talents","kickInspectTalents",-48,0,"Inspect outside combat for selected interrupts and supported cooldown changes. Choose scan timing under General → Tracking. Known abilities show Ready after a full cooldown spent in the same party, assuming no missed use. A detected cast starts the countdown immediately.")
    LUI:Checkbox(parent,"Enable combat logging","kickAutoLog",-96,0,"Enable instance logging for the engine fallback. Logs may arrive late. Turning this off leaves logging started by other addons running.")
    if not LUI.ForeverEngine then
        LUI:Checkbox(parent,"Read casts from screen","kickScreenCasts",-144,0,"Adds a cast-data strip beside the movable engine event strip. Colors use a 128 by 12 pixel band; Text uses the older number panel. The engine must confirm it can read the selected format. Turning this off leaves readable events and delayed combat logs.")
        LUI:Choice(parent,"Capture format","castCaptureMode",{"Colors (experimental)","Text (fallback)"},-196)
        LUI:Help(parent,"Capture format","Colors have no visible numbers and use native formatting. This needs live validation on your client. Text retains the older capture method. This setting also applies to LamdaGladius. Reload after changing formats.",270,-196)
        local test=LUI:Button(parent,"Test cast colors",4,-244,180,function()LUI.Observations:TestCastCapture()end)
        LUI:OnRefresh(test,function()
            local db=LUI:DB();test:SetEnabled(db.kickEnabled and db.kickScreenCasts and db.castCaptureMode~=2 and not IsInGroup() and not InCombatLockdown())
        end)
        LUI:Help(parent,"Test cast colors","While solo, show fixed test colors for 60 seconds in the capture strip. No casts are sent or counted. Keep WoW in front while the rendering is checked. Joining a party or entering combat ends the test.",200,-244)
    end
    LUI:ResetButton(parent,"LamdaKick tracking",{"kickLiveCasts","kickScreenCasts","castCaptureMode","kickInspectTalents","kickAutoLog"},4,-306)
end
local function spells(parent)
    local picker=LUI:SpellPicker(parent,"kickSpells",0,0);picker.notice:Hide()
    LUI:ResetButton(parent,"LamdaKick interrupt selection",{"kickSpells"},4,-294)
    LUI:Help(parent,"Tracked interrupts","Starter interrupts have engine coverage. Other added spells need an engine rule before a cooldown can be shown.",166,-294)
end
module.build=function(parent)
    LUI:Tabs(parent,{{id="display",name="Display",build=display},{id="bars",name="Bars",build=bars},
        {id="icons",name="Icons",build=icons},{id="visibility",name="Visibility",build=visibility},
        {id="spells",name="Spells",build=spells},{id="tracking",name="Tracking",build=tracking}})
    local reset=LUI:ResetButton(parent,"all LamdaKick settings",function()LUI:ResetModule("kick")end,4,-426)
    reset:SetText("Reset LamdaKick")
end
LUI:RegisterModule(module)
