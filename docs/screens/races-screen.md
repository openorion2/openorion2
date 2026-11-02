# The races screen (RACESCRN.C), screen 6, and the race report (RACERPRT.C)

Go: `internal/game/race.go`, `race_report.go`. Addresses are ELF. Reached
with the RACES button (hot key R) of the galaxy screen; RETURN (Esc) goes
back to screen 0.

## Race_Screen_ (0x10acba)

Saves the auto function and palette, loads the texts (BILLTEXT.LBX
0x21..0x31 the 17 relation names, 0x32 "Ignoring", 0x33 "no contact",
0x34/0x35 the bonus labels, 0x39/0x3a research and trade, 0x3b/0x3c
tribute given and received, 0x4c/0x4d network waits, BILLTEX2.LBX 0x1a the
declaration of war), fades out. Then the restart point (0x10b80b), taken
again after every action on a race and on space in who mode:
`_cur_plyr_ptr` = human, `Races_Help_` (8 entries), fields cleared, the
pointer's 0xbb x 0x19 bitmap, RACES.LBX 0 (background), 1 (arrow), 4
(pointer), the who pointer by language (5; 0x3d German, 0x3e French, 0x3f
Italian), 0x3b (net box). `Get_Players_Dead_Or_Alive_Or_Omniscient_(0, 0)`,
`Init_Race_Display_Data_`, the pointer's icon group, background,
`Turn_Off_Palettes_` when faded, outline 0xb1, `Draw_Race_Photos_`,
`Draw_Race_Text_`, `Get_Active_Players_(0, 0)`, `Draw_Relations_Sliders_`,
`Setup_Main_Fields_` (RETURN Esc at 0x217,0x1b1; with races, radio buttons
IGNORE "I" 0x1ad,0x1bc, REPORT "R" 0x1ad,0x1a7, AUDIENCE "A" 0x14e,0x1a7,
DECLARE WAR "D" 0x14e,0x1bc), `Setup_Race_Display_Fields_`, a full-screen
field with hot key " ", auto function `Draw_Race_Screen_`, fade in.

Input (right clicks count as left): a main button outside who mode clears
the other three and enters who mode (the buttons drawn, then
`Setup_Race_Who_Fields_`: a field over each race's panel row, x 0 or
0x140, the race's picture rows; the pointer becomes the who picture). In
who mode any main button leaves it (fields back, pointer back); space
restarts; a panel acts on its race and restarts: IGNORE toggles bit p of
the human's `Ignoring` (+0xe60); REPORT runs `Race_Report_Screen_`;
DECLARE WAR (unless the treaty status is already 4 or more) asks the
BILLTEX2 0x1a question with the race's name and on yes sets the human's
`DelayedOrder[p]` (+0x897) to 0; AUDIENCE clears the ignore bit and calls
`Diplomacy_Screen_` (docs/screens/diplomacy-screen.md, "The player's
diplomacy") — network games wait for the other side first (not ported). The 1.31 patch (0x16927d) makes REPORT act
like leaving who mode when bit 2 of byte 14 of star 0's name is set.

Spies (mouse only): clicking a race's spies or the agents (the field at
0x148,0x182) picks up the icons from the one under the pointer to the
last (`Move_Icon_Group_Icons_`, `_g_spy_xfer_count` from the cursor box),
drawn in the pointer's bitmap with their number (`Redraw_Spy_Mouse_`);
the next click drops them on a race or the agents (at most 63 there) or
back where they came from, restoring that race's mission. Mission buttons
(1 espionage, 2 sabotage, 3 hide; a 0 becomes 3). `Update_Spy_Stuff_`
writes missions (bits 6..7) and counts (bits 0..5) of +0xe57[p] and the
agents (+0xe57[human]) before reports, war, audience and leaving.

Leaving: put carried spies back, `Update_Spy_Stuff_`, fade out, the auto
function back, help off, `Reset_Mouse_Picture_`, the saved palette
restored and darkened, fields drawable, `_current_screen` = 0.

## Data (Init_Race_Display_Data_ 0x10ba3d)

Seven 0x54-byte panels: slot, player, player record, race, "no slot"
(+0xa), eliminated (+0xb), the spies' icon group (+0xc, from
`_spy_group_data` row i: x1, y1, x2, y2, height; count = the human's spies
in p; the human's spy icon RACES.LBX 0x2e+race), mission (+0x2c), slider y
(+0x2e: `_race_bar[i].y` + `_slider_remap_list[(relation+100)/8]` - 6),
bar field (+0x30), relation name (+0x32: first `_relation_name_table` entry
not below relation+100), spy/who field (+0x36), portrait 0x20+race,
slider RACES.LBX 2, crossed-out 0x1f, mission buttons 0xa+i, 0x11+i,
0x18+i. Icon groups (`Calculate_Icon_Group_Data_`): step =
(x2-x1+1-icon width)/(count-1), at most the icon width, at least 0.

## Drawing

`Draw_Race_Photos_`: portrait (crossed out when eliminated), boxes in
`_banner_color_high/low`, corners in `_banner_color_dark`.
`Draw_Race_Text_`: an empty slot, or a race only known by omniscience,
shows "no contact" (and the emperor's name for the latter); otherwise the
emperor's name squeezed under the portrait and, unless eliminated, a
paragraph of the treaty (upper-cased label, or label 0 if nothing else),
research ("\r%s%i%s" with RP/BC, FP/MC, PR by language and sign), trade
("\r%s%iBC"/"MC") and tribute (giving, else receiving, 5 or 10 percent);
then the spy and agent bonuses (`Compute_Spy_Bonuses_`, "%s%hi%%").
`Draw_Relations_Sliders_`: bar RACES.LBX 3 and the slider; empty slots
only the slider at bar y + 0x4b. `Draw_Race_Screen_` (auto): fields,
every race's spies, in who mode a 0xff box on the hovered panel,
otherwise the spy cursor (`Draw_Icon_Group_Cursor_`) on hovered spies or
the relation name on a hovered bar, "Ignoring" on ignored races, the
agents and their cursor, the carried spies at the pointer, colour 0xff
cycled.

## Race_Report_Screen_ (0x10a6c0)

For player p (`_cur_plyr_ptr` = p meanwhile): personality (and objective,
"%s\r%s", for AI players), RACERPRT.LBX 0..9, p's spies in the human's
empire as an icon group (`Setup_Icon_Group_Data_`, template 0x183f43) and
text 0x4a/0x4b (0x1c without), allies (treaty 2) and enemies (4+) joined
by "\n" (texts 0x1d/0x1e when none) in `_TREATIES_FMT` under texts
0x1f/0x20. RETURN (Esc) at 0x216,0x1b1. Each pass draws the page (portrait
framed, name, personality, spies, treaties), fades in and runs
`_Tech_Review_Subscreen_` with RACERPRT.LBX 2..9, first field RETURN+1 and
player p, with `_race_report_jim_hack` (applications the human lacks in
colours 4); until RETURN, then fade out.

## Quirks kept

- The report button's patch reads a flag from star 0's name.
- `Set_Paragraph_Integer_Pointer_(0)` in the report points at an unset
  local; the paragraph does not use it (not modelled).

## Engine / tools

`Picture_To_Mouse_` (image 0x13 of `_mouse_list_variable`).
`Options.ListKey` gives the who-mode panel fields hot key 1 (operand 0x10c85d), so key 1 picks the first race.

## Check

`TestRaceScreenStates` (build/oracle/races, SAVE1): races screen, who mode
(the cycled 0xff box skipped), space, back to the map — all match.
`TestRaceScreenActions` (build/oracle/race1, SAVE1 with the human's treaty
with player 1 cleared, `-listkey`): REPORT on the first race (the report
with its tech review), back, IGNORE it, DECLARE WAR on it with the
confirmation, F10: the captures and SAVE10.GAM against the original's.
`TestRaceScreenMouse` (build/oracle/racemouse1, `build/oracle/racemouse1.sh`,
`-clicks`): SABOTAGE and HIDE by click, spies picked up (the count follows
the pointer's x) and put down on another race, agents put on a race, spies
dropped on a mission button and carried to RETURN (both put back), F10:
every capture (the cursor box's cycled colour 0xff skipped), every draw and
the save match. Not compared: network games. The audience
is checked by the diplomacy runs (`Options.DiploKeys` gives the who mode's panels keys 1..7: panel i gets "1"+i).
