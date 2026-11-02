"""Reinforcement learning (PPO) of the network against the original AIs.

A TCP server: tools/aitrain processes (-policy tcp://host:port) play games
and ask it each turn for empire -control's decisions; it answers a batch at
a time on the GPU, keeps each game's turns, and after every --steps turns
updates the network on them. The network decides the research and the
queues (the rest stays the AI's), sampling from its masked distributions.

Reward per turn: the growth of the population and colonies; at the end of
a game the rank (+1 first, down to -1 last, -1 when eliminated).

    python rl.py --init policy-fog.pt --port 7700 -o policy-rl.pt
"""

import argparse
import json
import queue
import socket
import socketserver
import threading
import time

import numpy as np
import torch
import torch.nn.functional as F

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "aiplay"))  # model.py, data.py

import data  # noqa: E402
from model import load


def class_code(c):
    if 1 <= c <= 48:
        return c
    if 50 <= c <= 63:
        return 48 - c
    if 64 <= c <= 69:
        return -1000 - (c - 64)
    return {70: -1008, 71: -1016, 72: -1032}.get(c)


def parse(q):
    widths = {"co": 39, "sh": 20}
    rec = {}
    for k, v in q.items():
        a = np.asarray(v if v is not None else [], dtype=np.int64)
        if k in ("stars", "planets", "co", "sh"):
            a = a.reshape(-1, widths[k]) if a.size == 0 else a.reshape(len(a), -1)
        rec[k] = a
    return rec


def jobs_logits(jobs, pop, farmers=None):
    """Farmers' logits (at most the population) and, given the farmers,
    workers' (at most the rest)."""
    ar = torch.arange(data.MAX_POP, device=jobs.device)
    lf = jobs[..., 0, :].masked_fill(ar > pop[..., None], -1e4)
    if farmers is None:
        return lf
    lw = jobs[..., 1, :].masked_fill(ar > (pop - farmers)[..., None], -1e4)
    return lf, lw


def masked_mean(x, mask):
    return (x * mask).sum() / mask.sum().clamp(min=1)


class Step:
    """One turn of one game: what the network saw and chose."""

    __slots__ = ("feat", "research", "queue", "jobs", "dest", "logp", "value", "reward", "done", "pop", "cols", "fields")


class Game:
    def __init__(self):
        self.steps = []  # Steps of the game so far, the last one waiting for its reward


class Learner:
    def __init__(self, a):
        self.a = a
        self.dev = torch.device("cuda" if torch.cuda.is_available() else "mps" if torch.backends.mps.is_available() else "cpu")
        self.model, missing = load(a.init, self.dev)
        print("new parameters:", missing, flush=True)
        self.opt = torch.optim.Adam(self.model.parameters(), lr=a.lr)
        self.lock = threading.Lock()  # the model, between serving and updating
        self.requests = queue.Queue()
        self.ready = []  # Steps with their reward known
        self.games_done, self.results = 0, []
        self.iteration = 0
        threading.Thread(target=self.batcher, daemon=True).start()

    # Serving.

    def ask(self, feat):
        ev = threading.Event()
        box = {}
        self.requests.put((feat, ev, box))
        ev.wait()
        return box["out"]

    def batcher(self):
        while True:
            items = [self.requests.get()]
            deadline = time.time() + 0.005
            while len(items) < 256:
                try:
                    items.append(self.requests.get(timeout=max(0, deadline - time.time())))
                except queue.Empty:
                    break
            with self.lock, torch.no_grad():
                self.model.eval()
                b = data.collate([f for f, _, _ in items])
                b = {k: v.to(self.dev) for k, v in b.items()}
                out = self.model(b)
                rs = torch.distributions.Categorical(logits=out["research"]).sample()
                qs = torch.distributions.Categorical(logits=out["queue"]).sample()
                lp_r = F.log_softmax(out["research"], -1).gather(1, rs[:, None])[:, 0]
                lp_q = F.log_softmax(out["queue"], -1).gather(2, qs[..., None])[..., 0]
                lp_j = torch.zeros_like(lp_q)
                fw = None
                if "j" in self.a.heads:
                    pop = b["co_pop"].clamp(0, data.MAX_POP - 1)
                    lf = jobs_logits(out["jobs"], pop)
                    fs = torch.distributions.Categorical(logits=lf).sample()
                    _, lw = jobs_logits(out["jobs"], pop, fs)
                    ws = torch.distributions.Categorical(logits=lw).sample()
                    lp_j = (F.log_softmax(lf, -1).gather(2, fs[..., None])[..., 0] +
                            F.log_softmax(lw, -1).gather(2, ws[..., None])[..., 0])
                    fw = torch.stack([fs, ws], -1)
                ds, lp_d = None, None
                if "d" in self.a.heads:
                    ds = torch.distributions.Categorical(logits=out["dest"]).sample()
                    lp_d = F.log_softmax(out["dest"], -1).gather(2, ds[..., None])[..., 0]
                for i, (f, ev, box) in enumerate(items):
                    n, nsh = len(f["co_num"]), len(f["sh_num"])
                    due = f["due"]
                    q = qs[i, :n].cpu().numpy()
                    # Each decision's log-probability: PPO clips each one.
                    lp = {"r": float(lp_r[i]), "q": lp_q[i, :n].cpu().numpy(),
                          "j": lp_j[i, :n].cpu().numpy() if fw is not None else None,
                          "d": lp_d[i, :nsh].cpu().numpy() if ds is not None else None}
                    jobs = fw[i, :n].cpu().numpy() if fw is not None else None
                    dest = ds[i, :nsh].cpu().numpy() if ds is not None else None
                    box["out"] = (int(rs[i]) if due else -1, q, jobs, dest, lp, float(out["value"][i]))
                    ev.set()

    # Learning.

    def finish(self, g, reward_end):
        """The game ended: its last step gets the final reward."""
        if g.steps:
            s = g.steps[-1]
            s.reward += reward_end
            s.done = True
            self.add(g.steps)
        g.steps = []

    def add(self, steps):
        with self.lock:
            self.ready.append(steps)
            n = sum(len(x) for x in self.ready)
        if n >= self.a.steps:
            self.update()

    def update(self):
        with self.lock:
            episodes, self.ready = self.ready, []
            a, m = self.a, self.model
            # Advantages (GAE) per piece of a game; a piece cut before its end
            # bootstraps from its last value.
            feats, rs, qs, js, dss, lps, advs, rets = [], [], [], [], [], [], [], []
            for ep in episodes:
                last_v = 0.0 if ep[-1].done else ep[-1].value
                gae = 0.0
                body = ep if ep[-1].done else ep[:-1]
                for t in reversed(range(len(body))):
                    s = body[t]
                    nv = last_v if t == len(body) - 1 else body[t + 1].value
                    delta = s.reward + a.gamma * nv * (0.0 if s.done else 1.0) - s.value
                    gae = delta + a.gamma * a.lam * (0.0 if s.done else 1.0) * gae
                    advs.append(gae)
                    rets.append(gae + s.value)
                    feats.append(s.feat)
                    rs.append(s.research)
                    qs.append(s.queue)
                    js.append(s.jobs)
                    dss.append(s.dest)
                    lps.append(s.logp)
            if not feats:
                return
            adv = np.array(advs, dtype=np.float32)
            adv = (adv - adv.mean()) / (adv.std() + 1e-6)
            ret = np.array(rets, dtype=np.float32)
            m.train()
            n = len(feats)
            stats = []
            for _ in range(a.epochs):
                perm = np.random.permutation(n)
                for k in range(0, n, a.batch):
                    idx = perm[k:k + a.batch]
                    b = data.collate([feats[i] for i in idx])
                    b = {kk: v.to(self.dev) for kk, v in b.items()}
                    out = m(b)
                    A = torch.from_numpy(adv[idx]).to(self.dev)
                    # Per decision: (new log-prob, old log-prob, mask), [steps, decisions].
                    dec = []
                    r = torch.tensor([rs[i] for i in idx], device=self.dev)
                    due = (r >= 0).float()
                    lr_ = F.log_softmax(out["research"], -1).gather(1, r.clamp(min=0)[:, None])
                    old_r = torch.tensor([lps[i]["r"] for i in idx], device=self.dev)[:, None]
                    dec.append((lr_, old_r, due[:, None]))
                    free = b["co_free"]
                    q, _ = data.pad([qs[i].astype(np.int64) for i in idx])
                    q = q.to(self.dev).clamp(max=out["queue"].shape[-1] - 1)
                    lq = F.log_softmax(out["queue"], -1).gather(2, q[..., None])[..., 0]
                    old_q, _ = data.pad([lps[i]["q"].astype(np.float32) for i in idx])
                    dec.append((lq, old_q.to(self.dev), free))
                    ent_j = ent_d = torch.zeros((), device=self.dev)
                    if "j" in a.heads:
                        fw, _ = data.pad([js[i].astype(np.int64) for i in idx])
                        fw = fw.to(self.dev)
                        pop = b["co_pop"].clamp(0, data.MAX_POP - 1)
                        lf, lw = jobs_logits(out["jobs"], pop, fw[..., 0])
                        lj = (F.log_softmax(lf, -1).gather(2, fw[..., 0:1])[..., 0] +
                              F.log_softmax(lw, -1).gather(2, fw[..., 1:2])[..., 0])
                        old_j, _ = data.pad([lps[i]["j"].astype(np.float32) for i in idx])
                        dec.append((lj, old_j.to(self.dev), free))
                        ent_j = masked_mean(torch.distributions.Categorical(logits=lf).entropy(), free)
                    if "d" in a.heads:
                        dd, _ = data.pad([dss[i].astype(np.int64) for i in idx])
                        dd = dd.to(self.dev).clamp(max=out["dest"].shape[-1] - 1)
                        sf = b["ship_free"].float()
                        ld = F.log_softmax(out["dest"], -1).gather(2, dd[..., None])[..., 0]
                        old_d, _ = data.pad([lps[i]["d"].astype(np.float32) for i in idx])
                        dec.append((ld, old_d.to(self.dev), sf))
                        ent_d = masked_mean(torch.distributions.Categorical(logits=out["dest"]).entropy(), sf)
                    if a.ppo == "per":
                        # Each decision its own ratio and clip, a turn their mean.
                        num = torch.zeros_like(A)
                        cnt = torch.zeros_like(A)
                        clipped = []
                        for new_lp, old_lp, mk in dec:
                            ratio = (new_lp - old_lp).clamp(-20, 20).exp()
                            surr = torch.min(ratio * A[:, None], ratio.clamp(1 - a.clip, 1 + a.clip) * A[:, None])
                            num = num + (surr * mk).sum(-1)
                            cnt = cnt + mk.sum(-1)
                            clipped.append((((ratio - 1).abs() > a.clip).float() * mk).sum() / mk.sum().clamp(min=1))
                        has = cnt > 0
                        pl = -(num[has] / cnt[has]).mean() if has.any() else torch.zeros((), device=self.dev)
                    else:
                        # The turn's decisions as one action: the ratio of their
                        # product (what worked: run D beat the AI).
                        lp_new = sum((nl * mk).sum(-1) for nl, _, mk in dec)
                        lp_old = sum((ol * mk).sum(-1) for _, ol, mk in dec)
                        ratio = (lp_new - lp_old).clamp(-20, 20).exp()
                        pl = -torch.min(ratio * A, ratio.clamp(1 - a.clip, 1 + a.clip) * A).mean()
                        clipped = [((ratio - 1).abs() > a.clip).float().mean()]
                    vl = F.mse_loss(out["value"], torch.from_numpy(ret[idx]).to(self.dev))
                    ent_r = masked_mean(torch.distributions.Categorical(logits=out["research"]).entropy()[:, None], due[:, None])
                    ent_q = masked_mean(torch.distributions.Categorical(logits=out["queue"]).entropy(), free)
                    if a.ppo != "per":  # summed over a turn's decisions, as run D had it
                        ent_q = (torch.distributions.Categorical(logits=out["queue"]).entropy() * free).sum(-1).mean()
                        if "j" in a.heads:
                            ent_j = (torch.distributions.Categorical(logits=lf).entropy() * free).sum(-1).mean()
                        if "d" in a.heads:
                            ent_d = (torch.distributions.Categorical(logits=out["dest"]).entropy() * sf).sum(-1).mean()
                    loss = pl + a.vf * vl - a.ent * (ent_r + ent_q + ent_j + ent_d)
                    self.opt.zero_grad()
                    loss.backward()
                    torch.nn.utils.clip_grad_norm_(m.parameters(), 0.5)
                    self.opt.step()
                    stats.append((pl.item(), vl.item(), (ent_q + ent_j + ent_d).item(), float(torch.stack(clipped).mean())))
            self.iteration += 1
            st = np.mean(stats, 0)
            res = self.results[-200:]
            rank = np.mean([(x["Rank"] - 1) / max(1, x["Players"] - 1) for x in res]) if res else float("nan")
            elim = np.mean([x["Eliminated"] for x in res]) if res else float("nan")
            print(f"iter {self.iteration} steps {n} games {self.games_done} last200: rank {rank:.3f} eliminated {elim:.2f} "
                  f"| pi {st[0]:.3f} v {st[1]:.3f} entropy {st[2]:.2f} clipped {st[3]:.2f} | ret {ret.mean():.3f}", flush=True)
            torch.save({"model": m.state_dict(), "iteration": self.iteration, "config": m.config}, a.o)


def make_handler(L):
    class Handler(socketserver.StreamRequestHandler):
        def handle(self):
            g = Game()
            for line in self.rfile:
                q = json.loads(line)
                if "done" in q:
                    r = q["done"]
                    L.games_done += 1
                    L.results.append(r)
                    end = -1.0 if r["Eliminated"] else 1.0 - 2.0 * (r["Rank"] - 1) / max(1, r["Players"] - 1)
                    end *= L.a.r_end
                    L.finish(g, end)
                    continue
                rec = parse(q)
                f = data.featurize(rec, sample=False)
                f["due"] = int(rec["me"][data.ME_DUE])
                f["co_free"] = (rec["co"][:, 12] == 0).astype(np.float32) if len(rec["co"]) else np.zeros(0, np.float32)
                pop = int(rec["co"][:, 2].sum()) if len(rec["co"]) else 0
                cols = len(rec["co"])
                fields = int((rec["me"][data.ME_FIELDS] == 3).sum())
                # The previous turn's reward: the growth since.
                if g.steps:
                    p = g.steps[-1]
                    p.reward += L.a.r_pop * (pop - p.pop) + L.a.r_col * (cols - p.cols) + L.a.r_fields * (fields - p.fields)
                    if len(g.steps) >= L.a.piece:
                        # A piece ends at p: p's value bootstraps it, and p is
                        # learned with the next piece.
                        L.add(list(g.steps))
                        g.steps = [p]
                research, qcls, jobs, dest, logp, value = L.ask(f)
                s = Step()
                s.feat, s.research, s.queue, s.jobs, s.dest, s.logp, s.value = f, research, qcls, jobs, dest, logp, value
                s.reward, s.done, s.pop, s.cols, s.fields = 0.0, False, pop, cols, fields
                g.steps.append(s)
                ans = {"rs": [research] if research >= 0 else [], "cq": {}, "cj": {}, "sd": {}}
                co = rec["co"]
                for i in range(len(co)):
                    if co[i, 12]:
                        continue
                    c = class_code(int(qcls[i]))
                    if c is not None:
                        ans["cq"][int(co[i, 0])] = [c]
                    if jobs is not None:
                        ans["cj"][int(co[i, 0])] = [int(jobs[i, 0]), int(jobs[i, 1])]
                if dest is not None:
                    sh, n_st = f["sh_rows"], len(f["st_num"])
                    for j in range(len(sh)):
                        if sh[j, data.SH_ORDER] == 1:
                            t = int(dest[j])
                            ans["sd"][int(sh[j, 0])] = -1 if t >= n_st else t
                self.wfile.write((json.dumps(ans) + "\n").encode())
                self.wfile.flush()

    return Handler


class Server(socketserver.ThreadingMixIn, socketserver.TCPServer):
    daemon_threads = True
    allow_reuse_address = True


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--init", required=True)
    ap.add_argument("--port", type=int, default=7700)
    ap.add_argument("-o", default="policy-rl.pt")
    ap.add_argument("--steps", type=int, default=16384, help="turns per update")
    ap.add_argument("--piece", type=int, default=64, help="a game's turns are learned in pieces of this many")
    ap.add_argument("--epochs", type=int, default=3)
    ap.add_argument("--batch", type=int, default=512)
    ap.add_argument("--lr", type=float, default=2e-5)
    ap.add_argument("--gamma", type=float, default=0.995)
    ap.add_argument("--lam", type=float, default=0.95)
    ap.add_argument("--clip", type=float, default=0.2)
    ap.add_argument("--vf", type=float, default=0.5)
    ap.add_argument("--ent", type=float, default=0.0005)
    ap.add_argument("--ppo", default="sum", choices=["sum", "per"], help="clip a turn's decisions together, or each")
    ap.add_argument("--r-pop", type=float, default=0.01)
    ap.add_argument("--r-col", type=float, default=0.05)
    ap.add_argument("--r-end", type=float, default=1.0, help="the end of game reward's weight")
    ap.add_argument("--r-fields", type=float, default=0.0, help="per technology field researched")
    ap.add_argument("--heads", default="rq", help="what the network decides: r research, q queues, j jobs, d ship destinations")
    a = ap.parse_args()
    L = Learner(a)
    srv = Server(("127.0.0.1", a.port), make_handler(L))
    print(f"listening on {a.port}, device {L.dev}", flush=True)
    srv.serve_forever()


if __name__ == "__main__":
    main()
