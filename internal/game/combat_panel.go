package game

import (
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// The combat screen's panels (CMBTDRW1.C): the ship picture with its
// shields, damage bars, crew stars, systems, weapons and specials, the
// radar. Spec: docs/screens/combat-screen.md.

const cmbtBlinkTable = 0x2c54d // the radar's blink offsets, 4 words

// drawShipStatus is Draw_Ship_Status_ (0x2d1ed): the ship (or planet) in
// the 98x98 panel picture, ringed by its shields arc by arc.
func (g *Game) drawShipStatus(x, y int, i int16) error {
	e := g.e
	s := &g.state
	tc := g.tc
	sc := tc.scr
	v := e.V
	if x < 0x14 {
		v.Fill(0xf, 0x17b, 0x66, 0x1c5, 0)
		if err := g.drawCached(0xf, 0x17b, 0x2d); err != nil {
			return err
		}
	} else if x > 0x17c {
		v.Fill(0x191, 0x17b, 0x1e8, 0x1d3, 0)
		if err := g.drawCached(0x191, 0x17b, 0x2d); err != nil {
			return err
		}
	}
	if i == 0 {
		return g.drawPlanetStatus(x, y)
	}
	if err := g.drawShipToBitmap(i, 0, 0); err != nil {
		return err
	}
	cb := gfx.NewBlankBitmap(0x62, 0x62)
	sc.combatBitmap = cb
	t := g.tsp(i)
	f := int(t.u8(tsFacing))
	dx := 0x10
	if q := max((f-1)/4, 0); q == 1 || q == 2 {
		dx = 0xe
	}
	if err := cb.AddPicture(dx, 0xb, sc.drawShipBitmap); err != nil {
		return err
	}
	if t.u8(tsShield) != 0 && t.i16(tsShieldCur) >= 1 {
		level := min((int(t.i16(tsShieldMax))+0x13)/10, 10)
		// Rings outward, then per arc the colours its strength no longer reaches.
		var rings []byte
		var cuts []struct {
			below int16
			color byte
		}
		lim := int16(0)
		switch {
		case level == 0:
		case level < 3:
			rings = []byte{1, 2, 0x51, 0x52}
			lim, cuts = 0xb, []struct {
				below int16
				color byte
			}{{0xb, 0x52}, {1, 0x51}}
		case level < 5:
			rings = []byte{1, 2, 0x50, 0x52, 0x54, 0x56}
			lim, cuts = 0x1f, []struct {
				below int16
				color byte
			}{{0x1f, 0x56}, {0x16, 0x54}, {0xc, 0x52}, {1, 0x50}}
		case level < 7:
			rings = []byte{1, 2, 0x50, 0x51, 0x53, 0x54, 0x55, 0x56}
			lim, cuts = 0x33, []struct {
				below int16
				color byte
			}{{0x33, 0x56}, {0x2a, 0x55}, {0x20, 0x54}, {0x16, 0x53}, {0xc, 0x51}, {1, 0x50}}
		case level < 9:
			rings = []byte{0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57}
			lim, cuts = 0x47, []struct {
				below int16
				color byte
			}{{0x47, 0x57}, {0x3e, 0x56}, {0x34, 0x55}, {0x2a, 0x54}, {0x20, 0x53}, {0x16, 0x52}, {0xc, 0x51}, {1, 0x50}}
		default:
			rings = []byte{0x50, 0x51, 0x52, 0x53, 0x53, 0x54, 0x54, 0x55, 0x56, 0x57}
			lim, cuts = 0x5b, []struct {
				below int16
				color byte
			}{{0x5b, 0x57}, {0x52, 0x56}, {0x48, 0x55}, {0x3e, 0x54}, {0x2a, 0x53}, {0x16, 0x52}, {0xc, 0x51}, {1, 0x50}}
		}
		cb.Outline(cb, 1, false)
		cb.Outline(cb, 2, false)
		if level != 0 {
			for _, c := range rings {
				cb.Outline(cb, c, false)
			}
			for k := 0; k < 4; k++ {
				arc := t.i16(tsArc + 2*k)
				if arc >= lim {
					continue
				}
				sc.drawShipBitmap.CopyBitmapToBitmap(cb)
				m := sc.shieldMask[f%4][(4-f/4+k)%4]
				sc.drawShipBitmap.MaskOut(0, 0, m, true)
				for _, c := range cuts {
					if arc < c.below {
						if err := sc.drawShipBitmap.ReplaceColor(int(c.color), 1); err != nil {
							return err
						}
					}
				}
				if err := cb.AddPicture(0, 0, sc.drawShipBitmap); err != nil {
					return err
				}
			}
		}
		cb.MaskOut(0, 0, sc.shieldDiv[f%4], false)
		if err := g.drawShipToBitmap(i, 0, 0); err != nil {
			return err
		}
		if err := cb.AddPicture(dx, 0xb, sc.drawShipBitmap); err != nil {
			return err
		}
	}
	if err := cb.ReplaceColor(1, 0); err != nil {
		return err
	}
	if err := cb.ReplaceColor(2, 0); err != nil {
		return err
	}
	if err := e.Draw(x, y, cb); err != nil {
		return err
	}
	if t.i16(tsSpeed) < 1 {
		tx := e.Text()
		tx.SetStyleShadow(1, font.Colors{'H', 'J', 'J', 'J', 'J', 'J', 'J', 'J'}, font.ShadowDown)
		msg, err := g.kenText(0x47)
		if err != nil {
			return err
		}
		return tx.PrintCentered(x+0x2d, y+0x46, msg)
	}
	_ = s
	return nil
}

// drawPlanetStatus is Draw_Ship_Status_'s planet branch: the planet and its
// population, marines, tanks and defences.
func (g *Game) drawPlanetStatus(x, y int) error {
	e := g.e
	s := &g.state
	tc := g.tc
	if err := e.Draw(x+0x10, y+0xf, tc.scr.planet2); err != nil {
		return err
	}
	if g.tsp(0).u8(tsOwner) == 8 {
		return nil
	}
	t := e.Text()
	t.SetStyle(0, g.combatGreen())
	ken := func(n int) string { return g.must(g.kenText(n)) }
	brk := cStringAt(g.exe, cmbtColonyBreak)
	pop := 0x23
	if tc.colony != -1 {
		pop = int(s.Colonies[tc.colony].PopCount)
	}
	msg := ken(0) + itoa(pop) + brk
	if tc.colony != -1 {
		C := &s.Colonies[tc.colony]
		if C.Armor > 0 {
			msg += ken(1) + itoa(int(C.Armor)) + brk
		}
		if C.Marines > 0 {
			msg += ken(2) + itoa(int(C.Marines))
		}
	}
	if err := t.Print(x+2, y+0x48, msg); err != nil {
		return err
	}
	if tc.colony == -1 {
		return nil
	}
	b := &s.Colonies[tc.colony].Buildings
	yy := y + 4
	for _, d := range []struct {
		bld  int
		slot uint32
	}{{0x1a, 0x17ed2b}, {0x1b, 0x17ed3e}, {0x2f, 0x17eeba}, {0x2a, 0x17ee5b}} {
		if b[d.bld] == 0 {
			continue
		}
		name, err := g.buildingName(d.bld)
		if err != nil {
			return err
		}
		if err := t.Print(x+2, yy, name); err != nil {
			return err
		}
		yy += 7
		if d.bld == 0x1a {
			yy = y + 0xb
		}
	}
	return nil
}

// drawGreyDamageBars is Draw_Grey_Damage_Bars_ (0x2cbcc): the armour and
// structure bars under the ship picture, greyed by the damage taken.
func (g *Game) drawGreyDamageBars(x, y int, i int16) error {
	e := g.e
	v := e.V
	tc := g.tc
	if err := g.drawCached(x-1, y-1, 0x2b); err != nil {
		return err
	}
	if err := g.drawCached(x, y+0x4e, 0x2c); err != nil {
		return err
	}
	grey := func(xw1, xw2, gy int) error {
		v.SetWindow(xw1, 0, xw2, 0x1df)
		v.ClipOn()
		err := g.drawCached(x+1, gy, 0x57)
		v.ClipOff()
		v.ResetWindow()
		return err
	}
	halves := func(d int, gy int) error {
		v.SetWindow(0, 0, x+1+d, 0x1df)
		v.ClipOn()
		if err := g.drawCached(x+1, gy, 0x57); err != nil {
			return err
		}
		v.SetWindow(x+0x58-d, 0, 0x27f, 0x1df)
		err := g.drawCached(x+1, gy, 0x57)
		v.ClipOff()
		v.ResetWindow()
		return err
	}
	t := g.tsp(i)
	if i == 0 {
		if err := g.drawCached(x+1, y+0x4f, 0x57); err != nil {
			return err
		}
		if g.doesCombatPlanetHaveDefenses() == 0 {
			return g.drawCached(x+1, y+0x55, 0x57)
		}
		if err := g.drawCached(x+1, y+0x55, 0x58); err != nil {
			return err
		}
		pct := int32(g.armorField(g.bestArmor(int(t.u8(tsOwner))), armorHitsPct))
		full := uint32((pct+100)*100) / 100
		d := int(int16(uint32(int32(t.i16(tsStructDmg))*0x58)/full)) / 2
		return halves(d, y+0x55)
	}
	if t.u8(tsBuilder) >= 10 {
		if err := g.drawCached(x+1, y+0x4f, 0x59); err != nil {
			return err
		}
		if err := g.drawCached(x+1, y+0x55, 0x58); err != nil {
			return err
		}
		if t.i16(tsStructDmg) > 0 {
			d := int(int16(uint32(int32(t.i16(tsStructDmg))*0x58)/uint32(int32(t.i16(tsStruct))))) / 2
			if err := halves(d, y+0x55); err != nil {
				return err
			}
		}
		mx, cur := g.maxTotalInternalSpace(i), g.totalInternalSpace(i)
		if mx == cur {
			return nil
		}
		d := int(int16(uint32((int32(mx)-int32(cur))*0x58)/uint32(int32(mx)))) / 2
		return halves(d, y+0x4f)
	}
	size := int(t.u8(tsSize))
	if t.u8(tsDrive) == 0 {
		if tc.def == 8 {
			size = 8
		} else {
			size = int(g.typeOfSpaceStation(tc.colony))
		}
	}
	base := int32(g.exeI16(uint32(int32(hullTable) + int32(size)*hullStride + hullArmor)))
	full := uint32(base + int32(g.armorField(int(t.u8(tsArmor)), armorHitsPct))*base/100)
	if g.combatShipHasSpecial(i, 0xe) {
		full *= 3
	}
	if lost := int32(full) - int32(t.i16(tsArmorHits)); lost != 0 {
		d := int(int16(uint32(lost*0x58)/full)) / 2
		if err := halves(d, y+0x4f); err != nil {
			return err
		}
	}
	if dmg := t.i16(tsStructDmg); dmg > 0 {
		w := max(int(int16(uint32(int32(dmg)*0x2c)/uint32(int32(t.i16(tsStruct))))), 2)
		if err := grey(x+1, x+1+w, y+0x55); err != nil {
			return err
		}
	}
	mx, cur := g.maxTotalInternalSpace(i), g.totalInternalSpace(i)
	if mx == cur {
		return nil
	}
	w := max(int(int16(uint32((int32(mx)-int32(cur))*0x2c)/uint32(int32(mx)))), 2)
	return grey(x+0x58-w, x+0x58, y+0x55)
}

// drawCrewLevelStars is Draw_Crew_Level_Stars_ (0x2f445): a star per crew
// level, and the officer mark.
func (g *Game) drawCrewLevelStars(i int16, x, y int) error {
	e := g.e
	t := g.tsp(i)
	if i != 0 {
		for k := 0; k < int(t.u8(tsCrew)); k++ {
			if err := e.Draw(x+0xb*k, y, g.tc.scr.levelSymbol); err != nil {
				return err
			}
		}
	}
	tx := e.Text()
	tx.SetStyle(2, g.combatGreen())
	if t.i16(tsOfficer) > -1 {
		msg, err := g.kenText(0x56)
		if err != nil {
			return err
		}
		return tx.PrintRight(x+0x57, y+2, msg)
	}
	return nil
}

// drawInternalsDisplay is Draw_Internals_Display_ (0x3459e): the current
// ship's drive, shields, computer, structure, armour and moves.
func (g *Game) drawInternalsDisplay(x, y int) error {
	e := g.e
	tc := g.tc
	if tc.cur == 0 {
		return nil
	}
	t := e.Text()
	C := g.tsp(tc.cur)
	ken := func(n int) string { return g.must(g.kenText(n)) }
	t.SetStyleShadow(2, g.combatGreen(), font.ShadowDown)
	t.SetOutlineColor(0, 0)
	if err := t.PrintCentered(0x1bd, 0x16a, ken(0x24)); err != nil {
		return err
	}
	t.SetStyleShadow(1, g.combatGreen(), font.ShadowDown)
	t.SetOutlineColor(0, 0)
	r := x + 0x55
	line := func(dy, msg int, val string) error {
		if err := t.Print(x+4, y+dy, ken(msg)); err != nil {
			return err
		}
		if val == "" {
			return nil
		}
		return t.PrintRight(r, y+dy, val)
	}
	var err error
	if C.u8(tsDrive) == 0 {
		err = line(4, 0x25, "")
	} else {
		err = line(4, 0xe, itoa(int(C.i16(tsDriveCur))))
	}
	if err != nil {
		return err
	}
	if C.u8(tsShield) == 0 {
		err = line(0xe, 0x26, "")
	} else {
		err = line(0xe, 0x27, itoa(int(C.i16(tsShieldCur))))
	}
	if err != nil {
		return err
	}
	for _, l := range []struct {
		dy, msg int
		v       int
	}{
		{0x18, 0x15, int(C.i16(tsCompCur))},
		{0x22, 0x10, int(C.i16(tsStruct)) - int(C.i16(tsStructDmg))},
		{0x2c, 0x12, int(C.i16(tsArmorHits))},
		{0x40, 0x28, int(C.i16(tsSpeed))},
		{0x4b, 0x5a, int(C.i16(0x3d))},
	} {
		if err := line(l.dy, l.msg, itoa(l.v)); err != nil {
			return err
		}
	}
	return nil
}

// weaponName is a weapon's name from the tech names, plural or not.
func (g *Game) weaponName(w int, plural bool) (string, error) {
	walk := walksWeapons
	if plural {
		walk = walksWeaponsPl
	}
	return g.techName(walk + 1 + w)
}

// drawWeaponStatusDisplay is Draw_Weapon_Status_Display_ (0x2e2cd): the
// current ship's mounts, bright when they can fire at ship target (or
// missile m), dark when spent or off.
func (g *Game) drawWeaponStatusDisplay(target, m int16) error {
	e := g.e
	tc := g.tc
	cur := tc.cur
	inRange := font.Colors{'S', 'V', 'V', 'V', 'V', 'V', 'V', 'V'}
	normal := font.Colors{'P', 'S', 'S', 'S', 'S', 'S', 'S', 'S'}
	dark := font.Colors{0xdf, 0xdc, 0xdc, 0xdc, 0xdc, 0xdc, 0xdc, 0xdc}
	off := font.Colors{'H', 'J', 'J', 'J', 'J', 'J', 'J', 'J'}
	g.interlacedFill(0x72, 0x17a, 0x107, 0x1d2, 0, 199)
	C := g.tsp(cur)
	t := e.Text()
	ken := func(n int) string { return g.must(g.kenText(n)) }
	str := func(a uint32) string { return cStringAt(g.exe, a) }
	for k, y := 0, 0x17c; k < 8; k, y = k+1, y+0xb {
		w := C.i16(tw(k, twWeapon))
		if w == 0 {
			continue
		}
		arc := C.u8(tw(k, twArc))
		state := 0
		if !g.shipControlledByComputer(cur) {
			if g.weaponIsOn(cur, k) {
				var b int16
				ok := false
				if m < 0 {
					if target != -1 {
						b = g.relativeBearing(cur, target)
						ok = arc&getWeaponArc(b) != 0 && g.weaponInRange(k, cur, target, 0, 0)
					}
				} else {
					M := g.msl(m)
					b = g.relativeBearingXY(cur, M.i16(tmX), M.i16(tmY))
					ok = arc&getWeaponArc(b) != 0 && g.weaponInRange(k, cur, m, 1, 0)
				}
				if ok {
					tt := target
					if m >= 0 {
						tt = m
					}
					if g.targetIsLegalForWeapon(w, C.i16(tw(k, twMods)), tt, b2i(m >= 0)) {
						state = 1
					}
				}
			} else if C.u8(tw(k, twOn)) == 0 {
				state = 2
			} else {
				state = 3
			}
		}
		typ := g.wtype(w)
		ammo := C.i16(tw(k, twAmmo))
		spent := ((typ == 1 || typ == 3 || typ == 4) && ammo == 0) || (typ == 2 && ammo > 0) ||
			(C.u8(tw(k, twLeft)) == 0 && C.u8(tw(k, twFired)) != 1)
		pick := func() font.Colors {
			switch state {
			case 0:
				return normal
			case 1:
				return inRange
			case 2:
				return dark
			}
			return off
		}
		if spent {
			t.SetStyleShadow(1, dark, font.ShadowDown)
		} else {
			t.SetStyleShadow(1, pick(), font.ShadowDown)
		}
		t.SetOutlineColor(0, 0)
		t.SetSpacing(1)
		n := C.u8(tw(k, twLeft))
		if C.u8(tw(k, twLeft)) == 0 && C.u8(tw(k, twFired)) == 1 {
			n = C.u8(tw(k, twCount))
		}
		if err := t.PrintRight(0x7f, y, itoa(int(n))); err != nil {
			return err
		}
		label := ""
		mods := C.i16(tw(k, twMods))
		switch {
		case mods&2 != 0:
			label = g.must(g.techName(walksMods2+1+1)) + str(cmbtSpace)
		case mods&4 != 0:
			label = g.must(g.techName(walksMods2+1+2)) + str(cmbtSpace)
		}
		name, err := g.weaponName(int(w), C.u8(tw(k, twLeft)) != 1)
		if err != nil {
			return err
		}
		label += name
		if state <= 2 {
			t.SetStyleShadow(1, pick(), font.ShadowDown)
		}
		switch {
		case typ == 1 || typ == 3 || typ == 4 || w == 0x21:
			label += str(cmbtAmmoSep) + itoa(int(ammo))
		case typ == 2:
			if ammo > 0 {
				label += str(cmbtSpace) + itoa(int(ammo)) + ken(0x55)
			}
		case typ == 0:
			label += str(cmbtArcOpen)
			switch arc {
			case 1:
				label += ken(3)
			case 2:
				label += ken(4)
			case 4:
				label += ken(6)
			case 8:
				label += ken(5)
			case 0xf:
				label += str(cmbtArcAll)
			}
			label += str(cmbtArcClose)
		}
		t.SetOutlineColor(0, 0)
		t.SetSpacing(1)
		if err := t.Print(0x83, y, label); err != nil {
			return err
		}
	}
	return nil
}

// drawSpecialStatusDisplay is Draw_Special_Status_Display_ (0x2e98d).
func (g *Game) drawSpecialStatusDisplay() error {
	e := g.e
	tc := g.tc
	C := g.tsp(tc.cur)
	on := font.Colors{'P', 'S', 'S', 'S', 'S', 'S', 'S', 'S'}
	broken := font.Colors{'H', 'J', 'J', 'J', 'J', 'J', 'J', 'J'}
	offc := font.Colors{0xdf, 0xdc, 0xdc, 0xdc, 0xdc, 0xdc, 0xdc}
	g.interlacedFill(0x72, 0x17a, 0x107, 0x1d2, 0, 199)
	t := e.Text()
	n := 0
	y := 0x17c
	for b := 0; b < 0x27; b++ {
		if !testBit(C[tsSpecials:tsSpecials+5], b) {
			continue
		}
		c := broken
		if !testBit(C[tsDamaged:tsDamaged+5], b) {
			c = offc
			if C.u8(tsSpecialOn+n) == 1 {
				c = on
			}
		}
		t.SetStyleShadow(1, c, font.ShadowDown)
		t.SetOutlineColor(0, 0)
		n++
		t.SetSpacing(1)
		name, err := g.techName(walksSpecials + 1 + b)
		if err != nil {
			return err
		}
		if err := t.Print(0x7b, y, name); err != nil {
			return err
		}
		y += 0xb
	}
	if n == 0 {
		t.SetStyleShadow(2, g.combatGreen(), font.ShadowDown)
		t.SetOutlineColor(0, 0)
		return t.PrintCentered(0xbb, 0x1a2, g.must(g.kenText(7)))
	}
	return nil
}

// Draw_Blip2_..5_ (0x2eb0f..): radar dots by hull size in colours base+c.
func (g *Game) drawBlip(size, x, y int, base, b int) {
	v := g.e.V
	c := func(k int) byte { return byte(base + k + b) }
	dot := func(dx, dy int, k int) { v.Dot(x+dx, y+dy, c(k)) }
	switch size {
	case 2:
		dot(0, 0, 9)
		dot(1, 0, 6)
		dot(0, 1, 6)
		dot(1, 1, 4)
	case 3:
		rows := [3][3]int{{9, 6, 4}, {6, 6, 4}, {4, 4, 1}}
		for yy, r := range rows {
			for xx, k := range r {
				dot(xx, yy, k)
			}
		}
	case 4:
		rows := [4][4]int{{1, 4, 4, 1}, {4, 9, 6, 4}, {4, 6, 6, 4}, {1, 4, 4, 1}}
		for yy, r := range rows {
			for xx, k := range r {
				dot(xx, yy, k)
			}
		}
	case 5:
		rows := [5][5]int{{2, 4, 4, 4, 1}, {4, 9, 9, 6, 4}, {4, 9, 6, 6, 4}, {4, 6, 6, 6, 4}, {1, 4, 4, 4, 1}}
		for yy, r := range rows {
			for xx, k := range r {
				dot(xx, yy, k)
			}
		}
	}
}

// blipFor is the blip routine Draw_Reduced_Combat_Map_ picks by hull size.
func blipFor(sz uint8) int {
	switch {
	case sz == 0:
		return 2
	case sz < 3:
		return 3
	case sz < 4:
		return 4
	case sz < 6:
		return 5
	}
	return 0
}

// drawReducedCombatMap is Draw_Reduced_Combat_Map_ (0x2f172): the radar's
// planet, ships and missiles.
func (g *Game) drawReducedCombatMap() {
	s := &g.state
	tc := g.tc
	v := g.e.V
	at := func(c uint8, base int) int { return int(c)*3/2 + base }
	if tc.colony != -1 {
		P := g.tsp(0)
		y, x := at(P.u8(tsY), 0x16c), at(P.u8(tsX), 500)
		switch sz := int(s.Planets[s.Colonies[tc.colony].Planet].Size); {
		case sz == 0:
			g.drawBlip(4, x, y, 0x60, 0)
		case sz < 5:
			if p := tc.scr.mapPlanet; p != nil {
				_ = g.e.Draw(x-(7-sz), y-(7-sz), p)
			}
		}
	}
	for i := int16(1); i < tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsGone) != 0 {
			continue
		}
		base := 0x60
		if int16(t.u8(tsOwner)) == tc.att {
			base = 0x20
		}
		if k := blipFor(t.u8(tsSize)); k != 0 {
			g.drawBlip(k, at(t.u8(tsX), 500), at(t.u8(tsY), 0x16c), base, 0)
		}
	}
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		if M.i16(tmWeapon) > 0 {
			v.Dot(int(int32(M.i16(tmX))/0x14*3/2)+500, int(int32(M.i16(tmY))/0x14*3/2)+0x16c, 0x1c)
		}
	}
}

// drawRadarCurrentShip is the blinking blip of the current ship that
// Draw_Main_Combat_Screen_ puts on the radar.
func (g *Game) drawRadarCurrentShip() error {
	s := &g.state
	tc := g.tc
	sc := tc.scr
	C := g.tsp(tc.cur)
	y := int(C.u8(tsY))*3/2 + 0x16c
	x := int(C.u8(tsX))*3/2 + 500
	b := int(g.exeI16(cmbtBlinkTable + 2*uint32((int(sc.shipFrame)/2)%4)))
	if tc.cur != 0 {
		base := 0x60
		if int16(C.u8(tsOwner)) == tc.att {
			base = 0x20
		}
		if k := blipFor(C.u8(tsSize)); k != 0 {
			g.drawBlip(k, x, y, base, b)
		}
		return nil
	}
	sz := int(s.Planets[s.Colonies[tc.colony].Planet].Size)
	switch {
	case sz == 0:
		g.drawBlip(4, x, y, 0x60, b)
	case sz <= 4:
		var bm gfx.Picture
		bm.DrawPictureToBitmap(sc.mapPlanet)
		for c := 0x60; c < 0x70; c++ {
			if err := bm.ReplaceColor(c, max(c-b, 0x60)); err != nil {
				return err
			}
		}
		return g.e.Draw(x-(7-sz), y-(7-sz), &bm)
	}
	return nil
}

// drawReducedMapBox is Draw_Reduced_Map_Box_ (0x2ef51): the corners of the
// view on the radar.
func (g *Game) drawReducedMapBox() {
	tc := g.tc
	v := g.e.V
	x := int(tc.curX) * 3 / 2
	y := int(tc.curY) * 3 / 2
	v.Line(x+499, y+0x16b, x+0x1fa, y+0x16b, 0x8a)
	v.Line(x+499, y+0x16b, x+499, y+0x172, 0x8a)
	v.Line(x+499, y+0x188, x+0x1fa, y+0x188, 0x8a)
	v.Line(x+499, y+0x188, x+499, y+0x181, 0x8a)
	v.Line(x+0x225, y+0x16b, x+0x21e, y+0x16b, 0x8a)
	v.Line(x+0x225, y+0x16b, x+0x225, y+0x172, 0x8a)
	v.Line(x+0x225, y+0x188, x+0x21e, y+0x188, 0x8a)
	v.Line(x+0x225, y+0x188, x+0x225, y+0x181, 0x8a)
}

// drawShieldArcs is Draw_Shield_Arcs_ (0x32243): four lines out of the
// current ship splitting its shield arcs, each with its letter.
func (g *Game) drawShieldArcs() error {
	e := g.e
	tc := g.tc
	cur := tc.cur
	if cur == 0 {
		return nil
	}
	labels := cmbtArcLabelsEn
	if g.settings[0xd2] == 4 {
		labels = cmbtArcLabels
	}
	C := g.tsp(cur)
	radius := [6]int32{0xf, 0x16, 0x19, 0x1e, 0x21, 0x21}
	R := radius[min(int(C.u8(tsSize)), 5)]
	ox, oy := g.shipCenterOffsets(cur)
	cx := int32(ox) + int32(C.u8(tsX))*0x14
	cy := int32(oy) + int32(C.u8(tsY))*0x14
	var tx, ty, ldx, ldy [4]int32
	f := int32(C.u8(tsFacing))
	if f <= 0xf {
		q := f >> 2
		set := func(k int32, x, y int32) { tx[(q+k)%4], ty[(q+k)%4] = x, y }
		switch f % 4 {
		case 0:
			set(0, R, 0)
			set(1, 0, R)
			set(2, -R, 0)
			set(3, 0, -R)
			ldx, ldy = [4]int32{2000, -2000, -2000, 2000}, [4]int32{2000, 2000, -2000, -2000}
		case 1:
			a, b := R*0x17/0x19, R*0x13/0x32
			set(0, a, -b)
			set(1, b, a)
			set(2, -a, b)
			set(3, -b, -a)
			ldx, ldy = [4]int32{-0xef3, -0x2417, 0xef3, 0x2417}, [4]int32{0x2417, -0xef3, -0x2417, 0xef3}
		case 2:
			set(0, R, -R)
			set(1, R, R)
			set(2, -R, R)
			set(3, -R, -R)
			ldx, ldy = [4]int32{0, -2000, 0, 2000}, [4]int32{2000, 0, -2000, 0}
		case 3:
			a, b := R*0x13/0x32, R*0x17/0x19
			set(0, a, -b)
			set(1, b, a)
			set(2, -a, b)
			set(3, -b, -a)
			ldx, ldy = [4]int32{0xef3, -0x2417, -0xef3, 0x2417}, [4]int32{0x2417, 0xef3, -0x2417, -0xef3}
		}
	}
	e.Text().SetStyle(0, g.combatGreen())
	for k := 0; k < 4; k++ {
		x1 := int(int16(cx - int32(tc.curX)*0x14))
		y1 := int(int16(cy - int32(tc.curY)*0x14))
		e.V.Line(x1, y1, x1+int(int16(ldx[k])), y1+int(int16(ldy[k])), 0x54)
		lab := cStringAt(g.exe, uint32(labels+2*k))
		if err := e.Text().Print(int(int16(tx[k]-2+cx-int32(tc.curX)*0x14)), int(int16(ty[k]-3+cy-int32(tc.curY)*0x14)), lab); err != nil {
			return err
		}
	}
	return nil
}

// drawOptionPanelLights is Draw_Option_Panel_Lights_ (0x2f382).
func (g *Game) drawOptionPanelLights() error {
	tc := g.tc
	sc := tc.scr
	for k, v := range []int16{tc.mslImpact, tc.speedx2, sc.legalBoxes, tc.shieldsFlag, sc.gridFlag} {
		g.setAnimationFrame(sc.option, int(v))
		if err := g.e.Draw(0x10b, 0x182+0x10*k, sc.option); err != nil {
			return err
		}
	}
	return nil
}

// drawFakeCombatButtons is Draw_Fake_Combat_Buttons_ (0x34921): the panel
// buttons as pictures, for when there are no fields.
func (g *Game) drawFakeCombatButtons() error {
	e := g.e
	tc := g.tc
	sc := tc.scr
	draw := func(x, y int, p *gfx.Picture, f int) error {
		g.setAnimationFrame(p, f)
		return e.Draw(x, y, p)
	}
	if err := draw(0x72, 0x169, sc.weaponsBtn, int(1-sc.statusDisplay)); err != nil {
		return err
	}
	if err := draw(0xbb, 0x169, sc.specialsBtn, int(sc.statusDisplay)); err != nil {
		return err
	}
	if tc.orderInfo == 1 {
		for _, b := range []struct {
			x, y int
			p    *gfx.Picture
			f    int16
		}{{0x111, 0x16c, sc.autoBtn, tc.autoCombat}, {0x151, 0x16c, sc.scanBtn, tc.skan}, {0x111, 0x188, sc.boardBtn, 0},
			{0x151, 0x188, sc.fleeBtn, 0}, {0x111, 0x1a4, sc.waitBtn, 0}, {0x151, 0x1a4, sc.doneBtn, 0}, {0x12a, 0x1c0, sc.infoBtn, 0}} {
			if err := draw(b.x, b.y, b.p, int(b.f)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := g.drawOptionPanelLights(); err != nil {
		return err
	}
	if err := draw(0x12a, 0x12d, sc.infoBtn, 0); err != nil {
		return err
	}
	return draw(0x11c, 0x15c, sc.selfDestruct, 0)
}
