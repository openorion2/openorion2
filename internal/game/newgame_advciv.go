package game

import (
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// The advanced tech level's start (MOX\KEN\ADVCIV.C): colonies picked for
// each player, improved planets, officers. Spec: docs/newgame/generation.md.

// advCiv holds the arena blocks Advanced_Civilization_Colonies_ allocates.
// _adv_civ_player is kept as bytes (0x10a0 per player) since the code
// indexes its lists in ways a typed record would hide.
type advCiv struct {
	player    [8][0x10a0]byte // _adv_civ_player
	parsec    [72][72]int32   // _parsec
	selected  [0x168]int8     // _planet_selected_flag: the player a planet goes to
	starOwner [0x48]int8      // _star_colony_owner
	worthN    [8]int8         // _planet_worth_to_player's counts (the list is never read)
}

// _adv_civ_player record: +0 stars in reach, +2 listed planets, +4 chosen,
// +6 farthest star, +8 a flag per planet, +0x170 72 {star, parsecs},
// +0x290 0x168 entries of 10 bytes {flag, proximity, worth, planet, mark}.
const (
	acStars     = 0
	acListed    = 2
	acChosen    = 4
	acFar       = 6
	acFlags     = 8
	acStarList  = 0x170
	acEntries   = 0x290
	acEntrySize = 10
)

func (a *advCiv) i16(p, off int) int16 {
	return int16(binary.LittleEndian.Uint16(a.player[p][off:]))
}

func (a *advCiv) setI16(p, off int, v int16) {
	binary.LittleEndian.PutUint16(a.player[p][off:], uint16(v))
}

func (a *advCiv) i32(p, off int) int32 {
	return int32(binary.LittleEndian.Uint32(a.player[p][off:]))
}

func (a *advCiv) setI32(p, off int, v int32) {
	binary.LittleEndian.PutUint32(a.player[p][off:], uint32(v))
}

// starOwnerAt is _star_colony_owner[st]; st -1 reads the last byte of
// the arena block header before it, 0.
func (a *advCiv) starOwnerAt(st int16) int8 {
	if st == -1 {
		return 0
	}
	return a.starOwner[st]
}

// parsecAt is _parsec[st][star]; row -1 lies before the block: the last
// 0x114 bytes of _adv_civ_player's player 7, then the block header {0,
// 0x5100, 0}.
func (a *advCiv) parsecAt(st int16, star int) int32 {
	if st != -1 {
		return a.parsec[st][star]
	}
	off := -0x120 + 4*star
	if off >= -12 {
		return [3]int32{0, 0x5100, 0}[(off+12)/4]
	}
	return int32(binary.LittleEndian.Uint32(a.player[7][0x10a0-0x114+(off+0x120):]))
}

// entry is the offset of planet list entry k (k may be any int16).
func entry(k int16) int { return acEntries + int(k)*acEntrySize }

// advancedCivilizationColonies is Advanced_Civilization_Colonies_ (0x62c70):
// the RNG goes back to the universe seed, then each player's extra colonies.
func (g *Game) advancedCivilizationColonies() {
	s := &g.state
	s.Random.Set(g.settings.UniverseSeed())
	a := &advCiv{}
	g.adv = a
	for i := range a.selected {
		a.selected[i] = -1
	}
	for i := range a.starOwner {
		a.starOwner[i] = -1
	}
	for i := range a.worthN {
		a.worthN[i] = -1
	}
	for p := 0; p < int(s.NumPlayers); p++ {
		hp := s.Players[p].HomePlanet
		hs := int16(s.Planets[hp].Star)
		for k := int16(0); k < 0x168; k++ {
			a.setI16(p, entry(k)+7, -1)
		}
		for k := 0; k < 0x48; k++ {
			a.setI16(p, acStarList+4*k+2, 0x270f)
			a.setI16(p, acStarList+4*k, -1)
		}
		a.setI16(p, acStarList+4*int(hs), hs)
		a.starOwner[hs] = int8(p)
		e := entry(hp)
		a.player[p][e] = 1
		a.player[p][e+9] = 1
		a.setI32(p, e+3, -0x7ffe)
		a.setI16(p, e+7, hp)
	}
	g.buildParsecStarDistances()
	g.chooseAdvCivPlanets()
	g.twiddleSelectedAdvCivPlanets()
	for k := int16(0); k < s.PlanetCount; k++ {
		if o := a.selected[k]; o != -1 {
			g.initHomeworldColony2(int(k), int(o), false)
		}
	}
	g.adv = nil
}

// buildParsecStarDistances is Build_Parsec_Star_Distances_ (0x62bea); rows
// of stars past the last stay -1.
func (g *Game) buildParsecStarDistances() {
	s := &g.state
	a := g.adv
	for c := 0; c < 0x48; c++ {
		for k := range a.parsec[c] {
			a.parsec[c][k] = -1
		}
		if c >= int(s.NumStars) {
			continue
		}
		for b := 0; b < int(s.NumStars); b++ {
			if c == b {
				a.parsec[b][b] = 0
				continue
			}
			a.parsec[c][b] = g.parsecsBetweenStars(c, b)
		}
	}
}

// advCivLimit is the number of extra colonies a player may get.
func (g *Game) advCivLimit() int16 {
	s := &g.state
	return int16((int32(s.NumStars)/2*10/int32(s.NumPlayers)+9)/10 - 1)
}

// chooseAdvCivPlanets is Choose_Adv_Civ_Planets_ (0x63577): in a random
// order of the players, each takes its best planet in reach until all have
// their share; a round where not all can is undone (by list index, sic).
func (g *Game) chooseAdvCivPlanets() {
	s := &g.state
	a := g.adv
	lim := g.advCivLimit()
	active := s.NumPlayers
	var order [8]int16
	for i := range order {
		order[i] = -1
	}
	for p := int16(0); p < s.NumPlayers; p++ {
		order[p] = p
	}
	for {
		var pick [8]int16
		for i := range pick {
			pick[i] = -1
		}
		g.buildAdvCivStarList()
		g.randomizeList(order[:])
		for si := 0; si < 8; si++ {
			p := order[si]
			if p == -1 {
				continue
			}
			k := g.getNextAdvCivPlanet(int(p))
			if k == -1 {
				active--
				order[p] = -1
				continue
			}
			pi := a.i16(int(p), entry(k)+7)
			pick[p] = k
			a.selected[pi] = int8(p)
			a.starOwner[s.Planets[pi].Star] = int8(p)
		}
		n := int16(0)
		for d := int16(0); d < s.NumPlayers; d++ {
			if pick[d] != -1 {
				n++
			}
		}
		if n != s.NumPlayers {
			for d := int16(0); d < s.NumPlayers; d++ {
				if k := pick[d]; k > -1 && k < 0x168 {
					a.selected[k] = -1
				}
			}
			active = 0
		} else {
			for d := 0; d < int(s.NumPlayers); d++ {
				k := pick[d]
				if k == -1 {
					continue
				}
				listed := a.i16(d, acListed)
				if a.worthN[d] < 0 {
					a.worthN[d] = 0
				}
				a.worthN[d]++
				if a.worthN[d] >= 0x14 {
					a.worthN[d] = 0x13
				}
				a.setI16(d, acChosen, a.i16(d, acChosen)+1)
				e := entry(k)
				a.player[d][e+9] = 1
				a.setI32(d, e+3, -0x7ffe)
				if listed > 1 {
					last := entry(listed - 1)
					copy(a.player[d][e:e+acEntrySize], a.player[d][last:last+acEntrySize])
				}
				a.setI16(d, acListed, a.i16(d, acListed)-1)
				if a.i16(d, acChosen) >= lim {
					active--
					order[d] = -1
				}
			}
		}
		if active <= 0 {
			return
		}
	}
}

// buildAdvCivStarList is Build_Adv_Civ_Star_List_ (0x62e98): the stars
// within 9 parsecs of each player's colonies, then the planets on them and
// their worth.
func (g *Game) buildAdvCivStarList() {
	s := &g.state
	a := g.adv
	far := int16(0)
	for p := 0; p < int(s.NumPlayers); p++ {
		a.setI16(p, acStars, 0)
		hs := s.Planets[s.Players[p].HomePlanet].Star
		for c := 1; c < 0x48; c++ {
			if int32(far) < a.parsec[hs][c] {
				far = int16(a.parsec[hs][c])
			}
		}
		a.setI16(p, acFar, far)
		for c := 1; c < 0x48; c++ {
			if c >= int(s.NumStars) || s.Stars[c].Special == 9 || int16(a.starOwner[c]) != int16(p) {
				continue
			}
			for d := 1; d < int(s.NumStars); d++ {
				dist := int16(a.parsec[c][d])
				if dist > 9 || s.Stars[d].Special == 9 {
					continue
				}
				cur := a.i16(p, acStarList+4*d+2)
				if cur > dist {
					cur = dist
				}
				a.setI16(p, acStarList+4*d+2, cur)
				a.setI16(p, acStarList+4*d, int16(d))
			}
		}
	}
	g.buildAdvCivPlanetList()
	g.getWorthForAllPlanets()
}

// buildAdvCivPlanetList is Build_Adv_Civ_Planet_List_ (0x63035): free
// planets on the stars in reach, each listed once.
func (g *Game) buildAdvCivPlanetList() {
	s := &g.state
	a := g.adv
	for p := 0; p < int(s.NumPlayers); p++ {
		for k := 0; k < 0x48; k++ {
			if a.i16(p, acStarList+4*k+2) > 9 {
				continue
			}
			st := a.i16(p, acStarList+4*k)
			if st <= 0 {
				continue
			}
			a.setI16(p, acStars, a.i16(p, acStars)+1)
			for o := 0; o < 5; o++ {
				pi := s.Stars[st].Planets[o]
				n := a.i16(p, acListed)
				if pi <= -1 || s.Planets[pi].Type != 3 || a.player[p][acFlags+int(pi)] != 0 || s.Planets[pi].Colony != -1 {
					continue
				}
				e := entry(n)
				clear(a.player[p][e : e+acEntrySize])
				a.setI16(p, e+7, pi)
				a.setI16(p, acListed, n+1)
				a.player[p][acFlags+int(pi)] = 1
			}
		}
	}
}

// getWorthForAllPlanets is Get_Worth_For_All_Planets_ (0x63156): the
// listed planets' worth, then all 0x168 entries sorted by it.
func (g *Game) getWorthForAllPlanets() {
	s := &g.state
	a := g.adv
	type ent struct {
		b [acEntrySize]byte
	}
	for p := 0; p < int(s.NumPlayers); p++ {
		n := a.i16(p, acListed)
		for k := int16(0); k < n; k++ {
			e := entry(k)
			pi := a.i16(p, e+7)
			var worth int32
			var flag uint8
			var prox int16
			// pi -1 reads the header byte before the block, 0.
			if pi != -1 && a.selected[pi] == -1 {
				worth, flag, prox = g.advCivPlanetWorthiness(p, pi)
			}
			a.setI32(p, e+3, worth)
			a.player[p][e] = flag
			a.setI16(p, e+1, prox)
		}
		list := make([]ent, 0x168)
		for k := range list {
			copy(list[k].b[:], a.player[p][entry(int16(k)):])
		}
		clib.Qsort(list, acEntrySize, func(x, y *ent) int {
			return int(int32(binary.LittleEndian.Uint32(y.b[3:])) - int32(binary.LittleEndian.Uint32(x.b[3:])))
		})
		for k := range list {
			copy(a.player[p][entry(int16(k)):], list[k].b[:])
		}
	}
}

// advCivPlanetWorthiness is Get_Adv_Civ_Planet_Worthiness_ (0x63259):
// the worth, whether the star is the player's, the proximity bonus.
func (g *Game) advCivPlanetWorthiness(p int, pi int16) (int32, uint8, int16) {
	if pi <= -1 {
		return -1, 0, 0
	}
	s := &g.state
	star := s.Planets[pi].Star
	worth := int32(uint16(g.uncolonizedPlanetWorthToPlayer(int(pi), p)))
	prox := int16(int32(g.proximityBonus(p, pi)) * worth / 100)
	var flag uint8
	if int16(g.adv.starOwner[star]) == int16(p) {
		worth = worth * 0x43 / 100
		flag = 1
	}
	return worth + int32(prox), flag, prox
}

// proximityBonus is Proximity_Bonus_ (0x63312): 20 for each of the
// player's stars 1..2 parsecs away, 10 at 3..4, 5 at 5.
func (g *Game) proximityBonus(p int, pi int16) int16 {
	s := &g.state
	a := g.adv
	star := s.Planets[pi].Star
	v := int16(0)
	for k := 0; k < int(s.NumStars); k++ {
		st := a.i16(p, acStarList+4*k)
		if int16(a.starOwnerAt(st)) != int16(p) {
			continue
		}
		d := int16(a.parsecAt(st, int(star)))
		if d > 9 {
			continue
		}
		switch {
		case uint16(d) < 1:
		case uint16(d) < 3:
			v += 0x14
		case uint16(d) <= 4:
			v += 0xa
		case d == 5:
			v += d
		}
	}
	return v
}

// getNextAdvCivPlanet is Get_Next_Adv_Civ_Planet_ (0x6341c): the first
// (best) listed planet that is free, on no other player's star and in
// reach of the home star; -1 when none or the share is full.
func (g *Game) getNextAdvCivPlanet(p int) int16 {
	s := &g.state
	a := g.adv
	hs := s.Planets[s.Players[p].HomePlanet].Star
	if g.advCivLimit() <= a.i16(p, acChosen) {
		return -1
	}
	n := a.i16(p, acListed)
	maxd := int32(a.i16(p, acFar)) * 4 / 10
	if sz := int32(g.settings[0xd6]) + 9; sz >= maxd {
		maxd = sz
	}
	for k := int16(0); k < n; k++ {
		pi := a.i16(p, entry(k)+7)
		if pi <= -1 || a.selected[pi] != -1 {
			continue
		}
		st := s.Planets[pi].Star
		d := int16(a.parsec[hs][st])
		o := a.starOwner[st]
		if o > -1 && int16(o) != int16(p) {
			continue
		}
		if int32(d) <= maxd {
			return k
		}
	}
	return -1
}

// playersPlanetWorthinessAverage is Get_Players_Planet_Worthiness_Average_
// (0x63ca0): ten times the mean worth of the player's picked planets.
func (g *Game) playersPlanetWorthinessAverage(p int) int32 {
	s := &g.state
	sum := int32(0)
	for k := int16(0); k < s.PlanetCount; k++ {
		if int16(g.adv.selected[k]) == int16(p) {
			w, _, _ := g.advCivPlanetWorthiness(p, k)
			sum += w
		}
	}
	n := int32(g.adv.i16(p, acChosen))
	if n == 0 {
		g.Log("Get_Players_Planet_Worthiness_Average_: no planets (the original divides by zero)")
		return 0
	}
	return sum * 10 / n
}

// twiddleSelectedAdvCivPlanets is Twiddle_Selected_Adv_Civ_Planets_
// (0x638a9): the other players' picked planets get specials and better
// size, climate and minerals until their worth is 90% of the best's.
func (g *Game) twiddleSelectedAdvCivPlanets() {
	s := &g.state
	a := g.adv
	best, bestP := int32(0), int16(0)
	var cnt [8]int16
	for p := int16(0); p < s.NumPlayers; p++ {
		if v := g.playersPlanetWorthinessAverage(int(p)); v > int32(int16(best)) {
			best, bestP = v, p
		}
	}
	isGood := func(sp int8) bool { return sp == 5 || sp == 4 || sp == 0xa }
	for p := int16(0); p < s.NumPlayers; p++ {
		for k := int16(0); k < s.PlanetCount; k++ {
			if int16(a.selected[k]) == p && isGood(int8(s.Planets[k].Special)) {
				cnt[p]++
			}
		}
	}
	enough := func(p int16) bool {
		return int32(int16(g.playersPlanetWorthinessAverage(int(p)))) >= int32(int16(best))*0x5a/100
	}
	for p := int16(0); p < s.NumPlayers; p++ {
		if p == bestP {
			continue
		}
		done := false
		for k := int16(0); k < s.PlanetCount && !done; k++ {
			if int16(a.selected[k]) != p {
				continue
			}
			pl := &s.Planets[k]
			var w [3]uint8
			w[0] = 4 - pl.Size
			if d := 8 - int32(pl.Climate); d > 0 {
				w[1] = uint8(d)
			}
			w[2] = 4 - pl.Minerals
			if cnt[p] < cnt[bestP] && pl.Special == 0 && s.Stars[pl.Star].Special == 0 {
				var sp uint8
				if !(k == s.Players[p].HomePlanet && s.Players[p].Picks[16] != 0) {
					for tries := int16(0); tries < 0x64; tries++ {
						var worm int16
						g.getPlanetOrStarSpecial(int(pl.Star), &worm, &sp, false, 0, false)
						if isGood(int8(sp)) {
							break
						}
						sp = 0
					}
				}
				if int8(sp) > 0 {
					pl.Special = sp
					cnt[p]++
					s.Stars[pl.Star].Special = sp
				}
				if enough(p) {
					done = true
				}
			}
			for c := 0; c < 6 && !done; c++ {
				stop := g.twiddlePlanet(k, &w)
				g.setSecondaryPlanetData(int(pl.Star), int(k))
				if enough(p) {
					done = true
				}
				if stop {
					break
				}
			}
		}
	}
}

// twiddlePlanet is Twiddle_Planet_ (0x63b8f): one step up in size, climate
// or minerals, weighted by the room left; true when no room is left.
func (g *Game) twiddlePlanet(pi int16, w *[3]uint8) bool {
	p := &g.state.Planets[pi]
	c := getWeightedChoiceChar(&g.state.Random, w[:])
	if c == -1 {
		return true
	}
	switch c {
	case 0:
		p.Size = uint8(min(int32(p.Size)+1, 4))
	case 1:
		p.Climate = uint8(min(int32(p.Climate)+1, 9))
	case 2:
		p.Minerals = uint8(min(int32(p.Minerals)+1, 4))
	default:
		return false
	}
	w[c]--
	return false
}

// assignAdvancedCivilizationStartingShips is
// Assign_Advanced_Civilization_Starting_Ships_ (0x63848): it calls
// Init_Advanced_Civilization_Fleet_, an empty routine, per player.
func (g *Game) assignAdvancedCivilizationStartingShips() {}

// allocateAdvCivGameOfficers is Allocate_Adv_Civ_Game_Officers_ (0x98489):
// repulsive races get one of the first ten leaders (shuffled), the others
// two of the first 25.
func (g *Game) allocateAdvCivGameOfficers() {
	s := &g.state
	r := &s.Random
	var used [0x43]bool
	var list [0x43]int16
	shuffle := func(n uint32) {
		for i := range list {
			list[i] = int16(i)
		}
		for c := uint32(0); c < n; c++ {
			a := int16(r.Random(n)) - 1
			b := int16(r.Random(n)) - 1
			if a != b {
				list[a], list[b] = list[b], list[a]
			}
		}
	}
	give := func(o int16, p int) {
		if o <= -1 {
			return
		}
		s.Leaders[o].Status = 4
		s.Leaders[o].Owner = int8(p)
		used[o] = true
	}
	shuffle(10)
	si := 0
	for p := 0; p < int(s.NumPlayers); p++ {
		if s.Players[p].Picks[save.PickRepulsive] != 0 {
			give(list[si], p)
			si++
		}
	}
	shuffle(0x19)
	for p := 0; p < int(s.NumPlayers); p++ {
		if s.Players[p].Picks[save.PickRepulsive] != 0 {
			continue
		}
		for n := 0; n < 2; n++ {
			b := 0
			for used[list[b]] {
				b++
			}
			give(list[b], p)
		}
	}
}
