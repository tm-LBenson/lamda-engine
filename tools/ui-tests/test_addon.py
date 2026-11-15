"""Native addon regression checks: requires lupa with Lua 5.1."""
from pathlib import Path
import unittest
from lupa.lua51 import LuaRuntime
ROOT = Path(__file__).resolve().parents[1]
ADDON = ROOT.parent / '.build/lamdaui/full-package/lamdaUI'

MOCK = r'''
combat=false;reloads=0;loggingCalls=0;inside=false
function InCombatLockdown()return combat end
function ReloadUI()reloads=reloads+1 end
function IsInInstance()return inside,"party" end
function LoggingCombat(enabled)assert(enabled);loggingCalls=loggingCalls+1 end
function GetTime()return 0 end
C_Timer={After=function()end};SlashCmdList={};UISpecialFrames={}
LamdaUIDB={personal="preserve"};LamdaUIMigrationDB={personal="preserve"}
LUI={};frames={}
function widget(parent)
  local w={scripts={},parent=parent,shown=true}
  setmetatable(w,{__index=function(t,k)if k~="Low" and k~="High" and k:match("^%u")then return function()end end end})
  function w:SetScript(k,f)self.scripts[k]=f end
  function w:GetScript(k)return self.scripts[k]end
  function w:HookScript(k,f)local prior=self.scripts[k];self.scripts[k]=function(...)if prior then prior(...)end;f(...)end end
  function w:SetAlpha(value)self.alpha=value end
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

def runtime(modules_enabled=True):
    lua=LuaRuntime();lua.execute(MOCK)
    # Runtime behavior tests opt in explicitly. Fresh-install tests pass False.
    if modules_enabled:
        lua.execute('LamdaEngineDB={cdEnabled=true}')
    for name in ['Core.lua','SpellLists.lua','Modules/LamdaCD.lua','Modules/Settings.lua','Performance.lua','Modules/Performance.lua','UI.lua']:
        lua.execute('local f=assert(loadstring(...));f("lamdaUI",LUI)',(ADDON/name).read_text())
    return lua

class AddonTests(unittest.TestCase):
    def test_general_background_visibility_is_global_live_and_defaults_off(self):
        lua=runtime();lua.execute('''
        login();LUI:OpenUI();assert(LUI:DB().keepUIWhenUnfocused==false)
        local toggle
        for _,f in ipairs(frames)do
            if f.kind=='CheckButton' and f.helpTitle=='Keep UI visible when tabbed out' then toggle=f end
        end
        assert(toggle and below(toggle,LUI.generalPage))
        toggle:SetChecked(true);toggle.scripts.OnClick(toggle);LUI:Apply()
        assert(LUI:DB().keepUIWhenUnfocused and not LUI.reloadRequired and reloads==0)
        assert(LUI:CreateProfile('Second'));assert(LUI:DB().keepUIWhenUnfocused)
        assert(not LUI:ExportProfile():find('keepUIWhenUnfocused',1,true))
        LUI:ResetGeneral();assert(not LUI:DB().keepUIWhenUnfocused)
        LUI:DB().keepUIWhenUnfocused=true;LUI:ResetAllSettings();assert(not LUI:DB().keepUIWhenUnfocused)
        LUI:DB().keepUIWhenUnfocused='true';assert(LUI:DB().keepUIWhenUnfocused==false)
        ''')

    def test_settings_build_is_hidden_and_refreshes_only_when_complete(self):
        lua=runtime();lua.execute('''
        local complete=false;local refreshes=0
        LUI:OnRefresh(UIParent,function()
            assert(complete,'Refreshed settings while a module was still building')
            refreshes=refreshes+1
        end)
        LUI:RegisterModule({id='late',name='Late module',build=function(parent)
            assert(not LUI.frame:IsVisible(),'Settings appeared during login construction')
            assert(not parent:IsVisible(),'An unfinished module was visible')
            complete=true
        end})
        login()
        assert(refreshes>0 and not LUI.buildingUI and not LUI.frame:IsShown())
        LUI:OpenUI()
        assert(LUI.generalPage:IsVisible() and not LUI.modulesPage:IsVisible())
        click('Modules');click('Late module')
        for i,panel in ipairs(LUI.modulePanels)do
            assert(panel:IsVisible()==(i==3),'Multiple modules were visible')
        end
        LUI.frame:Hide();LUI:OpenUI()
        assert(LUI.generalPage:IsVisible() and not LUI.modulesPage:IsVisible())
        ''')

    def test_failed_module_build_cannot_expose_partial_window(self):
        lua=runtime();lua.execute('''
        LUI:RegisterModule({id='broken',name='Broken module',build=function(parent)
            LUI:Label(parent,'Unfinished settings',0,0)
            error('simulated module initialization failure')
        end})
        local ok,message=pcall(login)
        assert(not ok and message:find('simulated module initialization failure',1,true))
        assert(LUI.frame and not LUI.frame:IsShown())
        for _,frame in ipairs(frames)do
            if below(frame,LUI.frame)then assert(not frame:IsVisible())end
        end
        ''')

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
        assert(LUI:DB().nativeFrames and LUI:DB().nativeSize==26)
        for _,f in ipairs(frames)do
          if f.text=="Icon size" or f.text=="Crowd control"then assert(not below(f,LUI.generalPage))end
          assert(f.text~="Row width" and f.text~="Move & resize cooldowns")
        end
        click("Modules");assert(LUI.modulesPage:IsVisible() and not LUI.generalPage:IsVisible())
        assert(LUI.selectedModule=="cd" and LUI.modulePanels[1]:IsVisible())
        local content=LUI.modulePanels[1].content
        assert(content.selectedTab=="auras")
        click("Effects");click("Position");assert(content.selectedTab=="auras" and content.tabPages[1].selectedSection=="Position")
        click("Style");assert(content.selectedTab=="auras" and content.tabPages[1].selectedSection=="Style")
        click("Visibility");assert(content.selectedTab=="content")
        click("Test module");assert(LUI.selectedModule=="test" and LUI.modulePanels[3]:IsVisible() and not LUI.modulePanels[1]:IsVisible())
        assert(LUI:DB().testEnabled==false)
        assert(reloads==0)
        SlashCmdList.LAMDAUI("");assert(LUI.selectedPage=="general" and not LUI.modulesPage:IsVisible())
        assert(reloads==0)
        LUI.generalPage.profileName:SetText("Raid draft");LamdaEngineDB.checkUpdates=false;LUI:RefreshUI()
        assert(LUI.generalPage.profileName:GetText()=="Raid draft")
        LamdaEngineDB.rowWidth=403;LUI:RefreshUI();assert(LamdaEngineDB.rowWidth==403)
        combat=true;LUI:Save();assert(reloads==1)
        LUI:DB().allowCombatManagement=false;LUI:Save();assert(reloads==1)
        combat=false;LUI:Save();assert(reloads==2)
        assert(LamdaUIDB.personal=="preserve" and LamdaUIMigrationDB.personal=="preserve")
        ''')

    def test_unlock_and_lock_commands_coordinate_movable_modules(self):
        lua=runtime();lua.execute('''
        login();local calls={refresh=0}
        LUI.RefreshUI=function()calls.refresh=calls.refresh+1 end
        LUI.LookupEvents={SetMoving=function(self,value)self.moving=value end}
        LUI.Kick={SetMoving=function(self,value)self.moving=value end}
        LUI.Performance={SetMoving=function(self,value)self.moving=value end}
        LUI.Gladius={RefreshSettings=function(self,force)self.force=force end}
        LUI.KBB={SetDisplaysLocked=function(self,value)self.locked=value end}
        LUI:DB().gladiusEnabled=true
        local mover={Unlock=function(self)self.unlocked=true end,Lock=function(self)self.unlocked=false end}
        function LibStub(name,silent)assert(name=='LamdaMover-1.0' and silent);return mover end
        SlashCmdList.LAMDAUI('unlock')
        assert(LUI.LookupEvents.moving and LUI.Kick.moving and LUI.Performance.moving)
        assert(LUI:DB().gladiusLocked==false and LUI.Gladius.force==true)
        assert(LUI.KBB.locked==false and mover.unlocked==true and calls.refresh==1)
        SlashCmdList.LAMDAUI('lock')
        assert(not LUI.LookupEvents.moving and not LUI.Kick.moving and not LUI.Performance.moving)
        assert(LUI:DB().gladiusLocked==true and LUI.KBB.locked==true)
        assert(mover.unlocked==false and calls.refresh==2)
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
        click("Restore");assert(db.rowWidth==340)
        ''')

    def test_tracking_is_independent_of_native_display_and_profiles(self):
        lua=runtime();lua.execute('''
        login();local db=LUI:DB()
        assert(db.nativeFrames and db.engineTracking and loggingCalls==0)
        inside=true;LUI:EnsureLogging();assert(loggingCalls==1)
        db.engineTracking=false;LUI:EnsureLogging();assert(loggingCalls==1 and db.nativeFrames)
        LUI:SaveActiveProfile();assert(LUI:CreateProfile("Tracking on"))
        db.engineTracking=true;db.autoLog=false;LUI:EnsureLogging();assert(loggingCalls==1)
        db.autoLog=true;db.cdEnabled=false;LUI:EnsureLogging();assert(loggingCalls==1)
        db.cdEnabled=true
        function IsInInstance()return true,"raid"end
        LUI:EnsureLogging();assert(loggingCalls==2)
        function IsInInstance()return true,"arena"end
        LUI:EnsureLogging();assert(loggingCalls==3)
        assert(LUI:SelectProfile("Default") and db.engineTracking==false)
        assert(LUI:ImportProfile("Tracking imported",LUI:ExportProfile()) and db.engineTracking==false)
        click("Modules");click("Engine")
        assert(LUI.modulePanels[1].content.selectedTab=="tracking")
        local checkbox
        for _,f in ipairs(frames)do
          if f.text=="Estimate cooldowns with Lamda Engine" then checkbox=f.parent end
        end
        assert(checkbox and checkbox:GetChecked()==false)
        checkbox:SetChecked(true);checkbox.scripts.OnClick(checkbox)
        assert(db.engineTracking==true and db.nativeFrames==true)
        for _,f in ipairs(frames)do
          if f.text=="Engine" then assert(f:GetLeft()+f:GetWidth()<=620) end
        end
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
        for name in ['Core.lua','SpellLists.lua','Modules/LamdaCD.lua','Modules/Settings.lua','Performance.lua','Modules/Performance.lua','UI.lua']:
            lua.execute('local f=assert(loadstring(...));f("lamdaUI",LUI)',(ADDON/name).read_text())
        lua.execute('''
        login();assert(LUI:ProfileDB().active=="Dungeons")
        assert(LUI:DB().rowWidth==417 and LUI:DB().showNames==false)
        assert(LUI:SelectProfile("Default"));assert(LUI:DB().rowWidth==340)
        assert(LUI:SelectProfile("Dungeons"));assert(LUI:DB().rowWidth==417 and LUI:DB().showNames==false)
        assert(LamdaUIDB.personal=="preserve" and LamdaUIMigrationDB.personal=="preserve")
        ''')

    def test_native_preview_lifecycle_and_live_controls(self):
        lua=runtime();lua.execute('''
        LUI.FrameAuras={preview=false,requests=0,hides=0,changes=0}
        function LUI.FrameAuras:RequestRefresh()self.requests=self.requests+1 end
        function LUI.FrameAuras:HideSamples()self.hides=self.hides+1 end
        function LUI.FrameAuras:SetPreview(enabled)
          self.preview=enabled;self.changes=self.changes+1;LUI:RefreshUI()
        end
        LUI:RegisterModule({id="test",name="Test module",build=function()end})
        login();LUI:OpenUI();LUI:SelectPage("modules")
        local auras=LUI.FrameAuras
        click("Preview");click("Preview on frames");assert(auras.preview and auras.changes==1)
        local requests=auras.requests
        LUI:RefreshUI();assert(auras.preview and auras.changes==1 and auras.requests>requests)
        requests=auras.requests
        for _,f in ipairs(frames)do
          if f.kind=="Slider" then f.scripts.OnValueChanged(f,5);break end
        end
        assert(LUI:DB().nativeMaxCC==5 and auras.requests>requests)
        click("General");assert(not auras.preview and auras.hides==1 and auras.changes==1)
        click("Modules");click("Preview");click("Preview on frames");assert(auras.preview)
        click("Test module");assert(not auras.preview and auras.hides==2)
        click("LamdaCD");click("Preview");click("Preview on frames");assert(auras.preview)
        LUI.frame:Hide();assert(not auras.preview and auras.hides==3)
        LUI:OpenUI();assert(LUI.selectedPage=="general")
        click("Modules");click("Preview");click("Preview on frames");assert(auras.preview)
        LUI:OpenUI();assert(not auras.preview and auras.hides==4 and LUI.selectedPage=="general")
        click("Modules");combat=true;LUI.frame.scripts.OnEvent()
        assert(findButton("Preview on frames").enabled==false)
        findButton("Preview on frames").scripts.OnClick();assert(not auras.preview)
        assert(reloads==0)
        ''')

    def test_distribution_has_only_public_hub_modules(self):
        self.assertEqual({p.name for p in (ADDON/'Modules').iterdir()},{'LamdaCD.lua','LamdaKick.lua','Settings.lua','Performance.lua','Gladius.lua','Frames.lua','Frames','Vendor.lua','KBB.lua','KBBUI.lua', 'KBBSync.lua','Drawer.lua','Sim.lua','SimUI.lua','Overlays.lua','Augmentation.lua','Queue.lua','MarketCodec.lua','Market.lua','QuestTracker.lua'})
        toc=(ADDON/'lamdaUI.toc').read_text()
        self.assertIn('LamdaUIDB',toc)
        for term in ('DevastationMacros.lua','Profile.lua','Layout.lua'):
            self.assertNotIn('Modules\\'+term,toc)

if __name__=='__main__':unittest.main()
