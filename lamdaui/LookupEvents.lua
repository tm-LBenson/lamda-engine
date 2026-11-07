local _,LUI=...
local lookup={diagnostics={},sequence=0,id="cd",xKey="lookupBridgeX",yKey="lookupBridgeY",width=128,height=16,strips={},moveTogether=true}
LUI.LookupEvents=lookup
local function gap(reason)lookup.diagnostics[reason]=(lookup.diagnostics[reason] or 0)+1 end
local function plain(value)
    if issecretvalue and issecretvalue(value) then return nil end
    if type(value)~="string" or #value==0 or #value>128 or value:find("[|\r\n]") then return nil end
    return value
end
local function number(value)
    if issecretvalue and issecretvalue(value) then return nil end
    if type(value)~="number" or value~=value or value==math.huge or value==-math.huge then return nil end
    return value
end
function lookup:Enabled()
    local db=LUI:DB()
    if self.id=="kick" then return db.kickEnabled end
    if self.id=="gladius" then return not LUI.ForeverEngine and db.gladiusEnabled and db.gladiusEngine end
    if self.id=="kbb" then return db.kbbEnabled end
    if self.id=="overlays" then return db.overlaysEnabled end
    if self.id=="sim" then return db.simEnabled end
    if self.id=="augmentation" then return not LUI.ForeverEngine and db.augEnabled and db.meterCaptureEnabled==true end
    return LUI:CDTrackingEnabled(db)
end
local function enabled()
    local db=LUI:DB()
    return lookup:Enabled() or db.kickEnabled or (not LUI.ForeverEngine and (db.augEnabled or (db.gladiusEnabled and db.gladiusEngine)))
end
-- Client-relative physical pixels, shared with the Windows capture rectangle.
-- Keep every data tile opaque and unscaled, including reshaped Overlays tiles.
function lookup:ClampPosition(x,y)
    local width,height=GetPhysicalScreenSize()
    return math.max(0,math.min(math.floor(x+0.5),16000,width-self.width)),math.max(0,math.min(math.floor(y+0.5),16000,height-self.height))
end
function lookup:GetPosition()
    local preview=rawget(self,"dragPosition");if preview then return preview[1],preview[2]end
    local db=LUI:DB();return self:ClampPosition(db[self.xKey] or self.defaultX or 0,db[self.yKey] or self.defaultY or 0)
end
function lookup:ManagedStrips()
    local result={}
    for _,id in ipairs({"cd","kick","gladius","kbb","sim","overlays","augmentation"})do
        for _,module in ipairs(LUI.modules)do
            if module.id==id then result[#result+1]=self:GetStrip(id);break end
        end
    end
    return result
end
function lookup:MoveAll()
    if LUI:CombatManagementBlocked()then return end
    for _,strip in ipairs(self:ManagedStrips())do if strip:Enabled()then strip:SetMoving(true)end end
end
function lookup:FinishGroupDrag(cancel)
    local drag=rawget(self,"groupDrag");if not drag then return end
    self.groupDrag=nil;drag.owner.handle:SetScript("OnUpdate",nil)
    cancel=cancel or drag.profile~=LUI:ProfileDB().active
    local db=LUI:DB()
    for _,item in ipairs(drag.items)do
        local strip=item.strip;local position=rawget(strip,"dragPosition")
        if position and not cancel then db[strip.xKey],db[strip.yKey]=position[1],position[2]end
        strip.dragPosition=nil
    end
    for _,item in ipairs(drag.items)do item.strip:ApplyPosition()end
end
function lookup:UpdateGroupDrag()
    local drag=self.groupDrag;if not drag then return end
    if drag.profile~=LUI:ProfileDB().active or LUI:CombatManagementBlocked()then self:FinishGroupDrag(true);return end
    local x,y=GetCursorPosition()
    local dx=math.max(drag.minX,math.min(math.floor(x-drag.x+.5),drag.maxX))
    local dy=math.max(drag.minY,math.min(math.floor(drag.y-y+.5),drag.maxY))
    for _,item in ipairs(drag.items)do item.strip.dragPosition={item.x+dx,item.y+dy};item.strip:ApplyPosition()end
end
function lookup:BeginDrag()
    if not self.moving or self.resizing or LUI:CombatManagementBlocked()then return end
    lookup:FinishGroupDrag()
    local items={}
    if lookup.moveTogether then
        for _,strip in ipairs(lookup:ManagedStrips())do
            if strip.resizing then return end
            if strip.moving then local x,y=strip:GetPosition();items[#items+1]={strip=strip,x=x,y=y}end
        end
    end
    if #items<2 then self.dragging=true;self.frame:StartMoving();return end
    local width,height=GetPhysicalScreenSize();local x,y=GetCursorPosition()
    local drag={items=items,owner=self,profile=LUI:ProfileDB().active,x=x,y=y,minX=-math.huge,minY=-math.huge,maxX=math.huge,maxY=math.huge}
    for _,item in ipairs(items)do
        local w,h=item.strip:Footprint()
        drag.minX=math.max(drag.minX,-item.x);drag.minY=math.max(drag.minY,-item.y)
        drag.maxX=math.min(drag.maxX,16000-item.x,width-w-item.x)
        drag.maxY=math.min(drag.maxY,16000-item.y,height-h-item.y)
    end
    -- Edge-to-edge saved layouts may need the native bands to flip sides, as
    -- they do during individual moves. Always keep the packet rectangles in view.
    if drag.minX>drag.maxX then
        drag.maxX=math.huge
        for _,item in ipairs(items)do drag.maxX=math.min(drag.maxX,16000-item.x,width-item.strip.width-item.x)end
    end
    if drag.minY>drag.maxY then
        drag.maxY=math.huge
        for _,item in ipairs(items)do drag.maxY=math.min(drag.maxY,16000-item.y,height-item.strip.height-item.y)end
    end
    lookup.groupDrag=drag
    self.handle:SetScript("OnUpdate",function()lookup:UpdateGroupDrag()end)
end
-- Reserve the native bands, visibility button and resize grip when packing.
function lookup:Footprint()
    local db=LUI:DB()
    if self.id=="cd" then return 208,24 end
    if self.id=="kick" then return 128,db.castCaptureMode==2 and 200 or 40 end
    if self.id=="gladius" then return 128,db.castCaptureMode==2 and 200 or 40 end
    if self.id=="overlays" then return self.width+20,self.height+20 end
    if self.id=="augmentation" then
        if LUI.MeterBorder and LUI.MeterBorder.layout then local b=LUI.MeterBorder.layout;return b.width,b.height end
        return 256,LUI.MeterBridge and LUI.MeterBridge.root and (32+LUI.MeterBridge.root:GetHeight()) or 144
    end
    return self.width,self.height
end
function lookup:Arrange(direction)
    if LUI:CombatManagementBlocked()then return false end
    self:FinishGroupDrag()
    local items={};local width,height=GetPhysicalScreenSize()
    local x,y,line,totalWidth,totalHeight=0,0,0,0,0;local originX,originY=16000,16000
    for _,strip in ipairs(self:ManagedStrips())do
        if strip.moving or strip:Enabled()then
            strip:FinishResize();strip:FinishDrag();strip:ApplyShape()
            local w,h=strip:Footprint();if w>width or h>height then return false end
            if direction=="Row" then
                if x+w>width then x=0;y=y+line+8;line=0 end
            else
                if y+h>height then y=0;x=x+line+8;line=0 end
            end
            if x+w>width or y+h>height then return false end
            items[#items+1]={strip=strip,x=x,y=y}
            totalWidth=math.max(totalWidth,x+w);totalHeight=math.max(totalHeight,y+h)
            local sx,sy=strip:GetPosition();originX=math.min(originX,sx);originY=math.min(originY,sy)
            if direction=="Row"then x=x+w+8;line=math.max(line,h)else y=y+h+8;line=math.max(line,w)end
        end
    end
    originX=math.min(originX,width-totalWidth);originY=math.min(originY,height-totalHeight)
    local db=LUI:DB()
    for _,item in ipairs(items)do
        item.strip:SetMoving(true)
        db[item.strip.xKey],db[item.strip.yKey]=originX+item.x,originY+item.y;item.strip:ApplyPosition()
    end
    return true
end
function lookup:PositionResizeGrip()
    if not self.grip then return end
    local x,y=self:GetPosition();local width,height=GetPhysicalScreenSize()
    local size,gap=16,4
    -- Prefer just outside the bottom-right corner; use a free side at screen edges.
    local positions={{self.width+gap,self.height+gap},{self.width+gap,self.height-size},
        {self.width-size,self.height+gap},{-size-gap,self.height-size},{self.width-size,-size-gap}}
    for _,p in ipairs(positions)do
        if x+p[1]>=0 and y+p[2]>=0 and x+p[1]+size<=width and y+p[2]+size<=height then
            self.grip:ClearAllPoints();self.grip:SetPoint("TOPLEFT",self.frame,"TOPLEFT",p[1],-p[2]);return
        end
    end
end
function lookup:ApplyPosition()
    self:ApplyShape()
    if not self.frame or self.dragging then return end
    local x,y=self:GetPosition()
    -- Blizzard PixelUtil includes the physical-screen factor as well as UI scale.
    self.frame:SetScale(PixelUtil.GetPixelToUIUnitFactor()/UIParent:GetEffectiveScale())
    self.frame:ClearAllPoints();self.frame:SetPoint("TOPLEFT",UIParent,"TOPLEFT",x,-y)
    self:PositionResizeGrip()
    if not self.minimizeButton then return end
    self.minimizeButton:SetScale(PixelUtil.GetPixelToUIUnitFactor()/UIParent:GetEffectiveScale())
    self.minimizeButton:ClearAllPoints()
    local width,height=GetPhysicalScreenSize()
    local buttonWidth,buttonHeight=self.minimizeButton:GetWidth(),self.minimizeButton:GetHeight()
    local edge=y+buttonHeight>height and "BOTTOM" or "TOP"
    if x+128+2+buttonWidth<=width then self.minimizeButton:SetPoint(edge.."LEFT",self.frame,edge.."RIGHT",2,0)
    elseif x>=buttonWidth+2 then self.minimizeButton:SetPoint(edge.."RIGHT",self.frame,edge.."LEFT",-2,0)
    elseif y>=buttonHeight+2 then self.minimizeButton:SetPoint("BOTTOMRIGHT",self.frame,"TOPRIGHT",0,2)
    else self.minimizeButton:SetPoint("TOPRIGHT",self.frame,"BOTTOMRIGHT",0,-2)end
end
function lookup:RefreshOverlayButton()
    local button=self.minimizeButton;if not button then return end
    local db=LUI:DB()
    button:SetShown(enabled() or db.overlaysEnabled or db.kbbEnabled or db.simEnabled)
    local hidden=LUI.overlayHidden
    button:SetText(hidden and "Hidden" or "Shown")
    button.icon:SetTexture("Interface\\AddOns\\LamdaFrames\\Media\\Icons\\"..(hidden and "visibility_off" or "visibility")..".tga")
    if hidden then
        button:SetBackdropColor(.23,.14,.04,1);button:SetBackdropBorderColor(.85,.55,.17,1)
        button.icon:SetVertexColor(1,.77,.36,1);button.label:SetTextColor(1,.81,.50,1)
    else
        button:SetBackdropColor(.05,.21,.24,1);button:SetBackdropBorderColor(.2,.75,.84,1)
        button.icon:SetVertexColor(.4,.95,1,1);button.label:SetTextColor(.8,.97,1,1)
    end
    button.helpTitle=hidden and "Engine displays hidden" or "Engine displays shown"
    button.helpText=hidden and "Click to show color strips and engine displays. Screen-fed tracking and transfers resume." or "Click to hide color strips and engine displays. Screen-fed tracking and transfers pause. Choose Start hidden under General → Color strips to use this at login."
    if button.hovered then button:explain()end
end
function lookup:SetOverlayHidden(value)
    LUI.overlayHidden=not not value
    if value then
        self:LockStrips()
        if self.frame then self.frame:Hide()end
        for _,strip in pairs(self.strips)do strip.frame:Hide()end
        local o=LUI.Observations
        if o and o.auraWatch then o.auraWatch:Hide()end
        if o and o.screenCasts then o.screenCasts:Hide()end
        if LUI.Gladius and LUI.Gladius.tracking then LUI.Gladius.tracking:Hide()end
        if LUI.Overlays and LUI.Overlays.root then LUI.Overlays.root:Hide()end
        if LUI.Sim then LUI.Sim:Stop("Color strips hidden. Show them before sending again.")end
    end
    if LUI.Kick then LUI.Kick:FinishDrag();LUI.Kick:RefreshSettings()end
    self:RefreshOverlayButton()
    self:Render()
end
function lookup:FinishDrag()
    lookup:FinishGroupDrag()
    if not self.dragging then return end
    self.frame:StopMovingOrSizing();self.dragging=false
    local factor=PixelUtil.GetPixelToUIUnitFactor()
    local scale=self.frame:GetEffectiveScale()/factor;local parentScale=UIParent:GetEffectiveScale()/factor
    local x=self.frame:GetLeft()*scale-UIParent:GetLeft()*parentScale
    local y=UIParent:GetTop()*parentScale-self.frame:GetTop()*scale
    local db=LUI:DB();db[self.xKey],db[self.yKey]=self:ClampPosition(x,y)
    self:ApplyPosition()
end
function lookup:SetMoving(value,all)
    if value and LUI.overlayHidden then lookup:SetOverlayHidden(false)end
    if all and self==lookup then
        for _,strip in pairs(self.strips)do strip:SetMoving(value and strip:Enabled())end
    end
    if not self.frame then return end
    value=value and not LUI:CombatManagementBlocked() or false
    if not value then self:FinishResize();self:FinishDrag()end
    local wasMoving=self.moving
    if value and not self.moving then
        if self.id=="sim" and LUI.Sim then LUI.Sim:Stop("Color strip unlocked. Apply & Reload after moving.")end
        -- A non-packet preview prevents moving colors from becoming observations.
        for _,cell in ipairs(self.cells)do cell:SetColorTexture(0,0,0,1)end
        self.rendered=false
        local o=LUI.Observations
        if self.id=="cd" and o and o.auraWatch then o.auraWatch:Hide()end
        if self.id=="kick" and o and o.screenCasts then o.screenCasts:Hide()end
        if self.id=="gladius" and LUI.Gladius and LUI.Gladius.tracking then LUI.Gladius.tracking:Hide()end
    end
    self.moving=value;self.frame:EnableMouse(value);self.handle:EnableMouse(value);self.handle:SetShown(value)
    if self.grip then self.grip:SetShown(value)end
    if self.id=="overlays" and LUI.Overlays and LUI.Overlays.root then LUI.Overlays.root:SetShown(not LUI.overlayHidden and not value and self:Enabled())end
    if value then self.frame:Show()
    elseif (self.wide and wasMoving) or not self:Enabled() or not self.packet then self.frame:Hide()end
    self.moveProfile=value and LUI:ProfileDB().active or nil
end
function lookup:RefreshStrip()
    if not self.frame then return end
    if LUI.overlayHidden then self.frame:Hide();return end
    if self.moving and self.moveProfile~=LUI:ProfileDB().active then
        self.frame:StopMovingOrSizing();self.dragging=false;self:SetMoving(false)
    end
    if LUI:CombatManagementBlocked() then self:SetMoving(false)end
    self:ApplyPosition()
    if not self.wide then self.frame:SetShown(self.moving or (self:Enabled() and not not self.packet))
    elseif not self:Enabled() and not self.moving then self.frame:Hide()end
end
function lookup:RefreshSettings()
    if self.groupDrag and self.groupDrag.profile~=LUI:ProfileDB().active then self:FinishGroupDrag(true)end
    self:RefreshStrip();self:RefreshOverlayButton()
    for _,strip in pairs(self.strips)do strip:RefreshStrip()end
    if not self:Enabled() and LUI.Observations and LUI.Observations.auraWatch then LUI.Observations.auraWatch:Hide()end
    if not LUI:DB().kickEnabled and LUI.Observations and LUI.Observations.screenCasts then LUI.Observations.screenCasts:Hide()end
    if not LUI:DB().gladiusEnabled and LUI.Gladius and LUI.Gladius.tracking then LUI.Gladius.tracking:Hide()end
end
function lookup:LockStrips()
    self:SetMoving(false);for _,strip in pairs(self.strips)do strip:SetMoving(false)end
end
function lookup:ResetPosition()
    if LUI:CombatManagementBlocked() then return end
    self:SetMoving(false)
    local db=LUI:DB();db[self.xKey],db[self.yKey]=LUI:DefaultValue(self.xKey),LUI:DefaultValue(self.yKey)
    if self.id=="overlays"then db.overlaysBridgeWidth=LUI:DefaultValue("overlaysBridgeWidth")end
    self:ApplyPosition()
end

-- A 4x4-tile reflow preserves native glyphs and all packet pixels exactly.
function lookup:MapPixel(x,y)
    local index=math.floor(y/4)*64+math.floor(x/4);local columns=self.width/4
    return (index%columns)*4+x%4,math.floor(index/columns)*4+y%4
end
function lookup:ApplyShape()
    if self.id~="overlays" or not self.frame then return end
    local width=LUI:DB().overlaysBridgeWidth or 256
    if width<16 or width>1152 or width%4~=0 or 18432%width~=0 or (18432/width)%4~=0 then width=256 end
    if self.shapeWidth==width then return end
    self.shapeWidth=width;self.width=width;self.height=18432/width
    self.frame:SetSize(self.width,self.height)
    for i,cell in ipairs(self.cells)do
        local x,y=self:MapPixel(((i-1)%128)*2,math.floor((i-1)/128)*2)
        cell:ClearAllPoints();cell:SetPoint("TOPLEFT",self.frame,"TOPLEFT",x,-y)
    end
    if self.previewCells then
        for i,cell in ipairs(self.previewCells)do
            local x,y=self:MapPixel(((i-1)%64)*4,math.floor((i-1)/64)*4)
            cell:ClearAllPoints();cell:SetPoint("TOPLEFT",self.handle,"TOPLEFT",x,-y)
        end
    end
    if LUI.Overlays then LUI.Overlays:Layout()end
end
function lookup:FinishResize(cancel)
    if not self.resizing then return end
    self.resizing=nil;self.grip:SetScript("OnUpdate",nil)
    local mouse=self.moving and not LUI:CombatManagementBlocked()
    self.frame:EnableMouse(mouse);self.handle:EnableMouse(mouse)
    if cancel then LUI:DB().overlaysBridgeWidth=self.resizeWidth end
    self:ApplyPosition();LUI:RefreshUI()
end
function lookup:ResizeTo(cursorX,cursorY)
    if not self.resizing then return end
    local wantedWidth=math.max(16,self.resizeWidth+cursorX-self.resizeX)
    local wantedHeight=math.max(16,18432/self.resizeWidth+self.resizeY-cursorY)
    local screenWidth,screenHeight=GetPhysicalScreenSize()
    local best,score=self.width,math.huge
    for width=16,math.min(1152,screenWidth),4 do
        local height=18432/width
        if height%4==0 and height<=screenHeight then
            local distance=math.log(width/wantedWidth)^2+math.log(height/wantedHeight)^2
            if distance<score then best=width;score=distance end
        end
    end
    LUI:DB().overlaysBridgeWidth=best;self:ApplyPosition()
end
function lookup:CreateMover(name)
    local handle=CreateFrame("Frame",nil,self.frame);self.handle=handle
    handle:SetAllPoints(self.frame);handle:SetFrameStrata("TOOLTIP");handle:SetFrameLevel(11000);handle:EnableMouse(true);handle:Hide()
    self.previewCells={}
    local count=self.id=="overlays" and 1152 or self.width*self.height/16
    for i=1,count do
        local cell=handle:CreateTexture(nil,"OVERLAY");cell:SetSize(4,4)
        cell:SetPoint("TOPLEFT",handle,"TOPLEFT",((i-1)%(self.width/4))*4,-math.floor((i-1)/(self.width/4))*4)
        -- Match the fine colored texture of the wire image without a valid header.
        cell:SetColorTexture(((i*73)%251)/255,((i*151)%251)/255,((i*193)%251)/255,1)
        self.previewCells[i]=cell
    end
    LUI:Explain(handle,name,"Drag the colors to move."..(self.id=="overlays" and " Drag the gold grip to resize. Apply & Reload when finished." or " Apply & Reload when finished."))
    if self.id=="overlays" then
        local grip=CreateFrame("Button",nil,self.frame);self.grip=grip
        grip:SetSize(16,16);grip:SetPoint("TOPLEFT",self.frame,"BOTTOMRIGHT",4,-4)
        grip:SetFrameStrata("TOOLTIP");grip:SetFrameLevel(11010);grip:EnableMouse(true);grip:Hide()
        -- Keep the artwork and hit area on the same button, outside the move area.
        local edge=grip:CreateTexture(nil,"BACKGROUND");edge:SetAllPoints(grip);edge:SetColorTexture(1,.82,.2,1)
        local fill=grip:CreateTexture(nil,"ARTWORK")
        fill:SetPoint("TOPLEFT",grip,"TOPLEFT",1,-1);fill:SetPoint("BOTTOMRIGHT",grip,"BOTTOMRIGHT",-1,1);fill:SetColorTexture(.02,.03,.04,1)
        local icon=grip:CreateTexture(nil,"OVERLAY");icon:SetAllPoints(grip)
        icon:SetTexture("Interface\\ChatFrame\\UI-ChatIM-SizeGrabber-Up");icon:SetVertexColor(1,.88,.3,1)
        grip:SetHighlightTexture("Interface\\ChatFrame\\UI-ChatIM-SizeGrabber-Highlight","ADD")
        LUI:Explain(grip,"Resize","Drag to change the shape.")
        local function beginResize(_,button)
            if button~="LeftButton" or self.resizing or not self.moving or LUI:CombatManagementBlocked()then return end
            self:FinishDrag();self.resizing=true;self.resizeWidth=self.width;self.resizeX,self.resizeY=GetCursorPosition()
            self.frame:EnableMouse(false);handle:EnableMouse(false)
            grip:SetScript("OnUpdate",function()if LUI:CombatManagementBlocked()then self:FinishResize(true);return end;self:ResizeTo(GetCursorPosition())end)
        end
        grip:RegisterForDrag("LeftButton")
        grip:SetScript("OnMouseDown",beginResize);grip:SetScript("OnDragStart",beginResize)
        grip:SetScript("OnMouseUp",function()self:FinishResize()end)
        grip:SetScript("OnDragStop",function()self:FinishResize()end)
        grip:SetScript("OnHide",function()self:FinishResize()end)
    end
    return handle
end

function lookup:GetStrip(id)
    if not self.frame then self:Initialize()end
    if id=="cd" then return self end
    if self.strips[id]then return self.strips[id]end
    local names={kick="LamdaKick",gladius="LamdaGladius",kbb="LamdaKBB",sim="LamdaSim",overlays="LamdaOverlays",augmentation="LamdaMeters"}
    assert(names[id],"Unknown color strip")
    local wide=id=="kbb" or id=="sim" or id=="overlays" or id=="augmentation"
    local positions={kick={224,0},gladius={448,0},kbb={0,224},sim={288,224},overlays={0,320},augmentation={288,320}}
    local strip=setmetatable({id=id,xKey=id.."BridgeX",yKey=id.."BridgeY",width=wide and 256 or 128,height=wide and 32 or 16,wide=wide,
        defaultX=positions[id][1],defaultY=positions[id][2],cells={},packet=false,rendered=false,moving=false,dragging=false,minimizeButton=false},{__index=self})
    self.strips[id]=strip
    local frame=CreateFrame("Frame",nil,UIParent);strip.frame=frame
    frame:SetFrameStrata("TOOLTIP");frame:SetFrameLevel(10000);frame:EnableMouse(false)
    frame:SetSize(strip.width,strip.height);frame:SetMovable(true);frame:SetClampedToScreen(true)
    local handle=strip:CreateMover(names[id])
    for _,target in ipairs({frame,handle})do
        target:RegisterForDrag("LeftButton")
        target:SetScript("OnDragStart",function()
            strip:BeginDrag()
        end)
        target:SetScript("OnDragStop",function()strip:FinishDrag();LUI:RefreshUI()end)
    end
    for i=1,(wide and 2048 or 512)do
        local cell=frame:CreateTexture(nil,"OVERLAY");cell:SetSize(2,2)
        cell:SetPoint("TOPLEFT",frame,"TOPLEFT",((i-1)%(strip.width/2))*2,-math.floor((i-1)/(strip.width/2))*2)
        strip.cells[i]=cell
    end
    strip:ApplyPosition();frame:Hide();return strip
end
function lookup:Paint(packet)
    if LUI.overlayHidden then self.frame:Hide();return end
    if self.moving then return end
    self.packet=packet
    for i,texture in ipairs(self.cells)do
        local offset=(i-1)*3
        if not self.rendered or packet:sub(offset+1,offset+3)~=self.rendered:sub(offset+1,offset+3)then
            texture:SetColorTexture(packet:byte(offset+1)/255,packet:byte(offset+2)/255,packet:byte(offset+3)/255,1)
        end
    end
    self.rendered=packet;self.frame:Show()
end
local function roster()
    local members={}
    if not IsInGroup() or IsInRaid() then return members end
    local region=({[1]="us",[2]="kr",[3]="eu",[4]="tw"})[number(GetCurrentRegion()) or 0]
    if not region then gap("region_unavailable");return nil end
    for _,unit in ipairs({"player","party1","party2","party3","party4"})do
        if UnitExists(unit) then
            local guid=plain(UnitGUID(unit))
            if not guid then gap("identity_unavailable");return nil end
            -- Party slots can contain NPC companions. Only players have web
            -- profiles; native frame geometry keeps companions separately.
            if guid:match("^Player%-")then
                local name,realm=UnitFullName(unit)
                if issecretvalue and issecretvalue(realm) then gap("identity_unavailable");return nil end
                name=plain(name);realm=(realm==nil or realm=="") and plain(GetNormalizedRealmName()) or plain(realm)
                if not guid or not name or not realm then gap("identity_unavailable");return nil end
                members[#members+1]=table.concat({guid,region,realm,name},"|")
            end
        end
    end
    table.sort(members);return members
end
local function crc32(text)
    local crc=0xFFFFFFFF
    for i=1,#text do
        crc=bit.bxor(crc,text:byte(i))
        for _=1,8 do
            if bit.band(crc,1)==1 then crc=bit.bxor(bit.rshift(crc,1),0xEDB88320)
            else crc=bit.rshift(crc,1)end
        end
    end
    crc=bit.bnot(crc);if crc<0 then crc=crc+4294967296 end;return crc
end
local function bytes(value,count)
    local out={};for i=count-1,0,-1 do out[#out+1]=string.char(math.floor(value/256^i)%256)end
    return table.concat(out)
end
function lookup:Checksum(body) return crc32(body) end
function lookup:Packet(body,magic)
    local capacity=(magic=="LUS2" or magic=="LUR2" or magic=="LUK2" or magic=="LUO1" or magic=="LUM1") and 6134 or 1526
    if #body>capacity then gap("packet_too_large");return nil end
    return (magic or "LUI1")..bytes(#body,2)..bytes(crc32(body),4)..body..string.rep("\0",capacity-#body)
end
function lookup:Publish(kind,remaining)
    if not enabled() then return end
    -- Local cast matching needs party identities even when web lookups are off.
    local members=roster();if not members then return end
    local at=number(GetServerTime());if not at then gap("clock_unavailable");return end
    self.epoch=self.epoch or tostring(at)..":"..tostring(math.floor(GetTime()*1000))
    self.sequence=self.sequence+1
    local deadline=remaining and math.floor(at+remaining) or 0
    if kind=="challenge_start" then deadline=self.challengeAt or 0 end
    local lines={table.concat({self.epoch..":"..self.sequence,tostring(at),kind,tostring(deadline)},"|")}
    for _,member in ipairs(members)do lines[#lines+1]=member end
    self.body=table.concat(lines,"\n")
    self:Render()
end
function lookup:Render()
    if LUI.overlayHidden then return end
    if not enabled() or not self.body then return end
    if not self.frame then self:Initialize()end
    local head,tail=self.body:match("^([^\n]+)(.*)$")
    local event=head.."|"..(LUI:DB().keepUIWhenUnfocused and "1" or "0")..tail
    for _,id in ipairs({"cd","kick"})do
        local strip=self:GetStrip(id);strip.body=self.body
        if strip:Enabled() and not strip.moving then
            local geometry,observations
            local provider
            if id=="cd" then provider=LUI.FrameBridge else provider=LUI.Kick end
            if provider then
                local ok,result=pcall(provider.Body,provider,math.min(750,1526-#event-350))
                if ok then geometry=result else gap(id.."_snapshot_unavailable")end
            end
            if LUI.Observations then
                local ok,result=pcall(LUI.Observations.Body,LUI.Observations,1526-#event-#(geometry or "")-6,id)
                if ok then observations=result else gap("observation_snapshot_unavailable")end
            end
            local body=event.."\n\n"..(id=="cd" and geometry or "").."\n\n"..(observations or "").."\n\n"..(id=="kick" and geometry or "")
            local magic=id=="cd" and "LCD1" or "LKI1"
            if LUI.ForeverEngine then magic=id=="cd" and "FCD1" or "FKI1"end
            local packet=self:Packet(body,magic);if packet then strip:Paint(packet)end
        elseif not strip.moving then strip.packet=false;strip.frame:Hide()end
    end
    local arena=LUI.Gladius and LUI.Gladius.tracking
    if arena then
        local strip=self:GetStrip("gladius")
        local ok,body=pcall(arena.Body,arena)
        if ok and body and not strip.moving then
            local packet=self:Packet(body,"LUA1");if packet then strip:Paint(packet)end
            if arena.inactive then arena.wasActive=false end
        elseif not strip.moving then strip.packet=false;strip.frame:Hide();arena:Hide()end
    end
end
function lookup:Initialize()
    if self.frame then return end
    local frame=CreateFrame("Frame",nil,UIParent);self.frame=frame;self.cells={}
    frame:SetFrameStrata("TOOLTIP");frame:SetFrameLevel(10000);frame:EnableMouse(false)
    frame:SetSize(128,16);frame:SetMovable(true);frame:SetClampedToScreen(true)
    local handle=self:CreateMover("LamdaCD")
    local minimize=LUI:Button(UIParent,"",0,0,78,function()
        self:SetOverlayHidden(not LUI.overlayHidden)
    end)
    self.minimizeButton=minimize;minimize:SetSize(78,22);minimize:EnableMouse(true)
    minimize:SetFrameStrata("TOOLTIP");minimize:SetFrameLevel(10001)
    minimize.icon=minimize:CreateTexture(nil,"OVERLAY");minimize.icon:SetSize(18,18);minimize.icon:SetPoint("LEFT",4,0)
    minimize.label:ClearAllPoints();minimize.label:SetPoint("LEFT",25,0);minimize.label:SetWidth(49)
    minimize.label:SetJustifyH("LEFT");minimize.label:SetFontObject("GameFontHighlightSmall")
    function minimize:explain()
        if not GameTooltip then return end
        GameTooltip:SetOwner(self,"ANCHOR_RIGHT");GameTooltip:SetText(self.helpTitle)
        GameTooltip:AddLine(self.helpText,.85,.9,.95,true);GameTooltip:Show()
    end
    local function leave(button)
        button.hovered=false
        if GameTooltip and GameTooltip:IsOwned(button)then GameTooltip:Hide()end
    end
    minimize:SetScript("OnEnter",function(button)button.hovered=true;button:explain()end)
    minimize:SetScript("OnLeave",leave);minimize:SetScript("OnHide",leave)
    self:RefreshOverlayButton()
    local function start()
        self:BeginDrag()
    end
    local function stop()self:FinishDrag();LUI:RefreshUI()end
    for _,target in ipairs({frame,handle})do
        target:RegisterForDrag("LeftButton");target:SetScript("OnDragStart",start);target:SetScript("OnDragStop",stop)
    end
    self:ApplyPosition()
    for i=1,512 do
        local cell=frame:CreateTexture(nil,"OVERLAY");cell:SetSize(2,2)
        cell:SetPoint("TOPLEFT",frame,"TOPLEFT",((i-1)%64)*2,-math.floor((i-1)/64)*2)
        self.cells[i]=cell
    end
    frame:Hide()
end
function lookup:OnEvent(event,timerType,remaining)
    if event=="PLAYER_LOGIN" then self:Initialize() end
    if event=="UI_SCALE_CHANGED" or event=="DISPLAY_SIZE_CHANGED" then
        self:FinishGroupDrag(true)
        for _,strip in pairs(self.strips)do
            if strip.dragging then strip.frame:StopMovingOrSizing();strip.dragging=false end
            strip:FinishResize(true)
        end
    end
    if (event=="UI_SCALE_CHANGED" or event=="DISPLAY_SIZE_CHANGED") and self.dragging then
        -- The old drag geometry belongs to the previous scale/window.
        self.frame:StopMovingOrSizing();self.dragging=false
    end
    if event=="UI_SCALE_CHANGED" or event=="DISPLAY_SIZE_CHANGED" or
        (event=="PLAYER_REGEN_DISABLED" and LUI:CombatManagementBlocked()) then
        self:LockStrips()
    end
    if event=="UI_SCALE_CHANGED" or event=="DISPLAY_SIZE_CHANGED" or event=="PLAYER_REGEN_DISABLED" then
        self:RefreshSettings()
    end
    if event=="PLAYER_ENTERING_WORLD" or event=="PVP_MATCH_ACTIVE" or event=="PVP_MATCH_COMPLETE" then self.arenaDeadline=nil;self.arenaSent=nil;self.holdUntil=nil;self.prepKind=nil end
    if event=="STOP_TIMER_OF_TYPE" and number(timerType)==0 then self.arenaDeadline=nil;self.arenaSent=nil end
    if not enabled() then self:RefreshSettings();return end
    if event=="CHALLENGE_MODE_START" then
        local inside,kind=IsInInstance()
        local ok,level=pcall(function() return C_ChallengeMode.GetActiveKeystoneInfo() end)
        level=ok and number(level) or nil
        if not inside or kind~="party" or not level or level<=0 then return end
        self.challengeAt=number(GetServerTime())
        if not self.challengeAt then return end
        self.holdUntil=GetTime()+15;self.prepKind="challenge_start"
        if LUI.InspectTalents then LUI.InspectTalents:RequestRefresh("key") end
        self:Publish("challenge_start")
    elseif event=="CHALLENGE_MODE_COMPLETED" or event=="CHALLENGE_MODE_RESET" or event=="PLAYER_ENTERING_WORLD" then
        self.challengeAt=nil;self.holdUntil=GetTime()+4;self.prepKind="challenge_end"
        self:Publish("challenge_end")
    elseif event=="START_TIMER" then
        timerType,remaining=number(timerType),number(remaining)
        if not timerType or not remaining or remaining<=0 or remaining>600 then gap("countdown_unavailable");return end
        local inside,kind=IsInInstance()
        if inside and kind=="arena" and timerType==0 then
            local deadline=GetTime()+remaining
            if not self.arenaDeadline or math.abs(deadline-self.arenaDeadline)>2 then self.arenaSent=nil end
            self.arenaDeadline=deadline
        elseif inside and kind=="party" and timerType==1 and not self.challengeAt then
            self.holdUntil=GetTime()+remaining;self.prepKind="key_start"
            self:Publish("key_start",math.min(15,remaining))
        end
    elseif event=="CHALLENGE_MODE_KEYSTONE_SLOTTED" then
        -- The actual key start owns inspection refresh, avoiding duplicate scans.
        self.holdUntil=GetTime()+2;self.prepKind="key_start";self:Publish("key_start",10)
    elseif self.holdUntil and self.holdUntil>GetTime() and self.prepKind then
        self:Publish(self.prepKind,math.min(15,self.holdUntil-GetTime()))
    else self:Publish("roster") end
end
local eventFrame=CreateFrame("Frame")
for _,event in ipairs({"PLAYER_LOGIN","PLAYER_ENTERING_WORLD","GROUP_ROSTER_UPDATE","START_TIMER","STOP_TIMER_OF_TYPE","CHALLENGE_MODE_KEYSTONE_SLOTTED","PVP_MATCH_ACTIVE","PVP_MATCH_COMPLETE","UI_SCALE_CHANGED","DISPLAY_SIZE_CHANGED","PLAYER_REGEN_DISABLED"})do eventFrame:RegisterEvent(event)end
-- Classic/Forever may not expose challenge modes at all.
for _,event in ipairs({"CHALLENGE_MODE_START","CHALLENGE_MODE_COMPLETED","CHALLENGE_MODE_RESET"})do pcall(eventFrame.RegisterEvent,eventFrame,event)end
eventFrame:SetScript("OnEvent",function(_,event,...)
    local timing=LUI.Performance and LUI.Performance:BeginWork()
    local ok=pcall(lookup.OnEvent,lookup,event,...);if not ok then gap("event_unavailable")end
    if timing then LUI.Performance:EndWork("Engine event strip",timing)end
end)
local elapsed=0
eventFrame:SetScript("OnUpdate",function(_,delta)
    elapsed=elapsed+delta;if elapsed<0.2 then return end;elapsed=0
    local timing=LUI.Performance and LUI.Performance:BeginWork()
    if not enabled() then
        -- The settings UI/event path handles position and drag changes. A hidden
        -- bridge need not revalidate settings and reapply geometry on every tick.
        local visible=lookup.frame and lookup.frame:IsShown() and not lookup.moving
        for _,strip in pairs(lookup.strips)do visible=visible or (strip.frame:IsShown() and not strip.moving)end
        if visible then lookup:RefreshSettings()end
        if timing then LUI.Performance:EndWork("Engine event strip",timing)end
        return
    end
    local ok=pcall(function()
        local now=GetTime()
        if lookup.arenaDeadline and not lookup.arenaSent and now>=lookup.arenaDeadline-10 then
            lookup.arenaSent=true
            if lookup.arenaDeadline>now then lookup.holdUntil=lookup.arenaDeadline;lookup.prepKind="arena_ready";lookup:Publish("arena_ready",lookup.arenaDeadline-now)
            else gap("arena_window_missed")end
        end
        if not lookup.heartbeat or now-lookup.heartbeat>=2 then
            if lookup.holdUntil and now<=lookup.holdUntil then
                if lookup.prepKind=="challenge_start" or lookup.prepKind=="challenge_end" then
                    lookup.heartbeat=now;lookup:Publish(lookup.prepKind)
                end
            else lookup.heartbeat=now;lookup:Publish("roster") end
        end
        lookup:Render()
    end)
    if not ok then gap("event_unavailable")end
    if timing then LUI.Performance:EndWork("Engine event strip",timing)end
end)
