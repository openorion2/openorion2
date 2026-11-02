# Remote control (development)

`openorion2 -control 127.0.0.1:7000` serves a small HTTP API to play the
running game from another program as a person would: look at the screen,
move the pointer, click, type (`internal/host/control.go`). The pointer
glides to its target over a few frames before a click, so whoever watches
the window sees it move. It works with a window and with `-headless`.

| Request | Does |
|---|---|
| `GET /screen.png` | the frame on screen, 640×480 |
| `POST /click?x=&y=[&button=right]` | glide there, click |
| `POST /move?x=&y=` | glide there |
| `POST /drag?x=&y=&tox=&toy=` | press, glide, let go |
| `POST /press?x=&y=[&button=right]` | glide there and hold the button |
| `POST /release` | let the buttons go |
| `POST /key?name=` | a key: `enter`, `esc`, `f10`, `t`, `pgup`, `alt+e`… (`host.ParseKeys`) |
| `POST /text?s=` | typed text (as `Key.Text` where the lobby takes any language) |
| `POST /paste?s=` | a Ctrl+V of `s` |

| `GET /state` | the screen, its box and options, the player's empire (JSON, `internal/game/agentstate.go`) |
| `GET /fields` | the screen's input fields as the last input poll saw them |
| `POST /act` | an order in JSON, carried out on the main screen (below) |

Coordinates are the game's, 0..639 × 0..479. A command answers once its
gesture has played and a few frames more were drawn.

The API has no authentication: bind it to 127.0.0.1 and reach another
machine through an SSH tunnel (`ssh -L 7001:127.0.0.1:7000 host`).

## Playing by numbers (/act)

`/act` takes a JSON body at the game's next input poll on the main screen
and answers in one line (`agentstate.go`, `agentview.go`; the types are
`internal/playview`'s; a trained network plays through it, `docs/aiplay.md`):

- `{"move": {"from": "Sol", "to": "Alpha", "kind": "scouts"}}`,
  `{"jobs": {"colony": "Earth", "focus": "industry"}}`: by name, for
  `tools/agent`.
- `{"view": {}}`: what a human in PLAYER_NUM's place sees, by record
  number: the setup; his empire (`research_options`, the applications he
  may research now, and `research_due`, a choice to be made); his designs;
  each empire (race and relations once met); each star (its nature,
  visited, colonies of his and of others, a blockade, and the ships at or
  bound for it: his, the others' his scanners detect, monsters); each
  planet (its nature; owner, outpost, climate and farms once its star was
  visited, the population of his own colonies only); his colonies (jobs,
  output, queue, buildings, `can_build`); his ships (`can_order`: the
  game would take an order now).
- `{"orders": {...}}`: a turn's orders, as the screens give them. It
  answers `{"ok": true, "refused": [...]}`.
  - `research`: applications; the first he may research is chosen, when
    a choice is due.
  - `queue`: {colony id: codes}; the first the colony may build goes to
    the head of its queue (none if the head is already one of them).
  - `jobs`: {colony id: [farmers, workers]} (the rest scientists),
    colonists moved one at a time as on the colonies screen; with
    `"feed": true` farmers come back where the empire would starve.
  - `dest`: {ship id: star}; a ship at a star leaves for it, one in flight
    turns there when it may (`can_order`).

Queue codes: a building (1..48) or a special item (-2..-15) as the queue
holds it; a new ship as −1000 − (design type × 8 + hull size): warships of
the hull sizes (−1000..−1005), a colony ship (−1008), a transport
(−1016), an outpost ship (−1032).

## A window on another machine

The game must run in the desktop session to show its window. On Windows,
over SSH, start it through the task scheduler in the logged-in user's
session:

```
schtasks /create /f /tn openorion2 /sc once /st 00:00 /it ^
  /tr "C:\openorion2\openorion2.exe -noupdate -control 127.0.0.1:7000 -game C:\openorion2\game"
schtasks /run /tn openorion2
```
