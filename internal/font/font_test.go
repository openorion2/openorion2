package font

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/lbx"
)

func gameFont(t *testing.T) *Font {
	a, err := lbx.Open(filepath.Join("..", "..", "game", "FONTS.LBX"))
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("game files not present")
	}
	if err != nil {
		t.Fatal(err)
	}
	e, _ := a.Entry(0)
	f, err := Parse(e)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// TestEveryGlyph draws every glyph Set_Font copies in every style; a glyph
// that runs past the data or never ends its rows fails the test.
func TestEveryGlyph(t *testing.T) {
	f := gameFont(t)
	tg := Target{Pix: make([]byte, 64*64), Stride: 64, H: 64}
	col := Solid(1)
	for st := 0; st < Styles; st++ {
		if h := f.Styles[st].Height; h <= 0 || h > 64 {
			t.Fatalf("style %d height %d", st, h)
		}
		for c := 0; c < 255; c++ {
			if _, err := f.DrawChar(tg, st, 0, 0, byte(c), &col); err != nil {
				t.Errorf("style %d: %v", st, err)
			}
		}
	}
}

// TestGlyphA checks one glyph pixel by pixel: 'A' in the smallest style.
func TestGlyphA(t *testing.T) {
	f := gameFont(t)
	tg := Target{Pix: make([]byte, 8*8), Stride: 8, H: 8}
	col := Colors{'1', '2', '3', '4', '5', '6', '7', '8'}
	next, err := f.DrawChar(tg, 0, 0, 0, 'A', &col)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"121", "2.2", "222", "2.2", "2.2", "..."}
	for y, row := range want {
		for x := range row {
			got := tg.Pix[y*8+x]
			if got == 0 {
				got = '.'
			}
			if got != row[x] {
				t.Errorf("pixel (%d,%d) = %q, want %q", x, y, got, row[x])
			}
		}
	}
	if next != 4 {
		t.Errorf("next x = %d, want width 3 + spacing 1", next)
	}
	if w := f.Width(0, []byte("AA\x01A\x15A")); w != 12 {
		t.Errorf("Width = %d, want 12 (three A, control code free, stop at 0x15)", w)
	}
}

// TestNoZeroPixels backs drawChar's treatment of glyph value 0: the plain
// blitter would read outside the colour table for it, so it must not occur.
func TestNoZeroPixels(t *testing.T) {
	f := gameFont(t)
	for st := 0; st < Styles; st++ {
		s := &f.Styles[st]
		for c := 0; c < 255; c++ {
			p := int(s.Offsets[c])
			if p >= len(f.data) {
				continue
			}
			for row := 0; row < s.Height; row++ {
				for ; f.data[p] != 0x80; p++ {
					if f.data[p] == 0 {
						t.Fatalf("style %d glyph %d has a zero pixel", st, c)
					}
				}
				p++
			}
		}
	}
}

func pixels(tg Target) map[[2]int]byte {
	m := map[[2]int]byte{}
	for i, v := range tg.Pix {
		if v != 0 {
			m[[2]int{i % tg.Stride, i / tg.Stride}] = v
		}
	}
	return m
}

func TestPrinterControlCodes(t *testing.T) {
	f := gameFont(t)
	p := &Printer{Font: f}
	p.SetStyle(0, Solid(10))
	p.Highlight = Solid(20)
	tg := Target{Pix: make([]byte, 64*32), Stride: 64, H: 32}
	// "A", highlight "A", newline, tab to column 20, "A", stop before "A".
	end, err := p.String(tg, 2, 2, []byte("A\x02A\rB\x19\x14A\x15A"), true, 0)
	if err != nil {
		t.Fatal(err)
	}
	px := pixels(tg)
	if px[[2]int{2, 2}] != 10 || px[[2]int{6, 2}] != 20 {
		t.Errorf("colour switch: got %d and %d", px[[2]int{2, 2}], px[[2]int{6, 2}])
	}
	if _, ok := px[[2]int{2, 2 + p.LineFeed()}]; !ok {
		t.Errorf("no glyph at the start of the second line")
	}
	if end != 2+20+4 {
		t.Errorf("tab then one glyph should end at %d, got %d", 2+20+4, end)
	}
	if p.Current != Solid(20) {
		t.Errorf("highlight should persist across the line")
	}
}

func TestPrinterJustify(t *testing.T) {
	f := gameFont(t)
	p := &Printer{Font: f}
	p.SetStyle(0, Solid(1))
	tg := Target{Pix: make([]byte, 128*16), Stride: 128, H: 16}
	s := []byte("A A A")
	w := p.Width(s)
	end, err := p.String(tg, 0, 0, s, true, w+5)
	if err != nil {
		t.Fatal(err)
	}
	// Five extra pixels over two spaces: 2 each plus 1 for the first.
	if end != w+5 {
		t.Errorf("justified line ends at %d, want %d", end, w+5)
	}
}

func TestPrinterOutlinePasses(t *testing.T) {
	f := gameFont(t)
	p := &Printer{Font: f}
	p.SetStyleShadow(0, Solid(5), Outline)
	p.SetOutlineColor(9, 8)
	tg := Target{Pix: make([]byte, 32*16), Stride: 32, H: 16}
	if err := p.Print(tg, 4, 4, []byte("I")); err != nil {
		t.Fatal(err)
	}
	px := pixels(tg)
	// The outline surrounds the text on all eight sides of each pixel.
	for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {-1, -1}, {1, -1}} {
		q := [2]int{4 + d[0], 4 + d[1]}
		if v := px[q]; v != 9 && v != 8 {
			t.Errorf("no outline at offset %v: %d", d, v)
		}
	}
	if px[[2]int{4, 4}] != 5 {
		t.Errorf("text pixel = %d, want 5", px[[2]int{4, 4}])
	}
	if p.Current != Solid(5) {
		t.Errorf("colours not restored after the outline passes")
	}
}
