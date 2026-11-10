# Network game: combat

How battles are fought in a network game
(`_game_type` 2), read from `build/orion2.dis`. Ported in
`internal/game/netcombat.go`; the battle itself (`Do_1_Combat_`,
`Russ_Combat_`, `Do_Attacker_Beat_Colony_Stuff_`) in `turn_combatdo.go` and
`turn_combatcolony.go`. The turn around it is `turn.md`.

## Who fights where

Battles are found by the turn's host in `Next_Turn_` (`Search_For_Battles_`
0xe9d62). In game types 2/3 it does not fight them itself: it sets state
0xe and enters `Client_Combat_Loop_` (0xea2f1) like every client, and its
`Mox_Net_Update_` runs `Do_Net_Combat_Update_` (0xeaaa1) on every
`Mox_Update_` while the state is 0xe..0x22. When no battle is left and
every human's slot is back in state 0xe, the host's state becomes 0x23,
its loop ends and it sends **msg 0x1a** (len 0) to every human; that sets
state 7 on the clients and ends theirs.

- **AI or monster against AI or monster:** on the host, at once.
- **A human attacker:** on his machine. The host sends the targets and the
  star's state (msg 0x10), the human picks (msg 0x11), the host tells him
  to fight (msg 0x14).
- **An AI or a monster against a human:** tactical combat on the human's
  machine (msg 0x12); strategic combat on the host, which sends the human
  the result to look at (msg 0x16).
- **Human against human:** tactical: both machines (msgs 0x12, 0x14, 0x18,
  0x19, `Russ_Combat_`'s 0x40/0x41 and the lockstep 0x3e8..; not ported);
  strategic: the attacker's machine, the defender gets msg 0x16.
- **The Dragon against a human's colony:** the human is asked first
  (msg 0x23, the answer 0x24 or 0x25).

Every machine that fought sends the result to the host (msg 0x15,
`Send_Battle_Results_`); the host's `Decode_Battle_Results_` applies it.
The next broadcast carries the outcome to everyone.

## The slots

`0x1aaf8b` holds 15 slots of 12 bytes, one per owner (players 0..7,
monsters 8..14); the own block at `0x1ab03f` (the net state) has the same
layout:

| Offset | Own block | Field |
|---|---|---|
| +0 | 0x1ab03f | state |
| +1 | 0x1ab040 | i8 star of the battle |
| +2 | 0x1ab041 | i16 ship owner targeted, −1 none |
| +4 | 0x1ab043 | i16 planet targeted, −1 none |
| +6 | 0x1ab045 | i16 defender |
| +9 | 0x1ab048 | i8 the player the slot is busy with (own block: the attacker) |
| +0xa | | the result byte (`Decode_Battle_Results_`) |
| +0xb | | the argument of msgs 6, 7, 9 (diplomacy) |

`Host_Next_Turn_` zeroes the slots and sets each player's state 0xe and
busy −1 before `Next_Turn_`; after the broadcast state 4 and argument −1.
0x1ab024 is slot 12's busy byte (the Dragon's), which the Dragon cases
reset.

## Do_Net_Combat_Update_ (0xeaaa1)

Reentry-guarded (0x183579); a `Mox_Update_` first and at the end (the
shared tail at 0xea8be).

**Community patch 1.51: no battle inside a battle.** The guard stops only
a call inside itself. The host's own battle (`Client_Combat_Loop_` states
0x14..0x18) runs `Mox_Update_` all along, so its `Mox_Net_Update_` went on
picking battles and fought an AI's (`Do_1_Combat_` below) inside it, which
moved the current star (0x1aa3e0/0x1aa3e4) and the combat globals from
under the host's battle. In 1.51 every `Do_1_Combat_` of
`Client_Combat_Loop_` (0xea50d, 0xea5a6, 0xea63f, 0xea69b, 0xea72a) and of
`Do_Net_Combat_Update_` (0xeb121) goes through 0x404acf, which sets an
in-battle flag (0x404ace) around it and, entered with it set, exits with
"nested combat detected"; `Mox_Net_Update_` calls `Do_Net_Combat_Update_`
through 0x404b0e (0xf6e05), which skips it while the flag is set. The
other battles wait until the host's ends; the clients' messages queue.
`Do_Nonhuman_Attack_Against_Human_`'s strategic `Do_1_Combat_` is not
wrapped (it runs inside `Do_Net_Combat_Update_`). The port: `netDo1Combat`
(the exit is a panic: the skip makes it unreachable), `netGame.inBattle`. Over the players from the last, a `Mox_Update_`
each, the slot cases (jump table 0xeaa55, states 0xe..0x20):

- **0x11** (msg 0x11 came): defender −1: the slot 0xe, every slot busy
  with this player −1, 0x1ab04b 0, the player's bit of
  `star.IgnoreCombat` (+0x36) set. Else `IgnoreCombat` 0, the 6 bytes +2
  copied to the defender's slot, the other slots busy with the attacker
  −1, `GNN_Reports_Combat_(att, def)`, then: strategic (settings+0xd8):
  attacker 0x18, defender 0x19; an AI or monster defender: attacker 0x16;
  a human: attacker 0x14, defender 0x15 and `Send_You_Are_Target_Info_`
  to him. **Msg 0x14** (len 0) to the attacker.
- **0x1b** (results decoded): `Update_Player_Needs_To_Attack_(star, att,
  def)` (the slot's own player is the attacker when busy is itself), state
  0xe, busy −1, 0x1ab04b 0.
- **0x1d** (the Dragon refused): `Do_Nonhuman_Attack_Against_Human_`, then
  `0x1aa3b4[star] &= ~0x10`, state **0x19** (over whatever it set),
  0x1ab04b 0, 0x1ab024 −1.
- **0x1e** (the Dragon paid): `Do_Dragon_Bribe_(player, star, 0x1ab0cd)`,
  busy −1, state 0xe, 0x1ab04b 0, 0x1ab024 −1.
- **0x1f** (a long result waits): only while the host's own state is 0xe:
  own state 0x21, slot 0x20, **msg 0x31** to him.

Then, with `_n_battle_stars` 0: every human slot 0xe → state 0x23 (a
`Mox_Update_` per busy human). Else, unless the host's own slot is 0xf or
0x10 or 0x1ab04b is set, `Pick_Star_And_Attacker_`; nothing: 0x1ab04b 1.
A pick (0xeae5e):

- the slots of each ship target and each colony target's owner and of the
  attacker get the star and busy = attacker;
- a human attacker not ignoring combat there: slot 0x10,
  `Send_Pick_Target_Info_`. Ignoring: no target (below);
- else `Get_NPC_Target_` / `Get_AI_Target_` (its sneak flag is computed
  and then **not passed** to `Do_1_Combat_`: a quirk); a colony target's
  owner gets `CombatSlot` (+0x204) = attacker;
- the attacker's slot +2/+4/+6 = ship target, planet, defender;
- no defender: 0x1ab04b 0, every slot busy with the attacker −1;
- else `IgnoreCombat` 0, the 6 bytes to the defender's slot, the others
  busy with the attacker −1 (**community patch 1.51**, both loops: only
  the slots whose star, +1, is this battle's; hooks 0x402415 at 0xeb04c
  and 0x4023f5 at 0xeb0b8. In 1.40b23 an attacker in two battles at once,
  an AI or a monster with fleets at several stars, freed the human of his
  other battle, who could then be picked again while that battle ran: the
  battle rolled back); then an AI or monster defender:
  `Do_1_Combat_` on the host, `Update_Player_Needs_To_Attack_`, 0x1ab04b
  0, both busy −1; the Dragon (12) against a colony: defender slot 0x1c,
  `Send_Dragon_Info_`; else `Do_Nonhuman_Attack_Against_Human_`.

**Pick_Star_And_Attacker_** (0xe84a5) calls `Mox_Update_` per candidate
star (in every game type), and in game types 2/3 skips a star where
`Some_Star_Player_Is_Busy_` (0xe841a): a human with ships (+6) or colonies
(+7) there whose slot is busy, or the host itself in state 0x21/0x22 or
holding received chunks (0x1aaf72).

**Do_Nonhuman_Attack_Against_Human_** (0xea91b, the defender's index):
attacker = his slot's busy, `GNN_Reports_Combat_(att, def)`. Tactical:
`Send_You_Are_Target_Info_(def, att, slot+2, colony of slot+4, star)`, slot
0x17. Strategic: a zeroed 0x6ce record with the slot's 12 bytes at +0 and
the planet at +0xd, `Do_1_Combat_` on the host, `Update_Player_Needs_
To_Attack_`, 0x1ab04b 0, the attacker's busy −1, slot 0x19, **msg 0x16**
(the record) to the defender.

**GNN_Reports_Combat_** (0xea8ca): `Random_(2)` picks the order of the two
names (`Player_Name_` 0xe5de8: a monster's from E_Strings 0x28a + owner) in
COMBFIND string 0x1aa3dc; `Send_GNN_Chat_Msg_` (msg 0x2d to every human).

## Client_Combat_Loop_ (0xea2f1)

On entry 0x1ab04c points at a 0x6ce-byte frame buffer (msgs 0x16/0x18 copy
into it) and 0x1aa378 (the attacks a human confirmed) at a zeroed 0x40-byte
one; both are cleared on exit. While the state is 0xe..0x22: `Mox_Update_`,
`Chat_Box_Input_Loop_`, `Mox_Update_`, then the case (jump table 0xea291
over the ids at 0xea27a):

| State | Who | What |
|---|---|---|
| 0xf | attacker | wait for the chunks (msg 0x2f), `Decode_Pick_Target_Info_`, `Pop_Block_` |
| 0x10 | attacker | `Null_Autofunction_`, the mini main screen drawn to back, sound 0x33, `Get_Local_Human_Target_`; the planet, `Load_Net_Next_Turn_Screen_(0)`, state 0x11 and the defender (colony owner, else the ship target, else −1 and state 0xe); **msg 0x11** {ship target, planet, defender} to the host |
| 0x12 | defender | wait for the chunks (msg 0x35), `Decode_You_Are_Target_Info_` |
| 0x13 | defender | attacker ≥ 8 or an AI: 0x17, else 0x15; sound 0x33 |
| 0x14 | attacker vs human | `Do_1_Combat_` with the frame record (block at +0, planet at +0xd); **msg 0x18** (the record) to the defender; wait for state 0xe (msg 0x19); `Send_Battle_Results_` |
| 0x15 | defender vs human | `Do_1_Combat_`; state 0xe unless 0x1a; wait for 0x1a (msg 0x18); `Bomb_Results_Popups_`; **msg 0x19** to the attacker; `Send_Battle_Results_` |
| 0x16 | attacker vs AI/monster | `Do_1_Combat_`, state 0xe, `Send_Battle_Results_` |
| 0x17 | defender vs AI/monster | `Do_1_Combat_`, state 0xe, `Load_Net_Next_Turn_Screen_(1)`, `Send_Battle_Results_` |
| 0x18 | strategic attacker | `Do_1_Combat_` with the record, `Load_Net_Next_Turn_Screen_(1)`, a human defender gets **msg 0x16** (the record), state 0xe, `Send_Battle_Results_` |
| 0x19 | strategic defender | `Strategic_Results_Popup_`, `Bomb_Results_Popups_`, `Load_Net_Next_Turn_Screen_(0)`, state 0xe, **msg 0x17** to the host |
| 0x1c | Dragon's target | `Dragon_New_Star_Popup_`, `Load_Net_Next_Turn_Screen_(0)`; −1: state 0x1d, **msg 0x24**; else state 0xe, **msg 0x25** {s32 star} |
| 0x22 | host | wait for the chunks (msg 0x33), `Decode_Battle_Results_`, state 0xe |

`Send_Battle_Results_` gets (star, attacker, defender, ship target,
planet, result).

## The messages

| Id | Case | Payload | Effect |
|---|---|---|---|
| 0xf | 0xf5fd2 | — | slots zeroed, state 0xe (no sender found) |
| 0x10 | 0xf5fef | pick target info | state 0x10, `Decode_Pick_Target_Info_` |
| 0x11 | 0xf6009 | 6 bytes | the sender's slot +2..+7, slot 0x11 |
| 0x12 | 0xf6032 | you are target info | `Decode_You_Are_Target_Info_` |
| 0x13 | 0xf6045 | — | state 0x14 (no sender found) |
| 0x14 | 0xf6051 | — | strategic 0x18; an AI/monster defender 0x16; else 0x14 |
| 0x15 | 0xf609e | results | `Decode_Battle_Results_` |
| 0x16 | 0xf60b1 | 0x6ce record | copied to *0x1ab04c, state 0x19 |
| 0x17 | 0xf60ce | — | the sender's slot 0xe, busy −1, 0x1ab04b 0 |
| 0x18 | 0xf60f0 | 0x6ce record | copied to *0x1ab04c, state 0x1a |
| 0x19 | 0xf5fe3 | — | state 0xe |
| 0x23 | 0xf6406 | s32 colony | `Decode_Dragon_Info_` (planet, star to the block), state 0x1c |
| 0x24 | 0xf643d | — | the sender's slot 0x1d |
| 0x25 | 0xf644d | s32 star | the sender's slot 0x1e, 0x1ab0cd = star |
| 0x2e, 0x34, 0x32 | 0xf6594, 0xf6600, 0xf65e4 | u16 chunks | the chunk count, a block pushed; state 0xf / 0x12 / 0x22 |
| 0x2f, 0x35 | 0xf65b0 | a chunk | appended (msg 3's code) |
| 0x33 | 0xf5cb0 | a chunk | msg 3's case |
| 0x30 | 0xf65c8 | — | the sender's slot 0x1f |
| 0x31 | 0xf65d8 | — | state 0x20 |
| 0x3c | 0xf66c4 | 3 × s32 | `Event_Monster_Update_` (the low words) |
| 0x40, 0x41 | 0xf6711, 0xf671d | — | 0x1ab0c0 = 1, 2 |
| 0x43 | 0xf673e | 6 × s16 | `Change_Relations_` |

The info messages, built in a `Push_Block_` and sent whole up to 0x8000
bytes, else as a u16 chunk count and chunks of 0x7800 with chat loops
while sending (**community patch 1.51**, msgs 0x10 and 0x12 only: a
0xff00-byte block, whole up to 0x7800 bytes, one chat loop per chunk
without `Net_Is_Sending_`; 0xf890d, 0xf8bbf, 0xf8c48 and the same in
`Send_You_Are_Target_Info_`):

**Pick target** (`Send_Pick_Target_Info_` 0xf88d8, msg 0x10 / 0x2e, 0x2f):
s16 star, the 0x2c bytes of the targets (0x1aa380: colony targets[5],
count, ship owner targets[15], count). Unless the attacker's machine is the
host: the star's presence (`Send_Star_Player_Presence_` 0xe691a: the 0x58
bytes of `_star_has_player`[star] and the monster byte), each player's
0xd4 bytes from +0x117 (**community patch 1.51**: the 0x127 bytes from
+0xc4, the tech fields' status first; 0xf8991, and 0xf8c8e's decode)
(from the last player), every leader (u16 index,
0x3b bytes, from 0x42 down; then −1), the star's `Officer` bytes (+0x42,
from the last player), u16 `NUM_COLONIES`, per planet of the star from the
last orbit {planet +0 (colony), +4, +8, +0 again, the colony record if
any}, u16 `NUM_SHIPS`, the ships there not of status 5 {u16 index, 0x81};
−1. `Decode_Pick_Target_Info_` (0xf8c8e) writes it back on the client:
the block's star and attacker (= itself), `Copy_Target_Info_`; leaders'
owners all −1 before the list; each planet's colony index read, that
colony's owner and planet −1, then the index again and the record; **all
ships' status 5** before the list. Then `Update_Ship_Icons_In_Between_
Turns_`, own slot and state 0x10.

**You are the target** (`Send_You_Are_Target_Info_` 0xf8f98, msg 0x12 /
0x34, 0x35; arguments defender, attacker, ship target, colony, star):
s16 star, attacker, ship target, colony. Unless the defender's machine is
the host: the presence, the attacker's and the defender's 0xd4 bytes (each
if < 8; **community patch 1.51**: 0x127 from +0xc4, in both routines), the leaders of either (from 0x42, then −1), their `Officer`
bytes, with a colony {u16 `NUM_COLONIES`, u16 planet, planet +0, +4, +8,
the colony record}, u16 `NUM_SHIPS` and their ships there; −1.
`Decode_You_Are_Target_Info_` (0xf93e3): the block's star, attacker, ship
target, defender = itself; their leaders' owners −1 before the list, all
ships status 5 before the list; state 0x13, planet = the colony's.

**Results** (`Send_Battle_Results_` 0xf97cd, msg 0x15 / 0x30..0x33): s16
sender (`PLAYER_NUM`), attacker, defender, result. Only these 8 bytes when
the sender is the host, or the defender against a human attacker (the
attacker's results count). Else: s16 star, the presence of the battle star
(through 0x1aa3e0/0x1aa3e4), s16 ship target, s16 planet, per side < 8 its
0xd4 bytes and `Officer` byte, the leaders of either side, the planet
{+0, +4, +8, u16 colony, its record}, u16 `NUM_SHIPS`, the sides' ships at
the star; −1. Longer than 0x8000: own state 0x1f, msg 0x30, wait for
state 0x20 (msg 0x31), state 0xe, msg 0x32 {chunks}, msgs 0x33.
`Decode_Battle_Results_` (0xf9c58) on the host: both slots' result byte;
unless short: the presence into `_star_has_player`[star], the sides'
bytes, the leaders (no clearing), the planet: no colony any more destroys
the old one (`Destroy_Colony_(…, 1)`), else the record and
`Compute_Star_Colony_Stuff_`; `NUM_SHIPS`; ships of either side at the
star missing from the list (status not 5) get officer −1 and `Kill_Ship_`.
Then `Update_Ship_Icons_In_Between_Turns_` and the sender's slot 0x1b.

**The record** (msgs 0x16, 0x18) is `Do_1_Combat_`'s 0x6ce bytes: +0 the
caller's 12-byte block, +0xa result, +0xc outpost destroyed, +0xd planet,
+0xf the bombing record (0x49), +0x58 and +0x8f the ground forces
(`Get_Invasion_Info_`, 0x37 bytes each, memset first: +0 player, +2
strengths, +0xa counts, +0x12 hits, +0x16 class, +0x17 damage, +0x18 and
+0x19 the classes hurt and lost, +0x1a colony, +0x1c a pointer to the
ship list, +0x20 its length, +0x22 the enemy, +0x24 the bonus record),
from +0x58 on `Invade_`'s report: +0xc6 the ground battle's seed, +0xca
the captured applications (`Change_Colony_Ownership_` copies only as many
as it found), +0x272 their count, +0x274 the planet; +0x276 converted,
+0x277 Loknar's techs, +0x27f their count, +0x281 the end of combat title
(0x64), +0x2e5 its text, +0x6cd mind controlled. The buffer is
`Client_Combat_Loop_`'s stack frame, not cleared: the port writes the
parts it keeps and marks as stale the ship lists' pointers, the unwritten
application slots, the planet when nothing was captured, and +0xc6..
when there was no `Invade_` (netinv1 matches the rest byte for byte).

## Do_1_Combat_ and Russ_Combat_ in network games

- `Do_1_Combat_` (0xe938c) takes a record pointer (0: its own zeroed one).
  Only the host points 0x1aa3e0/0x1aa3e4 at the star's row; on a client
  they stay as they were: never set (the oracle's joiner kept the row and
  the monster byte apart, netbattle1) or a dead frame of an earlier
  `Search_For_Battles_`, which does not clear them in network games.
  `Decode_*_Info_` write the host's row there (`combatCur`).
- The defender at the controls, not strategic: `Message_Box_` with
  COMBFIND 0x1aa3bc ("%s attacks %s").
- After `Russ_Combat_`, when the attacker or the defender is the own
  player: `Load_Net_Next_Turn_Screen_(1)` (the fade), full redraw, a chat
  loop.
- Antares taken: the memset of 0x1aa3ac (0x90 bytes) on a client reaches
  whatever the never-set pointer reaches (modelled on the row).
- `Russ_Combat_` (0xe7343): `something_interesting_happened` (0x1ab124)
  for both sides in every game type; the defender's popups only outside
  network games; the strategic result box only on the attacker's machine;
  two humans in tactical combat: msg 0x40 to the other, wait for his,
  `Tactical_Combat_` (with 0x1998ce set: the lockstep), msg 0x41, wait,
  0x1ab0c0 0.
- `Russ_Change_Relations_` (0xf975c) and `Russ_Event_Monster_Update_`
  (0xf9711) send msgs 0x43 / 0x3c to the host in game types 2/3 (the host
  sends to itself: local delivery). Callers: `Do_Attacker_Beat_Colony_
  Stuff_`, `Apply_Bio_Weapon_Penalty_`, `End_Of_Combat_`,
  `Strategic_Combat_`, `Tactical_Combat_`, `Process_Retreating_Ships_`.
- `Do_Attacker_Beat_Colony_Stuff_` (0xe87d2): the defender's bombing and
  invasion screens in strategic combat only when the defender is the
  host's player (0xe88d8, 0xe913a, 0xe921f; others see them from msg 0x16)
  — the amoeba's climate change sits inside that test, so it is skipped
  for other defenders (quirk); a human attacker only on his own machine;
  after his choices `Load_Net_Next_Turn_Screen_(0)`.

## Popups from another machine

- **Strategic_Results_Popup_** (0xe859d): sound 0x33, the record's title
  and text in `Message_Box_Titled_`, Loknar (+0x27f) with
  `Play_Take_Orion_Flic_` and `Rescued_Loknar_Popup_`,
  `Load_Net_Next_Turn_Screen_(0)`.
- **Bomb_Results_Popups_** (0xe85f7): outpost (COMBFIND 0x1aa3d8) /
  planetary destruction / the bombing screen (the amoeba first); a ground
  battle when the attacker landed marines (+0x64, the attacker's force's
  marine count): `Colony_Combat_Screen_(+0x58, +0x8f, seed +0xc6, the
  colony of the block's planet)` between a saved and restored seed, then
  `Load_Net_Next_Turn_Screen_(1)`; the screen never follows the forces'
  ship list pointers, so the other machine's addresses do no harm; mind
  control (COMBFIND 0x1aa3b8).

## The lockstep (two humans, tactical)

Both machines run `Tactical_Combat_` (0x47939) with `_net_flag`
(0x1998ce) set: game type 2/3 and both sides humans (0x482d6). Ported in
`internal/game/netcombat_tac.go` and the tactical files.

- **Start** (0x48a96): `_new_message` (0x190460) cleared, **msg 0x3ed**
  {s16 `_combat_init`} to the other machine (a 1 received sets the
  initiative order there too), a barrier.
- **The barrier**, `Wait_Until_Net_Opponent_Finished_` (0x3fbfe, flag in
  eax): **msg 0x3ec** {s16 the other side's index} to the other machine,
  then input passes (with the flag: the combat screen and Ken text 0x37;
  without: `Mox_Update_`) until `_combat_done_waiting` (0x199f16, raised by
  his msg 0x3ec) is set; it is decremented. Every action is followed by one
  on both machines.
- **The moving side.** `Next_Ship_` (0x4576d) sets `_net_destination`
  (0x199908) to the side opposite the new ship's owner; that owner's
  machine sends **msg 0x3eb** {`_msl_impact_warning_flag`} (restored from
  `_saved_msl_warning` 0x199906 first), then a barrier. Each human action
  on his machine is sent first (when the destination is the other machine)
  as **msg 0x3e8** (`Send_Tactical_Combat_Message_` 0x42b0f): {s16
  `_cur_combat_x`, s16 `_cur_combat_y`, u32 `Get_Random_Seed_()`, s16 code,
  s16 bx, s16 cx, s16 arg}, then done, then a barrier.
- **The other side** (Do_Combat_Turn_ 0x43457; also for its ships under
  AUTO, 0x4331c): input passes until `_tactical_net_choice` (0x1999ac, the
  code from msg 0x3e8) is set; meanwhile its AUTO button calls
  `Toggle_Auto_Combat_Flags_`. Then the view centred on the sender's,
  `Set_Random_Seed_(seed)`, `_new_message` cleared and the action by its
  code (jump table 0x42f3b), `_tactical_net_choice` 0, a barrier.

| Code | Sent at | Replay |
|---|---|---|
| 0x2710 | self-destruct confirmed | `Destroy_Ship_` with the drive off, ship done, `Next_Ship_` |
| 0x2711 | a ship moved by the computer (AUTO) | `Do_Auto_Ship_Turn_` with the same seed, ship done, `Next_Ship_` |
| 0x2714 | DONE | ship done, `Next_Ship_` |
| 0x2715 {x, y} | a move | `Move_Cost_`, `Move_Ship_` (no `Assign_Combat_Grids_`), done when `Ship_Done_Check_` or the owner changed |
| 0x2716 {target, 1 missile / 0 ship} | fire | `Fire_Ship_`, fields, grids, mounts on |
| 0x2717 | missile warning | cycled |
| 0x2718 | AUTO | the side's AUTO flag toggled |
| 0x2719 | WAIT | `Next_Ship_` |
| 0x271a {mount} | a weapon mount toggled | the same |
| 0x271b | retreat | +0x4b 1, ship done, `Next_Ship_` |
| 0x271c {x, y} | turn to face | `Turn_To_Face_` |
| 0x271d {ship} | stasis released | `Remove_Stasis_Beam_` |
| 0x271e {ship} | another own ship taken (no initiative) | `Set_Cur_Ship_To_` |
| 0x271f {target, marines, kind} | boarding (`Initiate_Boarding_Action_`, `Resolve_Ai_Boarding_`) | raid or capture (the raid's cloak drawing compares the kind with `_combat_on_screen`) |
| 0x2720 {x, y} | end of turn | initiative: the side ends, `Next_Ship_`; else the turn ends |
| 0x2721 | all-wait, H (community patch 1.51) | the side waits, `Next_Ship_` from the current ship |

- **AUTO** off on one machine (`Toggle_Auto_Combat_Flags_` 0x29616) sends
  **msg 0x3ea** {0 attacker, 1 defender}; the receiver clears that flag
  (0x192b10 + 2·w) and `_auto_combat` when the current ship is that side's.
- **The missile impact flag**: the button sends msg 0x3eb at once; the
  receiver keeps its own in `_saved_msl_warn` (0x1998d0) and restores it at
  the end of the other side's turn (0x44e65), before the last barrier.
- `Init_Ship_For_Start_Of_Turn_` waits before a retreating ship leaves, with
  initiative only (0x42bc1).
- At the end `Tactical_Combat_` sets the full redraw flag to `_net_flag`.
- **The animations' speed.** `_net_flag` counts as double speed (MOX.SET's
  0x19987e == 1) in every effect that tests the speed: `Draw_Retreating_
  Ship_` (0x33043), `Draw_Bombs_` (0x33f97), `BHG_` (0x3b716),
  `Draw_Teleporting_Ship_` (0x3e688), `Test_Move_Until_Impact_` (0x3eb1f),
  `Beam_SFX_` (0xa8a69), `Draw_Anti_Missile_Rockets_`, `Draw_Gyro_Special_
  Effect_`, `Draw_Stasis_Beam_`, `Draw_Cloak_`, `Stellar_Converter_`,
  `Do_Spherical_SFX_` (0xae11c), `Destroy_Ship_FX_` (twice) and
  `Missile_Launch_FX_`, so both machines step their animations alike
  whatever their own settings. `Move_Ship_` (0x3ef80) moves 6 pixels a
  step in a network battle (else 12 at double speed, 2 at normal), and as
  `Defensive_Fire_Check_` runs every third step, that changes the draws:
  the netbattle4 mismatch (from the joiner's fourth AUTO turn the original
  fired first and the port moved first) was the port's 12-pixel step, one
  defensive fire roll short. `Do_Spherical_SFX_`'s shaking (two
  `Random_(5)` per ship and frame) depends on the frame step as well.
- **Community patch 1.51**: in 1.40b23 a human's boarding whose marines
  the target's Damper Field (special 7: a third, for a transporter
  boarding, special 0x23, from afar or onto a moving target) cuts to 0 became kind 0 (`xor edi, edi` at 0x45c68): it
  sent nothing but still waited at the barrier, and the other machine never
  met it (a deadlock). 1.51 NOPs that instruction: the boarding goes on with
  0 marines, msg 0x3e8 code 0x271f {target, 0, kind} is sent and both
  machines resolve it. A raid of 0 rolls nothing; a capture of 0 fails
  (`Resolve_Capture_` captures only with marines left), whatever the crew.
  It holds in every game type: the ship is marked as having boarded, a
  cloak drops and the report shows. `Resolve_Ai_Boarding_` keeps its kind
  0 (both machines replay it alike).
- **Community patch 1.51**: code 0x2721 is H, the all-wait
  (`docs/combat/tactical.md`): sent like WAIT (0x41b192), then the
  barrier. The receiver turns it into WAIT's 0x2719 with a flag
  (0x40b3c2, flag 0x45febd) that makes its `Next_Ship_` the all-wait
  (0x41b071); the port replays `allWait` for it.
- Quirk: `Resolve_Ai_Boarding_`'s msg 0x3e8 during a replayed
  `Do_Auto_Ship_Turn_` is overwritten when the replay ends.

## Tests

- Community patch 1.51: `TestNetSoak` `2+0ai-lockstep-allwait` (ship
  initiative on; each machine presses H on its first ship, then AUTO; the
  barriers carry the current ship and the all-wait flags too).
- Community patch 1.51 (`netp151_test.go`): `TestNetTargetInfoFields`,
  `TestNetTargetInfoChunks` (msgs 0x10/0x12), `TestNetBattleBusySlots`
  (the host starts a battle of an AI that another battle elsewhere holds:
  without and with a defender; the other battle's slot stays busy).
  `TestNetSoak` `2+6ai-nested` (`netSoakNested`): the host's player
  attacks the AIs' last colony star on the combat screen, the other human
  and the AIs another AI's star; the AIs' battles there wait for the
  other human's, whose result comes while the host still fights. Without
  the skip the host starts one inside its own and `netDo1Combat` stops.

- `TestNetSoak` `2+2ai-battles`, `3+1ai-battles` (strategic) and
  `2+2ai-tactical` (tactical, humans against AIs only): every other turn
  the host moves one warship of each human to another player's colony
  star before `Next_Turn_` (`netGame.preTurnHook`), every fourth one of
  each AI, at war, to a human's; target choice answers the first target
  (`combatUI`, so the tactical battle plays off screen with Auto). After
  every broadcast each client equals the host; after each full
  `Decode_Battle_Results_` the host's ships of both sides at the star and
  the planet's colony equal the fighting machine's (`netGame.battleHook`);
  the messages are counted (40 turns: 5..7 battles each, msgs 0x10, 0x14,
  0x15, 0x16, 0x43). Never reached in the soak: msg 0x12 (an AI attacking
  a human in tactical combat), the Dragon, the long results (msgs
  0x30..0x33), monster updates (0x3c); the oracle runs below reach them.
- `TestNetLoadOracle` netai1, netai2, netdrag1, netinv1, netorders1,
  netlong1 (`oracle.md`, phase 9): the original's COMBKEYS replayed
  (`netCombatScripts`), the real target popups clicked as the oracle did
  (`netPopupClicks`); every send and autosave equal (netorders1: and
  every draw; netlong1 up to the original's stall).
- `TestNetSoak` `2+0ai-lockstep`: two humans, tactical combat, warships
  moved at each other; both machines play every battle off screen with
  Auto (msgs 0x3ed, 0x3e8 codes 0x2718 and 0x2711, 0x3eb, 0x3ec). At every
  barrier the engine's records (`tactical.mem`) of both machines are equal,
  and the seed too from each battle's third barrier on (before the first
  action the machines' seeds differ, as in the original); `netGame.tacHook`.
  40 turns: 5 battles, 124 barriers. `2+0ai-lockstep-screen`: the same on
  the combat screen of both machines (`combatUIScreen`), the test clicking
  AUTO for its own ship: 128 barriers, equal. Both run on seeds 7100+: the
  first lockstep battle of the other soaks' galaxy (7000+) ends after an AI
  boarding in the same stall as netbattle4 (the quirks above). The other action codes (moves,
  fire, DONE, ... by hand) are not reached by a test.

- `TestNetBattleOracle` (build/oracle/netbattle1 and netbattle2,
  `oracle.md`): the original's network battles (a monster against the
  joiner; tactical with AUTO, and strategic with the 0x6ce-byte record in
  msg 0x16) replayed on two machines: all sends from Turn on equal,
  payloads included. It showed that a client's never-set 0x1aa3e0 and 0x1aa3e4 do
  not share bytes (the joiner's msg 0x15 carries the monster byte apart
  from the row) and that `Player_Name_` is the race's name.
- `TestNetCombatOracle`: the original's msgs 0x10, 0x12, 0x15 decode to
  exactly their payloads.
- `TestNetBattleOracle` netbattle3, netbattle4: the original's lockstep
  between two humans: msgs 0x10/0x11, the GNN line, the host's msg 0x12 to
  itself (8 bytes), 0x14, 0x40, 0x3ed, the barriers, 0x3eb and the first
  msgs 0x3e8 with their seeds are the port's, byte for byte. netbattle4:
  all 46 and 37 sends, up to where both machines of the original stall
  after the AUTO side's boarding (msg 0x3e8 code 0x271f sent during its
  `Do_Auto_Ship_Turn_`; the port stalls there too), and every `Random_` of
  both machines from the first replayed action to the end (11974 and 8520
  draws, the beam effects' included) equals the oracle's RNG.LOG
  (`netRNGAlign`, always on for netbattle4).

## Community patch 1.51: the tech fields in a battle

1.40b23's msgs 0x10 and 0x12 carry each player's record from +0x117 (the
applications on); the tech fields' status (+0xc4, 0x53 bytes) stays as the
fighting machine had it since the broadcast. A battle the host fought
earlier in the same combat phase (a capture or an invasion that gives a
tech, `Player_Gets_Tech_App_` advancing the field) leaves the fighting
machine with the new applications but the old fields. 1.51 sends from
+0xc4 for 0x127 bytes; the port does the same (`netTargetRec`). The
results (msg 0x15) keep 0xd4 bytes from +0x117 in 1.51 too: the next
broadcast carries the host's fields. The messages grow by 0x53 bytes a
player, hence 1.51's larger block and its chunking from 0x7800 bytes.
Test: `TestNetTargetInfoFields`, `TestNetTargetInfoChunks`.

## Long messages

An info message longer than 0x8000 bytes (pick target 0x2e/0x2f, you
are the target 0x34/0x35) is sent by the turn's host from
`Do_Net_Combat_Update_`, i.e. inside `Mox_Net_Update_`. Its chunk loop
waits on `Net_Is_Sending_` (0x1849a0) through `Chat_Box_Input_Loop_` and
`Mox_Update_`, but `Mox_Net_Update_` returns at once while its reentry
guard 0x183609 is set, so `Net_Update_` (`ComDoComms_`,
`UpdateTransportLayer_`) never runs and a 0x7800-byte chunk never
completes. netlong1 shows it (in both IPX roles): the host logs the first
msg 0x2f, the joiner never receives it, both wait. Smaller messages (msg
0x10 of 6831 bytes) and large ones sent outside `Mox_Net_Update_` (the
upload 0xe, the broadcast) go through. A client's results exceed 0x8000
only when the info before them did, so msgs 0x30..0x33 are not reachable
in the original over IPX. The port sends synchronously over TCP and
completes them; `TestNetLoadOracle/netlong1` compares the sends up to the
original's stall (the first chunk byte for byte) and checks that the
port's machines go on through 0x30..0x33.
**Community patch 1.51** takes `Net_Is_Sending_` out of the chunk loops of
msgs 0x2e/0x2f and 0x34/0x35 (one chat loop per chunk), which ends that
stall for them; the results' loop (0x30..0x33) keeps it.

## Open

- `Russ_Combat_`'s defender's popups and `Strategic_Results_Popup_`'s
  Loknar branch are not reached by an oracle run; nor are the lockstep's
  stasis release (0x271d) and missile warning button (0x2717), and a
  boarding by hand (`Board_Popup_`).

## Not ported

- Before the lockstep was ported the port's `Deploy_Ships_` lacked its
  `_net_flag` tests (both machines deploy as the attacker's): added.

