package game

import "github.com/openorion2/openorion2/internal/save"

// The AI's defence of its colony stars (AIMOVE.C): Intercept_Invaders_ and
// what it calls. Spec: docs/turn/overview.md, Move_All_AI_.

// aiThreat is one 0x3f-byte record of Intercept_Invaders_: an own colony
// star that enemy ships (players, or monsters as bits 8..14) will reach.
type aiThreat struct {
	star   int32       // +0
	mask   uint16      // +4 owners of the threatening ships
	minETA uint8       // +6 the first turn any arrives
	str    [10]uint32  // +7 their strength against p by turn
	seen   [8][2]uint8 // +0x2f per player: flag, its eta
}

// interceptMissionOK are the AI missions Gather_Intercepting_Ships_ takes
// (jump table 0xd99b8).
var interceptMissionOK = [8]bool{true, false, true, true, false, true, false, true}

// interceptInvaders is Intercept_Invaders_ (0xd9f85).
func (g *Game) interceptInvaders(ma *moveAI) {
	s := &g.state
	a := g.ai
	p := ma.p
	var threats [72]aiThreat
	n := 0
	var idx [72]int8
	for i := range idx {
		idx[i] = -1
	}
	newThreat := func(st int) *aiThreat {
		if idx[st] == -1 {
			threats[n] = aiThreat{star: int32(st), minETA: 0xff}
			idx[st] = int8(n)
			n++
		}
		return &threats[idx[st]]
	}
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		if !maskBit(s.Stars[st].ColonyOwners, p) {
			continue
		}
		for q := int(s.NumPlayers) - 1; q >= 0; q-- {
			if q == p {
				continue
			}
			en := a.at(aiEnrouteShips) + st*0x50 + q*10
			if a.u8(en+9) == 0 {
				continue
			}
			if a.u8(en) == 0 && a.u8(a.at(aiEnrouteNonstealthyShips)+st*8+q) == 0 {
				continue
			}
			for e := 0; e < 10; e++ {
				if a.u8(en+e) == 0 {
					continue
				}
				t := newThreat(st)
				t.mask |= 1 << uint(q)
				if int32(t.minETA) > int32(e) {
					t.minETA = uint8(e)
				}
				for k := e; k < 10; k++ {
					t.str[k] += a.u32(a.at(aiEnrouteShipStrengths) + st*0xa00 + q*0x140 + p*0x28 + k*4)
				}
				t.seen[q] = [2]uint8{1, uint8(e)}
				break
			}
		}
		if idx[st] != -1 {
			t := &threats[idx[st]]
			for k := int(t.minETA); k < 10; k++ {
				if t.str[k] == 0 {
					t.str[k] = 1
				}
			}
		}
	}

	// Monsters: [star][owner-8] {eta, strength}; the eta stays 0.
	type monster struct {
		eta uint8
		str uint32
	}
	var mon [72][7]monster
	any := false
	rated := -1
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if sh.Owner < 8 || sh.Design.FTLSpeed == 0 {
			continue
		}
		var st int
		switch sh.Status {
		case 0:
			st = int(sh.Location)
		case 1:
			st = int(sh.Location) - 500
		case 2:
			st = int(sh.Location) - 1000
		default:
			continue
		}
		if !maskBit(s.Stars[st].ColonyOwners, p) {
			continue
		}
		m := &mon[st][int(sh.Owner)-8]
		rated = i
		if m.str += uint32(uint16(g.shipStrengthVsShips(i, p))); m.str == 0 {
			m.str = 1
		}
		if sh.Status == 0 {
			m.eta = 0
		} else if sh.ETA < m.eta {
			m.eta = sh.ETA
		}
		any = true
	}
	if any {
		for mo := 6; mo >= 0; mo-- {
			bit := uint16(1) << uint(mo+8)
			for st := int(s.NumStars) - 1; st >= 0; st-- {
				m := &mon[st][mo]
				if m.str == 0 {
					continue
				}
				t := newThreat(st)
				t.mask |= bit
				if m.eta < t.minETA {
					t.minETA = m.eta
				}
				for k := int(t.minETA); k < 10; k++ {
					t.str[k] += m.str
				}
			}
		}
	}
	if rated != -1 && !g.strategic() {
		g.interceptStack = g.shipStrengthStackLeftover(rated)
	}
	if n > 0 {
		few := int16(a.u16(a.at(aiPlayerInfo)+p*0x3a+6)) <= 3
		g.interceptions(ma, threats[:], &n, idx[:], few)
	}
}

// gatherInterceptingShips is Gather_Intercepting_Ships_ (0xd99d8): p's
// combat ships free for interception, or already bound for a threatened star.
func (g *Game) gatherInterceptingShips(ma *moveAI, idx []int8) []int16 {
	s := &g.state
	a := g.ai
	var out []int16
	for k := int(a.nAllShips) - 1; k >= 0; k-- {
		i := int16(a.u16(a.at(aiAllShips) + k*2))
		sh := &s.Ships[i]
		if int16(sh.Owner) != int16(ma.p) {
			continue
		}
		if s.canOrderShip(int(i)) {
			if sh.AIMission <= 7 && interceptMissionOK[sh.AIMission] {
				out = append(out, i)
			}
		} else if idx[shipDestinationStar(sh)] != -1 {
			out = append(out, i)
		}
	}
	return out
}

// knockOutImpossibleInterceptions is Knock_Out_Impossible_Interceptions_
// (0xd983f): threats p cannot match on any turn are dropped (the last kept
// when few) and their stars marked to flee.
func (g *Game) knockOutImpossibleInterceptions(ma *moveAI, threats []aiThreat, n *int, few bool) {
	s := &g.state
	a := g.ai
	p := ma.p
	for t := *n - 1; t >= 0; t-- {
		st := int(threats[t].star)
		ok := false
		for k := 0; !ok && k < 10; k++ {
			minPos := ^uint32(0)
			var sum uint32
			for q := int(s.NumPlayers) - 1; q >= 0; q-- {
				if threats[t].seen[q][0] == 0 {
					continue
				}
				minPos = min(minPos, a.u32(a.at(aiPossibleShipStrengths)+st*0xa00+p*0x140+q*0x28+k*4))
				sum += a.u32(a.at(aiEnrouteShipStrengths) + st*0xa00 + q*0x140 + p*0x28 + k*4)
			}
			if sum < minPos {
				ok = true
			}
		}
		if ok || few && *n <= 1 {
			continue
		}
		*n--
		copy(threats[t:], threats[t+1:*n+1])
		for e := 0; e < 10; e++ {
			ma.flee[st][e] = 1
		}
	}
}

// shipMinStrengthVersusPlayersAtStar is
// Ship_Min_Strength_Versus_Players_At_Star_ (0xd7f34): the ship's weakest
// strength against the owners in mask (players by table, monsters live).
func (g *Game) shipMinStrengthVersusPlayersAtStar(ship int, mask uint16, st int) int32 {
	s := &g.state
	a := g.ai
	nebula := s.Stars[st].InNebula != 0
	m := int32(0x7fff)
	if mask&0xff != 0 {
		blk := aiShipStrengths
		if nebula {
			blk = aiShipStrengthsInNebula
		}
		for q := int(s.NumPlayers) - 1; q >= 0; q-- {
			if mask>>uint(q)&1 == 0 {
				continue
			}
			if v := int32(a.u16(a.at(blk) + ship*16 + q*2)); v < int32(int16(m)) {
				m = v
			}
		}
	}
	if mask&0xff00 != 0 {
		for o := 8; o < 15; o++ {
			if mask>>uint(o)&1 == 0 {
				continue
			}
			var v uint32
			if nebula {
				v = g.shipStrengthVsShipsInNebula(ship, o)
			} else {
				v = g.shipStrengthVsShips(ship, o)
			}
			if int32(uint16(v)) < int32(int16(m)) {
				m = int32(uint16(v))
			}
		}
	}
	return m
}

// compareRatios is Compare_Ratios_ (0xfeb70): the sign of a/b - c/d by
// continued fractions, without overflow.
func compareRatios(a, b, c, d uint32) int32 {
	for {
		switch {
		case a == 0:
			if c > 0 {
				return -1
			}
			return 0
		case c == 0:
			return 1
		case a < b:
			if c >= d {
				return -1
			}
			q1, q2 := d/c, b/a
			if q2 > q1 {
				return -1
			}
			if q2 < q1 {
				return 1
			}
			a, b, c, d = d%c, c, b%a, a
		default:
			if d > c {
				return 1
			}
			q1, q2 := a/b, c/d
			if q2 > q1 {
				return -1
			}
			if q2 < q1 {
				return 1
			}
			a, c = a%b, c%d
		}
	}
}

// shipNTurnsToStarWithIntermediate is
// Ship_N_Turns_To_Star_With_Intermediate_ (0xd383f), memoised in
// _ship_n_turns_to_intermediate: a ship in transit may stop at a star on the
// way. out gets the stop, -1 for none.
func (g *Game) shipNTurnsToStarWithIntermediate(ship, st int, out *int16) int16 {
	s := &g.state
	a := g.ai
	sh := &s.Ships[ship]
	e := a.at(aiShipNTurnsToIntermediate) + ship*0x90 + st*2
	nav := testBit(sh.Design.Specials[:], 0xb)
	if a.u8(e+1) == 0xff {
		if sh.Location < 0x48 || sh.Location >= 1000 {
			here := int(sh.Location)
			if sh.Location >= 0x48 {
				here = g.xyToStarID(sh.X, sh.Y)
			}
			var via int16
			t := g.playerNTurnsFromStarToStarWithIntermediate(int(sh.Owner), int(int16(here)), st, &via, nav)
			a.setU8(e, uint8(t))
			a.setU8(e+1, uint8(via))
		} else {
			o := int(sh.Owner)
			dest := int(sh.Location) - 500
			for m := int(s.NumStars) - 1; a.u8(e) > 1 && m >= 0; m-- {
				if !a.inRange(m, o) && !(a.extRange(m, o) && nav) {
					continue
				}
				M := &s.Stars[m]
				if st != m && m != dest && (M.SpectralClass == save.SpectralBlackHole || M.Special == 9 ||
					!maskBit(M.ColonyOwners, o) && M.ColonyOwners != 0) {
					continue
				}
				if s.blackHoleBlocksPoints(int32(sh.X), int32(sh.Y), int32(M.X), int32(M.Y)) {
					continue
				}
				t := int32(a.u8(a.at(aiNTurnsFromTo)+m*0x240+st*8+o)) + int32(a.u8(a.at(aiShipNTurnsTo)+ship*0x48+m))
				if t < int32(a.u8(e)) {
					a.setU8(e, uint8(t))
					a.setU8(e+1, uint8(m))
				}
			}
			if a.u8(e+1) == 0xff {
				a.setU8(e+1, 0x48)
			}
		}
	}
	if out != nil {
		if v := a.u8(e + 1); v < 0x48 {
			*out = int16(v)
		} else {
			*out = -1
		}
	}
	return int16(a.u8(e))
}

// interceptions is Interceptions_ (0xd9a7e): each free ship goes to the
// threat it can reach soonest, ties to the least covered; threats left
// uncovered are dropped one by one (from the end) until all are covered.
func (g *Game) interceptions(ma *moveAI, threats []aiThreat, n *int, idx []int8, few bool) {
	s := &g.state
	a := g.ai
	p := ma.p
	ships := g.gatherInterceptingShips(ma, idx)
	g.knockOutImpossibleInterceptions(ma, threats, n, few)
	// Stack arrays of the original: set per threat star, kept across passes;
	// interceptAt starts as what the stack holds (interceptStack).
	var firstETA [72]int8
	interceptAt := &g.interceptStack
	for *n > 0 {
		var target, dest [500]int8
		var bestETA [500]uint8
		for i := range target {
			target[i], dest[i] = -1, -1
		}
		var cover [72][10]uint32
		var shipStr [72]int16
		for i := len(ships) - 1; i >= 0; i-- {
			sh := int(ships[i])
			nav := testBit(s.Ships[sh].Design.Specials[:], 0xb)
			bestETA[sh] = 10
			best := -1
			for t := *n - 1; t >= 0; t-- {
				T := &threats[t]
				st := int(T.star)
				shipStr[st] = int16(g.shipMinStrengthVersusPlayersAtStar(sh, T.mask, st))
				var via int16
				var eta int32
				if loc := s.Ships[sh].Location; loc < 0x48 {
					eta = int32(g.playerNTurnsFromStarToStarWithIntermediate(p, int(loc), st, &via, nav))
				} else {
					eta = int32(g.shipNTurnsToStarWithIntermediate(sh, st, &via))
				}
				if eta >= 10 {
					continue
				}
				better := false
				switch cur := int32(bestETA[sh]); {
				case cur > eta:
					better = true
				case cur == eta && best == -1:
					better = true
				case cur == eta:
					B := &threats[best]
					k := int(cur)
					for T.strAt(k) == 0 && B.strAt(k) == 0 {
						k++
					}
					switch {
					case T.strAt(k) == 0:
					case B.strAt(k) == 0:
						better = true
					default:
						better = compareRatios(cover[st][k], T.strAt(k), cover[B.star][k], B.strAt(k)) < 0
					}
				}
				if better {
					best = t
					bestETA[sh] = uint8(eta)
					target[sh] = int8(T.star)
					dest[sh] = int8(via)
				}
			}
			if best != -1 {
				st := threats[best].star
				for k := int(bestETA[sh]); k < 10; k++ {
					cover[st][k] += uint32(int32(shipStr[st]))
				}
			}
		}
		drop := false
		for t := *n - 1; t >= 0; t-- {
			T := &threats[t]
			st := T.star
			found := false
			firstETA[st] = 10
			for k := 0; firstETA[st] == 10 && k < 10; k++ {
				if T.str[k] == 0 && T.str[9] != 0 {
					continue
				}
				if T.str[k] > cover[st][k] {
					continue
				}
				if !found {
					interceptAt[st] = int8(k)
				}
				found = true
				if (T.str[k]+2)*4 < cover[st][k] {
					firstETA[st] = int8(k)
				}
			}
			if !found && !few {
				drop = true
				interceptAt[st] = -1
			}
		}
		if drop {
			*n--
			continue
		}
		for i := len(ships) - 1; i >= 0; i-- {
			id := ships[i]
			sh := &s.Ships[id]
			if dest[id] == -1 || !s.canOrderShip(int(id)) {
				continue
			}
			one := []int16{id, -1}
			res := g.shipsTryToMoveTo(one, int(dest[id]))
			ts := target[id]
			if int32(firstETA[ts]) < int32(uint8(res.turns)) {
				continue
			}
			here := g.xyToStarID(sh.X, sh.Y)
			sh.AIMission = 5
			to := int32(ts)
			if int32(interceptAt[ts]) > int32(uint8(res.turns)) {
				to = -1
				switch sh.Status {
				case 1:
					to = int32(dest[id])
				case 2:
					sh.Location = int16(here)
					sh.Status = 0
				}
			}
			if to == -1 {
				continue
			}
			g.makeShipsMoveTo(one, int(dest[id]), &res)
			a.setU16(a.at(aiAllCombatShipsIndex)+p*0x92+here*2, 0xffff)
		}
		return
	}
}

// strAt reads the strength column past its 10 entries the way the original
// does: into the per-player bytes that follow.
func (t *aiThreat) strAt(k int) uint32 {
	if k < 10 {
		return t.str[k]
	}
	var b [0x3f]byte
	for i, v := range t.str {
		b[7+4*i], b[8+4*i], b[9+4*i], b[10+4*i] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	}
	for q, v := range t.seen {
		b[0x2f+2*q], b[0x30+2*q] = v[0], v[1]
	}
	o := 7 + 4*k
	if o+4 > len(b) {
		panic("Interceptions_: the original reads past the threat record")
	}
	return uint32(b[o]) | uint32(b[o+1])<<8 | uint32(b[o+2])<<16 | uint32(b[o+3])<<24
}

// The original's stack below Intercept_Invaders_' frame (E2 is its esp at
// its calls; the same in every run of the turn).
const (
	interceptE2    = 0x3f13c4 // Intercept_Invaders_' esp at its calls
	origCodeDelta  = 0x224000 // runtime minus ELF address of the code
	origShipsBase  = 0x191ac  // *0x197f9c, the ships' DOS block
	interceptAtOff = 0x124    // Interceptions_' interceptAt (ebp-0x92) below E2
)

// shipStrengthStackLeftover is what Ship_Strength_Vs_Ships_(k) leaves where
// Interceptions_ keeps interceptAt, with tactical combat: Get_Ship_Combat_
// Bonuses_' copy of the design, its registers and Test_Bit_Field_'s pushal.
func (g *Game) shipStrengthStackLeftover(k int) [72]int8 {
	var b [72]byte
	put := func(i int, v uint32) {
		b[i], b[i+1], b[i+2], b[i+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	}
	d := &g.state.Ships[k].Design
	defense, evasion, _ := g.designCombatBonuses(d)
	if testBit(d.Specials[:], 0x26) {
		evasion -= 0x82 // added after the last Test_Bit_Field_
	}
	put(0, interceptE2-0x108)                  // pushal: ebp (Get_Design_Combat_Bonuses_')
	put(4, interceptE2-0x10c)                  // esp
	put(8, uint32(int32(evasion)))             // ebx
	put(12, 0x26)                              // edx: the bit tested
	put(16, uint32(int32(defense)))            // ecx
	put(20, interceptE2-0xd9)                  // eax: the design's specials
	put(24, 0x54e30+origCodeDelta)             // Test_Bit_Field_'s return
	put(28, interceptE2-0x64)                  // Get_Design_Combat_Bonuses_ pushes: ebp,
	put(32, origShipsBase+uint32(k)*0x81+0x63) // esi past rep movsb,
	put(36, interceptE2-0x38)                  // edx,
	put(40, 0)                                 // ecx,
	put(44, uint32(k)*0x81)                    // ebx
	put(48, 0x54ea8+origCodeDelta)             // its return
	copy(b[52:], designBytes(d))               // the design's first 20 bytes
	if int8(g.state.Ships[k].Owner) <= 9 {
		// Max_Ship_Hits_ then reaches deeper: Get_Ship_Structure_ copies the
		// design for Get_Design_Structure_, whose Test_Bit_Field_ (0x1b) pushes.
		put(16, interceptE2-0x75)                  // pushal: edi past rep movsb,
		put(20, origShipsBase+uint32(k)*0x81+0x63) // esi,
		put(24, interceptE2-0xf0)                  // ebp,
		put(28, interceptE2-0xf4)                  // esp,
		put(32, uint32(d.Size))                    // ebx (bx: the size),
		put(36, 0x1b)                              // edx,
		put(40, 0)                                 // ecx,
		put(44, interceptE2-0xc1)                  // eax: the specials
		put(48, 0x582d8+origCodeDelta)             // Test_Bit_Field_'s return
		put(52, interceptE2-0x20)                  // Get_Design_Structure_ pushes: ebp,
		put(56, origShipsBase+uint32(k)*0x81+0x63) // esi,
		put(60, uint32(k))                         // edx,
		put(64, 0)                                 // ecx,
		put(68, 0x64)                              // ebx
	}
	var out [72]int8
	for i, v := range b {
		out[i] = int8(v)
	}
	return out
}
