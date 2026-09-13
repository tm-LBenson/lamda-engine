# Engine integration preview — 0.20.0-alpha.1

Adds LamdaCD and Settings entries for Lamda Engine. Engine preferences persist in
LamdaEngineDB on Save & Reload. See the repository README for current behavior.
The following documentation describes the inherited 0.19.3 addon. Its suspended
Augmentation module remains suspended; the reported dungeon error is not fixed.

# lamdaUI

**Recovery status — 0.19.3:** Augmentation icons are suspended after live UI
errors. The combat tracker and native layout setup do not run, and its checkbox
cannot restart them. Saved size, position, and display preferences are retained.
The other hub modules remain available. Reload or restart after installing.
The Augmentation behavior documented below describes the intended feature,
which is currently unavailable pending investigation.

lamdaUI is a hub for custom World of Warcraft helpers and action-bar setup.
Open **Custom Addons** with `/lui hub` to enable and configure each module.
The existing Overview, Profiles, and Preview tabs still learn from confirmed
setups and build action-placement proposals for new characters and specs.

It also enables advanced Blizzard combat logging whenever the player enters an
instance and leaves logging enabled afterward, preserving dungeon, raid, delve,
PvP, outdoor, and subsequent training-dummy activity for later review. A book
beside the minimap is visible whenever combat logging is active.

## Installation

Extract the release archive into the Retail WoW `Interface/AddOns` directory so
the installed path ends in `lamdaUI/lamdaUI.toc`. Enable **lamdaUI** at the
character-select AddOns screen, log in, and use `/lui` if the dashboard is not
already open. Back up `LamdaUIDB` and account bindings before testing Apply on a
release candidate.

Version 0.19 targets the Retail client interface `120100`. Classic-family
clients are not currently supported.

### Upgrading from the earlier prototype

Install lamdaUI without deleting the earlier prototype or its SavedVariables.
On the first login, lamdaUI detects the conflict and stays inactive. Its
one-time prompt can then copy the earlier local database into a separate
migration area, disable the older addon, and reload the interface. No copy,
addon-state change, or reload occurs unless the player accepts. The following
load runs the normal schema migration and removes only the temporary copy; the
older addon's SavedVariables file remains available as a rollback backup.

Do not leave both versions enabled or delete the older SavedVariables file
before completing and checking the migration. If the prompt reports a failure,
the older addon remains the source of truth and lamdaUI does not initialize its
dashboard.

## Custom Addons

- **Augmentation icons:** Prescience shows the normal action bar's count over its
  icon in combat, with the native recharge sweep. Its whole icon becomes
  transparent at zero charges. Ebon Might shows your active self-buff timer.
  Both hide outside combat, while dead, or on a taxi. Keep the actual Prescience
  spell on an action bar so it can supply the count.
- **Devastation macros:** the existing Deep Breath cancel macro helper, with
  enable, install/refresh, and undo controls. Its original character/spec scope
  is preserved. Disabling automatic refresh leaves existing macros usable.
- **Combat logging:** automatic advanced logging and the minimap book indicator.
  Disabling automation leaves an existing recording running; `/combatlog` stops it.
- **MiniAuras setup:** the existing Devastation aura setup integration. The toggle
  controls setup on login; existing groups are managed in MiniAuras itself.

`/aughelper` opens Augmentation settings in the hub. Use **Preview**, **Unlock /
move**, **Lock**, **Smaller / Larger**, and **Reset position**. Defaults are 48px
icons just below screen center. Settings persist in `LamdaUIDB.customModules`.
`/aughelper test`, `unlock`, `lock`, `reset`, `on`, `off`, and `status` remain
shortcuts. Preview uses sample numbers for ten seconds, outside combat only.

**Show observed Ebon time added** adds a small `+seconds` beneath the icon. It
measures increases in the buff's expiration timestamp since tracking began,
resets on a fresh cast, and does not count normal countdown ticks. This is an
observed total, not a combat-log total: reloads, combat breaks, and tracking
interruptions reset the observation window. Restricted timing or cast identity
leaves this optional counter blank. Ebon's timer is its buff duration, not its
cast cooldown. **Show a red ! icon when Ebon is missing** is optional and off by
default. If the self-buff query is unavailable, keep Ebon Might in a shown
Blizzard tracked-buff/bar viewer. Unknown state is never reported as missing.

From 0.19.2, lamdaUI places Ebon's self-buff into the native **Tracked Buffs**
icon viewer outside combat, saving its prior layout in `beforeEbonIconLayout`.
That viewer must stay shown to supply live buff data; Ebon is also included in
its native icons. If upgrading while fighting, leave combat once to finish setup.
Prescience opacity uses the native action cooldown with the global cooldown
excluded. It does not pass secret charge numbers into addon curve evaluation.
`/aughelper status` reports visibility, native setup state, and exact API errors.

The former `DevouringDevMacros` addon is replaced locally by a data-only
compatibility bridge. lamdaUI copies its macro undo state once, then owns the
modules. If the bridge was disabled, migration temporarily loads only that
verified data bridge outside combat and restores its disabled state. The legacy
SavedVariables file remains for rollback. No old alert frames or slash handlers
run from the bridge.

New helpers register through `Addon:RegisterCustomModule` in a separate
`Modules/*.lua` file loaded after `CustomHub.lua` and before `UI.lua`. A module
provides `id`, `name`, `description`, `defaults`, and optional `initialize`,
`changed`, `buildOptions`, and `refreshOptions` callbacks. Guard activity with
`IsCustomModuleEnabled(id)` and store settings with `GetCustomModuleSettings(id)`.
The registry builds the hub list automatically.

## Workflow

1. **Scan** reads the current specialization, form, action bars, bindings,
   spellbook, and active talent loadout.
2. **Capture Current** records the active Edit Mode layout, visible action
   bars, built-in Cooldown Manager setup, and every bound action-bar,
   direct-spell, or Blizzard Click Casting input it can resolve. Profiles also
   retain mouseover casting, automatic self cast, and the mouseover, self-cast,
   and focus-cast modifier keys. They can be named, switched, created, renamed,
   and deleted, so separate control setups remain reusable. On a first run,
   lamdaUI discovers those inputs without changing anything.
3. **Learn Current** refreshes and records the current specialization,
   form/page, talent loadout, profile, and placements in one click. It does not
   move actions or change bindings.
4. **Build Preview** scores the current spellbook against same-context,
   same-specialization, shared action, and shared role evidence. Blizzard's
   current specialization-aware Cooldown Manager categories provide portable
   semantics for active talents and previously unseen spells. A global
   assignment pass chooses the best complete set instead of greedily consuming
   a destination one spell at a time. Coverage shows how many possible
   placements are supported; its tooltip names every unassigned ability and
   explains whether it needs more evidence or simply ran out of managed inputs.
   After the selected action profile has learned at least one setup, unmatched
   abilities receive an explicitly low-confidence completion placement so a
   new specialization can still get a complete, reviewable first draft.
5. **Apply** first records the exact action, binding, and UI before-state
   in SavedVariables, reloads the interface so that restore point is on disk,
   and then makes the reviewed changes.
6. **Undo** restores the latest applied layout. If a managed action, binding,
   or UI setting was changed manually after Apply, Undo stops instead of
   overwriting that newer choice.

Preview never moves actions or changes bindings. Apply is enabled only when
every proposed destination has a supported, restorable before-state.

## Interface

The dashboard opens on **Custom Addons**. Its setup tools remain in three tabs:

- **Overview** shows the current specialization, active profile, learning state,
  preview state, and one concise next-step message. It contains only the actions
  needed to manage profiles, learn the current setup, or build a preview.
- **Profiles** shows every captured key, its learned use, and its current action.
  Profile capture, switching, naming, sharing, and profile-change Undo live in
  one compact card. Binding and action-slot details are available on hover.
- **Preview** shows learning totals, placement coverage, and a scrollable exact
  before/after diff. Apply, layout Undo, and layout history live here so a user
  cannot apply a proposal without seeing what will change.

The action-profile engine contains no class-, character-, hardware-, or key-specific starter
layout. Every managed input comes from the player's captured WoW bindings, and
every placement preference comes from confirmed learning or an imported
profile.

English clients can refine roles from localized tooltip wording. Every locale
uses Blizzard's language-independent specialization categories, helpful/harmful
flags, crowd-control flag, and resource-cost tokens; this identifies major
cooldowns, crowd control, and many spenders without translated spell names.
Placed interrupts also use Blizzard's language-independent action-bar flag.
The addon does not run English word matching against non-English tooltips.

## Context separation

Evidence is stored at several levels:

- Character + class + specialization + form/page + talent/hero loadout + active
  input profile
- Class + specialization
- Shared action history
- Profile-scoped action-role and input-usage preferences
- Profile-scoped preferred placements for explicit role rules
- Shared action-role preferences from other profiles

The most specific evidence receives the strongest weight. A Balance setup does
not replace a Feral setup, while shared concepts such as interrupts, movement,
defensives, builders, and spenders can still help configure an unfamiliar alt.
Blizzard Click Casting profiles are specialization-specific; lamdaUI captures,
learns, previews, applies, and restores them only in the matching specialization
and loadout context.

## Safety

- Scanning, learning, Apply, and Undo are blocked during protected combat
  states where required.
- A preview is pinned to the account-wide or character-specific binding set
  active when it was built. Stage, resumed Apply, and Undo stop if that scope
  changes, preventing bindings from being saved into the wrong set.
- Preview also stops before confirmation when the active binding scope cannot
  be identified, binding changes cannot be persisted, or a managed input
  resolves outside Blizzard's supported action bars.
- Action-bar Preview requires the complete forward and rollback API set,
  including cursor inspection, placement, clearing, and binding persistence.
  Resumed Apply checks those dependencies again before its first mutation;
  interrupted recovery does the same before making any additional change.
- Temporary vehicle and override bars cannot be learned as normal layouts.
- Learning and preview are read-only.
- Apply uses a persisted two-phase transaction. An interrupted or partial apply
  is detected on the next login and rolled back to its saved before-state.
- The complete immutable Apply/Undo payload carries an integrity checksum;
  inconsistent or altered recovery data is quarantined before any mutation.
- Undo verifies every saved action, binding, macro, and UI before-state before
  restoring the first one. A held cursor or any before-state that can no longer
  be recreated stops the complete Undo without clearing the cursor or making a
  partial change.
- Main-bar placements resolve the active stance or shapeshift page at preview
  time, including imported keys that are not bound on the new alt yet; fixed
  multi-bars continue to use their permanent slots.
- Blizzard Click Casting spell inputs use the same persisted transaction as
  action bars. Apply changes only the reviewed mouse-button and modifier pair,
  preserves unrelated target/menu bindings, and Undo restores the exact prior
  spell, macro, interaction, pet action, or empty input. Unsupported spells or
  unavailable Click Casting APIs block the complete preview before mutation.
- Mouseover casting, automatic self cast, and the mouseover/self/focus cast
  modifiers are captured as profile preferences. Preview shows each difference;
  Apply and Undo change them with the active binding set and persist modifiers
  only after the complete desired or restored state is in place.
- If the current Click Casting profile contains a malformed or newer record
  this version cannot reproduce without loss, profile capture and Apply stop
  without rewriting any Click Casting entry.
- The active Edit Mode layout, visible action bars, and Cooldown Manager setup
  use that same transaction and Undo path. When a portable profile's Edit Mode
  layout is missing locally, Preview validates its serialized layout, name,
  client APIs, and per-scope capacity. Apply installs and activates it only
  after the transaction is persisted; Undo and startup recovery remove that
  layout only while its exact imported contents are unchanged. A missing
  layout blob, invalid name/data, full layout scope, or unavailable API blocks
  the complete preview before any change is made.
- A locally installed Edit Mode layout is reused by name only when its
  serialized contents and scope match the captured profile. A same-name layout
  with different settings blocks Preview instead of being silently substituted.
- Actions that cannot be recreated exactly, including Assisted Combat and
  unidentified spell-backed macros, block the whole preview before anything is
  changed.
- Spell actions with a talent/form override retain their stable base spell ID
  in the recovery payload, so Undo can recreate the prior effective action
  instead of depending on a transient override ID.
- Incomplete or malformed spellbook records are ignored instead of becoming
  unplaceable recommendations.
- An unchanged setup is learned once, preventing repeated clicks or logins from
  inflating its weight.
- Relearning a changed context lowers the prior placement and raises the new
  placement.
- Invalid persisted recovery targets are quarantined before they can change an
  action, binding, or UI setting.
- Damaged SavedVariables roots, character snapshots, profiles, and history are
  reduced to safe defaults before they are read by the interface or apply path.
- When an earlier prototype is enabled, lamdaUI fails closed before database or
  UI initialization. A confirmation-only handoff copies the database into the
  new addon's own SavedVariables, disables the older addon, and reloads; a
  failed copy or disable attempt does not reload.
- Saved data remains local in WoW SavedVariables.
- Portable exports use a versioned format with Adler-32 integrity checking.
  Exports include only inputs owned by the selected profile; imports reject
  duplicate, mismatched, unsafe, malformed Click Casting, or foreign-input
  records before changing local data. Importing the exact same export again
  creates the requested profile but does not count its shared learning twice.
  A stable anonymous source ID prevents later full exports of the same profile
  from recounting cumulative shared learning. Existing `LAMDAUI1` through
  `LAMDAUI5` exports remain importable through the migration path.

## Commands

- `/lamdaui` or `/lui` — open or close the dashboard.
- `/lui hub` — open Custom Addons.
- `/aughelper` or `/lui aug` — configure Augmentation icons.
- `/lamdaui scan` — refresh the current setup.
- `/lamdaui keys` — open Profiles.
- `/lamdaui layout` — open Preview.
- `/lamdaui learn` — learn the current setup.
- `/lamdaui preview` — build a layout for the current specialization.
- `/lamdaui apply` — apply the reviewed preview.
- `/lamdaui undo` — restore the saved before-state.
- `/lamdaui history` — show recent Apply, Undo, and recovery activity.
- `/lamdaui export` — copy a portable profile.
- `/lamdaui import` — paste a portable profile.
- `/lamdaui profileundo` — undo the latest profile capture or import.
- `/lamdaui doctor` — print a privacy-safe readiness report with the binding
  scope, exact mutation capability groups, recovery state, and latest layout
  result. Character, realm, and user-created profile names are omitted.
- `/lamdaui help` — print command help.

## Local data

`LamdaUIDB` is stored in the account SavedVariables folder when the UI reloads
or the player logs out. Each character keeps one compact latest scan and up to
256 small context records; full spellbook and action-slot scans remain in memory
only. Learned specialization, form, loadout, and profile separation lives in the
training model. Portable exports contain
the selected action profile name, its input/UI map, learned spell placements,
and role preferences. They exclude automatically captured character names,
character snapshots, training sessions, macro identities and bodies, and layout
transaction history. The latest ten profile changes keep local rollback points;
imports add the action profile without deleting existing profiles and merge its
portable evidence into the local learned model. Undo Change rolls back both
parts of the import. Deleting a profile removes its profile-only contexts,
rules, and usage evidence while retaining shared transfer learning; Undo Change
restores both the profile and that scoped evidence. Exact local setup records
are capped at the newest 2,048 contexts. New exports use `LAMDAUI6` and preserve
the selected profile's aggregate evidence, role map, explicit preferred rules,
input-usage prior, and cast-targeting preferences. The anonymous profile-origin
ID contains no character or account identity. Legacy `LAMDAUI1` through
`LAMDAUI5` profiles are migrated during import.

`LamdaUIMigrationDB` is a short-lived, addon-owned handoff used only after the
player confirms an upgrade from the earlier prototype. It is cleared after a
successful database initialization and is not part of portable exports.

## Development and packaging

Run `tools/check-release.sh` from any directory to compile every shipped Lua
file, run the complete headless regression suite, scan the release payload for
prototype-specific text, validate the archive layout, and prove that two builds
are byte-for-byte identical. Run `tools/package.sh` to create
`dist/lamdaUI-<version>.zip`; the script refuses to overwrite an existing
package. Tests and release tools are not included in the player-facing archive.

## Production gates still in progress

- Locale-independent distinction among healing, defense, movement, and dispel
  abilities when Blizzard exposes no specific semantic flag
- In-game validation across every class and specialization
- Optional adapters for third-party UI addons

The live-client release matrix is maintained in [VALIDATION.md](VALIDATION.md).
