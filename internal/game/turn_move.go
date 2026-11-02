package game

import "github.com/openorion2/openorion2/internal/save"

// Movement and range steps of Next_Turn_Calc_ (SHIPMOVE.C, LOSTSHIP.C,
// COMBFIND.C). Spec: docs/turn/overview.md, phase D.

// moveAllShipsTowardStars is Move_All_Ships_Toward_Stars_ (0xffeea): one
// turn of travel for every ship in transit, ships 0 upwards.
func (g *Game) moveAllShipsTowardStars() {
	s := &g.state
	if g.gameType >= 2 && g.gameType <= 3 {
		for i := int(s.NumStars) - 1; i >= 0; i-- {
			s.Stars[i].SpecialPopup = 0
		}
	}
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		if sh.Status != save.ShipInTransit && sh.Status != save.ShipDeparting {
			sh.AIMission = 0
			continue
		}
		if sh.Location >= save.LocationDeparted {
			sh.Location -= 500
		}
		dest := int(sh.Location - save.LocationTransit)
		D := s.starRecord(dest)
		if dest < 0 || dest >= len(s.Stars) {
			// A transit location below 500 (open problem 0e): the original reads
			// its target from DOS memory below the star array, so X, Y are stale.
			g.stale = append(g.stale, staleSpan{save.BShips, i*0x81 + 0x67, 4})
		}
		tx, ty := D.X, D.Y
		if sh.Speed == -1 {
			sh.X, sh.Y = tx, ty
		} else {
			var neb, stop bool
			nebp := &neb
			if sh.Navigator != 0 {
				nebp = nil
			}
			g.movePlayer1TurnToStar(int(sh.Owner), sh.X, sh.Y, dest, int16(uint8(sh.Speed)), &sh.X, &sh.Y, nebp, &stop)
		}
		sh.ETA--
		if sh.X == tx && sh.Y == ty {
			g.makeShipArriveAtStar(i, dest)
		} else {
			sh.Status = save.ShipInTransit
		}
	}
	g.updateShipIconsInBetweenTurns()
}

// makeShipArriveAtStar is Make_Ship_Arrive_At_Star_ (0xffdda).
func (g *Game) makeShipArriveAtStar(ship, star int) {
	s := &g.state
	sh := &s.Ships[ship]
	st := s.starRecord(star)
	sh.Status = save.ShipAtStar
	sh.Location = int16(star)
	sh.X, sh.Y = st.X, st.Y
	sh.AIMission = 0
	st.IgnoreCombat = 0
	o := uint(sh.Owner)
	if sh.Owner < 8 {
		if st.Special == 11 && st.Visited == 0 {
			s.eventOrionDiscovered(int(sh.Owner))
		}
		if sh.Design.Type == 1 || sh.Design.Type == 4 {
			st.ColonizePromptOff &^= 1 << (o & 7)
		}
		if st.Visited>>o&1 == 0 {
			st.Visited |= 1 << (o & 7)
			st.SpecialPopup |= 1 << (o & 7)
		}
	} else if sh.Owner == 13 {
		sh.Design.BuildDate = 0
	}
	if sh.Owner < 8 {
		s.SomethingHappened[sh.Owner] = 1
		if s.Players[sh.Owner].Objective == save.ObjectiveHuman {
			st.Unknown66 = 0
		}
	}
}

// eventOrionDiscovered is Event_Orion_Discovered_ (0x233b8): event slot 32
// records who reached Orion first.
func (s *State) eventOrionDiscovered(p int) {
	s.EventData[0x121] = 1
	s.EventData[0x120] = byte(p)
}

// updateShipIconsInBetweenTurns is Update_Ship_Icons_In_Between_Turns_
// (0x8a4c4); the star owners it refreshes are saved.
func (g *Game) updateShipIconsInBetweenTurns() {
	s := &g.state
	for i := 0; i < int(s.NumStars); i++ {
		s.Stars[i].Owner = int8(s.starOwner(i, int(s.PlayerNum)))
	}
	g.buildShipIcons()
	g.setShipIconXYs()
}

// loseOutOfRangeShips is Lose_Out_Of_Range_Ships_ (0xed8bf): per player from
// the last, ships beyond fuel range start retreating, then all retreats run.
func (g *Game) loseOutOfRangeShips() {
	s := &g.state
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		var normal, ext [maxStars]bool
		s.computeStarsInRangeOfPlayer(&normal, &ext, p)
		s.playerLoseOutOfRangeShips(&normal, &ext, p)
		g.processRetreatingShips(false)
	}
}

// computeStarsInRangeOfPlayer is Compute_Stars_In_Range_Of_Player_ (0xed7a3):
// stars within the fuel range and within 1.5 times it, from the last star.
func (s *State) computeStarsInRangeOfPlayer(normal, ext *[maxStars]bool, p int) {
	r := int32(s.Players[p].FuelRange)
	e := int16((3*r + 1) / 2)
	for j := int(s.NumStars) - 1; j >= 0; j-- {
		normal[j] = s.starInRangeOfPlayer(j, p, int16(r))
		ext[j] = s.starInRangeOfPlayer(j, p, e)
	}
}

// playerLoseOutOfRangeShips is Player_Lose_Out_Of_Range_Ships_ (0xed82f).
// Its Update_Ship_Icons_In_Between_Turns_ call hangs on a flag never set.
func (s *State) playerLoseOutOfRangeShips(normal, ext *[maxStars]bool, p int) {
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if sh.Status != save.ShipAtStar || int16(sh.Owner) != int16(p) {
			continue
		}
		l := int(sh.Location)
		if !ext[l] || (!normal[l] && sh.Design.Specials[1]>>3&1 == 0) {
			sh.Status = save.ShipRetreating
		}
	}
}

// Layout of Process_Retreating_Ships_' frame from ebp-0xd38: the loss table
// (8 players x 72 stars x 4 bytes) runs into the ship list as in the original.
const (
	retreatList  = 0x900 // ebp-0x438, s16 ship ids
	retreatMsg   = 0xcec // ebp-0x4c, the message being built
	retreatPick  = 0xd10 // ebp-0x28, per owner star for ships at "nowhere"
	retreatFrame = 0xd38
)

// processRetreatingShips is Process_Retreating_Ships_ (0xe6e52), ships from
// the last: ships at "nowhere" (location NUM_STARS) come home through a
// dimensional portal or die; retreating stacks flee to the closest colony or
// die; Antarans at a star leave; some monsters vanish. Then loss messages.
// fromBattles: called by Search_For_Battles_ (else Lose_Out_Of_Range_Ships_).
func (g *Game) processRetreatingShips(fromBattles bool) {
	s := &g.state
	var f [retreatFrame]byte
	for i := 0; i < 8; i++ {
		f[retreatPick+i] = 0xff
	}
	// Owners past 7 reach the locals above the table; the original's values
	// there are not modelled.
	at := func(off int) *byte {
		if off < 0 || off >= len(f) {
			var junk byte
			return &junk
		}
		return &f[off]
	}
	entry := func(owner int8, loc int16) int { return int(owner)*0x120 + int(loc)*4 }
	count := func(e int) {
		n := uint16(*at(e + 1)) | uint16(*at(e + 2))<<8 + 1
		*at(e + 1), *at(e + 2) = byte(n), byte(n>>8)
	}
	listID := func(k int) int16 { return int16(uint16(f[retreatList+2*k]) | uint16(f[retreatList+2*k+1])<<8) }
	setList := func(k int, id int16) { f[retreatList+2*k], f[retreatList+2*k+1] = byte(id), byte(uint16(id)>>8) }

	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if sh.Location == s.NumStars {
			pick := at(retreatPick + int(sh.Owner))
			if *pick == 0xff {
				*pick = maxStars
				n := uint32(0)
				for j := int(s.NumStars) - 1; j >= 0; j-- {
					if s.Stars[j].DimensionalPortal>>(uint(sh.Owner)&31)&1 != 0 {
						n++
						if s.Random.Random(n) == 1 {
							*pick = byte(j)
						}
					}
				}
			}
			if *pick != maxStars {
				st := s.starRecord(int(int8(*pick)))
				sh.Location = int16(int8(*pick))
				sh.X = st.X
				sh.Status = save.ShipAtStar
				sh.Y = st.Y
				continue
			}
			e := entry(sh.Owner, s.NumStars)
			if o := int(sh.Officer); o != -1 {
				if s.Leaders[o].ETA > 0 {
					g.moveOfficerToPool(o)
				} else {
					s.Leaders[o].Location = -1
					sh.Officer = -1
					g.killOfficer(o)
					*at(e + 3)++
				}
			}
			count(e)
			*at(e) = 0
			g.killShip(i)
			continue
		}
		switch {
		case sh.Status == save.ShipRetreating:
			loc, owner := sh.Location, sh.Owner
			kind := int32(-1)
			n := 1
			setList(0, int16(i))
			sh.AIMission = 0
			for j := i - 1; j >= 0; j-- {
				o := &s.Ships[j]
				if o.Status == save.ShipRetreating && o.Location == loc && o.Owner == owner {
					setList(n, int16(j))
					n++
					o.AIMission = 0
				}
			}
			dest := s.closestOtherColonyStar(int(owner), int(loc))
			kill := false
			var info shipMoveInfo
			ids := make([]int16, n+1)
			if dest == -1 {
				kill, kind = true, 2
			} else {
				setList(n, -1)
				for k := range ids {
					ids[k] = listID(k)
				}
				info = g.shipsTryToMoveTo(ids, dest)
				if !info.canMove {
					kill, kind = true, 2
					if info.flux {
						kind = 1
					}
				}
			}
			if kill {
				for k := n - 1; k >= 0; k-- {
					id := int(listID(k))
					e := entry(sh.Owner, sh.Location)
					if o := int(s.Ships[id].Officer); o != -1 {
						if s.Leaders[o].ETA > 0 {
							g.moveOfficerToPool(o)
						} else {
							g.killOfficer(o)
							*at(e + 3)++
						}
					}
					count(e)
					*at(e) = byte(kind)
					g.killShip(id)
				}
				continue
			}
			g.makeShipsMoveTo(ids, dest, &info)
			for k := n - 1; k >= 0; k-- {
				o := &s.Ships[listID(k)]
				o.Status = save.ShipInTransit
				o.Location -= 500
			}
		case sh.Owner == 8 && sh.Status == save.ShipAtStar:
			s.restockAntaranShip(int(sh.Design.Size))
			g.killShip(i)
		case sh.Owner >= 8 && sh.Status == save.ShipAtStar && sh.Design.FTLSpeed != 0:
			switch uint8(sh.Owner) {
			case 10:
				g.killShip(i)
				g.russEventMonsterUpdate(10, -1, -1)
			case 11, 12, 14:
				// cmp eax, 1 then a jump to the owner == 10 test's jne
				// (0xe71fa): on 1 these monsters go the same way.
				if s.Random.Random(2) == 1 {
					g.killShip(i)
					g.russEventMonsterUpdate(int(sh.Owner), -1, -1)
				}
			}
		}
	}

	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		for c := int(s.NumStars); c >= 0; c-- {
			e := p*0x120 + c*4
			if *at(e + 1) == 0 && *at(e + 2) == 0 {
				continue
			}
			g.retreatResidue(f[retreatMsg : retreatMsg+msgSize])
			var m [msgSize]byte
			copy(m[:], f[retreatMsg:])
			m[0], m[2], m[3] = 14, byte(c), *at(e)
			m[4], m[5], m[6] = *at(e + 1), *at(e + 2), *at(e + 3)
			i := g.addMsg(p, m)
			if fromBattles {
				// After the battles the rest is what the last combat routine
				// left there (unmodelled), not Star_In_Range_Of_Player_'s.
				g.markStaleMsg(i, 1, 2)
				g.markStaleMsg(i, 7, msgSize)
			}
		}
	}
	g.updateShipIconsInBetweenTurns()
}

// retreatResidue fills the message bytes Process_Retreating_Ships_ never
// sets: they hold what Compute_Stars_In_Range_Of_Player_ left on the stack.
func (g *Game) retreatResidue(b []byte) {
	r := &g.state.rangeResidue
	le := func(off int, v uint32) {
		for k := 0; k < 4 && off+k < len(b); k++ {
			b[off+k] = byte(v >> (8 * k))
		}
	}
	// Absolute stack address of Star_In_Range_Of_Player_'s frame in the
	// original under DOSBox-X (docs/turn/overview.md, D8).
	ebp := uint32(0x3f3ac8)
	le(0, r.aux1ECX)
	le(4, r.aux1Ret+0x224000)
	le(8, uint32(r.player))
	le(12, uint32(r.rng))
	le(16, ebp)
}

// killOfficer is Kill_Officer_ (0x941c6).
func (g *Game) killOfficer(o int) {
	s := &g.state
	l := &s.Leaders[o]
	if loc := int32(l.Location); loc > -1 {
		if l.Type == 1 {
			if l.Owner > -1 && l.Owner < 8 {
				s.starRecord(int(loc)).Officer[l.Owner] = -1
			}
		} else {
			s.Ships[loc].Officer = -1
		}
	}
	l.Status = -2
	l.ETA = 0
	l.Location = -1
	if l.Type == 1 {
		g.doPlayerColonyCalculations(int(l.Owner))
	}
}

// restockAntaranShip is Restock_Antaran_Ship_ (0x6485f): one ship of the
// size goes back to the Antaran stock (s16 counters at antarans+0x2a).
func (s *State) restockAntaranShip(size int) {
	off := 0x2a + 2*size
	n := int32(int16(uint16(s.Antarans[off])|uint16(s.Antarans[off+1])<<8)) - 1
	if n <= 0 {
		n = 0
	}
	s.Antarans[off], s.Antarans[off+1] = byte(n), byte(n>>8)
}

// eventMonsterUpdate is Event_Monster_Update_ (0x2332c), reached through
// Russ_Event_Monster_Update_ (0xf9711) outside network games.
func (s *State) eventMonsterUpdate(monster int, loc, star int16) {
	if monster < 10 || monster > 14 {
		return
	}
	e := (monster - 10 + 0x13) * 9
	d := s.EventData[e : e+9]
	d[0] = byte(star)
	if loc != -1 {
		d[3], d[4] = byte(loc), byte(uint16(loc)>>8)
	} else {
		d[5], d[6] = byte(star), byte(uint16(star)>>8)
		if star == loc {
			d[2] = 0x14
		}
	}
	d[1] = 4
}

// computeBlockades is Compute_Blockades_ (0xe5097): combat ships at a star
// blockade the colonies of players their owner is at war with (monsters and
// Antarans everyone's); each blockaded player resents the blockaders.
func (g *Game) computeBlockades() {
	s := &g.state
	for i := int(s.NumStars) - 1; i >= 0; i-- {
		s.Stars[i].Blockaded = 0
		s.Stars[i].BlockadedBy = [8]uint8{}
	}
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if sh.Status != save.ShipAtStar || sh.Location >= maxStars || sh.Design.Type != 0 {
			continue
		}
		st := s.starRecord(int(sh.Location))
		if sh.Owner >= 8 {
			// Assigned, not or-ed: the last monster's view wins.
			st.Blockaded = st.ColonyOwners
			continue
		}
		o := int(sh.Owner)
		for q := int(s.NumPlayers) - 1; q >= 0; q-- {
			if q == o || int32(st.ColonyOwners)>>q&1 == 0 {
				continue
			}
			if t := s.Players[o].TreatyStatus[q]; t >= 4 && t <= 6 {
				st.Blockaded |= 1 << q
				st.BlockadedBy[q] |= 1 << (uint(o) & 7)
			}
		}
	}
	for i := int(s.NumStars) - 1; i >= 0; i-- {
		st := &s.Stars[i]
		if st.Blockaded == 0 {
			continue
		}
		for p := int(s.NumPlayers) - 1; p >= 0; p-- {
			if int32(st.Blockaded)>>p&1 == 0 {
				continue
			}
			for q := int(s.NumPlayers) - 1; q >= 0; q-- {
				if int32(st.BlockadedBy[p])>>q&1 != 0 && int32(st.ColonyOwners)>>q&1 == 0 {
					d := -int16(s.Random.Random(5))
					g.changeRelations(d, q, p, 7, int16(i), 0)
				}
			}
		}
	}
}

// Packed settler fields (player.Settlers): source colony, target planet,
// race, ETA and job (Settle_Pop_, 0xff116).
func settlerPlanet(v uint32) int { return int(v >> 8 & 0xff) }
func settlerRace(v uint32) int   { return int(v >> 16 & 0xf) }
func settlerETA(v uint32) uint32 { return v >> 20 & 0xf }

// moveSettlers is Move_Settlers_ (0xff212): settlers in transit come one turn
// closer; arrivals join their colony unless blockaded, anomalous or full,
// and each target planet reports to its player.
func (g *Game) moveSettlers() {
	s := &g.state
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		P := &s.Players[p]
		var arrived, failed [maxPlanets]uint8
		any := false
		for i := 0; i < int(P.SettlersCount); {
			v := &P.Settlers[i]
			// The low byte, the source colony, is overwritten on every turn.
			*v = *v&^0xf000ff | (settlerETA(*v)-1)&0xf<<20 | 0xfa
			if settlerETA(*v) != 0 {
				i++
				continue
			}
			pl := settlerPlanet(*v)
			arrived[pl]++
			star := int(s.Planets[pl].Star)
			any = true
			if int32(s.Stars[star].Blockaded)>>p&1 == 0 && !s.eventCheckSpaceAnomalyAtStar(int16(star)) {
				if ci := s.Planets[pl].Colony; ci != -1 {
					c := &s.Colonies[ci]
					if c.IsOutpost == 0 && int(c.Owner) == p && !g.addSettlerToColony(*v, c) {
						failed[pl]++
					}
				}
			}
			P.SettlersCount--
			copy(P.Settlers[i:], P.Settlers[i+1:int(P.SettlersCount)+1])
		}
		if !any {
			continue
		}
		for pl := maxPlanets - 1; pl >= 0; pl-- {
			if arrived[pl] == 0 {
				continue
			}
			var m [msgSize]byte
			m[2] = byte(pl)
			m[3] = 5
			m[4], m[5] = arrived[pl], failed[pl]
			m[6] = byte(int32(s.Stars[s.Planets[pl].Star].Blockaded) >> p & 1)
			g.addMsg(p, m)
		}
	}
}

// addSettlerToColony is Add_Settler_To_Colony_ (0xff015): whether the settler
// found room. A farmer beyond the colony's farmland is put to other work.
func (g *Game) addSettlerToColony(v uint32, c *save.Colony) bool {
	race := settlerRace(v)
	if !g.roomForAnother(c, race, false) {
		return false
	}
	if v&0x3000000 == 0 {
		farmers := 0
		spare := -1
		for k := int(c.PopCount) - 1; k >= 0; k-- {
			if uint16(c.Colonists[k])&0x180 != 0 {
				continue
			}
			farmers++
			if spare == -1 && c.Colonists[k]&0xf < 8 {
				spare = k
			}
		}
		if int(c.CanFarm) <= farmers {
			if v&0xf0000 < 0x80000 {
				v = v&^0x3000000 | 0x2000000
			} else if spare != -1 {
				c.Colonists[spare] = c.Colonists[spare]&^0x180 | 0x100
			}
		}
	}
	n := &c.Colonists[c.PopCount]
	r := uint32(race)
	*n = *n&^0xf | r
	*n = *n&^0x70 | (r&7)<<4
	*n = *n&^0x180 | (v>>24&3)<<7
	*n = *n | 0x200
	*n &^= 0x400
	c.PopCount++
	return true
}

// roomForAnother is Room_For_Another_ (0xfed77): whether the colony holds
// fewer pops (counting those of races with at most this race's limit, and
// with settlers those on their way) than the race's limit.
func (g *Game) roomForAnother(c *save.Colony, race int, settlers bool) bool {
	s := &g.state
	limit := int32(int16(g.colonyRacePopLimit(c, race)))
	n := int32(0)
	for k := int(c.PopCount) - 1; k >= 0; k-- {
		if int32(int16(g.colonyRacePopLimit(c, int(c.Colonists[k]&0xf)))) <= limit {
			n++
		}
	}
	if settlers {
		P := &s.Players[c.Owner]
		ci := int16(-1)
		for i := range s.Colonies {
			if &s.Colonies[i] == c {
				ci = int16(i)
			}
		}
		for k := int(P.SettlersCount) - 1; k >= 0; k-- {
			v := P.Settlers[k]
			if s.Planets[settlerPlanet(v)].Colony == ci &&
				int32(int16(g.colonyRacePopLimit(c, settlerRace(v)))) <= limit {
				n++
			}
		}
	}
	return n < limit
}
