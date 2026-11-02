# The AI data (AIDATA.C)

`Compute_AI_Data_` (0xd3d34) builds the AI's picture of the turn at the
start of `Next_Turn_Calc_` (step 3); the AI steps up to `Do_Surrenders_` read
it, and `Deallocate_AI_Data_` (step 23) pops it. It is not saved. Go:
`internal/game/turn_aidata.go` (the routine and the block table),
`turn_aistrength.go`, `turn_aiturns.go`, `turn_aiplanets.go`,
`turn_aitech.go`, `turn_aistage.go`; the arena itself in `aiarena.go`.

## The arena

`Push_Block_(screen_seg)` (arena pointer at 0x192ed4), then 25
`Allocate_Next_Block_` calls. Each block is a 12-byte header {0, requested
size, 0} and the data, rounded up to 4 bytes; the allocator does not clear
it. The sizes are constants, so the layout is fixed: 0xb88fc bytes from the
first header. The port keeps the same bytes (`aiArena.mem`) and reaches the
blocks through `aiBlockOff` (by block) or `aiBlockOfGlobal` (by the address
of the pointer global). All writes go through raw arena offsets, so an index
that runs past its block lands in the neighbour as in the original; past the
whole arena, reads give 0 and writes are dropped.

| # | Global | Pointer name | Size | Data at | Layout | Filled by |
|---|---|---|---|---|---|---|
| 0 | 0x1aa1dc | `_ship_strengths` | 0x1f40 | 0xc | [500 ship][8 player] u16 | `Ship_Strength_Vs_Ships_(k, p)` for ships with status < 3, combat design, p ≠ owner, p alive |
| 1 | 0x1aa1f4 | `_ship_strengths_in_nebula` | 0x1f40 | 0x1f58 | same | `Ship_Strength_Vs_Ships_In_Nebula_` (shield 0) |
| 2 | 0x1aa1fc | `_ship_bomb_strengths` | 0xfa0 | 0x3ea4 | [500][4] u16 | `Ship_Strength_Vs_Planetary_Shield_`, per planetary shield level (0x5eae1: 2, 7, 12, 22) |
| 3 | 0x1aa1bc | `_planet_stage_weights` | 0x1680 | 0x4e50 | [360 planet][8] u16 | base planet value of each colony for its owner |
| 4 | 0x1aa210 | `_base_star_stage_weights` | 0x900 | 0x64dc | [72 star][8] u32 | best own planet's weight + 1/16 of own planets within 2 turns |
| 5 | 0x1aa1e0 | `_star_stage_weights` | 0x900 | 0x6de8 | [72][8] u32 | ⌈threat/8⌉ · ⌈base/8⌉; `Compute_Defensive_Stage_Points_` zeroes stars covered by a stage point |
| 6 | 0x1aa1e8 | `_n_turns_from_to` | 0xa200 | 0x76f4 | [72 from][72 to][8] u8 | `Player_N_Turns_From_Star_To_Star_` for star pairs both in p's extended range, else 0xff |
| 7 | 0x1aa1b8 | `_n_turns_from_to_intermediate` | 0x28800 | 0x11900 | [72][72][8] × {turns, via, turns', via'} | memo of `Player_N_Turns_From_Star_To_Star_With_Intermediate_`; 0xff = not yet computed, via 0x48 = none; filled lazily here and by later AI steps |
| 8 | 0x1aa1b4 | `_ship_n_turns_to` | 0x8ca0 | 0x3a10c | [500][72] u8 | ships in transit: ETA to their destination, and `Ships_Try_To_Move_To_` turns to every other extended-range star when they can be ordered; else 0xff |
| 9 | 0x1aa1c0 | `_ship_n_turns_to_intermediate` | 0x11940 | 0x42db8 | [500][72] × 2 | 0xff only; later AI steps fill it |
| 10 | 0x1aa208 | `_possible_ship_strengths` | 0x2d000 | 0x54704 | [72 target][8 owner][8 vs][10 turns] u32 | strength of the owner's active combat ships that can be at the target within that many turns (with intermediate stop) |
| 11 | 0x1aa1ec | `_ai_colony_info` | 0x6d6 | 0x81710 | [250][7] u8 | 0 dominant player (`Colony_Dominant_Player_`), 1 its `Colony_Race_Pop_Limit_`, 2 not both lithovore, 3 0, 4 dominant not tolerant, 5 enemy ships arriving within 9 turns, 6 colonists with bit 10 (conquered) |
| 12 | 0x1aa204 | `_ai_player_info` | 0x1d0 | 0x81df4 | [8][0x3a] | +0 colonies, +2 most colonies of anyone − own, +4 blockaded colonies, +6 colony stars, +8 blockaded colony stars, +0xa contacts, +0xc players met or sharing a star in extended range, +0xe..+0x15 that per player, +0x1a [8] u32 own ship strength against each player (all i16 but the last) |
| 13 | 0x1aa1f8 | `_ai_star_info` | 0xdc8 | 0x81fd0 | [72][0x31] | +0 planets; per player p at +1+6p: +0 players in normal range with treaty 1..3, +1 neutral, +2 at war (4..6), +3 only in extended range (all only for stars p has a colony at), +4 p's colonies within p's fuel range (via wormholes too), +5 enemy ships arriving within 9 turns (AI players in extended range) |
| 14 | 0x1aa214 | `_star_in_range_of_player` | 0x48 | 0x82da4 | [72] mask | colony within 30·fuel range parsecs (squared distance), line not through a black hole |
| 15 | 0x1aa200 | `_star_in_extended_range_of_player` | 0x48 | 0x82df8 | [72] mask | same within 45·fuel range |
| 16 | 0x1aa1f0 | `_enroute_ship_strengths` | 0x2d000 | 0x82e4c | [72 star][8 owner][8 vs][10 eta] u32 | strength at or arriving within eta turns (cumulative), nebula strength if the star is in a nebula |
| 17 | 0x1aa1e4 | `_enroute_ships` | 0x1680 | 0xafe58 | [72][8][10] u8 | any ship of the owner there within eta turns (cumulative) |
| 18 | 0x1aa1d8 | `_enroute_human_ships` | 0x2d0 | 0xb14e4 | [72][10] u8 | any human ship |
| 19 | 0x1aa20c | `_enroute_nonstealthy_ships` | 0x240 | 0xb17c0 | [72][8] u8 | a ship without stealth equipment of an owner without the stealthy race pick |
| 20 | 0x1aa1d4 | `_all_ships` | 0x3e8 | 0xb1a0c | [500] i16 | active combat ships of the players (status < 3, AI mission ≠ 4), sorted by `sort_ships_`; count in `_n_all_ships` (0x1aa218) |
| 21 | 0x1aa1c8 | `_i_all_combat_ships` | 0x490 | 0xb1e00 | [8][73] i16 | first `_all_ships` index per owner and star; ships elsewhere at index NUM_STARS; -1 none |
| 22 | 0x1aa1d0 | `_aitech` | 0xa8 | 0xb229c | [8][0x15] | `Compute_AI_Tech_Info_`, below |
| 23 | 0x1aa1c4 | `_airefit` | 0x5dd0 | 0xb2350 | [8][0xbba]: i16 n, [500] {i16 ship, u32 value} | ships at a star wanting a refit, sorted by value ascending (`airefit_cmp_`) |
| 24 | 0x1aa1cc | `_ship_refit_desirabilities` | 0x7d0 | 0xb812c | [500] u32 | `Refit_Desirability_` of ships at a star |

`_aitech`, 0x15 bytes per player: +0 best ship shield, +2 armour, +4
computer, +6 fuel cells, +7 planetary shield, +0xb drive (`Player_Best_*_`,
0 when none), +0xc / +0xe the best beam (weapons 1..0xd) / missile
(0xe..0x14) by field cost / 100, +0x10 and +0x11 (i16) ground combat from
`Compute_Player_Ground_Combat_Bonuses_`, +0x14 no free planet left
(`Best_Uncolonized_Undestined_Planet_` = -1, unless application 0xb8). The
odd bytes collect how far behind the others the player is: for every other
living player the lead in shield (+1), armour (+3), computer (+5), beams
(+0xd), missiles (+0xf), ×3 with contact, and `Random(2)-1` on a tie; the
planetary shield lead with contact (+8); with contact +0xa counts and +0x13
the ground lead. Objective 1 adds 1 to +1 and +5, objective 3 to +5, and
personality 2 to both, once per player of the loop (the player itself and
eliminated ones included). These ties are all of the step's draws.

## Outside the arena

Three AIDATA.C statics are rewritten too and are not in the dump:
`_g_base_planet_values` (0x1a8b34, pointer `_base_planet_values` 0x183554)
and `_g_contextual_planet_values` (0x1a74b4, pointer 0x183558), both
[8 player][360 planet] u16, and `_n_all_ships` (0x1aa218). The port keeps
them in `aiArena`. They lie in one range with the block pointers,
0x1a74b4..0x1aa21a; dumping that too would let later AI steps start from the
original's values.

Base value: for planets in the player's extended range,
`Uncolonized_Planet_Worth_To_Player_` (max population, minerals, food,
specials, climate, gravity against the race picks, /64, ≤ 0xffff) or
`Colony_Worth_To_Player_` (production of the dominant race, growth,
climate, gravity, government with building 9, specials, /64; from
difficulty 2 at least the proximity worth; outposts only
`Proximity_Worth_To_Player_`). Contextual value: the base value adjusted by
the star's other planets (1/8 of the free ones; an uncolonised planet next
to own colonies scaled by (size+1)/10 and divided by enemy colonies + 2; a
colony plus 1/4 of the own ones).

## Order and save state

After the blocks: ship strengths and the en-route tables; fleet strength
(`player+0x5ec`, the sum of `_ship_strengths` by owner); cumulative
en-route tables; star ranges; travel tables; colony, star and player info
with `Compute_Star_Player_Range_Info_` between the last two; base and
contextual planet values; `Compute_AI_Tech_Info_`; refit tables and the
sorted ship list; possible strengths; stage weights;
`Compute_Defensive_Stage_Points_` per player (`player+0x59e` staging star,
`star+0x41` bits); pending surrenders (`star+0x67`); every star's
`special_popup_pending` (+0x34) cleared.

Other saved state it can change: `Update_Strategic_Ship_` refits every
rated ship when settings byte 0xd8 is set; `Delete_Already_Assigned_Planets_`
sets an AI colony's `Queue[0]` from 0xb (colony ship) to -2 when no free
planet is left at its star.

## Bytes the original never writes

`checkAIArena` skips them (`aiArena.staleRanges`):

- `_all_ships` past `_n_all_ships`: arena 0xb1a0c + 2·n .. 0xb1df4.
- the padding after `_ai_colony_info`: 0x81de6..0x81de8.

The dumps of s6, s7, s8 and t1tribute hold the same bytes there: `11 0e` in
the padding, and in `_all_ships` the bytes of MAINMENU.LBX entry 0 from
offset 0x199b8 + 2·n on, left in `screen_seg` by the main menu. In a game that ran more turns the previous turn's AI data
would be there. The port leaves them zero.

## Not reached by the traced turns

Strategic combat (settings 0xd8: computer bonus, `Update_Strategic_Ship_`,
the hull-based bomb strengths); monster owners in `Get_Best_Ship_Shield_Value_`,
`Best_Computer_`, `Best_Bomb_Weapon_`, `Best_Warp_Drive_`,
`Best_Point_Defense_Weapon_`; ships whose star resolves to -1 (the original
then reads `star[-1]`, the port reads a zero star); a non-eliminated player
without colonies in `Compute_Defensive_Stage_Points_` (the original sets a
bit before the star array; dropped); population picks other than −50, 0,
+50, +100 in `Colony_Worth_To_Player_` (the original multiplies by a stale
stack word; the port leaves the value unscaled); the
`Delete_Already_Assigned_Planets_` queue cancel.

## Later AI steps

Steps 9–14 read and write the arena raw (memo tables 7 and 9 fill lazily,
`Move_To_Stage_Points_` adds to block 5, refits remove `_airefit` entries,
interceptions and attacks clear `_i_all_combat_ships`). `TestTurnSteps` gives
each of them the arena dump before it where there is one, else a recomputed
arena; the statics outside it come from recomputing step 3, and the
AIBUILD.C needs (`Game.needs`) from rerunning step 10 on its input.
