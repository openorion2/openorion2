"""Behaviour cloning of the original AI from tools/aitrain's tables.

    build/venv/bin/python tools/aitrain/train.py build/aidata/s* -o build/policy.pt
"""

import argparse
import math
import time

import torch

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "aiplay"))  # model.py, data.py

import data  # noqa: E402
from model import Policy, load, losses


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("shards", nargs="+")
    ap.add_argument("-o", default="build/policy.pt")
    ap.add_argument("--batch", type=int, default=64)
    ap.add_argument("--steps", type=int, default=20000)
    ap.add_argument("--lr", type=float, default=3e-4)
    ap.add_argument("--val", type=int, default=1, help="shards held out for validation (the last ones)")
    ap.add_argument("--workers", type=int, default=6)
    ap.add_argument("--init", help="a checkpoint to start from")
    ap.add_argument("--d", type=int, default=192)
    ap.add_argument("--layers", type=int, default=4)
    ap.add_argument("--heads", type=int, default=6)
    ap.add_argument("--planets", action="store_true", help="planets as tokens")
    ap.add_argument("--dist", action="store_true", help="attention biased by map distance")
    a = ap.parse_args()
    dev = torch.device("cuda" if torch.cuda.is_available() else "mps" if torch.backends.mps.is_available() else "cpu")
    train = data.Dataset(a.shards[:-a.val] if a.val else a.shards)
    val = data.Dataset(a.shards[-a.val:]) if a.val else None
    print(f"train {len(train)} records, val {len(val) if val else 0}", flush=True)
    dl = torch.utils.data.DataLoader(train, batch_sampler=data.LengthBatches(train, a.batch), num_workers=a.workers,
                                     collate_fn=data.collate, persistent_workers=True)
    if a.init:
        model, missing = load(a.init, dev)
        print("new parameters:", missing, flush=True)
    else:
        model = Policy(d=a.d, layers=a.layers, heads=a.heads, planets=a.planets, dist=a.dist).to(dev)
    print(f"{sum(p.numel() for p in model.parameters()) / 1e6:.1f}M parameters", flush=True)
    opt = torch.optim.AdamW(model.parameters(), lr=a.lr, weight_decay=0.01)
    sched = torch.optim.lr_scheduler.LambdaLR(opt, lambda s: min(1, s / 500) * 0.5 * (1 + math.cos(math.pi * min(s, a.steps) / a.steps)))
    step, t0 = 0, time.time()
    run = {}
    while step < a.steps:
        for b in dl:
            b = {k: v.to(dev) for k, v in b.items()}
            out = model(b)
            L, acc = losses(out, b)
            loss = sum(L.values())
            opt.zero_grad()
            loss.backward()
            torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0)
            opt.step()
            sched.step()
            for k, v in list(L.items()) + [("acc_" + k, v) for k, v in acc.items()]:
                run[k] = 0.98 * run.get(k, v.item()) + 0.02 * v.item()
            step += 1
            if step % 200 == 0:
                print(f"step {step} {time.time() - t0:.0f}s " + " ".join(f"{k} {v:.3f}" for k, v in sorted(run.items())), flush=True)
            if step % 2000 == 0 or step == a.steps:
                torch.save({"model": model.state_dict(), "step": step, "config": model.config}, a.o)
                if val:
                    evaluate(model, val, dev, a)
            if step >= a.steps:
                break


@torch.no_grad()
def evaluate(model, val, dev, a, n=4000):
    model.eval()
    g = torch.Generator().manual_seed(0)
    idx = torch.randperm(len(val), generator=g)[:n].tolist()
    dl = torch.utils.data.DataLoader(torch.utils.data.Subset(val, idx), batch_size=a.batch, num_workers=a.workers,
                                     collate_fn=data.collate)
    tot = {}
    for b in dl:
        b = {k: v.to(dev) for k, v in b.items()}
        L, acc = losses(model(b), b)
        for k, v in list(L.items()) + [("acc_" + k, v) for k, v in acc.items()]:
            tot.setdefault(k, []).append(v.item())
    print("VAL " + " ".join(f"{k} {sum(v) / len(v):.3f}" for k, v in sorted(tot.items())), flush=True)
    model.train()


if __name__ == "__main__":
    main()
