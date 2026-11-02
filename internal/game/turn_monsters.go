package game

import (
	"bytes"
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// Space monsters (INITSHIP.C, SHIPS.C): the design loaders and
// Build_Nonplayer_Ship_. Spec: docs/turn/overview.md, phase F, Event_Twiddle_.

// monsterDesign is the 0x63-byte design buffer the loaders fill.
type monsterDesign [0x63]byte

func (b *monsterDesign) slot(k int, weapon uint16, count uint8, arc uint8, mods uint16, ammo uint8) {
	o := 0x1c + 8*k
	binary.LittleEndian.PutUint16(b[o:], weapon)
	b[o+2], b[o+3], b[o+4] = count, count, arc
	binary.LittleEndian.PutUint16(b[o+5:], mods)
	b[o+7] = ammo
}

func (b *monsterDesign) decode(d *save.Design) {
	if err := binary.Read(bytes.NewReader(b[:]), binary.LittleEndian, d); err != nil {
		panic(err)
	}
}

// monsterSpeed is the combat speed table entry for drive and hull.
func (g *Game) monsterSpeed(drive, size int) uint8 {
	return g.exeU8(uint32(shipCombatSpeedTbl + drive*shipCombatSpeedRow + size))
}

// newMonsterDesign is Clear_Structure_ and the strcpy of the E_Strings_ name.
func (g *Game) newMonsterDesign(name int) *monsterDesign {
	b := &monsterDesign{}
	if es, err := g.eStrings(); err != nil {
		g.Log("monster design: %v", err)
	} else {
		copy(b[:], es[name])
	}
	return b
}

// loadAmoebaShipDesign is Load_Amoeba_Ship_Design_ (0x57c0b); full is the
// roaming event monster, otherwise the small guard variant.
func (g *Game) loadAmoebaShipDesign(d *save.Design, full bool) {
	b := g.newMonsterDesign(0xad)
	if full {
		b[0x10], b[0x11], b[0x12] = 3, 0, 0
		b[0x13] = uint8(g.bestWarpDrive(10))
		b[0x15], b[0x16], b[0x14] = 2, 0, 1
		b.slot(0, 0x2d, 2, 0xf, 0, 0)
		b.slot(1, 0x17, 5, 0xf, 0, 0xa)
		b[0x5c] = 8
		b[0x60] = g.monsterSpeed(int(b[0x13]), int(b[0x10]))
	} else {
		b[0x10] = 2
		b.slot(0, 0x2d, 1, 0xf, 0, 0)
		b[0x15] = 2
		b[0x5c] = 0x14
		b[0x60] = g.monsterSpeed(g.bestWarpDrive(10), int(b[0x10]))
	}
	setBit(b[0x17:0x1c], 0x27)
	b[0x5e], b[0x5f] = 0, 0
	b[0x5d] = 10
	b.decode(d)
}

// loadCrystalShipDesign is Load_Crystal_Ship_Design_ (0x57e1b).
func (g *Game) loadCrystalShipDesign(d *save.Design, full bool) {
	b := g.newMonsterDesign(0xf6)
	b[0x15] = 5
	if full {
		b[0x10] = 4
		b[0x13] = uint8(g.bestWarpDrive(11))
		b[0x14] = 1
		b.slot(0, 0x2a, 1, 0xf, 2, 0)
		b.slot(1, 0x1a, 5, 0xf, 0, 0xa)
		b[0x5c] = 9
		b[0x60] = g.monsterSpeed(int(b[0x13]), int(b[0x10]))
	} else {
		b[0x10] = 2
		b.slot(0, 0x2a, 1, 0xf, 0, 0)
		b.slot(1, 0x19, 5, 0xf, 0, 0xa)
		b[0x5c] = 0x15
		b[0x60] = g.monsterSpeed(g.bestWarpDrive(11), int(b[0x10]))
	}
	b[0x5d] = 11
	b.decode(d)
}

// loadDragonShipDesign is Load_Dragon_Ship_Design_ (0x57a02).
func (g *Game) loadDragonShipDesign(d *save.Design, full bool) {
	b := g.newMonsterDesign(0x110)
	b[0x15] = 5
	if full {
		b[0x10] = 4
		b[0x14] = 1
		b[0x13] = uint8(g.bestWarpDrive(12))
		b.slot(0, 0x29, 0x14, 0xf, 4, 0)
		b.slot(1, 0x28, 1, 0xf, 0x4000, 0)
		b[0x60] = g.monsterSpeed(int(b[0x13]), int(b[0x10])) + 4
		b[0x5c] = 10
	} else {
		b[0x10] = 2
		b.slot(0, 0x29, 8, 0xf, 4, 0)
		b.slot(1, 0x28, 1, 0xf, 0, 0)
		b[0x60] = g.monsterSpeed(g.bestWarpDrive(12), int(b[0x10])) + 4
		b[0x5c] = 0x16
	}
	b[0x5d] = 12
	b.decode(d)
}

// loadEelShipDesign is Load_Eel_Ship_Design_ (0x57d14).
func (g *Game) loadEelShipDesign(d *save.Design, full bool) {
	b := g.newMonsterDesign(0x119)
	b[0x15] = 4
	if full {
		b[0x10] = 3
		b[0x13] = uint8(g.bestWarpDrive(13))
		b[0x14] = 1
		setBit(b[0x17:0x1c], 0x13)
		b.slot(0, 0x2c, 2, 0xf, 0, 0)
		b[0x60] = g.monsterSpeed(int(b[0x13]), int(b[0x10])) + 8
		b[0x5c] = 0xb
	} else {
		b[0x10] = 2
		setBit(b[0x17:0x1c], 0x13)
		b.slot(0, 0x2c, 1, 0xf, 0, 0)
		b[0x60] = g.monsterSpeed(g.bestWarpDrive(13), int(b[0x10])) + 8
		b[0x5c] = 0x17
	}
	b[0x5d] = 13
	b.decode(d)
}

// loadHydraShipDesign is Load_Hydra_Ship_Design_ (0x57b1c).
func (g *Game) loadHydraShipDesign(d *save.Design, full bool) {
	b := g.newMonsterDesign(0x144)
	b[0x15] = 2
	if full {
		b[0x10] = 4
		b[0x13] = uint8(g.bestWarpDrive(14))
		b[0x14] = 1
		b.slot(0, 0x2b, 5, 0xf, 2, 0)
		b[0x5c] = 0xc
		b[0x60] = g.monsterSpeed(int(b[0x13]), int(b[0x10]))
	} else {
		b[0x10] = 2
		b[0x16] = uint8(g.bestArmor(14))
		b.slot(0, 0x2b, 3, 0xf, 0, 0)
		b[0x5c] = 0x18
		b[0x60] = g.monsterSpeed(g.bestWarpDrive(14), int(b[0x10]))
	}
	setBit(b[0x17:0x1c], 0xa)
	b[0x5d] = 14
	b.decode(d)
}

// loadGuardianShipDesign is Load_Guardian_Ship_Design_ (0x57f2c).
func (g *Game) loadGuardianShipDesign(d *save.Design) {
	b := g.newMonsterDesign(0x293)
	b[0x12], b[0x15], b[0x10], b[0x16] = 3, 5, 4, 6
	b.slot(0, 0xb, 2, 0xf, 2, 0)
	b.slot(1, 4, 10, 0xf, 4, 0)
	b.slot(2, 0x14, 1, 0xf, 0x4500, 0)
	b.slot(3, 0x14, 1, 0xf, 0x4500, 0)
	b.slot(4, 0xd, 2, 0xf, 0, 0)
	for _, bit := range []int{3, 0xd, 1, 5, 0x19, 0x13, 0x15, 0x12} {
		setBit(b[0x17:0x1c], bit)
	}
	b[0x60] = g.monsterSpeed(g.bestWarpDrive(9), int(b[0x10]))
	b[0x5c] = 7
	b[0x5d] = 9
	b.decode(d)
}

// buildNonplayerShip is Build_Nonplayer_Ship_ (0xa16bf): a monster of the
// owner (9 Guardian … 14 Hydra) near the star, sent on to it when roaming.
func (g *Game) buildNonplayerShip(owner int16, star int16, roaming bool) int16 {
	var d save.Design
	switch owner {
	case 9:
		g.loadGuardianShipDesign(&d)
	case 10:
		g.loadAmoebaShipDesign(&d, roaming)
	case 11:
		g.loadCrystalShipDesign(&d, roaming)
	case 12:
		g.loadDragonShipDesign(&d, roaming)
	case 13:
		g.loadEelShipDesign(&d, roaming)
	case 14:
		g.loadHydraShipDesign(&d, roaming)
	default:
		return -1
	}
	ship := g.setNonplayerShipXY(star, int8(owner), &d, roaming)
	if ship > -1 {
		g.setNonplayerShipDestination(star, ship, roaming)
	}
	return ship
}

// setNonplayerShipDestination is Set_Nonplayer_Ship_Destination_ (0xa1a23):
// a roaming monster flies to the star, a guard sits at it. The original's
// debug print and key wait for a refused order are left out.
func (g *Game) setNonplayerShipDestination(star, ship int16, roaming bool) {
	s := &g.state
	sh := &s.Ships[ship]
	if roaming {
		ids := []int16{ship, -1}
		info := g.shipsTryToMoveTo(ids, int(star))
		if info.canMove {
			g.makeShipsMoveTo(ids, int(star), &info)
		}
		sh.Status = 1
		sh.Location = star + 500
		return
	}
	sh.Status = 0
	sh.Location = star
}
