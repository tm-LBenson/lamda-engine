-- Opponent observations use arena tokens and a round epoch, never party slots
-- or a guessed match between restricted GUIDs/names. Native text displays spell
-- IDs; only the companion reads pixels. No restricted value is read back by Lua.
local _,LUI=...
local G=LUI.Gladius
local T={sequence=0,round=0};G.tracking=T
local O=LUI.Observations
local num,str=O.Number,O.String
local function inside()local _,kind=IsInInstance();return kind=="arena" end
function T:Reset()
    self.round=self.round+1
    self.epoch=tostring(GetServerTime())..":"..math.floor(GetTime()*1000)..":"..self.round
    if self.screen then self.screen:Hide()end
end
function T:Hide()
    if self.screen then self.screen:Hide()end
end
function T:Body()
    local db=LUI:DB();local bridge=LUI.LookupEvents:GetStrip("gladius")
    local active=not LUI.ForeverEngine and G.active and G.native and inside() and db.gladiusEngine
        and not bridge.moving and not G.previewCount
    if not active then
        self:Hide()
        if not self.wasActive then return nil end
    end
    if not self.epoch then self:Reset()end
    local width,height=GetPhysicalScreenSize()
    local flags=active and ((db.gladiusInterrupts and 1 or 0)+(db.gladiusDefensives and 2 or 0)+(db.gladiusRacials and 4 or 0)
        +(db.gladiusAnnouncements and 8 or 0)+(db.gladiusLockouts and 16 or 0)) or 0
    local rows,identities,guids,actors={},{},{},{}
    local count=active and num(GetNumArenaOpponentSpecs()) or 0
    count=math.min(5,math.max(0,count or 0))
    local pixel=PixelUtil.GetPixelToUIUnitFactor()
    for i=1,count do
        local row=G.rows[i];local spec=num(GetArenaOpponentSpec(i)) or 0
        local scale=row:GetEffectiveScale()/pixel
        local left,top=num(row:GetLeft()),num(row:GetTop())
        if not left or not top then return nil end
        local size=math.floor(db.gladiusCooldownSize*scale+.5)
        local x,y
        if db.gladiusCooldownSide==1 then
            x=left*scale
            y=height-top*scale+(row:GetHeight()+(db.gladiusCast and db.gladiusCastHeight+3 or 0)+(db.gladiusPets and 16 or 0)+3)*scale
        else
            x=(left+db.gladiusWidth+(db.gladiusTrinket and db.gladiusTrinketSize+7 or 3)
                +(db.gladiusDR and db.gladiusDRSide==2 and 3*(db.gladiusDRSize+2)+4 or 0))*scale
            y=height-top*scale
        end
        local guid=str(UnitGUID(row.unit)) or "-"
        if not guid:match("^Player%-%w[%w%-]*$")then guid="-" end
        actors[#actors+1]=table.concat({i,spec,math.floor(x+.5),math.floor(y+.5),size,guid},",")
        rows[#rows+1]=(i-1)..","..i
        guids[i-1]="Arena:"..self.epoch..":"..i
        identities[#identities+1]=(i-1)..":"..guids[i-1]..":"..spec
    end
    if active and not self.screen then
        self.screen=O:NewCastScreen({"arena1","arena2","arena3","arena4","arena5"},
            {"arenapet1","arenapet2","arenapet3","arenapet4","arenapet5"},
            function(owner)return G.active and inside() and owner.guid=="Arena:"..T.epoch..":"..(owner.slot+1)end)
        self.screen.name="LamdaGladius"
    end
    local line
    if active and not self.screen.failed then
        local ok,err=pcall(self.screen.Create,self.screen,bridge.frame)
        if ok then line=self.screen:FormatLine(bridge,rows,identities,guids)
        else self.screen:Fail(err)end
    end
    self.sequence=self.sequence%2147483646+1
    local head=table.concat({1,self.epoch,self.sequence,GetServerTime(),width,height,flags,
        db.gladiusCooldownMax,db.gladiusOpacity,LUI:DB().keepUIWhenUnfocused and 1 or 0,
        LUI.overlayHidden and 1 or 0},"|")
    self.wasActive=active or self.wasActive
    self.inactive=not active
    return head.."\n"..table.concat(actors,";").."\n\n"..(line or "")
end
local f=CreateFrame("Frame")
for _,event in ipairs({"PVP_MATCH_ACTIVE","PVP_MATCH_COMPLETE"})do pcall(f.RegisterEvent,f,event)end
f:SetScript("OnEvent",function()T:Reset()end)
