# The planet summary (PLNTSUM.C), screen 0x20

Go: `internal/game/plntsum.go`. Addresses are ELF. Reached with the PLANETS
button (hot key P) of the galaxy screen; RETURN (Esc) goes back to screen 0.
Ported: the whole screen. Mouse paths compared with the original by
`TestPlanetSummaryMouse` (build/oracle/planetsmouse1, `-clicks`): a row,
SEND COLONY SHIP (the ship's order and ETA in the save), the sorts and a
filter by click, a star of the small map, the arrows, a drag of the scroll
bar (a held press); `TestPlanetSummaryRerouteMouse` (planetsmouse2): a
row clicked while the ship is under way asks to re-route it (y) and lays
the new course; `TestPlanetSummaryOutpostMouse` (planetsoutpost1: fcol1's
save with ship 22's design type set to 4 by saveedit): a row, SEND
OUTPOST SHIP, its box: the ship leaves for Jordanus (status 2, ETA 3) and,
as in the original, the design's first weapon words take the target
(33, 1); captures, draws and the F10 save match.

## Data

`Allocate_Space_For_Summary_Screen_` (0x99a6f) takes four blocks of
screen_seg: `_planet_summary` (0x2760 bytes, 0xff), `_screen` (0x115, 0),
`_ship_summary` (4000, 0xff), `_scroll_bar` (0x1d, 0). The port keeps
`_planet_summary` as bytes with the 0x1c bytes before it (the block header
{0, 0x2760, 0} ends there), because record -1 is read (see quirks).

`_planet_summary` record, 0x1c bytes: +0 max population, +2 food per farmer,
+4 `_minerals_per_mine[class]` (0xdd4b5), +6 mineral class, +8 climate, +0xa
planet, +0xc/+0xe colony/outpost ship ETA (+1000 in flight, +2000 leaving
orbit, 9999 reset, -1 none), +0x10/+0x12 the nearest colony/outpost ship,
+0x14 byte "a black hole blocks every ship", +0x15 its `_ship_summary` row,
+0x1a/+0x1b byte ETA of a ship already sent to settle the planet. Record 0's
spare bytes +0x17 (word) and +0x19 (byte) hold the marker colour cycle and
its direction.

`_ship_summary` row, 8 bytes: colony ship, outpost ship, ship icon, and two
"usable" flags (+6 colony, +7 outpost).

`_screen`: pictures at +0..+0x34 (PLNTSUM.LBX 0 background, 1..3 sort
buttons, 4..8 filters, 0xb/0xc arrows, 0xe RETURN, 0xd the bar's thumb, 0x4c +
banner colour the ship icon), fields from +0x38 (3 sorts, 5 filters, the two
SEND areas, 8 rows, 8 colony and 8 outpost icon fields, up, down, RETURN, 72
star fields), +0x112 "add the fields again", +0x113 the last scan (written
only). PLNTSUM.LBX 9/10 are the SEND COLONY/OUTPOST SHIP buttons (frame 0 off,
1 available, 2 chosen), 0xf..0x19 the small map's stars
(`_planet_summary_galaxy_star_seg`), 0x1a.. the 50 rotating planets
(climate·5 + size).

Initialised statics (0x182306..): `_sort_choice` 0 (climate; 1 minerals, 2
max population), the filters gravity, hostile, mineral, enemy 0 and "Planets
In Range" 1. They survive leaving the screen.

## Planet_Summary_Screen_ (0x9d252)

Outline colour 0xd7, help list `_plntsum_screen_help_list` (0x181862, 15),
`_send_choice` -1 (0 colony ship, 1 outpost ship), `_scanned_field` -1,
`Check_For_Colony_And_Outpost_Ships_` (buttons 1 when found), auto function
`Draw_Mini_Planet_Summary_Screen_`, `Fade_To_Black_`, palette 6 (0..0xfe),
pictures, fields, the marker cycle at 0/+1, `Set_Planet_Explored_Flags_`
(`Ok_To_View_System_` per planet), `Filter_Explored_Planets_` into the bar's
row count, the bar (pos = track = 0x197, 8 visible, kind 2, step 1, thumb
height from `Get_Vertical_Scroll_Bar_Height_`, pos = track - h/2), the
rotating planets, `Determine_Button_Status_`. Each pass: Get_Input, Scan_Input;
RETURN leaves; a sort button that changed `_sort_choice` sorts again; a filter
filters again (first row clamped, thumb moved) and rebuilds the ship summary
when ships were found; the arrows scroll by one when more than 8 rows; a star
field jumps to its first planet (H 0x14f when none); hovering one makes it
the scanned star; `Scan_Summary_Box_Fields_`; the SEND buttons (H 0x13f under
a hyperspace flux) choose or drop their kind; the fields are added again when
asked. The first frame fades in, then pages toggle. Leaving: fields cleared,
`Fade_To_Black_`, `_scroll_bar` = NULL, `Draw_Mini_Main_Screen_`, outline
colour 10.

## Drawing (Draw_Mini_Planet_Summary_Screen_ 0x9d0c6)

Background; `Draw_Rotating_Planets_` (centred in 0x56x0x19 at row tops
`_plntscrn_delta_y`+0x2d); `Wrapper_For_Planet_Summary_Galaxy_Map_` (window
0x1bb,0x11..0x26f,0x85: the route of the nearest ship to the scanned star, a
9x9 box in its owner's colour (0xd4 + 6·colour, else 0xe2), "c"/"o"/"oc"
markers by the ships, `Draw_Galaxy_Map_Box_` mode 2 where the scanned star
blinks through `_galaxy_star_anim_count`); `Draw_Generic_Vertical_Scroll_Bar_`
with `Fill_Summary_Screen_Scroll_Bar_` (ten lines 0x96 down, the thumb
picture); `Summary_Screen_Print_Data_` (per row: name in colours of the owner
and hover state (`Player_Colors_` modes 0/1/2), special, "(race)", climate
and food, gravity and penalty, minerals, size and max population, ship
icons); `Summary_Screen_Print_Scanned_` (the line at 0x215,0x8f); the SEND
buttons.

## Quirks kept

- `Filter_Explored_Planets_` copies its new list only when the row count
  changed; with the same count the old list (and its sort) stays.
- The row text window is one row too tall: its bottom is the next row's top
  (`_plntscrn_delta_y` read one past the end for the last row, 0x1df).
- `Check_For_Colony_And_Outpost_Ships_` sets `_found_colony_ship` for outpost
  ships too, so its loop runs over every stack.
- `Build_Ship_Summary_`: once both kinds are found, every later stack adds
  only its first ship.
- `Draw_Mini_Planet_Summary_Screen_` asks for the routes of ships in flight by
  their location (500+), which `Summary_Planet_Index_` (a byte compare) never
  finds: nothing is drawn.
- `Summary_Screen_Print_Scanned_` with a hovered star (1000+) reads record -1
  (the block header's size byte, 0x60) as its "unreachable" flag.
- The star special 9 branch (`First_Ship_Orbiting_Star_`, which scans stars
  up to NUM_SHIPS) is unreachable: 9 is not a planet special.
- With no ETA for a reachable, unvisited star the original prints its
  uninitialised buffer; the port prints nothing.
- The colony icon's field is at x 100-w, the icon is drawn at 0x66-w; the
  outpost icon's field at 0x66, drawn at 0x12.

## Engine

Add_Multi_Button_Field (type 3: pushing it stores its value in the shared
variable, `Push_Field_Down` keeps it down under other fields, Draw_Field up
writes -1000), Add_Locked_Button_Field (type 2: a press sets the variable to
1), and the press handling of radio (type 1, flips) and locked buttons in all
three paths of `Interpret_Mouse_Input`; `Draw_Galaxy_Map_Box_` modes 2 and 3;
keypad + and - keys.

## Check

`TestPlanetSummaryStates` replays the oracle run of `build/oracle/plntsum`
(SAVE1 as SAVE10, pointer at the screen centre, which hovers row 4): P, filter
5 off, keypad + (scroll), O (outpost ship), filter 1 on, C (colony ship), Esc;
each state matches the original's capture outside the small map (the
blinking star) and, back on the galaxy screen, outside the map window (black
holes).
