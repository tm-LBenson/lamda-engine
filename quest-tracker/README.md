# Lamda Quest Log

A standalone Lua/XML addon replacing the **on-screen quest tracker on the right**.
Forever is the first target; Retail uses the same source with capability checks.
The map and quest-log window remain available for quest details.

## Features

- A height-limited tracker with mouse-wheel scrolling and a Blizzard scrollbar.
- Blizzard tracker font objects, colors and header artwork when available.
- Quest levels in titles, with native circular badges showing `...` for
  incomplete quests and `?` for turn-in, in both the tracker and manager.
- Native quest difficulty colors on titles and levels: red, orange, yellow,
  green and gray, with player-level and scaling updates.
- Click a quest's circular badge to focus native quest navigation. Hover the
  badge or title to highlight its map area and POI without changing navigation.
- Tracked profession recipes and recrafts with live required-reagent counts,
  inventory/currency updates, recipe opening and explicit untracking actions.
- Native waypoint, filter, add, padlock and settings icons with hover tooltips.
  A separate chevron collapses the tracker. The padlock immediately switches
  between closed (locked) and open (unlocked).
- A dark slate manager and task editor with flat controls, sans serif text,
  icon navigation, active-page highlighting and slim draggable scrollbars.
- A sidebar with Quests & Tasks, Categories, Settings, and Appearance pages.
- Category and Settings dropdowns show a selectable list instead of cycling.
  An empty category selector is greyed out; the adjacent + creates and selects
  a category while preserving every unsaved field in the task draft.
- Compact menus on tracker right-clicks and manager three-dot buttons, with
  direct category choices and actions for quests, recipes and personal tasks.
  Tracker menus keep Move up / Move down beside the entry.
- Quest details open Blizzard's standalone details popup, independently of the
  map quest panel. Explicit menu actions ignore Shift/chat-link modifiers.
- Opaque manager/editor panels. Task and category dialogs open above a dimmed
  manager, blocking clicks on the list until the dialog closes.
- Live sliders for background opacity (0–100%) and tracker font size (12–20).
- Settings, sidebars and task/category edits remain available during combat.
  Opacity updates immediately; protected tracker reflow waits until combat ends.
- Quest switching, tracking, details, waypoint navigation and recipe actions
  accept clicks during combat. The selected marker updates immediately.
- Group by quest-log zone heading, your own category, quest type, or no grouping.
- Gold section titles with muted counts and thin underlines. Click the title
  to collapse or expand its section; there are no dash prefixes.
- Sort within groups by name or level in either direction, quest progress,
  campaign priority, quest-log order, completion, distance or manual rank.
- A **Quick sort** menu with distance, level, name, progress, campaign and
  quest-log orders. Combine any order with **Ready for turn-in first**.
- Filter by native quest type, zone and custom category using checkbox lists
  populated from your current quest log and tasks.
- Collapse groups; show tracked quests or the whole quest log; search in settings.
- Add, edit, complete and delete personal tasks with multiline notes, a zone and category.
- Save a `/way` command on a task and click its **waypoint pin icon** to navigate.
- Attach an existing WoW macro by name and click **M** to run it.
- Use native secure buttons for quest items and named macros.
- Drag and resize the tracker when unlocked. Settings and entries are saved per
  character, within each client's separate SavedVariables directory.

## Development status — 2026-11-14

Version 1.0.0 passes **65 behavioral scenarios and 1455 assertions**, plus
runtime-reference, Lua 5.1 syntax and XML checks. The data tests include 500
entries. The expanded sorting menu, combat interaction changes, native blinking
Notes caret and waypoint icon require a real client check; mocks cannot verify native
rendering, cursor visibility or taint. Local mock layout previews omit native art.

Version **1.0.0 is installed in Forever and Retail** as of 2026-11-14. Both
previous 0.4.8 builds were backed up before six changed runtime files were
replaced. All twelve runtime hashes match the checked release package; 101 other
Forever addon TOCs and 147 Retail addon TOCs were unchanged. See the
[Forever record](Install/Forever-20261114T204707Z.json) and
[Retail record](Install/Retail-20261114T204707Z.json). SavedVariables, macros and
bindings were not written. The user must `/reload` and verify sorting and combat actions in
the real client; no game input or restart was performed.

Version **0.4.7 was installed into Retail for Lewis's requested testing** on
2026-11-12 (local time). Retail's executable reports `12.1.5.70783`, matching the
TOC's `120105` interface tag. All twelve installed files match the checked release
archive, and 116 other addon TOCs were unchanged. This is a new Retail addon
installation; no previous copy needed backup. See the
[Retail installation record](Install/Retail-20261113T023728Z.json).

Version **0.4.7 was also installed into Forever** on 2026-11-12 (local time),
replacing 0.4.5 at Lewis's request. The previous addon directory was backed up
and verified before six changed runtime files were replaced. All twelve runtime
hashes match the checked release package; 23 other addon TOCs and the Retail
runtime hashes were unchanged. See the
[Forever update record](Install/Forever-20261113T040919Z.json). SavedVariables, macros and bindings
were not written, and WoW was not stopped or restarted. The user will `/reload`
and verify the new UI and Notes caret in-game.

Read-only executable version checks on 2026-11-11 found Forever `1.60.1.70888`
and Retail `12.1.5.70783`.
The interface list includes Forever `16001` (also verified against its local addon
setup), Retail `120105`, and locally observed addon declarations `120100`, `120107`.
The TOC list does not establish compatibility with every corresponding build.

The design uses native tracker fonts/art rather than recreating the entire
Blizzard tracker implementation. Exact visual parity, frame taint behavior,
quest-item activation, native module restoration and Forever waypoint support
require the in-game checks below. Non-quest tracker content remains Blizzard's
responsibility and can occupy a separate position from this tracker.

## CurseForge submission

Version 1.0.0 was uploaded as a **Release** for **WoW Forever 1.60.1** on
2026-11-14, under **All Rights Reserved**. Project ID: `1799597`;
file ID: `9618323`. The author dashboard reports **Processing**, with automatic
publication after approval selected. Public/app availability is not yet confirmed.
Retail was not tagged for this upload.

[Author dashboard](https://authors.curseforge.com/#/projects/1799597/files) ·
[Project](https://www.curseforge.com/wow/addons/lamda-quest-log) ·
[1.0.0 file page after approval](https://www.curseforge.com/wow/addons/lamda-quest-log/files/9618323).
The [submission record](Release/curseforge/submission-1.0.0.json) includes package
hashes, saved changelog verification and a CDN archive-length check. The checked
12-file runtime package matches both client installations. Release artwork, tests, mock
layout previews, local tooling and notes are excluded from the ZIP.

The previous 0.4.8 release is **Approved** and remains in history alongside
0.4.7, 0.4.6, 0.4.5 and 0.4.3. Version 0.4.4 was local only; its Notes fix is included in later
builds. The 0.4.7 native caret settings and viewport inset are a separate follow-up
to Lewis's report that typing worked while the caret was invisible.

## Usage

Click the small **settings gear** on the tracker to open its manager. The sidebar
provides these pages:

- **Quests & Tasks** lists your game quests, tracked recipes and personal tasks,
  with search, completion checkboxes and a **⋮** actions menu on each row.
- **Categories** creates, renames and removes your custom categories.
- **Settings** controls grouping, sorting, tracked/all quest scope, filtering,
  enabling the tracker, its lock state and resetting its position.
- **Appearance** adjusts background opacity and font size with sliders. Opacity
  follows Blizzard's default UI behavior: the background changes, and text stays
  readable. Headers are two points larger than the selected text size.

The **⋮** menu replaces the repeated Category/Edit buttons, leaving wider titles
and a quieter list with subtle row dividers. **Change category** lists your
categories directly and marks the current choice, including **Uncategorized**.
Right-click a tracker entry for the same compact menu, anchored at its title,
with **Move up / Move down** to set manual order within the displayed group.
The menu closes on scrolling or when its entry is replaced. Combat alone does
not close it or disable its actions.
Quest menus offer **Open quest details** and **Track quest / Untrack quest**.
**Open quest details** opens the native standalone description popup and closes
the manager so it cannot cover the description. Repeating the action leaves the
same quest open. Title-click Shift/chat-link shortcuts still work as before.
Untracked quests stay available in the manager, so you can track them again.
Recipe menus open or untrack the matching craft/recraft. Personal-task menus
offer editing, completion, waypoint navigation/copying when present, and deletion.

Menus use Blizzard's native context-menu system when available. A compact,
opaque popover provides the same actions on clients without it; long category
lists scroll. Click the dots again, click outside or press Escape to dismiss.
Unchanged list refreshes keep an open menu in place. Scrolling, replacing its
row, switching pages or closing the manager closes it. Entry actions remain
available during combat, including quest tracking, details, recipe actions and
waypoint navigation. Combat transitions preserve an unchanged menu.

Tracker background opacity affects only the on-screen tracker. The manager and
task/category editors always use an opaque background, so the game and list
behind a form cannot show through it. Closing or saving a dialog restores access
to the manager beneath it.

Click anywhere inside **Notes / objectives** to focus its multiline editor.
The native caret has an explicit 0.5-second blink rate and a small inset to keep
it inside the scroll viewport at the first column. A gold outline marks the
focused field. Enter inserts a new line; Tab / Shift-Tab moves through the task
fields. Use the wheel, click the slim rail or drag its thumb for longer notes.
Escape releases keyboard focus. Long notes remain scrollable, and the scrollbar
is hidden when everything fits.

The **settings gear** remains visible if you disable the tracker, so you can
enable it again with the mouse. Click the padlock beside it to toggle position/size
locking: a closed padlock means locked, and an open padlock means unlocked.
When unlocked, drag the header title to move or the lower-right corner to resize;
click the padlock again to lock. Its icon updates immediately, including when changing the
lock preference in combat. Position, lock, opacity and text size save per character.

Click the **add icon (+)** to add a task directly from the tracker. Right-click a quest or task
to assign a category or move it up/down. Moving an entry selects manual ordering
within its displayed group. Real quest titles include their level; status shows
`...` for incomplete, `?` for ready to turn in, and `!` for failed quests.

Quest titles and level labels use WoW's native difficulty colors: red for much
higher-level quests, orange for higher-level quests, yellow for quests near your
level, green for easier quests and gray for trivial quests. Colors refresh when
your level or effective level changes. Retail's content-difficulty API handles
scaled quests when available; other clients use their native level-based color
helper with the quest's difficulty level and scaling flag. Missing color APIs
leave the title gold. Section headings, status badges and objective progress
keep their existing colors; personal tasks and recipes have no quest difficulty.

The title has its own row with a thin divider and a collapse/expand chevron.
The native icon controls sit below it: waypoint and filter on the left, add,
lock and settings on the right. Hover an icon for its label and help. Active
quick sorting or filters add a subtle glow; the filter tooltip reports the
number of exclusions. The header controls remain available when collapsed.
Their mouse targets are separate from the title's drag area and sit above the
native tracker, with explicit mouse-click handling.

The **waypoint pin (Quick sort)** opens these choices:

- **Closest first**
- **Level: low to high** or **Level: high to low**
- **Name: A to Z** or **Name: Z to A**
- **Most progress first**
- **Campaign first**
- **Quest log order**

Select one order with its radio button. The separate **Ready for turn-in first**
checkbox puts real quests showing `?` ahead of the others, then applies your
chosen order within each priority. Completed tasks, recipes and failed quests
do not count as turn-ins. With turn-in priority alone, ties follow quest-log
order. The menu stays open while selecting an order or toggling the checkbox.
Your scope, search, filters and completed-entry visibility still apply.

Quick orders sort across zones in a single list. **Level** puts quests with
known levels ahead of unknown levels, recipes and tasks in both directions.
**Most progress first** averages each objective's fulfilled/required ratio,
using completed/unfinished flags when counts are unavailable. Completed quests
have full progress; failed quests and entries without progress data follow
known values. **Campaign first** promotes quests marked as campaign by the
client, with quest-log order for ties. Without campaign data it follows log order.

Choices save per character. Version 1.0 migrates the old nearest-first checkbox
to the distance order and keeps the turn-in preference. **Restore saved grouping
and sort** clears the quick order and turn-in priority. Your original grouping,
collapsed sections and manual ranks remain intact. Choosing grouping/sorting in
Settings or moving an entry manually ends the quick view. The manager's entry
list keeps its alphabetical order.

Distances refresh once a second while distance sorting is active, outside combat
and tracker movement. Unknown and other-continent distances follow known ones
within each priority; recipes and personal tasks follow real quests. These are
distance estimates rather than a route planner. Sorting does not select a quest
or change your waypoint. Sorting preferences can be changed in combat; tracker
reflow waits until it ends. **Settings → Sort within groups** offers the same
name/level directions, progress and campaign modes, plus completion and manual
sorting, while keeping the selected grouping.

The **filter icon** opens a scrolling list with **Quest types**, **Zones** and **Custom
categories**. Checked choices are shown. Uncheck **Dungeon** to hide dungeon
quests, or uncheck zones and categories you want to hide. Choices use the native
quest tag names and zone headings in your current log, plus your personal tasks
and custom categories; Dungeon or Raid appears when the log contains that type.
Type, zone and category filters combine, and apply to the quick sort view too.
Each section has **All** and **None** buttons; **Reset filters** clears all
exclusions, including choices no longer present in the log. New types and zones
start included. Filters save per character and never untrack or delete entries.
The Quests & Tasks manager still lets you edit hidden entries. You can also open
the popup through **Settings → Quest filters...** and edit it during combat;
the tracker applies the new layout after combat.

The circular quest badges are navigation buttons. Click one to select that
quest with `C_SuperTrack.SetSuperTrackedQuestID`, the same native focus used by
the default tracker and waypoint addons. The selected badge changes appearance.
Click it again to clear navigation; Shift-click stops watching that quest.
Hovering explicitly enables mouse-motion handling on the badge and title and
uses both `WorldMapFrame:SetHighlightedQuestID` for quest areas and
`POIButtonHighlightManager` for POI icons. It does not select a navigation target
or require a click. Rows stay visible through data refreshes so a stationary
hover is retained. Leaving or hiding a widget clears only the highlight it owns,
including when moving between the title and badge for the same quest. A
particular map or minimap's visual response depends on its native providers.

## Tracked recipes

Recipes already tracked in Blizzard's profession UI are imported automatically,
including separately tracked recrafts. With zone grouping they appear under
**Professions**, and with type grouping under **Recipes**. They can be assigned
custom categories and sorted along with other entries. Having all reagents does
not mark a recipe as a completed quest or hide it through the completed-quest filter.

Required reagent lines show owned/required counts. Quality alternatives are
combined using Blizzard's native ownership helper when available; currencies and
item-cache updates are supported. Variable requirements are shown as ranges;
unavailable quantities remain explicitly unknown rather than guessed.

Click a recipe title to open it. Shift-click, or choose **Stop tracking recipe**
from its context menu, to untrack the corresponding craft or recraft. Importing
recipes does not rewrite the server tracking list. **Settings → Show tracked
recipes** controls inclusion in Lamda; switching it off leaves those recipes
with Blizzard's tracker.

Create categories on the Categories page, then choose one from an entry's
**⋮ → Change category** submenu. The task/category editor has a dropdown
including Uncategorized. With no categories it reads No categories yet and is
disabled; the adjacent + remains available. Creating a category selects it for
the draft. Save, Cancel and closing the category dialog return to the same task
fields. Settings selectors also open lists and mark the selected choice.
Removing a category keeps all its entries. Group headers remain alphabetical;
the sort choice controls entries within each group.

`/lql add`, `/lql options`, `/lql show`, `/lql hide`, `/lql restore`,
`/lql unlock`, `/lql lock`, `/lql resetposition` are available. Hiding the addon
restores Blizzard quest tracking. The optional key binding is under **Lamda
Quest Log**; the addon does not assign or overwrite keys.

## Waypoints and macros

Task waypoint commands accept one navigation line, for example:

```text
/way #84 52.3 61.4 Visit the quest giver
/way 52.3 61.4 Visit the quest giver
```

Coordinates without a map ID are resolved to the current map **when the task is
saved**, so changing zones does not silently move the destination. The zone field
is a grouping label; it does not resolve a waypoint's map ID. Use an explicit
`#mapID` when entering another zone's coordinates.

The **waypoint pin icon** first delegates to the installed `/way` or `/waypoint` handler, including
WaypointUI or TomTom. That also permits zone-name syntax supported by that addon.
Without a provider, numeric coordinates use `C_Map.SetUserWaypoint` and native
super-tracking if the client/map supports it. A provider's successful return
does not prove a pin was placed; it may report errors in chat. Context actions
also let you copy the waypoint into chat for editing.

For other macro commands, create the macro yourself in `/macro` and attach its
exact name to the task. **M** invokes that existing macro through a secure WoW
button. Renaming/deleting the macro requires updating the task. The addon does
not create, overwrite or execute arbitrary stored macro bodies or Lua.

Quest selection, watch-list changes, details, recipe actions and waypoint
navigation have no addon-level combat block. Selection artwork and the manager's
tracking state refresh immediately without rebinding secure buttons or moving
the tracker. WoW and external waypoint providers still enforce their own rules.

Tracker layout and scrolling are held during combat because the tracker contains
secure item/macro buttons. Wheel, scrollbar-arrow and thumb requests retain the
last requested offset and apply it after combat, clamped to the current list.
Management lists and Notes remain scrollable. Movement and resizing stop when combat
starts; the saved lock preference is preserved. Management windows remain open
and can be opened during combat. Settings, search, categories, task edits and
the lock preference can all be changed then. Background opacity applies
immediately. Font size, sorting, filters, showing/hiding the tracker and other
protected frame-layout changes apply after combat; the panel explains this.
The last rendered item/macro buttons remain available under WoW's normal rules.

Tracker movement uses cursor updates outside combat rather than native
`StartMoving`/`StartSizing`. This lets it stop at combat entry without calling
the protected `StopMovingOrSizing` method on the tracker during combat.

## Native tracker integration

On clients with Blizzard's module-container API, `QuestObjectiveTracker` and
`CampaignQuestObjectiveTracker` are temporarily removed from their native
container using `RemoveModule` and restored with `AddModule`.
`ProfessionsRecipeTracker` is also replaced when recipe data can be read and
recipe display is enabled. If the recipe APIs are unavailable or tracking-list
reads fail, the native recipe section remains available. No Blizzard methods
are replaced and no watch list is rewritten on import. Scenario, achievement,
dungeon, world-quest and other modules remain attached to Blizzard's tracker.
On an unknown UI, the addon leaves the native tracker available. The legacy
watch-frame fallback is for compatibility and is not a supported Classic release.

## Build and check

Run from this directory:

```sh
python3 -m venv .tmp/venv
.tmp/venv/bin/pip install lupa
.tmp/venv/bin/python tools/check.py
python3 tools/package.py
```

The check runner uses `lupa.lua51`, not a different Lua dialect. Tests exercise
data persistence, grouping/sorting, large lists, waypoint parsing/dispatch,
client API fallbacks and mocked frame lifecycle/combat deferral. They cannot
prove actual Blizzard protected-frame or rendering behavior.
Pointer tests resolve anchor rectangles, mouse targets and frame layers to
exercise header clicks with an overlapping native tracker, at both width limits
and during combat. Section tests cover collapse, font growth and row reuse.
Manager-menu tests cover native and fallback menus, direct category assignment,
quest tracking, recipe variants, task actions, scrolling, row reuse and combat.
Quick-sort tests cover every native-menu order, name/level directions, progress
counts, campaign priority, old-preference migration, turn-in combinations,
native/fallback selection marks, restored views and combat reflow.
Notes tests dispatch pointer clicks and typed input through focus ownership,
including newlines, editor reuse, scrolling/caret bounds and saving during combat.
Difficulty-color tests cover both views, native content/level API paths, scaling
metadata, player-level events, row reuse and missing or invalid API responses.
Combat interaction tests switch quest targets, check immediate selection artwork
and watch state in both views, exercise navigation/details/recipe/menu actions,
and verify secure bindings and geometry remain unchanged. Wheel and native
scrollbar tests check deferred offsets and clamping after the list changes.

The ZIP is written to `dist/` and contains only the addon runtime. This project
does not install into a client or modify SavedVariables during a source build.

## In-game acceptance checklist

Lewis requested Retail installation for testing on 2026-11-12, followed by a
Forever update. Both now have 1.0.0 with separate character settings. Do not stop
or restart either game through tooling. Use a user-managed
Reload UI, enable Lamda Quest Log in the AddOns list if needed, then open
`/lql options`. A newly installed addon may need the user's next client launch
to be discovered.

1. Confirm build/interface with `/dump GetBuildInfo()` and enable Lua errors.
2. Compare the native and replacement tracker at the same UI scale. Check font,
   text colors, header art, wrapping and spacing; `/lql restore` provides the
   comparison. Pixel-for-pixel parity is not established by source inspection.
3. Track enough quests/tasks to overflow; scroll to the final item, collapse
   groups and change scope/sort/search. Test long notes, long titles and resize.
   Compare level prefixes, incomplete `...` markers and turn-in `?` markers.
   Navigate all four sidebar pages, adjust both Appearance sliders, and hide
   then re-enable the tracker using only its management button.
   Open Add personal task and verify its opaque form is above the dimmed manager,
   input goes into the form, and closing it restores manager interaction. Check
   the header padlock switches immediately between open and closed artwork.
   Check each icon's tooltip and click target, drag only the title area, and use
   the chevron to collapse/expand while keeping the icon controls available.
   Check underlined section titles, collapse/expand them, and test larger fonts.
   Open Quick sort and test name/level in both directions, progress, campaign,
   distance and quest-log order. Combine each with Ready for turn-in first,
   then restore the saved view. Check the menu stays open while selecting,
   moving updates distance order, and only real turn-ins are promoted.
   Check unknown-distance quests follow known ones within each priority.
   Open Filters, toggle a native Dungeon type and several zones/categories,
   scroll through a long list, test All/None/Reset, and combine filters with Quick sort.
4. Create/rename/delete categories; assign live quests and personal tasks;
   manually reorder; complete tasks; manually `/reload` and verify persistence.
   Click the Notes text area and its empty padding, type several lines with
   Enter, scroll long notes, then save and reopen the task to check its text.
   Compare quest title/level difficulty colors with the native quest log and
   check they refresh after leveling or a scaling/effective-level change.
   Use manager **⋮** menus to pick categories directly, untrack/retrack a quest,
   open/untrack a recipe or recraft, and edit/complete/delete a personal task.
   Check menu placement, long category lists, outside-click/Escape dismissal,
   scrolling and combat transitions. Confirm no Category/Edit button remains on
   each manager row and the title has room beside the three dots.
5. Test a current-zone `/way`, explicit map ID, installed WaypointUI, provider
   errors and native navigation without a provider. Verify the actual map pin.
6. Attach a pre-existing harmless macro, then rename/delete it and inspect the
   missing-macro state. Check actual quest-item usage and cooldown feedback.
7. Enter combat while the tracker is visible and while dragging or editing.
   Check item/macro buttons, deferred changes and post-combat refresh, plus Lua
   errors, blocked actions and taint logs. Confirm drag/resize stops immediately,
   then open the manager, change settings and save a task during combat. Check
   opacity changes immediately, font/layout changes apply after combat, and the
   manager remains open. Switch between multiple quest badges mid-fight and
   verify navigation and selection update immediately. Toggle tracking, open
   quest details, use a saved waypoint and open/untrack a recipe. Check menus
   remain usable across combat transitions. Test tracker wheel, arrows and thumb
   for deferred scrolling without a blocked-action warning; the manager and
   Notes should still scroll normally during combat.
8. Check scenarios, achievements, world quests and dungeon objectives remain
   visible; restore Blizzard tracking and verify quest sections return.
   Track a recipe and recraft in the native profession UI, check their reagent
   counts in Lamda, change bags/currency, open a recipe and untrack each variant.
   Toggle recipe display off and restore the default tracker; check its recipe
   section returns. Click quest badges to move native navigation between quests
   and hover them to verify the client's actual POI highlighting.
9. Repeat on Retail with its separate settings, character data and addon stack.

## API provenance

Reviewed 2026-11-11. Blizzard-authored source mirrored by Gethe:
Combat quest/recipe interaction and protected scrolling sources were checked
again on 2026-11-14 for the 0.4.8 update.

- [Tracker module/container](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_ObjectiveTracker/Blizzard_ObjectiveTrackerContainer.lua)
- [Tracker artwork/header](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_ObjectiveTracker/Blizzard_ObjectiveTrackerModule.xml)
- [Native tracker filter artwork](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_ObjectiveTracker/Blizzard_ObjectiveTrackerContainer.xml)
- [Native waypoint pin artwork](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_SharedMapDataProviders/WaypointLocationDataProvider.xml)
- [Native settings, add and chevron artwork](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_SharedXML/Mainline/SharedUIPanelTemplates.xml)
- [Native menu implementation guide](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_Menu/11_0_0_MenuImplementationGuide.lua)
- [Native context-menu and radio utilities](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_Menu/MenuUtil.lua)
- [Native checkbox menus and refresh behavior](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_Menu/MenuTemplates.lua)
- [Tracker fonts](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_ObjectiveTracker/Blizzard_ObjectiveTrackerFonts.xml)
- [Quest tracker behavior](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_ObjectiveTracker/Blizzard_QuestObjectiveTracker.lua)
- [Quest API declarations](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/QuestLogDocumentation.lua)
- [Native quest difficulty palette and level helpers](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_FrameXMLUtil/Mainline/DifficultyUtil.lua)
- [Native quest-log difficulty colors](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_UIPanels_Game/Mainline/QuestMapFrame.lua)
- [Player quest content-difficulty API](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/PlayerInfoDocumentation.lua)
- [Secure item and named-macro actions](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_FrameXML/SecureTemplates.lua)
- [Default opacity and text-size behavior](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_ObjectiveTracker/Blizzard_ObjectiveTrackerManager.lua)
- [Protected frame movement methods](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/SimpleFrameAPIDocumentation.lua)
- [Protected scroll-frame methods](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/SimpleScrollFrameAPIDocumentation.lua)
- [Quest and waypoint super-tracking API](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/SuperTrackManagerDocumentation.lua)
- [Native edit-box focus and multiline methods](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/SimpleEditBoxAPIDocumentation.lua)
- [Native circular quest badge artwork](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_POIButton/POIButton.lua)
- [Native quest focus and hover behavior](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_POIButton/POIButton.lua)
- [Shared POI highlighting](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_POIButton/POIButtonHighlightManager.lua)
- [Native map-area hover handling](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_UIPanels_Game/Mainline/QuestMapFrame.lua)
- [Native recipe tracker](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_ObjectiveTracker/Blizzard_ProfessionsRecipeTracker.lua)
- [Recipe tracking API declarations](https://github.com/Gethe/wow-ui-source/blob/live/Interface/AddOns/Blizzard_APIDocumentationGenerated/TradeSkillUIDocumentation.lua)

Local read-only evidence: Forever's installed WaypointUI `/way` registration and
ForeverDungeonJournal quest API usage; vault setup record
`../../Setup/ForeverDungeonJournal-install-2026-11-06.json`. No installed addon
source was modified or copied into this addon.
