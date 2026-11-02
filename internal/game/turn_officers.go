package game

import "github.com/openorion2/openorion2/internal/save"

// Common skills that make a player's own leaders attract new ones
// (Chance_To_Hire_Hero_ tests the masks of rows 10 and 11).
const (
	skillHireBonusFull    = 10
	skillHireBonusHalf    = 11
	leaderCount           = 0x43
	leaderNotForHireFirst = 0x41 // leaders 0x41 and 0x42 are never offered
)

// deltaStardate is Delta_Stardate_ (0x64395): turns since the start.
func (s *State) deltaStardate() int32 { return int32(s.Stardate) - 35000 }

// officerStatusOk is Officer_Status_Ok_ (0x9773f): pool, assigned, offered or 3.
func (s *State) officerStatusOk(o int) bool {
	switch s.Leaders[o].Status {
	case 0, 1, 4, 3:
		return true
	}
	return false
}

// nOfficersOwned is N_Fleet_Officers_Owned_By_Player_ (0x9776c) for type 0
// and N_Star_Officers_Owned_By_Player_ (0x977af) for type 1.
func (s *State) nOfficersOwned(p int, typ uint8) int16 {
	n := int16(0)
	for o := 0; o < leaderCount; o++ {
		l := &s.Leaders[o]
		if int16(l.Owner) == int16(p) && l.Type == typ && s.officerStatusOk(o) {
			n++
		}
	}
	return n
}

// chanceToHireHero is Chance_To_Hire_Hero_ (0x9781d), in percent.
func (g *Game) chanceToHireHero(p int) int16 {
	s := &g.state
	fleet := s.nOfficersOwned(p, 0)
	star := s.nOfficersOwned(p, 1)
	if s.deltaStardate() < 5 {
		return 0
	}
	pl := &s.Players[p]
	var c int16
	if pl.OfficerOfferDate == 0 {
		c = int16(s.deltaStardate())
	} else {
		c = int16(s.Stardate) - int16(pl.OfficerOfferDate)
	}
	c++
	if pl.Picks[save.PickCharismatic] != 0 {
		c += 5
	}
	if pl.Picks[save.PickRepulsive] != 0 {
		c -= 10
	}
	for o := 0; o < leaderCount; o++ {
		l := &s.Leaders[o]
		var lvl int16
		if l.Owner == -1 {
			lvl = min(officerBaseLevel(l.Experience, false), 5)
		} else {
			lvl = s.ownedOfficerLevel(o)
		}
		lvl++
		if int16(p) != int16(l.Owner) || l.Status != 0 && l.Status != 1 {
			continue
		}
		if g.leaderHasSkill(o, skillHireBonusFull) {
			c += lvl
		} else if g.leaderHasSkill(o, skillHireBonusHalf) {
			c = int16(int32(lvl)*15/10 + int32(c))
		}
	}
	d := int32(fleet) + 1 + int32(star)
	if d <= 1 {
		d = 1
	}
	return int16(int32(c) / d)
}

// leaderAvailableForHire is Leader_Available_For_Hire_ (0x97c2d).
func (s *State) leaderAvailableForHire(o int) bool {
	l := &s.Leaders[o]
	return l.Owner == -1 && o != leaderNotForHireFirst && o != leaderNotForHireFirst+1 && l.Status == -1
}

// selectLeaderForHire is Select_Leader_For_Hire_ (0x97b2d): a random free
// leader of a kind the player has fewer than 4 of, or -1 after 101 tries.
func (g *Game) selectLeaderForHire(p int) int {
	s := &g.state
	fleet := s.nOfficersOwned(p, 0)
	star := s.nOfficersOwned(p, 1)
	n := s.deltaStardate()/5 + 10 + int32(s.NumPlayers)
	pl := &s.Players[p]
	if pl.Picks[save.PickCharismatic] != 0 {
		n += 10
	} else if pl.Picks[save.PickRepulsive] != 0 {
		n = int32(int16(n) / 2)
	}
	limit := int16(leaderCount)
	if int16(n) < leaderCount {
		limit = int16(n)
	}
	found := -1
	for tries := int16(1); ; tries++ {
		o := int(int16(s.Random.Random(uint32(int32(limit))) - 1))
		l := &s.Leaders[o]
		kindOk := l.Type == 0 && fleet < 4 || l.Type == 1 && star < 4
		done := false
		if kindOk && s.leaderAvailableForHire(o) {
			done, found = true, o
		}
		if tries > 100 || done {
			return found
		}
	}
}

// generateRandomOfficer is Generate_Random_Officer_ (0x97ad4): the leader
// offered to player p this turn, if any.
func (g *Game) generateRandomOfficer(p int) int {
	s := &g.state
	o := g.selectLeaderForHire(p)
	pl := &s.Players[p]
	pl.OfficerForHire = int8(o)
	if int8(o) > -1 {
		pl.OfficerOfferDate = uint16(s.Stardate)
		s.Leaders[o].Owner = int8(p)
	}
	return o
}

// randomOfficerCheck is the Random_Officer_Check_ (0x97a66) loop of
// Next_Turn_Calc_, for every player, eliminated ones included.
func (g *Game) randomOfficerCheck() {
	s := &g.state
	for p := 0; p < int(s.NumPlayers); p++ {
		chance := g.chanceToHireHero(p)
		n := int32(s.nOfficersOwned(p, 0)) + int32(s.nOfficersOwned(p, 1))
		if n < 8 && chance > 0 && int16(s.Random.Random(100)-1) < chance {
			g.generateRandomOfficer(p)
		}
	}
}
