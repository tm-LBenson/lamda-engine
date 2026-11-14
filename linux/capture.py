"""Bounded pixel RPC for the Go reader. Never saves screenshots or sends input."""
import argparse
import base64
import json
import sys
from Xlib import X, error
from x11 import Desktop


def serve(game, client):
    desktop = Desktop()
    try:
        for line in sys.stdin:
            try:
                if len(line) > 4096:
                    raise ValueError('request too large')
                request = json.loads(line)
                window, info = desktop.game(game, client)
                if window is None:
                    result = dict(error='bridge_game_not_foreground')
                elif request.get('op') == 'context':
                    result = info
                elif request.get('op') == 'capture':
                    expected = request.get('expected', {})
                    if any(info[key] != expected.get(key) for key in ('window', 'pid', 'width', 'height', 'x', 'y')):
                        result = dict(error='bridge_window_changed')
                    else:
                        x, y, width, height = (int(request[k]) for k in ('x', 'y', 'width', 'height'))
                        if x < 0 or y < 0 or width < 1 or height < 1 or width * height > 65536 or x + width > info['width'] or y + height > info['height']:
                            raise ValueError('capture bounds')
                        # Read the foreground game's drawable, which also works
                        # with rootless XWayland. Other desktop windows are never
                        # captured. Require ordinary little-endian TrueColor.
                        raw = window.get_image(x, y, width, height, X.ZPixmap, 0xffffffff)
                        formats = desktop.connection.display.info
                        visual_id = window.get_attributes().visual
                        visual = next((v for depth in desktop.connection.screen().allowed_depths for v in depth.visuals if v.visual_id == visual_id), None)
                        masks = (visual.red_mask, visual.green_mask, visual.blue_mask) if visual else ()
                        if raw.depth not in (24, 32) or masks != (0xff0000, 0xff00, 0xff) or formats.image_byte_order != X.LSBFirst or len(raw.data) != width * height * 4:
                            result = dict(error='bridge_linux_pixel_format')
                        else:
                            _, current = desktop.game(game, client)
                            result = dict(info, pixels=base64.b64encode(raw.data).decode('ascii')) if current == info else dict(error='bridge_window_changed')
                else:
                    raise ValueError('unknown operation')
            except (ValueError, KeyError, TypeError, OSError, error.XError):
                result = dict(error='bridge_capture_failed')
            print(json.dumps(result, separators=(',', ':')), flush=True)
    finally:
        desktop.close()


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--wow', required=True)
    parser.add_argument('--client', choices=('retail', 'forever'), required=True)
    args = parser.parse_args()
    serve(args.wow, args.client)
