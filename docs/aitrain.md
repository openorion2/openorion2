# Training a network that plays (port-only)

A neural network learns to play an empire: first by imitating the original
AI (behaviour cloning), then by playing against the original AIs and
learning from the outcome (reinforcement learning, PPO). What plays a game
with a trained network, the networks themselves and their results are
`docs/aiplay.md` and `tools/aiplay/models/README.md`; this is how they are
made.

Nothing of it is in the game. The simulator is part of package `game` (it
needs the game's state and its turn) but builds only with the `aitrain`
tag: `internal/game/aitrain.go`, `aitrain_view.go`, and `tools/aitrain`,
which calls `game.Simulate`. It reaches the turn through `stepOverride`,
the hook the tests already use to take over a step of `Next_Turn_Calc_`.

    go build -tags aitrain -o build/aitrain ./tools/aitrain
    go test -tags aitrain ./internal/game -run TestSimObserveControl

## The pieces

| | |
|---|---|
| `internal/game/aitrain.go` | the simulator: AI-only games without screens, the empires' turns observed and their decisions read, a network's decisions applied in an empire's place, results |
| `internal/game/aitrain_view.go` | the observation: the player's view (`playerView`, the one `-control` serves) as the network's rows (`aiplay.FromView`) |
| `tools/aitrain` | the command: data for imitation, evaluation of a network, RL's games |
| `tools/aitrain/train.py` | behaviour cloning |
| `tools/aitrain/rl.py` | reinforcement learning (PPO), a server the games ask |
| `tools/aiplay/model.py`, `data.py`, `serve.py` | the network, the rows' features, a network as a process (shared with playing; `train.py` and `rl.py` import them from there) |
| `tools/aiplay/models/` | the trained networks |

Python 3.12 with PyTorch and numpy: `uv venv -p 3.12 build/venv && uv pip
install -p build/venv torch numpy`.

## The simulator

`game.Simulate(sys, dir, &SimOptions{...})` plays `Games` games, game i
from seed `Seed + i`: a new game with settings drawn from the seed
(difficulty, galaxy size, age, 2..8 players by size, tech level; the
default rules, not the player's `MOX.SET`), every empire an AI (the
autopilot, no council screens), then `Next_Turn_Calc_` until one empire is
left, the council elects someone or `Turns` (default 400). A game of 400
turns takes about 2 s; a panic loses that game only.

In each turn, the step override:

- before `All_Colony_AI_` observes each empire kept (all, or a `Sample`
  share), as a human in its place would see it (below);
- before `Make_Scrap_Ships_Dead_`, after `All_Colony_AI_`, `Move_All_AI_`,
  `All_AI_Colonize_` and `All_AI_Tech_Select_`, reads what the AI decided
  (`simDecide`): the research when it chose one, each colony's queue and
  farmers, workers and scientists, each ship's destination (or staying);
- for empire `Control` with a `Policy`, puts the network's decisions over
  the AI's (`simApply`: the first legal candidate of each; a ship kind
  becomes a new ship of a design of that kind). Combat, diplomacy and the
  council stay the AI's.

`Record` gets each observed turn with the decisions, `Result` how empire
`Control` fared at the end (`SimResult`: rank by population then
technology fields, colonies, population, eliminated). `Snapshot` saves the
game every `SnapshotEvery` turns with `Control` as the human, to look at in
the game.

### What the empire sees

The observation is the view `-control` serves a player
(`internal/game/agentview.go`, `docs/control.md`), turned into rows by
`aiplay.FromView`; `TestSimObserveControl` checks that a game played over
`-control` gives the same rows. Its own colonies and ships in full; the
planets of the stars it explored, of the others' colonies the owner only;
the others' ships as its galaxy map shows them (`Find_Ship_Stacks_` with
`Remove_Non_Detected_Ships_` as that player); the other empires' race,
relations and treaties after contact. Not the AI's personality, objective
or ship missions. And what the empire may do now: the applications it may
research and whether a choice is due, the queue codes each colony may put
at its head, whether each ship takes orders (`Can_Order_Ship_`). Besides, a
little memory the game does not keep: the turns a ship has stood at its
star, a colony has built its queue's head (`simTrack`).

Owners are relative (0 the empire, 1..7 the others in turn, 8 and up
monsters and Antarans, -1 none). A ship in a queue is recorded as its kind,
`-1000 - (type*8 + size)`. The rows' columns: `docs/aiplay.md`, "Rows".

## The command

    build/aitrain -game game -games 300 -seed 1000000 -sample 0.34 -o build/aidata/s0

writes int32 tables into `-o` (the game's archives are linked into a scratch
directory: nothing is written into `game/`):

| Table | Rows | Columns |
|---|---|---|
| `games` | per game | id, difficulty, size, age, players, tech level, stars, planets |
| `stars` | per game star | x, y, size, class, special, wormhole, nebula |
| `planets` | per game planet | star, orbit, type, size, gravity, minerals, special |
| `recs` | per record | game row, turn, empire, colonies, ships |
| `me` | per record | the empire's 112 columns, then the research decided (2) |
| `des` | per record | the six designs, 9 numbers each |
| `pl` | per record | the eight players from the empire on, 10 numbers each |
| `star` | per record and star | 9 columns |
| `pn` | per record and planet | 6 columns |
| `co` | per record and own colony | 29 observed, then the queue (7) and jobs (3) decided |
| `sh` | per record and own ship | 19 observed, then the destination decided |

`meta.json` has the widths and counts. Shards of a few hundred games each
(`s0`, `s1`, ...) are what `train.py` reads.

Other modes:

- `-policy "cmd"` plays empire `-control` (0) with a network (`cmd` runs
  `serve.py`, or `tcp://host:port` reaches `rl.py`) and writes
  `results.jsonl`, a line per game; `-baseline` writes the same for the AI
  in its place, on the same seeds, to compare.
- `-dagger share`: with `-policy`, records empire `-control`'s turns
  labelled by the AI's decisions while the network plays them (DAgger), the
  AI acting on that share of them.
- `-saves dir -every 25`: saves of the first game into a copy of the game
  directory, to look at in the game.

## Behaviour cloning

    build/venv/bin/python tools/aitrain/train.py build/aidata/s* -o build/policy.pt

`model.Policy` (2.4M parameters at the defaults `--d 192 --layers 4 --heads
6`) is a transformer over tokens: one for the empire (its numbers, the
designs, the players), one per star (its planets summed in), one per own
colony and per own ship. Heads:

- research: 212 applications, masked to those it may research; learned only
  on the turns the AI chose;
- queue head: 74 classes (buildings, specials, ship kinds), masked to what
  the colony may build;
- jobs: farmers and workers, each 0..population;
- ship destination: a pointer to a star or "stay", with a term from the
  distance and the fuel range; learned on the ships that take orders, a
  ship sent somewhere weighing 3 (staying is most of the labels);
- a value head for RL.

The loss is the heads' cross entropies. Records of about the same size are
batched together; a record keeps at most 48 ships and 32 colonies. The last
shard (`--val`) is held out. `--init` continues a checkpoint; `--planets`
makes planets tokens of their own and `--dist` biases attention by map
distance (neither helped). Checkpoints carry their config (`model.load`).

Imitation tops out near the AI it copies: research and queues it learns
well; jobs and above all the ship destinations worse than the AI plays them
(a target chosen as the AI would 29% of the time).

## Reinforcement learning

    build/venv/bin/python tools/aitrain/rl.py --init build/policy.pt --port 7700 -o build/policy-rl.pt
    build/aitrain -game game -games 100000 -seed 3000000 -policy tcp://127.0.0.1:7700 -o build/rl-run   # several of them

`rl.py` is a TCP server; `tools/aitrain -policy tcp://...` processes play
games and ask it each turn for empire 0's decisions. It answers a batch at
a time on the GPU, sampling from the masked distributions (`--heads`: `r`
research, `q` queues, `j` jobs, `d` ships; the rest stays the AI's), keeps
each game's turns, and after every `--steps` turns (16384) runs PPO on
them (`--epochs 3`, `--batch 512`, `--lr 2e-5`, `--clip 0.2`, `--vf 0.5`,
`--ent 0.0005`, GAE with `--gamma 0.995 --lam 0.95`), a game's turns in
pieces of `--piece` (64). It saves `-o` after each update.

Reward per turn: the growth of the population (`--r-pop`) and colonies
(`--r-col`), and `--r-fields` per technology field researched; at the end
of a game the rank, weighted `--r-end` (+1 first down to -1 last, -1
eliminated). A turn's decisions are one action by default (`--ppo sum`:
the ratio of their product is clipped); `--ppo per` clips each decision,
which learned slower.

## Evaluation

    build/aitrain -game game -games 100 -seed 2000000 -o build/eval/x -policy "cd tools/aiplay && ../../build/venv/bin/python serve.py build/policy-rl.pt rq 0.5 sample"
    build/aitrain -game game -games 100 -seed 2000000 -o build/eval/ai -baseline

100 games on the same seeds, the network as empire 0 against the AI in its
place: the mean rank (0 first, 1 last), the eliminated, the first, and the
games where the network ranked better or worse than the AI's own empire.
Between two evaluations of one network the rank moves by up to 0.05.

| network | decides | rank | eliminated | first | vs the AI |
|---|---|---|---|---|---|
| the original AI | | 0.506 | 44 | 26 | |
| imitation, seeing everything | all | 0.84 | 72 | 5 | |
| imitation, seeing everything | research, queues | 0.58 | 45 | 22 | |
| imitation, a human's view (`imitation.pt`) | research, queues | 0.59 | 43 | 18 | +24/-36 |
| RL, 131 iterations (an earlier observation) | research, queues | 0.481 | 34 | 30 | +47/-27 |
| RL, 816 iterations (`rl-growth.pt`), sampling | research, queues | 0.508 | 51 | 27 | +38/-28 |
| `rl-survival.pt`, sampling | research, queues | 0.504 | 51 | 27 | +36/-31 |

The RL networks grow empires twice the AI's in population, rank about as
well and die somewhat more often. A larger network (12M parameters)
imitated slightly better; planets as tokens with distance-biased attention
did not help.

## On a GPU server

The data is cheaper made where it is used: `tools/aitrain` cross-compiles
(`GOOS=linux CGO_ENABLED=0 go build -tags aitrain`) and needs only the game's
archives the simulation opens (about 9 MB). 3000 games take a few minutes
on 96 cores; a training of 12000 steps of 256 records about 7 minutes on
one GPU; for RL the games' processes run on the CPUs and `rl.py` on one
GPU. A shared server wants the work in `screen`
sessions (a dropped ssh connection must not stop it) and batches that fit
the GPU memory left by others.

## Network tests

`tools/nettest` plays network games between two machines through the real
lobby with a network on each side (`tools/aiplay` over `-control`), game
after game, and records what goes wrong: desyncs, crashes, hangs (a run
ends after five minutes on one stardate). `tools/nettest/README.md`.
