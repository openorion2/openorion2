# Command points (screen 0x2b)

A click on the command points of the galaxy screen's panel (Main_Screen_,
hidden field 0x221,0x7a..0x265,0xc0, not while `skip_fields`) sets
`_current_screen = 0x2b`, `_return_screen = 0`. Screen_Control_ (0x106af)
runs Show_Command_Points_Screen_. Port: `internal/game/commandpoints.go`.

## Show_Command_Points_Screen_ (0x8bab9)

`Deactivate_Help_List_`, `Assign_Auto_Function_(Draw_Mini_Main_Screen_, 1)`,
`Clear_Fields_`, `Add_Hidden_Field_(0, 0, 0x27f, 0x1df, "\x1b", 0x29)`, the
mini map into both pages (Disable_Draw_All_Fields_, Set_Page_Off_, Fill_
black, Draw_Mini_Main_Screen_, _TOGGLE_PAGES_, Enable_Draw_All_Fields_),
`Show_Command_Points_(PLAYER_NUM)`, `_current_screen = _return_screen`.
No Clear_Fields_ or Deactivate_Auto_Function_ afterwards.

## Show_Command_Points_(p) (0xe2644)

`Compute_Player_Maintenance_(&player[p], buf)` with a 3000-byte stack
buffer, then it jumps into Show_Morale_'s tail: `Text_Box_(_command_summary_msg, buf)`.

## Compute_Player_Maintenance_'s text (0xe2000)

With a buffer the routine (otherwise ported in `tech_economy.go`) appends,
each sprintf_ advancing the pointer. Colcalc strings by their
Load_Colcalc_Strings_ index (42..59, RSTRING); formats in .data:
`%d %s\n` (0x179fe1), `%d : %s\n` (0x179fe8), the rule
`-------------------\n` (0x179ff1).

After the total is known:
1. `%d %s\n` with 5 and Colcalc 42 (base).
2. Star bases n: n = 1 → Colcalc 44 (1); n > 1 → Colcalc 43 (n, n).
   Battlestations: 46 (2) or 45 (2n, n). Star fortresses: 48 (3) or 47 (3n, n).
3. Communications (n ≠ 0): `%d : %s\n`, n and the application's name
   (`_technology_applications[0x5b|0xb0|0xb4]`).
4. Best officer (≠ 0): `%d : %s\n`, the points and the leader's name.
5. Warlord (≠ 0): `%d %s\n` with Colcalc 49; government 3's half: Colcalc 50.
6. `CommandTotal > 5`: `%d %s\n`, the total, Colcalc 51. Then the rule.

After the ship loop (owned, status ≤ 2, not just built; design type 0
counted by hull size, others as support):
7. Per hull size 0..8 with ships: one → Colcalc 52 (size+1, singular hull
   name 0x18003a); more → 53 (n×(size+1), n, plural 0x18003e).
8. Support ships: 54 (1) or 55 (n, n).
9. `%d %s\n`, CommandUsed and Colcalc 56 (used = 1) or 57; the rule.
10. CommandUsed = 0: nothing. ≤ total: `(%s 0 BC)\n` with Colcalc 58.
    Over: `(%s %d * %d + %d * %d : %d BC)\n` with Colcalc 58, 0,
    CommandTotal, the multiplier, the excess and the penalty (the literal
    0 and the total make "0 * 22 + 10 * 2 : 20 BC").
