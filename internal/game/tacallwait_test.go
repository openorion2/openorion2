package game

import "testing"

// allWaitGame is tacGame with four ships in initiative order: attacker's
// records 1 and 3, defender's 2 and 4, the defender's record 2 to move.
func allWaitGame(t *testing.T) *Game {
	t.Helper()
	g := tacGame(t)
	tc := g.tc
	tc.n = 5
	for i := int16(3); i < 5; i++ {
		T := g.tsp(i)
		copy(T[:], g.tsp(i - 2)[:])
		T.set16(tsID, i)
	}
	tc.initOrder = 1
	tc.order[0], tc.order[1], tc.order[2], tc.order[3], tc.order[4] = -1, 2, 3, 4, -10
	for k := 5; k < len(tc.order); k++ {
		tc.order[k] = -1
	}
	tc.curPtr, tc.cur = 1, 2
	return g
}

// TestAllWaitOrder: community patch 1.51's H. Record 1 (the attacker's) has
// moved; the defender at record 2 presses H and outwaits the attacker.
func TestAllWaitOrder(t *testing.T) {
	t.Parallel()
	g := allWaitGame(t)
	tc := g.tc
	step := func(what string, done int16, want int16) {
		t.Helper()
		if done != 0 || tc.cur != want {
			t.Fatalf("%s: record %d (done %d), want %d", what, tc.cur, done, want)
		}
	}
	step("defender's H", g.allWait(), 3)
	// WAIT passes over the defender's record 4: the attacker's 3 again.
	step("attacker's WAIT", g.nextShip(), 3)
	// The attacker is all done: the defender moves, from record 4 on.
	tc.order[tc.curPtr] = -1
	step("attacker's DONE", g.nextShip(), 4)
	step("defender's WAIT", g.nextShip(), 2)
	// A second H with nobody else left keeps the ship (0x41abca from it).
	step("defender's H alone", g.allWait(), 2)

	// Both press H: the attacker first, then the defender outwaits him.
	g = allWaitGame(t)
	tc = g.tc
	tc.order[0] = 1
	tc.curPtr, tc.cur = 0, 1
	step("attacker's H", g.allWait(), 2)
	step("defender's H", g.allWait(), 3)
	step("attacker's WAIT", g.nextShip(), 1)

	// Without initiative's flags WAIT goes on in order (1.40).
	g = allWaitGame(t)
	tc = g.tc
	step("plain WAIT", g.nextShip(), 3)
	step("plain WAIT", g.nextShip(), 4)
}
