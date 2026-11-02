package game

import (
	"testing"

	"github.com/openorion2/openorion2/internal/gfx"
)

// TestDeployShipsToAttackAntares opens the box on a player stack at a star
// given a dimensional portal and sends its selected ships to Antares.
func TestDeployShipsToAttackAntares(t *testing.T) {
	t.Parallel()
	for name, g := range moveOrderGames(t) {
		s := &g.state
		g.cache = map[cacheKey]*gfx.Picture{}
		g.buildShipIcons()
		st := &g.stacks
		stack := -1
		for k := 0; k < int(st.count) && stack < 0; k++ {
			h := &s.Ships[st.node[st.start[k]].ship]
			if int(h.Owner) == int(s.PlayerNum) && h.Status == 0 && h.Design.Type == 0 && g.xyToStarID(h.X, h.Y) >= 0 {
				stack = k
			}
		}
		if stack < 0 {
			continue
		}
		g.fb.stack = int16(stack)
		head := g.fleetHead()
		star := g.xyToStarID(head.X, head.Y)
		if g.attackAntaresFlag() {
			t.Fatalf("%s: attack Antares without a portal", name)
		}
		s.Stars[star].DimensionalPortal = 1 << s.PlayerNum
		if !g.attackAntaresFlag() {
			t.Fatalf("%s: no attack Antares button at a portal", name)
		}
		var want []int16
		for n := st.start[stack]; n != -1; n = st.node[n].next {
			st.node[n].selected = 1
			want = append(want, st.node[n].ship)
		}
		if got := g.deployShipsToAttackAntares(); got != len(want) {
			t.Fatalf("%s: deployed %d, want %d", name, got, len(want))
		}
		for _, id := range want {
			sh := &s.Ships[id]
			// Through the portal: one turn, speed -1, already in transit.
			if int(sh.Location) != int(s.NumStars)+1000 || sh.Status != 2 || sh.ETA != 1 || sh.Speed != -1 {
				t.Errorf("%s: ship %d: location %d status %d eta %d speed %d", name, id, sh.Location, sh.Status, sh.ETA, sh.Speed)
			}
		}
		return
	}
	t.Skip("no player combat stack at a star")
}
