# Announcements (port-only)

The project tells every player what they need to know (a new version, a
lobby restart, the lobby being down) through one file:
`https://announce.openorion2.org/announce.json` (`announce.URL`). A name of
its own, so that the file can be served from elsewhere when the lobby's
server is down.

```json
{"messages": [
  {"id": "restart", "level": "warning", "text": "The lobby restarts at 16:00 UTC.",
   "from": "2026-11-05T12:00:00Z", "until": "2026-11-05T16:05:00Z"}
]}
```

- `id`: names the message; a changed text under the same id is new.
- `text`: what the player reads (ASCII: the game's fonts have no more).
- `level`: `info` (the default, drawn in green) or `warning` (in orange); the main menu draws them on black in a green frame.
- `from`, `until` (RFC 3339, optional): the message shows from `from` and
  not after `until`, so that it can be written ahead.

The game (`internal/announce`, `internal/game/announce.go`) fetches it at
the start and then every minute (`announce.Every`: a restart is announced a
minute ahead), in the background, and shows the active messages:

- on the main menu, one line above the picture, cut to the screen's width;
- in the lobby, a note in the chat ("Announcement: ..."), each new message
  once;
- in a game played through the lobby, a notice (a GNN line while waiting
  for the others, a message box on the main screen), each new message once.

`-noannounce` turns it off, `-announce URL` reads another file (development);
headless runs fetch nothing unless given `-announce`. The server's operators
publish the messages (openorion2-deploy: `announce.yml`); a restart of the
lobby announces itself a minute ahead.
