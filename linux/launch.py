#!/usr/bin/env python3
"""Build and run the native Linux engine from either supported source layout."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--client', choices=('retail', 'forever'), default='retail')
    parser.add_argument('--wow', type=Path, help='Linux path to _retail_ or _forever_ inside the Steam/Wine installation')
    parser.add_argument('--config', type=Path, help='Optional explicit lamdaUI.lua SavedVariables path')
    parser.add_argument('--state-dir', type=Path)
    parser.add_argument('--workspace', action='store_true', help='Browser workspace only; do not connect to WoW')
    parser.add_argument('--check', action='store_true', help='Read-only dependency and desktop report')
    parser.add_argument('--build-only', action='store_true')
    args = parser.parse_args()
    if not sys.platform.startswith('linux'):
        parser.error('Run this launcher on Linux.')
    dependencies = {name: importlib.util.find_spec(name) is not None for name in ('Xlib', 'gi', 'cairo')}
    if args.check:
        print(json.dumps(dict(python=sys.executable, go=shutil.which('go'), dependencies=dependencies,
                              display=os.environ.get('DISPLAY', ''), session=os.environ.get('XDG_SESSION_TYPE', ''),
                              live_backend='X11 / XWayland', native_wayland=False), indent=2))
        return
    live = args.wow is not None and not args.workspace
    if args.config and not args.wow:
        parser.error('--config requires --wow')
    if live:
        if not all(dependencies.values()):
            parser.error('Missing Linux desktop libraries. See linux/README.md; use /usr/bin/python3 with system packages.')
        if not os.environ.get('DISPLAY'):
            parser.error('Start in your graphical session with an X11/XWayland DISPLAY.')
        args.wow = args.wow.expanduser().resolve(strict=True)
        if args.config:
            args.config = args.config.expanduser().resolve(strict=True)
            if not args.config.is_relative_to(args.wow / 'WTF' / 'Account'):
                parser.error('SavedVariables must belong to the selected WoW client.')
        names = ('WowB.exe',) if args.client == 'forever' and args.wow.name.lower() != '_forever_' else ('Wow.exe', 'WowT.exe')
        if not any((args.wow / name).is_file() for name in names):
            parser.error('No matching WoW executable in --wow.')
    if not shutil.which('go'):
        parser.error('Go 1.25 or later is required to build this source checkout.')
    destination = ROOT / 'bin' / ('linux-' + args.client)
    destination.mkdir(parents=True, exist_ok=True)
    binary = destination / 'lamda-engine'
    environment = dict(os.environ, GOOS='linux', CGO_ENABLED='0')
    subprocess.run(['go', 'build', '-trimpath', '-ldflags=-X main.version=0.31.7-linux.2', '-o', str(binary) + '.new', './cmd/lamda-engine'], cwd=ROOT, env=environment, check=True)
    Path(str(binary) + '.new').replace(binary)
    helper_dir = destination / 'linux'
    helper_dir.mkdir(exist_ok=True)
    for name in ('capture.py', 'x11.py', 'frame_estimates.py'):
        shutil.copy2(ROOT / 'linux' / name, helper_dir / name)
    shutil.copytree(ROOT / 'windows' / 'icons', helper_dir / 'icons', dirs_exist_ok=True)
    shutil.copytree(ROOT / 'internal' / 'overlays' / 'web' / 'sounds', helper_dir / 'sounds', dirs_exist_ok=True)
    if args.build_only:
        print(binary)
        return
    state = args.state_dir or Path(os.environ.get('XDG_CONFIG_HOME', str(Path.home() / '.config'))) / 'LamdaUI' / args.client.capitalize()
    command = [str(binary), '--client', args.client, '--state-dir', str(state.expanduser().resolve())]
    if live:
        command += ['--wow', str(args.wow)]
        if args.config:
            command += ['--config', str(args.config)]
        print('Live X11/XWayland capture. WoW must be foreground with visible color strips.', flush=True)
    else:
        command += ['--sim-workspace']
    print('Workspace: http://127.0.0.1:' + ('47642' if args.client == 'forever' else '47641'), flush=True)
    print('Data: ' + str(state) + '\nCtrl+C stops this engine and its helpers.', flush=True)
    os.execvpe(command[0], command, dict(os.environ, LAMDA_PYTHON=sys.executable))


if __name__ == '__main__':
    main()
