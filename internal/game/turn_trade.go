package game

// Trade and research agreements (TRADE.C). Spec: docs/turn/overview.md,
// phase C.

// eventPirates is the pirate activity event in _event_data.
const eventPirates = 14

// eventCheckPirateActivity is Event_Check_Pirate_Activity_ (0x2346e): the
// pirates are active at a star where p has a colony.
func (s *State) eventCheckPirateActivity(p int16) bool {
	if !s.evActive(eventPirates) {
		return false
	}
	st := s.evWord(eventPirates, evWord1)
	return int32(s.Stars[st].ColonyOwners)>>uint(p)&1 != 0
}

// baseTradeAgreementGoal is Base_Trade_Agreement_Goal_ (0x101b3c).
func (g *Game) baseTradeAgreementGoal(a, b int16) int32 {
	s := &g.state
	v := min(int32(s.Players[a].TradePops), int32(s.Players[b].TradePops)) / 2
	if s.eventCheckPirateActivity(a) || s.eventCheckPirateActivity(b) {
		v /= 2
	}
	return v
}

// tradeAgreementGoal is Trade_Agreement_Goal_ (0x101ba4): the base goal
// scaled by government, Fantastic Traders and the best trading officer.
func (g *Game) tradeAgreementGoal(a, b int16) int16 {
	s := &g.state
	base := int32(int16(g.baseTradeAgreementGoal(a, b)))
	P := &s.Players[a]
	pct := int32(100)
	switch P.Picks[0] {
	case 4:
		pct = 150
	case 5:
		pct = 175
	}
	if P.Picks[24] != 0 {
		pct += 50
	}
	best := int16(0)
	for k := len(s.Leaders) - 1; k >= 0; k-- {
		l := &s.Leaders[k]
		if int32(l.Owner) != int32(a) || uint8(l.Status) >= 3 {
			continue
		}
		v := int32(0)
		if l.CommonSkills>>16&8 != 0 {
			v = (int32(s.ownedOfficerLevel(k)) + 1) * 15
		} else if l.CommonSkills>>16&4 != 0 {
			v = (int32(s.ownedOfficerLevel(k)) + 1) * 10
		}
		if int32(best) < v {
			best = int16(v)
		}
	}
	pct += int32(best)
	return int16(base * pct / 100)
}

// researchAgreementGoal is Research_Agreement_Goal_ (0x101cc5): half the
// smaller research population, scaled by government.
func (g *Game) researchAgreementGoal(a, b int16) int16 {
	s := &g.state
	v := min(s.Players[a].ResearchPops, s.Players[b].ResearchPops) / 2
	w := int32(v)
	switch s.Players[a].Picks[0] {
	case 0:
		return int16(w / 2)
	case 1:
		return int16(3 * w / 4)
	case 4:
		return int16(3 * w / 2)
	case 5:
		return int16(7 * w / 4)
	}
	return v
}

// creepToward is the shared step of Process_Trade_Agreement_ and
// Process_Research_Agreement_: a fifth of the goal per turn, the rest of
// the fifth by chance; a value above the goal drops to it.
func (g *Game) creepToward(value *int16, goal int16) {
	if *value >= goal {
		*value = goal
		return
	}
	extra := int32(0)
	if rem := int16(int32(goal) % 5); rem != 0 {
		if int32(rem) >= int32(g.state.Random.Random(5)) {
			extra = 1
		}
	}
	v := int16(int32(goal)/5 + int32(*value) + extra)
	*value = min(v, goal)
}

// processTradeAndResearchAgreements is Process_Trade_And_Research_Agreements_
// (0x101e77): the goals use last turn's trade and research populations,
// which are recounted after.
func (g *Game) processTradeAndResearchAgreements() {
	s := &g.state
	for a := s.NumPlayers - 1; a >= 0; a-- {
		P := &s.Players[a]
		for b := s.NumPlayers - 1; b >= 0; b-- {
			if P.TradeTreaty[b] != 0 {
				goal := g.tradeAgreementGoal(a, b)
				g.creepToward(&P.TradeValue[b], goal)
				P.TradeGoal[b] = goal
			}
			if P.ResearchTreaty[b] != 0 {
				goal := g.researchAgreementGoal(a, b)
				g.creepToward(&P.ResearchValue[b], goal)
				P.ResearchGoal[b] = goal
			}
		}
	}
	g.computeNTradeAndResearchPops()
}

// computeNTradeAndResearchPops is Compute_N_Trade_And_Research_Pops_
// (0x101a42): colonists of the eight player races trade and research,
// androids with bit 8 set (not bit 7) only research.
func (g *Game) computeNTradeAndResearchPops() {
	s := &g.state
	for p := s.NumPlayers - 1; p >= 0; p-- {
		s.Players[p].TradePops = 0
		s.Players[p].ResearchPops = 0
	}
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if c.Owner == -1 {
			continue
		}
		var trade, research int16
		for k := int(c.PopCount) - 1; k >= 0; k-- {
			e := c.Colonists[k]
			race := e & 0xf
			if race < 8 {
				trade++
				research++
			} else if e&0x180 == 0x100 && race == 8 {
				research++
			}
		}
		s.Players[c.Owner].TradePops += trade
		s.Players[c.Owner].ResearchPops += research
	}
}
