package game

// Colony slot reservation of Next_Turn_Calc_ (docs/turn/overview.md, D16).

// colonyBaseRefund is the s32 at 0x17ec16; trashing a colony base pays
// back half of it.
const colonyBaseRefund = 0x17ec16

// allocateNewColonySlots is Allocate_New_Colony_Slots_ (0xe64f4): the free
// colony slots are reserved round-robin from player stardate % NUM_PLAYERS,
// each player up to the empty planets at the stars it claims.
func (g *Game) allocateNewColonySlots() {
	s := &g.state
	g.findColonizePlayers()
	// Index 8 ("nobody yet") lands in the next stack array, which is zeroed
	// before it is read.
	var want [9]int16
	for i := int(s.NumStars) - 1; i >= 0; i-- {
		if cp := s.Stars[i].ColonizePlayer; cp != -1 {
			want[cp] += s.starNUncolonizedPlanets(i)
		}
	}
	var list [maxColonies]uint8
	n := 0
	for i := 0; i < maxColonies; i++ {
		c := &s.Colonies[i]
		c.ReservedFor = -1
		if i >= int(s.NumColonies) || c.Planet == -1 {
			list[n] = uint8(i)
			n++
		}
	}
	var count [8]int16
	np := int16(s.NumPlayers)
	p := int16(int32(s.Stardate) % int32(np))
	k, miss := 0, int16(0)
	for miss < np {
		if count[p] < want[p] {
			// The original does not stop at the end of the list and would read
			// stale stack; the port drops such reservations.
			if k < n {
				s.Colonies[list[k]].ReservedFor = int8(p)
			}
			k++
			miss = 0
			count[p]++
		} else {
			miss++
		}
		if p++; p == np {
			p = 0
		}
	}
}

// findColonizePlayers is Find_Colonize_Players_ (0xe6170): per star the one
// player that may colonise there next, or -1; colony bases with nothing
// left to settle are scrapped for half their cost.
func (g *Game) findColonizePlayers() {
	s := &g.state
	var outpost, colony [maxStars]uint8
	for i := int(s.NumStars) - 1; i >= 0; i-- {
		s.Stars[i].ColonizePlayer = 8
	}
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		t := sh.Design.Type
		if sh.Status != 0 || (t != 1 && t != 4) || sh.Location >= 0x48 {
			continue
		}
		st := &s.Stars[sh.Location]
		switch st.ColonizePlayer {
		case 8:
			st.ColonizePlayer = sh.Owner
			if t == 4 {
				outpost[sh.Location] |= shl8(sh.Owner)
			} else {
				colony[sh.Location] |= shl8(sh.Owner)
			}
		case sh.Owner:
		default:
			st.ColonizePlayer = -1
		}
	}
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if c.Owner == -1 || c.Buildings[0xb] == 0 {
			continue
		}
		star := s.Planets[c.Planet].Star
		st := &s.Stars[star]
		switch st.ColonizePlayer {
		case 8:
			st.ColonizePlayer = c.Owner
			colony[star] |= shl8(c.Owner)
		case c.Owner:
		default:
			st.ColonizePlayer = -1
		}
	}
	for i := int(s.NumStars) - 1; i >= 0; i-- {
		st := &s.Stars[i]
		cp := st.ColonizePlayer
		if cp == 8 {
			st.ColonizePlayer = -1
			continue
		}
		if sar8(colony[i], cp) && s.starNColonizablePlanets(i, int(cp)) > 0 {
			continue
		}
		if sar8(outpost[i], cp) && s.starNOutpostablePlanets(i) > 0 {
			continue
		}
		st.ColonizePlayer = -1
	}
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if sh.Status != 0 || sh.Design.Type != 0 || sh.Location >= 0x48 {
			continue
		}
		st := &s.Stars[sh.Location]
		cp := st.ColonizePlayer
		if uint32(int32(cp)) >= 8 || cp == sh.Owner {
			continue
		}
		if sh.Owner >= 8 {
			st.ColonizePlayer = -1
		} else if t := s.Players[cp].TreatyStatus[sh.Owner]; t >= 4 && t <= 6 {
			st.ColonizePlayer = -1
		}
	}
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if c.Owner == -1 || c.Buildings[0xb] == 0 {
			continue
		}
		star := int(s.Planets[c.Planet].Star)
		if s.Stars[star].ColonizePlayer == c.Owner && s.starNColonizablePlanets(star, int(c.Owner)) != 0 {
			continue
		}
		refund := int32(g.exe.u32(colonyBaseRefund)) / 2
		c.Buildings[0xb] = 0
		s.Players[c.Owner].BC += refund
		// Bytes 1 (the priority) and 8..17 are the caller's stack residue in
		// the original; the port sends zeros.
		var m [msgSize]byte
		m[2] = uint8(c.Planet)
		m[4] = 0xb
		m[6], m[7] = uint8(refund), uint8(refund>>8)
		g.addMsg(int(c.Owner), m)
	}
}

// shl8 is 1 << p in a byte register: bits past 7 are lost.
func shl8(p int8) uint8 { return uint8(1) << (uint8(p) & 31) }

// sar8 tests bit p of a byte mask as `sar eax, cl`: counts are taken mod 32.
func sar8(m uint8, p int8) bool { return uint32(m)>>(uint8(p)&31)&1 != 0 }

// starNUncolonizedPlanets is Star_N_Uncolonized_Planets_ (0xe608b).
func (s *State) starNUncolonizedPlanets(star int) int16 {
	return s.starNOutpostablePlanets(star)
}

// starNOutpostablePlanets is Star_N_Outpostable_Planets_For_Player_
// (0xe6132): planets without a colony, whoever asks.
func (s *State) starNOutpostablePlanets(star int) int16 {
	n := int16(0)
	for i := 4; i >= 0; i-- {
		if p := s.Stars[star].Planets[i]; p != -1 && s.Planets[p].Colony == -1 {
			n++
		}
	}
	return n
}

// starNColonizablePlanets is Star_N_Colonizable_Planets_For_Player_
// (0xe60c8): habitable planets that are free or p's outposts.
func (s *State) starNColonizablePlanets(star, p int) int16 {
	n := int16(0)
	for i := 4; i >= 0; i-- {
		pl := s.Stars[star].Planets[i]
		if pl == -1 || s.Planets[pl].Type != 3 {
			continue
		}
		ci := s.Planets[pl].Colony
		if ci == -1 {
			n++
			continue
		}
		if c := &s.Colonies[ci]; c.IsOutpost != 0 && int16(c.Owner) == int16(p) {
			n++
		}
	}
	return n
}
