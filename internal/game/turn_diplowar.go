package game

import "github.com/openorion2/openorion2/internal/save"

// War, peace and treaty breaking (DIPLOMAC.C), shared by the diplomacy
// steps. Spec: docs/turn/overview.md, phase C.

// lowerRelation subtracts d from a relation byte, floored at -100 before and
// after the byte subtraction as the original does.
func lowerRelation(r *int8, d int32) {
	if int32(*r)-d < -100 {
		*r = -100
	} else {
		*r -= int8(d)
	}
	if *r < -100 {
		*r = -100
	}
}

// declareWar is Declare_War_ (0x51078): a puts itself at war with b.
func (g *Game) declareWar(a, b int16) {
	s := &g.state
	n := s.NumPlayers
	// The original indexes before its bounds check; the port skips those.
	if a >= 0 && a < 8 && b >= 0 && b < 8 {
		if A := &s.Players[a]; A.Objective == save.ObjectiveHuman && A.WarDeclared[b] == 0 {
			A.WarDeclared[b] = 1
		}
		if B := &s.Players[b]; B.Objective == save.ObjectiveHuman && B.WarDeclared[a] == 0 {
			B.WarDeclared[a] = 1
		}
	}
	if a >= n || b >= n {
		return
	}
	A, B := &s.Players[a], &s.Players[b]
	if int8(A.TreatyStatus[b]) >= 4 {
		return
	}
	A.DemandState[b], B.DemandState[a] = 0, 0
	A.WarLosses[b], B.WarLosses[a] = 0, 0
	A.WarLosses2[b], B.WarLosses2[a] = 0, 0
	A.Diplo70F[b], B.Diplo70F[a] = 0, 0
	g.breakTrade(a, b)
	g.breakResearch(a, b)
	g.breakTribute(a, b)
	g.breakTreaties(a, b)
	A.BaseRelations[b] -= 10
	B.BaseRelations[a] -= 10
	A.Relations[b] = int8(-75 - int32(s.Random.Random(25)))
	B.Relations[a] = A.Relations[b]
	var st uint8 = 4
	if A.Objective == save.ObjectiveHuman || B.Objective == save.ObjectiveHuman {
		st = 5
		if g.settings.Difficulty() >= 3 {
			st = 6
		}
	}
	A.TreatyStatus[b], B.TreatyStatus[a] = st, st
	for k := 0; k < 3; k++ {
		A.Modifiers[k][b], B.Modifiers[k][a] = -200, -200
	}
	A.Modifiers[3][b], B.Modifiers[3][a] = -130, -130
	A.Diplo7E4[b], B.Diplo7E4[a] = 0xff, 0xff
	A.TurnsAtWar[b], B.TurnsAtWar[a] = 0, 0
	A.TreatyTimer[b], B.TreatyTimer[a] = 0, 0
}

// declarePeace is Declare_Peace_ (0x524fb): a timed peace of 30 turns.
func (g *Game) declarePeace(a, b int16) {
	s := &g.state
	A, B := &s.Players[a], &s.Players[b]
	A.TreatyStatus[b], B.TreatyStatus[a] = 3, 3
	A.Relations[b] += 50 // wraps as a byte
	if A.Relations[b] > 0 {
		A.Relations[b] = 0
	}
	B.Relations[a] = A.Relations[b]
	A.TreatyTimer[b], B.TreatyTimer[a] = 30, 30
	A.DemandState[b], B.DemandState[a] = 0, 0
	A.WarLosses[b], B.WarLosses[a] = 0, 0
	A.WarLosses2[b], B.WarLosses2[a] = 0, 0
	A.Diplo70F[b], B.Diplo70F[a] = 0, 0
}

// breakTreaties is Break_Treaties_ (0x5138e): a ends its pact or timed
// peace with b; witnesses in contact take note.
func (g *Game) breakTreaties(a, b int16) {
	s := &g.state
	if b >= s.NumPlayers || a >= s.NumPlayers {
		return
	}
	A, B := &s.Players[a], &s.Players[b]
	if A.TreatyStatus[b] == 0 || int8(B.TreatyStatus[a]) >= 4 {
		return
	}
	B.Diplo727[a] = 1
	B.DemandState[a]++
	B.Diplo6DF[a] = B.TreatyStatus[a]
	A.TreatyTimer[b], B.TreatyTimer[a] = 0, 0
	var pen int16
	switch A.TreatyStatus[b] {
	case 1:
		pen = -20
	case 2, 3:
		pen = -40
	}
	if A.Personality == 4 {
		pen *= 3
	}
	A.Diplo6D7[b] -= int8(pen)
	witness := func(k int16) {
		K := &s.Players[k]
		if K.Personality == 4 {
			K.Diplo7EE[a] -= 20
		} else {
			K.Diplo7EE[a] -= 10
		}
		K.Diplo7F6[a] = uint8(b)
		K.Diplo7FE[a] = A.TreatyStatus[b]
	}
	for k := int16(0); k < s.NumPlayers; k++ {
		if k == a || k == b || s.Players[k].Eliminated != 0 {
			continue
		}
		if A.Contact[k] == 1 || B.Contact[k] == 1 {
			witness(k)
		}
	}
	witness(b)
	if A.TreatyStatus[b] == 2 {
		lowerRelation(&A.BaseRelations[b], int32(pen))
		B.BaseRelations[a] = A.BaseRelations[b]
	}
	if pen != 0 {
		A.Diplo747[b] = A.TreatyStatus[b]
		B.Diplo747[a] = A.TreatyStatus[b]
		B.Relations[a] = A.Relations[b]
	}
	A.TreatyStatus[b], B.TreatyStatus[a] = 0, 0
	A.Diplo7E4[b], B.Diplo7E4[a] = 0, 0
	if A.Relations[b] < -100 {
		A.Relations[b] = -100
	}
	B.Relations[a] = A.Relations[b]
	for k := 0; k < 4; k++ {
		A.Modifiers[k][b], B.Modifiers[k][a] = -200, -200
	}
}

// breakTrade is Break_Trade_ (0x519ac).
func (g *Game) breakTrade(a, b int16) {
	g.breakAgreement(a, b, func(p *save.Player) *[8]uint8 { return &p.TradeTreaty }, 7)
}

// breakResearch is Break_Research_ (0x5175b), the same as Break_Trade_ on
// the research treaty.
func (g *Game) breakResearch(a, b int16) {
	g.breakAgreement(a, b, func(p *save.Player) *[8]uint8 { return &p.ResearchTreaty }, 8)
}

// breakAgreement is the body of Break_Trade_ and Break_Research_; field
// gives the treaty array, kind the code left in Diplo6DF and Diplo747.
func (g *Game) breakAgreement(a, b int16, field func(*save.Player) *[8]uint8, kind uint8) {
	s := &g.state
	if b >= s.NumPlayers || a >= s.NumPlayers {
		return
	}
	A, B := &s.Players[a], &s.Players[b]
	ta, tb := field(A), field(B)
	if ta[b] == 0 {
		return
	}
	B.Diplo6DF[a] = kind
	had := ta[b]
	if A.Relations[b] < -100 {
		A.Relations[b] = -100
	}
	B.Relations[a] = A.Relations[b]
	ta[b], tb[a] = 0, 0
	A.Diplo7E4[b], B.Diplo7E4[a] = 0, 0
	if int8(B.TreatyStatus[a]) >= 4 || had != 1 {
		return
	}
	if A.Personality == 4 {
		A.Diplo6D7[b] -= 15
	} else {
		A.Diplo6D7[b] -= 5
	}
	r := &s.Random
	d := int32(int16(r.Random(20) + r.Random(20)))
	lowerRelation(&A.Relations[b], d)
	B.Relations[a] = A.Relations[b]
	lowerRelation(&A.BaseRelations[b], int32(int16(r.Random(20))))
	B.BaseRelations[a] = A.BaseRelations[b]
	A.Diplo747[b] = kind
	for k := int16(0); k < s.NumPlayers; k++ {
		if k != a && k != b {
			A.Diplo6D7[k] -= 5
		}
	}
}

// breakTribute is Break_Tribute_ (0x51c02): tribute between a and b ends;
// one a was paying sours relations.
func (g *Game) breakTribute(a, b int16) {
	s := &g.state
	if b >= s.NumPlayers || a >= s.NumPlayers {
		return
	}
	A, B := &s.Players[a], &s.Players[b]
	B.DemandState[a]++
	paid := A.Tribute[b]
	if A.Relations[b] < -100 {
		A.Relations[b] = -100
	}
	B.Relations[a] = A.Relations[b]
	A.Tribute[b], B.Tribute[a] = 0, 0
	A.Diplo7E4[b], B.Diplo7E4[a] = 0, 0
	if int8(B.TreatyStatus[a]) >= 4 || paid <= 0 {
		return
	}
	if A.Personality == 4 {
		A.Diplo6D7[b] -= 20
	} else {
		A.Diplo6D7[b] -= 10
	}
	r := &s.Random
	lowerRelation(&A.Relations[b], int32(int16(r.Random(50)+r.Random(50))))
	B.Relations[a] = A.Relations[b]
}
