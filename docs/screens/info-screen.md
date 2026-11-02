# The info screen (INFO.C), screen 9

Go: `internal/game/info.go`, `info_history.go`, `info_tech.go`,
`info_racestats.go`, `info_turnsum.go`, `info_reference.go`. Addresses are
ELF. Reached with the INFO button (hot key I) of the galaxy screen; RETURN
(Esc) always goes back to screen 0. Ported: the whole screen with its five
pages. The page buttons, the list rows and the up/down buttons have no hot
keys (up/down carry Esc, which RETURN takes first), so only each page's
first state was compared with the original.

## Info_Screen_ (0x106cac)

`_cur_plyr_ptr` = the human; `Save_Palette` (never restored),
`Fast_Fade_Out_`, `_g_reference_screen_type` = 0, `_g_turn_message_ix` = 0
when `_return_screen` is 0. INFO.LBX 0..0x19 (0 background, 1 palette and
pointer, 2 RETURN, 3..7 page buttons, 8..0xb graph buttons, 0xc..0x13 the
list buttons (0xc/0xd up/down drawn disabled, 0xe/0xf up/down, 0x10..0x13
the tech review's four lists), 0x14 the race statistics' page button, 0x15
the reference's back button, 0x16..0x19 the maintenance column parts).
`Initialize_Maintenance_Values_`, `Build_Maint_Chart_Bitmaps_`, the palette
of INFO.LBX 1 (all 256, then darkened by `Turn_Off_Palettes_`), the pointer
from INFO.LBX 1. The page is bits 4..6 of the human's `InfoButtons`
(+0xe5f); five multi buttons at `_sub_scr_field` share it. Each pass:
`_active_player[0]` = human, `Get_Active_Players_(1, 0)`, black, outline
0xb0, `Tri_Interlaced_Fill` (0xd4,0x17)-(0x26c,0x1c9) in 0xb2/0xb3/0xb3,
the background, the stardate ("%i.%i", "%i,%i" for other languages) at
0x97,0x1b, the chart, fields above the page buttons cleared, the cycle
reset, style 5, the page's help entry copied into entry 2 of
`_info_help_list` (`History_Graph_Help_` and siblings) and the page run;
it returns the field that ended it (0 after a full redraw or a reference
choice). On RETURN the page goes back into `InfoButtons`, the fields are
cleared, the auto function is switched off and `_current_screen` = 0 —
also after the turn summary sent the player to a colony (quirk below).

`Introduce_Subscreen_` (0x109db5): faded out → clear palette changes,
flip, darken, `Fast_Fade_In_`; else `Blinds_Fade(0, 0, 0x27f, 0x1df, 3)`
(engine: three flips of every third row through the scan list).
`Draw_Drop_Area_` (0x109e01): nine interlaced fills of a 0x11-byte record
(x1, y1, x2, y2, nine colours) with cut corners.

## Maintenance chart

Values: gross income (+0xae) and the six maintenance parts (+0xb8..),
`(v*1000/max + 5)/10` of the largest (all zero when it is not positive);
the count of non-zero ones. Columns (0x18x0x6c): INFO.LBX 0x16 raised to
100-v, masked by 0x17 (inverse) at y 0x63, foot 0x19 at 0x66, colours 1..7
recoloured from `_maintenance_bar_colors[i]` (`Colorize_Bitmap_`); a zero
part has no column. Rows (0xa0x0x14): INFO.LBX 0x18 recoloured, text in
`_maintenance_text_colors[i]`; row 0 "<gross>" + text 0x13 centred at
0x5c (spacing 1 when wider than 0x7e, then style 2's spacing again), rows
1..6 "%3i%%" at 0x1b and texts 0x14..0x19 centred at 0x6c in colours
{0x14,0xa8,0xae,0xa2,0x97,0xb9}[i-1], 1,1,2,2,3,3,4. Drawn: column 0 at
0x1e,0xd4, the others right to left from ((n-1)·0x12-0x18)/2+0x78 every
0x12, rows at 0x19,0x158+0x10·i, texts 0x11/0x12, and text 0x1b with the
net income (gross - maintenance, red when negative) at 0x69,0xc4.

## History graph (History_Subscreen_ 0x107e95)

Title text 5; `Set_Up_Player_Colors_` (INFO.LBX 1's palette for 0xc0..0xff,
`_hist_graph_palettes[p]` = `_hist_palettes` + 8·banner colour);
`Get_Players_Alive_Or_Omniscient_(1, 0)`. Four radio buttons
(POPULATION, BUILDINGS, FLEET, TECH) on bits 0..3 of `InfoButtons` (all on
when none is). Names in columns of three (`Display_Graphed_Players_Names_`),
axes with ticks from 0x183d95 and eight stardate labels
(`Draw_The_History_Graph_`: from 3500 every 0.5/1/2.5/5 years by the
number of turns, later the last 35 years; returns the x step 10/5/2/1).
`Draw_Histories_`: per player and turn the sum of the chosen series times
their divisors (`bill_savegame` +4 population, +6 buildings, +8 fleet, +0xa
tech; the ring starts at +2 after 350 turns), smoothed ten times
(`Smooth_History_Data_`: sample i+1 = mean of i..i+2), scaled to the first
of 25, 50, 100, 250, 500, 1000, 2000, 4000, … above the largest (printed at
0xf6,0x8a), drawn as lines with a 0xb1 shadow one pixel lower right. A
graph button press stores the bits and redraws (over the old page).

## Tech review (_Tech_Review_Subscreen_ 0x108073)

Four lists (`_review_list`: pairs of id and flags, 4 = a BILLTEX2.LBX
heading, 8 = end) of the applications player p knows
(`Init_Tech_Review_List_`; items before the first heading would start at
slot i). The chosen list is a .data word (0x183e55, kept between visits);
four multi buttons pick it. The pointer's row becomes the choice when no
button is held (`Draw_Tech_Review_Subscreen_`); the choice's help text
(HELP.LBX record = application) and name go into two bitmaps, its
APP_PICS.LBX picture and palette are shown. Up/down page the list.
`Tech_Review_Subscreen_` sets `_race_report_jim_hack`, which in
`Display_List_Text_` gives items the human does not know colours 4 (only
the race report has such items).

## Race statistics (Race_Stats_Subscreen_ 0x1089c6)

`Get_Players_Dead_Or_Alive_Or_Omniscient_(1, 1)`; eight 0xb7x0xa4 bitmaps:
the race's name upper-cased and its picks (`Print_Player_Specials_To_Bitmap_`:
"^ " + government label or RACESTUF.LBX name, "%s%+i" levels, food
"-0.5"/"+1"/"+2", money "-"/"+" and "0.5 BC"/"1 BC"), or text 0xf. Four a
page at `_race_bitmap_x/y`; a button (no key) at 0x181,0x1ab flips the
page when more than four. Eliminated races get the "ELIMINATED" bitmap.
The help list is built per line (`Race_Statistics_Specials_Help_`) plus
three fixed entries. Quirk: slots past `_active_count` read whatever
`_active_player` still holds there.

## Turn summary (Turn_Summary_Subscreen_ 0x108f98)

Title text 8, text 0x1a with the year. The human's messages as a list
(`_g_turn_header`, `Init_Turn_Summary_List_`, style 4, line feed 10),
selected and paged to `_g_turn_message_ix`; the hovered one in
`_info_high_color`; up/down page. Without messages text 0x10. A colony
message clicked: `Goto_Msg_Colony_`, `_current_screen` 1,
`_return_screen` 9, and the page returns RETURN's field.

## Reference (Reference_Subscreen_ 0x109331)

Title text 9; by `_g_reference_screen_type`: 0 main, 1 category, 2 "how
to". Main: texts 0xa/0xb, the help categories (`_help_category_labels`
1..16 not starting with a digit) and the "how to" topics (HELP.LBX entry
0xf: name, record number) as "^ %s" with hidden fields; style 3 for
German/French. A pick sets the type and index and returns 0. Category:
text 0xc + the label; its topics (HELP.LBX entry = index: name, record)
bubble sorted by name (names and records swap, ids stay), a paged list;
the hovered topic's title (`Print_1_Or_More_Lines_Centered_To_Bitmap_`)
and text in a bitmap. How to: text 0xd + the record's title, its text.
Both have a back button (INFO.LBX 0x15).

## Quirks kept

- Leaving to a colony from the turn summary: `Goto_Msg_Colony_` sets
  screen 1, but `Info_Screen_` ends with `_current_screen` = 0, so the
  galaxy screen comes back.
- `Display_Graphed_Players_Names_`'s column step is uninitialised with
  three races or fewer; it is only used after the last name.
- Race statistics reads `_active_player` past `_active_count`.

## Check

`TestInfoPages` (build/oracle/info1..4, SAVE1 with the page set by
`saveedit player+0xe5f`) and step 0 of `TestRaceScreenStates`
(build/oracle/races, the history graph) match the original's captures;
the hovered list entry is skipped where its cycled colour 0xbf differs in
phase. `TestInfoScreenHelp` (build/oracle/infohelp, `build/oracle/infohelp.sh`,
`-clicks`) right-clicks the page area and both halves of the left panel,
then clicks each page button and right-clicks its area: every help box
(`Reference_Help_`, `Tech_Review_Help_`, `Race_Statistics_Help_`,
`Turn_Summary_Help_`, `History_Graph_Help_` and the two panel entries of
`_info_help_list`), the page buttons, the draws and the F10 save match. It
skips colour 0xbf and the help title's 0xbd: the title takes 0xbd or 0xbf
by the blink phase at the right click. `TestInfoScreenMouse`
(build/oracle/infomouse1, `build/oracle/infomouse1.sh`, `-clicks`): the tech
review's category buttons and list arrows, the history graph's four
buttons, race statistics' MORE twice, the turn summary's arrows, a
reference category with its arrows and another topic, BACK, a "how to"
topic, BACK, F10: every capture (colour 0xbf skipped), draw and the save
match. `TestInfoReferenceHelpMouse` (infomouse2): right clicks on the
reference page, inside a category (`Ref_Category_Help_`) and inside a "how
to" topic (`Ref_How_To_Help_`); Enter closes each help box and leaves the
subpage too. Not compared: a turn summary message click.
