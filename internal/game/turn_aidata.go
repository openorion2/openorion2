package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// The AI's per-turn picture (AIDATA.C): Compute_AI_Data_ and the blocks it
// allocates. Spec: docs/turn/ai-data.md.

// The blocks of Compute_AI_Data_, in allocation order.
const (
	aiShipStrengths            = iota // 0x1aa1dc
	aiShipStrengthsInNebula           // 0x1aa1f4
	aiShipBombStrengths               // 0x1aa1fc
	aiPlanetStageWeights              // 0x1aa1bc
	aiBaseStarStageWeights            // 0x1aa210
	aiStarStageWeights                // 0x1aa1e0
	aiNTurnsFromTo                    // 0x1aa1e8
	aiNTurnsFromToIntermediate        // 0x1aa1b8
	aiShipNTurnsTo                    // 0x1aa1b4
	aiShipNTurnsToIntermediate        // 0x1aa1c0
	aiPossibleShipStrengths           // 0x1aa208
	aiColonyInfo                      // 0x1aa1ec
	aiPlayerInfo                      // 0x1aa204
	aiStarInfo                        // 0x1aa1f8
	aiStarInRange                     // 0x1aa214
	aiStarInExtendedRange             // 0x1aa200
	aiEnrouteShipStrengths            // 0x1aa1f0
	aiEnrouteShips                    // 0x1aa1e4
	aiEnrouteHumanShips               // 0x1aa1d8
	aiEnrouteNonstealthyShips         // 0x1aa20c
	aiAllShips                        // 0x1aa1d4
	aiAllCombatShipsIndex             // 0x1aa1c8
	aiTech                            // 0x1aa1d0
	aiRefit                           // 0x1aa1c4
	aiShipRefitDesirabilities         // 0x1aa1cc
	aiNumBlocks
)

// aiBlocks gives each block's pointer global and size in bytes.
var aiBlocks = [aiNumBlocks]struct {
	global uint32
	size   int
}{
	{0x1aa1dc, 0x1f40},
	{0x1aa1f4, 0x1f40},
	{0x1aa1fc, 0xfa0},
	{0x1aa1bc, 0x1680},
	{0x1aa210, 0x900},
	{0x1aa1e0, 0x900},
	{0x1aa1e8, 0xa200},
	{0x1aa1b8, 0x28800},
	{0x1aa1b4, 0x8ca0},
	{0x1aa1c0, 0x11940},
	{0x1aa208, 0x2d000},
	{0x1aa1ec, 0x6d6},
	{0x1aa204, 0x1d0},
	{0x1aa1f8, 0xdc8},
	{0x1aa214, 0x48},
	{0x1aa200, 0x48},
	{0x1aa1f0, 0x2d000},
	{0x1aa1e4, 0x1680},
	{0x1aa1d8, 0x2d0},
	{0x1aa20c, 0x240},
	{0x1aa1d4, 0x3e8},
	{0x1aa1c8, 0x490},
	{0x1aa1d0, 0xa8},
	{0x1aa1c4, 0x5dd0},
	{0x1aa1cc, 0x7d0},
}

// aiBlockOff holds each block's data offset in the arena; the sizes are
// fixed, so the layout is too.
var aiBlockOff = func() (o [aiNumBlocks]int) {
	at := 0
	for i, b := range aiBlocks {
		o[i] = at + aiBlockHeader
		at += aiBlockHeader + (b.size+3)&^3
	}
	return
}()

// aiBlockOfGlobal returns the block whose pointer lives at global addr, or -1.
func aiBlockOfGlobal(addr uint32) int {
	for i, b := range aiBlocks {
		if b.global == addr {
			return i
		}
	}
	return -1
}

// at is the arena offset of block b's data.
func (a *aiArena) at(b int) int { return aiBlockOff[b] }

// Raw arena access. Out-of-block indices land in the neighbouring blocks as
// in the original; beyond the arena itself reads give 0 and writes are lost.
func (a *aiArena) u8(o int) uint8 {
	if o < 0 || o >= len(a.mem) {
		return 0
	}
	return a.mem[o]
}

func (a *aiArena) setU8(o int, v uint8) {
	if o >= 0 && o < len(a.mem) {
		a.mem[o] = v
	}
}

func (a *aiArena) u16(o int) uint16 { return uint16(a.u8(o)) | uint16(a.u8(o+1))<<8 }

func (a *aiArena) setU16(o int, v uint16) { a.setU8(o, uint8(v)); a.setU8(o+1, uint8(v>>8)) }

func (a *aiArena) u32(o int) uint32 { return uint32(a.u16(o)) | uint32(a.u16(o+2))<<16 }

func (a *aiArena) setU32(o int, v uint32) { a.setU16(o, uint16(v)); a.setU16(o+2, uint16(v>>16)) }

func (a *aiArena) fill(b int, v byte) {
	m := a.mem[a.at(b) : a.at(b)+aiBlocks[b].size]
	for i := range m {
		m[i] = v
	}
}

// newAIArena is Push_Block_ plus the 25 Allocate_Next_Block_ calls.
func newAIArena() *aiArena {
	a := &aiArena{}
	for _, b := range aiBlocks {
		a.alloc(b.size)
	}
	return a
}

// staleRanges lists the arena bytes Compute_AI_Data_ leaves as the memory
// was: _all_ships past _n_all_ships and the padding after _ai_colony_info.
func (a *aiArena) staleRanges() [][2]int {
	s := a.at(aiAllShips)
	c := a.at(aiColonyInfo) + aiBlocks[aiColonyInfo].size
	return [][2]int{{s + 2*int(a.nAllShips), s + aiBlocks[aiAllShips].size}, {c, c + 2}}
}

// computeAIData is Compute_AI_Data_ (0xd3d34).
func (g *Game) computeAIData() {
	s := &g.state
	a := newAIArena()
	g.ai = a
	np, ns := int(s.NumPlayers), int(s.NumStars)

	a.fill(aiPlayerInfo, 0)
	a.fill(aiShipStrengths, 0)
	a.fill(aiShipStrengthsInNebula, 0)
	a.fill(aiShipBombStrengths, 0)
	a.fill(aiEnrouteShipStrengths, 0)
	a.fill(aiEnrouteShips, 0)
	a.fill(aiEnrouteHumanShips, 0)
	a.fill(aiEnrouteNonstealthyShips, 0)
	g.aiShipStrengthsAndEnroute()

	for p := np - 1; p >= 0; p-- {
		s.Players[p].FleetStrength = [8]int32{}
	}
	for k := int(s.NumShips) - 1; k >= 0; k-- {
		o := int(s.Ships[k].Owner)
		if o >= 8 || o < 0 {
			continue // a negative owner only ever adds zero strengths
		}
		for q := np - 1; q >= 0; q-- {
			s.Players[o].FleetStrength[q] += int32(a.u16(a.at(aiShipStrengths) + k*16 + q*2))
		}
	}

	// Within e turns: strengths and arrivals accumulate over the ETA.
	for st := ns - 1; st >= 0; st-- {
		for e := 0; e < 9; e++ {
			for o := np - 1; o >= 0; o-- {
				for q := np - 1; q >= 0; q-- {
					x := a.at(aiEnrouteShipStrengths) + st*0xa00 + o*0x140 + q*0x28 + e*4
					a.setU32(x+4, a.u32(x+4)+a.u32(x))
					if y := a.at(aiEnrouteShips) + st*0x50 + o*10 + e; a.u8(y) != 0 {
						a.setU8(y+1, 1)
					}
				}
			}
			if y := a.at(aiEnrouteHumanShips) + st*10 + e; a.u8(y) != 0 {
				a.setU8(y+1, 1)
			}
		}
	}

	g.aiStarRanges()
	g.aiTurnTables()
	g.aiColonyInfo()
	g.aiStarInfo()
	g.computeStarPlayerRangeInfo()
	g.aiPlayerInfo()
	g.computeBasePlanetValues()
	g.computeContextualPlanetValues()
	g.computeAITechInfo()
	g.aiRefitTables()
	g.aiPossibleShipStrengths()
	g.aiStageWeights()
	for p := np - 1; p >= 0; p-- {
		g.computeDefensiveStagePoints(p)
	}

	for p := np - 1; p >= 0; p-- {
		for st := ns - 1; st >= 0; st-- {
			S := &s.Stars[st]
			if S.SurrenderTo[p] < 8 {
				continue
			}
			q := int(S.SurrenderTo[p]) - 8
			S.SurrenderTo[p] = -1
			if s.Players[p].Eliminated == 0 && s.Players[q].Eliminated == 0 && s.maySurrenderStar(st, p, q) {
				s.surrenderStar(st, p, q)
			}
		}
	}
	for st := ns - 1; st >= 0; st-- {
		s.Stars[st].SpecialPopup = 0
	}
}

// aiShipStrengthsAndEnroute is the first ship loop of Compute_AI_Data_: each
// ship's strength against each other player, and where it is or will be.
func (g *Game) aiShipStrengthsAndEnroute() {
	s := &g.state
	a := g.ai
	np := int(s.NumPlayers)
	for k := int(s.NumShips) - 1; k >= 0; k-- {
		sh := &s.Ships[k]
		if int8(sh.Status) >= 3 {
			continue
		}
		o := int(sh.Owner)
		st, eta := -1, -1
		switch sh.Status {
		case save.ShipAtStar:
			st, eta = int(sh.Location), 0
		case save.ShipInTransit:
			st, eta = int(sh.Location)-500, int(sh.ETA)
		case save.ShipDeparting:
			st, eta = g.xyToStarID(sh.X, sh.Y), 0
		}
		if st >= int(s.NumStars) {
			continue
		}
		// A star of -1 would read before the star array; the port sees 0 there.
		neb := s.starRecord(st).InNebula
		if o < 8 && eta < 10 && s.Players[o].Objective == save.ObjectiveHuman {
			a.setU8(a.at(aiEnrouteHumanShips)+st*10+eta, 1)
		}
		for p := np - 1; p >= 0; p-- {
			if p == o || s.Players[p].Eliminated != 0 {
				continue
			}
			if sh.Design.Type == 0 {
				v := uint16(g.shipStrengthVsShips(k, p))
				a.setU16(a.at(aiShipStrengths)+k*16+p*2, v)
				x := a.at(aiPlayerInfo) + o*0x3a + p*4 + 0x1a
				a.setU32(x, a.u32(x)+uint32(v))
				a.setU16(a.at(aiShipStrengthsInNebula)+k*16+p*2, uint16(g.shipStrengthVsShipsInNebula(k, p)))
				g.shipStrengthVsPlanetaryShield(k, a.at(aiShipBombStrengths)+k*8)
				if o < 8 && eta < 10 {
					src := a.at(aiShipStrengths)
					if neb != 0 {
						src = a.at(aiShipStrengthsInNebula)
					}
					x := a.at(aiEnrouteShipStrengths) + st*0xa00 + o*0x140 + p*0x28 + eta*4
					a.setU32(x, a.u32(x)+uint32(a.u16(src+k*16+p*2)))
				}
			}
			if o >= 8 {
				continue
			}
			if eta < 10 {
				a.setU8(a.at(aiEnrouteShips)+st*0x50+o*10+eta, 1)
			}
			x := a.at(aiEnrouteNonstealthyShips) + st*8 + o
			if s.Players[o].Picks[save.PickStealthyShips] == 0 && a.u8(x) == 0 && !s.shipHasStealthDevice(k) {
				a.setU8(x, 1)
			}
		}
	}
}

// aiStarRanges fills _star_in_range_of_player and _star_in_extended_range_of_player
// from the colonies: within 30 or 45 times the fuel range, line not blocked.
func (g *Game) aiStarRanges() {
	s := &g.state
	a := g.ai
	var normal, ext [8]int32
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		r := int32(s.Players[p].FuelRange) * 30
		normal[p] = r * r
		e := r * 3 / 2
		ext[p] = e * e
	}
	a.fill(aiStarInRange, 0)
	a.fill(aiStarInExtendedRange, 0)
	for c := int(s.NumColonies) - 1; c >= 0; c-- {
		col := &s.Colonies[c]
		if col.Planet == -1 {
			continue
		}
		C := &s.Stars[s.Planets[col.Planet].Star]
		o := int(col.Owner)
		bit := uint8(1) << (uint(col.Owner) & 31)
		for t := int(s.NumStars) - 1; t >= 0; t-- {
			T := &s.Stars[t]
			dx, dy := int32(C.X-T.X), int32(C.Y-T.Y)
			d := dx*dx + dy*dy
			if d > ext[o] || blackHoleBit(C, t) {
				continue
			}
			x := a.at(aiStarInExtendedRange) + t
			a.setU8(x, a.u8(x)|bit)
			if d > normal[o] || blackHoleBit(C, t) {
				continue
			}
			x = a.at(aiStarInRange) + t
			a.setU8(x, a.u8(x)|bit)
		}
	}
}

// extRange is _star_in_extended_range_of_player[st] >> p & 1.
func (a *aiArena) extRange(st, p int) bool {
	return maskBit(a.u8(a.at(aiStarInExtendedRange)+st), p)
}

// inRange is _star_in_range_of_player[st] >> p & 1.
func (a *aiArena) inRange(st, p int) bool { return maskBit(a.u8(a.at(aiStarInRange)+st), p) }

// colonyInfo is colony ci's 7 bytes of _ai_colony_info.
func (a *aiArena) colonyInfo(ci int) []byte {
	o := a.at(aiColonyInfo) + ci*7
	return a.mem[o : o+7]
}

// starInfo is star st's 0x31 bytes of _ai_star_info.
func (a *aiArena) starInfo(st int) []byte {
	o := a.at(aiStarInfo) + st*0x31
	return a.mem[o : o+0x31]
}

// aiTurnTables fills _n_turns_from_to (star to star, by player) and
// _ship_n_turns_to (each moving ship to each star its owner can reach).
func (g *Game) aiTurnTables() {
	s := &g.state
	a := g.ai
	ns := int(s.NumStars)
	a.fill(aiNTurnsFromTo, 0xff)
	a.fill(aiNTurnsFromToIntermediate, 0xff)
	for from := ns - 1; from >= 0; from-- {
		for p := int(s.NumPlayers) - 1; p >= 0; p-- {
			if !a.extRange(from, p) {
				continue
			}
			for to := ns - 1; to >= 0; to-- {
				if a.extRange(to, p) {
					a.setU8(a.at(aiNTurnsFromTo)+from*0x240+to*8+p, uint8(g.playerNTurnsFromStarToStar(p, from, to)))
				}
			}
		}
	}
	a.fill(aiShipNTurnsTo, 0xff)
	a.fill(aiShipNTurnsToIntermediate, 0xff)
	for k := int(s.NumShips) - 1; k >= 0; k-- {
		sh := &s.Ships[k]
		if int8(sh.Status) >= 5 || sh.Status != save.ShipInTransit {
			continue
		}
		dest := int(shipDestinationStar(sh))
		a.setU8(a.at(aiShipNTurnsTo)+k*0x48+dest, sh.ETA)
		if !s.canOrderShip(k) {
			continue
		}
		bit := uint8(1) << (uint(sh.Owner) & 31)
		ids := []int16{int16(k), -1}
		for t := ns - 1; t >= 0; t-- {
			if a.u8(a.at(aiStarInExtendedRange)+t)&bit == 0 || t == dest {
				continue
			}
			info := g.shipsTryToMoveTo(ids, t)
			a.setU8(a.at(aiShipNTurnsTo)+k*0x48+t, uint8(info.turns))
		}
	}
}

// shipDestinationStar is Ship_Destination_Star_ (0x1003f2).
func shipDestinationStar(sh *save.Ship) int16 {
	switch sh.Status {
	case save.ShipAtStar:
		return sh.Location
	case save.ShipInTransit:
		return sh.Location - 500
	case save.ShipDeparting:
		return sh.Location - 1000
	}
	return sh.Location
}

// aiColonyInfo fills _ai_colony_info, 7 bytes per colony: dominant race,
// its pop limit, food and tolerance flags, enemies arriving, androids.
func (g *Game) aiColonyInfo() {
	s := &g.state
	a := g.ai
	a.fill(aiColonyInfo, 0)
	for c := int(s.NumColonies) - 1; c >= 0; c-- {
		col := &s.Colonies[c]
		if col.Owner == -1 {
			continue
		}
		x := a.at(aiColonyInfo) + c*7
		a.setU8(x+6, 0)
		for i := int(col.PopCount) - 1; i >= 0; i-- {
			if colonistByte(col, i, 1)&4 != 0 {
				a.setU8(x+6, a.u8(x+6)+1)
			}
		}
		dom := uint8(s.colonyDominantPlayer(col))
		a.setU8(x, dom)
		a.setU8(x+1, uint8(g.colonyRacePopLimit(col, int(dom))))
		D := &s.Players[dom]
		if D.Picks[save.PickLithovore] == 0 || s.Players[col.Owner].Picks[save.PickLithovore] == 0 {
			a.setU8(x+2, 1)
		}
		if D.Picks[save.PickTolerant] == 0 {
			a.setU8(x+4, 1)
		}
	}
	for c := int(s.NumColonies) - 1; c >= 0; c-- {
		col := &s.Colonies[c]
		o := int(col.Owner)
		if o == -1 {
			continue
		}
		st := int(s.Planets[col.Planet].Star)
		for q := int(s.NumPlayers) - 1; ; q-- {
			if q != o && a.u8(a.at(aiEnrouteShips)+st*0x50+q*10+9) != 0 {
				a.setU8(a.at(aiColonyInfo)+c*7+5, 1)
				break
			}
			if q <= 0 {
				break
			}
		}
	}
}

// colonistByte is byte i of colonist n's packed record (colony+0xc+n*4).
func colonistByte(c *save.Colony, n, i int) uint8 { return uint8(c.Colonists[n] >> (8 * i)) }

// aiStarInfo fills the first parts of _ai_star_info (0x31 bytes per star):
// the planet count and, per AI player in range, whether enemies arrive.
func (g *Game) aiStarInfo() {
	s := &g.state
	a := g.ai
	np := int(s.NumPlayers)
	a.fill(aiStarInfo, 0)
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		x := a.at(aiStarInfo) + st*0x31
		for orbit := 4; orbit >= 0; orbit-- {
			if s.Stars[st].Planets[orbit] != -1 {
				a.setU8(x, a.u8(x)+1)
			}
		}
		for p := np - 1; p >= 0; p-- {
			P := &s.Players[p]
			if P.Objective == save.ObjectiveHuman || P.Eliminated != 0 || !a.extRange(st, p) {
				continue
			}
			for q := np - 1; q >= 0; q-- {
				if q != p && a.u8(a.at(aiEnrouteShips)+st*0x50+q*10+9) != 0 {
					a.setU8(x+p*6+6, 1)
					break
				}
			}
		}
	}
}

// aiPlayerInfo fills _ai_player_info (0x3a bytes per player): stars and
// colonies, blockades, contacts and neighbours.
func (g *Game) aiPlayerInfo() {
	s := &g.state
	a := g.ai
	np, ns := int(s.NumPlayers), int(s.NumStars)
	inc16 := func(x int) { a.setU16(x, a.u16(x)+1) }
	for p := np - 1; p >= 0; p-- {
		x := a.at(aiPlayerInfo) + p*0x3a
		for st := ns - 1; st >= 0; st-- {
			if !maskBit(s.Stars[st].ColonyOwners, p) {
				continue
			}
			inc16(x + 6)
			if maskBit(s.Stars[st].Blockaded, p) {
				inc16(x + 8)
			}
		}
		me := uint8(1) << (uint(p) & 31)
		for q := np - 1; q >= 0; q-- {
			if q == p {
				continue
			}
			met := false
			if s.Players[p].Contact[q] != 0 {
				met = true
				inc16(x + 0xa)
			} else {
				both := me | uint8(1)<<(uint(q)&31)
				for st := ns - 1; st >= 0; st-- {
					if a.u8(a.at(aiStarInfo)+st*0x31) != 0 && a.u8(a.at(aiStarInExtendedRange)+st)&both == both {
						met = true
						break
					}
				}
			}
			if met {
				inc16(x + 0xc)
				a.setU8(x+0xe+q, 1)
			}
		}
	}
	for c := int(s.NumColonies) - 1; c >= 0; c-- {
		col := &s.Colonies[c]
		if col.Owner == -1 {
			continue
		}
		x := a.at(aiPlayerInfo) + int(col.Owner)*0x3a
		inc16(x)
		if maskBit(s.Stars[s.Planets[col.Planet].Star].Blockaded, int(col.Owner)) {
			inc16(x + 4)
		}
	}
	most := int32(0)
	for p := np - 1; p >= 0; p-- {
		if v := int32(int16(a.u16(a.at(aiPlayerInfo) + p*0x3a))); most < v {
			most = v
		}
	}
	for p := np - 1; p >= 0; p-- {
		x := a.at(aiPlayerInfo) + p*0x3a
		a.setU16(x+2, uint16(most)-a.u16(x))
	}
}

// aiRefitTables collects the players' active combat ships (_all_ships,
// sorted), how much each wants a refit (_airefit) and per-star indices.
func (g *Game) aiRefitTables() {
	s := &g.state
	a := g.ai
	np := int16(s.NumPlayers)
	a.nAllShips = 0
	a.fill(aiRefit, 0)
	a.fill(aiShipRefitDesirabilities, 0)
	for k := int(s.NumShips) - 1; k >= 0; k-- {
		sh := &s.Ships[k]
		if sh.Design.Type != 0 || int16(sh.Owner) >= np || int8(sh.Status) >= 3 || sh.AIMission == 4 {
			continue
		}
		a.setU16(a.at(aiAllShips)+int(a.nAllShips)*2, uint16(k))
		a.nAllShips++
		if sh.Status != save.ShipAtStar {
			continue
		}
		v := g.refitDesirability(k)
		a.setU32(a.at(aiShipRefitDesirabilities)+k*4, v)
		r := a.at(aiRefit) + int(sh.Owner)*0xbba
		n := int(int16(a.u16(r)))
		a.setU32(r+n*6+4, v)
		if v != 0 {
			a.setU16(r+n*6+2, uint16(k))
			a.setU16(r, a.u16(r)+1)
		}
	}
	type refit struct {
		ship  uint16
		value uint32
	}
	for p := int(np) - 1; p >= 0; p-- {
		r := a.at(aiRefit) + p*0xbba
		list := make([]refit, int(int16(a.u16(r))))
		for i := range list {
			list[i] = refit{a.u16(r + 2 + i*6), a.u32(r + 4 + i*6)}
		}
		clib.Qsort(list, 6, func(x, y *refit) int { return int(int32(x.value - y.value)) })
		for i, e := range list {
			a.setU16(r+2+i*6, e.ship)
			a.setU32(r+4+i*6, e.value)
		}
	}
	a.fill(aiAllCombatShipsIndex, 0xff)
	all := make([]int16, a.nAllShips)
	for i := range all {
		all[i] = int16(a.u16(a.at(aiAllShips) + i*2))
	}
	clib.Qsort(all, 2, func(x, y *int16) int { return s.sortShips(int(*x), int(*y)) })
	for i, k := range all {
		a.setU16(a.at(aiAllShips)+i*2, uint16(k))
	}
	for i := int16(0); i < a.nAllShips; i++ {
		sh := &s.Ships[all[i]]
		x := a.at(aiAllCombatShipsIndex) + int(sh.Owner)*0x92 + int(s.NumStars)*2
		if sh.Location < 0x48 {
			x = a.at(aiAllCombatShipsIndex) + int(sh.Owner)*0x92 + int(sh.Location)*2
		}
		if int16(a.u16(x)) == -1 {
			a.setU16(x, uint16(i))
		}
	}
}

// sortShips is sort_ships_ (0xd3318): by owner, then by star for ships at
// stars (below 72), else by position.
func (s *State) sortShips(i, j int) int {
	A, B := &s.Ships[i], &s.Ships[j]
	switch {
	case int16(A.Owner) < int16(B.Owner):
		return -1
	case int16(B.Owner) < int16(A.Owner):
		return 1
	}
	la, lb := A.Location, B.Location
	switch {
	case la < lb && la < 0x48:
		return -1
	case lb < la && lb < 0x48:
		return 1
	case la < 0x48 && lb < 0x48:
		return 0
	case A.X < B.X:
		return -1
	case B.X < A.X:
		return 1
	case A.Y < B.Y:
		return -1
	case B.Y < A.Y:
		return 1
	}
	return 0
}

// aiPossibleShipStrengths fills _possible_ship_strengths: for each target
// star, owner and opposing player, the strength arriving within e turns.
func (g *Game) aiPossibleShipStrengths() {
	s := &g.state
	a := g.ai
	a.fill(aiPossibleShipStrengths, 0)
	for i := int(a.nAllShips) - 1; i >= 0; i-- {
		k := int(int16(a.u16(a.at(aiAllShips) + i*2)))
		sh := &s.Ships[k]
		stealth := testBit(sh.Design.Specials[:], 11)
		loc := int(sh.Location)
		o := int(sh.Owner)
		eta := 0
		switch {
		case sh.Status == save.ShipAtAntares || loc < int(s.NumStars):
		case loc >= 1000:
			loc = g.xyToStarID(sh.X, sh.Y)
		default:
			eta = int(sh.ETA)
			loc -= 500
		}
		src := a.at(aiShipStrengths)
		if s.starRecord(loc).InNebula != 0 {
			src = a.at(aiShipStrengthsInNebula)
		}
		for q := int(s.NumPlayers) - 1; q >= 0; q-- {
			if q == o {
				continue
			}
			for t := int(s.NumStars) - 1; t >= 0; t-- {
				if !a.extRange(t, q) && t != loc {
					continue
				}
				n := g.playerNTurnsFromStarToStarWithIntermediate(o, loc, t, nil, stealth)
				for e := eta + int(n); e < 10; e++ {
					x := a.at(aiPossibleShipStrengths) + t*0xa00 + o*0x140 + q*0x28 + e*4
					a.setU32(x, a.u32(x)+uint32(a.u16(src+k*16+q*2)))
				}
			}
		}
	}
}

// aiStageWeights fills the planet, base star and star stage weights: what
// each player's best planet at a star is worth and the threat to it.
func (g *Game) aiStageWeights() {
	s := &g.state
	a := g.ai
	np, ns := int(s.NumPlayers), int(s.NumStars)
	a.fill(aiPlanetStageWeights, 0)
	for pl := int(s.PlanetCount) - 1; pl >= 0; pl-- {
		if c := s.Planets[pl].Colony; c != -1 {
			o := int(s.Colonies[c].Owner)
			a.setU16(a.at(aiPlanetStageWeights)+pl*16+o*2, a.basePlanetValues[o][pl])
		}
	}
	a.fill(aiStarStageWeights, 0)
	a.fill(aiBaseStarStageWeights, 0)
	var bestPlanet, bestWeight [72 * 8]int16
	for i := range bestPlanet {
		bestPlanet[i] = -1
	}
	// threat[p][q]: q's strength counts against p; every player's when p is
	// at war with nobody, else only humans'.
	var threat [8][8]bool
	for p := np - 1; p >= 0; p-- {
		war := false
		for q := np - 1; q >= 0; q-- {
			if s.Players[q].Objective == save.ObjectiveHuman {
				threat[p][q] = true
			}
			if t := s.Players[p].TreatyStatus[q]; t >= 4 && t <= 6 {
				war = true
			}
		}
		if !war {
			threat[p] = [8]bool{true, true, true, true, true, true, true, true}
		}
		threat[p][p] = false
	}
	psw := func(pl, p int) uint16 { return a.u16(a.at(aiPlanetStageWeights) + pl*16 + p*2) }
	for st := ns - 1; st >= 0; st-- {
		S := &s.Stars[st]
		if S.ColonyOwners == 0 {
			continue
		}
		for orbit := 4; orbit >= 0; orbit-- {
			pl := int(S.Planets[orbit])
			if pl == -1 || s.Planets[pl].Colony == -1 {
				continue
			}
			o := int(s.Colonies[s.Planets[pl].Colony].Owner)
			i := st*8 + o
			if w := psw(pl, o); int32(bestWeight[i]) < int32(w) {
				bestPlanet[i] = int16(pl)
				bestWeight[i] = int16(w)
			}
		}
		for p := np - 1; p >= 0; p-- {
			pl := int(bestPlanet[st*8+p])
			if pl == -1 {
				continue
			}
			var threatSum, nearSum uint32
			for q := np - 1; q >= 0; q-- {
				if !threat[p][q] {
					continue
				}
				qs := int32(int16(a.u16(a.at(aiPlayerInfo) + q*0x3a + 6)))
				ps := int32(int16(a.u16(a.at(aiPlayerInfo) + p*0x3a + 6)))
				lim := int16(8)
				if qs*5 < ps {
					lim = 4
					if qs == 1 {
						lim = 1
					}
				}
				for t := ns - 1; t >= 0; t-- {
					v := a.u32(a.at(aiPossibleShipStrengths) + t*0xa00 + q*0x140 + p*0x28)
					if v == 0 {
						continue
					}
					if n := g.playerNTurnsFromStarToStarWithIntermediate(q, t, st, nil, true); n < lim {
						threatSum += v / uint32(int32(n)+1)
					}
				}
			}
			for pl2 := int(s.PlanetCount) - 1; pl2 >= 0; pl2-- {
				if pl2 == pl {
					continue
				}
				if g.playerNTurnsFromStarToStarWithIntermediate(p, st, int(s.Planets[pl2].Star), nil, false) <= 2 {
					nearSum += uint32(psw(pl2, p))
				}
			}
			base := nearSum>>4 + uint32(int32(bestWeight[st*8+p]))
			a.setU32(a.at(aiBaseStarStageWeights)+st*32+p*4, base)
			threatSum = (threatSum + 7) >> 3
			a.setU32(a.at(aiStarStageWeights)+st*32+p*4, threatSum*((base+7)>>3))
		}
	}
}
