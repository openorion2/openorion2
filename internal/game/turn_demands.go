package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// What an AI asks for or offers (DIPLOMAC.C, NPCDIPLO.C, COLCALC.C), and
// the player ratings diplomacy weighs. Spec: docs/turn/overview.md, phase C.

// diplomacyProposal holds the _diplomacy_proposal_* globals (0x19b580..):
// Get_Demands_ fills them, Set_Demands_ and the diplomacy screen read them.
type diplomacyProposal struct {
	gold     int16 // _diplomacy_proposal_gold_bribe
	system   int16 // _diplomacy_proposal_system_bribe, -1 none
	tribute  uint8 // _diplomacy_proposal_tribute_bribe
	rejected uint8 // _diplomacy_demand_rejected_message
	tech2    uint8 // _diplomacy_proposal_tech_bribe2
	tech1    uint8 // _diplomacy_proposal_tech_bribe1
}

// setDemands is Set_Demands_ (0x54cc0): the proposal goes into both
// players' records.
func (g *Game) setDemands(a, b int16) {
	s := &g.state
	A, B := &s.Players[a], &s.Players[b]
	pr := &g.proposal
	A.TurnScratch[1][b], A.TurnScratch[2][b] = pr.tribute, pr.tech1
	A.Demand[b], A.Diplo7B7[b] = pr.gold, pr.system
	B.TurnScratch[2][a], B.TurnScratch[1][a] = pr.tech1, pr.tribute
	B.Diplo7B7[a], B.Demand[a] = pr.system, pr.gold
}

// getDemands is Get_Demands_ (0x4f93b): what a asks of b at level lvl
// (1..10), choosing among tribute, tech, gold and a system as allowed;
// false when nothing fits. The level gates each kind and scales it.
func (g *Game) getDemands(a, b, lvl int16, tribute, tech, gold, system bool) bool {
	s := &g.state
	r := &s.Random
	pr := &g.proposal
	pr.tribute, pr.tech1, pr.gold, pr.system = 0, 0, 0, -1
	lvl = min(lvl, 10)
	var techs []uint8
	var ratio, goldLvl int16
	if tech {
		techs, _ = g.getDifferentialTechList(int(a), int(b), 0, 0xc343cb00, true)
		maxA, maxB := uint32(1), uint32(1)
		for app := 0; app < numTechApps; app++ {
			if s.Players[a].AppStatus[app] == techKnown {
				maxA = max(maxA, uint32(g.appRank(app)))
			}
			if s.Players[b].AppStatus[app] == techKnown {
				maxB = max(maxB, uint32(g.appRank(app)))
			}
		}
		ratio = 10
		if maxB < maxA {
			ratio = int16(maxB * 10 / maxA)
		}
		ratio = max(ratio, 1)
	}
	upkeep := func() int32 {
		m := uint32(s.Players[a].Maintenance)
		if m < 10 {
			m = 10
		}
		return int32(m)
	}
	if gold {
		if bc := s.Players[b].BC; bc != 0 {
			goldLvl = int16(bc / upkeep() / 10)
		}
		goldLvl = min(goldLvl, 10)
	}
	if lvl == 0 {
		return false
	}
	if lvl > 6 {
		tribute = false
	}
	if lvl > ratio || len(techs) == 0 {
		tech = false
	}
	if lvl > goldLvl {
		gold = false
	}
	if lvl < 6 {
		system = false
	}
	if !tribute && !tech && !gold && !system {
		return false
	}
	choice := 0
	switch {
	case system && (r.Random(3) == 1 || !tribute && !tech && !gold):
		choice = 3
	case tech && (r.Random(3) == 1 || !tribute && !gold):
		choice = 2
	case tribute && (r.Random(2) == 1 || !gold):
		choice = 4
	case gold:
		choice = 1
	}
	switch choice {
	case 4:
		pr.tribute = 1
		if lvl >= 3 {
			pr.tribute = 2
		}
		return true
	case 1:
		v := int32(lvl) * 10 * upkeep()
		v = min(v, s.Players[b].BC)
		if v < 100 {
			v = v / 10 * 10
		} else {
			v = v / 100 * 100
		}
		if v == 0 {
			return false
		}
		pr.gold = int16(min(v, 32000))
		return true
	case 3:
		st := g.getSurrenderSystem(b, a, lvl > 6)
		if st != -1 && s.maySurrenderStar(int(st), int(b), int(a)) {
			pr.system = int16(int8(st))
			return true
		}
		return false
	case 2:
		n := int32(len(techs))
		k := n*int32(lvl)/int32(ratio) + int32(r.Random(3)) - 2
		i := int16(n - 1 - k)
		i = max(i, 0)
		if int32(i) >= n {
			i = int16(n - 1)
		}
		pr.tech1 = techs[i]
		return true
	}
	return false
}

// getDifferentialTechList is Get_Differential_Tech_List_ (0x27094): the
// applications b knows that a may get, valued by Calc_Tech_Value_ between
// lo and hi, most valuable first when sorted.
func (g *Game) getDifferentialTechList(a, b int, lo, hi uint32, sorted bool) ([]uint8, []uint32) {
	s := &g.state
	type entry struct {
		v   uint32
		app uint8
	}
	var list []entry
	g.setCompetitionTechValues(a)
	gov := s.Players[a].Picks[save.PickGovernment]
	for app := 0; app < numTechApps; app++ {
		if s.Players[b].AppStatus[app] != techKnown {
			continue
		}
		v := uint32(g.calcTechValue(a, app))
		switch {
		case app == 0x2a && gov != 0, app == 0x5c && gov != 2, app == 0x41 && gov != 4, app == 0x4d && gov != 6:
			v = 0
		}
		if !g.playerMayGetTechApp(a, app) {
			v = 0
		}
		if v != 0 && v >= lo && v <= hi {
			list = append(list, entry{v, uint8(app)})
		}
	}
	if sorted && len(list) != 0 {
		clib.Qsort(list, 5, func(x, y *entry) int { return int(int32(y.v - x.v)) })
	}
	apps := make([]uint8, len(list))
	vals := make([]uint32, len(list))
	for i, e := range list {
		apps[i], vals[i] = e.app, e.v
	}
	return apps, vals
}

// getPossibleSurrenderSystems is Get_Possible_Surrender_Systems_ (0xe5cd4):
// a's stars without a's capitol that lie in b's range, from the last;
// none when a has a single star.
func (g *Game) getPossibleSurrenderSystems(a, b int16) []uint8 {
	s := &g.state
	fuel := s.Players[b].FuelRange
	var out []uint8
	stars := int16(0)
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		S := &s.Stars[st]
		if !maskBit(S.ColonyOwners, int(a)) {
			continue
		}
		stars++
		if !s.starInRangeOfPlayer(st, int(b), fuel) {
			continue
		}
		capitol := false
		for k := 4; k >= 0 && !capitol; k-- {
			pl := S.Planets[k]
			if pl == -1 || s.Planets[pl].Colony == -1 {
				continue
			}
			c := &s.Colonies[s.Planets[pl].Colony]
			if int16(c.Owner) == a && c.Buildings[bldCapitol] != 0 {
				capitol = true
			}
		}
		if !capitol {
			out = append(out, uint8(st))
		}
	}
	if stars == 1 {
		return nil
	}
	return out
}

// getSurrenderSystem is Get_Surrender_System_ (0xe5be3): one of a's
// possible surrender stars for b, sorted by a's population there; a pick
// from the more populous half when big, else from the less populous one.
func (g *Game) getSurrenderSystem(a, b int16, big bool) int16 {
	s := &g.state
	list := g.getPossibleSurrenderSystems(a, b)
	n := int32(len(list))
	if n == 0 {
		return -1
	}
	type entry struct{ star, pop uint8 }
	es := make([]entry, n)
	for k := n - 1; k >= 0; k-- {
		e := entry{star: list[k]}
		S := &s.Stars[e.star]
		for slot := 4; slot >= 0; slot-- {
			pl := S.Planets[slot]
			if pl == -1 || s.Planets[pl].Colony == -1 {
				continue
			}
			if c := &s.Colonies[s.Planets[pl].Colony]; int16(c.Owner) == a {
				e.pop += c.PopCount
			}
		}
		es[k] = e
	}
	clib.Qsort(es, 2, func(x, y *entry) int { return int(x.pop) - int(y.pop) })
	half := (n + 1) / 2
	var i int32
	if big {
		i = n - int32(s.Random.Random(uint32(half)))
	} else {
		i = int32(s.Random.Random(uint32(half))) - 1
	}
	return int16(es[i].star)
}

// fleetStrengthComparison is Fleet_Strength_Comparison_ (0x500cf): a's
// fleet against b's in percent, at most 800, halved for each other war of a.
func (g *Game) fleetStrengthComparison(a, b int16) uint32 {
	s := &g.state
	v := (uint32(s.Players[a].FleetStrength[b]) + 1) * 100 / (uint32(s.Players[b].FleetStrength[a]) + 1)
	v = min(v, 800)
	for k := int16(0); k < s.NumPlayers; k++ {
		if k != a && k != b && int8(s.Players[a].TreatyStatus[k]) >= 4 {
			v >>= 1
		}
	}
	return v
}

// systemProximityCount is System_Proximity_Count_ (0xdcb47): b's stars, 5
// for each shared with a, 1 for each in a's normal range.
func (g *Game) systemProximityCount(a, b int16) int32 {
	s := &g.state
	ma, mb := uint8(1)<<uint(a&7), uint8(1)<<uint(b&7)
	n := int32(0)
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		own := s.Stars[st].ColonyOwners
		switch {
		case own&mb == 0:
		case own&ma != 0:
			n += 5
		case s.starInNormalRangeOfPlayer(st, int(a)):
			n++
		}
	}
	return n
}

// bestLeaderDiplomatBonus is Best_Leader_Diplomat_Bonus_ (0xe5e09): the best
// diplomat (15 a level) or negotiator (10) among p's serving leaders.
func (g *Game) bestLeaderDiplomatBonus(p int16) int32 {
	s := &g.state
	best := int32(0)
	for k := len(s.Leaders) - 1; k >= 0; k-- {
		l := &s.Leaders[k]
		if int16(l.Owner) != p || (l.Status != 0 && l.Status != 1) {
			continue
		}
		v := int32(0)
		if l.CommonSkills&0x20 != 0 {
			v = (int32(s.ownedOfficerLevel(k)) + 1) * 15
		} else if l.CommonSkills&0x10 != 0 {
			v = (int32(s.ownedOfficerLevel(k)) + 1) * 10
		}
		best = max(best, v)
	}
	return best
}

// getPlayerMaxPopulation is Get_Player_Max_Population_ (0xe5b17).
func (g *Game) getPlayerMaxPopulation(p int16) int32 {
	s := &g.state
	n := int32(0)
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if int16(c.Owner) == p && c.IsOutpost == 0 {
			n += int32(int16(g.colonyRacePopLimit(c, int(p))))
		}
	}
	return n
}
