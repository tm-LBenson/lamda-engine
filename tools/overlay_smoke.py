"""Visual X11 smoke check. Briefly displays labelled preview rows; no game data."""
import json,subprocess,time,os,tempfile
from pathlib import Path
from Xlib import display,X
from PIL import Image
ROOT=Path(__file__).resolve().parents[1]
c=dict(enabled=True,preview=True,opacity=95,accent=1,border=True,bars=True,showNames=True,showSpells=True,showTimers=True,x=0,y=0)
l=dict(width=683,height=95,rowWidth=340,rowHeight=34,fontSize=14,headerStep=24,anchorX=.5,anchorY=.5,cells=[{'x':0,'y':24},{'x':343,'y':24},{'x':0,'y':61}])
with tempfile.TemporaryDirectory() as d:
 state=Path(d)/'state.json';state.write_text(json.dumps(dict(pid=os.getpid(),config=c,layout=l,rows=[dict(player='Example',name='Mirror Image',ends=time.time()+60,duration=120,charges=False),dict(player='Companion',name='Barrier',ends=time.time()+20,duration=30,charges=False),dict(player='Companion',name='Interrupt',ends=time.time()+8,observedOnly=True,charges=False)])))
 p=subprocess.Popen(['python3',str(ROOT/'linux/overlay.py'),'--state',str(state),'--engine-pid',str(os.getpid()),'--preview'],stderr=subprocess.PIPE)
 try:
  time.sleep(.8)
  if p.poll() is not None:raise RuntimeError(p.stderr.read().decode())
  conn=display.Display();root=conn.screen().root;found=False
  for w in root.query_tree().children:
   if w.get_wm_name()!='LamdaUI Overlay':continue
   g=w.get_geometry()
   if g.width!=683:continue
   origin=root.translate_coords(w,0,0);print('Preview:',g.width,g.height,'at',origin.x,origin.y)
   assert g.height==95;assert abs(origin.x-(conn.screen().width_in_pixels-683)/2)<=1
   assert abs(origin.y-(conn.screen().height_in_pixels-95)/2)<=1
   from Xlib.ext import shape
   assert len(w.shape_get_rectangles(shape.SK.Input).rectangles)==0
   raw=w.get_image(0,0,g.width,g.height,X.ZPixmap,0xffffffff)
   Image.frombytes('RGB',(g.width,g.height),raw.data,'raw','BGRX').save('/tmp/lamda-customization.png');found=True;break
  if not found:raise RuntimeError('New preview not visible')
  conn.close()
 finally:p.terminate();p.wait(timeout=3)
