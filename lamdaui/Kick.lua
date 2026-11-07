-- Own cooldowns render natively. Only public identity/layout ownership is
-- exported; duration objects and their secret values never leave native sinks.
local _,LUI=...
local K={sequence=0,discovered={},iconHandles={}}
LUI.Kick=K
local units={"player","party1","party2","party3","party4"}
local function plain(value,kind)
    if issecretvalue and issecretvalue(value)then return nil end
    if type(value)~=kind then return nil end
    if kind=="number" and (value~=value or value==math.huge or value==-math.huge)then return nil end
    return value
end
local function call(fn,kind,...)
    if type(fn)~="function"then return nil end
    local ok,value=pcall(fn,...);if ok then return plain(value,kind)end
end
local function guid(unit)
    local value=call(UnitGUID,"string",unit)
    if value and #value<=108 and value:match("^[%a]+%-[%w%-]+$")then return value end
end
local function playerGUID(unit)
    local value=guid(unit)
    if value and value:match("^Player%-")then return value end
end
local function round(n)return math.floor(n+.5)end
function K:ContextEnabled(db)
    if not db.kickEnabled then return false end
    if self.preview and not LUI:CombatManagementBlocked()then return true end
    if IsInRaid()then return false end
    local _,kind=IsInInstance()
    if kind=="raid" or kind=="pvp"then return false end
    if not IsInGroup() and not db.kickSolo then return false end
    if kind=="party" or kind=="scenario" then return db.kickDungeon end
    if kind=="arena"then return db.kickArena end
    return db.kickWorld
end
local function screen()
    local factor=PixelUtil.GetPixelToUIUnitFactor()
    local parentScale=UIParent:GetEffectiveScale()
    local width,height=GetPhysicalScreenSize()
    return parentScale/factor,width,height
end
local function position(frame)
    local factor=PixelUtil.GetPixelToUIUnitFactor()
    local scale=frame:GetEffectiveScale()/factor;local parentScale=UIParent:GetEffectiveScale()/factor
    return frame:GetLeft()*scale-UIParent:GetLeft()*parentScale,
        UIParent:GetTop()*parentScale-frame:GetTop()*scale
end
local function place(frame,x,y,w,h)
    frame:SetScale(PixelUtil.GetPixelToUIUnitFactor()/UIParent:GetEffectiveScale())
    frame:ClearAllPoints();frame:SetPoint("TOPLEFT",UIParent,"TOPLEFT",x,-y);frame:SetSize(w,h)
end
local function barLayout(db,scale,width,height)
    local w,h=round(db.kickWidth*scale),round(db.kickHeight*scale)
    local total=24+5*(h+3)-3
    return math.max(db.kickCounts and 72 or 0,math.min(round(db.kickX*scale),width-w)),
        math.max(0,math.min(round(db.kickY*scale),height-total)),w,h,total
end
function K:NewHandle(kind)
    local f=CreateFrame("Frame",nil,UIParent,"BackdropTemplate");f.kickKind=kind
    f:SetFrameStrata("HIGH");f:SetMovable(true);f:SetClampedToScreen(true);f:RegisterForDrag("LeftButton")
    f:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
    f:SetBackdropColor(.05,.2,.25,.25);f:SetBackdropBorderColor(.2,.8,.9,1)
    if kind=="bars"then
        local label=f:CreateFontString(nil,"OVERLAY","GameFontNormal");label:SetPoint("TOP",0,-4);label:SetText("LamdaKick")
    end
    f:SetScript("OnDragStart",function()
        local db=LUI:DB()
        if not self.moving or db.kickLocked or LUI:CombatManagementBlocked() or not f:IsShown()then return end
        local x,y=position(f);local scale=screen()
        self.drag={frame=f,x=x,y=y,scale=scale,offsetX=db.kickOffsetX,offsetY=db.kickOffsetY,
            profile=LUI:ProfileDB().active,mode=db.kickMode}
        self.dragging=true;f:StartMoving()
    end)
    f:SetScript("OnDragStop",function()self:FinishDrag();LUI:RefreshUI()end)
    f:Hide();return f
end
function K:CancelDrag()
    if self.drag then self.drag.frame:StopMovingOrSizing()end
    self.drag=nil;self.dragging=false
end
function K:UpdateDrag(db)
    local drag=self.drag;if not drag then return end
    local scale=screen()
    if drag.profile~=LUI:ProfileDB().active or drag.mode~=db.kickMode or math.abs(scale-drag.scale)>.001 then
        self:CancelDrag();return
    end
    local x,y=position(drag.frame)
    if drag.frame.kickKind=="bars"then
        db.kickX=math.max(0,math.min(16000,round(x/scale)))
        db.kickY=math.max(0,math.min(16000,round(y/scale)))
    else
        db.kickOffsetX=math.max(-200,math.min(200,round(drag.offsetX+(x-drag.x)/scale)))
        db.kickOffsetY=math.max(-200,math.min(200,round(drag.offsetY-(y-drag.y)/scale)))
    end
end
function K:FinishDrag()
    if not self.drag then return end
    self:UpdateDrag(LUI:DB());self:CancelDrag()
    self:RefreshSettings()
end
function K:RefreshSettings()
    local db=LUI:DB()
    if self.nativeBar and (not db.kickEnabled or not db.kickSelf or LUI.overlayHidden or not self:ContextEnabled(db))then
        self.nativeBar:Hide();self.nativeIcon:Hide()
    end
    if not self.handle then self.handle=self:NewHandle("bars")end
    local profile=self.moveProfile or self.previewProfile
    if profile and profile~=LUI:ProfileDB().active then
        self:CancelDrag();self.preview=false;db.kickLocked=true
    end
    if not db.kickEnabled or LUI:CombatManagementBlocked()then
        self:CancelDrag();self.preview=false;db.kickLocked=true
    end
    if not self.preview then self.previewProfile=nil end
    if self.drag and self.drag.mode~=db.kickMode then self:CancelDrag()end
    if db.kickLocked and self.drag then self:FinishDrag()end
    self.moving=not db.kickLocked and db.kickEnabled and not LUI:CombatManagementBlocked() and not LUI.overlayHidden
    self.moveProfile=self.moving and LUI:ProfileDB().active or nil
    local scale,width,height=screen()
    local x,y,w,_,total=barLayout(db,scale,width,height)
    if not self.drag or self.drag.frame~=self.handle then place(self.handle,x,y,w,total)end
    self.handle:EnableMouse(self.moving and db.kickMode~=1)
    self.handle:SetShown(self.moving and db.kickMode~=1)
    -- Body refreshes current icon positions every packet. Hide old handles now
    -- when locking or changing modes, including before the next bridge sample.
    if not self.moving or db.kickMode==2 then
        for _,f in pairs(self.iconHandles)do f:EnableMouse(false);f:Hide()end
    end
end
function K:SetMoving(value)
    if not value then self:FinishDrag()end
    LUI:DB().kickLocked=not (value and LUI:DB().kickEnabled and not LUI:CombatManagementBlocked())
    self:RefreshSettings()
end
function K:SetPreview(value)
    if value and (LUI:CombatManagementBlocked() or not LUI:DB().kickEnabled)then return end
    self.preview=not not value
    self.previewProfile=self.preview and LUI:ProfileDB().active or nil
    self:SetMoving(self.preview)
    if LUI.FrameAuras then LUI.FrameAuras:RequestRefresh()end
end
function K:StopPreview()self:SetPreview(false)end
function K:SyncHandles(db,rects,scale,width,height,mode)
    if not self.moving then return end
    local x,y,w,_,total=barLayout(db,scale,width,height)
    if not self.drag or self.drag.frame~=self.handle then place(self.handle,x,y,w,total)end
    local seen={};local size=round(db.kickSize*scale);local gap=round(db.kickGap*scale)
    if mode==1 or mode==3 then for id,r in pairs(rects)do
        if id~=guid("player") or db.kickSelf then
            x,y=r[1],r[2]+r[4]+gap
            if db.kickAnchor==2 then y=r[2]-size-gap
            elseif db.kickAnchor==3 then x=r[1]-size-gap;y=r[2]+math.modf((r[4]-size)/2)
            elseif db.kickAnchor==4 then x=r[1]+r[3]+gap;y=r[2]+math.modf((r[4]-size)/2)end
            x=x+round(db.kickOffsetX*scale);y=y-round(db.kickOffsetY*scale)
            if x>=0 and y>=0 and x+size<=width and y+size<=height then
                local f=self.iconHandles[id]
                if not f then f=self:NewHandle("icon");self.iconHandles[id]=f end
                seen[id]=true
                if not self.drag or self.drag.frame~=f then place(f,x,y,size,size)end
                f:EnableMouse(true);f:Show()
            end
        end
    end end
    for id,f in pairs(self.iconHandles)do if not seen[id]then
        if self.drag and self.drag.frame==f then self:CancelDrag()end
        f:EnableMouse(false);f:Hide()
    end end
end
function K:RefreshOwnSpell(db)
    if not self.ownDirty and self.ownSelection==db.kickSpells then return end
    self.ownDirty=false;self.ownSelection=db.kickSpells;self.ownSpell=nil;self.ownQuery=nil
    local known=C_SpellBook and C_SpellBook.IsSpellKnown
    if type(known)~="function" and type(IsSpellKnown)~="function"then self.ownResolved=false;return end
    self.ownResolved=true
    for _,id in ipairs(LUI.SpellLists.Parse(db.kickSpells))do
        local learned=call(known,"boolean",id)
        local pet=Enum and Enum.SpellBookSpellBank and Enum.SpellBookSpellBank.Pet
        if learned~=true and pet then learned=call(known,"boolean",id,pet)end
        if learned~=true and type(IsSpellKnown)=="function"then learned=call(IsSpellKnown,"boolean",id)or call(IsSpellKnown,"boolean",id,true)end
        if learned==true then
            local override=call(C_Spell and C_Spell.GetOverrideSpell,"number",id)
            if not override or override<1 or override>10000000 or override~=math.floor(override) then override=id end
            self.ownSpell=id;self.ownQuery=override;return
        elseif learned==nil then self.ownResolved=false end
    end
end
function K:CreateOwn()
    local function create(bar)
        local f=CreateFrame("Frame",nil,UIParent,"BackdropTemplate");f:SetFrameStrata("MEDIUM");f:EnableMouse(false)
        f:SetBackdrop({bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1})
        f:SetBackdropColor(.06,.10,.14,.95);f:SetBackdropBorderColor(.35,.42,.48,1)
        f.art=f:CreateTexture(nil,"ARTWORK");f.art:SetPoint("TOPLEFT",1,-1);f.art:SetTexCoord(.08,.92,.08,.92)
        f.cooldown=CreateFrame("Cooldown",nil,f,"CooldownFrameTemplate");f.cooldown:SetAllPoints(f.art)
        f.cooldown:SetDrawEdge(false);f.cooldown:SetHideCountdownNumbers(true)
        local labels=CreateFrame("Frame",nil,f);labels:SetAllPoints();labels:SetFrameLevel(f.cooldown:GetFrameLevel()+2)
        f.ready=labels:CreateFontString(nil,"OVERLAY","GameFontHighlight");f.ready:SetText(bar and "Ready" or "")
        f.timer=labels:CreateFontString(nil,"OVERLAY","GameFontHighlight")
        f.name=labels:CreateFontString(nil,"OVERLAY","GameFontHighlight");f.detail=labels:CreateFontString(nil,"OVERLAY","GameFontHighlightSmall")
        for _,text in ipairs({f.name,f.detail})do text:SetJustifyH("LEFT");text:SetMaxLines(1);text:SetWordWrap(false)end
        f:Hide();return f
    end
    self.nativeBar=create(true);self.nativeIcon=create(false)
end
function K:RenderOwn(db,rects,scale,width,height,mode)
    self:RefreshOwnSpell(db)
    if self.nativeBar then self.nativeBar:Hide();self.nativeIcon:Hide()end
    if not db.kickSelf or mode==0 or self.preview or LUI.overlayHidden then return 0 end
    if not self.ownSpell then return self.ownResolved and -1 or 0 end
    -- A setting edit in combat may remove the cached spell; honor it immediately.
    if not LUI.SpellLists.Map(db.kickSpells)[self.ownSpell] then return -1 end
    if not self.nativeBar then self:CreateOwn()end
    local bar,icon=self.nativeBar,self.nativeIcon
    local x,y,w,h=barLayout(db,scale,width,height)
    place(bar,x,y+24,w,h);bar:SetShown(mode==2 or mode==3)
    bar.art:SetSize(h-2,h-2);bar:SetAlpha(db.kickOpacity/100)
    bar.name:ClearAllPoints();bar.name:SetPoint("TOPLEFT",h+4,-2);bar.name:SetWidth(math.max(1,w-h-80));bar.name:SetText(call(UnitName,"string","player")or "You")
    bar.name:SetFont(STANDARD_TEXT_FONT,math.min(16,math.max(9,h*.4)))
    bar.detail:ClearAllPoints();bar.detail:SetPoint("BOTTOMLEFT",h+4,2);bar.detail:SetWidth(math.max(1,w-h-80));bar.detail:SetText(LUI.SpellLists.Name(self.ownQuery));bar.detail:SetShown(h>=26)
    for _,text in ipairs({bar.ready,bar.timer})do text:ClearAllPoints();text:SetPoint("RIGHT",-5,0);text:SetWidth(70);text:SetFont(STANDARD_TEXT_FONT,math.min(16,math.max(10,h*.43)),"OUTLINE")end
    local r=rects[guid("player")];local size=round(db.kickSize*scale);local gap=round(db.kickGap*scale)
    if r and (mode==1 or mode==3) then
        x,y=r[1],r[2]+r[4]+gap
        if db.kickAnchor==2 then y=r[2]-size-gap
        elseif db.kickAnchor==3 then x=r[1]-size-gap;y=r[2]+math.modf((r[4]-size)/2)
        elseif db.kickAnchor==4 then x=r[1]+r[3]+gap;y=r[2]+math.modf((r[4]-size)/2)end
        x=x+round(db.kickOffsetX*scale);y=y-round(db.kickOffsetY*scale)
        if x>=0 and y>=0 and x+size<=width and y+size<=height then place(icon,x,y,size,size);icon:Show()end
    end
    icon.art:SetSize(size-2,size-2);icon:SetAlpha(db.kickOpacity/100)
    icon.name:SetText("");icon.detail:SetText("");icon.ready:SetText("")
    icon.timer:ClearAllPoints();icon.timer:SetPoint("CENTER");icon.timer:SetFont(STANDARD_TEXT_FONT,math.max(10,size*.43),"OUTLINE")
    local texture=call(C_Spell and C_Spell.GetSpellTexture,"number",self.ownQuery)or 134400
    local ok,err=pcall(function()
        local duration=C_Spell and C_Spell.GetSpellCooldownDuration and C_Spell.GetSpellCooldownDuration(self.ownQuery,true)
        if duration then
            -- No branches, arithmetic, formatting or serialization in Lua on
            -- these results. WoW's own text/alpha/cooldown setters consume them.
            local zero=duration:IsZero()
            for _,f in ipairs({bar,icon})do
                f.art:SetTexture(texture);f.cooldown:SetCooldownFromDurationObject(duration,true)
                f.ready:SetAlphaFromBoolean(zero,1,0);f.timer:SetAlphaFromBoolean(zero,0,1)
                f.timer:SetFormattedText("%.0f",duration:GetRemainingDuration())
            end
        else
            local start,seconds,rate
            if C_Spell and C_Spell.GetSpellCooldown then
                local info=C_Spell.GetSpellCooldown(self.ownQuery)
                if info then start=plain(info.startTime,"number");seconds=plain(info.duration,"number");rate=plain(info.modRate,"number")end
            elseif GetSpellCooldown then
                local a,b=GetSpellCooldown(self.ownQuery);start=plain(a,"number");seconds=plain(b,"number")
            end
            if not start or not seconds then error("Cooldown API unavailable")end
            local remaining=math.max(0,start+seconds/(rate and rate>0 and rate or 1)-GetTime())
            if seconds<=1.5 then remaining=0 end -- legacy global cooldown
            for _,f in ipairs({bar,icon})do
                f.art:SetTexture(texture);f.cooldown:SetCooldown(start,seconds,rate or 1)
                f.ready:SetAlpha(remaining==0 and 1 or 0);f.timer:SetAlpha(remaining>0 and 1 or 0)
                f.timer:SetText(tostring(math.ceil(remaining)))
            end
        end
    end)
    self.nativeError=not ok and tostring(err)or nil
    if not ok then
        for _,f in ipairs({bar,icon})do f.cooldown:Clear();f.ready:SetAlpha(0);f.timer:SetAlpha(1);f.art:SetTexture(texture)end
        bar.timer:SetText("Unavailable");icon.timer:SetText("!")
    end
    return self.ownSpell
end
function K:Body(budget)
    local db=LUI:DB();local F=LUI.FrameAuras
    local own=playerGUID("player");local at=call(GetServerTime,"number")
    local width,height=GetPhysicalScreenSize();width=plain(width,"number");height=plain(height,"number")
    local factor=call(PixelUtil.GetPixelToUIUnitFactor,"number")
    local ps=call(UIParent.GetEffectiveScale,"number",UIParent)
    local pl=call(UIParent.GetLeft,"number",UIParent);local pt=call(UIParent.GetTop,"number",UIParent)
    if not own or not at or not width or not height or not factor or factor<=0 or not ps or ps<=0 or not pl or not pt then return nil end
    local scale=ps/factor;local mode=self:ContextEnabled(db) and db.kickMode or 0
    -- Empty frame-icon geometry hides both layouts while preserving member and
    -- pet identities for tracking. Mode zero deliberately drops those identities.
    if LUI.overlayHidden and mode>0 then mode=1 end
    if LUI:CombatManagementBlocked()then self.preview=false end
    self:UpdateDrag(db)
    local x,y,barWidth,barHeight=barLayout(db,scale,width,height)
    self.epoch=self.epoch or tostring(at)..":"..tostring(round(GetTime()*1000));self.sequence=self.sequence+1
    local head={self.epoch..":"..self.sequence,at,round(width),round(height),own,mode,db.kickSelf and 1 or 0,
        x,y,barWidth,barHeight,round(db.kickSize*scale),db.kickOpacity,db.kickAnchor,
        round(db.kickGap*scale),round(db.kickOffsetX*scale),round(db.kickOffsetY*scale),db.kickSort,self.preview and 1 or 0,0,db.kickCounts and 1 or 0}
    local rects={};local ranks={}
    if not LUI.overlayHidden and (mode==1 or mode==3) then
        for _,target in ipairs(self.discovered)do
            local f=target.frame;local unit=F:UnitForFrame(f)
            if unit and (unit=="player" or unit:match("^party[1-4]$")) and
                call(f.IsForbidden,"boolean",f)==false and call(f.IsVisible,"boolean",f)==true and
                call(UnitExists,"boolean",unit)==true and call(UnitIsVisible,"boolean",unit)==true then
                local id=playerGUID(unit);local left=call(f.GetLeft,"number",f);local top=call(f.GetTop,"number",f)
                local w=call(f.GetWidth,"number",f);local h=call(f.GetHeight,"number",f);local fs=call(f.GetEffectiveScale,"number",f)
                local rank=f==PlayerFrame and 4 or ((target.provider=="DandersFrames" or target.provider=="LamdaFrames") and 1 or (target.provider=="Blizzard" and 3 or 2))
                if id and left and top and w and h and fs and fs>0 and (not ranks[id] or rank<ranks[id])then
                    local rx,ry=round(left*fs/factor-pl*scale),round(pt*scale-top*fs/factor)
                    w=round(w*fs/factor);h=round(h*fs/factor)
                    if rx>=0 and ry>=0 and w>0 and h>0 and rx+w<=width and ry+h<=height then
                        rects[id]={rx,ry,w,h};ranks[id]=rank
                    end
                end
            end
        end
    end
    self:SyncHandles(db,rects,scale,width,height,mode)
    local nativeOK,nativeSpell=pcall(self.RenderOwn,self,db,rects,scale,width,height,mode)
    head[22]=nativeOK and nativeSpell or 0
    if not nativeOK then
        self.nativeError="Player cooldown display unavailable"
        if self.nativeBar then self.nativeBar:Hide()end;if self.nativeIcon then self.nativeIcon:Hide()end
    end
    local rows={}
    if mode>0 then for _,unit in ipairs(units)do
        local id=playerGUID(unit)
        if id and call(UnitExists,"boolean",unit)==true then
            local ok,_,_,class=pcall(UnitClass,unit);class=ok and plain(class,"number") or nil
            local spec=0;local cache=LUI.InspectTalents and LUI.InspectTalents.cache[id]
            if cache and at>=cache.at and at-cache.at<=300 then spec=cache.spec end
            local pet=unit=="player" and "pet" or "partypet"..unit:sub(6)
            rows[#rows+1]=table.concat({id,class or 0,spec,guid(pet) or "",rects[id] and table.concat(rects[id],",") or ""},"|")
        end
    end end
    local function body()return table.concat(head,"|")..(#rows>0 and "\n"..table.concat(rows,"\n") or "")end
    while #rows>0 and #body()>budget do table.remove(rows);head[20]=head[20]+1 end
    if #body()>budget then return nil end
    return body()
end
local events=CreateFrame("Frame")
events:RegisterEvent("PLAYER_LOGIN");events:RegisterEvent("PLAYER_REGEN_DISABLED")
events:RegisterEvent("UI_SCALE_CHANGED");events:RegisterEvent("DISPLAY_SIZE_CHANGED")
events:RegisterEvent("SPELLS_CHANGED");events:RegisterEvent("PLAYER_SPECIALIZATION_CHANGED");events:RegisterEvent("UNIT_PET");events:RegisterEvent("PLAYER_REGEN_ENABLED")
events:SetScript("OnEvent",function(_,event)
    if event=="SPELLS_CHANGED" or event=="PLAYER_SPECIALIZATION_CHANGED" or event=="UNIT_PET" or event=="PLAYER_LOGIN"then K.ownDirty=true end
    if event=="PLAYER_REGEN_ENABLED" or event=="SPELLS_CHANGED" or event=="PLAYER_SPECIALIZATION_CHANGED" or event=="UNIT_PET"then
        K:RefreshOwnSpell(LUI:DB());return
    end
    if event=="PLAYER_REGEN_DISABLED" and not LUI:CombatManagementBlocked()then K:RefreshSettings();return end
    K:CancelDrag();K:StopPreview()
    if LUI.RefreshUI then LUI:RefreshUI()end
end)
