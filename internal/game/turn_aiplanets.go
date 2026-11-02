package game

import "github.com/openorion2/openorion2/internal/save"

// How the AI values stars, planets and colonies (AIDATA.C). Spec:
// docs/turn/ai-data.md.

const (
	aiMineralWeights = 0xdd4b5 // u8 by planet minerals
	aiClimatePenalty = 0xdd4ba // u8 by planet climate
)

// colonyDominantPlayer is Colony_Dominant_Player_ (0xd2a08): the player with
// most colonists if they outnumber the rest, else usually the owner.
func (s *State) colonyDominantPlayer(c *save.Colony) int16 {
	var count [8]uint8
	n := int32(0)
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		if p := c.Colonists[i] & 0xf; p < 8 {
			n++
			count[p]++
		}
	}
	best := int(s.NumPlayers) - 1
	for j := best - 1; j >= 0; j-- {
		if count[best] < count[j] {
			best = j
		}
	}
	if n < 2*int32(count[best]) {
		return int16(best)
	}
	o := c.Owner
	if n >= 3*int32(count[o]) && count[best] > 0 && count[best] != count[o] {
		return int16(best)
	}
	return int16(o)
}

// computeStarPlayerRangeInfo is Compute_Star_Player_Range_Info_ (0xd3ba0):
// per star and colonist, colonies in range and the players around it.
func (g *Game) computeStarPlayerRangeInfo() {
	s := &g.state
	a := g.ai
	np := int(s.NumPlayers)
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		x := a.at(aiStarInfo) + st*0x31 + 1
		for c := int(s.NumColonies) - 1; c >= 0; c-- {
			col := &s.Colonies[c]
			o := int(col.Owner)
			if o == -1 {
				continue
			}
			if g.starInRangeOfGivenStar(st, int(s.Planets[col.Planet].Star), int32(s.Players[o].FuelRange)) {
				a.setU8(x+o*6+4, a.u8(x+o*6+4)+1)
			}
		}
		normal, ext := g.findPlayersInRange(st)
		for p := np - 1; p >= 0; p-- {
			if !maskBit(s.Stars[st].ColonyOwners, p) {
				continue
			}
			y := x + p*6
			for q := np - 1; q >= 0; q-- {
				if q == p || s.Players[q].Eliminated != 0 {
					continue
				}
				switch {
				case normal[q] != 0:
					switch t := s.Players[p].TreatyStatus[q]; {
					case t == 0:
						a.setU8(y+1, a.u8(y+1)+1)
					case t <= 3:
						a.setU8(y, a.u8(y)+1)
					case t <= 6:
						a.setU8(y+2, a.u8(y+2)+1)
					}
				case ext[q] != 0:
					a.setU8(y+3, a.u8(y+3)+1)
				}
			}
		}
	}
}

// starInRangeOfGivenStar is Star_In_Range_Of_Given_Star_ (0xd2aa9): within
// rng parsecs of star from, or of the star its wormhole leads to.
func (g *Game) starInRangeOfGivenStar(st, from int, rng int32) bool {
	if g.parsecsBetweenStars(from, st) <= rng {
		return true
	}
	w := int(g.state.starRecord(from).WormholeTo)
	return w != -1 && g.parsecsBetweenStars(w, st) <= rng
}

// findPlayersInRange is Find_Players_In_Range_ (0xd3a68): which players
// have a colony within normal or extended fuel range of star st.
func (g *Game) findPlayersInRange(st int) (normal, ext [8]uint8) {
	s := &g.state
	S := s.starRecord(st)
	var nr, er [8]int32
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		r := int32(s.Players[p].FuelRange)
		nr[p] = r * 900 * r
		er[p] = nr[p] * 9 / 4
	}
	for c := int(s.NumColonies) - 1; c >= 0; c-- {
		col := &s.Colonies[c]
		o := int(col.Owner)
		if o == -1 || normal[o] != 0 {
			continue
		}
		C := &s.Stars[s.Planets[col.Planet].Star]
		dx, dy := int32(C.X)-int32(S.X), int32(C.Y)-int32(S.Y)
		d := dx*dx + dy*dy
		if d > er[o] {
			continue
		}
		ext[o] = 1
		if d <= nr[o] {
			normal[o] = 1
		}
	}
	return
}

// computeBasePlanetValues is Compute_Base_Planet_Values_ (0xd302e): what
// each planet in a player's extended range is worth to it.
func (g *Game) computeBasePlanetValues() {
	s := &g.state
	a := g.ai
	a.basePlanetValues = [8][360]uint16{}
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		for pl := int(s.PlanetCount) - 1; pl >= 0; pl-- {
			P := &s.Planets[pl]
			if !a.extRange(int(P.Star), p) {
				continue
			}
			if P.Colony == -1 {
				a.basePlanetValues[p][pl] = uint16(g.uncolonizedPlanetWorthToPlayer(pl, p))
			} else {
				a.basePlanetValues[p][pl] = uint16(g.colonyWorthToPlayer(int(P.Colony), p))
			}
		}
	}
}

// gravityValue applies the gravity terms the two worth routines share.
func gravityApplies(gravity uint8, D *save.Player) (low, heavy bool) {
	switch gravity {
	case 0:
		low = D.Picks[save.PickLowG] == 0
	case 1:
		low = D.Picks[save.PickLowG] != 0
	case 2:
		heavy = D.Picks[save.PickHighG] == 0
	}
	return
}

// growthWeights are the minerals and population factors by the population
// pick (-50, 0, +50, +100); ok is false for any other value.
func growthWeights(pick uint8, uncolonized bool) (m, pop int32, ok bool) {
	type w struct{ m, pop int32 }
	var t map[uint8]w
	if uncolonized {
		t = map[uint8]w{0: {0x4b, 2}, 0x32: {0x46, 3}, 0x64: {0x41, 4}, 0xce: {0x50, 1}}
	} else {
		t = map[uint8]w{0: {0x5a, 10}, 0x32: {0x55, 0xf}, 0x64: {0x50, 0x14}, 0xce: {0x5f, 5}}
	}
	v, ok := t[pick]
	return v.m, v.pop, ok
}

// climateTerm is the climate penalty both worth routines apply, skipped
// for aquatic races on ocean and swamp worlds.
func (g *Game) climateTerm(v int32, climate uint8, special bool, D *save.Player) int32 {
	if special && D.Picks[save.PickAquatic] != 0 {
		return v
	}
	t := int32(g.exeU8(uint32(aiClimatePenalty+int(climate)))) / 4
	v = (100 - t) * v / 100
	if special {
		v = v * 3 / 4
	}
	return v
}

// uncolonizedPlanetWorthToPlayer is Uncolonized_Planet_Worth_To_Player_
// (0xd27a7).
func (g *Game) uncolonizedPlanetWorthToPlayer(pl, p int) int32 {
	s := &g.state
	P := &s.Players[p]
	pt := &s.Planets[pl]
	v := int32(0)
	if pt.Type == 3 {
		base := int32(0)
		if P.AppStatus[3] == 3 {
			base = 5
		}
		base += int32(g.planetMaxPopulationForPlayer(pl, p))
		if P.Picks[save.PickSubterranean] != 0 {
			base += int32(int16((int32(pt.Size) + 1) * 2))
		}
		pop := int16(base)
		mw := int32(g.exeU8(uint32(aiMineralWeights + int(pt.Minerals))))
		prod := int16((mw + 1) * int32(pop) / 3)
		special := pt.Climate == 5 || pt.Climate == 6
		food := int16(pt.FoodPerFarmer)
		soil := int32(0)
		if P.AppStatus[0x19a-0x117] == 3 {
			soil = 1
		}
		if food == 0 && P.AppStatus[0x134-0x117] == 3 {
			food = 1
		}
		if m, f, ok := growthWeights(uint8(P.Picks[save.PickPopulation]), true); ok {
			v = mw*m + int32(prod)*f
		}
		if P.Picks[save.PickLithovore] == 0 {
			if P.Picks[save.PickCybernetic] != 0 {
				v += int32(food) * 0x4b
			} else {
				v += int32(food) * 0x96
			}
		}
		switch pt.Special {
		case 4:
			v += 0x500
		case 5:
			v += 0xa00
		case 0xa:
			v += 0x280
		case 0xb:
			v += 0x640
		}
		v *= int32(pop)
		v = (mw + 7) * v / 10
		v = g.climateTerm(v, pt.Climate, special, P)
		switch low, heavy := gravityApplies(pt.Gravity, P); {
		case low:
			v = (soil + 0x12) * v / 0x14
		case heavy:
			v = (soil + 4) * v / 6
		}
	}
	v /= 64
	return min(v, 0xffff)
}

// colonyWorthToPlayer is Colony_Worth_To_Player_ (0xd2cae).
func (g *Game) colonyWorthToPlayer(c, p int) int32 {
	s := &g.state
	a := g.ai
	col := &s.Colonies[c]
	P := &s.Players[p]
	pt := &s.Planets[col.Planet]
	dom := int(a.u8(a.at(aiColonyInfo) + c*7))
	D := &s.Players[dom]
	pop := (int32(g.planetMaxPopulationForPlayer(int(col.Planet), dom)) + int32(g.planetMaxPopulationForPlayer(int(col.Planet), p)) + 1) / 2
	special := pt.Climate == 5 || pt.Climate == 6
	if col.IsOutpost != 0 {
		return int32(g.proximityWorthToPlayer(c, p))
	}
	food := int32(g.colonyEmpireBaseFood2(c, p, dom)) / 2
	ind := int32(g.colonyEmpireBaseIndustry(c, p, dom))
	res := int32(g.colonyEmpireBaseResearch(c, p, dom))
	v := ind + res
	switch {
	case D.Picks[save.PickLithovore] != 0:
		v *= 6
	case D.Picks[save.PickCybernetic] != 0:
		v = (res + food + ind) * 4
	default:
		v = v*3 + food*6
	}
	// Any other population pick multiplies by a stale stack word; the
	// four the game offers all have a factor.
	if m, f, ok := growthWeights(uint8(P.Picks[save.PickPopulation]), false); ok {
		v *= int32(col.PopCount)*m + pop*f
	}
	v = (pop + 100 - int32(col.PopCount)) * v / 100
	v = g.climateTerm(v, pt.Climate, special, D)
	if int16(col.Owner) != int16(p) || col.Buildings[0x14f-0x136] == 0 {
		soil := int32(0)
		if P.AppStatus[0x19a-0x117] == 3 {
			soil = 1
		}
		switch low, heavy := gravityApplies(pt.Gravity, D); {
		case low:
			v = (soil + 0xc) * v / 16
		case heavy:
			v = (soil + 6) * v / 12
		}
	}
	if col.Buildings[0x13f-0x136] != 0 {
		switch int32(s.Players[col.Owner].Picks[save.PickGovernment]) / 2 {
		case 0:
			v = v * 3 / 2
		case 1:
			v = v * 4 / 3
		case 2:
			v = v * 6 / 5
		}
	}
	switch pt.Special {
	case 4:
		v += 0x3e8
	case 5:
		v += 0x7d0
	}
	v /= 64
	if g.settings[0xd4] >= 2 {
		px := int32(g.proximityWorthToPlayer(c, p))
		v += px / 8
		if v < px {
			v = px
		}
	}
	return min(v, 0xffff)
}

// proximityWorthToPlayer is Proximity_Worth_To_Player_ (0xd2aea): from
// difficulty 1 on, how much a colony helps its owner reach other stars.
func (g *Game) proximityWorthToPlayer(c, p int) uint16 {
	s := &g.state
	a := g.ai
	col := &s.Colonies[c]
	if g.settings[0xd4] == 0 {
		return 0
	}
	o := int(col.Owner)
	O := &s.Players[o]
	if O.AppStatus[5] == 3 {
		return 0
	}
	rng := int32(O.FuelRange)
	cs := int(s.Planets[col.Planet].Star)
	cnt := func(st int) int32 { return int32(a.u8(a.at(aiStarInfo) + st*0x31 + o*6 + 5)) }
	v := uint16(0)
	if o == p {
		for st := int(s.NumStars) - 1; st >= 0; st-- {
			S := &s.Stars[st]
			if S.SpectralClass == save.SpectralBlackHole || !g.starInRangeOfGivenStar(st, cs, rng) {
				continue
			}
			w := int32(0x50)
			if uint32(S.ColonyOwners)&^(uint32(1)<<(uint(o)&31)) != 0 {
				w = 0x78
			}
			v += uint16(w / cnt(st))
		}
		return v
	}
	least := int32(10000)
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		if !g.starInRangeOfGivenStar(st, cs, rng) || !maskBit(s.Stars[st].ColonyOwners, p) {
			continue
		}
		n := cnt(st)
		v += uint16(100 / n)
		if n < least {
			least = n
		}
	}
	if v != 0 {
		v += uint16(400 / least)
	}
	return v
}

// computeContextualPlanetValues is Compute_Contextual_Planet_Values_
// (0xd3125): base values adjusted by the other planets of the same star.
func (g *Game) computeContextualPlanetValues() {
	s := &g.state
	a := g.ai
	a.contextualPlanetValues = [8][360]uint16{}
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		bv, cv := &a.basePlanetValues[p], &a.contextualPlanetValues[p]
		for pl := int(s.PlanetCount) - 1; pl >= 0; pl-- {
			if bv[pl] == 0 {
				continue
			}
			pt := &s.Planets[pl]
			S := &s.Stars[pt.Star]
			var free, ownSum, own, enemy int32
			cv[pl] = bv[pl]
			for orbit := 4; orbit >= 0; orbit-- {
				q := int(S.Planets[orbit])
				if q == -1 || q == pl {
					continue
				}
				switch c := s.Planets[q].Colony; {
				case c == -1:
					free += int32(bv[q])
				case int(s.Colonies[c].Owner) == p:
					own++
					ownSum += int32(bv[q])
				default:
					enemy++
				}
			}
			f8 := free / 8
			if pt.Colony != -1 {
				cv[pl] += uint16(f8)
				cv[pl] += uint16(ownSum / 4)
				continue
			}
			if own > 0 {
				cv[pl] = uint16((int32(pt.Size) + 1) * int32(cv[pl]) / 10)
			} else {
				cv[pl] += uint16(f8)
			}
			if enemy > 0 {
				cv[pl] /= uint16(enemy + 2)
			}
		}
	}
}
