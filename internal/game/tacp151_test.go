package game

import (
	"testing"

	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
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
		T.set16(tsOfficer, -1)
		T.set8(0x42, 0xff) // no tractor
		T.set8(tsStasis, 0xff)
		T.set8(0x137, 0xff) // no black hole
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

// driveless makes record 1 player 0's ship without drive.
func driveless(g *Game, td bool) {
	t := g.tsp(1)
	t.set8(tsDrive, 0)
	t.set8(tsCSpeed, 3)
	g.state.Players[0].Picks[save.PickTransDimensional] = int8(b2i(td))
}

// TestDrivelessShipSpeed: a ship without drive is no satellite; only
// trans-dimensional moves it (community patch 1.51).
func TestDrivelessShipSpeed(t *testing.T) {
	t.Parallel()
	g := tacGame(t)
	driveless(g, true)
	if v := g.calcCurrentSpeed(1); v != 4 {
		t.Errorf("trans-dimensional ship without drive: speed %d, want 4", v)
	}
	driveless(g, false)
	if v := g.calcCurrentSpeed(1); v != 0 {
		t.Errorf("ship without drive: speed %d, want 0", v)
	}
	// A satellite stays still, trans-dimensional or not.
	g.tsp(1).set16(tsID, -1)
	g.state.Players[0].Picks[save.PickTransDimensional] = 1
	if v := g.calcCurrentSpeed(1); v != 0 {
		t.Errorf("satellite: speed %d, want 0", v)
	}
}

// TestDrivelessShipBattle: player 1's trans-dimensional frigates without
// drive defend at a star without colony (1.40: Colonies[-1], or a hang).
func TestDrivelessShipBattle(t *testing.T) {
	t.Parallel()
	g := techGame(t, "start")
	g.cache = map[cacheKey]*gfx.Picture{}
	s := &g.state
	// Recipe "start": ships 0, 1 are player 0's frigates at star 1, 3, 4
	// player 1's; turn 0 has not set their combat speed yet.
	att, def := []int16{0, 1}, []int16{3, 4}
	star := s.Ships[0].Location
	for _, id := range att {
		s.Ships[id].Design.CombatSpeed = 4
	}
	for _, id := range def {
		h := &s.Ships[id]
		h.Location = star
		h.Design.Drive = 0
		h.Design.CombatSpeed = 0
	}
	s.Players[1].Picks[save.PickTransDimensional] = 1
	// The recipe's seed decides the battle: the defenders win.
	if r := g.tacticalCombat(att, def, star, -1, false); r != -1 {
		t.Errorf("result %d, want -1", r)
	}
	for i := int16(1); i < g.tc.n; i++ {
		if T := g.tsp(i); T.u8(tsOwner) == 1 && (T.u8(tsGone) != 0 || T.i16(tsSpeed) != 4) {
			t.Errorf("record %d (ship %d): gone %d, speed %d; want 0 and 4", i, T.i16(tsID), T.u8(tsGone), T.i16(tsSpeed))
		}
	}
	// The ship view shows its trans-dimensional speed.
	var v viewShip
	if err := g.loadDisplayCombatShip(&v, 3); err != nil || v.speed != 4 {
		t.Errorf("ship view: speed %d, %v; want 4", v.speed, err)
	}
}

// TestRetreatBlocked: a ship ordered to retreat leaves only when it can
// still move and still retreats (community patch 1.51).
func TestRetreatBlocked(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		set      func(T *tship)
		gone, rt uint8
	}{
		{"mobile", func(*tship) {}, 5, 2},
		{"in a black hole", func(T *tship) { T.set8(0x137, 2) }, 0, 0},
		{"drive destroyed", func(T *tship) { T.set16(tsDriveCur, 0) }, 0, 0},
		{"order gone", func(T *tship) { T.set8(0x4b, 0) }, 0, 0},
	} {
		g := tacGame(t)
		g.tc.onScreen = 1
		T := g.tsp(1)
		T.set8(tsDrive, 1)
		T.set16(tsDriveMax, 10)
		T.set16(tsDriveCur, 10)
		T.set16(tsStruct, 10)
		T.set8(tsCSpeed, 4)
		T.set16(0x3d, 3)
		T.set8(0x4b, 1)
		c.set(T)
		g.retreatShip(1)
		if T.u8(tsGone) != c.gone || T.u8(0x4b) != c.rt {
			t.Errorf("%s: gone %d, retreat %d; want %d, %d", c.name, T.u8(tsGone), T.u8(0x4b), c.gone, c.rt)
		}
		if c.gone == 0 && c.name != "order gone" && T.i16(0x3d) != 0 {
			t.Errorf("%s: %d moves left, want 0", c.name, T.i16(0x3d))
		}
	}
}
