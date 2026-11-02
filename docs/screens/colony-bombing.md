# Colony bombing summary (Colony_Bombing_Screen_)

What a human sees when an enemy bombs one of their colonies. Port:
`internal/game/colony_bombing.go`.

## Call sites (Do_Attacker_Beat_Colony_Stuff_ and Bomb_Results_Popups_)

- 0xe8974: a monster beat the colony and bombed it; human owner, not a
  remote network player (settings+0xd8 with game type 2/3 and another
  machine's player skips), `hit (+0x41) > 0`. After the amoeba's flic and
  its climate change. Port: `turn_combatcolony.go` (attacker ≥ 8 branch).
- 0xe917c: an AI bombed the colony (aiBeatColony), same condition, after
  Apply_Bio_Weapon_Penalty_ and before Change_Relations_.
- 0xe863b: Bomb_Results_Popups_, only reached from Client_Combat_Loop_
  (network), not ported.

## Colony_Bombing_Screen_(hit) (0xb4d02)

Copies the 0x49-byte hit record to the stack (`_bomb_ptr` 0x19deb4 points at
the copy, 0x19deb8 at the original), `Do_Summary_String_Popup_`,
`Set_Colony_Globals_(hit.colony)` (0xbb16a: `_colony_handle`,
`_planet_handle`, `_star_handle`, `_orbit_n`), then
`_current_screen = _return_screen`, in the middle of the turn. The port
applies the globals and the screen also without a display (TestTurnSteps).

The routines BOMBING.C has next to it — `Draw_Colony_Bombing_Screen_`
(0xb4800), `Load_Bombing_Anims_` (0xb435a), `Do_Bombs_` (0xb46cf),
`Do_Bomb_` (0xb4606) — have no caller and no pointer to them in the image
(the dwords appear only in the symbol table): dead code, not ported.

## Do_Summary_String_Popup_ (0xb494b)

`Set_Colony_Font_To_(4)`; a 4000-byte buffer:
1. `sprintf(E 0x33, Get_Planet_Name_(colony.planet), Race_Name_(hit.attacker))`.
2. `+0x0c` (pop killed) ≠ 0: `E 0x1d` with it; the line counter becomes 2.
3. `+0x08` (marines) ≠ 0: `E 0x18`; `+0x0a` (armour) ≠ 0: `E 0x11`;
   `+0x05` ≠ 0: `E 0x1b` with `+0x3f` (biological pop killed). Each counts a line.
4. Buildings 0..0x30 with `+0x0e+i` set, while fewer than 15 lines:
   `E 0x37` with `_buildings[i].name` (Italian, setting 0xd2 = 4: the
   literal "Edificio distrutto: %s", no newline).
5. More than 15 lines: `E 0x17a` (unreachable: the loop stops at 15).
6. `+0x42` (colony emptied): `E 0x37` with the planet's name again
   (Italian: "Perdita di:%s").
7. `Text_Box_(E 0x1f2, buffer)`.
