package game

import (
	"strings"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/save"
)

// Drawing of the design screen (Print_Current_Design_ and its helpers).
// Spec: docs/screens/design-screen.md, "Drawing".

// printCurrentDesign is Print_Current_Design_ (0x6d29c).
func (g *Game) printCurrentDesign() error {
	e, d := g.e, &g.des
	t := &d.t
	v := e.V
	tx := e.Text()
	rec := g.buildDesignTemplate()
	defense, evasion, _ := g.designCombatBonuses(&rec)
	v.ResetWindow()
	v.ClipOff()
	tx.SetOutlineColor(0, 0)
	c3 := g.moxFontColors(3, 0x55, 0x5b)
	c2 := g.moxFontColors(2, 0x55, 0x5b)
	big := func() { tx.SetStyle(3, c3); tx.SetSpacing(1) }
	small := func() { tx.SetStyle(2, c2); tx.SetSpacing(1) }
	pr := func(x, y int, s string) error { return tx.Print(x, y, s) }
	big()
	name, err := g.techName(walksDrives + 1 + int(t.i16(dtDrive)))
	if err != nil {
		return err
	}
	if err := pr(0xec, 0x38, name); err != nil {
		return err
	}
	small()
	if err := pr(0xfb, 0x47, cSprintf(g.hMessage(0x17e), g.exeI16(driveSpeedTbl+uint32(int32(t.i16(dtDrive))*0x2e)))); err != nil {
		return err
	}
	if err := pr(0xfb, 0x56, cSprintf(g.hMessage(0x54), g.currentDesignCombatSpeed(t))); err != nil {
		return err
	}
	big()
	if name, err = g.techName(walksArmor + 1 + int(t.i16(dtArmor))); err != nil {
		return err
	}
	if err := pr(0xec, 0x6b, name); err != nil {
		return err
	}
	small()
	st := g.getStructure(t.size(), t.hasSpecial(specReinforced), int(g.state.PlayerNum))
	if err := pr(0xfb, 0x7a, cSprintf(g.hMessage(0x55), st)); err != nil {
		return err
	}
	if err := pr(0xfb, 0x89, cSprintf(g.hMessage(0x56), int16(g.currentDesignArmorAbsorption()))); err != nil {
		return err
	}
	big()
	if g.ms.scannedField == int16(d.shieldF) {
		g.setFlashingColors(3, 1)
	}
	var line3 string
	if t.i16(dtShield) == 0 {
		if err := pr(0x1b5, 0x38, g.hMessage(0x36)); err != nil {
			return err
		}
		small()
		if err := pr(0x1c5, 0x47, g.hMessage(0x57)); err != nil {
			return err
		}
		line3 = g.hMessage(0x58)
	} else {
		if name, err = g.techName(walksShields + 1 + int(t.i16(dtShield))); err != nil {
			return err
		}
		if err := pr(0x1b5, 0x38, name); err != nil {
			return err
		}
		small()
		if err := pr(0x1c5, 0x47, cSprintf(g.hMessage(0x59), int16(g.shieldStrength()))); err != nil {
			return err
		}
		b := g.exeI16(shieldValueTbl + uint32(int32(t.i16(dtShield))*0x3b))
		if t.hasSpecial(specHardShields) {
			b += 3
		}
		suffix := cStringAt(g.exe, dsShieldPlural)
		if b == 1 {
			suffix = ""
		}
		line3 = cSprintf(g.hMessage(0x5a), b, suffix)
	}
	if err := pr(0x1c5, 0x55, line3); err != nil {
		return err
	}
	big()
	if g.ms.scannedField == int16(d.compF) {
		g.setFlashingColors(3, 1)
	}
	if name, err = g.techName(walksComputers + 1 + int(t.i16(dtComputer))); err != nil {
		return err
	}
	if err := pr(0x1b5, 0x61, name); err != nil {
		return err
	}
	small()
	if err := pr(0x1c5, 0x70, cSprintf(g.hMessage(0x5b), g.exeI16(compAttackTbl+uint32(int32(t.i16(dtComputer))*0x16)))); err != nil {
		return err
	}
	big()
	if err := pr(0x1b5, 0x7d, g.hMessage(0x5c)); err != nil {
		return err
	}
	sign := cStringAt(g.exe, dsPlus)
	if defense < 0 {
		sign = cStringAt(g.exe, dsMinus)
	}
	if err := tx.PrintRight(0x26a, 0x7d, cSprintf(cStringAt(g.exe, dsFmtSignInt), sign, defense)); err != nil {
		return err
	}
	if err := pr(0x1b5, 0x89, g.hMessage(0x5d)); err != nil {
		return err
	}
	if err := tx.PrintRight(0x26a, 0x89, cSprintf(cStringAt(g.exe, dsFmtPercent), evasion)); err != nil {
		return err
	}
	if err := g.printDesignWeapons(c2, c3); err != nil {
		return err
	}
	if err := g.printDesignSpecials(c2, c3); err != nil {
		return err
	}
	v.ClipOff()
	v.ResetWindow()
	pic, err := g.shipPictureSeg(int(g.state.PlayerNum), int(t.i16(dtPicture)))
	if err != nil {
		return err
	}
	if err := e.Draw(int(int16((0x36-int(int16(pic.Width)))/2+0x27)), int(int16((0x3a-int(int16(pic.Height)))/2+0x3a)), pic); err != nil {
		return err
	}
	tx.SetStyle(3, c3)
	if err := pr(0x19, 0x81, g.hMessage(0x30)); err != nil {
		return err
	}
	if err := tx.PrintRight(0x6a, 0x81, itoa(int(t.i32(dtSpaceTotal)))); err != nil {
		return err
	}
	tx.SetStyle(4, g.moxFontColors(4, 0x55, 0x59))
	y := int(int16((0x18-tx.Height())/2 + 2 + 0x1ba))
	if err := pr(0x18, y, g.hMessage(0x185)); err != nil {
		return err
	}
	if err := tx.PrintRight(0x87, y, itoa(int(g.printedDesignCost()))); err != nil {
		return err
	}
	if err := pr(0xa3, y, g.hMessage(0x63)); err != nil {
		return err
	}
	return tx.PrintRight(0x160, y, itoa(int(d.printedSpc)))
}

// printDesignWeapons is Print_Current_Design_'s weapon list in the window
// 0x10, 0x9b..0x272, 0x11c, with "Add New Weapon" under an empty row.
func (g *Game) printDesignWeapons(c2, c3 font.Colors) error {
	e, d := g.e, &g.des
	t := &d.t
	tx := e.Text()
	tx.SetSpacing(2)
	e.V.SetWindow(0x10, 0x9b, 0x272, 0x11c)
	e.V.ClipOn()
	tx.SetStyle(3, c3)
	for _, h := range []struct{ x, n int }{{0x53, 0x5e}, {0xf9, 0x5f}, {0x13c, 0x60}, {0x19e, 0x30}, {0x167, 0x185}, {0x1df, 0x61}} {
		if err := tx.Print(h.x, 0x9c, g.hMessage(h.n)); err != nil {
			return err
		}
	}
	y := 0xaa
	empty := false
	tx.SetStyle(2, c2)
	for i := 0; i < 8; i++ {
		k := int(int16(uint16(d.firstWpn) + uint16(i)))
		hov := int(g.ms.scannedField) == i+1000
		switch {
		case hov && t.weapon(k) > 0:
			g.setFlashingColors(2, 2)
		case hov:
			empty = true
		default:
			tx.SetStyle(2, c2)
		}
		if t.weapon(k) <= 0 {
			continue
		}
		// The modifications and arcs are read from row i, not slot k.
		mods := g.weaponModString(t.mods(i))
		arc, err := g.weaponArcString(t.arc(i))
		if err != nil {
			return err
		}
		walk := walksWeapons
		if t.count(k) > 1 {
			walk = walksWeaponsPl
		}
		name, err := g.techName(walk + 1 + int(t.weapon(k)))
		if err != nil {
			return err
		}
		if g.weaponTypeOf(t.weapon(k)) == 1 {
			name = cSprintf(cStringAt(g.exe, dsFmtNameCount), name, t.ammo(k))
		}
		if err := tx.Print(0x55, y, name); err != nil {
			return err
		}
		if err := tx.PrintRight(0x31, y, itoa(int(t.count(k)))); err != nil {
			return err
		}
		dmg := g.designWeaponDamageString(int16(k), -1, -1)
		if dmg == cStringAt(g.exe, dsSpecialWord) {
			g.changeFontStyle(1)
			if err := tx.PrintCentered(0x10b, y, dmg); err != nil {
				return err
			}
			g.changeFontStyle(2)
		} else if err := tx.PrintCentered(0x10b, y, dmg); err != nil {
			return err
		}
		if err := tx.PrintCentered(0x149, y, arc); err != nil {
			return err
		}
		per := func(v int32) int32 {
			if c := int32(t.count(k)); c != 0 && v/c > 1 {
				return v / c
			}
			return 1
		}
		if err := tx.PrintCentered(0x1b4, y, itoa(int(per(t.wspace(k))))); err != nil {
			return err
		}
		if err := tx.PrintCentered(0x179, y, itoa(int(per(t.i32(dtWeaponCost+4*k))))); err != nil {
			return err
		}
		if err := tx.PrintCentered(0x211, y, mods); err != nil {
			return err
		}
		y += 0xd
	}
	if empty {
		if left := 8 - int(d.nWeapons); left > 0 {
			if left > 1 {
				y = int(int16(y)) + (0x7f-int(d.nWeapons)*0xd)/2 - 0xe
			}
			msg := 0x17f
			if g.spaceForWeaponReplacement() {
				tx.SetStyle(2, c2)
				g.setFlashingColors(2, 2)
			} else {
				tx.SetStyle(2, g.fontColors2(2, 0x49, 0x4d))
				g.setFlashingColors(2, 2)
				msg = 0x18
				if d.printedSpc <= 0 {
					msg = 0x1a
				}
			}
			if err := tx.PrintCentered((0x280-0x13)/2, int(int16(y)), g.hMessage(msg)); err != nil {
				return err
			}
		}
	}
	e.V.ClipOff()
	return nil
}

// printDesignSpecials is Print_Current_Design_'s special system list in the
// window 0x11, 0x131..0x272, 0x1b0.
func (g *Game) printDesignSpecials(c2, c3 font.Colors) error {
	e, d := g.e, &g.des
	t := &d.t
	tx := e.Text()
	e.V.SetWindow(0x11, 0x131, 0x272, 0x1b0)
	e.V.ClipOn()
	tx.SetStyle(3, c3)
	if err := tx.Print(0x1e, 0x132, g.hMessage(0x62)); err != nil {
		return err
	}
	if err := tx.Print(0x118, 0x132, g.hMessage(0x38)); err != nil {
		return err
	}
	y := 0x140
	empty := false
	tx.SetStyle(2, c2)
	for i := 0; i < 8; i++ {
		k := int(int16(uint16(d.firstSpc) + uint16(i)))
		hov := int(g.ms.scannedField) == i+2000
		switch {
		case hov && t.special(k) > 0:
			tx.SetStyle(2, c2)
			g.setFlashingColors(2, 2)
		case hov:
			empty = true
		default:
			tx.SetStyle(2, c2)
		}
		sp := t.special(k)
		if sp <= 0 {
			continue
		}
		name, err := g.techName(walksSpecials + 1 + int(sp))
		if err != nil {
			return err
		}
		if err := tx.Print(0x23, y, name); err != nil {
			return err
		}
		if err := tx.Print(0x10e, y, string(d.specialDesc[t.special(k)])); err != nil {
			return err
		}
		y += 0xd
	}
	if empty {
		if left := 8 - int(d.nSpecials); left > 0 {
			if left > 1 {
				y = int(int16(y)) + (0x7d-0xd*int(d.nSpecials))/2 - 0xe
			}
			msg := 0x180
			if g.spaceForSystemReplacement() {
				tx.SetStyle(2, c2)
				g.setFlashingColors(2, 2)
			} else {
				tx.SetStyle(2, g.fontColors2(2, 0x49, 0x4e))
				g.setFlashingColors(2, 2)
				msg = 0x19
				if d.printedSpc <= 0 {
					msg = 0x1b
				}
			}
			if err := tx.PrintCentered((0x280-0x14)/2, int(int16(y)), g.hMessage(msg)); err != nil {
				return err
			}
		}
	}
	return nil
}

// printedDesignCost is Printed_Design_Cost_ (0x6b949): a refit shows its
// cost against the design in the slot.
func (g *Game) printedDesignCost() int16 {
	if !g.bq.refitAdd {
		return int16(g.des.printedCst)
	}
	rec := g.buildDesignTemplate()
	p := int(g.state.PlayerNum)
	return int16(g.refitCostDesigns(p, g.designRecord(p, int(g.screenData[4])), &rec))
}

// refitCostDesigns is Refit_Cost_ (0xd108b) on two design records.
func (g *Game) refitCostDesigns(p int, old, d *save.Design) int32 {
	floor := g.costReductionForGovtType(p, int32(g.exeI16(hullCostTable+uint32(d.Size)*0x24))/2)
	v := (int32(d.Cost) - int32(old.Cost)) * 2
	if v < floor {
		v = floor
	}
	return v
}

// getStructure is Get_Structure_ (0x58330): the hull's structure with the
// player's best armour's bonus (not the design's), x3 reinforced.
func (g *Game) getStructure(size int16, reinforced bool, p int) int16 {
	a := g.exeI16(armorHitsPctTbl + uint32(int32(int16(g.bestArmor(p)))*0xf))
	h := int32(g.exeI16(0x180026 + uint32(int32(size)*0x24)))
	v := int32(a)*h/100 + h
	if reinforced {
		v *= 3
	}
	return int16(v)
}

// currentDesignArmorAbsorption is Current_Design_Armor_Absorption_
// (0x6b77b).
func (g *Game) currentDesignArmorAbsorption() int32 {
	t := &g.des.t
	h := int32(g.exeI16(hullArmorTbl + uint32(int32(t.size())*0x24)))
	a := int32(g.exeI16(armorHitsPctTbl + uint32(int32(t.i16(dtArmor))*0xf)))
	v := h + a*h/100
	if t.hasSpecial(specHeavyArmor) {
		v *= 3
	}
	return v
}

// shieldStrength is Shield_Strength_ (0x6e3d3).
func (g *Game) shieldStrength() int32 {
	t := &g.des.t
	n := int32(t.size()) + 1
	if t.size() >= 5 {
		n++
	}
	v := int16(int32(int16(g.exeI16(shieldValueTbl+uint32(int32(t.i16(dtShield))*0x3b))*5)) * n)
	if t.hasSpecial(specShieldPlus) {
		v = int16(int32(v) * 3 / 2)
	}
	return int32(v)
}

// weaponModString is Weapon_Mod_String_ (0x6f07f): the modifications'
// names joined by ", ", or H 0x64.
func (g *Game) weaponModString(mods int16) string {
	var b strings.Builder
	any := false
	for bit := 0; bit < 0xf; bit++ {
		if int32(mods)>>bit&1 == 0 {
			continue
		}
		n, err := g.techName(walksMods2 + 1 + bit)
		if err != nil {
			g.logOnce(err.Error())
		}
		b.WriteString(n)
		b.WriteString(cStringAt(g.exe, dsComma))
		any = true
	}
	s := b.String()
	if !any {
		s = g.hMessage(0x64)
	}
	if n := int16(len(s)); n > 2 && s[n-2] == ',' {
		s = s[:n-2]
	}
	return s
}

// designWeaponDamageString is Design_Weapon_Damage_String_ (0x6c47e). With
// b == c == -1, a is a template slot and its modifications count; else a
// is a weapon and, when b == c, the weapons box's chosen modifications.
func (g *Game) designWeaponDamageString(a, b, c int16) string {
	t := &g.des.t
	slotMode := c == -1 && b == -1
	w := a
	if slotMode {
		w = t.weapon(int(a))
	}
	lo := g.exeI16(wpnMinDmgTbl + uint32(int32(w)*0x1c))
	hi := g.exeI16(wpnMaxDmgTbl + uint32(int32(w)*0x1c))
	special := ""
	switch w {
	case 0xd, 0x1c, 0x20, 0x21, 0x22, 0x1a, 0x19, 0x1f, 0x1e, 0x1d, 0x24, 0x23, 0x25, 0x27:
		special = g.hMessage(0x65)
	}
	half := func() {
		lo = int16((int32(lo)*0x32 + 0x32) / 100)
		v := int32(hi) * 0x32 / 100
		if int32(lo) >= v {
			hi = lo
		} else {
			hi = int16(v)
		}
	}
	boost := func() {
		lo = int16(int32(lo) * 0x96 / 100)
		hi = int16(int32(hi) * 0x96 / 100)
	}
	st := &g.des.box.modStatus
	switch {
	case slotMode:
		m := uint16(t.mods(int(a)))
		switch {
		case m&0x4000 != 0:
			boost()
		case m&4 != 0:
			half()
		case m&2 != 0:
			boost()
		}
	case b == c:
		switch {
		case st[0xe] > 0:
			boost()
		case st[2] > 0:
			half()
		case st[1] > 0:
			boost()
		}
	}
	switch {
	case w == 0:
		return cStringAt(g.exe, dsZero)
	case special != "":
		return special
	case hi == lo:
		return cSprintf(cStringAt(g.exe, dsFmtInt), hi)
	}
	return cSprintf(cStringAt(g.exe, dsFmtRange), lo, hi)
}

// techAppToTechID is Tech_App_To_Tech_Id_ (0x6e69e): the first weapon (type
// 3) or special (type 9) row of an application, -1 for none.
func (g *Game) techAppToTechID(app int16, typ uint16) int16 {
	switch typ {
	case 3:
		for w := int16(0); w < 0x28; w++ {
			if g.exeI16(wpnAppTbl+uint32(int32(w)*0x1c)) == app {
				return w
			}
		}
	case 9:
		for s := int16(0); s < 0x27; s++ {
			if g.exeI16(specAppTbl+uint32(int32(s)*0x2f)) == app {
				return s
			}
		}
	}
	return -1
}

// oneWeaponSpace is One_Weapon_Space_ (0x6edc6): one mount after
// miniaturisation, missiles with their smallest rack.
func (g *Game) oneWeaponSpace(w int16) int32 {
	app := g.exeI16(wpnAppTbl + uint32(int32(w)*0x1c))
	sp := func(v int16) int32 {
		cat := g.miniaturizationCategory(app)
		return int32(spaceGivenMLevel(v, g.techLevel(int(g.state.PlayerNum), int(app)), cat))
	}
	base := sp(g.exeI16(wpnSpaceTbl + uint32(int32(w)*0x1c)))
	if g.weaponTypeOf(w) == 1 {
		return base + sp(10)
	}
	return base
}

// spaceForWeaponReplacement is Space_For_Weapon_Replacement_ (0x6e571): a
// known weapon fits the space shown.
func (g *Game) spaceForWeaponReplacement() bool {
	pl := &g.state.Players[g.state.PlayerNum]
	for a := 0; a < 0xd4; a++ {
		if pl.AppStatus[a] != 3 || g.exeU8(appTypeTbl+uint32(a)*0xd) != 3 {
			continue
		}
		if int32(int16(g.oneWeaponSpace(g.techAppToTechID(int16(a), 3)))) <= g.des.printedSpc {
			return true
		}
	}
	return false
}

// spaceForSystemReplacement is Space_For_System_Replacement_ (0x6e42e): a
// known special not fitted yet fits the hull.
func (g *Game) spaceForSystemReplacement() bool {
	t := &g.des.t
	pl := &g.state.Players[g.state.PlayerNum]
	for a := 0; a < 0xd4; a++ {
		if pl.AppStatus[a] != 3 || g.exeU8(appTypeTbl+uint32(a)*0xd) != 9 {
			continue
		}
		for s := int16(1); s < 0x27; s++ {
			if g.exeI16(specAppTbl+uint32(int32(s)*0x2f)) != g.exeI16(techAppTable+4+uint32(a)*0xd) {
				continue
			}
			if t.hasSpecial(s) {
				continue
			}
			app := g.exeI16(specAppTbl + uint32(int32(s)*0x2f))
			v := spaceGivenMLevel(g.exeI16(specSpaceTbl+uint32(int32(s)*0x2f+2*int32(t.size()))), g.techLevel(int(g.state.PlayerNum), int(app)), g.miniaturizationCategory(app))
			if int32(v)+t.i32(dtSpaceUsed) <= t.i32(dtSpaceTotal) {
				return true
			}
		}
	}
	return false
}

// applicationTextBox is Get_Application_Description_ (0x6fbea) and
// Text_Box_ with the item's name: the help text of an application.
func (g *Game) applicationTextBox(app int16, nameWalk int) error {
	a, err := g.archive(g.helpLBXName())
	if err != nil {
		return err
	}
	tab, err := a.Table(0)
	if err != nil {
		return err
	}
	desc := string(cBytes(tab.Record(int(app))[0x67:]))
	name, err := g.techName(nameWalk)
	if err != nil {
		return err
	}
	return g.textBox(&name, desc)
}
