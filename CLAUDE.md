# OpenOrion2

A port of Master of Orion II (1996, DOS) to Go, with SDL3 as the host
layer. The port is complete (`docs/STATUS.md`): the game logic was ported
routine by routine from the original executable and verified against it.
The reverse engineering tools and the tests against the original's traces
are not part of this repository.

## Ground rules

- Fidelity is bit-exact: same RNG, same arithmetic, same AI decisions, and
  original save files load. A change to game logic must keep what the
  original does; the specs under `docs/` (with the original's routine names
  and addresses) are the source of truth, and the code links to them.
- Port-only additions (the online lobby, network play over TCP and
  WebSocket, window handling, Unicode text in the lobby) are marked
  `port-only` in comments and documented in `docs/`.
- Features of the unofficial community patch 1.51 come over one by one,
  each on its branch into master, by `docs/p151/process.md`
  (`docs/p151/`: what it changes and each item's fate) and are marked
  `community patch 1.51` in comments. Where 1.51 changes a rule, its rule
  replaces 1.40's; the game is one game, not two rulesets.
- Every change a player can notice gets its chapter in the player's
  manual, `docs/manual/` (from 1.40b23, with examples, and how it differs
  from 1.51), in the same change as the code.
- The game reads the original's LBX archives at run time (`game/`); the
  constant tables and short strings the original keeps in `ORION2.EXE` are
  built in (`internal/game/exedata.bin`, `docs/formats/exedata.md`; a read
  past them is logged while an `ORION2.EXE` is present: add it with
  `tools/exedata`). `game/` and `build/` are gitignored.
- The C runtime is Go's standard library, or the Open Watcom sources
  where behaviour is visible to the player (`qsort` order, printf float
  formatting, `atoi`, `stricmp`).
- A program that plays the game (an agent, a fuzzer, a trained network)
  plays from outside, over `-control` (`docs/control.md`): it may extend
  `/state` and `/act`, but adds no hooks to the game's loops. The training
  simulator alone is inside, behind the `aitrain` build tag, through the
  `stepOverride` the tests use.
- The online lobby's protocol, wire format, SRP and client are `lobbynet/`
  (outside internal/: the lobby server, a separate project, imports them;
  `docs/lobby/client.md`). Nothing here imports the server.

## Tests

- `go test ./internal/... ./cmd/... ./lobbynet/...` runs everything (about
  half a minute). Tests that need the original's files skip without
  `game/`. Headless runs use virtual time (`internal/host`): a frame is
  1/60 s of game time, so scripted runs are deterministic under any load.
- `go test -race ./internal/...` must stay clean. Tests run in parallel
  (`t.Parallel`); keep new ones parallel-safe: no package globals (hooks go
  through `Options`).
- Tests never write into `game/`: they copy it (`scratchGameDir`); a
  scratch dir must be a real directory, not a symlink to `game/`.
- Tests need only `game/` (the GOG 1.40b23 files), no saves: they make
  their own (`internal/game/gensave_test.go`: named recipes of a new game
  by a fixed seed, played on by the AI through `NewSave`, once per
  process) and never read `game/`'s. A recipe's save is the same on every
  machine, so its values may be pinned; a situation no recipe reaches is
  made by editing the loaded state.
- `OPENORION2_WATCH=1 go test ./internal/game -run X` shows a test's headless
  runs in a window.
- `OPENORION2_NO_EXE=1 go test ./internal/...` runs as players do, without
  `ORION2.EXE`: a read past `exedata.bin` panics there instead of falling
  back. A new read of the original's data needs `go run ./tools/exedata`.
- `-netcheck` hashes the state as a save but for what may differ between
  machines (`netStateHash`, `docs/net/hardening.md`). State that differs
  legitimately (a viewer's, a machine's settings, leftovers past a count)
  goes into those exceptions, or every network game reports a desync;
  `tools/savediff` leaves the same out (keep the two alike).
- `go test -tags aitrain ./internal/game ./tools/aitrain` covers the
  training simulator too (a change to the player's view or the turn's
  steps can break it unseen otherwise).
- CI (`.github/workflows/ci.yml`) has no `game/`: a test needing it must
  skip, and skip in the test's goroutine (a `t.Skip` inside `host.Run`'s
  callback leaves the host waiting forever).
- `GOOS=js GOARCH=wasm go build ./cmd/openorion2` builds the browser
  version: a new method of `host.System` needs its stub in
  `internal/host/*_js.go` too, or only that build breaks.
- A test that alone takes minutes is suspect: a script waiting for a
  screen that never comes, or a loop that neither waits nor reads input
  (the headless host never advances on wall time).

## Workflow

- Work on a branch; it goes into master once the tests and `-race` on the
  packages touched pass.
- `tools/cleardir.sh DIR...` empties or creates scratch directories.

## Layout

- `cmd/openorion2` — the game (`-headless` or `OPENORION2_HEADLESS=1`: no window,
  no real input; `-game dir` the game files, by default the program's
  directory, the current one or `game/` in either, whichever has them; `-load N` continues
  SAVE<N>.GAM; `-frames N`, `-shot f.png`, `-click`, `-key`, `-drag` for
  scripted runs; `-seed n`; `-autopilot` lets the AI play the human;
  network play: `-host [:port]`, `-join host[:port]`, `-name`, `-netdrop
  ai|end`, `-netcheck`; `docs/net/ui.md`; `-control addr` plays it over
  HTTP, `docs/control.md`)
- `cmd/webserve`, `web/`, `tools/web/build.sh` — the browser build and its
  server (`docs/web.md`)
- `internal/le` — LE executable loader (`tools/exedata`, and the fallback
  for reads past the built-in data)
- `internal/lbx`, `internal/gfx`, `internal/font`, `internal/smk`,
  `internal/sound`, `internal/audio`, `internal/save` — the original's
  formats (`docs/formats/`)
- `internal/engine` — the WLIBS engine layer: pages, palette, timing,
  fields and input (`docs/engine/`)
- `internal/game` — the game's own code: screens, turn, AI, combat
- `internal/host` — SDL3 window and event loop (`window.go`), the browser
  host (`web.go`)
- `internal/netplay` — network transports (TCP, WebSocket, in-memory)
- `internal/lobby`, `lobbynet/` — the online lobby's client side;
  `tools/lobbyfake` a lobby without accounts for development
- `internal/announce` — the project's announcements (`docs/announce.md`)
- `internal/crash` — crash reports: the runtime's output, the log's tail
  and the state, offered to the lobby at the next start
  (`docs/lobby/client.md`, "Crash reports")
- `internal/update`, `tools/release` — the update check and the release
  builds and signatures (`docs/release.md`)
- `tools/monkey` — plays a running game (`-control`) at random: a fuzzer of
  the screens that found crashes no test reached. Not on a copy whose
  lobby.yaml and lobby.session are the player's: it logs out of the lobby
  (no scripted run should: a copy of a played game dir carries them).
- `tools/agent` — plays a running game over `-control` with a local model
  (Ollama), or answers its screens while `-turncmd` (a program reading
  `/act`'s view, answering orders) plays the empire
- `tools/savediff a.GAM b.GAM` — two saves field by field, without what
  differs between a network game's machines by design (`-all`: with it);
  gzipped saves (the lobby's desync reports) too
- `internal/playview` — that view and those orders; `internal/aiplay`,
  `tools/aiplay` — a trained network plays an empire through them
  (`docs/aiplay.md`; Python with torch); `tools/aitrain` and the simulator
  (`internal/game/aitrain*.go`, build tag `aitrain`) train them
  (`docs/aitrain.md`); `tools/nettest` plays network games of networks
  between two machines
- `assets/` — the port's own art and font (the lobby's)
- `docs/` — the specs

## Code style

- Comments in English, at most two lines in a row; `any`, not `interface{}`.
- `README.md` has translations (`README.ru.md`, `README.zh-CN.md`,
  `README.de.md`, `README.fr.md`): a change to it goes into them too.
- Commit messages explain why; no `game/` content or derived binaries in
  git, but the trained networks (`tools/aiplay/models`).
