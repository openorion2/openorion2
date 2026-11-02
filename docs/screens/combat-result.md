# Strategic combat result box (Russ_Combat_)

With strategic combat (settings 0xd8) or two non-humans, `Russ_Combat_`
(0xe7343) resolves a battle with `Strategic_Combat_` (0x40148). When a human
takes part it then shows the result. Port: `russCombat`
(`internal/game/turn_combatdo.go`), `buildEndOfCombatMessage`
(`internal/game/combat_endmsg.go`).

## Russ_Combat_ after Strategic_Combat_ (0xe74a5)

`local` ([ebp-8]) is game type ≠ 2 and ≠ 3. If `local` or the attacker is
PLAYER_NUM, and the defender (< 8) or the attacker (< 8) is human:
`Update_Ship_Icons_In_Between_Turns_`, `Message_Box_Titled_(res+0x281,
res+0x2e5)` (0x3fdaa: `Do_Text_Box_(title, text, 0)`), again
`Update_Ship_Icons_In_Between_Turns_`, `_messed_up_fleet_palette = 1`. The
common tail calls Update_Ship_Icons_In_Between_Turns_ once more.

`_messed_up_fleet_palette` (0x183578) is also set after
`Player_Attacked_Popup_`, the amoeba's flic and other bombing sites;
`Search_For_Battles_` (0xea1ff), after `Process_Retreating_Ships_`, clears it
and calls `Set_Reload_Fleet_Box_Palette_Flag_` (0x8a159: `_mainscr_palette =
0x65`, `_ship_icon_player_id = -1`). The port sets it in Russ_Combat_ and
after the amoeba's flic; Get_Local_Human_Target_ (0xe7d87),
Bomb_Results_Popups_ (network) and Do_Attacker_Beat_Colony_Stuff_'s
0xe8cf3/0xe92fd are not ported as setters (the turn's end sets the same
palette flag anyway).

## Build_End_Of_Combat_Message_ (0x40d40)

Called by Strategic_Combat_ (0x40b93) after the monster updates, with the
title buffer (EAX), the text buffer (EDX), the star (EBX), the colony
defences loaded before the battle (ECX, `[ebp+0x2e]` = `_colony_defense_flags`
after Qload_Ships_ of the defenders, 0 for an Artemis net win) and the
result. It reads the qcombat records (0x21 bytes at `screen_seg`
0x192844, from 1 to `n`).

1. Records with hull size ≤ 5 count per size: attacker (owner = attacker)
   or defender, alive (+6 = 0) or lost. Records with size > 5 (satellites,
   monsters) only keep the last one's size and whether it is alive. Totals
   are alive + lost.
2. Title: Ken 0x62 ("Combat at ") + "Antares" when star = NUM_STARS, the
   star's name with no colony, else `Get_Planet_Name_`.
3. Names: player race name (+0x15) for < 8, else `Get_Non_Player_Name_`
   (0x40cc3, jump table 0x40ca7: 8 Antaran, 9 Guardian, 10 Amoeba,
   11 Crystal, 12 Dragon, 13 Eel, 14 Hydra; another owner copies an
   uninitialised stack buffer).
4. Text, with the `\x07X<n>.` column codes of .data 0x178dc9..0x178e10:
   `X35.` attacker `X255.` defender `\n`, Ken 99 `X+25.` Ken 100 `X227.`
   Ken 99 `X+21.` Ken 100 `\n`; per size with ships: `X18.` start `X80.`
   killed `X120.` the plural hull name (0x18003e) `X250.` start `X312.`
   killed `\n` ("-" for 0, itoa otherwise).
5. The last record of size > 5 (a star base, battle station or fortress): `X120.` singular hull name (0x18003a) `X250.1X312.`
   "-" if alive else "1".
6. For defence bits 1, 2, 8, 4 of ECX: `X120.` the building name (missile
   base 0x1a, ground batteries 0x1b, fighter garrison 0x2f, stellar
   converter 0x2a: pointers 0x17ed2b, 0x17ed3e, 0x17eeba, 0x17ee5b)
   `X250.1X312.` then "-" if `_colony_defense_flags` still has the bit,
   else "1".
7. `\n\n\x1a2` and Ken 0x65 (result ≥ 1, attacker won), 0x66 (< 0) or
   0x67 (0).

The port builds the strings only with a display attached (`g.e != nil`).
