package game

// Surrenders (COLCALC.C), step 22 of Next_Turn_Calc_: stars and whole
// empires handed over by diplomacy. Spec: docs/turn/overview.md, D7.

// doSurrenders is Do_Surrenders_ (0xe4dc9).
func (g *Game) doSurrenders() {
	s := &g.state
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		for p := int(s.NumPlayers) - 1; p >= 0; p-- {
			if to := s.Stars[st].SurrenderTo[p]; to != -1 {
				g.doSurrenderStar(st, p, int(to))
			}
		}
	}
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		if to := s.Players[p].SurrenderTo; to != -1 {
			g.doSurrenderEmpire(p, int(to))
		}
	}
}

// doSurrenderStar is Do_Surrender_Star_ (0xe493a): p's colonies at the
// star go to player to, and p's governor there leaves.
func (g *Game) doSurrenderStar(star, p, to int) {
	s := &g.state
	st := &s.Stars[star]
	for orbit := 4; orbit >= 0; orbit-- {
		pl := st.Planets[orbit]
		if pl == -1 {
			continue
		}
		if c := int(s.Planets[pl].Colony); c != -1 && int16(s.Colonies[c].Owner) == int16(p) {
			g.doSurrenderColony(c, to)
		}
	}
	st.ColonyOwners &^= shl8(int8(p))
	st.ColonyOwners |= shl8(int8(to))
	st.Visited |= shl8(int8(to))
	s.computeStarColonyStuff(star)
	st.ColonizePlayer = -1
	if o := st.Officer[p]; o != -1 {
		g.moveOfficerToPool(int(o))
		st.Officer[p] = -1
	}
	st.SurrenderTo[p] = -1
}

// doSurrenderColony is Do_Surrender_Colony_ (0xe481f): the colony changes
// hands, its pops' loyalty with it; the capitol and the queue go.
func (g *Game) doSurrenderColony(ci, to int) {
	s := &g.state
	c := &s.Colonies[ci]
	old := uint32(c.Owner)
	if c.Buildings[9] != 0 {
		c.Buildings[9] = 0
	}
	c.AutoBuild = 0
	// A star that surrendered to a bombing AI holds the attacker + 8
	// (aiBeatColony); the colony goes to the attacker, as the original's
	// saves show (docs/turn/overview.md). Owner 9 indexed past Players.
	c.Owner = int8(to & 7)
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		if c.Colonists[i]>>4&7 == old {
			c.Colonists[i] = c.Colonists[i]&^0x70 | uint32(to&7)<<4
		}
	}
	star := int(s.Planets[c.Planet].Star)
	s.computeStarColonyStuff(star)
	if c.Buildings[0x2d] != 0 {
		g.updateShipETAs(-1)
	}
	s.colonyEnvironmentalStuff(c)
	g.enforcePopulationLimitsAtColony(ci)
	for k := 0; k < 7; k++ {
		g.deleteFromColonyQueue(ci, 0)
	}
	if c.IsOutpost == 0 {
		g.colonyProductionCalculation(ci)
	}
	g.updatePlayerStats(int(c.Owner))
	c.TurnsOwned = 0
	s.Stars[star].Visited |= shl8(int8(to))
}

// doSurrenderEmpire is Do_Surrender_Empire_ (0xe4b5f): ships, colonies,
// spies, freighters and treasury of p go to player to, who also learns
// p's applications among the first 83 (the loop bound is the field count),
// then p is eliminated.
func (g *Game) doSurrenderEmpire(p, to int) {
	s := &g.state
	L, W := &s.Players[p], &s.Players[to]
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if int16(sh.Owner) != int16(p) {
			continue
		}
		sh.Owner = int8(to)
		if sh.Officer != -1 {
			g.deassignOfficer(int(sh.Officer))
			sh.Officer = -1
		}
	}
	// Update_Ship_Icons_In_Between_Turns_: its star owners are saved, the
	// ship icons are UI caches.
	g.state.updateStarOwners()
	for a := numTechFields - 1; a >= 0; a-- {
		if L.AppStatus[a] == techKnown && g.playerMayGetTechApp(to, a) {
			g.playerGetsTechApp(to, a)
		}
	}
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if int16(c.Owner) != int16(p) {
			continue
		}
		g.doSurrenderColony(i, to)
		s.computeStarColonyStuff(int(s.Planets[c.Planet].Star))
	}
	n := int32(s.theirSpyNumber(to, to))
	var others int32
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		others += int32(s.theirSpyNumber(p, k))
	}
	s.setTheirSpyNumber(to, to, int16(min(n+others, 0x3f)))
	W.FreightersTotal += L.FreightersTotal
	W.BC += L.BC
	L.SurrenderTo = -1
	g.eliminatePlayer(p)
}

// eliminatePlayer is Eliminate_Player_ (0xe45ff): outposts are destroyed,
// leaders die, ships are lost, spies come home and treaties end; the
// player who caused it records the date.
func (g *Game) eliminatePlayer(p int) {
	s := &g.state
	pl := &s.Players[p]
	pl.Eliminated = 1
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		if c := &s.Colonies[i]; int16(c.Owner) == int16(p) && c.IsOutpost != 0 {
			g.destroyColony(i, -1)
		}
	}
	for o := maxLeaders - 1; o >= 0; o-- {
		l := &s.Leaders[o]
		if int16(l.Owner) != int16(p) {
			continue
		}
		l.Status = -2
		l.ETA = 0
		if loc := l.Location; loc != -1 {
			if l.Type == 0 {
				s.Ships[loc].Officer = -1
			} else {
				s.Stars[loc].Officer[p] = -1
			}
			l.Location = -1
		}
		l.Owner = -1
	}
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		if int16(s.Ships[i].Owner) == int16(p) {
			g.killShip(i)
		}
	}
	// Update_Ship_Icons_In_Between_Turns_: its star owners are saved, the
	// ship icons are UI caches.
	g.state.updateStarOwners()
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		s.setTheirSpyNumber(p, k, 0)
		if k != p {
			s.bringHomeSpiesVs(k, p)
		}
		K := &s.Players[k]
		K.TreatyStatus[p], K.TradeTreaty[p], K.ResearchTreaty[p], K.Tribute[p] = 0, 0, 0, 0
		pl.TreatyStatus[k], pl.TradeTreaty[k], pl.ResearchTreaty[k], pl.Tribute[k] = 0, 0, 0, 0
	}
	pl.FreightersTotal = 0
	pl.SettlersCount = 0
	pl.BC = 0
	if by := int32(int8(pl.CombatSlot)); uint32(by) < 8 {
		s.Players[by].Eliminations[p] = int16(s.Stardate)
	}
}
