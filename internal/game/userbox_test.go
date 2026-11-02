package game

import (
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/lbx"
)

func TestTriInterlacedFill(t *testing.T) {
	t.Parallel()
	g := boxGame()
	v := g.e.V
	g.triInterlacedFill(12, 9, 3, 2, 7, 8, 9) // swapped corners
	for y := 2; y <= 9; y++ {
		want := [3]byte{7, 8, 9}[(y-2)%3]
		for x := 3; x <= 12; x++ {
			if got := v.Current[y*engine.W+x]; got != want {
				t.Fatalf("(%d,%d) = %d, want %d", x, y, got, want)
			}
		}
	}
	if v.Current[1*engine.W+3] != 0 || v.Current[2*engine.W+13] != 0 {
		t.Error("fill leaked out of the rectangle")
	}
	g = boxGame()
	v = g.e.V
	v.SetWindow(0, 5, 639, 479)
	v.ClipOn()
	g.triInterlacedFill(0, 0, 0, 7, 1, 2, 3) // rows restart at the clipped top
	if v.Current[5*engine.W] != 1 || v.Current[6*engine.W] != 2 || v.Current[4*engine.W] != 0 {
		t.Error("clipped fill does not restart at the window top")
	}
}

func TestGetMoxTextBoxBaseColors(t *testing.T) {
	t.Parallel()
	g := &Game{}
	cases := []struct {
		screen, kind int
		a, b         byte
	}{
		{0, 0, 0xa0, 0xa2}, {0, 1, 0xc0, 0xc2}, {0, 2, 0xe0, 0xe2}, {0, 3, 0xa0, 0xa2},
		{9, 0, 0xa9, 0xab}, {10, 1, 0xc3, 0xc5}, {0x14, 0, 0xa6, 0xa8}, {0x19, 2, 0xe6, 0xe8},
	}
	for _, c := range cases {
		g.currentScreen = c.screen
		if a, b := g.getMoxTextBoxBaseColors(c.kind); a != c.a || b != c.b {
			t.Errorf("screen %#x kind %d: %#x %#x, want %#x %#x", c.screen, c.kind, a, b, c.a, c.b)
		}
	}
}

// With an identity remap table Remap_Draw draws what Draw draws, clipped
// or not.
func TestRemapDrawMatchesDraw(t *testing.T) {
	t.Parallel()
	a, err := lbx.Open(filepath.Join("..", "..", "game", "TEXTBOX.LBX"))
	if err != nil {
		t.Skip("no TEXTBOX.LBX in game/")
	}
	for _, clip := range []bool{false, true} {
		for entry := 0; entry < 4; entry++ {
			g := &Game{dir: filepath.Join("..", "..", "game"), lbx: map[string]*lbx.Archive{"TEXTBOX.LBX": a}}
			g.e = &engine.Engine{V: engine.NewVideo()}
			r := g.e.PictureRemap()
			for i := range r {
				r[i] = byte(i)
			}
			p1, err := g.picture("TEXTBOX.LBX", entry)
			if err != nil {
				t.Fatal(err)
			}
			p2, _ := g.picture("TEXTBOX.LBX", entry)
			v := g.e.V
			if clip {
				v.SetWindow(40, 30, 200, 60)
				v.ClipOn()
			}
			if err := g.remapDraw(20, 20, p1); err != nil {
				t.Fatal(err)
			}
			want := append([]byte(nil), v.Current...)
			clear(v.Current)
			if err := g.e.Draw(20, 20, p2); err != nil {
				t.Fatal(err)
			}
			for i := range want {
				if want[i] != v.Current[i] {
					t.Fatalf("entry %d clip %v: pixel (%d,%d) %d, Draw %d", entry, clip, i%engine.W, i/engine.W, want[i], v.Current[i])
				}
			}
		}
	}
}
