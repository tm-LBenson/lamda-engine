# MiniCC / MiniAuras replacement coverage

**Complete replacement is required. It is not yet complete.** This inventory
comes from the installed MiniCC bridge and MiniAuras 5.40.0 modules/settings.
It records behavior to implement, not source to redistribute. Standalone
lamdaCD 0.2.1 already attached defensive estimate icons; 0.7.0 returns LamdaUI's
module to the intended frame-based direction.

“Implemented” describes source behavior. Live WoW rendering, combat safety, and
complete replacement need the checks in [VALIDATION.md](VALIDATION.md). Keep
MiniAuras features you use until their replacements are verified.

## Frame work in 0.7.0 / addon 0.26.0

| Feature | Current coverage | Outstanding acceptance |
| --- | --- | --- |
| Native attachment | DandersFrames/Blizzard; actual unit bindings; player/party/raid and supported pets | Live sorting/unit changes/combat and remaining providers |
| Friendly CC | Native crowd-control aura group | Live party/raid/pet coverage and full reference controls |
| Team debuffs | Native harmful group, separate CC category | Live restricted encounters; remaining dispel/caster/duration controls |
| Defensive buffs | Separate big/external defensive groups | Remaining category/selection/appearance parity |
| Important buffs | Native important-buff group | Full offensive/important selection and reference behavior |
| Placement | Nine frame anchors, distance and horizontal/vertical adjustments | Independent region/per-unit layouts, full growth options |
| Appearance | Icon size/spacing/wrapping/caps, text size, swipes/reverse, timers/stacks, borders, static glow, tooltips | Fonts/custom colors/text offsets, animated glow, skins/media |
| Visibility | Player/party/raid/pet and world/dungeon/follower/raid/arena/BG/delve controls | Live transitions and full reference filtering |
| Preview | Labelled samples on frames; fallback sample when needed | Native in-game visual checks |
| Engine independence | No engine/log dependency; native mode suppresses detached rows | Installed mode persists and applies on reload |
| Migration | Successful native setup hides only standalone Lamda display and preserves its preference | Live coexistence; other addon features remain untouched |
| Hub/profiles/distribution | General/modules, profiles/import/export, source installer, update preferences | Actual Windows install/update and regression checks |

These icons show active effects and their duration, not verified teammate ability
readiness. The separate disk-log estimator has measured delays up to about 26.6
seconds.

## Required remaining coverage

| Area | Required behavior still incomplete |
| --- | --- |
| Frame adapters | Remaining providers, provider changes, pet coverage, independent region/per-unit placement |
| Filters/categories | Full supported spell/category/caster/duration/dispel controls; separate party/raid configurations |
| Icon/text styling | Custom fonts/colors, class/dispel colors, text offsets/decimal thresholds, stack placement, skins/media |
| Healer CC | Dedicated region, warning text/icons, color and sound |
| Nameplates | Supported CC, defensive and offensive aura regions |
| Portraits | Player/target/focus presentation and selected additional buffs |
| Arena trinkets | Supported identity/visibility, sizing, borders and presentation |
| Ally interrupts | Dedicated tracker, ordering, settings and reliable supported observations |
| Enemy interrupts | Arena tracking, role filters, identity and honest unknown-event handling |
| Enemy alerts | Important/defensive regions, split groups, filters and sounds |
| Personal aura groups | Reference group editor: unit/spell selection, conditions, layouts, text, textures and sound |
| Sound/TTS | Selection, channel/volume, supported native triggers, TTS and per-trigger playback |
| Sharing/profiles | Group import/export and migrations for each runtime; module profiles alone do not cover these groups |
| Compatibility | Coexistence controls, optional skin/media adapters, combat-safe changes and taint regression tests |
| Cooldown readiness | Verified player observations/latency, talented intervals, charges, resets and missing events |

These are current acceptance work. Only Vision integration and the broader
WeakAuras-like authoring interface are explicitly later modules.

## Implementation boundaries

Use Blizzard's supported native machinery for current aura state. Lamda does not
read, infer, or decode its secret values. Friendly harmful auras cannot be
selected by spell-ID candidate filters on the current API; permitted native
category/dispel filters must be used where applicable. A cast record is not
live aura state.

MiniAuras is All Rights Reserved. Lamda uses original code, existing Lamda frame
work, public provider interfaces, and Blizzard APIs. Unknown engine status stays
silent; unfinished runtimes get no placeholder tabs implying that they work.
