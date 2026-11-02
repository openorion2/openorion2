# Saved game: stars, nebulas, map globals

Part of `docs/formats/save.md` (block offsets there). Records are little
endian, packed, no padding. Addresses are ELF addresses in
`build/orion2.elf`; routine names are the Watcom symbols. Every claim was
checked against the ten shipped saves (`SAVE1..10.GAM`) unless marked.

## `star` (0x71 bytes, 72 records, `NUM_STARS` in use)

`_star` (0x19306c) points to a 0x1fc8-byte block (`Allocate_Data_Space_`,
0x10df1). `Save_Game_` writes it in one `fwrite(…, 0x71, 0x48)`;
`Load_Game_` reads it record by record. Records `>= NUM_STARS` are zero
except `twinkle_frame`/`twinkle_end` = -1 (`Clear_Twinkling_Star_Data_`
loops over all 72). `NUM_STARS` is set by `Universe_Generation_` (0x8dae8)
from the galaxy size: 20, 36, 54, 71 (shipped saves: 20 and 71).

"Bitmask" means one bit per player, bit `p` = player `p` (LSB first);
"per player" arrays are indexed by player number 0..7.

| Off | Size | Type | Name | Meaning | Seen in saves | Evidence |
|---|---|---|---|---|---|---|
| 0x00 | 15 | char[15] | `name` | NUL-terminated name, copied from `STARNAME.LBX` record (0xf bytes). Unused tail bytes are 0. | 3..10 chars; bytes 0x0a..0x0e always 0 | `Set_Star_XYs_` 0x8cfff (copy from `_name_seg + name_index*0xf`); `Star_Is_Orion_` 0x7a115 (`strcmp` with "Orion"); `Do_Change_Star_Name_` |
| 0x0f | 2 | int16 | `x` | Map X, 0..`MAP_MAX_X` | 40..1447 | `Set_Star_XYs_` writes; `Dist_Between_Stars_`, `Star_Is_In_Some_Nebula_` 0xeba7c |
| 0x11 | 2 | int16 | `y` | Map Y, 0..`MAP_MAX_Y` | 25..1136 | same |
| 0x13 | 1 | uint8 | `size` | Star size class 0..2 (gen: `Random(10)-1` <3 → 0, <7 → 1, else 2). Picks picture and its width with zoom level (`word[0x1931ac + 2*(size+zoom)]`). | 0..2 | `Universe_Generation_`; `Get_Star_Anim_Dimension_` 0x85b71; `Draw_A_Star_` 0x83b02 |
| 0x14 | 1 | int8 | `owner` | Displayed owner, -1 none. Recomputed by `Update_Star_Owners_` 0x8a503 as `Star_Owner_(star, PLAYER_NUM)`: the viewing player if it has a colony here, else the player with the most colonies the viewer knows of. Not authoritative (61 records have colonies but -1). Colours the name: `player[owner]+0x26`. | -1, 0..7 | `Star_Owner_` 0x7a1a8; `Star_Player_Palette_Color_` 0x78cb0 |
| 0x15 | 1 | uint8 | `picture` | Picture variant 0..2 (`Random(3)-1`), argument of `Get_Star_Picture_Seg_(class, picture, size)` | 0..2 | `Universe_Generation_`; `Draw_A_Star_` |
| 0x16 | 1 | uint8 | `spectral_class` | 0..5 normal colour classes (<6 = can have planets), 6 = black hole | 0..6 (23 black holes) | `Habitable_Spectral_Class_` 0x792a9 (`<6`); `Set_Black_Hole_Visited_Flags_` 0x8d65d (`==6`); `Generate_Spectral_Class_` |
| 0x17 | 8 | int8[8] | `last_selected_orbit` | Per player: orbit (0..4) of the colony last selected at this star, -1 none | 0 or 1 (the 0s are zero-init, never reset) | `Set_Initial_Colony_Auto_Selects_` 0x123ce; `Last_Colony_Selected_` 0x7819e; `Set_Last_Colony_Selected_` 0x786e9; `Set_Colony_Auto_Select_For_Star_` 0x7862b |
| 0x1f | 9 | bitfield[72] | `black_hole_blocked` | Bit `j` (byte `j>>3`, bit `j&7`) set when the line to star `j` passes a black hole; symmetric. Black holes themselves have empty rows. | symmetric in all saves, no bits ≥ `NUM_STARS` | `Initialize_Black_Hole_Blocks_` 0xeb87d (memset 9, `Point_Within_D_Of_Seg_`, `Set_Bit_Field_` both ways); `Black_Hole_Blocks_Points_`, `Map_Is_Connected_`, `Star_In_Range_Of_Player_Aux1_` |
| 0x28 | 1 | uint8 | `special` | System special. 0 none, 1 stable wormhole, 2 space debris, 3 pirate cache, 4 gold deposits, 5 gem deposits, 6 natives, 7 splinter colony, 8 marooned hero, 9 space monster, 10 ancient artifacts, 11 Orion. Names from `ESTRINGS.LBX` entry 0 strings "No Special".."Orion Special" in this order; 9 and 11 confirmed by code, others by order only. Wormhole stars carry 0, not 1. | 0, 2..8, 10, 11 | `Player_Sighted_Monster_` 0x79e4d (`==9`); `Make_Ship_Arrive_At_Star_` 0xffdda (`==0xb` → `Event_Orion_Discovered_`); `Make_System_Monsters_Into_Ships_` 0x122cc; `Remove_Specials_` 0x7c73d |
| 0x29 | 1 | int8 | `wormhole_to` | Star at the other end of the wormhole, -1 none. Links are mutual. Not generated when the patched-in flag byte 0x1783d3 is set (code at 0xf4b56). | -1 or valid, always symmetric | `Set_Wormhole_Id_` 0x8d6d6; `Generate_Wormhole_Links_` 0x8cc15; `Draw_Wormhole_Links_` 0x85593 |
| 0x2a | 1 | bitmask | `blockaded` | Players whose colonies here are blockaded (all colony owners if a monster/Antaran ship, owner ≥ 8, is present) | subset of `colony_owners` | `Compute_Blockades_` 0xe5097; `Colony_Is_Blockaded_` 0xdf8c1 |
| 0x2b | 8 | bitmask[8] | `blockaded_by` | Per blockaded player: bitmask of players blockading it here (enemy armed ships with relation 4..6 at `player+0x627+p`) | almost all 0 | `Compute_Blockades_` (clears 8 bytes, `or [star+0x2b+p]`), used for `Change_Relations_` |
| 0x33 | 1 | bitmask | `visited` | Players who explored the system. Black holes get 0xff. | | `Player_Has_Visited_` 0x79e32; `Set_All_Visited_Flags_` 0x79fdb; `Set_Black_Hole_Visited_Flags_` |
| 0x34 | 1 | bitmask | `special_popup_pending` | Players who still have to be shown the special's popup; cleared after `Draw_Special_` | almost all 0 | `Set_G_Special_Value_` 0xc8d56 |
| 0x35 | 1 | bitmask | `colonize_prompt_off` | Players not to be asked to colonize here (set when declined, when a colony/outpost ship is built here, for the start homeworld; cleared when such a ship arrives or by `Add_Building_`). Name is a guess from use. | | `Colonization_` 0xfdb01 (test 0xfdbc6, set 0xfde12); `Apply_Production_` 0xe3b4f; `Make_Ship_Arrive_At_Star_` 0xffe5b; `Twiddle_Initial_Homeworld_` 0xe5615 |
| 0x36 | 1 | bitmask | `ignore_combat` | Players that chose not to fight here this turn | | `Search_For_Battles_` 0xea061/0xea0a4; `Set_Engage_Enemy_Forces_Flag_` 0x721d4; `Reset_Needless_Ignore_Combat_Ships_Flags_`; cleared on arrival |
| 0x37 | 1 | int8 | `colonize_player` | Turn-local: 8 = no claimant yet, `p` = sole player with a colony/outpost ship here, -1 = contested | -1 (655), 0, 1, 7 | `Find_Colonize_Players_` 0xe6170 (init 8); `AI_Colonize_`; `Set_Colonize_Button_Flag_` |
| 0x38 | 1 | bitmask | `colony_owners` | Players with a colony or outpost here. Equals the OR of `colony[planet.colony].owner` over the orbits in all saves. | | `Compute_Star_Colony_Stuff_` 0xe5296; `Player_Has_Colony_` 0x79d1c |
| 0x39 | 1 | bitmask | `warp_interdictor` | Players with a Warp Interdictor (building 45) here | always 0 | `Compute_Star_Colony_Stuff_`; `Colony_Building_Score_` |
| 0x3a | 1 | — | unknown | Not written by `Compute_Star_Colony_Stuff_` nor sent in game orders; no access found | always 0 | — |
| 0x3b | 1 | bitmask | `base_tachyon` | Players with an orbital base (Star Base 40, Battlestation 8 or Star Fortress 41) here and Tachyon Communications (app 180) | | `Compute_Star_Colony_Stuff_` |
| 0x3c | 1 | bitmask | `base_subspace` | Same with Sub-Space Communications (app 176) | | `Compute_Star_Colony_Stuff_` |
| 0x3d | 1 | bitmask | `stargate` | Players with a colony here and Star Gate (app 170) | always 0 | `Compute_Star_Colony_Stuff_`; `Player_Has_Stargate_` 0x79dea; `Ships_Try_To_Move_To_` |
| 0x3e | 1 | bitmask | `jumpgate` | Players with a colony here and Jump Gate (app 99) | | `Compute_Star_Colony_Stuff_`; `Player_Has_Jumpgate_` 0x79d68 |
| 0x3f | 1 | bitmask | `artemis_net` | Players with an Artemis System Net (building 3) here | always 0 | `Compute_Star_Colony_Stuff_` |
| 0x40 | 1 | bitmask | `dimensional_portal` | Players with a Dimensional Portal (building 14) here | always 0 | `Compute_Star_Colony_Stuff_`; `Ships_Try_To_Move_To_` 0xff982; `Process_Retreating_Ships_` |
| 0x41 | 1 | bitmask | `ai_stage_point` | AI: players using this star as a defensive staging point | | `Compute_Defensive_Stage_Points_` 0xdbe84; `Move_To_Stage_Points_` |
| 0x42 | 8 | int8[8] | `officer` | Per player: leader (index into `leaders`, 0..66) governing this system, -1 none | -1 or 8..51 | `Universe_Generation_` (memset -1); `Assign_Leader_To_Star_`, `Colony_Officer_` 0xdd9f2, `Eliminate_Player_` |
| 0x4a | 10 | int16[5] | `planet` | Per orbit 0..4: index into `planet`, -1 empty. Always valid (< `planet_count`) and `planet[i].star == this`, `planet[i].orbit == k`. Black holes: all -1. | | `Universe_Generation_` (init -1); `Star_Is_Outpost_Star_` 0x7a133, `Players_Home_Star_` 0x79e8d, ~40 more |
| 0x54 | 16 | int16[8] | `relocation` | Per player: ship relocation target star, -1 none | always -1 | `Universe_Generation_`; `Set_All_Star_Relocations_`, `Clear_All_Star_Relocations_` |
| 0x64 | 1 | int8 | `twinkle_frame` | Twinkle animation counter, -1 idle (UI state) | always -1 | `Draw_A_Star_`; `Get_Twinkling_Star_` 0x85f2f; `Clear_Twinkling_Star_Data_` 0x82d9b |
| 0x65 | 1 | int8 | `twinkle_end` | Frame at which the twinkle stops, -1 idle | always -1 | same |
| 0x66 | 1 | — | unknown | Only access found: set to 0 in `Make_Ship_Arrive_At_Star_` (0xffedf) when the ship's owner has `player+0x28 == 100`; never read | always 0 | — |
| 0x67 | 8 | int8[8] | `surrender_to` | Per player: player its colonies here surrender to, -1 none. `Init_Surrenders_` sets only the first `NUM_PLAYERS` bytes, so the rest keep 0 from generation. | -1, or 0 beyond `NUM_PLAYERS` | `Init_Surrenders_` 0xe4e54; `May_Surrender_Star_` 0xe4a09; `Do_Surrenders_` 0xe4dc9 |
| 0x6f | 1 | bool | `in_nebula` | Star lies inside a nebula (`Point_Is_In_Some_Nebula_`) | 0/1 (25) | `Initialize_Star_In_Nebula_Info_` 0xeba96; AI strength routines |
| 0x70 | 1 | bool | `artifacts_homeworld` | Set by `Twiddle_Initial_Homeworld_` (0xe57c7) when a start homeworld's system has special 10 (ancient artifacts); no reader found | 0/1 (16) | — |

Links: `planet` → `_planet` (17-byte records, `+2` star, `+3` orbit, `+0`
colony); `officer` → `leaders`; `wormhole_to`, `relocation` → `star`;
`last_selected_orbit` → `planet[orbit]`. Some saves hold `planet` records that
no star references (e.g. SAVE5 154..158); the star side is authoritative,
those are stale records.

Building and application numbers above come from the colony bytes
`colony+0x136+building` and `player+0x117+app` read in
`Compute_Star_Colony_Stuff_`, named via `TECHNAME.LBX` entry 0 (buildings
start at "No Building", applications at "No Tech"). `Player_Gets_Tech_App_`
(0xe4204) reruns `Compute_Star_Colony_Stuff_` exactly for apps 99, 170, 176
and 180. The per-player bitmask bytes 0x35, 0x36, 0x38, 0x39, 0x3b..0x40 are
also the ones replicated in network game orders (`Decode_Game_Orders_`
0xf83d8).

## `nebula` (0x14 bytes, one block, `NUM_NEBULAS` in use)

`_nebula` (0x193068) is a 20-byte block: four 5-byte entries.

| Off | Size | Type | Name | Meaning |
|---|---|---|---|---|
| 0x0 | 2 | int16 | `x` | Map X of the nebula picture's top-left corner |
| 0x2 | 2 | int16 | `y` | Map Y |
| 0x4 | 1 | uint8 | `picture` | Nebula picture 0..11 (`Random(12)-1`, each used once) |

Unused entries are zero. `NUM_NEBULAS` (uint8, 0x199f0d) is 0..4 (saves: 1,
3, 4). `Point_Is_In_Nebula_N_` (0xeb9c8) tests `((px-x)/3, (py-y)/3)` against a
mask indexed by nebula slot (`[0x1902a4 + 16*n]`, not saved; mask byte `> 5`
means inside). Source: `Generate_Nebulas_` (0x8c099).

## Map globals

| Global | Type | Meaning | Saves |
|---|---|---|---|
| `MAP_MAX_X` | int16 | Map width: 506, 759, 1012, 1518 for galaxy size 0..3 | 506, 1518 |
| `MAP_MAX_Y` | int16 | Map height: 400, 600, 800, 1200 | 400, 1200 |
| `max_map_scale` | int16 | Most zoomed-out scale: 10, 15, 20, 30 | = `cur_map_scale` |
| `max_zoom_count` | int16 | Highest zoom level: 0..3 | 0, 3 |
| `cur_map_scale` | int16 | Current scale, one of 10, 15, 20, 30; zoom level = index in that list, capped at `max_zoom_count` (`Map_Scale_To_Zoom_Level_` 0x79917) | 10, 30 |
| `cur_map_x`, `cur_map_y` | int16 | Main map scroll origin (`Center_Map_`, `Scroll_Map_`, `Main_Screen_`) | 0, 0 |
| `star_bg_index` | uint8 | Main screen star background picture (`Buffer_Reload_Star_Bg_` 0x81cb4); `Harold_Defaults_` sets 2 | 2 |

The size table is `Init_Main_Screen_` (0x8cf09). It first jumps into
appended patch code (0x1693b6) that, when a game option bit is set, uses
1011×800 with scale 15/20 instead (not seen in the saves, unverified).

## Open questions

- 0x3a and 0x66: purpose unknown; no reader found.
- 0x35 meaning is inferred from where it is set and cleared.
- `special` names 1..8 and 10 rely on string order; only 9 and 11 are tied to code.
- 0x70 has no reader in the disassembly found so far.
