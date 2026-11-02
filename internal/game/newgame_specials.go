package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// Orion, the monsters, wormholes and marooned heroes of a new galaxy
// (MOX\HAROLD\HOMEGEN.C, UNIVERSE.C). Spec: docs/newgame/generation.md.

const (
	orionName         = 0x179577 // "Orion"
	guardedSpecialAll = 2        // Set_Special_For_Star_Guarded_By_Monster_'s masks
)

// generateOrion is Generate_Orion_ (0x7bbbc): star 0 becomes Orion (its
// planets but one asteroids, a gaia world) and trades places with a star
// near the map's centre.
func (g *Game) generateOrion() int16 {
	s := &g.state
	id := g.getOrionStarID()
	orbit := int16(-1)
	o0 := &s.Stars[0]
	for k := int16(0); k < 5; k++ {
		if o0.Planets[k] > -1 && k > 2 && s.Random.Random(uint32(int32(k)-1)) == 1 {
			orbit = k
		}
	}
	if orbit < 2 {
		orbit = int16(s.Random.Random(3)) + 1
	}
	pi := o0.Planets[orbit]
	if pi == -1 {
		pi = s.PlanetCount
		o0.Planets[orbit] = pi
		s.PlanetCount++
	}
	for k := int16(0); k < 5; k++ {
		if k != orbit && o0.Planets[k] != -1 {
			s.Planets[o0.Planets[k]].Type = 1
		}
	}
	o0.Special = 0xb
	copy(o0.Name[:6], g.exe.bytes(orionName, 6))
	if o0.SpectralClass == 6 {
		o0.SpectralClass = uint8(s.Random.Random(6)) - 1
	}
	other := &s.Stars[id]
	x, y := other.X, other.Y
	other.X, other.Y = o0.X, o0.Y
	o0.X, o0.Y = x, y
	p := &s.Planets[pi]
	*p = save.Planet{Colony: -1, Type: 3, Size: 4, Gravity: 1, ClimateGroup: 2, Climate: 9, Minerals: 4, Special: 0xb, Orbit: uint8(orbit)}
	p.FoodPerFarmer = g.generateFoodPerFarmer(0, int(pi))
	p.MaxFarms = g.generateMaxFarms(0, int(pi))
	return id
}

// isHomeStar is Is_Home_Star_ (0x798c4) with Players_Home_Star_ (0x79e8d).
func (s *State) isHomeStar(star int) bool {
	for _, pi := range s.Stars[star].Planets {
		if pi <= -1 {
			continue
		}
		for p := int16(0); p < s.NumPlayers; p++ {
			if s.Players[p].HomePlanet == pi {
				return true
			}
		}
	}
	return false
}

// isAddedFoodWorld is Is_Added_Food_World_ (0x7c396).
func (g *Game) isAddedFoodWorld(star int) bool {
	for p := int16(0); p < g.state.NumPlayers; p++ {
		if g.gen.nearestFood[p] == int16(star) {
			return true
		}
	}
	return false
}

// getOrionStarID is Get_Orion_Star_Id_ (0x7bd87): a star near the map's
// centre that is no home and no added food world.
func (g *Game) getOrionStarID() int16 {
	s := &g.state
	sc := int32(s.CurMapScale)
	cx, cy := int16(sc*0xfd/10+0x15), int16(sc*0xc8/10+0x15)
	var taken [0x48]bool
	type cand struct {
		dist int32
		star int16
	}
	list := make([]cand, s.NumStars)
	for i := range list {
		list[i] = cand{-1, -1}
	}
	n := 0
	for maxD := int16(4); n < 4 && maxD < 8; maxD++ {
		for st := 0; st < int(s.NumStars); st++ {
			if taken[st] || s.isHomeStar(st) || g.isAddedFoodWorld(st) {
				continue
			}
			d := int16(parsecsBetweenPoints(s.Stars[st].X, s.Stars[st].Y, cx, cy))
			if d >= maxD {
				continue
			}
			taken[st] = true
			list[n] = cand{int32(d), int16(st)}
			n++
		}
	}
	clib.Qsort(list, 6, func(a, b *cand) int { return int(a.dist - b.dist) })
	k := int16(0)
	for i := range list {
		if list[i].dist > 0 {
			list[k] = list[i]
			k++
		}
	}
	if k > 1 {
		return list[int16(s.Random.Random(uint32(int32(k))))-1].star
	}
	for {
		st := int16(s.Random.Random(uint32(int32(s.NumStars)))) - 1
		if !s.isHomeStar(int(st)) && !g.isAddedFoodWorld(int(st)) {
			return st
		}
	}
}

// starIsOrion is Star_Is_Orion_ (0x7a115): by the name.
func (g *Game) starIsOrion(star int) bool {
	return save.CString(g.state.Stars[star].Name[:]) == cStringAt(g.exe, 0x179433)
}

// isColonyStar is Is_Colony_Star_ (0x7987a).
func (s *State) isColonyStar(star int) bool {
	for _, pi := range s.Stars[star].Planets {
		if pi > -1 && s.Planets[pi].Colony != -1 {
			return true
		}
	}
	return false
}

// emptySystem is Empty_System_ (0x79001).
func (s *State) emptySystem(star int) bool {
	for _, pi := range s.Stars[star].Planets {
		if pi != -1 {
			return false
		}
	}
	return true
}

// monsterCandidate is the test Make_System_Monsters_ makes twice.
func (g *Game) monsterCandidate(st int) bool {
	s := &g.state
	return !g.starIsOrion(st) && !s.isHomeStar(st) && !s.isColonyStar(st) &&
		!g.isAddedFoodWorld(st) && !s.emptySystem(st) && s.Stars[st].SpectralClass != 6
}

// makeSystemMonsters is Make_System_Monsters_ (0x7cdc5): the stars to be
// guarded, apart from each other; their special moves to TwinkleFrame.
func (g *Game) makeSystemMonsters() {
	s := &g.state
	gs := &g.gen
	mx, my := int32(g.scaled16(s.MapMaxX)), int32(g.scaled16(s.MapMaxY))
	maxD := uint32(mx*mx + my*my)
	if g.userWantsSpaceMonsters != -1 {
		excl := int16(0)
		for st := 0; st < int(s.NumStars); st++ {
			if !g.monsterCandidate(st) {
				excl++
			}
		}
		avail := int32(s.NumStars) - int32(excl)
		if w := int32(g.userWantsSpaceMonsters); w <= avail {
			avail = w
		}
		maxD = 0
		gs.nSpaceMonsters = int8(avail)
		g.userWantsSpaceMonsters = int8(avail)
	} else if uint8(gs.nSpaceMonsters) > 0x14 {
		gs.nSpaceMonsters = 0x14
	}
	if gs.nSpaceMonsters == 0 {
		return
	}
	var w [maxStars]uint16
	for st := 0; st < int(s.NumStars); st++ {
		if g.monsterCandidate(st) {
			w[st] = 1
		}
	}
	var chosen [maxStars]int16
	cx := int16(0)
	for placed := int16(0); int16(uint8(gs.nSpaceMonsters)) > placed; placed++ {
		n := uint32(1)
		if int32(cx)*2 > 1 {
			n = uint32(int32(cx) * 2)
		}
		minD := maxD / n
		pick := int16(-1)
		for tries := int16(0); ; {
			c := getWeightedChoiceShort(&s.Random, w[:s.NumStars])
			ok := true
			for si := int16(0); si < cx && ok; si++ {
				if uint32(s.distBetweenStars(int(c), int(chosen[si]))) < minD {
					ok = false
				}
			}
			if ok {
				pick = c
				break
			}
			tries++
			if tries > 0xfa {
				break
			}
		}
		if pick == -1 {
			return
		}
		st := &s.Stars[pick]
		st.TwinkleFrame = int8(st.Special)
		st.Special = 9
		w[pick] = 0
		chosen[cx] = pick
		cx++
	}
}

// makeSystemMonstersIntoShips is Make_System_Monsters_Into_Ships_ (0x122cc):
// a random monster at each guarded star (whose planet gets better), the
// Guardian at Orion.
func (g *Game) makeSystemMonstersIntoShips() {
	s := &g.state
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		star := &s.Stars[st]
		if star.Special != 9 {
			continue
		}
		typ := int16(s.Random.Random(5)) + 9
		pi := g.randomPlanet(st)
		s.Ships[s.NumShips].Owner = 8
		g.buildNonplayerShip(typ, int16(st), false)
		star.Special = uint8(star.TwinkleFrame)
		star.TwinkleFrame = 0
		g.upgradePlanetGuardedByMonster(int(uint16(pi)), typ)
	}
	for st := 0; st < int(s.NumStars); st++ {
		if s.Stars[st].Special == 0xb {
			s.Ships[s.NumShips].Owner = 8
			g.buildNonplayerShip(9, int16(st), false)
			break
		}
	}
}

// randomPlanet is Random_Planet_ (0x7c94e): one of the star's planets.
func (g *Game) randomPlanet(star int) int16 {
	s := &g.state
	var w [5]uint16
	for o, pi := range s.Stars[star].Planets {
		if pi == -1 {
			continue
		}
		if t := s.Planets[pi].Type; t > 0 && t < 4 {
			w[o] = 1
		}
	}
	o := getWeightedChoiceShort(&s.Random, w[:])
	return s.Stars[star].Planets[uint16(o)%5]
}

// upgradePlanetGuardedByMonster is Upgrade_Planet_Guarded_By_Monster_
// (0x7c9ee): the guarded planet becomes a bigger, richer, better planet.
func (g *Game) upgradePlanetGuardedByMonster(pi int, typ int16) {
	s := &g.state
	p := &s.Planets[pi]
	var size, minerals [5]uint16
	var climate [10]uint16
	doClimate := true
	allClimates := func() {
		if p.Type == 3 {
			doClimate = false
			return
		}
		for i := range climate {
			climate[i] = 1
		}
	}
	switch typ - 10 {
	case 0:
		size[2], size[3], size[4] = 1, 1, 1
		minerals[3], minerals[4] = 1, 1
		allClimates()
	case 2:
		size[2] = 1
		fallthrough
	case 1, 4:
		size[3], size[4] = 1, 1
		minerals[3], minerals[4] = 1, 1
		climate[8], climate[9] = 1, 1
	case 3:
		size[3], size[4] = 1, 1
		minerals[3], minerals[4] = 1, 1
		allClimates()
	}
	p.Type = 3
	if v := getWeightedChoiceShort(&s.Random, size[:]); int32(uint16(v)) > int32(p.Size) {
		p.Size = uint8(v)
	}
	if v := getWeightedChoiceShort(&s.Random, minerals[:]); int32(uint16(v)) > int32(p.Minerals) {
		p.Minerals = uint8(v)
	}
	if doClimate {
		if v := getWeightedChoiceShort(&s.Random, climate[:]); int32(uint16(v)) > int32(p.Climate) {
			p.Climate = uint8(v)
		}
	}
	g.setSecondaryPlanetData(int(p.Star), pi)
	g.setSpecialForStarGuardedByMonster(int(p.Star), typ)
}

// setSpecialForStarGuardedByMonster is Set_Special_For_Star_Guarded_By_
// Monster_ (0x7cb7c): a star special the monster's kind allows.
func (g *Game) setSpecialForStarGuardedByMonster(star int, typ int16) {
	s := &g.state
	st := &s.Stars[star]
	mask := int32(guardedSpecialAll)
	switch typ {
	case 0xc:
		mask = 0xa96
	case 0xd:
		mask = 0xf4e
	}
	has := false
	for _, pi := range st.Planets {
		if pi > -1 && int8(s.Planets[pi].Special) > 0 {
			has = true
			break
		}
	}
	redo := false
	if has {
		if mask>>uint(int8(st.Special)&31)&1 != 0 {
			redo = true
			for _, pi := range st.Planets {
				if pi > -1 {
					s.Planets[pi].Special = 0
				}
			}
		}
	} else {
		redo = true
	}
	if !redo {
		return
	}
	for {
		var worm int16
		g.getPlanetOrStarSpecial(star, &worm, &st.Special, true, mask, false)
		if int8(st.Special) > 0 {
			break
		}
	}
}

// nSatellites is N_Satellites_ (0x79bae).
func (s *State) nSatellites(star int) int8 {
	n := int8(0)
	for _, pi := range s.Stars[star].Planets {
		if pi != -1 {
			n++
		}
	}
	return n
}

// generateWormholeLinks is Generate_Wormhole_Links_ (0x8cc15): the
// wormholes the specials asked for, at random stars far from homes.
func (g *Game) generateWormholeLinks() {
	s := &g.state
	var homes [0x48]bool
	for p := int16(0); p < s.NumPlayers; p++ {
		homes[s.Planets[s.Players[p].HomePlanet].Star] = true
	}
	for k := int16(0); k < int16(g.gen.nWormholes); k++ {
		pick := int16(-1)
		for tries := int16(0); ; {
			r := int16(s.Random.Random(uint32(int32(s.NumStars) - 1)))
			if r != 0 {
				st := &s.Stars[r]
				if !homes[r] && st.Special == 0 && !s.emptySystem(int(r)) && st.SpectralClass != 6 && st.WormholeTo == -1 {
					pick = r
				}
			}
			tries++
			if tries > 0xc8 || pick != -1 {
				break
			}
		}
		if pick != -1 {
			g.setWormholeID(int(pick), &homes)
		}
	}
}

// setWormholeID is Set_Wormhole_Id_ (0x8d6d6) with the 1.40 patch's /nowh
// (0xf4b56): a far star to link with; none links the star to star 0.
func (g *Game) setWormholeID(star int, homes *[0x48]bool) {
	s := &g.state
	r := &s.Random
	target := int16(-1)
	var cand [20]int16
	for i := range cand {
		cand[i] = -1
	}
	cnt := int16(0)
	for b := 0; b < int(s.NumStars) && cnt != 0x13; b++ {
		if uint32(g.parsecsBetweenStars(star, b)) <= uint32(g.settings[0xd6])+3 {
			continue
		}
		st := &s.Stars[b]
		if homes[b] || st.WormholeTo != -1 || st.SpectralClass == 6 || st.Special != 0 || b == star {
			continue
		}
		cand[cnt] = int16(b)
		cnt++
	}
	done := cnt <= 0
	if !done {
		removed := int16(0)
		for b := int16(0); b < cnt; b++ {
			ok := s.Stars[cand[b]].WormholeTo <= 0
			if s.nSatellites(int(cand[b])) == 0 {
				ok = false
			}
			if !ok {
				removed++
				cand[b] = -1
			}
		}
		done = removed >= cnt
	}
	if !done {
		// The draws are 1..cnt and index the list from 0, as in the original.
		for si := int16(0); int32(si) < int32(cnt)*5/3; si++ {
			a := int16(r.Random(uint32(int32(cnt))))
			b := int16(r.Random(uint32(int32(cnt))))
			if a == b || cand[a] == -1 || cand[b] == -1 {
				continue
			}
			cand[a], cand[b] = cand[b], cand[a]
		}
		k := uint32(1)
		for b := int16(0); b < cnt; b++ {
			if cand[b] == -1 {
				continue
			}
			if r.Random(k) == 1 {
				target = cand[b]
				k++
			}
		}
	}
	if target <= -1 {
		s.Stars[star].WormholeTo = 0
		return
	}
	if g.exeU8(patchNoWormholes) != 0 {
		s.Stars[target].WormholeTo = -1
		s.Stars[star].WormholeTo = -1
		return
	}
	s.Stars[target].WormholeTo = int8(star)
	s.Stars[star].WormholeTo = int8(target)
}

// assignMaroonedHeroes is Assign_Marooned_Heroes_ (0x8bb51): a free leader
// for each marooned hero special, else the special goes.
func (g *Game) assignMaroonedHeroes() {
	s := &g.state
	for st := 0; st < int(s.NumStars); st++ {
		if s.Stars[st].Special != 8 {
			continue
		}
		hero := int16(-1)
		var r int16
		for tries := int16(0); ; {
			r = int16(s.Random.Random(0x26)) - 1
			if r != 0x41 && s.Leaders[r].Status == -1 {
				hero = r
				break
			}
			tries++
			if tries == 0x64 {
				break
			}
		}
		if hero != -1 {
			s.Leaders[r].Status = 5
			s.Leaders[r].Location = int16(st)
		} else {
			s.Stars[st].Special = 0
		}
	}
}
