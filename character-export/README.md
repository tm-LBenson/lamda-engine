# LamdaExport

A native, independent character snapshot addon. No Lamda Engine, website,
account connection or other addon is required. The initial installation targets
the owner's Forever production client. Retail API adapters share this source;
Retail installation is a separate explicit action.

Type `/export` (or `/lexport`). The window gathers a fresh JSON snapshot and
selects all of it. Ctrl+C uses WoW's native EditBox clipboard handling and closes
the window after key-up and a short delay. Releasing Ctrl before C still works.
Escape cancels. The window can be dragged. `/export status` prints the addon
version and the time of the last completed snapshot.

The snapshot includes identity/build, level/XP/rested XP, guild, gold, bind and
current location, map coordinates, gear and full item strings, enchants/gems,
durability, item levels and stats, carried bags/reagent bag/keyring, professions
and skill caps, talents, learned player/pet spells, active pet, active quests and
objectives, completed quest IDs, reputations, currencies, dungeon/raid lockouts,
owned mounts/toys/companions, action bars, bindings, group composition and addon
versions. Macro names and resolved actions are included; arbitrary macro bodies
and private chat contents are not collected.

Profession parent/child IDs are grouped into one entry; `skillLines` retains
each tier's own ID, rank and cap. Bag `freeSlots` counts general storage only;
`specializedFreeSlots`, `keyringFreeSlots` and `totalCarriedFreeSlots` keep the
other capacity separate. Resistances include a school name and the resistance
API index, distinct from the spell-damage school index.

Bank/account-bank contents are captured only during an actual bank visit and
kept with timestamps. Recipe lists are cached when a profession window is
opened. Unvisited storage, unsupported collections and restricted reads are
explicitly unavailable or partial, never presented as empty verified ownership.
Classic recipe-window filters and battle-pet filters can limit those lists.
Toy filters are temporarily broadened only when their full state can be read
and restored. Quest/reputation/currency headers are restored after enumeration;
combat exports leave collapsed headers alone and label missing rows partial.
The addon does not open bank/profession windows or move inventory items.

Exports run on request, with one section per frame and bounded item-data waits.
There is no continuously running scanner. Bank and profession cache refreshes
are event-driven and coalesced. One latest completed snapshot and the storage /
recipe caches are kept in the character's `LamdaExportDB`; WoW writes this on
its normal logout/reload cycle. There is no automatic upload or chat send.

API adapters are checked against Blizzard's Forever UI API source, including
[skills](https://github.com/Gethe/wow-ui-source/blob/70ef1b2fd78061a73f886c4a1e79dc5b5cff6d5e/Interface/AddOns/Blizzard_APIDocumentationGenerated/SkillInfoDocumentation.lua),
[quests](https://github.com/Gethe/wow-ui-source/blob/70ef1b2fd78061a73f886c4a1e79dc5b5cff6d5e/Interface/AddOns/Blizzard_APIDocumentationGenerated/QuestLogDocumentation.lua),
[mounts](https://github.com/Gethe/wow-ui-source/blob/70ef1b2fd78061a73f886c4a1e79dc5b5cff6d5e/Interface/AddOns/Blizzard_APIDocumentationGenerated/MountJournalDocumentation.lua)
and native EditBox methods. Storage IDs follow the client's
[bag enum](https://github.com/Gethe/wow-ui-source/blob/70ef1b2fd78061a73f886c4a1e79dc5b5cff6d5e/Interface/AddOns/Blizzard_APIDocumentationGenerated/BagIndexConstantsDocumentation.lua);
resistance indices/return values follow the
[native character panel](https://github.com/Gethe/wow-ui-source/blob/70ef1b2fd78061a73f886c4a1e79dc5b5cff6d5e/Interface/AddOns/Blizzard_UIPanels_Game/Camelot/PaperDollFrame.lua).
The first live Forever export was pasted successfully on 2026-11-06. Its empty
ammo, parent/child profession rows, mining bag/keyring and Frost resistance
observations are regression fixtures in 0.1.1. Automatic window closure and
the repaired snapshot still need user acceptance in the running client.

To install this independent addon manually, copy the contents of this directory
into `Interface/AddOns/LamdaExport` for the desired client. Its TOC and Lua files
must sit directly in that folder. Preserve any existing SavedVariables. On a
first installation, restart the game yourself; an update normally needs
`/reload`. This addon is separate from the LamdaUI release installer.
