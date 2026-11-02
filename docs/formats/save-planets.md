# Saved game: planet and colony records

Blocks `planet` (0x162e9, 360 x 17 bytes, `planet_count` in use) and
`colony` (0x0025d, 250 x 361 bytes, `NUM_COLONIES` in use) of a save; see
`save.md`. The game keeps both as heap arrays (`_planet` 0x1930d4, `_colony`
0x192b18 hold the pointers) and indexes them as `base + i*0x11` /
`base + i*0x169`. There are no debug types for them: every name below is
invented from the routines that use the field. Addresses are ELF addresses
in `build/orion2.elf`.

Confidence marks: **solid** (seen written and read with clear meaning,
consistent in all ten saves), *likely* (one clear use, or meaning inferred
from the routine name), `?` (unknown). Integers are little-endian.

## Planet record (17 bytes)

Written by planet generation (`Generate_Satellite_` 0x8c5d7,
`Set_Secondary_Planet_Data_` 0x8c6c0, `Planet_Generation_` 0x8e280,
`Make_One_Planet_` 0x7c3c4, the `Enforce_*` fix-ups 0x7b22b..0x7b5b2,
`Generate_Orion_` 0x7bbbc) and later changed by buildings (`Add_Building_`
0x13fd9).

| Off | Type | Name | Meaning | Saves | Conf. |
|---|---|---|---|---|---|
| 0x00 | i16 | colony | index into `colony`, -1 none. `Init_Colony_` 0x12d75 sets it, `Set_Secondary_Planet_Data_` clears it | -1, 0..78 | solid |
| 0x02 | u8 | star | index into `star` | 0..NUM_STARS-1 | solid |
| 0x03 | u8 | orbit | slot 0..4; `star[0x4a + 2*orbit]` (5 x i16 planet indices) points back | 0..4 | solid |
| 0x04 | u8 | type | 1 asteroid belt, 2 gas giant, 3 planet (`Add_Building_` case 48 turns 2 or 1 into 3) | 1..3 | solid |
| 0x05 | u8 | size | 0 tiny .. 4 huge; indexes `_planet_size_table` {1,3,7,9,10}, `_planet_max_farms`, `_planet_max_population`; `Colony_Race_Pop_Limit_` 0xe0c1d | 0..4 | solid |
| 0x06 | u8 | gravity | 0 low, 1 normal, 2 heavy (`Generate_Gravity_Class_`) | 0..2 | solid |
| 0x07 | u8 | climate_group | `Get_Planet_Group_` 0x8cd55: `_class_to_group[star.class*5 + orbit]`; selects the climate roll row (`Enforce_Climate_`); index into `_planet_group_string` | 0..3 | likely |
| 0x08 | u8 | climate | 0 toxic, 1 radiated, 2 barren, 3 desert, 4 tundra, 5 ocean, 6 swamp, 7 arid, 8 terran, 9 gaia. Terraforming (`Add_Building_` 44): 2→3/4, 3→7, 4→6, 5/6/7→8; Gaia (17): 8→9 | 0..9 | solid |
| 0x09 | u8 | picture | `Random(3)-1`; `C_Anims_` 0xbba8e draws PLANETS.LBX entry `climate*3 + picture` | 0..2 | solid |
| 0x0a | u8 | minerals | 0 ultra poor .. 4 ultra rich (`Generate_Mineral_Class_`, `Colony_Industry_Per_Worker_`) | 0..4 | solid |
| 0x0b | u8 | food_per_farmer | base food (`Generate_Food_Per_Farmer_` 0x8bfa3); +1 by Soil Enrichment, + table delta by Gaia/terraforming; read by `Colony_Food2_Per_Farmer_` 0xde03e | 0..4 | solid |
| 0x0c | u8 | terraform_count | +1 per Terraforming; scales its cost in `Colony_Product_Cost_` 0xe0dd6 | 0..3 | solid |
| 0x0d | u8 | max_farms | `_planet_max_farms[size]` {2,4,5,7,10} (`Generate_Max_Farms_` 0x8c01e); only reader found: `Init_Homeworld_Colony2_` 0x13a3d | 2..10 | likely |
| 0x0e | u8 | artificial_max_pop | `_planet_max_population[size]`, written only by Artificial Planet (`Add_Building_` 48); 0 otherwise. No reader found | 0, 20, 25 | likely |
| 0x0f | u8 | special | shared with `star+0x28`: 6 natives (`Make_New_Colony_Or_Outpost_` turns it into 3 race-9 pops), 10 artifacts (`Enforce_Artifacts_Special_`), 11 Orion (`Generate_Orion_`). Others seen: 4, 5, 7 (names not yet tied; strings are runtime `_planet_special_string`) | 0,4,5,6,7,10,11 | partial |
| 0x10 | u8 | flags | bit 1: soil enriched (set by `Add_Building_` 37, tested by `Colony_Can_Build_Product_` 0xe11bc). Other bits never seen | 0, 2 | solid |

Checks on all ten saves: `colony` back-links agree with `colony[i].planet`.
25 records with indices below `planet_count` are not referenced by their
star's orbit table (stars whose planets were regenerated during galaxy
setup, e.g. star 8 in SAVE1 now lists planets 174 and 173). The star list
is authoritative; those records are dead. Saves 1-3 and 7-9 also hold one
non-zero record past `planet_count`.

## Colony record (361 bytes)

Created by `Init_Colony_` 0x12d75 (clears the record, owner, planet, word 4
= -1, queue all -1, 0x123 = -1, 0x12f = 4), called from
`Make_New_Colony_Or_Outpost_` 0xe5eb3 and `Init_Homeworld_Colony2_` 0x13a3d.
`Init_Colonies_Array_` 0x1241a fills empty slots with owner -1, planet -1,
word 4 = -1. The per-turn economy is recomputed by `Pre_Import_Computing_`
0xe1d59 and `Pass_Out_Imports_` 0xdf8f0, so 0xdd..0x114 are caches of
derived values, but they are saved and must round-trip.

Population works in "half units" in the demand tables (2 = one colonist's
food). Commodity index used by the 4-element arrays: 0 food, 1 industry,
2 research, 3 BC.

| Off | Type | Name | Meaning | Saves | Conf. |
|---|---|---|---|---|---|
| 0x000 | i8 | owner | player index; -1 empty/dead slot | -1, 0..7 | solid |
| 0x001 | i8 | reserved_for | slot reserved for that player by `Allocate_New_Colony_Slots_` 0xe64f4; `Make_New_Colony_Or_Outpost_` claims it and sets -1 | -1, 1, 7 | solid |
| 0x002 | i16 | planet | index into `planet`; -1 in dead slots | | solid |
| 0x004 | i16 | ? | set -1 by `Init_Colony_`/`Init_Colonies_Array_`; no reader found | -1 (6 x 0) | ? |
| 0x006 | u8 | is_outpost | 1 for an outpost (`Make_New_Colony_Or_Outpost_` with BX≠0); many routines skip such colonies (`Colony_Calculation_` 0xe2a70) | 0 | solid |
| 0x007 | i8 | morale | `Colony_Morale_` 0xddb25; `Draw_Info_Morale_Both_` 0xb53cc draws abs(m/2) icons | -5..10 | solid |
| 0x008 | i16 | pollution | industry lost to pollution, subtracted in `Colony_Industry_Production_` 0xdee1b (0 with Core Waste Dump) | 0..49 | solid |
| 0x009 | — | (high byte of `pollution`) | listed separately by the first pass; the i16 at 0x008 covers it | 0 | solid |
| 0x00a | u8 | pop_count | colonists in use in the list at 0x0c | 0..31 | solid |
| 0x00b | i8 | specialty | `Colony_Specialty_` 0xe1e1f: 0 agricultural, 1 industrial, 2 research when one output > half the total, else -1; `Colony_Type_` 0xbc8df names it | -1, 0..2 | solid |
| 0x00c | u32[42] | colonists | one dword per colonist, see below | | solid |
| 0x0b4 | i16[10] | pop_fraction | per race 0..9, growth accumulated in 1/1000 colonist (`Draw_Info_Name_And_Pop_` 0xbd434 prints `pop + sum/1000`) | 0..998 | solid |
| 0x0c8 | i16[10] | pop_growth | per race, growth per turn in 1/1000 (summed and shown as "+n"); can be negative | -879..482 | solid |
| 0x0dc | u8 | turns_owned | +1 per turn saturating at 255 (`Apply_Colony_Changes_` 0xe3f6e); 0 on capture (`Change_Pop_Ownership_` 0xecbf7, `Do_Surrender_Colony_`) | 0..255 | solid |
| 0x0dd | u8 | food_per_farmer | `Colony_Food2_Per_Farmer_` result (half units) | 0..14 | solid |
| 0x0de | u8 | industry_per_worker | `Colony_Industry_Per_Worker_` 0xdec95 | 0..15 | solid |
| 0x0df | u8 | research_per_scientist | `Colony_Research_Per_Scientist_` 0xdfdc6 | 0..11 | solid |
| 0x0e0 | u8 | can_farm | `Colony_Environmental_Stuff_` 0xe1ced: 0xff if food per farmer ≠ 0, else 0 | 0, 0xff | solid |
| 0x0e1 | u8 | ? | always 0 | 0 | ? |
| 0x0e2 | u8 | climate | effective climate: planet climate, with radiated (1) treated as barren (2) when a Barrier, Flux or Radiation Shield stands (`Colony_Environmental_Stuff_`) | 0..9 | solid |
| 0x0e3 | u8[4] | ? | always 0, no access found | 0 | ? |
| 0x0e7 | i16[4] | output | food, industry, research, BC produced (`Colony_Food_Production_` 0xde664 etc.; sort keys in `cmp_Food_`, `cmp_Industry_`, `cmp_Science_`, `cmp_BC_`). Industry is net of tax and replicator use | 0..421 | solid |
| 0x0ef | u8[4] | upkeep | food eaten (`Colony_Food_Maintenance_` 0xdeb4b), industry eaten (`Colony_Industry_Maintenance_` 0xdf546), research (always 0), BC building upkeep (`Colony_BC_Maintenance_` 0xe094f) | | solid |
| 0x0f3 | i16 | food_import | food delivered by freighters (+) or surplus exported (−) (`Pass_Out_Imports_`) | -80..21 | solid |
| 0x0f5 | i16 | industry_upkeep_paid | min(industry upkeep, industry) (`Pre_Import_Computing_`) | 0..7 | solid |
| 0x0f7 | i16 | ? | always 0; would be the research slot of the 0xf3 array | 0 | ? |
| 0x0f9 | i16 | bc_deficit | BC upkeep − BC produced (`Colony_BC_Maintenance_`) | -177..29 | solid |
| 0x0fb | u8 | recyclotron_industry | reset in `Colony_Industry_Production_`, set when Recyclotron (bldg 33) stands | 0..47 | likely |
| 0x0fc | u8[4] | food_demand | half units eaten by: own race, other races not conquered, conquered, natives (race 9) | | solid |
| 0x100 | u8[4] | industry_demand | half units by: own race, androids (race 8), other not conquered, conquered (races with player+0x8b0 eat industry) | | solid |
| 0x104 | u8[8] | food_demand_by_race | per player race 0..7 | | solid |
| 0x10c | u8[8] | industry_demand_by_race | per player race 0..7 | | solid |
| 0x114 | u8 | replicated_food | food made from industry by Food Replicators (bldg 16; `Colony_Replicators_` 0xdf66f) | 0..31 | solid |
| 0x115 | i16[7] | queue | production queue, -1 empty; codes below | | solid |
| 0x123 | i16 | last_built | product finished this turn (queue code), else -1 (`Apply_Production_` 0xe36df); -3 housing reached max pop, -39 new colony, -40 captured | | solid |
| 0x125 | i16 | production | accumulated industry towards queue[0], capped 0x7fff | 0..3378 | solid |
| 0x127 | i16 | industry_taxed | industry converted to BC this turn (`Colony_Industry_To_Tax_` 0xe08f6, rate player+0x31) | 0..35 | solid |
| 0x129 | u8 | auto_build | `Player_Colony_Autobuild_` 0xd2783 refills an empty queue | 0, 1 | solid |
| 0x12a | u8[2] | ? | always 0 | 0 | ? |
| 0x12c | i16 | pending_production | production added on top of 0x125 next turn, then cleared (`Colony_Buy_Outright_` 0xe109d, scrapping) | 0 | solid |
| 0x12e | u8 | assimilation | accumulator; each 240 converts one conquered pop (`Apply_Assimilation_` 0xe3456) | 0..234 | solid |
| 0x12f | u8 | occupation | 3 normal/assimilating, 4 set on creation or capture (becomes 3 next turn in `Apply_Colony_Changes_`), 0 the kill-conquered policy (`Apply_Colony_Pop_Growth_` 0xe2dca); set by `Occupation_Policy_Popup_` 0xcd969 | 0, 3, 4 | likely |
| 0x130 | i16 | marines | infantry units; +1 every 5 turns up to barracks limit (`Produce_Ground_Military_` 0xe3616) | 0..18 | solid |
| 0x132 | i16 | armor | tank units, same rule with Armor Barracks | 0..8 | solid |
| 0x134 | u8 | armor_progress | 0..4 turns towards the next tank | 0..4 | solid |
| 0x135 | u8 | marine_progress | 0..4 turns towards the next marine | 0..4 | solid |
| 0x136 | u8[49] | buildings | 1 if building id i stands (`Add_Building_` sets `0x136+id`); names below | 0, 1 | solid |
| 0x167 | u16 | SoldStardate | low word of the stardate a building was last sold (Check_Bldg_Fields_: one sale per turn) | 0 | yes |

### Colonist dword (0x0c + 4*i)

| Bits | Name | Meaning |
|---|---|---|
| 0-3 | race | player index whose race this is (0..7), 8 android, 9 native |
| 4-6 | loyalty | player the colonist currently belongs to (`Change_Pop_Ownership_`) |
| 7-8 | job | 0 farmer, 1 worker, 2 scientist (`Job_String_`) |
| 9 | ? | set on every colonist created (`Make_New_Colony_Or_Outpost_`, `Apply_Production_`); always 1 in saves |
| 10 | conquered | not yet assimilated (`Conquered_String_`, `Apply_Assimilation_` clears it) |
| 11-31 | — | always 0 |

All 4385 live colonists in the saves fit this: races 0..7 and 9, bit 9 set,
bits 11+ clear. Slots past `pop_count` may hold stale colonists.

### Queue codes (0x115, 0x123)

| Code | Product |
|---|---|
| 1..48 | building id (below) |
| ≤ -100 | ship: `ship[-(code+100)]` holds the design being built |
| -1 | empty |
| -2 | trade goods (never completes) |
| -3 | housing (completes when pop is at the planet maximum) |
| -4, -5, -6 | android farmer, worker, scientist (new race-8 colonist) |
| -7 | spy |
| -10 | in queue[1] only: "repeat queue[0]" |
| -15 | freighter fleet (+5 freighters, player+0x36) |

### Building ids

Names are the 49 strings after the field and application names in
`TECHNAME.LBX` entry 0 (`Load_Tech_Names_` 0x5df0a stores them into
`_buildings`, 0x17eb3d, 19 bytes each); several ids are confirmed by code
(2, 9, 11, 13, 17, 22, 37, 44, 48 in `Add_Building_`).

| Id | Building | Id | Building | Id | Building |
|---|---|---|---|---|---|
| 0 | (none) | 17 | Gaia Transformation | 34 | Robotic Factory |
| 1 | Alien Control Center | 18 | Currency Exchange | 35 | Research Lab |
| 2 | Armor Barracks | 19 | Galactic Cybernet | 36 | Robo Miner Plant |
| 3 | Artemis System Net | 20 | Holo Simulator | 37 | Soil Enrichment |
| 4 | Astro University | 21 | Hydroponic Farm | 38 | Space Academy |
| 5 | Atmosphere Renewer | 22 | Marine Barracks | 39 | Spaceport |
| 6 | Autolab | 23 | Barrier Shield | 40 | Star Base |
| 7 | Automated Factory | 24 | Flux Shield | 41 | Star Fortress |
| 8 | Battlestation | 25 | Gravity Generator | 42 | Stellar Converter |
| 9 | Capitol | 26 | Missile Base | 43 | Subterranean Farms |
| 10 | Cloning Center | 27 | Ground Batteries | 44 | Terraforming |
| 11 | Colony Base | 28 | Radiation Shield | 45 | Warp Interdictor |
| 12 | Deep Core Mine | 29 | Stock Exchange | 46 | Weather Controller |
| 13 | Core Waste Dump | 30 | Supercomputer | 47 | Fighter Garrison |
| 14 | Dimensional Portal | 31 | Pleasure Dome | 48 | Artificial Planet |
| 15 | Biospheres | 32 | Pollution Processor | | |
| 16 | Food Replicators | 33 | Recyclotron | | |

One-shot products (Colony Base 11, Gaia 17, Soil Enrichment 37,
Terraforming 44, Artificial Planet 48, ...) change the planet or star and
never set their flag. Marine Barracks is given to every new colony
(`Make_New_Colony_Or_Outpost_` sets 0x14c).

## Links and consistency (all ten saves)

- planet.colony ↔ colony.planet agree for every live colony and planet.
- colony.owner has its bit set in `star[0x33]` (owner bitmask) of the
  colony's star (`Init_Colony_`); `star[0x35]` gets the Colony Base bit.
- Dead slots below `NUM_COLONIES` (7 in total) have owner -1 and planet -1;
  slots past `NUM_COLONIES` are left as `Init_Colonies_Array_` made them
  (owner, reserved, planet and word 4 all 0xff).
- 0x123 values seen are building ids, ship codes, -1, -7 and -40.

## Open questions

- Colony 0x04, 0x09, 0x0e1, 0x0e3..0x0e6, 0x0f7, 0x12a: no use
  found. Accesses go through colony pointers passed as arguments, so a
  scan by base address can miss readers.
- Colonist bit 9 (always set).
- Planet special codes 4, 5, 7 (counts suggest gold, gem deposits and
  another one-off special) and climate group names.
- Planet 0x0d and 0x0e look write-only in practice.
