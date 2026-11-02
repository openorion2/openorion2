# Score and the SCORE cheat

`Do_Calculate_Hi_Score_` (0x9db51) and the SCORE branch of
`Check_Hot_Keys_` (0x82a68). Go: `internal/game/score.go`,
`score_test.go`.

## SCORE

Typing Alt+SCORE on the galaxy screen calls `Do_Calculate_Hi_Score_` for
`_PLAYER_NUM` and shows `"Current score for %s is %d"` (literal at
0x179731, `%s` the emperor name at player+1) in a `User_Box_` over the main
screen. Unlike the other cheats it marks nothing as cheated.

## Do_Calculate_Hi_Score_

Pushes `_global_data_seg`, takes its first block (0x125 bytes) as `_hi`,
runs `Calculate_Hi_Scores_` (0x9d977) and returns the word at `_hi+0xae`.

| `_hi` | Value |
|---|---|
| +0x76 | 50 per player eliminated (`Get_Elimination_Score_`, Eliminations[k] != 0); count at +0x88 |
| +0xa8 | 3 per researched field + 5 per non-zero hyper level (`Get_Technology_`) |
| +0x69 | 250 if Antares captured (player +0x1f0) |
| +0xa2 | 100 if Orion captured (+0x1ef) |
| +0xb0 | 100 if the council was won (+0x1f1) |
| +0x8c, +0x6c | captured colonists (+0x202); score = n * 2 / (settings 0xd6 + 1) |
| +0xa0, +0xa4 | population: pop count of every colony the player owns, outposts included |
| +0xac | time bonus ((settings 0xd6 + 1) * 20 + 80) * players - (stardate - 35000) as shorts; 0 without population |
| +0xaa | the sum of the above (16 bits) |
| +0xb6 | `Player_Score_Modifier_(_hi[0x68])` |
| +0xae | (sum * modifier + 50) / 100 |

`Player_Score_Modifier_` (0x58f4a, player q): loads RACESTUF.LBX entry 6
(`_ability_costs`), turns the player's picks into the race screen's buttons
and toggles (`Convert_Player_To_Flags_EM_` 0x5b4cd: for ability rows 1..9,
which of the three `_special_bonus` values (0x17d1fa, 3 per row) the pick
equals; the government group is picks[0] / 2; picks 10..30 become 22
toggles, the rich/poor home world pick split into two, -1 being the second),
and counts the picks left (`Calculate_Picks_` 0x59053): start from the byte
at 0x178513 (10; a patch helper loads it), subtract `costs[row*3 + button -
1]`, `costs[0x1b + government group]` and `costs[0x1f + t]` for each toggle
equal to 1. Application 0x3e adds 4. Modifier = 100 + 10 * picks if
positive, else 100. The race screen globals it writes (`_picks`,
`_negative_picks`, `_button_variable`, `_option_toggle`) are not kept by the
port.

### The modifier's player

`Do_Calculate_Hi_Score_` never sets `_hi+0x68`; the hi-score screens set it
before calling `Calculate_Hi_Scores_` themselves. The block is the first
of `_global_data_seg`, which on the galaxy screen was last taken by
`Find_Ship_Stacks_` (0x5d41e) for its per-ship flags (1 for ships with
status 3 or more). So the byte is ship 0x68's flag: player 0 or 1. With
fewer than 0x69 ships it is stale; the port takes 0.

## Checked

SAVE1, player 0: 49 fields (147), 118 colonists, 6 captured colonists on a
huge galaxy (3), time bonus 160 * 8 - 362 = 918, no picks left: 1186. The
ten saves give 1186, 1193, 1181, 217, 1307, 1367, 740, 735, 738, 1293.
SAVE10's player 7 has 2 picks left (modifier 120); all others 100.
