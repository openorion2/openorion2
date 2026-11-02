# Network play: the transport and the Net API

The original's game code calls a small `Net_*` API
(net/netcode.c) over IPX or a modem. The port keeps everything above that
API, ported from the assembly, and puts TCP under it:

```
game code (Mox_Send_Message_, Receive_Message_, Mox_Net_Update_ ...)   ported, internal/game/netmox.go
Net_* (Net_Init_, Net_Send_Message_, Net_Update_ ...)                    port-only, internal/game/net.go
netplay.Endpoint / netplay.Network                                       port-only, internal/netplay
TCP relay and client (+ WebSocket for the browser, web.md) | in-memory hub (tests)
```

## internal/netplay

### API

- `Endpoint` is one station of a game:
  - `ID()`: its net id (`Net_Host_ID_`);
  - `Users()`: the members' net ids in id order, its own included
    (`Net_Get_Users_`); `Game()`: the game's name, open flag and members;
  - `Dropped()`: the members that left since it joined;
  - `Send(to, id, payload)`: to a member or `Broadcast` (0x7ffe, every
    other member); fails with `ErrNoUser` if `to` is not a member and with
    `ErrLost`/`ErrClosed` once its own link is gone;
  - `Recv()`: the next queued frame, never blocking;
  - `SetOpen(bool)`: the creator only (`Net_Enable/Disable_New_Players_`);
  - `Err()`, `Close()`.
- `Network` makes endpoints: `Create(game, player)`, `Join(game, player)`,
  `Games()`. Implementations: `TCP{Listen, Addr, Config, Started, Web}`
  and `*Mem`. With `Web` set (the browser build) the TCP side is done by
  cmd/webserve over a WebSocket: `HostRelay`, `ServeCreator`, `WebRelay`
  (`web.md`).
- `Listen(addr, game)` / `ListenConfig(addr, game, cfg)` start a `Relay`
  (its creator is `Relay.Local()`; `Members()`, `Done()`, `Close()`),
  `Dial(addr, game, player)` / `DialConfig` join, `Probe(addr)` reads the
  game's info without joining (the join list). `TCP.Started` gets the relay
  `Create` starts (cmd/openorion2 keeps it running, below). cmd/openorion2's
  network flags: `-host`, `-join`, `-name`, `-netdrop ai|end`,
  `-netcheck`, `-netcheckdir DIR` (`ui.md`).
- `NewMem()` is the in-memory hub; `Mem.Drop(game, id)` cuts a station off
  as a lost link would (tests); dropping the creator there is the relay
  dying.

### Topology

A star. The creator listens (default port 52525) and is net id 1; joiners
get 2, 3, ... in join order, at most 8 stations. The relay routes every
frame: to one member, or for `Broadcast` to every member but the sender. A
frame to oneself is queued locally. The relay overwrites `from` with the
sender's id. When a station leaves (its own `Close`, or a lost link), the
others see it in `Users()` and `Dropped()`.

*Port-only (phase 7)*: the creator's station leaving (his `Close`: quit,
the end of his game) is no different: **the relay outlives it** and goes on
routing for the others, then closes itself once the last station has left
(`Relay.Done()`). `cmd/openorion2` waits for that before the process exits
(the window is gone by then; it prints how many stations it still relays
for; Ctrl-C ends it, and the others lose the game). Only the relay itself
going (the creator's process ends or his machine drops, `Relay.Close`)
closes every link: each station's `Err()` becomes `ErrLost`. No relay
migration: the joiners know only the creator's address, and one of them
taking over would need every other to reach him (`hardening.md`).

### Frames

Every frame on the wire, little endian:

| Offset | Type | Field |
|---|---|---|
| 0 | u32 | n = 6 + payload length (6 .. 6 + 0xffff) |
| 4 | u16 | message id |
| 6 | u16 | from (net id) |
| 8 | u16 | to (net id, 0x7ffe all, 0 the relay) |
| 10 | n − 6 bytes | payload |

Game messages use the original's ids (all below 0x900) and payloads
byte for byte. Control frames use ids from 0xff01 and destination 0:

| Id | Direction | Payload |
|---|---|---|
| 0xff01 Hello | station → relay | u16 version, u8 kind (0 join, 1 query, 2 create on a hosted relay, `web.md`), player name\0, game name\0 |
| 0xff02 Welcome | relay → station | u16 version, u16 net id (0 for a query), u32 generation, u8 open, game name\0, u8 n, n × u16 ids |
| 0xff03 Refuse | relay → station | the reason as text; the link closes |
| 0xff04 Users | relay → stations | u32 generation, u8 open, u8 n, n × u16 ids |
| 0xff05 Lost | relay → stations | u16 net id that left |
| 0xff06 Ping | either way | empty; swallowed by the reader (keepalive) |
| 0xff07 Open | creator → hosted relay | u8 open; only from a hosted relay's creator link (`web.md`) |

The handshake (10 s limit) checks the protocol version (`Version`, 1): it
replaces the original's build-date check at the transport level; msg 0
still carries the date string and the game still compares it. A join is
refused for another game name, a game closed to new players, or a full
game. Membership changes carry a generation because the relay sends them
from several goroutines: a station keeps the newest.

### Goroutines

One reader goroutine per TCP link queues frames (and applies Users/Lost);
the relay runs one goroutine per joined link and routes in it, so frames
from one sender keep their order. Sends are written synchronously on the
caller's goroutine. The game never blocks on the network: `Net_Update_`
drains the queue on the game's goroutine.

### Dead links (port-only, phase 7)

`Config{Ping, Dead, Write}` (defaults 5 s, 30 s, 20 s):

- each side of a link writes a Ping once the link was idle for `Ping`
  (one pinger goroutine per link, ended with the link);
- each reader sets a read deadline of `Dead` before every frame: a link
  silent that long is lost (a station's `Err()` becomes `ErrLost`; on the
  relay the station leaves, and the others get Lost);
- every write has a deadline of `Write`: a peer that stops reading (its
  buffers full) is dropped instead of blocking the sender for ever. A send
  to such a peer may take up to `Write` on the game's goroutine once;
- the OS's TCP keepalive is on as well (period `Dead`/2).

`Close` ends a station's reader and pinger and waits for the reader; a
relay's `Close` closes the listener and every link and waits for all its
goroutines. `TestTCPDeadPeer` covers a silent station, a silent relay, a
station that stops reading and idle links that must stay.

### The in-memory transport

The same hub without sockets: a send lands in the receiver's queue at once.
Under `host.Lockstep` (one machine runs at a time, machine 0 first) a
message sent in frame f by machine i is drained by machine j in frame f if
j > i and in frame f + 1 if j < i, in every run (`TestNetLockstep`).

## The Net API (internal/game/net.go, port-only)

| Routine | Address | Port |
|---|---|---|
| `Net_Init_` | 0x138f3b | type 0 (network) succeeds when `Options.Net` is set and keeps the receive callback; the modem types fail |
| `Net_Uninit_` | 0x139062 | closes the endpoint |
| `Net_Update_` | 0x1390bb | with its reentry guard, hands every queued frame to the callback (`Receive_Message_`), then the lost-link check below |
| `Net_Is_Sending_` | 0x139127 | always false: sends are written at once |
| `Net_Send_Message_` | 0x139153 | `Endpoint.Send`; 0 without a link or when refused, else 1 |
| `Net_Status_` | 0x13920c | 0 (never called; bit 1 would be "sending") |
| `Net_Get_Users_` | 0x139294 | the members' ids as bytes, own included, in id order *(inference: the IPX station list holds the own station too)*; returns the count |
| `Net_Is_User_Valid_` | 0x1393b6 | membership (never called) |
| `Net_Host_ID_` | 0x139447 | `Endpoint.ID()` |
| `Net_Update_Game_List_` | 0x139476 | `Network.Games()`, at most 16 |
| `Net_Get_Game_Count_`, `_Name_`, `_Status_` | 0x139659, 0x13968a, 0x1396f8 | from that list; names cut to 9 characters (char[10]) |
| `Net_Join_Game_`, `Net_Create_Game_` | 0x13974f, 0x13982d | `Network.Join` / `Create`; the password and the player count are not used |
| `Net_Get_Player_Count_` | 0x1398ea | the member count (never called) |
| `Net_Enable/Disable_New_Players_`, `Net_New_Players_Enabled_` | 0x13994e, 0x13998f, 0x1399d0 | `SetOpen`, `Game().Open` |

`Set_Null_Modem_Slave_`/`_Host_` are not ported (null modem only).

**Lost links (port-only).** The original ends the game when a send fails
(`Mox_Send_Message_` → `Exit_With_Message_(_net_send_error)`); IPX sends
never fail, so a vanished machine left the others waiting.
`Net_Update_` therefore checks after every pump (`netCheckLost`): its own
link lost ends the game with a message naming the creator; a member whose
net id a player record holds has left becomes an AI on every machine, or
ends the game with `-netdrop end` (`hardening.md`). A send to a station
that is not in the game succeeds and goes nowhere, as over IPX, once the
game has started (`-netdrop ai`). Port-only frames 0xfe00..0xfe03 go
through `Endpoint.Send` outside `Mox_Send_Message_` and are taken by
`Net_Update_` before `Receive_Message_` (`hardening.md`, "Frames").

**Exit_With_Message_.** `exitWithMessage` runs `Reset_System_`'s network
part (`Net_Uninit_`) and panics with an `*ExitError{Msg}`; `game.Run`
recovers it and returns the error, which `cmd/openorion2` prints, as the
original prints the message and exits. Nothing is saved on the way out:
the per-turn autosave (`Do_Autosave_`, every turn in network games) is
what is left of the game.

## NETMOX.C plumbing (internal/game/netmox.go, ported)

### `_net` and the module's data

`_net` (0x1aaec4) and the data after it up to `_in_receive_message`
(0x1ab0df) are kept as bytes (`netGame.mem`), since the code indexes them
by player with values that may be −1. Fields used so far:

| Address | Field |
|---|---|
| 0x1aaf58 (`_net`+0x94) | connection type: 0 network, 1 modem, 2 null modem (`Set_Multi_Player_Game_Type_`) |
| 0x1aaf5e | i16 own net id (set from `Net_Host_ID_` by the net screens) |
| 0x1aaf76 / 0x1aaf7e | i8[8] host votes / u8[8] ready |
| 0x1aaf86 | i16 net id of this turn's host |
| 0x1aaf8b | 12-byte combat slot per player; +0xb (0x1aaf96) is the argument of msgs 6, 7, 9 |
| 0x1ab03f | u8 network state (4 and 8 send msg 0x36; 0xe..0x22 run `Do_Net_Combat_Update_` on the host) |
| 0x1ab04a | i8 human of a human diplomacy (msg 0x44) |
| 0x1ab050..0x1ab053 | bit per player: msgs 7, 6, 9, 0x1d due |
| 0x1ab055 | u8[8]: per AI talker k, bit p: msg 0x22 due to player p |
| 0x1ab05d | u8[8]: the human who talked with k |
| 0x1ab07a | i8[8]: the vote last sent to each player |
| 0x1ab08b, 0x1ab0a7, 0x1ab0af, 0x1ab0b7 | u8[8]: msgs 0x2b, 0x39, 0x3d, 0x3f due |
| 0x1ab094 | u8[8]: the player takes msg 0x22 |
| 0x1ab09c | i16 last turn's host (the diplomacy arbiter) |
| 0x1ab09e | u8: a network game is set up (the net screens set it) |
| 0x1ab09f | u8[8]: msg 0x36 sent to the player |
| 0x1ab0cb | i8: msg 0x44 due unless −1 |
| 0x1ab0df | `_in_receive_message` |
| 0x183608 / 0x183609 / 0x18360a | statics: `Mox_Send_Message_` runs / `Mox_Net_Update_`'s reentry guard / its round-robin counter (u8) |

`Net_Init_Host_Election_` (0xfc6a5) resets votes and sent votes to −1,
ready, 0x1ab06a.., 0x1ab072.. and 0x1ab094.. to 0, the host id and
0x1aaf88 to −1, 0x1aaf8a to 0 and msg 0x44 to −1. It takes an argument
(edx) it never reads.

### `Receive_Message_` (0xf67c1)

The receive callback: `(from, id, len, buf)`. It looks up the sender's
player (`Net_Id_To_Player_Id_` 0xfa16c: from the last player down, −1 if
none), sets `_in_receive_message`, and routes: id ≥ 2000 to
`Diplomacy_Receive_Message_` (0x1f230), 1000..1999 to
`Tactical_Combat_Receive_Message_` (0x3fcd8), the rest to
`Main_Receive_Message_` with the player index in eax. The flag is cleared
afterwards.

### `Mox_Send_Message_` (0xf6816)

`(to, id, buf, len)`:

1. A length of 0 becomes 1 with `buf` pointing at `[ebp-4]` of its own
   frame: the receiver gets one stale byte (00 in every oracle record).
   The port sends 0 and marks it stale, like `g.stale`.
2. `Sound_Update_` and, unless inside `Receive_Message_`, `Net_Update_`
   around every step.
3. To one's own net id: straight to `Receive_Message_` (from = to), result 1.
4. Otherwise, after waiting while `Net_Is_Sending_`, for `to` = 0x7ffe it
   counts the other humans: every player, from the last, whose net id is
   not −1 and not one's own (compared as u16). None: nothing is sent and
   the result is 1. Exactly one: `to` becomes his net id.
5. Still 0x7ffe on the network (connection type 0): one
   `Net_Send_Message_` per player, from the last, whose net id is > 0
   (signed) and not one's own. A net id of 0 is counted in step 4 but never
   sent to. Any failure → `Exit_With_Message_(_net_send_error)`
   ("net send error!", RSTRING string 154).
6. Otherwise one `Net_Send_Message_(to)` (the modem's broadcast passes
   0x7ffe down); a failure exits the same way.

### `Main_Receive_Message_` (0xf5a9f)

`(player, from, id, len, buf)`. Ids above 0x64 return at once. Otherwise
`repne scasb` looks the id up in the 0x44 bytes at 0xf594c:

```
64 44 43 .. 2a 27 26 .. 1f 1d 1c .. 01 00 f2
```

and jumps through the table at 0xf598f indexed by what is left in ecx:
id n ≤ 0x1d → entry n+1, 0x1f..0x27 → 31.., 0x2a..0x44 → 40.., 0x64 → 67;
an id not in the table (0x1e, 0x28, 0x29, 0x45..0x63) leaves ecx 0 and
entry 0 is the shared return 0xf5bf2. The scan covers the jump table's
first byte (0xf2) as its 68th id, which no message can reach. Shared
cases: ids 3 and 0x33 (0xf5cb0), 0x2f and 0x35 (0xf65b0), 0x37 and 0x38
(0xf662b). The port reads both tables from the executable. The cases
belong to phases 2–6 and log "not ported" until then.

### `Mox_Net_Update_` (0xf69fe)

Run by every `Mox_Update_` (0xfe8be: `Sound_Update_`, then this). Only in
game types 2 and 3, not reentrant (0x183609), not inside
`Receive_Message_`. It calls `Net_Update_`; then, once a network game is
set up (0x1ab09e):

- p = counter (u8, 0x18360a) mod `NUM_PLAYERS` (−1 without players);
  it calls `Mox_Update_` (which stops at the reentry guard: only the sound
  runs) and bumps the counter;
- nothing more while sending; then at most one message, the first that is
  due, to player p (or about p):
  1. state 4 or 8: msg 0x36 (len 0) once per player, unless his net id is −1;
  2. msg 7 {slot arg} (0x1ab050);
  3. msg 0x22 for each AI talker k (from the last) with p's bit in
     0x1ab055[k]: {u32 k, player k, 72 bytes `star[s].surrender_to[k]`
     for the stars, u32 partner, player partner}, 0x1da2 bytes, if p is not
     the local player nor the partner, is human (`objective` 100) and takes
     msg 0x22; the bit is cleared even when nothing is sent. With fewer than
     72 stars the rest of the 72 bytes is the frame's old stack (stale);
  4. msg 6 {slot arg} (0x1ab051) unless p's bit is set in 0x1ab055[arg]
     (arg signed: −1 reads 0x1ab054);
  5. msg 9 {slot arg} (0x1ab052); 6. msg 0x1d (0x1ab053, len 0);
  7. msg 4 {s32 own vote, own ready} when p is another human and the vote
     last sent to him differs from one's own;
  8. on the turn's host in states 0xe..0x22: `Do_Net_Combat_Update_` (phase 4);
  9. msg 0x2b {p's banner, p} to all; 10. msg 0x39 {p's race, p} to all;
  11. msg 0x3d {p's race} to all, then msg 0x3f due; 12. msg 0x3f (len 0) to p;
  13. msg 0x44 {own player record} to the human of 0x1ab04a.

`Basic_Update_` (0xf5939) is `Sound_Update_` plus the guarded
`Net_Update_`. `Net_Exit_With_Message_` (0xf5934) jumps to
`Exit_With_Message_`.

Single player and hotseat do not reach any of this: `Mox_Net_Update_`
returns at its game type test, and nothing else is called outside the
network screens.

## Tests

- `internal/netplay`: `TestFrameRoundTrip` (frames, limits, malformed
  input); `TestTransport` on both transports: net ids, unicast,
  `Broadcast`, local delivery, order, unknown destinations, the open flag,
  a joiner dropping, the relay dropping, the creator's station leaving;
  `TestTCPRefusals` (wrong game, closed, full); `TestTCPDeadPeer`,
  `TestTCPRelayOutlivesCreator`.
- `internal/game` (net_test.go):
  - `TestNetLogReplay`: every message the oracle's two machines sent
    (`build/oracle/net2t`, 56 + 40 records) replayed through
    `Mox_Send_Message_` on the machines' autosaves over the in-memory
    transport. Each machine's sends must match the original's NET.LOG
    records one for one and its receipts, per sender, the original's in
    order (local delivery, 0x7ffe rewritten to the one other human, the
    stale byte of zero-length sends).
  - `TestNetReceiveRouting`: `Receive_Message_`'s split by id and
    `Main_Receive_Message_`'s case for every id 0..0x64 against the table
    decoded by hand.
  - `TestNetSendRouting`: to oneself, one human, all humans (order, the
    net id 0 quirk), alone, and a send to a gone station ending the game
    with "net send error!".
  - `TestNetDisconnect`: with `-netdrop end` a joiner's link dropping ends
    the creator's game with a message naming him (in-memory and real TCP on
    localhost); the relay going ends the joiner's. The default (the lost
    player becomes an AI) and the dead-link tests: `hardening.md`.
  - `TestNetSinglePlayerUntouched`: game types 0 and 1 send nothing.
  - `TestNetLockstep`: two games on one `host.Lockstep` clock pumping
    `Mox_Net_Update_` (msg 0x36 round robin): arrival frames as above,
    identical in three runs. Its helper `netMachines` is the N-machine
    harness until the lobby (phase 2) can start a game.
