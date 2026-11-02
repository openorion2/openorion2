package game

import "github.com/openorion2/openorion2/internal/save"

// What each empire needs built (AIBUILD.C): Compute_Empire_Building_Needs_
// and its statics, read later by Assign_Empire_Building_.

const (
	tblNeedsDifficultyWeight = 0xcf38a // u8 by difficulty, Mean_Ship_Strength_
	tblNeedsHullWeight       = 0xcf38f // u8 by hull size
)

// aiBuildNeeds are the AIBUILD.C statics 0x1a7234..0x1a74b4. They outlive
// the AI arena; Compute_Empire_Building_Needs_ clears all but the freighters.
type aiBuildNeeds struct {
	colonyShips      [8][3]uint8 // _player_n_colony_ships: owned, wanted, moving
	freightersWanted [8]uint8    // _player_n_freighter_groups_wanted
	strengthVsColony [8]int32    // _player_total_strength_vs_colonies
	transports       [8][2]uint8 // _player_n_transports: owned, wanted
	spies            [8][2]uint8 // _player_n_spies: theirs on others, wanted
	// _player_ship_sizes: [0..5] hulls by size, [9] warship size wanted,
	// [10] ships, [11] Σ size+1, [12] Σ size+1 built, [13] Σ hull weight.
	shipSizes [8][14]int32
	// _player_total_strength_vs_ships: all, built, warship strength wanted.
	strengthVsShips [8][3]int32
}

// computeEmpireBuildingNeeds is Compute_Empire_Building_Needs_ (0xcfcb6),
// for every player, humans included (their draws count).
func (g *Game) computeEmpireBuildingNeeds() {
	s := &g.state
	n := &g.needs
	n.strengthVsShips = [8][3]int32{}
	n.shipSizes = [8][14]int32{}
	n.strengthVsColony = [8]int32{}
	n.colonyShips = [8][3]uint8{}
	n.transports = [8][2]uint8{}
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		st := int8(sh.Status)
		if !(st < 5 || st == 6) || sh.Owner >= 8 {
			continue
		}
		o := int(sh.Owner)
		switch sh.Design.Type {
		case 0:
			n.strengthVsShips[o][0] += int32(g.meanShipStrength(i))
			if st != 6 {
				n.strengthVsShips[o][1] += int32(g.meanShipStrength(i))
			}
			size := int(sh.Design.Size)
			n.shipSizes[o][size]++
			n.shipSizes[o][13] += int32(g.exeU8(tblNeedsHullWeight + uint32(size)))
			n.shipSizes[o][11] += int32(size) + 1
			n.shipSizes[o][10]++
			if st != 6 {
				n.shipSizes[o][12] += int32(size) + 1
			}
		case 1:
			n.colonyShips[o][0]++
			if st == 2 || st == 1 {
				n.colonyShips[o][2]++
			}
		case 2:
			n.transports[o][0]++
		}
	}
	n.spies = [8][2]uint8{}
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		for q := int(s.NumPlayers) - 1; q >= 0; q-- {
			n.spies[p][0] += uint8(s.theirSpyNumber(p, q))
		}
	}
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		g.computeNeeds(p)
	}
}

// meanShipStrength is Mean_Ship_Strength_ (0xcfae5): the ship's mean
// strength against the players its owner has met (humans if none). Against
// both kinds it is the weakest AI value, or a difficulty-weighted blend.
func (g *Game) meanShipStrength(i int) int16 {
	s := &g.state
	a := g.ai
	o := int(s.Ships[i].Owner)
	info := a.at(aiPlayerInfo) + o*0x3a
	str := func(q int) uint32 { return uint32(a.u16(a.at(aiShipStrengths) + i*16 + q*2)) }
	var r uint32
	if a.u16(info+0xc) == 0 {
		var n, sum uint32
		for q := int(s.NumPlayers) - 1; q >= 0; q-- {
			if q == o || s.Players[q].Eliminated != 0 || s.Players[q].Objective != objectiveHuman {
				continue
			}
			n++
			sum += str(q)
		}
		if n > 0 {
			r = sum / n
		}
		return int16(r)
	}
	var hSum, hN, aSum, aN uint32
	hMin, aMin := ^uint32(0), ^uint32(0)
	for q := int(s.NumPlayers) - 1; q >= 0; q-- {
		if q == o || s.Players[q].Eliminated != 0 || a.u8(info+0xe+q) == 0 {
			continue
		}
		v := str(q)
		if s.Players[q].Objective == objectiveHuman {
			hSum += v
			hMin = min(hMin, v)
			hN++
		} else {
			aSum += v
			aMin = min(aMin, v)
			aN++
		}
	}
	switch {
	case hN == 0:
		if aN > 0 {
			r = aSum / aN
		}
	case aN == 0:
		r = hSum / hN
	case aMin < hMin:
		r = aMin
	default:
		w := uint32(g.exeU8(tblNeedsDifficultyWeight + uint32(g.settings.Difficulty())))
		r = (hMin*w + aMin) / (w + 1)
	}
	return int16(r)
}

// computeNeeds is Compute_Needs_ (0xcf40d): warship size and strength,
// transports, spies, colony ships and freighters p wants, measured against
// the players p has met (or the humans).
func (g *Game) computeNeeds(p int) {
	s := &g.state
	a := g.ai
	n := &g.needs
	pl := &s.Players[p]
	info := a.at(aiPlayerInfo) + p*0x3a
	met := int16(a.u16(info + 0xc))
	obj1 := int32(0)
	if pl.Objective == 1 {
		obj1 = 1
	}
	var cnt, tMax, tSum, sMax, sSum, spyMax, wMax, hull int32
	for q := int(s.NumPlayers) - 1; q >= 0; q-- {
		if q == p {
			continue
		}
		if !(met == 0 && s.Players[q].Objective == objectiveHuman) && a.u8(info+0xe+q) == 0 {
			continue
		}
		t := int32(n.transports[q][0])
		tSum += t
		cnt++
		tMax = max(tMax, t)
		sSum += n.strengthVsShips[q][0]
		sMax = max(sMax, n.strengthVsShips[q][0])
		spyMax = max(spyMax, int32(n.spies[q][0]))
		wMax = max(wMax, n.shipSizes[q][13])
		for k := int32(8); k >= 0; k-- {
			if n.shipSizes[q][k] != 0 && hull < k {
				hull = k
			}
		}
	}
	n.shipSizes[p][9] = hull + 1
	if sMax < n.strengthVsShips[p][0] || met == 0 {
		n.shipSizes[p][9] += obj1 + int32(s.Random.Random(2)) - 1
	}
	if cnt == 0 {
		cnt = 1
	}

	tw := &n.transports[p][1]
	*tw = 0
	if pl.AppStatus[0xbd] == techKnown && n.shipSizes[p][13] > 1 {
		cap := (int32(int16(a.u16(info))) + 2) / 3
		if want := (tSum/cnt + tMax + 1) / 2; int32(n.transports[p][0]) < want {
			*tw = uint8(want) - n.transports[p][0]
		}
		if met > 0 {
			c := int32(met) + obj1 + int32(isqrt(uint32(int32(int16(a.u16(info))))))
			if own := int32(n.transports[p][0]); own < c {
				if d := c - own; int32(*tw) < d {
					*tw = uint8(d)
				}
			}
		}
		if n.transports[p][0] >= 20 {
			*tw = 0
		} else if int32(*tw) > cap {
			*tw = uint8(cap)
			if met == 0 {
				*tw /= 4
			}
		}
		if *tw > 1 && pl.CommandTotal < pl.CommandUsed {
			*tw = 1
		}
	}

	target := sMax
	if obj1 == 0 {
		target = (sSum/cnt + sMax + 1) / 2
	}
	need := &n.strengthVsShips[p][2]
	*need = 0
	if target > n.strengthVsShips[p][0] {
		*need = target - n.strengthVsShips[p][0]
		if met == 0 {
			*need /= 2
		}
	}
	if *need != 0 && pl.CommandTotal <= pl.CommandUsed {
		switch {
		case pl.BC < 50 || pl.NetIncome < 50:
			*need = 0
		case pl.NetIncome < 0x400:
			*need = int32(isqrt(uint32(int32(pl.NetIncome)))) * *need / 32
			if g.playerWarLevel(p) == 0 {
				*need = int32(isqrt(uint32(*need))) / 16
			}
		}
	} else if met > 0 && s.Random.Random(uint32(10-obj1)) == 1 {
		*need += obj1*4 + 1
	}
	if pl.NetIncome >= 0x400 {
		if *need < 10 && n.shipSizes[p][12] != 0 {
			v := n.strengthVsShips[p][1] / n.shipSizes[p][12]
			if cmd := int32(pl.CommandTotal); n.shipSizes[p][11] < cmd {
				v *= cmd - n.shipSizes[p][11]
			} else {
				v = v * int32(pl.NetIncome) / 15
			}
			*need = v
		}
	} else if wMax < n.shipSizes[p][13] {
		*need = *need * wMax / n.shipSizes[p][13]
	}

	g.computeSpyNeed(p, spyMax)
	if n.colonyShips[p][0] == 0 {
		n.colonyShips[p][1] = 1
	}
	g.computeFreighterNeed(p)
}

// computeSpyNeed is the spy part of Compute_Needs_: catch up with the
// busiest spy master met, sometimes one more, capped by the 63 spy slots.
func (g *Game) computeSpyNeed(p int, spyMax int32) {
	s := &g.state
	a := g.ai
	n := &g.needs
	pl := &s.Players[p]
	w := &n.spies[p][1]
	*w = 0
	if n.strengthVsShips[p][0] <= 0 {
		return
	}
	room := 0x3f - int32(s.theirSpyNumber(p, p))
	contacts := int32(int16(a.u16(a.at(aiPlayerInfo) + p*0x3a + 0xa)))
	if contacts <= 0 || room <= 0 {
		return
	}
	own := int32(n.spies[p][0])
	if own < spyMax {
		*w = uint8(spyMax) - n.spies[p][0]
	}
	if own < spyMax+2 && int32(s.Random.Random(100)) <= int32(pl.Picks[save.PickSpying]) {
		*w++
	}
	if pl.Objective == 0 && s.Random.Random(2) == 1 {
		*w++
	}
	if *w != 0 {
		good := int32(0)
		if pl.Picks[save.PickSpying] > 0 {
			good = 1
		}
		if (good*2+2)*contacts < own {
			if *w > 1 {
				*w = uint8(isqrt(uint32(*w)))
			} else {
				*w = 0
				if pl.Picks[save.PickSpying] > 0 || s.Random.Random(4) == 1 {
					*w = 1
				}
			}
		}
	}
	if room < int32(*w) {
		*w = uint8(room)
	}
}

// computeFreighterNeed is Compute_Freighter_Need_ (0xcf3bd): a group of
// five freighters per five missing, colony ships in flight counted as used.
func (g *Game) computeFreighterNeed(p int) {
	n := &g.needs
	d := g.state.Players[p].FreightersFree - int16(n.colonyShips[p][2])
	n.freightersWanted[p] = 0
	if d < 0 {
		n.freightersWanted[p] = uint8((4 - int32(d)) / 5)
	}
}

// playerWarLevel is Player_War_Level_ (0xd1006): 1..3 for the worst treaty
// status 4..6 with a player in contact, else 0.
func (g *Game) playerWarLevel(p int) int16 {
	s := &g.state
	lvl := int16(0)
	for q := int(s.NumPlayers) - 1; q >= 0; q-- {
		if q == p || s.Players[p].Contact[q] == 0 {
			continue
		}
		switch t := s.Players[p].TreatyStatus[q]; {
		case t == 4 && lvl < 1:
			lvl = 1
		case t == 5 && lvl < 2:
			lvl = 2
		case t == 6 && lvl < 3:
			lvl = 3
		}
	}
	return lvl
}
