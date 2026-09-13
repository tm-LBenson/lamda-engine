"""Native frame discovery and ownership checks without reading live aura data."""
import unittest
from test_addon import runtime, ADDON


def frames_runtime():
    lua=runtime()
    lua.execute('''
    kind="party";visible={player=true,party1=true,party2=true,raid1=true,partypet1=true}
    function IsInInstance()return kind~="none",kind end
    function UnitExists(unit)return visible[unit]~=nil end
    function UnitIsVisible(unit)return visible[unit]==true end
    SECRET={};function issecretvalue(v)return v==SECRET end
    C_Spell={GetSpellTexture=function(id)return id end}
    C_UnitAuras=setmetatable({},{__index=function()error("Aura data must stay native")end})
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
    ''')
    lua.execute('local f=assert(loadstring(...));f("lamdaUI",LUI)',(ADDON/'FrameAuras.lua').read_text())
    lua.execute('login()')
    return lua


class FrameRuntimeTests(unittest.TestCase):
    def test_actual_sorted_unit_and_provider_selection(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;f:Update()
        assert(#created==2 and f.bindings[df1].unit=="party2" and f.bindings[df2].unit=="party1")
        assert(f.bindings[df1].handle.options.relativeFrame==df1)
        assert(f.bindings[df2].handle.options.relativeFrame==df2)
        assert(not f.bindings[CompactPartyFrameMember1])
        assert(callbacks.OnFramesSorted)
        LUI:DB().nativeProvider=3;f:Update()
        assert(f.bindings[CompactPartyFrameMember1].handle.shown)
        assert(not f.bindings[df1].handle.shown)
        ''')

    def test_danders_pet_frame_collection_uses_actual_unit_binding(self):
        lua=frames_runtime();lua.execute('''
        local pet=partyFrame("partypet1");DandersFrames.partyPetFrames={[4]=pet}
        local f=LUI.FrameAuras;f:Update();assert(not f.bindings[pet])
        LUI:DB().nativePets=true;f:RequestRefresh();f:Update()
        assert(f.bindings[pet] and f.bindings[pet].unit=="partypet1")
        pet.attribute="partypet2";f:Update();assert(not f.bindings[pet].handle.shown)
        ''')

    def test_secrets_conflicts_visibility_and_context_fail_closed(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;f:Update();local first=f.bindings[df1].handle
        df1.attribute=SECRET;f:Update();assert(not first.shown)
        df1.attribute="party1";f:Update();assert(not first.shown)
        df1.attribute="party2";visible.party2=false;f:Update();assert(not first.shown)
        visible.party2=true;f:Update();assert(first.shown)
        LUI:DB().nativeDungeon=false;f:Update()
        for _,h in ipairs(created)do assert(not h.shown)end
        kind="scenario";f:Update();assert(first.shown)
        LUI:DB().nativeDelve=false;f:Update();assert(not first.shown)
        kind="party";LUI:DB().nativeDungeon=true;LUI:DB().cdEnabled=false;f:Update();assert(not first.shown)
        ''')

    def test_combat_reassignment_hides_then_rebinds(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;f:Update();local first=f.bindings[df1].handle;local second=f.bindings[df2].handle
        combat=true;df1.unit="party1";df1.attribute="party1";df2.unit="party2";df2.attribute="party2"
        callbacks.OnFramesSorted();assert(not first.shown and not second.shown and #created==2)
        combat=false;f:Update();assert(first.unit=="party1" and second.unit=="party2" and first.shown and second.shown)
        combat=true;all[#all+1]=partyFrame("raid1");f:Update();assert(#created==2)
        combat=false;f:Update();assert(#created==3)
        ''')

    def test_preview_uses_real_frames_and_clears_without_touching_settings(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;local exported=LUI:ExportProfile()
        f:SetPreview(true);assert(f.preview and #f.samples==2)
        for _,h in ipairs(created)do assert(not h.shown)end
        f:SetPreview(false);assert(not f.preview and not f.samples[1]:IsShown())
        for _,h in ipairs(created)do assert(h.shown)end
        assert(LUI:ExportProfile()==exported)
        LUI:DB().nativeProvider=2;all={};f:SetPreview(true)
        assert(f.demo:IsShown() and f.samples[1]:IsShown())
        LUI:OpenUI();assert(not f.preview and not f.demo:IsShown())
        ''')

    def test_only_successful_native_display_retires_own_predecessor(self):
        lua=frames_runtime();lua.execute('''
        lamdaCDDB={visible=true};hidden=0;disabled={}
        SlashCmdList.LAMDACD=function(command)assert(command=="hide");hidden=hidden+1;lamdaCDDB.visible=false end
        C_AddOns={DisableAddOn=function(name)disabled[#disabled+1]=name end}
        local f=LUI.FrameAuras;f:Update();f:Update()
        assert(hidden==1 and #disabled==1 and disabled[1]=="lamdaCD")
        assert(LUI:ProfileDB().standaloneCDVisible and not lamdaCDDB.visible)
        ''')

    def test_failed_native_creation_is_not_retried_every_tick(self):
        lua=frames_runtime();lua.execute('''
        attempts=0;LUI.NativeAuras.Create=function()attempts=attempts+1;return nil,"setup failed"end
        local f=LUI.FrameAuras;f:Update();assert(attempts==2)
        f:Update();f:Update();assert(attempts==2 and f.lastError=="setup failed")
        f:RequestRefresh();f:Update();assert(attempts==4)
        ''')

    def test_unit_filters_and_nine_frame_attachment_positions(self):
        lua=frames_runtime();lua.execute('''
        local f=LUI.FrameAuras;local db=LUI:DB()
        assert(f:AllowsUnit("party1") and f:AllowsUnit("raid40") and not f:AllowsUnit("raid41"))
        assert(not f:AllowsUnit("target") and not f:AllowsUnit("partypet1"))
        db.nativePets=true;assert(f:AllowsUnit("partypet1"))
        db.nativeParty=false;assert(not f:AllowsUnit("party1") and not f:AllowsUnit("partypet1"))
        for index=1,9 do
          db.nativeAnchor=index;local options=f:Options(df1)
          assert(options.relativeFrame==df1 and options.anchorPoint and options.relativePoint)
          if index==6 then assert(options.anchorPoint=="LEFT" and options.relativePoint=="RIGHT" and options.offsetX==6)end
        end
        ''')

if __name__=='__main__':unittest.main()
