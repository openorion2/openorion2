# Network play

The original plays over IPX, a modem or a null modem cable. The port keeps
its network game logic as it is, routine by routine (the message ids and
payloads byte for byte, the host election and the order of uploads, the
broadcast diffs and the seed handling, combat lockstep, the diplomacy
protocol), and replaces only what lies under it: the transport is TCP,
WebSocket for the browser build, or the online lobby's server
(`docs/lobby/client.md`). Single player and hotseat do not change: all
network code runs only when `_game_type` is 2.

Modem and null modem are not ported: their radio buttons on the
multiplayer screen are drawn disabled, and `Modem_Setup_Screen_`,
`NullModem_Setup_Screen_` and `modem.tmp` are left out.

## The Net API

The original's game code calls a small API: `Net_Init_`,
`Net_Send_Message_`, `Net_Update_`, `Net_Is_Sending_`, `Net_Status_`,
`Net_Get_Users_`, `Net_Host_ID_`, `Net_Create_Game_`, `Net_Join_Game_`,
`Net_Update_Game_List_`, `Net_Get_Game_*`,
`Net_Enable/Disable_New_Players_` and the `Mox_Net_Update_` pump. The port
implements it over `netplay.Network` and `netplay.Endpoint`
(`internal/netplay`); everything above is the original's code.

## Transport (`internal/netplay`)

- **A star.** The machine that creates the game listens on a TCP port
  (52525 by default) and the others connect to it. The creator's process
  relays every message: A to B goes A, relay, B, and destination 0x7ffe
  goes to every other human. The relay is transport only: the original's
  per-turn host election still runs on top of it, so the game's host may
  be any player. A creator who quits leaves the relay running for the
  others.
- **Frames:** a length-prefixed {msg id u16, from net id u16, to net id
  u16, payload}. One reader goroutine per connection queues incoming
  frames; `Net_Update_` and `Mox_Net_Update_`, called from `Mox_Update_` on
  every page flip and input poll, drain the queue on the game's goroutine,
  so the game stays single-threaded. Sends are written at once.
- **Handshake (port-only):** the protocol version (in place of the
  original's build-date check; msg 0 keeps the date string), the player's
  name and net id, and the game's name and status for the joiner's list.
- **Discovery:** Join Game lists the games on the LAN (a UDP broadcast);
  joining by address works without it.
- **Command line:** `openorion2 -host [:port]` and `openorion2 -join
  host:port`, plus `-name`; several instances on one machine each take
  their own `-game` directory.
- **In memory:** the same API without sockets backs the tests: several
  games in one process, headless, on virtual time.
- **Browser:** the browser build cannot open TCP sockets; it goes through
  a WebSocket to `cmd/webserve`, which hosts a relay or bridges to one
  (`web.md`).

## The documents

- `overview.md`: what the original's network game does on top of its
  transport: roles, the host election, the turn.
- `functions.md`: every network routine of the original and where the
  port has it.
- `transport.md`: the Net API over `netplay`.
- `lobby.md`: the network lobby and the game start (NETSTART.C).
- `turn.md`, `broadcast.md`: the turn and its broadcast diffs.
- `combat.md`, `diplomacy.md`: combat lockstep and the diplomacy protocol.
- `leaving.md`: leaving, loading a network save, the end of the game.
- `hardening.md` (port-only): a lost player's empire handed to the AI, the
  desync check.
- `ui.md` (port-only): the address boxes and the other screens the
  original does not have.
- `web.md`: network play in the browser.

## Limits

- The browser side of the WebSocket transport is compiled, not run, by the
  tests; a relay hosted by `cmd/webserve` dies with it; a browser busy for
  longer than the 30 s dead-link limit (a very long AI turn) is dropped.
- If the creator's process goes, the game ends for everyone.
- Some waits in the original are loops that poll `Net_Update_`; they
  behave the same over TCP's latency, but very slow links have not been
  tried.
