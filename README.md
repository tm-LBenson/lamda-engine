# Lamda Engine

A background engine for LamdaUI, the in-game addon hub. The first module is
**LamdaCD**: estimated teammate defensive cooldowns from WoW's disk combat log.
No desktop settings application. The Windows shortcut starts the engine and its
click-through overlay; configure it in WoW with `/lui`.

**0.6.0 is a preview.** The hub and cooldown editor work through saved settings.
Live laptop logs have shown delivery delays from a fraction of a second to more
than 20 seconds. Timers remain estimates, not dependable live readiness. Native
Windows rendering still needs a live test; full MiniCC parity is unfinished.

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
`v0.6.0` tag, runs Go tests, compiles locally, and installs LamdaUI and the engine.
It requires 64-bit Windows, 64-bit Windows PowerShell 5.1+, and winget if dependencies are
missing. An existing Go installation must support Go 1.25 or newer. GitHub source
access is public. There are no downloaded precompiled engine binaries.

The installer creates **lambaUI** on your desktop with an LUI icon. Click it to
start the background engine; repeated clicks do not start another instance.
WoW must use windowed or borderless mode for the external overlay. It appears only
while WoW is foreground. It is not anchored to DandersFrames; set its position in
LamdaUI's Modules → LamdaCD page.

1. Start WoW and open `/lui`.
2. Configure **General** and **Modules → LamdaCD**, then **Apply & Reload**.
3. Ensure combat logging is enabled. Modules → LamdaCD → Tracking includes
   Enable combat logging in dungeons & delves, which enables it when entering a party instance; `/combatlog`
   also toggles logging and prints the resulting state.
4. Enter a follower dungeon or delve with a supported companion, or party with a
   real player and observe a defensive. For layout work, **Appearance** and
   **Placement** show samples immediately. **Tracking → Show preview in game**
   enables external samples after Apply & Reload; turn it off for live observations.

The old standalone lamdaCD addon is not required by the engine. The installer
leaves it installed; disable it if you do not want its separate in-game display.
The public addon contains General settings and the LamdaCD module. Legacy class helpers, action-
bar setup, macros, layouts, training and legacy profile modules have been removed.
Legacy settings are preserved. New module profiles use a separate `engineHub`
namespace inside the already-declared LamdaUIDB; old class settings are not applied. The prior
dungeon error has not been diagnosed or claimed fixed.

## What the timers mean

Successful casts by friendly party/raid-affiliated players and explicitly supported friendly
companions are accepted. Your own player casts, enemies, unrelated NPCs, pets,
and aura-only records are excluded. **Companions** is on by
default and can be disabled under Modules → LamdaCD.

Timers use the original event timestamp, not the time the file reader receives
it. `~` means estimated; `*` marks an ability that may have additional charges.
Baselines come from lamdaCD 0.2.1's 48-spell catalog. They are not authoritative
current talent-aware cooldowns. Talent reductions, charges, deaths, missed casts,
roster changes and resets can invalidate an estimate. Cold Snap clears the observed
Ice Block timer without claiming readiness. Expired timers disappear.

The engine starts at the current file's end, handles new logs, truncation and
partial lines, and clears observations on actual instance/difficulty, challenge
and log-session boundaries. Room changes inside the same known instance preserve
observations. Cold Snap is timestamp-ordered so an older reset cannot erase a
newer Ice Block or restore a pre-reset timer.
It does not reconstruct the active party roster from logs, so leaving party
members can remain until their observed timer expires. Reloading does not magically
recover missed casts. Delivery delay is recorded in the diagnostic log; neither
an old recording nor a synthetic arrival test measures WoW's live flush delay.

## Followers, delves and preview

Supported NPCs currently include Captain Garrick, Meredy Huntswell, Shuja Grimaxe,
Austin Huxworth and Valeera Sanguinar. Their defensive, interrupt and selected
support spell IDs were verified in local recordings. Other companions, including
Brann, need their own verified mapping; this is not universal NPC support.

NPC variants never borrow player cooldown values. An observed cast first appears
as a plain name/spell row for eight seconds after arrival. It has **no countdown**
when the reuse time is unknown. After three casts with two sufficiently similar
intervals, the engine can display an approximate reuse timer. This measures NPC
behavior, not an authoritative cooldown. A contradictory shorter interval clears
the estimate. Learning is separate for each NPC GUID and ability, bounded in
memory, and resets with the model on zone/log transitions. Casts delayed more than
30 seconds do not generate companion rows.

**Show preview in game** shows labelled samples in the external overlay.
Appearance and Placement have separate inline samples that need no engine. Apply & Reload applies changes. Preview rows do not train the model,
count as observations or survive disabling Preview. Follower runs can now exercise
NPC display and layout, but do not establish real-player event coverage.

## Customizing the cooldown area

`/lui` always opens **General**, which contains engine update preferences and
module profiles. **Modules** has a list of installed modules. LamdaCD has its own
Enabled toggle and three pages:

- **Appearance:** sample preview, Compact/Standard/Large presets, row/text sizing,
  color, opacity, names, timers, progress bars and borders.
- **Placement:** sample preview, Move & resize cooldowns, screen anchors, overall
  scale, columns, growth, spacing and maximum displayed rows.
- **Tracking:** companions, external preview, automatic logging and module reset.

**Move & resize cooldowns** opens an in-game guide. Drag its body to move it or its
bottom-right corner to resize. Apply & Reload saves for the engine; Cancel restores
the prior placement and size. Combat entry or reopening `/lui` cancels placement.
Raw coordinate fields are absent. Live party-frame attachment remains unfinished.

General's **Profiles** can save, switch, rename, delete, import and export module
settings. Update preferences are global and do not change with a profile. Imports
are validated data, never executed Lua. Applying a profile to the engine still
requires Apply & Reload. Invalid saved sizes revert to valid defaults before
building previews; resetting LamdaCD only affects that module.

The editor is a native in-game preview; the external display receives settings on
reload. It does not claim a live engine acknowledgement or show actual cooldown
state. Windows rendering now converts physical layout pixels to WPF device units
so the placement guide and overlay can agree across UI scales; native Windows DPI
behavior still needs a live check.

The replacement target includes the full MiniCC/MiniAuras feature set. See
[FEATURE-COVERAGE.md](FEATURE-COVERAGE.md) for what is implemented and what still
needs work. **Do not remove MiniAuras expecting full feature coverage yet.**

## Settings and status

LamdaUI opens on **General** for engine update preferences. **Modules → LamdaCD** contains cooldown controls. Controls are deliberately minimal. SavedVariables are read only after
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
engine/addon folders and restores them on installation failure. Configuration and
the desktop shortcut are also restored or removed if they were newly created. Existing WoW
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

The Go reader/replay core builds on Linux and Windows. Windows uses the WPF
overlay. Linux has a Tk/X11 overlay (also usable with XWayland); it requires
Python 3, tkinter and python-xlib. After building, copy `linux/overlay.py` beside
the engine binary. Pure Wayland clients without X11 window metadata are not
supported. Vision and a visual rule editor are future modules.

Source release assets are built from a clean committed tree with
`python3 tools/package.py`. It verifies that the installer pins the engine version,
then writes the source archive, installer and SHA256 checksums under `dist/`.

[REQUIREMENTS.md](REQUIREMENTS.md) records the agreed scope and acceptance criteria.
