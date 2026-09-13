# Validation — 0.7.1 / addon 0.27.0

Source implementation, automated checks, and live game behavior are separate
claims. Native rendering and full replacement are not established by mocked
API calls.

## Completed for 0.7.1

- **36 Lua checks pass**: four independent native regions, separate party/raid
  layouts, all eight growth directions, settings isolation and profile sharing,
  combat/unit/context changes, bounded retries, samples and native API contracts.
- Go tests with race detection and `go vet` pass. Linux and Windows builds pass.
- The installed hot-reload bundle compiles under Lua 5.1. All seven addon files
  were checked against their expected contents after installation, with the
  local bundle used for `UI.lua` to support an already running WoW session.
- Addon 0.27.0 is installed locally. Engine 0.7.1 is running with its detached
  overlay disabled. The prior addon directory was backed up before replacement.

These are source, mock and installation checks. Live native aura rendering,
combat behavior and actual Windows installation remain unverified.

## Prior completed checks for 0.7.0

- **27 Lua checks pass**, including native Blizzard API contracts, frame/provider
  integration, sorting/reassignment, combat handling, out-of-world visibility,
  failed-initialization retries, and profile/settings UI behavior.
- Go tests pass with race detection; `go vet` passes.
- Linux and Windows engine builds pass.
- The 0.26.0 reload bundle compiled and was installed locally, with engine 0.7.0
  started without an external overlay. The user had not reloaded into that build
  at the last visual check; the older standalone icon display was still present.

Relevant commands (Lua checks require `lupa`):

```sh
go test -race ./...
go vet ./...
python tools/test_addon.py
python tools/test_frame_settings.py
python tools/test_frame_runtime.py
python tools/test_native_auras.py
```

These checks cover CC/debuff/big-defensive/external-defensive/important native
containers. They validate source contracts and behavior under mocks, not actual
WoW rendering or ability cooldown readiness.

## Required live checks

- Reload and confirm `/lui` opens General; all aura controls live under Modules
  → LamdaCD, with no detached bar editor.
- With engine stopped and combat logging off, observe real teammate CC,
  debuffs, and defensive buffs on DandersFrames and Blizzard frames.
- Verify frame movement/sorting, unit reassignment, party/raid transitions, and
  supported pets preserve attribution.
- Check all four regions independently, including separate party/raid size,
  placement, growth, wrapping and appearance; selecting one region must not
  overwrite another. Shared category caps stay consistent. Verify fonts,
  swipes/reverse, stacks, borders, static highlights and tooltips.
- Preview on actual frames, verify fallback samples, stop preview, and confirm
  real native auras return. Samples remain labelled.
- Exercise world/follower/delve/raid/arena/BG rules where available; one context
  does not establish all others.
- Enter combat while editing/previewing, check safe deferral and recovery, and
  look for protected-action/taint errors.
- Apply/reload to verify persistence, profile changes, module disable, and old
  engine-row suppression after reading `nativeFrames`.
- Confirm original Lamda display retirement preserves its prior preference and
  leaves MiniAuras/Danders features alone. Duplicates are not proof that the new
  module rendered an icon.

**These live native-rendering checks are pending.** User reload/preview feedback
is still needed. Full original MiniCC parity, dependable teammate cooldown
readiness, and the earlier dungeon error remain open.

## Earlier regression evidence

Previous releases completed local checks for these components. They do not
prove the new native display:

- Go filtering, timestamps/time zones, delayed casts, duplicates, independent
  actors, expiry, bounded histories, resets, partial lines, truncation/rotation,
  data-only settings parsing, and update selection.
- Engine process smoke tests with appended synthetic casts and module disable.
- Lua 5.1 compilation, General/modules, defaults/validation, profiles/imports,
  legacy data preservation, and save/reload combat guards.
- PowerShell syntax, Windows cross-builds, and mocked installer transactions
  using real temporary files with injected early/late failures. Rollback covers
  addon/engine files, configuration, and shortcuts.
- Linux legacy overlay sample rendering, placement, and empty input shape. This
  is not evidence for native WoW frames or Windows WPF rendering.
- Player log replay and five recorded follower/delve NPC mappings, with source
  filtering, unknown reuse, interval learning, and stale-arrival checks. No
  personal logs, names, GUIDs, or SavedVariables are distributed as fixtures.

## Observed log limitation

Live recordings showed approximately **0.214–26.637 seconds** between casts and
disk arrival. Replay and synthetic append tests do not remove that delay. The
reader does not establish dependable live readiness; talented intervals,
charges, resets, and complete player coverage also remain unverified.

Native aura icons use Blizzard's display path, independent of that reader. This
removes the disk dependency from active aura display without making cooldown
estimates authoritative.

## Windows deployment checks pending

- Real winget, elevation, shortcut launch, source updates, and rollback.
- Native WoW addon behavior on the Windows desktop.
- If deliberately using the retained legacy overlay: WPF rendering,
  click-through, focus, DPI, and multiple-monitor behavior.

The original dungeon error was never diagnosed. Removing legacy personal
modules while preserving SavedVariables does not prove its cause or a fix.
Unknown engine status remains silent.

## API review

Original lamdaCD 0.2.1 supplies the existing frame-attachment design. Installed
MiniCC is now a settings bridge; versioned MiniCC 4.6.3 supplies the original
feature baseline. MiniAuras supplies a current native API reference, not an
automatic expansion to all successor features.
Danders public lookup/sorting contracts were inspected without modifying its
unit frames. The new module is original code using supported APIs.

Primary Blizzard UI source confirms container groups, initializer-owned display
bindings, and identity-filter restrictions:

- [CustomAuraContainer](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_AuraContainer/Blizzard_CustomAuraContainer.lua)
- [CustomAuraButton](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_AuraContainer/Blizzard_CustomAuraButton.lua)
- [Unit aura API](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/UnitAuraDocumentation.lua)

Friendly harmful auras cannot be filtered by spell-ID candidate maps. Settings
and coverage claims must retain this boundary. Source review establishes API
intent; live tests establish compatibility.

The scope audit compared versioned original CC/active-indicator/cooldown settings
and the 5.0/5.23 successor changelog. Original cooldown tracking remains required;
generic Personal Auras editing is a later ambition rather than an inferred
current requirement.

- [Original CC anchors](https://github.com/Verubato/mini-auras/blob/4.6.3/src/Modules/CrowdControlModule.lua)
- [Original cooldown settings](https://github.com/Verubato/mini-auras/blob/4.6.3/src/Config/FriendlyCooldownTracker.lua)
- [Successor feature history](https://github.com/Verubato/mini-auras/blob/5.40.0/changelog.md)
