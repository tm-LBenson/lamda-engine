# lamdaUI release validation

## Emergency recovery — 0.19.3

The user reported the UI crashing with errors after 0.19.2. Its headless checks
did not establish live-client safety. The exact new error stack is not known.
The Augmentation module is now suspended with no combat events, polling, combat
API queries, icon creation, or native layout changes. It cannot be re-enabled
through the hub. Confirm errors stop after a reload/restart before any further
combat-helper development or live installation. Other modules are unchanged.

This checklist covers behavior that cannot be proved by the headless Lua tests.
Run it against a copied SavedVariables file on the supported live WoW client
before publishing a release.

## 0.19.0 custom hub — live check pending

Live report on 2026-09-11: preview works, but combat tracking reports a Prescience
API failure and unavailable Ebon data. Version 0.19.1 adds exact runtime error
reporting to identify those failures. Combat visibility is not yet verified.

The 16:20 diagnostic screenshot confirmed `Curve:Evaluate` rejected the secret
Prescience count, and Ebon was absent from the shown icon viewer while the bar
viewer was hidden. Version 0.19.2 corrects both paths. Headless tests reproduce
those conditions; the corrected live display still needs a check after reload.

Headless checks validate logic and documented rendering inputs. After the local
update, reload and verify these behaviors in the actual WoW client:

- `/lui hub` opens Custom Addons; all four modules can be selected.
- `/aughelper` opens its options; preview and move/lock work.
- Prescience matches the action bar at 2, 1, 0, then 1 charges. At 0 the whole
  icon disappears; at 1 it returns. Both icons hide when combat ends.
- Ebon tracks the self-buff timer. Optional added time increases on extensions
  and resets on a fresh cast. A reload or combat break starts a new observation.
- Size and position survive reload. Other specializations hide the Aug icons.
- Existing macros work and their undo data migrated unchanged.
- A disabled legacy data bridge remains disabled after migration.
- The old labeled helper frames never appear alongside the new icons.
- Combat logging and MiniAuras retain their existing behavior while enabled.

## Test setup

- Back up `LamdaUIDB` and the account binding files.
- Test once with no prior `LamdaUIDB` and once with data from 0.13.x.
- Keep at least one empty action profile and one trained profile.
- Use an English client and at least one non-English client.
- Record the WoW build, interface number, locale, and conflicting addons.
- Run `/lui doctor`; save the report and confirm the binding scope and mutation
  capability groups are accurate. Confirm it contains no character, realm, or
  user-created profile name.

## First run and interface

- The first login opens the compact dashboard once and does not change bars.
- Custom Addons, Overview, Profiles, and Preview fit at the minimum supported resolution and UI
  scale without clipped controls or overlapping text.
- No character-, class-, hardware-, or key-specific example appears.
- Damaged root, character, profile, and history SavedVariables recover to safe
  defaults without showing saved markup or producing a Lua error.
- Scan refreshes specialization, form, talents, bindings, spellbook, and status.
- Changing the active talent loadout or a live spell override refreshes the
  spellbook and recommendation state after the event burst settles.
- Combat disables protected actions without producing Lua errors or taint.

## Upgrade from the earlier prototype

- With both versions enabled, the earlier addon loads first and lamdaUI shows
  one generic migration prompt without initializing its dashboard or changing
  either database.
- Cancel and confirm that no migration copy exists, neither addon is disabled,
  and the interface does not reload.
- Accept and confirm that lamdaUI copies the complete old database, disables
  only the older addon, and reloads once.
- On the next load, confirm that the copied data reaches schema 6, profiles and
  learning remain present, and the temporary migration variable is cleared.
- Confirm that the older addon's SavedVariables file is still present as a
  rollback copy and that only lamdaUI is enabled afterward.
- Simulate an unreadable database, missing disable API, and disable failure;
  each case must keep the older setup active and must not reload.

## Profiles and learning

- Create, rename, switch, capture, delete, and Undo a profile.
- Deleting a profile removes its profile-scoped learning, and Undo restores that
  learning without changing shared cross-profile evidence.
- A new profile does not inherit profile-specific confirmation totals.
- Learn two different specializations, two Druid forms, and two talent loadouts.
- Each exact context reports its own learned state after relogging.
- Repeated scans do not persist duplicate full spellbooks or action-slot maps;
  legacy redundant snapshot indexes disappear after the character is scanned.
- Relearning an unchanged setup does not increase its weights.
- Changing one placement lowers the old choice and raises the new choice.
- Switching input profiles preserves separate role and input-usage evidence.
- A form name localized differently from the profile that trained it still
  selects the same form/page context.
- A second key bound to the same action-bar command is ignored as an alias;
  direct spell bindings on separate keys remain separate managed inputs.
- Capture a specialization with a spell in Blizzard Click Casting and confirm
  the mouse/modifier input appears once while target and menu interactions are
  left unmanaged. Switch specializations and confirm its independent Click
  Casting profile is learned only in that specialization context.
- Give the profile non-default mouseover casting, automatic self-cast,
  mouseover-cast, self-cast, and focus-cast settings. Capture and relog; confirm
  all five preferences remain attached to the profile.

## Fresh-alt suggestions

- Open an untrained specialization with a trained action profile.
- Build Preview fills every available managed input up to spellbook capacity.
- Known actions and semantic role matches outrank completion fallbacks.
- Completion fallbacks show low confidence and an explanatory hover reason.
- An explicitly rejected action/input pair is not reintroduced by fallback.
- Capacity-limited and unsupported abilities appear in the coverage tooltip.
- Adjust the proposal manually, learn it, and confirm the next proposal improves.

## Apply, recovery, and Undo

- Preview does not move actions, change bindings, or change UI settings.
- Apply persists a staged transaction before the required reload.
- The resumed transaction places slot spells and direct-bound spells exactly.
- The resumed transaction replaces the reviewed Click Casting spell input by
  its base spell ID without changing unrelated target/menu inputs. Undo restores
  the exact prior spell, macro, interaction, pet action, or empty input.
- Preview a Click Casting input currently occupied by a macro, edit that macro
  before Stage, and confirm Apply stops. With the macro unchanged, Apply and
  Undo must restore the same macro name, index, and body.
- Main-bar keys target the active Druid form or stance page; multi-bar keys keep
  their permanent slots.
- Import a main-bar key that is currently unbound, enter a non-caster form, and
  confirm Preview targets the visible form slot before Apply installs the key.
- Change between account-wide and character-specific bindings after Preview,
  after Stage, and after Apply. Stage, resumed Apply, and Undo must stop until
  the original binding set is active.
- Simulate an unavailable binding scope, binding mutation API, and binding-save
  API; each must block the complete Preview before confirmation or reload.
- Simulate a missing cursor, action placement, action clearing, or rollback API.
  Preview must block before Stage. If one becomes unavailable only after Stage,
  resumed Apply must remain staged without changing its first target.
- Undo restores empty slots, spells, bindings, action-bar visibility, Cooldown
  Manager data, and the prior installed Edit Mode layout.
- Replace an action currently shown through a talent or form spell override;
  Undo must restore it through the base spell and verify the same effective
  action returns.
- Hold a spell or item on the cursor and start Undo. Undo must stop before any
  action, binding, or UI setting changes and must leave the cursor untouched.
- Make a saved prior macro or item unavailable while a transaction also has an
  earlier restorable action. Undo must fail its preflight before restoring that
  earlier action; after restoring the missing dependency, the full Undo works.
- A manual change after Apply causes Undo to stop instead of overwriting it.
- A spell Blizzard marks ineligible for Click Casting blocks the complete
  preview before reload or mutation.
- Change all five cast-targeting preferences away from the captured profile.
  Preview must show exact before/after rows; Apply must persist the desired
  values to the active binding set, and Undo must persist the exact originals.
- Manually change one cast modifier after Apply and confirm Undo stops without
  overwriting it. Restore the applied value and confirm Undo can proceed.
- Force a reload during Apply and confirm startup recovery restores before-state.
- During an interrupted mixed-state recovery, make one required rollback API
  unavailable. Recovery must stop before making any additional change, retain
  an actionable Recovery state, and complete after the API is available again.
- Force a failed restore and confirm the Recovery state remains actionable.
- Corrupt a copied transaction target and confirm it is quarantined without
  changing actions, bindings, or CVars.
- Change a syntactically valid saved before-action or Cooldown Manager value
  without updating its checksum and confirm the entire transaction is quarantined.
- Vehicle, override, and temporary action bars cannot be learned or applied.

## Portable profiles

- Export/import a LAMDAUI6 profile between two test accounts.
- The selected profile name, inputs, UI settings, aggregate evidence, role map,
  input-usage prior, and cast-targeting preferences survive; character names,
  macros, and history do not.
- Existing profiles remain present and name collisions receive a safe suffix.
- Imported evidence merges without removing local learning, and Undo Change
  restores the complete pre-import model and profile set.
- Importing the exact same string twice creates a second safely named profile
  without increasing shared setup totals or global action weights twice.
- Importing a changed later export with the same anonymous source ID refreshes
  profile-specific evidence without recounting cumulative shared evidence.
- Explicit preferred rules affect only the profile that learned or imported
  them; switching profiles changes the rule set without replaying older model
  migrations.
- Duplicate records, mismatched spell keys and IDs, unsafe display text, and
  learning that references an unknown profile input are rejected before import.
- Invalid Click Casting modifier masks and mouse-button names are rejected
  before the imported profile or learning model is changed.
- Known-good LAMDAUI1 through LAMDAUI5 fixtures migrate successfully.
- Truncated, altered, oversized, and markup-bearing strings fail without mutation.

## Edit Mode layouts

- An installed captured layout previews, applies, and undoes by layout name even
  after layout indices are reordered.
- Import a profile whose named Edit Mode layout is missing locally. Preview
  must show **Install and use**, Apply must add it to the serialized layout's
  account or character scope and activate it, and Undo must reactivate the
  previous layout and remove the imported one.
- Force a reload or client interruption after the imported layout is saved but
  before it is activated; recovery must remove it and preserve the prior active
  layout.
- Force a later action-bar or Cooldown Manager change to fail after layout
  installation; the all-or-nothing rollback must remove the imported layout.
- Manually edit the imported layout after Apply; Undo must stop without deleting
  the newer layout. Restore its exact imported contents and confirm Undo works.
- Invalid serialized data, an invalid name, a missing portable layout string,
  unavailable APIs, and a full account/character layout scope must block the
  complete preview before any action, binding, or UI mutation.
- Create a local layout with the imported profile's name but different settings
  or scope. Preview must identify the collision and must not activate or replace
  the local layout, including when that same-name local layout is already active.

## Performance and compatibility

- Turn Blizzard combat logging off, enter each supported instance family
  (party, raid, scenario/delve, battleground, and arena), and confirm lamdaUI
  enables it with `advancedCombatLogging` set to `1`. Exit the instance and
  confirm lamdaUI leaves logging enabled for subsequent outdoor or
  training-dummy combat. Confirm the book indicator appears beside the minimap
  while logging is active, reports advanced logging in its tooltip, and hides
  within one second after logging is manually disabled.
- Scan, profile switching, and Build Preview do not hitch noticeably with a
  full spellbook and ten stored profile-history entries.
- Repeated spell, binding, form, and cooldown events coalesce into one scan.
- Reload and relog produce no Lua errors with only lamdaUI enabled.
- Repeat with the supported third-party UI addons and document any adapter gap.

## Automated release gate

- Run `tools/check-release.sh` and require every source compile, regression
  suite, payload-content check, archive check, and reproducibility check to pass.
- Build the distributable with `tools/package.sh` only after the live-client
  cases above pass for the release candidate.
