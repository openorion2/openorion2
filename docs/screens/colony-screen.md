# The colony screen (COLONY.C, COLDRAW.C, COLSYSDI.C), screen 1

Go: `internal/game/colony_screen.go` (loop, loading, fields),
`colony_draw.go` (drawing), `colony_input.go` (input), `colony_bldg.go`
(the planet surface: building array, roads, vegetation),
`colony_popups.go` (popups). Colonists and their rows share
`colsum.go` with the colony summary. Addresses are ELF.

Opened by `Do_Colony_Screen_` (0x8310e: from the main screen's system
display; an outpost only gets H 0xd9 in a box), from a colony summary row
and from the reports; `_screen_data[0]`, `[1]` are star and orbit.

## Colony_Screen_ (0xc058a)

Seeds the RNG with the old `_colony_handle` and restores it on leaving,
except when the planet has no colony: then it leaves through the shared
epilogue without the restore. `_colony_handle` = the planet's colony;
`_valid_colony`; `_drawing_display` 0 (system display) or 2 (planet text
for an empty planet); input delay 3, help list 0x1831f0 (0x11 entries).
`_PLAYER_NUM` becomes the colony's owner until the screen ends.
`Load_Colony_Screen_Seg_` (COLBLDG/COLSUM pictures, transport buttons,
the governor's portrait `Load_Officer_Picture_(o, 1)`, road pictures,
galaxy map stars, COLPUPS buttons into C_Anims_'s table),
`Make_Bldg_Array_For_Colony_`, the saved return screen
(`_colony_saved_screen`), palette 1 with the planet's palette, glass
colours, fades as `Fast_Fade_Out_Needed_For_Colony_Palette_` says. Then
`Do_Informational_And_Decision_Popups_` (occupation policy of a conquered
colony, Occupation_Policy_Popup_ 0xcd969).

Loop: Mark_Time, fields rebuilt when the last input was not 0, draw,
toggle, Get_Input, Scan_Input, `Evaluate_Input_` (false: another colony or
screen), Release_Time(1), until RETURN. On leaving: `Reset_Field_Mode_`,
the colony queue fixed (`Fix_Colony_Queue_`, an empty slot 0 becomes trade
goods), back to `_return_screen` or the saved screen.

## The planet surface (COLDRAW.C, colony_bldg.go)

`Make_Bldg_Array_For_Colony_(ci, small)` seeds the RNG with the colony
index (restored after), places the colony's buildings in a 6×6 grid
(`Insert_Bldg_Into_Array_`, `Find_Replacement_Slot_For_Building_`,
`Sort_Bldg_Array_Columns_`; quirk: a building is swapped into cell
(k, k) with k clamped from c−dc), satellites (up to 10), roads
(`Build_Road_List_Based_On_Bldg_List_`, `Pick_Random_Road_At_And_Around_CR_`
compares with a flag already cleared) and vegetation by climate.
Buildings are drawn back to front from BLDG<n>.LBX (cached), glassed;
the hovered one dark (`Draw_Bldg_CR_Dark_`).

## Drawing (Draw_Colony_Screen_ 0xbed21)

Top panel (`Darken_Fill(0, 0, 0x280, 0x96)`), name and population, the
colony type, morale (government icon COLONY2 0x13+gov), BC, food,
industry and research rows (shared with the colony summary, shadow
farmers for idle farm room), the build box (product picture, the
production bar, turns), troops (marines, armour), the governor, the system
display (planets, summaries, the cycling marker at the colony's orbit) or
planet text, satellites, buildings, the scanned item's name at the bottom.

## Input (Evaluate_Screen_Input_ 0xbfdee and the Check_* routines)

- RETURN / Esc leave; L or the governor go to the leaders screen (0x1d);
  "<" (E 0x8b) moves to the *next* colony and ">" to the previous one
  (`Get_Next_Colony_` with 0 and 1); a = autobuild toggle; b = buy
  (`Tested_Colony_Buys_Outright_`); BUILD opens the build queue popup
  (screen 0x19, `docs/screens/build-queue.md`). Its hot key "c" never
  works: the cheat key "c" is added first.
- Job rows: pick up colonists and drop them in another row
  (`Give_Colonist_New_Job_`), with the transport popup while held
  (see "Popups").
- Buildings and satellites: a click sells (E 0x107; once per colony and
  turn, E 0x243; the BC gain is half the cost as int16; the stardate goes
  to colony +0x167, `SoldStardate`), a right click shows the description.
  Houses, rubble and building 9 cannot be sold.
- The production rows open explanations (Show_Morale_ etc.), the troops
  the troop popup (see "Popups").
- The system display: another planet's colony, or where held colonists
  go; the galaxy map display (1) and its toggles are dead code.

## Popups (colony_popups.go)

- **Transport** (COLXPORT.C, Xport_Popup_ 0xc5934). The TRANSPORT
  button (C_Anims 0x18 at 0x133, 0x9f) exists while the colony has
  unemployed colonists. The popup draws the colony screen once under
  COLPUPS.LBX 0x10 (with `_g_hacky_dont_draw_cluster_flag` 0x182976 set,
  so Draw_Cluster_ stays quiet), then six rows of the player's colonies
  in colony order (Build_Global_Colony_List_ into a local list of 250;
  name with Squeeze_Print_Paragraph_ type 0 at 0xf, 0x36+0x26·i, red
  when the star is blockaded for the player or the colony has the
  plague; employed colonists from 0x94 to 0x1b9). It shares the colony
  summary's `_first` and `_slider_bar_position` (Update_First_ with 6),
  so it opens where the summary's bar was. Fields: a scroll bar from six
  colonies on, up/down buttons (Increment_First_/Decrement_First_ with
  6; down only while a seventh row exists), RETURN, and a hidden field
  per filled row. A row field keeps its number while its row is empty;
  an empty input then matches a stale 0 and gives -1, which changes
  nothing. The row picked goes to Send_Cluster_; RETURN gives -1.
  The colonists only become unemployed by being picked up
  (Get_Cluster_ sets `_cluster_colony_n`); a save with unemployed
  colonists shows the button with `_cluster_colony_n` = -1, and a row
  picked then makes Send_Cluster_ read colony -1 (the port logs and does
  nothing).
- **Troops** (COLGCBT.C, Troop_Popup_ 0xcf1e0). The colony screen
  darkened under COLGCBT.LBX 0x15 with Compute_Colony_Ground_Combat_Info_
  (0xed713; the governor's bonus comes before the player's, as in the
  original): the best weapons and armour, the race bonuses, and a column
  each for armour, marines and militia. Any input closes it (Esc does
  not: cancel is off on the colony screen).
- **Explanations** (COLCALC.C): Show_Morale_ (0xddefb), Show_BC_,
  Show_Food_Production_ (0xdeb1d), Show_Industry_Production_ (0xdf518),
  Show_Research_Production_. Each reruns the production routine with a
  breakdown (`prodBreakdown` in tech_economy.go) and prints the Colcalc
  strings (RSTRING 0..59) in a text box. Quirks kept: Show_Food adds the
  replicated food to the colony's food output and Show_Industry takes the
  replicated food (×2) and the taxed industry off its industry; the
  explanation's per-colonist totals include the specials, so research
  specials are counted twice; on a colony without colonists the line
  buffer is never set.
- **Cheats** (Do_Cheats_ 0xc0b87): TOGGLE (E 0x218, a multi hot key
  typed with Alt, Add_Screen_Fields_ 0xbf08e) switches them on and marks
  the player cheated (flag 6); then "c" reads one key with getch_: P +100
  production (and the recompute every other key gets), Q +10000, C
  colonizes the planet shown (only with the planet display, 2), F five
  freighters (free ones follow at the recompute), B +100 BC.
  `TestColonyCheats` (below) covers B, F, P, Q, C without the planet
  display and an unknown key. C's colonizing branch has no oracle: the
  planet display (`_colony_display_mode` 0x182ac8 = 2, set by a click on a
  planet's summary or by opening the screen on a planet without a colony)
  lasts one pass, since `Check_System_Display_Fields_` (0xbfba5) turns it
  into the description box (`Help_`) on the next pass whatever the input.
  C must therefore be the very next `Get_Input_` result after the click,
  which keyboard-paced runs cannot type. `TestColonyCheatColonize`
  (build/oracle/cheat3.sh: an empty planet added to Jabo's orbit 0, its
  summary clicked twice with C C after each) checks that path: the box
  comes up, C C changes nothing, the F10 save matches.
- **Star name** (Change_Star_Name_Popup_ 0x92281, homestarname.go): the
  landing screen's and the colonize popups' call. Oracle:
  `build/oracle/name1` (`build/turns/inputs/name1.GAM`: SAVE1 with colony
  ship 10 placed one turn from the empty Shimari, X 1350, Y 255, ETA 1;
  capture.sh with `-listkey -clicks`): TURN, the planet picked with an F11
  click, Y, the landing (animations on), "Enter Star Name", Enter, the
  colony screen. `TestStarNamePopup` matches every capture (but the
  blinking cursor) and every draw.
  Do_Change_Star_Name_ itself runs in the new game oracles' home star
  naming (`TestNewGameTurn1`, the default name kept).

## Landing screen (Colony_Landing_Screen_ 0xb9802), screen 0x21

`colony_landing.go`. Set by `Colonization_` when settings byte 6
(animations) is on. Colony font, `Load_Colony_Screen_Seg_`, star, orbit
and colony as on the colony screen, COLONY.LBX 6 (the landing animation,
a fresh copy), fades as the colony screen. Full draw: black, C_Anims 1
and 0 (sky, planet), E 0xe5 with the planet name centred at 0x13f, 10
(font style 4). Each frame the animation; from its second-to-last frame
on `Print_Planet_Data_` (0xb953e): name (E 0x46) or name and owner race
(E 0x28) at 0x14, 0x28; gas giants and asteroids E 0x132; else size and
climate (E 0x2d, "%s, %s" at 0x179dcc for language 4; the colony's
effective climate), E 0x1f room, E 0x176 minerals, the gravity, the
special (paragraphs width 0x26b). A conquered colony (occupation 4) with
colonists gets the occupation popup first. The only field is the whole
screen without a hot key: it leaves on a click, never on a key. Then a
sole colony at the star with animations on would ask for a star name
(Change_Star_Name_Popup_); `_current_screen` = 1. Sounds
are not played. `TestColonyLandingScreen` (build/oracle/land1: col2 with
animations on) matches the original's last landing frame.

## Not ported

Send_Cluster_ without a picked-up cluster (reads colony -1). Untested:
Do_Cheats_'s C with the planet display.
