"""The policy network: a transformer over the empire, stars, planets, colonies and ships."""

import torch
import torch.nn as nn
import torch.nn.functional as F

from data import MAX_POP, QUEUE_VOCAB

N_APPS = 212
N_FIELDS = 83


class CatEmbed(nn.Module):
    """A sum of embeddings, one table per categorical column (values 0..31)."""

    def __init__(self, cols, d):
        super().__init__()
        self.tab = nn.Embedding(32 * cols, d)
        self.register_buffer("off", torch.arange(cols) * 32, persistent=False)

    def forward(self, x):
        return self.tab(x.clamp(0, 31) + self.off).sum(-2)


class Policy(nn.Module):
    def __init__(self, d=192, layers=4, heads=6, planets=False, dist=False, glob_in=160, st_num=11, pn_num=2, co_num=15, sh_num=14):
        super().__init__()
        self.config = dict(d=d, layers=layers, heads=heads, planets=planets, dist=dist)
        self.d, self.nheads, self.planets, self.dist = d, heads, planets, dist
        self.glob = nn.Sequential(nn.Linear(glob_in, d), nn.GELU(), nn.Linear(d, d))
        self.field_emb = nn.Embedding(4, 16)
        self.fields = nn.Linear(N_FIELDS * 16, d)
        self.race = nn.Embedding(16, d)
        self.st = nn.Linear(st_num, d)
        self.st_cat = CatEmbed(4, d)
        self.pn = nn.Linear(pn_num, d)
        self.pn_cat = CatEmbed(10, d)
        self.co = nn.Linear(co_num + 64, d)
        self.co_queue = nn.Embedding(QUEUE_VOCAB, d)
        self.sh = nn.Linear(sh_num, d)
        self.sh_cat = CatEmbed(4, d)
        self.type_emb = nn.Embedding(5, d)  # empire, star, colony, ship, planet
        if dist:
            # Attention bias per head from the map distance of two tokens
            # (stars, planets at their star, colonies at theirs, ships).
            self.dist_a = nn.Parameter(torch.zeros(heads))
            self.dist_c = nn.Parameter(torch.zeros(heads))
        layer = nn.TransformerEncoderLayer(d, heads, 4 * d, dropout=0.0, batch_first=True, norm_first=True)
        self.enc = nn.TransformerEncoder(layer, layers)
        self.out_norm = nn.LayerNorm(d)
        # Heads.
        self.research = nn.Linear(d, N_APPS)
        self.value = nn.Linear(d, 1)  # RL: the expected return
        self.queue = nn.Linear(d, QUEUE_VOCAB)
        self.jobs = nn.Linear(d, 2 * MAX_POP)
        self.dest_q = nn.Linear(d, d)
        self.dest_k = nn.Linear(d, d)
        self.stay = nn.Linear(d, 1)
        # Ship to star: distance, the empire's fuel range, its own star.
        self.pair = nn.Sequential(nn.Linear(5, 32), nn.GELU(), nn.Linear(32, 1))
        nn.init.zeros_(self.pair[2].weight)
        nn.init.zeros_(self.pair[2].bias)

    @staticmethod
    def gather(tokens, idx):
        """tokens[b, idx[b, j]] (idx clamped to the valid range)."""
        idx = idx.clamp(0, tokens.shape[1] - 1)
        return torch.gather(tokens, 1, idx.unsqueeze(-1).expand(-1, -1, tokens.shape[-1]))

    def forward(self, b):
        B = b["glob_num"].shape[0]
        g = self.glob(b["glob_num"]) + self.fields(self.field_emb(b["fields"]).flatten(1)) + self.race(b["race"].clamp(0, 15))
        st = self.st(b["st_num"]) + self.st_cat(b["st_cat"])
        # Planets are not tokens: each adds to its star's, and a colony gets its own.
        pn = self.pn(b["pn_num"]) + self.pn_cat(b["pn_cat"])
        pn = pn * b["pn_mask"].unsqueeze(-1)
        st = st + torch.zeros_like(st).scatter_add(1, b["pn_star"].clamp(0, st.shape[1] - 1).unsqueeze(-1).expand_as(pn), pn)
        co = self.co(torch.cat([b["co_num"], b["co_bits"]], -1)) + self.co_queue(b["co_queue"]).mean(-2)
        co = co + self.gather(pn, b["co_planet"])
        sh = self.sh(b["sh_num"]) + self.sh_cat(b["sh_cat"])
        at = b["sh_at"]
        sh = sh + torch.where((at >= 0).unsqueeze(-1), self.gather(st, at), torch.zeros_like(sh))
        dev = g.device
        parts = [g.unsqueeze(1), st, co, sh]
        masks = [torch.ones(B, 1, dtype=torch.bool, device=dev), b["st_mask"], b["co_mask"], b["sh_mask"]]
        types = [0, 1, 2, 3]
        if self.planets:
            parts.append(pn + self.gather(st, b["pn_star"]))
            masks.append(b["pn_mask"])
            types.append(4)
        x = torch.cat([p + self.type_emb.weight[t] for t, p in zip(types, parts)], 1)
        mask = torch.cat(masks, 1)
        if self.dist:
            sxy = b["st_num"][..., :2]
            pxy = self.gather(sxy, b["pn_star"])
            cxy = self.gather(pxy, b["co_planet"])
            pos = [torch.zeros(B, 1, 2, device=dev), sxy, cxy, b["sh_num"][..., :2]] + ([pxy] if self.planets else [])
            has = [torch.zeros(B, 1, dtype=torch.bool, device=dev), b["st_mask"], b["co_mask"], b["sh_mask"]] + \
                ([b["pn_mask"]] if self.planets else [])
            pos, has = torch.cat(pos, 1), torch.cat(has, 1)
            dd = torch.cdist(pos, pos)  # in 1000s of map units
            both = (has[:, :, None] & has[:, None, :]).float()
            f1, f2 = dd * both, torch.log1p(10 * dd) * both
            bias = self.dist_a.view(1, -1, 1, 1) * f1[:, None] + self.dist_c.view(1, -1, 1, 1) * f2[:, None]
            bias = bias.masked_fill(~mask[:, None, None, :], -1e4)
            x = self.out_norm(self.enc(x, mask=bias.reshape(B * self.nheads, *bias.shape[2:])))
        else:
            x = self.out_norm(self.enc(x, src_key_padding_mask=~mask))
        n_st, n_co, n_sh = st.shape[1], co.shape[1], sh.shape[1]
        g_out = x[:, 0]
        st_out = x[:, 1:1 + n_st]
        co_out = x[:, 1 + n_st:1 + n_st + n_co]
        sh_out = x[:, 1 + n_st + n_co:1 + n_st + n_co + n_sh]
        # Ship destinations: a star, or stay (the last column).
        q, k = self.dest_q(sh_out), self.dest_k(st_out)
        dest = torch.einsum("bsd,btd->bst", q, k) / self.d ** 0.5
        sxy, txy = b["sh_num"][..., :2], b["st_num"][..., :2]
        dist = (sxy.unsqueeze(2) - txy.unsqueeze(1)).norm(dim=-1)  # in 1000s of map units
        fuel = b["glob_num"][:, 19].view(B, 1, 1).expand_as(dist)  # squashed FuelRange
        n_st_ = st_out.shape[1]
        here = (b["sh_at"].unsqueeze(-1) == torch.arange(n_st_, device=dist.device)).float()
        pf = torch.stack([dist, dist * dist, fuel, dist * fuel, here], -1)
        dest = dest + self.pair(pf).squeeze(-1)
        dest = dest.masked_fill(~b["st_mask"].unsqueeze(1), -1e4)
        dest = torch.cat([dest, self.stay(sh_out)], -1)
        research = self.research(g_out).masked_fill(~b["research_legal"], -1e4)
        queue = self.queue(co_out).masked_fill(~b["co_legal"], -1e4)
        return dict(research=research, queue=queue, value=self.value(g_out).squeeze(-1), jobs=self.jobs(co_out).view(B, n_co, 2, MAX_POP), dest=dest)


def losses(out, b):
    """The heads' cross entropies and accuracies, on the decisions there are."""
    L, acc = {}, {}
    # Research: only when the AI chose.
    y = b["y_research"]
    m = y >= 0
    if m.any():
        L["research"] = F.cross_entropy(out["research"][m], y[m])
        acc["research"] = (out["research"][m].argmax(-1) == y[m]).float().mean()
    # Queue head.
    yq = b["y_queue"]
    mq = yq >= 0
    if mq.any():
        L["queue"] = F.cross_entropy(out["queue"][mq], yq[mq])
        acc["queue"] = (out["queue"][mq].argmax(-1) == yq[mq]).float().mean()
    # Jobs: farmers and workers, at most the population.
    yj = b["y_jobs"]
    mj = yj[..., 0] >= 0
    if mj.any():
        pop = b["co_pop"].unsqueeze(-1).unsqueeze(-1)
        ar = torch.arange(MAX_POP, device=yj.device)
        logits = out["jobs"].masked_fill(ar > pop, -1e4)
        L["jobs"] = F.cross_entropy(logits[mj].reshape(-1, MAX_POP), yj[mj].reshape(-1))
        acc["jobs"] = (logits[mj].argmax(-1) == yj[mj]).float().mean()
    # Destinations of ships at a star: a star, or stay (-1).
    yd = b["y_dest"]
    md = b["ship_free"] & (yd >= -1)
    if md.any():
        n_st = out["dest"].shape[-1] - 1
        t = torch.where(yd < 0, torch.full_like(yd, n_st), yd)
        # A ship sent somewhere weighs more: staying is most of the labels.
        go = (t != n_st) & (t != b["sh_at"])
        w = torch.where(go, 3.0, 1.0)[md]
        L["dest"] = (F.cross_entropy(out["dest"][md], t[md], reduction="none") * w).sum() / w.sum()
        acc["dest"] = (out["dest"][md].argmax(-1) == t[md]).float().mean()
        mg = md & go
        if mg.any():
            acc["dest_go"] = (out["dest"][mg].argmax(-1) == t[mg]).float().mean()
    return L, acc


def load(path, device):
    """A checkpoint's Policy, built with its config."""
    ck = torch.load(path, map_location=device)
    m = Policy(**ck.get("config", {}))
    sd = ck["model"]
    te = sd.get("type_emb.weight")
    if te is not None and te.shape[0] < m.type_emb.weight.shape[0]:  # before the planet tokens
        sd["type_emb.weight"] = torch.cat([te, m.type_emb.weight.data[te.shape[0]:].to(te.device)])
    missing = m.load_state_dict(sd, strict=False).missing_keys
    return m.to(device), missing
