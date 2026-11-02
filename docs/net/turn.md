# Network game: the turn

What happens from TURN on the main screen to the
reports of the next turn in a network game (`_game_type` 2), read from
`build/orion2.dis`. Ported in `internal/game/netturn.go` (the protocol)
and `netturn_screen.go` (the waiting screen and the chat); the receive
cases in `netmox.go`. The broadcast itself is `broadcast.md`.

## The path

1. **Do_Next_Turn_** (0x83411): in game types 2 and 3 the next screen is
   0x25 (`Net_Next_Turn_`) instead of 0xc, the return screen 0x27 (the
   reports) as always.
2. **Net_Next_Turn_** (0xfc470, screen 0x25):
   - a player who surrendered (player+0xe72 ≠ −1) runs
     `Human_Surrenders_` (msg 0x38, `leaving.md`), `Mox_Net_Uninit_` and
     goes to the main menu;
   - 0x1ab069 and 0x1ab054 to 0, `_chat_info` (0x1aa250) to a 0x480-byte
     frame buffer, `Load_Net_Next_Turn_Screen_(0)`, the auto function
     `Draw_Net_Next_Turn_Screen_`, `Add_Net_Next_Turn_Fields_`;
   - state (0x1ab03f) 8, own ready flag (0x1aaf7e) 1, own vote (0x1aaf76)
     the own player index if still −1;
   - **msg 4** {s32 own vote, u8 1} to every other human, players from the
     last down, with a `Chat_Box_Input_Loop_` after every player (human or
     not);
   - **the election**: from the last player down, over the humans only,
     every vote must be the same and none −1; then the host's net id
     (0x1aaf86) = that player's net id. A `Chat_Box_Input_Loop_` per try,
     until the host is known;
   - the host runs `Host_Next_Turn_`, the others `Client_Next_Turn_`;
   - 0x1ab054, 0x1ab069 and `_chat_info` cleared; not eliminated:
     `Mox_Play_Sound_(0x33)`, screen 0x27; eliminated: `Play_Loose_Flic_`,
     wait while sending, `Mox_Net_Uninit_`, game type 0, the main menu.
     Previous screen 0x25.
3. **Msg 4** (case 0xf5ce2) {vote, ready}: the sender's vote and ready
   flag; one's own vote becomes the sender's when it is −1 or larger
   (signed bytes). `Mox_Net_Update_` sends {own vote, own ready} to each
   other human whenever the vote last sent to him (0x1ab07a) differs, so
   the vote of the first human to press Turn spreads: he hosts.
   `Net_Next_Turn_`'s own sends do not update 0x1ab07a, so each turn the
   round robin sends msg 4 once more (net2t: the creator's two msgs 4).
4. **Host_Next_Turn_** (0xfbfe2):
   - the uploads in (0x1aaf89) = the players that need none (not human,
     himself, eliminated), each marked in 0x1ab06a; state 9, or 0xb when
     that is everyone;
   - until state 0xb: over the players from 0 up, one
     `Chat_Box_Input_Loop_` each, the first whose ready flag and msg 0xb
     flag (0x1ab072) are both set becomes the uploader (0x1aaf88, its 0xb
     flag cleared); then state 0xa, **msg 0xc** (len 0) to him, chat loops
     while state 0xa, `Decode_Game_Orders_` of the received block,
     `Pop_Block_`, chunks 0, uploader marked and −1;
   - `Update_Ship_Icons_In_Between_Turns_`, `Rebuild_WFI_List_` (a bare
     `ret`), 0x1ab04b 0, the 0xb4 bytes of combat slots (0x1aaf8b) zeroed,
     then per player state 0xe and busy-with (+9) −1; state 0xe;
   - **`Next_Turn_`** (no autosave and no end-of-game check in game types
     2/3);
   - 0x1ab09c = own net id (the next turn's diplomacy arbiter),
     `Net_Init_Host_Election_`, 0x1ab068 0;
   - each other eliminated player with a net id gets **msg 0x3b** (len 0),
     objective 0, net id −1, one human less, `Mox_Update_`;
   - **the broadcast** (`Broadcast_Game_Data_Differential_(0)`, a diff);
   - eliminated himself: return. Else the msg 0x22 taker flag for himself,
     the msg 0x36 flags cleared, state 4, each slot state 4 and argument
     −1, `Update_Ship_Icons_In_Between_Turns_`; one human left:
     `Mox_Net_Uninit_`, game type 0, `something_interesting_happened`
     (0x1ab124)[own] 1.
5. **Client_Next_Turn_** (0xfc2d2):
   - **msg 0xb** (len 0) to the host, chat loops while state 8 (msg 0xc
     sets 0xd);
   - chat loops while `Must_Broadcast_AI_Player_Data_` (0xfc299: any bit
     of 0x1ab055[k] for k, p < NUM_PLAYERS: a msg 0x22 still due);
   - `Send_Game_Orders_`, `Client_Combat_Loop_`;
   - `_temp_string` = "waiting ..." (RSTRING 167, 0x1ab108); chat loops
     until msg 2 announced the chunks, or the player was eliminated (msg
     0x3b) and then returns;
   - while chunks are missing: `_temp_string` = "have %d of %d chunks\n"
     (RSTRING 168, 0x1ab104; received, announced) and a chat loop;
   - `Decode_Broadcast_Game_Data_Differential_` with its "full" flag from
     a frame variable that is always 0 (a diff); a false result exits with
     "craziness" (0x17a18c); the msg 0x36 flags cleared, `Pop_Block_`,
     chunks 0, state 4, `Update_Ship_Icons_In_Between_Turns_`; one human
     left: `Mox_Net_Uninit_`, game type 0.
   `_temp_string` is not drawn by the waiting screen.
6. **The reports** run on every machine as in a single game.
   `Must_Display_Reports_` (0xfe86b) autosaves (`Do_Autosave_`, every
   stardate in game types 2/3) once no report is left.

## The upload (Send_Game_Orders_ 0xf7e95, msgs 0xd, 0xe)

Built in a `Push_Block_` of 0x55730 bytes:

| Field | Size |
|---|---|
| `_stardate` | 4 |
| `PLAYER_NUM` | 1 |
| own player record | 0xea9 |
| per star (NUM_STARS × 0x18, below) | |
| `NUM_COLONIES`, then from the last colony down each own one: u16 index, 0x169 bytes; u16 −1 | |
| `NUM_SHIPS`, then from 0 up each own ship whose status is not 5: u16 index, 0x81 bytes; u16 −1 | |
| from leader 0 to 0x42 each own one: u16 index, 0x3b bytes; u16 −1 | |

A star's entry, from a zeroed 0x6c0-byte frame buffer, stars from the last
down:

| Offset | Content |
|---|---|
| +0 | bits 0..9: the sender's bit of star +0x35, +0x36, +0x38, +0x39, +0x3b, +0x3c, +0x3d, +0x3e, +0x3f, +0x40 |
| +4 | the star's name, only when star+0x37 (`ColonizePlayer`) is the sender (a quirk: not the owner) |
| +0x13 | s32 `Officer[me]` (star+0x42+me) |
| +0x17 | `SurrenderTo[me]` (star+0x67+me) |

`Mox_Update_` runs after the player record, before `NUM_COLONIES`, after
the colonies' −1, after the ships' −1 and after the leaders' −1. Then msg
0xd {u16 (len + 0x77ff) / 0x7800} to the host and per chunk: `Mox_Update_`,
with the last chunk state 0xe and 0x1ab048 −1, `_temp_string` = "sending
chunk %d of %d\n", chat loops while sending (at least one), msg 0xe.

Msg 0xd (case 0xf5f1d) sets the chunk count, zeroes the received count and
length and pushes a 0xe1000 block; msg 0xe (0xf5f63) copies the chunk at
received × 0x7800, and with the last one bumps 0x1aaf89: state 0xb once it
reaches NUM_PLAYERS, else 9.

**Decode_Game_Orders_** (0xf83d8) on the host: the stardate is skipped,
the player record copied over player p; each star entry (the host's
NUM_STARS, from the last down) sets or clears p's bit of the ten bytes,
writes `Officer[p]` (one byte of the s32), the name when star+0x37 is p,
and `SurrenderTo[p]`; `NUM_COLONIES` and `NUM_SHIPS` are raised (signed)
to the client's; each colony is copied and its planet's colony index set;
p's ships missing from the list (status not 5) get officer −1 and
`Kill_Ship_`, the listed ones are copied; p's leaders missing from the list
are freed by `Ditch_Leader_` (0xf83b7: ETA 0, status −1, location −1,
owner −1), the listed ones copied. `Mox_Update_` after the player, the
stars, each colony, each ship and the leaders. Nothing is validated.

## The combat phase (Search_For_Battles_, phase 4 for the battles)

In game types 2/3 `Search_For_Battles_` (0xea1db) does not fight: state
0xe, `Client_Combat_Loop_`, then **msg 0x1a** (len 0) to every human.

- **Client_Combat_Loop_** (0xea2f1), on every machine: while state is in
  0xe..0x22: `Mox_Update_`, `Chat_Box_Input_Loop_`, `Mox_Update_`, then the
  state's case (jump table 0xea291 over the ids at 0xea27a: states 0xf,
  0x10, 0x12..0x19, 0x1c, 0x22 have cases; the others loop). Msg 0x1a
  (case 0xf610d) sets state 7 on the clients.
- **Do_Net_Combat_Update_** (0xeaaa1), run on the turn's host by
  `Mox_Net_Update_` in states 0xe..0x22, reentry-guarded (0x183579): a
  `Mox_Update_` per player over the slots (cases 0x11, 0x1b, 0x1d..0x1f);
  then with no battle stars left (`_n_battle_stars`) and every human's
  slot in state 0xe it sets state 0x23, which ends the host's loop. With
  battle stars left (and its own slot not 0xf/0x10, 0x1ab04b clear) it
  picks the next (`Pick_Star_And_Attacker_`); none: 0x1ab04b 1.

The host enters state 0xe before `Next_Turn_`, so during the turn's steps
every `Mox_Update_` may already run it (with no battle list yet: state
0x23 at once); `Search_For_Battles_` sets 0xe again.

Ported: the loop, the empty slots and the no-battle ending, the pick that
finds nothing. Port-only: a combat state of the loop, a slot case or a
battle to fight end the game with a "not ported" message instead of
waiting forever (phase 4).

## The waiting screen and the chat

- **Load_Net_Next_Turn_Screen_** (0xf3e42, fade in al): `Mox_Update_`,
  the chat string empty, `_mp_screen` = `_g_mp_screen` (0x1aacb4) zeroed;
  MULTIGM.LBX 0x2a (+0x23, the animation; x = (640 − w) / 2), 0x2b
  (+0x27), 0x28 (+0x1f), 0x2d (+0x93, the ready plate); +0x1e0 1, +0x1e1
  `Get_N_Remaining_Humans_`, the player colours, +0xbf = h(0x2a) +
  h(0x2b), y = max(0, (480 − h0x2a − h0x2b − h0x28) / 2), font style
  (+0x10e) 2; the auto function, `_FULL_DRAW_SCREEN_FLAG`, `Clear_Fields_`,
  the fade (only from the combat callers: not ported), the fields.
- **Get_Net_Next_Turn_Player_Colors_** (0xf31bb): per player three colour
  sets (+0x118, +0x198, +0x158) from `_main_palette_player_colors` (banner
  3 has its own); computed with font mode 0 (style is set after) and not
  read by the drawing.
- **Add_Net_Next_Turn_Fields_** (0xefcea): the colours of
  `Get_Main_Palette_Base_Colors_` (0xf30b9: banner 3 → 0xa, 0xe, 0x17;
  4 → 0x3b, 0x3c, 0x3d; else b = table 0x10010[banner], b + 2, b + 3);
  the chat line, a continuous string field of 0x50 characters after the
  width of "(name)  " at (x + 0x19 + w, y + boxH + 0xbb), 0x237 − w × 0x11
  (+0xbb); a hidden field over the screen, hot key "C" (+0xbd).
- **Draw_Net_Next_Turn_Screen_** (0xf1075): on a full draw the black
  screen and `Draw_Mini_Main_Screen_` behind; the animation from frame 0
  to the counter (+0x1ea); 0x2b and 0x28 when +0x1e0; per human not
  eliminated: his banner (`Draw_Multi_Player_Banner_`, laid out by row)
  while not ready, else the plate (`Draw_Fill_Seg_` 0xf2742, laid out by
  player index), his name at x = (p % 2) × 0x126 + x + 0x5c (also by
  player index) and his race right-aligned at x + 0xcd if the name is
  narrower than 0x98; the chat lines from y + h(0x2b) + 0xe every 0xc,
  "(%s)  %s" in the speaker's colours or "( GNN )  %s" (speaker 8), the
  0x28 picture redrawn clipped under the first line; "(name)  " before
  the chat field. `_mox_font_colors_offset` is set after the last
  `Font_Colors2_` and reaches the next one (kept).
- **Chat_Box_Input_Loop_** (0xf55a4): `Get_Input_`; the first time
  (+0x10c) the chat field becomes the active input with an empty string;
  the chat field or the hidden field returned: a non-empty line goes out
  (`Send_Chat_Msg_` 0xdd3b8: **msg 0x27** with the NUL to every human,
  oneself included, `Mox_Update_` after each) and the field is active
  again; then the screen is drawn and the pages toggled. Port: the field
  is re-activated through `Engine.ActivateInputField`, which also resets
  the cursor's blink where the original keeps it when nothing was sent.
- **Msg 0x27** (0xf648e) / **0x2d** (0xf657d), only while `_chat_info` is
  set: `Receive_Chat_Msg_` (0xdd351) adds the line with the sender's
  player index (8 for GNN); a full box (14 lines of 0x52 bytes, count at
  +0x47c) scrolls up by one. `Send_GNN_Chat_Msg_` (0xdd42a: msg 0x2d, the
  line cut at 0x50) is ported for its phase 4 caller.

## Other routines

- **Mox_Net_Uninit_** (0xfaa94): `Net_Uninit_`, all eight net ids −1,
  0x1ab09e 0.
- **Draw_Player_Done_Lights_** (0x8520c): in game types 2/3 a player is
  done when his ready flag (0x1aaf7e) is set (hotseat: his status).
- **Msg 0x3b** (0xf66a7): the own player's eliminated flag.

## Tests

- `TestNetTurnsOracle`: both machines on `host.Lockstep` replay
  build/oracle/net2t from the multiplayer screen through its two turns
  (the creator presses Turn first; the joiner once its round robin has
  sent its vote back, as the oracle's 50 s later; the research choice is
  the first row of the first box, m0/SCR05.png). Each machine's sends from
  the first broadcast on equal the original's NET.LOG records, payloads
  included (each broadcast's chunks compared as one; runs of msg 0x36 in
  any order, where the round robin starts depends on every `Mox_Update_`
  before), and every per-turn autosave of both machines equals the
  original's but the description. Each machine uses its own settings
  (m1's local bytes 0xda.. differ).
- `TestNetTurns` (three turns), `TestNetTurnsTCP` (three turns over real
  TCP on localhost).
- `TestNetSoak`: 2 and 3 machines with every player human, and 2 humans
  with 2 AIs; random events and Antarans off; 40 turns (6 with -short;
  `OPENORION2_NET_SOAK=n` for more: 150 pass). The test answers the screens
  (research: the first row; the colony screen: Esc; Turn on the main
  screen, pressed again if an ambassador's screen took it; anything else:
  Enter, Esc, its first field). After every broadcast each client's state
  equals the host's at its broadcast but for `PLAYER_NUM`, the local
  settings bytes 0xda.., each viewer's star owners and the blocks the
  broadcast does not carry (`something_interesting_happened`, set by the
  host's AI turn). With AIs the only "not ported" message is the report
  phase's AI diplomacy (phase 5); an AI's ambassador still opens on the
  human's own machine there.

## Not ported (later phases)

- Battles of any kind in a network game (phase 4): the loop's combat
  states, the slot cases, `Do_1_Combat_` from `Do_Net_Combat_Update_`,
  the fade of `Load_Net_Next_Turn_Screen_`.
- (`Human_Surrenders_`, msg 0x38, is in `leaving.md`; msg 0x22 and AI
  diplomacy through the arbiter in `diplomacy.md`.)
- No oracle run with three machines or with AI players yet.
