# Network game: leaving, loading and the end

Leaving, loading and the end of a network game, read from `build/orion2.dis`. Ported in
`internal/game/netleave.go` (leaving), `netload.go` (loading) and the
receive cases in `netmox.go`.

## Leaving

### Quit (msg 0x37)

`Do_Main_Game_Popup_` (0x7dd41), QUIT and Yes: `Save_Game_(9)` (SAVE10.GAM
with the player still human), `_settings`+0x15 = 10,
`Save_Session_Related_Settings_`, then in game types 2 and 3
`Human_Abdicates_(PLAYER_NUM)` and `Net_Uninit_` (0x7de8f..0x7dead), and the
quit screen (7). Nothing else: the program ends.

### Surrender (msg 0x38)

`Net_Next_Turn_` (0xfc470) starts with the own player's `surrender_to`
(player+0xe72): unless −1, `Human_Surrenders_(PLAYER_NUM)`,
`Mox_Net_Uninit_`, screens 0xa (main menu). The main screen presses Turn
by itself for a player who surrendered (0x86574), so the next Turn after
a surrender to another human leaves the network game.

### Send_Drop_Out_Info_ (0xfa7b0)

`Human_Abdicates_` (0xfa97c) and `Human_Surrenders_` (0xfa989) call it with
msg id 0x37 or 0x38 (the latter jumps into the former's body).

1. player+0x2b (net id) = −1, `Make_Player_Into_AI_(p)` (a `Random_(5)`
   objective, ship designs, colony queues, colony calculations);
2. the payload, 0xef1 bytes in its frame: the player record (0xea9 bytes,
   after the conversion) and per star, from the last down,
   star+0x67+p (`surrender_to[p]`); the bytes for stars past NUM_STARS up
   to 72 are the frame's old stack (marked stale);
3. a flag per player (memset 1 over NUM_PLAYERS), the own one cleared;
   passes from the last player down until a pass finds no flag:
   - net id −1: flag cleared;
   - 0x1ab094[k] (msg 0x36 received from k: he takes msg 0x22) zero: the
     flag stays and the pass goes on; the original spins without pumping
     anything here (*port-only*: `Net_Update_` and a frame, so that msg
     0x36 can come in);
   - else the message to k's net id, `Mox_Update_` while `Net_Is_Sending_`,
     flag cleared.

### Decode_Drop_Out_Info_ (0xfa8a2), cases 0x37 and 0x38 (0xf662b)

With p the sender's player (`Net_Id_To_Player_Id_`):

1. the diplomacy arbiter 0x1ab09c = −1 if it was p's net id;
2. `Make_Player_Into_AI_(p)` (a draw on this machine too), then the
   record copied over player p, and the star bytes into star+0x67+p;
3. the human count 0x1aaf5d − 1, p ready (0x1aaf7e[p] = 1);
4. on the turn's host (0x1aaf86 = own id): the uploader 0x1aaf88 = −1 if
   p, p's msg 0xb flag cleared, the uploads in (0x1aaf89) + 1, state 0xb
   once that is NUM_PLAYERS.

The election only counts humans (objective 100), so the next Turn goes on
without him. When one human is left, the end of `Host_Next_Turn_` (and of
`Client_Next_Turn_`) runs `Mox_Net_Uninit_` and sets game type 0: the game
continues as a single-player game (`turn.md`).

Quirks kept: a drop-out of the player the host is taking the upload from
(state 0xa) clears the uploader but leaves state 0xa unless everybody is
in, so the host waits on; a player who uploaded and then quits counts
twice.

### Elimination (msg 0x3b)

Ported in phase 3 (`turn.md`): the host sends msg 0x3b to each other
eliminated player with a net id, sets him objective 0 and net id −1 and
counts one human less; the eliminated machine sees player+0x24 set, leaves
`Client_Next_Turn_`, plays the loser's video, runs `Mox_Net_Uninit_`, game
type 0 and the main menu. A client's broadcast decode zeroes the objective
of every eliminated player; the host keeps its own (a surrendered human is
eliminated by `Do_Surrenders_` with his AI objective on the host and 0 on
the clients).

### Port-only

- The star topology: the creator's machine relays every message. Since
  phase 7 a creator who quits leaves the relay running for the others, who
  go on as in the original (`hardening.md`); a lost creator ends the game.
- When the creator is left as the only human (the others quit or were
  eliminated) he runs `Mox_Net_Uninit_` and leaves the relay, which
  closes once the eliminated stations have gone. An
  eliminated joiner still on his way out (the loser's video) does not
  take that for a lost game (`netCheckLost`).

### Tests (leaving)

`TestNetSoak` `3+1ai-quit`, `2+1ai-quit` (the last joiner quits through the
game menu on turn 3: game menu, QUIT, Yes) and `3+0ai-surrender` (he gave
up to player 0 and presses Turn): every other machine decodes the drop-out
once, has him as an AI with net id −1 (eliminated, after a surrender), and
every client equals the host after each broadcast; with two humans the
remaining one plays on in game type 0.
`TestNetQuitOracle` replays `build/oracle/netquit1` (`oracle.md`) from the
load: the joiner quits through the game menu (G, Q, Y), the host then
plays a turn alone. Both machines' sends (the joiner's msg 0x37 with his
record and star bytes; the host's msg 0x1a and broadcast diff to nobody)
equal the original's, payloads included, and so do the joiner's QUIT save
and the host's F10 save after the turn (game type 0: the host went on as a
single-player game).

`TestNetElimOracle` replays `build/oracle/netelim1` (the joiner without a
colony is eliminated by the host's turn): every send of both machines
(msg 0x3b among them) and the host's F10 save after the turn (game type
0) equal the original's, and the joiner ends at the main menu.

### Three machines: netquit3

`build/oracle/netquit3` (m0..m2, battle.GAM = netload3's three-human save,
MOX.SET, run.sh, timeline.txt; machine 2 starts 2 s after machine 1): all
three load the save (the third human picks his own empire; with every
human placed the creator's waiting screen starts by itself). Then:

1. the second joiner quits (G, Q, Y): his msg 0x37 goes to each other
   human in turn (`Send_Drop_Out_Info_`'s passes), both decode it;
2. the creator and the first joiner play a turn (the creator hosts it;
   the election only counts the two humans left);
3. the creator quits: msg 0x37 to the joiner. Nothing in the original
   depends on who created the game: the IPX game simply goes on;
4. the joiner presses Turn alone and hosts it (msg 0x1a and the diff go to
   nobody), `Mox_Net_Uninit_`, game type 0; F10.

`TestNetQuit3Oracle` replays it from the load: every send of the three
machines, the quitters' QUIT saves, the last machine's autosaves and F10
save, and the joiners' waiting screens (`ui.md`, "Screen comparisons"). It
found two bugs: the main screen's GAME did not reset `_screen_data[0]`
(0x86bc9), so after the colony screen of star 3 the creator's game popup
opened on its save page; and the waiting screen placed a banner by the
human's row instead of his player number (0xf2845). Over TCP the creator's
quit leaves the relay running (phase 7), so the port plays it the same.

`TestNetSurrenderOracle` replays `build/oracle/netsur1` (the joiner of
the save surrendered to the host: his main screen presses Turn, msg 0x38):
every send of both machines and the host's saves equal the original's.

`TestNetSoak` `3+1ai-eliminated` and `2+1ai-eliminated`: the host's
`Eliminate_Player_` of the last joiner before `Next_Turn_` (set up like
the soak's battles); he gets msg 0x3b, plays the loser's video and goes to
the main menu; the others play on (with two humans in game type 0).
`3+1ai-creator-eliminated` and `2+1ai-creator-eliminated`: the creator
hosts the turn that eliminates him; he sends no msg 0x3b to himself, leaves
on his broadcast, and the next turn is hosted by another.

## Loading

### The multiplayer screen's LOAD (0xf5086)

`Get_Valid_MP_Save_Games_` (0xf33ae): per slot 0..9 the save's game type
(`Get_Game_Type_From_Saved_Game_`, byte 0x2d) when it is 1..3, else 0; any
such slot. None: `Warning_Box_` H 0xf4 ("No multi-player saved games
found") and the screen goes on. With the null modem (`_settings`+0x16 = 3)
a confirmation H 0x1c first. Then screen 0x29, `Save_Game_Settings_`,
`Clear_Fields_`, `Load_Multi_Player_Game_Screen_`: a slot 0..9 goes to
screen 0x11 (`Hotseat_Select_Player_`) in game type 1, else to 0x29
(`Load_Net_Screen_`); otherwise `Load_MP_Setup_Pictures_` and back to 0xf.

### Load_Multi_Player_Game_Screen_ (0xf4760)

- Game type 2: `Change_MP_Game_Name_` (0xf5777): over the multiplayer
  background, `Remapped_Input_Box_Popup_` (H 0xfe "Enter Net Game Name",
  8 characters, at 0xb1, 0x7d) edits `_settings`+0x17, asked again with
  H 0xff while one of the first ten listed games has the name (the list is
  whatever `Net_Update_Game_List_` last fetched: none here), then
  `Save_Session_Related_Settings_`. The name goes to 0x1ab082; empty:
  "MooII" and a letter from `clock_() % 25` (0x17a0e0).
- `Load_Load_MP_Game_Screen_` (0xf41ad): MULTIGM.LBX 0, the box by game
  type (0x12 network, 0xfe modem, 0xff hotseat), CANCEL 0x101; x centred,
  y = (0x1e0 − h − 0x51) / 2 + 0x25.
- `Get_Saved_Game_Descriptions_`, `Get_Star_Date_Strings_`,
  `Get_Save_Game_Date_Strings_` (the month names are whatever the game
  popup last loaded: empty before any).
- `Add_Load_MP_Game_Screen_Fields_` (0xefe7a): no multiplayer save, a
  warning and −1 (the auto function is not put back); else ten hidden rows
  (x+0x26..x+0x190, y+0x40+0x1b·i, 0x16 high) and CANCEL (Esc).
- `Draw_Load_MP_Game_Screen_` (0xf1ce6): the background once (or with
  `_FULL_DRAW_SCREEN_FLAG`), the box, `_green_pal_colors` into the palette
  at 0xc0, per slot the description (embedded codes removed; H 0x184 when
  empty) in `Font_Colors2_(2, 0x90, 0x93)`, valid slots 0x97, the hovered
  valid one 0x99 with `Increase_Font_Color_Range_` cycling −2..4 (its step
  is added twice per draw), the stardate and the file date in style 1, the
  type icon MULTIGM.LBX 0x102 (0), 0x103 (1), 0x104 (2, 3).
- A click on a valid row: `Load_Game_`, then **the game type is put
  back** to the one chosen on the multiplayer screen (a network save loads
  as a hotseat game under the hotseat radio) and every net id is −1. An
  invalid row: H 0xfd "Save %d is not a valid mutli-player saved game"
  with the 0-based slot. CANCEL: −1, screen 0xf.

### Load_Net_Screen_ (0xfb27b), screen 0x29: the creator

1. State 0x24, `Net_Init_Host_Election_`, the initializing screen,
   `Net_Init_`, `Russ_Net_Create_Game_` with the game name (*port-only*:
   the TCP port box first, as for a new game); failures as
   `Start_Net_Screen_`.
2. Net active, own id = game host = `Net_Host_ID_`, nebulas allowed.
3. `Choose_Network_Plyrs_Screen_` (nothing taken): his empire; PLAYER_NUM
   and its net id. 0x1aaf59 (the net id asked to pick) = −1.
4. The waiting screen (`Draw_Wait_For_New_Joiners_Screen_`, START). The
   humans of the save are counted (objective 100). A counter of picks
   asked starts at 1 (himself). Each pass:
   - nobody asked: the users of `Net_Get_Users_` from the last down; the
     first whose net id is no player's is asked: counter + 1, 0x1aaf59 =
     his id, **msg 0x20** (`Send_Pick_Your_Position_` 0xfa05d: u16
     NUM_PLAYERS and every player record) to him;
   - the screen; then, nobody being asked: done when the counter equals
     the humans (or is 2 with a modem); else, when every user has been
     asked, `Get_Input_`: START done when all humans are placed, else
     `Do_Confirmation_Box_` RSTRING 165 "Really begin game? (AI will take
     over unpicked human slots)";
   - `Mox_Update_`, the screen again.
5. Joiners closed; every player without a net id who is human becomes an
   AI (`Make_Player_Into_AI_`); the others are human (objective 100) and
   counted; `Do_Player_Colony_Calculations_` for every player; without
   nebulas (a joiner's msg 0x42) none, and no star in one.
6. The sending screen, the combat slots (state 4, argument −1), **msg
   0x2a** to all humans: 0x60 bytes, +0x28 NUM_PLAYERS, +6+4k each
   player's net id (s32), +0x2e the build date; the other bytes are the
   frame's old stack (marked stale).
7. `num_net_players`, the arbiter = own id, the full broadcast, own msg
   0x22 flag, the msg 0x36 flags cleared, state 4, the main screen
   (`Init_Main_Screen_`, `Harold_Init_`, `Set_Defaults_`).

`Screen_Control_` then shows the reports; the previous screen is 0x16
(`Load_Net_Screen_` shares `Join_Net_Screen_`'s tail).

### Choose_Network_Plyrs_Screen_ (0xf0e17)

Arguments: the position out, the taken flags (8 bytes), START allowed,
load the pictures. Both callers pass "no START" and "load", so START
(and its confirmation H 0xf5) never shows, and the screen has no CANCEL:
it ends only with a pick.

- `Load_Choose_Net_Plyrs_Screen_` (0xf3fc6): MULTIGM.LBX 0, 0x1b..0x1d
  (top, middle, bottom), START 0x10, CANCEL 0x101; the humans not
  eliminated (`Get_N_Remaining_Humans_`).
- y = (0x1e0 − (middle h · humans − 1 + top h + bottom h)) / 2.
- `Add_Choose_Net_Plyrs_Fields_` (0xefb50): per human not eliminated and
  not taken, a hidden name field (x+0x6a..x+0x1b3, 0x1d high, rows 0x24
  apart) and `Add_Banner_Field_`. **The taken flags are copied in only
  after the fields** (and after the fade), so every human gets fields; a
  click on a taken one is ignored (oracle: the joiner clicks his own,
  second row).
- `Draw_Choose_Net_Plyrs_Screen_` (0xf166e): the top animated (one more
  frame each draw), a middle per human but one, the bottom; per human
  "%s (%s)" (emperor, race name), taken ones in the dim colours of
  `Get_Player_Colors_`, the others bright with their banner, the hovered
  one cycling (−1..2).
- The loop ends on a pick (or on the word at 0x199a0e, which nothing
  here sets).

### Net_Pick_Position_ (0xfac85): a joiner of a loaded game

`Join_Net_Screen_` waits for msg 0 or **msg 0x20** (case 0xf633d: state
0x2c, the game host = the sender, NUM_PLAYERS and the records), sends msg
0x42 and calls `Net_Pick_Position_`:

1. taken = net id ≠ −1 per player; `Choose_Network_Plyrs_Screen_`;
2. state 7, **msg 0x21** {s32 the player} to the game host (case
   0xf637c: that player's net id = the sender, 0x1aaf59 = −1; −1 would
   be sent on a cancel, which cannot happen);
3. "waiting ...", the full broadcast with "Receiving chunk %d of %d",
   the decode (its result is not checked);
4. the humans counted, PLAYER_NUM = the human with the own net id;
   state 4, `Init_New_Session_`, the main screen.

Case 0xf64a8, **msg 0x2a**: the build date checked as in msg 0; then
from the count at +0x28 down (decremented in the receive buffer), each
player's net id from the table.

`Decode_Pick_Your_Position_` (0xfa0d3) does what case 0x20 does inline
(without the state and the host); nothing calls it.

### The saved custom race

Unreachable in network games: the 1.40 patch adds the saved race's button
only on screens below 0xf (`lobby.md`, "The saved custom race"). The code
behind it (0x5ccc5, `Request_Race_` of the race scan's leftover 13) is
ported as written.

The order of `Net_Get_Users_` decides who is asked first. IPX does not list
its stations by net id: in netquit3 (ids 1, 2, 3) the creator asked id 2
first, so the list ended with it. The port's relay lists by id; the
in-memory transport of the tests can take an oracle's order (`Mem.Order`).

### Tests (loading)

- `TestNetLoadOracle` (netbattle1, netbattle2, netload3): the original's
  runs from the load itself; netload3 is a three-human save loaded with two
  (START and Yes: the third becomes an AI). The host loads SAVE1.GAM on the multiplayer screen
  (LOAD, Enter on the name, the first row, his empire), the joiner joins
  and picks the second empire; the in-memory transport hands out the
  oracle's IPX ids (host 2, joiner 1). Every send of both machines from
  the first on (msgs 0x20 with all player records, 0x42, 0x21, 0x2a with
  its stale bytes masked, the full broadcast, then the turn and the
  battle) equals the original's, payloads included, and so do the
  autosaves after the load.
- `TestNetLoadSoak`: three humans and an AI play two turns; their
  autosave is loaded on two machines (START and Yes for the unclaimed
  third human, who becomes an AI) and played for 12 turns: each client
  equals the host after every broadcast.
- `TestNetLoadTCP`: netbattle1's save over TCP on localhost (as
  `-host`/`-join`), two turns.
- `TestHotseatLoad`: LOAD under the hotseat radio loads a hotseat save and
  goes to the hotseat picker in game type 1.

## The end of the game

`Next_Turn_` (0x13870) skips the end-of-game checks in game types 2 and
3 (already ported): a network game has no victory screen of its own.

- **Defeat**: msg 0x3b (above): the loser's video, `Mox_Net_Uninit_`,
  game type 0, the main menu.
- **Victory**: once one human is left the game is a single-player game
  (game type 0) and the next turn's `Next_Turn_` runs the usual checks
  (the last empire, the council, Antares).
- **The council** never meets in game type 2 (`Next_Turn_Calc_`
  0x137ba, `diplomacy.md`).
- **`Net_Exit_With_Message_`** (0xf5934) jumps to `Exit_With_Message_`:
  version mismatch (msgs 0, 0x2a), "only 1 host allowed" (0x39),
  "craziness" (a broadcast that does not decode), a failed send ("net
  send error!"); all ported as `ExitError` from `game.Run`.
- Do_Main_Game_Popup_'s NEW in a network game sets game type 0 and starts
  a new game without leaving the network (the others are not told).

## Saving and loading a 2-instance network game (port)

Every machine autosaves SAVE10.GAM at the start of each turn (game type
2). To continue later:

1. The creator: `openorion2 -host` (or `-host :port`), MULTIPLAYER, LOAD,
   enter a game name, pick the save (the network icon), then his empire.
2. The joiner: `openorion2 -join host:port`, MULTIPLAYER, JOIN, the game in
   the list, then his empire (the creator's is drawn dim).
3. The game starts once every human empire is claimed; with fewer
   machines, START on the creator's waiting screen and Yes make the
   unclaimed empires AIs.
