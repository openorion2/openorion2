# Status

OpenOrion2 is an open-source reimplementation of Master of Orion II:
Battle at Antares (the DOS release, `ORION2.EXE`). The game logic was
rebuilt routine by routine from the original executable and checked
against it: the same random number draws, the same arithmetic, the same
AI decisions, the same screens pixel for pixel, and the original's save
files load and continue.

## What works

- The whole single-player game: new game and galaxy generation, the main
  screen and every screen and popup reachable from it, turns, AI, combat
  (strategic and tactical), diplomacy, the council, events, Antarans,
  endings, the Hall of Fame, videos, music and sound.
- Hotseat.
- Network play over TCP (in place of IPX) and over WebSocket for the
  browser build ([docs/net/](net/)).
- The online lobby, the Galactic Council (port-only,
  [docs/lobby/client.md](lobby/client.md)): accounts, chat, private
  messages, game listing, games relayed by the server.
- A browser build ([docs/web.md](web.md)).

## Not ported

- Modem and null modem play (their buttons are disabled).

## Known departures

- **No IPX.** The original's network layer (IPX, modem and null modem
  under its `Net_*` calls) is gone; the port puts TCP in its place: the
  creator's machine relays every message (TCP port 52525), the browser
  build connects over WebSocket, and the online lobby relays through its
  server. The game code above `Net_*` is ported as it was, so a network
  game plays as the original's does. Games on the LAN, which IPX listed
  by itself, are found by a UDP question on port 52525. OpenOrion2 does
  not play network games with the original
  ([docs/net/transport.md](net/transport.md), [docs/net/ui.md](net/ui.md)).

The original sometimes reads memory it never wrote (stack and arena
leftovers). Where that changes the game, the port models the leftovers
it could pin down and uses zeros otherwise:

- `Interceptions_` reads a star's intercept slot from the stack without
  writing it; the port models the tactical-combat case, not the strategic
  one, so an AI ship may in rare cases stay where the original moves it.
- AI arena bytes the original never writes are zeros in the port.
- Stack leftovers in the messages about losses during a retreat are not
  modelled.
- `Print_Galaxy_Map_Scanned_Star_` takes colours from an uninitialised
  array; the port draws defined colours.
- Tactical combat: memory a wild missile index reaches past the arena
  reads as zeros.

## Documentation

The specs under [docs/](.) describe each file format
([docs/formats/](formats/)), the engine ([docs/engine/](engine/)), the
screens ([docs/screens/](screens/)), the turn ([docs/turn/](turn/)), combat
([docs/combat/](combat/)), network play ([docs/net/](net/)) and the lobby
([docs/lobby/](lobby/)), with the original's routine names and addresses.
The code links to them.
