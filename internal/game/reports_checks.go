package game

import "github.com/openorion2/openorion2/internal/save"

// The begin-of-turn report checks of Display_Report_Aux_ (REPORTS.C). Each
// one answers whether something is to be shown (query) or shows it. Spec:
// docs/screens/reports-and-begin-of-turn.md, "The checks".

// Report codes Display_Report_Aux_ returns; see the spec for what each means.
const (
	reportNone       = 0
	reportPopup      = 1
	reportFullScreen = 2
	reportLeaves     = 3
	reportSummary    = 4
	reportDiplomacy  = 5
	reportAntarans   = 6
	reportGNN        = 7
)

// reportCheck names the checks (those whose show side may be unported: see
// skipped).
type reportCheck int

const (
	checkAntarans reportCheck = iota
	checkGNN
	checkInitialTech
	checkBreakthrough
	checkEvolution
	checkColonization
	checkExplored
	checkStolenTech
	checkSummary
	checkColonyQueue
	checkOccupation
	checkHire
	checkOfficerLevel
	checkDiplomacy
	numReportChecks
)

// reportState is port-only bookkeeping of the report phase.
type reportState struct {
	// skipped marks checks whose show side is not ported and was passed
	// over this stardate, so that their query does not loop forever.
	skipped      [numReportChecks]bool
	skippedStamp uint32
	// last is the check that answered displayReportAux's last call.
	last reportCheck
}

// skipUnported logs an unported show path and silences its check until the
// next stardate: the original would have changed the state it tests.
func (g *Game) skipUnported(c reportCheck, what string) {
	r := &g.rep
	if r.skippedStamp != g.state.Stardate {
		r.skipped, r.skippedStamp = [numReportChecks]bool{}, g.state.Stardate
	}
	r.skipped[c] = true
	g.Log("report: %s is not ported; skipped", what)
}

func (g *Game) isSkipped(c reportCheck) bool {
	return g.rep.skippedStamp == g.state.Stardate && g.rep.skipped[c]
}

// displayReportAux is Display_Report_Aux_ (0xfe63e): the first report for p,
// shown when show is set. With nothing left, p's done bit is set, also in
// query mode.
func (g *Game) displayReportAux(p int, show bool) (int, error) {
	s := &g.state
	if s.DonePlayerReports>>uint(p)&1 != 0 || s.Stardate == 35000 {
		return reportNone, nil
	}
	r, err := g.antaransReport(show)
	if err != nil {
		return 0, err
	}
	if r == reportNone {
		if s.DisplayedGNN>>uint(p)&1 == 0 && g.checkForEventMessage() {
			r = reportGNN
			if show {
				if err := g.checkForEvent(); err != nil {
					return 0, err
				}
				s.DisplayedGNN |= 1 << uint(p)
			}
		}
	}
	if r == reportNone {
		// Community patch 1.51's order: reports (a stolen tech) before the tech
		// pick, exploration before colonization, the hire offer after both.
		for _, c := range []struct {
			id reportCheck
			f  func(int, bool) (int, error)
		}{
			{checkInitialTech, g.setInitialTech}, {checkStolenTech, g.hasReport},
			{checkBreakthrough, g.hasResearchBreakthrough}, {checkEvolution, g.hasEvolutionaryUpgrade},
			{checkExplored, g.exploredNewStar}, {checkColonization, g.colonization},
			{checkHire, g.hasOfficerForHire}, {checkOfficerLevel, g.officerMadeLevelReport},
			{checkDiplomacy, g.hasDiplomacyMessages}, {checkSummary, g.hasMsg},
			{checkColonyQueue, g.emptyColonyQueue}, {checkOccupation, g.needToSetOccupationPolicy},
		} {
			if r, err = c.f(p, show); err != nil || r != reportNone {
				g.rep.last = c.id
				break
			}
		}
		if err != nil {
			return 0, err
		}
	}
	if r == reportNone {
		s.DonePlayerReports |= 1 << uint(p)
	}
	return r, nil
}

// antaransReport is Antarans_ (0xfe552). It tests PLAYER_NUM's bit, not p's.
func (g *Game) antaransReport(show bool) (int, error) {
	s := &g.state
	pn := uint(s.PlayerNum)
	if int32(s.Stardate)-1 != s.antaranAttackDate() || s.Antarans[0x41]>>pn&1 != 0 || g.isSkipped(checkAntarans) {
		return reportNone, nil
	}
	if !show {
		return reportAntarans, nil
	}
	dest, best := -1, uint8(0)
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if sh.Owner == 8 && sh.Status == 1 && sh.ETA > best {
			best, dest = sh.ETA, int(sh.Location)-500
		}
	}
	r := reportPopup
	if dest != -1 && s.Stars[dest].ColonyOwners>>pn&1 != 0 && s.Random.Random(10) <= 4 {
		if err := g.mainAntaranRoomScreen(s.antaranAttackDate() < 35000); err != nil {
			return 0, err
		}
		r = reportAntarans
	} else {
		if err := g.playAntaransAttackFlic(); err != nil {
			return 0, err
		}
	}
	s.Antarans[0x41] |= 1 << pn
	return r, nil
}

// antaranAttackDate is the stardate at antarans+0x3d (s32).
func (s *State) antaranAttackDate() int32 {
	b := s.Antarans[0x3d:]
	return int32(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)
}

// checkForEventMessage is Check_For_Event_Message_ (0x2023f): an event in
// state 4, 5 or 6 has news.
func (g *Game) checkForEventMessage() bool {
	for i := 0; i < 0x24; i++ {
		if st := g.state.EventData[9*i+1]; st == 4 || st == 5 || st == 6 {
			return true
		}
	}
	return false
}

// setInitialTech is Set_Initial_Tech_ (0xfd911): after the first stardate a
// player with no research field (field 0 known) picks one.
func (g *Game) setInitialTech(p int, show bool) (int, error) {
	s := &g.state
	pl := &s.Players[p]
	if s.Stardate <= 35000 || pl.FieldStatus[pl.ResearchField] != 3 || pl.ResearchField != 0 || g.isSkipped(checkInitialTech) {
		return reportNone, nil
	}
	if show {
		if err := g.techSelect(); err != nil {
			return 0, err
		}
	}
	return reportFullScreen, nil
}

// hasResearchBreakthrough is Has_Research_Breakthrough_ (0xfe2e8).
func (g *Game) hasResearchBreakthrough(p int, show bool) (int, error) {
	pl := &g.state.Players[p]
	if pl.BreakthroughPending == 0 || g.isSkipped(checkBreakthrough) {
		return reportNone, nil
	}
	if !show {
		return reportPopup, nil
	}
	if err := g.techSelect(); err != nil {
		return 0, err
	}
	pl.BreakthroughPending = 0
	return reportFullScreen, nil
}

// hasEvolutionaryUpgrade is Has_Evolutionary_Upgrade_ (0xfe31f): Evolutionary
// Mutation known (application 0x3e) and not yet used.
func (g *Game) hasEvolutionaryUpgrade(p int, show bool) (int, error) {
	pl := &g.state.Players[p]
	if pl.AppStatus[0x3e] != 3 || pl.Evolved != 0 {
		return reportNone, nil
	}
	if show {
		if err := g.setupEvolutionaryUpgrade(p); err != nil {
			return 0, err
		}
	}
	return reportPopup, nil
}

// hasOfficerForHire is Has_Officer_For_Hire_ (0xfe251): the officer offered
// this turn joins p, paid when the player hires him.
func (g *Game) hasOfficerForHire(p int, show bool) (int, error) {
	pl := &g.state.Players[p]
	l := int(pl.OfficerForHire)
	if l == -1 {
		return reportNone, nil
	}
	if !show {
		return reportPopup, nil
	}
	hire, err := g.randomNewOfficerPopup(p, l)
	if err != nil {
		return 0, err
	}
	cost := int32(g.calculateOfficerCost(p, l, hire))
	g.setOfficerToPlayer(l, p, hire)
	if hire {
		pl.BC -= cost
		g.doPlayerColonyCalculations(p)
	}
	pl.OfficerForHire = -1
	return reportPopup, nil
}

// colonization is Colonization_ (0xfdb01): a star of p's with a colony base,
// or with a colony ship or outpost ship whose prompt is not switched off.
// Stars without either are marked done, also in query mode.
func (g *Game) colonization(p int, show bool) (int, error) {
	s := &g.state
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		if int16(s.Stars[st].ColonizePlayer) != int16(p) || testBit(s.DoneColonizeCheck[:], st) {
			continue
		}
		base := s.playerColonyBaseAtStar(p, st)
		colony := s.playerShipTypeAtStar(p, 1, st, false)
		outpost := s.playerShipTypeAtStar(p, 4, st, false)
		ship := colony != -1 || outpost != -1
		if base == -1 && (!ship || s.Stars[st].ColonizePromptOff>>uint(p)&1 != 0) {
			setBit(s.DoneColonizeCheck[:], st)
			continue
		}
		if !show {
			return reportPopup, nil
		}
		return g.colonizeShow(p, st, base, colony, outpost)
	}
	return reportNone, nil
}

// exploredNewStar is Explored_New_Star_ (0xfd95a): the last star p has newly
// explored (star+0x34) or carries p's marker (special >= 50, %10 == p). An
// omniscient player's plain discoveries are cleared one per call, unshown.
func (g *Game) exploredNewStar(p int, show bool) (int, error) {
	s := &g.state
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		S := &s.Stars[st]
		sp := int8(S.Special)
		marker := sp >= 0x32 && int16(sp)%10 == int16(p)
		if S.SpecialPopup>>uint(p)&1 == 0 && !marker {
			continue
		}
		if s.Players[p].Picks[save.PickOmniscient] != 0 && !marker {
			S.SpecialPopup &^= 1 << uint(p)
			return reportNone, nil
		}
		if show {
			if err := g.newSystemDiscoveryPopup(st, g.drawSpecial(st, p)); err != nil {
				return 0, err
			}
			S.SpecialPopup &^= 1 << uint(p)
			if marker {
				S.Special = 0
			}
		}
		return reportPopup, nil
	}
	return reportNone, nil
}

// officerMadeLevelReport is Officer_Made_Level_Report_ (0x949a7): the first
// of p's officers in service that gained a level.
func (g *Game) officerMadeLevelReport(p int, show bool) (int, error) {
	s := &g.state
	for l := 0; l < leaderCount; l++ {
		ld := &s.Leaders[l]
		if !(ld.Status == 1 && ld.Location != -1 || ld.Status == 0) {
			continue
		}
		if int16(ld.Owner) != int16(p) || ld.LevelUp == 0 {
			continue
		}
		if show {
			loc := ld.Location
			if ld.Type == 0 && loc > -1 {
				if sl := s.Ships[loc].Location; sl >= 500 && sl < 1000 {
					loc = -1
				} else {
					loc = absoluteLocation(sl)
				}
			}
			if err := g.officerGainsLevelPopup(int(loc), l); err != nil {
				return 0, err
			}
			ld.LevelUp = 0
		}
		return reportPopup, nil
	}
	return reportNone, nil
}

// hasReport is Has_Report_ (0xfe359): p's spy and combat reports, each shown
// as a message box or, for a stolen technology, the show-off screen.
func (g *Game) hasReport(p int, show bool) (int, error) {
	s := &g.state
	r := reportNone
	for i := 0; i < int(s.NReports); i++ {
		rec := s.Reports[i*reportSize : (i+1)*reportSize]
		if int16(uint16(rec[1])|uint16(rec[2])<<8) != int16(p) {
			continue
		}
		r = reportPopup
		if !show {
			continue
		}
		if rec[0] == 1 && rec[3] == 0 {
			// Remove_Box_Check_ (0x919a3) is empty.
			tech := int(int16(uint16(rec[6]) | uint16(rec[7])<<8))
			if err := g.showOffStolenTech(tech, int(rec[5])); err != nil {
				return 0, err
			}
			r = reportFullScreen
		} else {
			msg, err := g.sprintfReport(rec)
			if err != nil {
				return 0, err
			}
			if err := g.messageBox(msg); err != nil {
				return 0, err
			}
		}
		rec[1], rec[2] = 0xff, 0xff
	}
	return r, nil
}

// hasDiplomacyMessages is Has_Diplomacy_Messages_ (0xfe408): each player not
// ignored by p with a message for p sends its ambassador. In network games
// an interactive message goes through the arbiter (docs/net/diplomacy.md).
func (g *Game) hasDiplomacyMessages(p int, show bool) (int, error) {
	s := &g.state
	r := reportNone
	for i := 0; i < int(s.NumPlayers); i++ {
		if i == p || s.Players[p].Ignoring>>uint(i)&1 != 0 || s.Players[i].MessageFlags[p] == 0 {
			continue
		}
		r = reportDiplomacy
		if !show {
			continue
		}
		net := (g.gameType == 2 || g.gameType == 3) && diplomacyMessageIsInteractive(int16(s.Players[i].MessageFlags[p]))
		if net {
			// An answer changes the AI: the arbiter lets one human at a time.
			g.netRequestAIDiplomacy(int8(i))
			for !g.netMayBeginAIDiplomacy() {
				if err := g.netWait(); err != nil {
					return 0, err
				}
			}
		}
		if err := g.npcDiplomacyScreen(i, false); err != nil {
			return 0, err
		}
		s.Players[i].MessageFlags[p] = 0
		if net {
			g.netEndAIDiplomacy()
		}
	}
	return r, nil
}

// hasMsg is Has_Msg_ (0xfe0ea): with the summary option on, p's messages
// include a serious one (or any, unless only serious ones are wanted).
func (g *Game) hasMsg(p int, show bool) (int, error) {
	s := &g.state
	if s.DisplayedMsgs>>uint(p)&1 != 0 || s.NMsgs[p] == 0 || g.settings[0] == 0 {
		return reportNone, nil
	}
	ns := g.settings[0xc] == 0
	n := int(s.NMsgs[p])
	first := int(s.FirstMsg[p])
	serious := false
	for k := n - 1; k >= 0 && !serious; k-- {
		m := s.Msgs[(first+k)*msgSize:]
		switch t := m[0]; {
		case t == 0:
			switch c := m[3]; {
			case c == 4 || c == 9:
				serious = true
			case c >= 6 && c <= 8:
			default:
				serious = ns
			}
		case t == 4 || t == 5 || t == 7:
			serious = true
		case t == 8 || t == 0x10:
		case t == 9:
			serious = int16(uint16(m[2])|uint16(m[3])<<8) != -1 && m[7] != 0
		case t == 0x11:
			serious = int16(m[3]) != s.PlayerNum && ns
		default:
			serious = ns
		}
	}
	if !serious {
		return reportNone, nil
	}
	if show {
		g.currentScreen, g.returnScreen = ScreenTurnSummary, ScreenReports
		g.globalEsc = true
		s.DisplayedMsgs |= 1 << uint(p)
	}
	return reportSummary, nil
}

// emptyColonyQueue is Empty_Colony_Queue_ (0xfdf93): the last of p's
// colonies (not outposts) with nothing to build opens the colony screen.
func (g *Game) emptyColonyQueue(p int, show bool) (int, error) {
	s := &g.state
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if int16(c.Owner) != int16(p) || c.IsOutpost != 0 || c.Queue[0] != -1 {
			continue
		}
		if show {
			g.openColonyScreen(c.Planet)
			g.globalEsc = true
		}
		return reportLeaves, nil
	}
	return reportNone, nil
}

// needToSetOccupationPolicy is Need_To_Set_Occupation_Policy_ (0xfe02c): a
// newly conquered colony of p's (occupation 4) with conquered colonists.
func (g *Game) needToSetOccupationPolicy(p int, show bool) (int, error) {
	s := &g.state
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if int16(c.Owner) != int16(p) || c.Occupation != 4 {
			continue
		}
		for k := int(c.PopCount) - 1; k >= 0; k-- {
			if c.Colonists[k]>>8&4 == 0 {
				continue
			}
			if show {
				// The original runs Colony_Screen_ here and carries on.
				g.openColonyScreen(c.Planet)
				if err := g.colonyScreen(); err != nil {
					return 0, err
				}
			}
			return reportPopup, nil
		}
	}
	return reportNone, nil
}

// openColonyScreen sets up screen 1 for planet pl, back to the reports.
func (g *Game) openColonyScreen(pl int16) {
	P := &g.state.Planets[pl]
	g.currentScreen, g.returnScreen = ScreenColony, ScreenReports
	g.screenData[0], g.screenData[1] = int16(P.Star), int16(P.Orbit)
}

// clearAutoColonizationFlag is Clear_Auto_Colonization_Flag_ (0xa1bf4): a
// colony or outpost ship forgets the planet it was sent to settle.
func (s *State) clearAutoColonizationFlag(i int) {
	if d := &s.Ships[i].Design; d.Type == 1 || d.Type == 4 {
		d.Weapons[0].Weapon = -1
		d.Weapons[1].Weapon = 0
	}
}
