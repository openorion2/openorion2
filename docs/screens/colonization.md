# Colonisation in the report phase (Colonization_ and its popups)

`Colonization_(p, show)` (0xfdb01, report code 1 or 3; the query side is in
`reports-and-begin-of-turn.md`) asks the human what to do with a colony
ship, outpost ship or colony base at a star. Addresses are ELF. Go:
`internal/game/colonize_popups.go`.

## Show path (0xfdbe5)

Per star found (base = the colony base there, colony / outpost = p's ships),
repeated while a base is left:

1. Outpost (no base, and no colony ship or an outpost ship there): the
   planet the ship was sent to (`Colony_Ship_Auto_Colonization_`: design
   weapon 1 == 1 → weapon 0) if still free → "Build outpost on the
   preselected planet of %s?" (report string 7, `Do_Confirmation_Box_`; no
   → −1), else `New_Outpost_Selection_Popup_` (mode 1).
2. Colony: without a base, a free preselected planet → report string 6;
   else `New_Colony_Selection_Popup_(star, base == none)`. While a base
   remains and nothing was picked: "Really trash your colony base for
   %dBC?" (string 5, half the s32 at 0x17ec16); no → the selection again;
   yes → every base at the star: BC += refund, the colony's building 0xb = 0.
3. Both ships lose their auto-colonisation orders
   (`Clear_Auto_Colonization_Flag_`).
4. Nothing picked: done bit (`done_colonize_check`), star+0x35 |= 1<<p.
   A colony: `Make_New_Colony_` (`Make_New_Colony_Or_Outpost_(pl, p, 0)`),
   then the ship dies (`Kill_Ship_`, `Update_Ship_Icons_In_Between_Turns_`)
   or the base's building 0xb goes (`Remove_Building_`) and the next base
   is looked up; no colonisable planet left → done bit and every base
   refunded. Without animations (settings+6) → 1; with them → screen 0x21
   (the landing, `_screen_data` = star, orbit), return 0x27, `_global_esc` →
   3. An outpost: `Make_New_Outpost_`, the ship dies.
5. After a confirmed preselected planet, `Can_Change_Star_Name_` (no colony
   at the star) without animations runs `Change_Star_Name_Popup_`.

## New_Colony_Selection_Popup_(star, using_ship) (0xc8db8)

`_last_scanned_field` = −1, `_g_using_colony_ship`, style 1 shadow-down in
{0x9f, 0xa0, 0xa1, 0xa2}; MAINPUPS 0 loaded and dropped;
`Set_G_Special_Value_` (a special newly seen at the star, star+0x34 bit
cleared, markers ≥ '2' zeroed — state); BUFFER0.LBX 0x49 (cached) for
`Do_Popup_Setup_`; auto `Draw_Selection_Popup_`; `Eric_Push_Fields_`;
`Clear_Sys_Disp_Fields_`; `Add_Selection_Fields_`
(`_draw_system_display_without_button` = 1; the system display's fields
through `Wrapper_For_Informational_System_Fields_` — whose close pointer
0x1a11c8 is non-null, so the close button field (Esc) is added though not
drawn — and a hidden field over the screen with hot key E 0xc = "").
Loop: `Release_Time(2)`, `Get_Input`, `Scan_Input`,
`Evaluate_Selection_Input_`, draw, flip, until done. Then the star name
popup as above; `Do_Popup_Cleanup_`.

`Draw_Selection_Popup_`: style 4 `_main_scr_font_colors` (0x1831b7); full
draw → the screens behind, off → back; title by mode (E 0x1cd colony base,
0x1ce colony ship, 0x1cb outpost, 0x1cf troops, 0x1cc artificial planet)
with the star's name; `Draw_System_Display_Popup_`; a special at
(x, y+0x32).

`Evaluate_Selection_Input_`: no special → the close button cancels (−1,
done), else `Check_System_Display_Fields_Selection_`; special 8 → the
marooned officer popup and a reset; other specials go on input.

`Check_System_Display_Fields_Selection_` (0xc8981): `Scan_System_Display_
Fields_`; a clicked planet (star.planets[slot]) must suit the mode — colony:
a planet (type 3) without a colony (outposts may be settled over);
outpost: no colony or outpost; troops: the human's colony; artificial:
not a planet (E 0x235). Else E 0x236 / 0x23b in `Message_Box_` (`Help_`)
and the popup is reset (twice). A colony needs `New_Colony_Confirmation_Box_`.

`New_Colony_Confirmation_Box_` (0xc9fc4): E 0xd1 with the planet name, then
food per farmer (E 0x65 / 0x64 with the half), industry (0x6b), research
(0x73), maintenance penalty (0x6c), worker penalty (0x78), max population
(0x6e, `Planet_Max_Population_For_Player_`), each with the colour codes
"\x1a0", "\x1a1"; `Do_Confirmation_Box_` (Y / N).
`Compute_Potential_Colony_Info_` (0xe5430) gives the numbers from the
player's picks (tolerant, low/high G, farming, aquatic with the climate
food table 0x17d81c, industry, science), known applications 0x1d, 0x6c,
0x54, and the planet's climate (table 0xdd4ba), gravity, food, minerals
(0xdd4b5) and special (0xa +2, 0xb +5 research).

## Checks against the original

col1 (`build/turns/inputs/col1.GAM`: t1 with colony ship 10 put at Python,
star 6, which has a free planet; `col1.SET`: animations off; keys
`build/turns/col1.keys`): the selection popup, Esc → declined.
`TestTurnCol1Selection`: all draws, SAVE10 = the original's; the popup
frame identical but the rotating planets and black holes. col2 (col1 with
the ship sent to planet 19, keys `build/turns/col2.keys`): the preselected
planet's confirmation, yes, a new colony — whose empty build queue then
sends the original into the colony screen (`Empty_Colony_Queue_`), which is
not ported (it also reseeds the RNG in `Make_Bldg_Array_For_Colony_`), so
col2 has no end-to-end test yet.

With `-listkey` key 1 clicks a system display popup's first planet field:
the planets are added by display record from the last, so it is the
lowest planet on screen, which at Python is a settled one ("You cannot
build there.").

## Not ported

Nothing; the help lists are `Set_Planet_Selection_Popup_Help_List_`
(0x18352e, after each `Add_Selection_Fields_`) and
`Set_New_Colony_Confirmation_Box_Help_List_` (0x183506).
`Change_Star_Name_Popup_` and the landing screen (0x21) are in
docs/screens/colony-screen.md.
