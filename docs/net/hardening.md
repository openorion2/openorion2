# Network play: hardening (port-only)

Nothing here is in the original; every piece is
marked `// port-only` in the code. The game's own message stream does not
change: a game without lost links sends exactly what it sent before (the
oracle tests `TestNet*Oracle` are unchanged), and the port-only frames go
through `Endpoint.Send` outside `Mox_Send_Message_`, so they never show in
`NetLog`.

Code: `internal/netplay` (links), `internal/game/netdrop.go` (lost
players), `internal/game/netcheck.go` (the desync check).

## What the original does

IPX sends never fail. When a machine vanishes, the others keep waiting for
its messages for ever (a modem link ends the game with "net send error!"
from `Mox_Send_Message_`). Phase 1 of the port ended the game on every
other machine with that message instead. A **quit** (game menu, QUIT) is
handled: `Human_Abdicates_` sends msg 0x37 with the leaver's player record
after `Make_Player_Into_AI_`, and every other machine's
`Decode_Drop_Out_Info_` makes him an AI (`leaving.md`).

## Decisions

| Item | Default | Flag | Why |
|---|---|---|---|
| Pings, read and write deadlines, TCP keepalive | on | — | transport only, invisible to the game (`transport.md`, "Dead links") |
| A lost player becomes an AI, the game goes on | on | `-netdrop end` (`Options.NetDropEnd`) ends the game for everyone instead | the original's behaviour (an endless wait) is not worth keeping; it reuses the original's own quit path and adds nothing to a game without drops |
| A message naming who was lost | on | — | replaces "net send error!" where the port ended the game anyway |
| The relay outlives its creator's quit | on | — | the original's others go on after any quit; the relay is transport |
| The desync check | off, on in a lobby game | `-netcheck`, `-netcheckdir DIR` (`Options.NetCheck`, `NetCheckDir`) | a debugging aid; it adds frames; a lobby game reports a mismatch to the lobby |

## Lost players

### Who notices

`Net_Update_`, after handing the queued frames to `Receive_Message_`, runs
`netCheckLost` in a running network game (`_game_type` 2, `_net`+0x1da
set, not eliminated):

- **its own link lost** (`ErrLost`: the relay is gone, or this machine's
  network): the game ends: "The connection to *X*, who relays the game, is
  lost. The game ends." (*X* the player with net id 1, the creator);
- **a station left** whose net id a player still holds:
  - with `-netdrop end`: "*X* has lost the connection. The game ends.";
  - before the game started (`netGame.inGame`, set when `Start_Net_Screen_`,
    `Join_Net_Screen_` or `Load_Net_Screen_` reach the main screen): "*X*
    left before the game started. The game ends.", sent to the others
    first (abort, below). The race picks and `Init_New_Game_` cannot do
    without him;
  - else, on **the injector** only, his drop-out is sent (below). The
    injector is the station with the lowest net id among those that still
    belong to a human player (normally the creator, id 1); every machine
    computes it from the same membership, so exactly one sends. Should the
    injector itself be lost first, the next lowest takes over.

*X* is "emperor (race)" from the player record, as the network screens
name players.

### The drop-out sent for him

The injector does for the lost player p (net id d) what p's own machine
does on a quit (`Send_Drop_Out_Info_`): p's net id −1,
`Make_Player_Into_AI_(p)`, and msg 0x37's payload (the record and the
star bytes `star+0x67+p`). It goes to every other station as a port-only
frame 0xfe00 {u16 d, u16 0x37, payload}; each receiver hands it to
`Receive_Message_` **as msg 0x37 from net id d**, so
`Main_Receive_Message_`'s case 0xf662b and `Decode_Drop_Out_Info_` run
exactly as for a quit. A receiver that no longer has a player with net id
d (his own quit came first, or a second sender) ignores it. The injector
applies the end of `Decode_Drop_Out_Info_` itself (the human count, p
ready, the host's upload count).

Every machine thus ends with the same record for p, and the next
broadcast carries the rest.

### Sends to a lost station

With a player turned AI nobody sends to him any more (his net id is −1).
Before that, and for a few late sends (msg 0x44 after a talk), a send to a
station that is not in the game goes nowhere and succeeds
(`Net_Send_Message_`), as an IPX send would. Only a lost own link fails a
send.

### What else waited for him (`netLostFixups`)

Only for a drop-out sent for a lost player (`netGame.dropping`), on every
machine that applies it:

| Where | What |
|---|---|
| The election (msg 4) | votes for p are −1; a machine that pressed Turn votes for itself again (or the lowest vote left); every vote is sent again. A msg 4 voting for a lost player that comes later counts as −1. |
| The turn's host collecting uploads | if p was being asked (state 0xa) the host goes back to state 9 and takes the next one, and does not decode the missing upload; an upload already in is not counted twice (the original's quirks are kept for a real quit, which cannot happen there) |
| The turn's host lost mid-turn | the turn starts over (below) |
| The combat phase, on the turn's host | p's slot and every slot busy with p go back to state 0xe, busy −1, `0x1ab04b` 0: a battle he was in is picked again, now with an AI |
| A battle on this machine | a human attacker waiting for the lost defender's msg 0x19 goes on; a human defender waiting for the lost attacker's result, or either side of the tactical lockstep (`Wait_Until_Net_Opponent_Finished_`, the replay of his action), ends the game: "*X* has lost the connection in a battle with you." |
| The diplomacy arbiter | p's place in an AI's queue is withdrawn as msg 8 would (without the answer to him); the next asker gets his go |
| A talk between humans | it ends as if p had said good bye (`_net_diplomacy_esc_pressed`, and the first synchronisation is taken as done) |
| The player | a notice "*X* has lost the connection. The computer plays the empire from now on." as a GNN line on the waiting screen and a warning box on the main screen |

### The host of the turn lost mid-turn

Each client of the turn gets the drop-out while inside `Net_Next_Turn_`
(waiting for msg 0xc, uploading, in `Client_Combat_Loop_`, waiting for the
broadcast). Unless the whole broadcast is already in (state 7 and every
announced chunk received: it is decoded as usual), it **starts the turn
over**: `Chat_Box_Input_Loop_` and the combat loop's chunk wait unwind to
`Net_Next_Turn_` (a panic it recovers), which sets state 8, own ready and
vote, no host, nothing received, sends its vote again and holds the
election anew; the new host takes everybody's upload again and runs
`Next_Turn_`. This is consistent because:

- the host runs `Next_Turn_` only once every upload is in, and the clients'
  uploads change nothing on the clients;
- whatever a client changed meanwhile (a battle it fought) is overwritten
  by the new host's broadcast, which is a diff against the previous
  broadcast, the same on every machine;
- the only machine that can be ahead is one that got the lost host's whole
  broadcast. Each restarting machine sends frame 0xfe03 {stardate}; a
  machine at another stardate ends the game for everyone: "The host of the
  turn left after some machines had his turn and others not." (with N ≥ 3,
  only if the host was lost in the middle of sending the broadcast).

A battle lost with the old host (fought on a client, its result never
sent) is fought again by the new host's turn, from the state the client
had then. With two humans the remaining one hosts the turn alone and goes
on in game type 0.

### Where the game cannot go on

The game ends on every machine with a message box naming the lost player
(frame 0xfe01 carries the text to the others first): the lobby, the
tactical lockstep, a human defender waiting for the lost attacker's
result, a host lost while some had his broadcast, the relay lost.

### The creator

- **Quits**: his machine sends msg 0x37 as any quitter's (he becomes an AI
  everywhere); his station leaves the relay, which keeps routing for the
  others (`transport.md`); `cmd/openorion2` keeps the process running until
  the last station left, printing "still relaying the network game for n
  player(s)". The next injector is the next lowest net id.
- **Lost** (his process ends, his machine or network goes): the relay goes
  with it, every other machine's link is lost and the game ends with the
  message above. No relay migration: the joiners only know the creator's
  address, and a joiner taking over would need every other one to reach
  him (often behind NAT); each machine's last autosave (every turn) can be
  loaded as a new network game.

### The end

`netEnd` closes the network (`Reset_System_`) and panics with an
`*ExitError{Msg, Notice: true}`; `game.Run` shows the message in a warning
box (`Warning_Box_`'s style, `WARNING.LBX`) before it returns the error,
which `cmd/openorion2` prints. The original's own `Exit_With_Message_` paths
keep printing without a box.

## The desync check

With `-netcheck`, after each broadcast (the host after sending, a client
after decoding) every machine hashes (SHA-256) its state as a save but for
the per-viewer fields `TestNetSoak` also allows to differ: `PLAYER_NUM`,
the settings but the universe's (+0x02 and +0xd3..0xd9, what a load takes
from a save; the rest is each machine's MOX.SET: sound, the network game's
name), the save descriptions,
`something_interesting_happened`, the moveable boxes and their order, the
map's view (scale, position) and the fleet icon buttons, the stars' owners as
this viewer sees them, an eliminated player's objective and net id (the
host keeps the objective, a client's decode zeroes it; the host of the turn
that eliminates himself keeps his own net id, which the others set to −1),
and the nebula, planet and star slots past their counts (a
regenerated galaxy leaves the host's earlier ones there; the broadcast
carries the counted ones, and one star more). It sends frame 0xfe02 {u32 stardate, 32
bytes} to every other station and compares each hash it gets for a
stardate it has: a mismatch logs "NET DESYNC: stardate s: the state of net
id a differs from net id b's" and, with `-netcheckdir DIR`, writes this
machine's save of that stardate to `DIR/netcheck-s-ida.GAM` (every
machine writes its own, so both sides of a mismatch are there). Hashes and
saves are kept for four stardates. The game's messages do not change.

A game played through the online lobby runs the check without `-netcheck`
and reports a mismatch to the lobby (`DesyncReport`, `docs/lobby/client.md`):
the stardate, the other machine's net id, both hashes, the game's version
and this machine's save of that stardate, gzipped. Every machine that sees
the mismatch reports, so the lobby keeps both sides; at most four reports a
game. The report goes in the background and the game plays on; the
player is told once a game (a GNN line while waiting, a message box on the
main screen: "Desync: at stardate 3501.2 the game here differs from NAME's.
A report went to the lobby. The game goes on.").

`tools/savediff a.GAM b.GAM` compares two machines' saves of one stardate
(`-netcheckdir`'s, or a report's, gzipped or not) field by field without
the fields above: what it prints is the desync.

## Frames

| Id | Payload | Sent by |
|---|---|---|
| 0xfe00 | u16 lost net id, u16 msg id (0x37), the drop-out | the injector |
| 0xfe01 | the message, text | a machine that ends the game for everyone |
| 0xfe02 | u32 stardate, 32-byte SHA-256 | every machine, with `-netcheck` or in a lobby game |
| 0xfe03 | u32 stardate | a machine that starts the turn over |

## Tests

- `internal/netplay`: `TestTCPDeadPeer` (a silent station dropped, a silent
  relay lost, idle links kept by pings, a station that stops reading
  dropped by the write deadline), `TestTransport` "creator leaves" and
  `TestTCPRelayOutlivesCreator` (the others go on; the relay closes once
  everybody left).
- `TestNetSoak` (in-memory transport, every machine on one lockstep clock),
  new modes, with N = 2 and 3, the link cut on turn 3: `drop-main` (on the
  main screen), `drop-vote` (after he pressed Turn), `drop-upload` (the
  host has just asked him for his upload: the host gives it up),
  `drop-host` (machine 1 hosts the turn and is lost while taking an upload:
  every other machine starts the turn over), `creator-quit` (the creator
  quits; the relay stays). The remaining machines play on to the end, the
  lost player is an AI with net id −1 everywhere, exactly one machine sent
  his drop-out and the others decoded it as msg 0x37, and every client
  equals the host after every broadcast (as before). Every soak run has
  `-netcheck` on and fails if it fires.
- `TestNetDropOut` (mem and TCP, three stations): the injector and the
  other station end with the same record for the lost player; in the
  lobby both end with the message.
- `TestNetDropCreator`: the creator is lost on turn 3: both others end
  with the message naming him.
- `TestNetDropLockstep`: one side of a tactical lockstep battle is lost:
  the other ends with the message.
- `TestNetDropTCP`: three machines over TCP, the last joiner's machine
  goes on turn 3, the other two play on.
- `TestNetDesyncCheck`: equal states (but for per-viewer fields) are
  silent; a changed BC fires on both sides, which write their saves.
- `TestNetDisconnect`, `TestNetSendRouting`: `-netdrop end`.

## Limits

- No relay migration: a lost creator ends the game.
- A drop in the lockstep, or of the attacker a human defender waits for,
  ends the game.
- A host lost in the middle of sending his broadcast ends the game when
  some machine got all of it (N ≥ 3).
- Not reached by a test: the diplomacy fixups (arbiter queue, a talk
  between humans), the combat-slot reset on the host, a defender lost
  after the attacker's battle, a drop while the lost player was the
  diplomacy arbiter, two players lost at once.
- A peer that stops reading can hold one send on the game's goroutine for
  up to the write timeout (20 s) before it is dropped.
