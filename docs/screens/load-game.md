# Load_Game_ (MOX\OTHERS\LOADSAVE.C)

`Load_Game_(slot)` (0x10e2f) reads `SAVE<slot+1>.GAM` and returns 1 on success.
Ported in `internal/game/state.go` (`loadGame`), with the settings of
`main2_` in `internal/game/settings.go`.

## Flow

1. `fopen` fails: a debug print and `hgetch`, return 0.
2. The version dword is not 0xe0: `Warning_Box_` with the language's
   "SAVE%d.GAM is invalid" message (language = settings +0xd2), then
   `Set_Page_Off` and a black off page, return 0.
3. The blocks are read in file order (`docs/formats/save.md`). The
   description goes to `_save_game_description[min(slot, 9)]`. Settings and
   the random seed go to locals. `NUM_NEBULAS` is zeroed if
   `_nebula_screen_seg` is null (never, in practice).
4. `Set_Defaults_`: `cur_map_x/y = 0`, `current_screen = 0x27`,
   `council_winner = -1`, `Ken_Init_` and `Eric_Defaults_` (both empty),
   `Harold_Defaults_` (main screen globals; among them `star_bg_index = 2`
   and `cur_map_scale = max_map_scale`, so the saved values of these and of
   the map position are dropped), `Bill_Default_` (the paragraph system;
   it tests settings +0xd4 **before** the save's settings are applied), and
   `global_esc = 0`.
5. `Set_Random_Seed(saved seed)`.
6. Caches: `Cache_Player_Scientist_Anim_`, `Load_Nebula_Pictures_` when there
   are nebulas, `Update_Ship_Icons_In_Between_Turns_`,
   `Reload_Fleet_Box_Data_` if box 2 is selected, `Rebuild_WFI_List_`.
7. Settings +0x02 and +0xd3..+0xdd (universe settings and seed) are copied
   from the save into `_settings`.
8. `Confirm_Planet_And_Star_Index_Link_` (prints on a broken link; its
   duplicate-orbit check indexes by the planet's orbit but marks by the
   loop index, so it is weaker than it looks).
9. `Update_Star_Owners_`: `star.owner = Star_Owner_(star, PLAYER_NUM)`.
10. `Load_Leader_Titles_`: for each of the 67 leaders, `strcpy` of the title
    (record +0xf) from `HERODATA.LBX` entry 0 (`HERODAT[GFSI].LBX` for the
    other languages).
11. For all 8 players: a non-zero u16 at +0x2e moves to the u32 at +0x1eb.
12. For p, j < NUM_PLAYERS: `player[p].DelayedOrder[j] = -1` (+0x897+j).

## Star_Owner_ and Colony_Owners_

`Star_Owner_(star, p)`: every colony counts if player p has a leader with
Galactic Lore (+0xe75) or the Omniscient pick (+0x8ba); otherwise
`Colony_Owners_` counts a colony only if its owner is `PLAYER_NUM` or a
player `PLAYER_NUM` has contact with (+0x584). Note that the contact test
always uses `PLAYER_NUM`, not p. The result is `PLAYER_NUM` if it has a
colony there, else the player with strictly the most colonies (first wins
ties), else -1. Recomputing the owners of all ten saves reproduces the
saved values exactly (`state_test.go`).

## Main menu after the load

Continue is `Load_Game_(9)`. On success: game type 2 or 3 shows message
0xcf in a `Warning_Box_` and stays on screen 10; type 1 goes to screen 0x11,
otherwise 0x27. On failure screen 10. Load Game does the same after
`Do_Mainmenu_Load_Screen_` with message 0xd0.

## Random_ (random.c)

`_random_seed = seed * 0x41c64e6d + 0x3039` repeated until the seed is below
`(0xffffffff / n) * n`; the result is `seed / (0xffffffff / n) + 1`, in
1..n. The division happens before the n == 0 test, so `Random_(0)` faults.
`Randomize` seeds from `time(NULL)`. Ported in `internal/random`.
