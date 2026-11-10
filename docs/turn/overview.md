# The turn: overview

Map of everything that runs between pressing TURN on the galaxy screen and
the next `Main_Screen_`: `Do_Next_Turn_` → `Next_Turn_` → `Next_Turn_Calc_`
→ the begin-of-turn report phase. It is the plan for porting the turn, not
yet a spec of each routine: every phase below names its routines in call
order with what they touch, and later specs under `docs/turn/` should
replace the rows as each phase is ported.

Addresses are ELF (Watcom + 0x10000). Modules come from the Watcom debug
info. Save-state names follow `docs/formats/save-*.md` and `internal/save`'s field names.

Conventions in the tables:

- **RNG**: `R!` calls `Random_` directly, `R` only somewhere below, `-` never.
  "Restored" means the routine brackets its draws with
  `Get_Random_Seed_`/`Set_Random_Seed_` and leaves the seed unchanged.
- **Ported**: a Go function in `internal/game` documented as that routine.
- **Insn / n**: the routine's own instruction count from `build/orion2.dis`;
  `sub` = the whole reachable subtree with shared code counted once.
- Loops over players, ships, colonies and stars run from the last record
  down unless a row says otherwise; the direction is part of bit-exactness.

## How the map was made, and its blind spots

The call graph comes from the `call` instructions in `build/orion2.dis`,
grouped by ELF symbol, with RNG reachability computed over it; meanings
come from Ghidra pseudocode via REA. From `Next_Turn_Calc_` about 2280
routines are reachable (engine and tactical combat UI included); the game
logic is roughly 1000 routines and 100–120k instructions.

Known blind spots of a symbol-based call graph, found while mapping:

- **Fall-through and jumps into other symbols.** `NPC_Diplomacy_` has no
  `ret` and falls into `NPC_Tech_Exchange_`, whose loop sits under the
  `NPC_To_Human_Diplomacy_` symbol (two `Random_` calls hidden).
  `Npc_Diplomacy_Screen_` 0x1afa6 is a prologue whose body is filed under
  `Npc_Display_Diplomacy_Messages_` 0x1a507. `Reset_One_Shot_System_Special_`
  jumps into patch code under `Multi_Player_Screen_` (0xf4f74). Watcom
  shared epilogues (`Do_Surrenders_`, `Surrender_Star_`, several DIPLOMAC
  routines) are harmless. Port each routine from its assembly range, not
  its symbol extent.
- **Function pointers.** `Kill_Buildings_` calls `Phase1/2/3_Building_`
  through a pointer; `qsort_` comparators likewise.
- **Ghidra misreads** (dropped watcall arguments, `&&` chains reordered,
  blocks removed as unreachable in `Reward_Check_`): use the assembly where
  call order or evaluation order decides RNG draws.

## The whole turn at a glance

```
Main_Screen_  TURN → Assert_Settings_ (empty) → Do_Next_Turn_ → screen 0xc
Screen_Control_ 0xc → Next_Turn_
    Do_Autosave_ (stardate % 4 == 0 and autosave on)
    Confirm_Planet_And_Star_Index_Link_ (debug check)
    Next_Turn_Calc_                       ; 50 steps, phases B..F interleaved;
        an empty auto function (0xfe8d4) for game types 0/1 during the
        steps, the arrow pointer afterwards unless the turn is automatic
    end of game checks → screen 10, else screen 0x27
Screen_Control_ 0x27 → Reports_Screen_ → Do_Begin_Of_Turn_
    Begin_Of_Turn_ → Main_Screen_Report_Handler_ → Display_Report_ loop
        (Antarans, GNN, reports, tech picks, exploration, colonisation,
         officers, AI diplomacy screens, turn summary, colony prompts;
         community patch 1.51's order)
    auto next turn or → Main_Screen_
```

The phases are interleaved in `Next_Turn_Calc_`, so the sections below are
grouped by subject, and the call-order table in phase 0 says which section
holds each step:

- 0 — entry, `Next_Turn_`, the `Next_Turn_Calc_` call order (this file).
- B — AI: data, leaders, building needs, colony AI, fleet moves, AI colonisation, tech choice.
- C — Diplomacy and council.
- D — Economy, movement and colonies: spies, player and colony changes, surrenders, blockades, settlers, slot reservation, rebellions.
- E — Combat (`Search_For_Battles_`).
- F — Events, officers and bookkeeping.
- 10 — Begin of the new turn: the report phase and the AI diplomacy screen.

## The test pair t1

`build/turns/t1/before.GAM` (SAVE1, stardate 35362) and `after.GAM` (TURN
pressed once, an AI tribute demand accepted at the start of the new turn,
then saved). Comparing the two saves field by field gives 1743 differences:

| Block | Diffs | Produced by |
|---|---|---|
| `stardate` | 1 | +1 at the end of `Next_Turn_Calc_` |
| `random_seed` | 1 | every consuming draw of the turn and of the diplomacy screen |
| `save_game_description` | 1 | the user's save, not the turn |
| `settings` | 2 | +1 (end-of-turn wait) 1 → 0 and +0x14..0x16: not written by the turn (the user's options or the save path; open question) |
| `ship` | 913 | Owner 342 / Design 173: `Allocate_New_Ship_Slots_` re-reserving free slots and `Create_Ship_` (D); X/Y/Location/ETA/Speed/Status: movement (D3) and AI orders (B); Experience 86, CrewLevel 2: `Do_All_Ships_XP_Check_` (F); AIMission 17 (B); JustBuilt 2 (D6 new ships; `Redirect_Newly_Built_Ships_` for the human) |
| `colony` | 440 | economy and growth (D5/D6, D9/D12), slot reservations (D16), colony 54 surrender (D7), AI queues and jobs (B) |
| `player` | 289 | economy totals (D), diplomacy (C), research (D5, B), History 31 (F), tribute acceptance (10) |
| `leaders` | 38 | Experience +1 (F), ETA of offered leaders (F), leader 41 offered (B) |
| `star` | 7 | star 31 surrender (D7), AIStagePoint (B), ColonizePlayer (D16), SpecialPopup cleared by `Compute_AI_Data_` (B) |
| `msgs`, `n_msgs` | 46 | `Allocate_Msg_Slots_` + messages added by every phase |
| `n_reports`, `done_colonize_check` | 2 | `Initialize_Reports_` (D2/F), report phase |
| `bill_savegame` | 2 | `Record_History_` ring index and divisor (F) |
| `antarans` | 1 | `Antaran_Invasion_Check_` pressure counter (F) |

No battle happened in the pair (section E), but the battle search still
draws random numbers. The colony that changed owner (colony 54, star 31,
player 2 → 1) is a surrender flagged on the previous turn
(`star.SurrenderTo[2] = 9` = player 1 + 8) and carried out by `Do_Surrenders_`.

## Phase 0: from the TURN button to Next_Turn_Calc_

| Routine | Addr | Module | Purpose | Save state | RNG | Ported | Insn |
|---|---|---|---|---|---|---|---|
| `Assert_Settings_` | 0xfe8bd | RUSS.C | called by `Main_Screen_` before `Do_Next_Turn_`; a bare `ret` | — | - | no (trivial) | 1 |
| `Do_Next_Turn_(&leave, auto)` | 0x83411 | MAINSCR.C | clears `_at_least_one_report`, `_previous_report`, `_just_moved_stack`, `__animated_star = -1`; `_warn_about_cheaters = 1<<PLAYER_NUM`; picks the next screen; `_mainscr_palette = 0x65`, `_ship_icon_player_id = -1`; `Clear_System_Display_Loaded_Flag_`; manual press: `_in_auto_next_turn_mode = 0`, `Clear_Fields`, `Clear_Main_Screen_Fields_`, and when settings+1 (end-of-turn wait) is 0 a full-screen hidden exit field (`__global_exit_field`); auto press (from `Do_Begin_Of_Turn_`): `_in_auto_next_turn_mode = 1`, `Increment_Mouse_Clock_Image_`, `Get_Input` → `_auto_turn_exit`, a click aborts (screen 0, fields cleared); `*leave = 1` | hotseat: player+0x2d (turn done) of `PLAYER_NUM` = 1 | - | no | 68 |
| └ next screen | | | game_type 1 (hotseat): `Save_Hotseat_Map_Info_`, screen 0x11 (picker; it sends screen 0xc once every human is done); 2/3 (network): screen 0x25 `Net_Next_Turn_`; 0 (single): screen 0xc `Next_Turn_`; `_return_screen = 0x27` for 0/2/3 | | | | |
| `Next_Turn_` (screen 0xc) | 0x13870 | NEXTTURN.C | the turn proper, then end of game checks | player+0x2d = 0 for every player; `__old_system_index = -1` | R (via Next_Turn_Calc_; End_Of_Game_Hi_Score_) | no | 88 |
| ├ `Do_Autosave_` | 0x13822 | NEXTTURN.C | game_type 0/1 only: if settings+0xb (autosave) and `stardate % 4 == 0` (network: `% 1`) → description "(Auto Save)", `Save_Game_(9)` (SAVE10.GAM) | file only | - (Save_Game_ only reads the seed) | yes (`Save_Game_`) | 30 |
| ├ `Confirm_Planet_And_Star_Index_Link_("in next turn")` | 0x78e67 | HAROLD.C | debug check: `Planet_Number_` for every planet; duplicate orbit per star → PFMT + `hgetch` (debug prints, never fires on a sane save) | read only | - | referenced | 76 |
| ├ `Next_Turn_Calc_` | 0x136b3 | NEXTTURN.C | the whole simulation, phases 1..9 below | everything | R | no | 82 (all reachable: ~2280 routines) |
| ├ `_current_screen = _return_screen` (0x27) | | | | | | | |
| └ end of game (skipped for game_type 2/3 and `_all_ai_game`) | | | no living human → black fill, `Play_Loose_Flic_`, `End_Of_Game_Hi_Score_(PLAYER_NUM)`; living players < 2 → `Get_Winner_` 0x139e8 (council winner, else last player with +0x1f0 or not +0x24 eliminated), `Play_Normal_Win_Flic_`, hi score; `_council_winner == -1`: every player with player+0x1f0 (Antaran-home destroyed?) → `Play_All_Flics_`, `End_Of_Game_Hi_Score_(i)`, screen 10; council winner ≠ PLAYER_NUM → loose flic; == PLAYER_NUM → hi score; all of these → screen 10 (main menu) | hall of fame file | R (`End_Of_Game_Hi_Score_`) | no | UI |

`Net_Next_Turn_` (0xfc470, NETMOX/MULTPLAY, **multiplayer only**): chat box, waits for orders; the host runs `Host_Next_Turn_` 0xfbfe2 (`Decode_Game_Orders_`, `Update_Ship_Icons_In_Between_Turns_`, `Rebuild_WFI_List_`, `Next_Turn_`, `Broadcast_Game_Data_Differential_`), clients `Client_Next_Turn_` 0xfc2d2 (`Send_Game_Orders_`, `Client_Combat_Loop_`, `Decode_Broadcast_Game_Data_Differential_`, which reseeds the RNG). Out of scope.

`Next_Turn_Calc_` (0x136b3) in exact call order (game_type < 2: `Assign_Auto_Function(Null_Autofunction_, 1)` first):

| # | Call | Phase | RNG below | Section |
|---|---|---|---|---|
| 1 | `Allocate_Msg_Slots_` | bookkeeping | - | F |
| 2 | `Antaran_Invasion_Check_` (if 0x199caf && !0x1991b0) | events | R | F |
| 3 | `Apply_Evolutionary_Upgrades_` | AI/tech | - | B |
| 4 | `Compute_AI_Data_` (Go: yes) | AI | R | B |
| 5 | `Resolve_Delayed_Diplomacy_Orders_` | diplomacy | R | C |
| 6 | `Set_Opportunity_Attacks_` | diplomacy | R | C |
| 7 | `Clear_Diplomacy_Messages_` | diplomacy | R | C |
| 8 | `NPC_Diplomacy_` | diplomacy | R | C |
| 9 | `Diplomacy_Growth_` | diplomacy | R | C |
| 10 | `Do_AI_Leaders_` | AI | R | B |
| 11 | `Compute_Empire_Building_Needs_` | AI | R | B |
| 12 | `All_Colony_AI_` | AI | R | B |
| 13 | `Move_All_AI_` | AI | R | B |
| 14 | `All_AI_Colonize_` | AI | R | B |
| 15 | `All_AI_Tech_Select_` | AI | R | B |
| 16 | `Make_Scrap_Ships_Dead_` | ships | - | D |
| 17 | `Initialize_Reports_` | reports | - | D (also here: zeroes `something_interesting_happened`, `n_reports`, `done_colonize_check`, `done_player_reports`, `displayed_msgs`, `displayed_gnn`, 0x1991b5) |
| 18 | `Process_Trade_And_Research_Agreements_` | diplomacy | R | C |
| 19 | `Move_All_Ships_Toward_Stars_` | movement | - | D |
| 20 | `Resolve_Spies_` | spies | R | D |
| 21 | `Apply_All_Player_Changes_` | economy | R | D |
| 22 | `Apply_All_Colony_Changes_` | economy | R | D |
| 23 | `Do_Surrenders_` | economy | R | D |
| 24 | `Deallocate_AI_Data_` | AI | - | B |
| 25 | `Search_For_Battles_` | combat | R | E |
| 26 | `Determine_Event_` | events | R | F |
| 27 | `Do_All_Ships_XP_Check_` | officers/ships | - | F |
| 28 | `Event_Twiddle_` | events | R | F |
| 29 | `Lose_Out_Of_Range_Ships_` | ships | R | D |
| 30 | `All_Colony_Calculations_` (objdump: `Do_Colony_Calculations_`, 0xe2b31) | economy | - | D |
| 31 | `Compute_Blockades_` | economy | R | D |
| 32 | `Move_Settlers_` | economy | - | D |
| 33 | `All_Colony_Calculations_` (again) | economy | - | D |
| 34 | `Allocate_New_Ship_Slots_` | ships | - | D |
| 35 | `Check_For_Officer_Level_` | officers | - | F |
| 36 | `Decrement_Officer_ETA_` | officers | - | F |
| 37 | `Check_All_Rebellions_` | economy | R | D |
| 38 | `Repair_Ships_At_Colonies_` | ships | - | D |
| 39 | `Allocate_New_Colony_Slots_` | colonies | - | D |
| 40 | `Compute_Contacts_` | diplomacy | - | C |
| 41 | `Determine_First_Contacts_` | diplomacy | R | C |
| 42 | `Reset_Needless_Ignore_Combat_Ships_Flags_` | ships | - | D |
| 43 | `Determine_Diplomacy_Messages_` | diplomacy | R | C |
| 44 | `End_Of_Turn_Diplomacy_Adjustments_` | diplomacy | R | C |
| 45 | `Check_For_Council_Meeting_` (game_type 0 and !`_all_ai_game` only) | council | R | C |
| 46 | `Random_Officer_Check_(p)` for p < NUM_PLAYERS | officers | R | F |
| 47 | `Add_Ongoing_Event_Msgs_` | events | - | F |
| 48 | `Check_Release_Version_` → if 0: `Assert_Marooned_Leaders_` | officers | - | F |
| 49 | `Record_History_` | bookkeeping | - | F |
| 50 | `stardate += 1`; `!_in_auto_next_turn_mode` → `Set_Mouse_List(arrow)` | | | |

## Phase B: AI

Covers the AI calls of `Next_Turn_Calc_` (0x136b3), in call order:
`Apply_Evolutionary_Upgrades_` → `Compute_AI_Data_` → (diplomacy, other phase) →
`Do_AI_Leaders_` → `Compute_Empire_Building_Needs_` → `All_Colony_AI_` →
`Move_All_AI_` → `All_AI_Colonize_` → `All_AI_Tech_Select_` → … (other phases) →
`Deallocate_AI_Data_` (after `Do_Surrenders_`).

Conventions. Addresses are ELF. `n` = the routine's own instruction count,
`sub` = whole reachable subtree (shared code counted once, engine/UI included).
RNG: `R!` calls `Random_` directly, `R` only below, `-` never. "Ported" = a Go
function documented as that routine in `internal/game` (grep `is <Name>`).
Field names follow `docs/formats/save-*.md`: `player+0x..`, `star+0x..`,
`ship+0x..`, `colony+0x..`, `leader+0x..`. "AI player" = `player+0x28 != 100`
(100 = human). Every player loop below runs from `NUM_PLAYERS-1` down to 0,
every ship/colony/star loop from the last record down, unless stated.

Size: about 360 game-logic routines / 35 000 instructions reachable (engine
and UI excluded); about 250 routines / 26 500 instructions are not ported.

### Phase roots

| Root | Addr | Module | n | sub | RNG | Who | Save state written |
|---|---|---|---|---|---|---|---|
| `Apply_Evolutionary_Upgrades_` | 0xdcbb0 | AITECH.C | 34 | 34 | - | all players | `player+0x89f` race picks (31 B from evolution base `+0x8be`, keeping byte `+0x89f`), `+0x8de` = 1; only when `+0x8dd` set and `+0x8de` clear |
| `Compute_AI_Data_` | 0xd3d34 | AIDATA.C | 1653 | 17698 | R | all non-eliminated | `player+0x5ec` fleet strength [8]; `player+0x59e` AI staging star; `star+0x41` ai_stage_point; `star+0x67` surrender_to; **`star+0x34` special_popup_pending cleared for every star** (last statement). Allocates the AI scratch blocks in `screen_seg` (`Push_Block`, `docs/turn/ai-data.md`); can also cancel an AI colony's colony-ship order (`Queue[0]` -2) and, with settings 0xd8, refit ship designs |
| `Do_AI_Leaders_` | 0xd7439 | AILEADER.C | 146 | 25041 | R | AI | `player+0x32` BC, `+0x5a1` officer for hire = -1; leaders `+0x35/+0x37/+0x39/+0x3a`; techs via leader specials; ship `+0x74` / star `+0x42` officer |
| `Compute_Empire_Building_Needs_` | 0xcfcb6 | AIBUILD.C | 133 | 9497 | R | **all players incl. human** (RNG) | none (AI scratch globals only) |
| `All_Colony_AI_` | 0xd6f67 | AIDUDES.C | 31 | 29780 | R | AI | colonies (queue, colonists jobs, production via buy), player BC/freighters, ships created (status 6) and redesigned; afterwards every colony with `queue[0] == -37` gets -2 |
| `Move_All_AI_` | 0xdbb29 | AIMOVE.C | 37 | 28333 | R | AI with `player+0x5a0` FTL ≠ 0, then monsters | ship orders (status/location/eta/speed/x/y via `Make_Ships_Move_To_`), `ship+0x7f` ai_mission, colony `+0x130` marines, ships killed, msgs, diplomacy on sneak-attack war |
| `All_AI_Colonize_` | 0xe67f6 | COLONIZE.C | 19 | 18723 | R | AI | new colonies/outposts, ships killed, colony bases trashed (BC), `star+0x14` owner via ship-icon refresh |
| `All_AI_Tech_Select_` | 0xdca69 | AITECH.C | 38 | 41148 | R | AI, not eliminated | `player+0x321/0x322/0x323/0x59d`; evolution: `+0x8bf..` and `+0x8dd` |
| `Deallocate_AI_Data_` | 0xd574d | AIDATA.C | 6 | 7 | - | — | `Pop_Block(screen_seg)`: AI scratch freed; anything later that reads `_ai_*` reads stale memory |

### Apply_Evolutionary_Upgrades_ (0xdcbb0) — Go: yes (turn_aievolve.go)

Runs first in the AI group (right after `Antaran_Invasion_Check_`). For each
player with evolution chosen (`+0x8dd`) and not yet applied (`+0x8de == 0`):
copy 31 bytes `+0x8be` → `+0x89f`, restore the old byte `+0x89f`, `+0x8de = 1` (`EvolutionApplied`). Players from the last down.
Leaf, no RNG. Traced by evo2 (`build/turns/inputs/evo2.edit`: t1 with players 0..4
evolved, player 4 already applied): players 0..3 get their base picks, player 4 is skipped.

### Compute_AI_Data_ (0xd3d34), AIDATA.C, n=1653, R — Go: yes (turn_aidata.go)

Builds the AI's per-turn picture in 25 `screen_seg` blocks; the blocks,
their layout and the order are in `docs/turn/ai-data.md`. Byte-exact
(state, draws and, where dumped, the arena) on every traced turn.

| Depth | Routine | Addr | Module | n | RNG | Ported | Purpose / save fields |
|---|---|---|---|---|---|---|---|
| 1 | `XY_To_Star_Id_` | 0x7a3e3 | HAROLD.C | 38 | - | yes | ship x/y → star |
| 1 | `Ship_Strength_Vs_Ships_` | 0x5ef17 | AIPOWER.C | 20 | - | yes | strength against the player's best ship shield |
| 2 | `Get_Best_Ship_Shield_Value_` | 0x5f871 | AIPOWER.C | 34 | - | yes | |
| 2 | `Ship_Strength_Vs_Shield_` | 0x5ef4b | AIPOWER.C | 268 | - | yes | core formula; with settings 0xd8 `Update_Strategic_Ship_` refits **the ship itself** (save state), else `Get_Ship_Combat_Bonuses_`; `Base_Generic_Dcv_` (24), `Best_Point_Defense_Weapon_` (45), `Modified_Weapon_Strength_` (48), `Strength_Hits_Modifier_` (27) |
| 1 | `Ship_Strength_Vs_Ships_In_Nebula_` | 0x5ef09 | AIPOWER.C | 7 | - | yes | shield 0 |
| 1 | `Ship_Strength_Vs_Planetary_Shield_` | 0x5f379 | AIPOWER.C | 136 | - | yes | bombing strength per planetary shield level |
| 1 | `Ship_Has_Stealth_Device_` | 0x5d3db | SHIPSTAK.C | 26 | - | yes | |
| 1 | `Player_N_Turns_From_Star_To_Star_` | 0xffbd6 | SHIPMOVE.C | 105 | - | yes | ETA table |
| 1 | `Ship_Destination_Star_` | 0x1003f2 | SHIPMOVE.C | 19 | - | yes | |
| 1 | `Can_Order_Ship_`, `Ships_Try_To_Move_To_` | 0xff6be, 0xff799 | SHIPMOVE.C | 68, 330 | - | yes | used as a query only (ETA per ship per reachable star); no orders given |
| 1 | `Colony_Dominant_Player_` | 0xd2a08 | AIDATA.C | 63 | - | yes | |
| 1 | `Colony_Race_Pop_Limit_` | 0xe0c1d | COLCALC.C | 26 | - | yes | |
| 1 | `Compute_Star_Player_Range_Info_` | 0xd3ba0 | AIDATA.C | 119 | - | yes | → `Star_In_Range_Of_Given_Star_` 0xd2aa9 (28), `Find_Players_In_Range_` 0xd3a68 (93) |
| 1 | `Compute_Base_Planet_Values_` | 0xd302e | AIDATA.C | 66 | - | yes | → `Uncolonized_Planet_Worth_To_Player_` 0xd27a7 (203), `Colony_Worth_To_Player_` 0xd2cae (304), `Proximity_Worth_To_Player_` 0xd2aea (137), `Size_Subterranean_Pop_Bonus_` 0xe0a14 (3) |
| 1 | `Compute_Contextual_Planet_Values_` | 0xd3125 | AIDATA.C | 150 | - | yes | calls only memset |
| 1 | `Compute_AI_Tech_Info_` | 0xdc47c | AITECH.C | 444 | **R!** | yes | best equipment per player (`Player_Best_*_` 0xdc2f3..0xdc449), ground combat (`Compute_Player_Ground_Combat_Bonuses_` 0xec15c, 94); for each pair of living players `Random(2)` on every tie in **5** categories (shield, armour, computer, best beam, best missile); runs for humans too. `Best_Uncolonized_Undestined_Planet_` 0xd8c79 (39) → `Get_Colonizable_Planets_` 0xd76b8, qsort (`Sort_colonizable_` 0xd7662), `Delete_Already_Assigned_Planets_` 0xd7764, which can set an AI colony's `Queue[0]` 0xb → -2 (save state) |
| 1 | `Refit_Desirability_` | 0xd33d1 | AIDATA.C | 135 | - | yes | fills `_airefit` (used by `Do_Ship_Upgrade_Cheats_`); Watcom `qsort_` per player (`airefit_cmp_`) and `sort_ships_` |
| 1 | `Player_N_Turns_From_Star_To_Star_With_Intermediate_` | 0xd3576 | AIDATA.C | 213 | - | yes | memoised in `_n_turns_from_to_intermediate` |
| 1 | `Compute_Defensive_Stage_Points_` | 0xdbe84 | AISTAGE.C | 304 | - | yes | per non-eliminated player (humans too): `player+0x59e` = staging star (-1 first), `star+0x41` bit p cleared/set; zeroes covered `_star_stage_weights` |
| 1 | `May_Surrender_Star_` | 0xe4a09 | COLCALC.C | 59 | - | yes | star not p's home star, `surrender_to` conditions, `Star_In_Normal_Range_Of_Player_` 0xff666 (12), p has another colony |
| 1 | `Surrender_Star_` | 0xe4ab3 | COLCALC.C | 57 | - | yes | `star+0x67+p = q`, re-points every player that surrendered to p / to q |

Surrender step: for each player p (desc) and star (desc) with
`star+0x67+p ≥ 8` (a pending request q+8): set `0xff`, and if neither p nor q
is eliminated and `May_Surrender_Star_` → `Surrender_Star_(star, p, q)`.
Writer of the q+8 value is elsewhere (diplomacy / `Do_Surrenders_` phase).

RNG: only `Compute_AI_Tech_Info_` (count = number of ties, depends on every
player's tech).

### Do_AI_Leaders_ (0xd7439), AILEADER.C, n=146, R — Go: yes (turn_aileaders.go)

1. AI players (desc) with `player+0x5a1` = leader l ≠ -1: wanted if l has any
   of a fixed set of skill bits (`leader+0x26..0x2b`, some only when
   `player+0x59d == 0` or a race pick is absent); wanted and
   `Calculate_Officer_Cost_` (0x97a29, 2) + 50 < BC → pay (`player+0x32 -= cost`).
   `Set_Officer_To_Player_(l, p, paid)`. Then `player+0x5a1 = -1` (also
   when none was offered).
2. Stars (desc) whose `special` (`star+0x28`) is 0x50..0x59 (marooned-hero
   marker for player `special-0x50`) and that player is AI:
   `Marooned_Hero_At_Star_Can_Join_`.
3. AI players (desc): `Assign_Unassigned_Leaders_`.

| Depth | Routine | Addr | Module | n | RNG | Ported | Purpose / save fields |
|---|---|---|---|---|---|---|---|
| 1 | `Set_Officer_To_Player_` | 0x9718f | OFFICER.C | 83 | R | yes | if p already has 4 of that type: dismiss the one offered longest (`Deassign_Officer_` 0x933f2, 68 → `Increase_Officer_Level_By_One_` 0x979a0) or give up; leader `+0x37 = 0`, `+0x3a = p`, `+0x39 = paid ? 0 : 4`; paid → `Give_Leader_Specials_To_Player_`; colony leader → `Do_Player_Colony_Calculations_` (ported) |
| 2 | `Give_Leader_Specials_To_Player_` | 0x99223 | OFFICER.C | 93 | R | yes | leader 0x2f: app by `Get_Advanced_Government_Type_` 0x93d0a; else `leader+0x2e[3]` apps → `Player_May_Get_Tech_App_` / `Player_Gets_Tech_App_` (ported); ship officer with special skill 0x12/0x13 → `player+0xe75 = 1`. RNG via `Player_Gets_Tech_App_` → `Update_Player_Ship_Designs_` → **`Auto_Design_Ship_` (not ported, R!)** and `Ensure_Uncreative_Field_OK_` (R!, ported) |
| 1 | `Marooned_Hero_At_Star_Can_Join_` | 0x97f8d | OFFICER.C | 71 | R | yes | leader with status 5 at this star: room (`N_Fleet/Star_Officers_Owned_By_Player_` 0x9776c/0x977af < 4) → owner = p, location -1, `Set_Officer_To_Player_(l,p,1)`, `Move_Officer_To_Pool_` 0x944a3 (50; → `Do_Player_Colony_Calculations_`); full → `Deassign_Officer_` |
| 1 | `Assign_Unassigned_Leaders_` | 0xd73d4 | AILEADER.C | 42 | - | yes | leaders 66..0 owned by p: status 0 → `Assign_Fleet_Officer_` 0xd6fda (55) / `Assign_Star_Officer_` 0xd7171 (160); status 1 ship officer → `Reassign_Fleet_Officer_` 0xd7078 (77) |
| 2 | `Ai_Assign_Leader_To_Ship_` / `_To_Star_` | 0x98c23 / 0x98c87 | OFFICER.C | 37 / 37 | - | yes | `Get_Ship_Leader_ETA_` 0x98e4b (79), `Get_Star_Leader_ETA_` 0x98f42 (55), `Assign_Leader_To_Ship_` 0x98dad (48), `Assign_Leader_To_Star_` 0x98ceb (63): leader status 1, location, eta; ship `+0x74` / star `+0x42+p` |

RNG: only through tech gain → AI ship redesign (`Auto_Design_Ship_`) when a
hired leader brings a tech that changes designs.

All rows above are ported. Corrections to the map: a leader that is not
wanted (or too dear) is still given to the AI unpaid, as status 4; the
skill test reads CommonSkills bits 0..1, 6..17 (12/13 only while
`player+0x59d` is 0, 18/19 only without Repulsive) and, for ship officers,
SpecialSkills bits 6, 7, 14, 15. `Assign_Star_Officer_` scores every star
with own non-outpost colonies and no officer of p from the leader's
SpecialSkills (pollution with a non-tolerant dominant race, food, pop,
industry, research, missing pop room, enemies in range from `_ai_star_info`)
and picks the best, ties to the higher star. `Get_*_Leader_ETA_` read the
officer screen's table (0x19c4c8), null during the turn: ETA 0 when the
leader is already there (ship: same x/y; star: pool leader to the home star),
else 5. Traced: unpaid offers (s3, a1, t1), star and fleet assignment (s5,
s8). ldr1 (`build/turns/inputs/ldr1.edit`: t1 with offers of leaders 43, 65 and 55 to
players 2, 5 and 4, star 10's special 0x52 for the marooned hero 4) traces a paid hire
(43, 900 BC, then assigned to star 13), the marooned hero joining player 2, unpaid
offers, and player 4 dismissing the offered 53 for 55 at four star leaders. ldr2
(`build/turns/ldr2.sh`, inputs/ldr2.edit: ldr1 with leader 43's techs set to
applications 36 and 23, which player 2 lacks) checks `Give_Leader_Specials_To_Player_`
handing them over on the paid hire (`TestTurnSteps/ldr2`). ldr3
(`build/turns/ldr3.sh`, inputs/ldr3.edit: t1 with AI 4's ship 156 made a size-3 hull
with experience 400 and put at ship 36's place) checks `Reassign_Fleet_Officer_`:
leader 0 moves from ship 36 to 156 (`TestTurnSteps/ldr3`).

### Compute_Empire_Building_Needs_ (0xcfcb6), AIBUILD.C, n=133, R — Go: yes (turn_aineeds.go)

Counts per owner (< 8) over all ships with status < 5 or 6: strength vs ships
(`Mean_Ship_Strength_` 0xcfae5, 136), hull-size histogram, colony ships,
transports; spies per player (`Get_Their_Spy_Number_` 0x1026cf, BILL.C, 11);
then `Compute_Needs_(p)` for **every** player p (desc), humans included.

| Routine | Addr | n | RNG | Ported | Notes |
|---|---|---|---|---|---|
| `Compute_Needs_` | 0xcf40d | 455 | **R!** | yes | warship size need: `Random(2)` when out-matching every enemy or no enemies; extra ships `Random(10 − (objective==1))`; spies: `Random(100)` vs race pick `+0x8a8`, `Random(2)` when objective 0, `Random(4)`; transports, freighters (`Compute_Freighter_Need_` 0xcf3bd, 27), `Player_War_Level_` 0xd1006 (46). All conditional → RNG count depends on ship/colony state of every player |

Writes only AI globals (`_player_*` arrays at 0x1a72xx–0x1a74xx), no save state.
They are AIBUILD.C statics outside the arena (`Game.needs`, `aiBuildNeeds`):
`_player_n_colony_ships` 0x1a7234 [8][3] u8 (owned, wanted, in flight),
`_player_n_freighter_groups_wanted` 0x1a724c [8] u8, `_player_total_strength_vs_colonies`
0x1a7254 [8] u32 (cleared, not written here), `_player_n_transports` 0x1a7274 [8][2] u8
(owned, wanted), `_player_n_spies` 0x1a7284 [8][2] u8 (on others, wanted),
`_player_ship_sizes` 0x1a7294 [8][14] u32 (hulls 0..5, [9] warship size wanted, [10]
count, [11] Σ size+1, [12] Σ size+1 not being built, [13] Σ hull weight 1,2,3,5,8,13
from 0xcf38f), `_player_total_strength_vs_ships` 0x1a7454 [8][3] u32 (all, not being
built, wanted). Ships counted: status < 5 or 6 (being built), owner < 8. `Mean_Ship_Strength_`
averages `_ship_strengths` over the players met (`_ai_player_info` +0xe+q), humans
only when none; against both kinds the weakest AI value if below the weakest human,
else (hmin·w + amin)/(w+1), w by difficulty from 0xcf38a (1,1,2,3,4). The draws of
`Compute_Needs_`, in order: `Random(2)` for the warship size when out-matching
everyone met or meeting none; `Random(10−obj1)` when no warship strength is wanted
(or command points are over) and someone was met; `Random(100)` vs the Spying pick
when within 2 of the busiest spy master; `Random(2)` with objective 0; `Random(4)`
for a single wanted spy beyond (2+2·good spies)·contacts. Draws byte-exact on every
traced turn; the values are checked through `All_Colony_AI_`.

### All_Colony_AI_ (0xd6f67), AIDUDES.C, n=31, R — Go: yes (turn_aicolony.go, turn_aiempire.go)

AI players (desc): `Colony_AI_(p)`; afterwards colonies with `queue[0] == -37`
→ -2 (trade goods).

`Colony_AI_(p)` (0xd6ed4, 32):

| Depth | Routine | Addr | Module | n | RNG | Ported | Purpose / save fields |
|---|---|---|---|---|---|---|---|
| 1 | `Collect_AI_Colonies_` | 0xd5795 | AIDUDES.C | 65 | - | yes | p's non-outpost colonies without space anomaly; qsort blockaded first (`Colony_Is_Blockaded_` 0xdf8c1, `Event_Check_Space_Anomaly_` ported) |
| 1 | `Clear_Build_Queues_If_New_Tech_` | 0xd58d4 | AIDUDES.C | 275 | R | yes | only difficulty (settings+0xd4) ≠ 0 and war level < 3; per colony (unless `player+0x323` new-tech flag clear and queue busy): weighted choice of a new building (`Get_Weighted_Choice_Char_` 0xfe8da, R!) → `Clear_Colony_Queue_` 0xb20a1 (→ `Delete_From_Colony_Queue_` 0xb1e19, `Squish_Out_Colony_Queue_Slot_` 0xba9ec), `queue[0]` = it |
| 1 | `Keep_Ship_Designs_Up_To_Date_` | 0xd5cee | AIDUDES.C | 47 | R | yes | ships under construction (queue ≤ -100, combat type) get the current design from `player+0x326` (99 B) or a fresh doom star (`Load_Doom_Star_Ship_Design_` 0x56460 → `Auto_Design_Ship_`, R!) |
| 1 | `Freighter_Cheat_` | 0xd5d99 | AIDUDES.C | 36 | **R!** | yes | only if freighters free (`+0x38`) < 0 or food surplus (`+0xb0`) < 0: `Random(50) ≤ 1 − difficulty·free` → freighters total `+0x36 += 5`; else total 0 and difficulty → 5 |
| 1 | `Do_Ship_Upgrade_Cheats_` | 0xd5e19 | AIDUDES.C | 108 | **R!** | yes | stardate > 35035 and `_airefit[p]` non-empty: `Random(900)`; then (AI colonies ≥ 2 only) `Random(population +0xa6)`; free refits of ships at stars with no human ships enroute and no foreign colony: ship design ← `player+0x326` design or doom star |
| 1 | `Empire_Buy_Outright_` | 0xd0d2f | AIBUILD.C | 188 | R | yes | repeat: weighted choice (`Get_Weighted_Choice_Short_` 0xfe92d, R!) of colonies whose product is affordable keeping a reserve; `Colony_Buy_Outright_` 0xe109d (BC −, colony `+0x12c` pending production). Helpers `Colony_Cost_To_Buy_Product_` 0xe1085, `Colony_Cost_To_Buy_Given_Product_` 0xe1002, `Colony_Product_Cost_` 0xe0dd6 (116), `Colony_N_Turns_To_Produce_` 0xe0fa4. One RNG call per loop pass (the last returns -1) |
| 1 | `Colony_AI_Given_Tax_` | 0xd6e1d | AIDUDES.C | 50 | R | yes | job assignment and building choice, below |
| 2 | `Do_Blockaded_Colony_` | 0xd61e7 | AIDUDES.C | 80 | - | yes | jobs for blockaded colonies: `Assign_Required_Colonists_` 0xd5fe1 (99), qsorts of colonist dwords, `Pre_Import_Computing_` (ported) |
| 2 | `Do_Unblockaded_Colony_` | 0xd652c | AIDUDES.C | 81 | - | yes | farmers until food/industry upkeep met (colonist dword bits 0x80/0x100, byte1 \|2), qsort `Sort_Pops_FR_I_` / `Sort_Pops_R_I_`, `Compute_Max_Min_R_I_Worthiness_` 0xd648a (→ `Colony_Pop_Prod_Produced_` 0xddfd3, indirect table call) |
| 2 | `Assign_Additional_Research_Industry_` | 0xd66b3 | AIDUDES.C | 102 | - | yes | shifts workers/scientists to a research:industry ratio (objective `+0x28` 3/4, `+0x59d`); `Do_Player_Colony_Calculations_` once, then `Colony_Calculation_` 0xe2a70 per change |
| 2 | `Assign_Empire_Building_` | 0xd10ee | AIBUILD.C | 1489 | **R!** | yes | the empire build planner: colony ships, outposts, transports, warships (sizes from `Compute_Needs_`), refits, freighters, spies, Housing, trade goods. Writes `queue[0]` codes, creates ships with `Create_Ship_` 0xaf7b4 COLBLDG.C (158; ship slot status 6, design, owner; queue = -(100+ship)), `Load_Colony_Ship_Design_` 0x564cd, `Load_Transport_Ship_Design_` 0x5663e, `Load_Doom_Star_Ship_Design_`; `Move_Officer_To_Pool_`. RNG: `Random(700)`, `Random(5)`×2, `Random(100)`, `Get_Weighted_Choice_Int_` 0xfe96f, `Get_Weighted_Choice_Short_`; all conditional. Helpers: `Colony_Can_Build_Product_` 0xe11bc (427), `Room_For_Another_Dude_` 0xe0c9b (81), `Colony_Is_Conquered_` 0xcff02 (100), `Refit_Cost_` 0xd108b, `Industry_Worth_` 0xcfedc, `Best_Uncolonized_Undestined_Planet_` |
| 2 | `Assign_Colony_Building_` → `Assign_Colony_New_Building_` | 0xd2754 → 0xd0b08 | AIBUILD.C | 16 → 118 | R | yes | per colony (desc of the sorted list): score buildable products 1..48 (`Colony_Building_Score_` 0xd0036, 760), keep those ≥ max/(6−difficulty, 4 for humans), housing/trade weights, `Get_Weighted_Choice_Short_(…, 0x33)` (R!) → `Delete_Prod_From_Colony_Queue_` 0xb206f and `queue[0]` = building, or -3 housing / -2 trade goods |
| 2 | `Industry_Sanity_Check_` | 0xd6d80 | AIDUDES.C | 48 | - | yes | trade-goods or researching-event colonies: workers back to farmers; `Colony_Calculation_` |
| 2 | `Assign_Additional_Unblockaded_Farmers_` | 0xd6ad4 | AIDUDES.C | 179 | **R!** | yes | only if unblockaded colonies exist (else `Do_Player_Colony_Post_Production_`, ported): fix food deficit by making farmers (`Make_First_Unassigned_Into_Farmer_` 0xd6a00, `F_IR_Worthiness_` 0xd682a), one `Random_`; ends in `Do_Player_Colony_Post_Production_` |

All rows are ported and byte-exact (state and draws) on every traced turn.
Corrections and details found while porting:

- `Collect_AI_Colonies_` keeps 13-byte records on the stack (`aiColonyRec`):
  +0 colony, +1/+2 farmers/workers assigned, +3 androids and natives (race 8,
  9) sorted first, +4/+5 the free range, +6 its end before the worker pass,
  +7 low byte of colony +8 after it, +8/+0xa i16 research gain of the first /
  last free colonist, +0xc a Lithovore flag. The pop comparators read the
  colony and record through globals 0x1aa224 / 0x1aa228.
- `Clear_Build_Queues_If_New_Tech_` returns from the whole pass at the first
  colony building a ship that is being refitted (status 4).
- `Keep_Ship_Designs_Up_To_Date_`'s doom star "already loaded" test reads
  the buffer's own cost word ([ebp−6] = design +0x5e): the first doom star
  under construction runs `Auto_Design_Ship_`, the rest reuse it.
- `Do_Ship_Upgrade_Cheats_`: `Random(900) ≤ 2·(table 0xd575c[difficulty] +
  last refit value)`, then with ≥ 2 colonies `Random(population) > 8`; at most
  one refit.
- `Do_Blockaded_Colony_` loops forever when food is short and no colonist may
  farm; the port panics there. `Industry_Sanity_Check_` makes trailing
  workers scientists (not farmers).
  Community patch 1.51: there (0xd6285) it jumps to 0xd622d, not 0xd623a,
  testing food per farmer (colony +0xdd) again. The first test reads the
  value the previous owner left (a liberated colony whose old owner had
  Biomorphic Fungi on a planet without food); `Pre_Import_Computing_` has
  since made it the new owner's, 0, so the free colonists become workers
  (scientists while an event is researched) instead of the loop spinning.
  With food per farmer still above 0 it loops as before (the port panics).
- `Assign_Empire_Building_` marks idle colonies -38 and works on that list:
  conquered (or human-threatened in wartime) colonies take the first buildable
  of 0x1a, 0x1b, 0x28, 8, 0x29, 0x1c, 0x18, 0x17; a colony base or colony ship
  toward `Best_Uncolonized_Undestined_Planet_`; then per colony from the most
  industrious, trade goods (-37, turned into -2 at the end of All_Colony_AI_
  and skipped by `Assign_Colony_Building_`) under a deficit, else a weighted
  choice of warships, freighters, refit (weight never set: dead), spies,
  transports or nothing. The warship shipyard weights read `_ai_colony_info`
  by list position, not colony, and price shipyards on colony 0; the doom
  star design is loaded once per call: the test and the price check read the
  buffer's cost ([ebp−0x78] = design +0x5e at [ebp−0xd6], zeroed on entry);
  the port had read a separate, never-written word and always weighted the doom
  star (found by tra1).
- `Colony_Building_Score_` cases 8 and 41 had been ported with a shift by 4
  that the code skips; fixed (`turn_aibuild.go`).
- `Colony_Building_Score_` case 13 (Core Waste Dump, 0xd074b, jump table
  0xcff62 indexed by building − 1) returns 0 when busy and otherwise runs the
  pollution case of 5 and 32 (0xd0753), which itself has no busy test; the
  port had no case 13 and tested busy for 5 and 32 (found by ds1).

The blockaded colony is traced by blk1 (`build/turns/inputs/blk1.edit`: t1 with AI
colonies at Elysia, Fieras, Draconis and Yian marked blockaded for their owners, player 4
given Doom Star, Titan and transport apps): `Do_Blockaded_Colony_` reassigns their jobs,
byte-exact. tra1 (`build/turns/inputs/tra1.edit`: t1 with eight of player 7's ships made
transports, two of its hulls titans, players 2 and 3 given transport, titan and doom star
apps) gives players 2 and 3 transport wants and the doom star's price check (too dear:
weight 0), byte-exact after the fix above. tra2 (tra1 with 50 000 000 BC for players 2
and 3) builds a transport (ship 176, `Load_Transport_Ship_Design_`, `Create_Ship_`) and two
titans, byte-exact; tra3 (tra2 with player 7's doom-star hull and leader 55 offered to a
rich player 3: paid, its tech 195 not given) builds the same. ds1 is the user's save
(player 3, the Mrrshan, knows Doom Star construction and is at war level 3, but wants no
warships: over its command points with a negative income); ds3 (`build/turns/inputs/ds3.edit`:
ds1 with player 3's twelve warships at home freed, 20 000 BC, colonies 2 and 55 on trade
goods) gives both colonies a warship pick: weights 1, 3, 5, 0x40, 100, 0x96 (the doom star,
cost 9340 ≤ 418·54 and 233·54); colony 2 draws a battleship, colony 55 the doom star
(ship 52, `Load_Doom_Star_Ship_Design_` → `Auto_Design_Ship_` once), byte-exact. Still
untested: the refit case, which is dead (its weight is never set).

`Colony_Calculation_` (0xe2a70, COLCALC.C, 21) = `Pre_Import_Computing_` +
`Pass_Out_Imports_` + `Update_Player_Stats_` for one colony: all three are
ported, the wrapper is not.

### Move_All_AI_ (0xdbb29), AIMOVE.C, n=37, R — Go: yes (turn_aimove.go, turn_aiattack.go, turn_aiintercept.go, turn_aicolships.go, turn_diplowar.go)

AI players (desc) with FTL speed (`player+0x5a0`) ≠ 0: skipped when
`Event_Check_Hyperspace_Flux_` (ported) unless race pick byte `+0x8bc` is set;
otherwise `Move_AI_(p)`. Then `Move_NPCs_` (monsters, owner ≥ 8).

`Move_AI_(p)` (0xdb47e, 105), fixed order:
`Get_Colonizable_Planets_` + qsort `Sort_colonizable_`, `Mark_Flee_Stars_`
0xdb3e5 (46), monster-at-star map, `Delete_Already_Assigned_Planets_`,
`Launch_Sneak_Attack_`, `Launch_Attacks_(6,2)`, `Intercept_Invaders_`,
`Launch_Attacks_(1,2)`, `Assign_New_Colony_Ship_Destinations_`,
`Launch_Attacks_(2)`, `(6)`, `(1)`, `(2,0)`, `Move_Transports_`,
`Move_To_Stage_Points_`, and `Move_To_Unexplored_Stars_` only when
`_ai_player_info[p]+0x18 == +0xa` or difficulty 0.

| Depth | Routine | Addr | n | RNG | Ported | Purpose / save fields |
|---|---|---|---|---|---|---|
| 1 | `Launch_Sneak_Attack_` | 0xdb257 | 93 | R | yes | target planet `player+0x7c7` (s16) of player `+0x7ca`; at war → cancel (-1); else gather ships at the staging star (`Gather_Player_Ships_At_Star_` 0xd93f8), `ship+0x7f = 6`, move; enemy strength enroute → `Declare_War_` 0x51078 DIPLOMAC.C (181, R!: `Break_Trade_`/`Break_Research_`/`Break_Tribute_` R!, `Break_Treaties_`) — diplomacy writes `player+0x60f..` |
| 1 | `Launch_Attacks_` → `Find_Best_Attack_From_Star_` | 0xd97ee → 0xd94b3 | 26 → 200 | R | yes | per staging star: `Compute_Attack_Worthiness_` 0xd8ed2 (385; `Colony_Space_Strength_Vs_Player_` 0x5f804, `Satelite_Strength_Vs_Ships_` 0x5eb72, `Colony_Ground_Strength_Vs_Player_` 0x5f747, `Missile_Base_Strength_`, `Phasor_Base_Strength_`, `Fighter_Garrison_Strength_`, `Enemy_Colony_Worth_To_Player_` 0xd8d11, `N_Enemy_Stars_Reachable_` 0xd8e52, `AI_May_Attack_Player_` 0xd7669, `Player_Is_Hostile_To_Player_` 0xd8de1), `Get_Weighted_Choice_Short_` (R!), move, `ship+0x7f` = mission arg |
| 1 | `Intercept_Invaders_` → `Interceptions_` | 0xd9f85 → 0xd9a7e | 330 → 340 | - | yes | `Gather_Intercepting_Ships_` 0xd99d8, `Knock_Out_Impossible_Interceptions_` 0xd983f, `Ship_Min_Strength_Versus_Players_At_Star_` 0xd7f34 (indirect call), `Ship_N_Turns_To_Star_With_Intermediate_` 0xd383f, `Compare_Ratios_` 0xfeb70; `ship+0x7f = 5`, move |
| 1 | `Assign_New_Colony_Ship_Destinations_` | 0xd896f | 226 | R | yes | `Shuffle_Char_` 0xfe9b5 (R!), `Star_Has_Monster_` 0xd892b, `I_Soonest_Enemy_Colony_Ship_` 0xd87bb, `Get_Best_Escorted_Colony_Ship_` 0xd7ff7 (537; escorts `ship+0x7f = 4`; `Get_Enemy_Strength_On_Turn_` 0xd7c04, `Ship_Rough_Strength_At_Star_` 0xd7d53 indirect, `Move_Ships_With_Possible_Intermediate_` 0xd7923), `Get_Best_Unescorted_Colony_Ship_` 0xd7b48 |
| 1 | `Move_Transports_` | 0xda99c | 402 | - | yes | transports to targets; at own colony `Unload_Transports_` 0xed6b7 (INVASION.C, 35): `Kill_Ship_` per transport while marines `colony+0x130` < `Colony_Infantry_Limit_` 0xed59d, +4 each, `Update_Player_Stats_` |
| 1 | `Move_To_Stage_Points_` | 0xda485 | 340 | R | yes | `Shuffle_Sint_` 0xfe9f5 (R!), `Event_Check_Warp_Beast_` 0x234e5; `ship+0x7f = 3` |
| 1 | `Move_To_Unexplored_Stars_` | 0xdafd4 | 167 | - | yes | scouts; `ship+0x7f = 7` |
| 1 | `Move_NPCs_` | 0xdb8d8 | 159 | R | yes | monsters (owner > 7, FTL ≠ 0), ships desc. In transit: mark dest. At star: Eel (owner 13) counts `ship+0x61` (build_date reused) to 30 → `NPC_Dest_` + spawn a new Eel (`Load_Eel_Ship_Design_` 0x57d14, `Create_Nonplayer_Ship_` 0x100010) while < 4 Eels, message 0x12 (`Add_Msg_All_` 0xef6be → `Add_Msg_` 0xef629; msgs); other monsters and doubled Eels move: `NPC_Dest_` −1 → message 0x13 + `Kill_Ship_`, else `Ships_Try_To_Move_To_` + `Make_Ships_Move_To_` |
| 2 | `NPC_Dest_` | 0xdb6d2 | 130 | **R!** | yes | candidate stars with colonies (strength from fleet score/BC/colony counts; `Parsecs_Between_Stars_`), qsort `Sort_npc_dests_`, reservoir pick: `Random(k)` for each of the first ≤ 7 candidates within 15 parsecs |
| — | `Kill_Ship_` | 0xa163a | 49 | - | yes | SHIPS.C: frees the slot, officer → `Move_Officer_To_Pool_` |
| — | `Make_Ships_Move_To_` | 0xffd08 | 64 | - | yes | orders (also `Clear_All_Ships_Auto_Colonization_Flag_`, ported) |

All rows are ported; byte-exact (state, draws, AI arena incl. the memo tables)
on every traced turn. Details and corrections:

- Move_AI_'s state (globals 0x1aa23c..0x1aa24e): the colonisable planets
  (sorted, minus those already targeted), `flee` [72][10] (stars to avoid by
  turn: `Mark_Flee_Stars_` for players in +0x80e, and every target
  `Compute_Attack_Worthiness_` or `Knock_Out_Impossible_Interceptions_` gives
  up) and the monster owner at each star.
- `Compute_Attack_Worthiness_` (args colony, ships, eta, &kind, &seen, the
  interception record, mode) also counts ships the owner can finish or buy
  before arrival; kind 1 invade, 2 attack (0 when someone else blockades),
  6 invade with three times the force. `Find_Best_Attack_From_Star_` keeps
  targets of other kinds without flagging them; it picks among those ≥ 2/3 of
  the best by `Get_Weighted_Choice_Short_`.
- `Intercept_Invaders_` builds 0x3f-byte threat records (players' en-route
  strength from the first arrival; monsters with bits 8..14, their eta always
  0); `Interceptions_` assigns every free ship to the soonest threat it reaches
  (ties by `Compare_Ratios_` of cover to threat), and while a threat is left
  uncovered (unless few colony stars) drops the *last* record and starts over.
  Its tie walk has no bound (the port panics past the record).
- `Move_Ships_With_Possible_Intermediate_` compares the can-move byte (not the
  turns) with the best so far, and resets all ships to the first one's place.
- `Assign_New_Colony_Ship_Destinations_`: escorted where others are in
  extended range, enemies arrive or a monster sits; `Get_Best_Escorted_Colony_Ship_`
  compares the escort with later arrivals using ≥ from a star and > in flight.
  Leftover colony ships go to the AI stage-point stars after `Shuffle_Char_`
  and `Sort_candidate_stars_` (which reads b's strength only under a's range test).
- `Move_To_Stage_Points_` adds the stage points' weights, times war level + 1,
  to the staging star's `_star_stage_weights` (arena write); when no ship can
  reach the chosen point it copies `spIdx[nSP]` over `spIdx[best]` instead of
  removing the point.
- `Launch_Sneak_Attack_` → `Declare_War_` (with `Break_Trade_`, `Break_Research_`,
  `Break_Tribute_`, `Break_Treaties_`) in `turn_diplowar.go`, shared with
  diplomacy.

The sneak attack and declaring war are traced by sneak1 (on the human) and war1
(`build/turns/inputs/war1.edit`: sneak1's setup aimed at AI 1's Bue, with an alliance,
trade and research treaty between 1 and 2): `Declare_War_` runs `Break_Trade_`,
`Break_Research_` and `Break_Treaties_` (the alliance branch, witnesses' grudges),
byte-exact. Untested (no trace reaches them):
monsters (`Move_NPCs_`, `NPC_Dest_`, Eel spawning), most of
`Move_To_Unexplored_Stars_`, transports following fleets, monster threats in
`Intercept_Invaders_`. Guessed where the original reads outside its arrays:
a missing staging star (-1) and a 0 eta in the transport flee test.

### All_AI_Colonize_ (0xe67f6), COLONIZE.C, n=19, R — Go: yes (turn_aicolonize.go)

AI players (desc): `AI_Colonize_(p)` (0xe65f8, 148). Stars (desc) whose
`colonize_player` (`star+0x37`) == p — **the value saved at the end of the
previous turn** (`Find_Colonize_Players_` runs later in the turn). Repeats on
the same star while a colony base (`Player_Colony_Base_At_Star_` 0xfda3f,
building 11 = `colony+0x141`) or a colony ship (`Player_Ship_Type_At_Star_`
0xfdaa7, type 1) remains:

- best free planet by `Uncolonized_Planet_Worth_To_Player_` (orbit 4..0);
- none: every colony base trashed, `BC += DAT_0017ec16/2`, `colony+0x141 = 0`;
- else `Make_New_Colony_Or_Outpost_` 0xe5eb3 (131; `Init_Colony_` 0x12d75,
  `Colony_Calculation_`, `Compute_Star_Colony_Stuff_` ported); by ship: escorts
  with `ship+0x7f == 4` at that star → 0, `Kill_Ship_`,
  `Update_Ship_Icons_In_Between_Turns_` (MAINSCR.C; UI cache, but it also
  rewrites the saved display owner `star+0x14` for `PLAYER_NUM`); by base:
  `Remove_Building_(colony, 11)` 0x145ea ERICNET.C (135, R via
  `Enforce_Population_Limits_At_Colony_` 0xec97c → `Shuffle_` 0xfea82 R!;
  also `Assign_New_Home_Planet_` 0xecb65, `Update_Ship_ETAs_` 0x10041c,
  `Colony_Environmental_Stuff_` ported).

Ported with `Player_Colony_Base_At_Star_`, `Player_Ship_Type_At_Star_` (any
status below 5; with the flag clear only ships not just built),
`Make_New_Colony_Or_Outpost_` and `Init_Colony_`: the colony goes into the
slot `Allocate_New_Colony_Slots_` reserved for p (the search has no bound) or,
for a colony on a planet that has one, that record (then building 0x16 is
set); the settler farms unless the planet has no farmland or p is Lithovore
or Cybernetic; a native planet (special 6) adds three race-9 pops and clears
the planet and star specials. Byte-exact on every traced turn; only s7
colonises (by ship). col5 (`build/turns/inputs/col5.edit`: t1 with a colony base at Tur
for player 4, natives on its free planets and a slot reserved for 4, and two bases at
Phobeus for player 2, where nothing is free) settles by base with natives joining and
refunds both bases. Outposts never come from this step: `AI_Colonize_` passes 0 for
the outpost flag (only the human's colonisation popups make outposts).

### All_AI_Tech_Select_ (0xdca69), AITECH.C, n=38, R — Go: yes (turn_aitechsel.go)

`Compute_AI_Tech_Info_` again (second round of tie `Random(2)` calls, same
rules as above), then non-eliminated AI players (desc):

| Routine | Addr | n | RNG | Ported | Notes |
|---|---|---|---|---|---|
| `AI_Tech_Select_` | 0xdc288 | 33 | R | yes | when the current field (`+0x321`) is researched (`+0xc4+f == 3`) or `+0x323` set: clear `+0x323`, `+0x322` = `Choose_Tech_Application_`, `+0x321` = its field (table 0x17e07f); field > 0x4a → `+0x59d = 1` |
| `Choose_Tech_Application_` | 0xfd335 | 197 | R | yes | NEWTECH.C. Value per obtainable app (`Calc_Tech_Value_` 0xfc845, 655; `Set_Competition_Tech_Values_` 0xfd219, `Get_Competition_Player_Tech_Value_` 0xfc7ac, `All_Gray_Boxes_Researched_` 0xfd2f9); settings+0xd8 filters apps; **divides cost by player 0's research (`_player+0xac`), not p's** (bit-exact quirk); objective/difficulty cuts; `Get_Weighted_Choice_Int_(…, 0xd4)` (R!) or `Choose_Hyper_Advanced_Tech_` 0xfc734 (R!) |
| `Setup_Evolutionary_Upgrade_` | 0x5b0aa | 44 | - | yes (AI branch) | when `+0x155 == 3` (evolutionary mutation app known) and `+0x8dd == 0`: AI copies race picks into the evolution base (30 B), `+0x8dd = 1`. The human branch (`Evolutionary_Mutation_Screen_`, RACEOPT.C, UI) is unreachable here; humans get it from the `Has_Evolutionary_Upgrade_` report |

Byte-exact (state and draws) on every traced turn. Details and corrections:
`Compute_AI_Tech_Info_` here writes a stack copy of `_aitech` (the pointer
0x1aa1d0 is redirected and left null afterwards; the arena block keeps step
3's values). `Setup_Evolutionary_Upgrade_` for an AI copies the evolution
base *into* the picks (1..30), not the reverse. `Choose_Tech_Application_`
weights only gray-box fields (0x16, 0x17, 0x1c, 0x1d, 0x37, 0x39) until all
of them are researched; it reads `FieldStatus[-1]` for applications of field
-1 (the high byte of MaintenanceParts[5]). `Get_Competition_*_Tech_Value*_`
scan applications 0..0x4a only. hyp1 (`build/turns/inputs/hyp1.edit`: t1 with player 2's
researchable fields made unavailable and a new pick forced, player 3 knowing Evolutionary
Mutation) traces a hyper-advanced pick (0xcd, field 76, `+0x59d` = 1) and the AI's
`Setup_Evolutionary_Upgrade_`. hyp2 (`inputs/hyp2.edit`: t1 with settings +0xd8 = 1, strategic combat, and
player 2 made to pick again) traces the strategic filter: applications 17, 125, 126, 150,
153, 161, 171 are skipped for lack of the strategic flag.

### Shared helpers (listed once)

| Routine | Addr | Module | n | RNG | Ported | Notes |
|---|---|---|---|---|---|---|
| `Get_Weighted_Choice_Char_/Short_/Int_` | 0xfe8da / 0xfe92d / 0xfe96f | RUSS.C | 42/31/36 | R! | no | sum of weights; 0 → return -1 **without** an RNG call; else `r = Random(sum)` (1-based) and walk while `w[i] < r`, subtracting (Short_ read) |
| `Shuffle_`, `Shuffle_Char_/Sint_/Int_` | 0xfea82, 0xfe9b5/0xfe9f5/0xfea3c | RUSS.C | 74, 31/28/28 | R! | no | one `Random_` per element |
| `Compare_Ratios_` | 0xfeb70 | RUSS.C | 72 | - | no | |
| `Mox_Update_`, `Assert_Settings_` | 0xfe8be, 0xfe8bd | RUSS.C | 2, 1 | - | yes / no | no-ops |
| `Player_War_Level_` | 0xd1006 | AIBUILD.C | 46 | - | yes | |
| `Colony_Calculation_` | 0xe2a70 | COLCALC.C | 21 | - | wrapper no, parts yes | |
| `Do_Player_Colony_Calculations_`, `Do_Player_Colony_Post_Production_` | 0xe2d72, 0xe2d09 | COLCALC.C | 26, 29 | - | yes | |
| `Player_Gets_Tech_App_` | 0xe4204 | COLCALC.C | 141 | R | yes, except the AI design branch | → `Update_Player_Ship_Designs_` → `Auto_Design_Ship_` 0x616a5 AIDESIGN.C (1177, **R!**, not ported; `Get_Ship_Class_` R!, `Determine_Beam_Weapon_Modifications_` R!, `Add_*_To_Design_`, `Get_Ship_Design_Name_` reads LBX text) |
| `Load_Doom_Star_Ship_Design_` | 0x56460 | INITSHIP.C | 19 | R | no | → `Auto_Design_Ship_` |
| `Create_Ship_` | 0xaf7b4 | COLBLDG.C | 158 | - | no | new ship slot (status 6) |
| `Kill_Ship_`, `Move_Officer_To_Pool_` | 0xa163a, 0x944a3 | SHIPS.C, OFFICER.C | 49, 50 | - | no | |
| `Ships_Try_To_Move_To_`, `Make_Ships_Move_To_`, `Can_Order_Ship_` | 0xff799, 0xffd08, 0xff6be | SHIPMOVE.C | 330, 64, 68 | - | yes | |
| `Parsecs_Between_Stars_`, `Star_In_Range_Of_Player_`, `Black_Hole_Blocks_Points_` | 0xebeb7, 0xff5f8, 0xeb7fd | GEO.C/SHIPMOVE.C | | - | yes | |
| `Tech_Level_` | 0x6e70a | DESIGN.C | 38 | - | yes | its `hgetch_` branch is an assert (UI, error only) |
| `qsort_` | 0x133ef4 | clib | 338 | - | yes (`internal/clib`) | many AI sorts; comparator ties decide order |

UI reached from this phase (name only, not ported here): `Evolutionary_Mutation_Screen_`
and its drawing (human-only, unreachable from the AI loop), `Message_Box_`
via that screen, `Update_Ship_Icons_In_Between_Turns_` (ship icon cache, already
ported pieces), `Set_Design_Screen_Help_List_` (from `Update_Calculated_Design_Data_`).

### Bit-exactness notes

- **Human players consume RNG in the AI phase**: `Compute_AI_Tech_Info_`
  (twice per turn, over all non-eliminated players) and `Compute_Needs_` (all
  players). The human's tech and fleet therefore shift the AI's RNG stream.
- Almost every RNG call is conditional on state (ties, deficits, war level,
  difficulty, stardate > 35035, BC). Porting must reproduce all those
  conditions exactly; a single missed call desynchronises everything after it.
- `Auto_Design_Ship_` (RNG) is reachable from three places in this phase
  (leader techs, `Keep_Ship_Designs_Up_To_Date_`/`Do_Ship_Upgrade_Cheats_`/
  `Assign_Empire_Building_` doom stars) and from tech gain later in the turn;
  it is the one big unported piece shared with the economy phase.
- Many `qsort_` calls with custom comparators (colonies, colonizable planets,
  NPC destinations, pops, refits); Watcom `qsort_` is ported, comparators are not.
- `Choose_Tech_Application_` reads player 0's research total for every player.
- `Compute_AI_Data_` clears `star+0x34` (special_popup_pending) for all stars:
  pending special popups from the previous turn are dropped at turn start.
- `AI_Colonize_` uses `star+0x37` from the previous turn's save.
- `Update_Ship_Icons_In_Between_Turns_` makes `star+0x14` depend on `PLAYER_NUM`.
- `Deallocate_AI_Data_` frees the AI blocks after `Do_Surrenders_`; later
  phases must not depend on them (check `Search_For_Battles_`).

### Expected footprint in t1 (before → after)

Fields the AI phase plausibly accounts for: `ship.Design` (Create_Ship_,
redesigns, upgrade cheats), `ship.AIMission` (17), ship orders
(`Location/ETA/Speed/X/Y/Status` with movement), `colony.Queue` (21),
colonists' jobs (`colony.Colonists`), `player.StagingStar` (3) and
`star.AIStagePoint` (1), `star.SurrenderTo` (1), `star.SpecialPopup` (1,
the clear), `player.FleetStrength` (6), `player.ResearchGoal`/`ResearchApp`,
`player.OfficerForHire` (1), `player.BC`, `star.ColonizePlayer`.

## Phase C: Diplomacy

Eleven `Next_Turn_Calc_` steps in four separate places in the call order. The
first block (`Resolve_Delayed_Diplomacy_Orders_` … `Diplomacy_Growth_`) runs
**before** the AI turn; trade and research agreements run before ship
movement; contacts, first contacts, the per-human message pick and the
end-of-turn adjustments run after combat and colonies; the council runs last.

Notation. `P[i]` = player record i (0xea9 bytes); `P[i].X[j]` = the per-other-player
`[8]` array at that offset (field names from `docs/formats/save-players.md` /
`internal/save/players.go`). "human" = `+0x28 == 100` ('d'); "dead" = `+0x24`.
RNG: `R!` calls `Random_` directly, `R` only below, `-` none. "Go" = ported in
`internal/game` (the file is named; the routine-by-routine notes and the corrections
to this map are under "Ported" below). Insn = own instructions (disassembly); "sub" =
whole reachable subtree, engine excluded, for phase roots. Globals:
`DAT_00199cb0` = difficulty (settings +0xd4), `DAT_00199cb2` = galaxy size
(settings +0xd6, used only by the buildup penalty), `_COUNCIL_FLAG`
(save 0x32840; 1 normal, 3 = a council winner exists → everyone at war 6).

### Call order (phase roots in Next_Turn_Calc_ order)

| # | Routine | Addr | Module | Purpose | Save fields R/W | RNG | Go | Insn / sub |
|---|---|---|---|---|---|---|---|---|
| 1 | `Resolve_Delayed_Diplomacy_Orders_` | 0x1ffed | DIP-SCRN.C | apply orders a player queued in the diplomacy screen: `P[i].DelayedOrder[j]` 0 war / 1 peace / 2 break treaties / other nothing; then −1 | R +0x24, DelayedOrder; W DelayedOrder=−1 + whatever the action writes | R (via `Declare_War_`) | yes (turn_diplomacy.go) | 58 / 984 |
| 2 | `Set_Opportunity_Attacks_` | 0x1fd80 | DIP-SCRN.C | for every AI i vs human j in contact and normal range (and no hyperspace flux): `Find_Opportunity_Attack_(i,j,100,…)` → SneakStar/SneakValue/SneakStrength[j]; else −1/0/0 | R +0x24,+0x28, Contact, InNormalRange, ships/stars via AIMOVE; W SneakStar, SneakValue, SneakStrength | R (AIMOVE `Find_Best_Attack_From_Star_`, phase B) | yes (turn_diplomacy.go) | 95 / 15505 |
| 3 | `Clear_Diplomacy_Messages_` | 0x5090c | DIPLOMAC.C | per-turn reset; see below | W MessageParam→LastMessageParam, MessageParam=0, MessageThisTurn=0, TurnScratch[0..3]=0, Demand=0, MessageFlags=0; TreatyTimer−1 (both dirs, if >0); status 3 with timer 0 → 0; TurnsAtWar+1 (≤250) while status ≥4 | R (Modifier adj.) | yes (turn_diplomacy.go) | 108 / 302 |
| 4 | `NPC_Diplomacy_` | 0x252a7 | NPCDIPLO.C | AI→AI and AI→human diplomacy driver; 6 calls then **falls through** into `NPC_Tech_Exchange_` | see subtree | R | yes (turn_npcdiplo.go) | 6 (+8+tail) / 6872 |
| 5 | `Diplomacy_Growth_` | 0x4dd6b | DIPLOMAC.C | relation drift from treaties/tribute, threat penalties, relations decay to base, symmetry fix-up | Relations, BaseRelations(read), Modifiers, MessageParam/MessageThisTurn (via Change_Relations_), Diplo806/80E −1; COUNCIL_FLAG==3 → all TreatyStatus=6, Relations=−100 | R! | yes (turn_diplomacy.go) | 428 / 16354 |
| … | (AI turn, phase B) | | | | | | | |
| 6 | `Process_Trade_And_Research_Agreements_` | 0x101e77 | TRADE.C | treaty values creep toward goals (+goal/5 per turn, remainder by RNG) | R TradeTreaty, ResearchTreaty; W TradeValue/TradeGoal, ResearchValue/ResearchGoal, TradePops, ResearchPops | R! (below) | yes (turn_trade.go) | 34 / 508 |
| … | (movement, spies, colony changes, combat, events, colonies) | | | | | | | |
| 7 | `Compute_Contacts_` | 0xeb192 | CONTACT.C | contact = a colony of one player within the other's extended range (3r/2, no +1) of a colony star, pairs of colonies j < i, colonies with planet ≠ −1; in-normal-range likewise. Then per player p (from the last) only the **highest** q whose contact changed counts: a **new** contact sets LostContactMask bit q and LostContactCount[q]++ (the map's "lost" fields count gained contacts), either way `something_interesting_happened[p]` = 1. Without contact: trade/research treaty 0 and `Bring_Home_Spies_Vs_` both ways (spies home, cap 63) | W Contact, InNormalRange, LostContactMask/Count, TradeTreaty, ResearchTreaty, Spies, `something_interesting_happened` | - | yes (turn_contacts.go) | 221 / 552 |
| 8 | `Determine_First_Contacts_` | 0x50b57 | DIPLOMAC.C | for each **human** i: new contact → WarDeclared=1, status 0, TreatyTimer += 2·peace duration(pers) both ways; AI greets (MessageFlags = pers+1) unless council war; lost-contact re-meet messages (+7 / +0xd); human↔human uses `Change_Relations_(−10000, …, 0x11/0x12)` | W WarDeclared, TreatyStatus, TreatyTimer, MessageFlags, MessageParam(10000) | R (via Change_Relations_) | yes (turn_diplomacy.go) | 154 / 691 |
| 9 | `Reset_Needless_Ignore_Combat_Ships_Flags_` | — | SHIPMOVE.C | (not diplomacy; listed by the caller list, phase D) | | | | |
| 10 | `Determine_Diplomacy_Messages_` | 0x4eb06 | DIPLOMAC.C | picks the one message each AI i sends to each human j this turn → `P[i].MessageFlags[j]` (0x657), shown at begin of turn by `Has_Diplomacy_Messages_` | see below | R! | yes (turn_diplomsg.go) | 350 / 5461 |
| 11 | `End_Of_Turn_Diplomacy_Adjustments_` | 0x4dab2 | DIPLOMAC.C | treaty hatred, turn-summary msgs, PrevTreatyStatus = TreatyStatus, Diplo7EC (bankrupt-turn counter: +1 if +0xb0 < 0 else 0), Modifiers += 10 capped at 0, Diplo816 −1, LastProposalTurn/ThreatTurn +1 (≤250) for AI→human in contact, status 3 timer-0 → 0 + "peace ended" msg | W PrevTreatyStatus, Diplo7EC, Modifiers[0..3], Diplo816, LastProposalTurn, ThreatTurn, TreatyStatus, msgs | R (Hatred) | yes (turn_diplomacy.go) | 174 / 696 |
| 12 | `Check_For_Council_Meeting_` | 0x168af | COUNCIL.C | **single-player non-all-AI only** (caller tests `game_type==0 && !all_ai_game`); if stardate−35000 > 24, (≥ next_council_meeting_turn or COUNCIL_FLAG==0), ≥3 living players, ≥ half the stars owned (star+0x14 ≠ −1): `Main_Council_Screen_()`, next = stardate−35000+25 | R stars, players, COUNCIL_FLAG, next_council_meeting_turn; W next_council_meeting_turn + council results | R | decision only (turn_council.go) | 56 / 66347 |

### Subtrees

**3. Clear_Diplomacy_Messages_**

| d | Routine | Addr | Purpose | Fields | RNG | Insn |
|---|---|---|---|---|---|---|
| 1 | `Modifier_Diplomacy_Adjustments_` | 0x50dac | for all i,j with status ≠ 6: each of Modifiers[0..3] gets `+Random(5)` if < 0 and again if < 50; Modifiers[3] clamped ±200. Up to 8 `Random_` per ordered pair, **i,j over all players incl. i==j** | Modifiers | R! | 135 |

**4. NPC_Diplomacy_** (order: the six calls, then fall-through)

| d | Routine | Addr | Module | Purpose | Fields | RNG | Insn |
|---|---|---|---|---|---|---|---|
| 1 | `NPC_Sneak_Attacks_` | 0x53edb | DIPLOMAC.C | AI i with no war target (Diplo7C7 == −1) and not at war with a human: clears stale sneak values, evaluates each human j (`Sneak_Attack_Evaluations_`), then acts on the result: 2 → set war target/peace with other AIs; 1 → **threat/demand** (MessageFlags=0x6a, MessageParam=10000, Diplo887=reason, `Set_Demands_`); 3 → 0x69; 4 → 0x7c (+Diplo7CB=j); ThreatTurn reset both ways; Diplo816 = 10/20+Random(20). With a treaty timer running it first rolls `_personality_break_peace_treaty_chance`. If a target exists and Diplo816 == 0 → target = −1 | Diplo7C7, Diplo7C9, Diplo7CB, Diplo816, Diplo887, ThreatTurn, SneakValue/Strength, MessageParam, MessageThisTurn, MessageFlags, TurnScratch[1..2], Demand, Diplo7B7 | R! | 348 |
| 2 | `Sneak_Attack_Evaluations_` | 0x544a1 | DIPLOMAC.C | score how much AI i wants to hit human j (relations, threats, personality, fleet ratio, population, history trend `History[...]` at +0xb9b, council, diplomat leader); returns 0..4 + reason code; calls `Get_Demands_(i,j,lvl,1,1,1,1)` | R Relations, DemandState, Diplo6CF, Diplo7EE/7F6, FleetStrength, Population, History, TreatyStatus, ThreatTurn, picks +0x8b2/+0x8b3, +0x1df | R! (≥6 calls) | 542 |
| 3 | `Get_Player_Diplomacy_Personality_` | 0x53e96 | personality, 4→6 if Diplo727[j] | - | 23 |
| 3 | `Get_Player_Max_Population_` | 0xe5b17 | COLCALC | Σ colony pop limits | - | 30 |
| 3 | `Fleet_Strength_Comparison_` | 0x500cf | 100·(FS[i][j]+1)/(FS[j][i]+1) ≤ 800, halved per other war | - | 48 |
| 3 | `System_Proximity_Count_` | 0xdcb47 | AITECH | stars of j in i's normal range (phase B helper) | - | 41 |
| 3 | `Best_Leader_Diplomat_Bonus_` | 0xe5e09 | COLCALC | best diplomat officer level | - | 56 |
| 3 | `Get_Demands_` | 0x4f93b | DIPLOMAC.C | choose what i asks of j: tribute (level (lvl>2)+1), gold (BC), system (`Get_Surrender_System_`), tech (`Get_Differential_Tech_List_`); results in globals `_diplomacy_proposal_*_bribe`; returns 1/0 | R +0xb4, BC +0x32, AppStatus +0x117 | R! (Random(3)/(2) chain) | 267 |
| 4 | `Get_Differential_Tech_List_` | 0x27094 | NPCDIPLO | techs i has and j lacks | - | 112 |
| 4 | `Get_Surrender_System_` / `May_Surrender_Star_` | 0xe5be3 / 0xe4a09 | COLCALC | pick a system j could cede | R! / - | 78 / 59 |
| 3 | `Set_Demands_` | 0x54cc0 | DIPLOMAC.C | copy proposal globals into both directions: TurnScratch[1] (0x75f, tribute), TurnScratch[2] (0x767, tech), Demand (0x777, gold), Diplo7B7 (system) | W those | - | 33 |
| 3 | `Declare_Peace_` | 0x524fb | status 3, Relations += 50 capped 0, TreatyTimer = 30, clear DemandState, WarLosses, Diplo70F | - | 55 |
| 2 | `NPC_Diplomacy_Buildup_Penalties_` | 0x25c7c | NPCDIPLO.C | even turns only: for ordered pairs with j colonies≥5 count above 3·(age+1), 1 in 4: `Change_Relations_(−k)` | R ColoniesPop5 | R! | 99 |
| 2 | `NPC_Threats_` | 0x252d5 | NPCDIPLO.C | each pair with MessageParam ≠ 0, ≠ 10000 and MessageThisTurn ≠ 0: negative incident → DemandState += 1/2 (symmetric) by `_personality_threat_modifiers`; positive → DemandState −1; else clear MessageThisTurn | DemandState, MessageThisTurn | R! | 149 |
| 2 | `NPC_To_NPC_Treaty_Negotiations_` | 0x2552d | NPCDIPLO.C | AI↔AI only, 1/(250−40·diff) per ordered pair: NAP/alliance (`Start_Treaty_`), trade/research flags, rare tribute treaty (`Start_Tribute_Treaty_`), `Honor_Alliances_`, `Adjust_Diplomat_Modifiers_`×3; then `Eliminate_NPC_Treaty_Conflicts_` | TreatyStatus, TradeTreaty, ResearchTreaty, Tribute, Relations, TreatyTimer | R! | 371 |
| 3 | `Start_Treaty_` / `Start_Tribute_Treaty_` | 0x5232e / 0x52049 | set status 1/2 with timer × peace duration; tribute | - | 115 / 71 |
| 3 | `Honor_Alliances_` | 0x25ad2 | allied AI joins wars (`Declare_War_`, AllianceCall) | R | 66 |
| 3 | `Eliminate_NPC_Treaty_Conflicts_` | 0x25bc6 | break treaties with enemies of allies (`Break_Treaties_`) | - | 54 |
| 2 | `NPC_Declarations_Of_War_` | 0x25df1 | NPCDIPLO.C | AI i (not during hyperspace flux unless pick +0x8bc): candidate wars by personality 3 (aggressive) roll, low relations (only on `stardate % NUM_PLAYERS == j`), Diplo60E, bankrupt Diplo7EC, war losses WarLosses2>0; filter; then `Declare_War_` (and `Change_Relations_(−10000, …, reason)` to queue a message to a human) | TreatyStatus, Relations, BaseRelations, Modifiers, … | R! | 603 |
| 2 | `NPC_Peace_Negotiations_` | 0x2670a | NPCDIPLO.C | AI↔AI at war longer than 90−15·diff−20·(humans at war with i): `NPC_Surrender_Check_` → `Surrender_Empire_`+`Event_Surrender_` or `Declare_Peace_` | TurnsAtWar, TreatyStatus, SurrenderTo | R | 110 |
| 3 | `NPC_Surrender_Check_` | 0x27a3d | | | R! | 367 |
| 2 | `NPC_Tech_Exchange_` | 0x252c5 → body at 0x26991 (inside the `NPC_To_Human_Diplomacy_` symbol, which itself is a bare `ret`) | NPCDIPLO.C | AI i on `stardate % NUM_PLAYERS == i`, each other AI j in contact, not repulsive: if `Random(100) < Relations+50` and `Random(4) ≤ difficulty`, swap the most valuable known application each lacks (`Player_May_Get_Tech_App_`, `Player_Gets_Tech_App_` ×2) | AppStatus, research (via Player_Gets_Tech_App_) | R! | 132 |
| 3 | `Player_Gets_Tech_App_` | 0xe4204 | COLCALC | give a tech application (ported with the tech gain path) | R (its subtree 23k insn, design refits) | **yes** | 141 |

**5. Diplomacy_Growth_**: pairs i<j (unless both human): `Random(2)` drift +1 when relations < −50 and timer running; NAP / trade / research / tribute-1 / tribute-2 each `Random(100) ≤ 100−rel` → `Change_Relations_(Random(3) or Random(8))`; alliance `Change_Relations_(Random(5))`. Then for each **human** i and AI j not allied: `Player_Threatens_Player_` (AIMOVE, phase B) → `Change_Relations_(−…, reason 5)`. Then per i,j: `Random(105)` and `Random(4)`, `Random(2)` drift of Relations toward BaseRelations unless LastMessageParam ≠ 0; at war force ≤ −90; finally the whole matrix is made symmetric from the upper triangle, Diplo806/80E −1, and COUNCIL_FLAG 3 → total war. RNG count is data dependent; every pair draws at least `Random(2)` and `Random(105)`.

| d | Routine | Addr | Purpose | RNG | Insn |
|---|---|---|---|---|---|
| 1 | `Change_Relations_` | 0x4e3b5 | the central relation change (amount, i, j, reason, p5, p6): scaled by current relations, personality 0/4, pick +0x8b3, difficulty, stardate>35100; adds to Relations (±100, cap 65 unless allied, ≤ −25 at war) and Modifiers[0..2] (both ways) and Modifiers[3]; remembers the biggest incident in `P[j].MessageParam[i]`, `MessageThisTurn[i]` = reason, Diplo66F/67F; amount −10000 = "queue message to human" (MessageParam = 10000). Reason 10 first calls `Reward_Check_` | R (only via Reward_Check_) | 406 |
| 2 | `Reward_Check_` | 0x4ea03 | third parties whose Diplo7E4 names j get `Change_Relations_(200, …, 0xf)` | R! by symbol table; the decompile shows no `Random_` in the reachable blocks (Ghidra removed two "unreachable" blocks) — check the assembly | 72 |
| 1 | `Player_Threatens_Player_` | 0xdbcc8 | AIMOVE: strongest attack j could launch on i | R | 71 |

**6. Process_Trade_And_Research_Agreements_** (players from last to first, partners from last to first)

| d | Routine | Addr | Purpose | RNG | Insn |
|---|---|---|---|---|---|
| 1 | `Process_Trade_Agreement_` | 0x101d53 | goal = `Trade_Agreement_Goal_`; value += goal/5 + (`Random(5) ≤ goal%5` when goal%5 ≠ 0), capped at goal; TradeGoal = goal | R! | 46 |
| 2 | `Trade_Agreement_Goal_` / `Base_Trade_Agreement_Goal_` | 0x101ba4 / 0x101b3c | from TradePops, pirate event, officer (`Owned_Officer_Level_`, ported) | - | 74 / 40 |
| 1 | `Process_Research_Agreement_` | 0x101de5 | same for research (`Research_Agreement_Goal_` 0x101cc5) | R! | 46 |
| 1 | `Compute_N_Trade_And_Research_Pops_` | 0x101a42 | after the loop: per colony count colonists (`race nibble < 8` → both; android 8 with 0x100 → research only) into TradePops/ResearchPops | - | 71 |

Note the order: the goals use the **previous** turn's TradePops (recomputed after).

**10. Determine_Diplomacy_Messages_** (after `Limit_Treaty_Modifiers_`: Modifiers[0] clamped −200..120). For AI i, human j (alive, in contact, MessageFlags still 0, not permanent war 6/6):
- COUNCIL_FLAG 3: only MessageThisTurn 1..11 can trigger `0x7b` with `Random(100) ≤ |MessageParam|/10`.
- else by `c = MessageThisTurn[j]` (the reason recorded by `Change_Relations_` this turn):
  - 1..9 → `Determine_Bad_Message_`; 10 / 11 → war declaration (0x50 / 0x51, or 0x42 "treaty break" for personality 4 with a treaty) + `Declare_War_(j,i)`; 12..16 → `Determine_Good_Message_`; 17 → greeting pers+1; 18 → pers+7 or +0xd; 19 → 0x46; 20 → 0x6c; 21 → 0x52; 22 → 0x50; 23 → 0x6f.
  - then, while still empty: at war → `Peace_Proposal_(i,j,0)`; `Required_Alliances_` (allied, 1/10, ask to join a war: 0x56 + AllianceCall); `NPC_Required_Alliances_` (allied AI drags j into its war: 0x46 + `Declare_War_`); `Unprovoked_Good_Messages_`.
- Repulsive races: tribute/treaty-type codes (0x13..0x38, 0x56..0x6b, 0x7c..0x7d) are erased both ways.

| d | Routine | Addr | Purpose | RNG | Insn |
|---|---|---|---|---|---|
| 1 | `Determine_Bad_Message_` | 0x4f0dc | incident escalation: at war → `Peace_Proposal_(…,1)`; else 3 rolls of `Random(100)` raise DemandState; DemandState ≥3 (≥2 without treaty) → `Sneak_Attack_Or_Declare_War_`; =1 → warning (c+0x26 / c+0x12); =2 with treaty → `_threat_demand_chance[pers]` roll → `Get_Demands_(i,j,6,1,1,1,1)` → c+0x1b + `Set_Demands_`, else break treaties/research/trade → c+0x2f | R! | 328 |
| 2 | `Peace_Proposal_` | 0x4f694 | war ≥10 turns: score from war counts, fleets, attacks, losses, personality; `Random(100)` gate; ask for peace with a demand (0x55) / offer one (0x54) / plain (0x53); TurnsAtWar = 0 | R! | 212 |
| 2 | `Sneak_Attack_Or_Declare_War_` | 0x4f59b | with a prepared sneak star (strength ≥ 200, pers ≠ 4) set AI war target Diplo7C7/7C9; else if fleet ratio > 66% `Declare_War_` + c+0x38 | R | 82 |
| 1 | `Determine_Good_Message_` | 0x4fe25 | chance 3/10/20% by c: thanks (0x59..0x5b), gift reactions 0x5c..0x60, may ask for something (`Get_Demands_(j,i,4,0,1,1,0)`) | R! | 156 |
| 1 | `Unprovoked_Good_Messages_` | 0x501ca | not at war, `NPC_Good_Proposal_Check_` passes: 3× `Random(20)` choose proposal: tech exchange (`NPC_Tech_Exchange_Check_`), NAP 0x61, alliance 0x62, research 0x64, trade 0x63, gift/joint-war offer 0x7d with `Get_Demands_`; always `Clear_Last_Proposal_Turn_` | R! | 421 |
| 2 | `NPC_Good_Proposal_Check_` | 0x2736e | chance from LastProposalTurn, modifiers, relations, HumanBias, diplomat | R! | 115 |
| 2 | `NPC_Tech_Exchange_Check_` | 0x2720f | tech trade offer; `Calc_Tech_Value_`, **Watcom `qsort_`** over tech values | R! | 102 |

**12. Check_For_Council_Meeting_ → Main_Council_Screen_** (UI-driven; state changes inside)

| d | Routine | Addr | Purpose | RNG | Insn |
|---|---|---|---|---|---|
| 1 | `Main_Council_Screen_` | 0x15239 | music, fades, fields; UI loop | R | 76 |
| 2 | `Reload_Main_Council_Screen_` | 0x156d4 | load pictures/text, **picks nominees** (two most populous?) and members' vote weights (`Calc_Council_Vote_` 0x15b90: population-based) — not traced in detail | - | 307 |
| 2 | `Council_Votes_` | 0x15ebc | each AI member: `Vote_Check_` (R!, relations + `Orion_Owner_`) for both nominees → `Council_Vote_Result_` (`Change_Relations_` ×6 + message, UI); then `Council_Player_Vote_` (**human input**; writes CouncilVote +0xe71) | R | 80 |
| 2 | `Council_Finished_Voting_` / `Council_Accept_Ruling_` | 0x15df8 / 0x1660b | 2/3 majority → winner: `Council_Player_Voted_Winner_` (cinematic) or the human accepts/refuses; loser surrender (`Surrender_Empire_`, `Do_Surrenders_`), COUNCIL_FLAG, +0x1f1 | R | 50 / 155 |
| — | UI | | `Get_Council_Message_`, `Control_Main_Council_Screen_`, `Draw_Main_Council_Screen_`, `Load_Council_Text_Messages_`, `Play_Cinematic_`, `Play_Streaming_Music_`, fades, fields | | |

Not triggered by the test pair: `next_council_meeting_turn` = 372, stardate−35000 = 362, COUNCIL_FLAG = 1.

### Shared helpers (listed once)

| Routine | Addr | Module | Purpose | Fields W | RNG | Go | Insn |
|---|---|---|---|---|---|---|---|
| `Declare_War_(i,j)` | 0x51078 | DIPLOMAC.C | WarDeclared for humans; if not at war: clear DemandState, WarLosses(1,2), Diplo70F; `Break_Trade_`, `Break_Research_`, `Break_Tribute_`, `Break_Treaties_`; BaseRelations −10; Relations = −75−Random(25) (both); status 4, or 5/6 (difficulty < 3 / ≥ 3) if a human is involved; Modifiers −200,−200,−200,−130; Diplo7E4 = −1, TurnsAtWar 0, TreatyTimer 0 | many | R! | yes (turn_diplowar.go) | 181 |
| `Break_Treaties_` | 0x5138e | DIPLOMAC.C | end NAP/alliance with relation penalty, Diplo747/7FE | - | | yes (turn_diplowar.go) | 227 |
| `Break_Trade_` / `Break_Research_` | 0x519ac / 0x5175b | DIPLOMAC.C | clear treaty flag + value, relations −Random… (3 draws) | | R! | yes (turn_diplowar.go) | 147 / 144 |
| `Break_Tribute_` | 0x51c02 | DIPLOMAC.C | DemandState+1, clear Tribute both ways, Diplo7E4=0; if a tribute was paid and not at war: Diplo6D7 −10/−20, Relations −(Random(50)+Random(50)) | | R! | yes (turn_diplowar.go) | 113 |
| `Change_Relations_` | 0x4e3b5 | see above | | | R | yes (turn_relations.go) | 406 |
| `Fleet_Strength_Comparison_` | 0x500cf | | | | - | yes (turn_demands.go) | 48 |
| `Get_Player_Diplomacy_Personality_` | 0x53e96 | | | | - | yes (turn_diplomacy.go) | 23 |
| `Get_Demands_` / `Set_Demands_` | 0x4f93b / 0x54cc0 | | | | R! / - | yes (turn_demands.go) | 267 / 33 |
| `Clear_Last_Proposal_Turn_` | 0x27507 | NPCDIPLO.C | LastProposalTurn[j] = 0 both ways | | - | yes (turn_diplomsg.go) | 31 |
| `Star_In_Extended_Range_Of_Player_` / `…Normal_Range…` | 0xff68a / 0xff666 | SHIPMOVE.C | wrappers over `Star_In_Range_Of_Player_` (ported) | | - | wrapper no, core yes | 20 / 12 |
| `Bring_Home_Spies_Vs_` | 0x1019f0 | SPY.C | Spies[j] back to pool (`Get/Set_Their_Spy_Number_` BILL.C) | Spies | - | yes | 35 |
| `Event_Check_Hyperspace_Flux_` | 0x233fa | EVENTS.C | flux event active | | - | yes | 10 |
| `Add_Diplomacy_Turn_Summary_Messages_` → `Add_War/Peace/Alliance/Nonaggression_Msg_` → `Add_Diplomacy_Msg_` → `Add_Msg_` | 0x1fef5 … 0xef629 | DIP-SCRN.C / MSG.C | turn summary messages for status changes (TreatyStatus vs PrevTreatyStatus) | msgs, n_msgs | - | yes (turn_diplomacy.go) | 64 |
| `Diplomacy_Broken_Contacts_` | 0x52602 | DIPLOMAC.C | treaty cleanup for lost contact | | - | yes (turn_diplomacy.go) | 30 |
| `NPC_Treaty_Hatred_` | 0x277cf | NPCDIPLO.C | difficulty > 2: AIs without treaties dislike humans with many treaties (Relations −k/100 and `Random(100)` for the remainder) | Relations | R! | yes (turn_diplomacy.go) | 153 |
| AI-phase helpers (phase B) | | AIMOVE/AITECH | `Find_Opportunity_Attack_`, `Find_Opportunity_Attack_Aux_`, `Find_Best_Attack_From_Star_`, `Player_Threatens_Player_`, `Number_Of_Attacks_Underway_`, `System_Proximity_Count_` | | R | no | |

### The tribute demand in the test pair

Traced from the diff (`before`→`after`) to the code:
- Player 2 (AI) → player 0 (human): `P[2].MessageParam[0]` 0→10000, `P[2].Diplo887[0]` 0→117 (= reason 0x75, "your growth trend", from `Sneak_Attack_Evaluations_` history test), `P[2].Diplo816` 9→22 (= 20+Random(20) −1 in End_Of_Turn), `P[2].ThreatTurn[0]` 181→1 (reset, then +1 at end of turn), `TurnScratch[1]` (0x75f, tribute level) = 2 both ways. That is `NPC_Sneak_Attacks_` result **1**: MessageFlags = 0x6a (tribute-or-war demand), `Set_Demands_` with `Get_Demands_` choosing **tribute level 2** (lvl > 2). The message flag itself is cleared again when shown, so it does not appear in `after.GAM`.
- Acceptance at the begin of the next turn (`Has_Diplomacy_Messages_` → `Npc_Diplomacy_Screen_(2,0)`, report phase, not this group): `P[0].Tribute[2]` 0→2, Relations[0↔2] 0→21, Modifiers[0..2] +22 / [3] +11, TreatyTimer 0→10 both ways, HumanBias −4→1. That handler belongs to the begin-of-turn report phase and must be mapped with the diplomacy screen.
- Other diplomacy changes in the pair: AI 2 ↔ AI 7 war (`NPC_Declarations_Of_War_`: status 4, BaseRelations −10, Relations −100 — `Declare_War_` gives −75−Random(25) ≥ −99, so a later penalty in the same turn pushed it to the −100 clamp (not traced), Modifiers −190 = −200+10, TurnsAtWar 6→0); research treaty 1↔3 (`NPC_To_NPC_Treaty_Negotiations_`); contact 2↔5 lost; TradeValue/ResearchValue creep (`Process_*_Agreement_`); TreatyTimer −1 everywhere; Modifiers +10 drift; PrevTreatyStatus copy.

### Hazards for bit-exactness

1. **Fall-through and jumps into other symbols.** `NPC_Diplomacy_` has no `ret`: it falls into `NPC_Tech_Exchange_`, whose loop body lives inside the `NPC_To_Human_Diplomacy_` symbol (that symbol is a bare `ret` at 0x26990; the bytes after it are the tech-exchange loop). A call graph built from `call` instructions misses these four calls (2× `Random_`, `Player_May_Get_Tech_App_`, `Player_Gets_Tech_App_`). The other cross-symbol `jmp`s in this group are Watcom shared epilogues (`Init_Diplomatic_Relations_`, `Required_Alliances_`, `NPC_Threats_`, …) and harmless, but every routine should be ported from its assembly range, not from the symbol extent.
2. **RNG volume is data dependent** and huge: `Modifier_Diplomacy_Adjustments_` (up to 8 per ordered pair incl. self), `Diplomacy_Growth_` (≥2 per ordered pair + per-treaty rolls), every `Change_Relations_` with reason 10, `Get_Demands_` (short-circuit `||` order of `Random(3)`/`Random(2)` matters), `Sneak_Attack_Evaluations_` (rolls only on some branches; `Random(4)` before `isqrt`). Ghidra reorders `&&` chains; follow the assembly for the evaluation order.
3. **Signed byte arithmetic**: Relations, Diplo6D7, Diplo7EE are `s8`; additions wrap before clamping (`Change_Relations_` adds a short truncated to char, then clamps). Ported code must do the char truncation first.
4. **Loop directions differ**: `Process_Trade_And_Research_Agreements_` and `Compute_Contacts_` walk players/colonies from the end; most DIPLOMAC loops go forward. `Diplomacy_Growth_` iterates i<j for the first part, all i,j later.
5. **Symmetry writes**: many routines write only one direction and rely on a later copy (`Diplomacy_Growth_` rebuilds the whole Relations matrix from the upper triangle at the end; `Change_Relations_` copies j→i). Porting a routine in isolation and comparing against the save will show asymmetric intermediates.
6. **Decompiler mis-reads seen**: watcall arguments dropped (`Required_Alliances_()`, `Declare_Peace_()` shown without args), `iVar5 == -0x72f` in `End_Of_Turn_Diplomacy_Adjustments_` (really a TreatyTimer==0 test), `Reward_Check_` blocks removed as unreachable. Use `procedure_assembly` for these.
7. `stardate` is read both as s32 (`__stardate`) and s16 (`_stardate + 0x7747` = stardate−35001 mod 65536 for the history index, `+0x7761` for the council turn).
8. The council is reached from the turn calculation but is a full interactive screen with human input in the middle of `Next_Turn_Calc_`; for headless verification it needs scripted input.

### Ported (byte-exact with every draw on every traced turn that reaches the step: t1, s2..s5, s7, s8, s9 up to step 17, a1, e_t1, e_s3, e_s5, t1tribute)

Go files: `turn_diplomacy.go` (steps 4, 5, 6, 8, 40, 43), `turn_diplowar.go` (war, peace,
Break_*), `turn_npcdiplo.go` (step 7), `turn_diplomsg.go` (step 42), `turn_trade.go`
(step 17), `turn_council.go` (step 44, the decision only), `turn_demands.go`
(Get_Demands_, Get_Surrender_System_, the player ratings) and, shared with the
AI steps, `turn_aitechsel.go` (Calc_Tech_Value_, Set_Competition_Tech_Values_
into `_best_category`), `turn_aiattack.go` (AIMOVE: Player_Threatens_Player_,
Find_Opportunity_Attack_[_Aux_], Find_Best_Attack_From_Star_, Compute_Attack_Worthiness_,
and the AIPOWER colony defences), `turn_aimove.go` (Gather_Player_Ships_At_Star_) and
`turn_aicolony.go` (Colony_Cost_To_Buy_Given_Product_). What the map above got wrong or left out:

- Loops over pairs that the map calls "i, j" often start at j = i+1 because the
  compiler jumps to the increment first (`Clear_Diplomacy_Messages_` timers and war
  counters, `Diplomacy_Growth_` first block, the turn summary messages).
- `Resolve_Delayed_Diplomacy_Orders_` never acts on a loaded game: `Load_Game_` resets
  DelayedOrder.
- `Set_Opportunity_Attacks_` stores a **planet** in SneakStar (the attack search
  returns the planet; star −1 never appears). The search runs from the AI's staging
  star, difficulty ≥ 2 only.
- `Player_Threatens_Player_(human, AI)`: stars where the human's possible strength
  against the AI reaches a quarter of the AI's strength against it, then a probe of
  the attack search with threshold 150; its "strength" output is the attack's
  strength ratio in percent. `Diplomacy_Growth_` passes the star as arg5.
- `NPC_Diplomacy_Buildup_Penalties_`: two Random(4) per eligible pair (gate, size);
  the scale uses the galaxy size, and the halving applies to AIs (always).
- `NPC_Threats_`: grudges change only between AIs; positive incidents roll
  Random(100) < 2·param to lower DemandState, else MessageThisTurn is cleared.
- `NPC_To_NPC_Treaty_Negotiations_` tests the trade treaty twice (never research),
  and its "anyone receiving tribute" loop tests j each time. `Honor_Alliances_` and
  three `Adjust_Diplomat_Modifiers_` run whenever the pair's 1-in-(250−40·diff) roll
  succeeds and they are in contact, even without negotiation.
  `Eliminate_NPC_Treaty_Conflicts_` reuses its loop variable: the third player is
  always i+1.
- `Start_Tribute_Treaty_` is preceded by a Random(4) whose value is discarded.
- `NPC_Surrender_Check_` divides the average fleet by the largest **population**
  (the largest fleet is computed and unused); its second and third candidate loops
  look for populations below 0 and never match.
- `NPC_Tech_Exchange_`: apps 1..0xcb by field cost (×1000 for kind 0x27); each AI
  gets the other's best application through `Player_Gets_Tech_App_`, which redesigns
  ships (the Auto_Design_Ship_ draws in s2, s3, a1).
- `Sneak_Attack_Evaluations_`: the history test reads `History[2][stardate−35001]`
  directly (past 350 it runs into the next fields; `playerByte` follows the record);
  its stack argument is unused.
- `Get_Demands_(a, b, lvl, tribute, tech, gold, system)`: always builds the tech list
  when tech is asked for; the gold level divides b's BC by a's maintenance. The
  short-circuit order of the Random(3)/(3)/(2) picks decides the draws.
- `End_Of_Turn_Diplomacy_Adjustments_`: Diplo7EC follows player +0xb0 (FoodSurplus
  in our struct); the last loop (timed peace ending with Add_Peace_Ended_Msg_) tests
  the address of the timer array and never runs.
- `Determine_First_Contacts_`: only both-human pairs get a greeting flag directly; an
  AI's greeting is queued with `Change_Relations_(−10000, human, AI, 0x11/0x12)`.
- `Determine_Diplomacy_Messages_`: reasons 10/11 declare war **j on i** (the human
  on the AI), betrayal (0x42) for personality 4 with a treaty. `Peace_Proposal_`
  ignores its third argument. `Unprovoked_Good_Messages_` chooses kind 6 but never
  acts on it; a failed trade offer turns into the joint-war offer (0x7d), whose level
  divides by player NUM_PLAYERS' losses (the loop variable) instead of the target's
  (past the eighth record the port reads 0).
- `Check_For_Council_Meeting_` meets from turn 25 when the turn reaches
  next_council_meeting_turn **or COUNCIL_FLAG is 0**, with more than two players and
  at least half the stars owned; next = stardate − 34975. Main_Council_Screen_
  computes nominees (two highest ⌈pop/10⌉) and member votes, sets COUNCIL_FLAG 1,
  then waits for clicks: the voting (`Vote_Check_`, `Council_Player_Vote_`), the
  ruling and their state changes are not ported (the port logs the meeting).

Synthetic turns d1 and d2 (`build/turns/inputs/d1.GAM`, `d2.GAM`, edits of t1: wars with the human at status 5, LastProposalTurn 250 for four AIs,
P0.WarDeclared[3] = 0, an AI-AI war of 200 turns; d2: the human's spies on three AIs
and raised grudges) pass too and reach Declare_Peace_, Peace_Proposal_,
NPC_Tech_Exchange_Check_, first contact and half of NPC_Surrender_Check_.
Break_Treaties_/Break_Research_/Break_Trade_ are reached by war1 (an alliance with trade
and research treaties broken by a sneak attack). Still never reached (ported,
unverified): Start_Treaty_, Start_Tribute_Treaty_, the empire surrender,
Determine_Bad_Message_, Sneak_Attack_Or_Declare_War_, most of
Determine_Good_Message_, the joint-war offer, the council, the satellite/phasor/fighter
defence ratings and a positive opportunity attack.

### Open questions

- Exact meaning of the message codes (0x42..0x7d) — the text lives with `Npc_Diplomacy_Screen_`/`Get_Diplomacy_Statement_`; map them together with the begin-of-turn diplomacy screen.
- `Reward_Check_` RNG: symbol table says it calls `Random_` twice, the decompile drops those blocks; check the assembly.
- `Reload_Main_Council_Screen_`'s nominees, `Calc_Council_Vote_`'s weights and `Council_Accept_Ruling_` are traced since by cv1..cv5 (`docs/screens/council.md`; cv1, cv2, cv3, cv5 in `TestTurnSteps`).
- `DAT_00199cb0` / `DAT_00199cb2`: assumed difficulty and a second game setting (galaxy age?) from settings; confirm their offsets in `docs/formats/save-misc.md`.
- Diplo887 reason 117 (0x75) matches the "history trend" branch of `Sneak_Attack_Evaluations_` (`local_34 = 0x75`); confirm with a run.

## Phase D: Economy, movement and colonies

Sixteen of the `Next_Turn_Calc_` (0x136b3) steps. They do not run as one block: they are
spread through the turn and interleaved with AI, diplomacy, combat and events. The order
below is the call order inside `Next_Turn_Calc_`; `[..]` marks steps from other phases
that run in between.

```
[AI phase: Compute_AI_Data_ .. All_AI_Tech_Select_]
D1  Make_Scrap_Ships_Dead_
D2  Initialize_Reports_
[Process_Trade_And_Research_Agreements_]
D3  Move_All_Ships_Toward_Stars_
D4  Resolve_Spies_
D5  Apply_All_Player_Changes_
D6  Apply_All_Colony_Changes_
D7  Do_Surrenders_
[Deallocate_AI_Data_, Search_For_Battles_, Determine_Event_, Do_All_Ships_XP_Check_, Event_Twiddle_]
D8  Lose_Out_Of_Range_Ships_
D9  All_Colony_Calculations_            (0xe2b31; objdump labels it Do_Colony_Calculations_)
D10 Compute_Blockades_
D11 Move_Settlers_
D12 All_Colony_Calculations_            (second call)
D13 Allocate_New_Ship_Slots_
[Check_For_Officer_Level_, Decrement_Officer_ETA_]
D14 Check_All_Rebellions_
D15 Repair_Ships_At_Colonies_
D16 Allocate_New_Colony_Slots_
[Compute_Contacts_, Determine_First_Contacts_]
D17 Reset_Needless_Ignore_Combat_Ships_Flags_
[diplomacy, council, officers, msgs, history, stardate+1]
```

Column legend. **RNG**: `R!` calls `Random_` itself, `R` only through callees,
`-` never. **Go**: `yes` = a Go port already exists (found by grepping `internal/` for
the name), `no` = not ported. **n**: the routine's own instruction count, from
`build/orion2.dis`. **sub**: instructions in the game-logic subtree, for phase roots
only. The count stops at engine modules and at the UI-cache refresh
`Update_Ship_Icons_In_Between_Turns_`, and counts each routine once. Field names
come from `docs/formats/save-*.md`. `c.` = colony, `p.` = player, `s.` = ship,
`st.` = star, `l.` = leader.

### D1 Make_Scrap_Ships_Dead_ (0xedf92, MAINTAIN.C) — n 26, sub 161, RNG -

| Routine | Addr | Mod | Purpose | Save fields | RNG | Go | n |
|---|---|---|---|---|---|---|---|
| Make_Scrap_Ships_Dead_ | 0xedf92 | MAINTAIN.C | ships NUM_SHIPS-1..0 with s.status 7 (scrapped) or 8 (destroyed) → Kill_Ship_ | s.status | - | yes (`makeScrapShipsDead`) | 26 |
| · Kill_Ship_ | 0xa163a | SHIPS.C | Ship_Owner_ check; first ship officer (l.type 0, l.location == ship, l.eta > 0) → Move_Officer_To_Pool_; s.location = -1, s.status = 5 (free slot); tail-jumps to the epilogue at 0xa0807 | s.location, s.status, leaders | - | yes (`killShip`) | 49 |
| ·· Move_Officer_To_Pool_ | 0x944a3 | OFFICER.C | unless location −1 or status 5 (marooned): unlink the leader from its ship (s.officer = -1 only if it points at this leader) or star (st.officer[owner] = -1); then always l.status = 0, l.eta = 0, l.location = -1; a colony leader → Do_Player_Colony_Calculations_(owner) | leaders, s.officer, st.officer | - | yes (`moveOfficerToPool`) | 50 |

Kill_Ship_ does not compact anything. A freed slot keeps its old design bytes, owner and
experience until Create_Ship_ reuses it.

### D2 Initialize_Reports_ (0xfd81c, REPORT.C) — n 11, RNG -

Clears `something_interesting_happened[8]`, `n_reports`, `done_colonize_check[9]`,
`done_player_reports`, `displayed_msgs`, `displayed_gnn`, and the BSS byte 0x1991b5. Leaf.
It runs after the AI phase, so reports and messages produced by the AI phase before this
point are discarded: `n_reports` is zeroed here, while `msgs` are allocated earlier by
`Allocate_Msg_Slots_`.

### D3 Move_All_Ships_Toward_Stars_ (0xffeea, SHIPMOVE.C) — n 93, sub 456, RNG - — Go: yes (turn_move.go)

| Routine | Addr | Mod | Purpose | Save fields | RNG | Go | n |
|---|---|---|---|---|---|---|---|
| Move_All_Ships_Toward_Stars_ | 0xffeea | SHIPMOVE.C | network games (game_type 2/3) only: clear st.special_popup (**0x34**, not ignore_combat as mapped first). Then ships 0..NUM_SHIPS-1 with status 1 or 2: location ≥1000 → −500; s.speed == −1 → jump to the star; else one step (speed read unsigned, nebulas ignored with a navigator); s.eta −1; at the star's xy → Make_Ship_Arrive_At_Star_, else status = 1. Other ships: s.ai_mission = 0. Ends with Update_Ship_Icons_In_Between_Turns_ | s.location, x, y, eta, status, ai_mission, st.owner | - | yes (turn_move.go) | 93 |
| · Move_Player_1_Turn_To_Star_ | 0xebb0c | GEO.C | one turn of movement (30 units per speed step, nebula slowdown unless s.navigator, warp interdictors) | s.x, s.y | - | yes (moveorders.go) | 141 |
| · Make_Ship_Arrive_At_Star_ | 0xffdda | SHIPMOVE.C | status 0, location = star, xy = star; ai_mission = 0; st.ignore_combat = 0; owner < 8: Orion star (st.special 11, not yet visited by anyone) → Event_Orion_Discovered_; colony or outpost ship → clear st.colonize_prompt_off bit; first visit → set st.visited and st.special_popup bits; owner 13 (Eel) → design build_date = 0; owner < 8 → something_interesting_happened[owner] = 1, human → st+0x66 = 0 | s.*, st.visited, st.special_popup (0x34), st.colonize_prompt_off (0x35), st.ignore_combat, something_interesting_happened, event_data | - | yes | 85 |
| ·· Event_Orion_Discovered_ | 0x233b8 | EVENTS.C | 0x19acc4 = player, 0x19acc5 = 1: that is `event_data` slot 32, bytes 0 and 1 (saved) | event_data | - | yes | 3 |
| · Update_Ship_Icons_In_Between_Turns_ | 0x8a4c4 | MAINSCR.C | st.owner = Star_Owner_(star, PLAYER_NUM) for every star (**saved**), then Build_Ship_Icons_, Set_Ship_Icon_XYs_ (icon cache) | st.owner | - | yes (turn_move.go) | 22 |

Test pair: s.Location / X / Y / ETA / Status / Speed changes on moving ships. AIMission
resets also come from here (AI missions are set in the AI phase).

### D4 Resolve_Spies_ (0x10192b, SPY.C) — n 60, sub 14 258, RNG R

```
Resolve_Assassins_()
live = players NUM_PLAYERS-1..0 not eliminated (p+0x24 == 0); Compute_Spy_Bonuses_ for each → _spy_bonuses[p] (offence), 0x1ace7a[p] (defence)
Shuffle_Char_(live, n)                      ; RNG
for live from last to first: AI only (p+0x28 != 100) → Allocate_AI_Spies_(p)
Shuffle_Char_(live, n)                      ; RNG
for live from last to first: Resolve_Player_Spies_(p)
```

| Routine | Addr | Mod | Purpose | Save fields | RNG | Go | n |
|---|---|---|---|---|---|---|---|
| Resolve_Assassins_ | 0x1018a3 | SPY.C | leaders 66..0, owned (l.owner ≠ −1) and l.status 0/1, with assassin skill (common_skills byte +0x26 bit 2, value (lvl+1)·3) → Random(100) ≤ chance → Leader_Assassinates_. **Original quirk:** the second skill test is the same bit (0x1018f4), so the lvl·2+2 branch is dead code | leaders | R! | yes | 49 |
| · Leader_Assassinates_ | 0x101768 | SPY.C | pick, weighted by spy count with Random, a player whose spies are in the leader owner's empire (or a fallback), kill one spy; 2 msgs | p.spies (Set_Their_Spy_Number_), msgs | R! | yes | 106 |
| Compute_Spy_Bonuses_ | 0x100a83 | SPY.C | offence/defence = Tech_Spy_Bonus_ + p+0x8a8 + 10·p+0x8b8 + best leader spy skills (+ race table 0x100a36 for defence) | reads p, leaders | - | yes | 99 |
| · Tech_Spy_Bonus_ | 0x100a3e | SPY.C | bonus from known telepathic/security apps | reads p.apps | - | yes | 20 |
| Shuffle_Char_ | 0xfe9b5 | RUSS.C | Fisher–Yates with Random(n−i), i ascending | — | R! | yes | 31 |
| Allocate_AI_Spies_ | 0x100d19 | SPY.C | AI only, skipped if difficulty (0x199cb0) == 0. Counts its spies, may build one (stardate > 0x88d6, difficulty-dependent Random rolls), keeps up to max(enemy spies) at home (cap 63), sends the rest to targets picked with Get_Weighted_Choice_Int_ by stealable tech value; mission 1 (sabotage) with Random(8)==1 if p+0x27 == 3. **Difficulty 4 calls Random(8)/Random(5) and discards the result** — the calls still consume RNG | p.spies[8] (s16 per target), p spy missions | R! | yes | 326 |
| Resolve_Player_Spies_ | 0x1014a4 | SPY.C | for each other player i (NUM_PLAYERS−1..0) with spies in p: two Random(100) per side → mission 0 steal (Steal_App_; ≥90 frames another), mission 1 sabotage (Destroy_Random_Building_, ≥90 Frame_Another_); then a detection roll (two more Random(100)) → spy killed + msg; finally p's own defenders lost | p.spies, msgs, reports | R! | yes | 218 |
| · N_Spies_Bonus_ | 0x101483 | SPY.C | bonus from spy count | — | - | yes | 18 |
| · Steal_App_ | 0x10119c | SPY.C | weighted (by research cost) pick of an app the victim knows and the thief may get → Player_Gets_Tech_App_ (ported); optional Frame_Another_; Change_Relations_(−Random(15)−Random(5)…); 2 Add_Report_ | p.apps, relations, reports | R! | yes | 112 |
| · Destroy_Random_Building_ | 0x10130a | SPY.C | weighted (by building cost) pick among the victim's colonies' buildings except id 9 → Remove_Building_; Change_Relations_; 2 reports | c.buildings, reports | R! | yes | 113 |
| · Frame_Another_ | 0x100bc5 | SPY.C | pick a third player in contact with both, preferring by treaty status bucket, Random per bucket | — | R! | yes | 96 |
| Get/Set_Their_Spy_Number_, Get/Set_Their_Spy_Mission_ | 0x1026cf / 0x10278d / 0x1026f1 / 0x1027b5 | BILL.C | accessors for the per-player spy arrays (p.spies) | p.spies | - | yes | 10–12 |

Test pair: p.Spies changed for 2, 3, 4, 5, 7; reports and msgs.

Go: yes (`resolveSpies`, turn_spies.go; Change_Relations_/Reward_Check_ in turn_diplo.go,
Remove_Building_ in turn_buildings.go), byte-exact on t1, s2..s5 (s3 sabotages a building).
Corrections and findings:

- Allocate_AI_Spies_ reads `_ai_player_info[p]+0xa`, filled by Compute_AI_Data_ (step 3):
  the number of players p has contact with (p.contact[k] ≠ 0, k ≠ p), a word in AI block 12.
  The port reads it from `g.ai`, which `computeAIData` (step 3) fills; nothing between
  steps 3 and 19 changes contacts.
- `_spy_bonuses` (0x1ace78, offence/defence s16 per player) are not saved. Resolve_Spies_
  recomputes them for live players only; eliminated players keep last turn's values
  (`Game.spyBonus`, zero after a load). Defence adds the s8 table 0x100a36 indexed by the
  **government** pick (Picks[0]), not the race.
- Allocate_AI_Spies_ first recalls every spy (p.spies[k] = 0 for all k, home included); the
  weights use field costs of stealable apps scaled by the target's defence, the walk is
  Get_Weighted_Choice_Int_, and each pick *sets* (not adds) up to 63 spies.
- The spy byte is 6 bits of count and 2 of mission (0 steal, 1 sabotage, 2 hide);
  Add_Their_Spy_ stores the count alone and so clears the mission.
- Leader_Assassinates_' messages leave byte 1 (priority) and bytes 6..17 uninitialised;
  Steal_App_ and Destroy_Random_Building_ leave report bytes 8..18 uninitialised (s3: the
  text "ve\0Creative" from an earlier routine). The port writes zeros and marks the bytes
  (`Game.stale`); TestTurnSteps copies them from the original.
- Reward_Check_ (via Change_Relations_ type 10) compares a word with itself before its two
  draws, so that branch never runs; only the "asked to act against" reward remains.
- Change_Relations_ adds to the relation as a byte (it wraps before the ±100 clamp).

### D5 Apply_All_Player_Changes_ (0xe4f49, COLCALC.C) — n 61, sub 14 936, RNG R

```
for p = NUM_PLAYERS-1 .. 0, not eliminated:
    Player_Maintenance_(p)                       ; bankruptcy sell-off when BC + income < 0
    p.bc (+0x32) += p.net_income (+0xb2)
    Update_Player_Stats_(p)                      ; ported
    Check_For_Research_Breakthrough_(p)          ; RNG
    Compute_Player_Total_Known_Tech_Cost_(p)
    income < 0 and bc < 50 → Add_Msg_(type 4)
    food_surplus < 0 or freighters_free < 0 → something_interesting_happened[p] = 1
Unmark_Just_Built_AI_Ships_()
```

| Routine | Addr | Mod | Purpose | Save fields | RNG | Go | n |
|---|---|---|---|---|---|---|---|
| Player_Maintenance_ | 0xee0b0 | MAINTAIN.C | p+0x59d → Kill_Research_Buildings_. If bc+income < 0: ship worths (Ship_Strength_Vs_Ships_ against enemies, else AI players, else all others) into a 2000-byte local table; command points over 4/3 of total → kill ships first; then a fixed sequence of Kill_Buildings_(phase1), Kill_Ships_(…), Kill_Buildings_(phase2), Kill_Spy_, Kill_Leaders_, Kill_Buildings_(all), each stopping once bc+income ≥ 0 | p.bc, c.buildings, s.status, leaders, p.spies, msgs | R | yes | 295 |
| · Kill_Research_Buildings_ | 0xed908 | MAINTAIN.C | colonies NUM_COLONIES−1..0 of p: sell 4 research buildings (table at 0xed904) for cost/2 (+difficulty 0x199cb0 for AI); queue entries of them → −2 | c.buildings, c.queue, p.bc | R | yes | 69 |
| · Kill_Ships_(p, at_colony, mission_filter, hull) | 0xeddf7 | MAINTAIN.C | repeatedly scrap the lowest-worth ship of hull class `hull`; scrap value cost/4 when at an own colony; status = 7, officer back to pool; msg | s.status, s.officer, p.bc, msgs | - | yes | 125 |
| ·· Ship_Is_At_Colony_ | 0xedd73 | MAINTAIN.C | ship's star (or star under xy when departing) has an own colony | — | - | yes | 40 |
| · Kill_Buildings_(p, filter) | 0xedb35 | MAINTAIN.C | filter is a function pointer (indirect call): Phase1_Building_ 0xedae2, Phase2_Building_ 0xedb1d, Phase3_Building_ 0xedb2d. Picks the lowest Building_Worth_, Random(k) on ties, Remove_Building_, BC += cost/2 | c.buildings, p.bc, msgs | R! | yes | 124 |
| · Kill_Spy_ | 0xedcbf | MAINTAIN.C | withdraw spies to cover the deficit | p.spies | - | yes | 57 |
| · Kill_Leaders_ | 0xedfe0 | MAINTAIN.C | dismiss leaders with upkeep (Deassign_Officer_), recompute colonies | leaders | - | yes | 65 |
| Update_Player_Stats_ | 0xe2710 | COLCALC.C | player totals | p.* | - | yes (tech_economy.go) | 237 |
| Check_For_Research_Breakthrough_ | 0xe44e0 | COLCALC.C | if p.research_field ≠ 0: chance = Chance_For_Research_Breakthrough_ (ported); p.breakthrough = 0; p.research_acc += p.research_produced; **Random(100) ≤ chance** → breakthrough = 1, acc = 0, Give_Player_Field_ | p+0x30, p+0x1eb, tech | R! | yes | 27 |
| · Give_Player_Field_ | 0xe4410 | COLCALC.C | field > 0x4a: hyper field +1. Else Advance_Player_Field_ (ported); creative or certain fields → all apps of the field (Player_Gets_Tech_App_, ported), some fields → Init_Ship_Designs_ (R); else the chosen app (p+0x322) | p.fields, p.apps, designs | R | yes | 80 |
| Compute_Player_Total_Known_Tech_Cost_ | 0xe4535 | COLCALC.C | p+0x224 = Σ field cost · known share + hyper levels | p+0x224 | - | yes | 60 |
| Unmark_Just_Built_AI_Ships_ | 0x100828 | SHIPMOVE.C | s.just_built = 0 for AI-owned ships (owner < 8, not human) | s.just_built | - | yes | 24 |

Test pair: p.BC, NetIncome, ResearchPoints, the ship JustBuilt changes.

Go: yes (`applyAllPlayerChanges`, turn_players.go; Auto_Design_Ship_ and its subtree in
turn_autodesign.go; Ship_Strength_Vs_Ships_ in turn_aistrength.go), byte-exact on s2..s5 (s3 and
s5 reach Auto_Design_Ship_ through a breakthrough). Corrections and findings:

- Kill_Ships_' last argument is the **design type** (0 combat, 1 colony, 2 transport, 4
  outpost), not a hull class; the third is "only idle AI missions" (0, 3, 7). The sell-off order
  is in `playerMaintenance`. Kill_Spy_ only recalls spies (no BC), Kill_Leaders_ restarts its scan
  after every dismissal. Building_Worth_ weights government and defence buildings up.
- Give_Player_Field_ gives every application for fields 0x16/0x17/0x1c/0x1d/0x37/0x39 (and
  creative races); 0x16, 0x37 and 0x39 also call Init_Ship_Designs_.
- Auto_Design_Ship_: Get_Ship_Class_ picks a class 0..6 from the ship theme (+0x205) with
  Random(20) (theme 2 draws twice, the first is discarded; class 7 when Init_Ship_Designs_ sets
  0x1998f0 to −1000 for its first design); two Random(4)<4 checks may force class 6; the picture
  is Random(8)+8·size−1 and names come from SHIPNAME.LBX. Each class has a 0x90-byte row of
  per-hull percentages at 0x1812b0+0x90·class (class 7 shares class 0's) that is spent in a fixed
  order on theme specials, defence specials, special lists, fighters, missiles, beams (point
  defence / main / heavy variants) and bombs; `autoDesignShip` lists the orders. The beam
  variants' arcs cost up to four Random(100). Get_Competition_Player_Tech_Values_ reads **player
  0's** applications whatever player it is given. Add_Fighters_To_Design_ prices the space it
  returns with weapon 0x1f whatever bay it fitted. Determine_Missile_Weapon_Modifications_ zeroes
  words of its caller's frame for missiles too weak for the competition's shields (harmless for
  the first four, ignored by the port).
- Ship_Strength_Vs_Shield_ (the AI's strength estimate) is ported but not exercised by any trace
  (only bankrupt players reach it here).

### D6 Apply_All_Colony_Changes_ (0xe3fdc, COLCALC.C) — n 21, sub 11 816, RNG R

Iterates colonies **0..NUM_COLONIES−1 (ascending)**, skipping empty slots and outposts.

| Routine | Addr | Mod | Purpose | Save fields | RNG | Go | n |
|---|---|---|---|---|---|---|---|
| Apply_Colony_Changes_ | 0xe3f6e | COLCALC.C | skip colonies with a space anomaly (Event_Check_Space_Anomaly_, ported). c.occupation 4 → 3; c.turns_owned +1 unless 255. pop > 0 → the four routines below; pop 0 → Ghost_Colony_Stuff_ | c.occupation, c.turns_owned | R | yes | 38 |
| · Apply_Colony_Pop_Growth_ | 0xe2dca | COLCALC.C | occupation 0 (kill policy): Random-pick a conquered or native pop (reservoir sampling, Random(k)==1), remove it; last one → occupation 4 + msg. Negative growth: remove pops per race (Random-pick preferring unemployed), +1000. Positive growth: player order = owner first, then **Shuffle_Sint_ of the others**; add pops while frac ≥ 1000 and below Colony_Race_Pop_Limit_; the new pop's job is chosen by food/industry need or Get_Weighted_Choice_Int_ (RNG); at the cap frac = 999 | c.pop_count, c.colonists, c.pop_fraction, c.occupation, msgs | R! | yes | 461 |
| · Apply_Assimilation_ | 0xe3456 | COLCALC.C | occupation 3: rewrite colonist owner bits of eliminated races; c.assimilation += race rate (table 0xdd4f5; ×2 / ÷2 by p+0x8b3/0x8b2; 120 with building 1); per 240: Random-pick a conquered pop (bit 0x400) and assimilate it; last one → occupation 4 + msg | c.assimilation, c.colonists, c.occupation, msgs | R! | yes | 131 |
| · Produce_Ground_Military_ | 0xe3616 | COLCALC.C | marines/armor +1 every 5 turns up to the barracks limit, −1 when above the plain limit | c.marines, armor, marine/armor_progress | - | yes | 38 |
| · Apply_Production_ | 0xe36df | COLCALC.C | see below | c.queue, c.production, c.last_built, c.pending_production, buildings, ships, p.bc, p.freighters, msgs | R | yes | 547 |
| · Ghost_Colony_Stuff_ | 0xe3e9a | COLCALC.C | pop-0 colony: each building has a Random(10)==1 chance to be removed (Remove_Building_); no buildings left and no settler heading to the planet → Destroy_Colony_(c, 3) | c.buildings, c.* | R! | yes | 68 |

**Apply_Production_(c)** (0xe36df):
1. Space anomaly → return. c.auto_build and the queue head is a "special" code (≥ 0xfffd) → Player_Colony_Autobuild_.
2. cost = Colony_Product_Cost_(queue[0]); last_built = −1. Housing (−3) / trade goods (−2): return; but housing at max population → last_built = −3, queue[0] = −1, Fix_Colony_Queue_.
3. total = production + pending_production (pending cleared) + surplus industry (output[1] above industry upkeep). total < cost → store total (capped 0x7fff), done.
4. Completed product:
   - building 1..0x30 → Add_Building_, p.bc += table 0x17eb49; repeating items (queue[1] == −10) recompute and requeue if still buildable; 0x2c re-check, 0x30 → Delete_Obsolete_Artifical_Planets_Under_Construction_;
   - ship (queue ≤ −100, ship id = −(q+100)): refit (status 4) → status 0, s.cost = Cost_Of_Design_ (ported); new ship (status 6) → status 0, with "repeat" → Create_Ship_ a copy, status 6, queue = its id; s.just_built = 1, st.ignore_combat = 0, colony/outpost ship → set st.colonize_prompt_off; crew level +1 with building 0x26;
   - −15 freighters: p.freighters +5, BC +5; −7 spy: Add_Their_Spy_ (refund 100 BC when full) +1 BC; −6/−5/−4 marines, colonists, androids: add a pop if Room_For_Another_Dude_ (else evict a non-owner pop + msg 11, or msg 10 + refund).
5. total −= cost; Fix_Colony_Queue_; empty and auto_build → Player_Colony_Autobuild_; Add_Msg_(type 1); store total.

| Routine | Addr | Mod | Purpose | RNG | Go | n |
|---|---|---|---|---|---|---|
| Player_Colony_Autobuild_ | 0xd2783 | AIBUILD.C | Colony_Calculation_ + Assign_Colony_New_Building_ | R | yes | 16 |
| Colony_Calculation_ | 0xe2a70 | COLCALC.C | Pre_Import_Computing_, Pass_Out_Imports_, Update_Player_Stats_ for one colony's owner (all three ported, wrapper not) | - | yes | 21 |
| Assign_Colony_New_Building_ | 0xd0b08 | AIBUILD.C | score buildable products (Colony_Can_Build_Product_, Colony_Building_Score_), drop the weak ones by difficulty, plus housing/trade, **Get_Weighted_Choice_Short_** → queue[0] | R | yes | 118 |
| Colony_Can_Build_Product_ | 0xe11bc | COLCALC.C | product legality | - | yes | 427 |
| Colony_Building_Score_ | 0xd0036 | AIBUILD.C | AI value of a building | - | yes | 760 |
| Colony_Product_Cost_ | 0xe0dd6 | COLCALC.C | cost of a queue code (government reduction, terraform count) | - | yes | 116 |
| Fix_Colony_Queue_ / Defragment_ / Squish_Out_ | 0xbaa3b / 0xba9a5 / 0xba9ec | COLONY.C | compact the 7-entry queue | - | yes | 15/28/23 |
| Add_Building_ | 0x13fd9 | ERICNET.C | set c.buildings[id] and its side effects (terraform, Gaia, artificial planet, barracks, star colony stuff, ETAs) | R! | yes | 390 |
| Create_Ship_ | 0xaf7b4 | COLBLDG.C | **first slot 0..499 with s.owner == p and (status 5 or index ≥ NUM_SHIPS)**; grows NUM_SHIPS; memset, copy design p+0x326+99·d, owner/builder = p, at the colony's star, officer −1; with 0x199cb4 set, a numbered name and p+0x578[size]++ | - | yes | 158 |
| Add_Their_Spy_ | 0x10294b | BILL.C | +1 spy at home if below the cap | - | yes | 20 |
| Room_For_Another_Dude_ | 0xe0c9b | COLCALC.C | room for one more pop | - | yes | 81 |
| Remove_Building_ | 0x145ea | ERICNET.C | remove a building (home planet move, queue, environment, Enforce_Population_Limits_At_Colony_, recalc) | R | yes | 135 |
| Enforce_Population_Limits_At_Colony_ | 0xec97c | INVASION.C | **Shuffle_ the colonist list (4-byte items)**, then drop surplus pops per race limit (natives, others, conquered, androids, own); no food → farmers out | R | yes | 157 |
| Destroy_Colony_ | 0xdcdac | BOMB.C | kill the colony (events, leader, queue, msg) | - | yes | 88 |

Test pair: c.Production (64), Queue (21), LastBuilt (18), PopCount/Colonists/PopFraction,
Assimilation (75), TurnsOwned (59), MarineProgress/Marines, Buildings (4); new ships
(Design bytes on reused slots 106, 127, 158, 160).

Go: yes (`applyAllColonyChanges`, turn_colony.go; Add/Remove_Building_, the queue helpers and
Colony_Can_Build_Product_ in turn_buildings.go; Assign_Colony_New_Building_ and
Colony_Building_Score_ in turn_aibuild.go), byte-exact on t1, s2..s5. Corrections:

- **Step 21 reads the AI data of step 3.** Assign_Colony_New_Building_ (reached through
  Player_Colony_Autobuild_ for humans too) and Colony_Building_Score_ read `_ai_colony_info`
  (0x1aa1ec, 7 bytes per colony: dominant race, its pop limit, food flag, +3 (never written),
  tolerant flag, enemy ship within 9 turns, conquered pops) and `_ai_star_info` (0x1aa1f8,
  0x31 bytes per star: planets, then per colony owner p at +1+6p allied/neutral/war/far player
  counts and colony stars in range), both filled by Compute_AI_Data_ (`computeAIData`,
  docs/turn/ai-data.md). The port reads them from the AI arena `g.ai`. TestTurnSteps uses
  the original's arena from aidata.bin, or for older traces runs `computeAIData` on the
  state step 3 saw (`turnStepsReadingAI`). Both pass on every trace.
- Pop growth: only players grow (natives and androids never); the owner goes first, the
  others are shuffled with Shuffle_Sint_. A shrinking race loses a worker or scientist when
  it has one, else a farmer. The new pop's job: farmer when food demand exceeds twice the
  food output (lithovores and cybernetics differ at the margins) or the empire's food surplus
  is negative, or zero with a later non-outpost colony of the owner (the scan starts after
  this colony); worker when industry upkeep is not covered; scientist under a research
  event; else Get_Weighted_Choice_Int_ over the colony's worker and scientist counts. New
  pops get bit 0x200.
- Apply_Assimilation_ first gives the owner the loyalty of pops whose player has
  government pick 0 (Picks[0] of that player, as coded).
- Troop limits: infantry is pop (or the size table 0x17d806) halved, tanks quartered;
  warlords divide by half as much.
- Colony_Building_Score_ case 8 (and 41) jump into case 40's tail with the partial sum in
  eax, so the sum (not a2) is shifted left by four; ported as executed.
- Fix_Colony_Queue_ uses COLONY.C's Squish_Out_Colony_Queue_Slot_ (0xba9ec), which keeps
  repeat markers; Delete_From_Colony_Queue_ uses the one at 0xb20ed, which squeezes them.
- Ghost colonies are destroyed by Destroy_Colony_ (Kill_All_Events_At_Colony_,
  Handle_Leader_At_Lost_Colony_, Kill_Officer_, Clear_Colony_Queue_) only when no settler
  targets the planet.

### D7 Do_Surrenders_ (0xe4dc9, COLCALC.C) — n 43, sub 13 404, RNG R

Stars NUM_STARS−1..0, players NUM_PLAYERS−1..0: st.surrender_to[p] (0x67+p) ≠ −1 →
Do_Surrender_Star_(star, p, to). Then players NUM_PLAYERS−1..0 with p.surrender_target
(+0xe72) ≠ −1 → Do_Surrender_Empire_. It is also called from the council/diplomacy code
at 0x16836 (outside the turn).

| Routine | Addr | Mod | Purpose | Save fields | RNG | Go | n |
|---|---|---|---|---|---|---|---|
| Do_Surrender_Star_ | 0xe493a | COLCALC.C | orbits 4..0: colonies of p → Do_Surrender_Colony_; st.colony_owners −p +to, st.visited +to; Compute_Star_Colony_Stuff_ (ported); st.colonize_player = −1; star governor → pool; surrender_to[p] = −1 | st.*, c.* | R | yes | 66 |
| Do_Surrender_Colony_ | 0xe481f | COLCALC.C | c.owner = to; clear building 0x13f, auto_build; colonist owner bits p → to&7; Compute_Star_Colony_Stuff_; warp interdictor → Update_Ship_ETAs_(−1); environment; Enforce_Population_Limits_At_Colony_ (RNG); clear queue; recompute; turns_owned = 0; visited | c.*, st.* | R | yes | 87 |
| Do_Surrender_Empire_ | 0xe4b5f | COLCALC.C | all ships of p → to (officers deassigned); to gets p's known apps (Player_Gets_Tech_App_); all colonies surrender; spies merged (cap 63); freighters and BC added; Eliminate_Player_ | s.owner, c.*, p.* | R | yes | 104 |
| Eliminate_Player_ | 0xe45ff | COLCALC.C | p.eliminated = 1; destroy outposts; leaders dead (−2) and unlinked; Kill_Ship_ all ships; zero mutual treaties/spies (Bring_Home_Spies_Vs_); BC/freighters 0; record the elimination date for the killer (+0x1f2) | p.*, leaders, ships | - | yes | 132 |

**Test pair:** colony 54 at star 31 changed owner 2 → 1 (TurnsOwned → 0, colonist owner
bits → 1, st.surrender_to[2] 9 → −1, st.colony_owners 4 → 2). The only writer of −1 to
surrender_to during the turn is Do_Surrender_Star_, so the change belongs to this phase.
The before-save holds **9**, not 1, in surrender_to[2]. Do_Surrender_Colony_ would then write
c.owner = 9 while the colonist owner bits become 9&7 = 1. See the open questions.

Go: yes (`doSurrenders`, turn_surrender.go), byte-exact on t1, s2..s5 (t1 and s5 hand over
colonies). By step 22 star 31's surrender_to[2] holds 1, not 9: an earlier step of the turn
(diplomacy) rewrites it, so the open question below is about that step, not this one.
Corrections: Do_Surrender_Empire_ hands over only applications 0..0x52 (its loop bound is the
field count, 83), merges the loser's spies everywhere into the winner's home spies, and
Eliminate_Player_ records the date for the player in the loser's byte +0x204 (`CombatSlot`)
when it is below 8. Empire surrender and elimination traced by sur1
(`build/turns/inputs/sur1.edit`: t1 with player 3's surrender_target = 4), byte-exact
in `TestTurnSteps/sur1` and `TestTurnChain/sur1`.

### D8 Lose_Out_Of_Range_Ships_ (0xed8bf, LOSTSHIP.C) — n 29, sub 1 495, RNG R — Go: yes (turn_move.go)

For p = NUM_PLAYERS−1..0: Compute_Stars_In_Range_Of_Player_ (normal and 1.5× fuel range),
Player_Lose_Out_Of_Range_Ships_, then **Process_Retreating_Ships_ is called once per
player**, inside the loop.

| Routine | Addr | Mod | Purpose | Save fields | RNG | Go | n |
|---|---|---|---|---|---|---|---|
| Compute_Stars_In_Range_Of_Player_ | 0xed7a3 | LOSTSHIP.C | stars from the last: normal (p+0x324) then extended ((3r+1)/2) via Star_In_Range_Of_Player_ | — (stack residue, see below) | - | yes (turn_move.go) | 50 |
| Player_Lose_Out_Of_Range_Ships_ | 0xed82f | LOSTSHIP.C | p's ships at a star (status 0), from the last: out of extended range, or out of normal range without the range special (bit 11) → status 9. Its Update_Ship_Icons_ call hangs on a local that stays 0 (dead) | s.status | - | yes | 46 |
| Process_Retreating_Ships_ | 0xe6e52 | COMBFIND.C | ships NUM_SHIPS−1..0. **Any** ship with location == NUM_STARS (Antares): per owner, once per call, a star with the owner's dimensional portal (st+0x40) by reservoir sampling (Random(k)==1, stars from the last); found → status 0 there; none → officer to pool (eta > 0) or killed, Kill_Ship_, counted at (owner, NUM_STARS) kind 0. Status 9 → the stack of retreaters with the same owner and location (ai_mission 0), Closest_Other_Colony_Star_ + Ships_Try_To_Move_To_; no star or cannot move → each ship (last first) killed, officers to pool when eta > 0 else Kill_Officer_, kind 2 (1 under hyperspace flux); else Make_Ships_Move_To_ and status 1, location −500. Antaran (owner 8) at a star → Restock_Antaran_Ship_(hull) + Kill_Ship_. Monster at a star with design byte +0x14 (FTL) ≠ 0: owner 10 → Kill_Ship_ + Russ_Event_Monster_Update_(10, −1, −1); owners 11, 12, 14 → Random(2); on 1 the same Kill_Ship_ + Russ_Event_Monster_Update_(owner, −1) (the jump after `cmp eax, 1` lands on the owner == 10 test's jne at 0xe71fa; ap8). Then per player (from the last) and star NUM_STARS..0 with losses: msg 0x0e (byte 2 star, 3 kind, 4–5 count, 6 officers lost); Update_Ship_Icons_In_Between_Turns_ | s.*, leaders, antarans+0x2a, event_data, msgs | R! | yes (turn_move.go) | 315 |
| · Russ_Event_Monster_Update_ | 0xf9711 | NETMOX.C | network: Mox_Send_Message_ (**multiplayer only**, not ported); else Event_Monster_Update_ | event_data | - | yes (single player); traced: evm1 (`build/turns/evm1.sh`, SAVE5's Amoeba given a drive and arriving at its own star, killed here), `TestTurnSteps/evm1` | 24 |
| · Event_Monster_Update_ | 0x2332c | EVENTS.C | monster 10..14 → event 0x13..0x17: byte 0 = star arg, loc ≠ −1 → +3 = loc, else +5 = star and (star == loc) +2 = 0x14; state (+1) = 4 | event_data | - | yes | 36 |
| · Kill_Officer_ | 0x941c6 | OFFICER.C | unlink from star (colony leader, owner 0..7) or ship; status −2, eta 0, location −1; colony leader → Do_Player_Colony_Calculations_ | leaders, st.officer, s.officer | - | yes (turn_move.go) | 44 |
| · Restock_Antaran_Ship_ | 0x6485f | ANTARANS.C | s16 counter antarans+0x2a[hull] − 1, floor 0 (saved) | antarans | - | yes | 14 |

**Uninitialised message bytes.** Process_Retreating_Ships_ builds its type-0x0e message in a local
that it fills only at bytes 0, 2–6; bytes 1 and 7–17 are whatever Compute_Stars_In_Range_Of_Player_
left there, and they are saved. With Lose_Out_Of_Range_Ships_'s esp = E (after `enter`) the message
sits at E−0x68, exactly over the frames of Star_In_Range_Of_Player_ → _Aux2_ → _Aux1_ as called by
Compute_Stars_In_Range_Of_Player_: bytes 0–3 Aux1's pushed ecx (cl = player, **ch = 0**, so the
priority byte 1 is 0 and a full window is never overwritten), 4–7 Aux1's return address (0xff5c5 or
0xff5f2, +0x224000 at run time: byte 7 = 0), 8–11 Aux2's [ebp−8] (player of the last Aux2 call),
12–15 Aux2's [ebp−4] (range of the last Aux2 call that got past its own-colony test), 16–17 the low
word of Star_In_Range_Of_Player_'s ebp = E−0x3c. Next_Turn_Calc_'s esp when calling a step is
0x3f3bac in the DOSBox-X runs (found from saved-ebp chains in rng.log, e.g. the Do_Surrenders_
draws), so that ebp is 0x3f3ac8; the step traces' trampoline added a call level, 4 bytes less. The port tracks the Aux1/Aux2 values (`State.rangeResidue`). Not yet
checked against an oracle with a retreat loss. Ships of owners ≥ 8 at location NUM_STARS would
index the 8-byte per-owner pick array past its end into other locals (not modelled; they never
sit there in the saves seen).

### D9/D12 All_Colony_Calculations_ (0xe2b31, COLCALC.C) — n 121, sub 4 949, RNG -

Assert_Settings_ (empty), then Pre_Import_Computing_ for colonies NUM_COLONIES−1..0 (not
outposts), Pass_Out_Imports_(p) for p 0..7, Colony_Pop_Grows_ and Colony_Specialty_ for
colonies from the last down, and Update_Player_Stats_ for p 0..7. All of these are ported
in `tech_economy.go`. It then recounts p+0x5e8 (fields known), p+0x5ea (fleet size
score, Σ 1<<hull of combat ships status < 5), p+0x60c (colonies with pop ≥ 5). **The
wrapper and the three counters are not ported** (Do_Player_Colony_Calculations_ is the
per-player variant). Test pair: p.FleetSizeScore, the colony Output/Upkeep/Food fields.

### D10 Compute_Blockades_ (0xe5097, COLCALC.C) — n 139, sub 545, RNG R! — Go: yes (turn_move.go)

Clears st.blockaded (0x2a) and st.blockaded_by[8] (0x2b), stars from the last. Ships from the
last, combat (type 0) at a star (status 0, location < 0x48): a player owner o blockades every
colony owner q ≠ o (players from the last) it has treaty status 4..6 with (st.blockaded |= q,
st.blockaded_by[q] |= o); a monster or Antaran **assigns** st.blockaded = st.colony_owners (the
blockaded_by masks stay). Then stars from the last with a blockade, blockaded players p and
blockaders q (both from the last) that have no colony there:
**Change_Relations_(−Random(5), q, p, 7, star, 0)** — the draw happens before Change_Relations_'s
own checks. Change_Relations_ and Reward_Check_ are ported with it (turn_relations.go).

### D11 Move_Settlers_ (0xff212, SETTLER.C) — n 176, sub 519, RNG - — Go: yes (turn_move.go)

Players NUM_PLAYERS−1..0 (eliminated ones too), settlers 0..p.settlers_in_transit−1 (player+0x42,
u32: bits 0–7 source colony, 8–15 target planet, 16–19 race, 20–23 ETA, 24–25 job): ETA −1 (mod
16); low byte forced to 0xfa (**quirk:** it overwrites the source-colony byte). At ETA 0: counted
for the planet; unless the target star is blockaded for p or has an anomaly
(Event_Check_Space_Anomaly_At_Star_), and the target planet holds p's non-outpost colony →
Add_Settler_To_Colony_, whose failure (no room) is counted separately. The entry is removed by
memmove. If anyone arrived, per target planet from 359 down: msg bytes 0 = 0, 2 = planet, 3 = 5,
4 = arrived, 5 = failed, 6 = blockade bit. Writes p+0x40, p+0x42[], colonists.

| Routine | Addr | n | Purpose |
|---|---|---|---|
| Add_Settler_To_Colony_ | 0xff015 | 90 | Room_For_Another_ (without settlers) or fail; a farmer settler when farmers ≥ c.can_farm: race < 8 → its job becomes 2, else the last farmer of race < 8 becomes job 2; the new pop: race, loyalty race&7, job, bit 9 set, bit 10 clear. Returns success. **Go: yes** |
| Room_For_Another_ | 0xfed77 | 80 | pops (and optionally the owner's settlers bound for the colony) whose race limit ≤ this race's limit, < that limit. **Go: yes** |

### D13 Allocate_New_Ship_Slots_ (0x10011b, SHIPMOVE.C) — n 165, RNG - — **the ship.Owner churn**

This routine is not compaction. It **re-reserves every free ship slot for a player**:
Go: yes (`allocateNewShipSlots`), byte-exact on t1.

1. a = Σ of the 9 s16 at 0x19918c..0x19919c (antarans block +0x18..+0x28, saved);
   a > 20 → 20, else a + 2 (so up to 22);
2. per player, count queue entries > −100 in non-outpost colonies 0..NUM_COLONIES−1 → want[p]; human
   (game_type 0) reserve = min(want[PLAYER_NUM], 20);
3. slots 0..499 that are free (status 5, or index ≥ NUM_SHIPS): the first a → owner 8
   (Antarans); the next up to the reserve → PLAYER_NUM; the rest are listed;
4. the listed slots are dealt round-robin **starting at player stardate % NUM_PLAYERS**
   to players with want[p] > given[p] (the human's count starts at its reserve), until
   a full round finds nobody; leftovers go round-robin, continuing from the same player,
   to non-eliminated players (no living player: endless loop).

Create_Ship_ later takes the first slot with s.owner == builder. Free slots therefore
change owner every turn: the starting player rotates with the stardate. That is the
342 s.Owner changes in the test pair (shifts like 7→5, 5→4, 4→3, 3→2, 2→1, 1→0, 0→7).
The 173 s.Design differences are slots reused by Create_Ship_ (new ships: the whole 99-byte
design), not this routine. Port it byte-exact, slots ≥ NUM_SHIPS included: they are
saved too.

### D14 Check_All_Rebellions_ (0xed44a, INVASION.C) — n 21, sub 13 863, RNG R

Colonies 0..NUM_COLONIES−1, not outposts → Check_Rebellion_.

| Routine | Addr | Mod | Purpose | RNG | Go | n |
|---|---|---|---|---|---|---|
| Check_Rebellion_ | 0xed260 | INVASION.C | occupation ≠ 4; conquered pops (bit 0x400) of live races counted per race; k > 0: rebel race = Get_Weighted_Choice_Char_; chance 10k (+difficulty·4−8 human vs AI, ÷2 with building 1, ×2 under occupation 0); **Random(1000) ≤ chance** → rebels = Random(k), Get_Rebellion_Info_, Ground_Combat_Round_ until one side breaks, 2 msgs; rebels win → Change_Colony_Ownership_(c, race, 0), Resolve_Rebellion_Troops_, last_built = −41 | R! | yes | 155 |
| Get_Rebellion_Info_ | 0xec65a | INVASION.C | troop tables (militia, ground bonuses, officer bonus) | - | yes | 76 |
| Ground_Combat_Round_ | 0xec4fe | INVASION.C | Random(100) per side | R! | yes | 85 |
| Change_Colony_Ownership_ | 0xecf41 | INVASION.C | war-loss stats, Change_Pop_Ownership_, remove buildings 2/0x16/1, clear troops; with a report buffer (invasion only): tech capture rolls Random(1000) and Get_Weighted_Choice_Short_ | R! | yes | 217 |
| Change_Pop_Ownership_ | 0xecbf7 | INVASION.C | owner change, leader, home planet, environment, Enforce_Population_Limits_At_Colony_ (Shuffle_), queue, star colony stuff | R | yes | 143 |
| Resolve_Rebellion_Troops_ | 0xececa | INVASION.C | surviving rebels → marines (limit) | - | yes | 33 |

Go: yes (`checkAllRebellions`, turn_rebellion.go), byte-exact on t1, s2..s5 and reb1
(`build/turns/inputs/reb1.edit`: t1 with every pop of 14 colonies conquered from another
living race, occupation 0, garrisons removed from ten): five rebellions win the ground battle
and change the owner (colonies 0, 4, 7 (the human's), 9, 45), the others are put down.
Details: the rebel race is weighted by its conquered pops, the chance is 10 per conquered pop
(+4·difficulty−8 against a human owner, halved with building 1, doubled under the kill policy);
the battle (Get_Rebellion_Info_, Compute_Player_Ground_Combat_Bonuses_,
Compute_Colony_Officer_Bonus_, Compute_Ground_Combat_Info_, Ground_Combat_Round_) runs class by
class (armor, marines, militia, rebels), one Random(100) per side, the lower side (both on a
tie) taking a hit. Change_Colony_Ownership_ is ported with its invasion branch (tech capture
into the combat report, `captureReport`), which the rebellion does not use.

### D15 Repair_Ships_At_Colonies_ (0x580f5, INITSHIP.C) — n 74, sub 159, RNG -

Go: yes (`repairShipsAtColonies`), byte-exact on t1, s2, s3 (s3 refits 13 drives).

Best_Warp_Drive_ (ported) for players 0..7. Ships 0..NUM_SHIPS−1, ascending: owner < 8
(signed) at a star (status 0) whose drive differs from the best (also a downgrade) →
combat_speed = combat_speed − s8 table[old drive][size] + table[new drive][size] (drive
table 0x17fe70 + 0x20, byte arithmetic), drive = best. Then Repair_Ship_Full_ (0x581f3:
zero shield/drive/computer/armor/structure damage, working = count for the 8 weapon
slots, damaged_specials = 0) for **every ship with owner ≥ 9** (monsters, any status)
and for combat ships (design type 0) with owner ≤ 8 at a star (status 0, location
0..0x47) where Test_Bit_Field_(colony_owners, owner): owner 8 tests bit 0 of the next
byte, warp_interdictor.

### D16 Allocate_New_Colony_Slots_ (0xe64f4, COLONIZE.C) — n 80, sub 492, RNG -

| Routine | Addr | Purpose | Save fields | n |
|---|---|---|---|---|
| Find_Colonize_Players_ | 0xe6170 | st.colonize_player = 8 (stars descending); ships descending, status 0, design type 1/4 at a star: first claimant takes the star and sets its colony/outpost mask bit (a second ship of the same owner sets no bit), another owner → −1; colonies descending with a colony base (building 0xb, +0x141) likewise (colony mask); stars descending: still 8 → −1, else kept only if (colony bit and Star_N_Colonizable > 0) or (outpost bit and Star_N_Outpostable > 0); armed ships (type 0, status 0) of another owner ≥ 8 or with treaty status 4..6 (claimant's view) → −1; a colony base whose star is not its owner's or has no colonisable planet is trashed: building 0xb = 0, BC += s32 0x17ec16 / 2, Add_Msg_ {+2 planet low byte, +4 word 0xb, +6 word refund} | st.colonize_player, c.buildings, p.bc, msgs | 261 |
| Star_N_Colonizable/Outpostable_Planets_For_Player_, Star_N_Uncolonized_Planets_ | 0xe60c8 / 0xe6132 / 0xe608b | counts | — | 43/26/27 |
| (body) | | c.reserved_for = −1 for all 250 slots; free slots (index ≥ NUM_COLONIES or c.planet == −1) are dealt round-robin from **stardate % NUM_PLAYERS** to players up to their uncolonised-planet count at stars they claim, until a full round finds nobody | c.reserved_for | 80 |

Go: yes (`allocateNewColonySlots`, turn_colonize.go), byte-exact on t1, s2, s3. Test pair:
c.ReservedFor (1), st.ColonizePlayer (star 8: 0 → −1).

Departures: the message of a trashed colony base takes bytes 1 (priority) and 8..17 from
stale stack, the port sends zeros; the deal does not stop at the end of the free-slot
list (stale stack again), the port drops reservations past it. Neither is seen in the
traced turns.

### D17 Reset_Needless_Ignore_Combat_Ships_Flags_ (0x10087d, SHIPMOVE.C) — n 102, RNG -

Go: yes (`resetNeedlessIgnoreCombatShipsFlags`), byte-exact on t1, s2, s3 (nothing cleared in
any of them). Per star a u16 mask of owners of ships at a star (status 0, any owner ≠ −1,
monsters included) and of colonies; stars descending with ignore_combat ≠ 0, players
NUM_PLAYERS−1..0: bit p set and nobody but p present → the bit is cleared.

### Shared helpers (used by several phases)

| Routine | Addr | Mod | Purpose | RNG | Go | n |
|---|---|---|---|---|---|---|
| Add_Msg_ | 0xef629 | MSG.C | append an 18-byte msg (by value, then player) to p's window; nothing if max_msgs 0; when full, scan slots n (one **past** the window) down to 1 and stop at the first with priority (byte +1) below slot 0's, else slot 0; replace it if the new one ranks higher. For the last window slot n is the memory after `_msgs`: `_n_msgs`, `_max_msgs`, `_msg_april1_fmt` (ported except the pointer) | - | yes (`addMsg`) | 55 |
| Add_Report_ | 0xfd846 | REPORT.C | append a 19-byte report if n_reports < 50 | - | yes | 18 |
| Shuffle_ / Shuffle_Sint_ / Shuffle_Char_ | 0xfea82 / 0xfe9f5 / 0xfe9b5 | RUSS.C | Fisher–Yates, Random(n−i), i = 0.. | R! | yes | 74/28/31 |
| Get_Weighted_Choice_Int_/Short_/Char_ | 0xfe96f / 0xfe92d / 0xfe8da | RUSS.C | sum = 0 → −1 (no RNG); else r = Random(sum), walk while w[i] < r | R! | yes | 36/31/42 |
| Update_Ship_ETAs_ | 0x10041c | SHIPMOVE.C | recompute s.eta of moving ships by simulating Move_Player_1_Turn_To_Star_ (skips owner == arg) | - | yes | 76 |
| Change_Relations_ | 0x4e3b5 | DIPLOMAC.C | (delta, a, b, kind, arg5, arg6): b's attitude to a; scaled by the current relation, personality 0/4, a's Charismatic, war treaties (kinds 1..9 against a human only move modifier 3), AI-vs-AI after stardate 35100 (×2 / ÷(difficulty/2+1)); clamps ±100, −25 at war, 65 without alliance; modifiers 0..3; largest change → b.MessageParam/MessageThisTurn[a]; kind 10 → Reward_Check_ (whose Random branch compares WarLosses[i] with itself and is dead) | - | yes (turn_relations.go) | 406 |
| Update_Ship_Icons_In_Between_Turns_ | 0x8a4c4 | MAINSCR.C | UI cache; called many times (Kill_Ships_, Process_Retreating_Ships_, Eliminate_Player_, Do_Surrender_Empire_, D3). Can be dropped from the logic if nothing in the turn reads the icon/stack arrays. AI routines do read ship stacks, so check that first | - | partly | 22 |

### Bit-exactness hazards

- **Iteration direction differs from routine to routine.** Ascending: D3, D6, D13,
  D14, D15. Descending: D1, D4 player list, D5 players, D7, D8, D9 colonies, D11 players.
  Reservoir sampling (`Random(k) == 1` keeps the k-th candidate) makes the RNG count
  equal the candidate count, so the scan order decides which candidate is kept.
- RNG consumers in this group: Resolve_Spies_ (two shuffles + many Random(100)),
  Check_For_Research_Breakthrough_ (one Random(100) for every player with a research
  field, every turn), Apply_Colony_Pop_Growth_ (Shuffle_Sint_ of NUM_PLAYERS−1 entries for
  **every** growing colony even with one race present), Apply_Assimilation_,
  Ghost_Colony_Stuff_, Kill_Buildings_ ties, Compute_Blockades_, Process_Retreating_Ships_,
  Check_Rebellion_ (one Random(1000) per colony with conquered pops), Enforce_Population_Limits_At_Colony_
  (Shuffle_ even when nothing is removed).
- Allocate_AI_Spies_ on difficulty 4 discards Random(8)/Random(5) results; the calls still advance the seed.
- Resolve_Assassins_ tests skill bit 2 twice (0x1018e2, 0x1018f4); keep the dead branch dead.
- Move_Settlers_ overwrites the settler's low byte with 0xfa each turn.
- Allocate_New_Ship_Slots_ / Create_Ship_ / Allocate_New_Colony_Slots_ depend on `stardate % NUM_PLAYERS` and run
  over all 500 / 250 slots, including slots beyond NUM_SHIPS / NUM_COLONIES.
- Apply_All_Colony_Changes_ runs before Do_Surrenders_ and combat; All_Colony_Calculations_
  runs twice (before and after Move_Settlers_), so the second pass sees the settlers.
- Kill_Buildings_ uses a function pointer (Phase1/2/3_Building_), so the call graph built
  from objdump misses its callees.
- Do_Surrenders_ and Surrender_Star_ end with a tail jump into a shared epilogue under
  another symbol (0xe1f97, 0xe44db); the call lists are unaffected.
- None of the Get_/Set_Random_Seed_ pairs (Advanced_Civilization_Colonies_,
  Build_Pop_Array_, Make_Bldg_Array_For_Colony_, Build_Captured_Colonists_Array_,
  Do_Attacker_Beat_Colony_Stuff_) can be reached from this group's routines.
- Network-only branches: D3 st.ignore_combat clear (game_type 2/3), Russ_Event_Monster_Update_ →
  Mox_Send_Message_.

### Porting order for this group and what each step should reproduce

1. D13 Allocate_New_Ship_Slots_ + D16 Allocate_New_Colony_Slots_ (no RNG, pure):
   these should reproduce all 342 s.Owner changes on free slots, c.ReservedFor and
   st.ColonizePlayer. That is a cheap first proof of the turn order.
2. D2, D3 (+ arrival), D17, D15: ship positions, ETA, status, AI-mission resets.
3. D9/D12 wrapper counters (p.FleetSizeScore etc.) on top of the ported economy.
4. D5 + D6 (needs RNG alignment from the AI phase onward): BC, research, production,
   growth, assimilation, new ships (Design bytes).
5. D4 spies, D7 surrenders (colony 54), D8, D10, D11, D14.

### Open questions

- star 31 surrender_to[2] = 9 before the turn. Do_Surrender_Star_ passes 9 as the new owner
  and Do_Surrender_Colony_ stores `c.owner = 9` (byte), yet after.GAM has c.owner = 1 and
  st.colony_owners = 2. Something later (combat? Check_Rebellion_? a mask elsewhere)
  must turn 9 into 1, or the value written by Surrender_Star_ (0xe4ab3, diplomacy) uses
  another encoding. Trace Surrender_Star_'s callers (0x198f2, 0x19d2f, 0x1af96, 0x1b554)
  for the value they pass.
- Restock_Antaran_Ship_'s counters at 0x19919e and Allocate_New_Ship_Slots_' counters
  0x19918c..: in the save (antarans block) or BSS?
- Event_Orion_Discovered_ globals 0x19acc4/5: saved (event_data?) or not.
- Player byte +0x59d (Kill_Research_Buildings_ trigger) — which event sets it.

## Phase E: Combat

`Search_For_Battles_` (0xe9d62, COMBFIND.C) runs in `Next_Turn_Calc_` after
`Deallocate_AI_Data_` and before `Determine_Event_`. Its raw reach is ~1840
routines because it contains tactical combat and every popup; the game logic
below it is ~27k instructions over ~325 routines (sizes below are own
instruction counts; "logic" totals exclude engine modules, popups, screens,
flics, network and the already ported colony recalculation).

Legend: RNG `R!` calls `Random_` directly, `R` below, `-` none; `GO` = name
already appears in `internal/` (ported or at least referenced); MP = network
only (game_type 2/3); UI = player-facing only.

Settings bytes (base 0x199bdc): **+0xd8 (0x199cb4) = 1 means strategic
combat** (`Russ_Combat_`, `Bombardment_`, `Strategic_Combat_` unload;
`Newgame_Screen_` passes tactical = (+0xd8 == 0)); `save.Settings.StrategicCombat`. +0xd4 (0x199cb0) is compared `> 2` before AI surrender
(difficulty, presumably).

### Call order

| d | Routine | Addr | Module | Purpose | Save state | RNG | GO | Insn (own / logic subtree) |
|---|---|---|---|---|---|---|---|---|
| 0 | `Search_For_Battles_` | 0xe9d62 | COMBFIND | whole combat phase | see below | R | | 395 / 26.9k |
| 1 | `Get_Auto_Function_`, `Assign_Auto_Function_(Draw_Mini_Main_Screen_)` (game_type < 2) | | | UI | – | - | GO | |
| 1 | `Update_Ship_Icons_In_Between_Turns_` | 0x8a4c4 | MAINSCR | star owners + ship icons (called after nearly every kill below) | star `owner` (0x14) | - | partly (Star_Owner_, Build_Ship_Icons_) | 22 |
| 1 | build tables (stack, BSS-free) | | | `_star_has_player[star·0x58 + p·0xb]` (11 B: +0..4 ship type present, +5 transports, +6 any ship at star, +7 colony count, +8 s16 transports arriving in 1 turn, +10 transports in transit); `_star_has_nonplayer[star]` monster-owner bit mask (owners 8..14, bit `owner−8`); `_player_needs_to_attack[star]` u16 owner mask. Colonies from the end (`NUM_COLONIES−1..0`), ships 0..`NUM_SHIPS−1` (status 0, location < 0x48; status 1 type-2 transports bound for 500+star). A player ship sets needs-to-attack only if `type == 0` (combat); every monster ship sets it. `_star_has_nonplayer[NUM_STARS] \|= 1` = Antares as pseudo-star `NUM_STARS` | reads colony owner/planet, ship type/owner/status/location/eta | - | | |
| 1 | `Event_Check_Space_Anomaly_At_Star_` per star 0..`NUM_STARS+1` | 0x242cf | EVENTS | star with an active space anomaly (event 25 state 2/4/6) is skipped | event_data | - | GO | 12 |
| 1 | `Shuffle_Char_(_battle_stars, n)` | 0xfe9b5 | RUSS | Fisher-Yates over the battle-star list: **n calls `Random(n−i)`** | – | R! | | 31 |
| 1 | loop (game_type < 2) while `_n_battle_stars > 0`: | | | | | | | |
| 2 | `Pick_Star_And_Attacker_` | 0xe84a5 | COMBFIND | take battle stars from the end of the list; drops stars with no attacker left (swap with last) | – | R | | 71 |
| 3 | `Some_Star_Player_Is_Busy_` | 0xe841a | COMBFIND | MP only: skip a star where a human is busy | – | - | | 51 |
| 3 | `Pick_Attacker_At_Star_` | 0xe8194 | COMBFIND | reservoir pick: if any monster bit (mask ≥ 0x100) pick among owners 8..14, else among players 0..`NUM_PLAYERS−1`; one `Random(k)` per candidate; clears the picked bit | – | R! | | 59 |
| 3 | `Collect_Targets_For_Combat_` | 0xe82b4 | COMBFIND | targets: colonies at orbits 4..0 (`_target`, count 0x1aa38a) and ship owners `NUM_PLAYERS−1..0` with ships (+6) and no colony (+7 == 0) there, then monsters 14..8 (0x1aa38c, count 0x1aa3aa); none → attacker −1 and try another attacker | – | R | | 100 |
| 4 | `Player_May_Attack_Player_` | 0xe8231 | COMBFIND | AI attacker: war status (+0x627+t 4..6) yes; status 0 and no trade/research treaty (+0x62f/+0x637, +0x80e) → personality (+0x27) 0 yes, 3 → `Random(10) == 1`, else no; humans and monsters always may | player treaty bytes | R! | | 45 |
| 2 | attacker human (`player+0x28 == 100`) and `star+0x36` (`ignore_combat`) bit clear → `Get_Local_Human_Target_` | 0xe7cdb | COMBFIND | **UI**: sets `something_interesting_happened[p]`, `star+0x33` visited bit, **`PLAYER_NUM = p`**, `Defense_Colony_Selection_Popup_` + `Human_Confirms_Attack_` loop. Declined → `star+0x36 \|= 1<<p` | `something_interesting_happened`, star visited/ignore_combat, PLAYER_NUM | R (popup plasma box, seed restored) | | 69 (UI 51k) |
| 2 | attacker AI (< 8) → `Get_AI_Target_` | 0xe7dca | COMBFIND | strength compare: `Get_Ship_Strength_Info_` (AIPOWER `Ship_Strength_Vs_Ships_`, `_Vs_Planetary_Shield_`) and `Get_Best_Colony_Target_` (`Colony_Space/Ground_Strength_Vs_Player_`, `Enemy_Colony_Worth_To_Player_`, `Colony_Planetary_Shield_`); a pending war target (+0x7c7 planet at this star) is consumed (+0x7c7 = −1, +0x7ca = 0xff, +0x816 = 0) and flags a sneak attack; equal strength with personality 2..3 → `Random(2)` | player +0x7c7/+0x7ca/+0x816 | R! | | 172 / 3.6k |
| 2 | attacker monster (≥ 8) → `Get_NPC_Target_` | 0xe8029 | COMBFIND | Eel (13) never attacks; colonies preferred (non-"lucky" +0x8b9 owners first), else ship owners; reservoir `Random(k)` per candidate in two pools | – | R! | | 112 |
| 2 | colony target → `player[colony owner]+0x204 = attacker` | | | combat slot | player +0x204 | | | |
| 2 | Dragon (12) at a human colony → `Dragon_New_Star_Popup_` (UI) → bribe: `Do_Dragon_Bribe_` 0xe9cd3 (`Ships_Try_To_Move_To_`/`Make_Ships_Move_To_` GO) and no fight | | | dragon moves | ship move fields | R (popup, restored) | partly | 44 |
| 2 | `star+0x36 = 0`; `Do_1_Combat_(star, att, def, planet, 0, sneak)` | 0xe938c | COMBFIND | one battle, below | | R | | 335 / 22k |
| 2 | `Update_Player_Needs_To_Attack_` | 0xe6aa2 | COMBFIND | clear both sides' bits if they have no ships left at the star | – | - | | 57 |
| 1 | else game_type 2/3: `Client_Combat_Loop_` 0xea2f1 + `Mox_Send_Message_` | | | **MP** | | R | | 312 |
| 1 | `Process_Retreating_Ships_` | 0xe6e52 | COMBFIND | ships with status 9 (retreat): group per owner/star, `Closest_Other_Colony_Star_` (GO) → `Ships_Try_To_Move_To_`/`Make_Ships_Move_To_` (GO), status 1, location −500; no reachable star → kill (officer: ETA ≤ 0 `Kill_Officer_`, else `Move_Officer_To_Pool_`). Ships at location `NUM_STARS` (fled Antares): per owner pick a random own-colony star (`Random(k)` over `star+0x38` bits) or 'H' = none → kill. Monsters ≥ 8 status 0 with drive: Antarans `Restock_Antaran_Ship_`+kill; Amoeba/… 10 die, 13/14 `Random(2)`; `Russ_Event_Monster_Update_`. Finally `Add_Msg_` per (player, star) with losses | ship status/location/x/y/eta, leaders, msgs, antarans | R! | partly | 315 / 2.0k |
| 1 | `Set_Reload_Fleet_Box_Palette_Flag_` if `_messed_up_fleet_palette` | | | UI | | - | | |
| 1 | `Do_System_Discoveries_At_Star_` stars `NUM_STARS−1..0` | 0xe9927 | COMBFIND | specials picked up by ships, below | star special, BC, colonies, techs, msgs | R! | | 273 / 12k |
| 1 | `Put_Auto_Function_` | | | UI | | | GO | |
| 1 | `Check_For_Eliminated_Players_` | 0xe4eb3 | COLCALC | players without a populated colony (owner < 8, +6 == 0 … see note) → `Eliminate_Player_` 0xe45ff (`Destroy_Colony_`, `Kill_Ship_` all, `Set_Their_Spy_Number_`, `Bring_Home_Spies_Vs_`) + `Event_Race_Eliminated_`; every live player: +0x204 = 0xff | player +0x24, +0x204, colonies, ships, spies, event 29 | - | | 45 / 1.0k |

`star+0x28` (`special`) markers: **this is the writer of the marker that
`Explored_New_Star_` reports** (open question in reports-and-begin-of-turn.md):
`p+50` pirate cache/debris BC (special 2, +50 BC), `p+60` (special 3, +100 BC),
`p+70` splinter colony joined, `p+80` marooned hero, `p+90` ancient artifacts.

### Do_1_Combat_ (0xe938c)

| d | Routine | Addr | Module | Purpose | Save state | RNG | GO | Insn |
|---|---|---|---|---|---|---|---|---|
| 1 | defender from colony if −1; both < 8 → `player[att]+0x717+def = 0` (turns-at-war counter reset) | | | | player +0x717 | | | |
| 1 | MP: "%s attacks you at %s" `Message_Box_` | | | MP/UI | | | | |
| 1 | `Get_Combat_Ships_` | 0xe6a0c | COMBFIND | ships 0..`NUM_SHIPS−1` at the star with status 0: attacker list ascending, defender list filled backwards from slot 500 | – | - | | 49 |
| 1 | `Russ_Combat_` | 0xe7343 | COMBFIND | chooses the resolver; returns 1 attacker won, 0 draw, −1 lost | | R | | 225 |
| 2 | no attacker ships, or no defender ships and no colony (and not Antares) → 0 | | | | | | | |
| 2 | `something_interesting_happened[att/def] = 1` (< 8) | | | | save | | | |
| 2 | single-player and human defender: `Show_Sneak_Attack_Message_` (if sneak), `Player_Attacked_Popup_` | | | UI | | - | | 150 |

`Show_Sneak_Attack_Message_` (0x1afa0) is `mov edx, 1` falling into
`Npc_Diplomacy_Screen_` (the delayed message, +0x7c9). The sneak flag is
`Get_AI_Target_` meeting the AI's sneak planet (+0x7c7) with +0x7ca the
victim; the turn keeps +0x7c7 only while +0x816 ≠ 0 (`Npc_Diplomacy_`)
and the AI is at war with nobody (`Launch_Sneak_Attack_`), which declares
the war itself once its fleet is within one turn (enroute strength, ETA ≤ 1),
so the fleet must arrive in the same turn. sneak1
(`build/turns/sneak1.sh`; s2 with AI 2's plan on Trilar II and three of its
ships in transit there with ETA 1 and AI mission 6, so the move routines
leave them alone): the original shows the ambassador ("You have already
shown yourself to have no honor ... We are now at war!"), then the
attacked popup. `TestTurnSneak1Attack` matches the message text, the
popup, every step save and draw up to it.
| 2 | **strategic** if set+0xd8 ≠ 0 **or neither side is human** → `Strategic_Combat_` (logic, below); human involved → `Message_Box_Titled_` with the result text (UI) | 0x40148 | COMBAT | auto-resolve | | R | | 694 / 12.1k |
| 2 | **tactical** otherwise (a human side, set+0xd8 == 0): `AI_Can_Retreat_` 0xe7303 (AI side has another colony) → `Tactical_Combat_` 0x47939 COMBINIT (UI entry; 1232 own, ~83k non-engine below, ~1440 routines; consumes RNG for real decisions; MP both-human variant syncs seeds, `Do_Combat_Turn_` 0x43502 `Set_Random_Seed_` from the network combat message — MP only) | | | **UI, stop** | ships, colony, leaders | R | | |
| 1 | MP: `Load_Net_Next_Turn_Screen_`, `Chat_Box_Input_Loop_` | | | MP | | | | |
| 1 | result ≥ 0 (defender did not win): defender < 8 with no colony there and no planet → `Kill_Noncombat_Ships_` 0xe6b44 (transports/colony ships die; human gets `Message_Box_`); else `Determine_Retreat_Ships_` 0xe6caa (defender ships → status 9, or all killed under hyperspace flux unless trans-dimensional +0x8bc); monster defender: clear its nonplayer bit | | | | ship status, leaders | - | | 40 / 121 |
| 1 | result ≤ 0: attacker retreats (`Determine_Retreat_Ships_`); attacker was Guardian (9) → defender gets Orion: player +0x1ef = 1, `Give_Player_Loknar_Special_` 0x980db (`Assign_Loknar_Ship_To_Player_`, techs, `Get_Random_Antaran_Tech_` R), human: `Play_Take_Orion_Flic_`, `Rescued_Loknar_Popup_` (UI) | | | | player +0x1ef, leaders, ships, techs | R | | 67 |
| 1 | result < 0: recount defender ships; none → `Reset_Combat_Ships_At_Current_Star_Has_Player_` 0xe9358 | | | | – | - | | 19 |
| 1 | result > 0: defender Guardian → attacker gets Orion (as above); star == `NUM_STARS` → player +0x1f0 = 1 (Antares captured) and clear all needs-to-attack; else recount, and with a live target colony → `Do_Attacker_Beat_Colony_Stuff_` | | | | player +0x1f0 | | | |

### Strategic_Combat_ (0x40148, COMBAT.C) — the AI-vs-AI resolver

All AI-vs-AI and monster battles go here. ~12.1k logic instructions.

| d | Routine | Addr | Purpose | Save state | RNG | GO | Insn |
|---|---|---|---|---|---|---|---|
| 1 | `_qcombat_data` = 0x21-byte records in screen_seg (0x4075 B); `_combat_attacker/_defender_player`; Antares (star `NUM_STARS`) → defender 8 | | | – | | | |
| 1 | Guardian (9) vs monster, or monster vs Guardian: losing side's ships `Kill_Ship_` (no fight) | 0xa163a | | ships | - | | 49 |
| 1 | `Get_Leader_Combat_Bonuses_` 0x40034 (helmsman, ordnance, engineer, weaponry, galactic lore via `Owned_Officer_Level_`) per side | | | – | - | partly | 78 |
| 1 | `Qload_Ships_` 0x416cf: per ship best weapons/shield/armor/drive/computer (`Best_*_` GO, `Base_Generic_Ocv_`, `Update_Strategic_Ship_` GO, `Get_Colony_Hits_`); slot 0 = colony defence | | | – | - | partly | 433 |
| 1 | defender < 8 with Artemis net (`Player_Has_Artemis_Net_`) → `Apply_Combat_Artemis_Net_` 0x3868f (`Apply_Ship_Damage_`, `Select_Internal_`, `Apply_Internal_Damage_`, `Apply_Damage_To_Planet_` → `Apply_Bomb_Hits_`) — may end the battle | | | colony | R! | | 118 / 9.6k |
| 1 | Antares: `QLoad_Antaran_Defense_Fleet_` 0x3fef2; else defender `Get_Leader_Combat_Bonuses_` + `Qload_Ships_` | | | antarans | - | | 72 |
| 1 | rounds while both sides alive and turn < 50: per ship `QGet_Target_` 0x41e88 (`Random(n)` over live enemies; colony slot 0 when none), slot 0 with flag 4 → `QGet_Target_SC_` + 4×`Apply_Damage_To_Qship_`; turn < 4 or size > 5 or colony → `Beam_Attack_` (weapon class 8) or `Missile_Attack_`; turn > 2 → `Strat_Special_Attack_`, `Beam_Attack_`; turn > 3 attacker vs live colony → `Strat_Bomb_Attack_`; then `Resolve_Strat_Colony_Damage_` (`Remove_Building_` 0x145ea ERICNET R); dead flag 1 → 2 | 0x41e88.. | per shot `Random(100)` to hit vs `0x28 − (att − def)` capped 0x5f, `Random(max−min+1)` damage (Beam_Attack_ 0x4221f, Missile 0x41f80, Special 0x4215c, Bomb 0x420c0) | colony buildings | R! | | 43..96 each |
| 1 | result: turn < 50 → attackers − defenders; else 0 / −1 | | | | | | |
| 1 | per ship: building ids 0x28/8/0x29 of destroyed colony defences removed (`Remove_Building_`); structure damage written back to ship +0x7d (set+0xd8 == 1: raw; else scaled by `Get_Ship_Structure_` GO) unless the side has a repair bonus (cybernetic +0x8b0, +0x11b == 3) | | | ship +0x7d | | | |
| 1 | dead ships' officers: ETA ≥ 1 → `Move_Officer_To_Pool_` 0x944a3 (calls `Do_Player_Colony_Calculations_` GO), else `Kill_Officer_` 0x941c6; race +0x1ca == 3 keeps up to 8 officers to re-seat on surviving ships | | | leaders, ship +0x74 | - | | 50 / 44 |
| 1 | both players: colony fight `Russ_Change_Relations_(−10000, att, def, 10, star, 1)`, then `(−15 × kills, …, 0xb, star, 0)` → `Change_Relations_` 0x4e3b5 DIPLOMAC (`Reward_Check_` R!; MP: message) | 0xf975c | | player relations | R | | 34 / 406 |
| 1 | kills: Antaran defence/offence ships `Kill_Antaran_Defensive_Ship_`/`_Offensive_Ship_` (ANTARANS), `Kill_Ship_` | | | ships, antarans | - | | |
| 1 | monster beaten (owner ≥ 10 with drive) → `Russ_Event_Monster_Update_` → `Event_Monster_Update_` 0x2332c | | | event_data | R (MP send only) | | 24 |
| 1 | `Build_End_Of_Combat_Message_` 0x40d40 (text only, 1067 insn) | | | UI text | - | | |
| 1 | colony left with 0 pop and no buildings → `Destroy_Colony_(c, 1)` 0xdcdac BOMB (`Kill_All_Events_At_Colony_`, `Handle_Leader_At_Lost_Colony_`, `Add_Msg_`, `Assign_New_Home_Planet_`, `Clear_Colony_Queue_`, `Compute_Star_Colony_Stuff_` GO) | | | colony, planet, msgs, leaders | - | | 88 / 680 |

### Do_Attacker_Beat_Colony_Stuff_ (0xe87d2) — bombing, invasion, surrender

783 own, ~17.5k logic. Branches:

| Branch | Routines (call order) | Save state | RNG |
|---|---|---|---|
| colony pop 0 (outpost) | `Destroy_Colony_(c,1)` | colony, msgs | - |
| monster attacker (≥ 8) | `Russ_Event_Monster_Update_` (10..14); Amoeba (10) fixed bomb 20; `Bombardment_` 0x4267b → `Apply_Bomb_Hits_` 0xdd2f2 (`Resolve_Bomb_Hit_` 0xdcebd R!, `Resolve_Biological_Hit_` 0xdd13e R!); human victim: `Play_Amoeba_Flic_` + planet +8/+0xb cleared, `Colony_Environmental_Stuff_` GO, `Enforce_Population_Limits_At_Colony_` 0xec97c (`Shuffle_` R!), `Colony_Calculation_`; `Colony_Bombing_Screen_` (UI); pop gone → `Destroy_Colony_` | colony, planet | R |
| AI attacker | telepathic (`Player_Can_Mind_Control_Colony_`) → `Change_Colony_Ownership_` at once. Else `Has_Stellar_Converter_`, `Bombardment_`; if bombs/bio > 0 or converter: AI defender with difficulty > 2 and `May_Surrender_Star_` 0xe4a09 and bombs ≥ 2·pop → **`star+0x67+def = att+8`** and `Declare_Peace_` 0x524fb (the surrender is executed next turn by `Do_Surrenders_`); else attacker owns pop there → loop `AI_Invasion_Will_Succeed_` 0xcf26e (simulated `Resolve_Ground_Combat_`, R) / `Resolve_Bomb_Hit_` / `Resolve_Biological_Hit_` until invadable; few enemy stars in range → same loop; else `Apply_Bomb_Hits_` or stellar converter `Destroy_Colony_(c,2)`; `Apply_Bio_Weapon_Penalty_` 0x4b11e (R!, relations); human victim `Colony_Bombing_Screen_` (UI); `Russ_Change_Relations_(−10000,…,10,…)`; pop gone → `Destroy_Colony_`. Then `AI_Player_Invades_` 0xcf34c (troops > 0) → `Invade_` 0xed48b; human victim `Colony_Combat_Screen_` (UI, seed-neutral, see below); relations; star colony counts | colony owner/pop/buildings/marines (+0x130/+0x132), star surrender_to (+0x67), relations, ships (transports killed) | R |
| human attacker (single player, or MP local) | `Bombardment_` preview, `Has_Stellar_Converter_`, `Get_Invasion_Info_` 0xec831, `Colony_Selection_Popup_` (UI) loop: 1 destroy (converter): `Destroy_Colony_(c,2)`, `Play_Planetary_Destruction_Flic_`, planet +4 = 1, +0xf/star special cleared, relations; 2 bomb: `Bombardment_`, `Colony_Bombing_Popup_` (UI), relations, maybe destroy; 3 invade: `Invade_`, `Force_Fade_To_Colony_Palette_`, `Colony_Combat_Screen_`, `Show_Off_Captured_Tech_` (UI), relations; 4 mind control: `Change_Colony_Ownership_` + captured tech; then `Apply_Bio_Weapon_Penalty_`; MP chat | as AI branch | R |
| Antaran attacker (8) | after the above: each attacking ship `Restock_Antaran_Ship_` + `Kill_Ship_`; clear bit 0 of nonplayer | antarans, ships | - |

`Bombardment_`: collects the attacker's status-0 combat ships at the star
(owner = attacker, type 0), then **set+0xd8 == 1 → `Strategic_Bombardment_`
0x4257e** (`Qload_Ships_`, `Missile_Attack_`, `Beam_Attack_`, `Strat_Bomb_Attack_`; R!)
**else `Tactical_Bombardment_` 0x42781** (`Load_Combat_Ship_`, `Load_Tactical_Colony_`,
`Init_Special_Devices_`, `Fire_Ship_` — the tactical engine, ~40k non-engine
below, also for AI vs AI). `Invade_`: `Get_Invasion_Info_`, stores
`Get_Random_Seed()` in the result (+0x6e), `Ground_Combat_Round_` 0xec4fe
(2×`Random(100)` per round) until one side breaks; win →
`Change_Colony_Ownership_` 0xecf41 (`Change_Pop_Ownership_` 0xecbf7,
`Remove_Building_`, captured tech apps with `Random`/`Get_Weighted_Choice_Short_`,
`Player_Gets_Tech_App_` GO) + `Resolve_Invasion_Troops_` 0xece05; loss → colony
marines/armor (+0x130/+0x132) set from the survivors, attacker transports killed.

### Bombing (BOMB.C) — Go: yes (turn_bombing.go)

The bomb-hit record, 0x49 bytes (`bombHits`), filled by `Bombardment_` and the
event allocators, cleared by `Setup_To_Apply_Bomb_Hits_` 0xca81a (+8..+0x42):

| Off | Size | Meaning |
|---|---|---|
| +0x00 | s8 | attacker; −1 for an event (then `WarLosses2[-1]` is the owner's `WarLosses[7]`) |
| +0x01 | u8 | colony |
| +0x02 | u8 | 1 (set by Bombardment_ and the events; read by Do_Attacker_Beat_Colony_Stuff_) |
| +0x03 | s16 | bomb hits |
| +0x05 | u8 | 1 for the industrial accident, 0 otherwise |
| +0x06 | s16 | biological hits |
| +0x08/+0x0a/+0x0c | s16 | marines, tanks, colonists killed |
| +0x0e | 49 × u8 | 1 by building id destroyed |
| +0x3f | s16 | colonists killed by biological hits |
| +0x41 | u8 | something was hit |
| +0x42 | u8 | the last colonist died (and, for bio, no building is left) |
| +0x43 | s16 | colony production (+0x125) destroyed |
| +0x45 | s32 | bomb hits spent (sum of Resolve_Bomb_Hit_ returns) |

- `Apply_Bomb_Hits_` 0xdd2f2: local copies of +3/+6; while their sum > 0:
  a biological hit first if any (`Resolve_Biological_Hit_`, bio −1), else
  `Resolve_Bomb_Hit_(colony, rec, hits)`: 0 ends the loop, n > 0 → hits −= n, +0x45 += n.
- `Resolve_Bomb_Hit_` 0xdcebd: targets = buildings 0x30..0 except 8, 9, 0x1a,
  0x1b, 0x28, 0x29, 0x2a, 0x2f (listed from the top) + marines + tanks + 1 if
  production ≠ 0, + colonists unless pop == 1. Pop 1 with no other target:
  `PopFraction[race of colonist 0]` > 100 → −100, else the colony empties
  (capitol removed, +0x42, owner `WarLosses2[att]` += 10); returns 1. Else
  k = Random(targets) − 1: building → `Remove_Building_` and +0x0e flag; marine
  (cost 1, 2 with app 0x90, +1 High-G pick) and tank (cost 2, 3 with app 0x18,
  +1 High-G) only if cost ≤ hits left, returning the cost (else 0); then the
  production slot — **compared against the production amount, not 1**, and the
  colonist index is not reduced by it; colonist k replaced by the last, owner
  `WarLosses2[att]` += 1. Every hit sets +0x41.
- `Resolve_Biological_Hit_` 0xdd13e: reservoir pick over non-android colonists
  from the last; none → nothing. Pop ≥ 2: `Random(pop+marines+tanks) ≤ pop`
  kills the colonist (+0x0c, +0x3f), else marines (if any, `Random(m+t) ≤ m`)
  or a tank (even at 0 tanks: −1). Pop 1: marines as before, else a tank if
  any, else the last colonist (fraction of colonist 0's race > 100 → the
  victim's race fraction −100; else pop 0, +0x42 and +10 losses only when no
  building is left).
- Event allocators (record: attacker −1, +2 = 1): `Allocate_Comet_Hit_`
  0x23780 hits = (Random(3)+Random(3))·(buildings+pop)/10, at least 1;
  `Allocate_Earthquake_Hit_` 0x238a8 the same with Random(3)+Random(2), returns
  colonists killed and buildings destroyed; `Allocate_Industrial_Accident_Hit_`
  0x23833 one bomb hit, +5 = 1, bio hits = (Random(3)+Random(3))·pop/10.

### Shared helpers (once)

| Routine | Addr | Module | Purpose | Save state | RNG | GO | Insn |
|---|---|---|---|---|---|---|---|
| `Kill_Ship_` | 0xa163a | SHIPS | free a ship slot (officer to pool) | ship status/owner…, leaders | - | | 49 |
| `Move_Officer_To_Pool_` / `Kill_Officer_` | 0x944a3 / 0x941c6 | OFFICER | leader back to pool / dead (−2); `Do_Player_Colony_Calculations_` | leaders, player totals | - | | 50 / 44 |
| `Destroy_Colony_` | 0xdcdac | BOMB | remove colony | colony, planet, star colony stuff, msgs, leaders | - | | 88 |
| `Remove_Building_` | 0x145ea | ERICNET | remove building, fix queue, recalc | colony, ships ETA | R (`Enforce_Population_Limits_At_Colony_` → `Shuffle_`) | | 135 |
| `Russ_Change_Relations_` → `Change_Relations_` → `Reward_Check_` | 0xf975c / 0x4e3b5 / 0x4ea03 | NETMOX / DIPLOMAC | relation delta (MP: message) | player relations | R! (Reward_Check_) | | 34 / 406 / 72 |
| `Add_Msg_` | 0xef629 | MSG | append message | msgs, n_msgs | - | | 55 |
| `Get_Weighted_Choice_Char_` | 0xfe8da | RUSS | one `Random(sum)` | – | R! | | 42 |
| `Update_Ship_Icons_In_Between_Turns_` | 0x8a4c4 | MAINSCR | star owners + icon cache | star owner | - | partly | 22 |

### RNG accounting

Consumed every turn even when **no battle happens**:
- `Shuffle_Char_`: one `Random` per battle star. A battle star is any star
  (or Antares) where some player has an armed (type 0) ship at the star or any
  monster ship sits — a lone fleet counts.
- `Pick_Attacker_At_Star_`: `Random(k)` for each candidate bit, repeated each
  time an attacker turns out to have no targets; `Player_May_Attack_Player_`
  adds `Random(10)` for personality-3 AIs at peace with the target.
- `Do_System_Discoveries_At_Star_`: for every star (from the last) with
  special ≠ 0, no monster there and ≥ 1 player ship there: first
  `Reset_One_Shot_System_Special_` (0xe98f2, body in patch code 0xf4f74:
  markers 50..89 → clear planet +0xf of the orbits and special = 0; ≥ 90 →
  special = 0), then one `Get_Weighted_Choice_Char_` (one `Random`) even for
  specials that do nothing (4, 5, 6, 9, 11); special 7 adds `Random(2)` per
  colonist; 10 adds `Random(4)` + per-tech reservoir `Random(n)`.

Seed save/restore (checked all `Get_/Set_Random_Seed_` sites reachable from here):
- **Neutral** (save, use, restore): `Draw_Exploding_Plasma_Box_` 0x83efd (every
  popup's opening animation), `Beam_SFX_` 0xa8197, `Draw_Fighter_` 0x3dbd1
  (reseeds from `clock_()` then restores), `Draw_Fighter_Beam_` 0xa9033,
  `Draw_Colony_Combat_Screen_` 0xb7289, `Make_Bldg_Array_For_Colony_` 0xbc30b,
  `Bomb_Results_Popups_` 0xe85f7 (MP), `Jumble_Bitmap_`, `Create_Plasma_Line_`,
  `Bolt_Segment_` (engine). So UI animation does not perturb game RNG.
- **Replay**: `Invade_` records the seed before its rounds;
  `Colony_Combat_Screen_` 0xb7491 replays from it and sets the seed back to
  its parameter at the end; `Do_Attacker_Beat_Colony_Stuff_` brackets the
  screen with its own Get (0xe8c55/0xe9258) / Set (0xe8c85/0xe928c) →
  neutral overall.
- **Consuming, UI-dependent (human attacker only)**: `Colony_Selection_Popup_`
  / `Colony_Combat_Popup_` 0xcbad0 set the seed to the one passed in
  (`Get_Random_Seed()` at 0xe8a7e/0xe8b5a) and, in invade-preview state 2,
  run real `Ground_Combat_Round_`s without restoring. The RNG after the popup
  depends on what the player viewed — must be ported exactly as the popup does.
- **MP only**: `Do_Combat_Turn_` 0x43502 sets the seed from the network
  combat message buffer (0x190464, filled from `Send_Tactical_Combat_Message_`'s
  `Get_Random_Seed()`); `Client_Combat_Loop_`.
- Tactical combat (`Tactical_Combat_`) consumes RNG for its game logic and is
  driven by player input — human battles with set+0xd8 == 0 cannot be checked
  against a save without replaying the same inputs.

### Test pair (build/turns/t1)

No battle plausibly happened:
- No combat-style relation penalty: the drops are 2↔7 −50 together with
  treaty 0→4, which is a war declaration (diplomacy), and 0↔5 −7.
- No retreat or combat messages, no `ignore_combat` bit changes, and no monster
  ship losses.
- The only lost active ship is a player-3 Transport at star 27, which is the
  player's own colony 47 (marines 2→5): troops landed at home, not an invasion.
- Colony 54 / star 31 went from player 2 to player 1, but `before.GAM` already
  had `star[31].surrender_to[2] = 9` (= player 1 + 8). That flag was written
  last turn by this branch's `May_Surrender_Star_` path, and the transfer
  itself is `Do_Surrenders_` (COLCALC phase), not combat.
- The hundreds of ship owner/design diffs are not kills: they are the free
  slot reservations of `Allocate_New_Ship_Slots_` (section D13).

Combat still matters for the pair: it consumed RNG through `Shuffle_Char_`,
`Pick_Attacker_At_Star_`, `Player_May_Attack_Player_` and
`Do_System_Discoveries_At_Star_`, so it must be ported, at least the no-battle
path, before the random seed of after.GAM can match.

### Ported — Go: yes (turn_combat.go, turn_combattarget.go, turn_combatdo.go, turn_combatcolony.go, turn_stratcombat.go, turn_tactical.go, turn_tacfire.go)

`Search_For_Battles_` is step 24 in `turnSteps()`. Byte-exact with every draw
on all traced turns (none of the user's has a battle) and on the synthetic
turns c1–c7 (`build/turns/inputs/`, made from t1; c5 and
c7 traced with Esc presses for the message boxes):

| Turn | Setting | What happens in step 24 |
|---|---|---|
| c1 | tactical | two AIs at war attack each other's colonies: `Strategic_Combat_` against colony defences, `Tactical_Bombardment_`, `Apply_Bomb_Hits_` |
| c2 | tactical | a telepath's fleet held off by a battle station (50-round draw) |
| c3 | strategic | the AI fleet leaves before the search (no battle) |
| c4 | tactical | a monster loses at an AI colony; an AI attack is repulsed (retreat); an AI beats a colony and bombs it |
| c5 | strategic | human victims: a monster bombs a human colony (`Strategic_Bombardment_`), an AI bombs and invades one (ground combat, captured techs; `Colony_Combat_Screen_` replays and restores the seed) |
| c6 | tactical | fleets arriving by transit fight on arrival |
| c7 | tactical | a monster destroys unarmed transports (`Kill_Noncombat_Ships_`) and more AI bombing |

The step harness follows the seed chain (`netDraws` in `turn_test.go`): a
seed set that returns to an earlier draw's seed continues from that draw,
so screens that animate and restore the seed drop out of the comparison.

Notes from the port:

- The table pass skips ships whose location would index outside the
  73-star tables (negative, or transports in transit to 1000+star); the
  original would write into its stack there.
- `Update_Player_Needs_To_Attack_` shifts the monster mask by the full owner
  number (≥ 8), so a monster's needs bit is always cleared after a fight.
- `Enemy_Colony_Worth_To_Player_` reads the AIDATA.C statics (0x1a8b34),
  which outlive `Deallocate_AI_Data_`; the port keeps them in `Game.aiKept`.
- `_qcombat_data` and the tactical blocks live in screen_seg: the port keeps
  them in `Game.qc` / `Game.tc` across battles. `Strategic_Bombardment_`
  does not clear the records, so slot 0's shield (+9) is whatever the last
  `Strategic_Combat_` left (0 after a clear); a fresh port starts at zero.
- The discovery message (special 10) takes its priority byte and bytes
  5..17 from stale stack (marked for the tests).
- `Do_Dragon_Bribe_` gets the popup's chosen star as its destination.
- `Get_Combat_Ships_` fills the defender list backwards, so defenders are in
  descending ship order.

Without a screen (the step harness) the screens decide as follows, logged:
`Tactical_Combat_` counts as a draw, the target, confirmation and dragon
popups decline, the human attacker's colony popup leaves the colony alone.
Not ported: ship damage inside the tactical engine (unreachable from a
bombardment), network play.

`Give_Player_Loknar_Special_` (0x980db, loknar.go): `Assign_Loknar_Ship_
To_Player_` turns the first Guardian ship (owner 9) into the player's
"Avenger" (`Load_Loknar_Ship_Design_` 0x58487; ship record zeroed, owner and
design +0x5d the player, star 0, officer 0x41); when there is none or the
player has 4 fleet officers, Loknar (leader 0x41) gets location −1, owner
−1, status 6 (and ship[k].officer −1; with k = −1 the original writes before
the ship array, skipped in the port), else status 1 at the ship, owner p,
`Give_Leader_Specials_To_Player_`. Then app 0x2f if `Player_May_Get_Tech_
App_`, and `Get_Random_Antaran_Tech_(p, 3)`: the apps of field 0x4a the
player may get, up to 3 by `Random_(n)` with the last moved into the
taken slot; `Update_Ship_Icons_In_Between_Turns_`. `Rescued_Loknar_Popup_`
(human, after the flic): with techs, H 0x25 (Loknar gone) or 0x26, then
per tech 0x179b1d + 0x179b20 + its name + 0x179b23, in `User_Box_` type 0;
full redraw. Traced: lok1 (`build/turns/lok1.sh`, AI 4 beats a nearly
destroyed Guardian in s2), `TestTurnSteps/lok1`; lok2 (`build/turns/lok2.sh`,
the Guardian attacks three human ships at Orion, Auto battle, the flic,
the popup with four techs, the level popup): `TestTurnLok2HumanLoknar`
matches both popups, all 35366 draws and the F10 save.

The AI's star surrender (`May_Surrender_Star_` + `Declare_Peace_`) is traced
by c1 (star 2, colony 65: player 2 surrenders to 4). Untested branches (no
traced turn reaches them): mind control, stellar
converters, biological weapons, fighters, point defence against missiles and
destroyed defence buildings in the tactical engine, Artemis nets, Antares,
the Guardian, outposts, the Dragon, hyperspace-flux losses, system
discoveries other than plain specials, elimination, and every human-attacker
path (hooks).

## Phase F: Events, officers and bookkeeping

Covers the `Next_Turn_Calc_` (0x136b3) calls that are not AI, diplomacy,
economy, movement or combat: the prologue (`Assign_Auto_Function_`,
`Allocate_Msg_Slots_`, `Antaran_Invasion_Check_`), `Initialize_Reports_`, the
event pair `Determine_Event_` / `Do_All_Ships_XP_Check_` / `Event_Twiddle_`,
the officer block `Check_For_Officer_Level_` / `Decrement_Officer_ETA_`, the
tail (`Random_Officer_Check_` ×NUM_PLAYERS, `Add_Ongoing_Event_Msgs_`,
`Check_Release_Version` → `Assert_Marooned_Leaders_`, `Record_History_`,
`stardate++`, `Set_Mouse_List`).

Legend: **RNG** `R!` calls `Random_` directly, `R` only below, `-` none.
**Go**: `yes` ported in `internal/game`, `no` not. **n** = own instructions
(from `build/orion2.dis`), **Σ** = whole reachable subtree (roots only, counted
once per routine, includes UI leaves). Field names: `docs/formats/save-*.md`.
Globals: `settings` 0x199bdc (+2 = 0x199bde random events, +0xd3 = 0x199caf
Antarans attack, +0xd4 = 0x199cb0 difficulty, +0xd9 = 0x199cb5 tech level);
`antarans` 0x199174 (+0x3c = 0x1991b0 skip flag, +0x3d = 0x1991b1 last attack,
+0x41 = 0x1991b5); `event_data` 0x19aba4 (9 B × 36; +1 = 0x19aba5 state);
`bill_savegame` 0x17d634.

### Call order (as in `Next_Turn_Calc_`, positions in the full list)

| # | Routine | Addr | Module | n / Σ | RNG | Go | Purpose | Save state read → written |
|---|---|---|---|---|---|---|---|---|
| 1 | `Assign_Auto_Function` | 0x1191ca | fields.c | 31 | - | yes | `game_type < 2`: auto function = `Null_Autofunction_` (UI) | — |
| 2 | `Allocate_Msg_Slots_` | 0xeeb3a | MSG.C | 64 / 154 | - | no | Clears `first_msg`/`max_msgs`/`n_msgs`, splits the 100 `msgs` slots among living players (player+0x24 == 0) | player.+0x24 → `first_msg`, `max_msgs`, `n_msgs` (= 0). Old `msgs` bytes stay (stale) |
| 3 | `Antaran_Invasion_Check_` | 0x63d92 | ANTARANS.C | 17 / 4104 | R | no | Only if settings+0xd3 ≠ 0 and antarans+0x3c == 0. Antaran resource growth, ship building, attack decision and deployment | `antarans`, `ship`, settings+0xd4/+0xd9 |
| 4 | `Apply_Evolutionary_Upgrades_` … `All_AI_Tech_Select_`, `Make_Scrap_Ships_Dead_` | | | | | | other phases | |
| 17 | `Initialize_Reports_` | 0xfd81c | REPORT.C | 11 / 101 | - | no | Starts the turn's report state | → `something_interesting_happened` = 0, `n_reports` = 0, `done_colonize_check` = 0, `done_player_reports` = 0, `displayed_msgs` = 0, `displayed_gnn` = 0, antarans+0x41 = 0 |
| 18–25 | trade … `Search_For_Battles_` | | | | | | other phases | |
| 26 | `Determine_Event_` | 0x2230a | EVENTS.C | 664 / 12598 | R! | no | Lucky-player events; new random event (chance roll, type pick, victim, target) | settings+2, +0xd3, +0xd4; `last_event_year`, `event_delay_count`, `event_data`, player.+0x24, +0xa6, +0x8b9, +0xe73, +0x32, +0x36, +0x627, +0x8b6, +0x1eb; colonies, stars, ships → `event_data`, `last_event_year`, `event_delay_count`, player.+0xe73; via `Do_Wormhole_Event_` ship position/status |
| 27 | `Do_All_Ships_XP_Check_` | 0x14a27 | ERICNET.C | 34 / 418 | - | yes (`doAllShipsXPCheck`) | +1 crew experience (+ best instructor bonus, + academy) for every player ship slot, recomputes crew level | ship.`status`, `owner`, `location`, leaders, colony.buildings → ship.`experience` +0x72, `crew_level` +0x71 |
| 28 | `Event_Twiddle_` | 0x206a2 | EVENTS.C | 797 / 33462 | R! | yes (`eventTwiddle`) | Advances every active event one turn and applies its effect | `event_data` and, per event type: planets, colonies, players (BC, research), ships, leaders, relations |
| 29–31 | `Lose_Out_Of_Range_Ships_`, `All_Colony_Calculations_` ×2 … | | | | | | other phases | |
| 35 | `Check_For_Officer_Level_` | 0x92fda | OFFICER.C | 26 / 6307 | - | yes (`checkForOfficerLevel`) | +1 experience to every owned leader (status 0 or 1); level change → colony recompute and `level_up` flag | leaders → leaders.`experience`, `level_up`; colonies/player stats via `Star_Colony_Calculation_` |
| 36 | `Decrement_Officer_ETA_` | 0x934cf | OFFICER.C | 34 / 6353 | - | yes (`decrementOfficerETA`) | Offers (status 4) age, expire when the ETA reaches 30; leaders in transit (status 1) count down, arrival recomputes the star | leaders.`eta`, `status`, `location`, `owner`, `experience`; star+0x28, star+0x42; ship.`officer`; colonies/player stats |
| 37–45 | rebellions … `Check_For_Council_Meeting_` | | | | | | other phases | |
| 46 | `Random_Officer_Check_(p)` for p = 0..NUM_PLAYERS−1 | 0x97a66 | OFFICER.C | 36 / 483 | R! | no | Chance of a new officer offer for player p | leaders, player.+0x8b2, +0x8b3, +0xe6a, stardate → player.`officer for hire` +0x5a1, +0xe6a, leader.`owner` |
| 47 | `Add_Ongoing_Event_Msgs_` | 0xef827 | MSG.C | 231 / 1289 | - | yes (`addOngoingEventMsgs`; traced: ev2 has flux, plague, population boom and anomaly, ong1 (`build/turns/inputs/ong1.edit`: ev2 with the comet 5 turns out) the comet; the April message's bytes 1..17 are stale stack, the clock is `Game.Now`; apr1 (`inputs/apr1.edit`: SAVE5 at stardate 35023, the dword at 0x192fd8; `DOSDATE=04/01/1996` for run.sh, which DOSBox-X's DATE takes only with slashes) adds message 0x15: `localtime_`'s tm_mon = 3 and tm_mday = 1) | Turn-summary messages for ongoing hyperspace flux, plague, population boom, comet, space anomaly; April-fools message | `event_data`, colonies, stars → `msgs`, `n_msgs` |
| 48 | `Check_Release_Version` | 0x110b5c | capture.c | 21 | - | no | Returns `release_flag` | — |
| 49 | `Assert_Marooned_Leaders_` | 0x92c87 | OFFICER.C | 44 | - | no | Only when `release_flag` == 0: debug scan of marooned-hero stars; writes nothing (no-op) | — |
| 50 | `Record_History_` | 0x10208a | BILL.C | 223 / 347 | - | yes (turn_history.go) | Samples per player: population +0xa6 → History[2] (divisor bill+4), building costs → [3] (+6), fleet Σ 10·2^(hull+1) of ships with signed status ≤ 1 → [0] (+8), known tech cost +0x224 → [1] (+0xa). Divisors (0 → 1) grow while a sample / divisor > 250 (unsigned); if any grew, **every** player's 350 old bytes are rescaled old·d/d'; write at ring bill+2, ring + 1 mod 350 | colonies (buildings), ships, player.+0xa6, +0x224 → player history 0x8df/0xa3d/0xb9b/0xcf9, `bill_savegame` |
| 51 | `stardate += 1` | 0x13853 area | NEXTTURN.C | | - | — | | `stardate` |
| 52 | `Set_Mouse_List` | 0x123e6c | mouse.c | 23 | - | yes | Only if `_in_auto_next_turn_mode` == 0: arrow cursor (UI) | — |

### Subtrees

```
Antaran_Invasion_Check_ 0x63d92 R            settings+0xd3 && !antarans+0x3c (in Next_Turn_Calc_)
  [stardate == 35000: _offensive_titan_ship_built = 0]   (not saved)
  Increment_Antaran_Resource_Level_ 0x645ec -  every 25 turns after Antaran_Delay_: antarans+2 (offence)
      Antaran_Delay_ 0x63e4c -                  settings+0xd9: 200/100/0 turns
      Antaran_Defensive_Force_At_Maximum_ 0x646bd -  share goes to +2 instead of +4 when defence is full
  Build_Antaran_Offensive_Ships_ 0x63fcb -      Build_Antaran_Ships_(+2, +0x18, _n_max_antaran_off_ships, 1)
  Build_Antaran_Defensive_Ships_ 0x63f9c -      Build_Antaran_Ships_(+4, +6,   _n_max_antaran_def_ships, 0)
      Build_Antaran_Ships_ 0x63ff0 -  (n 252)    buys ships by size while resources ≥ cost; sprintf debug
                                                 log into _temp_string (dead output); first offensive titan on
                                                 difficulty ≥ 3 cuts _antaran_ship_cost to 90% (NOT saved)
          No_Small_Ships_After_N_Turns_ 0x647d7, No_Medium_Ships_After_N_Turns_ 0x6481b -
  Antarans_Should_Attack_ 0x63edd R!            if def+off+res > 4×offence cost → no (no RNG);
                                                 else antarans+0 += 1 if undeployed offensive ships;
                                                 if antarans+0 > 0: Random(200) − 1 < antarans+0 + 1
      Antaran_Defensive_Force_ 0x63dda -, Antaran_Offensive_Force_ 0x63e73 -
  [attack] antarans+0 = 0; Deploy_Antaran_Ships_ 0x643a0 R → true: antarans+0x3d = stardate
      Get_Antaran_Victim_ 0x22f5c R              weights by player.+0xa6 (population) vs the
                                                 smallest, difficulty, lucky (/3)
          Get_Weighted_Choice_ 0x586d4 R!         Random(sum) (halves weights while sum ≥ 512); no draw if sum 0
      Get_Antaran_Target_Star_ 0x645a4 R!        reservoir pick: Random(i+1) for each listed star
          Build_List_Of_Stars_By_Player_ 0x64311 R!  stars with victim colonies, n/2 swaps of 2×Random(n)
              N_Colonies_At_Star_ 0x7826a -
      loop ≤ 6 ships: Next_Antaran_Ship_Size_ 0x6478d -, Load_Antaran_Ship_Design_ 0x5514c -
          first: Set_Nonplayer_Ship_XY_ 0xa1762 R! (several Random) → Create_Nonplayer_Ship_ 0x100010 -
          others: Create_Nonplayer_Ship_ at the same x,y
          antarans+0x2a[size]++; Ships_Try_To_Move_To_ (yes) / Make_Ships_Move_To_ (yes);
          ship status 1, location = target+500, crew_level 2, experience = table+1
```

```
Determine_Event_ 0x2230a R!
  Update_Lucky_Players_Events_ 0x245c4 -        alive lucky (+0x8b9) players: +0xe73 += 1
  Determine_Lucky_Players_Events_ 0x24511 R!    for alive lucky players in order until one hits:
                                                 Random(1000) ≤ +0xe73 / (6|8|10|12 by settings+2, +0xd3); hit → +0xe73 = 0
  if settings+2 == 1 and no lucky hit (turn ≥ 50):
     chance = stardate − last_event_year (0 before turn 50 or while event_delay_count < 5, which is
     incremented here), scaled by difficulty; Random(512) ≤ chance    ← DRAWN EVERY TURN, even when chance is 0
  if event: up to 5 tries of t = Random(29) − 1, rejected by: tutor + bad event; already active;
     no victim (Get_Event_Victim_ R, weighted by population, skips lucky for bad events);
     early-game guards with Event_Check_Hyperspace_Flux_ (yes); per-type target finders:
       Get_Derelict_Applications_ 0x58853 - (Best_Ship_Shield_, Best_Beam_Weapon_: yes)
       Get_Event_Climate_Change_Colony_ 0x2310c R / Pick_Random_Colony_No_Outpost_ 0x23d44 R!
       Pick_Random_Player_In_Contact_ 0x23b7d R!, Pick_Random_Ship_ 0x23ced R!
       Get_Event_Industrial_Accident_Colony_ 0x231b4 R / Pick_Random_Colony_No_Capitol_ 0x23da0 R!
       Get_Event_Mineral_Depletion_Colony_ 0x2325e R, Get_Event_Mineral_Deposit_Colony_ 0x232bb R
       Colony_Has_Event_ 0x23e60 -, Star_Has_Event_ 0x242fc -, Planet_Max_Population_For_Player_ (yes)
       Do_Wormhole_Event_ 0x100519 R!  (type 28: reservoir-picks an in-transit ship of the victim,
            Make_Ship_Arrive_At_Star_ 0xffdda - for its whole group — moves ships NOW, even if the
            event is later rejected; then Update_Ship_Icons_In_Between_Turns_ (cache, yes))
     accepted: event_data[t] = {victim, 1, …}, last_event_year = stardate, event_delay_count = 0;
       per type: Random(5) ×2 (comet 2, supernova 24), Random(21) (15), Random(8) (16),
       Pick_Random_Star_ 0x23bec R!, Pick_Completely_Random_Star_ 0x23a5f R!,
       Event_Pirate_Activity_Strength_ 0x2448f -, Colony_Research_Points_ 0x23b64 -
     Event_Check_Planet_Warning_ 0x23563 -      news when one player holds ≥ ¼, ½, ¾ … of the
                                                 colonised stars (event 30)
     Random(40) == 1 and turn > 50 and COUNCIL_FLAG == 0 → event 31 state 6, +5 = Random(4) − 1
```
Note: `Event_Check_Planet_Warning_` and the `Random(40)` sit inside the
"event happened" branch (by the decompile), so they run only on event turns.

```
Do_All_Ships_XP_Check_ 0x14a27 -
  Best_Instructor_Bonus_ 0x14915 -              per owner 0..7, best Officer_Skill_Bonus_ (yes) of colony
                                                 leaders with status ≥ 0 (any, not only assigned) and ETA 0,
                                                 special skill 0x16, else 0x17; an owner == owner test is dead
      Officer_Has_Special_Skill_ 0x9469e -
  for ship 0..499 with status < 5 and owner < 8: Ship_Owner_ (yes),
      Do_XP_For_Ship_ 0x149d5 -                  academy first, then experience += 1 + bonus (cap s16 0x17d186)
          Do_Academy_At_Location_Check_ 0x1487a -  status 0 only: +1 per own colony at the star with buildings[0x26]
          Calc_Ship_Level_ 0x147e7 -              crew_level from thresholds 0x17d176, +1 warlord (+0x8bd), max 5
```
Note: the loop runs over all 500 slots, not `NUM_SHIPS`.

```
Event_Twiddle_ 0x206a2 R!   (Go: yes, turn_eventtwiddle.go; jump table 0x20612)
  event 19 (0x19ac50 is its state byte): 4 → 3. Then for t = 0..35: 5 → 3; 4/6 → 2;
  only states 1 and 2 go on, and 1 becomes 6 ("news turn") before the switch.
  Record: +0 victim, +1 state, +2 s8 duration/countdown, +3/+5/+7 s16 w1 w2 w3.
  "end" = state 5 (3 next turn); "tick" = byte > 4 and Random(20) == 1 → end,
     byte > 20 → end, then byte += 1.
  0 derelict: Player_Gets_Tech_App_(victim, w1), and w2 if ≠ 0; end
  1 climate change: planet w1 climate = 8, food_per_farmer += tbl[8] − tbl[old] (u8,
     0x17d81c); its colony: Colony_Calculation_, Delete_Prod_From_Colony_Queue_(0x2c); end
  2 comet (not on the news turn): w2 −= Ship_Size_Points_At_Star_ 0x23b28 (all 500 slots,
     status 0 at the star, Σ hull+1); w2 > 0: byte −= 1, at ≤ 0 Allocate_Comet_Hit_(w1)
     (turn_bombing.go) and end; w2 ≤ 0: end. Then Random(20) == 1 and w3 ≠ w2 → state 4
  3 research loss: r = Random(50)+50, capped at the research points; w1 = r,
     w2 = research field (+0x321); points −= r; end
  4/5: Change_Relations_(∓100, victim, w1, 0, 0, 0); end
  6: BC += w1; end.   15: BC −= w2, floor 0; end
  7 earthquake: end first; Allocate_Earthquake_Hit_(w1) → w2 colonists, w3 buildings
     lost; both < 1 → state 0, victim −1; Colony_Calculation_(w1)
  8 ship lost: officer ≠ −1 → w2 = officer, Kill_Officer_; Kill_Ship_(w1); end
  9 flux, 27: state 2 only: tick.   17 boom: state 2 only: Colony_Calculation_(w1), tick
  10 industrial accident: planet w1's colony ≠ −1 → Allocate_Industrial_Accident_Hit_,
     Colony_Calculation_; end
  11 / 12: planet w1 minerals −1 (floor 0) / +2 (cap 4); its colony recalculated; end
  13: nothing (stays 6, then 2)
  14 pirates (not news): chance = w2·100/w3 (s16); Random(100) ≤ chance → every player
     with a colony at star w1 loses a freighter (FreightersTotal > 0); w2 −= ship size
     points at w1; ≤ 0 → end
  16 plague (not news): w2 −= Colony_Research_Points_(w1), Colony_Calculation_; ≤ 0 → end;
     byte > 4 and Random(20) == 1 → state 4 (not 5); byte += 1 (no 20-turn cap)
  18: w1 = research field; Give_Player_Field_(victim, w1, 1); research points 0, field 0; end
  19..23 monsters: news turn → Build_Nonplayer_Ship_(owner t−9: 10 Amoeba, 11 Crystal,
     12 Dragon, 13 Eel, 14 Hydra; star w1; roaming 1); else byte > 4 → end; w2 ≠ −1 → end
     (Event_Monster_Update_ writes byte and w2, see D8)
  24 supernova (not news): w2 −= research of each colony at star w1 (each recalculated);
     w2 > 0: byte −= 1, at ≤ 0 every planet climate 1 and Destroy_Colony_(c, −1), end;
     else end. Then Random(20) == 1 and state ≠ 5 → state 4
  25 anomaly: state 2 only: colonies at star w1 recalculated, tick
  26 warp beast: state 2 only: byte > 4 and Random(20) == 1 → end; byte > 20 → end;
     unless ended Do_Hyperspace_Beast_(victim) and victim = Get_Event_Victim_(good[26]);
     byte += 1
  28..35: end
Build_Nonplayer_Ship_ 0xa16bf (jump table 0xa16a7, owner 9..14; other → −1)
  Load_<monster>_Ship_Design_(design, roaming) (turn_monsters.go): Clear_Structure_ 0x63,
     strcpy of E_Strings_ 0xad Amoeba, 0xf6 Crystal, 0x110 Dragon, 0x119 Eel, 0x144 Hydra,
     0x293 Guardian; roaming builds the big variant with Best_Warp_Drive_(owner) as drive
     and FTL 1; the small one keeps drive 0 but takes its combat speed from that drive
  Set_Nonplayer_Ship_XY_(star, owner, design, roaming) (scatter = roaming)
  Set_Nonplayer_Ship_Destination_ 0xa1a23: roaming → Ships_Try_To_Move_To_ /
     Make_Ships_Move_To_ (a refused order prints and waits for a key: debug), status 1,
     location star+500; else status 0 at the star; Parsecs_Between_Points_ result unused
Do_Hyperspace_Beast_ 0x100618 R!
  victim ≠ −1: reservoir Random(k) over the victim's status-1 ships from the last
  message (type 9, Add_Msg_ to every player NUM_PLAYERS−1..0, +7 = 1 for the victim):
     +2 ship (−1 none), +4 star (low byte of location + 12), +5 officer, +6 fate;
     +1 and +8..+17 are never written (stale stack), +4..+6 also not without a ship,
     +6 not when the ship had no officer
  officer: Move_Officer_To_Pool_ first (so its ETA test after it never holds), then
     owner without app 0xb3 → fate 0 (Kill_Officer_); else the largest (strictly) type-0
     group mate without officer (same owner, status, x, y, location, speed) takes it
     (leader status 1, location) → 2; mates but none free → 1; no mates → 0 (killed)
  Update_Ship_Icons_In_Between_Turns_; the ship's status = 8
```

```
Check_For_Officer_Level_ 0x92fda -              leaders 0..66 with status 0/1 and owner ≠ −1:
  Give_Officer_XP_(owner, o, 1) 0x97c64 -        experience += 1 (s16, cap 30000); level (unowned: base level
                                                 without Warlord, max 5; else Owned_Officer_Level_) changed →
                                                 colony leader with location: Star_Colony_Calculation_(loc, owner);
                                                 level_up = 1 (any type)
      Star_Colony_Calculation_ 0xe2ab1 -          orbits 4..0: Pre_Import_Computing_ + Pass_Out_Imports_(owner,
                                                 colony) per own non-outpost colony (the colony pointer makes
                                                 Pass_Out_Imports_ regrow the fed colonies and this one), then
                                                 Update_Player_Stats_(p) (all yes; Go `starColonyCalculation`)
Decrement_Officer_ETA_ 0x934cf -
  status 4: eta += 1 (s8); ≥ 30 → Deassign_Officer_ 0x933f2 - (Increase_Officer_Level_By_One_ 0x979a0 -:
     experience = start of the next level from the s16 table at 0x925a2 {0,60,150,300,500,1000}, Warlord
     counted, none past level 5; colony leader at a star: star+0x28 (special) = 0 if marooned (status 5),
     star+0x42[owner] = −1 if it is this leader; ship officer: ship.officer = −1 if Ship_Owner_ == owner and
     it is this leader; then eta 0, status −1, location −1, owner −1; no colony recompute)
  status 1, eta > 0: eta −= 1; arrives (0) as colony leader → Star_Colony_Calculation_(loc, owner)
```

```
Random_Officer_Check_(p) 0x97a66 R!              for every p < NUM_PLAYERS (dead ones too)
  Chance_To_Hire_Hero_ 0x9781d -                 0 before turn 5; else turns since last offer (+0xe6a)
                                                 +1 (+6 charismatic, −10 repulsive) + bonuses of own leaders
                                                 with skill masks 0x17d2cd/0x17d2df, / (officers + 1)
      N_Fleet_Officers_Owned_By_Player_ 0x9776c -, N_Star_Officers_Owned_By_Player_ 0x977af -,
      Officer_Status_Ok_ 0x9773f -, Delta_Stardate_ 0x64395 - (stardate − 35000)
  if officers < 8 and chance > 0: Random(100) − 1 < chance
  → Generate_Random_Officer_ 0x97ad4 -
      Select_Leader_For_Hire_ 0x97b2d R!         Random(min(67, turn/5 + 10 + NUM_PLAYERS (±10 / ÷2)))
                                                 repeated until an available leader of a type with < 4 owned,
                                                 at most 101 draws
          Leader_Available_For_Hire_ 0x97c2d -    owner −1, status −1, not 65/66
      player+0x5a1 = leader (−1 if none), +0xe6a = stardate, leader.owner = p
```

```
Add_Ongoing_Event_Msgs_ 0xef827 -
  Event_Check_Hyperspace_Flux_ (yes) → Add_Msg_ type 8 to every player
  colonies from the last: Event_Check_Plague_ (yes) type 6, Event_Check_Population_Boom_ (yes) 7,
     Event_Check_Comet_ 0x23536 - 8, to the owner
  stars from the last: Event_Check_Space_Anomaly_At_Star_ (yes) → type 0x10 to every player
  stardate == 35023 and host date is 1 April: type 0x15 to every player (time_, localtime_)
  Add_Msg_ 0xef629 -                             append to the player's window; when full replace the
                                                 lowest-priority (+1) message if the new one is higher
Record_History_ 0x10208a -
  Compute_Total_Building_Costs_ 0xe2671 -        per player, sum of building costs (table 0x17eb32) of
                                                 non-outpost colonies
  samples: population +0xa6, buildings, tech +0x224, fleet Σ 10·2^(size+1) of ships status < 2;
  each series divided by a shared divisor (bill_savegame) raised until every value ≤ 250; on a change
  all 350 old bytes are rescaled; then written at the ring index, index = (index + 1) % 350
```

### Shared helpers (first appearance above)

| Routine | Addr | RNG | Go | Note |
|---|---|---|---|---|
| `Get_Weighted_Choice_` | 0x586d4 | R! | no | `Random(sum)`; weights halved in place while sum ≥ 512; no draw when sum 0 |
| `Set_Nonplayer_Ship_XY_` | 0xa1762 | R! | no | placement near a star for Antarans and monsters; up to 5 draws |
| `Create_Nonplayer_Ship_` | 0x100010 | - | no | fills a free ship slot |
| `Colony_Calculation_` | 0xe2a70 | - | no (parts yes) | one colony: Pre_Import_Computing_, Pass_Out_Imports_, Update_Player_Stats_ |
| `Star_Colony_Calculation_` | 0xe2ab1 | - | no (parts yes) | same for the owner's colonies at a star |
| `Kill_Officer_` / `Kill_Ship_` / `Move_Officer_To_Pool_` | 0x941c6 / 0xa163a / 0x944a3 | - | no | `Do_Player_Colony_Calculations_` (yes) inside |
| `Change_Relations_` | 0x4e3b5 | R | no | diplomacy phase owns it |
| `Make_Ship_Arrive_At_Star_` | 0xffdda | - | no | movement phase owns it |
| `Update_Ship_Icons_In_Between_Turns_` | 0x8a4c4 | - | yes | display cache only (star owners, icons) |

### UI-only, debug and multiplayer branches

- `Assign_Auto_Function(Null_Autofunction_)` and `Set_Mouse_List` are UI.
- `Build_Antaran_Ships_` and `Deploy_Antaran_Ships_` sprintf a debug log into
  `_temp_string`; output unused but harmless to port as no-ops.
- `Assert_Marooned_Leaders_` runs only in a non-release build and writes
  nothing. `Set_Nonplayer_Ship_Destination_` can call `hgetch_` (debug
  assert, prints and waits for a key): unreachable in normal play.
- No multiplayer-only branch in this group; the hotseat/network logic sits
  in `Next_Turn_` / `Do_Next_Turn_`.

### Bit-exactness hazards

1. **Every-turn draws regardless of outcome**, in order:
   - `Determine_Lucky_Players_Events_`: one `Random(1000)` per alive lucky
     player, in player order, until the first hit.
   - `Determine_Event_`: `Random(512)` every turn when random events are on
     (settings+2 == 1) and no lucky hit — even before turn 50 and during
     the 5-turn delay, when the chance is 0.
   - `Random_Officer_Check_`: for each player whose chance is > 0 (from
     turn 5) and who owns < 8 officers, one `Random(100)` — dead players
     are not skipped; a hit adds 1..101 draws in `Select_Leader_For_Hire_`.
   - `Antarans_Should_Attack_`: `Random(200)` whenever pressure > 0 and the
     4× force test fails (with Antarans on).
   - `Event_Twiddle_`: `Random(20)` per running comet and supernova (not on
     the news turn), and per flux/boom/anomaly/plague/warp beast/27 whose
     duration byte is over 4.
2. `Do_Wormhole_Event_` moves ships while the type is still being
   validated; a later rejection in the same try does not undo the move.
3. `Add_Ongoing_Event_Msgs_` reads the host clock (April 1 at stardate
   35023). The port needs an injectable clock to stay deterministic.
4. Unsaved state that changes the future: `_antaran_ship_cost` (90% after
   the first offensive titan on Hard+), `_n_max_antaran_off/def_ships`
   (zeroed entries by `Build_Antaran_Ships_`), `_offensive_titan_ship_built`.
   A save/load resets them from the executable's data; the port must do the
   same (reload from `ORION2.EXE` at load), or a loaded game diverges from
   one played through.
5. `Get_Weighted_Choice_` halves the caller's weight array in place; the
   halving count changes the draw range.
6. `Record_History_` divisors and ring index live in `bill_savegame`
   (`.data`, saved); integer division order must be kept (unsigned).
7. `Do_All_Ships_XP_Check_` iterates 500 slots, not `NUM_SHIPS`, and skips
   by status < 5 and owner < 8 (monsters/Antarans get no XP).

### Test pair t1 (before → after), this group's share

- `leaders[*].Experience` +1 (27 rows): `Check_For_Officer_Level_`.
- `leaders[*].ETA` +1 for offered leaders (status 4): `Decrement_Officer_ETA_`;
  leader 41 status −1 → 4 with ETA 0 → 1 means it was put on offer earlier in
  the same turn (AI leaders phase consumed player 6's offer 41 →
  `OfficerForHire` −1); no new offer this turn.
- `ship[*].Experience` (86) and `CrewLevel` (2): `Do_All_Ships_XP_Check_`.
- `player[*].History[*]` (31) and `bill_savegame` +2 (ring index 0x0c →
  0x0d) and +6 (a divisor 0x73 → 0x74, triggering a rescale of one series):
  `Record_History_`.
- `n_reports` 4 → 0, `done_colonize_check` cleared: `Initialize_Reports_`
  (then begin-of-turn report phase may set bits again).
- `antarans` (1 byte), `msgs`/`n_msgs`: `Antaran_Invasion_Check_` (pressure
  counter), `Allocate_Msg_Slots_` + `Add_Msg_` from all phases.
- `stardate` 35362 → 35363.

Porting order inside the group: `Initialize_Reports_`, `Allocate_Msg_Slots_`,
`Add_Msg_`, `Record_History_`, `Do_All_Ships_XP_Check_`,
`Check_For_Officer_Level_`/`Decrement_Officer_ETA_` (pure, no RNG — verify
first against t1); then the RNG consumers in call order: Antarans,
`Determine_Event_`, `Event_Twiddle_`, `Random_Officer_Check_`.

## Phase 10: begin of the new turn (screen 0x27) up to Main_Screen_

`Next_Turn_` leaves `_current_screen = 0x27`. `Screen_Control_` runs
`Reports_Screen_` → `Must_Display_Reports_` (query) → fade → `Do_Begin_Of_Turn_`
→ `Begin_Of_Turn_` → `Main_Screen_Report_Handler_` → `Display_Report_(PLAYER_NUM)`
in a loop. All of this is specified in `docs/screens/reports-and-begin-of-turn.md`
and the no-report path is ported (`internal/game/reports.go`). What a real
turn adds is the "show" side of every check. Only the human (`PLAYER_NUM`)
goes through it; AI players never see reports.

Since `Initialize_Reports_` zeroed `done_player_reports`, `displayed_msgs` and
`displayed_gnn` during the turn, the checks run again after every turn.

| Check (in order) | Addr | What it shows | Logic / save state in show mode | RNG (show) | Ported |
|---|---|---|---|---|---|
| `Antarans_` | 0xfe552 | Antaran attack flic / Antaran room | `antarans+0x41` bit; `Main_Antaran_Room_Screen_` (ANTAROOM.C, `Get_Antaran_Room_Message_` R!) | R! (`Random_(10)`, only with a colony destination) + room | no |
| GNN: `Check_For_Event_` | 0x2027e | GNN event news | `Setup_Next_Event_` 0x21371 per event with state 4..6 (5 direct `Random_` calls: random GNN text), `Reset_Events_`; `displayed_gnn` | R! (consuming) | no |
| `Set_Initial_Tech_` | 0xfd911 | first research pick | `Tech_Select_` → `Show_Off_Researched_Tech_`/`Science_Room_` (RNG only in `Jumble_Bitmap_`, seed restored) | neutral | partly (tech gain ported) |
| `Has_Research_Breakthrough_` | 0xfe2e8 | breakthrough + pick | `Tech_Select_`, +0x30 = 0 | neutral (UI) unless tech gain designs ships (`Auto_Design_Ship_` R! for AI only) | partly |
| `Has_Evolutionary_Upgrade_` | 0xfe31f | Tolerant/evolution pick | `Setup_Evolutionary_Upgrade_` | - | no |
| `Has_Officer_For_Hire_` | 0xfe251 | hire popup | `Set_Officer_To_Player_`, BC, +0x5a1 | neutral (plasma) | no |
| `Colonization_` | 0xfdb01 | colonise/outpost prompts | `Make_New_Colony_`/`Make_New_Outpost_`, `Kill_Ship_` | see reports doc | no |
| `Explored_New_Star_` | 0xfd95a | system discovery popup | star+0x34, +0x28 | neutral (plasma) | no |
| `Officer_Made_Level_Report_` | 0x949a7 | level-up popup | leader +0x38 | neutral | no |
| `Has_Report_` | 0xfe359 | spy/combat reports, stolen tech | report +1 = −1 | neutral | no |
| `Has_Diplomacy_Messages_` | 0xfe408 | AI ambassador (the tribute demand) | `Npc_Diplomacy_Screen_(i, 0)` for each AI i with player[p]+0x657+… ≠ 0 and not ignored (+0xe60); clears the message byte. Network + interactive message: `Net_Request_AI_Diplomacy_` handshake (**multiplayer only**) | **R! consuming, in UI** (below) | no |
| `Has_Msg_` | 0xfe0ea | turn summary (screen 0x28, `Turn_Summary_Popup_` 0x10fb02) | `displayed_msgs` | neutral (plasma/mini main screen) | no |
| `Empty_Colony_Queue_` | 0xfdf93 | colony screen | — | Colony_Screen_ restores the seed around its building array | no |
| `Need_To_Set_Occupation_Policy_` | 0xfe02c | colony screen | — | same | no |

`Must_Display_Reports_` finally calls `Redirect_Newly_Built_Ships_(p)`
(ported) and, for network games, `Do_Autosave_`.

### The AI diplomacy screen (the tribute demand of the test pair)

`Npc_Diplomacy_Screen_(ambassador, 0)` (0x1afa6; Ghidra decompiles it with
its body at 0x1a53d..0x1abb5, which objdump files under
`Npc_Display_Diplomacy_Messages_` 0x1a507), DIP-SCRN.C. Returns at once
if the ambassador is eliminated (+0x24 == 1).

Order of save-state and RNG effects:

1. UI setup; `_response_message = player[amb]+0x657+PLAYER_NUM`
   (with flag 1: player[amb]+0x7c9).
2. `Diplomacy_Fade_In_` 0x17c07 → `Diplomacy_Mouse_Init_` 0x1b92e:
   **`Random_(3)`** (`_diplomacy_bad_music = 0xd + r`); →
   `Get_Diplomacy_Statement_` 0x1d565: **`Random_(n)`** with n the number of
   variants of the statement in DIPLOMS?.LBX (picks the sentence).
3. Demands/proposals (message 0x1c..0x24 demands, 0x53..0x55, 0x61..0x68,
   0x56, 0x69, 0x6a, 0x7d): option 2: response 0x80/0x81 ("hail", by
   relations < −12) is displayed, then the original message again; each
   change of `_response_message` makes `Draw_Diplomacy_Synch_Mode_` call
   `Get_Diplomacy_Statement_` once more (**one `Random_` per distinct
   message shown**, not per frame: guarded by `_old_response_message`).
4. `Get_Non_Player_Proposals_` 0x1ac12: list field accept/reject (UI); a
   tech-exchange proposal (0x65) runs `Player_Gets_Tech_App_` twice
   (R below via ship redesign of AI players).
5. Accept (`0`): `NPC_Proposal_Rejection_Accept_(amb, p, 1)` (player+0x827
   bias both ways, no RNG); demand 0x1c..0x24 or 0x69..0x6a (threat: "give in or war") → response 0x26 shown
   (**`Random_`** in the statement), `Accepted_Npc_Demand_Resolution_(p, amb)`
   0x1abbe: **`Random_(13)`**, `Change_Relations_(r + 11, p, amb)` (→
   `Reward_Check_`, R!), player[p]+0x71f+amb = player[amb]+0x71f+p = 0;
   then `Npc_Give_Gift_(p, amb)` 0x1aeb5: tribute treaty
   (`Start_Tribute_Treaty_`, +0x75f = 1/2 → 5 %/10 %), else tech
   (`Player_Gets_Tech_App_`), else BC (+0x777, moved from +0x32 to the AI),
   else a star (+0x7b7 → `May_Surrender_Star_`/`Surrender_Star_`).
   Reject: `NPC_Proposal_Rejection_Accept_(…, 0)`; demand → all treaties
   broken (`Break_Trade_`, `Break_Research_`, `Break_Tribute_`,
   `Break_Treaties_`), response 0x25; 0x56 → `Break_Treaties_`; 0x69 →
   `Change_Relations_(−50)`; 0x6a → sneak attack target or `Declare_War_`.
6. Tear-down: `Fade_Stream_Down`, restore auto function, release blocks,
   `Play_Background_Music_`.

In the test pair: player 2 sent message 0x6a (a threat demand from
`NPC_Sneak_Attacks_`, see section C) asking player 0 for a tribute treaty
(TurnScratch[1] = 2); the user accepted: `player[0].Relations[2]` and `player[2].Relations[0]` 0 → 21
(`Random_(13)` = 10, +11), `player[0].Tribute[2]` 0 → 2, `TreatyTimer[2]`
10, modifiers +22. These are the only after.GAM changes made after the turn
calculation (plus the RNG draws listed above).

### RNG in the UI: what counts

Many drawing routines call `Random_` but bracket it with
`Get_Random_Seed_`/`Set_Random_Seed_`, so they are seed-neutral:
`Draw_Exploding_Plasma_Box_` (popups, boxes), `Create_Plasma_Line_`,
`Jumble_Bitmap_`, `Build_Pop_Array_`, `Make_Bldg_Array_For_Colony_`,
`Draw_Fighter_`, `Beam_SFX_`, `Draw_Fighter_Beam_`, `Bolt_Segment_`,
`Draw_Main_Screen_For_Zooming_`, `Hi_Score_Screen_`. The port can run them
on a private copy of the seed.

Consuming UI RNG between the turn and `Main_Screen_` (must be ported in
order): the diplomacy screen (`Diplomacy_Mouse_Init_`,
`Get_Diplomacy_Statement_`, `Diplomacy_Need_Better_Offer_`,
`Diplomacy_Give_In_To_Demand_`, `Get_*_Response_`/`Get_Break_*_Message_`,
`Check_Treaty_Proposal_`, `Diplomacy_Test_`, `NPC_Message_Rejection_Check_`,
`Get_Tech_Exchange_Reaction_`, `Population_Comparison_` when the human
initiates diplomacy), GNN `Setup_Next_Event_`, `Antarans_` and the Antaran
room. `Human_Requesting_Diplomacy_` (called each `Main_Screen_` pass)
draws only when network request bits 0x1ab069 are set (**multiplayer only**).

## Appendix: direct `Random_` callers reachable from `Next_Turn_Calc_`

From the symbol call graph (so the fall-through cases above are missing,
and tactical combat UI is included). "Consumes" means no seed save/restore
in the routine itself; a caller may still restore (e.g. `Jumble_Bitmap_`
around `Jumble_Bitmap_Pixels_`, `Colony_Combat_Screen_` replaying from the
seed `Invade_` saved). Section E classifies the combat ones.

| Routine | Addr | Module | Direct calls | Seed |
|---|---|---|---|---|
| `Add_Building_` | 0x13fd9 | ERICNET.C | 2 | consumes |
| `Vote_Check_` | 0x16021 | COUNCIL.C | 1 | consumes |
| `Get_Player_Voted_Winner_Message_` | 0x16bfc | COUNCIL.C | 1 | consumes |
| `Event_Twiddle_` | 0x206a2 | EVENTS.C | 8 | consumes |
| `Determine_Event_` | 0x2230a | EVENTS.C | 9 | consumes |
| `Allocate_Comet_Hit_` | 0x23780 | EVENTS.C | 2 | consumes |
| `Allocate_Industrial_Accident_Hit_` | 0x23833 | EVENTS.C | 2 | consumes |
| `Allocate_Earthquake_Hit_` | 0x238a8 | EVENTS.C | 2 | consumes |
| `Pick_Completely_Random_Star_` | 0x23a5f | EVENTS.C | 2 | consumes |
| `Pick_Random_Player_In_Contact_` | 0x23b7d | EVENTS.C | 1 | consumes |
| `Pick_Random_Star_` | 0x23bec | EVENTS.C | 1 | consumes |
| `Pick_Random_Ship_` | 0x23ced | EVENTS.C | 1 | consumes |
| `Pick_Random_Colony_No_Outpost_` | 0x23d44 | EVENTS.C | 1 | consumes |
| `Pick_Random_Colony_No_Capitol_` | 0x23da0 | EVENTS.C | 1 | consumes |
| `Determine_Lucky_Players_Events_` | 0x24511 | EVENTS.C | 1 | consumes |
| `NPC_Threats_` | 0x252d5 | NPCDIPLO.C | 2 | consumes |
| `NPC_To_NPC_Treaty_Negotiations_` | 0x2552d | NPCDIPLO.C | 7 | consumes |
| `NPC_Diplomacy_Buildup_Penalties_` | 0x25c7c | NPCDIPLO.C | 2 | consumes |
| `NPC_Declarations_Of_War_` | 0x25df1 | NPCDIPLO.C | 5 | consumes |
| `NPC_Required_Alliances_` | 0x2689d | NPCDIPLO.C | 1 | consumes |
| `Get_Tech_Exchange_Expectation_` | 0x26fba | NPCDIPLO.C | 1 | consumes |
| `NPC_Tech_Exchange_Check_` | 0x2720f | NPCDIPLO.C | 1 | consumes |
| `NPC_Good_Proposal_Check_` | 0x2736e | NPCDIPLO.C | 1 | consumes |
| `NPC_Treaty_Hatred_` | 0x277cf | NPCDIPLO.C | 1 | consumes |
| `NPC_Surrender_Check_` | 0x27a3d | NPCDIPLO.C | 1 | consumes |
| `Ai_Self_Destruct_Check_` | 0x28168 | CMBTAI.C | 1 | consumes |
| `Auto_Move_Ship_` | 0x2a9a5 | CMBTAI.C | 1 | consumes |
| `Apply_Internal_Damage_` | 0x35251 | CMBTFIR2.C | 1 | consumes |
| `Select_Internal_` | 0x36810 | CMBTFIR2.C | 1 | consumes |
| `Defensive_Fire_Check_` | 0x36ed1 | CMBTFIR2.C | 1 | consumes |
| `Resolve_Raid_` | 0x37308 | CMBTFIR2.C | 2 | consumes |
| `Capture_Ship_` | 0x38312 | CMBTFIR2.C | 1 | consumes |
| `Select_Raid_Internal_` | 0x384a8 | CMBTFIR2.C | 1 | consumes |
| `Apply_Combat_Artemis_Net_` | 0x3868f | CMBTFIR2.C | 3 | consumes |
| `Fire_Beam_Weapon_` | 0x394f7 | CMBTFIRE.C | 3 | consumes |
| `Destroy_Colony_Defense_` | 0x3a19e | CMBTFIRE.C | 1 | consumes |
| `Apply_Damage_To_Planet_` | 0x3a3c3 | CMBTFIRE.C | 1 | consumes |
| `Fire_Bomb_` | 0x3a6ca | CMBTFIRE.C | 1 | consumes |
| `Fire_Fighter_Bomb_` | 0x3ac20 | CMBTFIRE.C | 1 | consumes |
| `Fire_Fighter_Beam_` | 0x3ad57 | CMBTFIRE.C | 1 | consumes |
| `Energy_Absorber_Attack_` | 0x3b077 | CMBTFIRE.C | 1 | consumes |
| `Resolve_Missile_` | 0x3d2df | CMBTMIS.C | 4 | consumes |
| `Draw_Fighter_` | 0x3dbd1 | CMBTMIS.C | 2 | restored |
| `QGet_Target_` | 0x41e88 | COMBAT.C | 1 | consumes |
| `Missile_Attack_` | 0x41f80 | COMBAT.C | 1 | consumes |
| `Strat_Bomb_Attack_` | 0x420c0 | COMBAT.C | 1 | consumes |
| `Strat_Special_Attack_` | 0x4215c | COMBAT.C | 1 | consumes |
| `Beam_Attack_` | 0x4221f | COMBAT.C | 2 | consumes |
| `Apply_Bio_Weapon_Penalty_` | 0x4b11e | COMBINIT.C | 1 | consumes |
| `Diplomacy_Growth_` | 0x4dd6b | DIPLOMAC.C | 15 | consumes |
| `Reward_Check_` | 0x4ea03 | DIPLOMAC.C | 2 | consumes |
| `Determine_Diplomacy_Messages_` | 0x4eb06 | DIPLOMAC.C | 1 | consumes |
| `Determine_Bad_Message_` | 0x4f0dc | DIPLOMAC.C | 4 | consumes |
| `Peace_Proposal_` | 0x4f694 | DIPLOMAC.C | 1 | consumes |
| `Get_Demands_` | 0x4f93b | DIPLOMAC.C | 4 | consumes |
| `Required_Alliances_` | 0x4fd30 | DIPLOMAC.C | 1 | consumes |
| `Determine_Good_Message_` | 0x4fe25 | DIPLOMAC.C | 4 | consumes |
| `Unprovoked_Good_Messages_` | 0x501ca | DIPLOMAC.C | 7 | consumes |
| `Modifier_Diplomacy_Adjustments_` | 0x50dac | DIPLOMAC.C | 8 | consumes |
| `Declare_War_` | 0x51078 | DIPLOMAC.C | 1 | consumes |
| `Break_Research_` | 0x5175b | DIPLOMAC.C | 3 | consumes |
| `Break_Trade_` | 0x519ac | DIPLOMAC.C | 3 | consumes |
| `Break_Tribute_` | 0x51c02 | DIPLOMAC.C | 2 | consumes |
| `NPC_Sneak_Attacks_` | 0x53edb | DIPLOMAC.C | 8 | consumes |
| `Sneak_Attack_Evaluations_` | 0x544a1 | DIPLOMAC.C | 8 | consumes |
| `Get_Weighted_Choice_` | 0x586d4 | KEN.C | 1 | consumes |
| `Get_Ship_Class_` | 0x5fa87 | AIDESIGN.C | 4 | consumes |
| `Determine_Beam_Weapon_Modifications_` | 0x60b59 | AIDESIGN.C | 4 | consumes |
| `Auto_Design_Ship_` | 0x616a5 | AIDESIGN.C | 3 | consumes |
| `Antarans_Should_Attack_` | 0x63edd | ANTARANS.C | 1 | consumes |
| `Build_List_Of_Stars_By_Player_` | 0x64311 | ANTARANS.C | 2 | consumes |
| `Get_Antaran_Target_Star_` | 0x645a4 | ANTARANS.C | 1 | consumes |
| `Draw_Exploding_Plasma_Box_` | 0x83efd | MAINSCR.C | 2 | restored |
| `Random_Officer_Check_` | 0x97a66 | OFFICER.C | 1 | consumes |
| `Select_Leader_For_Hire_` | 0x97b2d | OFFICER.C | 1 | consumes |
| `Get_Random_Antaran_Tech_` | 0x982b6 | OFFICER.C | 1 | consumes |
| `Set_Nonplayer_Ship_XY_` | 0xa1762 | SHIPS.C | 5 | consumes |
| `Draw_Generic_Beam_` | 0xa6b9c | BEAMS.C | 2 | consumes |
| `Beam_SFX_` | 0xa8197 | BEAMS.C | 8 | restored |
| `Draw_Beam2_` | 0xab52e | BEAMS.C | 2 | consumes |
| `Anti_Missile_Rockets_` | 0xaba09 | CMBTSPEC.C | 1 | consumes |
| `Gyro_Destablizer_` | 0xac136 | CMBTSPEC.C | 3 | consumes |
| `Plasma_Web_` | 0xacf83 | CMBTSPEC.C | 1 | consumes |
| `Do_Spherical_SFX_` | 0xade18 | CMBTSPEC.C | 13 | consumes |
| `Play_Ship_Explosion_SFX_` | 0xaec0b | CMBTSPEC.C | 1 | consumes |
| `Build_Road_List_Based_On_Bldg_List_` | 0xb6099 | COLDRAW.C | 4 | consumes |
| `Pick_Random_Veggie_Anim_Entry_For_Colony_CR_` | 0xb6647 | COLDRAW.C | 1 | consumes |
| `Veggie_Type_For_Colony_` | 0xb66d0 | COLDRAW.C | 5 | consumes |
| `Pick_Random_Road_At_And_Around_CR_` | 0xb6860 | COLDRAW.C | 1 | consumes |
| `Build_Veggie_List_Based_On_Bldg_List_` | 0xb6977 | COLDRAW.C | 5 | consumes |
| `Do_Unit_Turn_` | 0xb7e84 | COLGCBT.C | 4 | consumes |
| `Pick_Next_Action_` | 0xb828c | COLGCBT.C | 5 | consumes |
| `Move_Is_Legal_` | 0xb83ed | COLGCBT.C | 1 | consumes |
| `Do_Legal_Move_Check_` | 0xb8575 | COLGCBT.C | 1 | consumes |
| `Add_Unit_` | 0xb87cd | COLGCBT.C | 3 | consumes |
| `Add_Initial_Unit_` | 0xb88b2 | COLGCBT.C | 4 | consumes |
| `Get_Bldg_CR_` | 0xbbd37 | COLONY.C | 1 | consumes |
| `Insert_Bldg_Into_Array_` | 0xbc05e | COLONY.C | 1 | consumes |
| `Make_Bldg_Array_For_Colony_` | 0xbc30b | COLONY.C | 2 | restored |
| `Apply_Next_Bomb_Hit_` | 0xca85c | MAINPUPS.C | 4 | consumes |
| `Compute_Needs_` | 0xcf40d | AIBUILD.C | 5 | consumes |
| `Assign_Empire_Building_` | 0xd10ee | AIBUILD.C | 4 | consumes |
| `Freighter_Cheat_` | 0xd5d99 | AIDUDES.C | 1 | consumes |
| `Do_Ship_Upgrade_Cheats_` | 0xd5e19 | AIDUDES.C | 2 | consumes |
| `Assign_Additional_Unblockaded_Farmers_` | 0xd6ad4 | AIDUDES.C | 1 | consumes |
| `NPC_Dest_` | 0xdb6d2 | AIMOVE.C | 1 | consumes |
| `Compute_AI_Tech_Info_` | 0xdc47c | AITECH.C | 5 | consumes |
| `Resolve_Bomb_Hit_` | 0xdcebd | BOMB.C | 1 | consumes |
| `Resolve_Biological_Hit_` | 0xdd13e | BOMB.C | 4 | consumes |
| `Apply_Colony_Pop_Growth_` | 0xe2dca | COLCALC.C | 3 | consumes |
| `Apply_Assimilation_` | 0xe3456 | COLCALC.C | 1 | consumes |
| `Ghost_Colony_Stuff_` | 0xe3e9a | COLCALC.C | 1 | consumes |
| `Ensure_Uncreative_Field_OK_` | 0xe408f | COLCALC.C | 1 | consumes |
| `Check_For_Research_Breakthrough_` | 0xe44e0 | COLCALC.C | 1 | consumes |
| `Compute_Blockades_` | 0xe5097 | COLCALC.C | 1 | consumes |
| `Get_Surrender_System_` | 0xe5be3 | COLCALC.C | 2 | consumes |
| `Process_Retreating_Ships_` | 0xe6e52 | COMBFIND.C | 2 | consumes |
| `Get_AI_Target_` | 0xe7dca | COMBFIND.C | 1 | consumes |
| `Get_NPC_Target_` | 0xe8029 | COMBFIND.C | 4 | consumes |
| `Pick_Attacker_At_Star_` | 0xe8194 | COMBFIND.C | 2 | consumes |
| `Player_May_Attack_Player_` | 0xe8231 | COMBFIND.C | 1 | consumes |
| `Do_System_Discoveries_At_Star_` | 0xe9927 | COMBFIND.C | 3 | consumes |
| `Ground_Combat_Round_` | 0xec4fe | INVASION.C | 2 | consumes |
| `Change_Colony_Ownership_` | 0xecf41 | INVASION.C | 2 | consumes |
| `Check_Rebellion_` | 0xed260 | INVASION.C | 2 | consumes |
| `Kill_Buildings_` | 0xedb35 | MAINTAIN.C | 1 | consumes |
| `Choose_Hyper_Advanced_Tech_` | 0xfc734 | NEWTECH.C | 1 | consumes |
| `Get_Weighted_Choice_Char_` | 0xfe8da | RUSS.C | 1 | consumes |
| `Get_Weighted_Choice_Short_` | 0xfe92d | RUSS.C | 1 | consumes |
| `Get_Weighted_Choice_Int_` | 0xfe96f | RUSS.C | 1 | consumes |
| `Shuffle_Char_` | 0xfe9b5 | RUSS.C | 1 | consumes |
| `Shuffle_Sint_` | 0xfe9f5 | RUSS.C | 1 | consumes |
| `Shuffle_Int_` | 0xfea3c | RUSS.C | 1 | consumes |
| `Shuffle_` | 0xfea82 | RUSS.C | 1 | consumes |
| `Make_Player_Into_AI_` | 0xfec20 | RUSS.C | 1 | consumes |
| `Do_Wormhole_Event_` | 0x100519 | SHIPMOVE.C | 1 | consumes |
| `Do_Hyperspace_Beast_` | 0x100618 | SHIPMOVE.C | 1 | consumes |
| `Frame_Another_` | 0x100bc5 | SPY.C | 1 | consumes |
| `Allocate_AI_Spies_` | 0x100d19 | SPY.C | 8 | consumes |
| `Steal_App_` | 0x10119c | SPY.C | 3 | consumes |
| `Destroy_Random_Building_` | 0x10130a | SPY.C | 3 | consumes |
| `Resolve_Player_Spies_` | 0x1014a4 | SPY.C | 4 | consumes |
| `Leader_Assassinates_` | 0x101768 | SPY.C | 2 | consumes |
| `Resolve_Assassins_` | 0x1018a3 | SPY.C | 1 | consumes |
| `Process_Trade_Agreement_` | 0x101d53 | TRADE.C | 1 | consumes |
| `Process_Research_Agreement_` | 0x101de5 | TRADE.C | 1 | consumes |
| `Create_Plasma_Line_` | 0x1368d8 | line.c | 5 | restored |
| `Bolt_Segment_` | 0x13bec4 | bolt.c | 4 | restored |
| `Vanish_Bitmap_Pixels_` | 0x147abc | shear.asm | 1 | consumes |
| `Jumble_Bitmap_Pixels_` | 0x147e9f | shear.asm | 1 | consumes |

## Proposed porting order

The aim is to compare against `after.GAM` early, block by block, and to
reach the random seed only at the end. Two harness pieces come first:

- **A turn runner** that loads a save, runs `Next_Turn_Calc_` up to a chosen
  step and saves (or diffs in memory), so each step can be checked alone.
  Steps not yet ported are stubs; the runner reports which blocks match.
- **An RNG trace.** RNG order cannot be checked from saves alone. The best
  oracle is a trace of `Random_` calls (caller address, n, result) from the
  original under DOSBox's debugger (breakpoint at 0x1247a0 minus 0x10000 in
  the LE image) for the t1 turn; the port logs the same and the first
  diverging line names the routine. Without it, only the final seed tells
  whether the whole turn matched.

Order, cheapest first, with what each step should reproduce in t1:

1. **RNG-free bookkeeping**: `Initialize_Reports_`, `Allocate_Msg_Slots_`,
   `Make_Scrap_Ships_Dead_`, `Allocate_New_Ship_Slots_`,
   `Allocate_New_Colony_Slots_`, `Reset_Needless_Ignore_Combat_Ships_Flags_`,
   `Do_All_Ships_XP_Check_`, `Check_For_Officer_Level_`,
   `Decrement_Officer_ETA_`, `Record_History_`, stardate. Expect: all ship
   Owner churn on free slots, colony ReservedFor, star ColonizePlayer,
   ship Experience/CrewLevel, leader Experience/ETA, History +
   `bill_savegame`, `n_reports`, `done_colonize_check`, stardate.
2. **Movement**: `Move_All_Ships_Toward_Stars_` (with the ported
   `Move_Player_1_Turn_To_Star_`), arrival, `Repair_Ships_At_Colonies_`,
   `Move_Settlers_`. Expect: ship X/Y/Location/ETA/Status for ships whose
   orders predate the turn (AI orders given this turn need B).
3. **Economy without RNG**: `All_Colony_Calculations_` (twice, around
   settlers) on top of the ported `Do_Player_Colony_Calculations_`, the
   deterministic parts of `Apply_All_Player_Changes_` and
   `Apply_All_Colony_Changes_` (production, build queues, BC, research
   points, pollution). Expect: colony Production/Output/Upkeep, player
   BC/Income/Research for players whose results do not depend on a draw.
4. **The RNG chain, strictly in call order**, from here on with the trace:
   Antarans and events (F), `Compute_AI_Data_` and the AI phase (B; its
   tech tie draws involve the human too), the diplomacy phase (C), spies,
   breakthroughs, growth, assimilation (D), surrenders, the no-battle path
   of `Search_For_Battles_` (E), `Determine_Event_`, `Event_Twiddle_`,
   `Lose_Out_Of_Range_Ships_`, blockades, rebellions, first contacts,
   diplomacy messages, end-of-turn adjustments, `Random_Officer_Check_`.
   `Auto_Design_Ship_` (B, D, and the tech gain path) is needed along the
   way. Expect: everything else in the turn's part of `after.GAM`, except the
   final seed and the tribute.
5. **Begin-of-turn report phase**: the show side of the checks, starting
   with what t1 needs: `Has_Diplomacy_Messages_` → the AI diplomacy screen
   with its UI draws (`Diplomacy_Mouse_Init_`, `Get_Diplomacy_Statement_`),
   accept/reject, `Accepted_Npc_Demand_Resolution_`, `Npc_Give_Gift_`,
   `Start_Tribute_Treaty_`; then the turn summary (screen 0x28) and the
   remaining popups. Expect: tribute fields and finally the **random seed**
   of `after.GAM`.
6. **Combat proper**: `Strategic_Combat_` and the AI branch of
   `Do_Attacker_Beat_Colony_Stuff_` (AI-vs-AI fights, strategic bombing),
   then `Tactical_Bombardment_` (used for AI-vs-AI bombing when settings
   0xd8 is 0), and finally the tactical combat screen. Needs new test
   pairs with battles.
7. **Rare branches**: the council (an interactive screen inside
   `Next_Turn_Calc_`), Antaran attacks and the Antaran room, GNN events,
   rebellions, end of game. Each needs its own test pair.

Test pairs worth asking the user for next: (a) the same t1 turn with the
tribute *rejected* and with every popup dismissed the same way, to separate
the report phase from the turn; (b) a turn with no begin-of-turn reports at
all (only then is the seed of `after.GAM` the turn's alone); (c) a turn with
an AI-vs-AI battle and one with a human battle under strategic combat; (d)
a council turn; (e) an autosave turn (stardate % 4 == 0) to check that
`Do_Autosave_` changes nothing. Before each TURN press the user should not
open boxes or screens that draw with the seed (the diplomacy screen,
colony invasion popups).

## Open questions

- `settings` bytes +1 and +0x14..0x16 differ in t1 although no turn routine
  writes them; are they the user's in-game option changes, or written by the
  save path?
- Star 31: `Do_Surrender_Colony_` seems to store the raw `SurrenderTo` value
  (9 = player 1 + 8) as the colony owner, yet `after.GAM` has 1; find where
  the 8 is removed (section D). The port stores the player (`to & 7`): the
  raw 9 indexed past the players in `Colony_Race_Pop_Limit_` (a crash in
  AI-only games).
- `Diplomacy_Fade_In_` calls `Get_Diplomacy_Statement_` directly, then
  `Draw_Diplomacy_Synch_Mode_` calls it again when `_response_message !=
  _old_response_message`; check whether the first message is drawn (and
  `Random_` drawn) once or twice.
- `Reward_Check_`'s two `Random_` calls (dropped by Ghidra) and the exact
  meaning of the message codes 0x1c..0x7d (section C).
- Unsaved state that changes the future (Antaran ship cost and limits,
  `_offensive_titan_ship_built`, the April 1 clock check in
  `Add_Ongoing_Event_Msgs_`): the port must reset them on load exactly as
  the original does (section F).
- `Fire_Ship_` reached from `Tactical_Bombardment_` outside the combat
  screen: does it draw through `Beam_SFX_` (section E)?
- Whether the AI blocks freed by `Deallocate_AI_Data_` are read by
  `Search_For_Battles_` (section B).
- Hotseat: `Antarans_` tests `PLAYER_NUM`'s bit, `Get_Local_Human_Target_`
  writes `PLAYER_NUM`; both matter only once hotseat is ported.

Closed while mapping (to move into the relevant specs): settings 0xd8 = 1 is
strategic combat (section E); the star+0x28 report
markers (player + 50..90) are written by `Do_System_Discoveries_At_Star_`
(section E); the Antaran guard is settings+0xd3 (0x199caf, "Antarans attack")
and antarans+0x3c (0x1991b0) (section F).
