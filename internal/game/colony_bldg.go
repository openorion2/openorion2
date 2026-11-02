package game

import (
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/gfx"
)

// The colony screen's planet surface: the 6x6 building array, roads and
// vegetation (COLONY.C, COLDRAW.C). Spec: docs/screens/colony-screen.md,
// "The planet surface".

// Tables of COLONY.C and COLDRAW.C, read from the executable.
const (
	bldgBackToFrontTbl = 0xba7cc  // Bldg_Back_To_Front_'s 36 (c, r) pairs
	roadBackToFrontTbl = 0xb4d5b  // Road_Back_To_Front_'s 49 (c, r) pairs
	roadsAroundTbl     = 0xb4de9  // N_Roads_Around_CR_'s (dc, dr, side) triples
	roadsPickTbl       = 0xb4df5  // Pick_Random_Road_At_And_Around_CR_'s triples
	houseKindsTbl      = 0xba77c  // Make_Bldg_Array_For_Colony_'s four house ids
	bldgCornerXY       = 0x182c9c // CR_To_XY_: 7x7 corners, dword x and y
	bldgCenterXY       = 0x182e24 // Bldg_Coords_To_Centered_Screen_Coord_'s copy
	buildingCostTbl    = 0x17eb45 // _buildings[].cost, int32, 19-byte rows
	buildingTypeTbl    = 0x17eb4b // _buildings[].type: 7 satellite
	colveggieLBX       = 0x18299c
	colroadsLBX        = 0x182a09
	bldgLBXFmt         = 0xc9 // E string "bldg%d.lbx"
)

// roadListSize is _road_list's size: 7x7 cells of 0x1d bytes.
const roadListSize = 0x58d

// veggieCell is one cell of _veggie_list (0x19e038): two plants, each with
// a picture, a position and a draw flag.
type veggieCell struct {
	pic  [2]int16
	x, y [2]int16
	on   [2]int16
}

// colonySurface is the surface state COLONY.C and COLDRAW.C keep.
type colonySurface struct {
	bldgs      [6][6]int32        // _colony_bldgs (0x182afd), [column][row]
	satellites [10]int16          // _colony_satellites (0x19f99c)
	roads      [roadListSize]byte // _road_list (0x19e564)
	veggies    [6][6]veggieCell   // _veggie_list
}

// roadCell is the offset of cell (c, r) in _road_list.
func roadCell(c, r int) int { return c*0xcb + r*0x1d }

// roadOn is the flag of side k of cell (c, r) (+0x18).
func (cs *colonySurface) roadOn(c, r, k int) *byte {
	return &cs.roads[roadCell(c, r)+0x18+k]
}

// roadPic is the picture of side k of cell (c, r) (+0x10, a short).
func (cs *colonySurface) roadPic(c, r, k int) int16 {
	o := roadCell(c, r) + 0x10 + 2*k
	return int16(binary.LittleEndian.Uint16(cs.roads[o:]))
}

func (cs *colonySurface) setRoadPic(c, r, k int, v int16) {
	o := roadCell(c, r) + 0x10 + 2*k
	binary.LittleEndian.PutUint16(cs.roads[o:], uint16(v))
}

func (g *Game) buildingCost(id int32) int32 {
	return int32(g.exe.u32(uint32(int32(buildingCostTbl) + id*0x13)))
}

func (g *Game) buildingType(id int32) byte {
	return g.exe.bytes(uint32(int32(buildingTypeTbl)+id*0x13), 1)[0]
}

// isHouse reports the four house pictures Insert_Bldg_Into_Array_ uses.
func isHouse(id int32) bool { return id == 3 || id == 0xe || id == 0x28 || id == 0x29 }

// nBldgs is N_Bldgs_ (0xba96d): how many buildings colony ci has.
func (g *Game) nBldgs(ci int) int {
	n := 0
	for k := 1; k < 0x31; k++ {
		if g.state.Colonies[ci].Buildings[k] != 0 {
			n++
		}
	}
	return n
}

// makeBldgArrayForColony is Make_Bldg_Array_For_Colony_ (0xbc30b): place
// colony ci's buildings and houses on the 6x6 surface, then roads and plants,
// with the RNG seeded by the colony and restored after. small keeps to the
// front half (the landing screen's view).
func (g *Game) makeBldgArrayForColony(ci int, small bool) error {
	s, cs := &g.state, &g.cscr.surf
	saved := s.Random.Get()
	s.Random.Set(uint32(int32(int16(ci))))
	cs.bldgs = [6][6]int32{}
	for i := range cs.satellites {
		cs.satellites[i] = -1
	}
	n, nsat := 0, 0
	c := &s.Colonies[ci]
	for id := int32(0); id < 0x31; id++ {
		if c.Buildings[id] == 0 || (small && n >= 0x12) {
			continue
		}
		if g.buildingType(id) != 7 {
			g.insertBldgIntoArray(ci, id, n, small)
			n++
		} else if !g.satShouldBeReplaced(id) {
			cs.satellites[nsat] = int16(id)
			nsat++
		}
	}
	for k := 0; k < int(c.PopCount)/3+1; k++ {
		if n < 0x24 && ((small && n < 0x12) || !small) {
			g.insertBldgIntoArray(ci, -3, n, small)
			n++
		}
	}
	g.sortBldgArrayColumns()
	if !small {
		var houses [4]int16
		for i := range houses {
			houses[i] = g.exeI16(houseKindsTbl + 2*uint32(i))
		}
		for i := 0; i < 8; i++ {
			col, row, ok := g.getBldgCR(int32(houses[i%4]))
			if !ok {
				continue
			}
			moved := false
			for dc := 1; dc > -2 && !moved; dc-- {
				for dr := 1; dr > -2 && !moved; dr-- {
					// The row index is the column's too, as in the original.
					k := min(max(col-dc, 0), 5)
					if (cs.bldgs[k][k] != 0 && s.Random.Random(3) == 1) || s.Random.Random(3) == 1 {
						v := int32(int16(cs.bldgs[k][k]))
						cs.bldgs[k][k] = cs.bldgs[col][row]
						cs.bldgs[col][row] = v
						moved = true
					}
				}
			}
		}
		if col, row, ok := g.getBldgCR(9); ok && cs.bldgs[1][1] != 0 {
			v := int32(int16(cs.bldgs[1][1]))
			cs.bldgs[1][1] = cs.bldgs[col][row]
			cs.bldgs[col][row] = v
		}
	}
	g.buildRoadListBasedOnBldgList()
	var err error
	if !small {
		err = g.buildVeggieListBasedOnBldgList()
	} else {
		g.clearVeggieDrawFlags()
	}
	s.Random.Set(saved)
	return err
}

// satShouldBeReplaced is Sat_Should_Be_Replaced_ (0xbc21b): a satellite
// another of the colony screen's colony supersedes.
func (g *Game) satShouldBeReplaced(id int32) bool {
	c := &g.state.Colonies[g.colonyHandle]
	switch id {
	case 0x28:
		return c.Buildings[8] != 0 || c.Buildings[0x29] != 0
	case 8:
		return c.Buildings[0x29] != 0
	}
	return false
}

// insertBldgIntoArray is Insert_Bldg_Into_Array_ (0xbc05e): put building id
// (-3 a house, -1 rubble) in a random free cell, or replace the cheapest.
func (g *Game) insertBldgIntoArray(ci int, id int32, n int, small bool) {
	s, cs := &g.state, &g.cscr.surf
	col, row := -1, -1
	turn := (int(int16(g.nHousesInBldgArray())) + int(int16(ci))) % 4
	switch {
	case n >= 0x24:
		col, row = g.findReplacementSlotForBuilding(id, col, row)
	case small:
		col, row = g.bldgBackToFront(n)
	default:
		free := 0
		for c := 0; c < 6; c++ {
			for r := 0; r < 6; r++ {
				if cs.bldgs[c][r] == 0 {
					free++
					if s.Random.Random(uint32(int32(int16(free)))) == 1 {
						col, row = c, r
					}
				}
			}
		}
		if col == -1 || (row == -1 && id != -3) {
			col, row = g.findReplacementSlotForBuilding(id, col, row)
		}
		if col == -1 || row == -1 {
			return
		}
	}
	switch {
	case id >= 0:
		cs.bldgs[col][row] = id
	case id == -3:
		k := []int32{3, 0xe, 0x28, 0x29}[(turn+1)%4]
		cs.bldgs[col][row] = k
	case id == -1:
		cs.bldgs[col][row] = 8
	}
}

// nHousesInBldgArray is N_Houses_In_Bldg_Array_ (0xbbf5e).
func (g *Game) nHousesInBldgArray() int {
	n := 0
	for c := range g.cscr.surf.bldgs {
		for _, b := range g.cscr.surf.bldgs[c] {
			if isHouse(b) {
				n++
			}
		}
	}
	return n
}

// findReplacementSlotForBuilding is Find_Replacement_Slot_For_Building_
// (0xbbfae): the last house, else the cheapest building cheaper than 1000.
// A negative id leaves the cell as it was.
func (g *Game) findReplacementSlotForBuilding(id int32, col, row int) (int, int) {
	if int16(id) < 0 {
		return col, row
	}
	cs := &g.cscr.surf
	best := int16(1000)
	col, row = 0, 0
	for c := 0; c < 6; c++ {
		for r := 0; r < 6; r++ {
			b := int16(cs.bldgs[c][r])
			if isHouse(int32(b)) {
				col, row, best = c, r, 0
			} else if cost := g.buildingCost(int32(b)); cost < int32(best) {
				col, row, best = c, r, int16(cost)
			}
		}
	}
	return col, row
}

// sortBldgArrayColumns is Sort_Bldg_Array_Columns_ (0xbbdc9): up to six
// passes moving lower building types towards the back.
func (g *Game) sortBldgArrayColumns() {
	b := &g.cscr.surf.bldgs
	swap := func(c1, r1, c2, r2 int, last *int16) {
		a, z := b[c1][r1], b[c2][r2]
		if g.buildingType(a) > g.buildingType(z) && a != 0 && z != 0 {
			*last = int16(a)
			b[c1][r1] = z
			b[c2][r2] = int32(int16(a))
		}
	}
	for pass := 0; pass < 6; pass++ {
		last := int16(0)
		for c := 0; c < 6; c++ {
			for r := 0; r < 6; r++ {
				if c+1 >= 6 || r+1 >= 6 {
					continue
				}
				swap(c, r, c+1, r+1, &last)
				swap(c, r, c+1, r, &last)
				swap(c, r, c, r+1, &last)
			}
		}
		if last == 0 {
			return
		}
	}
}

// getBldgCR is Get_Bldg_CR_ (0xbbd37): the cell of building id, else a
// random empty cell (true only for id -1).
func (g *Game) getBldgCR(id int32) (col, row int, ok bool) {
	s, cs := &g.state, &g.cscr.surf
	free := 0
	for c := 0; c < 6; c++ {
		for r := 0; r < 6; r++ {
			v := cs.bldgs[c][r]
			if v == int32(int16(id)) {
				return c, r, true
			}
			if v == 0 {
				free++
				if s.Random.Random(uint32(int32(int16(free)))) == 1 {
					col, row = c, r
				}
			}
		}
	}
	return col, row, int16(id) == -1
}

// bldgBackToFront is Bldg_Back_To_Front_ (0xbebae): the n-th cell from the
// back.
func (g *Game) bldgBackToFront(n int) (int, int) {
	t := g.exe.bytes(bldgBackToFrontTbl+uint32(2*n), 2)
	return int(t[0]), int(t[1])
}

// roadBackToFront is Road_Back_To_Front_ (0xb6227).
func (g *Game) roadBackToFront(n int) (int, int) {
	t := g.exe.bytes(roadBackToFrontTbl+uint32(2*n), 2)
	return int(t[0]), int(t[1])
}

// loadRoadListAnims is Load_Road_List_Anims_ (0xb5fbe): number the road
// pictures of every side that exists.
func (g *Game) loadRoadListAnims() {
	cs := &g.cscr.surf
	cs.roads = [roadListSize]byte{}
	n := int16(0)
	for k := 0; k < 4; k++ {
		for c := 0; c < 7; c++ {
			for r := 0; r < 7; r++ {
				if (c == 6 && k != 0) || (r == 6 && k == 0) || (k > 1 && (r == 6 || c == 6)) {
					cs.setRoadPic(c, r, k, -1)
				} else {
					cs.setRoadPic(c, r, k, n)
					n++
				}
				*cs.roadOn(c, r, k) = 0
			}
		}
	}
}

// buildRoadListBasedOnBldgList is Build_Road_List_Based_On_Bldg_List_
// (0xb6099). Link_Buildings_ before it only calls the empty
// Draw_Simplest_Path_.
func (g *Game) buildRoadListBasedOnBldgList() {
	s, cs := &g.state, &g.cscr.surf
	rnd := func() uint32 { return s.Random.Random(2) }
	for c := 0; c < 6; c++ {
		for r := 0; r < 6; r++ {
			if cs.bldgs[c][r] != 0 {
				if rnd() == 1 {
					*cs.roadOn(c, r, 0) = 1
				} else {
					*cs.roadOn(c, r, 1) = 1
				}
				if rnd() != 1 {
					*cs.roadOn(c+1, r, 0) = 1
				}
				if rnd() != 1 {
					*cs.roadOn(c, r+1, 1) = 1
				}
				*cs.roadOn(c, r+1, 2) = 0
				*cs.roadOn(c, r+1, 3) = 0
				continue
			}
			if *cs.roadOn(c, r+1, 2) != 0 && *cs.roadOn(c, r+1, 3) != 0 {
				if rnd() == 1 {
					*cs.roadOn(c, r+1, 3) = 0
				} else {
					*cs.roadOn(c, r+1, 2) = 0
				}
			}
			if g.nRoadsAroundCR(c, r) == 4 && (*cs.roadOn(c, r, 2) == 1 || *cs.roadOn(c, r, 3) == 1) {
				pc, pr, pk := c, r, 0
				if g.pickRandomRoadAtAndAroundCR(&pc, &pr, &pk, 4) != 0 {
					*cs.roadOn(pc, pr, pk) = 0
				}
			}
		}
	}
}

// nRoadsAroundCR is N_Roads_Around_CR_ (0xb67e4).
func (g *Game) nRoadsAroundCR(c, r int) int {
	t := g.exe.bytes(roadsAroundTbl, 12)
	n := 0
	for i := 0; i < 4; i++ {
		if *g.cscr.surf.roadOn(c+int(t[3*i]), r+int(t[3*i+1]), int(t[3*i+2])) != 0 {
			n++
		}
	}
	return n
}

// pickRandomRoadAtAndAroundCR is Pick_Random_Road_At_And_Around_CR_
// (0xb6860): among the first n sides, a random one whose flag equals *k.
func (g *Game) pickRandomRoadAtAndAroundCR(c, r, k *int, n int) int {
	t := g.exe.bytes(roadsPickTbl, 12)
	s := &g.state
	nc, nr, nk := *c, *r, *k
	cnt := 0
	for i := 0; i < n; i++ {
		cc, rr, kk := *c+int(t[3*i]), *r+int(t[3*i+1]), int(t[3*i+2])
		if int(*g.cscr.surf.roadOn(cc, rr, kk)) == *k {
			cnt++
			if s.Random.Random(uint32(int32(int16(cnt)))) == 1 {
				nc, nr, nk = cc, rr, kk
			}
		}
	}
	*c, *r, *k = nc, nr, nk
	return cnt
}

// drawRoadList is Draw_Road_List_ (0xb6255): the roads that are on, back to
// front, sides in the order 3, 2, 0, 1.
func (g *Game) drawRoadList() error {
	cs := &g.cscr.surf
	order := [4]int{3, 2, 0, 1}
	for n := 0; n < 0x31; n++ {
		c, r := g.roadBackToFront(n)
		for _, k := range order {
			if *cs.roadOn(c, r, k) == 0 {
				continue
			}
			p, err := g.cached(cStringAt(g.exe, colroadsLBX), int(cs.roadPic(c, r, k)))
			if err != nil {
				return err
			}
			if err := g.e.Draw(0, 0, p); err != nil {
				return err
			}
		}
	}
	return nil
}

// clearVeggieDrawFlags is Clear_Veggie_Draw_Flags_ (0xb6932).
func (g *Game) clearVeggieDrawFlags() {
	for c := range g.cscr.surf.veggies {
		for r := range g.cscr.surf.veggies[c] {
			g.cscr.surf.veggies[c][r].on = [2]int16{}
		}
	}
}

// screenColony is the colony of the planet _screen_data points at.
func (g *Game) screenColony() int {
	s := &g.state
	return int(s.Planets[s.Stars[g.screenData[0]].Planets[g.screenData[1]]].Colony)
}

// buildVeggieListBasedOnBldgList is Build_Veggie_List_Based_On_Bldg_List_
// (0xb6977): plants on empty cells away from roads.
func (g *Game) buildVeggieListBasedOnBldgList() error {
	s, cs := &g.state, &g.cscr.surf
	g.clearVeggieDrawFlags()
	for c := 0; c < 6; c++ {
		for r := 0; r < 6; r++ {
			if cs.bldgs[c][r] != 0 {
				continue
			}
			roads := int16(g.nRoadsAroundCR(c, r))
			nb := int16(g.nBldgs(g.screenColony()))
			if int32(s.Random.Random(7))-2 >= int32(roads) {
				if roads != 0 || s.Random.Random(uint32(int32(nb)+2)) == 0 {
					continue
				}
			}
			if *cs.roadOn(c, r, 2) != 0 || *cs.roadOn(c, r, 3) != 0 {
				continue
			}
			v := &cs.veggies[c][r]
			for k := 0; k < 2; k++ {
				v.on[k] = int16(b2i(s.Random.Random(3) == 1))
				if v.on[k] == 0 {
					continue
				}
				v.pic[k] = g.pickRandomVeggieAnimEntryForColonyCR(g.screenColony(), c, r)
				p, err := g.cached(cStringAt(g.exe, colveggieLBX), int(v.pic[k]))
				if err != nil {
					return err
				}
				w, h := int32(int16(p.Width)), int32(int16(p.Height))
				x := g.bldgCoordsToCenteredScreenCoord(c, r, 0)
				v.x[k] = int16(int32(int16(x)) + int32(s.Random.Random(uint32(w))) - w/2)
				y := g.bldgCoordsToCenteredScreenCoord(c, r, 1)
				v.y[k] = int16(int32(int16(y)) + int32(s.Random.Random(uint32(h))) - h/4)
			}
		}
	}
	return nil
}

// pickRandomVeggieAnimEntryForColonyCR is
// Pick_Random_Veggie_Anim_Entry_For_Colony_CR_ (0xb6647): smaller plants
// towards the front.
func (g *Game) pickRandomVeggieAnimEntryForColonyCR(ci, c, r int) int16 {
	k := int32(int16(g.state.Random.Random(8) - 1))
	t := g.veggieTypeForColony(ci)
	k -= int32(c+r) / 2
	if k <= 0 {
		k = 0
	}
	return int16(t*8 + k)
}

// veggieTypeForColony is Veggie_Type_For_Colony_ (0xb66d0): a plant set by
// the planet's climate.
func (g *Game) veggieTypeForColony(ci int) int32 {
	s := &g.state
	rnd := func(n uint32) uint32 { return s.Random.Random(n) }
	switch s.Planets[s.Colonies[ci].Planet].Climate {
	case 0:
		if rnd(2) == 1 {
			return 5
		}
		return 9
	case 1, 2:
		if rnd(2) == 1 {
			return 6
		}
		return 4
	case 3:
		return 8
	case 5:
		return [4]int32{7, 9, 0xb, 4}[rnd(4)-1]
	case 6:
		return 0xa
	case 7:
		return 1
	case 8, 9:
		return [4]int32{0xc, 3, 0, 2}[rnd(4)-1]
	}
	return 4
}

// crToXY is CR_To_XY_ (0xbc5d8): corner (c, r) of the surface grid, x (0)
// or y (1).
func (g *Game) crToXY(c, r, xy int) int16 {
	return g.exeI16(uint32(bldgCornerXY + c*0x38 + r*8 + xy*4))
}

// bldgCoordsToCenteredScreenCoord is Bldg_Coords_To_Centered_Screen_Coord_
// (0xbc866): the centre of cell (c, r).
func (g *Game) bldgCoordsToCenteredScreenCoord(c, r, xy int) int32 {
	a := int32(g.exe.u32(uint32(bldgCenterXY + c*0x38 + r*8 + xy*4)))
	b := int32(g.exe.u32(uint32(bldgCenterXY + 8 + (c+1)*0x38 + r*8 + xy*4)))
	return (a + b) / 2
}

// bldgCoordsToEffectiveFrame is Bldg_Coords_To_Effective_Frame_ (0xbc8a6):
// the building pictures run in a zigzag over the cells.
func bldgCoordsToEffectiveFrame(c, r int) int {
	if r%2 == 0 {
		return r*6 + c
	}
	return r*6 + 5 - c
}

// cacheLoadBldg is Cache_Load_Bldg_ (0xaf6dc): building id's picture for
// cell (c, r) from BLDG<n>.LBX.
func (g *Game) cacheLoadBldg(c, r int, id int32) (*gfx.Picture, error) {
	k := int32(int16(id)) - 1
	name := cSprintf(g.eStr(bldgLBXFmt), int(k/10))
	return g.cached(name, int(int16(bldgCoordsToEffectiveFrame(c, r)))+int(k%10)*0x24)
}

// drawVeggieCR is Draw_Veggie_CR_ (0xb6b95): the plants of cell (c, r); with
// glass set they are drawn twice, first through the glass tables.
func (g *Game) drawVeggieCR(c, r int, glass bool) error {
	e, cs := g.e, &g.cscr.surf
	v := &cs.veggies[c][r]
	for u := int16(b2i(glass)); u >= 0; u-- {
		for k := 0; k < 2; k++ {
			if v.on[k] == 0 {
				continue
			}
			e.V.SetWindow(0, 0, 0x27f, 0x1df)
			e.V.ClipOn()
			p, err := g.cached(cStringAt(g.exe, colveggieLBX), int(v.pic[k]))
			if err != nil {
				return err
			}
			x, y := int(v.x[k])-int(int16(p.Width))/2, int(v.y[k])-int(int16(p.Height))
			if glass && u == 1 {
				err = e.Draw(x, y, p)
			} else {
				err = e.DrawNoGlass(x, y, p)
			}
			if err != nil {
				return err
			}
			e.V.ClipOff()
		}
	}
	return nil
}
