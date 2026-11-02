package game

import "github.com/openorion2/openorion2/internal/font"

// Text and box helpers of the game's utility module (module 55) and
// Print_Paragraph_Centered_Vertically_. Spec: docs/screens/text-boxes.md.

// printShadowDown is _Print_Shadow_Down_ (0x900ce): s at (x+1, y+1) with
// shade 0 kept and shades 1..7 in c, then at (x, y) in the current colours.
// Both prints are Print_, so a font shadow or outline still applies.
func (g *Game) printShadowDown(x, y int, c byte, s string) error {
	t := g.e.Text()
	saved := t.Current
	shadow := font.Colors{saved[0], c, c, c, c, c, c, c}
	t.Normal, t.Current = shadow, shadow
	if err := t.Print(int(int16(x+1)), int(int16(y+1)), s); err != nil {
		return err
	}
	t.Normal, t.Current = saved, saved
	return t.Print(int(int16(x)), int(int16(y)), s)
}

// printCenteredShadowDown is _Print_Centered_Shadow_Down_ (0x90089).
func (g *Game) printCenteredShadowDown(x, y int, c byte, s string) error {
	w := g.e.Text().Width([]byte(s))
	return g.printShadowDown(int(int16(x-w/2)), y, c, s)
}

// printRightShadowDown is _Print_Right_Shadow_Down_ (0x900b2): x minus the
// full width, one pixel left of Print_Right.
func (g *Game) printRightShadowDown(x, y int, c byte, s string) error {
	w := g.e.Text().Width([]byte(s))
	return g.printShadowDown(int(int16(x-w)), y, c, s)
}

// changeFontStyle is Change_Font_Style_ (0x8ebbb): Set_Font_Style with the
// current colours, keeping shadow_flag. Spacing and line feed are reset.
func (g *Game) changeFontStyle(style int) {
	t := g.e.Text()
	shadow := t.Shadow
	t.SetStyle(style, t.Current)
	t.Shadow = shadow
}

// printParagraphCenteredVertically is Print_Paragraph_Centered_Vertically_
// (0xc5ed3): the paragraph centred in a box of height h below y.
func (g *Game) printParagraphCenteredVertically(x, y, maxw, h int, s string, typ int) error {
	t := g.e.Text()
	mh, err := t.ParagraphMaxHeight(int(int16(maxw)), s)
	if err != nil {
		return err
	}
	py := y + int(uint16(h))/2 - int(int16(mh))/2
	return t.PrintParagraph(x, int(int16(py)), maxw, s, typ)
}

// drawHighlightBox is Draw_Highlight_Box_ (0x8f0bd): a two-pixel frame, outer
// top/right in c[3], inner top/right in c[2], outer left/bottom in c[0],
// inner left/bottom in c[1].
func (g *Game) drawHighlightBox(x, y, w, h int, c [4]byte) {
	v := g.e.V
	r, b := x+w, y+h
	v.Line(x, y, r, y, c[3])
	v.Line(r, y, r, b, c[3])
	v.Line(x+1, y+1, r-1, y+1, c[2])
	v.Line(r-1, y+1, r-1, b-1, c[2])
	v.Line(x, y+1, x, b, c[0])
	v.Line(x, b, r, b, c[0])
	v.Line(x+1, y+1, x+1, b-1, c[1])
	v.Line(x+1, b-1, r-1, b-1, c[1])
}

// drawDarkenedBox is Draw_Darkened_Box_ (0x8f055): the area darkened with
// glass 0 and a highlight frame in c, c+1, c+2, c+3, both offset by 2.
func (g *Game) drawDarkenedBox(x, y, w, h int, c byte) {
	x, y = int(int16(x+2)), int(int16(y+2))
	g.darkenFill(int(int16(x+1)), int(int16(y+1)), int(int16(w+x-1)), int(int16(h+y-1)))
	g.drawHighlightBox(x, y, w, h, [4]byte{c, c + 1, c + 2, c + 3})
}

// diagonalLineFill is Diagonal_Line_Fill_ (0x8ec70): hatch the window
// (x1, y1)-(x2, y2) with 45° lines step pixels apart in main, each flanked by
// lines in side; step must divide 640 and 480, else 16.
func (g *Game) diagonalLineFill(x1, y1, x2, y2, step int, main, side byte) {
	v := g.e.V
	wx1, wy1, wx2, wy2, clip := v.WinX1, v.WinY1, v.WinX2, v.WinY2, v.Clipping
	st := int32(int16(step))
	if st == 0 {
		st = 0x10
	}
	if (0x280/st)*st != 0x280 || int32(int16(0x1e0/st))*st != 0x1e0 {
		st = 0x10
	}
	v.SetWindow(int(int16(x1)), int(int16(y1)), int(int16(x2)), int(int16(y2)))
	v.ClipOn()
	rows, cols := int16(0x1e0/st), int16(0x280/st)
	s := int(st)
	line := func(x1, y1, x2, y2 int, c byte) { v.Line(x1, y1, x2, y2, c) }
	// Down-right lines from the top edge.
	for i := int16(0); i < cols; i++ {
		a := int(i) * s
		line(a-1, 0, a-1+0x1e0, 0x1e0, side)
		line(a+1, 0, a+1+0x1e0, 0x1e0, side)
		line(a, 0, a+0x1e0, 0x1e0, main)
	}
	// Down-left lines from the left edge.
	for i := int16(0); i < rows; i++ {
		a := int(i) * s
		line(-1, a, 0x1df-a, 0x1e0, side)
		line(1, a, 0x1e1-a, 0x1e0, side)
		line(0, a, 0x1e0-a, 0x1e0, main)
	}
	// Up-left lines into the top-left corner; the side lines end at (1, a),
	// not beside the main line (the original's bug).
	for i := int16(0); i < rows; i++ {
		a := int(i) * s
		line(a-1, 0, 1, a, side)
		line(a+1, 0, 1, a, side)
		line(a, 0, 0, a, main)
	}
	// Up-right lines from the bottom edge.
	for i := int16(0); i < cols; i++ {
		a := int(i) * s
		line(a-1, 0x1e0, a-1+0x1e0, 0, side)
		line(a+1, 0x1e0, a+1+0x1e0, 0, side)
		line(a, 0x1e0, a+0x1e0, 0, main)
	}
	v.WinX1, v.WinY1, v.WinX2, v.WinY2 = wx1, wy1, wx2, wy2 // Restore_Window_
	if clip {
		v.ClipOn()
	} else {
		v.ClipOff()
	}
}

// lbxMessage is Get_LBX_Message_ (0x8f6df): a copy of H_Message_(i).
func (g *Game) lbxMessage(i int) string { return g.hMessage(int(int16(i))) }
