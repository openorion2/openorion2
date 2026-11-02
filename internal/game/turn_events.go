package game

import (
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// Random events live in _event_data (0x19aba4): 36 records of 9 bytes.
const (
	eventCount          = 0x24
	eventSize           = 9
	evVictim            = 0 // int8 player
	evState             = 1 // 0 off, 1 new, 2/4/6 active, 3 …
	evByte              = 2
	evWord1             = 3 // colony, star, ship or player, by event
	evWord2             = 5
	evWord3             = 7
	eventTypes          = 0x1d     // Determine_Event_ picks among the first 29
	eventGoodTable      = 0x180e84 // u8 by event: 1 for a good event
	eventHyperspaceFlux = 9
	eventPlanetWarning  = 30 // news when one player holds a share of the stars
	eventGNNSpecial     = 31
)

// bldCapitol is the capitol's slot in Colony.Buildings (colony +0x13f).
const bldCapitol = 9

func (s *State) ev(t int) []byte { return s.EventData[t*eventSize : (t+1)*eventSize] }

func (s *State) evWord(t, off int) int16 {
	return int16(binary.LittleEndian.Uint16(s.EventData[t*eventSize+off:]))
}

func (s *State) setEvWord(t, off int, v int16) {
	binary.LittleEndian.PutUint16(s.EventData[t*eventSize+off:], uint16(v))
}

func (g *Game) eventGood(t int) uint8 { return g.exeU8(uint32(eventGoodTable + t)) }

// evActive tells whether event t is running (state 2, 4 or 6).
func (s *State) evActive(t int) bool {
	st := s.EventData[t*eventSize+evState]
	return st == 2 || st == 4 || st == 6
}

// updateLuckyPlayersEvents is Update_Lucky_Players_Events_ (0x245c4).
func (s *State) updateLuckyPlayersEvents() {
	for p := 0; p < int(s.NumPlayers); p++ {
		pl := &s.Players[p]
		if pl.Eliminated == 0 && pl.Picks[save.PickLucky] == 1 {
			pl.LuckyCounter++
		}
	}
}

// determineLuckyPlayersEvents is Determine_Lucky_Players_Events_ (0x24511):
// the first living lucky player whose counter wins a Random(1000), or -1.
func (g *Game) determineLuckyPlayersEvents() int16 {
	s := &g.state
	div := int32(6)
	switch {
	case !g.settings.RandomEvents() && !g.settings.AntaransAttack():
		div = 10
	case !g.settings.RandomEvents():
		div = 12
	case g.settings.AntaransAttack():
		div = 8
	}
	res := int16(-1)
	for p := int16(0); p < s.NumPlayers && res == -1; p++ {
		pl := &s.Players[p]
		if pl.Eliminated != 0 || pl.Picks[save.PickLucky] != 1 {
			continue
		}
		if int32(s.Random.Random(1000)) <= int32(pl.LuckyCounter)/div {
			res = p
			pl.LuckyCounter = 0
		}
	}
	return res
}

// playerHasBadEvent is Player_Has_Bad_Event_ (0x230b6).
func (g *Game) playerHasBadEvent(p int16) bool {
	s := &g.state
	for t := 0; t < eventCount; t++ {
		e := s.ev(t)
		if e[evState] != 0 && e[evState] != 3 && int16(int8(e[evVictim])) == p && g.eventGood(t) == 0 {
			return true
		}
	}
	return false
}

// getEventVictim is Get_Event_Victim_ (0x22d57): a weighted pick that favours
// populous players for bad events and small ones for good events; the
// extreme player is left out, and a lucky victim of a bad event is spared.
func (g *Game) getEventVictim(good uint8) int16 {
	s := &g.state
	var weights [8]uint16
	players := [8]int16{-1, -1, -1, -1, -1, -1, -1, -1}
	n := 0
	for p := int16(0); p < s.NumPlayers; p++ {
		if s.Players[p].Eliminated != 0 || good == 0 && g.playerHasBadEvent(p) {
			continue
		}
		players[n], weights[n] = p, uint16(s.Players[p].Population)
		n++
	}
	// The original leaves the extreme's index unset if no one beats the start.
	ext := 0
	if good == 0 {
		min := int16(32000)
		for k := 0; k < n; k++ {
			if pop := s.Players[players[k]].Population; min > pop {
				ext, min = k, pop
			}
		}
		n--
		for k := ext; k < n; k++ {
			players[k], weights[k] = players[k+1], weights[k+1]
		}
		for k := 0; k < n; k++ {
			d := int32(weights[k]) - int32(min)
			weights[k] = uint16(d * d)
		}
	} else {
		max := int16(0)
		for k := 0; k < n; k++ {
			if pop := s.Players[players[k]].Population; max < pop {
				ext, max = k, pop
			}
		}
		n--
		for k := ext; k < n; k++ {
			players[k], weights[k] = players[k+1], weights[k+1]
		}
		for k := 0; k < n; k++ {
			d := int32(max) - int32(weights[k])
			if d < 0 {
				d = -d
			}
			weights[k] = uint16(d * d)
		}
	}
	v := players[getWeightedChoice(&s.Random, weights[:max(n, 0)])]
	if good == 0 && v != -1 && s.Players[v].Picks[save.PickLucky] == 1 {
		return -1
	}
	return v
}

// pickRandomColony is Pick_Random_Colony_No_Outpost_ (0x23d44) and, with
// noCapitol, Pick_Random_Colony_No_Capitol_ (0x23da0): reservoir sampling.
func (g *Game) pickRandomColony(p int16, noCapitol bool) int16 {
	s := &g.state
	res, n := int16(-1), uint32(0)
	for i := int16(0); i < s.NumColonies; i++ {
		c := &s.Colonies[i]
		if int16(c.Owner) != p || noCapitol && c.Buildings[bldCapitol] != 0 || c.IsOutpost != 0 {
			continue
		}
		n++
		if s.Random.Random(n) == 1 {
			res = i
		}
	}
	return res
}

// pickRandomPlayerInContact is Pick_Random_Player_In_Contact_ (0x23b7d).
func (g *Game) pickRandomPlayerInContact(p int16) int16 {
	s := &g.state
	res, n := int16(-1), uint32(0)
	for o := int16(0); o < s.NumPlayers; o++ {
		if s.Players[o].Eliminated != 0 || !testBit(s.Players[p].Contact[:], int(o)) || o == p {
			continue
		}
		n++
		if s.Random.Random(n) == 1 {
			res = o
		}
	}
	return res
}

// pickRandomShip is Pick_Random_Ship_ (0x23ced): one of p's ships at a star
// or in flight.
func (g *Game) pickRandomShip(p int16) int16 {
	s := &g.state
	res, n := int16(-1), uint32(0)
	for i := int16(0); i < s.NumShips; i++ {
		sh := &s.Ships[i]
		if int16(sh.Owner) != p || sh.Status != 0 && sh.Status != 1 {
			continue
		}
		n++
		if s.Random.Random(n) == 1 {
			res = i
		}
	}
	return res
}

// pickRandomStar is Pick_Random_Star_ (0x23bec): a star owned by p, weighted
// by its colonies (not outposts) of p.
func (g *Game) pickRandomStar(p int16) int16 {
	s := &g.state
	res, n := int16(-1), uint16(0)
	for st := int16(0); st < s.NumStars; st++ {
		if int16(s.Stars[st].Owner) != p {
			continue
		}
		for _, pi := range s.Stars[st].Planets {
			if pi == -1 {
				continue
			}
			ci := s.Planets[pi].Colony
			if ci == -1 || s.Colonies[ci].IsOutpost != 0 || int16(s.Colonies[ci].Owner) != p {
				continue
			}
			// After 65535 colonies the counter wraps and picks without a draw.
			if n++; n == 0 || s.Random.Random(uint32(int32(int16(n)))) == 1 {
				res = st
			}
		}
	}
	return res
}

// pickCompletelyRandomStar is Pick_Completely_Random_Star_ (0x23a5f): a star
// with an owner and a colony that is neither an outpost nor a capitol. A star
// with owners but no such colony is tried again and again, never redrawn.
func (g *Game) pickCompletelyRandomStar() int16 {
	s := &g.state
	st := int16(s.Random.Random(uint32(int32(s.NumStars))) - 1)
	found := false
	tries := int16(0)
	for !found && tries < 1000 {
		if s.Stars[st].ColonyOwners > 0 {
			for _, pi := range s.Stars[st].Planets {
				if pi == -1 {
					continue
				}
				ci := s.Planets[pi].Colony
				if ci == -1 {
					continue
				}
				if c := &s.Colonies[ci]; c.IsOutpost == 0 && c.Buildings[bldCapitol] == 0 {
					found = true
				}
			}
		} else {
			st = int16(s.Random.Random(uint32(int32(s.NumStars))) - 1)
		}
		tries++
	}
	if tries > 990 {
		return -1
	}
	return st
}

// colonyResearchPoints is Colony_Research_Points_ (0x23b64).
func (s *State) colonyResearchPoints(ci int16) int16 { return s.Colonies[ci].Output[research] }

// colonyHasEvent is Colony_Has_Event_ (0x23e60): the running event that
// concerns the colony, or -1.
func (s *State) colonyHasEvent(ci int16) int16 {
	for _, t := range []int{2, 16, 17} {
		if s.evActive(t) && ci == s.evWord(t, evWord1) {
			return int16(t)
		}
	}
	star := int16(s.Planets[s.Colonies[ci].Planet].Star)
	for _, t := range []int{25, 14, 24} {
		if s.evActive(t) && star == s.evWord(t, evWord1) {
			return int16(t)
		}
	}
	return -1
}

// starHasEvent is Star_Has_Event_ (0x242fc).
func (s *State) starHasEvent(star int16) int16 {
	for _, t := range []int{2, 16, 17} {
		if s.evActive(t) {
			ci := s.evWord(t, evWord1)
			if int16(s.Planets[s.Colonies[ci].Planet].Star) == star {
				return int16(t)
			}
		}
	}
	for _, t := range []int{25, 14, 24} {
		if s.evActive(t) && star == s.evWord(t, evWord1) {
			return int16(t)
		}
	}
	return -1
}

// eventPirateActivityStrength is Event_Pirate_Activity_Strength_ (0x2448f):
// the command points of the players with a colony at the star, scaled by
// difficulty (level 3 is not scaled, level 4 is 4/5); under 5 is none.
func (g *Game) eventPirateActivityStrength(star int16) int16 {
	s := &g.state
	var sum int16
	for p := 0; p < int(s.NumPlayers); p++ {
		if s.Stars[star].ColonyOwners>>uint(p)&1 != 0 {
			sum += s.Players[p].CommandUsed
		}
	}
	v := int32(sum)
	switch g.settings.Difficulty() {
	case 0:
		v = v / 5
	case 1:
		v = v * 2 / 5
	case 2:
		v = v * 3 / 5
	case 4:
		v = v * 4 / 5
	}
	if int16(v) < 5 {
		return 0
	}
	return int16(v)
}

// eventColony is the shape of Get_Event_Climate_Change_Colony_ (0x2310c) and
// its siblings: up to 200 random picks of a colony that passes ok, then the
// last such colony in a scan (the Mineral ones have no scan).
func (g *Game) eventColony(p int16, noCapitol, scan bool, ok func(*save.Planet) bool) int16 {
	s := &g.state
	res := int16(-1)
	for tries := int16(0); res == -1 && tries < 200; tries++ {
		ci := g.pickRandomColony(p, noCapitol)
		if ci != -1 && ok(&s.Planets[s.Colonies[ci].Planet]) {
			res = ci
		}
	}
	if res != -1 || !scan {
		return res
	}
	for i := int16(0); i < s.NumColonies && res == -1; i++ {
		c := &s.Colonies[i]
		if int16(c.Owner) == p && ok(&s.Planets[c.Planet]) {
			res = i
		}
	}
	return res
}

// getDerelictApplications is Get_Derelict_Applications_ (0x58853): a beam
// weapon a little better than the best anyone has, and the best shield
// anyone has, each if p does not know it yet.
func (g *Game) getDerelictApplications(p int16) []int16 {
	s := &g.state
	bestShield := int16(0)
	beam := 3
	wv := func(w int) int16 { return g.exeI16(rowAddr(weaponTable, weaponStride, w, weaponStrength)) }
	for d := 0; d < int(s.NumPlayers); d++ {
		if sh := int16(g.bestShipShield(d)); sh > bestShield {
			bestShield = sh
		}
		if w := g.bestWeapon(d, 3, 0); wv(w) > wv(beam) {
			beam = w
		}
	}
	appField := func(app int16) int16 { return g.exeI16(uint32(techAppTable + int(app)*techAppRow + techAppField)) }
	beamApp := g.exeI16(rowAddr(weaponTable, weaponStride, beam, weaponApp))
	shieldApp := g.exeI16(rowAddr(shieldTable, shieldStride, int(bestShield), shieldApp))
	if appField(beamApp) == 0x4a {
		beam = 10
	}
	var apps []int16
	best, pick := int16(200), 0
	for k := 1; k < 0x28; k++ {
		if g.exeU8(rowAddr(weaponTable, weaponStride, k, weaponType)) != 0 {
			continue
		}
		app := g.exeI16(rowAddr(weaponTable, weaponStride, k, weaponApp))
		if appField(app) == 0x4a {
			continue
		}
		d := wv(k) - wv(beam)
		if d < 0 || d > best || d > 50 || s.Players[p].AppStatus[app] == 3 {
			continue
		}
		pick = k
		if k == beam {
			best = 1
		} else {
			best = d
		}
	}
	if best < 200 {
		apps = append(apps, g.exeI16(rowAddr(weaponTable, weaponStride, pick, weaponApp)))
	}
	if s.Players[p].AppStatus[shieldApp] != 3 {
		apps = append(apps, shieldApp)
	}
	return apps
}

// doWormholeEvent is Do_Wormhole_Event_ (0x100519): one of p's fleets in
// flight, chosen at random, arrives at its destination at once. It reports
// whether there was one, how many ships moved and where.
func (g *Game) doWormholeEvent(p int16) (moved bool, count int32, star int32) {
	s := &g.state
	chosen, n := -1, uint32(0)
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if int16(sh.Owner) == p && sh.Status == 1 && sh.ETA != 1 {
			n++
			if s.Random.Random(n) == 1 {
				chosen = i
			}
		}
	}
	if chosen != -1 {
		c := s.Ships[chosen]
		moved = true
		star = int32(c.Location) - 500
		for i := int(s.NumShips) - 1; i >= 0; i-- {
			sh := &s.Ships[i]
			if int16(sh.Owner) == p && sh.Status == 1 && sh.X == c.X && sh.Y == c.Y &&
				sh.Location == c.Location && sh.Speed == c.Speed {
				g.makeShipArriveAtStar(i, int(star))
				count++
			}
		}
	}
	g.updateShipIconsInBetweenTurns()
	return moved, count, star
}

// eventCheckPlanetWarning is Event_Check_Planet_Warning_ (0x23563): GNN news
// when one player alone holds stars past the next quarter of half the galaxy.
func (g *Game) eventCheckPlanetWarning() {
	s := &g.state
	const t = eventPlanetWarning
	level := s.evWord(t, evWord2)
	if level > 2 {
		return
	}
	half := int16(int32(s.NumStars) / 2)
	var counts [8]int16
	for st := int16(0); st < s.NumStars; st++ {
		p := -1
		switch s.Stars[st].ColonyOwners {
		case 1:
			p = 0
		case 2:
			p = 1
		case 4:
			p = 2
		case 8:
			p = 3
		case 0x10:
			p = 4
		case 0x20:
			p = 5
		case 0x40:
			p = 6
		case 0x80:
			p = 7
		}
		if p == -1 {
			continue
		}
		has := false
		for _, pi := range s.Stars[st].Planets {
			if pi == -1 {
				continue
			}
			if ci := s.Planets[pi].Colony; ci != -1 && s.Colonies[ci].IsOutpost == 0 {
				has = true
			}
		}
		if has {
			counts[p]++
		}
	}
	// The original leaves the leader unset when every count is 0.
	best, leader := int16(0), 0
	for p := 0; p < int(s.NumPlayers); p++ {
		if best < counts[p] {
			leader, best = p, counts[p]
		}
	}
	if int32(best) < (int32(level)+2)*int32(half)/4 {
		if level != 2 || int32(best) < int32(half)-2 || best >= half || s.CouncilFlag != 0 {
			return
		}
	}
	e := s.ev(t)
	e[evState] = 6
	s.setEvWord(t, evWord2, level+1)
	s.setEvWord(t, evWord1, best)
	e[evVictim] = byte(leader)
}

// determineEvent is Determine_Event_ (0x2230a): the turn's random event.
func (g *Game) determineEvent() {
	s := &g.state
	turn := s.deltaStardate()
	g.state.updateLuckyPlayersEvents()
	lucky := g.determineLuckyPlayersEvents()
	happens := lucky != -1 && turn >= 50
	if g.settings[0x02] == 1 && !happens {
		chance := int32(0)
		if int32(s.Stardate) >= int32(s.LastEventYear) && turn >= 50 {
			chance = max(int32(s.Stardate)-int32(s.LastEventYear), 0)
		}
		if s.EventDelayCount < 5 {
			s.EventDelayCount++
			chance = 0
		}
		switch g.settings.Difficulty() {
		case 0:
			chance /= 2
		case 1:
			chance = chance * 2 / 3
		case 2:
			chance = chance * 3 / 4
		case 3:
			chance = chance * 4 / 5
		case 4:
			chance = chance * 5 / 6
		}
		if int32(s.Random.Random(0x200)) <= chance {
			happens = true
		}
	}
	if !happens {
		return
	}
	t := -1
	victim := int16(-1)
	var apps []int16
	var other, ship, ci int16
	var wormCount, wormStar int32
	for try := 0; try < 5 && t == -1; try++ {
		t = int(int16(s.Random.Random(eventTypes) - 1))
		if g.settings.Difficulty() == 0 && g.eventGood(t) == 0 {
			t = -1
			continue
		}
		if s.ev(t)[evState] != 0 {
			t = -1
		} else {
			victim = -1
			if lucky != -1 {
				if g.eventGood(t) != 1 {
					t = -1
				} else {
					victim = lucky
				}
			} else if t != 9 && t != 0x18 {
				if victim = g.getEventVictim(g.eventGood(t)); victim == -1 {
					t = -1
				}
			}
		}
		if turn < 200 && (t == 2 || t == 0x18) {
			t = -1
		}
		if t == 2 && !g.hyperspaceFlux() {
			t = -1
		}
		for _, c := range []struct {
			typ   int
			turns int32
		}{{0x13, 100}, {0x16, 150}, {0x14, 200}, {0x17, 250}, {0x15, 300}} {
			if turn < c.turns && t == c.typ && !g.hyperspaceFlux() {
				t = -1
			}
		}
		if t == 0 {
			if apps = g.getDerelictApplications(victim); len(apps) < 1 {
				t = -1
			}
		}
		if t == 0xf && s.Players[victim].BC < 100 {
			t = -1
		}
		if t == 1 {
			if ci = g.eventColony(victim, false, true, func(p *save.Planet) bool { return p.Climate < 8 }); ci == -1 {
				t = -1
			}
		}
		if t == 4 || t == 5 {
			if other = g.pickRandomPlayerInContact(victim); other == -1 || s.Players[victim].TreatyStatus[other] >= 4 {
				t = -1
			}
		}
		if t == 8 {
			if ship = g.pickRandomShip(victim); ship == -1 {
				t = -1
			}
		}
		if t == 0xa {
			if s.Players[victim].Picks[save.PickTolerant] == 1 {
				t = -1
			} else if ci = g.eventColony(victim, true, true, func(p *save.Planet) bool { return p.Climate > 1 }); ci == -1 {
				t = -1
			}
		}
		if t == 0xb {
			if ci = g.eventColony(victim, true, false, func(p *save.Planet) bool { return p.Minerals > 3 }); ci == -1 {
				t = -1
			}
		}
		if t == 0xc {
			if ci = g.eventColony(victim, false, false, func(p *save.Planet) bool { return p.Minerals < 4 }); ci == -1 {
				t = -1
			}
		}
		if t == 0xe && s.Players[victim].FreightersTotal < 4 {
			t = -1
		}
		if t == 0xd {
			t = -1
		}
		if t == 0x10 {
			if ci = g.pickRandomColony(victim, true); ci == -1 {
				t = -1
			} else {
				if s.ev(17)[evState] == 2 && ci == s.evWord(17, evWord1) {
					t = -1
				}
				if s.colonyHasEvent(ci) != -1 {
					t = -1
				}
			}
		}
		if t == 0x11 {
			if ci = g.pickRandomColony(victim, false); ci == -1 {
				t = -1
			} else {
				if s.ev(16)[evState] == 2 && ci == s.evWord(16, evWord1) {
					t = -1
				}
				if s.colonyHasEvent(ci) != -1 {
					t = -1
				}
				c := &s.Colonies[ci]
				if g.planetMaxPopulationForPlayer(int(c.Planet), int(victim)) == int16(c.PopCount) {
					t = -1
				}
			}
		}
		if t == 0x1c {
			var moved bool
			if moved, wormCount, wormStar = g.doWormholeEvent(victim); !moved {
				t = -1
			}
		}
		if t == 0x1a || t == 0x1b {
			if ship = g.pickRandomShip(victim); ship == -1 {
				t = -1
			}
		}
		if t == -1 {
			continue
		}
		g.startEvent(t, victim, apps, other, ship, ci, wormCount, wormStar)
	}
	g.eventCheckPlanetWarning()
	if s.Random.Random(40) == 1 && turn > 50 && s.CouncilFlag == 0 {
		s.ev(eventGNNSpecial)[evState] = 6
		s.setEvWord(eventGNNSpecial, evWord2, int16(s.Random.Random(4)-1))
	}
}

// startEvent is the tail of Determine_Event_: record event t and fill in its
// parameters; some types cancel it again (state 0, no victim).
func (g *Game) startEvent(t int, victim int16, apps []int16, other, ship, ci int16, wormCount, wormStar int32) {
	s := &g.state
	e := s.ev(t)
	e[evState] = 1
	e[evByte] = 0xff
	s.setEvWord(t, evWord1, -1)
	s.LastEventYear = s.Stardate
	s.setEvWord(t, evWord2, -1)
	s.setEvWord(t, evWord3, -1)
	s.EventDelayCount = 0
	e[evVictim] = byte(victim)
	cancel := func() { e[evState] = 0; e[evVictim] = 0xff }
	diff := int32(g.settings.Difficulty())
	switch t {
	case 0:
		s.setEvWord(t, evWord1, apps[0])
		s.setEvWord(t, evWord2, 0)
		if len(apps) > 1 {
			s.setEvWord(t, evWord2, apps[1])
		}
	case 1:
		s.setEvWord(t, evWord1, s.Colonies[ci].Planet)
	case 2:
		c := g.pickRandomColony(victim, true)
		s.setEvWord(t, evWord1, c)
		if c == -1 || s.colonyHasEvent(c) != -1 {
			cancel()
			break
		}
		v := int16((int32(s.Random.Random(5)) + 10 + diff) * 10)
		s.setEvWord(t, evWord2, v)
		s.setEvWord(t, evWord3, v)
		e[evByte] = byte(s.Random.Random(5)) + 10 - byte(diff)
	case 3:
		if s.Players[victim].ResearchPoints < 10 {
			cancel()
		}
	case 4, 5:
		s.setEvWord(t, evWord1, other)
	case 6:
		s.setEvWord(t, evWord1, int16(s.deltaStardate()/20*100+100))
	case 7:
		c := g.pickRandomColony(victim, true)
		if s.setEvWord(t, evWord1, c); c == -1 {
			cancel()
		}
	case 8:
		s.setEvWord(t, evWord1, ship)
	case 9:
		e[evByte] = 0
	case 10, 11, 12:
		s.setEvWord(t, evWord1, s.Colonies[ci].Planet)
	case 13:
		s.setEvWord(t, evWord1, ship)
		s.setEvWord(t, evWord2, other)
	case 14:
		st := g.pickRandomStar(victim)
		s.setEvWord(t, evWord1, st)
		if st == -1 || s.starHasEvent(st) != -1 {
			cancel()
			break
		}
		v := g.eventPirateActivityStrength(st)
		s.setEvWord(t, evWord2, v)
		s.setEvWord(t, evWord3, v)
		if v < 1 {
			cancel()
		}
	case 15:
		st := g.pickRandomStar(victim)
		if s.setEvWord(t, evWord1, st); st == -1 {
			cancel()
			break
		}
		r := int32(s.Random.Random(21)) + 29
		s.setEvWord(t, evWord2, int16(uint32(s.Players[victim].BC*r)/100))
	case 16:
		s.setEvWord(t, evWord1, ci)
		k := diff * 2
		k += int32(s.Random.Random(8))
		v := int16(int32(s.colonyResearchPoints(s.evWord(t, evWord1))) * k)
		s.setEvWord(t, evWord2, v)
		s.setEvWord(t, evWord3, v)
		if v < 1 {
			cancel()
		}
	case 17:
		e[evByte] = 0
		s.setEvWord(t, evWord1, ci)
	case 19, 20, 21, 22, 23:
		st := g.pickRandomStar(victim)
		s.setEvWord(t, evWord1, st)
		// The victim is dropped whether or not a star was found.
		if st == -1 {
			e[evState] = 0
		}
		e[evVictim] = 0xff
	case 24:
		st := g.pickCompletelyRandomStar()
		s.setEvWord(t, evWord1, st)
		if st == -1 || s.starHasEvent(st) != -1 {
			cancel()
			break
		}
		e[evByte] = byte(s.Random.Random(5)) + 10 - byte(diff)
		s.setEvWord(t, evWord2, 0)
		for _, pi := range s.Stars[st].Planets {
			if pi != -1 && s.Planets[pi].Colony != -1 {
				s.setEvWord(t, evWord2, s.evWord(t, evWord2)+s.colonyResearchPoints(s.Planets[pi].Colony))
			}
		}
		v := int16(int8(e[evByte])) * s.evWord(t, evWord2)
		s.setEvWord(t, evWord2, v)
		s.setEvWord(t, evWord3, v)
	case 25:
		st := g.pickRandomStar(victim)
		s.setEvWord(t, evWord1, st)
		e[evByte] = 0
		if st == -1 || s.starHasEvent(st) != -1 {
			cancel()
		}
	case 26:
		e[evByte] = 0
	case 27:
		e[evState] = 3
		e[evByte] = 0
		s.setEvWord(t, evWord1, ship)
	case 28:
		s.setEvWord(t, evWord1, int16(wormCount))
		s.setEvWord(t, evWord2, int16(wormStar))
	}
}
