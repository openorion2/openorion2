//go:build aitrain

package game

import "github.com/openorion2/openorion2/internal/aiplay"

// simObserve is player p's view as the network's rows: the simulator
// observes through the view -control serves (aiplay.FromView), so its rows
// equal those of a game played over -control. t is what they remember
// between turns.
func (g *Game) simObserve(t *simTrack, p int, r *SimRecord) {
	_, rec := aiplay.FromView(g.playerView(p), func(i int) int { return t.shipIdle[i] }, func(i int) int { return t.headAge[i] })
	r.Me, r.Des, r.Pl, r.Star, r.Pn, r.Co, r.Sh = rec.Me, rec.Des, rec.Pl, rec.Star, rec.Pn, rec.Co, rec.Sh
}
