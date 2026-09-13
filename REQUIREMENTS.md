# LamdaUI requirements and acceptance criteria

This records the user's agreed product, including the corrections made after
testing. It is a requirements record, not a claim that the whole product is
finished. Requirements were audited on 2026-09-13 and source status was updated
after the corresponding hub review. Source checks and live verification are
distinguished below.

## Product and immediate priority

**LamdaUI is the lightweight in-game interface for Lamda Engine.** Lamda Engine
is a background process. A separate desktop settings application was explicitly
deferred. The immediate working module is **LamdaCD**, focused on teammates'
cooldowns. The user's own cooldowns already worked and are not the problem this
module is meant to solve.

The agreed order is: a usable General/modules hub, useful and honest teammate
cooldown tracking, full customization and MiniCC replacement coverage, then the
existing Vision helper and a broader visual rule editor. Calling the current
cooldown screen the entire hub does not meet that requirement. Recording a
missing feature in a roadmap does not implement it.

## Hub acceptance criteria

| ID | Requirement | Acceptance check |
| --- | --- | --- |
| HUB-1 | `/lui` opens LamdaUI on General. | Fresh login and reopening after editing a module both open General. |
| HUB-2 | General contains hub/engine settings only; Modules contains named module pages. | General contains no cooldown preview, sizing, positioning, companion or spell controls. LamdaCD owns all of those. |
| HUB-3 | Each module has its own controls and enable state. | Selecting another page hides the previous page and its editing tools; disabling a module stops its live output after settings are applied. |
| HUB-4 | Navigation grows with installed modules. | Registering another module gives it a named destination without putting its controls in General or rewriting the hub for that module. |
| HUB-5 | Controls explain what they change through their labels and location. | A layout editor clearly identifies its module/display, uses previews and direct placement, and does not present unexplained coordinate fields. |
| HUB-6 | Keep the UI lightweight and quiet. | No personal class helpers, long diagnostic text, fake status, unfinished-module placeholders, or guessed engine online/offline state. |
| HUB-7 | Engine update preferences belong in General. | A check-for-updates toggle persists; notification/frequency controls clearly apply to engine updates. Checking never installs code automatically. |
| HUB-8 | Public defaults work for friends. | No character, class, specialization, desktop path or private data is required by the public addon. Existing user data is preserved during migration. |
| HUB-9 | Editing is safe and predictable. | Saving is blocked during combat; entering combat or reopening the hub closes the placement editor safely; Cancel restores changes made in that editor. Invalid saved numeric settings cannot crash layout creation. |

## LamdaCD acceptance criteria

| ID | Requirement | Acceptance check |
| --- | --- | --- |
| CD-1 | Show useful teammate cooldown observations. | Real-player party casts are detected from a verified data source and appear for the correct teammate; own casts cannot masquerade as teammate coverage. |
| CD-2 | Preserve uncertainty. | Unknown cast identity, absent observations, talent changes and unsupported charge/reset behavior never become claims of confirmed readiness. Samples are visibly separate from observations. |
| CD-3 | Test in follower dungeons and delves. | Verified friendly companions produce display events; unsupported companions remain unknown. NPC abilities do not inherit player cooldown values without evidence. |
| CD-4 | Layout can be adjusted without fighting a boss. | A labelled preview works without the engine or fresh combat casts. Presets, size, scale, spacing and placement can be changed visually and persist after applying. |
| CD-5 | Full useful display customization. | Position, screen/frame anchoring, sizes, fonts, spacing, growth, limits, visibility and presentation controls affect the actual display as well as preview. Per-unit frame placement and icon presentation remain required for MiniCC parity. |
| CD-6 | Runtime reliability is measured, not inferred from replay. | Live log arrival delay, actual player coverage, zone/party transitions and overlay alignment are checked in game. Windows rendering must be tested on Windows. |
| CD-7 | Missing data has a discoverable diagnosis without cluttering gameplay. | An intentional troubleshooting action can distinguish no logging, no supported casts, a disabled module and display failure where the system has evidence. General stays free of guessed status. |

CD-7 is an implementation acceptance criterion inferred from the repeated
"teammates do not work" reports, rather than an explicit request for a permanent
diagnostic panel. It must not override the instruction to keep normal UI quiet.

## Distribution acceptance criteria

| ID | Requirement | Acceptance check |
| --- | --- | --- |
| DIST-1 | Public GitHub distribution for friends. | A fresh user can obtain the source without signing into GitHub. |
| DIST-2 | PowerShell builds locally. | The installer obtains missing Git and Go, builds the engine from source, and installs the addon and required runtime files. |
| DIST-3 | One desktop launch action. | The requested `lambaUI` shortcut and LUI logo start one engine instance; the overlay follows WoW visibility without a separate settings application. |
| DIST-4 | Updates preserve user settings. | Versioned releases are reproducible; installation failure restores replaced files and leaves SavedVariables intact. |

Local compilation is the requested distribution approach; it is not a guarantee
that Windows will never display an unsigned-software or script prompt.

## Full MiniCC replacement and later modules

The user explicitly requested that nobody need MiniCC once this is complete.
The locally installed MiniCC is now only a SavedVariables bridge into MiniAuras,
so its active MiniAuras modules are the reference feature inventory. The detailed
coverage matrix is [FEATURE-COVERAGE.md](FEATURE-COVERAGE.md).

Completion includes crowd-control and pet regions, important auras, party/raid
frame auras, healer warnings, nameplates, portraits, arena trinkets, ally/enemy
interrupts, enemy alerts, personal aura editing, profiles, sound, sharing, frame
adapters and the associated appearance/filter controls. These are independent
runtime features; adding empty tabs or an external cooldown bar is not parity.
Use supported native display APIs where applicable and verified engine data
where appropriate. A disk cast record is not live aura state.

The existing Vision marker helper should eventually be managed through LamdaUI
as another module. The requested future WeakAuras-like UI should support visual
creation/editing of rules and displays. Those later ambitions do not require
putting nonfunctional controls in the current hub or returning to a desktop
settings application.

## Current source status and unresolved checks

After the 2026-09-13 implementation review:

- General contains engine update preferences and module profiles. Modules has a
  scrollable list and separate detail pages. `/lui` selects General and cancels
  any placement guide first. Tests check recursive page visibility and ancestry
  with a second independent module, rather than only navigation flags.
- The module registry owns each module's defaults, validation and enable key.
  LamdaCD has Appearance, Placement and Tracking tabs. Configuration pages still
  build eagerly; registration does not yet implement an arbitrary engine
  module's runtime lifecycle.
- The visual cooldown editor has samples, presets, sliders and drag/resize.
  Screen anchors exist. Live party-frame attachment, spell icons, per-spell
  selection and most MiniCC/MiniAuras runtime modules are absent.
- Module profiles support create/clone, switch, rename, delete, reset,
  import and export. They preserve General update preferences. Import rejects
  unsupported keys and invalid values without executing Lua. Data is stored in
  the already-declared `LamdaUIDB.engineHub` namespace; other legacy fields are
  preserved.
- Teammate tracking uses disk-log successful casts and a 48-spell baseline
  catalog. It estimates timers; it does not establish actual readiness, current
  talented cooldowns, complete charges, or a current party roster.
- Five companions have explicit mappings. Brann and other unmapped companions
  are not supported. Follower observations do not establish real-player event
  coverage.
- Log tailing, replay, source filtering, settings parsing, geometry and installer
  rollback have automated checks. Live laptop observations have recorded cast
  delivery delays from 0.214 seconds to 26.637 seconds; a Valeera observation at
  15:04:57 arrived 20.327 seconds late. The delay problem is established, not
  merely an untested risk. Real-player Mythic+ event coverage remains unknown.
  Physical Windows overlay alignment and the native in-game editor still need
  live verification.
- The Go engine, public source releases, PowerShell installer, LUI assets and
  desktop shortcut exist. General update preferences persist through reload.
- The earlier in-dungeon addon error was never diagnosed. Removal of legacy
  code is not evidence that its cause has been identified or fixed.
- The earlier guide-over-General defect and invalid numeric-setting geometry
  failures have been corrected in source and covered by Lua tests. Those tests
  do not establish in-game rendering or persistence after the user's reload.

Update implementation status with concrete tests and observed behavior; retain
unverified or unfinished requirements until those checks have actually passed.

## Native sharing feasibility check

The idea of each client sharing its own readable casts was checked against
Blizzard's generated API documentation in the current public UI-source mirror
on 2026-09-13. It must not be advertised as a verified replacement for log
tracking in restricted encounters.

- `UNIT_SPELLCAST_SUCCEEDED` has the `SecretWhenUnitSpellCastRestricted`
  predicate. Its payload includes the unit, cast GUID and spell ID.
  [Unit event documentation](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/UnitDocumentation.lua).
- That predicate generally leaves player/pet casts readable, with per-spell
  secrecy flags taking precedence. A client must still check readability
  before interpreting its own event.
  [Secret predicate documentation](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/SecretPredicatesDocumentation.lua).
- `C_ChatInfo.SendAddonMessage` rejects secret arguments. The documented
  `AreOutgoingAddonChatMessagesRestricted()` query accounts for realm policy,
  including tournament exceptions, and notes that receive restrictions are
  separate. `InChatMessagingLockdown()` exposes active chat restrictions.
  [Chat API documentation](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/ChatInfoDocumentation.lua).
- The send-result enumeration includes `AddOnMessageLockdown`, so a send attempt
  is not proof that another client can receive usable observations.
  [Chat constants](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/ChatConstantsDocumentation.lua).

Inference: sharing readable own-cast observations could work where communication
is allowed and both users run the module. Those API declarations do not establish
delivery in the user's dungeon, delve or Mythic+ context. A scoped live send and
receive test would be needed before selecting it as a tracking source. Do not
try to decode secret payloads or route around a reported communication lock.
