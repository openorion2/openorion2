package game

// Contacts and spy bookkeeping (CONTACT.C, BILL.C, SPY.C). Spec:
// docs/turn/overview.md, phase C.

// computeContacts is Compute_Contacts_ (0xeb192): players are in contact when
// a colony of one lies within the other's extended fuel range; contacts
// that appear are counted, and without contact treaties lapse and spies
// come home.
func (g *Game) computeContacts() {
	s := &g.state
	var old [8][8]uint8
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		P := &s.Players[p]
		old[p] = P.Contact
		P.Contact = [8]uint8{}
		P.InNormalRange = [8]uint8{}
		P.InNormalRange[p] = 1
		P.LostContactMask = 0
		P.Contact[p] = 1
	}
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		ci := &s.Colonies[i]
		if ci.Planet == -1 {
			continue
		}
		a := int(ci.Owner)
		si := int(s.Planets[ci.Planet].Star)
		for j := i - 1; j >= 0; j-- {
			cj := &s.Colonies[j]
			if cj.Planet == -1 {
				continue
			}
			b := int(cj.Owner)
			sj := int(s.Planets[cj.Planet].Star)
			met := false
			if s.starInExtendedRangeOfPlayer(sj, a) {
				met = true
				if s.starInNormalRangeOfPlayer(sj, a) {
					s.Players[a].InNormalRange[b] = 1
				}
			}
			if s.starInExtendedRangeOfPlayer(si, b) {
				met = true
				if s.starInNormalRangeOfPlayer(si, b) {
					s.Players[b].InNormalRange[a] = 1
				}
			}
			if met {
				s.Players[a].Contact[b] = 1
				s.Players[b].Contact[a] = 1
			}
		}
	}
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		P := &s.Players[p]
		// Only the highest changed player counts; a new contact is marked
		// in the mask and its counter.
		for q := int(s.NumPlayers) - 1; q >= 0; q-- {
			if old[p][q] == P.Contact[q] {
				continue
			}
			if old[p][q] == 0 {
				P.LostContactMask |= 1 << (uint(q) & 7)
				P.LostContactCount[q]++
			}
			s.SomethingHappened[p] = 1
			break
		}
	}
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		for q := int(s.NumPlayers) - 1; q >= 0; q-- {
			if P := &s.Players[p]; P.Contact[q] == 0 {
				P.TradeTreaty[q] = 0
				P.ResearchTreaty[q] = 0
				s.bringHomeSpiesVs(p, q)
				s.bringHomeSpiesVs(q, p)
			}
		}
	}
}

// starInNormalRangeOfPlayer is Star_In_Normal_Range_Of_Player_ (0xff666).
func (s *State) starInNormalRangeOfPlayer(star, p int) bool {
	return s.starInRangeOfPlayer(star, p, s.Players[p].FuelRange)
}

// starInExtendedRangeOfPlayer is Star_In_Extended_Range_Of_Player_
// (0xff68a): half again the fuel range, rounded toward zero.
func (s *State) starInExtendedRangeOfPlayer(star, p int) bool {
	return s.starInRangeOfPlayer(star, p, int16(3*int32(s.Players[p].FuelRange)/2))
}

// bringHomeSpiesVs is Bring_Home_Spies_Vs_ (0x1019f0): p's spies in q's
// empire return home, at most 63 there.
func (s *State) bringHomeSpiesVs(p, q int) {
	n := int32(s.theirSpyNumber(p, q))
	if n == 0 {
		return
	}
	n += int32(s.theirSpyNumber(p, p))
	if n > 0x3f {
		n = 0x3f
	}
	s.setTheirSpyNumber(p, p, int16(n))
	s.setTheirSpyNumber(p, q, 0)
}
