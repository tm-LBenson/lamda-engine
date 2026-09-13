"""Behavior checks for the frame aura settings and profile migration."""
import unittest
from test_addon import runtime


class FrameSettingsTests(unittest.TestCase):
    def test_module_controls_frame_icons_and_keeps_general_separate(self):
        lua = runtime()
        lua.execute('''
        LUI.FrameAuras={preview=false,previewCalls=0}
        function LUI.FrameAuras:SetPreview(value)self.preview=value;self.previewCalls=self.previewCalls+1 end
        login();LUI:OpenUI()
        assert(LUI.selectedPage=="general")
        for _,f in ipairs(frames) do
          if f.text=="Crowd control" or f.text=="Attach icons to each frame" or f.text=="Icon size" then
            assert(not below(f,LUI.generalPage))
          end
          assert(f.text~="Row width" and f.text~="Move & resize cooldowns" and f.text~="Pin cooldowns to screen")
        end
        click("Modules");local content=LUI.modulePanels[1].content
        assert(content.selectedTab=="auras")
        click("Preview on frames");assert(LUI.FrameAuras.preview and LUI.FrameAuras.previewCalls==1)
        click("Stop preview");assert(not LUI.FrameAuras.preview and LUI.FrameAuras.previewCalls==2)
        click("Frames");assert(content.selectedTab=="frames")
        local db=LUI:DB();db.nativeCCPartyOffsetX=21;db.nativeCCPartyOffsetY=-17
        click("Top");assert(db.nativeCCPartyAnchor==2 and db.nativeCCPartyOffsetX==0 and db.nativeCCPartyOffsetY==0)
        assert(db.nativeCCRaidAnchor==5 and db.nativeDefensivesPartyAnchor==5)
        click("Appearance");assert(content.selectedTab=="appearance")
        local oldWidth=db.rowWidth
        click("Large");assert(db.nativeCCPartySize==34 and db.nativeCCPartySpacing==3 and db.nativeCCPartyFontSize==15 and db.nativeCCPartyPerRow==5)
        assert(db.nativeCCRaidSize==20 and db.nativeDefensivesPartySize==30)
        assert(db.rowWidth==oldWidth)
        click("Content");assert(content.selectedTab=="content")
        assert(reloads==0)
        ''')

    def test_existing_profiles_gain_native_settings_and_new_values_roundtrip(self):
        lua = runtime()
        lua.execute('''
        LamdaEngineDB={rowWidth=403,nativeSize=4,nativeAnchor=12,nativeGlow="yes",nativePerRow=0}
        login();local db=LUI:DB()
        assert(db.rowWidth==403 and db.nativeFrames==true)
        assert(db.nativeSize==26 and db.nativeAnchor==6 and db.nativeGlow==false and db.nativePerRow==6)
        db.nativeMaxCC=2;db.nativeMaxDebuffs=7;db.nativeBuffs=false;db.nativeCCPartyOffsetX=-42
        db.nativeDefensivesRaidSize=42;db.nativeDebuffsPartyGrowth=8;db.nativeBuffsRaidTooltips=false
        assert(LUI:CreateProfile("Frame layout"));local exported=LUI:ExportProfile()
        assert(LUI:ImportProfile("Shared layout",exported))
        assert(db.nativeMaxCC==2 and db.nativeMaxDebuffs==7 and db.nativeBuffs==false and db.nativeCCPartyOffsetX==-42)
        assert(db.nativeDefensivesRaidSize==42 and db.nativeDebuffsPartyGrowth==8 and not db.nativeBuffsRaidTooltips)
        assert(not LUI:ImportProfile("Invalid region","LUI1\\nnativeCCPartyGrowth=n:9"))
        assert(db.nativeCCPartyOffsetX==-42)
        assert(not LUI:ImportProfile("Invalid icons","LUI1\\nnativeSize=n:500"))
        assert(db.nativeSize==26 and db.nativeMaxDebuffs==7)
        assert(LUI:ImportProfile("Old layout","LUI1\\nrowWidth=n:420\\ncdEnabled=b:0"))
        assert(db.rowWidth==420 and db.cdEnabled==false and db.nativeFrames==true and db.nativeSize==26)
        assert(db.nativeMaxCC==3 and db.nativeMaxDebuffs==4 and db.nativeBuffs==true)
        assert(db.nativeCCPartySize==32 and db.nativeCCRaidSize==20 and db.nativeDefensivesRaidSize==25)
        ''')

    def test_preview_is_unavailable_in_combat_and_reset_is_module_scoped(self):
        lua = runtime()
        lua.execute('''
        LUI.FrameAuras={preview=false}
        function LUI.FrameAuras:SetPreview(value)self.preview=value end
        login();LUI:SelectPage("modules")
        combat=true;LUI:RefreshUI();assert(findButton("Preview on frames").enabled==false)
        combat=false;LUI:RefreshUI();assert(findButton("Preview on frames").enabled==true)
        local db=LUI:DB();db.checkUpdates=false;db.nativeCCPartySize=42;db.nativeMaxCC=9
        click("Content");click("Reset LamdaCD settings");assert(db.nativeCCPartySize==42)
        click("Confirm reset");assert(db.nativeCCPartySize==32 and db.nativeMaxCC==3 and db.checkUpdates==false)
        assert(reloads==0)
        ''')

    def test_editing_one_region_and_context_does_not_move_or_resize_others(self):
        lua = runtime()
        lua.execute('''
        login();LUI:OpenUI();click("Modules");click("Frames")
        local function visibleClick(text)
          for _,f in ipairs(frames)do
            if f.text==text and f.scripts.OnClick and f:IsVisible() then
              assert(f.enabled~=false);f.scripts.OnClick(f);return f
            end
          end
          for _,f in ipairs(frames)do
            if f.text==text and f:IsVisible() and f.parent and f.parent.kind=="CheckButton" then
              f.parent.scripts.OnClick(f.parent);return f.parent
            end
          end
          error("Missing visible button: "..text)
        end
        visibleClick("Crowd control  ▾");visibleClick("Debuffs")
        visibleClick("Party & player frames  ▾");visibleClick("Raid frames")
        assert(LUI.frameLayoutEdit.region=="Debuffs" and LUI.frameLayoutEdit.context=="Raid")
        local db=LUI:DB();db.nativeDebuffsRaidOffsetX=17
        visibleClick("Left")
        assert(db.nativeDebuffsRaidAnchor==4 and db.nativeDebuffsRaidOffsetX==0)
        assert(db.nativeDebuffsPartyAnchor==8 and db.nativeCCRaidAnchor==5)
        visibleClick("Appearance");visibleClick("Large")
        assert(db.nativeDebuffsRaidSize==34 and db.nativeDebuffsRaidSpacing==3)
        assert(db.nativeDebuffsPartySize==24 and db.nativeCCRaidSize==20)
        visibleClick("Right / down")
        assert(db.nativeDebuffsRaidGrowth==2 and db.nativeDebuffsPartyGrowth==1)
        local tooltip=visibleClick("Tooltips")
        tooltip:SetChecked(false);tooltip.scripts.OnClick(tooltip)
        assert(not db.nativeDebuffsRaidTooltips and db.nativeDebuffsPartyTooltips)
        visibleClick("Debuffs  ▾");visibleClick("Defensive buffs")
        visibleClick("Compact")
        assert(db.nativeDefensivesRaidSize==20 and db.nativeDebuffsRaidSize==34)
        assert(db.nativeDefensivesPartySize==30)
        assert(LUI:CreateProfile("Independent frames"))
        local exported=LUI:ExportProfile()
        assert(not exported:find("frameLayoutEdit") and not exported:find("selectedRegion"))
        assert(LUI:ImportProfile("Shared frames",exported))
        assert(db.nativeDebuffsRaidSize==34 and db.nativeDefensivesRaidSize==20 and not db.nativeDebuffsRaidTooltips)
        ''')


if __name__ == '__main__':
    unittest.main()
