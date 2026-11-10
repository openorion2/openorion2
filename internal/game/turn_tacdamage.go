package game

// Damage to ships, their destruction and the end of the battle (CMBTFIRE.C,
// CMBTFIR2.C, COMBINIT.C). Spec: docs/combat/tactical.md.

// applyShipDamage is Apply_Ship_Damage_ (0x39985): d damage on arc of
// record t through its shield (absorbed into *abs), then into its armour,
// structure and systems. flags are the weapon's (0x12): 1 no internal
// damage, 8 armour-piercing halved by monsters, 0x10 kills crew, 0x20 half
// straight to the structure. double doubles, allInternal (and mods 8)
// hits the systems first, drive forces drive hits.
func (g *Game) applyShipDamage(t int16, d, abs *int16, arc, mods, flags, double, allInternal, drive int16) int16 {
	tc := g.tc
	*abs = 0
	var ret, back int16
	if t == 0 {
		return g.applyDamageToPlanet(d, mods, flags)
	}
	T := g.tsp(t)
	if g.combatShipHasSpecial(t, 0xa) {
		v := int32(*d) / 4
		if v <= 1 {
			v = 1
		}
		*d -= int16(v)
		T.add16(tsAbsorbed, int16(v))
	}
	if g.combatShipHasSpecial(t, 7) {
		*d /= 4
	}
	shield := T.i16(tsArc + 2*int(arc))
	if mods&0x40 != 0 && !g.combatShipHasSpecial(t, 0xd) {
		shield = 0
	}
	if T.u8(tsShield) > 0 && T.i16(tsShieldCur) > 0 {
		if g.combatShipHasSpecial(t, 0xd) && mods&0x40 != 0 {
			mods ^= 0x40
		}
		if mods&0x40 == 0 && (tc.nebula == 0 || g.combatShipHasSpecial(t, 0xd)) {
			*d -= g.shieldStr(T.u8(tsShield))
			if g.combatShipHasSpecial(t, 0xd) {
				*d -= 3
			}
		}
		if *d < 0 {
			*d = 0
		}
		if *d > 0 && (tc.nebula == 0 || g.combatShipHasSpecial(t, 0xd)) {
			ret = 1
		}
		if shield > *d {
			T.add16(tsArc+2*int(arc), -*d)
			*abs = *d
			*d = 0
		} else {
			*d -= shield
			*abs = shield
			T.add16(tsArc+2*int(arc), -shield)
		}
	}
	if flags&1 != 0 {
		*d = 0
	}
	if flags&0x10 != 0 && !g.combatShipHasSpecial(t, 7) && T.u8(tsMarines) != 0xff {
		g.killCrew(t, int16(int32(*d)/5))
	}
	if double != 0 {
		*d <<= 1
	}
	first := int16(0)
	if allInternal != 0 {
		first = 1
	} else {
		if mods&8 != 0 {
			first = 1
		}
		if g.combatShipHasSpecial(t, 0xe) || T.u8(tsArmor) == 6 {
			first = 0
		}
	}
	pierce := flags & 8
	if pierce != 0 && T.u8(tsOwner) > 9 {
		*d /= 4
	}
	if *d > 0 {
		ret |= 2
		if flags&0x20 != 0 {
			half := int16(int32(*d) / 2)
			v := T.i16(tsStructDmg) + half
			T.set16(tsStructDmg, v)
			if T.i16(tsStruct) > v {
				v += half
			} else {
				v = T.i16(tsStruct)
			}
			T.set16(tsStructDmg, v)
			back = half
		}
		var done int16
		if first != 0 {
			done = *d
			for cnt := *d; cnt > 0 && g.isCombatShipDead(t) == 0; {
				kind, k := g.selectInternal(t, allInternal, pierce, 1, drive)
				if kind == -1 {
					cnt = 0
				}
				g.applyInternalDamage(t, &cnt, kind, k, 1)
			}
		}
		for cnt := *d - done; cnt > 0 && g.isCombatShipDead(t) == 0; {
			kind, k := g.selectInternal(t, allInternal, pierce, 0, drive)
			if kind == -1 {
				cnt = 0
			}
			g.applyInternalDamage(t, &cnt, kind, k, 1)
		}
	}
	*d += back
	return ret
}

// killCrew is Kill_Crew_ (0x366af).
func (g *Game) killCrew(i, n int16) {
	T := g.tsp(i)
	if n <= int16(T.u8(tsMarines)) {
		T.set8(tsMarines, T.u8(tsMarines)-uint8(n))
		return
	}
	T.set8(tsMarines, 0)
}

// selectInternal is Select_Internal_ (0x36810): what a point of damage
// hits: 7 armour, 0 structure, 5 drive, 8 computer, 3 weapon k, 9 special
// k, 2 shield, -1 nothing.
func (g *Game) selectInternal(t, triple, pierce, first, drive int16) (kind, k int16) {
	s := &g.state
	T := g.tsp(t)
	roll := int16(s.Random.Random(100))
	kind = -1
	if drive != 0 {
		return 5, k
	}
	if first == 0 && T.i16(tsArmorHits) > 0 && pierce == 0 {
		return 7, k
	}
	tot := g.totalInternalSpace(t)
	pct := int16(100)
	if pierce == 0 {
		pct = int16(int32(tot) * 100 / (int32(T.i16(tsStruct)-T.i16(tsStructDmg)) + int32(tot)))
	}
	if T.u8(tsOwner) > 9 && pct > 0x19 {
		pct = 0x19
	}
	if pct < roll {
		return 0, k
	}
	var w [19]uint16
	w[0] = uint16(T.i16(tsDriveCur))
	w[1] = uint16(T.i16(tsCompCur))
	for j := 0; j < 8; j++ {
		v := T.i16(tsCur + 4 + 2*j)
		if triple != 0 {
			v *= 3
		}
		w[2+j] = uint16(v)
	}
	for j := 0; j < 8; j++ {
		w[10+j] = uint16(T.i16(tsCur + 0x24 + 2*j))
	}
	sh := T.i16(tsShieldCur)
	if triple != 0 {
		sh *= 3
	}
	w[18] = uint16(sh)
	c := getWeightedChoiceShort(&s.Random, w[:])
	switch {
	case c == 0:
		return 5, k
	case c == 1:
		return 8, k
	case c >= 2 && c < 10:
		return 3, c - 2
	case c >= 10 && c < 18:
		return 9, c - 10
	case c == 0x12:
		return 2, k
	}
	return kind, k
}

// applyInternalDamage is Apply_Internal_Damage_ (0x35251): *d damage on one
// system; what it cannot take is left in *d. Each point taken may kill a
// marine.
func (g *Game) applyInternalDamage(t int16, d *int16, kind, k, crew int16) {
	tc := g.tc
	s := &g.state
	T := g.tsp(t)
	if kind == 7 {
		if T.i16(tsArmorHits) < *d {
			*d -= T.i16(tsArmorHits)
			kind = -1
			T.set16(tsArmorHits, 0)
		} else {
			T.add16(tsArmorHits, -*d)
			*d = 0
		}
	}
	took := *d
	structure := func() {
		v := T.i16(tsStructDmg) + *d
		if T.i16(tsStruct) < v {
			v = T.i16(tsStruct)
		}
		T.set16(tsStructDmg, v)
	}
	if took > 0 {
		zero := true
		switch kind {
		case 2:
			v := T.i16(tsShieldCur) - took
			T.set16(tsShieldCur, v)
			if v < 1 {
				if v < 0 {
					v = -v
				}
				*d = v
				T.set16(tsShieldCur, 0)
				for a := 0; a < 4; a++ {
					T.set16(tsArc+2*a, 0)
				}
				T.set16(tsShieldMax, 0)
				zero = false
				break
			}
			mx := g.calcCombatShieldMax(t)
			for a := 0; a < 4; a++ {
				if T.i16(tsArc+2*a) > mx {
					T.set16(tsArc+2*a, mx)
				}
			}
		case 3:
			cur := tsCur + 4 + 2*int(k)
			unit := T.i16(tsCur + 0x14 + 2*int(k))
			part := T.i16(cur) % unit
			if part == 0 {
				part = unit
			}
			if part < *d {
				T.add16(cur, -part)
				*d -= part
			} else {
				T.add16(cur, -*d)
				*d = 0
			}
			old := T.u8(tw(int(k), twCount))
			n := uint8((int32(T.i16(cur)) + int32(unit) - 1) / int32(unit))
			T.set8(tw(int(k), twCount), n)
			if T.i16(tw(int(k), twWeapon)) == 0x20 && n != old {
				left := int16(n)
				for j := int16(1); j < tc.n; j++ {
					if int16(g.tsp(j).u8(tsStasis)) == t&0xff {
						if left == 0 {
							g.removeStasisBeam(j)
						} else {
							left--
						}
					}
				}
			}
			if T.i16(tw(int(k), twWeapon)) == 0x25 && T.u8(tw(int(k), twCount)) != old {
				left := int16(T.u8(tw(int(k), twCount)))
				for j := int16(1); j < tc.n; j++ {
					J := g.tsp(j)
					if int16(J.u8(0x137)) == t&0xff {
						if left == 0 {
							J.set8(0x136, 0)
							J.set8(0x137, 0xff)
						} else {
							left--
						}
					}
				}
			}
			zero = false
		case 5:
			v := T.i16(tsDriveCur) - took
			T.set16(tsDriveCur, v)
			if v < 0 {
				*d = -v
				T.set16(tsDriveCur, 0)
			} else {
				*d = 0
			}
			T.set16(tsSpeed, g.calcCurrentSpeed(t))
			if *d < 1 {
				zero = false
				break
			}
			structure()
		case 8:
			v := T.i16(tsCompCur) - took
			T.set16(tsCompCur, v)
			if v < 0 {
				*d = -v
				T.set16(tsCompCur, 0)
				zero = false
			}
		case 9:
			cur := tsCur + 0x24 + 2*int(k)
			v := T.i16(cur) - took
			T.set16(cur, v)
			if v < 0 {
				*d = -v
				T.set16(cur, 0)
				n := int16(0)
				for b := 0; b < 0x27; b++ {
					if !testBit(T[tsSpecials:tsSpecials+5], b) {
						continue
					}
					if k == n {
						setBit(T[tsDamaged:tsDamaged+5], b)
						if b == 0xd && tc.nebula == 1 {
							for a := 0; a < 4; a++ {
								T.set16(tsArc+2*a, 0)
							}
							T.set16(tsShieldMax, 0)
						}
						break
					}
					n++
				}
				zero = false
			}
		case 7, 0:
			structure()
		default:
			zero = false
		}
		if zero {
			*d = 0
		}
	}
	took -= *d
	if crew == 1 && T.u8(tsMarines) != 0xff {
		for j := int16(0); j < took; j++ {
			if s.Random.Random(100) < 2 {
				if T.u8(tsMarines) != 0 {
					T.set8(tsMarines, T.u8(tsMarines)-1)
				}
			}
		}
	}
}

// destroyShip is Destroy_Ship_ (0x39e15): kind 1 a plain kill; other kinds
// explode with damage around (Do_Spherical_SFX_).
func (g *Game) destroyShip(i, x, y, kind int16) {
	tc := g.tc
	T := g.tsp(i)
	enemy := tc.att
	if int16(T.u8(tsOwner)) == tc.att {
		enemy = tc.def
	}
	T.set8(0x4b, 0)
	g.retargetMissilesAimedAt(i)
	g.removeContinuingEffectSpecials(i)
	if tc.onScreen == 1 && tc.superFast == 0 {
		if T.u8(tsGone) == 0 {
			g.destroyShipFX(i, x, y, kind)
		}
	} else {
		T.set8(tsGone, 5)
	}
	if T.u8(tsBuilder) == 0xb {
		for j := int16(1); j < tc.n; j++ {
			if J := g.tsp(j); J.u8(tsOwner) == 0xb && j != i {
				J.set8(tsOwner, uint8(enemy))
			}
		}
	}
}

// unloadCombatShip is Unload_Combat_Ship_ (0x4b9e1): the record's state
// back into the ship.
func (g *Game) unloadCombatShip(i int16) {
	s := &g.state
	T := g.tsp(i)
	id := T.i16(tsID)
	if id <= -1 { // community patch 1.51: Antaran ships are -0x101
		return
	}
	if s.Ships[id].Design.CombatSpeed == 0 {
		// community patch 1.51 (0x402b55): a ship that cannot move never
		// gains speed by drive hits; 1.40 looped here for ever.
		T.set16(tsDriveCur, T.i16(tsDriveMax))
	}
	for s.Ships[id].Design.CombatSpeed != 0 && T.i16(tsSpeed) <= 0 {
		T.add16(tsDriveCur, 1)
		T.set16(tsSpeed, g.calcCurrentSpeed(i))
	}
	if T.i16(tsDriveCur) > T.i16(tsDriveMax) {
		T.set16(tsDriveCur, T.i16(tsDriveMax))
	}
	sh := &s.Ships[id]
	sh.Owner = int8(T.u8(tsOwner))
	for k := 0; k < 8; k++ {
		sh.Design.Weapons[k].Working = T.u8(tw(k, twCount))
	}
	copy(sh.DamagedSpecials[:], T[tsDamaged:tsDamaged+5])
	sh.StructureDamage = T.i16(tsStructDmg)
	sh.ArmorDamage = int16(g.shipArmorHits(int(id))) - T.i16(tsArmorHits)
	sh.ComputerDamage = 0
	if T.i16(tsShieldHP) > 0 {
		sh.ShieldDamage = uint8(100 - int8(int32(T.i16(tsShieldCur))*100/int32(T.i16(tsShieldHP))))
	}
	if T.i16(tsDriveMax) > 0 {
		sh.DriveDamage = uint8(100 - int8(int32(T.i16(tsDriveCur))*100/int32(T.i16(tsDriveMax))))
	}
}

// endOfCombat is End_Of_Combat_ (0x4b184): ships, officers, experience and
// relations after the battle; result is Check_For_Winner_'s.
func (g *Game) endOfCombat(star, popLost, result int16) {
	s := &g.state
	tc := g.tc
	var defLost, attLost, attLostMask, defLostMask int32
	var keep []int16
	winner := tc.def
	if result >= 0 {
		winner = tc.att
		if result == 0x29a {
			winner = -1
		}
	}
	for i := int16(1); i < tc.n; i++ {
		g.initSpecialDevices(i)
		T := g.tsp(i)
		// community patch 1.51 (0x4b1eb): a satellite, not any record without
		// drive (a driveless ship at a star without colony took Colonies[-1]).
		if T.satellite() && (T.u8(tsGone) == 5 || int16(T.u8(tsOwner)) != tc.def) {
			if star != s.NumStars {
				b := &s.Colonies[tc.colony].Buildings
				switch {
				case b[0x28] != 0:
					g.removeBuilding(int(tc.colony), 0x28)
				case b[8] != 0:
					g.removeBuilding(int(tc.colony), 8)
				case b[0x29] != 0:
					g.removeBuilding(int(tc.colony), 0x29)
				}
			}
			continue
		}
		if (T.u8(tsGone) == 5 && T.u8(0x4b) != 2) || (T.u8(tsBoarded) != 0 && int16(T.u8(tsOwner)) != winner) ||
			(T.u8(tsOwner) > 7 && T.u8(tsOwner) != T.u8(tsBuilder)) {
			T.set8(tsGone, 5)
			if int16(T.u8(tsOwner)) == tc.att {
				attLost += int32(T.u8(tsSize)) + 1
				attLostMask += 1 << (uint(T.u8(tsSize)+1) & 0x1f)
			} else {
				defLost += int32(T.u8(tsSize)) + 1
				defLostMask += 1 << (uint(T.u8(tsSize)+1) & 0x1f)
			}
			// community patch 1.51: ids below -1 (Antaran ships) are no ships.
			if T.i16(tsID) > -1 {
				if o := T.i16(tsOfficer); o > -1 {
					switch {
					case s.Leaders[o].ETA >= 1:
						g.moveOfficerToPool(int(o))
					case s.Players[T.u8(tsOwner)].AppStatus[0xb3] == 3:
						keep = append(keep, o)
						s.Ships[T.i16(tsID)].Officer = -1
					default:
						g.killOfficer(int(o))
					}
				}
			}
			continue
		}
		repaired := g.combatShipHasSpecial(i, 3)
		if !repaired {
			repaired = g.fleetEngineerBonus(int16(T.u8(tsOwner))) > 0
			if !repaired && T.u8(tsOwner) < 8 {
				P := &s.Players[T.u8(tsOwner)]
				repaired = P.AppStatus[4] == 3 || P.Picks[17] != 0
			}
		}
		if repaired {
			copy(T[tsCur:tsCur+0x36], T[tsDriveMax:tsDriveMax+0x36])
			for k := 0; k < 5; k++ {
				T[tsDamaged+k] = 0
			}
			T.set16(tsStructDmg, 0)
			if T.i16(tsID) > -1 {
				T.set16(tsArmorHits, int16(g.shipArmorHits(int(T.i16(tsID)))))
			}
		}
		g.unloadCombatShip(i)
	}
	defKills := int16(defLost)
	if tc.att < 8 {
		if tc.bioWeapon != 0 {
			g.applyBioWeaponPenalty(tc.att, star, popLost)
		}
		if tc.def < 8 {
			s.Players[tc.att].WarLosses[tc.def] += int16(attLostMask)
			s.Players[tc.def].WarLosses[tc.att] += int16(defLostMask)
			if defKills < 1 {
				defKills = 1
			}
			if popLost > 0 && tc.realAttack != 0 {
				g.russChangeRelations(-10000, int(tc.att), int(tc.def), 10, star, popLost)
			}
			if tc.realAttack != 0 {
				g.russChangeRelations(defKills*-0xf, int(tc.att), int(tc.def), 0xb, star, 0)
			}
		}
	}
	xp := func() int16 {
		v := defKills
		if winner != tc.att {
			v = int16(attLost)
		}
		v /= 2
		if v < 1 {
			v = 1
		}
		return v
	}
	for i := int16(1); i < tc.n; i++ {
		T := g.tsp(i)
		if (T.u8(tsGone) == 0 || T.u8(0x4b) == 2) && T.i16(tsOfficer) != -1 && int16(T.u8(tsOwner)) == winner && T.i16(tsID) != -1 {
			g.giveOfficerXP(int(winner), int(T.i16(tsOfficer)), xp())
		}
	}
	for _, o := range keep {
		seated, found := false, false
		for i := int16(1); i < tc.n && !seated; i++ {
			T := g.tsp(i)
			if T.u8(tsGone) == 5 && T.u8(0x4b) != 2 {
				continue
			}
			id := T.i16(tsID)
			if id <= -1 || int16(s.Leaders[o].Owner) != int16(T.u8(tsOwner)) {
				continue
			}
			if s.Ships[id].Officer == -1 {
				s.Ships[id].Officer = o
				T.set16(tsOfficer, o)
				s.Ships[s.Leaders[o].Location].Officer = -1
				seated = true
				s.Leaders[o].Location = id
			}
			found = true
		}
		if (seated || found) && int16(s.Leaders[o].Owner) == winner {
			g.giveOfficerXP(int(winner), int(o), xp())
		}
		if !seated {
			if found {
				g.moveOfficerToPool(int(o))
			} else {
				g.killOfficer(int(o))
			}
		}
	}
	for i := int16(1); i < tc.n; i++ {
		T := g.tsp(i)
		switch {
		case star == s.NumStars && T.u8(tsOwner) == 8 && T.u8(tsGone) == 5 && T.u8(tsDrive) != 0:
			s.killAntaranDefensiveShip(int(T.u8(tsSize)))
		case T.u8(tsGone) == 5 && T.i16(tsID) > -1 && T.u8(0x4b) != 2:
			if T.u8(tsOwner) == 8 {
				s.killAntaranOffensiveShip(int(T.u8(tsSize)))
			}
			if s.Ships[T.i16(tsID)].Officer > -1 {
				g.moveOfficerToPool(int(s.Ships[T.i16(tsID)].Officer))
			}
			g.killShip(int(T.i16(tsID)))
		case int16(T.u8(tsOwner)) == winner && T.i16(tsID) > -1:
			s.Ships[T.i16(tsID)].Experience += xp()
		}
	}
	if tc.colony != -1 {
		C := &s.Colonies[tc.colony]
		if C.IsOutpost == 0 && C.PopCount == 0 {
			any := false
			for b := 0; b < 0x31 && !any; b++ {
				any = C.Buildings[b] != 0
			}
			if !any {
				g.destroyColony(int(tc.colony), 1)
			}
		}
	}
}

// sphericalSFXWeapon is the weapon whose damage a spherical blast of kind
// 0..2 rolls (0x17fce7, 0x17fc07, 0x17f983).
var sphericalSFXWeapon = [3]int16{0x2c, 0x24, 0xd}

// doSphericalSFX is Do_Spherical_SFX_ (0xade18): a blast round record i
// (kinds 0..2 spherical weapons with n shots, 3/4 an exploding ship with
// dmg damage). Ships within the animation's radius take damage, missiles
// may be shot down; while it is drawn, ships in the shock wave shake with
// two Random(5) each per frame.
func (g *Game) doSphericalSFX(i, x, y, kind, dmg int16) {
	s := &g.state
	tc := g.tc
	drawing := tc.onScreen == 1 && tc.superFast == 0
	if drawing && tc.scr != nil && !g.shipCompletelyOnScreen(i) {
		T := g.tsp(i)
		g.must2(g.snapCenterCombatScreen(int16(T.u8(tsX)), int16(T.u8(tsY))))
	}
	cx, cy := g.shipCenterOffsets(i)
	cx += x
	cy += y
	// 0xadec5: the blast's sounds; one in ten exploding ships adds 0x21.
	switch {
	case kind <= 2:
		g.moxPlaySound(0x14)
	case kind == 3:
		g.moxPlaySound(0x2f)
		g.moxPlaySound(8)
		if s.Random.Random(10) == 1 {
			g.moxPlaySound(0x21)
		}
	case kind == 4:
		g.moxPlaySound(0x19)
		g.moxPlaySound(8)
	}
	var file string
	var entry int
	switch {
	case kind < 2:
		file, entry = "CMBTSFX.LBX", 0x34
	case kind < 3:
		file, entry = "CMBTSFX.LBX", 6
	case kind < 4:
		file, entry = "SPHERSFX.LBX", 0
	case kind == 4:
		file, entry = "SPHERSFX.LBX", 1
	}
	p, err := g.picture(file, entry)
	if err != nil {
		g.Log("Do_Spherical_SFX_: %v", err)
		return
	}
	r2 := int32(int16(p.Width) / 2)
	r2 *= r2
	var list []sphereHit
	center := func(j int16) (int16, int16) {
		ox, oy := g.shipCenterOffsets(j)
		J := g.tsp(j)
		return ox + int16(J.u8(tsX))*0x14, oy + int16(J.u8(tsY))*0x14
	}
	abs16 := func(v int32) int32 {
		w := int16(v)
		if w < 0 {
			w = -w
		}
		return int32(w)
	}
	for j := int16(1); j < tc.n; j++ {
		J := g.tsp(j)
		if J.u8(tsGone) != 0 || g.shipInStasis(j) || j == i || J.u8(tsGone) == 5 {
			continue
		}
		jx, jy := center(j)
		dx, dy := abs16(int32(jx)-int32(cx)), abs16(int32(jy)-int32(cy))
		d2 := dx*dx + dy*dy
		if d2 > r2 {
			continue
		}
		a, b := cx, jx
		if abs16(int32(cx)-int32(jx)) < abs16(int32(cy)-int32(jy)) {
			a, b = cy, jy
		}
		ring := int32(a) - int32(b)
		if ring < 0 {
			ring = -ring
		}
		list = append(list, sphereHit{j, int16(ring / 0x14), d2})
	}
	if drawing {
		g.sphericalSFXFrames(i, x, y, kind, p, list)
	}
	shots := func(w int16, n int16) int16 {
		var sum int16
		lo, hi := g.wpnI16(int(w), weaponMinDmg), g.wpnI16(int(w), weaponMaxDmg)
		for k := int16(0); k < n; k++ {
			sum += int16(s.Random.Random(uint32(int32(hi)-int32(lo)+1))) + lo - 1
		}
		return sum
	}
	ord := tc.ordnance[g.tsp(i).u8(tsOwner)]
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		w := M.i16(tmWeapon)
		if w <= 0 || g.wtype(w) == 2 {
			continue
		}
		if g.wtype(w) == 4 && s.Random.Random(2) == 1 {
			continue
		}
		dx := abs16(int32(M.i16(tmX)) - int32(cx))
		dy := abs16(int32(M.i16(tmY)) - int32(cy))
		d2 := dy*dy + dx*dx
		if d2 > r2 {
			continue
		}
		var power, fall int16
		if kind <= 2 {
			power = int16(int32(ord) * int32(shots(sphericalSFXWeapon[kind], dmg)) / 100)
		} else {
			fall = int16(d2 * 100 / r2)
			power = dmg
		}
		h := g.missileHits(w, M.i16(tmOwner), M.i16(tmMods))
		chance := int32(int16(int32(power)*(100-int32(fall))/100)) * 0x19 / int32(h)
		for k, n := int16(0), M.i16(tmCount); k < n; k++ {
			if int32(s.Random.Random(100)) <= int32(int16(chance)) {
				M.set16(tmCount, M.i16(tmCount)-1)
			}
		}
		if M.i16(tmCount) == 0 {
			g.killMissile(m)
		}
	}
	for _, h := range list {
		var power, fall int16
		if kind <= 2 {
			power = int16(int32(ord) * int32(shots(sphericalSFXWeapon[kind], dmg)) / 100)
		} else {
			fall = int16(h.d2 * 100 / r2)
			power = dmg
		}
		per := int32(power) * (100 - int32(fall)) / 100
		if per < 2 {
			per = 1
		}
		var total int16
		J := g.tsp(h.j)
		for k := int32(0); k < int32(J.u8(tsSize))+1; k++ {
			v := int16(s.Random.Random(uint32(int16(per))))
			if v < 2 {
				v = 1
			} else {
				v = int16(s.Random.Random(uint32(int16(per))))
			}
			total += v
		}
		if kind == 2 {
			J.add16(tsStructDmg, int16(per))
			continue
		}
		var ab int16
		g.applyShipDamage(h.j, &total, &ab, facingShield(g.relativeBearing(h.j, i)), 0, 0, 0, 0, 0)
		if tc.onScreen == 1 && tc.superFast == 0 {
			x := (int16(J.u8(tsX)) - tc.curX) * 0x14
			y := (int16(J.u8(tsY)) - tc.curY) * 0x14
			g.addDamageIndicatorForShip(x, y, h.j, 1, total, ab)
		}
	}
	if tc.onScreen == 1 && tc.superFast == 0 {
		g.drawDamageMessageQueueUntilDone()
	}
	for _, h := range list {
		if g.tsp(h.j).u8(tsGone) == 0 {
			if dead := g.isCombatShipDead(h.j); dead != 0 {
				J := g.tsp(h.j)
				g.destroyShip(h.j, int16(J.u8(tsX))*0x14, int16(J.u8(tsY))*0x14, dead)
			}
		}
	}
	if drawing {
		// 0xaebe8: the full redraw that brings the weapon panel up to date.
		if tc.scr != nil {
			g.must2(g.refreshCombatScreenFull())
		} else {
			g.assignCombatGrids(tc.cur)
		}
	}
}

// damageFromShipExploding is Damage_From_Ship_Exploding_ (0x38656).
func (g *Game) damageFromShipExploding(i int16) int16 {
	v := g.hullI16(int(g.tsp(i).u8(tsSize)), hullDrive)
	if g.combatShipHasSpecial(i, 0x18) {
		return v * 0xf
	}
	return v * 5
}
