# Galaxy screen: zoom mode

Zoom mode is the "pick an area" state of the galaxy screen: after the zoom-in
button the map is darkened, a bright copy of the old view shows through a
box that follows the mouse, and a click picks the area to zoom into.
Everything here is in `Main_Screen_` (0x86188, module 52) and the engine. The
rest of `Main_Screen_` is in `main-screen-panel.md` and
`main-screen-background.md`; zoom out outside zoom mode, `Get_Map_Scale_`,
`Map_Scale_To_Zoom_Level_` and the zoom counters are in
`main-screen-stars.md` ("Map transform and zoom").

Addresses are ELF virtual addresses (`build/orion2.elf`). Data globals are
`0x178000 + offset` of the `wsyms -show globals` segment-2 entries.

## Globals

| Name | Addr | Type | Notes |
|---|---|---|---|
| `_zoom_in` | 0x199f0c | uint8 | 1 while in zoom mode |
| `_zoom_count` | 0x19999e | int16 | zoom level of the box (0 = closest) |
| `_max_zoom_count` | 0x19997c | int16 | |
| `_cur_map_scale`, `_max_map_scale` | 0x199992, 0x1999a0 | int16 | |
| `_cur_map_x`, `_cur_map_y` | 0x19998c, 0x199990 | int16 | |
| `_zoom_box_x`, `_zoom_box_y` | 0x19c15e, 0x19c160 | int16 | box top-left in screen pixels |
| `_temp_bitmap` | 0x19c150 | ptr | captured (bright) map, bitmap picture |
| `_whole_screen_field` | 0x19c15c | int16 | hidden field (0,0)-(0x27f,0x1df) |
| `_zoom_in_button`, `_zoom_out_button` | 0x19c176, 0x19c178 | int16 | field ids |
| `_last_field` | 0x19c154 | int16 | last cheat field (5) |
| `_zoom_box_color_increment` | 0x19c192 | uint8 | colour rotation of the box; BSS, never reset |
| `_zoom_box_counter` | 0x19c193 | uint8 | frame counter; BSS, never reset |
| `_zoom_box_increment_value` | 0x19c197 | int8 | set to 1 at every `Main_Screen_` entry (0x8626a) |
| `_draw_main_screen_to_back` | 0x199f28 | uint8 | |
| `_skip_fields` | 0x199976 | int16 | |
| `_mouse_list_zoom_mode` | 0x182175 | 5 x `s_mouse_list` | below |
| `_mouse_list_arrow` | 0x17cf00 | | |
| `_saved_boxes` | 0x199bcd | 5 x {int16 content, uint8 was_open}, stride 3 | `Box_Was_Open_(b)` (0x91999) reads `was_open` |
| `_moveable_box` | 0x192fdc | 5 x 0x1c | layout in `docs/formats/save-ships.md` |
| `_box_off_flag` | 0x1992c0 | int16[5] | |
| `zoomSteps` | `Main_Screen_` local `[ebp+0x6a]` | int | zoom-in presses since entering zoom mode |

## Main_Screen_ parts

### Get_Input result negative (0x86533)

Right after `Get_Input` (`r`): if `r < 0` and `_zoom_in == 1`:
`_zoom_in = 0`; `Set_Mouse_List(_mouse_list_arrow, 1)`; `zoomSteps = 0`;
`Assign_Auto_Function(Draw_Main_Screen_, 1)`; `Set_Main_Screen_Help_List_`.

That is all: **no `Restore_Boxes_`**, so boxes removed on entry stay closed
(and the next entry saves the closed state over them). Any negative `r`
leaves, not only the right button: the whole-screen field covers every
pixel, so a right click returns `-_whole_screen_field` (or `-` a zoom
button, see "Fields"); the Alt-F1..F10 codes (-1101..-1110) leave too.
`_draw_main_screen_to_back` is still 1 from the entry (nothing in zoom mode
clears it), so the end of this pass redraws the map in full.

(Already ported in `mainscreen_loop.go`.)

### Star twinkle reset (0x86fef)

If `(r == _zoom_out_button && cur_map_scale < max_map_scale)` or
`(r == _zoom_in_button && cur_map_scale > 10)`: for i in 0..0x47 (always 72,
not `NUM_STARS`) `star[i].+0x64 = star[i].+0x65 = -1`;
`_n_twinkling_stars` (0x19c164) = 0; `Deactivate_Help_List_`. This runs in
zoom mode too, before the branches below. Note the condition uses
`cur_map_scale`, which zoom mode never changes.

### Zoom in (0x8704d)

Condition: `r == _zoom_in_button && cur_map_scale > 10 && !_skip_fields`.
There is no `_zoom_in` test, so it also runs inside zoom mode.

1. If `zoomSteps == 0`: `Save_And_Remove_Boxes_()`. Then `zoomSteps++`.
2. If `_zoom_in == 0` (entering):
   1. `Deactivate_Help_List_`; `_zoom_in = 1`.
   2. `Set_Palette_Changes(0, 255)` (marks every entry changed).
   3. `Update_Glass_Remap_Colors()`.
   4. `Set_Refresh_Full`; `Set_Page_Off`; `Draw_Mini_Main_Screen_()` (the map
      without boxes into `off`; spec in `main-screen-background.md`).
   5. `Push_Block(_screen_seg)`; `_temp_bitmap =
      Allocate_Next_Block(_screen_seg, Get_Bitmap_Size(0x1fb, 0x191))`
      (one pixel larger each way than needed; 203324 bytes).
   6. `Screen_Bitmap_Capture(0x16, 0x16, 0x20f, 0x1a5, _temp_bitmap)`: a
      0x1fa x 0x190 (506 x 400) bitmap of `off`'s map window.
   7. `Pop_Block(_screen_seg)`: the allocation is released at once; the
      bitmap stays valid only because nothing allocates from `_screen_seg`
      while zooming. The port keeps a buffer.
   8. `_Darken_Fill_(0x16, 0x16, 0x20f, 0x1a5)` on `off` (clipping is off
      here, so no clamp; see `Tint_Fill` for which pixels change).
   9. `Copy_Off_To_Back`; `Set_Refresh_Stencil`.
   10. `Assign_Auto_Function(Draw_Main_Screen_For_Zooming_, 1)`.
3. Always: `_zoom_in = 1`; `Clear_Fields_Above(_last_field)`;
   `Add_Map_Fields_()` (zoom variant); `Set_Mouse_List(_mouse_list_zoom_mode,
   5)` (EDX = 5 is loaded before `Clear_Fields_Above` and survives it);
   `Decrement_Zoom_Count_()` (`zoom_count = max(zoom_count - 1, 0)`);
   `_draw_main_screen_to_back = 1`.

So each further zoom-in press inside zoom mode just shrinks the box by one
level, down to level 0.

### Zoom out inside zoom mode (0x87176)

Condition: `r == _zoom_out_button && _zoom_in == 1 && !_skip_fields`.
`Increment_Zoom_Count_With_Limit_()` (0x796e2): `zoom_count =
min(zoom_count + 1, max_zoom_count)`; `lvl = Map_Scale_To_Zoom_Level_()`
(the level of the current, unchanged `cur_map_scale`); if `lvl <=
zoom_count`: `zoom_count = lvl`, return 0; else return 1.

- Returns non-zero: nothing else (the box grows by one level).
- Returns 0 (box back to the full view): `zoomSteps = 0` (the return value);
  `_zoom_in = 0`; `Set_Mouse_List(_mouse_list_arrow, 1)`;
  `Assign_Auto_Function(Draw_Main_Screen_, 1)`; `Set_Main_Screen_Help_List_`.
  Again **no `Restore_Boxes_`**.

The ordinary zoom out (0x871d5, already ported) is only reached when this
branch is not taken, i.e. outside zoom mode.

### Picking the area: whole-screen field (0x873c1)

Condition: `r == _whole_screen_field && _zoom_in == 1 && !_skip_fields`
(left click anywhere outside the two zoom buttons, also on the frame).

1. `Restore_Boxes_()`.
2. `_zoom_in = 0`; `Set_Main_Screen_Help_List_`;
   `Set_Mouse_List(_mouse_list_arrow, 1)`;
   `Assign_Auto_Function(Draw_Main_Screen_, 1)`.
3. `cur_map_x += Get_Up_Scaled_Value_((int16)(_zoom_box_x - 0x15))`;
   `cur_map_y += Get_Up_Scaled_Value_((int16)(_zoom_box_y - 0x15))` — with
   the **old** `cur_map_scale` (`(v*scale)/10`, C division, 16-bit stores).
   No clamping (the box is kept inside the window, so the result is in
   range).
4. `zoomSteps = 0`; `s = Get_Map_Scale_(zoom_count)` (which zeroes the six
   parallax offsets if `s == max_map_scale`; cannot happen here).
5. `_far_star_x`, `_mid_star_x`, `_near_star_x`, `_far_star_y`,
   `_mid_star_y`, `_near_star_y` = 0x16 (in that store order; all six end
   0x16).
6. `_draw_main_screen_to_back = 1`; `cur_map_scale = s`.

`_zoom_box_x/y` are whatever the last `Draw_Main_Screen_For_Zooming_`
computed from the pointer, not the click position. `Get_New_Paralax_XYs_` is
not called.

### End of pass (0x8754a)

After `Add_Map_Fields_`: if `_zoom_in == 1`: `Set_Page_Off`,
`Draw_Main_Screen_For_Zooming_()`; else `Set_Page_Off`, `Draw_Main_Screen_()`.
Then `_TOGGLE_PAGES_`. The auto function draws the same picture while
`Get_Input` waits.

## Add_Map_Fields_ in zoom mode (0x81d7a)

When `_zoom_in == 1`: `Set_Main_Screen_Fields_To_Null_`; for b in 0..4
`moveable_box[b].drag_field` (0x192fea+0x1c·b) and `.field2` (0x192fec+…)
= -1000; `_zoom_in_button = Add_Button_Field(0xf4, 0x1ac, "", pic 6 (0x192ef8),
"+", 0x28)`; `_zoom_out_button = Add_Button_Field(0xf4, 0x1c7, "", pic 7
(0x192f00), "-", 0x28)`; `_whole_screen_field = Add_Hidden_Field(0, 0, 0x27f,
0x1df, "", 0x29)`; return. Hot-key strings: 0x179714 "+", 0x179717 "-",
0x179716 "".

`Scan_Field` returns the first matching field, so over the zoom buttons the
buttons win; the hidden field gets everything else. The cheat fields 1–5
stay. `_whole_screen_field` is not reset outside zoom mode and may hold a
stale id that a normal field reuses; the `_zoom_in == 1` test is what keeps
that harmless.

## _mouse_list_zoom_mode (0x182175)

`s_mouse_list {image, center_offset, x1, y1, x2, y2}`, count 5:

| # | image | offset | x1 | y1 | x2 | y2 |
|---|---|---|---|---|---|---|
| 0 | 6 | 0 | 0x16 | 0x16 | 0x20e | 0x1a4 |
| 1 | 1 | 0 | 0 | 0 | 0x27f | 0x15 |
| 2 | 1 | 0 | 0 | 0 | 0x15 | 0x1df |
| 3 | 1 | 0 | 0x20f | 0 | 0x27f | 0x1df |
| 4 | 1 | 0 | 0 | 0x1a5 | 0x27f | 0x1df |

Cursor image 6 inside the map window, the arrow (1) on the frame. The
entries do not overlap. (`_mouse_list_to` follows at 0x1821b1.)

## Draw_Main_Screen_For_Zooming_ (0x84bce)

No arguments; draws into the current page (`off`), does not flip.

1. `seed = Get_Random_Seed_()` (restored at the end; nothing in between uses
   the RNG).
2. Local colours `c[8]`: i < 2 or i > 5 → 0x0c; i == 2 or 5 → 0x10; i == 3,
   4 → 0x13. So `{0x0c,0x0c,0x10,0x13,0x13,0x10,0x0c,0x0c}`.
3. `_zoom_box_x = Pointer_X() > 0x16 ? Pointer_X() : 0x16`;
   `_zoom_box_y = Pointer_Y() > 0x16 ? Pointer_Y() : 0x16` (signed 16-bit
   compares; the pointer is not corrected by the cursor offset).
4. `w = Get_Map_Scale_(zoom_count) * 0x1fa / cur_map_scale`;
   `h = Get_Map_Scale_(zoom_count) * 400 / cur_map_scale` (32-bit `idiv`;
   `Get_Map_Scale_` is called twice, each may zero the parallax offsets if
   it returns `max_map_scale`, which it cannot while `zoom_count` is below
   the current level).
5. If `_zoom_box_x + w >= 0x218`: `_zoom_box_x = 0x20e - w`.
   If `_zoom_box_y + h >= 0x1a4`: `_zoom_box_y = 0x1a4 - h`.
   **Quirk:** x is tested against 0x218 but clamped to 0x20e, so for
   `_zoom_box_x + w` in 0x20f..0x217 the box sticks up to 9 pixels out of
   the map window into the frame (the bitmap does not cover that strip,
   but the box lines are drawn there). Port as is.
6. `Set_Window(bx, by, bx + w, by + h)` (bx, by = `_zoom_box_x/y`; x2/y2
   inclusive, so the box is (w+1) x (h+1)); `Clip_On`.
7. `Draw(0x16, 0x16, _temp_bitmap)`: the bright captured map, clipped to
   the box. Kind 1 bitmap: colour 0 is **transparent**, so black map pixels
   show the darkened background (`glass_remap_colors[0][0]`, never 0). If
   the box is not the full window this takes the clipped path, which in
   stencil mode marks the **unclipped** rectangle (0x16,0x16)-(0x20f,0x1a5)
   in the scan list, so the whole map window is restored from `back` and
   re-sent every frame (`main-screen-background.md`, "Draw (0x12a478) with
   clipping"; `Clipped_Draw_Bitmap_Sprite` 0x14861d).
8. `Draw_Multi_Colored_Box_(bx, by, bx + w, by + h, c,
   _zoom_box_color_increment)` (rotation passed zero-extended).
9. `_zoom_box_counter++` (uint8). If `counter % 3 == 0`:
   `_zoom_box_color_increment += _zoom_box_increment_value` (byte add).
   If `counter % 12 == 0`: `counter = 0`, `_zoom_box_increment_value = -1`.
   So after `Main_Screen_` entry (value 1) the pattern steps forward on the
   frames where the counter hits 3, 6, 9, 12 and backward (-1) from then
   on, until the next `Main_Screen_` entry. Counter and rotation persist
   across zoom sessions and screens (BSS, start at 0).
10. `Clip_Off`; `Reset_Window`; `Set_Random_Seed_(seed)`.

### Draw_Multi_Colored_Box_(x1, y1, x2, y2, colors, s) (0x7a5f7)

Registers AX, DX, BX, CX; `colors` and `s` (int16) on the stack, `ret 8`.
`s2 = 7 - s`, or 0 if that is `<= 0`. `rev[7 - i] = colors[i]`. Twelve
`Multi_Colored_Line_` calls (`docs/engine/lines.md`; colour `colors[rot &
7]`, `rot` +1 per pixel from the top/left end, clipped to the window):

| # | from | to | colours | rot |
|---|---|---|---|---|
| 1 | (x1, y1) | (x2, y1) | colors | s |
| 2 | (x1+1, y1+1) | (x2-1, y1+1) | colors | s2 |
| 3 | (x1+2, y1+2) | (x2-2, y1+2) | colors | s |
| 4 | (x2, y1) | (x2, y2) | colors | s |
| 5 | (x2-1, y1+1) | (x2-1, y2-1) | colors | s2 |
| 6 | (x2-2, y1+2) | (x2-2, y2-2) | colors | s |
| 7 | (x1, y2) | (x2, y2) | rev | s |
| 8 | (x1+1, y2-1) | (x2-1, y2-1) | rev | s2 |
| 9 | (x1+2, y2-2) | (x2-2, y2-2) | rev | s |
| 10 | (x1, y1) | (x1, y2) | rev | s |
| 11 | (x1+1, y1+1) | (x1+1, y2-1) | rev | s2 |
| 12 | (x1+2, y1+2) | (x1+2, y2-2) | rev | s |

All coordinates are int16. Three nested 1-pixel rings; corners are drawn
more than once (later lines win).

## Save_And_Remove_Boxes_ (0x919a4)

For b in 0..4:

1. `open = moveable_box[b].content > _box_off_flag[b]` (int16, indexed by
   `b`).
2. `_saved_boxes[b] = {moveable_box[b].content, open}`.
3. b == 0 and open: `Vanish_System_Display_Box_()`; b == 2 and open:
   `Vanish_Fleet_Movement_Box_()`.
4. int16 at 0x199ecb (`_mouse_xy`+4) = -1.
5. `moveable_box[b].content = _box_off_flag[moveable_box[b].id]`.

Ported, the vanishing calls too; oracle zoom2 (`build/oracle/zoom2.sh`, s2
with Sol's box open, then ZOOM in): `TestZoomWithBoxOpen` matches.

`Remove_Box_Check_` (0x919a3) is an empty `ret`.

### Vanish_System_Display_Box_ (0x91abc)

`moveable_box[0].content = _box_off_flag[0]`; `_planet_buttons[0..4]`
(0x1992d4) = -1000; the 8 int16 at 0x1931c0 + 12·i (`_system_display_ships`
+4) = -1000; box 0 `drag_field` (0x192fea) and `field2` (0x192fec) = -1000.

### Vanish_Fleet_Movement_Box_ (0x91a40)

`moveable_box[2].content` (0x193016) = `_box_off_flag[2]` (0x1992c4); for i
< int16 at 0x199bb8 (`_fleet_box`+0xc, the icon count): int16 at 0x197fc1 +
9·i (field id inside `_fleet_movement_icon[i]`) = -1000;
`_fleet_movement_all_button` (0x199982) = -1000; `_fleet_box_colonize_button`
(0x1999ce) = -1000; box 2 `drag_field`/`field2` (0x193022/0x193024) = -1000;
int16 0x199ec7 (`_mouse_xy`) = -1; `_eta_star_id` (0x19995c) = -1;
`_fleet_ship_scanned` (0x1999ca) = -1; `Clear_Fleet_Selection_Status_Flags_`
(0x702e5); `Set_Main_Screen_Help_List_`; `_reloaded_ship_palette` (0x199f38)
= 0.

## Restore_Boxes_ (0x9191d)

For b in 0..4, if `_saved_boxes[b].content != -1000`:

1. `moveable_box[b].content = _saved_boxes[b].content`.
2. If `was_open`: b == 0 → `Init_System_Display_(moveable_box[0].content)`
   (0xa3fe6); b == 2 → if `Get_Stack_Start_From_Saved_Ship_Array_()`
   (0x780d3) returns 0, `Vanish_Fleet_Movement_Box_()`.
3. `was_open = 0`; `content = -1000`.

After a load every slot is -1000 (`Harold_Defaults_`), so it does nothing.
Other caller: `Begin_Of_Turn_` (0x8a91c).

## Engine primitives

### Update_Glass_Remap_Colors (0x133237)

`glass_colors` (0x1ba318) is 16 x `{percent, red, green, blue}` (uint8),
loaded by `Load_Font_File_` (0x120526) from the first 0x40 bytes of
`FONTS.LBX` entry 0: slot 0 {25,0,0,0}, 1 {50,0,0,0}, 2 {10,0,0,0},
3 {50,64,64,0}, 4–15 all 0. `Set_Glass_Remap_Color_(n, r, g, b, percent)`
(0x131acb) changes a slot; only `Tactical_Combat_` calls it (7 times).
`glass_remap_colors` (0x1ba358) is `uint8[16][256]`; `current_palette`
(0x1bb358) is 256 x `[changed, r, g, b]`, 6-bit.

1. `glass_colors[0] = {50, 0, 0, 0}` (overwrites the file's 25 %).
2. `Sort_Palette_List()`.
3. For i in 0..255, for j in 0..15 (p = `glass_colors[j].percent`):
   - p == 0: `remap[j][i] = i`;
   - p == 100: `remap[j][i] = Find_Closest_Color(gc.r, gc.g, gc.b)`;
   - else per channel `v = (pal[i].c * (100 - p) + gc.c * p) / 100`
     (unsigned bytes, integer division, truncated to a byte);
     `remap[j][i] = Find_Closest_Color(r, g, b)`.

Slot 0 is therefore "halfway to black": `(c * 50) / 100` per channel.

### Sort_Palette_List (0x13372a)

Builds `palette_reds` (uint8[256], 0x1bb758) and `palette_list` (ptr[65],
0x1bb914):

1. Insertion sort of colours 0..254 (`count` from 0 while `count != 0xff`)
   by red, stable: for each `count`, `index` = first `i < count` with
   `pal[count].r < pal[reds[i]].r`, else `count`; shift `reds[i+1] =
   reds[i]` for i = count down to index, then `reds[index] = count`. The
   shift starts at `reds[count+1] = reds[count]`, copying a stale byte
   (from the previous call, or 0 from BSS); at count 254 this can leave
   `reds[255]` = the old `reds[254]`. Colour 255 is never inserted.
   Persist the array between calls to reproduce this.
2. `palette_list[0] = &reds[0]`. For r in 1..63: `palette_list[r]` =
   `&reds[j]` for the first j in 0..255 (includes the stale `reds[255]`)
   with `pal[reds[j]].r == r`; if none, `palette_list[r] =
   palette_list[r-1]` (**the start** of the previous group, not the end).
3. `palette_list[64] = &reds[255]`.

`Init_Drivers_` sorts once at start-up, right after `Init_Video_Drivers_`
(0x113d71), while `current_palette` is still all black: every group is
empty and `palette_list[64]` points past them, so until the next sort
`Find_Closest_Color` finds a colour only once `maxR` reaches 0x40, among
all colours whose green and blue lie in the window. The port's
`engine.New` does the same; without it the list stayed zero and
`Create_Brighten_Bitmap_Palettes_` filled its tables with colour 0 when
nothing had sorted before a battle (tc12n's shield hits).

### Find_Closest_Color(r, g, b) (0x1338c9)

```
minR=maxR=r; minG=maxG=g; minB=maxB=b
for count = 1..25:
    minR = minR < 11 ? 0 : minR-10;  maxR = maxR < 0x35 ? maxR+10 : 0x40
    minG = minG < 11 ? 0 : minG-10;  maxG = maxG < 0x35 ? maxG+10 : 0x3f
    minB = minB < 11 ? 0 : minB-10;  maxB = maxB < 0x35 ? maxB+10 : 0x3f
    best = 13000; color = -1
    n = (int16)(palette_list[maxR] - palette_list[minR])
    for i = 0 .. n-1:
        c = palette_list[minR][i]
        if minG <= pal[c].g <= maxG && minB <= pal[c].b <= maxB:
            d = (pal[c].r-r)^2 + (pal[c].g-g)^2 + (pal[c].b-b)^2
            if d < best && c != 0: best = d; color = c
    if color != -1: return color
return 0
```

The ranges widen cumulatively each round. Red is filtered only through the
`palette_list` slice (with the "missing red" quirk above); ties keep the
first in sorted order; colour 0 is never returned except as the fallback.

### Screen_Bitmap_Capture(x1, y1, x2, y2, bmp) (0x12e148)

If `x2 < x1` swap x1/x2 **and** y1/y2 (y is not swapped on its own).
`w = x2 - x1 + 1`, `h = y2 - y1 + 1`. `Create_Bitmap_Header(w, h, bmp)`
(0x12d943): width +0, height +2, current_frame 0, total_frames 1,
repeat_frame / multi_frame_full_store / group_count 0, mask 0x01 (kind 1
bitmap), `frame_offsets[0] = 0x10`. Then `Capture_Screen_Block_` (0x14791b):
copy `h` rows of `w` bytes from `current_video_page + y1*640 + x1` to
`bmp + 0x10`. No clipping, no scan-list marking.

`Get_Bitmap_Size(w, h)` (0x12daa4) = `((w*h + 0x13) / 4) * 4`.
`Allocate_Next_Block` rounds up to 4 and adds a 12-byte header;
`Push_Block`/`Pop_Block` save/restore the block's fill mark (stack of 40).

### _Darken_Fill_(x1, y1, x2, y2) (0x90006)

If `clipping_flag == 1`: `x1 = max(x1, wx1)`, `y1 = max(y1, wy1)`, `x2 =
min(x2, wx2)`, `y2 = min(y2, wy2)`. Then `Tint_Fill(x1, y1, x2, y2, 0)`.

### Tint_Fill(x1, y1, x2, y2, color) (0x12991d)

1. Swap x1/x2 if `x2 < x1`; swap y1/y2 if `y2 < y1` (independently).
2. If `clipping_flag == 1` clamp to the window as above.
3. If `screen_refresh_flag >= 2`: `Add_Square_To_Scan_List(x1, y1, x2, y2)`.
4. `c1 = x1/4`, `c2 = x2/4` (C division), `h = y2 - y1 + 1`. Every pixel
   written is `p = table[p]` (`Tint_Fill_Column_` 0x147723 /
   `Tint_Fill_Region_` 0x147760, in place on `current_video_page`).
   - `c1 == c2`: table `glass_remap_colors[color]`, columns x1..x2.
   - otherwise table `glass_remap_colors[0]` **whatever `color` is**;
     `center = c2 - c1 - 1`:
     - `x1 & 3 != 0`: columns x1 .. 4·c1+3, then `center` groups of 4 from
       4·c1+4;
     - `x1 & 3 == 0`: if `center != 0`, `center + 1` groups from 4·c1;
       if `center == 0` **nothing** (the first group is skipped);
     - then if `x2 & 3 != 0`: columns 4·c2 .. 4·c2 + (x2&3) - 1.
     So in the multi-group case **column x2 is never tinted**. (A
     `Tint_Fill_Region_` with a count of 0 would loop 2^32 times; the
     unaligned branch calls it unconditionally, so `center == 0` with an
     unaligned x1 must not occur.)

For the zoom call (0x16, 0x16, 0x20f, 0x1a5, clipping off, refresh full):
columns 0x16..0x20e and rows 0x16..0x1a5 become `glass_remap_colors[0][p]`;
column 0x20f stays bright. Not marked in the scan list (refresh 0).

### Glassed drawing

Not used by zoom mode: the captured bitmap has mask 0x01 (no 0x08), so
`Draw` takes the plain kind-1 path.

## Port notes

- `mainscreen_loop.go`: the zoom-in case (line ~198) logs "not ported";
  zoom-out-in-zoom-mode, the whole-screen field branch and the zoom variant
  of `addMapFields` are missing; `zoomSteps` exists. The right-click exit is
  ported.
- The twinkle reset loops `range s.Stars` over `[maxStars]` = 72 entries,
  which matches the binary's fixed 0x48.
- New engine pieces: `Update_Glass_Remap_Colors` with `Sort_Palette_List`
  and `Find_Closest_Color` (persistent `palette_reds`), `Screen_Bitmap_Capture`
  producing a kind-1 picture usable by the clipped `Draw`, `Tint_Fill` with
  its column quirks, `_Darken_Fill_`, `Draw_Multi_Colored_Box_`, mouse list
  `_mouse_list_zoom_mode` (cursor image 6).

## Open questions

1. Whether `Draw_Mini_Main_Screen_` at zoom entry can set `_skip_fields`
   (via `_FULL_DRAW_SCREEN_FLAG`), which would change the end-of-pass field
   rebuild; not traced.
2. Cursor image 6's appearance is not checked (FONTS.LBX palette record
   cursors, `fields-and-input.md`).
