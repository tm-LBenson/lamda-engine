"""Linux frame presentation: fail closed, preserve estimate semantics, render alpha.

The core tests run without desktop libraries. Real Cairo rendering checks run
when the system GTK/PyGObject packages are available, even without a DISPLAY.
"""

import copy
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("linux_frame_display", ROOT / "linux/frame_estimates.py")
display = importlib.util.module_from_spec(spec)
spec.loader.exec_module(display)


def fixture(now=100):
    icon = {"x": 20, "y": 30, "size": 48, "opacity": 100, "spell": 57994,
            "status": "estimated_cooldown", "ends": now + 72, "duration": 90,
            "deliverySeconds": 0, "chargesUnknown": False}
    bar = dict(icon, x=230, y=64, width=300, height=36, player="Teammate",
               name="Wind Shear", color="#0070DD")
    data = {"enabled": True, "window": "123", "width": 640, "height": 300,
            "at": now, "expires": now + 1.2, "icons": [icon], "kickBars": [bar],
            "kickPanel": {"x": 230, "y": 40, "width": 300, "opacity": 100, "omitted": 0}}
    return {"pid": 456, "frameDisplay": data}


class StateTests(unittest.TestCase):
    def valid(self, state, now=100, pid=456, window="123", width=640, height=300):
        return display.frame_state_valid(state, now, pid, window, width, height)

    def test_fresh_exact_identity_and_geometry_only(self):
        state = fixture()
        self.assertTrue(self.valid(state))
        for kwargs in ({"pid": 457}, {"window": "0"}, {"window": "999"},
                       {"width": 641}, {"height": 301}, {"now": 99.9}, {"now": 101.2}):
            with self.subTest(kwargs=kwargs):
                self.assertFalse(self.valid(state, **kwargs))
        state["frameDisplay"]["enabled"] = False
        self.assertFalse(self.valid(state))

    def test_classic_bars_need_no_frame_icons_but_empty_state_stays_hidden(self):
        state = fixture()
        state["frameDisplay"]["icons"] = []
        self.assertTrue(self.valid(state))
        state["frameDisplay"]["kickPanel"] = None
        self.assertTrue(self.valid(state))
        state["frameDisplay"]["kickBars"] = []
        self.assertFalse(self.valid(state))

    def test_malformed_timestamps_dimensions_and_shapes_fail_closed(self):
        for key, value in (("at", float("nan")), ("expires", float("inf")),
                           ("expires", 200), ("icons", "bad"), ("icons", [None]),
                           ("kickPanel", [])):
            state = fixture()
            state["frameDisplay"][key] = value
            with self.subTest(key=key, value=value):
                self.assertFalse(self.valid(state))
        for dims in ((0, 300), (True, 300), (16384, 16384), (32768, 1)):
            self.assertFalse(display.valid_dimensions(*dims))
        self.assertFalse(display.finite_number(10 ** 1000))

    def test_state_file_recovers_from_partial_atomic_replacement(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "frame-display.json"
            reader = display.StateFile(path)
            now = time.time()
            first = fixture(now)
            path.write_text(json.dumps(first))
            os.utime(path, (now, now))
            self.assertEqual(reader.read(now), first)
            second = fixture(now + .1)
            replacement = path.with_suffix(".tmp")
            replacement.write_text("{")
            os.utime(replacement, (now + .1, now + .1))
            replacement.replace(path)
            with self.assertRaises(ValueError):
                reader.read(now + .1)
            path.write_text(json.dumps(second))
            os.utime(path, (now + .2, now + .2))
            self.assertEqual(reader.read(now + .2), second)
            with self.assertRaises(ValueError):
                reader.read(now + 2)

    @unittest.skipUnless(sys.platform.startswith("linux"), "Linux process lifecycle")
    def test_engine_guard_exits_with_original_process(self):
        process = subprocess.Popen([sys.executable, "-c", "import time;time.sleep(30)"])
        guard = display.EngineGuard(process.pid)
        try:
            self.assertTrue(guard.alive())
            process.terminate()
            process.wait(timeout=5)
            self.assertFalse(guard.alive())
        finally:
            guard.close()
            if process.poll() is None:
                process.kill()
                process.wait(timeout=5)


class EstimateTests(unittest.TestCase):
    def test_timer_and_unknown_never_invent_readiness(self):
        icon = fixture()["frameDisplay"]["icons"][0]
        view = display.estimate_view(icon, 100)
        self.assertEqual(view["countdown"], "1:12")
        self.assertEqual(view["badge"], "~")
        self.assertEqual(view["bar_text"], "~ 72s")
        self.assertEqual(view["fraction"], .8)
        for status in ("estimated_cooldown", "estimated_recharge", "unknown", "unexpected"):
            view = display.estimate_view(dict(icon, status=status, ends=99), 100)
            self.assertTrue(view["unknown"])
            self.assertIsNone(view["countdown"])
            self.assertEqual(view["bar_text"], "?")
        ready = display.estimate_view(dict(icon, status="estimated_ready", ends=0), 100)
        self.assertFalse(ready["unknown"])
        self.assertIsNone(ready["countdown"])
        self.assertEqual(ready["bar_text"], "~ Ready")
        self.assertEqual(ready["fraction"], 1)

    def test_delivery_and_charge_uncertainty_remain_visible(self):
        icon = fixture()["frameDisplay"]["icons"][0]
        self.assertEqual(display.estimate_view(icon, 100)["border"], display.TEAL)
        for change in ({"chargesUnknown": True}, {"deliverySeconds": 2.1}):
            self.assertEqual(display.estimate_view(dict(icon, **change), 100)["border"], display.AMBER)

    def test_heartbeat_does_not_rebuild_unchanged_icons(self):
        first = fixture()["frameDisplay"]
        second = copy.deepcopy(first)
        second["at"], second["expires"] = 100.1, 101.3
        self.assertEqual(display.visual_key(first, 100), display.visual_key(second, 100.1))
        self.assertNotEqual(display.visual_key(first, 100), display.visual_key(first, 100.3))
        first["kickBars"] = []
        self.assertEqual(display.visual_key(first, 100), display.visual_key(first, 100.9))
        self.assertNotEqual(display.visual_key(first, 100), display.visual_key(first, 101))


class DrawingTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        try:
            display.drawing_modules()
        except ImportError as exc:
            raise unittest.SkipTest("System GTK/Cairo rendering packages unavailable: " + str(exc))

    def painter(self):
        return display.Painter(ROOT / "windows/icons")

    def test_real_render_has_transparent_background_and_exact_placement(self):
        painter = self.painter()
        surface = painter.render(fixture()["frameDisplay"], 100)
        pixels, stride = bytes(surface.get_data()), surface.get_stride()
        def alpha(x, y):
            return pixels[y * stride + x * 4 + (3 if sys.byteorder == "little" else 0)]
        for x, y in ((0, 0), (19, 30), (68, 30), (20, 29), (20, 78), (229, 70), (530, 70)):
            self.assertEqual(alpha(x, y), 0, (x, y))
        for x, y in ((20, 30), (45, 60), (231, 41), (240, 80), (500, 80)):
            self.assertGreater(alpha(x, y), 0, (x, y))

    def test_actual_spell_art_and_missing_icon_cached(self):
        painter = self.painter()
        fallback = painter.icon(0)
        self.assertIsNotNone(fallback)
        for file in (ROOT / "windows/icons").glob("*.jpg"):
            spell = int(file.stem)
            icon = painter.icon(spell)
            self.assertIsNotNone(icon, spell)
            if spell:
                self.assertIsNot(icon, fallback, spell)
        self.assertIs(painter.icon(9999999), fallback)
        cached = len(painter.icons)
        painter.render(fixture()["frameDisplay"], 100)
        painter.render(fixture()["frameDisplay"], 101)
        self.assertEqual(len(painter.icons), cached)

    def test_classic_unknown_ready_countdown_and_preview_labels(self):
        painter = self.painter()
        data = fixture()["frameDisplay"]
        labels = []
        original = painter.label
        def capture(ctx, text, *args, **kwargs):
            labels.append(text)
            return original(ctx, text, *args, **kwargs)
        painter.label = capture
        data["icons"] = []
        data["kickPanel"]["preview"] = True
        prototype = data["kickBars"][0]
        data["kickBars"] = [dict(prototype, y=64 + i * 39, status=status)
                            for i, status in enumerate(("unknown", "estimated_ready", "estimated_cooldown"))]
        painter.render(data, 100)
        for expected in ("LamdaKick   PREVIEW", "?", "~ Ready", "~ 72s"):
            self.assertIn(expected, labels)

    def test_overlay_only_state_renders_and_never_covers_capture_cells(self):
        state = fixture()
        data = state["frameDisplay"]
        data["icons"], data["kickBars"], data["kickPanel"] = [], [], None
        data["overlays"] = [dict(kind="bar", text="LINUX TEST", size=48, font=16,
                                  pixelX=100, pixelY=100, ends=110, duration=10,
                                  color="#00FF00", opacity=100, persistent=True,
                                  transparent=True, visual=dict(width=200, height=40, fillOpacity=100))]
        data["captureAreas"] = [dict(x=110, y=110, width=10, height=10)]
        self.assertTrue(display.frame_state_valid(state, 100, 456, "123", 640, 300))
        surface = self.painter().render(data, 100)
        pixels, stride = bytes(surface.get_data()), surface.get_stride()
        alpha = lambda x, y: pixels[y * stride + x * 4 + (3 if sys.byteorder == "little" else 0)]
        self.assertGreater(alpha(105, 105), 0)
        self.assertEqual(alpha(115, 115), 0)
        self.assertEqual(alpha(99, 99), 0)
        self.assertFalse(display.frame_state_valid(state, 101.3, 456, "123", 640, 300))


if __name__ == "__main__":
    unittest.main()
