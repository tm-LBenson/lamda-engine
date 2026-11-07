local _,LUI=...
local A=LUI.Augmentation
local module={id="augmentation",name="LamdaMeters",enabledKey="augEnabled",reloadKeys={"meterOverlay","augLiveHybrid","augDebug","augEnabled","augShowMeter","augMeterScope","augX","augY","augUIWidth","augUIHeight"},
    enabledHelp="Damage, healing, utility and combat breakdowns. Augmentation attribution uses Lamda Engine.",
    defaults={meterReportHistoryLimit=100,meterCaptureBorder=2,meterCaptureDock=1,meterOverlay=false,meterShareDpsRankings=true,meterShareDamageMVP=true,meterShareBossUptime=true,meterCaptureDockBottom=true,meterNativeWindows=true,meterCaptureEnabled=false,meterAutoCurrent=true,meterAutoOverall=true,augEnabled=false,augLiveHybrid=true,augPrivateSummary=true,augGroupSummary=false,augAfterBoss=false,augAfterKey=true,
        augmentationBridgeX=288,augmentationBridgeY=320,augMeterWidth=380,augMeterHeight=220,augMeterLocked=false,
        augNativeScope="boss",augNativeMetric=-1,augCaptureMode=1,augCaptureStripWidth=800,augCaptureBorder=8,augCaptureOverflow=1,augCaptureTopWeight=0,augCaptureRightWeight=0,augCaptureBottomWeight=1,augCaptureLeftWeight=0,
        augDebug=true,augShowMeter=true,augAutoLog=true,augX=20,augY=300,augUIWidth=1920,augUIHeight=1080,augMeterScope=1},
    limits={meterReportHistoryLimit={values={25,50,75,100,125,150,175,200}},meterCaptureBorder={2,16,integer=true},augCaptureMode={values={1,2}},meterCaptureDock={values={1,2,3,4,5}},augCaptureStripWidth={208,1920,integer=true},augNativeMetric={values={-1,0,2}},augMeterWidth={300,1000,integer=true},augMeterHeight={120,1000,integer=true},augCaptureBorder={8,64,integer=true},augCaptureOverflow={values={1,2,3,4,5,6}},augCaptureTopWeight={0,8,integer=true},augCaptureRightWeight={0,8,integer=true},augCaptureBottomWeight={0,8,integer=true},augCaptureLeftWeight={0,8,integer=true},augmentationBridgeX={0,16000,integer=true},augmentationBridgeY={0,16000,integer=true},augX={0,16000,integer=true},augY={0,16000,integer=true},augUIWidth={1,16000,integer=true},augUIHeight={1,16000,integer=true},augMeterScope={values={1,2}}}}
local function border(parent)
    if LUI.MeterOverlay and LUI.MeterOverlay:Enabled()then
        LUI:Checkbox(parent,"Enable Engine capture","meterCaptureEnabled",0,0,"Pause or resume the live meter feed. Report history is kept.")
        LUI.MeterOverlay:BuildStripControls(parent,4,-60)
        return
    end
    local capture=LUI:Checkbox(parent,"Enable Engine capture","meterCaptureEnabled",0,0,"Show the capture border or strip while color strips are Shown. Hidden pauses capture. The native WoW meter stays available without it.")
    capture:SetEnabled(not(LUI.MeterBorder and LUI.MeterBorder.ForcePaused))
    LUI:Choice(parent,"Capture layout","augCaptureMode",{"Border","Standalone strip"},-46,0)
    LUI:Slider(parent,"Minimum strip width","augCaptureStripWidth",-112,208,1920,8,0)
    LUI:Checkbox(parent,"Dock strip at bottom","meterCaptureDockBottom",-174,0,"Keep the strip against the bottom edge. It widens for a raid to stay thin. Turn off to position it manually.")
    LUI:Slider(parent,"Border thickness","augCaptureBorder",-252,8,32,2,0)
    LUI:Choice(parent,"Border overflow","augCaptureOverflow",{"Bottom","Top","Left","Right","Shared equally","Custom side weights"},-318,0)
    LUI:Slider(parent,"Top weight","augCaptureTopWeight",-384,0,8,1,0)
    LUI:Slider(parent,"Right weight","augCaptureRightWeight",-384,0,8,1,308)
    LUI:Slider(parent,"Bottom weight","augCaptureBottomWeight",-446,0,8,1,0)
    LUI:Slider(parent,"Left weight","augCaptureLeftWeight",-446,0,8,1,308)
end
local function damage(parent)
    LUI:Description(parent,"LamdaMeters works for every specialization. Open a metric menu for damage, DPS, healing, HPS, absorbs, interrupts, dispels, damage taken, avoidable damage, deaths or enemy damage taken.",4,0,588)
    LUI:Checkbox(parent,"Switch to Current when combat starts","meterAutoCurrent",-76,0,"Select Current once when you enter combat. Turn off to keep your selected session.")
    LUI:Checkbox(parent,"Switch to Overall when a key finishes","meterAutoOverall",-122,0,"Select Overall when a Mythic+ key is completed. Ordinary combat endings and abandoned keys do not switch it.")
    LUI:Button(parent,"Show combat meters",4,-176,196,function()if LUI.Meters then LUI.Meters:Command("show")end end)
    LUI:Button(parent,"New healing window",210,-176,196,function()if LUI.Meters then LUI.Meters:AddWindow("HealingDone")end end)
    LUI:Description(parent,"Drag a window's title bar normally. Lock/Unlock controls its position; the bottom-right corner resizes it. The ... menu has new windows, appearance, profiles, value formats and copyable reports. Changes apply immediately.",4,-224,588)
    LUI:Description(parent,"Click a player for spells, then a spell for targets. Right-click a row to go back. Death rows open Blizzard's recap when one is available. Use the session menu for current, previous, overall or a recorded fight. Scroll the bars with the mouse wheel.",4,-316,588)
    LUI:Description(parent,"/lmeters shows the windows; /lmeters help lists commands. WoW owns this combat data. Restricted details stay unavailable, and resetting its history also resets other meters using the same API.",4,-414,588)
end
local function meter(parent)
    LUI:Description(parent,"Track Ebon Might uptime, average observed Prescience targets, your casts and deaths. Aura restrictions and sampling gaps reduce coverage; unavailable data is never scored as zero.",4,0,588)
    LUI:Checkbox(parent,"Show Engine meter window","augShowMeter",-76,0,"Live damage/DPS and healing/HPS for every specialization. Recorded support is credited to each Aug in the party. Drag the title normally; use its menu for metric, session and locking.")
    LUI:Choice(parent,"Default engine session","augMeterScope",{"Current combat","Recorded whole key"},-128,0)
    LUI:Checkbox(parent,"Live hybrid attribution","augLiveHybrid",-166,0,"Project your contribution only over damage dealt under your captured buffs, then replace projections with recorded support. Turn off for recorded attribution only. Save and reload to change this mode.")
    local state=LUI:Description(parent,"",4,-190,588)
    local casts=LUI:Label(parent,"",4,-238)
    local extra=LUI:Description(parent,"",4,-276,588)
    A.views[#A.views+1]=function(s,line,castLine)
        if not parent:IsVisible()then return end
        state:SetText(not A:Eligible() and "Switch to an Augmentation Evoker to collect." or (not A.enabled and "Collection is off." or line))
        casts:SetText(castLine)
        local seconds=s and (s.seconds or GetTime()-s.started) or 0
        extra:SetText(s and string.format("Prescience: %s | Deaths: %d\nWhole boss/key duration: %.0fs. Includes downtime.",
            s.prescienceKnown>0 and string.format("%.2f average targets (%.0f%% observed)",s.prescienceTargets/s.prescienceKnown,math.min(100,100*s.prescienceKnown/math.max(1,seconds))) or "unavailable",s.deaths,seconds) or "Finish a boss or key to save a private report.")
    end
    LUI:Button(parent,"Print latest",4,-388,140,function()A:Print()end)
    LUI:Button(parent,"Share latest",154,-388,140,function()A:Share()end)
    LUI:Description(parent,"/laug opens this module. /laug summary prints privately. /laug share previews the Aug-adjusted group DPS rankings. /lmeters controls your native combat windows. The live view is provisional until support records arrive.",4,-430,588)
end
local function chat(parent)
    LUI:Checkbox(parent,"Print private summary","augPrivateSummary",0,0,"Print only in your own chat after the selected boss/key boundary.")
    LUI:Checkbox(parent,"Prepare group summary after fights","augGroupSummary",-46,0,"Open the adjusted DPS leaderboard after selected boundaries. Review it and click Send to group. Off by default. Private summaries can stay enabled.")
    LUI:Checkbox(parent,"After each boss attempt","augAfterBoss",-114,0,"Include kills and wipes. During a key this can also print boss reports if enabled.")
    LUI:Checkbox(parent,"After a completed Mythic+ key","augAfterKey",-160,0,"Summarize the whole key including travel and downtime. Reset/abandoned keys do not trigger a completion message.")
    LUI:Checkbox(parent,"Include Ebon Might uptime on boss shares","meterShareBossUptime",-206,0,"Add your recorded Ebon Might uptime to boss reports when the matching log review is available. Key reports omit it.")
    LUI:Checkbox(parent,"Include DPS rankings","meterShareDpsRankings",-252,0,"Share up to five DPS rankings. Turn off for an awards-only recap. Healing and utility meter rankings keep their own metric.")
    LUI:Checkbox(parent,"Include Damage MVP award","meterShareDamageMVP",-298,0,"Include the top recorded damage award independently of the numbered DPS rankings.")
    LUI:Slider(parent,"Saved report limit","meterReportHistoryLimit",-366,25,200,25,0)
    LUI:Description(parent,"History keeps compact rankings and awards on Lamda Engine. Oldest recaps roll off at this limit; spell breakdowns and combat events are not copied. The History picker also has retention and boss/key filters.",4,-430,588)
    LUI:Button(parent,"Open report history",4,-500,196,function()if LUI.Meters and LUI.Meters.ShowReportHistory then LUI.Meters:ShowReportHistory()end end)
end
local function logs(parent)
 LUI:Checkbox(parent,"Detailed comparison debug log","augDebug",-408,0,"Write a local ledger of normal damage, your support transfers, skipped events, and meter output. Records link to the original WoW log with timestamps and line hashes. On for testing; bounded to 256 MiB. Save and reload to change this setting.")
    LUI:Checkbox(parent,"Enable combat logging in instances","augAutoLog",0,0,"Start combat logging in dungeons, raids and scenarios for every Retail specialization while LamdaMeters is enabled. Keeps logs available for party Aug support. Never stops another module's logging or changes advanced logging settings.")
    LUI:Label(parent,"Log review",4,-64)
    local url="http://127.0.0.1:47641/augmentation"
    local link=CreateFrame("EditBox",nil,parent,"InputBoxTemplate");link:SetSize(450,28);link:SetPoint("TOPLEFT",100,-60);link:SetAutoFocus(false);link:SetText(url)
    link:SetScript("OnEditFocusGained",function(self)self:HighlightText()end)
    link:SetScript("OnMouseUp",function(self)self:HighlightText()end)
    link:SetScript("OnTextChanged",function(self,userInput)if userInput then self:SetText(url);self:HighlightText()end end)
    link:SetScript("OnEscapePressed",function(self)self:ClearFocus()end)
    link:SetScript("OnEnterPressed",function(self)self:ClearFocus()end)
    link:SetScript("OnHide",function(self)self:ClearFocus()end)
    LUI:Description(parent,"Click the field, Ctrl+C, and open it in Chrome for updating reports. The saved in-game copy below loads on manual /reload after the finished fight reaches the engine.",4,-104,588)
    local report=LUI:Description(parent,"",4,-178,588)
    A.views[#A.views+1]=function()
        if not parent:IsVisible()then return end
        local r=A:Review()
        if not r then report:SetText("No Aug log review loaded for this character yet. The engine needs the start and end of a boss/key, supported advanced combat logs, and an Aug specialization or Aug spell record.");return end
        report:SetText(string.format("%s — %s\nRecorded Ebon self uptime: %.1f%%\nPrescience: %.2f average observed targets\nBombardments support: %.0f damage (partial)\nReattributed group damage is in the browser and floating meter.",r.name or "Encounter",r.endedAt or "",100*(r.ebonSeconds or 0)/math.max(1,r.seconds or 0),(r.prescienceTargetSeconds or 0)/math.max(1,r.seconds or 0),r.recordedSupportDamage or 0))
    end
    LUI:Description(parent,"Log writes can arrive late. The browser displays the last event and receipt delay. Basic/unsupported logs, restarts, and mid-fight attachment stay pending instead of producing a complete score.",4,-360,588)
end
module.build=function(parent)
    LUI:Tabs(parent,{{id="damage",name="Combat meters",build=damage},{id="windows",name="Windows",build=function(parent)if LUI.Meters and LUI.Meters.BuildWindowsPage then LUI.Meters:BuildWindowsPage(parent)end end},{id="meter",name="Augmentation",build=meter},{id="border",name="Capture",build=border},{id="chat",name="Chat",build=chat},{id="logs",name="Engine review",build=logs}})
    LUI:OnRefresh(parent,function()A:Configure();if LUI.Meters then LUI.Meters:Refresh()end end)
end
LUI:RegisterModule(module)
