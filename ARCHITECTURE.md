# Architecture

A map of the code for someone who wants to change something. It says
where each part lives, which file to open first and which spec to read.
`CLAUDE.md` is the short version of the same map, with the test rules.

## The idea

OpenOrion2 is the original's code, rebuilt routine by routine in Go. It is
not a game "like" Master of Orion II: the same routines run in the same
order, draw the same random numbers and do the same integer arithmetic.
So the shape of the code follows the original's, not a design of our own:

- **One Go function per original routine**, with the routine's name and
  address in its doc comment:

  ```go
  // allocateMsgSlots is Allocate_Msg_Slots_ (0xeeb3a): the 100 message slots
  // are split evenly among the players still in the game; stale messages stay.
  func (g *Game) allocateMsgSlots() {
  ```

  Addresses are ELF addresses (the Watcom debug info's plus 0x10000), the
  same ones the specs use. Search the code for a routine name to find its
  port, or search `docs/` for it to find its spec.
- **The specs under `docs/` are the source of truth.** They describe what
  the original does, with routine names, addresses and data offsets. The
  code links to them (`Spec: docs/...` in file and function comments).
- **The state is the save.** The game's global state is the save file's
  blocks (`internal/game/state.go`, read through `internal/save`). Field
  names follow `docs/formats/save-*.md`.
- **Port-only code is marked.** What the original does not have (the
  online lobby, TCP and WebSocket play, window handling, the update check,
  Unicode text in the lobby, the remote control) carries `port-only` in a
  comment and is documented in `docs/`. Everything else should match the
  original.
- **The original's files stay outside.** The game reads the original's
  LBX archives (graphics, sound, text) from the game directory at run
  time. The constants and short strings the original keeps in
  `ORION2.EXE` are built in (`internal/game/exedata.bin`, below).

## How a frame runs

The original is a sequential DOS program: each screen has its own loop
that draws, waits for input and flips pages. The port keeps that shape.

1. `cmd/openorion2/main.go` parses the flags and calls `host.Run` with
   `game.Run` as the game.
2. `internal/host` (`host.go`, `window.go`) runs the game in a goroutine
   of its own. The main goroutine pumps SDL events and shows the latest
   page. The game hands pages over with a flip and reads the mouse and
   keyboard through `host.System`. Nothing above `internal/host` knows SDL.
3. `internal/engine` is the original's engine layer (WLIBS): pages and
   palette (`video.go`), fields and input (`fields.go`, `keys.go`), text,
   sound and timing. `Engine.TogglePages` is `_TOGGLE_PAGES_`: it runs
   `Mox_Update_` (`MoxUpdate`: sound, then the network pump), draws the
   fields and hands the page to the host.
4. `internal/game/game.go`: `Run` builds the `Game`, and `screenControl`
   is the original's screen dispatcher: a loop over `currentScreen`
   (`ScreenMainMenu`, `ScreenMain`, `ScreenNextTurn`...) that calls each
   screen's function. Each screen runs its own loop until it sets the next
   screen.

Specs: `docs/engine/video-and-timing.md`, `docs/engine/fields-and-input.md`.

### Virtual time

With `-headless` (or `OPENORION2_HEADLESS=1`) there is no window and no
real input (`internal/host/headless.go`). Time is virtual: a frame is
1/60 s of game time, taken as soon as the game waits. A scripted run
(`-load`, `-frames`, `-click`, `-key`, `-seed`) therefore gives the same
result on any machine and under any load. The tests rely on this. A loop
that neither waits nor reads input never advances the headless clock.

## Where things live

### The game's code: `internal/game`

One package, about 250 files, named after the screen or the turn step they
port. Most file headers name the original's C source file (`COLCALC.C`,
`COMBAT.C`, `NETTURN.C`) and the spec.

| Area | Open first | Spec |
|---|---|---|
| Galaxy screen | `mainscreen_loop.go`, `mainscreen.go`, `mainscreen_draw.go`, `panel.go` | `docs/screens/main-screen-*.md` |
| Other screens | named after the screen: `colony_screen.go`, `design.go`, `fleetscreen.go`, `diplomacy_screen.go`, `info.go`, `leaders.go`, `techselect.go`... | `docs/screens/` (one file per screen) |
| The turn | `turn.go` (`turnSteps`: the steps of `Next_Turn_Calc_` in order) | `docs/turn/overview.md` |
| Economy | `turn_colony.go`, `tech_economy.go`, `taxrate.go`, `turn_trade.go` | `docs/turn/overview.md` |
| AI | `turn_ai*.go`, `diplomacy_ai.go`; the AI data in `turn_aidata.go` and `aiarena.go` | `docs/turn/ai-data.md` |
| Battles in the turn | `turn_combat*.go`, `turn_stratcombat.go` | `docs/turn/combat.md` |
| Tactical combat | logic: `turn_tacloop.go`, `turn_tacai.go`, `turn_tacmove.go`, `turn_tacdamage.go`, `turn_tacfire.go`; screen: `combat_*.go` | `docs/combat/tactical.md`, `docs/screens/combat-screen.md` |
| New game | `newgame_*.go` | `docs/newgame/generation.md`, `docs/screens/new-game.md` |
| Saving, loading | `loadsave.go`, `state.go`; the format in `internal/save` | `docs/formats/save.md`, `docs/screens/load-game.md` |
| Network game logic | `net.go`, `netmox.go`, `netturn.go`, `netcombat*.go`, `netdiplo*.go`, `netcheck.go` | `docs/net/` |
| Online lobby UI | `lobby*.go` (port-only) | `docs/lobby/client.md` |
| Update offer | `updateui.go` (port-only) | `docs/release.md` |
| Agent view | `agentview.go`, `agentstate.go` (port-only) | `docs/control.md` |

The RNG is `internal/random` (the engine's `random.c`). The order of draws
is part of the behaviour: a draw moved, added or dropped changes every game
after it. Loops over players, ships and colonies often run from the last
record down; that direction matters for the same reason.

### The original's formats

| Package | What | Spec |
|---|---|---|
| `internal/lbx` | LBX archives | `docs/formats/lbx.md` |
| `internal/gfx` | pictures, bitmaps, palettes | `docs/formats/pictures.md`, `docs/formats/bitmaps.md` |
| `internal/font` | the game's fonts | `docs/formats/fonts.md` |
| `internal/smk` | Smacker videos | |
| `internal/sound`, `internal/audio` | sounds, music, the mixer | `docs/formats/sound.md`, `docs/engine/sound.md` |
| `internal/save` | save files, field by field; `Diff` compares two | `docs/formats/save*.md` |
| `internal/le` | the LE executable loader (for `tools/exedata`) | `docs/formats/watcom-debug.md` |
| `internal/clib` | Open Watcom's `qsort`, whose order the player sees | `docs/engine/qsort.md` |

### The executable's data

The game does not need `ORION2.EXE`. The tables and strings the code reads
from it (buildings, weapons, race and AI tables, screen layouts, file
names) are in `internal/game/exedata.bin`, embedded in the binary and read
at the original's addresses (`internal/game/exe.go`). If a game directory
holds an `ORION2.EXE`, a read past the built-in ranges falls back to it
and is logged. Such a read must be added to the data with
`go run ./tools/exedata`, or players without the executable crash there.
`OPENORION2_NO_EXE=1` turns the fallback off. Spec:
`docs/formats/exedata.md`.

### Network play

The game logic above the original's `Net_*` calls is ported as it was.
Only the transport under it is new (port-only).

- `internal/netplay`: the transports (TCP with the creator's machine as
  relay, WebSocket for the browser, in-memory for tests) behind
  `netplay.Network` and `netplay.Endpoint` (`endpoint.go`); LAN discovery
  over UDP (`discovery.go`).
- `internal/game/net.go`: the `Net_*` API over `internal/netplay`;
  `netmox.go`, `netturn.go` and the other `net*.go` files: the original's
  network game logic.
- `internal/lobby`: the online lobby's client, one more `netplay.Network`.
- `lobbynet/`: the lobby's protocol (`proto/`, `lobbyv1/`), framing
  (`wire/`), login (`srp/`) and client (`client/`). It is outside
  `internal/` because the lobby server, a separate project, imports it.
  Nothing here imports the server.
- `tools/lobbyfake`: a lobby without accounts, for development.

Specs: `docs/net/README.md` (start here), `docs/net/transport.md`,
`docs/net/turn.md`, `docs/net/hardening.md`, `docs/lobby/client.md`.

#### The desync check

With `-netcheck`, and always in a lobby game, each machine hashes its state
after every turn and the hashes are compared (`internal/game/netcheck.go`,
`netStateHash`). The hash covers the state as it would be saved, except
what differs between machines by design: the viewer's player, map
position, settings, slots past a count. `tools/savediff` leaves out the
same fields. If you add state that may legitimately differ between
machines, add it to both lists, or every network game reports a desync.
Spec: `docs/net/hardening.md`, "The desync check".

### The remote control

`-control addr` serves an HTTP API (`internal/host/control.go`): the
screen as PNG, clicks, keys, and `/state` and `/act` for programs that
play by numbers (`internal/game/agentview.go`, `agentstate.go`,
`internal/playview`). Everything that plays the game from a program
(`tools/agent`, `tools/monkey`, `tools/aiplay`) goes through it and adds
no hooks to the game's loops. Specs: `docs/control.md`, `docs/aiplay.md`.

The training simulator is the one exception: it runs inside the package,
behind the `aitrain` build tag (`internal/game/aitrain.go`,
`tools/aitrain`), through the same `stepOverride` hook the tests use.
Spec: `docs/aitrain.md`.

### Browser build

The same game, engine and mixer compiled to WebAssembly. Only the host
differs (`internal/host/web.go` and the `*_js.go` files);
`cmd/openorion2/web_js.go`, `web/`, `tools/web/build.sh` and
`cmd/webserve` build and serve it. A new method of `host.System` needs its
stub in `internal/host/*_js.go`. Spec: `docs/web.md`.

### Releases and updates

`tools/release` builds the binaries for every platform and signs their
checksums with the release key. `internal/update` checks GitHub for a newer
release at the start, verifies the signature and replaces the binary
(port-only). Spec: `docs/release.md`.

### Other tools

- `tools/savediff`: two saves, field by field.
- `tools/monkey`: plays a running game at random over `-control`.
- `tools/nettest`: network games between two machines.
- `tools/cleardir.sh`: empties or creates scratch directories.

## What is not here

The reverse engineering tools (the disassembly the specs cite as
`build/orion2.dis`, the traces of the original the port was checked
against) are not part of this repository. The specs and the tests are
what is left of them. The original's game files are never in the
repository either: `game/` and `build/` are gitignored.
