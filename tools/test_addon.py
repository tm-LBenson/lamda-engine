"""Optional development checks: requires lupa with Lua 5.1."""
from pathlib import Path
import unittest
from lupa.lua51 import LuaRuntime
ROOT = Path(__file__).resolve().parents[1]

class AddonTests(unittest.TestCase):
    def test_all_lua_compiles(self):
        lua=LuaRuntime()
        compile_lua=lua.eval('function(s) local f,e=loadstring(s); assert(f,e) end')
        for path in (ROOT/'addon/lamdaUI').rglob('*.lua'):
            with self.subTest(path=path.name):compile_lua(path.read_text())

    def test_hub_settings_reload_guard_and_legacy_preservation(self):
        lua=LuaRuntime()
        lua.execute('''
        combat=false;reloads=0;loggingCalls=0;inside=false
        function InCombatLockdown() return combat end
        function ReloadUI() reloads=reloads+1 end
        function IsInInstance() return inside,"party" end
        function LoggingCombat(enabled) assert(enabled);loggingCalls=loggingCalls+1 end
        C_Timer={After=function()end};UIParent={};SlashCmdList={};UISpecialFrames={}
        LamdaUIDB={personal="preserve"};LamdaUIMigrationDB={personal="preserve"}
        LUI={};frames={}
        function widget()
          local w={scripts={}}
          setmetatable(w,{__index=function(t,k)return function()end end})
          function w:SetScript(k,f)self.scripts[k]=f end
          function w:SetText(v)self.text=v end
          function w:GetText()return type(self.text)=="string" and self.text or "" end
          function w:CreateFontString()return widget()end
          function w:Show()self.shown=true end
          return w
        end
        function CreateFrame(...)local w=widget();table.insert(frames,w);return w end
        ''')
        for name in ['Core.lua','Modules/LamdaCD.lua','Modules/Settings.lua','UI.lua']:
            lua.execute('local f=assert(loadstring(...)); f("lamdaUI",LUI)',(ROOT/'addon/lamdaUI'/name).read_text())
        lua.execute('''
        assert(#LUI.modules==2 and LUI.modules[1].name=="LamdaCD" and LUI.modules[2].name=="Settings")
        frames[1].scripts.OnEvent(nil,"PLAYER_LOGIN")
        SlashCmdList.LAMDAUI("");assert(LUI.frame.shown)
        assert(LamdaEngineDB.schema==1 and LamdaEngineDB.cdEnabled)
        LamdaEngineDB.cdEnabled=false;assert(LUI:DB().cdEnabled==false)
        combat=true;LUI:Save();assert(reloads==0)
        combat=false;LUI:Save();assert(reloads==1)
        assert(LamdaUIDB.personal=="preserve" and LamdaUIMigrationDB.personal=="preserve")
        inside=true;LUI:EnsureLogging();assert(loggingCalls==0)
        LamdaEngineDB.cdEnabled=true;LUI:EnsureLogging();assert(loggingCalls==1)
        LamdaEngineDB.autoLog=false;LUI:EnsureLogging();assert(loggingCalls==1)
        ''')

    def test_distribution_has_only_public_hub_modules(self):
        root=ROOT/'addon/lamdaUI'
        self.assertEqual({p.name for p in (root/'Modules').iterdir()},{'LamdaCD.lua','Settings.lua'})
        toc=(root/'lamdaUI.toc').read_text()
        for term in ('Augmentation.lua','DevastationMacros.lua','Profile.lua','Layout.lua'):
            self.assertNotIn(term,toc)

if __name__=='__main__':unittest.main()
