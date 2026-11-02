# Tactical combat: map of Tactical_Combat_ (0x47939)

The combat screen that `Russ_Combat_` (0xe7343) enters when a human fights
and settings byte 0xd8 is 0 (`docs/turn/combat.md`). ELF addresses (Watcom +
0x10000). Status: **the logic is ported** (`internal/game/turn_tacloop.go`,
`turn_tacai.go`, `turn_tacmove.go`, `turn_tacdamage.go`, plus the engine of
`Tactical_Bombardment_` in `turn_tactical.go`, `turn_tacfire.go`), with the
screen behind `tacticalUI`; the screen itself is not. See "Port" below.

## Size

From `Tactical_Combat_` about 1440 routines are reachable through `call`
(engine, popups, flics, ground combat included). By module (routines /
own instructions in the reach):

| Module | n | insn | Role |
|---|---|---|---|
| COMBINIT | 32 | 6.0k | setup, loading, deploy, end-of-turn bookkeeping, end of combat |
| CMBTAI | 40 | 4.9k | the AI and "Auto" mode: player mode, targets, movement, retreat, boarding |
| COMBAT1 | 22 | 4.6k | `Do_Combat_Turn_` (the input loop), turn order, fields, seeking missiles |
| CMBTSPEC | 23 | 4.0k | special weapons and their effects (mixed logic and drawing) |
| CMBTFIR2 | 48 | 4.0k | bonuses, internal damage, boarding/capture, defensive fire, repairs |
| CMBTFIRE | 28 | 3.3k | firing, beam/bomb/fighter damage, ship destruction |
| COMBAT | 20 | 3.3k | strategic resolver (only `Apply_Combat_Artemis_Net_` from here) |
| CMBTMIS | 27 | 3.2k | missiles and fighters |
| CMBTMOV1 | 13 | 1.7k | movement, rotation, teleport |
| CMBTDRW1 | 42 | 9.6k | drawing (UI) |
| BEAMS | 41 | 4.6k | beam drawing and sound (UI) |
| COLGCBT, COLDRAW, COLONY | | ~5k | ground combat screen of boarding/capture (UI) |

The logic to port is therefore roughly 35k instructions in the nine CMBT*/
COMB* modules (minus the parts already ported for bombardment).

## Tactical_Combat_(attList, nAtt, defList, nDef, star, colony, canRetreat)

Pseudocode read from REA (Ghidra) and the assembly; call order:

1. star == `NUM_STARS` (Antares) → `Play_Dimensional_Portal_Flic_` (UI).
2. Globals reset: `_combat_turn` = 0, `_bio_weapon_flag` = 0,
   `_super_fast_flag` = 0, `_real_attack_flag` = 0, `_take_control_flag` = 0,
   `_apply_damage_to_planet` = 1 (screen on; `Tactical_Bombardment_` sets 0),
   `_ai_retreat_flag` = canRetreat, `_cmbt_colony_id` = colony,
   `_combat_total_ship_count` = 1 (record 0 is the planet), option flags from
   settings 0x199c90..0x199c94 (speed ×2, legal boxes, missile warning,
   shields, grid) and `_combat_init` from settings+0xd (0x199be9; non-zero =
   initiative order, see below), `_net_flag` = 0.
3. `_combat_data` = 0x100c2 bytes in screen_seg, cleared (0xd2 records of
   0x139). Attacker = owner of attList[0]; defender = 8 at Antares, else the
   colony owner or the owner of defList[0]. Pop before = colony+0xa.
4. `_in_nebula_flag` = `Point_Is_In_Some_Nebula_(star x, y)`.
5. `qsort_(attList, Qsort_Ship_Array_By_Size_)`; each attacker ship with
   design type 0 (ship+0x11 == 0), at most 99: `Load_Combat_Ship_` into the
   next record. While loading, the weapon ids select effect palettes (0x23 or
   0x2d plasma web, 0x25 black hole generator, 0x26 stellar converter, 0x27
   tractor) and a ship without special 0x17 sets `_real_attack_flag`.
6. Defender < 8 and not Antares and `Player_Has_Artemis_Net_` →
   `Apply_Combat_Artemis_Net_(0)` (R!); all attackers destroyed → message box,
   result −1, no screen.
7. No defender combat ship (all type ≠ 0) and (no colony or
   `Colony_Has_Defenses_` false) and not Antares → result 1 at once, no
   screen.
8. Otherwise the screen: palettes, LBX pictures (COMBAT.LBX, CMBTMISL.LBX
   0..0x6f), the mouse grid (0x2928 bytes, 32×18 squares of 20 px), the
   missile block (`_missile`, 300 × 0x1a, cleared), fighter weapons per owner
   (`Best_Point_Defense_Weapon_`, `Best_Bomb_Weapon_` into 0x199254/0x199242),
   then
   - Antares: `Load_Antaran_Defense_Fleet_` (`Load_Antaran_Star_Fortress_`,
     `Load_Combat_Antaran_Ship_`), `Load_Antaran_Planet_`;
   - else defenders sorted by size and loaded like the attackers (≤ 99 type-0
     ships), then `Load_Colony_Defense_(colony)` (→ `Load_Tactical_Colony_`,
     `Load_Combat_Satelite_` for a star base / battle station / star fortress,
     `Type_Of_Space_Station_`).
   - `Load_Combat_Ship_Palette_`, `Load_Individual_Ship_Pictures_` (UI),
     remap tables (UI), `Set_Combat_Ordnance_Values_`,
     `Init_Special_Devices_` per record, **`Deploy_Ships_`** (0x49043, with
     `Ship_Can_Deploy_At_`), `_auto_combat_flag` = 0 (attacker), 0x192b12 = 0
     (defender), `Add_Tactical_Combat_Fields_`, `Check_For_Winner_`.
9. Sides: first = attacker, second = defender; at Antares the Antarans (8)
   go first.
10. **Main loop** while `Check_For_Winner_` == 0 — one combat turn:
    - every live record (not the planet unless there is a colony):
      +0x3b = `Calc_Current_Speed_`, +0x34 = `Offensive_Combat_Bonus_(i,0)`,
      +0x36/+0x38 = `Defensive_Combat_Bonus_(i,0/1)`;
    - missiles with weapon > 0: +0x13 (travel) = speed byte (+0x16) × 20 when
      zero, +0x19 (live) = 1;
    - `_combat_init` == 0 (the shipped setting), checked in the assembly
      (DX = phase): `Do_Combat_Turn_(first, 0)`, `Check_For_Winner_`,
      `Plasma_Web_Damage_(second)`; no winner → `Do_Combat_Turn_(second, 0)`;
      `Plasma_Web_Damage_(first)`; no winner → `Do_Combat_Turn_(first, 1)` +
      `Increment_Bhg_(first)`; no winner → `Do_Combat_Turn_(second, 1)` +
      `Increment_Bhg_(second)`. Phase 1 moves only ships with special 0x22
      (the extra-turn device) and skips `Seeking_Missiles_`.
    - `_combat_init` ≠ 0: `Do_Combat_Turn_(0, 0)` over all ships in
      initiative order (`Qsort_Ship_Initiative_`), then a call into 0x1694b9
      (past the code end, `__SegmentEndPC32+0xe25`: patch code, unread) for
      both sides with 1, then `Do_Combat_Turn_(0, ?)`.
    - `_combat_turn`++, `End_Of_Turn_Bookeeping_` (`Recharge_Shields_`,
      `Repair_All_Combat_Ships_` with `Get_Fleet_Engineer_Bonus_`,
      `Repair_Combat_Ship_`).
11. `End_Of_Combat_(star, pop lost)` (0x4b184, 528 insn): writes the battle
    back to the save (`Unload_Combat_Ship_` per record, `Clear_Internal_Space_`,
    `Apply_Bio_Weapon_Penalty_` R!, captured ships, officers, colony
    buildings) — to be read in detail.
12. Monster beaten (owner > 9 with drive): `Russ_Event_Monster_Update_`.
13. UI teardown, settings 0x199c90..94 written back from the option flags
    (they are in MOX.SET), result 0x29a → 0. Returns >0 attacker won, 0
    draw, <0 defender won (`Check_For_Winner_`).

## Do_Combat_Turn_(player, phase) (0x42f7f, 1785 insn) — the input loop

1. `_player_mode[att/def]` = `Get_Player_Mode_` (CMBTAI: `Retreat_Check_`,
   `Get_Effective_Missile_Strength_`, `Get_Long_Range_Beam_Strength_`,
   `Get_Effective_Beam_Strength_`) — the AI's stance for the turn.
2. `_side_end_of_turn[0..14]` = 0. Every live record (+0x24 gone == 0,
   +0xb0 boarded == 0, not in stasis) of `player` (or all with
   `_combat_init`), restricted to special 0x22 in the special phase:
   `Init_Ship_For_Start_Of_Turn_` (retreat completion via `Retreat_Ship_`,
   capture flags, speed).
3. Turn order `_ship_turn_order` (0x1d2 entries): the eligible records in
   record order (boarded ones as −1), sorted by initiative with
   `_combat_init`, terminated by −10.
4. `Next_Ship_` picks the next record (`_cur_ship`, `_cur_ptr`), checks the
   winner, redraws.
5. Per ship while the order has entries:
   - **computer controlled** when owner > 7, or the owner is not human
     (player+0x28 ≠ 100), or **`_auto_combat` == 1** — and
     `_take_control_flag` == 0: `Do_Auto_Ship_Turn_` (CMBTAI 0x29837),
     then the ship is done.
   - else the human's input: `Get_Input` and the buttons: missiles as
     targets (Fire_Ship_ at a missile), missile warning, speed ×2, grid,
     legal boxes, shields, super fast (also toggles auto), self destruct,
     **Auto** (`_auto_button` 0x199898, radio button, hot key **"a"**
     (string 0x178e1b): toggles `_auto_combat_flag` (attacker) or 0x192b12
     (defender) into `_auto_combat`; while on, the field is replaced by a
     full-screen hidden field with an empty hot key, so a click ends it),
     weapons/specials display, wait (`w`), weapon toggles, info, scan, flee
     (`r`, 0x199896: refused with a message when an enemy dissipator exists,
     the drive is dead or speed < 1; else +0x4b = 1 and the ship is done),
     board, map, scroll at the screen edge, centre, right click (turn to
     face), end of turn, done (`d`, 0x19989a), clicks on the grid: move
     (`Move_Cost_`, `Move_Ship_`, `Ship_Done_Check_`), select own ship
     (`Set_Cur_Ship_To_`), fire at an enemy (`Fire_Ship_`), release stasis.
     Network play replays the opponent's choices from messages (the
     `_tactical_net_choice` switch, `Set_Random_Seed` from the message).
6. After the loop: +0x45 (absorber charge) = 0 for the side; phase 0 →
   `Seeking_Missiles_(player)` (moves every missile, `Resolve_Missile_`);
   mounts with +0x5c == 2 back to 1; cloak state machine (+0x40: 3 → 1, 4
   with countdown +0x41, 7 → 4 and retarget missiles); +0xb1 =
   `Max_Defensive_Fire_Range_`.

Fields (Add_Tactical_Combat_Fields_ 0x465d0) — hot keys from the strings at
0x178e18: "" (most), "[" 0x178e19 (0x1998dc), "a" auto, "s" 0x1998e6, "b"
0x1998ec, "r" flee, "w" wait, "d" done, "t" 0x178e27, "z" 0x178e29
(0x1998de, take control: only when exactly one side is human), "O" 0x178e2b,
Esc 0x178e2d.

## RNG

Direct `Random_` callers in the reach (R! in the table). Callers that save
and restore the seed around their draws: `Draw_Fighter_`,
`Draw_Fighter_Beam_`, `Beam_SFX_`, `Draw_Exploding_Plasma_Box_`,
`Create_Plasma_Line_`, `Bolt_Segment_`, `Jumble_Bitmap_`,
`Colony_Combat_Screen_`/`Draw_Colony_Combat_Screen_`,
`Make_Bldg_Array_For_Colony_`. **Not** bracketed, and drawing: 
`Draw_Generic_Beam_`, `Draw_Beam2_` (probably only under `Beam_SFX_`),
`Do_Spherical_SFX_` (13 calls; also applies damage),
`Play_Ship_Explosion_SFX_` (under `Destroy_Ship_FX_`, which also applies
`Damage_From_Ship_Exploding_`), `Vanish_Bitmap_Pixels_`,
`Draw_Gyro_Special_Effect_`, `Draw_Transporter_Bomb_Beam_`. Whether their
draws reach the seed when the screen is on must be checked against a trace
before porting: the port has to replay them without drawing.

`Do_Combat_Turn_` calls `Set_Random_Seed_` only in network play.

## Data

- Combat ship record, 0x139 bytes (`_combat_data`), record 0 = planet:
  layout in `internal/game/turn_tactical.go` (`ts*` constants); offsets seen
  in the loop besides those: +0x3d i16 movement points left, +0x41 cloak
  countdown, +0x42 tractor/0xff, +0x4b retreating, +0x5a/+0x5c weapon mount
  on/state (= `tw(k, twOn)` and `twFired`), +0xb1 defensive fire range.
- Missile record 0x1a × 300 (`_missile`), `tm*` constants.
- `_combat_grid` (word per square, 0x44 per column): 2000 = legal move,
  else record index; `_combat_mouse_grid` 0x2928 bytes (UI).
- Globals: `_cur_ship`, `_cur_ptr`, `_ship_turn_order`, `_player_mode[15]`,
  `_side_end_of_turn[15]`, `_auto_combat` 0x199900, `_auto_combat_flag`
  0x192b10, 0x192b12, `_super_fast_flag` 0x1998e0, `_take_control_flag`
  0x1998c6, `_combat_init` 0x17d853 (copied from settings), `_fleet_target`.

## Routines in the nine logic modules reachable from Tactical_Combat_

RNG: `R!` calls `Random_`, `R` below, `-` never. GO: a Go function names
the routine (ported for `Tactical_Bombardment_`, often without the ship
branches — see `docs/turn/combat.md`).

| Routine | Addr | Module | Insn | RNG | GO | Called from (in reach) |
|---|---|---|---|---|---|---|
| `Does_Ship_Have_Missiles_Incoming_` | 0x27f26 | CMBTAI | 40 | - | yes | Do_Auto_Ship_Turn_ |
| `Safe_To_Fire_Sphere_Weapon_` | 0x27f9f | CMBTAI | 134 | - | yes | Do_Auto_Ship_Turn_, Final_Fire_ |
| `Ai_Self_Destruct_Check_` | 0x28168 | CMBTAI | 307 | R! | yes | Do_Auto_Ship_Turn_ |
| `Release_Smallest_Ship_From_Stasis_` | 0x285c6 | CMBTAI | 44 | - | yes | Ai_Stasis_Check_ |
| `Ai_Stasis_Check_` | 0x2864a | CMBTAI | 51 | - | yes | Do_Auto_Ship_Turn_ |
| `Range_To_XY_` | 0x286e8 | CMBTAI | 129 | - | yes | Ai_Self_Destruct_Check_, Average_Missile_Kills_, Defensive_Fire_Check_, Do_Auto_Ship_Turn_, Move_Ship_, Range_To_Missile_... |
| `Range_To_Missile_` | 0x28836 | CMBTAI | 13 | - | yes | Fire_Beam_Weapon_, Pre_Weapon_Specials_, Retarget_Missile_, Weapon_In_Range_ |
| `Range_To_Ship_` | 0x2885a | CMBTAI | 126 | - | yes | Ai_Self_Destruct_Check_, Boarding_Action_Type_, Do_Auto_Ship_Turn_, Energy_Absorber_Attack_, Fire_Beam_Weapon_, Fire_Special_Weapon_... |
| `Expected_Weapon_Damage_` | 0x289c4 | CMBTAI | 336 | - | yes | Combat_Ship_Class_, Defensive_Fire_Check_, Retreat_Check_, Target_Missile_Value_, Target_Ship_Value_ |
| `Expected_Missile_Damage_` | 0x28eae | CMBTAI | 166 | - | yes | Expected_Weapon_Damage_, Missile_Overkill_Check_ |
| `Expected_Beam_Bomb_Damage_` | 0x290da | CMBTAI | 220 | - | yes | Expected_Weapon_Damage_ |
| `Missile_Overkill_Check_` | 0x293d8 | CMBTAI | 54 | - | yes | Target_Ship_Value_ |
| `Check_For_Launched_Fighters_` | 0x29480 | CMBTAI | 39 | - | yes | Is_Ship_Out_Of_Ammo_ |
| `Is_Ship_Out_Of_Ammo_` | 0x294df | CMBTAI | 74 | - | yes | Do_Auto_Ship_Turn_ |
| `Is_Ship_Targeted_By_Missile_` | 0x295d0 | CMBTAI | 25 | - | yes | Do_Auto_Ship_Turn_ |
| `Toggle_Auto_Combat_Flags_` | 0x29616 | CMBTAI | 72 | R | yes | Do_Auto_Ship_Turn_, Do_Combat_Turn_ |
| `Crystal_Control_` | 0x29790 | CMBTAI | 49 | R |  | Do_Auto_Ship_Turn_ |
| `Do_Auto_Ship_Turn_` | 0x29837 | CMBTAI | 419 | R | yes | Do_Combat_Turn_ |
| `Target_Missile_Value_` | 0x29f1b | CMBTAI | 152 | - | yes | Get_Current_Missile_Target_ |
| `Target_Ship_Value_` | 0x2a239 | CMBTAI | 169 | - | yes | Get_Current_Ship_Target_, Select_Ship_To_Target_ |
| `Select_Ship_To_Target_` | 0x2a46a | CMBTAI | 40 | - | yes | Choose_Target_ |
| `Choose_Target_` | 0x2a4e8 | CMBTAI | 34 | - | yes | Do_Auto_Ship_Turn_ |
| `Combat_Ship_Class_` | 0x2a545 | CMBTAI | 173 | - | yes | Do_Auto_Ship_Turn_ |
| `Evaluate_Possible_Move_` | 0x2a77d | CMBTAI | 54 | - | yes | Auto_Move_Ship_ |
| `Auto_Move_Ship_` | 0x2a9a5 | CMBTAI | 220 | R! | yes | Do_Auto_Ship_Turn_ |
| `Get_Current_Ship_Target_` | 0x2acf9 | CMBTAI | 134 | - | yes | Get_Current_Target_ |
| `Will_Missile_Do_Damage_` | 0x2aed2 | CMBTAI | 59 | - | yes | Does_Ship_Have_Missiles_Incoming_, Final_Fire_, Get_Current_Missile_Target_, Rotate_Ship_Until_Impact_, Target_Missile_Value_, Test_Move_Until_Impact_ |
| `Get_Current_Missile_Target_` | 0x2afa9 | CMBTAI | 74 | - | yes | Get_Current_Target_ |
| `Get_Current_Target_` | 0x2b09e | CMBTAI | 78 | - | yes | Do_Auto_Ship_Turn_ |
| `Convert_Ordinal_To_Msl_Type_` | 0x2b191 | CMBTAI | 45 | - | yes | Get_Effective_Missile_Strength_ |
| `Get_Effective_Missile_Strength_` | 0x2b1fd | CMBTAI | 392 | R | yes | Get_Player_Mode_ |
| `Average_Missile_Kills_` | 0x2b7cc | CMBTAI | 160 | - | yes | Get_Effective_Missile_Strength_ |
| `Get_Effective_Weapon_Damage_` | 0x2b9e3 | CMBTAI | 44 | - | yes | Get_Effective_Beam_Strength_, Get_Long_Range_Beam_Strength_ |
| `Get_Effective_Beam_Strength_` | 0x2ba5e | CMBTAI | 58 | R | yes | Get_Effective_Missile_Strength_, Get_Player_Mode_ |
| `Get_Total_Hits_` | 0x2bb06 | CMBTAI | 49 | - | yes | Missile_Overkill_Check_, Retreat_Check_, Target_Ship_Value_ |
| `Retreat_Check_` | 0x2bba1 | CMBTAI | 162 | - | yes | Get_Player_Mode_ |
| `Get_Long_Range_Beam_Strength_` | 0x2be25 | CMBTAI | 90 | R | yes | Get_Player_Mode_ |
| `Resolve_Ai_Boarding_` | 0x2bf73 | CMBTAI | 121 | R |  | Do_Auto_Ship_Turn_ |
| `Boarding_Action_Type_` | 0x2c129 | CMBTAI | 216 | - | yes | Do_Auto_Ship_Turn_ |
| `Get_Player_Mode_` | 0x2c43a | CMBTAI | 86 | R | yes | Do_Combat_Turn_ |
| `Apply_Internal_Damage_` | 0x35251 | CMBTFIR2 | 383 | R! | yes | Apply_Ship_Damage_, Resolve_Raid_ |
| `Repair_Combat_Ship_` | 0x35821 | CMBTFIR2 | 276 | - | yes | Repair_All_Combat_Ships_ |
| `Clear_Internal_Space_` | 0x35c3a | CMBTFIR2 | 13 | - | yes | End_Of_Combat_ |
| `Computer_Bonus_` | 0x35c7f | CMBTFIR2 | 15 | - | yes | Offensive_Combat_Bonus_ |
| `Crew_Boarding_Combat_Bonus_` | 0x35cad | CMBTFIR2 | 8 | - | yes | Ai_Self_Destruct_Check_, Boarding_Action_Type_, Resolve_Raid_ |
| `Defensive_Combat_Bonus_` | 0x35d0d | CMBTFIR2 | 73 | - | yes | Tactical_Bombardment_, Tactical_Combat_ |
| `Get_Fleet_Engineer_Bonus_` | 0x35e0c | CMBTFIR2 | 35 | - | yes | End_Of_Combat_, Repair_All_Combat_Ships_ |
| `Fighter_Pilot_Bonus_` | 0x35e6a | CMBTFIR2 | 31 | - | yes | Get_Fleet_Pilot_Bonus_ |
| `Get_Fleet_Pilot_Bonus_` | 0x35eae | CMBTFIR2 | 30 | - | yes | Fighter_Ocv_, Missile_Dcv_ |
| `Set_Combat_Ordnance_Values_` | 0x35f0a | CMBTFIR2 | 15 | - | yes | Tactical_Bombardment_, Tactical_Combat_ |
| `Leader_Ordnance_Bonus_` | 0x35f4d | CMBTFIR2 | 17 | - | yes | Get_Fleet_Ordnance_Bonus_, Get_Leader_Combat_Bonuses_ |
| `Get_Fleet_Ordnance_Bonus_` | 0x35f7e | CMBTFIR2 | 30 | - | yes | Set_Combat_Ordnance_Values_ |
| `Get_Fleet_Commando_Bonus_` | 0x35fda | CMBTFIR2 | 30 | - | yes | Ai_Self_Destruct_Check_, Boarding_Action_Type_, Resolve_Capture_, Resolve_Raid_ |
| `Commando_Bonus_` | 0x36036 | CMBTFIR2 | 23 | - | yes | Get_Fleet_Commando_Bonus_ |
| `Engineer_Bonus_` | 0x3607a | CMBTFIR2 | 18 | - | yes | Get_Fleet_Engineer_Bonus_, Get_Leader_Combat_Bonuses_ |
| `Get_Fleet_Security_Bonus_` | 0x360aa | CMBTFIR2 | 30 | - | yes | Ai_Self_Destruct_Check_, Boarding_Action_Type_, Resolve_Capture_, Resolve_Raid_ |
| `Security_Bonus_` | 0x36106 | CMBTFIR2 | 18 | - | yes | Get_Fleet_Security_Bonus_ |
| `Fleet_Defensive_Bonus_` | 0x36139 | CMBTFIR2 | 99 | - | yes | Defensive_Combat_Bonus_ |
| `Fleet_Offensive_Bonus_` | 0x36266 | CMBTFIR2 | 92 | - | yes | Fighter_Ocv_, Offensive_Combat_Bonus_ |
| `Init_Internal_Space_` | 0x36470 | CMBTFIR2 | 150 | - | yes | Load_Antaran_Planet_, Load_Antaran_Star_Fortress_, Load_Combat_Satelite_, Load_Combat_Ship_, Load_Display_Combat_Ship_, Load_Small_Antaran_Combat_Ship_... |
| `Kill_Crew_` | 0x366af | CMBTFIR2 | 14 | - | yes | Apply_Ship_Damage_ |
| `Offensive_Combat_Bonus_` | 0x366dd | CMBTFIR2 | 44 | - | yes | Tactical_Bombardment_, Tactical_Combat_ |
| `Helmsman_Bonus_` | 0x3677d | CMBTFIR2 | 17 | - | yes | Fleet_Defensive_Bonus_, Get_Leader_Combat_Bonuses_, Get_Ship_Combat_Bonuses_ |
| `Weaponry_Bonus_` | 0x367ae | CMBTFIR2 | 17 | - | yes | Fleet_Offensive_Bonus_, Get_Leader_Combat_Bonuses_, Get_Ship_Combat_Bonuses_ |
| `Galactic_Lore_Bonus_` | 0x367df | CMBTFIR2 | 17 | - | yes | Fleet_Offensive_Bonus_, Get_Leader_Combat_Bonuses_ |
| `Select_Internal_` | 0x36810 | CMBTFIR2 | 162 | R! | yes | Apply_Ship_Damage_ |
| `Ship_Specials_Defensive_Bonus_` | 0x36a63 | CMBTFIR2 | 75 | - | yes | Defensive_Combat_Bonus_ |
| `Total_Internal_Space_` | 0x36b64 | CMBTFIR2 | 40 | - | yes | Draw_Grey_Damage_Bars_, Get_Total_Hits_, Select_Internal_, Select_Raid_Internal_ |
| `Max_Total_Internal_Space_` | 0x36be2 | CMBTFIR2 | 40 | - | yes | Draw_Grey_Damage_Bars_, Repair_Combat_Ship_ |
| `Final_Fire_` | 0x36c9c | CMBTFIR2 | 174 | R | yes | Resolve_Missile_ |
| `Defensive_Fire_Check_` | 0x36ed1 | CMBTFIR2 | 188 | R! | yes | Move_Ship_, Rotate_Ship_ |
| `Max_Defensive_Fire_Range_` | 0x37173 | CMBTFIR2 | 72 | - | yes | Defensive_Fire_Check_, Do_Combat_Turn_ |
| `Get_Combat_Ship_Special_Id_` | 0x37254 | CMBTFIR2 | 38 | - |  | Resolve_Raid_ |
| `Init_Boarding_Action_` | 0x372bd | CMBTFIR2 | 27 | R |  | Initiate_Boarding_Action_ |
| `Resolve_Raid_` | 0x37308 | CMBTFIR2 | 846 | R! |  | Do_Combat_Turn_, Initiate_Boarding_Action_, Resolve_Ai_Boarding_ |
| `Resolve_Capture_` | 0x37da8 | CMBTFIR2 | 402 | R |  | Do_Combat_Turn_, Initiate_Boarding_Action_, Resolve_Ai_Boarding_, Resolve_Missile_ |
| `Capture_Ship_` | 0x38312 | CMBTFIR2 | 101 | R! |  | Crystal_Control_, Resolve_Capture_ |
| `Select_Raid_Internal_` | 0x384a8 | CMBTFIR2 | 142 | R! |  | Resolve_Raid_ |
| `Damage_From_Ship_Exploding_` | 0x38656 | CMBTFIR2 | 18 | - | yes | Destroy_Ship_FX_ |
| `Apply_Combat_Artemis_Net_` | 0x3868f | CMBTFIR2 | 118 | R! | yes | Strategic_Combat_, Tactical_Combat_ |
| `Combat_Ship_Done_` | 0x3883a | CMBTFIR2 | 10 | - | UI | Draw_Ship_ |
| `Set_Combat_Ship_Done_` | 0x38858 | CMBTFIR2 | 10 | - | yes | Do_Combat_Turn_ |
| `Used_Teleporter_` | 0x38876 | CMBTFIR2 | 8 | - | yes | Do_Combat_Turn_, Draw_Legal_Boxes_, Move_Ship_, Set_Legal_Moves_ |
| `Set_Teleporter_Used_` | 0x3888c | CMBTFIR2 | 8 | - |  | Move_Ship_ |
| `Already_Boarded_` | 0x388a2 | CMBTFIR2 | 7 | - | yes | Boarding_Action_Type_, Do_Combat_Turn_ |
| `Set_Boarded_Flag_` | 0x388b8 | CMBTFIR2 | 7 | - |  | Do_Combat_Turn_, Initiate_Boarding_Action_, Resolve_Ai_Boarding_ |
| `Being_Captured_` | 0x388ce | CMBTFIR2 | 7 | - | yes | Init_Ship_For_Start_Of_Turn_, Target_Ship_Value_ |
| `Set_Capturing_Flag_` | 0x388e7 | CMBTFIR2 | 7 | - | yes | Init_Ship_For_Start_Of_Turn_, Resolve_Ai_Boarding_ |
| `Retarget_Missiles_Aimed_At_` | 0x3891e | CMBTFIRE | 35 | - | yes | Capture_Ship_, Destroy_Ship_, Fire_Special_Weapon_, Retreat_Ship_ |
| `Is_Combat_Ship_Dead_` | 0x3897a | CMBTFIRE | 36 | - | yes | Apply_Combat_Artemis_Net_, Apply_Ship_Damage_, Defensive_Fire_Check_, Do_Auto_Ship_Turn_, Do_Combat_Turn_, Do_Spherical_SFX_... |
| `Target_Is_Legal_For_Weapon_` | 0x389e8 | CMBTFIRE | 107 | - | yes | Draw_Weapon_Status_Display_, Expected_Weapon_Damage_, Fire_Ship_, Retarget_Missile_ |
| `Fire_Ship_` | 0x38b5e | CMBTFIRE | 545 | R | yes | Defensive_Fire_Check_, Do_Auto_Ship_Turn_, Do_Combat_Turn_, Final_Fire_, Tactical_Bombardment_ |
| `Get_Beam_Range_To_Hit_Bonus_` | 0x39406 | CMBTFIRE | 16 | - |  | Get_Beam_Weapon_Modifiers_ |
| `Get_Beam_Weapon_Modifiers_` | 0x39434 | CMBTFIRE | 62 | - | yes | Do_Auto_Ship_Turn_, Energy_Absorber_Attack_, Expected_Beam_Bomb_Damage_, Fire_Beam_Weapon_ |
| `Fire_Beam_Weapon_` | 0x394f7 | CMBTFIRE | 328 | R! | yes | Fire_Ship_ |
| `Apply_Ship_Damage_` | 0x39985 | CMBTFIRE | 313 | R | yes | Apply_Combat_Artemis_Net_, Do_Spherical_SFX_, Energy_Absorber_Attack_, Fire_Beam_Weapon_, Fire_Fighter_Beam_, Fire_Fighter_Bomb_... |
| `Facing_Shield_` | 0x39de0 | CMBTFIRE | 19 | - | yes | Beam_SFX_, Boarding_Action_Type_, Do_Spherical_SFX_, Energy_Absorber_Attack_, Fire_Beam_Weapon_, Fire_Ship_... |
| `Destroy_Ship_` | 0x39e15 | CMBTFIRE | 74 | R | yes | Capture_Ship_, Do_Auto_Ship_Turn_, Do_Combat_Turn_, Do_Spherical_SFX_, Energy_Absorber_Attack_, Fire_Ship_... |
| `Weapon_In_Range_` | 0x39f1d | CMBTFIRE | 123 | - | yes | Draw_Weapon_Status_Display_, Expected_Weapon_Damage_, Fire_Ship_, Get_Current_Ship_Target_ |
| `Apply_Damage_To_Missile_` | 0x3a0b9 | CMBTFIRE | 47 | - | yes | Fire_Beam_Weapon_, Fire_Fighter_Beam_ |
| `Does_Combat_Planet_Have_Defenses_` | 0x3a142 | CMBTFIRE | 33 | - | yes | Ai_Stasis_Check_, Apply_Damage_To_Planet_, Check_For_Winner_, Draw_Grey_Damage_Bars_, Expected_Missile_Damage_, Get_Current_Ship_Target_... |
| `Destroy_Colony_Defense_` | 0x3a19e | CMBTFIRE | 144 | R! | yes | Apply_Damage_To_Planet_ |
| `Apply_Damage_To_Planet_` | 0x3a3c3 | CMBTFIRE | 198 | R! | yes | Apply_Ship_Damage_, Fire_Bomb_ |
| `Fire_Bomb_` | 0x3a6ca | CMBTFIRE | 69 | R! | yes | Fire_Ship_ |
| `Weapon_Can_Fire_At_Missiles_` | 0x3a7aa | CMBTFIRE | 41 | - | yes | Get_Effective_Missile_Strength_, Target_Is_Legal_For_Weapon_ |
| `Fire_Special_Weapon_` | 0x3a82f | CMBTFIRE | 257 | R | yes | Fire_Ship_ |
| `Fire_Fighter_Bomb_` | 0x3ac20 | CMBTFIRE | 91 | R! | yes | Resolve_Missile_ |
| `Fire_Fighter_Beam_` | 0x3ad57 | CMBTFIRE | 173 | R! | yes | Resolve_Missile_ |
| `Remove_Stasis_Beam_` | 0x3afb3 | CMBTFIRE | 20 | - | yes | Apply_Internal_Damage_, Do_Combat_Turn_, Release_Smallest_Ship_From_Stasis_, Remove_Continuing_Effect_Specials_ |
| `Special_Is_On_` | 0x3afe5 | CMBTFIRE | 10 | - | yes | Draw_Special_Status_Display_, Pre_Weapon_Specials_, Special_Is_Turned_On_ |
| `Special_Is_Turned_On_` | 0x3b004 | CMBTFIRE | 44 | - | yes | Set_Legal_Moves_ |
| `Energy_Absorber_Attack_` | 0x3b077 | CMBTFIRE | 179 | R! | yes | Pre_Weapon_Specials_ |
| `Pre_Weapon_Specials_` | 0x3b332 | CMBTFIRE | 61 | R | yes | Fire_Ship_ |
| `Draw_Energy_Absorber_` | 0x3b3ed | CMBTFIRE | 77 | - | UI | Draw_Ship_ |
| `Draw_BHG_` | 0x3b4c0 | CMBTFIRE | 110 | - | UI | Draw_Ship_ |
| `BHG_` | 0x3b627 | CMBTFIRE | 129 | R | yes | Fire_Special_Weapon_ |
| `Missile_Owner_` | 0x3bb3d | CMBTMIS | 7 | - | yes | Ai_Self_Destruct_Check_, Do_Combat_Turn_, Do_Spherical_SFX_, Does_Ship_Have_Missiles_Incoming_, Fighter_Combat_SFX_, Fire_Special_Weapon_... |
| `Missile_Type_` | 0x3bb50 | CMBTMIS | 69 | - | UI | Draw_Fighter_, Draw_Missile_, Pre_Weapon_Specials_ |
| `Coord_Movement_` | 0x3bbf6 | CMBTMIS | 55 | - | yes | Move_Missile_ |
| `Kill_Missile_` | 0x3bc80 | CMBTMIS | 31 | - | yes | Anti_Missile_Rockets_, Beam_SFX_, Do_Spherical_SFX_, Fire_Fighter_Beam_, Fire_Missile_, Move_Missile_... |
| `Did_Missile_Hit_` | 0x3bccd | CMBTMIS | 32 | - | yes | Move_Missile_ |
| `Picture_Count_` | 0x3bd2c | CMBTMIS | 69 | - | UI | Draw_Missile_ |
| `Get_Multiple_Torpedo_Coords_` | 0x3bdda | CMBTMIS | 113 | - | UI | Draw_Missile_ |
| `Get_Multiple_Missile_Coords_` | 0x3bf72 | CMBTMIS | 219 | - | UI | Draw_Fighter_, Draw_Missile_, Fighter_Combat_SFX_ |
| `Draw_Missile_` | 0x3c2dc | CMBTMIS | 175 | R | UI | Draw_Missile_Status_, Draw_Missiles_, Move_Ship_, Rotate_Ship_ |
| `Draw_Missile_Status_` | 0x3c539 | CMBTMIS | 244 | R | UI | Draw_Main_Combat_Screen_ |
| `Draw_Missiles_` | 0x3c834 | CMBTMIS | 37 | R | UI | Draw_Main_Combat_Screen_, Scroll_Draw_Combat_Screen_ |
| `Fire_Missile_` | 0x3c892 | CMBTMIS | 252 | R | yes | Fire_Ship_ |
| `Missile_Facing_` | 0x3cc9c | CMBTMIS | 42 | - | UI | Fire_Missile_ |
| `Missile_Speed_` | 0x3cd21 | CMBTMIS | 83 | - | yes | Fire_Missile_, Missile_Dcv_, Move_Missile_, Resolve_Missile_ |
| `Missile_On_Combat_Screen_` | 0x3ce4f | CMBTMIS | 40 | - | yes | Draw_Missile_, Seeking_Missiles_ |
| `Move_Missile_` | 0x3ceb7 | CMBTMIS | 264 | R | yes | Missile_Launch_FX_, Move_Missile_, Move_Ship_, Rotate_Ship_, Rotate_Ship_Until_Impact_, Seeking_Missiles_... |
| `Next_Missile_Id_` | 0x3d21d | CMBTMIS | 25 | - | yes | Fire_Missile_ |
| `On_Combat_Screen_` | 0x3d259 | CMBTMIS | 24 | - | UI | Add_Missile_Fields_, Draw_Main_Combat_Screen_, Draw_Teleporting_Ship_, Fighter_Combat_SFX_, Fire_Ship_, Ship_Completely_On_Screen_... |
| `Is_Missile_Target_Alive_` | 0x3d299 | CMBTMIS | 25 | - | yes | Fire_Fighter_Beam_, Resolve_Missile_ |
| `Resolve_Missile_` | 0x3d2df | CMBTMIS | 620 | R! | yes | Fire_Missile_, Missile_Launch_FX_, Move_Missile_, Move_Ship_, Rotate_Ship_, Seeking_Missiles_ |
| `Find_Weakest_Shield_` | 0x3db8f | CMBTMIS | 25 | - | yes | Resolve_Missile_ |
| `Draw_Fighter_` | 0x3dbd1 | CMBTMIS | 151 | R! | UI | Draw_Missile_, Fighter_Combat_SFX_ |
| `Retarget_Missile_` | 0x3ddd8 | CMBTMIS | 116 | - | yes | Do_Combat_Turn_, Fire_Fighter_Beam_, Kill_Missile_, Missile_Launch_FX_, Resolve_Missile_, Retarget_Missiles_Aimed_At_ |
| `Fighter_Ocv_` | 0x3df8d | CMBTMIS | 27 | - | yes | Fire_Fighter_Beam_ |
| `Missile_Dcv_` | 0x3dfe0 | CMBTMIS | 68 | - | yes | Average_Missile_Kills_, Expected_Beam_Bomb_Damage_, Fire_Beam_Weapon_, Fire_Fighter_Beam_ |
| `Get_Missile_Hits_` | 0x3e095 | CMBTMIS | 76 | - | yes | Apply_Damage_To_Missile_, Average_Missile_Kills_, Do_Spherical_SFX_, Expected_Weapon_Damage_, Target_Missile_Value_ |
| `Fighter_Combat_SFX_` | 0x3e1a4 | CMBTMIS | 264 | R | UI | Resolve_Missile_ |
| `Ship_Is_In_Stasis_` | 0x3e563 | CMBTMOV1 | 8 | - | yes | Ai_Self_Destruct_Check_, Ai_Stasis_Check_, Assign_Mouse_Images_, Combat_Ship_Is_Functional_, Do_Auto_Ship_Turn_, Do_Combat_Turn_... |
| `Ship_In_Black_Hole_` | 0x3e57c | CMBTMOV1 | 8 | - | yes | Calc_Current_Speed_, Draw_Ship_, Increment_Bhg_, Target_Ship_Value_ |
| `Draw_Teleporting_Ship_` | 0x3e598 | CMBTMOV1 | 141 | R | UI | Move_Ship_ |
| `Rotate_Ship_Until_Impact_` | 0x3e7a9 | CMBTMOV1 | 160 | R | yes | Test_Move_Until_Impact_ |
| `Ship_In_Legal_Square_` | 0x3ea09 | CMBTMOV1 | 60 | - | yes | Test_Move_Until_Impact_ |
| `Test_Move_Until_Impact_` | 0x3eaa5 | CMBTMOV1 | 241 | R | yes | Move_Ship_ |
| `Move_Ship_` | 0x3ee0f | CMBTMOV1 | 527 | R | yes | Auto_Move_Ship_, Do_Combat_Turn_ |
| `Get_Facing_` | 0x3f5f1 | CMBTMOV1 | 20 | - | yes | Assign_Mouse_Images_, Draw_Bombs_, Missile_Facing_, Move_Cost_, Move_Missile_, Move_Ship_... |
| `Rotate_Ship_` | 0x3f628 | CMBTMOV1 | 210 | R | yes | Do_Combat_Turn_, Draw_Retreating_Ship_, Move_Ship_, Turn_To_Face_ |
| `Move_Cost_` | 0x3f95f | CMBTMOV1 | 94 | - | yes | Auto_Move_Ship_, Do_Combat_Turn_, Max_Defensive_Fire_Range_, Move_Ship_, Set_Legal_Moves_ |
| `Turn_To_Face_` | 0x3fa61 | CMBTMOV1 | 125 | R | yes | Auto_Move_Ship_, Do_Combat_Turn_ |
| `Wait_Until_Net_Opponent_Finished_` | 0x3fbfe | CMBTMOV1 | 62 | R | UI | Do_Combat_Turn_, Init_Ship_For_Start_Of_Turn_, Initiate_Boarding_Action_, Next_Ship_, Resolve_Ai_Boarding_, Tactical_Combat_ |
| `Tactical_Combat_Receive_Message_` | 0x3fcd8 | CMBTMOV1 | 47 | - | UI | Receive_Message_ |
| `Message_Box_Titled_` | 0x3fdaa | COMBAT | 5 | R | UI | Russ_Combat_ |
| `QLoad_Combat_Antaran_Ship_` | 0x3fdb4 | COMBAT | 88 | - | yes | QLoad_Antaran_Defense_Fleet_ |
| `QLoad_Antaran_Defense_Fleet_` | 0x3fef2 | COMBAT | 72 | - | yes | Strategic_Combat_ |
| `Get_Leader_Combat_Bonuses_` | 0x40034 | COMBAT | 78 | - | yes | Strategic_Combat_ |
| `Strategic_Combat_` | 0x40148 | COMBAT | 694 | R | yes | Russ_Combat_ |
| `Apply_Damage_To_Qship_` | 0x40c2a | COMBAT | 50 | - | yes | Apply_Combat_Artemis_Net_, Beam_Attack_, Missile_Attack_, Strat_Bomb_Attack_, Strat_Special_Attack_, Strategic_Combat_ |
| `Get_Non_Player_Name_` | 0x40cc3 | COMBAT | 51 | - |  | Build_End_Of_Combat_Message_ |
| `Build_End_Of_Combat_Message_` | 0x40d40 | COMBAT | 1067 | R | yes | Strategic_Combat_ |
| `Qload_Ships_` | 0x416cf | COMBAT | 433 | - | yes | Strategic_Bombardment_, Strategic_Combat_ |
| `QGet_Target_` | 0x41e88 | COMBAT | 50 | R! | yes | Beam_Attack_, Missile_Attack_, Strat_Special_Attack_, Strategic_Combat_ |
| `QGet_Target_SC_` | 0x41f20 | COMBAT | 33 | - | yes | Strategic_Combat_ |
| `Missile_Attack_` | 0x41f80 | COMBAT | 93 | R! | yes | Strategic_Bombardment_, Strategic_Combat_ |
| `Strat_Bomb_Attack_` | 0x420c0 | COMBAT | 43 | R! | yes | Strategic_Bombardment_, Strategic_Combat_ |
| `Strat_Special_Attack_` | 0x4215c | COMBAT | 65 | R! | yes | Strategic_Combat_ |
| `Beam_Attack_` | 0x4221f | COMBAT | 90 | R! | yes | Strategic_Bombardment_, Strategic_Combat_ |
| `Get_Colony_Hits_` | 0x42371 | COMBAT | 39 | - | yes | Qload_Ships_ |
| `Resolve_Strat_Colony_Damage_` | 0x423f6 | COMBAT | 96 | R | yes | Strategic_Combat_ |
| `Strategic_Bombardment_` | 0x4257e | COMBAT | 69 | R | yes | Bombardment_ |
| `Bombardment_` | 0x4267b | COMBAT | 54 | R | yes | Do_Attacker_Beat_Colony_Stuff_ |
| `Tactical_Bombardment_` | 0x42781 | COMBAT | 159 | R | yes | Bombardment_ |
| `Combat_Ship_Is_Functional_` | 0x42a27 | COMBAT1 | 22 | - | yes | Boarding_Action_Type_, Calc_Current_Speed_, Defensive_Fire_Check_, Do_Auto_Ship_Turn_, Next_Ship_, Resolve_Missile_ |
| `Enemy_Dissipator_Exists_` | 0x42a61 | COMBAT1 | 31 | - | yes | Do_Auto_Ship_Turn_, Do_Combat_Turn_, Retreat_Check_ |
| `Ship_Controlled_By_Computer_` | 0x42abf | COMBAT1 | 23 | - | UI | Draw_Main_Combat_Screen_, Draw_Weapon_Status_Display_ |
| `Send_Tactical_Combat_Message_` | 0x42b0f | COMBAT1 | 25 | R | UI | Do_Combat_Turn_, Initiate_Boarding_Action_, Resolve_Ai_Boarding_ |
| `Init_Ship_For_Start_Of_Turn_` | 0x42b70 | COMBAT1 | 183 | R | yes | Do_Combat_Turn_ |
| `Do_Combat_Turn_` | 0x42f7f | COMBAT1 | 1785 | R | yes | Tactical_Combat_ |
| `Seeking_Missiles_` | 0x44ea4 | COMBAT1 | 249 | R | yes | Do_Combat_Turn_ |
| `Calc_Current_Speed_` | 0x4528f | COMBAT1 | 134 | - | yes | Apply_Internal_Damage_, Boarding_Action_Type_, Init_Ship_For_Start_Of_Turn_, Initiate_Boarding_Action_, Max_Defensive_Fire_Range_, Tactical_Combat_... |
| `Check_For_Winner_` | 0x4545b | COMBAT1 | 54 | - | yes | Next_Ship_, Tactical_Combat_ |
| `Set_Cur_Ship_To_` | 0x45507 | COMBAT1 | 51 | R | yes | Do_Combat_Turn_ |
| `Next_Ship_` | 0x455a4 | COMBAT1 | 135 | R | yes | Do_Combat_Turn_ |
| `Weapon_Is_On_` | 0x4584c | COMBAT1 | 11 | - | yes | Draw_Weapon_Status_Display_, Expected_Weapon_Damage_, Fire_Ship_, Get_Current_Ship_Target_ |
| `Remove_Continuing_Effect_Specials_` | 0x4588c | COMBAT1 | 39 | - | yes | Capture_Ship_, Destroy_Ship_, Retreat_Ship_ |
| `Ship_Done_Check_` | 0x4590d | COMBAT1 | 24 | - | UI | Do_Combat_Turn_ |
| `Retreat_Ship_` | 0x45964 | COMBAT1 | 38 | R | yes | Init_Ship_For_Start_Of_Turn_ |
| `Initiate_Boarding_Action_` | 0x459e7 | COMBAT1 | 294 | R |  | Do_Combat_Turn_ |
| `Add_Missile_Fields_` | 0x45ea7 | COMBAT1 | 101 | - | UI | Add_Tactical_Combat_Fields_ |
| `Set_Combat_Help_Fields_` | 0x4602a | COMBAT1 | 239 | - | UI | Add_Tactical_Combat_Fields_, Do_Combat_Turn_ |
| `Add_Tactical_Combat_Fields_` | 0x465d0 | COMBAT1 | 357 | - | UI | Display_Combat_View_Ship_, Do_Combat_Turn_, Move_Options_Background_, Next_Ship_, Refresh_Combat_Screen_Full_, Scroll_Combat_Screen_... |
| `Assign_Combat_Grids_` | 0x46cc8 | COMBAT1 | 175 | - | yes | Board_Popup_, Destroy_Ship_FX_, Display_Combat_View_Ship_, Do_Auto_Ship_Turn_, Do_Combat_Turn_, Move_Options_Background_... |
| `Assign_Mouse_Images_` | 0x46f42 | COMBAT1 | 381 | - | UI | Assign_Combat_Grids_ |
| `Set_Legal_Moves_` | 0x4752f | COMBAT1 | 246 | - | yes | Assign_Combat_Grids_ |
| `Combat_Screen_` | 0x478a2 | COMBINIT | 1 | - | UI |  |
| `Colony_Has_Defenses_` | 0x478a3 | COMBINIT | 26 | - | yes | Tactical_Combat_ |
| `Tactical_Combat_` | 0x47939 | COMBINIT | 1232 | R | yes | Russ_Combat_ |
| `Ship_Can_Deploy_At_` | 0x48f49 | COMBINIT | 88 | - | yes | Deploy_Ships_ |
| `Deploy_Ships_` | 0x49043 | COMBINIT | 293 | - | yes | Tactical_Combat_ |
| `Calc_Combat_Shield_Max_` | 0x494a8 | COMBINIT | 57 | - | yes | Apply_Combat_Artemis_Net_, Apply_Internal_Damage_, Get_Total_Hits_, Load_Combat_Satelite_, Load_Combat_Ship_ |
| `Load_Combat_Ship_` | 0x4954a | COMBINIT | 338 | - | yes | Tactical_Bombardment_, Tactical_Combat_ |
| `Scroll_Combat_Screen_` | 0x49a41 | COMBINIT | 169 | R | UI | Do_Combat_Turn_ |
| `Snap_Center_Combat_Screen_` | 0x49d09 | COMBINIT | 109 | R | UI | Destroy_Ship_FX_, Do_Auto_Ship_Turn_, Do_Combat_Turn_, Do_Spherical_SFX_, Draw_Bombs_, Draw_Retreating_Ship_... |
| `Ship_Center_Offsets_` | 0x49ed1 | COMBINIT | 61 | - | yes | Add_Damage_Indicator_For_Ship_, BHG_, Beam_SFX_, Defensive_Fire_Check_, Destroy_Ship_FX_, Do_Spherical_SFX_... |
| `Load_Combat_Ship_Palette_` | 0x49f99 | COMBINIT | 110 | R | UI | Tactical_Combat_ |
| `Load_Individual_Ship_Pictures_` | 0x4a12a | COMBINIT | 211 | R | UI | Tactical_Combat_ |
| `Increment_Bhg_` | 0x4a4e4 | COMBINIT | 45 | R | yes | Tactical_Combat_ |
| `End_Of_Turn_Bookeeping_` | 0x4a575 | COMBINIT | 29 | - | yes | Tactical_Combat_ |
| `Plasma_Web_Damage_` | 0x4a5ce | COMBINIT | 118 | R | yes | Tactical_Combat_ |
| `Recharge_Shields_` | 0x4a790 | COMBINIT | 66 | - | yes | End_Of_Turn_Bookeeping_ |
| `Type_Of_Space_Station_` | 0x4a884 | COMBINIT | 28 | - | yes | Combat_Draw_View_Ship_, Draw_Grey_Damage_Bars_, Load_Colony_Defense_ |
| `Load_Antaran_Planet_` | 0x4a8d3 | COMBINIT | 68 | R |  | Tactical_Combat_ |
| `Load_Colony_Defense_` | 0x4a9e9 | COMBINIT | 22 | R | yes | Tactical_Combat_ |
| `Load_Tactical_Colony_` | 0x4aa36 | COMBINIT | 370 | R | yes | Load_Colony_Defense_, Tactical_Bombardment_ |
| `Load_Combat_Colony_Picture_` | 0x4b01a | COMBINIT | 53 | R | UI | Load_Tactical_Colony_ |
| `Does_Combat_Ship_Have_Special_` | 0x4b0d3 | COMBINIT | 27 | - | yes | Ai_Self_Destruct_Check_, Apply_Damage_To_Planet_, Apply_Ship_Damage_, Average_Missile_Kills_, Boarding_Action_Type_, Calc_Combat_Shield_Max_... |
| `Apply_Bio_Weapon_Penalty_` | 0x4b11e | COMBINIT | 37 | R! | yes | Do_Attacker_Beat_Colony_Stuff_, End_Of_Combat_ |
| `End_Of_Combat_` | 0x4b184 | COMBINIT | 528 | R | yes | Tactical_Combat_ |
| `Unload_Combat_Ship_` | 0x4b9e1 | COMBINIT | 128 | - | yes | End_Of_Combat_ |
| `Load_Combat_Satelite_` | 0x4bbd5 | COMBINIT | 867 | - | yes | Load_Colony_Defense_ |
| `Init_Special_Devices_` | 0x4c9f6 | COMBINIT | 83 | - | yes | End_Of_Combat_, Load_Combat_Ship_, Tactical_Bombardment_, Tactical_Combat_ |
| `Load_Display_Combat_Ship_` | 0x4cb1e | COMBINIT | 291 | - | yes | Detailed_View_Ship_ |
| `Repair_All_Combat_Ships_` | 0x4cfe7 | COMBINIT | 78 | - | yes | End_Of_Turn_Bookeeping_ |
| `Load_Combat_Antaran_Ship_` | 0x4d10e | COMBINIT | 14 | - |  | Load_Antaran_Defense_Fleet_ |
| `Load_Antaran_Defense_Fleet_` | 0x4d141 | COMBINIT | 24 | - |  | Tactical_Combat_ |
| `Load_Antaran_Star_Fortress_` | 0x4d18e | COMBINIT | 388 | - |  | Load_Antaran_Defense_Fleet_ |
| `Draw_Anti_Missile_Rocket_Picture_` | 0xab850 | CMBTSPEC | 39 | - | UI | Draw_Anti_Missile_Rockets_ |
| `Draw_Anti_Missile_Rockets_` | 0xab8bf | CMBTSPEC | 89 | R | UI | Anti_Missile_Rockets_ |
| `Anti_Missile_Rockets_` | 0xaba09 | CMBTSPEC | 160 | R! | yes | Fire_Special_Weapon_ |
| `Draw_Gyro_Special_Effect_` | 0xabc5d | CMBTSPEC | 226 | R | UI | Gyro_Destablizer_ |
| `Gyro_Destablizer_` | 0xac136 | CMBTSPEC | 212 | R! | yes | Fire_Special_Weapon_ |
| `Lightning_Field_` | 0xac472 | CMBTSPEC | 124 | R | UI | Resolve_Missile_ |
| `Draw_Stasis_Ball_` | 0xac619 | CMBTSPEC | 194 | - | UI | Draw_Ship_, Draw_Stasis_Beam_ |
| `Draw_Stasis_Beam_` | 0xac87a | CMBTSPEC | 169 | R | UI | Fire_Special_Weapon_ |
| `Draw_Cloak_` | 0xacb4a | CMBTSPEC | 100 | R | UI | Do_Combat_Turn_, Fire_Ship_, Initiate_Boarding_Action_ |
| `Draw_Continuing_Tractor_` | 0xaccdb | CMBTSPEC | 76 | - | UI | Full_Draw_Combat_Screen_, Rotate_Ship_, Scroll_Draw_Combat_Screen_ |
| `Draw_Moving_Tractor_` | 0xace08 | CMBTSPEC | 99 | - | UI | Move_Ship_ |
| `Plasma_Web_` | 0xacf83 | CMBTSPEC | 297 | R! | yes | Fire_Special_Weapon_ |
| `Tractor_Beam_` | 0xad3ca | CMBTSPEC | 110 | R | yes | Fire_Special_Weapon_ |
| `Draw_Plasma_Web_` | 0xad580 | CMBTSPEC | 192 | - | UI | Draw_Ship_ |
| `Draw_Transporter_Bomb_Beam_` | 0xad7f5 | CMBTSPEC | 97 | R | UI | Draw_Bombs_ |
| `Stellar_Converter_` | 0xad98a | CMBTSPEC | 220 | R | yes | Fire_Special_Weapon_ |
| `Start_Ship_Engine_SFX_` | 0xadcff | CMBTSPEC | 10 | - | UI | Move_Ship_ |
| `End_Ship_Engine_SFX_` | 0xadd24 | CMBTSPEC | 7 | - | UI | Move_Ship_ |
| `Draw_Ship_Collapsing_` | 0xadd40 | CMBTSPEC | 59 | - | UI | Do_Spherical_SFX_ |
| `Do_Spherical_SFX_` | 0xade18 | CMBTSPEC | 922 | R! | yes | Destroy_Ship_FX_, Fire_Special_Weapon_ |
| `Play_Ship_Explosion_SFX_` | 0xaec0b | CMBTSPEC | 41 | R! | yes | Destroy_Ship_FX_ |
| `Destroy_Ship_FX_` | 0xaec7b | CMBTSPEC | 469 | R | yes | Destroy_Ship_ |
| `Missile_Launch_FX_` | 0xaf37e | CMBTSPEC | 119 | R | yes | Fire_Missile_ |

## Port

`tacticalCombat` runs `Tactical_Combat_`'s logic in the original order; the
screen is the `tacticalUI` interface (`HumanInput` for `Get_Input` in the
human's input loop, `AutoPressed` for the three `Get_Input` checks in
`Do_Auto_Ship_Turn_`). Until the screen exists the stand-in `autoUI` presses
Auto at the human's first input, which is exactly what the oracle traces do
(key "a"), so the AI plays both sides. GO column: "yes" ported, "UI" only
draws (left out), empty not ported.

What the screen does to the logic, and how the port keeps it:

- `_apply_damage_to_planet` (0x1998f6) is 1: missiles fly
  (`Missile_Launch_FX_` moves them 36 pixels and they resolve on arrival in
  `Seeking_Missiles_`), every arc is checked, ships explode through
  `Destroy_Ship_FX_`.
- `Fire_Ship_` collects one 0xf-byte record per beam mount that fired
  (weapon, shots, result, kills/damage, absorbed, mods; reflected beams a
  second record) when the screen is on and not super fast, and plays them
  with `Beam_SFX_` before the next non-beam mount and at the end. Against a
  missile stack `Beam_SFX_` takes each record's kills off the stack's
  count and kills it below 1: point defence only thins a stack through the
  drawing (`beamSFX`).
- Drawing that touches the seed: `Play_Ship_Explosion_SFX_` (Random(3) for
  the sound), `Do_Spherical_SFX_` (Random(10) for a sound when a ship blows up
  with kind 3; two Random(5) per frame for every ship the shock wave shakes,
  frames and radius from CMBTSFX.LBX 0x34/6 and SPHERSFX.LBX 0/1 and the
  table at 0xab756), `Gyro_Destablizer_` (19+ Random(0xd) spins, which also
  turn the target for good). Bracketed (seed saved and restored, the test
  harness follows the restore to an ancestor): `Beam_SFX_`, `Bolt_Segment_`,
  `Draw_Fighter_`. No draw seen from `Draw_Main_Combat_Screen_`.
- MOX.SET's speed x2 (settings 0xb4) changes the step of `Move_Ship_` (2 or
  12 pixels; missiles chasing the ship and defensive fire every third step
  follow it), `Missile_Launch_FX_` and `Test_Move_Until_Impact_`; the missile
  impact warning (0xb6) lets `Move_Ship_` stop short of an incoming missile.
  The view position (`_cur_combat_x/y`) only matters for drawing: the
  shock-wave distances are relative.
- Quirks kept: `Init_Ship_For_Start_Of_Turn_` gives a stasis projector its
  count minus the being-captured flag (not the number of ships it holds);
  `Seeking_Missiles_` picks the hit point by the at-missile flag of stack k
  of its hit list, not of the missile; `Deploy_Ships_` restores every base's
  hull to the last base's; `Repair_Combat_Ship_` sets a special back to its
  maximum; `Load_Combat_Satelite_` tests app 0x38 against the bool of app
  0x52; `Auto_Move_Ship_` leaves DI uninitialised for a class above 2 (never
  happens).

Ported but not yet seen in a trace: boarding (`Resolve_Ai_Boarding_`,
`Resolve_Raid_` with `Select_Raid_Internal_`, `Resolve_Capture_`,
`Capture_Ship_`), the special weapons, the Artemis net, satellites,
`Russ_Event_Monster_Update_` after a monster battle (as in strategic
combat), `Crystal_Control_` (the crystal takes over a ship without marines
it shot), the teleporter (`Set_Teleporter_Used_` when the target is within
range 0x14; `Draw_Teleporting_Ship_`'s fades draw `Random_(1000)` once per
`Vanish_Bitmap_` between 1 and 99 percent: 18 draws, 8 at double speed,
none in super fast mode, on the screen or not). Not ported: initiative
order (`_combat_init` ≠ 0, `Qsort_Ship_Initiative_` and the patch code at
0x1694b9); network play.

## Antares

At Antares (star = NUM_STARS) `Tactical_Combat_` skips the defenders' loop
and `Load_Colony_Defense_`: `Load_Antaran_Defense_Fleet_` (0x4d141; the
counts by size at antarans+6.., at least one titan) loads each ship with
`Load_Combat_Antaran_Ship_` (0x4d10e → the five size loaders, fixed mounts
and specials, one more special by difficulty; small, large and titan clear
a mount they just wrote), then `Load_Antaran_Star_Fortress_` (0x4d18e: size
4, the fortress hull's structure and armour ×3, its space in four runs of
mounts, officer 0x42), and `Load_Antaran_Planet_` (0x4a8d3) makes Antares
record 0 (CMBTPLNT 0x3c/0x3d, palette 0x3e). The screen's effect flags are
the black hole generator and reflection. Port: turn_tacantares.go, the
five loaders as one table (`antaranShips`, checked against each loader's
assembly: name length, mounts, cleared mount, specials, difficulty
special, picture); tc12 (large, titan) and tca1 (small, medium, huge,
titan) match.

## Where this stopped (2026-10-18)

- Done: the logic, initiative order (MOX.SET +0xd, the 1.40 loop: speed +
  OCV/10 order, both sides' turns per round, missiles under initiative seek
  unless fired this turn) and the screen (`docs/screens/combat-screen.md`),
  now with the animations' drawing, boarding, scrolling, the ground battle
  replay, the captured tech show and the planetary destruction video.
  Oracle battles: tc3s, tc7..tc14, tci1 (initiative; draws and CMBT.LOG
  records), tc1..tc3 off screen in the turn tests.
- Frame-by-frame captures of the original: tc10f, tcfx1, tcs1, tcg1. tcb1's
  trace stops at step 24: the capture fails and Auto then never ends the
  battle (build/turns/tcb1f/cap has the frames), so TestBattleTcb1 skips;
  tcb2 (capture) and tcb3 (raid) retreat instead and match draws,
  records, frames and the step save; tcb4 covers the AI's self-destruct
  (`Ai_Self_Destruct_Check_`) against marines it cannot hold, tcb5 a
  successful capture (`Capture_Ship_`).
- Not ported: network play; other cinematics' fading mode; sounds.
- Fighters, the teleporter, stasis and its release, and `Crystal_Control_`
  with its takeover have oracles (below).
- Traces: one DOSBox at a time (build/dosbox.lock is shared with the other
  worktree); keys must stay under DOSBox-X's autotype limit; keys typed
  during animations are eaten by their Get_Input calls, so pad with lshift
  or use -combatkeys.
