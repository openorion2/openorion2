package engine

import "errors"

// Paragraph printing: the game's own word-wrapping printer (_Print_Paragraph_Bill_
// and friends, module 122) and the globals of the formatted paragraph system
// (FMTPARA.C, fmtpara.go).
//
// Spec: docs/engine/paragraphs.md.

// Paragraph print types (print_type), as _Print_String_Bill_ dispatches them.
const (
	ParaLeft     = 0 // Print_
	ParaRight    = 1 // Print_Right_ at x + max_width - 1
	ParaCentered = 2 // Print_Centered_ at x + max_width/2
	ParaFull     = 3 // Print_Full_ to max_width; the last line prints left
)

var (
	// ErrParagraphStuck is returned where _Print_Paragraph_Bill_ loops forever:
	// a glyph wider than max_width with nothing on the line before it.
	ErrParagraphStuck = errors.New("paragraph: line cannot advance (the original hangs)")
	// ErrParagraphLine is returned where a line outgrows the 200-byte stack
	// buffer, which in the original overwrites the routine's own locals.
	ErrParagraphLine = errors.New("paragraph: line longer than 200 bytes")
)

// paraLineMax is the size of _Print_Paragraph_Bill_'s line buffer.
const paraLineMax = 200

// Paragraphs is the paragraph state the original keeps in globals.
type Paragraphs struct {
	// MaxHeight and MaxWidth are __pp_max_height (0x1aceb8) and
	// __pp_max_width (0x1aceba); they keep their value when the arguments
	// are rejected, so a rejected Get returns the previous result.
	MaxHeight int16
	MaxWidth  uint16

	// Formatted paragraph system (FMTPARA.C), cleared by
	// Initialize_Formatted_Paragraph_System_. Global is _para_global
	// (0x1acf18): +0 the extent x1, y1, x2, y2 (shorts), +8..0x47 the
	// variable pointers (kept in VarPtr and varRef), +0x48 the type bytes,
	// +0x5c the test flag, +0x5d the plural flag. Tabs is a short count and
	// ten short stops.
	Tabs      [0x16]byte // _tabs (0x1838f7)
	Global    [0x5e]byte
	Variables [16]int32  // _variable (0x1acebc)
	VarPtr    [16]*int32 // _para_global + 8: variable i when it is an int
	varRef    [16]any    // the same for other pointees ([]byte, *int16, ...)

	// Statics of FMTPARA.C that keep their value between paragraphs.
	InsertCase int16    // _insert_case (0x1acf76): 1 upper, 2 lower, 3 capitalise
	ordBuf     [32]byte // 0x1acf78: the ordinal insert
	numBuf     [16]byte // 0x1acf98: the number insert (itoa_)
	supSave    fmtStyle // 0x1acfa8: the style saved by function ^
	supSaved   bool     // false while 0x1acfa8 still holds its zero bytes
	varIndex   uint16   // 0x1acfb5: function = variable index
	lastFont   byte     // 0x1acfbb: Print_Line_'s last glyph style byte
	lastAttr   byte     // 0x1acfbc: Print_Line_'s last glyph attribute
}

// VarType is the type byte of paragraph variable i (_para_global + 0x48).
func (p *Paragraphs) VarType(i int) byte { return p.Global[0x48+i] }

// InitializeFormatted is Initialize_Formatted_Paragraph_System_ (0x103d1b),
// reached from Bill_Default_ (0x10206e).
func (p *Paragraphs) InitializeFormatted() {
	p.Tabs = [0x16]byte{}
	p.Global = [0x5e]byte{}
	p.VarPtr = [16]*int32{}
	p.varRef = [16]any{}
	for i := range 16 {
		p.ResetVariable(i)
	}
}

// ResetVariable is Reset_A_Paragraph_Variable_ (0x104b6b): variable i points
// at its own slot, holds 0 and has type 2. Out-of-range indices do nothing.
func (p *Paragraphs) ResetVariable(i int) {
	if uint16(i) >= 16 {
		return
	}
	p.VarPtr[i], p.varRef[i] = &p.Variables[i], nil
	p.Variables[i] = 0
	p.Global[0x48+i] = 2
}

// ResetVariables is Reset_Paragraph_Variables_ (0x104b2f).
func (p *Paragraphs) ResetVariables() {
	for i := range 16 {
		p.ResetVariable(i)
	}
}

// Paragraphs returns the engine's paragraph globals.
func (e *Engine) Paragraphs() *Paragraphs { return &e.para }

// PrintParagraph is _Print_Paragraph_ (0x103915): word-wrap s into lines of
// at most maxw pixels from (x, y) and print each with print type typ.
func (t *Text) PrintParagraph(x, y, maxw int, s string, typ int) error {
	return t.paragraphBill(x, y, maxw, s, typ, t.drawBill)
}

// ParagraphMaxHeight is _Get_Paragraph_Max_Height_ (0x103952): the height
// the paragraph would take, measured as print type 0 from (0, 0).
func (t *Text) ParagraphMaxHeight(maxw int, s string) (int, error) {
	err := t.paragraphBill(0, 0, maxw, s, ParaLeft, nil)
	return int(t.e.Paragraphs().MaxHeight), err
}

// ParagraphMaxWidth is _Get_Paragraph_Max_Width_ (0x103971): the widest line,
// where lines of types 1 and 3 count as maxw (the unjustified last line of
// type 3 counts its own width).
func (t *Text) ParagraphMaxWidth(maxw int, s string, typ int) (int, error) {
	err := t.paragraphBill(0, 0, maxw, s, typ, nil)
	return int(int16(t.e.Paragraphs().MaxWidth)), err
}

// paraArgsOK is the argument check both Bill routines start with.
func paraArgsOK(x, y, maxw int16) bool {
	return maxw >= 10 && maxw <= 0x280 && x >= 0 && x < 0x280 && y >= 0 && y < 0x1e0
}

// charWidth is current_font_widths[c] (c >= 0x20).
func (t *Text) charWidth(c byte) int { return t.Width([]byte{c}) - t.Spacing }

// paragraphBill is _Print_Paragraph_Bill_ (0x10370a). draw is nil when only
// measuring (print flag 0); the bitmap variants are not ported.
func (t *Text) paragraphBill(x, y, maxw int, str string, typ int, draw func(x, y, maxw int16, line []byte, typ int) error) error {
	X, Y, MW := int16(x), int16(y), int16(maxw)
	if !paraArgsOK(X, Y, MW) {
		return nil
	}
	ps := t.e.Paragraphs()
	ps.MaxWidth = 0
	lf := int16(t.LineFeed())
	base := int16(t.Font.Styles[t.Style].Height) // current_base_height
	limit := int16(0x1df) - base
	at := func(i int16) byte {
		if int(i) < len(str) {
			return str[i]
		}
		return 0
	}
	var buf [paraLineMax]byte
	put := func(i int16, c byte) error {
		if int(i) >= paraLineMax {
			return ErrParagraphLine
		}
		buf[i] = c
		return nil
	}
	var (
		src, dst, brkSrc, brkDst int16
		acc                      uint16
		brk                      byte
		yy                       = Y
	)
	stuck := map[int16]bool{}
	for {
		c := at(src)
		if c == 0 {
			break
		}
		if c == '\t' {
			c = ' '
		}
		if c == ' ' || (c == 8 && int(MW) >= int(acc)+t.charWidth('-')) {
			brk, brkSrc, brkDst = c, src, dst
		}
		if c != 0x0d && c != 0x0a {
			if c < 0x20 {
				src++
				continue
			}
			acc += uint16(t.charWidth(c))
			if c == ' ' || int(acc) <= int(MW) {
				if err := put(dst, c); err != nil {
					return err
				}
				src++
				acc += uint16(t.Spacing)
				dst++
				continue
			}
			// The word does not fit: back up to the last break, if any.
			if brkDst != 0 {
				dst = brkDst
			}
			if brkSrc != 0 {
				src = brkSrc
			}
		} else {
			brk = 0x0d
		}
		for dst > 1 && buf[dst-1] == ' ' {
			dst--
		}
		if dst > 0 || brk == 0x0d {
			if brk == 8 {
				if err := put(dst, '-'); err != nil {
					return err
				}
				dst++
			} else if brk <= 0x20 {
				src++ // the break character; with no break, the glyph that overflowed
			}
			if err := put(dst, 0); err != nil {
				return err
			}
			if err := t.printStringBill(X, yy, MW, buf[:dst], typ, draw); err != nil {
				return err
			}
			yy += lf
			dst = 0
			clear(stuck)
			if yy > limit {
				break
			}
		} else {
			if stuck[src] {
				return ErrParagraphStuck
			}
			stuck[src] = true
		}
		acc, brk, brkSrc, brkDst = 0, 0, 0, 0
	}
	if err := put(dst, 0); err != nil {
		return err
	}
	if dst != 0 {
		if typ == ParaFull {
			typ = ParaLeft
		}
		if err := t.printStringBill(X, yy, MW, buf[:dst], typ, draw); err != nil {
			return err
		}
	}
	ps.MaxHeight = yy + base - Y
	return nil
}

// printStringBill is _Print_String_Bill_ (0x1035af): print one line (when
// draw is set) and raise __pp_max_width to its width.
func (t *Text) printStringBill(x, y, maxw int16, line []byte, typ int, draw func(x, y, maxw int16, line []byte, typ int) error) error {
	if !paraArgsOK(x, y, maxw) {
		return nil
	}
	w := int16(t.Width(line))
	if typ == ParaRight || typ == ParaFull {
		w = maxw
	}
	if draw != nil {
		if err := draw(x, y, maxw, line, typ); err != nil {
			return err
		}
	}
	ps := t.e.Paragraphs()
	if int(w) > int(ps.MaxWidth) {
		ps.MaxWidth = uint16(w)
	}
	return nil
}

// drawBill is the screen half of _Print_String_Bill_'s dispatch (table at
// 0x10357b); other types print nothing.
func (t *Text) drawBill(x, y, maxw int16, line []byte, typ int) error {
	s := string(line)
	switch typ {
	case ParaLeft:
		return t.Print(int(x), int(y), s)
	case ParaRight:
		return t.PrintRight(int(int16(x+maxw-1)), int(y), s)
	case ParaCentered:
		return t.PrintCentered(int(int16(x+maxw/2)), int(y), s)
	case ParaFull:
		return t.display(int(x), int(y), line, max(int(maxw), 0)) // Print_Full_
	}
	return nil
}

// SqueezePrintParagraph is _Squeeze_Print_Paragraph_ (0x102fd8) for plain
// paragraphs: while s is taller than maxh, narrow the space down to the
// style's limit (limits, _space_limits by style), then the vertical spacing
// down to 2, then drop to the next smaller style; print (centred on y when
// center), then put the font back.
func (t *Text) SqueezePrintParagraph(x, y, maxw, maxh int, s string, typ int, limits []byte, center bool) error {
	return t.squeezePrintParagraph(x, y, maxw, maxh, s, typ, limits, center, false)
}

// SqueezePrintFormattedParagraph is _Squeeze_Print_Paragraph_ with its
// formatted flag: the FMTPARA measure and printer (Squeeze_Print_Formatted_
// Paragraph_ 0x1031aa, Squeeze_Formatted_Paragraph_Centered_ 0x10323b).
func (t *Text) SqueezePrintFormattedParagraph(x, y, maxw, maxh int, s string, typ int, limits []byte, center bool) error {
	return t.squeezePrintParagraph(x, y, maxw, maxh, s, typ, limits, center, true)
}

func (t *Text) squeezePrintParagraph(x, y, maxw, maxh int, s string, typ int, limits []byte, center, formatted bool) error {
	measure, print := t.ParagraphMaxHeight, t.PrintParagraph
	if formatted {
		measure, print = t.FormattedMaxHeight, t.PrintFormatted
	}
	space, hsp, vsp := t.SpaceWidth(), t.Spacing, t.VSpacing()
	style, shadow := t.Style, t.Shadow
	smaller := false
	h := 0
	for {
		var err error
		if h, err = measure(maxw, s); err != nil {
			return err
		}
		if int16(h) <= int16(maxh) {
			break
		}
		done := true
		for {
			if h, err = measure(maxw, s); err != nil {
				return err
			}
			if int16(h) <= int16(maxh) || t.Style >= len(limits) || byte(t.SpaceWidth()) <= limits[t.Style] {
				break
			}
			done = false
			t.SetSpaceWidth(t.SpaceWidth() - 1)
		}
		for {
			if h, err = measure(maxw, s); err != nil {
				return err
			}
			if int16(h) <= int16(maxh) || t.VSpacing() <= 2 {
				break
			}
			done = false
			t.SetVSpacing(t.VSpacing() - 1)
		}
		if h, err = measure(maxw, s); err != nil {
			return err
		}
		if int16(h) > int16(maxh) && t.Style > 0 {
			t.SetStyle(t.Style-1, t.Current)
			smaller, done = true, false
		}
		if done {
			break
		}
	}
	if center {
		y = int(int16(y) - int16(h)/2)
	}
	err := print(x, y, maxw, s, typ)
	t.SetSpaceWidth(space)
	t.SetSpacing(hsp)
	t.SetVSpacing(vsp)
	if smaller {
		t.SetStyle(style, t.Normal)
		t.Shadow = shadow
	}
	return err
}
