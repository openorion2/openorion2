# Main screen: panel, buttons, hot keys, boxes

Everything of the galaxy screen (`Main_Screen_` 0x86188, screen 0) outside the
map itself: the right-hand data panel, the button and window fields, hot keys
and cheats, the help list, the message strings, the moveable boxes (dispatch
level) and the multiplayer lights. ELF addresses (Watcom + 0x10000); data
addresses are ELF (0x178000 + DWARF offset). Engine behaviour that already has
a spec is in `docs/engine/fields-and-input.md` and `docs/formats/fonts.md`.

Common globals:

| Name | Addr | Notes |
|---|---|---|
| `_player` | 0x197f98 | pointer, records of 0xea9 (`save-players.md`) |
| `_PLAYER_NUM` | 0x19999c | s16 |
| `__stardate` | 0x192fd8 | s32, tenths |
| `language` (no symbol) | 0x199cae | u8; `Set_Mox_Language_` (0x10c2f) reads `language.ini` but always stores 0 (both paths end at 0x10c9a `mov [0x199cae],0`), `Reload_Language_Strings_` also sets 0. Treat as 0 = English |
| `_settings` | 0x199bdc | `save-misc.md` settings block (+0x01 at 0x199bdd, +0x15 at 0x199bf1) |
| `_red_color_pulse_value` / `_incr` | 0x199f07 / 0x199f06 | u8/s8; `Harold_Defaults_` sets 0 / 1 |
| `_game_type` | 0x199f3a | 0 single, 1 hotseat, 2 network, 3 modem |
| `_moveable_box` | 0x192fdc | 5 × 0x1c (`save-ships.md`); `content` of box i at 0x192fde+0x1c·i |
| `_box_off_flag` | 0x1992c0 | s16[5]; `Harold_Defaults_` (0x79353) {-1,0,0,0,0}, `Harold_Init_` (0x7962e) {-1,0,-1,0,0} |
| `_moveable_box_order` | 0x1992e8 | s16[5] |
| `_skip_fields` | 0x199976 | s16 |
| `_last_field` | 0x19c154 | s16 |
| `_draw_main_screen_to_back` | 0x199f28 | u8 |

## H_Message_ (0x7a990) and the message file

`H_Message_(i)`: `ix = (0 ≤ i < 0x18d) ? _h_string_ix[i] : 0`; returns
`&_h_strings[ix]` (`_h_strings` 0x19341c, 0x4074 bytes; `_h_string_ix` 0x1992fc,
u16[0x18d]). Out-of-range indices give string 0 (" ").

`Load_H_Strings_` (0x7a816), called from `Load_Mox_Global_Strings_`: language
>5 → 0; per language file, record size, max string length:

| lang | file | size | max len |
|---|---|---|---|
| 0 (default) | HESTRNGS.LBX | 0x34d1 | 0x181 |
| 1 | HGSTRNGS.LBX | 0x3d01 | 0x1a0 |
| 2 | HFSTRNGS.LBX | 0x408d | 0x118 |
| 3 | HSSTRNGS.LBX | 0x3ce8 | 0x1a1 |
| 4 | HISTRNGS.LBX | 0x36cf | 0x172 |

`memset(_h_strings, 0, 0x4074)`; `Farload_Data_Static(file, entry 0, dst, rec 0,
1 record, size)`: entry 0 is a record table (`u16 count=1, u16 size=0x34d1`,
then the data), so the strings start 4 bytes into the entry. Strings are
NUL-separated; `ix[n]` = start of string n, for n = 0..0x18c; a string longer
than the max → `Exit_With_Message("%s: String %d Too Long ...")`.

Verified (HESTRNGS.LBX, after the 4-byte header): 0 " ", 0x20 "Option disabled
during multi-player games", 0x21–0x24 move errors, 0xd1 "You have set only one
star relocation", 0xd2 "You have not set any star relocations", 0xd3 "GAME
SAVED", 0xd4 "Save Game function disabled", 0xd5 "Use the FORCE, Luke
(omniscience on)", 0xd6 "You have chosen POORLY! (omniscience off)", 0xd7 "%s
has been given a gift of %d bcs", 0xd8 "%s will be known", 0xda "Stardate:
%d.%d", 0xdd "All new ships built will be sent to %s", 0xde "Master of Orion ][
bids you a good day!", 0xdf "You do not have any freighters to scrap!", 0xe0
"space flux", 0xe1 "%d turn", 0xe2 "%d turns", 0x183 "Breakthrough", 0x188
"none", 0xa6.. option names ("End Of Turn Summary ", "End Of Turn Wait ", …).

Unit strings (3 bytes copied): `Get_Monetary_Unit_String_` (0x7aba6) "BC"
(lang 1 "MC"); `Get_Research_Unit_String_` (0x8a7fc) "RP" (1 "FP", 2/4 "PR",
3 "PI").

## Pictures (BUFFER0.LBX, palette FONTS.LBX entry 1 = `Load_Palette(0, 0, 0xad)`)

`Load_Main_Screen_Pictures_` (0x131c2), `Far_Reload_Next` into
`_global_screen_seg`. Rendered and checked:

| Entry | Global | Size | Frames | Use |
|---|---|---|---|---|
| 0 | `_main_screen_seg` 0x192ee4 | 640×480 | 1 | whole frame incl. panel art and all buttons in normal look; map area transparent. Reloaded (`Buffer_Reload`, `_screen_seg`-ish 0x193174) and drawn at (0,0) by `Draw_Main_Screen_Filled_` (0x84a95) |
| 1 | `_main_game_button_seg` 0x192eec | 59×17 | 2 | GAME; frame 1 lit |
| 2 | `_main_next_button_seg` 0x192e7c | 65×26 | 2 | TURN; frame 1 lit |
| 3, 4, 5 | colonies, planets, fleets | 239×52 | 2 | strip of the three left buttons, frame 1 has that one lit; drawn at (0, 0x1a9) |
| 6, 7 | zoom in, zoom out | 55×16, 55×19 | 2 | frame 0 dim, frame 1 lit |
| 8, 9, 10 | leaders, races, info | 237×52 | 2 | right strip, drawn at (0x132, 0x1a9) |
| 11 | `_fleet_box_selection_seg` | 54×53 | 1 | fleet box |
| 12 | `_darkened_turn_button_seg` 0x1933f0 | 65×26 | 1 | used by `Draw_Mini_Main_Screen_` |
| 13 | — | 1×1 | 1 | `Draw_Main_Screen_Filled_` draws it at (0x19f,0) for game type 2/3 and at (0x4a,0) for 1/2/3 |
| 0x3a–0x48 | fleet box buttons | | 2 | fleet box (not covered here) |
| 0x52 | 0x193080 | 67×23 | 2 | |
| 0x109–0x110 | `_player_lights_seg[8]` 0x19c128 | 11×14 | 8 | per banner colour |

Pictures have `multi_frame_full_store` (byte 9) = 4, so `Get_Full_Store_Flag` ≠ 0
for all buttons.

## Fields

### Clear_Main_Screen_Fields_ = Set_Main_Screen_Fields_To_Null_ (0x89f8a, one routine)

Sets to -1000: zoom in/out, game, races, fleets, colonies, `__design_button`,
info, next, `_system_display_grid_field`, `_star_button[_NUM_STARS]`,
`_planet_buttons[5]`, 8 words at 0x1931c0 (stride 12), `_main_screen_grid_field`,
`_debug_field`, `_galaxy_map_grid_field`, fleet box all/colonize/outpost/
transport/engage/attack, cheats all_tech, `__leader_cheat`, more_money, get_tech,
omniscience, all_ai. Not planets, leaders, the five windows, exit key, hi score.

### Add_Cheats_ (0x81d15)

`Add_Multi_Hot_Key_Field` for, in order (strings at 0x182104…):
`_all_tech_cheat` "EINSTEIN", `_more_money_cheat` "MOOLA", `_get_tech_cheat`
"MENLO", `_omniscience_cheat` "ISEEALL", `_hi_score_cheat` "SCORE"; then
`_last_field = fields_count - 1`. After `Clear_Fields` these are fields 1–5 and
`_last_field` = 5. "ALLAI", "LDR", "SND" exist but are not added; their globals
stay -1000 (`_omniscient_hot_key` 0x182153 and `_all_ai_cheat` likewise).

### Add_Map_Fields_ (0x81d73)

Zoomed (`_zoom_in == 1`): `Clear_Main_Screen_Fields_`; for i in 0..4 box
`drag_field` (0x192fea+0x1c·i) and `field2` (0x192fec+…) = -1000; zoom in
`Add_Button_Field(0xf4,0x1ac,"",pic6,"+",0x28)`, zoom out
`(0xf4,0x1c7,"",pic7,"-",0x28)`, `_whole_screen_field =
Add_Hidden_Field(0,0,0x27f,0x1df,"",0x29)`; return.

Normal (in this order; numbers after `Clear_Fields`+`Add_Cheats_`):

| # | Global | Call | Rect | Hot key | Sound |
|---|---|---|---|---|---|
| 6 | `_game_button` | Button (0xf9,5) pic 1 | 0xf9..0x133, 5..0x15 | G | 0x29 |
| 7 | `_next_button` | Button (0x220,0x1b9) pic 2 | 0x220..0x260, 0x1b9..0x1d2 | T | 0x28 |
| 8 | `_zoom_in_button` | Button (0xf4,0x1ac) pic 6 | 0xf4..0x12a, 0x1ac..0x1bb | + | 0x28 |
| 9 | `_zoom_out_button` | Button (0xf4,0x1c7) pic 7 | 0xf4..0x12a, 0x1c7..0x1d9 | - | 0x28 |
| 10 | `_colonies_button` | Irregular, pic 3 drawn at (0,0x1a9) | 0x11..0x4f, 0x1b2..0x1d7 | C | 0x28 |
| 11 | `_planets_button` | Irregular, pic 4 at (0,0x1a9) | 0x5b..0x9a, 0x1b2..0x1d7 | P | 0x28 |
| 12 | `_fleets_button` | Irregular, pic 5 at (0,0x1a9) | 0xa7..0xe6, 0x1b2..0x1d7 | F | 0x28 |
| 13 | `_leaders_button` | Irregular, pic 8 at (0x132,0x1a9) | 0x138..0x17b, 0x1b3..0x1d7 | L | 0x28 |
| 14 | `_races_button` | Irregular, pic 9 at (0x132,0x1a9) | 0x182..0x1c5, 0x1b3..0x1d7 | R | 0x28 |
| 15 | `_info_button` | Irregular, pic 10 at (0x132,0x1a9) | 0x1ce..0x20e, 0x1b3..0x1d7 | I | 0x28 |
| 16 | `_treasury_window_field` | Hidden | 0x221..0x265, 0x31..0x77 | none | 0x29 |
| 17 | `_command_points_window_field` | Hidden | 0x221..0x265, 0x7a..0xc0 | none | 0x29 |
| 18 | `__food_window_field` | Hidden | 0x221..0x265, 0xc5..0x10b | none | 0x29 |
| 19 | `_freighters_window_field` | Hidden | 0x221..0x265, 0x10e..0x154 | none | 0x29 |
| 20 | `_research_window_field` | Hidden | 0x221..0x265, 0x158..0x19f | none | 0x29 |
| … | box fields | `Add_Moveable_Box_Fields_` (below) | | | |
| | `_debug_field` | Hidden (0x276,0x1d6,0x27f,0x1df,"",0) only if `_add_mainscrn_debug_field` (0x19c190), else -1000 | | | |
| | `_exit_game_hot_key` 0x19c170 | `Add_Multi_Hot_Key_Field("Q")` | | Alt-Q | |
| last | `_main_screen_grid_field` | `Add_Grid_Field(0x16,0x16,1,1,0x1fa,400,&_main_screen_grid_x,&_main_screen_grid_y,0)` | 0x16..0x20f, 0x16..0x1a5 | | 0 |

Hot-key strings are one char at 0x179714 ("+"), 0x179717 ("-"), 0x179719.. ("G",
"T", "C", "P", "F", "L", "R", "I", "Q"); "" is 0x179716 (hidden fields get
hotkey 0). The leading `Moveable_Box_Selected_(2)` result is discarded. Between
the box fields and the debug field: `Save_Window_Status`, `Set_Window(0x16,0x16,
0x20f,0x1a5)`, `Clip_On`, `Restore_Window_Status`, `__first_field =
_star_button`, then `Reset_Window` after the debug field.

Since the grid is added last and hit testing takes the lowest field number,
buttons, windows and box fields win over the map grid.

### Engine field types used (not yet in `internal/engine`)

- `Add_Button_Field(x,y,str,pic,hot,snd)` (0x1151b0) → `Add_Button_Info`
  (0x114fba): rect x..x+w-1, y..y+h-1 from the picture header, `var1=str`,
  `text_style/text_color` = current font, `button_sound`, `var6=pic`,
  `var4.min_value = Get_Full_Store_Flag(pic)`, hotkey = `hot[0]` upper-cased;
  `type = 0`. Full → `Exit_With_Message(field_exceed_message)`.
- `Add_Irregular_Button_Field(x1,y1,x2,y2,str,pic,dx,dy,hot,snd)` (0x11a92e):
  explicit rect, same info, `var2=dx, var3=dy` (draw position), `type = 0xd`.
  Overflow check is `fields_count <= max` (off by one vs the others).
- `Add_Multi_Hot_Key_Field(str)` (0x11adb4): slot zeroed (55 bytes), rect -1,
  `type 8`, `var1=str`, `var2=0` (progress), `var3=strlen`;
  `multi_hotkey_active_field = 1`.
- `Add_Grid_Field(x,y,bw,bh,nx,ny,&gx,&gy,snd)` (0x114dca): rect
  x..x+bw·nx-1, y..y+bh·ny-1, `type 0xc`, `var2=bw, var3=bh, var4=&gx,
  var5=&gy`, hotkey 0. `Draw_Field(f,1)` for type 0xc writes
  `gx=(Pointer_X+off-xmin)/bw`, `gy=(Pointer_Y+off-ymin)/bh`, clamped to
  0..(xmax-xmin)/bw and 0..(ymax-ymin)/bh (here the pixel offset in the map).
- Multi hot keys, in `Interpret_Keyboard_Input` (0x11ccc4) after the F-key
  table, when `multi_hotkey_active_field`: `c = Get_Alt_Alpha_Key_(key)`
  (0x11ab8a: Alt+letter scan words 0x1000..0x3200 → 'A'..'Z'; 0x7800..0x7f00
  (Alt-1..Alt-8) → '0'..'7'; anything else 0xff), then
  `Check_Multi_Hot_Key_Fields_(c)` (0x11af64): first type-8 field with
  `str[progress] == c` gets `progress++`; if it reaches `strlen` that field is
  returned (search stops). A mismatch does **not** reset progress; only
  `Clear_Multi_Hot_Key_Offsets_` (0x11aef7, all type-8 `var2=0`) does, called
  from `Interpret_Mouse_Input` at 0x11dbe6 and 0x11e701. A hit sets `*field`
  and returns -1, which `Interpret_Mouse_Input` turns into "return field". So
  cheats and exit are typed with Alt held (Alt-E,I,N,S,T,E,I,N; Alt-Q exits).
  `Clear_Fields_Above(_last_field)` every frame keeps fields 1–5, so progress
  survives frames.
- Drawing (`Draw_Visible_Fields` 0x11e718, run by every `Toggle_Pages`, after
  `Reset_Window` + `Set_Page_Off`, skipped when `draw_fields_flag==0`): for
  i = 1..fields_count-1, if `i == down_mouse_button` and type ∉ {4,10,12} →
  `Draw_Field(i,1)`; else by type (jump table 0x11e7fd, types 0..0xb): type 0 →
  `Reset_Animation_Frame(pic)`, `Draw(xmin,ymin,pic)` (frame 0), and the
  string centred if non-empty; types 7, 8, 9 and > 0xb (0xc, 0xd) draw nothing.
  So the irregular buttons are invisible until pressed (entry 0 already shows
  them), the four type-0 buttons are redrawn in frame 0 on every flip.
- `Draw_Field(f,1)` (0x11b05a) for types 0 and 0xd: position (xmin,ymin), or
  (var2,var3) for 0xd; if `var4 == 0` `Reset_Animation_Frame`+`Draw`, else
  `Set_Animation_Frame(pic,1)`; then `Draw` → frame 1 (lit). `Draw_Field(f,0)`:
  reset, draw frame 0. Strings: none here (all "").

## Help list (Set_Main_Screen_Help_List_ 0x6f280)

`_main_screen_help_list` 0x181768, entries `{id,x1,y1,x2,y2}`; static 15:

```
(284,542,24,616,41) (285,237,0,320,20) (286,541,45,617,119) (288,541,121,617,193)
(290,541,195,617,268) (291,541,270,617,342) (292,541,344,617,418) (293,13,428,79,473)
(294,91,428,155,473) (295,166,428,231,473) (300,236,425,307,479) (297,311,428,381,475)
(298,385,428,454,475) (299,461,428,527,475) (296,542,426,618,479)
```

Count n = 15; game type 1: entry 15 = (0x12f,0x49,0,0x9c,0x13), n = 16; type 2:
also entry 16 = (0x12e,0x1a2,0,0x207,0x13), n = 17. If box 2 is open, fleet-box
entries are appended (ids 0x136, 0x130, then 0x135, 0x132, 0x133, 0x134, 0x131
for the shown buttons; geometry from `_fleet_box` 0x199bac, 0x199bae, 0x199bb2,
0x199bba and the close button height). Then `Deactivate_Help_List()`,
`Set_Help_List(list, n)`.

## Main_Screen_ parts that belong here

Start: `Assign_Auto_Function(Draw_Main_Screen_,1)`, `Set_Main_Screen_Help_List_`,
`Set_Field_Outline_Color(0x14)`, `Disable_Cancel()` (so right click returns
`-field` and Esc does not cancel), `_skip_fields=0`, …, `Clear_Fields`,
`Clear_Main_Screen_Fields_`, `Add_Cheats_`, later `Add_Map_Fields_` (or
`Fade_Into_Main_Screen_`).

Each pass (after Get_Input `r`, Scan_Input, map/box handling):
`Check_Function_Keys_(r,&leave)`, `Check_Hot_Keys_(r)` (both only if
`!_skip_fields`), then: `r == _exit_game_hot_key` → `Exit_With_Message(H 0xde)`;
game → screen 8; Alt-F9 (-1109) with settings+0x15 in 0..9 → single/hotseat
`Auto_Load_Last_Saved_Game_`, else `User_Box_(H 0x20)`; Alt-F10 (-1110) →
`Auto_Save_Last_Saved_Game_`; fleets → screen 4 (mini main screen first if box
0 or 2 open); colonies 0x14; info 9; next → `Assert_Settings_`, `Do_Next_Turn_`;
races 6; planets 0x20; leaders 0x1d; research window → screen 0x24 (mini main
screen into back); freighters window (no skip check) → if freighters total
(+0x36) < 1 `User_Box_(H 0xdf)` else screen 0x23; CP window → 0x2b; treasury
(no skip check) → `Tax_Rate_Popup_`, `Do_Player_Colony_Calculations_`; food
window: no action. Zoom/grid handling is the map's. End of pass if not leaving:
`!_skip_fields` → `Clear_Fields_Above(_last_field)`, `Enable_Draw_All_Fields`,
`Enable_Mouse_Pressed_Field_Update`; else `_skip_fields=0`,
`Disable_Draw_All_Fields`, `Clear_Fields`, `Clear_Main_Screen_Fields_`,
`Add_Cheats_`; then `Add_Map_Fields_`, `Set_Page_Off`, `Draw_Main_Screen_`
(or `_For_Zooming_`), flip; … `Release_Time_(1)`.

### Check_Function_Keys_ (0x825a8)

`r == -1110` → return. `-1109 < r < -1100` (Alt-F1..Alt-F8) →
`Toggle_Game_Option_(-1101 - r)` (0..7). Then:

| r | key | action |
|---|---|---|
| -1010 | F10 | `_disallow_saves_code` 0 → `Save_Game_(9)`, msg 0xd3; else 0xd4; `User_Box_(&_draw_main_screen_to_back, msg, 0)` |
| -1009 | F9 | screen 0x1c, return 0, leave |
| -1006 / -1005 | F6 / F5 | `Cycle_Star_Icons_(&_system_star_id_for_auto_select, player, 0/1)`; valid star → colony screen of `Last_Colony_Selected_` (only if auto-select-colony setting 0x199be3) else `Do_System_Popup_` (box 0 content = the star); oracle cyc1, `TestCycleStarsKeys`; the colony screen branch: cyc2, `TestAutoSelectColony` (MOX.SET byte 7 on; also a click on a star with a colony) |
| -1004 | F4 | relocations: >1 toggle `_merging_relocations` with mouse list `_mouse_list_to`(2)/arrow(1); 1 → msg 0xd1, 0 → 0xd2 |
| -1002 / -1001 | F2 / F1 | `Cycle_Ship_Icons_(&_fleet_box_ship_stack, &i, 0/1)`, valid → `Do_Fleet_Popup_(i,0)` |

### Check_Hot_Keys_ (0x82809)

- omniscience cheat (or the dead `_omniscient_hot_key` with
  `Check_Release_Version()==0`): toggle player+0x8ba; now on → `User_Box_(H 0xd5)`,
  +0xe69 = 1, +0xe77 = 1; off → `User_Box_(H 0xd6)`; then
  `_draw_main_screen_to_back=1` and `star+0x14 = Star_Owner_(s, player)` for all.
- EINSTEIN: `Give_Me_All_Tech_Advances_()`, +0xe69=1, +0xe77=2.
- MOOLA: +0x32 += 1000, `sprintf(H 0xd7, race name +0x15, 1000)`, box;
  +0xe69=1, +0xe77=3.
- MENLO: `sprintf(H 0xd8, _technology_applications[+0x322].name)` (13-byte rows,
  dword 0), +0x1eb = 9999, box; +0xe69=1, +0xe77=4.
- `_all_ai_cheat` (never added) → `Make_All_AI_Game_()`.
- SCORE: `sprintf("Current score for %s is %d", emperor name +1,
  Do_Calculate_Hi_Score_(player))`, box. No cheat flag.

## Draw_Main_Screen_ (0x84555), the non-map parts

If `0 ≤ _stencil_box_id ≤ 4` (0x199964): `Draw_Main_Screen_While_Moving_Boxes_`,
`Clip_Off`, `Reset_Window` and skip to the lights. Otherwise, inside
`Set_Window(0x16,0x16,0x20f,0x1a5)` + `Clip_On`: full redraw
(`_draw_main_screen_to_back==1` or `_FULL_DRAW_SCREEN_FLAG`) =
`Draw_Main_Screen_Filled_(1)` (background entry 0, map layers, `Copy_Off_To_Back`,
`Print_Star_Names_`, `Print_Main_Screen_Data_`), else links + star names; then:

**Stardate print.** If settings+1 (0x199bdd; toggled as "End Of Turn Wait" by
`Toggle_Game_Option_`, default 1 at 0x127fb): `sprintf(H 0xda "Stardate:
%d.%d", sd/10, sd%10)`, `Set_Font_Style(1, {00,0a,0c,0e,10,12,14,16})` (8 bytes
at 0x81c78), `Print_Right(0x27f, 0, s)`. The clip is on, so `Print_Display`
uses `Clipped_Print_String` (0x122309), which drops a string with
`y > window_y2` or `y + height < window_y1`: at y=0 with window y1=0x16 it is
never drawn. Port it as a no-op unless the clipped printer is ported.

Then map drawing (not covered), boxes (below), `Clip_Off`, `Reset_Window`,
`Print_Main_Screen_Data_()`, `_multi_colored_line_start = (+1) % 8`. After
either branch: game type 2/3 → `Draw_Diplomacy_Request_Lights_`; 1/2/3 →
`Draw_Player_Done_Lights_`; `Check_Mouse_XY_Fields_`; `_FULL_DRAW_SCREEN_FLAG`
→ `_skip_fields=1`, flag 0; `pulse += incr; if pulse == 0 || pulse == 8: incr =
-incr` (so pulse runs 0..8..0, first frame 0).

`Draw_Main_Screen_Before_Stencil_` (0x849cc) draws the boxes, then
`Reset_Animation_Frame`+`Draw` next button at (0x220,0x1b9) and game button at
(0xf9,5), then box `_stencil_box_id`.

## Print_Main_Screen_Data_ (0x87bae)

Drawn unclipped, full window, every `Draw_Main_Screen_` and once more inside
`Draw_Main_Screen_Filled_` (after `Copy_Off_To_Back`, so never on the back page).
`p = _player + _PLAYER_NUM*0xea9`. Constants: `L = 0x224`, `W = 0x3e`, centre
`CX = L + W/2 = 0x243`; fit start style 2, spacing 2, max width 0x43 (strict
`<`), min style 1 (0 for CP); outline shadow 4. `pulse` = 0x199f07.

Helpers (game, `MOX\` module 55 and `mainscrn`):

- `Font_Colors2_(style, c0, c1)` (0x8e5c5) → static `_mox_font_colors`
  (0x19c31c): zero 8; style 0–1: `[0]=c0, [1..7]=c1`; 2–3: same, then
  `[2] = _mox_font_colors_offset (0x199f34) ? it : min(c1, c0+1)`; 4: same
  fill, `[2] = offset ? it : c1-1`; 5: `[0]=0, [1]=c0, [2..4]=c1-1,
  [5..7]=c1`; >5: all 0. Always resets the offset to 0. (`mainmenu.go` has
  only modes 0–1.)
- `Get_Mox_Font_Colors_(style,c0,c1,dst)` (0x8f6f8): copy of the above.
- `Set_Fitted_Font_Style_(s, colors, style, sp, shadow, maxw, minst)`
  (0x8fda1): `shadow==-1 → 0`; loop { `Set_Font_Style(style, colors)`,
  `Set_Font_Spacing(sp)`, `shadow_flag = shadow`; if `width(s) < maxw` stop;
  `sp--`; if `sp < 1` { `style--`; `sp = start sp`; if `style < minst` stop } }.
  When it gives up, the font is left at `minst` with spacing 1.
- `Set_Post_Fitted_Font_Style_(c0,c1)` (0x88b0a): keep current style and
  spacing; `Get_Mox_Font_Colors_(style,c0,c1,tmp)`, `Set_Outline_Color(2,1)`,
  `Set_Font_Style_Outline(style,tmp)`, `Set_Font_Spacing(spacing)`.
- `Increase_Font_Color_Range_(n)` (0x8f7df): `current[1..7] += n`, clamp 255;
  copy to normal and current (shade 0 unchanged). Callers pass a second value
  in EDX that it ignores.
- `Get_Comma_String_(v)` (0x8e6de) → static `_temp_string` (0x18ff78):
  `sprintf("%ld", |v|)`, len L, `k = (L-1)/3` separators (',' or ' ' for lang
  1); built right to left with a position counter c from 0 at the NUL: c%4==0
  && c>1 && k>0 → separator; v<0 → prefix '-'. Callers pass the s16 fields
  sign-extended, +0x32 as s32.
- `Set_Font_Style` resets spacing to the style's own and shadow to 0;
  `Set_Font_Style_Outline` = `Set_Font_Style` + shadow 4; `Get_Font_Height` =
  style height (not line feed); `Get_Font_Spacing_`/`Get_Font_Style_` read the
  current values. Watcom keeps EBX/ECX/EDX across calls (also `sprintf_`),
  which several steps below rely on.

Two colour buffers persist through the routine: `A` (ebp-0xe) and `B`
(ebp-0x16). `sign(v)` is "" if v<0 else "+".

1. **Stardate**: sep = '.' (',' for lang 1/2); `s = sprintf("%d%c%d", sd/10,
   sep, sd%10)`; `Set_Fitted(s, A(uninitialised), 2, 2, -1, 0x43, 1)`; `st` =
   style; `A = Font_Colors2_(st, 0x0e, 0x17)`; `Set_Outline_Color(2,1)`;
   `Set_Font_Style_Outline(st, A)` (spacing back to the style default — the
   fitted spacing is lost); `Print_Centered(0x244, 0x1d, s)`.
2. **Hyperspace flux** if `Event_Check_Hyperspace_Flux_()` (0x233fa: event
   state 0x19abf6 ∈ {2,4,6}): `B = Font_Colors2_(3, 0x49, 0x4c)` =
   {49,4c,4a,4c,4c,4c,4c,4c}; `Set_Font_Style(3, B)`;
   `Increase_Font_Color_Range_(pulse*5/9)`; t = H 0xe0; `x0 = 0x20e -
   width(t)`; `Line(x0,0x15,0x20e,0x15,B[0])`, `Line(x0,0x16,…,B[1])`,
   `Print_Right(0x20e, 0x18, t)`, `h = Get_Font_Height`, `Line(x0,h+0x19,0x20e,
   h+0x19,B[1])`, `Line(x0,h+0x1a,…,B[0])`.
3. **Colours for the income style**: `s = sprintf("%s%s %s", sign(inc),
   comma(inc), "BC")` with `inc = p+0xb2` (net income); `Set_Fitted(s, A, 2, 2,
   4, 0x43, 1)`; `st`; `B = colors(st,0x4b,0x4d)`; `A = colors(st,0x0e,0x17)`.
   Nothing printed.
4. **Reserve** (`p+0x32` s32): `s = sprintf("%s %s", comma(bc), "BC")`. bc<0:
   `Set_Fitted(s, B, …,1)`, `Set_Post_Fitted(0x4b,0x4d)`,
   `Increase((pulse<<2)/7)`, `Set_Outline_Color(0x49,0x49)`. Else
   `Set_Fitted(s, A, …,1)` (colours stay those of the income style, not
   recomputed), `Set_Outline_Color(2,1)`. `Print_Centered(0x243, 0x60, s)`.
5. **Income**: same `s` as step 3; `Set_Fitted(s, A, 2,2,4,0x43,1)`; `st`,
   `sp`; `B = colors(st,0x4b,0x4d)`, `A = colors(st,0x0e,0x17)`. inc<0:
   `Set_Outline_Color(0x49,0x49)`, `Set_Font_Style_Outline(st,B)`,
   `Set_Font_Spacing(sp)`, `Increase((pulse<<2)/7)`; else
   `Set_Font_Style_Outline(st,A)`, `Set_Outline_Color(2,1)`,
   `Set_Font_Spacing(sp)`. `Print_Centered(0x243, Get_Font_Height()+0x60, s)`.
6. **Command points**: `d = (s16)(p+0x3a − p+0x3c)`, `tot = p+0x3a`;
   `s = sprintf("%s%d (%d)", sign(d), d, tot)`; `Set_Fitted(s, A, 2,2,4,0x43,`
   **`0`**`)`; `st`, `sp`; `B`, `A` recomputed for `st`. `s1 = sprintf("%s%d",
   sign(d), d)`, `w1 = width(s1)`; `s2 = sprintf("(%d)", tot)`, `w2 = width(s2)`;
   `x = 0x224 + 2 + 0x1f − (w1+4+w2)/2`. d<0: `Set_Font_Style_Outline(st,B)`,
   `Set_Outline_Color(0x49,0x49)`, `Set_Font_Spacing(sp)`,
   `Increase((pulse<<2)/7)`; else outline with `A`, outline colour (2,1),
   spacing. `Print(x, 0xb5, s1)`; `Set_Font_Style(st, A)` (no outline),
   `Set_Font_Spacing(sp)`, `Print(x + w1 + 4, 0xb5, s2)`. `hcp =
   Get_Font_Height()` is kept for step 9.
7. **Food** (`p+0xb0`): `s = sprintf("%s%s", sign(f), comma(f))` (no unit).
   f<0: `Set_Fitted(s, B, …,1)`, `Set_Post_Fitted(0x4b,0x4d)`,
   `Set_Outline_Color(0x49,0x49)`, `Increase((pulse<<2)/7)`; else
   `Set_Fitted(s, A, …,1)`, `Set_Post_Fitted(0x0e,0x17)`,
   `Set_Outline_Color(2,1)`. `Print_Centered(0x243, 0xff, s)`.
8. **Freighters**: `fr = p+0x38` (free), `tot = p+0x36`; `s = sprintf("%s%d
   (%d)", sign(fr), fr, tot)`; `Set_Fitted(s, B, …,1)`; `st`; `B, A`
   recomputed; `sp`; `wf = width(s) + 4` (whole string incl. the space);
   `s1 = sprintf("%s%d", sign(fr), fr)`. fr<0: outline with `B`, spacing,
   `Increase((pulse<<2)/7)`, `Set_Outline_Color(0x49,0x49)`; else outline with
   `A`, spacing, `Set_Outline_Color(2,1)`. `x = 0x224 + 2 + 0x1f − wf/2`;
   `Print(x, 0x14a, s1)`; `Set_Outline_Color(2,1)`; `w1 = width(s1)`;
   `s = sprintf("(%d)", tot)`; `Set_Font_Style_Outline(st, A)`, spacing;
   `Print(x + w1 + 2, 0x14a, s)`. (CP and freighters differ: CP's total has no
   outline and a 4-pixel gap, freighters' has an outline and a 2-pixel gap and
   is centred on the full string.)
9. **Research** (centre 0x243, base y `Y = 0x193`), unit "RP":
   - `p+0x30` (breakthrough pending) ≠ 0: `Set_Outline_Color(2,1)`,
     `Set_Font_Style_Outline(0, A)` (style 0, colours of the freighter style),
     `Print_Centered(0x243, Y, H 0x183 "Breakthrough")`.
   - else `p+0x321` (research field) == 0: `Set_Outline_Color(2,1)`,
     `Set_Fitted(<the "(tot)" freighter string still in the buffer>, A,
     2,2,4,0x43,1)`, `Set_Post_Fitted(0x0e,0x17)`, `Print_Centered(0x243, Y,
     H 0x188 "none")`.
   - else `n = Player_N_Turns_Until_Research_Complete_(p)` (0xe1f24):
     - n > 0: `c = Chance_For_Research_Breakthrough_(p)`; if c > 0:
       `sprintf("%d%%", c)`, fit (A,…,1), post (0x0e,0x17), outline (2,1),
       `Print_Centered(0x243, Y − 2·height, ·)`. `t = sprintf(H(n==1 ? 0xe1 :
       0xe2), n)`; fit `sprintf("@%s", t)` (A,…,1), post (0x0e,0x17); `y = Y −
       height`; outline (2,1); `x = (0x3e − width(t))/2 + 0x224` (C division);
       `Print(x, y+3, "@")`; `Print(x + width("@"), y, t)`; then `y += height`;
       `sprintf("%s %s", comma(p+0xac), "RP")`, fit, post, outline,
       `Print_Centered(0x243, y, ·)`.
     - n < 0: `"0 RP"`, fit, post, outline, `Print_Centered(0x243, Y, ·)`.
     - n == 0 (field already researched): `c = Chance…`; `sprintf("%d%%",c)`
       at `Print_Centered(0x243, Y−0xc)`; then `sprintf("%d %s", p+0xac, "RP")`
       (no commas) at `Y − 0xc + hcp`.

`Player_N_Turns_Until_Research_Complete_(p)`: field `f = p+0x321`; status
`p+0xc4+f == 3` → 0; `rp = p+0xac`, `cost = Player_Research_Cost_(p,f)`
(0xe1e96: f ≤ 0 → 0; `cost = dword[0x17d916 + f*0x17]`; f ≥ 0x4b adds
`p[0x1d1+f]*10000`), `acc = p+0x1eb`; rp == 0 && acc ≤ cost → -1; else
`n=0; while acc ≤ cost {n++; acc+=rp}; sum=0; while sum < 100 {acc+=rp; n++;
sum += Aux(acc, rp, cost)}`; return n. `Chance_For_Research_Breakthrough_(p)` =
`Aux(acc+rp, rp, cost)`; `Aux(t,_,cost)`: cost ≤ 0 or cost ≥ t → 0; else
`(t−cost)*100/cost` clamped to 1..100.

Player fields read: +0x30, +0x32 (s32), +0x36, +0x38, +0x3a, +0x3c, +0xac,
+0xb0, +0xb2, +0xc4[f], +0x1d1[f], +0x1eb (s32), +0x321. All are in the save
record; `Main_Screen_` does not recompute them on entry (only after the tax
popup), so after `Load_Game_` the saved values are what is printed.
`Do_Player_Colony_Calculations_(pl)` (0xe2d72): `Mox_Update_`; for colonies
from last to first with owner (colony+0) == pl and colony+6 == 0:
`Pre_Import_Computing_(c)`; then `Do_Player_Colony_Post_Production_(pl)`
(0xe2d09: `Pass_Out_Imports_`, which rewrites +0x38 and +0xb0, `Colony_Pop_Grows_`,
`Colony_Specialty_`, …). Its full effect belongs to the economy spec.

## Main_Screen_Text_Box_ (0x87644)

`Set_Refresh_Full`, `Text_Box_(msg)` (game 0x95eb2, not covered),
`Set_Page_Off`, `Fill(0,0,0x27f,0x1df,0)`, `_draw_main_screen_to_back=1`,
`_current_screen==0 ? Draw_Main_Screen_ : Draw_Mini_Main_Screen_`, flip,
`Set_Refresh_Stencil`.

## Moveable boxes

`Moveable_Box_Selected_(i)` (0x918d5): `_box_off_flag[i] < box[i].content`
(signed 16-bit). After a load (`Harold_Defaults_`) the flags are
{-1,0,0,0,0}; in all ten shipped saves box 0 has content -1 and boxes 1–4
content 0, so **every box is closed after loading a shipped save**.

`moveable_box_order` is drawn from index 4 down to 0 (`Draw_Main_Screen_`), so
index 0 is the **top**; `Move_Box_To_Top_` (0x9128c) moves a box to index 0
keeping the others' order. (`save-ships.md` says the first is the bottom; that
is backwards.)

`Draw_Moveable_Box_(i)` (0x91099): only if open; if
`_skip_draw_system_display` → clear it and return. By `box[i].id`: 0 →
`Wrapper_For_Normal_System_Popup_` (0xa4fbe: title `sprintf(H 0x16a, …star…)`,
`Init_System_Display_(star)`, `Draw_System_Display_Popup_(star, 1, title, x, y,
…, closed_w?, 0xd, 0x2f)`); 1 → `Draw_Galaxy_Map_Box_(pict[i], pict2[i], x, y,
_galaxy_map_width, _galaxy_map_height, 8, 0x26, closed_w, closed_h, 0, 0)`
(`_moveable_box_pict` 0x193070, 0x14 per box); 2 → `Draw_Fleet_Movement_Box_`
(0x70875); 3, 4 → nothing. Closed boxes draw nothing at all.

In `Draw_Main_Screen_`: first pass i=4..0 over the order: box 0 open → highlight
its star name (and step `_selected_star_color_offset` by `_mod`, reversing at 3
and -1); box 2 open → `Draw_Fleet_Box_Selected_Ship_Seg_`. Then the scanned
ship. Second pass i=4..0: open and ≠ `_stencil_box_id` → if a mouse button is
held and (`Auto_Input() == box.drag_field` or `box.dragging == 1`)
`Move_Moveable_Box_(box, button)`; `Draw_Moveable_Box_(box)`.

`Add_Moveable_Box_Fields_` (0x90430): window (0x16,0x16,0x20f,0x1a5) + clip;
for each box in order: `Clear_Moveable_Box_Fields_(id)` (drag_field, field2,
field3 = -1000, plus planet buttons for 0, galaxy map fields for 1, fleet box
fields for 2); box 0 open → `Wrapper_For_Normal_System_Fields_`; box 2 open →
`Node_Ptr_From_Ship_Stack_(_fleet_box_ship_stack)`: -1 →
`Vanish_Fleet_Movement_Box_`, else `Add_Fleet_Movement_Box_Fields_(node,
0x193020 == 0)`. `Reset_Window`, `Clip_Off`.

`Scan_Moveable_Box_Fields_(skip, r, scanned, &leave)` (0x89183), for i 0..4
open: `r == box.field2` (close) and !skip → box 0: draw + flip +
`Vanish_System_Display_Box_` (content = off flag, planet/ship fields -1000);
box 2: clear selections, `_draw_main_screen_to_back=1`,
`Vanish_Fleet_Movement_Box_`; others: content = off flag. Else: !skip and
`order[0] != i` and `Box_Field_Selected_(i, r)` (drag_field, field3, the
galaxy map grid for box 1, fleet bar/all for box 2) → `Move_Box_To_Top_`; box
id 1 → galaxy map scan/centre; box 2 → fleet box buttons (scroll, all,
colonise 0x1e, outpost 0x1f, transport 0x22, engage, attack Antares with
messages 0xe4–0xe6). Detail of boxes 0–2 belongs to the system display,
galaxy map and fleet box specs.

Init (new game only, `Init_Moving_Boxes_` 0x79756 from `Harold_Init_`): zero,
ids 0..4, order 0..4, x = y = -1, content = off flag;
`Init_Fleet_Movement_Box_` (w 200, h 0x110, x = y = 0x16, closed 200×0x27);
`Init_System_Display_Box_` loads **BUFFER0.LBX entry 0** (640×480) into
`_moveable_box_pict` and copies its size into box 0's width/height, x,y = 0x16
if negative. That is why SAVE10 has box 0 at 640×480; the other saves have
347×273, so something later resizes it (not traced).

## Lights (multiplayer only)

`Draw_Player_Done_Lights_` (0x8520c), game types 1–3: `n =
Get_N_Remaining_Humans_()` (players with +0x28 == 100 and +0x24 == 0); `done`
= number of players with `Hotseat_Player_Status_(i) == 1` (type 1: human,
alive, +0x2d ≠ 0; 2 means not done) or, types 2/3, `0x1aaf7e[i] ≠ 0`. Then for
each such player in order, k = 0,1,…: `light = _player_lights_seg[p+0x26]`; if
`done < n − 1` → `Set_Animation_Frame(light, 5)` (else it animates);
`Draw(0x49 + k·width(_player_lights_seg[0]), 5, light)`. Relevant for SAVE10
(hotseat).

`Draw_Diplomacy_Request_Lights_` (0x83d06), types 2/3: bits of
`Humans_Requesting_Diplomacy_()`; for each set player k-th: save the light's
frame, set frame `_diplo_lights_anim_frame[colour]` (0x19c148), `Draw(0x1fa −
k·width0, 5, light)`, advance that counter modulo `total_frames`, restore the
frame.

## Engine primitives needed

| Primitive | In `internal/engine` / `internal/font`? |
|---|---|
| Set_Font_Style, Set_Font_Style_Outline, Set_Outline_Color, Print, Print_Centered, Print_Right, Get_String_Width, Get_Font_Height | yes (`Printer.SetStyle`, `SetStyleShadow(Outline)`, `SetOutlineColor`, `Text.Print*`, `Width`) |
| Set_Font_Spacing / Get_Font_Spacing (`current_horizontal_spacing` overriding the style's) | **no**: `Font.Width`/`drawChar` use `Style.HSpacing`; needs a current spacing in `Printer` |
| `font_header.shadow_flag` set directly (by Set_Fitted_Font_Style_) | partly (`Printer.Shadow` is public) |
| Clipped printing (`Clipped_Print_String`, `Print_Clipped_Character`) and Set_Window/Reset_Window/Clip_On/Clip_Off, Save/Restore_Window_Status | **no** |
| Line | **no** |
| Draw, Reset_Animation_Frame, Set_Animation_Frame | Draw yes; frame setters via `Picture.CurrentFrame` |
| Fill, Set_Page_Off, Copy_Off_To_Back, Toggle_Pages, Set_Refresh_* | yes |
| Field types 0 (button), 8 (multi hot key), 0xc (grid), 0xd (irregular button); Add_*_Field; Draw_Field and Draw_Visible_Fields for them; Get_Alt_Alpha_Key_; Check/Clear_Multi_Hot_Key | **no** (only type 7) |
| Clear_Fields_Above | **no** (spec'd in fields-and-input.md) |
| Disable_Cancel, Enable_Mouse_Pressed_Field_Update, Disable/Enable_Draw_All_Fields, Set_Help_List, Deactivate_Help_List, Set_Mouse_List, Set_Input_Delay, Assign_Auto_Function | yes (`MouseCancelDisabled`, `DynamicFieldRefresh` + buffer clear) |
| Auto_Input (returns `auto_input_variable`), Set_Field_Outline_Color | no (trivial) |
| Farload_Data_Static (record entry) | `internal/lbx` reads entries; the 4-byte record header must be skipped |
| Game helpers: Font_Colors2_ (modes 2–5), Get_Mox_Font_Colors_, Set_Fitted_Font_Style_, Set_Post_Fitted_Font_Style_, Increase_Font_Color_Range_, Get_Comma_String_, unit strings, H_Message_/Load_H_Strings_, User_Box_, Text_Box_ | no |

## Open questions

1. `Draw_Field` type 0/0xd prints its string only when `*string == '\0'`
   per Ghidra (the up path in `Draw_Visible_Fields` tests `!= '\0'`); all main
   screen strings are "", so it does not matter here; check the assembly
   before porting buttons with labels.
2. BUFFER0 entry 13 is 1×1 yet drawn at (0x4a,0)/(0x19f,0) in multiplayer;
   maybe a placeholder. Not verified against the original.
3. What resizes box 0 from 640×480 to 347×273 (probably
   `Draw_System_Display_Popup_`).
4. Settings+1 gates the (invisible) stardate print and is named "End Of Turn
   Wait" by the option toggle; the name in `save-misc.md` may be off by one.
5. The '@' glyph before the turns count is presumably an icon in the font;
   not rendered yet.
6. `_add_mainscrn_debug_field` (0x19c190) writers not traced (assumed 0).
