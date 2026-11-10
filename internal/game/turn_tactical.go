package game

import (
	"encoding/binary"
	"unsafe"

	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// The tactical combat engine without its screen (COMBAT.C, COMBINIT.C,
// COMBAT1.C): what Tactical_Bombardment_ runs when settings 0xd8 is 0.
// Spec: docs/turn/combat.md.

// tship is one 0x139-byte combat ship record (0x192864); record 0 is the
// planet. Offsets are named in tsOff*.
type tship [0x139]byte

// Combat ship record offsets.
const (
	tsID        = 0x1e // i16 ship index, -1 for the planet
	tsOwner     = 0x20 // u8
	tsX, tsY    = 0x21, 0x22
	tsFacing    = 0x23
	tsGone      = 0x24 // u8 retreated or cloaked away
	tsSize      = 0x25 // u8 hull, 5 for the planet
	tsShield    = 0x26 // u8 class (planet: building id)
	tsShieldMax = 0x27 // i16 damage one arc blocks
	tsArc       = 0x29 // 4 x i16 shield arcs
	tsDrive     = 0x31 // u8
	tsComputer  = 0x32 // u8
	tsArmor     = 0x33 // u8
	tsOcv       = 0x34 // i16
	tsDcv       = 0x36 // i16 beam defence
	tsEvasion   = 0x38 // i16 missile defence
	tsCSpeed    = 0x3a // u8 design combat speed
	tsSpeed     = 0x3b // i16
	tsCloak     = 0x40 // u8 cloak state
	tsAbsorbed  = 0x45 // i16 energy absorber charge
	tsStasis    = 0x4a // u8 holder, 0xff none
	tsSpecials  = 0x4c // 5 bytes, bit set
	tsWeapons   = 0x52 // 8 x 0xb
	tsStruct    = 0xaa // i16 structure (planet: damage taken)
	tsCrew      = 0xac // u8
	tsOfficer   = 0xad // i16
	tsMarines   = 0xaf // u8, 0xff none
	tsBoarded   = 0xb0 // u8
	tsDamaged   = 0xb2 // 5 bytes damaged specials
	tsSpecialOn = 0xb8 // 8 bytes
	tsStructDmg = 0xc0 // i16
	tsArmorHits = 0xc2 // i16
	tsPicture   = 0xc4 // i16
	tsDriveMax  = 0xca // i16, then computer, weapons, specials, shield
	tsCompMax   = 0xcc
	tsWpnSpace  = 0xce  // 8 x i16 weapon hits
	tsWpnUnit   = 0xde  // 8 x i16 hits per weapon
	tsSpcSpace  = 0xee  // 8 x i16 special hits
	tsShieldHP  = 0xfe  // i16
	tsCur       = 0x100 // 0x36 bytes: current values of 0xca..0xff
	tsDriveCur  = 0x100
	tsCompCur   = 0x102
	tsShieldCur = 0x134
	tsBuilder   = 0x138
)

// Weapon record fields (0xb bytes from tsWeapons).
const (
	twWeapon = 0 // i16
	twCount  = 2 // u8 working mounts
	twArc    = 3
	twMods   = 4 // i16
	twAmmo   = 6 // i16
	twOn     = 8 // u8 1 enabled
	twLeft   = 9 // u8 shots left this turn
	twFired  = 0xa
)

func (t *tship) u8(o int) uint8       { return t[o] }
func (t *tship) set8(o int, v uint8)  { t[o] = v }
func (t *tship) i16(o int) int16      { return int16(binary.LittleEndian.Uint16(t[o:])) }
func (t *tship) set16(o int, v int16) { binary.LittleEndian.PutUint16(t[o:], uint16(v)) }
func (t *tship) add16(o int, v int16) { t.set16(o, t.i16(o)+v) }
func tw(k int, f int) int             { return tsWeapons + 0xb*k + f }

// satellite: a star base, battle station, star fortress or the planet. community
// patch 1.51 tells them by ship id -1 (high byte 0xff), not by drive 0 as 1.40 did.
func (t *tship) satellite() bool { return t.u8(tsID+1) == 0xff }

// tmissile is one 0x1a-byte missile record (0x192b14, 300 of them).
type tmissile [0x1a]byte

// Missile record offsets.
const (
	tmWeapon  = 0 // i16, 0 free
	tmOwner   = 2 // i16 player
	tmShip    = 4 // i16 firing record
	tmTarget  = 6 // i16 record or missile
	tmAtMis   = 8 // u8 1 when the target is a missile
	tmX, tmY  = 9, 0xb
	tmFuel    = 0xd  // i16
	tmCount   = 0xf  // i16
	tmMods    = 0x11 // i16
	tmTravel  = 0x13 // i16
	tmFace    = 0x15
	tmSpeed   = 0x16
	tmBombs   = 0x17 // u8 fighter bombs left (0xff for some)
	tmLive    = 0x19 // u8
	nMissiles = 300
)

func (m *tmissile) i16(o int) int16      { return int16(binary.LittleEndian.Uint16(m[o:])) }
func (m *tmissile) set16(o int, v int16) { binary.LittleEndian.PutUint16(m[o:], uint16(v)) }

// tactical is the engine's state: records, missiles and the globals
// around them.
type tactical struct {
	// mem is the two blocks the engine allocates on 0x192ed4 and what follows:
	// 0x100c2 bytes of records (210), then the missiles' 12-byte block header
	// and 300 missiles. Records past 210 run on into the missiles and past
	// them, as the original's do (Tactical_Bombardment_ loads every ship);
	// ship and mis are views of it.
	mem    []byte
	ship   []tship
	mis    []tmissile
	far    map[int16]*tship    // records past mem (see tsp)
	farMis map[int16]*tmissile // missiles past mem (see msl)

	beam     beamState // BEAMS.C, its logic also runs off screen
	n        int16     // 0x1998c0
	colony   int16     // 0x199878
	att, def int16     // 0x1998ac, 0x19988e
	onScreen int16     // 0x1998f6
	nebula   int16     // 0x1998ca, set by Tactical_Combat_ only
	ordnance [15]int16 // 0x199224 missile and bomb damage percent by owner
	bomb     bombHits  // 0x199b5a
	// 0x199242 and 0x199254: fighter bomb and beam weapons by owner, set
	// only by Tactical_Combat_.
	fighterBomb, fighterBeam [9]int16

	// The combat screen's state (Tactical_Combat_, docs/combat/tactical.md).
	turn        int16             // _combat_turn 0x199890
	cur, curPtr int16             // _cur_ship, _cur_ptr
	order       [0xd2]int16       // _ship_turn_order
	playerMode  [15]int8          // _player_mode
	sideEnd     [15]int16         // _side_end_of_turn
	grid        [0x51][0x44]int16 // _combat_grid: 2000 legal move, -1 empty, else record
	autoCombat  int16             // _auto_combat 0x199900
	autoAtt     int16             // _auto_combat_flag 0x192b10
	autoDef     int16             // 0x192b12
	superFast   int16             // 0x1998e0
	takeControl int16             // 0x1998c6
	initOrder   int16             // _combat_init 0x17d853
	// The original's records sit in the buffer at 0x192ed4, which a video or
	// the ground battle's pictures overwrite; record comparisons stop there.
	recordsLost bool
	aiRetreat   int16 // _ai_retreat_flag 0x1998c8
	startOfTurn int16
	fleetTarget int16
	mslWarning  int16 // _missile_warning 0x1998da
	autoMslOff  int16 // _auto_msl_warning_off
	speedx2     int16 // 0x19987e (MOX.SET)
	shieldsFlag int16 // 0x1998cc
	mslImpact   int16 // _msl_impact_warning_flag 0x1998c4
	realAttack  uint8 // 0x19b57c
	bioWeapon   int16 // 0x1998e2
	skan, board int16
	orderInfo   int16 // 0x199876
	curX, curY  int16 // the view's top-left square (UI)
	// The lockstep between two humans in a network game (netcombat_tac.go).
	netFlag         int16 // _net_flag 0x1998ce
	netDest         int16 // _net_destination 0x199908: the other side
	savedMslWarning int16 // _saved_msl_warning 0x199906
	savedMslImpact  int16 // _saved_msl_warn 0x1998d0 (msg 0x3eb)
	ui              tacticalUI
	scr             *combatScreen // the screen, nil when it is not drawn
}

func (g *Game) tsp(i int16) *tship {
	tc := g.tc
	if i >= 0 && int(i) < len(tc.ship) {
		return &tc.ship[i]
	}
	// A record index from overwritten bytes (records past 210 overlap the
	// missiles): the original reads memory far off; here zeros, kept per index.
	g.logOnce("a combat record outside the engine's memory (overlapping records and missiles)")
	if tc.far == nil {
		tc.far = map[int16]*tship{}
	}
	if tc.far[i] == nil {
		tc.far[i] = &tship{}
	}
	return tc.far[i]
}

// Layout of tactical.mem (Allocate_Next_Block_: a 12-byte header per block,
// sizes rounded up to 4).
const (
	tcRecords   = 0x100c2 / 0x139   // the records block holds 210
	tcMisHeader = 0x100c4           // the missiles block's header {0, 0x1e78, 0}
	tcMisOff    = tcMisHeader + 0xc // the missiles
	tcMemShips  = 1 + maxShips      // the colony and every ship of a bombardment
)

// clearBlocks is the engine's Push_Block_, both Allocate_Next_Block_ and
// Clear_Structure_ calls. The bytes past the missiles block are the arena's
// old contents in the original (not modelled: 0).
// fastFX is the animations' speed test: 0x19987e == 1 or _net_flag, so
// network battles always step their effects at double speed.
func (tc *tactical) fastFX() bool { return tc.speedx2 == 1 || tc.netFlag != 0 }

func (tc *tactical) clearBlocks() {
	n := max(tcMemShips*0x139, tcMisOff+nMissiles*0x1a)
	tc.mem = make([]byte, n)
	binary.LittleEndian.PutUint32(tc.mem[tcMisHeader+4:], nMissiles*0x1a)
	tc.ship = unsafe.Slice((*tship)(unsafe.Pointer(&tc.mem[0])), tcMemShips)
	tc.mis = unsafe.Slice((*tmissile)(unsafe.Pointer(&tc.mem[tcMisOff])), nMissiles)
}

// combatShipHasSpecial is Does_Combat_Ship_Have_Special_ (0x4b0d3).
func (g *Game) combatShipHasSpecial(i int16, sp int) bool {
	t := g.tsp(i)
	return testBit(t[tsSpecials:tsSpecials+5], sp) && !testBit(t[tsDamaged:tsDamaged+5], sp)
}

// loadCombatShip is Load_Combat_Ship_ (0x4954a): ship id into record i.
func (g *Game) loadCombatShip(id, i int16) {
	s := &g.state
	sh := &s.Ships[id]
	d := &sh.Design
	t := g.tsp(i)
	for k := 0; ; k++ {
		c := designBytes(d)[k]
		t[k] = c
		if c == 0 {
			break
		}
	}
	t.set8(tsOwner, uint8(sh.Owner))
	t.set8(tsBuilder, d.Builder)
	t.set16(tsID, id)
	t.set8(tsShield, d.Shield)
	t.set8(tsComputer, d.Computer)
	if t.u8(tsOwner) >= 8 {
		t.set8(tsDrive, uint8(g.bestWarpDrive(int(t.u8(tsOwner)))))
	} else {
		t.set8(tsDrive, d.Drive)
	}
	t.set8(tsCSpeed, d.CombatSpeed)
	t.set8(tsGone, 0)
	t.set8(tsSize, d.Size)
	for k := 0; k < 5; k++ {
		t[tsSpecials+k] = d.Specials[k]
		t[tsDamaged+k] = sh.DamagedSpecials[k]
	}
	for k := 0; k < 8; k++ {
		sl := &d.Weapons[k]
		t.set16(tw(k, twWeapon), sl.Weapon)
		t.set8(tw(k, twCount), sl.Working)
		t.set8(tw(k, twArc), sl.Arc)
		t.set16(tw(k, twMods), int16(sl.Mods))
		t.set16(tw(k, twAmmo), int16(sl.Ammo))
		if t.u8(tw(k, twArc)) == 0x10 {
			t.set8(tw(k, twArc), 0xf)
		}
		t.set8(tw(k, twOn), 1)
		t.set8(tw(k, twLeft), t.u8(tw(k, twCount)))
		switch g.wpnU8(int(sl.Weapon), weaponType) {
		case 2:
			t.set16(tw(k, twAmmo), 0)
		case 1:
		default:
			t.set16(tw(k, twAmmo), int16(g.wpnU8(int(sl.Weapon), weaponMaxAmmo)))
		}
		if t.i16(tw(k, twWeapon)) == 0x21 {
			t.set16(tw(k, twAmmo), t.i16(tw(k, twAmmo))*int16(t.u8(tw(k, twCount))))
		}
		t.set8(tw(k, twFired), 0)
	}
	for k := 0; k < 8; k++ {
		t[tsSpecialOn+k] = 1
	}
	t.set8(tsCrew, sh.CrewLevel)
	if o := sh.Officer; o > -1 && s.Leaders[o].ETA == 0 {
		t.set16(tsOfficer, o)
	} else {
		t.set16(tsOfficer, -1)
	}
	if d.Type == 0 && sh.Owner <= 8 {
		t.set8(tsMarines, g.exeU8(hullMarines+uint32(t.u8(tsSize))*hullStride))
		if testBit(t[tsSpecials:tsSpecials+5], 0x24) && !testBit(t[tsDamaged:tsDamaged+5], 0x24) {
			t[tsMarines] <<= 1
		}
	} else {
		t.set8(tsMarines, 0xff)
	}
	t.set16(tsStruct, int16(g.shipStructure(int(id))))
	t.set16(tsArmorHits, int16(g.shipArmorHits(int(id)))-sh.ArmorDamage)
	t.set8(tsArmor, d.Armor)
	t.set16(tsStructDmg, sh.StructureDamage)
	if t.i16(tsStructDmg) > t.i16(tsStruct) {
		t.set16(tsStructDmg, t.i16(tsStruct)-1)
	}
	if t.u8(tsSize) == 5 && t.u8(tsOwner) < 8 {
		t.set16(tsPicture, 0x2b)
	} else {
		t.set16(tsPicture, int16(int8(d.Picture)))
	}
	t.set8(0x4b, 0)
	g.initInternalSpaceRec(t)
	t.set16(tsShieldCur, int16((100-int32(sh.ShieldDamage))*int32(t.i16(tsShieldHP))/100))
	t.set16(tsDriveCur, int16((100-int32(sh.DriveDamage))*int32(t.i16(tsDriveMax))/100))
	t.set16(tsCompCur, t.i16(tsCompMax)-int16(sh.ComputerDamage))
	t.set16(tsShieldMax, g.calcCombatShieldMax(i))
	for k := 0; k < 4; k++ {
		t.set16(tsArc+2*k, t.i16(tsShieldMax))
	}
	if t.u8(tsOwner) > 8 && t.u8(tsDrive) == 0 {
		if k := t.u8(tsOwner) - 9; k <= 5 {
			t.set8(tsDrive, [6]uint8{4, 2, 4, 6, 6, 2}[k])
		}
	}
	g.initSpecialDevices(i)
}

// initInternalSpaceRec is Init_Internal_Space_ (0x36470): hits of drive,
// computer, each weapon mount and the specials, current = maximum.
func (g *Game) initInternalSpaceRec(t *tship) {
	size := int(t.u8(tsSize))
	if t.u8(tsDrive) > 0 {
		t.set16(tsDriveMax, g.hullI16(size, hullDrive))
	} else {
		t.set16(tsDriveMax, 0)
	}
	if testBit(t[tsSpecials:tsSpecials+5], 0x1b) && !testBit(t[tsDamaged:tsDamaged+5], 0x1b) {
		t.set16(tsDriveMax, t.i16(tsDriveMax)*3)
	}
	t.set16(tsCompMax, g.hullI16(size, hullComputer))
	for k := 0; k < 8; k++ {
		if t.i16(tw(k, twWeapon)) <= 0 {
			t.set16(tsWpnUnit+2*k, 0)
			t.set16(tsWpnSpace+2*k, 0)
			continue
		}
		v := int16(1)
		sp := func() int32 {
			return int32(int16(g.oneWeaponSpaceWithoutMLevel(t.i16(tw(k, twWeapon)), t.u8(tw(k, twArc)),
				t.i16(tw(k, twAmmo)), t.i16(tw(k, twMods)))))
		}
		if sp()/10 > 1 {
			v = int16(sp() / 10)
		}
		t.set16(tsWpnUnit+2*k, v)
		t.set16(tsWpnSpace+2*k, int16(t.u8(tw(k, twCount)))*v)
	}
	for k := 0; k < 8; k++ {
		t.set16(tsSpcSpace+2*k, 0)
	}
	n := 0
	for sp := 0; sp < 0x27 && n < 8; sp++ {
		if !testBit(t[tsSpecials:tsSpecials+5], sp) || testBit(t[tsDamaged:tsDamaged+5], sp) {
			continue
		}
		switch sp {
		case 0x1b, 0xb, 4, 0xe, 0x18:
			t.set16(tsSpcSpace+2*n, 0)
		default:
			v := int32(g.exeI16(rowAddr(specialTable, specialStride, sp, uint32(8+2*size)))) / 10
			if v <= 1 {
				v = 1
			}
			t.set16(tsSpcSpace+2*n, int16(v))
		}
		n++
	}
	if t.u8(tsShield) > 0 {
		t.set16(tsShieldHP, g.hullI16(size, hullShield))
	} else {
		t.set16(tsShieldHP, 0)
	}
	copy(t[tsCur:tsCur+0x36], t[tsDriveMax:tsDriveMax+0x36])
}

// oneWeaponSpaceWithoutMLevel is One_Weapon_Space_Without_M_Level_
// (0x6d1b4): Weapon_Space_ before miniaturisation, for one mount.
func (g *Game) oneWeaponSpaceWithoutMLevel(w int16, arcs uint8, ammo, mods int16) int32 {
	sp := int32(g.wpnI16(int(w), weaponSpace))
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
	_, other := g.bonusForSpecialMods(int32(mods))
	sp = int32(other) * sp / 100
	arcPct := int32(0)
	if g.wpnU8(int(w), weaponType) == 0 {
		switch {
		case arcs&2 != 0 || arcs&4 != 0:
			arcPct = 0x19
		case arcs&0x10 != 0:
			arcPct = 0x32
		}
	}
	_, reg := g.bonusForRegularMods(int32(mods))
	return (arcPct + 100 + int32(reg)) * sp / 100
}

// calcCombatShieldMax is Calc_Combat_Shield_Max_ (0x494a8): the damage one
// shield arc stops.
func (g *Game) calcCombatShieldMax(i int16) int16 {
	t := g.tsp(i)
	sh := int16(t.u8(tsShield))
	if sh <= 0 {
		return 0
	}
	m := int16(t.u8(tsSize)) + 1
	if t.u8(tsSize) >= 5 {
		m++
	}
	v := m * (g.exeI16(rowAddr(shieldTable, shieldStride, int(sh), shieldStrength)) * 5)
	if g.tc.nebula != 0 && !g.combatShipHasSpecial(i, 0xd) {
		v = 0
	}
	if g.combatShipHasSpecial(i, 0x14) {
		v = int16(int32(v) * 3 / 2)
	}
	return v
}

// initSpecialDevices is Init_Special_Devices_ (0x4c9f6).
func (g *Game) initSpecialDevices(i int16) {
	t := g.tsp(i)
	t.set8(tsStasis, 0xff)
	t.set8(0x137, 0xff)
	t.set16(tsAbsorbed, 0)
	t.set8(0x136, 0)
	t.set8(0x48, 0)
	t.set8(0x49, 0)
	t.set16(0x43, 0)
	t.set8(0x42, 0xff)
	t.set8(0x41, 0)
	switch {
	case g.combatShipHasSpecial(i, 6):
		t.set8(tsCloak, 1)
	case g.combatShipHasSpecial(i, 0x17):
		t.set8(tsCloak, 4)
		t.set8(0x41, 0xa)
	default:
		t.set8(tsCloak, 0)
	}
	if g.combatShipHasSpecial(i, 0x10) {
		for k := 0; k < 8; k++ {
			if g.wpnU8(int(t.i16(tw(k, twWeapon))), weaponType) == 0 {
				t.set8(tw(k, 0xa), 1)
			}
		}
	}
	if g.combatShipHasSpecial(i, 0xc) {
		for k := 0; k < 8; k++ {
			if g.wpnU8(int(t.i16(tw(k, twWeapon))), weaponType) == 1 {
				t.set8(tw(k, 0xa), 1)
			}
		}
	}
}

// loadTacticalColony is Load_Tactical_Colony_ (0x4aa36) without its
// picture: the colony as record 0 with its shield, bases and batteries.
func (g *Game) loadTacticalColony(ci int16) {
	s := &g.state
	tc := g.tc
	C := &s.Colonies[ci]
	t := g.tsp(0)
	copy(t[:], append([]byte(g.planetNameNoPrime(int(C.Planet))), 0))
	t.set16(tsID, -1)
	t.set8(tsDrive, 0)
	t.set16(tsOfficer, -1)
	t.set8(0x4b, 0)
	t.set8(tsOwner, uint8(C.Owner))
	t.set8(tsBuilder, uint8(C.Owner))
	blk := int16(0)
	switch {
	case C.Buildings[0x1c] != 0:
		blk = int16(g.exeU8(0x14aa5))
		t.set8(tsShield, 0x1c)
	case C.Buildings[0x17] != 0:
		blk = int16(g.exeU8(0x14aab))
		t.set8(tsShield, 0x17)
	case C.Buildings[0x18] != 0:
		blk = int16(g.exeU8(0x14aa8))
		t.set8(tsShield, 0x18)
	default:
		t.set8(tsShield, 0)
	}
	t.set16(tsShieldMax, blk)
	for k := 0; k < 4; k++ {
		t.set16(tsArc+2*k, blk)
	}
	t.add16(tsArc, g.exeI16(rowAddr(shieldTable, shieldStride, g.bestShipShield(int(tc.def)), shieldStrength)))
	owner := int(C.Owner)
	t.set8(tsComputer, uint8(g.bestComputer(owner)))
	t.set8(tsSize, 5)
	t.set8(tsGone, 0)
	t.set8(tsArmor, uint8(g.bestArmor(owner)))
	for k := 0; k < 5; k++ {
		t[tsSpecials+k] = 0
		t[tsDamaged+k] = 0
	}
	for k := 0; k < 8; k++ {
		t.set16(tw(k, twWeapon), 0)
	}
	k := 0
	put := func(w int16, n uint8, mods, ammo int16) {
		t.set8(tw(k, twArc), 0xf)
		t.set16(tw(k, twAmmo), ammo)
		t.set8(tw(k, twOn), 1)
		t.set8(tw(k, twFired), 0)
		t.set16(tw(k, twWeapon), w)
		t.set8(tw(k, twCount), n)
		t.set8(tw(k, twLeft), n)
		t.set16(tw(k, twMods), mods)
		k++
	}
	def := int(tc.def)
	if C.Buildings[0x1b] != 0 {
		w := int16(g.bestBeamWeapon(owner))
		ml := g.miniaturizationLevel(def, int(g.wpnI16(int(w), weaponApp)))
		mods := int16(2)
		if ml >= int16(g.exeU8(0x17fd41)) {
			mods = 0xa
		}
		if ml >= int16(g.exeU8(0x17fd50)) {
			mods |= 0x10
		}
		if ml >= int16(g.exeU8(0x17fd6e)) {
			mods |= 0x40
		}
		if ml >= int16(g.exeU8(0x17fd5f)) {
			mods |= 0x20
		}
		mods &= g.wpnI16(int(w), weaponFlags)
		n := 0xe1 / g.weaponSpace(def, int(w), 1, 1, 1, mods)
		put(w, uint8(n), mods, -1)
		w = int16(g.bestPointDefenseWeaponBySize(owner))
		ml = g.miniaturizationLevel(def, int(g.wpnI16(int(w), weaponApp)))
		mods = 4
		if ml >= int16(g.exeU8(0x17fd50)) {
			mods = 0x14
		}
		if ml >= int16(g.exeU8(0x17fd6e)) {
			mods |= 0x40
		}
		mods &= g.wpnI16(int(w), weaponFlags)
		n = 0x4b / g.weaponSpace(def, int(w), 1, 1, 1, mods)
		put(w, uint8(n), mods, -1)
	}
	if C.Buildings[0x2a] != 0 {
		put(0x26, 1, 0, -1)
	}
	if C.Buildings[0x2f] != 0 {
		P := &s.Players[owner]
		w, n := int16(0x1f), uint8(5)
		switch {
		case P.AppStatus[0x53] == 3:
			w, n = 0x1d, 2
		case P.AppStatus[0x1f] == 3:
			w, n = 0x1e, 3
		}
		for j := 0; j < 2; j++ {
			put(w, n, 0, 1)
		}
	}
	if C.Buildings[0x1a] != 0 {
		w := int16(g.bestMissileWeaponNT(owner))
		ml := g.miniaturizationLevel(def, int(g.wpnI16(int(w), weaponApp)))
		mods := int16(0)
		if ml >= int16(g.exeU8(0x17fdc8)) {
			mods = 0x1000
		}
		if ml >= int16(g.exeU8(0x17fdb9)) {
			mods |= 0x800
		}
		n := int16(0x12c / g.weaponSpace(def, int(w), 1, 0x10, 5, mods))
		reps := 1
		if n > 10 {
			n /= 2
			reps = 2
		}
		mods |= 0x400
		for j := 0; j < reps; j++ {
			ammo := int16(0)
			if g.wpnU8(int(w), weaponType) == 1 {
				ammo = 0x7d
			}
			put(w, uint8(n), mods, ammo)
		}
	}
	if k > 0 && s.Players[def].AppStatus[0x1a] == 3 {
		setBit(t[tsSpecials:tsSpecials+5], 5)
	}
	t.set16(tsStruct, 0)
	t.set8(tsCrew, 1)
	t.set16(tsOfficer, -1)
	t.set8(tsMarines, 0)
	t.set16(tsStructDmg, 0)
	t.set16(tsSpeed, 0)
	g.initInternalSpaceRec(t)
	t.set16(tsShieldHP, 0)
	t.set16(tsShieldCur, 0)
}

// setCombatOrdnanceValues is Set_Combat_Ordnance_Values_ (0x35f0a).
func (g *Game) setCombatOrdnanceValues() {
	tc := g.tc
	tc.ordnance[tc.att] = g.fleetOrdnanceBonus(tc.att) + 100
	tc.ordnance[tc.def] = g.fleetOrdnanceBonus(tc.def) + 100
}

// fleetOrdnanceBonus is Get_Fleet_Ordnance_Bonus_ (0x35f7e): the best
// ordnance officer among p's records that are still in the fight.
func (g *Game) fleetOrdnanceBonus(p int16) int16 {
	best := int16(0)
	for i := int16(0); i < g.tc.n; i++ {
		t := g.tsp(i)
		if int16(t.u8(tsOwner)) != p || t.u8(tsBoarded) != 0 || t.u8(tsGone) != 0 || t.i16(tsOfficer) <= -1 {
			continue
		}
		best = max(best, g.combatOfficerBonus(int(t.i16(tsOfficer)), 1, 0x04, 0x08))
	}
	return best
}

// computerBonus is Computer_Bonus_ (0x35c7f): nothing once the computer
// is shot away.
func (g *Game) computerBonus(i int16) int16 {
	t := g.tsp(i)
	if t.i16(tsCompCur) <= 0 {
		return 0
	}
	return g.exeI16(rowAddr(computerTable, computerStride, int(t.u8(tsComputer)), computerBonus))
}

// offensiveCombatBonus is Offensive_Combat_Bonus_ (0x366dd): computer,
// crew, battle scanner, fleet officers and the race pick.
func (g *Game) offensiveCombatBonus(i int16) int16 {
	t := g.tsp(i)
	comp := g.computerBonus(i)
	crew := g.exeI16(rowAddr(crewTable, crewStride, int(t.u8(tsCrew)), 0))
	scanner := int16(0)
	if g.combatShipHasSpecial(i, 5) {
		scanner = 0x32
	}
	fleet := g.fleetOffensiveBonus(int16(t.u8(tsOwner)))
	race := int16(0)
	if t.u8(tsOwner) < 8 {
		race = int16(g.state.Players[t.u8(tsOwner)].Picks[save.PickShipAttack])
	}
	return race + crew + comp + scanner + fleet
}

// fleetOffensiveBonus is Fleet_Offensive_Bonus_ (0x36266): the side's best
// weaponry (and, against monsters, galactic lore) officer, a command bonus
// from the biggest flagship, and +20 for a scout-lab ship against monsters.
func (g *Game) fleetOffensiveBonus(p int16) int16 {
	tc := g.tc
	var lab, weap, cmd, lore int16
	monsters := tc.att > 8 || tc.def > 8
	for i := int16(0); i < tc.n; i++ {
		t := g.tsp(i)
		if int16(t.u8(tsOwner)) != p || t.u8(tsBoarded) != 0 || t.u8(tsGone) != 0 {
			continue
		}
		if t.satellite() { // community patch 1.51 (0x362de)
			switch t.u8(tsSize) {
			case 2:
				cmd = 0
			case 3:
				cmd = 0xa
			case 4:
				cmd = 0x14
			}
		}
		if o := t.i16(tsOfficer); o > -1 {
			weap = max(weap, g.combatOfficerBonus(int(o), 1, 0x40, 0x80))
			if monsters {
				lore = max(lore, g.combatOfficerBonus(int(o), 0, 0x10, 0x20))
			}
		}
		if monsters && g.combatShipHasSpecial(i, 0x1c) {
			lab = 0x14
		}
	}
	return cmd + weap + lore + lab
}

// defensiveCombatBonus is Defensive_Combat_Bonus_ (0x35d0d): beam (kind 0)
// or missile (kind 1) defence.
func (g *Game) defensiveCombatBonus(i, kind int16) int16 {
	t := g.tsp(i)
	var own, fleetSp, race int16
	crew := g.exeI16(rowAddr(crewTable, crewStride, int(t.u8(tsCrew)), 2))
	var speed int16
	if kind == 0 {
		speed = t.i16(tsSpeed) * 5
		if t.i16(tsSpeed) == 0 {
			speed -= 0x14
		}
	} else {
		crew /= 2
	}
	sp := g.shipSpecialsDefensiveBonus(i, kind, &own)
	spec := sp
	fleet := g.fleetDefensiveBonus(int16(t.u8(tsOwner)), kind, &fleetSp)
	if fleetSp > own {
		spec = sp - own
	} else {
		fleet -= fleetSp
	}
	if !t.satellite() && t.u8(tsOwner) < 8 { // community patch 1.51 (0x35dd2)
		race = int16(g.state.Players[t.u8(tsOwner)].Picks[save.PickShipDefense])
	}
	return speed + crew + spec + fleet + race
}

// shipSpecialsDefensiveBonus is Ship_Specials_Defensive_Bonus_ (0x36a63);
// part receives the share a fleet-wide device would also give.
func (g *Game) shipSpecialsDefensiveBonus(i, kind int16, part *int16) int16 {
	t := g.tsp(i)
	v := int16(0)
	if t.u8(tsCloak) == 1 && kind == 0 {
		v = 0x50
	}
	switch {
	case g.combatShipHasSpecial(i, 0x26):
		if kind == 1 {
			v += 0x82
			*part = 0x82
		}
	case g.combatShipHasSpecial(i, 0x15):
		if kind == 1 {
			v += 0x64
			*part = 0x64
		}
	case g.combatShipHasSpecial(i, 9):
		if kind == 1 {
			v += 0x46
			*part = 0x46
		}
	}
	if g.combatShipHasSpecial(i, 0x11) {
		if kind == 0 {
			v += 0x64
		}
		if kind == 1 {
			v += 0x32
		}
	}
	if g.combatShipHasSpecial(i, 0x12) {
		if kind == 0 {
			v += 0x32
		}
		if kind == 1 {
			v += 0x19
		}
	}
	return v
}

// fleetDefensiveBonus is Fleet_Defensive_Bonus_ (0x36139): the best
// helmsman; for missiles half of it, and 70 from a fleet device.
func (g *Game) fleetDefensiveBonus(p, kind int16, dev *int16) int16 {
	tc := g.tc
	helm := int16(0)
	cmd := int16(0)
	for i := int16(0); i < tc.n; i++ {
		t := g.tsp(i)
		if int16(t.u8(tsOwner)) != p || t.u8(tsBoarded) != 0 || t.u8(tsGone) != 0 {
			continue
		}
		if kind == 1 && g.combatShipHasSpecial(i, 0x26) {
			*dev = 0x46
		}
		if o := t.i16(tsOfficer); o > -1 {
			helm = max(helm, g.combatOfficerBonus(int(o), 0, 0x40, 0x80))
		}
		if kind != 1 && t.u8(tsDrive) == 0 {
			switch t.u8(tsSize) {
			case 2, 3, 4:
				cmd = 0
			}
		}
	}
	if kind == 1 {
		return helm/2 + *dev
	}
	return helm + cmd
}

// tacticalBombardment is Tactical_Bombardment_ (0x42781): the ships (by
// size) fire ten volleys at the colony through the tactical engine; the
// planet's structure damage counts a hit per 100.
func (g *Game) tacticalBombardment(list []int16, b *bombHits) {
	s := &g.state
	if g.tc == nil {
		g.tc = &tactical{}
	}
	tc := g.tc
	tc.onScreen = 0
	tc.n = 1
	tc.colony = int16(b.Colony)
	// Bombardment_ clears the hit record (0x199b5a) and the two blocks are
	// cleared on allocation; the other globals persist.
	*tc = tactical{n: 1, colony: int16(b.Colony), nebula: tc.nebula, ordnance: tc.ordnance,
		fighterBomb: tc.fighterBomb, fighterBeam: tc.fighterBeam}
	tc.clearBlocks()
	tc.att = int16(s.Ships[list[0]].Owner)
	tc.def = int16(s.Colonies[tc.colony].Owner)
	clib.Qsort(list, 2, func(x, y *int16) int {
		a, c := s.Ships[*x].Design.Size, s.Ships[*y].Design.Size
		switch {
		case a == c:
			return 0
		case a < c:
			return 1
		}
		return -1
	})
	for _, id := range list {
		if s.Ships[id].Design.Type != 0 {
			continue
		}
		g.loadCombatShip(id, tc.n)
		t := g.tsp(tc.n)
		t.set8(tsX, 0)
		tc.n++
		t.set8(tsY, 0)
	}
	g.loadTacticalColony(tc.colony)
	g.setCombatOrdnanceValues()
	g.tsp(0).set8(tsX, 0)
	g.tsp(0).set8(tsY, 0)
	for i := int16(0); i < tc.n; i++ {
		t := g.tsp(i)
		t.set16(tsOcv, g.offensiveCombatBonus(i))
		t.set16(tsDcv, g.defensiveCombatBonus(i, 0))
		t.set16(tsEvasion, g.defensiveCombatBonus(i, 1))
		g.initSpecialDevices(i)
	}
	for v := 0; v < 10; v++ {
		for i := int16(0); i < tc.n; i++ {
			t := g.tsp(i)
			for k := 0; k < 8; k++ {
				t.set8(tw(k, twOn), 1)
				t.set8(tw(k, twLeft), t.u8(tw(k, twCount)))
				if w := t.i16(tw(k, twWeapon)); w > 0 && g.wpnU8(int(w), weaponType) != 1 && t.i16(tw(k, twAmmo)) > 0 {
					t.add16(tw(k, twAmmo), -1)
				}
			}
		}
		for i := int16(1); i < tc.n; i++ {
			g.fireShipAt(i, 0, 0, 0, 0, 0, 0)
		}
	}
	b.Hits = g.tsp(0).i16(tsStruct) / 100
	b.Kind = tc.bomb.Kind
	b.Accident = tc.bomb.Accident
	b.BioHits = tc.bomb.BioHits
}

// fighterBombOf and fighterBeamOf read _fighter_bomb_type[o] and
// _fighter_beam_type[o] as the original does for any owner: the two
// 9-word arrays are adjacent, so a monster (owner 9..14) reads the beam
// array; past that lies _monster_names, which the port cannot know.
func (g *Game) fighterBombOf(o int) int16 { return g.fighterTypeWord(o) }

func (g *Game) fighterBeamOf(o int) int16 { return g.fighterTypeWord(9 + o) }

func (g *Game) fighterTypeWord(k int) int16 {
	tc := g.tc
	switch {
	case k >= 0 && k < 9:
		return tc.fighterBomb[k]
	case k >= 9 && k < 18:
		return tc.fighterBeam[k-9]
	}
	g.logOnce("fighter weapon read past _fighter_beam_type (into _monster_names); taken as 0")
	return 0
}
