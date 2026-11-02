package game

import "encoding/binary"

// History graphs (BILL.C). Spec: docs/turn/overview.md, phase F, and
// docs/formats/save-players.md (History).

// tblBuildingCost is the s32 cost in the 0x13-byte building table rows that
// Compute_Total_Building_Costs_ sums.
const (
	tblBuildingCost    = 0x17eb45
	tblBuildingCostRow = 0x13
)

// History series in Player.History and their divisors in bill_savegame.
const (
	histFleet, histTech, histPop, histBuildings = 0, 1, 2, 3
	historyLen                                  = 350
)

// computeTotalBuildingCosts is Compute_Total_Building_Costs_ (0xe2671): per
// player the cost of every building in its colonies (outposts excluded).
func (g *Game) computeTotalBuildingCosts() (out [8]int32) {
	s := &g.state
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if c.Owner == -1 || c.IsOutpost != 0 {
			continue
		}
		for k := 48; k >= 0; k-- {
			if c.Buildings[k] != 0 {
				out[c.Owner] += int32(g.exe.u32(uint32(tblBuildingCost + k*tblBuildingCostRow)))
			}
		}
	}
	return out
}

// recordHistory is Record_History_ (0x10208a): one sample of the four graphs
// per player into the ring at bill_savegame+2. Each series has a shared
// divisor that grows until every sample fits in 250; old samples rescale.
func (g *Game) recordHistory() {
	s := &g.state
	b := s.BillSavegame[:]
	word := func(off int) uint16 { return binary.LittleEndian.Uint16(b[off:]) }
	costs := g.computeTotalBuildingCosts()
	for off := 4; off <= 0xa; off += 2 {
		if word(off) < 1 {
			binary.LittleEndian.PutUint16(b[off:], 1)
		}
	}
	// div[series] starts from bill_savegame +8 (fleet), +0xa (tech), +4
	// (population) and +6 (buildings).
	offs := [4]int{histFleet: 8, histTech: 0xa, histPop: 4, histBuildings: 6}
	var div [4]uint16
	for k, off := range offs {
		div[k] = word(off)
	}
	var v [4][8]uint32
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		if int8(sh.Status) > 1 || sh.Owner < 0 || sh.Owner >= 8 {
			continue
		}
		v[histFleet][sh.Owner] += 10 * (1 << (uint(sh.Design.Size) + 1))
	}
	order := [4]int{histPop, histBuildings, histTech, histFleet}
	for p := 0; p < int(s.NumPlayers); p++ {
		P := &s.Players[p]
		v[histPop][p] = uint32(int32(P.Population))
		v[histBuildings][p] = uint32(costs[p])
		v[histTech][p] = uint32(P.KnownTechCost)
		for _, k := range order {
			for v[k][p]/uint32(div[k]) > 250 {
				div[k]++
			}
		}
	}
	for p := 0; p < int(s.NumPlayers); p++ {
		for _, k := range order {
			v[k][p] /= uint32(div[k])
		}
	}
	ring := int(word(2))
	for p := 0; p < int(s.NumPlayers); p++ {
		h := &s.Players[p].History
		changed := false
		for k, off := range offs {
			if div[k] != word(off) {
				changed = true
			}
		}
		if changed {
			for i := 0; i < historyLen; i++ {
				for _, k := range order {
					h[k][i] = byte(int32(h[k][i]) * int32(word(offs[k])) / int32(div[k]))
				}
			}
		}
		for _, k := range order {
			h[k][ring] = byte(v[k][p])
		}
	}
	binary.LittleEndian.PutUint16(b[2:], uint16(ring+1))
	for k, off := range offs {
		binary.LittleEndian.PutUint16(b[off:], div[k])
	}
	if word(2) >= historyLen {
		binary.LittleEndian.PutUint16(b[2:], 0)
	}
}
