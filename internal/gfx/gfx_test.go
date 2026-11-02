package gfx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openorion2/openorion2/internal/lbx"
)

// TestGamePictures decodes every frame of every entry that parses as a picture.
// Archives also hold palettes, fonts, sounds and tables, which fail Parse; the
// test only insists that whatever does parse also draws.
func TestGamePictures(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "game", "*"))
	var pics, frames int
	for _, f := range files {
		if !strings.EqualFold(filepath.Ext(f), ".lbx") {
			continue
		}
		a, err := lbx.Open(f)
		if errors.Is(err, lbx.ErrSmacker) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < a.Len(); i++ {
			e, _ := a.Entry(i)
			p, err := Parse(e)
			if err != nil {
				continue
			}
			pics++
			c := p.NewCanvas()
			for k := 0; k < p.Frames; k++ {
				if err := p.Draw(c, k); err != nil {
					t.Errorf("%s #%d frame %d: %v", filepath.Base(f), i, k, err)
					break
				}
				frames++
			}
		}
	}
	if pics == 0 {
		t.Skip("game files not present")
	}
	t.Logf("%d pictures, %d frames", pics, frames)
}

func TestPaletteFromFonts(t *testing.T) {
	a, err := lbx.Open(filepath.Join("..", "..", "game", "FONTS.LBX"))
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("game files not present")
	}
	if err != nil {
		t.Fatal(err)
	}
	e, _ := a.Entry(1)
	p, err := ParsePalette(e)
	if err != nil {
		t.Fatal(err)
	}
	// Colour 0 is black and the 6-bit maximum widens to 255.
	if c := p[0]; c.R != 0 || c.G != 0 || c.B != 0 {
		t.Errorf("colour 0 = %v", c)
	}
	if vga(63) != 255 || vga(0) != 0 {
		t.Errorf("vga(63)=%d vga(0)=%d", vga(63), vga(0))
	}
}

// TestDrawClippedMatchesUnclipped draws BUFFER0.LBX pictures well inside a
// screen-sized window: the clipped blitters must then match the unclipped
// ones, except that they skip colour 0.
func TestDrawClippedMatchesUnclipped(t *testing.T) {
	a, err := lbx.Open(filepath.Join("..", "..", "game", "BUFFER0.LBX"))
	if err != nil {
		t.Skip(err)
	}
	w := Window{0, 0, ScreenW - 1, ScreenH - 1}
	n := 0
	for e := 0; e < a.Len(); e++ {
		d, err := a.Entry(e)
		if err != nil {
			continue
		}
		p, err := Parse(d)
		if err != nil || p.Kind() == KindBackground || p.Width > 300 || p.Height > 200 {
			continue
		}
		for i := 0; i < p.Frames; i++ {
			want := make([]byte, ScreenW*ScreenH)
			got := make([]byte, ScreenW*ScreenH)
			for k := range want {
				want[k], got[k] = 0xff, 0xff
			}
			if err := p.render(i, func(row, col int, v byte) {
				if v != 0 {
					want[(100+row)*ScreenW+100+col] = v
				}
			}); err != nil {
				t.Fatal(err)
			}
			if err := p.DrawClipped(got, ScreenW, 100, 100, i, w); err != nil {
				t.Fatalf("entry %d frame %d: %v", e, i, err)
			}
			for k := range want {
				if want[k] != got[k] {
					t.Fatalf("entry %d frame %d: pixel %d,%d is %d, want %d", e, i, k%ScreenW, k/ScreenW, got[k], want[k])
				}
			}
			n++
		}
	}
	t.Logf("%d frames", n)
}
