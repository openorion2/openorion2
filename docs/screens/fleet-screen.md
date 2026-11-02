# The fleet screen (FLT1.C, FLT2.C), screen 4

Go: `internal/game/fleetscreen.go` (loop, fields, lists),
`fleetscreen_draw.go` (drawing, texts), `fleetscreen_actions.go` (scrap,
relocations). Addresses are ELF. The FLEETS button of the main screen
(hot key F) sets `_current_screen` 4, `_return_screen` 0.

## Fleet_Screen_ (0x73980)

Saves `_cur_map_scale` (restored on exit), sets the big icon grid
(`_big_icon_x/y_offset` 0x15b, 0x35, 4 columns, 5 rows, 20 icons),
`Reload_Fleet_Screen_` (FLEET.LBX 0..0x12: background, scroll thumb,
buttons, dimmed buttons, selection and scan frames; officer portraits
FLEET.LBX 0x2d+picture for the player's assigned ship officers; small ship
icons 0x13.. each loaded twice, normal and flashing, eight banners then
seven monsters; galaxy stars 0x22..0x2c into `_fleet_galaxy_star_seg`), the
auto function `Draw_Fleet_Screen_`, `N_Player_Ships_`,
`Set_Fltscrn_Small_Ship_Icon_XYs_(0xf, 0x34, 0x131, 0xb6)` (sets the map
scale to the maximum), `Set_Initial_Stack_Pointer_` (the fleet box's stack
if box 2 is open, else the player's first icon). The scroll bar is
initialised inline (5 visible rows, track 0xea, kind 0, step 1). Fade:
palette 8, the shown stack's ship palette (owner of big icon 0), darken,
draw, the SUPPORT/COMBAT radio pictures in frame 1, fade in.

Loop per pass: `_fleet_ship_selection_status` = −1, fields rebuilt,
Get_Input, Scan_Input, then in this order:

- a right click (negative input) in relocate mode 1 leaves the mode;
  Alt-F5 (−1105) clears all relocations (H 0x76); F5 (−1005) toggles the
  "all to" mode (2) with `_mouse_list_all_to` (FLT1.C's `_merging_relocations`
  at 0x19bed0, not MAINSCR.C's).
- the player's own stack: `Scan_Fltscrn_Big_Icons_`; hover sets
  `_scanned_big_ship`; right click opens `Detailed_View_Ship_`.
- `Scan_Galaxy_Map_Fields_`: hover sets `_galaxy_map_scanned_star`; a click
  merges all relocations to the star (H 0x77), or picks relocation stars
  (`Star_Relocation_`), or refuses a black hole (H 0x78, warning), or moves
  the selected ships (`Fltscrn_Move_Ships_`) when the first selected ship
  can be ordered, else H 0x75. Its "1" result (cancel a relocation) is
  dead code: the scan never returns 1.
- `Scan_Small_Ship_Icons_`: a click shows that stack (thumb to the top,
  big icons rebuilt), hover sets `_scanned_small_ship`.
- ALL (A) toggles every big icon (all on unless all are on); RETURN (Esc)
  leaves to screen 0 with `Move_From_Limbo_To_Pool_`; SCRAP (S): see below;
  LEADERS (L) goes to screen 0x1d with `_return_screen` 4 and
  `_screen_data[4]` = `_system_star_id`, or says H 0x79 without officers;
  up/down (−, +) scroll; RELOCATE (R) toggles mode 0/1; next/previous
  stack (., , or F2/F1, `Next_Ship_Icon_` / `Previous_Ship_Icon_`: same side,
  with ships passing the filter); SUPPORT (U) / COMBAT (C) radio buttons:
  turning both off turns the other on, a stack left empty moves on.
- `Set_Fleet_Screen_Help_List_`, `Update_Selection_Flags_` (selection count;
  the first two selected ships go to `_screen_data[3]`, `[4]`), draw unless
  a filter button was pressed. `_fix_palette` is never set, so
  `Fix_Palette_` never runs.

On leaving with the fleet box open its stack becomes the shown stack
(vanishing the box when there is none).

## Fields (Add_Fleet_Screen_Fields_ 0x7267d)

SCRAP (0x225, 0x17c) when icons are selected on an own stack outside
relocate mode 1, ALL (0x15c, 0x17c) likewise with icons, RETURN
(0x22c, 0x1ae), with ships the scroll buttons (more than 20 icons) and
previous/next stack (0x13/0x11b, 0xf9), a hidden RELOCATE field (none in
mode 2: the field number 0), LEADERS as a button or a hidden field without
officers, the small icon fields, the big icon fields
(`Fleet_Screen_Big_Icon_Fields_`: rebuilt when the stack or first row
changed; the scroll field when there is more than one row), the galaxy map
star fields (`Add_Galaxy_Map_Fields_2_`, 11×12 boxes), a debug field and
the two radio buttons. Hot keys: S A Esc − + , . R L U C.

`Build_Fltscrn_Big_Icon_Fields_` keeps the old selection by ship id, takes
the stack's ships with status < 3 that pass `Ok_To_Add_Ship_` (filter:
both on all, SUPPORT only colony/transport/outpost, COMBAT only combat
ships, neither none — `Get_Filter_Button_Status_` returns 0xffff, compared
unsigned), and moves `_fleet_icon_ship_id_to_move` to the front. That
global starts at 0 with the program: the first visit moves ship 0 to the
front of its stack if shown.

## Drawing (Draw_Fleet_Screen_ 0x72f92)

To the back page when asked: background and small icons (last first).
Then the hovered star's text (`Print_Fltscrn_Scanned_Star_Name_`: name,
governor and ETA, or what a move there would do, fitted into 0xbd pixels
at y 0xf8, owner colours unless ships are selected), the hovered big
icon's ship (`Print_Scanned_Ship_Data_` in the window 0xf..0x140 ×
0x11a..0x1d1: help text for support ships, else name, crew, shield, attack
and defence, captain portrait (`Draw_Captains_Picture_`), destination,
weapons with arcs, specials, damaged ones red), relocation lines, ship
destination lines (green own, red enemies heading for our colonies, plus
the shown stack to the hovered star), the galaxy map in mode 1 (stars
from `_fleet_galaxy_star_seg` at −2, and `Do_Fleet_Screen_Stuff_` boxes
while a star is hovered), drag-selection painting of big icons with the
button held, the hovered small icon's text (`Print_Galmap_Scanned_Ship_`),
the flashing stack icon, the scroll bar (`Fill_FltScrn_Scroll_Bar_`), the
big icons (frames, pictures centred in 0x39, damage bars) and the dimmed
buttons.

## Scrap (Scrap_Ships_ 0x74afe)

The selected ships; away from an own colony a confirmation (H 0x7a..0x7d
by count and vowel); for each officer aboard a confirmation (H 0x7e); the
value (a quarter of the cost at an own colony, over the selected icons)
with a confirmation (H 0x80/0x81); any no gives H 0x7f. At a colony the BC
are paid and captured ships may yield technology
(`Get_Technology_From_Scrapped_Ship_`: one of the unknown fuel, drive,
computer, armour, weapon and special applications for certain, each other
with 25−5(k−1)% — RNG draws — shown in a type 2 box titled with the ship).
Officers return to the pool, the ships die, colonies are recalculated.

## Check

`TestFleetScreen` (SAVE1 as SAVE10 in `build/oracle/fleets`): the screen
after FLEETS, two next-stack presses, ALL, SUPPORT off, COMBAT off; every
frame matches the original's capture outside the galaxy map (destination
line phases and the flashing icon animate). `TestFleetScreenScrap`: ALL,
SCRAP, the confirmation box and the screen after YES match, and F10 then
writes the original's SAVE10.GAM. `TestFleetScreenScrapAtColony` does the
same for SAVE4 (at a colony: value box, BC credited) and SAVE7 (an officer
aboard: his box first) in `build/oracle/scrap4`, `scrap7`.

`TestFleetScreenMouse` (build/oracle/fleetmouse1, `-clicks -clickkeep`): the
big icon and its hover text, a star of the map, the stack arrows, RELOCATE
and a star, RETURN, F10: captures, draws and the save match (no order
results: the colony ship stays). `TestFleetScreenMoveOrder`
(build/oracle/fleetmove1): the next stack arrow three times (ship 19's
stack, which `Player_Can_Order_Ship_` allows), ALL, the star at (167, 125)
(star 13): `Fltscrn_Move_Ships_` sends ships 18 and 19 there (status 2,
location 1013, ETA 2); captures, draws and the F10 save match. Not tested
against the original: big icon drags, the detailed ship view by right click
(rename1 covers it from the galaxy screen); nor
the technology of a captured ship (no save scraps one at a colony).
