package gfx

import "fmt"

// Glassed drawing (graphics.c, animate.c): colours 0xf0..0xff of a glassed
// picture are not drawn but remap the pixel below through one of the 16
// glass tables. Spec: docs/formats/pictures.md, "Glass".

// GlassFirst is the first colour drawn through the glass tables.
const GlassFirst = 0xf0

// DrawGlassAt draws frame i at (x, y) unclipped, as Draw_Glassed_Animated_
// Sprite_ (0x12aca4) and Draw_Glassed_Bitmap_Sprite_ (0x12aec0) do, or,
// with noGlass, as their Draw_No_Glassed_ twins (0x12aaa1, 0x12abcc), which
// leave the glass colours out. The sprite blitters copy colour 0 too.
func (p *Picture) DrawGlassAt(page []byte, stride, x, y, i int, remap *[16][256]byte, noGlass bool) error {
	put := func(row, col int, v byte) {
		o := (y+row)*stride + x + col
		if o < 0 || o >= len(page) {
			return
		}
		switch {
		case v < GlassFirst:
			page[o] = v
		case !noGlass:
			page[o] = remap[v-GlassFirst][page[o]]
		}
	}
	switch p.Kind() {
	case KindSprite, KindBitmap:
		return p.render(i, put)
	}
	return fmt.Errorf("glassed kind %d is not ported", p.Kind())
}

// DrawClippedGlass draws frame i at (x, y) through w as Clipped_Draw_Glassed_
// Animated_Sprite_ (0x148c33) and Clipped_Draw_Glassed_Bitmap_Sprite_
// (0x1487e0) do. Both skip colour 0. The sprite blitter tests the right
// edge with the picture's height; the bitmap one leaves out the window's
// last column and row.
func (p *Picture) DrawClippedGlass(page []byte, stride, x, y, i int, w Window, remap *[16][256]byte) error {
	f, err := p.Frame(i)
	if err != nil {
		return err
	}
	put := func(o int, v byte) {
		if v == 0 || o < 0 || o >= len(page) {
			return
		}
		if v < GlassFirst {
			page[o] = v
		} else {
			page[o] = remap[v-GlassFirst][page[o]]
		}
	}
	switch p.Kind() {
	case KindSprite:
		return p.clippedSprite(f, int(p.offsets[i]), page, stride, x, y, w, p.Height, put)
	case KindBitmap:
		if len(f) < p.Width*p.Height {
			return fmt.Errorf("bitmap frame %d: %d bytes for %dx%d", i, len(f), p.Width, p.Height)
		}
		cols := min(x+p.Width, w.X2) - max(x, w.X1)
		rows := min(y+p.Height, w.Y2) - max(y, w.Y1)
		src, dst := 0, y*stride+x
		if w.X1 > x {
			src += w.X1 - x
			dst += w.X1 - x
		}
		if w.Y1 > y {
			src += (w.Y1 - y) * p.Width
			dst += (w.Y1 - y) * stride
		}
		for r := 0; r < rows; r++ {
			for k := 0; k < cols; k++ {
				if src+k < len(f) {
					put(dst+k, f[src+k])
				}
			}
			src += p.Width
			dst += stride
		}
		return nil
	}
	return fmt.Errorf("glassed kind %d cannot be drawn clipped", p.Kind())
}
