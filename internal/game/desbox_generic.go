package game

import "github.com/openorion2/openorion2/internal/font"

// The shield and computer boxes (Generic_Replacement_Box_, DESBOX.C).
// Spec: docs/screens/design-screen.md, "Boxes".

// Strings of DESBOX.C.
const (
	dbNone     = 0x1791b0 // ""
	dbKeyEsc   = 0x1791b1 // "\x1b"
	dbKeyUp    = 0x1791b3 // "-"
	dbKeyDown  = 0x1791b5 // "+"
	dbKeyBeam  = 0x1791b7 // "B"
	dbKeyMiss  = 0x1791b9 // "M"
	dbKeyBomb  = 0x1791bb // "O"
	dbKeySpec  = 0x1791bd // "S"
	dbKeyOK    = 0x1791bf // "A"
	dbFmtPlus  = 0x1791c1 // "+%d"
	dbFmtInt   = 0x1791c5 // "%d"
	dbFmtCount = 0x1791c8 // "%s (%d)"
	dbZero     = 0x1791d0 // "0"
	dbSpace    = 0x1791d2 // " "
	dbNewLines = 0x1791d4 // "\n\n"
	dbAn       = 0x1791d7 // "an"
	dbA        = 0x1791da // "a"
	dbArc360   = 0x1791dc // "360"
)

// designBoxState is DESBOX.C's state while a box is open.
type designBoxState struct {
	typ, item, slot int16 // _design_screen_replacement_type, _item, _slot
	bar             scrollBar
	chosen          int16 // _field_item_chosen
	scanned         int16 // _field_item_scanned
	baseX, baseY    int16 // _replacement_box_base_x/y
	width, height   int16
	midHeight       int16
	items           [0x28]int16 // _design_choice_items
	selected        [0x28]int16 // _design_choice_item_selected
	replF           [10]int     // _repl_fields
	scrollF         int         // _replacement_scroll_field
	upF, downF      int         // _replacement_scroll_up/dn_button
	spUpF, spDownF  int         // _special_scroll_up/dn_button

	// The weapons box.
	filter                       [0x28]weaponChoice // _weapon_filter
	arcs                         int16              // _weapon_replacement_arcs
	rack                         int16              // _weapon_replacement_rack
	mods                         uint16             // _weapon_replacement_mods
	modStatus                    [0xf]int16         // _weapon_mod_field_status
	validMods                    [0xf]uint8         // _valid_mods
	validArcs                    [5]uint8           // _valid_arcs_racks
	arcF, rackF                  [5]int             // _weapon_arc_field, _weapon_rack_field
	modF                         [0xf]int           // _weapon_mod_fields
	arcBoxF, acceptF             int                // _weapon_arc_rack_box_field, _replacement_box_accept_button
	beamF, missileF, bombF, spcF int                // the filter buttons
	filterOn                     [4]uint8           // beam, missile, bomb, special status (0x181750)
}

// weaponChoice is a 6-byte _weapon_filter row: the weapon and its field's
// research cost.
type weaponChoice struct {
	weapon int16
	cost   int32
}

// boxCycle is the colour cycle at the end of the box printers: every n
// calls _multi_colored_line_start moves between -1 and 3.
func (g *Game) boxCycle(n uint8) {
	d := &g.des
	d.waitCtr = int16(uint8(d.waitCtr) + 1)
	if uint8(d.waitCtr) < n {
		return
	}
	g.ms.multiColoredLineStart += int16(d.incr)
	d.waitCtr = 0
	switch {
	case g.ms.multiColoredLineStart >= 3:
		g.ms.multiColoredLineStart = 3
		d.incr = -1
	case g.ms.multiColoredLineStart <= -1:
		g.ms.multiColoredLineStart = -1
		d.incr = 1
	}
}

// drawDarkenedDesignScreen is the part of Draw_Darkened_Design_Screen_
// (0x65539) every box runs first: the screen drawn, then darkened as the
// background of the box.
func (g *Game) drawDarkenedDesignScreen() error {
	e := g.e
	e.V.SetPageOff()
	if err := g.drawDesignScreen(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	e.V.ResetWindow()
	e.V.ClipOff()
	e.UpdateGlassRemapColors()
	e.V.SetPaletteChanges(0, 0xff)
	e.V.SetPageOff()
	g.darkenFill(0, 0, 0x27f, 0x1df)
	e.V.CopyOffToBack()
	return nil
}

// redrawMainDesignScreenToBackground is
// Redraw_Main_Design_Screen_To_Background_ (0x67e50).
func (g *Game) redrawMainDesignScreenToBackground() error {
	e := g.e
	e.V.SetPageOff()
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	if err := g.drawDesignScreen(); err != nil {
		return err
	}
	e.V.ResetWindow()
	e.V.ClipOff()
	e.UpdateGlassRemapColors()
	e.V.SetPaletteChanges(0, 0xff)
	g.darkenFill(0, 0, 0x27f, 0x1df)
	e.V.CopyOffToBack()
	return nil
}

// genericReplacementBox is Generic_Replacement_Box_ (0x6603e): the shield
// (type 2) or computer (type 8) list; a row replaces the item, CANCEL
// keeps it.
func (g *Game) genericReplacementBox() error {
	e, d := g.e, &g.des
	b := &d.box
	d.incr = 1
	g.ms.multiColoredLineStart = 0
	b.bar = scrollBar{}
	g.fb.curBar = &b.bar
	if err := g.drawDarkenedDesignScreen(); err != nil {
		return err
	}
	e.ClearFields()
	e.DeactivateHelpList()
	e.AssignAutoFunction(g.drawGenericReplacementBox, 1)
	g.genericReplacementBoxChoices()
	g.genericReplacementBoxDimensions()
	first := true
	cancel, full, area, err := g.addGenericReplacementFields()
	if err != nil {
		return err
	}
	redraw := true
	var pick int16
	if b.slot != -1 {
		b.chosen = b.item
		pick = b.item
	} else {
		b.item, b.chosen = 0, 0
	}
	done := false
	for !done {
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return err
		}
		b.scanned = int16(scan)
		if int16(r) < 0 {
			if err := g.drawGenericReplacementBoxHelp(int16(r), int16(cancel), int16(full), int16(area)); err != nil {
				return err
			}
		}
		switch r {
		case cancel:
			pick = -1
			done = true
		case full:
			done = true
		}
		for i := int16(0); i < b.bar.rows && !done; i++ {
			if r == b.replF[i] {
				done = true
				pick = i
			}
		}
		if (!done && !first && redraw) || g.fullDrawScreen {
			if err := g.redrawMainDesignScreenToBackground(); err != nil {
				return err
			}
			redraw = false
			g.fullDrawScreen = false
		}
		if done {
			break
		}
		e.V.SetPageOff()
		if err := g.drawGenericReplacementBox(); err != nil {
			return err
		}
		if first {
			first = false
		} else if err := e.TogglePages(); err != nil {
			return err
		}
	}
	if pick >= 0 {
		switch b.typ {
		case 2:
			d.t.set16(dtShield, b.items[pick])
		case 8:
			d.t.set16(dtComputer, b.items[pick])
		}
	}
	return nil
}

// genericReplacementBoxChoices is Generic_Replacement_Box_Choices_
// (0x6626e): the six shields or computers, -1 for unknown ones.
func (g *Game) genericReplacementBoxChoices() {
	d := &g.des
	b := &d.box
	b.bar.rows = 1
	b.items = [0x28]int16{}
	for i := 0; i < 0x28; i++ {
		d.t[dtChoiceOK+i] = 0
	}
	var app uint32
	switch b.typ {
	case 2:
		app = shieldAppTbl
		b.bar.rows = 6
		for i := int16(0); i < b.bar.rows; i++ {
			g.genericChoice(i, g.exeI16(app+uint32(int32(i)*0x3b)))
		}
	case 8:
		app = compAppTbl
		b.bar.rows = 6
		for i := int16(0); i < b.bar.rows; i++ {
			g.genericChoice(i, g.exeI16(app+uint32(int32(i)*0x16)))
		}
	}
}

func (g *Game) genericChoice(i, app int16) {
	d := &g.des
	if i == 0 || g.state.Players[g.state.PlayerNum].AppStatus[app] == 3 {
		d.box.items[i] = i
		d.t[dtChoiceOK+int(i)] = 1
		return
	}
	d.box.items[i] = -1
}

// genericReplacementBoxDimensions is Generic_Replacement_Box_Dimensions_
// (0x663b8): the box centred on the screen above the bottom bar.
func (g *Game) genericReplacementBoxDimensions() {
	d := &g.des
	b, ps := &d.box, &d.pics
	rows := b.bar.rows
	if rows > 0xa {
		rows = 0xa
	}
	b.midHeight = rows<<4 + int16(ps.sysCancel.Height)
	b.width = int16(ps.sysTop.Width)
	b.height = b.midHeight + int16(ps.sysTop.Height) + int16(ps.sysBot.Height)
	b.baseX = int16((0x280 - int32(b.width)) / 2)
	b.baseY = int16((0x1b8 - int32(b.height)) / 2)
}

// addGenericReplacementFields is Add_Generic_Replacement_Fields_ (0x64a02):
// the rows, CANCEL, the box (with the weapons box's width) and the screen.
func (g *Game) addGenericReplacementFields() (cancel, full, area int, err error) {
	e, d := g.e, &g.des
	b, ps := &d.box, &d.pics
	none := cStringAt(g.exe, dbNone)
	top := int32(int16(ps.sysTop.Height))
	var yo int32
	for i := int16(0); i < b.bar.rows; i++ {
		if b.items[i] == -1 {
			b.replF[i] = dsNoField
			continue
		}
		y := int32(b.baseY) + top + 0x12 + yo
		f, err := e.AddHiddenField(int(int16(int32(b.baseX)+0xa)), int(int16(y)), int(int16(int32(b.baseX)+0x258)), int(int16(y+0xd)), none, 0x29)
		if err != nil {
			return 0, 0, 0, err
		}
		b.replF[i] = f
		yo += 0xe
	}
	esc := cStringAt(g.exe, dbKeyEsc)
	if cancel, err = e.AddButtonField(int(b.baseX+b.width-int16(ps.sysCancel.Width)-0x23), int(b.baseY+b.height-int16(ps.sysCancel.Height)-0x10), none, ps.sysCancel, esc, 0x28); err != nil {
		return
	}
	if area, err = e.AddHiddenField(int(b.baseX), int(b.baseY), int(b.baseX+int16(ps.wTop.Width)), int(b.baseY+b.height), none, 0); err != nil {
		return
	}
	full, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, esc, 0x29)
	return
}

// drawGenericReplacementBox is Draw_Generic_Replacement_Box_ (0x65758),
// the box's auto function.
func (g *Game) drawGenericReplacementBox() error {
	e, d := g.e, &g.des
	b, ps := &d.box, &d.pics
	x, y := int(b.baseX), int(b.baseY)
	if b.typ == 2 || b.typ == 8 {
		if err := e.Draw(x, y, ps.sysTop); err != nil {
			return err
		}
	}
	y = int(int16(y + int(int16(ps.sysTop.Height))))
	e.V.SetWindow(x, y, int(int16(int(b.width)+x)), int(int16(int(b.midHeight)+y)))
	e.V.ClipOn()
	if err := e.Draw(x, y, ps.sysMid); err != nil {
		return err
	}
	e.V.ClipOff()
	e.V.ResetWindow()
	if err := e.Draw(x, int(int16(y+int(b.midHeight))), ps.sysBot); err != nil {
		return err
	}
	var err error
	switch b.typ {
	case 2:
		err = g.printShieldData(int(b.baseX), int(b.baseY))
	case 8:
		err = g.printComputerData(int(b.baseX), int(b.baseY))
	}
	g.playStreamingMusic(8, -1, true) // 0x65849
	return err
}

// genericRowColors picks a generic box row's colours: the hovered one
// cycling, the chosen one bright, unknown ones dim.
func (g *Game) genericRowColors(i int16, chosen, avail, unavail font.Colors) font.Colors {
	d := &g.des
	b := &d.box
	switch {
	case b.scanned == int16(b.replF[i]) && d.t[dtChoiceOK+int(i)] != 0:
		var c font.Colors
		for k := 1; k < 8; k++ {
			c[k] = avail[k] + byte(g.ms.multiColoredLineStart)
		}
		return c
	case i == b.chosen:
		return chosen
	case d.t[dtChoiceOK+int(i)] != 0:
		return avail
	}
	return unavail
}

// printShieldData is Print_Shield_Data_ (0x67312). The cost column is
// centred by the space header's width and the space column by the cost
// header's.
func (g *Game) printShieldData(x, y int) error {
	e, d := g.e, &g.des
	b := &d.box
	tx := e.Text()
	e.V.SetWindow(x, y, int(int16(x+int(b.width))), int(int16(y+int(b.height))))
	e.V.ClipOn()
	y += 0x37
	tx.SetStyle(4, g.fontColors2(4, 0x55, 0x5b))
	x0 := x + 0x14
	costH, spaceH := g.lbxMessage(0x34), g.lbxMessage(0x35)
	if err := tx.Print(x0, y, g.hMessage(0x33)); err != nil {
		return err
	}
	if err := tx.Print(int(int16(x0+0xdc)), y, costH); err != nil {
		return err
	}
	if err := tx.Print(int(int16(x0+0x15e)), y, spaceH); err != nil {
		return err
	}
	wCost, wSpace := int16(tx.Width([]byte(costH))), int16(tx.Width([]byte(spaceH)))
	y += 0x12
	chosen := g.fontColors2(3, 0x55, 0x5b)
	avail := g.fontColors2(3, 0x55, 0x59)
	unavail := g.fontColors2(3, 0x55, 0x57)
	nx := int(int16(x0 + 3))
	for i := int16(0); i < b.bar.rows; i++ {
		tx.SetStyle(3, g.genericRowColors(i, chosen, avail, unavail))
		it := b.items[i]
		switch {
		case it == 0:
			if err := tx.Print(nx, y, g.hMessage(0x36)); err != nil {
				return err
			}
			if err := tx.PrintCentered(int(int16(int(wSpace/2)+x0+0xdc)), y, "0"); err != nil {
				return err
			}
			if err := tx.PrintCentered(int(int16(int(wCost/2)+x0+0x15e)), y, "0"); err != nil {
				return err
			}
		case it > 0:
			name, err := g.techName(walksShields + 1 + int(it))
			if err != nil {
				return err
			}
			if err := tx.Print(nx, y, name); err != nil {
				return err
			}
			app := g.exeI16(shieldAppTbl + uint32(int32(it)*0x3b))
			lvl := g.techLevel(int(g.state.PlayerNum), int(app))
			c := costGivenMLevel(g.exeI16(shieldCostTbl+uint32(int32(it)*0x3b+2*int32(d.t.size()))), lvl)
			if err := tx.PrintCentered(int(int16(int(wSpace/2)+x0+0xdc)), y, itoa(int(c))); err != nil {
				return err
			}
			lvl = g.techLevel(int(g.state.PlayerNum), int(app))
			sp := spaceGivenMLevel(g.exeI16(shieldSpaceTbl+uint32(int32(it)*0x3b+2*int32(d.t.size()))), lvl, 0)
			if err := tx.PrintCentered(int(int16(int(wCost/2)+x0+0x15e)), y, itoa(int(sp))); err != nil {
				return err
			}
		default:
			continue
		}
		y += 0xe
	}
	e.V.ResetWindow()
	e.V.ClipOff()
	g.boxCycle(2)
	return nil
}

// printComputerData is Print_Computer_Data_ (0x668fd).
func (g *Game) printComputerData(x, y int) error {
	e, d := g.e, &g.des
	b := &d.box
	tx := e.Text()
	e.V.SetWindow(x, y, int(int16(x+int(b.width))), int(int16(y+int(b.height))))
	e.V.ClipOn()
	y += 0x37
	tx.SetStyle(4, g.fontColors2(4, 0x55, 0x5b))
	h1, h2, h3 := g.lbxMessage(0x29), g.lbxMessage(0x2a), g.lbxMessage(0x2b)
	for _, h := range []struct {
		x int
		s string
	}{{x + 0x17, h1}, {x + 0xf5, h2}, {x + 0x1a9, h3}} {
		if err := tx.Print(int(int16(h.x)), y, h.s); err != nil {
			return err
		}
	}
	x0 := x + 0x14
	y += 0x12
	w1, w3 := int16(tx.Width([]byte(h1))), int16(tx.Width([]byte(h3)))
	chosen := g.fontColors2(3, 0x55, 0x5b)
	avail := g.fontColors2(3, 0x55, 0x59)
	unavail := g.fontColors2(3, 0x55, 0x57)
	nx := int(int16(x0 + 3))
	for i := int16(0); i < b.bar.rows; i++ {
		tx.SetStyle(3, g.genericRowColors(i, chosen, avail, unavail))
		it := b.items[i]
		switch {
		case it == 0:
			if err := tx.Print(nx, y, g.hMessage(0x2d)); err != nil {
				return err
			}
			if err := tx.PrintCentered(int(int16(int(w1/2)+x0+0xe1)), y, "0"); err != nil {
				return err
			}
			if err := tx.PrintCentered(int(int16(int(w3/2)+x0+0x195)), y, "0"); err != nil {
				return err
			}
		case it > 0:
			app := g.exeI16(compAppTbl + uint32(int32(it)*0x16))
			c := costGivenMLevel(g.computerCost(it), g.techLevel(int(g.state.PlayerNum), int(app)))
			name, err := g.techName(walksComputers + 1 + int(it))
			if err != nil {
				return err
			}
			if err := tx.Print(nx, y, name); err != nil {
				return err
			}
			s := cSprintf(cStringAt(g.exe, dbFmtPlus), g.exeI16(compAttackTbl+uint32(int32(it)*0x16)))
			if err := tx.PrintCentered(int(int16(int(w1/2)+x0+0xe1)), y, s); err != nil {
				return err
			}
			if err := tx.PrintCentered(int(int16(int(w3/2)+x0+0x195)), y, itoa(int(c))); err != nil {
				return err
			}
		default:
			continue
		}
		y += 0xe
	}
	e.V.ResetWindow()
	e.V.ClipOff()
	g.boxCycle(2)
	return nil
}

// computerCost is Computer_Cost_ (0x6b4d7): at least 1 for a computer.
func (g *Game) computerCost(c int16) int16 {
	v := g.exeI16(compCostTbl + uint32(int32(c)*0x16+2*int32(g.des.t.size())))
	if v == 0 && c > 0 {
		v = 1
	}
	return v
}

// drawGenericReplacementBoxHelp is Draw_Generic_Replacement_Box_Help_
// (0x6558c): right clicks on the box.
func (g *Game) drawGenericReplacementBoxHelp(r, cancel, full, area int16) error {
	d := &g.des
	b := &d.box
	pick := func(shield, comp int) int {
		switch b.typ {
		case 2:
			return shield
		case 8:
			return comp
		}
		return -1
	}
	id := -1
	switch {
	case r == -cancel:
		id = pick(0x2ab, 0x2ae)
	case r == -full:
		id = pick(0x2aa, 0x2ad)
	case r == -area:
		if b.typ > 1 {
			id = pick(0x2a9, 0x2ac)
		}
	}
	if id == -1 {
		found := false
		var it int16 = -1
		for i := int16(0); i < b.bar.rows && !found; i++ {
			if r == int16(-b.replF[i]) && b.items[i] != -1 {
				found = true
				it = b.items[i]
			}
		}
		if !found || it < 0 {
			return nil
		}
		if it != 0 {
			switch b.typ {
			case 2:
				return g.applicationTextBox(g.exeI16(shieldAppTbl+uint32(int32(it)*0x3b)), walksShields+1+int(it))
			case 8:
				return g.applicationTextBox(g.exeI16(compAppTbl+uint32(int32(it)*0x16)), walksComputers+1+int(it))
			}
			return nil
		}
		id = pick(0x2b4, 0x2b5)
		if id == -1 {
			id = 0
		}
	}
	return g.drawHelpEntry(id)
}
