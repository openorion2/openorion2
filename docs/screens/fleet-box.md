# Main screen: ship icon clicks, the fleet movement box, move orders

The fleet movement box is moveable box 2 of the galaxy screen. It opens when
a ship icon is clicked, lists the stack's ships, lets the player select
them, shows travel info for the star under the mouse, and on a click on a
star gives the move order. The move-order routines (`SHIPMOVE.C`) change
the save state and are shared with the AI, so they are specified to the
instruction.

Sources: `FLEETPOP.C` (mod 43, 0x6fd24..0x72690), `MAINSCR.C` (mod 52:
`Do_Fleet_Popup_`, the `Main_Screen_` click handling, scan routines),
`HACCESS.C` (mod 47: selection accessors), `MOVEBOX.C` (mod 56),
`SHIPMOVE.C` (mod 119, 0xff496..0x100a00), `AIMOVE.C`/`MAP.C` geometry
(0xeb4dd..0xebeb7). Addresses are ELF addresses in `build/orion2.elf`
(object-1 offsets + 0x10000, data + 0x178000). Background: stacks, icons,
`_ship_node`, `_ship_icon`, `Draw_ETA_Destination_Line_` and the selection
helpers are in `main-screen-ships.md`; boxes in general in
`main-screen-panel.md`; hit tests are ported in `internal/game/hittest.go`.

All arithmetic is 16-bit signed unless stated; `/` is C truncation. Marks:
**?** inferred, **bug** behaviour of the binary that looks unintended but
must be ported.

## Data

Box 2 is `_moveable_box[2]` at 0x193014 (record layout `save-ships.md`):

| Address | Field | Use here |
|---|---|---|
| 0x193016 | `content` | 1 = open (`Do_Fleet_Popup_`); `Vanish_Fleet_Movement_Box_` writes `_box_off_flag[2]` (0x1992c4) |
| 0x193018 / 0x19301a | `height` / `width` | written by `Set_Fleet_Box_Data_Dimensions_` |
| 0x19301c / 0x19301e | `x` / `y` | written by `Set_Popup_Draw_XY_`, clamped by `Set_Fleet_Box_Data_Dimensions_` |
| 0x193020 | `dragging` | `Add_Moveable_Box_Fields_` passes `dragging == 0` as "add buttons" |
| 0x193022 | `drag_field` | hidden field over the name bar |
| 0x193024 | `field2` | the CLOSE button (Esc) |
| 0x193026 | `field3` | whole-box hidden field (`Add_Whole_Window_Field_`) |
| 0x193028 / 0x19302a | closed w/h | 200 / 0x27 (`Init_Fleet_Movement_Box_` 0x7971f, new game only: w 200, h 0x110, x = y = 0x16) |
| 0x19302c / 0x19302e | `+0x18/+0x1a` | drag offset written by `Move_Moveable_Box_` (0 when idle, zeroed at 0x91727); added to every draw position |

`_fleet_box` 0x199bac (game globals, not saved):

| Address | Off | Meaning |
|---|---|---|
| 0x199bac | +0 | x (copy of box.x) |
| 0x199bae | +2 | y (copy of box.y) |
| 0x199bb0 | +4 | full height incl. extra buttons |
| 0x199bb2 | +6 | width = width of the name-bar picture (217 / 198) |
| 0x199bb4 | +8 | grid height (169 large; `_fleet_box_map_height` small) |
| 0x199bb8 | +0xc | `n`: ships in the stack (`N_Fleet_Icons_`) |
| 0x199bba | +0xe | base height (`Fleet_Box_Height_(n)`) |
| 0x199bbc | +0x10 | y offset of the ALL button = name.h + grid.h + 4 |
| 0x199bbe | +0x12 | name-bar picture (BUFFER0 0x33 / 0x34) |
| 0x199bc2 | +0x16 | bottom picture (0x37 / 0x38) |
| 0x199bc6 | +0x1a | small: 1 if `n < 10` |
| 0x199bc7 | +0x1b | 0 = recompute the extra buttons (`_first_time` clears it); never set to 1 in the traced code **?** |
| 0x199bc8..bcc | +0x1c.. | button flags: attack Antares, engage, colonize, transport, outpost |

Other globals:

| Global | Address | Meaning |
|---|---|---|
| `_fleet_movement_icon` | 0x197fbc | 500 × 9 bytes: `+0 u8` unused, `+1 i16 x`, `+3 i16 y`, `+5 i16 field`, `+7 i16 node` (memset 0x1194 to 0 on rebuild) |
| `_fleet_movement_box_ptr` | 0x1996ac | i16[9]: visible slot → icon index |
| `_first_fleet_movement_icon` | 0x1999fc | first listed icon = scroll first row × 3 |
| `_fleet_icon_buttons_added` | 0x1999fe | visible icons, min(n, 9) |
| `_fleet_box_ship_stack` | 0x1999b8 | stack index shown |
| `_first_node_ptr_in_stack` | 0x199a06 | head node the list was built for (-1 forces a rebuild) |
| `_fleet_box_ship_ids` | 0x199e44 | 63-byte bit field of the box's ship ids (`Build_Saved_Ship_Array_`) |
| `_fleet_icon_selection_status` | 0x199e05 | 63-byte bit field, saved selection |
| `_fltpop_scroll_bar` | 0x199ed0 | scroll bar record, below |
| `_fleet_movement_bar_field` | 0x199a00 | scroll field |
| `_fleet_movement_all_button` | 0x199982 | ALL button field |
| `_fleet_box_scroll_up/dn_button` | 0x19994e / 0x199950 | fields |
| `_fleet_box_outpost/colonize/attack/transport/engage_button` | 0x1999cc / ce / d4 / d6 / da | fields, -1000 when absent |
| `_fleet_movement_icons_on` | 0x199f08 | last `Set_Fleet_Movement_Icon_Selections_` mode |
| `_fleet_icon_chosen` | 0x19c186 | visible slot last left/right clicked, -1 |
| `_fleet_ship_scanned` | 0x1999ca | icon index under the mouse (box), -1 |
| `_eta_star_id` | 0x19995c | star under the mouse while ships are selected |
| `_g_ship_move_info` | 0x1992a8 | 14 bytes, result of `Ships_Try_To_Move_To_` |
| `_first_time` | 0x199f00 | set by `Do_Fleet_Popup_`, cleared at the end of `Draw_Fleet_Movement_Box_` |
| `_just_moved_stack` | 0x199f33 | 1 after a whole stack was ordered; suppresses auto-select |
| `_draw_expanding_box` | 0x199f23 | play the expanding animation on the next draw |
| `_add_help_fields` | 0x199f1b | 0 → `Draw_Fleet_Movement_Box_` calls `Set_Main_Screen_Help_List_` once |
| `_mouse_btn_pressed` | 0x19c19e | painting selection in progress |
| `_fltpop_field_selection_status` | 0x19c199 | value painted (0/1) |
| `_icon_overlay_x/y` | 0x1999f2 / 0x1999f4 | last clicked icon position (overlap cycling) |
| `_set_fade_for_first_fltpop` | 0x19bfc4 | first popup after a fade |
| `_mainscr_palette` | 0x199f1d | 2 after `Reload_Fleet_Box_Palette_` |
| `_reloaded_ship_palette` | | 1 after `Load_Player_Ship_Palette_`, 0 in Vanish |
| settings+4 / +5 | 0x199be0 / 0x199be1 | Expanding Help → `_draw_expanding_box`; Auto Select Ships |

Scroll bar record (`_fltpop_scroll_bar`, i16 unless noted): `+0 pos` (bar
centre), `+2 last` (-1 = reset), `+4 start`, `+6 bar height`, `+8 rows`,
`+0xa visible rows` (3), `+0xc first row`, `+0xe track` (0x66), `+0x18 u8`
(2), `+0x19 u8` press counter, `+0x1a u8 step` (1), `+0x1b i16` saved pos.

## Pictures (BUFFER0.LBX, verified by rendering)

| Entry | Size | Picture | Global (load order in `Load_Main_Screen_Pictures_` 0x13368..) |
|---|---|---|---|
| 0x0b | 54×53 | blue selection glow behind a selected ship | `_fleet_box_selection_seg` 0x19340c |
| 0x33 / 0x34 | 217×35 / 198×35 | name bar large / small | via `Buffer_Reload` |
| 0x35 / 0x36 | 217×169 / 198×169 | 3×3 ship grid; large has the scroll track and arrows | via `Buffer_Reload` |
| 0x37 / 0x38 | 217×37 / 198×37 | bottom with info panel and an ALL label | via `Buffer_Reload` |
| 0x39 | 15×106 | scroll bar thumb | `_fleet_box_scroll_bar_seg` |
| 0x3a / 0x3b | 14×23 / 14×25, 2 frames | scroll up / down | 0x193294 / 0x1932a4 |
| 0x3c | 48×21, 2 frames | ALL button | 0x19328c |
| 0x3d..0x42 | 215×32 (0x42: 214), 2 frames | large: ATTACK ANTARES, COLONIZE PLANET, ENGAGE ENEMY FORCES, UNLOAD TROOPS (transport), outpost, CLOSE | 0x193284, 0x1932a0, 0x1932c0, 0x193290, 0x1932b4, 0x193278 |
| 0x43..0x48 | 195×32, 2 frames | small: attack Antares, engage, colonize, transport, outpost, close | 0x1932c4, 0x19329c, 0x1932b8, 0x193280, 0x193288, 0x1931b8 |

Frame 0 is the normal look, frame 1 lit (orange text). The large set is used
when `n ≥ 10` (box with scroll bar), the small set when `n < 10`.

SHIPS.LBX: entry `picture + 0x32*b` is a ship picture (52×48), `b` =
banner (player+0x26) for builder < 8, else 8 (`Get_Ship_Picture_Segk_`
0x5869b, called by `Get_Ship_Id_Picture_Seg_` 0x58678 with design `builder`
+0x5d and `picture` +0x5c). Entry `0x31 + 0x32*banner` is a 2×1 dummy
carrying a palette chunk for colours 192..239; entries 0x19d..0x1a3 carry
192..255 for non-players (below).

## Opening the box

### Clicks on the map (Main_Screen_ 0x865d5..0x86b67)

Each loop, after `Scan_Input` (`scan`) and `Get_Input` (`r`):

1. If `r == ±_main_screen_grid_field` (0x1999c8; the galaxy map box has its own `_galaxy_map_grid_field` at 0x19997a): `kind = (r ≤ 0) + 1` (1
   left, 2 right), `(px, py)` = pointer. Box 2 open: `Check_Stars_XY_`,
   and only if no star was hit `Check_Ships_XY_`; else ships first, then
   stars. Otherwise both click results are -1. (Ported, `hittest.go`.)
2. If `!_skip_fields`: `res = Get_Mouse_Field_(0, &scanStar, &clickStar)`;
   `iconSel = (res is 0 or 4) && box 2 open && Some_Icon_Selected_()`.
   - `res == 0 && clickStar ≥ 0`: relocation / system popup / colony
     screen when `!iconSel` (not this spec).
   - `iconSel && clickStar > -1 && res == 0 && star not a black hole`:
     **move order**, below.
   - `iconSel && res == 4` (hovering a star) and box 2 open:
     `_fleet_ship_scanned = -1`; `_eta_star_id = -1` if the star is a black
     hole, else `scanStar`.
3. Box 0 handling may call `Do_Fleet_Popup_(Ship_Icon_Id_From_Node_Ptr_(node),
   0)` for a ship clicked in the system display (0x86ab1).
4. `Scan_Moveable_Box_Fields_(_skip_fields, r, scan, &leave)` (below).
5. If `!_skip_fields`: `res = Get_Mouse_Field_(1, &scanIcon, &clickIcon)`;
   `_main_screen_ship_icon_scanned = scanIcon` (≠ -1 clears `_eta_star_id`);
   `res == 0` (left click on an icon) → `Do_Fleet_Popup_(clickIcon, 1)`.
   A right click on an icon (res 1) does nothing here.
6. F1/F2 (`Check_Function_Keys_`) → `Cycle_Ship_Icons_` then
   `Do_Fleet_Popup_(i, 0)`.

End of every loop pass (0x87573), box 2 open: if `_mouse_btn_pressed` and
no button is held: `_mouse_btn_pressed = 0`, `Set_Input_Delay(2)`. Then
`Save_Ship_Selection_Status_()`, and `_eta_star_id = -1` unless
`Some_Icon_Selected_()`.

Main_Screen_ entry (0x86288): unless 0x199f3a == 1, if box 2 is open and
`Get_Stack_Start_From_Saved_Ship_Array_()` fails → `Vanish_Fleet_Movement_Box_`;
then `_ship_icon_player_id = -1`. After `Build_Ship_Icons_`:
`_clear_fleet_fields` → `_first_node_ptr_in_stack = -1`, else
`Some_Icon_Selected_()` → `Reload_Ship_Selection_Status_()`. On leaving
(0x875cc): `_mainscr_palette = 0x65`, `_ship_icon_player_id = -1`,
`_first_fleet_movement_icon = 0`, `_clear_fleet_fields` →
`_first_node_ptr_in_stack = -1`, `_eta_star_id = -1`.

### Get_Mouse_Field_ (0x857f8)

`(mode, &scanned, &clicked) → res`; both outputs -1 first. Click globals:
ship `_mouse_xy` 0x199ec7, star 0x199ecb; hover: ship 0x199ec9, star
0x199ecd; kind 0x199ecf. Box 2 closed: clicked ship (mode 1) or clicked star
(mode 0) → `*clicked`, return `kind != 1` (0 left, 1 right); else hovered
ship (mode 1) or hovered star (mode 0) → `*scanned`, return 4; else -1. Box
2 open: the same with star tested before ship. (Ported.)

### Do_Fleet_Popup_ (0x831b1)

`(icon, overlap)`:

1. `z = Map_Scale_To_Zoom_Level_()`; `old = -1`;
   `Clear_System_Display_Loaded_Flag_`; `_ship_icon_player_id =
   _fleet_ship_scanned = _eta_star_id = -1`;
   `Clear_Fleet_Box_Selected_Ship_Status_` (63 bytes of
   `_fleet_icon_selection_status` = 0); `Clear_Ship_Node_Selection_Flags_`
   (`selected = 0` in all 500 nodes, `_fleet_movement_icons_on = 0`);
   `_first_time = 1`; `_just_moved_stack = 0`.
2. `overlap` → `Overlapped_Ship_Icon_Button_(&icon, 13 - z, 9 - z)`; 1 →
   `_draw_main_screen_to_back = 1`.
3. Box 0 open: `Vanish_System_Display_Box_`, `Disable_Draw_All_Fields`,
   `Set_Page_Off`, `Draw_Mini_Main_Screen_`, flip, `Enable_Draw_All_Fields`,
   `_first_node_ptr_in_stack = -1`. Else box 2 open: `old =
   Ship_Stack_Owner_(start[_fleet_box_ship_stack])`.
4. `box2.content = 1`; `_fleet_box_ship_stack =
   Ship_Stack_From_Ship_Icon_(icon)` (first stack s with `start[s] ==
   icon.node`, -1 if none or no stacks). -1 → `Vanish_Fleet_Movement_Box_`,
   return.
5. `Set_Popup_Draw_XY_(2, icon, &box2.x, &box2.y)`. If settings+4
   (Expanding Help): `_draw_expanding_box = 1`.
6. `owner = Ship_Stack_Owner_(start[stack])`. If `owner != old ||
   _mainscr_palette != 2`: if `_set_fade_for_first_fltpop`:
   `Set_Fade_Percent_(0)`, clear it, `Draw_Mini_Main_Screen_`; else
   `Disable_Draw_All_Fields`, `Vanish_Fleet_Movement_Box_`,
   `Redraw_Main_Screen_(0)` (`main-screen-background.md`),
   `Enable_Draw_All_Fields`. Then `box2.content = 1`,
   `Reload_Fleet_Box_Palette_(owner, owner != PLAYER_NUM)`.
7. `Move_Box_To_Top_(2)`; scroll bar `+0x34` (0x199a04) = 0x66;
   `_add_help_fields = 0`.
8. If `stack > -1 && box.x ≤ 0x1f && box.y ≤ 0x1f`: `(ix, iy) =
   Get_Ship_Icon_Coords_(icon)`; if `Occluded_(box.x, box.y, box.x+box.w,
   box.y+box.h, ix, iy, ix+10, ix+13)` (**bug**: last value uses `ix`, not
   `iy`) → `box.x = 0x26b`, `box.y = 0x1a5` (clamped later by
   `Set_Fleet_Box_Data_Dimensions_`).

`Occluded_(x1,y1,x2,y2, X1,Y1,X2,Y2)` (0x8faf6, all i16): x overlaps if
`X1 ≤ x1 ≤ X2`, or `X1 ≤ x2 ≤ X2`, or `x1 ≤ X1 && x2 ≥ X1`, or `x1 ≤ X2 &&
x2 ≥ X2`; the same for y with y1, y2 against Y1, Y2; returns both.

`Set_Popup_Draw_XY_(2, icon)` (0x856f7): `x = icon.x - 0x15`, `y = icon.y -
0x15` (cached icon coords), then `Popup_XY_(2, x, y)` (0x8574e), `w` =
box2.width as it stands:

| Condition | box.x | box.y |
|---|---|---|
| `x < 0xfd && y < 200` | `0x20f - w` | `0x1a5` |
| `x < 0xfd && y > 200` | `0x20f - w` | `0x16` |
| `x > 0xfd && y < 200` | `0x16` | `0x1a5` |
| otherwise (incl. `x == 0xfd` or `y == 200`) | `0x16` | `0x16` |

(For box 0 the 0x1a5 cases are `0x1a5 - box.height`.) The box opens in the
quadrant opposite the icon; 0x1a5 is pulled up by the later clamp.

### Overlapped_Ship_Icon_Button_ (0xa0fa8)

`(&icon, dx, dy)`. If `|_icon_overlay_x - icon.x| > dx` or
`|_icon_overlay_y - icon.y| > dy`: store `icon.x/y` in `_icon_overlay_x/y`,
return 0. Otherwise (a second click near the same place): collect icons j
= 0.. with `|x_j - x_icon| < dx && |y_j - y_icon| < dy` (strict; the
clicked icon included) into `l[]`, stopping after 50; if any: for k = 0 ..
len-2 swap the 12-byte `_ship_icon[l[k]]` with `_ship_icon[l[k+1]]` and
`_ship_stack_start[l[k]]` with `_ship_stack_start[l[k+1]]` (**bug**: icon
indices used as stack indices), then `*icon = l[0]`, return 1. Repeated
clicks thus cycle through overlapping stacks. `Ship_Icon_Star_Id_` is called
and ignored; `Set_Memory_(l, 100)` fills the list first.

## Drawing

`Draw_Moveable_Box_(2)` → `Draw_Fleet_Movement_Box_` (0x70875), inside the
map window (0x16,0x16,0x20f,0x1a5) with clipping on.

### Draw_Fleet_Movement_Box_ (0x70875)

`dx, dy` = box2 +0x18/+0x1a; `head = Node_Ptr_From_Ship_Stack_(stack)`.

1. If `head != _first_node_ptr_in_stack` (new stack): `n =
   N_Fleet_Icons_(head)`; `Fleet_Box_Data_(n)`; scroll bar: `+0xa = 3`,
   `+0xe = 0x66`, `+8 = max((n+2)/3, 1)` (a quotient < 2 gives 1),
   `+0xc = 0`, `+6 = Get_Vertical_Scroll_Bar_Height_(bar)`, `+2 = -1`,
   `+0x18 = 2`, `+0x1a = 1`, `+0 = 0x66 - (+6)/2`, `_scroll_bar = &bar`;
   `Clear_Fleet_Box_Selected_Ship_Status_()`; `rebuilt = 1`.
2. `owner = Ship_Stack_Owner_(head)`; `_ship_icon_player_id = owner`.
   `Buffer_Reload_Fleetpop_Frame_` (sets `+0x16`/`+0x12` pictures by the
   small flag). `X = _fleet_box.x + dx`, `Y = _fleet_box.y + dy`.
3. Unless `_draw_expanding_box`: small → `Draw_Small_Fleet_Box_Frame_`,
   else `Draw_Large_Fleet_Box_Frame_`; `Draw_Fleet_Pop_Added_Buttons_(X,
   Y)`; title: owner < 9 → `_mox_font_colors_offset = 0x71`,
   `Get_Mox_Font_Colors_(4, 0x6f, 0x73, c)`, `sprintf(H 0x66 "%s Fleet",
   Race_Name_(owner))`, `Set_Font_Style_Outline(4, c)`,
   `Set_Outline_Color(2, 1)`, `Set_Font_Spacing(1)` (not reset
   afterwards); owner ≥ 9 → `Get_Mox_Font_Colors_(4, 0x4b, 0x4e, c)`, the
   head ship's name upper-cased, style 4 outline, outline colour (2, 1), no
   spacing call. `Print_Centered(X + width/2, Y + 0xe, title)`.
4. If `_first_node_ptr_in_stack != head || == -1`: memset
   `_fleet_movement_icon` (0x1194 bytes) to 0, `_fleet_icon_buttons_added =
   0`. `_first_fleet_movement_icon = bar.first × 3`.
5. If `_first_node_ptr_in_stack != head`: walk `k = 0, node = head`: each
   step: done if `node == -1 || k ≥ 500`; `added = min(added+1, 9)`; if `k
   < 500`: `icon[k] = {field -1000, node}`, `node = node.next`, `k++`, done
   when `node == -1`.
6. If `_first_time && settings+5 (Auto Select Ships) && owner == PLAYER_NUM`:
   `Set_Fleet_Movement_Icon_Selections_(1, 1)`.
7. If `rebuilt`: `Build_Saved_Ship_Array_()` (63 bytes 0, then set bit
   `node[icon[k].node].ship` for k < n, only if `_ship_stack_count > 0`).
8. Ships, unless `_draw_expanding_box`: for k = first .. min(first+added,
   n)-1, slot s = 0..: `_fleet_movement_box_ptr[s] = k`; `x = (s%3)*0x3a +
   0xe + X`, `y = (s/3)*0x38 + 0x26 + Y`, stored in `icon[k].x/y`; `oy =
   0` if the ship's hull size (+0x10) is 5 else 2. If the icon's node is
   selected: `Draw(x + small, y, BUFFER0 11)`. `Draw(x + 2, y + oy,
   Get_Ship_Id_Picture_Seg_(ship))`. If `owner == PLAYER_NUM` and the ship
   is a combat ship (`design.type == 0`): `Draw_Damage_Bars_(x-1, y,
   0x35, 0x35, 4, ship)`.
9. `Load_Player_Ship_Palette_(ship[node[head].ship].owner)` — every draw.
   `_first_node_ptr_in_stack = head`.
10. Unless `_draw_expanding_box`: `Print_Fleet_Box_Info_()`; if `n > 9`:
    `Draw_Generic_Vertical_Scroll_Bar_(&bar, Draw_Fltpop_Scroll_Bar_)`.
11. `_first_time = 0`. If `_draw_expanding_box`: `i =
    Ship_Icon_Id_From_Node_Ptr_(head)`, clear the flag,
    `Draw_Exploding_Plasma_Box_(icon[i].x + 3, icon[i].y + 3, box.x, box.y,
    box.w, box.h, 2)`, then `Draw_Fleet_Movement_Box_()` again.
12. `_add_help_fields == 0` → `_add_help_fields = -1`,
    `Set_Main_Screen_Help_List_()`.

`N_Fleet_Icons_(node)` (0x71465): count of the chain, starting at 1,
stopping at 500 (`Ship_Owner_` is called per node only for its abort check).

### Box geometry

`Fleet_Box_Data_(n)` (0x71a37): if `_first_time`: scroll first-ship
0x199f20 = 0, `+0x1b = 0`. Small (`n < 10`): flag 1, pictures 0x38/0x34,
`Set_Fleet_Movement_Box_Dimensions_(n)`, `x/y` = box.x/y, `width` =
name.w, `+4 = Fleet_Box_Height_(n)`, `+8 = _fleet_box_map_height`. Large:
flag 0, pictures 0x37/0x33, `x/y`, `width`, `+4 = Fleet_Box_Height_(n)`,
`+8 = height(0x35)` (169). Both: `+0x10 = name.h + (+8) + 4`, `+0xe = +4`,
`+0xc = n`; `+0x1b == 0` → `Set_Added_Button_Stuff_()`.
`Reload_Fleet_Box_Data_` (0x71a17, from `Load_Game_`): if
`Get_Stack_Start_From_Saved_Ship_Array_()` the same with `n` of the found
stack, else the body of `Vanish_Fleet_Movement_Box_`.

- `rows(n) = min((n-1)/3 + 1, 3)`; `Fleet_Box_Grid_Height_(r)` (0x71198):
  1 → 0x38, 2 → 0x70, 3 → height of 0x36 (169), else 0.
- `Set_Fleet_Movement_Box_Dimensions_(n)` (0x7226a):
  `_fleet_box_map_height = Grid_Height(rows(n))`, `box.h =
  Fleet_Box_Height_(n)`.
- `Fleet_Box_Height_(n)` (0x711e6) = name.h (35) + (n < 10 ?
  Grid_Height(rows(n)) : 169) + bottom.h (37) + height of the large CLOSE
  picture (32). Name/bottom by the small flag at call time. So 160, 216,
  273 for 1, 2, 3 rows; 273 with a scroll bar.
- `Set_Added_Button_Stuff_` (0x71c01): `_n_added_buttons = 1`; then in this
  order colonize, transport, outpost, engage (each `Set_*_Flag_(n)`) and
  attack Antares; each flag set adds 1 and the height of its **large**
  picture (32) to `h`. `+4 = h + (+0xe)`; `Set_Fleet_Box_Data_Dimensions_`.
- `Set_Fleet_Box_Data_Dimensions_` (0x721eb): `box.w = +6`, `box.h = +4`;
  if `+6 + x > 0x20f`: `box.x = x = 0x20f - +6`; if `+4 + y > 0x1a5`: `box.y
  = y = 0x1a5 - +4`.

`Draw_Large_Fleet_Box_Frame_(x, y, dx, dy, n, &outY)` (0x704a6): `X = x+dx`,
`Y = y+dy`; save window; `Set_Window(X, Y, X + width, min(Y + base, 0x1a5))`,
clip on, `Fill(X+6, Y+0x29, X+width-6, Y + base + 0x1d, 0)`; restore window.
Draw 0x33 at (X, Y), 0x35 below it, 0x37 below that (each `+= height`).

`Draw_Small_Fleet_Box_Frame_` (0x70602): the same fill; draw 0x34 at (X, Y);
`gh = Grid_Height(rows(n))`; save window, `Set_Window(X, Y', X + width,
min(Y' + gh, 0x1a5))` (`> 0x1a4` → 0x1a5) with `Y'` the grid top, clip on,
draw 0x36 (cropped to `gh` rows), restore; draw 0x38 at `Y' + gh`.

`Draw_Fleet_Pop_Added_Buttons_(X, Y)` (0x70f02): `h` = height of the large
CLOSE picture (32); set = small if `n < 10` else large; `bx = X + 2`, `by =
Y + base - h`. CLOSE at `(bx, by)`; then, each only if its flag is set and
each `h` lower than the previous: outpost, colonize, engage, transport,
attack Antares. Every picture gets `Reset_Animation_Frame` first.

`Draw_Fltpop_Scroll_Bar_` (0x7078b, verified in assembly): `X =
_fleet_box.x + dx + 0xc0`, `Yb = _fleet_box.y + dy`, `y1 = Yb + 0x43 +
(track - (bar.h/2 + bar.pos))`, `y2 = y1 + bar.h`; for i = 0..8:
`Line(X+i, y1, X+i, y2, c)`, `c` starts at 0x74 and is decremented after
every even i (0x74, 0x73, 0x73, 0x72, 0x72, 0x71, 0x71, 0x70, 0x70). Then
`Draw(_fleet_box.x + dx + 0xbd, Yb + 0x42, BUFFER0 0x39)` (the track
picture, drawn over the thumb lines).

`Draw_Damage_Bars_(x, y, w, h, off, ship)` (0x702ea): `L = x + off`, `B = y
+ h`; `max = Max_Ship_Hits_`, `cur = Current_Ship_Hits_`; `g = (w-4)*cur /
max` (int), `g = 2` if `g < 3`; `r = (w-4) - g`. Only if `cur != max`: if `r
== 1`: `g = w-6`, `r = 2`. If `g > 0`: lines (L,B-5)-(L+g,B-5) and
(L,B-5)-(L,B-1) colour 0x70, (L+g,B-4)-(L+g,B-1) and (L+g,B-1)-(L+1,B-1)
colour 0x72, `Fill(L+1, B-4, L+g-1, B-2, 0x71)`. If `r > 1`: `a = L+g+1`,
`e = a + r`: (a,B-5)-(e,B-5), (a,B-5)-(a,B-1) colour 0x4b; (e,B-4)-(e,B-1),
(e,B-1)-(L+g+2,B-1) colour 0x4e; `Fill(L+g+2, B-4, e-1, B-2, 0x4d)`. (Hit
points: `Max_Ship_Hits_` 0x5f2f6 = structure + armour hits, or hull table
+ best armour % when 0x199cb4; `Current_` subtracts +0x7d and, unless
0x199cb4, +0x7b.)

### Palette: Load_Player_Ship_Palette_ (0x585e0)

`owner < 8`: SHIPS.LBX entry `banner*0x32 + 0x31`; 8 → 0x19d, 9 → 0x1a3,
10 → 0x1a1, 11 → 0x1a0, 12 → 0x19e, 13 → 0x19f, 14 → 0x1a2; other owners
skip the load and pass a stale register (unreachable). `Buffer_Reload`
(cache), `Draw_Palette(picture)` (writes the chunk with change flags:
players 192..239, monsters 192..255), `Refresh_Palette()`,
`_reloaded_ship_palette = 1`. `Reload_Fleet_Box_Palette_(owner, x)`
(0x71b0e) = that plus `_mainscr_palette = 2` (second argument unused).

### Print_Fleet_Box_Info_ (0x71506)

`s0 = Fleet_Movement_Ship_Id_(0)` (first listed ship), `sel =
Some_Icon_Selected_()`, `case = 9` (nothing). Position: `x = _fleet_box.x
+ dx + 0x48 + (small ? 0x38 : 0x42)`; `n < 10`: `limit = 0x70`, `y =
name.h + (+8) + 10`; else `limit = 0x84`, `y = 0xd6`; `y += box.y + dy`.
`hl = 1`.

- `_eta_star_id == -1 && _fleet_ship_scanned != -1` → case 0.
- `_eta_star_id == -1 && !sel`: if `_fleet_ship_scanned == -1 &&
  _fleet_icon_chosen != -1` → `_fleet_ship_scanned = _fleet_icon_chosen`
  (a slot number stored as an icon index, **bug**; prints nothing this
  frame). If still -1: `s0.status == 0` → case 1 with `s0.location`; else
  `Get_Absolute_Location_Scan_Info_(s0)`.
- `_eta_star_id == -1 && sel`: as the line above without the chosen step.
- `_eta_star_id ≥ 0 && sel`, from `_g_ship_move_info` (`m`): `m.can_move`:
  `m.eta == 0` → case 1 with `_eta_star_id`; visited(PLAYER_NUM, eta star)
  → case 2 (`m.eta`, eta star) else case 3 (`m.eta`). Not movable:
  `m.blocked` → 4; `m.no_drive` → 5; `m.out_of_range` → visited ? 6
  (`m.short`, eta star) : 7 (`m.short`); `m.flux` → 8; else 9.

`Get_Absolute_Location_Scan_Info_(ship)` (0x71315): `d =
Absolute_Location_(location)`; `d != NUM_STARS && !Player_Has_Visited_(PLAYER_NUM,
d)` → case 3 with `eta` (+0x6d); else case 2 with (`eta`, `d`).

| Case | Text (HESTRNGS) | Colour |
|---|---|---|
| 0 | `Get_Fleetpop_Scanned_Ship_Info_`: ship of icon `_fleet_ship_scanned`; not ours, owner < 9 and combat → `"%s"` (0x1792b7) of the hull name pointer at 0x18003a + size·0x24; else the ship's name | hl |
| 1 | 0x67 `Orbiting %s` with the star name | hl |
| 2 | star == NUM_STARS → 0x68 `Destination: Antares`; else 0x69 `%d turn to %s` (value 1) / 0x6a `%d turns to %s` | hl |
| 3 | 0x6b `ETA %d turn` / 0x6c `ETA %d turns` | hl |
| 4 | 0x6d `Black Hole in-between` | error |
| 5 | 0x6e `Immobile Fleet` | error |
| 6 | 0x6f `%d parsec to %s` (1) / 0x70 `%d parsecs to %s` | error |
| 7 | value 0 → 0x71 `black hole blocks range`; else 0xeb `%d parsecs` | error |
| 8 | 0x73 `hyperspace flux` | error |

If the text is not empty: `Set_Fitted_Font_Style_(text, c, 3, 2, 0, limit,
1)`; hl → `_mox_font_colors_offset = 0x71`, `Get_Mox_Font_Colors_(style,
0x70, 0x73, c)`; error → `(style, 0x4b, 0x50, c)`; `Set_Font_Colors_(c)`;
style 1 → `y += 2`; width > limit → `Print_Paragraph_Centered_(x -
limit/2, y - 6, limit, 0x14, text)`, else `Print_Centered(x, y, text)`.

### Draw_Fleet_Box_Selected_Ship_Seg_ (0x843b3)

`i = Ship_Icon_Id_From_Node_Ptr_(Node_Ptr_From_Ship_Stack_(stack))`;
`Draw(icon[i].x, icon[i].y, Get_Ship_Icon_Pict_Seg_(Ship_Icon_Owner_(i),
1))` (the coords call in between is discarded). Drawn by `Draw_Main_Screen_`
in its first box pass (`main-screen-ships.md`).

### Draw_Exploding_Plasma_Box_ (0x83efd)

Expanding Help animation, `(fx, fy, x, y, w, h, id)`; id 2 →
`Fleet_Box_Data_(N_Fleet_Icons_(head))`, `w = +6`, `h = +4`. Saves the
random seed first and restores it at the end, so the game RNG is not
affected. Clips (x, y, w, h) to the window, redraws the map
(`Draw_Main_Screen_Filled_(1)`, `Draw_Main_Main_Screen_`,
`Copy_Off_To_Back`), then 10 frames of `Plasma_Box` growing from (fx, fy)
toward the box (`k*w/10`, `k*h/10`, direction by quadrant, colours
0xe,0x10,..,0x1c, `Random(8)-1` start, `Random(10)` phase), a final full
box, flip, `_draw_main_screen_to_back = 1`. Only reached with Expanding
Help on (default off); detail of `Plasma_Box` not decoded.

## Fields

`Add_Moveable_Box_Fields_` calls `Add_Fleet_Movement_Box_Fields_(node,
dragging == 0)` (0x6ff08) when box 2 is open:

1. Return unless `Node_Ptr_From_Ship_Stack_(stack) ==
   _first_node_ptr_in_stack` (the box was drawn for this stack).
2. `Fleet_Box_Data_(N_Fleet_Icons_(head))`; `bx, by` = box.x/y (no drag
   offset). `box.field3 = -1000`; `box.drag_field =
   Add_Hidden_Field(bx, by, bx + name.w, by + 0x23, "", 0)`.
3. For visible k: `icon[k].field = Add_Hidden_Field(x, y, x + 0x35, y +
   0x35, "", 0x29)` with the icon's stored x/y.
4. ALL: if `order[0] == 2 && buttons && head ship owner == PLAYER_NUM &&
   Player_Can_Order_Ship_(PLAYER_NUM, head ship)`:
   `Add_Clipped_Button_Field(bx + 0x10 + small, by + (+0x10), "",
   BUFFER0 0x3c, "", 0x28)` (`small` = 1 for the small box, 0x700a4); else
   -1000.
5. `n > 9`: scroll buttons -1000; if `buttons`: up
   `Add_Button_Field(bx + 0xbf, by + 0x2a, "", 0x3a, "-", 0x28)`, down
   `(bx + 0xbf, by + 0xae, "", 0x3b, "+", 0x28)`; copied to 0x199ee0/2;
   then, whatever `buttons`, `_fleet_movement_bar_field =
   Add_Scroll_Field(bx + 0xc0, by + 0x42, 0, 0x66, 0, 0x66, 10, 0x66,
   &bar.pos, "", 0)` (0x701b8): a vertical engine scroll field over the
   track whose value is `bar.pos` (engine side in
   `docs/engine/fields-and-input.md`). With `n ≤ 9` the bar field keeps the
   -1000 that `Clear_Moveable_Box_Fields_` wrote.
6. All five extra button fields = -1000; `Add_Added_Buttons_(bx, by)`:
   same layout as `Draw_Fleet_Pop_Added_Buttons_` (no drag offset), each
   `Add_Clipped_Button_Field(bx+2, y, "", picture, hotkey, 0x28)`: CLOSE →
   `box.field2` with hotkey Esc (0x1792a4 = 0x1b), the others with "".
7. `Add_Whole_Window_Field_(2)`: `box.field3 = Add_Hidden_Field(box.x,
   box.y, box.x + width(+6), box.y + base(+0xe), "", 0)`.

`Clear_Moveable_Box_Fields_(2)` (0x908da; `Add_Moveable_Box_Fields_` calls
it for every box before adding fields): drag field, field2 and field3,
the up/down buttons and the bar field = -1000; if `box.content >
_box_off_flag[2]` also `icon[k].field` for k < n.

`Box_Field_Selected_(2, r)` (0x90605): `r` is the drag field, field3, the
bar field or the ALL button (no -1000 test; `r` is never -1000).

### Scan_Moveable_Box_Fields_, box 2 part (0x89183)

For each open box i in 0..4 (`r` = input, `scan` = Scan_Input):

- `r == box.field2 && !skip` → box 2: `Clear_Fleet_Box_Selected_Ship_Status_`,
  `Clear_Ship_Node_Selection_Flags_`, `_draw_main_screen_to_back = 1`,
  `Vanish_Fleet_Movement_Box_`.
- else: `!skip && order[0] != i && Box_Field_Selected_(i, r)` (drag field,
  field3, for box 2 also the bar field and ALL) → `Move_Box_To_Top_`. Then
  for box id 2 (box 2 open):
  - `r == up` → `Scroll_Up_One_Item_(&bar)`; `r == down` →
    `Scroll_Down_One_Item_`.
  - `r == ALL`: if `_fleet_movement_icons_on == 1 || Fleet_Buttons_Chosen_()
    == n` → `Set_Fleet_Movement_Icon_Selections_(0, 0)`,
    `Clear_Fleet_Box_Selected_Ship_Status_`, `_eta_star_id = -1`; else
    `Set_Fleet_Movement_Icon_Selections_(1, 0)`.
  - otherwise `res = Scan_Fleet_Movement_Box_Fields_(scan, r, &hov,
    &_fleet_icon_chosen)`; `_fleet_ship_scanned = hov` (≥ 0 also clears
    `_eta_star_id`). `res == 1` (right click) → `Deactivate_Help_List`,
    `Detailed_View_Ship_(Fleet_Movement_Ship_Id_(chosen + first))`,
    `Set_Main_Screen_Help_List_`, `_draw_main_screen_to_back = 1`. Then
    the buttons (each only if its flag is set):
    - colonize → `_eta_star_id = -1`, screen 0x1e (the screens:
      `fleet-box-actions.md`);
    - transport → if `At_Least_One_Ship_Of_Type_Selected_(2)` screen 0x22,
      else `User_Box_(&_draw_main_screen_to_back, H 0x20, 0)` (string 0x20
      is "Option disabled during multi-player games" **?**);
    - outpost → screen 0x1f;
    - engage → `s = Ship_Stack_Star_Id_(start[stack])` (0x78b93: the head ship's
      `location`); `0 ≤ s <
      NUM_STARS` → `star[s].ignore_combat (+0x36) &= ~(1 << PLAYER_NUM)`;
    - attack Antares (0x8956d, checked in assembly; `r > 0`):
      `!Some_Icon_Selected_()` → `User_Box_(&_draw_main_screen_to_back
      (0x199f28), H 0xe6, 3)` (a warning box). Else `k =
      Get_Fleet_Box_Selected_Ship_Ids_(buf, 500, 0xff)` (the count), then
      `s0 = Fleet_Movement_Ship_Id_(0)`; `sprintf(buf, H(k == 1 ? 0xe4 :
      0xe5), k)`; `User_Box_(.., buf, 1)` (confirmation); a zero byte
      result → done. Yes → `m = Deploy_Ships_To_Attack_Antares_()`,
      `_first_node_ptr_in_stack = -1`, `Update_Star_Owners_`,
      `Build_Ship_Icons_`, `Set_Ship_Icon_XYs_`,
      `Set_Fleet_Movement_Icon_Selections_(0, 0)`,
      `Clear_Fleet_Box_Selected_Ship_Status_`; if `m == n` (16-bit
      compare): `stack = Ship_Stack_From_Ship_Id_(s0)`; -1 → Vanish
      (never: that routine returns 0 when not found). With `m < n` the
      stack index is kept although the stacks were rebuilt.

`Deploy_Ships_To_Attack_Antares_` (0xa1afa): a 500-entry i16 list filled
with -1; walk the chain from `start[_fleet_box_ship_stack]` and append
`node.ship` for every node with `selected` (+4) set (no owner test). Then
`Ships_Try_To_Move_To_(list, NUM_STARS, &info)` and `Make_Ships_Move_To_(list,
NUM_STARS, &info)` (the order is applied whatever `can_move` says; with a
dimensional portal at the fleet's star `eta` = 1, speed -1). Returns the
count. The list cannot be empty behind `Some_Icon_Selected_` (the port
returns 0 instead of reading `ship[-1]`).
    Screens set `_return_screen = 0` and `leave = 1`.

`Scan_Fleet_Movement_Box_Fields_(scan, r, &hov, &chosen)` (0x88fdd):
`chosen = hov = -1`. For visible k (slot s): only if
`Can_Select_Fleet_Box_Ship_(k)` (owner of `icon[k].node`'s stack ==
PLAYER_NUM): `r == icon[k].field` → `chosen = s`, `hov = k`, return 0;
`r == -icon[k].field` → `chosen = s`, return 1. If nothing matched: first
visible k with `scan == icon[k].field` → `hov = k`, return 4; else -1.

### Selecting ships (painting)

Selection is changed only here, in `Draw_Main_Screen_` (0x8475x), before the
box passes: if `Mouse_Button() == 1`, box 2 open,
`Can_Select_Fleet_Box_Ship_(0)` and `Mouse_Within_Fleet_Box_()` (0x91b0b:
`box.x ≤ px ≤ box.x + box.w`, `box.y ≤ py ≤ min(box.y + box.h, 0x1a5)`):
`f = Scan_Input()`; if `f != box.drag_field`: when `!_mouse_btn_pressed`,
`_fltpop_field_selection_status = !Icon_Selection_Status_(f)`;
`Set_Painted_Fleet_Fields_(f)`. Then `_mouse_btn_pressed = 1`. So the first
icon pressed toggles and dragging over others paints the same value.

- `Icon_Selection_Status_(f)` (0x7252f): `selected` of the first visible
  icon whose field is `f`, else 0.
- `Set_Painted_Fleet_Fields_(f)` (0x8a0b7): for visible k (slot s) with
  `icon[k].field == f`: node of `icon[_fleet_movement_box_ptr[s]]`; if
  `Player_Can_Order_Ship_(PLAYER_NUM, ship)` → `node.selected = status`.
- `Set_Fleet_Movement_Icon_Selections_(mode, auto)` (0x7229e): for k < n
  with ship owner == PLAYER_NUM: mode 0 → `selected = 0`; mode 1 and auto
  → `selected = 1` if `Ship_Should_Be_Auto_Selected_` (`Ship_Can_Be_Selected_
  && !_just_moved_stack`), else unchanged; mode 1, no auto → `selected =
  1` if `Ship_Can_Be_Selected_`. `_fleet_movement_icons_on = mode`.
- `Ship_Can_Be_Selected_(s)` (0x8a16a) = `Player_Can_Order_Ship_(PLAYER_NUM,
  ship s)` (0x7843d) = `ship.owner == p && Can_Order_Ship_(ship)`.
- `Fleet_Buttons_Chosen_` (0x712e1): selected icons among k < n.
  `First_Unselected_Icon_` (0x7109b): first k < n not selected, else 0.
- `Save_Ship_Selection_Status_` (0x71b91): if the box's stack owner is
  PLAYER_NUM, for k < n with node ≠ -1: set or clear bit `ship` of
  `_fleet_icon_selection_status` from `selected`. `Reload_` is in
  `main-screen-ships.md`.

#### Can_Order_Ship_ (0xff6be)

`(ship)`: owner ≥ `NUM_PLAYERS` → 0. Status 0 → 1. Status 2 → 1. Status
≥ 3 → 0. Status 1 (in transit): player app 91 (+0x172 == 3, tech
application 91, presumably Hyperspace Communications **?**) → 1; else
needs app 180 (+0x1cb, Tachyon Communications) or app 176 (+0x1c7, Sub-Space
Communications) known (== 3), else 0; then scanning stars `NUM_STARS-1`
down to 0: `d = Parsecs_Between_Points_(star, ship)`; 1 if `d ≤ 5` and the
star's `base_subspace` (+0x3c) has the owner bit, or `d ≤ 2` and
`base_tachyon` (+0x3b) has it; else 0.

## Move orders

### Main_Screen_ click on a star (0x867bf)

Condition above (`iconSel`, left click on a non-black-hole star `t`):

1. `Get_Fleet_Box_Selected_Ship_Ids_(buf, 500, 0xff)` (buf of 500 i16,
   -1 filled, `main-screen-ships.md`), `Ships_Try_To_Move_To_(buf, t,
   &_g_ship_move_info)`.
2. Not `can_move` (+5): message by the first set of +0 → H 0x21 "A Black
   Hole blocks movement", +1 → 0x22 "…not have enough fuel…" (+1 is never
   written: **unreachable**, an out-of-range order is refused silently),
   +2 → 0x23 "One or more ships chosen is immobile", +0xd → 0x24 "Ship
   movement is not allowed during a hyperspace flux";
   `User_Box_(&_draw_main_screen_to_back, msg, 3)`.
3. Else: `c = Fleet_Buttons_Chosen_()`; `Make_Ships_Move_To_(buf, t,
   &info)`; `_eta_star_id = -1`; `c != n` → `f = First_Unselected_Icon_()`,
   else `_just_moved_stack = 1`. `Build_Ship_Icons_`, `Set_Ship_Icon_XYs_`;
   `_fleet_box_ship_stack = Ship_Stack_From_Ship_Id_(
   Fleet_Movement_Ship_Id_(c != n ? f : 0))` — the box list still holds
   the old node numbers, now read against the rebuilt `_ship_node`
   (**bug**: may name another ship; port as is). `Ship_Stack_From_Ship_Id_`
   (0x78b0e) returns 0 when not found. `Set_Fleet_Movement_Icon_Selections_(0,
   0)`, `Clear_Fleet_Box_Selected_Ship_Status_`; stack -1 → Vanish (never);
   `_draw_main_screen_to_back = 1`; `_first_node_ptr_in_stack = -1`.

Hover: `Scan_Stars_XY_` (part of `Check_Mouse_XY_Fields_`, every
`Draw_Main_Screen_`) calls `Get_Fleet_Box_Selected_Ship_Ids_` and
`Ships_Try_To_Move_To_(buf, star, &_g_ship_move_info)` whenever the mouse
is on a star and `Some_Icon_Selected_()`, black holes included; that fills
the info used by the ETA line and `Print_Fleet_Box_Info_`.

### Ships_Try_To_Move_To_ (0xff799)

`(list, dest, info)`; `list` is i16 ship ids ending with -1 (callers never
pass an empty list). `D = &star[dest]` (dest may be `NUM_STARS`: Antares,
a record past the last star). All locals are int unless noted.

```
owner  = (i8) ship[list[0]].owner
range  = 30000
speed  = owner < 8 ? (u8) player[owner].ftl_speed (+0x5a0) : 1
s      = -1                         // +8 result
memset(info, 0, 14); nav = 0
if dest != NUM_STARS && owner < 8 && player[owner].trans_dimensional (+0x8bc) == 0
   && Event_Check_Hyperspace_Flux_():   // byte 0x19abf6 in {2,4,6}
    info.flux (+0xd) = 1; return
first = true
for each id in list until -1:
    r = (i16) Ship_Range_(&ship[id])
    o = ship[id].officer (+0x74)
    if o != -1 && leader[o].eta (+0x37) == 0:
        v = 0
        if leader[o].special_skills byte +0x2b & 2:  v = Owned_Officer_Level_(o)/3 + 1
        elif                         .. & 1:     v = Owned_Officer_Level_(o)/4 + 1
        nav = max(nav, (i16) v)
    if first: (x, y) = (ship.x, ship.y); first = false
    range = min(range, r)               // (i16) compare
here = XY_To_Star_Id_(x, y)             // scans 0..NUM_STARS inclusive
if dest == NUM_STARS && here != -1 && star[here].dimensional_portal (+0x40) bit owner:
    info.eta = 1; goto gate
if owner < 8 && !Star_In_Range_Of_Player_(D, owner, range):
    c = Closest_Other_Colony_Star_(owner, dest)
    if c != -1: info.short (+9) = (u8) Parsecs_Between_Stars_(c, dest)
    info.out_of_range (+6) = 1
if owner < 8:
    if here < 0:
        if Black_Hole_Blocks_Points_(x, y, D.x, D.y): info.blocked = 1
    else:
        H = &star[here]
        if (i16)(i8) H.wormhole_to (+0x29) == dest: info.wormhole (+0xa) = 1
        if H.stargate (+0x3d) bit owner && D.stargate bit owner: info.stargate (+0xc) = 1
        if H.jumpgate (+0x3e) bit owner && D.jumpgate bit owner: info.jumpgate (+0xb) = 1
        if Test_Bit_Field_(H.black_hole_blocked (+0x1f), dest) && nav == 0
           && !info.wormhole && !info.stargate && !info.jumpgate: info.blocked = 1
if info.stargate || info.wormhole:
    info.eta = (dest == here) ? 0 : 1
elif speed <= 0:
    info.no_drive (+2) = 1
else:
    (cx, cy) = (x, y)
    s = speed + nav; if info.jumpgate: s += 3
    turns = 0
    while !((i16)cx == D.x && (i16)cy == D.y):
        turns++
        Move_Player_1_Turn_To_Star_(owner, cx, cy, dest, (i16) s, &cx, &cy,
                                    nav == 0 ? &neb : NULL, &stop)
    info.eta = (u8) turns
info.navigator (+7) = nav > 0
if (info.blocked && !info.jumpgate && !info.stargate && nav <= 0)   // tested as !(nav > 0)
   || info.out_of_range
   || (speed == 0 && (D.x != x || D.y != y)):
    info.eta = -1; info.can_move (+5) = 0; return
if !info.stargate && !info.wormhole:
    info.can_move = 1; info.speed (+8) = (u8) s; return
gate: info.speed = -1; info.can_move = 1
```

Notes, all as in the binary:
- The nebula flag pointer is NULL with a navigator, so navigators ignore
  nebulas; `neb`/`stop` results are otherwise unused here.
- `blocked` from a star needs `nav == 0`; from deep space (`here < 0`) it
  ignores the navigator. With `speed == 0` the fleet can still "move" to
  the star it is on (eta 0).
- `dest == NUM_STARS` without a portal falls through and reads the record
  past the last star.
- `info` bytes +1 and +3 are never written.

`Ship_Range_(ship)` (0xff496): owner ≥ 8 → 0xff; else `r =
player[owner].fuel_range (+0x324, i16)`; if design special bit 11 (`specials`
+0x17, damage not considered): `r = (r*3 + 1)/2`. Returns `r`.

`Owned_Officer_Level_(o)` (0x94951): `Get_Officer_Base_Level_(leader.experience
(+0x24), o == 0x42 ? 0 : player[leader.owner (+0x3a)].+0x8bd)`, capped at 5.
Navigator skill bits 9 (byte +0x2b & 2) and 8 (& 1) of `special_skills`.

### Range: Star_In_Range_Of_Player_ (0xff5f8)

`(D, p, range)`: `Aux2(D, p, range)`; else for `q = NUM_PLAYERS-1` down to
0, `q != p` and `player[p].treaty[q] (+0x627+q) == 2` (alliance):
`Aux2(D, q, range)`. Any hit → 1.

`Aux2(D, p, range)` (0xff593): `D.colony_owners (+0x38)` bit p → 1;
`Aux1(D, p, range)` → 1; `W = D.wormhole_to`; `W != -1 &&
star[W].visited (+0x33)` bit p && `Aux1(&star[W], p, range)` → 1; else 0.

`Aux1(S, p, range)` (0xff4e9): `j = (S - star)/0x71`; `lim = range*900*range`
(int); for k = `NUM_STARS-1` down to 0 with `star[k].colony_owners` bit p,
skipping k whose `black_hole_blocked` row has bit j: `dx = S.x - star[k].x`,
`dy = S.y - star[k].y`; `dx*dx + dy*dy ≤ lim` → 1. Else 0.

`Star_In_Normal_Range_Of_Player_(S, p)` (0xff666) passes `player.fuel_range`,
`..Extended..` (0xff68a) `(fuel_range*3)/2` (truncated); not used here.

`Closest_Other_Colony_Star_(p, dest)` (0xe6bcd): `best = 0xffffffff`
(unsigned), `res = -1`; colonies c from `NUM_COLONIES-1` down to 0 with
`colony.owner (+0) == p`: `t = planet[colony.planet (+2)].star (u8 +2)`;
skip if `t == dest` or `t == res`; skip if `star[dest].black_hole_blocked`
bit t; `d = dy² + dx²` (to `star[dest]`, unsigned); `d < best` → `best = d`,
`res = t`.

`Parsecs_Between_Points_(x1, y1, x2, y2)` (0xebe79): `d = dx² + dy²`
(unsigned); `r = isqrt(d / 900)`; `while r*r*900 < d: r++`; returns `r`.
`Parsecs_Between_Stars_(a, b)` (0xebeb7) applies it to the two stars.

`isqrt_(n)` (0x134c92, unsigned): `n ≥ 0xfffe0001` → 0xffff; else binary
search `lo, hi` = `(0, 0x100)` if `n ≤ 0xffff` else `(0x100, 0xffff)`;
repeat `mid = (lo+hi)/2`; `mid*mid ≤ n` (unsigned) → `lo = mid` else `hi =
mid`; until `lo + 1 ≥ hi`; returns `lo` (= floor √n).

### Travel: Move_Player_1_Turn_To_Star_ (0xebb0c)

`(p, x0, y0, dest, speed, &x, &y, neb, &stop)` — one turn of movement:

```
(tx, ty) = (star[dest].x, star[dest].y)
dx = tx - x0; dy = ty - y0; d2 = dx*dx + dy*dy; L = isqrt_(d2)
if (i16) speed == 0: speed = 1
arrived = false; inNeb = false; *stop = 0
for k = 1; ; k++:                         // k compared as i16
    if (neb && inNeb) || *stop || arrived || k > speed: break
    if k*900*k >= d2: *x = tx; *y = ty; arrived = true
    else:
        *x = dx < 0 ? x0 - (k*30*(-dx) + L - 1)/L : x0 + (k*30*dx + L - 1)/L
        *y = dy < 0 ? y0 - (k*30*(-dy) + L - 1)/L : y0 + (k*30*dy + L - 1)/L
    if neb && Point_Is_In_Some_Nebula_(*x, *y): inNeb = true
    elif Point_Is_In_Warp_Field_Interdictor_Of_Star_(*x, *y, p, &star[dest]): *stop = 1
if inNeb: *neb = 1
```

Each step is 30 map units (one parsec) along the line from the turn's start,
rounded away from the start (ceil of the magnitude); positions are stored as
i16. Entering a nebula ends the turn (without a navigator), so a fleet in a
nebula moves one parsec per turn; an enemy Warp Interdictor at the
destination ends the turn inside 90 units.

`Point_Is_In_Warp_Field_Interdictor_Of_Star_(x, y, p, S)` (0xebac3):
`S.warp_interdictor (+0x39) & ~(1 << p)` non-zero and `(x - S.x)² + (y -
S.y)² ≤ 0x1fa4` (8100).

`Point_Is_In_Some_Nebula_(x, y)` (0xeba3a): any n < `NUM_NEBULAS` (u8
0x199f0d), in order, with `Point_Is_In_Nebula_N_(x, y, n)` (0xeb9c8): `u =
(x - neb[n].x)/3`, `v = (y - neb[n].y)/3` (truncation: -1, -2 give 0); `u
< 0 || v < 0` → 0; `B = _nebula_pict_seg[n][3]` (0x1902a4 + 16n, the zoom
level 3 **bitmap**, `+0 i16 w`, `+2 i16 h`, pixels at +0x10); `u ≥ w || v ≥
h` → 0; inside if `B[0x10 + v*w + u] > 5` (unsigned).

### Black holes

`Black_Hole_Blocks_Points_(x1, y1, x2, y2)` (0xeb7fd): equal points → 0;
`Collect_Black_Holes_` (0xeb7c5: stars with spectral class 6, ascending);
from the last to the first: `Point_Within_D_Of_Seg_(bh.x, bh.y, 45, x1,
y1, x2, y2)` → 1. Else 0.

`Point_Within_D_Of_Seg_(px, py, D, x1, y1, x2, y2)` (0xeb531; all inputs
i16, sign-extended; arithmetic 32-bit):

```
loop:
    A = y1 - y2; B = x2 - x1
    s  = A*px + B*py - (A*x2 + B*y2)       // signed
    as = |s| (unsigned); L2 = A*A + B*B; D2 = D*D
    if 0xffffffff / D2 >= L2 (unsigned):
        within = as <= 0xffff && as*as <= D2*L2      (unsigned products)
        break
    if as <= 0xffff: within = true; break
    r = isqrt_(L2)
    if as <= D*r: within = true; break
    if (r+1)*D < as: within = false; break
    px, py, D, x1, y1, x2, y2 = each /2 (truncation toward 0); continue
if !within: return 0
if s == 0:                                   // on the line, i16 compares
    if x1 < x2:  return x1 <= px <= x2
    if x2 < x1:  return x2 <= px <= x1
    if y1 <= y2: return y1 <= py <= y2
    return y2 <= py <= y1
(nx, ny) = (A, B); while nx*nx + ny*ny <= D2 (unsigned): nx += A; ny += B
(qx, qy) = (px + nx, py + ny)
if PSS(qx, qy, px, py, x1, y1, x2, y2) > 0: (qx, qy) = (px - nx, py - ny)   // second PSS result ignored
return PSS(x1, y1, x2, y2, qx, qy, px, py) <= 0
```

`PSS(ax, ay, bx, by, l1x, l1y, l2x, l2y)` = `Points_On_Same_Side_Of_Line_`
(0xeb4dd): `c1 = (l2x-l1x)*(ay-l1y) - (l2y-l1y)*(ax-l1x)`; `c1 == 0` → 0;
`c2 = (l2x-l1x)*(by-l1y) - (l2y-l1y)*(bx-l1x)`; returns `c1 > 0 ? c2 : -c2`.
The last test is "the segment's ends are not strictly on one side of the
perpendicular through the point". With map sizes up to 1518×1200 the
overflow branch can be taken for long segments (`L2 > 2 121 012`).
`Initialize_Black_Hole_Blocks_` (0xeb87d) uses the same routine for the
`black_hole_blocked` rows.

### Make_Ships_Move_To_ (0xffd08)

`(list, dest, info)`, applied without checking `can_move`:

```
if info.eta == 0: loc = dest; st = 0
elif (i16) ship[list[0]].location < 0x48 || >= 1000: loc = dest + 1000; st = 2
else: loc = dest + 500; st = 1           // already in transit
col = false
for each id in list until -1:
    if !col && (design.type == 1 || == 4): col = true
    ship.location (+0x65) = loc
    ship.eta (+0x6d) = info.eta
    ship.navigator (+0x6b) = info.navigator
    ship.speed (+0x6c) = info.speed       // patch stub 0x16915c
    if ship.status != 5: ship.status = st  // patch stub
if col: Clear_All_Ships_Auto_Colonization_Flag_(list)
```

`Clear_All_Ships_Auto_Colonization_Flag_` (0xa1c0d): for listed colony
(type 1) and outpost (type 4) ships: weapon slot 0 `weapon` (+0x1c) = -1,
slot 1 `weapon` (+0x24) = 0 (the slots double as colonisation data in these
designs **?**). `Set_Nonplayer_Ship_Destination_` (0xa1a23) and the AI use the
same pair; `Deploy_Ships_To_Attack_Antares_` (0xa1afa) collects the selected
nodes of the box's chain into a -1-filled list and calls both with
`NUM_STARS`.

`Event_Check_Hyperspace_Flux_` (0x233fa): byte 0x19abf6 (current event
state **?**) is 2, 4 or 6. The Go stub `hyperspaceFlux()` returns false.

## Button availability (Set_*_Flag_, `n` = icons)

Common: stack -1 → 0; `head` = start[stack], `h` = its ship, `loc =
h.location`; `atStar = 0 ≤ loc ≤ 499`; `leaving = 1001 ≤ loc ≤ 1071`. The
chain is walked from `head` for at most `n` nodes while `node ≥ 0`.

- Colonize (0x71f35): `leaving` → `loc -= 1000`; needs owner == PLAYER_NUM,
  `atStar || leaving`, `star[loc].colonize_player (+0x37) == PLAYER_NUM`;
  some colony ship (type 1) and an orbit planet with
  `Planet_Is_Colonizable_` (0x714a1: planet ≥ 0, planet +4 byte == 3,
  and no colony or colony +6 ≠ 0).
- Transport (0x72346): `leaving` → star from `XY_To_Star_Id_(h.x, h.y)`;
  needs owner == PLAYER_NUM and `atStar` (leaving is ignored); a transport
  (type 2) and an orbit planet with a colony owned by PLAYER_NUM.
- Outpost (0x71dd8): as colonize with outpost ships (type 4) and
  `Can_Build_Outpost_` (0x7022d: planet ≥ 0, no colony, and
  `!Planet_Has_Outpost_`).
- Engage (0x7209d): star as transport; owner == PLAYER_NUM and (`atStar ||
  leaving`); a combat ship (type 0, status < 3) of PLAYER_NUM in the chain,
  and `star.ignore_combat (+0x36)` has the player's bit.
- Attack Antares (0x71ce7): `here = XY_To_Star_Id_(h.x, h.y)` ≠ -1; scan
  ships i < `NUM_SHIPS` with status < 3, testing `h` (not i) to be a combat
  ship with `Absolute_Location_(h.location) != NUM_STARS`, and ship i at
  `h`'s position (**bug**: always true for i = h); then
  `star[here].dimensional_portal (+0x40) != 0` (any player).

## Vanish_Fleet_Movement_Box_ (0x91a40)

`box2.content = _box_off_flag[2]`; `icon[k].field = -1000` for k < n; ALL,
colonize, `box.drag_field`, `box.field2` = -1000 (the other button fields
keep their numbers); `_mouse_xy = -1`; `_eta_star_id = -1`;
`_fleet_ship_scanned = -1`; `Clear_Fleet_Selection_Status_Flags_` (63
bytes of `_fleet_icon_selection_status`); `Set_Main_Screen_Help_List_`;
`_reloaded_ship_palette = 0`. Node selection flags and the palette are
untouched. `Check_Fleet_Movement_Box_Status_` (0x8ab43): `stack =
Ship_Stack_From_Saved_Array_()`; -1 → Vanish, 0x199bd5 = 0, 0x199bd3 =
`_box_off_flag[2]`.

`Get_Stack_Start_From_Saved_Ship_Array_` (0x780d3): `stack = -1`; for each
stack s < count (at least one pass), walk its chain; the first node whose
ship has its bit in `_fleet_box_ship_ids` sets `stack = s`; returns `0 ≤
stack < count`. `Ship_Stack_From_Saved_Array_` (0x789d4): for ships i <
500 with status < 3 and bit set, scan stacks: remember the first stack of
the same owner, stop at a stack whose head ship is i; result that stack,
else the first same-owner stack, else -1. `Node_Ptr_From_Ship_Stack_(s ==
count)` inlines this and returns the **stack index** as a node (**bug**).

## Help list (Set_Main_Screen_Help_List_ 0x6f280, assembly)

The help list is the static `_main_screen_help_list` (0x181768, 10-byte
records `id, x1, y1, x2, y2`): 15 fixed entries, then the multiplayer
entries (overwritten in place at index 15/16), then, if box 2 is open
(0x918d5), the fleet box entries from `_fleet_box` (no drag offset), all
16-bit: `W` = width (+6), `h` = height of the large CLOSE picture (32),
`A` = base (+0xe) + y (+2) − h (the top of CLOSE), `x1` = x + 2, `x2` = W +
x1.

| Id | Rectangle |
|---|---|
| 0x136 | (x1, A − 0x23) – (x2, A): the info panel |
| 0x130 | (x1, A) – (x2, A + h): CLOSE |
| then, each only if its flag is set, in this order, with `b` starting at h and growing by h after each: 0x135 outpost, 0x132 colonize, 0x133 engage, 0x134 transport, 0x131 attack Antares | (x1, A) – (x2, A + h + b) |

Every button entry starts at the top of CLOSE; `Check_Help_List` takes the
first match, so each button still gets its own id. Then
`Deactivate_Help_List`, `Set_Help_List(list, count)`.

## Scroll bar routines (game side)

The record (`short *b`): `+0 pos`, `+2 last`, `+4 start`, `+6 h`, `+8 rows`,
`+0xa visible`, `+0xc first`, `+0xe track`, `+0x18 u8` (2, unread), `+0x19
u8` press counter, `+0x1a u8 step`, `+0x1b i16` (unaligned) saved pos.
`_scroll_bar` (0x1975c4) points at the bar last set up (the fleet box sets
it in `Draw_Fleet_Movement_Box_`). `pos` is the thumb centre measured up
from the bottom of the track; the engine scroll field writes it while the
track is pressed.

- `Get_Vertical_Scroll_Bar_Height_(b)` (0xa00f1): `+0x1b = +0`; rows <
  visible → track; else `(visible - 1 + track*visible) / rows`.
- `Scroll_Up_One_Item_` (0xa01c6), if visible < rows: `first -= step`; `<
  1` → `first = 0`, `pos = h/2 + track`; else `pos += step * (track /
  rows)`; `last = start = pos`. `Scroll_Down_One_Item_` (0xa015d): `first
  += step`; `> rows - visible` → `first = rows - visible`, `pos = h/2`;
  else `pos -= (track/rows)*step`; `last = start = pos`. (pos grows
  upward: 0x66 is the top.)
- `Draw_Generic_Vertical_Scroll_Bar_(b, fn)` (0xa0226):

  ```
  btn = Mouse_Button(); b.press = 0; b.saved = b.pos; old = b.first
  b.h = rows < visible ? track : (visible - 1 + track*visible) / rows
  if b.last != -1:
      if b.last == b.pos || |b.pos - b.start| <= b.h/2: goto clamp
      b.h = Get_Vertical_Scroll_Bar_Height_(b); Set_..._Maxima_(b)
      b.first = Get_..._First_Item_(b)
  b.last = b.start = b.pos
  clamp: if Set_..._Maxima_(b): b.first = Get_..._First_Item_(b); b.last = b.start = b.pos
  fn(); if btn == 1: b.press++
  return old != b.first
  ```

  So a dragged thumb only changes the first row once it is more than half
  its height away from where the last change left it. The fleet box reads
  `first` before drawing the bar, so the icons follow one frame later.
- `Set_Vertical_Scroll_Bar_Maxima_` (0xa0125): `hh = h/2`; `track < pos +
  hh` → `pos = track - hh`; else `pos - hh < 0` → `pos = hh`; returns
  whether it clamped.
- `Get_Vertical_Scroll_Bar_First_Item_` (0xa0040, assembly): rows ≤ 0 → 0;
  `f = (i16)(((track - (i16)(h/2 + pos))*10) / (i16)((step*track*10)/rows))`;
  if `visible + f > rows` or (`visible < rows` and `h + (i16)(T.track -
  (T.pos + T.h/2)) ≥ track` with `T = _scroll_bar`) → `f = rows - visible`;
  `f < 1` → 0.
- `Handle_Vertical_Scroll_Field_(b)` (0x9fff5): `+0x1b = pos`, `h` by the
  formula, maxima, `first = First_Item`. Not used by the fleet box.

## Engine primitives

| Primitive | In `internal/engine` / Go |
|---|---|
| `Buffer_Reload` | `g.cached` |
| `Draw` (clipped), `Reset_Animation_Frame` | yes (`drawClipped`, `CurrentFrame = 0`) |
| `Set_Window`, `Clip_On/Off`, `Save/Restore_Window_Status_` | yes |
| `Fill`, `Line` | yes |
| `Draw_Palette`, `Refresh_Palette` | `DrawPalette`; refresh at the flip |
| `Add_Hidden_Field`, `Add_Button_Field`, `Scan_Input`, `Mouse_Button`, `Set_Input_Delay`, `Pointer_X/Y` | yes |
| `Add_Clipped_Button_Field` | yes |
| `Add_Scroll_Field` (engine scroll field driving `_fltpop_scroll_bar`) | yes (`AddScrollField`, `docs/engine/fields-and-input.md`) |
| `Set_Font_Style_Outline`, `Set_Outline_Color`, `Set_Font_Spacing`, `Print_Centered`, `Get_String_Width` | yes (`Printer`, `Text`) |
| `Set_Fitted_Font_Style_` | yes (`setFittedFontStyle`) |
| `Get_Mox_Font_Colors_`, `Set_Font_Colors_`, `Print_Paragraph_Centered_`, `Race_Name_`, `String_To_Upper` | **no** |
| `User_Box_` | yes (`userbox.go`) |
| `Detailed_View_Ship_` | `docs/screens/ship-view.md` |
| `Plasma_Box`, `Get/Set_Random_Seed`, `Random` | Plasma_Box **no**; RNG in `internal/random` |
| `isqrt_` | **no** (trivial, above) |
| `Set/Clear/Test_Bit_Field_`, `Swap_Structure_` | trivial, not shared yet |

## Open questions

1. Tech application 91 (player +0x172) is assumed to be Hyperspace
   Communications; the names of the TECHNAME.LBX records were not read.
2. Transport button without a transport selected shows H 0x20; the string
   at that index reads "Option disabled during multi-player games". Check
   the index base against the original.
3. (Solved: `Add_Scroll_Field` and thumb dragging, above and in
   `docs/engine/fields-and-input.md`.)
4. `Plasma_Box` (Expanding Help animation) is not decoded.
5. Byte 0x19abf6 (hyperspace flux event state) and the `+0x1b` "added
   buttons set" flag of `_fleet_box` have no traced writers here.
6. `Draw_Damage_Bars_`, `Print_Fleet_Box_Info_` and the `Set_*_Flag_`
   routines were read from Ghidra pseudocode only; the move-order routines
   (`Ships_Try_To_Move_To_`, `Make_Ships_Move_To_`,
   `Move_Player_1_Turn_To_Star_`, range, black hole and nebula tests,
   `isqrt_`, `Parsecs_Between_Points_`) and the click flow were checked
   against the assembly.

## Port notes (internal/game/moveorders.go)

- `Point_Is_In_Nebula_N_`: the zoom-3 nebula pictures (STARBG.LBX `6 + 4t +
  3`) are kind-1 pictures loaded raw, with frame offsets `0x14, 0x14 + w*h`:
  the pixels start at +0x14, not +0x10. The test reads `B[0x10 + v*w + u]`
  anyway, so it is shifted four pixels, and `(u, v) = (0, 0)` reads the low
  byte of the first frame offset (0x14 > 5: inside). The port reads the raw
  LBX entry to keep this.
- The star array is allocated as exactly 72 × 0x71 bytes (0x10de2), so with
  72 stars `star[NUM_STARS]` (Antares orders, `XY_To_Star_Id_`) lies past
  it; the port uses a zero record there. `Test_Bit_Field_(black_hole_blocked,
  72)` reads byte 9 of the row, i.e. `star.special` (+0x28).
- Player +0x8bc is race pick Trans-Dimensional and +0x8bd pick Warlord
  (`Picks[29]`, `Picks[30]`); `Get_Officer_Base_Level_(xp, warlord)` (0x93d4b)
  is 0/1/2/3/4 at < 60/150/300/500, else 4, and 5 only for xp ≥ 1000 with
  Warlord.
- Verified: recomputing every saved `black_hole_blocked` row with
  `Point_Within_D_Of_Seg_` (D 45, pairs `(i, j < i)`, as
  `Initialize_Black_Hole_Blocks_` does) reproduces all ten saves.
