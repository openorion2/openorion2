package font

// Printing into a kind-1 bitmap (fonts.c, Print_To_Bitmap and friends).
// The target is the bitmap's current frame: width*height bytes, so
// Target.Stride is the bitmap width and Target.H its height.

// ToBitmap is Print_To_Bitmap (0x122a6e).
func (p *Printer) ToBitmap(b Target, x, y int, s []byte) error {
	return p.DisplayToBitmap(b, x, y, s, 0)
}

// FullToBitmap is Print_Full_To_Bitmap (0x122a1f): negative means none.
func (p *Printer) FullToBitmap(b Target, x, y int, s []byte, rightSide int) error {
	return p.DisplayToBitmap(b, x, y, s, max(rightSide, 0))
}

// CenteredToBitmap is Print_Centered_To_Bitmap (0x1229c5).
func (p *Printer) CenteredToBitmap(b Target, x, y int, s []byte) error {
	return p.DisplayToBitmap(b, x-p.Width(s)/2, y, s, 0)
}

// RightToBitmap is Print_Right_To_Bitmap (0x12297b).
func (p *Printer) RightToBitmap(b Target, x, y int, s []byte) error {
	return p.DisplayToBitmap(b, x-(p.Width(s)-1), y, s, 0)
}

// DisplayToBitmap is Print_Display_To_Bitmap (0x122aab): Print_Display's
// passes through StringToBitmap, with no clipping flag and no scan list.
// The original exits with "Print only works on bitmaps" for other kinds;
// that check belongs to the caller, which owns the picture header.
func (p *Printer) DisplayToBitmap(b Target, x, y int, s []byte, full int) error {
	return p.passes(x, y, func(x, y int, colorsOK bool) error {
		return p.StringToBitmap(b, x, y, s, colorsOK, full)
	})
}

// StringToBitmap is Print_String_To_Bitmap (0x122d8a). Unlike Print_String
// it prints nothing when y > height, y + font height <= 0, x > width or
// x + string width <= 0; 0x0d, 0x14, 0x19 and 0x1d end the string instead
// of breaking the line or tabbing; and codes above 0x80 still advance.
func (p *Printer) StringToBitmap(b Target, x, y int, s []byte, colorsOK bool, full int) error {
	p.cur()
	if y > b.H || y+p.height <= 0 || x > b.Stride || x+p.Width(s) <= 0 {
		return nil
	}
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
	px := x
	for _, c := range s {
		switch {
		case c == 0 || c == '\r' || c == 0x14 || c == 0x15 || c == 0x19 || c == 0x1d:
			return nil
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
		}
		if err := p.charToBitmap(b, px, y, c); err != nil {
			return err
		}
		px += int(p.widths[c]) + p.Spacing
		if full != 0 && c == ' ' {
			px += spaceAdd
			if spaceN < spaceRem {
				px++
			}
			spaceN++
		}
	}
	return nil
}

// charToBitmap is Print_Character_To_Bitmap (0x123070): every glyph, even
// one wholly inside, goes through the clipped letter blitter, so pixels
// past the glyph's width are always cut and glyph value 0 is transparent.
func (p *Printer) charToBitmap(b Target, x, y int, c byte) error {
	if c > 0x80 {
		return nil
	}
	cw, h := int(p.widths[c]), p.height
	sx := 0
	if x < 0 {
		sx = -x
		if sx >= cw {
			return nil
		}
		x = 0
		cw -= sx
	}
	if x+cw > b.Stride-1 {
		if cw = b.Stride - x; cw < 1 {
			return nil
		}
	}
	sy := 0
	if y < 0 {
		sy = -y
		y = 0
	}
	maxY := h
	if y+h > b.H-1 {
		maxY = b.H - y // from the cut y, as on the screen
	}
	return p.clippedLetter(b.Pix, b.Stride, x, y, c, sx, cw, sy, maxY)
}
