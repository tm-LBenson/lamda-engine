# Changelog

## 0.19.3 — 2026-09-11, emergency recovery

- Suspended Augmentation after the user reported the UI crashing with errors.
  Removed its combat driver, polling loop, icon creation, API queries, and native
  Cooldown Manager layout edits from the loaded module.
- Retained its hub entry and saved preferences. Its checkbox and old shortcuts
  cannot reactivate the failed tracker. Other modules are unchanged.
- Retained the failed build and prior tests in local rollback backups. Recovery
  tests assert that the suspended module cannot call combat or tracker APIs.
- Reload or restart is required to unload code already running in WoW.

## 0.19.2 — 2026-09-11, combat tracking corrections

- Fixed the confirmed Prescience error: addons cannot pass a secret charge count
  to `Curve:Evaluate`. Opacity now uses the action's native cooldown duration
  with the global cooldown excluded, and passes its active flag to the renderer.
  The count and recharge sweep still follow the action bar.
- Fixed the confirmed Ebon source problem: its TrackedBar viewer was hidden and
  the visible TrackedBuff viewer did not contain Ebon. Outside combat, place only
  the Ebon self-buff in TrackedBuff and save the previous layout for rollback.
- Keep the detailed diagnostics; clear old-version error records on upgrade.
- Tests now reject secret inputs to Curve:Evaluate and reproduce the hidden
  Ebon viewer, including deferred setup and unchanged unrelated tracker entries.

## 0.19.1 — 2026-09-11, runtime diagnosis

- Retain the actual Augmentation API errors and failing stage in module settings
  and `/aughelper status`; report why Ebon's source could not be read.
- Show combat/preview/disabled status on the options page.
- Contain frame-initialization failures and hide partial frames.
- This diagnostic update does not claim to resolve the reported live API failure.

## 0.19.0 — 2026-09-11

- Added Custom Addons as the dashboard landing tab, with a module registry and
  enable/configuration controls for four existing custom features.
- Moved Augmentation and Devastation helpers into lamdaUI; preserved legacy
  slash commands and macro rollback data through a data-only compatibility addon.
- Replaced labeled Augmentation boxes with combat-only icons. Prescience uses
  action-slot charge/display APIs and an engine-rendered opacity curve.
- Added Ebon buff duration, optional missing-buff warning and observed added
  time, size controls, preview, and drag positioning.
- Added regression coverage for charge transitions, secret rendering, buff
  extensions, module controls, and disabled legacy addon migration.

## 0.18.4

- Added Strafing Run to the confirmed Devastation MiniAuras row so the second
  Deep Breath window has a visible countdown.
- Re-centred Dragonrage, Strafing Run, Imminent Destruction, and Essence Burst
  as a four-icon combat row.

## 0.18.3

- Added the confirmed Devastation MiniAuras tracker for Imminent Destruction.
  Dragonrage, Imminent Destruction, and Essence Burst now remain centred as a
  three-icon combat row.

## 0.18.2

- Added a book-shaped minimap status indicator that is visible only while
  Blizzard combat logging is active. Its tooltip reports whether advanced
  combat data is being recorded. Status checks use Blizzard's read-only API so
  the indicator does not consume the rate limit used to start or stop logging.

## 0.18.1

- Automatically enables advanced Blizzard combat logging when entering any
  dungeon, raid, scenario, delve, battleground, or arena. Logging is left on
  after exiting so outdoor encounters and later training-dummy tests remain
  available for review.

## 0.18.0 — release candidate

- Corrected Retail spellbook identity handling: the stable action/base spell ID
  is now kept separately from the currently overridden spell ID. Partial or
  malformed spellbook records are excluded from placement proposals.
- Added locale-independent crowd-control detection plus Blizzard's current
  group-buff, specialization-essential, and specialization-tracked cooldown
  categories. Talent-loadout and cooldown-override events now refresh the
  recommendation inputs.
- Learned placed interrupts from Blizzard's locale-independent action-bar
  semantic flag instead of requiring English tooltip wording.
- Added a complete Undo preflight. A held cursor, missing prior macro or item,
  unavailable restore API, or other unrecoverable before-state now stops Undo
  before the first managed value is changed.
- Extended the same all-or-nothing preflight to resumed Apply and interrupted
  startup recovery. Preview now requires the complete action placement, cursor,
  clearing, binding, and rollback API set before it can be staged.
- Expanded `/lui doctor` with exact action, binding, Click Casting, Edit Mode,
  Cooldown Manager, and cast-setting capability groups plus the binding scope
  and latest transaction result. User-created profile names are excluded.
- Added portable mouseover casting, automatic self cast, and mouseover,
  self-cast, and focus-cast modifier preferences. Each appears in Preview and
  uses the same persisted Apply, exact Undo, recovery, integrity, and
  manual-change protection as other UI settings.
- Added `LAMDAUI6` for those cast-targeting preferences while retaining imports
  from `LAMDAUI1` through `LAMDAUI5`.
- Made Click Casting profile handling fail closed when Blizzard returns an
  unknown, duplicate, malformed, sparse, or extended record, preventing a
  future profile type from being silently dropped during an unrelated change.
- Corrected rollback ordering so restored modified-click bindings are written
  to the active binding set only after all before-state values are back.
- Pinned every preview, resumed Apply, and Undo to its original account-wide or
  character-specific binding set, and resolved unbound imported main-bar keys
  through the currently visible stance/form page instead of caster page 1.
- Made Preview reject an unavailable binding scope, missing binding mutation or
  persistence APIs, and action-slot resolutions outside Blizzard's supported
  action bars before a transaction can be staged.
- Made Preview reject unreadable action-bar before-state and same-name Edit Mode
  layouts whose serialized contents or scope differ from the captured profile.
- Preserved base spell IDs for action-bar overrides so exact Undo can restore a
  talent- or form-overridden action reliably.
- Included Click Casting macro name and body in preview/recovery identity,
  blocking Apply if the macro changes and verifying the exact macro on Undo.
- Kept imported preference provenance out of suggestion explanations and
  normalized old person/device setup labels to generic imported metadata.
- Reduced routine learning to one explicit click; Learn Current now refreshes
  its own snapshot without an additional confirmation dialog.
- Replaced red warning and blocked-change text with high-contrast pale gold
  against the dark panels and in tooltips.
- Replaced the unsupported before/after arrow glyph with portable text and
  reset each newly built preview to its first change without disrupting later
  manual scrolling.

## 0.17.0 — release candidate

- Rebuilt the dashboard as a compact, generic Overview → Profiles → Preview
  workflow with no character-, class-, hardware-, or key-specific defaults.
- Added specialization, form/page, talent-loadout, and input-profile separation
  to confirmed learning and complete fresh-alt proposals.
- Added persisted two-phase Apply, integrity-checked recovery, exact Undo,
  interrupted-apply rollback, and manual-change protection for actions,
  bindings, Edit Mode, visible bars, and Cooldown Manager settings.
- Added named profiles, profile-change Undo, compact SavedVariables retention,
  and cleanup of deleted profile evidence.
- Added anonymous-origin `LAMDAUI5` sharing so repeated and updated imports do
  not recount cumulative shared learning; `LAMDAUI1` through `LAMDAUI4` remain
  importable.
- Added locale-stable form identities, current Retail API fallbacks, secondary
  action-binding deduplication, trailing-edge scan coalescing, and a rectangular
  optimal-assignment implementation for larger profiles.
- Added strict headless regression, payload-content, archive-integrity, and
  reproducible-package release gates.
- Added `/lui doctor` for a privacy-safe in-game readiness report without
  expanding the compact dashboard.
- Added a confirmation-only migration bridge for the earlier prototype. The
  new addon fails closed while both versions are loaded, stages an independent
  database copy only after acceptance, disables the older addon, reloads, and
  preserves the older SavedVariables file as rollback.
- Added transactional installation for a portable Edit Mode layout that is not
  yet present locally. Preview validates its name, data, APIs, and capacity;
  Apply installs and activates it after reload; Undo or interrupted-apply
  recovery removes it only if its imported contents are still exact.
- Added first-class Blizzard Click Casting discovery and specialization-aware
  learning. Portable profiles retain valid spell-click inputs, while Apply and
  Undo replace only the reviewed mouse/modifier pair and preserve unrelated
  target, menu, macro, interaction, and pet-action entries.
