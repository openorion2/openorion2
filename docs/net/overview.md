# Network games in the original: the game logic

What the original does on top of its network layer, read from
`build/orion2.dis`. The transport (IPX, modem, null modem, packets, acks,
Greenleaf, `ComSend`/`SendPDU`) is left out: the port replaces it (see
`README.md`). Lines marked *(inference)* are not read directly from the code.

## Roles

- **The host is elected every turn** (`Net_Next_Turn_` 0xfc470, msg 4 handled
  at 0xf5ce2).
  - Pressing Turn sets the ready flag (0x1aaf7e). If the vote (0x1aaf76) is
    still −1, the player votes for himself.
  - He sends msg 4 {vote, ready} to every other human. `Mox_Net_Update_`
    (0xf69fe) resends it whenever the vote changes.
  - A receiver takes the sender's vote when its own is −1 or larger.
  - Once every human holds the same vote, the host's net id (0x1aaf86) is
    set and the host runs `Host_Next_Turn_`; the others run
    `Client_Next_Turn_`.
  - So the first human to end the turn hosts it, ties going to the lower
    index *(inference from the min-vote rule)*.
  - `Net_Init_Host_Election_` (0xfc6a5) resets the votes every turn.
- **The host simulates the turn** (`Host_Next_Turn_` 0xfbfe2). It calls
  `Decode_Game_Orders_` for each client, then `Rebuild_WFI_List_`, then the
  full `Next_Turn_` (0x13870): AI, production, movement, events and combat.
  Clients never run `Next_Turn_`.
- **Diplomacy arbiter.** The host of the last turn keeps its net id in
  0x1ab09c (0xfc185). Clients learn it from msg 2 (0xf5ca0). During the
  next turn, requests to talk to an AI go to this machine.
- **The host leaving.** Nobody takes over in the middle of a turn. The next
  election picks a new host, but a failed send ends the game for everyone
  (see Drop-outs).

## A turn

1. **Playing.** All humans play at the same time, each on his own copy of
   the game. A chat box (`Chat_Box_Input_Loop_`) is open while they wait.
2. **Client** (`Client_Next_Turn_` 0xfc2d2).
   - It sends msg 0xb (ready) to the host and waits for msg 0xc (go).
   - It also waits until any pending broadcast of AI player data has gone
     out (`Must_Broadcast_AI_Player_Data_` 0xfc299).
   - Then it calls `Send_Game_Orders_` and enters `Client_Combat_Loop_`.
3. **Host.** It sends msg 0xc to one ready client at a time and decodes that
   client's upload before letting the next one send (0xfc061–0xfc130).
4. **The upload** (`Send_Game_Orders_` 0xf7e95). It is not a list of
   commands but a dump of everything the sender owns, sent as msg 0xd (the
   chunk count) then msg 0xe (chunks of up to 0x7800 bytes):
   - the stardate and the player number;
   - the whole player record (0xea9 bytes);
   - 24 bytes per star: the sender's bit of the star bitmasks +0x35..+0x40,
     `officer[me]`, `surrender_to[me]`, and the star's name when star+0x37
     is the sender;
   - each colony he owns (index, then 0x169 bytes), ended by −1;
   - each ship he owns whose status is not 5 (index, then 0x81 bytes), ended
     by −1;
   - each leader he owns (index, then 0x3b bytes), ended by −1.
5. **Applying the upload** (`Decode_Game_Orders_` 0xf83d8). The host
   overwrites its copies with the upload:
   - the sender's ships missing from the list are killed (`Kill_Ship_`);
   - his missing leaders are freed;
   - `NUM_SHIPS` and `NUM_COLONIES` are raised when the client's counts are
     larger;
   - the stardate is ignored.

   Nothing is validated.
6. **The broadcast** (`Broadcast_Game_Data_Differential_` 0xf74cd, msgs 2
   and 3 to everyone). It carries the whole game state:
   - the stardate, the settings bytes and **the host's RNG seed**;
   - players, stars, planets, colonies, ships, leaders;
   - `_reports`, `_event_data`, `_msgs`, the council flags, `_antarans`,
     the nebulas and the event timers.

   The first broadcast of a game is full: flag 1, sent from
   `Start_Net_Screen_` or `Load_Net_Screen_`. Later ones are a byte-run diff
   against the previous broadcast (`Make_Difference_Buffer_`), which is kept
   in memory or in `modem.tmp`.
7. **Clients apply it** (`Decode_Broadcast_Game_Data_Differential_`
   0xf79f6). Each applies the diff and calls `Set_Random_Seed_`. Then it
   recounts the network humans and turns eliminated ones into AI.

## Combat

Battles are fought inside the host's `Next_Turn_`. In network games
`Search_For_Battles_` hands them to `Client_Combat_Loop_` (0xe9fd5,
0xea1e7). On the host, `Mox_Net_Update_` drives `Do_Net_Combat_Update_`
(0xeaaa1), which keeps a 12-byte slot per player at 0x1aaf8b: state, star,
target and busy-with. Battles whose participants do not overlap may run
together *(inference from the busy marker at 0x1aaf94)*.

- **A human attacker** (0xeaf3a).
  - The host sends `Send_Pick_Target_Info_`: msg 0x10, or 0x2e and 0x2f in
    chunks. It carries the star's presence record
    (`Send_Star_Player_Presence_`) and the colonies, ships and leaders
    there, since the client has not seen this turn's movement yet.
  - The client picks with `Get_Local_Human_Target_` and answers msg 0x11.
    An answer of −1 sets `ignore_combat`.
- **Human against human.**
  - The host sends `Send_You_Are_Target_Info_` to the defender (msg 0x12,
    or 0x34 and 0x35) and msg 0x14 to the attacker (0xeac98).
  - Both machines run `Do_1_Combat_`. `Russ_Combat_` (0xe7343) handshakes
    with msg 0x40 before `Tactical_Combat_` and msg 0x41 after it.
  - The battle runs in **lockstep**. Each action is a 16-byte msg 0x3e8
    {cursor, `Get_Random_Seed_()`, action code 0x2710..0x2720, arguments}
    (`Send_Tactical_Combat_Message_` 0x42b0f). The receiver calls
    `Set_Random_Seed_` with the packet's seed and then replays the action
    (`Do_Combat_Turn_` 0x43502).
  - Other combat messages: 0x3ea toggles AUTO, 0x3eb passes the turn, 0x3ec
    means finished (`Wait_Until_Net_Opponent_Finished_` 0x3fbfe), 0x3ed is
    a flag.
  - Afterwards the attacker sends a 0x6ce-byte result to the defender
    (msg 0x18), and the defender acknowledges with msg 0x19.
  - Both then call `Send_Battle_Results_` (0xf97cd). It leaves out the
    objects when sent by the host, or by the defender against a human
    attacker, so the attacker's state is the one that counts (`combat.md`).
  - The host applies the results in `Decode_Battle_Results_`
    (`Kill_Ship_`, `Destroy_Colony_`, `Compute_Star_Colony_Stuff_`).
- **A human against an AI or a monster.** The human's machine computes the
  battle (client states 0x16 and 0x17) and uploads the result. Relation
  changes and monster updates reach the host as msg 0x43
  (`Russ_Change_Relations_`) and msg 0x3c (`Russ_Event_Monster_Update_`).
- **AI against AI** is fought on the host (0xeb121).
- **Strategic combat** (settings+0xd8, 0x199cb4). The attacker's machine,
  or the host through `Do_Nonhuman_Attack_Against_Human_` (0xea91b),
  resolves the battle. The defender only gets msg 0x16 for
  `Strategic_Results_Popup_` and answers msg 0x17.
- **The Dragon** (attacker 12). `Send_Dragon_Info_` sends msg 0x23. The
  answer is msg 0x24 (refused: the attack happens) or msg 0x25 with a bribe,
  which `Do_Dragon_Bribe_` applies.
- **End of the combat phase:** msg 0x1a to everyone (0xea1f5).

## Diplomacy

- **With an AI.** The request goes to the arbiter as msg 5. Only one human
  talks to a given AI at a time (`Get_Next_AI_Talker_` 0xfa1a3; the
  priority comes from player+0x8b2 and +0x8b3).
  - The answer is msg 6 (go ahead, state 0x28) or msg 7 (wait in the queue,
    state 0x26).
  - Msg 8 cancels a request; msg 9 confirms the cancel.
  - The talk ends with `Net_End_AI_Diplomacy_` (0xfa2c5): msg 0xa with both
    player records and the surrender bytes.
  - The arbiter sends both records to the other humans as msg 0x22
    (`Mox_Net_Update_` 0xf6c32). Clients hold back their upload until that
    broadcast has gone out.
- **Human to human** (`Net_Request_Human_Diplomacy_` 0xfa3f6).
  - **Requesting.** The request is msg 0x1b with the requester's player
    record. It is refused once the other side has ended the turn.
  - **Answers.** Two crossing requests, or an accept (msg 0x1c), put both
    sides in state 0x2e. If the other side ignores the requester
    (player+0xe60), he answers msg 0x1d (rejected). Msg 0x1f cancels a
    request, and msg 0x44 exchanges the player records.
  - **The talk.** It is a dialogue in turns over msg 0x7d0, an 8-byte packet
    {speaker, listener, choice, argument}:
    - code 0x3f4 is the first sync (`Diplomacy_Initial_Synch_Up_` 0x1ed9c);
    - code 0x3f3 hands over the turn (`Diplomacy_Net_Switcheroo_`
      0x1ed17);
    - the side waiting for its turn sits in `Diplomacy_Synch_Mode_`.
  - **Chat** inside the talk is msg 0x834.
  - **The treaty.** `Start_Net_Treaty_` (0x1f17d) applies an agreed treaty
    on the local machine *(inference: on both)*. The host learns it from the
    next upload.

## Random numbers and desync

There are no checksums, and only two things keep the machines in step:

- every turn the host's state and seed overwrite every client, so a client
  never drifts for more than one turn;
- within a turn, tactical combat is the only lockstep part, with the seed in
  every packet.

There are only two checks:

- on joining, a build-date string ("May 22 2006", in msg 0 and msg 0x2a)
  is compared, and a mismatch exits with "incompatible versions". Port-only:
  the port's release follows the date ("May 22 2006 v1.0.0",
  `update.Version`, `Game.release`), so two releases of the port do not
  play together (`TestNetReleaseMismatch`);
- a diff that fails to decode exits with "error decoding broadcast" or
  "craziness" (0xf7a63, 0xfc3fa).

## Drop-outs, saves, setup

- **Quitting.** Quit in the game menu saves SAVE10.GAM, calls
  `Human_Abdicates_` and then `Net_Uninit_` (0x7dea8). That sends msg 0x37
  with the player record (msg 0x38 when surrendering). The others run
  `Make_Player_Into_AI_`, mark him ready, and count him as having uploaded
  (`Decode_Drop_Out_Info_` 0xfa8a2).
- **Elimination.** The host sends msg 0x3b to eliminated humans (0xfc1d7).
  When one human is left, `Mox_Net_Uninit_` runs and the game type becomes
  0, so the game goes on as a single-player one.
- **Losing the connection.** A failed send goes straight to
  `Exit_With_Message_` (`Mox_Send_Message_` 0xf6995), and the game ends for
  everyone. `lost_net_user_error` is loaded but never used.
- **Autosaves.** Every machine autosaves every turn: `Must_Display_Reports_`
  (0xfe8b2) calls `Do_Autosave_` with modulus 1 for game types 2 and 3.
- **Loading** (`Load_Net_Screen_` 0xfb27b).
  - The host picks the human slots and sends msg 0x20 with all player
    records.
  - Each joiner claims an empire with msg 0x21 (`Net_Pick_Position_`).
  - Unclaimed empires become AI. Then come msg 0x2a and a full broadcast.
- **New game.**
  - The creator sends msg 0 with the settings and the net ids.
  - Each joiner picks a race and sends msg 1 (`Net_Select_Race_`).
  - The host runs `Init_New_Game_` with the setup bytes, gives random
    banners to those still without one, and makes a full broadcast.
  - Race and banner requests are msgs 0x3a, 0x3e and 0x2c. The host's
    decisions are msgs 0x39, 0x3d and 0x2b.
  - Joiners send msg 0x42. If any of them sends 0, the host turns nebulas
    off *(inference: a capability flag)*.
- **Chat.** Msg 0x27 goes to all humans. GNN chat is msg 0x2d, at most 80
  characters, shown as speaker 8.

## Message ids

`Receive_Message_` (0xf67c1) routes messages by id:

- below 1000 to `Main_Receive_Message_` (jump tables 0xf594c and 0xf598f);
- 1000..1999 to tactical combat;
- 2000 and up to diplomacy.

Destination 0x7ffe means every human. A message addressed to oneself is
delivered locally.

| Id | Meaning |
|---|---|
| 0 | game setup |
| 1 | race picked |
| 2 / 3 | broadcast: count / chunk |
| 4 | host vote |
| 5..0xa | AI diplomacy |
| 0xb / 0xc | ready / go ahead with the upload |
| 0xd / 0xe | upload: count / chunk |
| 0xf | slot reset (no sender found) |
| 0x10, 0x11, 0x2e, 0x2f | pick a target / the choice |
| 0x12, 0x34, 0x35 | you are the target |
| 0x13 | combat state signal |
| 0x14 | start combat |
| 0x15, 0x30..0x33 | battle results |
| 0x16..0x19 | result buffers and acknowledgements |
| 0x1a | combat phase over |
| 0x1b..0x1f, 0x44 | diplomacy between humans |
| 0x20 / 0x21 | pick your position / claim |
| 0x22 | an AI's player record, sent again to everyone |
| 0x23..0x25 | the Dragon |
| 0x26 | a diplomacy flag (meaning unclear) |
| 0x27 / 0x2d | chat / GNN chat |
| 0x2a | the loaded game's net id table |
| 0x2b / 0x2c | banner colour |
| 0x36 | ready to receive |
| 0x37 / 0x38 | drop-out |
| 0x39, 0x3a, 0x3d..0x3f | race requests |
| 0x3b | eliminated |
| 0x3c | monster update |
| 0x40 / 0x41 | tactical combat start / end |
| 0x42 | join flag |
| 0x43 | relations changed |
| 0x64 | declare war (a handler exists; no sender found) |
| 0x3e8, 0x3ea..0x3ed | tactical combat |
| 0x7d0, 0x834 | diplomacy dialogue, its chat |
