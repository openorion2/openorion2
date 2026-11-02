package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// Drawing of the combat screen (CMBTDRW1.C). Spec:
// docs/screens/combat-screen.md.

// combatCached is Buffer_Reload of a COMBAT.LBX picture.
func (g *Game) combatCached(n int) (*gfx.Picture, error) {
	return g.cached(g.cmbtLBX(cmbtCombatLBX), n)
}

// drawCached draws COMBAT.LBX picture n at (x, y).
func (g *Game) drawCached(x, y, n int) error {
	p, err := g.combatCached(n)
	if err != nil {
		return err
	}
	return g.e.Draw(x, y, p)
}

// shipControlledByComputer is Ship_Controlled_By_Computer_ (0x42abf).
func (g *Game) shipControlledByComputer(i int16) bool {
	s := &g.state
	tc := g.tc
	o := g.tsp(i).u8(tsOwner)
	return (o >= 8 || s.Players[o].Objective != 100 || tc.autoCombat == 1) && tc.takeControl == 0
}

// combatShipDone is Combat_Ship_Done_ (0x3883a).
func (g *Game) combatShipDone(i int16) bool { return g.tsp(i)[0x48]&4 != 0 }

// onCombatScreen is On_Combat_Screen_ (0x3d259): the square is in the view.
func (g *Game) onCombatScreen(x, y int16) bool {
	tc := g.tc
	return x >= tc.curX && int32(x) < int32(tc.curX)+combatScreenW && y >= tc.curY && int32(y) < int32(tc.curY)+combatScreenH
}

// planetSpan is the squares the planet takes past its corner, by size (the
// tables of Ship_On_Screen_ and Ship_Completely_On_Screen_).
func (g *Game) planetSize() int {
	s := &g.state
	tc := g.tc
	if tc.def == 8 {
		return 2
	}
	return int(s.Planets[s.Colonies[tc.colony].Planet].Size)
}

// shipOnScreen is Ship_On_Screen_ (0x33b19): a corner of the ship shows.
func (g *Game) shipOnScreen(i int16) bool {
	t := g.tsp(i)
	x, y := int16(t.u8(tsX)), int16(t.u8(tsY))
	on := g.onCombatScreen(x, y)
	d := int16(-1)
	if i == 0 {
		d = [5]int16{2, 3, 5, 6, 7}[g.planetSize()]
	} else {
		switch sz := t.u8(tsSize); {
		case sz == 0 || sz > 5:
			return on
		case sz <= 2:
			d = 1
		default:
			d = 2
		}
	}
	return on || g.onCombatScreen(x+d, y+d)
}

// shipCompletelyOnScreen is Ship_Completely_On_Screen_ (0x339a7). A big
// ship only tests its far corner.
func (g *Game) shipCompletelyOnScreen(i int16) bool {
	t := g.tsp(i)
	x, y := int16(t.u8(tsX)), int16(t.u8(tsY))
	on := g.onCombatScreen(x, y)
	if !on {
		return false
	}
	if i == 0 {
		d := [5]int16{2, 3, 5, 6, 7}[g.planetSize()]
		on = on && g.onCombatScreen(x+d, y+d)
	}
	switch sz := g.tsp(i).u8(tsSize); {
	case sz == 0:
	case sz < 3:
		on = on && g.onCombatScreen(x+1, y+1)
	case sz < 6:
		on = g.onCombatScreen(x+2, y+2)
	}
	return on
}

// getDrawShipOffsets is Get_Draw_Ship_Offsets_ (0x34454).
func (g *Game) getDrawShipOffsets(i int16) (dx, dy int) {
	tc := g.tc
	if i == 0 {
		if tc.colony == -1 {
			return 9, 7
		}
		if g.planetSize() != 2 {
			return 0, 0
		}
		return 9, 7
	}
	t := g.tsp(i)
	switch (t.u8(tsSize) + 1) / 2 {
	case 0:
		dx, dy = -0x10, -0x10
	case 1:
		dx, dy = -6, -6
	case 2, 3:
		dx, dy = 4, 4
	}
	switch max(int(int16(t.u8(tsFacing))-1)/4, 0) {
	case 1:
		dx -= 2
	case 2:
		dx -= 2
		dy--
	case 3:
		dy--
	}
	return dx - 2, dy - 3
}

// combatShipPicture is the picture Draw_Ship_ and Draw_Ship_To_Bitmap_ use:
// the record's own one, or SHIPS.LBX (MONSTER.LBX) recoloured for its side.
func (g *Game) combatShipPicture(i int16) (*gfx.Picture, error) {
	s := &g.state
	tc := g.tc
	t := g.tsp(i)
	if p := g.recordPicture(i); p != nil && t.u8(tsOwner) == t.u8(tsBuilder) {
		return p, nil
	}
	var file string
	var entry int
	if b := t.u8(tsBuilder); b < 8 && t.i16(tsPicture) != 0x2c {
		file, entry = g.cmbtLBX(cmbtShipsLBX), int(s.Players[b].Color)*0x2d+int(t.i16(tsPicture))
	} else {
		file, entry = g.cmbtLBX(cmbtMonsterLBX), int(t.i16(tsPicture))
		if entry == 0x2c {
			entry = 0x19
		}
	}
	p, err := g.cached(file, entry)
	if err != nil {
		return nil, err
	}
	var tab [0x20]byte
	if int16(t.u8(tsOwner)) == tc.att {
		for k := range tab {
			tab[k] = byte(0x20 + k)
		}
		p.ReplaceColorRange(tab[:], 0x60, 0x20)
	} else {
		for k := range tab {
			tab[k] = byte(0x60 + k)
		}
		p.ReplaceColorRange(tab[:], 0x20, 0x20)
	}
	return p, nil
}

// shipFrameBitmaps puts frame of facing f (and anim offset a) into the ship
// bitmaps the way Draw_Ship_ and Draw_Ship_To_Bitmap_ do: the four facing
// quarters come from one strip, mirrored as needed.
func (g *Game) shipFrameBitmaps(p *gfx.Picture, f, per, a int, done bool) {
	sc := g.tc.scr
	frame := func(n int) {
		if !done {
			n += a
		}
		g.setAnimationFrame(p, n)
	}
	switch max((f-1)/4, 0) {
	case 0:
		frame(f * per)
		sc.drawShipBitmap.DrawPictureToBitmap(p)
	case 1:
		frame((8 - f) * per)
		sc.shipBitmap.DrawPictureToBitmap(p)
		sc.drawShipBitmap.FlipBitmap(sc.shipBitmap, 0)
	case 2:
		frame((f - 8) * per)
		sc.drawShipBitmap.DrawPictureToBitmap(p)
		sc.shipBitmap.FlipBitmap(sc.drawShipBitmap, 0)
		sc.drawShipBitmap.FlipBitmap(sc.shipBitmap, 3)
	case 3:
		frame((0x10 - f) * per)
		sc.shipBitmap.DrawPictureToBitmap(p)
		sc.drawShipBitmap.FlipBitmap(sc.shipBitmap, 3)
	}
}

// setAnimationFrame is Set_Animation_Frame: clamped to the last frame.
func (g *Game) setAnimationFrame(p *gfx.Picture, n int) {
	if n >= p.Frames {
		n = p.Frames - 1
	}
	if n < 0 {
		n = 0
	}
	p.CurrentFrame = n
}

// drawShip is Draw_Ship_ (0x30062) at screen position (x, y).
func (g *Game) drawShip(i int16, x, y int) error {
	tc := g.tc
	sc := tc.scr
	t := g.tsp(i)
	var anim int
	if t.u8(tsBuilder) < 10 {
		anim = [4]int{1, 2, 3, 2}[(int(sc.shipFrame)/2+int(i))%4]
	} else {
		anim = (int(sc.shipFrame) / 2) % 4
	}
	dx, dy := g.getDrawShipOffsets(i)
	if i == 0 {
		P := g.tsp(0)
		return g.e.Draw((int(P.u8(tsX))-int(tc.curX))*0x14+dx, (int(P.u8(tsY))-int(tc.curY))*0x14+dy, sc.planet)
	}
	p, err := g.combatShipPicture(i)
	if err != nil {
		return err
	}
	done := g.combatShipDone(i) || t.u8(tsBoarded) != 0
	g.shipFrameBitmaps(p, int(t.u8(tsFacing)), 4, anim, done)
	switch t.u8(tsCloak) {
	case 4:
		if g.tsp(tc.cur).u8(tsOwner) == uint8(g.state.PlayerNum) || g.gameType == 1 {
			sc.shipBitmap.Outline(sc.drawShipBitmap, 5, false)
			sc.shipBitmap.MaskOut(0, 0, sc.drawShipBitmap, false)
			return g.e.Draw(x+dx, y+dy, sc.shipBitmap)
		}
		return nil
	case 1, 5:
		g.plasmaDarken(sc.drawShipBitmap, (sc.shipFrame+2*i)%0x10, 3)
	}
	if t.i16(tsAbsorbed) > 0 && !g.shipInStasis(i) && !g.shipInBlackHole(i) && t.i16(0x43) == 0 {
		g.drawEnergyAbsorber(i, x, y)
	}
	if err := g.e.Draw(x+dx, y+dy, sc.drawShipBitmap); err != nil {
		return err
	}
	switch {
	case g.shipInStasis(i):
		g.drawStasisBall(i, 100, 0, 0)
	case t.i16(0x43) >= 1:
		g.drawPlasmaWeb(i, int16(x), int16(y))
	case g.shipInBlackHole(i):
		g.drawBHG(i, 0, 0)
	}
	return nil
}

// drawShipToBitmap is Draw_Ship_To_Bitmap_ (0x32b26): the ship's still
// frame into the ship bitmaps (darkening for cloaks is not ported).
func (g *Game) drawShipToBitmap(i, kind, frame int16) error {
	t := g.tsp(i)
	p, err := g.combatShipPicture(i)
	if err != nil {
		return err
	}
	g.shipFrameBitmaps(p, int(t.u8(tsFacing)), 4, 0, true)
	if kind != 0 {
		g.plasmaDarken(g.tc.scr.drawShipBitmap, frame, kind)
	}
	return nil
}

// drawLegalBoxes is Draw_Legal_Boxes_ (0x34ab6): the squares the current
// ship may move to.
func (g *Game) drawLegalBoxes() {
	tc := g.tc
	cur := tc.cur
	if cur == 0 {
		return
	}
	t := g.tsp(cur)
	if !(t.i16(0x3d) > 0 || ((g.combatShipHasSpecial(cur, 0x21) || g.combatShipHasSpecial(cur, 0x16)) && !g.usedTeleporter(cur))) {
		return
	}
	w := 0
	switch sz := t.u8(tsSize); {
	case sz == 0:
		w = 0x14
	case sz < 3:
		w = 0x28
	case sz < 6:
		w = 0x3c
	}
	c := byte(0x81)
	if tc.nebula == 1 {
		c = 0
	}
	for x := tc.curX; int32(x) < int32(tc.curX)+0x20; x++ {
		for y := tc.curY; int32(y) < int32(tc.curY)+0x12; y++ {
			if tc.grid[x][y] == gridLegalBox {
				x1, y1 := int(x-tc.curX)*0x14, int(y-tc.curY)*0x14
				g.e.V.Fill(x1, y1, w+x1, w+y1, c)
			}
		}
	}
}

// fullDrawCombatScreen is Full_Draw_Combat_Screen_ (0x2c6ba): background,
// panels, star layers, legal boxes, the current ship's status, grid, map.
func (g *Game) fullDrawCombatScreen() error {
	e := g.e
	s := &g.state
	tc := g.tc
	sc := tc.scr
	if tc.onScreen == 0 || tc.superFast != 0 {
		return nil
	}
	v := e.V
	if tc.nebula == 0 {
		v.Fill(0, 0, 0x27f, 0x1df, 0)
	}
	g.interlacedFill(0xf, 0x16a, 0x66, 0x176, 0, 199)
	g.interlacedFill(0x191, 0x16a, 0x1e9, 0x176, 0, 199)
	g.interlacedFill(0x191, 0x17b, 0x1e8, 0x1d3, 0, 199)
	if tc.cur > -1 {
		if err := g.drawInternalsDisplay(0x191, 0x17b); err != nil {
			return err
		}
	}
	if err := g.drawCached(0, 0x15f, 0); err != nil {
		return err
	}
	auto := tc.autoDef
	if s.PlayerNum == tc.att {
		auto = tc.autoAtt
	}
	if auto != 0 {
		g.setAnimationFrame(sc.autoBtn, 1)
		if err := e.Draw(0x111, 0x16c, sc.autoBtn); err != nil {
			return err
		}
	}
	v.SetWindow(0, 0, 0x27f, 0x15e)
	v.ClipOn()
	layer := func(n, lx, ly int) error {
		p, err := g.combatCached(n)
		if err != nil {
			return err
		}
		for _, d := range [4][2]int{{0, 0}, {-0x280, 0}, {-0x280, -0x1e0}, {0, -0x1e0}} {
			if err := e.Draw(lx+d[0], ly+d[1], p); err != nil {
				return err
			}
		}
		return nil
	}
	far := 0x2e
	if tc.nebula == 1 {
		far = 0x23
	}
	if err := layer(far, int(sc.farX), int(sc.farY)); err != nil {
		return err
	}
	if sc.legalBoxes == 1 && tc.cur > -1 {
		if o := g.tsp(tc.cur).u8(tsOwner); o < 8 && s.Players[o].Objective == 100 {
			g.drawLegalBoxes()
		}
	}
	if err := layer(0x2f, int(sc.midX), int(sc.midY)); err != nil {
		return err
	}
	if err := layer(0x30, int(sc.nearX), int(sc.nearY)); err != nil {
		return err
	}
	v.ClipOff()
	v.ResetWindow()
	for i := int16(0); i < tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsGone) == 0 && i != sc.dontDrawShip && t.u8(0x42) != 0xff && int16(t.u8(0x42)) != sc.dontDrawShip {
			if err := g.drawContinuingTractor(i); err != nil {
				return err
			}
		}
	}
	if tc.cur != -1 {
		t := e.Text()
		t.SetStyleShadow(2, g.combatGreen(), font.ShadowDown)
		t.SetOutlineColor(0, 0)
		if err := g.squeezePrintCentered(0x3b, 0x16a, g.combatShipName(tc.cur), 0x58); err != nil {
			return err
		}
		if err := g.drawShipStatus(0xe, 0x178, tc.cur); err != nil {
			return err
		}
		if err := g.drawGreyDamageBars(0xe, 0x178, tc.cur); err != nil {
			return err
		}
		if err := g.drawCrewLevelStars(tc.cur, 0xf, 0x17b); err != nil {
			return err
		}
	}
	if sc.gridFlag == 1 {
		for x := 0; x < 0x280; x += 0x14 {
			v.Line(x, 0, x, 0x15e, 4)
		}
		for y := 0; y < 0x168; y += 0x14 {
			v.Line(0, y, 0x280, y, 4)
		}
	}
	g.drawReducedCombatMap()
	sc.lastShipStatus = -2
	return nil
}

// combatShipName is the record's name (its first bytes).
func (g *Game) combatShipName(i int16) string {
	t := g.tsp(i)
	n := 0
	for n < len(t) && t[n] != 0 {
		n++
	}
	return string(t[:n])
}

// refreshCombatScreenFull is Refresh_Combat_Screen_Full_ (0x2c601).
func (g *Game) refreshCombatScreenFull() error {
	e := g.e
	tc := g.tc
	sc := tc.scr
	if err := g.addTacticalCombatFields(); err != nil {
		return err
	}
	g.assignCombatGrids(tc.cur)
	if tc.superFast != 0 {
		return nil
	}
	e.V.SetPageBack()
	if err := g.fullDrawCombatScreen(); err != nil {
		return err
	}
	var err error
	if sc.statusDisplay == 0 {
		err = g.drawWeaponStatusDisplay(-1, -1)
	} else {
		err = g.drawSpecialStatusDisplay()
	}
	if err != nil {
		return err
	}
	g.interlacedFill(0x191, 0x16a, 0x1e9, 0x176, 0, 199)
	g.interlacedFill(0x191, 0x17b, 0x1e8, 0x1d3, 0, 199)
	if err := g.drawInternalsDisplay(0x191, 0x17b); err != nil {
		return err
	}
	sc.lastShipStatus = 0x29a
	e.V.CopyBackToOff()
	e.V.SetPageOff()
	if err := g.drawMainCombatScreen(); err != nil {
		return err
	}
	return e.TogglePages()
}

// drawMainCombatScreen is Draw_Main_Combat_Screen_ (0x2f4ee), also the
// screen's auto function: hover box, the hovered ship's status, shields,
// ships, the current ship's cursor, missiles, the radar.
func (g *Game) drawMainCombatScreen() error {
	e := g.e
	s := &g.state
	tc := g.tc
	sc := tc.scr
	v := e.V
	sc.clockFrame++
	if sc.clockFrame > 0x12 {
		sc.clockFrame = 0xf
	}
	back := v.Current
	sc.autoBtnValue = tc.autoCombat
	if tc.onScreen == 0 || tc.superFast != 0 {
		return nil
	}
	px, py := e.Pointer()
	img := e.PointerImage()
	scan, err := e.ScanInput()
	if err != nil {
		return err
	}
	mis := int16(-1000)
	if scan >= 1 && scan <= int(sc.missileCount) {
		// The loop leaves the field number in place when it finds nothing.
		mis = int16(scan)
		k := 0
		for m := int16(0); m < nMissiles && k < int(sc.missileCount); m++ {
			M := g.msl(m)
			if M.i16(tmWeapon) > 0 && g.onCombatScreen((M.i16(tmX)-8)/0x14, (M.i16(tmY)-8)/0x14) {
				if k++; k == scan {
					mis = m
					break
				}
			}
		}
	}
	if tc.cur > -1 {
		if px/0x14 < 0x20 && py/0x14 < 0x12 {
			v.SetWindow(0, 0, 0x27f, 0x15e)
			v.ClipOn()
			C := g.tsp(tc.cur)
			off := 0
			switch sz := C.u8(tsSize); {
			case sz == 0:
			case sz < 3:
				off = 10
			default:
				off = 0x14
			}
			cx, cy := (px+0xc-off)/0x14, (py+0xc-off)/0x14
			gx, gy := int(tc.curX)+cx, int(tc.curY)+cy
			if gx >= 0 && gx < len(tc.grid) && gy >= 0 && gy < len(tc.grid[0]) && tc.grid[gx][gy] == gridLegalBox && tc.skan == 0 &&
				(g.gameType == 1 || C.u8(tsOwner) == uint8(s.PlayerNum)) && tc.orderInfo == 1 {
				w := -1
				switch sz := C.u8(tsSize); {
				case sz == 0:
					w = 0x14
				case sz < 3:
					w = 0x28
				case sz <= 5:
					w = 0x3c
				}
				if w > 0 {
					g.box(cx*0x14, cy*0x14, w, w, 1, 0xb1)
				}
			}
			v.ClipOff()
			v.ResetWindow()
			po := e.PointerOffset()
			hx, hy := (px+po)/0x14+int(tc.curX), (py+po)/0x14+int(tc.curY)
			hov := int16(-1)
			if hx >= 0 && hx < len(tc.grid) && hy >= 0 && hy < len(tc.grid[0]) {
				hov = tc.grid[hx][hy]
			}
			if err := g.drawHoverStatus(mis, hov, img, back); err != nil {
				return err
			}
		} else if sc.lastShipStatus != 0x29a {
			if err := g.clearHoverStatus(back); err != nil {
				return err
			}
		}
		if tc.shieldsFlag == 1 {
			v.SetWindow(0, 0, 0x27f, 0x15e)
			v.ClipOn()
			if err := g.drawShieldArcs(); err != nil {
				return err
			}
			v.ClipOff()
			v.ResetWindow()
		}
	}
	v.SetWindow(0, 0, 0x27f, 0x15e)
	v.ClipOn()
	for i := int16(0); i < tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsGone) == 0 && i != sc.dontDrawShip && g.shipOnScreen(i) {
			if err := g.drawShip(i, (int(t.u8(tsX))-int(tc.curX))*0x14, (int(t.u8(tsY))-int(tc.curY))*0x14); err != nil {
				return err
			}
		}
	}
	if cur := tc.cur; cur != sc.dontDrawShip && cur != 0 && g.tsp(cur).u8(tsGone) == 0 && !g.shipControlledByComputer(cur) &&
		(g.tsp(cur).u8(tsOwner) == uint8(s.PlayerNum) || g.gameType == 1) {
		C := g.tsp(cur)
		fr := (int(sc.shipFrame) / 2) % 4
		var p *gfx.Picture
		var dx, dy int
		switch sz := C.u8(tsSize); {
		case sz == 0:
			p, dx, dy = sc.shipCursor[0], -3, -3
		case sz < 3:
			p, dx, dy = sc.shipCursor[1], -5, -4
		case sz <= 5:
			p, dx, dy = sc.shipCursor[2], -8, -8
		}
		if p != nil {
			g.setAnimationFrame(p, fr)
			if err := e.Draw((int(C.u8(tsX))-int(tc.curX))*0x14+dx, (int(C.u8(tsY))-int(tc.curY))*0x14+dy, p); err != nil {
				return err
			}
		}
	}
	if err := g.drawMissiles(); err != nil {
		return err
	}
	v.ClipOff()
	v.ResetWindow()
	t := e.Text()
	t.SetStyleShadow(2, g.combatGreen(), font.ShadowDown)
	t.SetOutlineColor(0, 0)
	C := g.tsp(tc.cur)
	if tc.mslWarning < 1 || C.u8(tsOwner) > 7 || s.Players[C.u8(tsOwner)].Objective != 100 {
		if err := g.drawRadarCurrentShip(); err != nil {
			return err
		}
		g.drawReducedMapBox()
	} else if tc.mslWarning == 1 {
		n := sc.mslWarningOn.CurrentFrame
		sc.mslWarningOn.CurrentFrame = 0
		for k := 0; k < n; k++ {
			if err := e.Draw(500, 0x16b, sc.mslWarningOn); err != nil {
				return err
			}
		}
		if err := e.Draw(500, 0x16b, sc.mslWarningOn); err != nil {
			return err
		}
		if err := e.Draw(0x211, 0x197, sc.warningMissile); err != nil {
			return err
		}
	} else {
		g.drawReducedCombatMap()
		g.drawReducedMapBox()
	}
	if sc.shipFrame++; sc.shipFrame == 32000 {
		sc.shipFrame = 0
	}
	if tc.orderInfo == 0 {
		if err := g.drawFakeCombatButtons(); err != nil {
			return err
		}
		if err := g.drawCached(0xf8, 0x12a, 9); err != nil {
			return err
		}
		if err := g.drawOptionPanelLights(); err != nil {
			return err
		}
	}
	if tc.autoCombat == 1 {
		return g.drawFakeCombatButtons()
	}
	return nil
}

// drawHoverStatus is Draw_Main_Combat_Screen_'s status panel for what the
// pointer is over: a missile, a ship (drawn on the back page once), or
// nothing.
func (g *Game) drawHoverStatus(mis, hov int16, img int, back []byte) error {
	e := g.e
	tc := g.tc
	sc := tc.scr
	v := e.V
	if sc.statusDisplay != 0 {
		return g.drawSpecialStatusDisplay()
	}
	if mis >= 0 {
		if err := g.drawWeaponStatusDisplay(-1, mis); err != nil {
			return err
		}
		g.interlacedFill(0x191, 0x16a, 0x1e9, 0x176, 0, 199)
		t := e.Text()
		t.SetStyleShadow(1, g.combatGreen(), font.ShadowDown)
		t.SetOutlineColor(0, 0)
		name, err := g.weaponName(int(g.msl(mis).i16(tmWeapon)), false)
		if err != nil {
			return err
		}
		if err := g.squeezePrintCentered(0x1bd, 0x16d, name, 0x59); err != nil {
			return err
		}
		if err := g.drawMissileStatus(0x191, 0x17b, mis); err != nil {
			return err
		}
		sc.lastShipStatus = -1
		return nil
	}
	if (img == 0xc || img == 1) && hov != -1 && hov < tc.n {
		if err := g.drawWeaponStatusDisplay(hov, -1); err != nil {
			return err
		}
		if sc.lastShipStatus == hov {
			return nil
		}
		if g.tsp(hov).u8(tsGone) != 0 {
			v.SetPageBack()
			return g.clearHoverPanel(back)
		}
		v.SetPageBack()
		t := e.Text()
		t.SetStyleShadow(2, g.combatGreen(), font.ShadowDown)
		t.SetOutlineColor(0, 0)
		g.interlacedFill(0x191, 0x16a, 0x1e9, 0x176, 0, 199)
		if err := g.squeezePrintCentered(0x1bd, 0x16a, g.combatShipName(hov), 0x59); err != nil {
			return err
		}
		if err := g.drawShipStatus(400, 0x178, hov); err != nil {
			return err
		}
		if err := g.drawGreyDamageBars(400, 0x178, hov); err != nil {
			return err
		}
		if err := g.drawCrewLevelStars(hov, 0x191, 0x17b); err != nil {
			return err
		}
		if isOff(v, back) {
			v.SetPageOff()
		}
		sc.lastShipStatus = hov
		return nil
	}
	if sc.lastShipStatus != 0x29a {
		return g.clearHoverStatus(back)
	}
	return nil
}

// isOff is Get_Current_Video_Page_Flag == 1: back was the off page.
func isOff(v *engine.Video, back []byte) bool { return len(back) > 0 && &back[0] == &v.Off[0] }

// clearHoverStatus empties the hovered ship's panel on the back page.
func (g *Game) clearHoverStatus(back []byte) error {
	g.e.V.SetPageBack()
	var err error
	if g.tc.scr.statusDisplay == 0 {
		err = g.drawWeaponStatusDisplay(-1, -1)
	} else {
		err = g.drawSpecialStatusDisplay()
	}
	if err != nil {
		return err
	}
	return g.clearHoverPanel(back)
}

func (g *Game) clearHoverPanel(back []byte) error {
	v := g.e.V
	g.interlacedFill(0x191, 0x16a, 0x1e9, 0x176, 0, 199)
	g.interlacedFill(0x191, 0x17b, 0x1e8, 0x1d3, 0, 199)
	if err := g.drawInternalsDisplay(0x191, 0x17b); err != nil {
		return err
	}
	if isOff(v, back) {
		v.SetPageOff()
	}
	g.tc.scr.lastShipStatus = 0x29a
	return nil
}

// Coordinate tables of the missile groups (code segment data).
const (
	missileGroupOffset = 0x3b83d // 8 (x, y) word pairs per facing (0x20 bytes)
	torpedoGroupOffset = 0x3ba3d // 4 pairs per facing (0x10 bytes)
)

// missileType is Missile_Type_ (0x3bb50): the row of CMBTMISL.LBX pictures.
func missileType(w int16) int {
	u := uint16(w)
	switch {
	case u == 0xf, u == 0x1d:
		return 1
	case u == 0x10, u == 0x1e:
		return 2
	case u == 0x11, u == 0x1c:
		return 3
	case u == 0x12:
		return 4
	case u == 0x13:
		return 5
	case u == 0x14, u == 0x28:
		return 6
	}
	return 0
}

// pictureCount is Picture_Count_ (0x3bd2c): how many missiles of a stack
// are drawn.
func (g *Game) pictureCount(m int16) int {
	M := g.msl(m)
	n := int(M.i16(tmCount))
	switch g.wtype(M.i16(tmWeapon)) {
	case 1:
		return min(max(int(int16(n/5+1)), 1), 9)
	case 2:
		return min(max(int(int16(n/5+1)), 1), 4)
	case 4:
		if n < 9 {
			return 9
		}
		return n
	}
	return 1
}

// missileCoords is Get_Multiple_Missile_Coords_ (0x3bf72) and, for the
// other kinds, Get_Multiple_Torpedo_Coords_ (0x3bdda): where the n
// pictures of a stack sit around its point.
func (g *Game) missileCoords(m int16, n int, missile bool) (xs, ys [9]int) {
	f := uint32(g.msl(m)[tmFace])
	odd := g.state.Stardate%2 == 1
	if missile {
		pair := func(k int) (int, int) {
			a := missileGroupOffset + f*0x20 + uint32(k)*4
			return int(g.exeI16(a)), int(g.exeI16(a + 2))
		}
		put := func(slots []int, pairs []int) {
			for i, k := range pairs {
				xs[slots[i]], ys[slots[i]] = pair(k)
			}
		}
		switch n {
		case 2:
			put([]int{0, 1}, []int{1, 5})
			for k := 0; k < 2; k++ {
				xs[k], ys[k] = int(int16(xs[k])/2), int(int16(ys[k])/2)
			}
		case 3:
			if odd {
				put([]int{1, 2}, []int{3, 7})
			} else {
				put([]int{1, 2}, []int{1, 5})
			}
		case 4:
			put([]int{0, 1, 2, 3}, []int{1, 3, 5, 7})
		case 5:
			put([]int{1, 2, 3, 4}, []int{1, 3, 5, 7})
		case 6:
			put([]int{0, 1, 2, 3, 4, 5}, []int{1, 2, 3, 5, 6, 7})
		case 7:
			put([]int{1, 2, 3, 4, 5, 6}, []int{1, 2, 3, 5, 6, 7})
		case 8:
			put([]int{0, 1, 2, 3, 4, 5, 6, 7}, []int{0, 1, 2, 3, 4, 5, 6, 7})
		case 9:
			put([]int{1, 2, 3, 4, 5, 6, 7, 8}, []int{0, 1, 2, 3, 4, 5, 6, 7})
		}
		return
	}
	pair := func(k int) (int, int) {
		a := torpedoGroupOffset + f*0x10 + uint32(k)*4
		return int(g.exeI16(a)), int(g.exeI16(a + 2))
	}
	first, second := 1, 3
	if odd {
		first, second = 0, 2
	}
	switch n {
	case 2:
		xs[0], ys[0] = pair(first)
		xs[1], ys[1] = pair(second)
	case 3:
		xs[1], ys[1] = pair(first)
		xs[2], ys[2] = pair(second)
	case 4:
		for k := 0; k < 4; k++ {
			xs[k], ys[k] = pair(k)
		}
	}
	return
}

// drawMissiles is Draw_Missiles_ (0x3c834): every stack, last first.
func (g *Game) drawMissiles() error {
	tc := g.tc
	for m := int16(nMissiles - 1); m >= 0; m-- {
		M := g.msl(m)
		if err := g.drawMissile(m, int(M.i16(tmX))-int(tc.curX)*0x14, int(M.i16(tmY))-int(tc.curY)*0x14, 0); err != nil {
			return err
		}
	}
	return nil
}

// drawMissile is Draw_Missile_ (0x3c2dc): a stack's pictures at (x, y);
// all sets the full-screen window and draws off the view too.
func (g *Game) drawMissile(m int16, x, y, all int) error {
	e := g.e
	tc := g.tc
	sc := tc.scr
	M := g.msl(m)
	if M.i16(tmWeapon) <= 0 || !(g.missileInView(m) || all != 0) {
		return nil
	}
	w := M.i16(tmWeapon)
	p := sc.msl[missileType(w)*0x10+int(M[tmFace])]
	typ := g.wtype(w)
	if typ != 4 {
		if M.i16(tmTarget) == tc.cur || w == 0x13 {
			g.setAnimationFrame(p, (int(sc.shipFrame)+int(m))%3)
		} else {
			p.CurrentFrame = 0
		}
	}
	if M.i16(tmTarget) == tc.cur && sc.activeMissiles != 1 {
		return nil
	}
	if typ == 4 {
		g.drawFighter(m, x, y, all)
		return nil
	}
	n := g.pictureCount(m)
	missile := typ == 1 || w == 0x12
	xs, ys := g.missileCoords(m, n, missile)
	bottom := 0x15e
	if all == 1 {
		bottom = 0x1df
	}
	v := e.V
	v.SetWindow(0, 0, 0x27f, bottom)
	v.ClipOn()
	d := 0x17
	if missile {
		d = 0xc
	}
	for k := 0; k < n; k++ {
		if err := e.Draw(xs[k]+x-d, y-d+ys[k], p); err != nil {
			return err
		}
	}
	v.ClipOff()
	v.ResetWindow()
	return nil
}

// missileInView is Missile_On_Combat_Screen_ (0x3ce4f): the stack's square
// is in the view.
func (g *Game) missileInView(m int16) bool {
	M := g.msl(m)
	return g.onCombatScreen(int16(int32(M.i16(tmX))/0x14), int16(int32(M.i16(tmY))/0x14))
}
