"""Strict Lua 5.1 contracts for Blizzard-owned aura rendering and lifecycle.

Mocks model CustomAuraContainer's batch initialization and the native inbound
bindings. The addon cannot inspect the aura buttons once initialization ends.
Requires lupa (the same dependency as tools/test_addon.py).
"""
from pathlib import Path
import unittest
from lupa.lua51 import LuaRuntime

SOURCE = Path(__file__).resolve().parents[1] / "addon/lamdaUI/NativeAuras.lua"
MOCK = r'''
combat=false;frames={};containers={};buttons={};LUI={};failBinding=false;invalidFilter=false
STANDARD_TEXT_FONT="Fonts\\FRIZQT__.TTF"
function InCombatLockdown()return combat end
function GetBuildInfo()return "12.1.0","mock","today",120100 end
local secretObject={}
function issecretvalue(value)return value==secretObject end
function getSecret()return secretObject end
AuraContainerSortMethod={Default=0,BigDefensive=1,UnitFrameDebuff=2,ImportantOnly=3}
AuraContainerSortDirection={Normal=0,Reverse=1}
AnchorUtil={FlowDirection={Left=-1,Right=1,Up=1,Down=-1},FlowLayoutAxis={Horizontal=0,Vertical=1}}
Enum={CustomAuraButtonDispelTypeTextureStyle={Border=0}}
AuraUtil={IsValidFilterString=function(filter)return not invalidFilter end}
local Frame={}
function Frame:IsForbidden()return self.forbidden or false end
function Frame:SetScript(event,func)assert(self.kind~="AuraButton","Addon script on native button");self.scripts[event]=func end
function Frame:RegisterEvent(event)self.events[event]=true end
function Frame:SetSize(width,height)assert(not self.sealed);self.width=width;self.height=height end
function Frame:SetAllPoints(relative)assert(not self.sealed);self.allPoints=relative end
function Frame:SetPoint(...)assert(not self.sealed);self.point={...} end
function Frame:ClearAllPoints()assert(not self.sealed);self.point=nil end
function Frame:SetFrameStrata(strata)self.strata=strata end
function Frame:SetFrameLevel(level)self.level=level end
function Frame:SetTexCoord(...)self.texcoord={...} end
function Frame:SetFont(...)self.font={...} end
function Frame:SetTextColor(...)self.color={...} end
function Frame:SetColorTexture(...)self.color={...} end
function Frame:SetTexture(texture)self.texture=texture end
function Frame:SetDrawEdge(value)self.drawEdge=value end
function Frame:SetDrawSwipe(value)self.drawSwipe=value end
function Frame:SetReverse(value)self.reverse=value end
function Frame:SetHideCountdownNumbers(value)self.hideNumbers=value end
function Frame:SetSwipeColor(...)self.swipeColor={...} end
function Frame:SetMouseClickEnabled(value)assert(not self.sealed);self.mouseClick=value end
function Frame:SetMouseMotionEnabled(value)assert(not self.sealed);self.mouseMotion=value end
function Frame:EnableMouse(value)self.mouseClick=value;self.mouseMotion=value end
function Frame:SetTooltipAnchorPoint(...)self.tooltip={...} end
function Frame:SetHideTooltipInCombat(value)self.hideTooltipInCombat=value end
function Frame:Hide()assert(not self.sealed);self.shown=false end
function Frame:Show()assert(not self.sealed);self.shown=true end
function Frame:IsShown()error("Native renderer may not query aura visibility")end
function Frame:GetWidth()error("Native renderer may not query native dimensions")end
function Frame:GetHeight()error("Native renderer may not query native dimensions")end
local function new(kind,parent)
 local frame={kind=kind,parent=parent,scripts={},events={},shown=false,regions={}}
 setmetatable(frame,{__index=function(t,key)
  if key=="auraData" or key=="spellId" or key=="duration" or key=="unitToken" then error("Secret aura field read: "..key) end
  return Frame[key]
 end})
 table.insert(frames,frame);return frame
end
function Frame:CreateTexture()assert(not self.sealed);local region=new("Texture",self);table.insert(self.regions,region);return region end
function Frame:CreateFontString()assert(not self.sealed);local region=new("FontString",self);table.insert(self.regions,region);return region end
local function bind(self,kind,object)
 assert(not self.sealed);assert(object.parent==self,"Bindings need a fresh descendant")
 assert(object.kind==kind,"Wrong binding region type");assert(not failBinding,"Injected binding failure")
 self.bindings=self.bindings or {};self.bindings[kind]=object
end
function Frame:SetIcon(object)bind(self,"Texture",object);self.icon=object end
function Frame:SetDurationCooldown(object)bind(self,"Cooldown",object);self.cooldown=object end
function Frame:SetDurationText(object,options)bind(self,"FontString",object);assert(next(options)==nil);self.durationText=object end
function Frame:SetApplicationCount(object,options)bind(self,"FontString",object);assert(next(options)==nil);self.count=object end
function Frame:AddDispelTypeTexture(object,options)bind(self,"Texture",object);self.border=object;self.borderOptions=options end
function Frame:SetEnabled(value)
 if value then assert(self.unit,"Unit not set before enable");assert(self.groups,"Groups not registered before enable")end
 self.enabled=value
end
function Frame:SetUnit(unit)assert(not combat,"Retargeting in combat must defer");self.unit=unit end
function Frame:SetFlowLayoutAxis(value)assert(not combat);self.axis=value end
function Frame:SetFlowLayoutPadding(...)assert(not combat);self.padding={...} end
function Frame:SetFlowLayoutAnchorPoint(value)assert(not combat);self.layoutPoint=value end
function Frame:SetFlowLayoutGrowthDirection(...)assert(not combat);self.growth={...} end
function Frame:SetFlowLayoutMaximumLineSize(value)assert(not combat);self.lineSize=value end
function Frame:UpdateAllAuras()self.refreshes=(self.refreshes or 0)+1 end
function Frame:AddAuraGroup(key,filter,options)
 assert(not combat);assert(self.enabled==false);assert(self.unit)
 local group={key=key,filter=filter,options=options,buttons={}};table.insert(self.groups,group)
 -- Blizzard initializes batches independent of actual aura presence.
 for i=1,10 do
  local button=new("AuraButton",self);options.initializeFrame(button)
  button.sealed=true;table.insert(buttons,button);table.insert(group.buttons,button)
 end
end
function CreateFrame(kind,name,parent,template)
 assert(kind~="AuraButton","Blizzard, not the addon, must create aura buttons")
 if kind=="AuraContainer" then assert(not combat);assert(template=="CustomAuraContainerTemplate")end
 local frame=new(kind,parent)
 if kind=="AuraContainer" then frame.groups={};table.insert(containers,frame)end
 if kind=="Cooldown" then assert(parent.kind=="AuraButton");assert(not parent.sealed);assert(template==nil)end
 return frame
end
UIParent=new("Frame")
partyFrame=new("Frame",UIParent)
function regen()combat=false;LUI.NativeAuras:FlushPending()end
function create(options)
 local handle,reason=LUI.NativeAuras:Create(partyFrame,"party1",options)
 assert(handle,reason);return handle
end
'''


def runtime():
    lua = LuaRuntime()
    lua.execute(MOCK)
    lua.execute('local f=assert(loadstring(...));f("lamdaUI",LUI)', SOURCE.read_text())
    return lua


class NativeAuraTests(unittest.TestCase):
    def test_engine_receives_groups_and_bindings_without_aura_reads(self):
        lua = runtime()
        lua.execute(r'''
        h=create({size=30,spacing=3,perRow=4,fontSize=13,relativeFrame=partyFrame,
          anchorPoint="LEFT",relativePoint="RIGHT",offsetX=8,offsetY=-2})
        c=h.container
        assert(c.parent==UIParent and h:GetFrame().parent==UIParent)
        assert(c.enabled and c.shown and #c.groups==5 and #buttons==50)
        assert(c.lineSize==129 and c.axis==AnchorUtil.FlowLayoutAxis.Horizontal and c.layoutPoint=="TOPLEFT")
        assert(c.groups[1].filter=="HARMFUL|CROWD_CONTROL")
        assert(c.groups[2].filter=="HARMFUL|!CROWD_CONTROL")
        assert(c.groups[3].filter=="HELPFUL|BIG_DEFENSIVE")
        assert(c.groups[4].filter=="HELPFUL|EXTERNAL_DEFENSIVE|!BIG_DEFENSIVE")
        assert(c.groups[5].filter=="HELPFUL|IMPORTANT|!BIG_DEFENSIVE|!EXTERNAL_DEFENSIVE")
        assert(c.groups[2].options.sortMethod==AuraContainerSortMethod.UnitFrameDebuff)
        assert(c.groups[1].options.layout.elementSpacing==3 and c.groups[1].options.layout.groupSpacing==0)
        assert(h.frame.point[2]==partyFrame and h.frame.point[4]==8 and h.frame.point[5]==-2)
        assert(c.point[1]=="LEFT" and c.point[2]==h.frame)
        for _,button in ipairs(buttons)do
          assert(button.icon and button.cooldown and button.durationText and button.count)
          assert(button.mouseClick==false and button.mouseMotion==true and button.hideTooltipInCombat)
          assert(button.cooldown.hideNumbers and not button.cooldown.drawEdge)
          assert(next(button.scripts)==nil)
        end
        assert(buttons[1].border and buttons[11].border and not buttons[21].border)
        h:Refresh();assert(c.refreshes==1)
        ''')

    def test_static_highlight_applies_to_every_aura_category(self):
        lua = runtime()
        lua.execute('''
        h=create({glow=true,borders=false,timers=false,stacks=false,swipe=false})
        local colors={}
        for _,group in ipairs(h.container.groups)do
          for _,button in ipairs(group.buttons)do
            assert(#button.regions==5 and next(button.scripts)==nil)
            -- Icon plus four fixed edges; no callbacks read aura state.
            for i=2,5 do
              local edge=button.regions[i];assert(edge.color and edge.color[4]==0.8)
              assert(edge.width==2 or edge.height==2)
            end
            colors[group.key]=table.concat(button.regions[2].color,",")
          end
        end
        assert(colors.cc~=colors.debuffs and colors.debuffs~=colors.defensives)
        assert(colors.defensives~=colors.buffs and colors.cc~=colors.buffs)
        assert(colors.external==colors.defensives)
        assert(h:Configure({glow=false,borders=false,timers=false,stacks=false,swipe=false}))
        for _,group in ipairs(h.container.groups)do for _,button in ipairs(group.buttons)do
          assert(#button.regions==1)
        end end
        ''')

    def test_placement_changes_reuse_native_buttons(self):
        lua = runtime()
        lua.execute('''
        h=create({});local count=#containers;local old=h.container
        assert(h:Configure({anchorPoint="BOTTOMRIGHT",relativePoint="BOTTOMLEFT",offsetX=-8,offsetY=4}))
        assert(#containers==count and h.container==old)
        assert(old.layoutPoint=="BOTTOMRIGHT" and old.growth[1]==AnchorUtil.FlowDirection.Left
          and old.growth[2]==AnchorUtil.FlowDirection.Up)
        assert(h:Configure({anchorPoint="BOTTOMRIGHT",relativePoint="BOTTOMLEFT",offsetX=-8,offsetY=4}))
        assert(#containers==count)
        assert(h:Configure({size=40,swipe=false,stacks=false,timers=false,glow=false,borders=false}))
        assert(#containers==count+1 and not old.enabled and not old.shown)
        for _,group in ipairs(h.container.groups)do for _,button in ipairs(group.buttons)do
          assert(button.icon and not button.cooldown and not button.count and not button.durationText and not button.border)
        end end
        ''')

    def test_eight_growth_directions_are_independent_of_attachment(self):
        lua = runtime()
        lua.execute('''
        local expected={
          {0,1,-1,"TOPLEFT"}, {0,-1,-1,"TOPRIGHT"},
          {0,1,1,"BOTTOMLEFT"}, {0,-1,1,"BOTTOMRIGHT"},
          {1,1,-1,"TOPLEFT"}, {1,1,1,"BOTTOMLEFT"},
          {1,-1,-1,"TOPRIGHT"}, {1,-1,1,"BOTTOMRIGHT"},
        }
        for growth,case in ipairs(expected)do
          local h=create({growth=growth,anchorPoint="BOTTOMRIGHT",relativePoint="TOPLEFT",size=30,spacing=3,perRow=2})
          local c=h.container
          assert(c.axis==case[1] and c.growth[1]==case[2] and c.growth[2]==case[3] and c.layoutPoint==case[4])
          assert(c.point[1]=="BOTTOMRIGHT" and h.frame.point[3]=="TOPLEFT")
          assert(c.lineSize==63 and h.options.growth==growth)
          -- Moving the strip's outer anchor does not change explicit flow.
          local count=#containers
          assert(h:Configure({growth=growth,anchorPoint="TOPLEFT",relativePoint="BOTTOMRIGHT",size=30,spacing=3,perRow=2}))
          assert(#containers==count and h.container==c)
          assert(c.axis==case[1] and c.growth[1]==case[2] and c.growth[2]==case[3] and c.layoutPoint==case[4])
        end
        ''')

    def test_growth_zero_preserves_anchor_flow_and_growth_change_rebuilds(self):
        lua = runtime()
        lua.execute('''
        h=create({anchorPoint="BOTTOMRIGHT"});local c=h.container;local count=#containers
        assert(h.options.growth==0 and c.axis==0 and c.layoutPoint=="BOTTOMRIGHT")
        assert(c.growth[1]==-1 and c.growth[2]==1)
        assert(h:Configure({growth=0,anchorPoint="LEFT"}))
        assert(#containers==count and h.container==c and c.axis==0 and c.layoutPoint=="TOPLEFT")
        assert(c.growth[1]==1 and c.growth[2]==-1)
        assert(h:Configure({growth=7,anchorPoint="LEFT"}))
        assert(#containers==count+1 and not c.enabled and not c.shown)
        assert(h.container.axis==1 and h.container.growth[1]==-1 and h.container.growth[2]==-1)
        assert(h.container.layoutPoint=="TOPRIGHT" and h.container.point[1]=="LEFT")
        combat=true;local changed=h.container;assert(not h:Configure({growth=6,anchorPoint="LEFT"}))
        assert(not changed.enabled and not changed.shown);regen()
        assert(h.options.growth==6 and h.container.axis==1 and h.container.layoutPoint=="BOTTOMLEFT")
        ''')

    def test_combat_changes_suspend_immediately_and_coalesce(self):
        lua = runtime()
        lua.execute('''
        h=create({});local old=h.container;local count=#containers
        combat=true
        assert(not h:SetUnit("party2"));assert(not old.enabled and not old.shown)
        assert(h.unit=="party1" and h.pendingUnit=="party2")
        assert(not h:Configure({size=40}));assert(not h:Configure({size=36}))
        assert(not h:SetShown(true));assert(not old.enabled)
        assert(#containers==count)
        regen();assert(h.unit=="party2" and h.options.size==36)
        assert(h.container.enabled and h.container.shown and #containers==count+1)
        assert(h:GetError()==nil)
        combat=true;h:Configure({size=44});h:Configure({size=36})
        regen();assert(h.options.size==36 and #containers==count+1)
        combat=true;assert(h:SetShown(false));assert(not h.container.enabled and not h.container.shown)
        h:Destroy();regen();assert(not h.container.enabled and not LUI.NativeAuras.handles[h])
        ''')

    def test_capability_probe_and_creation_never_run_in_combat(self):
        lua = runtime()
        lua.execute('''
        combat=true;local count=#frames
        local supported,reason=LUI.NativeAuras:IsSupported();assert(not supported and reason=="combat")
        local handle,why=LUI.NativeAuras:Create(partyFrame,"party1",{});assert(not handle and why=="combat")
        assert(#frames==count and LUI.NativeAuras.supported==nil)
        regen();assert(LUI.NativeAuras:IsSupported());assert(#containers==1)
        h=create({});assert(#containers==1)
        ''')

    def test_invalid_bindings_fail_closed_and_settings_are_bounded(self):
        lua = runtime()
        lua.execute('''
        h=create({size=math.huge,spacing=-99,fontSize=999,offsetX=0/0,offsetY=getSecret()})
        assert(h.options.size==80 and h.options.spacing==0 and h.options.fontSize==32)
        assert(h.options.offsetX==6 and h.options.offsetY==0)
        assert(not h:SetUnit(getSecret()));assert(not h.container.enabled and not h.container.shown)
        assert(h:SetUnit("party1"));assert(h.container.enabled)
        failBinding=true
        local broken,reason=LUI.NativeAuras:Create(partyFrame,"party1",{})
        assert(not broken and reason)
        for _,container in ipairs(containers)do if container~=h.container then assert(not container.enabled and not container.shown)end end
        failBinding=false;partyFrame.forbidden=true
        assert(not LUI.NativeAuras:Create(partyFrame,"party1",{}))
        ''')

    def test_real_frame_runtime_binds_discovered_frames_through_native_api(self):
        lua = runtime()
        lua.execute('''
        db={};function LUI:RegisterModule(module)for k,v in pairs(module.defaults)do db[k]=v end end
        function LUI:DB()return db end
        function IsInInstance()return true,"party" end
        function UnitExists(unit)return unit=="party1" or unit=="party2" end
        function UnitIsVisible(unit)return UnitExists(unit)end
        partyFrame.unit="party1"
        function partyFrame:GetAttribute()return self.unit end
        function partyFrame:IsVisible()return self.available~=false end
        function DandersFrames_GetAllFrames()return {partyFrame}end
        C_UnitAuras=setmetatable({},{__index=function()error("Addon must never read aura data")end})
        ''')
        addon = SOURCE.parent
        for name in ("Modules/LamdaCD.lua", "FrameAuras.lua"):
            lua.execute('local f=assert(loadstring(...));f("lamdaUI",LUI)', (addon / name).read_text())
        lua.execute('''
        local runtime=LUI.FrameAuras;runtime:Update()
        local binding=runtime.bindings[partyFrame];assert(binding and binding.regions)
        local cc=binding.regions.CC.handle;local debuffs=binding.regions.Debuffs.handle
        assert(cc.container.enabled and debuffs.container.enabled and #containers==4)
        assert(#cc.container.groups==1 and cc.container.groups[1].filter=="HARMFUL|CROWD_CONTROL")
        assert(#debuffs.container.groups==1 and debuffs.container.groups[1].filter=="HARMFUL|!CROWD_CONTROL")
        assert(cc.options.relativeFrame==partyFrame and debuffs.options.relativeFrame==partyFrame)
        assert(cc.options.size==db.nativeCCPartySize and debuffs.options.size==db.nativeDebuffsPartySize)
        assert(cc.container.point[1]=="LEFT" and cc.container.layoutPoint=="TOPLEFT")
        combat=true;partyFrame.unit="party2";runtime:Update()
        for _,entry in pairs(binding.regions)do assert(not entry.handle.container.enabled and not entry.handle.container.shown)end
        combat=false;runtime:Update();assert(binding.unit=="party2" and cc.container.enabled and debuffs.container.enabled)
        local debuffContainer=debuffs.container
        db.nativeCCPartySize=34;db.nativeCCPartyGrowth=7;runtime:RequestRefresh();runtime:Update()
        assert(cc.options.size==34 and cc.options.growth==7 and cc.container.axis==1)
        assert(cc.container.layoutPoint=="TOPRIGHT" and cc.container.point[1]=="LEFT")
        assert(debuffs.container==debuffContainer and debuffs.options.size==db.nativeDebuffsPartySize)
        partyFrame.available=false;runtime:Update()
        for _,entry in pairs(binding.regions)do assert(not entry.handle.container.enabled)end
        ''')

    def test_safe_diagnostics_preserve_errors_and_bound_output(self):
        lua = runtime()
        lua.execute('''
        h=create({});h.container.UpdateAllAuras=function()error(string.rep("x",600))end
        assert(not h:Refresh());assert(#h:GetError()==400 and h:GetError():find("xxxxx"))
        assert(not h.container.enabled and not h.container.shown)
        h.error=nil;h.container.UpdateAllAuras=function()error(getSecret())end
        assert(not h:Refresh());assert(h:GetError()=="Native aura display unavailable.")
        ''')

    def test_reloading_file_reuses_runtime_and_native_border_is_optional(self):
        lua = runtime()
        lua.execute('oldRuntime=LUI.NativeAuras;oldFrames=#frames;Enum.CustomAuraButtonDispelTypeTextureStyle=nil')
        lua.execute('local f=assert(loadstring(...));f("lamdaUI",LUI)', SOURCE.read_text())
        lua.execute("assert(LUI.NativeAuras==oldRuntime and #frames==oldFrames);h=create({});assert(h.container.enabled)")

    def test_category_toggles_and_counts_are_native_policy(self):
        lua = runtime()
        lua.execute('''
        h=create({groups={cc={enabled=false},debuffs={max=7},defensives={enabled=false},buffs={max=0}}})
        assert(#h.container.groups==1 and h.container.groups[1].key=="debuffs")
        assert(h.container.groups[1].options.maxFrameCount==7)
        assert(h.container.groups[1].filter=="HARMFUL|!CROWD_CONTROL")
        invalidFilter=true
        assert(not h:Configure({size=38}));assert(not h.container.enabled and not h.container.shown)
        ''')


if __name__ == "__main__":
    unittest.main()
