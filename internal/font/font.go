// Package font reads the game's bitmap fonts (FONTS.LBX entry 0) and draws
// text the way WLIBS\fonts\fonts.c does.
//
// Spec: docs/formats/fonts.md.
package font

import (
	"encoding/binary"
	"fmt"
)

// Styles is how many font styles the file holds.
const Styles = 6

// Layout of the font entry, as Load_Font_File copies it.
const (
	glassSize  = 0x40   // glass remap colours, copied to glass_colors
	headerSize = 0x235c // struct s_font_header, copied to font_header
	dataStart  = glassSize + headerSize

	offBaseHeight = 0x52c // short[8]
	offHSpacing   = 0x53c // short[8]
	offVSpacing   = 0x54c // short[8]
	offWidths     = 0x55c // char[6][256]
	offOffsets    = 0xb5c // unsigned int[6][256], relative to the glyph data
)

// Style is one font: metrics and where each glyph's data starts.
type Style struct {
	Height   int // rows drawn per glyph (base_height)
	HSpacing int // pixels added after every glyph
	VSpacing int // pixels added between lines
	Widths   [256]byte
	Offsets  [256]uint32
}

// LineFeed is the distance between lines, as Set_Font computes it.
func (s *Style) LineFeed() int { return s.Height + s.VSpacing }

// Font holds every style and the glyph data they share.
type Font struct {
	Glass  [glassSize]byte
	Styles [Styles]Style
	data   []byte
}

// Parse reads a font entry.
func Parse(entry []byte) (*Font, error) {
	if len(entry) < dataStart {
		return nil, fmt.Errorf("font entry has %d bytes, the header alone needs %d", len(entry), dataStart)
	}
	le := binary.LittleEndian
	f := &Font{data: entry[dataStart:]}
	copy(f.Glass[:], entry)
	h := entry[glassSize:dataStart]
	for i := range f.Styles {
		s := &f.Styles[i]
		s.Height = int(int16(le.Uint16(h[offBaseHeight+2*i:])))
		s.HSpacing = int(int16(le.Uint16(h[offHSpacing+2*i:])))
		s.VSpacing = int(int16(le.Uint16(h[offVSpacing+2*i:])))
		copy(s.Widths[:], h[offWidths+256*i:])
		for c := range s.Offsets {
			s.Offsets[c] = le.Uint32(h[offOffsets+1024*i+4*c:])
		}
	}
	return f, nil
}

// Colors are the eight shades a glyph is drawn with; glyph pixel value n uses
// Colors[n-1]. Set_Font fills all eight with one palette index.
type Colors [8]byte

// Solid returns colours that draw every pixel with palette index c.
func Solid(c byte) Colors { return Colors{c, c, c, c, c, c, c, c} }

// Target is an 8-bit page to draw on.
type Target struct {
	Pix    []byte
	Stride int
	H      int
}

// DrawChar draws glyph c of a style at (x, y) and returns the x of the next
// glyph, as Print_Character does. Characters Set_Font did not copy (255),
// glyphs whose data would start past the end of the font (the zero-width 126
// and 127 of style 5) and pixels off the page are skipped.
func (f *Font) DrawChar(t Target, style, x, y int, c byte, col *Colors) (int, error) {
	return f.drawChar(t, style, x, y, c, col, false)
}

// drawChar is Print_Character_ASM with Set_Font's tables: code 255 is not
// copied there, so it is skipped.
func (f *Font) drawChar(t Target, style, x, y int, c byte, col *Colors, noAlias bool) (int, error) {
	s := &f.Styles[style]
	next := x + int(s.Widths[c]) + s.HSpacing
	if c == 255 {
		return next, nil
	}
	return next, f.glyph(t, style, x, y, c, col, noAlias)
}

// glyph is Print_Character_ASM; with noAlias it is the _No_Alias variant
// the shadow passes use, which leaves glyph value 0 transparent. The plain
// one would draw the byte in memory before the colour table for it; no
// shipped glyph contains a 0 (TestNoZeroPixels), so both skip it. Glyphs
// whose data would start past the end of the font are skipped.
func (f *Font) glyph(t Target, style, x, y int, c byte, col *Colors, noAlias bool) error {
	s := &f.Styles[style]
	p := int(s.Offsets[c])
	if p >= len(f.data) {
		return nil
	}
	for row := 0; row < s.Height; row++ {
		dx := x
		for {
			if p >= len(f.data) {
				return fmt.Errorf("glyph %d of style %d runs past the font data", c, style)
			}
			b := f.data[p]
			p++
			switch {
			case b == 0x80: // end of row
			case b > 0x80: // transparent run
				dx += int(b & 0x7f)
				continue
			default:
				if b > 0 && y+row >= 0 && y+row < t.H && dx >= 0 && dx < t.Stride {
					t.Pix[(y+row)*t.Stride+dx] = col[b-1]
				}
				dx++
				continue
			}
			break
		}
	}
	return nil
}

// Width returns the width of a string in a style, as Get_String_Width
// measures it: control characters below 0x20 take no space, and the string
// ends at a NUL or at 0x15.
func (f *Font) Width(style int, s []byte) int {
	st := &f.Styles[style]
	w := 0
	for _, c := range s {
		if c == 0 || c == 0x15 {
			break
		}
		if c >= 0x20 {
			w += int(st.Widths[c]) + st.HSpacing
		}
	}
	return w
}

// Draw draws a plain string from (x, y) and returns the x after it. Control
// characters are skipped; their effects belong to the higher-level printers.
func (f *Font) Draw(t Target, style, x, y int, s []byte, col *Colors) (int, error) {
	for _, c := range s {
		if c == 0 || c == 0x15 {
			break
		}
		if c < 0x20 {
			continue
		}
		var err error
		if x, err = f.DrawChar(t, style, x, y, c, col); err != nil {
			return x, err
		}
	}
	return x, nil
}
