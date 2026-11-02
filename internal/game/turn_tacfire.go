package game

import "github.com/openorion2/openorion2/internal/save"

// Weapons fire of the tactical engine off screen (COMBAT.C, COMBAT1.C):
// the parts Tactical_Bombardment_ reaches. Spec: docs/turn/combat.md.

func (g *Game) msl(i int16) *tmissile {
	tc := g.tc
	if i >= 0 && int(i) < len(tc.mis) {
		return &tc.mis[i]
	}
	// As tsp: an index from overlapped bytes reads memory the port lacks.
	g.logOnce("a missile outside the engine's memory (overlapping records and missiles)")
	if tc.farMis == nil {
		tc.farMis = map[int16]*tmissile{}
	}
	if tc.farMis[i] == nil {
		tc.farMis[i] = &tmissile{}
	}
	return tc.farMis[i]
}

// shipCenterOffsets is Ship_Center_Offsets_ (0x49ed1): the record's centre
// within its square, by hull or (record 0) planet size.
func (g *Game) shipCenterOffsets(i int16) (x, y int16) {
	if i == 0 {
		if g.tc.colony == -1 {
			return 0x3c, 0x3c
		}
		switch g.state.Planets[g.state.Colonies[g.tc.colony].Planet].Size {
		case 0:
			return 0x1e, 0x1e
		case 1:
			return 0x28, 0x28
		case 2:
			return 0x3c, 0x3c
		case 3:
			return 0x46, 0x46
		case 4:
			return 0x50, 0x50
		}
		return 0, 0
	}
	switch s := g.tsp(i).u8(tsSize); {
	case s == 0:
		return 0xa, 0xa
	case s <= 2:
		return 0x14, 0x14
	case s <= 5:
		return 0x1e, 0x1e
	}
	return 0, 0
}

// rangeToShip is Range_To_Ship_ (0x2885a): squares between the nearest
// cells of two records (big hulls cover two squares).
func (g *Game) rangeToShip(a, b int16) int16 {
	ax, _ := g.shipCenterOffsets(a)
	ta := g.tsp(a)
	x1, y1 := int16(ta.u8(tsX)), int16(ta.u8(tsY))
	var aBig, aTwo, bBig, bTwo int32
	if ax > 0xa {
		aBig = 1
	}
	if ax > 0x14 {
		aTwo = 1
		x1++
		y1++
	}
	bx, _ := g.shipCenterOffsets(b)
	tb := g.tsp(b)
	x2, y2 := int16(tb.u8(tsX)), int16(tb.u8(tsY))
	if bx > 0xa {
		bBig = 1
	}
	if bx > 0x14 {
		bTwo = 1
		x2++
		y2++
	}
	clamp := func(v, lim, sub, add int32, over bool) int32 {
		if over {
			if v-sub < lim {
				return lim
			}
			return v - sub
		}
		if v+add > lim {
			return lim
		}
		return v + add
	}
	cx := int16(clamp(int32(x1), int32(x2), aTwo, aBig, x1 > x2))
	cy := int16(clamp(int32(y1), int32(y2), aTwo, aBig, y1 > y2))
	dx := int16(clamp(int32(x2), int32(cx), bTwo, bBig, x2 > cx))
	dy := int16(clamp(int32(y2), int32(cy), bTwo, bBig, y2 > cy))
	return int16((int32(rangeXY(cx, cy, dx, dy)) + 2) / 3)
}

// rangeToXY is Range_To_XY_ (0x286e8): from a record to a point in pixels.
func (g *Game) rangeToXY(a int16, px, py int16) int16 {
	ox, oy := g.shipCenterOffsets(a)
	ox, oy = ox/0x14, oy/0x14
	t := g.tsp(a)
	cx, cy := int32(t.u8(tsX)), int32(t.u8(tsY))
	tx, ty := int32(px)/0x14, int32(py)/0x14
	near := func(v, lim, off int32) int32 {
		if v > lim {
			if v-off < lim {
				return lim
			}
			return v - off
		}
		if v+off > lim {
			return lim
		}
		return v + off
	}
	ax := near(cx, tx, int32(ox))
	ay := near(cy, ty, int32(oy))
	bx := near(tx, ax, 0)
	by := near(ty, ay, 0)
	return int16((int32(rangeXY(int16(ax), int16(ay), int16(bx), int16(by))) + 2) / 3)
}

// rangeToMissile is Range_To_Missile_ (0x28836).
func (g *Game) rangeToMissile(a, m int16) int16 {
	M := g.msl(m)
	return g.rangeToXY(a, M.i16(tmX), M.i16(tmY))
}

// relativeBearingXY is Relative_Bearing_XY_ (0x329a4): the bearing of a
// point from the record's centre, relative to its facing.
func (g *Game) relativeBearingXY(i, px, py int16) int16 {
	ox, oy := g.shipCenterOffsets(i)
	t := g.tsp(i)
	ox += int16(t.u8(tsX)) * 0x14
	oy += int16(t.u8(tsY)) * 0x14
	a := int16(g.getAngle(int(int16(px-ox)), int(int16(oy-py))))
	f := int32(t.u8(tsFacing))
	r := int16(int32(a) - (f*0x16 + f/2))
	if r < 0 {
		r += 0x168
	}
	return r
}

// relativeBearing is Relative_Bearing_ (0x32a20): of record b from a.
func (g *Game) relativeBearing(a, b int16) int16 {
	ax, ay := g.shipCenterOffsets(a)
	ta := g.tsp(a)
	ax += int16(ta.u8(tsX)) * 0x14
	ay += int16(ta.u8(tsY)) * 0x14
	bx, by := g.shipCenterOffsets(b)
	tb := g.tsp(b)
	bx += int16(tb.u8(tsX)) * 0x14
	by += int16(tb.u8(tsY)) * 0x14
	angle := int16(g.getAngle(int(int16(bx-ax)), int(int16(ay-by))))
	f := int32(ta.u8(tsFacing))
	r := int16(int32(angle) - (f*0x16 + f/2))
	if r < 0 {
		r += 0x168
	}
	return r
}

// facingShield is Facing_Shield_ (0x39de0): the shield arc a bearing hits.
func facingShield(b int16) int16 {
	switch {
	case b > 0x13b || b <= 0x2d:
		return 0
	case b <= 0x87:
		return 3
	case b <= 0xe1:
		return 2
	}
	return 1
}

// findWeakestShield is Find_Weakest_Shield_ (0x3db8f).
func (g *Game) findWeakestShield(i int16) int16 {
	t := g.tsp(i)
	best, v := int16(0), int16(0xc8)
	for k := int16(0); k < 4; k++ {
		if v > t.i16(tsArc+2*int(k)) {
			best, v = k, t.i16(tsArc+2*int(k))
		}
	}
	return best
}

// isCombatShipDead is Is_Combat_Ship_Dead_ (0x3897a): 1 destroyed, 3
// drifting without drive, 4 with a special 0x18 aboard; the planet never.
func (g *Game) isCombatShipDead(i int16) int16 {
	if i == 0 {
		return 0
	}
	t := g.tsp(i)
	r := int16(0)
	switch {
	case t.i16(tsStructDmg) >= t.i16(tsStruct):
		r = 1
	case t.i16(tsDriveCur) == 0 && t.u8(tsOwner) <= 9 && t.u8(tsDrive) > 0:
		r = 3
	}
	if r != 0 && g.combatShipHasSpecial(i, 0x18) {
		r = 4
	}
	return r
}

func (g *Game) shipInStasis(i int16) bool { return g.tsp(i).u8(tsStasis) != 0xff }

// doesCombatPlanetHaveDefenses is Does_Combat_Planet_Have_Defenses_
// (0x3a142): missile base 1, batteries 2, converter 4, fighters 8.
func (g *Game) doesCombatPlanetHaveDefenses() int16 {
	if g.tc.colony == -1 {
		return 0
	}
	C := &g.state.Colonies[g.tc.colony]
	return int16(C.Buildings[0x1a]) | int16(C.Buildings[0x1b])<<1 | int16(C.Buildings[0x2a])<<2 | int16(C.Buildings[0x2f])<<3
}

// weaponInRange is Weapon_In_Range_ (0x39f1d) for mount k of record a.
func (g *Game) weaponInRange(k int, a, t, mode, rng int16) bool {
	A := g.tsp(a)
	mods := A.i16(tw(k, twMods))
	w := A.i16(tw(k, twWeapon))
	r := rng
	if rng == 0 {
		if mode == 1 {
			r = g.rangeToMissile(a, t)
		} else {
			r = g.rangeToShip(a, t)
		}
	}
	if mods&4 != 0 {
		r += r
	}
	if mods&2 != 0 {
		r /= 2
	}
	switch g.wpnU8(int(w), weaponType) {
	case 0:
		return r <= 8
	case 1, 2, 4:
		switch {
		case w == 0x13 && r > 8, w == 0x28 && r > 5:
			return false
		}
		return true
	case 3:
		ok := false
		if g.combatShipHasSpecial(a, 0x23) {
			ok = r <= 4
		} else {
			ok = r <= 1
		}
		return ok && t == 0
	case 5:
		switch w {
		case 0x27:
			return r <= 4
		case 0x20, 0x24, 0xd, 0x2c:
			return r <= 1
		case 0x21, 0x22, 0x23, 0x25, 0x2d:
			return r <= 5
		case 0x26:
			return true
		}
	}
	return false
}

// weaponCanFireAtMissiles is Weapon_Can_Fire_At_Missiles_ (0x3a7aa).
func (g *Game) weaponCanFireAtMissiles(w, mods int16) bool {
	if w == 0 {
		return false
	}
	switch g.wpnU8(int(w), weaponType) {
	case 0:
		return mods&2 == 0
	case 4:
		return w == 0x1f
	case 5:
		return w == 0x24 || w == 0xd || w == 0x21
	}
	return false
}

// targetIsLegalForWeapon is Target_Is_Legal_For_Weapon_ (0x389e8).
func (g *Game) targetIsLegalForWeapon(w, mods, t, mode int16) bool {
	tc := g.tc
	if mode == 1 {
		ok := g.weaponCanFireAtMissiles(w, mods)
		mt := g.wpnU8(int(g.msl(t).i16(tmWeapon)), weaponType)
		if mt == 4 && w == 0x21 {
			ok = false
		}
		if mt != 2 {
			return ok
		}
		return false
	}
	ok := true
	T := g.tsp(t)
	switch {
	case g.shipInStasis(t):
		ok = false
	case g.wpnI16(int(w), 0x12)&8 != 0 && T.u8(tsBuilder) >= 8 && T.u8(tsBuilder) != 9:
		ok = false
	case g.wpnU8(int(w), weaponType) == 3 && t != 0:
		ok = false
	case w == 0x21:
		ok = false
	case t == 0:
		if w == 0x1c {
			ok = false
		}
		if g.wpnU8(int(w), weaponType) == 5 {
			switch {
			case w >= 0x22 && w <= 0x25, w == 0x27, w == 0x2c || w == 0x2d, w == 0xd, w == 0x20:
				ok = false
			}
		}
	case w == 0x1c && T.u8(tsOwner) > 8:
		ok = false
	}
	if T.u8(0x136) > 0 && w == 0x25 {
		ok = false
	}
	_ = tc
	return ok
}

// fireShip is Fire_Ship_ (0x38b5e): record a fires every mount that can
// reach t (a record, or with mode 1 a missile) at (px, py). Off screen
// (Tactical_Bombardment_) every arc counts.
func (g *Game) fireShip(a, t, px, py, mode int16) bool {
	A := g.tsp(a)
	return g.fireShipAt(a, int16(A.u8(tsX))*0x14, int16(A.u8(tsY))*0x14, t, px, py, mode)
}

// fireShipAt is Fire_Ship_ with the attacker's pixel position given, as
// Final_Fire_ passes a moving ship's (and Tactical_Bombardment_ zeros).
func (g *Game) fireShipAt(a, ax, ay, t, px, py, mode int16) bool {
	tc := g.tc
	A := g.tsp(a)
	done := false
	fired := false
	var arc uint8
	if mode == 0 {
		arc = getWeaponArc(g.relativeBearing(a, t))
	} else {
		M := g.msl(t)
		arc = getWeaponArc(g.relativeBearingXY(a, M.i16(tmX), M.i16(tmY)))
	}
	if tc.onScreen == 0 {
		arc = 0xf
	}
	g.preWeaponSpecials(a, t, mode)
	sfx := tc.onScreen == 1 && tc.superFast == 0
	var recs []beamRec
	ok := false
	if mode == 1 || !g.shipInStasis(t) {
		if mode != 0 {
			ok = g.msl(t).i16(tmCount) > 0
		} else {
			ok = g.isCombatShipDead(t) == 0
		}
	}
	for k := 0; ok && k < 8 && !done; k++ {
		if g.isCombatShipDead(a) != 0 {
			break
		}
		w := A.i16(tw(k, twWeapon))
		total := int16(0)
		if w == 0 {
			continue
		}
		typ := g.wpnU8(int(w), weaponType)
		fired = false
		if !g.weaponIsOn(a, k) || A.u8(tw(k, twArc))&arc == 0 {
			continue
		}
		ammo := A.i16(tw(k, twAmmo))
		if !((typ != 2 && ammo != 0) || (typ == 2 && ammo == 0)) {
			continue
		}
		if !g.weaponInRange(k, a, t, mode, 0) || !g.targetIsLegalForWeapon(w, A.i16(tw(k, twMods)), t, mode) {
			continue
		}
		if A.u8(tw(k, twLeft)) == 0 && A.u8(tw(k, twFired)) == 1 {
			A.set8(tw(k, twFired), 0)
			A.set8(tw(k, twLeft), A.u8(tw(k, twCount)))
		}
		switch A.u8(tsCloak) {
		case 1:
			if tc.onScreen == 1 && tc.superFast == 0 {
				g.drawCloak(a, 2)
			}
			A.set8(tsCloak, 2)
		case 4:
			if tc.onScreen == 1 && tc.superFast == 0 {
				g.drawCloak(a, 5)
			}
			A.set8(tsCloak, 6)
		}
		if sfx && tc.scr != nil && typ == 0 {
			if mode != 0 {
				if !g.onCombatScreen(px/0x14, py/0x14) {
					g.must2(g.snapCenterCombatScreen(px/0x14, py/0x14))
				}
			} else if !g.shipCompletelyOnScreen(t) {
				T := g.tsp(t)
				g.must2(g.snapCenterCombatScreen(int16(T.u8(tsX)), int16(T.u8(tsY))))
			}
		}
		if sfx && len(recs) > 0 && typ != 0 {
			g.beamSFX(a, ax, ay, t, px, py, recs, mode)
			recs = recs[:0]
		}
		var rec beamRec
		more := true
		for !done && A.u8(tw(k, twLeft)) > 0 && more && g.isCombatShipDead(a) == 0 {
			switch typ {
			case 1, 2, 4:
				g.fireMissile(a, t, k, mode)
				if typ == 2 {
					A.set16(tw(k, twAmmo), int16(g.wpnU8(int(A.i16(tw(k, twWeapon))), weaponMaxAmmo)))
				} else {
					A.add16(tw(k, twAmmo), -1)
				}
				A.set8(tw(k, twLeft), 0)
			case 0:
				var dmg, abs, refl int16
				r := g.fireBeamWeapon(a, t, w, A.i16(tw(k, twMods)), mode, &dmg, &abs, &refl)
				if refl > 0 {
					var ab int16
					g.applyShipDamage(a, &refl, &ab, facingShield(g.relativeBearing(a, t)), 0, g.wpnI16(int(w), 0x12), 0, 0, 0)
					rec.reflDmg += refl
					rec.reflAbs += ab
				}
				rec.result |= r & 1
				if r&2 != 0 {
					rec.pierced = 1
				}
				rec.shots++
				rec.dmg += dmg
				rec.abs += abs
				rec.mods = A.i16(tw(k, twMods))
				if g.wpnI16(int(w), 0x12)&0x200 != 0 {
					rec.mods |= 0x100
				}
				total += dmg
				A.set8(tw(k, twLeft), A.u8(tw(k, twLeft))-1)
			case 3:
				total = g.fireBomb(a, k)
				A.set8(tw(k, twLeft), 0)
				A.add16(tw(k, twAmmo), -1)
			case 5:
				g.fireSpecialWeapon(a, t, k, mode)
				if A.i16(tw(k, twWeapon)) == 0x20 {
					more = false
				}
			}
			fired = true
			tc.realAttack = 1
			if A.u8(tw(k, twFired)) == 2 {
				A.set8(tw(k, twFired), 0)
			}
			if mode != 0 {
				M := g.msl(t)
				if int32(M.i16(tmCount))-int32(total) <= 0 || M.i16(tmWeapon) == 0 {
					done = true
				}
			} else {
				done = g.isCombatShipDead(t) != 0
			}
		}
		switch A.u8(tsCloak) {
		case 3:
			A.set8(tsCloak, 2)
		case 7:
			A.set8(tsCloak, 6)
		}
		if fired && sfx && typ == 3 && tc.scr != nil {
			g.drawBombs(a, t, A.i16(tw(k, twWeapon)), total)
		}
		if fired && sfx && typ == 0 {
			rec.weapon = w
			recs = append(recs, rec)
			if rec.reflDmg > 0 || rec.reflAbs > 0 {
				recs = append(recs, beamRec{weapon: w, reflected: true, dmg: rec.reflDmg, abs: rec.reflAbs,
					result: b2i(rec.reflAbs > 0), pierced: b2i(rec.reflDmg > 0), shots: rec.shots, mods: rec.mods})
				if tc.scr != nil {
					g.e.V.SetPageBack()
					g.must2(g.drawShipStatus(0xe, 0x178, tc.cur))
					g.must2(g.drawGreyDamageBars(0xe, 0x178, tc.cur))
					g.must2(g.drawCrewLevelStars(tc.cur, 0xf, 0x17b))
					g.e.V.SetPageOff()
				}
			}
		}
	}
	if len(recs) > 0 {
		g.beamSFX(a, ax, ay, t, px, py, recs, mode)
	}
	if dead := g.isCombatShipDead(a); dead > 0 {
		g.destroyShip(a, int16(A.u8(tsX))*0x14, int16(A.u8(tsY))*0x14, dead)
	}
	if done && mode == 0 && g.tsp(t).u8(tsGone) == 0 {
		g.destroyShip(t, px, py, g.isCombatShipDead(t))
	}
	if tc.scr != nil {
		tc.scr.lastShipStatus = -1
	}
	return fired
}

// beamRec is one 0xf-byte beam effect record Fire_Ship_ collects for
// Beam_SFX_ (one per mount, a reflected beam gets one of its own).
type beamRec struct {
	weapon, shots, result, pierced, dmg, abs, mods int16
	reflected                                      bool
	reflDmg, reflAbs                               int16 // Fire_Ship_'s running reflection totals
}

// beamSFX is Fire_Ship_'s call of Beam_SFX_ (0xa8197) with the records
// collected, at pixels relative to the view.
func (g *Game) beamSFX(a, ax, ay, t, px, py int16, recs []beamRec, mode int16) {
	if len(recs) == 0 || recs[0].weapon == 0 {
		return
	}
	tc := g.tc
	var rr beamRecs
	for i, r := range recs {
		if i >= len(rr) {
			break
		}
		rr[i] = beamRec16{weapon: r.weapon, shots: r.shots, result: r.result, pierced: r.pierced, dmg: r.dmg, abs: r.abs, mods: r.mods, refl: byte(b2i(r.reflected))}
	}
	vx, vy := tc.curX*0x14, tc.curY*0x14
	g.beamSFXPlay(a, ax-vx, ay-vy, t, px-vx, py-vy, &rr, mode)
}

// preWeaponSpecials is Pre_Weapon_Specials_ (0x3b332): a charged energy
// absorber discharges at a ship within 7 squares first.
func (g *Game) preWeaponSpecials(a, t, mode int16) {
	var rng int16
	if mode == 0 {
		rng = g.rangeToShip(a, t)
	} else {
		rng = g.rangeToMissile(a, t)
	}
	k := 0
	A := g.tsp(a)
	for sp := 0; sp < 0x27; sp++ {
		if sp == 10 && mode == 0 && rng < 8 && A.i16(tsAbsorbed) > 0 && A.u8(tsSpecialOn+k) == 1 {
			g.energyAbsorberAttack(a, t)
		}
		if testBit(A[tsSpecials:tsSpecials+5], sp) {
			k++
		}
	}
}

// beamWeaponModifiers is Get_Beam_Weapon_Modifiers_ (0x39434): to-hit and
// damage percent by range and modifications.
func (g *Game) beamWeaponModifiers(a, rng, mods, w int16) (hit, dmg int16) {
	dmg = 100
	switch {
	case mods&2 != 0:
		rng /= 2
	case mods&4 != 0:
		rng = int16(min(int32(rng)*2, 8))
	}
	r := rng
	if g.combatShipHasSpecial(a, 0x19) {
		r = int16(int32(rng) / 3)
	}
	hit += g.exeI16(uint32(0x17d855 + 2*int32(r)))
	if g.wpnI16(int(w), 0x12)&2 != 0 {
		hit <<= 1
	}
	if g.combatShipHasSpecial(a, 0xf) {
		dmg += 0x32
	}
	if mods&0x20 == 0 && g.wpnI16(int(w), 0x12)&4 == 0 {
		dmg += g.exeI16(uint32(0x17d867 + 2*int32(rng)))
	}
	switch {
	case mods&4 != 0:
		hit += 0x19
		dmg -= 0x32
	case mods&2 != 0:
		dmg += 0x32
	}
	return
}

// missileSpeed is Missile_Speed_ (0x3cd21).
func (g *Game) missileSpeed(w, mods, owner int16) int16 {
	s := &g.state
	warp := true
	v := int16(0)
	td := func() bool { return owner < 8 && s.Players[owner].Picks[save.PickTransDimensional] != 0 }
	switch {
	case w == 0x1c:
		v = 6
		if td() {
			v = 0xa
		}
	case w == 0x1d, w == 0x1e:
		v = 8
		if td() {
			v = 0xc
		}
	case w == 0x1f:
		v = 0xa
		if td() {
			v = 0xe
		}
	case w == 0x28, w == 0x14:
		v, warp = 0x18, false
	case w == 0x12, w == 0x13:
		v, warp = 0x14, false
	case w >= 0xe && w < 0x12:
		v = 0xc
	}
	if warp {
		v += int16((g.bestWarpDrive(int(owner)) - 1) * 2)
	}
	if uint16(mods)&0x1000 != 0 {
		v += 4
	}
	return v
}

// missileDcv is Missile_Dcv_ (0x3dfe0): evasion of a missile or fighter.
func (g *Game) missileDcv(w, owner int16) int16 {
	d := g.missileSpeed(w, 0, owner) * 5
	f := int16(0)
	if g.wpnU8(int(w), weaponType) == 4 {
		p := g.fleetPilotBonus(owner)
		if owner < 8 && owner != -1 {
			f = int16(g.state.Players[owner].Picks[save.PickShipDefense]) + p
		}
	}
	switch {
	case w == 0xe:
		return d - 0xa
	case w == 0xf:
		return d + 0xf
	case w == 0x10:
		return d + 0x28
	case w == 0x11:
		return d + 0x46
	case w >= 0x12 && w <= 0x14:
		return d
	case w >= 0x1c && w <= 0x1f:
		return d + f
	}
	return d
}

// fleetPilotBonus is Get_Fleet_Pilot_Bonus_ (0x35eae).
func (g *Game) fleetPilotBonus(p int16) int16 {
	best := int16(0)
	for i := int16(0); i < g.tc.n; i++ {
		t := g.tsp(i)
		if int16(t.u8(tsOwner)) != p || t.u8(tsBoarded) != 0 || t.u8(tsGone) != 0 || t.i16(tsOfficer) <= -1 {
			continue
		}
		best = max(best, g.combatOfficerBonus(int(t.i16(tsOfficer)), 0, 0x04, 0x08))
	}
	return best
}

// fighterOcv is Fighter_Ocv_ (0x3df8d).
func (g *Game) fighterOcv(owner int16) int16 {
	v := g.fleetOffensiveBonus(owner) + g.exeI16(rowAddr(computerTable, computerStride, g.bestComputer(int(owner)), computerBonus))
	if owner < 8 {
		v += int16(g.state.Players[owner].Picks[save.PickShipAttack])
	}
	return g.fleetPilotBonus(owner) + v + 0x32
}

// missileHits is Get_Missile_Hits_ (0x3e095): hits one missile or fighter
// takes to destroy.
func (g *Game) missileHits(w, owner, mods int16) int16 {
	armor := func(m int32) int16 {
		return int16((int32(g.armorField(g.bestArmor(int(owner)), armorHitsPct)) + 100) * m / 0xc8)
	}
	v := int16(0)
	switch {
	case w == 0xe:
		v = 4
	case w == 0xf:
		v = 8
	case w == 0x10:
		v = 0xc
	case w == 0x11:
		v = 0x10
	case w >= 0x12 && w <= 0x14, w == 0x28:
		v = 5
	case w == 0x1c:
		v = armor(6)
	case w == 0x1d:
		v = armor(0xa)
	case w == 0x1e:
		v = armor(8)
	case w == 0x1f:
		v = armor(4)
	}
	if uint16(mods)&0x800 != 0 {
		v += v
	}
	return v
}

// applyDamageToMissile is Apply_Damage_To_Missile_ (0x3a0b9): damage builds
// up on the missile stack; *d becomes the number of missiles it destroys.
func (g *Game) applyDamageToMissile(m int16, d *int16) {
	M := g.msl(m)
	h := g.missileHits(M.i16(tmWeapon), int16(g.tsp(M.i16(tmShip)).u8(tsOwner)), M.i16(tmMods))
	if h < *d {
		*d = h
	}
	*d += M.i16(tmFuel)
	n := int16(0)
	for h <= *d {
		n++
		*d -= h
	}
	M.set16(tmFuel, *d)
	*d = n
}

// fireBeamWeapon is Fire_Beam_Weapon_ (0x394f7): one beam shot (three with
// mods 0x80) at a record or a missile stack.
func (g *Game) fireBeamWeapon(a, t, w, mods, mode int16, dmgOut, absOut, refl *int16) int16 {
	s := &g.state
	*dmgOut, *refl, *absOut = 0, 0, 0
	flags := g.wpnI16(int(w), 0x12)
	result := int16(0)
	if flags&0x100 != 0 {
		mods |= 0x40
	}
	if flags&0x200 != 0 {
		mods |= 0x100
	}
	if flags&0x40 != 0 {
		mods |= 0x10
	}
	hi := int32(g.wpnI16(int(w), weaponMaxDmg))
	lo := int32(g.wpnI16(int(w), weaponMinDmg))
	var facing, rng int16
	if mode == 0 {
		facing = facingShield(g.relativeBearing(t, a))
		rng = g.rangeToShip(a, t)
	} else {
		rng = g.rangeToMissile(a, t)
	}
	hitMod, dmgMod := g.beamWeaponModifiers(a, rng, mods, w)
	A := g.tsp(a)
	var ocv int16
	if mode == 0 {
		ocv = A.i16(tsOcv) - g.tsp(t).i16(tsDcv)
	} else {
		M := g.msl(t)
		ocv = A.i16(tsOcv) - g.missileDcv(M.i16(tmWeapon), int16(g.tsp(M.i16(tmShip)).u8(tsOwner)))
	}
	shots := int16(1)
	if mods&0x80 != 0 {
		shots = 3
		ocv -= 0x14
	}
	if mods&0x10 != 0 {
		ocv += 0x19
	}
	ord := int32(g.tc.ordnance[A.u8(tsOwner)])
	minD := int16(max(((ord-100+int32(dmgMod))*lo+0x32)/100, 1))
	maxD := int16(max(((ord-100+int32(dmgMod))*hi+0x32)/100, int32(minD)))
	span := maxD - minD
	chance := int16(min(0x28-int32(hitMod), 0x5f))
	more := true
	for j := int16(0); j < shots && more; j++ {
		r := int16(s.Random.Random(100))
		if flags&0x400 != 0 {
			r = 100
		}
		if r > 0x5f {
			r = 100
		} else if r += ocv; r > 100 {
			r = 100
		}
		if g.combatShipHasSpecial(t, 8) && s.Random.Random(100) <= 0x1e {
			r = -1000
		}
		if r < chance {
			continue
		}
		var d int16
		if span > 0 {
			d = int16((int32(span)+1)*(int32(r)-int32(chance))/(100-int32(chance)) + int32(minD))
		} else {
			d = minD
		}
		if d > maxD {
			d = maxD
		}
		if mode != 0 {
			result = 2
			n := 1
			if mods&0x100 != 0 {
				n = 4
			}
			for i := 0; i < n; i++ {
				k := d
				g.applyDamageToMissile(t, &k)
				if M := g.msl(t); M.i16(tmCount) <= 0 || M.i16(tmWeapon) == 0 {
					more = false
				}
				*dmgOut += k
			}
			continue
		}
		if t == 0 {
			d /= 2
		}
		if g.combatShipHasSpecial(t, 0x1a) && int16(s.Random.Random(uint32(int32(d)+0xf))) > d {
			*refl += d
			d = 0
		}
		if d <= 0 {
			continue
		}
		sp1 := b2i(g.combatShipHasSpecial(a, 1))
		sp20 := b2i(g.combatShipHasSpecial(a, 0x20))
		if mods&0x100 != 0 {
			for arc := int16(0); arc < 4; arc++ {
				k := d
				var ab int16
				result |= g.applyShipDamage(t, &k, &ab, arc, mods, flags, sp20, sp1, 0)
				*dmgOut += k
				*absOut += ab
			}
			continue
		}
		var ab int16
		result |= g.applyShipDamage(t, &d, &ab, facing, mods, flags, sp20, sp1, 0)
		*dmgOut += d
		*absOut += ab
	}
	return result
}

func b2i(b bool) int16 {
	if b {
		return 1
	}
	return 0
}

// applyDamageToPlanet is Apply_Damage_To_Planet_ (0x3a3c3): shields first,
// then the defence buildings take the damage (a building per armour+100)
// or, without defences, the colony (a hit per 100 of structure damage).
// Biological weapons kill a pop with a chance of the damage in percent.
// mods is unused: the original overwrites edx; flags (ebx) decide.
func (g *Game) applyDamageToPlanet(d *int16, mods, flags int16) int16 {
	s := &g.state
	tc := g.tc
	P := g.tsp(0)
	armor := int16((int32(g.armorField(g.bestArmor(int(P.u8(tsOwner))), armorHitsPct)) + 100) * 100 / 100)
	var si, bx, ret int16
	if flags&0x80 != 0 {
		tc.bomb.Accident = 1
		if P.u8(tsShield) == 0x17 {
			*d = 0
			return 0
		}
		apps := &s.Players[tc.def].AppStatus
		switch {
		case apps[0x6b] == 3:
			*d /= 2
		case apps[0xc1] == 3:
			*d /= 4
		}
		if int16(s.Random.Random(100)) > *d {
			*d = 0
			return 0
		}
		tc.bomb.Accident = 1
		tc.bomb.BioHits++
		*d = 1
		return 0
	}
	if g.combatShipHasSpecial(0, 7) {
		*d = *d/4 - 1
	}
	if g.doesCombatPlanetHaveDefenses() > 0 {
		si = *d - P.i16(tsArc)
		if P.u8(tsShield) != 0 {
			ret = 1
		}
		bx = 0
	} else {
		si = 0
		if P.u8(tsShield) != 0 {
			ret = 1
		}
		bx = *d - P.i16(tsArc+2)
	}
	if si > 0 && flags&8 == 0 {
		if flags&0x20 != 0 {
			si += si
		}
		P.add16(tsStructDmg, si)
		if armor < P.i16(tsStructDmg) {
			n := int16(int32(P.i16(tsStructDmg)) / int32(armor))
			*d = n
			g.destroyColonyDefense(n)
			P.set16(tsStructDmg, int16(int32(P.i16(tsStructDmg))%int32(armor)))
		}
	} else {
		si = 0
	}
	if bx > 0 {
		if tc.colony == -1 {
			*d = si + bx
			return ret
		}
		if flags&8 != 0 {
			bx = 0
		} else {
			if flags&0x20 != 0 {
				bx += bx
			}
			if P.i16(tsStruct) < 0x7d00 {
				P.add16(tsStruct, bx)
			}
			if P.i16(tsStruct) > 0x64 {
				tc.bomb.Kind = 1
			}
		}
	} else {
		bx = 0
	}
	*d = si + bx
	return ret
}

// destroyColonyDefense is Destroy_Colony_Defense_ (0x3a19e): n random
// defence buildings go, with the planet's mounts they carried.
func (g *Game) destroyColonyDefense(n int16) {
	s := &g.state
	tc := g.tc
	if tc.colony == -1 {
		return
	}
	C := &s.Colonies[tc.colony]
	var list []int16
	if C.Buildings[0x1a] != 0 {
		list = append(list, 0x1a)
	}
	if C.Buildings[0x1b] != 0 {
		list = append(list, 0x1b)
	}
	if C.Buildings[0x2a] != 0 {
		list = append(list, 0x2a)
	}
	if C.Buildings[0x2f] != 0 {
		list = append(list, 0x2f)
	}
	if int16(len(list)) < n {
		n = int16(len(list))
	}
	P := g.tsp(0)
	for ; n > 0; n-- {
		k := int16(s.Random.Random(uint32(len(list)))) - 1
		b := list[k]
		g.removeBuilding(int(tc.colony), int(b))
		var kinds []uint8
		switch b {
		case 0x1a:
			kinds = []uint8{1, 2}
		case 0x1b:
			kinds = []uint8{0}
		case 0x2a:
			kinds = []uint8{5}
		case 0x2f:
			kinds = []uint8{4}
		}
		for m := 0; m < 8; m++ {
			typ := g.wpnU8(int(P.i16(tw(m, twWeapon))), weaponType)
			for _, kd := range kinds {
				if typ == kd {
					P.set16(tw(m, twWeapon), 0)
					P.set8(tw(m, twCount), 0)
					break
				}
			}
		}
		list = append(list[:k], list[k+1:]...)
	}
}

// fireBomb is Fire_Bomb_ (0x3a6ca): each shot of the mount hits the planet.
func (g *Game) fireBomb(a int16, k int) int16 {
	s := &g.state
	A := g.tsp(a)
	w := A.i16(tw(k, twWeapon))
	ord := int32(g.tc.ordnance[A.u8(tsOwner)])
	lo := int32(g.wpnI16(int(w), weaponMinDmg)) * ord / 100
	span := int16(int32(g.wpnI16(int(w), weaponMaxDmg))*ord/100 - lo)
	total := int16(0)
	for j := int16(0); int16(A.u8(tw(k, twLeft))) > j; j++ {
		d := int16(int32(int16(s.Random.Random(100)))*int32(span)/100 + int32(int16(lo)))
		g.applyDamageToPlanet(&d, 0, g.wpnI16(int(w), 0x12))
		total += d
	}
	return total
}

// fireSpecialWeapon is Fire_Special_Weapon_ (0x3a82f): mount k's special
// weapon at record (or with mode 1 missile) t.
func (g *Game) fireSpecialWeapon(a, t int16, k int, mode int16) {
	tc := g.tc
	A := g.tsp(a)
	ax, ay := int16(A.u8(tsX))*0x14, int16(A.u8(tsY))*0x14
	var rng int16
	if mode == 0 {
		rng = g.rangeToShip(a, t)
	} else {
		cx, cy := g.shipCenterOffsets(a)
		M := g.msl(t)
		r := rangeXY(ax+cx, ay+cy, M.i16(tmX), M.i16(tmY))
		rng = int16(((int32(r)+0x13)/0x14 + 2) / 3)
	}
	left := func() { A.set8(tw(k, twLeft), 0) }
	switch w := A.i16(tw(k, twWeapon)); {
	case w == 0xd:
		if tc.onScreen == 1 {
			g.doSphericalSFX(a, ax, ay, 2, int16(A.u8(tw(k, twLeft))))
		}
		left()
	case w == 0x20:
		if t == 0 || mode != 0 {
			return
		}
		g.drawStasisBeam(a, t)
		g.tsp(t).set8(tsStasis, uint8(a))
		g.retargetMissilesAimedAt(t)
		A.set8(tw(k, twLeft), A.u8(tw(k, twLeft))-1)
	case w == 0x21:
		if mode == 0 {
			return
		}
		if mt := g.wtype(g.msl(t).i16(tmWeapon)); mt != 1 && mt != 4 {
			return
		}
		if g.msl(t).i16(tmOwner) == int16(A.u8(tsOwner)) {
			return
		}
		g.antiMissileRockets(a, ax, ay, t, k, rng)
	case w == 0x22:
		if t == 0 || mode != 0 {
			return
		}
		g.gyroDestabilizer(a, t, int16(A.u8(tw(k, twLeft))))
		left()
	case w == 0x23, w == 0x2d:
		if t != 0 {
			g.plasmaWeb(a, t, k)
			left()
			return
		}
		if tc.onScreen != 0 {
			return
		}
		left()
	case w == 0x24:
		if tc.onScreen == 1 {
			g.doSphericalSFX(a, ax, ay, 1, int16(A.u8(tw(k, twLeft))))
		}
		left()
	case w == 0x25:
		if t == 0 || mode != 0 {
			return
		}
		g.bhg(a, t)
		left()
	case w == 0x26:
		if mode != 0 {
			return
		}
		g.stellarConverter(a, t, int16(A.u8(tw(k, twLeft))))
		left()
	case w == 0x27:
		if t != 0 && mode == 0 && int16(A.u8(0x42)) != t&0xff && A.u8(tw(k, twLeft)) != 0 {
			g.tractorBeam(a, t)
			left()
		}
		if (t != 0 || tc.onScreen != 0) && A.u8(0x42) == 0xff {
			return
		}
		left()
	case w == 0x2c:
		if tc.onScreen == 1 {
			g.doSphericalSFX(a, ax, ay, 0, int16(A.u8(tw(k, twLeft))))
		}
		left()
	}
}

// tractorBeam is Tractor_Beam_ (0xad3ca).
func (g *Game) tractorBeam(a, t int16) {
	g.tsp(a).set8(0x42, uint8(t))
	g.tsp(t).set16(tsSpeed, g.calcCurrentSpeed(t))
}

// bhg is BHG_ (0x3b627): the target is held in a black hole.
func (g *Game) bhg(a, t int16) {
	if tc := g.tc; tc.onScreen != 0 && tc.superFast == 0 && tc.scr != nil {
		g.drawBHGShot(a, t)
	}
	T := g.tsp(t)
	T.set8(0x136, 1)
	T.set8(0x137, uint8(a))
}

// plasmaWeb is Plasma_Web_ (0xacf83): the web's strength (one roll per shot
// between the ordnance-scaled damages) builds up on the target.
func (g *Game) plasmaWeb(a, t int16, k int) {
	s := &g.state
	tc := g.tc
	if tc.onScreen == 1 && tc.superFast == 0 && tc.scr != nil {
		g.drawPlasmaWebShot(a, t)
	}
	A := g.tsp(a)
	n := int16(A.u8(tw(k, twLeft)))
	w := A.i16(tw(k, twWeapon))
	ord := int32(g.tc.ordnance[A.u8(tsOwner)])
	lo := int16(int32(g.wpnI16(int(w), weaponMinDmg)) * ord / 100)
	hi := int16(int32(g.wpnI16(int(w), weaponMaxDmg)) * ord / 100)
	var sum int32
	for j := int16(0); j < n; j++ {
		r := int32(int16(s.Random.Random(100)))
		sum = int32(int16(sum)) + int32(lo) + r*int32(hi-lo)/100
	}
	g.tsp(t).add16(0x43, int16(sum))
}

// drawPlasmaWebShot is Plasma_Web_'s animation (0xacfdd): the ball flies
// from a to t (6 pixels a frame, 24 in auto combat), then the web by t's
// size wraps it (every frame, every other one in auto combat).
func (g *Game) drawPlasmaWebShot(a, t int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	A, T := g.tsp(a), g.tsp(t)
	if !g.shipCompletelyOnScreen(t) {
		g.must2(g.snapCenterCombatScreen(int16(T.u8(tsX)), int16(T.u8(tsY))))
	}
	ball := g.sfxPic(0xe)
	at := func(i int16, R *tship) (int16, int16) {
		cx, cy := g.shipCenterOffsets(i)
		return cx - 0x14 + (int16(R.u8(tsX))-tc.curX)*0x14, cy - 0x14 + (int16(R.u8(tsY))-tc.curY)*0x14
	}
	x, y := at(a, A)
	x0, y0 := x, y
	ex, ey := at(t, T)
	step := int16(6)
	if tc.autoCombat != 0 {
		step = 0x18
	}
	frame := 0
	frame2 := func() {
		e.V.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		e.V.SetWindow(0, 0, 0x27f, 0x15e)
		e.V.ClipOn()
	}
	snd := g.moxPlaySound(0x1b) // 0xad0e5: the ball's flight
	for x != ex || y != ey {
		frame2()
		g.setAnimationFrame(ball, frame)
		g.must2(e.Draw(int(x), int(y), ball))
		e.V.ClipOff()
		frame = (frame + 1) % ball.Frames
		e.V.ResetWindow()
		g.must2(e.TogglePages())
		absoluteInterpolateLine(&x, &y, x0, y0, ex, ey, step)
	}
	g.fadeoutSound(snd, 0xa)
	web := ball
	switch sz := T.u8(tsSize); {
	case sz == 0:
		web = g.sfxPic(0x24)
	case sz <= 2:
		web = g.sfxPic(0x25)
	case sz == 3:
		web = g.sfxPic(0x26)
	case sz <= 5:
		web = g.sfxPic(0x27)
	}
	x += 0x14 - int16(web.Width)/2
	y += 0x14 - int16(web.Height)/2
	inc := 1
	if tc.autoCombat != 0 {
		inc = 2
	}
	snd = g.moxPlaySound(0x44) // 0xad24a: the web, faded at its end
	defer g.fadeoutSound(snd, 0xa)
	for f := 0; f < web.Frames; f += inc {
		frame2()
		e.RestoreGlassRemapBlock(sc.plasmaWebRemap)
		g.setAnimationFrame(web, f)
		g.must2(e.Draw(int(x), int(y), web))
		e.RestoreGlassRemapBlock(sc.shieldRemap)
		e.V.ClipOff()
		e.V.ResetWindow()
		g.must2(e.TogglePages())
	}
}

// gyroDestabilizer is Gyro_Destablizer_ (0xac136): structure damage by hull
// size; on screen the target spins first, which turns it for good.
func (g *Game) gyroDestabilizer(a, t, n int16) {
	s := &g.state
	tc := g.tc
	T := g.tsp(t)
	sc := tc.scr
	e := g.e
	drawing := tc.onScreen == 1 && tc.superFast == 0
	var saved int16
	snd := 0
	at := func() (int, int) {
		return int(int16(T.u8(tsX))-tc.curX) * 0x14, int(int16(T.u8(tsY))-tc.curY) * 0x14
	}
	if drawing {
		if sc != nil {
			if !g.shipCompletelyOnScreen(t) {
				g.must2(g.snapCenterCombatScreen(int16(T.u8(tsX)), int16(T.u8(tsY))))
			}
			saved = sc.dontDrawShip
			sc.dontDrawShip = t
			e.V.SetPageBack()
			g.must2(g.fullDrawCombatScreen())
			g.must2(g.drawMainCombatScreen())
			e.V.CopyBackToOff()
			e.V.SetPageOff()
			e.V.SetWindow(0, 0, 0x27f, 0x15e)
			e.V.ClipOn()
			x, y := at()
			g.must2(g.drawShip(t, x, y))
			e.V.ClipOff()
			e.V.ResetWindow()
			g.must2(e.TogglePages())
		}
		g.drawGyroSpecialEffect(a, t)
		var spins int32
		for j := 0; j < 0x13; j++ {
			spins += int32(s.Random.Random(0xd))
		}
		for v := int16(spins); v%0x10 == 0xf || v%0x10 < 2; v = int16(spins) {
			spins += int32(s.Random.Random(0xd))
		}
		snd = g.moxPlaySound(3) // 0xac28d: the spin, faded after the damage
		for j := int16(0); j < int16(spins); j++ {
			T.set8(tsFacing, uint8((uint32(T.u8(tsFacing))+1)%0x10))
			if sc != nil {
				e.V.SetPageOff()
				e.V.SetWindow(0, 0, 0x27f, 0x15e)
				e.V.ClipOn()
				x, y := at()
				g.must2(g.drawShip(t, x, y))
				e.V.ResetWindow()
				e.V.ClipOff()
				g.must2(e.TogglePages())
			}
		}
		if sc != nil {
			e.V.SetPageBack()
			g.must2(g.fullDrawCombatScreen())
			e.V.SetPageOff()
			sc.dontDrawShip = saved
		}
	}
	lo := uint32(g.wpnI16(0x22, weaponMinDmg))
	span := int16(g.wpnI16(0x22, weaponMaxDmg) - int16(lo) + 1)
	var sum int32
	for j := int16(0); j < n; j++ {
		r := s.Random.Random(uint32(span))
		sum += int32(r+lo-1) * (int32(T.u8(tsSize)) + 1)
	}
	d := int16(int32(int16(sum)) * int32(tc.ordnance[g.tsp(a).u8(tsOwner)]) / 100)
	if int32(T.i16(tsStruct)) < int32(T.i16(tsStructDmg))+int32(d) {
		d = T.i16(tsStruct) - T.i16(tsStructDmg)
	}
	T.add16(tsStructDmg, d)
	if drawing {
		x, y := at()
		g.addDamageIndicatorForShip(int16(x), int16(y), t, 1, d, 0)
		g.drawDamageMessageQueueUntilDone()
		g.fadeoutSound(snd, 0xa)
	}
}

// antiMissileRockets is Anti_Missile_Rockets_ (0xaba09): each rocket left
// may destroy a missile of the stack, the better the closer.
func (g *Game) antiMissileRockets(a, ax, ay, m int16, k int, rng int16) {
	s := &g.state
	tc := g.tc
	A := g.tsp(a)
	mx, my := g.msl(m).i16(tmX)-tc.curX*0x14, g.msl(m).i16(tmY)-tc.curY*0x14
	drawing := tc.onScreen == 1 && tc.superFast == 0
	if drawing && tc.scr != nil {
		g.drawAntiMissileRockets(a, ax, ay, m)
	}
	var hits int16
	chance := int16(0x46 - (int16(int32(rng+2)*10/3) - 1))
	n := int16(A.u8(tw(k, twLeft)))
	for j := int16(0); j < n && g.msl(m).i16(tmWeapon) != 0; j++ {
		r := int16(s.Random.Random(100))
		A.set8(tw(k, twLeft), A.u8(tw(k, twLeft))-1)
		A.add16(tw(k, twAmmo), -1)
		if r < chance {
			M := g.msl(m)
			M.set16(tmCount, M.i16(tmCount)-1)
			if M.i16(tmCount) < 1 {
				g.killMissile(m)
			}
			hits++
		}
	}
	if hits > 0 && drawing {
		g.antiMissileHits(a, ax, ay, mx, my, hits)
	}
}

// energyAbsorberAttack is Energy_Absorber_Attack_ (0x3b077): the charge
// absorbed so far hits t as a beam.
func (g *Game) energyAbsorberAttack(a, t int16) {
	s := &g.state
	tc := g.tc
	A := g.tsp(a)
	d := int32(A.i16(tsAbsorbed))
	arc := facingShield(g.relativeBearing(t, a))
	_, dmgMod := g.beamWeaponModifiers(a, g.rangeToShip(a, t), 0, 3)
	d = (int32(int16(d))*(int32(dmgMod)+int32(tc.ordnance[A.u8(tsOwner)])-100) + 0x32) / 100
	if g.combatShipHasSpecial(t, 8) && s.Random.Random(100) > 0x1e {
		d = 0
	}
	v := int16(d)
	if t == 0 {
		v = int16(d) / 2
	}
	var ret, ab int16
	if v > 0 {
		sp1 := b2i(g.combatShipHasSpecial(a, 1))
		sp20 := b2i(g.combatShipHasSpecial(a, 0x20))
		ret = g.applyShipDamage(t, &v, &ab, arc, 0, 0, sp20, sp1, 0)
	}
	dead := g.isCombatShipDead(t)
	if tc.onScreen != 0 && tc.superFast == 0 {
		// 0x3b1e1: the discharge is shown as a beam of weapon 0x2e; note bits
		// 2 and 1 of the result go to +4 and +6, the other way round from Fire_Ship_.
		if tc.scr != nil && !g.shipCompletelyOnScreen(t) {
			T := g.tsp(t)
			g.must2(g.snapCenterCombatScreen(int16(T.u8(tsX)), int16(T.u8(tsY))))
		}
		var rr beamRecs
		rr[0] = beamRec16{weapon: 0x2e, shots: 1, result: b2i(ret&2 != 0), pierced: ret & 1, dmg: v, abs: ab}
		T := g.tsp(t)
		tx, ty := (int16(T.u8(tsX))-tc.curX)*0x14, (int16(T.u8(tsY))-tc.curY)*0x14
		ax, ay := (int16(A.u8(tsX))-tc.curX)*0x14, (int16(A.u8(tsY))-tc.curY)*0x14
		g.beamSFXPlay(a, ax, ay, t, tx, ty, &rr, 0)
	}
	if dead != 0 {
		T := g.tsp(t)
		g.destroyShip(t, int16(T.u8(tsX))*0x14, int16(T.u8(tsY))*0x14, dead)
	}
	A.set16(tsAbsorbed, 0)
}

// stellarConverter is Stellar_Converter_ (0xad98a): four arcs take
// shots × 400 × ordnance%; on screen the beam first, the damage after.
func (g *Game) stellarConverter(a, t, shots int16) {
	tc := g.tc
	drawing := tc.onScreen == 1 && tc.superFast == 0
	if drawing && tc.scr != nil {
		g.drawStellarConverterShot(a, t)
	}
	A := g.tsp(a)
	d0 := int16(int32(tc.ordnance[A.u8(tsOwner)]) * (int32(g.exeI16(converterDmg)) * int32(shots)) / 100)
	sp1 := b2i(g.combatShipHasSpecial(a, 1))
	sp20 := b2i(g.combatShipHasSpecial(a, 0x20))
	var dmg, abs int16
	for arc := int16(0); arc < 4; arc++ {
		d := d0
		var ab int16
		g.applyShipDamage(t, &d, &ab, arc, 0, 0, sp20, sp1, 0)
		dmg += d
		abs += ab
	}
	if drawing {
		// 0xadca5: the shield remap back, the damage over the target.
		if tc.scr != nil {
			g.e.RestoreGlassRemapBlock(tc.scr.shieldRemap)
		}
		T := g.tsp(t)
		g.addDamageIndicatorForShip((int16(T.u8(tsX))-tc.curX)*0x14, (int16(T.u8(tsY))-tc.curY)*0x14, t, 1, dmg, abs)
		g.drawDamageMessageQueueUntilDone()
	}
}

// nextMissileID is Next_Missile_Id_ (0x3d21d).
func (g *Game) nextMissileID() int16 {
	for i := int16(0); i < nMissiles; i++ {
		if g.msl(i).i16(tmWeapon) == 0 {
			return i
		}
	}
	return -1
}

// fireMissile is Fire_Missile_ (0x3c892) off screen: a new stack at the
// firer, resolved at once, merged into an identical stack if one exists.
func (g *Game) fireMissile(a, t int16, k int, mode int16) {
	id := g.nextMissileID()
	if id == -1 {
		return
	}
	tc := g.tc
	M := g.msl(id)
	*M = tmissile{}
	A := g.tsp(a)
	M.set16(tmShip, a)
	M.set16(tmFuel, 0)
	M.set16(tmOwner, int16(A.u8(tsOwner)))
	M.set16(tmWeapon, A.i16(tw(k, twWeapon)))
	M.set16(tmCount, int16(A.u8(tw(k, twCount))))
	M.set16(tmMods, A.i16(tw(k, twMods)))
	M.set16(tmTarget, t)
	ox, oy := g.shipCenterOffsets(a)
	M.set16(tmX, ox+int16(A.u8(tsX))*0x14)
	M.set16(tmY, oy+int16(A.u8(tsY))*0x14)
	var tx, ty int16
	if mode == 0 {
		cx, cy := g.shipCenterOffsets(t)
		tx = cx + int16(g.tsp(t).u8(tsX))*0x14
		ty = cy + int16(g.tsp(t).u8(tsY))*0x14
	} else {
		tx, ty = g.msl(t).i16(tmX), g.msl(t).i16(tmY)
	}
	// Missile_Facing_ (0x3cc9c).
	M[tmFace] = uint8(g.getFacing(M.i16(tmX), M.i16(tmY), tx, ty))
	init := g.combatInit
	if !g.combatInitSet {
		init = g.exeI16(0x17d853)
	}
	if init != 0 {
		M[tmBombs] = 0xff
	} else {
		M[tmBombs] = 0
	}
	sp := g.missileSpeed(M.i16(tmWeapon), M.i16(tmMods), int16(A.u8(tsOwner)))
	M[tmSpeed] = uint8(sp)
	M.set16(tmTravel, int16(M[tmSpeed])*0x14)
	M[tmLive] = 1
	M[tmAtMis] = uint8(mode)
	if g.wpnU8(int(M.i16(tmWeapon)), weaponType) == 4 {
		M.set16(tmCount, int16(A.u8(tw(k, twCount)))*4)
		switch M.i16(tmWeapon) {
		case 0x1c, 0x1e:
			M[tmBombs] = 1
		case 0x1d:
			M[tmBombs] = 2
		case 0x1f:
			M[tmBombs] = 4
		}
	}
	if tc.onScreen == 1 && (tc.superFast == 0 || (M.i16(tmWeapon) != 0x13 && M.i16(tmWeapon) != 0x28)) {
		g.missileLaunchFX(id, tx, ty, mode)
	} else {
		g.resolveMissile(id, tx, ty)
	}
	if M.i16(tmWeapon) <= 0 {
		return
	}
	for i := int16(0); i < nMissiles; i++ {
		O := g.msl(i)
		if O.i16(tmWeapon) == M.i16(tmWeapon) && O.i16(tmMods) == M.i16(tmMods) && O.i16(tmTarget) == M.i16(tmTarget) &&
			O.i16(tmX) == M.i16(tmX) && O.i16(tmY) == M.i16(tmY) && O[tmAtMis] == M[tmAtMis] && i != id {
			O.set16(tmCount, O.i16(tmCount)+M.i16(tmCount))
			g.killMissile(id)
		}
	}
	_ = tc
}

// killMissile is Kill_Missile_ (0x3bc80): the stack goes; missiles aimed
// at it look for a new target.
func (g *Game) killMissile(m int16) {
	*g.msl(m) = tmissile{}
	for i := int16(0); i < nMissiles; i++ {
		if O := g.msl(i); O.i16(tmTarget) == m && O[tmAtMis] == 1 {
			g.retargetMissile(i)
		}
	}
}

// retargetMissile is Retarget_Missile_ (0x3ddd8): the nearest legal enemy
// record; fighters without one fly home, other missiles are lost.
func (g *Game) retargetMissile(m int16) {
	s := &g.state
	tc := g.tc
	M := g.msl(m)
	owner := int16(g.tsp(M.i16(tmShip)).u8(tsOwner))
	best := int16(-1)
	if g.wpnU8(int(M.i16(tmWeapon)), weaponType) == 4 || (owner < 8 && s.Players[owner].AppStatus[0x2e] == 3) {
		dist := int16(0x7530)
		enemy := tc.att
		if owner == tc.att {
			enemy = tc.def
		}
		for i := int16(0); i < tc.n; i++ {
			if !g.targetIsLegalForWeapon(M.i16(tmWeapon), 0, i, 0) {
				continue
			}
			if i == 0 && g.doesCombatPlanetHaveDefenses() == 0 {
				continue
			}
			T := g.tsp(i)
			if int16(T.u8(tsOwner)) != enemy || T.u8(tsBoarded) != 0 || T.u8(tsCloak) == 4 || g.shipInStasis(i) || T.u8(tsGone) != 0 || i == M.i16(tmTarget) {
				continue
			}
			if r := g.rangeToMissile(i, m); r < dist {
				dist, best = r, i
			}
		}
	}
	if best != -1 {
		M[tmAtMis] = 0
		M.set16(tmTarget, best)
		return
	}
	if g.wpnU8(int(M.i16(tmWeapon)), weaponType) == 4 && g.tsp(M.i16(tmShip)).u8(tsGone) == 0 {
		M[tmAtMis] = 0
		M.set16(tmTarget, M.i16(tmShip))
		return
	}
	g.killMissile(m)
}

// isMissileTargetAlive is Is_Missile_Target_Alive_ (0x3d299).
func (g *Game) isMissileTargetAlive(m int16) bool {
	M := g.msl(m)
	if M[tmAtMis] != 0 {
		return g.msl(M.i16(tmTarget)).i16(tmCount) > 0
	}
	return M.i16(tmTarget) == 0 || g.isCombatShipDead(M.i16(tmTarget)) == 0
}

// willMissileDoDamage is Will_Missile_Do_Damage_ (0x2aed2): its damage
// beats record i's shield.
func (g *Game) willMissileDoDamage(i, m int16) bool {
	M := g.msl(m)
	w := M.i16(tmWeapon)
	var d int16
	if g.wpnU8(int(w), weaponType) != 4 {
		d = g.wpnI16(int(w), weaponMaxDmg)
	} else {
		owner := g.tsp(M.i16(tmShip)).u8(tsOwner)
		switch w {
		case 0x1f:
			d = g.wpnI16(int(g.fighterBeamOf(int(owner))), weaponMaxDmg)
		case 0x1d, 0x1e:
			d = g.wpnI16(int(g.fighterBombOf(int(owner))), weaponMaxDmg)
		case 0x1c:
			d = 0x64
		}
	}
	T := g.tsp(i)
	sh := int16(0)
	if T.i16(tsShieldCur) > 0 {
		sh = g.exeI16(rowAddr(shieldTable, shieldStride, int(T.u8(tsShield)), shieldStrength))
	}
	return d > sh
}

// safeToFireSphereWeapon is Safe_To_Fire_Sphere_Weapon_ (0x27f9f): the
// spherical weapon around record i hurts the enemy more than its own side.
func (g *Game) safeToFireSphereWeapon(i int16) bool {
	tc := g.tc
	var ownNear, enemyNear int32
	var ownFighters, ownMis, enemyFighters, enemyMis int32
	for j := int16(1); j < tc.n; j++ {
		T := g.tsp(j)
		if T.u8(tsGone) != 0 || j == i || g.shipInStasis(j) {
			continue
		}
		r := g.rangeToShip(i, j)
		same := T.u8(tsOwner) == g.tsp(i).u8(tsOwner)
		switch {
		case r <= 1 && same:
			ownNear += 2
		case r == 2 && same:
			ownNear++
		case r <= 1:
			enemyNear += 2
		case r == 2:
			enemyNear++
		}
	}
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		if M.i16(tmWeapon) <= 0 || g.wpnU8(int(M.i16(tmWeapon)), weaponType) == 2 {
			continue
		}
		if g.rangeToXY(i, M.i16(tmX), M.i16(tmY)) >= 3 {
			continue
		}
		fighter := g.wpnU8(int(M.i16(tmWeapon)), weaponType) == 4
		if M.i16(tmOwner) == int16(g.tsp(i).u8(tsOwner)) {
			if fighter {
				ownFighters += int32(M.i16(tmCount))
			} else {
				ownMis += int32(M.i16(tmCount))
			}
		} else if fighter {
			enemyFighters += int32(M.i16(tmCount))
		} else {
			enemyMis += int32(M.i16(tmCount))
		}
	}
	v := int16(enemyFighters*5 + enemyMis - ownFighters*8 - ownMis + enemyNear*0xf - ownNear*0xa)
	return v >= 0
}

// finalFire is Final_Fire_ (0x36c9c): record i's point defence (and
// spherical weapons) fire at a missile stack about to hit it.
func (g *Game) finalFire(i, px, py, m int16) {
	if !g.willMissileDoDamage(i, m) {
		return
	}
	M := g.msl(m)
	if g.wpnU8(int(M.i16(tmWeapon)), weaponType) == 2 || M.i16(tmCount) <= 0 {
		return
	}
	T := g.tsp(i)
	var saved [8]int16
	for k := 0; k < 8; k++ {
		saved[k] = int16(int8(T.u8(tw(k, twOn))))
		T.set8(tw(k, twOn), 0xff)
	}
	cx, cy := g.shipCenterOffsets(i)
	for k := 0; k < 8 && g.msl(m).i16(tmWeapon) > 0; k++ {
		w := T.i16(tw(k, twWeapon))
		sphere := w == 0xd
		safe := g.safeToFireSphereWeapon(i)
		ok := sphere && saved[k] != -1 && safe
		T.set8(tw(k, twOn), 1)
		pd := g.wpnU8(int(w), weaponType) == 0 && T.i16(tw(k, twMods))&4 != 0
		if pd || ok || w == 0x21 {
			dx, dy := px-cx, py-cy
			fire := true
			if w == 0xd {
				fire = int32(dx)%0x14 == 0 && int32(dy)%0x14 == 0 &&
					int32(dx)/0x14 == int32(T.u8(tsX)) && int32(dy)/0x14 == int32(T.u8(tsY))
			}
			if fire {
				g.fireShipAt(i, dx, dy, m, M.i16(tmX), M.i16(tmY), 1)
			}
		}
		T.set8(tw(k, twOn), 0xff)
	}
	for k := 0; k < 8; k++ {
		T.set8(tw(k, twOn), uint8(saved[k]))
	}
}

// resolveMissile is Resolve_Missile_ (0x3d2df) for a stack reaching its
// target at (px, py).
func (g *Game) resolveMissile(m, px, py int16) {
	s := &g.state
	tc := g.tc
	M := g.msl(m)
	owner := M.i16(tmOwner)
	tc.mslWarning = 0 // 0x3d30f: every call, before the live test
	if M[tmLive] != 1 {
		return
	}
	t := M.i16(tmTarget)
	mods := M.i16(tmMods)
	w := M.i16(tmWeapon)
	atMis := int16(M[tmAtMis])
	ship := M.i16(tmShip)
	if ship == t && atMis == 0 {
		left := int16(int32(M.i16(tmCount)) / 4)
		A := g.tsp(ship)
		for k := 0; left != 0 && k < 8; k++ {
			if A.i16(tw(k, twWeapon)) != w || A.i16(tw(k, twAmmo)) != 0 {
				continue
			}
			A.set16(tw(k, twAmmo), 1)
			A.set8(tw(k, twLeft), 0)
			if c := int16(A.u8(tw(k, twCount))); c <= left {
				left -= c
			} else {
				A.set8(tw(k, twCount), uint8(left))
				left = 0
			}
		}
		g.killMissile(m)
		return
	}
	if !g.isMissileTargetAlive(m) {
		return
	}
	var shield, evasion, hitFacing int16
	if atMis == 0 {
		if g.combatShipHasSpecial(t, 0x13) {
			for j := int16(0); j < M.i16(tmCount); j++ {
				if s.Random.Random(100) < 0x32 {
					M.set16(tmCount, M.i16(tmCount)-1)
				}
			}
			if tc.onScreen == 1 && tc.superFast == 0 && tc.scr != nil {
				g.drawLightningField(t, m, px, py)
			}
		}
		g.finalFire(t, px, py, m)
		rb := int32(g.relativeBearingXY(t, M.i16(tmX), M.i16(tmY)))
		hitFacing = int16(((rb*2+0x16)/0x2d + int32(g.tsp(t).u8(tsFacing))) % 0x10)
		if g.wpnU8(int(w), weaponType) == 4 {
			shield = g.findWeakestShield(t)
			evasion = 0
		} else {
			shield = facingShield(g.relativeBearingXY(t, M.i16(tmX), M.i16(tmY)))
			evasion = g.tsp(t).i16(tsEvasion)
		}
		e := evasion - 0x28
		switch {
		case owner >= 8:
			evasion = e
		case s.Players[owner].AppStatus[0xa0] == 3:
			evasion -= 0x46
		case s.Players[owner].AppStatus[0x74] == 3:
			evasion = e
		case s.Players[owner].AppStatus[0xb5] == 3:
			evasion -= 0x14
		}
		if evasion < 0 {
			evasion = 0
		} else if uint16(mods)&0x400 != 0 {
			evasion /= 2
		}
		if w == 0x28 {
			evasion = 0
		}
	}
	if M.i16(tmCount) <= 0 {
		g.killMissile(m)
		return
	}
	if uint16(mods)&0x200 != 0 {
		M.set16(tmCount, M.i16(tmCount)<<2)
	}
	if g.wpnU8(int(w), weaponType) == 4 {
		g.resolveFighters(m, t, shield, atMis, px, py)
		return
	}
	pct := tc.ordnance[M.i16(tmOwner)]
	if uint16(mods)&0x4000 != 0 {
		pct += 0x32
	}
	dmg := int16(int32(pct) * int32(g.wpnI16(int(w), weaponMaxDmg)) / 100)
	if (w == 0x14 || w == 0x28) && mods&0x20 == 0 {
		var cut int32
		if w == 0x14 {
			v := int16(int32(g.missileSpeed(w, mods, M.i16(tmOwner)))*0x14) - M.i16(tmTravel) - 0x14
			if v < 0 {
				v = 0
			}
			cut = int32(v) / 0x14 * 5
		} else {
			v := int16(10000) - M.i16(tmTravel) - 0x14
			if v < 0 {
				v = 0
			}
			cut = int32(v) / 0x14 * 0xf
		}
		dmg = int16(int32(dmg) - cut)
		if dmg <= 0 {
			M.set16(tmCount, 0)
		}
	}
	if t == 0 && g.wpnU8(int(w), weaponType) == 2 {
		dmg /= 2
	}
	x38 := int16(uint16(mods) & 0x2000)
	var hitDmg, hitAbs int16
	for j := int16(0); j < M.i16(tmCount); j++ {
		r := int16(s.Random.Random(100))
		if g.combatShipHasSpecial(t, 8) && s.Random.Random(100) < 0x1e {
			r = -1000
		}
		if g.tsp(t).u8(tsCloak) == 1 && s.Random.Random(100) < 0x32 {
			r = -1000
		}
		if r <= evasion {
			continue
		}
		if uint16(mods)&0x100 != 0 {
			for arc := int16(0); arc < 4; arc++ {
				d := dmg
				var ab int16
				g.applyShipDamage(t, &d, &ab, arc, mods, 0, 0, 0, x38)
				hitDmg += d
				hitAbs += ab
			}
			continue
		}
		d := dmg
		var ab int16
		g.applyShipDamage(t, &d, &ab, shield, mods, 0, 0, 0, x38)
		hitDmg += d
		hitAbs += ab
	}
	g.killMissile(m)
	g.missileHitView(t, atMis, hitDmg, hitAbs, hitFacing, px, py)
	g.missileAftermath(m, t, atMis, w)
}

// missileHitView is Resolve_Missile_'s view part (0x3d906..0x3da93): a hit
// on a ship outside the view centres it on the impact point, then the
// damage numbers rise, over a shield hit when some was absorbed.
func (g *Game) missileHitView(t, atMis, dmg, abs, facing, px, py int16) {
	tc := g.tc
	sc := tc.scr
	if int32(dmg)+int32(abs) <= 0 || atMis != 0 || tc.onScreen != 1 || tc.superFast != 0 {
		return
	}
	if sc != nil && !g.shipCompletelyOnScreen(t) {
		g.must2(g.snapCenterCombatScreen(px/0x14, py/0x14))
	}
	ox, oy := g.shipCenterOffsets(t)
	vx, vy := px-tc.curX*0x14, py-tc.curY*0x14
	g.addDamageIndicatorForShip(vx-ox, vy-oy, t, 1, dmg, abs)
	if abs > 0 {
		g.drawShieldHitOnceThrough(t, facing, vx, vy)
		return
	}
	e := g.e
	for !g.doneDrawingDamageMessageQueue() {
		if sc != nil {
			e.MarkTime()
			e.V.SetPageOff()
			g.must2(g.drawMainCombatScreen())
			e.V.SetWindow(0, 0, 0x27f, 0x15e)
			e.V.ClipOn()
			if t == sc.dontDrawShip {
				g.must2(g.drawShip(t, int(vx-ox), int(vy-oy)))
			}
		}
		g.drawDamageMessageQueue()
		if sc != nil {
			g.must2(e.TogglePages())
			g.must2(e.ReleaseTime(1))
			e.V.ClipOff()
			e.V.ResetWindow()
		}
	}
}

// drawShieldHitOnceThrough is Draw_Shield_Hit_Once_Through_ (0xa9867): the
// shield flash on record t at view centre (x, y) played out with the
// damage numbers.
func (g *Game) drawShieldHitOnceThrough(t, facing, x, y int16) {
	tc := g.tc
	sc := tc.scr
	b := &tc.beam
	e := g.e
	size := int16(g.tsp(t).u8(tsSize))
	if size > 5 {
		g.logOnce("Ship_Size_To_Shield_Size_: a size past 5 reads stale stack")
		size = 0
	}
	ss := shipSizeToShieldSize(size)
	ox, oy := g.shipCenterOffsets(t)
	half := g.shieldDiameter(ss) / 2
	sx, sy := x-half, y-half
	g.shieldAnims(0, ss, false, 0)
	var ctr int16
	b.shieldHitNow = true
	for {
		if sc != nil {
			e.MarkTime()
			e.V.SetPageOff()
			g.must2(g.drawMainCombatScreen())
			e.V.SetWindow(0, 0, 0x27f, 0x15e)
			e.V.ClipOn()
			if t == sc.dontDrawShip {
				g.must2(g.drawShip(t, int(x-ox), int(y-oy)))
			}
		}
		g.adjustShieldDisplayStatus(&ctr)
		g.drawShieldHit(sx, sy, t, false, 0, facing, ctr)
		g.drawDamageMessageQueue()
		if sc != nil {
			g.must2(e.ReleaseTime(1))
			g.must2(e.TogglePages())
			e.V.ClipOff()
			e.V.ResetWindow()
		}
		b.shieldHitNow = false
		if b.shieldDisplay == 0 && g.doneDrawingDamageMessageQueue() {
			return
		}
	}
}

// resolveFighters is the fighter part of Resolve_Missile_ (0x3d801).
func (g *Game) resolveFighters(m, t, shield, atMis, px, py int16) {
	M := g.msl(m)
	w := M.i16(tmWeapon)
	M[tmLive] = 0
	switch w {
	case 0x1c:
		g.resolveCapture(M.i16(tmShip), t, M.i16(tmCount), px, py)
		M[tmBombs]--
	default:
		var d, ab int16
		if w != 0x1e {
			g.fireFighterBeam(m, shield, atMis, &d, &ab)
			if w != 0x1d {
				M[tmBombs]--
			}
		}
		g.fighterBombs(m, shield, &d, &ab)
		if g.tc.onScreen == 1 && g.tc.superFast == 0 {
			g.fighterCombatSFX(m, px, py, d, ab)
		}
	}
	g.missileAftermath(m, t, atMis, w)
}

func (g *Game) fighterBombs(m, shield int16, dmg, abs *int16) {
	M := g.msl(m)
	if M[tmBombs] > 0 && M.i16(tmWeapon) != 0x1f {
		var d, ab int16
		g.fireFighterBomb(m, shield, &d, &ab)
		M[tmBombs]--
		*dmg += d
		*abs += ab
	}
}

// missileAftermath is the end of Resolve_Missile_ (0x3d906 on): a dead
// target is destroyed; fighters out of bombs fly home or are lost.
func (g *Game) missileAftermath(m, t, atMis, w int16) {
	if atMis == 0 {
		if dead := g.isCombatShipDead(t); dead != 0 {
			T := g.tsp(t)
			g.destroyShip(t, int16(T.u8(tsX))*0x14, int16(T.u8(tsY))*0x14, dead)
		}
	}
	if g.wpnU8(int(w), weaponType) != 4 {
		return
	}
	M := g.msl(m)
	if M[tmBombs] == 0 {
		ship := M.i16(tmShip)
		if int16(g.tsp(ship).u8(tsOwner)) == M.i16(tmOwner) && g.combatShipIsFunctional(ship) && M.i16(tmWeapon) != 0x1c {
			M[tmAtMis] = 0
			M[tmLive] = 1
			M.set16(tmTarget, ship)
			return
		}
		g.killMissile(m)
		return
	}
	if M.i16(tmTarget) == 0 && M[tmAtMis] == 0 && g.doesCombatPlanetHaveDefenses() == 0 {
		g.retargetMissile(m)
	}
}

// combatShipIsFunctional is Combat_Ship_Is_Functional_ (0x42a27).
func (g *Game) combatShipIsFunctional(i int16) bool {
	t := g.tsp(i)
	return t.u8(tsGone) == 0 && t.u8(tsBoarded) == 0 && !g.shipInStasis(i)
}

// fireFighterBeam is Fire_Fighter_Beam_ (0x3ad57): each fighter of the
// stack shoots the owner's fighter beam.
func (g *Game) fireFighterBeam(m, shield, atMis int16, dmgOut, absOut *int16) {
	s := &g.state
	M := g.msl(m)
	t := M.i16(tmTarget)
	owner := int16(g.tsp(M.i16(tmShip)).u8(tsOwner))
	*dmgOut, *absOut = 0, 0
	w := g.fighterBeamOf(int(owner))
	flags := g.wpnI16(int(w), 0x12)
	mods := int16(0)
	if flags&0x100 != 0 {
		mods = 0x40
	}
	hi := g.wpnI16(int(w), weaponMaxDmg)
	lo := g.wpnI16(int(w), weaponMinDmg)
	var ocv int16
	if atMis == 0 {
		ocv = g.fighterOcv(owner) - g.tsp(t).i16(tsDcv)
	} else {
		T := g.msl(t)
		ocv = g.fighterOcv(owner) - g.missileDcv(T.i16(tmWeapon), int16(g.tsp(T.i16(tmShip)).u8(tsOwner)))
	}
	span := hi - lo
	for j := int16(0); j < g.msl(m).i16(tmCount) && g.isMissileTargetAlive(m); j++ {
		r := int16(s.Random.Random(100))
		if r > 0x5f {
			r = 100
		} else if r += ocv; r > 100 {
			r = 100
		}
		if r < 0x28 {
			continue
		}
		var d int16
		if span > 0 {
			d = int16((int32(r)-0x28)*(int32(span)+1)/0x3c + int32(lo))
		} else {
			d = lo
		}
		if atMis != 0 {
			g.applyDamageToMissile(t, &d)
			g.msl(t).set16(tmCount, g.msl(t).i16(tmCount)-d)
			*dmgOut += d
			continue
		}
		ab := int16(0)
		if t == 0 {
			d /= 2
		}
		g.applyShipDamage(t, &d, &ab, shield, mods, flags, 0, 0, 0)
		*dmgOut += d
		*absOut += ab
	}
	if !g.isMissileTargetAlive(m) && atMis == 1 {
		g.retargetMissile(m)
		g.killMissile(t)
	}
}

// fireFighterBomb is Fire_Fighter_Bomb_ (0x3ac20): each fighter drops the
// owner's fighter bomb.
func (g *Game) fireFighterBomb(m, shield int16, dmgOut, absOut *int16) {
	s := &g.state
	M := g.msl(m)
	t := M.i16(tmTarget)
	owner := g.tsp(M.i16(tmShip)).u8(tsOwner)
	*absOut, *dmgOut = 0, 0
	w := g.fighterBombOf(int(owner))
	flags := g.wpnI16(int(w), 0x12)
	mods := int16(0)
	if flags&0x100 != 0 {
		mods = 0x40
	}
	lo := g.wpnI16(int(w), weaponMinDmg)
	span := g.wpnI16(int(w), weaponMaxDmg) - lo
	for j := int16(0); j < g.msl(m).i16(tmCount); j++ {
		r := int16(s.Random.Random(100))
		if r <= 0 {
			continue
		}
		var d int16
		if span > 0 {
			d = int16((100/(int32(span)*2)+int32(r))*int32(span)/100 + int32(lo))
		} else {
			d = lo
		}
		var ab int16
		g.applyShipDamage(t, &d, &ab, shield, mods, flags, 0, 0, 0)
		*dmgOut += d
		*absOut += ab
	}
}
