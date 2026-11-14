# Linux / Nobara

Use the current Go engine with WoW through Wine/Steam Proton on X11 or XWayland.
The Linux backend reads the visible, foreground game's public color strips and
uses the same packet decoders as Windows. GTK renders frame icons, interrupt bars
and custom text, icon, image, bar, shape and arc displays. Capture holes remain
transparent and stale, resized or wrong-client state hides the display.

The compact Proton raster correction is enabled only when every calibration
color agrees in both rows of the retained column. CRC, duplicate native values,
marked durations and all field ranges are still validated by the shared reader.

Go 1.25+, Python 3, python-xlib, PyGObject, Pycairo and GTK 3 are required.
On Nobara these are the `golang`, `python3-xlib`, `python3-gobject`,
`python3-cairo` and `gtk3` packages. Use the system Python containing those
libraries. Build and launch from a graphical desktop terminal:

```sh
/usr/bin/python3 linux/launch.py --check
/usr/bin/python3 linux/launch.py --client forever --wow '/path/to/World of Warcraft/_forever_'
# Retail is independent:
/usr/bin/python3 linux/launch.py --client retail --wow '/path/to/World of Warcraft/_retail_'
```

The launcher builds into `bin/linux-<client>`, copies the helpers/artwork, and
keeps data in `~/.config/LamdaUI/Forever` or `~/.config/LamdaUI/Retail` (respecting
XDG_CONFIG_HOME). `--config` chooses an account's SavedVariables explicitly;
otherwise exactly one account file is selected. `--state-dir` overrides runtime
data. Ctrl+C or SIGTERM stops the engine and its helpers. Building does not
install an addon, edit game settings, change desktop shortcuts or stop WoW.

With no game path, the launcher starts only the browser workspace. Retail uses
http://127.0.0.1:47641 and Forever uses http://127.0.0.1:47642. Planner is `/kbb`
and the overlay editor is `/overlays`. A Linux source build does not bundle a
SimulationCraft worker or module catalog; those require separately prepared,
platform-appropriate packages.

Enable the modules you want in `/lamda`, Apply & Reload, and keep their color
strips visible. The engine never enables them on its own. SavedVariables,
bindings, macros and each client's histories stay separate.

The old Linux implementation survived only in a local checkout while later
public source replaced its launch path with Windows-only capture/display stubs.
This restores the Linux launch path and adds the current independent capture
channels, including wide transfers and meter-border decoding. It does not launch
or authenticate to Steam or Battle.net.

Current limits: native Wayland game windows require a separate portal backend;
use XWayland for this backend. Captures currently require a lossless 1:1 RGB
render surface and foreground WoW. Custom animation/sprite/font effects,
interactive detached meters, background capture, queue acceptance, automatic
reload input, desktop tray and automatic package installation are not ported.
Use the in-game native meters and manual `/reload`. Unknown/estimated cooldowns
keep their evidence labels; automated checks do not establish combat accuracy.

Run `python3 -m unittest discover -s linux -p 'test_*.py'` and `go test ./...` /
`go vet ./...`. Cairo rendering checks require the desktop libraries and can run
without opening a window. Real Proton capture and display acceptance additionally
requires a character session.
