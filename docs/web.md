# Browser build

The port also runs in a browser as WebAssembly (`GOOS=js GOARCH=wasm`). Only
the host layer differs; the game, the engine and the mixer are the same code,
so fidelity is unchanged. It is meant for a private host: the original's
files are served from the host's own `game/` directory and never go into the
build or the repository.

## Build and serve

    tools/web/build.sh                  # build/web: index.html, fs.js, host.js, wasm_exec.js, openorion2.wasm(.gz)
    go run ./cmd/webserve               # http://localhost:8080/
    go run ./cmd/webserve -addr :8080 -auth user:pass -game /path/to/game

`cmd/webserve` serves `build/web` at `/` (openorion2.wasm gzipped when the
browser takes it), the game files at `/game/NAME` (range requests work) and
their list at `/game.json`. `-auth` (or `OPENORION2_AUTH`) adds HTTP basic auth;
put it behind TLS when it faces the internet. For another server, copy
`build/web`, serve `game/` under `game/` and write `game.json` as webserve
does (`[{"name","size","mtime"}]`, mtime in ms).

URL parameters: `?load=N` continues SAVE<N>.GAM, `?logos=off` skips the
start-up logos; `?host[=:PORT]`, `?join=HOST[:PORT]` and `?name=NAME` are
the network flags (`docs/net/web.md`). webserve's `-net=false` turns its
`/net` WebSocket off, `-netgames N` bounds the games browsers create. Alt+Enter toggles full screen.

## How it works

- **Host** (`internal/host/web.go`, `web/host.js`): `host.Run` for `js`.
  `requestAnimationFrame` signals a channel; the host goroutine then drains the
  page's events, runs the shared `tick`, mixes sound and copies the RGBA frame
  (with the game's cursor drawn in, the system pointer hidden) to a canvas.
  The canvas holds the frame enlarged by a whole factor with nearest neighbour
  and the browser scales it smoothly to the largest 4:3 box (sharp bilinear,
  as `present.go`). JS callbacks never block.
- **One thread.** Go in the browser has no preemption: a game loop that polls
  input or the clock without sleeping would freeze the page. `System.poll`
  and `Elapsed` call `yield`, which on `js` sleeps 1 ms after 8 ms of polling
  (`cooperative`; a no-op elsewhere). Long computations (the AI's turn) block
  the page until they finish, frames included.
- **Clicks** are held at least four host frames: a release that comes sooner
  (a tap, a synthetic click) waits, so the game sees the press as the
  scripted clicks of the other hosts make it.
- **Keys**: `KeyboardEvent.code` maps to the USB HID usage (what
  `engine.biosWord` reads) and the SDL keycode. Cmd shortcuts stay the
  browser's.
- **Sound**: `audio.Mixer` output is scheduled as Web Audio buffers 0.12 s
  ahead. Until the first click or key press the browser forbids sound; the
  mixer is then drained at the real-time rate so voices and streams keep time.
  Hidden tabs stop `requestAnimationFrame`, so the game pauses at its next flip.
- **Files** (`web/fs.js`): Go's `os` package on `js` calls a Node-style
  `globalThis.fs`; fs.js implements it over one directory, `/game`, with
  case-insensitive names. Game files are fetched on demand: up to 4 MB, or read
  mostly at once, whole and kept (LRU, 256 MB); larger ones by HTTP range
  (music, videos). Everything the game writes (saves, MOX.SET, HOF.M2) goes to
  IndexedDB (`openorion2` / `files`) and shadows the server's file of that name;
  the server's `game/` is never written. Saves therefore live per browser.

## Limits

- `WarpMouse` (Set_Pointer_Position) cannot move the real pointer: the game's
  cursor jumps there and back with the next mouse move.
- Network play goes through webserve's `/net` WebSocket
  (`docs/net/web.md`); browser and native players can share a game.
- To clear the browser's saves: DevTools → Application → IndexedDB → openorion2.
