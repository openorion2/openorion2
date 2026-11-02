package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// The ship design screen (screen 3, Design_Screen_): MOX\OTHERS\DESIGN.C.
// Spec: docs/screens/design-screen.md.

// Addresses of DESIGN.C's data.
const (
	dsMouseVariable = 0x17cf0c // _mouse_list_variable (1 entry)
	dsMouseArrow    = 0x17cf00 // _mouse_list_arrow
	dsHelpList      = 0x1818f8 // _static_design_screen_help_list, 13 entries
	dsLBXName       = 0x18175c // _design_lbx_name, a pointer
	dsKeyCancel     = 0x1791ec // "\x1b"
	dsKeyClear      = 0x1791ee // "L"
	dsKeyBuild      = 0x1791f0 // "B"
	dsShieldPlural  = 0x179217 // "s"
	dsMinus         = 0x179219 // "-"
	dsPlus          = 0x17921b // "+"
	dsFmtSignInt    = 0x17921d // "%s%d"
	dsFmtPercent    = 0x179222 // "%d%%"
	dsFmtNameCount  = 0x179227 // "%s (%d)"
	dsSpecialWord   = 0x17922f // "special"
	dsFmtInt        = 0x179201 // "%d"
	dsFmtRange      = 0x179204 // "%d-%d"
	dsZero          = 0x1791ff // "0"
	dsArc360        = 0x17925f // "360"
	dsComma         = 0x179263 // ", "
	dsFilterStatus  = 0x181750 // _beam/_missile/_bomb/_special_filter_button_status
	dsNoField       = -1000
)

// designState is DESIGN.C's and DESBOX.C's state.
type designState struct {
	t          designTemplate
	augmented  bool  // _use_augmented_engines
	nWeapons   uint8 // _n_weapons_loaded
	nSpecials  uint8 // _n_special_systems
	printedSpc int32 // _printed_space_avail
	printedCst int32 // _printed_cost
	origCost   int32 // _original_cost
	size       int16 // _design_size, the size buttons' variable
	sizeFlag   [6]uint8
	changed    bool  // _changed_design_name
	toBack     bool  // _draw_design_screen_to_back, never read
	firstWpn   uint8 // _first_weapon_ctr, always 0
	firstSpc   uint8 // _first_special_system_ctr, always 0
	replWait   uint8 // _replacement_wait_ctr
	repeat     int16 // Check_For_Auto_Repeat_'s held-button count (0x181760)
	incr       int8  // _incr_value
	waitCtr    int16 // DESBOX.C's _wait_ctr
	sizeFld    int16 // _design_size_field, set to -1000 and never read

	// Fields (-1000 for none).
	sizeF                         [6]int
	wClear, wMinus, wPlus, wField [8]int
	sClear, sField                [8]int
	iconLf, iconRt, nameF         int
	shieldF, compF                int
	cancelF, clearF, buildF       int
	debugF                        int

	pics designPictures
	box  designBoxState

	specialDesc [0x27][]byte // _special_desc_string: TECHDESC.LBX entry 1
	weaponNotes [0x28][]byte // _weapon_notes_string: entry 3
	statusInit  bool
}

// designPictures are the DESIGN.LBX pictures Load_Design_Screen_Pictures_
// loads into _screen_seg.
type designPictures struct {
	screen, iconLf, iconRt, clear, cancel, build *gfx.Picture
	size, darkSize                               [6]*gfx.Picture
	darkBuild, plus, minus, up, down             *gfx.Picture
	dullPlus, dullMinus, dullUp, dullDown        *gfx.Picture
	scrollBar                                    *gfx.Picture
	beam, missile, bomb, special                 *gfx.Picture
	wCancel, wAccept, wTop, wMid, wBot, wSel     *gfx.Picture
	rack                                         *gfx.Picture
	arcPict, arcText, rackText                   [5]*gfx.Picture
	blankMods                                    *gfx.Picture
	wSpecial                                     [18]*gfx.Picture
	sysBar, sysTop, sysMid, sysBot, sysCancel    *gfx.Picture
}

// designScreen is Design_Screen_ (0x6b9b2), screen 3. Its boxes leave with
// _current_screen 3, so it runs again without reloading (the second path).
func (g *Game) designScreen() error {
	e, s, d := g.e, &g.state, &g.des
	if !d.statusInit {
		// The filter buttons' statuses are initialised data of DESBOX.C.
		copy(d.box.filterOn[:], g.exe.bytes(dsFilterStatus, 4))
		d.statusInit = true
	}
	e.V.SetRefreshFull()
	g.ms.multiColoredLineStart = 0
	d.incr = 1
	d.firstWpn, d.firstSpc, d.replWait = 0, 0, 0
	slot := g.screenData[4]
	d.augmented = true
	e.AssignAutoFunction(g.drawDesignScreen, 2)
	e.ClearFields()
	d.sizeFld = dsNoField
	e.SetMouseList(g.exeMouseList(dsMouseVariable, 1))
	p := int(s.PlayerNum)
	if g.bq.firstCall {
		d.toBack = true
		if err := g.loadDesignScreenPictures(); err != nil {
			return err
		}
		g.buildInitialDesignTemplate(p, int(slot))
		d.origCost = g.totalDesignCost()
		if g.bq.refitAdd {
			d.t.set32(dtCost, 0)
		}
		d.printedSpc = d.t.spaceLeft()
		d.printedCst = d.t.i32(dtCost)
		d.size = d.t.size()
		e.ClearFields()
		d.sizeFld = dsNoField
		if err := g.addDesignButtons(); err != nil {
			return err
		}
		e.DisableDrawAllFields()
		e.V.SetRefreshFull()
		if err := e.FastFadeOut(); err != nil {
			return err
		}
		if err := e.LoadPalette(4, 0, 0xff); err != nil {
			return err
		}
		e.V.SetPaletteChanges(0, 0xff)
		if err := g.loadPlayerShipPalette(p); err != nil {
			return err
		}
		e.RefreshPalette()
		if err := e.DarkenPalette(100); err != nil {
			return err
		}
		e.V.SetPageOff()
		if err := g.drawDesignScreen(); err != nil {
			return err
		}
		e.V.ClearPaletteChanges(0, 0xff)
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.FastFadeIn(); err != nil {
			return err
		}
		e.EnableDrawAllFields()
		g.bq.firstCall = false
		e.V.ResetWindow()
		e.V.ClipOff()
	} else {
		d.changed = false
		if err := g.loadPlayerShipPalette(p); err != nil {
			return err
		}
		e.V.ResetWindow()
		e.V.ClipOff()
		d.size = d.t.size()
		e.ClearFields()
		d.sizeFld = dsNoField
		if err := g.addDesignButtons(); err != nil {
			return err
		}
		d.toBack = true
		e.V.SetPageOff()
		d.printedSpc = d.t.spaceLeft()
		d.printedCst = d.t.i32(dtCost)
		if err := g.drawDesignScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	e.SetMouseList(g.exeMouseList(dsMouseArrow, 1))
	g.setDesignScreenHelpList(d.t.i16(dtNWeapons), d.t.i16(dtNSpecials))
	leave := int16(0)
	rebuild := true
	for leave == 0 {
		e.MarkTime()
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return err
		}
		g.ms.scannedField = int16(scan)
		if !d.changed && string(d.t.name()) != string(g.designName(p, int(slot))) {
			d.changed = true
		}
		if r == d.nameF {
			rebuild = false
		} else if r != 0 {
			rebuild = true
		}
		if r == d.cancelF {
			leave = 1
			g.screenData[4] = 0
			g.currentScreen = g.returnScreen
		}
		if r == d.clearF {
			g.clearDesign()
			g.setDesignScreenHelpList(d.t.i16(dtNWeapons), d.t.i16(dtNSpecials))
		}
		if r == d.buildF {
			g.updatePlayerDesign(p, int(slot))
			leave = 1
			g.screenData[4] = 1
			g.currentScreen = g.returnScreen
		}
		// The debug field only formats "cstdes%d.log" into a local buffer.
		recalc := false
		if err := g.checkGenericReplacementFields(r, &recalc, &leave); err != nil {
			return err
		}
		if err := g.checkWeaponFields(r, &recalc, &leave); err != nil {
			return err
		}
		if err := g.checkSpecialSystemFields(r, &recalc, &leave); err != nil {
			return err
		}
		g.checkPlusMinusButtons(r)
		t := &d.t
		if d.size != t.size() {
			t.set16(dtSize, d.size)
			t.set32(dtSpaceTotal, int32(g.exeI16(hullSpaceTbl+uint32(int32(d.size)*0x24))))
			recalc = true
			if t.size() == 5 {
				t.set16(dtPicture, 0x2b)
			} else {
				t.set16(dtPicture, t.size()<<3)
			}
		}
		done := false
		for i := 0; i < 8 && !done; i++ {
			k := int(int16(uint16(d.firstWpn) + uint16(i)))
			if r == d.wPlus[i] {
				sp := g.weaponSpace(p, int(t.weapon(k)), t.count(k)+1, int16(t.arc(k)), int16(t.ammo(k)), t.mods(k))
				v := int16(sp) - int16(t.wspace(k)) + int16(t.i32(dtSpaceUsed))
				done = true
				if int32(v) <= t.i32(dtSpaceTotal) {
					t.set16(dtCount+2*k, t.count(k)+1)
					if t.count(k) > 0x63 {
						t.set16(dtCount+2*k, 0x63)
					}
					d.replWait = 0
					recalc = true
				}
			} else if r == d.wMinus[i] {
				t.set16(dtCount+2*k, t.count(k)-1)
				if t.count(k) < 1 {
					t.set16(dtCount+2*k, 1)
				}
				recalc = true
				d.replWait = 0
			}
		}
		if recalc {
			g.updateCalculatedDesignData(p)
		}
		if rebuild {
			e.ClearFields()
			d.sizeFld = dsNoField
			if err := g.addDesignButtons(); err != nil {
				return err
			}
		}
		for i := 0; i < 6; i++ {
			if r != d.sizeF[i] {
				continue
			}
			if d.sizeFlag[i] > 0xf0 {
				if err := g.printShipSizeErrorMessage(i); err != nil {
					return err
				}
				continue
			}
			pic := t.i16(dtPicture) % 8
			d.changed = false
			g.getShipDesignNameInto(s.Players[p].Race, int(t.size()), int(pic), t[:])
		}
		switch leave {
		case 0:
			e.V.SetPageOff()
			d.printedSpc = t.spaceLeft()
			d.printedCst = t.i32(dtCost)
			fallthrough
		case 0x63:
			if leave == 0x63 {
				e.V.SetPageOff()
			}
			if err := g.drawDesignScreen(); err != nil {
				return err
			}
			if err := e.TogglePages(); err != nil {
				return err
			}
		}
		trimName(t[:])
		if t[0] == 0 {
			t.setName(g.designName(p, int(slot)))
		}
		if err := e.ReleaseTime(2); err != nil {
			return err
		}
	}
	e.ClearFields()
	g.ms.reloadedShipPalette = false
	if g.currentScreen != 3 {
		g.playBackgroundMusic() // 0x6c076
	}
	return nil
}

// getShipDesignNameInto is Get_Ship_Design_Name_ (0x6c897) into a buffer.
func (g *Game) getShipDesignNameInto(race uint8, size, idx int, dst []byte) {
	a, err := g.archive("SHIPNAME.LBX")
	if err != nil {
		g.Log("ship name: %v", err)
		return
	}
	t, err := a.Table(0)
	if err != nil {
		g.Log("ship name: %v", err)
		return
	}
	strcpyBytes(dst, t.Record(int(int16(int(race)*0x30+size*8+idx))))
}

// loadDesignScreenPictures is Load_Design_Screen_Pictures_ (0x6c98d).
func (g *Game) loadDesignScreenPictures() error {
	d, ps := &g.des, &g.des.pics
	name := cStringAt(g.exe, g.exe.u32(dsLBXName))
	var err error
	n := 0
	next := func() *gfx.Picture {
		if err != nil {
			return nil
		}
		var p *gfx.Picture
		p, err = g.picture(name, n)
		n++
		return p
	}
	ps.screen, ps.iconLf, ps.iconRt, ps.clear, ps.cancel, ps.build = next(), next(), next(), next(), next(), next()
	for i := range ps.size {
		ps.size[i] = next()
	}
	n = 6
	for i := range ps.darkSize {
		ps.darkSize[i] = next()
	}
	ps.darkBuild, ps.plus, ps.minus, ps.up, ps.down = next(), next(), next(), next(), next()
	ps.dullPlus, ps.dullMinus, ps.dullUp, ps.dullDown = next(), next(), next(), next()
	ps.scrollBar = next()
	ps.beam, ps.missile, ps.bomb, ps.special = next(), next(), next(), next()
	ps.wCancel, ps.wAccept, ps.wTop, ps.wMid, ps.wBot, ps.wSel = next(), next(), next(), next(), next(), next()
	ps.rack = next()
	for i := range ps.arcPict {
		ps.arcPict[i] = next()
	}
	for i := range ps.arcText {
		ps.arcText[i] = next()
	}
	for i := range ps.rackText {
		ps.rackText[i] = next()
	}
	ps.blankMods = next()
	for i := range ps.wSpecial {
		ps.wSpecial[i] = next()
	}
	ps.sysBar, ps.sysTop, ps.sysMid, ps.sysBot, ps.sysCancel = next(), next(), next(), next(), next()
	if err != nil {
		return err
	}
	d.t = designTemplate{}
	techDesc := g.techDescriptionLBXName()
	a, err := g.archive(techDesc)
	if err != nil {
		return err
	}
	t1, err := a.Table(1)
	if err != nil {
		return err
	}
	for i := range d.specialDesc {
		d.specialDesc[i] = cBytes(t1.Record(i))
	}
	t3, err := a.Table(3)
	if err != nil {
		return err
	}
	for i := range d.weaponNotes {
		d.weaponNotes[i] = cBytes(t3.Record(i))
	}
	return nil
}

// cBytes is a C string's bytes.
func cBytes(b []byte) []byte { return append([]byte(nil), b[:cstrLen(b)]...) }

// techDescriptionLBXName is Get_Tech_Description_Lbx_Name_ (0x6f23c).
func (g *Game) techDescriptionLBXName() string {
	addr := []uint32{0x179266, 0x179273, 0x17927f, 0x17928b, 0x179297}
	lang := int(int16(g.settings[0xd2]))
	if lang < 1 || lang > 4 {
		lang = 0
	}
	return cStringAt(g.exe, addr[lang])
}

// setDesignScreenHelpList is Set_Design_Screen_Help_List_ (0x6f544): the
// weapon and special areas' help entries end at their last row.
func (g *Game) setDesignScreenHelpList(nw, ns int16) {
	l := g.helpListAt(dsHelpList, 0xd)
	if nw > 0 {
		l[6].Y1 = min(l[6].Y2, int(nw)*0xe+0xf+l[6].Y1)
	}
	if ns > 0 {
		l[7].Y1 = min(l[7].Y2, int(ns)*0xe+0xf+l[7].Y1)
	}
	g.e.DeactivateHelpList()
	g.e.SetHelpList(l)
}

// addSizeButton is Add_Size_Button_ (0x6a3e5): 0xf4 while refitting.
func (g *Game) addSizeButton() uint8 {
	if g.bq.refitAdd {
		return 0xf4
	}
	return 1
}

// shipSizeButtonXY is Get_Ship_Size_Button_Field_XY_ (0x6c8f9).
func shipSizeButtonXY(i int) (x1, y1, x2, y2 int) {
	ys := [6][2]int{{0x36, 0x45}, {0x46, 0x54}, {0x55, 0x66}, {0x67, 0x75}, {0x76, 0x84}, {0x85, 0x95}}
	if i < 0 || i > 5 {
		return 0, 0, 0, 0
	}
	return 0x76, ys[i][0], 0xe3, ys[i][1]
}

// addDesignButtons is Add_Design_Buttons_ (0x69e62).
func (g *Game) addDesignButtons() error {
	e, d := g.e, &g.des
	t := &d.t
	ps := &d.pics
	none := ""
	var err error
	for i := 0; i < 6; i++ {
		d.sizeFlag[i] = g.addSizeButton()
	}
	for i := 0; i < 6; i++ {
		x1, y1, x2, y2 := shipSizeButtonXY(i)
		if d.sizeFlag[i] == 1 {
			d.sizeF[i], err = e.AddMultiButtonField(x1, y1, none, ps.size[i], &d.size, int16(i), none, 0x29)
		} else {
			d.sizeF[i], err = e.AddHiddenField(x1, y1, x2, y2, none, 0x29)
		}
		if err != nil {
			return err
		}
	}
	repl := g.replacementExists(3)
	y2 := 0xb6
	for i := 0; i < 8; i++ {
		k := int(int16(uint16(d.firstWpn) + uint16(i)))
		y := 0xa9 + 0xd*i
		d.wClear[i] = dsNoField
		if (i <= 0 || t.weapon(k) <= 0) && !repl {
			d.wMinus[i], d.wPlus[i], d.wField[i] = dsNoField, dsNoField, dsNoField
			y2 += 0xd
			continue
		}
		if t.weapon(k) > 0 && t.count(k) > 1 {
			if d.wMinus[i], err = e.AddButtonField(0x37, y, none, ps.minus, none, 0x28); err != nil {
				return err
			}
		} else {
			d.wMinus[i] = dsNoField
			if t.weapon(k) > 0 {
				if _, err := e.AddHiddenField(0x37, y, 0x44, y+0xc, none, 0); err != nil {
					return err
				}
			}
		}
		old := int16(t.wspace(k))
		sp := int16(g.weaponSpace(int(g.state.PlayerNum), int(t.weapon(k)), t.count(k)+1, int16(t.arc(k)), int16(t.ammo(k)), t.mods(k)))
		if t.weapon(k) > 0 && t.i32(dtSpaceTotal)-(int32(sp)-int32(old)+t.i32(dtSpaceUsed)) >= 0 {
			if d.wPlus[i], err = e.AddButtonField(0x13, y, none, ps.plus, none, 0x28); err != nil {
				return err
			}
		} else {
			d.wPlus[i] = dsNoField
			if t.weapon(k) > 0 {
				if _, err := e.AddHiddenField(0x13, y, 0x20, y+0xc, none, 0x29); err != nil {
					return err
				}
			}
		}
		if t.weapon(k) > 0 {
			d.wField[i], err = e.AddHiddenField(0x4d, y, 0x22c, y2, none, 0x29)
		} else {
			d.wField[i], err = e.AddHiddenField(0x10, y, 0x26f, y2, none, 0x29)
		}
		if err != nil {
			return err
		}
		y2 += 0xd
	}
	replS := g.replacementExists(9)
	for i := 0; i < 8; i++ {
		d.sClear[i] = dsNoField
		if !replS {
			d.sField[i] = dsNoField
			continue
		}
		if d.sField[i], err = e.AddHiddenField(0x11, 0xd*i+0x13f, 0x26f, 0xd*i+0x13f+0xd, none, 0x29); err != nil {
			return err
		}
	}
	if d.iconLf, err = e.AddButtonField(0x11, 0x52, none, ps.iconLf, none, 0x28); err != nil {
		return err
	}
	if d.iconRt, err = e.AddButtonField(0x5e, 0x52, none, ps.iconRt, none, 0x28); err != nil {
		return err
	}
	tx := e.Text()
	tx.SetStyle(4, g.fontColors2(4, 0x55, 0x59))
	h := tx.Height()
	if d.nameF, err = e.AddContinuousStringInputField(0x12, int(int16((0x1b-h)/2+0x11)), 0x86, int(int16(h+2)), t[:], 0xf, 0, nil, 0x29, 0); err != nil {
		return err
	}
	if d.shieldF, err = e.AddHiddenField(0x1b5, 0x38, 0x273, 0x5f, none, 0x29); err != nil {
		return err
	}
	if d.compF, err = e.AddHiddenField(0x1b5, 0x61, 0x273, 0x7b, none, 0x29); err != nil {
		return err
	}
	if d.cancelF, err = e.AddButtonField(0x1cd, 0x1bb, none, ps.cancel, cStringAt(g.exe, dsKeyCancel), 0x28); err != nil {
		return err
	}
	if d.clearF, err = e.AddButtonField(0x176, 0x1bb, none, ps.clear, cStringAt(g.exe, dsKeyClear), 0x28); err != nil {
		return err
	}
	if t.i32(dtSpaceTotal) < t.i32(dtSpaceUsed) {
		d.buildF = dsNoField
	} else if d.buildF, err = e.AddButtonField(0x223, 0x1bb, none, ps.build, cStringAt(g.exe, dsKeyBuild), 0x28); err != nil {
		return err
	}
	if d.debugF, err = e.AddHiddenField(0, 0x1d6, 0xa, 0x1df, none, 0x29); err != nil {
		return err
	}
	_, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, none, 0)
	return err
}

// replacementExists is Replacement_Exists_ (0x6e1dc): a known application
// of the type (shields and type 5 count one more, for "none").
func (g *Game) replacementExists(typ int16) bool {
	var n int16
	if typ == 2 || typ == 5 {
		n = 1
	}
	pl := &g.state.Players[g.state.PlayerNum]
	for a := 0; a < 0xd4; a++ {
		if pl.AppStatus[a] == 3 && int16(g.exeU8(appTypeTbl+uint32(a)*0xd)) == typ {
			n++
		}
	}
	return n > 0
}

// checkGenericReplacementFields is Check_Generic_Replacement_Fields_
// (0x6ad7c): the shield and computer boxes.
func (g *Game) checkGenericReplacementFields(r int, recalc *bool, leave *int16) error {
	d := &g.des
	p := int(g.state.PlayerNum)
	if r == d.compF {
		if uint8(g.bestComputer(p)) == 0 {
			if err := g.warningBox(g.hMessage(0x50)); err != nil {
				return err
			}
		} else {
			d.box.typ = 8
			d.box.item = d.t.i16(dtComputer)
			d.printedCst -= int32(d.t.i16(dtCompCost))
			if err := g.genericReplacementBox(); err != nil {
				return err
			}
			d.t[dtHelpRefresh] = 1
			*recalc = true
			g.currentScreen = ScreenDesign
			*leave = 0x63
		}
	}
	if r != d.shieldF {
		return nil
	}
	msg := 0x51
	if !d.t.hasSpecial(specDamper) {
		if uint8(g.bestShipShield(p)) != 0 {
			d.box.item = d.t.i16(dtShield)
			d.box.typ = 2
			d.printedCst -= int32(d.t.i16(dtShieldCost))
			d.printedSpc += int32(d.t.i16(dtShieldSpace))
			if err := g.genericReplacementBox(); err != nil {
				return err
			}
			d.t[dtHelpRefresh] = 1
			*recalc = true
			g.currentScreen = ScreenDesign
			*leave = 0x63
			return nil
		}
		msg = 0x52
	}
	return g.warningBox(g.hMessage(msg))
}

// checkPlusMinusButtons is Check_Plus_Minus_Buttons_ (0x6ae96): the arrows
// beside the ship cycle its picture through the hull's eight; an unchanged
// name follows the picture.
func (g *Game) checkPlusMinusButtons(r int) {
	d := &g.des
	t := &d.t
	race := g.state.Players[g.state.PlayerNum].Race
	var idx int16
	switch r {
	case d.iconLf:
		lo := t.size() << 3
		if t.size() != 5 {
			t.set16(dtPicture, t.i16(dtPicture)-1)
			if lo > t.i16(dtPicture) {
				t.set16(dtPicture, lo+7)
			}
			idx = t.i16(dtPicture) % 8
		}
	case d.iconRt:
		lo := t.size() << 3
		if t.size() != 5 {
			t.set16(dtPicture, t.i16(dtPicture)+1)
			if lo+7 < t.i16(dtPicture) {
				t.set16(dtPicture, lo)
			}
			idx = t.i16(dtPicture) % 8
		}
	default:
		return
	}
	if !d.changed {
		g.getShipDesignNameInto(race, int(t.size()), int(idx), t[:])
	}
}

// checkSpecialSystemFields is Check_Special_System_Fields_ (0x6af9b): a
// special row opens the special systems box on it, an empty row on the
// first free slot; a right click shows the special's description.
func (g *Game) checkSpecialSystemFields(r int, recalc *bool, leave *int16) error {
	d := &g.des
	t := &d.t
	for i := 0; i < 8; i++ {
		slot := int16(uint16(d.firstSpc) + uint16(i))
		if int(g.ms.scannedField) == d.sField[i] {
			g.ms.scannedField = int16(i) + 2000
		}
		if r == d.sField[i] {
			if t.special(int(slot)) == 0 {
				found := false
				for k := 0; k < 8 && !found; k++ {
					if t.special(k) == 0 {
						found = true
						slot = int16(k)
					}
				}
			}
			d.box.slot = slot
			d.box.typ = 9
			d.printedSpc += int32(t.i16(dtSpecialSpc + 2*int(slot)))
			d.printedCst -= int32(t.i16(dtSpecialCost + 2*int(slot)))
			g.e.DeactivateHelpList()
			if err := g.specialSystemsBox(); err != nil {
				return err
			}
			t[dtHelpRefresh] = 1
			g.currentScreen = ScreenDesign
			*leave = 0x63
		} else if r == -d.sField[i] {
			if sp := t.special(i); sp > 0 {
				if err := g.applicationTextBox(g.exeI16(specAppTbl+uint32(int32(sp)*0x2f)), walksSpecials+1+int(sp)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// checkWeaponFields is Check_Weapon_Fields_ (0x6b0ef): a weapon row opens
// the weapons box on its slot, an empty row on the first empty slot.
func (g *Game) checkWeaponFields(r int, recalc *bool, leave *int16) error {
	d := &g.des
	t := &d.t
	for i := 0; i < 8; i++ {
		slot := int16(uint16(d.firstWpn) + uint16(i))
		if int(g.ms.scannedField) == d.wField[i] {
			g.ms.scannedField = int16(i) + 1000
		}
		if r == d.wField[i] {
			g.clearMouseBuffer()
			g.e.SetInputDelay(0x14)
			d.box.slot = slot
			// The empty test reads row i, not slot first+i.
			if t.weapon(i) == 0 {
				found := false
				for k := 0; k < 8 && !found; k++ {
					if t.weapon(k) == 0 {
						found = true
						d.box.slot = int16(k)
					}
				}
			}
			k := int(d.box.slot)
			d.box.typ = 3
			d.printedSpc += t.wspace(k)
			d.printedCst -= t.i32(dtWeaponCost + 4*k)
			g.e.DeactivateHelpList()
			if err := g.weaponsReplacementBox(); err != nil {
				return err
			}
			t[dtHelpRefresh] = 1
			g.currentScreen = ScreenDesign
			*leave = 0x63
		} else if r == -d.wField[i] {
			if w := t.weapon(i); w > 0 {
				if err := g.applicationTextBox(g.exeI16(wpnAppTbl+uint32(int32(w)*0x1c)), walksWeapons+1+int(w)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// drawDesignScreen is Draw_Design_Screen_ (0x6c6cd), also the screen's
// auto function. Play_Streaming_Music_ is not ported.
func (g *Game) drawDesignScreen() error {
	if err := g.e.Draw(0, 0, g.des.pics.screen); err != nil {
		return err
	}
	if err := g.drawDownButtons(); err != nil {
		return err
	}
	if err := g.printCurrentDesign(); err != nil {
		return err
	}
	g.checkForAutoRepeat()
	g.playStreamingMusic(8, -1, true) // 0x6c6fc (and Design_Screen_'s own draws)
	return nil
}

// drawDownButtons is Draw_Down_Buttons_ (0x6c704): dull +/- buttons, the
// dark BUILD, and the size buttons that cannot be picked.
func (g *Game) drawDownButtons() error {
	e, d := g.e, &g.des
	ps := &d.pics
	for i := 0; i < 8; i++ {
		k := int(int16(uint16(d.firstWpn) + uint16(i)))
		if d.wMinus[i] == dsNoField && d.t.weapon(k) > 0 {
			if err := e.Draw(0x37, 0xd*i+0xa9, ps.dullMinus); err != nil {
				return err
			}
		}
		if d.wPlus[i] == dsNoField && d.t.weapon(k) > 0 {
			if err := e.Draw(0x13, 0xd*i+0xa9, ps.dullPlus); err != nil {
				return err
			}
		}
	}
	if d.buildF == dsNoField {
		if err := e.Draw(0x220, 0x1b7, ps.darkBuild); err != nil {
			return err
		}
	}
	for i := 0; i < 6; i++ {
		if g.bq.refitAdd {
			if int16(i) != d.t.size() {
				continue
			}
		} else if d.sizeFlag[i] == 1 {
			continue
		}
		x, y, _, _ := shipSizeButtonXY(i)
		ps.darkSize[i].CurrentFrame = 0
		if err := e.Draw(x, y, ps.darkSize[i]); err != nil {
			return err
		}
		if g.bq.refitAdd {
			return nil
		}
	}
	return nil
}

// checkForAutoRepeat is Check_For_Auto_Repeat_ (0x6ab9a): holding the left
// button on + or - repeats it.
func (g *Game) checkForAutoRepeat() {
	d := &g.des
	t := &d.t
	btn := g.e.MouseButton()
	if btn != 1 {
		d.repeat = 0
		return
	}
	changed := false
	d.repeat += int16(btn)
	f := g.e.AutoInput()
	done := false
	for i := 0; i < 8 && !done; i++ {
		k := int(int16(uint16(d.firstWpn) + uint16(i)))
		if d.wPlus[i] != dsNoField && f == d.wPlus[i] && t.weapon(i) > 0 && d.repeat > 4 {
			sp := g.weaponSpace(int(g.state.PlayerNum), int(t.weapon(i)), 1, int16(t.arc(i)), int16(t.ammo(i)), t.mods(i))
			done = true
			if int32(int16(sp))+t.i32(dtSpaceUsed) <= t.i32(dtSpaceTotal) {
				t.set16(dtCount+2*k, t.count(k)+1)
				if t.count(k) > 0x63 {
					t.set16(dtCount+2*k, 0x63)
				}
				changed = true
			}
			continue
		}
		if d.wMinus[i] != dsNoField && f == d.wMinus[i] && t.weapon(i) > 0 && d.repeat > 5 {
			t.set16(dtCount+2*k, t.count(k)-1)
			done = true
			if t.count(k) < 1 {
				t.set16(dtCount+2*k, 1)
			}
			changed = true
		}
	}
	d.replWait++
	if changed {
		g.updateCalculatedDesignData(int(g.state.PlayerNum))
	}
}

// setFlashingColors is Set_Flashing_Colors_ (0x6e240): the current colours
// shifted by _multi_colored_line_start, which swings between -2 and 3.
func (g *Game) setFlashingColors(style, spacing int) {
	d := &g.des
	t := g.e.Text()
	c := t.Current
	start := int(g.ms.multiColoredLineStart)
	for i := 1; i < 8; i++ {
		v := int(c[i]) + start
		if int(c[0]) >= v {
			v = int(c[0])
		}
		c[i] = byte(v)
	}
	t.SetStyle(style, c)
	t.SetSpacing(spacing)
	g.ms.multiColoredLineStart += int16(d.incr)
	neg := -d.incr
	switch {
	case g.ms.multiColoredLineStart >= 3:
		g.ms.multiColoredLineStart = 3
		d.incr = neg
	case g.ms.multiColoredLineStart <= -2:
		g.ms.multiColoredLineStart = -2
		d.incr = neg
	}
}

// moxFontColors is Get_Mox_Font_Colors_ (0x8f6f8).
func (g *Game) moxFontColors(style int, a, b byte) font.Colors { return g.fontColors2(style, a, b) }

// printShipSizeErrorMessage is Print_Ship_Size_Error_Message_ (0x6e091):
// why a size button is dark (only while refitting, flag 0xf4).
func (g *Game) printShipSizeErrorMessage(i int) error {
	d := &g.des
	n := -1
	withPlanet := false
	var planet string
	switch d.sizeFlag[i] {
	case 0xf1:
		n = i - 3
	case 0xf2:
		n = i + 1
	case 0xf3:
		n = i + 7
	case 0xf4:
		n = 0xd
	case 0xf5:
		n = 0xe
		if g.returnScreen == ScreenBuildQueue {
			s := &g.state
			pl := s.Stars[g.screenData[0]].Planets[g.screenData[1]]
			planet = g.planetName(int(s.Colonies[s.Planets[pl].Colony].Planet))
			withPlanet = true
		}
	}
	if n <= -1 {
		return nil
	}
	msg := g.lbxMessage(n)
	if withPlanet {
		hull, err := g.hullName(int(d.t.size()), false)
		if err != nil {
			return err
		}
		msg = cSprintf(msg, planet, hull)
	}
	return g.messageBox(msg)
}

var _ = engine.MouseNone
