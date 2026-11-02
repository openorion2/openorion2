package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// Home worlds, MOX\HAROLD\HOMEGEN.C: Generate_Home_Worlds_ and what it
// runs. Spec: docs/newgame/generation.md.

const (
	climateGroupAllowed = 0x17d77f // [climate*4 + group]: the group can have the climate
	homePlanetsMin      = 0x178485 // the 1.40 patch's /planets= (2 by default)
	homeStartBuildings  = 0x17d8ac // i16 buildings a home world may start with, 0 ends
	homeBuildingLimit   = 0x13a3a  // u8 by tech level: most buildings to start with
	patchMinStart       = 0x17844a // /minstart
	patchGoodStart      = 0x178456 // /goodstart
	patchHugeStart      = 0x17846e // /hugestart
	patchRichStart      = 0x17847a // /richstart
	patchFlagsByte      = 0x1784dd // the patch's first data byte; 0 runs Generate_Orion_
	patchNoReport       = 0x1787b8
	patchNoScan         = 0x1787c1
	patchNoHousing      = 0x178755
)

// starDist is a row of _good_stars: the stars at least the minimum
// distance away and their squared distances.
type starDist struct {
	far   [9]byte
	dist  [maxStars]int32
	count int16
}

// generateHomeWorlds is Generate_Home_Worlds_ (0x7b8cd): home stars as far
// apart as the map allows (the spacing shrinks with each failure); false
// when none fit.
func (g *Game) generateHomeWorlds() bool {
	s := &g.state
	gs := &g.gen
	gs.goodStars = make([]starDist, maxStars)
	for i := range gs.nearestFood {
		gs.nearestFood[i] = -1
	}
	si := s.NumPlayers
	if si > 6 {
		si += gs.galaxySize
	}
	limit := si + 4
	var list [8]int16
	g.progressLog(g.setUpHomeGenMsg())
	for done := false; !done; {
		for i := 0; i < int(s.NumStars); i++ {
			s.Stars[i].Owner = -1
		}
		div := int32(si) * 10
		dx := int32(s.MapMaxX) * int32(s.MaxMapScale) / div
		dy := int32(s.MapMaxY) * int32(s.MaxMapScale) / div
		d := dx*dx + dy*dy
		if d <= 0x3a98 {
			d = 0x3a98
		}
		g.homeGenStep()
		g.buildMinStarDistances(d)
		g.homeGenStep()
		if g.buildHomeStarList(&list, d) {
			si++
			if si > limit {
				return false
			}
			continue
		}
		done = true
	}
	g.randomizeHomeWorlds(&list)
	for p := int16(0); p < s.NumPlayers; p++ {
		s.Stars[list[p]].Owner = int8(p)
	}
	g.modifyHomeWorlds()
	g.progressLog(g.genProgressStep(int16(g.genProg.homeMax), true))
	// The 1.40 patch (0x169245): Orion, and its switches kept in star 0's
	// name bytes 0xd and 0xe.
	if g.exeU8(patchFlagsByte) == 0 {
		g.generateOrion()
	}
	f := g.exeU8(patchNoReport)&4 | g.exeU8(patchNoScan)&2 | g.patchByte(patchCluster) | g.exeU8(patchNoHousing)&8
	s.Stars[0].Name[0xe] = f
	s.Stars[0].Name[0xd] = 0
	return true
}

// homeGenStep is Generate_Home_Worlds_'s progress call: a step while the
// counter is below _HOME_WORLD_GEN_MAX_CTR.
func (g *Game) homeGenStep() {
	if int32(g.genProg.counter) < g.genProg.homeMax {
		g.progressLog(g.genProgressStep(int16(g.genProg.homeMax), false))
	}
}

// progressLog notes a failed progress drawing; the generation goes on.
func (g *Game) progressLog(err error) {
	if err != nil {
		g.Log("generation progress: %v", err)
	}
}

// distBetweenStars is Dist_Between_Stars_ (0x7b0b4), squared.
func (s *State) distBetweenStars(a, b int) int32 {
	A, B := &s.Stars[a], &s.Stars[b]
	return distBetweenPoints(A.X, A.Y, B.X, B.Y)
}

// buildMinStarDistances is Build_Min_Star_Distances_ (0x7b020).
func (g *Game) buildMinStarDistances(d int32) {
	s := &g.state
	for c := 0; c < int(s.NumStars); c++ {
		gd := &g.gen.goodStars[c]
		gd.count = 0
		for i := 0; i < int(s.NumStars); i++ {
			v := s.distBetweenStars(c, i)
			if v < d {
				continue
			}
			gd.far[i>>3] |= 1 << uint(i&7)
			gd.dist[i] = v
			gd.count++
		}
	}
}

// buildHomeStarList is Build_Home_Star_List_ (0x7ad13): for each star (but
// star 0) the stars far from it and from each other, at most 20; one group
// big enough, picked at random and shuffled, gives the homes. It reports a
// failure.
func (g *Game) buildHomeStarList(list *[8]int16, d int32) bool {
	s := &g.state
	r := &s.Random
	n := int(s.NumStars)
	// groups[i][20] is the group's count; a 21st member would overwrite it.
	var groups [maxStars][21]int16
	for si := 1; si < n; si++ {
		if int32(g.genProg.counter) < g.genProg.homeMax {
			g.progressLog(g.genProgressStep(int16(g.genProg.homeMax), false))
		} else {
			g.progressLog(g.setUpHomeGenMsg())
		}
		grp := &groups[si]
		grp[20] = 1
		grp[0] = int16(si)
		for bx := 1; bx < n; bx++ {
			cnt := grp[20]
			if si == bx || !testBit(g.gen.goodStars[si].far[:], bx) {
				continue
			}
			ok := true
			for di := int16(0); di < cnt && ok; di++ {
				m := int(grp[di])
				dist := s.distBetweenStars(m, bx)
				bit := testBit(g.gen.goodStars[m].far[:], bx)
				if bx != m && (!bit || dist < d) {
					ok = false
				}
			}
			if !ok {
				continue
			}
			grp[grp[20]] = int16(bx)
			c := int32(grp[20]) + 1
			if c >= 0x14 {
				c = 0x14
			}
			grp[20] = int16(c)
		}
	}
	pick := int16(-1)
	k := int32(1)
	for dx := 0; dx < 0x48; dx++ {
		if groups[dx][20] < s.NumPlayers {
			continue
		}
		if r.Random(uint32(k)) == 1 {
			pick = int16(dx)
			k++
		}
	}
	if pick <= -1 {
		return true
	}
	grp := &groups[pick]
	for i := 0; i < 4; i++ {
		a := int16(r.Random(uint32(int32(grp[20])))) - 1
		b := int16(r.Random(uint32(int32(grp[20])))) - 1
		grp[a], grp[b] = grp[b], grp[a]
	}
	for i := range list {
		list[i] = -1
	}
	for i := 0; i < int(s.NumPlayers); i++ {
		list[i] = grp[i]
	}
	return false
}

// randomizeHomeWorlds is Randomize_Home_Worlds_ (0x7c6ef): 100 swaps.
func (g *Game) randomizeHomeWorlds(list *[8]int16) {
	r := &g.state.Random
	n := uint32(int32(g.state.NumPlayers))
	for c := 0; c < 100; c++ {
		a := int16(r.Random(n)) - 1
		b := int16(r.Random(n)) - 1
		if a != b {
			list[a], list[b] = list[b], list[a]
		}
	}
}

// modifyHomeWorlds is Modify_Home_Worlds_ (0x7c4af): each home star gets
// its planets, the home planet fitted to the race, the colony, a planet
// near by.
func (g *Game) modifyHomeWorlds() {
	s := &g.state
	var homes [8]int16
	for i := range homes {
		homes[i] = -1
	}
	n := 0
	for c := 0; c < int(s.NumStars); c++ {
		if o := s.Stars[c].Owner; o > -1 && o < 8 {
			homes[n] = int16(c)
			n++
		}
	}
	for c := 0; c < int(s.NumStars); c++ {
		o := int16(s.Stars[c].Owner)
		if o <= -1 || o >= 8 {
			continue
		}
		pl := &s.Players[o]
		g.enforce3PlanetMinimum(c)
		climate := int16(8)
		if pl.Picks[12] != 0 {
			climate = 5
		}
		home := g.enforceClimate(c, climate)
		g.removeSpecials(c)
		g.patchHomeStart(c, home)
		pl.HomePlanet = home
		if pl.Picks[14] != 0 {
			s.Planets[home].Size = 3
		} else {
			g.enforceMediumPlanetSize(home)
		}
		g.enforcePlanetType(c, home)
		if s.Planets[home].Minerals != 2 {
			s.Planets[home].Minerals = 2
		}
		switch {
		case pl.Picks[15] > 0:
			g.enforceMineralRichness(home, 3)
		case pl.Picks[15] < 0:
			g.enforceMineralRichness(home, 1)
		}
		if pl.Picks[16] != 0 {
			s.Planets[home].Special = 0xa
			s.Stars[c].Special = 0xa
		}
		grav := uint8(1)
		if pl.Picks[10] != 0 {
			grav = 0
		} else if pl.Picks[11] != 0 {
			grav = 2
		}
		g.enforceGravity(home, grav, int(o))
		g.initHomeworldColony2(int(home), int(o), true)
		g.guaranteePlanetAtMinDist(c, 4, homes[:])
		if s.Stars[c].Special == 9 {
			s.Stars[c].Special = 0
		}
	}
	s.NumColonies = s.NumPlayers
	g.assignHomeStarNames()
	g.ensureAtLeast1FoodPlanet()
}

// nPlanetsInSystem is N_Planets_In_System_ (0x79b2d): type 3 planets.
func (s *State) nPlanetsInSystem(star int) int8 {
	n := int8(0)
	for _, pi := range s.Stars[star].Planets {
		if pi != -1 && s.Planets[pi].Type == 3 {
			n++
		}
	}
	return n
}

// nFoodPlanetsInSystem is N_Food_Planets_In_System_ (0x79ae8).
func (s *State) nFoodPlanetsInSystem(star int) int8 {
	n := int8(0)
	for _, pi := range s.Stars[star].Planets {
		if pi > -1 && s.Planets[pi].FoodPerFarmer > 0 {
			n++
		}
	}
	return n
}

// enforce3PlanetMinimum is Enforce_3_Planet_Minimum_ (0x7b11a) with the 1.40
// patch's /planets= limit: a habitable class and planets in usable orbits.
func (g *Game) enforce3PlanetMinimum(star int) {
	s := &g.state
	min := int8(g.exeU8(homePlanetsMin))
	if s.nPlanetsInSystem(star) >= min {
		return
	}
	for s.Stars[star].SpectralClass >= 6 {
		s.Stars[star].SpectralClass = g.generateSpectralClass(int16(g.settings[0xd7]))
	}
	for s.nPlanetsInSystem(star) < min {
		o := g.usableOrbit(star)
		if s.Stars[star].Planets[o] == -1 {
			g.makeOnePlanet(star, s.PlanetCount, o, false)
			s.PlanetCount++
			continue
		}
		saved := s.PlanetCount
		s.PlanetCount = s.Stars[star].Planets[o]
		p := &s.Planets[s.PlanetCount]
		p.Type = 3
		p.Orbit = uint8(o)
		g.generateSatellite(star, uint8(o), 3)
		s.PlanetCount = saved
	}
}

// usableOrbit is Usable_Orbit_ (0x7ccb5): a random orbit that is free or
// holds no type 3 planet (testing the planet at _planet_count, as the
// original does), else the first such, else any.
func (g *Game) usableOrbit(star int) int16 {
	s := &g.state
	res := int16(-1)
	found := false
	for tries := int16(0); !found; {
		o := int16(s.Random.Random(5)) - 1
		if o > -1 && o < 5 {
			pi := s.Stars[star].Planets[o]
			switch {
			case pi == -1:
				found, res = true, o
			case pi <= -1 || pi >= s.PlanetCount:
			case s.Planets[s.PlanetCount].Type == 3:
			default:
				found, res = true, o
			}
		}
		tries++
		if tries > 0x64 {
			break
		}
	}
	for o := int16(0); o < 5 && !found; o++ {
		pi := s.Stars[star].Planets[o]
		if pi == -1 || s.Planets[pi].Type != 3 {
			found, res = true, o
		}
	}
	if res == -1 {
		res = int16(s.Random.Random(5)) - 1
	}
	return res
}

// makeOnePlanet is Make_One_Planet_ (0x7c3c4): a type 3 planet at the
// orbit (a food planet when food is set); Generate_Satellite_ fills the
// record at _planet_count.
func (g *Game) makeOnePlanet(star int, pc int16, orbit int16, food bool) {
	s := &g.state
	if food && s.Stars[star].SpectralClass >= 5 {
		s.Stars[star].SpectralClass = uint8(s.Random.Random(5)) - 1
	}
	g.gen.makeFoodPlanet = food
	s.Planets[pc] = save.Planet{}
	s.Planets[pc].Type = 3
	s.Planets[pc].Orbit = uint8(orbit)
	g.generateSatellite(star, uint8(orbit), 3)
	g.gen.makeFoodPlanet = false
	if pc == s.PlanetCount && s.Stars[star].Planets[orbit] >= 0 {
		s.PlanetCount--
		s.Planets[s.Stars[star].Planets[orbit]] = s.Planets[pc]
		return
	}
	s.Stars[star].Planets[orbit] = pc
}

// enforceClimate is Enforce_Climate_ (0x7b22b): the home planet, one of
// the star's planets with the climate, else a random one given it (the
// star's class changed until an orbit's group allows the climate).
func (g *Game) enforceClimate(star int, climate int16) int16 {
	s := &g.state
	type entry struct{ planet, orbit int16 }
	var list [5]entry
	n := int16(0)
	home := int16(-1)
	found := false
	for o := int16(0); o < 5 && !found; o++ {
		pi := s.Stars[star].Planets[o]
		if pi <= -1 {
			continue
		}
		list[n] = entry{pi, o}
		n++
		if int16(s.Planets[pi].Climate) == climate {
			found = true
			home = pi
		}
	}
	if n <= 0 {
		g.Log("Enforce_Climate_: star %d has no planets (the original exits)", star)
		return home
	}
	if !found {
		var groups [4]int16
		var ng int16
		for ok := false; !ok; {
			groups = [4]int16{-1, -1, -1, -1}
			ng = 0
			cls := int32(int8(s.Stars[star].SpectralClass))
			for gr := int16(0); gr < 4; gr++ {
				if g.exeU8(uint32(int32(climateGroupAllowed)+int32(climate)*4+int32(gr))) > 0 {
					groups[ng] = gr
					ng++
				}
			}
			match := false
			for a := int32(0); a < 5; a++ {
				for k := int16(0); k < ng && !match; k++ {
					if int16(g.exeU8(uint32(int32(tblClassToGroup)+cls*5+a))) == groups[k] {
						match = true
					}
				}
			}
			if !match {
				s.Stars[star].SpectralClass = g.generateSpectralClass(int16(g.settings[0xd7]))
				continue
			}
			ok = true
		}
		home = list[int16(s.Random.Random(uint32(int32(n))))-1].planet
		s.Planets[home].Climate = uint8(climate)
		s.Planets[home].ClimateGroup = uint8(groups[int16(s.Random.Random(uint32(int32(ng))))-1])
	}
	s.Planets[home].FoodPerFarmer = g.generateFoodPerFarmer(star, int(home))
	return home
}

// removeSpecials is Remove_Specials_ (0x7c73d).
func (g *Game) removeSpecials(star int) {
	s := &g.state
	st := &s.Stars[star]
	st.Special = 0
	st.WormholeTo = -1
	for _, pi := range st.Planets {
		if pi > -1 {
			s.Planets[pi].Special = 0
		}
	}
}

// patchHomeStart is the 1.40 patch's code after Remove_Specials_ (0xf4b94):
// /hugestart, /minstart, /goodstart, /richstart improve the first other
// habitable planet of the home system (the switches are exe bytes, 0xff on).
func (g *Game) patchHomeStart(star int, home int16) {
	if g.exeU8(patchHugeStart) != 0xff && g.exeU8(patchMinStart) != 0xff &&
		g.exeU8(patchGoodStart) != 0xff && g.exeU8(patchRichStart) != 0xff {
		return
	}
	s := &g.state
	var pl *save.Planet
	for k := 0; pl == nil; k++ { // no bound: the first other habitable slot (0xf4bae)
		if star*0x71+0x4a+2*k+2 > 0x1fc8 {
			g.Log("the patch's home start found no habitable planet in the star array")
			return
		}
		pi := g.starSlotWord(star, k)
		if pi != -1 && pi != home && pi >= 0 && int(pi) < len(s.Planets) && s.Planets[pi].Type == 3 {
			pl = &s.Planets[pi]
		}
	}
	gravity := func() uint8 { // 0xf4c45: by the home star owner's G picks
		P := &s.Players[s.Stars[s.Planets[home].Star].Owner]
		return uint8(1 - P.Picks[10] + P.Picks[11])
	}
	if g.exeU8(patchHugeStart) == 0xff {
		pl.Size, pl.MaxFarms = 4, 10
		pl.Gravity = gravity()
	}
	if g.exeU8(patchMinStart) == 0xff {
		if pl.Size < 2 {
			pl.Size, pl.MaxFarms = 2, 5
		}
		pl.Minerals = pl.Minerals&1 + 1
		pl.Gravity = gravity()
	}
	switch {
	case g.exeU8(patchGoodStart) == 0xff:
		pl.Minerals = 4
		pl.Gravity = gravity()
	case g.exeU8(patchRichStart) == 0xff:
		pl.Minerals = 3
		pl.Gravity = gravity()
	}
}

// enforceMediumPlanetSize is Enforce_Medium_Planet_Size_ (0x7b4b4).
func (g *Game) enforceMediumPlanetSize(pc int16) {
	p := &g.state.Planets[pc]
	if p.Size != 2 {
		p.Size = 2
		p.MaxFarms = g.exeU8(tblPlanetMaxFarms + 2)
	}
}

// enforcePlanetType is Enforce_Planet_Type_ (0x7b5e4).
func (g *Game) enforcePlanetType(star int, pc int16) {
	g.state.Planets[pc].Type = 3
}

// enforceMineralRichness is Enforce_Mineral_Richness_ (0x7b4da): the
// minerals, the star's class changed until it can have them, the gravity.
func (g *Game) enforceMineralRichness(pc int16, m uint8) {
	s := &g.state
	p := &s.Planets[pc]
	p.Minerals = m
	star := int(p.Star)
	for ok := false; !ok; {
		cls := int8(s.Stars[star].SpectralClass)
		if cls < 5 {
			for r := int32(0); r < 10 && !ok; r++ {
				if g.exeU8(uint32(int32(tblClassToMineral)+r*6+int32(cls))) == m {
					ok = true
				}
			}
		}
		if !ok {
			s.Stars[star].SpectralClass = g.generateSpectralClass(int16(g.settings[0xd7]))
		}
	}
	p.Gravity = g.exeU8(tblGravity + uint32(p.Minerals)*5 + uint32(p.Size))
}

// enforceGravity is Enforce_Gravity_ (0x7b45c). Ultra rich minerals are
// rolled again unless the race is high-G, into planet[player] (sic).
func (g *Game) enforceGravity(pc int16, grav uint8, player int) {
	s := &g.state
	p := &s.Planets[pc]
	p.Gravity = grav
	if p.Minerals == 4 && s.Players[player].Picks[11] == 0 {
		s.Planets[player].Minerals = g.generateMineralClass(int(p.Star))
	}
}

// initHomeworldColony2 is Init_Homeworld_Colony2_ (0x13a3d) (and
// Init_Homeworld_Colony_ with home set): the colony, its pops and
// buildings, and on average tech the starting ships.
func (g *Game) initHomeworldColony2(planet, owner int, home bool) {
	s := &g.state
	ci := int(s.NumColonies)
	s.NumColonies++
	added := int16(0)
	g.initColony(ci, planet, owner)
	c := &s.Colonies[ci]
	pl := &s.Planets[planet]
	s.Stars[pl.Star].Visited |= 1 << uint(owner&7)
	c.Climate = pl.Climate
	limit := int32(int16(g.colonyRacePopLimit(c, owner)))
	if g.settings[0xd9] == 2 {
		v := (int32(s.Players[owner].Picks[1]) + 0x12c) * (limit * 6) / 0xbb8
		c.PopCount = uint8(v)
		if limit <= 8 {
			c.PopCount = uint8((int32(c.PopCount) + limit) / 2)
		}
		if c.PopCount == 0 {
			c.PopCount = 1
		}
		if home && c.PopCount < 8 {
			c.PopCount = 8
		}
	} else {
		c.PopCount = 8
	}
	for i := int32(0); i < int32(c.PopCount); i++ {
		var job uint32
		if pl.FoodPerFarmer != 0 && s.Players[owner].Picks[18] == 0 {
			job = uint32(i%3) & 3
		} else {
			job = uint32(i%2+1) & 3
		}
		v := uint32(c.Colonists[i])
		v = v&^0x180 | job<<7
		v = v&^0xf | uint32(owner)&0xf
		v = v&^0x70 | (uint32(owner)&7)<<4
		v = v&^0x600 | 0x200
		c.Colonists[i] = v
	}
	c.Buildings = [49]uint8{}
	if s.Players[c.Owner].Picks[0] < 6 && home {
		g.addBuilding(ci, 9)
	}
	lim := int16(g.exeU8(homeBuildingLimit + uint32(g.settings[0xd9])))
	for k := 0; ; k++ {
		b := g.exeI16(homeStartBuildings + uint32(2*k))
		if b == 0 || lim <= added || int32(added)*3/2 >= int32(c.PopCount) {
			break
		}
		if g.colonyCanBuildProduct(ci, b) == 0 {
			continue
		}
		p := &s.Players[owner]
		ok := true
		switch b {
		case 0x14, 0x1f:
			ok = int32(p.Picks[0])/2 != 3
		case 0x15, 0x25, 0x2b:
			ok = p.Picks[18] == 0
		case 5, 0xd, 0x20:
			ok = p.Picks[23] == 0
		case 0xa, 0x19:
			ok = int32(c.PopCount) < limit
		}
		if ok {
			added++
			g.addBuilding(ci, int(b))
		}
	}
	if g.settings[0xd9] == 1 && home {
		s.Ships[s.NumShips].Owner = int8(owner)
		g.createShip(owner, 0, ci)
		s.Ships[s.NumShips].Owner = int8(owner)
		g.createShip(owner, 0, ci)
		s.Ships[s.NumShips].Owner = int8(owner)
		g.loadColonyShipDesign(owner, &s.Players[owner].Designs[5])
		g.createShip(owner, 5, ci)
	}
	c = &s.Colonies[ci]
	c.FoodPerFarmer = s.Planets[c.Planet].FoodPerFarmer
	c.CanFarm = s.Planets[c.Planet].MaxFarms
	c.Climate = s.Planets[c.Planet].Climate
	c.Marines = 3
	c.Output = [4]int16{}
	c.Upkeep = [4]uint8{}
}

// guaranteePlanetAtMinDist is Guarantee_A_Planet_At_Min_Dist_ (0x7c107): a
// star with a planet within dist parsecs of the home star; else the
// nearest star that is no home gets two planets (if it has none) and is
// moved closer.
func (g *Game) guaranteePlanetAtMinDist(home int, dist int16, homes []int16) {
	s := &g.state
	type entry struct{ dist, star int16 }
	list := make([]entry, s.NumStars)
	for i := range list {
		list[i] = entry{0x3e7, 0x3e7}
	}
	found := false
	for b := 1; b < int(s.NumStars) && !found; b++ {
		if b == home {
			continue
		}
		list[b].star = int16(b)
		list[b].dist = int16(g.parsecsBetweenStars(home, b))
		if s.starHasPlanet(b) && list[b].dist <= dist {
			found = true
		}
	}
	if found {
		return
	}
	clib.Qsort(list, 4, func(x, y *entry) int {
		switch {
		case y.dist == x.dist:
			return 0
		case y.dist > x.dist:
			return -1
		}
		return 1
	})
	di := int16(0)
	var si int16
	for {
		si = list[di].star
		free := true
		for a := int16(0); a < s.NumPlayers; a++ {
			if si == homes[a] {
				free = false
				break
			}
		}
		if free {
			break
		}
		di++
		if di >= s.NumStars {
			break
		}
	}
	if !s.starHasPlanet(int(si)) {
		st := &s.Stars[si]
		st.SpectralClass = 4
		switch r := int16(s.Random.Random(10)) - 1; {
		case r < 3:
			st.Size = 0
		case r < 7:
			st.Size = 1
		default:
			st.Size = 2
		}
		st.Picture = uint8(s.Random.Random(3)) - 1
		for k := range st.Planets {
			st.Planets[k] = -1
		}
		for k := 0; k < 2; k++ {
			o := g.generateOrbit(int(si))
			st.Planets[o] = s.PlanetCount
			p := &s.Planets[s.PlanetCount]
			p.Type = 3
			p.Orbit = o
			g.generateSatellite(int(si), o, 3)
			s.PlanetCount++
		}
	}
	if di >= s.NumStars {
		return
	}
	h := &s.Stars[home]
	for {
		st := &s.Stars[si]
		dx, dy := coordMovement(st.X, st.Y, h.X, h.Y, 10)
		st.X += dx
		st.Y += dy
		if int16(g.parsecsBetweenStars(home, int(si))) <= dist {
			break
		}
	}
}

// starHasPlanet is Star_Has_Planet_ (0x7c8e6): a type 3 planet, no black
// hole.
func (s *State) starHasPlanet(star int) bool {
	if s.Stars[star].SpectralClass == 6 {
		return false
	}
	for _, pi := range s.Stars[star].Planets {
		if pi > -1 && s.Planets[pi].Type == 3 {
			return true
		}
	}
	return false
}

// ensureAtLeast1FoodPlanet is Ensure_At_Least_1_Food_Planet_ (0x7b631):
// a star with a food planet within 6 parsecs of every colony (of a race
// that eats), noted in _nearest_food_worlds.
func (g *Game) ensureAtLeast1FoodPlanet() bool {
	s := &g.state
	var list [0x168]int16
	for i := range list {
		list[i] = -1
	}
	count := int16(0)
	ok := true
	for ci := int32(0); int16(ci) < s.NumColonies; ci++ {
		col := &s.Colonies[ci]
		esi := int16(-1)
		owner := int16(col.Owner)
		if col.Planet <= -1 || s.Players[owner].Picks[18] != 0 {
			continue
		}
		found := int8(0)
		best := int16(0x3e7)
		empty, near := int16(-1), int16(-1)
		home := int(s.Planets[col.Planet].Star)
		for st := 1; st < int(s.NumStars) && found == 0; st++ {
			if st == home {
				continue
			}
			d := int16(g.parsecsBetweenStars(home, st))
			if d < best {
				best = d
				if d <= 6 {
					near = int16(st)
				}
			}
			if d > 6 {
				continue
			}
			if s.nPlanetsInSystem(st) == 0 {
				if s.Stars[st].SpectralClass != 6 {
					empty = int16(st)
				}
				continue
			}
			found = s.nFoodPlanetsInSystem(st)
			if found == 0 {
				for _, pi := range s.Stars[st].Planets {
					if pi > -1 {
						list[count] = pi
						count++
					}
				}
			} else if esi == -1 {
				esi = int16(st)
			}
		}
		switch {
		case found != 0:
			if count > 0 && esi == -1 {
				esi = int16(s.Planets[list[0]].Star)
			}
		case empty > -1:
			o := g.usableOrbit(int(empty))
			g.makeOnePlanet(int(empty), s.PlanetCount, o, true)
			s.PlanetCount++
			esi = empty
		case near > -1:
			saved := s.PlanetCount
			pi := list[int16(s.Random.Random(uint32(int32(count))))-1]
			spec := s.Planets[pi].Special
			star := int(s.Planets[pi].Star)
			s.PlanetCount = pi
			o := int16(-1)
			for k, v := range s.Stars[star].Planets {
				if v == pi {
					o = int16(k)
					break
				}
			}
			g.makeOnePlanet(star, pi, o, true)
			s.Planets[pi].Special = spec
			esi = int16(star)
			s.PlanetCount = saved
		}
		g.gen.nearestFood[owner] = esi
		if esi == -1 && s.Players[owner].Picks[18] == 0 {
			ok = false
		}
	}
	return ok
}
