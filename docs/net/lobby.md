# Network game: the lobby and the start of a new game

The network lobby and the game start, ported from NETSTART.C and the network screens of
module 111 in `internal/game/netstart.go` and `netui.go`; the receive
cases in `netmox.go`; the broadcast in `broadcast.md`. Port-only pieces:
`ui.md`.

## The path

1. **Multi_Player_Screen_** (screen 0xf): the Network radio (settings
   +0x16 = 1) gives `_game_type` 2 and connection type 0. START → the new
   game screen (0xd); JOIN → screen 0x16. The modem radios are drawn
   disabled (frame 2) and have no field; a MOX.SET with a modem link is
   taken as the network (port-only).
2. **Newgame_Screen_** in game type 2: `Add_Fields_` (0xcd261) copies
   settings+0x17 (the "Moo II x" name `Multi_Player_Screen_` makes) to the
   game name at 0x1ab082 and adds a continuous string field for it (x+0xe1,
   y+0x18a, 0xa8 × 0x12, 8 characters); at the end (0xcd8b4) the name goes
   back to settings+0x17. ACCEPT → screen 0x15.
3. **Start_Net_Screen_** (0xfb7e5, screen 0x15), the creator:
   - state (0x1ab03f) 1, `Net_Init_Host_Election_`, the "initializing"
     screen (`Reload_Initializing_Net_Info_`, entry 0x19, faded in;
     port-only: skipped with TCP, `ui.md`, "No initializing screen");
   - `Net_Init_(conn, 0x17, Receive_Message_)`; failure: "Comm failure"
     (RSTRING 159), back to 0xf. (The Windows check is not ported.)
   - port-only: the "create a game" box (`ui.md`), then
     `Russ_Net_Create_Game_(0x17, name, 8, "", "myname")` (0xfaa71);
     failure: the message, `Net_Uninit_`, every net id −1, 0x1ab09e 0;
   - 0x1ab09e 1, own net id (0x1aaf5e) from `Net_Host_ID_`, 0x1ab0c1 1
     (nebulas), the creator's id at 0x1aaf68;
   - **waiting for joiners** (`Reload_Waiting_For_Joiners_Screen_`, entry
     0xf, START = MULTIGM 0x10 at x+0x9e, y+0x6a, hot key S;
     `Draw_Wait_For_New_Joiners_Screen_` "n humans have joined", H 0xf9 /
     0xfa): until START (with one station: "Really start with only 1
     player?", RSTRING 166) or the station count reaches settings+0xd5;
     then `Net_Disable_New_Players_`;
   - the humans (0x1aaf5d) = min(stations, settings+0xd5); players 0..n−1
     get the stations' ids in station order, PLAYER_NUM the own one (0 if
     none); NUM_PLAYERS = settings+0xd5;
   - msg 0 (0x60 bytes): s32 own id, u16 humans, 8 × s32 net ids, settings
     +2, +0xd4..+0xd9, +0xd3, then "May 22 2006" in a 50-byte buffer whose
     tail is the frame's stale stack;
   - the race-info flags (0x1aaf60), `_color_flag`, `_race_flag`, msg 0x2b
     due flags cleared, state 3, every banner 8,
     `Race_Selection_Screen_` for the own player, msg 0x2b due for him,
     objective 100, `Mox_Update_`, **msg 0 to all**;
   - "waiting for race info" (entry 0x18, H 0xfb) until every human's
     msg 1 is in;
   - `Init_New_Game_(difficulty, size, age, players)` (the 1.40 wrapper's
     Cluster size is not applied here), players past the humans net id −1;
     banners still 8 get a free one at random (reservoir over 7..0,
     `Random_(k)`);
   - "sending data" (`Reload_Sending_Data_Info_`, entry 0x1a, faded in),
     NUM_NEBULAS 0 unless 0x1ab0c1, each player's combat slot state 4 and
     argument 0xff, `num_net_players`, 0x1ab09c = own id, **the full
     broadcast**, msg 0x22 taker flag for himself, msg 0x36 flags cleared,
     state 4, `Init_Main_Screen_`, `Harold_Init_`, `Set_Defaults_`, the main
     screen's fade, the ship icons. `Screen_Control_` then goes to the
     reports (0x27).
4. **Join_Net_Screen_** (0xfaedb, screen 0x16), a joiner: state 5,
   `Net_Init_`, port-only "join a game" box, **the game list**
   (`Choose_Multi_Network_Game_Screen_` 0xf0c8e: up to 10 games, rows
   x+0x26..x+400 from y+0x40 every 0x1b, CANCEL MULTIGM 0x101 at x+0xbf,
   y+0x158; the row under the pointer cycles its colours
   (`Increase_Font_Color_Range_`), the green palette 0x183580 goes to
   colours 0xc0..; port-only: the list is asked for again every 60 frames,
   where IPX kept it current by itself), `Net_Join_Game_`; failure "Game is
   full (or nonexistent)" (160). Then "waiting to join" (entry 0x17, one
   hidden field over the screen) until msg 0 sets state 6 (a slot) or 0x2d
   ("Sorry, no more player slots", 163); 0x2c (msg 0x20: a loaded game):
   msg 0x42, then `Net_Pick_Position_` (`leaving.md`). In state 6: msg 0x42 {u32
   `_nebula_screen_seg`} to the creator, then `Net_Select_Race_`.
5. **Net_Select_Race_** (0xfaad8): `Race_Selection_Screen_`, objective 100,
   "generating map" (entry 0x1e), 0x1aaf70 = 0, state 7, msg 1 {own player
   record} to the creator; wait for msg 2; "getting data" (entry 0x1f,
   "waiting ...", then "Receiving chunk %d of %d", RSTRING 167 / 161)
   until all chunks are in; `Decode_Broadcast_Game_Data_Differential_(full)`,
   msg 0x36 flags cleared, state 4, `Init_New_Session_`
   (`Load_Nebula_Pictures_` + `Set_Defaults_`), then as the creator.

`_g_mp_screen` of these screens (`netScreen`): +0xf back, +0x2f the
animation (reloaded and drawn from frame 0 to the counter +0x1ea each
draw, the counter wrapping at the frame count), +0xf3/+0xf5 its centred
position, +0x10e font style 4, +0x10f the "getting" text position, +0x1e0
draw the background once more (`Draw_Join_Net_Screen_` when not fading).
Text: offset 0xf2, `Font_Colors2_(4, 0x9c, 0xf8)`.

## Requests (the race and banner screens)

In game types 2 and up `Race_Selection_Screen_` has no Esc and asks the
creator for a race before taking it (0x5c718, 0x5c985), and gives it back
on a cancel (0x5c850, 0x5ca03); `Flag_Screen_` asks for a banner
(0x5d1b3). A refusal shows Ken text 0x48 (race) / 0x69 (banner).

| Routine | Address | |
|---|---|---|
| `Request_Race_` | 0xfbee1 | waits while 0x1ab0bf; own race 0xe; taken: false; msg 0x3a {race} to the creator, wait for `_race_flag[race]`, true if the race is now his |
| `Unrequest_Race_` | 0xfbf76 | flag cleared, 0x1ab0bf 1, msg 0x3e {race} |
| `Request_Banner_Color_` | 0xfbe75 | taken: false; msg 0x2c {banner}, wait for `_color_flag`, true if his |

The waits spin on `Mox_Update_`; the port also reads the key queue so
the headless host can advance (`netWait`).

A refusal comes only from crossing requests: a race or banner the creator
already gave away is drawn taken and its key does nothing, so a joiner is
refused only when he asked before the creator's 0x39 / 0x2b reached him.
The creator then ignores his request (0x3a / 0x2c on a taken flag sends
nothing) and the decision for the other player, arriving later, ends his
wait with someone else's race or banner (netlobby3).

`Unrequest_Race_` runs when the naming popup or the custom race's picks
screen is left with Esc: msg 0x3e to the creator, who clears the flag and
sends 0x3d to every human and 0x3f back to the giver through the round
robin; the next request waits for that 0x3f (0x1ab0bf). The creator gives
his own race back the same way, to himself (netone1).

### The saved custom race

With a lastrace.rac the 1.40 patch adds a button for the saved race when
Custom is pressed, at 0x5cc51, jumping to a stub at 0xf4f8d inside
`Multi_Player_Screen_`: the button is added only while `_current_screen`
(0x199a08) is below 0xf. The network race screens run under screens 0x15
and 0x16, so the button is never there; the stub leaves eax (the x, 0x1dd)
as the field number. In a network game the saved race cannot be picked
(netone1: Custom shows the base races only). The code behind the button,
0x5ccc5, would ask `Request_Race_` for the race scan's leftover index 13
and give back the saved race's own number on a cancel; it is ported as
written and unreachable. Esc does nothing on the network race screens
(no Esc hot key in game types 2 and up), so custom mode is left only by
picking a base race.

## Receive cases (Main_Receive_Message_)

| Msg | Case | Effect |
|---|---|---|
| 0 | 0xf5adc | NUM_PLAYERS = +0x28; settings bytes; the date compared ("Incompatible program versions (host %s, you %s)", 152, exits); PLAYER_NUM −1; 0x1aaf68 = +0; humans = +4; player i's net id = u16 of +6+4i, PLAYER_NUM where it is own; state 6, or 0x2d without a slot |
| 1 | 0xf5bfe | race info flag; the payload over the sender's player record; his banner 8 if another player has it |
| 2 | 0xf5c5c | `broadcast.md` |
| 3 | 0xf5cb0 | `broadcast.md` |
| 0x2b | 0xf651d | {banner, p}: `_color_flag`, p's banner |
| 0x2c | 0xf6541 | {banner} if free: flag, the sender's banner, msg 0x2b due |
| 0x36 | 0xf661c | msg 0x22 taker flag for the sender |
| 0x39 | 0xf6638 | {race, p}: on the creator exits ("only 1 host allowed", 153); flag, p's race |
| 0x3a | 0xf6676 | {race} if free: flag, the sender's race, msg 0x39 due |
| 0x3d | 0xf66dd | flag cleared |
| 0x3e | 0xf66ec | flag cleared, msg 0x3d due |
| 0x3f | 0xf6705 | 0x1ab0bf 0 |
| 0x42 | 0xf6729 | u32 0: 0x1ab0c1 0 (no nebulas) |

Port-only guards: a short payload reads zeros past its end (the original
reads on in the receive buffer), and the cases that index the sender's
player do nothing for a sender outside the game (index −1).

## Quirks kept

- Msg 0's date buffer tail is stale stack (marked stale in the port's log).
- Msg 0x42 carries a heap pointer; the port sends the oracle runs' value
  0x006f5044.
- The round robin of `Mox_Net_Update_` sends a due 0x2b / 0x39 only when
  its counter (u8 at 0x18360a, incremented by every `Mox_Net_Update_`)
  modulo NUM_PLAYERS is that player. The creator's own 0x2b is due twice
  (its 0x2c, then once more after `Race_Selection_Screen_`), so the second
  one lands before or after msg 0 depending on the counter. The counter
  counts every `Mox_Update_`, also those of the input loops while a screen
  waits for a key, so in the original it depends on wall time: netlobby3's
  NET.LOG (which records the counter, `oracle.md`) shows every round robin
  send at counter − 1 ≡ its player (mod 8). The port runs the same rule on
  its own counter, whose count of polls differs; the original itself
  varies (net2t's and netlobby3's second 0x2b came after msg 0, netver1's
  before it), so the tests take a repeated 0x2b / 0x39 at any place.

## Tests

- `TestNetNewGame`: two machines on `host.Lockstep` over the in-memory
  transport from the multiplayer screen to the main screen; their states
  differ only in PLAYER_NUM, the local settings bytes 0xda.. and the two
  home stars' owners.
- `TestNetNewGameTCP`: the same over real TCP on localhost.
- `OPENORION2_NET_SHOTS=dir` saves each machine's screen before each scripted
  input of these tests.
- `TestNetNewGameOracle`: the same with net2t's settings (m0's
  SAVE11.GAM) and both machines' clock seeds: the creator's full broadcast
  equals the original's (203126 bytes), and each machine's sends (ids,
  destinations, payloads; msg 0's stale tail masked) equal the original's
  up to the broadcast, the round robin's resends aside.
- `TestNetReceiveRouting` now sees the ported cases through a hook.
- `TestNetLobby3Oracle`, `TestNetOneOracle`, `TestNetVersionOracle`:
  "Oracle runs (phase 9)" below.

## Version check

Msg 0 (and 0x2a for a loaded game) carries the build date "May 22 2006"
from 0x17a17c; the receiver formats its own copy at 0x17a0f0 with "%s"
and `strcmp`s. A mismatch ends the joiner's program at once with
"Incompatible program versions (host %s, you %s)" (RSTRING 152,
`Exit_With_Message_`), before he answers msg 0; the creator waits for his
race on "receiving race setups" (the original forever; the port ends with
the lost-player box, `hardening.md`). netver1: both copies patched on the joiner.

## Not done

- (Loading a network game is in `leaving.md`.)