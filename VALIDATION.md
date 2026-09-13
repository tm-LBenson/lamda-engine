# Validation — 0.7.0 / addon 0.26.0

Source implementation, automated checks, and live game behavior are separate
claims. Native rendering and full replacement are not established by mocked
API calls.

## Completed for 0.7.0

- **27 Lua checks pass**, including native Blizzard API contracts, frame/provider
  integration, sorting/reassignment, combat handling, out-of-world visibility,
  failed-initialization retries, and profile/settings UI behavior.
- Go tests pass with race detection; `go vet` passes.
- Linux and Windows engine builds pass.
- The local reload bundle compiles. Addon 0.26.0 is installed locally and engine
  0.7.0 is running with its external overlay disabled.

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
- Check category caps, anchors, spacing/wrapping, fonts, swipes/reverse, stacks,
  borders, static glow, and tooltips.
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
is still needed. Full MiniCC/MiniAuras parity, dependable teammate cooldown
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
MiniCC is a settings bridge; MiniAuras supplies the runtime feature reference.
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
