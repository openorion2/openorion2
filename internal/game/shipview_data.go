package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/save"
)

// The ship record the detailed ship view shows (_global_combat_data), how it
// is filled from a ship, and the strategic-combat design update it may make
// first. Spec: docs/screens/ship-view.md.

// Constant tables read from ORION2.EXE (ELF addresses, row strides).
const (
	weaponTable    = 0x17f803 // _weapons, 0x1c-byte rows
	weaponStride   = 0x1c
	weaponApp      = 0x0a // i16 tech application
	weaponType     = 0x0c // 0 beam, 1 missile, 2 fighter, 3 bomb, 4/5 special
	weaponMaxAmmo  = 0x0d
	weaponStrength = 0x16 // i16, compared by the Best_*_Weapon_ routines
	weaponRank     = 0x19 // u8, Best_Strat_Special_Weapon_
	weaponFlags    = 0x1a // u16 allowed modifications; bit 2: point defence capable

	specialTable  = 0x17eee0 // _specials, 0x2f-byte rows
	specialStride = 0x2f
	specialApp    = 0x06
	specialKind   = 0x2c // u8
	specialValue  = 0x2d // i16

	shieldTable    = 0x17f6a1 // _shields, 0x3b-byte rows
	shieldStride   = 0x3b
	shieldApp      = 0x06
	shieldStrength = 0x20

	computerTable  = 0x17fdec // 0x16-byte rows
	computerStride = 0x16
	computerApp    = 0x06
	computerBonus  = 0x14

	driveTable   = 0x17fe70 // 0x2e-byte rows
	driveStride  = 0x2e
	driveApp     = 0x06
	driveSpeed   = 0x2c
	driveCombat  = 0x20 // s8 combat speed by hull size
	driveDefence = 0x22 // s8 beam defence, Base_Generic_Dcv_

	modTable  = 0x17fd0b // weapon modifications, 0x0f-byte rows
	modStride = 0x0f

	crewTable  = 0x17d170 // 8-byte rows by crew level
	crewStride = 8

	hullComputer = 0x04 // i16 computer hits
	hullDrive    = 0x06 // i16 drive hits
	hullShield   = 0x08 // i16 shield hits
	hullBeams    = 0x0a // i16 strategic beam count
	hullMissiles = 0x0c
	hullSpecials = 0x0e
	hullBombs    = 0x10
	hullMarines  = 0x180022 // u8 at hullMarines + size*hullStride

	officerLBXName = 0x182278 // _officer_lbx_name, a pointer
)

// Walks Load_Tech_Names_ makes before each name table after the buildings
// (docs/screens/system-display.md); name i is techName(w+1+i).
const (
	walksSpecials  = techWalksBuildings + 49
	walksArmor     = walksSpecials + 40
	walksShields   = walksArmor + 7
	walksWeapons   = walksShields + 6
	walksMods2     = walksWeapons + 46 + 6 + 7 + 4 + 6 + 15 // 0x17fd0f
	walksWeaponsPl = walksMods2 + 15                        // 0x17f807
)

func (g *Game) exeI16(addr uint32) int16 { return int16(g.exeU16(addr)) }
func (g *Game) exeU8(addr uint32) uint8  { return g.exe.bytes(addr, 1)[0] }

func rowAddr(table, stride uint32, i int, off uint32) uint32 {
	return uint32(int32(table) + int32(i)*int32(stride) + int32(off))
}

func (g *Game) wpnI16(w int, off uint32) int16 {
	return g.exeI16(rowAddr(weaponTable, weaponStride, w, off))
}

func (g *Game) wpnU8(w int, off uint32) uint8 {
	return g.exeU8(rowAddr(weaponTable, weaponStride, w, off))
}

// viewWeapon is one 0xb-byte weapon record of the combat ship.
type viewWeapon struct {
	weapon int16 // +0
	count  uint8 // +2, the slot's working count (+3)
	arc    uint8 // +3
	mods   uint16
	ammo   int16
	ready  uint8 // +8, set to 1
	count2 uint8 // +9, a copy of count
	used   uint8 // +0xa, cleared
}

// viewShip is the part of the combat ship record (_global_combat_data,
// 0x199a12) the view reads; offsets are from that address.
type viewShip struct {
	name         [16]byte      // +0, strcpy of the ship name
	id           int16         // +0x1e
	owner        uint8         // +0x20
	cloaked      uint8         // +0x24, cleared
	size         uint8         // +0x25
	shield       uint8         // +0x26
	shieldBlock  int16         // +0x27, damage blocked
	arcShields   [4]int16      // +0x29
	drive        uint8         // +0x31
	computer     uint8         // +0x32
	armor        uint8         // +0x33
	attack       int16         // +0x34 (Get_Ship_Combat_Bonuses_)
	defense      int16         // +0x36
	evasion      int16         // +0x38
	speed        int16         // +0x3b
	flag4b       uint8         // +0x4b, cleared
	specials     [5]byte       // +0x4c
	weapons      [8]viewWeapon // +0x52
	structure    int16         // +0xaa
	crew         uint8         // +0xac
	officer      int16         // +0xad
	marines      uint8         // +0xaf, 0xff for none
	captured     uint8         // +0xb0, written by combat only
	damaged      [5]byte       // +0xb2
	weaponOK     [8]uint8      // +0xb8
	structDamage int16         // +0xc0
	armorHits    int16         // +0xc2
	picture      int16         // +0xc4
	driveMax     int16         // +0xca
	computerMax  int16         // +0xcc
	shieldMax    int16         // +0xfe
	driveCur     int16         // +0x100
	computerCur  int16         // +0x102
	shieldCur    int16         // +0x134
	builder      uint8         // +0x138
}

// shipViewOffMinus666 is the drive value (+0x100) combat stores for a
// destroyed-drive ship; the view then hides several lines.
const shipViewOffMinus666 = -0x29a

// loadDisplayCombatShip is Load_Display_Combat_Ship_ (0x4cb1e).
func (g *Game) loadDisplayCombatShip(v *viewShip, id int) error {
	if g.settings[0xd8] == 1 {
		g.updateStrategicShip(id)
	}
	s := &g.state.Ships[id]
	d := &s.Design
	n := copy(v.name[:], save.CString(d.Name[:]))
	if n < len(v.name) {
		v.name[n] = 0
	}
	v.owner = uint8(s.Owner)
	v.builder = d.Builder
	v.cloaked = 0
	v.shield = d.Shield
	v.computer = d.Computer
	v.drive = d.Drive
	v.size = d.Size
	v.id = int16(id)
	v.specials = d.Specials
	v.damaged = s.DamagedSpecials
	for k := range v.weapons {
		w := &v.weapons[k]
		if d.Type != 0 {
			w.weapon = 0
			continue
		}
		sl := &d.Weapons[k]
		w.weapon, w.count, w.arc, w.mods, w.ammo = sl.Weapon, sl.Working, sl.Arc, sl.Mods, int16(sl.Ammo)
		if w.arc == 0x10 {
			w.arc = 0xf
		}
		w.count2 = w.count
		w.ready = 1
		switch g.exeU8(rowAddr(weaponTable, weaponStride, int(w.weapon), weaponType)) {
		case 2:
			w.ammo = 0
		case 1:
		default:
			w.ammo = int16(g.exeU8(rowAddr(weaponTable, weaponStride, int(w.weapon), weaponMaxAmmo)))
		}
		if w.weapon == 0x21 {
			w.ammo *= int16(w.count)
		}
		w.used = 0
	}
	for k := range v.weaponOK {
		v.weaponOK[k] = 1
	}
	v.crew = s.CrewLevel
	v.officer = s.Officer
	if d.Type == 0 && s.Owner < 9 {
		v.marines = g.exeU8(hullMarines + uint32(v.size)*hullStride)
		if testBit(v.specials[:], 0x24) && !testBit(v.damaged[:], 0x24) {
			v.marines <<= 1
		}
	} else {
		v.marines = 0xff
	}
	v.structure = int16(g.shipStructure(id))
	v.armorHits = int16(g.shipArmorHits(id)) - s.ArmorDamage
	v.armor = d.Armor
	v.structDamage = s.StructureDamage
	if v.size == 5 {
		v.picture = 0x2b
	} else {
		v.picture = int16(int8(d.Picture))
	}
	v.flag4b = 0
	g.initInternalSpace(v)
	v.shieldCur = int16((100 - int(s.ShieldDamage)) * int(v.shieldMax) / 100)
	v.driveCur = int16((100 - int(s.DriveDamage)) * int(v.driveMax) / 100)
	v.computerCur = v.computerMax - int16(s.ComputerDamage)
	if v.shield == 0 {
		v.shieldBlock = 0
	} else {
		m := int16(v.size) + 1
		if v.size >= 5 {
			m++
		}
		v.shieldBlock = g.exeI16(rowAddr(shieldTable, shieldStride, int(v.shield), shieldStrength)) * 5 * m
		if testBit(v.specials[:], 0x14) && !testBit(v.damaged[:], 0x14) {
			v.shieldBlock = int16(int(v.shieldBlock) * 3 / 2)
		}
		if v.shieldMax == 0 {
			return fmt.Errorf("Load_Display_Combat_Ship_: shield hits 0 (divide by zero)")
		}
		v.shieldBlock = int16((int(v.shieldCur)*int(v.shieldBlock) + int(v.shieldMax) - 1) / int(v.shieldMax))
	}
	for k := range v.arcShields {
		v.arcShields[k] = v.shieldBlock
	}
	v.speed = int16(d.CombatSpeed)
	if testBit(d.Specials[:], 2) && !testBit(s.DamagedSpecials[:], 2) {
		v.speed += 5
	}
	a := int(v.driveMax) * 2 / 3
	if int(v.driveMax)-a == 0 {
		return fmt.Errorf("Load_Display_Combat_Ship_: no drive hits (divide by zero)")
	}
	v.speed = int16(int(v.speed) * (int(v.driveCur) - a) / (int(v.driveMax) - a))
	if v.speed < 0 {
		v.speed = 0
	}
	if v.owner < 8 && g.state.Players[v.owner].Picks[save.PickTransDimensional] != 0 {
		v.speed += 4
	}
	return nil
}

// initInternalSpace is the part of Init_Internal_Space_ (0x36470) the view
// reads: the drive, computer and shield hits. The per-weapon and special
// space tables it also fills are not ported (see the spec).
func (g *Game) initInternalSpace(v *viewShip) {
	sz := v.size
	if v.drive == 0 {
		v.driveMax = 0
	} else {
		v.driveMax = g.hullField(sz, hullDrive)
	}
	if testBit(v.specials[:], 0x1b) && !testBit(v.damaged[:], 0x1b) {
		v.driveMax *= 3
	}
	v.computerMax = g.hullField(sz, hullComputer)
	if v.shield == 0 {
		v.shieldMax = 0
	} else {
		v.shieldMax = g.hullField(sz, hullShield)
	}
	// The 0x36 bytes from +0xca are copied to +0x100: current = maximum.
	v.driveCur, v.computerCur = v.driveMax, v.computerMax
}

// getShipCombatBonuses is Get_Ship_Combat_Bonuses_ (0x54e5b): beam
// defense, missile evasion and beam attack, all 16-bit.
func (g *Game) getShipCombatBonuses(id int) (defense, evasion, attack int16) {
	s := &g.state.Ships[id]
	defense, evasion, attack = g.designCombatBonuses(&s.Design)
	if s.Officer > -1 {
		h := g.officerBonus(int(s.Officer), 0)
		defense += h
		evasion += h / 2
		attack += g.officerBonus(int(s.Officer), 1)
	}
	c := rowAddr(crewTable, crewStride, int(s.CrewLevel), 0)
	cd := g.exeI16(c + 2)
	defense += cd
	evasion += cd / 2
	attack += g.exeI16(c)
	if s.Owner < 8 {
		p := &g.state.Players[s.Owner]
		pd := int16(p.Picks[save.PickShipDefense])
		defense += pd
		evasion += pd / 2
		attack += int16(p.Picks[save.PickShipAttack])
		if p.Picks[save.PickTransDimensional] != 0 {
			defense += 0x14
		}
	}
	return
}

// designCombatBonuses is Get_Design_Combat_Bonuses_ (0x54d80).
func (g *Game) designCombatBonuses(d *save.Design) (defense, evasion, attack int16) {
	sp := d.Specials[:]
	attack = g.exeI16(rowAddr(computerTable, computerStride, int(d.Computer), computerBonus))
	if testBit(sp, 5) {
		attack += 0x32
	}
	defense = int16(d.CombatSpeed)
	if testBit(sp, 2) {
		defense += 5
	}
	defense *= 5
	if testBit(sp, 9) {
		evasion += 0x46
	}
	if testBit(sp, 0x11) {
		defense += 0x64
		evasion += 0x32
	}
	if testBit(sp, 0x12) {
		defense += 0x32
		evasion += 0x19
	}
	if testBit(sp, 0x15) {
		evasion += 0x64
	}
	if testBit(sp, 0x26) {
		evasion += 0x82
	}
	return
}

// officerBonus is Helmsman_Bonus_ (byte 0, 0x3677d) or Weaponry_Bonus_
// (byte 1, 0x367ae): special skill bit 6 gives (level+1)*5, else bit 7
// (level+1)*15/2.
func (g *Game) officerBonus(o, b int) int16 {
	l := &g.state.Leaders[o]
	sk := byte(l.SpecialSkills >> (8 * b))
	switch {
	case sk&0x40 != 0:
		return (g.state.ownedOfficerLevel(o) + 1) * 5
	case sk&0x80 != 0:
		return int16((int(g.state.ownedOfficerLevel(o)) + 1) * 15 / 2)
	}
	return 0
}

// crewQualityString is Crew_Quality_String_ (0x33cfa).
func (g *Game) crewQualityString(level uint8) (string, error) {
	vals := [5]int{0, 100, 200, 300, 300}
	if level > 4 {
		return "", nil
	}
	f, err := g.kenText(0x1f + int(level))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(f, vals[level]), nil
}

// kenText is Ken_Get_Text_Message_ (0x58f1e) with Get_Text_Message_
// (0x249f9): record 0 of entry n*6+language of KENTEXT.LBX, or of
// KENTEXT1.LBX from n = 0x50; the callers' buffer is 0x50 bytes.
func (g *Game) kenText(n int) (string, error) { return g.kenTextMax(n, 0x50) }

// kenTextMax is kenText for a caller's buffer of max bytes (EBX).
func (g *Game) kenTextMax(n, max int) (string, error) {
	file := "KENTEXT.LBX"
	if n >= 0x50 {
		file, n = "KENTEXT1.LBX", n-0x50
	}
	lang := int(g.settings[0xd2])
	if lang > 5 {
		lang = 0
	}
	a, err := g.archive(file)
	if err != nil {
		return "", err
	}
	t, err := a.Table(lang + n*6)
	if err != nil {
		return "", err
	}
	r := t.Record(0)
	s := save.CString(r[:min(len(r), 100)])
	if len(s) >= max-1 || len(s) >= 99 {
		return "", fmt.Errorf("Message %d in %s too long (%d)", n, file, len(s))
	}
	return s, nil
}

// playerKnows tests player p's tech application app (player+0x117+app == 3).
func (g *Game) playerKnows(p int, app int16) bool {
	a := &g.state.Players[p].AppStatus
	if int(app) < 0 || int(app) >= len(a) {
		return false
	}
	return a[app] == 3
}

// bestShipShield is Best_Ship_Shield_ (0x5679e).
func (g *Game) bestShipShield(owner int) int {
	switch {
	case int16(owner) < 8:
		best := 0
		for i := 1; i < 6; i++ {
			if g.playerKnows(owner, g.exeI16(rowAddr(shieldTable, shieldStride, i, shieldApp))) {
				best = i
			}
		}
		return best
	case uint16(owner) == 8:
		return 7
	case uint16(owner) == 9:
		return 3
	}
	return 0
}

// bestWarpDrive is Best_Warp_Drive_ (0x56726); Antarans and monsters
// (8..14) have fixed drives.
func (g *Game) bestWarpDrive(p int) int {
	if int16(p) >= 8 {
		if k := uint16(p) - 8; k <= 6 {
			return [7]int{6, 4, 2, 4, 6, 6, 2}[k]
		}
		return 0
	}
	best := 0
	for i := 1; i < 7; i++ {
		if g.playerKnows(p, g.exeI16(rowAddr(driveTable, driveStride, i, driveApp))) {
			best = i
		}
	}
	return best
}

// bestComputer is Best_Computer_ (0x5680d); Antarans and monsters get 5.
func (g *Game) bestComputer(p int) int {
	if int16(p) >= 8 {
		return 5
	}
	best := 0
	for i := 1; i < 6; i++ {
		if g.playerKnows(p, g.exeI16(rowAddr(computerTable, computerStride, i, computerApp))) {
			best = i
		}
	}
	return best
}

// bestWeapon is the player loop of Best_Beam_Weapon_ (0x568eb, types
// {0}, default 3), Best_Missile_Weapon_ (0x56b26, {1, 2}, default 0xe) and
// Best_Bomb_Weapon_ (0x56ca2, {3}, default 0x15): the known weapon of rows
// 0..0x27 with the greatest i16 at +0x16, the first on ties.
func (g *Game) bestWeapon(p, def int, types ...uint8) int {
	best, val := def, int16(0)
	for i := 0; i < 0x28; i++ {
		if !g.playerKnows(p, g.exeI16(rowAddr(weaponTable, weaponStride, i, weaponApp))) {
			continue
		}
		t := g.exeU8(rowAddr(weaponTable, weaponStride, i, weaponType))
		ok := false
		for _, x := range types {
			ok = ok || t == x
		}
		if v := g.exeI16(rowAddr(weaponTable, weaponStride, i, weaponStrength)); ok && val < v {
			best, val = i, v
		}
	}
	return best
}

// bestBombWeapon is Best_Bomb_Weapon_ (0x56ca2).
func (g *Game) bestBombWeapon(p int) int {
	if int16(p) >= 8 {
		return 0x18
	}
	return g.bestWeapon(p, 0x15, 3)
}

// bestStratSpecialWeapon is Best_Strat_Special_Weapon_ (0x56d3a) for
// players: types 4 and 5 ranked by the byte at +0x19; weapon 0x25 needs
// hull size > 3, 0x26 size > 4.
func (g *Game) bestStratSpecialWeapon(p, size int) int {
	best, val := 0, uint8(0)
	for i := 0; i < 0x28; i++ {
		if (i == 0x25 && int16(size) <= 3) || (i == 0x26 && int16(size) <= 4) {
			continue
		}
		if !g.playerKnows(p, g.exeI16(rowAddr(weaponTable, weaponStride, i, weaponApp))) {
			continue
		}
		t := g.exeU8(rowAddr(weaponTable, weaponStride, i, weaponType))
		if r := g.exeU8(rowAddr(weaponTable, weaponStride, i, weaponRank)); (t == 5 || t == 4) && val < r {
			best, val = i, r
		}
	}
	return best
}

// bestSpecial is the loop shared by Best_Beam_To_Hit_Special_ (kinds {1}),
// Best_Beam_Damage_Special_ ({2, 7}), Best_Beam_Defense_Special_ ({3, 6}),
// Best_Missile_Defense_Special_ ({4, 6}) and Best_Missile_Damage_Special_
// ({5, 7}, which turns 0x22 into 0): known specials 0..0x26 by value.
func (g *Game) bestSpecial(p int, kinds ...uint8) int {
	best, val := 0, int16(0)
	for i := 0; i < 0x27; i++ {
		if !g.playerKnows(p, g.exeI16(rowAddr(specialTable, specialStride, i, specialApp))) {
			continue
		}
		k := g.exeU8(rowAddr(specialTable, specialStride, i, specialKind))
		ok := false
		for _, x := range kinds {
			ok = ok || k == x
		}
		if v := g.exeI16(rowAddr(specialTable, specialStride, i, specialValue)); ok && val < v {
			best, val = i, v
		}
	}
	return best
}

// updateStrategicShip is Update_Strategic_Ship_ (0x5794b): with settings
// 0xd8 set, a player's ship is refitted in place with the owner's best
// equipment. It changes the save state.
func (g *Game) updateStrategicShip(id int) {
	s := &g.state.Ships[id]
	p := int(s.Owner)
	if int16(p) >= 8 || g.settings[0xd8] == 0 {
		return
	}
	d := &s.Design
	d.Drive = uint8(g.bestWarpDrive(p))
	d.FTLSpeed = uint8(g.ftlSpeed(p, int(d.Drive)))
	d.Shield = uint8(g.bestShipShield(p))
	d.Computer = uint8(g.bestComputer(p))
	d.Armor = uint8(g.bestArmor(p))
	g.updateStrategicDesignWeaponsSpecials(d, p, int(d.Size))
}

// ftlSpeed is Get_FTL_Speed_ (0x575d6).
func (g *Game) ftlSpeed(p, drive int) int {
	if int16(p) < 8 {
		v := int(g.exeU16(rowAddr(driveTable, driveStride, drive, driveSpeed)))
		if g.state.Players[p].Picks[save.PickTransDimensional] != 0 {
			v += 2
		}
		return v
	}
	if int16(drive) > 0 {
		return 1
	}
	return 0
}

// stratBeamWeaponCount is Strat_Beam_Weapon_Count_ (0x57617): the hull's
// beam count, half again with tech application 89 (player+0x170).
func (g *Game) stratBeamWeaponCount(p, size int) int16 {
	n := g.hullField(uint8(size), hullBeams)
	if int16(p) < 8 && g.state.Players[p].AppStatus[89] == 3 {
		n = int16(int(n) * 3 / 2)
	}
	return n
}

// updateStrategicDesignWeaponsSpecials is
// Update_Strategic_Design_Weapons_Specials_ (0x57675) for players.
func (g *Game) updateStrategicDesignWeaponsSpecials(d *save.Design, p, size int) {
	slot := 0
	put := func(weapon int, count int16, ammo uint8) {
		w := &d.Weapons[slot]
		w.Arc, w.Mods, w.Ammo = 0x10, 0, ammo
		w.Weapon = int16(weapon)
		w.Count, w.Working = uint8(count), uint8(count)
	}
	if b, n := int16(g.bestWeapon(p, 3, 0)), g.stratBeamWeaponCount(p, size); b > 0 && n > 0 {
		put(int(b), n, 5)
		slot++
	}
	if m, n := int16(g.bestWeapon(p, 0xe, 1, 2)), g.hullField(uint8(size), hullMissiles); m > 0 && n > 0 {
		put(int(m), n, 5)
		slot++
	}
	if w, n := int16(g.bestStratSpecialWeapon(p, size)), g.hullField(uint8(size), hullSpecials); w > 0 && n > 0 {
		put(int(w), n, 5)
		slot++
	}
	if b, n := int16(g.bestWeapon(p, 0x15, 3)), g.hullField(uint8(size), hullBombs); b > 0 && n > 0 {
		put(int(b), n, 10)
	}
	set := func(bit int) {
		if int16(bit) > 0 {
			d.Specials[bit>>3] |= 1 << (bit & 7)
		}
	}
	set(g.bestSpecial(p, 1))
	set(g.bestSpecial(p, 2, 7))
	md := g.bestSpecial(p, 5, 7)
	if md == 0x22 {
		md = 0
	}
	set(md)
	if g.state.Players[p].AppStatus[81] == 3 {
		set(0xd)
	}
	set(g.bestSpecial(p, 3, 6))
	set(g.bestSpecial(p, 4, 6))
}
