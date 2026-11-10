package game

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/save"
)

// d6Game is recipe n with the player's first colony.
func d6Game(t *testing.T, n string) (*Game, int, int) {
	t.Helper()
	g := techGame(t, n)
	s := &g.state
	p := int(s.PlayerNum)
	for i := 0; i < int(s.NumColonies); i++ {
		if c := &s.Colonies[i]; int(c.Owner) == p && c.IsOutpost == 0 {
			return g, p, i
		}
	}
	t.Fatal("no colony")
	return nil, 0, 0
}

// TestDesign6NewGame: a new game gives the human six designs, the sixth a
// size up from the fifth (1.40b23 cleared it).
func TestDesign6NewGame(t *testing.T) {
	t.Parallel()
	g, p, _ := d6Game(t, "start")
	d := &g.state.Players[p].Designs
	for k := range d {
		if save.CString(d[k].Name[:]) == "" || d[k].Type != 0 {
			t.Errorf("design %d: %q type %d", k, save.CString(d[k].Name[:]), d[k].Type)
		}
	}
	if d[5].Size != d[4].Size+1 {
		t.Errorf("sizes %d then %d", d[4].Size, d[5].Size)
	}
}

// TestDesign6Support: a colony ship built through slot 5 leaves the sixth
// design as it was, when it is made and when no ship is left.
func TestDesign6Support(t *testing.T) {
	t.Parallel()
	g, p, ci := d6Game(t, "mid")
	s := &g.state
	pl := &s.Players[p]
	pl.Designs[5] = pl.Designs[2]
	copy(pl.Designs[5].Name[:], "Sixth\x00")
	want := pl.Designs[5]
	g.loadColonyShipDesign(p, &pl.Designs[5])
	sh := g.createShip(p, 5, ci)
	if sh == -1 || s.Ships[sh].Design.Type != 1 {
		t.Fatalf("ship %d", sh)
	}
	if pl.Designs[5] != want {
		t.Errorf("the sixth design is now %q", save.CString(pl.Designs[5].Name[:]))
	}
	// No ship left for p: it comes back too.
	s.NumShips = maxShips
	for i := range s.Ships {
		if int(s.Ships[i].Owner) == p && s.Ships[i].Status == 5 {
			s.Ships[i].Status = 0
		}
	}
	g.loadTransportShipDesign(p, &pl.Designs[5])
	if g.createShip(p, 5, ci) != -1 {
		t.Skip("a ship was free")
	}
	if pl.Designs[5] != want {
		t.Errorf("after the 500 ships limit the sixth design is %q", save.CString(pl.Designs[5].Name[:]))
	}
}

// TestDesign6Refit: a refit through slot 5 gives it back once the ship is
// queued.
func TestDesign6Refit(t *testing.T) {
	t.Parallel()
	g, p, ci := d6Game(t, "mid")
	s := &g.state
	pl := &s.Players[p]
	want := pl.Designs[5]
	k := int16(-1)
	for i := 0; i < int(s.NumShips); i++ {
		if int(s.Ships[i].Owner) == p && s.Ships[i].Status < 3 && s.Ships[i].Design.Type == 0 {
			k = int16(i)
			break
		}
	}
	if k == -1 {
		t.Skip("no ship")
	}
	g.cache = map[cacheKey]*gfx.Picture{}
	g.colonyHandle = ci
	g.bq.cur = s.Colonies[ci].Queue
	g.bq.active = -1
	g.keepSpare(&pl.Designs[bqRefitSlot])
	pl.Designs[bqRefitSlot] = pl.Designs[0]
	g.bq.refitShip = k
	if err := g.addRefitedShipToQueue(); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(g.bq.cur[:], -(k + 100)) {
		t.Fatalf("not queued: %v", g.bq.cur)
	}
	if pl.Designs[5] != want || g.spare.saved {
		t.Errorf("the sixth design is %q, kept %v", save.CString(pl.Designs[5].Name[:]), g.spare.saved)
	}
}

// TestDesign6Load: a 1.40b23 leftover in slot 5 (a support ship) becomes
// the doom star design on loading; a real design stays.
func TestDesign6Load(t *testing.T) {
	t.Parallel()
	g, p, _ := d6Game(t, "mid")
	g.settings[0xd8] = 0 // tactical combat: the doom star is a tactical design
	s := &g.state
	other := (p + 1) % int(s.NumPlayers)
	g.loadColonyShipDesign(p, &s.Players[p].Designs[5])
	g.spare.saved = false
	keep := s.Players[other].Designs[5]
	keep.Type = 0
	s.Players[other].Designs[5] = keep
	g.fixSpareDesigns()
	if d := &s.Players[p].Designs[5]; d.Type != 0 || d.Size != 5 {
		t.Errorf("leftover became type %d size %d, want a doom star", d.Type, d.Size)
	}
	if s.Players[other].Designs[5] != keep {
		t.Error("a real sixth design changed")
	}
}

// TestDesign6Lists: the sixth design is offered in the build screen and
// by the build lists' design6.
func TestDesign6Lists(t *testing.T) {
	t.Parallel()
	g, ci := blGame(t)
	pl := &g.state.Players[g.state.PlayerNum]
	// Strategic combat: a titan or doom star design needs one of the hulls.
	g.settings[0xd8] = 1
	g.calculateMilitaryArray()
	titan := pl.AppStatus[appTitan] == 3 || pl.AppStatus[appDoomStar] == 3
	if slices.Contains(g.bq.mil, -55) != titan || slices.Contains(g.bq.mil, bqDoomStar) {
		t.Fatalf("strategic military list %v (titan or doom star hull %v)", g.bq.mil, titan)
	}
	g.settings[0xd8] = 0
	g.calculateMilitaryArray()
	if !slices.Contains(g.bq.mil, -55) {
		t.Fatalf("military list %v", g.bq.mil)
	}
	pl.Designs[5] = pl.Designs[1] // a hull the colony can build
	if g.colonyCanBuildProduct(ci, -55) == 0 {
		t.Fatal("the sixth design cannot be built")
	}
	q := blApply(t, g, "design6")
	if q[0] > -100 {
		t.Fatalf("queue %v", q)
	}
}

// TestAutoDesignAll: ALT-AD redoes the six designs a hull up once the doom
// star hull is known, and leaves the tech level as it was.
func TestAutoDesignAll(t *testing.T) {
	t.Parallel()
	g, p, _ := d6Game(t, "mid")
	g.settings[0xd8], g.settings[0xd9] = 0, 1
	pl := &g.state.Players[p]
	for _, doom := range []bool{false, true} {
		pl.AppStatus[appDoomStar] = 0
		if doom {
			pl.AppStatus[appDoomStar] = techKnown
		}
		pl.Designs = [6]save.Design{}
		g.autoDesignAll()
		first := uint8(0)
		if doom {
			first = 1
		}
		if pl.Designs[0].Size != 0 {
			t.Errorf("doom %v: design 1 size %d, want the scout", doom, pl.Designs[0].Size)
		}
		for k := 1; k < 6; k++ {
			if d := &pl.Designs[k]; d.Size != first+uint8(k-1) || save.CString(d.Name[:]) == "" {
				t.Errorf("doom %v: design %d size %d %q", doom, k+1, d.Size, save.CString(d.Name[:]))
			}
		}
		if g.settings[0xd9] != 1 {
			t.Fatalf("tech level left at %d", g.settings[0xd9])
		}
	}
}

// TestAutoDesignKey: Alt+A, Alt+D in the build screen runs it.
func TestAutoDesignKey(t *testing.T) {
	t.Parallel()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	putRecipeSave(t, "start", dir, 1)
	step := 0
	err := host.Run(host.Options{Headless: true, Frames: 60 * 120}, func(sys *host.System) error {
		opt := Options{Load: 1}
		opt.testHook = func(g *Game) {
			g.Log = func(string, ...any) {}
			idle, wait := 0, 0
			var fx, fy int
			g.e.InputHook = func() {
				s := &g.state
				idle++
				switch {
				case step == 0 && !netMainScreenUp(g):
					if idle%20 == 0 {
						sys.PushKey(letterKey('\r'))
					}
				case step == 0:
					step = 1
				case step == 1 && g.currentScreen != ScreenColonySummary:
					if idle%30 == 0 {
						sys.PushKey(letterKey('c'))
					}
				case step == 1:
					x1, y1, x2, y2 := g.e.FieldRect(g.cs.prodF[0])
					fx, fy = (x1+x2)/2, (y1+y2)/2
					sys.WarpMouse(fx, fy)
					step, wait = 2, 0
				case step == 2 && g.currentScreen != ScreenBuildQueue:
					if wait++; wait == 5 {
						sys.Press(fx, fy, 1)
					}
				case step == 2:
					s.Players[s.PlayerNum].Designs = [6]save.Design{}
					step, wait = 3, 0
				case step == 3:
					wait++
					if wait == 5 {
						k := letterKey('a')
						k.Alt = true
						sys.PushKey(k)
						k = letterKey('d')
						k.Alt = true
						sys.PushKey(k)
					}
					if wait == 40 {
						step = 4
						sys.Quit()
					}
				}
			}
			t.Cleanup(func() {
				d := &g.state.Players[g.state.PlayerNum].Designs
				for k := range d {
					if save.CString(d[k].Name[:]) == "" {
						t.Errorf("design %d empty after Alt+A, Alt+D", k+1)
					}
				}
			})
		}
		return Run(sys, dir, opt)
	})
	if err != nil && !errors.Is(err, host.ErrQuit) {
		t.Fatal(err)
	}
	if step != 4 {
		t.Fatalf("stopped at step %d", step)
	}
}
