# Main screen: clicking a star, the system display box

What happens when the player clicks a star on the galaxy map: the hit test,
the dispatch in `Main_Screen_` (0x86188), `Do_System_Popup_`, and the system
display moveable box (box id 0) that shows the star's planets on their orbits.
Addresses are ELF (Watcom + 0x10000); data is ELF (0x178000 + DWARF offset).
Box records, `Moveable_Box_Selected_`, `Draw_Moveable_Box_` dispatch and
`Add_Moveable_Box_Fields_` are in `main-screen-panel.md`; the box record
layout is in `docs/formats/save-ships.md`; planets and colonies in
`docs/formats/save-planets.md`; stars in `docs/formats/save-stars.md`.

Most of the box code is module 63 (`sysdisp`, 0xa1c4e..0xa59a1).

## Globals

| Name | Addr | Type | Notes |
|---|---|---|---|
| `_moveable_box[0]` | 0x192fdc | 0x1c | fields used here: `content` 0x192fde (star id), `height` 0x192fe0, `width` 0x192fe2, `x` 0x192fe4, `y` 0x192fe6, `dragging` 0x192fe8, `drag_field` 0x192fea, `field2` (close) 0x192fec, `field3` (whole box) 0x192fee, `closed_w/h` 0x192ff0/2, `drag_dx` 0x192ff4, `drag_dy` 0x192ff6 |
| `_moveable_box_pict` | 0x193070 | ptr | last `Buffer_Reload(BUFFER0, 0x49)` |
| 0x193080 | | ptr | close button picture, BUFFER0 entry 0x52 (`main-screen-background.md`) |
| `_sys_data_buffer` | 0x19d214 | ptr | 0xdc-byte arena (`main-screen-background.md`) |
| `_star_name_loaded_string` | 0x193274 | ptr | 20-byte "loaded" tag, first block of `_sys_data_buffer` |
| `_sys_disp` | 0x193184 | ptr | 5 × 14-byte records, next block (0x46 bytes) |
| `_system_display_sun_seg` | 0x19d218 | ptr | sun picture |
| `_sun_seg` | 0x190470 | ptr[7] | per spectral class |
| `_o_ring_seg` | 0x193194 | ptr[5] | orbit ring pictures |
| `_planet_mask_seg` | 0x1933f8 | ptr[5] | light masks by size |
| `_sys_disp_bitmap` | 0x19d21c | ptr | 288×154 ring bitmap (scratch) |
| `_system_display_ships` | 0x1931bc | 15 × 12 | per owner: +0 x, +2 y (box-relative), +4 field, +6 ship node, +8 picture ptr |
| `_planet_buttons` | 0x1992d4 | s16[5] | field per `_sys_disp` slot |
| `_gate_icon_field` | 0x190490 | s16[4] | [0] dimensional portal, [3] Artemis net |
| `_system_display_grid_field` | 0x1999b2 | s16 | grid field over the orbit area |
| `_system_display_grid_x/y` | 0x1999ae / 0x1999b0 | s16 | written by the grid field |
| `_system_display_base_x/y` | 0x1999e6 / 0x1999e8 | s16 | box x, y of the last draw (without drag offset) |
| `_system_star_id` | 0x1999c6 | s16 | star of the loaded display |
| `_system_star_id_for_auto_select` | 0x1999d0 | s16 | set by every draw (also used by F5/F6) |
| `_system_icon_scanned` | 0x1999c0 | s16 | hover: orbit slot 0..4, 1000+ship node, 2000+gate; -1 none |
| `_system_ship_scanned` | 0x1999dc | s16 | hovered ship node |
| `_ship_frame` | 0x19987a | s16 | +1 (mod 32000) per box draw; planet rotation frame |
| `_skip_draw_system_planets` | 0x19d32c | u8 | skip the planets once |
| `_draw_sun_before_planet` | 0x19d32d | u8 | |
| `_skip_draw_system_display` | 0x19d32f | u8 | skip one box draw / field build |
| `_player_has_ship_in_system` | 0x199f26 | u8 | |
| `_look_at_colonies` | 0x199f18 | u8 | skips the "may view" test |
| `_star_description_string` | 0x19d228 | char[] | unexplored-star text |
| `_rot_plan_dim` | 0x199ec2 | u8[5] | planet hit size by size class, 4 = gas giant: {19, 20, 22, 22, 24} (`Harold_Defaults_` 0x79587) |
| `_system_display_ring_radius` | 0x1992b6 | s16[5] | {21, 33, 45, 57, 69} (`Harold_Defaults_` 0x7955a). **Write-only**: no reader in the binary |
| `_orbit_consts` | 0xeb4c1 | s16[7][2] | orbit ellipse radii in tenths of a pixel (in the code segment): {225,118} {464,242} {704,372} {949,502} {1199,632} {1423,751} {1650,873}; row 0 and 6 are guard rings, orbit o uses row o+1 |
| `_mouse_xy` .. 0x199ecf | 0x199ec7.. | | map hit test results (below) |
| `_stencil_box_id` | 0x199964 | s16 | box being dragged; -1 none, -2 drag ended |
| `_draw_main_screen_stencilled` | 0x199956 | s16 | |
| `_mouse_button_pressed` | 0x199f32 | u8 | button of the current drag |
| settings+4 "Expanding Help" | 0x199be0 | u8 | |
| settings+7 "Auto Select Colony" | 0x199be3 | u8 | |

`_star_fields_dim` is not used by the box; star sizes on the map use it
(`main-screen-stars.md`).

### `_sys_disp` record (14 bytes, one per orbit slot, sorted)

| Off | Type | Meaning |
|---|---|---|
| 0 | s16 | planet index, -1 empty slot |
| 2 | s16 | x of the planet picture's top-left, relative to the orbit centre (empty: 999) |
| 4 | s16 | y, likewise (empty: 999); sort key |
| 6 | u8 | angular position 0..31 (light mask frame) |
| 7 | u8 | slot index in `star.planets[0..4]` (= orbit) |
| 8 | s16 | `(slot*5) % frames`; written, never read here |
| 10 | ptr | rotating planet picture |

## Hit tests on the map (ported in `internal/game/hittest.go`)

Results live in five words: `_mouse_xy` 0x199ec7 (clicked ship icon),
0x199ec9 (hovered ship icon), 0x199ecb (clicked star), 0x199ecd (hovered
star), byte 0x199ecf (button: 1 left, 2 right).

- `Check_Mouse_XY_Fields_` (0x82ac1), once per `Draw_Main_Screen_`: clear
  0x199ecd, 0x199ec9 = -1, 0x199ecf = 0. Box rect = box 0 if open, else box 2
  if open; inside = `x < px < x+w && y < py < y+h` (strict). If
  `0x15 < px < 0x20f && 0x15 < py < 0x1a5` and not inside a box:
  `Scan_Stars_XY_` (0x897cc), `Scan_Ships_XY_` (0x896a7) (hover).
- `Check_Stars_XY_(px, py, button)` (0x82cb7): 0x199ecb = `_mouse_xy` = -1;
  radius table at 0x81c04 = {12, 10, 8} by `star.size` (not by zoom; the two
  `Map_Scale_To_Zoom_Level_` calls discard their result). First star `i` with
  `(sy-py)² + (sx-px)² <= r²`, `(sx, sy) = Get_Star_Draw_Coords_(i)`:
  0x199ecb = i, 0x199ecf = button, `Mox_Play_Sound_(0x29)`.
- `Check_Ships_XY_` (0x82be8): same for ship icons (rect inclusive of
  `x+w`, `y+h`), sets `_mouse_xy`.
- `Get_Star_Draw_Coords_(s)` (0x85b93): `x = (s.x - cur_map_x)*10/scale + 0x15`,
  same for y (`Get_Scaled_Value_` 0x7926c, C division).
- `Get_Mouse_Field_(kind, &hover, &click)` (0x857f8), kind 0 stars, 1 ships:
  both outputs -1. Box 2 closed: ship click (kind 1) or star click (kind 0)
  first; box 2 open: star click first, then ship click. A click returns
  `0x199ecf != 1` (0 left, 1 right) with `*click` = index. Otherwise the
  hover of the asked kind (box 2 closed tests ships then stars, open stars
  then ships, each only for its kind) returns 4 with `*hover`; else -1.

## Main_Screen_ star click (0x86600..0x867bf)

Per pass, after `Scan_Input` (`h`, hovered field) and `Get_Input` (`r`):

1. If `r == _main_screen_grid_field` (0x1999c8) or `r == -grid`: `button =
   (r <= 0) + 1`; box 2 open → `Check_Stars_XY_`, then `Check_Ships_XY_`
   only if no star was hit; box 2 closed → ships first, stars only if no ship.
   Else `_mouse_xy = 0x199ecb = -1`.
2. If `!_skip_fields`: `k = Get_Mouse_Field_(0, &hov, &star)`; 0x19c18a =
   hov. `cl = (k == 0 || k == 4) && box 2 open ? Some_Icon_Selected_() : 0`.
3. Relocation pick (0x19c19c set by F4 in `Check_Function_Keys_` when the
   player has more than one relocation; k == 0, star not a black hole): the
   arrow pointer back, 0x19c19c = 0, `Set_All_Star_Relocations_(star,
   player)` (every relocation of the player now leads there),
   `User_Box_(H 0xdd with the star's name)`. Oracle reloc1
   (`build/oracle/reloc1.sh`, s2 with two relocations made by saveedit):
   `TestMergeRelocations` matches captures, draws and the save.
4. **k == 0, star > -1, !cl**: `c = -1`; if settings+7 and `star.class != 6`:
   `c = Last_Colony_Selected_(star, _PLAYER_NUM)` (0x7819e). `c == -1` →
   `Do_System_Popup_(star)`; else `Do_Colony_Screen_(c, &leave)` (0x8310e,
   colony screen, not covered).
5. k == 4 with cl and box 2 open: fleet ETA target (fleet box spec).

Other entry points of `Do_System_Popup_`: F5/F6 (`Check_Function_Keys_`,
`main-screen-panel.md`), `Next_Star_` 0x876db / `Previous_Star_` 0x87720,
`Fade_Into_Main_Screen_` (with settings+4 forced 0).

## Do_System_Popup_(star) (0x83669)

1. `reopened = 0`; `Set_Fleet_Movement_Icon_Selections_(0, 0)`.
2. `box0.content = star`.
3. If box 2 open: `Vanish_Fleet_Movement_Box_()`,
   `Clear_System_Display_Loaded_Flag_()`, `reopened = 1`.
4. `Init_System_Display_(star)`.
5. `(sx, sy) = Get_Star_Draw_Coords_(star)`.
6. `Set_Popup_Draw_XY_(0, star, &box0.x, &box0.y)` (0x856f7): for box 0 it
   gets the star's draw coords again and calls `Popup_XY_(0, sx, sy, &x, &y)`
   (0x8574e), which puts the box in the quadrant opposite the star
   (W, H = box0 width, height):
   - `sx < 0xfd && sy < 200`: x = 0x20f − W, y = 0x1a5 − H
   - `sx < 0xfd && sy > 200`: x = 0x20f − W, y = 0x16
   - `sx > 0xfd && sy < 200`: x = 0x16, y = 0x1a5 − H
   - otherwise (incl. sx == 0xfd or sy == 200): x = 0x16, y = 0x16

   (For other box ids the "0x1a5 − H" cases store 0x1a5.)
7. If settings+4: `Draw_Exploding_Plasma_Box_(sx, sy, x, y, W, H, 0)`.
   Else if `reopened`: `Clear_Fields`, `Clear_Main_Screen_Fields_`,
   `Disable_Draw_All_Fields`, `Set_Page_Off`, `Draw_Mini_Main_Screen_`, flip,
   `Enable_Draw_All_Fields`.
8. `Move_Box_To_Top_(0)`.

Nothing is drawn here otherwise: the box appears on the next
`Draw_Main_Screen_` (boxes, `main-screen-panel.md`), and its fields on the
next `Add_Map_Fields_`.

### Draw_Exploding_Plasma_Box_(sx, sy, x, y, w, h, box) (0x83efd) — "expanding help"

Off by default (settings+4 = 0); skipped in `Fade_Into_Main_Screen_`.

1. `seed = Get_Random_Seed()`; `d = 3`; `_animated_star` (0x199988) = -1.
   Box 2: w, h from the fleet box (0x199bb2, 0x199bb0) after
   `Fleet_Box_Data_`.
2. `Save_Window_Status_`; window (0x16,0x16,0x20f,0x1a5), clip on. If
   `clipping_flag` (0x1845d8) == 1: x = max(x, wx1), y = max(y, wy1); if
   `x + w > wx2` w = wx2 − x; if `y + h > wy2` h = wy2 − y.
3. `Disable_Draw_All_Fields`; `w -= 3`, `h -= 3`;
   `N_Moveable_Boxes_Selected_(box)` (result unused); `Set_Page_Off`;
   `Draw_Main_Screen_Filled_(1)`; window + clip again;
   `Draw_Main_Main_Screen_`; `Copy_Off_To_Back`.
4. `colors[k] = 0x0e + 2k`, k = 0..7. `r = Random(8) − 1`. `ph = 0`.
5. End corner `(tx, ty)` (C int arithmetic, x, y are the box corner):
   - x > sx, y ≥ sy: tx = x + 3, ty = y + 3.
   - x > sx, y < sy: tx = x; ty = (h + y ≤ 0x1a5 ? y + h : y) − 3.
   - x ≤ sx, y ≥ sy: tx = (w + x ≤ 0x20f ? x + w : x) − 3; ty = y.
   - x ≤ sx, y < sy: tx = (w + x ≤ 0x20f ? x + w : x) − 3;
     ty = (h + y ≤ 0x1cb ? y + h : y) − 3.
6. `stx = (tx − sx)/10`, `sty = (ty − sy)/10`; `cx = sx`, `cy = sy`.
7. For k = 0..9: `ox = w*k/10`, `oy = h*k/10`, negated when the box lies
   left of the star (x ≤ sx) resp. above it (y < sy) — exactly: x > sx keeps
   ox positive; x ≤ sx negates ox; y ≥ sy keeps oy positive; y < sy negates
   oy. `px = cx + ox`, `py = cy + oy`; px < 0x15 → 0x16, px > 0x20f → 0x20e,
   py < 0x15 → 0x16, py > 0x1a5 → 0x1a4 (16-bit); `px -= ox`, `py -= oy`.
   k > 0 → `Set_Page_Off`. `Plasma_Box(px, py, ox, oy, 3, ph, r, colors)`.
   `ph = (Random(10) + ph) % 256`; `_TOGGLE_PAGES_`; `cx += stx`, `cy += sty`.
8. Clip step 2 again for (x, y, w, h); `Plasma_Box(x, y, w, h, 3, ph, r,
   colors)`; flip; `Enable_Draw_All_Fields`; `_draw_main_screen_to_back = 1`;
   `Set_Random_Seed(seed)` (the game RNG is unaffected);
   `Restore_Window_Status_`.

`Plasma_Box` (0x136728) is an engine routine, not decoded.

## Loading the display

### Init_System_Display_(star) (0xa3fe6)

If `Need_To_Reload_System_Display_(star)`: `_system_star_id = star`;
`Set_System_Display_Loaded_Flag_(star)`; `_skip_draw_system_planets = 1`; if
`box0.content < _box_off_flag[0]` (box closed) `_skip_draw_system_display =
1`; `Load_New_System_Display_(star)`; `_system_icon_scanned = -1`; if
`Player_Sighted_Monster_(player, star)` (star.special == 9 and visited by the
player) → `Clear_System_Special_Slots_()` (0xc6f26: the special's picture
pointers 0x1a1130, 0x1a1114 and `_g_msg_ptr` cleared).

**Sighted monster** (`Draw_System_Display_Popup_` 0xa3284): when the system
is not `Ok_To_View_System_` (a black hole, or neither visited nor
omniscient) but `Player_Sighted_Monster_` holds — so only a visited black
hole with special 9 — `Hide_System_Display_(x, y, dx, dy, 0xd, 0x2f, 1)`
hatches the inside (`Diagonal_Line_Fill_` step 0x10, colours 0x3d/0x3b,
clipped to the window's y2; the leaders screen uses 0x132,0x11–0x270,0xc5),
the border when asked, then `Draw_System_Special_Popup_(x+dx+5,
y+dy+0x2f, 9, star)` (on the leaders screen x−5 and y+0x28 with the full
window) and the title `H 0x16a` with the star's name. The picture is
MAINPUPS entry 0xf + `Star_To_Monster_(star)` (star % 5 + 1); in mon1 it
draws black in the original too. Oracle mon1 (`build/oracle/mon1.sh`, s2
with Capella given special 9): `TestSightedMonsterSystem` matches.

Called by `Do_System_Popup_`, by every box draw
(`Wrapper_For_Normal_System_Popup_`) and every field build
(`Wrapper_For_Normal_System_Fields_`); it only reloads when the tag differs.

- Tag: `t = sprintf("%d> %s", star, star.name)` (fmt 0x179c46), cut to 20
  chars (`t[20] = 0` if longer than 19).
- `Need_To_Reload_System_Display_` (0xa4426): 0 if the loaded string exists
  and `strncmp(t, loaded, strlen(t)) == 0`, else 1.
- `Set_System_Display_Loaded_Flag_` (0xa4e1b): `loaded =
  Allocate_First_Block(_sys_data_buffer, 0x14)` (resets the arena), zero it,
  `sprintf(loaded, t)` (a 20-char tag writes its NUL one byte past the block;
  `_sys_disp` is reallocated over it right after).
- `Clear_System_Display_Loaded_Flag_` (0xa20cd): if the pointer is set,
  zero its 20 bytes — forces the next reload.

### Load_New_System_Display_(star) (0xa404e)

1. `Set_Palette_Changes(0, 0xff)`; `Clear_Reduced_Palette_Groups()`;
   `Mox_Update_`.
2. `_sys_disp = Allocate_Next_Block(_sys_data_buffer, 0x46)`.
3. `p = Buffer_Reload(BUFFER0, 0x49)`; **box0.width = p.width (347),
   box0.height = p.height (273)**. (This is what resizes box 0 from the
   640×480 of `Init_System_Display_Box_`; answers open question 3 of
   `main-screen-panel.md`.)
4. Sun: `_sun_seg[class] = Buffer_Reload(BUFFER0, 0x53 + star.class)`;
   `_system_display_sun_seg` = it. `Mox_Update_`. `_draw_sun_before_planet = 1`.
   `any = 0`.
5. For slot i = 0..4, `pl = star.planets[i]` (star+0x4a):
   - pl < 0: record {-1, 999, 999}, other bytes left as they were.
   - else: `dim = _rot_plan_dim[planet.type == 2 ? 4 : planet.size]`;
     `(ox, oy, pos) = Get_Orbit_Display_X_Y_(star, planet.orbit)`; if i == 0
     and (0 ≤ pos ≤ 2 or 12 ≤ pos ≤ 15): `_draw_sun_before_planet = 0`
     (only slot 0 is tested). Record: planet = pl, x = ox − dim/2, y = oy −
     dim/2, +7 = i, +6 = pos; `Get_Rotating_Planet_Segment_(&rec.pict, pl,
     pos, 0)`; +8 = `(i*5) % rec.pict.frames`;
     `Add_Reduced_Palette_Group(rec.pict)`; `any = 1`.
6. `Mox_Update_`. If any: `Compress_Reduced_Palette_Groups(0xc0, 0x40)`,
   `Update_Glass_Remap_Colors()`, `Calculate_Remap_Gray_Palettes_()`; then if
   screen 0 and `!_skip_draw_system_planets` and box 2 open → vanish the
   fleet box and redraw the mini main screen (dead when called from
   `Init_System_Display_`, which sets the skip flag first).
7. `_mainscr_palette` (0x199f1d) = 0; `Mox_Update_`.
8. `qsort_(_sys_disp, 5, 14, qsort_sys_disp_)` (0xa5045: `a.y − b.y`,
   s16 at +4); Watcom qsort (`internal/clib`), so equal y keep Watcom's order.
   Drawing goes back to front by y; `_planet_buttons[i]` follows this order.
9. If `!_look_at_colonies` and `!Ok_To_View_System_(star, player)`:
   `_star_description_string = H(0x0f + star.class)` (`Get_LBX_Message_`
   0x8f6df copies an H string; 0x0f "The blue-white stars ..." .. 0x15
   "Black holes ...").

`Ok_To_View_System_(star, p)` (0xa44fc): false for a black hole (class 6),
else `Player_Has_Visited_(p, star) || Player_Is_Omniscient_(p)`.

### Get_Orbit_Display_X_Y_(star, orbit) → (ox, oy, pos) (0xebeeb)

All C int (truncating) arithmetic, `sd = __stardate` (s32, tenths):

```
a   = ((star*7 + sd) * 0x14bf) / (orbit*100 + 0x96 + star) % 360   (then as s16)
c   = Cos(a, rx[orbit])          rx = _orbit_consts[orbit+1][0]
s   = Sin(a, ry[orbit])          ry = _orbit_consts[orbit+1][1]
ox  = c+5 >= 0 ? (c+10)/10 : -((-c)/10)
y   = s-5 >= 0 ? s/10      : -((10-s)/10)
oy  = -y
pos = (a*32 + 0xc0) / 360;  if pos == 32: pos = 0
```

Planets move along their ellipse once per stardate change; the "rotation"
seen while the box is open is the picture animation (below).

`Sin(angle, len)` (0x138615, engine): `len < 0` → `len = −len`, angle += 180;
normalise angle to 0..359; table `T` = u16[90] at 0x138405
(`_angle_to_sin`, read from the exe): 0..89 → `(T[a]*len + 0x8000) >> 16`;
90 → len; 91..179 → `T[180−a]`; 180..269 → −`T[a−180]` term; 270 → −len;
271..359 → −`T[360−a]` term (unsigned 32-bit product). `Cos(a, l) = Sin(a +
90, l)` (0x138776).

## Box geometry

Everything is relative to the box origin `X = box0.x`, `Y = box0.y`, plus the
drag offset `dx = box0.drag_dx`, `dy = box0.drag_dy` (0 unless being
dragged). The box's inner frame offsets are `L = 0xd`, `T = 0x2f` (constants
passed by the wrappers). Shorthands: `BX = X+dx`, `BY = Y+dy`.

| Thing | Position |
|---|---|
| frame (entry 0x49, 347×273) | (BX, BY) |
| title bar drag field | (BX, BY) .. (BX + W, BY + 0x2f) |
| close button (entry 0x52, 67×23, 2 frames) | (BX + 0x107, BY + 0xec) |
| orbit centre (planets, markers, fields) | (BX + L + 0xa1, BY + T + 0x5a) = (BX+0xae, BY+0x89) |
| orbit centre for pointer → orbit | (X + 0xab, Y + 0x86) (`_system_display_base`, no drag offset) |
| ring / asteroid pictures (288×154) | (BX + L + 0x10, BY + T + 0xc) |
| sun | x = (W − sun.height)/2 + BX, y = (H − sun.width)/2 + BY (the binary mixes width and height; suns are 32..35 px) |
| grid field | (BX + L, BY + T), 0x140 × 0xb7 cells of 1×1 |
| ship icons | (BX + e.x, BY + e.y), e.x = 0x12 + Σ previous (w + 3), e.y = (0x19 − h)/2 + 0xec |
| gate icons | gx = BX + 0xe9, gy = BY + 0xf3 |
| wormhole line (right-aligned) | x = BX + 0x149, y = BY + 0xe4 − font height (style 1) |
| info boxes | at (BX + L, BY + T) |

## Drawing

`Draw_Moveable_Box_(0)` → **`Wrapper_For_Normal_System_Popup_`** (0xa4fbe):
`title = sprintf(H 0x16a "Star System %s", star.name)`;
`Init_System_Display_(star)`; `Draw_System_Display_Popup_(star, 1, title, X,
Y, dx, dy, 0xd, 0x2f)`.

### Draw_System_Display_Popup_(star, border, title, X, Y, dx, dy, L, T) (0xa31da)

1. `_ship_frame = (_ship_frame + 1) % 32000`. Screen 0: `Set_Window(0x16,
   0x16, 0x20f, 0x1a5)`, `Clip_On`. `_system_display_base_x/y = X, Y`.
2. If `!_look_at_colonies` and `!Ok_To_View_System_(star, player)`:
   - **Monster sighted** (`Player_Sighted_Monster_`): `Hide_System_Display_(…,
     1)` = `Diagonal_Line_Fill_(BX+L−1, BY+T−1, BX+L+0x13f−5, min(BY+T+0xb6,
     window_y2), 0x10, 0x3d, 0x3b)` and `Set_Font_Style(4, Font_Colors2_(4,
     0x6e, 0x73))`; then the frame if `border`; screen 0x1d (officers) has its
     own branch; else `Draw_System_Special_Popup_(BX + 5, BY + T, 9, star)`
     (0xc6ff3, not covered), title `sprintf(H 0x16a, name)` and print (step 9).
     The frame is drawn after the hatching.
   - **Unexplored**: `Hide_System_Display_(…, 0)` (draws nothing, sets no
     font); return if `!border`; frame; title = `Contact_With_One_Colony_(star)`
     ? `sprintf(H 0x16a, name)` : H 0x16b "Star System Unexplored";
     `Print_System_Display_Box_Title_`; `Print_Empty_System_Data_(star)`;
     return.
3. Frame: `Draw_System_Display_Border_` (0xa30c5) = `Draw(BX, BY,
   Buffer_Reload(BUFFER0, 0x49))`, if `border`.
4. `_system_star_id_for_auto_select = star`.
5. Wormhole (`star.wormhole_to` ≥ 0): `Set_Font_Style(1, Font_Colors2_(1,
   0x6e, 0x73))`; text = target visited by the player ? `sprintf(H 0x16c
   "Wormhole links %s", target.name)` : H 0x16d "Stable Wormhole";
   `Print_Right(BX + 0x149, BY + 0xe4 − Get_Font_Height(), text)`. (Screen
   0x1d: (0x270, 0xbb).) The `wormhole_to < 0` branch in this block is dead.
6. Sun before planets if `_draw_sun_before_planet` (`Draw_Sun_Seg_` 0xa3025,
   reloads entry 0x53 + class). Planets: if `_skip_draw_system_planets`
   clear it, else `Draw_System_Display_Planets_` and `clipping_flag = 1`.
   Sun after planets if `!_draw_sun_before_planet`.
7. Colony markers, for each `_sys_disp` slot with a planet that has a colony
   (planet.colony ≥ 0): `mx = BX + L + 0xa1 + rec.x − 2`, `my = BY + T + 0x5a
   + rec.y − 2`; asteroid (type 1) adds `dim/2` to both (dim of size 4 if gas
   giant, else of the size). Owner `o = colony.owner`, banner `b =
   player[o]+0x26`. Outpost (colony+6 ≠ 0): `Draw_Outpost_Marker_` =
   `Draw(mx − pic.w, my, BUFFER0 0x2b + b)`; else `Draw_Colony_Marker_` =
   `Draw(mx − pic.w + 2, my, BUFFER0 0x23 + b)` (8×7 flags).
8. Hovered planet: for each slot with a planet and `rec.slot ==
   _system_icon_scanned`: `hx = BX+L+0xa1+rec.x−2`, `hy = BY+T+0x5a+rec.y−2`
   (no asteroid shift). If type ≠ 1: `Draw_System_Highlight_(planet, dim,
   dim, hx, hy)`. Then the info box: `Do_Print_Planet_Info_` in measure mode
   → (w, h); `Draw_Darkened_Box_(BX+L, BY+T, w + 9, h + 4, 0x3f)`;
   `Do_Print_Planet_Info_` in print mode.
9. `Draw_System_Ship_Icons_`, `Draw_Gate_Icons_`, and if `2000 ≤
   _system_icon_scanned < 2004` `Print_Gate_Info_(star, scanned − 2000, …)`
   (0xa4989, not covered).
10. Screen ≠ 0x1d: `Print_System_Display_Box_Title_(X, Y, dx, dy, title)`.

#### Draw_System_Display_Planets_ (0xa30fd)

For slot i = 0..4 in `_sys_disp` order with a planet: type 2 or 3 →
`Draw_Planet_With_Pierced_Ring_(planet, i, …)`; type 1 (asteroid belt) →
`Set_Animation_Frame(BUFFER0 0x5b, rec.slot)`, `Draw(BX + L + 0x10, BY + T +
0xc)`. Entry 0x5b is 288×154, 5 frames, the asteroid belt of orbit 0..4.
Orbits without a planet get no ring. (Entry 0x5a, the same with plain rings,
is not used here.)

#### Draw_Planet_With_Pierced_Ring_(planet, i, …) (0xa2d7c)

`rec = _sys_disp[i]`; `sz = type == 3 ? size : 4`.

1. `Push_Block(_screen_seg)`; `ring = Allocate_Next_Block(Get_Bitmap_Size(
   0x120, 0x9a) + 10)`; `pb = Allocate_Next_Block(Get_Bitmap_Size(0x22,
   0x22) + 10)` (the 0x23×0x23 bitmap made below is larger than this block).
2. `x0 = BX + L + 0x10`, `y0 = BY + T + 0xc`.
3. `_o_ring_seg[slot] = Buffer_Reload(BUFFER0, 0x4d + rec.slot)` (288×154,
   one ellipse); `Draw_Picture_To_Bitmap(ring, it)`.
4. `px = BX + L + 0xa1 + rec.x − 2`, `py = BY + T + 0x5a + rec.y − 2`.
5. `mask = _planet_mask_seg[sz] = Buffer_Reload(BUFFER0, 0x8f + sz)` (31×31,
   31 frames, kind 1 bitmap of light levels 0..10);
   `Set_Animation_Frame(mask, rec.pos)` (clamped: pos 31 → 30).
6. `Get_Rotating_Planet_Segment_(&rec.pict, planet, rec.pos, 0)`;
   `Calculate_Picture_Remap_Colors(rec.pict)`; `Create_Blank_Bitmap(0x23,
   0x23, pb)`; `Add_Picture_To_Bitmap(2, 2, rec.pict, pb)` (at the picture's
   current frame, left by the previous draw); `Outline_Bitmap(pb, pb, 0x1f,
   0)`; `Outline_Bitmap(pb, pb, 0x1e, 0)`;
   `Mask_Out_Bitmap_With_Bitmap(px − x0 − 2, py − y0 − 2, ring, pb, 0)` —
   the ring is cut where the planet plus a 2-pixel outline covers it.
7. `Draw(x0, y0, ring)`.
8. Reload the mask, `Set_Animation_Frame(mask, rec.pos)`; reload the planet
   picture; `Set_Animation_Frame(pict, _ship_frame % pict.frames)`;
   `Remap_Draw_With_Mask_(px, py, pict, mask)`.
9. `Pop_Block`.

`Get_Rotating_Planet_Segment_(dst, planet, pos, 0)` (0xa3d84): `c =
planet.climate` (+8); if the planet has a colony (index < `NUM_COLONIES`),
type ≠ 2 and the colony is not an outpost, `c = colony+0xe2` (colony
climate byte); `n = type == 2 ? 50 : c*5 + min(size, 4)`; `*dst =
Buffer_Reload(BUFFER0, 0x5c + n)`. Entries 0x5c..0x8d are climate 0..9 ×
size 0..4 (all 31×31, 39 or 40 frames, own palette chunk, verified by
rendering: e.g. 0x80 = arid size 1), 0x8e the gas giant. `pos` is unused.

#### Remap_Draw_With_Mask_(x, y, pict, mask) (0xa512e)

Draws a sprite (kind 0) frame of `pict` through the picture remap table, with
the light mask:

- Clipping (when `clipping_flag == 1`): fully outside the window → return;
  partly outside → `Remap_Clipped_Draw_Animated_Sprite_Mask_` (0xa54f2), else
  `Remap_Draw_Animated_Sprite_Mask_` (0xa53dc). Kinds 1..3 →
  `Exit_With_Message("Remap draw not support …")`.
- Scan list (refresh > 1): the unclipped path adds the picture rect **and a
  second rect offset by (+100, +100)**; the clipped path adds the clipped rect.
- Per pixel: `v = remap[pix]` (`picture_remap_color_list` 0x1bc79c); mask
  byte `m = maskframe[row*pict.width + skip + k]` where `skip` is the current
  run's own skip word (not the running x) and k the index in the run; `m ==
  0` → v, else `0x19c714[m*256 + v]`, i.e. `_remap_gray_mask_palette`
  (0x19c814) level m−1. Rows: the frame's top word (+2) then (count, skip)
  word records, count 0 = next row(s); data word-aligned.
- Afterwards both pictures advance one frame with the usual repeat/hold rules
  (`advanceFrame`), the mask's on its own header.

`Calculate_Remap_Gray_Palettes_` (0xa5050): `Sort_Palette_List()`; for level
l = 0..9, `pct = _remap_gray_percentages[l]` (0x1823e0: 10, 20, …, 100), for
each colour c: `f = 100 − pct`, `table[l][c] = Find_Closest_Color(R*f/100,
G*f/100, B*f/100)` of the current palette. Level 9 is black.

The planet pictures carry their own palette chunks; `Add_Reduced_Palette_Group`
/ `Compress_Reduced_Palette_Groups(0xc0, 0x40)` squeeze all of a system's
planet palettes into colours 0xc0..0xff and `Calculate_Picture_Remap_Colors`
fills the remap table for one picture. That subsystem (0x13aac1..0x13ac01,
0x133c9c) is not decoded here.

#### Draw_System_Highlight_(planet, dimx, dimy, hx, hy) (0xa3a15)

`p = _o_highlight_seg = Buffer_Reload(BUFFER0, 0x4b)` (35×35, 7 frames, four
red corner brackets; animates through `Draw`). Position `(hx − (p.w −
dimx)/2, hy − (p.h − dimy)/2)`. No colony: `Replace_Color(p, 0x49+k, 6+k)`
for k = 0..8, draw, `Replace_Color(p, 6+k, 0x49+k)` (in the picture data, so
pixels that were already 6..14 also turn into 0x49..). Colony with banner `b
= player[owner]+0x26 ≠ 0`: replace `0x49+k → C+k` with `C =
_main_palette_player_colors[b]` (0x10010: 73, 98, 110, 32, 62, 148, 45, 85),
draw, replace back; banner 0 draws unchanged.

#### Ship icons

`Build_System_Popup_Ships_(star)` (0xa200e, from the field builder):
`Clear_System_Display_Ship_Fields_` (0xa20ec: zero the 15 entries, field
−1000, node −1); `_player_has_ship_in_system = 0`; `n = 0`, `cur = 0`; for
each map ship icon in icon order whose first ship (`_ship_node` of the icon)
is at `location (+0x65) == star` with `status (+0x64) < 3`: owner `o =
ship+0x63`; `e[o].node = icon node`; o == player → has-ship = 1;
`Get_System_Display_Ship_Icon_XY_` (0xa3e3d): `pic =
Buffer_Reload_System_Ships_(o)`, `e[o].x = cur + 0x12`, `e[o].y = (0x19 −
pic.h)/2 + 0xec`, `cur += pic.w + 3`. One icon per owner; a later icon of the
same owner overwrites the node but still advances `cur`.

`Buffer_Reload_System_Ships_(o)` (0xa5844): owner < 8 → BUFFER0 0x111 +
banner; 8 → 0x119; 9..14 → 0x11a + (o − 9); else none (8 frames each).

`Draw_System_Ship_Icons_` (0xa3bc4), owners 0..14 with a node: at (BX + e.x,
BY + e.y); hovered (`_system_icon_scanned − 1000 == node`): measure
`Do_Print_System_Ship_Data_` (0xa26e3), `Draw_Darkened_Box_(BX+L, BY+T, w+9,
h+4, 0x3f)`, print, then the icon; otherwise `Reset_Animation_Frame` then
draw.

#### Gate icons (0xa2c66)

`x = BX + 0xe9`, `y = BY + 0xf3`. Artemis net: `x −= 12`, draw BUFFER0 0x21
at (x, y). Dimensional portal: BUFFER0 0x15 at (x − 12, y + 1).

#### Title: Print_System_Display_Box_Title_(X, Y, dx, dy, s) (0xa4c41)

1. `C = Font_Colors2_(4, 0x3c, 0x41)`; `Set_Font_Style(4, C)`; `w =
   width(s)`; w > 0x109 → `Set_Font_Style(3, C)`; `Set_Font_Spacing(w < 0x3c
   ? 4 : 2)`; `st`, `sp` = current style and spacing; `h = Get_Font_Height`.
2. `cx = pic49.width/2 + X + dx` (= BX + 173); `cy = (0x14 − (h+1))/2 + Y + dy
   + 0x11`.
3. `Set_Fitted_Font_Style_(s, C, st, sp, 0, 0x131, 1)`; `st2`, `sp2` =
   result; `C = Font_Colors2_(st2, 0x3c, 0x41)`; `Set_Font_Style(st2, C)`;
   `Set_Font_Spacing(sp2)`; st2 < 3 → `cy += 3 − st2`.
4. `_Print_Centered_Shadow_Down_(cx, cy, 0x3d, s)` (0x90089: `x − width/2`,
   then `_Print_Shadow_Down_` 0x900ce, not traced).

#### Planet info box: Do_Print_Planet_Info_(X, Y, dx, dy, planet, L, T, &w, &h, print) (0xa2123)

From Ghidra pseudocode; the register-carried colony index needs checking in
the assembly before porting. Font style 2, colours `Font_Colors2_(2, 0x3c,
0x43)`, spacing 1, line `lh = height + 1`; text at `x = X + dx + L + 6`, `y =
Y + dy + T + 7`; measure mode keeps `w = max width`, `h` starts at 0xc and
adds a line per row, +2 at the end. `detail` = player has a ship in the
system, a colony at the star, or is omniscient.

- Gas giant: H 0x15f "Gas Giant (uninhabitable)". Asteroid: H 0x160
  "Asteroid (uninhabitable)". One line each.
- Else: line 1 `"%s "` of `Get_Planet_Name_No_Prime_(planet)`, plus for a
  colony `"(%s"` race name (player+0x15) and `")"` or H 0x161 " Outpost)";
  line 2 `"%s, %s"` size and climate strings (climate = colony+0xe2 when
  colonised); line 3 `H 0x162 "%d max pop"` or `H 0x181 "%d / %d pop"`
  (colony+0x0a, `Planet_Max_Population_For_Player_`); line 4 minerals, plus
  gravity unless normal (`"%s, %s"` / `"%s"`); then
  `Print_Special_Building_Data_` (0xa58ce) if colonised and `detail`; then
  the planet special string unless special is 0 or 11 (4 px gap if no
  building lines).

#### Unexplored text: Print_Empty_System_Data_(star) (0xa573b)

Screen 0: `x0 = BX + L`, `y0 = BY + T`, `mw = W − 0x33`, `aw = W − 0x1a`,
`ah = H − 0x5b`. Window (0x16,0x16,0x20f,0x1a5) + clip;
`Change_Font_Style_(3, desc, 3)` (0x8ebbb); `pw =
_Get_Paragraph_Max_Width_(mw)` (0x103971); `Print_Paragraph_Centered_
Vertically_((aw − pw)/2 + x0, y0, mw, ah, desc, 3)` (0xc5ed3); restore window.

## Fields

`Add_Moveable_Box_Fields_` → **`Wrapper_For_Normal_System_Fields_`**
(0xa4f58): `Init_System_Display_(star)`; `Add_System_Display_Popup_Fields_(
star, X, Y, dx, dy, 0xd, 0x2f, &drag_field, &field2, &field3)`.

### Add_System_Display_Popup_Fields_ (0xa1c74)

Nothing if the star is out of range or `_skip_draw_system_display`. Fields in
this order (lower numbers win hit tests):

1. `Build_System_Popup_Ships_(star)`.
2. `field2` (close) = `Add_Clipped_Button_Field(BX + 0x107, BY + 0xec, "",
   entry 0x52, "\x1b", 0x28)` — hot key Esc (string at 0x179c1c). The engine
   returns -1000 without adding when clipping is on and the button sticks
   out of the window (x < wx1, x + w > wx2, y < wy1 or y + h > wy2).
   `drag_field = Add_Hidden_Field(BX, BY, BX + W, BY + 0x2f, "", 0)`. (With
   a null close pointer — the informational variant — `drag_field = -1000`.)
3. Planets, slots i = 4 down to 0 of `_sys_disp`: empty or asteroid →
   `_planet_buttons[i] = -1000`; else `(ox, oy) = Get_Orbit_Display_X_Y_(star,
   orbit)` (recomputed), `x1 = ox + L + BX + 0xa1 − dim/2`, `y1 = T + BY +
   0x5a + oy − dim/2`, `Add_Hidden_Field(x1, y1, x1 + dim, y1 + dim, "",
   0x29)`.
4. Ships, owners 0..14 with a node: picture reloaded into e.pict, field
   `Add_Hidden_Field(BX + e.x, BY + e.y, BX + e.x + w, BY + e.y + h, "",
   0x29)`.
5. `_gate_icon_field[0..3] = -1000`; `gx = BX + 0xe9`, `gy = BY + 0xf3`.
   Artemis net: `_gate_icon_field[3] = Add_Hidden_Field(gx − 12, gy, gx + 3,
   gy + 15, "", 0)`, `gx −= 12`. Dimensional portal: `_gate_icon_field[0] =
   Add_Hidden_Field(gx − 12, gy, gx + 3, gy + 15, "", 0)`.
6. Screen 0: `_system_display_grid_field = Add_Grid_Field(BX + L, BY + T, 1,
   1, 0x140, 0xb7, &grid_x, &grid_y, 0)`.
7. `field3 = Add_Hidden_Field(BX, BY, BX + W, BY + H, "", 0)` (the whole box;
   catches clicks so the map below does not get them).

### Main_Screen_ input for the box (0x86987..0x86b09)

If box 0 open, `!_skip_fields`, `0 ≤ _system_star_id < NUM_STARS` and `r !=
field2`: `ret = Scan_System_Display_Fields_(h, r, _planet_buttons,
&hov_planet, &clk_planet, &hov_ship, &clk_ship, &icon)`;
`_system_ship_scanned = hov_ship`. Then:

- `icon == -1` and `2000 < _system_icon_scanned < 2004` → -1 (a hovered gate
  2000 is not cleared here); else `icon > -1` → `_system_icon_scanned =
  icon`. A planet or ship hover therefore stays lit after the pointer leaves
  it, until it leaves the box:
- pointer outside `[x, x+W] × [y, y+H]` (inclusive) → `_system_icon_scanned
  = -1`.
- `ret == 0` (a click): clicked ship → `Do_Fleet_Popup_(Ship_Icon_Id_From_
  Node_Ptr_(clk_ship), 0)` (fleet box spec; oracle fbox1,
  `TestFleetBoxFromSystemDisplay`: Zuban's ship icon in s2; the icon
  cells' rings cycle their colours by the clock); else `pl = Planet_Index_(star,
  clk_planet)` (0x79c54), its colony `c`; `0 ≤ c < NUM_COLONIES` and
  `colony.owner == _PLAYER_NUM` → `Do_Colony_Screen_(c, &leave)`. Clicks on
  others' or empty planets do nothing.

Then `Scan_Moveable_Box_Fields_` (close, bring to top) as in
`main-screen-panel.md`.

### Scan_System_Display_Fields_(h, r, buttons, &hovP, &clkP, &hovS, &clkS, &icon) (0x899ab)

All outputs −1, `ret = −1`, `gate = −1`.

1. Ships, k = 0..14 with a picture: `r == e[k].field` → `clkS = e[k].node`,
   ret 0, stop.
2. If ret == −1: same with `h` → `hovS`, ret 4.
3. If ret == −1: planets i = 0..4: `r == buttons[i]` → `clkP =
   Get_Sint_Index_From_List_(_sys_disp[i].planet, star.planets, 5)` (orbit
   slot), ret 0, stop. `r == −buttons[i]` (right click): gas giant → return
   −1; else `Potential_Colony_Info_Popup_()` (0xa453f, below), 0x19994c
   = 1 (next `Draw_Main_Screen_` sets `_skip_fields`), return −1. `h ==
   buttons[i]` → `hovP` = slot, ret 4 (loop continues).
4. If ret == −1: `cx = base_x + 0xab`, `cy = base_y + 0x86`; `r ==
   _system_display_grid_field` → `o = Get_Orbit_From_X_Y_(cx − px, cy − py)`;
   first slot j whose planet has orbit o and type 1 → `clkP = j`, ret 0.
5. If still −1: if `base_x < px < base_x + W` and `base_y < py < base_y + H`,
   `o` as above, else −1; first asteroid slot with orbit o → `hovP = j`, ret 4.
6. If still −1: g = 0..3, `h == _gate_icon_field[g]` → gate = g, ret 4.
7. `icon = hovP ≥ 0 ? hovP : hovS ≥ 0 ? hovS + 1000 : gate ≥ 0 ? gate + 2000
   : −1`. Return ret.

So asteroid belts have no button: clicks come through the grid field, hover
through the pointer position.

### Get_Orbit_From_X_Y_(dx, dy) (0xebfcb)

`dx == 0 && dy == 0` → −1. `best = 0xffffffff` (unsigned), `bi = −1`. For i = 6
down to 0 with `rx, ry = _orbit_consts[i]`: `a = Get_Angle(dx*ry, rx*(−dy))`;
`c = Cos(a, rx)`, `x' = c+5 ≥ 0 ? (c+10)/10 : −((−c)/10)`; `s = Sin(a, ry)`,
`y' = s−5 ≥ 0 ? s/10 : −((10−s)/10)` (not negated); `d = (y' + dy)² + (x' −
dx)²` as unsigned; `d < best` → best = d, bi = i. Result `1 ≤ bi ≤ 5 ? bi − 1
: −1`. (Both axes are mirrored against the display formula; an ellipse is
point-symmetric, so the nearest ring is the same.)

`Get_Angle(x, y)` (0x1384b9, engine): x == 0 → y < 0 ? 270 : y == 0 ? 0 :
90; y == 0 → x < 0 ? 180 : 0; else fold into the first quadrant (x<0,y<0:
(−x,−y)+180; x<0,y>0: (y,−x)+90; x>0,y<0: (−y,x)+270; else +0); `x == y` →
45; `x < y` → `90 − A[(x<<7)/y]`; else `A[(y<<7)/x]` (unsigned division,
`A` = u8[128] at 0x138385); add the correction; 360 → 0.

## Closing, dragging, top

- **Close**: the Close button or Esc → `r == field2` → in
  `Scan_Moveable_Box_Fields_` (skip == 0): `Set_Page_Off`,
  `Draw_Main_Screen_`, flip, `Vanish_System_Display_Box_` (0x91abc):
  `content = _box_off_flag[0]`, `_planet_buttons[0..4] = −1000`, ship fields
  (0x1931c0 + 12k, k < 8 only) = −1000, `drag_field = field2 = −1000`. The
  loaded tag is kept, so reopening the same star does not reload.
- **To top**: a click on `drag_field` or `field3` (`Box_Field_Selected_`
  0x90605) while box 0 is not `order[0]` → `Move_Box_To_Top_(0)` (0x9128c:
  take it out, put it at index 0, keep the others' order).
- **Drag** (`Move_Moveable_Box_(0, button)` 0x91349), started from
  `Draw_Main_Screen_` when a button is held and `Auto_Input() == drag_field`
  or `dragging == 1`:
  - `button ≠ 0` and (Auto_Input == drag_field or dragging): close the help
    list, `_mouse_button_pressed = button`, disable pressed-field update and
    field drawing, `_skip_fields = 1`; if `_stencil_box_id == −1`:
    `_draw_main_screen_stencilled = 1`, `_stencil_box_id = 0`. `drag_dx =
    Pointer_X − Pressed_Mouse_X`, `drag_dy` likewise; clamp: `x + dx < 0x16`
    → `dx = 0x16 − x`; `x + dx > 0x20e − W` → `dx = 0x20f − W − x`; `y + dy <
    0x16` → `dy = 0x16 − y`; bottom: right button → `y + dy + 0x2b > 0x1a4`
    → `dy = 0x1a5 − y − 0x2b` (only the 0x2b-pixel title must stay on the
    map); else `y + dy > 0x1a4 − H` → `dy = 0x1a5 − H − y`. `dragging = 1`.
  - otherwise (released): `_draw_main_screen_to_back = 1`,
    `Enable_Draw_All_Fields`, `Set_Main_Screen_Help_List_`, `Clear_Fields`,
    `Clear_Moveable_Box_Fields_(0)`, `_skip_fields = 1`; if dragging: `x +=
    Pointer_X − Pressed_Mouse_X`, `y += …`, `_stencil_box_id = −2`; clamp x
    to 0x16..0x20f − W (same tests as above), y ≥ 0x16, bottom with 0x2b when
    `_mouse_button_pressed == 2`, else H; `drag_dx = drag_dy = dragging = 0`;
    `Move_Box_To_Top_(0)`; `_draw_main_screen_to_back = 1`;
    `Set_Main_Screen_Help_List_`.
  - While `_stencil_box_id` is 0..4, `Draw_Main_Screen_` runs
    `Draw_Main_Screen_While_Moving_Boxes_` (0x84db2): first call (stencilled
    flag) → `Draw_Main_Screen_Before_Stencil_` (0x849cc: map into both pages
    without the moving box, other open boxes each get
    `Move_Moveable_Box_(b, Mouse_Button())` — with their own drag field not
    active this takes the release path, so they are moved to the top — and
    `Draw_Moveable_Box_`, then the next and game buttons, then the moving
    box); later calls draw the other boxes, `Move_Moveable_Box_(stencil,
    Mouse_Button())`, draw the moving box, `Print_Main_Screen_Data_`; after a
    release (`-2`): `Set_Page_Off`, `Draw_Main_Screen_`, `_stencil_box_id =
    −1`.
  - The drawing during a drag uses `X + drag_dx`; fields are rebuilt after
    the release from the new `x, y`.

## LBX entries (BUFFER0.LBX)

| Entry | Size | Frames | Use |
|---|---|---|---|
| 0x15 | 8×8 | 1 | dimensional portal icon |
| 0x21 | 11×10 | 1 | Artemis net icon |
| 0x23..0x2a | 8×7 | 1 | colony flag by banner |
| 0x2b..0x32 | 8×7 | 1 | outpost flag by banner |
| 0x49 | 347×273 | 1 | box frame: title bar, star field, bottom strip with CLOSE art; size = box 0 size |
| 0x4b | 35×35 | 7 | hover brackets (colours 0x49..0x51 recoloured) |
| 0x4d..0x51 | 288×154 | 1 | orbit ring 0..4 |
| 0x52 | 67×23 | 2 | CLOSE button, frame 1 lit |
| 0x53..0x59 | 32..35 | 1 | sun by spectral class 0..6 (6 = black hole) |
| 0x5a | 288×154 | 5 | all orbit rings (unused by the box) |
| 0x5b | 288×154 | 5 | asteroid belt, frame = orbit |
| 0x5c..0x8d | 31×31 | 39/40 | rotating planet, climate×5 + size, own palette |
| 0x8e | 31×31 | 40 | rotating gas giant |
| 0x8f..0x93 | 31×31 | 31 | light mask by size (0x93 gas giant), frame = position |
| 0x111..0x118 | 17×14 | 8 | player ship icon by banner |
| 0x119 | 18×15 | 8 | owner 8 |
| 0x11a..0x11f | up to 24×21 | 8 | owners 9..14 |

Strings (HESTRNGS.LBX via `H_Message_`): 0x0f..0x15 star descriptions,
0x15f..0x162, 0x16a "Star System %s", 0x16b "Star System Unexplored", 0x16c
"Wormhole links %s", 0x16d "Stable Wormhole", 0x181 "%d / %d pop".

## Engine primitives needed

| Primitive | Addr | In `internal/engine`? |
|---|---|---|
| Draw, Set_Animation_Frame (clamps), Reset_Animation_Frame, frame advance | | yes (`Draw`, `setAnimationFrame`, `advanceFrame`) |
| Set_Window, Clip_On/Off, Save/Restore_Window_Status | | window yes; status save no (trivial) |
| Buffer_Reload cache (frame state survives between calls) | 0x127c27 | no (see `main-screen-stars.md`) |
| Add_Hidden_Field, Add_Grid_Field, Add_Button_Field | | yes |
| Add_Clipped_Button_Field | 0x11c3c5 | no (button field + window test) |
| Auto_Input, Mouse_Button, Pointer_X/Y | | yes |
| Pressed_Mouse_X/Y | 0x113f32 / 0x113f5f | state exists (`pressedX/Y`), no accessor |
| Disable_Mouse_Pressed_Field_Update, Deactivate_Help_List | | yes |
| Push/Pop_Block, Allocate_First/Next_Block | | n/a (Go allocation) |
| Create_Blank_Bitmap, Draw_Picture_To_Bitmap, Add_Picture_To_Bitmap, Outline_Bitmap, Mask_Out_Bitmap_With_Bitmap, Get_Bitmap_Size | 0x12d8f5, 0x12f578, 0x12f7e6, 0x12e64f, 0x12efbd, 0x12daa4 | no (first two specified in `main-screen-stars.md`) |
| Replace_Color (in-place, all frames; kind 0 runs, kind 1 bytes, kind 2 banks) | 0x13719f | no |
| Reduced palette groups, Calculate_Picture_Remap_Colors, Update_Glass_Remap_Colors, Sort_Palette_List, Find_Closest_Color, Set_Palette_Changes | 0x13aac1.., 0x133c9c, 0x133237, 0x13372a, 0x1338c9, 0x1318d4 | only `SetPaletteChanges`; the rest not decoded |
| Remap sprite-with-mask drawing (game module 63) | 0xa512e.. | no |
| Sin, Cos, Get_Angle (tables at 0x138405, 0x138385 read from the exe) | 0x138615, 0x138776, 0x1384b9 | no |
| Plasma_Box | 0x136728 | no, not decoded |
| Diagonal_Line_Fill_, Draw_Highlight_Box_, _Darken_Fill_ (game, module 55) | 0x8ec70, 0x8f0bd, 0x90006 | no |
| Print_Right, Set_Font_Style, Set_Font_Spacing, Get_Font_Height, width | | yes (`Text`, `Printer.SetSpacing`) |
| Set_Fitted_Font_Style_, Font_Colors2_ | | yes in game (`panel.go`, `stars.go`) |
| _Print_Shadow_Down_, paragraph printing | 0x900ce, 0xc5ed3, 0x103971 | no |
| qsort_ | 0x133ef4 | yes (`internal/clib`) |
| Random, Get/Set_Random_Seed | | yes (`internal/random`) |

## Open questions

1. `Do_Print_Planet_Info_`, `Do_Print_System_Ship_Data_`, `Print_Gate_Info_`,
   `Print_Special_Building_Data_`, `Potential_Colony_Info_Popup_` and
   `Draw_System_Special_Popup_` are summarised or only named; the planet
   info lines need an assembly pass (the colony index is carried in
   registers Ghidra lost).
2. Reduced palette groups and `Calculate_Picture_Remap_Colors` decide the
   planets' colours; without them the remap table is unknown.
3. `Plasma_Box` (expanding help) is not decoded.
4. `_skip_draw_system_display` is set only when `Init_System_Display_` runs
   with the box closed (e.g. a reload from `Restore_Boxes_` before the
   content is restored?); not traced.
5. `Clear_System_Display_Loaded_Flag_` after a load: a saved game with box 0
   open would reload on first draw; `Main_Screen_` entry already clears the
   tag when box 0 is open (`main-screen-background.md`).
6. Ship icon builder: whether several icons of one owner at a star can occur
   (each advances the x cursor but only the last is kept).

## Info boxes (ported in `internal/game/sysinfo.go`)

From the assembly; answers open question 1 for the planet, ship and gate
boxes (`Draw_System_Special_Popup_` stays open). Measure mode (`print` = 0) only fills `w`, `h` (s16, signed max for
`w`); print mode prints with `Print_` at `movsx` 16-bit coordinates.

### Potential_Colony_Info_Popup_(planet) (0xa453f)

`Compute_Potential_Colony_Info_(player, planet)` gives info[0..6]. The text
(a `Text_Box_` titled `sprintf(H 0x178, Get_Planet_Name_No_Prime_)`):
`sprintf("\x07M25,30.\x1a0%s", H 0x170) + "\x1a1" + "%d\n"` filled with
`Planet_Max_Population_For_Player_`; then "\x1a0%s\x1a1%d\n" lines: H 0x171
with info[4]/2 (or "\x1a0%s\x1a10\n" when 0), H 0x172 info[5], H 0x173
info[6]; then for info[1] (H 0x174, 0x175, 0x18a/0x189) and info[3] (H 0x176,
0x177, 0x18b/0x18c): 0 → "\x07M25,27." and "\x1a0%s\x1a1--\n" with the first;
else "\x07M25,17.", the second H printed into the line buffer and at once
overwritten by "\x1a0%s\x1a1" with H 0x18a/0x18b (positive) or 0x189/0x18c,
then "%d%%\n" of the absolute value (**quirk**: H 0x175 and 0x177 never
show). Called from `Scan_System_Display_Fields_` on a right click on a
planet that is not an asteroid belt (type 2); then a full redraw.
Ported (`potentialColonyInfoPopup`). Oracle: `build/oracle/pop1.sh`, a
right click on Sol's free planet in s2; `TestMainScreenPopups` matches.

### String tables

- `Load_E_Strings_` (0xcdf65), language 0: `Farload_Data_Static_
  ("estrings.lbx", 0, 0x1a1370, 0, 1, 0x5208)`; 0x32c consecutive
  NUL-terminated strings, pointers at 0x1a6578; overflow exits with "you blew
  the array eric". Then 13 bytes from 0x179eff ("maintext.lbx\0") are copied
  over string 0x170. `E_Strings_(i)` (0xcdf5c) returns pointer i.
- `Load_H_And_Pro_File_Strings_` (0xce0e5) fills pointer tables from
  `E_Strings_` with immediate indices: sizes 0x192be0 = {0x2ab, 0x1e0, 0x173,
  0x168, 0x143} "Tiny".."Huge"; minerals 0x192bf4 = {0x2ac, 0x1a7, 0x2ad, 0x1c2,
  0x2ae} "Ultra Poor".."Ultra Rich"; gravity 0x192c74 = {0x2af, 0x2b0, 0x2b1}
  "Low G", "Normal G", "Heavy G"; climates 0x18f990 = {0x21b, 0x2cf, 0x2d0,
  0x2d1, 0x2d2, 0x18f, 0x1f5, 0xb8, 0x2d3, 0x12f} "Toxic".."Gaia"; planet
  specials 0x18f9b8 = {0x2b9..0x2c4, 0x2b5..0x2b8} "No Special", "Stable
  Wormhole", …, "Orion Special" (11), "Bombarded", "Hostile", "Life
  Supporting", "Lifeless".
- `Load_Tech_Names_` (0x5df0a): `Far_Reload_("techname.lbx", language)`, a raw
  run of strings. Field 0 points at the start; every later record is one
  *walk*: skip the current string, skip the NUL run, point there. Tables in
  walk order (count): fields 1..82 (82), applications (212), buildings
  0x17eb3d/19 bytes (49: 0 "No Building", 15 "Biospheres", 40 "Star Base"),
  then 0x17eee0 (40), 0x17f638 (7), 0x17f6a1 (6), 0x17f803 (46), 0x17fdec
  (6), 0x17fe70 (7), 0x17ffb2 (4), 0x17ffe2 (6), 0x17fd0b (15), 0x17fd0f
  (15), 0x17f807 (46), hull singular 0x18003a/36 bytes (9, "Frigate"..), hull
  plural 0x18003e (9, "Frigates"..). The initialised pointers in the image
  all point at an empty string (0x178a04), so these names must come from the
  LBX, not from the exe.

### Get_Planet_Name_No_Prime_(planet) (0x77b42)

`Do_Get_Planet_Name_(planet, 0x18ff78, prime = 0)` (0x77e1f): `n =
Planet_Number_(planet)` (0x79c6b: occupied orbits of the star before the
planet); `sprintf("%s %s", star.name, roman[n])` with `roman` the pointer
table 0x17d5e4 ("I", "II", …); the prime variant replaces `roman[n] == "I"`
by H 0xa0. Cut to 30 characters.

### Planet_Max_Population_For_Player_(planet*, player) (0xe0b4f)

- `Size_And_Climate_Max_Population_(size, climate, tol)` (0xe0a18): `pct =
  byte[0xdd4eb + climate] + 25·tol` capped at 100 (climates: 25 ×6, 40, 60,
  80, 100), `(pct · byte[0x17d806 + size] + 50) / 100` (sizes 5, 10, 15, 20,
  25).
- `Size_And_Climate_Race_Pop_Limit_(size, climate, race)` (0xe0a93): `tol =
  race == 8 || (race < 8 && tolerant)`; aquatic (race < 8): tundra, swamp →
  terran; ocean, terran → gaia; subterranean adds `(size + 1)·2`.
- `Colony_Race_Pop_Limit_(colony, race)` (0xe0c1d): the above with planet
  size and colony+0xe2 climate, `cwde`, +5 when the colony owner's
  application 3 is known (player+0x11a == 3), +2 with building 15
  (Biospheres, colony+0x145).
- Main: no colony (`colony == -1`) or an outpost → race limit with the
  planet's size and climate for `player`, +5 for `player`'s application 3.
  Colony with pop 0 → `Colony_Race_Pop_Limit_(colony, player)`. Else the
  races present (colonist & 0xf over `pop_count` slots) are tried from 9 down
  to 0 and the largest `Colony_Race_Pop_Limit_` wins. Result s16. No live
  colony in the ten saves exceeds it.

### Do_Print_Planet_Info_(X, Y, dx, dy, planet, L, T, &w, &h, print) (0xa2123)

Registers X, Y, dx, dy; stack planet, L, T, &w, &h, print. `w = 0`, `h =
0xc`; `detail = _player_has_ship_in_system || Player_Has_Colony_(player,
planet.star) || Player_Is_Omniscient_(player)`; `Buffer_Reload(BUFFER0,
0x49)`; colours `Font_Colors2_(2, 0x3c, 0x43)`, style 2, spacing 1, `lh =
(byte)(height + 1)`; `x = X+dx+L+6`, `y = Y+dy+T+7`. Each line is printed at
(x, y) or widens `w`.

1. Gas giant (type 2) H 0x15f, asteroids (type 1) H 0x160; then the end.
2. `"%s "` planet name; colony: `"(%s"` of the owner's race name, then H
   0x161 " Outpost)" if colony+6 else ")". Print mode restores style 2 and
   spacing 1 after it.
3. `"%s, %s"` size, climate (colony+0xe2 if colonised, else planet+8): print
   `y += lh`, measure `h += lh`.
4. `sprintf(H 0x181 "%d / %d pop", colony+0x0a, max)` or `sprintf(H 0x162 "%d
   max pop", max)`, max = `Planet_Max_Population_For_Player_(&planet,
   player)`; same advance.
5. `y += lh`, `h += lh` (both modes); `"%s, %s"` minerals (planet+0xa),
   gravity (planet+6), or `"%s"` minerals for normal gravity.
6. Colony and `detail`: `n = Print_Special_Building_Data_(colony, x, &y, &w,
   &h, lh, print)`.
7. Special (planet+0xf, signed) ≠ 0 and ≠ 11: `n == 0` → `y += 4`, `h += 4`;
   `y += lh`, `h += lh`; the special's string.
8. End: `h += 2`.

`Draw_System_Display_Popup_` measures with all coordinates 0, then
`Draw_Darkened_Box_(BX+L, BY+T, w + 9, h + 4, 0x3f)` and prints.

### Print_Special_Building_Data_(colony, x, &y, &w, &h, lh, print) (0xa58ce)

Building list: 7 s16 at 0xa1c66 = {41 Star Fortress, 40 Star Base, 8
Battlestation, 26 Missile Base, 27 Ground Batteries, 42 Stellar Converter,
47 Fighter Garrison}. For each standing one (colony+0x136+id): before the
first, `y` and `h` += `Get_Font_Style_() + 3` (16-bit adds); `y += lh`;
print the building name, or `h += lh` and widen `w`. Returns the count.

### Do_Print_System_Ship_Data_(X, Y, dx, dy, node, &w, &h, print) (0xa26e3)

`w = 0`, `h = 4`; `x = X+dx+0x12`, `y = Y+dy+0x36` (screen 0x1d: +8, +0xa).
Style variable 2, spacing 1.

- Named ship: node ≥ 5000 → `sprintf(buf, ship[node−5000].name)` (the name
  is the format); else a stack whose first ship has owner ≥ 8 (Antarans,
  monsters) gets its name the same way. Non-empty: `Font_Colors2_(1, 0x6e,
  0x73)`, style 2, print at (x, y), or `w = width`, `h = height`; return.
- Fleet: `Buffer_Reload(BUFFER0, 0x49)`; ship = first ship of the node, `o =
  Ship_Owner_`; `c = _main_palette_player_colors[player[o].banner]`;
  `Font_Colors2_(1, c, c + 5)`, style 2, spacing 1, `lh = (byte)(height+1)`.
  `N_Ships_By_Type_At_Location_(ship.x, ship.y, o, …)` (0xa432f): over all
  ships of owner o with status < 3 at that x, y: design type 0 → `hull[min(
  size, 5)]++`, 1 colony, 2 transport, 4 outpost, others nothing; a total
  counts every match (unused). Lines = non-empty groups.
  1. `sprintf(H 0x163 "%s Fleet", race name)`, style/spacing re-set, at (x, y).
  2. For hull 0..5 with count > 0: `y += lh`, `"%2d %s"` with the singular
     (count 1) or plural hull name, at `x + 3` (style + 1); measure widens to
     `width + 3`.
  3. Colony ships: count 1 `sprintf(H 0x164, 1)`, else `sprintf(H 0x165)`
     **without the count**: H 0x165 is "%2d Colony ships", and the dword
     `%d` reads is the start of the line buffer (ebp−0xee = esp), i.e. the
     first four bytes of the previous line (last hull line, else the
     header) as a little-endian int; " 2 B" prints 1109406240. Then
     outposts H 0x166/0x167 and transports H 0x168/0x169 with the count.
     Same advance and x as hulls.
  4. `h = (lines + 1)·lh`.

`Draw_System_Ship_Icons_` measures (coordinates as given; they are ignored),
`Draw_Darkened_Box_(BX+L, BY+T, w + 9, h + 4, 0x3f)`, prints, then draws the
hovered icon without resetting its animation.

### Print_Gate_Info_(star, gate, X, Y, dx, dy, L, T) (0xa4989)

`x = X+dx+L`, `y = Y+dy+T`; `c` = the viewing player's palette colour,
`Font_Colors2_(1, c, c + 6)`, style 1. By `gate` (jump table 0xa4979; > 3
leaves the text empty and the width uninitialised):

- 0: H 0x17c "Dimensional Portal (", then for each player with the star's
  portal bit (star+0x40, `Player_Has_Dimensional_Portal_` 0x79d50) the
  **emperor** name (player+1) and ", ".
- 1: H 0x17a "Jump Gate". 2: H 0x179 "Star Gate".
- 3: H 0x17b "Artemis Net (", then race names (player+0x15) of the players
  with bit star+0x3f (`Player_Has_Artemis_Net_` 0x79cf9) and ", ".

`width` is the header's width only. Cases 0 and 3 cut the last two
characters (even with nobody listed, eating " (") and append ")". `pw =
width + 2`; `mw = Get_Formatted_Paragraph_Max_Width_(pw, s, 0)`, `mh =
Get_Formatted_Paragraph_Max_Height_(pw, s)` (FMTPARA.C, `docs/engine/paragraphs.md`);
`Draw_Darkened_Box_(x, y, mw + 0xc, mh + 5, 0x3f)`; `_Print_Paragraph_(x +
7, y + 5, pw, s, 0)`. The fields only exist for gates 0 and 3.
