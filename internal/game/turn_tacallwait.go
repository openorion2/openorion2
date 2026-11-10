package game

// Community patch 1.51's all-wait (hot key H, 1.50.24): with ship initiative
// every ship of the side waits at once. Spec: docs/combat/tactical.md.

// allWaitSides is 0x41abca's view of the flags (0x41ac27..0x41ac81): which
// side's ships Next_Ship_ passes over, attacker first.
func (g *Game) allWaitSides() (skip [2]bool) {
	tc := g.tc
	skip = [2]bool{tc.allWait[tc.att] != 0, tc.allWait[tc.def] != 0}
	if !skip[0] && !skip[1] {
		return skip
	}
	// Who has a ship left to move (0x408842 with the closure at 0x40518a).
	var left [2]bool
	for k := 0; k < len(tc.order) && tc.order[k] != -10; k++ {
		if o := tc.order[k]; o >= 0 && g.allWaitMayMove(o, [2]bool{}) {
			left[g.allWaitSide(o)] = true
		}
	}
	// A side that waits for one with nothing left moves; when both wait,
	// the defender outwaits the attacker.
	skip[0] = skip[0] && left[1]
	skip[1] = skip[1] && left[0]
	if skip[0] && skip[1] {
		skip[0] = false
	}
	return skip
}

// allWaitSide is 1 for the defender's ships, 0 for any other's.
func (g *Game) allWaitSide(o int16) int {
	return int(b2i(int16(g.tsp(o).u8(tsOwner)) == g.tc.def))
}

// allWaitSkips reports whether record o's side waits for the other.
func (g *Game) allWaitSkips(o int16, skip [2]bool) bool { return skip[g.allWaitSide(o)] }

// allWaitMayMove is 0x408842's test of an entry of the order (the planet
// with defences only, functional, its side not ended) and the side's wait.
func (g *Game) allWaitMayMove(o int16, skip [2]bool) bool {
	tc := g.tc
	switch {
	case o < 0, o == 0 && g.doesCombatPlanetHaveDefenses() == 0, !g.combatShipIsFunctional(o):
		return false
	}
	return tc.sideEnd[g.tsp(o).u8(tsOwner)] == 0 && !g.allWaitSkips(o, skip)
}

// allWait is 0x41b06a mode 3: the current ship's side waits, and Next_Ship_
// looks from that ship on. It returns Next_Ship_'s value.
func (g *Game) allWait() int16 {
	tc := g.tc
	tc.allWait[g.tsp(tc.cur).u8(tsOwner)] = 1
	return g.nextShipFrom(true)
}
