package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The build queue's two refit popups (COLREFIT.C): the ship to refit, then
// its new design or the design screen. Spec: docs/screens/design-screen.md,
// "Refit".

// Addresses of COLREFIT.C's data.
const (
	refitLBX      = 0x179df0 // "refitpup.LBX"
	refitColon    = 0x179dfd // ":" between a design's name and its hull
	refitHelpList = 0x183376 // 7 entries
)

// refitState is COLREFIT.C's state.
type refitState struct {
	anims    [4]*gfx.Picture // _anims (0x1830d2)
	list     []int16         // _g_ship_list_ptr: ship indexes, -1 after the last
	shipF    [15]int         // _g_ship_field_list_ptr
	n        int16           // _n_ships_in_list (0x19fa28)
	first    int16           // _first_ship (0x1830cc)
	slider   int16           // _slider_bar_position (0x1830ce)
	esc      bool            // _esc (0x1830d0)
	scanned  int16           // _scanned_ship_index (0x1830e2)
	ship     int16           // _g_ship_n (0x19fa26)
	scannedD int16           // _scanned_ship_d_index (0x1830e4)
	lscanned int             // _lscanned_field (0x1830e6)
	saved    engine.AutoFunction
	inited   bool // the three scan globals hold their image values (-1, -1, -1000)

	fWhole, fDown, fUp, fCancel, fScroll, fNew int // _fields
	designF                                    [5]int
}

// refitInit gives the scan globals their values in the image; the popups
// only reset them in their evaluators.
func (r *refitState) refitInit() {
	if !r.inited {
		r.scanned, r.scannedD, r.lscanned, r.inited = -1, -1, -1000, true
	}
}

// refitRows is the scroll range: the list rounded up to whole rows of 5,
// one more row.
func (r *refitState) refitRows() int16 { return (r.n + 5) / 5 * 5 }

// loadRefitShipAnims is Load_Refit_Ship_Anims_ (0xc0ed4); Load_Refit2_
// Ship_Anims_ (0xc19ba) enters it with entry 4 as the backdrop.
func (g *Game) loadRefitShipAnims(bg int) error {
	r := &g.rf
	name := cStringAt(g.exe, refitLBX)
	for _, k := range []struct{ slot, entry int }{{0, bg}, {2, 1}, {1, 2}, {3, 3}} {
		p, err := g.picture(name, k.entry)
		if err != nil {
			return err
		}
		r.anims[k.slot] = p
	}
	return nil
}

// updateFirstShip is Update_First_Ship_ (0xc0f3e): the first row from the
// slider when the slider moved.
func (g *Game) updateFirstShip() {
	r := &g.rf
	n5 := r.refitRows()
	if n5 < 15 {
		r.slider, r.first = -1, 0
		return
	}
	if n5-15-r.first == r.slider {
		return
	}
	r.first = (n5 - 15 - r.slider) / 5 * 5
	g.col.fullDraw = true
	r.slider = n5 - 15 - r.first
}

// drawRefitBarIndicator is COLREFIT.C's Draw_Bar_Indicator_ (0xc0fbd). The top scales by the
// ship count, the bottom by the rounded rows.
func (g *Game) drawRefitBarIndicator() {
	r, v := &g.rf, g.e.V
	g.updateFirstShip()
	if r.n < 15 {
		return
	}
	y1 := int(int16(int(r.first)*0x14f/int(r.n) + 0x45))
	y2 := int(int16((int(r.first)+15)*0x14f/int(r.refitRows()) + 0x45))
	v.Fill(0x1d4, y1, 0x1de, y2, 0xe5)
	v.Line(0x1d4, y1, 0x1de, y1, 0xe6)
	v.Line(0x1d4, y1, 0x1d4, y2, 0xe6)
	v.Line(0x1de, y1, 0x1de, y2, 0xe4)
	v.Line(0x1d4, y2, 0x1de, y2, 0xe4)
	v.Dot(0x1d4, 0x45, 0x50)
	v.Dot(0x1de, 0x45, 0x50)
	v.Dot(0x1d4, 0x194, 0x50)
	v.Dot(0x1de, 0x194, 0x50)
}

// refitAddFields is Add_Fields_ (0xc10ea).
func (g *Game) refitAddFields() error {
	e, r := g.e, &g.rf
	none := g.eStr(0xc)
	if err := g.addShipListFields(); err != nil {
		return err
	}
	var err error
	if r.fDown, err = e.AddButtonField(0x1d5, 0x197, none, r.anims[1], g.eStr(0x89), 0x28); err != nil {
		return err
	}
	if r.fUp, err = e.AddButtonField(0x1d4, 0x2e, none, r.anims[2], g.eStr(0xb), 0x28); err != nil {
		return err
	}
	if r.fCancel, err = e.AddButtonField(0x112, 0x1b8, none, r.anims[3], none, 0x28); err != nil {
		return err
	}
	n5 := int(r.refitRows())
	if r.fScroll, err = e.AddScrollField(0x1d4, 0x45, 0, n5, 0, int(int16(n5-15)), 0xb, 0x150, &r.slider, none, 0); err != nil {
		return err
	}
	r.fWhole, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, none, 0)
	return err
}

// addShipListFields is Add_Ship_List_Fields_ (0xc1209): one field per
// cell. Whether a cell gets one goes by the list's start, not the first
// row shown.
func (g *Game) addShipListFields() error {
	r := &g.rf
	for i := 0; i < 15; i++ {
		if r.list[i] == -1 {
			r.shipF[i] = -1000
			continue
		}
		x, y := i%5*0x3c, i/5*0x3d
		f, err := g.e.AddHiddenField(x+0xa1, y+0x31, x+0xd8, y+0x69, g.eStr(0xc), 0)
		if err != nil {
			return err
		}
		r.shipF[i] = f
	}
	return nil
}

// buildShipList is Build_Ship_List_ (0xc12ab): the player's combat ships at
// the star, not moving off.
func (g *Game) buildShipList() {
	s, r := &g.state, &g.rf
	r.list = make([]int16, 500+15)
	k := 0
	for i := 0; i < 500; i++ {
		sh := &s.Ships[i]
		if int16(sh.Owner) == int16(s.PlayerNum) && sh.Status < 3 && sh.Location == g.screenData[0] && sh.Design.Type == 0 && k < 500 {
			r.list[k] = int16(i)
			k++
		}
	}
	r.n = int16(k)
	for ; k < len(r.list); k++ {
		r.list[k] = -1
	}
}

// refitColony is the colony at _screen_data's star and orbit (Planet_Index_).
func (g *Game) refitColony() int {
	s := &g.state
	pl := s.Stars[g.screenData[0]].Planets[g.screenData[1]]
	return int(s.Planets[pl].Colony)
}

// drawShip2 is Draw_Ship2_ (0xc132a): ship k in cell i, marked when it
// cannot be refitted here.
func (g *Game) drawShip2(k int16, i int) error {
	s := &g.state
	x, y := i%5*0x3c, i/5*0x3d
	if err := g.remapDrawShipCentered(x+0xbc, y+0x4d, int(k)); err != nil {
		return err
	}
	d := &s.Ships[k].Design
	var msg int
	switch {
	case g.colonyCanBuildProduct(g.refitColony(), -(k+100)) == 0:
		msg = 0x18b
	case int16(int8(d.Builder)) != int16(s.PlayerNum):
		msg = 0xd8
	case d.Picture == 0x2c:
		msg = 0x16a
	default:
		return nil
	}
	g.setColonyFontToRed(1, 1)
	return g.e.Text().SqueezePrintParagraph(x+0xa1, y+0x31+0x38/2, 0x37, 0x38, g.eStr(msg), 2, g.exe.bytes(spaceLimits, 6), true)
}

// scanShipInfo is Scan_Ship_Info_ (0xc143e): the hovered ship's paragraph.
func (g *Game) scanShipInfo(k int16) error {
	g.setColonyFontTo(1)
	p, err := g.buildShipDesignInfoFormattedParagraph(&g.state.Ships[k].Design, int(k))
	if err != nil {
		return err
	}
	return g.e.Text().SqueezePrintFormattedParagraph(0xa8, 0xf8, 0x11b, 0xa9, p, 0, g.exe.bytes(spaceLimits, 6), false)
}

// drawShipList is Draw_Ship_List_ (0xc1499).
func (g *Game) drawShipList() error {
	r := &g.rf
	g.setColonyFontTo(1)
	for i := 0; i < 15; i++ {
		if k := r.list[int(r.first)+i]; k != -1 && g.col.fullDraw {
			if err := g.drawShip2(k, i); err != nil {
				return err
			}
		}
	}
	return nil
}

// drawRefitPopup is Draw_Refit_Popup_ (0xc14e8): the build queue beneath,
// the popup and the ships once per full draw, then the bar and the
// hovered ship.
func (g *Game) drawRefitPopup() error {
	e, r := g.e, &g.rf
	if g.col.fullDraw {
		g.setColonyFont()
		e.V.SetPageOff()
		if err := r.saved.Run(); err != nil {
			return err
		}
		g.col.fullDraw = true
		if err := e.Draw(0x93, 6, r.anims[0]); err != nil {
			return err
		}
		g.drawRefitBarIndicator()
		if err := g.drawShipList(); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		g.col.fullDraw = false
	} else {
		g.drawRefitBarIndicator()
	}
	if r.scanned == -1 {
		return nil
	}
	return g.scanShipInfo(r.list[r.scanned])
}

// refitEvaluateInput is Evaluate_Input_ (0xc1576): a usable ship ends the
// popup with it; an officer aboard is offered to the pool first, and the
// ship is taken whatever the answer.
func (g *Game) refitEvaluateInput(in, scan int) (int16, error) {
	s, r := &g.state, &g.rf
	r.scanned = -1
	for i := 0; i < 15; i++ {
		k := r.list[int(r.first)+i]
		if r.shipF[i] != in || k == -1 {
			continue
		}
		sh := &s.Ships[k]
		msg := 0
		switch {
		case sh.Design.Picture == 0x2c:
			msg = 0x23f
		case g.colonyCanBuildProduct(g.refitColony(), -(k+100)) == 0:
			msg = 0x23e
		case int16(int8(sh.Design.Builder)) != int16(s.PlayerNum):
			msg = 0x23d
		}
		if msg != 0 {
			if err := g.messageBox(g.eStr(msg)); err != nil {
				return 0, err
			}
			g.col.fullDraw = true
			continue
		}
		if sh.Officer != -1 {
			ok, err := g.confirmationBox(cSprintf(g.eStr(0x212), g.leaderName(int(sh.Officer), false)))
			if err != nil {
				return 0, err
			}
			if ok {
				g.moveOfficerToPool(int(sh.Officer))
				sh.Officer = -1
			}
		}
		r.esc = true
		return k, nil
	}
	for i := 0; i < 15; i++ {
		if r.shipF[i] != scan || r.list[int(r.first)+i] == -1 {
			continue
		}
		g.pulsingBox(i%5*0x3c+0xa1, i/5*0x3d+0x31, 0x37, 0x38, 1, 2)
		r.scanned = r.first + int16(i)
	}
	n5 := r.refitRows()
	if in == r.fDown && r.list[int(r.first)+15] != -1 {
		r.first = min(r.first+5, 500)
		r.slider = n5 - 15 - r.first
		g.col.fullDraw = true
		g.updateFirstShip()
	}
	if in == r.fUp {
		r.first = max(r.first-5, 0)
		r.slider = n5 - 15 - r.first
		g.updateFirstShip()
		g.col.fullDraw = true
	}
	if in == r.fCancel {
		r.esc = true
	}
	g.refitFullDrawScreen()
	return -1, nil
}

// refitFullDrawScreen is the evaluators' common end: _FULL_DRAW_SCREEN_FLAG
// becomes a full draw.
func (g *Game) refitFullDrawScreen() {
	if g.fullDrawScreen {
		g.col.fullDraw = true
		g.fullDrawScreen = false
	}
}

// pulsingBox is Pulsing_Box_ (0xc5de7): a frame in colour 0xff, which
// cycles.
func (g *Game) pulsingBox(x, y, w, h, thickness, step int) {
	g.e.Box(x, y, w, h, thickness, 0xff)
	g.e.CyclePaletteColor(0xff, 0x13, 0x10, 9, 0x20, 0x28, 0x15, int16(step))
}

// refitRun is the popups' common body after the setup: the fields, the
// loop until _esc, then everything put back.
func (g *Game) refitRun(draw func() error, addFields func() error, eval func(in, scan int) (int16, error)) (int16, error) {
	e, r := g.e, &g.rf
	e.ClearFields()
	if err := addFields(); err != nil {
		return 0, err
	}
	e.V.SetWindow(0, 0, 0x27f, 0x1df)
	e.V.ClipOn()
	if err := draw(); err != nil {
		return 0, err
	}
	e.V.SetRefreshStencil()
	g.col.fullDraw = true
	var res int16
	for !r.esc {
		in, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return 0, err
		}
		if err := draw(); err != nil {
			return 0, err
		}
		if res, err = eval(in, scan); err != nil {
			return 0, err
		}
		if err := e.TogglePages(); err != nil {
			return 0, err
		}
	}
	e.ClearFields()
	e.DeactivateHelpList()
	e.DeactivateAutoFunction()
	e.PutAutoFunction(r.saved)
	e.V.CopyBackToOff()
	e.V.ClipOff()
	return res, nil
}

// colonyRefitPopup is Colony_Refit_Popup_ (0xc187b): the ship to refit, or
// -1.
func (g *Game) colonyRefitPopup() (int16, error) {
	e, r := g.e, &g.rf
	r.refitInit()
	g.buildShipList()
	r.slider = r.refitRows() - 15
	r.first = 0
	g.setColonyFont()
	r.esc = false
	if err := g.loadRefitShipAnims(0); err != nil {
		return 0, err
	}
	r.saved = e.GetAutoFunction()
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawRefitPopup, 1)
	e.DeactivateHelpList()
	e.SetHelpList(g.helpListAt(refitHelpList, 7))
	return g.refitRun(g.drawRefitPopup, g.refitAddFields, g.refitEvaluateInput)
}

// refitAdd2Fields is Add2_Fields_ (0xc19d6): CANCEL, the five design rows
// (named ones only), the design screen row and the whole screen.
func (g *Game) refitAdd2Fields() error {
	e, r := g.e, &g.rf
	none := g.eStr(0xc)
	var err error
	if r.fCancel, err = e.AddButtonField(0x112, 0x1b8, none, r.anims[3], none, 0x28); err != nil {
		return err
	}
	row := func(i int) (int, error) {
		return e.AddHiddenField(0xa1, (i+1)*0xb3/8+0x31, 0x1de, (i+2)*0xb3/8+0x30, none, 0x29)
	}
	for i := 0; i < 5; i++ {
		if g.selectionName(int16(-50-i)) == none {
			r.designF[i] = -1000
			continue
		}
		if r.designF[i], err = row(i); err != nil {
			return err
		}
	}
	if r.fNew, err = row(6); err != nil {
		return err
	}
	r.fWhole, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, none, 0)
	return err
}

// drawShipDList is Draw_Ship_D_List_ (0xc1b68): the designs, bright when of
// the ship's size and brighter when hovered, the design screen row and the
// title.
func (g *Game) drawShipDList() error {
	s, r := &g.state, &g.rf
	t := g.e.Text()
	pl := &s.Players[s.PlayerNum]
	size := s.Ships[r.ship].Design.Size
	for i := 0; i < 5; i++ {
		var c int
		if r.designF[i] == r.lscanned && size == pl.Designs[i].Size {
			c = 1
		}
		if size == pl.Designs[i].Size {
			c++
		}
		g.setColonyFontToRegBlue(4, c)
		name := g.selectionName(int16(-50 - i))
		if name == g.eStr(0xc) {
			continue
		}
		y := (i+1)*0xb3/8 + 0x31
		if err := t.Print(0xb7, y, name); err != nil {
			return err
		}
		hull, err := g.hullName(int(pl.Designs[i].Size), false)
		if err != nil {
			return err
		}
		if err := t.PrintRight(0x1c9, y, hull); err != nil {
			return err
		}
		if err := t.PrintCentered(0x140, y, cStringAt(g.exe, refitColon)); err != nil {
			return err
		}
	}
	c := 1
	if r.fNew == r.lscanned {
		c = 2
	}
	g.setColonyFontToRegBlue(4, c)
	if err := t.PrintCentered(0x140, 7*0xb3/8+0x31, g.eStr(0x320)); err != nil {
		return err
	}
	title := cSprintf(g.eStr(0x321), save.CString(s.Ships[r.ship].Design.Name[:]))
	g.setColonyFontToRegBlue(4, 1)
	return t.PrintCentered(0x140, 0x14, title)
}

// drawRefit2Popup is Draw_Refit2_Popup_ (0xc1d6b): the hovered design's
// paragraph, or what the design screen row does.
func (g *Game) drawRefit2Popup() error {
	e, s, r := g.e, &g.state, &g.rf
	if g.col.fullDraw {
		g.setColonyFont()
		e.V.SetPageOff()
		if err := r.saved.Run(); err != nil {
			return err
		}
		g.col.fullDraw = true
		if err := e.Draw(0x93, 6, r.anims[0]); err != nil {
			return err
		}
		if err := g.drawShipDList(); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		g.col.fullDraw = false
	} else if err := g.drawShipDList(); err != nil {
		return err
	}
	t := e.Text()
	if r.scannedD != -1 {
		g.setColonyFontTo(1)
		p, err := g.buildShipDesignInfoFormattedParagraph(&s.Players[s.PlayerNum].Designs[r.scannedD], -1)
		if err != nil {
			return err
		}
		if err := t.SqueezePrintFormattedParagraph(0xa8, 0xf8, 0x131, 0xa9, p, 0, g.exe.bytes(spaceLimits, 6), false); err != nil {
			return err
		}
	}
	if r.fNew != r.lscanned {
		return nil
	}
	g.setColonyFontToRegBlue(4, 1)
	return t.SqueezePrintFormattedParagraph(0xa8, 0xf8+0xa9/2, 0x131, 0xa9, g.eStr(0x322), 2, g.exe.bytes(spaceLimits, 6), true)
}

// refitEvaluate2Input is Evaluate2_Input_ (0xc1e74): a design of the
// ship's size goes into slot 5 (-3), the design screen row gives -2; right
// clicks explain.
func (g *Game) refitEvaluate2Input(in, scan int) (int16, error) {
	s, r := &g.state, &g.rf
	pl := &s.Players[s.PlayerNum]
	sh := &s.Ships[r.ship]
	r.scannedD, r.lscanned = -1, -1000
	for i := 0; i < 5; i++ {
		f := r.designF[i]
		if f == scan {
			r.lscanned, r.scannedD = f, int16(i)
		}
		if f == in {
			if sh.Design.Size == pl.Designs[i].Size {
				pl.Designs[bqRefitSlot] = pl.Designs[i]
				g.screenData[4] = bqRefitSlot
				r.esc = true
				return -3, nil
			}
			if err := g.messageBox(g.eStr(0x323)); err != nil {
				return 0, err
			}
			r.esc = false
		}
		if -in == f {
			title := g.eStr(0x325)
			msg := cSprintf(g.eStr(0x324), save.CString(sh.Design.Name[:]), save.CString(pl.Designs[i].Name[:]))
			if err := g.textBox(&title, msg); err != nil {
				return 0, err
			}
		}
	}
	if r.fNew == scan {
		r.lscanned = r.fNew
	}
	help := func(title, msg int) error {
		ti := g.eStr(title)
		return g.textBox(&ti, g.eStr(msg))
	}
	if r.fNew == in {
		r.esc = true
		return -2, nil
	} else if -in == r.fNew {
		if err := help(0x326, 0x327); err != nil {
			return 0, err
		}
	}
	if in == r.fCancel {
		r.esc = true
	}
	if -in == r.fWhole {
		if err := help(0x328, 0x329); err != nil {
			return 0, err
		}
	}
	if -in == r.fCancel {
		if err := help(0x32a, 0x32b); err != nil {
			return 0, err
		}
	}
	g.refitFullDrawScreen()
	return -1, nil
}

// colonyRefit2Popup is Colony_Refit2_Popup_ (0xc20af): -3 refit to a
// design (now in slot 5), -2 design a new one, else cancelled. With no
// design of the ship's size it gives -2 at once.
func (g *Game) colonyRefit2Popup(ship int16) (int16, error) {
	e, s, r := g.e, &g.state, &g.rf
	r.refitInit()
	r.ship = ship
	pl := &s.Players[s.PlayerNum]
	n := 0
	for i := 0; i < 5; i++ {
		if s.Ships[ship].Design.Size == pl.Designs[i].Size {
			n++
		}
	}
	if n == 0 {
		return -2, nil
	}
	g.setColonyFont()
	r.esc = false
	if err := g.loadRefitShipAnims(4); err != nil {
		return 0, err
	}
	r.saved = e.GetAutoFunction()
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawRefit2Popup, 1)
	return g.refitRun(g.drawRefit2Popup, g.refitAdd2Fields, g.refitEvaluate2Input)
}

// setColonyFontToRegBlue is Set_Colony_Font_To_Reg_Blue_ (0xbafc9): dim,
// regular or bright by n; other n keep the style's colours.
func (g *Game) setColonyFontToRegBlue(style, n int) {
	t := g.e.Text()
	t.SetOutlineColor(0x50, 0x50)
	if n >= 0 && n <= 2 {
		t.SetStyleShadow(style, g.colors([]uint32{0x182c36, colorsFont, colorsFontBright}[n]), font.ShadowDown)
	}
	t.SetHighlightColors(g.colors(colorsBrightBlue))
	t.SetSpecialColors(g.colors(colorsRed + uint32(n)*8))
}
