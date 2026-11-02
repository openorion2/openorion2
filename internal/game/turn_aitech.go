package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// The AI's view of everyone's technology (AITECH.C) and the planets it
// may still colonise (AICOLONY.C). Spec: docs/turn/ai-data.md.

const (
	aiFuelApps = 0x17ffe8 // i16 tech application, 0xa-byte rows (fuelTable row 1)
	fuelStride = 0xa
	fieldCost  = 0x17d916 // i32 research cost of a field, 0x17-byte rows
)

// aiTech is the 0x15-byte record of _aitech for one player.
const (
	aitShield      = 0x00
	aitShieldAdv   = 0x01
	aitArmor       = 0x02
	aitArmorAdv    = 0x03
	aitComputer    = 0x04
	aitComputerAdv = 0x05
	aitFuel        = 0x06
	aitPlanShield  = 0x07
	aitPlanAdv     = 0x08
	aitContacts    = 0x0a
	aitDrive       = 0x0b
	aitBeam        = 0x0c
	aitBeamAdv     = 0x0d
	aitMissile     = 0x0e
	aitMissileAdv  = 0x0f
	aitGround      = 0x10
	aitGround2     = 0x11 // i16
	aitGroundAdv   = 0x13
	aitNoPlanet    = 0x14
)

// playerBestFromTable is the Player_Best_*_ pattern (0xdc2f3..0xdc449): the
// highest row 1..n whose tech application p knows, else 0.
func (g *Game) playerBestFromTable(p, n int, table, stride uint32) int {
	for i := n; i > 0; i-- {
		if g.playerKnows(p, g.exeI16(table+uint32(i)*stride)) {
			return i
		}
	}
	return 0
}

// playerBestPlanetaryShield is Player_Best_Planetary_Shield_ (0xdc3aa).
func (g *Game) playerBestPlanetaryShield(p int) int {
	a := &g.state.Players[p].AppStatus
	switch {
	case a[0x81] == 3:
		return 3
	case a[0x82] == 3:
		return 2
	case a[0x86] == 3:
		return 1
	}
	return 0
}

// techValueOfWeapon is a weapon's field research cost / 100, as a byte.
func (g *Game) techValueOfWeapon(w int16) uint8 {
	f := g.techAppFieldOf(int(g.wpnI16(int(w), weaponApp)))
	return uint8(int32(g.exe.u32(uint32(fieldCost+f*0x17))) / 100)
}

// computeAITechInfo is Compute_AI_Tech_Info_ (0xdc47c): each player's best
// equipment, then per pair of players who is ahead, a Random(2) on ties.
func (g *Game) computeAITechInfo() {
	s := &g.state
	a := g.ai
	a.fill(aiTech, 0)
	np := int(s.NumPlayers)
	rec := func(p int) int { return a.at(aiTech) + p*0x15 }
	add := func(o int, v uint8) { a.setU8(o, a.u8(o)+v) }
	for p := np - 1; p >= 0; p-- {
		if s.Players[p].Eliminated != 0 {
			continue
		}
		r := rec(p)
		a.setU8(r+aitShield, uint8(g.playerBestFromTable(p, 5, shieldTable+shieldApp, shieldStride)))
		a.setU8(r+aitArmor, uint8(g.playerBestFromTable(p, 6, armorTable+armorTech, armorStride)))
		a.setU8(r+aitComputer, uint8(g.playerBestFromTable(p, 5, computerTable+computerApp, computerStride)))
		a.setU8(r+aitFuel, uint8(g.playerBestFromTable(p, 5, aiFuelApps, fuelStride)))
		a.setU8(r+aitPlanShield, uint8(g.playerBestPlanetaryShield(p)))
		a.setU8(r+aitDrive, uint8(g.playerBestFromTable(p, 6, driveTable+driveApp, driveStride)))
		for w := int16(1); w <= 0x14; w++ {
			if !g.playerKnows(p, g.wpnI16(int(w), weaponApp)) {
				continue
			}
			o := r + aitBeam
			if w >= 0xe {
				o = r + aitMissile
			}
			if v := g.techValueOfWeapon(w); v > a.u8(o) {
				a.setU8(o, v)
			}
		}
		var b groundBonus
		g.computePlayerGroundCombatBonuses(p, &b, true)
		v := uint16(b[6]) + uint16(b[0]) + uint16(b[8]) + uint16(b[10]) + uint16(int8(b[11])) + uint16(b[3]) + uint16(int8(b[13]))
		v += uint16(b[14] / 2)
		v += uint16(b[1] / 4)
		a.setU16(r+aitGround2, v)
		a.setU8(r+aitGround, (b[12]+1+b[4])<<2+b[2])
	}
	for p := np - 1; p >= 0; p-- {
		P := &s.Players[p]
		if P.Eliminated != 0 {
			continue
		}
		r := rec(p)
		for q := np - 1; q >= 0; q-- {
			if q != p && s.Players[q].Eliminated == 0 {
				Q := rec(q)
				diff := func(off int) int16 { return int16(uint16(a.u8(Q+off)) - uint16(a.u8(r+off))) }
				contact := P.Contact[q] != 0
				// Ahead: three times the lead with contact; level: a coin.
				ahead := func(d int16, off int) {
					switch {
					case d > 0 && contact:
						add(r+off, uint8(d)*3)
					case d > 0:
						add(r+off, uint8(d))
					case d == 0:
						add(r+off, uint8(s.Random.Random(2))-1)
					}
				}
				ahead(diff(aitShield), aitShieldAdv)
				ahead(diff(aitArmor), aitArmorAdv)
				ahead(diff(aitComputer), aitComputerAdv)
				if d := diff(aitPlanShield); d > 0 && contact {
					add(r+aitPlanAdv, uint8(d))
				}
				ahead(diff(aitBeam), aitBeamAdv)
				ahead(diff(aitMissile), aitMissileAdv)
				if contact {
					a.setU8(r+aitContacts, a.u8(r+aitContacts)+1)
					dg := diff(aitGround)
					dg2 := int16(a.u16(Q+aitGround2) - a.u16(r+aitGround2))
					v := int16(1)
					switch {
					case dg > 0 && dg2 > 0:
						v = int16(int32(dg) * int32(dg2) / 20)
					case dg > 0:
						v = dg / 4
					case dg2 > 0:
						v = dg2 / 5
					}
					if v == 0 {
						v = 1
					}
					add(r+aitGroundAdv, uint8(v))
				}
			}
			switch P.Objective {
			case 1:
				add(r+aitShieldAdv, 1)
				add(r+aitComputerAdv, 1)
			case 3:
				add(r+aitComputerAdv, 1)
			}
			if P.Personality == 2 {
				add(r+aitShieldAdv, 1)
				add(r+aitComputerAdv, 1)
			}
		}
		none := uint8(0)
		if P.AppStatus[0x1cf-0x117] != 3 && g.bestUncolonizedUndestinedPlanet(p) == -1 {
			none = 1
		}
		a.setU8(r+aitNoPlanet, none)
	}
}

// colonizable is one entry of Get_Colonizable_Planets_' list.
type colonizable struct {
	planet int16
	value  uint32
}

// bestUncolonizedUndestinedPlanet is Best_Uncolonized_Undestined_Planet_
// (0xd8c79): p's most valuable free planet no colony ship is headed for.
func (g *Game) bestUncolonizedUndestinedPlanet(p int) int16 {
	list := g.getColonizablePlanets(p)
	clib.Qsort(list, 6, func(x, y *colonizable) int { return int(int32(x.value - y.value)) })
	list = g.deleteAlreadyAssignedPlanets(p, list)
	if len(list) > 0 {
		return list[len(list)-1].planet
	}
	return -1
}

// getColonizablePlanets is Get_Colonizable_Planets_ (0xd76b8): free
// habitable planets in p's extended range with their contextual values.
func (g *Game) getColonizablePlanets(p int) []colonizable {
	s := &g.state
	a := g.ai
	var list []colonizable
	for pl := int(s.PlanetCount) - 1; pl >= 0; pl-- {
		P := &s.Planets[pl]
		if P.Colony != -1 || P.Type != 3 {
			continue
		}
		if uint32(a.u8(a.at(aiStarInExtendedRange)+int(P.Star)))&(uint32(1)<<(uint(p)&31)) == 0 {
			continue
		}
		list = append(list, colonizable{int16(pl), uint32(a.contextualPlanetValues[p][pl])})
	}
	return list
}

// deleteAlreadyAssignedPlanets is Delete_Already_Assigned_Planets_
// (0xd7764): drops planets colony ships fly or are being built for.
func (g *Game) deleteAlreadyAssignedPlanets(p int, list []colonizable) []colonizable {
	s := &g.state
	drop := func(st int16) bool {
		for i := len(list) - 1; i >= 0; i-- {
			if int16(s.Planets[list[i].planet].Star) == st {
				list = append(list[:i], list[i+1:]...)
				return true
			}
		}
		return false
	}
	if s.Players[p].AppStatus[0x172-0x117] != 3 {
		for k := int(s.NumShips) - 1; k >= 0; k-- {
			sh := &s.Ships[k]
			if int16(sh.Owner) != int16(p) || sh.Design.Type != 1 || sh.Status != save.ShipInTransit || s.canOrderShip(k) {
				continue
			}
			drop(sh.Location - 500)
		}
	}
	if s.Players[p].Objective == save.ObjectiveHuman {
		return list
	}
	for c := int(s.NumColonies) - 1; c >= 0; c-- {
		col := &s.Colonies[c]
		if int16(col.Owner) != int16(p) || col.Queue[0] != 0xb {
			continue
		}
		if !drop(int16(s.Planets[col.Planet].Star)) {
			col.Queue[0] = -2
		}
	}
	return list
}
