package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// Ship stacks and icons on the galaxy map. Spec:
// docs/screens/main-screen-ships.md.

const maxNodes = 500

// shipNode is one 5-byte _ship_node record.
type shipNode struct {
	ship, next int16
	selected   uint8
}

// shipIcon is one 12-byte _ship_icon record.
type shipIcon struct {
	field, node, location, slot, x, y int16
}

// shipStacks holds the globals of SHIPSTAK.C and SHIPS.C.
type shipStacks struct {
	node       [maxNodes]shipNode
	nextFree   int16
	start      [maxNodes]int16
	count      int16
	icon       [572]shipIcon
	iconCount  int16
	typeKeys   [1000]int16 // Sort_Ships_In_Stack_'s stack array; stale keys survive between calls
	iconWidth  [4]uint8
	iconHeight [4]uint8
}

var (
	shipLineGreen = [8]byte{0x6e, 0x6f, 0x72, 0x74, 0x74, 0x72, 0x6f, 0x6e}
	shipLineRed   = [8]byte{0x49, 0x4a, 0x4d, 0x4f, 0x4f, 0x4d, 0x4a, 0x49}
)

func testBit(b []byte, bit int) bool { return b[bit>>3]>>(bit&7)&1 != 0 }

// rangeXY is Range_: the game's distance estimate.
func rangeXY(x1, y1, x2, y2 int16) int16 {
	dx, dy := x2-x1, y2-y1
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dy < dx {
		return dx + dy/2
	}
	return dy + dx/2
}

// shipHasSpecial is Ship_Has_Special_: fitted and not damaged.
func (s *State) shipHasSpecial(k, bit int) bool {
	sh := &s.Ships[k]
	return testBit(sh.Design.Specials[:], bit) && !testBit(sh.DamagedSpecials[:], bit)
}

// findShipStacks is Find_Ship_Stacks_ (0x5d41e).
func (g *Game) findShipStacks() {
	s, st := &g.state, &g.stacks
	n := int(s.NumShips)
	excl := make([]bool, n)
	st.count, st.nextFree = 0, 0
	for i := 0; i < n; i++ {
		if int8(s.Ships[i].Status) > 2 {
			excl[i] = true
		}
	}
	for i := 0; i < n; i++ {
		if excl[i] {
			continue
		}
		sh := &s.Ships[i]
		found := false
		for j := int16(0); j < st.count && !found; j++ {
			h := &s.Ships[st.node[st.start[j]].ship]
			if sh.Location == h.Location && sh.X == h.X && sh.Y == h.Y && sh.Owner == h.Owner {
				last := st.start[j]
				for k := last; k != -1; k = st.node[k].next {
					last = k
				}
				st.node[last].next = st.nextFree
				st.node[st.nextFree] = shipNode{int16(i), -1, 0}
				st.nextFree++
				found = true
			}
		}
		if !found {
			st.start[st.count] = st.nextFree
			st.node[st.nextFree] = shipNode{int16(i), -1, 0}
			st.nextFree++
			st.count++
		}
	}
	if s.Players[s.PlayerNum].Picks[save.PickOmniscient] == 0 {
		g.removeNonDetectedShips()
	}
	for j := int16(0); j < st.count; j++ {
		g.sortShipsInStack(st.start[j])
	}
}

// removeNonDetectedShips is Remove_Non_Detected_Ships_ (0x5d953) with the
// stealth patch at 0xf4d5c / 0x169357.
func (g *Game) removeNonDetectedShips() {
	s, st := &g.state, &g.stacks
	pn := int(s.PlayerNum)
	me := &s.Players[pn]
	base := 2
	if me.AppStatus[0x1cc-0x117] == 3 {
		base = 4
	}
	if me.AppStatus[0x18b-0x117] == 3 {
		base = 6
	}
	if me.AppStatus[0x1b7-0x117] == 3 {
		base = 9
	}
	for r := int16(0); r < st.count; r++ {
		head := &s.Ships[st.node[st.start[r]].ship]
		if int(head.Owner) == pn {
			continue
		}
		best := int16(-300)
		for c := 0; c < int(s.NumColonies); c++ {
			col := &s.Colonies[c]
			if int(col.Owner) != pn {
				continue
			}
			star := &s.Stars[s.Planets[col.Planet].Star]
			d := (int(rangeXY(head.X, head.Y, star.X, star.Y)) + 0x1d) / 0x1e
			if int16(d) == 0 {
				best = 1000
				continue
			}
			v := base
			if col.Buildings[0x15e-0x136] != 0 {
				v = base + 2
			}
			if col.Buildings[0x13e-0x136] != 0 {
				v += 4
			}
			if col.Buildings[0x15f-0x136] != 0 {
				v += 6
			}
			v -= d
			if best < int16(v) {
				best = int16(v)
			}
		}
		for k := 0; k < int(s.NumShips); k++ {
			o := &s.Ships[k]
			if int(o.Owner) != pn || int8(o.Status) >= 3 {
				continue
			}
			d := (int(rangeXY(head.X, head.Y, o.X, o.Y)) + 0x1d) / 0x1e
			if int16(d) == 0 {
				best = 1000
				continue
			}
			v := base - d
			if s.shipHasSpecial(k, 5) {
				v += 2
			}
			if best < int16(v) {
				best = int16(v)
			}
		}
		prev := int16(-1)
		for n := st.start[r]; n != -1; n = st.node[n].next {
			sh := &s.Ships[st.node[n].ship]
			if sh.Owner > 8 && sh.Design.FTLSpeed == 0 {
				// A bribed Dragon departs (location 1000 + star): the visited bit
				// lies in free DOS memory past the last block, zero (drag2m).
				if loc := int(sh.Location); loc < 0 || loc >= len(s.Stars) || !s.playerHasVisited(pn, loc) {
					if loc < 0 {
						g.logOnce("Remove_Non_Detected_Ships_ reads a star below the array; taken as not visited")
					}
					st.deleteNode(r, prev, n)
				}
				continue
			}
			var v int16
			switch {
			case sh.Owner == 8:
				v = -5
			case sh.Owner < 8 && (s.Players[sh.Owner].Picks[save.PickStealthyShips] == 1 ||
				sh.Design.Specials[3]&0x80 != 0 || sh.Design.Specials[0]&0x40 != 0 || sh.Design.Specials[2]&0x80 != 0):
				v = 500
			case sh.Status == 1:
				v = -int16(sh.Design.Size)
			}
			for _, b := range []int{31, 6, 23} {
				if testBit(sh.DamagedSpecials[:], b) {
					v += 500
				}
			}
			if best < v {
				st.deleteNode(r, prev, n)
			} else {
				prev = n
			}
		}
		if st.start[r] == -1 {
			copy(st.start[r:st.count], st.start[r+1:st.count])
			r--
			st.count--
		}
	}
}

// deleteNode is Delete_Ship_Node_.
func (st *shipStacks) deleteNode(stack, prev, n int16) {
	if prev == -1 {
		st.start[stack] = st.node[n].next
	} else {
		st.node[prev].next = st.node[n].next
	}
}

// sortShipsInStack is Sort_Ships_In_Stack_ with Build_Ship_Type_Array_:
// combat ships by size and picture, then colony ships, transports and
// outposts. Keys of other design types are left from earlier calls.
func (g *Game) sortShipsInStack(head int16) {
	s, st := &g.state, &g.stacks
	type entry struct{ ship, key int16 }
	var arr []entry
	n := head
	for i := 0; ; i++ {
		ship := st.node[n].ship
		if ship >= 0 {
			k := len(arr)
			key := st.typeKeys[2*k+1]
			d := &s.Ships[ship].Design
			switch {
			case d.Type == 0:
				key = int16(int8(d.Picture)) + int16(d.Size)*0x32
			case d.Type < 2:
				key = -1
			case d.Type < 3:
				key = -2
			case d.Type == 4:
				key = -3
			}
			st.typeKeys[2*k], st.typeKeys[2*k+1] = ship, key
			arr = append(arr, entry{ship, key})
		}
		n = st.node[n].next
		if !(n > 0 && n < maxNodes) {
			break
		}
	}
	if len(arr) > 1 {
		clib.Qsort(arr, 4, func(a, b *entry) int { return int(b.key) - int(a.key) })
		n := head
		for _, e := range arr {
			st.node[n].ship = e.ship
			n = st.node[n].next
			if n < 0 {
				break
			}
		}
		for k, e := range arr {
			st.typeKeys[2*k], st.typeKeys[2*k+1] = e.ship, e.key
		}
	}
}

// shipStackOwner is Ship_Stack_Owner_; the original exits with "Memory
// Corruption" outside 0..15.
func (g *Game) shipStackOwner(n int16) int {
	ship := g.stacks.node[n].ship
	if ship < 0 || ship >= maxShips || g.state.Ships[ship].Owner < 0 || g.state.Ships[ship].Owner > 15 {
		panic(fmt.Sprintf("Memory Corruption: ship node %d ship %d", n, ship))
	}
	return int(g.state.Ships[ship].Owner)
}

// xyToStarID is XY_To_Star_Id_: it scans one star past NUM_STARS.
func (g *Game) xyToStarID(x, y int16) int {
	s := &g.state
	for i := 0; i <= int(s.NumStars) && i < len(s.Stars); i++ {
		if s.Stars[i].X == x && s.Stars[i].Y == y {
			return i
		}
	}
	return -1
}

// buildShipIcons is Build_Ship_Icons_ (0xa0305).
func (g *Game) buildShipIcons() {
	s, st := &g.state, &g.stacks
	g.findShipStacks()
	type starSlot struct {
		owner       int8
		stack, icon int16
	}
	var slots [72]starSlot
	for i := range slots {
		slots[i] = starSlot{-1, -1, -1}
	}
	var nextSlot [288]uint8
	for i := range nextSlot {
		nextSlot[i] = 2
	}
	for i := range st.icon {
		st.icon[i] = shipIcon{-1, -1, -1, -1, -1, -1}
	}
	st.iconCount = 0
	sorted := false
	add := func(stk, loc, slot int16) {
		st.icon[st.iconCount] = shipIcon{-1, stk, loc, slot, -1, -1}
		st.iconCount++
	}
	bump := func(i int) uint8 {
		v := nextSlot[i]
		nextSlot[i] = min(v+1, 3)
		return v
	}
	for i := int16(0); i < st.count; i++ {
		stk := st.start[i]
		ship := &s.Ships[st.node[stk].ship]
		owner := g.shipStackOwner(stk)
		xy := g.xyToStarID(ship.X, ship.Y)
		loc := ship.Location
		switch {
		case ship.Status == 1:
			add(stk, loc, 5)
			sorted = true
		case loc >= 0 && loc < 500:
			if int(loc) >= len(slots) {
				g.Log("Build_Ship_Icons_: stack %d at location %d", i, loc)
				continue
			}
			sl := &slots[loc]
			switch {
			case sl.owner == -1:
				*sl = starSlot{int8(owner), stk, st.iconCount}
				add(stk, loc, 0)
			case int(sl.owner) != int(s.PlayerNum) && owner == int(s.PlayerNum):
				st.icon[sl.icon].slot = int16(bump(int(loc)))
				*sl = starSlot{int8(s.PlayerNum), stk, st.iconCount}
				add(stk, loc, 0)
			default:
				add(stk, loc, int16(bump(int(loc))))
			}
		case xy == int(loc)-1000:
			// Bug: the search for a departing icon never matches, so the
			// original writes to icon -1 and adds nothing; the stack is
			// invisible.
			g.Log("Build_Ship_Icons_: invisible departing stack %d", i)
		case loc >= 1000 && xy > -1:
			add(stk, loc, 1)
		case int(loc) == xy && xy >= 0:
			add(stk, int16(xy), int16(bump(xy)))
		case int(loc) == xy:
			// Location -1 off every star: next_slot[-1] is the last byte of
			// the star table, 0xff after its memset.
			add(stk, int16(xy), 0xff)
		default:
			add(stk, loc, 5)
			sorted = true
		}
	}
	if st.iconCount > 0 && sorted {
		clib.Qsort(st.icon[:st.iconCount], 12, func(a, b *shipIcon) int { return int(b.slot) - int(a.slot) })
	}
}

// shipIconPicture is Get_Ship_Icon_Pict_Seg_; hl only picks the pointer
// table, the cached picture is the same.
func (g *Game) shipIconPicture(owner int) (*gfxPicture, error) {
	z := g.mapScaleToZoomLevel()
	p := 1
	switch z {
	case 3:
		p = 0
	case 1:
		p = 2
	case 0:
		p = 3
	}
	switch {
	case owner < 8:
		banner := int(g.state.Players[owner&7].Color)
		return g.cached(buffer0LBX, banner*4+0xcd+p)
	case owner == 8:
		return g.cached(buffer0LBX, p+0xed)
	case owner < 15:
		return g.cached(buffer0LBX, (owner-9)*4+0xf1+z)
	}
	return nil, nil
}

// shipIconDimensions is Get_Ship_Icon_Dimensions_: banner 0's picture i.
func (g *Game) shipIconDimensions(i int) (int, int, error) {
	p, err := g.cached(buffer0LBX, 0xcd+i)
	if err != nil {
		return 0, 0, err
	}
	g.stacks.iconWidth[i], g.stacks.iconHeight[i] = uint8(p.Width), uint8(p.Height)
	return int(uint8(p.Width)), int(uint8(p.Height)), nil
}

// starAnimDimension is Get_Star_Anim_Dimension_.
func (g *Game) starAnimDimension(i int) int16 {
	return starFieldsDim[g.mapScaleToZoomLevel()+int(g.state.Stars[i].Size)]
}

// shipIconCoords is Get_Ship_Icon_Coords_ (0xa0a5c).
func (g *Game) shipIconCoords(i int) (int16, int16, error) {
	s, st := &g.state, &g.stacks
	ic := &st.icon[i]
	ship := &s.Ships[st.node[ic.node].ship]
	if ship.Status == 1 {
		z := g.mapScaleToZoomLevel()
		w, h, err := g.shipIconDimensions(z)
		if err != nil {
			return 0, 0, err
		}
		return int16(g.scaled(ship.X-s.CurMapX) + 0x15 - w/2), int16(g.scaled(ship.Y-s.CurMapY) + 0x15 - h/2), nil
	}
	star := g.xyToStarID(ship.X, ship.Y)
	pic, err := g.shipIconPicture(int(ship.Owner))
	if err != nil {
		return 0, 0, err
	}
	w := int16(pic.Width)
	z := int16(g.mapScaleToZoomLevel())
	d := int16(int8(1 - z))
	var sx, sy, a int16
	if star >= 0 && star < len(s.Stars) {
		sx, sy = g.starDrawCoords(star)
		a = g.starAnimDimension(star)
	}
	sx -= a / 2
	sy -= a / 2
	step := 11 - z
	x0, y0 := d+(a+sx-w/2), sy+d-1
	x1, y1 := sx-w/2-d+2, sy+d-2
	switch ic.slot {
	case 0:
		return x0, y0, nil
	case 1:
		if ship.Owner >= 10 && (s.CurMapScale == 10 || s.CurMapScale == 15) {
			x1 -= 2
		}
		return x1, y1, nil
	case 2, 3, 4:
		k := min(ic.slot-2, 2)
		return x1 - 1, y1 + step*(k+1), nil
	case 5:
		return int16(g.scaled(ship.X)), int16(g.scaled(ship.Y)), nil
	}
	return ic.x, ic.y, nil
}

// setShipIconXYs is Set_Ship_Icon_XYs_.
func (g *Game) setShipIconXYs() {
	st := &g.stacks
	for i := int(st.iconCount) - 1; i >= 0; i-- {
		x, y, err := g.shipIconCoords(i)
		if err != nil {
			g.Log("ship icon %d: %v", i, err)
			continue
		}
		st.icon[i].x, st.icon[i].y = x, y
	}
}

// drawShipIcons is Draw_Ship_Icons_ (0xa070f).
func (g *Game) drawShipIcons(all bool) error {
	st := &g.stacks
	sel := -1
	if g.moveableBoxSelected(2) {
		sel = g.shipIconIDFromNodePtr(g.nodePtrFromShipStack(int(g.fb.stack)))
	}
	for i := int(st.iconCount) - 1; i >= 0; i-- {
		x, y, err := g.shipIconCoords(i)
		if err != nil {
			return err
		}
		pic, err := g.shipIconPicture(g.shipStackOwner(st.icon[i].node))
		if err != nil {
			return err
		}
		st.icon[i].x, st.icon[i].y = x, y
		if (i == sel || i == int(g.ms.mainScreenShipIconScanned)) && !all {
			continue
		}
		if pic == nil {
			continue
		}
		pic.CurrentFrame = 0
		if err := g.e.Draw(int(x), int(y), pic); err != nil {
			return err
		}
	}
	return nil
}

// drawScannedShipSeg is Draw_Scanned_Ship_Seg_: the icon under the mouse,
// drawn without a frame reset.
func (g *Game) drawScannedShipSeg() {
	st := &g.stacks
	i := int(g.ms.mainScreenShipIconScanned)
	if i < 0 || i >= int(st.iconCount) {
		return
	}
	pic, err := g.shipIconPicture(g.shipStackOwner(st.icon[i].node))
	if err != nil || pic == nil {
		return
	}
	g.e.Draw(int(st.icon[i].x), int(st.icon[i].y), pic)
}

// absoluteLocation is Absolute_Location_.
func absoluteLocation(l int16) int16 {
	switch {
	case l >= 500 && l < 1000:
		return l - 500
	case l >= 1000 && l < 0x430:
		return l - 1000
	}
	return l
}

// iconShip is Ship_Icon_Ship_Id_.
func (g *Game) iconShip(i int) int {
	n := g.stacks.icon[i].node
	if n >= 0 && n < maxNodes {
		return int(g.stacks.node[n].ship)
	}
	return 0
}

// enemyHeadingTowardOurColony is Enemy_Ship_Heading_Toward_Our_Colony_:
// any outpost at the destination counts, whoever owns it.
func (g *Game) enemyHeadingTowardOurColony(stk int16) bool {
	s := &g.state
	sh := &s.Ships[g.stacks.node[stk].ship]
	if int(sh.Owner) == int(s.PlayerNum) || sh.Status == 0 {
		return false
	}
	d := int(absoluteLocation(sh.Location))
	if s.Stars[d].ColonyOwners>>s.PlayerNum&1 != 0 {
		return true
	}
	out := false
	s.starColonies(d, func(c *save.Colony) {
		if c.IsOutpost != 0 {
			out = true
		}
	})
	return out
}

// doShipDestinationLines is Do_Ship_Destination_Lines_ (0xa080d).
func (g *Game) doShipDestinationLines() {
	s, st := &g.state, &g.stacks
	for i := int(st.iconCount) - 1; i >= 0; i-- {
		k := g.iconShip(i)
		sh := &s.Ships[k]
		stk := st.icon[i].node
		mine := int(sh.Owner) == int(s.PlayerNum)
		own := mine && absoluteLocation(sh.Location) != s.NumStars
		// community patch 1.51 (0xa08a6 -> 0x40452b): with Enemy Moves on
		// (settings +3 == 1) every moving enemy fleet shown gets its line.
		threat := !mine && sh.Status != 0 && (g.settings[0x03] == 1 || g.enemyHeadingTowardOurColony(stk))
		boxed := g.moveableBoxSelected(2) && g.nodePtrFromShipStack(int(g.fb.stack)) == stk
		if !(own || threat || boxed) || sh.Location < 500 || sh.Location >= 1500 {
			continue
		}
		dest := absoluteLocation(sh.Location)
		if dest == s.NumStars {
			continue
		}
		sx, sy := g.starDrawCoords(int(dest))
		ix, iy, err := g.shipIconCoords(i)
		if err != nil {
			g.Log("ship line %d: %v", i, err)
			continue
		}
		colors := shipLineGreen
		if threat || !mine {
			colors = shipLineRed
		}
		g.drawShipDestinationLine(sx, sy, ix, iy, colors)
		if sh.Location >= 500 && sh.Location < 1000 {
			g.printEtaOnShipIcon(stk, g.mapScaleToZoomLevel(), ix, iy)
		}
	}
}

// drawShipDestinationLine is Draw_Ship_Destination_Line_.
func (g *Game) drawShipDestinationLine(sx, sy, ix, iy int16, colors [8]byte) {
	p := max(3-g.mapScaleToZoomLevel(), 0)
	x, y := int(ix), int(iy)
	if g.currentScreen == ScreenMain || g.currentScreen == ScreenReports || g.currentScreen == 0xc {
		w, h, err := g.shipIconDimensions(p)
		if err != nil {
			return
		}
		x += w / 2
		y += h / 2
	} else {
		x += 4
		y += 4
	}
	g.e.V.DirectionalMultiColoredLine(int(int16(x)), int(int16(y)), int(sx), int(sy), colors, int(g.ms.multiColoredLineStart))
}

// printEtaOnShipIcon is Print_Eta_On_Ship_Icon_.
func (g *Game) printEtaOnShipIcon(stk int16, z int, x, y int16) {
	owner := g.shipStackOwner(stk)
	sh := &g.state.Ships[g.stacks.node[stk].ship]
	pic, err := g.shipIconPicture(owner)
	if err != nil || pic == nil {
		return
	}
	g.setPlayerFontStyle(owner, false)
	t := g.e.Text()
	c := t.Current
	style := 0
	if z < 2 {
		style = 1
	}
	t.SetStyle(style, c)
	t.PrintRight(int(x)+pic.Width, int(y)+pic.Height, fmt.Sprintf(g.hMessage(0x133), sh.ETA))
}
