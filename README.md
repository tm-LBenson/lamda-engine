# LamdaUI and Lamda Engine

[**Download LamdaUI 1.5.0**](https://github.com/tm-LBenson/lamda-engine/releases/tag/v1.5.0) · [Latest release](https://github.com/tm-LBenson/lamda-engine/releases/latest)

LamdaUI combines an in-game addon hub with a Windows companion. Retail and WoW
Forever have separate addon builds, settings, companion data and backups.

## Install or update

1. Download **Install-LamdaUI.cmd** from the release page and run it on 64-bit Windows.
2. Select **World of Warcraft** for Retail or **WoW Forever**, then choose the game folder if it was not found.
3. Complete setup, launch LamdaUI, and open `/lamda` in game. The companion's **Browse Addons** page installs the optional bundled modules.

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

## Included features

- **Frames and LamdaCD:** unit frames, click casting and native aura displays.
- **LamdaKick and LamdaGladius:** interrupt and arena observations, with explicit estimates when native timing is unavailable.
- **Overlays:** editable display rules, previews and per-character backgrounds. Aura checks can restrict harmful effects to the player's own casts.
- **Planner (LamdaKBB), Drawer, Vendor, Settings and Performance:** optional UI and companion modules.
- **LamdaSim:** local calculation jobs, including a pinned offline SimulationCraft worker for Retail DPS. Its GPL license and corresponding source are included under `providers/simc`.
- **LamdaQueue:** an optional module with its own explicit enable switch; installing the companion alone does not enable queue acceptance.

Open `/lamda` for the module controls. Optional features are installed and enabled
individually. Native aura display works without the companion. Features that use
capture or companion calculations require the corresponding module and reader.

Cooldown estimates can miss unseen casts, talents and resets. Capture formats
and some combat behavior still need live validation; automated tests establish
software contracts, not perfect combat tracking. Calculation models have not
been independently reviewed. Healer scores are not verified HPS. A private
healer worker is not redistributed in this release.

## Source

- `cmd/`, `internal/`, `assets/`, `windows/`: shared Go companion and platform helpers.
- `lamdaui/`: addon hub and module sources; this is an integration source tree rather than an installable assembled addon.
- `desktop/`: native Windows tray application.
- `setup/`: current Windows installer and updater.
- `character-export/`: independent LamdaExport addon source. It is separate from the LamdaUI installer.
- `tools/sim_results_runtime.py`: the independent data validator used by the cross-language overlay contract test.

The older `addon/`, other `tools/` scripts and historical design documents remain
for reference. Their version-specific installation instructions predate the
current release; use the release assets above for installation.

With Go 1.25 or later and Python 3 available:

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
