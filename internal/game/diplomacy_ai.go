package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// How an AI ambassador answers the human's proposals, demands and gifts
// (DIPLOMAC.C, NPCDIPLO.C, DIP-SCRN.C). Spec: docs/screens/diplomacy-screen.md,
// "The player's diplomacy".

// Tables in .data.
const (
	personalityRelationMods = 0x180ccc // i16 by personality; index 6 for an offended pacifist
	personalityBreakChance  = 0x180db8 // i16 by personality: Random(100) at most this breaks
)

// diplomacyOffer is what Check_Treaty_Proposal_ asks for besides the treaty:
// its five out parameters (kind 1 gold, 2 star, 3 tribute, 4 tech).
type diplomacyOffer struct {
	kind, tech, gold, system, tribute int16
}

// personalityRelationMod is the personality's modifier as Diplomacy_Test_
// and Get_Demand_Response_ read it: a personality 4 that p offended (+0x727)
// takes entry 6.
func (g *Game) personalityRelationMod(a int, offended bool) int16 {
	A := &g.state.Players[a]
	if A.Personality == 4 && offended {
		return g.exeTableI16(personalityRelationMods, 6)
	}
	return g.exeTableI16(personalityRelationMods, int(A.Personality))
}

// findWorstModifier is Find_Worst_Modifier_ (0x50fdf): the lowest of a's
// four modifiers for b; 0 while the council's ruling stands.
func (g *Game) findWorstModifier(a, b int) int16 {
	s := &g.state
	if s.CouncilFlag == 3 {
		return 0
	}
	m := &s.Players[a].Modifiers
	v := min(m[0][b], m[1][b])
	v = min(v, m[2][b])
	return min(v, m[3][b])
}

// validTreatyProposal is Valid_Treaty_Proposal_ (0x5265e): may a propose
// treaty kind (1 pact, 2 alliance, 3 peace, 7 trade, 8 research) to b.
func (g *Game) validTreatyProposal(a, b int, kind uint16) bool {
	A := &g.state.Players[a]
	st := int8(A.TreatyStatus[b])
	if int16(st) == int16(kind) {
		return false
	}
	switch kind {
	case 1:
		return st < 4 && st != 2
	case 2:
		return st == 1
	case 3:
		return st >= 4
	case 7:
		return st < 4 && A.TradeTreaty[b] != 1
	case 8:
		return st < 4 && A.ResearchTreaty[b] != 1
	}
	return true
}

// diplomacyTest is Diplomacy_Test_ (0x53146): a's verdict on b's proposal
// that needs need points: 0 no, 1 or 2 only with something more, 3 yes.
// Modifier mod (0..3) counts and wears off.
func (g *Game) diplomacyTest(a, b int, need int16, mod uint16) int16 {
	s := &g.state
	A, B := &s.Players[a], &s.Players[b]
	sum := int16(A.Diplo6D7[b]) + int16(A.Relations[b]) + int16(A.Diplo7EE[b])
	if A.Personality == 4 {
		sum += int16(A.Diplo7EE[b])
	}
	need -= g.personalityRelationMod(a, A.Diplo727[b] == 1)
	v := int16(s.Random.Random(100)) + sum - need
	if mod <= 3 {
		v += A.Modifiers[mod][b]
	}
	g.adjustDiplomatModifiers(int16(a), int16(b))
	switch mod {
	case 0, 1:
		A.Modifiers[mod][b] -= int16(s.Random.Random(30)) + 20
	case 2:
		A.Modifiers[2][b] -= int16(s.Random.Random(50)) + 20
	case 3:
		A.Modifiers[3][b] -= int16(s.Random.Random(50)) + 50
	}
	if B.Picks[save.PickCharismatic] != 0 {
		v += 50
	} else if B.Picks[save.PickRepulsive] != 0 {
		v -= 50
	}
	if B.Picks[save.PickTelepathic] != 0 {
		v += 25
	}
	if B.AppStatus[0xc8] == techKnown {
		v += 30
	}
	v += int16(g.bestLeaderDiplomatBonus(int16(b)))
	switch {
	case v < -75:
		return 0
	case v < -50:
		return 2
	case v < 0:
		return 1
	}
	return 3
}

// treatyRefusal is the reason Check_Treaty_Proposal_ gives for a plain no:
// four Random(4) chances at a grudge, else 0x8a.
func (g *Game) treatyRefusal(a, b int, resp *int16) {
	s := &g.state
	A := &s.Players[a]
	r := &s.Random
	switch {
	case r.Random(4) == 1 && A.Diplo6DF[b] != 0:
		*resp = 0x8b
	case r.Random(4) == 1 && A.Personality == 4 && A.Diplo727[b] != 0:
		*resp = 0x8c
	case r.Random(4) == 1 && A.Diplo7EE[b] != 0:
		*resp = 0x8d
	case r.Random(4) == 1 && A.Diplo6CF[b] != 0:
		*resp = int16(A.Diplo6CF[b]) + 0x8d
	default:
		*resp = 0x8a
	}
}

// treatyAcceptance is the yes of Check_Treaty_Proposal_: with ally, a
// Random(3) chance to mention the alliance; a Random(4) chance to mention
// b's last gift; else msg.
func (g *Game) treatyAcceptance(a, b int, ally bool, msg int16, resp *int16) {
	s := &g.state
	r := &s.Random
	if ally && r.Random(3) == 1 && s.Players[a].TreatyStatus[b] == 2 {
		*resp = 0x88
		return
	}
	if r.Random(4) == 1 && s.Players[b].Diplo6E7[a] != 0 {
		*resp = 0x89
		return
	}
	*resp = msg
}

// treatyBribe is the end of Check_Treaty_Proposal_ when a wants more: the
// demand Get_Demands_ found becomes the offer, each kind in turn.
func (g *Game) treatyBribe(a, b int16, resp *int16, off *diplomacyOffer) {
	pr := &g.proposal
	if pr.tribute != 0 {
		off.kind, off.tribute = 3, int16(int8(pr.tribute))
		*resp = 0xaa
		g.setDemands(a, b)
	}
	if pr.gold != 0 {
		off.kind, off.gold = 1, pr.gold
		*resp = 0xaa
		g.setDemands(a, b)
	}
	if pr.system != -1 {
		off.kind, off.system = 2, pr.system
		*resp = 0xaa
		g.setDemands(a, b)
	}
	if pr.tech1 != 0 {
		off.kind, off.tech = 4, int16(pr.tech1)
		*resp = 0xaa
		g.setDemands(a, b)
	}
}

// checkTreatyProposal is Check_Treaty_Proposal_ (0x5272d): AI a's answer to
// b's proposal of treaty kind, with the response message; 1 or 2 come with
// the price in off, 3 is a yes.
func (g *Game) checkTreatyProposal(a, b int16, kind uint16, resp *int16, off *diplomacyOffer) int16 {
	s := &g.state
	A := &s.Players[a]
	r := &s.Random
	off.kind = 0
	lvl := int16(4)
	var ret int16
	ai, bi := int(a), int(b)
	switch kind {
	case 1, 2:
		if kind == 1 {
			ret = g.diplomacyTest(ai, bi, 0x4b, 0)
		} else {
			ret = g.diplomacyTest(ai, bi, int16(A.Diplo6D7[b])+0x7d, 0)
			if A.Relations[b] < 50 {
				ret = 0
			}
		}
		switch ret {
		case 0:
			g.treatyRefusal(ai, bi, resp)
		case 1, 2:
			if ret == 2 {
				lvl = 8
			}
			chance := uint32(2)
			if kind == 2 {
				chance = 4
			}
			if r.Random(chance) == 1 && g.getDemands(a, b, lvl, false, true, true, false) {
				g.treatyBribe(a, b, resp, off)
				return ret
			}
			*resp = 0x8a
			return 0
		case 3:
			if kind == 1 {
				g.treatyAcceptance(ai, bi, true, 0x83, resp)
			} else {
				g.treatyAcceptance(ai, bi, false, 0x84, resp)
			}
		}
		return ret
	case 3:
		w := 100 - 25*int32(g.getWarCount(a, b))
		need := int32(g.getWarCount(b, a))*25 + w + (int32(g.fleetStrengthComparison(a, b))-100)/2
		need += g.numberOfAttacksUnderway(a, b) * 100
		need -= g.numberOfAttacksUnderway(b, a) * 50
		need -= int32(A.Diplo7EE[b]) + int32(A.Diplo6D7[b])
		need -= int32(int16(g.totalWarLossComparison(a, b)))
		need -= int32(A.TurnsAtWar[b]) - 9
		need += int32(A.Relations[b])
		ret = g.diplomacyTest(ai, bi, int16(need), 0)
		if A.TurnsAtWar[b] < 10 {
			ret = 0
		}
		switch ret {
		case 0:
			g.treatyRefusal(ai, bi, resp)
		case 1, 2:
			if ret == 2 {
				lvl = 8
			}
			if g.getDemands(a, b, lvl, true, true, true, true) {
				g.treatyBribe(a, b, resp, off)
				return ret
			}
			*resp = 0x8a
			return 0
		case 3:
			g.treatyAcceptance(ai, bi, false, 0x87, resp)
		}
		return ret
	case 7, 8:
		need, msg := int16(0x3c), int16(0x85)
		if kind == 8 {
			need, msg = 0x28, 0x86
		}
		ret = g.diplomacyTest(ai, bi, need, 0)
		switch ret {
		case 0:
			g.treatyRefusal(ai, bi, resp)
		case 1, 2:
			// A trade treaty is simply refused, the message untouched.
			if kind == 8 {
				*resp = 0x8a
			}
			return 0
		case 3:
			g.treatyAcceptance(ai, bi, true, msg, resp)
		}
		return ret
	}
	return 0
}

// npcMessageRejectionCheck is NPC_Message_Rejection_Check_ (0x2755f): an
// ambassador whose bias for p fell to -4 or below answers a refusal with
// 0xaf and sets the bias anew by personality.
func (g *Game) npcMessageRejectionCheck(amb, p int, resp *int16) {
	s := &g.state
	A := &s.Players[amb]
	v := A.HumanBias[p]
	if v > -4 {
		return
	}
	switch r := *resp; {
	case r >= 0x8a && r <= 0x98, r == 0xae:
	default:
		return
	}
	*resp = 0xaf
	rnd := &s.Random
	switch A.Personality {
	case 0:
		v = int16(rnd.Random(4)) - 6
	case 1, 2:
		v = -1 - int16(rnd.Random(3))
	case 3:
		v = -int16(rnd.Random(4))
	case 4:
		if A.Diplo727[p] == 1 {
			v = int16(rnd.Random(4)) - 6
		} else {
			v = -int16(rnd.Random(3))
		}
	case 5:
		v = -int16(rnd.Random(3))
	}
	A.HumanBias[p] = v
	s.Players[p].HumanBias[amb] = v
}

// fleetComparison is Fleet_Comparison_ (0x51dce): a's fleet against b's,
// from -300 to 300.
func (g *Game) fleetComparison(a, b int) int32 {
	s := &g.state
	r := (s.Players[a].FleetStrength[b] + 1) * 100 / (s.Players[b].FleetStrength[a] + 1)
	if r >= 100 {
		r++
	} else {
		r = -100 / (r + 1)
	}
	return min(max(r, -300), 300)
}

// getMaxTechnologyValue is Get_Max_Technology_Value_ (0xdcaee): the research
// cost of p's dearest researched field, over 25.
func (g *Game) getMaxTechnologyValue(p int) int16 {
	P := &g.state.Players[p]
	best := int32(0)
	for f := 0x52; f >= 0; f-- {
		if P.FieldStatus[f] == 3 {
			best = max(best, int32(g.exe.u32(uint32(fieldCost+f*0x17))))
		}
	}
	return int16(best / 25)
}

// techComparison is Tech_Comparison_ (0x51e3b): how far b's best field is
// above a's, in percent of a's, from -300 to 300.
func (g *Game) techComparison(a, b int) int32 {
	ma := int32(g.getMaxTechnologyValue(a))
	if ma == 0 {
		return 300
	}
	r := (int32(g.getMaxTechnologyValue(b)) - ma) * 100 / ma
	return min(max(r, -300), 300)
}

// populationComparison is Population_Comparison_ (0x51e98): a random share
// of how much more people a has than b, signed.
func (g *Game) populationComparison(a, b int) int32 {
	s := &g.state
	pa := int32(s.Players[a].Population)
	if pa == 0 {
		return 300
	}
	r := (pa - int32(s.Players[b].Population)) * 100 / pa
	r = min(max(r, -300), 300)
	switch {
	case r < 0:
		return -int32(s.Random.Random(uint32(-r)))
	case r > 0:
		return int32(s.Random.Random(uint32(r)))
	}
	return 0
}

// getSystemPopulation is Get_System_Population_ (0xe5b69): p's colonists at
// star st.
func (g *Game) getSystemPopulation(p, st int) int16 {
	s := &g.state
	var n int16
	for o := 4; o >= 0; o-- {
		pl := s.Stars[st].Planets[o]
		if pl == -1 {
			continue
		}
		c := s.Planets[pl].Colony
		if c == -1 || int(s.Colonies[c].Owner) != p {
			continue
		}
		n += int16(s.Colonies[c].PopCount)
	}
	return n
}

// playerHasShipsPesteringOtherPlayer is Player_Has_Ships_Pestering_Other_
// Player_ (0x1009c3): a has a ship orbiting a star of b where a owns no
// colony.
func (g *Game) playerHasShipsPesteringOtherPlayer(a, b int) bool {
	s := &g.state
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if int(sh.Owner) != a || sh.Status != 0 {
			continue
		}
		own := s.Stars[sh.Location].ColonyOwners
		if own>>uint(b)&1 != 0 && own>>uint(a)&1 == 0 {
			return true
		}
	}
	return false
}

// checkSurrenderSystem is Check_Surrender_System_ (0x1c417): p has a star
// it may surrender to q (it looks at up to three).
func (g *Game) checkSurrenderSystem(p, q int) bool {
	s := &g.state
	n := 0
	for st := 0; st < int(s.NumStars) && n < 3; st++ {
		if s.Stars[st].ColonyOwners>>uint(p)&1 != 0 && s.maySurrenderStar(st, p, q) {
			n++
		}
	}
	return n > 0
}

// getTechExchangeReaction is Get_Tech_Exchange_Reaction_ (0x26bbd): how
// willing b is to trade technology with a, in percent (always above 0).
func (g *Game) getTechExchangeReaction(a, b int, sure bool) int32 {
	s := &g.state
	A, B := &s.Players[a], &s.Players[b]
	rnd := int32(s.Random.Random(150))
	v := int32(A.Relations[b]) + int32(A.Diplo6D7[b]) + g.bestLeaderDiplomatBonus(int16(a))
	v -= int32(g.settings[0xd4]) * 50
	v += int32(A.HumanBias[b])*20 + 150
	if A.Picks[save.PickCharismatic] != 0 {
		v += 50
	}
	if A.AppStatus[0xc8] == techKnown {
		v += 50
	}
	v += int32(g.personalityRelationMod(b, B.Diplo727[a] == 1))
	if sure {
		v += 150
	} else {
		v += int32(A.Modifiers[2][b]) + rnd - 1
	}
	if v < 0 {
		return 10000 / (100 - v)
	}
	return v + 100
}

// getExchangeTechList is Get_Exchange_Tech_List_ (0x26d19): the human a's
// choice of b's technology (_target_tech_list) and what b wants for each
// (_exchange_tech_list: an application, 1 for nothing fitting, 0 refused).
func (g *Game) getExchangeTechList(a, b int) {
	s := &g.state
	d := &g.dip
	d.targetTech, d.exchangeTech = d.targetTech[:0], d.exchangeTech[:0]
	r := g.getTechExchangeReaction(a, b, false)
	if r <= 0 {
		return
	}
	type entry struct {
		v   int32
		app uint8
	}
	var la, lb []entry
	g.setCompetitionTechValues(b)
	for app := 0; app < 0xcc; app++ {
		if s.Players[a].AppStatus[app] != techKnown || !g.playerMayGetTechApp(b, app) {
			continue
		}
		if v := g.calcTechValue(b, app); v != 0 {
			la = append(la, entry{v, uint8(app)})
		}
	}
	best := int32(0)
	for i := range la {
		la[i].v = la[i].v * r / 100
		best = max(best, la[i].v)
	}
	limit := best * r / r
	g.setCompetitionTechValues(a)
	for app := 0; app < 0xd4; app++ {
		if s.Players[b].AppStatus[app] != techKnown || !g.playerMayGetTechApp(a, app) {
			continue
		}
		if v := g.calcTechValue(a, app); v != 0 && v <= limit {
			lb = append(lb, entry{v, uint8(app)})
		}
	}
	if len(la) == 0 || len(lb) == 0 {
		return
	}
	clib.Qsort(la, 5, func(x, y *entry) int { return int(x.v - y.v) })
	clib.Qsort(lb, 5, func(x, y *entry) int { return int(y.v - x.v) })
	mid := len(la) / 2
	midv := la[mid].v
	for _, e := range lb {
		d.targetTech = append(d.targetTech, e.app)
		if e.v > best {
			d.exchangeTech = append(d.exchangeTech, 0)
			continue
		}
		j := 0
		if e.v >= midv {
			j = mid
		}
		for j < len(la) && la[j].v < e.v {
			j++
		}
		if j < len(la) {
			d.exchangeTech = append(d.exchangeTech, la[j].app)
		} else {
			d.exchangeTech = append(d.exchangeTech, 1)
		}
	}
}

// getDeclareWarMessage is Get_Declare_War_Message_ (0x536bc).
func (g *Game) getDeclareWarMessage(p, amb int) int16 {
	s := &g.state
	A := &s.Players[amb]
	if A.Personality == 4 && s.Players[p].Diplo727[amb] == 0 && (A.TreatyStatus[p] == 1 || A.TreatyStatus[p] == 2) {
		return 0x42
	}
	return 0xa0
}

// breakChance is the common start of the Get_Break_*_Message_ routines:
// Random(200) less amb's fleet comparison and 50 a war, then Random(lim)
// against the personality's chance to strike back.
func (g *Game) breakChance(p, amb int, lim uint32) (int16, bool) {
	s := &g.state
	fc := g.fleetComparison(amb, p)
	v := int16(int32(s.Random.Random(200)) - fc)
	v -= g.getWarCount(int16(amb), int16(p)) * 50
	t := int32(g.exeTableI16(personalityBreakChance, int(s.Players[amb].Personality)))
	return v, int32(s.Random.Random(lim)) <= t
}

// getBreakTreatyMessage is Get_Break_Treaty_Message_ (0x533f4): breaking a
// pact or alliance may bring war; the message set with the war is
// overwritten, as in the original.
func (g *Game) getBreakTreatyMessage(p, amb int) int16 {
	s := &g.state
	if v, hit := g.breakChance(p, amb, 100); hit && v > 100 {
		g.declareWar(int16(p), int16(amb))
	}
	resp := int16(0x99)
	if s.Players[amb].Personality == 4 {
		resp++
	}
	s.Players[p].Diplo727[amb] = 1
	s.Players[amb].Diplo727[p] = 1
	return resp
}

// getBreakAgreementMessage is Get_Break_Research_Message_ (0x534c9) and
// Get_Break_Trade_Message_ (0x5356c): they may end amb's treaty too; the
// answer is always msg.
func (g *Game) getBreakAgreementMessage(p, amb int, msg int16) int16 {
	s := &g.state
	if v, hit := g.breakChance(p, amb, 100); hit && v > 100 {
		if st := s.Players[amb].TreatyStatus[p]; st == 1 || st == 2 {
			g.breakTreaties(int16(p), int16(amb))
		}
	}
	return msg
}

// getBreakTributeTreatyMessage is Get_Break_Tribute_Treaty_Message_ (0x5360f).
func (g *Game) getBreakTributeTreatyMessage(p, amb int) int16 {
	s := &g.state
	if v, hit := g.breakChance(p, amb, 150); hit && v > 75 {
		g.declareWar(int16(p), int16(amb))
	}
	resp := int16(0xab)
	if s.Players[amb].Personality == 4 {
		resp++
	}
	return resp
}

// getGiftResponse is Get_Gift_Response_ (0x53723): p's gift of kind (1
// money, 2 tribute, 3 tech, 4 star) worth value pleases amb, by its worth.
func (g *Game) getGiftResponse(p, amb int, kind uint16, value int16) int16 {
	s := &g.state
	P, A := &s.Players[p], &s.Players[amb]
	A.Diplo6E7[p] = uint8(kind)
	var w int32
	var resp int16
	switch kind {
	case 1:
		resp = 0xa3
		m := max(P.Maintenance, 10)
		w = int32(int16(int32(value) / m / 10))
		w = min(w, 8)
	case 2:
		w, resp = 2, 0xa2
	case 3:
		resp = 0xa1
		best := int32(1)
		for app := 0; app < 0xcc; app++ {
			if P.AppStatus[app] == techKnown {
				best = max(best, int32(g.appRank(app)))
			}
		}
		w = int32(g.appRank(int(value))) * 5 / best
	case 4:
		resp = 0xa4
		w = int32(g.getSystemPopulation(p, int(value)))/2 + 1
	}
	if w != 0 {
		P.LastProposalTurn[amb] /= int16(w)
		A.LastProposalTurn[p] /= int16(w)
	}
	v := w * (int32(s.Random.Random(4)) + 4)
	if int32(P.Relations[amb])+v > 100 {
		P.Relations[amb] = 100
	} else {
		P.Relations[amb] += int8(v)
	}
	if P.Relations[amb] > 100 {
		P.Relations[amb] = 100
	}
	if int32(P.BaseRelations[amb])+w > 100 {
		P.BaseRelations[amb] = 100
	} else {
		P.BaseRelations[amb] += int8(w)
	}
	if P.BaseRelations[amb] > 100 {
		P.BaseRelations[amb] = 100
	}
	A.BaseRelations[p] = P.BaseRelations[amb]
	A.Relations[p] = P.Relations[amb]
	return resp
}

// getDemandResponse is Get_Demand_Response_ (0x539d9): does amb give in to
// p's demand kind (0 war on, 1 peace with, 2 break the alliance with item,
// 3/4 tribute, 5 stop spying, 6 tech item, 7 star item, 8 fleet away)?
// Refusals may cost the treaty or mean war. With a score of exactly 0 and
// no treaty nothing is set: accepted keeps the caller's stale value.
func (g *Game) getDemandResponse(p, amb int, kind uint16, item int16, accepted, resp *int16) {
	s := &g.state
	P, A := &s.Players[p], &s.Players[amb]
	sum := int32(int16(g.fleetComparison(p, amb)))
	sum += g.populationComparison(p, amb)
	sum += int32(int16(g.techComparison(p, amb)))
	v := int16(sum/3) + int16(s.Random.Random(200))
	v += g.findWorstModifier(p, amb)
	v += 2 * g.personalityRelationMod(amb, P.Diplo727[amb] == 1)
	switch kind {
	case 0:
		x := int(item)
		switch A.TreatyStatus[x] {
		case 1:
			v -= 75
		case 2:
			v -= 150
		default:
			v -= 25
		}
		v -= int16(A.Relations[x] / 2)
		v -= int16(int32(int16(g.fleetComparison(amb, x))) / 4)
		if s.Players[x].Objective != objectiveHuman {
			v -= 25
		}
	case 1:
		x := int(item)
		v -= g.exeTableI16(personalityRelationMods, int(s.Players[x].Personality)) + 0x50
		if s.Players[x].Objective == objectiveHuman {
			v = 0
		}
	case 2:
		v -= 2 * int16(A.Relations[item])
	case 3, 5, 8:
		v -= 50
	case 4:
		v -= 150
	case 6:
		v -= int16(g.calcTechValue(p, int(item)) * 100 / int32(g.getMaxTechnologyValue(p)))
	case 7:
		q := int32(g.getSystemPopulation(amb, int(item))) * 1000 / int32(A.Population)
		if int16(q) > 300 {
			q = 300
		}
		v -= int16(q)
	}
	for range 5 {
		g.adjustDiplomatModifiers(int16(p), int16(amb))
	}
	war := func() {
		*resp = 0xa5
		g.declareWar(int16(p), int16(amb))
		*accepted = 0
	}
	cool := func() {
		P.DemandState[amb]++
		A.DemandState[p]++
		P.BaseRelations[amb] -= 10
		if P.BaseRelations[amb] < -100 {
			P.BaseRelations[amb] = -100
		}
		A.BaseRelations[p] = P.BaseRelations[amb]
		*resp = 0xa7
		*accepted = 0
	}
	if st := P.TreatyStatus[amb]; st == 1 || st == 2 {
		switch {
		case v < -100 || s.getPlayerDiplomacyPersonality(int16(amb), int16(p)) == 6:
			war()
		case v < -50:
			P.DemandState[amb]++
			A.DemandState[p]++
			*resp = 0xa6
			g.breakTreaties(int16(p), int16(amb))
			g.breakResearch(int16(p), int16(amb))
			g.breakTrade(int16(p), int16(amb))
			*accepted = 0
		case v < -25:
			cool()
		case v >= 0:
			*resp, *accepted = 0xa9, 1
		default:
			*resp, *accepted = 0xa8, 0
		}
	} else {
		switch {
		case v < -150 || s.getPlayerDiplomacyPersonality(int16(amb), int16(p)) == 6:
			war()
		case v < -75:
			cool()
		case v < 0:
			*resp, *accepted = 0xa8, 0
		case v == 0:
			g.logOnce("Get_Demand_Response_: a score of 0 leaves the answer to stale stack")
		default:
			*resp, *accepted = 0xa9, 1
		}
	}
	if *accepted == 1 && kind == 5 {
		r := uint8(s.Random.Random(50)) + 10
		A.Diplo806[item] = r + uint8(s.Random.Random(50))
	}
}
