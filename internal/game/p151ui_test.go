package game

import (
	"bytes"
	"testing"
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
