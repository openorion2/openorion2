package game

import (
	"bytes"
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// Ship designs after a new technology: rebuilt, or refitted and re-costed.
// Spec: docs/formats/tech.md, "Design updates".

// Constant tables of ORION2.EXE (ELF addresses) the design cost reads.
const (
	hullCostTable = 0x18001e // _hull_data: i16 cost, 0x24-byte rows by hull

	shieldCostTable   = 0x17f6b5 // shieldTable + 0x14: i16 cost by hull
	driveCostTable    = 0x17fe84 // driveTable + 0x14
	driveCombatSpeed  = 0x17fe90 // driveTable + 0x20: u8 combat speed by hull
	computerCostTable = 0x17fdf4 // computerTable + 8
	armorCostPct      = 0x17f644 // armorTable + 6: i16 percent of the hull cost
	specialCostTable  = 0x17eef4 // specialTable + 0x14

	weaponCostBase = 0x10 // i16 in a weapon row
	modCostPct     = 0x0a // i16 in a modification row
	modOther       = 0x0c // i16, the second value Bonus_For_*_Mods_ sums

	initialPicks = 0x178513 // u8, the picks Calculate_Picks_ starts from
)

// updatePlayerShipDesigns is Update_Player_Ship_Designs_ (0x57112): an AI
// player's five designs are redone, hull size = slot.
func (g *Game) updatePlayerShipDesigns(p int) {
	if g.settings[0xd8] == 1 {
		g.updatePlayerStrategicShipDesigns(p)
		return
	}
	if g.state.Players[p].Objective == save.ObjectiveHuman {
		g.updateAutomaticDesignEquipment(p)
		g.updateEquipmentOnShipsBeingBuilt(p)
		return
	}
	g.updateEquipmentOnShipsBeingBuilt(p)
	for slot := 0; slot <= 4; slot++ {
		g.autoDesignShip(p, &g.state.Players[p].Designs[slot], int16(slot), false)
	}
}

// updatePlayerStrategicShipDesigns is Update_Player_Strategic_Ship_Designs_
// (0x574a1): every slot rebuilt, the picture stepping through the hull's.
func (g *Game) updatePlayerStrategicShipDesigns(p int) {
	pl := &g.state.Players[p]
	for slot := 0; slot < 6; slot++ {
		d := &pl.Designs[slot]
		pic := int16(int8(d.Picture))
		*d = save.Design{}
		g.autoStrategicDesignShip(p, slot, d)
		if slot == 5 {
			pic = 0x2b
		} else if pic++; int32(pic) == int32(slot*8+8) {
			pic = int16(slot * 8)
		}
		d.Picture = uint8(pic)
	}
}

// autoStrategicDesignShip is Auto_Strategic_Design_Ship_ (0x57871).
func (g *Game) autoStrategicDesignShip(p, slot int, d *save.Design) {
	name, err := g.hullName(slot, false)
	if err != nil {
		g.Log("hull name %d: %v", slot, err)
	}
	strcpyDesign(d, name)
	d.Size = uint8(slot)
	d.Drive = uint8(g.bestWarpDrive(p))
	d.Shield = uint8(g.bestShipShield(p))
	d.Computer = uint8(g.bestComputer(p))
	d.Armor = uint8(g.bestArmor(p))
	cost := int32(g.exeI16(rowAddr(hullCostTable, hullStride, int(g.state.Players[p].Designs[slot].Size), 0))) * 3 / 2
	cost = g.costReductionForGovtType(p, cost)
	d.Cost = int16(cost)
	g.updateStrategicDesignWeaponsSpecials(d, p, int(d.Size))
	if slot == 5 {
		d.Picture = 0x2b
	} else {
		d.Picture = uint8(slot) << 3
	}
	d.Builder = uint8(p)
	d.FTLSpeed = uint8(g.ftlSpeed(p, int(d.Drive)))
	d.BuildDate = uint16(g.state.Stardate)
}

// strcpyDesign copies name over the design as strcpy does, spilling into
// the fields after the name if it is too long.
func strcpyDesign(d *save.Design, name string) {
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, d); err != nil {
		panic(err)
	}
	raw := b.Bytes()
	copy(raw, name+"\x00")
	if err := binary.Read(bytes.NewReader(raw), binary.LittleEndian, d); err != nil {
		panic(err)
	}
}

// costReductionForGovtType is Cost_Reduction_For_Govt_Type_ (0x6e1a0).
func (g *Game) costReductionForGovtType(p int, v int32) int32 {
	switch g.state.Players[p].Picks[save.PickGovernment] {
	case 0:
		return (v*2 + 2) / 3
	case 1:
		return (v + 2) / 3
	}
	return v
}

// combatSpeedFor is the drive table's combat speed for a hull.
func (g *Game) combatSpeedFor(drive, size uint8) uint8 {
	return g.exeU8(driveCombatSpeed + uint32(size) + uint32(drive)*driveStride)
}

// refitDrive puts the best drive into a design and moves its combat speed
// by the difference, as both refit routines below do.
func (g *Game) refitDrive(p int, d *save.Design, best int) {
	if uint16(d.Drive) == uint16(best) {
		return
	}
	speed := d.CombatSpeed - g.combatSpeedFor(d.Drive, d.Size)
	d.Drive = uint8(best)
	d.FTLSpeed = uint8(g.ftlSpeed(p, best))
	d.CombatSpeed = speed + g.combatSpeedFor(d.Drive, d.Size)
}

// updateAutomaticDesignEquipment is Update_Automatic_Design_Equipment_
// (0x572ab).
func (g *Game) updateAutomaticDesignEquipment(p int) {
	armor, drive := g.bestArmor(p), g.bestWarpDrive(p)
	pl := &g.state.Players[p]
	for slot := 0; slot < 6; slot++ {
		d := &pl.Designs[slot]
		d.Armor = uint8(armor)
		g.refitDrive(p, d, drive)
		d.Cost = int16(g.costOfDesign(p, d))
	}
}

// updateEquipmentOnShipsBeingBuilt is Update_Equipment_On_Ships_Being_Built_
// (0x57197): ships refitting or being built.
func (g *Game) updateEquipmentOnShipsBeingBuilt(p int) {
	s := &g.state
	if s.Players[p].Eliminated != 0 {
		return
	}
	armor, drive := g.bestArmor(p), g.bestWarpDrive(p)
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		if int16(sh.Owner) != int16(p) || (sh.Status != save.ShipRefitting && sh.Status != save.ShipBuilding) {
			continue
		}
		g.refitDrive(p, &sh.Design, drive)
		sh.Design.Armor = uint8(armor)
	}
}

// costGivenMLevel is Cost_Given_M_Level_ (0x6b519): the cost after
// miniaturisation, at least 1 for a positive cost.
func costGivenMLevel(cost, level int16) int16 {
	pct := [5]int32{100, 75, 55, 40, 30}
	p := int32(25)
	if uint16(level) <= 4 {
		p = pct[level]
	}
	v := int16(p * int32(cost) / 100)
	if v == 0 && cost > 0 {
		v = 1
	}
	return v
}

// miniaturizationLevel is Miniaturization_Level_ (0x6d048): fields researched
// past the application's own, plus hyper levels.
func (g *Game) miniaturizationLevel(p, app int) int16 {
	pl := &g.state.Players[p]
	f := g.techAppFieldOf(app)
	var n int16
	for {
		nx := int16(g.techFieldNextOf(f))
		switch {
		case nx == 0:
			return n
		case nx >= 0x4b:
			return n + int16(pl.HyperLevels[nx-0x4b])
		case pl.FieldStatus[nx] != techKnown:
			return n
		}
		n++
		f = int(nx)
	}
}

// techLevel is Tech_Level_ (0x6e70a). An application outside 0..0xd3
// makes the original stop with a message.
func (g *Game) techLevel(p, app int) int16 {
	if int16(p) <= -1 || int16(p) >= g.state.NumPlayers {
		return 0
	}
	if int16(app) < 0 || int16(app) >= 0xd4 {
		panic("Tech_Level_: application out of range")
	}
	if app == 0 || g.state.Players[p].AppStatus[app] == techObtainable {
		return 0
	}
	return g.miniaturizationLevel(p, app)
}

// bitValueToNum is Bit_Value_To_Num_ (0x8e94d): the lowest set bit below
// n, 0 for none.
func bitValueToNum(v int32, n int16) int16 {
	for i := int16(0); i < n; i++ {
		if v>>uint(i)&1 != 0 {
			return i
		}
	}
	return 0
}

func (g *Game) modField(bit int32, off uint32) int16 {
	return g.exeI16(rowAddr(modTable, modStride, int(bitValueToNum(bit, 0xf)), off))
}

// bonusForSpecialMods is Bonus_For_Special_Mods_ (0x6a636).
func (g *Game) bonusForSpecialMods(mods int32) (cost, other int16) {
	cost, other = 100, 100
	var bit int32
	switch {
	case mods&2 != 0:
		bit = 2
	case mods&4 != 0:
		bit = 4
	default:
		return
	}
	return cost + g.modField(bit, modCostPct), other + g.modField(bit, modOther)
}

// bonusForRegularMods is Bonus_For_Regular_Mods_ (0x6a406).
func (g *Game) bonusForRegularMods(mods int32) (cost, other int16) {
	for _, bit := range []int32{0x4000, 8, 0x10, 0x20, 0x40, 0x80, 0x100, 0x200, 0x400, 0x800, 0x1000, 0x2000} {
		if mods&bit != 0 {
			cost += g.modField(bit, modCostPct)
			other += g.modField(bit, modOther)
		}
	}
	return
}

// weaponCost is Weapon_Cost_ (0x6ec74); its miniaturisation level is the
// viewing player's (_PLAYER_NUM), whoever owns the design.
func (g *Game) weaponCost(w, count int16, arcs, ammo uint8, mods int16) int16 {
	base := int32(g.exeI16(rowAddr(weaponTable, weaponStride, int(w), weaponCostBase)))
	typ := g.exeU8(rowAddr(weaponTable, weaponStride, int(w), weaponType))
	if typ == 1 {
		switch ammo {
		case 2:
			base += 10
		case 5:
			base += 20
		case 10:
			base += 30
		case 15:
			base += 35
		case 20:
			base += 40
		}
	}
	pct, _ := g.bonusForSpecialMods(int32(mods))
	base = int32(pct) * base / 100
	var arc int32
	if typ == 0 {
		switch {
		case arcs&2 != 0, arcs&4 != 0:
			arc = 25
		case arcs&0x10 != 0:
			arc = 50
		}
	}
	reg, _ := g.bonusForRegularMods(int32(mods))
	v := (arc + 100 + int32(reg)) * base / 100 * int32(count)
	app := int(g.exeI16(rowAddr(weaponTable, weaponStride, int(w), weaponApp)))
	return costGivenMLevel(int16(v), g.techLevel(int(g.state.PlayerNum), app))
}

// costOfDesign is Cost_Of_Design_ (0x6b577) with Design_Template_Costs_ and
// Total_Design_Cost_; it never copies the shield, so shield class 0 is costed.
func (g *Game) costOfDesign(p int, d *save.Design) int32 {
	size := int(d.Size)
	var specials [8]int16
	for b, n := 0, 0; b < 0x27 && n < 8; b++ {
		if testBit(d.Specials[:], b) {
			specials[n] = int16(b)
			n++
		}
	}
	const shield = 0
	shieldCost := g.exeI16(rowAddr(shieldCostTable+uint32(size)*2, shieldStride, shield, 0))
	shieldCost = costGivenMLevel(shieldCost, g.techLevel(p, int(g.exeI16(rowAddr(shieldTable, shieldStride, shield, shieldApp)))))
	drive := int(d.Drive)
	driveCost := costGivenMLevel(g.exeI16(rowAddr(driveCostTable+uint32(size)*2, driveStride, drive, 0)),
		g.techLevel(p, int(g.exeI16(rowAddr(driveTable, driveStride, drive, driveApp)))))
	hull := g.exeI16(rowAddr(hullCostTable, hullStride, size, 0))
	armorCost := int16(int32(hull) * int32(g.exeI16(rowAddr(armorCostPct, armorStride, int(d.Armor), 0))) / 100)
	var weapons int32
	for k := range d.Weapons {
		w := &d.Weapons[k]
		if w.Weapon > 0 {
			weapons += int32(g.weaponCost(w.Weapon, int16(w.Count), w.Arc, w.Ammo, int16(w.Mods)))
		}
	}
	var specialCost int32
	for _, sp := range specials {
		c := g.exeI16(rowAddr(specialCostTable+uint32(size)*2, specialStride, int(sp), 0))
		switch {
		case sp <= 0:
		case sp == 4:
			specialCost += int32(c)
		default:
			app := int(g.exeI16(rowAddr(specialTable, specialStride, int(sp), specialApp)))
			specialCost += int32(costGivenMLevel(c, g.techLevel(p, app)))
		}
	}
	comp := int(d.Computer)
	compCost := g.exeI16(rowAddr(computerCostTable+uint32(size)*2, computerStride, comp, 0))
	if compCost == 0 && comp > 0 {
		compCost = 1
	}
	compCost = costGivenMLevel(compCost, g.techLevel(p, int(g.exeI16(rowAddr(computerTable, computerStride, comp, computerApp)))))
	total := int32(shieldCost) + int32(hull) + int32(driveCost) + int32(armorCost) + int32(compCost) + specialCost + weapons
	return g.costReductionForGovtType(int(g.state.PlayerNum), total)
}
