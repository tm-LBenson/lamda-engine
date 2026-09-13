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
          setmetatable(w,{__index=function(t,k)if k~="Low" and k~="High" and k:match("^%u")then return function()end end end})
          function w:SetScript(k,f)self.scripts[k]=f end
          function w:SetText(v)self.text=v end
          function w:SetValue(v)if self.scripts.OnValueChanged then self.scripts.OnValueChanged(self,v)end end
          function w:GetText()return type(self.text)=="string" and self.text or "" end
          function w:CreateFontString()return widget()end
          function w:CreateTexture()return widget()end
          function w:IsShown()return self.shown==true end
          function w:SetShown(v)self.shown=v end
          function w:Hide()self.shown=false end
          function w:SetSize(x,y)self.width=x;self.height=y end
          function w:GetWidth()return self.width or 0 end
          function w:GetHeight()return self.height or 0 end
          function w:GetLeft()return self.left or 0 end
          function w:GetTop()return self.top or 0 end
          function w:SetPoint(a,b,c,x,y)if type(x)=="number" then self.left=x;self.top=800+y end end
          function w:Show()self.shown=true end
          return w
        end
        function CreateFrame(kind,name,parent,template)local w=widget();w.parent=parent;table.insert(frames,w);return w end
        UIParent=widget();UIParent.width=1280;UIParent.height=800;UIParent.top=800
        function GetPhysicalScreenSize()return 2560,1600 end
        STANDARD_TEXT_FONT="test-font"
        ''')
        for name in ['Core.lua','Modules/LamdaCD.lua','Modules/Settings.lua','UI.lua']:
            lua.execute('local f=assert(loadstring(...)); f("lamdaUI",LUI)',(ROOT/'addon/lamdaUI'/name).read_text())
        lua.execute('''
        assert(#LUI.modules==1 and LUI.modules[1].name=="LamdaCD")
        frames[1].scripts.OnEvent(nil,"PLAYER_LOGIN")
        SlashCmdList.LAMDAUI("");assert(LUI.frame.shown)
        assert(LUI.selectedPage=="general" and LUI.generalPage:IsShown() and not LUI.modulesPage:IsShown())
        LUI:SelectPage("modules")
        assert(not LUI.generalPage:IsShown() and LUI.modulesPage:IsShown())
        assert(LUI.selectedModule=="cd" and LUI.modulePanels[1]:IsShown())
        assert(LUI.modulePanels[1].parent==LUI.modulesPage)
        assert(LUI.inlinePreview.parent.parent==LUI.modulePanels[1])
        SlashCmdList.LAMDAUI("")
        assert(LUI.selectedPage=="general" and not LUI.modulesPage:IsShown())
        LamdaEngineDB.rowWidth=403
        for _,f in ipairs(frames)do if f.scripts.OnShow then f.scripts.OnShow(f)end end
        assert(LamdaEngineDB.rowWidth==403)
        LamdaEngineDB.rowWidth=340
        assert(LamdaEngineDB.schema==1 and LamdaEngineDB.cdEnabled and LamdaEngineDB.companions and not LamdaEngineDB.preview)
        LamdaEngineDB.cdEnabled=false;assert(LUI:DB().cdEnabled==false)
        combat=true;LUI:Save();assert(reloads==0)
        combat=false;LUI:Save();assert(reloads==1)
        assert(LamdaUIDB.personal=="preserve" and LamdaUIMigrationDB.personal=="preserve")
        
        for anchor=1,9 do
          local ax=((anchor-1)%3)/2;local ay=math.floor((anchor-1)/3)/2
          local x,y=LUI:GuideOffsets((2560-340)*ax+42,(1600-108)*ay-18,340,108,2560,1600,anchor)
          assert(x==42 and y==-18)
        end
        local calls=0;LUI.commitInputs=function()calls=calls+1 end
        combat=true;LUI:MoveCooldowns();assert(calls==0 and LUI.cdGuide==nil)
        combat=false
        local priorX,priorY=LamdaEngineDB.overlayX,LamdaEngineDB.overlayY
        local priorW,priorH=LamdaEngineDB.rowWidth,LamdaEngineDB.rowHeight
        LUI:MoveCooldowns();assert(LUI.cdGuide:IsShown() and not LUI.frame:IsShown())
        LUI.cdGuide.left=300;LUI.cdGuide.top=500;LUI:RecordGuide()
        assert(LamdaEngineDB.overlayX==600 and LamdaEngineDB.overlayY==600)
        local grip
        for _,f in ipairs(frames)do if f.text=="//"then grip=f end end
        grip.scripts.OnMouseDown(nil,"LeftButton")
        LUI.cdGuide.width=200;LUI.cdGuide.height=75
        grip.scripts.OnMouseUp()
        assert(LamdaEngineDB.rowWidth==400 and LamdaEngineDB.rowHeight==48)
        LUI:FinishMove(true)
        assert(not LUI.cdGuide:IsShown() and LUI.frame:IsShown())
        assert(LamdaEngineDB.overlayX==priorX and LamdaEngineDB.overlayY==priorY and LamdaEngineDB.rowWidth==priorW and LamdaEngineDB.rowHeight==priorH)
        for _,f in ipairs(frames)do assert(f.text~="Offset X" and f.text~="Offset Y")end
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
