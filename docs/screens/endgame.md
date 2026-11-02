# The end of the game: Next_Turn_'s checks, the score screen, the Hall of Fame

Addresses are ELF. Go: `internal/game/turn.go` (`checkEndOfGame`),
`hiscore.go`, `hof.go`.

## Next_Turn_ (0x13870) after Next_Turn_Calc_

Not in network games or an all-AI game:

1. No living human (`N_Living_Human_Players_`): fade out, black page,
   flip, copy to the back page, stencil refresh, `Play_Loose_Flic_`,
   `End_Of_Game_Hi_Score_(PLAYER_NUM)`, screen 10.
2. At most one living player: `Get_Winner_` (the council winner, else the
   last player who took Antares or is alive), fade out,
   `Play_Normal_Win_Flic_`, `End_Of_Game_Hi_Score_(winner)`.
3. `_council_winner` set: the human → `End_Of_Game_Hi_Score_(PLAYER_NUM)`;
   else the loss flic and the same.
4. Else every player (last first) with +0x1f0 (took Antares): auto off,
   `Play_Antaran_Win_Flic_`, `End_Of_Game_Hi_Score_(p)`, screen 10.

Always: every player's TurnDone (+0x2d) = 0, `_old_system_index` = −1.
The flics are Smacker videos (`Play_Cinematic_`): not ported.

## End_Of_Game_Hi_Score_(p) (0x9f712)

`Hi_Score_Screen_(p)` gives the score. The HOF (as `Read_HOF_File_` left it,
lowest first) is sorted again (`Sort_HOF_Scores_`); the rank is the entry
of the last score ≤ the player's (−1 none). With a rank: a cheater
(+0xe69) gets `Get_LBX_Message_(0x17)` and no entry; else a warning box,
H_Message 0x27 when the score is at least the best, 0x28 otherwise (the
emperor's name); the entries below the rank move down one (strcpy, so the
name tails stay) and the player goes in: name, score, race, difficulty
(settings +0xd4), race name. Written to HOF.M2 unless cheated, then
`Hall_Of_Fame_Screen_(rank)`; screen 10, previous screen 0xe.

## HOF.M2 (0x1c4 bytes, memory 0x19c5fc)

| Offset | Field |
|---|---|
| 0 | version 0x82 |
| 2 | names [10][20] |
| 0xca | scores s16[10] |
| 0xde | races s16[10] |
| 0xf2 | difficulty u8[10] |
| 0xfc | race names [10][20] |

`Read_HOF_File_` (0x9f4ad) reads it and reorders the entries by
ascending score (`qsort_` of (score, index) pairs, ties by index) through
`Move_Sorted_HOF_Data_`, which builds the copy on its stack with strcpy:
the bytes after each name's NUL are stale stack, and they reach the file
on the next write. The port leaves them zero; tests compare the strings
up to their NUL.

`Create_Default_HOF_File_` (0x9f540): HOF.M2 opened for writing first; all
zero, version; a race permutation (10 swaps of `Random_(11)`+1 pairs over
0..12); 8 swaps of two entries 1..8 of the default names table 0x182318 (in
place: the table stays shuffled for the rest of the run); entry k: score
(`Random_(n+1)`−1)·15 + `Random_(5)`·10 + base, n from 10 down, base from
1500 down by 125; difficulty k % 5; name k; race name: race 5's for k < 2,
else race perm[k]'s.

## Hi_Score_Screen_(p) (0x9ea3b)

HOF read (or the defaults); auto `Draw_Hi_Score_Screen_`; SCORE.LBX 0 (the
page), 1 (the eliminated mark), 2..14 (race portraits); +0x68 = p;
`Calculate_Hi_Scores_` (`score.md`); colours (`Font_Colors2_(4, 0xd0,
0xd2)`); a full-screen Esc field; with the seed saved and put back,
`Set_Hi_Score_Pop_Icon_Ids_` (p's colonists listed by colony, shuffled with
`Random_(n)` pairs, then per race ⌊(count·160/n + 5)/10⌋ icons of the first
unused colonists, 16 at most; a colonist's race is `players[c & 0xf]
+0x25` (0x9fda3), and natives and androids (8, 9) index past the eight
player records: the original reads the next heap block, the colonies
(players and colonies are consecutive `Allocate_Space_` mallocs, a 4-byte
malloc tag and a 12-byte header between them), so i ≥ 8 reads colony
byte (i−8)·0xea9 + 0x15; the port does the same (`playerRaceAnyIndex`,
which panicked before; win2's capture agrees) and `Build_Captured_Colonists_Array_` (up to
16 races by `Get_Weighted_Choice_Short_`: 2 for a war declared, 5 for a
race eliminated, sorted); `Fade_Into_Screen_` (the clear flag +0x6b,
palette 0xc). Until Esc: draw, flip. Returns +0xae.

`Draw_Hi_Score_Screen_`: the page; lines (`Print_Score_String_`: the text,
a dotted leader in `Font_Colors2_(4, 0xd0, 0xd1)`, the number right at
0x1f3) for time (at y 0x8b), population (with `Draw_Pop_` icons),
eliminations (the portraits with the date and the emperor, framed),
captured colonists (race icons), technology, Orion, Antares, the council
(when won); then a triple rule and the total ("(%d x %s) %d" with the
modifier when it is not 100%).

## Hall_Of_Fame_Screen_(flash) (0x9f286)

`_entry_to_flash` = flash; the five difficulty names (E strings 0x221,
0x118, 0xbe, 0x13e, 0x148); the HOF read, recreated when missing or not
version 0x82; cancel off, help off; read again; SCORE.LBX 15 (the hall), 16
(the cursor picture); `Init_Hall_Of_Fame_Screen_`: rows by score, highest
first (`qsort_`, ties by index, higher first), hidden row fields, Esc,
the multi hot key "C", black, palette 0xc, the screen, fade in. Loop:
`Get_Input_`, `Scan_Input_` (the hover test is dead: it compares with a
value never updated); Esc leaves; C → `Create_Default_HOF_File_`, flash −1,
init again; draw, flip. Leaving: fade out, the return screen.

`Draw_Hall_Of_Fame_Screen_`: the smallest fitted styles of all race names
(width 0x43) and difficulties (0x56); on the first draw black and the hall;
each row at y 0x87 + 0x1d·k: name (0xa4), race (0x124), difficulty (0x16c)
in windows, the score right at 0x1f4; the flashed entry's colours shifted
by +0xb8, which swings −1..6 every 6 frames; the cursor picture at the
pointer (clamped to 0x27c, 0x1dc) after the first draw. Other languages
than English use the fitted styles.
