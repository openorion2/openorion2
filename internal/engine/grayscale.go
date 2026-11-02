package engine

import "github.com/openorion2/openorion2/internal/gfx"

// Gray scaling of the off page (palette\gscale.c). Spec:
// docs/screens/build-queue.md, "Drawing".

// establishColorGradient is Establish_Color_Gradient_ (0x1380e0): each
// colour's brightness r+g+b/2 scaled to levels steps, reversed with invert.
func (e *Engine) establishColorGradient(levels, invert int16) [256]byte {
	var t [256]byte
	for i := range t {
		c := &e.V.Palette[i]
		lum := byte(int(c.R) + int(c.G) + int(c.B)/2)
		t[i] = byte(int32(int16(lum)) * int32(levels) / 0x9e)
		if invert == 1 {
			t[i] = byte(levels) - 1 - t[i]
		}
	}
	return t
}

// InterlacedGrayScaleOffPage is Interlaced_Gray_Scale_Off_Page_ (0x1382bf):
// the even rows of the off page become first+steps of a levels1-step
// brightness scale, the odd rows second+steps of a levels2 one
// (Interlace_Scale_Off_Page_ASM_ 0x1481c2 on 0xf0 row pairs).
func (e *Engine) InterlacedGrayScaleOffPage(first, levels1, second, levels2, invert int16) {
	for k, p := range [2][2]int16{{first, levels1}, {second, levels2}} {
		t := e.establishColorGradient(p[1], invert)
		for i := range t {
			t[i] += byte(p[0])
		}
		off := e.V.Off
		for row := k; row < 0xf0*2; row += 2 {
			line := off[row*W : row*W+W]
			for x, v := range line {
				line[x] = t[v]
			}
		}
	}
}

// GrayScaleBitmap is Gray_Scale_Bitmap_ (0x12f498) with Gray_Scale_Fill_
// Pixels_ (0x147e61): every pixel becomes table[its levels-step brightness].
func (e *Engine) GrayScaleBitmap(b *gfx.Picture, levels int16, table []byte, invert int16) {
	t := e.establishColorGradient(levels, invert)
	for i := range t {
		t[i] = table[t[i]]
	}
	pix := b.Pixels()
	for k, p := range pix {
		pix[k] = t[p]
	}
}
