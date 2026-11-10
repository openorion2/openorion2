# The leaders screen (OFFICER.C), screen 0x1d

Go: `internal/game/leaders.go`, with `printOfficerData` and
`printETAOnOfficerPicture` in `mainpups.go`. Addresses are ELF. Reached
from the main screen's LEADERS button (hot key L, `_return_screen` 0) or
the fleet screen's LEADERS (`_return_screen` 4); the screen returns to
`_previous_screen`.

## Entry from the main screen

Main_Screen_ sets `_screen_data[4]` before switching: on the colony page
`Get_Star_Id_For_Officers_Screen_` (the system display's star with an own
colony, else the first colony star from the capitol's), on the ship page
the fleet box's stack, else `Get_Ship_Stack_For_Officers_Screen_`, which
returns the first own stack's *node*, not its index (only read when
coming from the colony screen). `_officer_screen_return_screen` = 0.

## Officers_Screen_ (0x94c1d)

`_officer_scrn_type` (1 after Harold_Defaults_: colony leaders; −1 → 0).
From the colony screen (1) the page is colonies and `_screen_data[4]` the
stack. `Reload_Officer_Screen_Pictures_`: the fleet screen's tables again,
`_prev_leaders` (status, location, ETA of every leader; colony leaders in
the pool or offered are put at the capitol's star), `Build_Captain_Id_List_`
(up to four of the page's kind: hired or offered), OFFICER.LBX 0..0x14 and
0x8d, 0x8e, portraits (0x15+picture) and darkened ones (0x8f+picture),
skill names and descriptions (SKILDESC.LBX or the language's file),
skill icons 0x58.., small ship icons 0x73.. and galaxy stars 0x82... Then
the ship icons on the leaders map (0x132, 0xeb, 0x13e×0xa9); the ship page
takes the first own stack, the colony page initialises the system display
of `_screen_data[4]` (chosen star: the capitol's if −1). Big icon grid
0x12e, 0x13, 5×3; scroll bar 3 rows, track 0x79, kind 2. The leaders'
locations are kept twice (`_officers_original_star_id`, `_ship_id`).
`Fade_Into_Leaders_Screen_`: palette 8 and the stack owner's ship palette.

Each pass: CANCEL HIRE (X) leaves hire mode; HIRE (H) hires the selected
leader or toggles hire mode (cursor `_mouse_list_who`);
`Check_Officer_Fields_`; with no mode: small icon clicks show a stack (ship
page: pictures reloaded), big icons (`Scan_Offscrn_Big_Icon_Fields_`: a
click selects a combat ship — H 0x12c otherwise — and assigns the selected
officer, H 0x12d if he cannot be; a right click shows the ship), map stars
(colony page: a click on an own colony star shows it, a second click
unchooses), the system display (a click on a colonised planet with
`_officer_screen_return_screen` 0x27 says H 0x12e), next/previous (.,):
stacks (`Cycle_Ship_Icons_`) or colony stars (`Cycle_Star_Icons_`);
DISMISS (D) and POOL (P) toggle their modes (POOL with a selected leader
moves him to the pool at once); scroll −/+; the page buttons C and S
(`Change_Screen_Type_`); RETURN (Esc). On leaving: limbo leaders back to
the pool, colony recalculation of stars a governor left, the player's
colonies recalculated, `_prev_leaders` and `_scroll_bar` cleared.

`Check_Officer_Fields_` (with `Scan_Officer_Fields_`: skill lines first,
then the name and portrait fields; hover sets `_officer_scanned`): a right
click on a skill line shows its description (`Print_Officer_Skill_Help_`),
on a portrait where the leader is (`Find_Selected_Leader_`: H 0x111..0x116,
the ship's stack or star becomes the shown one, a timed box of 15 ticks
unless difficulty 0). A click selects; then by mode: hire
(`Do_Hire_Officer_`: H 0x122 already hired, H 0x123 too poor, else the hire
popup: `Ask_For_Hire_` (0x92d72) formats H 0x124 (maintenance 1) or 0x125
with the name, `Get_Officer_Costs_`' cost and maintenance, and
`Hire_Officer_Popup_` (0xc7ab8) runs `Random_New_Officer_Popup_` with
`_on_officer_screen_flag`: MAINPUPS 0x39..0x3b, no skill icon reload, that
text in `Print_Paragraph_Centered_` (x+0xe, y+0x95, 0x119×0x3e); oracle
hire2, `TestOfficerScreenHire`), dismiss (confirmation H 0x126, `Dismiss_Officer_`; community patch 1.51:
it marks the leader for the next network upload, `docs/net/turn.md`), pool
(`Move_Officer_To_Limbo_`: status 2, H 299 for offered leaders), or assign
to the chosen ship or star (`Assign_Captain_`: confirmation H 0x119 / 0x118
with the ETA from `Get_Ship_Leader_ETA_` / `Get_Star_Leader_ETA_`, which
judge by `_prev_leaders` while the screen runs; replacing a leader asks
H 0x11a/0x11b).

## Drawing (Draw_Officer_Screen_ 0x93550)

To the back page when asked: the four portraits (darkened while
travelling or offered), the screen picture, the fleet or colony box. Then
the scroll bar and big icons (ship page) or the system display (colony
page, `Draw_System_Display_Popup_` at 0x132, 0x11 with the inner margins
3, 3 instead of 0xd, 0x2f, no border, the sun at the fixed 0x1c0, 0x5c),
the galaxy map in mode 1 with flag 0 (the map window is cleared with box
1's size) and `Do_Officer_Screen_Stuff_` (hovered star framed and named at
0x1d2, 0x1a2; the shown star framed, flashing `_offscrn_flash_value`+0x6e
when chosen), destination lines, small icons, hover texts, each row's
`Print_Officer_Data_` (name, cost or upkeep with the monetary unit, the
status line under the portrait, ETA bars, skills; the scanned or selected
row bright), the mode buttons, the hire cost in hire mode, the shown star
with its governor, the page buttons (with `_global_var` 'c' a quirk draws
one pressed), the flash counters.

Community patch 1.51: `Print_Officer_Data_`'s Print_ of the cost or
upkeep label (0x95e3e) goes through the patch (0x404b22: the label's y
and the leader, then 0x40e6a6), which prints the leader's experience
(+0x24, u16) over the next level's threshold right-aligned at (293, 91 +
the label's y) in the label's font: 60, 150, 300, 500 for
`Owned_Officer_Level_` 0..3, 1000 above (the patch's table at 0x45cd0f).
Its template is "$XP$/$NEXTLVL$ EP"; the port takes the bracketed part of
KENTEXT 0x1f (the crew experience, "Green Crew (%d EP)", per language: "%d
EP", "%d PE", "Esp. %d") with "%d/%d" for its "%d", so no new text. Only
the full leaders screen shows it (not the hire popup). Tests:
`TestOfficerExperience`, `TestLeadersScreenExperience`.

## Check

`TestLeadersScreen` (SAVE1 as SAVE10 in `build/oracle/fleets`): the colony
page after LEADERS and the ship page after S match the original's
captures but for the animations (planet rotation, line phases, the
flashing box).

`TestLeadersScreenMouse` (build/oracle/leadmouse1, `build/oracle/leadmouse1.sh`,
`-clicks -clickkeep`: the screen rebuilds its fields every pass): a
governor's portrait assigns him to the shown colony (the ETA box, y; the
governor there goes to the pool, y), POOL, a skill row, Ship Officers, HIRE
mode, RETURN, F10: captures (the maps and system display animate), draws
and the save match. A confirmation box under `-clickkeep` takes keys, not
clicks: the kept click goes back to the screen and asks again.
`TestLeadersHireDismissMouse` (leadmouse2): HIRE on the ship page, an
officer the treasury cannot pay for, CANCEL, DISMISS of a governor (y).
Quirk kept: `Not_Enough_BCs_Message_` (0x92cf8) formats H 0x123 with the
BC, the name and EDX, which `Do_Hire_Officer_` left holding the cost (the
port had dropped the third value and printed Go's "%!d(MISSING)"). The hire popup's
officer-screen variant (`Load_New_Officer_Popup_For_Officer_Screen_`) is
not ported.

## Port-only: the map's stars all take the pointer

`Add_System_Display_Popup_Fields_` (0xa1c74) ends with a hidden field over
the whole box, box 0's size (347 × 273). The leaders screen draws its
system display at 0x132, 0x11 without the box and discards that field; a
field of that size there lies over the map's top part, added before the
map's stars, so those stars never take the pointer. The port adds no
whole-box field on this screen (`sd.offscreen`). `TestLeadersMapStars`.

The call reads (y, x)-(y + width, x + height) in the disassembly
(0xa1fda..0xa1ffd); the port keeps the box's own rectangle, which the
main screen needs (stars beside the open system box take clicks): the
swapped one covered them. Not settled against the original.
