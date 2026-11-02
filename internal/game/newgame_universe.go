package game

import (
	"strings"

	"github.com/openorion2/openorion2/internal/save"
)

// The galaxy: Universe_Generation_ (MOX\HAROLD\UNIVERSE.C) and its stars,
// planets, nebulas and specials. Spec: docs/newgame/generation.md.

const (
	starNameCount      = 0x33d
	starNameLBX        = 0x1798d1 // "starname.lbx", entry 1: 0x33d names of 0xf bytes
	starNameFmt        = 0x1798c0 // "(%s)"
	starNameShortExtra = 0x1798c5 // "w", added to names shorter than 7
	starBgLBX          = 0x179845 // "starbg.lbx"
	firstTimeMapInit   = 0x182246 // _first_time_map_is_generated's initial value
	orbitWeights       = 0x8bb39  // Generate_Orbit_'s table in the code: 5 orbits by 3 ages
	blackHoleFixSize   = 0x8bb4e  // Black_Hole_Fix_'s nebula test offset (a code constant)

	tblClassToSatellites = 0x17d680 // _class_to_num_satellites[roll*6 + class]
	tblOrbitToSatType    = 0x17d6bc // _orbit_to_satellite_type[roll*5 + orbit]
	tblClassToMineral    = 0x17d6ee // [roll*6 + class]
	tblGravity           = 0x17d72a // [minerals*5 + size]
	tblClassToGroup      = 0x17d761 // [class*5 + orbit]
	tblNormalClimate     = 0x17d7a7 // [climate*4 + group]
	tblOldClimate        = 0x17d7cf
	tblPlanetSize        = 0x17d7f7
	tblPlanetMaxFarms    = 0x17d7fc
	tblPlanetSpecial     = 0x17d826 // percent of stars without a special
	tblSpecialWeights    = 0x17d832 // _planet_special_weighted_chance
	tblStarClass         = 0x17d83e // _star_class_table[class*3 + age]

	// Command line switches of the 1.40 patch, 0 unless given (0xff).
	patchNoWormholes = 0x1783d3 // /nowh
	patchNoBlackHole = 0x1783da // /nobh
	patchNoSplint    = 0x178461 // /nosplint, bit 0 toggled by the special hooks
	patchNoNebula    = 0x178749 // /nonebula
)

// genState holds HAROLD's generation globals.
type genState struct {
	galaxySize, galaxyAge int16 // _g_galaxy_size, _g_galaxy_age
	nMapsGenerated        uint8 // _n_maps_generated
	makeFoodPlanet        bool  // _make_food_planet
	companionStar         int16 // _companion_star
	names                 [starNameCount][0xf]byte
	nameIndex             [starNameCount]int16
	nMarooned, nWormholes uint8 // _n_marooned_heroes, _n_wormholes
	nSpaceMonsters        int8  // _n_space_monsters
	specialCount          [12]uint8
	tRoll                 int16
	goodStars             []starDist // _good_stars, Build_Min_Star_Distances_
	nearestFood           [8]int16   // _nearest_food_worlds
}

// generateUniverse is Universe_Generation_ (0x8dae8): stars, planets and
// nebulas until the map is connected. The progress screen is not drawn.
func (g *Game) generateUniverse(size, age, difficulty int16) error {
	s := &g.state
	gs := &g.gen
	gs.galaxySize, gs.galaxyAge = size, age
	gs.nMapsGenerated = 0
restart:
	gs.makeFoodPlanet = false
	gs.nMapsGenerated++
	g.mapMarginX, g.mapMarginY = 0x25, 0x25
	s.Planets = [maxPlanets]save.Planet{}
	gs.companionStar = 0
	a, err := g.archive(strings.ToUpper(cStringAt(g.exe, starNameLBX)))
	if err != nil {
		return err
	}
	t, err := a.Table(1)
	if err != nil {
		return err
	}
	for i := range gs.names {
		copy(gs.names[i][:], t.Record(i))
	}
	g.randomizeStarNames()
	if err := g.universeProgressSetup(); err != nil {
		return err
	}
	if !g.firstMapDone {
		s.Random.Set(g.settings.UniverseSeed())
		g.firstMapDone = true
	}
	var cols, rows int16
	switch size {
	case 0:
		s.NumStars, cols, rows = 0x14, 5, 4
	case 1:
		s.NumStars, cols, rows = 0x24, 6, 6
	case 2:
		s.NumStars, cols, rows = 0x36, 9, 6
	case 3:
		s.NumStars, cols, rows = 0x47, 9, 8
	}
	g.initMainScreenMap()
	s.PlanetCount = 0
	g.generateNumberOfNebulas(size)
	if g.exeU8(patchNoNebula) != 0 {
		s.NumNebulas = 0
	}
	if s.NumNebulas > 0 {
		if err := g.generateNebulas(); err != nil {
			return err
		}
	}
	gs.nSpaceMonsters = 0
	gs.specialCount = [12]uint8{}
	s.NumColonies = 0
	s.PlanetCount = 0
	gs.nMarooned, gs.nWormholes = 0, 0
	s.Planets = [maxPlanets]save.Planet{}
	s.Stars = [maxStars]save.Star{}
	var sats [maxStars]uint8
	if g.setStarXYs(cols, rows, age, true, true) {
		if err := g.genProgressStep(s.NumStars*2, true); err != nil {
			return err
		}
		goto restart
	}
	for i := 0; i < int(s.NumStars); i++ {
		st := &s.Stars[i]
		for k := range st.Relocation {
			st.Relocation[k] = -1
		}
		for k := range st.Officer {
			st.Officer[k] = -1
		}
		st.WormholeTo = -1
		if err := g.genProgressStep(s.NumStars*2, false); err != nil {
			return err
		}
		st.Owner = -1
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
		gs.companionStar = 0
		if st.SpectralClass < 6 {
			sats[i] = g.generateNumberOfSatellites(i)
			for k := 0; k < int(sats[i]); k++ {
				g.planetGeneration(i)
			}
		}
		if st.SpectralClass != 6 && sats[i] > 0 && i > 0 {
			var worm int16
			g.generateStarSpecial(i, &worm)
		}
		st.WormholeTo = -1
	}
	for ok := false; !ok; {
		if s.NumNebulas > 0 {
			g.blackHoleFix(age)
		}
		g.initializeBlackHoleBlocks(0x2d)
		if g.mapIsConnected() {
			ok = true
		} else {
			g.setStarXYs(cols, rows, age, false, false)
		}
	}
	return nil
}

// randomizeStarNames is Randomize_Star_Names_ (0x8d602): 0x33d swaps.
func (g *Game) randomizeStarNames() {
	r := &g.state.Random
	ni := &g.gen.nameIndex
	for i := range ni {
		ni[i] = int16(i)
	}
	for c := 0; c < starNameCount; c++ {
		a := int16(r.Random(starNameCount)) - 1
		b := int16(r.Random(starNameCount)) - 1
		if a != b {
			ni[a], ni[b] = ni[b], ni[a]
		}
	}
}

// initMainScreenMap is Init_Main_Screen_ (0x8cf09) with the 1.40 patch's
// hook (0x1693b6): the map size and scales by the number of stars, or the
// Cluster map.
func (g *Game) initMainScreenMap() {
	s := &g.state
	if g.patchByte(patchCluster)&1 != 0 {
		s.MapMaxX, s.MapMaxY = 0x3f3, 0x320
		s.CurMapScale, s.MaxMapScale = 0xf, 0x14
		g.ms.zoomCount, s.MaxZoomCount = 3, 3
		return
	}
	var size int16
	switch n := s.NumStars; {
	case n <= 0x14:
		size = 0
	case n <= 0x24:
		size = 1
	case n <= 0x36:
		size = 2
	case n <= 0x48:
		size = 3
	default:
		size = int16(g.settings[0xd6])
	}
	switch size {
	case 0:
		s.MapMaxX, s.MapMaxY, s.CurMapScale, s.MaxMapScale = 0x1fa, 0x190, 0xa, 0xa
		s.MaxZoomCount, g.ms.zoomCount = 0, 0
	case 1:
		s.MapMaxX, s.MapMaxY, s.CurMapScale, s.MaxMapScale = 0x2f7, 0x258, 0xf, 0xf
		s.MaxZoomCount, g.ms.zoomCount = 1, 1
	case 2:
		s.MapMaxX, s.MapMaxY, s.CurMapScale, s.MaxMapScale = 0x3f4, 0x320, 0x14, 0x14
		g.ms.zoomCount, s.MaxZoomCount = 2, 2
	case 3:
		s.MapMaxX, s.MapMaxY, s.CurMapScale, s.MaxMapScale = 0x5ee, 0x4b0, 0x1e, 0x1e
		s.MaxZoomCount, g.ms.zoomCount = 3, 3
	}
}

// scaled16 is Get_Scaled_Value_ (0x7926c) as its callers keep it, a word.
func (g *Game) scaled16(v int16) int16 { return int16(g.scaled(v)) }

// upScaled16 is Get_Up_Scaled_Value_ (0x7927f) as a word.
func (g *Game) upScaled16(v int16) int16 { return int16(g.upScaled(v)) }

// generateNumberOfNebulas is Generate_Number_Of_Nebulas_ (0x8c4d3).
func (g *Game) generateNumberOfNebulas(size int16) {
	s := &g.state
	r := &s.Random
	s.NumNebulas = 0
	switch size {
	case 0:
		s.NumNebulas = uint8(r.Random(2)) - 1
	case 1:
		s.NumNebulas = uint8(r.Random(2))
	case 2:
		s.NumNebulas = uint8(r.Random(3))
	case 3:
		s.NumNebulas = uint8(r.Random(3)) + 1
	}
}

// nebulaPictureSize is the width and height of STARBG.LBX picture entry.
func (g *Game) nebulaPictureSize(entry int) (int16, int16, error) {
	a, err := g.archive(strings.ToUpper(cStringAt(g.exe, starBgLBX)))
	if err != nil {
		return 0, 0, err
	}
	e, err := a.Entry(entry)
	if err != nil {
		return 0, 0, err
	}
	return int16(uint16(e[0]) | uint16(e[1])<<8), int16(uint16(e[2]) | uint16(e[3])<<8), nil
}

// generateNebulas is Generate_Nebulas_ (0x8c099): the nebulas' types (one
// draw each: the taken flags it tests are never set) and places at least
// a minimum distance apart, the distance shrinking after failures.
func (g *Game) generateNebulas() error {
	s := &g.state
	r := &s.Random
	var w, h [12]int16
	for k := 0; k < 12; k++ {
		var err error
		if w[k], h[k], err = g.nebulaPictureSize(6 + 4*k); err != nil {
			return err
		}
	}
	var types [4]int16
	for n := 0; n < int(s.NumNebulas); n++ {
		types[n] = int16(r.Random(12)) - 1
	}
	margin := int16(0x14)
	tries := int16(0)
	var pts []point16
	// The bounds are computed once; failures shrink only the distance.
	d1, d2 := g.scaled16(0xa5), g.scaled16(0x82)
	mx, my, ymax, xmax := margin+d1, margin+d2, 0x190-d2, 0x1fa-d1
	for {
		tries++
		if tries > 0x64 {
			s.NumNebulas = 1
			break
		}
		x := int16(r.Random(uint32(int16(0x1fa - w[0]))))
		y := int16(r.Random(uint32(int16(0x190 - h[0]))))
		s.Nebulas[0].Picture = uint8(types[0])
		s.Nebulas[0].X = g.upScaled16(x)
		s.Nebulas[0].Y = g.upScaled16(y)
		if s.NumNebulas <= 1 {
			break
		}
		d := (int32(mx)+int32(margin))*(int32(mx)+int32(margin)) + (int32(my)+int32(margin))*(int32(my)+int32(margin))
		pts = g.buildMinPointDistances(x, y, 10, 10, xmax, ymax, d, 30000)
		if int16(s.NumNebulas) > int16(len(pts)) {
			margin -= 10
			continue
		}
		kept := []point16{{x, y}}
		weights := []uint16{1}
		full := false
		for i := 0; i < len(pts) && !full; i++ {
			ok := true
			for j := 0; j < len(kept) && !full && ok; j++ {
				p, q := pts[i], kept[j]
				if p == q {
					continue
				}
				if distBetweenPoints(p.x, p.y, q.x, q.y) < d {
					ok = false
				}
			}
			if ok {
				kept = append(kept, pts[i])
				weights = append(weights, 1)
				if len(kept) >= 10000 {
					full = true
				}
			}
		}
		if int16(s.NumNebulas) <= int16(len(kept)) {
			for n := 0; n < int(s.NumNebulas); n++ {
				i := getWeightedChoiceShort(r, weights)
				s.Nebulas[n].Picture = uint8(types[n])
				s.Nebulas[n].X = g.upScaled16(kept[i].x)
				s.Nebulas[n].Y = g.upScaled16(kept[i].y)
				weights[i] = 0
			}
			break
		}
		if tries%20 == 0 {
			margin -= 10
		}
	}
	return g.loadNebulaPictures()
}

type point16 struct{ x, y int16 }

// distBetweenPoints is Dist_Between_Points_ (0x7b0fb), squared.
func distBetweenPoints(x1, y1, x2, y2 int16) int32 {
	dx, dy := int32(x1)-int32(x2), int32(y1)-int32(y2)
	return dx*dx + dy*dy
}

// randomizeList is Randomize_List_ (0x79f26) without its clock variant:
// n*3/2 random swaps.
func (g *Game) randomizeList(list []int16) {
	r := &g.state.Random
	n := int32(int16(len(list)))
	for c := int16(0); int32(c) < n*3/2; c++ {
		a := int16(r.Random(uint32(n))) - 1
		b := int16(r.Random(uint32(n))) - 1
		if b != a {
			list[a], list[b] = list[b], list[a]
		}
	}
}

// buildMinPointDistances is Build_Min_Point_Distances_ (0x8e341): up to max
// points of the shuffled grid at least d (squared) from (px, py).
func (g *Game) buildMinPointDistances(px, py, xmin, ymin, xmax, ymax int16, d int32, max int) []point16 {
	var xs [0x280]int16
	var ys [0x1e0]int16
	for i := range xs {
		xs[i] = -1
	}
	for i := range ys {
		ys[i] = -1
	}
	for v := xmin; v < xmax; v++ {
		xs[v] = v
	}
	for v := ymin; v < ymax; v++ {
		ys[v] = v
	}
	g.randomizeList(xs[:])
	g.randomizeList(ys[:])
	var out []point16
	for i := 0; i < len(xs) && len(out) < max; i++ {
		for j := 0; j < len(ys) && len(out) < max; j++ {
			if xs[i] > -1 && ys[j] > -1 && distBetweenPoints(px, py, xs[i], ys[j]) >= d {
				out = append(out, point16{xs[i], ys[j]})
			}
		}
	}
	return out
}

// starNameWidth is the width Set_Star_XYs_ keeps free for a star's name:
// "(name)", with "w" for names shorter than 7, in the player font style.
func (g *Game) starNameWidth(name string) int16 {
	buf := cSprintf(cStringAt(g.exe, starNameFmt), name)
	if len(name) < 7 {
		buf += cStringAt(g.exe, starNameShortExtra)
	}
	g.setPlayerFontStyle(0, false)
	return int16(g.e.Text().Width([]byte(buf)))
}

// setStarXYs is Set_Star_XYs_ (0x8cfff): each star in its grid cell, apart
// from the others; with spectral new spectral classes, with names the
// names from the shuffled list. It reports a star it could not place.
func (g *Game) setStarXYs(cols, rows, age int16, spectral, names bool) bool {
	s := &g.state
	r := &s.Random
	cellW := int16(int32(s.MapMaxX) / int32(cols))
	cellH := int16(int32(s.MapMaxY) / int32(rows))
	maxX := g.scaled16(s.MapMaxX)
	maxY := g.scaled16(s.MapMaxY)
	var minDist int16
	var extra int32
	switch s.CurMapScale {
	case 0xa, 0xf:
		minDist, extra = 0x32, 8
	case 0x14:
		minDist, extra = 0x24, 6
	case 0x1e:
		minDist, extra = 0x1e, 4
	}
	threeRandom := func() int16 {
		v := int32(r.Random(8)) + 0x20
		v += int32(r.Random(8))
		return int16(v + int32(r.Random(8)))
	}
	for i := int32(0); int16(i) < s.NumStars; i++ {
		if err := g.genProgressStep(s.NumStars*2, false); err != nil {
			g.Log("generation progress: %v", err)
		}
		st := &s.Stars[i]
		baseX := int32(cellW) * (i % int32(cols))
		baseY := int32(cellH) * i / int32(cols)
		if spectral {
			st.SpectralClass = g.patchedSpectralClass(age)
		}
		var w int16
		if names {
			strcpyField(st.Name[:], save.CString(g.gen.names[g.gen.nameIndex[i]][:]))
			w = g.starNameWidth(save.CString(st.Name[:]))
			if g.gen.nMapsGenerated >= 3 {
				w = threeRandom()
			}
		} else {
			w = threeRandom()
		}
		tries := int16(0)
		for {
			var x, sx, y, sy int16
			for {
				c := baseX + int32(r.Random(uint32(int32(cellW)*2)))
				x = int16(int32(int16(c)) % int32(s.MapMaxX))
				sx = g.scaled16(x)
				if sx >= 0x18 && int32(sx) <= int32(maxX)-int32(g.mapMarginX)/2 {
					break
				}
			}
			st.X = x
			for {
				c := baseY + int32(r.Random(uint32(int32(cellH)*2)))
				y = int16(int32(int16(c)) % int32(s.MapMaxY))
				sy = g.scaled16(y)
				if sy >= 0x16 && int32(sy) <= int32(maxY)-(extra+int32(g.mapMarginY)/2) {
					break
				}
			}
			st.Y = y
			if !g.starXYInvalid(int(i), w, minDist, sx) {
				break
			}
			tries++
			if tries <= 0x96 {
				continue
			}
			tries = 0
			for {
				fx := int16(r.Random(0x189)) + 0x14
				fy := int16(r.Random(0x155)) + 0x14
				st.X = g.upScaled16(fx)
				st.Y = g.upScaled16(fy)
				if !g.starXYInvalid(int(i), w, minDist, fx) {
					break
				}
				tries++
				if tries > 0x96 {
					return true
				}
			}
			break
		}
	}
	return false
}

// patchedSpectralClass is Generate_Spectral_Class_ (0x8c807) with the 1.40
// patch's hook (0xf4b41): /nobh turns black holes into a random class.
func (g *Game) patchedSpectralClass(age int16) uint8 {
	c := g.generateSpectralClass(age)
	if c == 6 && g.exeU8(patchNoBlackHole) == 0xff {
		c = uint8(g.state.Random.Random(6)) - 1
	}
	return c
}

// generateSpectralClass is Generate_Spectral_Class_ (0x8c807).
func (g *Game) generateSpectralClass(age int16) uint8 {
	var w [7]uint8
	for k := range w {
		w[k] = g.exeU8(tblStarClass + uint32(k*3) + uint32(age))
	}
	return uint8(getWeightedChoiceChar(&g.state.Random, w[:]))
}

// starXYInvalid is Star_XY_Invalid_ (0x8d43d): whether star i is too close
// to an earlier star or its name would leave the map.
func (g *Game) starXYInvalid(i int, w, minDist, sx int16) bool {
	s := &g.state
	maxX := int32(g.scaled16(s.MapMaxX))
	si := &s.Stars[i]
	for j := 0; j < i; j++ {
		sj := &s.Stars[j]
		dx := int32(g.scaled16(sj.X)) - int32(g.scaled16(si.X))
		dy := int32(g.scaled16(sj.Y)) - int32(g.scaled16(si.Y))
		d2 := dx*dx + dy*dy
		if sj.SpectralClass == 6 || si.SpectralClass == 6 {
			if g.parsecsBetweenStars(j, i) < 5 {
				return true
			}
			continue
		}
		t := int32(g.scaled16(0x3c))
		a := d2 <= 0x320
		b := abs32(dy) < int32(minDist) && abs32(dx) < int32(w)
		c := abs32(dy) < 0xf && abs32(dx) < int32(int16(t))
		d := int32(w)*2/3+int32(sx) > maxX
		if a || b || c || d {
			return true
		}
	}
	return false
}

// generateNumberOfSatellites is Generate_Number_Of_Satellites_ (0x8c527).
func (g *Game) generateNumberOfSatellites(star int) uint8 {
	r := int32(g.state.Random.Random(10)) - 1
	c := g.state.Stars[star].SpectralClass
	if c >= 6 {
		return 0
	}
	return g.exeU8(uint32(int32(tblClassToSatellites) + r*6 + int32(c)))
}

// planetGeneration is Planet_Generation_ (0x8e280): one orbit of the star,
// a planet when it holds asteroids, a gas giant or a planet.
func (g *Game) planetGeneration(star int) {
	s := &g.state
	orbit := g.generateOrbit(star)
	typ := g.generateSatelliteType(star, orbit)
	if typ != 1 && typ != 2 && typ != 3 {
		return
	}
	pc := s.PlanetCount
	s.Stars[star].Planets[orbit] = pc
	s.Planets[pc].Orbit = orbit
	s.Planets[pc].Type = uint8(typ)
	g.generateSatellite(star, orbit, typ)
	s.PlanetCount++
}

// generateOrbit is Generate_Orbit_ (0x8c567): a free orbit, weighted by age.
func (g *Game) generateOrbit(star int) uint8 {
	var w [5]uint8
	for k := range w {
		w[k] = g.exeU8(orbitWeights + uint32(k*3) + uint32(g.settings[0xd7]))
	}
	for {
		o := getWeightedChoiceChar(&g.state.Random, w[:])
		if g.state.Stars[star].Planets[o] == -1 {
			return uint8(o)
		}
	}
}

// generateSatelliteType is Generate_Satellite_Type_ (0x8c6fe): 1 asteroids,
// 2 gas giant, 3 planet, 4+ a companion star (no planet); no gas giant in
// the first orbit.
func (g *Game) generateSatelliteType(star int, orbit uint8) int16 {
	s := &g.state
	var t int8
	for done := false; !done; {
		roll := int8(s.Random.Random(10)) - 1
		g.gen.tRoll = int16(roll)
		t = int8(g.exeU8(uint32(int32(tblOrbitToSatType) + int32(roll)*5 + int32(orbit))))
		if t == 4 {
			x := int8(s.Random.Random(100))
			if roll == 1 && x < 0xb && orbit == 0 {
				g.gen.companionStar = 1
				if c := s.Stars[star].SpectralClass; c == 0 {
					t = 5
				} else {
					t = int8(c) + 4
				}
			} else {
				t = 1
			}
		}
		done = t != 2 || orbit != 0
	}
	if t == 0 {
		g.Log("Generate_Satellite_Type_: type 0 (the original waits for a key)")
	}
	return int16(t)
}

// generateSatellite is Generate_Satellite_ (0x8c5d7): the planet's
// properties.
func (g *Game) generateSatellite(star int, orbit uint8, typ int16) {
	s := &g.state
	p := &s.Planets[s.PlanetCount]
	p.Star = uint8(star)
	p.Size = g.generateSize()
	p.Minerals = g.generateMineralClass(star)
	p.Gravity = g.generateGravityClass(int(s.PlanetCount))
	grp := g.getPlanetGroup(int(s.PlanetCount))
	p.ClimateGroup = uint8(grp)
	p.Climate = g.generateClimate(grp)
	p.Picture = uint8(s.Random.Random(3)) - 1
	g.setSecondaryPlanetData(star, int(s.PlanetCount))
}

// generateSize is Generate_Size_ (0x8c7d1); 0xff when no row matches.
func (g *Game) generateSize() uint8 {
	r := int16(g.state.Random.Random(10))
	for i := 0; i < 5; i++ {
		if r <= int16(g.exeU8(tblPlanetSize+uint32(i))) {
			return uint8(i)
		}
	}
	return 0xff
}

// generateMineralClass is Generate_Mineral_Class_ (0x8c05b).
func (g *Game) generateMineralClass(star int) uint8 {
	r := int16(g.state.Random.Random(10)) - 1
	c := int8(g.state.Stars[star].SpectralClass)
	if c < 0 || c >= 6 {
		return 0xff
	}
	return g.exeU8(uint32(int32(tblClassToMineral) + int32(r)*6 + int32(c)))
}

// generateGravityClass is Generate_Gravity_Class_ (0x8bfe0).
func (g *Game) generateGravityClass(pc int) uint8 {
	p := &g.state.Planets[pc]
	if p.Minerals >= 5 {
		return 0
	}
	return g.exeU8(tblGravity + uint32(p.Minerals)*5 + uint32(p.Size))
}

// getPlanetGroup is Get_Planet_Group_ (0x8cd55): the climate group by the
// star's class and the orbit (two lower beside a companion star).
func (g *Game) getPlanetGroup(pc int) int16 {
	s := &g.state
	p := &s.Planets[pc]
	o := int16(p.Orbit)
	c := int16(s.Stars[p.Star].SpectralClass)
	if g.gen.companionStar == 1 {
		o -= 2
		if o < 0 {
			return 0
		}
	}
	if c >= 6 {
		return 0
	}
	return int16(g.exeU8(uint32(int32(tblClassToGroup) + int32(c)*5 + int32(o))))
}

// generateClimate is Generate_Climate_ (0x8beab).
func (g *Game) generateClimate(grp int16) uint8 {
	var w [10]uint8
	for i := 0; i < 10; i++ {
		switch g.gen.galaxyAge {
		case 0, 1:
			w[i] = g.exeU8(uint32(int32(tblNormalClimate) + int32(i)*4 + int32(grp)))
		case 2:
			w[i] = g.exeU8(uint32(int32(tblOldClimate) + int32(i)*4 + int32(grp)))
		}
	}
	c := int8(-1)
	if g.gen.makeFoodPlanet {
		n, last := 0, int8(0)
		for i := int8(0); i < 10; i++ {
			if w[i] != 0 && g.exeU8(tblFoodPerFarmer+uint32(i)) > 0 {
				n++
				last = i
			}
		}
		if n == 1 {
			c = last
		}
	}
	if c == -1 {
		for {
			c = int8(getWeightedChoiceChar(&g.state.Random, w[:]))
			if !g.gen.makeFoodPlanet || g.exeU8(uint32(int32(tblFoodPerFarmer)+int32(c))) != 0 {
				break
			}
		}
	}
	return uint8(c)
}

// setSecondaryPlanetData is Set_Secondary_Planet_Data_ (0x8c6c0).
func (g *Game) setSecondaryPlanetData(star, pc int) {
	p := &g.state.Planets[pc]
	p.MaxFarms = g.generateMaxFarms(star, pc)
	p.FoodPerFarmer = g.generateFoodPerFarmer(star, pc)
	p.Colony = -1
}

// generateMaxFarms is Generate_Max_Farms_ (0x8c01e).
func (g *Game) generateMaxFarms(star, pc int) uint8 {
	if g.state.Stars[star].SpectralClass >= 6 && g.state.Stars[star].Special != 0xb {
		return 0
	}
	return g.exeU8(tblPlanetMaxFarms + uint32(g.state.Planets[pc].Size))
}

// generateFoodPerFarmer is Generate_Food_Per_Farmer_ (0x8bfa3).
func (g *Game) generateFoodPerFarmer(star, pc int) uint8 {
	if g.state.Stars[star].SpectralClass >= 6 && g.state.Stars[star].Special != 0xb {
		return 0
	}
	return g.exeU8(tblFoodPerFarmer + uint32(g.state.Planets[pc].Climate))
}

// generateStarSpecial is Generate_Star_Special_ (0x8cb93), with the 1.40
// patch's /nosplint hook around Get_A_Planet_Or_Star_Special_.
func (g *Game) generateStarSpecial(star int, worm *int16) {
	r := &g.state.Random
	var special uint8
	if int16(r.Random(100)) > int16(g.exeU8(tblPlanetSpecial)) {
		g.getPlanetOrStarSpecial(star, worm, &special, true, 0, g.exeU8(patchNoSplint)|1 == 0xff)
	}
	d := int32(5)
	switch g.settings[0xd4] {
	case 3:
		d = 8
	case 4:
		d = 0xc
	}
	if int32(r.Random(100))-1 < d {
		g.gen.nSpaceMonsters++
	}
	g.gen.specialCount[special]++
}

// getPlanetOrStarSpecial is Get_A_Planet_Or_Star_Special_ (0x8c840): a
// weighted special for the star or one of its planets; apply writes it,
// mask excludes specials; splint is the patch's /nosplint (7 becomes 4).
func (g *Game) getPlanetOrStarSpecial(star int, worm *int16, special *uint8, apply bool, mask int32, splint bool) {
	s := &g.state
	st := &s.Stars[star]
	var w1, w2, w3 [5]uint16
	bigHab, hab := false, false
	for o := 0; o < 5; o++ {
		pi := st.Planets[o]
		if pi <= -1 {
			continue
		}
		p := &s.Planets[pi]
		if p.Type != 3 || p.Special != 0 {
			continue
		}
		w1[o] = 1
		switch p.Climate {
		case 3, 7, 4, 5, 6, 8, 9:
			w3[o] = 1
			hab = true
			if p.Size >= 2 {
				bigHab = true
				w2[o] = 1
			}
		}
	}
	var ws [12]uint16
	for k := 1; k < 12; k++ {
		zero := mask>>uint(k)&1 != 0
		if !zero && k == 8 && int32(g.gen.nMarooned) > int32(g.ms.zoomCount)+2 {
			zero = true
		}
		if !zero && (k == 7 || k == 6) && !bigHab {
			zero = true
		}
		if !zero && k == 10 && !hab {
			zero = true
		}
		if !zero && k == 9 && g.userWantsSpaceMonsters == 0 {
			zero = true
		}
		if !zero {
			ws[k] = uint16(g.exeU8(tblSpecialWeights + uint32(k)))
		}
	}
	setPlanet := func(w []uint16) bool {
		o := getWeightedChoiceShort(&s.Random, w)
		if o <= -1 {
			return false
		}
		if apply {
			s.Planets[st.Planets[o]].Special = *special
			st.Special = *special
		}
		return true
	}
	done := false
	for tries := int16(0); !done; {
		c := getWeightedChoiceShort(&s.Random, ws[:])
		if splint && c == 7 {
			c = 4
		}
		*special = uint8(c)
		switch c {
		case 1:
			if apply && int32(g.gen.nWormholes) < int32(g.settings[0xd6])*4+4 {
				*worm = 1
				done = true
				g.gen.nWormholes++
			}
		case 2, 3, 9:
			if apply {
				st.Special = uint8(c)
			}
			done = true
		case 4, 5:
			done = setPlanet(w1[:])
		case 6, 7:
			done = setPlanet(w2[:])
		case 8:
			g.gen.nMarooned++
			if apply {
				st.Special = uint8(c)
			}
			done = true
		case 10:
			done = setPlanet(w3[:])
		}
		tries++
		if tries > 0x64 {
			done = true
			*special = 0
		}
	}
}

// blackHoleFix is Black_Hole_Fix_ (0x8d8a1): a black hole touching a
// nebula becomes a habitable star with one planet.
func (g *Game) blackHoleFix(age int16) {
	s := &g.state
	w := g.exeI16(blackHoleFixSize)
	h := w / 2
	for i := 0; i < int(s.NumStars); i++ {
		st := &s.Stars[i]
		if st.SpectralClass != 6 {
			continue
		}
		x, y := st.X, st.Y
		in := g.pointIsInSomeNebula(x, y) ||
			g.pointIsInSomeNebula(x+w, y) ||
			g.pointIsInSomeNebula(x, y+w) ||
			g.pointIsInSomeNebula(x+w, y+w) ||
			g.pointIsInSomeNebula(x+h, y) ||
			g.pointIsInSomeNebula(x+w, y+h) ||
			g.pointIsInSomeNebula(x+h, y+w) ||
			g.pointIsInSomeNebula(x, y+h)
		if !in {
			continue
		}
		for tries := int16(0); ; {
			st.SpectralClass = g.generateSpectralClass(age)
			if tries > 0xc8 {
				st.SpectralClass = 4
			}
			tries++
			if st.SpectralClass < 6 {
				break
			}
		}
		g.planetGeneration(i)
		st.Visited = 0
	}
}

// initializeBlackHoleBlocks is Initialize_Black_Hole_Blocks_ (0xeb87d): the
// lines between stars that pass within d of a black hole.
func (g *Game) initializeBlackHoleBlocks(d int16) {
	s := &g.state
	bh := s.collectBlackHoles()
	n := int(s.NumStars)
	for i := 0; i < n; i++ {
		s.Stars[i].BlackHoleBlocked = [9]byte{}
	}
	for i := 0; i < n; i++ {
		a := &s.Stars[i]
		if a.SpectralClass == 6 {
			continue
		}
		for j := 0; j < i; j++ {
			b := &s.Stars[j]
			if b.SpectralClass == 6 {
				continue
			}
			for _, k := range bh {
				h := &s.Stars[k]
				if pointWithinDOfSeg(h.X, h.Y, d, a.X, a.Y, b.X, b.Y) {
					a.BlackHoleBlocked[j>>3] |= 1 << uint(j&7)
					b.BlackHoleBlocked[i>>3] |= 1 << uint(i&7)
					break
				}
			}
		}
	}
}

// mapIsConnected is Map_Is_Connected_ (0xebd80): whether every star but
// black holes is reachable in jumps of at most 8 parsecs not crossing a
// black hole.
func (g *Game) mapIsConnected() bool {
	s := &g.state
	n := int(s.NumStars)
	uf := make([]int16, n)
	for i := range uf {
		uf[i] = -1
	}
	for i := 0; i < n; i++ {
		if s.Stars[i].SpectralClass == 6 {
			continue
		}
		for j := i + 1; j < n; j++ {
			if s.Stars[j].SpectralClass == 6 || s.Stars[i].BlackHoleBlocked[j>>3]>>uint(j&7)&1 != 0 {
				continue
			}
			if g.parsecsBetweenStars(i, j) <= 8 {
				g.unionFind(uf, int16(i), int16(j), true)
			}
		}
	}
	roots := 0
	for i := 0; i < n; i++ {
		if s.Stars[i].SpectralClass != 6 && uf[i] == -1 {
			roots++
		}
	}
	return roots == 1
}

// unionFind is Union_Find_ (0xebcb2): the roots of a and b with path
// compression; with join, a coin decides which root goes under the other.
func (g *Game) unionFind(uf []int16, a, b int16, join bool) bool {
	ra := a
	for uf[ra] >= 0 {
		ra = uf[ra]
	}
	rb := b
	for uf[rb] >= 0 {
		rb = uf[rb]
	}
	for x := a; uf[x] >= 0; {
		next := uf[x]
		uf[x] = ra
		x = next
	}
	for x := b; uf[x] >= 0; {
		next := uf[x]
		uf[x] = rb
		x = next
	}
	if join && ra != rb {
		if g.state.Random.Random(2) == 1 {
			uf[ra] = rb
		} else {
			uf[rb] = ra
		}
	}
	return ra == rb
}

// initializeStarInNebulaInfo is Initialize_Star_In_Nebula_Info_ (0xeba96).
func (g *Game) initializeStarInNebulaInfo() {
	s := &g.state
	for i := int(s.NumStars) - 1; i >= 0; i-- {
		s.Stars[i].InNebula = b2u8(g.pointIsInSomeNebula(s.Stars[i].X, s.Stars[i].Y))
	}
}
