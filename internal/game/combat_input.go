package game

import (
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/engine"
)

// The human's input in Do_Combat_Turn_ (0x42f7f) with the screen: one pass
// of its loop for a ship the human commands. Spec:
// docs/screens/combat-screen.md.

// combatHumanPass is one Get_Input of a human ship and what follows; it
// returns the value Next_Ship_ left (1 when the side is done).
func (g *Game) combatHumanPass(p int16) int16 {
	e := g.e
	s := &g.state
	tc := g.tc
	sc := tc.scr
	done := int16(0)
	cur := tc.cur
	in := sc.doneField
	if g.isCombatShipDead(cur) == 0 {
		var err error
		sc.humanInput = true
		in, err = e.GetInput()
		sc.humanInput = false
		if err != nil {
			panic(err)
		}
	}
	mountsOn := func() {
		C := g.tsp(tc.cur)
		for k := 0; k < 8; k++ {
			if C.u8(tw(k, twOn)) == 0 {
				C.set8(tw(k, twOn), 1)
			}
		}
	}
	for m := int16(0); m < nMissiles; m++ {
		if in == sc.missileField[m] && uint8(g.msl(m).i16(tmOwner)) != g.tsp(tc.cur).u8(tsOwner) {
			C := g.tsp(tc.cur)
			M := g.msl(m)
			g.tacSend(tacFire, m, 1, 0)
			g.fireShip(tc.cur, m, M.i16(tmX), M.i16(tmY), 1)
			_ = C
			g.must2(g.addTacticalCombatFields())
			g.assignCombatGrids(tc.cur)
			mountsOn()
			g.tacWaitIf()
			in = 0
		}
	}
	toggle := func(f *int16) { *f = 1 - *f }
	switch in {
	case sc.mslImpactBtn:
		toggle(&tc.mslImpact)
		tc.savedMslWarning = tc.mslImpact
		if tc.netFlag != 0 {
			g.moxSendMessage(g.netIDOf(int(tc.netDest)), 0x3eb, binary.LittleEndian.AppendUint16(nil, uint16(tc.mslImpact)))
		}
		in = 0
	case sc.speedx2Btn:
		toggle(&tc.speedx2)
		in = 0
	case sc.gridBtn:
		toggle(&sc.gridFlag)
		in = 0
		g.must2(g.refreshCombatScreenFull())
	case sc.legalBoxesBtn:
		toggle(&sc.legalBoxes)
		in = 0
		g.must2(g.refreshCombatScreenFull())
	case sc.shieldsBtn:
		toggle(&tc.shieldsFlag)
		in = 0
	}
	if in == sc.superFastBtn {
		tc.superFast = 1
		if int16(g.tsp(tc.cur).u8(tsOwner)) == tc.att {
			tc.autoCombat = 1 - tc.autoAtt
			tc.autoAtt = tc.autoCombat
		} else {
			tc.autoCombat = 1 - tc.autoDef
			tc.autoDef = tc.autoCombat
		}
		g.must2(g.drawSuperFastScreen())
	}
	if in == sc.selfDestBtn {
		if tc.cur == 0 {
			g.combatMessage(0x38)
			g.must2(g.refreshCombatScreenFull())
		} else {
			msg, err := g.kenText(0x39)
			g.must2(err)
			e.RefreshPalette()
			ok, err := g.confirmationBox(msg)
			g.must2(err)
			if ok {
				tc.orderInfo = 1 - tc.orderInfo
				g.must2(g.moveOptionsBackground())
				g.tacSend(tacSelfDestruct, 0, 0, 0)
				C := g.tsp(tc.cur)
				C.set16(tsDriveCur, 0)
				drive := C.u8(tsDrive)
				C.set8(tsDrive, 1)
				dead := g.isCombatShipDead(tc.cur)
				g.destroyShip(tc.cur, int16(C.u8(tsX))*0x14, int16(C.u8(tsY))*0x14, dead)
				g.tsp(tc.cur).set8(tsDrive, drive)
				tc.order[tc.curPtr] = -1
				done = g.nextShip()
				g.tacWaitIf()
			} else {
				g.must2(g.refreshCombatScreenFull())
			}
		}
		in = 0
	}
	if in == sc.mslWarningBtn {
		g.tacSend(tacMslWarning, 0, 0, 0)
		tc.mslWarning = (tc.mslWarning + 1) % 3
		g.tacWaitIf()
		g.must2(g.addTacticalCombatFields())
		g.assignCombatGrids(tc.cur)
		in = 0
	}
	if in == sc.autoField {
		g.tacSend(tacAutoField, 0, 0, 0)
		if int16(g.tsp(tc.cur).u8(tsOwner)) == tc.att {
			tc.autoCombat = 1 - tc.autoAtt
			tc.autoAtt = tc.autoCombat
		} else {
			tc.autoCombat = 1 - tc.autoDef
			tc.autoDef = tc.autoCombat
		}
		if tc.autoCombat == 0 {
			tc.superFast = 0
		}
		e.ClearFields()
		in = 0
		f, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, g.cmbtKey(cmbtKeyNone), 0)
		g.must2(err)
		sc.autoField = f
		g.tacWaitIf()
	}
	if in == sc.weaponsField || in == sc.specialsField {
		sc.lastShipStatus = -1
		g.setCombatHelpFields()
	}
	if in == sc.waitField {
		g.tacSend(tacWait, 0, 0, 0)
		done = g.nextShip()
		g.tacWaitIf()
		in = 0
	}
	for k := 0; k < 8; k++ {
		if in != sc.weaponBtn[k] {
			continue
		}
		if sc.statusDisplay == 0 {
			g.tacSend(tacWeapon, int16(k), 0, 0)
			C := g.tsp(tc.cur)
			v := int8(C.u8(tw(k, twOn))) - 1
			if v < -1 {
				v = 1
			}
			C.set8(tw(k, twOn), uint8(v))
			if v != 1 && C.i16(tw(k, twWeapon)) == 0x27 {
				C.set8(0x42, 0xff)
			}
			g.tacWaitIf()
			sc.lastShipStatus = -1
		}
		in = 0
	}
	if in == sc.infoField {
		tc.orderInfo = 1 - tc.orderInfo
		in = 0
		g.must2(g.moveOptionsBackground())
	}
	if in == sc.scanField {
		tc.board = 0
		g.assignCombatGrids(tc.cur)
		in = 0
	}
	if in == sc.fleeField {
		C := g.tsp(tc.cur)
		if C.u8(0x4b) == 0 {
			msg := 0
			switch {
			case g.enemyDissipatorExists(int16(C.u8(tsOwner))):
				msg = 0x3a
			case C.u8(tsDrive) == 0:
				msg = 0x3b
			case C.i16(tsSpeed) < 1:
				msg = 0x4f
			}
			if msg != 0 {
				g.combatMessage(msg)
				g.must2(g.refreshCombatScreenFull())
			} else {
				g.tacSend(tacFlee, 0, 0, 0)
				C.set8(0x4b, 1)
				tc.order[tc.curPtr] = -1
				done = g.nextShip()
				g.tacWaitIf()
			}
		}
		in = 0
	}
	if in == sc.boardField {
		tc.skan = 0
		if !g.alreadyBoarded(tc.cur) {
			g.must2(g.addTacticalCombatFields())
			g.assignCombatGrids(tc.cur)
		} else {
			g.combatMessage(0x3c)
			g.must2(g.refreshCombatScreenFull())
			tc.board = 0
		}
		in = 0
	}
	if in == sc.mapField {
		in = 0
		g.must2(g.snapCenterCombatScreen(int16(sc.mapX*2/3), int16(sc.mapY*2/3)))
	}
	if px, py := e.Pointer(); (px < 5 && tc.curX > 0) || (px > 0x27b && tc.curX < 0x31) || (py < 5 && tc.curY > 0) || (py > 0x1db && tc.curY < 0x32) {
		g.must2(g.scrollCombatScreen())
	}
	if in == sc.centerBtn {
		C := g.tsp(tc.cur)
		g.must2(g.snapCenterCombatScreen(int16(C.u8(tsX)), int16(C.u8(tsY))))
	}
	if in == -sc.spaceField {
		x, y := int16(sc.xpos/2)+tc.curX, int16(sc.ypos/2)+tc.curY
		g.tacSend(tacTurnToFace, x, y, 0)
		g.turnToFace(tc.cur, x, y)
		g.tacWaitIf()
		in = 0
	}
	if in == sc.endOfTurnBtn {
		g.tacSend(tacEndOfTurn, tc.curX, tc.curY, 0)
		if tc.initOrder == 0 {
			done = 1
		} else {
			tc.sideEnd[g.tsp(tc.cur).u8(tsOwner)] = 1
			done = g.nextShip()
		}
		g.tacWaitIf()
		in = 0
	}
	if in == sc.spaceField {
		g.combatGridClick()
		return done
	}
	if in == sc.doneField {
		g.tacSend(tacDone, 0, 0, 0)
		g.tsp(tc.cur)[0x48] |= 4
		tc.order[tc.curPtr] = -1
		done = g.nextShip()
		g.tacWaitIf()
		e.SetInputDelay(3)
	}
	_ = s
	return done
}

// combatGridClick is the click on a square: move there, take another own
// ship, fire at an enemy, or scan or board.
func (g *Game) combatGridClick() int16 {
	e := g.e
	tc := g.tc
	sc := tc.scr
	cur := tc.cur
	C := g.tsp(cur)
	img := e.PointerImage()
	var x, y int16
	if img < 4 || img > 0xb || C.u8(tsSize) == 0 {
		x = int16(sc.xpos/2) + tc.curX
		y = int16(sc.ypos/2) + tc.curY
	} else {
		a, b := sc.xpos/2, sc.ypos
		if C.u8(tsSize) < 3 {
			a, b = (sc.xpos+1)/2, sc.ypos+1
		}
		x = int16(a) + tc.curX - 1
		y = int16(b/2) + tc.curY - 1
	}
	v := tc.grid[x][y]
	done := int16(0)
	if tc.board+tc.skan < 1 {
		if v == gridLegalBox {
			owner := C.u8(tsOwner)
			g.tacSend(tacMove, x, y, 0)
			g.moveCost(cur, x, y)
			if C.i16(0x3d) < 0 {
				C.set16(0x3d, 0)
			}
			t1 := g.usedTeleporter(cur)
			cost := g.moveShip(cur, x, y)
			if t1 == g.usedTeleporter(cur) {
				C.add16(0x3d, -cost)
			}
			g.assignCombatGrids(cur)
			g.must2(g.refreshCombatScreenFull())
			if g.shipDoneCheck() || g.tsp(cur).u8(tsOwner) != owner {
				tc.order[tc.curPtr] = -1
				done = g.nextShip()
			}
			g.tacWaitIf()
			return done
		}
		if v < 0 || v >= tc.n || g.tsp(v).u8(tsGone) != 0 {
			return 0
		}
		T := g.tsp(v)
		if T.u8(tsOwner) == C.u8(tsOwner) {
			if tc.initOrder == 0 {
				g.tacSend(tacSetCur, v, 0, 0)
				g.setCurShipTo(v)
				g.tacWaitIf()
			}
			return 0
		}
		if !g.shipInStasis(v) {
			if T.u8(tsCloak) != 4 {
				g.tacSend(tacFire, v, 0, 0)
				g.fireShip(cur, v, int16(T.u8(tsX))*0x14, int16(T.u8(tsY))*0x14, 0)
				g.must2(g.addTacticalCombatFields())
				g.assignCombatGrids(tc.cur)
				C = g.tsp(tc.cur)
				for k := 0; k < 8; k++ {
					if C.u8(tw(k, twOn)) == 0 {
						C.set8(tw(k, twOn), 1)
					}
				}
				g.tacWaitIf()
				return 0
			}
			g.combatMessage(0x50)
		} else {
			msg, err := g.kenText(0x3d)
			g.must2(err)
			e.SetPaletteGradient(0x90, 0x20, 0, 0, 0, 0, 0, 0)
			e.RefreshPalette()
			ok, err := g.confirmationBox(msg)
			g.must2(err)
			if ok {
				g.tacSend(tacStasisOff, v, 0, 0)
				g.removeStasisBeam(v)
				g.tacWaitIf()
			}
		}
		g.must2(g.refreshCombatScreenFull())
		return 0
	}
	switch {
	case tc.skan != 0 && v >= 0 && v < tc.n && g.tsp(v).u8(tsGone) == 0:
		g.must2(g.displayCombatViewShip(v))
	case tc.board != 0 && v >= 0 && v < tc.n && g.tsp(v).u8(tsGone) == 0:
		g.initiateBoardingAction(tc.cur, v)
	}
	return 0
}

// combatMessage is "Ken_Get_Text_Message_(n); Refresh_Palette; Message_Box_".
func (g *Game) combatMessage(n int) {
	msg, err := g.kenText(n)
	g.must2(err)
	g.e.RefreshPalette()
	g.must2(g.messageBox(msg))
}

// shipDoneCheck is Ship_Done_Check_ (0x4590d): no moves and no shots left.
func (g *Game) shipDoneCheck() bool {
	C := g.tsp(g.tc.cur)
	r := C.i16(0x3d) < 1
	for k := 0; k < 8; k++ {
		if C.u8(tw(k, twLeft)) != 0 {
			r = false
		}
	}
	return r
}

// setCurShipTo is Set_Cur_Ship_To_ (0x45507).
func (g *Game) setCurShipTo(i int16) {
	tc := g.tc
	if i == 0 && g.doesCombatPlanetHaveDefenses() < 1 {
		return
	}
	tc.curPtr = -1
	for k := int16(0); k < tc.n && tc.order[k] != -10; k++ {
		if tc.order[k] == i {
			tc.curPtr = k
		}
	}
	if tc.curPtr == -1 {
		for k := int16(0); k < tc.n && tc.curPtr == -1; k++ {
			if tc.order[k] == -1 {
				tc.order[k] = i
				tc.curPtr = k
			}
		}
	}
	tc.cur = i
	if tc.scr != nil {
		g.must2(g.refreshCombatScreenFull())
	}
}

// drawSuperFastScreen is Draw_Super_Fast_Screen_ (0x2c555).
func (g *Game) drawSuperFastScreen() error {
	e := g.e
	tc := g.tc
	sc := tc.scr
	tc.superFast = 0
	e.V.SetPageOff()
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
	if err := g.drawMainCombatScreen(); err != nil {
		return err
	}
	if err := e.DrawVisibleFields(); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	if err := e.TogglePages(); err != nil {
		return err
	}
	tc.superFast = 1
	return nil
}

// moveOptionsBackground is Move_Options_Background_ (0x34c42): the
// options panel slides in or out.
func (g *Game) moveOptionsBackground() error {
	e := g.e
	tc := g.tc
	sc := tc.scr
	v := e.V
	var y, end, step int
	if tc.orderInfo == 0 {
		y, end, step = 0x1bd, 0x12a, -7
		tc.orderInfo = 1
		v.SetPageBack()
		if err := g.fullDrawCombatScreen(); err != nil {
			return err
		}
		if err := g.drawMainCombatScreen(); err != nil {
			return err
		}
		if err := g.drawFakeCombatButtons(); err != nil {
			return err
		}
		v.CopyBackToOff()
		if err := e.TogglePages(); err != nil {
			return err
		}
		tc.orderInfo = 0
	} else {
		v.SetPageBack()
		if err := g.fullDrawCombatScreen(); err != nil {
			return err
		}
		if err := g.drawMainCombatScreen(); err != nil {
			return err
		}
		if err := g.drawFakeCombatButtons(); err != nil {
			return err
		}
		v.CopyBackToOff()
		v.SetPageOff()
		y, end, step = 0x12a, 0x1bd, 7
		if err := g.drawCached(0xf8, 0x12a, 9); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	e.DisableDrawAllFields()
	for y != end {
		y += step
		if (step < 0 && y < end) || (step > 0 && end < y) {
			y = end
		}
		v.SetPageOff()
		g.setAnimationFrame(sc.weaponsBtn, int(1-sc.statusDisplay))
		if err := e.Draw(0x72, 0x169, sc.weaponsBtn); err != nil {
			return err
		}
		g.setAnimationFrame(sc.specialsBtn, int(sc.statusDisplay))
		if err := e.Draw(0xbb, 0x169, sc.specialsBtn); err != nil {
			return err
		}
		v.SetWindow(0, 0, 0x27f, 0x1df)
		v.ClipOn()
		if err := g.drawCached(0xf8, y, 9); err != nil {
			return err
		}
		for k, f := range []int16{tc.mslImpact, tc.speedx2, sc.legalBoxes, tc.shieldsFlag, sc.gridFlag} {
			g.setAnimationFrame(sc.option, int(f))
			if err := e.Draw(0x10b, y+0x58+0x10*k, sc.option); err != nil {
				return err
			}
		}
		v.ClipOff()
		v.ResetWindow()
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	if err := g.addTacticalCombatFields(); err != nil {
		return err
	}
	g.assignCombatGrids(tc.cur)
	e.EnableDrawAllFields()
	return g.refreshCombatScreenFull()
}

var _ = engine.MouseArrow

// combatScreenPass is one pass of Do_Combat_Turn_'s ship loop with the
// screen: the computer plays its ship, or the human gives one input; then
// the screen is redrawn.
func (g *Game) combatScreenPass(p int16) int16 {
	e := g.e
	s := &g.state
	tc := g.tc
	e.MarkTime()
	moves := g.tsp(tc.cur).i16(0x3d)
	g.fullDrawScreen = false
	owner := g.tsp(tc.cur).u8(tsOwner)
	computer := (owner > 7 || s.Players[owner].Objective != 100 || tc.autoCombat == 1) && tc.takeControl == 0
	if computer && g.tacRemote() {
		computer = false // 0x4331c: the other machine moves its ships
	}
	done := int16(0)
	if computer {
		g.tacticalNetAutoSend()
		g.doAutoShipTurn()
		if g.fullDrawScreen {
			g.must2(g.addTacticalCombatFields())
			g.assignCombatGrids(tc.cur)
			g.must2(g.refreshCombatScreenFull())
		}
		tc.order[tc.curPtr] = -1
		done = g.nextShip()
		g.tacWaitIf()
	} else {
		tc.cur = tc.order[tc.curPtr]
		if g.shipInStasis(tc.cur) {
			tc.order[tc.curPtr] = -1
			done = g.nextShip()
		} else if tc.initOrder != 0 || int16(g.tsp(tc.cur).u8(tsOwner)) == p {
			if g.tacRemote() {
				done = g.netTacticalReplay(p)
			} else {
				done = g.combatHumanPass(p)
			}
		}
		if done == 0 {
			if !g.fullDrawScreen && moves == g.tsp(tc.cur).i16(0x3d) {
				e.V.SetPageOff()
				g.must2(g.drawMainCombatScreen())
				g.must2(e.TogglePages())
			} else {
				g.must2(g.addTacticalCombatFields())
				g.assignCombatGrids(tc.cur)
				g.must2(g.refreshCombatScreenFull())
			}
		}
	}
	g.must2(e.ReleaseTime(1))
	return done
}

// displayCombatViewShip is Display_Combat_View_Ship_ (0x33c05): the frame
// behind, then Detailed_View_Ship_ on a copy of record i (Antares and
// colonies with drive -666), and the screen back.
func (g *Game) displayCombatViewShip(i int16) error {
	e := g.e
	tc := g.tc
	sc := tc.scr
	rec := viewShipFromRecord(g.tsp(i))
	sc.scanned = i
	if i == 0 {
		rec.driveCur = shipViewOffMinus666
	}
	e.V.SetPageOff()
	sc.lastShipStatus = 1
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
	if err := g.drawMainCombatScreen(); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	if err := e.TogglePages(); err != nil {
		return err
	}
	g.bqSetMouseList(0x17cf00, 1)
	for x := range tc.grid {
		for y := range tc.grid[x] {
			tc.grid[x][y] = -1
		}
	}
	sc.viewing = true
	err = g.detailedViewShipRec(-1, &rec)
	sc.viewing = false
	if err != nil {
		return err
	}
	if err := g.addTacticalCombatFields(); err != nil {
		return err
	}
	g.assignCombatGrids(tc.cur)
	if err := g.refreshCombatScreenFull(); err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawMainCombatScreen, 1)
	return nil
}
