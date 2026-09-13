# Validation — 0.2.0 preview

## Completed locally

- Go tests with race detector: party filtering, timestamps/time zones, delayed
  casts, duplicates, separate players, timer expiry, partial lines, truncation,
  rotation, data-only settings parsing, and release-tag selection.
- End-to-end Linux engine process: appended synthetic cast becomes a timer based
  on the original cast timestamp; saved module disable clears the display state.
- Lua 5.1: all distributed Lua files compile. Engine module defaults, persistence,
  registration, and combat guard on Save & Reload pass mocked runtime checks.
- PowerShell syntax checks for launcher, installer and overlay.
- Mocked installer deployment and rollback after a failed addon copy. Uses real
  temporary filesystem operations; mocks Windows dependency tools and shortcut COM.
- Windows amd64 cross-compilation of the Go engine.
- Local replay of the desktop's 2026-09-13 combat log detected 97 real teammate
  defensive casts, including the 01:35:24.767 Mirror Image record, Blur and Anti-
  Magic Shell. No user combat logs, names, GUIDs or SavedVariables are distributed
  as test fixtures.

## Still requires a live Windows run

- Native WPF appearance, click-through, focus behavior, scaling and multiple monitors.
- Real winget dependency installation, UAC behavior, shortcut and update installation.
- Actual time between a teammate cast and its appearance on disk in a dungeon.
- In-game module controls and protection/taint behavior during combat.
- Full M+ coverage, talents, charges, and incomplete/missing cast handling.

The original LamdaUI dungeon crash is deferred, not fixed. Legacy modules are removed from the current addon; prior SavedVariables remain
preserved. No live engine status is displayed in the addon.

## 0.2.0 additions

- New standalone five-file addon: exactly LamdaCD and Settings; no legacy module
  files or third-party/class-specific dependencies are shipped.
- Lua 5.1 tests cover slash opening, settings, combat reload guard, legacy data
  preservation and logging enable conditions.
- Linux Tk/X11 overlay rendered and was visually inspected in an explicitly
  labelled preview on this laptop. Actual party defensive display remains a live
  test; follower NPC casts are deliberately excluded.
- Installed the clean addon locally and restarted the engine with Linux overlay
  enabled. A copy of the prior addon is outside WoW AddOns for rollback.
