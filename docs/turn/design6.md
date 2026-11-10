# The sixth ship design (community patch 1.51)

In 1.40b23 design slot 5 of the player record (save offset 0x515 of the
player, `Designs[5]`) is a scratch slot. The support ship loaders
(`Load_Colony_Ship_Design_` 0x564cd, `Load_Outpost_Ship_Design_` 0x56586,
`Load_Transport_Ship_Design_` 0x5663e) write it before `Create_Ship_`
builds from it, refits copy a design into it, the strategic build screen
loads the doom star into it, and `Init_Ship_Designs_` clears it for
humans. Patch 1.51 turns it into a sixth design. Read from ORION150.EXE
(the tools: `build/p151/README.txt`); code: `internal/game/design6.go` and
the places below. Player's view: `docs/manual/ships.md`.

## The copy aside

The patch keeps one copy of a design (0x99 bytes at its 0x400f06) and a
flag (0x400f6a), the same for all players (`spareDesign`, `g.spare`):

- The three support loaders first copy the design they are given aside
  and set the flag (`keepSpare`; hooks at 0x564e0, 0x56599, 0x5664e
  replacing their `Clear_Structure_`).
- The refits copy slot 5 aside before overwriting it: the build screen's
  refit to a new design (`Evaluate_Input_` +0x456, buildq.go) and the
  refit popup's pick of a design (`Evaluate2_Input_` +0xb1, colrefit.go).
- `Create_Ship_`, once the design is copied into the ship (+0xe1) and on
  the no-ship-left exit (+0x229, the 500 ships limit), copies it back into
  the player's slot 5 and clears the flag (`restoreSpare`).
- `Add_Refited_Ship_To_Queue_` copies it back after the refitted ship took
  the design (+0x173). Port: also when the queue was full; the patch then
  copies through a pointer it never set.
- `Evaluate_Input_` of the build screen, back from the design screen
  without a refit to add, copies it back (+0x5f).

Every caller in the game loads a support design and calls `Create_Ship_`
at once, so the flag never outlives a ship's creation.

## New game

`Init_Ship_Designs_` runs its hull loop over slots 1..5 (1.40b23: 1..4):
the human's sixth design is the next hull up, and the human's slot is no
longer cleared. An AI still gets the doom star in slot 5 after the loop,
so it pays one more `Auto_Design_Ship_` (random numbers) first. A new game
from the same seed therefore goes differently from 1.40b23's.

## Loading a game

After `Load_Game_` (the patch's save reader, called at +0x7bf), every
player whose slot 5 has a non-zero design type (a support ship: what
1.40b23 left there) gets `Load_Doom_Star_Ship_Design_` into it
(`fixSpareDesigns`, file loads only, not a network game's turn-start
snapshot). Under strategic combat the doom star is designed by
`Auto_Strategic_Design_Ship_`, which leaves the type, so such a slot is
redone at every load, as in the patch.

`Load_Doom_Star_Ship_Design_` itself designs a strategic doom star under
strategic combat (hook at 0x5646b; 1.40b23 always made the tactical one).

## Lists

- `Calculate_Military_Array_` offers six designs. Under strategic combat
  a design of titan size or larger is offered when the titan or the doom
  star hull is known (1.40b23: a titan design needed the titan hull), and
  the doom star item (−16) is gone: slot 5 is the doom star design.
- The refit popup (`Add2_Fields_`, `Draw_Ship_D_List_`, `Evaluate2_Input_`,
  `Colony_Refit2_Popup_`) shows six designs; its design screen row moves
  to y 0xcb..0xe0 (1.40b23: 0xcd..0xe3).
- Build lists' `design6` is −55.
- `-control`'s queue codes (`queueCodeSlot`) look at six designs; a
  support ship answers `queueSupportSlot`.

## Tests

`design6_test.go`: the new game's six designs; a support ship and the 500
ships limit leave the sixth design; a refit gives it back; the load fix;
the military list under both combat modes and `design6` in a list.
