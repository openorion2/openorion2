package game

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/lbx"
	"github.com/openorion2/openorion2/internal/save"
)

func moveOrderGames(t *testing.T) map[string]*Game {
	dir := filepath.Join("..", "..", "game")
	files, _ := filepath.Glob(filepath.Join(savesDir(), "SAVE*.GAM"))
	if len(files) == 0 {
		t.Skip("no saves in game/")
	}
	gs := map[string]*Game{}
	for _, p := range files {
		f, err := save.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		g := &Game{dir: dir, lbx: map[string]*lbx.Archive{}, Log: t.Logf}
		if err := g.state.readBlocks(f, 9); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		gs[filepath.Base(p)] = g
	}
	return gs
}

func TestIsqrt(t *testing.T) {
	t.Parallel()
	for _, n := range []uint32{0, 1, 2, 3, 4, 15, 16, 17, 255, 256, 0xffff, 0x10000, 0x10001, 899, 900, 123456789, 0xfffe0000, 0xfffe0001, 0xffffffff} {
		want := uint32(math.Sqrt(float64(n)))
		if n >= 0xfffe0001 {
			want = 0xffff
		}
		if got := isqrt(n); got != want {
			t.Errorf("isqrt(%d) = %d, want %d", n, got, want)
		}
	}
}

func TestParsecsBetweenPoints(t *testing.T) {
	t.Parallel()
	cases := []struct {
		x1, y1, x2, y2 int16
		want           int32
	}{
		{0, 0, 0, 0, 0}, {0, 0, 30, 0, 1}, {0, 0, 31, 0, 2}, {0, 0, 1, 0, 1},
		{10, 10, 40, 50, 2}, {0, 0, 300, 400, 17}, {0, 0, -60, 0, 2},
	}
	for _, c := range cases {
		if got := parsecsBetweenPoints(c.x1, c.y1, c.x2, c.y2); got != c.want {
			t.Errorf("parsecs(%d,%d,%d,%d) = %d, want %d", c.x1, c.y1, c.x2, c.y2, got, c.want)
		}
	}
}

func TestPointWithinDOfSeg(t *testing.T) {
	t.Parallel()
	cases := []struct {
		px, py int16
		want   bool
	}{
		{50, 0, true}, {50, 45, true}, {50, 46, false}, {50, -45, true},
		{150, 0, false}, {-1, 0, false}, {0, 0, true}, {100, 0, true},
		{-30, 10, false}, {120, 10, false}, {0, 40, true},
	}
	for _, c := range cases {
		if got := pointWithinDOfSeg(c.px, c.py, 45, 0, 0, 100, 0); got != c.want {
			t.Errorf("(%d,%d): %v, want %v", c.px, c.py, got, c.want)
		}
	}
	// A long diagonal takes the halving branch.
	if !pointWithinDOfSeg(700, 700, 45, 0, 0, 1400, 1400) || pointWithinDOfSeg(700, 800, 45, 0, 0, 1400, 1400) {
		t.Error("long segment")
	}
}

// Initialize_Black_Hole_Blocks_ builds the saved rows with the same test,
// so recomputing them checks Point_Within_D_Of_Seg_ on real maps.
func TestBlackHoleRowsMatchSaves(t *testing.T) {
	t.Parallel()
	for name, g := range moveOrderGames(t) {
		s := &g.state
		bh := s.collectBlackHoles()
		for i := 0; i < int(s.NumStars); i++ {
			A := &s.Stars[i]
			for j := 0; j < int(s.NumStars); j++ {
				B := &s.Stars[j]
				if i == j || A.SpectralClass == save.SpectralBlackHole || B.SpectralClass == save.SpectralBlackHole {
					continue
				}
				x1, y1, x2, y2 := A.X, A.Y, B.X, B.Y
				if j > i { // the original tests pairs (i, j < i) only
					x1, y1, x2, y2 = B.X, B.Y, A.X, A.Y
				}
				want := false
				for _, h := range bh {
					want = want || pointWithinDOfSeg(s.Stars[h].X, s.Stars[h].Y, 45, x1, y1, x2, y2)
				}
				if got := blackHoleBit(A, j); got != want {
					t.Errorf("%s: stars %d-%d saved %v, computed %v", name, i, j, got, want)
				}
			}
		}
	}
}

func TestShipsTryToMoveToSaves(t *testing.T) {
	t.Parallel()
	for name, g := range moveOrderGames(t) {
		s := &g.state
		ordered, ships := 0, 0
		for id := 0; id < int(s.NumShips); id++ {
			sh := &s.Ships[id]
			if sh.Owner < 0 || sh.Status > 2 {
				continue
			}
			ships++
			ids := []int16{int16(id), -1}
			here := g.xyToStarID(sh.X, sh.Y)
			for dest := 0; dest < int(s.NumStars); dest++ {
				info := g.shipsTryToMoveTo(ids, dest)
				switch {
				case info.canMove:
					ordered++
					if info.turns < 0 {
						t.Fatalf("%s: ship %d to %d: can move with %d turns", name, id, dest, info.turns)
					}
					if dest == here && info.turns != 0 {
						t.Errorf("%s: ship %d at star %d: %d turns to itself", name, id, dest, info.turns)
					}
					if dest != here && info.turns == 0 {
						t.Errorf("%s: ship %d to %d: 0 turns", name, id, dest)
					}
				case info.flux:
				default:
					if info.turns != -1 {
						t.Errorf("%s: ship %d to %d: refused with %d turns", name, id, dest, info.turns)
					}
					if !info.blocked && !info.outOfRange && !info.noFTL {
						t.Errorf("%s: ship %d to %d: refused without reason %+v", name, id, dest, info)
					}
				}
				if info.noFuel || info.unused3 != 0 {
					t.Errorf("%s: bytes +1/+3 written", name)
				}
			}
		}
		if ships > 0 && ordered == 0 {
			t.Errorf("%s: no possible move order at all", name)
		}
	}
}

// A hand-checked order: the first human ship at a star, to the nearest
// other colony star of its owner, travels ceil(parsecs/speed) turns when
// nothing interferes.
func TestShipsTryToMoveToTurns(t *testing.T) {
	t.Parallel()
	checked := 0
	for name, g := range moveOrderGames(t) {
		s := &g.state
		pn := int(s.PlayerNum)
		p := &s.Players[pn]
		s.NumNebulas = 0 // nebulas end turns early
		for id := 0; id < int(s.NumShips); id++ {
			sh := &s.Ships[id]
			if int(sh.Owner) != pn || sh.Status != 0 || sh.Officer != -1 {
				continue
			}
			here := g.xyToStarID(sh.X, sh.Y)
			if here < 0 {
				continue
			}
			for dest := 0; dest < int(s.NumStars); dest++ {
				H, D := &s.Stars[here], &s.Stars[dest]
				if dest == here || D.WarpInterdictor != 0 || H.WormholeTo == int8(dest) ||
					maskBit(H.Stargate, pn) && maskBit(D.Stargate, pn) || maskBit(H.Jumpgate, pn) && maskBit(D.Jumpgate, pn) {
					continue
				}
				info := g.shipsTryToMoveTo([]int16{int16(id), -1}, dest)
				if !info.canMove {
					continue
				}
				// Each turn moves speed parsecs along the line from the turn's start,
				// so the ETA is the parsec distance over speed, rounded up, give or
				// take the rounding of intermediate positions.
				d := parsecsBetweenPoints(H.X, H.Y, D.X, D.Y)
				sp := int32(p.FTLSpeed)
				want := (d + sp - 1) / sp
				if got := int32(info.turns); got < want-1 || got > want+1 {
					t.Errorf("%s: ship %d %d->%d: %d turns, about %d expected", name, id, here, dest, got, want)
				}
				if info.speed != int8(sp) || info.navigator {
					t.Errorf("%s: ship %d: speed %d nav %v", name, id, info.speed, info.navigator)
				}
				checked++
			}
			break
		}
	}
	if checked == 0 {
		t.Skip("no suitable ship")
	}
}

func TestMakeShipsMoveTo(t *testing.T) {
	t.Parallel()
	for name, g := range moveOrderGames(t) {
		s := &g.state
		pn := int(s.PlayerNum)
		for id := 0; id < int(s.NumShips); id++ {
			sh := &s.Ships[id]
			if int(sh.Owner) != pn || sh.Status != 0 {
				continue
			}
			for dest := 0; dest < int(s.NumStars); dest++ {
				ids := []int16{int16(id), -1}
				info := g.shipsTryToMoveTo(ids, dest)
				if !info.canMove || info.turns == 0 {
					continue
				}
				saved := *sh
				g.makeShipsMoveTo(ids, dest, &info)
				if sh.Location != int16(dest+1000) || sh.Status != 2 || sh.ETA != uint8(info.turns) || sh.Speed != info.speed {
					t.Errorf("%s: ship %d after order: %+v", name, id, *sh)
				}
				*sh = saved
				break
			}
			break
		}
	}
}

func TestCanOrderShipSaves(t *testing.T) {
	t.Parallel()
	for name, g := range moveOrderGames(t) {
		s := &g.state
		for id := 0; id < int(s.NumShips); id++ {
			sh := &s.Ships[id]
			c := s.canOrderShip(id)
			if (sh.Status == 0 || sh.Status == 2) && int16(sh.Owner) < s.NumPlayers && !c {
				t.Errorf("%s: ship %d status %d not orderable", name, id, sh.Status)
			}
			if sh.Status > 2 && c {
				t.Errorf("%s: ship %d status %d orderable", name, id, sh.Status)
			}
			if g.shipCanBeSelected(id) != (c && int(sh.Owner) == int(s.PlayerNum)) {
				t.Errorf("%s: ship %d selection", name, id)
			}
		}
	}
}
