# The ship design screen (DESIGN.C, DESBOX.C), screen 3

Go: `internal/game/design.go` (loop, fields, hot keys, Check_* input),
`design_template.go` (the template, costs, space, BUILD),
`design_draw.go` (Print_Current_Design_ and its helpers),
`desbox_generic.go` (shield and computer boxes), `desbox_special.go`
(special systems box), `desbox_weapons.go` (weapons box), `colrefit.go`
(the build queue's refit popups, COLREFIT.C). Addresses are ELF.

Reached only from the build queue popup (screen 0x19): DESIGN (d) puts the
popup into design-picking mode and a design row then sets
`_screen_data[4]` to the slot and `_return_to_build_popup_from_custom_
design`; the popup leaves with `_first_design_screen_call` set,
`_return_screen` 0x19 and `_current_screen` 3. The custom ship (−13) of
`Add_To_Queue_` would go the same way with slot 4, but −13 is never in
the military list. Refit to a new design (`Colony_Refit2_Popup_`'s −2)
goes there with slot 5 and `_refit_ship` (0x199ef1) set (see "Refit").

There is no separate design list: the screen redesigns the slot it was
opened on, and BUILD overwrites that slot (five designs, slot 5 for the
doom star and refits).

## Design_Screen_ (0x6b9b2)

Sets `_multi_colored_line_start` 0, `_incr_value` 1, the weapon/special
first-row counters 0 (never changed), `_use_augmented_engines` 1, the auto
function `Draw_Design_Screen_` (delay 2), the pointer list
`_mouse_list_variable`. The slot is `_screen_data[4]` at entry.

- First call (`_first_design_screen_call`): `Load_Design_Screen_Pictures_`
  (DESIGN.LBX 0..71 into `_screen_seg`, see below; `_design` 0x125 bytes
  and `_weapon_filter` 0xf0 bytes from `_global_data_seg`; TECHDESC.LBX
  (by language) entry 1 records 0..0x26 as the specials' descriptions,
  entry 3 records 0..0x27 as the weapons' notes),
  `Build_Initial_Design_Template_`, `_original_cost`, cost 0 when
  refitting, the printed space and cost, `Add_Design_Buttons_`, fade out,
  palette 4 with the player's ship palette, darkened 100, the screen to
  the off page, fade in; the flag is cleared.
- Later calls (after a box): `_changed_design_name` cleared, the ship
  palette, fields, the screen drawn and shown.

Loop: Mark_Time, Get_Input, Scan_Input into `_scanned_field`; the name
counts as changed once it differs from the slot's; the name field stops
field rebuilding (any other input restarts it). CANCEL (Esc):
`_screen_data[4]` 0, back to `_return_screen`. CLEAR (L): `Clear_Design_`.
BUILD (B, only while the space used fits): `Update_Player_Design_`,
`_screen_data[4]` 1, back. Then `Check_Generic_Replacement_Fields_`,
`Check_Weapon_Fields_`, `Check_Special_System_Fields_`,
`Check_Plus_Minus_Buttons_`; a new hull size (the size buttons are a multi
button field on `_design_size`) sets the hull's space *without* the 125%
of application 0x59 (until the next recalculation) and picture size·8
(0x2b for the doom star); + and − change a weapon's count (1..99, + only
while it fits: the new space computed in 16 bits). A size button also
renames the design after the hull (`Get_Ship_Design_Name_`, SHIPNAME.LBX
by race, size and picture mod 8) and clears `_changed_design_name`.
The screen is drawn every pass; after a box (exit code 0x63) once more,
then the loop ends and screen control calls it again. Each pass trims the
name (`Trim_`: leading and trailing blanks) and restores the slot's name
when empty. Release_Time(2). On leaving `_reloaded_ship_palette` is
cleared.

## The template (`_design`)

| Off | Field | Off | Field |
|---|---|---|---|
| 0x00 | name (15 chars edited in place) | 0x6d | weapon costs, 8 × i32 |
| 0x11 | shield, cost 0x13, space 0x15 | 0x8d | weapon spaces, 8 × i32 |
| 0x1b | drive, speed 0x1d, cost 0x1f, space 0x21 | 0xad | ammo, 8 × u8 |
| 0x25 | combat speed | 0xb5 | specials, 8 × i16 |
| 0x2b | computer, cost 0x2d | 0xc5 | special costs, 0xd5 spaces |
| 0x2f | fuel | 0xe5 | hull size, 0xe7 picture |
| 0x31 | armour, cost 0x33 | 0xe9 | hull space, 0xed used, 0xf1 cost (i32) |
| 0x35 | weapons, 8 × i16 | 0xf5 | "reset the help list" flag |
| 0x45 | counts, 8 × i16 | 0xf6 | weapons, 0xf8 specials (counts) |
| 0x55 | arcs, 8 × u8 | 0xfa | 0x28 bytes: a box's rows that fit |
| 0x5d | modifications, 8 × i16 | 0x122 | specials box first row, 0x123 thumb |

`Build_Initial_Design_Template_` (0x6a7c8) takes the slot's hull,
picture, weapons and specials (the first eight set bits) and the player's
best drive, computer, armour, fuel and shield. Missile and fighter mounts
without a forward arc get arc 0x10. `Update_Calculated_Design_Data_`
(0x6e8da): a damper field (special 7) removes the shield; costs
(`Design_Template_Costs_`, miniaturisation by `Tech_Level_`), spaces
(`Design_Template_Space_Requirements_`; the shield's level is
`_PLAYER_NUM`'s), totals (`Cost_Reduction_For_Govt_Type_`), the hull space
with application 0x59's 125%, the weapon and special counts, the printed
space and cost, and the help list when 0xf5 is set. Weapon costs and
spaces are recomputed only for slots with a weapon.

`Update_Player_Design_` (0x6ea48) writes name, size, shield, drive,
computer, armour, the special bits, the weapons (working count = count),
picture, cost (16 bits) and build date (low 16 bits of the stardate); the
combat speed with `_use_augmented_engines` cleared first (so without the
+5 of augmented engines) and the FTL speed. Type and builder stay.
`Build_Design_Template_` (the record the drawing and refit cost use)
writes shield 0 and leaves type, FTL, builder and working counts as stale
stack.

## Fields (Add_Design_Buttons_ 0x69e62)

Size buttons (0x76, y per size .. 0xe3) as a multi button field, or hidden
while refitting (flag 0xf4: `Print_Ship_Size_Error_Message_` E 0xd).
Weapon rows 0..7 at y 0xa9 + 0xd·i: none unless a weapon is loaded or
weapons are known (`Replacement_Exists_(3)`); − (0x37) only for counts
above 1, + (0x13) only while one more fits, else hidden fields; the row
(0x4d..0x22c, or 0x10..0x26f when empty). Special rows (0x11, 0x13f +
0xd·i) when specials are known. Picture arrows (0x11/0x5e, 0x52), the name
(continuous string input, 0x12, (0x1b − h)/2 + 0x11, 0x86 wide, 15
characters, style 4), the shield (0x1b5, 0x38..0x273, 0x5f) and computer
(0x1b5, 0x61..0x273, 0x7b) rows, CANCEL (Esc), CLEAR (L), BUILD (B, or a
dark picture when overfull), a debug field, the screen.

## Drawing (Print_Current_Design_ 0x6d29c)

Drive (name, speed, combat speed), armour (structure by
`Get_Structure_` with the player's *best* armour, armour points), shield
(none: E 0x57/0x58; else strength `Shield_Strength_` and points blocked),
computer, beam defence and missile evasion from
`Get_Design_Combat_Bonuses_` on the built record, the weapons table (name
plural for counts above 1, missiles with their ammo, count, damage by
`Design_Weapon_Damage_String_`, arc, cost and space per mount, at least 1;
the modifications and arcs are read from row i, not slot first+i), the
specials with their descriptions, the ship picture, hull space, cost
(`Printed_Design_Cost_`: a refit's cost) and space left. The hovered
shield, computer, weapon or special row flashes (`Set_Flashing_Colors_`:
the colours shifted by `_multi_colored_line_start` between −2 and 3); an
empty hovered row prints E 0x17f/0x180 or why nothing fits (E 0x18/0x1a,
0x19/0x1b). `Draw_Down_Buttons_` adds dull +/−, the dark BUILD and dark
size buttons.

## Boxes (DESBOX.C)

Each box draws the screen, darkens it (`Darken_Fill`) as its background,
runs its own Get_Input loop with an auto function and leaves with
`_current_screen` 3 and the template's 0xf5 set.

- Shield (E 0x52 without shields, 0x51 with a damper field) and computer
  (E 0x50) boxes, `Generic_Replacement_Box_` (0x6603e): six rows, unknown
  ones dim and without fields; a row replaces the item, CANCEL (Esc) keeps
  it; the full-screen field (also Esc, but CANCEL comes first) keeps the
  current item. `Print_Shield_Data_`/`Print_Computer_Data_`: the cost
  column is centred by the space header's width and the other way round
  (shields); the area field has the weapons box's width.
- Special systems, `Special_Systems_Box_` (0x68755): none and every known
  special, scrolling 10 rows (the position stays in the template); a row
  fits when its space fits and no fitted special excludes it; row 0 clears
  the slot (`Clear_Special_System_Slot_`); a special in another slot
  says E 0x41, an excluded one E 0x42/0x43 (with articles); all rows of a
  style fitting every special name in 0xa5 pixels.
- Weapons, `Weapons_Replacement_Box_` (0x69583): none and the known
  weapons the filters (BEAM B, MISSILE M, BOMB O, SPECIAL S; their status
  is DESBOX.C's initialised data, all on, and survives the box) let
  through, one tractor beam per design, sorted by research cost
  (`qsort_by_research_cost_`, dearest first); the rows show damage
  (modified on the chosen row by heavy mount/point defence/etc.), cost
  and space of one mount, "(%d)" a quotient that is always 0 or 1
  (0 / space). Choosing a row resets arcs (forward for beams, 0x10
  otherwise), rack 1 and the modifications; arcs (beams) or rack sizes
  (missiles, 2/5/10/15/20); modifications by `Valid_Modifications_`
  (heavy mount needs application 0x3b, the others the weapon's mask and
  miniaturisation) in two columns (`Get_Weapon_Mod_Field_XYs_`'s jump
  table picks the pictures), refusals by
  `Print_Weapon_Modification_Message_`; mod 1 and 2 exclude each other.
  ACCEPT (A) or a second click on the chosen row
  (`Update_Weapon_Replacement_`) puts the weapon in the slot, the count
  kept by space (old space / new space, 1..99); row 0 clears the slot.
  `Weapon_Choice_Valid_` puts an unset choice on row 0, so an empty slot
  opens on "No Weapon". `_valid_arcs_racks` is set and never cleared.

## Refit (COLREFIT.C)

The build queue's REFIT (r) needs fields 0x16 and 0x17 researched, then:

- `Colony_Refit_Popup_` (0xc187b): `Build_Ship_List_` takes the player's
  ships at the star (`_screen_data[0]`) with status < 3 and type 0
  (combat), up to 500, in ship order; 15 cells of 5 x 3 (`Draw_Ship2_`:
  the ship, and in red "NO STAR BASE" E 0x18b when `Colony_Can_Build_
  Product_(-(ship+100))` is 0, "CAPTURED" E 0xd8 for another builder,
  "LOKNAR" E 0x16a for picture 0x2c). A click: Loknar E 0x23f, no star base
  (cruisers and up without a shipyard building) E 0x23e, captured E 0x23d
  (`Help_`, the popup stays); an officer aboard asks E 0x212 and Yes sends
  him to the pool, but either answer takes the ship. "-"/"+" buttons and a
  scroll bar move by rows; the hovered cell pulses and shows the ship's
  paragraph with its name, officer and crew line.
- `Colony_Refit2_Popup_` (0xc20af): returns −2 (the design screen) at
  once when no design has the ship's hull size. Else the five named
  designs (bright for the ship's size, brighter hovered) and "Goto design
  screen" (E 0x320). A design of the size is copied into slot 5,
  `_screen_data[4]` = 5, −3: the build queue queues the refit at once
  (`Add_Refited_Ship_To_Queue_`). Another size: E 0x323. The design
  screen row: −2, slot 5 = the ship's design, the design screen in refit
  mode. Right clicks give E 0x324..0x32b text boxes.

## Quirks

- The size buttons set the hull space without application 0x59's 125%;
  the next recalculation adds it.
- `Get_Structure_` uses the player's best armour, not the design's.
- `Check_Weapon_Fields_` tests row i for an empty weapon, not slot
  first+i (the same while the counter stays 0).
- `Cost_Of_Design_` points `_design` at its own stack frame and leaves it
  there; the design screen allocates it again on each first call.
- The generic box leaves `_scroll_bar` pointing at its freed block.
- Weapon "(%d)" after a name is 0/space: 1 for rows that fit, else 0.
- Refit: `Add_Ship_List_Fields_` decides which cells get a field from the
  list's start, not the first row shown; the bar's top scales by the ship
  count and its bottom by the rounded rows; an officer index of 0 reads as
  none in the ship paragraph (`jg 0`); declining to free the officer still
  refits the ship.

## Not tested against the original

Right clicks inside the boxes and the refit popups' text boxes, the hovered refit cell's pulsing box and paragraph (the pointer
never rests on a cell), Loknar's ship, refit mode's dark size buttons
beyond des4's one BUILD, the debug field. The custom ship (−13) is
unreachable.
