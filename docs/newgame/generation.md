# The galaxy generation: Init_New_Game_

What runs between the banner screen of NEW GAME and turn 1's main screen:
`Screen_Control_` case 0xd → the 1.40 patch's wrapper (0x169410) →
`Init_New_Game_` (0x12479) → screen 0x27 (reports) → `Main_Screen_` with the
home star's name. Ported byte-exact (every `Random_` draw and the saved
state). Go: `internal/game/newgame_init.go` (order, leaders, turn-1 setup),
`newgame_players.go`, `newgame_universe.go`, `newgame_homeworlds.go`,
`newgame_specials.go`, `newgame_advciv.go`, `homestarname.go`. Addresses are
ELF. Field names are `internal/save`'s.

Conventions: **R!** draws itself, **R** below, **-** never. Loops run from
the first record up unless a row says otherwise.

## Init_New_Game_ (0x12479)

Arguments from `_settings`: difficulty +0xd4, size +0xd6 (+1 with the
patch's Cluster byte 0x1784df), age +0xd7, players +0xd5; the two stack
arguments (tactical, tech) are unused.

| Step | Routine | RNG | Notes |
|---|---|---|---|
| 1 | `Get_Random_Seed_` → `_settings`+0xda | - | the seed after the ruler name draws; overwritten in step 9 |
| 2 | `Set_Game_Random_Seed_` (0x107ca) | set | `_cmd_line_seed`, else `Randomize_` = time(NULL) (`Options.Clock` in the port) |
| 3 | stardate 35000, `_NUM_PLAYERS`, `Init_Ships_Array_`, `Init_Colonies_Array_`, `_antarans` cleared, `_PLAYER_NUM` 0 unless network | - | ships status 5, location/officer −1; colonies owner/planet/+4 −1 |
| 4 | `Init_Leaders_` (0x1307f) | R! | officer data LBX records → `_leaders`; leaders 0x29 and 0x34 `Get_Random_Skills_` (mixed, kind 2 / 1, five), 0x41 two random techs; `Officer_Skill_Value_` for all |
| 5 | `Init_Players_` (0x12983) | R! | see below |
| 6 | `Cache_Player_Scientist_Anim_` | - | graphics only, not ported |
| 7 | `Allocate_Msg_Slots_` | - | (ported for the turn) |
| 8 | `Set_Game_Random_Seed_`, `Get_Random_Seed_` → +0xda | set | the **universe seed** (the clock again) |
| 9 | loop: `Universe_Generation_` (size, age, difficulty), `Generate_Home_Worlds_` until it succeeds; on success black screen, `Enforce_Planet_Max_(0xfa)`, `Set_Black_Hole_Visited_Flags_`, `Confirm_Planet_And_Star_Index_Link_` (debug check), on tech 2 `Advanced_Civilization_Colonies_` and `Assign_Advanced_Civilization_Starting_Ships_` (empty) | R | `_maps_to_generate` (command line) debug loop not ported |
| 10 | `Make_System_Monsters_`, `Make_System_Monsters_Into_Ships_`, `Generate_Wormhole_Links_`, tech 2 `Allocate_Adv_Civ_Game_Officers_`, `Assign_Marooned_Heroes_` | R! | |
| 11 | `Initialize_Black_Hole_Blocks_(0x2d)`, `Initialize_Star_In_Nebula_Info_` | - | |
| 12 | map x/y 0, screen 0x27, council winner −1, `Harold_Defaults_`, `Bill_Default_`, `Harold_Init_` (boxes), `Eric_Init_` (`Init_Events_`), `Bill_Init_` (`_bill_savegame` = 2,0,1,1,1,1) | - | |
| 13 | `Update_Star_Owners_`, `Allocate_New_Colony_Slots_`, `Twiddle_Initial_Homeworlds_`, `Allocate_New_Ship_Slots_`, `Compute_All_Star_Colony_Stuff_` (stars from the last), `Process_Trade_And_Research_Agreements_`, settings +7: `Set_Initial_Colony_Auto_Selects_`, `Initialize_Reports_`, `Init_Surrenders_` | - | |
| 14 | `Save_Game_(9)` = SAVE10.GAM (the oracle's `gen.GAM`), `Save_Game_Settings_` | - | |
| 15 | `Load_Nebula_Pictures_`, `Set_Defaults_`, `Save_Moo_Custom_Player_Data_` (empty) | - | |
| 16 | the patch's end (0x1691e7): `/droids` apps; Post Warp: with tactical combat (the new game screen's radio) apps 0x178802 ×5, always apps 0x17880c ×0x17 (`Player_Gets_Tech_App_` for every player), fields 3, 4, 21 researched and 20 researchable; `Set_Random_Seed_(+0xda)` | set | turn 1 starts from the universe seed |

### Init_Players_ (0x12983)

RACESTUF.LBX 7 records 0..12. Per player: a human copies its picks to the
base picks; an AI draws a race not taken by an earlier player (Random(13)),
copies `_race_names` and the race's picks, `Race_Customization_` (Hard and
Impossible: one of three variants from tables 0x180184 (both sets) and
0x180a26 (base picks), Random(3)), converts picks 1..9 to their values
(0x17d1f9), a banner (Random(8), then the next free one), a ruler name
(RACENAME.LBX record race·8 + Random(8) − 1). Then
`Init_NPC_Personalities_Objectives_Themes_` for every player (humans too:
the objective draw is made, 100 kept): personality from table 0x181090 by
race and Random(10)+1−difficulty, then three weighted picks (objective,
ship theme, special theme) whose weights the picks and difficulty raise;
weights are halved while their sum exceeds 1000. Then per player: officer
for hire and staging star −1, contact with itself, 50 BC, info buttons
0x4f, stardates, `Init_Player_Tech_`, `Init_Ship_Designs_`, fuel range;
`Assign_Home_Star_Names_` (names star 0, overwritten later),
`Init_Diplomatic_Relations_` (relations table 0x180ed4; AI personalities
bias towards humans: −3, −2, 0, 0, 2, 5; council turn 0x19).

`Init_Player_Tech_` (0x5e55f): the eight starting fields, every
application legal for the race (Uncreative: only cheap fields; Lithovore,
Tolerant, Feudal and strategic combat drop some), government apps, then per
field 1..0x49 without a known app one random legal app (Random(apps)); the
head start: tech level 0, 1, 2 research 1, 6, 25 fields: the first six from
table 0x18111c, the rest by `Choose_Tech_Application_` (a human counts as
objective 1 meanwhile; Creative takes the whole field; field 6 raises the
government).

## Universe_Generation_ (0x8dae8)

Restarts from the top when `Set_Star_XYs_` cannot place a star. Each pass:
`_n_maps_generated`+1, map margins 0x25, planets cleared, STARNAME.LBX 1
(0x33d names), `Randomize_Star_Names_` (0x33d swaps, **1658 draws**); the
first time in a session (`_first_time_map_is_generated`) the RNG is set to
the universe seed (so the name shuffle runs on the clock seed and is then
discarded); stars 20/36/54/71 in a 5×4/6×6/9×6/9×8 grid;
`Init_Main_Screen_` (patched: Cluster gives a 0x3f3×0x320 map at scale 15);
`Generate_Number_Of_Nebulas_` (0..1, 1..2, 1..3, 2..4); `Generate_Nebulas_`
(below). Then the stars: `Set_Star_XYs_` (spectral classes, names, places),
per star size (Random(10)), picture (Random(3)), satellites (Random(10) by
class) and `Planet_Generation_` for each, a special (`Generate_Star_Special_`,
not for star 0); then until `Map_Is_Connected_` (union find over jumps of
≤ 8 parsecs, a coin per union): `Black_Hole_Fix_` (with nebulas),
`Initialize_Black_Hole_Blocks_`, else `Set_Star_XYs_` again (no names).

- `Set_Star_XYs_` (0x8cfff): each star in its cell, x then y retried until
  inside the margins, then `Star_XY_Invalid_` (closer than 800 scaled² or
  name boxes overlapping, black holes 5 parsecs) up to 150 tries, then up
  to 150 anywhere; failing those it returns 1. The name box width is the
  player font's width of "(name)" (+"w" for names under 7 letters), or
  three Random(8) after the third map; without names always the draws.
- `Generate_Spectral_Class_`: weighted by age (0x17d83e); the patch's
  `/nobh` makes black holes random classes.
- `Planet_Generation_`: `Generate_Orbit_` (weights in the code at
  0x8bb39 by age), `Generate_Satellite_Type_` (asteroids, gas giant,
  planet, companion star; no gas giant in orbit 0), `Generate_Satellite_`
  (size, minerals, gravity, climate group (two orbits less with a
  companion), climate weighted by age (ages 0 and 1 share a table),
  picture, max farms, food per farmer). `Generate_Satellite_` always fills
  the record at `_planet_count`.
- `Generate_Star_Special_`/`Get_A_Planet_Or_Star_Special_`: Random(100) above
  table 0x17d826 gives a special: weights 0x17d832 (wormhole limited to
  size·4+4, marooned hero to zoom+2, big habitable / habitable planets
  needed, monster only with `_user_wants_n_space_monsters`); planet
  specials go on a random fitting planet. Random(100) adds a space monster
  (5%, 8% Hard, 12% Impossible). The patch's `/nosplint` hook never fires
  without the switch.
- `Generate_Nebulas_` (0x8c099): one Random(12) per nebula for its type
  (the taken flags it tests are never set, and a stale nonzero `di` ends
  the retry loop at once); the first nebula's place uses picture 0's size;
  further ones come from `Build_Min_Point_Distances_` (two
  `Randomize_List_` shuffles of 640 and 480 coordinates, 3360 draws) and a
  weighted pick; the distance shrinks by 10 on failures.

## Generate_Home_Worlds_ (0x7b8cd)

Spacing from the map size and players (players over 6 add the galaxy
size); `Build_Min_Star_Distances_` and `Build_Home_Star_List_` (for every
star but 0 the stars far from it and each other, at most 20; a group of
enough stars by reservoir sampling, shuffled 4 times); on failure one more
step apart, up to 4 tries, else 0 and the universe is generated again.
`Randomize_Home_Worlds_` (100 swaps), owners, `Modify_Home_Worlds_`:
`Enforce_3_Planet_Minimum_` (the patch's `/planets=`, 2 by default;
`Usable_Orbit_` tests the planet at `_planet_count`, sic),
`Enforce_Climate_` (terran, aquatic races ocean), `Remove_Specials_`, the
patch's start switches (none by default), size, type, minerals (Rich/Poor
home world), artifacts, gravity (`Enforce_Gravity_` writes rerolled
minerals into `planet[player]`, sic), `Init_Homeworld_Colony2_` (pop 8, or
on advanced tech from the pop pick, the starting buildings by tech level,
on average tech two scouts and a colony ship whose design goes to slot 5),
`Guarantee_A_Planet_At_Min_Dist_` (a planet within 4 parsecs, the nearest
non-home star moved in with `Coord_Movement_`); `Assign_Home_Star_Names_`,
`Ensure_At_Least_1_Food_Planet_` (`_nearest_food_worlds`). The patch's hook
(0x169245) runs `Generate_Orion_` and keeps its switches in star 0's name
bytes 0xd (0) and 0xe (/noreport 4, /noscan 2, Cluster 1, /nohousing 8).

`Generate_Orion_`: star 0 becomes Orion (its planets but one asteroids, a
gaia world) and swaps coordinates with a star near the centre
(`Get_Orion_Star_Id_`).

## Monsters, wormholes, heroes

- `Make_System_Monsters_`: up to 20 guarded stars (or the command line's
  count), spread by `Get_Weighted_Choice_Short_` and a shrinking distance;
  the special moves to the twinkle byte (+0x64) meanwhile.
  `Make_System_Monsters_Into_Ships_` (stars from the last): Random(5)+9 monster,
  `Random_Planet_`, `Build_Nonplayer_Ship_`, `Upgrade_Planet_Guarded_By_Monster_`
  (`Set_Special_For_Star_Guarded_By_Monster_` until a special fits); the
  Guardian at Orion.
- `Generate_Wormhole_Links_`: for each wormhole special a random star (not
  a home, no special), `Set_Wormhole_Id_`: candidates over size+3 parsecs
  away, shuffled with draws 1..n used as indices from 0, a reservoir pick;
  none links the star to star 0. The patch's `/nowh` removes links.
- `Assign_Marooned_Heroes_`: Random(0x26) leaders until a free one.

## Advanced tech (Advanced_Civilization_Colonies_, 0x62c70)

The RNG goes back to the universe seed. Arena blocks: `_adv_civ_player`
(0x10a0 per player: counts, a per-planet flag, 72 {star, parsecs}, 0x168
planet entries {flag, proximity, worth, planet, mark}), `_parsec`,
`_planet_selected_flag`, `_star_colony_owner`, `_planet_worth_to_player`.
`Choose_Adv_Civ_Planets_` loops rounds: the star and planet lists
(`Build_Adv_Civ_Star_List_`, `_Planet_List_`, worth and a qsort of all 0x168
entries), the players in a `Randomize_List_` order each take the best free
planet in reach (`Get_Next_Adv_Civ_Planet_`); a full round marks them
(swap-remove), a short one clears `selected[list index]` (sic) and ends.
`Twiddle_Selected_Adv_Civ_Planets_` gives the other players specials and
`Twiddle_Planet_` steps until their average worth is 90% of the best's;
then `Init_Non_Homeworld_Colony_` for each picked planet.
`Twiddle_Initial_Homeworlds_` on tech 2 also gives money ((money pick+2)·100),
explored stars in range, freighters (multiples of 5) and ships by the
smallest research.

Reads past the arena blocks, modelled: `_star_colony_owner[-1]` (the block
header's last byte, 0) and `_parsec` row −1 (player 7's last 0x114 bytes
of `_adv_civ_player`, then the header {0, 0x5100, 0}), both through star
−1 in the star lists of player 0 (`Proximity_Bonus_`).

## Quirks found

- `[0x1800ee]` (Auto_Design_Ship_'s transport name) is the hull table's
  size 5 entry, "Doom Star"; the AI's transport design keeps "?????Star"
  after the shorter name `Get_Ship_Design_Name_` copies over it.
- `Twiddle_Initial_Homeworld_` turns colonist −1 back into a scientist
  when no worker was made: the job bits land in the colony's pollution word.
- `Get_Random_Skills_` adds (not ors) the skill mask; no candidate adds
  skill 0.

## Turn 1

Reports screen (no reports), `Main_Screen_`: on stardate 35000 the first
time per player `Change_Home_Star_Name_` (0x8249c): `Do_Change_Star_Name_`
over the map at (0x83, 0x92) with H message 0x101, width from
`Max_Pixel_Width_Star_Name_Can_Be_` (room to the nearest names on the same
line at the widest zoom, `Get_Star_Name_Info_`) bounded by "WWWWWWW" and
0xcd; the patch's hook keeps star 0's switch bytes; an empty name keeps the
old one. Then fields, the mini main screen, the auto function.

## Progress

Universe_Generation_'s set-up (after the name shuffle; `Fast_Fade_In_`
when faded): RACEOPT.LBX 0 on the back page, copied to the off page, H
message 0xf2 printed outlined (style 4, `Font_Colors2_(4, 0x77, 0x79)`,
outline 2/1) into a blank 0x145×0x1e bitmap. Each progress call
(`Set_Star_XYs_` per star, the star loop per star, a failed placement with
the whole message): the message's x centred on 640, then
`Draw_Advancing_Fill_Message_`: `Get_Advancing_Bitmap_String_Range_` gives
the part of the width step n of max (2·stars) covers, `Replace_Bitmap_Colors_`
recolours [0, right) to `Font_Colors2_(4, 0x77, 0x7c)`, the bitmap is
drawn at y 0xd7, the counter advances, `_TOGGLE_PAGES_`.
`Set_Up_Home_Gen_Msg_` does the same set-up with H message 0xa1 and max
stars·25/20 (0x174 above 54 stars); Generate_Home_Worlds_ steps before and
after `Build_Min_Star_Distances_`, `Build_Home_Star_List_` per star (or a
new set-up once the counter reaches max), the whole message at the end.
Go: `internal/game/newgame_progress.go`. Captures of every page flip also cover `Universe_Generation_`, `Set_Star_XYs_`,
`Generate_Home_Worlds_` and `Build_Home_Star_List_` (their
`_TOGGLE_PAGES_` calls 0x8df54, 0x8e027, 0x8d15f, 0x7ba5d, 0x7bad4, 0x7bba2,
0x7ae00); genf1 (`build/oracle/genf1.sh`: gen-small2's new game with
`-frames`) — `TestGenerationProgressFrames` runs the generation with the
trace's clock and matches all 62 flips.

## Enforce_Planet_Max_(max) (0x8bc39)

`n = PLANET_COUNT`; with `n − Gas_Giants_() < max` nothing happens (max
0xfa, so 250 planets besides gas giants are needed). Else the home stars of
players 0..NUM_PLAYERS−1 are marked; the planet array (0x17e8 bytes) and
the stars (0x1fc8) are copied; while `PLANET_COUNT − gas ≥ max`: `k =
Random_(PLANET_COUNT) − 1`; skipped when the copy's planet k is already
removed (colony word −2) or at a home star, or when the word at
`stars[planet.star] + 0x4a + 2k` (**quirk**: the star's slot array indexed
by the planet number) is ≤ −1 or names a gas giant in the copy; else
PLANET_COUNT−1 and the copy's colony word = −2. Then
`Clear_Planets_In_Star_Structure_` and the survivors are copied back in
order, each home planet index (`Home_Planet_`: the first player with it)
renumbered and the star's orbit slot set. (A debug PFMT_ prints are
skipped in the release.) Ported; the read past the star array (into the
next DOS block) is taken as −1 and logged, and no oracle galaxy reaches
the limit yet.

## Not ported

- `Cache_Player_Scientist_Anim_`, the `_maps_to_generate` / `_n_stats_to_generate`
  debug loops, the patch's command line parsing: the switches' bytes are
  read from the executable, so they behave as without the switch. What the
  home start switches do is ported (`patchHomeStart`, 0xf4b94): the first
  other planet of type 3 in the home star's slots (the loop has no bound),
  /hugestart size 4, farms 10; /minstart size ≥ 2 (farms 5), minerals
  (m & 1) + 1; /goodstart minerals 4, else /richstart 3; each recomputes
  gravity as 1 − picks[10] + picks[11] of the home star's owner. Not traced
  (it needs the switch on the original's command line).
- Hotseat and network games (`_game_type` ≠ 0).
