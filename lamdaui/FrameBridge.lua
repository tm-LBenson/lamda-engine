-- Public frame geometry for engine-owned, frame-attached estimates.
-- Native widget ownership is public presentation metadata, not cooldown state.
-- No aura/cooldown values are queried or serialized here.
local _,LUI=...
local B={sequence=0,diagnostics={}}
LUI.FrameBridge=B
local function plain(value,kind)
    if issecretvalue and issecretvalue(value) then return nil end
    if type(value)~=kind then return nil end
    if kind=="number" and (value~=value or value==math.huge or value==-math.huge) then return nil end
    return value
end
local function call(fn,kind,...)
    if type(fn)~="function" then return nil end
    local ok,value=pcall(fn,...);if ok then return plain(value,kind)end
end
local function guid(unit)
    local value=call(UnitGUID,"string",unit)
    if value and #value<=108 and value:match("^[%a]+%-[%w%-]+$") then return value end
end
local function round(n)return math.floor(n+.5)end
local function flag(value)return value and 1 or 0 end
local function mergeNative(a,b)
    if a=="" then return b end;if b=="" or a==b then return a end
    local seen,ids={},{}
    for id in (a..","..b):gmatch("%d+")do seen[tonumber(id)]=true end
    for id in pairs(seen)do ids[#ids+1]=id end;table.sort(ids)
    return table.concat(ids,",")
end
local function ownSlots(target,db,x,y,w,h,scale,width,height)
    local C=LUI.SelfCooldowns
    if not C or not C.EngineSlotsForFrame then return nil end
    local row=C:EngineSlotsForFrame(target,db)
    if not row then return nil end
    local layout=row.layout
    if not layout then return "" end
    local columns=math.min(layout.perRow,layout.maximum)
    local rows=math.ceil(layout.maximum/layout.perRow)
    local rowWidth=(columns*layout.size+(columns-1)*layout.spacing)*scale
    local rowHeight=(rows*layout.size+(rows-1)*layout.spacing)*scale
    local left,top=x,y+h+layout.gap*scale
    if layout.anchor==2 then top=y-rowHeight-layout.gap*scale
    elseif layout.anchor==3 then left=x-rowWidth-layout.gap*scale;top=y
    elseif layout.anchor==4 then left=x+w+layout.gap*scale;top=y end
    left=left+layout.x*scale;top=top-layout.y*scale
    local result={};local size=round(layout.size*scale)
    for _,slot in ipairs(row.slots)do
        local column=(slot.index-1)%layout.perRow
        if layout.reverse then column=columns-1-column end
        local sx=round(left+column*(layout.size+layout.spacing)*scale)
        local sy=round(top+math.floor((slot.index-1)/layout.perRow)*(layout.size+layout.spacing)*scale)
        if size>=8 and size<=256 and sx>=0 and sy>=0 and sx+size<=width and sy+size<=height then
            result[#result+1]=table.concat({sx,sy,size,layout.opacity},",")..":"..table.concat(slot.ids,",")
        end
    end
    return table.concat(result,";")
end
function B:Body(budget)
    local db=LUI:DB();local F=LUI.FrameAuras
    if not db.cdEnabled or not db.engineTracking or not db.engineFrameEstimates or not F then return nil end
    local own=guid("player");local at=call(GetServerTime,"number")
    local width,height=GetPhysicalScreenSize();width=plain(width,"number");height=plain(height,"number")
    local factor=call(PixelUtil.GetPixelToUIUnitFactor,"number")
    local parentScale=call(UIParent.GetEffectiveScale,"number",UIParent)
    local parentLeft=call(UIParent.GetLeft,"number",UIParent);local parentTop=call(UIParent.GetTop,"number",UIParent)
    if not own or not at or not width or not height or not factor or factor<=0 or not parentScale or parentScale<=0 or not parentLeft or not parentTop then return nil end
    local scale=parentScale/factor
    self.epoch=self.epoch or tostring(at)..":"..tostring(round(GetTime()*1000))
    self.sequence=self.sequence+1
    local head={self.epoch..":"..self.sequence,at,round(width),round(height),own,
        round(db.engineFrameSize*scale),round(db.engineFrameSpacing*scale),db.engineFramePerRow,
        db.engineFrameMax,db.engineFrameOpacity,db.engineFrameAnchor,round(db.engineFrameGap*scale),
        round(db.engineFrameX*scale),round(db.engineFrameY*scale),flag(db.estimateSelf),flag(db.estimateParty),
        flag(db.estimateReady),flag(db.engineFrameUnknown),0,flag(db.engineFramePersistent)}
    if db.engineFrameMatchSelf then
        head[21]=table.concat({round(db.selfCDSize*scale),round(db.selfCDSpacing*scale),db.selfCDPerRow,
            db.selfCDMax,db.selfCDOpacity,db.selfCDAnchor,round(db.selfCDGap*scale),
            round(db.selfCDX*scale),round(db.selfCDY*scale),db.selfCDLayoutVersion>=2 and db.selfCDGrowth or 0},",")
    else head[21]="" end
    head[22]=db.engineFrameGrowth
    local rows={};local seen={}
    if not LUI.overlayHidden and F:ContextEnabled(db) and not F.preview and not F.layoutPreview and not (LUI.LookupEvents and LUI.LookupEvents.moving) then
        local selected=F:SelectOwnCooldownFrame(F.discovered,db)
        -- Discovery caches candidates, but re-read identity, visibility and geometry
        -- every packet. Never reuse the unit that occupied this frame previously.
        for _,target in ipairs(F.discovered or {})do
            local frame=target.frame
            local unit=F:UnitForFrame(frame)
            if unit and F:AllowsUnit(unit,db) and call(frame.IsForbidden,"boolean",frame)==false and
                call(frame.IsVisible,"boolean",frame)==true and call(UnitExists,"boolean",unit)==true and call(UnitIsVisible,"boolean",unit)==true then
                local identity=guid(unit)
                local left=call(frame.GetLeft,"number",frame);local top=call(frame.GetTop,"number",frame)
                local w=call(frame.GetWidth,"number",frame);local h=call(frame.GetHeight,"number",frame)
                local effective=call(frame.GetEffectiveScale,"number",frame)
                if identity and (identity~=own or db.selfCDFrame==4 or frame==selected)
                    and left and top and w and h and effective and effective>0 then
                    local x=round(left*effective/factor-parentLeft*scale)
                    local y=round(parentTop*scale-top*effective/factor)
                    w=round(w*effective/factor);h=round(h*effective/factor)
                    if x>=0 and y>=0 and w>0 and h>0 and x+w<=width and y+h<=height then
                        local row=table.concat({identity,x,y,w,h},"|")
                        local native=""
                        if identity==own and LUI.SelfCooldowns then native=LUI.SelfCooldowns:NativeSpellsForFrame(frame,db)end
                        local slots=identity==own and ownSlots(frame,db,x,y,w,h,scale,width,height) or nil
                        local entry=seen[row]
                        if not entry then entry={own=identity==own,native=""};rows[#rows+1]=entry;seen[row]=entry end
                        entry.native=mergeNative(entry.native,native)
                        -- Coincident frames can include a hidden duplicate. Keep
                        -- the slot layout supplied by the visible native row.
                        if slots~=nil and (entry.slots==nil or slots~="") then entry.slots=slots end
                        entry.row=row..(entry.slots~=nil and "|"..entry.native.."|"..entry.slots or (entry.native~="" and "|"..entry.native or ""))
                    end
                end
            end
        end
    end
    table.sort(rows,function(a,b)if a.own~=b.own then return a.own end;return a.row<b.row end)
    local lines={};for i,row in ipairs(rows)do lines[i]=row.row end
    local function body()return table.concat(head,"|")..(#lines>0 and "\n"..table.concat(lines,"\n") or "")end
    while #lines>0 and (#body()>budget or #lines>80) do table.remove(lines);head[19]=head[19]+1 end
    local result=body()
    if #result>budget then return nil end
    self.diagnostics.omitted=head[19];self.diagnostics.frames=#lines
    return result
end
