-- Queue metadata only. Lamda Engine performs the optional Windows click;
-- this module never calls a protected acceptance function or installs bindings.
local _, LUI = ...
local Q = {pvp={}, serial=0, tick=0, status="Waiting for a supported queue pop."}
LUI.Queue = Q
-- Keep clear of the overlay/meter packet bands along the screen's bottom edge.
-- The engine's QueueStripBottomInset uses the same physical-pixel offset.
local stripBottomInset=48
local function plain(value,kind)
    if issecretvalue and issecretvalue(value) then return nil end
    if type(value)~=kind then return nil end
    if kind=="number" and (value~=value or math.abs(value)==math.huge) then return nil end
    return value
end
local function call(fn,...)
    if type(fn)~="function" then return nil end
    local result={pcall(fn,...)}
    if result[1] then return unpack(result,2,20) end
end
local function number(value)return plain(value,"number")end
local function round(value)return math.floor(value+.5)end
function Q:Status(text)
    self.status=text
    if self.statusLabel then self.statusLabel:SetText(text) end
end
function Q:Record(started)
    self.serial=self.serial+1
    self.session=self.session or (tostring(GetServerTime()).."_"..math.floor(GetTime()*1000).."_"..(plain(call(UnitGUID,"player"),"string") or "player"))
    return {nonce=self.session.."_"..self.serial,started=started}
end
function Q:Geometry(button)
    if not button or call(button.IsForbidden,button)~=false or call(button.IsVisible,button)~=true or call(button.IsEnabled,button)~=true then return end
    local factor=PixelUtil and number(call(PixelUtil.GetPixelToUIUnitFactor))
    local scale=number(call(button.GetEffectiveScale,button))
    local parentScale=number(call(UIParent.GetEffectiveScale,UIParent))
    local left,top=number(call(button.GetLeft,button)),number(call(button.GetTop,button))
    local width,height=number(call(button.GetWidth,button)),number(call(button.GetHeight,button))
    local parentLeft,parentTop=number(call(UIParent.GetLeft,UIParent)),number(call(UIParent.GetTop,UIParent))
    local screenWidth,screenHeight=GetPhysicalScreenSize()
    screenWidth,screenHeight=number(screenWidth),number(screenHeight)
    if not factor or factor<=0 or not scale or scale<=0 or not parentScale or not left or not top or not width or not height or not parentLeft or not parentTop or not screenWidth or not screenHeight then return end
    local x=round(left*scale/factor-parentLeft*parentScale/factor)
    local y=round(parentTop*parentScale/factor-top*scale/factor)
    width,height=round(width*scale/factor),round(height*scale/factor)
    if x<0 or y<0 or width<8 or height<8 or x+width>screenWidth or y+height>screenHeight then return end
    return {round(screenWidth),round(screenHeight),x,y,width,height,button=button}
end
function Q:MarkerColors(nonce)
    local checksum=LUI.LookupEvents:Checksum(nonce)
    local r,g,b=math.floor(checksum/65536)%256,math.floor(checksum/256)%256,checksum%256
    return {255,0,255,r,g,b,0,255,255,255-r,255-g,255-b}
end
function Q:HideMarker()
    for _,cell in ipairs(self.marker or {}) do cell:Hide() end
    self.marker=nil
end
function Q:PaintMarker(candidate)
    local geometry=candidate.geometry;local button=geometry.button
    self.markers=self.markers or {}
    local cells=self.markers[button]
    if not cells then
        cells={};self.markers[button]=cells
        for index=1,4 do cells[index]=button:CreateTexture(nil,"OVERLAY",nil,7) end
    end
    if self.marker~=cells then self:HideMarker() end
    self.marker=cells
    local unit=PixelUtil.GetPixelToUIUnitFactor()/button:GetEffectiveScale()
    local colors=self:MarkerColors(candidate.record.nonce)
    for index,cell in ipairs(cells) do
        cell:SetSize(2*unit,2*unit);cell:ClearAllPoints()
        cell:SetPoint("TOPLEFT",button,"TOPLEFT",(math.floor(geometry[5]/2)-4+(index-1)*2)*unit,-(math.floor(geometry[6]/2)-1)*unit)
        local offset=(index-1)*3
        cell:SetColorTexture(colors[offset+1]/255,colors[offset+2]/255,colors[offset+3]/255,1);cell:Show()
    end
end
function Q:Candidates(now,db)
    local candidates={}
    local exists,_,_,_,_,_,_,responded=call(GetLFGProposal)
    exists,responded=plain(exists,"boolean"),plain(responded,"boolean")
    if exists==true and responded==false then
        -- Discovering a popup after /reload does not establish when it began.
        self.lfg=self.lfg or self:Record(nil)
        local geometry=self:Geometry(LFGDungeonReadyDialog and LFGDungeonReadyDialog.enterButton)
        local remaining=self.lfg.started and db.queueLFGWindow-(now-self.lfg.started)
        if db.queueLFG and not self.lfg.skip and geometry and (not remaining or remaining>0) then
            candidates[#candidates+1]={kind="lfg",record=self.lfg,geometry=geometry,remaining=remaining,estimated=true}
        end
    elseif exists==false or responded==true then self.lfg=nil end
    local maximum=number(MAX_BATTLEFIELD_QUEUES) or 3
    for index=1,math.min(maximum,16) do
        local status=plain(call(GetBattlefieldStatus,index),"string")
        if status=="confirm" then
            local record=self.pvp[index]
            if not record then record=self:Record(now);self.pvp[index]=record end
            local remaining=number(call(GetBattlefieldPortExpiration,index))
            local button
            for popupIndex=1,math.min(number(STATICPOPUP_NUMDIALOGS) or 4,16) do
                local popup=_G["StaticPopup"..popupIndex]
                if popup and plain(popup.which,"string")=="CONFIRM_BATTLEFIELD_ENTRY" and number(popup.data)==index and call(popup.IsVisible,popup)==true then
                    button=call(popup.GetButton1,popup) or popup.button1
                    break
                end
            end
            local geometry=self:Geometry(button)
            if db.queuePVP and not record.skip and geometry and remaining and remaining>0 and remaining<=300 then
                candidates[#candidates+1]={kind="pvp",record=record,geometry=geometry,remaining=remaining}
            end
        elseif status then self.pvp[index]=nil end
    end
    return candidates
end
function Q:Due(candidate,now,db)
    if db.queueMode==1 then return true end
    if db.queueMode==2 then return candidate.remaining~=nil and candidate.remaining>0 and candidate.remaining<=db.queueRemaining end
    return candidate.record.started~=nil and now-candidate.record.started>=db.queueDelay
end
function Q:Body(candidate,now,db)
    self.tick=self.tick+1
    local geometry=candidate.geometry
    local client=LUI.ForeverEngine and "forever" or "retail"
    local remaining=candidate.remaining and math.max(0,math.floor(candidate.remaining*1000)) or -1
    return table.concat({1,client,candidate.record.nonce,candidate.kind,GetServerTime(),self.tick,
        geometry[1],geometry[2],geometry[3],geometry[4],geometry[5],geometry[6],remaining,self:Due(candidate,now,db) and 1 or 0},"|")
end
function Q:CreateStrip()
    if self.strip then return end
    local frame=CreateFrame("Frame",nil,UIParent)
    frame:SetSize(128,16);frame:SetFrameStrata("TOOLTIP");frame:SetFrameLevel(10000);frame:EnableMouse(false);frame:Hide()
    self.strip=frame;self.cells={}
    for index=1,512 do
        local cell=frame:CreateTexture(nil,"ARTWORK")
        cell:SetSize(2,2);cell:SetPoint("TOPLEFT",frame,"TOPLEFT",((index-1)%64)*2,-math.floor((index-1)/64)*2)
        self.cells[index]=cell
    end
end
function Q:Hide()
    self:HideMarker()
    self.current=nil
    if self.strip then self.strip:Hide() end
end
function Q:Tick(now)
    if not self.ready or self.failed then return end
    local db=LUI:DB()
    if not db.queueEnabled or LUI.overlayHidden then self:Hide();self:Status(db.queueEnabled and "Paused while LamdaUI is hidden." or "Automatic acceptance is off.");return end
    if not LUI.LookupEvents or not LUI.LookupEvents.Packet or not PixelUtil then self:Hide();self:Status("Queue capture is unavailable on this client.");return end
    local candidates=self:Candidates(now,db)
    table.sort(candidates,function(a,b)
        local ad,bd=self:Due(a,now,db),self:Due(b,now,db)
        if ad~=bd then return ad end
        local ar,br=a.remaining or 301,b.remaining or 301
        if ar==br then return a.record.nonce<b.record.nonce end
        return ar<br
    end)
    local candidate=candidates[1]
    if not candidate then self:Hide();self:Status("Waiting for a supported visible queue pop.");return end
    local packet=LUI.LookupEvents:Packet(self:Body(candidate,now,db),"LUQ1")
    if not packet then self:Hide();self:Status("Queue capture packet unavailable.");return end
    self:CreateStrip()
    self:PaintMarker(candidate)
    self.strip:SetScale(PixelUtil.GetPixelToUIUnitFactor()/UIParent:GetEffectiveScale())
    self.strip:ClearAllPoints();self.strip:SetPoint("BOTTOMLEFT",UIParent,"BOTTOMLEFT",0,stripBottomInset)
    for index,cell in ipairs(self.cells) do
        local offset=(index-1)*3
        cell:SetColorTexture(packet:byte(offset+1)/255,packet:byte(offset+2)/255,packet:byte(offset+3)/255,1)
    end
    self.current=candidate;self.strip:Show()
    local timer=candidate.remaining and string.format("%s%.1fs left",candidate.estimated and "~" or "",candidate.remaining) or "start time unknown"
    self:Status((candidate.kind=="lfg" and "Dungeon/LFR: " or "PvP: ")..timer..(self:Due(candidate,now,db) and "; requesting engine acceptance." or "; waiting for configured timing."))
end
function Q:Skip()
    if self.current then self.current.record.skip=true;self:Tick(GetTime()) end
end
local module={id="queue",name="LamdaQueue",enabledKey="queueEnabled",settingsScope="global",
    enabledHelp="Allow Lamda Engine to click supported queue acceptance buttons while WoW has focus. Requires the matching engine update.",
    defaults={queueEnabled=false,queueLFG=true,queuePVP=true,queueMode=2,queueRemaining=3,queueDelay=5,queueLFGWindow=40},
    limits={queueMode={1,3,integer=true},queueRemaining={1,30,integer=true},queueDelay={0,60,integer=true},queueLFGWindow={10,120,integer=true}}}
module.build=function(parent)
    LUI:Description(parent,"Requires Lamda Engine. Acceptance pauses while WoW is unfocused, a button is covered, or a mouse button/modifier is held.",4,0,650)
    LUI:Checkbox(parent,"Dungeon Finder, Raid Finder and scenarios","queueLFG",-55,0)
    LUI:Checkbox(parent,"Battleground and arena queues","queuePVP",-90,0)
    LUI:Choice(parent,"Accept timing","queueMode",{"Immediately","With N seconds remaining","N seconds after the pop"},-140)
    LUI:Slider(parent,"Seconds remaining","queueRemaining",-205,1,30,1,0)
    LUI:Slider(parent,"Delay after pop (seconds)","queueDelay",-205,0,60,1,340)
    LUI:Slider(parent,"Dungeon proposal window (estimate)","queueLFGWindow",-280,10,120,1,0)
    LUI:Description(parent,"PvP uses the game's remaining time. Dungeon/LFR timing estimates a 40-second window from the pop; it is not a server deadline. After a reload during a dungeon pop, only Immediately can run. Already accepted, cancelled, hidden or skipped pops are ignored.",4,-355,650)
    LUI:Button(parent,"Skip current pop",4,-435,165,function()Q:Skip()end)
    LUI:Button(parent,"Turn off",185,-435,120,function()LUI:DB().queueEnabled=false;Q:Tick(GetTime());if LUI.RefreshUI then LUI:RefreshUI()end end)
    Q.statusLabel=LUI:Description(parent,Q.status,4,-485,650)
end
LUI:RegisterModule(module)
local events=CreateFrame("Frame");Q.events=events
for _,event in ipairs({"PLAYER_LOGIN","PLAYER_ENTERING_WORLD","LFG_PROPOSAL_SHOW","LFG_PROPOSAL_DONE","LFG_PROPOSAL_FAILED","LFG_PROPOSAL_SUCCEEDED","UPDATE_BATTLEFIELD_STATUS"}) do pcall(events.RegisterEvent,events,event) end
events:SetScript("OnEvent",function(_,event)
    if event=="PLAYER_LOGIN" then Q.ready=true end
    if event=="LFG_PROPOSAL_SHOW" then Q.lfg=Q:Record(GetTime())
    elseif event=="LFG_PROPOSAL_DONE" or event=="LFG_PROPOSAL_FAILED" or event=="LFG_PROPOSAL_SUCCEEDED" then Q.lfg=nil;Q:Hide() end
end)
local elapsed=0
events:SetScript("OnUpdate",function(_,delta)
    elapsed=elapsed+delta;if elapsed<.1 then return end;elapsed=0
    local ok,problem=pcall(Q.Tick,Q,GetTime())
    if not ok and not Q.failed then Q.failed=true;Q:Hide();Q:Status("Queue acceptance paused after an error; reload after fixing it.");if geterrorhandler then pcall(geterrorhandler(),problem)end end
end)
SLASH_LAMDAQUEUE1="/lqueue"
SlashCmdList.LAMDAQUEUE=function(command)
    command=(command or ""):lower():match("^%s*(.-)%s*$")
    if command=="off" then LUI:DB().queueEnabled=false;Q:Tick(GetTime())
    elseif command=="on" then LUI:DB().queueEnabled=true;Q:Tick(GetTime())
    elseif command=="skip" then Q:Skip()
    else LUI:OpenUI();LUI:SelectPage("modules");LUI:SelectModule("queue") end
end
