package game

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/openorion2/openorion2/assets"
	"github.com/openorion2/openorion2/internal/engine"
)

// port-only: the lobby's Unicode text (chat, game descriptions, player
// names) in its own TrueType font (assets/lobby/font, a 12 px pixel font
// covering Latin, Cyrillic, Greek, Chinese, Japanese and Korean), not the
// game's bitmap fonts, which have ASCII only. Glyphs are drawn as masks in
// one palette colour; a glyph is rasterized once.

const (
	uniSize   = 12 // the font's pixel size
	uniLineH  = 14 // a line of text in the lobby
	uniAscent = 11 // the baseline below a line's top
)

type uniGlyph struct {
	x0, y0, w, h int // the mask's place relative to the pen (y0 from the baseline)
	mask         []uint8
	adv          int
}

type uniFont struct {
	mu     sync.Mutex
	face   font.Face
	glyphs map[rune]*uniGlyph
}

var uniFace = sync.OnceValue(func() *uniFont {
	f, err := opentype.Parse(assets.LobbyFont)
	if err != nil {
		panic("lobby font: " + err.Error())
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: uniSize, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic("lobby font: " + err.Error())
	}
	return &uniFont{face: face, glyphs: map[rune]*uniGlyph{}}
})

// glyph is r's glyph; a rune the font lacks shows as its '?'.
func (u *uniFont) glyph(r rune) *uniGlyph {
	u.mu.Lock()
	defer u.mu.Unlock()
	if g := u.glyphs[r]; g != nil {
		return g
	}
	dr, mask, mp, adv, ok := u.face.Glyph(fixed.P(0, 0), r)
	if !ok && r != '?' {
		u.mu.Unlock()
		g := u.glyph('?')
		u.mu.Lock()
		u.glyphs[r] = g
		return g
	}
	g := &uniGlyph{x0: dr.Min.X, y0: dr.Min.Y, w: dr.Dx(), h: dr.Dy(), adv: adv.Round()}
	g.mask = make([]uint8, g.w*g.h)
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			_, _, _, al := mask.At(mp.X+x, mp.Y+y).RGBA()
			g.mask[y*g.w+x] = uint8(al >> 8)
		}
	}
	u.glyphs[r] = g
	return g
}

// uniWidth is s's width in pixels.
func uniWidth(s string) int {
	u := uniFace()
	w := 0
	for _, r := range s {
		w += u.glyph(r).adv
	}
	return w
}

// uniPrint draws s with its line's top at y from x, in colour c, cut at
// x2; it returns the pen's x after the last glyph drawn.
func (g *Game) uniPrint(x, y, x2 int, s string, c byte) int {
	return g.uniPrintStyle(x, y, x2, s, c, false)
}

// uniPrintBold is uniPrint in a made-up bold (the font has none): each
// glyph drawn again a pixel to the right, and a pixel wider.
func (g *Game) uniPrintBold(x, y, x2 int, s string, c byte) int {
	return g.uniPrintStyle(x, y, x2, s, c, true)
}

// uniWidthBold is s's width in uniPrintBold.
func uniWidthBold(s string) int { return uniWidth(s) + utf8.RuneCountInString(s) }

func (g *Game) uniPrintStyle(x, y, x2 int, s string, c byte, bold bool) int {
	u, v := uniFace(), g.e.V
	extra := 0
	if bold {
		extra = 1
	}
	base := y + uniAscent
	for _, r := range s {
		gl := u.glyph(r)
		if x+gl.adv+extra > x2+1 {
			break
		}
		for yy := 0; yy < gl.h; yy++ {
			sy := base + gl.y0 + yy
			if sy < 0 || sy >= engine.H {
				continue
			}
			for xx := 0; xx < gl.w; xx++ {
				if gl.mask[yy*gl.w+xx] < 0x80 {
					continue
				}
				for d := 0; d <= extra; d++ {
					if sx := x + gl.x0 + xx + d; sx >= 0 && sx < engine.W {
						v.Current[sy*engine.W+sx] = c
					}
				}
			}
		}
		x += gl.adv + extra
	}
	if v.RefreshMode() > engine.RefreshMasked {
		v.AddSquare(0, y, engine.W-1, min(y+uniLineH, engine.H-1))
	}
	return x
}

// uniWrap splits s into lines at most w pixels wide: at spaces when it
// can, anywhere in a run without them (CJK text has none).
func uniWrap(s string, w int) []string {
	var out []string
	for s != "" {
		width, cut, lastSpace := 0, len(s), -1
		for i, r := range s {
			if r == ' ' {
				lastSpace = i
			}
			width += uniFace().glyph(r).adv
			if width > w && i > 0 {
				cut = i
				if lastSpace > 0 {
					cut = lastSpace
				}
				break
			}
		}
		out = append(out, s[:cut])
		s = strings.TrimLeftFunc(s[cut:], unicode.IsSpace)
	}
	return out
}
