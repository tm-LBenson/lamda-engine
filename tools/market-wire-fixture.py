"""Emit actual LamdaMarket optical packets for the Go end-to-end test."""
import base64
import json
from pathlib import Path
import sys
sys.path.insert(0, str(Path(__file__).parent / 'ui-tests'))
from test_addon import MOCK
from lupa.lua51 import LuaRuntime
root = Path(__file__).resolve().parents[1]
addon = root/'Lamda/UIAddon' if (root/'Lamda/UIAddon').is_dir() else root/'lamdaui'
lua = LuaRuntime(unpack_returned_tuples=True, encoding=None)
lua.execute(MOCK.encode())
lua.execute(b'''
LUI.modules={};function LUI:RegisterModule(module)table.insert(self.modules,module)end
function LUI:DB()return {marketEnabled=true}end
function GetServerTime()return 1794686400 end
function GetNormalizedRealmName()return 'Test Realm'end
function GetCurrentRegion()return 1 end
function UnitFactionGroup()return 'Alliance'end
LUI.ForeverEngine=true
bit={}
''')
bit=lua.globals().bit
bit.bxor=lambda a,b:int(a)^int(b)
bit.band=lambda a,b:int(a)&int(b)
bit.rshift=lambda a,n:(int(a)&0xffffffff)>>int(n)
bit.bnot=lambda a:(~int(a))&0xffffffff
for name in ['LookupEvents.lua','Modules/MarketCodec.lua','Modules/Market.lua']:
 lua.execute(b'assert(loadstring(...))("lamdaUI",LUI)',(addon/name).read_bytes())
lua.execute(b'''
M=LUI.Market;M.ready=true
local e=M:Event('auction');e.scan='fixture';e.page=1;e.pages=1;e.coverage='full'
e.lots=M.Array({{item='fish',price=20,quantity=1,count=5,divisible=false}})
e.items=M.Array({{id='fish',name='Raw Fish',transferable=true}})
-- Exercise more than one optical chunk with valid bounded metadata.
local seed=42
for i=1,700 do
 local chars={};for n=1,180 do seed=(seed*48271)%2147483647;chars[n]=string.char(33+seed%90)end
 e.items[#e.items+1]={id=tostring(i),name=table.concat(chars),transferable=true}
end
assert(M:Enqueue(e));PixelUtil={};M.bridge={width=512,height=64,frame=widget(),ApplyPosition=function()end,Paint=function()end}
M:Tick();result=M.packets;assert(#result>1,'Fixture must span multiple compressed packets')
''')
print(json.dumps([base64.b64encode(value).decode() for _,value in lua.globals().result.items()]))
