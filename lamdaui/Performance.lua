-- Performance module runtime, independent of LamdaCD and the external engine.
-- Disabled: no OnUpdate. Enabled: counters only per frame; text/profiler once/sec.
local addonName,LUI=...
local P={enabled=false,history={},clock=0,next=1,elapsed=0,frames=0,worst=0,stalls=0}
LUI.Performance=P
local function number(value)
    if issecretvalue and issecretvalue(value)then return nil end
    if type(value)=="number" and value==value and value>=0 and value<math.huge then return value end
end
local function metric(name,overall)
    local api=C_AddOnProfiler;local metrics=Enum and Enum.AddOnProfilerMetric
    local fn=api and api.GetAddOnMetric
    if overall then fn=api and api.GetOverallMetric end
    if type(fn)~="function" or not metrics or metrics[name]==nil then return nil end
    if type(api.IsEnabled)=="function"then
        local ok,on=pcall(api.IsEnabled);if not ok or on~=true then return nil end
    end
    local ok,value
    if overall then ok,value=pcall(fn,metrics[name])else ok,value=pcall(fn,addonName,metrics[name])end
    return ok and number(value) or nil
end
local workNames={"Frame discovery and auras","Own cooldown refreshes","Engine event strip","Settings refresh","Performance sampling","Talent inspection"}
local knownWork={};for _,name in ipairs(workNames)do knownWork[name]=true end
function P:BeginWork()
    if self.enabled and self.trackCPU and not self.paused and not self.skipNext and type(debugprofilestop)=="function"then
        return number(debugprofilestop())
    end
end
function P:EndWork(name,started)
    if not started or not self.enabled or not self.trackCPU or not knownWork[name] or type(debugprofilestop)~="function"then return end
    local finished=number(debugprofilestop());if not finished or finished<started then return end
    local elapsed=finished-started
    self.work=self.work or {};local row=self.work[name]
    if not row then row={calls=0,total=0,peak=0,spikes=0};self.work[name]=row end
    row.calls=row.calls+1;row.total=row.total+elapsed;row.peak=math.max(row.peak,elapsed)
    if elapsed>100 then row.spikes=row.spikes+1 end
end
function P:WorkSummary()
    local result={}
    for _,name in ipairs(workNames)do
        local row={name=name,calls=0,total=0,peak=0,spikes=0}
        local function add(work)
            local value=work and work[name]
            if value then row.calls=row.calls+value.calls;row.total=row.total+value.total;row.peak=math.max(row.peak,value.peak);row.spikes=row.spikes+value.spikes end
        end
        for _,entry in pairs(self.history)do if entry.at>self.clock-60 then add(entry.work)end end
        add(self.work)
        if row.calls>0 then result[#result+1]=row end
    end
    table.sort(result,function(a,b)if a.peak~=b.peak then return a.peak>b.peak end;return a.name<b.name end)
    return result
end
function P:RebaseCounters()
    self.previousSpikes=self.trackCPU and metric("CountTimeOver100Ms") or nil
    self.previousOverallSpikes=self.trackCPU and metric("CountTimeOver100Ms",true) or nil
end
local function increase(current,previous)
    -- Missing or reset counters are unknown, never zero. Rebase after a gap.
    if current and previous and current>=previous then return current-previous end
end
function P:Reset()
    self.history={};self.next=1;self.clock=0;self.elapsed=0;self.frames=0;self.worst=0;self.stalls=0
    self.previousSpikes=nil;self.previousOverallSpikes=nil
    self.work={}
    if self.enabled and not self.paused then self:RebaseCounters()end
    self.latest=nil;self:Draw()
end
function P:Summary()
    local peak,stalls,seconds,spikes,overallSpikes=0,0,0,0,0
    local complete,overallComplete=true,true
    for _,entry in pairs(self.history)do
        if entry.at>self.clock-60 then
            peak=math.max(peak,entry.worst);stalls=stalls+entry.stalls;seconds=seconds+entry.elapsed
            if entry.spikes==nil then complete=false else spikes=spikes+entry.spikes end
            if entry.overallSpikes==nil then overallComplete=false else overallSpikes=overallSpikes+entry.overallSpikes end
        end
    end
    return {fps=self.latest and self.latest.fps,worst=peak,stalls=stalls,
        seconds=math.min(60,seconds),spikes=seconds>0 and complete and spikes or nil,
        overallSpikes=seconds>0 and overallComplete and overallSpikes or nil,
        home=self.latest and self.latest.home,world=self.latest and self.latest.world,
        recent=self.latest and self.latest.recent,peak=self.latest and self.latest.peak}
end
function P:Observe(delta)
    if not self.enabled or self.paused then return end
    if self.skipNext then self.skipNext=false;self:RebaseCounters();return end
    if not number(delta) or delta==0 then return end
    self.clock=self.clock+delta;self.elapsed=self.elapsed+delta;self.frames=self.frames+1
    self.worst=math.max(self.worst,delta*1000)
    if delta>=.1 then self.stalls=self.stalls+1 end
    if self.elapsed<1 then return end
    local timing=self:BeginWork()
    local sample={at=self.clock,elapsed=self.elapsed,fps=self.frames/self.elapsed,worst=self.worst,stalls=self.stalls}
    if self.trackCPU then
        local spikes,overallSpikes=metric("CountTimeOver100Ms"),metric("CountTimeOver100Ms",true)
        sample.recent=metric("RecentAverageTime");sample.peak=metric("PeakTime")
        sample.spikes=increase(spikes,self.previousSpikes);sample.overallSpikes=increase(overallSpikes,self.previousOverallSpikes)
        self.previousSpikes=spikes;self.previousOverallSpikes=overallSpikes
    end
    if self.trackNetwork and type(GetNetStats)=="function" then
        local ok,_,_,home,world=pcall(GetNetStats)
        if ok then sample.home=number(home);sample.world=number(world)end
    end
    sample.work=self.work;self.work={}
    self.history[self.next]=sample;self.next=self.next%60+1;self.latest=sample
    self.elapsed=0;self.frames=0;self.worst=0;self.stalls=0;self:Draw()
    if timing then self:EndWork("Performance sampling",timing)end
end
function P:Draw()
    if not self.frame then return end
    local data=self:Summary()
    local fps=data.fps and string.format("%.0f FPS",data.fps) or "FPS: collecting..."
    self.head:SetText(fps..string.format("   |   Worst frame: %.0f ms",data.worst))
    self.head:SetTextColor(data.worst>=100 and 1 or .65,data.worst>=100 and .65 or 1,.75)
    self.detail:SetText(string.format("Sample: %.0fs | %d frames over 100 ms",data.seconds,data.stalls))
    local cpu=data.recent and string.format("Lamda Lua: %.2f ms/tick",data.recent) or "Lamda Lua: profiler unavailable"
    self.cpu:SetText(cpu)
    self.spikes:SetText(string.format("Lua >100 ms: Lamda %s | all addons %s",data.spikes or "?",data.overallSpikes or "?"))
    self.network:SetText(string.format("Latency: Home %s ms | World %s ms",data.home and string.format("%.0f",data.home) or "?",data.world and string.format("%.0f",data.world) or "?"))
    self.moveHint:SetShown(self.moving==true)
    if self.RefreshPanels then self:RefreshPanels(data)end
end
function P:LayoutTracker()
    local y=50
    self.cpu:SetShown(self.trackCPU);self.spikes:SetShown(self.trackCPU)
    if self.trackCPU then y=y+40 end
    self.network:ClearAllPoints();self.network:SetPoint("TOPLEFT",10,-y);self.network:SetShown(self.trackNetwork)
    if self.trackNetwork then y=y+20 end
    self.moveHint:ClearAllPoints();self.moveHint:SetPoint("TOPLEFT",10,-y-2)
    self.frame:SetHeight(y+24)
end
function P:ApplyPosition()
    if not self.frame or self.dragging then return end
    local db=LUI:DB();local width,height=UIParent:GetWidth(),UIParent:GetHeight()
    local x=math.max(0,math.min(db.performanceX,math.max(0,width-350)))
    local y=math.max(0,math.min(db.performanceY,math.max(0,height-self.frame:GetHeight())))
    self.frame:ClearAllPoints();self.frame:SetPoint("TOPLEFT",UIParent,"TOPLEFT",x,-y)
end
function P:StopDrag()
    if not self.dragging then return end
    self.frame:StopMovingOrSizing();self.dragging=false
    local x=self.frame:GetLeft();local top=self.frame:GetTop()
    if number(x) and number(top)then
        local db=LUI:DB();db.performanceX=math.max(0,x);db.performanceY=math.max(0,UIParent:GetHeight()-top)
    end
    self:ApplyPosition()
end
function P:SetMoving(value)
    if not self.frame then return end
    if not value then self:StopDrag()end
    self.moving=not not (value and self.enabled and self.showTracker)
    self.frame:EnableMouse(self.moving);self:Draw()
    if self.moveButton then self.moveButton:SetText(self.moving and "Lock tracker" or "Move tracker")end
end
function P:Initialize()
    if self.frame then return end
    self.sampler=CreateFrame("Frame") -- Keeps collecting when the readout is hidden.
    local frame=CreateFrame("Frame","LamdaUIPerformance",UIParent,"BackdropTemplate");self.frame=frame
    frame:SetSize(350,114);frame:SetFrameStrata("HIGH");frame:SetMovable(true);frame:SetClampedToScreen(true)
    frame:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
    frame:SetBackdropColor(.03,.05,.08,.9);frame:SetBackdropBorderColor(.2,.6,.65,1)
    self.head=LUI:Label(frame,"",10,-10);self.detail=LUI:Label(frame,"",10,-30)
    self.head:SetWidth(270);self.head:SetMaxLines(1);self.head:SetWordWrap(false);self.head:SetJustifyH("LEFT")
    local function control(text,right,width,action,title,help)
        local button=LUI:Button(frame,text,0,0,width,action)
        button:ClearAllPoints();button:SetPoint("TOPRIGHT",frame,"TOPRIGHT",right,-6);button:SetHeight(18)
        button.label:SetFontObject("GameFontHighlightSmall");button.label:SetWidth(width-4)
        -- Only these small child buttons catch clicks while the tracker is locked.
        button:EnableMouse(true);button:RegisterForClicks("LeftButtonUp")
        LUI:Explain(button,title,help)
        return button
    end
    self.resetButton=control("Reset",-28,40,function()P:Reset()end,"Reset performance sample",
        "Clear the current frame, Lua-spike and task-timing sample and start measuring again. Your settings and tracker position are kept. The historical CPU peak in the report is not reset.")
    self.hideButton=control("-",-6,18,function()P:HideTracker()end,"Hide performance tracker",
        "Hide this readout while measurements continue. Run /lamda fps to show it again, or use Performance > Display. Disable the Performance module to stop collection.")
    self.cpu=LUI:Label(frame,"",10,-50);self.spikes=LUI:Label(frame,"",10,-70)
    self.network=LUI:Label(frame,"",10,-90)
    self.moveHint=LUI:Label(frame,"Drag to move; lock in Performance",10,-112)
    frame:RegisterForDrag("LeftButton")
    frame:SetScript("OnDragStart",function()if P.moving then P.dragging=true;frame:StartMoving()end end)
    frame:SetScript("OnDragStop",function()P:StopDrag()end)
    frame:EnableMouse(false);frame:Hide();self:ApplyPosition();self:Draw()
end
function P:RefreshSettings()
    self:Initialize()
    local db=LUI:DB();local on=db.performanceEnabled
    local trackCPU=db.performanceAddonCPU and LUI:OptionalWork("resourceCPU",db)
    local changed=self.trackCPU~=trackCPU or self.trackNetwork~=db.performanceNetwork
    self.trackCPU=trackCPU;self.trackNetwork=db.performanceNetwork;self.showTracker=db.performanceShowTracker
    if on~=self.enabled or changed then
        self.enabled=on;self:Reset()
        self.sampler:SetScript("OnUpdate",on and function(_,delta)P:Observe(delta)end or nil)
    end
    if not on or not self.showTracker then self:SetMoving(false)end
    self.frame:SetShown(on and self.showTracker);self:LayoutTracker();self:ApplyPosition();self:Draw()
    if self.moveButton then self.moveButton:SetEnabled(on and self.showTracker)end
end
function P:ToggleTracker()
    local db=LUI:DB()
    if not db.performanceEnabled then db.performanceEnabled=true;db.performanceShowTracker=true
    else db.performanceShowTracker=not db.performanceShowTracker end
    LUI:RefreshUI()
    print("LamdaUI: Performance tracker "..(db.performanceShowTracker and "shown. Move it in Modules > Performance > Display." or "hidden; measurements continue. Disable the Performance module to stop collection."))
end
function P:HideTracker()
    self:SetMoving(false)
    LUI:DB().performanceShowTracker=false
    self:RefreshSettings()
    -- Refresh open controls once; sampling itself never refreshes the settings UI.
    if LUI.frame and LUI.frame:IsShown()then LUI:RefreshUI()end
end
function P:TopAddons()
    if not self.enabled or not self.trackCPU then return nil end
    local api=C_AddOnProfiler;local metrics=Enum and Enum.AddOnProfilerMetric
    if api and type(api.IsEnabled)=="function"then
        local ok,on=pcall(api.IsEnabled);if not ok or on~=true then return nil end
    end
    if api and type(api.GetTopKAddOnsForMetric)=="function" and metrics and metrics.RecentAverageTime then
        local ok,top=pcall(api.GetTopKAddOnsForMetric,metrics.RecentAverageTime,3)
        if ok and type(top)=="table"then
            local result={}
            for _,entry in ipairs(top)do
                if #result>=3 then break end
                if type(entry)=="table" and type(entry.addOnName)=="string" and number(entry.metricValue)then
                    result[#result+1]={name=entry.addOnName,value=entry.metricValue}
                end
            end
            return result
        end
    end
end
function P:Report()
    if not self.enabled then print("LamdaUI: Enable the Performance module to collect measurements.");return end
    local data=self:Summary()
    if data.fps then
        print(string.format("LamdaUI: %.0f FPS; worst frame %.0f ms; %d frames over 100 ms in %.0fs sampled gameplay (up to 60s).",data.fps,data.worst,data.stalls,data.seconds))
        if self.trackCPU then
            print(string.format("Lua ticks over 100 ms during that sample: Lamda %s; all addons combined %s.",data.spikes or "unknown",data.overallSpikes or "unknown"))
            print("These are counters within the sample, not exact frame matches. Low Lua counts do not exclude engine, capture, GPU or other game stalls.")
        end
    end
    if self.trackNetwork then print(string.format("Network latency: Home %s ms; World %s ms (game-reported values).",data.home or "unknown",data.world or "unknown"))end
    if not self.trackCPU then print("LamdaUI: Addon CPU collection is off.");return end
    local recent,peak=metric("RecentAverageTime"),metric("PeakTime")
    if recent then print(string.format("LamdaUI addon CPU: %.3f ms/tick recent; %s ms peak since application startup (includes loading).",recent,peak and string.format("%.3f",peak) or "unknown"))
    else print("LamdaUI: Built-in addon CPU profiler unavailable.")end
    for _,entry in ipairs(self:TopAddons() or {})do
        print(string.format("Addon CPU: %s %.3f ms/tick recent",entry.name,entry.value))
    end
    for _,entry in ipairs(self:WorkSummary())do
        print(string.format("Lamda task: %s; max %.2f ms/call; %d calls over 100 ms; %d calls in sample.",entry.name,entry.peak,entry.spikes,entry.calls))
    end
end
local events=CreateFrame("Frame")
for _,event in ipairs({"LOADING_SCREEN_ENABLED","LOADING_SCREEN_DISABLED","UI_SCALE_CHANGED","DISPLAY_SIZE_CHANGED"})do events:RegisterEvent(event)end
events:SetScript("OnEvent",function(_,event)
    if event=="LOADING_SCREEN_ENABLED"then P.paused=true
    elseif event=="LOADING_SCREEN_DISABLED"then
        P.paused=false;P.skipNext=true;P.elapsed=0;P.frames=0;P.worst=0;P.stalls=0;P.work={}
    else P:StopDrag();P:ApplyPosition()end
end)
