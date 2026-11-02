# Trained networks

Three checkpoints of `model.Policy` (2.4M parameters: d 192, 4 layers, 6
heads; the config is in each file). All three see what a human in the
empire's place would see (docs/aiplay.md, "What the network sees").

| File | How it was made | Plays best with |
|---|---|---|
| `imitation.pt` | Behaviour cloning of the original AI: 3000 AI-only games (1.5M empire turns), 12000 steps of 256 | its most likely choices (`serve.py ... rq 0.5`) |
| `rl-growth.pt` | `imitation.pt` + PPO, 816 iterations (about 40000 games against the AIs); reward: population and colony growth, technology fields, the final rank | sampling (`... rq 0.5 sample`) |
| `rl-survival.pt` | `rl-growth.pt` + 33 more PPO iterations with the reward shifted to survival and rank (`--r-end 3 --r-pop 0.003 --r-col 0.1`) | sampling |

What they decide: the research and every colony's queue head (`rq`), the
jobs (`j`) and ship destinations (`d`). The jobs and fleet heads play worse
than the AI's code; in an AI's place (training's evaluation below) the AI
decided those. In a human's place nothing else plays the empire: use all
four, `rqjd`.

100 evaluation games (seeds 2000000..2000099, the network as empire 0, the
original AI for the others); rank 0 is first, 1 last; "vs the AI" counts the
games where the network's empire ranked better / worse than the AI's own
empire on the same seed:

| | rank | eliminated | first | colonies | population | vs the AI |
|---|---|---|---|---|---|---|
| the original AI | 0.506 | 44 | 26 | 9.1 | 101 | |
| `imitation.pt` (research, queues) | 0.59 | 43 | 18 | 7.6 | 80 | +24/-36 |
| `rl-growth.pt`, sampling | 0.508 | 51 | 27 | 17.5 | 241 | +38/-28 |
| `rl-survival.pt`, sampling | 0.504 | 51 | 27 | 17.7 | 224 | +36/-31 |

The RL networks grow empires twice the AI's in population, rank about as
well as the AI and die somewhat more often (the AI's code fights for them
with fewer warships). Between two evaluations of one network the rank moves
by up to 0.05: these differences are within that.

To play one: docs/aiplay.md.
