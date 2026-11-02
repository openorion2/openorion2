# The online lobby: the game's side

The Galactic Council is OpenOrion2's online lobby (port-only): log in from
the game, chat, send private messages, see the open games, create one or
join one, and play it through the server. The server is a separate
project; this repository holds everything the game needs and nothing of
the server, and the server imports the protocol packages from here.

## The game protocol stays

The game reaches the network only through `netplay.Network` and
`netplay.Endpoint` (`internal/netplay/endpoint.go`): create, join, list,
send a frame, receive a frame, members, open or closed. The lobby is one
more implementation beside TCP and the in-memory one: `lobby.Network`
(`internal/lobby/network.go`), whose endpoints send the game's usual frames
{msg id, from, to, payload} wrapped in a protobuf `GameFrame` to the
server, which relays them as `netplay.Relay` does: net ids from 1 (the
creator), `Members{generation, open, ids}` after each change,
`MemberLost{id}`, destination 0x7ffe for every other member, frames in
order per sender. So the game's messages, the turn, combat, diplomacy and
the drop handling (`docs/net/`) are the same over the lobby as over TCP.

`SetOpen(false)`, which the game calls only to start, is the lobby's
`StartGame`: the server takes the game off the list and refuses joins.
`GameRemoved` only takes a game off the list; a member's game ends with a
`GameUpdate` whose state is FINISHED.

## Connection and security

- One TLS connection per client, by default to
  `galacticcouncil.openorion2.org:52530`. Inside, both ways: a big-endian
  u32 length, then one protobuf `Envelope` (`lobbynet/proto/lobby/v1/lobby.proto`,
  a `oneof` of every message; `lobbynet/wire`).
- The first message is `Hello{protocol, build}`; a server of another
  protocol answers `VersionMismatch` with a text for the player and
  closes. `lobbyv1.Protocol` is bumped on any change an older peer would
  misread; additions it would only ignore (private messages) keep it.
- Login is SRP-6a (RFC 5054, 3072-bit group, SHA-256; `lobbynet/srp`,
  checked against the RFC's vectors): the password never leaves the game,
  not even hashed. `x = Argon2id(password, salt)` with the parameters the
  server sends. `LoginStart{username}`, `LoginChallenge{salt, kdf, B}`,
  `LoginProof{A, M1}`, then `LoginOk{M2, session}` (the game checks M2,
  so it knows the server holds its verifier) or `LoginFailed` with a wait
  the server enforces.
- The session is a server-side record; the game keeps only its UUID in
  `lobby.session` (readable by the player only) and logs in with
  `Resume{session}` next time. LOGOUT ends it.
- Accounts are made on the web (`https://openorion2.org`), not in the
  game.

## Files in the game directory

- `lobby.yaml`: `server: host:port`, written with the default if missing;
  for development `ca: file.pem` (port-only), the certificates the lobby's
  must be signed by instead of the system's: `tools/lobbyfake` runs the
  server's rooms and relay without accounts (a session is the user it
  names) and writes its certificate, for games across machines.
- `lobby.session`: the last username and the session.
- `SAVEn.LBY`: beside a save made in a lobby game, that game's UUID; LOAD
  through the lobby sends it as `ContinueGame`, so the server links the
  evenings of one long game. A save made outside the lobby removes it.

## Screens

- **Multiplayer screen**: TEN's button is the lobby's own, GALACTIC
  COUNCIL ONLINE LOBBY (hot key T).
- **Login** (`internal/game/lobbyui.go`): the username and password boxes
  (the password shown as `*`, paste with Ctrl+V or Cmd+V); a remembered
  session resumes without them. Errors come in the usual message box.
- **The lobby** (`lobbyhome.go`): the chat over the screen's height with
  who is online beside it, the input line, and CREATE (LEAVE while in a
  game), LOAD, JOIN (counting the open games), LOGOUT, BACK. The lobby's
  own news (a game created, a refusal) are system lines of the chat.
- **Chat tabs** (`lobbytabs.go`): ALL, then one tab per private dialog. A
  click on a name online, `/w name [text]` (or `/msg`) in ALL, or an
  unread line from someone opens it; an unread tab counts its lines and is
  read once shown; its x closes it, Esc goes back to ALL. At most 8 dialog
  tabs: a new one closes the oldest read one. Private lines go to a user
  online or not; the server keeps them until read.
- **CREATE and LOAD** ask for the game's description and whether it is
  public (private games are joined only by their id, which goes to the
  clipboard), then go on as the multiplayer screen's START and LOAD do,
  over the lobby's network. LOAD does not ask for a network game name: the
  lobby's game is named by its description.
- **JOIN** (`lobbyjoin.go`) opens a box over the chat: the open games
  (click to pick, Up/Down) and a line for a game's id, typed or pasted.
  JOIN takes the id if one is typed, else the picked game; then the
  joiner's usual path, without the game list.
- After a lobby game the player is back in the lobby on the same
  connection.

## Text in any language

The lobby's own text (chat, descriptions, names) is Unicode, printed in a
TrueType pixel font of its own (`assets/lobby/font`: Fusion Pixel Font
12 px, OFL, through `golang.org/x/image/font/opentype`); the game's bitmap
fonts have ASCII only and the game itself is untouched. While the lobby's
screen is up the window turns on SDL's text input
(`host.System.SetTextInput`), so any language and the system's input
method come as `host.Key.Text`; the lobby's lines (`uniline.go`) take the
keys before the engine's fields (`engine.KeyHook`). Without text input
(headless, the browser) they take plain ASCII keys. The game's own
9-character network game name keeps only the description's ASCII. Not
done: the input method's composition shown in the line.

## Scrolling

The chat scrolls by the mouse wheel (`host.System.Wheel`, 3 lines a
notch), PgUp/PgDn and Home/End, with a scroll bar once it overflows; lines
coming in while it is scrolled back keep it on the lines shown, and saying
a line brings it to its end. The JOIN box's list and who is online scroll
the same way, each with its own bar; the wheel goes to the panel under the
pointer.

## Chat commands

`/w name text` (or `/msg`) whispers; every other line, `/`-commands too, goes
to the server as a line of chat: the server runs `/report` and the
moderators' `/mute`, `/ban` and their undoing, and answers with an error the
game shows as a note. A banned account's login fails with
`LOGIN_FAILURE_BANNED` (for `retry_after_ms`, or for good when 0).

## Desync reports

In a lobby game the desync check runs always (`docs/net/hardening.md`, "The
desync check"); a mismatch goes to the server as a `DesyncReport` request
(`lobby.Network.ReportDesync`, in the background) with this machine's save
of that stardate, gzipped. The server keeps it with the game for the
developers; one that does not know the message answers an Error, which the
game ignores, so it needs no new protocol version.

## Tests

`internal/lobby/lobbytest` is a test server with the real server's rooms
and relay (any session resumes, as the user it names); the game's tests
play games through it (`TestNetTurnsLobby`, `TestNetDropLobby`,
`TestNetLoadLobby`, `TestNetDesyncLobby`). The screens' tests (`internal/game/lobbyui_test.go`)
run against a fake lobby with real SRP.

## Not done

The lobby's chat during a game, a lobby in the browser build (it would
need WSS), a lost creator not ending the game, reconnecting into a
running game.
