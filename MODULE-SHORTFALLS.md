# LamdaUI module shortfalls

**Last reviewed: October 18, 2026 · Release baseline: [LamdaUI 1.3.0](https://github.com/tm-LBenson/lamda-engine/releases/tag/v1.3.0)**

This is the working backlog for all ten modules, plus the shared engine,
installer and release bot. It records missing capabilities, known limits and
unfinished verification. An item here does not necessarily mean a feature is broken.

Baseline: addon **0.69.0**, engine **0.24.0** (separate Retail/Forever packages),
desktop **0.4.0**, installer **1.0.1**. New installations start with all modules disabled.

## How to use this file

- **P1:** reliability, misleading results or an important release/verification gap.
- **P2:** substantial capability or usability work.
- **P3:** lower-priority extension or polish.
- **Open:** confirmed missing work. **Investigate:** reported symptom; cause not established.
- **Verify:** implementation exists; the specified real-client acceptance is outstanding.
- **Limited:** a known boundary. **Proposed:** an improvement to evaluate before implementation.
- Keep IDs stable. Add an owner and issue/PR link when work starts. Close an item
  only with a release/commit and evidence satisfying its completion check.
- Record Retail and Forever results separately. Automated tests, an installation
  receipt and a successful gameplay test establish different things.

The review used current integration sources, module documentation and release
records. Twelve directly copied module files were byte-compared with the Retail
1.3.0 package; they matched. Frames is assembled separately. No new gameplay
session or performance benchmark was run for this review.

**Source caveat:** GitHub `main` and the `v1.3.0` tag pointed to `13b3b77` during
this review, an older source tree containing only LamdaCD and Settings under
`addon/lamdaUI/Modules`. Release assets contain the newer product. Evidence paths
below refer to the integration checkout, not files currently available on this
branch. Closing **SHARED-001** will make those sources reviewable here.

## Module index

| Module | Main remaining concern |
| --- | --- |
| [LamdaCD](#lamdacd) | Patch-specific cooldown rules, uncertain teammate timers and live frame coverage |
| [LamdaKick](#lamdakick) | Missed/delayed observations and validation of the latest capture fix |
| [Performance](#performance) | Connecting game stalls with engine/Windows measurements |
| [LamdaGladius](#lamdagladius) | Real arena acceptance and opponent cooldown uncertainty |
| [LamdaOverlays](#lamdaoverlays) | Remaining WeakAuras capabilities and broad live acceptance |
| [LamdaFrames](#lamdaframes) | Advanced editors and Forever client restrictions |
| [LamdaVendor](#lamdavendor) | Unclear waiting states and upgrade-track recognition |
| [LamdaKBB](#lamdakbb) | End-to-end sync acceptance and concurrent edit recovery |
| [LamdaDrawer](#lamdadrawer) | Non-LibDBIcon buttons and compatibility acceptance |
| [LamdaSim](#lamdasim) | Incomplete models and a simple, unified recommendation workflow |

## LamdaCD

Present: native aura regions, own cooldowns, engine teammate estimates, spell
selection, independent layouts and full-capacity previews. Native effects and
own cooldowns do not require the engine; teammate estimates do.

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| CD-001 | P1 · Open | Talent cooldown rules are pinned to game version 12.1.0 and applied only to matching inspected versions. Later patches can fall back to less-specific baselines. | Audit the supported patch's spell/talent data; update versioned rules and test representative changed spells. Record the patch and source revision. |
| CD-002 | P2 · Limited | Proc reductions, resets, equipment and PvP modifiers are not fully modeled. Charge history can become uncertain. A teammate timer cannot guarantee readiness. | Track supported modifiers explicitly; test reductions, resets, charges and missed casts. Preserve an unknown/estimated result wherever evidence is insufficient. |
| CD-003 | P1 · Verify | Current frame attachment and mixed player/NPC handling need a fresh gameplay pass. The NPC packet correction has automated evidence, not complete live acceptance. | Exercise party/raid sorting, followers, pets, zoning, combat and reload on supported frame providers; verify attribution, preview agreement and native operation with the engine stopped. |

Evidence: `Lamda/UIAddon/Modules/LamdaCD.lua`, `Lamda/UIAddon/FrameAuras.lua`,
`Lamda/Engine/internal/engine/cooldown_rules.go`, `cooldown_rules.json`,
`known_cooldowns.go`; `UPDATES.md` (1.2.2/1.3.0).

## LamdaKick

Present: own native cooldown, teammate bars/frame icons, talent-aware estimates,
observed attempts/success counts and several observation sources.

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| KICK-001 | P1 · Verify | The reported missed/delayed kick led to a native header-overlap fix, but a fresh real-key acceptance run for that fix is not recorded. | Record teammate cast-to-display delay in a key, including a missed kick, pet interrupt, group change and delayed duplicate log. Verify each use is counted once. |
| KICK-002 | P1 · Limited | The meter fallback detects landed interrupts only and excludes ambiguous builds. Restricted/missed screen events can leave slow disk logs as the remaining source. | Document a tested class/spec coverage matrix and measured delays. Improve supported fast paths; retain uncertainty after capture gaps instead of implying authoritative readiness. |
| KICK-003 | P2 · Open | The spell picker accepts additions, but an added interrupt needs an engine rule before it can produce a teammate timer. The UI currently explains this through Help. | Make unsupported selections apparent at selection time, or supply a validated rule workflow; a user must be able to tell whether the selected spell will work. |

Boundary: current friendly tracking is for solo/five-player contexts and arenas;
raids and battlegrounds are hidden. Expanding that scope needs separate design.

Evidence: `Lamda/UIAddon/Modules/LamdaKick.lua`,
`Lamda/Engine/internal/engine/interrupts.go`, `interrupt_counts.go`,
`Lamda/Engine/internal/lookup/meter_casts.go`; `KICK_RELIABILITY.md`, `COLOR_CAPTURE.md`.

## Performance

Present: frame timing, slow-frame counts, optional addon CPU/network readings,
selected Lamda task timings, a floating tracker and a printed report.

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| PERF-001 | P2 · Proposed | The rolling game sample covers up to 60 seconds. Its Lua timings do not explain external engine, GPU or Windows mouse stalls, and there is no combined incident report. | Add an explicit, privacy-reviewed diagnostic export that aligns game and engine measurements around a reported stall. Include module/performance settings and distinguish unavailable measurements. |

The friend's reported lag remains **SHARED-003**. A ping spike alone does not
establish that addon Lua, the engine or the network caused it. Hiding the
Performance readout intentionally leaves collection running; disabling the
module stops collection.

Evidence: `Lamda/UIAddon/Modules/Performance.lua`, `Lamda/UIAddon/Performance.lua`.

## LamdaGladius

Present, **experimental**: native arena frames, casts, trinkets, CC/DR, pets and
optional engine opponent cooldown estimates. Native arena widgets must be
available; other arena-frame addons can cause LamdaGladius to pause.

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| GLAD-001 | P1 · Verify | Native widgets plus the full opponent capture/rendering chain need real arena-match acceptance. | Test multiple rounds with CC/DR, trinkets, pets, targets/focus, combat layout deferral and round resets; record client/build and any conflicting addons. |
| GLAD-002 | P2 · Limited | Opponent cooldowns use observed casts and baseline durations. Enemy talents, resets and charges are unverified; school lockouts require a successful interrupt record and verified target identity. | Review the pinned catalog for supported patches and test the supported observations. Keep missing identity and uncertain modifiers visibly unknown/estimated. |
| GLAD-003 | P3 · Open | Custom text tags and configurable team-chat announcements from the intended arena-addon replacement are not implemented. Current cast announcements are local only. | Define and implement the needed tags/opt-in announcements, with event deduplication and arena testing. |

Evidence: `Lamda/UIAddon/Modules/Gladius.lua`, `Lamda/UIAddon/Gladius.lua`,
`Lamda/Engine/internal/arena/model.go`, `CATALOG.md`; `GLADIUS.md`.

## LamdaOverlays

Present, **experimental**: nested events/conditions, all resources and comparisons,
persistent displays, aura stacks/timers, configurable arches, sounds, load
filters, character organization, sharing, spell search, cooldown/cast/encounter
inputs, groups/clones, media/animations, typed calculations, remembered values,
reusable actions and versioned packs. These are implemented features, not a
list of missing MVP work.

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| OVR-001 | P1 · Verify | The expanded trigger/clone/pack feature set has automated coverage but lacks broad real-character acceptance. A previously successful stack-arch test does not verify every aura or unit. | Test resources, buff/debuff presence/stacks/duration, missing inputs, new-source reload, persistent hide, casts, boss timers and party/raid/nameplate clones. Record source, unit, patch and result. |
| OVR-002 | P2 · Open | Direct WeakAuras imports are unsupported. Lamda's own import/export already works. | Build a supported-format converter with a review screen and explicit unsupported-feature results; test representative historical exports without executing imported scripts. |
| OVR-003 | P2 · Limited | Arbitrary Lua and third-party custom scripts are unsupported; the current logic system uses typed blocks and reusable actions. | Inventory real rules that blocks cannot express. Extend blocks where practical and explicitly decide the scope of any scripting feature before implementing it. |
| OVR-004 | P2 · Open | Other-unit cooldown discovery is not a general-purpose trigger. Existing player cooldowns and observed teammate/arena estimates do not provide arbitrary units' exact cooldowns. | Define supported units and evidence sources; add tested triggers with explicit estimated/unknown states. |
| OVR-005 | P2 · Open | The full historical utility-trigger catalogue has not been covered or mapped to current blocks. | Create a feature-by-feature trigger matrix, identify concrete missing cases, and split them into independently testable follow-ups. Do not count existing resource/aura/combat triggers as absent. |
| OVR-006 | P3 · Open | 3D model displays are unsupported. Existing icons, images, shapes and sprites do not provide that capability. | Define supported model sources and controls; implement browser/runtime rendering with positioning, animation and cleanup checks. |
| OVR-007 | P2 · Open | Packs cannot declare dependencies on other packs. Shared settings and reusable actions within the existing scope already work. | Add versioned dependency validation, missing/cyclic dependency handling and safe update/rollback behavior before enabling cross-pack references. |
| OVR-008 | P2 · Limited | Capacity is bounded: 200 overlays, 64 aura sources, 90 timer slots, 64 extra measurements/attachments and 64 simultaneous displays. Extreme combined metadata can exceed the shared packet; extra SHOW actions can be omitted at capacity. | Stress-test realistic multi-character libraries, packet limits and recovery. Improve attribution of the exhausted resource where needed; raise limits only with capture/runtime measurements. Existing Help already shows usage. |

Boundaries: new capture sources load on `/reload`. The manager is browser-based
and display output depends on the engine. Inactive/hidden capture must not be
treated as proof that a buff is absent. A complete historical WeakAuras
replacement is still unfinished.

Evidence: `Lamda/UIAddon/Modules/Overlays.lua`,
`Lamda/Engine/internal/overlays/{model.go,runtime.go,store.go,web/limits.js}`;
`OVERLAYS.md` (Implementation goal, Replacement scope and Implemented and bounded).

## LamdaFrames

Present: party/raid frames, saved healing profiles, spell/macro/item bindings and
Retail's retained advanced rendering backend.

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| FRAMES-001 | P2 · Open | The Lamda settings UI does not recreate all retained upstream advanced editors. Some existing advanced Retail configurations can render without a corresponding Lamda editor. | Inventory retained settings against exposed controls; prioritize Aura Designer, custom text and advanced layout/profile editing with round-trip preservation tests. |
| FRAMES-002 | P2 · Limited | Forever clients without the secure compiler cannot use keyboard-hover bindings or pinned boss frames; compatibility mode also constrains layout updates. Retail has a different capability set. | Recheck capabilities on new Forever builds. Verify native behavior when available and safe fallback otherwise; do not promise Retail parity on an unsupported client. |
| FRAMES-003 | P1 · Verify | Macro/item binding integration and preserved profiles have extensive backend checks, but still need a current real-character acceptance matrix. | Test mouse/wheel/keyboard where supported, spec/loadout switches, macros with explicit targets, trinket swaps and combat guards. Confirm existing binds and native macros remain intact. |

Macro bindings intentionally keep independent copies; editing the original
Blizzard macro does not silently alter an existing Lamda binding.

Evidence: `Lamda/UIFrames.lua`, `Lamda/UIFrameActions.lua`,
`Lamda/UIFrameSpellbook.lua`, `Lamda/Runtime.lua`; `BINDINGS.md`, `RETAIL.md`.

## LamdaVendor

Present: opt-in gear rules, per-character slot overrides, warbound handling,
keep protections, a read-only preview, bounded sale retries and repairs.

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| VENDOR-001 | P1 · Investigate | Adventure gear was reported unsold while the module said Waiting. The specific item's blocking condition was not captured. | Reproduce with the exact item variant and active rules; inspect Preview's reason and merchant state. Verify intended sell/keep behavior without weakening unknown-data protections. |
| VENDOR-002 | P2 · Open | The merchant summary groups bags, locks and retries into one waiting count. Preview has item-level reasons, but the summary does not identify the blocker. | Provide a concise route from Waiting to the blocked items/reasons; distinguish unavailable metadata from locked items and exhausted retries. |
| VENDOR-003 | P2 · Limited | Upgrade tracks are recognized from readable English names. Localized, trackless, crafted or unknown labels cannot safely satisfy a track rule. | Support stable client track identifiers where available, otherwise tested locale mappings. Test readable Adventure gear plus unknown labels; keep unknowns unsold. |
| VENDOR-004 | P1 · Open | In 1.3.0, gear-selling permission and general rules are account-wide. An unconfigured alt can sell Champion gear after another character enables the shared rule; only slot overrides are character-specific. Reproduced with a second character in the merchant regression. | Release the per-character settings/opt-in correction, then verify an unconfigured alt retains its gear in game. Preserve existing rules and protections, require explicit gear-selling permission on each character, and test migration, resets and reload. |

Evidence: `Lamda/UIAddon/Modules/Vendor.lua` (`Track`, `Matches`, `Evaluate`,
`Tick`, Preview); `VENDOR.md`. A fresh merchant acceptance check remains required.

VENDOR-004 follow-up: addon **0.69.1** is the locally validated correction
(47 Vendor scenarios and both full UI suites); it is not part of the linked
1.3.0 public release. The old data has no reliable
record of which character enabled gear selling, so migration retains rules but
starts gear selling off for every character. The module Enabled switch remains
account-wide. Until the corrected addon is loaded, holding Shift while opening
a merchant skips automation for that visit.

## LamdaKBB

Present: character boards, floating and quest-tracker placement, transparency,
bulleted notes, resizing/minimizing, content visibility rules, browser editing,
timestamped two-way sync and recovery history.

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| KBB-001 | P1 · Verify | Desktop-to-game edits on reload need a recorded end-to-end acceptance pass. Seeing a character in the manager proves only part of the sync path. | Edit on desktop, reload, edit in game and confirm the echo. Include deletion, offline edits, two characters and recovery after interrupted capture. |
| KBB-002 | P2 · Proposed | Conflicts resolve at whole-card/whole-board-configuration granularity. Concurrent edits to different fields of one record can displace one version into the bounded 200-record history. | Evaluate field-level merging or a compact conflict review. Demonstrate that users can identify and restore displaced changes without reviving deleted records. |
| KBB-003 | P2 · Verify | The tooltip argument fix is shipped, while fresh hover acceptance is outstanding. Large notes and tracker/floating layouts also need a current combined visual pass. | Hover cards in all placements, resize tall wrapped notes, page/scroll, minimize and cross instance/key visibility boundaries; confirm no Lua error or clipped required content. |

Desktop changes loading on the next manual `/reload` is the selected workflow,
not an outstanding request for immediate desktop-to-game updates.

Evidence: `Lamda/UIAddon/Modules/{KBB.lua,KBBUI.lua,KBBSync.lua}`,
`Lamda/Engine/internal/kbb/{model.go,store.go}`; `KBB.md` (sync and tooltip sections).

## LamdaDrawer

Present: movable addon-button drawer, hover/pin behavior, exclusions and restored
button ownership when disabled. It uses the original LibDBIcon buttons.

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| DRAWER-001 | P3 · Proposed | Arbitrary non-LibDBIcon buttons and multiple/custom bars are unsupported. HidingBar users may have layouts this single drawer cannot replace. | Inventory requested unsupported buttons/layouts before expanding scope. Preserve original click/tooltip behavior and avoid protected frames. |
| DRAWER-002 | P2 · Verify | The removed-API hover failure was corrected, but current real-client compatibility acceptance is not recorded. | Check hover, pin, exclusions, late-loaded icons, combat deferral, pet battles, scaling and restoration on disable; verify coexistence/pause behavior with HidingBar. |

Evidence: `Lamda/UIAddon/Modules/Drawer.lua`; `DRAWER.md`.

## LamdaSim

Present: snapshots, supported damage comparisons, Augmentation group scenarios,
basic all-spec gear guidance, build trends, bounded loot/target-set planning,
preset healer gear scores, experimental Healing Lab and Forever item-stat comparisons.

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| SIM-001 | P1 · Open | The intended simple goal → analyze → prioritized actions workflow is unfinished. Separate calculation/reference screens do not yet provide complete personal gear and talent advice. | Walk a new user through choosing content and allowed sources, analyzing the equipped baseline, and receiving a short equip/craft/farm/talent action list with supported gains and clear model limits. |
| SIM-002 | P2 · Open | Tank combat/survival calculations are unavailable. Basic gear guidance is not a substitute for a tank model. | Add reviewed tank models and appropriate survival/throughput metrics, or explicitly retain unsupported status per spec. |
| SIM-003 | P1 · Open | Healer gear scoring uses provider presets; Healing Lab rejects gear/talent replacements. Complete actual-build healer comparisons remain unavailable. | Apply captured talents, racial effects and relevant gear/procs; validate replacements and representative logs per supported spec. Keep preset scores and predicted encounter HPS distinct. |
| SIM-004 | P2 · Limited | Forever compares captured item stats only; it does not model combat, talents, procs or legal optimized sets. | Add reviewed Forever models/eligibility checks before ranking upgrades. Until then retain stat deltas without presenting them as DPS/HPS or a best set. |
| SIM-005 | P2 · Limited | Loot planning excludes incomplete pools, and drop/reroll probabilities depend on explicit assumptions. Bounded target searches and separate gear/talent comparisons do not establish global best-in-slot. | Track pool completeness by patch/spec/source, resolve supported missing outcomes, and validate acquisition assumptions and whole-set interactions. Preserve best-tested-set wording until broader search/model evidence exists. |
| SIM-006 | P1 · Verify | Current-patch accuracy is not established by a successful worker run. Models have pinned assumptions; Augmentation uses fixed simulated allies, while learned healing data has selection and sample-size limits. | Maintain per-build/spec model reviews, held-out checks and effect coverage. Record known omissions and scenario limits alongside results. |
| SIM-007 | P2 · Open | Some calculator capabilities require separately installed workers. The private healer worker is not included in ordinary public updates, so a working developer setup does not establish a complete new-user installation. | Test each advertised capability on a clean public install; provide a simple supported setup path or clearly identify unavailable providers. Do not redistribute a private worker without permission to do so. |

Evidence: `Lamda/UIAddon/Modules/{Sim.lua,SimUI.lua}`,
`Lamda/Engine/internal/simming/{provider.go,healing.go,healing_encounter.go,forever.go,loot.go}`;
`SIM_FEATURE_WORK.md`, `SIM_AUGMENTATION.md`, `HEALING_SIM.md`,
`Lamda/SimProviders/{healing,forever}/README.md`.

## Shared engine, UI and releases

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| SHARED-001 | P1 · Open | Public source/tag provenance lags the distributed release. The 1.3.0 tag currently identifies the older source snapshot, not the full ten-module integration. | Publish a reviewed, credential-free source snapshot and reproducible build instructions for the distributed components. Record the actual source commit in future releases; preserve old tags and explain historical asset/source differences. |
| SHARED-002 | P2 · Open | README, REQUIREMENTS and FEATURE-COVERAGE describe an older product. The old MiniCC checklist mixes genuinely outstanding work with features now delivered by CD, Kick, Gladius and Overlays. | Refresh installation/product docs and map every original MiniCC requirement to its current module and evidence. Keep unmet requirements; remove stale missing-feature claims only after verification. |
| SHARED-003 | P1 · Investigate | A user's party-join lag, mouse stalls and ping spikes have no confirmed root cause. Performance changes shipped, but no before/after result from the affected machine is recorded. | Compare the same party-join/key-start scenario with modules off, selected modules on, and Normal/Low resource settings. Record frame time, engine CPU/memory and game latency; identify the responsible work before claiming resolution. |
| SHARED-004 | P1 · Verify | The new scan policies, hidden-strip behavior and independent disabled-module shutdown need a combined live acceptance matrix. Capture also depends on screen visibility, focus and geometry. | Test scan timing, key-start combat deferral, each module off/on, Hidden/Start hidden, strip moves/resizing, UI scaling, focus loss and recovery. Confirm no unused requests/readers remain and no stale values become fresh observations. |

Current settings already exist under **General → Performance**, **General →
Tracking** and **General → Color strips**. They are not missing-feature requests.
Compatible existing-file updates can load on `/reload`; new addon files/load-list
changes can still require closing WoW. Code signing is not planned by the owner.

Evidence: `UPDATES.md`, `COLOR_CAPTURE.md`, `BACKGROUND_UI.md`, `DESKTOP.md`,
current public branch/tag contents and the 1.3.0 release.

## Discord release bot

This is supporting release tooling, not an addon module.

| ID | Priority / status | Shortfall and impact | Completion check / next action |
| --- | --- | --- | --- |
| BOT-001 | P2 · Open | The bot posts through the message API without a persistent Gateway connection, so it appears offline. | Add a maintained connection, heartbeat/reconnect handling and honest online/connection status; verify a disconnect recovers without duplicate posts. |
| BOT-002 | P2 · Limited | Authorization is memory-only. Restarting the container starts the process but requires private reconnection before it can post. | Clearly expose reconnect-required status and test restart recovery. Any unattended credential solution needs an approved secure design; never put a token in this repository or tracker. |

Evidence: `Lamda/Discord/{bot.py,README.md}`. Existing release posts and post
editing work; automatic posting/chat commands are not implied requirements.

## Keep fixed work closed

Do not reopen these as unimplemented without a fresh reproduction:

- Low resource/Advanced controls, configurable scan timing and disabled-module work shutdown.
- Hidden color strips, optional Start hidden, individual/group movement and Overlays resizing.
- KBB transparent styling, note bullets, corner resizing, minimization and quest-tracker placement.
- KBB tooltip argument correction and Kick native header-overlap correction
  (their remaining live checks are tracked above).
- Overlays nested conditions, resource comparisons, persistent displays,
  character organization, import/export and configurable stack arches.

## Updating an item

For each change, retain its ID and record:

`ID · owner · status · issue/PR · release/commit · client/build · evidence · last reviewed`

When complete, move it to a **Completed** section with its evidence instead of
deleting its history. New reports should include the module, version, shortest
reproduction, expected/actual behavior and relevant settings. Keep private
captures, character identifiers and credentials out of public evidence.
