package game

import "github.com/openorion2/openorion2/internal/font"

// The special systems box (Special_Systems_Box_, DESBOX.C). Spec:
// docs/screens/design-screen.md, "Boxes".

// specialSystemsBox is Special_Systems_Box_ (0x68755): the known specials
// with cost and space; a row puts its special into the slot, row 0 (none)
// clears the slot. Its scroll position stays in the template.
func (g *Game) specialSystemsBox() error {
	e, d := g.e, &g.des
	b := &d.box
	t := &d.t
	b.bar = scrollBar{}
	bar := &b.bar
	g.fb.curBar = bar
	bar.first = 0
	bar.pos = 0x8c
	b.chosen = -1
	bar.visible = 0xa
	g.ms.multiColoredLineStart = 0
	bar.track = 0x8c
	d.incr = 1
	bar.kind = 2
	bar.step = 1
	if err := g.drawDarkenedDesignScreen(); err != nil {
		return err
	}
	g.specialSystemChoices()
	bar.h = bar.height()
	first := true
	noToggle := true
	step := int16(int32(int32(bar.step)*int32(bar.track)*0xa+0x5a) / int32(bar.rows) / 0xa)
	top := int16(t[dtSpecialTop])
	bar.first = top
	pos := bar.track - bar.h/2 - int16(int32(step)*int32(top))
	bar.pos, bar.last, bar.start = pos, pos, pos
	g.specialSystemReplacementBoxDimensions(bar.rows)
	e.AssignAutoFunction(g.drawSpecialSystemsReplacementBox, 3)
	e.ClearFields()
	done := false
	cancel, full, err := g.addSpecialSystemFields()
	if err != nil {
		return err
	}
	d.waitCtr = 0
	for !done {
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return err
		}
		b.scanned = -1
		for i := 0; i < min(int(bar.rows), 10) && !done; i++ {
			idx := int16(i) + bar.first
			if scan == b.replF[i] {
				b.scanned = int16(i)
			}
			if r == b.replF[i] {
				if idx == 0 {
					if t.special(int(b.slot)) > 0 {
						g.clearSpecialSystemSlot(b.slot)
					}
					done = true
					continue
				}
				item := b.items[idx]
				excl, by := g.specialSystemExclusion(item)
				sel := g.itemsSelected()
				if item != 4 {
					app := g.exeI16(specAppTbl + uint32(int32(item)*0x2f))
					_ = g.miniaturizationCategory(app)
					_ = g.techLevel(int(g.state.PlayerNum), int(app))
				}
				ok := idx == 0 || (sel < 8 && !excl)
				name := func(sp int16) string {
					n, _ := g.techName(walksSpecials + 1 + int(sp))
					return n
				}
				var msg string
				switch {
				case b.selected[idx] != 0 && t.special(int(b.slot)) != item:
					msg = cSprintf(g.lbxMessage(0x41), name(item))
				case ok:
					done = true
					b.chosen = idx
					t.set16(dtSpecial+2*int(b.slot), item)
					continue
				case excl:
					if name(item) == name(by) {
						msg = cSprintf(g.lbxMessage(0x42), name(item))
					} else {
						art := func(sp int16) string {
							if n := name(sp); n != "" && isVowel(n[0]) {
								return cStringAt(g.exe, dbAn)
							}
							return cStringAt(g.exe, dbA)
						}
						msg = cSprintf(g.lbxMessage(0x43), art(by), name(by), art(item), name(item))
					}
				default:
					continue
				}
				noToggle = true
				if err := g.warningBox(msg); err != nil {
					return err
				}
			} else if r == -b.replF[i] {
				if it := b.items[idx]; it > 0 {
					if err := g.applicationTextBox(g.exeI16(specAppTbl+uint32(int32(it)*0x2f)), walksSpecials+1+int(it)); err != nil {
						return err
					}
				} else if err := g.drawHelpEntry(0x2b3); err != nil {
					return err
				}
			}
		}
		if r == b.spUpF {
			bar.scrollUpOneItem()
			d.waitCtr = 0
		}
		if r == b.spDownF {
			bar.scrollDownOneItem()
			d.waitCtr = 0
		}
		switch {
		case r == full || r == cancel:
			done = true
		case r == -full:
			if err := g.drawHelpEntry(0x2b1); err != nil {
				return err
			}
		case r == -cancel:
			if err := g.drawHelpEntry(0x2af); err != nil {
				return err
			}
		}
		e.ClearFields()
		if cancel, full, err = g.addSpecialSystemFields(); err != nil {
			return err
		}
		if !done && !noToggle && first {
			if err := g.redrawMainDesignScreenToBackground(); err != nil {
				return err
			}
			first = false
		}
		if done {
			break
		}
		e.V.SetPageOff()
		if err := g.drawSpecialSystemsReplacementBox(); err != nil {
			return err
		}
		if noToggle {
			noToggle = false
		} else if err := e.TogglePages(); err != nil {
			return err
		}
	}
	t[dtSpecialTop] = byte(bar.first)
	t.set16(dtSpecialBar, bar.pos)
	g.fb.curBar = nil
	g.currentScreen = ScreenDesign
	g.updateCalculatedDesignData(int(g.state.PlayerNum))
	e.ClearFields()
	return nil
}

// specialSystemChoices is Special_System_Choices_ (0x68413): none, then
// every known special; those fitted in other slots are marked selected.
func (g *Game) specialSystemChoices() int {
	d := &g.des
	b, t := &d.box, &d.t
	for i := 0; i < 0x28; i++ {
		t[dtChoiceOK+i] = 0
	}
	for i := range b.items {
		b.items[i] = -1
		b.selected[i] = 0
	}
	b.items[0] = 0
	b.bar.rows = 1
	pl := &g.state.Players[g.state.PlayerNum]
	for s := int16(0); s < 0x27 && b.bar.rows < 0x28; s++ {
		if pl.AppStatus[g.exeI16(specAppTbl+uint32(int32(s)*0x2f))] == 3 {
			b.items[b.bar.rows] = s
			b.bar.rows++
		}
	}
	for c := 0; c < 8; c++ {
		if t.special(c) <= 0 {
			continue
		}
		for i := int16(0); i < b.bar.rows; i++ {
			if t.special(c) == b.items[i] && b.slot > -1 && t.special(int(b.slot)) != b.items[i] {
				b.selected[i] = 1
			}
		}
	}
	n := 0
	for i := int16(1); i < b.bar.rows; i++ {
		it := b.items[i]
		app := g.exeI16(specAppTbl + uint32(int32(it)*0x2f))
		cat := g.miniaturizationCategory(app)
		sp := spaceGivenMLevel(g.exeI16(specSpaceTbl+uint32(int32(it)*0x2f+2*int32(t.size()))), g.techLevel(int(g.state.PlayerNum), int(app)), cat)
		if t.i16(dtSpecial+2*int(b.slot)) > 0 {
			sp -= t.i16(dtSpecialSpc + 2*int(b.slot))
		}
		if int32(sp)+t.i32(dtSpaceUsed) <= t.i32(dtSpaceTotal) && b.selected[i] == 0 {
			n++
		}
	}
	return n
}

// specialSystemExclusion is Special_System_Exclusion_ (0x68613): a special
// fitted in another slot that excludes sp.
func (g *Game) specialSystemExclusion(sp int16) (bool, int16) {
	t := &g.des.t
	slot := int(g.des.box.slot)
	for k := 0; k < 6; k++ {
		ex := g.exeI16(specExcludeTbl + uint32(int32(sp)*0x2f+int32(k)*2))
		if ex <= 0 {
			continue
		}
		for c := 0; c < 8; c++ {
			if v := t.special(c); v == ex && v != t.i16(dtSpecial+2*slot) {
				return true, v
			}
		}
	}
	return false, 0
}

// itemsSelected is Items_Selected_ (0x66742), over the first ten rows.
func (g *Game) itemsSelected() int16 {
	b := &g.des.box
	var n int16
	for i := 0; i < min(int(b.bar.rows), 10); i++ {
		if b.selected[i] != 0 {
			n++
		}
	}
	return n
}

// specialSystemReplacementBoxDimensions is
// Special_System_Replacement_Box_Dimensions_ (0x686cb).
func (g *Game) specialSystemReplacementBoxDimensions(rows int16) {
	d := &g.des
	b, ps := &d.box, &d.pics
	if rows > 0xa {
		rows = 0xa
	}
	b.midHeight = rows<<4 + 0x14 + int16(ps.sysCancel.Height)
	b.height = b.midHeight + int16(ps.sysTop.Height) + int16(ps.sysBot.Height)
	b.width = int16(ps.sysTop.Width)
	b.baseX = int16((0x280 - int32(b.width)) / 2)
	b.baseY = int16((0x1b8 - int32(b.height)) / 2)
}

// addSpecialSystemFields is Add_Special_System_Fields_ (0x65288).
func (g *Game) addSpecialSystemFields() (cancel, full int, err error) {
	e, d := g.e, &g.des
	b, ps := &d.box, &d.pics
	bar := &b.bar
	none := cStringAt(g.exe, dbNone)
	for i := range b.replF {
		b.replF[i] = dsNoField
	}
	for i := 0; i < min(int(bar.rows), 10); i++ {
		y := int(int16(int32(b.baseY) + 0x59 + int32(i)*0x10))
		f, err := e.AddHiddenField(int(b.baseX+0x19), y, int(b.baseX+0x248), int(int16(y+0x10)), none, 0x29)
		if err != nil {
			return 0, 0, err
		}
		b.replF[i] = f
	}
	b.scrollF = dsNoField
	var x, y int16
	if bar.rows > 0xa {
		x, y = b.baseX+0x25a, b.baseY+0x5d
		if b.scrollF, err = e.AddScrollField(int(x), int(y), 0, 0x8c, 0, 0x8c, 9, 0x8c, &bar.pos, none, 0); err != nil {
			return
		}
	}
	if bar.rows <= 0xa || x <= 0 || y <= 0 {
		b.spUpF, b.spDownF, b.scrollF = dsNoField, dsNoField, dsNoField
	} else {
		if b.spUpF, err = e.AddButtonField(int(x), int(y-int16(ps.up.Height)-3), none, ps.up, cStringAt(g.exe, dbKeyUp), 0x28); err != nil {
			return
		}
		if b.spDownF, err = e.AddButtonField(int(x), int(int16(ps.sysBar.Height)+y+3), none, ps.down, cStringAt(g.exe, dbKeyDown), 0x28); err != nil {
			return
		}
	}
	if cancel, err = e.AddButtonField(int(b.baseX+b.width-int16(ps.sysCancel.Width)-0x23), int(b.baseY+b.height-int16(ps.sysCancel.Height)-0x10), none, ps.sysCancel, cStringAt(g.exe, dbKeyEsc), 0x28); err != nil {
		return
	}
	if _, err = e.AddHiddenField(int(b.baseX), int(b.baseY), int(b.baseX+b.width), int(b.baseY+b.height), none, 0); err != nil {
		return
	}
	full, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, none, 0x29)
	bar.upField, bar.downField = b.spUpF, b.spDownF
	return
}

// drawSpecialSystemsReplacementBox is Draw_Special_Systems_Replacement_Box_
// (0x65853), the box's auto function.
func (g *Game) drawSpecialSystemsReplacementBox() error {
	e, d := g.e, &g.des
	b, ps := &d.box, &d.pics
	x, y := int(b.baseX), int(b.baseY)
	if err := e.Draw(x, y, ps.sysTop); err != nil {
		return err
	}
	y = int(int16(y + int(int16(ps.sysTop.Height))))
	e.V.SetWindow(x, y, int(int16(x+int(b.width))), int(int16(int(b.midHeight)+y)))
	e.V.ClipOn()
	if err := e.Draw(x, y, ps.sysMid); err != nil {
		return err
	}
	e.V.ClipOff()
	e.V.ResetWindow()
	if err := e.Draw(x, int(int16(y+int(b.midHeight))), ps.sysBot); err != nil {
		return err
	}
	if err := g.printSpecialSystemData(int(b.baseX), int(b.baseY)); err != nil {
		return err
	}
	if b.bar.rows > 0xa && b.scrollF != dsNoField {
		if _, err := g.drawGenericVerticalScrollBar(&b.bar, g.drawSpecialSystemsScrollBar); err != nil {
			return err
		}
	}
	return nil
}

// drawSpecialSystemsScrollBar is Draw_Special_Systems_Scroll_Bar_
// (0x65923).
func (g *Game) drawSpecialSystemsScrollBar() error {
	e, d := g.e, &g.des
	b, ps := &d.box, &d.pics
	bar := &b.bar
	x0, y0 := int(int16(int32(b.baseX)+0x254)), int(int16(int32(b.baseY)+0x5c))
	ws := g.saveWindowStatus()
	e.V.SetWindow(x0, y0, int(int16(x0+int(int16(ps.sysBar.Width)))), int(int16(y0+int(int16(ps.sysBar.Height)))))
	e.V.ClipOn()
	y1 := int(int32(b.baseY) + 0x5d + int32(bar.track) - (int32(bar.h)/2 + int32(bar.pos)))
	x := int(int16(int32(b.baseX) + 0x25a))
	c := byte(0x5b)
	for i := 0; i < 9; i++ {
		e.V.Line(x, int(int16(y1+1)), x, int(int16(y1+int(bar.h))), c)
		if i%2 == 0 {
			c--
		}
		x = int(int16(x + 1))
	}
	if err := e.Draw(x0, y0, ps.sysBar); err != nil {
		return err
	}
	g.restoreWindowStatus(ws)
	return nil
}

// printSpecialSystemData is Print_Special_System_Data_ (0x6767f): the rows
// in one style small enough for every special's name; what cannot be added
// is dim.
func (g *Game) printSpecialSystemData(x, y int) error {
	e, d := g.e, &g.des
	b, t, ps := &d.box, &d.t, &d.pics
	bar := &b.bar
	tx := e.Text()
	tx.SetStyle(4, g.fontColors2(4, 0x55, 0x5c))
	hy := int(int16(y + 0x46))
	for _, h := range []struct{ dx, n int }{{0x44, 0x37}, {0xbe, 0x30}, {0x106, 0x2b}, {0x156, 0x38}} {
		if err := tx.Print(int(int16(x+h.dx)), hy, g.hMessage(h.n)); err != nil {
			return err
		}
	}
	style := 3
	dim := g.fontColors2(2, 0x55, 0x57)
	bright := g.fontColors2(3, 0x55, 0x5a)
	e.V.SetWindow(int(int16(x+0x19)), int(int16(y+0x40)), int(int16(x+0x247)), int(int16(int(b.midHeight)+y+0x40+5)))
	e.V.ClipOn()
	for s := 1; s < 0x27; s++ {
		tx.SetSpacing(1)
		c := tx.Current
		name, err := g.techName(walksSpecials + 1 + s)
		if err != nil {
			return err
		}
		g.setFittedFontStyle(name, c, style, 1, -1, 0xa5, 1)
		if tx.Style < style {
			style = tx.Style
		}
	}
	defSpacing := func() { tx.SetSpacing(tx.Font.Styles[tx.Style].HSpacing) }
	ry := y + 0x59
	for i := 0; i < min(int(bar.rows), 10); i++ {
		idx := int16(i) + bar.first
		it := b.items[idx]
		if it > -1 {
			cost, space := g.systemAddedCostAndSpace(idx)
			adj := space
			if b.slot > -1 && t.special(int(b.slot)) > 0 {
				adj = space - t.i16(dtSpecialSpc+2*int(b.slot))
			}
			ok := false
			if idx == 0 {
				ok = true
			} else if int32(adj)+t.i32(dtSpaceUsed) <= t.i32(dtSpaceTotal) {
				if ex, _ := g.specialSystemExclusion(it); !ex {
					ok = true
				}
			}
			if ok {
				tx.SetStyle(style, bright)
			} else {
				tx.SetStyle(style, dim)
			}
			if b.scanned == int16(i) && ok {
				tx.SetStyle(style, g.fontColors2(2, 0x55, 0x58))
				g.increaseFontColorRange(int(g.ms.multiColoredLineStart))
			}
			yy := int(int16(ry))
			if idx == 0 {
				if err := tx.Print(int(int16(x+0x20)), yy, g.hMessage(0x39)); err != nil {
					return err
				}
				if err := tx.PrintRight(int(int16(x+0xe3)), yy, cStringAt(g.exe, dbZero)); err != nil {
					return err
				}
				if err := tx.PrintRight(int(int16(x+0x124)), yy, cStringAt(g.exe, dbZero)); err != nil {
					return err
				}
				if err := tx.Print(int(int16(x+0x13e)), yy, cStringAt(g.exe, dbSpace)); err != nil {
					return err
				}
			} else {
				name, err := g.techName(walksSpecials + 1 + int(it))
				if err != nil {
					return err
				}
				tx.SetSpacing(1)
				if err := tx.Print(int(int16(x+0x20)), yy, name); err != nil {
					return err
				}
				defSpacing()
				if err := tx.PrintRight(int(int16(x+0xe3)), yy, itoa(int(space))); err != nil {
					return err
				}
				if err := tx.PrintRight(int(int16(x+0x124)), yy, itoa(int(cost))); err != nil {
					return err
				}
				tx.SetStyle(style, tx.Current)
				tx.SetSpacing(1)
				if err := tx.Print(int(int16(x+0x13e)), yy, string(d.specialDesc[it])); err != nil {
					return err
				}
				defSpacing()
			}
		}
		ry += 0x10
	}
	if bar.first > 0 {
		if err := e.Draw(int(b.baseX+b.width-0x20), int(b.baseY+0x47), ps.dullUp); err != nil {
			return err
		}
	}
	if bar.rows > 0xa && int32(bar.first)+0xa > int32(bar.rows) {
		if err := e.Draw(int(b.baseX+b.width-0x20), int(b.baseY+b.height-0x1e), ps.dullDown); err != nil {
			return err
		}
	}
	e.V.ClipOff()
	e.V.ResetWindow()
	g.boxCycle(3)
	return nil
}

// systemAddedCostAndSpace is System_Added_Cost_And_Space_ (0x68d3d): the
// cost and space of choice row i (special 4 unminiaturised).
func (g *Game) systemAddedCostAndSpace(i int16) (cost, space int16) {
	t := &g.des.t
	sp := g.des.box.items[i]
	row := int32(sp)*0x2f + 2*int32(t.size())
	c := g.exeI16(specCostTbl + uint32(row))
	s := g.exeI16(specSpaceTbl + uint32(row))
	if sp == 4 {
		return c, s
	}
	app := g.exeI16(specAppTbl + uint32(int32(sp)*0x2f))
	p := int(g.state.PlayerNum)
	cost = costGivenMLevel(c, g.techLevel(p, int(app)))
	space = spaceGivenMLevel(s, g.techLevel(p, int(app)), g.miniaturizationCategory(app))
	return
}

var _ font.Colors
