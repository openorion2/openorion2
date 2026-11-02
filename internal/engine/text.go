package engine

import "github.com/openorion2/openorion2/internal/font"

// Text is the engine's printer state (font_header and friends), drawing into
// the current page.
type Text struct {
	*font.Printer
	e *Engine
	// glyphHook, when set (tests), sees every glyph Print_Line_ prints.
	glyphHook func(x, y int16, c byte)
}

// SetFont makes f the font the engine prints with.
func (e *Engine) SetFont(f *font.Font) { e.text = &Text{Printer: &font.Printer{Font: f}, e: e} }

// Text returns the engine's printer.
func (e *Engine) Text() *Text { return e.text }

func (t *Text) target() font.Target {
	return font.Target{Pix: t.e.V.Current, Stride: W, H: H}
}

// display is Print_Display: print (through the window when clipping is on),
// then mark the area including the shadow margin when the refresh mode keeps
// scan lists. The mark is the same with or without clipping.
func (t *Text) display(x, y int, s []byte, full int) error {
	v := t.e.V
	t.Printer.Clip = nil
	if v.Clipping {
		t.Printer.Clip = &font.Window{X1: v.WinX1, Y1: v.WinY1, X2: v.WinX2, Y2: v.WinY2}
	}
	err := t.Printer.Display(t.target(), x, y, s, full)
	t.Printer.Clip = nil
	if err != nil {
		return err
	}
	ddx := 0
	switch t.Shadow {
	case font.ShadowHeavy, font.OutlineHeavy:
		ddx = 2
	case font.ShadowNone:
	default:
		ddx = 1
	}
	x1, y1 := max(x-ddx, 0), max(y-ddx, 0)
	x2 := ddx + x + t.Width(s)
	if full != 0 {
		x2 = ddx + x + full
	}
	y2 := ddx + y + t.Height()
	x2, y2 = min(x2, W-1), min(y2, H-1)
	if t.e.V.refresh > RefreshFull+1 {
		t.e.V.AddSquare(x1, y1, x2, y2)
	}
	return nil
}

// Print is Print.
func (t *Text) Print(x, y int, s string) error { return t.display(x, y, []byte(s), 0) }

// PrintCentered is Print_Centered.
func (t *Text) PrintCentered(x, y int, s string) error {
	return t.display(x-t.Width([]byte(s))/2, y, []byte(s), 0)
}

// PrintRight is Print_Right.
func (t *Text) PrintRight(x, y int, s string) error {
	return t.display(x-(t.Width([]byte(s))-1), y, []byte(s), 0)
}
