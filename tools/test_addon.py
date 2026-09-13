"""Native addon regression checks: requires lupa with Lua 5.1."""
from pathlib import Path
import unittest
from lupa.lua51 import LuaRuntime
ROOT = Path(__file__).resolve().parents[1]
ADDON = ROOT / 'addon/lamdaUI'

MOCK = r'''
combat=false;reloads=0;loggingCalls=0;inside=false
function InCombatLockdown()return combat end
function ReloadUI()reloads=reloads+1 end
function IsInInstance()return inside,"party" end
function LoggingCombat(enabled)assert(enabled);loggingCalls=loggingCalls+1 end
C_Timer={After=function()end};SlashCmdList={};UISpecialFrames={}
LamdaUIDB={personal="preserve"};LamdaUIMigrationDB={personal="preserve"}
LUI={};frames={}
function widget(parent)
  local w={scripts={},parent=parent,shown=true}
  setmetatable(w,{__index=function(t,k)if k~="Low" and k~="High" and k:match("^%u")then return function()end end end})
  function w:SetScript(k,f)self.scripts[k]=f end
  function w:SetText(v)self.text=v;if self.scripts.OnTextChanged then self.scripts.OnTextChanged(self)end end
  function w:SetValue(v)if self.scripts.OnValueChanged then self.scripts.OnValueChanged(self,v)end end
  function w:GetText()return type(self.text)=="string" and self.text or "" end
  function w:CreateFontString()local f=widget(self);table.insert(frames,f);return f end
  function w:CreateTexture()return widget(self)end
  function w:IsShown()return self.shown end
  function w:IsVisible()return self.shown and (not self.parent or self.parent:IsVisible())end
  function w:Show()local changed=not self.shown;self.shown=true;if changed and self.scripts.OnShow then self.scripts.OnShow(self)end end
  function w:Hide()local changed=self.shown;self.shown=false;if changed and self.scripts.OnHide then self.scripts.OnHide(self)end end
  function w:SetShown(v)if v then self:Show()else self:Hide()end end
  function w:SetEnabled(v)self.enabled=v end
  function w:SetChecked(v)self.checked=v end
  function w:GetChecked()return self.checked end
  function w:SetSize(x,y)self.width=x;self.height=y end
  function w:SetHeight(y)self.height=y end
  function w:GetWidth()return self.width or 0 end
  function w:GetHeight()return self.height or 0 end
  function w:GetLeft()return self.left or 0 end
  function w:GetTop()return self.top or 0 end
  function w:SetPoint(a,b,c,x,y)if type(x)=="number"then self.left=x;self.top=800+y end end
  return w
end
function CreateFrame(kind,name,parent,template)local w=widget(parent);w.kind=kind;w.name=name;table.insert(frames,w);return w end
UIParent=widget();UIParent.width=1280;UIParent.height=800;UIParent.top=800
function GetPhysicalScreenSize()return 2560,1600 end
STANDARD_TEXT_FONT="test-font";ChatFontNormal="test-font"
function login()frames[1].scripts.OnEvent(nil,"PLAYER_LOGIN")end
function findButton(text)
  for _,f in ipairs(frames)do if f.text==text and (f.scripts.OnClick or f.kind=="Button") then return f end end
  error("Missing button: "..text)
end
function click(text)local f=findButton(text);assert(f.enabled~=false);f.scripts.OnClick(f)end
function below(frame,ancestor)while frame do if frame==ancestor then return true end;frame=frame.parent end;return false end
'''

def runtime():
    lua=LuaRuntime();lua.execute(MOCK)
    for name in ['Core.lua','Modules/LamdaCD.lua','Modules/Settings.lua','UI.lua']:
        lua.execute('local f=assert(loadstring(...));f("lamdaUI",LUI)',(ADDON/name).read_text())
    return lua

class AddonTests(unittest.TestCase):
    def test_all_lua_compiles(self):
        lua=LuaRuntime();compile_lua=lua.eval('function(s)local f,e=loadstring(s);assert(f,e)end')
        for path in ADDON.rglob('*.lua'):
            with self.subTest(path=path.name):compile_lua(path.read_text())

    def test_hub_navigation_and_module_isolation(self):
        lua=runtime();lua.execute('''
        LUI:RegisterModule({id="test",name="Test module",enabledKey="testEnabled",defaults={testEnabled=false,testSize=10},
          limits={testSize={1,20,integer=true}},build=function(parent)LUI:Label(parent,"Independent content",0,0)end})
        login();SlashCmdList.LAMDAUI("")
        assert(LUI.selectedPage=="general" and LUI.generalPage:IsVisible() and not LUI.modulesPage:IsVisible())
        assert(not LUI.inlinePreview:IsVisible())
        for _,f in ipairs(frames)do
          if f.text=="Row width" or f.text=="LamdaCD" or f.text=="Cooldown preview"then assert(not below(f,LUI.generalPage))end
        end
        click("Modules");assert(LUI.modulesPage:IsVisible() and not LUI.generalPage:IsVisible())
        assert(LUI.selectedModule=="cd" and LUI.inlinePreview:IsVisible())
        local content=LUI.modulePanels[1].content
        click("Placement");assert(content.selectedTab=="placement" and not LUI.inlinePreview:IsVisible())
        click("Tracking");assert(content.selectedTab=="tracking")
        click("Test module");assert(LUI.selectedModule=="test" and LUI.modulePanels[2]:IsVisible() and not LUI.modulePanels[1]:IsVisible())
        assert(LUI:DB().testEnabled==false)
        assert(reloads==0)
        SlashCmdList.LAMDAUI("");assert(LUI.selectedPage=="general" and not LUI.modulesPage:IsVisible())
        assert(reloads==0)
        LUI.generalPage.profileName:SetText("Raid draft");LamdaEngineDB.checkUpdates=false;LUI:RefreshUI()
        assert(LUI.generalPage.profileName:GetText()=="Raid draft")
        LamdaEngineDB.rowWidth=403;LUI:RefreshUI();assert(LamdaEngineDB.rowWidth==403)
        combat=true;LUI:Save();assert(reloads==0)
        combat=false;LUI:Save();assert(reloads==1)
        assert(LamdaUIDB.personal=="preserve" and LamdaUIMigrationDB.personal=="preserve")
        ''')

    def test_validation_profiles_and_scoped_reset(self):
        lua=runtime();lua.execute('''
        LamdaEngineDB={rowWidth=403,columns=0,overlayScale=math.huge,maxRows=0,opacity=101,anchor=-4,checkDays=2}
        login();local db=LUI:DB()
        assert(db.rowWidth==403 and db.columns==1 and db.overlayScale==1 and db.maxRows==12 and db.opacity==95 and db.anchor==1 and db.checkDays==1)
        db.overlayScale=0/0;assert(LUI:DB().overlayScale==1)
        db.showNames=false;db.overlayX=-17;db.checkUpdates=false;LUI:SaveActiveProfile()
        assert(LamdaUIDB.engineHub.profiles.Default.rowWidth==403 and LamdaUIDB.engineHub.profiles.Default.showNames==false)
        assert(LUI:CreateProfile(" Raid "));assert(LUI:ProfileDB().active=="Raid")
        db.rowWidth=500;db.showNames=true;db.checkUpdates=true
        assert(LUI:SelectProfile("Default"));assert(db.rowWidth==403 and db.showNames==false and db.overlayX==-17)
        assert(db.checkUpdates==true)
        assert(LUI:SelectProfile("Raid"));assert(db.rowWidth==500 and db.showNames==true)
        assert(LUI:RenameProfile("Dungeons"));assert(not LUI:ProfileDB().profiles.Raid)
        assert(not LUI:CreateProfile("Dungeons"));assert(not LUI:RenameProfile(""))
        assert(LUI:ResetModule("cd"));assert(db.rowWidth==340 and db.checkUpdates==true)
        assert(LUI:DeleteProfile());assert(LUI:ProfileDB().active=="Default" and db.rowWidth==403)
        assert(not LUI:DeleteProfile())
        assert(LamdaUIDB.personal=="preserve" and LamdaUIMigrationDB.personal=="preserve")
        assert(reloads==0)
        LUI:SelectPage("modules");LUI.modulePanels[1].content.selectTab(3)
        click("Reset LamdaCD settings");assert(db.rowWidth==403)
        click("Confirm reset");assert(db.rowWidth==340)
        ''')

    def test_profile_import_is_data_only_and_transactional(self):
        lua=runtime();lua.execute(r'''
        login();local db=LUI:DB();db.showNames=false;db.overlayX=-51;db.overlayScale=1.35
        local exported=LUI:ExportProfile()
        assert(LUI:ImportProfile("Imported",exported));assert(LUI:ExportProfile()==exported)
        assert(db.showNames==false and db.overlayX==-51 and db.overlayScale==1.35)
        local active=LUI:ProfileDB().active
        local bad={"print('execute')", "LUI1\ncolumns=n:0", "LUI1\nrowWidth=n:1e999", "LUI1\ncdEnabled=b:0\ncdEnabled=b:1", "LUI1\nunknown=n:2", "LUI1\nrowWidth=s:313233", "LUI1\n"}
        for _,input in ipairs(bad)do
          assert(not LUI:ImportProfile("Rejected",input));assert(LUI:ProfileDB().active==active and not LUI:ProfileDB().profiles.Rejected)
          assert(LUI:ExportProfile()==exported)
        end
        assert(LUI:ImportProfile("Old partial","LUI1\nrowWidth=n:420\ncdEnabled=b:0"))
        assert(db.rowWidth==420 and db.cdEnabled==false and db.showNames==true)
        LUI:ShowProfileDialog(false);assert(LUI.profileDialog:IsShown())
        LUI:SelectPage("modules");assert(not LUI.profileDialog:IsShown())
        assert(reloads==0)
        ''')

    def test_profiles_survive_addon_reload_in_existing_saved_variables(self):
        lua=runtime();lua.execute('''
        login();assert(LUI:CreateProfile("Dungeons"));LUI:DB().rowWidth=417;LUI:DB().showNames=false
        LUI:Save();assert(LamdaUIDB.engineHub.profiles.Dungeons.rowWidth==417)
        LUI={};frames={}
        ''')
        for name in ['Core.lua','Modules/LamdaCD.lua','Modules/Settings.lua','UI.lua']:
            lua.execute('local f=assert(loadstring(...));f("lamdaUI",LUI)',(ADDON/name).read_text())
        lua.execute('''
        login();assert(LUI:ProfileDB().active=="Dungeons")
        assert(LUI:DB().rowWidth==417 and LUI:DB().showNames==false)
        assert(LUI:SelectProfile("Default"));assert(LUI:DB().rowWidth==340)
        assert(LUI:SelectProfile("Dungeons"));assert(LUI:DB().rowWidth==417 and LUI:DB().showNames==false)
        assert(LamdaUIDB.personal=="preserve" and LamdaUIMigrationDB.personal=="preserve")
        ''')

    def test_move_cancel_reopen_and_combat(self):
        lua=runtime();lua.execute('''
        login();LUI:OpenUI();LUI:SelectPage("modules")
        for anchor=1,9 do
          local ax=((anchor-1)%3)/2;local ay=math.floor((anchor-1)/3)/2
          local x,y=LUI:GuideOffsets((2560-340)*ax+42,(1600-108)*ay-18,340,108,2560,1600,anchor)
          assert(x==42 and y==-18)
        end
        combat=true;LUI:MoveCooldowns();assert(LUI.cdGuide==nil)
        combat=false;local db=LUI:DB();local x,y,w,h=db.overlayX,db.overlayY,db.rowWidth,db.rowHeight
        LUI:MoveCooldowns();assert(LUI.cdGuide:IsVisible() and not LUI.frame:IsShown())
        LUI.cdGuide.left=300;LUI.cdGuide.top=500;LUI:RecordGuide();assert(db.overlayX==600 and db.overlayY==600)
        local grip=findButton("//")
        grip.scripts.OnMouseDown(nil,"LeftButton");LUI.cdGuide.width=200;LUI.cdGuide.height=75;grip.scripts.OnMouseUp()
        assert(db.rowWidth==400 and db.rowHeight==48)
        LUI:OpenUI();assert(LUI.generalPage:IsVisible() and not LUI.cdGuide:IsShown() and not LUI.moveControls:IsShown())
        assert(db.overlayX==x and db.overlayY==y and db.rowWidth==w and db.rowHeight==h)
        LUI:MoveCooldowns();db.rowWidth=600;combat=true;LUI.cdGuide.scripts.OnEvent()
        assert(db.rowWidth==w and not LUI.cdGuide:IsShown())
        assert(reloads==0)
        inside=true;db.cdEnabled=false;LUI:EnsureLogging();assert(loggingCalls==0)
        db.cdEnabled=true;LUI:EnsureLogging();assert(loggingCalls==1)
        db.autoLog=false;LUI:EnsureLogging();assert(loggingCalls==1)
        ''')

    def test_distribution_has_only_public_hub_modules(self):
        self.assertEqual({p.name for p in (ADDON/'Modules').iterdir()},{'LamdaCD.lua','Settings.lua'})
        toc=(ADDON/'lamdaUI.toc').read_text()
        self.assertIn('LamdaUIDB',toc)
        for term in ('Augmentation.lua','DevastationMacros.lua','Profile.lua','Layout.lua'):
            self.assertNotIn(term,toc)

if __name__=='__main__':unittest.main()
