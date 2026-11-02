# Saved game: player records

Block `player` of `SAVEn.GAM` (see `save.md`): 8 records of 0xea9 (3753)
bytes at file offset 0x1aa0e; the first `NUM_PLAYERS` are in use, the rest
are all zero in every shipped save. `_player` (ELF 0x197f98) points to the
array; code addresses a field as `_player + i*0xea9 + off`, or takes a record
pointer (`Update_Player_Stats_`, `Player_Maintenance_`, `Settle_Pop_`).

There are no types in the debug info. Every field below comes from how
routines use it, checked against the ten shipped saves (66 records). Little
endian; `s8/s16` signed. Confidence: **H** high, **M** medium, **L** low
(name is a guess), **?** unknown. Arrays `[8]` are indexed by the *other*
player number `j` (the entry `[i]` of record `i` is "self").

## Region map

| Off | Size | Region |
|---|---|---|
| 0x000 | 0x0c4 | identity, economy, settlers, per-turn totals |
| 0x0c4 | 0x053 | tech field status [83] |
| 0x117 | 0x0d4 | tech application status [212] |
| 0x1eb | 0x13b | research, victory, AI themes, hyper levels, unused |
| 0x326 | 0x252 | ship designs, 6 × 99 bytes |
| 0x578 | 0x00c | per-hull-size ship counters |
| 0x584 | 0x08b | contacts, ranges, treaties pops, AI scalars |
| 0x60f | 0x290 | diplomacy tables, mostly `[8]` per other player |
| 0x89f | 0x01f | race picks (current) |
| 0x8be | 0x01f | race picks (evolution base) |
| 0x8dd | 0x002 | evolution flag, unknown |
| 0x8df | 0x578 | history graphs, 4 × 350 bytes |
| 0xe57 | 0x052 | spies, UI and misc flags, tribute, unused tail |

## 0x000 identity and economy

| Off | Type | Name | Notes (evidence) | C |
|---|---|---|---|---|
| 0x000 | u8 | ? | always 0 | ? |
| 0x001 | char[20] | emperor name | NUL-terminated; `Init_Players_` copies a RACENAME.LBX record (size 0x14) to +1; `lea [p+1]` in `Get_Diplomacy_Statement_` etc. "Karaaw Hrik" in SAVE7–9 | H |
| 0x015 | char[15] | race name | from `_race_names[race]` in `Init_Players_` (0x12a7a) | H |
| 0x024 | u8 | eliminated | 1 = out (BC 0, home 0xffff); `Apply_All_Player_Changes_` skips such players | H |
| 0x025 | u8 | race | 0..12 (Alkari 0 … Trilarian 12), unique per game (`Init_Players_`) | H |
| 0x026 | u8 | colour/banner | 0..7, unique (`Init_Players_`, `Replace_Colgcbt_Color_With_Player_Colors_`) | H |
| 0x027 | u8 | AI personality | 0..5, table 0x181090[race*10+…] (`Init_NPC_Personalities_Objectives_Themes_`); `Get_Player_Diplomacy_Personality_` returns 6 if it is 4 and 0x727[j]==1 | H |
| 0x028 | u8 | AI objective / human | 0..5 weighted pick (same routine); 100 = human player (`cmp [p+0x28],0x64` everywhere) | H |
| 0x029 | s16 | home planet | planet index, 0xffff when eliminated (`Home_Planet_`, `Assign_New_Home_Planet_`) | H |
| 0x02b | s16 | network player id | `Net_Id_To_Player_Id_`, `Send_Drop_Out_Info_` writes -1; 0 in saves | M |
| 0x02d | u8 | turn done | set by `Do_Next_Turn_`, cleared by `Next_Turn_`, read by `Hotseat_Player_Status_` | M |
| 0x02e | u16 | legacy research points | `Load_Game_` (0x1158e) moves a non-zero value to 0x1eb and clears it; 0 in saves | H |
| 0x030 | u8 | research breakthrough pending | `Check_For_Research_Breakthrough_`, `Has_Research_Breakthrough_` | M |
| 0x031 | u8 | tax rate | 0/10/20 seen (`Tax_Rate_Popup_`, `Evaluate_Tax_Rate_Input_`) | H |
| 0x032 | s32 | BC reserve | starts 50 (`Init_Players_`); += net income in `Apply_All_Player_Changes_`; "Reserve:" in `Draw_Empire_Info_` | H |
| 0x036 | s16 | freighters total | +5 per freighter fleet (`Apply_Production_`, `Freighter_Cheat_`) | H |
| 0x038 | s16 | freighters free | total − 5·settlers − food shipped (`Pass_Out_Imports_` 0xdfa90); "Freighters:" | H |
| 0x03a | s16 | command points total | main screen prints 0x3a−0x3c (`Print_Main_Screen_Data_` 0x88125) | M |
| 0x03c | s16 | command points used | compared with 0x3a in `Assign_Empire_Building_`, `Compute_Needs_` | M |
| 0x03e | s16 | food shipped by freighters | `Pass_Out_Imports_` 0xdfa8c | M |
| 0x040 | s16 | settlers in transit | count of 0x042 entries (`Settle_Pop_`, `Move_Settlers_`) | H |
| 0x042 | u32[25] | settlers | packed: bits 0–7 source colony, 8–15 target planet, 16–19 and 24–25 pop record bits (race), 20–23 ETA turns. Stale entries beyond the count stay. 25 = room up to 0xa6 (capacity not seen in code) | H/M |
| 0x0a6 | s16 | population | Σ colony+0x0a (`Update_Player_Stats_`); "Population:"; council votes | H |
| 0x0a8 | s16 | food produced | Σ colony+0xe7 | H |
| 0x0aa | s16 | industry produced | Σ colony+0xe9 | H |
| 0x0ac | s16 | research produced | colonies + ships + leaders + research treaties, cap 0x7fff; "Research:" | H |
| 0x0ae | s16 | gross income | colonies + trade treaties + tribute 0xe78 + surplus food sold (halved unless trader pick 0x8b7) | H |
| 0x0b0 | s16 | food surplus | "Food:"; recomputed in `Pass_Out_Imports_` | H |
| 0x0b2 | s16 | net income | 0xae − 0xb4; "Income:" | H |
| 0x0b4 | s32 | maintenance total | Σ of the six words below (`Compute_Player_Maintenance_` 0xe261e) | H |
| 0x0b8 | s16[6] | maintenance parts | +0xb8 colonies/buildings?, +0xba ships?, +0xbc ?, +0xbe spies (`Get_Their_Spy_Number_`), +0xc0 tribute paid, +0xc2 officers (`Officer_Maintenance_`); sums match 0xb4 in saves | M |

## 0x0c4 technology

| Off | Type | Name | Notes | C |
|---|---|---|---|---|
| 0x0c4 | u8[83] | field status | index = research field (`Init_Player_Tech_` loops 1..0x52). 0 unknown, 1 not yet available, 2 researchable, 3 researched. `Do_Colony_Calculations_` counts 3s into 0x5e8 | H |
| 0x117 | u8[212] | application status | index = tech application 0..0xd3 (table 0x17e07f, 13-byte rows). 0 not obtainable (illegal for race / not chosen), 1 obtainable, 3 known | H |
| 0x1eb | s32 | research points accumulated | compared with field cost (`Event_Twiddle_`, `Check_For_Research_Breakthrough_`); cheat sets 9999 | H |
| 0x1ef | u8 | captured Orion | `Do_1_Combat_`, `Get_Orion_Score_` | H |
| 0x1f0 | u8 | captured Antares | `Get_Winner_`, `Get_Antares_Score_` | H |
| 0x1f1 | u8 | won council vote | `Council_Accept_Ruling_`, `Won_Council_Vote_` | H |
| 0x1f2 | s16[8] | eliminations | per eliminated player, written by `Eliminate_Player_`, scored by `Get_Elimination_Score_` | M |
| 0x202 | s16 | captured colonists | `Change_Colony_Ownership_`, `Get_Captured_Colonist_Score_` | M |
| 0x203 | — | (high byte of captured colonists) | the s16 at 0x202 covers it | |
| 0x204 | u8 | combat slot | 0xff in saves; set by `Search_For_Battles_`, 0xff on elimination | L |
| 0x205 | u8 | AI ship theme | 0..3, weighted pick (`Init_NPC_…Themes_`, `Get_Ship_Class_`) | M |
| 0x206 | u8 | AI special theme | 0..6 (`Add_Theme_Special_To_Design_`) | M |
| 0x207 | 21 | ? | 0, no accesses found | ? |
| 0x21c | u8[8] | hyper-advanced levels | one per research area; `Promote_Hyper_Field_Values_` +1 each, `Miniaturization_Level_` | H |
| 0x224 | s32 | total known tech cost | `Compute_Player_Total_Known_Tech_Cost_`; history graph "technology" | H |
| 0x228 | u8 | ? | 0 | ? |
| 0x229 | s32[8] | stardate stamps | set to the start stardate for each player in `Init_Players_`; no reader found | L |
| 0x249 | 216 | unused | always 0, no accesses found | ? |
| 0x321 | u8 | research field | current topic (`Set_Initial_Tech_`, `_Tech_Select_`) | M |
| 0x322 | u8 | research application | read by `Check_Hot_Keys_`; values 2..0xd0 | L |
| 0x323 | u8 | ? | 0/1 | ? |
| 0x324 | s16 | fuel range (parsecs) | `Init_Players_` from `Best_Ship_Fuel_`; `Ship_Range_`, range checks | H |

## 0x326 ship designs

6 records of 99 bytes (`Init_Ship_Designs_`: `0x326 + k*99`; slot 5 at 0x515
is the AI transport, cleared for humans). Written by `Update_Player_Design_`
from the design screen buffer `_design`.

| Rel | Type | Field | Source in `_design` |
|---|---|---|---|
| 0x00 | char[16] | name | string |
| 0x10 | u8 | hull size | +0xe5 |
| 0x11 | u8 | ? (0..2; transport 2) | not written here |
| 0x12 | u8 | shield | +0x11 |
| 0x13 | u8 | drive | +0x1b |
| 0x14 | u8 | FTL speed | `Get_FTL_Speed_` |
| 0x15 | u8 | computer | +0x2b |
| 0x16 | u8 | armor | +0x31 |
| 0x17 | u8[5] | specials bitfield | bits from +0xb5 (8 words), `Set_Bit_Field_` |
| 0x1c | 8 × 8 | weapon slots | s16 weapon (+0x35), u8 count (+0x45), u8 count again (current?), u8 arc (+0x55), u16 mods (+0x5d), u8 ammo (+0xad) |
| 0x5c | u8 | picture | +0xe7 |
| 0x5d | u8 | ? (0..7) | not written here |
| 0x5e | s16 | cost | +0xf1 |
| 0x60 | u8 | combat speed | `Current_Design_Combat_Speed_` |
| 0x61 | u16 | stardate designed | `_stardate` |

0x578 `s16[6]`: per hull size counter used to number default ship names
(`Create_Ship_` 0xaf8f5, `inc` at 0xaf9d3). 0 in every save. **M**

## 0x584 contacts and AI scalars

| Off | Type | Name | Notes | C |
|---|---|---|---|---|
| 0x584 | u8[8] | contact | 1 = in contact (extended range); self = 1 (`Compute_Contacts_`, `Contact_With_Player_`) | H |
| 0x58c | u8 | lost-contact mask | bit j set when contact with j is lost (`Compute_Contacts_` 0xeb44b) | M |
| 0x58d | u8[8] | lost-contact count | += 1 at the same place; 0..3 | M |
| 0x595 | u8[8] | in normal range | `Star_In_Normal_Range_Of_Player_` result | M |
| 0x59d | u8 | ? | 0; `Do_AI_Leaders_`, `Colony_Building_Score_` | ? |
| 0x59e | s8 | AI staging star | -1 init; `Compute_Defensive_Stage_Points_`, `Launch_Sneak_Attack_` | M |
| 0x59f | u8 | best warp drive | `Best_Warp_Drive_` (`Init_Player_Tech_`) | H |
| 0x5a0 | u8 | FTL speed | `Calc_Player_FTL_Speed_` | H |
| 0x5a1 | s8 | officer for hire | leader index, -1 none (`Has_Officer_For_Hire_`) | H |
| 0x5a2 | s16 | trade pops | `Compute_N_Trade_And_Research_Pops_` | M |
| 0x5a4 | s16[8] | trade treaty value | `Start_Trade_Agreement_`; added to income if 0x62f[j] | M |
| 0x5b4 | s16[8] | trade treaty goal? | `Start_Trade_Agreement_` | L |
| 0x5c4 | s16 | research pops | as 0x5a2 | M |
| 0x5c6 | s16[8] | research treaty value | added to research if 0x637[j] | M |
| 0x5d6 | s16[8] | research treaty goal? | `Start_Research_Agreement_` | L |
| 0x5e6 | s16 | ? | 0 | ? |
| 0x5e8 | s16 | fields researched | count of 0xc4 == 3 | H |
| 0x5ea | s16 | fleet size score | Σ 1<<hull of combat ships | H |
| 0x5ec | s32[8] | fleet strength | `Compute_AI_Data_`, `Fleet_Comparison_` | M |
| 0x60c | s16 | colonies with pop ≥ 5 | `Do_Colony_Calculations_` | H |
| 0x60e | u8 | ? | 0; read as `[p+j+0x60e]`, may be 0x60f[j-1] | ? |

## 0x60f diplomacy tables

`[8]` bytes unless noted; `w[8]` = s16[8], `d[8]` = s32[8]. Most are
symmetric, written for both i→j and j→i. `Init_Diplomatic_Relations_` sets
the initial values; `Clear_Diplomacy_Messages_` clears the per-turn ones.

| Off | Type | Name | Notes | C |
|---|---|---|---|---|
| 0x60f | | war-declared flag | set in `Declare_War_` when the target is human | M |
| 0x617 | s8 | relations | -100..+100; start `_starting_relations[race][race]`; −75−rnd(25) on war | H |
| 0x61f | s8 | base relations | same start value; −10 on war | M |
| 0x627 | u8 | treaty status | 0 none, 1 and 2 treaties (`Start_Treaty_` ×1.5 / ×2 peace duration; probably NAP / alliance), 3 timed (reset to 0 when 0x72f runs out), 4 war, 5/6 war involving a human (by difficulty) | M |
| 0x62f | | trade treaty | 0/1 (`Break_Trade_`) | H |
| 0x637 | | research treaty | 0/1 (`Break_Research_`) | H |
| 0x63f | w[8] | tribute | `Break_Tribute_`, `Tribute_Received_From_` | M |
| 0x64f | | message this turn | cleared per turn | M |
| 0x657 | | message flags | cleared per turn | M |
| 0x65f | w[8] | message parameter | copied to 0x737 each turn | M |
| 0x66f | w[8] | ? | `Get_Diplomacy_Statement_` | ? |
| 0x67f | w[8] | ? | same | ? |
| 0x68f | w[8] | modifier 1 | -200 on war; `Modifier_Diplomacy_Adjustments_`, `Limit_Treaty_Modifiers_` | M |
| 0x69f | w[8] | modifier 2 | -200 on war | M |
| 0x6af | w[8] | modifier 3 | -200 on war | M |
| 0x6bf | w[8] | modifier 4 | -130 on war | M |
| 0x6cf | | ? (0..2) | `Check_Treaty_Proposal_` | ? |
| 0x6d7 | s8 | ? (-15..0) | `Break_*`, tech exchange reactions | L |
| 0x6df | | ? (0..8) | treaty proposals | ? |
| 0x6e7 | | ? (0..2) | treaty proposals | ? |
| 0x6ef | w[8] | war losses | `War_Loss_Comparison_`, `End_Of_Combat_`; 0 on war | M |
| 0x6ff | w[8] | war losses 2 | `Resolve_Bomb_Hit_`, `Change_Pop_Ownership_` | L |
| 0x70f | | ? | cleared by war/peace | ? |
| 0x717 | | turns at war | +1 (≤250) while status ≥ 4 | M |
| 0x71f | | demand/threat state | 0..2 | L |
| 0x727 | | ? (0/1) | turns personality 4 into 6 | L |
| 0x72f | | treaty timer | +peace duration on treaty, −1 per turn | M |
| 0x737 | w[8] | last message parameter | | M |
| 0x747 | | ? (0..8) | `Break_Treaties_` | ? |
| 0x74f | | previous treaty status | mirrors 0x627 | M |
| 0x757–0x76f | 4 × [8] | per-turn diplomacy scratch | cleared per turn; 0 in saves | L |
| 0x777 | w[8] | per-turn demand | `Set_Demands_` | L |
| 0x787 | | alliance call? | `Honor_Alliances_`; 0 in saves | L |
| 0x78f | d[8] | tech exchange value | `NPC_Tech_Exchange_Check_` | L |
| 0x7af | | tech exchange application | values ≤ 0xd3 | L |
| 0x7b7 | w[8] | ? (-1..3) | `Set_Demands_`, `Npc_Give_Gift_` | ? |
| 0x7c7 | s16 | AI war target | -1 init (`Get_AI_Target_`) | L |
| 0x7c9 | u8 | ? | | ? |
| 0x7ca | u8 | ? | | ? |
| 0x7cb | u8 | ? | 0xff init | ? |
| 0x7cc | w[8] | ? | `Determine_Good_Message_` | ? |
| 0x7dc | | ? | same | ? |
| 0x7e4 | | ? | 0xff init and on war | ? |
| 0x7ec | s16 | ? | | ? |
| 0x7ee | s8 | ? (-20..0) | `Check_Treaty_Proposal_` | ? |
| 0x7f6 | | ? | 0xff init | ? |
| 0x7fe | | ? (0/1) | `Break_Treaties_` | ? |
| 0x806 | | ? | `Allocate_AI_Spies_` | ? |
| 0x80e | | ? | `AI_May_Attack_Player_` | ? |
| 0x816 | u8 | ? | `NPC_Sneak_Attacks_` | ? |
| 0x817 | w[8] | last proposal turn | `Clear_Last_Proposal_Turn_` | M |
| 0x827 | w[8] | personality bias to humans | `Init_Diplomatic_Relations_`: -3/-2/0/0/2/5 by personality | M |
| 0x837 | w[8] | sneak attack star? | -1 init (`Set_Opportunity_Attacks_`) | L |
| 0x847 | w[8] | sneak attack value? | | L |
| 0x857 | d[8] | sneak attack strength? | | L |
| 0x877 | 16 | ? | 0, no accesses found | ? |
| 0x887 | | ? | `NPC_Sneak_Attacks_` | ? |
| 0x88f | | threat turn | `Clear_Threat_Turn_` | L |
| 0x897 | s8 | delayed diplomacy order | -1 none; `Load_Game_` resets to -1 | M |

## 0x89f race picks

Two copies of the 31-byte RACESTUF.LBX record (entry 7, size 0x1f). After
`Init_Players_` levels 1..9 are mapped through table 0x17d1f9 to the stored
values below. 0x89f is the live set, read everywhere. 0x8be is the base
set used by Evolutionary Mutation (`Setup_Evolutionary_Upgrade_`); for
humans it equals 0x89f, for AIs `Race_Customization_` adds different
difficulty bonuses to each copy.

| Idx | Off | Pick | Values seen |
|---|---|---|---|
| 0 | 0x89f | government | 0,1,2,4,6; +1 when field 6 is learned (`Init_Player_Tech_`) |
| 1 | 0x8a0 | population | -50, 50, 100 |
| 2 | 0x8a1 | farming | 2, 4 (half food units?) |
| 3 | 0x8a2 | industry | 1, 2 |
| 4 | 0x8a3 | science | 1 |
| 5 | 0x8a4 | money | 1, 2 |
| 6 | 0x8a5 | ship defense | 25, 50 |
| 7 | 0x8a6 | ship attack | 20, 50 |
| 8 | 0x8a7 | ground combat | -10, 20 |
| 9 | 0x8a8 | spying | -10, 20 |
| 10 | 0x8a9 | low-g | 0/1 (`Gravity_Player_Production_Factor_`) |
| 11 | 0x8aa | high-g | 0/1 (Bulrathi; `Enforce_Gravity_`) |
| 12 | 0x8ab | aquatic | 0/1 (`Player_Effective_Climate_`) |
| 13 | 0x8ac | subterranean | 0/1 |
| 14 | 0x8ad | large home world | 0/1 (`Modify_Home_Worlds_`) |
| 15 | 0x8ae | rich/poor home world | 0/1 |
| 16 | 0x8af | artifacts world | 0/1 |
| 17 | 0x8b0 | cybernetic | 0/1 (`Repair_All_Combat_Ships_`) |
| 18 | 0x8b1 | lithovore | 0/1 (food) |
| 19 | 0x8b2 | repulsive | 0/1 (diplomacy) |
| 20 | 0x8b3 | charismatic | 0/1 |
| 21 | 0x8b4 | uncreative | 0/1 (`Init_Player_Tech_`) |
| 22 | 0x8b5 | creative | 0/1 (all applications of a field) |
| 23 | 0x8b6 | tolerant | 0/1 |
| 24 | 0x8b7 | fantastic traders | 0/1 (full food sale price) |
| 25 | 0x8b8 | telepathic | 0/1 (`Player_Can_Mind_Control_Colony_`) |
| 26 | 0x8b9 | lucky | 0/1 (`Determine_Lucky_Players_Events_`) |
| 27 | 0x8ba | omniscient | 0/1 (`Player_Is_Omniscient_`) |
| 28 | 0x8bb | stealthy ships | 0/1 |
| 29 | 0x8bc | trans-dimensional | 0/1 (speed) |
| 30 | 0x8bd | warlord | 0/1 (officer level, infantry) |

Names for 2–5 and 10–30 follow the in-game pick order and the routines
that read them; the value encoding of 2–5 is **M**. Note
`Init_NPC_Personalities_…` tests 0x8a5 for 20/40 and 0x8a6 for 25/50,
the reverse of what the saves hold: port as is.

0x8dd `u8`: evolutionary upgrade taken (`Setup_Evolutionary_Upgrade_`,
`Has_Evolutionary_Upgrade_`). 0x8de `u8`: unknown, 0.

## 0x8df history

Four ring buffers of 350 bytes, one sample per turn, slot
`DAT_0017d636` (a global, not saved here), rescaled so every value fits
250 (`Record_History_`, read by `Draw_Histories_`):

| Off | Graph | Source |
|---|---|---|
| 0x8df | fleet | Σ 10<<(hull+1) of own ships |
| 0xa3d | technology | 0x224 |
| 0xb9b | population | 0xa6 |
| 0xcf9 | buildings | `Compute_Total_Building_Costs_` |

## 0xe57 tail

| Off | Type | Name | Notes | C |
|---|---|---|---|---|
| 0xe57 | u8[8] | spies | spies placed against player j (`Set_My_Spy_Number_`, `Take_Their_Spy_`) | M |
| 0xe5f | u8 | info screen buttons | 0x4f init, 0x0f/0x1f for humans (`Set_Plyr_Hist_Btns_`) | M |
| 0xe60 | u8 | ignoring diplomacy | `Is_Ignoring_`, `Stop_Ignoring_` | M |
| 0xe61 | 8 | ? | 0 | ? |
| 0xe69 | u8 | cheated | `Set_Player_Cheated_Flag_` | H |
| 0xe6a | u16 | officer offer stardate | `Chance_To_Hire_Hero_`, `Generate_Random_Officer_`; values ≈ stardate | M |
| 0xe6c | 5 | ? | 0 | ? |
| 0xe71 | s8 | council vote | -1 init; player voted for (`Council_Player_Vote_`) | M |
| 0xe72 | s8 | surrender target | -1 in saves (`Surrender_Empire_`, `Do_Surrenders_`) | L |
| 0xe73 | s16 | lucky event counter | `Determine_Lucky_Players_Events_` | L |
| 0xe75 | u8 | a leader with Galactic Lore serves (0/1); `Star_Owner_` then counts every colony | `Hire_Officer_`, `One_Leader_With_Galactic_Lore_` | L |
| 0xe76 | u8 | cheat warning | `Warn_About_Cheating_Players_` | L |
| 0xe77 | u8 | cheat flag | `Check_Hot_Keys_` | L |
| 0xe78 | s32 | tribute received | Σ `Tribute_Received_From_` (`Update_Player_Stats_`) | H |
| 0xe7c | 45 | unused | 0, no accesses found | ? |

## Method notes

Accesses were collected by a symbolic pass over `build/orion2.dis`
(tracking `imul r,0xea9` and `[0x197f98]`), then confirmed by decompiling
the routines named above. Accesses through a passed record pointer are only
partly covered by that pass, so "no accesses found" means "none found by
that pass and by grepping the displacement".
