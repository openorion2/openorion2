# Galaxy map box (moveable box 1)

Moveable box 1 of the galaxy screen is a small overview map of the whole
galaxy: one coloured dot per star, clicking a dot or an empty spot centres the
big map there. The routines behind it (`MOX\` module 56, `Draw_Galaxy_Map_Box_`
and friends) are shared with the fleet, officer, colony, colony summary,
planet summary, diplomacy and Dragon screens, which draw the same map with
their own pictures and fields.

**In this build box 1 never opens on the galaxy screen.** Its `content`
(0x192ffa) is written only by `Init_Moving_Boxes_` (0x79756) and
`Harold_Defaults_` (0x7940b) — both set the off flag 0 — by
`Scan_Moveable_Box_Fields_` closing it (off flag), `Save_And_Remove_Boxes_`
(0x919a4, off flag) and `Restore_Boxes_` (0x9191d, the value saved just
before). No key, button or cheat sets it, and `Harold_Defaults_` resets it
after every load, so a save cannot open it either. Its fields are never
added (`Add_Moveable_Box_Fields_` 0x90430 has cases for boxes 0 and 2 only)
and its picture `_moveable_box_pict[1]` (0x193084) is never written. All of
it is ported anyway, because it is reachable in principle and the shared
routines are needed by the other screens.

## Globals

| Global | Addr | Type | Notes |
|---|---|---|---|
| `_galaxy_map_width` | 0x19995a | i16 | 0xa0, `Harold_Defaults_` 0x795aa |
| `_galaxy_map_height` | 0x199970 | i16 | 0x75 |
| `_galaxy_map_scanned_star` | 0x199960 | i16 | -1 none; -1 at `Harold_Defaults_` and at `Main_Screen_` entry (0x8624e) |
| `_galaxy_map_grid_x/y` | 0x199968/a | i16 | grid field position outputs |
| `_galaxy_map_grid_field` | 0x19997a | i16 | field id; -1000 |
| `_galaxy_map_star_field` | 0x190178 | i16[NUM_STARS] | one hidden field per star |
| `_galaxy_map_star_seg` | 0x197490 | pict[11] | `gstar.lbx` 1..11 |
| `_using_colony_screen_palette` | 0x182ace | u8 | colony screen only; 0 on the galaxy screen |

`_main_screen_grid_field` is 0x1999c8, a different field (the big map's
grid, `Add_Map_Fields_` 0x8219b). `fleet-box.md` calls 0x1999c8
`_galaxy_map_grid_field`; the symbol table says otherwise.

## Pictures

`Load_Main_Screen_Pictures_` (0x135d5..0x135ff): `Far_Reload_Next_("gstar.lbx",
1+i, _global_screen_seg)` → `_galaxy_map_star_seg[i]`, i = 0..10. Each is a
3×3 dot with 6 frames. Index 0..7 is a banner colour, 8 a neutral star, 9 a
black hole on the colony screen palette, 10 a black hole.

## Draw_Moveable_Box_ case 1 (0x910f6)

`Draw_Galaxy_Map_Box_(pict = _moveable_box_pict[1], pict2 = 0x193088,
x = box1.x, y = box1.y, w = _galaxy_map_width, h = _galaxy_map_height,
ox = 8, oy = 0x26, dx = box1.drag_dx (+0x18), dy = box1.drag_dy (+0x1a),
mode = 0, flag = 0)`. The spec in `main-screen-panel.md` names the 9th and
10th arguments `closed_w, closed_h`; they are the `+0x18/+0x1a` words. ESI
holds `box1.x` (sign-extended) on entry, which matters below.

## Draw_Galaxy_Map_Box_ (0x90c4f)

Arguments: EAX pict, EDX pict2 (unused), EBX x, ECX y, stack w, h, ox, oy,
dx, dy, mode, flag. All 32-bit arithmetic; coordinates are cut to 16 bits
where they are passed on.

1. `X = x + dx`, `Y = y + dy`; `sx = 1000·0x1fa / w`, `sy = 1000·400 / h`
   (`idiv`).
2. `flag == 1` → return. `flag < 0` → go to 4.
3. Box size `bw, bh` = **box 1's** width/height (0x192ffe/0x192ffc, always
   box 1 whatever the caller), or `w, h` when `_using_colony_screen_palette`.
   If `mode != 4`:
   - `pict == 0` or colony palette: `Set_Window(X, Y, X+bw, Y+bh)`,
     `Clip_On`; unless `_current_screen == 6` or colony palette,
     `Fill(X, Y, X+bw, Y+bh, 0)`; `Clip_Off`, `Reset_Window`.
   - else (only screen 0 prints): colours `{0x62, 0x65..0x6b}`,
     `Set_Font_Spacing(2)`, `Set_Font_Style_Shadow_Down(3, c)`,
     `Set_Outline_Color(0x62, 0x65)`, `Print_Centered(X+0x57, Y+0xd,
     H_Message_(0xf3))`, `Set_Font_Default_Spacing_`.
   The box picture itself is never drawn here.
4. `Set_Window(X+ox, Y+oy, X+ox+w, Y+oy+h)`, `Clip_On`. For each star i:
   - `lx = star.x·1000 / max_map_scale`, `ly = star.y·1000 / max_map_scale`;
     `px = ox + X + lx·10/sx`, `py = Y + oy + ly·10/sy`.
   - Index `si`: black hole and colony palette → 9; black hole → 10;
     spectral class 5 → 8; owner 0..7 → `player[owner].banner` (+0x26);
     otherwise 8 if `Player_Has_Visited_(PLAYER_NUM, i)` or owner is -1 or
     -2, else **`si` keeps its previous value** (from the previous star, or
     the caller's ESI for the first). With owners always in -2..7 this never
     happens.
   - mode 0: `Draw(px-1, py-1, _galaxy_map_star_seg[si])` (the frame
     advances per draw, so the dots twinkle through the shared pictures).
     mode 1/4: `Reset_Animation_Frame` then `Draw(px-2, py-2,
     _fleet_galaxy_star_seg[si])`. mode 2: `_planet_summary_galaxy_star_seg`
     at `px-2, py-2`, animated (`_galaxy_star_anim_count`) only for the star
     of `_scanned_field` (minus 1000 if ≥ 1000), reset otherwise. mode 3:
     `_colony_galaxy_star_seg` at `px-1, py-1`, reset unless it is
     `_galaxy_map_scanned_star`.
5. `Clip_Off`, `Reset_Window`; screen 4 with a scanned star →
   `Do_Fleet_Screen_Stuff_(ox, oy, sx, sy)`; screen 0x1d →
   `Do_Officer_Screen_Stuff_`.

## Get_Galaxy_Map_Star_XY_ (0x911d5)

`(star, x, y, w, h, ox, oy, big, &px, &py)`: `half = (big ? 6 : 4)/2`;
`px = x + ox + (star.x·1000/max_map_scale)·10 / (0x7b890/w + 1) - half`,
`py = y + oy + (star.y·1000/max_map_scale)·10 / (400000/h + 1) - half`.
Note the `+1` the drawing does not have.

## Fields

`Clear_Moveable_Box_Fields_(1)` (0x908ae): drag/close/field3 -1000 like
every box, then `_galaxy_map_grid_field` and `_galaxy_map_star_field[0..
NUM_STARS-1]` = -1000. `Set_Main_Screen_Fields_To_Null_` also clears the grid
field.

`Add_Galaxy_Map_Fields_2_` (0x9023b, other screens): `Set_Window(x+dx, y+dy,
x+dx+w, y+dy+h)`; on screen 1 the grid field is first set to -1000; per star
`Get_Galaxy_Map_Star_XY_` and `Add_Hidden_Field` (px, py, px+5, py+5) or, when
big, (px-3, py-3, px+8, py+9), sound 0x29; on screen 1 `Add_Grid_Field(x+dx,
y+dy, 1, 1, w, h, &grid_x, &grid_y, 0)`; `Reset_Window`. Not ported yet (no
ported screen calls it).

`Box_Field_Selected_` (0x90605) counts `_galaxy_map_grid_field` as a field of
box 1, besides drag_field and field3.

## Scan_Galaxy_Map_Fields_ (0x890ef)

`(scan, r, &hov, &clk)`: `hov = clk = -1`, result -1. First star whose field
equals `r` → `clk`, result 0. Else first star whose field equals `scan` and
that is not a black hole → `hov`, result 4. Else `r ==
_galaxy_map_grid_field` → 2.

## Scan_Moveable_Box_Fields_, box id 1 (0x89261..0x8933b)

After the close and move-to-top tests, and **regardless of `_skip_fields`**:
`res = Scan_Galaxy_Map_Fields_(scan, r, &hov, &clk)`,
`_galaxy_map_scanned_star = hov`. If `Map_Scale_To_Zoom_Level_() !=
_max_zoom_count`:
- `res == 2`: `cx = 0x13c4 / w`, `cy = 0xfa0 / h`; `Center_Map_((grid_x·
  max_map_scale/10)·(i16)cx/10, (grid_y·max_map_scale/10)·(i16)cy/10)`
  (both cut to 16 bits).
- `res == 0`: `Center_Map_(star[clk].x, star[clk].y)`.

## Print_Galaxy_Map_Scanned_Star_ (0x879bc)

From `Draw_Main_Screen_` after the boxes, when `0 <= _galaxy_map_scanned_star
< NUM_STARS`, inside the map window with clipping on. `X = box1.x +
box1[+0x18]`, `Y = box1.y + box1[+0x1a]` (16-bit). Owner `o`:
- 0..7: `Set_Player_Font_Style_(o, 0)`, `"%s (%s)"` (name,
  `Race_Name_String_(o)`), colours **uninitialised** (stack bytes).
- -1: colours `{8, 8, 0xc, …}`, `"%s"`.
- -2: colours `{0x14, 0x14, 0x1f, …}`, `"%s (contested)"`.
- other: neither set (garbage buffer).
Then `Set_Font_Style(2, c)`, `Print(X+0xf, box1.height + Y - 0x19, buf)`.

The port uses the colours `Set_Player_Font_Style_` leaves for 0..7 and
prints nothing for other owners: both cases read undefined stack memory in
the original, and the routine is unreachable on the galaxy screen.

## Port

`internal/game/galaxymap.go`: `loadGalaxyMapPictures`, `drawGalaxyMapBox`
(mode 0 and the box header; modes 1–4 and the screen 4/0x1d tails log),
`galaxyMapStarXY`, `scanGalaxyMapFields`, `scanGalaxyMapBox`,
`clearGalaxyMapFields`, `printGalaxyMapScannedStar`.
