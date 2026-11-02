package game

import "github.com/openorion2/openorion2/internal/clib"

// Where the AI's colony ships go (AIMOVE.C): Assign_New_Colony_Ship_Destinations_
// and its helpers. Spec: docs/turn/overview.md, Move_All_AI_.

// starHasMonster is Star_Has_Monster_ (0xd892b): a monster sits at st.
func (s *State) starHasMonster(st int) bool {
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if sh.Owner >= 8 && int(sh.Location) == st && sh.Status == 0 {
			return true
		}
	}
	return false
}

// iSoonestEnemyColonyShip is I_Soonest_Enemy_Colony_Ship_ (0xd87bb): the
// enemy colony ship at st, or the one arriving there first, or -1.
func (s *State) iSoonestEnemyColonyShip(enemy []int16, st int) int {
	best, eta := -1, int16(0x7fff)
	for i := len(enemy) - 1; i >= 0; i-- {
		sh := &s.Ships[enemy[i]]
		if int16(st) == sh.Location {
			return i
		}
		if int(sh.Location)-500 != st && int(sh.Location)-1000 != st {
			continue
		}
		if int16(sh.ETA) < eta {
			eta, best = int16(sh.ETA), i
		}
	}
	return best
}

// moveShipsWithPossibleIntermediate is Move_Ships_With_Possible_Intermediate_
// (0xd7923): the turns to dest directly or via one of p's colony stars. The
// via test compares the move's can-move byte, not its turns, with the best.
// The ships are put at each candidate in turn and all reset to the first
// ship's place.
func (g *Game) moveShipsWithPossibleIntermediate(ma *moveAI, ships []int16, dest int, via, turns *int16) {
	s := &g.state
	first := &s.Ships[ships[0]]
	loc, x, y := first.Location, first.X, first.Y
	*via, *turns = -1, 0x7fff
	if res := g.shipsTryToMoveTo(ships, dest); res.canMove {
		*turns = int16(uint8(res.turns))
		*via = int16(dest)
		return
	}
	bestT, bestVia := int16(0x7fff), int16(-1)
	for ci := int(s.NumColonies) - 1; ci >= 0; ci-- {
		c := &s.Colonies[ci]
		if int16(c.Owner) != int16(ma.p) {
			continue
		}
		m := int16(s.Planets[c.Planet].Star)
		if m == loc || m == int16(dest) {
			continue
		}
		res := g.shipsTryToMoveTo(ships, int(m))
		if !res.canMove || int16(b2u(res.canMove)) >= bestT {
			continue
		}
		t := int16(uint8(res.turns))
		n := 0
		for ; ships[n] != -1; n++ {
			sh := &s.Ships[ships[n]]
			sh.Location = m
			sh.X, sh.Y = s.Stars[m].X, s.Stars[m].Y
		}
		res = g.shipsTryToMoveTo(ships, dest)
		for n--; n >= 0; n-- {
			sh := &s.Ships[ships[n]]
			sh.Location, sh.X, sh.Y = loc, x, y
		}
		if !res.canMove {
			continue
		}
		if t += int16(uint8(res.turns)); t < bestT {
			bestT, bestVia = t, m
		}
	}
	if bestVia != -1 && bestT < *turns {
		*turns, *via = bestT, bestVia
	}
}

// getEnemyStrengthOnTurn is Get_Enemy_Strength_On_Turn_ (0xd7c04): other
// owners' ship strength against p arriving at st by turn (perTurn) and in
// total by then (cum).
func (g *Game) getEnemyStrengthOnTurn(ma *moveAI, st int, perTurn, cum *[10]uint32) {
	s := &g.state
	a := g.ai
	p := ma.p
	blk := aiShipStrengths
	if s.Stars[st].InNebula != 0 {
		blk = aiShipStrengthsInNebula
	}
	*perTurn = [10]uint32{}
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if int16(sh.Owner) == int16(p) || int8(sh.Status) >= 3 {
			continue
		}
		v := uint32(a.u16(a.at(blk) + i*16 + p*2))
		switch int(sh.Location) {
		case st:
			perTurn[0] += v
		case st + 500, st + 1000:
			if sh.ETA < 10 {
				perTurn[sh.ETA] += v
			}
		}
	}
	cum[0] = perTurn[0]
	for k := 1; k < 10; k++ {
		cum[k] = cum[k-1] + perTurn[k]
	}
}

// shipRoughStrengthAtStar is Ship_Rough_Strength_At_Star_ (0xd7d53): the
// ship against the players present at st (or met), or the monster there.
func (g *Game) shipRoughStrengthAtStar(ma *moveAI, ship, st int) uint32 {
	s := &g.state
	a := g.ai
	p := ma.p
	nebula := s.Stars[st].InNebula != 0
	blk := aiShipStrengths
	if nebula {
		blk = aiShipStrengthsInNebula
	}
	minA, minB := ^uint32(0), ^uint32(0)
	var sumA, sumB, cnt uint32
	for q := int(s.NumPlayers) - 1; q >= 0; q-- {
		if q == p {
			continue
		}
		v := uint32(a.u16(a.at(blk) + ship*16 + q*2))
		if a.extRange(st, q) && (a.inRange(st, q) || s.Players[q].Objective == objectiveHuman) {
			cnt++
			sumA += v
			minA = min(minA, v)
		}
		if a.u8(a.at(aiPlayerInfo)+p*0x3a+0xe+q) != 0 {
			sumB += v
			minB = min(minB, v)
		}
	}
	if m := ma.monsterAt[st]; m != 0 {
		var v uint32
		if nebula {
			v = g.shipStrengthVsShipsInNebula(ship, int(m))
		} else {
			v = g.shipStrengthVsShips(ship, int(m))
		}
		return min(uint32(uint16(v)), minA)
	}
	if cnt > 0 {
		return ((minA+2)*cnt + sumA - 1) / (2 * cnt)
	}
	if met := int32(int16(a.u16(a.at(aiPlayerInfo) + p*0x3a + 0xc))); met > 0 {
		return ((minB+2)*uint32(met) + sumB - 1) / uint32(2*met)
	}
	return uint32(s.Ships[ship].Design.Size) + 1
}

// escortOK is the shared test of Get_Best_Escorted_Colony_Ship_: the escort
// matches (from a star) or outweighs (in flight) every later arrival.
func escortOK(str uint32, t int, perTurn *[10]uint32, strict bool) bool {
	for k := t + 1; k < 10; k++ {
		if str < perTurn[k] || strict && str == perTurn[k] {
			return false
		}
	}
	return true
}

// getBestEscortedColonyShip is Get_Best_Escorted_Colony_Ship_ (0xd7ff7): the
// colony ship that can reach st soonest with enough escort (enemies arrive
// from enemyETA on), sent with it (escorts get AI mission 4).
func (g *Game) getBestEscortedColonyShip(ma *moveAI, A *[]int16, st int, enemyETA int32) bool {
	s := &g.state
	a := g.ai
	p := ma.p
	var their, perTurn, cum [10]uint32
	g.getEnemyStrengthOnTurn(ma, st, &perTurn, &cum)
	for q := int(s.NumPlayers) - 1; q >= 0; q-- {
		if q == p {
			continue
		}
		if t := s.Players[p].TreatyStatus[q]; t >= 1 && t <= 3 && s.Players[q].Objective != objectiveHuman {
			continue
		}
		for k := 9; k >= 0; k-- {
			their[k] += a.u32(a.at(aiPossibleShipStrengths) + st*0xa00 + q*0x140 + p*0x28 + k*4)
		}
	}
	bestT, bestI, bestVia := int32(0x7fff), -1, int16(-1)
	mode := uint8(0xff)
	nEsc, nThere := int32(-1), int32(-1)
	allShip := func(k int) int16 { return int16(a.u16(a.at(aiAllShips) + k*2)) }
	idxAt := func(here int) int { return a.at(aiAllCombatShipsIndex) + p*0x92 + here*2 }
	L := make([]int16, 0, 502)
	for i := len(*A) - 1; i >= 0; i-- {
		cs := (*A)[i]
		L = append(L[:0], cs)
		sh := &s.Ships[cs]
		if sh.Location < 0x48 {
			here := int(sh.Location)
			idx := int(int16(a.u16(idxAt(here))))
			if idx == -1 {
				continue
			}
			var escStr, allStr int32
			need := enemyETA < 10 || cum[9] > 0
			count := int32(0)
			for k := idx; k < int(a.nAllShips); k++ {
				o := &s.Ships[allShip(k)]
				if int16(o.Owner) != int16(p) || o.Location != int16(here) {
					break
				}
				count++
				allStr += int32(int16(g.shipRoughStrengthAtStar(ma, int(allShip(k)), st)))
			}
			esc := int32(0)
			for j := idx; ; j++ {
				if int(a.nAllShips) <= j {
					break
				}
				e := allShip(j)
				o := &s.Ships[e]
				if int16(o.Owner) != int16(p) || o.Location != int16(here) {
					break
				}
				escStr += int32(int16(g.shipRoughStrengthAtStar(ma, int(e), st)))
				L = append(L, e)
				esc++
				var via, turns int16
				g.moveShipsWithPossibleIntermediate(ma, append(L, -1), st, &via, &turns)
				if via == -1 || int32(turns) > enemyETA {
					break
				}
				if !need && escStr*2 < allStr {
					continue
				}
				if turns >= 10 || int32(turns) >= bestT {
					break
				}
				x := int(turns) + 2
				if x >= 10 {
					x = 9
				}
				if cum[x] < uint32(escStr) {
					ok := !(uint32(escStr) < their[turns] && escStr < allStr)
					ok = ok && escortOK(uint32(escStr), int(turns), &perTurn, false)
					if ok {
						bestT, bestI, bestVia = int32(turns), i, via
						nEsc, mode, nThere = esc, 2, count
					}
				}
				if !need && escStr*2 >= allStr {
					break
				}
			}
			continue
		}
		var escStr uint32
		for k := int(s.NumShips) - 1; k >= 0; k-- {
			o := &s.Ships[k]
			if int16(o.Owner) != int16(p) || o.Location != sh.Location || int8(o.Status) >= 3 || o.X != sh.X || o.Y != sh.Y {
				continue
			}
			escStr += uint32(int32(int16(g.shipRoughStrengthAtStar(ma, k, st))))
			L = append(L, int16(k))
		}
		L = append(L, -1)
		if int(sh.Location)-500 == st {
			eta := int32(sh.ETA)
			if sh.ETA >= 10 || eta > enemyETA || eta >= bestT || cum[eta] >= escStr || !escortOK(escStr, int(eta), &perTurn, true) {
				continue
			}
			bestT, bestI, mode, bestVia = eta, i, 0, int16(st)
			nEsc, nThere = int32(len(L)-2), int32(len(L)-2)
			continue
		}
		var via, turns int16
		g.moveShipsWithPossibleIntermediate(ma, L, st, &via, &turns)
		t := int32(turns)
		if via == -1 || turns >= 10 || t > enemyETA || t >= bestT || cum[t] >= escStr || !escortOK(escStr, int(t), &perTurn, true) {
			continue
		}
		bestT, bestI, bestVia, mode = t, i, via, 1
		nEsc, nThere = int32(len(L)-2), int32(len(L)-2)
	}
	if bestI == -1 {
		return false
	}
	cs := (*A)[bestI]
	L = append(L[:0], cs)
	switch mode {
	case 1:
		c := &s.Ships[cs]
		for k := int(s.NumShips) - 1; k >= 0; k-- {
			o := &s.Ships[k]
			if int16(o.Owner) != int16(p) || o.Location != c.Location || int8(o.Status) >= 3 || o.X != c.X || o.Y != c.Y {
				continue
			}
			o.AIMission = 4
			L = append(L, int16(k))
		}
	case 2:
		here := int(s.Ships[cs].Location)
		idx := int(int16(a.u16(idxAt(here))))
		for b := int32(0); b < nEsc; b++ {
			e := allShip(idx + int(b))
			s.Ships[e].AIMission = 4
			L = append(L, e)
		}
		if nEsc < nThere {
			a.setU16(idxAt(here), a.u16(idxAt(here))+uint16(nEsc))
		} else {
			a.setU16(idxAt(here), 0xffff)
		}
	}
	L = append(L, -1)
	if mode != 0 {
		res := g.shipsTryToMoveTo(L, int(bestVia))
		g.makeShipsMoveTo(L, int(bestVia), &res)
	}
	n := len(*A) - 1
	(*A)[bestI] = (*A)[n]
	*A = (*A)[:n]
	return true
}

// getBestUnescortedColonyShip is Get_Best_Unescorted_Colony_Ship_ (0xd7b48):
// the colony ship reaching st soonest flies there alone.
func (g *Game) getBestUnescortedColonyShip(ma *moveAI, A *[]int16, st int) {
	bestT, bestI, bestVia := int16(0x7fff), -1, int16(-1)
	for i := len(*A) - 1; i >= 0; i-- {
		var via, turns int16
		g.moveShipsWithPossibleIntermediate(ma, []int16{(*A)[i], -1}, st, &via, &turns)
		if via != -1 && turns < bestT {
			bestT, bestI, bestVia = turns, i, via
		}
	}
	if bestI == -1 {
		return
	}
	one := []int16{(*A)[bestI], -1}
	res := g.shipsTryToMoveTo(one, int(bestVia))
	g.makeShipsMoveTo(one, int(bestVia), &res)
	n := len(*A) - 1
	(*A)[bestI] = (*A)[n]
	*A = (*A)[:n]
}

// sortCandidateStars is Sort_candidate_stars_ (0xd8847): by the weakest
// strength p can bring against any player in range of a (b is read under
// a's range test too), unknown counting as 0.
func (g *Game) sortCandidateStars(ma *moveAI) func(x, y *uint8) int {
	s := &g.state
	a := g.ai
	p := ma.p
	return func(x, y *uint8) int {
		minA, minB := ^uint32(0), ^uint32(0)
		for q := int(s.NumPlayers) - 1; q >= 0; q-- {
			if q == p || !a.inRange(int(*x), q) {
				continue
			}
			minA = min(minA, a.u32(a.at(aiPossibleShipStrengths)+int(*x)*0xa00+p*0x140+q*0x28))
			minB = min(minB, a.u32(a.at(aiPossibleShipStrengths)+int(*y)*0xa00+p*0x140+q*0x28))
		}
		if minA == ^uint32(0) {
			minA = 0
		}
		if minB == ^uint32(0) {
			minB = 0
		}
		switch {
		case minA < minB:
			return -1
		case minB < minA:
			return 1
		}
		return 0
	}
}

// assignNewColonyShipDestinations is Assign_New_Colony_Ship_Destinations_
// (0xd896f): p's free colony ships go to the best free planets (escorted
// where enemies or monsters lurk), the rest to p's stage-point stars.
func (g *Game) assignNewColonyShipDestinations(ma *moveAI) {
	s := &g.state
	a := g.ai
	p := ma.p
	var A, E []int16
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if sh.Design.Type != 1 {
			continue
		}
		if int16(sh.Owner) == int16(p) && s.canOrderShip(i) {
			A = append(A, int16(i))
		} else if int8(sh.Status) < 3 {
			E = append([]int16{int16(i)}, E...)
		}
	}
	for len(ma.planets) > 0 && len(A) > 0 {
		st := int(s.Planets[ma.planets[len(ma.planets)-1].planet].Star)
		others := uint32(a.u8(a.at(aiStarInExtendedRange)+st)) &^ (uint32(1) << uint(p))
		if others != 0 || a.starInfo(st)[6*p+6] != 0 || s.starHasMonster(st) {
			soon := s.iSoonestEnemyColonyShip(E, st)
			eta := int32(0x7fff)
			if soon != -1 {
				eta = int32(s.Ships[E[soon]].ETA)
			}
			if !g.getBestEscortedColonyShip(ma, &A, st, eta) && soon != -1 {
				E[soon] = E[0]
				E = E[1:]
			}
		} else {
			d := g.settings.Difficulty()
			if d >= 2 || d == 1 && s.Players[p].Objective == 2 || a.inRange(st, p) {
				g.getBestUnescortedColonyShip(ma, &A, st)
			}
		}
		ma.planets = ma.planets[:len(ma.planets)-1]
	}
	if len(A) == 0 {
		return
	}
	var seen [72]bool
	var C []uint8
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		S := &s.Stars[st]
		if !maskBit(S.AIStagePoint, p) || maskBit(S.Blockaded, p) || seen[st] {
			continue
		}
		seen[st] = true
		C = append(C, uint8(st))
	}
	for k := len(A) - 1; k >= 0; k-- {
		shuffleChar(&s.Random, C)
		clib.Qsort(C, 1, g.sortCandidateStars(ma))
		for i := len(C) - 1; i >= 0; i-- {
			one := []int16{A[k], -1}
			res := g.shipsTryToMoveTo(one, int(C[i]))
			if !res.canMove {
				continue
			}
			g.makeShipsMoveTo(one, int(C[i]), &res)
			n := len(C) - 1
			C[i] = C[n]
			C = C[:n]
			break
		}
	}
}
