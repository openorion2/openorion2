package game

// The combat screen's animations (CMBTMOV1.C, CMBTSPEC.C, BEAMS.C): the
// drawing the logic does while ships move, turn, fire and blow up. Spec:
// docs/screens/combat-screen.md.

// combatDrawing reports whether the screen animates (it does not super
// fast or without a screen).
func (g *Game) combatDrawing() bool {
	tc := g.tc
	return tc.scr != nil && tc.onScreen == 1 && tc.superFast == 0
}

// combatMoveFPS is how many frames a second a ship flies at (2 pixels a
// frame, 10 frames a square; 12 pixels with the speed button). Move_Ship_
// has no timer: the original flew as fast as the machine drew. Port-only,
// to tune.
const combatMoveFPS = 120

// moveShipStart is the start of Move_Ship_'s drawing: the screen without
// the moving ship (and its chasing missiles) onto both pages.
func (g *Game) moveShipStart(i int16) {
	tc := g.tc
	sc := tc.scr
	sc.dontDrawShip = i
	sc.savedShields = tc.shieldsFlag
	tc.shieldsFlag = 0
	if tc.superFast == 0 {
		g.startShipEngineSFX() // 0x3eea2
	}
	if !g.combatDrawing() {
		g.e.DisableDrawAllFields()
		return
	}
	e := g.e
	e.V.SetPageOff()
	g.must2(g.fullDrawCombatScreen())
	if sc.statusDisplay == 0 {
		g.must2(g.drawWeaponStatusDisplay(-1, -1))
	} else {
		g.must2(g.drawSpecialStatusDisplay())
	}
	g.interlacedFill(0x191, 0x16a, 0x1e9, 0x176, 0, 199)
	g.interlacedFill(0x191, 0x17b, 0x1e8, 0x1d3, 0, 199)
	g.must2(g.drawInternalsDisplay(0x191, 0x17b))
	sc.lastShipStatus = 0x29a
	sc.activeMissiles = 0
	g.must2(g.drawMainCombatScreen())
	sc.activeMissiles = 1
	g.must2(e.DrawVisibleFields())
	e.V.CopyOffToBack()
	e.DisableDrawAllFields()
}

// moveShipFrame is one step of Move_Ship_'s drawing: the ship at pixel
// (px, py) and the missiles chasing it.
func (g *Game) moveShipFrame(i, px, py int16, tractors []int16) {
	if !g.combatDrawing() {
		return
	}
	e := g.e
	tc := g.tc
	sc := tc.scr
	v := e.V
	v.SetWindow(0, 0, 0x27f, 0x15e)
	v.ClipOn()
	v.SetPageOff()
	if len(tractors) > 0 {
		// The beams go under the screen Draw_Main_Combat_Screen_ draws next.
		for _, j := range tractors {
			g.drawMovingTractor(j, px, py)
		}
		g.must2(g.drawMainCombatScreen())
	}
	g.must2(g.drawShip(i, int(px-tc.curX*0x14), int(py-tc.curY*0x14)))
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		if M.i16(tmWeapon) > 0 && M.i16(tmTarget) == i {
			g.must2(g.drawMissile(m, int(M.i16(tmX)-tc.curX*0x14), int(M.i16(tmY)-tc.curY*0x14), 0))
		}
	}
	e.PaceNextToggle(combatMoveFPS)
	g.must2(e.TogglePages())
	if sc.shipFrame++; sc.shipFrame == 32000 {
		sc.shipFrame = 0
	}
	v.ResetWindow()
	v.ClipOff()
}

// moveShipEnd is the end of Move_Ship_'s drawing.
func (g *Game) moveShipEnd() {
	tc := g.tc
	sc := tc.scr
	sc.dontDrawShip = -1
	tc.shieldsFlag = sc.savedShields
	g.e.EnableDrawAllFields()
	if tc.superFast == 0 {
		g.endShipEngineSFX() // 0x3f53e
	}
}

// startShipEngineSFX is Start_Ship_Engine_SFX_ (0xadcff): sound 0x1f looped
// unless it already plays (0x182760).
func (g *Game) startShipEngineSFX() {
	sc := g.tc.scr
	if sc.engineSound != 0 {
		return
	}
	sc.engineSound = g.moxPlaySound(0x1f)
	if g.e != nil {
		g.e.LoopSound(sc.engineSound, -1)
	}
}

// endShipEngineSFX is End_Ship_Engine_SFX_ (0xadd24).
func (g *Game) endShipEngineSFX() {
	sc := g.tc.scr
	g.fadeoutSound(sc.engineSound, 0xa)
	sc.engineSound = 0
}

// rotateShipFrame is a step of Rotate_Ship_'s drawing: the screen, the ship
// at its new facing at pixel (px, py), its chasing missiles.
func (g *Game) rotateShipFrame(i, px, py int16) {
	e := g.e
	tc := g.tc
	v := e.V
	v.SetPageOff()
	for j := int16(0); j < tc.n; j++ {
		if J := g.tsp(j); J.u8(tsGone) == 0 && J.u8(0x42) != 0xff {
			g.must2(g.drawContinuingTractor(j))
		}
	}
	g.must2(g.drawMainCombatScreen())
	v.SetWindow(0, 0, 0x27f, 0x15e)
	v.ClipOn()
	g.must2(g.drawShip(i, int(px-tc.curX*0x14), int(py-tc.curY*0x14)))
	v.ClipOff()
	v.ResetWindow()
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		if M.i16(tmWeapon) > 0 && M.i16(tmTarget) == i {
			g.must2(g.drawMissile(m, int(M.i16(tmX)-tc.curX*0x14), int(M.i16(tmY)-tc.curY*0x14), 0))
		}
	}
	g.must2(e.TogglePages())
	g.must2(e.ReleaseTime(1))
}
