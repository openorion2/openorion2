package game

import (
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// Ship hit points: structure, armour and their sum. Spec:
// docs/formats/ship-stats.md.

// Constant tables in ORION2.EXE (ELF addresses).
const (
	hullTable   = 0x180024 // _hull_data, 0x24-byte rows indexed by design size
	hullStride  = 0x24
	hullArmor   = 0x00     // i16 base armour hits
	hullStruct  = 0x02     // i16 base structure hits
	hullSimple  = 0x14     // u16 hits when combat is strategic (settings 0xd8)
	armorTable  = 0x17f63e // 0x0f-byte rows indexed by armour class
	armorStride = 0x0f
	armorTech   = 0x00 // i16 tech application that gives the armour
	armorPct    = 0x04 // i16 percent added to the hull's base hits
)

// Special-system bits of a design.
const (
	specialHeavyArmor     = 14 // armour hits x3 (Get_Ship_Armor_Hits_)
	specialReinforcedHull = 27 // structure x3 (Get_Design_Structure_)
)

// strategicCombatOn is DAT_00199cb4, settings byte 0xd8 (1 strategic combat): hits
// then come from the hull's own total and armour damage is not counted.
func (g *Game) strategicCombatOn() bool { return g.settings.StrategicCombat() }

func (g *Game) exeU16(addr uint32) uint16 {
	return binary.LittleEndian.Uint16(g.exe.bytes(addr, 2))
}

func (g *Game) hullField(size uint8, off uint32) int16 {
	return int16(g.exeU16(hullTable + uint32(size)*hullStride + off))
}

func (g *Game) armorField(class int, off uint32) int16 {
	return int16(g.exeU16(uint32(int32(armorTable) + int32(class)*armorStride + int32(off))))
}

// bonusHits is base + base*pct/100 with C's truncating division.
func bonusHits(base, pct int16) int {
	b := int(base)
	return int(pct)*b/100 + b
}

// designStructure is Get_Design_Structure_ (0x582bf); the design is passed
// by value on the stack.
func (g *Game) designStructure(d *save.Design) int {
	reinforced := testBit(d.Specials[:], specialReinforcedHull)
	if g.strategicCombatOn() {
		return int(uint16(g.hullField(d.Size, hullSimple)))
	}
	n := bonusHits(g.hullField(d.Size, hullStruct), g.armorField(int(d.Armor), armorPct))
	if reinforced {
		n *= 3
	}
	return n
}

// monsterStructure gives the fixed structure of monster owners 10..14
// (Get_Ship_Structure_'s jump table at 0x58373); drive != 0 marks the
// stronger variant.
var monsterStructure = [5][2]int{
	{400, 750},  // 10 Amoeba
	{500, 2500}, // 11 Crystal
	{500, 2500}, // 12 Dragon
	{300, 1000}, // 13 Eel
	{500, 1500}, // 14 Hydra
}

// shipStructure is Get_Ship_Structure_ (0x58387).
func (g *Game) shipStructure(ship int) int {
	s := &g.state.Ships[ship]
	if s.Owner <= 9 {
		return g.designStructure(&s.Design)
	}
	o := int(s.Owner) - 10
	if o > 4 {
		// The original returns the caller's ECX here; no owner above 14
		// reaches it in practice.
		return 0
	}
	v := 0
	if s.Design.Drive != 0 {
		v = 1
	}
	return monsterStructure[o][v]
}

// shipArmorHits is Get_Ship_Armor_Hits_ (0x58425).
func (g *Game) shipArmorHits(ship int) int {
	d := &g.state.Ships[ship].Design
	n := bonusHits(g.hullField(d.Size, hullArmor), g.armorField(int(d.Armor), armorPct))
	if testBit(d.Specials[:], specialHeavyArmor) {
		n *= 3
	}
	return n
}

// bestArmor is Best_Armor_ (0x5685f): the best armour class whose tech
// application the player knows, 1 if none; Antarans and the Guardian get 6,
// monsters 0.
func (g *Game) bestArmor(owner int) int {
	best := 1
	switch {
	case int16(owner) < 8:
		p := &g.state.Players[owner]
		for i := 2; i < 7; i++ {
			if p.AppStatus[g.armorField(i, armorTech)] == 3 {
				best = i
			}
		}
	case uint16(owner) <= 9:
		best = 6
	case uint16(owner) <= 14:
		best = 0
	}
	return best
}

// maxShipHits is Max_Ship_Hits_ (0x5f2f6). Callers use the low 16 bits.
func (g *Game) maxShipHits(ship int) int {
	if !g.strategicCombatOn() {
		return int(int16(g.shipStructure(ship) + g.shipArmorHits(ship)))
	}
	s := &g.state.Ships[ship]
	hits := g.hullField(s.Design.Size, hullSimple)
	n := int(uint16(hits))
	if g.bestArmor(int(s.Owner)) > 0 {
		n = bonusHits(hits, g.armorField(g.bestArmor(int(s.Owner)), armorPct))
	}
	return int(int16(n))
}

// currentShipHits is Current_Ship_Hits_ (0x5f2c3): 16-bit arithmetic.
func (g *Game) currentShipHits(ship int) int {
	s := &g.state.Ships[ship]
	n := int16(g.maxShipHits(ship)) - s.StructureDamage
	if !g.strategicCombatOn() {
		n -= s.ArmorDamage
	}
	return int(n)
}

// isCombatShip is Is_Combat_Ship_ (0x78182): design type 0.
func (g *Game) isCombatShip(ship int) bool { return g.state.Ships[ship].Design.Type == 0 }
