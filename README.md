# Lamda Engine

A background engine for LamdaUI, the in-game addon hub. The first module is
**LamdaCD**: estimated teammate defensive cooldowns from WoW's disk combat log.
No desktop settings application. The Windows shortcut starts the engine and its
click-through overlay; configure it in WoW with `/lui`.

**0.1.0 is an initial preview.** Real-log replay and automated checks pass. Native
Windows overlay behavior and actual in-combat log delivery delay still need live
validation. This is not a confirmed replacement for restricted addon tracking.

## Install on Windows

Close WoW. Download and review `windows/install.ps1` from this repository, then run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
```

For a nonstandard installation or multiple WoW accounts:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -WowPath 'D:\World of Warcraft\_retail_' -Account 'YOUR_ACCOUNT_FOLDER'
```

The script installs missing Git and Go using Windows Package Manager, clones the
`v0.1.0` tag, runs Go tests, compiles locally, and installs LamdaUI and the engine.
It requires 64-bit Windows, Windows PowerShell 5.1+, and winget if dependencies are
missing. An existing Go installation must support Go 1.25 or newer. GitHub source
access is public. There are no downloaded precompiled engine binaries.

The installer creates **lambaUI** on your desktop with an LUI icon. Click it to
start the background engine; repeated clicks do not start another instance.
WoW must use windowed or borderless mode for the external overlay. It appears only
while WoW is foreground. It is not anchored to DandersFrames; set its position in
LamdaUI's LamdaCD tab.

1. Start WoW and open `/lui`.
2. Configure **LamdaCD** and **Settings**, then **Save & Reload**.
3. Ensure combat logging is enabled. LamdaUI includes its existing Combat Logging
   module; `/combatlog` also toggles logging and prints the resulting state.
4. Party with a real player and observe them use a defensive.

The old standalone lamdaCD addon is not required by the engine. The installer
leaves it installed; disable it if you do not want its separate in-game display.
The suspended Augmentation module stays suspended. The prior dungeon error has
not been diagnosed or claimed fixed.

## What the timers mean

Only successful casts by party-affiliated player GUIDs are accepted. Your own
casts, NPC followers, enemies, aura-only records and raid-only affiliations are
excluded. Follower dungeons can test the UI but cannot validate this module's
real-player tracking.

Timers use the original event timestamp, not the time the file reader receives
it. `~` means estimated; `*` marks an ability that may have additional charges.
Baselines come from lamdaCD 0.2.1's 48-spell catalog. They are not authoritative
current talent-aware cooldowns. Talent reductions, charges, deaths, missed casts,
roster changes and resets can invalidate an estimate. Cold Snap clears the observed
Ice Block timer without claiming readiness. Expired timers disappear.

The engine starts at the current file's end, handles new logs, truncation and
partial lines, and clears observations on recognized zone/challenge transitions.
It does not reconstruct the active party roster from logs, so leaving party
members can remain until their observed timer expires. Reloading does not magically
recover missed casts. Delivery delay is recorded in the diagnostic log; neither
an old recording nor a synthetic arrival test measures WoW's live flush delay.

## Settings and status

LamdaUI has **LamdaCD** and **Settings** module entries alongside the existing
helpers. Controls are deliberately minimal. SavedVariables are read only after
WoW persists them; the engine never executes Lua or writes back to those settings.
A partial/invalid settings snapshot retains the last valid configuration.

There is no addon-to-engine live acknowledgement channel. Unknown engine status
stays silent. External timers cannot be written directly into in-game addon frames.
The overlay hides if its engine exits, state stops refreshing, or WoW loses focus.
No input automation, memory reading or network access to the game is used.

Update checks can be daily or weekly. Notifications appear only for a confirmed
newer version. Checks never install code. To update, rerun the reviewed installer
with the desired `-Ref vX.Y.Z`. Failed checks stay silent. Updates are checked via
GitHub releases, with Git tags as a fallback.

## Files and rollback

- Engine: `%LOCALAPPDATA%\LamdaUI\current`
- Backups: `%LOCALAPPDATA%\LamdaUI\backups`
- Engine configuration/state/log: `%APPDATA%\LamdaUI`
- Addon preferences: `_retail_\WTF\Account\<account>\SavedVariables\lamdaUI.lua`

The installer builds before stopping the installed engine. It backs up replaced
engine/addon folders and restores them on installation failure. Existing WoW
SavedVariables remain untouched. Builds are unsigned; local compilation does not
guarantee that Windows, antivirus, or PowerShell will not prompt.

For manual rollback, close WoW and stop the engine, restore the desired backup's
`engine` folder to `current` and `lamdaUI` to WoW's AddOns directory. If an
`install.json` backup exists, restore it to `%APPDATA%\LamdaUI`. Launch again.

## Development

```sh
go test -race ./...
go build -o bin/lamda-engine ./cmd/lamda-engine
go run ./cmd/lamda-engine --replay /path/to/WoWCombatLog.txt
python3 tools/smoke.py bin/lamda-engine
```

`tools/smoke.py` runs on Linux with an isolated config directory. Optional addon
checks require `lupa`: `python tools/test_addon.py`. PowerShell parser and mocked
installer transaction tests can run with PowerShell 7 on Linux. They do not prove
WPF or winget works on a real Windows machine.

The Go reader/replay core builds on Linux and Windows. The graphical overlay and
installer currently target Windows. Vision and a visual rule editor are future
modules; neither is bundled as a working module in this release.
