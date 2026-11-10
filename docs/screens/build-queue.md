# The build queue popup (COLBLDG.C), screen 0x19

Go: `internal/game/buildq.go` (popup loop, fields, input),
`buildq_queue.go` (lists and the queue copy), `buildq_draw.go` (drawing,
the ship design paragraph). Addresses are ELF. Reached from the colony
screen's BUILD button and from a colony summary row's product (both set
`_current_screen` 0x19 and `_return_screen` to themselves).

## Build_Queue_Popup_ (0xb4041)

Seeds the RNG with the old `_colony_handle` (restored on leaving), takes
star and orbit from `_screen_data[0]`, `[1]`, the planet's colony becomes
`_colony_handle`, `_using_colony_screen_palette` is set (never cleared
here). With settings byte 0xd8 (strategic combat) design slot 5 of the
colony's owner is loaded with `Load_Doom_Star_Ship_Design_`. Unless
coming back from the design screen (`_return_to_build_popup_from_custom_
design`, 0x1827bd) and when `_current_screen != _return_screen`, the
return screen is kept in `_colony_bldg_saved_screen` (0x19dea6).
`Load_Colony_Screen_Seg_` (COLBLDG.LBX 0..6 into `_anims`, COLSUM.LBX
0x15, the transport buttons, the governor, road pictures, galaxy map
stars, COLPUPS buttons), help list 0x1833bc (13 entries). Unless back from
the design screen the colony queue is copied into `_current_item`
(0x1828ac, 7 words), `_active_prod` (0x1828aa) = −1 and
`_colony_auto_building` (0x19deac) = the colony's autobuild byte. The two
lists are computed, `Make_Bldg_Array_For_Colony_(ci, 0)`; a fade when
`Fast_Fade_Out_Needed_For_Colony_Palette_` (then `Update_Colony_Palette_`
0xbf55a: palette 1, the planet's palette, glass colours).

Loop: draw, toggle, Get_Input, Scan_Input, `Evaluate_Input_`; it ends on
OK, CANCEL, Esc, "<", ">" or the design-screen flag. On leaving: fields
cleared, the pointer reset if an entry is held, font spacing 3, help list
and auto function off, back page copied to the off page. Unless "<"/">"
(the popup runs again on the new colony): with the design flag screen 3
(Design_Screen_, not ported; `_first_design_screen_call` set, return 0x19);
else `_current_screen = _return_screen`, or when they were equal the saved
screen for both.

## Lists (Calculate_Building_Array_ 0xafefc, Calculate_Military_Array_ 0xaff9e)

Buildings: trade goods (−2) and housing (−3) head the list. The 1.40
patch (0x16948f) makes them −1 and −2 when bit 3 of byte 0xe of star 0 is
set. Then every building 1..0x30 but 0xb for which
`Colony_Can_Build_Product_` is 1 (all with `_cheats`), sorted by name
(`strcmp`). The count limit 0x36 is never reached.

Military: colony base (0xb) while the star has more uncolonised type-3
planets (`Star_N_Uncolonized_Worlds_`: no colony, or an outpost) than
queued bases and the colony can build one; freighters (−15) with
application 0x45 of `_PLAYER_NUM`. With fuel range and FTL speed of the
owner: colony ship (−12, application 0x29), outpost ship (−17, 0x6d),
transport (−11, 0xbd and building 0x16), a separator (−9) if anything so
far, the five designs (−50..−54) with a name (under setting 0xd8 a titan
design needs application 0xba), the doom star (−16, application 0x37 and
setting 0xd8). Then a separator if anything, and androids (−4, −5, −6)
and spies (−7) the colony can build.

## Drawing (Draw_Build_Queue_Popup_ 0xb3cf7)

Full draw: black, the planet backdrop, roads, buildings,
`Interlaced_Gray_Scale_Off_Page_(0xf6, 9, 0xf4, 10, 0)` (even rows a
9-step scale from colour 0xf6, odd rows 10 steps from 0xf4), the panel's
palette, `Darken_Fill(0xca, 0x130, 0x1cf, 0x1d6)`, the panel, the title E
0xd2 with the planet name (`Squeeze_Paragraph_Centered_` at 0xeb, 0x135,
0xc2 by 0x11, style 3). Coming from or saved for the colony summary
(0x14) the panel's and COLSUM 0x15's palettes are applied again. Every
frame, in style 4 (colours 0x1827ad):

- buildings at x 0xd (width 0xab), military at 0x1e5 (0x8a), rows from y
  0x14 with `Calculate_Squish_Step_`; `Calc_Font_Off_` and
  `Get_Font_Off_For_*` shrink the style (down to 2 below) while a name is
  too wide. Queued buildings bright (0x1827ad), others dim (0x1827b5).
  Military rows are dim for designs the colony cannot build (size 2+ with
  none of buildings 8, 0x28, 0x29; or `Colony_Can_Build_Product_` 0, not
  while picking a design) and for the doom star without slot 5's tech;
  while a design is picked the hovered one blinks (`Global_Cycler_`).
- the queue: seven rows centred on 0x14c from y 0x14f, style 3,
  `Squeeze_Print_Centered_` width 0xfb; the held entry (with its repeat
  mark) reads E 0x101 ("delete %s") while the pointer is on a queue row
  over it.
- the selection (hovered item, else queue slot 0): name at 0x132, 0x15,
  lines of 0xd, a picture centred on 0xf4, 0x38 (buildings in the window
  0xcb, 9..0x11d, 0x67), `Draw_Cost_And_Time_Info_` (E 0xf2 cost, E 0x16f
  maintenance for buildings, E 0xd3 build time, then E 0x220 turns left
  when it is the colony's product, or E 0x13a gravity for building 0x19)
  and a description at 0xd1, 0x77, width 0xf8: help record (buildings by
  their table, −17 0x6d, −12 0x29, −15 0x45, −11 0xbd, −7 0xa6, androids
  6/8/7) or MAINTEXT.LBX entry 0xc/0xd (trade goods, housing). Designs and
  ships print `Build_Ship_Design_Info_Formatted_Paragraph_` (style 1,
  squeezed into 0xd4, 0x72, 0xf8 by 0xba): cost, shield, computer,
  armour, drive, weapons and specials in two tab columns, names wider than
  0x6e (0x73) after them. The crew line needs the ship flag, which
  `Draw_Ship_Design_Info2_` never passes.
- `Draw_Active_Prod_`: a box (0xcf, 0x14c+0x14·slot, 0xfc by 0x10) round
  the held entry and its repeat mark, colour 0xef while hovered (pointer
  reset), else 0x64 with the pointer list 0x182765.

## Input (Evaluate_Input_ 0xb2542)

`_scanned` (0x19deaa) is cleared, the scan kept (0x1828be). Back from the
design screen or a refit the flags are cleared; with `_screen_data[4]` set
and the refit flag (0x199ef1) the refitted ship is queued
(`Add_Refited_Ship_To_Queue_`).

- OK: the autobuild radio is stored; while something follows trade goods,
  housing or a repeat mark (`Something_Behind_Settting_`), E 0x3b asks
  (skipped with settings byte 0xa) and those settings with entries after
  them are squeezed out (the loop skips the entry moving into the slot);
  no leaves the queue as it is. `Do_Exit_Screen_Cleanup_` frees ships that
  left the colony's queue and writes `_current_item` back.
- CANCEL / Esc: drops a held entry or a mode; else a queued ship in refit
  that is new asks E 0x1dc (no: stay), then ships created here are freed
  (`Remove_Ships_(0)`). The queue is not written back.
- DESIGN (d): picking mode 1 (pointer 0x1827a1): a design row opens the
  design screen on it. REPEAT BUILD (e): mode 2 (pointer 0x182795): the
  next added item gets a repeat mark (−10) after it.
- REFIT (r): needs fields 0x16 and 0x17 researched (else E 0x244);
  `Colony_Refit_Popup_` and `Colony_Refit2_Popup_` (docs/screens/
  design-screen.md, "Refit").
- b: `Do_Exit_Screen_Cleanup_`, then `Tested_Colony_Buys_Outright_`.
- "<" / ">": `Get_Next_Colony_` with 0 for "<" and 1 for ">", so the
  arrows are swapped ("<" goes to the next colony, as on the colony
  screen); the queue is written back.
- Building rows: an unqueued building is added (E 0x188 when the queue is
  full), a queued one deleted, except a bought slot 0 (E 0x23a).
- Military rows (not separators): colony bases only while planets remain
  (E 0x232); in mode 1 a design opens the design screen.
- Queue rows: the first click holds the entry (`_active_prod` = slot +
  0x31); a bought slot 0, or a repeat mark in slot 1 behind it (the 1.40
  patch at 0x169020), refuses with E 0x23a. A second click puts the held
  entry before the row (`Insert_Before_Queue_Slot_`, repeat marks move
  along) or, on itself or its own repeat mark, deletes it.

Then the colony is recalculated (`Colony_Calculation_`); refusals return
before that.

`Add_To_Queue_`: buildings, settings, freighters and ships go into the
first free slot; androids past the planet's room ask E 0x208; support
ships load design slot 5 (colony, outpost, transport loaders) and are
created at once (`Create_Ship_`, status 6; E 0x21a when no ship slot is
free); the custom ship (−13) leaves for the design screen with slot 4; the
doom star needs `Colony_Can_Build_Product_(−55)` (E 0x245). Designs the
colony cannot build say E 0x246 (titan), E 0x245 (doom star) or E 0x237.
While an entry is held the new one goes before it.

`Delete_From_Queue_`: a refitting ship asks E 0x213; ships not in the
colony's queue are freed; a repeat mark goes with its entry.
`Squish_Out_Queue_Slot_` also removes repeat marks moving into the slot.
`Insert_Before_Queue_Slot_` quirk: when the moved entry has a repeat mark,
the first squeeze already removed both, so the second squeeze drops one
more entry.

## Quirks

- `Sint_Is_In_List_` is passed 14 entries for the colony queue, so the
  test reads on into the colony record (0x115..0x130).
- "<" moves to the next colony and ">" to the previous one.
- `Add_To_Queue_`'s failure E 0x14c returns false, so the caller also says
  E 0x188.
- An item with no case in `Draw_Current_Selection_` (−9, −14, −18..−49)
  prints a debug line and waits for a key; the port logs it.

## Not ported

Nothing on these paths: DESIGN, refit and refit-to-new-design reach the
design screen (docs/screens/design-screen.md). The custom ship (−13) is
never in the military list.

The keys Q and 0-9 (build lists) and BUY taking a purchase back come
from the community patch 1.51: `build-lists.md`.
