package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// Modal message popups: User_Box_ and the boxes it dispatches to
// (Message_Box_, Text_Box_, Timed_Text_Box_, Warning_Box_,
// Do_Confirmation_Box_). Spec: docs/screens/user-box.md.

// User_Box_ box types (BX).
const (
	boxMessage = iota
	boxConfirm
	boxTempTitle
	boxWarning
	boxTimed
	boxTimedNoTitle
)

// userBoxState holds the statics of MSGBOX and TEXTBOX.C.
type userBoxState struct {
	confirming     bool                 // port-only: in Do_Confirmation_Box_'s loop, for tests
	exploding      bool                 // port-only: in Message_Box_Exploding_'s loop, for tests
	textBoxing     bool                 // port-only: in a text box (Message_Box_, Text_Box_), for tests
	agentText      string               // port-only: the text of the box up, for the remote control's /state
	savedMouseList []engine.MouseRegion // _saved_mouse_list, at most 10
	oldHelpActive  bool                 // _old_help_list_active
	oldDrawFields  bool                 // _old_draw_fields_flag
	saveRefresh    int                  // _save_refresh_mode
	gendrawAF      engine.AutoFunction  // _gendraw_af
	textboxAF      engine.AutoFunction  // _textbox_af
	messageField   *engine.FieldStats   // _message_field_save
	textboxField   [4]*engine.FieldStats
	fieldSaveIx    int // _field_save_ix

	bottomY     int             // _bottom_y
	messageBoxY int             // _message_box_y
	animFrame   int             // _animation_frame
	direction   int             // _direction
	image       [3]*gfx.Picture // _image_seg[0..2]
	boxBitmap   *gfx.Picture    // _box_bitmap
	firstField  int             // _first_field
	warningBox  bool            // _warning_box
	globalVar   int8            // _global_var: timed box ticks
	tempString  string          // _temp_string: the type-2 title

}

// userBox is User_Box_ (0x7a25f). flag (the screen's draw-to-back byte) is
// set; the result is the confirmation answer for type 1, else -1.
func (g *Game) userBox(flag *bool, msg string, typ int) (int, error) {
	v, u := g.e.V, &g.ub
	res := -1
	var err error
	switch typ {
	case boxConfirm:
		v.SetRefreshFull()
		*flag = true
		var yes bool
		yes, err = g.confirmationBox(msg)
		res = 0
		if yes {
			res = 1
		}
	case boxTempTitle:
		title := u.tempString
		v.SetRefreshFull()
		*flag = true
		err = g.textBox(&title, msg)
	case boxWarning:
		v.SetRefreshFull()
		*flag = true
		err = g.warningBox(msg)
	case boxTimed:
		if u.globalVar < 0 {
			u.globalVar = 0x7e
		}
		v.SetRefreshFull()
		*flag = true
		// The title is an uninitialised stack buffer in the original.
		title := ""
		err = g.doTextBox(&title, msg, int(u.globalVar))
	case boxTimedNoTitle:
		if u.globalVar < 0 {
			u.globalVar = 0x7e
		}
		v.SetRefreshFull()
		*flag = true
		err = g.doTextBox(nil, msg, int(u.globalVar))
		*flag = true
		v.SetRefreshStencil()
		u.globalVar = 0
		return res, err
	default:
		v.SetRefreshFull()
		*flag = true
		err = g.messageBox(msg)
	}
	*flag = true
	v.SetRefreshStencil()
	return res, err
}

// mainUserBox is User_Box_(&_draw_main_screen_to_back, msg, 0), the call
// the galaxy screen makes.
func (g *Game) mainUserBox(msg string) error {
	_, err := g.userBox(&g.ms.drawToBack, msg, boxMessage)
	return err
}

// messageBox is Message_Box_ (0x77423): Do_Text_Box_ without a title.
func (g *Game) messageBox(msg string) error { return g.doTextBox(nil, msg, 0) }

// textBox is Text_Box_ (0xa5eb2).
func (g *Game) textBox(title *string, msg string) error { return g.doTextBox(title, msg, 0) }

// getMoxTextBoxBaseColors is Get_Mox_Text_Box_Base_Colors_ (0xa64ab): a
// base shade per kind, shifted by three per screen family.
func (g *Game) getMoxTextBoxBaseColors(kind int) (byte, byte) {
	c := byte(0xa0)
	switch {
	case kind == 1:
		c = 0xc0
	case kind == 2:
		c = 0xe0
	}
	k := byte(0)
	switch cs := g.currentScreen; {
	case cs == 1 || cs == 0x14 || cs == 0x19 || cs == 0x21:
		k = 2
	case cs == 9:
		k = 3
	case cs == 10:
		k = 1
	}
	c += k * 3
	return c, c + 2
}

// setRemapFontStyle is Set_Remap_Font_Style (0x120940): Set_Font_Style with
// the colours put through picture_remap_color_list; no shadow.
func (g *Game) setRemapFontStyle(style int, c font.Colors) {
	r := g.e.PictureRemap()
	var m font.Colors
	for i := range c {
		m[i] = r[c[i]]
	}
	t := g.e.Text()
	t.SetStyle(style, m)
	t.Shadow = font.ShadowNone
}

// formattedParagraphMaxHeight is Get_Formatted_Paragraph_Max_Height_
// (0x103caf, docs/engine/paragraphs.md).
func (g *Game) formattedParagraphMaxHeight(maxw int, s string) (int, error) {
	return g.e.Text().FormattedMaxHeight(maxw, s)
}

// lastFormattedParagraphMaxWidth is Get_Last_Formatted_Paragraph_Max_Width_
// (0x103d0c).
func (g *Game) lastFormattedParagraphMaxWidth() int {
	return g.e.Paragraphs().LastFormattedMaxWidth()
}

// printFormattedParagraph is Print_Formatted_Paragraph_ (0x103bc4). Its
// glyphs go through Print_Character, which ignores the clipping window.
func (g *Game) printFormattedParagraph(x, y, maxw int, s string, typ int) error {
	return g.e.Text().PrintFormatted(x, y, maxw, s, typ)
}

// print1OrMoreFormattedLinesCentered is _Print_1_Or_More_Formatted_Lines_
// Centered_ (0x77a74): the paragraph centred, its middle on y.
func (g *Game) print1OrMoreFormattedLinesCentered(x, y, maxw int, s string) error {
	t := g.e.Text()
	base := t.Font.Styles[t.Style].Height
	h, err := g.formattedParagraphMaxHeight(maxw, s)
	if err != nil {
		return err
	}
	y = int(int16(y - (h-base)/2))
	return g.printFormattedParagraph(x, y, maxw, s, engine.ParaCentered)
}

// saveMouseList is Save_Mouse_List_ (0x7a795): at most ten regions.
func (g *Game) saveMouseList() {
	l := g.e.MouseList()
	g.ub.savedMouseList = append([]engine.MouseRegion(nil), l[:min(len(l), 10)]...)
}

// restoreMouseList is Restore_Mouse_List_ (0x7a7cf).
func (g *Game) restoreMouseList() {
	g.e.SetMouseList(g.ub.savedMouseList)
	g.ub.savedMouseList = nil
}

// clearMouseBuffer is Clear_Mouse_Buffer.
func (g *Game) clearMouseBuffer() {
	g.e.Sys.MouseBuffer()
	g.e.Sys.MouseBuffer2()
}

// setDrawAllFields puts draw_fields_flag back.
func (g *Game) setDrawAllFields(on bool) {
	if on {
		g.e.EnableDrawAllFields()
	} else {
		g.e.DisableDrawAllFields()
	}
}

// remapDraw is Remap_Draw (0x12b7e1): a sprite through the picture remap
// table, clipped to the window when clipping is on; the frame advances.
func (g *Game) remapDraw(x, y int, p *gfx.Picture) error {
	v := g.e.V
	x2, y2 := x+p.Width-1, y+p.Height-1
	clipped := false
	if v.Clipping {
		if x > v.WinX2 || y > v.WinY2 || x2 < v.WinX1 || y2 < v.WinY1 {
			return nil
		}
		if x < v.WinX1 || y < v.WinY1 || x2 > v.WinX2 || y2 > v.WinY2 {
			clipped = true
			x2, y2 = min(x2, v.WinX2), min(y2, v.WinY2)
		}
	}
	if p.Kind() != gfx.KindSprite {
		return fmt.Errorf("Remap draw not support kind %d", p.Kind())
	}
	f, err := p.Frame(p.CurrentFrame)
	if err != nil {
		return err
	}
	if v.RefreshMode() > engine.RefreshMasked {
		v.AddSquare(x, y, x2, y2)
	}
	r := g.e.PictureRemap()
	if clipped {
		err = remapClippedSprite(v.Current, x, y, p, f, r, v)
	} else {
		err = remapSprite(v.Current, x, y, p, f, r)
	}
	if err != nil {
		return err
	}
	advanceFrame(p)
	return nil
}

// advanceFrame is the frame step at the end of Draw and Remap_Draw.
func advanceFrame(p *gfx.Picture) {
	p.CurrentFrame++
	if p.Mask&0x20 == 0 {
		if p.CurrentFrame >= p.Frames {
			p.CurrentFrame = int(p.RepeatFrame)
		}
		return
	}
	if int(p.RepeatFrame) < p.CurrentFrame {
		if p.CurrentFrame == int(p.RepeatFrame)+1 {
			p.CurrentFrame = 0
		} else if p.CurrentFrame >= p.Frames {
			p.CurrentFrame = p.Frames - 1
		}
	}
}

// spriteWord reads the 16-bit word at byte b of a frame, 0 past its end.
func spriteWord(f []byte, b int) int {
	if b < 0 || b+2 > len(f) {
		return 0
	}
	return int(int16(uint16(f[b]) | uint16(f[b+1])<<8))
}

// remapSprite is Remap_Draw_Animated_Sprite (0x12baeb).
func remapSprite(page []byte, x, y int, p *gfx.Picture, f []byte, r *[256]byte) error {
	row := int(uint16(spriteWord(f, 2)))
	start := (y+row)*engine.W + x
	pos := start
	w := 2 // word index
	for guard := 0; row < p.Height; guard++ {
		if guard > 1<<20 || 2*w+4 > len(f) {
			return fmt.Errorf("remap sprite frame does not end")
		}
		n, skip := spriteWord(f, 2*w), spriteWord(f, 2*w+2)
		w += 2
		if n == 0 {
			row += skip
			start += skip * engine.W
			pos = start
			continue
		}
		pos += skip
		for k := 0; k < n; k++ {
			if o := pos + k; o >= 0 && o < len(page) && 2*w+k < len(f) {
				page[o] = r[f[2*w+k]]
			}
		}
		pos += n
		w += (n + 1) / 2
	}
	return nil
}

// remapClippedSprite is Remap_Clipped_Draw_Animated_Sprite (0x12bc0b).
func remapClippedSprite(page []byte, x, y int, p *gfx.Picture, f []byte, r *[256]byte, v *engine.Video) error {
	sx, cx := 0, x
	if x < v.WinX1 {
		sx, cx = v.WinX1-x, v.WinX1
	}
	sy, cy := 0, y
	if y < v.WinY1 {
		sy, cy = v.WinY1-y, v.WinY1
	}
	aw, ah := x+p.Width-1, y+p.Height-1
	if aw < v.WinX2 {
		aw -= cx
	} else {
		aw = v.WinX2 - cx
	}
	if ah < v.WinY2 {
		ah -= cy
	} else {
		ah = v.WinY2 - cy
	}
	row := int(uint16(spriteWord(f, 2)))
	pos := cy*engine.W + x
	w := 2
	for row < sy {
		if 2*w+4 > len(f) {
			return fmt.Errorf("remap sprite frame does not end")
		}
		if n := spriteWord(f, 2*w); n == 0 {
			row += int(uint16(spriteWord(f, 2*w+2)))
		} else {
			w += (n + 1) / 2
		}
		w += 2
	}
	if sy < row {
		pos += (row - sy) * engine.W
	}
	row -= sy
	start := pos
	lineStart, lineEnd := pos+sx, pos+sx+aw+1
	for guard := 0; row < ah+1; guard++ {
		if guard > 1<<20 || 2*w+4 > len(f) {
			return fmt.Errorf("remap sprite frame does not end")
		}
		n, skip := spriteWord(f, 2*w), spriteWord(f, 2*w+2)
		w += 2
		if n == 0 {
			row += skip
			start += skip * engine.W
			lineStart, lineEnd = start+sx, start+sx+aw+1
			pos = start
			continue
		}
		pos += skip
		b := 2 * w
		end := pos + n
		if pos < lineEnd {
			if pos < lineStart {
				a := min(lineStart-pos, n)
				b += a
				pos += a
			}
			c := lineEnd - pos
			if end <= c+pos {
				c = end - pos
			}
			for k := 0; k < c; k++ {
				if o := pos + k; o >= 0 && o < len(page) && b+k < len(f) {
					page[o] = r[f[b+k]]
				}
			}
			pos += max(c, 0)
		}
		w += (n + 1) / 2
	}
	return nil
}

// triInterlacedFill is Tri_Interlaced_Fill (0x129130): rows from y1 take
// c1, c2, c3 in turn; clipped to the window when clipping is on.
func (g *Game) triInterlacedFill(x1, y1, x2, y2 int, c1, c2, c3 byte) {
	v := g.e.V
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
	if v.RefreshMode() > engine.RefreshMasked {
		v.AddSquare(x1, y1, x2, y2)
	}
	cs := [3]byte{c1, c2, c3}
	for y := y1; y <= y2; y++ {
		c := cs[(y-y1)%3]
		for x := x1; x <= x2; x++ {
			if o := y*engine.W + x; o >= 0 && o < len(v.Current) {
				v.Current[o] = c
			}
		}
	}
}

// textBoxStartup is Text_Box_Startup_ (0xa62e0); the block bookkeeping is
// memory management only.
func (g *Game) textBoxStartup() error {
	e, u := g.e, &g.ub
	g.saveMouseList()
	e.SetMouseList(engine.MouseArrow)
	u.oldHelpActive = e.HelpActive()
	e.SetHelpActive(false)
	u.textboxAF = e.GetAutoFunction()
	e.DeactivateAutoFunction()
	e.V.SetRefreshStencil()
	if err := u.textboxAF.Run(); err != nil {
		return err
	}
	u.oldDrawFields = e.DrawAllFields()
	e.EnableDrawAllFields()
	if err := e.DrawVisibleFields(); err != nil {
		return err
	}
	e.DisableDrawAllFields()
	if u.fieldSaveIx >= len(u.textboxField) {
		return fmt.Errorf("text boxes nested too deep")
	}
	u.textboxField[u.fieldSaveIx] = e.SaveFieldStats()
	u.fieldSaveIx++
	p, err := g.cached("TEXTBOX.LBX", 0)
	if err != nil {
		return err
	}
	return e.CalculatePictureRemapColors(p)
}

// textBoxCleanup is Text_Box_Cleanup_ (0xa63dd). It restores the first
// saved field set whatever the nesting, as the original does.
func (g *Game) textBoxCleanup() error {
	e, u := g.e, &g.ub
	if err := e.TogglePages(); err != nil {
		return err
	}
	u.fieldSaveIx--
	e.RestoreFieldStats(u.textboxField[0])
	e.EnableDrawAllFields()
	e.PutAutoFunction(u.textboxAF)
	e.ActivateAutoFunction()
	g.clearMouseBuffer()
	e.SetInputDelay(3)
	e.SetHelpActive(u.oldHelpActive)
	g.setDrawAllFields(u.oldDrawFields)
	g.restoreMouseList()
	return nil
}

// textBoxGetInput is Text_Box_Get_Input_ (0xa644d): wait for the button to
// come up, then for any input; with ticks, Timed_Wait_For_Input instead.
func (g *Game) textBoxGetInput(ticks int) error {
	e := g.e
	if ticks == 0 {
		for e.MouseButton() != 0 {
			e.MarkTime()
			if err := e.TogglePages(); err != nil {
				return err
			}
			if err := e.ReleaseTime(1); err != nil {
				return err
			}
		}
		g.clearMouseBuffer()
		for {
			e.MarkTime()
			if err := e.TogglePages(); err != nil {
				return err
			}
			if err := e.ReleaseTime(1); err != nil {
				return err
			}
			r, err := e.GetInput()
			if err != nil {
				return err
			}
			if r != 0 {
				return nil
			}
		}
	}
	if err := g.timedWaitForInput(ticks); err != nil {
		return err
	}
	g.clearMouseBuffer()
	e.SetInputDelay(3)
	return nil
}

// timedWaitForInput is Timed_Wait_For_Input (0x120486): a key or a button,
// or time auto-function calls. The key stays queued (Keyboard_Status only
// clears key_pressed).
func (g *Game) timedWaitForInput(t int) error {
	e := g.e
	for e.MouseButton() != 0 {
		if err := e.Sys.Sleep(0); err != nil {
			return err
		}
	}
	for n := 0; ; {
		hit := e.Sys.KeyWaiting() || e.MouseButton() != 0
		if err := e.QuickCallAutoFunction(); err != nil {
			return err
		}
		if t != 0 {
			n++
			if n == t {
				hit = true
			}
		}
		if hit {
			return nil
		}
	}
}

// boxCenteredOnXY is Box_Centered_On_XY_ (0xa6166): the frame for a text
// h pixels high, centred in the map area on the galaxy screen.
func (g *Game) boxCenteredOnXY(h int) (int, int, error) {
	e, u := g.e, &g.ub
	hh := int(int16(h + 0x55))
	var x, y int
	if g.currentScreen == 0 {
		x = 0x54
		y = (400-hh)/2 + 0x15
		if y < 6 {
			y = 5
		}
	} else {
		x = 0x82
		y = (0x1e0 - hh) / 2
	}
	y = int(int16(y))
	top, err := g.cached("TEXTBOX.LBX", 0)
	if err != nil {
		return 0, 0, err
	}
	if err := e.CalculatePictureRemapColors(top); err != nil {
		return 0, 0, err
	}
	r := e.PictureRemap()
	g.triInterlacedFill(x+5, y+5, x+0x177, hh+y-5, r[2], r[3], r[4])
	if err := g.remapDraw(x, y, top); err != nil {
		return 0, 0, err
	}
	sy := y + 0x2a
	mid, err := g.cached("TEXTBOX.LBX", 1)
	if err != nil {
		return 0, 0, err
	}
	ws := g.saveWindowStatus()
	by := int(int16(sy + h))
	e.V.SetWindow(x, sy, x+0x17c, by)
	e.V.ClipOn()
	err = g.remapDraw(x, sy, mid)
	g.restoreWindowStatus(ws)
	if err != nil {
		return 0, 0, err
	}
	bot, err := g.cached("TEXTBOX.LBX", 2)
	if err != nil {
		return 0, 0, err
	}
	if err := g.remapDraw(x, by, bot); err != nil {
		return 0, 0, err
	}
	u.bottomY = by + 0x2b
	return x, y, nil
}

// doTextBox is Do_Text_Box_ (0xa5ebf), reached as Text_Box_ (ticks 0),
// Message_Box_ (no title) and Timed_Text_Box_. title nil means none.
func (g *Game) doTextBox(title *string, msg string, ticks int) error {
	e, u := g.e, &g.ub
	u.textBoxing, u.agentText = true, msg
	defer func() { u.textBoxing = false }()
	t := e.Text()
	b0, b2 := g.getMoxTextBoxBaseColors(0)
	if err := g.textBoxStartup(); err != nil {
		return err
	}
	c5 := g.fontColors2(5, b0, b2)
	c3 := g.fontColors2(3, b0, b2)
	t.SetStyle(3, c5)
	h, err := g.formattedParagraphMaxHeight(0x153, msg)
	if err != nil {
		return err
	}
	w := g.lastFormattedParagraphMaxWidth()
	if w >= 0x153 {
		w = 0x153
	}
	x, y, err := g.boxCenteredOnXY(h)
	if err != nil {
		return err
	}
	close, err := g.picture("TEXTBOX.LBX", 3)
	if err != nil {
		return err
	}
	if title != nil {
		g.setRemapFontStyle(5, c5)
		tx, ty := x+0xbe, y+0xb
		g.setFittedFontStyle(*title, c5, 5, 2, -1, 0x17c, 3)
		style, sp := t.Style, t.Spacing
		g.setRemapFontStyle(style, g.fontColors2(style, b0, b2))
		t.SetSpacing(sp)
		if err := t.PrintCentered(tx, ty, *title); err != nil {
			return err
		}
	}
	g.setRemapFontStyle(3, c3)
	px, py := x+0x14, y+0x2a
	if title == nil {
		px = (0x16a-int(uint16(w)))/2 + x + 8
		py = (int(int16(h+0x2b))-int(uint16(h)))/2 + y + 0xc
	}
	ws := g.saveWindowStatus()
	e.V.SetWindow(x+0x14, y+0x2a, x+0x167, y+0x1a2)
	e.V.ClipOn()
	err = g.printFormattedParagraph(int(int16(px)), int(int16(py)), w, msg, engine.ParaLeft)
	g.restoreWindowStatus(ws)
	if err != nil {
		return err
	}
	e.V.CopyOffToBack()
	if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "\x1b", 0x29); err != nil {
		return err
	}
	if err := g.textBoxGetInput(ticks); err != nil {
		return err
	}
	if close.Frames > 1 {
		close.CurrentFrame = 1
	}
	if err := g.remapDraw(x+0x9e, u.bottomY-0x1b, close); err != nil {
		return err
	}
	if err := g.textBoxCleanup(); err != nil {
		return err
	}
	g.fullDrawScreen = true
	return nil
}

// messageBoxStartup is Message_Box_Startup_ (0x77961).
func (g *Game) messageBoxStartup() error {
	e, u := g.e, &g.ub
	g.saveMouseList()
	e.SetMouseList(engine.MouseArrow)
	if err := e.FastFadeIn(); err != nil {
		return err
	}
	u.gendrawAF = e.GetAutoFunction()
	u.oldHelpActive = e.HelpActive()
	e.SetHelpActive(false)
	if err := u.gendrawAF.Run(); err != nil {
		return err
	}
	u.oldDrawFields = e.DrawAllFields()
	e.EnableDrawAllFields()
	if err := e.DrawVisibleFields(); err != nil {
		return err
	}
	e.DisableDrawAllFields()
	u.messageField = e.SaveFieldStats()
	e.V.SetPageOff()
	u.saveRefresh = e.V.RefreshMode()
	e.V.SetRefreshStencil()
	u.animFrame, u.direction = 0, 1
	return nil
}

// messageBoxCleanup is Message_Box_Cleanup_ (0x779fc).
func (g *Game) messageBoxCleanup() {
	e, u := g.e, &g.ub
	e.RestoreFieldStats(u.messageField)
	e.PutAutoFunction(u.gendrawAF)
	switch u.saveRefresh {
	case engine.RefreshFull:
		e.V.SetRefreshFull()
	case engine.RefreshDifferential:
		e.V.SetRefreshDifferential()
	case engine.RefreshStencil:
		e.V.SetRefreshStencil()
	}
	g.setDrawAllFields(u.oldDrawFields)
	e.SetHelpActive(u.oldHelpActive)
	g.clearMouseBuffer()
	e.SetInputDelay(3)
	g.restoreMouseList()
}

// messageTextColors is _message_text_colors (0x182098).
var messageTextColors = font.Colors{0xa0, 0xa3, 0xa3, 0xa5, 0xab, 0xab, 0xab, 0xab}

// drawAnimatingMessageBox is Draw_Animating_Message_Box_ (0x77860): the
// warning frame pulses through frames 0..6 and back.
func (g *Game) drawAnimatingMessageBox() error {
	u := &g.ub
	y := u.messageBoxY
	p := u.image[0]
	p.CurrentFrame = min(u.animFrame, p.Frames-1)
	if u.animFrame == 0 {
		u.direction = 1
	}
	if u.animFrame == 6 {
		u.direction = -1
	}
	u.animFrame += u.direction
	if err := g.remapDraw(0x9a, y, p); err != nil {
		return err
	}
	return g.e.Draw(0xd0, y+0x25, u.boxBitmap)
}

// warningBox is Warning_Box_ (0x77433): it marks the box a warning and
// falls into Message_Box_Exploding_.
func (g *Game) warningBox(msg string) error {
	g.ub.warningBox = true
	return g.messageBoxExploding(msg)
}

// messageBoxExploding is Message_Box_Exploding_ (0x7743a): the pulsing
// warning frame with the message, until any input.
func (g *Game) messageBoxExploding(msg string) error {
	e, u := g.e, &g.ub
	u.agentText = msg
	t := e.Text()
	if err := g.messageBoxStartup(); err != nil {
		return err
	}
	t.SetStyle(4, messageTextColors)
	t.SetLF(0x12)
	p, err := g.cached("WARNING.LBX", 0)
	if err != nil {
		return err
	}
	u.image[0] = p
	if err := e.CalculatePictureRemapColors(p); err != nil {
		return err
	}
	u.boxBitmap = gfx.NewBlankBitmap(0x14b, 0xbf)
	u.messageBoxY = 0x90
	if u.firstField, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, "\x1b", 0x29); err != nil {
		return err
	}
	// The colours are an uninitialised local here; only the metrics matter.
	g.setRemapFontStyle(4, font.Colors{})
	t.SetLF(0x12)
	style := 4
	h, err := g.formattedParagraphMaxHeight(0xe2, msg)
	if err != nil {
		return err
	}
	if h > 0x78 {
		g.setRemapFontStyle(3, font.Colors{})
		style = 3
		t.SetLF(0x12)
	}
	b0, b2 := g.getMoxTextBoxBaseColors(1)
	g.setRemapFontStyle(style, g.fontColors2(style, b0, b2))
	t.SetLF(0x12)
	if err := g.remapDraw(0x9a, u.messageBoxY, p); err != nil {
		return err
	}
	if err := g.print1OrMoreFormattedLinesCentered(0xd0, 0xe8, 0xe2, msg); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	e.V.AddSquare(0, 0, 0x27f, 0x1df)
	if err := e.TogglePages(); err != nil {
		return err
	}
	u.boxBitmap = e.ScreenBitmapCapture(0xd0, u.messageBoxY+0x25, 0x1b2, u.messageBoxY+0x9d)
	e.AssignAutoFunction(g.drawAnimatingMessageBox, 2)
	u.exploding = true
	defer func() { u.exploding = false }()
	for {
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		if r != 0 {
			break
		}
		e.MarkTime()
		if err := g.drawAnimatingMessageBox(); err != nil {
			return err
		}
		// The registers left from the call before make this rectangle.
		e.V.AddSquare(5, 0x14, 0x24d, 0x1cb)
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.ReleaseTime(2); err != nil {
			return err
		}
	}
	g.messageBoxCleanup()
	return nil
}

// drawConfirmBox is Draw_Confirm_Box_ (0x778e4): the yes and no buttons,
// lit while held.
func (g *Game) drawConfirmBox() error {
	u := &g.ub
	yes, no := u.image[1], u.image[2]
	yes.CurrentFrame, no.CurrentFrame = 0, 0
	switch a := g.e.AutoInput(); {
	case a == u.firstField:
		yes.CurrentFrame = min(1, yes.Frames-1)
	case a == u.firstField+1:
		no.CurrentFrame = min(1, no.Frames-1)
	}
	if err := g.remapDraw(0xeb, 0x12e, yes); err != nil {
		return err
	}
	return g.remapDraw(0x159, 0x12e, no)
}

// confirmationBox is Do_Confirmation_Box_ (0x77658): true for yes.
func (g *Game) confirmationBox(msg string) (bool, error) {
	e, u := g.e, &g.ub
	u.agentText = msg
	t := e.Text()
	b0, b2 := g.getMoxTextBoxBaseColors(2)
	g.setRemapFontStyle(4, g.fontColors2(4, b0, b2))
	if err := g.messageBoxStartup(); err != nil {
		return false, err
	}
	for i := 0; i < 3; i++ {
		p, err := g.cached("CONFIRM.LBX", i)
		if err != nil {
			return false, err
		}
		u.image[i] = p
		if i == 0 {
			if err := e.CalculatePictureRemapColors(p); err != nil {
				return false, err
			}
		}
	}
	var err error
	if u.firstField, err = e.AddHiddenField(0xeb, 0x12e, 0x11e, 0x143, "Y", 0x29); err != nil {
		return false, err
	}
	no, err := e.AddHiddenField(0x159, 0x12e, 0x18c, 0x143, "N", 0x29)
	if err != nil {
		return false, err
	}
	if err := g.remapDraw(0xa1, 0x75, u.image[0]); err != nil {
		return false, err
	}
	for style := 4; ; {
		b0, b2 = g.getMoxTextBoxBaseColors(2)
		g.setRemapFontStyle(style, g.fontColors2(style, b0, b2))
		t.SetLF(0x12)
		style--
		h, err := g.formattedParagraphMaxHeight(0xe0, msg)
		if err != nil {
			return false, err
		}
		if h <= 0x7e || style <= 1 {
			break
		}
	}
	if err := g.print1OrMoreFormattedLinesCentered(0xcc, 0xd0, 0xe0, msg); err != nil {
		return false, err
	}
	if err := g.drawConfirmBox(); err != nil {
		return false, err
	}
	e.V.CopyOffToBack()
	e.V.AddSquare(0, 0, 0x27f, 0x1df)
	if err := e.TogglePages(); err != nil {
		return false, err
	}
	e.AssignAutoFunction(g.drawConfirmBox, 2)
	var r int
	u.confirming = true
	defer func() { u.confirming = false }()
	for {
		e.MarkTime()
		if r, err = e.GetInput(); err != nil {
			return false, err
		}
		if r == 0 {
			if err := e.TogglePages(); err != nil {
				return false, err
			}
		}
		if err := e.ReleaseTime(2); err != nil {
			return false, err
		}
		if r == u.firstField || r == no {
			break
		}
	}
	g.messageBoxCleanup()
	g.fullDrawScreen = true
	return r == u.firstField, nil
}

// logBoxError reports a box error where the caller cannot return one; a
// closed window surfaces again at the next Get_Input.
func (g *Game) logBoxError(err error) {
	if err != nil {
		g.Log("message box: %v", err)
	}
}
