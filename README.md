# LamdaUI and Lamda Engine

**LamdaUI is an in-game addon hub. LamdaCD puts aura icons on your unit frames.**
Open `/lui` for General settings, then **Modules → LamdaCD** for its controls.
There is no separate desktop settings application.

Engine **0.7.3** / addon **0.27.2** retains the independent frame regions and
party/raid layouts introduced in 0.7.1 / 0.27.0: crowd control, debuffs, active
defensive buffs, and important buffs beside
DandersFrames or Blizzard frames. Blizzard's native aura containers supply the
icons and their remaining durations. **This display needs neither the engine nor
combat logging.** Preview attachment to DandersFrames is confirmed by the user
and screenshot. Real teammate effects and combat behavior still need live tests. Full original MiniCC replacement remains required and unfinished;
see [FEATURE-COVERAGE.md](FEATURE-COVERAGE.md).

## Using LamdaCD

After updating, `/reload`, then open `/lui`. For a first installation, restart WoW.

1. **General** contains engine update preferences and module profiles.
2. **Modules → LamdaCD → Auras** selects crowd control, debuffs, defensive buffs,
   and important buffs, with separate icon limits.
3. **Frames** controls the selected region's layout. Choose a party or raid layout and the CC,
   Debuffs, Defensives, or Important buffs region, then set its anchor, distance,
   adjustments, and growth direction. Each region has its own position.
4. **Appearance** edits the chosen region in the party or raid layout: icon size,
   spacing, icons per line, text size, timers, stacks, borders, static highlights,
   swipes/reverse, and tooltips. Category switches and limits on Auras are shared.
5. **Content** selects DandersFrames or Blizzard, which player/party/raid/pet
   frames to use, and world, dungeon/follower, raid, arena, battleground, or delve visibility.

**Preview on frames** shows all enabled regions simultaneously at their configured
icon limits on every module page, with a sample frame when none are available.
This represents the maximum layout. The user prefers this full preview;
crowding alone is expected and is not a rendering bug. Selecting a region changes
which settings you edit, not which samples appear. Use **Stop preview** to return
to real auras and **Apply & Reload** to persist settings. Samples do not represent
observed spells or train cooldown estimates.

Icons follow each frame's displayed unit, including frame sorting. A timer on an
active defensive buff is its **remaining buff duration**, not the time until
that player can cast the ability again. Nothing here claims confirmed teammate
cooldown readiness. `/lui debug` reports known frame-provider discovery and native
runtime errors when troubleshooting is needed; normal gameplay stays quiet.

The old detached cooldown-bar editor has been removed. Native frame mode
suppresses those engine bars after saved settings are read, and does not turn
on automatic combat logging. The original standalone lamdaCD already provided
attached defensive estimate icons. On successful native frame creation, LamdaUI
hides that original Lamda display, disables its addon for subsequent reloads, and
preserves its prior visibility preference.
It does not disable MiniAuras or DandersFrames features. MiniCC's compatibility
bridge draws no icons itself; MiniAuras provides its active features. Keep the
MiniAuras features you still use until their replacements are verified.

## Install on Windows

Close WoW. Download and review `windows/install.ps1` from this public repository,
then run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
```

For a nonstandard installation or multiple WoW accounts:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -WowPath 'D:\World of Warcraft\_retail_' -Account 'YOUR_ACCOUNT_FOLDER'
```

The script installs missing Git and Go with Windows Package Manager, obtains the
`v0.7.3` source, runs Go tests, compiles locally, and installs LamdaUI and the
engine. It requires 64-bit Windows and Windows PowerShell 5.1+, with winget when
dependencies are missing. Existing Go installations need Go 1.25 or newer.
No precompiled engine binary is downloaded.

The **lambaUI** desktop shortcut, with its LUI logo, starts one background engine
instance. Native LamdaCD icons work without launching it. For an addon-only
installation, copy `addon/lamdaUI` into WoW Retail's `Interface/AddOns` folder.

The public addon contains the General/modules hub and LamdaCD. Legacy personal
class helpers, action-bar setup, macros, and training are absent. Existing
SavedVariables are preserved. The earlier dungeon addon error has not been
diagnosed or claimed fixed.

## Profiles, updates, and status

General's profiles save, switch, rename, delete, import, and export module
settings. Engine update preferences remain global. Imports are validated data,
never executed Lua. Existing unrelated settings in `LamdaUIDB` are preserved.

Update checks can be daily or weekly. Notifications appear only for a confirmed
newer version; checking never installs code. Rerun the reviewed installer with a
desired `-Ref vX.Y.Z` to update. Failed checks stay silent.

There is no tested live addon-to-engine acknowledgement channel. **Unknown
engine status stays silent.** Reload saves settings; it is not proof that an
engine is running or received them. Native aura display is independent of that
status. The engine reads saved settings without executing Lua or writing back
to the SavedVariables file.

## Cooldown tracking remains unfinished

Original MiniCC had a friendly cooldown tracker as well as active-aura icons.
The 12.1 successor removed that tracker. Lamda still needs useful teammate
cooldown observations; displaying buff duration does not complete that work.
[Original tracker settings](https://github.com/Verubato/mini-auras/blob/4.6.3/src/Config/FriendlyCooldownTracker.lua),
[12.1 changes](https://github.com/Verubato/mini-auras/blob/5.0.0/changelog.md).

## Existing log reader

The Go engine retains its disk-log reader and replay tools. They recognize a
48-spell defensive reference catalog plus explicitly mapped follower/delve
companions. Reference cooldowns, additional charges, talents, resets, and missed
casts remain uncertain. NPC reuse estimates describe observed behavior, not
authoritative cooldowns. Five companions have mappings; this is not universal
NPC log support.

Live recordings showed disk delivery delays of roughly 0.2–26.6 seconds. The log
reader therefore does not establish dependable live teammate readiness. Its
estimates are not fed into native aura icons. Native aura display does not use
this catalog or require an NPC spell mapping.

## Files and rollback

- Engine: `%LOCALAPPDATA%\LamdaUI\current`
- Backups: `%LOCALAPPDATA%\LamdaUI\backups`
- Engine configuration/state/log: `%APPDATA%\LamdaUI`
- Addon preferences: `_retail_\WTF\Account\<account>\SavedVariables\lamdaUI.lua`

The installer builds before replacing the existing engine. It backs up replaced
engine/addon files, configuration, and shortcuts, and restores them on failure.
Existing WoW SavedVariables remain untouched. Local compilation does not sign
the executable or guarantee the absence of Windows or PowerShell prompts.

For manual rollback, close WoW and stop the engine, restore the backup's `engine`
folder to `current` and its `lamdaUI` folder to WoW's AddOns directory. Restore
`install.json` to `%APPDATA%\LamdaUI` if the backup includes it.

## Development

```sh
go test -race ./...
go build -o bin/lamda-engine ./cmd/lamda-engine
go run ./cmd/lamda-engine --replay /path/to/WoWCombatLog.txt
python tools/test_addon.py
python tools/test_frame_settings.py
python tools/test_frame_runtime.py
python tools/test_native_auras.py
```

Lua checks require `lupa`. Mocked checks do not prove native WoW rendering,
combat safety, or actual Windows installation behavior. See
[VALIDATION.md](VALIDATION.md) for evidence and outstanding live checks.

The optional legacy overlays remain in the engine source: WPF on Windows and
Tk/X11 on Linux. Linux overlay development requires Python 3, tkinter, and
python-xlib; copy `linux/overlay.py` beside the binary. They are not the LamdaCD
frame renderer.

Build source release assets from a clean committed tree with
`python3 tools/package.py`. Requirements are in [REQUIREMENTS.md](REQUIREMENTS.md),
and architecture is in [DESIGN.md](DESIGN.md). Vision integration and a broader
WeakAuras-like rule editor are later modules. Full original MiniCC replacement
plus the explicitly requested team debuffs are current scope. The generic
Personal Auras editor added by the successor is not silently added to that scope.
