# Main screen: stars on the galaxy map

Everything Main_Screen_ (screen 0) draws per star: map transform, star and
black-hole pictures, twinkle, gate icons, wormhole and relocation lines, star
names, the scanned-star caption. Addresses are ELF (`build/orion2.elf`,
debug address + 0x10000). Records: `docs/formats/save-stars.md` (star),
`save-planets.md` (colony), `save-players.md` (player), `save-ships.md`
(moveable boxes). Engine background: `docs/engine/video-and-timing.md`,
`docs/formats/fonts.md`, `docs/formats/pictures.md`.

Conventions: "int16" arithmetic is done in 32-bit registers on sign-extended
words and stored back as words; `/` is C division (truncates toward zero,
`idiv` or `cdq; sub eax,edx; sar eax` for `/2`).

## Globals

| Global | Addr | Type | Meaning |
|---|---|---|---|
| `_star` | 0x19306c | ptr | star records, 0x71 bytes |
| `NUM_STARS` | 0x19999a | int16 | |
| `PLAYER_NUM` | 0x19999c | int16 | viewing player |
| `cur_map_x`, `cur_map_y` | 0x19998c, 0x199990 | int16 | map origin (map units) |
| `cur_map_scale` | 0x199992 | int16 | 10, 15, 20 or 30 map units per 10 pixels |
| `max_map_scale` | 0x1999a0 | int16 | 10/15/20/30 by galaxy size |
| `max_zoom_count` | 0x19997c | int16 | 0..3 |
| `zoom_count` | 0x19999e | int16 | |
| `MAP_MAX_X`, `MAP_MAX_Y` | 0x199a0c, 0x199a0a | int16 | |
| `far/mid/near_star_x/y` | 0x199980/7e, 0x19998a/8e, 0x199984/86 | int16 | parallax offsets |
| `_star_fields_dim` | 0x1931ac | int16[6] | star picture size by `size+zoom`: 33, 29, 25, 23, 21, 17. Set by `Init_New_Game_` (0x12599) and `Harold_Defaults_` (0x79524); not in .data |
| `_star_seg` | 0x19330c | ptr[40] | cached star pictures, index `class*6 + zoom (+size)` |
| `_n_twinkling_stars` | 0x19c164 | int16 | see Twinkle |
| `_black_hole_anim_count` | 0x1992f2 | uint8[10] | per-black-hole animation counters; `Harold_Defaults_` (0x795f8) sets `[i] = 2*i`, i = 0..9. Slot 10 (used, see Draw_Black_Holes_) is the low byte of `_h_string_ix` (0x1992fc) |
| `_selected_star_color_offset` | 0x199f31 | int8 | highlight pulse, -1..3; `Harold_Defaults_` sets 0 |
| `_selected_star_color_mod` | 0x199f30 | int8 | ±1; `Harold_Defaults_` sets 1 |
| `_star_name_highlight_value` | 0x182146 | int8 | .data 0; bitmap-name pulse 0..3 |
| `_star_name_highlight_direction` | 0x182145 | int8 | .data 1 |
| `_multi_colored_line_start` | 0x19995e | int16 | relocation-line colour phase; `Draw_Main_Screen_` end: `= (v+1) % 8` |
| `_eta_star_id` | 0x19995c | int16 | |
| `_fleet_ship_scanned` | 0x1999ca | int16 | |
| `_main_screen_scanned_star` | 0x19c18a | int16 | -1 none |
| `_galaxy_map_scanned_star` | 0x199960 | int16 | -1 none |
| `_moveable_box` | 0x192fdc | 5 x 0x1c | box 0 `content` (0x192fde) = star of the system box |
| `_main_palette_player_colors` | 0x10010 (code) | uint8[8] | by player colour 0..7: 0x49 0x62 0x6e 0x20 0x3e 0x94 0x2d 0x55 |
| settings+0x08 | 0x199be4 | uint8 | Show Relocation Lines |
| `screen_window_x1/x2/y1/y2` | 0x1bba4c/4e/50/52 | int16 | engine window |
| `old_screen_window_*` | 0x1bc294/96/98/9a | int16 | one save slot (`Save_Window`) |
| `clipping_flag` | 0x1845d8 | int16 | |

Constant tables in the code segment (copied to the stack by `movsd`):

| Addr | Bytes | Use |
|---|---|---|
| 0x81c68 | int16 39, 33, 33, 24 | black hole draw size by zoom |
| 0x81c78 | 00 0a 0c 0e 10 12 14 16 | stardate font colours (not this area) |
| 0x81c80 | 6e 6f 70 70 6e 6f 70 70 | relocation line colours |
| 0x81c88 | 80 7b 78 2c 86 9f 83 7e | surrender underline colour by player colour |
| 0x179757 | `(%s)` | 0x17975c `%s (%s)`, 0x179764 `%s`, 0x179767 `%s (contested)` |

## Map transform and zoom

All in module 48 unless noted.

- `Get_Scaled_Value_(v)` 0x7926c: `(int)v*10 / cur_map_scale` (idiv). Map → pixels.
- `Get_Up_Scaled_Value_(v)` 0x7927f: `(int)v*cur_map_scale / 10`. Pixels → map.
- `Get_Zoom_Ratio_Value_(v)` 0x79293: `(int)v*cur_map_scale / max_map_scale`.
- `Map_Scale_To_Zoom_Level_()` 0x79917: scale 10 → 0 if `max_zoom_count >= 0`;
  15 → 1 if `>= 1`; 20 → 2 if `>= 2`; any other → 3 if `>= 3`; otherwise
  returns `max_zoom_count`. Note the zoom level is 0 when zoomed in.
- `Get_Map_Scale_(level)` 0x790a4: scale for level 0/1/2/other = 10/15/20/30,
  but `max_map_scale` if that is smaller. If the result equals
  `max_map_scale`, zero the six parallax offsets. Returns the scale in AX
  (the final store of DX to 0x1999a0 writes `max_map_scale` back unchanged).
- `Inverse_Map_Scale_Ratio_()` 0x7983d: max 10→30, 15→20, 20→15, 30 (>20)→10;
  any other value <15 → 10, 16..19 → 10.
- `Increment_Zoom_Count_` 0x796bf: `zoom_count = min(zoom_count+1,
  max_zoom_count)`. `Decrement_Zoom_Count_` 0x78fe6: `max(zoom_count-1, 0)`.
  `Increment_Zoom_Count_With_Limit_` 0x796e2: same increment, then if
  `Map_Scale_To_Zoom_Level_() <= zoom_count` set `zoom_count` to it and
  return 0, else 1.
- `Get_Star_Draw_Coords_(star, &x, &y)` 0x85b93 (module 52):
  `x = Get_Scaled_Value_((int16)(star.x - cur_map_x)) + 0x15`,
  same for y with `star.y - cur_map_y`. Results stored as int16. The
  difference is truncated to 16 bits before scaling.
- `Star_On_Screen_(star)` 0x8a188: true when `0x15 < x < 0x20f` and
  `0x15 < y < 0x1a5` (strict) for the draw coords.
- `Center_Map_(mx, my)` 0x8238b and `Scroll_Map_(mx, my)` 0x89df9 set
  `cur_map_x = clamp(mx - Up(0xfd), 0, MAP_MAX_X - Up(0x1fa))`,
  `cur_map_y = clamp(my - Up(200), 0, MAP_MAX_Y - Up(400))` (Up =
  `Get_Up_Scaled_Value_`; the upper clamp is applied after the lower, so it
  wins when the map is narrower than the window). `Center_Map_` then calls
  `Get_New_Paralax_XYs_(old_x, old_y, 1)` if the origin moved and sets
  `_draw_main_screen_to_back = 1`. `Scroll_Map_` animates when
  `Range(old, new) >= 0x33` (steps of 50 via `Interpolate_Line`, full
  redraws and `_TOGGLE_PAGES_`); recorded here only for the transform, the
  scroll loop belongs to the main-screen loop spec.

## Draw order inside Draw_Main_Screen_ (0x84555)

When `_stencil_box_id` is not 0..4: `Set_Window(0x16,0x16,0x20f,0x1a5)`,
`Clip_On`. If `_draw_main_screen_to_back == 1` or `_FULL_DRAW_SCREEN_FLAG`:
`Draw_Main_Screen_Filled_(1)` and clear the first flag; else
`Draw_Wormhole_Links_`, `Draw_Relocation_Links_`, `Print_Star_Names_`.
Window/clip again, stardate (settings+1, not this area), then:

1. `Draw_Black_Holes_()`
2. `Draw_A_Gate_Icon_(i)` for i = 0..NUM_STARS-1
3. `Do_Ship_Destination_Lines_` (not this area)
4. if `eta_star_id >= 0` and `fleet_ship_scanned == -1`:
   `Draw_ETA_Destination_Line_`; if that star's class != 6:
   `Print_Highlighted_Star_Name_(eta_star_id)` then **pulse**.
5. if `main_screen_scanned_star != -1` and its class != 6:
   `Print_Highlighted_Star_Name_(it)`, pulse.
6. `Draw_Stars_()`, then ship icons etc.
7. In the box loop (order index 4 down to 0): for an open box whose id is 0
   and whose star (`box0.content`) is not class 6:
   `Print_Highlighted_Star_Name_(box0.content)`, pulse.
8. After the boxes: if `0 <= galaxy_map_scanned_star < NUM_STARS`:
   `Print_Galaxy_Map_Scanned_Star_()`.
9. `Clip_Off`, `Reset_Window`, ..., `multi_colored_line_start = (+1) % 8`.

Pulse: `offset += mod` (int8); if `offset == 3` or `offset == -1`:
`mod = -mod`. So `offset` runs 0,1,2,3,2,1,0,-1,0,..., once per highlighted
name drawn (it can step several times per frame).

`Draw_Main_Screen_Filled_(copy)` 0x84a95 (star parts): after the
background (`BUFFER0.LBX` 0 at 0,0, full-screen window, clip on) it calls
`Draw_Wormhole_Links_`, `Draw_Relocation_Links_` (so these lines are also in
the back page), then nebulae, then `Copy_Off_To_Back` if `copy == 1`, then
`Print_Star_Names_` (off page only).

## Star pictures

`Get_Star_Picture_Seg_(class, picture, size)` 0x81cd3 (AL, DL, BL):
`i = class*6 + zoom + (class == 6 ? 0 : size)` with zoom =
`Map_Scale_To_Zoom_Level_()`; `_star_seg[i] = Buffer_Reload("BUFFER0.LBX",
0x94 + i, _global_cache_seg)`; returns it. The `picture` argument (star+0x15)
is **not used**.

`BUFFER0.LBX` entries (rendered and checked; header mask
0x04, kind 0 sprite, repeat 0):

| Entries | What | Size | Frames |
|---|---|---|---|
| 0x94+6c .. 0x94+6c+5 | class c (0 blue, 1 white, 2 yellow, 3 orange, 4 red, 5 brown), index size+zoom | 33, 29, 25, 23, 21, 17 square | 5 (twinkle) |
| 0xb8..0xbb (184..187) | black hole, by zoom 0..3 | 40, 34, 34, 25 | 16 (spinning) |
| 0x15, 0x16 (21, 22) | Dimensional Portal icon, zoom 0 / zoom ≠ 0 | 8x8, 6x6 | 1 |
| 0x21, 0x22 (33, 34) | Artemis System Net icon, zoom 0 / ≠ 0 | 11x10, 8x9 | 1 |
| 0 | main screen frame (background) | 640x480 | 1 |

Size 0 is the biggest star; `star.size + zoom` ranges 0..5.

### Draw_Stars_ (0x85550)

`_n_twinkling_stars = 0`; for each star with class != 6 and
`Star_On_Screen_`: `Draw_A_Star_(i)`.

### Draw_A_Star_ (0x83b02)

1. `dim = _star_fields_dim[size + zoom]`.
2. `pic = Get_Star_Picture_Seg_(class, picture, size)`.
3. `(x, y) = Get_Star_Draw_Coords_`; `x -= dim/2`, `y -= dim/2`.
4. If `star.twinkle_frame (+0x64) < 0` (int8): `Reset_Animation_Frame(pic)`
   (current_frame = 0).
   Else: `tf = ++star.twinkle_frame` (int8 wrap); `f = (tf/2) % pic.frames`
   (signed); if `tf >= star.twinkle_end (+0x65)` (signed int8):
   `_n_twinkling_stars = max(_n_twinkling_stars - 1, 0)` and both bytes = -1.
   `Set_Animation_Frame(pic, f)` (clamps `f >= frames` to `frames-1`); the
   frame chosen before the reset is still drawn.
5. `Draw(x, y, pic)` (clipped; window and clip come from the caller).

### Twinkle

`Clear_Twinkling_Star_Data_` 0x82d9b: for all 72 records `+0x64 = +0x65 =
-1`; `_n_twinkling_stars = 0`. Only caller: `Harold_Defaults_` (0x79618).

`Get_Twinkling_Star_` 0x85f2f has **no callers** (no xrefs, no pointer).
It collects stars with `twinkle_frame < 0` that are either any star when
`cur_map_scale == max_map_scale`, or on screen and not class 6; shuffles the
list (`Randomize_List_(list, n, 1)`), then for k = 1..n-1 picks `list[k]`
when `clock() % (k+1) == 1`; returns the pick (0 if none). Nothing else in
the binary writes star+0x64/+0x65, so in this build twinkling never starts:
every star always shows frame 0. `_twinkle_star_ctr` (0x199978) has no
access.

### Draw_Black_Holes_ (0x83bf9)

`dims = {39,33,33,24}`, `z = zoom`, `k = 0`. `Save_Window_Status_`,
`Set_Window(0x16,0x16,0x20f,0x1a5)`, `Clip_On`. For each star with
class 6 (on-screen test not applied; clipping handles it):

1. coords; `x -= dims[z]/2`, `y -= dims[z]/2`.
2. `pic = Get_Star_Picture_Seg_(6, …)` (entry 0xb8 + z).
3. `c = (_black_hole_anim_count[k] + 1) % (2*pic.frames)` (unsigned byte
   source, int divide), store `c` as byte; `Set_Animation_Frame(pic, c/2)`.
4. `k++`; `Draw(x, y, pic)`; `if k > 10: k = 10`.

So the 11th and later black holes all use slot 10, i.e. the low byte of
`_h_string_ix`, which each of them increments. `Restore_Window_Status_`.

## Gate icons: Draw_A_Gate_Icon_(star) (0x83741)

Returns at once unless `Player_Has_Visited_(PLAYER_NUM, star)` (star+0x33
bit). Then:

- `dim = _star_fields_dim[size + zoom]`, `e = (zoom != 0) + 2` (2 or 3).
- `Player_Has_Stargate_` and `Player_Has_Jumpgate_` are called and the
  results discarded.
- `art = star.artemis_net (+0x3f) bit PLAYER_NUM`,
  `por = star.dimensional_portal (+0x40) bit PLAYER_NUM`
  (`Player_Has_Artemis_Net_` 0x79cf9, `Player_Has_Dimensional_Portal_`
  0x79d50; player index = (ptr - _player)/0xea9).
- `Push_Block(_screen_seg)`; four 18x18 blank bitmaps b[0..3].
- `n = 0`, `maxh = 0`. If `por`: `_gate_icon_seg[e]` (0x190498) =
  `Buffer_Reload(BUFFER0, 0x13 + e)` (0x15/0x16);
  `Draw_Picture_To_Bitmap(b[n], it)`; `maxh = max(maxh, it.height)`; n++.
  If `art`: `[0x1904c8 + 4e]` = entry `0x1f + e` (0x21/0x22), same.
- If `n >= 1`: `(cx, cy) = Get_Star_Draw_Coords_`; `base = cy + dim/2 - maxh`.
  - n == 1: `w = Get_Bitmap_Actual_Size(b[0]).width`;
    `Draw(cx - w - 4, base + (por ? 2 : 0), b[0])`.
  - n == 2: `Draw(cx - w0 - 4, base + (por ? 2 : 0), b[0])`;
    `Draw(cx + 4, base, b[1])`.
  - 3..4: dead code (75x20 bitmap, `Add_Picture_To_Bitmap` side by side,
    centred); unreachable with two icon types.
- `Pop_Block`.

Only Dimensional Portal and Artemis Net are drawn here; the shipped saves
have both bitmasks 0 (`save-stars.md`), so nothing is drawn for them.
Window and clip come from `Draw_Main_Screen_`.

## Wormhole links: Draw_Wormhole_Links_ (0x85593)

Save window status; window 0x16,0x16,0x20f,0x1a5; clip on. For i in
0..NUM_STARS-1 with `star.wormhole_to (+0x29) != -1` and
(`Player_Has_Visited_(PLAYER_NUM, i)` or `Player_Is_Omniscient_(PLAYER_NUM)`):
`Line(coords(i), coords(wormhole_to), 4)`. Each link is drawn from both
ends (links are mutual), the second time only if that end is visited too.
Restore.

## Relocation links: Draw_Relocation_Links_ (0x85320)

Save; window; clip on. If settings+0x08 (Show Relocation Lines): for each
star i with `star.relocation[PLAYER_NUM] (+0x54+2p) != -1`
(`Star_Has_Relocation_` 0x78c94, `Relocation_` 0x784f0):
`Draw_Directional_Multi_Colored_Line_(coords(i), coords(dest),
{6e 6f 70 70 6e 6f 70 70}, multi_colored_line_start)`. Restore.

`Draw_Directional_Multi_Colored_Line_(x1,y1,x2,y2,colors,start)` 0xa11c0:
`r = 7 - start; if r <= 0: r = 0`. If (`x2 > x1` and `y2 >= y1`) or
(`x2 <= x1` and `y2 > y1`): `Multi_Colored_Line(x1,y1,x2,y2, colors, r)`.
Else `Multi_Colored_Line(x1,y1,x2,y2, reversed colors (rev[7-i] =
colors[i]), start)`. (Multi_Colored_Line always draws top to bottom, this
keeps the crawl moving source → destination.)

## Star names

### Font helpers (MOX)

- `Zoom_Level_Font_Style_()` 0x7a440: scale 10 → 3, 15/20 → 2, 30 → 1
  (other scales return EAX unchanged).
- `Font_Colors2_(mode, a, b)` 0x8e5c5 → `_mox_font_colors` (0x19c31c, 8
  bytes; zeroed first). Mode 0..1: c[0] = a, c[1..7] = b. Mode 2..3: same,
  then `c[2] = offset != 0 ? offset : min(b, a+1)` (signed). Mode 4: same
  fill, `c[2] = offset != 0 ? offset : b-1`. Mode 5: c = {0, a, b-1, b-1,
  b-1, b, b, b}. Mode > 5: all 0. Always clears `_mox_font_colors_offset`
  (0x199f34). The existing `fontColors2` in `internal/game` covers modes 0–1.
- `Get_Mox_Font_Colors_(mode, a, b, out)` 0x8f6f8: copies `Font_Colors2_`.
- `Get_Player_Font_Colors_(p, style, out, hl)` 0x79138:
  `off = hl ? (int8)selected_star_color_offset : 0`; defaults a = 1,
  b = 0x0e, kind 0. If 8 <= p < 15: a = 4, b = 0x0b, kind 1. Else (p < 8,
  also negative p, or p >= 15): if `player[p].colour (+0x26) == 3`: a = 0x0c,
  b = 0x16, kind 2; else if p < 8: `m = main_palette_player_colors[colour]`,
  a = m+1, b = m+6 (m+5 for colour 4). `out = Font_Colors2_(style, a, b)`.
  For i = 1..7: kind 1: `out[i] += off`; kind 2: `out[i] += (off*7)/4`;
  kind 0: `out[i] += (off*3)/2` (truncating, byte add); then
  `out[i] = min(out[i] (unsigned), b + 8)`.
- `Set_Player_Font_Style_(p, hl)` 0x7a00a: style = `Zoom_Level_Font_Style_`
  logic; `Get_Player_Font_Colors_(p, style, buf, hl)`; `Set_Font_Style(style,
  buf)`.
- `Decrease_Font_Color_Range_(n)` 0x8ebfe: c = current colours; c[1..7] =
  `c[i]-n`, 0 if that is < 1; normal and current colours = c.
- `Get_Font_Colors_(out)` 0x8f625: out = current colours.

### Get_Star_Name_(star, hl, &bmflag, name, &bitmap) (0x85c8a)

`omni = Player_Is_Omniscient_(PLAYER_NUM)` (leader with Galactic Lore or
player+0x8ba). `name[0] = 0`, `bmflag = 0`. `off = hl ? (int16)(int8)
selected_star_color_offset : 0`. `outpost = Star_Is_Outpost_Star_(star)`
(0x7a133: at least one colony and all of them have colony+6 set).
`o = star.owner (+0x14)`.

A. If `!omni` and (`o < 0`, `o >= 8`, or not
`Contact_With_One_Colony_(star)` (0x78f4b: some colony's owner has contact
with PLAYER_NUM via `Contact_With_Player_`)):
- not `Player_Has_Visited_` → return (no name, nothing printed).
- `s = Zoom_Level_Font_Style_()`, `c = Font_Colors2_(s, 1, 0x0c)`; **brighten**;
  `Set_Font_Style(s, c)`; `name = star.name`; if outpost:
  `Decrease_Font_Color_Range_(2)`, `String_To_Lower(name)` (A–Z only). Return.

B. Else:
- `name = (omni && o != PLAYER_NUM && !visited) ? "(%s)" : star.name`.
- if 0 <= o < 8: `Set_Player_Font_Style_(o, hl)`; else font as in A
  (style, `Font_Colors2_(s,1,0x0c)`, brighten, `Set_Font_Style`).
- if outpost: `Decrease_Font_Color_Range_(1)`, lower-case.
- if `N_Colony_Owners_(star) > 1` (0x79979: distinct colony owners over the
  five orbits, all colonies): `bmflag = Print_Star_Name_To_Bitmap_(star,
  name, &bitmap)`.

Brighten (only if `off > 0`): for i = 1..7: `c[i] += off*9/5` (byte);
if `c[i] < c[0]` (unsigned) `c[i] = c[0]`; if `c[i] > 0x10` `c[i] = 0x10`.

### Print_Star_Name_To_Bitmap_(star, name, &bitmap) (0x88b5b)

`Colony_Owners_(star, counts[8], &n, onlyKnown = !omni)` (port:
`State.colonyOwners`). If `n <= 1` return 0. Else: `total = Σ counts`;
`cur = Get_Font_Colors_()`; `Set_Font_Spacing(1)`; `Push_Block`; 200x20
blank bitmap → `*bitmap`; `Print_To_Bitmap(0, 0, name, bitmap)` with the
current style (shadow 0, as `Set_Font_Style` cleared it, so no outline);
`x = 0`; for p = 0..7 with `counts[p] > 0`:
`W = Get_String_Width(name)`, `per = ((W*10)/total + 9)/10`,
`w = min(counts[p]*per, W)`, `h = Get_Font_Height()`;
`Get_Player_Font_Colors_(p, current_font_style, new, 0)`;
`Swap_Bitmap_Colors(x, 0, x+w, h, cur, new, 8, bitmap)`; `x += w`.
`Pop_Block` (the bitmap stays in the popped block and is used afterwards);
return 1.

`Get_Bitmap_String_Range_` 0x8f4ca returns `w`, 0, `min(w+per, W)` (unused)
and `h`. `Swap_Bitmap_Colors_(x1,y1,x2,y2,from,to,n,bm)` 0x8fe88: for
`y1 <= y < y2`, `x1 <= x < x2` (bitmap width stride, data at +0x10): the
first k < n with `pix == from[k]` sets `pix = to[k]`.

### Print_Star_Names_ (0x88cb7)

Save window status; window 0x16,0x16,0x20f,0x1a5; clip on. For each star
with class != 6: `Get_Star_Name_(i, 0, &bm, name, &bitmap)`; if `name[0]`:
`Print_A_Star_Name_(i, bm, name, bitmap)`. Restore. No on-screen test; the
clipped printer drops what falls outside.

### Print_Highlighted_Star_Name_(star) (0x87af9)

Same window setup; `Get_Star_Name_(star, 1, …)`; if a name: if `bm`:
`Tint_Bitmap_Colors_(0,0,0,0, whole=1, highlight_value, bitmap)` (0x8ff08:
every nonzero pixel += value), then `highlight_value += direction`; if it
becomes 0 or 3, `direction = -direction`. `Print_A_Star_Name_(star, bm,
name, bitmap)`. Restore. (`selected_star_color_offset` is stepped by the
caller, see Draw order.)

### Print_A_Star_Name_(star, bm, name, bitmap) (0x87768)

1. `dim = _star_fields_dim[size + zoom]`; `(cx, cy)` = coords.
   `y = cy + dim/2 - star.size`; `x = cx + 1`.
2. If `bm`: `Draw(x - Get_String_Width(name)/2, y, bitmap)`; return.
3. `s = Zoom_Level_Font_Style_()`; `c = Get_Font_Colors_()` (as left by
   Get_Star_Name_); `Set_Outline_Color(2, 1)`;
   `Set_Font_Style_Outline(s, c)` (shadow 4); `Set_Font_Spacing(1)`;
   `W = Get_String_Width(name)`; `left = x - W/2`.
   - if `cur_map_scale == max_map_scale` and `left < 0x16`: `Print(0x16, y)`;
   - else if `cur_map_scale == max_map_scale` and `left + W > 0x20f`:
     `Print_Right(0x20f, y)`;
   - else `Print_Centered(x, y)`.
4. Surrender mark: `q = star.surrender_to[PLAYER_NUM] (+0x67+p)`; if
   `(unsigned)q < 8`: `Set_Player_Font_Style_(0, 0)` (player 0 — the font is
   left in that state), `Set_Font_Spacing(1)`, `W2 = Get_String_Width(name)`,
   `pc = player[q].colour`, `col = {80 7b 78 2c 86 9f 83 7e}[pc]`,
   `lx = cx - W2/2` (no +1), `ly = Get_Font_Height() + cy + dim/2 -
   star.size`; `Line(lx, ly, lx + W2, ly, col)`; if `cur_map_scale < 30`:
   `Line(lx, ly+1, lx+W2, ly+1, (pc == 3 || pc == 5) ? col-3 : col+1)`.

### Print_Galaxy_Map_Scanned_Star_ (0x879bc)

`s = _galaxy_map_scanned_star`, `o = star[s].owner`. Box 1 fields
(0x192ff8): `px = box1.x + box1[+0x18]`, `py = box1.y + box1[+0x1a]`.
- 0 <= o < 8: `Set_Player_Font_Style_(o, 0)`; `buf = "%s (%s)" (name,
  Race_Name_String_(o))`.
- o == -1: colours c[i] = (i/2 == 0) ? 8 : 0x0c (c[0..1] = 8, rest 0x0c);
  `buf = name`.
- o == -2: c = 0x14,0x14,0x1f,…; `buf = "%s (contested)"`.
- other o: neither buf nor c is set.
Then always `Set_Font_Style(2, c)` and `Print(px + 0xf, box1.height + py -
0x19, buf)`. In the 0..7 case `c` is the uninitialised stack array (see
open questions). Window/clip: `Draw_Main_Screen_`'s (0x16..0x20f, 0x16..0x1a5,
clip on).

## Engine primitives

Status against `internal/engine` / `internal/font` / `internal/gfx`.

| Primitive | Addr | Behaviour | Port |
|---|---|---|---|
| `Set_Window(x1,y1,x2,y2)` | 0x128ab6 | clamp x1,y1 >= 0, x2 <= 639, y2 <= 479 (`>= width` → width-1); store | fields `WinX1..` only; missing |
| `Reset_Window` | 0x128be7 | 0,0,639,479 | missing |
| `Save_Window` / `Restore_Window` | 0x128b49 / 0x128b98 | copy window to/from the single `old_screen_window_*` slot (not a stack) | missing |
| `Clip_On` / `Clip_Off` | 0x12b634 / 0x12b65c | `clipping_flag` = 1/0 | field `Clipping`; missing |
| `Save_Window_Status_(&f)` / `Restore_Window_Status_(f)` (MOX) | 0x8fd71 / 0x8fd56 | Save_Window + return clip flag / Restore_Window + Clip_On if f else Clip_Off | missing |
| `Draw` clipped path | 0x12a478 | with clip on: skip drawing (frame still advances) if `x > wx2`, `y > wy2`, `x+w-1 < wx1` or `y+h-1 < wy1`; if not fully inside: mark `Add_Square(x, y, x+w-1, y+h-1)` (unclipped, clamped to screen) when refresh >= 2, then kind 0 → `Clipped_Draw_Animated_Sprite_`, kind 1 → `Clipped_Draw_Bitmap_Sprite_`, kind 2 → fatal; glassed variants for mask 0x08; `dot_stencil_flag` (0x1845da) → stenciled variant. Fully inside → normal path | returns an error; missing |
| `Clipped_Draw_Animated_Sprite_` | 0x1489f4 | see below | missing |
| `Clipped_Draw_Bitmap_Sprite_` | 0x14861d | left = max(x, wx1), right = min(x+w-1, wx2), top = max(y, wy1), bottom = min(y+h-1, wy2); skip `wx1-x` columns and `wy1-y` rows of source; per row `Color_Stream_Copy` (0 transparent) | missing |
| `Color_Stream_Copy_` | 0x148605 | copy n bytes, skipping zeros | — |
| `Reset_Animation_Frame` / `Set_Animation_Frame` | 0x12b726 / 0x12b753 | frame = 0 / frame = min(f, frames-1) (no lower clamp) | missing (trivial: `CurrentFrame`) |
| `Line(x1,y1,x2,y2,c)` | 0x134d2d | see below | missing |
| `Clip_Line` | 0x135357 | see below | missing |
| `Multi_Colored_Line(x1,y1,x2,y2,colors,rot)` | 0x135870 | `Line` with pixel colour `colors[rot & 7]`, `rot++` per pixel in drawing order (top to bottom after the swap) | missing |
| `Add_Dot_To_Scan_List(x,y)` | 0x125e13 | merge column x>>2 into scan list row y (and stencil unless refresh 2); no clamping | missing |
| `Get_Bitmap_Size(w,h)` | 0x12daa4 | `((w*h + 0x13) / 4) * 4` | n/a (Go alloc) |
| `Create_Blank_Bitmap(w,h,bm)` | 0x12d8f5 | header (`Create_Bitmap_Header` 0x12d943: kind 1, one frame at +0x10) + zeroed pixels | missing |
| `Draw_Picture_To_Bitmap(dst, src)` | 0x12f578 | re-creates `dst` blank at src's size; kind 1: copy the current frame; else render frames from `(cur / mfs)*mfs` (0 if mfs = 0) up to `current_frame` inclusive, sprite runs, no transparency test (zero bytes are copied) | missing |
| `Get_Bitmap_Actual_Size(bm, &x1,&y1,&w,&h)` | 0x131528 | bounding box of nonzero pixels; x1,y1 start at 1000; `w = maxx - x1 + 1` (maxx starts 0) | missing |
| `Print_To_Bitmap(x,y,s,bm)` | 0x122a6e | `Print_Display_To_Bitmap(x,y,s,0,bm)` (0x122aab): `Print_Display` passes into a kind-1 bitmap via `Print_String_To_Bitmap_` 0x122d8a / `Print_Character_To_Bitmap_` 0x123070 | missing |
| `Swap_Bitmap_Colors_`, `Tint_Bitmap_Colors_` (MOX) | 0x8fe88, 0x8ff08 | above | missing |
| `Push_Block`/`Allocate_Next_Block`/`Pop_Block` | 0x1112ec/0x110d3c/0x1113cc | scratch arena on `_screen_seg` | n/a |
| `Print_Display` clipped branch | 0x1212eb | when `clipping_flag == 1` every pass uses `Clipped_Print_String`; marks `Add_Square(x-d, y-d, x+d+width, y+d+height)` clamped to the screen (d = 1, 2 for heavy) | `Printer.Display` has the passes, not the clipped branch |
| `Clipped_Print_String` | 0x122309 | returns 1 if `y > wy2` or `y + height < wy1`; else `Print_String` logic with `Print_Clipped_Character` | missing |
| `Print_Clipped_Character(x,y,c)` | 0x12260f | c >= 0x81: return x unchanged. Inside (`x >= wx1`, `x+w <= wx2`, `y >= wy1`, `y+height <= wy2`): `Print_Character`. Else: `next = x + w + spacing`; left clip `sx = wx1-x` (return next if `w <= sx`), `w -= sx`, `x = wx1`; right: if `x+w > wx2`: `w = wx2+1-x`, return next if `< 1`; top `sy = wy1-y`, `y = wy1`; `maxy = height` or `wy2+1-y`; `Print_Clipped_Letter`; return `x + spacing + w` (clipped values) | missing |
| `Print_Clipped_Letter` | 0x1227ec | glyph rows, skip `sy` rows, draw `min(maxy, base_height) - sy` rows, only pixels in `[sx, sx+w)`; byte < 0x81 writes the colour table entry (no zero test), > 0x80 skips `b & 0x7f` | missing |
| `Set_Font_Spacing(n)` | 0x120e8c | `current_horizontal_spacing = n` (reset by `Set_Font_Style`); used by printing and `Get_String_Width` | missing (port uses the style's spacing) |
| `Set_Font_Style_Outline(s,c)` | 0x120d79 | `Set_Font_Style` + shadow 4 | `SetStyleShadow(s, c, font.Outline)` |
| `Set_Outline_Color(c, alias)` | 0x120ded | | `SetOutlineColor` |
| `Get_Font_Height` | 0x122259 | current style height | via `Font.Styles[Style].Height` |
| `Print`, `Print_Centered`, `Print_Right`, `Get_String_Width` | 0x1212b3, 0x1210fd, 0x1210b7, 0x12066f | | ported (`internal/font`) |
| `Buffer_Reload(file, entry, cache)` | 0x127c27 | picture from LBX via the cache | `Game.picture` parses fresh each call (no shared object) |
| `String_To_Lower` | 0x132fcd | 'A'..'Z' += 0x20 | trivial |

### Clipped_Draw_Animated_Sprite_ (0x1489f4)

`cx1 = max(wx1, x)`, `cy1 = max(wy1, y)`; `cx2 = (wx2 < x+w) ? wx2 : 639`;
`cy2 = (wy2 < y+h) ? wy2 : 479`. Data from frame+4: `row = y + top`;
`last = min(y+h, cy2)`; while `row <= last`: read `(count, skip)`;
count 0 → `row += skip`, column back to x. Else if `row < cy1` or the
current column `>= cx2` (tested before adding skip): source += count only
(column not advanced). Else column += skip; left-clip to `cx1`; if
`col + count > cx2`: copy `cx2 - col + 1` bytes if `cx2 - col > 0`; else copy
`count` if > 0 (zeros transparent); column, source += count. Realign source
to even after every run. Note the right edge is inclusive here and the loop
may read one `(count, skip)` pair past the last row when not clipped at the
bottom.

### Line (0x134d2d) and Clip_Line (0x135357)

`Line`: if clip on and `Clip_Line` returns 1, nothing. Swap ends so
`y1 <= y2`. `dx = |x2-x1|` (advance ±1), `dy = y2-y1`. Vertical, horizontal
and 45° lines are straight loops (inclusive ends). Otherwise Abrash run
slicing: major axis `M`, minor `m`; `whole = M/m`, `adj_up = (M%m)*2`,
`adj_down = 2m`, `err = M%m - 2m`; `initial = whole/2`, `final = initial+1`;
if `adj_up == 0` and `whole` even, `initial` stays, else `initial = final`
(for x-major the same: `initial = final` unless `adj_up == 0` and whole
even); if whole odd `err += m`. Draw `initial`, then `m-1` runs of `whole`
(+1 when `err += adj_up` > 0, then `err -= adj_down`), then `final`. Each
vertical run steps x by the advance after it; each horizontal run steps y
by one. Dirty marking (refresh >= 2): dots per pixel row for vertical runs,
`Add_Square` per row for horizontal runs and straight lines.

`Clip_Line` (window inclusive): if `x1 == x2`: out (1) if outside x;
else order y, out if outside y, clamp y1/y2. Else order by x (swap both
ends), `ddy = y2-y1`, `ddx = x2-x1` (after the swap, before clipping); if
`x1 < wx1`: `y1 += (wx1-x1)*ddy/ddx`, `x1 = wx1`; out if `x2 < x1`; if
`x2 > wx2`: `y2 = (wx2-x1)*ddy/ddx + y1` (with the new x1,y1), `x2 = wx2`;
out if `x2 < x1`. Then the same in y (always run, even after an x reject):
if `y1 == y2`: out if outside, order x, out if outside, clamp; else order by
y, deltas, clip top/bottom the same way. Returns 1 when rejected.
32-bit products, `idiv` truncation.

## Open questions

1. `Print_Galaxy_Map_Scanned_Star_` with owner 0..7 calls
   `Set_Font_Style(2, c)` on an uninitialised local after
   `Set_Player_Font_Style_`, so the colours are stack garbage (leftover of
   earlier calls in `Draw_Main_Screen_`). Compare with the original before
   choosing what the port prints; owners outside -2..7 also print an
   uninitialised buffer.
2. It positions with box 1 `+0x18/+0x1a` (`drag_dx/dy` in `save-ships.md`);
   those names may be wrong for box 1.
3. Twinkling is dead in this build (`Get_Twinkling_Star_` uncalled). Worth a
   check against `ORION95.EXE`.
4. `_black_hole_anim_count` has 10 slots; the 11th+ black hole uses
   `_h_string_ix`'s low byte. Need a save with > 10 black holes to see
   whether that matters.
5. The star picture header mask bit 0x04 is not documented in
   `pictures.md`.
6. `Clipped_Draw_Animated_Sprite_` reading one pair past the last row: check
   what the sprite data ends with.
7. `Print_Clipped_Letter` reads colours via `palette_block + b + 3`;
   assumed equal to the current colour table entry `b-1` as in
   `Print_Character_ASM`; verify.
8. `Buffer_Reload` caching: whether repeated reloads return the same
   object (shared `current_frame`). It doesn't affect these routines, which
   set the frame before each Draw.
9. `Print_Display_To_Bitmap` assumed to mirror `Print_Display`'s passes; for
   star names it runs with shadow 0.
