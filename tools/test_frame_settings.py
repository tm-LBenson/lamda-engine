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
        local db=LUI:DB();db.nativeOffsetX=21;db.nativeOffsetY=-17
        click("Top");assert(db.nativeAnchor==2 and db.nativeOffsetX==0 and db.nativeOffsetY==0)
        click("Appearance");assert(content.selectedTab=="appearance")
        local oldWidth=db.rowWidth
        click("Large");assert(db.nativeSize==34 and db.nativeSpacing==3 and db.nativeFontSize==15 and db.nativePerRow==5)
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
        db.nativeMaxCC=2;db.nativeMaxDebuffs=7;db.nativeBuffs=false;db.nativeOffsetX=-42
        assert(LUI:CreateProfile("Frame layout"));local exported=LUI:ExportProfile()
        assert(LUI:ImportProfile("Shared layout",exported))
        assert(db.nativeMaxCC==2 and db.nativeMaxDebuffs==7 and db.nativeBuffs==false and db.nativeOffsetX==-42)
        assert(not LUI:ImportProfile("Invalid icons","LUI1\\nnativeSize=n:500"))
        assert(db.nativeSize==26 and db.nativeMaxDebuffs==7)
        assert(LUI:ImportProfile("Old layout","LUI1\\nrowWidth=n:420\\ncdEnabled=b:0"))
        assert(db.rowWidth==420 and db.cdEnabled==false and db.nativeFrames==true and db.nativeSize==26)
        assert(db.nativeMaxCC==3 and db.nativeMaxDebuffs==4 and db.nativeBuffs==true)
        ''')

    def test_preview_is_unavailable_in_combat_and_reset_is_module_scoped(self):
        lua = runtime()
        lua.execute('''
        LUI.FrameAuras={preview=false}
        function LUI.FrameAuras:SetPreview(value)self.preview=value end
        login();LUI:SelectPage("modules")
        combat=true;LUI:RefreshUI();assert(findButton("Preview on frames").enabled==false)
        combat=false;LUI:RefreshUI();assert(findButton("Preview on frames").enabled==true)
        local db=LUI:DB();db.checkUpdates=false;db.nativeSize=42;db.nativeMaxCC=9
        click("Content");click("Reset LamdaCD settings");assert(db.nativeSize==42)
        click("Confirm reset");assert(db.nativeSize==26 and db.nativeMaxCC==3 and db.checkUpdates==false)
        assert(reloads==0)
        ''')


if __name__ == '__main__':
    unittest.main()
