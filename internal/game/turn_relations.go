package game

import "github.com/openorion2/openorion2/internal/save"

// Relations bookkeeping shared by the turn (DIPLOMAC.C). Spec:
// docs/turn/overview.md, phase C.

// changeRelations is Change_Relations_ (0x4e3b5): player b's attitude to a
// moves by delta, scaled by personality, traits and treaties, and the
// biggest change of the turn is remembered as b's message about a. The
// relations themselves stay symmetric.
func (g *Game) changeRelations(delta int16, a, b int, kind int16, arg5, arg6 int16) {
	s := &g.state
	if int16(a) >= 8 || int16(b) >= 8 || int16(a) > s.NumPlayers || int16(b) > s.NumPlayers || a == b || delta == 0 {
		return
	}
	A, B := &s.Players[a], &s.Players[b]
	if B.Objective == save.ObjectiveHuman || A.Eliminated == 1 || B.Eliminated == 1 ||
		A.Contact[b] == 0 || B.Contact[a] == 0 || A.TreatyStatus[b] == 6 {
		return
	}
	if kind == 10 {
		g.rewardCheck(delta, a, b, arg5, arg6)
	}
	remember := func(v int16) {
		B.MessageParam[a] = v
		B.MessageThisTurn[a] = uint8(kind)
		B.Diplo66F[a] = arg6
		B.Diplo67F[a] = arg5
	}
	if delta == -10000 {
		if A.Objective == save.ObjectiveHuman && kind > 0 {
			remember(10000)
		}
		return
	}
	d := delta
	r := int32(B.Relations[a])
	switch {
	case delta > 0 && r < 0:
		d = 2 * delta
		if r+int32(d) > 10 {
			d = int16(10 - r)
		}
	case delta <= 0 && r > 0:
		d = 2 * delta
	default:
		var div int32
		if delta > 0 {
			div = r/25 + 1
		} else {
			div = r/-25 + 1
		}
		if int16(div) == 0 {
			div = 1
		}
		d = int16(int32(delta) / int32(int16(div)))
	}
	if B.Personality == 4 && d < 0 {
		d <<= 1
	}
	if B.Personality == 0 && d < 0 {
		d = int16(3 * int32(d) / 2)
	}
	if B.Personality == 0 && d > 0 {
		d = int16(3 * int32(d) / 4)
	}
	base := d
	if A.Picks[save.PickCharismatic] == 1 {
		if d > 0 {
			d <<= 1
		} else {
			d /= 2
		}
	}
	if t := B.TreatyStatus[a]; (t == 4 || t == 5) && kind >= 1 && kind <= 9 && d < 0 {
		if A.Objective == save.ObjectiveHuman {
			B.Modifiers[3][a] += d / 4
			A.Modifiers[3][b] -= d / 4
			return
		}
		d = 0
	}
	if int8(B.TreatyStatus[a]) >= 4 {
		return
	}
	late := int32(s.Stardate)-35000 > 100
	ais := A.Objective != save.ObjectiveHuman && B.Objective != save.ObjectiveHuman
	if ais && d > 0 && late {
		d <<= 1
	}
	if ais && d < 0 && late {
		d = int16(int32(d) / (int32(g.settings.Difficulty())/2 + 1))
	}
	if d != 0 {
		B.Relations[a] += int8(d)
		if B.Relations[a] < -100 {
			B.Relations[a] = -100
		}
		if B.Relations[a] > 100 {
			B.Relations[a] = 100
		}
		for k := 0; k < 3; k++ {
			B.Modifiers[k][a] += d
			A.Modifiers[k][b] = B.Modifiers[k][a]
		}
		if base < 0 {
			A.Modifiers[3][b] -= base / 4
			B.Modifiers[3][a] += base / 4
		} else {
			B.Modifiers[3][a] += base
		}
		if abs32(int32(B.MessageParam[a])) < abs32(int32(d)) {
			remember(d)
		}
	}
	if int8(B.TreatyStatus[a]) >= 4 && B.Relations[a] > -25 {
		B.Relations[a] = -25
	}
	if B.TreatyStatus[a] != 2 && B.Relations[a] > 65 {
		B.Relations[a] = 65
	}
	A.Relations[b] = B.Relations[a]
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// rewardCheck is Reward_Check_ (0x4ea03): players who asked i to deal with
// b reward a for a's act against b.
func (g *Game) rewardCheck(delta int16, a, b int, arg5, arg6 int16) {
	s := &g.state
	for i := 0; i < int(s.NumPlayers); i++ {
		if i == a || i == b {
			continue
		}
		if int16(int8(s.Players[i].Diplo7E4[a])) == int16(b) {
			g.changeRelations(200, a, i, 15, arg5, int16(b))
		}
		// The other branch, a gratitude roll for players hostile to b,
		// compares WarLosses[i] with itself (0x4eaac) and never runs.
	}
}
