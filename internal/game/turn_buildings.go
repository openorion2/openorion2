package game

import "github.com/openorion2/openorion2/internal/save"

// Adding and removing buildings and the colony queue helpers (ERICNET.C,
// COLONY.C, INVASION.C). Spec: docs/turn/overview.md, D6.

// removeBuilding is Remove_Building_ (0x145ea): the building goes, and
// whatever depended on it is recomputed or dequeued.
func (g *Game) removeBuilding(ci, b int) {
	s := &g.state
	c := &s.Colonies[ci]
	c.Buildings[b] = 0
	switch uint16(b) {
	case 3, 0xe, 0x2d:
		g.removeBuildingStar(ci, b)
	case 8, 0x28, 0x29:
		// Ships of size 2 and up in the queue need the yard.
		for i := int16(0); i < 7; i++ {
			if q := int32(c.Queue[i]); q <= -100 && s.Ships[-(q+100)].Design.Size >= 2 {
				g.deleteFromColonyQueue(ci, i)
				i--
			}
		}
		g.removeBuildingStar(ci, b)
	case 9:
		s.assignNewHomePlanet(int(c.Owner), ci)
	case 0x16:
		for i := int16(0); i < 7; i++ {
			if q := int32(c.Queue[i]); q <= -100 && s.Ships[-(q+100)].Design.Type == 2 {
				g.deleteFromColonyQueue(ci, i)
				i--
			}
		}
	case 0x17, 0x18, 0x1c:
		if s.Planets[c.Planet].Climate < 2 {
			g.deleteProdFromColonyQueue(ci, 0x2c)
		}
		g.removeBuildingPop(ci)
	case 0xf:
		g.removeBuildingPop(ci)
	}
}

func (g *Game) removeBuildingStar(ci, b int) {
	s := &g.state
	c := &s.Colonies[ci]
	s.computeStarColonyStuff(int(s.Planets[c.Planet].Star))
	if b == 0x2d {
		g.updateShipETAs(int16(c.Owner))
	}
}

func (g *Game) removeBuildingPop(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	s.colonyEnvironmentalStuff(c)
	g.enforcePopulationLimitsAtColony(ci)
	g.colonyCalculation(ci)
}

// colonyCalculation is Colony_Calculation_ (0xe2a70): one colony's
// economy and its owner's totals.
func (g *Game) colonyCalculation(ci int) {
	c := &g.state.Colonies[ci]
	if c.IsOutpost == 0 {
		g.preImportComputing(ci)
		g.passOutImports(int(c.Owner), ci)
	}
	g.updatePlayerStats(int(c.Owner))
}

// assignNewHomePlanet is Assign_New_Home_Planet_ (0xecb65): p's largest
// other colony (the last scanned on ties) becomes its home planet.
func (s *State) assignNewHomePlanet(p, except int) {
	best, size := -1, int32(0)
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if i == except || int(c.Owner) != p {
			continue
		}
		if int32(c.PopCount) >= size {
			size, best = int32(c.PopCount), i
		}
	}
	if best == -1 {
		s.Players[p].HomePlanet = -1
		return
	}
	s.Players[p].HomePlanet = s.Colonies[best].Planet
}

// deleteFromColonyQueue is Delete_From_Colony_Queue_ (0xb1e19): a queued
// ship's reserved slot is freed, then the entry is squeezed out.
func (g *Game) deleteFromColonyQueue(ci int, slot int16) {
	s := &g.state
	if slot == -1 {
		return
	}
	if q := int32(s.Colonies[ci].Queue[slot]); q <= -100 {
		sh := &s.Ships[int16(-(q + 100))]
		sh.Status = 5
		sh.Location = -1
	}
	s.squishOutColonyQueueSlot(ci, slot)
}

// squishOutColonyQueueSlot is Squish_Out_Colony_Queue_Slot_ (0xb20ed); a
// repeat marker moving into the slot is squeezed out as well.
func (s *State) squishOutColonyQueueSlot(ci int, slot int16) {
	q := &s.Colonies[ci].Queue
	for {
		for i := slot; i < 6; i++ {
			q[i] = q[i+1]
		}
		q[6] = -1
		if q[slot] != -10 {
			return
		}
	}
}

// deleteProdFromColonyQueue is Delete_Prod_From_Colony_Queue_ (0xb206f).
func (g *Game) deleteProdFromColonyQueue(ci int, prod int16) {
	for slot := g.state.firstProdColonyQueueSlot(ci, prod); slot != -1; slot = g.state.firstProdColonyQueueSlot(ci, prod) {
		g.deleteFromColonyQueue(ci, slot)
	}
}

// firstProdColonyQueueSlot is First_Prod_Colony_Queue_Slot_ (0xb1c5d).
// Codes from 0x31 up match the slot numbered code-0x31, as in the original.
func (s *State) firstProdColonyQueueSlot(ci int, prod int16) int16 {
	q := &s.Colonies[ci].Queue
	for i := int16(0); i < 7; i++ {
		if prod > 0 && prod < 0x31 {
			if prod == q[i] {
				return i
			}
			continue
		}
		if prod > -100 && prod <= -50 {
			return -1
		}
		if q[i] <= -100 {
			if prod == q[i] {
				return i
			}
			continue
		}
		if prod >= 0x31 {
			if int32(prod)-0x31 == int32(i) {
				return i
			}
			continue
		}
		switch prod {
		case -6, -4, -5, -7, -3, -2, -10, -15:
			if prod == q[i] {
				return i
			}
		}
	}
	return -1
}

// enforcePopulationLimitsAtColony is Enforce_Population_Limits_At_Colony_
// (0xec97c): the colonists are shuffled, then for each race limit from low
// to high surplus pops leave (natives first, the owner's race last).
func (g *Game) enforcePopulationLimitsAtColony(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	var limitSet [42]bool
	limit := [10]int16{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		race := c.Colonists[i] & 0xf
		if limit[race] == -1 {
			limit[race] = int16(g.colonyRacePopLimit(c, int(race)))
			if limit[race] >= 0 && limit[race] < 42 {
				limitSet[limit[race]] = true
			}
		}
	}
	shuffleInt(&s.Random, c.Colonists[:c.PopCount])
	for L := 1; L < 42; L++ {
		if !limitSet[L] {
			continue
		}
		for {
			var native, android, own, loyal, other = -1, -1, -1, -1, -1
			n := 0
			for i := int(c.PopCount) - 1; i >= 0; i-- {
				col := c.Colonists[i]
				race := int(col & 0xf)
				if int(limit[race]) > L {
					continue
				}
				n++
				switch {
				case race == 9:
					native = i
				case race == 8:
					android = i
				case int(c.Owner) == race:
					own = i
				case int(c.Owner) == int(col>>4&7):
					loyal = i
				default:
					other = i
				}
			}
			if L >= n {
				break
			}
			c.PopCount--
			victim := own
			for _, v := range []int{native, other, loyal, android} {
				if v != -1 {
					victim = v
					break
				}
			}
			c.Colonists[victim] = c.Colonists[c.PopCount]
		}
	}
	if s.colonyFood2PerFarmer(c, int(c.Owner)) == 0 {
		for i := int(c.PopCount) - 1; i >= 0; i-- {
			if col := c.Colonists[i]; col&0x180 == 0 && col&0xf != 8 {
				c.Colonists[i] = col&^0x180 | 0x80
			}
		}
	}
}

// updateShipETAs is Update_Ship_ETAs_ (0x10041c): the ETA of every moving
// ship not owned by except, by simulating its flight turn by turn.
func (g *Game) updateShipETAs(except int16) {
	s := &g.state
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if (sh.Status != 2 && sh.Status != 1) || int16(sh.Owner) == except {
			continue
		}
		dest := sh.Location
		switch sh.Status {
		case 1:
			dest -= 500
		case 2:
			dest -= 1000
		}
		D := s.starRecord(int(dest))
		x, y := sh.X, sh.Y
		var neb, stop bool
		nebp := &neb
		if sh.Navigator != 0 {
			nebp = nil
		}
		var eta int16
		for int32(x) != int32(D.X) || int32(y) != int32(D.Y) {
			eta++
			g.movePlayer1TurnToStar(int(sh.Owner), x, y, int(dest), int16(uint8(sh.Speed)), &x, &y, nebp, &stop)
		}
		sh.ETA = uint8(eta)
	}
}

// addBuilding is Add_Building_ (0x13fd9): the building stands, with its
// side effects; terraforming, Gaia, soil enrichment and artificial planets
// change the planet instead and leave no building behind.
func (g *Game) addBuilding(ci, b int) {
	s := &g.state
	c := &s.Colonies[ci]
	pl := &s.Players[c.Owner]
	planet := &s.Planets[c.Planet]
	star := int(planet.Star)
	set, starStuff, clearIgnore := true, false, false
	switch b {
	case 2:
		c.ArmorProgress = 0
		if lim := min(g.colonyTankBarracksLimit(c), 2); int32(c.Armor) < lim {
			c.Armor = int16(lim)
		}
	case 3, 0xe, 0x2d:
		starStuff = true
	case 0x29:
		c.Buildings[8] = 0
		g.deleteProdFromColonyQueue(ci, 8)
		fallthrough
	case 8:
		c.Buildings[0x28] = 0
		g.deleteProdFromColonyQueue(ci, 0x28)
		fallthrough
	case 0x28:
		starStuff, clearIgnore = true, true
	case 9:
		pl.HomePlanet = c.Planet
		for k := int(s.NumColonies) - 1; k >= 0; k-- {
			if k != ci && s.Colonies[k].Owner == c.Owner {
				g.deleteProdFromColonyQueue(k, 9)
			}
		}
	case 0xb:
		s.Stars[star].ColonizePromptOff &^= shl8(c.Owner)
	case 0xd:
		if c.Buildings[0x20] != 0 {
			c.Buildings[0x20] = 0
			pl.BC += g.bldgCostOf(0x20)
		}
		g.deleteProdFromColonyQueue(ci, 0x20)
		if c.Buildings[5] != 0 {
			c.Buildings[5] = 0
			pl.BC += g.bldgCostOf(5)
		}
		g.deleteProdFromColonyQueue(ci, 5)
	case 0x11:
		if c.Climate == 8 {
			planet.Climate = 9
			planet.FoodPerFarmer += g.exeU8(tblClimateFood+9) - g.exeU8(tblClimateFood+8)
		}
		set = false
		g.deleteProdFromColonyQueue(ci, 0x2c)
	case 0x16:
		c.MarineProgress = 0
		if lim := min(int32(int16(g.colonyInfantryBarracksLimit(c))), 4); int32(c.Marines) < lim {
			c.Marines = int16(lim)
		}
	case 0x17:
		c.Buildings[0x18] = 0
		g.deleteProdFromColonyQueue(ci, 0x18)
		fallthrough
	case 0x18:
		c.Buildings[0x1c] = 0
		g.deleteProdFromColonyQueue(ci, 0x1c)
		fallthrough
	case 0x1a, 0x1b, 0x1c, 0x2a:
		clearIgnore = true
	case 0x25:
		planet.FoodPerFarmer++
		set = false
		planet.Flags |= 2
	case 0x2c:
		g.terraform(c, planet)
		set = false
	case 0x30:
		g.artificialPlanet(star)
		set = false
	}
	if set {
		c.Buildings[b] = 1
	}
	if starStuff {
		s.computeStarColonyStuff(star)
		if b == 0x2d {
			g.updateShipETAs(int16(c.Owner))
		}
	}
	if clearIgnore {
		s.Stars[star].IgnoreCombat = 0
	}
}

// terraform is Add_Building_'s terraforming: the climate improves one step
// (barren turns tundra or desert by orbit, else at random) and the food per
// farmer follows the climate table.
func (g *Game) terraform(c *save.Colony, planet *save.Planet) {
	s := &g.state
	old := c.Climate
	switch old {
	case 2:
		st := &s.Stars[planet.Star]
		switch c.Planet {
		case st.Planets[0], st.Planets[1]:
			planet.Climate = 3
		case st.Planets[3], st.Planets[4]:
			planet.Climate = 4
		default:
			planet.Climate = 4
			if s.Random.Random(2) == 1 {
				planet.Climate = 3
			}
		}
	case 3:
		planet.Climate = 7
	case 4:
		planet.Climate = 6
	case 5, 6, 7:
		planet.Climate = 8
	}
	planet.TerraformCount++
	planet.FoodPerFarmer += g.exeU8(uint32(tblClimateFood+int(planet.Climate))) - g.exeU8(uint32(tblClimateFood+int(old)))
}

// artificialPlanet is Add_Building_'s artificial planet: the star's last
// gas giant (a large world), else its last asteroid belt (a medium one),
// becomes a barren planet.
func (g *Game) artificialPlanet(star int) {
	s := &g.state
	st := &s.Stars[star]
	found, size := int16(-1), uint8(0)
	for k := 4; k >= 0 && found == -1; k-- {
		if p := st.Planets[k]; p != -1 && s.Planets[p].Type == 2 {
			found, size = p, 4
		}
	}
	for k := 4; k >= 0 && found == -1; k-- {
		if p := st.Planets[k]; p != -1 && s.Planets[p].Type == 1 {
			found, size = p, 3
		}
	}
	if found == -1 {
		return
	}
	p := &s.Planets[found]
	p.Type = 3
	p.Gravity = 1
	p.ClimateGroup = 0
	p.Size = size
	p.Climate = 2
	p.Minerals = 2
	p.Picture = uint8(s.Random.Random(3)) - 1
	p.FoodPerFarmer = 0
	p.TerraformCount = 0
	p.MaxFarms = g.exeU8(uint32(tblArtificialFarm + int(size)))
	p.ArtificialMaxPop = g.exeU8(uint32(tblSizeTroops + int(size)))
	p.Flags = 0
}

// nProdsInColonyQueue is N_Prods_In_Colony_Queue_ (0xafe0c).
func (s *State) nProdsInColonyQueue(ci int, prod int16) int16 {
	var n int16
	for i := 0; i < 7; i++ {
		if s.Colonies[ci].Queue[i] == prod {
			n++
		}
	}
	return n
}

// colonyCanBuildProduct is Colony_Can_Build_Product_ (0xe11bc): 0 no,
// 1 yes, 2 allowed but pointless now (no room, nothing to gain).
func (g *Game) colonyCanBuildProduct(ci int, prod int16) uint8 {
	s := &g.state
	c := &s.Colonies[ci]
	pl := &s.Players[c.Owner]
	planet := &s.Planets[c.Planet]
	b2u := func(v bool) uint8 {
		if v {
			return 1
		}
		return 0
	}
	gov := func() int32 { return int32(pl.Picks[save.PickGovernment]) / 2 }
	if prod > 0 && prod < 0x31 {
		b := int(prod)
		app := g.exeI16(uint32(bldgTable + b*bldgRow + bldgApp))
		if pl.AppStatus[app] != techKnown || c.Buildings[b] != 0 {
			return 0
		}
		switch b {
		case 1:
			seen := int32(-1)
			for i := int(c.PopCount) - 1; i >= 0; i-- {
				col := c.Colonists[i]
				if col&0x400 != 0 {
					return 1
				}
				race := int32(col & 0xf)
				if race >= 8 {
					continue
				}
				if seen == -1 {
					seen = race
				} else if seen != race && gov() != 3 {
					return 1
				}
			}
			return 2
		case 3:
			r := uint8(1)
			st := &s.Stars[planet.Star]
			for k := 4; k >= 0; k-- {
				p := st.Planets[k]
				if p == -1 {
					continue
				}
				col := int(s.Planets[p].Colony)
				if col == -1 || s.Colonies[col].Owner != c.Owner {
					continue
				}
				if s.Colonies[col].Buildings[b] != 0 || s.nProdsInColonyQueue(col, 3) >= 1 {
					r = 2
				}
			}
			return r
		case 5, 0x20:
			return b2u(c.Buildings[0xd] == 0)
		case 0x28:
			if c.Buildings[8] != 0 {
				return 0
			}
			fallthrough
		case 8:
			return b2u(c.Buildings[0x29] == 0)
		case 9:
			if gov() == 3 {
				return 0
			}
			// port-only: a lost home world (no colony there, or no home
			// planet) leaves no capitol standing; the original reads
			// before the arrays.
			if pl.HomePlanet < 0 {
				return 1
			}
			home := s.Planets[pl.HomePlanet].Colony
			if home < 0 {
				return 1
			}
			return b2u(s.Colonies[home].Buildings[9] == 0)
		case 0xb:
			habitable, free := false, false
			st := &s.Stars[planet.Star]
			for k := 4; k >= 0 && !free; k-- {
				p := st.Planets[k]
				if p == -1 || s.Planets[p].Type != 3 {
					continue
				}
				habitable = true
				free = s.Planets[p].Colony == -1
			}
			switch {
			case free:
				return 1
			case habitable:
				return 2
			}
			return 0
		case 0x11:
			return b2u(c.Climate == 8)
		case 0x12:
			return 0
		case 0x14, 0x1f:
			return b2u(gov() != 3)
		case 0x1c:
			if c.Buildings[0x18] != 0 {
				return 0
			}
			fallthrough
		case 0x18:
			return b2u(c.Buildings[0x17] == 0)
		case 0x19:
			r := uint8(2)
			grav := planet.Gravity
			for i := int(c.PopCount) - 1; i >= 0; i-- {
				race := c.Colonists[i] & 0xf
				if race >= 8 {
					continue
				}
				R := &s.Players[race]
				switch grav {
				case 0:
					if R.Picks[save.PickLowG] == 0 {
						r = 1
					}
				case 1:
					if R.Picks[save.PickLowG] != 0 {
						r = 1
					}
				case 2:
					if R.Picks[save.PickHighG] == 0 {
						r = 1
					}
				}
			}
			return r
		case 0x25:
			if planet.Flags&2 != 0 {
				return 0
			}
			return b2u(c.Climate >= 3 && c.Climate <= 9)
		case 0x2c:
			return b2u(c.Climate >= 2 && c.Climate <= 7)
		case 0x2e:
			if c.FoodPerFarmer > 0 {
				return 1
			}
			return 2
		case 0x30:
			return b2u(s.starHasAsteroidOrGasGiant(int(planet.Star)))
		}
		return 1
	}
	if prod <= -100 || prod <= -50 {
		if pl.FuelRange == 0 || pl.FTLSpeed == 0 {
			return 0
		}
		var d *save.Design
		if prod > -100 {
			d = &pl.Designs[-(int32(prod) + 50)]
		} else {
			d = &s.Ships[int16(-(int32(prod) + 100))].Design
		}
		switch d.Type {
		case 0:
			if d.Size < 2 {
				return 1
			}
			yard := c.Buildings[0x28] != 0 || c.Buildings[8] != 0 || c.Buildings[0x29] != 0
			if !yard {
				return 0
			}
			switch d.Size {
			case 4:
				return b2u(pl.AppStatus[0xba] == techKnown)
			case 5:
				return b2u(pl.AppStatus[0x37] == techKnown)
			}
			return 1
		case 1:
			return b2u(pl.AppStatus[0x29] == techKnown)
		case 2:
			return b2u(c.Buildings[0x16] != 0 && pl.AppStatus[0xbd] == techKnown)
		case 4:
			return b2u(pl.AppStatus[0x6d] == techKnown)
		}
		return 0
	}
	android := func(app int) uint8 {
		if pl.AppStatus[app] != techKnown {
			return 0
		}
		if g.roomForAnotherDude(c, 8) {
			return 1
		}
		return 2
	}
	switch q := uint16(prod); {
	case q == 0xffda || q == 0xffdb:
		return 1
	case q == 0xfff1:
		return b2u(pl.AppStatus[0x45] == techKnown)
	case q == 0xfff9:
		return 1
	case q == 0xfffa:
		return android(7)
	case q == 0xfffb:
		return android(8)
	case q == 0xfffc:
		return android(6)
	case q >= 0xfffd:
		return 1
	}
	return 0
}
