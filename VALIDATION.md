# Validation — 0.6.0 preview

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

## 0.3.0 additions

- Explicit NPC GUID/spell allowlist supports four recorded follower NPCs and
  Valeera. Friendly group/mine affiliation required; enemies and unrelated NPCs
  are rejected. Player spell baselines are never applied to NPC variants.
- Tests cover source filtering, unknown-reuse pulses, duplicate suppression,
  learning across pulse expiry, contradictory intervals, separate identities,
  stale arrivals and preview isolation.
- Replayed follower and delve recordings to validate the added mappings.
- Preview is a labelled presentation mode controlled from the LamdaCD tab.
- Live player coverage and actual delivery latency are still unverified.

## 0.4.0 additions

- Nine window-relative anchors, signed offsets, row dimensions, spacing, columns,
  row caps, up/down growth, font/opacity, palette, border/progress and text toggles.
- Go tests verify grid geometry, anchors, scaling, upward growth, typography bounds,
  configuration validation and known versus unknown progress duration.
- The X11 visual smoke check verifies an actual centered two-column preview with
  an empty input shape. It caught and fixed moving the Tk child instead of its
  top-level wrapper. The corrected result was visually inspected.
- All distributed Lua compiles in Lua 5.1; settings/combat guard tests pass.
- Windows overlay parses; native Windows appearance and DPI behavior still need
  a live run. Windows engine cross-build and installer rollback checks pass.
- Full MiniCC/MiniAuras feature coverage is tracked separately, not claimed here.

## 0.5.0 additions

- Replaced the coordinate-first interface with Team cooldown display, inline samples,
  size presets, live sliders, Move & resize, and secondary More options.
- Native placement guide supports dragging, corner resizing, Apply & Reload and
  Cancel; it closes on combat entry and restores cancelled edits.
- Lua 5.1 checks exercise anchor round-trips at non-default UI scale, movement,
  resize math, cancellation restoration, combat guards and preservation of exact
  values when sliders are initialized. All addon files compile.
- Engine receives settings on reload; the editor is not live engine feedback.
- Native in-game visuals still need user verification. Windows device conversion
  parses but still requires a real DPI test.

## 0.6.0 additions

- Requirements reviewed against the user conversation; General and Modules are
  separate. Registered modules have independent defaults, validation and enable
  controls. LamdaCD has Appearance, Placement and Tracking pages.
- Module profile create/switch/rename/delete and data-only import/export preserve
  shared engine update preferences and all existing legacy settings.
- Same-instance room changes preserve cooldowns. Timestamp-ordered Cold Snap
  handling rejects duplicate/stale resets. Tail generations detect same-name log
  replacement and truncate/regrowth; initial partial and oversized lines are safe.
- Friendly raid teammates are accepted; self, hostile and unrelated actors remain
  excluded. Observation histories are bounded and expire. Invalid optional config
  values cannot silently turn settings back on.
- Installer rollback now includes partially written configuration and shortcuts.
  Mocked early/late failures verify restoration and fresh-file removal.
- Local live logs have already shown approximately 0.214–26.637 second cast
  delivery delays. This is an observed limitation. Reliable real-player dungeon/M+
  coverage, full MiniCC parity and actual Windows UI behavior remain unverified
  or unfinished; no claim of live readiness or full replacement is made.
