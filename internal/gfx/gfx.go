// Package gfx decodes the engine's picture format, which every sprite, bitmap
// and full-screen background in the LBX archives uses, and its palettes.
//
// The layout follows WLIBS\animate (animate.c, bitmap.c) and the assembly
// blitters it calls. A picture starts with the animation header
//
//	int16  width, height       (ddx, ddy)
//	int16  current_frame
//	int16  total_frames
//	uint8  repeat_frame
//	uint8  multi_frame_full_store
//	uint8  group_count
//	uint8  mask
//	uint32 frame_offsets[total_frames+1]   relative to the header
//
// followed, when mask has MaskPalette set, by a palette chunk: int16 start,
// int16 count and count four-byte colours. The low two bits of mask select how
// frames are encoded; see Kind.
//
// Spec: docs/formats/pictures.md.
package gfx

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// Kind is how a picture's frames are encoded, the low two bits of mask.
type Kind byte

const (
	// KindSprite frames are run-length encoded with transparent gaps.
	KindSprite Kind = 0
	// KindBitmap frames are width*height raw pixels; colour 0 is transparent.
	KindBitmap Kind = 1
	// KindBackground frames cover the whole 640x480 screen as runs of dwords.
	KindBackground Kind = 2
)

// Mask bits above the kind.
const (
	MaskGlass   = 0x08 // drawn translucently through the glass remap table
	MaskPalette = 0x10 // a palette chunk follows the frame offsets
	MaskHold    = 0x20 // after repeat_frame, play to the end and hold
)

// Screen size the engine draws to.
const (
	ScreenW = 640
	ScreenH = 480
)

const headerSize = 12

// Picture is a parsed picture: header, frame table and optional palette.
type Picture struct {
	Width, Height int
	CurrentFrame  int
	Frames        int
	RepeatFrame   int
	FullStore     bool
	Groups        int
	Mask          byte
	Palette       *PaletteChunk
	offsets       []uint32
	data          []byte
}

// Kind returns how the frames are encoded.
func (p *Picture) Kind() Kind { return Kind(p.Mask & 3) }

// PaletteChunk replaces Count colours starting at Start when the picture is drawn.
type PaletteChunk struct {
	Start  int
	Colors []color.RGBA
	Raw    []byte // the colours as stored: changed flag and 6-bit RGB each
}

// Parse parses a picture from an LBX entry.
func Parse(data []byte) (*Picture, error) {
	if len(data) < headerSize+4 {
		return nil, fmt.Errorf("picture too short")
	}
	le := binary.LittleEndian
	p := &Picture{
		Width:        int(int16(le.Uint16(data[0:]))),
		Height:       int(int16(le.Uint16(data[2:]))),
		CurrentFrame: int(int16(le.Uint16(data[4:]))),
		Frames:       int(int16(le.Uint16(data[6:]))),
		RepeatFrame:  int(data[8]),
		FullStore:    data[9] != 0,
		Groups:       int(data[10]),
		Mask:         data[11],
		data:         data,
	}
	if p.Width < 0 || p.Height < 0 || p.Frames <= 0 || p.Frames > 509 {
		return nil, fmt.Errorf("implausible header: %dx%d, %d frames", p.Width, p.Height, p.Frames)
	}
	if p.Kind() == 3 {
		return nil, fmt.Errorf("refresh masks cannot be drawn")
	}
	end := headerSize + 4*(p.Frames+1)
	if len(data) < end {
		return nil, fmt.Errorf("frame table truncated")
	}
	p.offsets = make([]uint32, p.Frames+1)
	for i := range p.offsets {
		p.offsets[i] = le.Uint32(data[headerSize+4*i:])
		if p.offsets[i] > uint32(len(data)) {
			return nil, fmt.Errorf("frame %d offset 0x%x past the end", i, p.offsets[i])
		}
	}
	if p.Mask&MaskPalette != 0 {
		if len(data) < end+4 {
			return nil, fmt.Errorf("palette chunk truncated")
		}
		start := int(int16(le.Uint16(data[end:])))
		count := int(int16(le.Uint16(data[end+2:])))
		if start < 0 || count < 0 || start+count > 256 || len(data) < end+4+4*count {
			return nil, fmt.Errorf("bad palette chunk: %d colours from %d", count, start)
		}
		p.Palette = &PaletteChunk{Start: start, Colors: decodeColors(data[end+4:], count), Raw: data[end+4 : end+4+4*count]}
	}
	return p, nil
}

// Frame returns the encoded bytes of frame i.
func (p *Picture) Frame(i int) ([]byte, error) {
	if i < 0 || i >= p.Frames {
		return nil, fmt.Errorf("frame %d out of range [0,%d)", i, p.Frames)
	}
	a, b := p.offsets[i], p.offsets[i+1]
	if a > b {
		return nil, fmt.Errorf("frame %d ends before it starts", i)
	}
	return p.data[a:b], nil
}

// Canvas is an 8-bit indexed image the size of a picture, or of the screen for
// backgrounds. Opaque marks pixels any frame has drawn so far.
type Canvas struct {
	W, H   int
	Pix    []byte
	Opaque []bool
}

// NewCanvas returns a blank canvas for the picture.
func (p *Picture) NewCanvas() *Canvas {
	w, h := p.Width, p.Height
	if p.Kind() == KindBackground {
		w, h = ScreenW, ScreenH
	}
	return &Canvas{W: w, H: h, Pix: make([]byte, w*h), Opaque: make([]bool, w*h)}
}

func (c *Canvas) set(i int, v byte) {
	if i >= 0 && i < len(c.Pix) {
		c.Pix[i] = v
		c.Opaque[i] = true
	}
}

// Draw draws frame i over the canvas the way the engine's unclipped blitters
// do, leaving transparent pixels untouched.
func (p *Picture) Draw(c *Canvas, i int) error {
	return p.render(i, func(row, x int, v byte) {
		if x < c.W && row < c.H {
			c.set(row*c.W+x, v)
		}
	})
}

// DrawAt draws frame i onto an 8-bit page of the given stride with its top
// left corner at (x, y), as the unclipped blitters write into the video page:
// rows and columns are not clipped against the picture or the page, only the
// final address against the buffer, so a pixel past the right edge lands on
// the next row as it would in the original. Backgrounds ignore x and y.
func (p *Picture) DrawAt(page []byte, stride, x, y, i int) error {
	put := func(row, col int, v byte) {
		if o := (y+row)*stride + x + col; o >= 0 && o < len(page) {
			page[o] = v
		}
	}
	if p.Kind() == KindBackground {
		put = func(row, col int, v byte) {
			if o := row*stride + col; o >= 0 && o < len(page) {
				page[o] = v
			}
		}
	}
	return p.render(i, put)
}

// render decodes frame i, calling put for every opaque pixel with its row and
// column relative to the picture (to the screen for backgrounds).
func (p *Picture) render(i int, put func(row, x int, v byte)) error {
	f, err := p.Frame(i)
	if err != nil {
		return err
	}
	switch p.Kind() {
	case KindSprite:
		return p.renderSprite(f, int(p.offsets[i]), put)
	case KindBitmap:
		if len(f) < p.Width*p.Height {
			return fmt.Errorf("bitmap frame %d: %d bytes for %dx%d", i, len(f), p.Width, p.Height)
		}
		for k, v := range f[:p.Width*p.Height] {
			if v != 0 {
				put(k/p.Width, k%p.Width, v)
			}
		}
		return nil
	case KindBackground:
		return renderBackground(f, put)
	}
	return fmt.Errorf("kind %d cannot be drawn", p.Kind())
}

// renderSprite follows Draw_Animated_Sprite: after a uint16 the blitter
// ignores and the number of blank rows on top, the frame is a list of (count,
// skip) pairs. A zero count moves skip rows down to the start of a row;
// otherwise skip pixels are passed over and count pixels copied, after which
// the source is realigned to an even address. base is the frame's offset in
// the picture, which the engine keeps at an even address.
func (p *Picture) renderSprite(f []byte, base int, put func(row, x int, v byte)) error {
	le := binary.LittleEndian
	if len(f) < 4 {
		return fmt.Errorf("sprite frame too short")
	}
	top := int(le.Uint16(f[2:]))
	row := top
	x := 0
	pos := 4
	for remaining := p.Height - top; remaining > 0; {
		if pos+4 > len(f) {
			return fmt.Errorf("sprite frame runs out at row %d", row)
		}
		count, skip := int(le.Uint16(f[pos:])), int(le.Uint16(f[pos+2:]))
		pos += 4
		if count == 0 {
			remaining -= skip
			row += skip
			x = 0
			continue
		}
		x += skip
		if pos+count > len(f) {
			return fmt.Errorf("sprite run of %d overruns the frame", count)
		}
		for k := 0; k < count; k++ {
			put(row, x, f[pos+k])
			x++
		}
		pos += count
		if (base+pos)&1 != 0 {
			pos++
		}
	}
	return nil
}

// renderBackground follows Draw_Screen_Background_Off: the screen is five
// banks of 16384 dwords, each a list of (skip, count) dword pairs followed by
// count dwords of pixels, closed by (0, 0).
func renderBackground(f []byte, put func(row, x int, v byte)) error {
	le := binary.LittleEndian
	dw := func(i int) (uint32, error) {
		if 4*i+4 > len(f) {
			return 0, fmt.Errorf("background frame runs out")
		}
		return le.Uint32(f[4*i:]), nil
	}
	const total = ScreenW * ScreenH / 4
	pos := 0
	for bank := 0; bank < 5; bank++ {
		screen := bank << 14
		for {
			skip, err := dw(pos)
			if err != nil {
				return err
			}
			count, err := dw(pos + 1)
			if err != nil {
				return err
			}
			pos += 2
			if skip == 0 && count == 0 {
				break
			}
			screen += int(skip)
			for end := pos + int(count); pos < end && screen < total; pos, screen = pos+1, screen+1 {
				v, err := dw(pos)
				if err != nil {
					return err
				}
				for k := 0; k < 4; k++ {
					px := 4*screen + k
					put(px/ScreenW, px%ScreenW, byte(v>>(8*k)))
				}
			}
		}
	}
	return nil
}

// Palette is a full 256-colour palette.
type Palette [256]color.RGBA

// ParsePalette decodes a palette as Load_Palette reads it from FONTS.LBX: 256
// entries of a changed flag followed by 6-bit red, green and blue.
func ParsePalette(data []byte) (*Palette, error) {
	if len(data) < 256*4 {
		return nil, fmt.Errorf("palette needs 1024 bytes, have %d", len(data))
	}
	var p Palette
	copy(p[:], decodeColors(data, 256))
	return &p, nil
}

// Apply returns a copy of the palette with the chunk's colours replaced.
func (p *Palette) Apply(ch *PaletteChunk) *Palette {
	q := *p
	if ch != nil {
		copy(q[ch.Start:], ch.Colors)
	}
	return &q
}

func decodeColors(b []byte, n int) []color.RGBA {
	out := make([]color.RGBA, n)
	for i := range out {
		out[i] = color.RGBA{vga(b[4*i+1]), vga(b[4*i+2]), vga(b[4*i+3]), 0xff}
	}
	return out
}

// vga widens a 6-bit DAC value to 8 bits.
func vga(v byte) byte {
	v &= 0x3f
	return v<<2 | v>>4
}

// Image renders the canvas through a palette; pixels never drawn come out
// transparent.
func (c *Canvas) Image(pal *Palette) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, c.W, c.H))
	for i, v := range c.Pix {
		if !c.Opaque[i] {
			continue
		}
		col := pal[v]
		img.Pix[4*i], img.Pix[4*i+1], img.Pix[4*i+2], img.Pix[4*i+3] = col.R, col.G, col.B, 0xff
	}
	return img
}

// Window is a clipping rectangle, inclusive on all sides.
type Window struct{ X1, Y1, X2, Y2 int }

// DrawClipped draws frame i at (x, y) through the window w, the way
// Clipped_Draw_Animated_Sprite and Clipped_Draw_Bitmap_Sprite do. Both skip
// colour 0 (Color_Stream_Copy_). The sprite blitter keeps the original's
// edges: its right limit is the window only when the picture reaches past
// x2 + 1, and a run starting exactly on that limit is dropped.
func (p *Picture) DrawClipped(page []byte, stride, x, y, i int, w Window) error {
	f, err := p.Frame(i)
	if err != nil {
		return err
	}
	put := func(o int, v byte) {
		if v != 0 && o >= 0 && o < len(page) {
			page[o] = v
		}
	}
	switch p.Kind() {
	case KindSprite:
		return p.clippedSprite(f, int(p.offsets[i]), page, stride, x, y, w, p.Width, put)
	case KindBitmap:
		if len(f) < p.Width*p.Height {
			return fmt.Errorf("bitmap frame %d: %d bytes for %dx%d", i, len(f), p.Width, p.Height)
		}
		x2 := min(w.X2, p.Width+x-1)
		x1 := max(x, w.X1)
		y2 := min(w.Y2, p.Height+y-1)
		y1 := max(y, w.Y1)
		src, dst := 0, x+y*stride
		if x < w.X1 {
			src += w.X1 - x
			dst += w.X1 - x
		}
		if y < w.Y1 {
			src += p.Width * (w.Y1 - y)
			dst += (w.Y1 - y) * stride
		}
		for r := 0; r < y2-y1+1; r++ {
			for k := 0; k < x2-x1+1; k++ {
				if src+k < len(f) {
					put(dst+k, f[src+k])
				}
			}
			src += p.Width
			dst += stride
		}
		return nil
	}
	return fmt.Errorf("kind %d cannot be drawn clipped", p.Kind())
}

// limit is the size the right edge is tested with: the width, or the
// height in the glassed blitter.
func (p *Picture) clippedSprite(f []byte, base int, page []byte, stride, x1, y1 int, w Window, limit int, put func(int, byte)) error {
	le := binary.LittleEndian
	if len(f) < 4 {
		return fmt.Errorf("sprite frame too short")
	}
	cx2, cy2 := ScreenW-1, ScreenH-1
	cx1 := max(x1, w.X1)
	cy1 := max(y1, w.Y1)
	if w.X2 < limit+x1 {
		cx2 = w.X2
	}
	if w.Y2 < p.Height+y1 {
		cy2 = w.Y2
	}
	y := y1 + int(int16(le.Uint16(f[2:])))
	line := x1 + y*stride
	dst := line
	x := x1
	bottom := min(y1+p.Height, cy2)
	pos := 4
	for y <= bottom {
		if pos+4 > len(f) {
			return fmt.Errorf("sprite frame runs out at row %d", y)
		}
		data, skip := int(int16(le.Uint16(f[pos:]))), int(int16(le.Uint16(f[pos+2:])))
		pos += 4
		switch {
		case data == 0:
			dst = line + skip*stride
			y += skip
			x = x1
			line = dst
		case y < cy1 || cx2 <= x:
			pos += data
		default:
			dst += skip
			x += skip
			if x < cx1 {
				pos += cx1 - x
				dst += cx1 - x
				data -= cx1 - x
				x = cx1
			}
			n := 0
			if cx2 < x+data {
				if cx2 != x && cx2-x >= 0 {
					n = cx2 - x + 1
				}
			} else if data > 0 {
				n = data
			}
			for k := 0; k < n; k++ {
				if pos+k < len(f) {
					put(dst+k, f[pos+k])
				}
			}
			x += data
			pos += data
			dst += data
		}
		if (base+pos)&1 != 0 {
			pos++
		}
	}
	return nil
}

// NewBitmap builds a one-frame kind-1 picture from w*h pixels, as
// Create_Bitmap_Header followed by a pixel copy does (pixels at +0x10).
func NewBitmap(w, h int, pix []byte) *Picture {
	data := make([]byte, 0x10+w*h)
	le := binary.LittleEndian
	le.PutUint16(data[0:], uint16(w))
	le.PutUint16(data[2:], uint16(h))
	le.PutUint16(data[6:], 1)
	data[11] = byte(KindBitmap)
	le.PutUint32(data[12:], 0x10)
	copy(data[0x10:], pix)
	return &Picture{Width: w, Height: h, Frames: 1, Mask: byte(KindBitmap),
		offsets: []uint32{0x10, uint32(len(data))}, data: data}
}

// Render decodes frame i, calling put for every pixel it carries with its row
// and column relative to the picture (to the screen for backgrounds).
func (p *Picture) Render(i int, put func(row, x int, v byte)) error { return p.render(i, put) }

// ShiftPaletteRange is Shift_Palette_Range (0x1381b5): the picture's palette
// chunk is drawn from start on (the original exits past colour 256).
func (p *Picture) ShiftPaletteRange(start int) {
	if p.Palette != nil {
		p.Palette.Start = start
	}
}
