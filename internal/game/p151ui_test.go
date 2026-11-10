package game

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/save"
)

// uiGame is starsGame with the executable's data and the H strings.
func uiGame(t *testing.T, n string) *Game {
	t.Helper()
	g := starsGame(t, n)
	x, err := newExeImage("", nil)
	if err != nil {
		t.Skip(err)
	}
	g.exe = x
	if err := g.loadHStrings(); err != nil {
		t.Fatal(err)
	}
	return g
}

// TestStarDistanceAcrossBlackHole: F9 prints the parsecs between two stars
// with a black hole between them as for any other pair (community patch 1.51).
func TestStarDistanceAcrossBlackHole(t *testing.T) {
	t.Parallel()
	g := uiGame(t, "mid")
	s := &g.state
	a, b := -1, -1
	for i := 0; i < int(s.NumStars) && b < 0; i++ {
		if !g.starOnScreen(i) {
			continue
		}
		if a < 0 {
			a = i
		} else {
			b = i
		}
	}
	if b < 0 {
		t.Fatal("two stars on screen")
	}
	v := g.e.V
	draw := func(blocked bool) []byte {
		bits := &s.Stars[a].BlackHoleBlocked
		bits[b>>3] &^= 1 << (b & 7)
		if blocked {
			bits[b>>3] |= 1 << (b & 7)
		}
		g.ms.multiColoredLineStart = 0
		v.SetPageOff()
		clear(v.Current)
		v.SetWindow(0x16, 0x16, 0x20f, 0x1a5)
		v.ClipOn()
		if err := g.printStarDistance(a, b); err != nil {
			t.Fatal(err)
		}
		v.ClipOff()
		v.ResetWindow()
		return bytes.Clone(v.Current)
	}
	clearPair := draw(false)
	if bytes.Count(clearPair, []byte{0}) == len(clearPair) {
		t.Fatal("nothing drawn")
	}
	if !bytes.Equal(draw(true), clearPair) {
		t.Error("across a black hole the label differs from the parsecs")
	}
}

// TestEnemyMovesLines: with Enemy Moves on, an enemy fleet in flight to a
// star without the player's colonies gets its red line (community patch 1.51).
func TestEnemyMovesLines(t *testing.T) {
	t.Parallel()
	g := uiGame(t, "late")
	s := &g.state
	p := int(s.PlayerNum)
	dest := -1
	for i := 0; i < int(s.NumStars) && dest < 0; i++ {
		if g.starOnScreen(i) && s.Stars[i].ColonyOwners>>p&1 == 0 {
			out := false
			s.starColonies(i, func(c *save.Colony) { out = out || c.IsOutpost != 0 })
			if !out {
				dest = i
			}
		}
	}
	g.buildShipIcons()
	ship := -1
	for i := 0; i < int(g.stacks.iconCount) && ship < 0; i++ {
		if k := g.iconShip(i); int(s.Ships[k].Owner) != p {
			ship = k
		}
	}
	if dest < 0 || ship < 0 {
		t.Fatalf("star %d, enemy ship %d", dest, ship)
	}
	sh := &s.Ships[ship]
	sh.Status, sh.Location, sh.ETA = 1, int16(1000+dest), 3
	v := g.e.V
	draw := func(on bool) []byte {
		g.settings[0x03] = 0
		if on {
			g.settings[0x03] = 1
		}
		g.ms.multiColoredLineStart = 0
		v.SetPageOff()
		clear(v.Current)
		g.mapWindow()
		g.doShipDestinationLines()
		v.ClipOff()
		v.ResetWindow()
		return bytes.Clone(v.Current)
	}
	off, on := draw(false), draw(true)
	n := 0
	for i := range on {
		if on[i] != off[i] {
			if off[i] != 0 {
				t.Fatal("Enemy Moves took a pixel away")
			}
			n++
		}
	}
	if n == 0 {
		t.Error("no line for the enemy fleet with Enemy Moves on")
	}
}

// TestAutosaveEveryTurn: a local game with Auto Save on writes SAVE10.GAM on
// every stardate, as network games do (community patch 1.51; 1.40b23: every 4th).
func TestAutosaveEveryTurn(t *testing.T) {
	t.Parallel()
	g := techGame(t, "start")
	g.dir = t.TempDir() // never game/
	g.gameType = save.GameSingle
	name := filepath.Join(g.dir, "SAVE10.GAM")
	base := g.state.Stardate
	for k := uint32(0); k < 4; k++ {
		g.state.Stardate = base + k
		for _, on := range []uint8{0, 1} {
			os.Remove(name)
			g.settings[0xb] = on
			if err := g.doAutosave(); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(name)
			if on == 0 {
				if err == nil {
					t.Fatalf("stardate %d: autosave with the option off", g.state.Stardate)
				}
				continue
			}
			if err != nil {
				t.Fatalf("stardate %d: no autosave: %v", g.state.Stardate, err)
			}
			f, err := save.Parse(b)
			if err != nil || f.U32(save.BStardate) != g.state.Stardate {
				t.Fatalf("stardate %d: autosave %v", g.state.Stardate, err)
			}
		}
	}
}
