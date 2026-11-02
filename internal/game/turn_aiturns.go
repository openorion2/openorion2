package game

import "github.com/openorion2/openorion2/internal/save"

// Travel times the AI plans with (SHIPMOVE.C, AIDATA.C). Spec:
// docs/turn/ai-data.md.

// playerNTurnsFromStarToStar is Player_N_Turns_From_Star_To_Star_ (0xffbd6):
// turns p's ships need from one star to another, -1 if they cannot go.
func (g *Game) playerNTurnsFromStarToStar(p, from, to int) int {
	s := &g.state
	A, B := s.starRecord(from), s.starRecord(to)
	if int16(from) == int16(to) {
		return 0
	}
	if maskBit(A.Stargate, p) && maskBit(B.Stargate, p) {
		return 1
	}
	if int16(A.WormholeTo) == int16(to) {
		return 1
	}
	speed := int16(s.Players[p].FTLSpeed)
	gate := false
	if maskBit(A.Jumpgate, p) && maskBit(B.Jumpgate, p) {
		gate = true
		speed += 3
	}
	if !gate && blackHoleBit(A, int(int16(to))) || speed == 0 {
		return -1
	}
	x, y := A.X, A.Y
	n := 0
	for x != B.X || y != B.Y {
		n++
		var neb, stop bool
		g.movePlayer1TurnToStar(p, x, y, to, speed, &x, &y, &neb, &stop)
	}
	return n
}

// playerNTurnsFromStarToStarWithIntermediate is
// Player_N_Turns_From_Star_To_Star_With_Intermediate_ (0xd3576).
func (g *Game) playerNTurnsFromStarToStarWithIntermediate(p, from, to int, out *int16, stealth bool) int16 {
	s := &g.state
	a := g.ai
	nt := func(i, j int) uint8 { return a.u8(a.at(aiNTurnsFromTo) + i*0x240 + j*8 + p) }
	e := a.at(aiNTurnsFromToIntermediate) + from*0x900 + to*0x20 + p*4
	if a.u8(e+1) == 0xff {
		P := &s.Players[p]
		if P.FTLSpeed != 0 && P.AppStatus[0x17a-0x117] != 3 && P.AppStatus[0x1c1-0x117] != 3 &&
			s.starRecord(from).WormholeTo == -1 && s.starRecord(to).WormholeTo == -1 {
			if d := nt(from, to); d < 0xff {
				sp := int32(P.FTLSpeed)
				est := (sp + g.parsecsBetweenStars(from, to) - 1) / sp
				if int32(d) <= est {
					if a.inRange(to, p) {
						a.setU8(e, uint8(est))
						a.setU8(e+1, uint8(to))
					} else {
						a.setU8(e+1, 0x48)
					}
					a.setU8(e+2, uint8(est))
					a.setU8(e+3, uint8(to))
				}
			}
		}
		if a.u8(e+1) == 0xff {
			for m := int(s.NumStars) - 1; a.u8(e) > 1 && m >= 0; m-- {
				if !a.extRange(m, p) {
					continue
				}
				M := &s.Stars[m]
				if m != to && (M.SpectralClass == save.SpectralBlackHole || M.Special == 9 ||
					!maskBit(M.ColonyOwners, p) && M.ColonyOwners != 0) {
					continue
				}
				t := int32(nt(from, m)) + int32(nt(m, to))
				via := uint8(m)
				if m == from {
					via = uint8(to)
				}
				if a.inRange(m, p) && t < int32(a.u8(e)) {
					a.setU8(e, uint8(t))
					a.setU8(e+1, via)
				}
				if t < int32(a.u8(e+2)) {
					a.setU8(e+2, uint8(t))
					a.setU8(e+3, via)
				}
			}
			if a.u8(e+1) == 0xff {
				a.setU8(e+1, 0x48)
				if a.u8(e+3) == 0xff {
					a.setU8(e+3, 0x48)
				}
			}
		}
	}
	if out != nil {
		switch {
		case stealth && a.u8(e+3) < 0x48:
			*out = int16(a.u8(e + 3))
		case !stealth && a.u8(e+1) < 0x48:
			*out = int16(a.u8(e + 1))
		default:
			*out = -1
		}
	}
	if stealth {
		return int16(a.u8(e + 2))
	}
	return int16(a.u8(e))
}
