# Main screen: ship icons, stacks, destination lines

Ship side of the galaxy map (screen 0 / 0x27). Sources: `SHIPS.C` (mod 62:
icon list, coordinates, pictures, lines), `SHIPSTAK.C` (mod 32: stacks and
nodes), `MAINSCR.C` (mod 52: scanned icon, ETA line, between-turn rebuild),
`FLEETPOP.C` (mod 43: selection flags), `HACCESS.C` (mod 47: accessors).
Addresses are ELF addresses in `build/orion2.elf`. The map transform
(`Map_Scale_To_Zoom_Level_` 0x79917, `Get_Scaled_Value_` 0x7926c,
`Get_Star_Draw_Coords_` 0x85b93, `Get_Star_Anim_Dimension_` 0x85b71,
`_cur_map_x/y/scale`) belongs to `main-screen-stars.md`; summarised here only
where the ship code depends on a detail:

- `z = Map_Scale_To_Zoom_Level_()`: scale 10→0, 15→1, 20→2, other→3, each
  only if `_max_zoom_count` allows it, else `_max_zoom_count`. z=0 is the
  closest zoom (biggest pictures).
- `Get_Scaled_Value_(v) = v*10 / _cur_map_scale` (C int division).
- `Get_Star_Draw_Coords_(s) = (Scaled(star.x - cur_map_x) + 0x15,
  Scaled(star.y - cur_map_y) + 0x15)` (the subtraction is done in 16 bits).

All arithmetic below is 16-bit signed unless stated; `/2` is C truncation.
Marks: **?** inferred, **bug** behaviour of the binary that looks unintended
but must be ported.

## Data

| Global | ELF | Type | Meaning |
|---|---|---|---|
| `_ship_node` | 0x1975d4 | node[500], 5 bytes | `+0 i16 ship`, `+2 i16 next` (-1 ends), `+4 u8 selected`. Persistent: rebuilt from 0 each `Find_Ship_Stacks_`, entries ≥ `_next_free_node` keep stale data and are still scanned by `Get_Fleet_Box_Selected_Ship_Ids_` and `Reload_Ship_Selection_Status_` (all 500) |
| `_next_free_node` | 0x1999f6 | i16 | nodes in use |
| `_ship_stack_start` | 0x192248 | i16[] | head node of each stack |
| `_ship_stack_count` | 0x199a02 | i16 | stacks |
| `_ship_icon` | 0x1906c0 | icon[572], 12 bytes | `+0 i16 field` (from `Add_Ship_Icon_Fields_`, which has no callers: stays -1), `+2 i16 node` (head node of its stack), `+4 i16 location` (ship.location of the head ship), `+6 i16 slot` (position class, below), `+8 i16 x`, `+0xa i16 y` (draw position, cached by `Set_Ship_Icon_XYs_`/`Draw_Ship_Icons_`) |
| `_ship_icon_count` | 0x1999f8 | i16 | icons in use |
| `_ship_icon_seg` | 0x18fef8 | ptr[8 banners][4] | player icon pictures, index `[banner][p]` |
| `_hl_ship_icon_seg` | 0x1905f8 | ptr[8][4] | same entries (see Pictures) |
| `_monster_ship_icons` / `_hl_monster_ship_icons` | 0x190598 / 0x190518 | ptr[6][4] | `[owner-9][z]` |
| `_antaran_ship_icons` / `_hl_antaran_ship_icons` | 0x190678 / 0x1906b0 | ptr[4] | `[p]` |
| `_ship_icon_width` / `_ship_icon_height` | 0x192e80 / 0x192ecc | u8[4] | filled by `Get_Ship_Icon_Dimensions_` |
| `_multi_colored_line_start` | 0x19995e | i16 | colour rotation of all dotted lines; 0 in `Harold_Defaults_` (0x7930b); `(v+1) % 8` at the end of every `Draw_Main_Screen_` (0x84930) |
| `_main_screen_ship_icon_scanned` | 0x19c180 | i16 | icon under the mouse; -1 in `Harold_Defaults_`; set each loop of `Main_Screen_` from `Get_Mouse_Field_(1)` (0x86b3f; non -1 also clears `_eta_star_id`) |
| `_fleet_box_ship_stack` | 0x1999b8 | i16 | stack index shown in the fleet movement box (box 2) |
| `_first_node_ptr_in_stack` | 0x199a06 | i16 | first node listed in the fleet box (`Draw_Fleet_Movement_Box_` 0x70e49 writes it); -1 in `Harold_Defaults_`, on `Main_Screen_` entry when `_clear_fleet_fields` (0x19c18f), and by many screens |
| `_ship_icon_player_id` | 0x199966 | i16 | owner of the stack in the fleet box; `Draw_Fleet_Movement_Box_` reloads the box frame when it changes (0x709b1); -1 on `Main_Screen_` entry (0x862ae), `Do_Fleet_Popup_`, `Begin_Of_Turn_` |
| `_eta_star_id` | 0x19995c | i16 | destination star under the mouse while the fleet box is open, -1 none |
| `_fleet_ship_scanned` | 0x1999ca | i16 | ETA line drawn only while -1 |
| `_g_ship_move_info` | 0x1992a8 | 14 bytes | result of `Ships_Try_To_Move_To_` 0xff799 (below) |
| `_fleet_icon_selection_status` | 0x199e05 | bit field by ship id | saved selection |
| `_highlighted_ship_seg` | 0x193414 | ptr | written only by `Load_Highlighted_Ship_Icon_Picture_` 0xa1455, which has no callers |
| `_space_ship_icon_count`, `_next_player_ship_icon` | 0x1999fa, 0x1999b4 | i16 | no references |

`_ship_direction_line_green_colors` 0x182135 = `6e 6f 72 74 74 72 6f 6e`,
`_ship_direction_line_red_colors` 0x18213d = `49 4a 4d 4f 4f 4d 4a 49`.

Icon slot (`+6`): 0 first stack at a star (right of it), 1 departing
(status 2, left of it), 2 and 3 second and further stacks at a star (left
column, under each other; all beyond the second share slot 3), 5 in space.

## Find_Ship_Stacks_ (0x5d41e)

1. `excl[i] = 0` for i < `NUM_SHIPS`; `_ship_stack_count = _next_free_node = 0`.
   `excl[i] = 1` if `ship.status >= 3` (signed byte compare).
2. For i = 0..NUM_SHIPS-1 not excluded: scan stacks j = 0.. while not
   matched; ship i matches stack j if its `location`, `x`, `y` and `owner`
   all equal those of the head ship `node[start[j]].ship`. Match: walk the
   chain to its last node, `last.next = _next_free_node`, new node
   `{ship i, next -1, selected 0}`, `_next_free_node++`. No match: new node
   as head, `start[_ship_stack_count++] = node`.
3. If `player[PLAYER_NUM].omniscient` (+0x8ba) is 0: `Remove_Non_Detected_Ships_`.
4. `Sort_Ships_In_Stack_(start[j])` for every stack.

### Remove_Non_Detected_Ships_ (0x5d953)

`base = 2`; `= 4` if player+0x1cc == 3; `= 6` if player+0x18b == 3; `= 9` if
player+0x1b7 == 3 (checked in that order, last wins; PLAYER_NUM's record).
For each stack s (index loop; a deleted stack re-uses the same index) whose
head ship owner ≠ PLAYER_NUM:

- `best = -300`. Colonies c < NUM_COLONIES owned by PLAYER_NUM: star of the
  colony's planet (`planet[colony+2].star`), `d = (Range_(ship.x, ship.y,
  star.x, star.y) + 29) / 30` with the **head** ship. `d == 0` → `best =
  1000` (unconditional). Else `v = base`, `v = base+2` if colony+0x15e ≠ 0,
  `+4` if colony+0x13e ≠ 0, `+6` if colony+0x15f ≠ 0, `v -= d`, `best =
  max(best, v)`.
- Ships k < NUM_SHIPS owned by PLAYER_NUM with status < 3: `d` likewise
  (head ship to ship k); `d == 0` → `best = 1000`; else `v = base - d`, `+2`
  if `Ship_Has_Special_(k, 5)`, `best = max(best, v)`.
- Walk the chain with `prev = -1`. For each node n, ship = node.ship:
  - owner > 8 and `design.ftl_speed` (+0x14) == 0 (stationary monster):
    delete n if `!Player_Has_Visited_(PLAYER_NUM, ship.location)` (star+0x33
    bit); `prev` unchanged either way. A departing Dragon's location 1000 +
    star reads free DOS memory, zero under DOSBox (main-popups.md, drag2m).
  - otherwise stealth value `v`: owner 8 → -5; owner < 8 (patch code at
    0xf4d5c / 0x169357) → 500 if `player[owner].stealthy_ships` (+0x8bb) == 1
    or design special bit 31, 6 or 23 is set (bytes +0x1a&0x80, +0x17&0x40,
    +0x19&0x80), else as for monsters; monsters (owner > 8, moving) and the
    fall-through: `-design.size` (+0x10) if status == 1, else 0. Then `+500`
    for each of bits 31, 6, 23 set in `damaged_specials` (+0x76). If `v >
    best`: delete n (prev unchanged), else `prev = n`.
  - `Delete_Ship_Node_(s, prev, n)` 0x5dda9: prev == -1 → `start[s] =
    n.next`, else `prev.next = n.next`.
- If `start[s]` became -1: `Delete_Structure_(s, start, 2, count)` (shift
  left), `s--`, `count--`.

`Range_` 0x134637: `dx=|x2-x1|, dy=|y2-y1|`; `dy < dx ? dx + dy/2 : dy + dx/2`.
`Ship_Has_Special_(k, b)`: bit b in specials (+0x17) and not in damaged (+0x76).

### Sort_Ships_In_Stack_ (0x5dde0)

`Build_Ship_Type_Array_` 0x5de49 walks from the head while `next` is in
1..499 (`next ≤ 0` ends the walk), collecting `{ship, key}` for nodes with
`ship ≥ 0`: key = `size*50 + (i8)picture` for type 0, -1 type 1, -2 type 2,
-3 type 4; other types leave key **uninitialised (stack garbage, bug)**. If
n > 1: Watcom `qsort_(arr, n, 4, qsort_ships_by_type_)` with comparator
`b.key - a.key` (descending), then the chain's `node.ship` fields are
rewritten in sorted order (nodes keep their links).

## Build_Ship_Icons_ (0xa0305)

Scratch (released immediately but used): `st[72]` of `{i8 owner, i16 stack,
i16 icon}` all -1 (0x168 bytes), `next_slot[288]` all 2 (u8). `_ship_icon`
all bytes 0xff, `_ship_icon_count = 0`, `sorted = false`. After
`Find_Ship_Stacks_`, for each stack i in order, with `stk = start[i]`, `ship =
node[stk].ship`, `owner = Ship_Stack_Owner_(stk)` (aborts on owner outside
0..15), `xy = XY_To_Star_Id_(ship.x, ship.y)` (first star with equal x,y,
scanning 0..**NUM_STARS inclusive**, -1 none), `loc = ship.location`, `n =
_ship_icon_count`:

1. `ship.status == 1`: icon[n] = `{node stk, loc, slot 5}`, count++, sorted = true.
2. `0 ≤ loc < 500` (at star loc; `sown = (i8)star[loc].owner` +0x14):
   - `st[loc].owner == -1`: `st[loc] = {owner, stk, n}`; icon[n] = `{stk, loc, 0}`; count++.
   - `st[loc].owner ≠ PLAYER_NUM && owner == PLAYER_NUM`: the player's
     stack takes slot 0: `icon[st[loc].icon].slot = next_slot[loc]`,
     `next_slot[loc] = min(next_slot[loc]+1, 3)`, `st[loc] = {PLAYER_NUM, stk,
     n}`, icon[n] = `{stk, loc, 0}`, count++.
   - else: icon[n] = `{stk, loc, next_slot[loc]}`, `next_slot[loc] =
     min(+1, 3)`, count++. (The code tests `st.owner ≠ -1 && ≠ owner && owner
     == sown && st.owner == PLAYER_NUM && owner ≠ st.owner`, but both arms
     are identical.)
3. otherwise (loc < 0 or ≥ 500):
   - `xy == loc - 1000`: searches icons j < count for `slot == 1 && node ==
     stk && location == loc` (cannot match: one icon per stack), so j = -1;
     then writes `st[loc]` (loc ≥ 1000, outside the scratch) and
     `icon[-1].slot = 0` (bytes inside `_hl_antaran_ship_icons`). **bug**:
     no icon is added, the stack is invisible. A port adds nothing.
   - `loc ≥ 1000 && xy > -1`: icon[n] = `{stk, loc, 1}`, count++.
   - `loc == xy`: icon[n] = `{stk, xy, next_slot[xy]}`, `next_slot[loc] =
     min(next_slot[xy]+1, 3)`, count++.
   - else: icon[n] = `{stk, loc, 5}`, count++, sorted = true.

End: if count > 0 and sorted: Watcom `qsort_(_ship_icon, count, 12,
qsort_ship_icon_)`, comparator `b.slot - a.slot` (slot 5 first). The drawing
loops run from count-1 down, so in-space icons are drawn last (on top).

## Get_Ship_Icon_Coords_ (0xa0a5c) and helpers

`(icon) → (x, y)`, the top-left of the picture. `ship = node[icon.node].ship`
(`Ship_Icon_Star_Id_` is called, result unused). `ship.status == 1`:
`Get_Ship_Icon_Coords_In_Space_`. Else `star = XY_To_Star_Id_(ship.x,
ship.y)` and `Get_XYs_For_Orbiting_Ships_(star, ship.owner)`, then by slot:

- 0: `(x0, y0)`; 1: `(x1, y1)`, then `x -= 2` if owner ≥ 10 and
  `_cur_map_scale` is 10 or 15; 2..4: `k = min(slot-2, 2)`, `(xs[k], ys[k])`;
  5: `(Scaled(ship.x), Scaled(ship.y))` **without** the map offset and +0x15
  (**bug**, reachable only for a non-transit stack off every star); other: outputs untouched.

`Get_XYs_For_Orbiting_Ships_` (0xa0bb0): `w = width of
Get_Ship_Icon_Pict_Seg_(owner, 0)`, `z`, `d = (i8)(1 - z)`, `(sx, sy) =
Get_Star_Draw_Coords_(star)`, `a = Get_Star_Anim_Dimension_(star)`, `sx -=
a/2`, `sy -= a/2`, `step = 11 - z`:
`x0 = a + sx - w/2 + d`, `y0 = sy + d - 1`, `x1 = sx - w/2 - d + 2`,
`y1 = sy + d - 2`, `xs[k] = x1 - 1`, `ys[k] = y1 + step*(k+1)` (k = 0..2).

`Get_Ship_Icon_Coords_In_Space_` (0xa0cd5): `Get_Ship_Icon_Dimensions_(z)`,
`x = Scaled(ship.x - cur_map_x) + 0x15 - width[z]/2`, `y = Scaled(ship.y -
cur_map_y) + 0x15 - height[z]/2`.

`Get_Ship_Icon_Dimensions_(i)` (0xa1bc9): `pic = Buffer_Reload("BUFFER0.LBX",
0xcd + i)`; `width[i] = (u8)pic.width`, `height[i] = (u8)pic.height`. Called
with **z** here but with `p = max(3-z, 0)` by the line routines; entry 0xcd+i
is banner 0's i-th size, smallest first, so in-space icons are centred with
the dimensions of the wrong size (z=0: 11×11 for a 16×12 picture). Port as is.

## Pictures: Get_Ship_Icon_Pict_Seg_ (0xa0d78)

`(owner, hl) → picture`, from `BUFFER0.LBX` (string 0x179be1) via
`Buffer_Reload(..., _global_cache_seg)`. `p` = 0 for z=3, 1 for z=2, 2 for
z=1, 3 for z=0.

| Owner | Entry | Stored in |
|---|---|---|
| 0..7 | `0xcd + banner*4 + p`, banner = player+0x26 | `_ship_icon_seg[banner][p]` (hl: `_hl_ship_icon_seg`) |
| 8 Antarans | `0xed + p` | `_antaran_ship_icons[p]` |
| 9..14 monsters | `0xf1 + (owner-9)*4 + z` (z, not p) | `_monster_ship_icons[owner-9][z]` |
| ≥ 15 | returns null | |

Verified by rendering: 0xcd..0xec are 8 banners × 4 sizes,
smallest first (banner 0: 11×11, 12×11, 12×10, 16×12; banner 0 red); 0xed..0xf0
Antaran 11×10 → 18×15; 0xf1..0x108 six monsters, largest first (Guardian
16×13, 14×11, 9×9, 7×7). Every entry: 8 frames, repeat 0, mask 0x04 (kind 0,
animated sprite); frame 0 is the plain icon, frames 1..7 a symmetric highlight
pulse (pixels differing from frame 0: 48, 78, 88, 88, 88, 78, 48 for entry
0xd0). `hl` only selects which table slot stores the pointer: the cache hands
back the same picture, so normal and highlighted icons share one
`current_frame`.

## Set_Ship_Icon_XYs_ (0xa07c8)

For i = count-1 down to 0: `Get_Ship_Icon_Coords_(i)` → `icon[i].x/y`.

## Draw_Ship_Icons_ (0xa070f)

`(all)`. `sel = -1`; if `Moveable_Box_Selected_(2)` (box[2].content >
`_box_off_flag[2]`): `sel = Ship_Icon_Id_From_Node_Ptr_(
Node_Ptr_From_Ship_Stack_(_fleet_box_ship_stack))`. For i = count-1 down to 0:
coords → `icon[i].x/y`; `pic = Get_Ship_Icon_Pict_Seg_(Ship_Icon_Owner_(i),
0)`; if `i == sel || i == _main_screen_ship_icon_scanned`: only when `all`,
`Reset_Animation_Frame(pic)` and `Draw(x, y, pic)`; else always reset and
draw. So every plain icon shows frame 0 (Draw then advances the shared frame
to 1). `Draw_Main_Screen_` passes 0, `Draw_Main_Main_Screen_` (0x8440e) 1.

Highlighted icons, drawn later in `Draw_Main_Screen_` without a reset (frame
= whatever the shared picture holds, so they pulse only while no plain icon
of the same picture is drawn):

- `Draw_Fleet_Box_Selected_Ship_Seg_` (0x843b3), per moveable box in order when box 2 is open:
  pict(owner, 1), `Draw(icon.x, icon.y)` of the fleet box's icon.
- `Draw_Scanned_Ship_Seg_` (0x854c0), when `0 ≤ scanned < count`: pict(owner,
  1); `Get_Ship_Icon_Coords_` (result discarded); `Draw(icon.x, icon.y)`.
  (A banner range test on the owner has no effect.)

## Do_Ship_Destination_Lines_ (0xa080d)

For i = count-1 down to 0, `ship = Ship_Icon_Ship_Id_(i)` (`node[icon.node].ship`,
0 if node outside 0..499), `stk = icon.node`:

- `enemy = Ship_Owner_(ship) ≠ PLAYER_NUM`
- `own = !enemy && Absolute_Location_(ship.location) ≠ NUM_STARS`
- `threat = enemy && ship.status ≠ 0 &&
  Enemy_Ship_Heading_Toward_Our_Colony_(stk)` (0xa1341: head ship not ours,
  status ≠ 0, and `Player_Has_Colony_(PLAYER_NUM, dest)` (star+0x38 bit) or
  `Player_Has_Outpost_` (any of the star's 5 planets with a colony whose
  +6 ≠ 0), dest = `Absolute_Location_(location)`)
- `boxed = Moveable_Box_Selected_(2) && Node_Ptr_From_Ship_Stack_(_fleet_box_ship_stack) == stk`

Skip unless `own || threat || boxed`; skip unless `500 ≤ location < 1500`;
`dest = Absolute_Location_(location)` (500..999 → -500, 1000..1071 → -1000,
else unchanged); skip if `dest == NUM_STARS`. `(sx, sy) =
Get_Star_Draw_Coords_(dest)`, `(ix, iy) = Get_Ship_Icon_Coords_(i)`;
`Draw_Ship_Destination_Line_(sx, sy, ix, iy, threat || enemy ? red :
green)`. If `500 ≤ location < 1000`: `Print_Eta_On_Ship_Icon_(stk, z, ix, iy)`.

Community patch 1.51: the call to `Enemy_Ship_Heading_Toward_Our_Colony_`
(0xa08a6) jumps to its code at 0x40452b, which takes the fleet as a threat
without the call when Enemy Moves (`_settings` +3, 0x199bdf) is 1: every
moving enemy fleet among the icons (those the player sees) gets its red
line. Drawing only; the port does so (`doShipDestinationLines`).

`Draw_Ship_Destination_Line_` (0xa1299): `p = max(3 - z, 0)`; if
`_current_screen` is 0, 0x27 or 0xc: `Get_Ship_Icon_Dimensions_(p)`, `ix +=
width[p]/2`, `iy += height[p]/2`; else `ix += 4`, `iy += 4`. Then
`Draw_Directional_Multi_Colored_Line_(ix, iy, sx, sy, colors,
_multi_colored_line_start)`.

`Draw_Directional_Multi_Colored_Line_` (0xa11c0) `(x1, y1, x2, y2, c, start)`:
`rev = max(7 - start, 0)`. If `(x2 > x1 && y2 ≥ y1) || (x2 ≤ x1 && y2 > y1)`:
`Multi_Colored_Line(x1, y1, x2, y2, c, rev)`; else with `r[7-k] = c[k]`:
`Multi_Colored_Line(x1, y1, x2, y2, r, start)`. (Makes the dots crawl from
the ship toward the star whichever way the line is drawn.)

`Print_Eta_On_Ship_Icon_` (0xa13b0) `(stk, z, x, y)`: `owner =
Ship_Stack_Owner_(stk)`, `ship = node[stk].ship`, `pic =
Get_Ship_Icon_Pict_Seg_(owner, 0)`; `Set_Player_Font_Style_(owner, 0)`
(0x7a00a: style 3/2/2/1 for scale 10/15/20/30, colours from
`Get_Player_Font_Colors_`); `Get_Font_Colors_(cols)`; `Set_Font_Style(z ≤ 1 ?
1 : 0, cols)`; `sprintf(buf, H_Message_(0x133), ship.eta)` — HESTRNGS.LBX
string 0x133 = `"eta %d"` (strings follow the 4-byte record header of entry
0); `Print_Right(x + pic.width, y + pic.height, buf)`.

## Draw_ETA_Destination_Line_ (0x83dea)

Called by `Draw_Main_Screen_` after `Do_Ship_Destination_Lines_` when
`_eta_star_id > -1 && _fleet_ship_scanned == -1`. `p = max(3 - z, 0)`; `i =
Ship_Icon_Id_From_Node_Ptr_(Node_Ptr_From_Ship_Stack_(_fleet_box_ship_stack))`;
`(x, y) = Get_Ship_Icon_Coords_(i)`; `Get_Ship_Icon_Dimensions_(p)`; `x +=
width[p]/2`, `y += height[p]/2`; `(sx, sy) = Get_Star_Draw_Coords_(_eta_star_id)`.
Colours from `_g_ship_move_info` (set by `Ships_Try_To_Move_To_` when the
mouse is over a star): green if `reachable (+5) ≠ 0 && (u8)eta (+4) > 0`
(-1 counts as > 0); else red if `blocked (+0)` or `flux (+0xd)` or `(out_of_range
(+6) && !no_drive (+2))`; else no line. `Draw_Directional_Multi_Colored_Line_
(x, y, sx, sy, copy of colours, _multi_colored_line_start)`.

`_g_ship_move_info` (from `Ships_Try_To_Move_To_`, zeroed first): +0
blocked (black hole / nebula without navigator **?**), +2 no FTL (player+0x5a0
== 0), +4 i8 turns (-1 unreachable), +5 can move, +6 out of fuel range, +7
navigator, +8 speed, +9 parsecs short, +0xa wormhole to dest, +0xb jump gate
both ends, +0xc star gate both ends, +0xd hyperspace flux. +1, +3 unused.

## Selection helpers

- `Ship_Icon_Id_From_Node_Ptr_(n)` (0x78800): first icon whose node == n;
  else its chain is searched, but the walk checks only every second node
  (`next`, then skips one) — **bug**, harmless for head nodes. 0 if not found.
- `Node_Ptr_From_Ship_Stack_(s)` (0x7836a): 0 if no stacks; `s > count`:
  `Vanish_Fleet_Movement_Box_`, 0; `s == count`: `Ship_Stack_From_Saved_Array_`
  0x789d4 (finds the stack of a ship in `_fleet_box_ship_ids`); else `start[s]`.
- `Get_Fleet_Box_Selected_Ship_Ids_(buf, n, type)` (0x7802a): `buf[0..n) =
  -1`; for node k = 0..499 (until > 500 found): patch stub 0x169169 clears
  `node[k].selected` if `ship[node[k].ship].status == 5`; ship counts if
  `selected` and owner == PLAYER_NUM and (`type == 0xff` or design.type ==
  type); returns the count.
- `Some_Icon_Selected_` (0x72617): box 2 open and
  `Get_Fleet_Box_Selected_Ship_Ids_(local[500], 500, 0xff) > 0`.
- `Reload_Ship_Selection_Status_` (0x71b1c): if `Ship_Stack_Owner_(
  Node_Ptr_From_Ship_Stack_(_fleet_box_ship_stack)) == PLAYER_NUM`: for k =
  0..499, `node[k].selected = Test_Bit_Field_(_fleet_icon_selection_status,
  node[k].ship) && Ship_Can_Be_Selected_(node[k].ship)` (→
  `Player_Can_Order_Ship_` 0x7843d, not decoded).
- `Save_Ship_Selection_Status_` (0x71b91) is the inverse over the box's
  `_fleet_movement_icon` list (9-byte records, node at +7).

## Callers

- `Main_Screen_` entry (0x8631a): `star.owner = Star_Owner_(s, PLAYER_NUM)`
  for all stars, `Build_Ship_Icons_`, `Set_Ship_Icon_XYs_`; then
  `_clear_fleet_fields` → `_first_node_ptr_in_stack = -1`, else
  `Some_Icon_Selected_()` → `Reload_Ship_Selection_Status_()`. Rebuilt again
  after a move order (0x868e0); `Scan_Moveable_Box_Fields_` also calls both
  (trigger not traced).
- `Update_Ship_Icons_In_Between_Turns_` (0x8a4c4): the same star owner loop,
  `Build_Ship_Icons_`, `Set_Ship_Icon_XYs_`. Called by `Load_Game_` and ~30
  turn/combat routines.
- `Draw_Main_Screen_` order inside `Set_Window(0x16,0x16,0x20f,0x1a5)` +
  `Clip_On`: … `Draw_A_Gate_Icon_` per star, `Do_Ship_Destination_Lines_`,
  `Draw_ETA_Destination_Line_` (condition above), …, `Draw_Stars_`,
  `Draw_Ship_Icons_(0)`, box loop with `Draw_Fleet_Box_Selected_Ship_Seg_`,
  `Draw_Scanned_Ship_Seg_`, moveable boxes; `Clip_Off`, `Reset_Window`; …;
  `_multi_colored_line_start = (+1) % 8`.

## Engine primitives

| Primitive | Address | In `internal/engine` |
|---|---|---|
| `Draw` (clipped path: `Clipped_Draw_Animated_Sprite`) | 0x12a478 | unclipped only; returns an error when `Clipping` is set — clipped animated sprite missing |
| `Reset_Animation_Frame` | 0x12b726 | trivial (`p.CurrentFrame = 0`), no helper |
| `Set_Window` / `Reset_Window` / `Clip_On` / `Clip_Off` | 0x128ab6 / 0x128be7 / 0x12b634 / 0x12b65c | fields exist (`Video.Clipping`, `WinX1..`), no functions. `Set_Window` clamps x1,y1 ≥ 0 and x2,y2 < screen size |
| `Multi_Colored_Line` (+ `Clip_Line` 0x135357, `Draw_Horizontal/Vertical_Run_Multi` 0x135edd/0x135f5d, `Swap_Short`) | 0x135870 | missing. Run-slice (Abrash) line, colour `c[rot & 7]`, rot++ per pixel; scan list: dots per pixel for vertical/diagonal/y-major runs, rows via `Add_Square_To_Scan_List_` for horizontal/x-major runs (first and inner runs one pixel too long) |
| `Add_Dot_To_Scan_List` | 0x125e13 | missing (`AddSquare` exists) |
| `Buffer_Reload` (LRU cache of LBX entries, `Farload_Buffer_Library`) | 0x127c27 | missing; needs one shared picture object per (file, entry) |
| `Set_Font_Style`, `Get_Font_Colors`, `Print_Right`, `Get_String_Width` | 0x120bb5, 0x8f625, 0x1210b7 | `font.Printer.SetStyle`, `Text.PrintRight` exist; `Set_Player_Font_Style_` / `Get_Player_Font_Colors_` (game) missing |
| `qsort_` (Watcom) | 0x133ef4 | missing; order of equal keys matters (slots, ship keys) |
| `Range_`, `Test_Bit_Field_` 0x1276f0, `Delete_Structure_` 0x1331d2 | | missing (small) |

## Open questions

1. `Buffer_Reload` eviction: if the cache drops and reloads a BUFFER0 entry,
   the picture comes back with `current_frame` 0 and the old pointers in the
   tables go stale. Needs `Farload_Buffer_Library` read.
2. `Multi_Colored_Line` was taken from Ghidra pseudocode of a typed module;
   verify the scan-list edge cases against the assembly before porting.
3. Ship types other than 0/1/2/4 in a stack sort on garbage keys; check
   whether any design (monsters, Antarans) has such a type.
4. The `xy == loc-1000` branch of `Build_Ship_Icons_` (stack invisible):
   which order leaves a status-2 ship heading to its own star.
5. Meaning of player +0x1cc/+0x18b/+0x1b7 == 3 (scanner techs **?**) and
   colony +0x13e/+0x15e/+0x15f (radar buildings **?**) in detection.
6. Hit testing of icons: `Get_Mouse_Field_` 0x857f8 returns globals
   0x199ec9..0x199ecf filled by `Check_Mouse_XY_Fields_` 0x82ac1 (not traced
   here); `Add_Ship_Icon_Fields_` is dead code.
