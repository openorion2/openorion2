package game

import "time"

// Scrolling the battlefield at the screen edge (COMBINIT.C
// Scroll_Combat_Screen_, CMBTDRW1.C Scroll_Draw_Combat_Screen_). Spec:
// docs/screens/combat-screen.md.

// combatScrollTicks is the port's least time of a scroll step, in ticks.
const combatScrollTicks = 1

// scrollCombatScreen is Scroll_Combat_Screen_ (0x49a41): while the pointer
// is within 5 pixels of an edge the view moves, 1, then 2, then 3 squares a
// frame. The star layers move against the view, unlike Snap_Center's.
func (g *Game) scrollCombatScreen() error {
	e := g.e
	tc := g.tc
	sc := tc.scr
	n := int16(0)
	for {
		start := e.Ticks()
		px, py := e.Pointer()
		if px >= 5 && px <= 0x27b && py >= 5 && py <= 0x1db {
			break
		}
		step := int16(1)
		ox, oy := tc.curX, tc.curY
		x, y := ox, oy
		n++
		if n > 2 {
			step = 2
		}
		if n > 4 {
			step = 3
		}
		if px < 5 {
			x = tc.curX - step
		}
		if px > 0x27b {
			x = tc.curX + step
		}
		if py < 5 {
			y = tc.curY - step
		}
		if py > 0x1db {
			y = tc.curY + step
		}
		x = min(max(x, 0), 0x31)
		y = min(max(y, 0), 0x32)
		sign := func(d int16) int16 {
			switch {
			case d < 0:
				return 1
			case d > 0:
				return -1
			}
			return 0
		}
		dx, dy := sign(ox-x), sign(oy-y)
		if x != tc.curX {
			sc.nearX -= dx * 0x50
			sc.midX -= dx * 0x28
			sc.farX -= int16(int32(dx) * 0x14 / 2)
		}
		if y != tc.curY {
			sc.nearY -= dy * 0x50
			sc.midY -= dy * 0x28
			sc.farY -= int16(int32(dy) * 0x14 / 2)
		}
		for _, v := range []struct {
			p *int16
			m int16
		}{{&sc.nearX, 0x280}, {&sc.nearY, 0x1e0}, {&sc.midX, 0x280}, {&sc.midY, 0x1e0}, {&sc.farX, 0x280}, {&sc.farY, 0x1e0}} {
			if *v.p < 0 {
				*v.p += v.m
			}
			*v.p %= v.m
		}
		tc.curX, tc.curY = x, y
		e.V.SetPageOff()
		if err := g.scrollDrawCombatScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		// port-only: a step a timer tick (55 ms) at most. The original
		// moves a step a frame, as fast as the machine draws one: DOS
		// machines took about that, today's draw at 60 frames a second and
		// the battlefield flew by.
		for e.Ticks()-start < combatScrollTicks {
			if err := e.Sys.Sleep(5 * time.Millisecond); err != nil {
				return err
			}
		}
	}
	g.assignCombatGrids(tc.cur)
	if err := g.addTacticalCombatFields(); err != nil {
		return err
	}
	if err := g.refreshCombatScreenFull(); err != nil {
		return err
	}
	sc.lastShipStatus = -1
	return nil
}

// scrollDrawCombatScreen is Scroll_Draw_Combat_Screen_ (0x33543): the
// view while scrolling, onto the current page.
func (g *Game) scrollDrawCombatScreen() error {
	e := g.e
	tc := g.tc
	sc := tc.scr
	v := e.V
	v.SetWindow(0, 0, 0x27f, 0x15e)
	v.ClipOn()
	if tc.nebula == 0 {
		v.Fill(0, 0, 0x27f, 0x15e, 0)
	}
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
	for _, l := range []struct{ n, x, y int }{{far, int(sc.farX), int(sc.farY)}, {0x2f, int(sc.midX), int(sc.midY)}, {0x30, int(sc.nearX), int(sc.nearY)}} {
		if err := layer(l.n, l.x, l.y); err != nil {
			return err
		}
	}
	v.ClipOff()
	v.ResetWindow()
	if sc.gridFlag == 1 {
		for x := 0; x < 0x280; x += 0x14 {
			v.Line(x, 0, x, 0x15e, 4)
		}
		for y := 0; y < 0x168; y += 0x14 {
			v.Line(0, y, 0x280, y, 4)
		}
	}
	for i := int16(0); i < tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsGone) == 0 && i != sc.dontDrawShip && t.u8(0x42) != 0xff && int16(t.u8(0x42)) != sc.dontDrawShip {
			if err := g.drawContinuingTractor(i); err != nil {
				return err
			}
		}
	}
	v.SetWindow(0, 0, 0x27f, 0x15e)
	v.ClipOn()
	for i := int16(0); i < tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsGone) == 0 {
			if err := g.drawShip(i, (int(t.u8(tsX))-int(tc.curX))*0x14, (int(t.u8(tsY))-int(tc.curY))*0x14); err != nil {
				return err
			}
		}
	}
	if err := g.drawMissiles(); err != nil {
		return err
	}
	v.ClipOff()
	v.ResetWindow()
	if tc.mslWarning == 1 {
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
	if sc.lastShipStatus != 0x29a {
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
	}
	if tc.orderInfo == 0 {
		if err := g.drawFakeCombatButtons(); err != nil {
			return err
		}
		if err := g.drawCached(0xf8, 0x12a, 9); err != nil {
			return err
		}
		return g.drawOptionPanelLights()
	}
	return nil
}
