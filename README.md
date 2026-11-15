# LamdaUI and Lamda Engine

[**Download LamdaUI 1.6.0**](https://github.com/tm-LBenson/lamda-engine/releases/tag/v1.6.0) · [Latest release](https://github.com/tm-LBenson/lamda-engine/releases/latest)

LamdaUI combines an in-game addon hub with a Windows companion. Retail and WoW
Forever have separate addon builds, settings, companion data and backups.

## Install or update

1. Download **Install-LamdaUI.cmd** from the release page and run it on 64-bit Windows.
2. Select **World of Warcraft** for Retail or **WoW Forever**, then choose the game folder if it was not found.
3. Complete setup, launch LamdaUI, and open `/lamda` in game. The companion's **Browse Addons** page installs the optional bundled modules.

Setup installs Git and Go through Windows Package Manager if they are missing,
then builds the LamdaUI engine and tray app on your PC. Updates use the same
source build. The source is packaged with the release and verified before
building. If a build fails, setup keeps the existing installation. Build logs
are under `%LOCALAPPDATA%\\LamdaUI-Setup\\Logs`. Windows App Installer (winget)
and .NET Framework 4.8 are prerequisites. Normal Windows permission prompts may
appear when installing build tools. The optional Retail SimulationCraft worker
remains the separately licensed upstream binary.

Forever setup recognizes the production `_forever_\Wow.exe` folder and the older
`_classic_beta_\WowB.exe` folder. Retail uses `_retail_\Wow.exe`. Setup checks the
selected executable path so the two clients remain separate even when both use
`Wow.exe`.

The release also contains **lamdaui-update.zip**, the complete package consumed
by the installer, and **SHA256SUMS.txt**. **spell-index-source.zip** contains the
spell index inputs and a portable rebuild script. This ZIP contains both clients, the
Windows companion, updater and optional module catalogs. It is not a single
folder to extract over AddOns. The installer validates file hashes, backs up
replaced files and preserves saved settings. A new addon may require restarting
the game yourself; ordinary compatible updates use `/reload` afterward.

`windows/install.ps1` is a compatibility entry point for the setup source in a
full checkout. The release installer is the supported download for normal use.

## Linux / Nobara

The current source restores native Linux capture and overlays for Steam Proton
through X11/XWayland. See [Linux setup and current limits](linux/README.md).

```sh
git clone https://github.com/tm-LBenson/lamda-engine.git
cd lamda-engine
/usr/bin/python3 linux/launch.py --check
/usr/bin/python3 linux/launch.py --client forever --wow "/path/to/World of Warcraft/_forever_"
```

The Linux launcher is independent of the Windows installer.

## Included modules

Browse Addons offers **LamdaMarket**, **Lamda Quest Tracker**, **LamdaFrames**,
**LamdaOverlays**, **LamdaSim**, **LamdaVendor**, **LamdaDrawer** and **Performance**.
LamdaMarket no longer carries the Experimental label. LamdaOverlays remains
experimental and installable.

LamdaKBB, LamdaMeters, LamdaQueue, LamdaKick, LamdaCD and LamdaGladius remain
visible as Experimental, with Install disabled. Existing installations retain
their settings and can still update or uninstall.

Quest Tracker is the standalone LamdaQuestLog addon, managed through LamdaUI.
It keeps its own per-character settings and bindings. Existing standalone copies
are recognized; choose Update to add its LamdaUI settings link. A first install
of a new addon needs one WoW restart. Installed trackers open from
**/lamda → Modules → Lamda Quest Tracker → Open tracker settings**.

LamdaSim includes a pinned offline SimulationCraft worker for Retail DPS. Its GPL
license and corresponding source are bundled under `providers/simc`. Calculation
models remain unreviewed; healer scores are not verified HPS. A private healer
worker is not redistributed.

## Source

- `cmd/`, `internal/`, `assets/`, `windows/`: shared Go companion and platform helpers.
- `lamdaui/`: addon hub and module sources; this is an integration source tree rather than an installable assembled addon.
- `desktop/`: native Windows tray application.
- `setup/`: current Windows installer and updater.
- `quest-tracker/`: standalone quest tracker source, packaged without modifications.
- `character-export/`: independent LamdaExport addon source. It is separate from the LamdaUI installer.
- `tools/sim_results_runtime.py`: the independent data validator used by the cross-language overlay contract test.

The older `addon/`, other `tools/` scripts and historical design documents remain
for reference. Their version-specific installation instructions predate the
current release; use the release assets above for installation.

With Go 1.25 or later and Python 3 with `lupa` available:

```sh
go test ./...
go vet ./...
go build -trimpath -o bin/lamda-engine ./cmd/lamda-engine
```

Build Windows engine code from another platform with `GOOS=windows GOARCH=amd64`.
The release additionally passes the integration workspace's addon, updater,
installer and desktop checks before packaging. Native UI behavior still requires
normal play in the relevant WoW client.

Player histories, saved settings, screenshots and personal overlay presets are
not release defaults. Existing library licenses and attribution remain with the
relevant sources; third-party components retain their own terms.
