package game

import "github.com/openorion2/openorion2/internal/save"

// The AI's leaders (AILEADER.C) and the OFFICER.C routines they use.
// Spec: docs/turn/overview.md, Do_AI_Leaders_.

// objectiveHuman marks a human player in Player.Objective (+0x28).
const objectiveHuman = 100

// doAILeaders is Do_AI_Leaders_ (0xd7439).
func (g *Game) doAILeaders() {
	s := &g.state
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		pl := &s.Players[p]
		if pl.Objective == objectiveHuman {
			continue
		}
		if l := int(pl.OfficerForHire); l != -1 {
			paid := false
			if g.aiWantsLeader(p, l) {
				cost := int32(g.officerCost(p, l))
				if cost+50 < pl.BC {
					paid = true
					pl.BC -= cost
				}
			}
			g.setOfficerToPlayer(l, p, paid)
		}
		pl.OfficerForHire = -1
	}
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		sp := int8(s.Stars[st].Special)
		if sp < 0x50 || sp >= 0x5a {
			continue
		}
		p := int(sp) - 0x50
		if s.Players[p].Objective == objectiveHuman {
			continue
		}
		var who int16
		g.maroonedHeroAtStarCanJoin(st, &who, p)
	}
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		if s.Players[p].Objective != objectiveHuman {
			g.assignUnassignedLeaders(p)
		}
	}
}

// aiWantsLeader is the skill test of Do_AI_Leaders_: leader bytes +0x26..
// +0x2b, some skills only while p researches no hyper field or is not repulsive.
func (g *Game) aiWantsLeader(p, l int) bool {
	s := &g.state
	pl := &s.Players[p]
	ld := &s.Leaders[l]
	c := ld.CommonSkills
	if c&0x200 != 0 || c&0x100 != 0 {
		return true
	}
	if c&0x2000 != 0 && pl.Unknown59D == 0 || c&0x1000 != 0 && pl.Unknown59D == 0 {
		return true
	}
	if c&(0x8000|0x4000|0x20000|0x10000|0x2|0x1|0x80|0x40|0x800|0x400) != 0 {
		return true
	}
	if c&0x80000 != 0 && pl.Picks[save.PickRepulsive] == 0 || c&0x40000 != 0 && pl.Picks[save.PickRepulsive] == 0 {
		return true
	}
	return ld.Type == 0 && ld.SpecialSkills&(0x8000|0x4000|0x80|0x40) != 0
}

// setOfficerToPlayer is Set_Officer_To_Player_ (0x9718f): l joins p, paid
// or only offered (status 4). With four of l's kind, the offered one that
// waited longest is dismissed, or l is refused when there is none.
func (g *Game) setOfficerToPlayer(l, p int, paid bool) {
	s := &g.state
	best, bestETA, n := int16(-1), int16(0), int16(0)
	for o := 0; o < leaderCount; o++ {
		if o == l {
			continue
		}
		lo := &s.Leaders[o]
		if int16(lo.Owner) != int16(p) || lo.Status == -2 || lo.Status == -1 || lo.Type != s.Leaders[l].Type {
			continue
		}
		if lo.Status == 4 && int16(lo.ETA) > bestETA {
			bestETA, best = int16(lo.ETA), int16(o)
		}
		n++
	}
	if n == 4 {
		if best == -1 {
			return
		}
		g.deassignOfficer(int(best))
	}
	ld := &s.Leaders[l]
	ld.ETA = 0
	ld.Owner = int8(p)
	ld.Status = 4
	if paid {
		ld.Status = 0
		g.giveLeaderSpecialsToPlayer(l, p)
	}
	if s.Leaders[l].Type == 1 {
		g.doPlayerColonyCalculations(p)
	}
}

// giveLeaderSpecialsToPlayer is Give_Leader_Specials_To_Player_ (0x99223):
// leader 0x2f brings the advanced government, others their techs.
func (g *Game) giveLeaderSpecialsToPlayer(l, p int) {
	s := &g.state
	if l == 0x2f {
		app := 0
		switch uint16(g.getAdvancedGovernmentType(p)) {
		case 1:
			app = 0x2a
		case 3:
			app = 0x5c
		case 5:
			app = 0x41
		case 7:
			app = 0x4d
		}
		if g.playerMayGetTechApp(p, app) {
			g.playerGetsTechApp(p, app)
		}
	} else {
		for i := 0; i < 3; i++ {
			if a := int(s.Leaders[l].Techs[i]); a > 0 && g.playerMayGetTechApp(p, a) {
				g.playerGetsTechApp(p, a)
			}
		}
	}
	if s.Leaders[l].Type == 0 && (g.leaderHasSpecialSkill(l, 0x12) || g.leaderHasSpecialSkill(l, 0x13)) {
		s.Players[p].GalacticLore = 1
	}
}

// getAdvancedGovernmentType is Get_Advanced_Government_Type_ (0x93d0a): the
// advanced form of p's government (jump table 0x93cfa: 1, 3, 5, 7).
func (g *Game) getAdvancedGovernmentType(p int) int16 {
	switch int8(g.state.Players[p].Picks[save.PickGovernment] / 2) {
	case 0:
		return 1
	case 1:
		return 3
	case 2:
		return 5
	case 3:
		return 7
	}
	return 0
}

// maroonedHeroAtStarCanJoin is Marooned_Hero_At_Star_Can_Join_ (0x97f8d):
// the leader marooned at st joins p, or is dismissed when p has four.
func (g *Game) maroonedHeroAtStarCanJoin(st int, who *int16, p int) bool {
	s := &g.state
	dismiss := int16(-1)
	joins, found := false, false
	*who = -1
	for o := int16(0); o < leaderCount && !found; o++ {
		l := &s.Leaders[o]
		if l.Status != 5 || l.Location != int16(st) {
			continue
		}
		n := s.nOfficersOwned(p, l.Type)
		found = true
		*who = o
		if n < 4 {
			joins = true
		} else {
			joins, dismiss = false, o
		}
	}
	if dismiss != -1 {
		g.deassignOfficer(int(dismiss))
	} else if *who != -1 {
		l := &s.Leaders[*who]
		l.Owner = int8(p)
		l.Location = -1
		g.setOfficerToPlayer(int(*who), p, true)
		g.moveOfficerToPool(int(*who))
	}
	return joins
}

// assignUnassignedLeaders is Assign_Unassigned_Leaders_ (0xd73d4).
func (g *Game) assignUnassignedLeaders(p int) {
	s := &g.state
	for l := leaderCount - 1; l >= 0; l-- {
		ld := &s.Leaders[l]
		if int(ld.Owner) != p {
			continue
		}
		switch uint8(ld.Status) {
		case 0:
			if ld.Type == 0 {
				g.assignFleetOfficer(p, l)
			} else {
				g.assignStarOfficer(p, l)
			}
		case 1:
			if ld.Type == 0 {
				g.reassignFleetOfficer(p, l)
			}
		}
	}
}

// officerShipCandidate is the ship test of Assign_Fleet_Officer_ and
// Reassign_Fleet_Officer_: p's combat ship without an officer.
func (s *State) officerShipCandidate(p, i int) bool {
	sh := &s.Ships[i]
	return int(sh.Owner) == p && int8(sh.Status) <= 2 && sh.Design.Type == 0 && sh.Officer == -1
}

// officerShipBetter compares two ships for an officer: c must match best in
// hull, shield, armour and computer and have more experience.
func (s *State) officerShipBetter(best, c int) bool {
	b, x := &s.Ships[best], &s.Ships[c]
	return b.Design.Size <= x.Design.Size && b.Design.Shield <= x.Design.Shield &&
		b.Design.Armor <= x.Design.Armor && b.Design.Computer <= x.Design.Computer &&
		b.Experience < x.Experience
}

// assignFleetOfficer is Assign_Fleet_Officer_ (0xd6fda).
func (g *Game) assignFleetOfficer(p, l int) {
	s := &g.state
	best := -1
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		if s.officerShipCandidate(p, i) && (best == -1 || s.officerShipBetter(best, i)) {
			best = i
		}
	}
	if best != -1 {
		g.aiAssignLeaderToShip(p, l, best)
	}
}

// reassignFleetOfficer is Reassign_Fleet_Officer_ (0xd7078): a better ship
// in the same place as the officer's.
func (g *Game) reassignFleetOfficer(p, l int) {
	s := &g.state
	cur := int(s.Leaders[l].Location)
	best := cur
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		if s.officerShipCandidate(p, i) && s.shipsInSamePlace(i, cur) && s.officerShipBetter(best, i) {
			best = i
		}
	}
	if best != cur {
		g.aiAssignLeaderToShip(p, l, best)
	}
}

// shipsInSamePlace is Ships_In_Same_Place_ (0x97287): same x and y.
func (s *State) shipsInSamePlace(a, b int) bool {
	return s.Ships[a].X == s.Ships[b].X && s.Ships[a].Y == s.Ships[b].Y
}

// assignStarOfficer is Assign_Star_Officer_ (0xd7171): the colony leader
// goes to the star where its skills add most, ties to the higher star.
func (g *Game) assignStarOfficer(p, l int) {
	s := &g.state
	a := g.ai
	var score [72]int32
	sk := s.Leaders[l].SpecialSkills
	for ci := int(s.NumColonies) - 1; ci >= 0; ci-- {
		c := &s.Colonies[ci]
		if int(c.Owner) != p || c.IsOutpost != 0 {
			continue
		}
		st := int(s.Planets[c.Planet].Star)
		if s.Stars[st].Officer[p] != -1 {
			continue
		}
		info := a.colonyInfo(ci)
		v := &score[st]
		if sk&3 != 0 && info[4] != 0 {
			*v += int32(c.Pollution)
		}
		if sk&0xc != 0 && c.FoodPerFarmer != 0 {
			*v += int32(c.Output[0])
		}
		if sk&0x30 != 0 {
			*v += int32(c.PopCount)
		}
		if sk&0x300 != 0 {
			*v += int32(c.Output[1])
		}
		if sk&0x3000 != 0 {
			*v += int32(c.Output[2])
		}
		if sk&0xc000 != 0 && s.Players[p].Picks[save.PickGovernment]/2 != 3 {
			*v += int32(c.PopCount)
		}
		if sk&0xc00 != 0 {
			if d := int32(info[1]) - int32(c.PopCount); d > 0 {
				*v += d
			}
		}
		if sk&0x30000 != 0 {
			si := a.starInfo(st)
			*v += int32(si[6*p+2]) + int32(si[6*p+3])
		}
	}
	best := int(s.NumStars) - 1
	for st := best - 1; st >= 0; st-- {
		if score[best] < score[st] {
			best = st
		}
	}
	if score[best] != 0 {
		g.aiAssignLeaderToStar(p, l, best)
	}
}

// aiAssignLeaderToShip is Ai_Assign_Leader_To_Ship_ (0x98c23).
func (g *Game) aiAssignLeaderToShip(p, l, ship int) {
	g.assignLeaderToShip(l, ship, g.getShipLeaderETA(p, l, ship))
}

// aiAssignLeaderToStar is Ai_Assign_Leader_To_Star_ (0x98c87).
func (g *Game) aiAssignLeaderToStar(p, l, st int) {
	g.assignLeaderToStar(p, l, st, g.getStarLeaderETA(p, l, st))
}

// getShipLeaderETA is Get_Ship_Leader_ETA_ (0x98e4b): 0 when the leader is
// already with the ship, else 5. The officer screen's table (0x19c4c8) is
// getShipLeaderETA is Get_Ship_Leader_ETA_ (0x98e4b): 0 when the officer is
// already with the ship, else 5. While the leaders screen runs it judges
// by _prev_leaders: a pool leader reaches a ship at the capitol at once,
// a moved one keeps his ETA when the ship is where he was.
func (g *Game) getShipLeaderETA(p, l, ship int) uint8 {
	s := &g.state
	prev := g.prevLeaders
	var loc int16
	if prev == nil {
		loc = absoluteLocation(s.Leaders[l].Location)
	} else {
		loc = prev[l].location
	}
	home := int16(s.Planets[s.Players[p].HomePlanet].Star)
	same := loc != -1 && s.shipsInSamePlace(int(loc), ship)
	if prev == nil {
		if same {
			return 0
		}
		return 5
	}
	if st := prev[l].status; st == 0 || st == 4 {
		sh := &s.Ships[ship]
		if sh.Location == home || int16(g.xyToStarID(sh.X, sh.Y)) == home {
			return 0
		}
		return 5
	}
	if same {
		return uint8(prev[l].eta)
	}
	return 5
}

// getStarLeaderETA is Get_Star_Leader_ETA_ (0x98f42): 0 for a pool leader
// sent to the capitol's star, else 5; with _prev_leaders a leader sent
// back where he was keeps his ETA.
func (g *Game) getStarLeaderETA(p, l, st int) uint8 {
	s := &g.state
	home := int16(s.Planets[s.Players[p].HomePlanet].Star)
	prev := g.prevLeaders
	if prev == nil {
		if int16(st) == home && s.Leaders[l].Status == 0 {
			return 0
		}
		return 5
	}
	switch status := prev[l].status; {
	case status == 0 || status == 4:
		if int16(st) == home {
			return 0
		}
	case prev[l].location == int16(st):
		return uint8(prev[l].eta)
	}
	return 5
}

// assignLeaderToShip is Assign_Leader_To_Ship_ (0x98dad).
func (g *Game) assignLeaderToShip(l, ship int, eta uint8) {
	s := &g.state
	sh := &s.Ships[ship]
	if o := sh.Officer; o > -1 && o < leaderCount {
		old := &s.Leaders[o]
		old.Status = 0
		old.Location = -1
		old.ETA = 0
	}
	ld := &s.Leaders[l]
	if ld.Location > -1 {
		s.Ships[ld.Location].Officer = -1
	}
	ld.Status = 1
	ld.ETA = int8(eta)
	ld.Location = int16(ship)
	sh.Officer = int16(l)
}

// assignLeaderToStar is Assign_Leader_To_Star_ (0x98ceb).
func (g *Game) assignLeaderToStar(p, l, st int, eta uint8) {
	s := &g.state
	S := &s.Stars[st]
	if o := S.Officer[p]; o > -1 && o < leaderCount {
		old := &s.Leaders[o]
		old.ETA = 0
		old.Status = 0
		old.Location = -1
	}
	ld := &s.Leaders[l]
	if ld.Location > -1 {
		s.Stars[ld.Location].Officer[p] = -1
	}
	ld.Location = int16(st)
	ld.ETA = int8(eta)
	ld.Status = 1
	S.Officer[p] = int8(l)
}
