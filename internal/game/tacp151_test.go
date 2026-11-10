package game

import (
	"testing"
)

// tacGame is recipe "start" with an empty combat engine: record 1 is
// player 0's ship, record 2 player 1's.
func tacGame(t *testing.T) *Game {
	t.Helper()
	g := techGame(t, "start")
	g.tc = &tactical{ui: autoUI{}}
	g.tc.clearBlocks()
	g.tc.n = 3
	g.tc.att, g.tc.def = 0, 1
	g.tsp(0).set8(tsGone, 5)
	for i := int16(1); i < 3; i++ {
		T := g.tsp(i)
		T.set8(tsOwner, uint8(i-1))
		T.set16(tsID, i)
	}
	return g
}

// TestMissileOfCapturedShip: fighters out when a crystal (owner 10) takes
// their ship over count by their own owner (community patch 1.51).
func TestMissileOfCapturedShip(t *testing.T) {
	t.Parallel()
	g := tacGame(t)
	M := g.msl(0)
	M.set16(tmWeapon, 0x1c) // interceptors
	M.set16(tmOwner, 0)
	M.set16(tmShip, 1)
	M.set16(tmCount, 4)
	M.set16(tmTarget, 2)
	g.tsp(1).set8(tsOwner, 10)
	if h := g.missileHits(0x1c, 10, 0); h != 1 {
		t.Fatalf("a monster's fighter takes %d hits, want 1", h)
	}
	want := g.missileHits(0x1c, 0, 0)
	if want < 2 {
		t.Fatalf("player 0's fighter takes %d hits", want)
	}
	// One hit short of a kill, then one more.
	M.set16(tmFuel, want-1)
	d := int16(1)
	g.applyDamageToMissile(0, &d)
	if d != 1 || M.i16(tmFuel) != 0 {
		t.Errorf("the last hit destroyed %d, left %d; want 1 and 0", d, M.i16(tmFuel))
	}
	// Record 2 has no mounts: no kills, divided by the hits.
	if v := g.targetMissileValue(2, 0, 0); v != -10000 {
		t.Errorf("target missile value %d, want -10000", v)
	}
}
