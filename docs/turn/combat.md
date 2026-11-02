# Combat in the turn: strategic resolver and the tactical engine off screen

How `Search_For_Battles_` (step 24 of `Next_Turn_Calc_`, the search itself is
in `overview.md`, phase E) resolves a battle without the combat screen. ELF
addresses (Watcom + 0x10000). Go: `internal/game/turn_combat*.go`,
`turn_stratcombat.go`, `turn_tactical.go`, `turn_tacfire.go`.

## Who resolves what

`Russ_Combat_` (0xe7343) picks the resolver:

- **Strategic_Combat_** (0x40148) when settings byte 0xd8 is set, or when
  neither side is a human. So every AI-vs-AI and monster battle goes here,
  whatever the setting.
- **Tactical_Combat_** (0x47939) otherwise: a human fights on the combat
  screen. Its logic is ported; the screen is not (see the end).

After a won battle at a colony, `Do_Attacker_Beat_Colony_Stuff_` bombs it with
`Bombardment_` (0x4267b): **Strategic_Bombardment_** (0x4257e) when 0xd8 == 1,
else **Tactical_Bombardment_** (0x42781), which runs the tactical engine with
the screen off (0x1998f6 = 0). The latter is the common case: all the shipped
saves have 0xd8 = 0.

## Strategic_Combat_ (0x40148)

State: `_qcombat_data` (0x192844), 0x21-byte records in screen_seg
(`Allocate_First_Block_`, cleared here, *not* by Strategic_Bombardment_),
and globals: 0x1998c0 records in use, 0x1998a2 / 0x1998be live attackers /
defenders, 0x199890 round, 0x1998ac / 0x19988e attacker / defender,
0x199878 colony, 0x199ef6 colony defence flags (1 missile base 0x1a,
2 batteries 0x1b, 4 converter 0x2a, 8 fighters 0x2f).

Record: +0 owner, +1 size (4 colony, 6..8 satellites 0x28 / 8 / 0x29), +2
ship (-1 colony/satellite), +6 state (0 alive, 1 hit this round, 2 out), +7
officer, +9 shield class, +0xb/+0xd/+0xf/+0x11 missile, beam, bomb, special
weapon (the owner's best, not the ship's own), +0x13 missile defence, +0x15
beam defence, +0x17 attack, +0x19 beam %, +0x1b missile %, +0x1d hits, +0x1f
damage.

1. Guardian (9) against monsters, or monsters against it: the losing side's
   ships are killed outright (result ±1).
2. `Get_Leader_Combat_Bonuses_` (0x40034): best helmsman, ordnance, engineer
   (repair, per owner), weaponry, galactic lore officer aboard (ETA 0).
3. `Qload_Ships_` (0x416cf) per side: combat ships only (design type 0),
   `Update_Strategic_Ship_` first; hits = (armour% + 100) × hull hits / 100
   (raw hull hits for owners > 9); defence = hull + (warp − 1) × 10 + race
   pick; special systems (kind table at 0x17ef0c) give running maxima that are
   **not reset between ships**. Slot 0 is reset by both calls and loaded as
   the colony only when the caller is the defender: hits from
   `Get_Colony_Hits_` (40 × (pop + marines + tanks + buildings other than
   the satellites), 16-bit), shields 0x1c/0x17/0x18 add 10/30/20 missile
   defence (and multiply a zero beam defence), weapons per defence building,
   then a satellite record if there is one.
4. Defender with an Artemis net at the star: `Apply_Combat_Artemis_Net_`
   (0x3868f, flag 1) — per attacker `Random(100)` ≤ chance by size
   (0x180ec8), then `Random(0xc9) + 0x4a` damage ignoring shields; all
   destroyed → result −1, no fight.
5. Antares: `QLoad_Antaran_Defense_Fleet_` (0x3fef2) from the Antaran counts
   (antarans +6), plus the home world record.
6. Up to 50 rounds while both sides have records: each live record picks a
   target with `QGet_Target_` (0x41e88, `Random(n)` over live enemies; the
   attacker falls back to a live colony); slot 0 with the converter flag first
   fires `QGet_Target_SC_` × 4 × 400 damage; missiles (Antarans: beams) on
   rounds 1–3, for big hulls (> 5) or the colony; specials and beams from
   round 3; bombs from round 4 when the colony still stands. Then
   `Resolve_Strat_Colony_Damage_` (0x423f6): each 40 damage removes a defence
   building (converter, missile base, batteries, fighters) via
   `Remove_Building_`; no defences left → colony beaten.
   - `Missile_Attack_` (0x41f80): `Random(100)`, > 95 counts as 1000; hits
     when the roll ≥ target missile defence − 70/−40/−20 by the firer's
     ECM-beating applications (monsters −70).
   - `Beam_Attack_` (0x4221f): miss below 40 − (attack − defence), capped 95;
     damage `Random(max − min + 1) + min − 1` scaled by beam %.
   - `Apply_Damage_To_Qship_` (0x40c2a): shield class 7 quarters the damage,
     else the shield strength is subtracted; damage ≥ hits → state 1.
7. Result: attackers − defenders alive, or −1/0 after 50 rounds.
8. Aftermath per record: destroyed satellites remove their building;
   structure damage written back (repaired fully with an engineer, app 4 or
   cybernetics; raw with 0xd8 == 1, else scaled to `Get_Ship_Structure_`);
   officers of lost ships go to the pool (ETA > 0), are kept for re-seating
   (app 0xb3) or die. Relations: −10000 kind 10 when a colony was fought, and
   −15 × (enemy hulls lost, by size + 1) kind 0xb. Kills
   (`Kill_Antaran_Defensive_Ship_`/`_Offensive_Ship_` for Antarans), monster
   events, and a colony left with no pop and no buildings is destroyed.
   `Build_End_Of_Combat_Message_` only writes text.

## Tactical_Bombardment_ (0x42781) and the engine off screen

Allocates 0x192864 (combat ship records, 0x139 bytes) and 0x192b14 (300
missile records, 0x1a bytes), both cleared. The ships are sorted by hull,
largest first (`qsort_`), loaded with `Load_Combat_Ship_` (0x4954a; all at
square 0,0), the colony with `Load_Tactical_Colony_` (0x4aa36: shield
building as shield class, batteries = best beam ×(0xe1 / space) + point
defence ×(0x4b / space), converter 0x26, two fighter mounts, missile bases),
then `Set_Combat_Ordnance_Values_` (0x199224 by owner: 100 + best ordnance
officer), and per record `Offensive_Combat_Bonus_`, `Defensive_Combat_Bonus_`
(beam and missile), `Init_Special_Devices_`.

**More than 210 ships.** The records block is 0x100c2 bytes, 210 records,
and nothing caps the list (oob1: 238 ships). Allocate_Next_Block_ puts the
missiles block's 12-byte header {0, 0x1e78, 0} at records + 0x100c4 and the
missiles at + 0x100d0, so record 210 on overlaps the header and the
missiles, and the records past 0x11f48 the arena behind them. Missiles fired
in the volleys overwrite those records and their records overwrite the
missiles; indices read back from such bytes (3844 in oob1) point anywhere.
The port keeps records and missiles in one memory with that layout
(`tactical.mem`), which matches all 81210 draws of oob1's turn; the arena
past the missiles and the memory a wild index reaches are zeros (`tsp`,
`msl` log it), a known departure (docs/STATUS.md).

Ten volleys: every record's mounts are re-armed (beam ammo counts down),
then each ship runs `Fire_Ship_(ship, target 0)` (0x38b5e). Off screen the
arc is always 0xf. Per mount that is on, in range (`Weapon_In_Range_`,
`Range_To_Ship_`) and legal (`Target_Is_Legal_For_Weapon_`):

- beams: `Fire_Beam_Weapon_` (0x394f7): `Random(100)` (+ attack − planet
  defence, > 95 = 100) ≥ 40 − range modifier (`Get_Beam_Weapon_Modifiers_`);
  damage interpolated between the ordnance-scaled min and max; halved
  against the planet; `Apply_Ship_Damage_` → `Apply_Damage_To_Planet_`;
- missiles and fighters: `Fire_Missile_` (0x3c892) makes a stack and
  resolves it at once (`Resolve_Missile_`, 0x3d2df): the planet's point
  defence fires first (`Final_Fire_`, which re-enters `Fire_Ship_` with the
  missile as target — its kills only accumulate on the stack, they never
  reduce the count), then one `Random(100)` per missile against the
  planet's missile defence; fighters use `Fire_Fighter_Beam_` /
  `Fire_Fighter_Bomb_` with the owner's fighter weapons (0x199254/0x199242,
  set only by `Tactical_Combat_`, so zero in a fresh session);
- bombs: `Fire_Bomb_` (0x3a6ca): `Random(100)` × span / 100 + min per shot;
- stellar converter: `Stellar_Converter_` (0xad98a): 400 × shots ×
  ordnance% on each of the four arcs. Other specials only use up their
  shots off screen (spherical and plasma weapons) or cannot target a planet.

`Apply_Damage_To_Planet_` (0x3a3c3): biological weapons (flag 0x80) kill a
pop when `Random(100)` ≤ damage (halved/quartered by the victim's
immunity applications); else with defence buildings the damage past shield
arc 0 goes to them (a random building per armour+100, `Destroy_Colony_
Defense_`), without them the damage past arc 1 adds to +0xaa. The result is
+0xaa / 100 bomb hits, the bio count and flags in the hit record that
`Apply_Bomb_Hits_` (BOMB.C) then spends.

Not ported in this engine (unreachable from a bombardment): damage to ships
(`Apply_Ship_Damage_` on a ship record, internal damage, `Destroy_Ship_`),
reflected beams, energy absorbers, assault shuttles (`Resolve_Capture_`),
plasma webs, the screen and sound.

## What is left: the combat screen

Mapped in `docs/combat/tactical.md`.

`Russ_Combat_` calls `Tactical_Combat_(attList, nAtt, defList, nDef, star,
colony, canRetreat)` when a human fights and 0xd8 == 0, and uses its return
value like `Strategic_Combat_`'s (> 0 attacker won, 0 draw, < 0 lost). Its
logic is ported (`tacticalCombat` in `turn_tacloop.go`, with the screen behind
an interface whose stand-in presses Auto); see `docs/combat/tactical.md`. Its effects on the save are those of the strategic
resolver's aftermath plus the tactical ones: ship structure/armour/system
damage (+0x7b, +0x7d, +0x6e..+0x70, damaged specials), crew experience,
retreats (status 9 via the screen), captured ships, officers, colony
buildings, relations, the AI's per-turn combat decisions — all driven by the
player's input. The other human-only screens are hooks too:
`Defense_Colony_Selection_Popup_` / the confirmation box (a human attacker
picks a target), `Dragon_New_Star_Popup_`, and the human attacker's
`Colony_Selection_Popup_` loop in `Do_Attacker_Beat_Colony_Stuff_` (bomb,
invade, destroy, mind control; its invade preview changes the seed), plus
`Give_Player_Loknar_Special_` for beating the Guardian.
