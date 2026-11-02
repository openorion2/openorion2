package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/engine"
)

// The weapons box (Weapons_Replacement_Box_, DESBOX.C): the known weapons
// by research cost, filters, arcs or rack sizes and modifications. Spec:
// docs/screens/design-screen.md, "Boxes".

// Tables of DESBOX.C in the code segment.
const (
	dbRackSizes    = 0x649d5 // 5 x i16: 2, 5, 10, 15, 20 (one copy per routine)
	dbRackSizesB   = 0x649fd // 5 x u8, Weapons_Replacement_Box_'s copy
	dbWeaponBoxHlp = 0x19bc14
)

// rackSize is a missile rack's ammunition (the routines' local copies of
// 2, 5, 10, 15, 20).
func (g *Game) rackSize(i int16) int16 { return g.exeI16(dbRackSizes + uint32(int32(i)*2)) }

// weaponsReplacementBox is Weapons_Replacement_Box_ (0x69583).
func (g *Game) weaponsReplacementBox() error {
	e, d := g.e, &g.des
	b, t := &d.box, &d.t
	var arcNames [5]string
	for i := 0; i < 4; i++ {
		arcNames[i] = g.hMessage(0x46 + i)
	}
	arcNames[4] = cStringAt(g.exe, dbArc360)
	b.bar = scrollBar{}
	bar := &b.bar
	g.fb.curBar = bar
	slot := b.slot
	b.chosen = -1
	b.arcs = 0
	g.ms.multiColoredLineStart = 0
	d.incr = 1
	d.waitCtr = 0
	bar.first = 0
	bar.kind = 3
	e.V.SetPageOff()
	if err := g.drawDesignScreen(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	b.items = [0x28]int16{}
	done := false
	g.weaponChoices(int(g.state.PlayerNum))
	redraw := true
	if bar.rows > 1 {
		g.sortWeaponFilter()
	}
	g.spaceWithWeapons()
	bar.visible = 0xa
	bar.track = 0x78
	bar.h = bar.height()
	bar.last = -1
	bar.step = 1
	g.ms.multiColoredBoxColorStart = 0
	bar.pos = bar.track - bar.h/2
	e.V.ResetWindow()
	e.V.ClipOff()
	e.UpdateGlassRemapColors()
	e.V.SetPaletteChanges(0, 0xff)
	e.V.SetPageOff()
	g.darkenFill(0, 0, 0x27f, 0x1df)
	e.V.CopyOffToBack()
	e.ClearFields()
	cancel, full, err := g.addReplacementWeaponFields()
	if err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawWeaponsReplacementBox, 1)
	g.getWeaponsBoxBaseDimensions()
	g.setUpWeaponDisplay(slot)
	noToggle := true
	g.weaponReplacementHelp(b.slot)
	for !done {
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return err
		}
		switch {
		case r == full || r == cancel:
			done = true
		case r == -full:
			if err := g.drawHelpEntry(0x2b0); err != nil {
				return err
			}
		}
		if b.chosen > -1 {
			typ := g.weaponTypeOf(b.filter[b.chosen].weapon)
			for i := 0; i < 5; i++ {
				msg := ""
				switch {
				case typ == 0 && r == b.arcF[i]:
					if b.validArcs[i] != 0 {
						b.arcs = 1 << uint(i)
						continue
					}
					msg = cSprintf(g.lbxMessage(0x4b), arcNames[i])
				case (typ == 1 || typ == 2) && r == b.rackF[i]:
					if b.validArcs[i] != 0 {
						b.arcs = 0x10
						b.rack = int16(i)
						continue
					}
					msg = cSprintf(g.lbxMessage(0x4c), g.exeU8(dbRackSizesB+uint32(i)))
				default:
					continue
				}
				if err := g.messageBox(msg); err != nil {
					return err
				}
			}
		}
		switch r {
		case b.beamF:
			b.filterOn[0] = boolByte(b.filterOn[0] == 0)
		case b.missileF:
			b.filterOn[1] = boolByte(b.filterOn[1] == 0)
		case b.bombF:
			b.filterOn[2] = boolByte(b.filterOn[2] == 0)
		case b.spcF:
			b.filterOn[3] = boolByte(b.filterOn[3] == 0)
		}
		for i := 1; i < 0xf; i++ {
			switch {
			case r == -b.modF[i]:
				if err := g.printWeaponModHelpDescription(i); err != nil {
					return err
				}
			case r == b.modF[i] && b.validMods[i] == 1:
				on := b.modStatus[i] == 0
				b.modStatus[i] = int16(boolByte(on))
				if i == 1 && on {
					b.modStatus[2] = 0
				} else if i == 2 && b.modStatus[2] != 0 {
					b.modStatus[1] = 0
				}
			}
		}
		kind, scanned, row := g.scanReplacementFields(scan, r)
		b.scanned = scanned
		switch kind {
		case 0:
			if row == 0 {
				done = true
				b.arcs, b.mods, b.chosen = 0, 0, 0
			}
			if row <= -1 || b.chosen == row {
				g.updateWeaponReplacement(&done, b.chosen, slot)
				break
			}
			b.chosen = row
			w := b.filter[row].weapon
			switch {
			case w == -1:
				done = true
			case t.weapon(int(slot)) > 0 && w == t.weapon(int(slot)):
				g.setUpWeaponDisplay(b.slot)
			default:
				typ := g.weaponTypeOf(w)
				b.mods = 0
				b.modStatus = [0xf]int16{}
				if typ == 0 {
					b.arcs = 1
				} else {
					b.arcs = 0x10
				}
				b.rack = 1
				b.mods = g.modsFromStatus()
				g.weaponReplacementHelp(b.chosen)
			}
		case 1:
			if w := b.filter[row].weapon; w > 0 {
				if err := g.applicationTextBox(g.exeI16(wpnAppTbl+uint32(int32(w)*0x1c)), walksWeapons+1+int(w)); err != nil {
					return err
				}
			} else if err := g.drawHelpEntry(0x2b2); err != nil {
				return err
			}
		}
		for i := 0; i < 0xf; i++ {
			if r == b.modF[i] && b.validMods[i] > 0xf8 {
				if err := g.printWeaponModificationMessage(i); err != nil {
					return err
				}
			}
		}
		if r == b.arcBoxF {
			var msg string
			switch {
			case b.chosen == -1:
				msg = g.hMessage(0x4e)
			default:
				switch g.weaponTypeOf(b.filter[b.chosen].weapon) {
				case 3, 4, 5:
					name, _ := g.techName(walksWeapons + 1 + int(b.filter[b.chosen].weapon))
					msg = cSprintf(g.lbxMessage(0x4f), name)
				}
			}
			if msg != "" || b.chosen == -1 {
				if err := g.messageBox(msg); err != nil {
					return err
				}
			}
		}
		if r == b.beamF || r == b.missileF || r == b.bombF || r == b.spcF {
			old := bar.rows
			pick := int16(-1)
			if b.chosen > -1 {
				pick = b.filter[b.chosen].weapon
			}
			g.weaponChoices(int(g.state.PlayerNum))
			if bar.rows > 0 {
				g.sortWeaponFilter()
				switch {
				case bar.rows < 0xa:
					bar.first = 0
				case int32(bar.rows) < int32(bar.first)+0xa:
					bar.first = int16(max(int32(bar.rows)-0xa, 0))
				}
			}
			b.chosen = g.updateFieldItemChosen(pick)
			if bar.rows != old {
				g.getWeaponsBoxBaseDimensions()
				redraw = true
			}
			g.weaponReplacementHelp(b.chosen)
			g.spaceWithWeapons()
		}
		if r == b.upF {
			d.waitCtr = 0
			bar.scrollUpOneItem()
		}
		if r == b.downF {
			d.waitCtr = 0
			bar.scrollDownOneItem()
		}
		if r == b.acceptF {
			g.updateWeaponReplacement(&done, b.chosen, slot)
		}
		if !done {
			b.mods = g.modsFromStatus()
			e.ClearFields()
			if cancel, full, err = g.addReplacementWeaponFields(); err != nil {
				return err
			}
		}
		if !done && !noToggle && redraw {
			if err := g.redrawMainDesignScreenToBackground(); err != nil {
				return err
			}
			redraw = false
		}
		if done {
			break
		}
		e.V.SetPageOff()
		if err := g.drawWeaponsReplacementBox(); err != nil {
			return err
		}
		if noToggle {
			noToggle = false
		} else if err := e.TogglePages(); err != nil {
			return err
		}
	}
	g.fb.curBar = nil
	e.DeactivateAutoFunction()
	e.ClearFields()
	g.currentScreen = ScreenDesign
	return nil
}

func boolByte(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// modsFromStatus is the loop that rebuilds _weapon_replacement_mods from
// the modification buttons (Set_New_Weapon_Modifications_ without its
// chosen test).
func (g *Game) modsFromStatus() uint16 {
	var m uint16
	for i := 1; i < 0xf; i++ {
		if g.des.box.modStatus[i] == 1 {
			m |= 1 << uint(i)
		}
	}
	return m
}

// sortWeaponFilter is qsort_ of the choices with qsort_by_research_cost_
// (0x69e59): dearest research first.
func (g *Game) sortWeaponFilter() {
	b := &g.des.box
	clib.Qsort(b.filter[:b.bar.rows], 6, func(x, y *weaponChoice) int { return int(y.cost - x.cost) })
}

// weaponChoices is Weapon_Choices_ (0x6921b): none, then the known weapons
// the filters let through (one tractor beam per design); it reports
// whether any fits.
func (g *Game) weaponChoices(p int) bool {
	d := &g.des
	b, t := &d.box, &d.t
	for i := 0; i < 0x28; i++ {
		t[dtChoiceOK+i] = 0
	}
	for i := range b.filter {
		b.filter[i] = weaponChoice{-1, -1}
	}
	b.filter[0] = weaponChoice{0, 99999}
	t[dtChoiceOK] = 1
	b.bar.rows = 1
	pl := &g.state.Players[p]
	for w := int16(1); w < 0x28; w++ {
		t[dtChoiceOK+int(w)] = 0
		app := g.exeI16(wpnAppTbl + uint32(int32(w)*0x1c))
		if pl.AppStatus[app] != 3 {
			continue
		}
		add := false
		switch typ := g.weaponTypeOf(w); {
		case typ == 0:
			add = b.filterOn[0] != 0
		case typ <= 2:
			add = b.filterOn[1] != 0
		case typ == 3:
			add = b.filterOn[2] != 0
		case typ <= 5:
			add = b.filterOn[3] != 0 && (g.exeI16(0x17f80b+uint32(int32(w)*0x1c)) != wpnTractor || !g.haveTractorBeamWeapon())
		}
		if add {
			field := g.exeI16(appFieldTbl + uint32(int32(app)*0xd))
			b.filter[b.bar.rows] = weaponChoice{w, int32(g.exe.u32(fieldCostTbl + uint32(int32(field)*0x17)))}
			if cur := t.weapon(int(b.slot)); cur > 0 && cur == w {
				b.chosen = b.bar.rows
			}
			b.bar.rows++
		}
		if b.bar.rows >= 0x28 {
			break
		}
	}
	g.setUpWeaponScrollBar()
	n := 0
	for i := int16(1); i < b.bar.rows; i++ {
		w := b.filter[i].weapon
		app := g.exeI16(wpnAppTbl + uint32(int32(w)*0x1c))
		base := g.exeI16(wpnSpaceTbl + uint32(int32(w)*0x1c))
		lvl := g.techLevel(int(g.state.PlayerNum), int(app))
		cat := g.miniaturizationCategory(app)
		sp := int16(spaceGivenMLevel(base, lvl, cat))
		var rack int16
		if g.weaponNeedsRacks(w) {
			rack = spaceGivenMLevel(10, lvl, cat)
		}
		v := int16(t.i32(dtSpaceUsed)) + sp + rack
		if t.weapon(int(b.slot)) > 0 {
			v -= int16(t.wspace(int(b.slot)))
		}
		if int32(v) <= t.i32(dtSpaceTotal) {
			n++
		}
	}
	return n != 0
}

// haveTractorBeamWeapon is Have_Tractor_Beam_Weapon_ (0x666f5): a tractor
// beam in another slot.
func (g *Game) haveTractorBeamWeapon() bool {
	t := &g.des.t
	slot := g.des.box.slot
	if slot != -1 && t.weapon(int(slot)) == wpnTractor {
		return false
	}
	for k := 0; k < 8; k++ {
		if t.weapon(k) == wpnTractor {
			return true
		}
	}
	return false
}

// spaceWithWeapons is Space_With_Weapons_ (0x6828d): which choices fit.
func (g *Game) spaceWithWeapons() {
	d := &g.des
	b, t := &d.box, &d.t
	t[dtChoiceOK] = 1
	pl := &g.state.Players[g.state.PlayerNum]
	for i := 1; i < 0x28; i++ {
		w := b.filter[i].weapon
		t[dtChoiceOK+i] = 0
		if w <= -1 || pl.AppStatus[g.exeI16(wpnAppTbl+uint32(int32(w)*0x1c))] != 3 {
			continue
		}
		if int32(g.spaceWithWeapon(w)) <= t.i32(dtSpaceTotal) {
			t[dtChoiceOK+i] = 1
		}
	}
}

// spaceWithWeapon is Space_With_Weapon_ (0x6831f): the space used with one
// mount of w in the slot.
func (g *Game) spaceWithWeapon(w int16) int16 {
	t := &g.des.t
	var sp, rack int16
	if w > 0 {
		app := g.exeI16(wpnAppTbl + uint32(int32(w)*0x1c))
		lvl := g.techLevel(int(g.state.PlayerNum), int(app))
		cat := g.miniaturizationCategory(app)
		sp = spaceGivenMLevel(g.exeI16(wpnSpaceTbl+uint32(int32(w)*0x1c)), lvl, cat)
		if g.weaponNeedsRacks(w) {
			rack = spaceGivenMLevel(10, lvl, cat)
		}
	}
	v := int16(t.i32(dtSpaceUsed)) + sp + rack
	if s := g.des.box.slot; s > -1 && t.weapon(int(s)) > 0 {
		v -= int16(t.wspace(int(s)))
	}
	return v
}

// setUpWeaponScrollBar is Set_Up_Weapon_Scroll_Bar_ (0x68177): the thumb at
// the chosen row.
func (g *Game) setUpWeaponScrollBar() {
	b := &g.des.box
	bar := &b.bar
	if bar.rows < 0xa {
		bar.first = 0
		bar.pos = bar.track
	} else {
		step := int16((0x4b0/int32(bar.rows) + 9) / 0xa)
		if int32(bar.visible)+int32(bar.first) > int32(bar.rows) {
			bar.pos = step * bar.visible
			bar.first = bar.rows - 0xa
		} else {
			bar.pos = bar.track - int16(int32(uint16(b.chosen))*int32(step))
			if int32(bar.pos) < int32(step)*int32(bar.visible) {
				bar.pos = step * bar.visible
			}
			if bar.pos > bar.track {
				bar.pos = bar.track
			}
			if bar.pos < 0 {
				bar.pos = 0
			}
		}
	}
	bar.pos -= bar.h / 2
	if bar.pos < 0 {
		bar.pos = bar.h / 2
	}
	bar.last = -1
}

// setUpWeaponDisplay is Set_Up_Weapon_Display_ (0x67ffc): the slot's
// weapon, arcs, rack and modifications picked in the box.
func (g *Game) setUpWeaponDisplay(slot int16) {
	b, t := &g.des.box, &g.des.t
	b.modStatus = [0xf]int16{}
	b.arcs = 0
	k := int(slot)
	if t.weapon(k) > 0 {
		b.arcs = int16(t.arc(k))
		b.mods = uint16(t.mods(k))
		found := false
		for i := int16(1); i < b.bar.rows && !found; i++ {
			if b.filter[i].weapon == t.weapon(k) {
				found = true
				b.chosen = i
				b.bar.first = i
				g.setUpWeaponScrollBar()
			}
		}
		if !found {
			b.chosen = -1
		}
		if g.weaponNeedsRacks(t.weapon(k)) {
			b.rack = -1
			for i := int16(0); i < 5; i++ {
				if int16(t.ammo(k)) == g.exeI16(0x649e9+uint32(i)*2) {
					b.rack = i
					break
				}
			}
		} else {
			b.rack = 1
		}
	} else {
		b.mods = 0
	}
	for i := 1; i < 0xf; i++ {
		if int32(int16(b.mods))>>uint(i)&1 != 0 {
			b.modStatus[i] = 1
		}
	}
	b.mods = g.modsFromStatus()
}

// updateWeaponReplacement is Update_Weapon_Replacement_ (0x68e78): ACCEPT or
// a second click on the chosen row puts it into the slot (the count kept
// by space); row 0 clears the slot.
func (g *Game) updateWeaponReplacement(done *bool, chosen, slot int16) {
	d := &g.des
	b, t := &d.box, &d.t
	k := int(slot)
	if chosen == 0 {
		if t.weapon(k) > 0 {
			g.clearWeaponSlot(slot)
			g.updateCalculatedDesignData(int(g.state.PlayerNum))
		}
		g.currentScreen = ScreenDesign
		*done = true
		return
	}
	ok, msg := g.weaponChoiceValid()
	if !ok {
		if err := g.messageBox(msg); err != nil {
			g.Log("design: %v", err)
		}
		return
	}
	if chosen <= -1 {
		return
	}
	oldSpace := int16(t.wspace(k))
	had := t.weapon(k) > 0
	t.set16(dtWeapon+2*k, b.filter[chosen].weapon)
	t.set16(dtCount+2*k, 1)
	t.set16(dtMods+2*k, int16(g.setNewWeaponModifications()))
	t[dtArc+k] = byte(b.arcs)
	if g.weaponNeedsRacks(t.weapon(k)) {
		t[dtAmmo+k] = byte(g.exeI16(0x649f3 + uint32(int32(b.rack)*2)))
	} else {
		t[dtAmmo+k] = 0
	}
	t.set32(dtWeaponSpace+4*k, g.weaponSpace(int(g.state.PlayerNum), int(t.weapon(k)), t.count(k), int16(t.arc(k)), int16(t.ammo(k)), t.mods(k)))
	t.set32(dtWeaponCost+4*k, int32(g.weaponCost(t.weapon(k), t.count(k), t.arc(k), t.ammo(k), t.mods(k))))
	if sp := int16(t.wspace(k)); had && sp > 0 {
		n := int32(oldSpace) / int32(sp)
		if n <= 1 {
			n = 1
		}
		c := int16(n)
		if c > 0x63 {
			c = 0x63
		}
		t.set16(dtCount+2*k, c)
	}
	g.updateCalculatedDesignData(int(g.state.PlayerNum))
	g.currentScreen = ScreenDesign
	*done = true
}

// setNewWeaponModifications is Set_New_Weapon_Modifications_ (0x67f5c).
func (g *Game) setNewWeaponModifications() uint16 {
	if g.des.box.chosen < 0 {
		return 0
	}
	return g.modsFromStatus()
}

// weaponChoiceValid is Weapon_Choice_Valid_ (0x691a3); it also puts an
// out-of-range choice back on row 0.
func (g *Game) weaponChoiceValid() (bool, string) {
	b := &g.des.box
	ok, msg := true, ""
	if b.chosen > -1 && g.weaponTypeOf(b.filter[b.chosen].weapon) == 0 && b.arcs == -1 {
		ok, msg = false, g.hMessage(0x44)
	}
	if b.chosen < 0 || b.chosen > b.bar.rows {
		b.chosen = 0
	}
	return ok, msg
}

// updateFieldItemChosen is Update_Field_Item_Chosen_ (0x68de3): the row of
// weapon w after the list changed.
func (g *Game) updateFieldItemChosen(w int16) int16 {
	b := &g.des.box
	if b.bar.rows > 1 && w > -1 {
		for i := int16(0); i < b.bar.rows; i++ {
			if b.filter[i].weapon == w {
				return i
			}
		}
	}
	return -1
}

// scanReplacementFields is Scan_Replacement_Fields_ (0x67e9c): 0 for a
// click on a row, 1 for a right click, 4 for a hovered row, -1 for none.
func (g *Game) scanReplacementFields(scan, r int) (kind, scanned, row int16) {
	b := &g.des.box
	kind, scanned, row = -1, -1, -1
	n := min(int(b.bar.rows), 10)
	for i := 0; i < n; i++ {
		switch {
		case r == b.replF[i]:
			kind, row = 0, int16(i)+b.bar.first
		case -r == b.replF[i]:
			kind, row = 1, int16(i)+b.bar.first
		}
	}
	if kind == -1 {
		for i := 0; i < n; i++ {
			if scan == b.replF[i] {
				kind, scanned = 4, int16(i)
			}
		}
	}
	return
}

// modAllowedOnWeaponType is Mod_Allowed_On_Weapon_Type_ (0x6678c).
func (g *Game) modAllowedOnWeaponType(i int) bool {
	b := &g.des.box
	typ := uint16(0xffff)
	if b.chosen > -1 {
		typ = g.weaponTypeOf(b.filter[b.chosen].weapon)
	}
	var mask int32
	switch typ {
	case 0:
		mask = 0x1fe
	case 1:
		mask = 0x3e00
	case 2:
		mask = 0x6520
	default:
		return false
	}
	return mask>>uint(i)&1 != 0
}

// shouldAddMod is Should_Add_Mod_ (0x6825f).
func (g *Game) shouldAddMod(i int) bool {
	switch g.des.box.validMods[i] {
	case 1, 0xf9, 0xfc, 0xfb:
		return true
	}
	return false
}

// newArcSpace is New_Arc_Space_ (0x66810), computed and dropped by its
// caller.
func (g *Game) newArcSpace(i int) int32 {
	b := &g.des.box
	if b.chosen <= -1 {
		return 0
	}
	return g.weaponSpace(int(g.state.PlayerNum), int(b.filter[b.chosen].weapon), 1, int16(1<<uint(i)), b.rack, int16(b.mods))
}

// newModSpace is New_Mod_Space_ (0x66870): the weapon's space with mod i
// added to those switched on.
func (g *Game) newModSpace(i int) int32 {
	b := &g.des.box
	if b.chosen <= -1 {
		return 0
	}
	var m uint16
	for k := 0; k < 0xf; k++ {
		if b.modStatus[k] == 1 {
			m |= 1 << uint(k)
		}
	}
	m |= 1 << uint(i)
	return g.weaponSpace(int(g.state.PlayerNum), int(b.filter[b.chosen].weapon), 1, b.arcs, b.rack, int16(m))
}

// validModifications is Valid_Modifications_ (0x6908a): per modification 1
// (allowed), 0xfd (heavy mount without the tech), 0xfe (not for this
// weapon) or 0xfa (miniaturisation too low).
func (g *Game) validModifications() int {
	b := &g.des.box
	b.validMods = [0xf]uint8{}
	n := 0
	if b.chosen <= -1 {
		return 0
	}
	w := b.filter[b.chosen].weapon
	app := g.exeI16(wpnAppTbl + uint32(int32(w)*0x1c))
	pl := &g.state.Players[g.state.PlayerNum]
	for i := 1; i < 0xf; i++ {
		switch {
		case i == 0xd:
			if g.exeU8(wpnTypeTbl+uint32(int32(w)*0x1c)) == 1 && pl.AppStatus[appHeavyMount] == 3 {
				g.newModSpace(i)
				b.validMods[i] = 1
				n++
			} else {
				b.validMods[i] = 0xfd
			}
		case int32(g.exeI16(wpnModsTbl+uint32(int32(w)*0x1c)))>>uint(i)&1 == 0:
			b.validMods[i] = 0xfe
		case g.miniaturizationLevel(int(g.state.PlayerNum), int(app)) < int16(g.exeU8(modLevelTbl+uint32(i)*0xf)):
			b.validMods[i] = 0xfa
		default:
			g.newModSpace(i)
			b.validMods[i] = 1
			n++
		}
	}
	return n
}

// weaponModFieldXYs is Get_Weapon_Mod_Field_XYs_ (0x6649d): for each shown
// modification its picture and place, in two columns.
func (g *Game) weaponModFieldXYs() (pic, xs, ys [0xf]int16) {
	b := &g.des.box
	if b.chosen <= -1 {
		return
	}
	colA, colB := int16(0xb), int16(0xb)
	n := int16(0)
	for i := 1; i < 0xf; i++ {
		si := i - 1
		if !g.shouldAddMod(i) {
			continue
		}
		typ := g.weaponTypeOf(b.filter[b.chosen].weapon)
		f := int16(-1)
		if typ <= 2 {
			beam, missile := typ == 0, typ == 1
			switch si {
			case 0, 1, 2, 3, 5, 6, 8:
				f = int16(si)
			case 4:
				f = 0xd
				if beam {
					f = 4
				}
			case 7:
				f = 0xe
				if beam {
					f = 7
				}
			case 9:
				f = 0x10
				if missile {
					f = 0xc
				}
			case 10:
				f = 9
			case 11:
				f = 10
			case 12:
				f = 0xf
				if missile {
					f = 0xb
				}
			case 13:
				f = 0x11
			}
		}
		switch {
		case typ == 0, typ == 2, typ == 3:
			colA, colB = 0x11, 0x11
		case typ == 1:
			colA, colB = 0x15, 0x15
		}
		off := colB
		xs[i] = 0x182
		if n%2 == 0 {
			xs[i] = 0xe9
			off = colA
		}
		ys[i] = off + 0x46 + (n/2)*0x11
		pic[i] = f
		n++
	}
	return
}

// getWeaponsBoxBaseDimensions is Get_Weapons_Box_Base_Dimensions_
// (0x6667c).
func (g *Game) getWeaponsBoxBaseDimensions() {
	d := &g.des
	b, ps := &d.box, &d.pics
	b.height = g.weaponsMidHeight() + int16(ps.wTop.Height) + int16(ps.wBot.Height)
	b.width = 0x27c
	y := (0x195-int32(b.height))/2 + 0x26
	b.baseX = 2
	if y <= 0x26 {
		y = 0x26
	}
	b.baseY = int16(y)
}

// weaponsMidHeight is the middle part's height: 0x66 for fewer than seven
// choices.
func (g *Game) weaponsMidHeight() int16 {
	if g.des.box.bar.rows < 7 {
		return 0x66
	}
	return int16(g.des.pics.wMid.Height)
}

// addReplacementWeaponFields is Add_Replacement_Weapon_Fields_ (0x64b94).
func (g *Game) addReplacementWeaponFields() (cancel, full int, err error) {
	e, d := g.e, &g.des
	b, ps := &d.box, &d.pics
	bar := &b.bar
	none := cStringAt(g.exe, dbNone)
	mid := g.weaponsMidHeight()
	top := int16(ps.wTop.Height)
	cancel, full = dsNoField, dsNoField
	b.scrollF = dsNoField
	g.clearWeaponReplacementFields()
	x0 := b.baseX + 0x16
	y := b.baseY + top
	for i := 0; i < min(int(bar.rows), 10); i++ {
		f, err := e.AddHiddenField(int(x0+6), int(y+0x13+int16(i)*0xe), int(x0+0x240), int(y+0x1f+int16(i)*0xe), none, 0x29)
		if err != nil {
			return 0, 0, err
		}
		b.replF[i] = f
	}
	if bar.rows > 0xa {
		if b.upF, err = e.AddButtonField(int(b.baseX+b.width-0x20), int(y+1), none, ps.up, cStringAt(g.exe, dbKeyUp), 0x28); err != nil {
			return
		}
		if b.scrollF, err = e.AddScrollField(int(b.baseX+0x25d), int(y+0x14), 0, 0x78, 0, 0x78, 9, 0x78, &bar.pos, none, 0); err != nil {
			return
		}
		if b.downF, err = e.AddButtonField(int(b.baseX+b.width-0x20), int(y+mid-0xe), none, ps.down, cStringAt(g.exe, dbKeyDown), 0x28); err != nil {
			return
		}
	} else {
		b.upF, b.scrollF, b.downF = dsNoField, dsNoField, dsNoField
	}
	fy := y + mid + 0xa
	for _, f := range []struct {
		dst    *int
		x1, x2 int16
		key    uint32
	}{{&b.beamF, 0x66, 0xc5, dbKeyBeam}, {&b.missileF, 0xcc, 0x13f, dbKeyMiss}, {&b.bombF, 0x146, 0x1a6, dbKeyBomb}, {&b.spcF, 0x1ad, 0x20f, dbKeySpec}} {
		if *f.dst, err = e.AddHiddenField(int(b.baseX+f.x1), int(fy), int(b.baseX+f.x2), int(fy+0x18), cStringAt(g.exe, f.key), 0x29); err != nil {
			return
		}
	}
	by := y + mid
	if b.chosen > -1 {
		w := b.filter[b.chosen].weapon
		for i := int16(0); i < 5; i++ {
			typ := g.weaponTypeOf(w)
			switch {
			case g.weaponNeedsRacks(w):
				g.weaponSpace(int(g.state.PlayerNum), int(w), 1, b.arcs, g.rackSize(i), int16(b.mods))
				ry := by + 0x46 + 0xe + 0xf*i
				f, err := e.AddHiddenField(int(b.baseX+0x77), int(ry), int(b.baseX+0xcc), int(ry+0xf), none, 0x29)
				if err != nil {
					return 0, 0, err
				}
				b.rackF[i] = f
				b.validArcs[i] = 1
			case b.chosen > -1 && typ == 0:
				ay := by + 0x46 + 0xe + 0x10*i
				g.newArcSpace(int(i))
				f, err := e.AddHiddenField(int(b.baseX+0x77), int(ay), int(b.baseX+0x77+0x5c), int(ay+0xd), none, 0x29)
				if err != nil {
					return 0, 0, err
				}
				b.rack = 0
				b.validArcs[i] = 1
				b.arcF[i] = f
			default:
				b.rack = 0
			}
		}
	}
	ry := by + 0x46
	if b.arcBoxF, err = e.AddHiddenField(int(b.baseX+0x1d), int(ry), int(b.baseX+0x1d+int16(ps.rack.Width)), int(int16(ps.rack.Height)+ry), none, 0x29); err != nil {
		return
	}
	g.validModifications()
	pics, xs, ys := g.weaponModFieldXYs()
	_ = pics
	for i := 1; i < 0xf; i++ {
		if b.chosen <= -1 || !g.modAllowedOnWeaponType(i) || !g.shouldAddMod(i) {
			continue
		}
		my := ys[i] + by
		f, err := e.AddHiddenField(int(xs[i]+b.baseX), int(my), int(xs[i]+b.baseX+0x8c), int(my+0xd), none, 0x29)
		if err != nil {
			return 0, 0, err
		}
		b.modF[i] = f
	}
	cy := b.baseY + top + mid - 9
	if cancel, err = e.AddButtonField(int(b.baseX+0x219), int(cy+0x5b), none, ps.wCancel, cStringAt(g.exe, dbKeyEsc), 0x28); err != nil {
		return
	}
	ok, _ := g.weaponChoiceValid()
	ax, ay := b.baseX+0x21a, cy+0x8b
	if ok {
		b.acceptF, err = e.AddButtonField(int(ax), int(ay), none, ps.wAccept, cStringAt(g.exe, dbKeyOK), 0x28)
	} else {
		b.acceptF, err = e.AddHiddenField(int(ax), int(ay), int(int16(ps.wAccept.Width)+ax), int(int16(ps.wAccept.Height)+ay), none, 0x29)
	}
	if err != nil {
		return
	}
	if _, err = e.AddHiddenField(int(b.baseX), int(b.baseY), int(b.baseX+int16(ps.wTop.Width)), int(b.baseY+top+mid+int16(ps.wBot.Height)), none, 0); err != nil {
		return
	}
	full, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, cStringAt(g.exe, dbKeyEsc), 0x29)
	bar.upField, bar.downField = b.upF, b.downF
	return
}

// clearWeaponReplacementFields is Clear_Weapon_Replacement_Fields_
// (0x654ea).
func (g *Game) clearWeaponReplacementFields() {
	b := &g.des.box
	for i := range b.replF {
		b.replF[i] = dsNoField
	}
	for i := 0; i < 5; i++ {
		b.arcF[i], b.rackF[i] = dsNoField, dsNoField
	}
	for i := range b.modF {
		b.modF[i] = dsNoField
	}
}

// drawWeaponsReplacementBox is Draw_Weapons_Replacement_Box_ (0x65cfa), the
// box's auto function.
func (g *Game) drawWeaponsReplacementBox() error {
	e, d := g.e, &g.des
	b, ps := &d.box, &d.pics
	mid := int(g.weaponsMidHeight())
	top := int(int16(ps.wTop.Height))
	x, y := int(b.baseX), int(b.baseY)
	if err := e.Draw(x, y, ps.wTop); err != nil {
		return err
	}
	ty := int(int16(y + top))
	e.V.SetWindow(x, ty, 0x27f, int(int16(ty+mid)))
	e.V.ClipOn()
	if err := e.Draw(x, ty, ps.wMid); err != nil {
		return err
	}
	e.V.SetWindow(x, ty, 0x256, int(int16(ty+mid)))
	e.V.ClipOn()
	if err := g.printMainWeaponBox(int(int16(x+0x16)), ty); err != nil {
		return err
	}
	e.V.ResetWindow()
	e.V.ClipOff()
	if b.scrollF != dsNoField {
		if _, err := g.drawGenericVerticalScrollBar(&b.bar, g.drawWeaponsScrollBar); err != nil {
			return err
		}
	}
	by := int(int16(ty + mid))
	if err := e.Draw(x, by, ps.wBot); err != nil {
		return err
	}
	if b.chosen > -1 && g.weaponNeedsRacks(b.filter[b.chosen].weapon) {
		if err := g.drawWeaponsRackBox(x, by); err != nil {
			return err
		}
	} else if err := g.drawWeaponsArcBox(x, by); err != nil {
		return err
	}
	if err := g.drawWeaponsModBox(x, by); err != nil {
		return err
	}
	fy := int(int16(by + 0xa))
	for i, p := range []struct {
		pic *gfxPicture
		dx  int
	}{{ps.beam, 0x66}, {ps.missile, 0xcc}, {ps.bomb, 0x146}, {ps.special, 0x1ad}} {
		setAnimationFrame(p.pic, int(b.filterOn[i]))
		if err := e.Draw(int(int16(x+p.dx)), fy, p.pic); err != nil {
			return err
		}
	}
	return nil
}

// drawWeaponsScrollBar is Draw_Weapons_Scroll_Bar_ (0x65f33).
func (g *Game) drawWeaponsScrollBar() error {
	e, d := g.e, &g.des
	b, ps := &d.box, &d.pics
	bar := &b.bar
	ty := int32(int16(ps.wTop.Height)) + int32(b.baseY)
	y1 := int(int16(ty + 0x13))
	x := int(int16(int32(b.baseX) + 0x257))
	ws := g.saveWindowStatus()
	e.V.SetWindow(x, y1, int(int16(x+int(int16(ps.scrollBar.Width)))), int(int16(y1+int(int16(ps.scrollBar.Height))-2)))
	e.V.ClipOn()
	top := ty + 0x14 + int32(bar.track) - (int32(bar.h)/2 + int32(bar.pos))
	lx := int(int16(int32(b.baseX) + 0x25d))
	c := byte(0x5b)
	for i := 0; i < 9; i++ {
		e.V.Line(lx, int(int16(top+1)), lx, int(int16(top+int32(bar.h))), c)
		if i%2 == 0 {
			c--
		}
		lx = int(int16(lx + 1))
	}
	if err := e.Draw(x, y1, ps.scrollBar); err != nil {
		return err
	}
	g.restoreWindowStatus(ws)
	return nil
}

// drawWeaponsArcBox is Draw_Weapons_Arc_Box_ (0x65a2b).
func (g *Game) drawWeaponsArcBox(x, y int) error {
	e, d := g.e, &g.des
	b, ps := &d.box, &d.pics
	var on [5]int
	a := int32(b.arcs)
	switch {
	case a>>1&1 != 0:
		on[1] = 1
	case a>>2&1 != 0:
		on[2] = 1
	case a>>3&1 != 0:
		on[3] = 1
	case a>>4&1 != 0:
		on[4] = 1
	default:
		on[0] = 1
	}
	if err := e.Draw(int(int16(x+0x1d)), int(int16(y+0x46)), ps.rack); err != nil {
		return err
	}
	ty := y + 0x54
	for i := 0; i < 5; i++ {
		setAnimationFrame(ps.arcText[i], on[i])
		if err := e.Draw(int(int16(x+0x77)), int(int16(ty)), ps.arcText[i]); err != nil {
			return err
		}
		if on[i] == 1 {
			if err := e.Draw(int(int16(x+0x21)), int(int16(y+0x49)), ps.arcPict[i]); err != nil {
				return err
			}
		}
		ty += 0x10
	}
	return nil
}

// drawWeaponsRackBox is Draw_Weapons_Rack_Box_ (0x65c32): the "360"
// picture stands for missiles.
func (g *Game) drawWeaponsRackBox(x, y int) error {
	e, d := g.e, &g.des
	b, ps := &d.box, &d.pics
	if err := e.Draw(int(int16(x+0x1d)), int(int16(y+0x46)), ps.rack); err != nil {
		return err
	}
	if err := e.Draw(int(int16(x+0x21)), int(int16(y+0x49)), ps.arcPict[4]); err != nil {
		return err
	}
	ty := y + 0x58
	for i := 0; i < 5; i++ {
		p := ps.rackText[i]
		switch {
		case int16(i) == b.rack:
			setAnimationFrame(p, 2)
		case b.validArcs[i] != 0:
			setAnimationFrame(p, 1)
		default:
			p.CurrentFrame = 0
		}
		if err := e.Draw(int(int16(x+0x7f)), int(int16(ty)), p); err != nil {
			return err
		}
		ty += 0xf
	}
	return nil
}

// drawWeaponsModBox is Draw_Weapons_Mod_Box_ (0x65b25): on 2, allowed 1,
// else 0.
func (g *Game) drawWeaponsModBox(x, y int) error {
	e, d := g.e, &g.des
	b, ps := &d.box, &d.pics
	if err := e.Draw(int(int16(x+0xe0)), int(int16(y+0x46)), ps.blankMods); err != nil {
		return err
	}
	pics, xs, ys := g.weaponModFieldXYs()
	for i := 1; i < 0xf; i++ {
		if b.chosen <= -1 || !g.modAllowedOnWeaponType(i) || !g.shouldAddMod(i) {
			continue
		}
		p := ps.wSpecial[pics[i]]
		switch {
		case b.modStatus[i] == 1:
			setAnimationFrame(p, 2)
		case b.validMods[i] == 1:
			setAnimationFrame(p, 1)
		default:
			setAnimationFrame(p, 0)
		}
		if err := e.Draw(int(int16(int(xs[i])+x)), int(int16(int(ys[i])+y)), p); err != nil {
			return err
		}
	}
	return nil
}

// printMainWeaponBox is Print_Main_Weapon_Box_ (0x66cec): the rows with
// damage, cost and space of one mount; the chosen row with the box's arcs,
// rack and modifications.
func (g *Game) printMainWeaponBox(x, y int) error {
	e, d := g.e, &g.des
	b, t, ps := &d.box, &d.t, &d.pics
	tx := e.Text()
	tx.SetStyle(3, g.fontColors2(3, 0x55, 0x5b))
	hy := int(int16(y + 3))
	for _, h := range []struct {
		dx, n int
		right bool
	}{{0x1c, 0x2e, false}, {0xc6, 0x2f, true}, {0xfd, 0x30, false}, {0xd3, 0x2b, false}, {0x183, 0x31, false}} {
		hx := int(int16(x + h.dx))
		var err error
		if h.right {
			err = tx.PrintRight(hx, hy, g.hMessage(h.n))
		} else {
			err = tx.Print(hx, hy, g.hMessage(h.n))
		}
		if err != nil {
			return err
		}
	}
	idx := b.bar.first
	var cost, space int32
	ry := y + 0x13
	x3 := x + 3
	defSpacing := func() { tx.SetSpacing(tx.Font.Styles[tx.Style].HSpacing) }
	for row := int16(0); ; {
		w := b.filter[idx].weapon
		sel := false
		if idx > 0 {
			switch {
			case b.chosen > -1 && w > 0 && idx == b.chosen:
				var ammo int16
				if g.weaponNeedsRacks(w) {
					ammo = g.rackSize(b.rack)
				}
				cost = int32(g.weaponCost(w, 1, uint8(b.arcs), uint8(ammo), int16(b.mods)))
				space = g.weaponSpace(int(g.state.PlayerNum), int(w), 1, b.arcs, ammo, int16(b.mods))
			case w > 0:
				var ammo int16
				if g.weaponNeedsRacks(w) {
					ammo = 5
				}
				cost = int32(g.weaponCost(w, 1, 1, uint8(ammo), 0))
				space = g.weaponSpace(int(g.state.PlayerNum), int(w), 1, 1, ammo, 0)
			}
		}
		if b.chosen > -1 && idx == b.chosen {
			sel = true
		}
		ok := idx > -1 && t[dtChoiceOK+int(idx)] != 0
		switch {
		case b.scanned == row && ok:
			tx.SetStyle(2, g.fontColors2(2, 0x55, 0x58))
			g.increaseFontColorRange(int(g.ms.multiColoredLineStart))
		case sel:
			tx.SetStyle(2, g.fontColors2(2, 0x55, 0x5c))
		case ok:
			tx.SetStyle(2, g.fontColors2(2, 0x55, 0x5a))
		case idx > -1:
			tx.SetStyle(2, g.fontColors2(2, 0x55, 0x57))
		default:
			tx.SetStyle(2, g.fontColors2(2, 0x55, 0x59))
		}
		costS, spaceS := itoa(int(cost)), itoa(int(space))
		yy := int(int16(ry))
		if sel {
			if err := e.Draw(int(int16(x3-3)), int(int16(ry+1)), ps.wSel); err != nil {
				return err
			}
		}
		// The percentage divides 0 by the space.
		var pct int32
		if space > 0 && ok {
			pct = 1
		}
		var name string
		if w > 0 {
			n, err := g.techName(walksWeapons + 1 + int(w))
			if err != nil {
				return err
			}
			name = cSprintf(cStringAt(g.exe, dbFmtCount), n, int16(pct))
		} else {
			name = g.hMessage(0x32)
		}
		tx.SetSpacing(1)
		if err := tx.Print(int(int16(x3+9)), yy, name); err != nil {
			return err
		}
		defSpacing()
		dmg := g.designWeaponDamageString(w, b.chosen, idx)
		dx := int(int16(x3 + 0xc3))
		if dmg == g.hMessage(0x65) {
			g.changeFontStyle(1)
			if err := tx.PrintRight(dx, int(int16(ry+2)), dmg); err != nil {
				return err
			}
			g.changeFontStyle(2)
		} else if err := tx.PrintRight(dx, yy, dmg); err != nil {
			return err
		}
		if err := tx.PrintRight(int(int16(x3+0xe6)), yy, costS); err != nil {
			return err
		}
		if err := tx.PrintRight(int(int16(x3+0x11c)), yy, spaceS); err != nil {
			return err
		}
		if w > 0 {
			tx.SetStyle(2, tx.Current)
			tx.SetSpacing(1)
			if err := tx.Print(int(int16(x+0x131)), yy, string(d.weaponNotes[w])); err != nil {
				return err
			}
			defSpacing()
		}
		row++
		idx = b.bar.first + row
		ry += 0xe
		if row >= 0xa || idx >= b.bar.rows {
			break
		}
	}
	g.boxCycle(2)
	return nil
}

// printWeaponModificationMessage is Print_Weapon_Modification_Message_
// (0x67ccf): why a modification cannot be switched on.
func (g *Game) printWeaponModificationMessage(i int) error {
	d := &g.des
	b, t := &d.box, &d.t
	modName, _ := g.techName(walksMods2 - 15 + 1 + i)
	var msg string
	switch b.validMods[i] {
	case 0xf9:
		sp := int16(g.newModSpace(i))
		msg = cSprintf(g.lbxMessage(0x3a), modName, int32(sp)+t.i32(dtSpaceUsed)-t.i32(dtSpaceTotal))
	case 0xfa:
		app := g.exeI16(wpnAppTbl + uint32(int32(b.filter[b.chosen].weapon)*0x1c))
		lvl := g.miniaturizationLevel(int(g.state.PlayerNum), int(app))
		need := int32(g.exeU8(modLevelTbl + uint32(i)*0xf))
		n := 0x3c
		if need-int32(lvl) == 1 {
			n = 0x3b
		}
		appName, _ := g.techAppName(int(app))
		msg = cSprintf(g.lbxMessage(n), need-int32(lvl), appName, modName)
	case 0xfb:
		msg = g.hMessage(0x3d)
	case 0xfc:
		msg = g.hMessage(0x3e)
	case 0xfd:
		msg = g.hMessage(0x3f)
	case 0xfe:
		wn, _ := g.techName(walksWeapons + 1 + int(b.filter[b.chosen].weapon))
		msg = cSprintf(g.lbxMessage(0x40), modName, wn)
	}
	if msg == "" {
		return nil
	}
	return g.messageBox(msg)
}

// printWeaponModHelpDescription is Print_Weapon_Mod_Help_Description_
// (0x67bc7): help record 400 and the modification's record.
func (g *Game) printWeaponModHelpDescription(i int) error {
	a, err := g.archive(g.helpLBXName())
	if err != nil {
		return err
	}
	tab, err := a.Table(0)
	if err != nil {
		return err
	}
	text := string(cBytes(tab.Record(400)[0x67:])) + cStringAt(g.exe, dbNewLines)
	rec := tab.Record(400 + i)
	text += string(cBytes(rec[0x67:]))
	title := string(cBytes(rec))
	return g.textBox(&title, text)
}

// weaponReplacementHelp is Weapon_Replacement_Help_ (0x6953b) with
// Set_Weapon_Replacement_Screen_Help_List_ (0x6f5c1).
func (g *Game) weaponReplacementHelp(chosen int16) {
	d := &g.des
	b, ps := &d.box, &d.pics
	racks := chosen > -1 && g.weaponNeedsRacks(b.filter[chosen].weapon)
	bx := int(b.baseX)
	by := int(g.weaponsMidHeight()) + int(int16(ps.wTop.Height)) + int(b.baseY)
	l := make([]engine.HelpEntry, 0, 7)
	y := by + 0xa
	for _, f := range []struct {
		id, dx int
		pic    *gfxPicture
	}{{0x188, 0x65, ps.beam}, {0x189, 0xcb, ps.missile}, {0x18a, 0x146, ps.bomb}, {0x18b, 0x1ad, ps.special}} {
		x1 := bx + f.dx
		l = append(l, engine.HelpEntry{ID: f.id, X1: x1, Y1: y, X2: x1 + int(int16(f.pic.Width)), Y2: y + int(int16(f.pic.Height))})
	}
	x1 := bx + 0x21a
	l = append(l, engine.HelpEntry{ID: 0x18c, X1: x1, Y1: by + 0x52, X2: x1 + int(int16(ps.wCancel.Width)), Y2: by + 0x52 + int(int16(ps.wCancel.Height))})
	l = append(l, engine.HelpEntry{ID: 0x18d, X1: x1, Y1: by + 0x82, X2: x1 + int(int16(ps.wAccept.Width)), Y2: by + 0x82 + int(int16(ps.wAccept.Height))})
	id := 0x18e
	if racks {
		id++
	}
	l = append(l, engine.HelpEntry{ID: id, X1: bx + 0x18, Y1: by + 0x45, X2: bx + 0xd2, Y2: by + 0xaa})
	for i := range l {
		e := &l[i]
		e.X1, e.Y1, e.X2, e.Y2 = int(int16(e.X1)), int(int16(e.Y1)), int(int16(e.X2)), int(int16(e.Y2))
	}
	g.e.DeactivateHelpList()
	g.e.SetHelpList(l)
}
