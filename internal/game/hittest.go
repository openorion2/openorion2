package game

// Mouse hit tests of the galaxy map: Check_Mouse_XY_Fields_ (hover, every
// Draw_Main_Screen_), Check_Stars_XY_ / Check_Ships_XY_ (clicks on the
// grid field) and Get_Mouse_Field_ which reads both.

// mapMouse holds 0x199ec9..0x199ecf and _mouse_xy.
type mapMouse struct {
	shipClick  int16 // _mouse_xy
	starClick  int16 // 0x199ecb
	shipScan   int16 // 0x199ec9
	starScan   int16 // 0x199ecd
	clickKind  uint8 // 0x199ecf: 1 left, 2 right
	movingStak bool  // _moving_a_stack
}

// starHitRadius is the hit radius by star size (12, 10, 8).
var starHitRadius = [3]int{0xc, 10, 8}

func (g *Game) starHit(i, x, y int) bool {
	sx, sy := g.starDrawCoords(i)
	r := starHitRadius[g.state.Stars[i].Size%3]
	dx, dy := int(sx)-x, int(sy)-y
	return dx*dx+dy*dy <= r*r
}

func (g *Game) shipIconHit(i, x, y int) bool {
	st := &g.stacks
	pic, err := g.shipIconPicture(g.shipStackOwner(st.icon[i].node))
	if err != nil || pic == nil {
		return false
	}
	ix, iy := int(st.icon[i].x), int(st.icon[i].y)
	return ix <= x && x <= ix+pic.Width && iy <= y && y <= iy+pic.Height
}

// checkMouseXYFields is Check_Mouse_XY_Fields_ (0x82ac1).
func (g *Game) checkMouseXYFields() {
	m, s := &g.mm, &g.state
	x, y := g.e.Pointer()
	inBox := false
	m.starScan, m.shipScan, m.clickKind = -1, -1, 0
	var b *[4]int16
	switch {
	case g.moveableBoxSelected(0):
		bx := &s.MoveableBoxes[0]
		b = &[4]int16{bx.X, bx.Y, bx.X + bx.Width, bx.Y + bx.Height}
	case g.moveableBoxSelected(2):
		bx := &s.MoveableBoxes[2]
		b = &[4]int16{bx.X, bx.Y, bx.X + bx.Width, bx.Y + bx.Height}
	}
	if b != nil && int(b[0]) < x && x < int(b[2]) && int(b[1]) < y && y < int(b[3]) {
		inBox = true
	}
	if x > 0x15 && x < 0x20f && y > 0x15 && y < 0x1a5 && !inBox {
		g.scanStarsXY(x, y)
		g.scanShipsXY(x, y)
	}
}

// scanStarColors are Scan_Stars_XY_'s font colours (the last four from
// 0x81cb0).
func (g *Game) scanStarColors() [8]byte {
	c := [8]byte{0, 0x0a, 0x0c, 0x0e}
	copy(c[4:], g.exe.bytes(0x81cb0, 4))
	return c
}

// scanStarsXY is Scan_Stars_XY_ (0x897cc): the star under the mouse.
func (g *Game) scanStarsXY(x, y int) {
	m, s := &g.mm, &g.state
	m.movingStak = false
	for i := 0; i < int(s.NumStars); i++ {
		if !g.starHit(i, x, y) || m.shipScan != -1 {
			continue
		}
		if g.someIconSelected() {
			g.fb.moveInfo = g.shipsTryToMoveTo(g.fleetBoxSelectedShipIDs(0xff), i)
			m.movingStak = true
		}
		m.shipScan, m.clickKind, m.starScan = -1, 0, int16(i)
		t := g.e.Text()
		t.SetStyle(2, g.scanStarColors())
		t.SetSpacing(1)
		return
	}
}

// scanShipsXY is Scan_Ships_XY_ (0x896a7): the ship icon under the mouse.
func (g *Game) scanShipsXY(x, y int) {
	m, st := &g.mm, &g.stacks
	for i := 0; i < int(st.iconCount); i++ {
		if !g.shipIconHit(i, x, y) {
			continue
		}
		if !m.movingStak || m.starScan == -1 {
			m.clickKind, m.shipScan, m.starScan = 0, int16(i), -1
		}
		return
	}
}

// checkStarsXY is Check_Stars_XY_ (0x82cb7): a click on a star.
func (g *Game) checkStarsXY(x, y int, kind uint8) {
	m, s := &g.mm, &g.state
	m.starClick, m.shipClick = -1, -1
	for i := 0; i < int(s.NumStars); i++ {
		if g.starHit(i, x, y) {
			m.starClick, m.clickKind = int16(i), kind
			g.moxPlaySound(0x29)
			return
		}
	}
}

// checkShipsXY is Check_Ships_XY_ (0x82be8): a click on a ship icon.
func (g *Game) checkShipsXY(x, y int, kind uint8) {
	m, st := &g.mm, &g.stacks
	m.starClick, m.shipClick = -1, -1
	for i := 0; i < int(st.iconCount); i++ {
		if g.shipIconHit(i, x, y) {
			m.shipClick, m.clickKind = int16(i), kind
			g.moxPlaySound(0x29)
			return
		}
	}
}

// getMouseField is Get_Mouse_Field_ (0x857f8): kind 0 asks for stars, 1
// for ship icons. It returns 0 (left click) or 1 (right click) with the
// clicked index, 4 with the hovered index, or -1.
func (g *Game) getMouseField(kind int) (r int, hovered, clicked int16) {
	m := &g.mm
	hovered, clicked = -1, -1
	clickedRes := func(v int16) (int, int16, int16) {
		res := 1
		if m.clickKind == 1 {
			res = 0
		}
		return res, -1, v
	}
	var v int16
	if !g.moveableBoxSelected(2) {
		if m.shipClick >= 0 && kind == 1 {
			return clickedRes(m.shipClick)
		}
		if m.starClick >= 0 && kind == 0 {
			return clickedRes(m.starClick)
		}
		if m.shipScan < 0 || kind != 1 {
			if m.starScan < 0 || kind != 0 {
				return -1, -1, -1
			}
			v = m.starScan
		} else {
			v = m.shipScan
		}
	} else {
		if m.starClick >= 0 && kind == 0 {
			return clickedRes(m.starClick)
		}
		if m.shipClick >= 0 && kind == 1 {
			return clickedRes(m.shipClick)
		}
		if m.starScan < 0 || kind != 0 {
			if m.shipScan < 0 || kind != 1 {
				return -1, -1, -1
			}
			v = m.shipScan
		} else {
			v = m.starScan
		}
	}
	return 4, v, -1
}

// lastColonySelected is Last_Colony_Selected_ (0x7819e): the colony at the
// orbit the player last picked at this star if it is the player's, else the
// one Set_Colony_Auto_Select_For_Star_ picks.
func (g *Game) lastColonySelected(star, p int) int {
	s := &g.state
	S := &s.Stars[star]
	pl := -1
	if o := int8(S.LastOrbit[p]); o > -1 && o < 5 {
		pl = int(S.Planets[o])
	}
	c := -1
	if pl > -1 && pl < int(s.PlanetCount) {
		c = int(s.Planets[pl].Colony)
	}
	if c != -1 && int16(s.Colonies[c].Owner) == int16(p) {
		return c
	}
	pl = g.setColonyAutoSelectForStar(star, p)
	if pl > -1 && pl < int(s.PlanetCount) {
		return int(s.Planets[pl].Colony)
	}
	return -1
}

// setColonyAutoSelectForStar is Set_Colony_Auto_Select_For_Star_
// (0x7862b): the first orbit with a colony (not an outpost) of p becomes
// the star's last pick for p; its planet, or -1.
func (g *Game) setColonyAutoSelectForStar(star, p int) int {
	s := &g.state
	S := &s.Stars[star]
	out, col, found := -1, -1, false
	for o := 0; o < 5 && !found; o++ {
		pl := int(S.Planets[o])
		if pl <= -1 {
			continue
		}
		out = pl
		c := int(s.Planets[pl].Colony)
		if c <= -1 {
			continue
		}
		col = c
		if C := &s.Colonies[c]; C.IsOutpost == 0 && int16(C.Owner) == int16(p) {
			found = true
			S.LastOrbit[p] = int8(o)
		} else {
			col = -1
		}
	}
	if col == -1 {
		out = -1
	}
	return out
}
