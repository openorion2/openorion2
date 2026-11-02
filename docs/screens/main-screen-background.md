# Main screen: background, frame, starfield, nebulas

The part of the galaxy screen (screen 0, `Main_Screen_` 0x86188) that is
not map content: the pictures it loads, the palette, the fade in after a
load, the parallax starfield, the frame with its buttons, the nebulas and
the map window. Source: `MOX\` modules 52–54 (`wsyms`), engine `WLIBS`
`animate.c`/`clipped`/`window`. All addresses are ELF addresses. Map content
(stars, names, links, ships, boxes, `Print_Main_Screen_Data_`) is out of
scope and only named where it is called.

## Pictures

All are single-frame unless noted; the palette used is FONTS.LBX entry 1
(`Load_Palette(0, …)`). Verified by rendering the pictures and dumping their headers.

| LBX | Entry | Size / kind | Use |
|---|---|---|---|
| STARBG.LBX | 0 | 640×480 sprite | far star layer (`_far_star_seg` 0x192e84) |
| STARBG.LBX | 1 | 640×480 sprite | mid layer (`_mid_star_seg`) |
| STARBG.LBX | 2 | 640×480 sprite | near layer (`_near_star_seg`) |
| STARBG.LBX | 3–5 | 640×480 sprite | not used by the main screen |
| STARBG.LBX | 6+4t+z | sprite (z 0–2), bitmap (z 3) | nebula picture type t = 0..11 at zoom level z |
| BUFFER0.LBX | 0 | 640×480 sprite, mask 0x10, palette chunk 0..191 | the frame (`_main_screen_seg` 0x192ee4): map hole, buttons, side panels |
| BUFFER0.LBX | 1 | 59×17, 2 frames | GAME button (`_main_game_button_seg` 0x192eec) |
| BUFFER0.LBX | 2 | 65×26, 2 frames | TURN button (`_main_next_button_seg` 0x192e7c) |
| BUFFER0.LBX | 3, 4, 5 | 239×52, 2 frames | COLONIES / PLANETS / FLEETS bar (whole left bar, one button lit) |
| BUFFER0.LBX | 6, 7 | 55×16, 55×19, 2 frames | zoom in / zoom out |
| BUFFER0.LBX | 8, 9, 10 | 237×52, 2 frames | LEADERS / RACES / INFO bar |
| BUFFER0.LBX | 11 | 54×53 | `_fleet_box_selection_seg` |
| BUFFER0.LBX | 12 | 65×26 | darkened TURN (`_darkened_turn_button_seg` 0x1933f0) |
| BUFFER0.LBX | 13 | 1×1 | network/hotseat lights placeholder (game types 1–3 only) |

Button pictures: frame 0 = normal look (identical to the frame art), frame 1
= lit. The 1×1 entry 13 is drawn for both light kinds; it is only reached for
`_game_type` 1, 2, 3.

The colour chunk of BUFFER0 0 equals FONTS palette 0 colours 0..191 exactly,
with all flag bytes 0. `Set_Animation_Palette` compares the flag byte too, so
an entry whose `changed` is 1 is rewritten as `{1, same rgb}`; there is never
a visible colour change.

No sprite run in STARBG 0–2, 6–8, 10 or BUFFER0 0/12 contains a 0 byte, so the
unclipped/clipped transparency difference (below) has no effect on them. The
zoom-level-3 nebula bitmaps do contain 0s (transparent in both paths).

## Load_Main_Screen_Pictures_ (0x131c2)

Called once at start-up: `main2_` → `Load_Pictures_` (0x13174) →
`Load_Main_Screen_Pictures_`, then `Load_Colony_Anims_`, `Init_Tech_`, and
`_screen_seg = Allocate_Next_Block(_global_screen_seg, remaining - 12)` with
its first 0x14 bytes after +0xc zeroed. Every picture goes into
`_global_screen_seg` (0x192ef4) with `Far_Reload_Next` (append; never freed),
so the port loads them once and keeps them. Order (BUFFER0.LBX unless noted):

1. Entries 1..12 → game, next, colonies, planets, fleets, zoom in, zoom out,
   leaders, races, info buttons, fleet box selection, darkened turn button.
2. Entry 0x52 → `_moveable_box_pict`+0x10 (0x193080, system display popup
   button); then `0x1930a8 = 0`.
3. Entries 0x3a..0x48 → fleet box buttons: scroll up, scroll down, all,
   att-xenons lg, colonize lg, engage lg, transport lg, outpost lg, close lg,
   att-xenons sm, engage sm, colonize sm, transport sm, outpost sm, close sm.
4. Entries 0xf1..0x108: for j = 0..5, i = 0..3, entry `0xf1 + 4j + i` is
   loaded **twice**, into `_hl_monster_ship_icons[j][i]` (0x190518) and
   `_monster_ship_icons[j][i]` (0x190598). (The entry counter starts at
   0xcd and is advanced by 36 in empty loops left over from removed code.)
5. `_star_width = _star_height = 0x21`; `gstar.lbx` entries 1..11 →
   `_galaxy_map_star_seg[0..10]` (0x197490).
6. `_sys_data_buffer = Allocate_Next_Block(_global_screen_seg, 0xdc)`.
7. Entries 0x109..0x110 → `_player_lights_seg[0..7]` (0x19c128).

The `Get_Remaining_Block_Space` and `Mox_Update_` calls in between have no
visible effect.

## Load_Nebula_Pictures_ (0x8da07)

Called by `Load_Game_` (when `NUM_NEBULAS != 0`), `Init_New_Game_`,
`Init_New_Session_`, `Generate_Nebulas_`. For nebula n = 0..`NUM_NEBULAS`-1
(byte 0x199f0d), for type t = 0..11, for z = 0..3: if `nebula[n].picture ==
t`, load STARBG.LBX entry `6 + 4t + z` into `_nebula_pict_seg[n][z]`
(0x190298 + 16n + 4z). The first load of the call uses `Far_Reload` (resets
`_nebula_screen_seg`), later ones `Far_Reload_Next`. Pictures of nebulas that
share a type are loaded separately.

## Allocate_First_Screen_Seg_ (0x82348)

`_other_ships_screen_seg = Allocate_First_Block(_screen_seg, 76000)`;
`_global_bitmap_seg = Allocate_Next_Block(_screen_seg, Get_Bitmap_Size(60,60))`
where `Get_Bitmap_Size(w,h) = ((w*h + 0x13) / 4) * 4` = 3620. Callers pass an
argument (5) that is ignored. Memory only: it resets the `_screen_seg`
sub-allocator, so anything previously loaded there is dead. `Main_Screen_`
does the same two allocations inline at entry. Nothing to draw in the port.

## Palette

- `Load_Palette(n, a, b)`: FONTS.LBX entry n+1, colours a..b, sets
  `changed` (ported, `engine.LoadPalette`).
- `Main_Screen_` on its non-fade path: `Load_Palette(0, 0, 0xad)` — colours
  0..173 only. 174..255 keep whatever was loaded (ship palettes of the fleet
  box, `Load_Player_Ship_Palette_`).
- `Fade_Into_Main_Screen_`: `Load_Palette(0, 0, 0xff)`.
- `_mainscr_palette` (byte 0x199f1d) is **not** a palette number but a marker
  of which ship palette sits in the upper colours: 0xff `Harold_Defaults_`
  and `Main_Screen_Report_Handler_`; 0x65 when leaving `Main_Screen_`,
  `Do_Next_Turn_`, `Set_Reload_Fleet_Box_Palette_Flag_`; 2
  `Reload_Fleet_Box_Palette_`; 0 `Load_New_System_Display_`. Only
  `Do_Fleet_Popup_` reads it (`== 2`: skip reloading the fleet palette).
- `star_bg_index` (0x199f0a, `Harold_Defaults_` = 2, diplomacy screens = 0)
  is read only by `Buffer_Reload_Star_Bg_` (0x81cb4, BUFFER0.LBX entry
  index+3), which has **no callers**. The starfield does not depend on it.
  (`docs/formats/save-stars.md` calls it the main screen background; that is
  wrong.)

### Is_Main_Screen_Palette_ (0x860c3)

Returns 1 if `_previous_screen` (byte 0x199f19) is one of 0x00, 0x08, 0x0b,
0x0c, 0x1c, 0x1e, 0x1f, 0x23, 0x24, 0x27, 0x28, 0x2b; else 0. Screen
Control writes `_previous_screen` after each screen returns (Main 0, Reports
0x27, Main menu 10, …).

## Flow after Load_Game_

`Screen_Control_` runs `Reports_Screen_` (0x8b956, previous screen 10),
then `Main_Screen_` (previous screen 0x27).

### Reports_Screen_ (0x8b956), the background part

1. `_current_screen = 0`; `r = Must_Display_Reports_(PLAYER_NUM)`; if r:
   mouse list arrow. `Set_Refresh_Stencil`; auto function
   `Draw_Mini_Main_Screen_`, delay 1.
2. `fade` = false. If `_stardate_at_beg_of_turn == 0` (always after a load,
   `Harold_Defaults_`): fade = true unless r ∈ {2, 3, 5, 6} or (r == 7 and
   settings+9 "Show GNN Report"). Otherwise: fade = true only if
   `_previous_report == 3` and r != 3.
3. If r == 3 and previous screen != 10: fade = false; else if
   `_at_least_one_tactical_combat` and r != 0: fade = true.
4. If fade and `!_already_faded_into_main_screen`:
   `_at_least_one_tactical_combat = 0`, `Allocate_First_Screen_Seg_`,
   `Clear_Fields`, `Clear_Main_Screen_Fields_`, `Update_Star_Owners_`,
   `Build_Ship_Icons_`, `Fast_Fade_Out_`, `Add_Map_Fields_`,
   `Fade_Into_Main_Screen_`.
5. `Do_Begin_Of_Turn_` (0x8ad82; its `Main_Screen_Report_Handler_` 0x8b17b
   fades into the main screen itself when reports are shown and it has not
   faded yet: `Fade_Into_Main_Screen_` if `!Is_Main_Screen_Palette_()`,
   else `Draw_Mini_Main_Screen_` + flip). Then flags:
   `_main_screen_fade_in_flag = _at_least_one_report`; if box 2 (fleet) is
   selected and there was a report, `_force_main_screen_fade = 1` and
   `_reloaded_ship_palette = 1`; if r == 0, `_stardate_at_beg_of_turn =
   stardate`. `Deactivate_Auto_Function`.

So the first visible frame of a loaded game always comes from
`Fade_Into_Main_Screen_`.

### Fade_Into_Main_Screen_ (0x8a6c2)

1. `sys = Moveable_Box_Selected_(0)` (system box open).
2. If `_already_faded_into_main_screen` (0x19c18e): clear it and return.
3. `_main_screen_fade_in_flag = 0`, `_force_main_screen_fade = 0`,
   `_already_faded_into_main_screen = 1`.
4. `Disable_Draw_All_Fields`, `Set_Refresh_Stencil`, mouse list none,
   `Fast_Fade_Out_` (does nothing if already faded — after step 4 of
   `Reports_Screen_` it is).
5. `Load_Palette(0, 0, 0xff)`; `Darken_Palette_(100)` (DAC black, all
   `changed` = 1).
6. `Add_Map_Fields_`.
7. If sys: `Do_System_Popup_(moveable_box[0].star)` (s16 at 0x192fde) with
   settings+4 "Expanding Help" forced to 0 for the call and restored.
8. `Set_Page_Off`; `Draw_Mini_Main_Screen_`;
   `Clear_Palette_Changes(0,255)`; `_TOGGLE_PAGES_` — refresh is stencil and
   dump is full, so the whole new screen goes up while the DAC is black.
9. Mouse list arrow. If `_reloaded_ship_palette` and box 2 is **not**
   selected: `Set_Fade_Percent_(0)` only (no `Darken_Palette_`). Otherwise
   `Fast_Fade_In_` (DAC 100 → 0 in steps of 10 over the already shown
   picture; a real fade in, unlike `Fade_Into_Screen_`).
10. `_reloaded_ship_palette = 0`; `Enable_Draw_All_Fields`.

`Mox_Update_` calls between the steps are omitted.

### Main_Screen_ (0x86188), the background part

At entry: `c = Is_Main_Screen_Palette_()`; auto function
`Draw_Main_Screen_`, delay 1; the two `Allocate_First_Screen_Seg_`
allocations; …; `Set_Refresh_Stencil`; `_draw_main_screen_to_back = 1`
(0x199f28); `_zoom_count = Map_Scale_To_Zoom_Level_()`; fields cleared.
Then:

- if `!_force_main_screen_fade` and (c or (`!_main_screen_fade_in_flag` and
  `!_at_least_one_tactical_combat`)): if box 0 is selected, `_system_star_id
  = -1` + `Clear_System_Display_Loaded_Flag_`; `Load_Palette(0, 0, 0xad)`;
  if previous screen == 0x24: `Set_Page_Off`, `Draw_Mini_Main_Screen_`,
  `Copy_Off_To_Back`, flip, `Set_Refresh_Stencil`. `Add_Map_Fields_`.
- else `Fade_Into_Main_Screen_`, `_draw_fleet_screen_to_back = 1`.

After a load c = 1 (previous 0x27), so only the palette is reloaded; the
first loop pass draws. Loop (per pass): `Mark_Time_`, `Get_Input`, … ,
`Add_Map_Fields_`, `Set_Page_Off`, `Draw_Main_Screen_` (or
`Draw_Main_Screen_For_Zooming_` while zooming), `_TOGGLE_PAGES_`,
`Release_Time_(1)`.

## Draw_Main_Screen_ (0x84555), background part only

If `_stencil_box_id` is 0..4: `Draw_Main_Screen_While_Moving_Boxes_` (not
covered). Otherwise `Set_Window(0x16,0x16,0x20f,0x1a5)`, `Clip_On`, then:

- if `_draw_main_screen_to_back == 1` or `_FULL_DRAW_SCREEN_FLAG`
  (0x19994c): `Draw_Main_Screen_Filled_(1)`, `_draw_main_screen_to_back =
  0`;
- else `Draw_Wormhole_Links_`, `Draw_Relocation_Links_`, `Print_Star_Names_`
  (on top of the background restored from `back` by the stencil).

Then the map window again and the map content, ending with `Clip_Off`,
`Reset_Window`, `Print_Main_Screen_Data_`. So the background is drawn once
into `off` and copied to `back`; every later frame relies on
`Stencil_Out_Background` restoring it.

## Draw_Main_Screen_Filled_(to_back) (0x84a95)

1. `Reset_Window`; `Clip_Off`; `Fill(0,0,0x27f,0x1df,0)` (marks the whole
   screen in stencil mode).
2. `Set_Window(0,0,0x27f,0x1df)`; `Clip_On`.
3. `Draw_Paralax_` (below).
4. `Draw(0,0, BUFFER0.LBX 0)` — the frame. Inside the full-screen window,
   so the unclipped path: palette chunk, square (0,0,639,479), sprite blit.
5. `Draw_Wormhole_Links_`, `Draw_Relocation_Links_` (map content; drawn
   with the full-screen window still set).
6. Game type 2 or 3: `Draw(0x19f, 0, BUFFER0 13)`. Game type 1, 2 or 3:
   `Draw(0x4a, 0, BUFFER0 13)`.
7. `Clip_Off`, `Reset_Window`, `Set_Window(0x16,0x16,0x20f,0x1a5)`, `Clip_On`,
   `Draw_Nebulae_`, `Clip_Off`, `Reset_Window`.
8. If `to_back == 1`: `Copy_Off_To_Back`.
9. `Print_Star_Names_`, `Print_Main_Screen_Data_` (not in `back`).

Note the nebulas are drawn over the frame-time links, and the links before
the nebulas are clipped only to the screen.

## Draw_Paralax_ (0x8500f)

`Save_Window_Status_`; `Set_Window(0x16,0x16,0x20f,0x1a5)`; `Clip_On`; for
each layer L in far (STARBG 0), mid (1), near (2), with offset (lx, ly):

```
Draw(lx,       ly,       L)
Draw(lx - 640, ly,       L)
Draw(lx - 640, ly - 480, L)
Draw(lx,       ly - 480, L)
```

then `Restore_Window_Status_`. Offsets are 0..639 / 0..479, so the four
copies tile the window; copies wholly outside it are skipped by `Draw`.
Globals (int16): `_far_star_x/y` 0x199980/0x19997e, `_mid_star_x/y`
0x19998a/0x19998e, `_near_star_x/y` 0x199984/0x199986. `Harold_Defaults_`
zeroes all six (so after a load only the first copy of each layer is
visible); leaving zoom mode on the whole-screen field sets all six to 0x16.

## Get_New_Paralax_XYs_(x, y, div) (0x8590a)

EAX, EDX, EBX; all int16. `dx = (x - cur_map_x) / div`, `dy = (y -
cur_map_y) / div` (32-bit C division). `sx = -1 if (int16)dx > 0, 1 if < 0,
else 0`; same `sy`. Then:

- if `x != cur_map_x`: `p = sx*dx` (= -|dx|); `far_x -= p`, `mid_x -= 2p`,
  `near_x -= 4p` (16-bit);
- if `y != cur_map_y`: `q = sy*dy`; `far_y -= q`, `mid_y -= 2q`, `near_y -=
  4q`;
- each of the six: if negative, `+= 640` (x) or `+= 480` (y) once; then `%=
  640` / `%= 480` (16-bit signed `idiv`).

**Quirk:** the layers always move by +|d|, +2|d|, +4|d|, whatever the scroll
direction. Callers:

- `Center_Map_` (0x8238b): `(old_x, old_y, 1)` if the map moved.
- `Scroll_Map_` (0x89df9): when the move is longer than 50, in each of
  `steps = Range/50` animation frames `(target_x, target_y, steps)` before
  `Interpolate_Line_` moves `cur_map_x/y`, then `Draw_Main_Screen_Filled_(0)`
  + `Draw_Main_Screen_` + flip in refresh full; ends with
  `Draw_Main_Screen_Filled_(1)` and stencil. Shorter moves just set the
  position and `_draw_main_screen_to_back = 1`.
- `Main_Screen_` zoom out (0x873b7): `(new_x - old_x, new_y - old_y, 1)`
  after `cur_map_x/y` already hold the new values, so the delta used is
  `-old_x`, `-old_y` (bug, port as is).

## Draw_Nebulae_ (0x84f8f)

For n = 0..`NUM_NEBULAS`-1: `z = Map_Scale_To_Zoom_Level_()`;
`x = Get_Scaled_Value_(int16(neb[n].x - cur_map_x)) + 0x15`;
`y = Get_Scaled_Value_(int16(neb[n].y - cur_map_y)) + 0x15`;
`Draw(int16 x, int16 y, _nebula_pict_seg[n][z])`. Called with the map
window and clipping on.

- `Get_Scaled_Value_(v)` (0x7926c) = `(v*10) / cur_map_scale`, C division.
  `Get_Up_Scaled_Value_(v)` (0x7927f) = `(v*cur_map_scale) / 10`.
- `Map_Scale_To_Zoom_Level_` (0x79917): scale 10 → 0 if `max_zoom_count >=
  0`; 15 → 1 if `> 0`; 20 → 2 if `> 1`; any other → 3 if `> 2`; otherwise
  `max_zoom_count`.
- The map's screen origin is 21 (0x15), one pixel left/above the window
  (22): map column 0 is always clipped.

## Draw_Mini_Main_Screen_ (0x84e9d)

1. If `_at_least_one_tactical_combat`: clear it, `Fade_Into_Main_Screen_`.
2. Save `cur_map_x/y`, `cur_map_scale`, `zoom_count`.
3. `Draw_Main_Screen_Filled_(1)`, or `(0)` and clear the flag if
   `_dont_draw_main_screen_to_back` (0x199f37).
4. `Draw_Main_Main_Screen_` (0x8440e: map content in the map window, each
   step re-setting the window and `Clip_On`; ends `Reset_Window`,
   `Clip_Off`, `Print_Main_Screen_Data_`, diplomacy lights if game type ≠ 0).
5. `_multi_colored_line_start = (+1) % 8`; restore the four saved values.
6. If settings+1 "End Of Turn Wait" == 0 and `_check_next_turn_exit_field`
   (0x19c194) and `Mouse_Button()`: `_auto_turn_exit = 1`.
7. If `_check_next_turn_exit_field`: `Draw(0x220, 0x1b9, BUFFER0 12)`.
8. If `_FULL_DRAW_SCREEN_FLAG`: `_skip_fields = 1`, clear the flag.

## Redraw_Main_Screen_(full) (0x88d39)

Only caller `Do_Fleet_Popup_`.

- full == 0: `Set_Page_Off`, `Draw_Mini_Main_Screen_`, flip; if box 2 is
  selected and `Get_Stack_Start_From_Saved_Ship_Array_()`:
  `Reload_Fleet_Box_Palette_(owner, owner != PLAYER_NUM)`,
  `Draw_Main_Screen_`, flip; if the stack is gone,
  `Vanish_Fleet_Movement_Box_`.
- full != 0: `Fast_Fade_Out_`, `Set_Page_Off`, `Fill(0,0,0x289,0x1df,0)`
  (x2 = 649, clipping off: the fill wraps 10 pixels into each next row and
  past the last row into the buffer slack; all black, so no visible
  effect), flip, `Set_Refresh_Full`, `Draw_Main_Screen_Filled_(1)`,
  `Fast_Fade_In_`, `Set_Refresh_Stencil`, then the box-2 palette step as
  above without drawing. There is no flip between the draw and the fade
  in, so the fade runs over the black screen and the picture appears at the
  caller's next flip.

Related, same module: `Restore_Main_Screen_` (0x7e57c): `Set_Page_Off`,
`Draw_Mini_Main_Screen_`, `Copy_Off_To_Back`, `Set_Refresh_Stencil`,
`_FULL_DRAW_SCREEN_FLAG = 0`. `Darken_Main_Screen_Background_` (0x7da13):
`Set_Palette_Changes(0,255)`, `Update_Glass_Remap_Colors`,
`Set_Refresh_Full`, `Set_Page_Off`, `Draw_Mini_Main_Screen_`,
`Screen_Bitmap_Capture(0x16,0x16,0x20f,0x1a5,_temp_bitmap)`,
`_Darken_Fill_(0x16,0x16,0x20f,0x1a5)`, `Copy_Off_To_Back`,
`Set_Refresh_Stencil` (popups over the map).

## The frame's buttons (fields)

`Add_Map_Fields_` (0x81d73), outside zoom mode, adds:

| Field | Type | Rect / draw position | Picture | Hot key, sound |
|---|---|---|---|---|
| GAME | 0 button | (0xf9, 5) | BUFFER0 1 | 0x179719, 0x29 |
| TURN | 0 button | (0x220, 0x1b9) | BUFFER0 2 | 0x17971b, 0x28 |
| zoom in | 0 button | (0xf4, 0x1ac) | BUFFER0 6 | 0x179714, 0x28 |
| zoom out | 0 button | (0xf4, 0x1c7) | BUFFER0 7 | 0x179717, 0x28 |
| COLONIES | 0xd irregular | 0x11,0x1b2–0x4f,0x1d7; draw (0,0x1a9) | BUFFER0 3 | 0x17971d, 0x28 |
| PLANETS | 0xd | 0x5b,0x1b2–0x9a,0x1d7; (0,0x1a9) | BUFFER0 4 | 0x17971f |
| FLEETS | 0xd | 0xa7,0x1b2–0xe6,0x1d7; (0,0x1a9) | BUFFER0 5 | 0x179721 |
| LEADERS | 0xd | 0x138,0x1b3–0x17b,0x1d7; (0x132,0x1a9) | BUFFER0 8 | 0x179723 |
| RACES | 0xd | 0x182,0x1b3–0x1c5,0x1d7; (0x132,0x1a9) | BUFFER0 9 | 0x179725 |
| INFO | 0xd | 0x1ce,0x1b3–0x20e,0x1d7; (0x132,0x1a9) | BUFFER0 10 | 0x179727 |

plus hidden fields over the five side panels (x 0x221..0x265, y 0x31–0x77,
0x7a–0xc0, 0xc5–0x10b, 0x10e–0x154, 0x158–0x19f), the moveable box fields,
an optional debug field, a multi hot key field and the map grid field
`Add_Grid_Field(0x16,0x16,1,1,0x1fa,400,…)`. Zoom mode adds only the two
zoom buttons and a whole-screen hidden field.

`Draw_Visible_Fields` (0x11e718, run inside every flip): window reset, page
off; for each field: if it is the pushed field (`down_mouse_button`, types
other than 4/10/12) `Draw_Field(n, 1)` (0x11b05a, pressed look, not
covered); else type 0: `Reset_Animation_Frame` + `Draw(xmin, ymin,
picture)` — frame 0, which matches the frame art; type 0xd: nothing. Since
these draws mark the stencil list, the four type-0 buttons are redrawn on
every flip. In the port `drawVisibleFields` is still empty.

## Engine primitives

| Primitive | Addr | Behaviour | In `internal/engine` |
|---|---|---|---|
| `Set_Window(x1,y1,x2,y2)` | 0x128ab6 | x1,y1 clamped to ≥ 0; x2 ≤ 639, y2 ≤ 479; no order check. Globals int16 x1 0x1bba4c, x2 0x1bba4e, y1 0x1bba50, y2 0x1bba52 | fields `WinX1..WinY2` only, no function |
| `Reset_Window` | 0x128be7 | window = 0,0,639,479 | missing |
| `Save_Window` / `Restore_Window` | 0x128b49 / 0x128b98 | one saved slot `old_screen_window_*` | missing |
| `Save_Window_Status_(&s)` / `Restore_Window_Status_(s)` | 0x8fd71 / 0x8fd56 | Save_Window + `s = clipping_flag`; Restore_Window + Clip_On if s else Clip_Off (MOX) | missing |
| `Clip_On` / `Clip_Off` | 0x12b634 / 0x12b65c | `clipping_flag` (int16 0x1845d8) = 1 / 0 | field `Clipping` only |
| `Draw` clipped path | 0x12a478 | see below | missing (returns an error) |
| `Clipped_Draw_Animated_Sprite` | 0x1489f4 | see below | missing |
| `Clipped_Draw_Bitmap_Sprite` | 0x14861d | see below | missing |
| `Color_Stream_Copy_(src,dst,n)` | 0x148605 | copy n bytes, **skipping 0** (transparent) | missing |
| `Reset_Animation_Frame(p)` | 0x12b726 | `current_frame = 0` | missing (trivial) |
| `Set_Animation_Frame(p, n)` | 0x12b753 | used by field drawing | missing |
| `Set_Fade_Percent_(p)` | 0xc5b94 | `fade_percent = p`; if p == 0 mouse list arrow; no DAC change | missing (`fadePercent` field exists) |
| `Fill` (clipped and unclipped) | 0x128c32 | spec'd | present |
| `Load_Palette`, `Set/Clear_Palette_Changes`, `Darken_Palette_`, `Fast_Fade_Out_/In_`, `Copy_Off_To_Back`, refresh modes, `Toggle_Pages` | | spec'd | present |
| `Buffer_Reload(file, entry, cache)` | 0x127c27 | LRU cache of LBX entries in `_global_cache_seg` (0x193174), keyed by file (`stricmp`) and entry; returns the cached copy, so `current_frame` survives between calls unless evicted | missing; load once and keep is equivalent for these single-frame pictures |
| `Far_Reload` / `Far_Reload_Next` | 0x126afd / 0x126b42 | load an entry into a block (first / append) | not needed beyond loading |
| `Draw_Visible_Fields` drawing of button fields | 0x11e718 | above | missing (no-op) |
| `Screen_Bitmap_Capture(x1,y1,x2,y2,bmp)` | 0x12e148 | bitmap header (`Create_Bitmap_Header` 0x12d943) + copy block of `current_video_page` (`Capture_Screen_Block_` 0x14791b); swaps corners if x2 < x1 | missing (popups only) |
| `_Darken_Fill_(x1,y1,x2,y2)` | 0x90006 | clamp to window if clipping, then `Tint_Fill(…, 0)` (0x12991d) | missing (popups only) |
| `Update_Glass_Remap_Colors` | 0x133237 | builds `glass_remap_colors[16][256]` from `current_palette` via `Find_Closest_Color` (0x1338c9), slot 0 = 50 % black | missing (popups only) |

### Draw (0x12a478) with clipping

1. `x2 = x + w - 1`, `y2 = y + h - 1` (int16). If mask 0x10:
   `Set_Animation_Palette` — **before** any clip test, so a wholly clipped
   picture still applies its colours.
2. If `clipping_flag != 0`:
   - wholly outside (`x > wx2 || y > wy2 || x2 < wx1 || y2 < wy1`): draw
     nothing, mark nothing, but still advance the frame;
   - partly outside (`x < wx1 || y < wy1 || x2 > wx2 || y2 > wy2`): clipped
     path;
   - else the unclipped path (ported).
3. Clipped path, `dot_stencil_flag == 0`: if refresh > 1,
   `Add_Square_To_Scan_List(x, y, x2, y2)` with the **unclipped** rectangle
   (only clamped to the screen); no zero-size early return. Kind 0 →
   `Clipped_Draw_Animated_Sprite` (glassed variant 0x148c33 if mask 0x08);
   kind 1 → `Clipped_Draw_Bitmap_Sprite` (glassed variant if 0x08); kind 2
   and 3 → `Exit_With_Message`. With `dot_stencil_flag` set only kind 0 is
   drawn, by `Clipped_Draw_Stenciled_Animated_Sprite` (0x148f57).
4. Frame advance as in the unclipped path.

### Clipped_Draw_Animated_Sprite(x1, y1, frame) (0x1489f4)

```
cx1 = max(x1, wx1); cy1 = max(y1, wy1)
cx2 = (wx2 < x1 + w) ? wx2 : 639          // note: x1+w, not x1+w-1
cy2 = (wy2 < y1 + h) ? wy2 : 479
bottom = (y1 + h < cy2) ? y1 + h : cy2
y = y1 + int16(frame[2..3]); x = x1; src = frame + 4
line = dst = page + x1 + y*640
while y <= bottom:
    count, skip = s16 at src, s16 at src+2; src += 4
    if count == 0: line += skip*640; dst = line; y += skip; x = x1
    elif y < cy1 or x >= cx2: src += count          // x not advanced
    else:
        dst += skip; x += skip
        if x < cx1: d = cx1-x; src += d; dst += d; count -= d; x = cx1
        if x + count > cx2:
            if x != cx2 and cx2 - x >= 0: Color_Stream_Copy(src, dst, cx2-x+1)
        elif count > 0: Color_Stream_Copy(src, dst, count)
        x += count; src += count; dst += count
    if src is odd: src += 1
```

Quirks to keep: a run that starts exactly at `cx2` draws nothing (the
`x != cx2` test), so the window's right column can lose pixels; rows above
the window skip runs without advancing x (harmless, the row restarts on the
next zero record); runs are copied with 0 transparent while the unclipped
`Draw_Animated_Sprite_` (0x14852c) copies 0 bytes opaque. All frames seen
end with `(0, 1000)`, so the loop never reads past the frame.

### Clipped_Draw_Bitmap_Sprite(x1, y1, src) (0x14861d)

`xe = min(wx2, x1+w-1)`, `xs = max(x1, wx1)`, `ye = min(wy2, y1+h-1)`,
`ys = max(y1, wy1)`; source/destination advanced by the left and top clip;
for `ye - ys + 1` rows: `Color_Stream_Copy(src, dst, xe - xs + 1)`, src +=
w, dst += 640. Inclusive and exact (unlike the sprite version).

## Map window

`Set_Window(0x16, 0x16, 0x20f, 0x1a5)`: x 22..527, y 22..421 inclusive (506
× 400 = `0x1fa` × `400`, the grid field size). Set by `Draw_Paralax_`,
`Draw_Main_Screen_Filled_` (nebulas), `Draw_Main_Screen_`,
`Draw_Main_Main_Screen_` (before every content step), `Add_Map_Fields_`
(set and restored, no effect), `Darken_Main_Screen_Background_` (capture
rectangle). The frame picture's transparent hole is slightly larger (about
20..528 × 20..422), the 1–2 pixel ring between hole and window is only
covered by the black `Fill`.

## Open questions

1. `Fade_Into_Main_Screen_` with `_reloaded_ship_palette` set and the fleet
   box closed calls only `Set_Fade_Percent_(0)`: the DAC stays black (the
   `changed` flags were cleared before the flip) until some later palette
   change. Whether something else restores it on that path is not traced.
2. `Main_Screen_Report_Handler_` / `Do_Begin_Of_Turn_` decide whether the
   first frame comes from step 4 of `Reports_Screen_` or from the report
   handler; the result is the same `Fade_Into_Main_Screen_`, but the exact
   order of report popups belongs to the reports spec.
3. `Draw_Field(n, 1)` (pressed buttons, irregular bars) and
   `Get_Full_Store_Flag` are not specified here; mask bit 0x04, set on all
   button pictures, is not described in `docs/formats/pictures.md`.
4. The exact edges of the frame's transparent hole were read off a render,
   not measured.
5. `Init_Main_Screen_` (0x8cf09) decompiles oddly (jump into patch code at
   0x1693b6, see `save-stars.md`); not needed for a loaded game.
