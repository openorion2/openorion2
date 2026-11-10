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

// TestMissileTargetByMissileOwner: a ship's missiles count as its owner's
// after the ship is captured (community patch 1.51).
func TestMissileTargetByMissileOwner(t *testing.T) {
	t.Parallel()
	g := tacGame(t)
	beam := int16(-1)
	for w := int16(1); w < 0x40 && beam < 0; w++ {
		if g.wpnU8(int(w), weaponType) == 0 && g.wpnI16(int(w), weaponMaxDmg) > 0 {
			beam = w
		}
	}
	A := g.tsp(1)
	A.set8(tsX, 5)
	A.set8(tsY, 5)
	for k := 0; k < 8; k++ {
		A.set16(tw(k, twWeapon), beam)
		A.set8(tw(k, twCount), 200)
		A.set8(tw(k, twLeft), 200)
		A.set8(tw(k, twArc), 0xf)
		A.set8(tw(k, twOn), 1)
		A.set16(tw(k, twAmmo), -1)
	}
	// Record 2 launched the missile for player 0, then player 1 took it.
	M := g.msl(0)
	M.set16(tmWeapon, 0xe)
	M.set16(tmOwner, 0)
	M.set16(tmShip, 2)
	M.set16(tmCount, 2)
	M.set16(tmTarget, 1)
	M.set16(tmX, 7*0x14)
	M.set16(tmY, 5*0x14)
	g.tsp(2).set8(tsOwner, 1)
	var v int32
	if m := g.getCurrentMissileTarget(1, &v); m != -1 {
		t.Errorf("player 0 targets its own missile %d (value %d)", m, v)
	}
	// Player 1's ship launched it for player 1 and was taken by player 0.
	M.set16(tmOwner, 1)
	g.tsp(2).set8(tsOwner, 0)
	if m := g.getCurrentMissileTarget(1, &v); m != 0 {
		t.Errorf("player 0 targets %d (value %d), want player 1's missile 0", m, v)
	}
}
