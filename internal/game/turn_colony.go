package game

import (
	"fmt"
	"strings"

	"github.com/openorion2/openorion2/internal/save"
)

// Apply_All_Colony_Changes_ (COLCALC.C), step 21 of Next_Turn_Calc_: pop
// growth, assimilation, troops and production. Spec: docs/turn/overview.md, D6.

// Constant tables of ORION2.EXE (ELF addresses).
const (
	tblClimateFood    = 0x17d81c // u8 food per farmer by climate
	tblArtificialFarm = 0x17d7fc // u8 max farms of an artificial planet by size
	tblSizeTroops     = 0x17d806 // u8 by planet size: troop limit, artificial max pop
	tblCrewExperience = 0x17d16e // i16 experience of crew level n, 8-byte rows
)

// applyAllColonyChanges is Apply_All_Colony_Changes_ (0xe3fdc): colonies
// in ascending order, outposts skipped.
func (g *Game) applyAllColonyChanges() {
	s := &g.state
	for i := 0; i < int(s.NumColonies); i++ {
		if c := &s.Colonies[i]; c.Owner != -1 && c.IsOutpost == 0 {
			g.applyColonyChanges(i)
		}
	}
}

// applyColonyChanges is Apply_Colony_Changes_ (0xe3f6e).
func (g *Game) applyColonyChanges(ci int) {
	s := &g.state
	if s.eventCheckSpaceAnomaly(ci) {
		return
	}
	c := &s.Colonies[ci]
	if c.Occupation == 4 {
		c.Occupation = 3
	}
	if c.TurnsOwned < 0xff {
		c.TurnsOwned++
	}
	if c.PopCount == 0 {
		g.ghostColonyStuff(ci)
		return
	}
	g.applyColonyPopGrowth(ci)
	g.applyAssimilation(ci)
	g.produceGroundMilitary(c)
	g.applyProduction(ci)
}

// colonyMsg is the common message shape of the colony changes: type 0,
// the planet's low byte, a subtype and one data byte.
func colonyMsg(c *save.Colony, sub, v uint8) [msgSize]byte {
	var m [msgSize]byte
	m[2], m[3], m[4] = uint8(c.Planet), sub, v
	return m
}

// applyColonyPopGrowth is Apply_Colony_Pop_Growth_ (0xe2dca): the kill
// policy, then shrinking races, then the growing ones with the owner first
// and the other players in shuffled order.
func (g *Game) applyColonyPopGrowth(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	if s.eventCheckSpaceAnomaly(ci) || c.PopCount == 0 {
		return
	}
	owner := int(c.Owner)
	if c.Occupation == 0 {
		pick, k := -1, int16(0)
		for i := int(c.PopCount) - 1; i >= 0; i-- {
			col := c.Colonists[i]
			race := int(col & 0xf)
			if race != 9 && (race >= 8 || race == owner || col&0x400 == 0) {
				continue
			}
			k++
			if s.Random.Random(uint32(int32(k))) == 1 {
				pick = i
			}
		}
		if k == 0 {
			c.Occupation = 4
		} else {
			if k == 1 {
				c.Occupation = 4
				g.addMsg(owner, colonyMsg(c, 2, uint8(c.Colonists[pick]&0xf)))
			}
			c.PopCount--
			c.Colonists[pick] = c.Colonists[c.PopCount]
		}
	}
	if c.PopCount == 0 {
		return
	}
	pl := &s.Players[owner]
	var cnt [10]int16
	for i := 0; i < int(c.PopCount); i++ {
		cnt[c.Colonists[i]&0xf]++
	}
	var keep [10]uint8
	if cnt[owner] > 0 {
		keep[owner] = 1
	} else {
		best, most := int16(-1), int16(0)
		for d := s.NumPlayers - 1; d >= 0; d-- {
			if most < cnt[d] {
				best, most = d, cnt[d]
			}
		}
		switch {
		case best != -1:
			keep[best] = 1
		case cnt[8] > 0:
			keep[8] = 1
		default:
			keep[9] = 1
		}
	}
	shrank := false
	for r := int16(0); r < 10; r++ {
		if (r >= s.NumPlayers && r != 8 && r != 9) || c.PopGrowth[r] >= 0 {
			continue
		}
		c.PopFraction[r] += c.PopGrowth[r]
		shrank = true
		for c.PopFraction[r] < 0 {
			if int16(keep[r]) >= cnt[r] {
				c.PopFraction[r] = 0
				continue
			}
			farmer, other := -1, -1
			var nf, no int16
			for i := int16(0); i < int16(c.PopCount); i++ {
				col := c.Colonists[i]
				if int16(col&0xf) != r {
					continue
				}
				if col&0x180 == 0 {
					nf++
					if s.Random.Random(uint32(int32(nf))) == 1 {
						farmer = int(i)
					}
				} else {
					no++
					if s.Random.Random(uint32(int32(no))) == 1 {
						other = int(i)
					}
				}
			}
			victim := farmer
			if no > 0 {
				victim = other
			}
			cnt[r]--
			c.PopCount--
			c.Colonists[victim] = c.Colonists[c.PopCount]
			c.PopFraction[r] += 1000
		}
	}
	if shrank {
		food := int16(c.Upkeep[save.Food]) - c.Output[save.Food] - c.FoodImport
		ind := int16(c.Upkeep[save.Industry]) - c.Output[save.Industry]
		m := colonyMsg(c, 4, 0)
		if food > 0 {
			m[4] = uint8(food)
		}
		if ind > 0 {
			m[5] = uint8(ind)
		}
		if food > 0 || ind > 0 {
			g.addMsg(owner, m)
		}
	}
	var order [8]int16
	for d := int16(0); d < s.NumPlayers; d++ {
		order[d] = d
	}
	order[0], order[owner] = order[owner], order[0]
	shuffleSint(&s.Random, order[1:s.NumPlayers])
	for j := int16(0); j < s.NumPlayers; j++ {
		r := order[j]
		if c.PopGrowth[r] <= 0 {
			continue
		}
		c.PopFraction[r] += c.PopGrowth[r]
		for c.PopFraction[r] >= 1000 {
			n := int16(c.PopCount)
			if int16(g.colonyRacePopLimit(c, int(r))) <= n {
				c.PopFraction[r] = 999
				continue
			}
			job := g.newPopJob(ci, r, n, pl)
			col := uint32(job&3)<<7 | 0x200 | uint32(r)&0xf | uint32(owner&7)<<4
			c.Colonists[n] = col
			c.PopCount++
			c.PopFraction[r] -= 1000
		}
	}
}

// newPopJob is the job choice for a new pop of race r in
// Apply_Colony_Pop_Growth_: farmer when food runs short, worker when
// industry does, scientist under a research event, else weighted by jobs.
func (g *Game) newPopJob(ci int, r, n int16, owner *save.Player) int32 {
	s := &g.state
	c := &s.Colonies[ci]
	R := &s.Players[r]
	job := int32(-1)
	if c.FoodPerFarmer > 0 {
		v := int32(c.FoodDemand[0]) + int32(c.FoodDemand[3]) + int32(c.FoodDemand[2]) + int32(c.FoodDemand[1]) -
			2*int32(c.Output[save.Food])
		switch {
		case v > 0:
			job = 0
		case v == 0 && R.Picks[save.PickLithovore] == 0:
			job = 0
		case v == -1 && R.Picks[save.PickLithovore] == 0 && R.Picks[save.PickCybernetic] == 0:
			job = 0
		case owner.FoodSurplus < 0:
			job = 0
		case owner.FoodSurplus == 0 && owner.Picks[save.PickLithovore] == 0:
			// Only colonies after this one are looked at.
			for i := ci + 1; i < int(s.NumColonies); i++ {
				if o := &s.Colonies[i]; o.Owner == c.Owner && o.IsOutpost == 0 {
					job = 0
					break
				}
			}
		}
	}
	if job == -1 {
		up, out := int16(c.Upkeep[save.Industry]), c.Output[save.Industry]
		if up > out {
			job = 1
		} else if up == out && R.Picks[save.PickCybernetic] != 0 {
			sum := int32(c.IndustryDem[0]) + int32(c.IndustryDem[1]) + int32(c.IndustryDem[2]) + int32(c.IndustryDem[3])
			if sum%2 == 0 {
				job = 1
			}
		}
	}
	if job == -1 && s.eventCheckColonyResearching(ci) {
		job = 2
	}
	if job == -1 {
		var jobs [3]uint32
		for i := int16(0); i < n; i++ {
			// Job 3 would index past the array in the original; it never occurs.
			if j := c.Colonists[i] >> 7 & 3; j < 3 {
				jobs[j]++
			}
		}
		if jobs[1]+jobs[2] == 0 {
			job = 1
		} else {
			jobs[0] = 0
			job = int32(getWeightedChoiceInt(&s.Random, jobs[:]))
		}
	}
	return job
}

// applyAssimilation is Apply_Assimilation_ (0xe3456): conquered pops are
// assimilated one per 240 points, the rate set by government and picks.
func (g *Game) applyAssimilation(ci int) {
	s := &g.state
	if s.eventCheckSpaceAnomaly(ci) {
		return
	}
	c := &s.Colonies[ci]
	if c.Occupation != 3 {
		return
	}
	acc := uint32(c.Assimilation)
	pl := &s.Players[c.Owner]
	rate := uint32(g.exeU8(uint32(0xdd4f5 + int32(pl.Picks[save.PickGovernment]))))
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		if s.Players[c.Colonists[i]>>4&7].Picks[save.PickGovernment] == 0 {
			c.Colonists[i] = c.Colonists[i]&^0x70 | uint32(c.Owner&7)<<4
		}
	}
	if c.Buildings[1] != 0 {
		rate = 0x78
	}
	if pl.Picks[save.PickCharismatic] != 0 {
		rate *= 2
	} else if pl.Picks[save.PickRepulsive] != 0 {
		rate /= 2
	}
	acc += rate
	for ; acc >= 0xf0; acc -= 0xf0 {
		pick, k := int16(-1), int16(0)
		for i := int16(0); i < int16(c.PopCount); i++ {
			if col := c.Colonists[i]; col&0x400 != 0 && col&0xf < 8 {
				k++
				if s.Random.Random(uint32(int32(k))) == 1 {
					pick = i
				}
			}
		}
		if k == 0 {
			continue
		}
		if k == 1 {
			c.Occupation = 4
			g.addMsg(int(c.Owner), colonyMsg(c, 3, uint8(c.Colonists[pick]&0xf)))
		}
		c.Colonists[pick] = c.Colonists[pick]&^0x400&^0x70 | uint32(c.Owner&7)<<4
	}
	c.Assimilation = uint8(acc)
}

// colonyInfantryBarracksLimit, colonyInfantryLimit, colonyTankBarracksLimit
// and colonyTankLimit are Colony_Infantry_Barracks_Limit_ (0xed5e7),
// Colony_Infantry_Limit_ (0xed59d), Colony_Tank_Barracks_Limit_ (0xed674)
// and Colony_Tank_Limit_ (0xed61d); warlords divide by half as much.
func (g *Game) troopLimit(c *save.Colony, base, div int32) int32 {
	if g.state.Players[c.Owner].Picks[save.PickWarlord] != 0 {
		div /= 2
	}
	if v := base / div; v != 0 {
		return v
	}
	return 1
}

func (g *Game) colonyInfantryBarracksLimit(c *save.Colony) int32 {
	return g.troopLimit(c, int32(c.PopCount), 2)
}

func (g *Game) colonyInfantryLimit(c *save.Colony) int32 {
	return g.troopLimit(c, int32(g.exeU8(uint32(tblSizeTroops+int(g.state.Planets[c.Planet].Size)))), 2)
}

func (g *Game) colonyTankBarracksLimit(c *save.Colony) int32 {
	return g.troopLimit(c, int32(c.PopCount), 4)
}

func (g *Game) colonyTankLimit(c *save.Colony) int32 {
	return g.troopLimit(c, int32(g.exeU8(uint32(tblSizeTroops+int(g.state.Planets[c.Planet].Size)))), 4)
}

// produceGroundMilitary is Produce_Ground_Military_ (0xe3616): with
// barracks (marines) or an armor barracks, one troop per five turns up to
// the barracks limit, one lost per turn above the plain limit.
func (g *Game) produceGroundMilitary(c *save.Colony) {
	if c.Buildings[0x16] != 0 {
		if int16(g.colonyInfantryBarracksLimit(c)) > c.Marines {
			if c.MarineProgress++; c.MarineProgress == 5 {
				c.MarineProgress = 0
				c.Marines++
			}
		} else if int16(g.colonyInfantryLimit(c)) < c.Marines {
			c.Marines--
		}
	}
	if c.Buildings[2] != 0 {
		if int16(g.colonyTankBarracksLimit(c)) > c.Armor {
			if c.ArmorProgress++; c.ArmorProgress == 5 {
				c.ArmorProgress = 0
				c.Armor++
			}
		} else if int16(g.colonyTankLimit(c)) < c.Armor {
			c.Armor--
		}
	}
}

// ghostColonyStuff is Ghost_Colony_Stuff_ (0xe3e9a): an empty colony
// loses each building with chance 1/10; with none left and no settler on
// the way it is destroyed.
func (g *Game) ghostColonyStuff(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	kept := 0
	for b := 0x30; b >= 0; b-- {
		if c.Buildings[b] == 0 {
			continue
		}
		if s.Random.Random(10) == 1 {
			g.removeBuilding(ci, b)
		} else {
			kept++
		}
	}
	if kept != 0 {
		return
	}
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		pl := &s.Players[p]
		for j := int(pl.SettlersCount) - 1; j >= 0; j-- {
			if int16(pl.Settlers[j]>>8&0xff) == c.Planet {
				return
			}
		}
	}
	g.destroyColony(ci, 3)
}

// destroyColony is Destroy_Colony_ (0xdcdac): events and the governor go,
// the owner hears why (unless why is -1), and the slot is emptied.
func (g *Game) destroyColony(ci int, why int16) {
	s := &g.state
	c := &s.Colonies[ci]
	star := int(s.Planets[c.Planet].Star)
	s.killAllEventsAtColony(ci)
	lost := g.handleLeaderAtLostColony(ci)
	if why != -1 {
		var m [msgSize]byte
		m[0] = 7
		m[2], m[3] = uint8(c.Planet), uint8(c.Planet>>8)
		m[4], m[5] = uint8(why), uint8(why>>8)
		m[6], m[7] = uint8(lost), uint8(lost>>8)
		m[8] = c.IsOutpost
		g.addMsg(int(c.Owner), m)
	}
	if s.Players[c.Owner].HomePlanet == c.Planet {
		s.assignNewHomePlanet(int(c.Owner), ci)
	}
	for k := 0; k < 7; k++ {
		g.deleteFromColonyQueue(ci, 0)
	}
	s.Planets[c.Planet].Colony = -1
	*c = save.Colony{}
	c.Owner = -1
	c.Planet = -1
	s.computeStarColonyStuff(star)
}

// killAllEventsAtColony is Kill_All_Events_At_Colony_ (0x2401f): colony
// events here end, and star events once fewer than two players remain.
func (s *State) killAllEventsAtColony(ci int) {
	end := func(ev int) {
		s.EventData[ev*9+1] = 3
		s.EventData[ev*9] = 0xff
	}
	active := func(ev int) bool {
		st := s.EventData[ev*9+1]
		return st == 2 || st == 4 || st == 6
	}
	for _, ev := range []int{2, 16, 17} {
		if active(ev) && int16(ci) == s.eventWord(ev*9+3) {
			end(ev)
		}
	}
	star := int16(s.Planets[s.Colonies[ci].Planet].Star)
	for _, ev := range []int{25, 14, 24} {
		if !active(ev) || star != s.eventWord(ev*9+3) {
			continue
		}
		n := int16(0)
		for p := int16(0); p < s.NumPlayers; p++ {
			if s.Stars[star].ColonyOwners>>(uint8(p)&31)&1 != 0 {
				n++
			}
		}
		if n < 2 {
			end(ev)
		}
	}
}

// handleLeaderAtLostColony is Handle_Leader_At_Lost_Colony_ (0xdcccc): the
// star's governor leaves when this was the owner's last colony there; one
// not in transit dies. It returns the leader killed, -1 for none.
func (g *Game) handleLeaderAtLostColony(ci int) int16 {
	s := &g.state
	c := &s.Colonies[ci]
	st := &s.Stars[s.Planets[c.Planet].Star]
	o := int(st.Officer[c.Owner])
	if o == -1 {
		return -1
	}
	for orbit := 4; orbit >= 0; orbit-- {
		p := st.Planets[orbit]
		if p == -1 {
			continue
		}
		if k := int(s.Planets[p].Colony); k != -1 && s.Colonies[k].Owner == c.Owner && k != ci {
			return -1
		}
	}
	lost := int16(-1)
	if s.Leaders[o].ETA > 0 {
		g.moveOfficerToPool(o)
	} else {
		g.killOfficer(o)
		lost = int16(o)
	}
	st.Officer[c.Owner] = -1
	return lost
}

// applyProduction is Apply_Production_ (0xe36df): the industry of the turn
// goes into the queue head; a finished item is built and announced.
func (g *Game) applyProduction(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	pl := &s.Players[c.Owner]
	if s.eventCheckSpaceAnomaly(ci) {
		return
	}
	evicted := false
	if c.AutoBuild != 0 && uint16(c.Queue[0]) >= 0xfffd {
		g.playerColonyAutobuild(ci)
	}
	cost := int32(int16(g.colonyProductCost(c, c.Queue[0])))
	c.LastBuilt = -1
	switch q := uint16(c.Queue[0]); {
	case q == 0xfffd:
		if int16(c.PopCount) == g.planetMaxPopulationForPlayer(int(c.Planet), int(c.Owner)) {
			c.LastBuilt = -3
			c.Queue[0] = -1
			s.fixColonyQueue(ci)
		}
		return
	case q > 0xfffd:
		return
	}
	total := int32(c.Production) + int32(c.Pending)
	c.Pending = 0
	if int16(c.Upkeep[save.Industry]) < c.Output[save.Industry] {
		total += int32(c.Output[save.Industry]) - int32(c.Upkeep[save.Industry])
	}
	if cost <= total {
		m := colonyMsg(c, 1, 0)
		m[4], m[5] = uint8(c.Queue[0]), uint8(uint16(c.Queue[0])>>8)
		if c.Queue[0] <= -100 {
			sh := &s.Ships[int16(-(int32(c.Queue[0]) + 100))]
			if sh.Status == 4 {
				m[8] = 1
			} else if c.Buildings[0x26] != 0 {
				sh.CrewLevel++
				sh.Experience = g.exeI16(uint32(tblCrewExperience + 8*int(sh.CrewLevel)))
			}
		}
		if c.Queue[1] == -1 {
			c.LastBuilt = c.Queue[0]
		}
		switch q := c.Queue[0]; {
		case q > 0 && q < 0x31:
			g.completeBuilding(ci, int(q))
		case q <= -100:
			g.completeShip(ci)
		default:
			switch uint16(q) {
			case 0xfff9:
				if s.addTheirSpy(int(c.Owner), int(c.Owner)) == 0 {
					pl.BC += 100
					g.deleteProdFromColonyQueue(ci, -7)
				}
				pl.BC++
			case 0xfffa, 0xfffb, 0xfffc:
				if room, ev := g.addAndroid(ci); room {
					evicted = ev
				} else {
					evicted = ev
					g.addMsg(int(c.Owner), colonyMsg(c, 0xa, 0))
					total += cost
				}
			case 0xfff1:
				pl.FreightersTotal += 5
				pl.BC += 5
			}
			if c.Queue[1] != -10 {
				c.Queue[0] = -1
			} else if evicted {
				c.Queue[0] = -1
				c.Queue[1] = -1
			}
		}
		total -= cost
		s.fixColonyQueue(ci)
		if c.Queue[0] == -1 && c.AutoBuild != 0 {
			g.playerColonyAutobuild(ci)
			m[9] = 1
		}
		m[6], m[7] = uint8(c.Queue[0]), uint8(uint16(c.Queue[0])>>8)
		g.addMsg(int(c.Owner), m)
	}
	if total >= 0x7fff {
		c.Production = 0x7fff
	} else {
		c.Production = int16(total)
	}
}

// completeBuilding is the building branch of Apply_Production_.
func (g *Game) completeBuilding(ci, b int) {
	s := &g.state
	c := &s.Colonies[ci]
	pl := &s.Players[c.Owner]
	g.addBuilding(ci, b)
	pl.BC += int32(g.exeI16(uint32(bldgTable + b*bldgRow + bldgUpk)))
	keep := false
	if c.Queue[1] == -10 {
		if c.IsOutpost == 0 {
			g.colonyProductionCalculation(ci)
		}
		g.updatePlayerStats(int(c.Owner))
		keep = g.colonyCanBuildProduct(ci, c.Queue[0]) != 0
	}
	if !keep {
		c.Queue[0] = -1
	}
	switch b {
	case 0x2c:
		if g.colonyCanBuildProduct(ci, 0x2c) == 0 {
			g.deleteProdFromColonyQueue(ci, 0x2c)
		}
	case 0x30:
		g.deleteObsoleteArtificialPlanetsUnderConstruction(int(s.Planets[c.Planet].Star))
	}
}

// completeShip is the ship branch of Apply_Production_: a refit is
// re-costed, a new ship goes into service (and with repeat its copy is
// queued); colony and outpost ships clear the star's colonise prompt.
func (g *Game) completeShip(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	id := int16(-(int32(c.Queue[0]) + 100))
	sh := &s.Ships[id]
	switch sh.Status {
	case 4:
		sh.Status = 0
		sh.Design.Cost = int16(g.ownerCostOfDesign(int(c.Owner), &sh.Design)) // community patch 1.51 (0xe3aec)
		c.Queue[0] = -1
	case 6:
		sh.Status = 0
		n := int16(-1)
		if c.Queue[1] == -10 {
			n = g.createShip(int(c.Owner), 0, ci)
		}
		if n == -1 {
			c.Queue[0] = -1
			break
		}
		s.Ships[n].Design = sh.Design
		s.Ships[n].Status = 6
		c.Queue[0] = int16(-(int32(n) + 100))
	}
	sh.JustBuilt = 1
	s.Stars[sh.Location].IgnoreCombat = 0
	if sh.Design.Type == 1 || sh.Design.Type == 4 {
		s.Stars[sh.Location].ColonizePromptOff |= shl8(sh.Owner)
	}
}

// addAndroid is the android branch of Apply_Production_: a pop of race 8
// joins if the empire limit allows; a full colony evicts another pop
// first. It reports whether one was added and whether room ran out.
func (g *Game) addAndroid(ci int) (added, full bool) {
	s := &g.state
	c := &s.Colonies[ci]
	ok := true
	if int32(int16(g.colonyRacePopLimit(c, 8))) <= g.colonyNPopsOfEmpire(c, 8) {
		ok = false
	} else if !g.roomForAnotherDude(c, 8) {
		full = true
		native, conq, other, own := -1, -1, -1, -1
		for i := int(c.PopCount) - 1; i >= 0; i-- {
			col := c.Colonists[i]
			race := int(col & 0xf)
			switch {
			case race == 9:
				native = i
			case race == 8:
			case col&0x400 != 0:
				conq = i
			case race != int(c.Owner):
				other = i
			default:
				own = i
			}
		}
		victim := -1
		for _, v := range []int{native, conq, other, own} {
			if v != -1 {
				victim = v
				break
			}
		}
		if victim == -1 {
			ok = false
		} else {
			g.addMsg(int(c.Owner), colonyMsg(c, 0xb, uint8(c.Colonists[victim]&0xf)))
			c.PopCount--
			c.Colonists[victim] = c.Colonists[c.PopCount]
		}
	}
	if !ok {
		return false, full
	}
	col := uint32(0x208) | uint32(c.Owner&7)<<4
	switch uint16(c.Queue[0]) {
	case 0xfffb:
		col |= 0x80
	case 0xfffa:
		col |= 0x100
	}
	c.Colonists[c.PopCount] = col
	c.PopCount++
	return true, full
}

// colonyProductCost is Colony_Product_Cost_ (0xe0dd6): the cost of a
// queue code, less the building it upgrades.
func (g *Game) colonyProductCost(c *save.Colony, prod int16) int32 {
	s := &g.state
	if prod > 0 && prod < 0x31 {
		v := g.bldgCostOf(int(prod))
		sub := func(b int) { v -= g.bldgCostOf(b) }
		switch prod {
		case 8:
			if c.Buildings[0x28] != 0 {
				sub(0x28)
			}
		case 0x17:
			if c.Buildings[0x18] != 0 {
				sub(0x18)
			} else if c.Buildings[0x1c] != 0 {
				sub(0x1c)
			}
		case 0x18:
			if c.Buildings[0x1c] != 0 {
				sub(0x1c)
			}
		case 0x29:
			if c.Buildings[8] != 0 {
				sub(8)
			} else if c.Buildings[0x28] != 0 {
				sub(0x28)
			}
		case 0x2c:
			v += int32(s.Planets[c.Planet].TerraformCount) * 250
		}
		return v
	}
	switch {
	case prod <= -100:
		return int32(s.Ships[int16(-(int32(prod) + 100))].Design.Cost)
	case prod <= -50:
		return int32(s.Players[c.Owner].Designs[-(int32(prod) + 50)].Cost)
	}
	switch uint16(prod) {
	case 0xffef, 0xfff5:
		return g.costReductionForGovtType(int(c.Owner), 100)
	case 0xfff4:
		return g.costReductionForGovtType(int(c.Owner), 500)
	case 0xfff1, 0xfffa, 0xfffb, 0xfffc:
		return 50
	case 0xfff9:
		return 100
	}
	return 0
}

// createShip is Create_Ship_ (0xaf7b4): design slot of p built at the
// colony's star, in the first free slot reserved for p; -1 when none.
func (g *Game) createShip(p, slot, ci int) int16 {
	s := &g.state
	star := int16(s.Planets[s.Colonies[ci].Planet].Star)
	i := int16(0)
	for ; i < maxShips; i++ {
		if sh := &s.Ships[i]; int16(sh.Owner) == int16(p) && (sh.Status == 5 || s.NumShips <= i) {
			break
		}
	}
	if i >= maxShips {
		g.restoreSpare(p) // community patch 1.51
		return -1
	}
	if s.NumShips <= i {
		s.NumShips = i + 1
	}
	pl := &s.Players[p]
	sh := &s.Ships[i]
	*sh = save.Ship{Design: pl.Designs[slot]}
	g.restoreSpare(p) // community patch 1.51
	sh.Owner = int8(p)
	sh.Design.Builder = uint8(p)
	sh.Location = star
	sh.Status = 0
	sh.X, sh.Y = s.Stars[star].X, s.Stars[star].Y
	sh.Officer = -1
	if g.settings[0xd8] != 0 {
		g.numberShipName(pl, sh)
	}
	return i
}

// numberShipName is Create_Ship_'s naming under settings 0xd8: the design
// name with the hull's running number, falling back to a shorter format and
// then the plain name when 15 characters do not suffice.
func (g *Game) numberShipName(pl *save.Player, sh *save.Ship) {
	es, err := g.eStrings()
	if err != nil {
		g.Log("ship name: %v", err)
		return
	}
	name := save.CString(sh.Design.Name[:])
	n := pl.HullCounters[sh.Design.Size]
	buf := cSprintf(es[0x2a], name, n)
	if len(buf) >= 15 {
		if buf = cSprintf(es[0x48], name, n); len(buf) >= 15 {
			buf = name
		}
	}
	copy(sh.Design.Name[:], buf+"\x00")
	pl.HullCounters[sh.Design.Size]++
}

// colonyNPopsOfEmpire is Colony_N_Pops_Of_Empire_ (0xe0c6e): pops of race r.
func (g *Game) colonyNPopsOfEmpire(c *save.Colony, r int) int32 {
	var n int32
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		if int(c.Colonists[i]&0xf) == r {
			n++
		}
	}
	return n
}

// roomForAnotherDude is Room_For_Another_Dude_ (0xe0c9b): one more pop of
// race r fits when, at every limit from r's own up, fewer pops than the
// limit have a limit that low.
func (g *Game) roomForAnotherDude(c *save.Colony, r int) bool {
	var present [42]bool
	var count [10]int32
	limit := [10]int32{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	setLimit := func(race int) {
		limit[race] = int32(int16(g.colonyRacePopLimit(c, race)))
		if limit[race] >= 0 && limit[race] < 42 {
			present[limit[race]] = true
		}
	}
	setLimit(r)
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		race := int(c.Colonists[i] & 0xf)
		count[race]++
		if limit[race] == -1 {
			setLimit(race)
		}
	}
	full := false
	for v := limit[r]; v < 42; v++ {
		if v < 0 || !present[v] {
			continue
		}
		var n int32
		for j := 9; j >= 0; j-- {
			if count[j] > 0 && v >= limit[j] {
				n += count[j]
			}
		}
		if v <= n {
			full = true
		}
	}
	return !full
}

// colonyProductionCalculation is Colony_Production_Calculation_ (0xe1e84).
func (g *Game) colonyProductionCalculation(ci int) {
	g.preImportComputing(ci)
	g.passOutImports(int(g.state.Colonies[ci].Owner), ci)
}

// starHasAsteroidOrGasGiant is Star_Has_Asteroid_Or_Gas_Giant_ (0xe112b).
func (s *State) starHasAsteroidOrGasGiant(star int) bool {
	for orbit := 4; orbit >= 0; orbit-- {
		if p := s.Stars[star].Planets[orbit]; p != -1 {
			if t := s.Planets[p].Type; t >= 1 && t <= 2 {
				return true
			}
		}
	}
	return false
}

// deleteObsoleteArtificialPlanetsUnderConstruction is
// Delete_Obsolete_Artifical_Planets_Under_Construction_ (0xe116f): with no
// asteroid or gas giant left, queued artificial planets at the star go.
func (g *Game) deleteObsoleteArtificialPlanetsUnderConstruction(star int) {
	s := &g.state
	if s.starHasAsteroidOrGasGiant(star) {
		return
	}
	for orbit := 4; orbit >= 0; orbit-- {
		p := s.Stars[star].Planets[orbit]
		if p == -1 {
			continue
		}
		if col := s.Planets[p].Colony; col != -1 {
			g.deleteProdFromColonyQueue(int(col), 0x30)
		}
	}
}

// fixColonyQueue is Fix_Colony_Queue_ (0xbaa3b): empty slots and a
// leading repeat marker are squeezed out.
func (s *State) fixColonyQueue(ci int) {
	s.defragmentColonyQueue(ci)
	for s.Colonies[ci].Queue[0] == -10 {
		s.squishColonyQueue(ci, 0)
	}
}

// defragmentColonyQueue is Defragment_Colony_Queue_ (0xba9a5); a slot is
// not looked at again after the entries behind it move up.
func (s *State) defragmentColonyQueue(ci int) {
	for i := 0; i < 7; i++ {
		if s.Colonies[ci].Queue[i] == -1 {
			s.squishColonyQueue(ci, i)
		}
	}
}

// squishColonyQueue is COLONY.C's Squish_Out_Colony_Queue_Slot_ (0xba9ec),
// which unlike the one at 0xb20ed leaves repeat markers alone.
func (s *State) squishColonyQueue(ci, slot int) {
	q := &s.Colonies[ci].Queue
	for i := slot; i < 6; i++ {
		q[i] = q[i+1]
	}
	q[6] = -1
}

// cSprintf is sprintf_ with one of the game's C format strings. Like C it
// ignores arguments the format does not use (skill text "+10BC").
func cSprintf(f string, args ...any) string {
	f = cFormat(f)
	n := strings.Count(f, "%") - 2*strings.Count(f, "%%")
	return fmt.Sprintf(f, args[:min(n, len(args))]...)
}
