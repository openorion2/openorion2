package game

import "github.com/openorion2/openorion2/internal/save"

// Apply_All_Player_Changes_ (COLCALC.C), step 20 of Next_Turn_Calc_, with
// the bankruptcy sell-off of MAINTAIN.C. Spec: docs/turn/overview.md, D5.

// Building table of ORION2.EXE (ELF addresses), 0x13-byte rows.
const (
	bldgTable = 0x17eb43
	bldgRow   = 0x13
	bldgApp   = 0x00 // i16 application that gives the building
	bldgCost  = 0x02 // i32
	bldgUpk   = 0x06 // i16 BC upkeep

	researchBldgs = 0xed904 // u8[4] the buildings Kill_Research_Buildings_ sells
)

func (g *Game) bldgCostOf(b int) int32 {
	return int32(g.exe.u32(uint32(bldgTable + b*bldgRow + bldgCost)))
}

// applyAllPlayerChanges is Apply_All_Player_Changes_ (0xe4f49).
func (g *Game) applyAllPlayerChanges() {
	s := &g.state
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		pl := &s.Players[p]
		if pl.Eliminated != 0 {
			continue
		}
		g.playerMaintenance(p)
		pl.BC += int32(pl.NetIncome)
		g.updatePlayerStats(p)
		g.checkForResearchBreakthrough(p)
		g.computePlayerTotalKnownTechCost(p)
		if pl.NetIncome < 0 && pl.BC < 50 {
			var m [msgSize]byte
			m[0] = 4
			m[2], m[3] = uint8(pl.BC), uint8(pl.BC>>8)
			m[4], m[5] = uint8(pl.NetIncome), uint8(pl.NetIncome>>8)
			g.addMsg(p, m)
		}
		if pl.FoodSurplus < 0 || pl.FreightersFree < 0 {
			s.SomethingHappened[p] = 1
		}
	}
	g.unmarkJustBuiltAIShips()
}

// unmarkJustBuiltAIShips is Unmark_Just_Built_AI_Ships_ (0x100828). For an
// owner of -1 the original reads before the player array; the port clears.
func (g *Game) unmarkJustBuiltAIShips() {
	s := &g.state
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if sh.Owner >= 8 {
			continue
		}
		if sh.Owner < 0 || s.Players[sh.Owner].Objective != 100 {
			sh.JustBuilt = 0
		}
	}
}

// checkForResearchBreakthrough is Check_For_Research_Breakthrough_
// (0xe44e0): research accumulates, then one roll against the chance.
func (g *Game) checkForResearchBreakthrough(p int) {
	pl := &g.state.Players[p]
	field := int(pl.ResearchField)
	if field == 0 {
		return
	}
	chance := g.chanceForResearchBreakthrough(p)
	pl.BreakthroughPending = 0
	pl.ResearchPoints += int32(pl.ResearchProduced)
	if int32(g.state.Random.Random(100)) <= chance {
		pl.BreakthroughPending = 1
		pl.ResearchPoints = 0
		g.givePlayerField(p, field, false)
	}
}

// givePlayerField is Give_Player_Field_ (0xe4410): the field is finished
// and its chosen application learnt, or all of them for creative races,
// gray-box fields and when all is set.
func (g *Game) givePlayerField(p, field int, all bool) {
	pl := &g.state.Players[p]
	if field >= 0x4b {
		pl.HyperLevels[field-0x4b]++
		return
	}
	g.advancePlayerField(p, field, false)
	all = all || pl.Picks[save.PickCreative] != 0
	designs := false
	switch field {
	case 0x16, 0x37, 0x39:
		all, designs = true, true
	case 0x17, 0x1c, 0x1d:
		all = true
	}
	if !all {
		g.playerGetsTechApp(p, int(pl.ResearchApp))
		return
	}
	for k := 0; k < 4; k++ {
		if a := g.techFieldApp(field, k); a > 0 && pl.AppStatus[a] == techObtainable {
			g.playerGetsTechApp(p, a)
		}
	}
	if designs {
		g.initShipDesigns(p)
	}
}

// computePlayerTotalKnownTechCost is Compute_Player_Total_Known_Tech_Cost_
// (0xe4535): each field's cost by the share of it known, plus hyper levels.
func (g *Game) computePlayerTotalKnownTechCost(p int) {
	pl := &g.state.Players[p]
	pl.KnownTechCost = 0
	for f := 0x4a; f >= 0; f-- {
		known, n := int32(0), int32(1)
		if pl.FieldStatus[f] == techKnown {
			known = 1
		}
		for k := 3; k >= 0; k-- {
			if a := int16(g.techFieldApp(f, k)); a > 0 {
				n++
				if pl.AppStatus[a] == techKnown {
					known++
				}
			}
		}
		cost := int32(g.exe.u32(uint32(techFieldCost + f*techFieldRow)))
		pl.KnownTechCost += cost * int32(int16(known)) / int32(int16(n))
	}
	for f := 0x4b; f < 0x53; f++ {
		if lvl := int32(pl.HyperLevels[f-0x4b]); lvl > 0 {
			pl.KnownTechCost += int32(g.exe.u32(uint32(techFieldCost+f*techFieldRow))) + (lvl-1)*10000
		}
	}
}

// playerMaintenance is Player_Maintenance_ (0xee0b0): research buildings
// go when flagged; a player who cannot pay sells ships, buildings, spies
// and leaders in a fixed order until the books balance.
func (g *Game) playerMaintenance(p int) {
	s := &g.state
	pl := &s.Players[p]
	if pl.Unknown59D != 0 {
		g.killResearchBuildings(p)
	}
	if int32(pl.NetIncome)+pl.BC >= 0 {
		return
	}
	// The worth of each ship against the players p meets: those in contact,
	// else the humans, else everyone.
	g.shipWorth = make([]int32, maxShips)
	defer func() { g.shipWorth = nil }()
	var vs [8]bool
	n := int32(0)
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if k != p && pl.Contact[k] != 0 {
			n++
			vs[k] = true
		}
	}
	if n == 0 {
		for k := int(s.NumPlayers) - 1; k >= 0; k-- {
			if k != p && s.Players[k].Eliminated == 0 && s.Players[k].Objective == 100 {
				n++
				vs[k] = true
			}
		}
	}
	if n == 0 {
		for k := int(s.NumPlayers) - 1; k >= 0; k-- {
			if k != p && s.Players[k].Eliminated == 0 {
				n++
				vs[k] = true
			}
		}
	}
	if n != 0 {
		for i := int(s.NumShips) - 1; i >= 0; i-- {
			sh := &s.Ships[i]
			if int(sh.Owner) != p || int8(sh.Status) >= 5 || sh.Design.Type != 0 {
				continue
			}
			for k := int(s.NumPlayers) - 1; k >= 0; k-- {
				if vs[k] {
					g.shipWorth[i] += int32(uint16(g.shipStrengthVsShips(i, k)))
				}
			}
			g.shipWorth[i] /= n * (int32(sh.Design.Size) + 1)
		}
	}
	type ks struct {
		atColony, missions bool
		typ                uint8
	}
	kill := func(list ...ks) {
		for _, k := range list {
			g.killShips(p, k.atColony, k.missions, k.typ)
		}
	}
	if int32(pl.CommandTotal)*4/3 < int32(pl.CommandUsed) {
		kill(ks{true, true, 4}, ks{true, false, 4}, ks{true, true, 2}, ks{true, true, 0},
			ks{false, true, 4}, ks{false, false, 4}, ks{false, true, 2}, ks{false, true, 0})
	}
	g.killBuildings(p, phase1Building)
	kill(ks{true, true, 4}, ks{true, false, 4}, ks{false, true, 4}, ks{false, false, 4},
		ks{true, true, 2}, ks{true, true, 0}, ks{false, true, 2}, ks{false, true, 0})
	g.killBuildings(p, phase2Building)
	kill(ks{true, false, 2}, ks{false, false, 2}, ks{true, true, 1}, ks{false, true, 1},
		ks{true, false, 1}, ks{false, false, 1})
	g.killSpy(p)
	kill(ks{true, false, 0}, ks{false, false, 0})
	g.killLeaders(p)
	g.killBuildings(p, phase3Building)
}

// killResearchBuildings is Kill_Research_Buildings_ (0xed908): four
// research buildings are sold for half their cost and dequeued.
func (g *Game) killResearchBuildings(p int) {
	s := &g.state
	pl := &s.Players[p]
	changed := false
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if int(c.Owner) != p {
			continue
		}
		for k := 4; k >= 1; k-- {
			b := int(g.exeU8(uint32(researchBldgs + k - 1)))
			if c.Buildings[b] != 0 {
				refund := g.bldgCostOf(b) / 2
				g.removeBuilding(i, b)
				pl.BC += refund
				changed = true
				if pl.Objective != 100 {
					pl.BC += int32(g.settings[0xd4])
				}
			}
			if int32(c.Queue[0]) == int32(b) {
				c.Queue[0] = -2
			}
		}
	}
	if changed {
		g.doPlayerColonyCalculations(p)
	}
}

// shipIsAtColony is Ship_Is_At_Colony_ (0xedd73): the ship's star (the one
// under it when departing) has a colony of its owner.
func (g *Game) shipIsAtColony(i int) bool {
	s := &g.state
	sh := &s.Ships[i]
	var star int
	switch sh.Status {
	case 0, 4:
		star = int(sh.Location)
	case 2:
		star = g.xyToStarID(sh.X, sh.Y)
	default:
		return false
	}
	return s.starRecord(star).ColonyOwners>>(uint8(sh.Owner)&31)&1 != 0
}

// killShips is Kill_Ships_ (0xeddf7): the least worth ship of design type
// typ is scrapped (a quarter of its cost back at an own colony) until the
// books balance or none is left; missions limits it to idle AI missions.
func (g *Game) killShips(p int, atColony, missions bool, typ uint8) {
	s := &g.state
	pl := &s.Players[p]
	for int32(pl.NetIncome)+pl.BC < 0 {
		pick, best := -1, int16(0x7fff)
		for i := int(s.NumShips) - 1; i >= 0; i-- {
			sh := &s.Ships[i]
			if int16(sh.Owner) != int16(p) || sh.Design.Type != typ || int8(sh.Status) >= 3 {
				continue
			}
			if atColony && !g.shipIsAtColony(i) {
				continue
			}
			if missions && sh.AIMission != 0 && sh.AIMission != 3 && sh.AIMission != 7 {
				continue
			}
			if w := int16(g.shipWorth[i]); w < best {
				best, pick = w, i
			}
		}
		if pick == -1 {
			return
		}
		sh := &s.Ships[pick]
		var val int32
		if g.shipIsAtColony(pick) {
			val = int32(sh.Design.Cost) / 4
		}
		pl.BC += val
		if pl.Objective != 100 {
			pl.BC += int32(g.settings[0xd4])
		}
		g.doPlayerColonyPostProduction(p)
		var m [msgSize]byte
		m[0] = 6
		m[2], m[3] = uint8(sh.Location), uint8(sh.Location>>8)
		m[4], m[5] = uint8(val), uint8(val>>8)
		m[6], m[7] = uint8(pick), uint8(pick>>8)
		g.addMsg(p, m)
		sh.Status = 7
		if sh.Officer != -1 {
			g.moveOfficerToPool(int(sh.Officer))
			sh.Officer = -1
		}
		// Update_Ship_Icons_In_Between_Turns_ follows: UI caches only.
	}
}

// phase1Building, phase2Building and phase3Building are Phase1_Building_
// (0xedae2), Phase2_Building_ (0xedb1d) and Phase3_Building_ (0xedb2d),
// the filters Kill_Buildings_ is called with.
func phase1Building(g *Game, b int) bool {
	if g.exeI16(uint32(bldgTable+b*bldgRow+bldgUpk)) <= 0 {
		return false
	}
	app := int32(g.exeI16(uint32(bldgTable + b*bldgRow + bldgApp)))
	kind := g.exeU8(uint32(int32(techAppTable) + app*techAppRow + 9))
	return kind != 3 && kind != 1 && b != 9
}

func phase2Building(g *Game, b int) bool {
	return g.exeI16(uint32(bldgTable+b*bldgRow+bldgUpk)) > 0
}

func phase3Building(_ *Game, b int) bool { return b != 9 }

// buildingWorth is Building_Worth_ (0xed9ec): its cost, weighted up for
// buildings an empire should keep.
func (g *Game) buildingWorth(c int, b int) int32 {
	s := &g.state
	col := &s.Colonies[c]
	w := g.bldgCostOf(b)
	gov := func() int32 { return int32(s.Players[col.Owner].Picks[save.PickGovernment]) / 2 }
	switch uint16(b) {
	case 7, 0xf:
		return w * 8
	case 0xc:
		return w * 7
	case 0x15:
		return w * 6
	case 0x19:
		return w * 4
	case 0x14, 0x1f:
		if gov() == 3 {
			return w
		}
		return w * 7
	case 2, 0x16:
		if uint32(gov()) > 1 {
			return w
		}
		other := 0x16
		if b == 0x16 {
			other = 2
		}
		if col.Buildings[other] != 0 {
			return w
		}
		return w * 8
	}
	return w
}

// killBuildings is Kill_Buildings_ (0xedb35): the least worth building
// that passes filter is sold for half its cost, ties broken at random,
// until the books balance or none is left.
func (g *Game) killBuildings(p int, filter func(*Game, int) bool) {
	s := &g.state
	pl := &s.Players[p]
	for int32(pl.NetIncome)+pl.BC < 0 {
		best, ties := int32(0x7fff), uint32(0)
		col, bld := -1, 0
		for i := int(s.NumColonies) - 1; i >= 0; i-- {
			if int16(s.Colonies[i].Owner) != int16(p) {
				continue
			}
			for b := 0x30; b >= 0; b-- {
				if s.Colonies[i].Buildings[b] == 0 || !filter(g, b) {
					continue
				}
				w := g.buildingWorth(i, b)
				switch {
				case w < best:
					ties, best = 1, w
					col, bld = i, b
				case w == best:
					ties++
					if s.Random.Random(ties) == 1 {
						col, bld = i, b
					}
				}
			}
		}
		if col == -1 {
			return
		}
		refund := g.bldgCostOf(bld) / 2
		g.removeBuilding(col, bld)
		pl.BC += refund
		if pl.Objective != 100 {
			pl.BC += int32(g.settings[0xd4])
		}
		g.colonyCalculation(col)
		var m [msgSize]byte
		m[2] = uint8(s.Colonies[col].Planet)
		m[4], m[5] = uint8(bld), uint8(bld>>8)
		m[6], m[7] = uint8(refund), uint8(refund>>8)
		g.addMsg(p, m)
	}
}

// killSpy is Kill_Spy_ (0xedcbf): spies abroad are recalled, one per BC of
// deficit, from player 0 up.
func (g *Game) killSpy(p int) {
	s := &g.state
	pl := &s.Players[p]
	d := int32(pl.NetIncome) + pl.BC
	if d >= 0 {
		return
	}
	d = -d
	for k := 0; k < int(s.NumPlayers) && d != 0; k++ {
		if s.Players[k].Eliminated != 0 {
			continue
		}
		n := int32(s.theirSpyNumber(p, k))
		if n == 0 {
			continue
		}
		if d >= n {
			s.setTheirSpyNumber(p, k, 0)
			d -= n
		} else {
			s.setTheirSpyNumber(p, k, int16(n-d))
			d = 0
		}
	}
	g.doPlayerColonyPostProduction(p)
}

// killLeaders is Kill_Leaders_ (0xedfe0): leaders with upkeep are
// dismissed, the highest index first, until the books balance.
func (g *Game) killLeaders(p int) {
	s := &g.state
	pl := &s.Players[p]
	for done := false; int32(pl.NetIncome)+pl.BC < 0 && !done; {
		done = true
		for o := maxLeaders - 1; o >= 0; o-- {
			if int(s.Leaders[o].Owner) != p || g.officerMaintenance(p, o) <= 0 {
				continue
			}
			g.deassignOfficer(o)
			g.doPlayerColonyCalculations(p)
			var m [msgSize]byte
			m[0], m[2] = 0x16, uint8(o)
			g.addMsg(p, m)
			done = false
			break
		}
	}
}
