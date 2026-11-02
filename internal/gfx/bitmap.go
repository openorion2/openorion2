package gfx

import (
	"encoding/binary"
	"fmt"
)

// Bitmap operations of WLIBS\animate\bitmap.c and the assembly pixel loops
// they call, plus two small MOX helpers. Spec: docs/formats/bitmaps.md.

// bitmapPix is where Create_Bitmap_Header puts the single frame of a bitmap;
// several routines address pixels at this offset regardless of the frame table.
const bitmapPix = 0x10

// NewBlankBitmap is Create_Blank_Bitmap (0x12d8f5): a one-frame kind-1
// header from Create_Bitmap_Header (0x12d943) and w*h zero pixels.
func NewBlankBitmap(w, h int) *Picture { return NewBitmap(w, h, nil) }

// BitmapSize is Get_Bitmap_Size (0x12daa4): the bytes a w*h bitmap takes,
// header included, rounded up to a multiple of four.
func BitmapSize(w, h int) int {
	return (int(int16(h))*int(int16(w)) + 0x13) / 4 * 4
}

// Pixels returns frame 0 of a kind-1 picture, width*height bytes, for direct
// reading and writing. It is nil for other kinds or a truncated frame.
func (p *Picture) Pixels() []byte {
	if p.Kind() != KindBitmap || len(p.offsets) < 2 {
		return nil
	}
	a, n := int(p.offsets[0]), p.Width*p.Height
	if a+n > len(p.data) {
		return nil
	}
	return p.data[a : a+n : a+n]
}

// raw returns the picture's bytes from off on, or nil past the end.
func (p *Picture) raw(off int) []byte {
	if off < 0 || off > len(p.data) {
		return nil
	}
	return p.data[off:]
}

// curFrame returns the bytes from the current frame's offset to the end of
// the picture, as the original's frame_offsets[current_frame] pointer sees them.
func (p *Picture) curFrame() []byte {
	if p.CurrentFrame < 0 || p.CurrentFrame >= len(p.offsets) {
		return nil
	}
	return p.raw(int(p.offsets[p.CurrentFrame]))
}

// FrameData returns the raw bytes at the current frame's offset, as code
// that reads a frame as plain pixels sees them.
func (p *Picture) FrameData() []byte { return p.curFrame() }

// DrawPictureToBitmap is Draw_Picture_To_Bitmap (0x12f578). dst is overwritten
// in place with a blank bitmap of src's size, then: a kind-1 src copies its
// current frame; any other kind renders as sprite frames from
// (current/mfs)*mfs (0 when mfs is 0) through current_frame, copying every
// run byte, zeros included. src == dst leaves a blank bitmap, as in the original.
func (dst *Picture) DrawPictureToBitmap(src *Picture) {
	w, h := src.Width, src.Height
	*dst = *NewBlankBitmap(w, h)
	pix := dst.data[bitmapPix:]
	if src.Kind() == KindBitmap {
		copy(pix, clip(src.curFrame(), w*h))
		return
	}
	start := 0
	if len(src.data) > 9 && src.data[9] != 0 {
		mfs := int(src.data[9])
		start = src.CurrentFrame / mfs * mfs
	}
	put := func(o int, v byte) {
		if o >= 0 && o < len(pix) {
			pix[o] = v
		}
	}
	for n := start; n <= src.CurrentFrame && n < src.Frames; n++ {
		f := src.raw(int(src.offsets[n]))
		if len(f) < 4 {
			continue
		}
		y := int(le16(f, 2))
		line := y * w
		pos := line
		for wp := 2; y < h && 2*wp+4 <= len(f); {
			count, skip := int(int16(le16(f, 2*wp))), int(int16(le16(f, 2*wp+2)))
			wp += 2
			if count == 0 {
				y += skip
				line += w * skip
				pos = line
				continue
			}
			pos += skip
			for b, end := 2*wp, pos+count; pos < end; pos, b = pos+1, b+1 {
				if b < len(f) {
					put(pos, f[b])
				}
			}
			wp += (count + 1) / 2
		}
	}
}

// AddPicture is Add_Picture_To_Bitmap (0x12f7e6): draws src's current frame
// into the kind-1 dst with its corner at (x, y), clipped to dst. A kind-1 src
// skips colour 0; any other src is read as a sprite whose run bytes are all
// copied. dst pixels are addressed at +0x10. A non-bitmap dst is the
// original's fatal "target must be a bitmap" exit, returned as an error.
func (dst *Picture) AddPicture(x, y int, src *Picture) error {
	if dst.Kind() != KindBitmap {
		return fmt.Errorf("Add_Picture_To_Bitmap: target must be a bitmap")
	}
	dw, dh := dst.Width, dst.Height
	sw := src.Width
	x2 := int(int16(x + sw - 1))
	y2 := int(int16(y + src.Height - 1))
	if x >= dw || y >= dh || x2 < 0 || y2 < 0 {
		return nil
	}
	pix := dst.raw(bitmapPix)
	put := func(o int, v byte) {
		if o >= 0 && o < len(pix) {
			pix[o] = v
		}
	}
	f := src.curFrame()
	skipX, startX := 0, x
	if x < 0 {
		skipX, startX = -x, 0
	}
	skipY := 0
	if y < 0 {
		skipY, y = -y, 0
	}
	aw := dw - startX
	if x2 < dw-1 {
		aw = x2 - startX + 1
	}
	ah := dh - y
	if y2 < dh-1 {
		ah = y2 - y + 1
	}
	if src.Kind() == KindBitmap {
		bp := skipX + sw*skipY
		sp := startX + y*dw
		for r := 0; r < ah; r++ {
			for k := 0; k < aw; k++ {
				if bp < len(f) && f[bp] != 0 {
					put(sp, f[bp])
				}
				bp++
				sp++
			}
			bp += sw - aw
			sp += dw - aw
		}
		return nil
	}
	if len(f) < 4 {
		return nil
	}
	word := func(wp int) (int, bool) {
		if 2*wp+2 > len(f) {
			return 0, false
		}
		return int(le16(f, 2*wp)), true
	}
	row := int(le16(f, 2))
	line := dw*y + x
	wp := 2
	for row < skipY {
		c, ok := word(wp)
		s, ok2 := word(wp + 1)
		if !ok || !ok2 {
			return nil
		}
		if int16(c) == 0 {
			row += s
		} else {
			wp += (int(int16(c)) + 1) / 2
		}
		wp += 2
	}
	if skipY < row {
		line += (row - skipY) * dw
	}
	row -= skipY
	lineStart, lineEnd := line+skipX, line+skipX+aw
	pos := line
	for row < ah {
		c, ok := word(wp)
		s, ok2 := word(wp + 1)
		if !ok || !ok2 {
			return nil
		}
		count, skip := int(int16(c)), int(int16(s))
		wp += 2
		if count == 0 {
			row += skip
			pos = line + skip*dw
			lineStart, lineEnd = pos+skipX, pos+skipX+aw
			line = pos
			continue
		}
		pos += skip
		b, end := 2*wp, pos+count
		if pos < lineEnd {
			if pos < lineStart {
				add := min(lineStart-pos, count)
				b += add
				pos += add
			}
			n := lineEnd - pos
			if end <= pos+n {
				n = end - pos
			}
			for stop := pos + n; pos < stop; pos, b = pos+1, b+1 {
				if b < len(f) {
					put(pos, f[b])
				}
			}
		}
		wp += (count + 1) / 2
	}
	return nil
}

// Outline is Outline_Bitmap(dst, src, color, ignoreGlass) (0x12e64f). Unless
// dst is src, dst first becomes a one-frame copy of src's current frame. Then
// Outline_Bitmap_Pixels_ (0x147b89) or, with ignoreGlass,
// Outline_Bitmap_Pixels_No_Glass_ (0x147c6a) paint color on the empty pixels
// next to solid ones, scanning src's current frame. Both scan width*width
// pixels (mul eax), so the height is never used.
func (dst *Picture) Outline(src *Picture, color byte, ignoreGlass bool) {
	w := src.Width
	sp := src.curFrame()
	tp := sp
	if src != dst {
		h := src.Height
		frame := clip(sp, w*h)
		*dst = *NewBlankBitmap(w, h)
		copy(dst.data[bitmapPix:], frame)
		tp = dst.data[bitmapPix:]
	}
	outlinePixels(sp, tp, w, color, ignoreGlass)
}

// outlinePixels is the assembly loop: a horizontal pass over rows, then a
// vertical pass over columns, each over w*w pixels. A pixel is solid when it
// is neither 0 nor color (nor >= 0xf0 with noGlass). Entering a solid run
// paints the empty pixel before it, leaving one paints the empty pixel itself;
// the state resets at each row or column start.
func outlinePixels(src, dst []byte, w int, color byte, noGlass bool) {
	n := w * w
	if w <= 0 {
		return
	}
	get := func(i int) byte {
		if i >= 0 && i < len(src) {
			return src[i]
		}
		return 0
	}
	put := func(i int) {
		if i >= 0 && i < len(dst) {
			dst[i] = color
		}
	}
	solid := func(v byte) bool { return v != 0 && v != color && !(noGlass && v >= 0xf0) }
	// Horizontal: prev is the pixel to the left (offset -1).
	inSolid, inEmpty := false, false
	for i := 0; i < n; i++ {
		if solid(get(i)) {
			if inEmpty {
				put(i - 1)
			}
			inEmpty, inSolid = false, true
		} else {
			if inSolid {
				put(i)
			}
			inSolid, inEmpty = false, true
		}
		if (i+1)%w == 0 {
			inSolid, inEmpty = false, false
		}
	}
	// Vertical: columns of w rows; prev is the pixel above (offset -w).
	inSolid, inEmpty = false, false
	for c := 0; c < w; c++ {
		inSolid, inEmpty = false, false
		for r := 0; r < w; r++ {
			i := r*w + c
			if solid(get(i)) {
				if inEmpty {
					put(i - w)
				}
				inEmpty, inSolid = false, true
			} else {
				if inSolid {
					put(i)
				}
				inSolid, inEmpty = false, true
			}
		}
	}
}

// MaskOut is Mask_Out_Bitmap_With_Bitmap(x, y, target, mask, inverse)
// (0x12efbd): mask's current frame laid over p's current frame at (x, y)
// clears the target pixels under nonzero mask pixels (Mask_Bitmap_Pixels_
// 0x147f3e) or, with inverse, under zero ones (Mask_Bitmap_Pixels_Inverse_
// 0x147fdf). Quirk: the bottom clip tests y + mask WIDTH < height before
// using y + mask height. An empty or negative area, which loops ~2^32 times
// in the original, does nothing here.
func (p *Picture) MaskOut(x, y int, mask *Picture, inverse bool) {
	mw, mh := mask.Width, mask.Height
	tw, th := p.Width, p.Height
	mo := 0
	switch {
	case x < 0 && y < 0:
		mo = -y*mw - x
	case x < 0:
		mo = -x
	case y < 0:
		mo = -y * mw
	}
	if th < y || tw < x {
		return
	}
	to := 0
	switch {
	case x < 0 && y < 0:
	case x >= 0 && y >= 0:
		to = tw*y + x
	case x >= 0:
		to = x
	default:
		to = tw * y
	}
	maskAdd := 0
	if x < 0 {
		maskAdd = -x
	}
	if x+mw > tw {
		maskAdd += x + mw - tw
	}
	targetAdd := max(x, 0)
	if x+mw < tw {
		targetAdd += tw - x - mw
	}
	x1, x2 := max(x, 0), tw
	if x+mw < tw {
		x2 = x + mw
	}
	y1, y2 := max(y, 0), th
	if y+mw < th {
		y2 = y + mh
	}
	w, h := x2-x1, y2-y1
	if w <= 0 || h <= 0 {
		return
	}
	mf, tf := mask.curFrame(), p.curFrame()
	for r := 0; r < h; r++ {
		for k := 0; k < w; k++ {
			if mo >= 0 && mo < len(mf) && to >= 0 && to < len(tf) && (mf[mo] != 0) != inverse {
				tf[to] = 0
			}
			mo++
			to++
		}
		mo += maskAdd
		to += targetAdd
	}
}

// ReplaceColor is Replace_Color (0x13719f): in place, in every frame, pixels
// of colour from become to. Sprites (kind 0) change their run bytes;
// backgrounds (kind 2) the pixel dwords of each bank. Those two compare the
// byte zero-extended against from as a 16-bit value, so from outside 0..255
// never matches there; bitmaps (kind 1) compare the low byte. Kind 3 is the
// original's No_Replace_Mask_Msg error.
func (p *Picture) ReplaceColor(from, to int) error {
	from16 := int(int16(from))
	for n := 0; n < p.Frames; n++ {
		f := p.raw(int(p.offsets[n]))
		switch p.Kind() {
		case KindSprite:
			if len(f) < 4 {
				continue
			}
			y := int(le16(f, 2))
			for wp := 2; y < p.Height && 2*wp+4 <= len(f); {
				count := int(int16(le16(f, 2*wp)))
				skip := int(int16(le16(f, 2*wp+2)))
				wp += 2
				if count == 0 {
					y += skip
					continue
				}
				for k, b := 0, 2*wp; k < count && b < len(f); k, b = k+1, b+1 {
					if int(f[b]) == from16 {
						f[b] = byte(to)
					}
				}
				wp += (count + 1) / 2
			}
		case KindBitmap:
			for i := 0; i < p.Width*p.Height && i < len(f); i++ {
				if f[i] == byte(from) {
					f[i] = byte(to)
				}
			}
		case KindBackground:
			pos := 0
			for bank := 0; bank < 5; bank++ {
				for pos+6 <= len(f) {
					skip, count := le16(f, pos), le16(f, pos+4)
					pos += 8
					if skip == 0 && count == 0 {
						break
					}
					m := int(int16(count << 2))
					for i := 0; i < m && pos+i < len(f); i++ {
						if int(f[pos+i]) == from16 {
							f[pos+i] = byte(to)
						}
					}
					pos += m
				}
			}
		default:
			return fmt.Errorf("Replace_Color: refresh masks cannot be recoloured")
		}
	}
	return nil
}

// ActualSize is Get_Bitmap_Actual_Size (0x131528): the bounding box of the
// nonzero pixels at +0x10. x1 and y1 start at 1000 and the maxima at 0, so a
// blank bitmap yields (1000, 1000, -999, -999).
func (p *Picture) ActualSize() (x1, y1, w, h int) {
	pix := p.raw(bitmapPix)
	x1, y1 = 1000, 1000
	x2, y2 := 0, 0
	for y, o := 0, 0; y < p.Height; y++ {
		for x := 0; x < p.Width; x, o = x+1, o+1 {
			if o < len(pix) && pix[o] != 0 {
				x1, y1 = min(x1, x), min(y1, y)
				x2, y2 = max(x2, x), max(y2, y)
			}
		}
	}
	return x1, y1, int(int16(x2 - x1 + 1)), int(int16(y2 - y1 + 1))
}

// SwapColors is the MOX Swap_Bitmap_Colors_ (0x8fe88): for y1 <= y < y2 and
// x1 <= x < x2 of the pixels at +0x10, the first k with pixel == from[k]
// sets the pixel to to[k]; len(from) is the count n.
func (p *Picture) SwapColors(x1, y1, x2, y2 int, from, to []byte) {
	pix := p.raw(bitmapPix)
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			o := x + y*p.Width
			if o < 0 || o >= len(pix) {
				continue
			}
			for k := range from {
				if pix[o] == from[k] {
					pix[o] = to[k]
					break
				}
			}
		}
	}
}

// TintColors is the MOX Tint_Bitmap_Colors_ (0x8ff08): over y1 <= y < y2,
// x1 <= x < x2 (the whole bitmap when whole is set) every nonzero pixel at
// +0x10 has value added, wrapping at 256.
func (p *Picture) TintColors(x1, y1, x2, y2 int, whole bool, value byte) {
	if whole {
		x1, y1, x2, y2 = 0, 0, p.Width, p.Height
	}
	pix := p.raw(bitmapPix)
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			if o := x + y*p.Width; o >= 0 && o < len(pix) && pix[o] != 0 {
				pix[o] += value
			}
		}
	}
}

func le16(b []byte, i int) uint16 { return binary.LittleEndian.Uint16(b[i:]) }

// clip returns at most n bytes of b.
func clip(b []byte, n int) []byte {
	if n < 0 {
		n = 0
	}
	if len(b) > n {
		return b[:n]
	}
	return b
}

// CopyBitmapToBitmap is Copy_Bitmap_To_Bitmap (0x12e0b2): dst becomes a
// bitmap of src's size holding the w*h bytes at src's current frame.
func (dst *Picture) CopyBitmapToBitmap(src *Picture) {
	w, h := int(int16(src.Width)), int(int16(src.Height))
	pix := clip(src.curFrame(), max(w*h, 0))
	*dst = *NewBitmap(src.Width, src.Height, pix)
}

// ReduceBitmap is Scale_Bitmap (0x12e374) for scales up to 100 in both
// directions: dst gets src's current frame shrunk to sx and sy percent by
// Reduce_Bitmap_ (0x147a12), which steps through the source in 8.8 fixed
// point. A size below one pixel gives a 1x1 header.
func (dst *Picture) ReduceBitmap(src *Picture, sx, sy int) error {
	w, h := int(int16(src.Width)), int(int16(src.Height))
	nw, nh := w*sx/100, sy*h/100
	if nw < 1 || nh < 1 {
		*dst = *NewBitmap(1, 1, nil)
		return nil
	}
	if sx > 100 || sy > 100 {
		return fmt.Errorf("Enlarge_Bitmap_ is not ported")
	}
	in := src.curFrame()
	out := NewBitmap(nw, nh, nil)
	pix := out.data[bitmapPix:]
	xstep := uint32(uint16(0x6400 / uint16(sx)))
	ystep := uint32(uint16(0x6400 / uint16(sy)))
	si, di := 0, 0
	yacc := uint32(0)
	for r := 0; r < nh; r++ {
		p, acc := si, uint32(0)
		for c := 0; c < nw; c++ {
			if p < len(in) {
				pix[di] = in[p]
			}
			di++
			acc += xstep
			p += int(acc >> 8)
			acc &= 0xff
		}
		yacc += ystep
		// mul ch: the low byte of the width times the whole rows stepped.
		si += int(uint32(w)&^0xffff | (uint32(w)&0xff)*(yacc>>8&0xff)&0xffff)
		yacc &^= 0xff00
	}
	*dst = *out
	return nil
}

// JumblePixels is Jumble_Bitmap_Pixels_ (0x147e9f): pixels of src swap with
// a neighbour up to rng+1 away, alternately left and right, where the
// 256-byte table from index start is below percent. toggle is the routine's
// static side flag (0x189010).
func JumblePixels(src, dst []byte, w, h, rng, percent int, table []byte, start int, toggle *uint32) {
	n := w*h - 2*rng - 2
	si, di := rng+1, rng+1
	idx := start & 0xff
	at := func(b []byte, i int) byte {
		if i >= 0 && i < len(b) {
			return b[i]
		}
		return 0
	}
	for ; n > 0; n-- {
		al := at(src, si)
		si++
		if d := table[idx]; d < byte(percent) {
			off := int(d&byte(rng)) + 1
			if *toggle != 0 {
				off = -off
			}
			if di < len(dst) && di >= 0 {
				dst[di] = at(src, si+off-1)
			}
			if j := di + off; j >= 0 && j < len(dst) {
				dst[j] = al
			}
			*toggle = ^*toggle
		}
		idx = (idx + 1) & 0xff
		di++
	}
}

// MaskOutColors is Mask_Out_Colors (0x12f2a2): pixels of the current frame
// in lo..hi become 0, over width*height rounded down to whole dwords
// (Mask_Bitmap_Colors_Pixels_ 0x14801c).
func (p *Picture) MaskOutColors(lo, hi byte) {
	f := p.curFrame()
	n := min(int(int16(p.Width))*int(int16(p.Height))/4*4, len(f))
	for i := 0; i < n; i++ {
		if f[i] >= lo && f[i] <= hi {
			f[i] = 0
		}
	}
}

// ReplaceColorAll is Replace_Color_All_ (0x13743e) for bitmaps: every
// nonzero pixel of every frame becomes color.
func (p *Picture) ReplaceColorAll(color byte) {
	if p.Kind() != KindBitmap {
		return
	}
	n := int(int16(p.Width)) * int(int16(p.Height))
	for i := 0; i < p.Frames && i+1 < len(p.offsets); i++ {
		f := clip(p.raw(int(p.offsets[i])), n)
		for k, v := range f {
			if v != 0 {
				f[k] = color
			}
		}
	}
}

// SetGlassed is Set_Picture_Glassed_ (0x12dafc).
func (p *Picture) SetGlassed() {
	p.Mask |= MaskGlass
	if len(p.data) > 11 {
		p.data[11] |= MaskGlass
	}
}

// ReplaceColorRange is Replace_Color_Range_ (0x137d86): in every frame, a
// nonzero pixel in first..first+n-1 becomes table[pixel-first]. Sprites
// (kind 0) change the bytes of their runs, kind 2 those of its packets.
func (p *Picture) ReplaceColorRange(table []byte, first, n int) {
	last := first + n - 1
	repl := func(b []byte) {
		for k, v := range b {
			if v != 0 && int(v) >= first && int(v) <= last {
				b[k] = table[int(v)-first]
			}
		}
	}
	size := int(int16(p.Width)) * int(int16(p.Height))
	for i := 0; i < p.Frames && i+1 < len(p.offsets); i++ {
		f := p.raw(int(p.offsets[i]))
		switch p.Kind() {
		case KindBitmap:
			repl(clip(f, size))
		case 0:
			if len(f) < 4 {
				continue
			}
			y := int(le16(f, 2))
			for wp := 2; y < int(int16(p.Height)) && 2*wp+4 <= len(f); {
				count := int(int16(le16(f, 2*wp)))
				skip := int(int16(le16(f, 2*wp+2)))
				wp += 2
				if count == 0 {
					y += skip
					continue
				}
				repl(clip(f[min(2*wp, len(f)):], count))
				wp += (count + 1) / 2
			}
		case 2:
			pos := 0
			for k := 0; k < 5; k++ {
				for pos+6 <= len(f) {
					a, b := int16(le16(f, pos)), int16(le16(f, pos+4))
					pos += 8
					if a == 0 && b == 0 {
						break
					}
					n := int(int16(b << 2))
					repl(clip(f[min(pos, len(f)):], n))
					pos += n
				}
			}
		}
	}
}

// FlipBitmap is Flip_Bitmap (0x12dfb2): directions 0 and 1 mirror the rows
// left to right, 2 and 3 turn the picture upside down; dst gets src's size.
func (dst *Picture) FlipBitmap(src *Picture, dir int) {
	w, h := int(int16(src.Width)), int(int16(src.Height))
	in := append([]byte(nil), clip(src.curFrame(), w*h)...)
	if dst != src {
		*dst = *NewBlankBitmap(w, h)
	}
	out := clip(dst.raw(bitmapPix), w*h)
	if len(in) < w*h || len(out) < w*h {
		return
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if dir == 0 || dir == 1 {
				out[y*w+w-1-x] = in[y*w+x]
			} else {
				out[(h-1-y)*w+x] = in[y*w+x]
			}
		}
	}
}
