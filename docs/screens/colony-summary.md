# The colony summary (COLSUM.C), screen 0x14

Go: `internal/game/colsum.go`. Addresses are ELF. Reached with the COLONIES
button (hot key C) of the galaxy screen. RETURN (Esc) goes back to the screen
it came from (`_colony_summary_saved_screen`). It draws through COLONY.C
routines it shares with the colony screen (screen 1, not ported), which are
ported in their summary forms. Clicking a name goes to the colony screen
(screen 1), a product to the build popup (screen 0x19); both are routed only.

## State

Initialised statics, kept between visits: `_first` and `_slider_bar_position`
(0x18315a, 0x18315c: the first row shown and the bar), `_g_sort_index`
(0x183104: name, population, food, industry, science, producing, BC). COLONY.C
globals: `_g_colony_n` (the scanned colony), `_cluster_colony_n` (-1, the
colony whose colonists are picked up), `_scanned_pop`, `_step_squish`,
`_full_draw`, `_g_count`. The list itself (250 words) lives on the screen's
stack (`_g_colony_list_ptr`); `_list_col` holds the ten visible colonies.

Pictures: COLBLDG.LBX 0..6 then COLSUM.LBX 0..0x14 over them in `_anims`
(0 background, 1 RETURN, 2/3 arrows, 4..10 sort buttons, 11..20 buy buttons),
COLSUM.LBX 0x15, gstar.lbx 0x17..0x20 for the small map (mode 3).

## Colony_Summary_Screen_ (0xc4562)

Saved screen unless coming back from the build popup; pictures; the player's
colonies (not outposts), bubble sorted by `Switched_cmp_` (250 passes);
`Update_Col_List_` (rows from `_first`, then all fields again: up "+", down
"-", RETURN, Esc, seven multi buttons on `_g_sort_index` with hot keys n p f i
s r b, the bar when ten or more colonies, "z" (redraw), per row a name field,
a product field, a buy button (locked button on `_list_buy_data`, or a hidden
field when the product cannot be bought), the three job scroll fields
(`Do_Colony_Info_Pop_Stuff_For_Pop_` mode 1), and a full-screen " " field).
If row 10 is empty the list restarts at 0. Palette 1 (with a fade when
`Fast_Fade_Out_Needed_For_Colony_Palette_`), `Draw_Palette` of the
background, glass remap; the scanned colony is row 0's; help list 0x18329a
(0x16 entries); the full draw, fade in, then Get_Input/Scan_Input/
`Evaluate_Input_`/draw/flip until RETURN.

`Evaluate_Input_` (0xc4343): up/down move `_first` by one, RETURN/Esc drop a
cluster and leave, "z" redraws, a sort button sorts and restarts at the top;
otherwise `Evaluate_Col_List_Input_` (names: open the colony screen or send
the cluster; right click on a name: the planet popup; product: the build
popup; buy: `Tested_Colony_Buys_Outright_`) and `Evaluate_Colony_Pop_Input_`
(job fields: pick up colonists with `Get_Cluster_`, or send the cluster
there with that job; hovering a colonist scans it), then
`Check_Galaxy_Map_Fields_`.

## Drawing (Draw_Colony_Summary_Screen_ 0xc3d34)

`Update_First_` follows the bar. Full draw: background, rows refreshed,
`Draw_Empire_Info_` (reserve, income in red when negative, population,
free freighters, food, research; `String_Builder2_` lines joined by "\r",
formatted paragraph at 0x208,0x162). Always: the small map
(`Draw_Galaxy_Map_Box_` mode 3 at 0x17c,0x15d, 0x80x0x5b), each row
(`Draw_Colony_Summary_For_Colony_`: name with its event (`_wee_event_strings`,
colour by `_event_good_lookup`) or "Blockaded", squeezed and centred in
0x59x0x17; on a full draw the colonists and the product with its turns or
"Autobuilding"), the fields on a full draw, `Draw_Colony_Scan_Info_` (the
scanned colony's output icons by `Draw_Colony_Prod_Both_` in small form, its
morale, and its planet: size, climate, gravity, minerals, population/max,
growth), the scanned star's name at 0x1bc,0x1af, the line to the cluster's
colony, the planet popup, `Draw_Bar_Indicator_`, the fields, the cluster at
the pointer.

`Do_Colony_Info_Pop_Stuff_For_Pop_` (0xc4ef6) walks a colony's colonists of
a job in display order (state 0..6, the flag bit 0x400, races 9, 0..8) to
draw them (the scanned one tinted on clock ticks: `Global_Cycler_(30, 2)`,
`Tint_Bitmap` through glass remap 2), list them with a scroll field, count
them (and set the overlap `_step_squish`), or find the one at an x.

## Moving colonists (the cluster)

`Get_Cluster_` picks up a colonist and the identical ones after it (clears
bit 0x200); the pointer goes blank and `Draw_Cluster_` draws them by it.
`Send_Cluster_` (0xc9e94) puts them back with a new job (same colony), or
moves them through `Pop_Tries_To_Settle_` and `Settle_Pop_` (same system: at
once; else as settlers with `Settler_ETA_`), with the confirmations (ETA, no
room) asked once per kind and the reports (E 0x1d6 refusals, E 0x1d5 moved).
`Clear_Cluster_` re-employs them.

## Quirks kept

- `Update_Col_List_` clears every field and adds them again; `Check_Galaxy_
  Map_Fields_` tests the map's star fields, which this screen never adds, so
  clicking the small map does nothing.
- `Draw_Planet_Summary_` (the right-click popup) computes the remap colours
  of the planet picture and never draws it.
- `Tested_Colony_Buys_Outright_` ends by assigning the colony screen's auto
  function and help list; its caller replaces them at once.
- `Settle_Pop_`'s settler word starts from ECX, which `Send_Cluster_` leaves
  at cluster·0x169; only bits that are overwritten come from it.
- Draw_Fake_Buttons_ runs, but nothing adds fake buttons here.

## Not ported

The government icon of the full-size morale row and the shadow farmers are
the colony screen's; they stop with an error if reached.

## Check

`TestColonySummaryStates` replays `build/oracle/colsum` (SAVE1 as SAVE10,
pointer at the centre): C, the seven sorts by hot key, three scrolls down,
one up, back to names, Esc; every state matches the original's capture outside
the colonist columns, and inside them up to two colonist cells may differ:
the scanned colonist is drawn tinted on alternate clock ticks
(`Global_Cycler_(30, 2)`), and after a sort or scroll the full redraw uses
the scanned colonist of the previous frame (Evaluate_Input_ returns before
scanning), so the phase it caught stays in the rows. On the galaxy screen the
map window (black holes) is skipped.

`TestColonySummaryMouse` (build/oracle/colsummouse1, `build/oracle/colsummouse1.sh`,
`-clicks -clickkeep`: `Update_Col_List_` rebuilds the fields every pass)
plays the mouse paths: colonists picked up from a job field and dropped on
another job, scientists sent to another system's colony (the population
limit question, YES; the settlers boxes), the name's right-click popup,
three buys (trade goods refused, too dear, one bought with YES), the scroll
bar. Every capture (colonist blink and the small map's line phases
skipped), every draw and the F10 save match the original.

The keys Q and 0-9 (build lists) and BUY taking a purchase back come
from the community patch 1.51: `build-lists.md`.
