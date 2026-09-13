"""Native frame discovery and independent-region ownership checks without aura reads."""
import unittest
from test_addon import runtime, ADDON


def frames_runtime():
    lua=runtime()
    lua.execute('''
    kind="party";inRaid=false;visible={player=true,party1=true,party2=true,raid1=true,partypet1=true,pet=true}
    function IsInInstance()return kind~="none",kind end
    function IsInRaid()return inRaid end
    function UnitExists(unit)return visible[unit]~=nil end
    function UnitIsVisible(unit)return visible[unit]==true end
    SECRET={};function issecretvalue(v)return v==SECRET end
    C_Spell={GetSpellTexture=function(id)return id end}
    C_UnitAuras=setmetatable({},{__index=function()error("Aura data must stay native")end})
    local baseWidget=widget
    function widget(parent)
      local f=baseWidget(parent);local original=f.SetPoint
      function f:SetPoint(...)self.point={...};return original(self,...)end
      return f
    end
    function partyFrame(unit)
      local f=widget(UIParent);f.unit=unit;f.attribute=unit
      function f:GetAttribute()return self.attribute end
      function f:IsForbidden()return false end
      return f
    end
    df1=partyFrame("party2");df2=partyFrame("party1")
    all={df1,df2};function DandersFrames_GetAllFrames()return all end
    CompactPartyFrameMember1=partyFrame("party1")
    callbacks={};DandersFrames={RegisterCallback=function(token,event,fn)callbacks[event]=fn end}
    created={}
    LUI.NativeAuras={IsSupported=function()return true end,Create=function(_,parent,unit,options)
      assert(not combat);local h={unit=unit,options=options,shown=true,configured=0}
      function h:Configure(o)assert(not combat);self.options=o;self.configured=self.configured+1;return true end
      function h:SetUnit(u)assert(not combat);self.unit=u;return true end
      function h:SetShown(v)self.shown=v end
      function h:Destroy()self.shown=false;self.destroyed=true end
      created[#created+1]=h;return h
    end}
    function allShown(binding,expected)
      for _,region in pairs(binding.regions)do assert(region.handle.shown==expected)end
    end
    ''')
    lua.execute('local f=assert(loadstring(...));f("lamdaUI",LUI)',(ADDON/'FrameAuras.lua').read_text())
    lua.execute('login()')
    return lua


class FrameRuntimeTests(unittest.TestCase):
    def test_sorted_units_have_four_independent_regions_and_provider_selection(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;f:Update()
        assert(#created==8 and f.bindings[df1].unit=="party2" and f.bindings[df2].unit=="party1")
        for _,binding in pairs(f.bindings)do
          for id,region in pairs(binding.regions)do
            assert(region.handle.unit==binding.unit)
            local count=0;for _,group in pairs(region.handle.options.groups)do if group.enabled then count=count+1 end end
            assert(count==1)
          end
        end
        assert(f.bindings[df1].regions.CC.handle.options.relativeFrame==df1)
        assert(f.bindings[df2].regions.Debuffs.handle.options.relativeFrame==df2)
        assert(f.bindings[df1].regions.CC.handle~=f.bindings[df1].regions.Debuffs.handle)
        assert(not f.bindings[CompactPartyFrameMember1]);assert(callbacks.OnFramesSorted)
        LUI:DB().nativeProvider=3;f:Update()
        allShown(f.bindings[CompactPartyFrameMember1],true);allShown(f.bindings[df1],false)
        ''')

    def test_danders_pet_collection_uses_actual_unit_binding(self):
        lua=frames_runtime();lua.execute('''
        local pet=partyFrame("partypet1");DandersFrames.partyPetFrames={[4]=pet}
        local f=LUI.FrameAuras;f:Update();assert(not f.bindings[pet])
        LUI:DB().nativePets=true;f:RequestRefresh();f:Update()
        assert(f.bindings[pet] and f.bindings[pet].unit=="partypet1")
        pet.attribute="partypet2";f:Update();allShown(f.bindings[pet],false)
        ''')

    def test_secrets_conflicts_visibility_and_context_fail_closed(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;f:Update();local first=f.bindings[df1]
        df1.attribute=SECRET;f:Update();allShown(first,false)
        df1.attribute="party1";f:Update();allShown(first,false)
        df1.attribute="party2";visible.party2=false;f:Update();allShown(first,false)
        visible.party2=true;f:Update();allShown(first,true)
        LUI:DB().nativeDungeon=false;f:Update()
        for _,h in ipairs(created)do assert(not h.shown)end
        kind="scenario";f:Update();allShown(first,true)
        LUI:DB().nativeDelve=false;f:Update();allShown(first,false)
        kind="party";LUI:DB().nativeDungeon=true;LUI:DB().cdEnabled=false;f:Update();allShown(first,false)
        ''')

    def test_all_regions_hide_on_combat_reassignment_then_rebind(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;f:Update();local first=f.bindings[df1];local second=f.bindings[df2]
        combat=true;df1.unit="party1";df1.attribute="party1";df2.unit="party2";df2.attribute="party2"
        callbacks.OnFramesSorted();allShown(first,false);allShown(second,false);assert(#created==8)
        combat=false;f:Update();assert(first.unit=="party1" and second.unit=="party2")
        for _,region in pairs(first.regions)do assert(region.unit=="party1" and region.handle.unit=="party1")end
        allShown(first,true);allShown(second,true)
        combat=true;all[#all+1]=partyFrame("raid1");f:Update();assert(#created==8)
        combat=false;f:Update();assert(#created==12)
        ''')

    def test_party_raid_and_each_region_keep_independent_options(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;local db=LUI:DB();local raid=partyFrame("raid1");all[#all+1]=raid
        db.nativeCCPartySize=41;db.nativeCCRaidSize=22;db.nativeDebuffsPartySize=19
        db.nativeCCPartyAnchor=5;db.nativeCCPartyGap=0;db.nativeCCPartyOffsetX=13;db.nativeCCPartyOffsetY=-7
        db.nativeCCPartyGrowth=5;db.nativeDebuffsPartyGrowth=2;db.nativeCCPartyTooltips=false
        f:Update()
        local cc=f.bindings[df1].regions.CC.handle.options;local debuffs=f.bindings[df1].regions.Debuffs.handle.options
        assert(cc.size==41 and debuffs.size==19 and f.bindings[raid].regions.CC.handle.options.size==22)
        assert(cc.anchorPoint=="CENTER" and cc.relativePoint=="CENTER" and cc.offsetX==13 and cc.offsetY==-7)
        assert(cc.growth==5 and debuffs.growth==2 and cc.tooltips==false)
        db.nativeCCPartySize=43;f:RequestRefresh();f:Update()
        assert(f.bindings[df1].regions.CC.handle.options.size==43)
        assert(f.bindings[raid].regions.CC.handle.options.size==22 and debuffs.size==19)
        assert(f:LayoutContext("raidpet4")=="Raid" and f:LayoutContext("party1")=="Party")
        inRaid=true;assert(f:LayoutContext("player")=="Raid" and f:LayoutContext("pet")=="Raid")
        inRaid=SECRET;assert(f:LayoutContext("player")=="Party")
        ''')

    def test_player_layout_context_change_waits_for_combat_to_end(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;local db=LUI:DB();all={partyFrame("player")}
        db.nativeCCPartySize=40;db.nativeCCRaidSize=21;f:Update()
        local binding=f.bindings[all[1]];local cc=binding.regions.CC
        assert(cc.context=="Party" and cc.handle.options.size==40)
        combat=true;inRaid=true;f:Update();allShown(binding,false)
        assert(cc.context=="Party" and cc.handle.options.size==40)
        combat=false;f:Update();assert(cc.context=="Raid" and cc.handle.options.size==21);allShown(binding,true)
        ''')

    def test_failed_context_reconfiguration_waits_for_revision_or_new_context(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;all={partyFrame("player")};f:Update()
        local region=f.bindings[all[1]].regions.CC;local attempts=0
        function region.handle:Configure()attempts=attempts+1;return false end
        function region.handle:GetError()return "configuration failed" end
        inRaid=true;f:Update();assert(attempts==1 and region.context=="Party" and region.attemptedContext=="Raid")
        f:Update();f:Update();assert(attempts==1 and not region.handle.shown)
        f:RequestRefresh();f:Update();assert(attempts==2)
        f:Update();assert(attempts==2)
        inRaid=false;f:Update();assert(attempts==3 and region.attemptedContext=="Party")
        ''')

    def test_disabled_category_has_no_handle_until_enabled_and_hides_independently(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;local db=LUI:DB();db.nativeCC=false;f:Update()
        assert(#created==6 and not f.bindings[df1].regions.CC)
        assert(f.bindings[df1].regions.Debuffs.handle.shown)
        combat=true;db.nativeCC=true;f:RequestRefresh();f:Update();assert(#created==6)
        combat=false;f:Update();assert(#created==8 and f.bindings[df1].regions.CC.handle.shown)
        db.nativeCC=false;f:RequestRefresh();f:Update();assert(not f.bindings[df1].regions.CC.handle.shown)
        assert(f.bindings[df1].regions.Debuffs.handle.shown and f.bindings[df1].regions.Defensives.handle.shown)
        ''')

    def test_preview_regions_follow_actual_frames_and_selected_context_demo(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;local exported=LUI:ExportProfile()
        f:SetPreview(true);assert(f.preview and #f.samples==8)
        for _,h in ipairs(created)do assert(not h.shown)end
        local labels={};for _,sample in ipairs(f.samples)do
          assert(sample:IsShown() and sample.context=="Party" and (sample.target==df1 or sample.target==df2))
          local count=0;for _,icon in ipairs(sample.icons)do if icon:IsShown()then count=count+1 end end
          assert(count==1)
          labels[sample.title.text]=true
        end
        assert(labels["LamdaCD preview: CC"] and labels["LamdaCD preview: Debuffs"] and labels["LamdaCD preview: Defensives"] and labels["LamdaCD preview: Important buffs"])
        f:SetPreview(false);assert(not f.preview)
        for _,sample in ipairs(f.samples)do assert(not sample:IsShown())end
        for _,h in ipairs(created)do assert(h.shown)end
        assert(LUI:ExportProfile()==exported)
        LUI.frameLayoutEdit={region="CC",context="Raid"};f:SetPreview(true)
        assert(f.demo:IsShown() and f.demo.context=="Raid" and f.demo.title.text=="Sample raid frame")
        for i=1,4 do assert(f.samples[i]:IsShown() and f.samples[i].context=="Raid" and f.samples[i].target==f.demo)end
        for i=5,8 do assert(not f.samples[i]:IsShown())end
        assert(LUI:ExportProfile()==exported)
        LUI:OpenUI();assert(not f.preview and not f.demo:IsShown())
        ''')

    def test_layout_preview_focuses_selected_region_without_changing_live_settings(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;local exported=LUI:ExportProfile()
        LUI:OpenUI();click("Modules");click("Frames")
        assert(LUI.frameLayoutPreviewFocus)
        f:SetPreview(true)
        local function check(region,expected)
          local shown=0
          for _,sample in ipairs(f.samples)do if sample:IsShown()then
            shown=shown+1;assert(sample.region==region)
            local count=0;for _,icon in ipairs(sample.icons)do if icon:IsShown()then count=count+1 end end
            assert(count==expected)
          end end
          assert(shown==2)
        end
        check("CC",3)
        LUI.frameLayoutEdit.region="Debuffs";LUI:RefreshUI();f:Update();check("Debuffs",4)
        click("Appearance");f:Update();check("Debuffs",4)
        click("Auras");assert(not LUI.frameLayoutPreviewFocus);f:Update()
        local shown=0;for _,sample in ipairs(f.samples)do if sample:IsShown()then
          shown=shown+1
          local count=0;for _,icon in ipairs(sample.icons)do if icon:IsShown()then count=count+1 end end
          assert(count==1)
        end end
        assert(shown==8 and LUI:ExportProfile()==exported)
        f:SetPreview(false)
        for _,handle in ipairs(created)do assert(handle.shown)end
        ''')

    def test_preview_grid_matches_all_eight_growth_directions(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;local db=LUI:DB()
        db.nativeMaxCC=3;db.nativeCCPartySize=20;db.nativeCCPartySpacing=2;db.nativeCCPartyPerRow=2
        local expected={
          {{0,0},{22,0},{0,-22}},{{22,0},{0,0},{22,-22}},
          {{0,-22},{22,-22},{0,0}},{{22,-22},{0,-22},{22,0}},
          {{0,0},{0,-22},{22,0}},{{0,-22},{0,0},{22,-22}},
          {{22,0},{22,-22},{0,0}},{{22,-22},{22,0},{0,-22}},
        }
        for growth=1,8 do
          db.nativeCCPartyGrowth=growth;f:DrawSample(df1,1,"CC","Party")
          local sample=f.samples[1];assert(sample:GetWidth()==42 and sample:GetHeight()==42)
          for i=1,3 do
            local point=sample.icons[i].point
            assert(point[2]==expected[growth][i][1] and point[3]==expected[growth][i][2])
          end
        end
        db.nativeCCPartyPerRow=3;db.nativeCCPartyGrowth=5;f:DrawSample(df1,1,"CC","Party")
        assert(f.samples[1]:GetWidth()==20 and f.samples[1]:GetHeight()==64)
        ''')

    def test_handoff_requires_all_enabled_regions_to_succeed(self):
        lua=frames_runtime();lua.execute('''
        lamdaCDDB={visible=true};hidden=0;disabled={}
        SlashCmdList.LAMDACD=function(command)assert(command=="hide");hidden=hidden+1;lamdaCDDB.visible=false end
        C_AddOns={DisableAddOn=function(name)disabled[#disabled+1]=name end}
        local create=LUI.NativeAuras.Create
        LUI.NativeAuras.Create=function(self,parent,unit,options)
          if options.groups.debuffs.enabled then return nil,"debuff setup failed" end
          return create(self,parent,unit,options)
        end
        local f=LUI.FrameAuras;f:Update();assert(hidden==0 and #disabled==0)
        assert(f.bindings[df1].regions.CC.handle.shown and not f.bindings[df1].regions.Debuffs)
        LUI.NativeAuras.Create=create;f:RequestRefresh();f:Update();f:Update()
        assert(hidden==1 and #disabled==1 and disabled[1]=="lamdaCD")
        assert(LUI:ProfileDB().standaloneCDVisible and not lamdaCDDB.visible)
        ''')

    def test_failed_creation_retries_each_region_only_on_revision_or_new_binding(self):
        lua=frames_runtime();lua.execute('''
        attempts=0;LUI.NativeAuras.Create=function()attempts=attempts+1;return nil,"setup failed"end
        local f=LUI.FrameAuras;f:Update();assert(attempts==8)
        f:Update();f:Update();assert(attempts==8 and f.lastError=="setup failed")
        f:RequestRefresh();f:Update();assert(attempts==16)
        df1.unit="player";df1.attribute="player";f:Update();assert(attempts==20)
        ''')

    def test_unit_filters_and_nine_region_attachment_positions(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;local db=LUI:DB()
        assert(f:AllowsUnit("party1") and f:AllowsUnit("raid40") and not f:AllowsUnit("raid41"))
        assert(not f:AllowsUnit("target") and not f:AllowsUnit("partypet1"))
        db.nativePets=true;assert(f:AllowsUnit("partypet1"))
        db.nativeParty=false;assert(not f:AllowsUnit("party1") and not f:AllowsUnit("partypet1"))
        db.nativeCCPartyGap=6;db.nativeCCPartyOffsetX=0;db.nativeCCPartyOffsetY=0
        for index=1,9 do
          db.nativeCCPartyAnchor=index;local options=f:Options(df1,"CC","Party")
          assert(options.relativeFrame==df1 and options.anchorPoint and options.relativePoint)
          if index==6 then assert(options.anchorPoint=="LEFT" and options.relativePoint=="RIGHT" and options.offsetX==6)end
        end
        ''')

if __name__=='__main__':unittest.main()
