# Saved game: header, settings and small blocks

Part of `docs/formats/save.md` (block offsets there). Covers the header,
`settings`, `bill_savegame`, reports, messages, events, council and Antaran
blocks, and what `Load_Game_` does. All values little-endian. "File" offsets
are absolute in `SAVEn.GAM`; "+x" offsets are inside a block. "Observed" means
the ten shipped saves.

## Header

| File | Size | Type | Global | Meaning |
|---|---|---|---|---|
| 0x000 | 4 | u32 | (local) | Format number, always 0xe0. The only thing `Load_Game_` checks. |
| 0x004 | 37 | char[37] | `save_game_description[min(n,9)]` | NUL-terminated slot description. `_save_game_description` is 10 × 37 bytes (0x1996be). `Do_Autosave_` (0x13822) puts "(Auto Save)" in slot 9 and calls `Save_Game_(9)`. |
| 0x029 | 4 | s32 | `stardate` | Tenths of a stardate: 35000 = 3500.0 at the start of a game (`Init_New_Game_` 0x1249c), +1 per turn. Shown as `%d.%d` of `/10`, `%10` (`Build_Antaran_Ships_`). Turn = stardate − 35000. Observed 35001–35370. |
| 0x02d | 1 | u8 | `game_type` | 0 single player, 1 hotseat, 2 network, 3 modem or null modem (`Set_Multi_Player_Game_Type_` 0xf5691). `Get_Game_Type_From_Saved_Game_` (0x11fcc) reads this byte alone with `fseek(0x2d)`. Observed 0, and 1 in SAVE10. |
| 0x02e | 553 | — | `settings` | See below. `Read_Settings_From_Save_File_` (0x11eae) reads it alone with `fseek(0x2e)`. |
| 0x257 | 4 | u32 | (local) | RNG state, `Get_Random_Seed_()` at save time (0x1161b). On load it is given to `Set_Random_Seed_` after everything else is read (0x114c4), so the RNG continues where it stopped. |

`Do_Autosave_` saves to slot 9 when settings+0x0b is set and
`stardate % 4 == 0` (any stardate when `game_type` is 2 or 3); its call site
was not traced.

## `settings` (0x229 bytes, global 0x199bdc)

The same struct is the whole of `MOX.SET` (`Save_Game_Settings_` 0x11be4,
`Load_Game_Settings_` 0x11c39, `Write_Game_Settings_` 0x12937). A save stores
the live copy. On load only the "universe" fields (marked U) are taken from
the save; everything else stays as `MOX.SET` had it (see Load_Game_ below).
`Save_Session_Related_Settings_` (0x12030) writes bytes 0x00–0xd2 of the live
copy into `MOX.SET` and keeps the file's bytes 0xd3.. as they were.
Defaults are from `Set_Default_Game_Settings_` (0x127e1): main2_ calls it
without a MOX.SET (and `Update_Game_Settings_Version_Number_` 0x12227 when
the file cannot be reopened); it clears the struct, sets the values in the
table below, the first byte of each save description, the game name
`sprintf("MOX-%d", clock() % 10)`, `Set_Default_Modem_Settings_` (0xf46e7:
+0x24 19200, +0x28 'E', +0x29 7, +0x2d 0, +0x31 "ATZ", +0x4f "AT &K0 M1 &D2",
+0x6d "AT H0 S0=0", +0x8b "5551212", +0xb3 1; +0x20 set to 1 and back to 0)
and writes the file (`Write_Game_Settings_`: fopen "wb", 0x229 bytes,
"Unable to open mox.set" exits). An older version is reloaded, stamped 0x82
and saved. `TestDefaultSettingsFile` compares the port's file with the
original's (build/oracle/moxset) but the clock digit.

### Game options (popup toggles, bytes 0x00–0x0d)

`Update_Game_Settings_` (0x7f14c) and `Set_Current_Game_Option_Flags_`
(0x7efef) copy these to/from `game_popup_fields` +0x90.., one word each.
Labels come from `HESTRNGS.LBX` through `Toggle_Game_Option_` (0x8a216),
`H_Message_(0xa6..)`. All are 0/1.

| + | Def | Name | Evidence / use | Observed |
|---|---|---|---|---|
| 0x00 | 1 | End Of Turn Summary | `Has_Msg_` shows the summary only when set | 0 |
| 0x01 | 1 | End Of Turn Wait | read only when `game_type == 0` (`Set_Current_Game_Option_Flags_`), `Do_Next_Turn_` | 1 |
| 0x02 | 1 | U: Random events | `Determine_Event_` 0x2233e (`== 1` enables); new-game button `random_events_button_variable` | 1 |
| 0x03 | 0 | Enemy Moves | toggle case 2; community patch 1.51: red lines of all enemy fleets shown (`Do_Ship_Destination_Lines_`) | 0 |
| 0x04 | 0 | Expanding Help | popups (`Do_Fleet_Popup_`, `Fade_Into_Main_Screen_`, ...) | 0 |
| 0x05 | 1 | Auto Select Ships | `Draw_Fleet_Movement_Box_` | 1 |
| 0x06 | 1 | Animations | council, diplomacy, events, colonisation, science screens | 1 |
| 0x07 | 0 | Auto Select Colony | turning on calls `Set_All_Colony_Auto_Select_Flags_` | 0 |
| 0x08 | 1 | Show Relocation Lines | `Draw_Relocation_Links_` | 1 |
| 0x09 | 1 | Show GNN Report | `Check_For_Event_`, `Setup_Next_Event_`, `Reports_Screen_` | 0 |
| 0x0a | 0 | Auto Delete Trade Goods / Housing | `Evaluate_Input_` | 1 |
| 0x0b | 1 | Auto Save Game | `Do_Autosave_` | 1 |
| 0x0c | 0 | Show Only Serious Turn Summary Reports | `Has_Msg_` (filters message types) | 0 |
| 0x0d | 0 | Ship initiative (uncertain name) | copied to `_combat_init` at `Tactical_Combat_` 0x47aa4; popup field +0xa8; used throughout combat turn order | 0 |
| 0x0e, 0x0f | 0 | unknown | only zeroed by defaults | 0 |
| 0x10 | — | unknown | no reference | 0 |

### Sound, slots, multiplayer (0x11–0xb3)

| + | Size | Def | Meaning | Evidence | Observed |
|---|---|---|---|---|---|
| 0x11 | 1 | 1 | Sound effects on | `Set_Sound_Volume_To_Settings_` 0x7a06c | 1 or 0 |
| 0x12 | 1 | 50 | Sound effects volume (0–100) | same; popup shows `v*155/100` | 0x18, 0 |
| 0x13 | 1 | 1 | Music on | same (`Stream_On/Off`) | 1, 0 |
| 0x14 | 1 | 50 | Music volume, clamped to 100 | same | 0x52, 0 |
| 0x15 | 1 | 0xff | Last save slot used (0xff none) | `Do_Load/Save_Game_Popup_`, `Auto_Load_Last_Saved_Game_` | 0–8, SAVE10: 1 |
| 0x16 | 1 | 0 | MP connection: 1 network, 2 modem, 3 null modem, 4 hotseat | `Multi_Player_Screen_` 0xf4ecf.., `Set_Multi_Player_Game_Type_` | 0, SAVE10: 4 |
| 0x17 | 9 | "MOX-%d" | MP game name, NUL-terminated, max 8 chars (`Change_MP_Game_Name_`); an empty name becomes "Moo II " + a random letter at +0x1d | `Set_Default_Game_Settings_` uses `clock()%10` | "MOX-0" |
| 0x20 | 17 | | Serial block copied to `_net` (0x1aaec4) by `Copy_Modem_Defaults_To_Net_Struct_` (0xf45db): | `Set_Default_Modem_Settings_` 0xf46e7 | |
| 0x20 | 4 | 0 | port index (clamped 0..7) | | 0 |
| 0x24 | 4 | 19200 | baud (1200..38400) | | 19200 |
| 0x28 | 1 | 'E' | 'E' or 'O' (meaning uncertain) | | 'E' |
| 0x29 | 4 | 7 | unknown | | 7 |
| 0x2d | 4 | 0 | unknown | | 0 |
| 0x31 | 30 | "ATZ" | modem string 1 (init) | copied with +0x4f.. to 0x1aaed5 (0x83 bytes) | defaults |
| 0x4f | 30 | "AT &K0 M1 &D2" | modem string 2 | | |
| 0x6d | 30 | "AT H0 S0=0" | modem string 3 | | |
| 0x8b | 40 | "5551212" | phone number | | |
| 0xb3 | 1 | 1 | unknown modem flag (tone dial?) | | 1 |

### Combat display flags (0xb4–0xb8)

Copied in and out of the combat globals by `Tactical_Combat_` (0x47a2b,
0x48e8b), so they persist between combats.

| + | Def | Combat global | Observed |
|---|---|---|---|
| 0xb4 | 0 | `_speedx2_flag` | 0 |
| 0xb5 | 0 | `_draw_legal_boxes_flag` | 1 |
| 0xb6 | 1 | `_msl_impact_warning_flag` (and `_saved_msl_warning`) | 1 |
| 0xb7 | 0 | `_shields_flag` | 1 |
| 0xb8 | 0 | `_grid_flag` | 1 |

0xb9–0xd1: no reference anywhere; zero in all saves.

### Language and universe (0xd2–0xe3)

New-game buttons go through `Temps_To_Globals_` (0xcd361) /
`Globals_To_Temps_` (0xcd3bc). Value names are `ESTRINGS.LBX` entries in
button order (`Load_New_Game_Strings_` 0xcc81c).

| + | Size | Def | Meaning | Values | Observed |
|---|---|---|---|---|---|
| 0xd2 | 1 | | Language (session) | 0 English, 1 German, 2 French, 3 Spanish, 4 Italian (>4 forced to 0); picks message strings in `Load_Game_` 0x10efb and `Load_H_Strings_` | 0 |
| 0xd3 | 1 | 1 | U: Antarans attack | `Next_Turn_Calc_` 0x136d1 runs `Antaran_Invasion_Check_` when set; button `xenon_attacks_button_variable` | 1 |
| 0xd4 | 1 | 0 | U: Difficulty | 0 Tutor, 1 Easy, 2 Average, 3 Hard, 4 Impossible | 1, 3, 4 |
| 0xd5 | 1 | 5 | U: Number of players (incl. human) | 2..8 (`opp_button + 2`) | 3, 5, 8 |
| 0xd6 | 1 | 1 | U: Galaxy size | 0 Small (20 stars), 1 Medium (36), 2 Large (54), 3 Huge (72) — `Galaxy_Size_From_N_Stars_` 0x798d2 | 0, 3 |
| 0xd7 | 1 | 1 | U: Galaxy age | 0 Mineral Rich, 1 Average, 2 Organic Rich (`Enforce_Mineral_Richness_`, `Enforce_Climate_`) | 1 |
| 0xd8 | 1 | 1 | U: Combat | 1 Strategic, 0 Tactical (`Newgame_Screen_` passes tactical = (+0xd8 == 0); `Strategic_Combat_`, `Update_Strategic_Ship_` and the strategic hit totals when 1; `Do_1_Combat_` plays the Orion flic itself only when 0) | 0 |
| 0xd9 | 1 | 1 | U: Tech level | 0 Pre Warp, 1 Average, 2 Advanced (`Init_New_Game_` 0x12575 adds advanced colonies when 2; `Antaran_Delay_` 200/100/0 turns) | 0, 1 |
| 0xda | 4 | | U: Universe seed | RNG state just before `Universe_Generation_` (`Init_New_Game_` 0x1250f), printed as "Seed: %ld" | random |
| 0xde | 2 | 2 | unknown, set only by defaults | | 2 |
| 0xe0 | 2 | 1 | unknown, set only by defaults | | 1 |
| 0xe2 | 2 | 0x82 | `MOX.SET` version; `main2_` 0x100cf rewrites the file via `Update_Game_Settings_Version_Number_` if different | | 0x82 |

0xe4–0x228: no reference; zero in all saves.

## `bill_savegame` (12 bytes, global 0x17d634, in `.data`)

History-graph state (`Bill_Init_` 0x102037, `Record_History_` 0x10208a,
`Get_History_Params_` 0x1029d1). Four per-player 350-byte rings live in the
player record (+0xb9b, +0xcf9, +0x8df, +0xa3d); this block holds their
cursor and divisors.

| + | Type | Init | Meaning | Observed |
|---|---|---|---|---|
| 0x0 | u16 | 2 | unknown; never read except copied | 2 |
| 0x2 | u16 | 0 | ring cursor, `(turn) mod 350` | e.g. 12 at turn 362 |
| 0x4 | u16 | 1 | divisor for ring +0xb9b (from player +0xa6) | 1–2 |
| 0x6 | u16 | 1 | divisor for ring +0xcf9 (building costs) | 3–269 |
| 0x8 | u16 | 1 | divisor for ring +0x8df (ship strength) | 1–12 |
| 0xa | u16 | 1 | divisor for ring +0xa3d (player +0x224) | 1–285 |

A divisor grows until every value/divisor ≤ 250; when it changes the whole
ring is rescaled. 0 is replaced by 1 before use.

## Reports

| File | Size | Global | Meaning |
|---|---|---|---|
| 0x31c10 | 2 | `n_reports` s16 | Records in use, ≤ 50 (`Add_Report_` 0xfd846). |
| 0x31c12 | 950 | `reports` | 50 × 19-byte spy reports. Records past `n_reports` are stale and saved verbatim. |
| 0x31fc8 | 1 | `done_player_reports` | Bitmask by player: reports shown (`Display_Report_Aux_` 0xfe64e). Observed 1, 3. |
| 0x31fc9 | 8 | `something_interesting_happened` | One byte per player, set to 1 on contact, combat, council, ship arrival; `Do_Begin_Of_Turn_` 0x8adcd checks the local player's. |
| 0x31fd1 | 9 | `done_colonize_check` | 72-bit field by star index (bit `i&7` of byte `i>>3`, `Test_Bit_Field_`); star already offered for colonisation this turn (`Colonization_`). |

`Initialize_Reports_` (0xfd81c) clears all of these plus `antarans`+0x41.

Report record (19 bytes), from `Has_Report_` (0xfe359) and `sprintf_report_`
(0xfd69f):

| + | Type | Meaning |
|---|---|---|
| 0x00 | u8 | 0 none, 1 our spy did it, 2 enemy spy did it to us |
| 0x01 | s16 | Player the report is for; −1 once shown |
| 0x03 | u8 | 0 stole tech, 1 sabotaged building |
| 0x04 | s8 / s16 | type 1: player framed (−1 none); type 2: s16 thief player |
| 0x05 | u8 | type 1: victim player |
| 0x06 | s16 / u8,u8 | steal: tech application index; sabotage: planet (u8 at +6), building (u8 at +7) |
| 0x08 | 11 | never read; the caller's stack residue (identical across records in a save) |

## Messages (turn summary)

| File | Size | Global | Meaning |
|---|---|---|---|
| 0x3211e | 1800 | `msgs` | 100 × 18-byte messages, split into per-player windows |
| 0x32826 | 8 | `first_msg` u8[8] | First slot of each player's window |
| 0x3282e | 8 | `max_msgs` u8[8] | Window size |
| 0x32836 | 8 | `n_msgs` u8[8] | Messages in use |
| 0x3283e | 1 | `displayed_msgs` | Bitmask by player: summary shown (`Has_Msg_` 0xfe0ea) |
| 0x3283f | 1 | `displayed_gnn` | Bitmask by player: GNN shown (`Has_GNN_` 0xfe514) |

`Allocate_Msg_Slots_` (0xeeb3a), run by `Init_New_Game_` and every
`Next_Turn_Calc_`, zeroes all three arrays and, for each player with
player+0x24 == 0 (k-th such player of `alive`), sets
`first = k*100/alive`, `max = (k+1)*100/alive − first`. Observed: 8 alive →
0,12,25,…; dead players 0/0. `Add_Msg_` (0xef629) appends; when full it
replaces the message with the lowest byte +1 if the new one's +1 is higher,
so byte +0 is the message type and +1 its priority. The other 16 bytes
depend on the type (e.g. 0x11 diplomacy: +2 kind, +3 and +4 players,
`Add_Diplomacy_Msg_`) and belong to the message spec. Slots outside
`n_msgs` keep stale data.

## Events

| File | Size | Global | Meaning |
|---|---|---|---|
| 0x31fda | 324 | `event_data` | 36 × 9 bytes, indexed by event type (`Init_Events_` 0x201a4) |
| 0x3284a | 4 | `last_event_year` s32 | Stardate of the last random event (initially the start stardate) |
| 0x3284e | 2 | `event_delay_count` s16 | Turns since the last event, counts up to 5; no event while < 5 |

Event record (`Init_Events_`, `Determine_Event_` 0x2230a):

| + | Type | Init | Meaning |
|---|---|---|---|
| 0 | s8 | −1 | Player affected (−1 none / galaxy-wide) |
| 1 | u8 | 0 | State: 0 inactive, 1 just started, 3 ongoing, 4/5/6 news pending (`Check_For_Event_Message_` 0x2023f). 2 not seen. |
| 2 | s8 | 0 | Counter/duration (e.g. `10 + rnd(5) − difficulty`) |
| 3 | s16 | −1 | Target: colony, star, planet, ship or player, by type |
| 5 | s16 | −1 | Strength/amount |
| 7 | s16 | −1 | Initial strength (copy of +5) |

Random events are types 0–28 (`Random(29) − 1`). Types identified by their
routines: 0 derelict, 1 climate change, 2 comet, 9 hyperspace flux, 10
industrial accident, 11 mineral depletion, 12 mineral deposit, 13 never
chosen, 14 pirates, 16 plague, 17 population boom, 25 space anomaly, 26 warp
beast, 28 wormhole. 29–35 are not random: race eliminated, planet warning,
general status, Orion discovered, Antarans defeated, surrender, rebel
assimilation (`Event_*_` routines touch +29*9.. +35*9). Event chance per turn:
`stardate − last_event_year`, scaled by difficulty (×1/2, ×2/3, ×3/4, ×4/5,
×5/6 for 0..4), against `Random(512)`, only from turn 50 and when settings+2
is 1.

## Council

| File | Size | Global | Meaning | Observed |
|---|---|---|---|---|
| 0x32840 | 2 | `COUNCIL_FLAG` s16 | 0 never met, 1 met without a winner, 2 the local player won, 3 another player won (`Main_Council_Screen_` 0x152ba, `Council_Finished_Voting_` 0x15e37) | 0, 1 |
| 0x32842 | 2 | `next_council_meeting_turn` s16 | Turn (stardate − 35000) of the next meeting; 25 initially (`Init_Diplomatic_Relations_`), then meeting turn + 25 | 25, 213–372 |

`Check_For_Council_Meeting_` (0x168af): meets when turn > 24, (turn ≥ next
or flag == 0), more than two players alive and at least half of the stars
have star+0x14 ≠ −1.

## Antarans and save lock

| File | Size | Global | Meaning |
|---|---|---|---|
| 0x32844 | 2 | `antaran_resource_level` | Saved and loaded, read nowhere else (dead). 0 in all saves. |
| 0x32846 | 2 | `chance_for_antaran_invasion` | Same: dead, always 0. |
| 0x32848 | 2 | `disallow_saves_code` | Non-zero disables saving ("Save Game function disabled", `Do_Main_Game_Popup_` 0x7df18; F-key save; main menu). Only ever set to 0 (`Check_Command_Line_Parameters_`). 0 in all saves. |
| 0x32850 | 66 | `antarans` | See below; zeroed by `Init_New_Game_` |

`antarans` (0x199174):

| + | Type | Meaning | Evidence |
|---|---|---|---|
| 0x00 | s16 | Attack pressure: +1 per turn when undeployed offensive ships exist (skipped while total forces exceed 4× the offensive fleet cost); attack if `Random(200) − 1 < it + 1`, reset to 0 on attack | `Antarans_Should_Attack_` 0x63edd, `Antaran_Invasion_Check_` 0x63d92 |
| 0x02 | s16 | Offensive resources | `Increment_Antaran_Resource_Level_` 0x645ec, `Build_Antaran_Offensive_Ships_` |
| 0x04 | s16 | Defensive resources (its share goes to +2 once the defence is at maximum) | same |
| 0x06 | s16[9] | Defensive ships by size | `Antaran_Defensive_Force_`, `Load_Antaran_Defense_Fleet_` |
| 0x18 | s16[9] | Offensive ships by size | `Antaran_Offensive_Force_` |
| 0x2a | s16[9] | Offensive ships deployed by size | `Next_Antaran_Ship_Size_`, `Restock_Antaran_Ship_` |
| 0x3c | u8 | Non-zero skips the Antaran turn; never written by code (uncertain) | `Next_Turn_Calc_` 0x136da |
| 0x3d | s32 | Stardate of the last attack | `Antaran_Invasion_Check_` |
| 0x41 | u8 | Bitmask by player: attack announcement shown | `Antarans_` 0xfe552 |

Resources grow every 25 turns after `Antaran_Delay_`, by 100% (150% Hard,
200% Impossible) of `(turn − delay)/25`. Observed: +0x3d 35191–35348, ship
counts 0–3. Not saved although they change during play: `_antaran_ship_cost`
(cut to 90% when the first offensive titan is built on Hard+),
`_n_max_antaran_def_ships`, `_offensive_titan_ship_built`.

## Load_Game_ (0x10e2f)

Reads exactly the blocks `Save_Game_` writes, in the same order and sizes
(colony, planet, star, leader, player and ship arrays one record per `fread`,
which gives the same bytes). Then:

- Validation: only the format number. If ≠ 0xe0 it shows "SAVE%d.GAM is
  invalid!" in the current language and returns 0 without closing the file
  or restoring the mouse list. A missing file prints "File read error" and
  waits for a key. No `fread` result, size or count is checked.
- `settings` is read into a local buffer. Only these go live: +0x02, +0xd3–0xd9
  and the dword +0xda (the same set as `Copy_Over_Game_Universe_Settings_`
  0x120fa). Options, sound, language, slots and modem settings in the save
  are ignored.
- The random seed goes to a local and is applied after `Set_Defaults_`.
- `NUM_NEBULAS` is forced to 0 if `nebula_screen_seg` is null (no memory
  for nebulas).
- The 1000-byte tail is read into a local and discarded.

After `fclose`, in order:

1. `Set_Defaults_` (0x1279f): `cur_map_x = cur_map_y = 0` (the loaded values
   are lost), `current_screen = 0x27`, `council_winner = −1`;
   `Harold_Defaults_` (0x792c3) resets main-screen UI state, including
   `star_bg_index = 2`, `cur_map_scale = max_map_scale`, `saved_boxes`, a
   field of each `moveable_box`, hotseat boxes when `game_type == 1`;
   `Bill_Default_` resets the paragraph/text system; `Ken_Init_` and
   `Eric_Defaults_` are empty.
2. `Set_Random_Seed_(saved seed)`.
3. `Cache_Player_Scientist_Anim_`; `Load_Nebula_Pictures_` if there are
   nebulas; `Update_Ship_Icons_In_Between_Turns_`; `Reload_Fleet_Box_Data_` if
   moveable box 2 is selected; `Rebuild_WFI_List_`.
4. The universe settings copy (above).
5. `Confirm_Planet_And_Star_Index_Link_("loading a game")`,
   `Update_Star_Owners_` (recomputes star+0x14 for every star),
   `Load_Leader_Titles_`.
6. Migration, for all 8 player records: if the u16 at player+0x2e is non-zero
   it moves into the dword at player+0x1eb and +0x2e becomes 0.
7. For each player p < `NUM_PLAYERS`: bytes player[p]+0x897+j = 0xff for
   j < `NUM_PLAYERS`.
8. `Restore_Mouse_List_`, `Release_All_Blocks_(screen_seg)`; returns 1.

`Save_Game_` (0x1160b) ends with `Save_Game_Settings_`, which writes
`MOX.SET` and closes the file twice (harmless). It refuses nothing itself;
the caller checks `disallow_saves_code`.

## Open questions

- settings +0x0d: is it the 1.3 "ship initiative" option? `_combat_init` is
  used all over combat turn order, but the popup label has not been read.
- settings +0x0e, +0x0f, +0x10, +0x29, +0x2d, +0xb3, +0xde, +0xe0 and
  `bill_savegame`+0: purpose unknown.
- `antarans`+0x3c: nothing writes it, but it blocks Antaran turns.
- Event types 3–8, 15, 18–24, 27 not yet named; message record payloads not
  decoded.
