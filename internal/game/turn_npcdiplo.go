package game

import (
	"bytes"
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// The AI's own diplomacy (NPC_Diplomacy_ and below, DIPLOMAC.C and
// NPCDIPLO.C). Spec: docs/turn/overview.md, phase C.

const (
	persMaxThreats          = 0x180cf0 // _personality_max_threats, i16
	persBreakPeaceChance    = 0x181070 // _personality_break_peace_treaty_chance, i16
	persSneakAttackModifier = 0x181080 // _personality_sneak_attack_modifiers, i16
	appDiplomacyBonus       = 200      // the application tested at player +0x1df
	historyPopulation       = 0xb9b    // player offset of History[2]
	playerRecordSize        = 0xea9
)

func (g *Game) exeTableI16(table uint32, i int) int16 { return g.exeI16(table + uint32(2*i)) }

// playerByte reads byte off of player p's record the way the original
// does, running on into the next records for offsets past the end.
func (g *Game) playerByte(p, off int) uint8 {
	p += off / playerRecordSize
	off %= playerRecordSize
	if p < 0 || p >= len(g.state.Players) {
		return 0
	}
	if off >= historyPopulation && off < historyPopulation+350 {
		return g.state.Players[p].History[2][off-historyPopulation]
	}
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, &g.state.Players[p])
	return b.Bytes()[off]
}

// npcDiplomacy is NPC_Diplomacy_ (0x252a7), which runs on into
// NPC_Tech_Exchange_.
func (g *Game) npcDiplomacy() {
	g.npcSneakAttacks()
	g.npcDiplomacyBuildupPenalties()
	g.npcThreats()
	g.npcToNPCTreatyNegotiations()
	g.npcDeclarationsOfWar()
	g.npcPeaceNegotiations()
	g.npcTechExchange()
}

// npcTechExchange is NPC_Tech_Exchange_ (0x252c5, its loop at 0x26991 under
// the NPC_To_Human_Diplomacy_ symbol): on its turn of the rotation an AI
// may swap its most expensive application for the other AI's.
func (g *Game) npcTechExchange() {
	s := &g.state
	r := &s.Random
	for i := int16(0); i < s.NumPlayers; i++ {
		A := &s.Players[i]
		if A.Objective == save.ObjectiveHuman || A.Picks[save.PickRepulsive] != 0 || int32(s.Stardate)%int32(s.NumPlayers) != int32(i) {
			continue
		}
		for j := int16(0); j < s.NumPlayers; j++ {
			B := &s.Players[j]
			if j == i || B.Objective == save.ObjectiveHuman || B.Picks[save.PickRepulsive] != 0 || A.Contact[j] == 0 {
				continue
			}
			if int32(r.Random(100)) >= int32(A.Relations[j])+50 {
				continue
			}
			if int32(r.Random(4)) > int32(g.settings.Difficulty()) {
				continue
			}
			var bestA, bestB uint32
			appA, appB := int16(-1), int16(-1)
			cost := func(app int) uint32 {
				v := g.exe.u32(uint32(techFieldTable + g.techAppFieldOf(app)*techFieldRow + 0x12))
				if g.appKind(app) == 0x27 {
					v *= 1000
				}
				return v
			}
			for app := 1; app < numTechApps; app++ {
				if A.AppStatus[app] == techKnown && B.AppStatus[app] != techKnown && g.playerMayGetTechApp(int(j), app) {
					if v := cost(app); v > bestA {
						bestA, appA = v, int16(app)
					}
				}
				if B.AppStatus[app] == techKnown && A.AppStatus[app] != techKnown && g.playerMayGetTechApp(int(i), app) {
					if v := cost(app); v > bestB {
						bestB, appB = v, int16(app)
					}
				}
			}
			if appA == -1 || appB == -1 {
				continue
			}
			g.playerGetsTechApp(int(i), int(appB))
			g.playerGetsTechApp(int(j), int(appA))
		}
	}
}

// npcSneakAttacks is NPC_Sneak_Attacks_ (0x53edb): an AI at peace with
// every human weighs each one it borders and may plan a sneak attack,
// threaten, demand tribute or ask for help.
func (g *Game) npcSneakAttacks() {
	s := &g.state
	r := &s.Random
	n := s.NumPlayers
	for i := int16(0); i < n; i++ {
		A := &s.Players[i]
		if A.Objective == save.ObjectiveHuman || A.Eliminated != 0 {
			continue
		}
		var humanWars, aiWars int16
		for k := int16(0); k < n; k++ {
			if int8(A.TreatyStatus[k]) < 4 {
				continue
			}
			if s.Players[k].Objective == save.ObjectiveHuman {
				humanWars++
			} else {
				aiWars++
			}
		}
		if A.AIWarTarget != -1 || humanWars > 0 {
			if A.Diplo816 == 0 {
				A.AIWarTarget = -1
			}
			continue
		}
		for k := int16(0); k < n; k++ {
			if A.SneakStar[k] == -1 {
				A.SneakValue[k] = 0
				A.SneakStrength[k] = 0
			}
		}
		var res, need, reason [8]int16
		for k := int16(0); k < n; k++ {
			H := &s.Players[k]
			if H.Objective == save.ObjectiveHuman && int8(A.TreatyStatus[k]) < 4 && A.InNormalRange[k] == 1 &&
				A.Contact[k] == 1 && H.Eliminated == 0 && A.PrevTreatyStatus[k] != 1 &&
				A.PrevTreatyStatus[k] != 2 && A.PrevTreatyStatus[k] != 3 {
				res[k] = g.sneakAttackEvaluations(i, k, &need[k], &reason[k])
			}
		}
		peaceWithAIs := func() {
			for m := int16(0); m < n; m++ {
				if s.Players[m].Objective != save.ObjectiveHuman && int8(A.TreatyStatus[m]) >= 4 {
					g.declarePeace(i, m)
				}
			}
		}
		for k := int16(0); k < n; k++ {
			if A.TreatyTimer[k] != 0 {
				chance := int32(g.exeTableI16(persBreakPeaceChance, int(s.getPlayerDiplomacyPersonality(i, k))))
				if int32(r.Random(100)) > chance && res[k] != 0 {
					A.Diplo816 = uint8(r.Random(20)) + 20
					continue
				}
			}
			threaten := func(flags uint8) {
				A.ThreatTurn[k], s.Players[k].ThreatTurn[i] = 0, 0
				A.MessageParam[k] = 10000
				A.MessageThisTurn[k] = 0
				A.MessageFlags[k] = flags
			}
			switch {
			case res[k] == 2 && s.getPlayerDiplomacyPersonality(i, k) != 4:
				A.ThreatTurn[k], s.Players[k].ThreatTurn[i] = 0, 0
				if A.SneakStar[k] != -1 && A.SneakStrength[k] > int32(need[k]) {
					if A.SneakValue[k] <= 0 {
						continue
					}
					A.AIWarTarget = A.SneakStar[k]
					A.Diplo7C9[0] = uint8(reason[k])
					A.Diplo816 = uint8(r.Random(20)) + 20
					if aiWars > 0 {
						peaceWithAIs()
					}
					continue
				}
				A.Diplo816 = uint8(r.Random(20)) + 20
				if aiWars > 0 && int32(r.Random(4)) < int32(g.settings.Difficulty()) {
					peaceWithAIs()
				}
			case res[k] == 1:
				A.ThreatTurn[k], s.Players[k].ThreatTurn[i] = 0, 0
				A.Diplo887[k] = uint8(reason[k])
				A.Diplo816 = uint8(r.Random(20)) + 20
				A.MessageParam[k] = 10000
				A.MessageThisTurn[k] = 0
				A.MessageFlags[k] = 0x6a
				g.setDemands(i, k)
			case res[k] == 3:
				threaten(0x69)
				A.Diplo816 = uint8(r.Random(20)) + 10
				g.setDemands(i, k)
			case res[k] == 4:
				A.Diplo7C9[2] = uint8(k)
				threaten(0x7c)
				A.Diplo816 = uint8(r.Random(20)) + 20
				g.setDemands(i, k)
			}
		}
	}
}

// sneakAttackEvaluations is Sneak_Attack_Evaluations_ (0x544a1): whether AI
// i turns on human j: 0 nothing, 1 threat with a demand, 2 sneak attack,
// 3 plain threat, 4 call for help. need gets the strength ratio a sneak
// attack wants, reason the worst grievance (message code).
func (g *Game) sneakAttackEvaluations(i, j int16, need, reason *int16) int16 {
	s := &g.state
	r := &s.Random
	A, H := &s.Players[i], &s.Players[j]
	// score sums the grievances; worst and why track the largest one.
	var score, worst int16
	why := int16(0x43)
	threshold := int16(200)
	rel := int32(A.Relations[j])
	if rel >= 50 {
		worst = int16(rel / 5)
	} else {
		worst = int16(rel / 10)
	}
	score = worst
	grudge := func(t int16, code int16) {
		if t < worst {
			worst, why = t, code
		}
		score += t
	}
	if int8(A.DemandState[j]) > 0 && A.Diplo6CF[j] != 0 {
		t := int16(int32(int8(A.DemandState[j])) * -10 / int32(g.exeTableI16(persMaxThreats, int(s.getPlayerDiplomacyPersonality(i, j)))))
		if t < score {
			worst, why = t, int16(A.Diplo6CF[j])+0x46
		}
		score += t
	}
	hate := func(code int16) { worst, score, why = -150, -150, code }
	if A.Unknown60E == 1 && (A.Objective == 2 || int32(A.Population) > int32(int16(g.getPlayerMaxPopulation(j)))/2) {
		hate(0x72)
	}
	if A.Personality == 3 && int32(r.Random(200)) <= int32(g.settings.Difficulty())+1 {
		hate(0x6d)
		threshold = 100
	}
	if int32(r.Random(100)) < int32(A.Diplo7EC) {
		hate(0x77)
	}
	if A.Personality == 1 && g.fleetStrengthComparison(i, j) >= 100 && A.SneakStrength[j] >= 200 && A.SneakStar[j] != -1 {
		grudge(int16(-A.SneakStrength[j]/20), 0x73)
	}
	if A.Personality == 0 && int32(r.Random(400)) <= int32(int16(g.systemProximityCount(i, j))) {
		hate(0x79)
	}
	t := int16(3 * int32(A.Diplo7EE[j]) / 5)
	if t < worst && int8(A.Diplo7F6[j]) != -1 {
		worst, why = t, 0xb0
		if int16(int8(A.Diplo7F6[j])) == i {
			why = 0xb1
		}
	}
	score += t
	if g.fleetStrengthComparison(i, j) >= 300 || 2*int32(A.Population) >= 3*int32(A.Population) {
		if A.Personality != 5 {
			grudge(int16(-int32(g.fleetStrengthComparison(i, j))/40), 0x70)
			threshold = 150
		}
	}
	var alive, maxPop int16
	for k := int16(0); k < s.NumPlayers; k++ {
		K := &s.Players[k]
		if K.Objective == save.ObjectiveHuman || K.Eliminated != 0 {
			continue
		}
		alive++
		if k != i && maxPop < K.Population {
			maxPop = K.Population
		}
	}
	if (alive < 3 && A.Population > maxPop) || alive == 1 {
		grudge(-10, 0xb2)
		threshold = 150
	}
	if now := int16(uint16(s.Stardate) - 35001); now >= 100 {
		back := now - 40
		if now >= 350 {
			back = 0x135
		}
		hist := func(p int16, at int16) int16 {
			return int16(g.playerByte(int(p), historyPopulation+int(at)))
		}
		mine := hist(i, now) - hist(i, back)
		theirs := hist(j, now) - hist(j, back)
		if theirs > mine {
			grudge(int16(-(int32(theirs)-int32(mine))/2), 0x75)
		}
	}
	switch A.TreatyStatus[j] {
	case 1:
		score += 10
	case 2:
		score += 20
	}
	score += g.exeTableI16(persSneakAttackModifier, int(s.getPlayerDiplomacyPersonality(i, j)))
	if H.AppStatus[appDiplomacyBonus] == 3 {
		score += 5
	}
	if H.Picks[save.PickCharismatic] == 1 {
		score += 10
	}
	score += int16(g.bestLeaderDiplomatBonus(j))
	score += 15 - int16(g.settings.Difficulty())
	var chance int16
	if score >= 0 {
		c := int32(score)
		chance = int16(int32(A.ThreatTurn[j]) / int32(isqrt(uint32(c*c*c+5))))
	} else {
		chance = int16(uint16(A.ThreatTurn[j]) * uint16(-score))
	}
	if (A.TreatyStatus[j] == 1 || A.TreatyStatus[j] == 2) && s.getPlayerDiplomacyPersonality(i, j) == 4 {
		chance = 0
	}
	lvl := int16(int32(g.fleetStrengthComparison(i, j))/40 + int32(r.Random(3)) - 1)
	lvl = max(lvl, 1)
	if !g.getDemands(i, j, lvl, true, true, true, true) {
		lvl = 0
	}
	if int32(r.Random(100)) > int32(chance) || A.ThreatTurn[j] < 10 {
		return 0
	}
	out := func(v int16) int16 {
		*need, *reason = threshold, why
		return v
	}
	if int32(r.Random(16))+int32(g.settings.Difficulty()) >= 16 || lvl <= 0 || worst == -150 {
		return out(2)
	}
	top := true
	for k := int16(0); k < s.NumPlayers; k++ {
		if A.Population < s.Players[k].Population && k != i {
			top = false
		}
	}
	if r.Random(4) <= 3 && top && s.CouncilFlag == 1 && lvl > 3 {
		return out(4)
	}
	if A.Picks[save.PickRepulsive] != 0 || H.Picks[save.PickRepulsive] != 0 {
		return 0
	}
	if r.Random(4) > isqrt(uint32(int32(lvl)+4)) {
		return out(3)
	}
	return out(1)
}

// persRelationModifier is _personality_relation_modifiers (0x180ccc), i16.
const persRelationModifier = 0x180ccc

// npcDiplomacyBuildupPenalties is NPC_Diplomacy_Buildup_Penalties_
// (0x25c7c): on even turns an AI with many large colonies worries the
// other AIs it has no alliance or timed peace with.
func (g *Game) npcDiplomacyBuildupPenalties() {
	s := &g.state
	r := &s.Random
	if (int32(s.Stardate)-35000)%2 != 0 {
		return
	}
	size := int32(g.settings.GalaxySize())
	diff := int32(g.settings.Difficulty())
	for i := int16(0); i < s.NumPlayers; i++ {
		for j := int16(0); j < s.NumPlayers; j++ {
			A, B := &s.Players[i], &s.Players[j]
			if i == j || A.Objective == save.ObjectiveHuman || B.Objective == save.ObjectiveHuman || B.ColoniesPop5 <= 0 {
				continue
			}
			if B.TreatyStatus[i] == 2 || B.TreatyStatus[i] == 3 || int32(A.ColoniesPop5) <= 3*(size+1) {
				continue
			}
			if r.Random(4) != 1 {
				continue
			}
			d := -(int32(A.ColoniesPop5) - 3*(size+1)) / (size + 6 - diff)
			v := int32(r.Random(4)) * d / 3 << 2
			if A.Objective != save.ObjectiveHuman {
				v = int32(int16(v)) / 2
			}
			g.changeRelations(int16(v), int(i), int(j), 8, 0, 0)
		}
	}
}

// npcThreats is NPC_Threats_ (0x252d5): an AI's grudge against another AI
// grows with this turn's incident or fades with a friendly one.
func (g *Game) npcThreats() {
	s := &g.state
	r := &s.Random
	for i := int16(0); i < s.NumPlayers; i++ {
		for j := int16(0); j < s.NumPlayers; j++ {
			A, B := &s.Players[i], &s.Players[j]
			if A.Objective == save.ObjectiveHuman || B.Objective == save.ObjectiveHuman {
				continue
			}
			mp := A.MessageParam[j]
			if mp == 0 || A.MessageThisTurn[j] == 0 || mp == 10000 {
				continue
			}
			step := uint8(1)
			if t := int8(A.TreatyStatus[j]); t >= 1 && t <= 3 || A.TradeTreaty[j] != 0 || A.ResearchTreaty[j] != 0 || A.Tribute[j] < 0 {
				step = 2
			}
			if mp > 0 {
				if int32(r.Random(100)) < int32(int16(2*mp)) {
					A.DemandState[j]--
					if int8(A.DemandState[j]) < 0 {
						A.DemandState[j] = 0
					}
					B.DemandState[i] = A.DemandState[j]
				} else {
					A.MessageThisTurn[j] = 0
				}
				continue
			}
			pers := int(A.Personality)
			if A.Personality == 4 && A.Diplo727[j] == 1 {
				pers = 6
			}
			t := int16(-2*int32(mp)) + g.exeTableI16(0x180cdc, pers)
			if int32(r.Random(100)) < int32(t) || step == 2 {
				A.DemandState[j] += step
				B.DemandState[i] = A.DemandState[j]
			} else {
				A.MessageThisTurn[j] = 0
			}
		}
	}
}

// startTreaty is Start_Treaty_ (0x5232e): kind 1 non-aggression pact or 2
// alliance, with treaty timers from both personalities.
func (g *Game) startTreaty(a, b int16, kind int16) {
	s := &g.state
	A, B := &s.Players[a], &s.Players[b]
	A.TreatyStatus[b], B.TreatyStatus[a] = uint8(kind), uint8(kind)
	A.DemandState[b], B.DemandState[a] = 0, 0
	pa, pb := s.getPlayerDiplomacyPersonality(a, b), s.getPlayerDiplomacyPersonality(b, a)
	switch kind {
	case 1:
		da := int32(g.exeTableI16(personalityPeaceDuration, int(pa)))
		A.TreatyTimer[b] = uint8(int32(int8(A.TreatyTimer[b])) + da + da/2)
		db := int32(g.exeTableI16(personalityPeaceDuration, int(pb)))
		B.TreatyTimer[a] = uint8(int32(int8(B.TreatyTimer[a])) + db + db/2)
	case 2:
		A.TreatyTimer[b] += 2 * g.peaceDuration(pa)
		B.TreatyTimer[a] += 2 * g.peaceDuration(pb)
	}
}

// startTributeTreaty is Start_Tribute_Treaty_ (0x52049): a pays b amount.
func (g *Game) startTributeTreaty(a, b, amount int16) {
	s := &g.state
	A, B := &s.Players[a], &s.Players[b]
	A.Tribute[b] = amount
	A.DemandState[b], B.DemandState[a] = 0, 0
	A.TreatyTimer[b] += 2 * g.peaceDuration(s.getPlayerDiplomacyPersonality(a, b))
	B.TreatyTimer[a] += 2 * g.peaceDuration(s.getPlayerDiplomacyPersonality(b, a))
}

// adjustDiplomatModifiers is Adjust_Diplomat_Modifiers_ (0x524c3).
func (g *Game) adjustDiplomatModifiers(a, b int16) {
	P := &g.state.Players[a]
	for _, k := range [4]int{1, 0, 2, 3} {
		P.Modifiers[k][b] -= 10
	}
}

// honorAlliances is Honor_Alliances_ (0x25ad2): a joins its ally b's wars
// on players a borders and is at peace with.
func (g *Game) honorAlliances(a, b int16) {
	s := &g.state
	A, B := &s.Players[a], &s.Players[b]
	if A.TreatyStatus[b] != 2 {
		return
	}
	for k := int16(0); k < s.NumPlayers; k++ {
		if k == a || int8(B.TreatyStatus[k]) < 4 || int8(A.TreatyStatus[k]) >= 4 || A.Contact[k] != 1 || A.InNormalRange[k] != 1 {
			continue
		}
		if s.Players[k].Objective == save.ObjectiveHuman {
			g.changeRelations(-10000, int(k), int(a), 0x13, 0, 0)
		}
		A.AllianceCall[b] = uint8(k)
		g.declareWar(a, k)
	}
}

// eliminateNPCTreatyConflicts is Eliminate_NPC_Treaty_Conflicts_ (0x25bc6):
// an AI allied to two AIs at war with each other drops both alliances. The
// third player is always i+1, the loop variable being reused.
func (g *Game) eliminateNPCTreatyConflicts() {
	s := &g.state
	for i := int16(0); i < s.NumPlayers; i++ {
		k := i + 1
		for j := i + 1; j < s.NumPlayers; j++ {
			A := &s.Players[i]
			if A.Objective == save.ObjectiveHuman || s.Players[j].Objective == save.ObjectiveHuman {
				continue
			}
			if int8(s.Players[j].TreatyStatus[k]) >= 4 && A.TreatyStatus[j] == 2 && A.TreatyStatus[k] == 2 {
				g.breakTreaties(i, j)
				g.breakTreaties(i, k)
			}
		}
	}
}

// npcToNPCTreatyNegotiations is NPC_To_NPC_Treaty_Negotiations_ (0x2552d):
// now and then two AIs in contact sign pacts, agreements or tribute.
func (g *Game) npcToNPCTreatyNegotiations() {
	s := &g.state
	r := &s.Random
	n := s.NumPlayers
	pm := func(p int16) int32 { return int32(g.exeTableI16(persRelationModifier, int(s.Players[p].Personality))) }
	for i := int16(0); i < n; i++ {
		for j := int16(0); j < n; j++ {
			A, B := &s.Players[i], &s.Players[j]
			if A.Objective == save.ObjectiveHuman || B.Objective == save.ObjectiveHuman ||
				A.Picks[save.PickRepulsive] != 0 || B.Picks[save.PickRepulsive] != 0 || i == j {
				continue
			}
			if r.Random(uint32(250-40*int32(g.settings.Difficulty()))) != 1 || A.Contact[j] != 1 {
				continue
			}
			if int8(A.TreatyStatus[j]) < 4 && A.Modifiers[0][j] >= -30 {
				g.npcNegotiate(i, j, pm)
			}
			g.honorAlliances(i, j)
			for k := 0; k < 3; k++ {
				g.adjustDiplomatModifiers(i, j)
			}
		}
	}
	g.eliminateNPCTreatyConflicts()
}

// npcNegotiate is the treaty part of NPC_To_NPC_Treaty_Negotiations_ for
// one pair.
func (g *Game) npcNegotiate(i, j int16, pm func(int16) int32) {
	s := &g.state
	r := &s.Random
	A := &s.Players[i]
	base := int32(A.Diplo6D7[j]) + int32(A.Relations[j])
	if A.TreatyStatus[j] == 1 {
		base += 20
	}
	if A.TreatyStatus[j] == 2 {
		base += 40
	}
	// The original tests the trade treaty twice, never the research one.
	if A.TradeTreaty[j] != 0 {
		base += 20
	}
	for k := int16(0); k < s.NumPlayers; k++ {
		if k == i || k == j {
			continue
		}
		if int8(A.TreatyStatus[k]) >= 4 {
			base += 10
		}
		if int8(s.Players[j].TreatyStatus[k]) >= 4 {
			base += 20
		}
		if int8(s.Players[j].DemandState[k]) > 0 {
			base += 5
		}
	}
	var humanWars, aiWars int32
	for k := int16(0); k < s.NumPlayers; k++ {
		if k == i || k == j {
			continue
		}
		for _, p := range [2]int16{j, i} {
			if int8(s.Players[p].TreatyStatus[k]) < 4 {
				continue
			}
			if s.Players[k].Objective == save.ObjectiveHuman {
				humanWars++
			} else {
				aiWars++
			}
		}
	}
	mod0 := int32(A.Modifiers[0][j])
	v := mod0 + int32(r.Random(100)) + base + 50*humanWars + pm(j) + pm(i)
	switch st := A.TreatyStatus[j]; {
	case v >= 200 && st != 2 && st == 1 && aiWars == 0:
		g.startTreaty(i, j, 2)
	case v >= 100 && st != 1 && st != 2:
		g.startTreaty(i, j, 1)
	}
	mod1 := int32(A.Modifiers[1][j])
	v = int32(r.Random(100)) + base + mod1 + pm(j)
	switch {
	case v > 110 && A.ResearchTreaty[j] == 0:
		A.ResearchTreaty[j], s.Players[j].ResearchTreaty[i] = 1, 1
	case v > 80 && A.TradeTreaty[j] == 0:
		A.TradeTreaty[j], s.Players[j].TradeTreaty[i] = 1, 1
	}
	mod0 = int32(A.Modifiers[0][j])
	v = mod0 + int32(r.Random(100)) + base + pm(j) + pm(i)
	// The loop meant to look for tribute paid to anyone tests j each time.
	paying := A.Tribute[j] < 0 && s.NumPlayers > 0
	if v <= 150 || paying || g.fleetStrengthComparison(i, j) >= 100 || A.GrossIncome >= s.Players[j].GrossIncome {
		return
	}
	if int32(r.Random(20)) > int32(g.settings.Difficulty())+1 {
		return
	}
	r.Random(4) // drawn and discarded
	g.startTributeTreaty(i, j, 2)
	g.changeRelations(int16(r.Random(3)+3), int(i), int(j), 0xe, 0, 0)
}

// npcDeclarationsOfWar is NPC_Declarations_Of_War_ (0x25df1): each AI picks
// war candidates by personality, bad relations, a bankrupt treasury,
// losses and opportunity, then declares on the second kind at once and
// on one random candidate of the first kind when at peace.
func (g *Game) npcDeclarationsOfWar() {
	s := &g.state
	r := &s.Random
	n := s.NumPlayers
	diff := int32(g.settings.Difficulty())
	for i := int16(0); i < n; i++ {
		A := &s.Players[i]
		if A.Objective == save.ObjectiveHuman || A.Eliminated != 0 {
			continue
		}
		if g.hyperspaceFlux() && !(g.hyperspaceFlux() && A.Picks[save.PickTransDimensional] == 1) {
			continue
		}
		var cand, why [8]int16
		for k := int16(0); k < n; k++ {
			if int8(A.TreatyStatus[k]) >= 4 && A.Contact[k] == 1 && i != k {
				cand[k] = 3
			}
		}
		// open is a living AI other than i, in contact and range, at peace
		// with no timer running.
		open := func(k int16) bool {
			K := &s.Players[k]
			return K.Objective != save.ObjectiveHuman && K.Eliminated == 0 && k != i && A.Contact[k] == 1 && A.InNormalRange[k] == 1
		}
		if A.Personality == 3 {
			for k := int16(0); k < n; k++ {
				K := &s.Players[k]
				if A.InNormalRange[k] == 1 && int8(A.TreatyTimer[k]) < 1 && K.Eliminated == 0 && cand[k] == 0 && i != k &&
					g.fleetStrengthComparison(i, k) >= 100 && K.Objective != save.ObjectiveHuman && r.Random(uint32(30*(diff*diff+1))) == 1 {
					cand[k], why[k] = 1, 0x14
				}
			}
		}
		for k := int16(0); k < n; k++ {
			if !open(k) || int8(A.TreatyTimer[k]) >= 1 || g.fleetStrengthComparison(i, k) < 100 || cand[k] != 0 ||
				int32(s.Stardate)%int32(n) != int32(k) {
				continue
			}
			t := (-int32(A.Relations[k]) - 5) / (2*diff + 1)
			if int32(r.Random(100)) <= t {
				cand[k], why[k] = 1, 0x44
			}
		}
		bankrupt := func() {
			for k := int16(0); k < n; k++ {
				if open(k) && g.fleetStrengthComparison(i, k) >= 100 && int8(A.TreatyTimer[k]) < 1 && cand[k] == 0 {
					cand[k], why[k] = 1, 0x71
				}
			}
		}
		if A.Unknown60E == 1 {
			bankrupt()
		}
		if int32(r.Random(100)) < int32(A.Diplo7EC) {
			bankrupt()
		}
		for k := int16(0); k < n; k++ {
			if open(k) && int8(A.TreatyTimer[k]) < 1 && cand[k] == 0 && A.WarLosses2[k] > 0 {
				cand[k], why[k] = 2, 0x16
			}
		}
		for k := int16(0); k < n; k++ {
			K := &s.Players[k]
			if K.Objective == save.ObjectiveHuman || k == i || K.Eliminated != 0 {
				continue
			}
			for m := int16(0); m < n; m++ {
				M := &s.Players[m]
				if M.Objective == save.ObjectiveHuman && m != i && M.Eliminated == 0 && int8(M.TreatyStatus[k]) >= 4 {
					cand[k] = 0
				}
			}
		}
		for k := int16(0); k < n; k++ {
			K := &s.Players[k]
			if K.Objective == save.ObjectiveHuman || K.Eliminated != 0 || i == k || A.InNormalRange[k] != 1 || cand[k] != 0 ||
				int32(s.Stardate)%int32(n) != int32(k) || int8(A.TreatyTimer[k]) >= 1 {
				continue
			}
			pers := s.getPlayerDiplomacyPersonality(i, k)
			t := 3*g.exeTableI16(persRelationModifier, int(pers)) + int16(r.Random(200)) + 125 + 25*int16(diff)
			switch A.TreatyStatus[k] {
			case 1:
				t += 50
			case 2:
				t += 100
			case 3:
				t += 200
			}
			if A.TradeTreaty[k] != 0 {
				t += 50
			}
			if A.ResearchTreaty[k] != 0 {
				t += 50
			}
			switch A.Tribute[k] {
			case 1:
				t += 50
			case 2:
				t += 100
			}
			for m := int16(0); m < n; m++ {
				M := &s.Players[m]
				if M.Objective != save.ObjectiveHuman || M.Eliminated != 0 || m == k || m == i || M.InNormalRange[k] != 1 {
					continue
				}
				if g.fleetStrengthComparison(m, i) > 100 {
					t -= int16(g.fleetStrengthComparison(m, i))
				}
			}
			if int32(g.fleetStrengthComparison(i, k)) >= int32(t) {
				cand[k], why[k] = 1, 0x17
			}
		}
		if diff >= 3 {
			for k := int16(0); k < n; k++ {
				K := &s.Players[k]
				if K.Objective == save.ObjectiveHuman || k == i || K.Eliminated != 0 {
					continue
				}
				for m := int16(0); m < n; m++ {
					M := &s.Players[m]
					if M.Objective != save.ObjectiveHuman && m != i && m != k && M.Eliminated == 0 && int8(M.TreatyStatus[k]) >= 4 {
						cand[k] = 0
					}
				}
			}
		}
		declare := func(k int16) {
			if s.Players[k].Objective == save.ObjectiveHuman {
				g.changeRelations(-10000, int(k), int(i), why[k], 0, 0)
			}
			g.declareWar(i, k)
		}
		for k := int16(0); k < n; k++ {
			if cand[k] == 2 && int8(A.TreatyTimer[k]) < 1 && k != i {
				declare(k)
			}
		}
		var busy, ones int16
		for k := int16(0); k < n; k++ {
			if int8(A.TreatyTimer[k]) >= 1 || i == k {
				continue
			}
			if int8(A.TreatyStatus[k]) >= 4 {
				busy++
			}
			if A.AIWarTarget != -1 {
				busy++
			}
			if cand[k] == 1 {
				ones++
			}
		}
		if busy != 0 || ones <= 0 {
			continue
		}
		pick := int16(r.Random(uint32(ones)))
		for k := int16(0); k < n; k++ {
			if cand[k] != 1 || i == k {
				continue
			}
			if pick--; pick == 0 {
				declare(k)
				break
			}
		}
	}
}

// npcPeaceNegotiations is NPC_Peace_Negotiations_ (0x2670a): a long war
// between two AIs ends in surrender or peace.
func (g *Game) npcPeaceNegotiations() {
	s := &g.state
	diff := int32(g.settings.Difficulty())
	for i := int16(0); i < s.NumPlayers; i++ {
		A := &s.Players[i]
		if A.Objective == save.ObjectiveHuman || A.Eliminated != 0 {
			continue
		}
		for j := int16(0); j < s.NumPlayers; j++ {
			B := &s.Players[j]
			if i == j || B.Objective == save.ObjectiveHuman || B.Eliminated != 0 {
				continue
			}
			if A.TreatyStatus[j] != 4 && A.TreatyStatus[j] != 5 || A.Contact[j] != 1 {
				continue
			}
			var humanWars int32
			for k := int16(0); k < s.NumPlayers; k++ {
				K := &s.Players[k]
				if K.Objective == save.ObjectiveHuman && K.Eliminated == 0 && k != i && k != j && int8(A.TreatyStatus[k]) >= 4 {
					humanWars++
				}
			}
			if 90-15*diff-20*int32(int16(humanWars)) >= int32(A.TurnsAtWar[j]) {
				continue
			}
			if to := g.npcSurrenderCheck(i); to != -1 {
				s.surrenderEmpire(i, to)
				s.eventSurrender(i, to)
				continue
			}
			g.declarePeace(j, i)
		}
	}
}

// npcSurrenderCheck is NPC_Surrender_Check_ (0x27a3d): whether a weak,
// battered AI surrenders its empire, and to whom (-1 not).
func (g *Game) npcSurrenderCheck(i int16) int16 {
	s := &g.state
	A := &s.Players[i]
	if A.Objective == save.ObjectiveHuman || int32(s.Stardate)-35000 < 150 {
		return -1
	}
	maxPop := int32(1)
	for k := int16(0); k < s.NumPlayers; k++ {
		if k != i && s.Players[k].Eliminated == 0 && int32(s.Players[k].Population) > maxPop {
			maxPop = int32(s.Players[k].Population)
		}
	}
	// The strongest fleet of the others is computed and never used; the
	// fleet share below is divided by the population maximum instead.
	var fleet, seen int32
	for k := int16(0); k < s.NumPlayers; k++ {
		if k != i && A.Contact[k] == 1 && s.Players[k].Eliminated == 0 {
			seen++
			fleet += A.FleetStrength[k]
		}
	}
	if seen > 0 {
		fleet /= seen
	}
	pp := int32(A.Population) * 100 / maxPop
	fp := fleet * 100 / maxPop
	var c int32
	if pp < 26 || fp < 26 {
		c = 5
		if pp < 26 {
			c = (26 - pp) * c / 12
		}
		if fp < 26 {
			c = (26 - fp) * c / 12
		}
	}
	pop := max(A.Population, 1)
	var losses int16
	for k := int16(0); k < s.NumPlayers; k++ {
		if k != i {
			losses += A.WarLosses2[k]
		}
	}
	l := int32(losses)
	d := int32(pop) + l
	if t := l * l * 100 / d / d; t > c {
		c = t
	}
	if c < int32(s.Random.Random(100)) {
		return -1
	}
	to := int16(-1)
	may := func(k int16) bool {
		return A.Contact[k] == 1 && k != i && s.Players[k].Eliminated == 0 && s.maySurrenderEmpire(i, k)
	}
	var best int16
	for k := int16(0); k < s.NumPlayers; k++ {
		K := &s.Players[k]
		if may(k) && K.Objective != save.ObjectiveHuman && int8(A.TreatyStatus[k]) < 4 && K.Population >= best {
			to, best = k, K.Population
		}
	}
	if to == -1 {
		best = 0
		for k := int16(0); k < s.NumPlayers; k++ {
			if K := &s.Players[k]; may(k) && K.Objective != save.ObjectiveHuman && K.Population < best {
				to, best = k, K.Population
			}
		}
	}
	if to == -1 && g.settings.Difficulty() < 3 {
		best = 0
		for k := int16(0); k < s.NumPlayers; k++ {
			if K := &s.Players[k]; may(k) && int8(A.TreatyStatus[k]) < 4 && K.Population < best {
				to, best = k, K.Population
			}
		}
	}
	if to == -1 && g.settings.Difficulty() < 3 {
		best = 0
		for k := int16(0); k < s.NumPlayers; k++ {
			if K := &s.Players[k]; may(k) && K.Population > best {
				to, best = k, K.Population
			}
		}
	}
	return to
}

// maySurrenderEmpire is May_Surrender_Empire_ (0xe4cd2).
func (s *State) maySurrenderEmpire(a, b int16) bool {
	return int16(s.Players[b].SurrenderTo) != a && s.Players[a].SurrenderTo == -1
}

// surrenderEmpire is Surrender_Empire_ (0xe4d06): a and everyone who had
// surrendered to a now belong to b, or to whomever b surrendered to.
func (s *State) surrenderEmpire(a, b int16) {
	s.Players[a].SurrenderTo = int8(b)
	for k := s.NumPlayers - 1; k >= 0; k-- {
		if int16(s.Players[k].SurrenderTo) == a {
			s.Players[k].SurrenderTo = int8(b)
		}
	}
	if s.Players[b].SurrenderTo == -1 {
		return
	}
	for k := s.NumPlayers - 1; k >= 0; k-- {
		if int16(s.Players[k].SurrenderTo) == b {
			s.Players[k].SurrenderTo = s.Players[b].SurrenderTo
		}
	}
}

// eventSurrender is Event_Surrender_ (0x233d2): the GNN news of a's
// surrender to b.
func (s *State) eventSurrender(a, b int16) {
	e := s.ev(eventSurrenderNews)
	e[evVictim] = uint8(a)
	e[evState] = 1
	s.setEvWord(eventSurrenderNews, evWord1, b)
}

// eventSurrenderNews is the event slot Event_Surrender_ fills.
const eventSurrenderNews = 34
