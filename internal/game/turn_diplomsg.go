package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// The message each AI sends each human at the start of the next turn
// (Determine_Diplomacy_Messages_ and below, DIPLOMAC.C, NPCDIPLO.C).
// Codes are shown by the diplomacy screen. Spec: docs/turn/overview.md,
// phase C.

const persThreatDemandChance = 0x180d54 // _threat_demand_chance, i16

// limitTreatyModifiers is Limit_Treaty_Modifiers_ (0x50ad1).
func (g *Game) limitTreatyModifiers() {
	s := &g.state
	for i := int16(0); i < s.NumPlayers; i++ {
		for j := int16(0); j < s.NumPlayers; j++ {
			if i != j {
				m := &s.Players[i].Modifiers[0][j]
				*m = max(min(*m, 120), -200)
			}
		}
	}
}

// determineDiplomacyMessages is Determine_Diplomacy_Messages_ (0x4eb06):
// the one message each AI picks for each human it knows, from this turn's
// incidents first, else a peace offer, a call to arms or a proposal.
func (g *Game) determineDiplomacyMessages() {
	s := &g.state
	r := &s.Random
	g.limitTreatyModifiers()
	for i := int16(0); i < s.NumPlayers; i++ {
		for j := int16(0); j < s.NumPlayers; j++ {
			A, H := &s.Players[i], &s.Players[j]
			if A.Contact[j] != 1 || A.MessageFlags[j] != 0 || A.Objective == save.ObjectiveHuman ||
				H.Objective != save.ObjectiveHuman || H.Eliminated != 0 || A.Eliminated != 0 {
				continue
			}
			if A.TreatyStatus[j] == 6 && A.PrevTreatyStatus[j] == 6 {
				continue
			}
			if s.CouncilFlag == 3 {
				if c := A.MessageThisTurn[j]; c >= 1 && c <= 0xb {
					if int32(r.Random(100)) <= abs32(int32(A.MessageParam[j])/10) {
						A.MessageFlags[j] = 0x7b
					}
				}
			} else {
				g.reactToIncident(i, j)
				if A.MessageFlags[j] == 0 && int8(A.TreatyStatus[j]) >= 4 {
					g.peaceProposal(i, j)
				}
				if A.MessageFlags[j] == 0 {
					g.requiredAlliances(i, j)
				}
				if A.MessageFlags[j] == 0 {
					g.npcRequiredAlliances(i, j)
				}
				if A.MessageFlags[j] == 0 {
					g.unprovokedGoodMessages(i, j)
				}
			}
			// Repulsive races take no part in treaty and tribute talk.
			f := A.MessageFlags[j]
			if f >= 0x13 && f <= 0x38 || f >= 0x56 && f <= 0x6b || f >= 0x7c && f <= 0x7d {
				if A.Picks[save.PickRepulsive] == 1 || H.Picks[save.PickRepulsive] == 1 {
					A.MessageFlags[j], H.MessageFlags[i] = 0, 0
				}
			}
		}
	}
}

// reactToIncident is the switch of Determine_Diplomacy_Messages_ on the
// reason Change_Relations_ recorded this turn.
func (g *Game) reactToIncident(i, j int16) {
	s := &g.state
	A := &s.Players[i]
	c := A.MessageThisTurn[j]
	warOrBetrayal := func(code uint8) {
		if int8(A.TreatyStatus[j]) >= 4 {
			return
		}
		if s.getPlayerDiplomacyPersonality(i, j) == 4 && (A.TreatyStatus[j] == 1 || A.TreatyStatus[j] == 2 ||
			A.TradeTreaty[j] != 0 || A.ResearchTreaty[j] != 0) {
			A.Diplo727[j], s.Players[j].Diplo727[i] = 1, 1
			A.MessageFlags[j] = 0x42
		} else {
			A.MessageFlags[j] = code
		}
		g.declareWar(j, i)
	}
	switch {
	case c == 0:
	case c <= 9:
		g.determineBadMessage(i, j)
	case c == 0xa:
		warOrBetrayal(0x50)
	case c == 0xb:
		warOrBetrayal(0x51)
	case c <= 0x10:
		g.determineGoodMessage(i, j)
	case c == 0x11:
		A.MessageFlags[j] = A.Personality + 1
	case c == 0x12:
		if A.Relations[j] < 0 && int8(A.TreatyStatus[j]) < 4 {
			A.MessageFlags[j] = A.Personality + 7
		} else {
			A.MessageFlags[j] = A.Personality + 0xd
		}
	case c == 0x13:
		A.MessageFlags[j] = 0x46
	case c == 0x14:
		A.MessageFlags[j] = 0x6c
	case c == 0x15:
		A.MessageFlags[j] = 0x52
	case c == 0x16:
		A.MessageFlags[j] = 0x50
	case c == 0x17:
		A.MessageFlags[j] = 0x6f
	}
}

// determineBadMessage is Determine_Bad_Message_ (0x4f0dc): an incident
// raises the grudge by up to three rolls; a deep grudge means war or a
// sneak attack, a lesser one a warning, a demand or broken treaties.
func (g *Game) determineBadMessage(i, j int16) {
	s := &g.state
	r := &s.Random
	A, H := &s.Players[i], &s.Players[j]
	if A.MessageParam[j] == 0 {
		return
	}
	if int8(A.TreatyStatus[j]) >= 4 {
		g.peaceProposal(i, j)
		return
	}
	if (H.Picks[save.PickRepulsive] == 1 || A.Picks[save.PickRepulsive] == 1) && A.MessageParam[j] != -10000 {
		return
	}
	b := int16(abs32(2 * int32(A.MessageParam[j])))
	b -= g.exeTableI16(persRelationModifier, int(s.getPlayerDiplomacyPersonality(i, j)))
	b -= int16(A.Diplo6D7[j])
	b -= int16(A.Diplo7EE[j])
	if A.MessageThisTurn[j] == 9 && int16(int8(A.Diplo7C9[2])) == j {
		b = 100
		A.Diplo7C9[2] = 0xff
	}
	if A.Relations[j] < 0 {
		b -= int16(A.Relations[j])
	}
	for _, t := range [3]int16{b, b / 2, b / 4} {
		if int32(r.Random(100)) <= int32(t) {
			A.DemandState[j]++
		}
	}
	treaty := A.TreatyStatus[j] == 1 || A.TreatyStatus[j] == 2 || A.TradeTreaty[j] != 0 || A.ResearchTreaty[j] != 0
	c := A.MessageThisTurn[j]
	if c >= 1 && c <= 9 {
		A.Diplo6CF[j] = c
	}
	ds := int8(A.DemandState[j])
	if ds > 0 && s.getPlayerDiplomacyPersonality(i, j) == 4 && treaty {
		A.Diplo727[j], H.Diplo727[i] = 1, 1
		g.declareWar(j, i)
		A.MessageFlags[j] = 0x42
		return
	}
	switch {
	case ds >= 3 || ds >= 2 && !treaty:
		g.sneakAttackOrDeclareWar(i, j, c)
	case ds == 1:
		if treaty {
			A.MessageFlags[j] = c + 0x26
		} else {
			A.MessageFlags[j] = c + 0x12
		}
	case ds == 2 && treaty:
		chance := int32(g.exeTableI16(persThreatDemandChance, int(s.getPlayerDiplomacyPersonality(i, j))))
		if int32(r.Random(100)) <= chance && g.getDemands(i, j, 6, true, true, true, true) {
			A.MessageFlags[j] = c + 0x1b
			g.setDemands(i, j)
			return
		}
		g.breakTreaties(j, i)
		g.breakResearch(j, i)
		g.breakTrade(j, i)
		A.MessageFlags[j] = c + 0x2f
	}
}

// sneakAttackOrDeclareWar is Sneak_Attack_Or_Declare_War_ (0x4f59b): with a
// prepared target the AI attacks without warning, else it declares war if
// its fleet is more than two thirds of the human's.
func (g *Game) sneakAttackOrDeclareWar(i, j int16, code uint8) {
	s := &g.state
	A := &s.Players[i]
	if star := A.SneakStar[j]; star != -1 && A.SneakStrength[j] >= 200 && s.getPlayerDiplomacyPersonality(i, j) != 4 {
		A.AIWarTarget = star
		A.Diplo7C9[0] = code + 0x46
		return
	}
	ratio := (A.FleetStrength[j] + 1) * 100 / (s.Players[j].FleetStrength[i] + 1)
	if ratio > 66 {
		g.declareWar(j, i)
		A.MessageFlags[j] = code + 0x38
	}
}

// getWarCount is Get_War_Count_ (0x50028): a's wars with others than b,
// two for a human.
func (g *Game) getWarCount(a, b int16) int16 {
	s := &g.state
	var n int16
	for k := int16(0); k < s.NumPlayers; k++ {
		if k == a || k == b || int8(s.Players[a].TreatyStatus[k]) < 4 {
			continue
		}
		if s.Players[k].Objective == save.ObjectiveHuman {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// numberOfAttacksUnderway is Number_Of_Attacks_Underway_ (0xdbdb8): stars
// with a colony of b where a's combat ships are, arriving or leaving.
func (g *Game) numberOfAttacksUnderway(a, b int16) int32 {
	s := &g.state
	var seen [72]bool
	for k := int(s.NumShips) - 1; k >= 0; k-- {
		sh := &s.Ships[k]
		if int32(sh.Owner) != int32(a) || sh.Design.Type != 0 {
			continue
		}
		star := int32(-1)
		switch sh.Status {
		case 0:
			star = int32(sh.Location)
		case 1:
			star = int32(sh.Location) - 500
		case 2:
			star = int32(sh.Location) - 1000
		}
		if star != -1 && maskBit(s.Stars[star].ColonyOwners, int(uint8(b))) {
			seen[star] = true
		}
	}
	n := int32(0)
	for st := int(s.NumStars); st > 0; st-- {
		if seen[st-1] {
			n++
		}
	}
	return n
}

// totalWarLossComparison is Total_War_Loss_Comparison_ (0x51f0f): a's losses
// to b against b's losses to the AIs at war with b, -300..300.
func (g *Game) totalWarLossComparison(a, b int16) int32 {
	s := &g.state
	B := &s.Players[b]
	theirs := int32(0)
	for k := int16(0); k < s.NumPlayers; k++ {
		if s.Players[k].Objective != save.ObjectiveHuman && int8(B.TreatyStatus[k]) >= 4 {
			theirs += int32(B.WarLosses[k])
		}
	}
	theirs = max(theirs, 10)
	mine := max(int32(s.Players[a].WarLosses[b]), 10)
	var v int32
	if mine > theirs {
		v = 100 - mine*100/theirs
	} else {
		v = theirs*100/mine - 100
	}
	return max(min(v, 300), -300)
}

// peaceProposal is Peace_Proposal_ (0x4f694): after ten turns of war the AI
// may offer peace, asking for something when winning or offering something
// when losing. Its third argument is overwritten before use.
func (g *Game) peaceProposal(i, j int16) {
	s := &g.state
	r := &s.Random
	A := &s.Players[i]
	if A.TurnsAtWar[j] < 10 {
		return
	}
	turns := int16(A.TurnsAtWar[j]) - 9
	v := 5*int32(g.getWarCount(i, j)) - 5*int32(g.getWarCount(j, i))
	v += int32(g.fleetStrengthComparison(i, j)) / 10
	v += 40*g.numberOfAttacksUnderway(i, j) - 10
	v -= 20 * g.numberOfAttacksUnderway(j, i)
	v += int32(A.Diplo7EE[j])
	v += int32(A.Diplo6D7[j]) / 2
	v += int32(int16(g.totalWarLossComparison(i, j))) / 10
	v -= int32(g.exeTableI16(persRelationModifier, int(s.getPlayerDiplomacyPersonality(i, j)))) / 5
	for k := int16(0); k < s.NumPlayers; k++ {
		if k != i && k != j {
			v += 20 * g.numberOfAttacksUnderway(k, j)
		}
	}
	score := int16(v)
	var chance int16
	if score < 0 {
		chance = int16(uint16(-score) * uint16(turns))
	} else {
		chance = int16(int32(turns) / int32(isqrt(uint32(int32(score)+3))))
	}
	if int32(r.Random(100)) > int32(chance) {
		return
	}
	switch {
	case score < -10:
		lvl := int16((-int32(score)-10)/5 + 2)
		if g.getDemands(j, i, lvl, true, true, true, true) {
			A.MessageFlags[j] = 0x55
			g.setDemands(i, j)
		} else {
			A.MessageFlags[j] = 0x53
		}
	case score > 10:
		lvl := int16((int32(score)-10)/5 + 2)
		if g.getDemands(i, j, lvl, true, true, true, true) {
			A.MessageFlags[j] = 0x54
			g.setDemands(i, j)
		} else {
			A.MessageFlags[j] = 0x53
		}
	default:
		A.MessageFlags[j] = 0x53
	}
	A.TurnsAtWar[j] = 0
}

// determineGoodMessage is Determine_Good_Message_ (0x4fe25): sometimes a
// friendly act earns thanks, a gift or a request.
func (g *Game) determineGoodMessage(i, j int16) {
	s := &g.state
	r := &s.Random
	A := &s.Players[i]
	chance := int32(3)
	switch A.MessageThisTurn[j] {
	case 0xf:
		chance = 10
	case 0x10:
		chance = 20
	}
	if int32(r.Random(100)) > chance {
		return
	}
	A.DemandState[j] = 0
	ask := func(yes uint8) bool {
		if !g.getDemands(j, i, 4, false, true, true, false) {
			return false
		}
		A.MessageFlags[j] = yes
		return true
	}
	switch A.MessageThisTurn[j] {
	case 0xc:
		A.MessageFlags[j] = 0x59
	case 0xd:
		A.MessageFlags[j] = 0x5a
	case 0xe:
		A.MessageFlags[j] = 0x5b
	case 0xf:
		if int8(A.Diplo7E4[j]) != -1 {
			A.Diplo7E4[j] = 0xff
			A.MessageFlags[j] = 0x5d
			return
		}
		if int32(r.Random(100))+int32(A.Relations[j]) >= 75 {
			A.MessageFlags[j] = 0x5e
			return
		}
		if r.Random(3) == 1 && ask(0x5d) {
			A.Diplo7CC[j] = g.proposal.gold
			A.Diplo7DC[j] = g.proposal.tech1
			g.setDemands(i, j)
			return
		}
		A.MessageFlags[j] = 0x5c
	case 0x10:
		if r.Random(4) == 1 && ask(0x60) {
			g.setDemands(i, j)
			return
		}
		A.MessageFlags[j] = 0x5f
	}
}

// requiredAlliances is Required_Alliances_ (0x4fd30): now and then an AI asks
// its human ally to join a war against someone the human borders.
func (g *Game) requiredAlliances(a, b int16) {
	s := &g.state
	A, B := &s.Players[a], &s.Players[b]
	if A.TreatyStatus[b] != 2 || s.Random.Random(10) != 1 {
		return
	}
	target := int16(-1)
	for k := int16(0); k < s.NumPlayers; k++ {
		if k != a && k != b && int8(s.Players[k].TreatyStatus[a]) >= 4 && B.Contact[k] == 1 &&
			B.InNormalRange[k] == 1 && int8(B.TreatyStatus[k]) < 4 {
			target = k
		}
	}
	if target != -1 {
		A.MessageFlags[b] = 0x56
		A.AllianceCall[b] = uint8(target)
	}
}

// npcRequiredAlliances is NPC_Required_Alliances_ (0x2689d): an AI joins its
// human ally's war on someone it borders and tells the human.
func (g *Game) npcRequiredAlliances(a, b int16) {
	s := &g.state
	A := &s.Players[a]
	if A.TreatyStatus[b] != 2 || s.Random.Random(10) != 1 {
		return
	}
	target := int16(-1)
	for k := int16(0); k < s.NumPlayers; k++ {
		if k == a || k == b {
			continue
		}
		if t := s.Players[k].TreatyStatus[b]; (t == 4 || t == 5) && A.Contact[k] == 1 && A.InNormalRange[k] == 1 && int8(A.TreatyStatus[k]) < 4 {
			target = k
		}
	}
	if target != -1 {
		A.MessageFlags[b] = 0x46
		A.AllianceCall[b] = uint8(target)
		g.declareWar(a, target)
	}
}

// clearLastProposalTurn is Clear_Last_Proposal_Turn_ (0x27507).
func (g *Game) clearLastProposalTurn(a, b int16) {
	g.state.Players[a].LastProposalTurn[b] = 0
	g.state.Players[b].LastProposalTurn[a] = 0
}

// npcGoodProposalCheck is NPC_Good_Proposal_Check_ (0x2736e): whether the AI
// makes a proposal now, likelier the longer since the last one.
func (g *Game) npcGoodProposalCheck(i, j int16) bool {
	s := &g.state
	A, H := &s.Players[i], &s.Players[j]
	if int8(A.TreatyStatus[j]) >= 4 {
		return false
	}
	v := 10 - int32(A.Modifiers[0][j])/20 - int32(A.Diplo6D7[j])/20 + int32(A.Relations[j])/20
	v -= g.bestLeaderDiplomatBonus(j) / 15
	v += 3*int32(A.HumanBias[j]) + 4*int32(g.settings.Difficulty())
	v -= int32(g.exeTableI16(persRelationModifier, int(A.Personality))) / 5
	if H.TreatyStatus[i] == 1 {
		v -= 4
	}
	if H.TreatyStatus[i] == 2 {
		v -= 8
	}
	if H.AppStatus[appDiplomacyBonus] == 3 {
		v -= 5
	}
	v = min(max(v, 0), 40)
	t := int32(A.LastProposalTurn[j]) / (v*v/50 + 1)
	return int32(s.Random.Random(100)) <= t
}

// getTechExchangeExpectation is Get_Tech_Exchange_Expectation_ (0x26fba):
// how much a wants in return, in percent of what it gives b.
func (g *Game) getTechExchangeExpectation(a, b int16) int32 {
	s := &g.state
	A := &s.Players[a]
	r := int32(s.Random.Random(150))
	v := int32(A.Diplo6D7[b]) - 25*int32(g.settings.Difficulty()) + r
	if A.Picks[save.PickCharismatic] != 0 {
		v += 50
	}
	if A.AppStatus[appDiplomacyBonus] == 3 {
		v += 50
	}
	v += int32(g.exeTableI16(persRelationModifier, int(s.getPlayerDiplomacyPersonality(b, a))))
	if v >= 0 {
		return v + 100
	}
	return 10000 / (100 - v)
}

// npcTechExchangeCheck is NPC_Tech_Exchange_Check_ (0x2720f): the AI offers
// one of the human's applications it wants against one of its own worth
// about as much to the human.
func (g *Game) npcTechExchangeCheck(i, j int16) {
	s := &g.state
	A := &s.Players[i]
	if A.MessageFlags[j] != 0 || int8(A.TreatyStatus[j]) >= 4 {
		return
	}
	expect := max(g.getTechExchangeExpectation(j, i), 10)
	g.setCompetitionTechValues(int(i))
	type entry struct {
		v   uint32
		app uint8
	}
	var list []entry
	for app := 0; app < numTechApps; app++ {
		if s.Players[j].AppStatus[app] != techKnown {
			continue
		}
		if v := uint32(g.calcTechValue(int(i), app)) * 2; v != 0 {
			list = append(list, entry{v, uint8(app)})
		}
	}
	if len(list) == 0 {
		return
	}
	clib.Qsort(list, 5, func(x, y *entry) int { return int(int32(y.v - x.v)) })
	pick := list[int32(s.Random.Random(uint32((len(list)+1)/2)))-1]
	val := int32(pick.v) * expect / 100
	apps, _ := g.getDifferentialTechList(int(j), int(i), 0, uint32(val), false)
	if int16(len(apps)) <= 0 {
		return
	}
	A.MessageFlags[j] = 0x65
	A.TechExchangeValue[j] = val
	A.TechExchangeApp[j] = pick.app
}

// unprovokedGoodMessages is Unprovoked_Good_Messages_ (0x501ca): three rolls
// choose a proposal (tech exchange, pact, alliance, research or trade
// agreement); a failed trade offer turns into an offer of a joint war.
// Kind 6, an alliance-free gift, is chosen but never acted on.
func (g *Game) unprovokedGoodMessages(i, j int16) {
	s := &g.state
	r := &s.Random
	A := &s.Players[i]
	if int8(A.TreatyStatus[j]) >= 4 || !g.npcGoodProposalCheck(i, j) {
		return
	}
	kind := 0
	for n := 0; n < 3; n++ {
		switch x := r.Random(20); {
		case x >= 1 && x <= 5:
			kind = 5
		case x >= 6 && x <= 9:
			if A.TreatyStatus[j] == 0 {
				kind = 2
			}
		case x >= 10 && x <= 12:
			if A.Relations[j] >= 0 && A.TradeTreaty[j] == 0 {
				kind = 1
			}
		case x >= 13 && x <= 15:
			if A.Relations[j] >= 0 && A.ResearchTreaty[j] == 0 {
				kind = 4
			}
		case x >= 16 && x <= 18:
			if A.TreatyStatus[j] == 1 {
				kind = 3
			}
		case x >= 19 && x <= 20:
			if A.TreatyStatus[j] != 2 && r.Random(3) == 1 && A.Relations[j] >= 0 {
				kind = 6
			}
		}
	}
	diff := int32(g.settings.Difficulty())
	pm2 := 2 * int32(g.exeTableI16(persRelationModifier, int(A.Personality)))
	treatyBonus := func(v int16) int16 {
		if A.TreatyStatus[j] == 1 {
			v -= 25
		}
		if A.TreatyStatus[j] == 2 {
			v -= 50
		}
		return v
	}
	switch kind {
	case 5:
		g.npcTechExchangeCheck(i, j)
	case 2:
		if f := int32(g.fleetStrengthComparison(i, j)); int32(r.Random(uint32(500-50*diff+pm2))) >= f {
			A.MessageFlags[j] = 0x61
		}
	case 3:
		if f := int32(g.fleetStrengthComparison(i, j)); int32(r.Random(uint32(350-50*diff+pm2))) >= f {
			A.MessageFlags[j] = 0x62
		}
	case 4:
		t := uint32(int32(A.FieldsResearched)*100)/uint32(int32(s.Players[j].FieldsResearched)+1) + 1
		v := treatyBonus(int16(int32(t) + 50*diff - pm2))
		if int32(v) <= int32(r.Random(300)) {
			A.MessageFlags[j] = 0x64
		}
	case 1:
		t := uint32(A.Maintenance*100)/(uint32(s.Players[j].Maintenance)+1) + 1
		v := treatyBonus(int16(int32(t) + 50*diff - pm2))
		if int32(v) <= int32(r.Random(300)) {
			A.MessageFlags[j] = 0x63
			break
		}
		if g.offerJointWar(i, j) {
			return
		}
	}
	g.clearLastProposalTurn(i, j)
}

// offerJointWar is the joint-war offer at the end of Unprovoked_Good_Messages_
// (0x50637): against a random enemy the AI is losing to, after 20 turns.
// The level divides by the losses of player NUM_PLAYERS, not the target's.
func (g *Game) offerJointWar(i, j int16) bool {
	s := &g.state
	A, H := &s.Players[i], &s.Players[j]
	var list []int16
	for k := int16(0); k < s.NumPlayers; k++ {
		if k == i || k == j || A.Contact[k] != 1 || H.Contact[k] != 1 || int8(H.TreatyStatus[k]) >= 4 || A.Relations[k] > -50 {
			continue
		}
		if A.WarLosses[k] > s.Players[k].WarLosses[i] && A.Personality != 0 && A.TurnsAtWar[k] >= 20 && A.WarLosses[k] > 0 {
			list = append(list, k)
		}
	}
	if len(list) == 0 {
		return false
	}
	target := list[s.Random.Random(uint32(len(list)))-1]
	n := s.NumPlayers
	theirs := int32(0) // player record NUM_PLAYERS; past the eighth it is not ours to read
	if int(n) < len(s.Players) {
		theirs = int32(s.Players[n].WarLosses[i])
	}
	mine := int32(0)
	if n < 8 {
		mine = int32(A.WarLosses[n])
	} else {
		mine = int32(A.WarLosses2[n-8])
	}
	lvl := int16(mine / (theirs + 1))
	lvl = min(lvl, 6)
	if !g.getDemands(j, i, lvl, false, true, true, false) {
		return false
	}
	A.MessageFlags[j] = 0x7d
	A.Diplo7CC[j] = g.proposal.gold
	A.Diplo7DC[j] = g.proposal.tech1
	A.Diplo7E4[j] = uint8(target)
	g.clearLastProposalTurn(i, j)
	g.setDemands(i, j)
	return true
}
