"""Minimal X11/XWayland overlay. Requires system tkinter and python-xlib."""
import argparse
import json
import os
from pathlib import Path
import time
import tkinter as tk
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
labels = []
state_path = Path(args.state)

def game_foreground():
    prop = screen.get_full_property(active_atom, X.AnyPropertyType)
    if prop is None or not len(prop.value) or not prop.value[0]:
        return False
    window = connection.create_resource_object('window', int(prop.value[0]))
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

def click_through():
    root.update_idletasks()
    for wid in (root.winfo_id(), int(root.tk.call('wm', 'frame', root._w), 0)):
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
        cfg = state['config']
        lines = []
        if args.preview or cfg.get("preview", False):
            lines.append('Preview')
        for row in state['rows']:
            remaining = int(__import__('math').ceil(row['ends'] - time.time()))
            if remaining > 0 and cfg['enabled']:
                charge = '*' if row['charges'] else ''
                suffix = "" if row.get("observedOnly") else f"  ~{remaining}s{charge}"
                lines.append(f"{row['player']}  {row['name']}{suffix}")
        if state.get('update'):
            lines.append('LamdaUI update available: ' + state['update'])
        if not lines:
            root.withdraw()
            return
        while len(labels) < len(lines):
            label = tk.Label(root, bg='#0f1723', fg='#edf9fa', anchor='w', padx=10, pady=5)
            labels.append(label)
        for i, label in enumerate(labels):
            if i < len(lines):
                label.configure(text=lines[i], font=('DejaVu Sans', max(7, round(11 * cfg['scale']))))
                label.pack(fill='x')
            else:
                label.pack_forget()
        root.geometry(f"+{int(cfg['x'])}+{int(cfg['y'])}")
        root.deiconify()
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
