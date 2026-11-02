package game

import "testing"

// TestNetStateHashSlots: the slots past the nebula, planet and star counts
// (a regenerated galaxy's leftovers on the host) do not change the hash;
// the counted ones do.
func TestNetStateHashSlots(t *testing.T) {
	t.Parallel()
	g := &Game{}
	s := &g.state
	s.NumNebulas, s.PlanetCount, s.NumStars = 1, 10, 5
	h := g.netStateHash()
	s.Nebulas[2].X = 7
	s.Planets[10].Star = 3
	s.Stars[7].X = 9
	if g.netStateHash() != h {
		t.Error("a slot past a count changed the hash")
	}
	s.Planets[9].Star = 3
	if g.netStateHash() == h {
		t.Error("a counted planet did not change the hash")
	}
}
