package game

// Event_Twiddle_ (EVENTS.C) and its helpers. Spec: docs/turn/overview.md,
// phase F, "Event_Twiddle_".

// Event states as Event_Twiddle_ moves them: 1 new → 6 (the news turn) →
// 2 running; 5 ends next turn (→ 3), 4 shows again (→ 2).
const (
	evStateNew   = 1
	evStateOn    = 2
	evStateOff   = 3
	evStateAgain = 4
	evStateEnd   = 5
	evStateNews  = 6
)

// shipSizePointsAtStar is Ship_Size_Points_At_Star_ (0x23b28): hull size + 1
// summed over all 500 slots of ships orbiting the star.
func (s *State) shipSizePointsAtStar(star int16) int16 {
	n := int16(0)
	for i := 0; i < maxShips; i++ {
		if sh := &s.Ships[i]; sh.Location == star && sh.Status == 0 {
			n += int16(sh.Design.Size) + 1
		}
	}
	return n
}

// eventTwiddle is Event_Twiddle_ (0x206a2): every event moves one state on
// and the running ones act.
func (g *Game) eventTwiddle() {
	s := &g.state
	if e := s.ev(19); e[evState] == evStateAgain {
		e[evState] = evStateOff
	}
	for t := 0; t < eventCount; t++ {
		e := s.ev(t)
		switch e[evState] {
		case evStateEnd:
			e[evState] = evStateOff
		case evStateAgain, evStateNews:
			e[evState] = evStateOn
		}
		if e[evState] != evStateNew && e[evState] != evStateOn {
			continue
		}
		if e[evState] == evStateNew {
			e[evState] = evStateNews
		}
		g.twiddleEvent(t, e)
	}
}

// twiddleEvent is the body of Event_Twiddle_'s switch for event t.
func (g *Game) twiddleEvent(t int, e []byte) {
	s := &g.state
	victim := int(int8(e[evVictim]))
	w1, w2, w3 := s.evWord(t, evWord1), s.evWord(t, evWord2), s.evWord(t, evWord3)
	end := func() { e[evState] = evStateEnd }
	news := e[evState] == evStateNews
	planetColony := func() int16 { return s.Planets[w1].Colony }
	// tick is the shared tail of the duration events: a 1-in-20 end after
	// four turns, a hard end after twenty, one turn older.
	tick := func(rnd bool) {
		if rnd && int8(e[evByte]) > 4 && s.Random.Random(20) == 1 {
			end()
		}
		if int8(e[evByte]) > 20 {
			end()
		}
		e[evByte]++
	}
	switch t {
	case 0: // derelict ship
		g.playerGetsTechApp(victim, int(w1))
		if w2 != 0 {
			g.playerGetsTechApp(victim, int(w2))
		}
		end()
	case 1: // climate change
		pl := &s.Planets[w1]
		old := pl.Climate
		pl.Climate = 8
		pl.FoodPerFarmer += g.exeU8(uint32(tblClimateFood+int(pl.Climate))) - g.exeU8(uint32(tblClimateFood+int(old)))
		if ci := planetColony(); ci != -1 {
			g.colonyCalculation(int(ci))
			g.deleteProdFromColonyQueue(int(ci), 0x2c)
		}
		end()
	case 2: // comet
		if news {
			return
		}
		star := int16(s.Planets[s.Colonies[w1].Planet].Star)
		w2 -= s.shipSizePointsAtStar(star)
		s.setEvWord(t, evWord2, w2)
		if w2 > 0 {
			e[evByte]--
			if int8(e[evByte]) <= 0 {
				g.allocateCometHit(int(w1))
				end()
			}
		} else {
			end()
		}
		if s.Random.Random(20) == 1 && s.evWord(t, evWord3) != s.evWord(t, evWord2) {
			e[evState] = evStateAgain
		}
	case 3: // research loss
		r := int16(s.Random.Random(50)) + 50
		pl := &s.Players[victim]
		if int32(r) > pl.ResearchPoints {
			r = int16(pl.ResearchPoints)
		}
		s.setEvWord(t, evWord1, r)
		s.setEvWord(t, evWord2, int16(pl.ResearchField))
		pl.ResearchPoints -= int32(r)
		end()
	case 4, 5: // relations worse, better
		d := int16(-100)
		if t == 5 {
			d = 100
		}
		g.changeRelations(d, victim, int(w1), 0, 0, 0)
		end()
	case 6: // windfall
		s.Players[victim].BC += int32(w1)
		end()
	case 7: // earthquake
		end()
		pop, bld := g.allocateEarthquakeHit(int(w1))
		s.setEvWord(t, evWord2, pop)
		s.setEvWord(t, evWord3, bld)
		if pop < 1 && bld < 1 {
			e[evState] = 0
			e[evVictim] = 0xff
		}
		g.colonyCalculation(int(w1))
	case 8: // ship lost
		if o := s.Ships[w1].Officer; o != -1 {
			s.setEvWord(t, evWord2, o)
			g.killOfficer(int(o))
		}
		g.killShip(int(w1))
		end()
	case 9: // hyperspace flux
		if e[evState] == evStateOn {
			tick(true)
		}
	case 10: // industrial accident
		if ci := planetColony(); ci != -1 {
			g.allocateIndustrialAccidentHit(int(ci))
			g.colonyCalculation(int(planetColony()))
		}
		end()
	case 11, 12: // mineral depletion, deposit
		pl := &s.Planets[w1]
		if t == 11 {
			pl.Minerals = uint8(max(int32(pl.Minerals)-1, 0))
		} else {
			pl.Minerals = uint8(min(int32(pl.Minerals)+2, 4))
		}
		if ci := pl.Colony; ci != -1 {
			g.colonyCalculation(int(ci))
		}
		end()
	case 14: // pirates
		if news {
			return
		}
		chance := int16(int32(w2) * 100 / int32(w3))
		if int32(s.Random.Random(100)) <= int32(chance) {
			for p := int16(0); p < s.NumPlayers; p++ {
				if s.Stars[w1].ColonyOwners>>uint(p)&1 != 0 && s.Players[p].FreightersTotal > 0 {
					s.Players[p].FreightersTotal--
				}
			}
		}
		w2 -= s.shipSizePointsAtStar(w1)
		s.setEvWord(t, evWord2, w2)
		if w2 <= 0 {
			end()
		}
	case 15: // pirate raid on BC
		pl := &s.Players[victim]
		if pl.BC -= int32(w2); pl.BC < 0 {
			pl.BC = 0
		}
		end()
	case 16: // plague
		if news {
			return
		}
		s.setEvWord(t, evWord2, w2-s.colonyResearchPoints(w1))
		g.colonyCalculation(int(w1))
		if s.evWord(t, evWord2) <= 0 {
			end()
		}
		if int8(e[evByte]) > 4 && s.Random.Random(20) == 1 {
			e[evState] = evStateAgain
		}
		e[evByte]++
	case 17: // population boom
		if e[evState] == evStateOn {
			g.colonyCalculation(int(w1))
			tick(true)
		}
	case 18: // research breakthrough
		pl := &s.Players[victim]
		f := int16(pl.ResearchField)
		s.setEvWord(t, evWord1, f)
		g.givePlayerField(victim, int(f), true)
		pl.ResearchPoints = 0
		pl.ResearchField = 0
		end()
	case 19, 20, 21, 22, 23: // space monsters
		if news {
			g.buildNonplayerShip(int16(t-19+10), w1, true)
			return
		}
		if int8(e[evByte]) > 4 {
			end()
		}
		if w2 != -1 {
			end()
		}
	case 24: // supernova
		if news {
			return
		}
		st := &s.Stars[w1]
		for k := 0; k < 5; k++ {
			if pi := st.Planets[k]; pi != -1 {
				if ci := s.Planets[pi].Colony; ci != -1 {
					s.setEvWord(t, evWord2, s.evWord(t, evWord2)-s.colonyResearchPoints(ci))
					g.colonyCalculation(int(ci))
				}
			}
		}
		if s.evWord(t, evWord2) > 0 {
			e[evByte]--
			if int8(e[evByte]) <= 0 {
				for k := 0; k < 5; k++ {
					pi := st.Planets[k]
					if pi == -1 {
						continue
					}
					s.Planets[pi].Climate = 1
					if ci := s.Planets[pi].Colony; ci != -1 {
						g.destroyColony(int(ci), -1)
					}
				}
				end()
			}
		} else {
			end()
		}
		if s.Random.Random(20) == 1 && e[evState] != evStateEnd {
			e[evState] = evStateAgain
		}
	case 25: // space anomaly
		if e[evState] != evStateOn {
			return
		}
		for k := 0; k < 5; k++ {
			if pi := s.Stars[w1].Planets[k]; pi != -1 {
				if ci := s.Planets[pi].Colony; ci != -1 {
					g.colonyCalculation(int(ci))
				}
			}
		}
		tick(true)
	case 26: // hyperspace beast
		if e[evState] != evStateOn {
			return
		}
		if int8(e[evByte]) > 4 && s.Random.Random(20) == 1 {
			end()
		}
		if int8(e[evByte]) > 20 {
			end()
		}
		if e[evState] != evStateEnd {
			g.doHyperspaceBeast(int16(victim))
			e[evVictim] = byte(g.getEventVictim(g.eventGood(t)))
		}
		e[evByte]++
	case 27:
		if e[evState] == evStateOn {
			tick(true)
		}
	case 13:
	default: // 28..35: news only
		end()
	}
}

// doHyperspaceBeast is Do_Hyperspace_Beast_ (0x100618): one of p's ships in
// flight is eaten; its officer escapes to a ship of the same group when the
// race has app 0xb3, and every player hears of it.
func (g *Game) doHyperspaceBeast(p int16) {
	s := &g.state
	var m [msgSize]byte
	m[0] = 9
	stale := [][2]int{{1, 2}, {8, msgSize}}
	victim := -1
	if p != -1 {
		n := uint32(0)
		for i := int(s.NumShips) - 1; i >= 0; i-- {
			if sh := &s.Ships[i]; int16(sh.Owner) == p && sh.Status == 1 {
				n++
				if s.Random.Random(n) == 1 {
					victim = i
				}
			}
		}
	}
	if victim == -1 {
		m[2], m[3] = 0xff, 0xff
		stale = [][2]int{{1, 2}, {4, msgSize}}
	} else {
		sh := &s.Ships[victim]
		o := sh.Officer
		m[2], m[3] = byte(victim), byte(victim>>8)
		m[4] = byte(sh.Location) + 12 // the star of location 500 + star
		m[5] = byte(o)
		if o == -1 {
			stale = [][2]int{{1, 2}, {6, 7}, {8, msgSize}}
		} else {
			g.moveOfficerToPool(int(o))
			m[6] = g.beastOfficerFate(victim, o)
			if m[6] == 0 {
				g.killOfficer(int(o))
			}
		}
		g.updateShipIconsInBetweenTurns()
		sh.Status = 8
	}
	for q := int(s.NumPlayers) - 1; q >= 0; q-- {
		if q == int(p) {
			m[7] = 1
		} else {
			m[7] = 0
		}
		i := g.addMsg(q, m)
		for _, r := range stale {
			g.markStaleMsg(i, r[0], r[1])
		}
	}
}

// beastOfficerFate returns 1 when the officer was away (or a group mate had
// no room), 2 when it moved to the largest free group mate, 0 when it dies.
func (g *Game) beastOfficerFate(victim int, o int16) byte {
	s := &g.state
	l := &s.Leaders[o]
	if l.ETA > 0 {
		return 1
	}
	sh := &s.Ships[victim]
	if s.Players[sh.Owner].AppStatus[0x1ca-0x117] != 3 {
		return 0
	}
	best, mates := -1, false
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		c := &s.Ships[i]
		if c.Owner != sh.Owner || c.Status != sh.Status || c.X != sh.X || c.Y != sh.Y || i == victim ||
			c.Location != sh.Location || c.Speed != sh.Speed {
			continue
		}
		mates = true
		if c.Officer != -1 {
			continue
		}
		if (best == -1 || s.Ships[best].Design.Size < c.Design.Size) && c.Design.Type == 0 {
			best = i
		}
	}
	if best != -1 {
		s.Ships[best].Officer = o
		l.Status = 1
		l.Location = int16(best)
		return 2
	}
	if mates {
		return 1
	}
	return 0
}
