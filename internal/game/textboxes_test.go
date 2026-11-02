package game

import (
	"testing"

	"github.com/openorion2/openorion2/internal/engine"
)

func boxGame() *Game {
	return &Game{e: &engine.Engine{V: engine.NewVideo()}}
}

func TestDrawHighlightBox(t *testing.T) {
	t.Parallel()
	g := boxGame()
	v := g.e.V
	g.drawHighlightBox(10, 20, 6, 5, [4]byte{1, 2, 3, 4})
	px := func(x, y int) byte { return v.Current[y*engine.W+x] }
	cases := []struct {
		x, y int
		c    byte
	}{
		{12, 20, 4}, {16, 20, 4}, {16, 23, 4}, // outer top, right
		{12, 21, 3}, {15, 23, 3}, // inner top, right
		{10, 23, 1}, {13, 25, 1}, {16, 25, 1}, // outer left, bottom (over the right line)
		{11, 23, 2}, {13, 24, 2}, // inner left, bottom
		{13, 22, 0}, // inside untouched
	}
	for _, c := range cases {
		if got := px(c.x, c.y); got != c.c {
			t.Errorf("(%d,%d) = %d, want %d", c.x, c.y, got, c.c)
		}
	}
}

func TestDiagonalLineFill(t *testing.T) {
	t.Parallel()
	g := boxGame()
	v := g.e.V
	v.SetWindow(1, 2, 300, 400)
	g.diagonalLineFill(100, 100, 131, 131, 5, 9, 7) // 5 divides both: kept
	if v.WinX1 != 1 || v.WinY1 != 2 || v.WinX2 != 300 || v.WinY2 != 400 || v.Clipping {
		t.Fatalf("window/clip not restored: %d %d %d %d %v", v.WinX1, v.WinY1, v.WinX2, v.WinY2, v.Clipping)
	}
	for y := range engine.H {
		for x := range engine.W {
			c := v.Current[y*engine.W+x]
			in := x >= 100 && x <= 131 && y >= 100 && y <= 131
			if !in && c != 0 {
				t.Fatalf("pixel outside window at (%d,%d)", x, y)
			}
		}
	}
	if c := v.Current[105*engine.W+105]; c != 9 {
		t.Errorf("(105,105) = %d, want main colour 9", c)
	}
}
