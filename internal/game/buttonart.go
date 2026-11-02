package game

import (
	"bytes"
	"image"
	"image/png"

	"github.com/openorion2/openorion2/assets"
	"github.com/openorion2/openorion2/internal/engine"
)

// port-only: the port's own button art (assets/lobby/button.png at the
// repo's root, cut by tools/buttoncut from a generated sheet): per state a row 24 px high of
// the left cap, an 8 px middle tile and the right cap. A button of any
// width is the caps with the tile repeated between them, its colours the
// nearest of the palette on screen.

// Button states, the art's rows.
const (
	buttonNormal = iota
	buttonHover
	buttonPressed
	buttonDisabled
)

const (
	buttonArtH    = 24
	buttonArtCap  = 24
	buttonArtTile = 8
)

// buttonArt is the art mapped to a palette: an index per pixel, -1 clear.
type buttonArt struct {
	pal [256]engine.Color // the palette it was mapped to
	pix [][]int16         // [y][x] over the whole art
}

var buttonArtImage = func() *image.NRGBA {
	img, err := png.Decode(bytes.NewReader(assets.LobbyButton))
	if err != nil {
		panic("button art: " + err.Error())
	}
	n, ok := img.(*image.NRGBA)
	if !ok {
		panic("button art: not NRGBA")
	}
	return n
}()

// buttonArtFor maps the art to the palette now in e.V.Palette, once per
// palette.
func (g *Game) buttonArtFor() *buttonArt {
	pal := g.e.V.Palette
	if a := g.buttonArt; a != nil && a.pal == pal {
		return a
	}
	img := buttonArtImage
	b := img.Bounds()
	a := &buttonArt{pal: pal, pix: make([][]int16, b.Dy())}
	near := map[[3]uint8]int16{}
	for y := range b.Dy() {
		a.pix[y] = make([]int16, b.Dx())
		for x := range b.Dx() {
			c := img.NRGBAAt(x, y)
			if c.A < 128 {
				a.pix[y][x] = -1
				continue
			}
			k := [3]uint8{c.R >> 2, c.G >> 2, c.B >> 2} // the palette's 6 bits
			i, ok := near[k]
			if !ok {
				i = nearestColor(&pal, k)
				near[k] = i
			}
			a.pix[y][x] = i
		}
	}
	g.buttonArt = a
	return a
}

// nearestColor is the palette entry closest to c (6-bit RGB), entry 0
// (the screen's black, often a transparent key) left out.
func nearestColor(pal *[256]engine.Color, c [3]uint8) int16 {
	best, bestD := int16(1), 1<<30
	for i := 1; i < 256; i++ {
		p := pal[i]
		dr, dg, db := int(p.R)-int(c[0]), int(p.G)-int(c[1]), int(p.B)-int(c[2])
		if d := 3*dr*dr + 4*dg*dg + 2*db*db; d < bestD {
			best, bestD = int16(i), d
		}
	}
	return best
}

// drawButtonArt draws a button w px wide (at least the two caps) in state
// at (x, y) on the current page.
func (g *Game) drawButtonArt(x, y, w, state int) {
	a := g.buttonArtFor()
	v := g.e.V
	w = max(w, 2*buttonArtCap)
	for yy := range buttonArtH {
		row := a.pix[state*buttonArtH+yy]
		sy := y + yy
		if sy < 0 || sy >= engine.H {
			continue
		}
		for xx := range w {
			var sx int
			switch {
			case xx < buttonArtCap:
				sx = xx
			case xx >= w-buttonArtCap:
				sx = 2*buttonArtCap + buttonArtTile - (w - xx)
			default:
				sx = buttonArtCap + (xx-buttonArtCap)%buttonArtTile
			}
			if i := row[sx]; i >= 0 && x+xx >= 0 && x+xx < engine.W {
				v.Current[sy*engine.W+x+xx] = byte(i)
			}
		}
	}
	if v.RefreshMode() > engine.RefreshMasked {
		v.AddSquare(x, y, x+w-1, y+buttonArtH-1)
	}
}
