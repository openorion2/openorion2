# NEW GAME: the settings, the race, the ruler, the banner (NEWGAME.C, RACESEL.C, RACEOPT.C)

Go: `internal/game/newgame_screen.go`, `racesel.go`, `raceopt.go`. Addresses
are ELF. The main menu's NEW GAME (hot key N) sets screen 0xd with
`_return_screen` 0. Ported: the single player path up to where the game
would be generated; `Init_New_Game_` and everything it runs is **not
ported** (the port logs it and goes back to the main menu with the chosen
settings in `_settings` and player 0 filled in).

## Screen_Control_, case 0xd (0x1059e)

Single player: `Newgame_Screen_`; a non-zero result starts the game through
the 1.40 patch's wrapper (0x169410), which adds one to the galaxy size when
the patch's "Cluster" flag (byte 0x1784df) is set, then `Init_New_Game_
(difficulty, size, age, players; tactical, tech)` from `_settings` +0xd4,
+0xd6, +0xd7, +0xd5, +0xd8, +0xd9. A zero result goes to the main menu
(screen 10). Other game types call `Newgame_Screen_` with `_return_screen`
0xf (not ported).

## Newgame_Screen_ (0xcd435)

`_color_ken` = {0x79, 0x7b, 0x7a, 0x7c×5}. Clears stars, planets,
colonies, ships, players and leaders, blank pointer, `Fast_Fade_Out_`,
palette 9; single player and hotseat clear `_color_flag` (banners taken)
and `_race_flag` (races taken). `Reload_Newgame_Screen_`; the button
variables from `_settings` (`Globals_To_Temps_` inline: difficulty +0xd4,
size +0xd6, age +0xd7, players +0xd5 − 2, tactical = (+0xd8 == 0), tech
+0xd9, random events +2, Antarans +0xd3); `Clear_Click_Thru_Fields_`,
`Clear_Fields`, `Add_Fields_`, `_full_draw` = 1, stencil refresh, help list,
auto function `Draw_Newgame_Screen_` (2), the screen drawn on the off page,
`Darken_Palette_(100)`, palette changes cleared, `Fast_Fade_In_`, arrow
pointer.

Loop: `Get_Input`, `Evaluate_Click_Thru_Fields_(r)`; CANCEL or Esc: done,
`_current_screen` = `_return_screen`. ACCEPT: the variables back into
`_settings` (`Temps_To_Globals_` inline); hotseat goes to screen 0x10,
network/modem to 0x15 (result 1); single player runs
`Race_Selection_Screen_(player 0)`: 1 → player 0's objective = 100
(human), done; otherwise the screen is reloaded and set up again. When
the tech level changed, the help list again. Unless done: style 3,
`Draw_Newgame_Screen_`, style 3, toggle. On leaving with result 1 the
variables go to `_settings` once more; fields, click-through fields, help
and the auto function are cleared.

`Reload_Newgame_Screen_` (0xcca1c): RACEOPT.LBX 0 (background),
NEWGAME.LBX 1..3 galaxy age, 4..8 difficulty, 9..12 galaxy size, 13..19
players, 20..22 tech level, then 23 tactical combat, 24 random events, 25
Antarans, 26 CANCEL, 27 ACCEPT, 28 the frame, 29 the network box;
`Load_New_Game_Strings_` (E strings: difficulty 0x221 0x118 0xbe 0x13e
0x148; size 0x1e0 0x173 0x168 0x143; age 0x179 0xbe 0x192; players 0x94
0x96 0x98 0x9a 0x9c 0x9d 0x9e; tech 0x1ad 0xbe 0xa9; combat 0x1f7 0x1f0.
Italian puts "Piccola" in size 0; French and Italian also write a hack
string into size slot 2, which the next string overwrites at once).

`Draw_Newgame_Screen_` (0xccd3c): style 4 shadow down, outline (0, 0x40),
the frame (NEWGAME 28) at (`_g_x`, `_g_y`) = (0xf, 5) on the current page;
style 2; off page; with `_full_draw`: black, background, style 3, off to
back, `_full_draw` = 0; `Draw_Click_Thru_Fields_`; `Draw_Pictures_` (the
five pictures at (`_g_x`+0x79+col·0x9b, `_g_y`+0x77+row·0x8c): difficulty,
size, age; players, tech); the network box for network games; style 3,
`Set_Alias_Color_(0x9a)` (shade 0 of the current, normal, highlight and
special colours).

`Add_Fields_` (0xcce2e): per picture a hidden field over it and a
click-through field over its label (0x64×0x14 at (+0x69, +0xcc), (+0x105,
+0xcc), (+0x1a1, +0xcc), (+0x69, +0x15d), (+0x105, +0x15d)) linked to the
picture's field; three radio buttons at (+0x17c, +0x127 random events,
+0x14a Antarans, +0x103 tactical combat), CANCEL at (+100, +0x182), ACCEPT
at (+0x1a2, +0x183), Esc. Every hot key is E string 0xc (none).

`Set_New_Game_Screen_Help_List_` (0xcdb72): the 11 entries at 0x18348e,
the first one's id set by the tech level (0 → 0x226, 1 → 0x225, 2 →
0x224); the change stays in the table.

## Click-through fields (FAKEBUTS.C) and the 1.40 patch

`Add_Click_Thru_Field_` (0xc6a43): a hidden field over the label plus a
0x17-byte record {x1, y1, x2, y2, field, linked field, strings, count,
variable, box colour}. `Evaluate_Click_Thru_Fields_` (0xc6aa4): the first
record whose field or linked field was hit steps its variable (v+1) mod n.
`Draw_Click_Thru_Fields_` (0xc68c4): a box when the colour is set, the
window, `Squeeze_Paragraph_Centered_(x1, y1+2, x2−x1, y2−y1−2, label, 2)`.

The 1.40 patch hooks both (0x169044, 0x169088) by the variable's address:
the tech level steps 0 → 1 → "Post Warp" (1 with the patch byte 0x1784de
set) → 2; the galaxy size, when entered from the main menu
(`_previous_screen` 10), steps 2 → "Cluster" (2 with byte 0x1784df set) →
3. Drawing outside screen 10 clears the Cluster byte. The labels come from
the patch's data (0x178514, 0x1784e0). The port keeps the bytes in
`Game.patchFlags`.

## Race_Selection_Screen_ (0x5c510)

`Disable_Cancel`, help off, fields cleared, fade out, palette 9,
`_custom_flag` 0, shown race 3, button status 3 on;
`Reload_Race_Selection_Screen_` (0x5bc74: the 13 races' picks, RACESTUF.LBX
entry 7 records 0..12; with lastrace.rac its picks into slot 13 (converted
back by `Convert_Custom_Race_Flags_`) and its name; RACESEL.LBX 1..14 the
buttons; patch13.lbx 1 the last race button), `Set_Race_Select_Help_` (a
0x2c-high entry per button, ids 0x276..0x283, then the screen 0x284),
RACEOPT.LBX 0 on the back page copied to off; Esc; 14 radio buttons at
(0x15f + i/7·0x7e, 0x5a + i%7·0x30) with no hot key; auto function
`Draw_Race_Selection_Screen_`; `_old_race` 0; drawn, darkened, toggled,
`Fast_Fade_In_`.

Loop: `Get_Input`, the shown race = `Scan_Input` − the first button's
field (outside 0..13: the old one). A free race i (0..12):
- normal mode: 5 ticks, fields cleared, player picks = the race's, race,
  race name (`_race_names`), help off; then `Naming_Popup_` (ruler name)
  and, accepted, `Flag_Screen_` (banner); Esc on the banner goes back to
  the name. Done: picks 1..9 become their values (table 0x17d1f9, 3
  bytes per pick), `_race_flag[i]`. Cancelled: the screen set up again.
- custom mode: the race's picks and name, then `Racial_Option_Screen_`;
  cancelled: set up again with the Custom button, custom mode off.

Custom (button 13): the first press enters custom mode (fields above race
12 cleared; with lastrace.rac the 1.40 patch (0xf4f8d) adds the last race
button there, outside the multiplayer screens), so the next race picked is
the base of a custom race. In custom mode the last race button loads
lastrace.rac (picks, race, name) into `Racial_Option_Screen_`. Esc in
custom mode leaves it (the Custom button back); otherwise Esc returns 0.

`Draw_Race_Selection_Screen_` (0x5bd97): the title (RACESEL 0x21 at
(0x16e, 0x34), in custom mode 0x1d at (0x16f, 0x30)); a new shown race
moves the button status and sets the palette of its picture (RACESEL
0xf+race), the same race draws the picture at (0x36, 0x3f), or for a
taken race Ken text 0x48 at (0x3c, 0xaf, 0x118). The 1.40 patch (0xf4faa)
then makes the old race the shown one. The description (free races, or the
saved custom race in custom mode): picks 1..9 with their names and values
("+", "1/2", the halved value of pick 2), the yes/no picks 10..30 set to
1, the government, joined by ", " and Ken text 0x68 (`Message_Build_`), at
(0x32, 0x187, 0x127) in style 2; the saved race's name centred at (0xc5,
0x2a). Last RACESEL 0 (the frame) at (0x30, 0x3a).

## Naming_Popup_ (0x5d2bb) and the input box (NAMESTAR.C)

Auto function `Draw_Naming_Popup_` (black and RACEOPT 0 when
`_FULL_DRAW_SCREEN_FLAG`). **Draws Random(8)**: the default name is
RACENAME.LBX record race·8 + r − 1; title Ken text 0x6a;
`Remapped_Input_Box_Popup_(name, 0x14, 0x66, title, 0xb1, 0x7d)`; an empty
result keeps the default.

`Input_Box_Popup_` (0x91f14): saves the help list and auto function, help
0x181f88; the box record at 0x19c330 (text ≤ 0x64, title ≤ 0x3c, field at
(x+0x22, y+0x36) 0x1a high, width 0xcd at most, button at (x+0x60,
y+0x64)); colours by screen (0xd: offset 0x14, `Font_Colors2_(3, 0,
0x14)`); `Setup_Input_Box_Popup_`. Loop: Enter in the field accepts, a
cancel (−1) puts the text back and returns 0, the button accepts (drawn
pressed when remapped); otherwise redraw on full draw and toggle. Then
`Trim_`, help and auto function back, full draw, the result.

`Setup_Input_Box_Popup_` (0x91bd4): inbox.lbx 0 (box) and 1 (button); the
saved auto function on the off page, off to back, toggle, stencil; the box
(remapped: `Calculate_Picture_Remap_Colors_` on both and `Remap_Draw_`);
the title fitted (`Set_Fitted_Font_Style_`, style 4, spacing 2, width
w−0x36, style 1 at least) and centred in colours `Font_Colors2_(style,
0xa, 0x1a)` (screen 0xd), remapped style 4; off to back, toggle; the
button field (hidden when remapped), the text input field (sound 0x29),
activated with "first" set (the first Backspace clears the text); auto
function `Draw_Input_Box_Popup_`.

## Flag_Screen_ (0x5d03c)

Palette of RACESEL 0x20 between two toggles, auto function
`Draw_Flag_Screen_` (RACESEL 0x20 at (0x37, 0x43), the free banners
RACESEL 0x22 + race·8 + c at (0x60 + c%4·0x7c, 0x91 + c/4·0x88), one pixel
left for banners 3 and 7), help 0x2a4. Each pass: a hidden field per free
banner (0x53×0x62, sound 0x29, no hot key), Esc; a banner picked: player
banner, `_color_flag`, the clock pointer (`_cmbt_mouse_list_clock` image
7), drawn, toggled, 1. Esc: `_hot_seat_flag` 0, 0.

## Racial_Option_Screen_ (0x5a3bc), the custom race

`Convert_Player_To_Flags_` (picks 1..9 → the nine groups' levels,
government/2, picks 10..30 → 22 toggles, pick 15 (rich/poor home world)
into two), `Reload_Racial_Option_Screen_` (RACEOPT.LBX 1 box, 2 DEFAULT, 3
ACCEPT, 5 greyed box; racestuf.lbx entry language: 64 names, entry 6:
the costs), the race name, `Calculate_Picks_` (picks from byte 0x178513
(10, the 1.40 patch) minus every chosen cost; also the sum of negative
costs), auto function `Draw_Racial_Option_Screen_` (1), `_color_opt` =
{0x8e, 0x9d, 0x9c, 0x9d×5}, `Add_Race_Options_Fields_` (name field 0x96
wide at (0xf8, 0xf) once, then above it: Esc, ACCEPT "a" (0x1fa, 0x1c0),
DEFAULT "c" (0x4c, 0x1c0), a hidden field per group level (columns X =
0x50, 0xfe, 0x1ba, 0x283; rows Y = 0x48, 0x93, 0xde, 0x129, 0x174, 0xd
apart), the government's four, the 22 toggles 0x10 apart in column 2).

Clicks: a group level sets it (the same level clears it); group 1 is
refused (Ken 0x49) with toggle 9 on; a cost taking the negative picks
below −10 is refused (Ken 0x4a); the same for the government and the
toggles, which also switch off their exclusive partner (5/6, 0/1, 8/9
with group 1, 10/11, 12/13). DEFAULT clears all (government 1). ACCEPT
with negative picks: Ken 0x4b; otherwise `Convert_Flags_To_Player_`,
RACEOPT 4 and 6, `Naming_Popup_` and `Flag_Screen_` as for a race, and the
race goes to lastrace.rac (picks, race, name; the port never writes
through a symlink there). `Draw_Race_Option_Background_` prints every
pick's name, dots and cost; `Draw_Racial_Option_Screen_` the picks left and
the score (picks·10+100 %), then each pick's box chosen or greyed, with
the text in the greyed colours where it cannot be taken
(`Real_Ability_Cost_` counts what a toggle switches off).

## Check

Oracle runs under `build/oracle/newgame1..3` (MOX.SET, rng.txt, the F12
captures; gen.GAM where the generation ran; lastrace.out the original's
lastrace.rac). The tests replay the same keys through the port (main menu,
`-newgamekeys`, the run's timer seed as `_cmd_line_seed`) and compare each
frame with the capture:
- `TestNewGameScreens` (newgame1): every setting cycled, Cluster and Post
  Warp, the three radio buttons, the race screen and Esc back, the ruler
  name and its cancel, a race, the banner screen, Esc back to the name, the
  banner screen again. Differences confined to the name box's blinking
  cursor are allowed.
- `TestNewGameStart` (newgame1 + banner 1): `_settings` +2, +0xd3..+0xd9,
  player 0 (ruler, race name, race, banner, picks, objective) and the RNG
  seed after the ruler name draws against gen.GAM and the trace.
- `TestNewGameCustomScreens`, `TestNewGameCustomStart` (newgame2): custom
  mode and Esc out of it, a letter typed into the name, its cancel, a
  custom race on the Bulrathi, ACCEPT, the name, the banner; player 0 and
  lastrace.rac byte for byte.
- `TestNewGameCustomCancel` (newgame3): CLEAR, ACCEPT, Esc on the name back
  to the picks, Esc on the picks back to the races (CANCEL to the main menu
  is not compared: the credits there move by the clock; see main-menu.md).

Not compared: the last race button (needs lastrace.rac before the screen),
network and hotseat games.
