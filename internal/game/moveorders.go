package game

import (
	"bytes"
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// Move orders (SHIPMOVE.C) and the map geometry they use (MAP.C, AIMOVE.C).
// Spec: docs/screens/fleet-box.md, "Move orders" and "Can_Order_Ship_".
// These routines write save state and are shared with the AI, so the
// arithmetic follows the binary: int32 wraps, i16 truncations, unsigned
// compares where the original uses them.

// shipMoveInfo is the 14-byte _g_ship_move_info filled by
// Ships_Try_To_Move_To_ (0xff799).
type shipMoveInfo struct {
	blocked      bool  // +0: a black hole blocks the way
	noFuel       bool  // +1: never written
	noFTL        bool  // +2: the owner has no FTL drive
	unused3      uint8 // +3: never written
	turns        int8  // +4: ETA in turns, -1 when the order is refused
	canMove      bool  // +5
	outOfRange   bool  // +6
	navigator    bool  // +7: a navigator officer helps
	speed        int8  // +8: parsecs per turn, -1 through a gate or wormhole
	parsecsShort uint8 // +9: parsecs from the closest own colony when out of range
	wormhole     bool  // +0xa
	jumpgate     bool  // +0xb
	stargate     bool  // +0xc
	flux         bool  // +0xd: hyperspace flux forbids movement
}

// starRecord is &_star[i]. Callers may name NUM_STARS (Antares) and
// XY_To_Star_Id_ scans that far; with 72 stars the original then reads past
// its 0x1fc8-byte allocation, which the port replaces by a zero record.
func (s *State) starRecord(i int) *save.Star {
	if i >= 0 && i < len(s.Stars) {
		return &s.Stars[i]
	}
	return &save.Star{}
}

// starByte is the byte at offset off of a star record.
func starByte(st *save.Star, off int) byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, st)
	if off < b.Len() {
		return b.Bytes()[off]
	}
	return 0
}

// blackHoleBit is Test_Bit_Field_(&star.black_hole_blocked, bit). Bit 72
// (Antares) reads the byte after the 9-byte field, star.special.
func blackHoleBit(st *save.Star, bit int) bool {
	i := bit >> 3
	var b byte
	if i < len(st.BlackHoleBlocked) {
		b = st.BlackHoleBlocked[i]
	} else {
		b = starByte(st, 0x1f+i)
	}
	return b>>(bit&7)&1 != 0
}

// maskBit is `(mask >> p) & 1` with x86 shift-count masking.
func maskBit(mask uint8, p int) bool { return uint32(mask)>>(uint(p)&31)&1 != 0 }

// isqrt is isqrt_ (0x134c92): floor of the square root by binary search.
func isqrt(n uint32) uint32 {
	if n >= 0xfffe0001 {
		return 0xffff
	}
	lo, hi := int32(0), int32(0x100)
	if n > 0xffff {
		lo, hi = 0x100, 0xffff
	}
	for {
		mid := (lo + hi) / 2
		if uint32(mid)*uint32(mid) <= n {
			lo = mid
		} else {
			hi = mid
		}
		if lo+1 >= hi {
			return uint32(lo)
		}
	}
}

// parsecsBetweenPoints is Parsecs_Between_Points_ (0xebe79): distance in
// parsecs (30 units), rounded up.
func parsecsBetweenPoints(x1, y1, x2, y2 int16) int32 {
	dx, dy := int32(x1)-int32(x2), int32(y1)-int32(y2)
	d := uint32(dx*dx + dy*dy)
	r := isqrt(d / 900)
	for r*r*900 < d {
		r++
	}
	return int32(r)
}

// parsecsBetweenStars is Parsecs_Between_Stars_ (0xebeb7).
func (g *Game) parsecsBetweenStars(a, b int) int32 {
	A, B := g.state.starRecord(a), g.state.starRecord(b)
	return parsecsBetweenPoints(A.X, A.Y, B.X, B.Y)
}

// pointsOnSameSideOfLine is Points_On_Same_Side_Of_Line_ (0xeb4dd).
func pointsOnSameSideOfLine(ax, ay, bx, by, l1x, l1y, l2x, l2y int32) int32 {
	c1 := (l2x-l1x)*(ay-l1y) - (l2y-l1y)*(ax-l1x)
	if c1 == 0 {
		return 0
	}
	c2 := (l2x-l1x)*(by-l1y) - (l2y-l1y)*(bx-l1x)
	if c1 > 0 {
		return c2
	}
	return -c2
}

// pointWithinDOfSeg is Point_Within_D_Of_Seg_ (0xeb531): whether (px, py)
// lies within d of the segment; long segments are halved until the
// products fit in 32 bits.
func pointWithinDOfSeg(px, py, d, x1, y1, x2, y2 int16) bool {
	var a, b, s int32
	var d2 uint32
	var within bool
	for {
		a = int32(y1) - int32(y2)
		b = int32(x2) - int32(x1)
		s = a*int32(px) + b*int32(py) - (a*int32(x2) + b*int32(y2))
		as := uint32(s)
		if s < 0 {
			as = uint32(-s)
		}
		l2 := uint32(a*a + b*b)
		D := int32(d)
		d2 = uint32(D * D)
		if 0xffffffff/d2 >= l2 {
			within = as <= 0xffff && as*as <= d2*l2
			break
		}
		if as <= 0xffff {
			within = true
			break
		}
		r := isqrt(l2)
		if as <= uint32(D)*r {
			within = true
			break
		}
		if (r+1)*uint32(D) < as {
			break
		}
		px, py, d = px/2, py/2, int16(D/2)
		x1, y1, x2, y2 = x1/2, y1/2, x2/2, y2/2
	}
	if !within {
		return false
	}
	if s == 0 {
		switch {
		case x1 < x2:
			return x1 <= px && px <= x2
		case x2 < x1:
			return x2 <= px && px <= x1
		case y1 <= y2:
			return y1 <= py && py <= y2
		}
		return y2 <= py && py <= y1
	}
	nx, ny := a, b
	for uint32(nx*nx+ny*ny) <= d2 {
		nx += a
		ny += b
	}
	X1, Y1, X2, Y2 := int32(x1), int32(y1), int32(x2), int32(y2)
	qx, qy := int32(px)+nx, int32(py)+ny
	if pointsOnSameSideOfLine(qx, qy, int32(px), int32(py), X1, Y1, X2, Y2) > 0 {
		// The original calls the test again here and ignores the result.
		qx, qy = int32(px)-nx, int32(py)-ny
	}
	return pointsOnSameSideOfLine(X1, Y1, X2, Y2, qx, qy, int32(px), int32(py)) <= 0
}

// collectBlackHoles is Collect_Black_Holes_ (0xeb7c5).
func (s *State) collectBlackHoles() []int {
	var r []int
	for i := 0; i < int(s.NumStars); i++ {
		if s.Stars[i].SpectralClass == save.SpectralBlackHole {
			r = append(r, i)
		}
	}
	return r
}

// blackHoleBlocksPoints is Black_Hole_Blocks_Points_ (0xeb7fd).
func (s *State) blackHoleBlocksPoints(x1, y1, x2, y2 int32) bool {
	if x1 == x2 && y1 == y2 {
		return false
	}
	bh := s.collectBlackHoles()
	for i := len(bh) - 1; i >= 0; i-- {
		h := &s.Stars[bh[i]]
		if pointWithinDOfSeg(h.X, h.Y, 45, int16(x1), int16(y1), int16(x2), int16(y2)) {
			return true
		}
	}
	return false
}

// pointIsInNebulaN is Point_Is_In_Nebula_N_ (0xeb9c8). The test reads the
// zoom-3 nebula bitmap as loaded from STARBG.LBX at byte 0x10 + v*w + u,
// four bytes before the frame the drawing code uses.
func (g *Game) pointIsInNebulaN(x, y int16, n int) bool {
	s := &g.state
	nb := &s.Nebulas[n]
	u := (int32(x) - int32(nb.X)) / 3
	if u < 0 {
		return false
	}
	v := (int32(y) - int32(nb.Y)) / 3
	if v < 0 {
		return false
	}
	b := g.nebulaBitmap(n)
	if b == nil {
		return false
	}
	w, h := int32(int16(binary.LittleEndian.Uint16(b))), int32(int16(binary.LittleEndian.Uint16(b[2:])))
	if u >= w || v >= h {
		return false
	}
	i := 0x10 + int(w*v+u)
	return i < len(b) && b[i] > 5
}

// nebulaBitmap is _nebula_pict_seg[n][3]: STARBG.LBX entry 6 + 4*type + 3,
// as Load_Nebula_Pictures_ loads it.
func (g *Game) nebulaBitmap(n int) []byte {
	t := int(g.state.Nebulas[n].Picture)
	if t > 11 {
		g.logOnce("Point_Is_In_Nebula_N_: nebula type out of range")
		return nil
	}
	a, err := g.archive("STARBG.LBX")
	if err != nil {
		g.logOnce("Point_Is_In_Nebula_N_: " + err.Error())
		return nil
	}
	e, err := a.Entry(6 + 4*t + 3)
	if err != nil || len(e) < 0x10 {
		g.logOnce("Point_Is_In_Nebula_N_: bad STARBG.LBX entry")
		return nil
	}
	return e
}

// pointIsInSomeNebula is Point_Is_In_Some_Nebula_ (0xeba3a).
func (g *Game) pointIsInSomeNebula(x, y int16) bool {
	for n := 0; n < int(g.state.NumNebulas) && n < len(g.state.Nebulas); n++ {
		if g.pointIsInNebulaN(x, y, n) {
			return true
		}
	}
	return false
}

// pointIsInWarpFieldInterdictorOfStar is
// Point_Is_In_Warp_Field_Interdictor_Of_Star_ (0xebac3).
func pointIsInWarpFieldInterdictorOfStar(x, y int16, p int, st *save.Star) bool {
	if st.WarpInterdictor&^(byte(1)<<(uint(p)&31)) == 0 {
		return false
	}
	dx, dy := int32(x)-int32(st.X), int32(y)-int32(st.Y)
	return dx*dx+dy*dy <= 0x1fa4
}

// movePlayer1TurnToStar is Move_Player_1_Turn_To_Star_ (0xebb0c): one turn
// of travel from (x0, y0) toward star dest. A nil neb ignores nebulas.
func (g *Game) movePlayer1TurnToStar(p int, x0, y0 int16, dest int, speed int16, x, y *int16, neb, stop *bool) {
	D := g.state.starRecord(dest)
	tx, ty := int32(D.X), int32(D.Y)
	dx, dy := tx-int32(x0), ty-int32(y0)
	d2 := dx*dx + dy*dy
	l := int32(isqrt(uint32(d2)))
	if speed == 0 {
		speed = 1
	}
	arrived, inNeb := false, false
	*stop = false
	for k := int32(1); ; k++ {
		if (neb != nil && inNeb) || *stop || arrived || int16(k) > speed {
			break
		}
		k16 := int32(int16(k))
		if k16*900*k16 >= d2 {
			*x, *y = int16(tx), int16(ty)
			arrived = true
		} else {
			if dx < 0 {
				*x = int16(int32(x0) - (k16*30*-dx+l-1)/l)
			} else {
				*x = int16(int32(x0) + (k16*30*dx+l-1)/l)
			}
			if dy < 0 {
				*y = int16(int32(y0) - (k16*30*-dy+l-1)/l)
			} else {
				*y = int16(int32(y0) + (k16*30*dy+l-1)/l)
			}
		}
		if neb != nil && g.pointIsInSomeNebula(*x, *y) {
			inNeb = true
		} else if pointIsInWarpFieldInterdictorOfStar(*x, *y, p, D) {
			*stop = true
		}
	}
	if inNeb {
		*neb = true
	}
}

// starInRangeOfPlayerAux1 is Star_In_Range_Of_Player_Aux1_ (0xff4e9):
// whether star j is within rng parsecs of a colony star of p whose line to
// j is not blocked by a black hole.
func (s *State) starInRangeOfPlayerAux1(j, p int, rng int16) bool {
	S := s.starRecord(j)
	lim := int32(rng) * 900 * int32(rng)
	for k := int(s.NumStars) - 1; k >= 0; k-- {
		K := &s.Stars[k]
		if !maskBit(K.ColonyOwners, p) || blackHoleBit(K, int(int16(j))) {
			continue
		}
		dx, dy := int32(S.X)-int32(K.X), int32(S.Y)-int32(K.Y)
		if dx*dx+dy*dy <= lim {
			return true
		}
	}
	return false
}

// starInRangeOfPlayerAux2 is Star_In_Range_Of_Player_Aux2_ (0xff593).
// It records what its frame leaves on the stack (rangeResidue).
func (s *State) starInRangeOfPlayerAux2(j, p int, rng int16) bool {
	r := &s.rangeResidue
	r.player = int32(int16(p))
	S := s.starRecord(j)
	if maskBit(S.ColonyOwners, p) {
		return true
	}
	r.rng, r.aux1Ret, r.aux1ECX = int32(rng), 0xff5c5, uint32(uint8(p))
	if s.starInRangeOfPlayerAux1(j, p, rng) {
		return true
	}
	w := int(S.WormholeTo)
	if w == -1 || !maskBit(s.starRecord(w).Visited, p) {
		return false
	}
	r.aux1Ret = 0xff5f2
	return s.starInRangeOfPlayerAux1(w, p, rng)
}

// rangeResidue is the stack Star_In_Range_Of_Player_Aux2_ and _Aux1_ leave
// behind; Process_Retreating_Ships_ sends part of it in its messages.
type rangeResidue struct {
	player, rng int32  // Aux2's locals
	aux1Ret     uint32 // return address of the last Aux1 call
	aux1ECX     uint32 // ecx pushed by Aux1
}

// starInRangeOfPlayer is Star_In_Range_Of_Player_ (0xff5f8); allies'
// colonies count too.
func (s *State) starInRangeOfPlayer(j, p int, rng int16) bool {
	if s.starInRangeOfPlayerAux2(j, p, rng) {
		return true
	}
	for q := int(s.NumPlayers) - 1; q >= 0; q-- {
		if q != p && s.Players[p].TreatyStatus[q] == 2 && s.starInRangeOfPlayerAux2(j, q, rng) {
			return true
		}
	}
	return false
}

// closestOtherColonyStar is Closest_Other_Colony_Star_ (0xe6bcd): the star
// of p's colony nearest to dest with a clear line, -1 if none.
func (s *State) closestOtherColonyStar(p, dest int) int {
	D := s.starRecord(dest)
	best, res := uint32(0xffffffff), -1
	for c := int(s.NumColonies) - 1; c >= 0; c-- {
		col := &s.Colonies[c]
		if int(col.Owner) != p {
			continue
		}
		t := int(s.Planets[col.Planet].Star)
		if int16(t) == int16(dest) || t == res || blackHoleBit(D, t) {
			continue
		}
		T := s.starRecord(t)
		dy, dx := int32(T.Y)-int32(D.Y), int32(T.X)-int32(D.X)
		if d := uint32(dy*dy + dx*dx); d < best {
			best, res = d, t
		}
	}
	return res
}

// officerBaseLevel is Get_Officer_Base_Level_ (0x93d4b); level 5 needs
// the Warlord pick.
func officerBaseLevel(xp int16, warlord bool) int16 {
	switch {
	case xp < 60:
		return 0
	case xp < 150:
		return 1
	case xp < 300:
		return 2
	case xp < 500:
		return 3
	case xp >= 1000 && warlord:
		return 5
	}
	return 4
}

// ownedOfficerLevel is Owned_Officer_Level_ (0x94951). Leader 0x42 never
// gets the Warlord bonus. An owner outside 0..7 (the original reads
// another player record) counts as no Warlord.
func (s *State) ownedOfficerLevel(o int) int16 {
	l := &s.Leaders[o]
	warlord := false
	if o != 0x42 && l.Owner >= 0 && int(l.Owner) < len(s.Players) {
		warlord = s.Players[l.Owner].Picks[save.PickWarlord] != 0
	}
	return min(officerBaseLevel(l.Experience, warlord), 5)
}

// shipRange is Ship_Range_ (0xff496): the owner's fuel range, half again
// with special 11 fitted (damage ignored); 0xff for non-players.
func (s *State) shipRange(ship int) int16 {
	sh := &s.Ships[ship]
	if sh.Owner >= 8 {
		return 0xff
	}
	r := s.Players[sh.Owner].FuelRange
	if testBit(sh.Design.Specials[:], 11) {
		r = int16((int32(r)*3 + 1) / 2)
	}
	return r
}

// shipsTryToMoveTo is Ships_Try_To_Move_To_ (0xff799): whether the listed
// ships may move to star dest (NUM_STARS = Antares) and how. ids is read up
// to its end or a -1; it must not be empty and owners must be >= 0.
func (g *Game) shipsTryToMoveTo(ids []int16, dest int) shipMoveInfo {
	s := &g.state
	var info shipMoveInfo
	D := s.starRecord(dest)
	owner := int(s.Ships[ids[0]].Owner)
	rng := int32(30000)
	speed := int32(1)
	if owner < 8 {
		speed = int32(s.Players[owner].FTLSpeed)
	}
	sp := int32(-1)
	nav := int32(0)
	if int16(dest) != s.NumStars && owner < 8 && s.Players[owner].Picks[save.PickTransDimensional] == 0 && g.hyperspaceFlux() {
		info.flux = true
		return info
	}
	var x, y int32
	first := true
	for _, id := range ids {
		if id == -1 {
			break
		}
		sh := &s.Ships[id]
		r := s.shipRange(int(id))
		if o := int(sh.Officer); o != -1 && s.Leaders[o].ETA == 0 {
			v := int32(0)
			if b := byte(s.Leaders[o].SpecialSkills >> 8); b&2 != 0 {
				v = int32(s.ownedOfficerLevel(o))/3 + 1
			} else if b&1 != 0 {
				v = int32(s.ownedOfficerLevel(o))/4 + 1
			}
			if int32(int16(v)) > nav {
				nav = int32(int16(v))
			}
		}
		if first {
			x, y = int32(sh.X), int32(sh.Y)
			first = false
		}
		if int32(r) < rng {
			rng = int32(r)
		}
	}
	here := g.xyToStarID(int16(x), int16(y))
	if int16(dest) == s.NumStars && here != -1 && maskBit(s.starRecord(here).DimensionalPortal, owner) {
		info.turns = 1
		info.speed = -1
		info.canMove = true
		return info
	}
	if owner < 8 && !s.starInRangeOfPlayer(dest, owner, int16(rng)) {
		if c := s.closestOtherColonyStar(owner, dest); c != -1 {
			info.parsecsShort = uint8(g.parsecsBetweenStars(c, dest))
		}
		info.outOfRange = true
	}
	if owner < 8 {
		if here < 0 {
			if s.blackHoleBlocksPoints(x, y, int32(D.X), int32(D.Y)) {
				info.blocked = true
			}
		} else {
			H := s.starRecord(here)
			info.wormhole = int16(H.WormholeTo) == int16(dest)
			info.stargate = maskBit(H.Stargate, owner) && maskBit(D.Stargate, owner)
			info.jumpgate = maskBit(H.Jumpgate, owner) && maskBit(D.Jumpgate, owner)
			if blackHoleBit(H, int(int16(dest))) && nav == 0 && !info.wormhole && !info.stargate && !info.jumpgate {
				info.blocked = true
			}
		}
	}
	switch {
	case info.stargate || info.wormhole:
		info.turns = 1
		if int16(dest) == int16(here) {
			info.turns = 0
		}
	case speed <= 0:
		info.noFTL = true
	default:
		cx, cy := int16(x), int16(y)
		sp = speed + nav
		if info.jumpgate {
			sp += 3
		}
		turns := 0
		for !(cx == D.X && cy == D.Y) {
			turns++
			var neb, stop bool
			nebp := &neb
			if nav != 0 {
				nebp = nil
			}
			g.movePlayer1TurnToStar(int(int16(owner)), cx, cy, dest, int16(sp), &cx, &cy, nebp, &stop)
		}
		info.turns = int8(turns)
	}
	info.navigator = nav > 0
	if (info.blocked && !info.jumpgate && !info.stargate && !info.navigator) || info.outOfRange ||
		(speed == 0 && (int32(D.X) != x || int32(D.Y) != y)) {
		info.turns = -1
		info.canMove = false
		return info
	}
	if info.stargate || info.wormhole {
		info.speed = -1
		info.canMove = true
		return info
	}
	info.canMove = true
	info.speed = int8(sp)
	return info
}

// makeShipsMoveTo is Make_Ships_Move_To_ (0xffd08) with its patch stub at
// 0x16915c: the order is applied without checking info.canMove; ids as in
// shipsTryToMoveTo.
func (g *Game) makeShipsMoveTo(ids []int16, dest int, info *shipMoveInfo) {
	s := &g.state
	if len(ids) == 0 || ids[0] == -1 {
		return
	}
	loc, st := int16(dest), uint8(save.ShipAtStar)
	if info.turns != 0 {
		if l := s.Ships[ids[0]].Location; l < 0x48 || l >= 1000 {
			loc, st = int16(dest+save.LocationDeparted), save.ShipDeparting
		} else {
			loc, st = int16(dest+save.LocationTransit), save.ShipInTransit
		}
	}
	col := false
	nav := uint8(0)
	if info.navigator {
		nav = 1
	}
	for _, id := range ids {
		if id == -1 {
			break
		}
		sh := &s.Ships[id]
		if t := sh.Design.Type; t == 1 || t == 4 {
			col = true
		}
		sh.Location = loc
		sh.ETA = uint8(info.turns)
		sh.Navigator = nav
		sh.Speed = info.speed
		if sh.Status != save.ShipFree {
			sh.Status = st
		}
	}
	if col {
		s.clearAllShipsAutoColonizationFlag(ids)
	}
}

// clearAllShipsAutoColonizationFlag is
// Clear_All_Ships_Auto_Colonization_Flag_ (0xa1c0d).
func (s *State) clearAllShipsAutoColonizationFlag(ids []int16) {
	for _, id := range ids {
		if id == -1 {
			break
		}
		d := &s.Ships[id].Design
		if d.Type == 1 || d.Type == 4 {
			d.Weapons[0].Weapon = -1
			d.Weapons[1].Weapon = 0
		}
	}
}

// canOrderShip is Can_Order_Ship_ (0xff6be). Ships in transit take orders
// only with Hyperspace Communications, or Sub-Space/Tachyon Communications
// and a base of that kind in range.
func (s *State) canOrderShip(ship int) bool {
	sh := &s.Ships[ship]
	if int16(sh.Owner) >= s.NumPlayers {
		return false
	}
	switch sh.Status {
	case save.ShipAtStar, save.ShipDeparting:
		return true
	case save.ShipInTransit:
	default:
		return false
	}
	if sh.Owner < 0 {
		return false // the original reads before the player array
	}
	p := &s.Players[sh.Owner]
	if p.AppStatus[0x172-0x117] == 3 {
		return true
	}
	if p.AppStatus[0x1cb-0x117] != 3 && p.AppStatus[0x1c7-0x117] != 3 {
		return false
	}
	mask := byte(1) << (uint(sh.Owner) & 31)
	for k := int(s.NumStars) - 1; k >= 0; k-- {
		st := &s.Stars[k]
		d := parsecsBetweenPoints(st.X, st.Y, sh.X, sh.Y)
		if d > 5 {
			continue
		}
		if st.BaseSubspace&mask != 0 || d <= 2 && st.BaseTachyon&mask != 0 {
			return true
		}
	}
	return false
}

// playerCanOrderShip is Player_Can_Order_Ship_ (0x7843d).
func (g *Game) playerCanOrderShip(p, ship int) bool {
	return int16(g.state.Ships[ship].Owner) == int16(p) && g.state.canOrderShip(ship)
}

// shipCanBeSelected is Ship_Can_Be_Selected_ (0x8a16a).
func (g *Game) shipCanBeSelected(ship int) bool {
	return g.playerCanOrderShip(int(g.state.PlayerNum), ship)
}
