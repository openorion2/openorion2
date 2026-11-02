package game

import "github.com/openorion2/openorion2/internal/save"

// Automatic colony building choice (AIBUILD.C) and the parts of the AI data
// it reads; Compute_AI_Data_ (step 3, turn_aidata.go) fills that data.

// colonyIsConquered is Colony_Is_Conquered_ (0xcff02): a pop of a living
// race that is at war with the owner.
func (g *Game) colonyIsConquered(c *save.Colony) bool {
	s := &g.state
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		r := int(c.Colonists[i] & 0xf)
		if r >= 8 || r == int(c.Owner) || s.Players[r].Eliminated != 0 {
			continue
		}
		if t := s.Players[r].TreatyStatus[c.Owner]; t >= 4 && t <= 6 {
			return true
		}
	}
	return false
}

// playerColonyAutobuild is Player_Colony_Autobuild_ (0xd2783).
func (g *Game) playerColonyAutobuild(ci int) {
	g.colonyCalculation(ci)
	g.assignColonyNewBuilding(ci)
}

// assignColonyNewBuilding is Assign_Colony_New_Building_ (0xd0b08): every
// building the colony can use is scored, the weak ones dropped by
// difficulty, then one picked by weight with housing and trade goods.
func (g *Game) assignColonyNewBuilding(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	var w [51]uint16
	for b := int16(1); b < 0x31; b++ {
		if g.colonyCanBuildProduct(ci, b) == 1 {
			w[b] = 1
		}
	}
	best := int16(0)
	for b := 48; b >= 0; b-- {
		if int16(w[b]) <= 0 {
			continue
		}
		v := int16(g.colonyBuildingScore(ci, b))
		w[b] = uint16(v)
		if best < v {
			best = v
		}
	}
	w[50] = 1
	best = max(best, 1)
	for b := 50; b >= 0; b-- {
		diff := int32(g.settings[0xd4])
		if s.Players[c.Owner].Objective == 100 {
			diff = 4
		}
		if (6-diff)*int32(int16(w[b])) < int32(best) {
			w[b] = 0
		}
	}
	v, pop := int32(g.ai.colonyInfo(ci)[1]), int32(c.PopCount)
	if 3*pop <= v {
		w[49] = uint16(isqrt(uint32(3 * (v - pop))))
	}
	switch k := getWeightedChoiceShort(&s.Random, w[:]); {
	case k < 0x31:
		g.deleteProdFromColonyQueue(ci, k)
		c.Queue[0] = k
	case k == 0x31:
		c.Queue[0] = -3
	case k == 0x32:
		c.Queue[0] = -2
	}
}

// colonyBuildingScore is Colony_Building_Score_ (0xd0036): the AI's value
// of building b at the colony, 0..1000, from its threats (AI data), the
// empire's objective, wealth and the colony itself.
func (g *Game) colonyBuildingScore(ci, b int) int32 {
	s := &g.state
	c := &s.Colonies[ci]
	p := int(c.Owner)
	pl := &s.Players[p]
	planet := &s.Planets[c.Planet]
	b2i := func(v bool) int32 {
		if v {
			return 1
		}
		return 0
	}
	f5, f4, f1, f3 := b2i(pl.Objective == 5), b2i(pl.Objective == 4), b2i(pl.Objective == 1), b2i(pl.Objective == 3)
	var rich int32
	if pl.BC >= 1500 {
		rich = int32(isqrt(uint32(int32(pl.NetIncome) / 64)))
	}
	star := int(planet.Star)
	si := g.ai.starInfo(star)[1+p*6:]
	a0, a1, a2, a3 := int32(si[0]), int32(si[1]), int32(si[2]), int32(si[3])
	info := g.ai.colonyInfo(ci)
	t := int32(info[3] + info[4])
	ci5 := int32(info[5])
	flagA := c.Buildings[7] == 0 && pl.AppStatus[0x16] == techKnown && planet.Minerals <= 2
	rich = min(rich, 10)
	gov := func() int32 { return int32(pl.Picks[save.PickGovernment]) / 2 }
	flagB := false
	if uint32(gov()) <= 1 {
		flagB = (c.Buildings[0x16] == 0 && pl.AppStatus[0x67] == techKnown) ||
			(c.Buildings[2] == 0 && pl.AppStatus[0xe] == techKnown)
	}
	busy := flagA || flagB
	pop := int32(c.PopCount)
	var v int32
	// withF1 adds the objective bonus to a nonzero threat score, then the
	// wealth bonus (the tail at 0xd0540).
	withF1 := func(x int32) int32 {
		if x != 0 {
			x += f1
		}
		return x + rich
	}
	command := func() int32 {
		if pl.CommandTotal <= pl.CommandUsed {
			return int32(pl.CommandUsed) + 1 - int32(pl.CommandTotal)
		}
		return 0
	}
	idle := busy && ci5 == 0
	foodBase := func(vals [4]int32) int32 {
		switch c.FoodPerFarmer {
		case 0, 1, 2:
			return vals[c.FoodPerFarmer]
		}
		return vals[3]
	}
	switch b {
	case 40:
		if idle {
			break
		}
		v = withF1(ci5*10 + a0*4 + a1*8 + a2*16 + a3*3 + command())
	case 8, 41:
		if idle {
			break
		}
		// Both jump into case 40's tail past its shift of a2.
		v = withF1(ci5*10 + a0*3 + a1*6 + a2*12 + a3*3 + command())
	case 2:
		if pop >= 3 || rich != 0 {
			if v = 2*ci5 + a0 + a1 + 3*a2 + a3; v != 0 {
				v += f1
			}
		}
		if c.Buildings[0x16] == 0 && uint32(gov()) <= 1 {
			v += 6
		}
		if g.colonyIsConquered(c) {
			v++
		}
		v += rich / 2
	case 22:
		if pop >= 2 || rich != 0 {
			if v = 5*ci5 + a0 + 3*a1 + 6*a2 + 2*a3; v != 0 {
				v += f1
			}
		}
		if c.Buildings[2] == 0 && uint32(gov()) <= 1 {
			v += 12
		}
		if g.colonyIsConquered(c) {
			v += 3
		}
		v += rich
	case 28, 24, 23:
		if idle {
			break
		}
		if b == 28 {
			v = 10*ci5 + 4*a0 + 8*a1 + 12*a2 + 4*a3
		} else {
			v = 10*ci5 + a0 + 4*a1 + 12*a2 + 2*a3
		}
		if v != 0 {
			v += f1
		}
		if planet.Climate == 1 {
			v += 2 * f5
		}
		v += rich
	case 47, 26, 27, 42:
		if idle {
			break
		}
		v = withF1(10*ci5 + 4*a0 + 8*a1 + 16*a2 + 4*a3)
	case 3, 45:
		if idle {
			break
		}
		if !maskBit(s.Stars[star].WarpInterdictor, p) {
			if b == 3 {
				v = 10*ci5 + a0 + a1 + 3*a2 + a3
			} else {
				v = 5*ci5 + 2*a0 + 3*a1 + 4*a2 + a3
			}
			if v != 0 {
				v += f1
			}
		}
		v += rich / 2
	case 20, 31:
		if gov() == 3 || (c.PopCount < 3 && (rich == 0 || c.PopCount < 2)) {
			break
		}
		v = 10
		if b == 31 {
			v = 16
		}
	case 4:
		v = 5
	case 6, 19:
		if busy || pl.Unknown59D != 0 {
			break
		}
		v = 11
		if b == 6 {
			v = f3*4 + 11
		}
	case 7:
		v = f4*2 + pop + 13
	case 9:
		if gov() != 3 && pl.HomePlanet == c.Planet {
			v = 100
		}
	case 10:
		if busy {
			break
		}
		if pl.Picks[save.PickPopulation] < 0 {
			v = f5
		}
		v += rich / 2
	case 12:
		v = f4*4 + pop + 12
	case 13:
		// Core Waste Dump: the busy test, then the pollution case below.
		if busy {
			break
		}
		fallthrough
	case 5, 32:
		if t == 0 || c.Pollution <= 5 {
			break
		}
		v = f5
		if c.Pollution > 10 {
			v += int32(isqrt(uint32(int32(c.Pollution))))
		}
	case 15:
		if !busy {
			v = f5 + 18
		}
	case 16:
		if info[2] != 0 {
			v = 4 + f5
			if pl.FoodSurplus < 0 {
				v = 8 + f5
			}
		}
	case 17:
		v = f5 + rich
	case 21, 43:
		if info[2] == 0 {
			break
		}
		if b == 21 {
			v = foodBase([4]int32{12, 11, 10, 6})
		} else {
			if busy {
				break
			}
			v = foodBase([4]int32{13, 12, 10, 7})
		}
		if pl.FoodSurplus < 0 {
			v -= int32(pl.FoodSurplus)
		}
		if b == 21 {
			v += f5 * 4
		} else {
			v += f5 * 3
		}
	case 25:
		grav := planet.Gravity
		switch {
		case pl.Picks[save.PickHighG] != 0:
			if grav == 0 {
				v = f5 + 3
			}
		case pl.Picks[save.PickLowG] != 0:
			switch grav {
			case 1:
				v = f5 + 3
			case 2:
				v = f5 + 6
			}
		default:
			switch grav {
			case 0:
				v = f5 + 3
			case 2:
				v = f5 + 6
			}
		}
	case 29, 39:
		lim := int32(5)
		if b == 39 {
			lim = 3
		}
		if !busy && pop >= lim {
			v = (int32(info[1]) + pop + f4) / 3
		}
	case 30, 35:
		if busy || pl.Unknown59D != 0 {
			break
		}
		v = f3*3 + 8
		if b == 35 {
			v = f3*2 + 5
		}
	case 33:
		v = (int32(info[1])+2*pop)/3 + 2*(t+f5+f4)
	case 34:
		v = f4*2 + 12
	case 36:
		v = 2*f4 + pop + 5
	case 37, 46:
		if busy || info[2] == 0 || c.FoodPerFarmer == 0 {
			break
		}
		if b == 37 {
			v = 3
			if pl.FoodSurplus < 0 {
				v = 5
			}
		} else {
			v = 5
			if pl.FoodSurplus < 0 {
				v = 10
			}
		}
		v += 2 * f5
	case 38:
		if busy || (c.Output[save.Industry] < 17 && c.PopCount < 5 && rich == 0) {
			break
		}
		v = int32(isqrt(uint32(int32(c.Output[save.Industry]) - 15)))
	case 44:
		if busy {
			break
		}
		aquatic := pl.Picks[save.PickAquatic] != 0
		inRange := true
		switch planet.Climate {
		case 2:
			v = 2
		case 3, 7:
			v = 1
		case 4:
			if aquatic {
				v = 1
			}
		case 5:
			if !aquatic {
				v = 4
			}
		case 6:
			if !aquatic {
				v = 6
			}
		default:
			inRange = false
		}
		if inRange {
			v += f5 * 3
		}
		v += rich
	}
	if v <= 0 {
		return 0
	}
	return min(v, 1000)
}
