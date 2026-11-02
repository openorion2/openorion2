"""The tables tools/aitrain writes (docs/aitrain.md), as batches of padded tensors."""

import json
import os

import numpy as np
import torch

# Column layouts, as aitrain.go writes them.
ME_FIELDS = slice(21, 104)  # FieldStatus
ME_LEGAL = slice(104, 111)  # the applications it may research, a bit each
ME_DUE = 111  # it picks an application this turn
ME_RESEARCH_AFTER = (112, 113)
CO_LEGAL = slice(25, 28)  # the queue classes it may put first, a bit each
CO_HEAD_AGE = 28  # turns the queue's head has been built
CO_IN = 29  # colony columns before the decided ones
CO_QUEUE_AFTER = 29
CO_JOBS_AFTER = 36
# Ships: id, status, x, y, star at, destination, may take orders, experience,
# design (9), ETA, turns standing at its star; then the destination decided.
SH_ORDER = 6
SH_IN = 19
SH_DEST_AFTER = 19
QUEUE_VOCAB = 74
MAX_POP = 48
MAX_SHIPS = 48
MAX_COLONIES = 32


def queue_class(v):
    """A queue code as a class: 0 empty, buildings 1..48, specials 49..63 (-2..-15),
    ships 64.. by kind (aitrain.go records -1000 - type*8 - size)."""
    v = np.asarray(v)
    out = np.zeros_like(v)
    out = np.where((v >= 1) & (v <= 48), v, out)
    out = np.where((v <= -2) & (v >= -15), 48 - v, out)
    kind = -1000 - v  # type*8 + size: combat 0..5, colony 8.., transport 16.., outpost 32..
    t, size = kind // 8, kind % 8
    ship = np.where(t == 0, size, np.where(t == 1, 6, np.where(t == 2, 7, np.where(t == 4, 8, 9))))
    out = np.where(v <= -1000, 64 + ship, out)
    return np.clip(out, 0, QUEUE_VOCAB - 1)


def bits(words, n):
    """The low n bits of int32 words, as bools (rows of words in, rows of bits out)."""
    w = np.asarray(words, dtype=np.int64) & 0xFFFFFFFF
    b = (w[..., :, None] >> np.arange(32)) & 1
    return b.reshape(*w.shape[:-1], -1)[..., :n].astype(bool)


def squash(x):
    """Signed log scale for counts and money."""
    x = np.asarray(x, dtype=np.float32)
    return np.sign(x) * np.log1p(np.abs(x))


class Shard:
    """One directory of tables, memory-mapped, with each record's row offsets."""

    def __init__(self, path):
        meta = json.load(open(os.path.join(path, "meta.json")))
        self.w = meta["widths"]

        def load(name):
            w = self.w[name]
            a = np.fromfile(os.path.join(path, name + ".i32"), dtype="<i4")
            return a.reshape(-1, w) if w else a.reshape(0, 1)

        self.t = {n: load(n) for n in self.w}
        games, recs = self.t["games"], self.t["recs"]
        self.n = len(recs)
        gs_off = np.concatenate([[0], np.cumsum(games[:, 6])])
        gp_off = np.concatenate([[0], np.cumsum(games[:, 7])])
        gi = recs[:, 0]
        ns, np_ = games[gi, 6], games[gi, 7]
        self.star_static = gs_off[gi]
        self.planet_static = gp_off[gi]
        self.nstars, self.nplanets = ns, np_
        self.star_dyn = np.concatenate([[0], np.cumsum(ns)])[:-1]
        self.pn_dyn = np.concatenate([[0], np.cumsum(np_)])[:-1]
        self.co = np.concatenate([[0], np.cumsum(recs[:, 3])])[:-1]
        self.sh = np.concatenate([[0], np.cumsum(recs[:, 4])])[:-1]

    def record(self, i):
        t = self.t
        rec = t["recs"][i]
        g = t["games"][rec[0]]
        ns, npl = self.nstars[i], self.nplanets[i]
        a, b = self.star_static[i], self.star_dyn[i]
        stars = np.concatenate([t["stars"][a:a + ns], t["star"][b:b + ns]], axis=1)
        a, b = self.planet_static[i], self.pn_dyn[i]
        planets = np.concatenate([t["planets"][a:a + npl], t["pn"][b:b + npl]], axis=1)
        co = t["co"][self.co[i]:self.co[i] + rec[3]]
        sh = t["sh"][self.sh[i]:self.sh[i] + rec[4]]
        return dict(setup=g[1:6], me=t["me"][i], des=t["des"][i], pl=t["pl"][i],
                    stars=stars, planets=planets, co=co, sh=sh)


def featurize(r, sample=True):
    """Model inputs and targets of one record, as numpy arrays."""
    me, stars, planets, co, sh = r["me"], r["stars"], r["planets"], r["co"], r["sh"]
    # Ships being built decide nothing; of a big navy a sample (attention
    # costs the square of the tokens).
    sh = sh[sh[:, 1] != 6]
    if sample and len(sh) > MAX_SHIPS:
        sh = sh[np.sort(np.random.choice(len(sh), MAX_SHIPS, replace=False))]
    if sample and len(co) > MAX_COLONIES:
        co = co[np.sort(np.random.choice(len(co), MAX_COLONIES, replace=False))]
    # The empire: numbers, its fields, designs, the others, the setup.
    glob_num = np.concatenate([squash(me[:21]), squash(r["des"]), squash(r["pl"]), r["setup"].astype(np.float32)])
    fields = me[ME_FIELDS].astype(np.int64)  # 0..3 each
    race = int(me[1])
    # Stars: static x, y, size, class, special, wormhole, nebula; dynamic 9
    # (visited first). Every star is on the map; its special only once visited.
    visited = stars[:, 7]
    st_num = np.concatenate([stars[:, :2] / 1000.0, squash(stars[:, 7:])], axis=1).astype(np.float32)
    st_cat = np.stack([stars[:, 2], stars[:, 3], np.clip(stars[:, 4], 0, 31) * visited, stars[:, 6]], axis=1)
    # Planets: star, orbit, type, size, gravity, minerals, special; owner
    # (-2 unknown), pop, outpost, climate, max farms, known.
    pn_star = planets[:, 0].astype(np.int64)
    known = planets[:, 12]
    pn_num = squash(planets[:, [8, 11]])
    pn_cat = np.stack([planets[:, 1] * known, planets[:, 2] * known, planets[:, 3] * known, planets[:, 4] * known,
                       planets[:, 5] * known, np.clip(planets[:, 6], 0, 31) * known, planets[:, 7] + 2,
                       planets[:, 9], np.clip(planets[:, 10], 0, 15) * known, known],
                      axis=1).clip(0, 31)
    # Colonies: the planet, numbers, the queue now.
    co_planet = co[:, 1].astype(np.int64)
    co_num = squash(np.concatenate([co[:, 2:16], co[:, CO_HEAD_AGE:CO_HEAD_AGE + 1]], 1))
    co_bits = ((co[:, 23:25, None] >> np.arange(32)) & 1).reshape(len(co), 64).astype(np.float32)
    co_queue = queue_class(co[:, 16:23])
    # Ships: where, numbers, design.
    sh_num = np.concatenate([sh[:, 2:4] / 1000.0, squash(sh[:, 7:17]), squash(sh[:, 17:19])], axis=1).astype(np.float32)
    sh_cat = np.stack([sh[:, 1], sh[:, SH_ORDER], sh[:, 8], sh[:, 9]], axis=1).clip(0, 31)
    sh_at = sh[:, 4].astype(np.int64)
    sh_dest = sh[:, 5].astype(np.int64)
    # Targets.
    rf, ra = me[ME_RESEARCH_AFTER[0]], me[ME_RESEARCH_AFTER[1]]
    research = int(ra) if (rf != me[7] or ra != me[8]) else -1
    y_queue = queue_class(co[:, CO_QUEUE_AFTER])
    # What it may choose; the AI's choice and the head it keeps count as legal.
    research_legal = bits(me[ME_LEGAL], 212)
    if research >= 0:
        research_legal[research] = True
    co_legal = bits(co[:, CO_LEGAL], QUEUE_VOCAB) if len(co) else np.zeros((0, QUEUE_VOCAB), bool)
    co_legal[:, 0] = False
    co_legal[np.arange(len(co)), co_queue[:, 0]] = True
    co_legal[np.arange(len(co)), y_queue] = True
    y_jobs = np.clip(co[:, CO_JOBS_AFTER:CO_JOBS_AFTER + 2], 0, MAX_POP - 1)
    # A course kept (in flight) is "stay": the same class as staying at a star.
    y_dest = sh[:, SH_DEST_AFTER].astype(np.int64)
    y_dest = np.where((y_dest == sh[:, 5]) & (sh[:, 5] >= 0), -1, y_dest)
    ship_free = sh[:, SH_ORDER] == 1  # it may take orders: it decides
    return dict(glob_num=glob_num, fields=fields, race=race, st_num=st_num, st_cat=st_cat,
                pn_star=pn_star, pn_num=pn_num, pn_cat=pn_cat,
                co_planet=co_planet, co_num=co_num, co_bits=co_bits, co_queue=co_queue, co_pop=co[:, 2],
                sh_num=sh_num, sh_cat=sh_cat, sh_at=sh_at, sh_dest=sh_dest,
                research_legal=research_legal, co_legal=co_legal,
                y_research=research, y_queue=y_queue, y_jobs=y_jobs, y_dest=y_dest, ship_free=ship_free, sh_rows=sh)


def pad(arrs, fill=0):
    n = -(-max(1, max(len(a) for a in arrs)) // 16) * 16  # few shapes: MPS compiles each
    shape = (len(arrs), n) + arrs[0].shape[1:]
    out = np.full(shape, fill, dtype=arrs[0].dtype)
    mask = np.zeros((len(arrs), n), dtype=bool)
    for i, a in enumerate(arrs):
        out[i, :len(a)] = a
        mask[i, :len(a)] = True
    return torch.from_numpy(out), torch.from_numpy(mask)


def collate(feats):
    b = {}
    b["glob_num"] = torch.from_numpy(np.stack([f["glob_num"] for f in feats]))
    b["fields"] = torch.from_numpy(np.stack([f["fields"] for f in feats]))
    b["race"] = torch.tensor([f["race"] for f in feats])
    b["y_research"] = torch.tensor([f["y_research"] for f in feats])
    b["research_legal"] = torch.from_numpy(np.stack([f["research_legal"] for f in feats]))
    for k, m in [("st_num", "st_mask"), ("pn_num", "pn_mask"), ("co_num", "co_mask"), ("sh_num", "sh_mask")]:
        b[k], b[m] = pad([f[k] for f in feats])
    for k, fill in [("st_cat", 0), ("pn_star", 0), ("pn_cat", 0), ("co_planet", 0), ("co_bits", 0), ("co_queue", 0), ("co_legal", False),
                    ("co_pop", 0), ("sh_cat", 0), ("sh_at", -1), ("sh_dest", -1), ("y_queue", -100),
                    ("y_jobs", -100), ("y_dest", -2), ("ship_free", False)]:
        b[k], _ = pad([np.asarray(f[k]) for f in feats], fill)
    if "co_free" in feats[0]:
        b["co_free"], _ = pad([np.asarray(f["co_free"], dtype=np.float32) for f in feats])
    for k, v in b.items():
        if v.dtype == torch.int32:
            b[k] = v.long()
    return b


class LengthBatches(torch.utils.data.Sampler):
    """Shuffled batches of records of about the same number of tokens."""

    def __init__(self, ds, batch, pool=100):
        self.batch, self.pool = batch, pool
        self.size = np.array([min(sh.t["recs"][i, 4], MAX_SHIPS) + min(sh.t["recs"][i, 3], MAX_COLONIES) + sh.nstars[i]
                              for sh in ds.shards for i in range(sh.n)])

    def __iter__(self):
        idx = np.random.permutation(len(self.size))
        out = []
        for k in range(0, len(idx), self.batch * self.pool):
            chunk = idx[k:k + self.batch * self.pool]
            chunk = chunk[np.argsort(self.size[chunk], kind="stable")]
            out += [chunk[j:j + self.batch].tolist() for j in range(0, len(chunk) - self.batch + 1, self.batch)]
        np.random.shuffle(out)
        return iter(out)

    def __len__(self):
        return len(self.size) // self.batch


class Dataset(torch.utils.data.Dataset):
    def __init__(self, paths):
        self.shards = [Shard(p) for p in paths]
        self.index = [(s, i) for s, sh in enumerate(self.shards) for i in range(sh.n)]

    def __len__(self):
        return len(self.index)

    def __getitem__(self, k):
        s, i = self.index[k]
        return featurize(self.shards[s].record(i))
