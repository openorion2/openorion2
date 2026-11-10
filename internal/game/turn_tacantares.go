package game

import "encoding/binary"

// The Antaran home fleet in tactical combat (COMBINIT.C, ANTARANS.C):
// Load_Antaran_Defense_Fleet_, the five ship loaders, the star fortress and
// Antares itself as record 0. Spec: docs/combat/tactical.md, "Antares".

// Image tables the loaders read.
const (
	antHullMarines   = 0x180022 // hull table: marines, armour, structure by size (stride 0x24)
	antHullArmor     = 0x180024
	antHullStructure = 0x180026
	antHullStride    = 0x24
	antArmorHitsPct  = 0x17f642 // armour table +4 (stride 0xf)
	antDriveSpeed    = 0x17fe90 // combat speed by drive (stride 0x2e) and size
	antFortressSpace = 0x180140 // hull row 8: space, marines, armour, structure
	antaresName      = 0x178e8a
)

// antSlot is one weapon mount a loader writes.
type antSlot struct {
	weapon, mods, ammo int16
	count, arc         uint8
}

// antShip is what one Load_*_Antaran_Combat_Ship_ writes.
type antShip struct {
	name      uint32 // the name in the image
	nameLen   int    // the bytes the loader copies
	slots     []antSlot
	clearFrom int // mounts cleared after the writes, from this one on
	big       bool
	picture   int16
}

// antaranShips are the five loaders by size. Some clear a mount they just
// wrote (small 1, large 4, titan 7).
var antaranShips = [5]antShip{
	// Load_Small_Antaran_Combat_Ship_ (0x55161).
	{0x178ea0, 7, []antSlot{{4, 0, -1, 2, 1}, {0x18, 0, -1, 1, 0xf}}, 1, false, 0},
	// Load_Medium_Antaran_Combat_Ship_ (0x5542c).
	{0x178ea7, 9, []antSlot{{4, 0, -1, 3, 1}, {4, 2, -1, 1, 1}, {0x18, 0, 0xa, 1, 0xf}}, 3, false, 1},
	// Load_Large_Antaran_Combat_Ship_ (0x55738).
	{0x178eb0, 9, []antSlot{{4, 0, -1, 4, 1}, {4, 2, -1, 2, 1}, {0x18, 0, 0xa, 5, 0xf}, {0xd, 0, -1, 2, 0xf},
		{0x1f, 0, -1, 3, 0xf}}, 4, false, 2},
	// Load_Huge_Antaran_Combat_Ship_ (0x55b12).
	{0x178eb9, 12, []antSlot{{4, 0, -1, 6, 1}, {4, 2, -1, 2, 1}, {0x18, 0, 0xa, 0xf, 0xf}, {0xd, 0, -1, 2, 0xf},
		{4, 4, -1, 8, 1}, {0xb, 0, -1, 2, 1}}, 6, true, 3},
	// Load_Titan_Antaran_Combat_Ship_ (0x55f67).
	{0x178ec5, 10, []antSlot{{4, 0, -1, 0xa, 1}, {4, 2, -1, 2, 1}, {0x18, 0, 0xa, 0x14, 0xf}, {0xd, 0, -1, 3, 0xf},
		{4, 4, -1, 0xf, 1}, {0xb, 2, -1, 2, 1}, {0x25, 0, -1, 1, 0xf}, {0x1f, 0, -1, 6, 0xf}}, 7, true, 4},
}

// setWeaponMount writes a mount as the loaders do: on, all shots left.
func setWeaponMount(t *tship, k int, s antSlot) {
	t.set16(tw(k, twWeapon), s.weapon)
	t.set8(tw(k, twCount), s.count)
	t.set8(tw(k, twArc), s.arc)
	t.set16(tw(k, twMods), s.mods)
	t.set16(tw(k, twAmmo), s.ammo)
	t.set8(tw(k, twOn), 1)
	t.set8(tw(k, twLeft), s.count)
	t.set8(tw(k, twFired), 0)
}

// loadAntaranDefenseFleet is Load_Antaran_Defense_Fleet_ (0x4d141): the
// ships guarding Antares by size (at least one titan), then the fortress.
func (g *Game) loadAntaranDefenseFleet() {
	s := &g.state
	tc := g.tc
	if s.antWord(0xe) < 1 {
		s.setAntWord(0xe, 1)
	}
	for size := 0; size < 5; size++ {
		for k := int16(0); k < s.antWord(6+2*size); k++ {
			g.loadCombatAntaranShip(tc.n, size)
			tc.n++
		}
	}
	g.loadAntaranStarFortress()
}

// loadCombatAntaranShip is Load_Combat_Antaran_Ship_ (0x4d10e).
func (g *Game) loadCombatAntaranShip(i int16, size int) {
	a := &antaranShips[size]
	t := g.tsp(i)
	copy(t[:a.nameLen], g.exe.bytes(a.name, a.nameLen))
	t.set8(tsOwner, 8)
	t.set8(tsBuilder, 8)
	// community patch 1.51: 0xfeff, no ship and no satellite (1.40: -1).
	t.set16(tsID, -0x101)
	t.set8(tsShield, 0)
	t.set8(tsComputer, 5)
	t.set8(tsDrive, 5)
	t.set8(tsSize, uint8(size))
	t.set8(tsGone, 0)
	for k := 0; k < 5; k++ {
		t.set8(tsSpecials+k, 0)
		t.set8(tsDamaged+k, 0)
	}
	for k, sl := range a.slots {
		setWeaponMount(t, k, sl)
	}
	for k := a.clearFrom; k < 8; k++ {
		clear(t[tw(k, 0) : tw(k, 0)+0xb])
	}
	sp := t[tsSpecials : tsSpecials+5]
	setBit(sp, 7)
	setBit(sp, 0x18)
	if a.big {
		setBit(sp, 0x21)
		setBit(sp, 0x1a)
	}
	if b := g.antaranDifficultySpecial(size); b != 0 {
		setBit(sp, b)
	}
	t.set8(tsCrew, 3)
	t.set16(tsOfficer, -1)
	t.set8(tsArmor, 6)
	t.set8(tsMarines, g.exeU8(antHullMarines+uint32(size)*antHullStride))
	pct := int32(g.exeI16(antArmorHitsPct + 6*0xf))
	t.set16(tsStruct, int16(int32(g.exeI16(antHullStructure+uint32(size)*antHullStride))*(pct+100)/100))
	t.set16(tsArmorHits, int16((pct+100)*int32(g.exeI16(antHullArmor+uint32(size)*antHullStride))/100))
	t.set16(tsStructDmg, 0)
	t.set16(tsPicture, a.picture)
	t.set8(tsCSpeed, g.exeU8(antDriveSpeed+uint32(t.u8(tsDrive))*0x2e+uint32(size)))
	t.set8(0x4b, 0)
	g.initInternalSpaceRec(t)
}

// antaranDifficultySpecial is the loaders' special by difficulty (settings
// 0xd4), 0 for none.
func (g *Game) antaranDifficultySpecial(size int) int {
	d := g.settings[0xd4]
	switch size {
	case 0:
		if d > 2 {
			return 9
		}
	case 1:
		switch {
		case d > 3:
			return 0x15
		case d > 2:
			return 9
		}
	case 2:
		switch d {
		case 2:
			return 9
		case 3:
			return 0x15
		case 4:
			return 0x26
		}
	case 3:
		switch {
		case d == 1:
			return 9
		case d == 2:
			return 0x15
		case d == 3 || d == 4:
			return 0x26
		}
	case 4:
		switch {
		case d == 0:
			return 9
		case d == 1:
			return 0x15
		case d >= 2 && d <= 4:
			return 0x26
		}
	}
	return 0
}

// loadAntaranStarFortress is Load_Antaran_Star_Fortress_ (0x4d18e): a
// size-4 record with the fortress hull's structure and armour tripled and
// its space spent on four kinds of mounts, at most 99 a mount.
func (g *Game) loadAntaranStarFortress() {
	tc := g.tc
	t := g.tsp(tc.n)
	t.set8(tsComputer, 5)
	t.set8(tsArmor, 6)
	// 0x18015a is hull row 8's name pointer, which Load_Tech_Names_ sets at
	// start-up (see hullName): the image's word points at nothing.
	name := g.must(g.hullName(8, false))
	copy(t[:], name)
	t[len(name)] = 0
	pct := int32(g.exeI16(antArmorHitsPct + uint32(t.u8(tsArmor))*0xf))
	h := int32(g.exeI16(antFortressSpace + 6))
	t.set16(tsStruct, int16(pct*h/100+h)*3)
	h = int32(g.exeI16(antFortressSpace + 4))
	t.set16(tsArmorHits, int16(pct*h/100+h)*3)
	space := int32(int16(int32(g.exeI16(antFortressSpace))*5/4)) / 2
	t.set8(tsSize, 4)
	t.set16(tsPicture, 5)
	t.set16(tsID, -1)
	t.set8(tsDrive, 0)
	t.set8(tsShield, 0)
	t.set8(tsGone, 0)
	t.set8(tsOwner, uint8(tc.def))
	t.set8(tsBuilder, uint8(tc.def))
	for k := 0; k < 5; k++ {
		t.set8(tsSpecials+k, 0)
		t.set8(tsDamaged+k, 0)
	}
	sp := t[tsSpecials : tsSpecials+5]
	for _, b := range []int{5, 7, 0x18, 0x1a, 0xe, 0x1b, 0x1d, 0x20} {
		setBit(sp, b)
	}
	for k := 0; k < 8; k++ {
		clear(t[tw(k, 0) : tw(k, 0)+0xb])
	}
	mount := func(k int, w, mods int16) {
		t.set16(tw(k, twWeapon), w)
		t.set8(tw(k, twOn), 1)
		t.set16(tw(k, twAmmo), -1)
		t.set8(tw(k, twArc), 0xf)
		t.set16(tw(k, twMods), mods)
	}
	count := func(k int, n int16) {
		t.set8(tw(k, twCount), uint8(n))
		t.set8(tw(k, twLeft), uint8(n))
	}
	ws := func(w, mods int16) int32 { return g.weaponSpace(int(tc.def), int(w), 1, 1, 0, mods) }
	// A run of mounts: the space's share in weapons, overflowing past 99.
	run := func(k int, w, mods int16, share int32, limit int16) int {
		mount(k, w, mods)
		n := int16(share / ws(w, mods))
		if n > limit {
			n = limit
		}
		for n > 99 {
			count(k, 99)
			k++
			mount(k, w, mods)
			n -= 99
		}
		count(k, n)
		return k + 1
	}
	k := run(0, 0xb, 2, space/2, 99)
	k = run(k, 4, 0, space/4, 0xc6)
	k = run(k, 4, 4, space/4, 0xc6)
	run(k, 4, 2, space/2, 99)
	for k := 0; k < 8; k++ {
		t.set8(tsSpecialOn+k, 1)
	}
	t.set8(tsCrew, 3)
	t.set16(tsOfficer, 0x42)
	t.set16(tsSpeed, 0)
	t.set16(tsStructDmg, 0)
	t.set8(0x4b, 0)
	t.set8(tsMarines, g.exeU8(antFortressSpace+2))
	g.initInternalSpaceRec(t)
	t.set16(tsShieldMax, 0)
	for k := 0; k < 4; k++ {
		t.set16(tsArc+2*k, t.i16(tsShieldMax))
	}
	tc.n++
}

// loadAntaranPlanet is Load_Antaran_Planet_ (0x4a8d3): Antares as record
// 0, and with the screen its palette and pictures.
func (g *Game) loadAntaranPlanet() error {
	tc := g.tc
	t := g.tsp(0)
	*t = tship{}
	copy(t[:8], g.exe.bytes(antaresName, 8))
	t.set16(tsID, -1)
	t.set16(tsOfficer, -1)
	t.set8(tsSize, 5)
	t.set8(tsGone, 0)
	t.set8(tsOwner, 8)
	t.set8(tsBuilder, 8)
	t.set8(tsDrive, 0)
	t.set8(0x4b, 0)
	t.set8(tsShield, 0)
	t.set16(tsShieldMax, 0)
	for k := 0; k < 4; k++ {
		t.set16(tsArc+2*k, 0)
	}
	t.set8(tsComputer, 5)
	t.set8(tsArmor, 6)
	for k := 0; k < 5; k++ {
		t.set8(tsSpecials+k, 0)
		t.set8(tsDamaged+k, 0)
	}
	g.initInternalSpaceRec(t)
	binary.LittleEndian.PutUint16(t[tsShieldHP:], 0)
	binary.LittleEndian.PutUint16(t[0x134:], 0)
	if tc.scr == nil {
		return nil
	}
	lbx := g.cmbtLBX(cmbtPlntLBX)
	pal, err := g.cached(lbx, 0x3e)
	if err != nil {
		return err
	}
	g.e.DrawPalette(pal)
	g.e.RefreshPalette()
	if tc.scr.planet, err = g.picture(lbx, 0x3c); err != nil {
		return err
	}
	tc.scr.planet2, err = g.picture(lbx, 0x3d)
	return err
}
