# OpenOrion2

OpenOrion2 is an open-source reimplementation of **Master of Orion II:
Battle at Antares** (1996): the game as source code anyone can read, fix,
change and keep alive. It runs on modern macOS, Windows and Linux without
DOSBox. It plays exactly like the original: the game logic was rebuilt
routine by routine from the original executable and checked against it,
down to the random number draws and the AI's decisions. Saves from the
original load and continue.

On top of the original it adds:

- native builds for macOS, Windows and Linux, on x86-64 and ARM (Apple
  Silicon included): one file, no DOSBox, no emulator, nothing to install;
- network play over TCP (in place of IPX), on a LAN or over the internet;
- the **[Galactic Council](https://openorion2.org)**, an online lobby:
  chat, private messages, and games listed and relayed by the server, so
  no port forwarding is needed;
- fullscreen (maximize the window, or Alt+Enter) and a window of any size;
- a browser build (WebAssembly).

What is ported and what is not: [docs/STATUS.md](docs/STATUS.md).

> OpenOrion2 is a fan project. It is not affiliated with or endorsed by
> the owners of Master of Orion. Master of Orion is a trademark of its
> respective owner. OpenOrion2 contains none of the original game's files:
> you need your own copy of Master of Orion II.

## Why Go?

Go is the main language of the project's author, and the AI agents that do
much of the work write any language equally well. The point is not the
language but that the game is open.

## Installing

OpenOrion2 is the program; the game's data comes from your own copy of
Master of Orion II. It is made for and tested with **version 1.40b23,
the [GOG](https://www.gog.com/en/game/master_of_orion_1_2) release**.
Other releases (the 1996 CD, 1.31, other stores) are not tested.

1. **Get the game.** Buy Master of Orion II on GOG, where it comes as
   [Master of Orion 1+2](https://www.gog.com/en/game/master_of_orion_1_2),
   and install it.
2. **Get OpenOrion2.** Download the binary for your system from the
   [releases](https://github.com/openorion2/openorion2/releases)
   (`openorion2-<system>-<cpu>`: `darwin` is macOS; `arm64` is Apple
   Silicon and ARM PCs, `amd64` the rest), or build it (below).
3. **Put it into the game's directory**, the one that holds the `.LBX`
   files (on Windows, the GOG install directory), and start it there:
   it finds the game's files next to itself.
   On macOS or Linux, copy that directory from a Windows install, or
   unpack the GOG installer for Windows with
   [innoextract](https://constexpr.org/innoextract/).
4. **The first start.** The binaries are not signed by Apple or
   Microsoft. On macOS, after `chmod +x openorion2-darwin-*`, remove the
   download quarantine with `xattr -d com.apple.quarantine
   openorion2-darwin-*` (or, after macOS refuses it once, System
   Settings, Privacy & Security, Open Anyway). On Windows, SmartScreen's
   "More info", "Run anyway".

OpenOrion2 also finds the game's files in the current directory or in a
directory named `game` next to it; anywhere else, say where:

```
openorion2 -game "/path/to/Master of Orion 2"
```

OpenOrion2 writes its saves (`SAVE1.GAM` … `SAVE10.GAM`), the settings
and the Hall of Fame into the game's directory, as the original does, and
reads the original's saves there. Work on a copy if you want to keep the
original's saves apart.

A release build looks for a newer version at the start and offers to
install it (`-noupdate` turns that off).

## Building

You need Go 1.27 or newer. SDL3 is embedded in the binary: there is
nothing else to install.

```
go build ./cmd/openorion2
./openorion2 -game /path/to/game
```

`go run ./cmd/openorion2 -h` lists the options.

Builds of your own never look for updates. Making a release:
[docs/release.md](docs/release.md).

## Playing with others

- **Hotseat**: Multiplayer, Hotseat.
- **Network**: Multiplayer, Network. Start New Game listens on TCP port
  52525 (`-host :port` picks another); Join Game finds games on the LAN, or
  `-join host[:port]` joins one directly. See [docs/net/ui.md](docs/net/ui.md).
- **Galactic Council Online Lobby**: register at
  [openorion2.org](https://openorion2.org), then Multiplayer, Galactic
  Council Online Lobby. Create or join a game there;
  the lobby relays it, so nobody needs an open port. Private messages: click
  a name, or type `/w name text`.

## Browser build

OpenOrion2 also runs in a browser, as WebAssembly: the same game code,
saves kept in the browser, and network play with native players.
[tools/web/build.sh](tools/web/build.sh) builds it and
[cmd/webserve](cmd/webserve) serves it together with your game files:
[docs/web.md](docs/web.md).

There is no public site to play it on. The browser build needs the
original's files on the server it is loaded from, and serving them to
anyone who opens a page would be handing out the game, which this project
has no right to do. Host it for yourself, on your own machine or behind
`webserve -auth` for your friends, with your own copy of the game.

## Plans

- Open the source of the online lobby's server, so that anyone can run
  one.
- Bring in as much of the community's unofficial 1.50 patch as possible.
- Sign and notarize the macOS builds, so that macOS starts them without
  a warning.

## Documentation

[docs/](docs) holds the specifications the game was rebuilt from: the file
formats, the engine, every screen, the turn, combat, network play and the
lobby, with the original's routine names and addresses.

## License

OpenOrion2 is free software under the GNU General Public License,
version 3: see [LICENSE](LICENSE).

The port's own art ([assets/](assets)) has nothing of the original game.
It includes the Fusion Pixel Font (SIL Open Font License 1.1,
[assets/lobby/font/](assets/lobby/font)) and embeds SDL3 (zlib license)
through [go-sdl3](https://github.com/Zyko0/go-sdl3).
