# The galaxy map popups of the begin-of-turn reports (MAINPUPS.C)

Popups the report checks show over the galaxy map (or the mini main screen
during the reports): officer gains a level, officer for hire, new system
discovery, and the spy report message boxes. Addresses are ELF (Watcom +
0x10000). Go: `internal/game/mainpups.go`, `reports_popups.go`.

## The framework

| Address | Name | Go (`g.mp`) |
|---|---|---|
| 0x1831c0 | `_autofunction_ctr` | `autofunctionCtr` |
| 0x1831c2 / 0x1a11f8 | `_num_field_saves` / `_stats_array` (0x12 each) | `fieldSaves` |
| 0x1a11c0 / 0x1a11bc | `_eric_saved_autofunction` / `…2` | `saved`, `saved2` |
| 0x182aca | `_full_draw` | `fullDraw` |
| 0x1831c4 | `_explode_flag` (1 in .data) | `explodeOff` (inverted) |
| 0x1831b6 | `_no_flicker_between_mainpups_flag` | `noFlicker` |
| 0x1a123e | `_g_refresh_mode` | `refreshMode` |
| 0x1a123a / 0x1a123c | `_g_x`, `_g_y` | `x`, `y` |
| 0x19f82c | `_sys_disp_star_handle` | `sysDispStar` |
| 0x1831c6 / 0x1831c8 | `_g_officer`, `_officer_gains_level_state` | `officer`, `levelState` |

**Eric_Push_Fields_** (0xc6e7e) saves the field state (`Save_Field_Stats_`)
on a stack; **Do_Popup_Cleanup_** (0xc7283) pops it, deactivates the auto
function and puts back the saved one (ctr 1 → saved, 2 → saved2, ctr−1),
`Copy_Back_To_Off`, `_FULL_DRAW_SCREEN_FLAG = 1`, `Pop_Block`,
`Set_Refresh_Mode_(saved mode)`, `_draw_system_display_without_button = 0`,
explode flag 1, `Deactivate_Help_List`, `_need_to_restore_help_list = 1`.

**Do_Popup_Setup_(star, bg)** (0xc702e): off the galaxy screen (screen ≠ 0)
the explode flag is cleared. Save the refresh mode, stencil mode; `_g_x =
320 − w/2`, `_g_y = 240 − h/2` of bg; star −1 = the star of the human's home
planet (0 without one); clear the mouse buffer, input delay 3, arrow, reset
window, clip off, off → back, page off. With the explode flag and the star
on the map, **`Center_Map_` on that star** (this moves the saved map
position). ctr 0: save and run the auto function (the screen behind), and
unless the star is on screen with explode set (or no-flicker), flip, full
draw, run it again; ctr 1: save the second one and run both; ctr+1. With
explode: `Draw_Visible_Fields`, and with settings+4 and no field save
`Draw_Exploding_Plasma_Box_` (not ported; the traced settings have it off).
No-flicker cleared, full draw set.

**Mainpups_Tested_Execute_Autofunction_** (0xc7f84): ctr 1 → full-draw
flags, `_dont_draw_main_screen_to_back = 1`, run saved; ctr 2 → both; then
`_FULL_DRAW_SCREEN_FLAG` and `_dont_draw_main_screen_to_back` 0.

## Officer_Gains_Level_Popup_(loc, l) (0xc8209)

Style 1 shadow-down in the shades at 0xc6e2e; `_g_officer = l`; MAINPUPS.LBX
0x33 (the frame), the portrait (`Load_Officer_Picture_` 0x925ae:
`_officer_lbx_name` entry 0x15 + picture; 0xd2 / 0x115 for the smaller
sets), the 27 skill icons (entries 0x58..). Setup at loc; auto function the
draw; push fields; one hidden field over the screen (hot key E 0xc = "", so
a click or Enter); help list 0x183538 (`Set_Officer_Information_Popup_
Help_List_` 0xcdc2d: one entry, help 0x234 over the whole screen). Draw, flip; loop:
`Get_Input`, `Scan_Input`, draw, flip, until a field. Cleanup.

**Draw_Officer_Gains_Level_Popup_** (0xc8020) draws only with full draw: the
screen behind, the frame at (_g_x, _g_y), the officer panel
(`Wrapper_For_Colony_Officer_Popup_Data_` at y+0x35), the portrait at
(+0xd, +0x37), the text E 1 "%s gains a level" (state 1: E 0x248, rescued
and joins; 2: E 0x249, no room) of `Leader_Name_(l, 0)` as
`Print_Paragraph_Centered_(x+0xe, y+0x95, 0x119, 0x3e)`, the name with
title at (x+0xb, y+0xc, 0x11d, 0x21); off → back; full draw off.

**Print_Officer_Data_** (0x95baa), outside the officer screen, as the
wrapper calls it (−1, l, x+0x5f, y+5; portrait box x+0xa, y+2, 0x50, 0x5a;
width 0xc3): outline colour 0; the ETA over the portrait for assigned
officers in transit (`Print_ETA_On_Officer_Picture_`, not ported); style 3
outline in `_normal_text_colors` (0x182284), spacing 1; then for each row of
`_skill_data` (0x17d218, 0x12-byte rows, 0x36 rows: kind +5, name +0xa — an
E string pointer `Load_H_And_Pro_File_Strings_` sets, found by reading its
code —, format +0xe): first the special skills of the officer's kind (1 ship
officer, 2 colony leader), then the general ones (kind 0): icon `[row/2]` at
(x+2, y−4), name at x+0x18, `Officer_Skill_Bonus_` in the row's format
right-aligned at x+width+3, spacing 1 again, y += 0x11.

**Leader_Name_(l, title)** (0x97588): the level name (E 0x308+level for ship
officers, 0x30e+level for colony leaders; the level from experience for
unowned officers, at most 5) + " " + name; with title and a title: H 0x110
", the " (0x182 for leaders 10, 0x16, 0x22, 0x25, 0x27, 0x3c; French H 0x186
before a vowel; Italian `Get_Italian_The_`, not ported) + title.

## Random_New_Officer_Popup_(p) (0xc7ada)

The popup shows over the star of `Player_Capitol_World_(p)` (0xc5fb0: the
first colony of p with a capitol, else `Pick_Random_Colony_` 0xc5f5c, one
`Random_(n)` per colony of p) — the original takes the colony index as a
**planet** index. State (`_field_state`, 0x1a1238): 0 offer, 1 rejected, 2
too poor (BC < cost at the start), 3 hired. MAINPUPS 0x30 frame, 0x31 / 0x32
the buttons. Fields (`Add_Random_New_Officer_Fields_` 0xc754a): the left
button at (x+0x2c, y+0xe2) (hot key "r", later "n"), the right one at
(x+0xaf, y+0xe2) while asking or after hiring ("h", later "y"), a hidden
field over the screen except while asking, and the skill lines' hidden help
fields (`Add_Skill_Description_Help_Fields_` 0x92a92: x+0x61..+0xbe, up to
five lines from y+0x36; a right click explains the skill:
`Wrapper_For_Officer_Popup_Help_Fields_` 0x9733e loads the skill texts and
calls `Print_Officer_Skill_Help_(skill, Officer_Skill_Bonus_, l)`). Loop: hire → state 3 (2 when p's BC
< `Calculate_Officer_Cost_(PLAYER_NUM, l)`), fields rebuilt, and with
settings+0xd4 < 2 the popup stays to show the answer; reject → state 1;
right clicks on the buttons show a text box (0xc7dab: `sprintf(E(0x1fb)`
for the left button, `E(0x1fa)` for the right, `_officer_level_names[rank]`,
the name, the price)`, `Text_Box_` titled E(0x1bf) / E(0x141), the auto
function back, the fields popped, pushed and added again, full draw); in
states 1..3 a field ends it. Oracle: `build/oracle/hirehelp` (s4's turn with
F11 right clicks on Reject and the skill line, then Hire;
`TestOfficerPopupHelp`: captures, draws and the F10 save). Returns whether hired.

**Draw_Random_New_Officer_Popup_** (0xc76fa): with full draw (which it never
clears): `_draw_main_screen_to_back = 1`, the screen behind, frame, panel,
portrait, then by state E 2 (name, cost, `Officer_Maintenance_`, techs —
`Leader_Technology_String_` 0xc760e: E 0xd0, ", " separators and E 7 per
tech; for leader 0x2f the government label E(`_government_labels`[gov+1])
when the government pick is even; E 0xc when empty), E 6, E 5, E 4 (name,
cost) through `Squeeze_Paragraph_Centered_(x+0xe, y+0x95, 0x119, 0x3e, type
2)`; the title; off → back.

**_Squeeze_Print_Paragraph_** (0x102fd8): while the paragraph is taller than
the box, narrow ' ' down to `_space_limits`[style] (0x183684), then the
vertical spacing down to 2, then take the next smaller style; print centred
on y; restore the space width, the spacings and (if changed) the style and
shadow.

## New_System_Discovery_Popup_(star, special) (0xc8556)

Style 1 shadow-down; the special's picture and frame reload on use; frame
BUFFER0.LBX 0x49 (the system display box); setup; off → back; auto function
the draw; push fields; `Clear_Sys_Disp_Fields_`; `Add_Discovery_Fields_`
(`_draw_system_display_without_button = 1`; the system display's fields
through `Add_System_Display_Popup_Fields_(star, _g_x, _g_y, 0, 0, 0xd, 0x2f)`
without a close button — the sizes are box 0's — then the full-screen
hidden field). Loop with `Release_Time(2)`: special 0 →
`Evaluate_Discovery_Input_` (the system display's ship and icon hover);
special 8 → the marooned officer at the star joins
(`Marooned_Hero_At_Star_Can_Join_`, state 1) or not (state 2) in
`Officer_Gains_Level_Popup_`; any other special: the first field removes it.
Ends on a field once no special is left.

**Draw_Discovery_Popup_** (0xc83c5): with full draw the screen behind, off →
back, flags off; title E 0x1c8 with the star's name;
`Draw_System_Display_Popup_(star, 1, title, _g_x, _g_y, 0, 0, 0xd, 0x2f)`;
with a special other than 0 and 8 `Draw_Special_Info_Popup_(x+6, y+0x4c,
picture, name, text)`: frame MAINPUPS 0, picture MAINPUPS special+4
(monsters by `Star_To_Monster_`, not ported), name E(0x18f9b8[special]),
text record 0 of entry special in the file E 0x170 names (at most 0x258
bytes). Drawn: black fill (x+3, y+3)..(x+w−3, y+h−3), frame, picture at +4,
name style 1 (shades 0xc6e1e) centred at (x+0xde, y+0xa), text
`_Print_Paragraph_(x+0x78, y+0x17, 0xcd, type 2)`.

`Compute_AI_Data_` clears every star's +0x34 each turn; the bit comes from
ships reaching stars their owner had not visited
(`Move_All_Ships_Toward_Stars_`).

## sprintf_report_ (0xfd69f)

The report formats are the 8 strings after the first 169 of RSTRING<lang>.LBX
entry 0 (`Load_Report_Strings_` 0xfd5ee, chained after the Colcalc,
Combfind, Msg, Netmox, Netstart and Netturn loaders: 60 + 10 + 80 + 8 + 9 +
2), in the pointer order 0x1ab12c, 0x144, 0x148, 0x13c, 0x130, 0x134, 0x138,
0x140. Type 1 (p's spies): +3 = 0 tech (s16 +6) from race +5, +3 = 1
building +7 of race +5 on planet +6; then with a blame (s8 +4 ≠ −1) "  You
framed the %s.". Type 2 (against p): race (s16 +4) and tech, or race,
building and planet. Other types leave the buffer as it was.

## Checks against the original

`internal/game/turn_e2e_test.go` plays each turn through the port's screens
(frames with `OPENORION2_E2E_SHOTS=dir`); captures of the original are its F12 captures, the after-save from F10 at the end of the key list.

| Test | Popup | Result |
|---|---|---|
| TestTurnS2OfficerLevel | officer level (Claw) | identical but the black holes' animation phase |
| TestTurnS4OfficerForHire | offer, Hire, the answer | both popup frames 0 pixels off; SAVE10 = the original's save |
| TestTurnDisc1Discovery | discovery with special 4, then without (build/turns/inputs/disc1.GAM: SAVE1 with Zuban unvisited, special 4, scouts arriving) | identical but black holes and rotating planets; all 1521 draws; SAVE10 = the original's save |

## Tax rate

**Tax_Rate_Popup_** (0xcc20b), from the treasury window (0x86f8e: fields
cleared, the mini main screen as auto function, drawn to both pages, flip;
after it `Do_Player_Colony_Calculations_(PLAYER_NUM)`, the main screen
again): the rate (player +0x31) into 0x1a122e, `_last_scanned_field` −1,
style 1 shadow down in the 8 colours at 0xc6e6e, `Load_Tax_Rate_Popup_`
(MAINPUPS 0x3e..0x43 the six rate buttons, 0x44 OK, 0x45 the frame),
`Do_Popup_Setup_(−1, frame)`, help list 0x183510 (one entry),
`Draw_Tax_Rate_Popup_` as auto function, `Eric_Push_Fields_`,
`Add_Tax_Rate_Fields_` (OK at 0xed, 0xfe, hot key E(0x87); multi buttons
at x 0x93, 0xc1, 0xea, 0x113, 0x13c, 0x165, y 0xc6 for 0..50%, hot keys
E(0x8e, 0x93, 0x95, 0x97, 0x99, 0x9b)). Loop: `Get_Input_`, `Scan_Input_`,
`Evaluate_Tax_Rate_Input_` (OK stores the rate and ends; any other field
stores it, recomputes the colonies and redraws), the frame redrawn when
full draw is on, flip. `Do_Popup_Cleanup_`. Ported (taxrate.go).
Checked against the original by clicks (the main screen empties the mouse buffer at the end of every pass, see
docs/engine/fields-and-input.md): the treasury, 30%, ACCEPT;
`TestMainScreenPopups` matches every capture, draw and the F10 save.

## Dragon (Dragon_New_Star_Popup_, 0xcc735)

Called from `Search_For_Battles_` when the Dragon (owner 12) attacks a
human's colony; the answer is −1 (no bribe) or the star to send it to
(`Do_Dragon_Bribe_`: 500 BC). Player → 0x1831b0, state 0x1831cc = 0,
0x1a1240 = −1, `Set_Font_Style_Shadow_Down_(1, 0xc6e76)`.
`Load_Dragon_New_Star_Popup_`: MAINPUPS 0x46 (CANCEL), 0x47 (ACCEPT), 0x48
(backdrop), `Load_Galaxy_Map_Anims_`; `Do_Popup_Setup_(star, backdrop)`,
auto function the draw, help list 0x18351a, `Eric_Push_Fields_`,
`Clear_Galaxy_Map_Fields_`, the fields (CANCEL 0xba,0x139 hot key E 0x87;
ACCEPT 0x17d,0x139 hot key E 0x231 only in state 0, else −1000; both
labelled E 0xc, sound 0x28; `Add_Galaxy_Map_Fields_2_(0x12e, 0x9c, 0x94,
0x5e)`), draw, flip, differential refresh; then `Get_Input_`,
`Scan_Input_`, `Evaluate_Dragon_New_Star_Input_`, draw, flip until done;
`Do_Popup_Cleanup_`, help off, 0x1831b0 = 0x1831ac = −1.

Evaluate: the hover (0x1831ce) = −1; CANCEL → −1, done; ACCEPT → state 1
if the player has 500 BC, else 2, full draw; in state 1
`Scan_Galaxy_Map_Fields_`: a hover (4) sets the hover star, a click (0)
on another star ends with it, on the attacked star `Help_` (=
`Message_Box_`) E 0x240 and a full draw.

Draw: a changed hover forces a full draw; then (full draw or the screen
flag) the background through `Mainpups_Tested_Execute_Autofunction_`,
the backdrop at (0, 0), `Draw_Galaxy_Map_Box_(C_Anims_(0xb), …, 0x12e,
0x9c, 0x94, 0x5e, 0…)` — the C_Anims_ slot is loaded only by the colony
screen; null gives the black-filled box; ESI is 0x…6e7e from the colour
copy — and the offer `Squeeze_Formatted_Paragraph_Centered_(0xbc, 0x10f,
0x106, 0x23, …, 2)`: state 1 E 0x1d0 (emperor), 2 E 0x247 (500, emperor,
star), 0 E 0x22b (the same); `Copy_Off_To_Back_`. Always the hovered
star's name `Print_Centered_(0x178, 0xfe)`. Oracle: drag1
(`build/turns/drag1.sh`, SAVE5 with a Dragon moved to Meklon), Esc =
CANCEL: `TestTurnDrag1DragonBribe` matches the popup, the draws and step
saves up to the attacked popup. drag2 (`build/turns/drag2.sh`) accepts:
ACCEPT, the prompt E 0x1d0, a click on Goi; `Do_Dragon_Bribe_` sends the
Dragon (departing, location 1000 + star) and takes 500 BC.
`TestTurnDrag2DragonAccepted` matches the popup, every draw and the F10
save. `Remove_Non_Detected_Ships_` then tests the visited bit of star 1010
for the departing monster without a drive: the original reads far past
the star array, 0x1b8f3 bytes past its start. drag2m (a memory dump of the original: the stars and ships pointers and
0x30000 bytes from 0x10000 below the stars at the first such test) shows
where: the DOS blocks of `Allocate_Data_Space_` are the stars (0x171bc),
the ships (0x191ac) and 0xf71 bytes after them, ending near 0x29d30; the
read lands at 0x32fc1 in free DOS memory, which is zero from there up to
the end of the dump. So the bit is clear and the icon is hidden, as in the
port; `TestTurnDrag2DragonAccepted` checks that no stack holds the Dragon
and that the map between Meklon and Goi matches the original's capture.
(Under real DOS with resident drivers above the game the byte could differ.)

## Port-only: the officer popups' pace

`Random_New_Officer_Popup_` (0xc7ada, the hire popup, also over the
leaders screen) and `Officer_Gains_Level_Popup_` (0xc8209) loop on
`Get_Input_` and `_TOGGLE_PAGES_` with no `Release_Time_`: a pass as fast
as the machine drew one. Each pass draws the system display under the
popup, whose planets turn a frame a draw; on a DOS machine that was about a
timer tick, here a 60th of a second, and the planets spun three times as
fast as on the leaders screen (`Officers_Screen_`: `Release_Time_(1)`). The
port waits for a tick a pass (`paceTicks`), as the discovery, colony and
defence popups do with their own `Release_Time_(2)`. `TestHirePopupPace`.

