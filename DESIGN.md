# Lamda Engine distribution and interface

Status: first Windows preview implemented; see VALIDATION.md for completed checks and live validation gaps.

## Scope

LamdaUI remains the in-game addon hub. No separate desktop settings application.
Lamda Engine runs in the background. First feature: lamdaCD teammate defensive
estimates from disk combat logs. Measure delivery delay before promising live
tracking. Vision integration and a visual rule editor come later. Defer the
existing LamdaUI dungeon error investigation; remove legacy class helpers from the public hub while preserving old SavedVariables.

## Modules and settings

Each module registers a LamdaUI tab. Start with LamdaCD, later Vision and other
modules. A shared Settings tab configures automatic engine update checks,
frequency, and notifications. Checking does not authorize automatic installation.

Addon preferences persist via SavedVariables on reload/logout. The engine reads
completed snapshots without executing Lua. The engine must not modify the same
SavedVariables file. Save/reload is not an acknowledgement of engine receipt.

## Windows distribution

- Source hosted publicly at github.com/tm-LBenson/lamda-engine (user selected public).
- A reviewable PowerShell installer obtains Git and Go when missing, fetches a
  specific release, and compiles the engine locally.
- Build and verify in staging before replacing an existing installation. Preserve
  configuration and a rollback version. Fail clearly if a required step fails.
- Install the addon into the selected WoW Retail installation, preserving existing
  settings and backing up any replaced addon files.
- Install engine files per user. Request elevation only when dependency setup
  requires it. Do not permanently weaken PowerShell execution policy.
- Create a desktop shortcut named `lambaUI` (user-requested spelling) using an LUI
  logo. The shortcut launches the background engine; repeat clicks reuse the
  running instance. No desktop dashboard is required.
- Update installation follows the same fetch/build/verify/replace process.
- Local compilation does not sign the executable or guarantee the absence of
  Windows, antivirus, or PowerShell prompts.

## Status boundary

Normal WoW addon APIs do not provide arbitrary file reads or a localhost socket
for engine heartbeats. Therefore the addon cannot currently claim live Engine
Online/Offline status. Show no status text when status is unknown. Only show a
status when supported by a tested acknowledgement mechanism. A file loaded on reload can
at most establish a last-known snapshot, never continuing liveness.

The external overlay can show engine/module health and log freshness while it is
running. If the entire engine stops, its own overlay cannot reliably announce that
failure; a separate watchdog would be required for an independently live warning.

## UI presentation

Keep LamdaUI lightweight: module tabs and necessary controls, with no explanatory
filler, speculative status, or extra information text. Unknown state stays silent.
Keep technical diagnostics out of the normal interface; expose them only through
an explicit diagnostic action when needed.

## First implementation milestone

1. Read existing logs and replay real teammate defensive casts.
2. Measure event timestamp versus first appearance on disk during a run.
3. Show estimated cooldowns and data freshness in an external overlay.
4. Connect LamdaUI configuration snapshots.
5. Package the verified feature with the Windows source-build installer and icon.

Do not present a module as functional just because its settings tab or installer
exists. Engine-generated timers do not directly populate in-game addon frames.

## Full replacement target

The user requires the eventual hub to replace the full MiniCC/MiniAuras feature
set. FEATURE-COVERAGE.md records the inspected modules and missing behaviors.
LamdaCD customization is the current deliverable; publishing it does not establish
full replacement coverage. Preserve the clean generic hub, with no legacy personal
class setup modules.
