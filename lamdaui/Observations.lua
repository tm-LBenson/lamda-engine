local _,LUI=...
local observations={casts={},diagnostics={},serial=0}
LUI.Observations=observations
local function gap(code)observations.diagnostics[code]=(observations.diagnostics[code] or 0)+1 end
local function public(value) return not issecretvalue or not issecretvalue(value) end
local function num(value)
    if not public(value) or type(value)~="number" or value~=value or value==math.huge or value==-math.huge then return nil end
    return value
end
local function str(value)
    if not public(value) or type(value)~="string" or #value<1 or #value>128 or value:find("[|,;\r\n]")then return nil end
    return value
end
observations.Number=num;observations.String=str;observations.Public=public
-- Native text owns restricted spell IDs. Lua supplies only a fixed format and
-- public delivery metadata; it never reads the resulting text or branches on it.
local screen={owners={},entries={},serial=0,generation=0,partyMeter=true}
observations.screenCasts=screen
-- WoW performs the hex formatting and color-markup rendering natively. Lua
-- never formats, compares, indexes, or reads back a restricted spell value.
local colorFormat="|cff%06xI|r"
local calibrationColors={0x055c6a,0x001992,0xabcdef,0x123456,0xffffff,0x000001,0x00ff00,0xff0000}
local castFonts={
    "Interface\\AddOns\\LamdaFrames\\Fonts\\RobotoMono-Bold.ttf",
    "Fonts\\ARIALN.TTF",
}
local function setCastFont(text,path,size)
    local ok=pcall(text.SetFont,text,path,size,"")
    if not ok then return false end
    -- Verify the applied public font rather than trusting SetFont's return.
    -- A failed call can leave the inherited font in place.
    local applied,height=text:GetFont()
    -- Native font heights can round to e.g. 10.999999 at an 11px request.
    -- Keep validating the face and size; the measured glyph pitch below is
    -- authoritative for whether seven digits actually fit the capture field.
    local valid=str(applied) and applied:lower()==path:lower() and num(height) and math.abs(height-size)<0.01
    local detail=string.format("requested %dpx, actual %s, face %s",size,tostring(num(height)),str(applied) or "unavailable")
    return valid,detail
end
function screen:Hide()
    if self.root then self.root:Hide()end
    for _,owner in pairs(self.owners)do owner.frame:UnregisterAllEvents()end
    self.enabled=false;self.key=nil;self.line=nil
end
function screen:Fail(err)
    self.failed=true;self:Hide();gap("screen_setup_unavailable")
    self.error=public(err) and type(err)=="string" and err or "Native display setup failed"
    print((self.name or "LamdaKick").." screen capture: "..self.error)
end
function screen:ReduceFont()
    local size=self.fontSize-1
    assert(size>=9,"Cast font cannot fit seven digits")
    local ok,detail=setCastFont(self.calibration,self.fontPath,size)
    assert(ok,"Cast calibration font unavailable: "..(detail or "SetFont failed"))
    self.calibration:SetText("0123456789")
    for _,entry in pairs(self.entries)do
        entry.at=nil;entry.text:SetText("")
        assert(setCastFont(entry.text,self.fontPath,size),"Cast font unavailable")
    end
    self.fontSize=size
end
local function castHeader(entry,index,now)
    local serial=entry.at and entry.serial or 0
    local age=entry.at and math.floor((now-entry.at)*1000) or 0
    if age<0 or age>30000 then serial=0;age=0 end
    local bytes={172,index,117,math.floor(serial/65536)%256,math.floor(serial/256)%256,serial%256,
        math.floor(age/256),age%256,57,0,91,203}
    local sum=0;for i=1,9 do sum=(sum+bytes[i]*i)%256 end;bytes[10]=sum
    for i,texture in ipairs(entry.cells)do
        local j=(i-1)*3;texture:SetColorTexture(bytes[j+1]/255,bytes[j+2]/255,bytes[j+3]/255,1)
    end
end
function screen:Cast(owner,spellID)
    if not self.enabled then return end
    if self.ownerValid then
        if not self.ownerValid(owner) then return end
    elseif str(UnitGUID(owner.unit))~=owner.guid then return end
    owner.next=(owner.next or 0)%4+1
    local index=owner.slot*4+owner.next-1;local entry=self.entries[index]
    entry.at=nil
    -- This is a native secret-value display sink, not string.format/GetText.
    local ok=pcall(entry.text.SetFormattedText,entry.text,self.colorMode and colorFormat or "%07d",spellID)
    if ok and entry.copy then ok=pcall(entry.copy.SetFormattedText,entry.copy,colorFormat,spellID)end
    if ok then
        self.serial=self.serial%16777215+1;entry.serial=self.serial;entry.at=GetTime()
        gap("screen_cast_displayed")
    else gap("screen_cast_unavailable")end
    castHeader(entry,index,GetTime())
end
-- The interrupt meter still receives landed interrupts when Retail suppresses
-- another player's instant UNIT_SPELLCAST_SUCCEEDED. Query by a public roster
-- GUID; pass totals straight to native color display sinks.
-- Never inspect, compare, format in Lua, or read back those restricted values.
function screen:UpdateMeter(now)
    if not self.meterMode then return end
    if not self.meterAt or now-self.meterAt>=0.25 then
        self.meterAt=now
        self.meterError=nil
        for slot,owner in pairs(self.owners)do
            local entry=self.meterEntries[slot];entry.at=nil
            if owner.guid and str(UnitGUID(owner.unit))==owner.guid then
                local ok,err=pcall(function()
                    local data=C_DamageMeter.GetCombatSessionSourceFromType(Enum.DamageMeterSessionType.Overall,Enum.DamageMeterType.Interrupts,owner.guid)
                    -- combatSpells names the ENEMY spells interrupted, not the
                    -- kick used. Only the total is an owner interrupt counter.
                    entry.text:SetFormattedText(colorFormat,data.totalAmount)
                    entry.copy:SetFormattedText(colorFormat,data.totalAmount)
                    self.serial=self.serial%16777215+1
                    entry.serial=self.serial;entry.at=now
                end)
                if not ok then
                    gap("screen_meter_unavailable")
                    self.meterError=public(err) and type(err)=="string" and err or "Interrupt meter unavailable"
                end
            end
        end
    end
    for index,entry in pairs(self.meterEntries)do castHeader(entry,index,now)end
end
function screen:Create(bridge,calibrationOnly)
    if self.root then return end
    self.colorMode=LamdaEngineDB.castCaptureMode~=2
    self.meterMode=self.partyMeter and self.colorMode and not calibrationOnly and C_DamageMeter
        and type(C_DamageMeter.GetCombatSessionSourceFromType)=="function" and Enum and Enum.DamageMeterSessionType and Enum.DamageMeterType
    self.fontSize=self.colorMode and 64 or 12
    self.root=CreateFrame("Frame",nil,bridge);self.root:SetSize(128,self.meterMode and 16 or (self.colorMode and 12 or 176))
    self.root:SetPoint("TOPLEFT",bridge,"TOPLEFT",0,-24)
    -- WoW clamps frame levels to 10000. Leave room for the viewport and its
    -- canvas: flattened levels let the backing cover every native glyph.
    -- Live calibration changed from eight black tiles to eight exact colors
    -- when this hierarchy was lowered to 9000 / 9001 / 9002.
    self.root:SetFrameStrata("TOOLTIP");self.root:SetFrameLevel(9000);self.root:EnableMouse(false)
    self.root:Hide()
    local background=self.root:CreateTexture(nil,"BACKGROUND");background:SetAllPoints();background:SetColorTexture(0,0,0,1)
    local function label(x,y,width,parent)
        parent=parent or self.root
        local text=parent:CreateFontString(nil,"OVERLAY","GameFontHighlightSmall")
        if not self.fontPath then
            for _,path in ipairs(castFonts)do
                if setCastFont(text,path,self.fontSize)then self.fontPath=path;break end
            end
            assert(self.fontPath,"Cast font unavailable (bundled and built-in)")
        else assert(setCastFont(text,self.fontPath,self.fontSize),"Cast font unavailable")end
        text:SetPoint("TOPLEFT",parent,"TOPLEFT",x,-y);text:SetSize(width,16)
        text:SetJustifyH("LEFT");text:SetJustifyV("MIDDLE");text:SetTextColor(1,1,1,1)
        text:SetShadowOffset(0,0);text:SetWordWrap(false);text:SetNonSpaceWrap(false)
        return text
    end
    if self.colorMode then
        self.colorViewports={}
        self.root:SetScript("OnShow",function()
            -- Resolve scrolling again after the final anchor/parent is visible;
            -- a hidden initial layout can otherwise have an empty scroll range.
            for _,viewport in ipairs(self.colorViewports)do
                -- The native I's ink is offset from its layout-box center.
                -- At x=46 the live client can draw two black columns inside
                -- each tile. x=48 covers the solid stem in every cast field;
                -- the engine still requires exact colors in all inner pixels.
                viewport:UpdateScrollChildRect();viewport:SetHorizontalScroll(48);viewport:SetVerticalScroll(46)
            end
        end)
        -- Clip the thick center stem of a large ASCII I to four physical pixels.
        -- The bundled font has no Unicode full-block glyph. ASCII also works in
        -- the built-in fallback. Calibration verifies solid coverage at runtime.
        local function patch(x,y,value,renderer)
            renderer=renderer or "scroll"
            local clip=CreateFrame(renderer=="scroll" and "ScrollFrame" or "Frame",nil,self.root);clip:SetSize(4,4)
            clip:SetPoint("TOPLEFT",self.root,"TOPLEFT",x,-y)
            clip:SetFrameStrata("TOOLTIP");clip:SetFrameLevel(self.root:GetFrameLevel()+1)
            clip:EnableMouse(false);clip:SetClipsChildren(true);clip:Show()
            -- Give the glyph a full-size layout surface and scroll its center
            -- into a four-pixel viewport. Direct regions on a tiny clipping
            -- frame produced black patches in the live client. Public self-test
            -- controls retain both that path and a plain child-frame path.
            local canvas=clip
            if renderer~="direct" then
                canvas=CreateFrame("Frame",nil,clip);canvas:SetSize(96,96)
                canvas:SetFrameLevel(clip:GetFrameLevel()+1);canvas:EnableMouse(false)
                if renderer=="scroll" then
                    clip:SetScrollChild(canvas);self.colorViewports[#self.colorViewports+1]=clip
                else canvas:SetPoint("CENTER",clip,"CENTER",0,0)end
                canvas:Show()
            end
            local text=label(0,0,96,canvas);text:ClearAllPoints()
            text:SetPoint("CENTER",canvas,"CENTER",0,0);text:SetSize(96,96);text:SetJustifyH("CENTER")
            text:SetTextHeight(self.fontSize);text:SetAlpha(1);text:Show()
            if value then
                text:SetFormattedText(colorFormat,value)
                -- Only fixed public calibration strings may be inspected. Cast
                -- regions are never read back, even after the event expires.
                local actual=text:GetText()
                if public(actual) then
                    assert(actual==string.format(colorFormat,value),"Native color calibration formatting failed")
                end
            end
            return text
        end
        self.colorCalibration={}
        for i,value in ipairs(calibrationColors)do self.colorCalibration[i]=patch((i-1)*4,0,value)end
        if calibrationOnly then
            self.calibrationControls={direct={},child={}}
            for i,value in ipairs(calibrationColors)do
                self.calibrationControls.direct[i]=patch(32+(i-1)*4,0,value,"direct")
                self.calibrationControls.child[i]=patch(64+(i-1)*4,0,value,"child")
            end
            -- An unclipped public glyph distinguishes font/markup failure from
            -- clipping. This occupies unused space and never carries a cast.
            local control=label(120,0,8);control:SetSize(8,12)
            assert(setCastFont(control,self.fontPath,12),"Control font unavailable")
            control:SetTextHeight(12);control:SetFormattedText(colorFormat,0xffffff)
            self.unclippedControl=control
            return
        end
        -- Native ScrollFrame clipping can round a color viewport one physical
        -- pixel into its neighboring header at fractional UI scales. Keep the
        -- public serial/age/checksum textures above every glyph canvas so that
        -- overlap cannot corrupt an otherwise valid cast or interrupt total.
        -- The engine still verifies the unchanged header and both color copies.
        self.headerRoot=CreateFrame("Frame",nil,self.root)
        self.headerRoot:SetAllPoints(self.root)
        self.headerRoot:SetFrameStrata("TOOLTIP");self.headerRoot:SetFrameLevel(self.root:GetFrameLevel()+3)
        self.headerRoot:EnableMouse(false)
        for index=0,19 do
            local x=(index%10)*12;local y=4+math.floor(index/10)*4
            local entry={text=patch(x+4,y),copy=patch(x+8,y),cells={}};self.entries[index]=entry
            for cell=0,3 do
                local t=self.headerRoot:CreateTexture(nil,"OVERLAY");t:SetSize(2,2)
                t:SetPoint("TOPLEFT",self.root,"TOPLEFT",x+(cell%2)*2,-y-math.floor(cell/2)*2)
                entry.cells[cell+1]=t
            end
            castHeader(entry,index,GetTime())
        end
        if self.meterMode then
            self.meterEntries={}
            for index=0,4 do
                local x=index*12;local y=12
                local entry={text=patch(x+4,y),copy=patch(x+8,y),cells={}}
                self.meterEntries[index]=entry
                for cell=0,3 do
                    local t=self.headerRoot:CreateTexture(nil,"OVERLAY");t:SetSize(2,2)
                    t:SetPoint("TOPLEFT",self.root,"TOPLEFT",x+(cell%2)*2,-y-math.floor(cell/2)*2)
                    entry.cells[cell+1]=t
                end
                castHeader(entry,index,GetTime())
            end
            self.meterEvents=CreateFrame("Frame")
            for _,event in ipairs({"DAMAGE_METER_RESET","PLAYER_ENTERING_WORLD","CHALLENGE_MODE_START"})do self.meterEvents:RegisterEvent(event)end
            self.meterEvents:SetScript("OnEvent",function()self.key=nil;self.meterAt=nil end)
        end
    else
    self.calibration=label(10,0,110);self.calibration:SetText("0123456789")
    for index=0,19 do
        local x=(index%2)*64;local y=16+math.floor(index/2)*16
        local entry={text=label(x+10,y,54),cells={}};self.entries[index]=entry
        for cell=0,3 do
            local t=self.root:CreateTexture(nil,"OVERLAY");t:SetSize(2,2)
            t:SetPoint("TOPLEFT",self.root,"TOPLEFT",x+2+(cell%2)*2,-y-6-math.floor(cell/2)*2)
            entry.cells[cell+1]=t
        end
        castHeader(entry,index,GetTime())
    end
    end
    for slot=0,4 do
        local owner={slot=slot,unit=self.units and self.units[slot+1] or (slot==0 and "player" or "party"..slot),pet=self.pets and self.pets[slot+1] or (slot==0 and "pet" or "partypet"..slot),frame=CreateFrame("Frame")}
        self.owners[slot]=owner
        owner.frame:SetScript("OnEvent",function(_,_,_,_,spellID)
            local ok=pcall(self.Cast,self,owner,spellID)
            if not ok then gap("screen_cast_unavailable")end
        end)
    end
end
function observations:ScreenCastLine()
    local db=LamdaEngineDB;local bridge=LUI.LookupEvents and LUI.LookupEvents.GetStrip and LUI.LookupEvents:GetStrip("kick")
    if LUI.ForeverEngine or not (db.kickEnabled and db.kickScreenCasts)
        or not bridge or not bridge.frame or not bridge.body or bridge.moving or IsInRaid()
        or (LUI.Kick and LUI.Kick.preview) then self:StopCastCaptureTest();screen:Hide();return nil end
    if IsInGroup() or InCombatLockdown() then self:StopCastCaptureTest()end
    if screen.failed then return nil end
    local ok,err=pcall(screen.Create,screen,bridge.frame)
    if not ok then
        screen:Fail(err)
        return nil
    end
    local roster={};local member=0
    for guid in bridge.body:gmatch("\n(Player%-%w[%w%-]*)|")do member=member+1;roster[guid]=member end
    local rows,identities,guids={},{},{}
    for slot,owner in pairs(screen.owners)do
        local guid=UnitExists(owner.unit) and str(UnitGUID(owner.unit))
        if guid and roster[guid]then rows[#rows+1]=slot..","..roster[guid];identities[#identities+1]=slot..":"..guid;guids[slot]=guid end
    end
    return screen:FormatLine(bridge,rows,identities,guids)
end
function screen:FormatLine(bridge,rows,identities,guids)
    local db=LamdaEngineDB
    table.sort(rows);table.sort(identities)
    if #rows==0 then self:Hide();return nil end
    local _,height=GetPhysicalScreenSize();local _,y=bridge:GetPosition()
    local panelHeight=self.meterMode and 16 or (self.colorMode and 12 or 176)
    local offset=y+24+panelHeight<=height and 24 or -(panelHeight+8)
    if y+offset<0 then self:Hide();return nil end
    -- The first bridge render happens while its parent is hidden. Anchor the
    -- native text before measuring it, and let a later render finish startup
    -- when font/layout metrics are not available yet. Never measure cast text.
    if self.positionOffset~=offset then
        self.root:ClearAllPoints();self.root:SetPoint("TOPLEFT",bridge.frame,"TOPLEFT",0,-offset)
        self.positionOffset=offset
    end
    local width=self.colorMode and 0 or num(self.calibration:GetUnboundedStringWidth())
    self.fontWidth=width
    if not self.colorMode and (not width or width<=40 or width>=100) then
        self:Hide();self.waitingForFont=true;self.root:Show();return nil
    end
    -- Seven digits must fit the 54px field without native text fitting or
    -- clipping. A live 12px font measured 8px per digit (56px per cast), so
    -- calibration succeeded while every event was unreadable. Leave 2px slack
    -- and resize calibration AND event text together before registering events.
    if not self.colorMode and width*0.7>52 then
        self:Hide()
        local resized,resizeError=pcall(self.ReduceFont,self)
        if not resized then self:Fail(resizeError);return nil end
        self.waitingForFont=true;self.root:Show();return nil
    end
    self.waitingForFont=false
    local pitch=math.floor(width*100+0.5)
    local key=table.concat(identities,";").."|"..offset.."|"..pitch
    if self.key~=key then
        self:Hide();self.generation=self.generation+1
        self.epoch=self.epoch or tostring(GetServerTime())..":"..math.floor(GetTime()*1000)
        for slot,owner in pairs(self.owners)do
            owner.guid=guids[slot];owner.next=0
            if owner.guid then owner.frame:RegisterUnitEvent("UNIT_SPELLCAST_SUCCEEDED",owner.unit,owner.pet)end
        end
        for _,entry in pairs(self.entries)do entry.at=nil;entry.text:SetText("");if entry.copy then entry.copy:SetText("")end end
        self.pitch=pitch
        self.meterAt=nil
        self.line=table.concat({"T",self.meterMode and 3 or (self.colorMode and 2 or 1),self.epoch..":"..self.generation,offset,table.concat(rows,";"),self.pitch},"|")
        self.key=key;self.enabled=true;self.root:Show()
    end
    for index,entry in pairs(self.entries)do castHeader(entry,index,GetTime())end
    self:UpdateMeter(GetTime())
    return self.line
end
function observations:NewCastScreen(units,pets,ownerValid)
    local instance={owners={},entries={},serial=0,generation=0,units=units,pets=pets,ownerValid=ownerValid}
    for key,value in pairs(screen)do if type(value)=="function" then instance[key]=value end end
    return instance
end
function observations:StopCastCaptureTest()
    if self.captureTest then self.captureTest:Hide();self.captureTest.active=false end
end
function observations:TestCastCapture()
    local db=LamdaEngineDB;local bridge=LUI.LookupEvents and LUI.LookupEvents.GetStrip and LUI.LookupEvents:GetStrip("kick")
    if LUI.ForeverEngine or not db.kickEnabled or not db.kickScreenCasts or db.castCaptureMode==2
        or IsInGroup() or InCombatLockdown() or not bridge or not bridge.frame or bridge.moving then return false end
    local test=self.captureTest or self:NewCastScreen();self.captureTest=test
    if test.failed then return false end
    local ok,err=pcall(test.Create,test,bridge.frame,true)
    if not ok then test:Fail(err);return false end
    local _,height=GetPhysicalScreenSize();local _,y=bridge:GetPosition()
    local offset=y+36<=height and 24 or -20
    if y+offset<0 then return false end
    test.root:ClearAllPoints();test.root:SetPoint("TOPLEFT",bridge.frame,"TOPLEFT",0,-offset)
    test.generation=test.generation+1;local generation=test.generation
    test.active=true;test.root:Show()
    -- Fixed public colors only: no owners, event registration, serials, or T
    -- packet. A successful self-test cannot be mistaken for a player's kick.
    C_Timer.After(60,function()if test.generation==generation then self:StopCastCaptureTest()end end)
    return true
end
function observations:ScreenStatusLine()
    local db=LamdaEngineDB;local bridge=LUI.LookupEvents and LUI.LookupEvents.GetStrip and LUI.LookupEvents:GetStrip("kick");local status="idle"
    if LUI.ForeverEngine then status="unsupported"
    elseif not (db.kickEnabled and db.kickScreenCasts)then status="disabled"
    elseif screen.failed then status="failed"
    elseif screen.waitingForFont then status="font_pending"
    elseif screen.line then status="active"
    elseif bridge and bridge.moving then status="moving"
    elseif IsInRaid()then status="raid"
    elseif LUI.Kick and LUI.Kick.preview then status="preview"end
    local errorValue=screen.error or screen.meterError
    local errorText=errorValue and errorValue:gsub("[|%c]"," "):sub(1,140) or ""
    local width=screen.fontWidth and math.max(0,math.min(1000000,math.floor(screen.fontWidth*1000+0.5))) or -1
    local d=self.diagnostics
    return table.concat({"S",status,width,d.screen_cast_displayed or 0,(d.screen_setup_unavailable or 0)+(d.screen_cast_unavailable or 0)+(d.screen_meter_unavailable or 0),errorText},"|")
end
-- Fixed personal-buff slots shared with lookup/aura_watch.go. The native
-- container alone decides whether each colored tile is visible. Lua never
-- queries aura data, aura buttons, durations or restricted visibility.
local auraSpells={
    [1]={871,12975,118038,184364,23920},[2]={498,642,31850},[3]={186265,264735},
    [4]={31224,5277},[5]={19236,47585},[6]={48707,48792,49028,49039,55233},
    [7]={108271},[8]={45438,110959,55342,414658,342245},[9]={108416,104773},[10]={122470},
    [11]={22812,61336},[12]={198589},[13]={363916},
}
observations.AuraSpells=auraSpells
local watch={containers={},generation=0}
observations.auraWatch=watch
local function tileColor(slot,cell,active)
    if cell==0 then if active then return 24/255,224/255,96/255,1 end;return 160/255,32/255,96/255,1 end
    return (24+(slot*37+cell*53)%208)/255,(24+(slot*61+cell*29)%208)/255,(24+(slot*43+cell*71)%208)/255,1
end
local function paintTile(parent,slot,active)
    for cell=0,3 do
        local texture=parent:CreateTexture(nil,"OVERLAY");texture:SetSize(2,2)
        texture:SetPoint("TOPLEFT",parent,"TOPLEFT",(cell%2)*2,-math.floor(cell/2)*2)
        texture:SetColorTexture(tileColor(slot,cell,active))
    end
end
function watch:Hide()
    if self.root then self.root:Hide()end
    if self.enabled then for _,c in pairs(self.containers)do pcall(c.SetEnabled,c,false)end end
    self.enabled=false;self.key=nil;self.line=nil
end
function watch:Create(bridge)
    if not AuraContainerSortMethod and C_AddOns and C_AddOns.LoadAddOn then
        C_AddOns.LoadAddOn("Blizzard_AuraContainer")
    end
    if not self.root then
        self.root=CreateFrame("Frame",nil,bridge);self.root:SetSize(128,8)
        self.root:SetFrameStrata("TOOLTIP");self.root:SetFrameLevel(10001);self.root:EnableMouse(false);self.root:Hide()
        for slot=0,39 do
            local tile=CreateFrame("Frame",nil,self.root);tile:SetSize(4,4)
            tile:SetPoint("TOPLEFT",self.root,"TOPLEFT",(slot%32)*4,-math.floor(slot/32)*4)
            paintTile(tile,slot,false)
        end
    end
    for slot=0,4 do
        if not self.containers[slot] then
            local c=CreateFrame("AuraContainer",nil,self.root,"CustomAuraContainerTemplate")
            c:Hide();c:SetEnabled(false)
            if type(c.AddAuraSlot)~="function" or type(c.SetAuraSlotCandidateFilters)~="function"then error("Native aura slots unavailable")end
            self.containers[slot]=c
            c:SetSize(1,1);c:SetPoint("TOPLEFT",self.root,"TOPLEFT",0,0)
            c:SetFrameStrata("TOOLTIP");c:SetFrameLevel(10003)
            c:SetUnit(slot==0 and "player" or "party"..slot)
            for index=1,5 do
                local tile=slot*8+index-1
                c:AddAuraSlot(tostring(index),"HELPFUL",{
                    candidateFilters={includeSpellIDs={}},sortMethod=AuraContainerSortMethod.Default,
                    sortDirection=AuraContainerSortDirection.Normal,
                    initializeFrame=function(button)
                        button:SetSize(4,4);button:SetPoint("TOPLEFT",self.root,"TOPLEFT",(tile%32)*4,-math.floor(tile/32)*4)
                        button:SetMouseClickEnabled(false);button:SetMouseMotionEnabled(false)
                        paintTile(button,tile,true)
                    end,
                })
            end
            -- Bound initialization work to one unit per bridge tick.
            return false
        end
    end
    return true
end
function observations:AuraWatchLine()
    local db=LamdaEngineDB;local bridge=LUI.LookupEvents
    if LUI.ForeverEngine or not (db.cdEnabled and db.engineTracking and db.engineFrameEstimates and db.lookupBridge and db.auraStarts)
        or not bridge or not bridge.frame or not bridge.body or bridge.moving or IsInRaid() or not IsInGroup()
        or (LUI.FrameAuras and LUI.FrameAuras.preview) then watch:Hide();return nil end
    if EditModeManagerFrame and type(EditModeManagerFrame.IsEditModeActive)=="function" and EditModeManagerFrame:IsEditModeActive()then watch:Hide();return nil end
    if watch.failed then return nil end
    if not watch.ready then
        if InCombatLockdown()then return nil end
        local ok,ready=pcall(watch.Create,watch,bridge.frame)
        if not ok then watch.failed=true;watch:Hide();gap("aura_watch_unavailable");return nil end
        watch.ready=ready;if not ready then return nil end
    end
    local roster={};local member=0
    for guid in bridge.body:gmatch("\n(Player%-%w[%w%-]*)|")do member=member+1;roster[guid]=member end
    if watch.selection~=db.engineFrameSpells then
        watch.selection=db.engineFrameSpells;watch.selected=LUI.SpellLists.Map(db.engineFrameSpells)
    end
    local rows,identities,units={},{},{}
    for slot=0,4 do
        local unit=slot==0 and "player" or "party"..slot
        if UnitExists(unit)then
            local guid=str(UnitGUID(unit));local _,_,class=UnitClass(unit);class=num(class)
            local visible=UnitIsVisible(unit)
            if guid and roster[guid] and class and auraSpells[class] and public(visible) and visible==true then
                local mask=0
                for i,id in ipairs(auraSpells[class])do
                    if watch.selected[id] or (id==414658 and watch.selected[45438])then mask=mask+2^(i-1)end
                end
                if mask>0 then
                    rows[#rows+1]=table.concat({slot,roster[guid],class,mask},",")
                    identities[#identities+1]=guid..":"..class..":"..mask
                    units[slot]={class=class,mask=mask}
                end
            end
        end
    end
    if #rows==0 then watch:Hide();return nil end
    local _,height=GetPhysicalScreenSize();local _,y=bridge:GetPosition()
    local offset=y+24<=height and 16 or -8
    local key=table.concat(rows,";").."|"..table.concat(identities,";").."|"..offset
    if key~=watch.key then
        watch:Hide()
        if InCombatLockdown()then return nil end
        local ok=pcall(function()
            watch.root:ClearAllPoints();watch.root:SetPoint("TOPLEFT",bridge.frame,"TOPLEFT",0,-offset)
            for slot,c in pairs(watch.containers)do
                local entry=units[slot]
                for i=1,5 do
                    local ids={};local id=entry and auraSpells[entry.class][i]
                    -- Alter Time's helpful aura is separate from its initial
                    -- cast. Its return button (342247) is never a new use.
                    if id and bit.band(entry.mask,2^(i-1))~=0 then ids[id==342245 and 342246 or id]=true end
                    c:SetAuraSlotCandidateFilters(tostring(i),{includeSpellIDs=ids})
                end
                c:SetEnabled(entry~=nil);c:SetShown(entry~=nil)
            end
        end)
        if not ok then watch.failed=true;watch:Hide();gap("aura_watch_unavailable");return nil end
        watch.key=key;watch.generation=watch.generation+1
        watch.epoch=watch.epoch or tostring(GetServerTime())..":"..math.floor(GetTime()*1000)
        watch.line=table.concat({"A",1,watch.epoch..":"..watch.generation,offset,table.concat(rows,";")},"|")
        watch.after=GetTime()+0.6;watch.enabled=true;watch.root:Show()
    end
    if GetTime()>=watch.after then return watch.line end
end
function observations:Enabled()
    local db=LamdaEngineDB
    return type(db)=="table" and (db.kickEnabled or LUI:CDTrackingEnabled(db))
end
function observations:Unit(guid)
    guid=str(guid)
    if not guid or not guid:match("^Player%-")then return nil end
    for _,unit in ipairs({"player","party1","party2","party3","party4"})do
        if UnitExists(unit) and str(UnitGUID(unit))==guid then return unit end
    end
end
function observations:SpellEnabled(id)
    local db=LamdaEngineDB
    if LUI.InterruptSpells and LUI.InterruptSpells[id]then return db.kickEnabled and db.kickLiveCasts end
    return LUI:CDTrackingEnabled(db) and db.liveCasts
end
function observations:Cast(unit,castGUID,spellID)
    if not self:Enabled() then return end
    -- Never index, compare, stringify, or export restricted combat values.
    if not public(unit) then gap("restricted_cast");gap("restricted_unit");return end
    if not public(spellID) then gap("restricted_cast");gap("restricted_spell");return end
    spellID=num(spellID)
    if not spellID or not LUI.ObservationSpells[spellID] or not self:SpellEnabled(spellID) then return end
    local pet=unit=="pet" or (type(unit)=="string" and unit:match("^partypet[1-4]$"))
    if pet then
        if not (LUI.InterruptSpells and LUI.InterruptSpells[spellID])then return end
        if not str(UnitGUID(unit))then return end
        unit=unit=="pet" and "player" or "party"..unit:sub(9)
    end
    if unit~="player" and not (type(unit)=="string" and unit:match("^party[1-4]$"))then return end
    local guid=str(UnitGUID(unit));castGUID=str(castGUID)
    if not guid then gap("cast_identity_unavailable");return end
    if not guid:match("^Player%-")then return end
    if castGUID and self.lastCast==castGUID then return end
    self.lastCast=castGUID;self.serial=self.serial+1
    -- The event GUID is only a delivery/dedup key. A public unit and spell
    -- still identify the cast when that optional key is missing/restricted.
    -- Generate an unrelated local key without reading the restricted value.
    if not castGUID then
        self.epoch=self.epoch or tostring(GetServerTime())..":"..tostring(math.floor(GetTime()*1000))
        castGUID="Local:"..self.epoch..":"..self.serial
        gap("local_cast_identity")
    end
    if #self.casts>=64 then table.remove(self.casts,1);gap("cast_queue_full")end
    self.casts[#self.casts+1]={guid=guid,id=castGUID,spell=spellID,at=GetTime()}
    gap("readable_cast")
end
function observations:Body(budget,source)
    if not self:Enabled() then self.casts={};self:StopCastCaptureTest();watch:Hide();screen:Hide();return nil end
    local now=GetTime();local lines={};local used=0
    local function add(line)if #line+used+1>budget then return false end;lines[#lines+1]=line;used=used+#line+1;return true end
    if source~="kick" and not LUI.ForeverEngine then
        local _,kind=IsInInstance();local mode=kind=="arena" and 1 or (kind=="pvp" and 2 or 0)
        if self.contextMode~=mode or not self.contextEpoch then
            self.contextMode=mode;self.contextRound=(self.contextRound or 0)+1
            self.contextEpoch=table.concat({GetServerTime(),math.floor(now*1000),self.contextRound},":")
        end
        add(table.concat({"P",1,mode,GetServerTime(),self.contextEpoch},"|"))
    end
    local watchLine=source~="kick" and self:AuraWatchLine()
    if watchLine and not add(watchLine)then watch:Hide();gap("aura_watch_budget")end
    local screenLine=source~="cd" and self:ScreenCastLine()
    if screenLine and not add(screenLine)then screen:Hide();gap("screen_cast_budget")end
    local kept={}
    for _,c in ipairs(self.casts)do
        if now-c.at<=30 and now>=c.at and self:Unit(c.guid) and self:SpellEnabled(c.spell) then kept[#kept+1]=c end
    end
    self.casts=kept
    -- Hold a page long enough for the 250ms Windows sampler, repeat for loss recovery.
    local page=math.floor(now/0.75)
    for i=1,math.min(2,#kept)do
        local c=kept[(page*2+i-1)%#kept+1]
        local interrupt=LUI.InterruptSpells and LUI.InterruptSpells[c.spell]
        if not source or (source=="kick" and interrupt) or (source=="cd" and not interrupt)then add(table.concat({"C",c.id,c.guid,c.spell,math.floor((now-c.at)*1000)},"|"))end
    end
    if source~="cd" and not LUI.ForeverEngine then add(self:ScreenStatusLine())end
    if LUI.InspectTalents then
        local line=LUI.InspectTalents:BridgeLine(page,math.max(0,budget-used-1))
        if line then add(line)end
    end
    local d=self.diagnostics;local i=LUI.InspectTalents and LUI.InspectTalents.diagnostics or {}
    add(table.concat({"D",d.readable_cast or 0,d.restricted_cast or 0,d.cast_queue_full or 0,i.completed or 0,i.failed or 0},"|"))
    if #lines>0 then return table.concat(lines,"\n")end
end
function observations:Diagnostic()
    local d=self.diagnostics;local i=LUI.InspectTalents and LUI.InspectTalents.diagnostics or {}
    print("Lamda tracking: readable casts "..(d.readable_cast or 0)..", restricted casts "..(d.restricted_cast or 0)..", inspect results "..(i.completed or 0)..", inspect failures "..(i.failed or 0)..".")
    print("Cast capture: restricted unit "..(d.restricted_unit or 0)..", restricted spell "..(d.restricted_spell or 0)..", local event IDs "..(d.local_cast_identity or 0)..".")
    print("Buff-start capture: "..(watch.line and "configured" or "inactive")..", setup failures "..(d.aura_watch_unavailable or 0)..".")
    print("Kick screen capture: "..(screen.line and "configured" or "inactive")..", displayed "..(d.screen_cast_displayed or 0)..", errors "..((d.screen_setup_unavailable or 0)+(d.screen_cast_unavailable or 0))..".")
    if screen.error then print("Kick screen setup: "..screen.error)end
    if screen.meterMode then print("Landed-kick meter: "..(screen.meterError or "enabled")..". Missed kicks still need a cast/log record.")end
    if screen.waitingForFont then print("Kick screen setup: waiting for font metrics (width "..tostring(screen.fontWidth or "unavailable")..").")end
    print("Live events require the event strip and WoW in front. Missed screen casts use the combat-log fallback. Timers remain estimates.")
end
local frame=CreateFrame("Frame")
frame:RegisterEvent("UNIT_SPELLCAST_SUCCEEDED")
for _,event in ipairs({"PVP_MATCH_ACTIVE","PVP_MATCH_COMPLETE"})do pcall(frame.RegisterEvent,frame,event)end
frame:SetScript("OnEvent",function(_,event,...)
    if event~="UNIT_SPELLCAST_SUCCEEDED"then observations.contextEpoch=nil;return end
    local ok=pcall(observations.Cast,observations,...);if not ok then gap("cast_unavailable")end
end)
