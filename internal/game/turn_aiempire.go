package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// Assign_Empire_Building_ (AIBUILD.C): what the AI's idle colonies build
// for the empire. Spec: docs/turn/overview.md, All_Colony_AI_.

const (
	queueAIMarker   = -38 // a colony waiting for Assign_Empire_Building_
	queueAITradeTmp = -37 // trade goods for now, -2 after All_Colony_AI_
)

// industryWorth is Industry_Worth_ (0xcfedc): pop/8 plus spare industry.
func industryWorth(c *save.Colony) int16 {
	return int16(int32(c.PopCount)/8 + int32(c.Output[1]) - int32(c.Upkeep[1]))
}

// colonyCanBuildLargeShips is Colony_Can_Build_Large_Ships_ (0xe5275): one
// of the buildings 0x28, 8, 0x29 stands.
func colonyCanBuildLargeShips(c *save.Colony) bool {
	return c.Buildings[0x28] != 0 || c.Buildings[8] != 0 || c.Buildings[0x29] != 0
}

// refitCost is Refit_Cost_ (0xd108b): twice the cost difference, at least
// half the hull's cost after the government discount.
func (g *Game) refitCost(p int, sh *save.Ship, d *save.Design) int32 {
	floor := g.costReductionForGovtType(p, int32(g.exeI16(hullCostTable+uint32(d.Size)*0x24))/2)
	v := (int32(d.Cost) - int32(sh.Design.Cost)) * 2
	if v < floor {
		v = floor
	}
	return v
}

// Ship design table fields the AI's own design loaders read.
const (
	colonyShipNames    = 0x17cfee // 16-byte names by language
	transportShipNames = 0x17d04e
	driveFTLSpeed      = 0x17fe9c // u8, 0x2e-byte rows by drive
)

// shipTypeCostForPlayer is Ship_Type_Cost_For_Player_ (0xe0d98).
func (g *Game) shipTypeCostForPlayer(typ int, p int) int32 {
	var v int32
	switch typ {
	case 1:
		v = 500
	case 2, 4:
		v = 100
	}
	return g.costReductionForGovtType(p, v)
}

// loadColonyShipDesign is Load_Colony_Ship_Design_ (0x564cd).
func (g *Game) loadColonyShipDesign(p int, d *save.Design) {
	g.loadSupportShipDesign(p, d, colonyShipNames, 1, 0x2d)
	setBit(d.Specials[:], 0xb)
}

// loadTransportShipDesign is Load_Transport_Ship_Design_ (0x5663e).
func (g *Game) loadTransportShipDesign(p int, d *save.Design) {
	g.loadSupportShipDesign(p, d, transportShipNames, 2, 0x2f)
}

// loadSupportShipDesign is what the two loaders share: a frigate with the
// best drive and the armour below the best.
func (g *Game) loadSupportShipDesign(p int, d *save.Design, names uint32, typ uint8, pic uint8) {
	*d = save.Design{}
	copy(d.Name[:], cStringAt(g.exe, names+uint32(g.settings.Language())*16)+"\x00")
	d.Size, d.Type, d.Shield = 0, typ, 0
	d.Drive = uint8(g.bestWarpDrive(p))
	d.Computer = 0
	armor := int16(g.bestArmor(p))
	if armor > 1 {
		armor--
	}
	d.Armor = uint8(armor)
	d.Picture = pic
	d.FTLSpeed = g.exeU8(driveFTLSpeed + uint32(d.Drive)*0x2e)
	d.Cost = int16(g.shipTypeCostForPlayer(int(typ), p))
	d.CombatSpeed = g.combatSpeedFor(d.Drive, d.Size)
}

// assignEmpireBuilding is Assign_Empire_Building_ (0xd10ee).
func (g *Game) assignEmpireBuilding(ca *colonyAI) {
	s := &g.state
	a := g.ai
	n := &g.needs
	p := ca.p
	pl := &s.Players[p]
	rf := a.at(aiRefit) + p*0xbba
	// The buffer's cost ([ebp-0x78]) is zeroed on entry: the design loads once.
	ca.doomDesign.Cost = 0
	war := int32(g.playerWarLevel(p))
	obj1 := pl.Objective == 1
	recalc := false
	deficit := pl.BC+int32(pl.NetIncome) < pl.Maintenance
	rich := int32(0)
	if !deficit && pl.BC >= 1500 {
		rich = int32(isqrt(uint32(int32(pl.NetIncome) / 64)))
	}
	if rich > 10 {
		rich = 10
	}
	if pl.Diplo816 != 0 && war == 0 {
		war = int32(b2u(obj1)) + 1
	}

	var list []uint8
	for ci := int(s.NumColonies) - 1; ci >= 0; ci-- {
		c := &s.Colonies[ci]
		if int16(c.Owner) != int16(p) || c.IsOutpost != 0 || c.PopCount == 0 || s.eventCheckSpaceAnomaly(ci) {
			continue
		}
		if s.colonyIsBlockaded(c) || a.colonyInfo(ci)[5] != 0 || deficit {
			del := false
			if q := c.Queue[0]; q <= -100 {
				del = s.Ships[-(int(q)+100)].Design.Type != 0
			} else if u := uint16(q); u >= 0xfff9 {
				switch {
				case u <= 0xfffc:
					del = true
				case u == 0xfffe:
					recalc, del = true, true
				}
			}
			if del {
				g.deleteFromColonyQueue(ci, 0)
			}
		}
		switch uint16(c.Queue[0]) {
		case 0xfffe:
			recalc = true
			fallthrough
		case 0xfffd, 0xffff:
			c.Queue[0] = queueAIMarker
			list = append(list, uint8(ci))
		}
	}
	if recalc {
		g.doPlayerColonyCalculations(p)
	}
	if len(list) <= 0 {
		return
	}
	noShips := pl.FuelRange == 0 || pl.FTLSpeed == 0
	best := g.bestUncolonizedUndestinedPlanet(p)
	freighters, colonize := int32(0), false
	obj2 := pl.Objective == 2
	remove := func(i int) { list = append(list[:i], list[i+1:]...) }

	// Conquered colonies, or all near human fleets in wartime, first build
	// defences and morale.
	for i := 0; i < len(list); {
		c := &s.Colonies[list[i]]
		threat := war != 0 && a.u8(a.at(aiEnrouteHumanShips)+int(s.Planets[c.Planet].Star)*10+9) != 0
		if !threat && !g.colonyIsConquered(c) {
			i++
			continue
		}
		done := false
		for _, b := range []int16{0x1a, 0x1b, 0x28, 8, 0x29, 0x1c, 0x18, 0x17} {
			if g.colonyCanBuildProduct(int(list[i]), b) != 0 {
				c.Queue[0] = b
				remove(i)
				done = true
				break
			}
		}
		if !done {
			i++
		}
	}

	if best != -1 {
		info := a.at(aiPlayerInfo) + p*0x3a
		ok := int16(a.u16(info+0xa)) == 0 ||
			war == 0 && n.strengthVsShips[p][0] > 0 ||
			n.strengthVsShips[p][2] < 10 || rich > 1
		if ok && pl.AppStatus[0x29] == techKnown && !noShips {
			v := int32(a.contextualPlanetValues[p][best])
			if war > 0 {
				v /= 4
			}
			extra := int32(int16(a.u16(info+2))) * 10
			if v/2 < extra {
				extra = v / 2
			}
			v += extra
			base := int32(200)
			if obj2 {
				base = 100
			}
			if base+int32(s.Random.Random(700)) < (rich+0x40)*v {
				colonize = true
			}
		}
	}
	clib.Qsort(list, 1, func(x, y *uint8) int {
		return int(industryWorth(&s.Colonies[*x])) - int(industryWorth(&s.Colonies[*y]))
	})
	bestStar := uint8(0)
	if best != -1 {
		bestStar = s.Planets[best].Star
	}
	if pl.AppStatus[0x28] == techKnown {
		for i := len(list) - 1; colonize && i >= 0; i-- {
			c := &s.Colonies[list[i]]
			if s.colonyIsBlockaded(c) || industryWorth(c) < 13 || s.Planets[c.Planet].Star != bestStar {
				continue
			}
			if c.Buildings[11] != 1 {
				c.Queue[0] = 0xb
				remove(i)
			}
			colonize = false
		}
	}
	if colonize && pl.AppStatus[0x29] == techKnown {
		k := int32(0)
		if obj2 && s.Random.Random(5) == 1 {
			k = 1
		}
		if k >= int32(n.colonyShips[p][0]) {
			for i := len(list) - 1; colonize && i >= 0; i-- {
				c := &s.Colonies[list[i]]
				if s.colonyIsBlockaded(c) || industryWorth(c) < 20 {
					continue
				}
				lim := int32(int16(a.u16(a.at(aiPlayerInfo)+p*0x3a+2))) + 0x32
				if int32(s.Random.Random(100)) <= lim {
					continue
				}
				g.loadColonyShipDesign(p, &pl.Designs[5])
				if sh := g.createShip(p, 5, int(list[i])); sh == -1 {
					noShips = true
				} else {
					s.Ships[sh].Status = 6
					c.Queue[0] = -(sh + 100)
					remove(i)
				}
				colonize = false
			}
		}
	}

	ww := war * 2
	for len(list) > 0 {
		last := len(list) - 1
		ci := int(list[last])
		c := &s.Colonies[ci]
		if pl.BC+int32(pl.NetIncome) < pl.Maintenance && pl.NetIncome < 0 && int16(c.Upkeep[1]) < c.Output[1] {
			c.Queue[0] = queueAITradeTmp
			list = list[:last]
			g.colonyCalculation(ci)
			continue
		}
		var w [6]uint32
		if want := n.strengthVsShips[p][2]; want > 0 && pl.FuelRange != 0 && pl.FTLSpeed != 0 && !noShips {
			v := want * 1000 / (n.strengthVsShips[p][0] + want)
			if war > 0 {
				v <<= 1
			}
			w[0] = uint32(v)
		}
		info := a.colonyInfo(ci)
		if info[5] == 0 && n.freightersWanted[p] > 0 && freighters <= 1 && pl.AppStatus[0x45] == techKnown {
			w[1] = uint32((int32(n.freightersWanted[p])+5)*100 - freighters*200)
			if war > 0 {
				w[1] /= uint32(war * war)
			}
		}
		if info[5] == 0 && n.spies[p][1] > 0 {
			w[3] = uint32((int32(n.spies[p][1]) + 10) * 100)
			if war > 0 {
				w[3] /= uint32(war * war)
			}
		}
		if info[5] == 0 && n.transports[p][1] > 0 && pl.AppStatus[0xbd] == techKnown && !noShips {
			w[4] = uint32((int32(n.transports[p][1]) + 5) * 100)
			if war > 0 {
				w[4] = w[4] * 3 >> 1
			}
		}
		w[5] = 0x14d
		if war != 0 {
			w[5] = uint32(0x15e - war*100)
		}
		for len(list) == last+1 {
			switch getWeightedChoiceInt(&s.Random, w[:]) {
			case -1, 5:
				list = list[:last]
			case 0:
				g.aiBuildWarship(ca, &list, ci, last, ww, &w, &noShips)
			case 1:
				if !s.colonyIsBlockaded(c) && industryWorth(c) >= 12 {
					c.Queue[0] = save.QueueFreighters
					list = list[:last]
					freighters++
					n.freightersWanted[p]--
				} else {
					w[1] = 0
				}
			case 2:
				if !g.aiRefitAtColony(ca, &list, ci, last, war, rf) {
					w[2] = 0
				}
			case 3:
				if !s.colonyIsBlockaded(c) && industryWorth(c) >= 15 {
					c.Queue[0] = save.QueueSpy
					list = list[:last]
					n.spies[p][1]--
				} else {
					w[3] = 0
				}
			case 4:
				ok := false
				if !s.colonyIsBlockaded(c) && industryWorth(c) >= 14 {
					if c.Buildings[0x16] == 0 {
						c.Queue[0] = 0x16
						list = list[:last]
						n.transports[p][1]--
						ok = true
					} else {
						g.loadTransportShipDesign(p, &pl.Designs[5])
						if sh := g.createShip(p, 5, ci); sh == -1 {
							noShips = true
						} else {
							s.Ships[sh].Status = 6
							c.Queue[0] = -(sh + 100)
							list = list[:last]
							n.transports[p][1]--
							ok = true
						}
					}
				}
				if !ok {
					w[4] = 0
				}
			}
		}
	}
}

// aiBuildWarship is the warship case of Assign_Empire_Building_: a hull the
// colony's spare industry (plus some of the treasury) affords, or a
// shipyard. The planet-info weights read _ai_colony_info by list position,
// not by colony, as the original does.
func (g *Game) aiBuildWarship(ca *colonyAI, list *[]uint8, ci, last int, ww int32, w *[6]uint32, noShips *bool) {
	s := &g.state
	a := g.ai
	n := &g.needs
	p := ca.p
	pl := &s.Players[p]
	c := &s.Colonies[ci]
	si := a.starInfo(int(s.Planets[c.Planet].Star))[1+p*6:]
	posInfo := a.colonyInfo(last)
	own := a.colonyInfo(ci)
	var big, mid, small bool
	ind := int32(industryWorth(c))
	if pl.NetIncome > 5 && pl.BC > 50 {
		ind += int32(isqrt(uint32(int32(pl.NetIncome)))) + int32(isqrt(isqrt(uint32(pl.BC))))
	}
	var sw [9]uint16
	cmdExtra := func() uint16 {
		if pl.CommandTotal < pl.CommandUsed {
			return uint16(pl.CommandUsed-pl.CommandTotal) * 2
		}
		return 0
	}
	cost := func(i int) int32 { return int32(pl.Designs[i].Cost) }
	if cost(0) <= (ww+0x1e)*ind {
		sw[0] = 2
		if cost(1) <= (ww+0x21)*ind {
			sw[1], sw[0] = 0x10, 1
			if c.Buildings[0x29] == 0 {
				if pl.AppStatus[0xa9] == techKnown && int32(int16(g.colonyProductCost(&s.Colonies[0], 0x29))) <= (ww+0x1f)*ind {
					sw[8] = uint16(si[0])*2 + 8 + uint16(si[1])*4 + uint16(si[2])*8 + uint16(posInfo[5])*8
					small = true
					sw[8] += cmdExtra()
				}
				if c.Buildings[8] == 0 {
					if pl.AppStatus[0x1b] == techKnown && int32(int16(g.colonyProductCost(&s.Colonies[0], 8))) <= (ww+0x21)*ind {
						sw[7] = uint16(si[0])*3 + 10 + uint16(si[1])*5 + uint16(si[2])*10 + uint16(posInfo[5])*10
						mid = true
						sw[7] += cmdExtra()
					}
					if c.Buildings[0x28] == 0 && pl.AppStatus[0xa8] == techKnown && int32(int16(g.colonyProductCost(&s.Colonies[0], 0x28))) <= (ww+0x23)*ind {
						sw[6] = uint16(si[0])*4 + 0xc + uint16(si[1])*6 + uint16(si[2])*10 + uint16(posInfo[5])*10
						big = true
						sw[6] += cmdExtra()
					}
				}
			}
		}
	}
	size := n.shipSizes[p][9]
	if colonyCanBuildLargeShips(c) {
		if size >= 2 && (ww+0x2d)*ind >= cost(2) {
			sw[1] /= 5
			sw[2] = 0x20
			if size >= 3 && (ww+0x37)*ind >= cost(3) {
				sw[3], sw[2] = 0x40, 5
				if pl.AppStatus[0xba] == techKnown && size >= 4 && cost(4) <= (ww+0x30)*ind {
					sw[4] = 100
				}
				if pl.AppStatus[0x37] == techKnown && size >= 5 {
					if ca.doomDesign.Cost == 0 {
						g.loadDoomStarShipDesign(p, &ca.doomDesign)
					}
					if int32(ca.doomDesign.Cost) <= ind*(ww+0x30) {
						sw[5] = 0x96
					}
				}
			}
		}
	} else {
		if big {
			sw[6] += uint16(own[5])*7 + 2
		}
		if mid {
			sw[7] += uint16(own[5])*6 + 2
		}
		if small {
			sw[8] += uint16(own[5])*5 + 2
		}
	}
	if n.strengthVsShips[p][0] != 0 && s.Random.Random(5) == 1 {
		sw[0] = 0
	}
	k := getWeightedChoiceShort(&s.Random, sw[:])
	switch {
	case k == -1:
		w[0] = 0
	case k >= 0 && k <= 5:
		if k == 5 {
			pl.Designs[5] = ca.doomDesign
		}
		sh := g.createShip(p, int(k), ci)
		if sh == -1 {
			*noShips = true
			w[0] = 0
			return
		}
		s.Ships[sh].Status = 6
		c.Queue[0] = -(sh + 100)
		*list = (*list)[:last]
		n.strengthVsShips[p][2] -= int32(g.meanShipStrength(int(sh)))
	case k == 6:
		c.Queue[0] = 0x28
		*list = (*list)[:last]
		n.strengthVsShips[p][2] -= int32(si[0]) + int32(si[1]) + int32(si[2])*2
	case k == 7:
		c.Queue[0] = 8
		*list = (*list)[:last]
		n.strengthVsShips[p][2] -= int32(si[0]) + int32(si[1])*2 + int32(si[2])*3
	case k == 8:
		c.Queue[0] = 0x29
		*list = (*list)[:last]
		n.strengthVsShips[p][2] -= int32(si[0]) + int32(si[1])*4 + int32(si[2])*6
	}
}

// aiRefitAtColony is the refit case of Assign_Empire_Building_ (its weight
// is never set, so it is not chosen): a ship from _airefit at the colony's
// star is refitted there.
func (g *Game) aiRefitAtColony(ca *colonyAI, list *[]uint8, ci, last int, war int32, rf int) bool {
	s := &g.state
	a := g.ai
	p := ca.p
	c := &s.Colonies[ci]
	star := int32(s.Planets[c.Planet].Star)
	for j := int32(int16(a.u16(rf))) - 1; j >= 0; j-- {
		e := rf + 2 + int(j)*6
		ship := int(int16(a.u16(e)))
		sh := &s.Ships[ship]
		if int32(sh.Location) != star || sh.Design.Size >= 2 && !colonyCanBuildLargeShips(c) {
			continue
		}
		var d *save.Design
		if sh.Design.Size == 5 {
			if ca.doomDesign.Cost == 0 {
				g.loadDoomStarShipDesign(p, &ca.doomDesign)
			}
			d = &ca.doomDesign
		} else {
			d = &s.Players[p].Designs[sh.Design.Size]
		}
		cost := g.refitCost(p, sh, d)
		if int16(cost) > d.Cost || int32(int16(cost)) > int32(industryWorth(c))*(war+10) {
			continue
		}
		if sh.Officer != -1 {
			g.moveOfficerToPool(int(sh.Officer))
			sh.Officer = -1
		}
		sh.Design = *d
		sh.Design.Cost = int16(cost)
		sh.Status = 4
		c.Queue[0] = -(int16(ship) + 100)
		*list = (*list)[:last]
		cnt := int32(int16(a.u16(rf))) - 1
		a.setU16(rf, uint16(cnt))
		copy(a.mem[e:], a.mem[e+6:e+6+int(cnt-j)*6])
		return true
	}
	return false
}
