"""Optional development checks: python -m pip install lupa; python tools/test_addon.py."""
from pathlib import Path
import unittest
from lupa.lua51 import LuaRuntime

ROOT = Path(__file__).resolve().parents[1]

class AddonTests(unittest.TestCase):
    def test_all_lua_compiles(self):
        lua = LuaRuntime()
        compile_lua = lua.eval('function(s) local f,e=loadstring(s); assert(f,e) end')
        for path in (ROOT/'addon/lamdaUI').rglob('*.lua'):
            with self.subTest(path=path.name): compile_lua(path.read_text())

    def test_engine_settings_and_combat_reload_guard(self):
        lua = LuaRuntime()
        lua.execute('''
        combat=false; reloads=0
        function InCombatLockdown() return combat end
        function ReloadUI() reloads=reloads+1 end
        modules={}; states={}; buttons={}
        Addon={}
        function Addon:RegisterCustomModule(m) modules[m.id]=m end
        function Addon:GetCustomModuleSettings(id) states[id]=states[id] or {}; return states[id] end
        function Addon:IsCustomModuleEnabled(id) return self:GetCustomModuleSettings(id).enabled~=false end
        function Addon:CustomHubText(...) end
        function widget()
          local w={scripts={}}
          setmetatable(w,{__index=function(t,k) return function() end end})
          function w:SetScript(k,f) self.scripts[k]=f end
          function w:GetScript(k) return self.scripts[k] end
          function w:GetText() return "1" end
          function w:HookScript(k,f) self.scripts[k]=f end
          return w
        end
        function CreateFrame(...) return widget() end
        function Addon:CustomHubButton(p,text,x,y,width,cb)
          local w=widget(); w.click=cb; buttons[text]=w; return w
        end
        function Addon:CustomHubCheckbox(...) return widget() end
        ''')
        lua.execute('local f=assert(loadstring(...)); f("lamdaUI",Addon)', (ROOT/'addon/lamdaUI/Modules/Engine.lua').read_text())
        lua.execute('''
        modules.lamdaCD.initialize()
        assert(LamdaEngineDB.schema==1 and LamdaEngineDB.cdEnabled)
        states.lamdaCD.enabled=false; modules.lamdaCD.changed()
        assert(LamdaEngineDB.cdEnabled==false)
        modules.lamdaCD.initialize(); assert(states.lamdaCD.enabled==false)
        local parent=widget(); parent.engineInputs={}
        modules.lamdaCD.buildOptions(parent)
        combat=true; buttons["Save & Reload"].click(); assert(reloads==0)
        combat=false; buttons["Save & Reload"].click(); assert(reloads==1)
        assert(modules.engineSettings.noEnable)
        ''')

if __name__=='__main__': unittest.main()
