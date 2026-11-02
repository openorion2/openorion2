# Contributing

Thank you for helping. OpenOrion2 has one rule above the others: the game
must keep playing exactly as the original does. Most of this file is about
how to change code without breaking that. `ARCHITECTURE.md` says where
things are; `CLAUDE.md` holds the same rules in short form.

## What you need

- Go 1.27 or newer. SDL3 is embedded: nothing else to install.
- For most game tests: your own copy of Master of Orion II, version
  1.40b23 (the GOG release), in `game/` at the root of the checkout. A
  symlink to your install will do. Without it those tests skip. That is
  all they need: no saves. The tests make their own from the game files
  (a new game by a fixed seed, played on by the AI; `gensave_test.go`),
  the same on every machine, and never read the saves in `game/`.

`game/` and `build/` are gitignored. Never commit anything from them.

## Building

```
go build ./cmd/openorion2
./openorion2 -game /path/to/game
```

`go run ./cmd/openorion2 -h` lists the options. The browser build:

```
GOOS=js GOARCH=wasm go build ./cmd/openorion2
```

## Testing

```
go test ./internal/... ./cmd/... ./lobbynet/...   # everything, about half a minute
go test -race ./internal/...                      # must stay clean
OPENORION2_NO_EXE=1 go test ./internal/...        # as players run it, without ORION2.EXE
go test -tags aitrain ./internal/game ./tools/aitrain   # the training simulator
gofmt -l .                                        # must print nothing
```

- **`-race`.** Tests run in parallel (`t.Parallel`). Keep new tests
  parallel-safe: no package globals. Hooks go through `game.Options`.
- **`OPENORION2_NO_EXE=1`.** Without the variable, a read of the original's
  data past `internal/game/exedata.bin` falls back to `game/ORION2.EXE`
  and is logged. With it, such a read panics, as it would for a player
  without the executable. If your change reads new data from the
  executable, run `go run ./tools/exedata -exe game/ORION2.EXE` and commit
  the new `exedata.bin` (`docs/formats/exedata.md`).
- **`-tags aitrain`.** A change to the player's view or to the turn's steps
  can break the simulator without breaking anything else.
- **The browser build.** A new method of `host.System` needs its stub in
  `internal/host/*_js.go` too, or only the WebAssembly build breaks.
- **Watching a test.** `OPENORION2_WATCH=1 go test ./internal/game -run X`
  shows a test's headless runs in a window.
- **Slow tests.** A test that alone takes minutes is suspect: usually a
  script waiting for a screen that never comes. The headless host runs on
  virtual time (a frame is 1/60 s of game time) and never advances on wall
  time.

### Tests and the original's files

CI (`.github/workflows/ci.yml`) has no `game/`. It builds every platform,
runs `go vet`, `gofmt`, the tests that need no game files and `-race`. A
test that needs the game's files must skip without them, and skip in the
test's own goroutine: a `t.Skip` inside `host.Run`'s callback leaves the
host waiting forever.

Tests never write into `game/`. They work in a scratch copy
(`scratchGameDir` in `internal/game/turn_e2e_test.go`), which must be a
real directory, not a symlink to `game/`. Do the same when you run the
game by hand for a test: the game writes saves, settings and the Hall of
Fame into its directory.

## The rules

### Fidelity

A change to game logic must keep what the original does: the same random
number draws in the same order, the same integer arithmetic (widths,
signedness, truncation, overflow), the same order of loops, the same AI
decisions, the same pixels. Original saves must still load.

The specs under `docs/` describe the original and are the source of truth.
Before you change a routine, read its spec. If the code and the spec
disagree, say so in the issue or PR; do not just pick one.

How to check that you kept the behaviour:

- the tests above, with your own `game/`;
- a characterization test of the routine you changed (see the recipe
  below), written before the change;
- for whole turns: two headless runs from the same save and seed, before
  and after, compared with `go run ./tools/savediff -all a.GAM b.GAM`.

The traces of the original that the port was checked against are not in
this repository. Your tests compare the port with itself before and after
your change. That is enough for a refactoring. A change that is meant to
alter behaviour (a bug fix where the port differs from the original) needs
an argument from the spec or the original's code in the PR.

### Port-only code

What the original does not have (the online lobby, TCP and WebSocket play,
window handling, the update check, the remote control) is marked
`port-only` in a comment and documented in `docs/`. Keep port-only code
out of the ported routines where possible. A program that plays the game
(an agent, a fuzzer, a network) plays it from outside, over `-control`
(`docs/control.md`), and adds no hooks to the game's loops.

### Network state

The desync check hashes the state as it would be saved, except what may
differ between machines (`netStateHash` in `internal/game/netcheck.go`).
`tools/savediff` leaves the same fields out. If you add state that differs
legitimately between machines, add it to both, or every network game
reports a desync (`docs/net/hardening.md`).

### No game files

Never commit the original's files or anything derived from them: LBX
files, saves, extracted graphics, videos, sounds, screenshots of the
original. `internal/game/exedata.bin` is the one exception, and only as
`tools/exedata` writes it.

### Code style

- `gofmt`. CI fails otherwise.
- Comments in English, at most two lines in a row. `any`, not
  `interface{}`.
- A ported function's doc comment names the original routine and its
  address, and links to the spec when there is one.
- `README.md` has translations (`README.ru.md`, `README.zh-CN.md`,
  `README.de.md`, `README.fr.md`). A change to it goes into them too.

## Branches, commits and pull requests

- Fork the repository and work on a branch. Pull requests go to `master`.
- Before you open one: the tests and `-race` on the packages you touched
  pass, and `gofmt -l .` prints nothing.
- Say in the PR which tests you ran with `game/` present. CI cannot run
  them.
- Commit messages say why, not only what. "Keep the RNG order in
  Move_Settlers_" helps a reviewer more than "fix".
- One change per PR. A refactoring and a behaviour change go in separate
  PRs.

## Recipe: rewrite a ported function into readable code

Much of the port reads like the original: short names, offsets, flags as
bytes. Making it readable is welcome, as long as behaviour stays the same.

### 1. Pick a routine and read its spec

Find the Go function and the original routine named in its doc comment,
for example `allocateMsgSlots is Allocate_Msg_Slots_ (0xeeb3a)` in
`internal/game/turn.go`. Search `docs/` for the routine name. For a turn
step, `docs/turn/overview.md` lists it with what it touches and whether it
draws random numbers (`R!` directly, `R` somewhere below, `-` never).

Start with a small routine that does not draw random numbers and has no
screen.

### 2. Find its tests, or write a characterization test

Look for existing tests first: `grep -rn functionName internal/ --include=*_test.go`.
If there are none, write one before you touch the code. What it checks
depends on the routine:

- **A pure function** (arithmetic on its arguments): a table test, like
  `TestIsqrt` or `TestParsecsBetweenPoints` in
  `internal/game/moveorders_test.go`. Include edge values: zero,
  negative, the limits of the integer type.
- **A routine that changes the game state**: load the saves, run the
  routine, and record the result. `moveOrderGames` (in
  `moveorders_test.go`) and `techGame` (in `tech_test.go`) load the
  tests' saves (the recipes of `gensave_test.go`: `recipeState`,
  `recipeSave`) into a `Game` without a screen. After the call,
  `g.netStateHash()` is a hash of almost the whole state, and
  `g.state.Random.Get()` is the RNG seed. Record both for each save
  before the change and compare after it. If the routine touches the
  fields `netStateHash` leaves out (the viewer's player, map position,
  settings), check those fields directly.
- **A screen or a whole turn**: a headless run with `host.Run`, as in
  `internal/game/logos_test.go`: `scratchGameDir`, a recipe's save in
  (`putRecipeSave`),
  `newGame`, `continueSave`, run, compare what you can observe (the
  state, the frames shown). For a manual check, run
  the same headless script (`-headless -game COPY -load N -seed S
  -frames F`, with `-click` and `-key` to play and save) on a copy of the
  game directory before and after, and compare the two saves with
  `go run ./tools/savediff -all`.

A characterization test may stay in the repository if it is fast and
skips without `game/`. It uses the recipes' saves, never your own: a
recipe's save is the same for everyone with the GOG files, so values
captured from it may be pinned. A situation no recipe reaches (a given
combat, a building) is made by editing the loaded state in the test; a
new recipe goes into `saveRecipes`.

### 3. Refactor

Change names and structure. Keep everything the player could observe:

- **Integer widths and signedness.** If the original computes in 16 bits,
  keep `int16` or `uint16` where the value can overflow or is truncated.
  `int` is not a safe replacement.
- **Arithmetic as written.** Keep the order of operations where it changes
  rounding: `a*b/c` is not `a/c*b` in integers. Keep shifts that stand for
  signed division as they are unless you are sure they are the same.
- **The RNG.** Every `Random` call stays, in the same order, with the same
  argument. Do not move a draw out of a loop or into a condition.
- **Loop order.** If the original loops from the last record down, so does
  the port.
- **Reads of the executable's data.** Keep reading from the same addresses.
  A table you copy into a Go literal must match `exedata.bin` byte for
  byte.
- **Side effects.** Writes to the state, including ones that look useless,
  stay. Later code or the save may read them.

Keep the doc comment's link to the original: the routine's name and
address stay, so the next reader can still find the spec:

```go
// messageSlotsPerPlayer is Allocate_Msg_Slots_ (0xeeb3a): ...
```

Comments: English, at most two lines in a row.

### 4. Run the tests

```
go test ./internal/game -run YourTest
go test ./internal/... ./cmd/... ./lobbynet/...
go test -race ./internal/game
OPENORION2_NO_EXE=1 go test ./internal/game
go test -tags aitrain ./internal/game ./tools/aitrain
gofmt -l .
```

If the routine runs in network games, the network tests in
`internal/game/net*_test.go` exercise it on several machines at once and
compare their states.

### 5. Open the PR

Say which routine you rewrote, which spec you read, how you checked that
nothing changed (the test, the saves you ran it on), and that `-race`
passed.

## Reporting bugs

Open an issue. Useful to include: the platform, the OpenOrion2 version
(`openorion2 -version`), what happened and what the original does in the
same situation, and a save from just before (yours, not the game's files).
A network game's desync report carries both machines' saves; attach them.

## Conduct

Everyone taking part follows the [Code of Conduct](CODE_OF_CONDUCT.md).
