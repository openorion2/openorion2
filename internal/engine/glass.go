package engine

import "github.com/openorion2/openorion2/internal/gfx"

// Glass remapping and tinting (palette\remap.c, graphics.c). Spec:
// docs/screens/main-screen-zoom.md, "Engine primitives".

// glassState holds glass_remap_colors and the sorted palette index.
type glassState struct {
	remap [16][256]byte
	reds  [256]byte // palette_reds; kept between calls (Sort_Palette_List reads a stale byte)
	list  [65]int   // palette_list: indices into reds
}

// glassColors are the 16 {percent, r, g, b} slots from FONTS.LBX entry 0.
func (e *Engine) glassColors() [16][4]byte {
	var g [16][4]byte
	src := e.text.Font.Glass
	for i := range g {
		copy(g[i][:], src[4*i:4*i+4])
	}
	return g
}

// sortPaletteList is Sort_Palette_List (0x13372a).
func (e *Engine) sortPaletteList() {
	gs, pal := &e.glass, &e.V.Palette
	for count := 0; count != 0xff; count++ {
		index := count
		for i := 0; i < count; i++ {
			if pal[count].R < pal[gs.reds[i]].R {
				index = i
				break
			}
		}
		for i := count; i >= index; i-- {
			gs.reds[i+1] = gs.reds[i]
		}
		gs.reds[index] = byte(count)
	}
	gs.list[0] = 0
	for r := 1; r < 64; r++ {
		found := -1
		for j := 0; j < 256; j++ {
			if int(pal[gs.reds[j]].R) == r {
				found = j
				break
			}
		}
		if found < 0 {
			found = gs.list[r-1]
		}
		gs.list[r] = found
	}
	gs.list[64] = 255
}

// findClosestColor is Find_Closest_Color (0x1338c9).
func (e *Engine) findClosestColor(r, g, b int) byte {
	gs, pal := &e.glass, &e.V.Palette
	minR, maxR, minG, maxG, minB, maxB := r, r, g, g, b, b
	for count := 1; count <= 25; count++ {
		down := func(v int) int {
			if v < 11 {
				return 0
			}
			return v - 10
		}
		up := func(v, lim int) int {
			if v < 0x35 {
				return v + 10
			}
			return lim
		}
		minR, maxR = down(minR), up(maxR, 0x40)
		minG, maxG = down(minG), up(maxG, 0x3f)
		minB, maxB = down(minB), up(maxB, 0x3f)
		best, color := 13000, -1
		n := int(int16(gs.list[maxR] - gs.list[minR]))
		for i := 0; i < n; i++ {
			c := gs.reds[(gs.list[minR]+i)&0xff]
			p := pal[c]
			if int(p.G) >= minG && int(p.G) <= maxG && int(p.B) >= minB && int(p.B) <= maxB {
				dr, dg, db := int(p.R)-r, int(p.G)-g, int(p.B)-b
				if d := dr*dr + dg*dg + db*db; d < best && c != 0 {
					best, color = d, int(c)
				}
			}
		}
		if color != -1 {
			return byte(color)
		}
	}
	return 0
}

// UpdateGlassRemapColors is Update_Glass_Remap_Colors (0x133237); slot 0
// becomes 50 % black whatever the font file says.
func (e *Engine) UpdateGlassRemapColors() {
	gc := e.glassColors()
	gc[0] = [4]byte{50, 0, 0, 0}
	e.sortPaletteList()
	pal := &e.V.Palette
	for i := 0; i < 256; i++ {
		for j := 0; j < 16; j++ {
			p := int(gc[j][0])
			switch p {
			case 0:
				e.glass.remap[j][i] = byte(i)
			case 100:
				e.glass.remap[j][i] = e.findClosestColor(int(gc[j][1]), int(gc[j][2]), int(gc[j][3]))
			default:
				mix := func(c, g byte) int { return int(byte((int(c)*(100-p) + int(g)*p) / 100)) }
				e.glass.remap[j][i] = e.findClosestColor(mix(pal[i].R, gc[j][1]), mix(pal[i].G, gc[j][2]), mix(pal[i].B, gc[j][3]))
			}
		}
	}
}

// TintFill is Tint_Fill (0x12991d): pixels pass through a glass table. With
// more than one 4-pixel column it always uses table 0 and leaves column x2
// alone.
func (v *Video) tintFill(gs *glassState, x1, y1, x2, y2, color int) {
	if x2 < x1 {
		x1, x2 = x2, x1
	}
	if y2 < y1 {
		y1, y2 = y2, y1
	}
	if v.Clipping {
		x1, y1 = max(x1, v.WinX1), max(y1, v.WinY1)
		x2, y2 = min(x2, v.WinX2), min(y2, v.WinY2)
	}
	if v.refresh >= 2 {
		v.AddSquare(x1, y1, x2, y2)
	}
	tint := func(table *[256]byte, xa, xb int) {
		for y := y1; y <= y2; y++ {
			for x := xa; x <= xb; x++ {
				if o := y*W + x; o >= 0 && o < Size {
					v.Current[o] = table[v.Current[o]]
				}
			}
		}
	}
	c1, c2 := x1/4, x2/4
	if c1 == c2 {
		tint(&gs.remap[color], x1, x2)
		return
	}
	t := &gs.remap[0]
	center := c2 - c1 - 1
	if x1&3 != 0 {
		tint(t, x1, 4*c1+3)
		tint(t, 4*c1+4, 4*c1+4+4*center-1)
	} else if center != 0 {
		tint(t, 4*c1, 4*c1+4*(center+1)-1)
	}
	if x2&3 != 0 {
		tint(t, 4*c2, 4*c2+(x2&3)-1)
	}
}

// TintFill is Tint_Fill.
func (e *Engine) TintFill(x1, y1, x2, y2, color int) { e.V.tintFill(&e.glass, x1, y1, x2, y2, color) }

// ScreenBitmapCapture is Screen_Bitmap_Capture (0x12e148): a block of the
// current page as a kind-1 picture.
func (e *Engine) ScreenBitmapCapture(x1, y1, x2, y2 int) *gfx.Picture {
	if x2 < x1 {
		x1, x2 = x2, x1
		y1, y2 = y2, y1
	}
	w, h := x2-x1+1, y2-y1+1
	pix := make([]byte, w*h)
	for r := 0; r < h; r++ {
		o := (y1+r)*W + x1
		if o >= 0 && o+w <= Size {
			copy(pix[r*w:], e.V.Current[o:o+w])
		}
	}
	return gfx.NewBitmap(w, h, pix)
}

// TintBitmap is Tint_Bitmap (0x12e8d4) with dst = src, a one-frame bitmap:
// every nonzero pixel goes through glass_remap_colors[n]
// (Tint_Bitmap_Pixels_ 0x147e23).
func (e *Engine) TintBitmap(p *gfx.Picture, n int) {
	t := &e.glass.remap[n]
	pix := p.Pixels()
	for i, c := range pix {
		if c != 0 {
			pix[i] = t[c]
		}
	}
}

// SetGlassRemapColor is Set_Glass_Remap_Color (0x131acb): glass slot i
// becomes a tint of percent pct towards r, g, b (the font's glass table).
func (e *Engine) SetGlassRemapColor(i, r, g, b, pct int) {
	gl := e.text.Font.Glass[4*i : 4*i+4]
	gl[0], gl[1], gl[2], gl[3] = byte(pct), byte(r), byte(g), byte(b)
}

// GlassBlock is a saved glass remap block: the tables and the slots.
type GlassBlock struct {
	remap  [16][256]byte
	colors [64]byte
}

// SaveGlassRemapBlock is Save_Glass_Remap_Block (0x13209c).
func (e *Engine) SaveGlassRemapBlock() *GlassBlock {
	b := &GlassBlock{remap: e.glass.remap}
	copy(b.colors[:], e.text.Font.Glass[:64])
	return b
}

// RestoreGlassRemapBlock is Restore_Glass_Remap_Block (0x1320e9).
func (e *Engine) RestoreGlassRemapBlock(b *GlassBlock) {
	e.glass.remap = b.remap
	copy(e.text.Font.Glass[:64], b.colors[:])
}

// SetPaletteGradient is Set_Palette_Gradient (0x1319e4): count colours from
// start shade from one RGB to the other, byte arithmetic as in the original.
func (e *Engine) SetPaletteGradient(start, count, r1, g1, b1, r2, g2, b2 int) {
	for i := 0; i < count; i++ {
		e.V.Palette[i+start] = Color{1, byte(r1 + (r2-r1)*i/(count-1)), byte(g1 + (g2-g1)*i/(count-1)), byte(b1 + (b2-b1)*i/(count-1))}
	}
}

// GlassRemap is the glass table entry for glass colour c over pixel p.
func (e *Engine) GlassRemap(c, p byte) byte { return e.glass.remap[c-0xf0][p] }
