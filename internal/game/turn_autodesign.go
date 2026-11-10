package game

import "github.com/openorion2/openorion2/internal/save"

// The AI ship designer (AIDESIGN.C): Auto_Design_Ship_ fills a hull by a
// class's percentages with specials, fighters, missiles, beams and bombs.
// Spec: docs/turn/overview.md, D5 ("Auto_Design_Ship_").

// Weapon and special table fields (rows at weaponTable and specialTable).
const (
	weaponSpace   = 0x0e // i16 space
	weaponFlagsA  = 0x12 // u8
	weaponFlagsB  = 0x13 // u8
	specialSpace  = 0x02 // i16 by hull size
	specialBlocks = 0x1a // 6 x i16 specials it excludes

	specialRow = 0x2f
	specialTbl = 0x17eee6

	designClassTable = 0x1812b0 // i16 percentages, 0x18-byte rows by hull, 0x90 per class
	missileMinCount  = 0x1812a4 // i16 by hull size
	hullSpaceTable   = 0x180020 // i16 space, 0x24-byte rows by hull
	special0bSpace   = 0x17f0ed // i16 special 0xb's space for hull 0

	listBeamOffense  = 0x1811a4
	listMissileOff   = 0x1811c0
	listSpecialWpn   = 0x1811c8
	listSpecialDev   = 0x1811dc
	listDefenseT     = 0x18120c
	listDefenseF     = 0x181258
	competitionShips = 0x1a // Get_Competition_Player_Tech_Values_ kinds
)

// weaponsIndexFromTechApp is Weapons_Index_From_Tech_App_ (0x78d3e): the
// first weapon row of app, 0 for none.
func (g *Game) weaponsIndexFromTechApp(app int16) int {
	for w := 0; w < 0x28; w++ {
		if g.wpnI16(w, weaponApp) == app {
			return w
		}
	}
	return 0
}

// miniaturizationCategory is Miniaturization_Category_ (0x6f11c): which
// space table an application's miniaturisation follows (0, 1 or 2).
func (g *Game) miniaturizationCategory(app int16) uint8 {
	g.initTech()
	kind := g.exeU8(uint32(techAppTable + int32(app)*techAppRow + 0xc))
	switch kind {
	case 3:
		for _, a := range []uint32{0x17fb71, 0x17fb55, 0x17fb1d, 0x17fb39} {
			if app == g.exeI16(a) {
				return 2
			}
		}
		switch g.wpnU8(g.weaponsIndexFromTechApp(app), weaponType) {
		case 3, 1, 0:
			return 0
		}
		return 1
	case 9:
		for _, a := range []uint32{0x17ef44, 0x17f3db, 0x17f0eb, 0x17f178, 0x17f582} {
			if app == g.exeI16(a) {
				return 2
			}
		}
	}
	return 1
}

// spaceGivenMLevel is Space_Given_M_Level_ (0x6e60e): space after
// miniaturisation of the category's table, rounded, at least 1.
func spaceGivenMLevel(v, level int16, cat uint8) int16 {
	pct := int32(1000)
	switch cat {
	case 0:
		pct = 250
		if uint16(level) <= 4 {
			pct = [5]int32{1000, 800, 650, 500, 350}[level]
		}
	case 1:
		pct = 400
		if uint16(level) <= 4 {
			pct = [5]int32{1000, 800, 700, 600, 500}[level]
		}
	}
	r := (pct*int32(v) + 500) / 1000
	if r == 0 && v > 0 {
		r = 1
	}
	return int16(r)
}

// shieldSpaceGivenMLevel is Shield_Space_Given_M_Level_ (0x6e38e).
func (g *Game) shieldSpaceGivenMLevel(shield, size uint8, p int) int16 {
	sp := g.exeI16(rowAddr(shieldTable, shieldStride, int(shield), 8+2*uint32(size)))
	if shield > 0 && sp == 0 {
		sp = 1
	}
	app := g.exeI16(rowAddr(shieldTable, shieldStride, int(shield), shieldApp))
	return spaceGivenMLevel(sp, g.techLevel(p, int(app)), 0)
}

// weaponSpace is Weapon_Space_ (0x6ee8e): the space count weapons w take
// with arcs, ammunition and modifications.
func (g *Game) weaponSpace(p, w int, count, arcs, ammo, mods int16) int32 {
	sp := int32(g.wpnI16(w, weaponSpace))
	typ := g.wpnU8(w, weaponType)
	if typ == 1 {
		switch uint16(ammo) {
		case 2:
			sp += 10
		case 5:
			sp += 20
		case 10:
			sp += 30
		case 15:
			sp += 35
		case 20:
			sp += 40
		}
	}
	_, other := g.bonusForSpecialMods(int32(mods))
	sp = int32(other) * sp / 100
	var arcPct int32
	if typ == 0 {
		switch {
		case arcs&2 != 0 || arcs&4 != 0:
			arcPct = 25
		case arcs&0x10 != 0:
			arcPct = 50
		}
	}
	_, reg := g.bonusForRegularMods(int32(mods))
	sp = (arcPct + 100 + int32(reg)) * sp / 100
	app := g.wpnI16(w, weaponApp)
	cat := g.miniaturizationCategory(app)
	r := int32(spaceGivenMLevel(int16(sp), g.techLevel(p, int(app)), cat)) * int32(count)
	return max(r, 1)
}

// getCompetitionPlayerTechValues is Get_Competition_Player_Tech_Values_
// (0x5f8d1): the best field rank among known applications of a kind. It
// reads player 0's applications whatever the argument.
func (g *Game) getCompetitionPlayerTechValues(_ int, kind uint8) uint16 {
	g.initTech()
	var best int16
	for a := 0; a < 0xd4; a++ {
		if g.exeU8(uint32(techAppTable+a*techAppRow+9)) != kind || g.state.Players[0].AppStatus[a] != techKnown {
			continue
		}
		f := g.techAppFieldOf(a)
		if v := int16(g.exeU8(uint32(techFieldTable + f*techFieldRow + 0x16))); v > best {
			best = v
		}
	}
	return uint16(best)
}

// firstLivingPlayer is the loop Auto_Design_Ship_ and its helpers use to
// find a "competitor": the first player not eliminated.
func (s *State) firstLivingPlayer() int {
	p := 0
	for s.Players[p].Eliminated != 0 {
		p++
	}
	return p
}

// fightersAvailable, specialWeaponsAvailable and specialDevicesAvailable
// are Fighters_Available_ (0x5f968), Special_Weapons_Available_ (0x5f9a0)
// and Special_Devices_Available_ (0x5f9ea).
func (g *Game) fightersAvailable(p int) bool {
	a := &g.state.Players[p].AppStatus
	return a[0x42] == techKnown || a[0x1f] == techKnown || a[0x53] == techKnown
}

func (g *Game) specialWeaponsAvailable(p int) bool {
	a := &g.state.Players[p].AppStatus
	return a[0x50] == techKnown || a[0x3c] == techKnown || a[0x1e] == techKnown ||
		a[0xae] == techKnown || a[0x8c] == techKnown
}

func (g *Game) specialDevicesAvailable(p int) bool {
	a := &g.state.Players[p].AppStatus
	for i := 0; i < 0xc; i++ {
		id, weapon := g.specialListEntry(listSpecialDev, i)
		var app int16
		if weapon {
			app = g.wpnI16(int(id), weaponApp)
		} else {
			app = g.exeI16(uint32(specialTbl + int32(id)*specialRow))
		}
		if a[app] == techKnown {
			return true
		}
	}
	return false
}

// specialListEntry reads entry i of a 4-byte special list: an id and
// whether it is a weapon row rather than a special device.
func (g *Game) specialListEntry(list uint32, i int) (int16, bool) {
	return g.exeI16(list + uint32(4*i)), g.exeI16(list+uint32(4*i)+2) != 0
}

// getShipClass is Get_Ship_Class_ (0x5fa87): the AI's design class for
// player p by its ship theme and a Random(20) or two; 7 when forced.
func (g *Game) getShipClass(p int, forced bool) int16 {
	s := &g.state
	fighters := g.fightersAvailable(p)
	specW := g.specialWeaponsAvailable(p)
	specD := g.specialDevicesAvailable(p)
	if forced {
		return 7
	}
	pl := &s.Players[p]
	plain := func() int16 { // the 0x64 check shared by most outcomes
		if pl.AppStatus[0x64] == techKnown {
			return 0
		}
		return 3
	}
	fightersOr := func(v int16, other func() int16) int16 {
		if fighters {
			return v
		}
		return other()
	}
	two := func() int16 { return 2 }
	switch pl.ShipTheme {
	case 0, 100:
		switch r := s.Random.Random(20); {
		case r >= 1 && r <= 13:
			return plain()
		case r == 14:
			return 3
		case r == 15:
			return fightersOr(4, plain)
		case r == 16:
			return fightersOr(1, plain)
		case r == 17:
			return 2
		case r <= 19:
			if specW {
				return 6
			}
			return plain()
		case r == 20:
			if specD {
				return 5
			}
			return plain()
		}
		return 0
	case 1:
		mid := func() int16 { return fightersOr(1, two) }
		switch r := s.Random.Random(20); {
		case r == 1:
			return plain()
		case r == 2:
			return 3
		case r >= 3 && r <= 15:
			return fightersOr(4, plain)
		case r <= 18:
			return mid()
		case r == 19:
			if specW {
				return 6
			}
			return mid()
		case r == 20:
			if specD {
				return 5
			}
			return mid()
		}
		return 0
	case 2:
		// The first draw's class is always replaced by the second's.
		s.Random.Random(20)
		fallthrough
	case 3:
		mid := func() int16 { return fightersOr(1, two) }
		switch r := s.Random.Random(20); {
		case r == 1:
			return plain()
		case r == 2:
			return 3
		case r == 3:
			return fightersOr(4, plain)
		case r <= 5:
			return mid()
		case r <= 7:
			return 2
		case r <= 11:
			if specW {
				return 6
			}
			return mid()
		case r <= 15:
			if specW {
				return 6
			}
			return 2
		case r <= 17:
			if specD {
				return 5
			}
			return mid()
		case r <= 20:
			if specD {
				return 5
			}
			return 2
		}
		return 0
	}
	return 0
}

// getShipDesignName is Get_Ship_Design_Name_ (0x6c897): record
// race*0x30+size*8+idx of SHIPNAME.LBX.
func (g *Game) getShipDesignName(race uint8, size, idx int, d *save.Design) {
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
	copy(d.Name[:], save.CString(t.Record(int(race)*0x30+size*8+idx))+"\x00")
}

// copyDoomStarName is the strcpy of [0x1800ee]: the hull table's size 5
// name (Load_Tech_Names_ fills it at start-up), "Doom Star".
func (g *Game) copyDoomStarName(d *save.Design) {
	n, err := g.hullName(5, false)
	if err != nil {
		g.Log("design name: %v", err)
		return
	}
	copy(d.Name[:], n+"\x00")
}

// autoDesignShip is Auto_Design_Ship_ (0x616a5): an AI design for hull
// size of player p in d. forced is _forced_design_class (0x1998f0) set to
// -1000 by Init_Ship_Designs_ for its first design.
func (g *Game) autoDesignShip(p int, d *save.Design, size int16, forced bool) {
	s := &g.state
	pl := &s.Players[p]
	class := g.getShipClass(p, forced)
	if size == 5 && pl.AppStatus[0xae] == techKnown && s.Random.Random(4) < 4 {
		class = 6
	}
	if pl.AppStatus[0x1e] == techKnown && size > 3 && s.Random.Random(4) < 4 {
		class = 6
	}
	*d = save.Design{}
	if size == 5 {
		d.Picture = 0x2b
		g.copyDoomStarName(d)
	} else {
		d.Picture = uint8(s.Random.Random(8)) + uint8(size)<<3 - 1
		g.getShipDesignName(pl.Race, int(size), int(int8(d.Picture))%8, d)
	}
	if class == 7 {
		if t, err := g.kenText(0x54); err == nil {
			copy(d.Name[:], t+"\x00")
		} else {
			g.Log("design name: %v", err)
		}
	}
	d.Size = uint8(size)
	d.Type = 0
	d.Shield = uint8(g.bestShipShield(p))
	d.Drive = uint8(g.bestWarpDrive(p))
	d.FTLSpeed = uint8(g.ftlSpeed(p, int(d.Drive)))
	d.Computer = uint8(g.bestComputer(p))
	d.Armor = uint8(g.bestArmor(p))
	space := g.exeI16(uint32(hullSpaceTable + int32(size)*0x24))
	if pl.AppStatus[0x19] == techKnown {
		space = space/2 + space
		setBit(d.Specials[:], 4)
	}
	space -= g.shieldSpaceGivenMLevel(d.Shield, d.Size, p)
	var slots int16
	left := int16(0)
	pct := func(base uint32, off uint32, round bool) (int16, bool) {
		v := g.exeI16(base + uint32(size)*0x18 + off)
		if v == -1 {
			return 0, false
		}
		r := int32(v) * int32(space)
		if round {
			r += 50
		}
		return int16(int32(left) + r/100), true
	}
	type step struct {
		off   uint32
		kind  int // 0 theme, 1 defence, 2 list, 3 fighters, 4 missile, 5 beam
		arg   int16
		list  uint32
		round bool
	}
	var steps []step
	base := uint32(designClassTable)
	switch class {
	case 7:
		if pl.AppStatus[0x3f] == techKnown {
			setBit(d.Specials[:], 0xb)
			cat := g.miniaturizationCategory(0x3f)
			space -= spaceGivenMLevel(g.exeI16(special0bSpace), g.miniaturizationLevel(p, 0x3f), cat)
		}
		fallthrough
	case 0:
		steps = []step{{0, 0, 0, 0, false}, {6, 1, 0, 0, false}, {2, 2, 7, listBeamOffense, false},
			{0xe, 4, 2, 0, false}, {0x10, 5, 2, 0, false}, {0x12, 5, 1, 0, true}}
	case 1:
		base += 0x90
		steps = []step{{0, 0, 0, 0, false}, {6, 1, 0, 0, false}, {0xc, 3, 0, 0, false},
			{0x10, 5, 2, 0, false}, {0x12, 5, 1, 0, true}}
	case 2:
		base += 2 * 0x90
		steps = []step{{0, 0, 0, 0, false}, {6, 1, 0, 0, false}, {0xe, 4, 5, 0, false},
			{0x10, 5, 2, 0, false}, {0x12, 5, 1, 0, true}}
	case 3:
		base += 3 * 0x90
		steps = []step{{0, 0, 0, 0, false}, {6, 1, 0, 0, false}, {4, 2, 2, listMissileOff, false},
			{0xe, 4, 10, 0, false}, {0x12, 5, 1, 0, true}, {0x14, 5, 0, 0, true}}
	case 4:
		base += 4 * 0x90
		steps = []step{{0, 0, 0, 0, false}, {6, 1, 0, 0, false}, {0xc, 3, 0, 0, false},
			{0x12, 5, 1, 0, true}, {0x14, 5, 0, 0, true}}
	case 5:
		base += 5 * 0x90
		steps = []step{{0, 0, 0, 0, false}, {0xa, 2, 0xc, listSpecialDev, false}, {6, 1, 0, 0, false},
			{0x10, 5, 2, 0, false}, {0x12, 5, 1, 0, true}}
	case 6:
		base += 6 * 0x90
		steps = []step{{0, 0, 0, 0, false}, {6, 1, 0, 0, false}, {8, 2, 5, listSpecialWpn, false},
			{0x12, 5, 1, 0, true}, {0x10, 5, 2, 0, false}, {0x14, 5, 0, 0, true}}
	}
	if class >= 0 && class <= 7 {
		for _, st := range steps {
			amount, ok := pct(base, st.off, st.round)
			if !ok {
				continue
			}
			left = amount
			var used int32
			switch st.kind {
			case 0:
				used = g.addThemeSpecialToDesign(p, d, amount, &slots)
			case 1:
				q := s.firstLivingPlayer()
				list := uint32(listDefenseT)
				if g.getCompetitionPlayerTechValues(q, competitionShips) >= g.getCompetitionPlayerTechValues(q, 0x15) {
					list = listDefenseF
				}
				used = g.addSpecialsToDesign(list, 0x13, p, d, amount, &slots)
			case 2:
				used = g.addSpecialsToDesign(st.list, int(st.arg), p, d, amount, &slots)
			case 3:
				used = g.addFightersToDesign(p, d, &slots, amount)
			case 4:
				used = g.addMissileWeaponToDesign(p, d, &slots, amount, st.arg)
			case 5:
				used = g.addBeamWeaponToDesign(p, d, &slots, amount, st.arg)
			}
			left -= int16(used)
		}
		if amount, ok := pct(base, 0x16, false); ok {
			g.addBombWeaponToDesign(p, d, &slots, amount)
		}
	}
	d.CombatSpeed = g.exeU8(rowAddr(driveTable, driveStride, int(d.Drive), driveCombat) + uint32(d.Size))
	d.Builder = uint8(p)
	d.Cost = int16(g.ownerCostOfDesign(p, d)) // community patch 1.51 (0x61b36)
	d.BuildDate = uint16(s.Stardate)
}

// setBit sets bit n of a little-endian bit field (Set_Bit_Field_).
func setBit(b []byte, n int) { b[n>>3] |= 1 << (n & 7) }

// addThemeSpecialToDesign is Add_Theme_Special_To_Design_ (0x600bf): the
// specials of the player's special theme.
func (g *Game) addThemeSpecialToDesign(p int, d *save.Design, amount int16, slots *int16) int32 {
	type tl struct {
		list uint32
		n    int
	}
	var t tl
	switch g.state.Players[p].SpecialTheme {
	case 0:
		t = tl{0x18115c, 4}
	case 1:
		t = tl{0x18113c, 5}
	case 2:
		t = tl{0x181128, 5}
	case 3, 100:
		t = tl{0x18116c, 8}
	case 4:
		t = tl{0x181150, 3}
	case 5:
		t = tl{0x18118c, 3}
	case 6:
		t = tl{0x181198, 3}
	default:
		return 0
	}
	return g.addSpecialsToDesign(t.list, t.n, p, d, amount, slots)
}

// specialLegalForDesign is Special_Legal_For_Design_ (0x5fd8c): no fitted
// special excludes sp, and shield-based specials need a shield.
func (g *Game) specialLegalForDesign(d *save.Design, sp int16) bool {
	ok := true
	for b := 0; ok && b < 0x27; b++ {
		if !testBit(d.Specials[:], b) {
			continue
		}
		for k := 0; k < 6; k++ {
			if sp == g.exeI16(uint32(specialTbl+b*specialRow+specialBlocks+2*k)) {
				ok = false
			}
		}
	}
	if d.Shield == 0 && (sp == 0xd || sp == 0x14 || sp == 0x1e) {
		ok = false
	}
	return ok
}

// fillSlots is the slot filling shared by the Add_*_To_Design_ routines:
// n weapons in chunks of chunk per slot, each new slot set up by init.
// It returns the number placed; the slot count goes past 8 by one.
func fillSlots(d *save.Design, slots *int16, n, chunk int16, init func(*save.WeaponSlot)) int32 {
	var placed int32
	init(&d.Weapons[*slots])
	for n > chunk && *slots < 8 {
		w := &d.Weapons[*slots]
		w.Count, w.Working = uint8(chunk), uint8(chunk)
		placed += int32(chunk)
		*slots++
		n -= chunk
		if *slots < 8 {
			init(&d.Weapons[*slots])
		}
	}
	if *slots < 8 {
		w := &d.Weapons[*slots]
		w.Count, w.Working = uint8(n), uint8(n)
		placed += int32(n)
	}
	*slots++
	return placed
}

// addSpecialsToDesign is Add_Specials_To_Design_ (0x5fe14): the list's
// weapons (by space) and special devices (when they fit) the player
// knows, within amount space; it returns the space used.
func (g *Game) addSpecialsToDesign(list uint32, n, p int, d *save.Design, amount int16, slots *int16) int32 {
	s := &g.state
	pl := &s.Players[p]
	var used int32
	have21 := false
	for i := int16(0); i < int16(n); i++ {
		id, weapon := g.specialListEntry(list, int(i))
		if weapon {
			if *slots >= 8 {
				continue
			}
			app := g.wpnI16(int(id), weaponApp)
			if id == 0x21 {
				for j := int16(0); j < *slots; j++ {
					if d.Weapons[j].Weapon == 0x21 {
						have21 = true
					}
				}
			}
			if pl.AppStatus[app] != techKnown || have21 {
				continue
			}
			k := int16(int32(amount) / g.weaponSpace(p, int(id), 1, 0x10, 1, 0))
			if id == 0x21 && int32(k) > int32(d.Size)*10 {
				k = int16(d.Size) * 10
			}
			var placed int32
			if k > 0 {
				placed = fillSlots(d, slots, k, 99, func(w *save.WeaponSlot) {
					w.Weapon, w.Arc, w.Mods, w.Ammo = id, 0x10, 0, 1
				})
			}
			used += g.weaponSpace(p, int(id), 1, 0x10, 1, 0) * placed
			amount -= int16(g.weaponSpace(p, int(id), 1, 0x10, 1, 0) * placed)
			continue
		}
		app := g.exeI16(uint32(specialTbl + int32(id)*specialRow))
		if pl.AppStatus[app] != techKnown || !g.specialLegalForDesign(d, id) {
			continue
		}
		if pl.Picks[save.PickStealthyShips] != 0 && id == 0x1f {
			continue
		}
		cat := g.miniaturizationCategory(app)
		lvl := g.techLevel(p, int(app))
		base := g.exeI16(uint32(specialTbl + int32(id)*specialRow + specialSpace + 2*int32(d.Size)))
		if sp := spaceGivenMLevel(base, lvl, cat); sp < amount {
			setBit(d.Specials[:], int(id))
			used += int32(sp)
			amount -= sp
		}
	}
	return used
}

// addFightersToDesign is Add_Fighters_To_Design_ (0x601ac): the best
// fighter bay type that fits, in slots of 20. The space returned is
// counted with weapon 0x1f whichever bay was fitted.
func (g *Game) addFightersToDesign(p int, d *save.Design, slots *int16, amount int16) int32 {
	pl := &g.state.Players[p]
	var w int16
	switch {
	case pl.AppStatus[0x53] == techKnown && int32(amount)/g.weaponSpace(p, 0x1d, 1, 0x10, 1, 0) > 0:
		w = 0x1d
	case pl.AppStatus[0x1f] == techKnown && int32(amount)/g.weaponSpace(p, 0x1e, 1, 0x10, 1, 0) > 0:
		w = 0x1e
	default:
		w = 0x1f
	}
	n := int16(int32(amount) / g.weaponSpace(p, int(w), 1, 0x10, 1, 0))
	if n <= 0 || w == 0 || *slots >= 8 {
		return 0
	}
	placed := fillSlots(d, slots, n, 20, func(s *save.WeaponSlot) {
		s.Weapon, s.Arc, s.Mods, s.Ammo = w, 0x10, 0, 1
	})
	return g.weaponSpace(p, 0x1f, 1, 0x10, 1, 0) * placed
}

// determineBombWeapon is Determine_Bomb_Weapon_ (0x608af): the cheapest
// known bomb whose level reaches the competition's, else the best.
func (g *Game) determineBombWeapon(p int) int16 {
	s := &g.state
	list, rank := g.knownWeaponsByRank(p, func(w int) bool {
		return g.wpnU8(w, weaponType) == 3 && g.wpnU8(w, weaponFlagsA)&0x80 == 0
	})
	if len(list) == 0 {
		return 0
	}
	q := s.firstLivingPlayer()
	t := (int32(g.getCompetitionPlayerTechValues(q, 0x21))*2+2)/3 + int32(g.getCompetitionPlayerTechValues(q, 0xa))
	for i := range list {
		ml := min(g.miniaturizationLevel(p, int(g.wpnI16(int(list[i]), weaponApp))), 5)
		if int32(rank[i])+int32(ml) >= int32(int16(t)) {
			return list[i]
		}
	}
	return list[len(list)-1]
}

// knownWeaponsByRank lists the known weapons 1..0x27 that pass keep with
// their field rank, bubble-sorted by rank as the Determine_* routines do.
func (g *Game) knownWeaponsByRank(p int, keep func(int) bool) (list, rank []int16) {
	pl := &g.state.Players[p]
	for w := 1; w < 0x28; w++ {
		if !keep(w) {
			continue
		}
		app := int(g.wpnI16(w, weaponApp))
		if pl.AppStatus[app] != techKnown {
			continue
		}
		f := g.techAppFieldOf(app)
		list = append(list, int16(w))
		rank = append(rank, int16(g.exeU8(uint32(techFieldTable+f*techFieldRow+0x16))))
	}
	for swapped := true; swapped; {
		swapped = false
		for j := 0; j < len(list)-1; j++ {
			if rank[j+1] < rank[j] {
				rank[j], rank[j+1] = rank[j+1], rank[j]
				list[j], list[j+1] = list[j+1], list[j]
				swapped = true
			}
		}
	}
	return list, rank
}

// addBombWeaponToDesign is Add_Bomb_Weapon_To_Design_ (0x60a5d).
func (g *Game) addBombWeaponToDesign(p int, d *save.Design, slots *int16, amount int16) int32 {
	w := g.determineBombWeapon(p)
	n := int16(int32(amount) / g.weaponSpace(p, int(w), 1, 1, 0, 0))
	var placed int32
	if n > 0 && *slots < 8 && w != 0 {
		placed = fillSlots(d, slots, n, 99, func(s *save.WeaponSlot) {
			s.Weapon, s.Arc, s.Mods, s.Ammo = w, 0x10, 0, 0xa
		})
	}
	return g.weaponSpace(p, int(w), 1, 1, 0, 0) * placed
}

// addMissileWeaponToDesign is Add_Missile_Weapon_To_Design_ (0x60779):
// missiles with ammo shots, in slots of 20.
func (g *Game) addMissileWeaponToDesign(p int, d *save.Design, slots *int16, amount, ammo int16) int32 {
	w, mods := g.determineMissileWeaponModifications(p, int16(d.Size), amount, &ammo)
	if w == 0 {
		return 0
	}
	n := int16(int32(amount) / g.weaponSpace(p, int(w), 1, 1, ammo, mods))
	if n <= 0 || *slots >= 8 {
		return 0
	}
	placed := fillSlots(d, slots, n, 20, func(s *save.WeaponSlot) {
		s.Weapon, s.Arc, s.Mods, s.Ammo = w, 0x10, uint16(mods), uint8(ammo)
	})
	return g.weaponSpace(p, int(w), 1, 1, ammo, mods) * placed
}

// hiAdd adds v to the high byte of a 16-bit value, without carry out.
func hiAdd(m uint16, v uint8) uint16 { return m&0xff | uint16(uint8(m>>8)+v)<<8 }

// determineMissileWeaponModifications is
// Determine_Missile_Weapon_Modifications_ (0x60362): the missile that does
// most damage per space past the competition's shields, its modifications
// by miniaturisation, then fewer shots and options while too few fit.
func (g *Game) determineMissileWeaponModifications(p int, size, amount int16, ammo *int16) (int16, int16) {
	s := &g.state
	pl := &s.Players[p]
	list, _ := g.knownWeaponsByRank(p, func(w int) bool {
		t := g.wpnU8(w, weaponType)
		return t == 1 || t == 2
	})
	n := len(list)
	if n == 0 {
		return 0, 0
	}
	mods := make([]uint16, n)
	q := s.firstLivingPlayer()
	shield := g.bestShipShieldValue(int16(q))
	cptv := int16(g.getCompetitionPlayerTechValues(q, 0x1c))
	for i := 0; i < n; i++ {
		w := int(list[i])
		if int32(g.wpnI16(w, weaponStrength)) < int32(shield)+4 {
			// The original also zeroes the caller's words at mods+2i.
			list[i], mods[i] = 0, 0
			continue
		}
		ml := min(g.miniaturizationLevel(p, int(g.wpnI16(w, weaponApp))), 5)
		if cptv > 0 && ml >= 1 {
			mods[i] = hiAdd(mods[i], 4)
		}
		if ml >= 1 {
			mods[i] += 0x5820
		}
		if ml >= 2 {
			mods[i] = hiAdd(mods[i], 3)
		}
		if pl.AppStatus[0x3b] == techKnown && ml >= 2 {
			mods[i] = hiAdd(mods[i], 0x20)
		}
		mods[i] &= uint16(g.wpnI16(w, weaponFlags))
	}
	var w int16
	var m uint16
	switch {
	case g.settings[0xd4] < 2:
		return list[n-1], 0
	case n == 1:
		w, m = list[0], mods[0]
	default:
		var best int32
		bi := 0
		for k := 0; k < n; k++ {
			if list[k] == 0 {
				continue
			}
			sp := g.weaponSpace(p, int(list[k]), 1, 1, 5, 0)
			v := (int32(g.wpnI16(int(list[k]), weaponStrength)) - int32(shield)) * 10 / sp
			if v > best {
				best, bi = v, k
			}
		}
		w, m = list[bi], mods[bi]&0x7ff9
	}
	mi := int16(m)
	minCount := g.exeI16(uint32(missileMinCount + 2*int32(size)))
	fits := func() bool {
		return int16(int32(amount)/g.weaponSpace(p, int(w), 1, 1, *ammo, mi)) >= minCount
	}
	if fits() {
		return w, mi
	}
	if *ammo > 5 {
		*ammo = 5
	}
	if fits() {
		return w, mi
	}
	mi &^= 0x800
	if fits() {
		return w, mi
	}
	mi &^= 0x1000
	g.weaponSpace(p, int(w), 1, 1, *ammo, mi)
	return w, mi
}

// addBeamWeaponToDesign is Add_Beam_Weapon_To_Design_ (0x6147f): beams of
// one of three variants (0 point defence, 1 main, 2 heavy), in slots of 99.
func (g *Game) addBeamWeaponToDesign(p int, d *save.Design, slots *int16, amount, kind int16) int32 {
	var weap, mods, arcs [3]int16
	g.determineBeamWeaponModifications(p, int16(d.Size), &weap, &mods, &arcs)
	mods[0] |= 4
	mods[2] |= 2
	k := kind
	n := int16(int32(amount) / g.weaponSpace(p, int(weap[k]), 1, arcs[k], 0, mods[k]))
	if n <= 0 || *slots >= 8 || weap[k] == 0 {
		return 0
	}
	placed := fillSlots(d, slots, n, 99, func(s *save.WeaponSlot) {
		s.Weapon, s.Arc, s.Mods, s.Ammo = weap[k], uint8(arcs[k]), uint16(mods[k]), 0
	})
	return g.weaponSpace(p, int(weap[k]), 1, arcs[k], 0, mods[k]) * placed
}

// determineBeamWeaponModifications is Determine_Beam_Weapon_Modifications_
// (0x60b59): for each variant the beam and modifications to fit, then
// arcs: one Random(100) or two for variant 0, one each for 1 and 2.
func (g *Game) determineBeamWeaponModifications(p int, size int16, weap, mods, arcs *[3]int16) {
	s := &g.state
	pl := &s.Players[p]
	list, _ := g.knownWeaponsByRank(p, func(w int) bool { return g.wpnU8(w, weaponType) == 0 })
	n := len(list)
	if n == 0 {
		weap[2], weap[1], weap[0] = 0, 0, 0
		return
	}
	A, B, C := make([]uint16, n), make([]uint16, n), make([]uint16, n)
	q := s.firstLivingPlayer()
	att := g.exeI16(rowAddr(computerTable, computerStride, g.bestComputer(p), computerBonus)) + int16(pl.Picks[7])
	var defense, shield int16
	if pl.Objective == 100 {
		defense = int16(int8(g.exeU8(0x17feee))) + 20
		shield = 1
	} else {
		shield = g.bestShipShieldValue(int16(q))
		defense = int16(int8(g.exeU8(rowAddr(driveTable, driveStride, g.bestWarpDrive(q), driveDefence))))*5 +
			int16(s.Players[q].Picks[6])
	}
	diff := att - defense
	// The level a modification needs, byte +9 of its row.
	lv := func(row int) int16 { return int16(g.exeU8(uint32(modTable + row*modStride + 9))) }
	for i := 0; i < n; i++ {
		w := int(list[i])
		ml := min(g.miniaturizationLevel(p, int(g.wpnI16(w, weaponApp))), 5)
		d := diff
		if lv(4) <= ml {
			A[i] += 0x10
			if d < 50 {
				d = diff + 25
				B[i] += 0x10
				C[i] += 0x10
			}
		}
		if d > 0 && lv(5) <= ml {
			B[i] += 0x20
			C[i] += 0x20
		}
		if d >= 0 && lv(7) <= ml {
			if d > 30 {
				A[i] += 0x80
			}
			B[i] += 0x80
			C[i] += 0x80
		}
		flags := uint16(g.wpnI16(w, weaponFlags))
		A[i] = (A[i] + 4) & flags
		if A[i]&4 == 0 {
			A[i] = 0
		}
		if lv(3) <= ml {
			B[i] += 8
			C[i] += 8
		}
		if lv(6) <= ml {
			B[i] += 0x40
			C[i] += 0x40
		}
		if lv(8) <= ml {
			B[i] = hiAdd(B[i], 1)
			C[i] = hiAdd(C[i], 1)
		}
		B[i] &= flags
		C[i] = (C[i] + 2) & flags
		if C[i]&2 == 0 {
			C[i] = 0
		}
	}
	flag := func(w int16, bit uint16) bool { return uint16(g.wpnI16(int(w), weaponFlags))&bit != 0 }
	lastWith := func(bit uint16) int {
		i := n - 1
		for i >= 0 && !flag(list[i], bit) {
			i--
		}
		return i
	}
	switch {
	case g.settings[0xd4] < 2:
		if i := lastWith(4); i < 0 {
			weap[0], mods[0] = 0, 0
		} else {
			weap[0], mods[0] = list[i], 4
		}
		weap[1], mods[1] = list[n-1], 0
		if j := lastWith(2); j < 0 {
			weap[2], mods[2] = 0, 0
		} else {
			weap[2], mods[2] = list[j], 2
		}
	case n == 1:
		if flag(list[0], 4) {
			weap[0], mods[0] = list[0], int16(A[0]&0x7ffd)
		} else {
			weap[0] = 0
		}
		weap[1], mods[1] = list[0], int16(B[0]&0x7ff9)
		if flag(list[0], 2) {
			weap[2], mods[2] = list[0], int16(C[0]&0x7ffb)
		} else {
			weap[2] = 0
		}
	default:
		pd := int16(g.bestPointDefenseWeaponBySize(p))
		k := 0
		for k < n && list[k] != pd {
			k++
		}
		if k >= n {
			weap[0] = 0
		} else {
			weap[0], mods[0] = list[k], int16(A[k]&0x7ffd)
		}
		score := func(i int, heavy bool) int32 {
			w := int(list[i])
			str := int32(g.wpnI16(w, weaponStrength))
			fa, fb := g.wpnU8(w, weaponFlagsA), g.wpnU8(w, weaponFlagsB)
			var v int32
			if heavy {
				sp := g.weaponSpace(p, w, 1, 1, 1, 2)
				v = str * 3 / 2
				if fb&1 == 0 {
					v -= int32(shield)
				}
				v = v * 10 / sp
			} else {
				sp := g.weaponSpace(p, w, 1, 1, 1, 0)
				v = str
				if fb&1 == 0 {
					v -= int32(shield)
				}
				v = v * 10 / sp
			}
			if fa&0x20 != 0 {
				v = v * 3 / 2
			}
			if fa&4 != 0 {
				v = v * 5 / 4
			}
			if fb&4 != 0 {
				v = v * 5 / 4
			}
			if fb&2 != 0 {
				v *= 2
			}
			if !heavy && fa&0x10 != 0 && pl.SpecialTheme == 1 {
				v *= 2
			}
			return v
		}
		pick := func(heavy bool) int {
			var best int32
			bi := 0
			for i := 0; i < n; i++ {
				if list[i] == 0 {
					continue
				}
				if v := score(i, heavy); v > best {
					best, bi = v, i
				}
			}
			return bi
		}
		bi := pick(false)
		weap[1], mods[1] = list[bi], int16(B[bi]&0x7ff9)
		bj := pick(true)
		weap[2], mods[2] = list[bj], int16(C[bj]&0x7ffb)
	}
	arcs[1], arcs[2], arcs[0] = 1, 1, 1
	t := (20 - int32(g.getCompetitionPlayerTechValues(p, 0x12))) * int32(size)
	t16 := int32(int16(t))
	bits := func(m int16) int32 {
		var c int32
		for b := int32(8); b < 0x200; b <<= 1 {
			if int32(m)&b != 0 {
				c++
			}
		}
		return c
	}
	c0, c1, c2 := bits(mods[0]), bits(mods[1]), bits(mods[2])
	r := &s.Random
	if int32(r.Random(100)) < t16+c0*20 {
		if int32(r.Random(100)) < c0*20+t16/2 {
			arcs[0] = 0x10
		} else {
			arcs[0] = 2
		}
	}
	if int32(r.Random(100)) < t16/2+c1*20 {
		arcs[1] = 2
	}
	if int32(r.Random(100)) < t16/4+c2*20 {
		arcs[2] = 2
	}
}

// bestPointDefenseWeaponBySize is Best_Point_Defense_Weapon_By_Size_
// (0x56a78): the point-defence beam with the most damage per space, the
// last of equals.
func (g *Game) bestPointDefenseWeaponBySize(p int) int {
	if int16(p) >= 8 {
		return 4
	}
	best, val := 3, int16(0)
	for w := 0; w < 0x28; w++ {
		app := g.wpnI16(w, weaponApp)
		if g.state.Players[p].AppStatus[app] != techKnown || g.wpnU8(w, weaponFlags)&4 == 0 || g.wpnU8(w, weaponType) != 0 {
			continue
		}
		if g.miniaturizationLevel(p, int(app)) < int16(g.exeU8(pdMinMiniaturize)) {
			continue
		}
		if v := int16(g.pointDefenseDamagePerSpace(w, p)); v >= val {
			best, val = w, v
		}
	}
	return best
}

// pointDefenseDamagePerSpace is Point_Defense_Damage_Per_Space_ (0x56a3a).
func (g *Game) pointDefenseDamagePerSpace(w, p int) int32 {
	str := int32(g.wpnI16(w, weaponStrength))
	return str*100/g.weaponSpace(p, w, 1, 1, 1, 4) + str
}

// initShipDesigns is Init_Ship_Designs_ (0x54fbf): six designs (the first
// forced to class 7), an AI's last redone as the doom star; or strategic ones.
// Community patch 1.51: humans keep the sixth (1.40b23: five, slot 5 cleared).
func (g *Game) initShipDesigns(p int) {
	s := &g.state
	pl := &s.Players[p]
	if g.settings[0xd8] == 1 {
		for slot := 0; slot < 6; slot++ {
			pl.Designs[slot] = save.Design{}
			g.autoStrategicDesignShip(p, slot, &pl.Designs[slot])
		}
		return
	}
	// Humans start one hull smaller, except at tech level 2.
	size := int16(1)
	if g.settings[0xd9] != 2 && pl.Objective == 100 {
		size = 0
	}
	g.autoDesignShip(p, &pl.Designs[0], 0, true)
	for slot := 1; slot < 6; slot++ {
		size++
		g.autoDesignShip(p, &pl.Designs[slot], size-1, false)
	}
	if pl.Objective == 100 {
		return
	}
	g.autoDesignShip(p, &pl.Designs[5], 5, false)
	d := &pl.Designs[5]
	g.getShipDesignName(pl.Race, int(d.Size), int(int8(d.Picture))%8, d)
}
