"""Minimal X11/XWayland overlay. Requires system tkinter and python-xlib."""
import argparse
import json
import os
from pathlib import Path
import time
import tkinter as tk
import tkinter.font as tkfont
import math
from Xlib import X, display, error
from Xlib.ext import shape

parser = argparse.ArgumentParser()
parser.add_argument('--state', required=True)
parser.add_argument('--engine-pid', required=True, type=int)
parser.add_argument('--preview', action='store_true', help='Show explicitly labelled preview rows for display testing')
args = parser.parse_args()
root = tk.Tk(className='LamdaOverlay')
root.title('LamdaUI Overlay')
root.withdraw()
root.overrideredirect(True)
root.attributes('-topmost', True)
root.configure(background='#0f1723')
connection = display.Display()
screen = connection.screen().root
active_atom = connection.intern_atom('_NET_ACTIVE_WINDOW')
pid_atom = connection.intern_atom('_NET_WM_PID')
canvas=tk.Canvas(root,highlightthickness=0,bg="#0f1723")
canvas.pack()
game_rect=(0,0,root.winfo_screenwidth(),root.winfo_screenheight())
font=tkfont.Font(family="DejaVu Sans",size=-14)
state_path = Path(args.state)

def game_foreground():
    global game_rect
    prop = screen.get_full_property(active_atom, X.AnyPropertyType)
    if prop is None or not len(prop.value) or not prop.value[0]:
        return False
    window = connection.create_resource_object('window', int(prop.value[0]))
    geometry=window.get_geometry()
    origin=screen.translate_coords(window,0,0)
    game_rect=(origin.x,origin.y,geometry.width,geometry.height)
    pid = window.get_full_property(pid_atom, X.AnyPropertyType)
    if pid is not None and len(pid.value):
        try:
            name = Path(f'/proc/{pid.value[0]}/comm').read_text().strip().lower()
            if name in ('wow.exe', 'wowt.exe', 'wow', 'wowt'):
                return True
        except OSError:
            pass
    # Some Wine windows do not supply a process ID; use the window's own title.
    return (window.get_wm_name() or '').lower() == 'world of warcraft'

def outer_window():
    win=connection.create_resource_object('window',root.winfo_id())
    for _ in range(8):
        parent=win.query_tree().parent
        if parent.id==screen.id:return win
        win=parent
    return win

def click_through():
    root.update_idletasks()
    for wid in (root.winfo_id(), outer_window().id):
        win = connection.create_resource_object('window', wid)
        win.shape_rectangles(shape.SO.Set, shape.SK.Input, X.Unsorted, 0, 0, [])
    connection.sync()

def update():
    try:
        os.kill(args.engine_pid, 0)
        if not args.preview and not game_foreground():
            root.withdraw()
            return
        if time.time() - state_path.stat().st_mtime > 3:
            root.withdraw()
            return
        state = json.loads(state_path.read_text())
        if state['pid'] != args.engine_pid:
            root.withdraw()
            return
        cfg=state['config'];layout=state['layout']
        if not layout['width'] or not layout['height']:
            root.withdraw();return
        w,h=layout['width'],layout['height']
        canvas.configure(width=w,height=h)
        root.attributes('-alpha',cfg['opacity']/100)
        canvas.delete('all')
        font.configure(size=-layout['fontSize'])
        accent=['#35bfa7','#3485d5','#9567d8','#d88934'][cfg['accent']-1]
        bar_color='#'+''.join(f'{round(int(accent[i:i+2],16)*.25+int("142331"[i-1:i+1],16)*.75):02x}' for i in (1,3,5))
        header=0
        if args.preview or (cfg['preview'] and cfg['enabled']):
            canvas.create_text(8,layout['headerStep']/2,text='Preview',anchor='w',fill=accent,font=font);header+=1
        if state.get('update'):
            canvas.create_text(8,header*layout['headerStep']+layout['headerStep']/2,text='LamdaUI update available: '+state['update'],anchor='w',fill=accent,font=font)
        for row,cell in zip(state['rows'],layout['cells']):
            remaining=max(0,row['ends']-time.time())
            if not remaining or not cfg['enabled']:continue
            x,y=cell['x'],cell['y'];rw,rh=layout['rowWidth'],layout['rowHeight']
            canvas.create_rectangle(x,y,x+rw-1,y+rh-1,fill='#142331',outline=accent if cfg['border'] else '')
            duration=row.get('duration',0)
            if cfg['bars'] and duration>0 and not row.get('observedOnly'):
                fill=max(0,min(1,remaining/duration))
                canvas.create_rectangle(x+1,y+1,x+1+(rw-2)*fill,y+rh-2,fill=bar_color,outline='')
            parts=[]
            if cfg['showNames']:parts.append(row['player'])
            if cfg['showSpells']:parts.append(row['name'])
            timer=''
            if cfg['showTimers'] and not row.get('observedOnly'):
                timer=f"~{math.ceil(remaining)}s"+('*' if row.get('charges') else '')
            text='  '.join(parts);limit=rw-16-(font.measure(timer)+12 if timer else 0)
            if font.measure(text)>limit:
                while text and font.measure(text+'…')>limit:text=text[:-1]
                text+='…'
            canvas.create_text(x+8,y+rh/2,text=text,anchor='w',fill='#edf9fa',font=font)
            if timer:canvas.create_text(x+rw-8,y+rh/2,text=timer,anchor='e',fill='#edf9fa',font=font)
        gx,gy,gw,gh=game_rect
        left=round(gx+(gw-w)*layout['anchorX']+cfg['x'])
        top=round(gy+(gh-h)*layout['anchorY']+cfg['y'])
        # Move through X11 for signed virtual-desktop coordinates (Tk negative geometry
        # offsets otherwise mean distance from the right/bottom edge).
        root.geometry(f"{w}x{h}")
        root.deiconify()
        root.update_idletasks()
        frame=outer_window()
        frame.configure(x=left,y=top)
        connection.sync()
        click_through()
    except ProcessLookupError:
        root.quit()
        return
    except (OSError, ValueError, KeyError, tk.TclError, error.XError):
        root.withdraw()
    finally:
        if root.winfo_exists():
            root.after(250, update)

root.after(0, update)
try:
    root.mainloop()
finally:
    connection.close()
