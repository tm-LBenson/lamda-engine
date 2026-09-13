# Original MiniCC replacement coverage

**Complete original MiniCC replacement remains required and unfinished.** The
concrete audited baseline is MiniCC 4.6.3, before the 12.1 restrictions, plus the
user's explicit team-debuff request. Installed MiniAuras is the successor and
runtime reference, not permission to add every new successor module to scope.

[MiniCC 4.6.3 source inventory](https://github.com/Verubato/mini-auras/tree/4.6.3/src),
[original friendly cooldown settings](https://github.com/Verubato/mini-auras/blob/4.6.3/src/Config/FriendlyCooldownTracker.lua).

Source behavior and live verification are separate. Keep existing addon features
you use until their replacements pass the checks in [VALIDATION.md](VALIDATION.md).

## Native frame work — 0.7.1 / addon 0.27.0

| Feature | Source coverage | Outstanding acceptance |
| --- | --- | --- |
| Attachment | DandersFrames/Blizzard, actual unit bindings, player/party/raid and supported pets | Live sorting/combat/visibility and other original providers |
| Aura regions | Four independently placed regions: CC, Debuffs, Defensives, Important buffs; big/external groups share the defensive region | Live team effects; original combined-indicator options and reference category behavior |
| Party/raid layouts | Each region has its own party and raid settings, selected within Frames/Appearance | Live selection/transitions and preview agreement |
| Placement | Nine frame anchors, distance/adjustments, eight growth directions, per-region wrapping | Original centered growth behavior and independent per-unit overrides where needed |
| Appearance | Per-region size/spacing/wrapping/font size/timers/stacks/borders/static highlights/swipe/reverse/tooltips | Relative percentage size, exact original glow/dispel coloring, decimals, additional styling |
| Categories | Shared category switches and icon caps | Original per-context/per-region selection rules, combined budgets and spell selection |
| Visibility | Player/party/raid/pet and world/dungeon/follower/raid/arena/BG/delve switches | Original separate region enable rules and live transitions |
| Preview | Labelled samples on actual frames or fallback | Actual WoW rendering, all regions/layouts and safe combat cancellation |
| Engine independence | Native auras need no engine/log; native mode suppresses detached rows | Persisted installed mode and engine behavior after reload |
| Migration | Successful native setup retires only the original Lamda display and preserves its preference | Live coexistence without disabling MiniAuras/Danders features |
| Hub/profiles/install | General/modules, profiles/import-export, source installer and update preferences | Actual Windows installation/update and runtime regression checks |

These icons show active effects and remaining aura duration, not ability
cooldown readiness. Original MiniCC's cooldown tracker remains separate missing
functionality. The retained log estimator has measured delays up to about 26.6
seconds and cannot establish dependable live readiness.

## Mandatory original features still incomplete

| Area | Remaining work |
| --- | --- |
| Friendly cooldown tracker | Correct teammate attribution, supported observations, talent-aware estimates, charges/resets, per-spell selection, trinket option, desaturation, active-buff/predictive presentation |
| Enemy cooldown tracker | Original enemy tracking/presentation, split groups, visibility and unknown-data handling within current API limits |
| Frame regions | Exact original separate CC/active-indicator behavior, context-specific selection/caps, centered growth, frame-relative percentage sizing, alpha/fade behavior |
| CC and interruptions | Original party/raid/pet controls, duration/decimal options, interrupted-unit indicators, dispel colors and glow styles |
| Healer CC | Separate warning region, icons/text, color, sound and supported detection |
| Nameplates | Original CC/defensive/important regions and frame scaling/filtering controls |
| Portraits | Player/target/focus and supported provider aura presentation |
| Trinkets | Original party/arena trinket display, supported identity/readiness behavior, sizing and borders |
| Interrupt timer | Original kick-timer behavior, filters, duration display and supported observations |
| Enemy alerts | Original important/defensive alerts, split groups, filters and audio/TTS |
| Personal helper | Original Precognition/Nullifying Shroud behavior within supported APIs; not a generic aura authoring system |
| Compatibility/styling | Remaining original providers, relative sizing, selected glow/color/decimal/font controls, media/skin integration and taint regression checks |
| Profiles/sharing | Coverage for every implemented original runtime, migrations and validated imports |
| Team debuffs | Explicit user request: useful native debuff regions and permitted dispel/category controls |

## Scope boundary from version history

MiniCC 3.0 introduced friendly cooldown guessing; later releases added charges,
per-spell configuration and enemy cooldown tracking. MiniAuras 5.0 removed the
friendly/enemy cooldown trackers for 12.1. Their removal from the successor does
not erase the user's original replacement requirement.
[Versioned 5.0 history](https://github.com/Verubato/mini-auras/blob/5.0.0/changelog.md).

The generic Personal Auras editor arrived in successor 5.0; its generalized
Blizzard Frame Auras replacement arrived in 5.23. These are not automatically
current scope. Team debuffs are explicit, but recreating every later successor
editor is not. Vision and the broader WeakAuras-like authoring interface remain
later modules. Nameplates, portraits, healer warnings, trinkets and TTS already
existed in original MiniCC and remain part of full replacement.
[Versioned successor history](https://github.com/Verubato/mini-auras/blob/5.40.0/changelog.md),
[MiniCC 2.35 release history](https://www.curseforge.com/wow/addons/minicc/files/7775867).

Use supported native display APIs; do not read, infer or decode secret aura
values. Friendly harmful auras cannot be selected by spell-ID candidate maps.
An active aura or cast record is not confirmed readiness. Lamda uses original
code and public APIs, not wholesale All Rights Reserved MiniAuras source.
Unknown engine status stays silent, and missing runtimes get no fake tabs.
