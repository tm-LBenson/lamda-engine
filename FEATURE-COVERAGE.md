# MiniCC / MiniAuras replacement target

The user's target is to replace the whole addon, not just its cooldown panel.
Reference: the installed MiniCC settings bridge and current MiniAuras configuration
panels inspected locally. This is a feature inventory, not copied implementation.
LamdaUI is **not yet a complete replacement**; keep MiniAuras for unfinished features.

## Current deliverable

| Feature | Lamda status |
| --- | --- |
| Teammate defensive estimates from disk logs | Implemented; live coverage/latency still need validation |
| Follower/delve companions | Five mapped NPCs; observed casts and isolated reuse estimates |
| Layout preview | Inline live samples and native drag/resize guide, labelled and isolated from observations |
| Window anchors and signed offsets | Nine anchors implemented for cooldown area |
| Row sizing, spacing, columns, count limit and upward/downward growth | Implemented for cooldown rows |
| Scale, font size, opacity, border, progress bars and color presets | Implemented for cooldown rows |
| Name, spell and timer visibility | Implemented for cooldown rows |
| Settings, module enable controls, source-build installer and update checks | Implemented |

## Still required for a full replacement

| Area | Required behavior |
| --- | --- |
| Positioning | Live unit-frame attachment, supported third-party frames and per-unit layouts (native drag/resize placement guide now implemented) |
| Icon presentation | Spell icons, icon sizing/padding, cooldown swipes, reverse swipes, stack placement, tooltips, glow and skin integration |
| Text and colors | Font selection, custom colors, category/class/dispel colors, text offsets and decimal thresholds |
| Display rules | Self filters; separate rules for world, dungeon, raid, arena and battleground contexts |
| Crowd control | Friendly/raid CC regions, category filters, per-unit limits and native aura display |
| Pet CC | Pet unit-frame regions and supported party/raid pet frames |
| Important auras | Defensive/offensive/CC/interrupt categories and independent limits |
| Party/raid frame auras | Buff/debuff regions, selected-spell filters, caster/duration filters and frame anchors |
| Healer CC | Separate healer warning region, icons, warning text, color and sound |
| Nameplates | Permitted CC, defensive and offensive auras attached to nameplates |
| Portraits | Player/target/focus aura presentation and selected additional buffs |
| Arena trinkets | Arena trinket presentation, identity/visibility limits, sizing and borders |
| Ally interrupts | Dedicated party interrupt tracker with its own settings and ordering |
| Enemy interrupts | Arena interrupt tracking, role filters, identity handling and unknown-event behavior |
| Enemy alerts | Important/defensive spell regions, split groups, filtering and sounds |
| Personal auras | Visual editor for triggers, units/spells, conditions, groups, layout, text, textures and sounds |
| Profiles | Create, rename, clone, switch, delete, import and export validated settings |
| Sound | Sound selection, volume/channel controls, TTS and per-trigger playback |
| Sharing | Module/group import/export, version migrations and validation |
| Compatibility | Coexistence settings, optional skin/media adapters, combat-safe updates and taint regression testing |

## Implementation boundary

Some displays belong inside WoW using supported aura/UI APIs. Others can use engine
log data with measured delay. A log event is not proof of live aura state, and an
external overlay cannot simply attach itself to an addon frame without position
information. Choose the appropriate implementation for each feature rather than
presenting an unverified equivalent. Unknown state must remain silent.

A feature is complete only after its settings, runtime behavior, persistence,
applicable combat restrictions and live tests are verified. The legacy personal
class helpers are not part of this replacement target.
