-- Settings belong to this module; sample collection lives in Performance.lua.
local _,LUI=...
local P=LUI.Performance
local module={id="performance",name="Performance",enabledKey="performanceEnabled",settingsScope="global",
    enabledHelp="Collect frame timing and the selected performance measurements. Turning this off stops all Performance sampling. LamdaCD keeps running. These settings are shared across profiles and apply without reloading.",
    defaults={performanceEnabled=false,performanceX=20,performanceY=100,
        performanceAddonCPU=true,performanceNetwork=false,performanceShowTracker=true},
    limits={performanceX={0,16000},performanceY={0,16000}}}
P.settingsViews={}
local function value(n,format)return n and string.format(format or "%.0f",n) or "unavailable" end
local function status()
    if not P.enabled then return "Collection is off. Enable Performance above to begin." end
    if not P.latest then return "Collecting a new sample..." end
    return string.format("%.0f seconds of sampled gameplay (up to 60 seconds).",P:Summary().seconds)
end
local function watch(parent,draw)
    P.settingsViews[#P.settingsViews+1]={frame=parent,draw=draw}
    LUI:OnRefresh(parent,function()if parent:IsVisible()then draw(P:Summary())end end)
end
function P:RefreshPanels(data)
    for _,view in ipairs(self.settingsViews)do
        if view.frame:IsVisible()then view.draw(data)end
    end
end
local function frameTiming(parent)
    LUI:Description(parent,"Measure how smoothly WoW draws frames. A high FPS average can still hide a short freeze; worst-frame time and slow-frame counts help expose it.",4,0,588)
    local state=LUI:Label(parent,"",4,-84)
    local fps=LUI:Label(parent,"",4,-126,true)
    local worst=LUI:Label(parent,"",4,-162)
    local stalls=LUI:Label(parent,"",4,-196)
    watch(parent,function(data)
        state:SetText(status());fps:SetText("Current FPS: "..value(data.fps))
        worst:SetText("Worst frame in sample: "..(data.fps and value(data.worst).." ms" or "collecting"))
        stalls:SetText("Frames over 100 ms: "..(data.fps and data.stalls or "collecting"))
    end)
    LUI:Description(parent,"Loading screens are excluded. WoW's background FPS limit can still register as slow frames. Keep the game in front when comparing samples.",4,-250,588)
    LUI:Description(parent,"Frame timing is collected whenever this module is enabled. Settings are shared across profiles; Reset General and Reset LamdaCD leave them alone.",4,-328,588)
end
local function addonCPU(parent)
    LUI:Checkbox(parent,"Track addon CPU","performanceAddonCPU",0,0,
        "Read WoW's built-in Lua CPU profiler once per sampled second. Count new ticks over 100 ms for Lamda and all addons combined. Turning this off stops CPU queries, including the busiest-addon list. Changing collection settings starts a new sample.")
    LUI:ResetButton(parent,"addon CPU collection",{"performanceAddonCPU"},338,0)
    LUI:Description(parent,"CPU time and new spikes are measured separately from frame timing. Counts share a sample window; they do not identify an exact frozen frame.",4,-46,588)
    local state=LUI:Label(parent,"",4,-110)
    local recent=LUI:Label(parent,"",4,-146)
    local spikes=LUI:Label(parent,"",4,-180)
    LUI:Label(parent,"Busiest addons by recent Lua time",4,-226,true)
    local rows={};for i=1,3 do
        rows[i]=LUI:Label(parent,"",4,-258-(i-1)*24);rows[i]:SetWidth(588);rows[i]:SetMaxLines(1);rows[i]:SetWordWrap(false);rows[i]:SetJustifyH("LEFT")
    end
    watch(parent,function(data)
        state:SetText(not P.trackCPU and "Addon CPU collection is off." or status())
        recent:SetText("Lamda recent CPU: "..value(data.recent,"%.3f").." ms/tick")
        spikes:SetText("Lua ticks over 100 ms: Lamda "..value(data.spikes).." | all addons "..value(data.overallSpikes))
        local top=P:TopAddons()
        for i,row in ipairs(rows)do
            local entry=top and top[i]
            row:SetText(entry and string.format("%s: %.3f ms/tick",entry.name,entry.value)
                or (i==1 and "No current addon ranking available." or ""))
        end
    end)
    LUI:Description(parent,"CPU values describe addon Lua. The external engine, GPU and other game work need separate measurements. The detailed report labels the historical peak separately.",4,-326,588)
end
local function network(parent)
    LUI:Checkbox(parent,"Track network latency","performanceNetwork",0,0,
        "Read the Home and World latency values WoW already reports. This sends no extra network requests. Values refresh at the game's own pace, so this is not a measurement of every packet. Changing collection settings starts a new sample.")
    LUI:Description(parent,"Compare connection delay with frame stalls. These are WoW's reported Home and World latency values, not FPS or engine update delay.",4,-46,588)
    local state=LUI:Label(parent,"",4,-114)
    local home=LUI:Label(parent,"",4,-158,true)
    local world=LUI:Label(parent,"",4,-196,true)
    watch(parent,function(data)
        state:SetText(not P.trackNetwork and "Network collection is off." or status())
        home:SetText("Home latency: "..value(data.home).." ms")
        world:SetText("World latency: "..value(data.world).." ms")
    end)
    LUI:Description(parent,"Enable this only when you want connection measurements. Unavailable values stay unknown. This module does not infer packet loss from latency.",4,-256,588)
    LUI:ResetButton(parent,"network collection",{"performanceNetwork"},4,-346)
end
local function tasks(parent)
    LUI:Description(parent,"Find which Lamda task takes time. These are elapsed times per call, including nested work. They overlap, so do not add them together.",4,0,588)
    local state=LUI:Label(parent,"",4,-70)
    LUI:Label(parent,"Task",4,-110,true);LUI:Label(parent,"Max ms",280,-110)
    LUI:Label(parent,">100 ms",386,-110);LUI:Label(parent,"Calls",484,-110)
    local rows={}
    for i=1,5 do
        rows[i]={LUI:Label(parent,"",4,-146-(i-1)*34),LUI:Label(parent,"",280,-146-(i-1)*34),
            LUI:Label(parent,"",386,-146-(i-1)*34),LUI:Label(parent,"",484,-146-(i-1)*34)}
    end
    watch(parent,function()
        state:SetText(not P.trackCPU and "Enable Track addon CPU to collect task timings." or status())
        local data=P:WorkSummary()
        for i,row in ipairs(rows)do
            local entry=data[i]
            row[1]:SetText(entry and entry.name or (i==1 and "No task timings collected yet." or ""))
            row[2]:SetText(entry and string.format("%.2f",entry.peak) or "")
            row[3]:SetText(entry and tostring(entry.spikes) or "")
            row[4]:SetText(entry and tostring(entry.calls) or "")
        end
    end)
    LUI:Description(parent,"Timings use the current sample and are sorted by the longest call. Reset sample clears them. This measures only the listed paths, not every function or external process.",4,-326,588)
end
local function display(parent)
    LUI:Checkbox(parent,"Show floating tracker","performanceShowTracker",0,0,
        "Show the compact performance readout while this module is enabled. Hiding the readout keeps measurements running and available on these pages. Disable the module above to stop collecting.")
    LUI:Description(parent,"The tracker shows frame timing plus any enabled CPU and network measurements. Its position is shared across profiles.",4,-46,588)
    P.moveButton=LUI:Button(parent,"Move tracker",4,-124,150,function()P:SetMoving(not P.moving)end)
    LUI:Button(parent,"Reset position",166,-124,150,function()
        P:SetMoving(false);LUI:ResetKeys({"performanceX","performanceY"})
    end)
    LUI:Description(parent,"Click Move tracker, drag the readout, then lock it. Closing settings also locks it. The small Reset and minus buttons on the readout remain clickable while locked.",4,-176,588)
    LUI:Description(parent,"/lamda fps shows or hides the tracker. /lamda performance opens this module. /lamda perf prints the current report.",4,-250,588)
    LUI:ResetButton(parent,"performance display",{"performanceShowTracker","performanceX","performanceY"},4,-346)
end
module.build=function(parent)
    LUI:Tabs(parent,{{id="frames",name="Frame timing",build=frameTiming},
        {id="cpu",name="Addon CPU",build=addonCPU},{id="tasks",name="Lamda tasks",build=tasks},
        {id="network",name="Network",build=network},
        {id="display",name="Display",build=display}})
    LUI:Button(parent,"Reset sample",4,-426,132,function()P:Reset()end)
    local report=LUI:Button(parent,"Print report",146,-426,132,function()P:Report()end)
    LUI:Explain(report,"Performance report","Print current frame, Lua CPU and enabled network measurements. The CPU peak is historical and includes loading. Reset sample starts fresh local history and spike baselines.")
    local reset=LUI:ResetButton(parent,"all Performance settings (shared across profiles)",function()LUI:ResetModule("performance")end,290,-426)
    reset:SetText("Reset Performance")
end
LUI:RegisterModule(module)
