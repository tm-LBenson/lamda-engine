# LamdaUI requirements and acceptance criteria

Reviewed against the user conversation on 2026-09-13 for engine 0.7.1 / addon
0.27.0. This is the accepted product scope, not a completion claim.

## Product and priority

**LamdaUI is the lightweight in-game interface for Lamda Engine.** General is
the hub settings page; Modules contains each installed module. A separate
desktop settings application was deferred.

**LamdaCD must behave like MiniCC: icons on the user's unit frames, with the
corresponding aura, debuff, defensive, CC, and customization features.** The
user explicitly requested complete MiniCC replacement. This is required work,
not an optional roadmap. A detached timer bar and its settings editor do not
satisfy the request.

The original standalone lamdaCD 0.2.1 already had frame attachment. Inspection
also confirmed that installed MiniCC is a saved-settings bridge and MiniAuras
supplies the current CC functionality. The audited original baseline is MiniCC
4.6.3, before 12.1. Its features remain required; the full evolving successor
inventory must not silently expand the request.

Teammate cooldown readiness and active aura duration are separate requirements.
Native aura icons address current effects on teammates; they do not resolve
unknown cooldown readiness. The existing Vision helper and a broader
WeakAuras-like editor are explicitly later modules.

## Hub acceptance

| ID | Requirement | Acceptance check |
| --- | --- | --- |
| HUB-1 | `/lui` opens General. | Fresh login and reopening both select General. |
| HUB-2 | General contains hub/engine settings only. | Aura previews, positioning, filters, and sizing live under Modules → LamdaCD. |
| HUB-3 | Modules have independent controls and enable states. | Disabling LamdaCD stops its output; page changes hide the prior workspace. |
| HUB-4 | Navigation supports installed modules. | Registration adds a named module without rewriting the hub or adding nonfunctional placeholders. |
| HUB-5 | Controls show what they affect. | Frame previews and meaningful labels replace unexplained coordinate forms. |
| HUB-6 | Normal UI stays lightweight and quiet. | No personal helpers, diagnostic filler, or guessed Engine Online/Offline text. |
| HUB-7 | General owns update preferences. | Check/notification/frequency settings persist; checking never installs code. |
| HUB-8 | Public defaults work for friends. | No character, class, private path, or personal data is required; migration preserves saved data. |
| HUB-9 | Editing respects combat restrictions. | Container creation and changes are guarded/deferred safely; save/reload is combat guarded. |
| HUB-10 | Profiles cover module settings. | Save, switch, rename, delete, import/export, and reset preserve global preferences and reject invalid data. |

## Frame module acceptance

| ID | Requirement | Acceptance check |
| --- | --- | --- |
| FRAME-1 | Icons attach to actual frames. | DandersFrames and Blizzard show the correct unit's auras and follow movement/sorting. |
| FRAME-2 | Frame identity is authoritative. | Public unit bindings determine identity; slot number never substitutes for identity. Hidden, forbidden, secret, or conflicting targets are safe. |
| FRAME-3 | Show team CC, debuffs, defensives, and important auras. | Native groups show real effects during restricted encounters; own auras do not stand in for teammate validation. |
| FRAME-4 | Support pets and content rules. | Player/party/raid/pet toggles and world/dungeon/raid/arena/BG/delve rules control supported regions. |
| FRAME-5 | Useful customization affects live output. | CC, debuff, defensive and important-buff regions have independent position, growth, size and appearance, with separate party/raid layouts. Category switches/caps remain shared. Live icons and preview agree. |
| FRAME-6 | Preview without combat or engine. | Labelled samples use available frames or a fallback; stopping preview restores native auras. |
| FRAME-7 | Native display is independent. | Team icons work with engine stopped and logging off; native mode suppresses detached engine bars. |
| FRAME-8 | Preserve uncertainty. | Aura duration never means ability readiness; missing observations and samples never create readiness claims. |
| FRAME-9 | Respect supported filters. | Unsupported friendly-debuff spell-ID selection is not advertised as functional. |
| FRAME-10 | Verify real behavior. | Combat, sorting, roster changes, zoning, reload, and providers pass live checks; mocked calls alone are insufficient. |
| FRAME-11 | Replace old Lamda display without disabling unrelated features. | Successful native setup retires the standalone Lamda estimate region, preserves its preference, and leaves MiniAuras/Danders features alone. |

## Cooldown readiness acceptance

| ID | Requirement | Acceptance check |
| --- | --- | --- |
| CD-1 | Find useful teammate cooldown information. | A verified source supplies correctly attributed player observations with measured latency. |
| CD-2 | Do not invent readiness. | Unknown identities, talents, charges, resets, and missing observations stay unknown or explicitly estimated. |
| CD-3 | Test appropriately with followers/delves. | Native aura/layout tests use displayed units; log reuse estimates require NPC mappings and never borrow player cooldowns. |
| CD-4 | Diagnose missing data intentionally. | Explicit troubleshooting distinguishes known evidence without speculative status in normal UI. |

The disk reader has demonstrated delays above 20 seconds and cannot be called
dependable live readiness. Native aura containers do not cure that separate
problem. Addon-message sharing also remains unverified in restricted encounters;
readable own casts do not establish permitted or reliable send/receive delivery.

## Complete original MiniCC replacement acceptance

[FEATURE-COVERAGE.md](FEATURE-COVERAGE.md) records the audited MiniCC 4.6.3
baseline and mandatory remaining work. This includes separate CC and active
indicator regions, friendly and enemy cooldown estimates, talent/charge/reset
handling, per-spell selection, pet CC, healer warnings, nameplates, portraits,
trinkets, interrupt effects/timers, enemy alerts, sounds/TTS, the original
Precognition/Nullifying Shroud helper, profiles, and provider/customization parity.
Team debuffs are additionally explicit in the user's request.

The original frame controls include separate default versus raid layouts,
left/right/center/up/down growth, offset adjustments, relative icon sizing,
rows/columns, tooltip and decimal controls, and dispel/glow styling.
[Original CC controls](https://github.com/Verubato/mini-auras/blob/4.6.3/src/Config/CrowdControl.lua),
[original cooldown controls](https://github.com/Verubato/mini-auras/blob/4.6.3/src/Config/FriendlyCooldownTracker.lua).

The successor added a generic Personal Auras editor in 5.0 and a general
Blizzard Frame Auras replacement in 5.23. Those whole modules are not automatically
mandatory. Team debuffs remain required, while the broader WeakAuras-style
editor and Vision integration stay deferred as requested.
[Versioned successor history](https://github.com/Verubato/mini-auras/blob/5.40.0/changelog.md).

Each original feature needs working settings, runtime behavior, persistence,
restriction handling, and live checks. Where the current API prevents the old
behavior, record the exact limit and retain the unfulfilled requirement. Native
buff durations do not satisfy cooldown readiness. Empty tabs or generic bars do
not establish parity. Lamda uses original code and public Blizzard/provider
APIs, not wholesale copied All Rights Reserved MiniAuras source.

## Distribution acceptance

| ID | Requirement | Acceptance check |
| --- | --- | --- |
| DIST-1 | Public GitHub distribution. | Friends obtain source without authentication. |
| DIST-2 | PowerShell installs and compiles locally. | Missing Git/Go install; versioned source builds and addon/runtime files install. |
| DIST-3 | One desktop launch action. | Requested `lambaUI` shortcut/LUI logo start one engine; no desktop settings app. |
| DIST-4 | Updates preserve settings. | Reproducible releases; failed installs restore files and preserve SavedVariables. |

Local compilation does not guarantee the absence of Windows script/software
prompts.

## Current acceptance status

0.7.0 restored native aura icons; the 0.7.1 / addon 0.27.0 change separates four
regions and party/raid layouts, with eight growth directions and per-region
appearance. General/module profiles remain the hub. Native mode suppresses detached engine
rows and does not enable automatic logging. `/lui debug` exposes known provider
and runtime evidence without claiming engine liveness.

Full replacement, verified teammate readiness, and live combat/frame rendering
remain open. Automated results and observed limits are in
[VALIDATION.md](VALIDATION.md). The earlier dungeon error is undiagnosed; removing
legacy code does not establish its cause or prove it fixed.
