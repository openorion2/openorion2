# Playing with a trained network (port-only)

A network trained on the original AI's games, then by reinforcement
learning, can play an empire in place of a human: in a single-player game,
or a network game where every machine's empire is a network's
(`tools/nettest`). The game has no part in it: the
network plays from outside, over the remote control (`docs/control.md`,
"Playing by numbers").

    openorion2 -control 127.0.0.1:7100 ...
    go build -o build/aiplay ./tools/aiplay
    go run ./tools/agent -nollm \
      -turncmd "build/aiplay 'cd tools/aiplay && python3 serve.py models/rl-survival.pt rqjd 0.5 sample'" \
      127.0.0.1:7100

Each turn, on the main screen before TURN, `tools/agent` hands `/act`'s
view to `tools/aiplay`, which turns it into the network's rows
(`internal/aiplay`), asks the network (`serve.py`, a child process, one
JSON line each way) and answers its decisions as `/act` orders. The agent
answers the other screens by fixed rules: boxes yes or ok, a settler's
first planet, research choices the network did not make, AUTO in tactical
combat. Diplomacy and the council are whatever those screens' defaults do.

`serve.py` needs Python 3 with `torch` and `numpy` (a CPU is enough: a turn
takes some tens of milliseconds). Its arguments: the network
(`models/`, `tools/aiplay/models/README.md`), the heads it decides with
(`r` research, `q` queue heads, `j` jobs, `d` ship destinations: in a
human's place all four, `rqjd`, since nothing else plays the empire), the
share above which a ship stays (0.5), and `sample` to draw from its
distributions as reinforcement learning played (else its most likely
choice).

## What the network sees

What a human in the empire's place would see (`/act` view): its own
colonies and ships in full; the planets of the stars it explored, of the
others' colonies the owner only; the others' ships as its galaxy map shows
them; the other empires' race, relations and treaties after contact. And
what it may do now: the applications it may research and whether a choice
is due, the queue codes each colony may put at its head, whether each ship
takes orders. Besides, `aiplay.Player` counts how long each ship has stood
at its star and each colony has built its queue head.

## Rows

`aiplay.FromView` (owners relative to the empire: 0 itself, 1..7 the others
in turn, 8 and up monsters and Antarans, -1 none, -2 unknown):

| Row | Columns |
|---|---|
| `Game.Setup` | difficulty, galaxy size, age, players, tech level |
| `Game.Stars` | per star: x, y, size, class, special, wormhole, nebula |
| `Game.Planets` | per planet: star, orbit, type, size, gravity, minerals, special |
| `Me` | stardate − 35000, race, 0, 0, BC, tax, research points, field, application, food, income, industry, research, population, command points total and used, freighters total and free, FTL speed, fuel range, home planet; FieldStatus (83); the applications it may research (7 words of bits); a choice due |
| `Des` | the six designs: size, type, shield, drive, computer, armor, weapons, combat speed, cost |
| `Pl` | the eight players from the empire on: exists, eliminated, race, relations, treaty, contact, 0 × 4 |
| `Star` | per star: visited, own and others' colonies, blockaded, own ships and weapons, others' ships and weapons, monsters |
| `Pn` | per planet: owner, population, outpost, climate, max farms, known |
| `Co` | per own colony: id, planet, population, farmers, workers, scientists, output (4), morale, pollution, outpost, production, marines, armor, queue codes (7), buildings (2 words of bits), the classes it may build (3 words of bits), turns on the head |
| `Sh` | per own ship: id, status, x, y, star at, star bound for, takes orders, experience, design (9), ETA, turns at its star |

The network's 74 queue classes are queue codes (`aiplay.QueueClasses`).
Training and the models' evaluation: `docs/aitrain.md`; its simulator
observes through the same view, so these rows equal training's.
