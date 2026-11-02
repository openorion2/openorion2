package game

import (
	"fmt"
	"strings"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The detailed ship view: Detailed_View_Ship_ (0x31f25), the popup a right
// click on a ship opens, drawn by Combat_Draw_View_Ship_ (0x30631). Spec:
// docs/screens/ship-view.md.

// shipView is the state the popup's draw routine reads: the combat ship
// record and the module globals it sets up.
type shipView struct {
	rec      viewShip
	icon     *gfx.Picture // _detail_fleet_icon_seg
	colorKen font.Colors  // _color_ken, 0x1a125c
	green    [8]byte      // _green, 0x19b544
	colonyID int          // _cmbt_colony_id, a combat global
}

// detailedViewShip is Detailed_View_Ship_ (0x31f25). The record is the
// global _global_combat_data; here it lives for the call, so the fields only
// combat writes read as zero, and ship -1 shows an empty record.
func (g *Game) detailedViewShip(ship int) error { return g.detailedViewShipRec(ship, nil) }

// detailedViewShipRec is Detailed_View_Ship_ with the record it reads when
// ship is -1: combat's scan copies a combat record there first.
func (g *Game) detailedViewShipRec(ship int, rec *viewShip) error {
	e := g.e
	v := e.V
	none := ship == -1
	sv := &shipView{}
	if rec != nil {
		sv.rec = *rec
	}
	helpWasActive := e.HelpActive() // Save_Help_List_
	saved := e.GetAutoFunction()
	v.SetPageBack()
	g.sd.shipFrame--
	if err := saved.Run(); err != nil {
		return err
	}
	if err := g.drawVisibleFieldsBack(); err != nil {
		return err
	}
	v.CopyBackToOff()
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.AssignAutoFunction(func() error { return g.combatDrawViewShip(sv) }, 1)
	var name [16]byte
	if !none {
		if err := g.loadDisplayCombatShip(&sv.rec, ship); err != nil {
			return err
		}
		name = sv.rec.name
		var err error
		sv.rec.defense, sv.rec.evasion, sv.rec.attack = g.getShipCombatBonuses(ship)
		p, err := g.cached("COMBAT.LBX", 0xb)
		if err != nil {
			return err
		}
		if err := e.CalculatePictureRemapColors(p); err != nil {
			return err
		}
		r := e.PictureRemap()
		sv.colorKen = font.Colors{r[0x84], r[0x8c], r[0x8a], r[0x8c], r[0x8c], r[0x8c], r[0x8c], r[0x8c]}
	}
	var err error
	if o := sv.rec.officer; o == -1 {
		p := sv.rec.owner
		if p > 7 {
			p = 0
		}
		sv.icon, err = g.picture("FLTICONS.LBX", int(g.state.Players[p].Race))
	} else {
		mode := 0x15
		if none {
			mode = 0x115
		}
		sv.icon, err = g.picture(cStringAt(g.exe, g.exe.u32(officerLBXName)), mode+int(g.state.Leaders[o].Picture))
	}
	if err != nil {
		return err
	}
	stats := e.SaveFieldStats() // Mark_Fields
	if !none {
		e.Text().SetStyle(4, sv.colorKen)
		if _, err := e.AddContinuousStringInputField(0x106, 0x32, 0x67, 0xc, name[:], 0xf, 0, nil, 0, 0); err != nil {
			return err
		}
		if _, err := e.AddHiddenField(0x10c, 0x32, 0x17f, 0x40, "", 0); err != nil {
			return err
		}
	}
	whole, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0)
	if err != nil {
		return err
	}
	for done := false; !done; {
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		if r == whole {
			done = true
		}
		v.SetPageOff()
		if err := g.combatDrawViewShip(sv); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	if !none {
		if t := trimSpaces(save.CString(name[:])); t != "" {
			d := &g.state.Ships[ship].Design.Name
			n := copy(d[:], t)
			if n < len(d) {
				d[n] = 0
			}
		}
	}
	e.RestoreFieldStats(stats) // Release_Fields
	if helpWasActive {         // Restore_Help_List_
		e.SetHelpActive(true)
	} else {
		e.DeactivateHelpList()
	}
	e.PutAutoFunction(saved)
	return nil
}

// trimSpaces is Trim (0x13303e): leading and trailing blanks only.
func trimSpaces(s string) string { return strings.Trim(s, " ") }

// drawVisibleFieldsBack is Draw_Visible_Fields_Back_ (0x1174cb):
// Draw_Visible_Fields onto the back page.
func (g *Game) drawVisibleFieldsBack() error {
	v := g.e.V
	v.Off, v.Back = v.Back, v.Off
	err := g.e.DrawVisibleFields()
	v.Off, v.Back = v.Back, v.Off
	v.Current = v.Back
	return err
}

// interlacedFill is Interlaced_Fill (0x128ede): rows from y1 alternate c1
// and c2, clipped to the window when clipping is on.
func (g *Game) interlacedFill(x1, y1, x2, y2 int, c1, c2 byte) {
	v := g.e.V
	if v.Clipping {
		x1, y1 = max(x1, v.WinX1), max(y1, v.WinY1)
		x2, y2 = min(x2, v.WinX2), min(y2, v.WinY2)
	}
	if x1 > x2 || y1 > y2 {
		return
	}
	if v.RefreshMode() > engine.RefreshMasked {
		v.AddSquare(x1, y1, x2, y2)
	}
	for y := y1; y <= y2; y++ {
		c := c1
		if (y-y1)%2 == 1 {
			c = c2
		}
		for x := x1; x <= x2; x++ {
			if o := y*engine.W + x; o >= 0 && o < len(v.Current) {
				v.Current[o] = c
			}
		}
	}
}

// box is Box (0x1297c3): a rectangle outline thick pixels wide.
func (g *Game) box(x, y, w, h, thick int, c byte) {
	v := g.e.V
	for i := 0; i < thick; i++ {
		r, b := x+w-1-i, y+h-1-i
		v.Line(x+i, y+i, r, y+i, c)
		v.Line(x+i, y+i, x+i, b, c)
		v.Line(r, y+i, r, b, c)
		v.Line(x+i, b, r, b, c)
	}
}

// remapDrawShipStatus is Remap_Draw_Ship_Status_ (0x33199): the ship
// picture centred in a 98x98 bitmap, wrapped in shield rings by shield
// class.
func (g *Game) remapDrawShipStatus(sv *shipView, x, y int) error {
	src, err := g.shipPicture(int(sv.rec.id))
	if err != nil {
		return err
	}
	var ds gfx.Picture
	ds.DrawPictureToBitmap(src)
	cb := gfx.NewBlankBitmap(0x62, 0x62)
	if err := cb.AddPicture(0x10, 0xb, &ds); err != nil {
		return err
	}
	if s := sv.rec.shield; s != 0 {
		gr := &sv.green
		rings := [6][]byte{
			1: {1, 2, gr[2], gr[4]},
			2: {1, 2, gr[1], gr[2], gr[3], gr[5]},
			3: {1, 2, gr[0], gr[1], gr[3], gr[4], gr[5], gr[6]},
			4: {1, gr[0], gr[1], gr[2], gr[3], gr[4], gr[5], gr[6], gr[7]},
			5: {gr[0], gr[1], gr[2], gr[2], gr[3], gr[4], gr[4], gr[5], gr[6], gr[7]},
		}
		cb.Outline(cb, 1, false)
		cb.Outline(cb, 2, false)
		if s < 6 {
			for _, c := range rings[s] {
				cb.Outline(cb, c, false)
			}
		}
		if err := cb.ReplaceColor(1, 0); err != nil {
			return err
		}
		if err := cb.ReplaceColor(2, 0); err != nil {
			return err
		}
	}
	return g.e.Draw(x, y, cb)
}

// itoa is Watcom's itoa in base 10.
func itoa(n int) string { return fmt.Sprintf("%d", n) }

// combatDrawViewShip is Combat_Draw_View_Ship_ (0x30631), the popup's draw
// and auto function.
func (g *Game) combatDrawViewShip(sv *shipView) error {
	e := g.e
	t := e.Text()
	r := &sv.rec
	frame, err := g.cached("COMBAT.LBX", 5)
	if err != nil {
		return err
	}
	X := 0x149 - int(int16(frame.Width))/2
	Y := 0xf9 - int(int16(frame.Height))/2
	m666 := r.driveCur == shipViewOffMinus666
	var c1c byte
	var b4, ac, a4 font.Colors
	gr := &sv.green
	mainScreens := g.currentScreen == 0 || g.currentScreen == 4 || g.currentScreen == 0x1d
	if mainScreens {
		rm := e.PictureRemap()
		for i := range gr {
			gr[i] = rm[0x80+2*i]
		}
		c1c = rm[0xc7]
		a4[0] = rm[0x48]
		for i := 1; i < 8; i++ {
			a4[i] = rm[0x4a]
		}
	} else {
		for i := range gr {
			gr[i] = byte(0x80 + 2*i)
		}
		c1c = 0xc7
		a4 = font.Colors{0x48, 0x4a, 0x4a, 0x4a, 0x4a, 0x4a, 0x4a, 0x4a}
	}
	fillTop := gr[2]
	if !mainScreens {
		fillTop = gr[1]
	}
	g.triInterlacedFill(X, Y, X+0x145, Y+0x183, fillTop, fillTop, 0)
	g.interlacedFill(X+10, Y+0x1a, X+0x5f, Y+0x69, 0, c1c)
	if r.owner < 8 {
		g.interlacedFill(X+0xec, Y+0x1a, X+0x134, Y+0x69, 0, c1c)
	}
	b4 = font.Colors{gr[2], gr[6], gr[5], gr[6], gr[6], gr[6], gr[6], gr[6]}
	ac = font.Colors{gr[0], gr[4], gr[2], gr[4], gr[4], gr[4], gr[4], gr[4]}
	if mainScreens {
		if err := g.remapDrawShipStatus(sv, X+10, Y+0x18); err != nil {
			return err
		}
		if r.owner < 8 {
			if r.officer == -1 {
				err = g.remapDraw(X+0xf6, Y+0x27, sv.icon)
			} else {
				err = e.Draw(X+0xec, Y+0x16, sv.icon)
			}
			if err != nil {
				return err
			}
		}
	} else if err := g.drawViewShipStatusCombat(sv, X, Y); err != nil {
		return err
	}
	style := func(n int, c font.Colors) { t.SetStyleShadow(n, c, font.ShadowDown) }
	msg := g.kenText
	print := func(x, y int, s string) error { return t.Print(x, y, s) }
	var errs []error
	try := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}
	ken := func(n int) string {
		s, err := msg(n)
		try(err)
		return s
	}
	plus := func(n int16) string {
		if n < 1 {
			return itoa(int(n))
		}
		return "+" + itoa(int(n))
	}

	y := Y + 0x1a
	style(2, b4)
	try(t.PrintCentered(X+0xa7, y, ken(8)))
	g.interlacedFill(X+0x6a, y+0xc, X+0xdb, y+0x30, 0, c1c)
	style(1, ac)
	y += 0xf
	try(print(X+0x6f, y, ken(9)))
	try(t.PrintRight(X+0xd6, y, plus(r.attack)))
	y += 0xc
	try(print(X+0x6f, y, ken(10)))
	try(t.PrintRight(X+0xd6, y, plus(r.defense)))
	y += 0xc
	try(print(X+0x6f, y, ken(0xb)))
	try(t.PrintRight(X+0xd6, y, itoa(int(r.evasion))+"%"))
	y += 0x12
	if r.owner < 9 {
		style(2, b4)
		s, err := g.crewQualityString(r.crew)
		try(err)
		if i := strings.IndexByte(s, '('); i >= 0 {
			s = s[:i]
		}
		try(t.PrintCentered(X+0xa6, y, s))
	}
	y += 0xe
	if r.marines != 0xff {
		try(print(X+0x6f, y, ken(0xc)))
		try(t.PrintRight(X+0xd6, y, itoa(int(r.marines))))
	}
	yy := Y + 0x76
	x2c := X + 5
	x38 := X + 0xa5
	if r.captured == 1 {
		style(4, a4)
		t.SetOutlineColor(0, 1)
		s := ken(0xd)
		try(print(x2c, Y+3, s))
		try(t.PrintRight(X+0x143, Y+3, s))
	}
	style(4, b4)
	t.SetOutlineColor(0, 1)
	g.interlacedFill(0x10c, 0x31, 0x17f, 0x41, 0, c1c)
	if !mainScreens {
		try(t.PrintCentered(0x145, 0x32, save.CString(r.name[:])))
	}
	style(2, b4)
	if r.owner < 10 {
		try(print(x2c, yy, ken(0xe)))
	} else {
		try(print(x2c, yy, ken(0x57)))
	}
	if !m666 && r.drive != 0 {
		style(1, ac)
		try(t.PrintRight(x2c+0x9b, yy+3, "("+itoa(int(r.driveCur))+"/"+itoa(int(r.driveMax))+")"))
	}
	style(2, b4)
	if r.owner < 10 {
		s, err := g.techName(walksArmor + 1 + int(r.armor))
		try(err)
		try(print(x38, yy, s))
	} else {
		try(print(x38, yy, ken(0x58)))
	}
	bottom := yy + 0xe + 8
	if r.owner < 10 {
		bottom = yy + 0xe + 0x15
	}
	g.interlacedFill(x2c, yy+0xc, x2c+0x9b, bottom, 0, c1c)
	g.interlacedFill(x38, yy+0xc, x38+0x9b, bottom, 0, c1c)
	yy += 0xe
	style(1, ac)
	if r.owner < 10 {
		sp := g.exeI16(rowAddr(driveTable, driveStride, int(r.drive), driveSpeed))
		try(print(x2c+3, yy, "^ "+itoa(int(sp))+ken(0xf)))
	}
	if !m666 {
		try(print(x38+3, yy, fmt.Sprintf(ken(0x59), int(r.structure)-int(r.structDamage), int(r.structure))))
	} else {
		ba := g.bestArmor(int(r.owner))
		pct := int(g.armorField(ba, armorPct))
		try(print(x38+3, yy, "^ "+itoa(int(int16((pct+100)*100/100)))+" "+ken(0x10)))
	}
	if r.owner < 10 {
		yy += 0xc
	}
	try(print(x2c+3, yy, "^ "+itoa(int(r.speed))+ken(0x11)))
	if r.owner < 10 {
		if !m666 {
			hull := int(r.size)
			if r.drive == 0 {
				switch {
				case r.owner == 8:
					hull = 8
				case g.tc != nil && g.tc.colony >= 0:
					hull = int(g.typeOfSpaceStation(g.tc.colony))
				default:
					g.logOnce("Type_Of_Space_Station_ reads the combat colony; there is none outside combat")
					hull = -1
				}
			}
			base := g.exeI16(uint32(int32(hullTable) + int32(hull)*hullStride + hullArmor))
			n := int(base)*int(g.armorField(int(r.armor), armorPct))/100 + int(base)
			if testBit(r.specials[:], 0xe) {
				n *= 3
			}
			try(print(x38+3, yy, fmt.Sprintf(ken(0x5b), int(r.armorHits), int(int16(n)))))
		} else {
			try(print(x38+t.Width([]byte("^ "))+3, yy, ken(0x14)))
		}
	}
	if r.owner >= 10 {
		yy += 0xc
	}
	yy += 0x10
	if r.owner < 10 {
		style(2, b4)
		try(print(x2c, yy, ken(0x15)))
		style(1, ac)
		y3 := yy + 3
		try(t.PrintRight(x2c+0x9b, y3, "("+itoa(int(r.computerCur))+"/"+itoa(int(r.computerMax))+")"))
		style(2, b4)
		if r.shield < 7 {
			s, err := g.techName(walksShields + 1 + int(r.shield))
			try(err)
			try(print(x38, yy, s))
			if r.shield != 0 {
				style(1, ac)
				try(t.PrintRight(x38+0x9b, y3, "("+itoa(int(r.shieldCur))+"/"+itoa(int(r.shieldMax))+")"))
			}
		} else {
			s, err := g.buildingName(int(r.shield))
			try(err)
			try(print(x38, yy, s))
		}
		g.interlacedFill(x2c, yy+0xc, x2c+0x9b, yy+0x19, 0, c1c)
		g.interlacedFill(x38, yy+0xc, x38+0x9b, yy+0x19, 0, c1c)
		yy += 0xf
		style(1, ac)
		if r.computerCur < 1 {
			try(print(x2c+3, yy, ken(0x17)))
		} else {
			b := g.exeI16(rowAddr(computerTable, computerStride, int(r.computer), computerBonus))
			try(print(x2c+3, yy, fmt.Sprintf(ken(0x16), int(b))))
		}
		v := int(r.shieldBlock)
		if m666 {
			v = 0
			if r.owner != 8 {
				v = int(r.shieldBlock) + int(g.exeI16(rowAddr(shieldTable, shieldStride, g.bestShipShield(int(r.owner)), shieldStrength)))
			}
		}
		try(print(x38+3, yy, fmt.Sprintf(ken(0x18), v)))
		yy += 0x12
	}
	style(2, b4)
	try(print(x2c, yy, ken(0x19)))
	try(print(x2c+0x96, yy, ken(0x1a)))
	try(print(x2c+200, yy, ken(0x1b)))
	g.interlacedFill(x2c, yy+0xd, x2c+0x13b, yy+0x66, 0, c1c)
	yy += 0x10
	for k := range r.weapons {
		w := &r.weapons[k]
		if w.weapon != 0 {
			try(g.drawViewWeapon(w, x2c, yy, ac))
		}
		if k < 7 {
			yy += 0xb
		}
	}
	yy += 0x12
	style(2, b4)
	try(print(x2c, yy, ken(0x1e)))
	g.interlacedFill(x2c, yy+0xd, x2c+0x13b, yy+0x3b, 0, c1c)
	yy += 0x10
	n := 0
	for i := 0; i < 0x27; i++ {
		if !testBit(r.specials[:], i) {
			continue
		}
		c := ac
		if testBit(r.damaged[:], i) {
			c = a4
		}
		style(1, c)
		t.SetSpacing(1)
		if n&1 == 0 && n != 0 {
			yy += 0xb
		}
		x := x2c + 3
		if n&1 != 0 {
			x = x38 + 3
		}
		s, err := g.techName(walksSpecials + 1 + i)
		try(err)
		try(print(x, yy, s))
		n++
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return e.Draw(X-9, Y-9, frame)
}

// drawViewWeapon prints one weapon line of Combat_Draw_View_Ship_: count
// and name with the firing arc, the ammunition column and the mods.
func (g *Game) drawViewWeapon(w *viewWeapon, x2c, y int, ac font.Colors) error {
	t := g.e.Text()
	t.SetStyleShadow(1, ac, font.ShadowDown)
	t.SetSpacing(1)
	walk := walksWeaponsPl
	if w.count == 1 {
		walk = walksWeapons
	}
	name, err := g.techName(walk + 1 + int(w.weapon))
	if err != nil {
		return err
	}
	s := itoa(int(w.count)) + " " + name
	arc := " ("
	switch a := uint16(w.arc); {
	case a == 1:
		arc += g.must(g.kenText(3))
	case a == 2:
		arc += g.must(g.kenText(4))
	case a == 4:
		arc += g.must(g.kenText(6))
	case a == 8:
		arc += g.must(g.kenText(5))
	case a == 0xf:
		arc += "360"
	}
	arc += ")"
	if w.arc != 1 {
		s += arc
	}
	if err := t.Print(x2c+3, y, s); err != nil {
		return err
	}
	switch typ := g.exeU8(rowAddr(weaponTable, weaponStride, int(w.weapon), weaponType)); {
	case typ == 0:
		if err := t.Print(x2c+0x99, y, "-"); err != nil {
			return err
		}
	case typ == 2:
		shots := int16(g.exeU8(rowAddr(weaponTable, weaponStride, int(w.weapon), weaponMaxAmmo)))
		for k, bx := int16(0), x2c+0x99; k < shots; k, bx = k+1, bx+9 {
			if w.ammo == 0 {
				s, err := g.kenText(0x1c)
				if err != nil {
					return err
				}
				if err := t.Print(x2c+0x99, y, s); err != nil {
					return err
				}
				continue
			}
			c := [4]byte{0x49, 0x4b, 0x4b, 0x4b}
			if k >= w.ammo {
				c = [4]byte{0x51, 0x54, 0x54, 0x54}
			}
			g.box(bx, y, 7, 7, 1, c[0])
			g.box(bx+1, y+1, 5, 5, 1, c[1])
			g.box(bx+2, y+2, 3, 3, 1, c[2])
			g.box(bx+3, y+3, 1, 1, 1, c[3])
		}
	case typ < 5:
		if err := t.PrintRight(x2c+0xa1, y, itoa(int(w.ammo))); err != nil {
			return err
		}
	}
	mods := ""
	if w.mods == 0 {
		s, err := g.kenText(0x1d)
		if err != nil {
			return err
		}
		mods = s
	} else {
		for i, m := 0, int16(w.mods); i < 0xf; i, m = i+1, m>>1 {
			if m&1 == 0 {
				continue
			}
			if mods != "" {
				mods += ", "
			}
			s, err := g.techName(walksMods2 + 1 + i)
			if err != nil {
				return err
			}
			mods += s
		}
	}
	return t.Print(x2c+0xcb, y, mods)
}

// must drops an error the caller has no way to report; the strings read
// are fixed records of KENTEXT.LBX.
func (g *Game) must(s string, err error) string {
	if err != nil {
		g.Log("%v", err)
	}
	return s
}

// drawViewShipStatusCombat is Combat_Draw_View_Ship_'s combat part
// (0x309ce..0x30aa6): the scanned record's status picture, then the race
// icon, or the officer in grey.
func (g *Game) drawViewShipStatusCombat(sv *shipView, X, Y int) error {
	e := g.e
	r := &sv.rec
	if g.tc != nil && g.tc.scr != nil {
		if err := g.drawShipStatus(X+10, Y+0x18, g.tc.scr.scanned); err != nil {
			return err
		}
	}
	if r.owner > 8 {
		return nil
	}
	if r.officer == -1 {
		if r.owner >= 8 {
			return nil
		}
		return e.Draw(X+0xf6, Y+0x27, sv.icon)
	}
	var ramp [0x20]byte
	for k := range ramp {
		ramp[k] = byte(k)
	}
	b := gfx.NewBlankBitmap(1, 1)
	b.DrawPictureToBitmap(sv.icon)
	e.GrayScaleBitmap(b, 0x20, ramp[:], 0)
	return e.Draw(X+0xeb, Y+0x19, b)
}

// viewShipFromRecord reads a combat record as the view's record, as
// Display_Combat_View_Ship_'s copy to _global_combat_data does.
func viewShipFromRecord(t *tship) viewShip {
	var v viewShip
	copy(v.name[:], t[:16])
	v.id = t.i16(tsID)
	v.owner = t.u8(tsOwner)
	v.cloaked = t.u8(tsGone)
	v.size = t.u8(tsSize)
	v.shield = t.u8(tsShield)
	v.shieldBlock = t.i16(tsShieldMax)
	for k := range v.arcShields {
		v.arcShields[k] = t.i16(tsArc + 2*k)
	}
	v.drive = t.u8(tsDrive)
	v.computer = t.u8(tsComputer)
	v.armor = t.u8(tsArmor)
	v.attack = t.i16(tsOcv)
	v.defense = t.i16(tsDcv)
	v.evasion = t.i16(tsEvasion)
	v.speed = t.i16(tsSpeed)
	v.flag4b = t.u8(0x4b)
	copy(v.specials[:], t[tsSpecials:tsSpecials+5])
	for k := range v.weapons {
		w := &v.weapons[k]
		w.weapon = t.i16(tw(k, twWeapon))
		w.count = t.u8(tw(k, twCount))
		w.arc = t.u8(tw(k, twArc))
		w.mods = uint16(t.i16(tw(k, twMods)))
		w.ammo = t.i16(tw(k, twAmmo))
		w.ready = t.u8(tw(k, twOn))
		w.count2 = t.u8(tw(k, twLeft))
		w.used = t.u8(tw(k, twFired))
	}
	v.structure = t.i16(tsStruct)
	v.crew = t.u8(tsCrew)
	v.officer = t.i16(tsOfficer)
	v.marines = t.u8(tsMarines)
	v.captured = t.u8(tsBoarded)
	copy(v.damaged[:], t[tsDamaged:tsDamaged+5])
	copy(v.weaponOK[:], t[tsSpecialOn:tsSpecialOn+8])
	v.structDamage = t.i16(tsStructDmg)
	v.armorHits = t.i16(tsArmorHits)
	v.picture = t.i16(tsPicture)
	v.driveMax = t.i16(tsDriveMax)
	v.computerMax = t.i16(tsCompMax)
	v.shieldMax = t.i16(tsShieldHP)
	v.driveCur = t.i16(tsDriveCur)
	v.computerCur = t.i16(tsCompCur)
	v.shieldCur = t.i16(0x134)
	v.builder = t.u8(tsBuilder)
	return v
}
