package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/save"
)

// Main_Screen_ (0x86188), the galaxy screen's loop. Spec:
// docs/screens/main-screen-panel.md ("Main_Screen_ parts"),
// main-screen-background.md.

// isMainScreenPalette is Is_Main_Screen_Palette_.
func (g *Game) isMainScreenPalette() bool {
	switch g.previousScreen {
	case 0, 8, 0xb, 0xc, 0x1c, 0x1e, 0x1f, 0x23, 0x24, 0x27, 0x28, 0x2b:
		return true
	}
	return false
}

// mainScreen is Main_Screen_.
func (g *Game) mainScreen() error {
	e, m, s := g.e, &g.ms, &g.state
	if g.autopilot && g.gameType == save.GameSingle && s.Players[s.PlayerNum].Objective == save.ObjectiveHuman {
		g.makePlayerIntoAI(int(s.PlayerNum))
	}
	c := g.isMainScreenPalette()
	m.trackTime = true
	e.AssignAutoFunction(g.drawMainScreen, 1)
	g.clearSystemDisplayLoadedFlag()
	g.setMainScreenHelpList()
	for i := range s.Stars {
		s.Stars[i].TwinkleFrame, s.Stars[i].TwinkleEnd = -1, -1
	}
	m.nTwinklingStars = 0
	m.fleetShipScanned, m.galaxyMapScannedStar = -1, -1
	e.SetFieldOutlineColor(0x14)
	e.DisableCancel()
	m.skipFields = false
	m.mainScreenScannedStar = -1
	if g.gameType != save.GameHotseat && g.moveableBoxSelected(2) {
		if !g.getStackStartFromSavedShipArray() {
			g.vanishFleetMovementBox()
		}
		m.shipIconPlayerID = -1
	}
	e.V.SetRefreshStencil()
	m.drawToBack = true
	m.zoomBoxIncrement = 1
	zoomSteps := 0
	m.zoomCount = int16(g.mapScaleToZoomLevel())
	leave := false
	e.ClearFields()
	g.setMainScreenFieldsToNull()
	if err := g.addCheats(); err != nil {
		return err
	}
	if !c && g.previousScreen == ScreenReports && g.moveableBoxSelected(0) && m.atLeastOneReport {
		g.sd.skipPlanets = true
	}
	for i := 0; i < int(s.NumStars); i++ {
		s.Stars[i].Owner = int8(s.starOwner(i, int(s.PlayerNum)))
	}
	g.buildShipIcons()
	g.setShipIconXYs()
	if !m.clearFleetFields {
		if g.someIconSelected() {
			g.reloadShipSelectionStatus()
		}
	} else {
		m.firstNodePtrInStack = -1
	}
	if !m.forceFade && (c || (!m.fadeInFlag && !m.atLeastOneTacticalCombat)) {
		if g.moveableBoxSelected(0) {
			g.clearSystemDisplayLoadedFlag()
		}
		if err := e.LoadPalette(0, 0, 0xad); err != nil {
			return err
		}
		if g.previousScreen == 0x24 {
			e.V.SetPageOff()
			if err := g.drawMiniMainScreen(); err != nil {
				return err
			}
			e.V.CopyOffToBack()
			if err := e.TogglePages(); err != nil {
				return err
			}
			e.V.SetRefreshStencil()
		}
		if err := g.addMapFields(); err != nil {
			return err
		}
	} else {
		if err := g.fadeIntoMainScreen(); err != nil {
			return err
		}
		m.drawFleetScreenToBack = true
	}
	m.trackTime = false
	m.clearFleetFields = true
	if !g.renamedHomeStar[s.PlayerNum] {
		if s.Stardate == 35000 {
			e.DeactivateHelpList()
			if err := g.changeHomeStarName(); err != nil {
				return err
			}
			g.setMainScreenHelpList()
		}
		g.renamedHomeStar[s.PlayerNum] = true
	}
	m.fadeInFlag = true
	m.atLeastOneTacticalCombat = false
	// 0x86443: once a turn in a game against others, the High Council names
	// the players who cheated.
	if g.gameType != save.GameSingle && m.cheatWarnMask>>uint(s.PlayerNum)&1 != 0 {
		m.cheatWarnMask &^= 1 << uint(s.PlayerNum)
		if err := g.warnAboutCheatingPlayers(); err != nil {
			return err
		}
	}

	for {
		if leave || g.globalEsc {
			m.mainscrPalette = 0x65
			m.shipIconPlayerID = -1
			e.DeactivateAutoFunction()
			if m.clearFleetFields {
				m.firstNodePtrInStack = -1
			}
			m.etaStarID, m.stencilBoxID = -1, -1
			g.clearSystemDisplayLoadedFlag()
			m.animatedStar = -1
			e.DeactivateHelpList()
			m.alreadyFaded = false
			m.reloadedShipPalette = false
			return nil
		}
		if g.gameType == save.GameNetwork || g.gameType == save.GameModem {
			// 0x86490: requests to talk from other humans.
			if err := g.mainScreenDiplomacyRequests(); err != nil {
				return err
			}
			g.announceInGame()                         // port-only
			if err := g.netShowNotices(); err != nil { // port-only: lost players
				return err
			}
		}
		e.MarkTime()
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		if r < 0 && m.zoomIn {
			m.zoomIn = false
			e.SetMouseList(engine.MouseArrow)
			zoomSteps = 0
			e.AssignAutoFunction(g.drawMainScreen, 1)
			g.setMainScreenHelpList()
		}
		// 0x86574: an all-AI game and a human who surrendered press TURN.
		for _, auto := range []bool{g.allAIGame, g.state.Players[g.state.PlayerNum].SurrenderTo != -1} {
			if !auto {
				continue
			}
			e.V.SetPageOff()
			if err := g.drawMainScreen(); err != nil {
				return err
			}
			if err := e.TogglePages(); err != nil {
				return err
			}
			r = m.fields.next
		}
		h, err := e.ScanInput()
		if err != nil {
			return err
		}
		f := &m.fields
		if r == f.grid || r == -f.grid {
			px, py := e.Pointer()
			kind := uint8(1)
			if r < 1 {
				kind = 2
			}
			if !g.moveableBoxSelected(2) {
				g.checkShipsXY(px, py, kind)
				if g.mm.shipClick == -1 {
					g.checkStarsXY(px, py, kind)
				}
			} else {
				g.checkStarsXY(px, py, kind)
				if g.mm.starClick == -1 {
					g.checkShipsXY(px, py, kind)
				}
			}
		} else {
			g.mm.shipClick, g.mm.starClick = -1, -1
		}
		if !m.skipFields {
			res, scanned, clicked := g.getMouseField(0)
			m.mainScreenScannedStar = scanned
			iconSel := false
			if (res == 0 || res == 4) && g.moveableBoxSelected(2) {
				iconSel = g.someIconSelected()
			}
			switch {
			case res == 0 && clicked >= 0 && s.Stars[clicked].SpectralClass != save.SpectralBlackHole && g.mergingRelocations:
				// 0x866fd: every relocation goes to the star; H 0xdd says so.
				e.SetMouseList(engine.MouseArrow)
				g.mergingRelocations = false
				g.setAllStarRelocations(int(clicked), int(s.PlayerNum))
				msg := cSprintf(g.hMessage(0xdd), save.CString(s.Stars[clicked].Name[:]))
				if err := g.mainUserBox(msg); err != nil {
					return err
				}
			case res == 0 && clicked >= 0 && !iconSel:
				col := -1
				if g.settings[0x07] != 0 && s.Stars[clicked].SpectralClass != save.SpectralBlackHole {
					col = g.lastColonySelected(int(clicked), int(s.PlayerNum))
				}
				if col == -1 {
					if err := g.doSystemPopup(int(clicked)); err != nil {
						return err
					}
				} else if err := g.doColonyScreen(col, &leave); err != nil {
					return err
				}
			case iconSel && clicked >= 0 && res == 0 && s.Stars[clicked].SpectralClass != save.SpectralBlackHole:
				g.moveOrder(int(clicked))
			case iconSel && res == 4 && g.moveableBoxSelected(2):
				m.fleetShipScanned = -1
				m.etaStarID = scanned
				if s.Stars[scanned].SpectralClass == save.SpectralBlackHole {
					m.etaStarID = -1
				}
			}
			if err := g.systemDisplayInput(h, r, &leave); err != nil {
				return err
			}
			if err := g.scanMoveableBoxFields(h, r, &leave); err != nil {
				return err
			}
			res, scanned, clicked = g.getMouseField(1)
			m.mainScreenShipIconScanned = scanned
			if scanned != -1 {
				m.etaStarID = -1
			}
			if res == 0 {
				if err := g.doFleetPopup(int(clicked), true); err != nil {
					return err
				}
			}
		}
		if !m.skipFields {
			if err := g.checkFunctionKeys(r, &leave); err != nil {
				return err
			}
			if err := g.checkHotKeys(r); err != nil {
				return err
			}
		}
		switch {
		case r == f.exitGame && !m.skipFields:
			g.Log("%s", g.hMessage(0xde))
			g.currentScreen, leave = ScreenQuit, true
		case r == f.game && !m.skipFields:
			// 0x86bc9: the popup opens on its main page (_screen_data[0] may
			// still hold a colony screen's star, which it takes for a page).
			g.screenData[0] = 0
			g.currentScreen, g.returnScreen, leave = ScreenGamePopup, 0, true
		}
		if r == f.fleets && !m.skipFields {
			g.currentScreen, g.returnScreen, leave = 4, 0, true
			m.officerScreenReturnScreen = 4
		}
		if r == f.colonies && !m.skipFields {
			g.currentScreen, g.returnScreen, leave = 0x14, 0, true
		}
		if r == f.info && !m.skipFields {
			g.currentScreen, g.returnScreen, leave = 9, 0, true
		}
		if r == f.next && !m.skipFields {
			if err := g.doNextTurn(); err != nil {
				return err
			}
			leave = g.currentScreen != ScreenMain // Do_Next_Turn_ sets the flag
		}
		if r == f.races && !m.skipFields {
			g.currentScreen, g.returnScreen, leave = 6, 0, true
		}
		if r == f.planets && !m.skipFields {
			g.currentScreen, g.returnScreen, leave = 0x20, 0, true
		}
		if r == f.leaders && !m.skipFields {
			switch {
			case m.officerScrnType != 0:
				g.screenData[4] = int16(g.starIDForOfficersScreen())
			case g.moveableBoxSelected(2):
				g.screenData[4] = g.fb.stack
			default:
				g.screenData[4] = g.shipStackForOfficersScreen()
			}
			m.officerScreenReturnScreen = 0
			g.currentScreen, g.returnScreen, leave = 0x1d, 0, true
		}
		switch {
		case r == f.resrch && !m.skipFields:
			// The map goes to both pages for Tech_Change_ to draw over.
			m.dontDrawToBack = true
			e.ClearFields()
			g.setMainScreenFieldsToNull()
			e.V.SetPageOff()
			if err := g.drawMiniMainScreen(); err != nil {
				return err
			}
			e.V.CopyOffToBack()
			if err := e.TogglePages(); err != nil {
				return err
			}
			e.V.SetRefreshStencil()
			if g.moveableBoxSelected(2) {
				m.forceFade = true
			}
			g.currentScreen, g.returnScreen, leave = ScreenTechChange, 0, true
			m.clearFleetFields = false
		case r == f.freighters:
			if s.Players[s.PlayerNum].FreightersTotal < 1 {
				if err := g.mainUserBox(g.hMessage(0xdf)); err != nil {
					return err
				}
			} else {
				g.currentScreen, g.returnScreen, leave = ScreenScrapFreighters, 0, true
				m.clearFleetFields = false
			}
		case r == f.commandPoints && !m.skipFields:
			g.currentScreen, g.returnScreen, leave = ScreenCommandPoints, 0, true
			m.clearFleetFields = false
		case r == f.treasury:
			// 0x86f8e: the mini map behind, the popup, then the map again.
			e.ClearFields()
			g.setMainScreenFieldsToNull()
			e.AssignAutoFunction(g.drawMiniMainScreen, 1)
			e.V.SetPageOff()
			if err := g.drawMiniMainScreen(); err != nil {
				return err
			}
			e.V.CopyOffToBack()
			e.V.SetRefreshStencil()
			if err := e.TogglePages(); err != nil {
				return err
			}
			if err := g.taxRatePopup(); err != nil {
				return err
			}
			r = -1
			g.doPlayerColonyCalculations(int(s.PlayerNum))
			leave, m.clearFleetFields = true, false
			g.currentScreen = ScreenMain
		}
		if (r == f.zoomOut && s.CurMapScale < s.MaxMapScale) || (r == f.zoomIn && s.CurMapScale > 10) {
			for i := range s.Stars {
				s.Stars[i].TwinkleFrame, s.Stars[i].TwinkleEnd = -1, -1
			}
			m.nTwinklingStars = 0
			e.DeactivateHelpList()
		}
		ox, oy := s.CurMapX, s.CurMapY
		switch {
		case r == f.zoomIn && s.CurMapScale > 10 && !m.skipFields:
			if zoomSteps == 0 {
				g.saveAndRemoveBoxes()
			}
			zoomSteps++
			if !m.zoomIn {
				if err := g.enterZoomMode(); err != nil {
					return err
				}
			}
			m.zoomIn = true
			e.ClearFieldsAbove(f.lastField)
			if err := g.addMapFields(); err != nil {
				return err
			}
			e.SetMouseList(mouseListZoomMode)
			m.zoomCount = max(m.zoomCount-1, 0)
			m.drawToBack = true
		case r == f.zoomOut && m.zoomIn && !m.skipFields:
			if !g.incrementZoomCountWithLimit() {
				zoomSteps = 0
				m.zoomIn = false
				e.SetMouseList(engine.MouseArrow)
				e.AssignAutoFunction(g.drawMainScreen, 1)
				g.setMainScreenHelpList()
			}
		case r == f.zoomOut && s.CurMapScale < s.MaxMapScale && !m.skipFields:
			g.zoomOut(ox, oy)
			zoomSteps = 0
		case r == f.whole && m.zoomIn && !m.skipFields:
			g.restoreBoxes()
			m.zoomIn = false
			g.setMainScreenHelpList()
			e.SetMouseList(engine.MouseArrow)
			e.AssignAutoFunction(g.drawMainScreen, 1)
			s.CurMapX += int16(g.upScaled(m.zoomBoxX - 0x15))
			s.CurMapY += int16(g.upScaled(m.zoomBoxY - 0x15))
			zoomSteps = 0
			sc := g.getMapScale(int(m.zoomCount))
			m.farX, m.midX, m.nearX, m.farY, m.midY, m.nearY = 0x16, 0x16, 0x16, 0x16, 0x16, 0x16
			m.drawToBack = true
			s.CurMapScale = sc
		}
		if r == -f.grid && !m.skipFields && s.CurMapScale != s.MaxMapScale {
			if err := g.scrollMap(int16(g.upScaled(int16(g.gridX)))+s.CurMapX, int16(g.upScaled(int16(g.gridY)))+s.CurMapY); err != nil {
				return err
			}
		}
		if !leave && !g.globalEsc {
			if !m.skipFields {
				e.ClearFieldsAbove(f.lastField)
				e.EnableDrawAllFields()
				e.EnableMousePressedFieldUpdate()
			} else {
				m.skipFields = false
				e.DisableDrawAllFields()
				e.ClearFields()
				g.setMainScreenFieldsToNull()
				if err := g.addCheats(); err != nil {
					return err
				}
			}
			if err := g.addMapFields(); err != nil {
				return err
			}
			e.V.SetPageOff()
			draw := g.drawMainScreen
			if m.zoomIn {
				draw = g.drawMainScreenForZooming
			}
			if err := draw(); err != nil {
				return err
			}
			if err := e.TogglePages(); err != nil {
				return err
			}
		}
		if g.moveableBoxSelected(2) {
			if g.fb.btnPressed && e.MouseButton() == 0 {
				g.fb.btnPressed = false
				e.SetInputDelay(2)
			}
			g.saveShipSelectionStatus()
			if !g.someIconSelected() {
				m.etaStarID = -1
			}
		}
		if err := e.ReleaseTime(1); err != nil {
			return err
		}
	}
}

// zoomOut is the zoom-out branch of Main_Screen_ (0x873b7) outside zoom
// mode.
func (g *Game) zoomOut(ox, oy int16) {
	m, s := &g.ms, &g.state
	before := g.mapScaleToZoomLevel()
	g.incrementZoomCount()
	s.CurMapScale = g.getMapScale(int(m.zoomCount))
	after := g.mapScaleToZoomLevel()
	s.CurMapX -= int16((after - before) * 0x1fa / 4)
	s.CurMapY -= int16((after - before) * 400 / 4)
	if lim := (int(s.MaxMapScale) - int(s.CurMapScale)) * 0x1fa / 10; lim < int(s.CurMapX) {
		s.CurMapX = int16(lim)
	}
	if lim := (int(s.MaxMapScale) - int(s.CurMapScale)) * 400 / 10; lim < int(s.CurMapY) {
		s.CurMapY = int16(lim)
	}
	s.CurMapX, s.CurMapY = max(s.CurMapX, 0), max(s.CurMapY, 0)
	if s.CurMapScale == s.MaxMapScale {
		s.CurMapX, s.CurMapY = 0, 0
	}
	if v := g.upScaled(s.MapMaxX - 0x280); v < int(s.CurMapX) {
		s.CurMapX = int16(v)
	}
	if v := g.upScaled(400); int(s.MapMaxY)-v < int(s.CurMapY) {
		s.CurMapY = s.MapMaxY - int16(v)
	}
	m.drawToBack = true
	g.getNewParalaxXYs(s.CurMapX-ox, s.CurMapY-oy, 1)
}

// incrementZoomCount is Increment_Zoom_Count_.
func (g *Game) incrementZoomCount() {
	m := &g.ms
	m.zoomCount = min(m.zoomCount+1, g.state.MaxZoomCount)
}

// getMapScale is Get_Map_Scale_: the scale for a zoom level, at most
// max_map_scale; reaching it resets the parallax.
func (g *Game) getMapScale(level int) int16 {
	m, s := &g.ms, &g.state
	sc := int16(30)
	switch level {
	case 0:
		sc = 10
	case 1:
		sc = 15
	case 2:
		sc = 20
	}
	if s.MaxMapScale < sc {
		sc = s.MaxMapScale
	}
	if sc == s.MaxMapScale {
		m.farX, m.farY, m.midX, m.midY, m.nearX, m.nearY = 0, 0, 0, 0, 0, 0
	}
	return sc
}

// getNewParalaxXYs is Get_New_Paralax_XYs_ (0x8590a): the layers move by
// |d|, 2|d| and 4|d| whatever the direction.
func (g *Game) getNewParalaxXYs(x, y int16, div int) {
	m, s := &g.ms, &g.state
	dx := int16(int(x-s.CurMapX) / div)
	dy := int16(int(y-s.CurMapY) / div)
	sx, sy := int16(0), int16(0)
	switch {
	case dx > 0:
		sx = -1
	case dx < 0:
		sx = 1
	}
	switch {
	case dy > 0:
		sy = -1
	case dy < 0:
		sy = 1
	}
	if x != s.CurMapX {
		p := sx * dx
		m.farX -= p
		m.midX -= 2 * p
		m.nearX -= 4 * p
	}
	if y != s.CurMapY {
		q := sy * dy
		m.farY -= q
		m.midY -= 2 * q
		m.nearY -= 4 * q
	}
	for _, v := range []*int16{&m.farX, &m.midX, &m.nearX} {
		if *v < 0 {
			*v += 640
		}
		*v %= 640
	}
	for _, v := range []*int16{&m.farY, &m.midY, &m.nearY} {
		if *v < 0 {
			*v += 480
		}
		*v %= 480
	}
}

// mainScreenHelp is _main_screen_help_list, the static part.
var mainScreenHelp = []engine.HelpEntry{
	{ID: 284, X1: 542, Y1: 24, X2: 616, Y2: 41}, {ID: 285, X1: 237, Y1: 0, X2: 320, Y2: 20},
	{ID: 286, X1: 541, Y1: 45, X2: 617, Y2: 119}, {ID: 288, X1: 541, Y1: 121, X2: 617, Y2: 193},
	{ID: 290, X1: 541, Y1: 195, X2: 617, Y2: 268}, {ID: 291, X1: 541, Y1: 270, X2: 617, Y2: 342},
	{ID: 292, X1: 541, Y1: 344, X2: 617, Y2: 418}, {ID: 293, X1: 13, Y1: 428, X2: 79, Y2: 473},
	{ID: 294, X1: 91, Y1: 428, X2: 155, Y2: 473}, {ID: 295, X1: 166, Y1: 428, X2: 231, Y2: 473},
	{ID: 300, X1: 236, Y1: 425, X2: 307, Y2: 479}, {ID: 297, X1: 311, Y1: 428, X2: 381, Y2: 475},
	{ID: 298, X1: 385, Y1: 428, X2: 454, Y2: 475}, {ID: 299, X1: 461, Y1: 428, X2: 527, Y2: 475},
	{ID: 296, X1: 542, Y1: 426, X2: 618, Y2: 479},
}

// setMainScreenHelpList is Set_Main_Screen_Help_List_ (0x6f280).
func (g *Game) setMainScreenHelpList() {
	list := append([]engine.HelpEntry(nil), mainScreenHelp...)
	switch g.gameType {
	case save.GameHotseat:
		list = append(list, engine.HelpEntry{ID: 0x12f, X1: 0x49, Y1: 0, X2: 0x9c, Y2: 0x13})
	case save.GameNetwork:
		list = append(list, engine.HelpEntry{ID: 0x12f, X1: 0x49, Y1: 0, X2: 0x9c, Y2: 0x13},
			engine.HelpEntry{ID: 0x12e, X1: 0x1a2, Y1: 0, X2: 0x207, Y2: 0x13})
	}
	if g.moveableBoxSelected(2) {
		list = append(list, g.fleetBoxHelpEntries()...)
	}
	g.e.DeactivateHelpList()
	g.e.SetHelpList(list)
}

// addMapFields is Add_Map_Fields_ (0x81d73) outside zoom mode, without the
// moveable box fields.
func (g *Game) addMapFields() error {
	e, m := g.e, &g.ms
	f := &m.fields
	var err error
	add := func(dst *int, fn func() (int, error)) {
		if err == nil {
			*dst, err = fn()
		}
	}
	if m.zoomIn {
		g.setMainScreenFieldsToNull()
		add(&f.zoomIn, func() (int, error) { return e.AddButtonField(0xf4, 0x1ac, "", m.zoomInButton, "+", 0x28) })
		add(&f.zoomOut, func() (int, error) { return e.AddButtonField(0xf4, 0x1c7, "", m.zoomOutButton, "-", 0x28) })
		add(&f.whole, func() (int, error) { return e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0x29) })
		return err
	}
	add(&f.game, func() (int, error) { return e.AddButtonField(0xf9, 5, "", m.gameButton, "G", 0x29) })
	add(&f.next, func() (int, error) { return e.AddButtonField(0x220, 0x1b9, "", m.nextButton, "T", 0x28) })
	add(&f.zoomIn, func() (int, error) { return e.AddButtonField(0xf4, 0x1ac, "", m.zoomInButton, "+", 0x28) })
	add(&f.zoomOut, func() (int, error) { return e.AddButtonField(0xf4, 0x1c7, "", m.zoomOutButton, "-", 0x28) })
	irr := func(dst *int, x1, y1, x2, y2 int, pic *gfxPicture, dx int, hot string) {
		add(dst, func() (int, error) {
			return e.AddIrregularButtonField(x1, y1, x2, y2, "", pic, dx, 0x1a9, hot, 0x28)
		})
	}
	irr(&f.colonies, 0x11, 0x1b2, 0x4f, 0x1d7, m.coloniesButton, 0, "C")
	irr(&f.planets, 0x5b, 0x1b2, 0x9a, 0x1d7, m.planetsButton, 0, "P")
	irr(&f.fleets, 0xa7, 0x1b2, 0xe6, 0x1d7, m.fleetsButton, 0, "F")
	irr(&f.leaders, 0x138, 0x1b3, 0x17b, 0x1d7, m.leadersButton, 0x132, "L")
	irr(&f.races, 0x182, 0x1b3, 0x1c5, 0x1d7, m.racesButton, 0x132, "R")
	irr(&f.info, 0x1ce, 0x1b3, 0x20e, 0x1d7, m.infoButton, 0x132, "I")
	hid := func(dst *int, y1, y2 int, hot string) {
		add(dst, func() (int, error) { return e.AddHiddenField(0x221, y1, 0x265, y2, hot, 0x29) })
	}
	hid(&f.treasury, 0x31, 0x77, "")
	hid(&f.commandPoints, 0x7a, 0xc0, "")
	hid(&f.food, 0xc5, 0x10b, "")
	hid(&f.freighters, 0x10e, 0x154, "")
	hid(&f.resrch, 0x158, 0x19f, "")
	if err != nil {
		return err
	}
	if err := g.addMoveableBoxFields(); err != nil {
		return err
	}
	f.debug = engine.NoField
	add(&f.exitGame, func() (int, error) { return e.AddMultiHotKeyField("Q") })
	add(&f.grid, func() (int, error) {
		return e.AddGridField(0x16, 0x16, 1, 1, 0x1fa, 400, &g.gridX, &g.gridY, 0)
	})
	return err
}

// drawPlayerDoneLights is Draw_Player_Done_Lights_ (0x8520c): a light for
// each human done with the turn (hotseat: his status; network: ready).
func (g *Game) drawPlayerDoneLights() error {
	s, m := &g.state, &g.ms
	remaining := g.nRemainingHumans()
	done := 0
	for p := 0; p < int(s.NumPlayers); p++ {
		switch g.gameType {
		case save.GameHotseat:
			if g.hotseatPlayerStatus(p) == 1 {
				done++
			}
		case save.GameNetwork, save.GameModem:
			if g.net.u8(netReady+uint32(p)) != 0 {
				done++
			}
		}
	}
	keep := done >= remaining-1
	n := 0
	for p := 0; p < int(s.NumPlayers); p++ {
		netDone := (g.gameType == save.GameNetwork || g.gameType == save.GameModem) && g.net.u8(netReady+uint32(p)) != 0
		if !netDone && (g.gameType != save.GameHotseat || g.hotseatPlayerStatus(p) != 1) {
			continue
		}
		light := m.playerLights[s.Players[p].Color]
		x := m.playerLights[0].Width*n + 0x49
		if !keep {
			g.setAnimationFrame(light, 5)
		}
		n++
		if err := g.e.Draw(x, 5, light); err != nil {
			return err
		}
	}
	return nil
}

// nRemainingHumans is Get_N_Remaining_Humans_ (0x7a54f).
func (g *Game) nRemainingHumans() int {
	s, n := &g.state, 0
	for p := 0; p < int(s.NumPlayers); p++ {
		if s.Players[p].Objective == save.ObjectiveHuman && s.Players[p].Eliminated == 0 {
			n++
		}
	}
	return n
}

// printStardateCorner is the stardate print of Draw_Main_Screen_: at y 0 it
// is above the map window, so the clipped printer always drops it.
func (g *Game) printStardateCorner() error {
	t := g.e.Text()
	sd := g.state.Stardate
	str := fmt.Sprintf(g.hMessage(0xda), int(sd/10), int(sd%10))
	t.SetStyle(1, [8]byte{0x00, 0x0a, 0x0c, 0x0e, 0x10, 0x12, 0x14, 0x16})
	return t.PrintRight(0x27f, 0, str)
}

// shipStackForOfficersScreen is Get_Ship_Stack_For_Officers_Screen_
// (0x85aec): it means the player's first stack but returns its first node,
// not its index.
func (g *Game) shipStackForOfficersScreen() int16 {
	st, p := &g.stacks, int(g.state.PlayerNum)
	node := st.start[0]
	if node <= -1 {
		return node
	}
	for i := int16(0); ; i++ {
		if int(g.state.Ships[st.node[node].ship].Owner) == p {
			return node
		}
		if i >= st.count {
			return -1
		}
		node = st.start[i]
	}
}
