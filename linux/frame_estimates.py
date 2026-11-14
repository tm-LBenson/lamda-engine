#!/usr/bin/env python3
"""Transparent X11/XWayland presentation of engine frame-display.json.

All geometry and evidence comes from the engine. This process never reads game
memory, sends game input, or promotes an elapsed estimate to observed readiness.
GTK and Xlib are loaded only by main so model tests need no desktop packages.
"""

import argparse
import json
import math
import os
from pathlib import Path
import select
import signal
import shutil
import subprocess
import sys
import time

FRESH_SECONDS = 1.2
MAX_STATE_BYTES = 4 * 1024 * 1024
TEAL = "#6CD9CF"
AMBER = "#F0BC70"


def finite_number(value):
    try:
        return isinstance(value, (int, float)) and not isinstance(value, bool) and math.isfinite(value)
    except OverflowError:
        return False


def valid_dimensions(width, height):
    return (type(width) is int and type(height) is int
            and 1 <= width <= 16384 and 1 <= height <= 16384
            and width * height <= 64 * 1024 * 1024)


def frame_state_valid(state, now, expected_pid, window, width, height):
    """Fail closed on stale state, another foreground client, or a resize."""
    if not isinstance(state, dict) or state.get("pid") != expected_pid or expected_pid <= 0:
        return False
    data = state.get("frameDisplay")
    if not isinstance(data, dict) or data.get("enabled") is not True:
        return False
    if (not valid_dimensions(width, height) or data.get("width") != width
            or data.get("height") != height or not str(window).isdecimal()
            or int(window) <= 0 or data.get("window") != str(window)):
        return False
    at, expires = data.get("at"), data.get("expires")
    if not all(finite_number(v) for v in (now, at, expires)):
        return False
    if not (0 <= now - at <= FRESH_SECONDS and now < expires <= at + FRESH_SECONDS + .01):
        return False
    icons, bars, panel = data.get("icons") or [], data.get("kickBars") or [], data.get("kickPanel")
    overlays, notices = data.get("overlays") or [], data.get("notices") or []
    return (isinstance(icons, list) and len(icons) <= 4096
            and isinstance(bars, list) and len(bars) <= 256
            and all(isinstance(item, dict) for item in icons + bars)
            and (panel is None or isinstance(panel, dict))
            and isinstance(overlays, list) and len(overlays) <= 4096
            and all(isinstance(item, dict) for item in overlays + notices)
            and (bool(icons) or bool(bars) or panel is not None or bool(overlays)
                 or bool(notices) or bool(data.get("meters")) or bool(data.get("overlaySounds"))))


def estimate_view(entry, now):
    """Same unknown/estimate semantics as the Windows renderer."""
    status = entry.get("status", "unknown")
    ends, duration = entry.get("ends", 0), entry.get("duration", 0)
    delay = entry.get("deliverySeconds", 0)
    if not all(finite_number(v) for v in (now, ends, duration, delay)):
        raise ValueError("invalid estimate timing")
    remaining = math.ceil(ends - now)
    ready = status == "estimated_ready"
    unknown = (status not in ("estimated_cooldown", "estimated_recharge", "estimated_ready")
               or (remaining <= 0 and not ready))
    fraction = 0 if unknown else (1 if ready else min(1, max(0, (ends - now) / max(1, duration))))
    caption = str(max(0, remaining))
    if remaining >= 60:
        caption = f"{remaining // 60}:{remaining % 60:02}"
    return {"unknown": unknown, "ready": ready, "remaining": remaining,
            "fraction": fraction, "countdown": None if unknown or ready else caption,
            "badge": "?" if unknown else "~",
            "bar_text": "?" if unknown else ("~ Ready" if ready else f"~ {remaining}s"),
            "border": AMBER if unknown or entry.get("chargesUnknown") or delay > 2 else TEAL,
            "timer_color": AMBER if unknown or delay > 2 else "#FFFFFF"}


def visual_key(data, now):
    bars = data.get("kickBars", [])
    bar_step = math.floor(now * 5) if any(b.get("status") == "estimated_cooldown" for b in bars) else 0
    return (json.dumps([data["width"], data["height"], data.get("icons", []), bars,
                        data.get("kickPanel"), data.get("overlays"), data.get("notices"),
                        data.get("meters"), data.get("captureAreas")], sort_keys=True, separators=(",", ":")),
            math.floor(now * 10) if data.get("overlays") else math.floor(now), bar_step)


class StateFile:
    def __init__(self, path):
        self.path = Path(path)
        self.signature = None
        self.cached = None

    def read(self, now=None):
        stat = self.path.stat()
        if now is None:
            now = time.time()
        if not 0 <= now - stat.st_mtime <= FRESH_SECONDS or stat.st_size > MAX_STATE_BYTES:
            raise ValueError("frame state file is stale")
        signature = stat.st_ino, stat.st_size, stat.st_mtime_ns
        if signature != self.signature:
            # Opening an atomic replacement returns one complete inode snapshot.
            with self.path.open("rb") as file:
                raw = file.read(MAX_STATE_BYTES + 1)
            if len(raw) > MAX_STATE_BYTES:
                raise ValueError("frame state file is too large")
            state = json.loads(raw)
            self.cached, self.signature = state, signature
        return self.cached


def process_identity(pid):
    """Linux starttime distinguishes a reused PID from the original engine."""
    fields = Path(f"/proc/{pid}/stat").read_text().rsplit(")", 1)[1].split()
    if fields[0] == "Z":
        raise ProcessLookupError(pid)
    return fields[19]


class EngineGuard:
    def __init__(self, pid):
        self.pid = pid
        self.identity = process_identity(pid)
        self.fd = None
        self.poller = None
        if hasattr(os, "pidfd_open"):
            try:
                self.fd = os.pidfd_open(pid)
                self.poller = select.poll()
                self.poller.register(self.fd, select.POLLIN)
            except OSError:
                self.close()
        if process_identity(pid) != self.identity:
            self.close()
            raise ProcessLookupError(pid)

    def alive(self):
        if self.poller is not None:
            return not self.poller.poll(0)
        try:
            return process_identity(self.pid) == self.identity
        except (OSError, IndexError):
            return False

    def close(self):
        if self.fd is not None:
            os.close(self.fd)
        self.fd, self.poller = None, None


def drawing_modules():
    import gi
    gi.require_version("Gdk", "3.0")
    gi.require_version("GdkPixbuf", "2.0")
    gi.require_version("Pango", "1.0")
    gi.require_version("PangoCairo", "1.0")
    from gi.repository import Gdk, GdkPixbuf, Pango, PangoCairo
    import cairo
    return cairo, Gdk, GdkPixbuf, Pango, PangoCairo


class Painter:
    def __init__(self, icon_root):
        self.cairo, self.Gdk, self.Pixbuf, self.Pango, self.PangoCairo = drawing_modules()
        self.icon_root = Path(icon_root)
        self.icons = {}
        self.media = {}

    def media_art(self, path):
        if not path:
            return None
        if path not in self.media:
            try:
                art = self.Pixbuf.Pixbuf.new_from_file(str(path))
                if art.get_width() * art.get_height() > 16 * 1024 * 1024:
                    raise ValueError("oversize media")
                self.media[path] = art
            except Exception:
                self.media[path] = None
        return self.media[path]

    def icon(self, spell):
        if type(spell) is not int or not 0 <= spell <= 10000000:
            spell = 0
        if spell not in self.icons:
            art = None
            try:
                pixbuf = self.Pixbuf.Pixbuf.new_from_file(str(self.icon_root / f"{spell}.jpg"))
                w, h = pixbuf.get_width(), pixbuf.get_height()
                if not 8 <= w <= 256 or not 8 <= h <= 256:
                    raise ValueError("invalid icon dimensions")
                art = pixbuf.new_subpixbuf(3, 3, w - 6, h - 6)
            except Exception:
                if spell != 0:
                    art = self.icon(0)
            self.icons[spell] = art
        return self.icons[spell]

    @staticmethod
    def color(ctx, value, opacity=1):
        value = str(value).lstrip("#")
        if len(value) != 6:
            raise ValueError("invalid display color")
        rgb = [int(value[i:i + 2], 16) / 255 for i in (0, 2, 4)]
        ctx.set_source_rgba(*rgb, opacity)

    def rectangle(self, ctx, x, y, width, height, color, opacity=1):
        self.color(ctx, color, opacity)
        ctx.rectangle(x, y, width, height)
        ctx.fill()

    def label(self, ctx, text, size, color, x, y, width, height, align="center", bold=False, background=False):
        ctx.save()
        ctx.rectangle(x, y, width, height)
        ctx.clip()
        layout = self.PangoCairo.create_layout(ctx)
        font = self.Pango.FontDescription("Sans")
        font.set_absolute_size(size * self.Pango.SCALE)
        if bold:
            font.set_weight(self.Pango.Weight.BOLD)
        layout.set_font_description(font)
        layout.set_text(str(text), -1)
        layout.set_width(max(1, int(width * self.Pango.SCALE)))
        layout.set_single_paragraph_mode(True)
        layout.set_ellipsize(self.Pango.EllipsizeMode.END)
        layout.set_alignment(self.Pango.Alignment.CENTER if align == "center" else self.Pango.Alignment.LEFT)
        _, text_height = layout.get_pixel_size()
        top = y + max(0, (height - text_height) / 2)
        if background:
            # The countdown background covers only its text, not the artwork.
            extents = layout.get_pixel_extents()[1]
            self.rectangle(ctx, x + extents.x, top, extents.width, text_height, "#000000", .4)
        self.color(ctx, color)
        ctx.move_to(x, top)
        self.PangoCairo.show_layout(ctx, layout)
        ctx.restore()

    def artwork(self, ctx, spell, x, y, size, opacity=1):
        art = self.icon(spell)
        if art is None:
            return
        ctx.save()
        ctx.translate(x, y)
        ctx.scale(size / art.get_width(), size / art.get_height())
        self.Gdk.cairo_set_source_pixbuf(ctx, art, 0, 0)
        ctx.paint_with_alpha(opacity)
        ctx.restore()

    @staticmethod
    def number(entry, key, default=0, lower=-32768, upper=32768):
        number = entry.get(key, default)
        if not finite_number(number) or not lower <= number <= upper:
            raise ValueError(f"invalid display {key}")
        return number

    def begin_box(self, ctx, entry, width, height):
        x, y = self.number(entry, "x"), self.number(entry, "y")
        opacity = self.number(entry, "opacity", 100, 0, 100) / 100
        ctx.save()
        ctx.translate(x, y)
        ctx.rectangle(0, 0, width, height)
        ctx.clip()
        ctx.push_group()
        return opacity

    @staticmethod
    def end_box(ctx, opacity):
        ctx.pop_group_to_source()
        ctx.paint_with_alpha(opacity)
        ctx.restore()

    def draw_icon(self, ctx, entry, now):
        size = self.number(entry, "size", lower=1, upper=512)
        opacity = self.begin_box(ctx, entry, size, size)
        view = estimate_view(entry, now)
        self.rectangle(ctx, 0, 0, size, size, "#101923", .92)
        ctx.save()
        ctx.rectangle(1, 1, max(0, size - 2), max(0, size - 2))
        ctx.clip()
        self.artwork(ctx, entry.get("spell", 0), 1, 1, size - 2, .45 if view["unknown"] else 1)
        if not view["unknown"] and not view["ready"]:
            if entry.get("duration", 0) > 0:
                fraction = min(1, max(0, (entry["ends"] - now) / entry["duration"]))
                self.color(ctx, "#000000", .67)
                center, radius = size / 2, size * .72
                ctx.move_to(center, center)
                ctx.arc(center, center, radius, -math.pi / 2, -math.pi / 2 + math.tau * fraction)
                ctx.close_path()
                ctx.fill()
            self.label(ctx, view["countdown"], max(10, size * .43), "#FFFFFF",
                       1, 1, size - 2, size - 2, bold=True, background=True)
        badge_width, badge_height = max(10, size * .31), max(10, size * .35)
        badge_x = size - badge_width - 1 if view["unknown"] else 1
        self.rectangle(ctx, badge_x, 1, badge_width, badge_height, "#080B10", .85)
        self.label(ctx, view["badge"], max(8, size * .28), AMBER if view["unknown"] else TEAL,
                   badge_x, 1, badge_width, badge_height, bold=True)
        ctx.restore()
        self.color(ctx, view["border"])
        ctx.set_line_width(1)
        ctx.rectangle(.5, .5, size - 1, size - 1)
        ctx.stroke()
        self.end_box(ctx, opacity)

    def draw_header(self, ctx, panel):
        width = self.number(panel, "width", lower=1, upper=16384)
        opacity = self.begin_box(ctx, panel, width, 22)
        self.rectangle(ctx, 0, 0, width, 22, "#101923", .92)
        text = "LamdaKick   PREVIEW" if panel.get("preview") else "LamdaKick   ~ estimates"
        omitted = self.number(panel, "omitted", lower=0)
        if omitted:
            text += f"   +{int(omitted)} omitted"
        self.label(ctx, text, 11, "#C9E7EC", 1, 0, width - 2, 22)
        self.end_box(ctx, opacity)

    def draw_bar(self, ctx, entry, now):
        width = self.number(entry, "width", lower=1, upper=16384)
        height = self.number(entry, "height", lower=3, upper=512)
        opacity = self.begin_box(ctx, entry, width, height)
        view = estimate_view(entry, now)
        self.rectangle(ctx, 0, 0, width, height, "#101923", .93)
        if not view["unknown"]:
            self.rectangle(ctx, 1, 1, (width - 2) * view["fraction"], height - 2,
                           entry.get("color", "#8995A5"), .38)
        self.artwork(ctx, entry.get("spell", 0), 1, 1, height - 2, .45 if view["unknown"] else 1)
        names_x, names_width = height + 4, max(1, width - height - 77)
        name_height = height * .57 if height >= 26 else height
        self.label(ctx, entry.get("player", ""), min(16, max(9, height * .40)), "#FFFFFF",
                   names_x, 0, names_width, name_height, align="left", bold=True)
        if height >= 26:
            self.label(ctx, entry.get("name", ""), min(11, height * .30), "#C4CED8",
                       names_x, name_height, names_width, height - name_height, align="left")
        self.label(ctx, view["bar_text"], min(16, max(10, height * .43)), view["timer_color"],
                   width - 69, 1, 68, height - 2, bold=True)
        self.color(ctx, "#596574")
        ctx.set_line_width(1)
        ctx.rectangle(.5, .5, width - 1, height - 1)
        ctx.stroke()
        self.end_box(ctx, opacity)

    def draw_custom(self, ctx, entry, now, client_width, client_height):
        if entry.get("ends", 0) <= now:
            return
        visual = entry.get("visual") or {}
        kind, text = entry.get("kind", "text"), entry.get("text", "")
        size, font = min(400, max(16, entry.get("size", 48))), min(72, max(10, entry.get("font", 16)))
        width, height = size, size
        if kind in ("text", "bar"):
            width, height = max(size * 3, len(text) * font), max(32, font * 2)
        width, height = min(client_width, visual.get("width") or width), min(client_height, visual.get("height") or height)
        x = entry.get("pixelX", min(client_width - width, max(0, entry.get("x", 0) / 100 * client_width)))
        y = entry.get("pixelY", min(client_height - height, max(0, entry.get("y", 0) / 100 * client_height)))
        color = entry.get("color", "#FFFFFF")
        fraction = entry.get("fraction", 1 if entry.get("persistent") else (entry["ends"] - now) / max(.001, entry.get("duration", 1)))
        fraction = min(1, max(0, fraction))
        ctx.save()
        ctx.translate(x, y)
        ctx.translate(width / 2, height / 2)
        ctx.rotate(math.radians(visual.get("rotation", 0)))
        ctx.scale(-1 if visual.get("flipX") else 1, -1 if visual.get("flipY") else 1)
        ctx.translate(-width / 2, -height / 2)
        ctx.push_group()
        if not entry.get("transparent"):
            self.rectangle(ctx, 0, 0, width, height, "#101923", .86)
        if kind in ("arc", "ring"):
            arc = entry.get("arc") or {}
            if not arc.get("timed", True):
                fraction = min(1, max(0, entry.get("fraction", arc.get("fraction", 1))))
            span = min(359.9, arc.get("sweep", 359.9 if kind == "ring" else 270))
            thickness, gap = arc.get("thickness", size * .06), arc.get("gap", 0)
            capacity = min(20, max(1, arc.get("capacity", 1)))
            count = min(capacity, max(0, arc.get("count", 1)))
            slots = min(4, capacity) if arc.get("layout") == "around" else 1
            if slots > 1:
                span = min(span, 180 if capacity <= 2 else 90)
            radius = size * .43
            fit = min(1, (radius - 1) / max(.001, thickness / 2 + (math.ceil(capacity / slots) - 1) * (thickness + gap)))
            thickness, gap = thickness * fit, gap * fit
            self.color(ctx, color)
            ctx.set_line_width(thickness)
            ctx.set_line_cap(self.cairo.LINE_CAP_ROUND)
            ctx.save()
            ctx.scale(width / size, height / size)
            for i in range(count):
                r = radius - (i // slots) * (thickness + gap)
                anchor = arc.get("start", -135) + span / 2
                if slots > 1:
                    anchors = [270, 90, 180, 0] if arc.get("pair") == "vertical" else [180, 0, 270, 90]
                    anchor = anchors[i % slots] + arc.get("start", 0)
                direction = -1 if arc.get("reverse") else 1
                start = math.radians(anchor - direction * span / 2)
                finish = start + direction * math.radians(span * fraction)
                if fraction > 0:
                    ctx.new_sub_path()
                    (ctx.arc_negative if direction < 0 else ctx.arc)(size / 2, size / 2, r, start, finish)
                    ctx.stroke()
            ctx.restore()
        elif kind in ("icon", "image"):
            art = self.media_art(entry.get("imagePath") if kind == "image" else entry.get("iconPath"))
            if art is None and kind == "icon":
                art = self.icon(entry.get("spell", 0))
            if art is not None:
                ctx.save()
                ctx.rectangle(0, 0, width, height)
                ctx.clip()
                ctx.scale(width / art.get_width(), height / art.get_height())
                self.Gdk.cairo_set_source_pixbuf(ctx, art, 0, 0)
                ctx.paint()
                ctx.restore()
        elif kind == "shape":
            def shape(fill_color, alpha, progress):
                ctx.save()
                self.progress_clip(ctx, width, height, progress, visual.get("direction"))
                self.color(ctx, fill_color, alpha)
                name = visual.get("shape", "rectangle")
                if name == "ellipse":
                    ctx.translate(width / 2, height / 2); ctx.scale(width / 2, height / 2); ctx.arc(0, 0, 1, 0, math.tau)
                elif name in ("triangle", "diamond"):
                    points = [(width / 2, 0), (width, height), (0, height)] if name == "triangle" else [(width / 2, 0), (width, height / 2), (width / 2, height), (0, height / 2)]
                    ctx.move_to(*points[0])
                    for point in points[1:]: ctx.line_to(*point)
                    ctx.close_path()
                else:
                    ctx.rectangle(0, 0, width, height)
                ctx.fill(); ctx.restore()
            shape(visual.get("trackColor", "#101923"), visual.get("trackOpacity", 0) / 100, 1)
            shape(color, visual.get("fillOpacity", 100) / 100, fraction)
        else:
            if kind == "bar":
                self.rectangle(ctx, 0, 0, width, height, visual.get("trackColor", "#101923"), visual.get("trackOpacity", 0) / 100)
                ctx.save()
                self.progress_clip(ctx, width, height, fraction, visual.get("direction"))
                self.rectangle(ctx, 0, 0, width, height, color, visual.get("fillOpacity", 30) / 100)
                ctx.restore()
            self.label(ctx, text, font, color, 0, 0, width, height, align=visual.get("textAlign", "center"), bold=visual.get("bold", True))
        ctx.pop_group_to_source()
        ctx.paint_with_alpha(min(1, max(0, entry.get("opacity", 100) / 100)))
        ctx.restore()

    @staticmethod
    def progress_clip(ctx, width, height, fraction, direction):
        x, y, w, h = 0, 0, width, height
        if direction in ("up", "down"):
            h *= fraction
            if direction == "up": y = height - h
        else:
            w *= fraction
            if direction == "left": x = width - w
        ctx.rectangle(x, y, w, h)
        ctx.clip()

    def render(self, data, now):
        width, height = data["width"], data["height"]
        if not valid_dimensions(width, height):
            raise ValueError("invalid client dimensions")
        surface = self.cairo.ImageSurface(self.cairo.FORMAT_ARGB32, width, height)
        ctx = self.cairo.Context(surface)
        # The final clear also removes shadows/antialiasing from capture holes.
        for entry in sorted(data.get("overlays") or [], key=lambda e: (e.get("visual") or {}).get("layer", 0)):
            self.draw_custom(ctx, entry, now, width, height)
        for entry in data.get("notices") or []:
            if entry.get("ends", 0) > now:
                self.label(ctx, entry.get("text", ""), 11, "#F3D887", entry["x"], entry["y"], entry["width"], 18)
        for entry in data.get("icons", []):
            self.draw_icon(ctx, entry, now)
        if data.get("kickPanel") is not None:
            self.draw_header(ctx, data["kickPanel"])
        for entry in data.get("kickBars", []):
            if not entry.get("native"):
                self.draw_bar(ctx, entry, now)
        ctx.set_operator(self.cairo.OPERATOR_CLEAR)
        for area in data.get("captureAreas") or []:
            ctx.rectangle(area["x"], area["y"], area["width"], area["height"])
            ctx.fill()
        surface.flush()
        return surface


class Foreground:
    def __init__(self):
        from Xlib import Xatom, display
        self.Xatom = Xatom
        self.display = display.Display()
        self.root = self.display.screen().root
        self.active_atom = self.display.intern_atom("_NET_ACTIVE_WINDOW")

    def bounds(self):
        value = self.root.get_full_property(self.active_atom, self.Xatom.WINDOW)
        if value is None or len(value.value) != 1 or int(value.value[0]) == 0:
            return None
        xid = int(value.value[0])
        client = self.display.create_resource_object("window", xid)
        if client.get_attributes().map_state != 2:
            return None
        geometry = client.get_geometry()
        # TranslateCoords is invoked on the destination window in python-xlib.
        origin = self.root.translate_coords(client, 0, 0)
        if not origin.same_screen:
            return None
        return xid, origin.x, origin.y, geometry.width, geometry.height

    def close(self):
        self.display.close()


def run_overlay(args):
    # Force physical X client pixels even on a scaled Wayland desktop. WoW via
    # Wine/Proton and this overlay must share the same XWayland coordinate space.
    os.environ["GDK_BACKEND"] = "x11"
    os.environ["GDK_SCALE"] = "1"
    os.environ["GDK_DPI_SCALE"] = "1"
    import gi
    gi.require_version("Gtk", "3.0")
    from gi.repository import Gtk, Gdk, GLib
    import cairo

    if not Gtk.init_check()[0]:
        raise RuntimeError("Linux frame display needs an X11/XWayland DISPLAY")
    screen = Gdk.Screen.get_default()
    visual = screen.get_rgba_visual()
    if visual is None or not screen.is_composited():
        raise RuntimeError("Linux frame display requires an RGBA compositing desktop")
    guard = EngineGuard(args.engine_pid)
    foreground = Foreground()
    reader, painter = StateFile(args.state), Painter(args.icons)
    # An unmanaged popup has no WM wrapper that could intercept mouse input or
    # reposition a notification. This is a plain Gtk.Window, never a menu: it
    # does not take an input grab. Coordinates stay exact across monitors.
    window = Gtk.Window(type=Gtk.WindowType.POPUP)
    window.set_title("LamdaUI frame estimates")
    window.set_visual(visual)
    window.set_app_paintable(True)
    window.set_decorated(False)
    window.set_resizable(True)
    window.set_accept_focus(False)
    window.set_focus_on_map(False)
    window.set_skip_taskbar_hint(True)
    window.set_skip_pager_hint(True)
    window.set_type_hint(Gdk.WindowTypeHint.NOTIFICATION)
    window.set_keep_above(True)
    window.set_default_size(1, 1)
    # Store the empty shape on the widget as well as its native window. GTK
    # otherwise restores its default full input region when it maps/resizes.
    window.input_shape_combine_region(cairo.Region())
    runtime = {"surface": None, "key": None, "bounds": None, "error": None}
    sounds_seen = set()
    sound_player = next((p for p in ("pw-play", "paplay", "aplay") if shutil.which(p)), None)
    sound_children = []

    def realized(widget, *_):
        native = widget.get_window()
        if native is None:
            return
        native.set_pass_through(True)
        native.input_shape_combine_region(cairo.Region(), 0, 0)

    def draw(widget, ctx):
        ctx.set_operator(cairo.OPERATOR_SOURCE)
        ctx.set_source_rgba(0, 0, 0, 0)
        ctx.paint()
        if runtime["surface"] is not None:
            ctx.set_source_surface(runtime["surface"], 0, 0)
            ctx.paint()
        return True

    def hide():
        if window.get_visible():
            window.hide()

    def tick():
        if not guard.alive():
            Gtk.main_quit()
            return False
        try:
            state = reader.read()
            bounds = foreground.bounds()
            if bounds is None or not screen.is_composited():
                hide()
                return True
            xid, x, y, width, height = bounds
            # A writer can replace the state while we query X11. Compare its
            # payload with a clock sampled after the read, avoiding false future
            # timestamps and flicker around an engine heartbeat.
            now = time.time()
            if not frame_state_valid(state, now, args.engine_pid, str(xid), width, height):
                hide()
                return True
            data = state["frameDisplay"]
            sound_children[:] = [p for p in sound_children if p.poll() is None]
            for sound in data.get("overlaySounds") or []:
                token = sound.get("id")
                if token not in sounds_seen and 0 <= now - sound.get("at", 0) <= FRESH_SECONDS:
                    sounds_seen.add(token)
                    path = sound.get("path") or str(Path(__file__).parent / "sounds" / (sound.get("name", "alert") + ".wav"))
                    if sound_player and Path(path).is_file() and len(sound_children) < 4:
                        sound_children.append(subprocess.Popen([sound_player, path], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL))
            key = visual_key(data, now)
            if key != runtime["key"]:
                # Finish the new render before replacing the previous image.
                runtime["surface"] = painter.render(data, now)
                runtime["key"] = key
                window.queue_draw()
            if bounds != runtime["bounds"] or not window.get_visible():
                window.move(x, y)
                window.resize(width, height)
                runtime["bounds"] = bounds
            if not window.get_visible():
                window.show()
                window.get_window().raise_()
            runtime["error"] = None
        except (FileNotFoundError, ValueError, json.JSONDecodeError):
            hide()
        except Exception as exc:
            hide()
            message = str(exc)
            if message != runtime["error"]:
                print("Frame display: " + message, file=sys.stderr, flush=True)
                runtime["error"] = message
        return True

    window.connect("realize", realized)
    window.connect_after("map-event", realized)
    window.connect_after("size-allocate", realized)
    window.connect("draw", draw)
    window.connect("destroy", lambda *_: Gtk.main_quit() if Gtk.main_level() else None)
    window.realize()
    timer = GLib.timeout_add(100, tick)
    for number in (signal.SIGINT, signal.SIGTERM):
        GLib.unix_signal_add(GLib.PRIORITY_DEFAULT, number, lambda: (Gtk.main_quit(), False)[1])
    try:
        Gtk.main()
    finally:
        if GLib.MainContext.default().find_source_by_id(timer) is not None:
            GLib.source_remove(timer)
        window.destroy()
        foreground.close()
        guard.close()
        for child in sound_children:
            if child.poll() is None:
                child.terminate()


def icon_directory():
    here = Path(__file__).resolve().parent
    for candidate in (here / "icons", here.parent / "windows" / "icons"):
        if candidate.is_dir():
            return candidate
    return here / "icons"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--state", required=True)
    parser.add_argument("--engine-pid", type=int, default=0)
    parser.add_argument("--icons", type=Path, default=icon_directory())
    parser.add_argument("--render-test", type=Path, help="Render one state snapshot to a PNG without showing a window")
    args = parser.parse_args()
    try:
        if args.render_test:
            with open(args.state, encoding="utf-8") as file:
                data = json.load(file)["frameDisplay"]
            Painter(args.icons).render(data, data["at"]).write_to_png(str(args.render_test))
        elif args.engine_pid <= 0:
            parser.error("--engine-pid is required for a live display")
        else:
            run_overlay(args)
    except Exception as exc:
        print("Frame display: " + str(exc), file=sys.stderr, flush=True)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
