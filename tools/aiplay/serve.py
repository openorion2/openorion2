"""Plays an empire: a record per line on stdin, the decisions per line on
stdout (internal/aiplay, docs/aiplay.md).

    python3 serve.py models/rl-survival.pt [heads] [stay] [sample]
"""

import json
import sys

import numpy as np
import torch

import data
from model import load


def class_code(c):
    """queue_class's inverse: the queue code (docs/control.md)."""
    if 1 <= c <= 48:
        return c
    if 50 <= c <= 63:
        return 48 - c
    if 64 <= c <= 69:
        return -1000 - (c - 64)
    return {70: -1008, 71: -1016, 72: -1032}.get(c)


STAY_MAX = float(sys.argv[3]) if len(sys.argv) > 3 else 0.5
SAMPLE = len(sys.argv) > 4 and sys.argv[4] == "sample"  # draw from the distributions, as RL plays


def ranked(logits, k):
    """The candidates best first, or with SAMPLE a draw first, then the rest."""
    top = logits.topk(k).indices.tolist()
    if not SAMPLE:
        return top
    d = int(torch.distributions.Categorical(logits=logits).sample())
    return [d] + [c for c in top if c != d]


def main():
    dev = torch.device("cpu")  # one record at a time: the CPU is faster than MPS
    model, _ = load(sys.argv[1], dev)
    model.eval()
    torch.set_num_threads(2)
    for line in sys.stdin:
        q = json.loads(line)
        if "done" in q:  # a game ended (for rl.py); no answer
            continue
        widths = {"co": 39, "sh": 20}
        rec = {}
        for k, v in q.items():
            a = np.asarray(v if v is not None else [], dtype=np.int64)
            if k in ("stars", "planets", "co", "sh"):
                a = a.reshape(-1, widths[k]) if a.size == 0 else a.reshape(len(a), -1)
            rec[k] = a
        f = data.featurize(rec, sample=False)
        b = data.collate([f])
        with torch.no_grad():
            out = model(b)
        ans = {"rs": ranked(out["research"][0], 12), "cq": {}, "cj": {}, "sd": {}}
        co, sh = rec["co"], f["sh_rows"]
        for i in range(len(co)):
            ci = int(co[i, 0])
            if co[i, 12]:  # an outpost
                continue
            codes = [class_code(c) for c in ranked(out["queue"][0, i], 6)]
            ans["cq"][ci] = [c for c in codes if c is not None]
            pop = int(co[i, 2])
            jobs = out["jobs"][0, i].argmax(-1).tolist()
            farm = min(jobs[0], pop)
            ans["cj"][ci] = [farm, min(jobs[1], pop - farm)]
        n_st = out["dest"].shape[-1] - 1
        prob = out["dest"][0].softmax(-1)
        for j in range(len(sh)):
            if sh[j, data.SH_ORDER] != 1:
                continue
            at = int(sh[j, 4])
            p = prob[j].clone()
            stay = float(p[n_st]) + (float(p[at]) if 0 <= at < n_st else 0.0)
            # Go when staying is less likely than STAY_MAX, to the likeliest star.
            p[n_st] = 0
            if 0 <= at < n_st:
                p[at] = 0
            t = int(p.argmax())
            ans["sd"][int(sh[j, 0])] = t if stay < STAY_MAX else -1
        heads = sys.argv[2] if len(sys.argv) > 2 else "rqjd"  # research, queue, jobs, destinations
        for h, k in (("r", "rs"), ("q", "cq"), ("j", "cj"), ("d", "sd")):
            if h not in heads:
                ans[k] = [] if k == "rs" else {}
        sys.stdout.write(json.dumps(ans) + "\n")
        sys.stdout.flush()


if __name__ == "__main__":
    main()
