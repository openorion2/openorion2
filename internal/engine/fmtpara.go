package engine

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/openorion2/openorion2/internal/font"
)

// The formatted paragraph system, FMTPARA.C (module 124, 0x103bc4–0x106cac):
// word wrap with control codes, functions, tabs, variables and inserts.
// Spec: docs/engine/paragraphs.md ("Formatted paragraphs").

var (
	// ErrFmtOverrun is returned where the original reads on past a string's
	// terminator (a function argument at the very end of a string).
	ErrFmtOverrun = errors.New("formatted paragraph: read past the end of the string")
	// ErrFmtLineFull is returned where a character record would land
	// outside the 150-entry line buffer (the original overwrites its locals).
	ErrFmtLineFull = errors.New("formatted paragraph: character record outside the line buffer")
	// ErrFmtOrigin is function O: it re-enters the printer, which leaves the
	// globals on its own dead stack frame, and the original continues there.
	ErrFmtOrigin = errors.New("formatted paragraph: function O continues on a dead stack frame")
	// ErrFmtStyle is a font style past the six a font holds, or the superscript
	// restore of a style never saved (a NULL widths table in the original).
	ErrFmtStyle = errors.New("formatted paragraph: invalid font style")
	// ErrFmtJustify is full justification that can never use up its pixels;
	// the original loops forever.
	ErrFmtJustify = errors.New("formatted paragraph: full justification cannot advance (the original hangs)")
)

const (
	fmtMaxChars = 0x96 // char records in the line buffer
	fmtMaxNest  = 10   // insert nesting levels
)

// Static tables of the module (data object 2).
var (
	fmtFunctions = []byte("FRTXYSHVPIMOC^=") // _legal_functions 0x1838d2
	fmtArgChars  = []byte("1234567890+- ")   // _legal_fn_arg_chars 0x1838e2
	// fmtBaseOffsets is _font_base_offsets (0x18390d), by style.
	fmtBaseOffsets = [8]byte{0x05, 0x07, 0x0a, 0x0b, 0x0e, 0x13, 0, 0}
	// fmtResetFlags is the table at 0x183915 indexed by code & 0xf; its last
	// seven entries are the first bytes of fmtJustify.
	fmtResetFlags = [16]byte{0x01, 0x02, 0x04, 0x08, 0x0c, 0x0d, 0x10, 0x20, 0x1d, 0x01, 0x03, 0x01, 0, 0, 0, 0}
	// fmtJustify is the pass pattern table at 0x18391e: bit 0 widens spaces,
	// bit 1 every glyph; a row ends at 0.
	fmtJustify = [3][8]byte{
		{1, 3, 1, 0, 0, 0, 0, 0},
		{1, 1, 1, 3, 1, 0, 0, 0},
		{1, 1, 1, 1, 3, 1, 3, 0},
	}
	fmtPluralS   = []byte("s")  // _plural_s_str 0x18386c
	fmtPluralES  = []byte("es") // _plural_es_str 0x18386e
	fmtOrdEnding = [10]string{"th", "st", "nd", "rd", "th", "th", "th", "th", "th", "th"}
	fmtOrdWord   = [11]string{"zeroeth", "first", "second", "third", "fourth", "fifth",
		"sixth", "seventh", "eighth", "ninth", "tenth"}
	// fmtInsertClass is the 256 bytes at 0x17a228 that Insert_Item_ indexes
	// with the item letter: anything but '0' takes an argument. For 'A'..'T'
	// it is the string "11110101121212120000"; below 'A' it runs through the
	// ordinal words, above 'T' through unrelated strings.
	fmtInsertClass = mustHex("6669727374007365636f6e6400746869726400666f7572746800666966746800736978746800736576656e746800656967687468006e696e74680074656e7468003131313130313031313231323132313230303030000000494e464f2e4c4258001b000025692e25690025692c25690062696c6c746578742e6c6278002569002533692525006170705f706963732e6c6278004150505f504943532e4c42580062696c6c746578322e6c6278005e20002d002b003120424300302e35204243002d302e35002b31002b32002573252b69005e005e202573000000000072616365732e6c62780072616365727072742e6c6278000a0062696c6c746578742e6c62")
)

func mustHex(s string) []byte {
	b := make([]byte, len(s)/2)
	for i := range b {
		v, err := strconv.ParseUint(s[2*i:2*i+2], 16, 8)
		if err != nil {
			panic(err)
		}
		b[i] = byte(v)
	}
	return b
}

// fmtPtr is a char pointer: a string and an index into it. Index len(s) is
// the terminator.
type fmtPtr struct {
	s []byte
	i int
}

// fmtStyle is the 13-byte style record (_style_p, inside the line record).
type fmtStyle struct {
	font   byte  // +0: style number
	remap  byte  // +1: printed through Set_Remap_Font_Style (function R)
	shadow byte  // +2: shadow flag (or '0'..'5' after code 0x1c)
	color  byte  // +3: stored per glyph, never read (uninitialised at start)
	set    byte  // +4: colour set << 5 (normal 0, highlight 0x20, special 0x40)
	base   int16 // +5: base height
	widths byte  // +7: whose font_widths the layout measures with
	space  int16 // +0xb: width of ' '
}

// fmtChar is one 8-byte glyph record of the line buffer.
type fmtChar struct {
	dx    int16 // +0: x relative to the previous glyph's
	y     int16 // +2
	font  byte  // +4: style, | 0x80 for a remapped style
	attr  byte  // +5: shadow flag | colour set
	color byte  // +6
	c     byte  // +7
}

// fmtLine is the line record (_line_p).
type fmtLine struct {
	x, left, right int16 // +0, +2, +4
	wrapX, wrapR   int16 // +6, +8: continuation past an exclusion area
	lastR          int16 // +0xa: last column of the last glyph
	n, spaces      int16 // +0xc, +0xe
	yoff           int16 // +0x10: raise applied when the line completes
	prevX          int16 // +0x12: x of the previous glyph
	width          int16 // +0x14
	hs             int16 // +0x16: horizontal spacing
	done, start    byte  // +0x18, +0x19
	style          fmtStyle
}

// fmtBreak is the last break position (_break_p, 0x23 bytes).
type fmtBreak struct {
	ptr                   fmtPtr // +0: where to resume
	x, lastR, icase, yoff int16  // +4, +6, +8, +0xa
	hs, nest, n, spaces   int16  // +0xc, +0xe, +0x10, +0x12
	hyph, valid           byte   // +0x14, +0x15
	style                 fmtStyle
}

// fmtState is one _Print_Formatted_Paragraph_ frame: the paragraph record
// (_para_p), the line, break and glyph records and the char pointer.
type fmtState struct {
	t  *Text
	ps *Paragraphs

	y             int16  // +0
	ptr           fmtPtr // +2
	x, x2, y0     int16  // +6, +8, +0xa
	ylim, mw      int16  // +0xc, +0xe
	bitmap        *font.Target
	print         bool  // +0x14
	lineL, lineR  int16 // +0x15, +0x17: written, never read
	ymax, lf, ff  int16 // +0x19, +0x1d, +0x1f
	indent, rm    int16 // +0x21, +0x23
	lm            int16 // +0x25
	just          byte  // +0x27
	newPara       bool  // +0x28
	nest          int16 // +0x29
	stack         [fmtMaxNest]fmtPtr
	line          fmtLine
	brk           fmtBreak
	chars         [fmtMaxChars]fmtChar
	cp            int // _char_p, an index into chars
	err           error
	justifyRounds int
}

// Box returns _para_global's first four shorts: the extent (x1, y1, x2, y2)
// of the glyphs the last formatted paragraph laid out.
func (p *Paragraphs) Box() (x1, y1, x2, y2 int16) {
	return p.box(0), p.box(1), p.box(2), p.box(3)
}

func (p *Paragraphs) box(i int) int16 { return int16(binary.LittleEndian.Uint16(p.Global[2*i:])) }
func (p *Paragraphs) setBox(i int, v int16) {
	binary.LittleEndian.PutUint16(p.Global[2*i:], uint16(v))
}

// LastFormattedMaxHeight is Get_Last_Formatted_Paragraph_Max_Height_
// (0x103cfd): y2 − y1 + 1, 16 bits.
func (p *Paragraphs) LastFormattedMaxHeight() int { return int(p.box(3) - p.box(1) + 1) }

// LastFormattedMaxWidth is Get_Last_Formatted_Paragraph_Max_Width_
// (0x103d0c): x2 − x1 + 1, 16 bits.
func (p *Paragraphs) LastFormattedMaxWidth() int { return int(p.box(2) - p.box(0) + 1) }

// testFlag and pluralFlag are bytes 0x1acf74 and 0x1acf75 (_para_global +
// 0x5c, + 0x5d).
func (p *Paragraphs) testFlag() byte        { return p.Global[0x5c] }
func (p *Paragraphs) setTestFlag(v byte)    { p.Global[0x5c] = v }
func (p *Paragraphs) pluralFlag() byte      { return p.Global[0x5d] }
func (p *Paragraphs) setPluralFlag(v bool)  { p.Global[0x5d] = b2u(v) }
func (p *Paragraphs) tabCount() int16       { return int16(binary.LittleEndian.Uint16(p.Tabs[0:])) }
func (p *Paragraphs) setTabCount(n int16)   { binary.LittleEndian.PutUint16(p.Tabs[0:], uint16(n)) }
func (p *Paragraphs) tab(i int) int16       { return int16(binary.LittleEndian.Uint16(p.Tabs[2+2*i:])) }
func (p *Paragraphs) setTab(i int, v int16) { binary.LittleEndian.PutUint16(p.Tabs[2+2*i:], uint16(v)) }

// sortTabs is the qsort of the stops with _Comp_Tabs_ (0x1067bb); equal
// stops are identical shorts, so the order of ties does not matter.
func (p *Paragraphs) sortTabs() {
	n := int(p.tabCount())
	v := make([]int16, n)
	for i := range v {
		v[i] = p.tab(i)
	}
	sort.Slice(v, func(a, b int) bool { return v[a] < v[b] })
	for i, x := range v {
		p.setTab(i, x)
	}
}

func b2u(v bool) byte {
	if v {
		return 1
	}
	return 0
}

// Paragraph variable pointers: Set_Paragraph_*_Pointer_ (0x103c07–0x103c93)
// point variable i (< 16) at caller memory and set its type byte.

// SetVarString is Set_Paragraph_String_Pointer_ (type 1).
func (p *Paragraphs) SetVarString(i int, s []byte) { p.setVar(i, s, 1) }

// SetVarInt is Set_Paragraph_Integer_Pointer_ (type 2).
func (p *Paragraphs) SetVarInt(i int, v *int32) { p.setVar(i, v, 2) }

// SetVarUint is Set_Paragraph_Uint_Pointer_ (type 3).
func (p *Paragraphs) SetVarUint(i int, v *uint16) { p.setVar(i, v, 3) }

// SetVarSint is Set_Paragraph_Sint_Pointer_ (type 4).
func (p *Paragraphs) SetVarSint(i int, v *int16) { p.setVar(i, v, 4) }

// SetVarChar is Set_Paragraph_Char_Pointer_ (type 5).
func (p *Paragraphs) SetVarChar(i int, v *uint8) { p.setVar(i, v, 5) }

// SetVarSchar is Set_Paragraph_Schar_Pointer_ (type 6).
func (p *Paragraphs) SetVarSchar(i int, v *int8) { p.setVar(i, v, 6) }

func (p *Paragraphs) setVar(i int, ref any, typ byte) {
	if uint16(i) >= 16 {
		return
	}
	if v, ok := ref.(*int32); ok {
		p.VarPtr[i], p.varRef[i] = v, nil
	} else {
		p.VarPtr[i], p.varRef[i] = nil, ref
	}
	p.Global[0x48+i] = typ
}

// varBytes is the memory variable i points at, as far as the port knows it:
// a string, or the little-endian bytes of the integer.
func (p *Paragraphs) varBytes(i int) []byte {
	var b [4]byte
	switch v := p.varRef[i].(type) {
	case []byte:
		return v
	case *uint16:
		binary.LittleEndian.PutUint16(b[:], *v)
	case *int16:
		binary.LittleEndian.PutUint16(b[:], uint16(*v))
	case *uint8:
		b[0] = *v
	case *int8:
		b[0] = byte(*v)
	default:
		if p.VarPtr[i] != nil {
			binary.LittleEndian.PutUint32(b[:], uint32(*p.VarPtr[i]))
		}
	}
	return b[:]
}

// varString is variable i read as a string, up to its terminator.
func (p *Paragraphs) varString(i uint16) []byte {
	b := p.varBytes(int(i))
	for k, c := range b {
		if c == 0 {
			return b[:k]
		}
	}
	return b
}

// getItem is Get_Item_Value_ (0x104f6a; jump table 0x104f52 by type − 1):
// 2 int, 3 unsigned short, 4 short, 6 signed char; 1, 5 and anything else
// the first byte, zero-extended. Indices past 15 give 0.
func (p *Paragraphs) getItem(i uint16) int32 {
	if i >= 16 {
		return 0
	}
	b := p.varBytes(int(i))
	switch p.Global[0x48+i] {
	case 2:
		return int32(binary.LittleEndian.Uint32(b))
	case 3:
		return int32(binary.LittleEndian.Uint16(b))
	case 4:
		return int32(int16(binary.LittleEndian.Uint16(b)))
	case 6:
		return int32(int8(b[0]))
	}
	if len(b) == 0 {
		return 0
	}
	return int32(b[0])
}

// setItem is Set_Item_Value_ (0x104feb; table 0x104fd3): types 2 store a
// dword, 3 and 4 a word, 5 and 6 a byte; strings and bad indices fail.
func (p *Paragraphs) setItem(i uint16, v int32) bool {
	if i >= 16 {
		return false
	}
	typ := p.Global[0x48+i]
	if typ < 2 || typ > 6 {
		return false
	}
	switch r := p.varRef[i].(type) {
	case *uint16:
		*r = uint16(v)
	case *int16:
		*r = int16(v)
	case *uint8:
		*r = uint8(v)
	case *int8:
		*r = int8(v)
	case []byte:
		return false
	default:
		if p.VarPtr[i] == nil {
			return false
		}
		switch typ {
		case 2:
			*p.VarPtr[i] = v
		case 3, 4:
			*p.VarPtr[i] = int32(uint32(*p.VarPtr[i])&0xffff0000 | uint32(uint16(v)))
		default:
			*p.VarPtr[i] = int32(uint32(*p.VarPtr[i])&0xffffff00 | uint32(uint8(v)))
		}
	}
	return true
}

// PrintFormatted is Print_Formatted_Paragraph_ (0x103bc4): lay s out from
// (x, y) in lines of at most maxw pixels with justification typ and print it
// glyph by glyph (Print_Character, no clipping).
func (t *Text) PrintFormatted(x, y, maxw int, s string, typ int) error {
	_, err := t.formatted(x, y, maxw, []byte(s), typ, nil, true)
	return err
}

// PrintFormattedToBitmap is Print_Formatted_Paragraph_To_Bitmap_ (0x103be2):
// the same into a bitmap through Print_Character_To_Bitmap.
func (t *Text) PrintFormattedToBitmap(x, y, maxw int, s string, typ int, bm font.Target) error {
	_, err := t.formatted(x, y, maxw, []byte(s), typ, &bm, true)
	return err
}

// FormattedMaxHeight is Get_Formatted_Paragraph_Max_Height_ (0x103caf): the
// paragraph laid out at (0, 0), type 0, nothing printed; y2 − y1 + 1. As in
// the original the extent still goes into the scan list.
func (t *Text) FormattedMaxHeight(maxw int, s string) (int, error) {
	_, err := t.formatted(0, 0, maxw, []byte(s), 0, nil, false)
	return t.e.Paragraphs().LastFormattedMaxHeight(), err
}

// FormattedMaxWidth is Get_Formatted_Paragraph_Max_Width_ (0x103cd6): the
// same with justification typ; x2 − x1 + 1.
func (t *Text) FormattedMaxWidth(maxw int, s string, typ int) (int, error) {
	_, err := t.formatted(0, 0, maxw, []byte(s), typ, nil, false)
	return t.e.Paragraphs().LastFormattedMaxWidth(), err
}

// formatted is _Print_Formatted_Paragraph_ (0x103d53); it returns the char
// pointer where processing stopped.
func (t *Text) formatted(x, y, maxw int, s []byte, typ int, bm *font.Target, print bool) (fmtPtr, error) {
	X, Y, MW := int16(x), int16(y), int16(maxw)
	if MW < 10 || MW > 0x280 || X < 0 || X >= 0x280 || Y < 0 || Y >= 0x1e0 || s == nil {
		return fmtPtr{s: s}, nil
	}
	ps := t.e.Paragraphs()
	f := &fmtState{t: t, ps: ps, y: Y, ptr: fmtPtr{s: s}, x: X, x2: X + MW - 1, y0: Y,
		ylim: 0x1df, mw: MW, bitmap: bm, print: print, just: byte(typ), newPara: true}
	st := &f.line.style
	st.font = byte(t.Style)
	st.shadow = byte(t.Shadow)
	st.set = f.colorSet()
	if st.font >= font.Styles {
		return f.ptr, ErrFmtStyle
	}
	fs := &t.Font.Styles[st.font]
	st.base = int16(fs.Height)
	f.lf = int16(fs.VSpacing) + st.base
	f.line.hs = int16(fs.HSpacing)
	st.widths = st.font
	st.space = int16(fs.Widths[' '])
	f.ff = f.lf
	f.ymax = 0x1e0 - st.base
	for !f.setLineLimits() {
		f.y++
		if f.y > f.ymax {
			return fmtPtr{}, nil
		}
	}
	ps.setBox(2, 0)
	ps.setBox(3, 0)
	ps.setBox(0, f.x)
	ps.setBox(1, f.y0)
	saved := *t.Printer // font_header and current_font_style
	f.processText()
	*t.Printer = saved
	return f.ptr, f.err
}

// ch reads the byte at p; past the terminator it records ErrFmtOverrun.
func (f *fmtState) ch(p fmtPtr) byte {
	if p.i >= 0 && p.i < len(p.s) {
		return p.s[p.i]
	}
	if p.i != len(p.s) && f.err == nil {
		f.err = ErrFmtOverrun
	}
	return 0
}

// width is font_header.font_widths[style][c], the font file's widths.
func (f *fmtState) width(style, c byte) int16 {
	if style >= font.Styles {
		f.fail(ErrFmtStyle)
		return 0
	}
	return int16(f.t.Font.Styles[style].Widths[c])
}

func (f *fmtState) styleOf(style byte) *font.Style {
	if style >= font.Styles {
		f.fail(ErrFmtStyle)
		return &font.Style{}
	}
	return &f.t.Font.Styles[style]
}

func (f *fmtState) fail(err error) {
	if f.err == nil {
		f.err = err
	}
}

// colorSet is Get_Current_Color_Set_ (0x1067f0): 0 when the current colours
// are the normal ones, 0x20 when they are the highlight ones, else 0x40.
func (f *fmtState) colorSet() byte {
	p := f.t.Printer
	var n, h int
	for i := range 8 {
		if p.Current[i] == p.Normal[i] {
			n++
		} else if p.Current[i] == p.Highlight[i] {
			h++
		}
	}
	switch {
	case n == 8:
		return 0
	case h == 8:
		return 0x20
	}
	return 0x40
}

func isAlpha(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }

func toUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 0x20
	}
	return c
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 0x20
	}
	return c
}

// strchr reports whether c is in set, its terminator included.
func strchr(set []byte, c byte) bool {
	if c == 0 {
		return true
	}
	for _, b := range set {
		if b == c {
			return true
		}
	}
	return false
}

// processText is Process_Paragraph_Text_ (0x103f5d).
func (f *fmtState) processText() {
	ln, st, ps := &f.line, &f.line.style, f.ps
	f.cp = 0
	for {
		ln.done, ln.start, ln.prevX = 0, 1, 0
		ln.lastR = ln.x
		f.recordBreak(0)
		quit := false
		for {
			c := f.ch(f.ptr)
			if f.err != nil {
				return
			}
			if c >= 0x20 {
				if isAlpha(c) && ps.InsertCase != 0 {
					switch ps.InsertCase {
					case 1, 3:
						c = toUpper(c)
					case 2:
						c = toLower(c)
					}
				}
				var r int16
				if c == ' ' {
					r = ln.x + st.space
				} else {
					r = f.width(st.widths, c) + ln.x
				}
				r--
				if ln.n < fmtMaxChars && (r <= ln.right || ln.start != 0) {
					ln.lastR = r
					f.addChar(c)
					x := ln.x
					ln.start = 0
					ln.prevX = x
					ln.x = r + ln.hs + 1
					if ps.InsertCase == 3 && isAlpha(c) {
						ps.InsertCase = 0
					}
				} else {
					if c != ' ' && c != '\t' {
						if f.returnToBreak() && f.brk.hyph != 0 {
							if f.cp < 0 || f.cp >= fmtMaxChars {
								f.fail(ErrFmtLineFull)
								return
							}
							f.chars[f.cp].c = '-'
						}
						f.ptr.i--
					}
					if ln.wrapX > ln.x {
						v := ln.wrapX
						ln.wrapX = 0
						ln.x, ln.lastR, ln.left = v, v, v
						ln.right = ln.wrapR
					} else {
						c = 0x0d
					}
				}
			}
			if c < 0x20 {
				quit = f.command(c)
			}
			f.ptr.i++
			if f.err != nil {
				return
			}
			if ln.done != 0 || quit {
				break
			}
		}
		if quit {
			break
		}
	}
	x1, y1, x2, y2 := ps.Box()
	f.t.e.V.AddSquare(int(x1), int(y1), int(x2), int(y2))
}

// addChar is Add_Character_To_Line_ (0x104141).
func (f *fmtState) addChar(c byte) {
	ln, st := &f.line, &f.line.style
	if f.cp < 0 || f.cp >= fmtMaxChars {
		f.fail(ErrFmtLineFull)
		return
	}
	g := &f.chars[f.cp]
	g.c, g.y, g.dx = c, f.y, ln.x-ln.prevX
	g.attr = st.shadow | st.set
	g.font, g.color = st.font, st.color
	if st.remap != 0 {
		g.font |= 0x80
	}
	f.cp++
	if c == ' ' {
		f.recordBreak(' ')
		ln.spaces++
	}
	ln.n++
}

// returnToBreak is Return_To_Last_Break_ (0x1041bb).
func (f *fmtState) returnToBreak() bool {
	ln, b := &f.line, &f.brk
	if b.valid == 0 {
		return false
	}
	f.ptr = b.ptr
	ln.x, ln.lastR = b.x, b.lastR
	f.ps.InsertCase = b.icase
	f.nest = b.nest
	ln.yoff, ln.hs, ln.n, ln.spaces = b.yoff, b.hs, b.n, b.spaces
	ln.style = b.style
	if b.hyph != 0 && int(ln.n)-1 == f.cp {
		f.addChar('-')
		f.cp--
		ln.n--
	} else {
		f.cp = int(ln.n) - 1
	}
	return true
}

// recordBreak is Record_Break_Position_ (0x104292).
func (f *fmtState) recordBreak(c byte) {
	ln, b := &f.line, &f.brk
	if ln.start != 0 || c == 0 {
		b.valid = 0
		return
	}
	b.valid = 1
	var r int16
	if c == 8 {
		r = ln.x + f.width(ln.style.widths, '-')
		if r > ln.right {
			return
		}
	}
	b.ptr = f.ptr
	b.ptr.i++
	b.x, b.lastR, b.icase = ln.x, ln.lastR, f.ps.InsertCase
	b.yoff, b.hs, b.nest = ln.yoff, ln.hs, f.nest
	b.n, b.spaces = ln.n, ln.spaces
	b.style = ln.style
	b.hyph = 0
	switch c {
	case 8:
		b.hyph = 1
		b.n++
		b.x, b.lastR = r, r
	case ' ':
		b.lastR -= f.width(ln.style.widths, ' ') + ln.hs
	}
}

// quitParagraph is Quit_Paragraph_ (0x1046ce): leave every insert and stop
// before the terminator, so the next command read is the 0.
func (f *fmtState) quitParagraph() {
	if f.nest != 0 {
		f.ptr = f.stack[0]
		f.nest = 0
	}
	for f.ch(f.ptr) != 0 {
		f.ptr.i++
	}
	f.ptr.i--
}

// previousNesting is Previous_Nesting_Level_ (0x105508).
func (f *fmtState) previousNesting() {
	if f.nest > 0 {
		f.nest--
		f.ptr = f.stack[f.nest]
		f.ptr.i--
	}
}

// command is Process_Command_ (0x1043b0) for a code below 0x20; true ends
// the paragraph.
func (f *fmtState) command(c byte) bool {
	ln, st, ps := &f.line, &f.line.style, f.ps
	if c >= 0x17 {
		// Jump table 0x10438c.
		switch c {
		case 0x18:
			f.insertItem()
			f.ptr.i--
			return false
		case 0x19:
			f.testItem()
			return false
		case 0x1a:
			f.ptr.i++
			d := f.ch(f.ptr) - '0'
			if ln.n > 0 {
				r := ln.right
				ln.right = ln.x
				f.completeLine()
				ln.right = r
				f.lineL = ln.x
				f.initLineStart()
				f.cp = 0
				ln.done = 0
			}
			f.just = d
			return false
		case 0x1b:
			f.ptr.i++
			st.set = (f.ch(f.ptr) - '0') << 5
			return false
		case 0x1c:
			f.setCurrentStyle()
			return false
		case 0x1d:
			f.resetParameters()
			return false
		case 0x1e:
			return false
		case 0x17:
			f.setValue()
		}
		// 0x17 and 0x1f: Quit_Context_ inlined. It reads the command byte
		// itself, so L and P never match; the pointer just moves on.
		b := f.ch(f.ptr)
		f.ptr.i++
		switch b {
		case 'L':
			if ps.testFlag() != 0 {
				f.previousNesting()
			}
		case 'P':
			f.quitParagraph()
			return true
		}
		return false
	}
	switch c {
	case 0:
		ps.InsertCase = 0
		if f.nest > 0 {
			f.nest--
			f.ptr = f.stack[f.nest]
			f.ptr.i--
			return false
		}
		f.completeLine()
		return true
	case 7:
		f.processFunction()
		return f.ch(f.ptr) == 0
	case 8:
		f.recordBreak(8)
	case 9:
		f.recordBreak(9)
		for i := 0; i < int(ps.tabCount()); i++ {
			if v := ps.tab(i); v > ln.x {
				ln.x = v
				break
			}
		}
	case 0x0a, 0x0d:
		if c == 0x0a {
			f.newPara = true
		}
		f.completeLine()
		f.cp = 0
		if !f.verticalMove(f.lf) {
			f.quitParagraph()
		}
	case 0x0b:
		if !f.verticalMove(f.lf) {
			f.quitParagraph()
		}
	case 0x0c:
		f.completeLine()
		f.cp = 0
		f.newPara = true
		if !f.verticalMove(f.ff) {
			f.quitParagraph()
		}
	}
	return false
}

// completeLine is Complete_Line_ (0x10478c): a fully justified line that
// ends the paragraph or is followed by CR, LF or FF is left aligned.
func (f *fmtState) completeLine() {
	f.completeLineData()
	j := f.just
	if j == 3 {
		switch b := f.ch(f.ptr); {
		case b == 0x0d || b == 0x0a || b == 0x0c, f.nest == 0 && b == 0:
			j = 0
		}
	}
	f.justifyLine(j)
	f.printLine()
}

// completeLineData is Complete_Line_Data_ (0x1047d5).
func (f *fmtState) completeLineData() {
	ln := &f.line
	ln.done = 1
	if ln.n <= 0 {
		ln.width = 0
		return
	}
	if ln.yoff != 0 {
		for i := 0; i < int(ln.n) && i < fmtMaxChars; i++ {
			g := &f.chars[i]
			g.y += ln.yoff
			if g.y < 0 {
				g.y = 0
			}
		}
		f.cp = int(ln.n)
		f.y += ln.yoff
	}
	d := int32(ln.lastR) - int32(ln.left)
	if d < 0 {
		d = -d
	}
	ln.width = int16(d)
}

// verticalMove is Vertical_Move_ (0x10494e): move down d and on to the first
// row with room; false (y unchanged) past the bottom limit.
func (f *fmtState) verticalMove(d int16) bool {
	old := f.y
	f.y = old + d
	for !f.setLineLimits() && f.y <= f.ymax {
		f.y++
	}
	if f.y > f.ymax {
		f.y = old
		return false
	}
	return true
}

// setLineLimits is Set_Line_Limits_ (0x106af3). Its exclusion-area loop
// needs _exclusion_count > 0, which nothing in the game sets, so it is not
// ported.
func (f *fmtState) setLineLimits() bool {
	ln := &f.line
	l, r := f.x+f.lm, f.x2-f.rm
	ln.yoff, ln.spaces, ln.n, ln.prevX = 0, 0, 0, 0
	f.brk.n, f.brk.spaces = 0, 0
	wr := r
	if f.newPara {
		l += f.indent
	}
	if l < r {
		ln.left, ln.x, ln.lastR = l, l, l
		ln.right, ln.wrapR, ln.wrapX = r, wr, 0
		f.lineL, f.lineR = ln.left, ln.right
		return true
	}
	ln.wrapX = 0
	ln.right, ln.x, ln.lastR = ln.left, ln.left, ln.left
	return false
}

// initLineStart is Initialize_Line_Start_ (0x10683f).
func (f *fmtState) initLineStart() {
	f.line.yoff, f.line.spaces, f.line.n, f.line.prevX = 0, 0, 0, 0
	f.brk.n, f.brk.spaces = 0, 0
}

// setValue is Set_Value_ (0x104de7): code 0x17 and the letter after it.
func (f *fmtState) setValue() {
	ln, ps := &f.line, f.ps
	cx := ln.x - f.x
	switch f.ch(fmtPtr{f.ptr.s, f.ptr.i + 1}) {
	case 'T':
		if n := ps.tabCount(); n < 10 {
			ps.setTab(int(n), ln.x)
			ps.setTabCount(n + 1)
			if n+1 > 1 {
				ps.sortTabs()
			}
		}
	case 't':
		ps.setTabCount(0)
	case 'L':
		f.lm = cx
	case 'l':
		f.lm = 0
	case 'R':
		f.rm = f.mw - cx
	case 'r':
		f.rm = 0
	case 'm':
		f.lm, f.rm = 0, 0
	case 'I':
		f.indent = cx - f.lm
	case 'i':
		f.indent = 0
	}
}

// setCurrentStyle is Set_Current_Style_ (0x104bd7): code 0x1c and a digit.
// '6' and an argument up to 7 set the per-glyph colour byte; '0'..'5' are
// stored as the ASCII digit into the shadow byte.
func (f *fmtState) setCurrentStyle() {
	st := &f.line.style
	f.ptr.i++
	d := f.ch(f.ptr)
	if d == '6' {
		if v, ok := f.nextArg(); ok && v <= 7 {
			st.color = byte(v)
		}
		return
	}
	if uint16(d)-0x30 <= 5 {
		st.shadow = d
	}
}

// resetParameters is Reset_Paramaters_ (0x104a24): code 0x1d and a digit
// whose low nibble picks flags from the table at 0x183915.
func (f *fmtState) resetParameters() {
	ln, st := &f.line, &f.line.style
	f.ptr.i++
	fl := fmtResetFlags[f.ch(f.ptr)&0xf]
	fs := f.styleOf(st.font)
	if fl&1 != 0 {
		ln.hs = int16(fs.HSpacing)
		st.widths = st.font
		st.space = int16(fs.Widths[' '])
	}
	if fl&2 != 0 {
		st.space = int16(fs.Widths[' '])
	}
	if fl&4 != 0 {
		st.base = int16(fs.Height)
		f.lf = int16(fs.VSpacing) + st.base
	}
	if fl&8 != 0 {
		f.ff = f.lf
	}
	if fl&0x10 != 0 {
		f.ps.ResetVariables()
	}
	if fl&0x20 != 0 {
		if v, ok := f.nextArg(); ok {
			f.ps.ResetVariable(int(v))
		}
	}
}

// nextArg is Get_Insert_Argument_Next_ (0x10544b): step on, skip spaces
// (unless only spaces remain) and read one base-16 digit, 0-9 or A-F.
func (f *fmtState) nextArg() (uint16, bool) {
	f.ptr.i++
	p := f.ptr
	for f.ch(p) == ' ' {
		p.i++
	}
	if f.ch(p) != 0 {
		f.ptr = p
	}
	c := toUpper(f.ch(f.ptr))
	var v uint16
	switch {
	case c >= '0' && c <= '9':
		v = uint16(c) - 0x30
	case c >= 'A' && c <= 'Z':
		v = uint16(c) - 0x37
	default:
		return 0, false
	}
	if v >= 16 {
		return 0, false
	}
	return v, true
}

// insertString is Insert_A_String_ (0x1054e1): continue reading at s, coming
// back here at its terminator; ignored past ten levels.
func (f *fmtState) insertString(s []byte) {
	if f.nest < fmtMaxNest {
		f.stack[f.nest] = f.ptr
		f.ptr = fmtPtr{s: s}
		f.nest++
	}
}

// skipArgInsert is Skip_Arg_Insert_A_String_ (0x1054b7): step past the
// argument, then Insert_A_String_.
func (f *fmtState) skipArgInsert(s []byte) {
	f.ptr.i++
	f.insertString(s)
}

// insertArgString is Insert_Argument_String_ (0x10541b).
func (f *fmtState) insertArgString(v uint16, cond bool) {
	f.ptr.i++
	if cond {
		f.insertString(f.ps.varString(v))
	}
}

// ordinal is the ordinal path of Insert_Item_ (from 0x10519c): the number,
// then the ending of its last digit raised with function ^2.
func (f *fmtState) ordinal(n int32) {
	ps := f.ps
	num := ps.itoa(n)
	d := num[len(num)-1] - '0'
	s := fmt.Sprintf("%s\a^2.%s\a^0.", num, fmtOrdEnding[d])
	ps.ordBuf = [32]byte{}
	copy(ps.ordBuf[:], s)
	f.ptr.i++
	f.insertString(ps.ordBuf[:])
}

// itoa is itoa_(n, 0x1acf98, 10) into the shared number buffer.
func (p *Paragraphs) itoa(n int32) string {
	s := strconv.Itoa(int(n))
	p.numBuf = [16]byte{}
	copy(p.numBuf[:], s)
	return s
}

// insertItem is Insert_Item_ (0x105089; letters A..T through 0x105039).
func (f *fmtState) insertItem() {
	ps := f.ps
	f.ptr.i++
	l := f.ch(f.ptr)
	var v uint16
	ok := true
	if fmtInsertClass[l] != '0' {
		v, ok = f.nextArg()
	}
	if !ok || l < 'A' || l > 'T' {
		return
	}
	switch l {
	case 'A':
		f.skipArgInsert(ps.varString(v))
	case 'B':
		n := ps.getItem(v)
		ps.setPluralFlag(n != 1)
		ps.itoa(n)
		f.ptr.i++
		f.insertString(ps.numBuf[:])
	case 'C':
		n := ps.getItem(v)
		ps.setPluralFlag(n != 1)
		f.ordinal(n)
	case 'D':
		n := ps.getItem(v)
		ps.setPluralFlag(n != 1)
		if n >= 0 && n <= 10 {
			f.ptr.i++
			f.insertString([]byte(fmtOrdWord[n]))
		} else {
			f.ordinal(n)
		}
	case 'E', 'G':
		if ps.pluralFlag() != 0 {
			f.ptr.i++
			f.insertString(map[byte][]byte{'E': fmtPluralS, 'G': fmtPluralES}[l])
		}
	case 'F', 'H':
		if ps.getItem(v) != 1 {
			f.ptr.i++
			f.insertString(map[byte][]byte{'F': fmtPluralS, 'H': fmtPluralES}[l])
		}
	case 'I', 'K', 'M', 'O':
		var cond bool
		switch l {
		case 'I':
			cond = ps.pluralFlag() != 0
		case 'K':
			cond = ps.pluralFlag() == 0
		case 'M':
			cond = ps.testFlag() != 0
		case 'O':
			cond = ps.testFlag() == 0
		}
		if cond {
			f.skipArgInsert(ps.varString(v))
		} else {
			f.ptr.i++
		}
	case 'J', 'L', 'N', 'P':
		n := ps.getItem(v)
		cond := map[byte]bool{'J': n != 1, 'L': n == 1, 'N': n != 0, 'P': n == 0}[l]
		if w, ok := f.nextArg(); ok {
			f.insertArgString(w, cond)
		}
	case 'Q':
		ps.InsertCase = 1
	case 'R':
		ps.InsertCase = 2
	case 'S':
		ps.InsertCase = 3
	case 'T':
		ps.InsertCase = 0
	}
}

// testItem is Test_Item_ (0x104c2c): code 0x19, an operator digit and up to
// two variable arguments; the result goes into the test flag.
func (f *fmtState) testItem() {
	ps := f.ps
	f.ptr.i++
	ps.setTestFlag(0)
	save := f.ptr
	op := f.ch(f.ptr)
	var v1 int32
	if a, ok := f.nextArg(); ok {
		v1 = ps.getItem(a)
	}
	next := func() int32 {
		if a, ok := f.nextArg(); ok {
			return ps.getItem(a)
		}
		return 0
	}
	switch {
	case op < '0':
		return
	case op == '0':
		ps.setTestFlag(b2u(v1 != 0))
	case op == '1':
		ps.setTestFlag(ps.pluralFlag())
		f.ptr = save
	case op == '2':
		ps.setTestFlag(b2u(ps.testFlag() == 0))
		f.ptr = save
	case op == '3' || op == '4':
		idx := uint16(v1)
		if op == '3' {
			f.ptr = save
			idx = uint16(ps.testFlag())
		}
		b, ok := f.nextArg()
		if !ok {
			return
		}
		ps.setTestFlag(b2u(ps.setItem(idx, int32(b))))
	case op == '5':
		ps.setTestFlag(b2u(v1 == next()))
	case op == '6':
		ps.setTestFlag(b2u(v1 != next()))
	case op == '7':
		ps.setTestFlag(b2u(v1 < next()))
	case op == '8':
		ps.setTestFlag(b2u(v1 > next()))
	case op == '9':
		ps.setTestFlag(b2u(v1 <= next()))
	case op == 'A':
		ps.setTestFlag(b2u(v1 >= next()))
	}
}

// atoi is Watcom atoi_ on the bytes at p: white space, a sign, digits.
func (f *fmtState) atoi(p fmtPtr) int32 {
	for {
		c := f.ch(p)
		if c != ' ' && (c < 9 || c > 13) {
			break
		}
		p.i++
	}
	neg := false
	if c := f.ch(p); c == '-' || c == '+' {
		neg = c == '-'
		p.i++
	}
	var v int32
	for c := f.ch(p); c >= '0' && c <= '9'; c = f.ch(p) {
		v = v*10 + int32(c-'0')
		p.i++
	}
	if neg {
		v = -v
	}
	return v
}

// functionArgument is Get_Function_Argument_ (0x10599e): the value, whether
// it is relative (a leading sign) and whether the argument was empty. It
// steps over every byte of "1234567890+- " and over a terminator too.
func (f *fmtState) functionArgument() (val int32, rel, none bool) {
	none = true
	p := f.ptr
	for f.ch(p) == ' ' {
		p.i++
	}
	if f.ch(p) != 0 {
		f.ptr = p
	}
	b := f.ch(f.ptr)
	rel = b == '+' || b == '-'
	val = f.atoi(f.ptr)
	for f.err == nil {
		c := f.ch(f.ptr)
		if f.err != nil || !strchr(fmtArgChars, c) {
			break
		}
		if c != ' ' {
			none = false
		}
		f.ptr.i++
	}
	return val, rel, none
}

// skipSpaces moves the pointer over spaces unless only spaces remain.
func (f *fmtState) skipSpaces() {
	p := f.ptr
	for f.ch(p) == ' ' {
		p.i++
	}
	if f.ch(p) != 0 {
		f.ptr = p
	}
}

// processFunction is Process_Function_ (0x105533): code 7, then function
// letters, each with comma-separated arguments, up to a '.'.
func (f *fmtState) processFunction() {
	ln, st, ps := &f.line, &f.line.style, f.ps
	if f.ch(f.ptr) == 7 {
		f.ptr.i++
	}
	tabsSet := false
	for f.err == nil {
		f.ylim = 0x1e0 - st.base
		c := toUpper(f.ch(f.ptr))
		if !strchr(fmtFunctions, c) {
			for {
				b := f.ch(f.ptr)
				if b == 7 || b == '.' || b == 0 || f.err != nil {
					break
				}
				f.ptr.i++
			}
			break
		}
		if c == 'T' {
			ps.setTabCount(0)
		}
		tabsSet = false
		letter := c
		si := 0
		for f.err == nil {
			f.ptr.i++
			si++
			val, rel, none := f.functionArgument()
			if f.err != nil {
				return
			}
			switch letter {
			case 'R', 'F':
				letter = f.fnSetFont(letter, val, rel)
			case 'T':
				tabsSet = true
				letter = f.fnSetTabs(letter, val, none, rel)
			case 'X':
				x := val + int32(f.x)
				if rel {
					x = val + int32(ln.x)
				}
				if x >= int32(f.x) && x <= int32(f.x2) {
					ln.x = int16(x)
				}
				letter = 'Y'
			case 'Y':
				letter = f.fnSetYPos(val, rel)
			case 'H':
				if !none {
					v := val
					if rel {
						v += int32(ln.hs)
					}
					if v >= 0 && v < 0x40 {
						ln.hs = int16(v)
					}
				}
				letter = 'S'
			case 'S':
				if none {
					st.space = ln.hs
				} else {
					v := val
					if rel {
						v += int32(st.space)
					}
					if v >= 0 && v < 0x40 {
						st.space = int16(v)
					}
				}
				letter = 0
			case 'V':
				if !none {
					v := val
					if rel {
						v += int32(f.lf)
					}
					if v > -0x40 && v < 0x40 {
						f.lf = int16(v)
					}
				}
				letter = 'P'
			case 'P':
				if none {
					f.ff = f.lf
				} else {
					v := val
					if rel {
						v += int32(f.ff)
					}
					if v > -0x40 && v < 0x40 {
						f.ff = int16(v)
					}
				}
				letter = 0
			case 'M':
				letter = f.fnSetMargins(letter, val, si, none, rel)
				f.setLineLimits()
			case 'I':
				v := val
				if rel {
					v += int32(f.indent)
				}
				l := int32(f.lm) + v
				if l+int32(f.rm) < int32(f.mw) {
					if l < 0 {
						f.indent = f.lm
					} else {
						f.indent = int16(v)
					}
				}
				f.setLineLimits()
				letter = 0
			case 'O':
				letter = f.fnSetOrigin(letter, val, si, none, rel)
			case 'C':
				letter = 0 // FN_User_Call_ with no user call set (nothing sets one)
			case '^':
				f.fnSuperscript(val)
				letter = 0
			case '=':
				letter = f.fnSetVariable(letter, val, si, none, rel)
			}
			f.skipSpaces()
			if f.ch(f.ptr) != ',' {
				break
			}
		}
		if letter == 'O' {
			f.fail(ErrFmtOrigin)
			return
		}
	}
	if tabsSet && ps.tabCount() > 1 {
		ps.sortTabs()
	}
}

// fnSetFont is FN_Set_Font_ (0x105a2c): functions F and R (R prints the
// glyphs through Set_Remap_Font_Style). A style with a higher baseline
// offset raises the line.
func (f *fmtState) fnSetFont(letter byte, val int32, rel bool) byte {
	ln, st := &f.line, &f.line.style
	if rel {
		val += int32(st.font)
	}
	if val < 0 || val >= 6 {
		return 0
	}
	old := int32(fmtBaseOffsets[st.font&7])
	st.font = byte(val)
	st.remap = b2u(letter != 'F')
	fs := f.styleOf(st.font)
	ln.hs = int16(fs.HSpacing)
	st.widths = st.font
	st.space = int16(fs.Widths[' '])
	st.base = int16(fs.Height)
	f.lf = int16(fs.VSpacing) + st.base
	f.ymax = 0x1e0 - st.base
	d := int32(fmtBaseOffsets[st.font]) - old
	if int32(ln.yoff) < d {
		ln.yoff = int16(d)
	}
	if y := int32(f.y) - d; y >= 0 {
		f.y = int16(y)
	} else {
		ln.yoff = f.y
		f.y = 0
	}
	return 0
}

// fnSetTabs is FN_Set_Tabs_ (0x105bfb).
func (f *fmtState) fnSetTabs(letter byte, val int32, none, rel bool) byte {
	ps := f.ps
	n := ps.tabCount()
	if n >= 10 {
		return 0
	}
	x := val + int32(f.x)
	if rel {
		x = val + int32(f.line.x)
	}
	if x >= int32(f.x) && x <= int32(f.x2) && !none {
		ps.setTab(int(n), int16(x))
		ps.setTabCount(n + 1)
	}
	return letter
}

// fnSetYPos is FN_Set_Y_Pos_ (0x105ca4).
func (f *fmtState) fnSetYPos(val int32, rel bool) byte {
	y := val + int32(f.y0)
	if rel {
		y = val + int32(f.y)
	}
	if y > int32(f.ylim) {
		y = int32(f.ylim)
	} else if y < 0 {
		y = 0
	}
	f.y = int16(y)
	if d := int32(f.y0) - int32(f.y); d > 0 && d > int32(f.line.yoff) {
		f.line.yoff = int16(d)
	}
	return 0
}

// fnSetMargins is FN_Set_Margins_ (0x105def): the first argument is the left
// margin, the second the right one; a third goes to function I.
func (f *fmtState) fnSetMargins(letter byte, val int32, si int, none, rel bool) byte {
	if !none {
		if uint16(si) == 1 {
			v := val
			if rel {
				v += int32(f.lm)
			}
			if v < int32(f.mw) {
				f.lm = int16(max(v, 0))
				if int32(f.x)+int32(f.lm) >= int32(f.rm)+int32(f.x2) {
					f.rm = 0
				}
			}
		} else {
			v := val
			if rel {
				v += int32(f.rm)
			}
			if v >= 0 && v < int32(f.mw)-int32(f.lm) {
				f.rm = int16(v)
			}
		}
	}
	if uint16(si) == 1 {
		return letter
	}
	return 'I'
}

// fnSetOrigin is FN_Set_Origin_ (0x105edc): x, y and width of the paragraph
// the following re-entry prints.
func (f *fmtState) fnSetOrigin(letter byte, val int32, si int, none, rel bool) byte {
	if none {
		return letter
	}
	switch uint16(si) {
	case 1:
		if rel {
			val += int32(f.x)
		}
		if val >= 0 && val < 0x280 {
			f.x = int16(val)
		}
	case 2:
		if rel {
			val += int32(f.y0)
		}
		if val >= 0 && val < 0x1e0 {
			f.y0 = int16(val)
		}
	case 3:
		if rel {
			val += int32(f.mw)
		}
		if val > 0 && val <= 0x280 {
			f.mw = int16(val)
		}
	}
	return letter
}

// fnSuperscript is FN_Superscript_ (0x105b24): ^n saves the style record
// (0x1acfa8) and measures with the style n lower; ^0 restores it.
func (f *fmtState) fnSuperscript(val int32) {
	ln, st, ps := &f.line, &f.line.style, f.ps
	if val == 0 {
		if !ps.supSaved {
			f.fail(ErrFmtStyle)
			return
		}
		*st = ps.supSave
		return
	}
	ps.supSave, ps.supSaved = *st, true
	s := int32(st.font) - val
	switch {
	case s < 0:
		st.font = 0
	case s < 6:
		st.font = byte(s)
	default:
		st.font = 5
	}
	fs := f.styleOf(st.font)
	ln.hs = int16(fs.HSpacing)
	st.widths = st.font
	st.space = int16(fs.Widths[' '])
}

// fnSetVariable is FN_Set_Variable_ (0x105ba0): =v,n sets variable v to n
// (or adds n with a sign).
func (f *fmtState) fnSetVariable(letter byte, val int32, si int, none, rel bool) byte {
	ps := f.ps
	switch uint16(si) {
	case 1:
		if !none {
			ps.varIndex = uint16(max(val, -val))
		}
		return letter
	case 2:
		if ps.varIndex < 16 {
			if rel {
				val += ps.getItem(ps.varIndex)
			}
			ps.setItem(ps.varIndex, val)
		}
	}
	return 0
}

// justifyLine is Justify_Line_ (0x105fe9): 1 right, 2 centred, 3 full; the
// spare pixels go into the first glyph's offset.
func (f *fmtState) justifyLine(typ byte) {
	ln := &f.line
	extra := ln.right - ln.left + 1 - ln.width
	if typ == 0 || ln.n <= 0 || extra <= 0 {
		return
	}
	switch typ {
	case 1:
		f.chars[0].dx += extra
	case 2:
		f.chars[0].dx += extra / 2
	case 3:
		f.justifyFully(extra)
	}
}

// justifyFully is _Justify_Line_Fully_ (0x106053): passes from the table at
// 0x18391e (by how many of the glyphs are spaces) widen spaces or every
// glyph after the first non-space one, one pixel per glyph, until extra runs
// out; after three every-glyph passes only spaces widen.
func (f *fmtState) justifyFully(extra int16) {
	ln := &f.line
	n := int(ln.n)
	var p int
	switch sp := int32(ln.spaces); {
	case sp == 0 || sp > int32(ln.n)/4:
		p = 0
	case sp > int32(ln.n)/8:
		p = 1
	default:
		p = 2
	}
	i := 0
	for i < fmtMaxChars && f.chars[i].c == ' ' && i < n {
		i++
	}
	if i == n {
		return
	}
	i++
	cx, cnt, si := extra, 0, 0
	for {
		fl := fmtJustify[p][si]
		if fl&2 != 0 {
			cnt++
			if uint16(cnt) > 3 {
				fl = 1
				if ln.spaces == 0 {
					return
				}
			}
		}
		moved := false
		for j := i; j < n; j++ {
			g := &f.chars[j]
			if g.c == ' ' && fl&1 != 0 || fl&2 != 0 {
				cx--
				g.dx++
				moved = true
				if cx <= 0 {
					break
				}
			}
		}
		if !moved && fl == 1 && cnt > 3 {
			f.fail(ErrFmtJustify)
			return
		}
		si++
		if fmtJustify[p][si] == 0 {
			si = 0
		}
		if cx <= 0 {
			return
		}
	}
}

// pickColor is the colour switch of Print_Line_ (Pick_Font_Color_ 0x10686e
// inlined): Set_Normal_, Set_Highlight_ or Set_Special_Colors_On_.
func (f *fmtState) pickColor(attr byte) {
	p := f.t.Printer
	switch attr & 0xe0 {
	case 0:
		p.Current = p.Normal
	case 0x20:
		p.Current = p.Highlight
	case 0x40:
		p.Current = p.Special
	}
}

// pickFont is Pick_Font_Number_ (0x10688c): a remapped style (bit 7) goes
// through Set_Remap_Font_Style_ with the normal colours, which remaps them
// in place; the current colours are kept.
func (f *fmtState) pickFont(fb byte) {
	p := f.t.Printer
	cur := p.Current
	if fb&0x80 != 0 {
		s := fb & 0x7f
		if s >= font.Styles {
			f.fail(ErrFmtStyle)
			return
		}
		r := f.t.e.PictureRemap()
		var m font.Colors
		for i := range m {
			m[i] = r[p.Normal[i]]
		}
		p.SetStyle(int(s), m)
	}
	p.Current = cur
}

// glyph is Print_Character_ (0x121deb) or Print_Character_To_Bitmap_
// (0x123070): one glyph in the current colours, no clipping window.
func (f *fmtState) glyph(x, y int16, c byte) {
	var err error
	if f.bitmap != nil {
		err = f.t.Printer.StringToBitmap(*f.bitmap, int(x), int(y), []byte{c}, false, 0)
	} else {
		_, err = f.t.Printer.String(f.t.target(), int(x), int(y), []byte{c}, false, 0)
	}
	if err != nil {
		f.fail(err)
	}
}

// printLine is Print_Line_ (0x106171): grow the paragraph extent by each
// glyph and, when printing, draw its shadow passes and the glyph.
func (f *fmtState) printLine() {
	ln, ps, p := &f.line, f.ps, f.t.Printer
	n := int(ln.n)
	if n <= 0 {
		return
	}
	n = min(n, fmtMaxChars)
	f.cp = 0
	if f.newPara {
		g := &f.chars[0]
		p.Shadow = font.Shadow(g.attr & 0xf)
		f.pickColor(g.attr)
		ps.lastAttr = g.attr
		f.pickFont(g.font)
		ps.lastFont = g.font
		f.newPara = false
	}
	var x int16
	for ; f.cp < n && f.err == nil; f.cp++ {
		g := &f.chars[f.cp]
		if ps.lastAttr != g.attr {
			p.Shadow = font.Shadow(g.attr & 0xf)
			if g.attr&0xe0 != ps.lastAttr&0xe0 {
				f.pickColor(g.attr)
			}
			ps.lastAttr = g.attr
		}
		if ps.lastFont != g.font {
			f.pickFont(g.font)
			ps.lastFont = g.font
		}
		x += g.dx
		ax := x
		if x > f.chars[0].dx {
			ax = x + f.width(g.font&0x7f, g.c)
		}
		if ax < ps.box(0) {
			ps.setBox(0, ax)
		} else if ax > ps.box(2) {
			ps.setBox(2, ax)
		}
		if g.y < ps.box(1) {
			ps.setBox(1, g.y)
		}
		if yb := g.y + int16(f.styleOf(g.font&0x7f).Height); yb > ps.box(3) {
			ps.setBox(3, yb)
		}
		if !f.print {
			continue
		}
		if f.t.glyphHook != nil {
			f.t.glyphHook(x, g.y, g.c)
		}
		if sf := int16(p.Shadow); sf != 0 {
			p.Current = font.Solid(p.OutlineColor)
			X, Y := x, g.y
			if sf != 2 {
				f.glyph(X+1, Y+1, g.c)
				f.glyph(X, Y+1, g.c)
				f.glyph(X+1, Y, g.c)
			}
			if sf != 1 && sf != 3 {
				f.glyph(X-1, Y, g.c)
				f.glyph(X-1, Y-1, g.c)
				f.glyph(X, Y-1, g.c)
			}
			if sf == 3 || sf == 5 {
				f.glyph(X+2, Y+2, g.c)
				f.glyph(X+1, Y+2, g.c)
				f.glyph(X+2, Y+1, g.c)
			}
			if sf > 3 {
				f.glyph(X+1, Y-1, g.c)
				f.glyph(X-1, Y+1, g.c)
			}
			if sf == 5 {
				f.glyph(X+2, Y, g.c)
				f.glyph(X, Y+2, g.c)
			}
			p.Current = p.Normal
		}
		f.glyph(x, g.y, g.c)
	}
}
