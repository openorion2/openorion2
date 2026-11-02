package font

import (
	"errors"
	"math/rand"
	"testing"
)

func ascii() []byte {
	var s []byte
	for c := byte(0x20); c < 0x7f; c++ {
		s = append(s, c)
	}
	return s
}

// TestSpacing: Set_Font_Spacing changes width and drawing, Set_Font_Style
// resets it to the style's own.
func TestSpacing(t *testing.T) {
	f := gameFont(t)
	p := &Printer{Font: f}
	p.SetStyle(2, Solid(1))
	s := []byte("AB C")
	w := p.Width(s)
	if p.Spacing != f.Styles[2].HSpacing {
		t.Fatalf("spacing %d after SetStyle, want %d", p.Spacing, f.Styles[2].HSpacing)
	}
	p.SetSpacing(p.Spacing + 3)
	if got := p.Width(s); got != w+3*len(s) {
		t.Errorf("width with spacing+3 = %d, want %d", got, w+3*len(s))
	}
	tg := Target{Pix: make([]byte, 128*16), Stride: 128, H: 16}
	end, err := p.String(tg, 0, 0, s, true, 0)
	if err != nil || end != w+3*len(s) {
		t.Errorf("String ends at %d (%v), want %d", end, err, w+3*len(s))
	}
	p.SetSpaceWidth(10)
	if got := p.Width([]byte(" ")); got != 10+p.Spacing {
		t.Errorf("space width %d", got)
	}
	p.SetStyle(2, Solid(1))
	if p.Width(s) != w {
		t.Errorf("SetStyle did not reset spacing and widths")
	}
}

// TestClippedMatchesWindow: with windows at least as tall as the glyphs,
// clipped printing equals unclipped printing cut to the window, shadow
// passes included.
func TestClippedMatchesWindow(t *testing.T) {
	f := gameFont(t)
	rng := rand.New(rand.NewSource(1))
	const W, H = 160, 64
	text := ascii()
	for st := 0; st < 5; st++ {
		for i := 0; i < 200; i++ {
			sh := Shadow(rng.Intn(6))
			x1, y1 := rng.Intn(W/2), rng.Intn(H/2)
			minH := f.Styles[st].Height + 2
			w := Window{x1, y1, x1 + rng.Intn(W-x1), min(H-1, y1+minH+rng.Intn(H))}
			if w.Y2-w.Y1+1 < minH {
				continue
			}
			x, y := rng.Intn(W)-W/2, rng.Intn(H)-f.Styles[st].Height
			s := text[rng.Intn(len(text)/2):]
			s = s[:rng.Intn(len(s))]

			a := Target{Pix: make([]byte, W*H), Stride: W, H: H}
			b := Target{Pix: make([]byte, W*H), Stride: W, H: H}
			p := &Printer{Font: f}
			p.SetStyleShadow(st, Colors{1, 2, 3, 4, 5, 6, 7, 8}, sh)
			p.SetOutlineColor(9, 10)
			if err := p.Print(a, x, y, s); err != nil {
				t.Fatal(err)
			}
			p.Clip = &w
			if err := p.Print(b, x, y, s); err != nil {
				t.Fatal(err)
			}
			for py := 0; py < H; py++ {
				for px := 0; px < W; px++ {
					want := a.Pix[py*W+px]
					if px < w.X1 || px > w.X2 || py < w.Y1 || py > w.Y2 {
						want = 0
					}
					if got := b.Pix[py*W+px]; got != want {
						t.Fatalf("style %d shadow %d window %v at (%d,%d) %q: pixel (%d,%d) = %d, want %d",
							st, sh, w, x, y, s, px, py, got, want)
					}
				}
			}
		}
	}
}

// TestClippedQuirks pins the clipped printer's departures from the plain one.
func TestClippedQuirks(t *testing.T) {
	f := gameFont(t)
	p := &Printer{Font: f}
	p.SetStyle(1, Solid(1))
	tg := Target{Pix: make([]byte, 64*32), Stride: 64, H: 32}
	w := Window{0, 0, 63, 31}

	// Codes above 0x80 are neither drawn nor advanced over.
	if end, _ := p.ClippedString(tg, w, 5, 5, []byte{0x90, 0x90}, true, 0); end != 5 {
		t.Errorf("0x90 advanced to %d", end)
	}
	// A right-cut glyph returns x2 + 1 + spacing.
	cw := int(f.Styles[1].Widths['M'])
	end, _ := p.ClippedString(tg, Window{0, 0, 10 + cw/2, 31}, 10, 5, []byte("M"), true, 0)
	if want := 10 + cw/2 + 1 + p.Spacing; end != want {
		t.Errorf("right-cut glyph returns %d, want %d", end, want)
	}
	// A first line wholly outside prints nothing and returns 1.
	if end, _ := p.ClippedString(tg, Window{0, 10, 63, 20}, 0, 21, []byte("A"), true, 0); end != 1 {
		t.Errorf("rejected string returns %d", end)
	}

	// Cut at both top and bottom: sy rows too few.
	h := f.Styles[1].Height
	a := Target{Pix: make([]byte, 64*32), Stride: 64, H: 32}
	p.Clip = &Window{0, 11, 63, 11 + h - 4}
	if err := p.Print(a, 4, 10, []byte("I")); err != nil {
		t.Fatal(err)
	}
	b := Target{Pix: make([]byte, 64*32), Stride: 64, H: 32}
	p.Clip = nil
	p.Print(b, 4, 10, []byte("I"))
	for row := 11; row <= 11+h-4; row++ {
		drawn, full := false, false
		for x := 0; x < 64; x++ {
			drawn = drawn || a.Pix[row*64+x] != 0
			full = full || b.Pix[row*64+x] != 0
		}
		if lost := row > 11+h-4-1; full && drawn == lost {
			t.Errorf("row %d: drawn %v, expected the last row to be lost", row, drawn)
		}
	}

	// Style 5 from 'W' on lies past 0x7fff and is unreachable when cut.
	p.SetStyle(5, Solid(1))
	p.Clip = &Window{0, 0, 63, 10}
	if err := p.Print(Target{Pix: make([]byte, 64*32), Stride: 64, H: 32}, 0, 0, []byte("X")); !errors.Is(err, ErrGlyphOffset) {
		t.Errorf("style 5 'X' cut: err %v", err)
	}
	p.Clip = nil
	if err := p.Print(Target{Pix: make([]byte, 64*32), Stride: 64, H: 32}, 0, 0, []byte("X")); err != nil {
		t.Errorf("style 5 'X' unclipped: %v", err)
	}
}

// TestToBitmap: printing into a bitmap equals printing on a page cut to the
// bitmap for one line; line breaks end the string.
func TestToBitmap(t *testing.T) {
	f := gameFont(t)
	rng := rand.New(rand.NewSource(2))
	text := ascii()
	for st := 0; st < 5; st++ {
		for i := 0; i < 200; i++ {
			bw, bh := 8+rng.Intn(80), f.Styles[st].Height+rng.Intn(20)
			sh := Shadow(rng.Intn(6))
			x, y := rng.Intn(bw+20)-20, rng.Intn(bh)-f.Styles[st].Height/2
			s := text[rng.Intn(len(text)/2):]
			s = s[:rng.Intn(len(s))]
			const W, H = 256, 128
			const ox, oy = 64, 32
			page := Target{Pix: make([]byte, W*H), Stride: W, H: H}
			bm := Target{Pix: make([]byte, bw*bh), Stride: bw, H: bh}
			p := &Printer{Font: f}
			p.SetStyleShadow(st, Colors{1, 2, 3, 4, 5, 6, 7, 8}, sh)
			p.SetOutlineColor(9, 10)
			if err := p.Print(page, ox+x, oy+y, s); err != nil {
				t.Fatal(err)
			}
			if err := p.ToBitmap(bm, x, y, s); err != nil {
				t.Fatal(err)
			}
			for py := 0; py < bh; py++ {
				for px := 0; px < bw; px++ {
					if got, want := bm.Pix[py*bw+px], page.Pix[(oy+py)*W+ox+px]; got != want {
						t.Fatalf("style %d shadow %d %dx%d at (%d,%d) %q: (%d,%d) = %d, want %d",
							st, sh, bw, bh, x, y, s, px, py, got, want)
					}
				}
			}
		}
	}
	p := &Printer{Font: f}
	p.SetStyle(1, Solid(1))
	bm := Target{Pix: make([]byte, 64*32), Stride: 64, H: 32}
	p.ToBitmap(bm, 0, 0, []byte("\rA"))
	for _, v := range bm.Pix {
		if v != 0 {
			t.Fatal("0x0d should end a bitmap string")
		}
	}
}
