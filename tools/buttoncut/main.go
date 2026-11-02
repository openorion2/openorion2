// Command buttoncut cuts the port's button pieces out of a sheet of whole
// metal buttons generated in the style of the original's interface (four
// rows: normal, hover, pressed, disabled, drawn about 4x the game's size on
// a painted checkerboard) into assets/lobby/button.png: per state (row), at
// the game's size (24 px high), the left cap, an 8 px middle tile and the
// right cap.
//
//	go run ./tools/buttoncut SHEET.png assets/lobby/button.png
//
// The sheet used was 1774x887, its top four buttons at x 140..759, rows
// found by their dark outline.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

const (
	h      = 24 // a button's height in the game
	capW   = 24 // each cap, the end's light spot whole
	tileW  = 8  // the repeated middle
	x0, x1 = 140, 759
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: buttoncut SHEET.png OUT.png")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fail(err)
	}
	src, err := png.Decode(f)
	if err != nil {
		fail(err)
	}
	lum := func(x, y int) int {
		r, g, b, _ := src.At(x, y).RGBA()
		return int((r*299 + g*587 + b*114) / 1000 >> 8)
	}
	var rows [][2]int
	for y := 0; y < src.Bounds().Dy() && len(rows) < 4; {
		if lum((x0+x1)/2, y) < 90 {
			rows = append(rows, [2]int{y, y + 95})
			y += 100
			continue
		}
		y++
	}
	if len(rows) != 4 {
		fail(fmt.Errorf("found %d buttons, want 4", len(rows)))
	}
	w := 2*capW + tileW
	out := image.NewNRGBA(image.Rect(0, 0, w, 4*h))
	for s, r := range rows {
		scale := float64(r[1]-r[0]) / h
		for y := range h {
			for x := range w {
				// the button this piece is cut from is capW+tileW+capW wide:
				// the caps are its ends, the tile a run from its centre.
				var sx int
				switch {
				case x < capW:
					sx = x0 + int(float64(x)*scale)
				case x >= capW+tileW:
					sx = x1 - int(float64(w-x)*scale)
				default:
					sx = (x0+x1)/2 + int(float64(x-capW)*scale)
				}
				var rr, gg, bb, c uint32
				for yy := r[0] + int(float64(y)*scale); yy < r[0]+int(float64(y+1)*scale); yy++ {
					for xx := sx; xx < sx+int(scale); xx++ {
						a, b2, c2, _ := src.At(xx, yy).RGBA()
						rr, gg, bb, c = rr+a>>8, gg+b2>>8, bb+c2>>8, c+1
					}
				}
				// the chamfered corners stay transparent (the checkerboard)
				cx := x
				if x >= capW+tileW {
					cx = w - 1 - x
				} else if x >= capW {
					cx = h // the tile has no corner
				}
				if cx+min(y, h-1-y) < 3 {
					continue
				}
				out.Set(x, s*h+y, color.NRGBA{uint8(rr / c), uint8(gg / c), uint8(bb / c), 255})
			}
		}
	}
	o, err := os.Create(os.Args[2])
	if err != nil {
		fail(err)
	}
	if err := png.Encode(o, out); err != nil {
		fail(err)
	}
	if err := o.Close(); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "buttoncut:", err)
	os.Exit(1)
}
