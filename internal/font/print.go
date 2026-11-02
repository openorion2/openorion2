package font

import "fmt"

// Shadow is font_header.shadow_flag: how Print_Display surrounds text.
type Shadow int

// Values set by Set_Font_Style_Shadow_Down, _Up, _Heavy, _Outline and
// _Outline_Heavy.
const (
	ShadowNone Shadow = iota
	ShadowDown
	ShadowUp
	ShadowHeavy
	Outline
	OutlineHeavy
)

// Window is the clipping window (screen_window_x1..y2), inclusive.
type Window struct{ X1, Y1, X2, Y2 int }

// Printer holds the text state the engine keeps in font_header and the
// globals around it, and prints the way fonts.c does. Change the style only
// through SetStyle: the current metrics are copies taken there.
type Printer struct {
	Font  *Font
	Style int // current_font_style (Get_Font_Style, Get_Current_Font_Style)

	Current   Colors // current_colors: what glyphs are drawn with right now
	Normal    Colors // control code 0x01
	Highlight Colors // control codes 0x02 and 0x04
	Special   Colors // control code 0x03

	Shadow            Shadow
	OutlineColor      byte // outline_color, for glyph values 2..8
	AliasOutlineColor byte // alias_outline_color, for glyph value 1

	// Spacing is current_horizontal_spacing (Get_Font_Spacing, 0x8f645):
	// pixels after every glyph, reset to the style's by SetStyle.
	Spacing int

	// Clip, when set, makes Display print through Clipped_Print_String
	// (clipping_flag == 1) inside this window.
	Clip *Window

	noAlias bool // draw_alias_flag

	// Current copies Set_Font_Style takes from the style tables.
	loaded   bool
	widths   [256]byte // current_font_widths
	vspacing int       // current_vertical_spacing
	height   int       // font_header.height
	lineFeed int       // current_line_feed
}

// load is the metric part of Set_Font_Style.
func (p *Printer) load(style int) {
	s := &p.Font.Styles[style]
	p.Style = style
	p.widths = s.Widths
	p.Spacing, p.vspacing = s.HSpacing, s.VSpacing
	p.height = s.Height
	p.lineFeed = s.Height + s.VSpacing
	p.loaded = true
}

// cur makes a Printer that never saw SetStyle behave as if it had.
func (p *Printer) cur() {
	if !p.loaded {
		p.load(p.Style)
	}
}

// SetStyle is Set_Font_Style (0x120bb5): pick a style with a set of eight
// shades, reset the current metrics to the style's and clear any shadow.
func (p *Printer) SetStyle(style int, colors Colors) {
	p.load(style)
	p.Normal, p.Current = colors, colors
	p.Shadow = ShadowNone
}

// SetStyleShadow is Set_Font_Style followed by one of the shadow setters.
func (p *Printer) SetStyleShadow(style int, colors Colors, s Shadow) {
	p.SetStyle(style, colors)
	p.Shadow = s
}

// SetOutlineColor is Set_Outline_Color.
func (p *Printer) SetOutlineColor(c, alias byte) {
	p.OutlineColor, p.AliasOutlineColor = c, alias
}

// SetAliasColor is Set_Alias_Color (0x120f55): shade 0 of the current,
// normal, highlight and special colours.
func (p *Printer) SetAliasColor(c byte) {
	p.Current[0], p.Normal[0], p.Highlight[0], p.Special[0] = c, c, c, c
}

// SetSpacing is Set_Font_Spacing (0x120e8c): the pixels added after every
// glyph until the next SetStyle.
func (p *Printer) SetSpacing(n int) { p.cur(); p.Spacing = int(int16(n)) }

// SetSpaceWidth is Set_Font_Spacing_Width (0x120e60): the width of ' ' in
// the current copy of the widths.
func (p *Printer) SetSpaceWidth(n int) { p.cur(); p.widths[' '] = byte(n) }

// SpaceWidth is the current width of ' ' (0x1b3ec8).
func (p *Printer) SpaceWidth() int { p.cur(); return int(p.widths[' ']) }

// SetVSpacing sets current_vertical_spacing (0x1b3ea4) directly, as
// _Squeeze_Print_Paragraph_ does.
func (p *Printer) SetVSpacing(n int) { p.cur(); p.vspacing = int(int16(n)) }

// SetLF is Set_Font_LF (0x120e24). As in the original it sets the line feed
// to n and the height to base height + n.
func (p *Printer) SetLF(n int) {
	p.cur()
	p.lineFeed = int(int16(n))
	p.height = int(int16(p.Font.Styles[p.Style].Height + n))
}

// Height is Get_Font_Height (0x122259): font_header.height, which clipping
// and scan-list marking use; glyphs are drawn with the base height.
func (p *Printer) Height() int { p.cur(); return p.height }

// VSpacing is Get_Font_Vertical_Spacing (0x122286).
func (p *Printer) VSpacing() int { p.cur(); return p.vspacing }

// LineFeed is current_line_feed.
func (p *Printer) LineFeed() int { p.cur(); return p.lineFeed }

// Width is Get_String_Width (0x12066f) with the current widths and spacing,
// truncated to 16 bits as its callers read it.
func (p *Printer) Width(s []byte) int {
	p.cur()
	w := 0
	for _, c := range s {
		if c == 0 || c == 0x15 {
			break
		}
		if c >= 0x20 {
			w += int(p.widths[c]) + p.Spacing
		}
	}
	return int(int16(w))
}

// char is Print_Character / Print_Character_No_Alias: the glyph in full,
// then x + current width + current spacing.
func (p *Printer) char(t Target, x, y int, c byte) (int, error) {
	err := p.Font.glyph(t, p.Style, x, y, c, &p.Current, p.noAlias)
	return x + int(p.widths[c]) + p.Spacing, err
}

// layout is the loop Print_String and Clipped_Print_String share: control
// codes, line breaks, tabs and justification, with put printing a glyph.
func (p *Printer) layout(x, y int, s []byte, colorsOK bool, full int, put func(x, y int, c byte) (int, error)) (int, error) {
	p.cur()
	px, py := x, y
	var spaceAdd, spaceRem, spaceN int
	if full != 0 {
		spaces := 0
		for _, c := range s {
			if c == 0 || c == '\r' || c == 0x14 || c == 0x19 || c == 0x15 || c == 0x1d {
				break
			}
			if c == ' ' {
				spaces++
			}
		}
		if spaces == 0 {
			full = 0
		}
		full -= p.Width(s)
		if full < 1 {
			full = 0
		} else {
			spaceRem, spaceAdd = full%spaces, full/spaces
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == 0 || c == 0x15:
			return px, nil
		case c == 1:
			if colorsOK {
				p.Current = p.Normal
			}
			continue
		case c == 2 || c == 4:
			if colorsOK {
				p.Current = p.Highlight
			}
			continue
		case c == 3:
			if colorsOK {
				p.Current = p.Special
			}
			continue
		case c == 8:
			continue
		case c == '\r' || c == 0x14:
			py += p.lineFeed
			px = x
			continue
		case c == 0x19 || c == 0x1d:
			// Tab: the next byte is the column, counted from the start x.
			if i+1 < len(s) {
				px = x + int(s[i+1])
			}
			i++
			continue
		}
		var err error
		if px, err = put(px, py, c); err != nil {
			return px, err
		}
		if full != 0 && c == ' ' {
			px += spaceAdd
			if spaceN < spaceRem {
				px++
			}
			spaceN++
		}
	}
	return px, nil
}

// String is Print_String (0x121814): print s from (x, y) honouring control
// codes and return the x after it. colorsOK lets codes 0x01..0x04 switch
// colours. A positive full widens spaces so the line spans full pixels.
func (p *Printer) String(t Target, x, y int, s []byte, colorsOK bool, full int) (int, error) {
	return p.layout(x, y, s, colorsOK, full, func(x, y int, c byte) (int, error) {
		return p.char(t, x, y, c)
	})
}

// ClippedString is Clipped_Print_String (0x122309): String through the
// window w. A string whose first line lies wholly above or below the window
// prints nothing and returns 1, as the original does.
func (p *Printer) ClippedString(t Target, w Window, x, y int, s []byte, colorsOK bool, full int) (int, error) {
	p.cur()
	if y > w.Y2 || y+p.height < w.Y1 {
		return 1, nil
	}
	return p.layout(x, y, s, colorsOK, full, func(x, y int, c byte) (int, error) {
		return p.clippedChar(t, w, x, y, c)
	})
}

// clippedChar is Print_Clipped_Character (0x12260f). Codes above 0x80 are
// neither drawn nor advanced over. A glyph inside the window (by its width
// and font_header.height) goes to Print_Character whole; any other is cut
// to the window by Print_Clipped_Letter. The returned x is computed from
// the cut x and width, so after a right-cut glyph it is x2 + 1 + spacing.
func (p *Printer) clippedChar(t Target, w Window, x, y int, c byte) (int, error) {
	if c > 0x80 {
		return x, nil
	}
	cw, h, sp := int(p.widths[c]), p.height, p.Spacing
	if x >= w.X1 && x+cw <= w.X2 && y >= w.Y1 && y+h <= w.Y2 {
		// Print_Character, not the _No_Alias variant, even in shadow passes.
		err := p.Font.glyph(t, p.Style, x, y, c, &p.Current, false)
		return x + cw + sp, err
	}
	next := x + cw + sp
	sx := 0
	if x < w.X1 {
		sx = w.X1 - x
		if cw <= sx {
			return next, nil
		}
		cw -= sx
		x = w.X1
	}
	if x+cw > w.X2 {
		if cw = w.X2 + 1 - x; cw < 1 {
			return next, nil
		}
	}
	sy := 0
	if y < w.Y1 {
		sy = w.Y1 - y
		y = w.Y1
	}
	maxY := h
	if y+h > w.Y2 {
		// Measured from the already cut y: with both edges cut, sy rows
		// too few are drawn (the original's arithmetic).
		maxY = w.Y2 + 1 - y
	}
	err := p.clippedLetter(t.Pix, t.Stride, x, y, c, sx, cw, sy, maxY)
	return x + cw + sp, err
}

// ErrGlyphOffset reports a glyph the 16-bit clipped blitters cannot reach.
var ErrGlyphOffset = fmt.Errorf("glyph data offset past 0x7fff read through a 16-bit index")

// clippedLetter is Print_Clipped_Letter (0x1227ec) and
// Print_Clipped_Letter_To_Bitmap (0x1231b1): glyph rows sy..min(maxY, base
// height)-1 at (x, y), only glyph columns sx..sx+w-1. Both index the glyph
// data with a signed 16-bit offset, so glyphs at 0x8000 and beyond (style 5
// from 'W' on) read memory before the font; the port returns ErrGlyphOffset.
func (p *Printer) clippedLetter(pix []byte, stride, x, y int, c byte, sx, w, sy, maxY int) error {
	f := p.Font
	s := &f.Styles[p.Style]
	off := uint16(s.Offsets[c])
	at := func() (byte, error) {
		i := int(int16(off))
		if i < 0 || i >= len(f.data) {
			return 0, fmt.Errorf("style %d glyph %d: %w", p.Style, c, ErrGlyphOffset)
		}
		return f.data[i], nil
	}
	start := x + y*stride - sx
	maxY = min(maxY, s.Height)
	for line := 0; line < sy; line++ {
		for {
			b, err := at()
			if err != nil {
				return err
			}
			off++
			if b == 0x80 {
				break
			}
		}
	}
	rows := int(int16(maxY - sy))
	for line := 0; line < rows; line++ {
		pos := start
		for {
			b, err := at()
			if err != nil {
				return err
			}
			off++
			if b == 0x80 {
				break
			}
			if b > 0x80 {
				pos += int(b & 0x7f)
				continue
			}
			// The screen blitter has no zero test (it would read the byte
			// before current_colors); no shipped glyph has a 0.
			if b != 0 && pos >= start+sx && pos < start+sx+w && pos >= 0 && pos < len(pix) {
				pix[pos] = p.Current[b-1]
			}
			pos++
		}
		start += stride
	}
	return nil
}

// passes runs the shadow or outline passes of Print_Display and
// Print_Display_To_Bitmap in outline_color, then the text itself with
// colour codes enabled.
func (p *Printer) passes(x, y int, str func(x, y int, colorsOK bool) error) error {
	if sh := p.Shadow; sh != ShadowNone {
		p.Current[0] = p.AliasOutlineColor
		for i := 1; i < 8; i++ {
			p.Current[i] = p.OutlineColor
		}
		p.noAlias = true
		var offsets [][2]int
		if sh != ShadowUp {
			offsets = append(offsets, [2]int{1, 1}, [2]int{0, 1}, [2]int{1, 0})
		}
		if sh != ShadowDown && sh != ShadowHeavy {
			offsets = append(offsets, [2]int{-1, 0}, [2]int{-1, -1}, [2]int{0, -1})
		}
		if sh == ShadowHeavy || sh == OutlineHeavy {
			offsets = append(offsets, [2]int{2, 2}, [2]int{1, 2}, [2]int{2, 1})
		}
		if sh > ShadowHeavy {
			offsets = append(offsets, [2]int{1, -1}, [2]int{-1, 1})
		}
		if sh == OutlineHeavy {
			offsets = append(offsets, [2]int{2, 0}, [2]int{0, 2})
		}
		for _, o := range offsets {
			if err := str(x+o[0], y+o[1], false); err != nil {
				p.noAlias = false
				return err
			}
		}
		p.Current = p.Normal
	}
	p.noAlias = false
	return str(x, y, true)
}

// Display is Print_Display (0x1212eb), what Print, Print_Full and the
// centred and right-aligned printers end in. With Clip set every pass goes
// through ClippedString, otherwise through String.
func (p *Printer) Display(t Target, x, y int, s []byte, full int) error {
	return p.passes(x, y, func(x, y int, colorsOK bool) error {
		var err error
		if p.Clip != nil {
			_, err = p.ClippedString(t, *p.Clip, x, y, s, colorsOK, full)
		} else {
			_, err = p.String(t, x, y, s, colorsOK, full)
		}
		return err
	})
}

// Print is Print: Print_Display without justification.
func (p *Printer) Print(t Target, x, y int, s []byte) error {
	return p.Display(t, x, y, s, 0)
}

// Full is Print_Full: justify to rightSide pixels; negative means none.
func (p *Printer) Full(t Target, x, y int, s []byte, rightSide int) error {
	return p.Display(t, x, y, s, max(rightSide, 0))
}

// Centered is Print_Centered: x minus half the width, truncated.
func (p *Printer) Centered(t Target, x, y int, s []byte) error {
	return p.Display(t, x-p.Width(s)/2, y, s, 0)
}

// Right is Print_Right: the last column of the text lands on x.
func (p *Printer) Right(t Target, x, y int, s []byte) error {
	return p.Display(t, x-(p.Width(s)-1), y, s, 0)
}
