package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// The AI's colonies (AIDUDES.C): jobs, build queues, purchases and the
// upgrade and freighter cheats. Spec: docs/turn/overview.md, All_Colony_AI_.

// aiColonyRec is one 13-byte record of Collect_AI_Colonies_'s stack array.
type aiColonyRec struct {
	colony   uint8 // +0
	farmers  uint8 // +1 colonists made farmers
	workers  uint8 // +2 colonists made workers
	fixed    uint8 // +3 androids and natives, sorted first
	first    uint8 // +4 first colonist still free
	end      uint8 // +5 one past the last free colonist
	end2     uint8 // +6
	poll     uint8 // +7 low byte of colony +8 after the job pass
	bestR    int16 // +8 research gain of the first free colonist
	worstR   int16 // +0xa research gain of the last free colonist
	starving uint8 // +0xc
}

// colonyAI is the per-player state Colony_AI_ keeps in globals: the player
// (0x1aa234), the records (0x1aa22c), blockaded ones first (0x1aa23a of
// them), then the unblockaded (0x1aa236, from 0x1aa230).
type colonyAI struct {
	p          int
	recs       []aiColonyRec
	nBlockaded int
	nFree      int
	sortColony int          // 0x1aa224, the colony the pop comparators read
	sortRec    *aiColonyRec // 0x1aa228
	doomDesign save.Design  // Assign_Empire_Building_'s doom star buffer ([ebp-0xd6])
}

func (ca *colonyAI) free() []aiColonyRec { return ca.recs[ca.nBlockaded : ca.nBlockaded+ca.nFree] }

// colonyIsBlockaded is Colony_Is_Blockaded_ (0xdf8c1).
func (s *State) colonyIsBlockaded(c *save.Colony) bool {
	return maskBit(s.Stars[s.Planets[c.Planet].Star].Blockaded, int(c.Owner))
}

// allColonyAI is All_Colony_AI_ (0xd6f67).
func (g *Game) allColonyAI() {
	s := &g.state
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		if s.Players[p].Objective != objectiveHuman {
			g.colonyAIFor(p)
		}
	}
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		if s.Colonies[i].Queue[0] == -37 {
			s.Colonies[i].Queue[0] = save.QueueTradeGoods
		}
	}
}

// colonyAIFor is Colony_AI_ (0xd6ed4).
func (g *Game) colonyAIFor(p int) {
	s := &g.state
	ca := &colonyAI{p: p}
	g.collectAIColonies(ca)
	if g.settings.Difficulty() >= 1 && g.playerWarLevel(p) < 3 {
		g.clearBuildQueuesIfNewTech(ca, s.Players[p].Unknown323)
	}
	g.keepShipDesignsUpToDate(ca)
	g.freighterCheat(p)
	g.doShipUpgradeCheats(ca)
	g.empireBuyOutright(p)
	g.colonyAIGivenTax(ca)
}

// collectAIColonies is Collect_AI_Colonies_ (0xd5795): p's colonies with
// people and no space anomaly, blockaded ones first.
func (g *Game) collectAIColonies(ca *colonyAI) {
	s := &g.state
	ca.recs = make([]aiColonyRec, 0, s.NumColonies)
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if int16(c.Owner) != int16(ca.p) || s.eventCheckSpaceAnomaly(i) || c.IsOutpost != 0 {
			continue
		}
		ca.recs = append(ca.recs, aiColonyRec{colony: uint8(i)})
		if s.colonyIsBlockaded(c) {
			ca.nBlockaded++
		} else {
			ca.nFree++
		}
	}
	clib.Qsort(ca.recs, 13, func(a, b *aiColonyRec) int {
		return int(b2u(s.colonyIsBlockaded(&s.Colonies[b.colony]))) - int(b2u(s.colonyIsBlockaded(&s.Colonies[a.colony])))
	})
}

func b2u(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// clearBuildQueuesIfNewTech is Clear_Build_Queues_If_New_Tech_ (0xd58d4):
// after a new tech, or at idle colonies, maybe start one of the buildings
// it made available. A ship being refitted (status 4) ends the whole pass.
func (g *Game) clearBuildQueuesIfNewTech(ca *colonyAI, newTech uint8) {
	s := &g.state
	pl := &s.Players[ca.p]
	for r := len(ca.recs) - 1; r >= 0; r-- {
		ci := int(ca.recs[r].colony)
		c := &s.Colonies[ci]
		if newTech == 0 {
			switch c.Queue[0] {
			case -1, -3, -2, -10:
			default:
				continue
			}
		}
		if q := c.Queue[0]; q <= -100 && s.Ships[-(int(q)+100)].Status == 4 {
			return
		}
		var w [0x32]uint8
		w[0x31] = 0x37
		has := func(app int, b int) bool { return pl.AppStatus[app] == techKnown && c.Buildings[b] == 0 }
		pop := c.PopCount
		if pop >= 5 && has(0xa4, 0x27) {
			w[0x27] = pop + 0x19
		}
		if pop >= 5 && has(0x87, 0x1d) {
			w[0x1d] = pop + 0x14
		}
		if has(0xb2, 0x2b) {
			w[0x2b] = 0x14
		}
		if has(0x57, 0x15) {
			w[0x15] = 0x14
		}
		if has(0x16, 7) {
			w[7] = 0x23
		}
		if has(0x9c, 0x24) {
			w[0x24] = 0x14
		}
		if has(0x31, 0xc) {
			w[0xc] = 0xa
		}
		if has(0x9a, 0x22) {
			w[0x22] = 7
		}
		if pop >= 5 && has(0x98, 0x21) {
			w[0x21] = pop + 0xf
		}
		if pop >= 5 && pl.Unknown59D == 0 {
			if has(0x9b, 0x23) {
				w[0x23] = 0x14
			}
			if has(0x88, 0x1e) {
				w[0x1e] = 0xa
			}
		}
		if gov := pl.Picks[save.PickGovernment] / 2; gov != 3 {
			if gov == 1 || gov == 0 {
				if has(0x67, 0x16) && c.Buildings[2] == 0 {
					w[0x16] = 0x25
				}
				if has(0xe, 2) && c.Buildings[0x16] == 0 {
					w[2] = 0xa
				}
			}
			if pop >= 3 && has(0x56, 0x14) {
				w[0x14] = pop + 0x19
			}
			if pop >= 3 && has(0x8d, 0x1f) {
				w[0x1f] = pop + 0xa
			}
		}
		if has(0x3d, 0xf) {
			w[0xf] = 0x1e
		}
		if c.Pollution >= 5 && ca.recs[r].starving == 0 {
			if has(0x32, 0xd) {
				w[0xd] = uint8(isqrt(uint32(int32(c.Pollution)))) + 8
			}
			if has(0x13, 5) {
				w[5] = uint8(isqrt(uint32(int32(c.Pollution)))) + 8
			}
			if has(0x8e, 0x20) {
				w[0x20] = 0xa
			}
		}
		if has(0x83, 0x19) {
			switch s.Planets[c.Planet].Gravity {
			case 0:
				if pl.Picks[save.PickLowG] == 0 {
					w[0x19] = 0xa
				}
			case 1:
				if pl.Picks[save.PickLowG] != 0 {
					w[0x19] = 0xa
				}
			case 2:
				if pl.Picks[save.PickHighG] == 0 {
					w[0x19] = 0x14
				}
			}
		}
		if k := getWeightedChoiceChar(&s.Random, w[:]); k < 0x31 {
			g.clearColonyQueue(ci)
			c.Queue[0] = k
		}
	}
}

// clearColonyQueue is Clear_Colony_Queue_ (0xb20a1).
func (g *Game) clearColonyQueue(ci int) {
	for i := 0; i < 7; i++ {
		g.deleteFromColonyQueue(ci, 0)
	}
}

// keepShipDesignsUpToDate is Keep_Ship_Designs_Up_To_Date_ (0xd5cee): warships
// under construction take the current design of their hull; doom stars share
// one design, made for the first.
func (g *Game) keepShipDesignsUpToDate(ca *colonyAI) {
	s := &g.state
	pl := &s.Players[ca.p]
	var doom save.Design
	for r := len(ca.recs) - 1; r >= 0; r-- {
		q := s.Colonies[ca.recs[r].colony].Queue[0]
		if q > -100 {
			continue
		}
		sh := &s.Ships[-(int(q) + 100)]
		if sh.Design.Type != 0 {
			continue
		}
		if sh.Design.Size < 5 {
			sh.Design = pl.Designs[sh.Design.Size]
		} else {
			// The "loaded" test reads the buffer's cost ([ebp-6]): loaded once.
			if doom.Cost == 0 {
				g.loadDoomStarShipDesign(ca.p, &doom)
			}
			sh.Design = doom
		}
	}
}

// loadDoomStarShipDesign is Load_Doom_Star_Ship_Design_ (0x56460).
func (g *Game) loadDoomStarShipDesign(p int, d *save.Design) {
	g.autoDesignShip(p, d, 5, false)
	g.copyDoomStarName(d)
}

// freighterCheat is Freighter_Cheat_ (0xd5d99): short of freighters or
// food, the AI may get five free freighters.
func (g *Game) freighterCheat(p int) {
	s := &g.state
	pl := &s.Players[p]
	if pl.FreightersFree >= 0 && pl.FoodSurplus >= 0 {
		return
	}
	lim := 1 - int32(g.settings.Difficulty())*int32(pl.FreightersFree)
	if int32(s.Random.Random(50)) <= lim {
		pl.FreightersTotal += 5
		return
	}
	if pl.FreightersTotal == 0 && g.settings.Difficulty() >= 1 {
		pl.FreightersTotal = 5
	}
}

const tblUpgradeCheat = 0xd575c // u8 by difficulty

// doShipUpgradeCheats is Do_Ship_Upgrade_Cheats_ (0xd5e19): at most one
// free refit of a ship from _airefit, where no human ships come and no
// foreign colony is.
func (g *Game) doShipUpgradeCheats(ca *colonyAI) {
	s := &g.state
	a := g.ai
	p := ca.p
	rf := a.at(aiRefit) + p*0xbba
	n := int32(int16(a.u16(rf)))
	if int32(s.Stardate) <= 35035 || n == 0 {
		return
	}
	last := a.u32(rf + 2 + int(n-1)*6 + 2)
	t := uint32(g.exeU8(tblUpgradeCheat+uint32(g.settings.Difficulty()))) + last
	ok := int32(0)
	if s.Random.Random(900) <= t+t {
		ok = 1
	}
	if int16(a.u16(a.at(aiPlayerInfo)+p*0x3a)) < 2 || int32(s.Random.Random(uint32(int32(s.Players[p].Population)))) <= 8 {
		ok = 0
	}
	for i := n; i != 0 && ok != 0; {
		i--
		e := rf + 2 + int(i)*6
		ship := int(int16(a.u16(e)))
		sh := &s.Ships[ship]
		loc := int(sh.Location)
		if a.u8(a.at(aiEnrouteHumanShips)+loc*10+3) != 0 {
			continue
		}
		if s.Stars[loc].ColonyOwners&^(uint8(1)<<uint(p)) != 0 {
			continue
		}
		if sh.Design.Size < 5 {
			sh.Design = s.Players[p].Designs[sh.Design.Size]
		} else {
			g.loadDoomStarShipDesign(p, &sh.Design)
		}
		cnt := int32(int16(a.u16(rf))) - 1
		a.setU16(rf, uint16(cnt))
		copy(a.mem[e:], a.mem[e+6:e+6+int(cnt-i)*6])
		ok--
	}
}

// empireBuyOutright is Empire_Buy_Outright_ (0xd0d2f): buy colony products
// at random, weighted by speed-up per BC, colonies under threat first,
// while a reserve of 3/4 of the BC (50..1500) stays.
func (g *Game) empireBuyOutright(p int) {
	s := &g.state
	a := g.ai
	pl := &s.Players[p]
	reserve := pl.BC * 3 / 4
	if reserve > 1500 {
		reserve = 1500
	} else if reserve < 50 {
		reserve = 50
	}
	maint := int32(int16(pl.Maintenance))
	for {
		threat := false
		var maxThreat, maxCalm uint16
		var w [250]uint16
		for i := int(s.NumColonies) - 1; i >= 0; i-- {
			c := &s.Colonies[i]
			if int16(c.Owner) != int16(p) || c.Pending != 0 || s.colonyIsBlockaded(c) {
				continue
			}
			if q := uint16(c.Queue[0]); q >= 0xfffd {
				continue
			}
			cost := g.colonyCostToBuyProduct(c)
			if cost <= 0 || cost+maint >= pl.BC {
				continue
			}
			info := a.colonyInfo(i)
			if info[5] == 0 && reserve > pl.BC-cost-maint {
				continue
			}
			t := int32(g.colonyNTurnsToProduce(c))
			if t < 0 || t > 50 {
				t = 50
			}
			v := t * (t * 1000) / (cost * cost)
			if uint32(v+1) >= 0xffff {
				w[i] = 0xffff
			} else {
				w[i] = uint16(v + 1)
			}
			if info[5] != 0 {
				threat = true
				maxThreat = max(maxThreat, w[i])
			} else {
				maxCalm = max(maxCalm, w[i])
			}
		}
		if maxCalm > 0 || maxThreat > 0 {
			for i := int(s.NumColonies) - 1; i >= 0; i-- {
				if w[i] == 0 {
					continue
				}
				if threat {
					if a.colonyInfo(i)[5] == 0 || int32(w[i])*4 < int32(maxThreat)*3 {
						w[i] = 0
					}
				} else if int32(maxCalm)*3 > int32(w[i])*4 {
					w[i] = 0
				}
			}
		}
		k := getWeightedChoiceShort(&s.Random, w[:s.NumColonies])
		if k == -1 {
			return
		}
		g.colonyBuyOutright(int(k))
	}
}

// colonyBuyOutright is Colony_Buy_Outright_ (0xe109d).
func (g *Game) colonyBuyOutright(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	s.Players[c.Owner].BC -= g.colonyCostToBuyProduct(c)
	c.Pending = int16(g.colonyProductCost(c, c.Queue[0])) - c.Production
}

// colonyCostToBuyProduct is Colony_Cost_To_Buy_Product_ (0xe1085).
func (g *Game) colonyCostToBuyProduct(c *save.Colony) int32 {
	return g.colonyCostToBuyGivenProduct(c, c.Queue[0], int32(c.Production))
}

// colonyCostToBuyGivenProduct is Colony_Cost_To_Buy_Given_Product_
// (0xe1002): twice the missing cost down to half, five times down to a
// tenth and ten times below; housing, trade goods and the AI's markers are
// free.
func (g *Game) colonyCostToBuyGivenProduct(c *save.Colony, prod int16, done int32) int32 {
	switch u := uint32(int32(prod)); {
	case u >= 0xffffffda && u <= 0xffffffdb, u >= 0xfffffffd:
		return 0
	}
	cost := int32(int16(g.colonyProductCost(c, prod)))
	half, tenth := cost/2, cost/10
	switch {
	case cost <= done:
		return 0
	case half <= done:
		return (cost - done) * 2
	case tenth <= done:
		return (cost-half)*2 + (half-done)*5
	}
	return (cost-half)*2 + (half-tenth)*5 + (tenth-done)*10
}

// colonyNTurnsToProduce is Colony_N_Turns_To_Produce_ (0xe0fa4): -1 without
// spare industry.
func (g *Game) colonyNTurnsToProduce(c *save.Colony) int16 {
	cost := int16(g.colonyProductCost(c, c.Queue[0]))
	if cost <= c.Production {
		return 1
	}
	ind := c.Output[1] - int16(c.Upkeep[1])
	if ind <= 0 {
		return -1
	}
	t := (int32(cost) - int32(c.Production) + int32(ind) - 1) / int32(ind)
	if int16(t) > 0 {
		return int16(t)
	}
	return 1
}

// colonyAIGivenTax is Colony_AI_Given_Tax_ (0xd6e1d).
func (g *Game) colonyAIGivenTax(ca *colonyAI) {
	for r := ca.nBlockaded - 1; r >= 0; r-- {
		g.doBlockadedColony(ca, &ca.recs[r])
	}
	free := ca.free()
	for r := len(free) - 1; r >= 0; r-- {
		g.doUnblockadedColony(ca, &free[r])
	}
	g.assignAdditionalResearchIndustry(ca)
	g.assignEmpireBuilding(ca)
	for r := len(ca.recs) - 1; r >= 0; r-- {
		g.assignColonyBuilding(int(ca.recs[r].colony))
	}
	g.industrySanityCheck(ca)
	if ca.nFree > 0 {
		g.assignAdditionalUnblockadedFarmers(ca)
	} else {
		g.doPlayerColonyPostProduction(ca.p)
	}
}

// Colonist job bits (Colonist dwords, colony +0xc).
const (
	colJobMask   = 0x180
	colWorker    = 0x80
	colScientist = 0x100
	colAssigned  = 0x200
	colConquered = 0x400
)

func setJob(v *uint32, job uint32) { *v = *v&^colJobMask | job | colAssigned }

// colonyPopProdProduced is Colony_Pop_Prod_Produced_ (0xddfd3): a colonist's
// output at a job, x4 (x3 when conquered) and gravity; farmers halved.
func (g *Game) colonyPopProdProduced(ci int, col uint32, job int) int32 {
	s := &g.state
	c := &s.Colonies[ci]
	race := int(col & 0xf)
	var v int16
	switch job {
	case 0:
		v = g.colonyEmpireBaseFood2(ci, int(c.Owner), race)
	case 1:
		v = g.colonyEmpireBaseIndustry(ci, int(c.Owner), race)
	default:
		v = g.colonyEmpireBaseResearch(ci, int(c.Owner), race)
	}
	if v > 0 {
		if col&colConquered != 0 {
			v *= 3
		} else {
			v *= 4
		}
		v *= s.gravityProductionFactor(c, race)
	}
	if job == 0 {
		v /= 2
	}
	return int32(v)
}

func isFixedRace(col uint32) bool { r := col & 0xf; return r == 8 || r == 9 }

// assignRequiredColonists is Assign_Required_Colonists_ (0xd5fe1): counts
// the androids and natives by job, frees the others, and sorts the fixed
// ones first.
func (g *Game) assignRequiredColonists(ca *colonyAI, r *aiColonyRec) {
	s := &g.state
	ci := int(r.colony)
	c := &s.Colonies[ci]
	keepSci := s.Players[c.Owner].Unknown59D
	r.fixed, r.farmers, r.workers = 0, 0, 0
	tolerant := int16(0)
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		v := &c.Colonists[i]
		clear := true
		if isFixedRace(*v) {
			r.fixed++
			clear = false
			switch *v >> 7 & 3 {
			case 0:
				r.farmers++
			case 1:
				r.workers++
			case 2:
				clear = keepSci != 0
			}
		}
		if clear {
			*v &^= colAssigned
		}
		if race := *v & 0xf; race < 8 && s.Players[race].Picks[save.PickTolerant] == 0 {
			if *v&colConquered == 0 || c.Occupation >= 2 && c.Occupation <= 3 {
				tolerant++
			}
		}
	}
	if int32(tolerant)*2 < int32(c.PopCount) {
		if pl := &s.Players[c.Owner]; pl.Picks[save.PickLithovore] != 0 && pl.FoodSurplus >= 0 {
			r.starving = 1
		}
	}
	ca.sortColony = ci
	clib.Qsort(c.Colonists[:c.PopCount], 4, func(a, b *uint32) int {
		return int(b2u(isFixedRace(*b))) - int(b2u(isFixedRace(*a)))
	})
	r.first = r.fixed
	r.end = c.PopCount
	r.end2 = c.PopCount
}

// sortPopsFI is Sort_Pops_FI_ (0xd614d): best farmers (food over industry)
// first.
func (g *Game) sortPopsFI(ci int) func(a, b *uint32) int {
	return func(a, b *uint32) int {
		da := g.colonyPopProdProduced(ci, *a, 0) - g.colonyPopProdProduced(ci, *a, 1)
		db := g.colonyPopProdProduced(ci, *b, 0) - g.colonyPopProdProduced(ci, *b, 1)
		if int16(db) != int16(da) {
			return int(int16(db)) - int(int16(da))
		}
		return int(int16(g.colonyPopProdProduced(ci, *a, 0))) - int(int16(g.colonyPopProdProduced(ci, *b, 0)))
	}
}

// sortPopsRI is Sort_Pops_R_I_ (0xd6315): best scientists (research over
// industry) first.
func (g *Game) sortPopsRI(ci int) func(a, b *uint32) int {
	return func(a, b *uint32) int {
		ia := g.colonyPopProdProduced(ci, *a, 1)
		ib := g.colonyPopProdProduced(ci, *b, 1)
		ra := g.colonyPopProdProduced(ci, *a, 2)
		rb := g.colonyPopProdProduced(ci, *b, 2)
		da, db := int16(ra-ia), int16(rb-ib)
		switch {
		case da != db:
			return int(db) - int(da)
		case int16(ib) != int16(ia):
			return int(int16(ia)) - int(int16(ib))
		}
		return int(*a&0xf) - int(*b&0xf)
	}
}

// sortPopsFRI is Sort_Pops_FR_I_ (0xd63a6): by food plus research over
// twice industry, or Sort_Pops_R_I_ where nothing grows.
func (g *Game) sortPopsFRI(ci int) func(a, b *uint32) int {
	ri := g.sortPopsRI(ci)
	return func(a, b *uint32) int {
		if g.state.Colonies[ci].FoodPerFarmer == 0 {
			return ri(a, b)
		}
		fa := g.colonyPopProdProduced(ci, *a, 0)
		ia := g.colonyPopProdProduced(ci, *a, 1)
		ka := int16(fa + g.colonyPopProdProduced(ci, *a, 2) - 2*ia)
		fb := g.colonyPopProdProduced(ci, *b, 0)
		ib := g.colonyPopProdProduced(ci, *b, 1)
		kb := int16(g.colonyPopProdProduced(ci, *b, 2) + fb - 2*ib)
		switch {
		case kb != ka:
			return int(kb) - int(ka)
		case int16(fa) != int16(fb):
			return int(int16(fb)) - int(int16(fa))
		case int16(ia) != int16(ib):
			return int(int16(ia)) - int(int16(ib))
		}
		return int(*a&0xf) - int(*b&0xf)
	}
}

// doBlockadedColony is Do_Blockaded_Colony_ (0xd61e7): farmers until the
// colony feeds itself, the rest workers (scientists while researching an
// event).
func (g *Game) doBlockadedColony(ca *colonyAI, r *aiColonyRec) {
	s := &g.state
	ci := int(r.colony)
	c := &s.Colonies[ci]
	g.assignRequiredColonists(ca, r)
	ca.sortColony = ci
	clib.Qsort(c.Colonists[r.fixed:c.PopCount], 4, g.sortPopsFI(ci))
	if c.FoodPerFarmer > 0 {
		for spins := 0; ; {
			g.preImportComputing(ci)
			if r.first >= r.end {
				break
			}
			dem := int32(c.FoodDemand[0]) + int32(c.FoodDemand[1]) + int32(c.FoodDemand[2]) + int32(c.FoodDemand[3])
			if int32(c.Output[0])*2 >= dem {
				r.end--
				setJob(&c.Colonists[r.end], colWorker)
				continue
			}
			if r.farmers >= c.CanFarm {
				// The original spins here forever: nothing changes.
				if spins++; spins > 1000 {
					panic("Do_Blockaded_Colony_: the original loops forever")
				}
				continue
			}
			setJob(&c.Colonists[r.first], 0)
			r.farmers++
			r.first++
		}
	} else {
		job := uint32(colWorker)
		if s.eventCheckColonyResearching(ci) {
			job = colScientist
		}
		for ; r.first < r.end; r.first++ {
			setJob(&c.Colonists[r.first], job)
		}
	}
	g.preImportComputing(ci)
}

// doUnblockadedColony is Do_Unblockaded_Colony_ (0xd652c): workers from the
// end of the list until a fifth of the colony works (none while researching
// an event) and industry upkeep is met; the rest wait, best scientists first.
func (g *Game) doUnblockadedColony(ca *colonyAI, r *aiColonyRec) {
	s := &g.state
	ci := int(r.colony)
	c := &s.Colonies[ci]
	quota := (int32(c.PopCount) + 4) / 5
	g.assignRequiredColonists(ca, r)
	ca.sortColony = ci
	clib.Qsort(c.Colonists[r.fixed:c.PopCount], 4, g.sortPopsFRI(ci))
	if s.eventCheckColonyResearching(int(r.colony)) {
		quota = 0
	}
	for {
		g.preImportComputing(ci)
		if r.first >= r.end2 {
			break
		}
		if int32(r.workers) >= quota {
			dem := int32(c.IndustryDem[0]) + int32(c.IndustryDem[1]) + int32(c.IndustryDem[2]) + int32(c.IndustryDem[3])
			if int32(c.Output[1])*2 >= dem {
				break
			}
		}
		r.end2--
		setJob(&c.Colonists[r.end2], colWorker)
		r.workers++
	}
	r.end = r.end2
	r.poll = uint8(c.Pollution)
	ca.sortColony = ci
	clib.Qsort(c.Colonists[r.fixed:int(r.fixed)+int(r.end)-int(r.first)], 4, g.sortPopsRI(ci))
	g.computeMaxMinRIWorthiness(r)
}

// computeMaxMinRIWorthiness is Compute_Max_Min_R_I_Worthiness_ (0xd648a):
// research over industry of the first and the last free colonist.
func (g *Game) computeMaxMinRIWorthiness(r *aiColonyRec) {
	ci := int(r.colony)
	c := &g.state.Colonies[ci]
	if r.first >= r.end {
		r.bestR, r.worstR = -0x7fff, 0x7fff
		return
	}
	f, l := c.Colonists[r.first], c.Colonists[r.end-1]
	r.bestR = int16(g.colonyPopProdProduced(ci, f, 2) - g.colonyPopProdProduced(ci, f, 1))
	r.worstR = int16(g.colonyPopProdProduced(ci, l, 2) - g.colonyPopProdProduced(ci, l, 1))
}

// assignAdditionalResearchIndustry is Assign_Additional_Research_Industry_
// (0xd66b3): free colonists go to research or industry, one at a time where
// it gains most, until research:industry reaches the player's ratio.
func (g *Game) assignAdditionalResearchIndustry(ca *colonyAI) {
	s := &g.state
	pl := &s.Players[ca.p]
	rw, iw := int32(10), int32(0x12)
	if pl.NetIncome < 0 {
		iw += int32(isqrt(uint32(-int32(pl.NetIncome))))
	}
	switch {
	case pl.Unknown59D != 0:
		rw = 0
	case pl.Objective == 3:
		rw += 2
	case pl.Objective == 4:
		iw += 2
	}
	g.doPlayerColonyCalculations(ca.p)
	free := ca.free()
	for {
		var r *aiColonyRec
		if uint32(int32(pl.ResearchProduced)*iw) < uint32(int32(pl.IndustryProduced)*rw) {
			best := int16(-0x7fff)
			for i := len(free) - 1; i >= 0; i-- {
				if best < free[i].bestR {
					r, best = &free[i], free[i].bestR
				}
			}
			if r == nil {
				return
			}
			c := &s.Colonies[r.colony]
			v := &c.Colonists[r.first]
			*v = *v&^colJobMask | colScientist | colAssigned
			r.first++
		} else {
			worst := int16(0x7fff)
			for i := len(free) - 1; i >= 0; i-- {
				if worst > free[i].worstR {
					r, worst = &free[i], free[i].worstR
				}
			}
			if r == nil {
				return
			}
			r.end--
			setJob(&s.Colonies[r.colony].Colonists[r.end], colWorker)
		}
		g.computeMaxMinRIWorthiness(r)
		g.colonyCalculation(int(r.colony))
	}
}

// assignColonyBuilding is Assign_Colony_Building_ (0xd2754): colonies left
// idle or marked -38 choose a building (-37, trade goods for now, does not).
func (g *Game) assignColonyBuilding(ci int) {
	c := &g.state.Colonies[ci]
	if c.IsOutpost != 0 || c.PopCount == 0 {
		return
	}
	switch q := uint16(c.Queue[0]); {
	case q == 0xffda, q >= 0xfffd:
		g.assignColonyNewBuilding(ci)
	}
}

// industrySanityCheck is Industry_Sanity_Check_ (0xd6d80): colonies making
// trade goods (without the hyper-research flag) or researching an event
// turn their trailing workers into scientists.
func (g *Game) industrySanityCheck(ca *colonyAI) {
	s := &g.state
	free := ca.free()
	for r := len(free) - 1; r >= 0; r-- {
		rec := &free[r]
		ci := int(rec.colony)
		c := &s.Colonies[ci]
		if !(c.Queue[0] == save.QueueTradeGoods && s.Players[ca.p].Unknown59D == 0) && !s.eventCheckColonyResearching(ci) {
			continue
		}
		for i := int(rec.end2) - 1; int(rec.fixed) <= i && c.Colonists[i]&colJobMask == colWorker; i-- {
			c.Colonists[i] = c.Colonists[i]&^colJobMask | colScientist
		}
		g.colonyCalculation(ci)
	}
}

// fIRWorthiness is F_IR_Worthiness_ (0xd682a): food over the colonist's
// current output; workers count 1000 more while pollution rose.
func (g *Game) fIRWorthiness(r *aiColonyRec, col uint32) int32 {
	ci := int(r.colony)
	f := int32(int16(g.colonyPopProdProduced(ci, col, 0)))
	if col&colJobMask == colWorker {
		v := f - int32(int16(g.colonyPopProdProduced(ci, col, 1)))
		if int16(r.poll) < g.state.Colonies[ci].Pollution {
			v += 1000
		}
		return v
	}
	return f - int32(int16(g.colonyPopProdProduced(ci, col, 2)))
}

// sortPopsFIR is Sort_Pops_F_IR_ (0xd68cb).
func (g *Game) sortPopsFIR(ci int, r *aiColonyRec) func(a, b *uint32) int {
	val := func(col uint32, f int32) int32 {
		if col&colJobMask == colWorker {
			v := f - int32(int16(g.colonyPopProdProduced(ci, col, 1)))
			if int16(r.poll) < g.state.Colonies[ci].Pollution {
				v += 1000
			}
			return v
		}
		return f - int32(int16(g.colonyPopProdProduced(ci, col, 2)))
	}
	return func(a, b *uint32) int {
		fa := int32(int16(g.colonyPopProdProduced(ci, *a, 0)))
		fb := int32(int16(g.colonyPopProdProduced(ci, *b, 0)))
		va, vb := val(*a, fa), val(*b, fb)
		switch {
		case va != vb:
			return int(vb - va)
		case fa != fb:
			return int(fa - fb)
		case *a>>7&3 != *b>>7&3:
			return int(*a>>7&3) - int(*b>>7&3)
		}
		return int(*a&0xf) - int(*b&0xf)
	}
}

// makeFirstUnassignedIntoFarmer is Make_First_Unassigned_Into_Farmer_
// (0xd6a00).
func (g *Game) makeFirstUnassignedIntoFarmer(ca *colonyAI, r *aiColonyRec) {
	s := &g.state
	ci := int(r.colony)
	c := &s.Colonies[ci]
	before := c.Pollution
	setJob(&c.Colonists[r.first], 0)
	r.first++
	r.farmers++
	g.preImportComputing(ci)
	if c.Pollution < before {
		ca.sortColony, ca.sortRec = ci, r
		clib.Qsort(c.Colonists[r.first:r.end], 4, g.sortPopsFIR(ci, r))
	}
	if r.farmers < c.CanFarm && r.first < c.PopCount {
		r.bestR = int16(g.fIRWorthiness(r, c.Colonists[r.first]))
	} else {
		r.bestR = -0x7fff
	}
}

// assignAdditionalUnblockadedFarmers is Assign_Additional_Unblockaded_Farmers_
// (0xd6ad4): while food or freighters run short, the colonist losing least
// becomes a farmer. Without free freighters only starving colonies count,
// and the AI then may get five freighters.
func (g *Game) assignAdditionalUnblockadedFarmers(ca *colonyAI) {
	s := &g.state
	pl := &s.Players[ca.p]
	cheat, stuck := false, false
	if ca.nFree <= 0 {
		return
	}
	var list []*aiColonyRec
	free := ca.free()
	for i := len(free) - 1; i >= 0; i-- {
		r := &free[i]
		c := &s.Colonies[r.colony]
		r.first, r.end = r.fixed, r.end2
		if r.first >= r.end || r.farmers >= c.CanFarm || c.FoodPerFarmer == 0 {
			continue
		}
		ca.sortColony, ca.sortRec = int(r.colony), r
		clib.Qsort(c.Colonists[r.first:r.end], 4, g.sortPopsFIR(int(r.colony), r))
		r.bestR = int16(g.fIRWorthiness(r, c.Colonists[r.first]))
		list = append(list, r)
	}
	g.doPlayerColonyPostProduction(ca.p)
	for (pl.FoodSurplus < 0 || pl.FreightersFree < 0) && len(list) > 0 && !stuck {
		best := -1
		if pl.FreightersFree > 0 {
			best = 0
			for i := len(list) - 1; i >= 1; i-- {
				if list[best].bestR < list[i].bestR {
					best = i
				}
			}
		} else {
			cheat = true
			for i := len(list) - 1; i >= 0; i-- {
				c := &s.Colonies[list[i].colony]
				dem := int32(c.FoodDemand[0]) + int32(c.FoodDemand[1]) + int32(c.FoodDemand[2]) + int32(c.FoodDemand[3])
				if int32(c.Output[0])*2 >= dem {
					continue
				}
				if best == -1 || list[best].bestR < list[i].bestR {
					best = i
				}
			}
		}
		if best == -1 {
			stuck = true
			continue
		}
		r := list[best]
		g.makeFirstUnassignedIntoFarmer(ca, r)
		g.colonyCalculation(int(r.colony))
		if r.first < r.end && r.farmers < s.Colonies[r.colony].CanFarm {
			continue
		}
		list = append(list[:best], list[best+1:]...)
	}
	if cheat && int32(s.Random.Random(10)) <= int32(g.settings.Difficulty()) {
		pl.FreightersTotal += 5
	}
}
