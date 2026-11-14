"""X11/XWayland metadata for one configured Wine/Proton game installation.

Only window metadata and /proc process name, owner, cwd and executable arguments
are inspected. No process memory, environment, game input or title fallback.
"""
import os
from pathlib import Path
from Xlib import X, display


def game_process(pid, game, client):
    game = Path(game).resolve()
    names = {'Wow.exe', 'WowT.exe'}
    if client == 'forever':
        names = {'Wow.exe'} if game.name.lower() == '_forever_' else {'WowB.exe'}
    try:
        proc = Path('/proc') / str(pid)
        if proc.stat().st_uid != os.getuid():
            return False
        name = (proc / 'comm').read_text().strip()
        if name.lower() not in {n.lower() for n in names}:
            return False
        # Wine/Proton normally sets the Unix working directory to the Windows
        # executable's directory. Match that exact installation, not its title.
        if (proc / 'cwd').resolve() == game:
            return any((game / n).is_file() for n in names)
        # A native absolute executable argument is also unambiguous. Do not
        # guess how an arbitrary Windows drive letter maps to a Wine prefix.
        for arg in (proc / 'cmdline').read_bytes().split(b'\0'):
            value = os.fsdecode(arg)
            if value.startswith('/') and Path(value).name in names:
                if Path(value).resolve() == game / Path(value).name:
                    return Path(value).is_file()
    except (OSError, ValueError):
        pass
    return False


class Desktop:
    def __init__(self):
        self.connection = display.Display()
        self.root = self.connection.screen().root
        self.active_atom = self.connection.intern_atom('_NET_ACTIVE_WINDOW')
        self.pid_atom = self.connection.intern_atom('_NET_WM_PID')

    def close(self):
        self.connection.close()

    def foreground(self):
        prop = self.root.get_full_property(self.active_atom, X.AnyPropertyType)
        if prop is not None and len(prop.value) and prop.value[0]:
            return self.connection.create_resource_object('window', int(prop.value[0]))
        # Minimal WMs may omit EWMH. Walk the input-focus window to its client.
        window = self.connection.get_input_focus().focus
        if not hasattr(window, 'query_tree'):
            return None
        for _ in range(16):
            if window.get_full_property(self.pid_atom, X.AnyPropertyType) is not None:
                return window
            parent = window.query_tree().parent
            if parent.id == self.root.id:
                return window
            window = parent
        return None

    def metadata(self, window):
        if window is None or window.get_attributes().map_state != X.IsViewable:
            return None
        prop = window.get_full_property(self.pid_atom, X.AnyPropertyType)
        if prop is None or not len(prop.value):
            return None
        geometry = window.get_geometry()
        origin = self.root.translate_coords(window, 0, 0)
        return dict(window=window.id, pid=int(prop.value[0]), width=geometry.width,
                    height=geometry.height, x=origin.x, y=origin.y, foreground=True)

    def game(self, path, client):
        window = self.foreground()
        info = self.metadata(window)
        if not info or not game_process(info['pid'], path, client):
            return None, None
        return window, info
