package game

import (
	"bytes"
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// Refits, defensive staging and surrenders decided while the AI data is
// built (AIDATA.C, AISTAGE.C, COLCALC.C). Spec: docs/turn/ai-data.md.

// refitDesirability is Refit_Desirability_ (0xd33d1): how far ship k's
// equipment lags its owner's best, 0 if built elsewhere or already current.
func (g *Game) refitDesirability(k int) uint32 {
	s := &g.state
	a := g.ai
	sh := &s.Ships[k]
	d := &sh.Design
	o := int16(sh.Owner)
	if int16(int8(d.Builder)) != o {
		return 0
	}
	t := a.at(aiTech) + int(o)*0x15
	v := uint32(0)
	if b := a.u8(t + aitShield); d.Shield < b {
		v = uint32(b-d.Shield) * 10
	}
	if b := a.u8(t + aitComputer); d.Computer < b {
		v += uint32(b-d.Computer) * 7
	}
	if b := a.u8(t + aitArmor); d.Armor < b {
		v += uint32(b-d.Armor) * 5
	}
	var beam, missile uint8
	for w := 7; w >= 0; w-- {
		wp := d.Weapons[w].Weapon
		if wp == 0 {
			continue
		}
		val := g.techValueOfWeapon(wp)
		switch g.wpnU8(int(wp), weaponType) {
		case 0:
			beam = max(beam, val)
		case 1, 2:
			missile = max(missile, val)
		}
	}
	if b := a.u8(t + aitBeam); beam > 0 && beam < b {
		v += uint32(b-beam) * 8
	}
	if b := a.u8(t + aitMissile); missile > 0 && missile < b {
		v += uint32(b-missile) * 6
	}
	if v != 0 && d.Size < 5 && bytes.Equal(designBytes(d), designBytes(&s.Players[o].Designs[d.Size])) {
		v = 0
	}
	return v
}

// designBytes is the 0x63-byte record of a design.
func designBytes(d *save.Design) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, d)
	return b.Bytes()
}

// computeDefensiveStagePoints is Compute_Defensive_Stage_Points_ (0xdbe84):
// p's staging stars by stage weight; without any, a colony near the enemy.
func (g *Game) computeDefensiveStagePoints(p int) {
	s := &g.state
	a := g.ai
	P := &s.Players[p]
	if P.Eliminated != 0 {
		return
	}
	ns := int(s.NumStars)
	var used [0x48]bool
	P.StagingStar = -1
	bit := uint8(1) << (uint(p) & 31)
	for st := ns - 1; st >= 0; st-- {
		s.Stars[st].AIStagePoint &^= bit
	}
	ssw := func(st int) int { return a.at(aiStarStageWeights) + st*32 + p*4 }
	bsw := func(st int) uint32 { return a.u32(a.at(aiBaseStarStageWeights) + st*32 + p*4) }
	count := 0
	for {
		best, bv := -1, uint32(0)
		for st := ns - 1; st >= 0; st-- {
			if !used[st] && bv < a.u32(ssw(st)) {
				best, bv = st, a.u32(ssw(st))
			}
		}
		if best == -1 {
			break
		}
		s.Stars[best].AIStagePoint |= bit
		if count == 0 {
			P.StagingStar = int8(best)
		}
		count++
		used[best] = true
		// Stars a turn away from a stage point need none of their own.
		for t := ns - 1; t >= 0; t-- {
			if t == best || a.u32(ssw(t)) == 0 || maskBit(s.Stars[t].AIStagePoint, p) {
				continue
			}
			if a.u8(a.at(aiNTurnsFromTo)+best*0x240+t*8+p) <= 1 {
				a.setU32(ssw(t), 0)
				used[t] = true
			}
		}
	}
	if count != 0 {
		return
	}
	others := ^(uint32(1) << (uint(p) & 31))
	best, bv := int16(-1), uint32(0)
	for rng := int32(P.FTLSpeed); ; rng += 3 {
		for st := ns - 1; st >= 0; st-- {
			if !maskBit(s.Stars[st].ColonyOwners, p) || uint32(a.u8(a.at(aiStarInRange)+st))&others == 0 {
				continue
			}
			for q := int(s.NumPlayers) - 1; q >= 0; q-- {
				if q == p || !a.inRange(st, q) || !s.starInRangeOfPlayer(st, q, int16(rng)) {
					continue
				}
				if bv < bsw(st) {
					best, bv = int16(st), bsw(st)
					break
				}
			}
		}
		if best != -1 || rng+3 >= 0x32 {
			break
		}
	}
	if best == -1 {
		fr := int32(P.FuelRange)
		for st := ns - 1; st >= 0; st-- {
			if !maskBit(s.Stars[st].ColonyOwners, p) {
				continue
			}
			for t := ns - 1; t >= 0; t-- {
				if uint32(s.Stars[t].ColonyOwners)&others == 0 || g.parsecsBetweenStars(st, t) > fr {
					continue
				}
				if bv < bsw(st) {
					best, bv = int16(st), bsw(st)
				}
			}
		}
	}
	if best == -1 {
		for st := ns - 1; st >= 0; st-- {
			if maskBit(s.Stars[st].ColonyOwners, p) && bv <= bsw(st) {
				best, bv = int16(st), bsw(st)
			}
		}
	}
	P.StagingStar = int8(best)
	// With no colony at all the original sets a bit in the memory before
	// the star array; the port drops that write.
	if best >= 0 {
		s.Stars[best].AIStagePoint |= bit
	}
}

// maySurrenderStar is May_Surrender_Star_ (0xe4a09): p may give star st to
// q unless it is p's home, already promised, out of q's range or p's last.
func (s *State) maySurrenderStar(st, p, q int) bool {
	if h := s.Players[p].HomePlanet; h != -1 && int(s.Planets[h].Star) == st {
		return false
	}
	S := &s.Stars[st]
	if int(S.SurrenderTo[q]) == p || S.SurrenderTo[p] != -1 {
		return false
	}
	if !s.starInNormalRangeOfPlayer(st, q) {
		return false
	}
	for t := int(s.NumStars) - 1; t >= 0; t-- {
		if maskBit(s.Stars[t].ColonyOwners, p) && t != st {
			return true
		}
	}
	return false
}

// surrenderStar is Surrender_Star_ (0xe4ab3): p surrenders st to q, and so
// does everyone who had surrendered it to p; q's own pledge passes on.
func (s *State) surrenderStar(st, p, q int) {
	S := &s.Stars[st]
	S.SurrenderTo[p] = int8(q)
	for r := int(s.NumPlayers) - 1; r >= 0; r-- {
		if int16(S.SurrenderTo[r]) == int16(p) {
			S.SurrenderTo[r] = int8(q)
		}
	}
	if S.SurrenderTo[q] == -1 {
		return
	}
	for r := int(s.NumPlayers) - 1; r >= 0; r-- {
		if d := S.SurrenderTo[r]; int16(d) == int16(q) {
			S.SurrenderTo[r] = S.SurrenderTo[d]
		}
	}
}
