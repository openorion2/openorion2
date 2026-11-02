package game

import (
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// The tactical AI (CMBTAI.C): what Do_Auto_Ship_Turn_ decides for a
// computer ship, or for the human's ships in Auto mode. Spec:
// docs/combat/tactical.md.

func (g *Game) wtype(w int16) uint8 { return g.wpnU8(int(w), weaponType) }

// weaponIsOn is Weapon_Is_On_ (0x4584c).
func (g *Game) weaponIsOn(i int16, k int) bool { return g.tsp(i).u8(tw(k, twOn)) == 1 }

// getWeaponArc is Get_Weapon_Arc_ (0x32ad1): the arcs covering a bearing.
func getWeaponArc(b int16) uint8 {
	var a uint8
	if b < 0x3d || b > 299 {
		a = 1
	}
	if b < 0x79 || b > 0xef {
		a += 2
	}
	if b > 0x3b && b < 0x12d {
		a += 4
	}
	if b > 0x77 && b < 0xf1 {
		a += 8
	}
	return a
}

func (g *Game) shieldStr(class uint8) int16 {
	return g.exeI16(rowAddr(shieldTable, shieldStride, int(class), shieldStrength))
}

// expectedWeaponDamage is Expected_Weapon_Damage_ (0x289c4): what mount k of
// a would do to record (or with mode 1 missile) t at range rng; rng 0 means
// "anywhere, every mount".
func (g *Game) expectedWeaponDamage(a int16, k int, t, mode, rng int16) int16 {
	A := g.tsp(a)
	w := A.i16(tw(k, twWeapon))
	mods := A.i16(tw(k, twMods))
	n := int16(A.u8(tw(k, twCount)))
	var v uint32
	typ := g.wtype(w)
	if w >= 1 && g.targetIsLegalForWeapon(w, mods, t, mode) {
		if rng == 0 {
			rng = 1
		} else {
			var arc uint8
			if mode == 0 {
				arc = getWeaponArc(g.relativeBearing(a, t))
			} else {
				M := g.msl(t)
				arc = getWeaponArc(g.relativeBearingXY(a, M.i16(tmX), M.i16(tmY)))
			}
			n = 0
			ammo := A.i16(tw(k, twAmmo))
			if g.weaponIsOn(a, k) && A.u8(tw(k, twArc))&arc != 0 &&
				((typ != 2 && ammo != 0) || (typ == 2 && ammo == 0)) &&
				g.weaponInRange(k, a, t, mode, rng) {
				n = int16(A.u8(tw(k, twLeft)))
			}
		}
		if n != 0 {
			v = g.expectedDamageByType(a, k, t, mode, rng, w, mods, n, typ)
		}
	}
	if g.combatShipHasSpecial(t, 7) {
		v >>= 2
	}
	if typ == 0 || typ == 3 {
		v = (v + 99) / 100
	}
	return int16(v)
}

func (g *Game) expectedDamageByType(a int16, k int, t, mode, rng, w, mods, n int16, typ uint8) uint32 {
	switch typ {
	case 1, 4, 2:
		return uint32(int32(g.expectedMissileDamage(t, w, n, mods, int16(g.tsp(a).u8(tsOwner)))))
	case 0, 3:
		return uint32(g.expectedBeamBombDamage(a, k, t, n, rng, mode))
	case 5:
	default:
		return 0
	}
	lo := int32(g.wpnI16(int(w), weaponMinDmg))
	hi := int32(g.wpnI16(int(w), weaponMaxDmg))
	avg := (lo + hi) / 2
	T := g.tsp(t)
	shield := func() int32 { return int32(g.shieldStr(T.u8(tsShield))) }
	switch {
	case w == 0xd:
		return uint32(int32(n) * avg * (int32(T.u8(tsSize)) + 1))
	case w == 0x20, w == 0x25:
		return 1000
	case w == 0x21:
		if mode == 0 {
			return 0
		}
		M := g.msl(t)
		h := int32(g.missileHits(M.i16(tmWeapon), int16(g.tsp(M.i16(tmShip)).u8(tsOwner)), M.i16(tmMods)))
		return uint32(h * int32(int16(((int32(rng)-1)*-10+0x46)*int32(n)+0x32)/100))
	case w == 0x22:
		return uint32(avg * (int32(T.u8(tsSize)) + 1) * int32(n))
	case w == 0x24:
		return uint32(int32(n) * (avg - shield()) * (int32(T.u8(tsSize)) + 1))
	case w == 0x27:
		if T.i16(tsSpeed) > 0 {
			return 0x32
		}
		return 0
	case w == 0x2c:
		return uint32(int32(n) * (avg - shield()) * (int32(T.u8(tsSize)) + 1) * 4)
	case w == 0x23, w == 0x26, w == 0x2d: // 0x28c36: the converter shares the web's case
		return uint32(int32(n) * (avg - shield()) * 4)
	}
	return 0
}

// expectedMissileDamage is Expected_Missile_Damage_ (0x28eae): n missiles
// or fighters of weapon w (owner p) against record t.
func (g *Game) expectedMissileDamage(t, w, n, mods, p int16) int16 {
	s := &g.state
	tc := g.tc
	var sh int16
	T := g.tsp(t)
	if t == 0 {
		if g.doesCombatPlanetHaveDefenses() == 0 {
			sh = T.i16(tsArc + 2)
		} else {
			sh = T.i16(tsArc)
		}
	} else {
		if T.i16(tsShieldCur) > 0 {
			sh = g.shieldStr(T.u8(tsShield))
		}
		if g.combatShipHasSpecial(t, 0xd) {
			sh += 3
		}
	}
	var d int16
	if g.wtype(w) != 4 {
		v := uint32(uint16(g.wpnI16(int(w), weaponMaxDmg)))
		if mods&0x4000 != 0 {
			v = uint32(int32(g.wpnI16(int(w), weaponMaxDmg)) * 3 / 2)
		}
		dmg := int16(v)
		if mods&0x200 != 0 {
			dmg = int16(v << 2)
		}
		dmg = int16(int32(dmg) * int32(tc.ordnance[p]) / 100)
		ev := T.i16(tsEvasion)
		e := ev - 0x28
		if p < 8 {
			ap := &s.Players[p].AppStatus
			switch {
			case ap[0xa0] == 3:
				e = ev - 0x46
			case ap[0x74] == 3:
			case ap[0xb5] == 3:
				e = ev - 0x14
			default:
				e = ev
			}
		}
		if w == 0x28 {
			e = 0
		}
		if e < 0 {
			e = 0
		} else if mods&0x400 != 0 {
			e /= 2
		}
		if (w == 0x14 || w == 0x28) && mods&0x20 == 0 {
			dmg /= 2
		}
		d = int16((100 - int32(e)) * int32(int16((dmg-sh)*n)) / 100)
		if T.u8(tsCloak) == 1 {
			d /= 2
		}
	} else {
		var base int16
		switch {
		case w == 0x1c:
			base = 100
		case w == 0x1d, w == 0x1e:
			base = g.wpnI16(int(g.fighterBombOf(int(p))), weaponMaxDmg)
		case w == 0x1f:
			base = g.wpnI16(int(g.fighterBeamOf(int(p))), weaponMaxDmg)
		}
		d = (int16(int32(base)*int32(tc.ordnance[p])/100) - sh) * n * 2
	}
	if d < 1 {
		return 0
	}
	return d
}

// expectedBeamBombDamage is Expected_Beam_Bomb_Damage_ (0x290da).
func (g *Game) expectedBeamBombDamage(a int16, k int, t, n, rng, mode int16) int32 {
	A := g.tsp(a)
	mods := A.i16(tw(k, twMods))
	w := A.i16(tw(k, twWeapon))
	var sh int16
	planet := false
	var shield int32
	if mode == 0 && (g.combatShipHasSpecial(t, 0xd) || mods&0x40 == 0) {
		if t == 0 {
			planet = true
			sh = g.tsp(0).i16(tsShieldMax)
		} else {
			T := g.tsp(t)
			if T.i16(tsShieldCur) > 0 {
				sh = g.shieldStr(T.u8(tsShield))
			}
			if g.combatShipHasSpecial(t, 0xd) {
				sh += 3
			}
			if g.wpnU8(int(w), 0x13)&1 != 0 || (mods&0x40 != 0 && !g.combatShipHasSpecial(t, 0xd)) {
				sh = 0
			}
		}
		shield = int32(sh)
	}
	var chance, ocv int32
	var dmgMod int16
	if g.wtype(w) == 0 {
		var hit int16
		hit, dmgMod = g.beamWeaponModifiers(a, rng, mods, w)
		chance = 0x28 - int32(hit)
		if chance > 0x5e {
			chance = 0x5f
		}
		var dcv int16
		if mode == 0 {
			dcv = g.tsp(t).i16(tsDcv)
		} else {
			M := g.msl(t)
			dcv = g.missileDcv(M.i16(tmWeapon), int16(g.tsp(M.i16(tmShip)).u8(tsOwner)))
		}
		ocv = int32(A.i16(tsOcv)) - int32(dcv)
	} else {
		dmgMod = 100
	}
	ord := int32(g.tc.ordnance[A.u8(tsOwner)])
	lo := (int32(g.wpnI16(int(w), weaponMinDmg))*(ord-100+int32(dmgMod)) + 0x32) / 100
	if lo < 2 {
		lo = 1
	}
	hi := (int32(g.wpnI16(int(w), weaponMaxDmg))*(int32(dmgMod)+ord-100) + 0x32) / 100
	if hi < lo {
		hi = lo
	}
	if planet && g.wtype(w) == 0 {
		bs := g.bestShipShield(int(g.tc.def))
		lo /= 2
		hi /= 2
		if bs == 7 {
			lo /= 4
			hi /= 4
		} else {
			lo -= int32(g.shieldStr(uint8(bs)))
			hi -= int32(g.shieldStr(uint8(bs)))
		}
		if lo < 0 {
			lo = 0
		}
		if hi < 0 {
			hi = 0
		}
	}
	return actualExpectedDamage(lo, hi, chance, ocv, shield) * int32(n)
}

// actualExpectedDamage is Actual_Expected_Damage_ (0xec0de): the sum over
// the hundred rolls of the damage past the shield.
func actualExpectedDamage(lo, hi, chance, ocv, shield int32) int32 {
	var sum int32
	if shield >= hi {
		return 0
	}
	for r := int32(0); r < 100; r++ {
		v := r + 1 + ocv
		if r+1 > 0x5f || v > 100 {
			v = 100
		}
		if chance > v {
			continue
		}
		d := hi
		if v != 100 && lo < hi {
			d = (v-chance)*(hi+1-lo)/(100-chance) + lo
		} else if v != 100 {
			d = lo
		}
		if d-shield > 0 {
			sum += d - shield
		}
	}
	return sum
}

// totalInternalSpace is Total_Internal_Space_ (0x36b64).
func (g *Game) totalInternalSpace(i int16) int16 {
	T := g.tsp(i)
	v := int16(int32(T.i16(tsDriveCur))/2) + T.i16(tsCompCur)
	for k := 0; k < 8; k++ {
		v += T.i16(tsCur + 4 + 2*k)
	}
	for k := 0; k < 8; k++ {
		v += T.i16(tsCur + 0x24 + 2*k)
	}
	return v + T.i16(tsShieldCur)
}

// maxTotalInternalSpace is Max_Total_Internal_Space_ (0x36be2).
func (g *Game) maxTotalInternalSpace(i int16) int16 {
	T := g.tsp(i)
	v := int16(int32(T.i16(tsDriveMax))/2) + T.i16(tsCompMax)
	for k := 0; k < 8; k++ {
		v += T.i16(tsWpnSpace + 2*k)
	}
	for k := 0; k < 8; k++ {
		v += T.i16(tsSpcSpace + 2*k)
	}
	return v + T.i16(tsShieldHP)
}

// getTotalHits is Get_Total_Hits_ (0x2bb06): what it takes to destroy a
// record (the planet: its defence buildings).
func (g *Game) getTotalHits(i int16) int16 {
	if i == 0 {
		per := (int32(g.armorField(g.bestArmor(int(g.tsp(0).u8(tsOwner))), armorHitsPct)) + 100) * 100 / 100
		d := g.doesCombatPlanetHaveDefenses()
		var v int32
		for b := 0; b < 4; b++ {
			if int32(d)&(1<<b) != 0 {
				v += per
			}
		}
		return int16(v)
	}
	T := g.tsp(i)
	return g.totalInternalSpace(i) + T.i16(tsArmorHits) + T.i16(tsStruct) - T.i16(tsStructDmg) + g.calcCombatShieldMax(i)
}

// missileOverkillCheck is Missile_Overkill_Check_ (0x293d8): the missiles
// already flying at i will kill it.
func (g *Game) missileOverkillCheck(i int16) bool {
	var sum int16
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		if M.i16(tmTarget) == i && M.i16(tmShip) != i && M.i16(tmWeapon) > 0 {
			sum += g.expectedMissileDamage(i, M.i16(tmWeapon), M.i16(tmCount), M.i16(tmMods), int16(g.tsp(M.i16(tmShip)).u8(tsOwner)))
		}
	}
	if i == 0 {
		return int32(sum) > int32(g.getTotalHits(0))*3/2
	}
	return sum > g.getTotalHits(i)
}

func (g *Game) shipInBlackHole(i int16) bool { return g.tsp(i).u8(0x137) != 0xff }
func (g *Game) beingCaptured(i int16) bool   { return g.tsp(i)[0x48]&0x10 != 0 }
func (g *Game) alreadyBoarded(i int16) bool  { return g.tsp(i)[0x48]&2 != 0 }
func (g *Game) usedTeleporter(i int16) bool  { return g.tsp(i)[0x48]&1 != 0 }

// targetShipValue is Target_Ship_Value_ (0x2a239): how much a wants to shoot
// at t; inRange 1 counts only mounts that reach from here.
func (g *Game) targetShipValue(a, t, inRange int16) int32 {
	var dmg, back int16
	hits := g.getTotalHits(t)
	if g.shipInStasis(t) {
		return -20000
	}
	rng := int16(0)
	if inRange >= 1 {
		rng = g.rangeToShip(a, t)
		if rng < 1 {
			rng = 1
		}
	}
	A := g.tsp(a)
	for k := 0; k < 8; k++ {
		d := g.expectedWeaponDamage(a, k, t, 0, rng)
		if t != 0 && g.wtype(A.i16(tw(k, twWeapon))) == 4 {
			d *= 2
		}
		dmg += d
		if g.tsp(t).u8(tsBoarded) == 0 {
			back += g.expectedWeaponDamage(t, k, a, 0, 0)
		}
	}
	if dmg == 0 {
		return -10000
	}
	v := int32(uint32(int32(dmg)*int32(back)*3) / uint32(int32(hits)))
	if inRange != 0 && t != 0 {
		T := g.tsp(t)
		if T.i16(tsShieldCur) > 0 {
			arc := T.i16(tsArc + 2*int(facingShield(g.relativeBearing(t, a))))
			switch {
			case int32(dmg) < int32(arc)/4:
				v = 1
			case dmg < arc:
				v /= 2
				if v < 2 {
					v = 2
				}
			}
		}
	}
	if t == 0 {
		if dmg > hits {
			v += int32(back) * 3
		}
	} else if int32(dmg) > int32(hits)/3 {
		v += int32(back) * 5
	}
	if g.shipInBlackHole(t) {
		v /= 2
	}
	if g.missileOverkillCheck(t) {
		v -= 1000
	}
	if back == 0 {
		v -= 2000
	}
	if g.beingCaptured(t) {
		v -= 5000
	}
	return v
}

// selectShipToTarget is Select_Ship_To_Target_ (0x2a46a).
func (g *Game) selectShipToTarget(a int16) int16 {
	tc := g.tc
	best, bv := int16(-1), int32(-5000)
	for i := int16(1); i < tc.n; i++ {
		T := g.tsp(i)
		if T.u8(tsGone) != 0 || T.u8(tsOwner) == g.tsp(a).u8(tsOwner) || T.u8(tsCloak) == 4 || g.shipInStasis(i) {
			continue
		}
		if v := g.targetShipValue(a, i, 0); v > bv {
			bv, best = v, i
		}
	}
	return best
}

// chooseTarget is Choose_Target_ (0x2a4e8): the ship to close in on; an
// attacker without one goes for the planet.
func (g *Game) chooseTarget(a int16) int16 {
	g.tsp(a).set8(0x3f, 0)
	t := g.selectShipToTarget(a)
	if t == -1 && g.tsp(0).u8(tsGone) == 0 && int16(g.tsp(a).u8(tsOwner)) == g.tc.att {
		t = 0
	}
	return t
}

// combatShipClass is Combat_Ship_Class_ (0x2a545): 0 close range, 1 medium,
// 2 long range, from where a's weapons do best against t.
func (g *Game) combatShipClass(a, t int16) uint8 {
	var short, mid, long int16
	A := g.tsp(a)
	for k := 0; k < 8; k++ {
		w := A.i16(tw(k, twWeapon))
		if w == 0 {
			continue
		}
		ewd := func(r int16) int16 { return g.expectedWeaponDamage(a, k, t, 0, r) }
		switch g.wtype(w) {
		case 0:
			d1, d5, d12 := ewd(1), ewd(5), ewd(0xc)
			switch {
			case int32(d5)/2 < int32(d12):
				long += d12
			case int32(d1)/2 < int32(d5):
				mid += d5
			default:
				short += d1
			}
		case 1:
			if g.tc.turn == 1 {
				mid += ewd(5)
			} else {
				long += ewd(0xc)
			}
		case 2:
			switch w {
			case 0x14, 0x28:
				short += ewd(1)
			case 0x13:
				mid += ewd(5)
			default:
				long += ewd(0xc)
			}
		case 3:
			if t == 0 {
				short += ewd(0)
			}
		case 4:
			short += ewd(1)
		case 5:
			switch w {
			case 0x24, 0x2c, 0x20, 0xd:
				short += ewd(1)
			case 0x22, 0x2d, 0x25, 0x23:
				mid += ewd(1)
			case 0x26:
				long += ewd(0xc)
			case 0x27:
				short += ewd(1)
			}
		}
	}
	if m := g.tc.playerMode[g.tsp(g.tc.cur).u8(tsOwner)]; m == 0 || m == 1 {
		short <<= 1
	}
	s3 := int32(short) * 3 / 2
	if int32(long) < s3 {
		if int32(mid) < s3 {
			return 0
		}
	} else if int32(long) >= int32(mid)*3/2 {
		return 2
	}
	return 1
}

// targetMissileValue is Target_Missile_Value_ (0x29f1b).
func (g *Game) targetMissileValue(a, m, inRange int16) int32 {
	M := g.msl(m)
	rng := int16(0)
	if inRange >= 1 {
		rng = g.rangeToXY(a, M.i16(tmX), M.i16(tmY))
		if rng < 1 {
			rng = 1
		}
	}
	var kills int16
	for k := 0; k < 8; k++ {
		kills += g.expectedWeaponDamage(a, k, m, 1, rng)
	}
	kills /= g.missileHits(M.i16(tmWeapon), int16(g.tsp(M.i16(tmShip)).u8(tsOwner)), M.i16(tmMods))
	if M.i16(tmCount) < kills {
		kills = M.i16(tmCount)
	}
	w := M.i16(tmWeapon)
	var dmg int16
	if g.wtype(w) != 4 {
		dmg = g.wpnI16(int(w), weaponMaxDmg)
	} else {
		own := g.tsp(a).u8(tsOwner)
		switch {
		case w == 0x1c:
			dmg = 100
		case w == 0x1d, w == 0x1e:
			dmg = g.wpnI16(int(g.fighterBombOf(int(own))), weaponMaxDmg)
		case w == 0x1f:
			dmg = g.wpnI16(int(g.fighterBeamOf(int(own))), weaponMaxDmg)
		}
	}
	var sh int16
	if a == 0 {
		sh = g.tsp(0).i16(tsArc)
	} else {
		sh = g.shieldStr(g.tsp(a).u8(tsShield))
	}
	if g.combatShipHasSpecial(a, 7) {
		kills /= 4
	}
	kills *= dmg - sh
	if g.wtype(w) != 4 {
		kills /= 2
	}
	v := int32(kills)
	if v < 1 {
		v = -10000
	}
	if !g.willMissileDoDamage(M.i16(tmTarget), m) {
		v = -10000
	}
	return v
}

// getCurrentShipTarget is Get_Current_Ship_Target_ (0x2acf9): the best
// record some mount of a can hit now; prev and the fleet target count more.
func (g *Game) getCurrentShipTarget(a, prev int16, value *int32) int16 {
	tc := g.tc
	best, bv := int16(-1), int32(-5000)
	start := int16(0)
	if g.doesCombatPlanetHaveDefenses() == 0 {
		start = 1
	}
	A := g.tsp(a)
	for i := start; i < tc.n; i++ {
		T := g.tsp(i)
		if T.u8(tsGone) != 0 || T.u8(tsOwner) == A.u8(tsOwner) || T.u8(tsCloak) == 4 || g.shipInStasis(i) {
			continue
		}
		ok := false
		for k := 0; k < 8 && !ok; k++ {
			w := A.i16(tw(k, twWeapon))
			if w == 0 {
				continue
			}
			arc := getWeaponArc(g.relativeBearing(a, i))
			typ := g.wtype(w)
			ammo := A.i16(tw(k, twAmmo))
			if g.weaponIsOn(a, k) && A.u8(tw(k, twLeft)) != 0 && A.u8(tw(k, twArc))&arc != 0 &&
				((typ != 2 && ammo != 0) || (typ == 2 && ammo == 0)) && g.weaponInRange(k, a, i, 0, 0) {
				ok = true
			}
		}
		if !ok {
			continue
		}
		v := g.targetShipValue(a, i, 1)
		if i == prev && v > 0 {
			if v < 0x1771 {
				v *= 10
			} else {
				v = 60000
			}
		}
		if i == tc.fleetTarget && v > 0 {
			if v < 0x2ee1 {
				v *= 5
			} else {
				v = 60000
			}
		}
		if v > bv {
			bv, best = v, i
		}
	}
	if best == -1 {
		*value = -20000
	} else {
		*value = bv
	}
	return best
}

// getCurrentMissileTarget is Get_Current_Missile_Target_ (0x2afa9).
func (g *Game) getCurrentMissileTarget(a int16, value *int32) int16 {
	bv, best := int32(-5000), int16(-1)
	A := g.tsp(a)
	A.set8(0x3f, 0)
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		if M.i16(tmWeapon) <= 0 || M.i16(tmCount) <= 0 || g.tsp(M.i16(tmShip)).u8(tsOwner) == A.u8(tsOwner) {
			continue
		}
		if !g.willMissileDoDamage(a, m) {
			continue
		}
		if t := g.wtype(M.i16(tmWeapon)); t != 1 && t != 4 {
			continue
		}
		if v := g.targetMissileValue(a, m, 1); v > bv {
			best, bv = m, v
		}
	}
	if best == -1 {
		*value = -20000
	} else {
		*value = bv
	}
	return best
}

// getCurrentTarget is Get_Current_Target_ (0x2b09e): a ship or (+0x3f = 1)
// a missile stack to fire at, by the side's mode.
func (g *Game) getCurrentTarget(a, prev int16) int16 {
	A := g.tsp(a)
	var sv, mv int32
	if m := g.tc.playerMode[A.u8(tsOwner)]; m == 1 || m == 3 {
		s := g.getCurrentShipTarget(a, prev, &sv)
		ms := g.getCurrentMissileTarget(a, &mv)
		if sv < mv/2 {
			A.set8(0x3f, 1)
			return ms
		}
		A.set8(0x3f, 0)
		return s
	}
	ms := g.getCurrentMissileTarget(a, &mv)
	s := g.getCurrentShipTarget(a, -1, &sv)
	if mv < sv/2 {
		A.set8(0x3f, 0)
		return s
	}
	A.set8(0x3f, 1)
	return ms
}

// convertOrdinalToMslType is Convert_Ordinal_To_Msl_Type_ (0x2b191); other
// ordinals return what the caller left in EDX (here: keep).
func convertOrdinalToMslType(o int16, keep int16) int16 {
	switch o {
	case 0:
		return 0xe
	case 1:
		return 0xf
	case 2:
		return 0x10
	case 3:
		return 0x11
	case 4:
		return 0x1d
	case 5:
		return 0x1e
	case 6:
		return 0x1f
	case 7:
		return 0x12
	case 8:
		return 0x14
	case 9:
		return 0x26
	}
	return keep
}

// averageMissileKills is Average_Missile_Kills_ (0x2b7cc): how many of n
// missiles of weapon w mount k of a shoots down with *shots shots.
func (g *Game) averageMissileKills(a int16, k int, n, w int16, shots *int16, m int16) int16 {
	tc := g.tc
	A := g.tsp(a)
	bw := A.i16(tw(k, twWeapon))
	mods := A.i16(tw(k, twMods))
	rng := int16(2)
	if m >= 0 {
		M := g.msl(m)
		rng = g.rangeToXY(a, M.i16(tmX), M.i16(tmY))
	}
	enemy := tc.att
	if int16(A.u8(tsOwner)) == tc.att {
		enemy = tc.def
	}
	have := *shots
	if g.wtype(w) == 2 {
		return 0
	}
	pen := g.exeI16(uint32(0x17d855 + 2*int32(rng)))
	var chance int16
	if (0x28-int32(pen))+int32(g.missileDcv(w, enemy))-int32(A.i16(tsOcv)) < 0x5f {
		chance = int16((0x28 - int32(pen)) + int32(g.missileDcv(w, enemy)) - int32(A.i16(tsOcv)))
	} else {
		chance = 0x5f
	}
	d := (g.wpnI16(int(bw), weaponMinDmg) + g.wpnI16(int(bw), weaponMaxDmg)) * 5
	if mods&0x20 == 0 && g.wpnI16(int(bw), 0x12)&4 == 0 && !g.combatShipHasSpecial(a, 0xf) {
		dp := int32(g.exeI16(uint32(0x17d867 + 2*int32(rng))))
		if dp < 0 {
			dp = -dp
		}
		d -= int16(dp * int32(d) / 100)
	}
	h := g.missileHits(w, 0, 0)
	d = int16(int32(d) * (100 - int32(chance)) / 100)
	switch {
	case h*10 < d:
		d = 1
	case d == 0:
		d = 1000
	default:
		d = int16((int32(h*10) + int32(d) - 1) / int32(d))
	}
	kills := int16(int32(have) / int32(d))
	if n < kills {
		*shots = have - d*n
		return n
	}
	*shots = 0
	return kills
}

// getEffectiveMissileStrength is Get_Effective_Missile_Strength_ (0x2b1fd):
// p's missile and fighter punch after the enemy's point defence; *beam gets
// the enemy beam strength left after shooting at them.
func (g *Game) getEffectiveMissileStrength(p int16, beam *int16) int16 {
	tc := g.tc
	enemy := tc.att
	if p == tc.att {
		enemy = tc.def
	}
	bs := g.bestShipShield(int(enemy))
	shield := int16(100)
	if bs != 7 {
		shield = g.shieldStr(uint8(bs))
	}
	var total int16
	*beam = 0
	var cnt [10]int16
	for i := int16(0); i < tc.n; i++ {
		T := g.tsp(i)
		if T.u8(tsGone) != 0 || T.u8(tsBoarded) != 0 || int16(T.u8(tsOwner)) != p {
			continue
		}
		for k := 0; k < 8; k++ {
			w := T.i16(tw(k, twWeapon))
			typ := g.wtype(w)
			n := int16(T.u8(tw(k, twCount)))
			if w <= 0 || n == 0 || (typ != 1 && typ != 2 && typ != 4) {
				continue
			}
			if !(((typ == 1 || typ == 4) && T.i16(tw(k, twAmmo)) > 0) || typ == 2) {
				continue
			}
			switch {
			case w >= 0xe && w <= 0x11:
				cnt[w-0xe] += n
				total += n
			case w == 0x12:
				cnt[7] += n
			case w == 0x14:
				cnt[8] += n
			case w >= 0x1d && w <= 0x1f:
				cnt[4+w-0x1d] += n * 4
				total += n
			}
		}
	}
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		w := M.i16(tmWeapon)
		if w <= 0 || g.wtype(w) != 4 || int16(g.tsp(M.i16(tmShip)).u8(tsOwner)) != p {
			continue
		}
		if w >= 0x1d && w <= 0x1f {
			cnt[4+w-0x1d] += M.i16(tmCount)
			total += M.i16(tmCount)
		}
	}
	if total < 1 {
		*beam = g.getEffectiveBeamStrength(enemy)
	} else {
		o := int16(0)
		for cnt[o] == 0 && o < 7 {
			o++
		}
		mt := convertOrdinalToMslType(o, 0)
		for i := int16(0); i < tc.n && o < 7; i++ {
			T := g.tsp(i)
			if T.u8(tsGone) != 0 || T.u8(tsBoarded) != 0 || int16(T.u8(tsOwner)) == p {
				continue
			}
			for k := 0; k < 8 && o < 7; k++ {
				w := T.i16(tw(k, twWeapon))
				var shots int16
				if g.weaponCanFireAtMissiles(w, T.i16(tw(k, twMods))) && g.wtype(w) == 0 {
					shots = int16(T.u8(tw(k, twCount)))
					for {
						cnt[o] -= g.averageMissileKills(i, k, cnt[o], mt, &shots, -1)
						for cnt[o] == 0 && o < 7 {
							o++
						}
						mt = convertOrdinalToMslType(o, mt)
						if cnt[o] == 0 || o >= 7 || shots <= 0 {
							break
						}
					}
				}
				if g.wtype(w) == 0 {
					*beam += shots * g.wpnI16(int(w), weaponMaxDmg)
				}
			}
		}
	}
	var v int32
	for o := int16(0); o < 10; o++ {
		var a, b int32
		if shield == 100 {
			a = int32(g.wpnI16(int(convertOrdinalToMslType(o, 0)), weaponMaxDmg)) / 4
			b = int32(cnt[o])
		} else {
			d := int32(g.wpnI16(int(convertOrdinalToMslType(o, 0)), weaponMaxDmg)) - int32(shield)
			if d > 0 {
				a = d
			}
			b = int32(cnt[o])
		}
		v = int32(int16(v)) + a*b
	}
	return int16(v)
}

// getEffectiveBeamStrength is Get_Effective_Beam_Strength_ (0x2ba5e).
func (g *Game) getEffectiveBeamStrength(p int16) int16 {
	var v int32
	for i := int16(0); i < g.tc.n; i++ {
		T := g.tsp(i)
		if T.u8(tsGone) != 0 || T.u8(tsBoarded) != 0 || int16(T.u8(tsOwner)) != p {
			continue
		}
		for k := 0; k < 8; k++ {
			if w := T.i16(tw(k, twWeapon)); g.wtype(w) == 0 || w == 0x13 {
				v += int32(T.u8(tw(k, twCount)))
			}
		}
	}
	return int16(v)
}

// getLongRangeBeamStrength is Get_Long_Range_Beam_Strength_ (0x2be25).
func (g *Game) getLongRangeBeamStrength(p int16) int16 {
	var v int32
	for i := int16(0); i < g.tc.n; i++ {
		T := g.tsp(i)
		if T.u8(tsGone) != 0 || T.u8(tsBoarded) != 0 || int16(T.u8(tsOwner)) != p {
			continue
		}
		for k := 0; k < 8; k++ {
			w := T.i16(tw(k, twWeapon))
			if g.wtype(w) != 0 && w != 0x13 {
				continue
			}
			mods := T.i16(tw(k, twMods))
			noFalloff := g.wpnI16(int(w), 0x12)&4 != 0 || g.combatShipHasSpecial(i, 0xf) || mods&0x20 != 0
			if (noFalloff && !g.combatShipHasSpecial(i, 0x19)) || (!noFalloff && mods&2 == 0 && w != 0x13) {
				continue
			}
			v += int32(T.u8(tw(k, twCount)))
		}
	}
	return int16(v)
}

// enemyDissipatorExists is Enemy_Dissipator_Exists_ (0x42a61).
func (g *Game) enemyDissipatorExists(p int16) bool {
	r := false
	for i := int16(0); i < g.tc.n; i++ {
		T := g.tsp(i)
		if g.combatShipHasSpecial(i, 0x25) && T.u8(tsBoarded) == 0 && T.u8(tsGone) == 0 && int16(T.u8(tsOwner)) != p {
			r = true
		}
	}
	return r
}

// retreatCheck is Retreat_Check_ (0x2bba1): an AI side retreats when the
// enemy would hurt it five times more than it hurts the enemy.
func (g *Game) retreatCheck(p int16) bool {
	tc := g.tc
	if p >= 8 || g.state.Players[p].Objective == save.ObjectiveHuman || tc.aiRetreat != 1 || g.enemyDissipatorExists(p) {
		return false
	}
	var hits [0xd2]int16
	var own, enemy, ownDmg, enemyDmg int32
	for i := int16(0); i < tc.n; i++ {
		if i == 0 && g.doesCombatPlanetHaveDefenses() == 0 {
			continue
		}
		T := g.tsp(i)
		if T.u8(tsGone) != 0 || T.u8(tsBoarded) != 0 {
			continue
		}
		h := int32(g.getTotalHits(i))
		hits[i] = int16(h)
		if i == 0 {
			h *= 3
		}
		if int16(T.u8(tsOwner)) == p {
			own += int32(int16(h))
		} else {
			enemy += int32(int16(h))
		}
	}
	for i := int16(0); i < tc.n; i++ {
		T := g.tsp(i)
		if T.u8(tsGone) != 0 || T.u8(tsBoarded) != 0 {
			continue
		}
		var best int32
		bt := int16(-1)
		for j := int16(0); j < tc.n; j++ {
			if j == 0 && g.doesCombatPlanetHaveDefenses() == 0 {
				continue
			}
			J := g.tsp(j)
			if J.u8(tsGone) != 0 || J.u8(tsBoarded) != 0 || J.u8(tsOwner) == T.u8(tsOwner) {
				continue
			}
			var d int32
			for k := 0; k < 8; k++ {
				d += int32(g.expectedWeaponDamage(i, k, j, 0, 0))
			}
			if int32(hits[j]) < int32(int16(d)) {
				d = int32(hits[j])
			}
			if int32(int16(best)) < int32(int16(d)) {
				best, bt = d, j
			}
		}
		if int16(best) > 0 {
			hits[bt] -= int16(best)
			if int16(T.u8(tsOwner)) == p {
				ownDmg += int32(int16(best))
			} else {
				enemyDmg += int32(int16(best))
			}
		}
	}
	var ratio, threat uint32
	if enemy == 0 {
		ratio = uint32(0xb2d05e00) // -0x4d2fa200
	} else {
		ratio = uint32(ownDmg * 100 / enemy)
	}
	if own == 0 {
		threat = 3000000000
	} else {
		threat = uint32(enemyDmg * 100 / own)
	}
	return ratio*5 < threat
}

// getPlayerMode is Get_Player_Mode_ (0x2c43a): -1 retreat, else the
// stance from missile against beam strengths.
func (g *Game) getPlayerMode(p int16) int8 {
	tc := g.tc
	enemy := tc.att
	if p == tc.att {
		enemy = tc.def
	}
	if g.retreatCheck(p) {
		return -1
	}
	var ownLeft, enemyLeft int16
	ownMsl := g.getEffectiveMissileStrength(p, &ownLeft)
	ownLong := g.getLongRangeBeamStrength(p)
	enemyMsl := g.getEffectiveMissileStrength(enemy, &enemyLeft)
	ownBeam := g.getEffectiveBeamStrength(p)
	enemyBeam := g.getEffectiveBeamStrength(enemy)
	enemyLong := g.getLongRangeBeamStrength(enemy)
	if int32(enemyLong)+int32(enemyMsl) < int32(ownMsl)+int32(ownLong) && tc.playerMode[p] != 1 && tc.playerMode[p] != 0 {
		if int32(ownLeft) <= int32(enemyBeam)*3/4 {
			return 3
		}
		if p != tc.att || ownLeft < 0x33 {
			return 2
		}
	} else if int32(ownBeam) <= int32(enemyBeam)*3/2 {
		return 1
	}
	return 0
}

// isShipOutOfAmmo is Is_Ship_Out_Of_Ammo_ (0x294df).
func (g *Game) isShipOutOfAmmo(i int16) bool {
	if g.combatShipHasSpecial(i, 0x25) || g.combatShipHasSpecial(i, 0x26) {
		return false
	}
	out := true
	T := g.tsp(i)
	for k := 0; k < 8 && out; k++ {
		w := T.i16(tw(k, twWeapon))
		if w == 0 || T.u8(tw(k, twCount)) == 0 {
			continue
		}
		ammo := T.i16(tw(k, twAmmo))
		switch g.wtype(w) {
		case 1:
			if ammo > 0 {
				out = false
			}
		case 3:
			if ammo > 0 && int16(T.u8(tsOwner)) == g.tc.att {
				out = false
			}
		case 4:
			if ammo > 0 {
				out = false
			} else {
				out = !g.checkForLaunchedFighters(i)
			}
		case 5:
			if w == 0x21 {
				if ammo > 0 {
					out = false
				}
				break
			}
			out = false
		case 0, 2:
			out = false
		}
	}
	return out
}

// checkForLaunchedFighters is Check_For_Launched_Fighters_ (0x29480).
func (g *Game) checkForLaunchedFighters(i int16) bool {
	for m := int16(0); m < nMissiles; m++ {
		if M := g.msl(m); M.i16(tmShip) == i && g.wtype(M.i16(tmWeapon)) == 4 {
			return true
		}
	}
	return false
}

// isShipTargetedByMissile is Is_Ship_Targeted_By_Missile_ (0x295d0).
func (g *Game) isShipTargetedByMissile(i int16) bool {
	for m := int16(0); m < nMissiles; m++ {
		if M := g.msl(m); M.i16(tmTarget) == i && M[tmAtMis] == 0 && M.i16(tmShip) != i {
			return true
		}
	}
	return false
}

// doesShipHaveMissilesIncoming is Does_Ship_Have_Missiles_Incoming_ (0x27f26).
func (g *Game) doesShipHaveMissilesIncoming(i int16) bool {
	r := false
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		if M.i16(tmTarget) == i && M.i16(tmWeapon) > 0 && M[tmAtMis] == 0 && g.willMissileDoDamage(i, m) &&
			M.i16(tmOwner) != int16(g.tsp(i).u8(tsOwner)) {
			r = true
		}
	}
	return r
}

// removeStasisBeam is Remove_Stasis_Beam_ (0x3afb3).
func (g *Game) removeStasisBeam(i int16) {
	T := g.tsp(i)
	for k := 0; k < 8; k++ {
		T.set8(tw(k, twLeft), 0)
	}
	T.set8(tsStasis, 0xff)
}

// releaseSmallestShipFromStasis is Release_Smallest_Ship_From_Stasis_ (0x285c6).
func (g *Game) releaseSmallestShipFromStasis(p int16) {
	best, size := int16(-1), uint8(100)
	for i := int16(1); i < g.tc.n; i++ {
		T := g.tsp(i)
		if T.u8(tsGone) == 0 && T.u8(tsBoarded) == 0 && int16(T.u8(tsOwner)) == p && g.shipInStasis(i) && T.u8(tsSize) < size {
			best, size = i, T.u8(tsSize)
		}
	}
	if best != -1 {
		g.removeStasisBeam(best)
	}
}

// aiStasisCheck is Ai_Stasis_Check_ (0x2864a): enemies not in stasis; when
// none is left the smallest is let go.
func (g *Game) aiStasisCheck(p int16) int16 {
	tc := g.tc
	enemy := tc.att
	if p == tc.att {
		enemy = tc.def
	}
	var n, held int32
	start := int16(0)
	if g.doesCombatPlanetHaveDefenses() < 1 {
		start = 1
	}
	for i := start; i < tc.n; i++ {
		T := g.tsp(i)
		if T.u8(tsGone) == 0 && T.u8(tsBoarded) == 0 && int16(T.u8(tsOwner)) != p {
			n++
			if g.shipInStasis(i) {
				held++
			}
		}
	}
	if int32(int16(n))-int32(int16(held)) < 1 {
		g.releaseSmallestShipFromStasis(enemy)
	}
	return int16(n - held)
}

// crewBoardingCombatBonus is Crew_Boarding_Combat_Bonus_ (0x35cad).
func (g *Game) crewBoardingCombatBonus(i int16) int16 {
	return g.exeI16(uint32(0x17d174 + 8*int32(g.tsp(i).u8(tsCrew))))
}

// fleetSkillBonus is Get_Fleet_Commando_Bonus_ (0x35fda) and
// Get_Fleet_Security_Bonus_ (0x360aa): the best officer aboard p's ships.
func (g *Game) fleetSkillBonus(p int16, security bool) int16 {
	s := &g.state
	var best int16
	for i := int16(0); i < g.tc.n; i++ {
		T := g.tsp(i)
		o := T.i16(tsOfficer)
		if int16(T.u8(tsOwner)) != p || T.u8(tsBoarded) != 0 || T.u8(tsGone) != 0 || o <= -1 {
			continue
		}
		var v int16
		L := &s.Leaders[o]
		lvl := s.ownedOfficerLevel(int(o)) + 1
		if security {
			b := byte(L.SpecialSkills >> 8)
			switch {
			case b&0x10 != 0:
				v = lvl * 2
			case b&0x20 != 0:
				v = lvl * 3
			}
		} else {
			b := byte(L.CommonSkills >> 0)
			switch {
			case b&4 != 0:
				v = lvl * 2
			case b&8 != 0:
				v = lvl * 3
			}
		}
		if v > best {
			best = v
		}
	}
	return best
}

// getBoardingInfo is Get_Boarding_Info_ (0xec767): marines of p against e.
func (g *Game) getBoardingInfo(p, n, officer, e, n2, officer2 int16) (A, B groundForce) {
	A = groundForce{player: p, colony: -1, enemy: e}
	A.count[1] = n
	B = groundForce{player: e, colony: -1, enemy: p}
	B.count[1] = n2
	g.computePlayerGroundCombatBonuses(int(p), &A.bonus, false)
	A.bonus[0x10], A.bonus[0x11] = uint8(officer), uint8(uint16(officer)>>8)
	g.computePlayerGroundCombatBonuses(int(e), &B.bonus, false)
	B.bonus[0x10], B.bonus[0x11] = uint8(officer2), uint8(uint16(officer2)>>8)
	computeGroundCombatInfo(&A, &A.bonus, &B.bonus)
	computeGroundCombatInfo(&B, &B.bonus, &A.bonus)
	return
}

// boardingActionType is Boarding_Action_Type_ (0x2c129): 0 none, 1 raid,
// 2 capture.
func (g *Game) boardingActionType(a, t int16) int16 {
	tc := g.tc
	own := int16(g.tsp(a).u8(tsOwner))
	if t == -1 {
		return 0
	}
	rng := g.rangeToShip(a, t)
	reach := int16(1)
	if g.combatShipHasSpecial(a, 0x23) {
		reach = 4
	}
	T := g.tsp(t)
	ok := false
	if T.u8(tsOwner) < 9 && g.tsp(a).u8(tsOwner) < 9 && g.combatShipIsFunctional(t) && T.u8(tsCloak) != 4 &&
		!g.alreadyBoarded(a) && t != 0 && T.u8(tsOwner) != g.tsp(tc.cur).u8(tsOwner) && rng <= reach &&
		(reach == 4 || g.calcCurrentSpeed(t) < 1) && g.tsp(tc.cur).u8(tsMarines) != 0 {
		ok = true
		if reach == 4 && !(rng == 1 && T.i16(tsSpeed) < 1) {
			if T.i16(tsArc+2*int(facingShield(g.relativeBearing(t, a)))) >= 1 || g.combatShipHasSpecial(t, 0xd) {
				ok = false
			}
		}
	}
	if !ok {
		return 0
	}
	enemy := tc.att
	if own == tc.att {
		enemy = tc.def
	}
	if tc.att >= 9 || tc.def >= 9 || g.alreadyBoarded(a) {
		return 0
	}
	if own == 8 {
		return 1
	}
	A, B := g.getBoardingInfo(own, 1, 0, enemy, 1, 0)
	mine := int32(int16(A.strength[1] + g.crewBoardingCombatBonus(a) + g.fleetSkillBonus(own, false)))
	theirs := int32(int16(g.crewBoardingCombatBonus(t) + g.fleetSkillBonus(enemy, false) + g.fleetSkillBonus(enemy, true) +
		b2i(g.combatShipHasSpecial(t, 0x1d))*0x14 + B.strength[1]))
	if mine <= theirs-0x14 {
		return 0
	}
	if g.state.Players[own].Picks[save.PickTelepathic] == 0 {
		if g.tsp(a).u8(tsMarines) < T.u8(tsMarines) || mine <= theirs-10 {
			return 1
		}
	}
	return 2
}

// evaluatePossibleMove is Evaluate_Possible_Move_ (0x2a77d): squares at the
// wanted distance for the ship's class (3, 15, 50) collect, up to 20 ties.
func evaluatePossibleMove(dist, x, y int16, class uint8, n *int16, xs, ys *[20]int16, best int16) int16 {
	var want int16
	switch {
	case class == 0:
		want = 3
	case class < 2:
		want = 0xf
	case class == 2:
		want = 0x32
	}
	d := dist - want
	if d < 0 {
		return best
	}
	switch {
	case d < best:
		*n = 0
		xs[*n], ys[*n] = x, y
		best = d
	case d != best || *n > 0x13:
		return best
	default:
		xs[*n], ys[*n] = x, y
	}
	*n++
	return best
}

// autoMoveShip is Auto_Move_Ship_ (0x2a9a5): move to a random square at the
// distance the class wants from target t, then turn towards it. True when
// the ship moved.
func (g *Game) autoMoveShip(a, t int16) bool {
	tc := g.tc
	C := g.tsp(tc.cur)
	moved := false
	if tc.playerMode[C.u8(tsOwner)] == -1 {
		C.set8(0xb1, 2)
	}
	stay := false
	if C.u8(0x3f) == 0 {
		if m := tc.playerMode[C.u8(tsOwner)]; m == 3 || m == 2 {
			stay = !(tc.turn == 1 && g.wtype(C.i16(tw(0, twWeapon))) == 1)
		}
	}
	tc.autoMslOff = 0
	target := func() (int16, int16) {
		if g.tsp(tc.cur).u8(0x3f) == 1 {
			M := g.msl(t)
			return M.i16(tmX) / 0x14, M.i16(tmY) / 0x14
		}
		T := g.tsp(t)
		return int16(T.u8(tsX)), int16(T.u8(tsY))
	}
	if !stay {
		tx, ty := target()
		d := rangeXY(int16(C.u8(tsX)), int16(C.u8(tsY)), tx, ty)
		var best int16
		switch cl := C.u8(0xb1); {
		case cl == 0:
			best = d - 3
		case cl == 1:
			best = d - 0xf
		case cl == 2:
			best = d - 0x32
		}
		var n int16
		var xs, ys [20]int16
		if best > 0 {
			for x := int16(0); x < gridW; x++ {
				for y := int16(0); y < gridH; y++ {
					if tc.grid[x][y] != gridLegal {
						continue
					}
					tx, ty := target()
					best = evaluatePossibleMove(rangeXY(x, y, tx, ty), x, y, g.tsp(a).u8(0xb1), &n, &xs, &ys, best)
				}
			}
		}
		if n > 0 {
			k := int16(g.state.Random.Random(uint32(n))) - 1
			g.moveCost(tc.cur, xs[k], ys[k])
			if used := g.moveShip(tc.cur, xs[k], ys[k]); used > 0 {
				moved = true
				g.tsp(tc.cur).add16(0x3d, -used)
			}
		}
	}
	tx, ty := target()
	g.turnToFace(tc.cur, tx, ty)
	tc.autoMslOff = 0
	return moved
}

// doAutoShipTurn is Do_Auto_Ship_Turn_ (0x29837): the computer moves and
// fires the current ship.
func (g *Game) doAutoShipTurn() {
	tc := g.tc
	cur := tc.cur
	if g.shipInStasis(cur) {
		return
	}
	if tc.scr != nil && !g.shipCompletelyOnScreen(cur) {
		C := g.tsp(cur)
		g.must2(g.snapCenterCombatScreen(int16(C.u8(tsX)), int16(C.u8(tsY))))
	}
	C := g.tsp(cur)
	if g.isShipOutOfAmmo(cur) || tc.playerMode[C.u8(tsOwner)] == -1 {
		if C.u8(tsDrive) != 0 && !g.enemyDissipatorExists(int16(C.u8(tsOwner))) && C.u8(tsOwner) < 8 && tc.aiRetreat == 1 {
			C.set8(0x4b, 1)
			C.set8(0x47, 0xff)
		}
	}
	g.aiStasisCheck(int16(C.u8(tsOwner)))
	g.assignCombatGrids(tc.cur)
	again := false
	for {
		fired := false
		t := g.chooseTarget(tc.cur)
		if g.autoPressed() {
			g.toggleAutoCombatFlags()
		}
		if t == -1 {
			again = false
		} else {
			g.tsp(tc.cur).set8(0xb1, g.combatShipClass(tc.cur, t))
			fired = g.autoMoveShip(tc.cur, t)
			if fired {
				again = false
			} else {
				again = !again
				tc.mslWarning = 2
			}
		}
		if g.autoPressed() {
			g.toggleAutoCombatFlags()
		}
		C = g.tsp(tc.cur)
		if C.u8(tsGone) == 0 && C.u8(tsBoarded) == 0 {
			for k := 0; k < 8; k++ {
				C.set8(tw(k, twOn), 0xff)
			}
			prev := int16(-1)
			for k := 0; ; {
				tgt := g.autoMountTarget(k, &prev)
				if tgt == -1 {
					C.set8(tw(k, twOn), 0xff)
					k++
				} else if C.u8(0x3f) == 1 {
					M := g.msl(tgt)
					fired = g.fireShip(tc.cur, tgt, M.i16(tmX), M.i16(tmY), 1)
				} else {
					T := g.tsp(tgt)
					fired = g.fireShip(tc.cur, tgt, int16(T.u8(tsX))*0x14, int16(T.u8(tsY))*0x14, 0)
					if C.u8(tsOwner) == 0xb {
						g.crystalControl(tgt)
					}
				}
				g.combatFrame()
				g.aiStasisCheck(int16(C.u8(tsOwner)))
				if k > 7 || g.isCombatShipDead(tc.cur) != 0 {
					break
				}
			}
			for k := 0; k < 8; k++ {
				C.set8(tw(k, twOn), 1)
			}
		}
		if !(g.tsp(tc.cur).i16(0x3d) > 0 && (fired || again)) {
			break
		}
	}
	if g.combatShipIsFunctional(tc.cur) {
		for i := int16(0); i < tc.n; i++ {
			if bt := g.boardingActionType(tc.cur, i); bt != 0 {
				g.resolveAiBoarding(tc.cur, i, bt)
			}
		}
	}
	if g.autoPressed() {
		g.toggleAutoCombatFlags()
	}
	if g.aiSelfDestructCheck(tc.cur) {
		C = g.tsp(tc.cur)
		drive := C.u8(tsDrive)
		C.set16(tsDriveCur, 0)
		C.set8(tsDrive, 1)
		dead := g.isCombatShipDead(tc.cur)
		g.destroyShip(tc.cur, int16(C.u8(tsX))*0x14, int16(C.u8(tsY))*0x14, dead)
		C.set8(tsDrive, drive)
		tc.order[tc.curPtr] = -1
	}
}

// autoPressed is Get_Input == _auto_button in Do_Auto_Ship_Turn_: with the
// screen the input is read (and anything else dropped).
func (g *Game) autoPressed() bool {
	tc := g.tc
	if tc.scr == nil {
		return tc.ui.AutoPressed(g)
	}
	in, err := g.e.GetInput()
	if err != nil {
		panic(err)
	}
	return in == tc.scr.autoField
}

// combatFrame is "Set_Page_Off; Draw_Main_Combat_Screen_; _TOGGLE_PAGES_".
func (g *Game) combatFrame() {
	if g.tc.scr == nil {
		return
	}
	g.e.V.SetPageOff()
	g.must2(g.drawMainCombatScreen())
	g.must2(g.e.TogglePages())
}

// must2 stops the game on an error the logic has no way to pass on (the
// engine's drawing deep in the combat logic).
func (g *Game) must2(err error) {
	if err != nil {
		panic(err)
	}
}

// autoMountTarget is the per-mount part of Do_Auto_Ship_Turn_'s fire loop:
// switch mount k on and pick its target, or -1 (the mount is then skipped).
func (g *Game) autoMountTarget(k int, prev *int16) int16 {
	tc := g.tc
	C := g.tsp(tc.cur)
	w := C.i16(tw(k, twWeapon))
	if w == 0xd && g.doesShipHaveMissilesIncoming(tc.cur) {
		return -1
	}
	if (w == 0x24 || w == 0xd) && !g.safeToFireSphereWeapon(tc.cur) {
		return -1
	}
	if w == 0x20 && g.aiStasisCheck(int16(C.u8(tsOwner))) < 2 {
		return -1
	}
	C.set8(tw(k, twOn), 1)
	t := g.getCurrentTarget(tc.cur, *prev)
	if C.u8(0x3f) == 0 {
		if t != -1 {
			*prev = t
		}
		if bt := g.boardingActionType(tc.cur, t); bt != 0 {
			g.resolveAiBoarding(tc.cur, t, bt)
			t = g.getCurrentTarget(tc.cur, t)
		}
	}
	if t != -1 && g.wtype(C.i16(tw(k, twWeapon))) == 0 && C.i16(tw(k, twMods))&2 == 0 {
		if C.i16(tw(k, twMods))&4 != 0 && g.isShipTargetedByMissile(tc.cur) {
			return t
		}
		// Get_Beam_Weapon_Modifiers_ runs here for a to-hit figure the
		// routine never uses.
	}
	return t
}

// toggleAutoCombatFlags is Toggle_Auto_Combat_Flags_ (0x29616) outside
// network play: Auto is switched off for the side of the current ship.
func (g *Game) toggleAutoCombatFlags() {
	s := &g.state
	tc := g.tc
	o := int16(g.tsp(tc.cur).u8(tsOwner))
	human := o == int16(s.PlayerNum) || g.gameType == 1
	// In a lockstep battle msg 0x3ea {0 attacker, 1 defender} tells the other
	// machine whose AUTO went off (0x2968d..0x29786).
	send := func(to, side int16) {
		if tc.netFlag != 0 {
			g.moxSendMessage(g.netIDOf(int(to)), 0x3ea, binary.LittleEndian.AppendUint16(nil, uint16(side)))
		}
	}
	switch {
	case o == tc.att && human:
		tc.autoAtt, tc.autoCombat, tc.superFast = 0, 0, 0
		send(tc.def, 0)
	case o == tc.att:
		tc.autoDef, tc.superFast = 0, 0
		send(o, 1)
	case !human:
		tc.autoAtt, tc.superFast = 0, 0
		send(o, 0)
	default:
		tc.autoCombat, tc.autoDef, tc.superFast = 0, 0, 0
		send(tc.att, 1)
	}
}

// resolveAiBoarding is Resolve_Ai_Boarding_ (0x2bf73): half the marines
// (at least one) board t to raid (kind 1) or capture it (kind 2).
func (g *Game) resolveAiBoarding(a, t, kind int16) {
	tc := g.tc
	if kind == 0 {
		return
	}
	rng := g.rangeToShip(a, t)
	reach := int16(1)
	if g.combatShipHasSpecial(a, 0x23) {
		reach = 4
	}
	A := g.tsp(a)
	n := int16(A.u8(tsMarines) / 2)
	if n == 0 {
		n = 1
	}
	A.set8(tsMarines, A.u8(tsMarines)-uint8(n))
	if reach == 4 && (rng != 1 || g.tsp(t).i16(tsSpeed) > 0) && g.combatShipHasSpecial(t, 7) {
		n /= 3
		if n < 1 {
			kind = 0
		}
	}
	if kind == 1 || kind == 2 {
		g.tacSend(tacBoard, t, n, kind)
	}
	switch kind {
	case 1:
		A[0x48] |= 2
		g.resolveRaid(a, t, &n)
		A.set8(tsMarines, A.u8(tsMarines)+uint8(n))
	case 2:
		A[0x48] |= 2
		g.tsp(t)[0x48] |= 0x10
		T := g.tsp(t)
		g.resolveCapture(a, t, n, int16(T.u8(tsX))*0x14, int16(T.u8(tsY))*0x14)
	}
	tc.board = 0
	g.tacWaitIf()
}

// resolveCapture is Resolve_Capture_ (0x37da8): n marines of a fight t's
// crew; winning captures the ship. A message box tells the human (UI).
func (g *Game) resolveCapture(a, t, n, x, y int16) {
	A, T := g.tsp(a), g.tsp(t)
	if A.u8(tsOwner) == T.u8(tsOwner) {
		return
	}
	def := int16(T.u8(tsMarines))
	n0, def0 := n, def
	ao, to := int16(A.u8(tsOwner)), int16(T.u8(tsOwner))
	cb := g.fleetSkillBonus(ao, false)
	tb := g.fleetSkillBonus(to, false) + g.fleetSkillBonus(to, true)
	F, D := g.getBoardingInfo(ao, n, cb, to, def, tb+b2i(g.combatShipHasSpecial(t, 0x1d))*0x14)
	for def >= 1 && n >= 1 {
		g.groundCombatRound(&F, &D)
		if F.lost > -1 {
			n--
		}
		if D.lost > -1 {
			def--
		}
	}
	r := boardCapture{a: a, t: t, n0: n0, def0: def0, n: n, def: def, fs: F.strength[1], ds: D.strength[1]}
	if ao >= 8 || g.state.Players[ao].Objective != save.ObjectiveHuman {
		r.fs -= int16(int8(F.bonus[0xf]))
	}
	if to >= 8 || g.state.Players[to].Objective != save.ObjectiveHuman {
		r.ds -= int16(int8(D.bonus[0xf]))
	}
	if n < 1 {
		T.set8(tsMarines, uint8(def))
		r.result = 0x33
	} else {
		T.set8(tsMarines, uint8(n)-uint8(def))
		g.captureShip(t, x, y)
		r.result = 0x32
		if T.u8(tsGone) != 0 {
			r.result = 0x31
		}
	}
	if g.tc.scr != nil {
		g.captureReport(&r)
	}
}

// crystalControl is Crystal_Control_ (0x29790): the crystal takes over a
// ship it shot that has no marines; the screen says so.
func (g *Game) crystalControl(t int16) {
	if t == 0 {
		return
	}
	T := g.tsp(t)
	if T.u8(tsGone) != 0 || T.u8(tsMarines) != 0 {
		return
	}
	g.captureShip(t, int16(T.u8(tsX))*0x14, int16(T.u8(tsY))*0x14)
	T.set8(tsBoarded, 0)
	if g.e == nil || g.tc.scr == nil {
		return
	}
	msg, err := g.kenText(0x4c)
	g.must2(err)
	g.e.RefreshPalette()
	g.must2(g.messageBox(msg + save.CString(T[:])))
	g.must2(g.refreshCombatScreenFull())
}

// captureShip is Capture_Ship_ (0x38312): t changes sides (or blows up with
// special 0x18 six times in ten); its officer dies.
func (g *Game) captureShip(t, x, y int16) {
	s := &g.state
	tc := g.tc
	T := g.tsp(t)
	if o := T.i16(tsOfficer); o > -1 {
		g.killOfficer(int(o))
		T.set16(tsOfficer, -1)
		if id := T.i16(tsID); id != -1 {
			s.Ships[id].Officer = -1
		}
	}
	T[0x48] &^= 0x10
	if g.combatShipHasSpecial(t, 0x18) && s.Random.Random(10) < 6 {
		g.destroyShip(t, x, y, 4)
		return
	}
	enemy := tc.att
	if int16(T.u8(tsOwner)) == tc.att {
		enemy = tc.def
	}
	T.set8(tsOwner, uint8(enemy))
	g.retargetMissilesAimedAt(t)
	g.removeContinuingEffectSpecials(t)
	if enemy > 7 || s.Players[enemy].Picks[save.PickTelepathic] == 0 {
		T.set8(tsBoarded, 1)
	}
	T.set8(0x4b, 0)
	T.set8(0x42, 0xff)
	k := int16(0)
	for tc.order[k] != -10 {
		k++
	}
	for ; k > -1; k-- {
		tc.order[k+1] = tc.order[k]
	}
	tc.order[0] = -1
	tc.curPtr++
}

// aiSelfDestructCheck is Ai_Self_Destruct_Check_ (0x28168): a crippled ship
// with enemies close may blow itself up.
func (g *Game) aiSelfDestructCheck(a int16) bool {
	s := &g.state
	tc := g.tc
	A := g.tsp(a)
	if A.u8(tsOwner) >= 9 || a == 0 || A.u8(tsDrive) == 0 {
		return false
	}
	enemy := tc.att
	if int16(A.u8(tsOwner)) == tc.att {
		enemy = tc.def
	}
	var ownNear, enemyNear, ownFighters, enemyFighters, ownMis, enemyMis int32
	var marines, mine, theirs int32
	if enemy < int16(s.NumPlayers) {
		own := int16(A.u8(tsOwner))
		cb := g.fleetSkillBonus(enemy, false)
		def := int32(g.fleetSkillBonus(own, false)) + int32(g.fleetSkillBonus(own, true))
		def += int32(b2i(g.combatShipHasSpecial(a, 0x1d))) * 0x14
		def += int32(g.crewBoardingCombatBonus(a))
		E, O := g.getBoardingInfo(enemy, 5, cb, own, 5, int16(def))
		theirs = int32(E.strength[1])
		mine = int32(O.strength[1])
	}
	for i := int16(1); i < tc.n; i++ {
		T := g.tsp(i)
		if T.u8(tsGone) != 0 || i == a || g.shipInStasis(i) {
			continue
		}
		r := g.rangeToShip(a, i)
		theirs += int32(g.crewBoardingCombatBonus(i))
		if T.u8(tsOwner) == A.u8(tsOwner) {
			switch {
			case r < 2:
				ownNear += 2
			case r == 2:
				ownNear++
			}
			continue
		}
		pods := g.combatShipHasSpecial(i, 0x23)
		switch {
		case r < 2, r == 2:
			if r < 2 {
				enemyNear += 2
			} else {
				enemyNear++
			}
			if !pods && A.i16(tsSpeed) != 0 {
				continue
			}
		case r > 6 || !pods:
			continue
		}
		marines += int32(T.u8(tsMarines) / 2)
	}
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		if M.i16(tmWeapon) <= 0 || g.wtype(M.i16(tmWeapon)) == 2 {
			continue
		}
		if g.rangeToXY(a, M.i16(tmX), M.i16(tmY)) >= 3 {
			continue
		}
		fighter := g.wtype(M.i16(tmWeapon)) == 4
		if M.i16(tmOwner) == int16(A.u8(tsOwner)) {
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
	v := int16(enemyMis) + int16(enemyFighters)*5 - int16(ownFighters)*5 - int16(ownMis) + int16(enemyNear)*0x14 - int16(ownNear)*10
	if marines > 0 && enemy < int16(s.NumPlayers) {
		d := int16(theirs) - int16(mine)
		crew := int16(A.u8(tsMarines))
		switch {
		case d >= 0x33:
			marines *= 10
		case d >= 0x28:
			marines *= 7
		case d >= 0x1e:
			marines <<= 2
		case d >= 0x14:
			marines *= 3
		case d >= 10:
			marines *= 2
		case d < -0x32:
			crew *= 10
		case d < -0x27:
			crew *= 7
		case d < -0x1d:
			crew <<= 2
		case d < -0x13:
			crew *= 3
		case d < -9:
			crew *= 2
		}
		if int32(crew) < marines {
			k := int16(5)
			if s.Players[enemy].Picks[25] != 0 {
				k = 10
			}
			v += (int16(marines) - crew) * k
		}
	}
	return v > 0 && int16(s.Random.Random(100)) < v && A.i16(tsSpeed) == 0
}

// resolveRaid is Resolve_Raid_ (0x37308): each of *n raiders rolls against
// t's crew; a clear win wrecks two systems (50 damage each), the losers lose
// men. The report goes to a text box (UI).
func (g *Game) resolveRaid(a, t int16, n *int16) {
	s := &g.state
	A, T := g.tsp(a), g.tsp(t)
	if A.u8(tsOwner) == T.u8(tsOwner) {
		return
	}
	var hits [0x13]int16
	crew := int16(T.u8(tsMarines))
	raiders := *n
	ao, to := int16(A.u8(tsOwner)), int16(T.u8(tsOwner))
	ab := g.fleetSkillBonus(ao, false) + g.crewBoardingCombatBonus(a)
	tb := g.fleetSkillBonus(to, false) + g.fleetSkillBonus(to, true) + b2i(g.combatShipHasSpecial(t, 0x1d))*0x14 + g.crewBoardingCombatBonus(t)
	F, D := g.getBoardingInfo(ao, *n, ab, to, crew, tb)
	for j := int16(0); j < *n; j++ {
		r1 := int16(s.Random.Random(100))
		r2 := int16(s.Random.Random(100))
		d := F.strength[1] + r1 - (r2 + D.strength[1])
		if d > 0x14 {
			for k := 0; k < 2; k++ {
				kind, w := g.selectRaidInternal(t)
				if kind == -1 {
					continue
				}
				switch kind {
				case 2:
					hits[1]++
				case 3:
					hits[w+3]++
				case 5:
					hits[0]++
				case 8:
					hits[2]++
				case 9:
					hits[w+0xb]++
				}
				v := int16(0x32)
				g.applyInternalDamage(t, &v, kind, w, 0)
			}
		}
		if d > 0x32 && crew > 0 {
			crew--
		}
		if d < -0x32 {
			raiders--
		}
	}
	var total int16
	for _, h := range hits {
		total += h
	}
	if total >= 1 {
		if dead := g.isCombatShipDead(t); dead > 0 {
			g.destroyShip(t, int16(T.u8(tsX))*0x14, int16(T.u8(tsY))*0x14, dead)
		}
	}
	if g.tc.scr != nil {
		g.raidReport(a, t, &hits, total, *n-raiders, int16(T.u8(tsMarines))-crew, F.strength[1], D.strength[1])
	}
	T.set8(tsMarines, uint8(crew))
	*n = raiders
	if g.tc.scr != nil {
		g.must2(g.refreshCombatScreenFull())
	}
}

// selectRaidInternal is Select_Raid_Internal_ (0x384a8): a system weighted
// towards the small ones; its first Random(100) is unused.
func (g *Game) selectRaidInternal(t int16) (kind, k int16) {
	s := &g.state
	s.Random.Random(100)
	T := g.tsp(t)
	tot := int32(g.totalInternalSpace(t))
	var w [19]uint16
	w[0] = uint16(T.i16(tsDriveCur) / 2)
	w[1] = uint16(T.i16(tsCompCur))
	for j := 0; j < 8; j++ {
		w[2+j] = uint16(T.i16(tsCur + 4 + 2*j))
		w[10+j] = uint16(T.i16(tsCur + 0x24 + 2*j))
	}
	w[18] = uint16(T.i16(tsShieldCur))
	for j := range w {
		if w[j] != 0 {
			w[j] = uint16(100 - int16(int64(uint32(w[j])*100)/int64(tot)))
		}
	}
	c := getWeightedChoiceShort(&s.Random, w[:])
	switch {
	case c == 0:
		return 5, 0
	case c == 1:
		return 8, 0
	case c >= 2 && c < 10:
		return 3, c - 2
	case c >= 10 && c < 18:
		return 9, c - 10
	case c == 0x12:
		return 2, 0
	}
	return -1, 0
}
