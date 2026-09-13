# LamdaUI and engine design

Engine 0.7.0 / addon 0.26.0 makes native frame icons the primary gameplay display.
[REQUIREMENTS.md](REQUIREMENTS.md) records scope; [VALIDATION.md](VALIDATION.md)
records evidence and open checks.

## Hub and modules

LamdaUI is an in-game hub. `/lui` opens General, which owns engine update
preferences and module profiles. Modules lists installed runtimes; LamdaCD owns
Auras, Frames, Appearance, and Content. There is no desktop settings application
or placeholder module.

Modules own defaults, validation, enabled state, and controls. Profiles hold
module settings while update preferences stay global. Imports are validated
data, never executed Lua. Legacy personal helpers are absent; existing saved
data is preserved.

## Native frame rendering

Standalone lamdaCD 0.2.1 already supplied frame-attached defensive estimate
icons. The new module uses owned anchors and Blizzard's
`CustomAuraContainerTemplate` to show current auras beside DandersFrames and
Blizzard units. Successful native initialization retires only that old Lamda
display and preserves its visibility preference; it does not disable other
addons' features.

Actual public unit bindings identify player, party, raid, and supported pet
frames. Slot order never substitutes for identity. Danders public lookups and
sorting callbacks provide discovery; Blizzard uses supported frame targets.
Hidden, forbidden, secret, or conflicting bindings never become guessed matches.

Five native groups cover CC, other debuffs, big defensives, external defensives,
and important buffs. Controls select categories/caps, units/contexts, anchors,
size, spacing, wrapping, and appearance. Owned labelled samples preview real
frames or a fallback; they do not use Blizzard's global sample aura provider or
record casts.

Container creation and structural changes are combat guarded. Unit/groups are
set before enabling. An initializer creates fresh texture/cooldown/text regions
and registers them with each native button. Blizzard supplies aura identity,
texture, visibility, duration, and stacks. Lamda does not inspect secret fields
or calculate from secret container geometry. Safe binding updates do not rewrite
protected frame scripts or click handlers.

Spell-ID candidate filtering applies to helpful friendly auras and harmful
enemy auras, not friendly debuffs. Supported category/dispel filters must be
used where applicable. A silently ignored filter is not functional support.
Active aura duration never means ability cooldown readiness.

## Engine boundary

Lamda Engine remains a background Go process. Native icons need neither the
engine nor logging. `nativeFrames=true` suppresses detached engine cooldown rows
and prevents LamdaUI from enabling automatic logging. The old bar editor is
removed. Legacy config fields remain readable for migration/profiles without
redefining native rendering.

The disk reader/replay model remains separate for defensive observations and
mapped NPC reuse estimates. Measured delivery delay prevents dependable live
readiness claims. Its estimates do not directly populate addon frames. Native
aura rendering does not require NPC spell mappings.

SavedVariables persist on reload/logout. The engine reads completed snapshots
without executing Lua or writing back to that file. Invalid/partial snapshots
retain the last valid engine configuration.

## Status and presentation

Normal addon APIs provide no arbitrary file reader or localhost heartbeat
socket. Reload saves settings; it is not engine acknowledgement. Unknown engine
status stays silent, and a loaded snapshot cannot prove continuing liveness.

General and module controls remain lightweight. `/lui debug` reports known
provider discovery and native runtime errors only when requested. Update checks
notify on confirmed newer releases and never install code automatically.

## Distribution

- Public source: `github.com/tm-LBenson/lamda-engine`.
- Reviewed PowerShell installs missing Git/Go and builds a pinned source release.
- Build/test in staging; back up engine, addon, configuration and shortcut;
  restore on failure and preserve SavedVariables.
- The requested `lambaUI` desktop shortcut/LUI logo launch one engine instance.
  Native icons also work without that process.
- Updates repeat fetch/build/verify/replace. No permanent execution-policy
  weakening. Local builds are unsigned and may still prompt in Windows.

## Replacement scope

Full MiniCC/MiniAuras behavior is current required scope, tracked in
[FEATURE-COVERAGE.md](FEATURE-COVERAGE.md). Native frame auras implement part of
it; the remaining modules/controls remain acceptance work. Lamda uses original
code and public APIs, not wholesale copied All Rights Reserved MiniAuras source.

Vision and a broader visual rule editor are later modules. The earlier dungeon
error remains undiagnosed; this architecture change does not establish its
cause or claim it fixed.
