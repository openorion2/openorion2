package game

import "github.com/openorion2/openorion2/internal/save"

// Diplomacy steps of Next_Turn_Calc_ (DIPLOMAC.C, DIP-SCRN.C, NPCDIPLO.C).
// Spec: docs/turn/overview.md, phase C.

// clearDiplomacyMessages is Clear_Diplomacy_Messages_ (0x5090c): the
// per-turn reset of messages and demands, treaty timers and war counters.
func (g *Game) clearDiplomacyMessages() {
	s := &g.state
	g.modifierDiplomacyAdjustments()
	n := s.NumPlayers
	for i := int16(0); i < n; i++ {
		P := &s.Players[i]
		for j := int16(0); j < n; j++ {
			P.LastMessageParam[j] = P.MessageParam[j]
			P.MessageParam[j] = 0
			P.MessageThisTurn[j] = 0
			for k := 0; k < 4; k++ {
				P.TurnScratch[k][j] = 0
			}
			P.Demand[j] = 0
			P.MessageFlags[j] = 0
		}
	}
	for i := int16(0); i < n; i++ {
		for j := i + 1; j < n; j++ {
			A, B := &s.Players[i], &s.Players[j]
			if int8(A.TreatyTimer[j]) > 0 {
				A.TreatyTimer[j]--
			}
			if int8(B.TreatyTimer[i]) > 0 {
				B.TreatyTimer[i]--
			}
			// Only i's side is tested; both sides are cleared.
			if A.TreatyTimer[j] == 0 && A.TreatyStatus[j] == 3 {
				A.TreatyStatus[j] = 0
				B.TreatyStatus[i] = 0
			}
		}
	}
	for i := int16(0); i < n; i++ {
		for j := i + 1; j < n; j++ {
			A, B := &s.Players[i], &s.Players[j]
			if int8(A.TreatyStatus[j]) < 4 {
				continue
			}
			if A.TurnsAtWar[j] < 250 {
				A.TurnsAtWar[j]++
			}
			if B.TurnsAtWar[i] < 250 {
				B.TurnsAtWar[i]++
			}
		}
	}
}

// modifierDiplomacyAdjustments is Modifier_Diplomacy_Adjustments_ (0x50dac):
// negative and small modifiers creep up by Random(5), for every ordered
// pair including a player with itself.
func (g *Game) modifierDiplomacyAdjustments() {
	s := &g.state
	r := &s.Random
	for i := int16(0); i < s.NumPlayers; i++ {
		P := &s.Players[i]
		for j := int16(0); j < s.NumPlayers; j++ {
			if P.TreatyStatus[j] == 6 {
				continue
			}
			for k := 0; k < 4; k++ {
				if P.Modifiers[k][j] < 0 {
					P.Modifiers[k][j] += int16(r.Random(5))
				}
				if P.Modifiers[k][j] < 50 {
					P.Modifiers[k][j] += int16(r.Random(5))
				}
			}
			P.Modifiers[3][j] = max(min(P.Modifiers[3][j], 200), -200)
		}
	}
}

// resolveDelayedDiplomacyOrders is Resolve_Delayed_Diplomacy_Orders_
// (0x1ffed): orders given on the diplomacy screen take effect now.
func (g *Game) resolveDelayedDiplomacyOrders() {
	s := &g.state
	for i := int16(0); i < s.NumPlayers; i++ {
		for j := int16(0); j < s.NumPlayers; j++ {
			if i == j || s.Players[j].Eliminated != 0 || s.Players[i].Eliminated != 0 {
				continue
			}
			switch uint8(s.Players[i].DelayedOrder[j]) {
			case 0:
				g.declareWar(i, j)
			case 1:
				g.declarePeace(i, j)
			case 2:
				g.breakTreaties(i, j)
			}
			s.Players[i].DelayedOrder[j] = -1
		}
	}
}

// endOfTurnDiplomacyAdjustments is End_Of_Turn_Diplomacy_Adjustments_
// (0x4dab2).
func (g *Game) endOfTurnDiplomacyAdjustments() {
	s := &g.state
	n := s.NumPlayers
	g.diplomacyBrokenContacts()
	g.npcTreatyHatred()
	g.addDiplomacyTurnSummaryMessages()
	for i := int16(0); i < n; i++ {
		P := &s.Players[i]
		for j := int16(0); j < n; j++ {
			P.PrevTreatyStatus[j] = P.TreatyStatus[j]
		}
	}
	for i := int16(0); i < n; i++ {
		if P := &s.Players[i]; P.FoodSurplus >= 0 {
			P.Diplo7EC = 0
		} else {
			P.Diplo7EC++
		}
	}
	for i := int16(0); i < n; i++ {
		P := &s.Players[i]
		for j := int16(0); j < n; j++ {
			for k := 0; k < 4; k++ {
				P.Modifiers[k][j] += 10
			}
			for k := 0; k < 4; k++ {
				P.Modifiers[k][j] = min(P.Modifiers[k][j], 0)
			}
		}
	}
	for i := int16(0); i < n; i++ {
		if P := &s.Players[i]; P.Diplo816 > 0 {
			P.Diplo816--
		}
	}
	for i := int16(0); i < n; i++ {
		P := &s.Players[i]
		if P.Objective == save.ObjectiveHuman {
			continue
		}
		for j := int16(0); j < n; j++ {
			if s.Players[j].Objective != save.ObjectiveHuman || P.Contact[j] == 0 {
				continue
			}
			if P.LastProposalTurn[j] < 250 {
				P.LastProposalTurn[j]++
			}
			if P.ThreatTurn[j] < 250 {
				P.ThreatTurn[j]++
			}
		}
	}
	// The last loop, ending timed peace with Add_Peace_Ended_Msg_, tests the
	// address of the timer array instead of an element (0x4dd28) and never runs.
}

// diplomacyBrokenContacts is Diplomacy_Broken_Contacts_ (0x52602).
func (g *Game) diplomacyBrokenContacts() {
	s := &g.state
	for i := int16(0); i < s.NumPlayers; i++ {
		P := &s.Players[i]
		for j := int16(0); j < s.NumPlayers; j++ {
			if P.Contact[j] == 0 {
				P.Modifiers[1][j] = 0
				P.TradeTreaty[j] = 0
				P.ResearchTreaty[j] = 0
			}
		}
	}
}

// appTreatyHatred is the application whose holder resents treaties less
// in NPC_Treaty_Hatred_ (player +0x1df).
const appTreatyHatred = 200

// npcTreatyHatred is NPC_Treaty_Hatred_ (0x277cf): from difficulty 3 on, AIs
// without any pact with a human resent the human's pacts with others.
func (g *Game) npcTreatyHatred() {
	s := &g.state
	if g.settings.Difficulty() < 3 {
		return
	}
	for i := int16(0); i < s.NumPlayers; i++ {
		H := &s.Players[i]
		if H.Objective != save.ObjectiveHuman {
			continue
		}
		var w int16
		for k := int16(0); k < s.NumPlayers; k++ {
			if H.Contact[k] == 0 || k == i {
				continue
			}
			if H.TreatyStatus[k] == 1 {
				w += 100
			}
			if H.TreatyStatus[k] == 2 {
				w += 200
			}
			if H.TradeTreaty[k] != 0 {
				w += 50
			}
			if H.ResearchTreaty[k] != 0 {
				w += 50
			}
		}
		if H.AppStatus[appTreatyHatred] == 3 {
			w = int16(int32(w) * 2 / 3)
		}
		if g.settings.Difficulty() == 3 {
			w /= 2
		}
		if w <= 0 {
			continue
		}
		for j := int16(0); j < s.NumPlayers; j++ {
			A := &s.Players[j]
			if A.Objective == save.ObjectiveHuman || H.Contact[j] == 0 || H.TreatyStatus[j] != 0 ||
				H.ResearchTreaty[j] != 0 || H.TradeTreaty[j] != 0 || A.Personality == 5 {
				continue
			}
			H.Relations[j] = int8(int32(H.Relations[j]) - int32(w)/100)
			if int32(s.Random.Random(100)) <= int32(w)%100 {
				H.Relations[j]--
			}
			if H.Relations[j] <= -100 {
				H.Relations[j] = -100
			}
			A.Relations[i] = H.Relations[j]
		}
	}
}

// addDiplomacyTurnSummaryMessages is Add_Diplomacy_Turn_Summary_Messages_
// (0x1fef5): news of changed treaties the human player can see.
func (g *Game) addDiplomacyTurnSummaryMessages() {
	s := &g.state
	me := &s.Players[s.PlayerNum]
	for i := int16(0); i < s.NumPlayers; i++ {
		for j := i + 1; j < s.NumPlayers; j++ {
			if s.Players[j].Eliminated != 0 || s.Players[i].Eliminated != 0 {
				continue
			}
			if me.Contact[j] != 1 && me.Contact[i] != 1 {
				continue
			}
			P := &s.Players[i]
			if P.PrevTreatyStatus[j] == P.TreatyStatus[j] {
				continue
			}
			switch P.TreatyStatus[j] {
			case 1:
				g.addDiplomacyMsg(i, j, 3)
			case 2:
				g.addDiplomacyMsg(i, j, 2)
			case 3:
				g.addDiplomacyMsg(i, j, 1)
			case 4, 5, 6:
				g.addDiplomacyMsg(i, j, 0)
			}
		}
	}
}

// addDiplomacyMsg is Add_Diplomacy_Msg_ (0xef766), reached through
// Add_War_Msg_ (kind 0), Add_Peace_Msg_ (1), Add_Alliance_Msg_ (2),
// Add_Nonaggression_Msg_ (3) and Add_Peace_Ended_Msg_ (4, only a and b).
func (g *Game) addDiplomacyMsg(a, b int16, kind uint8) {
	s := &g.state
	var m [msgSize]byte
	m[0], m[2], m[3], m[4] = 0x11, kind, uint8(a), uint8(b)
	for p := s.NumPlayers - 1; p >= 0; p-- {
		if kind == 4 {
			if p != a && p != b {
				continue
			}
		} else if s.Players[p].Contact[a] == 0 && s.Players[p].Contact[b] == 0 {
			continue
		}
		g.addMsg(int(p), m)
	}
}

// personalityPeaceDuration is _personality_peace_duration, i16 by
// personality (0..6); the code reads only its low byte.
const personalityPeaceDuration = 0x18105c

// peaceDuration is the low byte of _personality_peace_duration[p].
func (g *Game) peaceDuration(p uint8) uint8 {
	return g.exeU8(personalityPeaceDuration + 2*uint32(p))
}

// getPlayerDiplomacyPersonality is Get_Player_Diplomacy_Personality_
// (0x53e96), also inlined in places: personality 4 counts as 6 once i has
// been betrayed by j.
func (s *State) getPlayerDiplomacyPersonality(i, j int16) uint8 {
	P := &s.Players[i]
	if P.Personality == 4 && P.Diplo727[j] == 1 {
		return 6
	}
	return P.Personality
}

// determineFirstContacts is Determine_First_Contacts_ (0x50b57): humans
// meeting someone for the first time, or again after losing contact.
func (g *Game) determineFirstContacts() {
	s := &g.state
	for i := int16(0); i < s.NumPlayers; i++ {
		H := &s.Players[i]
		if H.Objective != save.ObjectiveHuman {
			continue
		}
		for j := int16(0); j < s.NumPlayers; j++ {
			if H.Contact[j] != 1 {
				continue
			}
			O := &s.Players[j]
			bothHuman := O.Objective == save.ObjectiveHuman
			if H.WarDeclared[j] == 0 {
				H.WarDeclared[j] = 1
				H.TreatyStatus[j], O.TreatyStatus[i] = 0, 0
				H.TreatyTimer[j] += 2 * g.peaceDuration(s.getPlayerDiplomacyPersonality(i, j))
				O.TreatyTimer[i] += 2 * g.peaceDuration(s.getPlayerDiplomacyPersonality(j, i))
				switch {
				case !bothHuman:
					g.changeRelations(-10000, int(i), int(j), 0x11, 0, 0)
				case s.CouncilFlag != 3:
					O.MessageFlags[i] = O.Personality + 1
				}
				continue
			}
			if H.LostContactMask>>uint(j)&1 == 0 {
				continue
			}
			switch {
			case !bothHuman:
				g.changeRelations(-10000, int(i), int(j), 0x12, 0, 0)
			case s.CouncilFlag == 3:
			case O.Relations[i] < 0 && int8(O.TreatyStatus[i]) < 4:
				O.MessageFlags[i] = O.Personality + 7
			default:
				O.MessageFlags[i] = O.Personality + 0xd
			}
		}
	}
}

// diplomacyGrowth is Diplomacy_Growth_ (0x4dd6b): treaties warm relations,
// threatening fleets chill them, the rest drifts back to the base value.
func (g *Game) diplomacyGrowth() {
	s := &g.state
	r := &s.Random
	n := int32(s.NumPlayers)
	for i := int32(0); i < n; i++ {
		for j := i + 1; j < n; j++ {
			A, B := &s.Players[i], &s.Players[j]
			if A.Objective == save.ObjectiveHuman && B.Objective == save.ObjectiveHuman {
				continue
			}
			a, b := int(int16(i)), int(int16(j))
			if r.Random(2) == 1 && A.Relations[j] < -50 && int8(A.TreatyTimer[j]) > 0 {
				g.changeRelations(1, a, b, 0, 0, 0)
			}
			// gate is the Random(100) <= 100-relations roll of each treaty.
			gate := func() bool { return int32(r.Random(100)) <= 100-int32(A.Relations[j]) }
			if A.TreatyStatus[j] == 1 && gate() {
				g.changeRelations(int16(r.Random(3)), a, b, 0, 0, 0)
			}
			if A.TradeTreaty[j] == 1 && gate() {
				g.changeRelations(int16(r.Random(3)), a, b, 0xc, 0, 0)
			}
			if A.ResearchTreaty[j] == 1 && gate() {
				g.changeRelations(int16(r.Random(3)), a, b, 0xd, 0, 0)
			}
			if A.TreatyStatus[j] == 2 {
				g.changeRelations(int16(r.Random(5)), a, b, 0, 0, 0)
			}
			if A.Tribute[j] == 1 && gate() {
				g.changeRelations(int16(r.Random(3)), a, b, 0xe, 0, 0)
			}
			if A.Tribute[j] == 2 && gate() {
				g.changeRelations(int16(r.Random(8)), a, b, 0xe, 0, 0)
			}
			A.Relations[j] = min(A.Relations[j], 100)
			B.Relations[i] = min(B.Relations[i], 100)
		}
	}
	for i := int32(0); i < n; i++ {
		for j := int32(0); j < n; j++ {
			H := &s.Players[i]
			if H.Objective != save.ObjectiveHuman || s.Players[j].Objective == save.ObjectiveHuman || H.TreatyStatus[j] == 2 {
				continue
			}
			star, pct, planet := g.playerThreatensPlayer(int16(i), int16(j))
			if planet == -1 {
				continue
			}
			ci := s.Planets[planet].Colony
			if ci == -1 || star == -1 {
				continue
			}
			pop := int32(s.Colonies[ci].PopCount)
			if pop <= 4 || pct <= 100 {
				continue
			}
			g.changeRelations(int16(-(pop - 4 + pct/50)), int(int16(i)), int(int16(j)), 5, int16(star), 0)
		}
	}
	for i := int32(0); i < n; i++ {
		P := &s.Players[i]
		for j := int32(0); j < n; j++ {
			var d int8
			if int32(r.Random(105)) > abs32(int32(P.Relations[j])) && r.Random(4) == 1 {
				d = int8(r.Random(2) - 1)
			}
			if i != j && P.LastMessageParam[j] == 0 {
				rel, base := &P.Relations[j], P.BaseRelations[j]
				if base > *rel {
					*rel += d
					if base < *rel {
						*rel = base
					}
				} else {
					*rel -= d
					if base > *rel {
						*rel = base
					}
				}
				*rel = min(max(*rel, -100), 100)
			}
			if int8(P.TreatyStatus[j]) >= 4 && P.Relations[j] > -90 {
				P.Relations[j] = -90
				s.Players[j].Relations[i] = -90
			}
		}
	}
	for i := int32(0); i < n; i++ {
		for j := int32(0); j < n; j++ {
			if i == j {
				s.Players[i].Relations[i] = 0
			} else {
				s.Players[i].Relations[j] = s.Players[j].Relations[i]
			}
		}
	}
	for i := int32(0); i < n; i++ {
		P := &s.Players[i]
		for j := int32(0); j < n; j++ {
			if P.Diplo806[j] != 0 {
				P.Diplo806[j]--
			}
			if P.Diplo80E[j] != 0 {
				P.Diplo80E[j]--
			}
		}
	}
	if s.CouncilFlag != 3 {
		return
	}
	for i := int32(0); i < n; i++ {
		for j := int32(0); j < n; j++ {
			if i != j {
				s.Players[i].TreatyStatus[j], s.Players[j].TreatyStatus[i] = 6, 6
				s.Players[i].Relations[j], s.Players[j].Relations[i] = -100, -100
			}
		}
	}
}

// setOpportunityAttacks is Set_Opportunity_Attacks_ (0x1fd80): what each AI
// could take from each human it borders, for the sneak attack decisions.
// SneakStar receives a planet.
func (g *Game) setOpportunityAttacks() {
	s := &g.state
	for i := int16(0); i < s.NumPlayers; i++ {
		A := &s.Players[i]
		for j := int16(0); j < s.NumPlayers; j++ {
			H := &s.Players[j]
			if A.Eliminated == 0 && A.Objective != save.ObjectiveHuman && !g.hyperspaceFlux() &&
				H.Objective == save.ObjectiveHuman && i != j && H.Eliminated == 0 &&
				A.Contact[j] == 1 && A.InNormalRange[j] == 1 {
				planet, worth, pct := int16(-1), int16(0), int32(0)
				g.findOpportunityAttack(i, j, 100, &planet, &worth, &pct)
				A.SneakStar[j], A.SneakValue[j], A.SneakStrength[j] = planet, worth, pct
				continue
			}
			A.SneakStar[j], A.SneakValue[j], A.SneakStrength[j] = -1, 0, 0
		}
	}
}
