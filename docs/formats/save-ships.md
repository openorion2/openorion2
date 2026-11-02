# Saved game: ships, ship designs, leaders, moveable boxes

Record layouts for the `ship`, `leaders`, `moveable_box` and
`moveable_box_order` blocks of a save (block offsets in `save.md`). All
values are little-endian; `i8/u8/i16/u16/u32` give signedness as the code
loads them (`movsx` vs `movzx`). Addresses are ELF addresses in
`build/orion2.elf`. "Seen" ranges come from the ten shipped saves.
Marks: **?** means the meaning is inferred and not fully proven.

## Ship design (99 bytes, 0x63)

Shared by ship records (offset 0) and player designs: each player record
holds 6 designs at player+0x326 (`Design_Name_` 0x77ae6 and the other
`Design_*_` accessors at 0x77c59..0x77dfa index
`_player + p*0xea9 + 0x326 + d*0x63`; `Update_Automatic_Design_Equipment_`
0x572ab loops d = 0..5). `Create_Ship_` (0xaf7b4) copies the design whole
into the new ship (`Copy_Structure_(.., 99)`). Monster and Antaran designs
are built in place by `Load_*_Ship_Design_` (e.g. Guardian 0x57f2c).
`Update_Player_Design_` (0x6ea48) fills it from the design screen's
`_design` working buffer (a different, larger layout).

| Off | Size | Name | Meaning | Seen |
|---|---|---|---|---|
| 0x00 | char[16] | `name` | NUL-terminated design/ship name | readable text |
| 0x10 | u8 | `size` | hull size 0..5 (frigate..doom star); indexes the hull table `_hull_data` (0x24-byte rows) | 0–4 |
| 0x11 | u8 | `type` | 0 combat, 1 colony ship, 2 transport, 4 outpost (`Load_Colony/Transport/Outpost_Ship_Design_` 0x564cd/0x5663e/0x56586); 1 and 4 are treated as colonisers in `Make_Ship_Arrive_At_Star_` 0xffdda | 0, 1, 2 |
| 0x12 | u8 | `shield` | shield class (combat 0x26) | 0–4 |
| 0x13 | u8 | `drive` | drive tech; owners ≥ 8 use `Best_Warp_Drive_` instead (`Load_Combat_Ship_` 0x4954a) | 0–5 |
| 0x14 | u8 | `ftl_speed` | `Get_FTL_Speed_(player, drive)` (`Update_Player_Design_`) | 0–7 |
| 0x15 | u8 | `computer` | computer class (combat 0x32) | 0–5 |
| 0x16 | u8 | `armor` | armor class (combat 0x33; set to `Best_Armor_` by `Update_Automatic_Design_Equipment_`) | 0–6 |
| 0x17 | u8[5] | `specials` | special-system bit set, bit n = byte n/8, bit n%8 (`Set_Bit_Field_` 0x127712). Bit 11 = range ×1.5 (`Ship_Range_` 0xff496), bit 27 = structure ×3 (`Get_Design_Structure_` 0x582bf) | |
| 0x1c | slot[8] | `weapons` | weapon slots, 8 bytes each, see below | |
| 0x5c | u8 | `picture` | ship picture index (0x2d colony, 0x2e outpost, 0x2f transport) | 0–47 |
| 0x5d | u8 | `builder` | player (or monster owner id) that built the design; `Create_Ship_` sets it to the owner; copied to combat 0x138 | 0–14, always = owner in saves |
| 0x5e | i16 | `cost` | production cost (`Cost_Of_Design_`); `Kill_Ships_` uses cost/4 as scrap value; refit stores the refit cost here. `Design_Cost_` 0x77c97 reads only the low byte (game quirk) | 0–3503 |
| 0x60 | u8 | `combat_speed` | `Current_Design_Combat_Speed_` (`Update_Player_Design_`); combat 0x3a | 0–24 |
| 0x61 | u16 | `build_date` | low 16 bits of `stardate` when the design was saved (`Update_Player_Design_`; values exceed 32767, read it unsigned); `Make_Ship_Arrive_At_Star_` zeroes it for owner 13 (Eel) **?** | 0 or 34952–35480 |

Weapon slot (at 0x1c + 8·i; `Design_Weapon*_` accessors, `Load_Combat_Ship_`):

| Off | Size | Name | Meaning | Seen |
|---|---|---|---|---|
| +0 | i16 | `weapon` | weapon id (row of the 0x1c-byte weapon table at 0x17f800); 0 = empty slot (also -1 in some non-combat designs) | -1–45 |
| +2 | u8 | `count` | mounts in the slot | 0–20 |
| +3 | u8 | `working` | mounts still working; `Update_Player_Design_` sets = count, `Unload_Combat_Ship_` 0x4b9e1 writes back, `Repair_Ship_Full_` 0x581f3 restores | = count in all saves |
| +4 | u8 | `arc` | firing arc bit mask; 0x10 is mapped to 0x0f at combat load | 0, 1, 2, 15, 16 |
| +5 | u16 | `mods` | weapon modification bit mask (`Design_Weapon_Specials_`) | |
| +7 | u8 | `ammo` | ammo/shots for limited weapons (`Design_Weapon_Ammo_`) | 0–10 |

## Ship (129 bytes, 0x81; 500 records, `NUM_SHIPS` in use)

`Init_Ships_Array_` (0x12268) zeroes all 500 and sets every slot to
status 5, location -1, officer -1. `Create_Ship_` reuses the first slot
with status 5 (or the first ≥ `NUM_SHIPS`) and raises `NUM_SHIPS`, so
records below `NUM_SHIPS` may be free (554 of 1381 in the saves). A free
slot keeps stale design/position bytes; only status, location and officer
are reset (`Delete_Ship_` 0xaf9ec).

| Off | Size | Name | Meaning | Seen (non-free) |
|---|---|---|---|---|
| 0x00 | design | `design` | 99-byte ship design above | |
| 0x63 | i8 | `owner` | player 0..7; 8 Antarans, 9 Guardian, 10 Amoeba, 11 Crystal, 12 Dragon, 13 Eel, 14 Hydra; `Ship_Owner_` 0x77ff5 aborts if < 0 or > 15 | 0–14, players < `NUM_PLAYERS` |
| 0x64 | u8 | `status` | 0 at star, 1 in transit, 2 departing (first turn of a move), 4 refitting (`Add_Refited_Ship_To_Queue_` 0xb2190), 5 free slot, 6 under construction (`Colbldg_Create_Ship_` 0xaf76f), 7 scrapped and 8 destroyed, both removed by `Make_Scrap_Ships_Dead_` 0xedf92, 9 retreating (`Process_Retreating_Ships_` 0xe6e52), 10 attacking Antares (`Reset_Ships_Attacking_Antares_` 0xa1b8e). Values < 3 are "active in space" (`N_Player_Ships_` 0x79b73) | 0, 1, 2, 5, 6 |
| 0x65 | i16 | `location` | status 0/6: star index; status 1: 500 + destination star; status 2: 1000 + destination star (`Ship_Status_` 0x78bec, `Update_Ship_ETAs_` 0x10041c; `Move_All_Ships_Toward_Stars_` 0xffeea turns 1000+ into 500+); -1 free. `NUM_STARS` means "nowhere" during retreat | star < `NUM_STARS` in all |
| 0x67 | i16 | `x` | map x; equals the star's x when at a star | 82–1441, ≤ `MAP_MAX_X` |
| 0x69 | i16 | `y` | map y | 25–1136, ≤ `MAP_MAX_Y` |
| 0x6b | u8 | `navigator` | 1 if the fleet had a navigator officer when ordered: movement then ignores nebula slowdown (`Ships_Try_To_Move_To_` 0xff799 byte 7, `Make_Ships_Move_To_` 0xffd08) | 0, 1 |
| 0x6c | i8 | `speed` | move steps per turn (one step = 30 map units, `Move_Player_1_Turn_To_Star_` 0xebb0c); -1 = arrive next turn (wormhole/star gate). Written by the patch stub at 0x16915c called from `Make_Ships_Move_To_` | 0–10 |
| 0x6d | u8 | `eta` | turns to arrival; decremented each turn, recomputed by `Update_Ship_ETAs_` | 0 at star, 1–8 moving |
| 0x6e | u8 | `shield_damage` | % of shield system lost, kept between combats (combat 0x134/0xfe; `Recharge_Shields_` uses combat 0x134) **?** | 0 |
| 0x6f | u8 | `drive_damage` | % of drive lost (combat 0x100/0xca, `Calc_Current_Speed_`) **?** | 0 |
| 0x70 | u8 | `computer_damage` | computer points lost (combat 0x102, `Computer_Bonus_`); cleared by `Unload_Combat_Ship_` **?** | 0 |
| 0x71 | u8 | `crew_level` | 0–5 from `experience` thresholds plus a racial bonus (`Calc_Ship_Level_` 0x147e7); text via `Crew_Description_String_` 0x753f2 | 0–4 |
| 0x72 | i16 | `experience` | crew experience; `Do_XP_For_Ship_` 0x149d5, capped by the table at 0x17d186 | 0–500 |
| 0x74 | i16 | `officer` | leader index (0..66) captaining the ship, -1 none. Back-pointer: leader.`location` = ship (verified in all saves) | -1, 0–56 |
| 0x76 | u8[5] | `damaged_specials` | specials knocked out, same bit layout as `specials` (`Unload_Combat_Ship_`, cleared by `Repair_Ship_Full_`) | all 0 |
| 0x7b | i16 | `armor_damage` | armor hit points lost (`Current_Ship_Hits_` 0x5f2c3) | 0 |
| 0x7d | i16 | `structure_damage` | structure hit points lost | 0–244 |
| 0x7f | u8 | `ai_mission` | AI order: 0 none (reset on arrival and when not moving), 3 staging (`Move_To_Stage_Points_`), 4 colony escort (`Get_Best_Escorted_Colony_Ship_`), 5 intercept (`Interceptions_`), 6 sneak attack (`Launch_Sneak_Attack_`), 7 explore (`Move_To_Unexplored_Stars_`); 1/2 come from `Find_Best_Attack_From_Star_` **?** | 0–6 |
| 0x80 | u8 | `just_built` | AI flag for ships built this turn (`Redirect_Newly_Built_Ships_`), cleared by `Unmark_Just_Built_AI_Ships_` 0x100828 | 0, 1 |

## Leader (59 bytes, 0x3b; 67 records)

The array is loaded whole from `HERODATA.LBX` (or the G/F/S/I language
variant) by `Init_Leaders_` (0x1307f, record size 0x3b, 67 records), so this
is also the LBX record format. Index is fixed per hero; 65 is Loknar, 66
the Antaran Xyphys (excluded from hiring by `Leader_Available_For_Hire_`
0x97c2d). `Officer_Is_Female_` (0x99443) works on the index.

| Off | Size | Name | Meaning | Seen |
|---|---|---|---|---|
| 0x00 | char[15] | `name` | NUL-terminated name | readable |
| 0x0f | char[20] | `title` | NUL-terminated title (may be empty) | readable |
| 0x23 | u8 | `type` | 0 ship officer (captain), 1 colony leader | 0, 1 |
| 0x24 | i16 | `experience` | level thresholds 60/150/300/500/1000 (`Get_Officer_Base_Level_` 0x93d4b); capped at 30000 (`Give_Officer_XP_` 0x97c64) | 0–1000 |
| 0x26 | u32 | `common_skills` | general skill bit set; skill i tests the mask at 0x17d219 + i·0x12 (`Officer_Has_General_Skill_` 0x9467d) | |
| 0x2a | u32 | `special_skills` | type-specific skill bit set, same mask table (`Officer_Has_Special_Skill_` 0x9469e); bits 8/9 are navigator skills (`Ships_Try_To_Move_To_`) | |
| 0x2e | u8[3] | `techs` | tech applications given to the hiring player, 0 none (`Give_Leader_Specials_To_Player_` 0x99223); Loknar's two are randomised in `Init_Leaders_` | 0–195 |
| 0x31 | u8 | `picture` | portrait index (`Load_Officer_Picture_`) | = record index |
| 0x32 | i16 | `skill_value` | `Officer_Skill_Value_` 0x946b7 computed at init; hire cost = value·(level+1)·10 (`Officer_Cost_` 0x97a2d) | 3–37 |
| 0x34 | u8 | `rank_name` | index into `_officer_level_names`, read only by `Random_New_Officer_Popup_` 0xc7ada **?** | 0 |
| 0x35 | i16 | `location` | status 1: ship index (type 0) or star index (type 1); status 5: star where marooned; else -1 | -1–161 |
| 0x37 | i8 | `eta` | status 1: turns until the leader reaches its post (no effect before 0); status 4: turns the offer has waited, expires above 29 (`Decrement_Officer_ETA_` 0x934cf) | 0–28 |
| 0x38 | u8 | `level_up` | pending "gained a level" report (`Give_Officer_XP_`, `Officer_Made_Level_Report_` 0x949a7) | 0, 1 |
| 0x39 | i8 | `status` | -2 dead (`Kill_Officer_` 0x941c6), -1 not hired/available, 0 in owner's pool, 1 assigned, 2 dismissed this screen (`Move_Officer_To_Limbo_` 0x943a0), 3 counted as alive by `Officer_Status_Ok_` 0x9773f (no writer found) **?**, 4 offered for hire (`Set_Officer_To_Player_` 0x9718f with hire=0), 5 marooned at a star (`Assign_Marooned_Heroes_` 0x8bb51) | -2, -1, 0, 1, 4, 5 |
| 0x3a | i8 | `owner` | owning player, -1 none | -1–7 |

A colony leader's post is also recorded in the star: star+0x42+player =
leader index (`Assign_Leader_To_Star_` 0x98ceb; verified in all saves).

## Moveable box (28 bytes, 0x1c; 5 records) and order

Draggable main-screen windows. `Init_Moving_Boxes_` (0x79756) zeroes them
and sets `id`; `Init_Fleet_Movement_Box_` (0x7971f) and
`Init_System_Display_Box_` (0x797dd) set sizes. Box 0 is the system
display box, box 2 the fleet movement box, box 1 is tied to the galaxy map
fields (`Clear_Moveable_Box_Fields_` 0x9086b); boxes 3 and 4 are unused in
the saves.

| Off | Size | Name | Meaning |
|---|---|---|---|
| 0x00 | i16 | `id` | box number 0–4 |
| 0x02 | i16 | `content` | what the box shows (box 0: star id, `Do_System_Popup_` 0x83669); open while `content > _box_off_flag[id]` (`Moveable_Box_Selected_` 0x918d5) |
| 0x04 | i16 | `height` | open height |
| 0x06 | i16 | `width` | open width (box 2 uses globals instead, `Moveable_Box_Dimensions_` 0x91776) |
| 0x08 | i16 | `x` | left; clamped to 22..0x20f-width (`Move_Moveable_Box_` 0x91349) |
| 0x0a | i16 | `y` | top; clamped to 22..0x1a5-height |
| 0x0c | i16 | `dragging` | 1 while being dragged |
| 0x0e | i16 | `drag_field` | input field id of the drag handle; -1000 when cleared |
| 0x10 | i16 | `field2` | field id, -1000 when cleared **?** |
| 0x12 | i16 | `field3` | field id, -1000 when cleared **?** |
| 0x14 | i16 | `closed_width` | width when not open |
| 0x16 | i16 | `closed_height` | height when not open |
| 0x18 | i16 | `drag_dx` | pointer offset during a drag |
| 0x1a | i16 | `drag_dy` | |

`moveable_box_order` is `i16[5]`: box ids in drawing (z) order, first is
bottom; `Moveable_Box_Occluded_` (0x917dd) tests boxes earlier in the list.
Seen: a permutation of 0..4 in every save.

## Open questions

- Ship status 3 and leader status 3: compared against but no writer found.
- Weapon `arc` bit meanings and `mods` bits need the weapon/combat specs.
- Ship 0x6e/0x6f/0x70 system attribution comes from the combat routines
  that read the combat-record fields, not from names; all zero in the saves.
- Player design slot 5 is the refit source in `Add_Refited_Ship_To_Queue_`
  (player+0x515); in the saves it holds the colony ship/transport.
