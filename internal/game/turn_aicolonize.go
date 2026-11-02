package game

import "github.com/openorion2/openorion2/internal/save"

// AI colonisation (COLONIZE.C): All_AI_Colonize_ and the colony founding it
// shares with the player. Spec: docs/turn/overview.md, All_AI_Colonize_.

// allAIColonize is All_AI_Colonize_ (0xe67f6).
func (g *Game) allAIColonize() {
	s := &g.state
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		if s.Players[p].Objective != objectiveHuman {
			g.aiColonize(p)
		}
	}
}

// playerColonyBaseAtStar is Player_Colony_Base_At_Star_ (0xfda3f): p's
// colony at st with a colony base (building 11), or -1.
func (s *State) playerColonyBaseAtStar(p, st int) int {
	S := &s.Stars[st]
	for k := 4; k >= 0; k-- {
		pl := S.Planets[k]
		if pl == -1 || s.Planets[pl].Colony == -1 {
			continue
		}
		ci := int(s.Planets[pl].Colony)
		if c := &s.Colonies[ci]; int16(c.Owner) == int16(p) && c.Buildings[11] > 0 {
			return ci
		}
	}
	return -1
}

// playerShipTypeAtStar is Player_Ship_Type_At_Star_ (0xfdaa7): p's ship of
// the design type at st (with any, else only one not just built), or -1.
func (s *State) playerShipTypeAtStar(p int, typ uint8, st int, any bool) int {
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if sh.Design.Type != typ || int16(sh.Owner) != int16(p) || sh.Location != int16(st) || int8(sh.Status) >= 5 {
			continue
		}
		if any || sh.JustBuilt == 0 {
			return i
		}
	}
	return -1
}

// aiColonize is AI_Colonize_ (0xe65f8): at each star the previous turn
// marked for p (+0x37), colony bases and colony ships settle the best free
// planet until none is left; bases with nothing to settle are sold.
func (g *Game) aiColonize(p int) {
	s := &g.state
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		S := &s.Stars[st]
		for int16(S.ColonizePlayer) == int16(p) {
			base := s.playerColonyBaseAtStar(p, st)
			ship := s.playerShipTypeAtStar(p, 1, st, true)
			if base == -1 && ship == -1 {
				break
			}
			best, bestV := int16(-1), uint32(0)
			for k := 4; k >= 0; k-- {
				pl := S.Planets[k]
				if pl == -1 || s.Planets[pl].Colony != -1 {
					continue
				}
				if v := uint32(uint16(g.uncolonizedPlanetWorthToPlayer(int(pl), p))); v > bestV {
					bestV, best = v, pl
				}
			}
			if best == -1 {
				for base != -1 {
					s.Players[p].BC += int32(g.exe.u32(colonyBaseRefund)) / 2
					s.Colonies[base].Buildings[11] = 0
					base = s.playerColonyBaseAtStar(p, st)
				}
				break
			}
			g.makeNewColonyOrOutpost(int(best), p, false)
			if base != -1 {
				g.removeBuilding(base, 11)
				continue
			}
			for i := int(s.NumShips) - 1; i >= 0; i-- {
				if sh := &s.Ships[i]; sh.AIMission == 4 && int16(sh.Owner) == int16(p) && int(sh.Location) == st {
					sh.AIMission = 0
				}
			}
			g.killShip(ship)
			g.updateShipIconsInBetweenTurns()
		}
	}
}

// initColony is Init_Colony_ (0x12d75).
func (g *Game) initColony(ci, planet, owner int) {
	s := &g.state
	c := &s.Colonies[ci]
	*c = save.Colony{}
	pl := &s.Planets[planet]
	pl.Colony = int16(ci)
	c.Unknown004 = -1
	c.Planet = int16(planet)
	c.Owner = int8(owner)
	s.Stars[pl.Star].Visited |= uint8(1) << (uint(owner) & 7)
	for i := range c.Queue {
		c.Queue[i] = -1
	}
	c.Occupation = 4
	c.LastBuilt = -1
}

// makeNewColonyOrOutpost is Make_New_Colony_Or_Outpost_ (0xe5eb3): in the
// slot reserved for p (or the planet's own colony record), one settler
// farming (working without farmland or for Lithovores and Cybernetics);
// natives on the planet join as three more pops.
func (g *Game) makeNewColonyOrOutpost(planet, p int, outpost bool) {
	s := &g.state
	pl := &s.Planets[planet]
	reused := false
	ci := 0
	if !outpost && pl.Colony != -1 {
		ci, reused = int(pl.Colony), true
	} else {
		for int16(s.Colonies[ci].ReservedFor) != int16(p) {
			ci++
		}
	}
	if int(s.NumColonies) <= ci {
		s.NumColonies = int16(ci + 1)
	}
	g.initColony(ci, planet, p)
	c := &s.Colonies[ci]
	c.ReservedFor = -1
	if outpost {
		c.IsOutpost = 1
		c.PopCount = 0
		c.Climate = pl.Climate
	} else {
		c.PopCount = 1
		v := &c.Colonists[0]
		P := &s.Players[p]
		if pl.FoodPerFarmer > 0 && P.Picks[save.PickLithovore] == 0 && P.Picks[save.PickCybernetic] == 0 {
			*v &^= colJobMask
		} else {
			*v = *v&^colJobMask | colWorker
		}
		*v = *v&^0xf | uint32(p)&0xf
		*v = *v&^0x70 | (uint32(p)&7)<<4
		*v |= colAssigned
		if pl.Special == 6 {
			for i := 1; i <= 3; i++ {
				n := &c.Colonists[i]
				*n = (*n|9)&^0x1f6 | (uint32(p)&7)<<4 | colAssigned
			}
			c.PopCount = 4
			pl.Special = 0
			s.Stars[pl.Star].Special = 0
		}
		if reused {
			c.Buildings[0x16] = 1
		}
		g.colonyCalculation(ci)
		c.LastBuilt = -39
	}
	c.Queue[0] = -1
	s.computeStarColonyStuff(int(pl.Star))
}
