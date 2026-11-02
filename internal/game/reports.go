package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/save"
)

// Reports_Screen_ and the begin-of-turn path. Spec:
// docs/screens/reports-and-begin-of-turn.md.

// reportsScreen is Reports_Screen_ (screen 0x27).
func (g *Game) reportsScreen() error {
	e, m, s := g.e, &g.ms, &g.state
	g.currentScreen = ScreenMain
	r, err := g.mustDisplayReports(int(s.PlayerNum))
	if err != nil {
		return err
	}
	if r != 0 {
		e.SetMouseList(engine.MouseArrow)
	}
	e.V.SetRefreshStencil()
	e.AssignAutoFunction(g.drawMiniMainScreen, 1)
	fade := false
	if m.stardateAtBegOfTurn == 0 {
		fade = !(r == 2 || r == 3 || r == 5 || r == 6 || (r == 7 && g.settings[0x09] != 0))
	} else {
		fade = m.previousReport == 3 && r != 3
	}
	if r == 3 && g.previousScreen != ScreenMainMenu {
		fade = false
	} else if m.atLeastOneTacticalCombat && r != 0 {
		fade = true
	}
	if fade && !m.alreadyFaded {
		m.atLeastOneTacticalCombat = false
		e.ClearFields()
		g.setMainScreenFieldsToNull()
		s.updateStarOwners()
		g.buildShipIcons()
		if err := e.FastFadeOut(); err != nil {
			return err
		}
		if err := g.addMapFields(); err != nil {
			return err
		}
		if err := g.fadeIntoMainScreen(); err != nil {
			return err
		}
	}
	if err := g.doBeginOfTurn(); err != nil {
		return err
	}
	if g.currentScreen == 1 {
		g.returnScreen = ScreenReports
	}
	m.fadeInFlag = m.atLeastOneReport
	if m.atLeastOneReport {
		g.clearSystemDisplayLoadedFlag()
	}
	if g.moveableBoxSelected(2) && m.atLeastOneReport {
		m.forceFade = true
		m.reloadedShipPalette = true
	}
	if r == 0 {
		m.stardateAtBegOfTurn = s.Stardate
	}
	e.DeactivateAutoFunction()
	return nil
}

// mustDisplayReports is Must_Display_Reports_ (0xfe86b): the report p
// would see next, without showing it. With none left, p's new ships take
// their relocation orders.
func (g *Game) mustDisplayReports(p int) (int, error) {
	s := &g.state
	r := reportNone
	if s.Players[p].Objective == 100 {
		var err error
		if r, err = g.displayReportAux(p, false); err != nil {
			return 0, err
		}
	}
	if r == reportNone {
		g.redirectNewlyBuiltShips(p)
		if g.gameType == 2 || g.gameType == 3 {
			if err := g.doAutosave(); err != nil {
				return 0, err
			}
		}
	}
	return r, nil
}

// redirectNewlyBuiltShips is Redirect_Newly_Built_Ships_ (0xfe793): p's ships
// built this turn lose the flag, and those at a star with a relocation
// order head for its target.
func (g *Game) redirectNewlyBuiltShips(p int) {
	s := &g.state
	changed := false
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if int16(sh.Owner) != int16(p) || sh.JustBuilt == 0 {
			continue
		}
		sh.JustBuilt = 0
		changed = true
		if sh.Status != 0 || s.Stars[sh.Location].Relocation[p] == -1 {
			continue
		}
		dest := int(s.Stars[sh.Location].Relocation[p])
		ids := []int16{int16(i), -1}
		if info := g.shipsTryToMoveTo(ids, dest); info.canMove {
			g.makeShipsMoveTo(ids, dest, &info)
		}
	}
	if changed {
		g.doPlayerColonyCalculations(p)
	}
}

// doBeginOfTurn is Do_Begin_Of_Turn_ (0x8ad82).
func (g *Game) doBeginOfTurn() error {
	m, s := &g.ms, &g.state
	m.inReportsPhase = true
	rep, err := g.beginOfTurn()
	if err != nil {
		return err
	}
	m.officerScreenReturnScreen = ScreenReports
	enemy := g.enemyShipGoingToMyColony()
	if g.settings[0x01] == 0 && rep == 0 && s.SomethingHappened[s.PlayerNum] == 0 &&
		m.nextTurnHit && g.gameType == 0 && !enemy {
		if err := g.autoNextTurn(); err != nil {
			return err
		}
	} else {
		m.checkNextTurnExitField = false
		g.e.SetMouseList(engine.MouseArrow)
		m.nextTurnHit = false
		g.e.ClearFields()
	}
	m.inReportsPhase = false
	return nil
}

// autoNextTurn is Do_Begin_Of_Turn_'s automatic next turn (0x8adfc): with
// "end of turn wait" off, the "<race> year N.N" banner over the map and
// straight on to the next turn; a click or Esc stops it (Do_Next_Turn_).
func (g *Game) autoNextTurn() error {
	m, s, e := &g.ms, &g.state, g.e
	m.animatedStar = -1
	m.checkNextTurnExitField = true
	e.DisableDrawAllFields()
	e.AssignAutoFunction(g.drawMiniMainScreen, 1)
	e.V.SetPageOff()
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	c := g.playerFontColors(int(s.PlayerNum), 3, false)
	tx := e.Text()
	tx.SetStyle(3, c)
	msg := cSprintf(cFormat(g.hMessage(0xe9)), save.CString(s.Players[s.PlayerNum].RaceName[:]),
		s.Stardate/10, s.Stardate%10)
	w, h := int16(tx.Width([]byte(msg))), int16(tx.Height())
	x := (0x1fa - (w + 8)) / 2
	y := (400 - (h + 4)) / 2
	e.V.SetPaletteChanges(0, 0xff)
	e.UpdateGlassRemapColors()
	g.drawDarkenedBox(int(x+0x15), int(y+0x15), int(w+8), int(h+4), c[1])
	if err := tx.Print(int(x+0x1d), int(y+0x1b), msg); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.EnableDrawAllFields()
	return g.doNextTurnAt(true)
}

// enemyShipGoingToMyColony is Enemy_Ship_Going_To_My_Colony_ (0x77eed):
// some stack of another owner is at or bound for a star where the human
// has a colony.
func (g *Game) enemyShipGoingToMyColony() bool {
	st, s := &g.stacks, &g.state
	for i := int16(0); i < st.count; i++ {
		ship := int(st.node[st.start[i]].ship)
		if g.shipOwner(ship) == int(s.PlayerNum) {
			continue
		}
		if g.playerHasColony(int(s.PlayerNum), int(absoluteLocation(s.Ships[ship].Location))) {
			return true
		}
	}
	return false
}

// beginOfTurn is Begin_Of_Turn_ (0x8a84b).
func (g *Game) beginOfTurn() (int, error) {
	m, s := &g.ms, &g.state
	rep, err := g.mainScreenReportHandler()
	if err != nil || g.globalEsc {
		return rep, err
	}
	m.shipIconPlayerID, m.etaStarID = -1, -1
	g.setMainScreenFieldsToNull()
	s.updateStarOwners()
	g.buildShipIcons()
	if g.gameType != 1 && (m.savedBoxWasOpen[2] || g.moveableBoxSelected(2)) {
		g.fb.stack = int16(g.shipStackFromSavedArray())
		if g.fb.stack == -1 {
			g.vanishFleetMovementBox()
			m.savedBoxWasOpen[2], m.savedBoxes[2] = false, m.boxOffFlag[2]
		}
	}
	g.e.DisableDrawAllFields()
	g.e.AssignAutoFunction(g.drawMainScreen, 2)
	open2 := m.savedBoxWasOpen[2]
	g.restoreBoxes()
	if open2 || g.moveableBoxSelected(2) {
		g.reloadShipSelectionStatus()
	}
	if open2 {
		g.fb.stack = int16(g.shipStackFromSavedArray())
		if g.fb.stack == -1 {
			s.MoveableBoxes[2].Content = m.boxOffFlag[2]
		} else {
			s.MoveableBoxes[2].Content = 1
			if err := g.fadeIntoMainScreen(); err != nil {
				return rep, err
			}
			m.forceFade = false
		}
	}
	g.e.EnableDrawAllFields()
	return rep, nil
}

// mainScreenReportHandler is Main_Screen_Report_Handler_ (0x8b17b): show
// the human's reports one after another over the mini main screen.
func (g *Game) mainScreenReportHandler() (int, error) {
	e, m, s := g.e, &g.ms, &g.state
	rep, err := g.mustDisplayReports(int(s.PlayerNum))
	if err != nil {
		return 0, err
	}
	if m.stardateAtBegOfTurn != 0 && m.stardateAtBegOfTurn != s.Stardate {
		m.alreadyFaded = false
	}
	if rep == 0 {
		if m.atLeastOneTacticalCombat {
			if g.moveableBoxSelected(2) || g.moveableBoxSelected(0) {
				m.forceFade = true
			}
			m.atLeastOneTacticalCombat = false
		}
		return 0, nil
	}
	m.atLeastOneReport = true
	m.mainscrPalette = 0xff
	e.AssignAutoFunction(g.drawMiniMainScreen, 1)
	if m.stardateAtBegOfTurn != s.Stardate {
		if g.isMainScreenPalette() {
			e.V.SetPageOff()
			if err := g.drawMiniMainScreen(); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
		} else if err := g.fadeIntoMainScreen(); err != nil {
			return 0, err
		}
		m.stardateAtBegOfTurn = s.Stardate
	}
	for {
		m.alreadyFaded = false
		g.clearSystemDisplayShipFields()
		g.clearSystemDisplayLoadedFlag()
		r, err := g.displayReportAux(int(s.PlayerNum), true)
		if err != nil {
			return 0, err
		}
		m.previousReport = r
		full := r == reportFullScreen || r == reportDiplomacy || r == reportAntarans ||
			r == reportGNN && g.settings[0x09] != 0
		if g.globalEsc {
			return rep, nil
		}
		n, err := g.mustDisplayReports(int(s.PlayerNum))
		if err != nil {
			return 0, err
		}
		if full && n != reportLeaves {
			g.sd.skipPlanets = g.moveableBoxSelected(0)
			if err := g.fadeIntoMainScreen(); err != nil {
				return 0, err
			}
			m.alreadyFaded = true
		}
		if n == reportNone {
			e.AssignAutoFunction(g.drawMainScreen, 1)
			return rep, nil
		}
	}
}

// clearSystemDisplayShipFields is Clear_System_Display_Ship_Fields_ (0xa20ec).
func (g *Game) clearSystemDisplayShipFields() {
	for i := range g.sd.ships {
		g.sd.ships[i] = sysDisplayShip{field: -1000, node: -1}
	}
}

// restoreBoxes is Restore_Boxes_ (0x9191d).
func (g *Game) restoreBoxes() {
	m, s := &g.ms, &g.state
	for b := 0; b < numBoxes; b++ {
		if m.savedBoxes[b] == -1000 {
			continue
		}
		s.MoveableBoxes[b].Content = m.savedBoxes[b]
		if m.savedBoxWasOpen[b] {
			switch b {
			case 0:
				// Init_System_Display_ gets stale registers for its size
				// outputs (zero: nothing the port keeps).
				if err := g.initSystemDisplay(int(s.MoveableBoxes[0].Content)); err != nil {
					g.Log("Init_System_Display_: %v", err)
				}
			case 2:
				if !g.getStackStartFromSavedShipArray() {
					g.vanishFleetMovementBox()
				}
			}
		}
		m.savedBoxWasOpen[b] = false
		m.savedBoxes[b] = -1000
	}
}

// fadeIntoMainScreen is Fade_Into_Main_Screen_ (0x8a6c2).
func (g *Game) fadeIntoMainScreen() error {
	e, m := g.e, &g.ms
	sys := g.moveableBoxSelected(0)
	if m.alreadyFaded {
		m.alreadyFaded = false
		return nil
	}
	e.MoxUpdate()
	m.fadeInFlag, m.forceFade, m.alreadyFaded = false, false, true
	e.DisableDrawAllFields()
	e.V.SetRefreshStencil()
	e.SetMouseList(engine.MouseNone)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	e.MoxUpdate()
	if err := e.LoadPalette(0, 0, 0xff); err != nil {
		return err
	}
	e.MoxUpdate()
	if err := e.DarkenPalette(100); err != nil {
		return err
	}
	e.MoxUpdate()
	if err := g.addMapFields(); err != nil {
		return err
	}
	e.MoxUpdate()
	if sys {
		// 0x8a74b: the exploding box is off while the system box reopens.
		boom := g.settings[4] != 0
		g.settings[4] = 0
		e.MoxUpdate()
		if err := g.doSystemPopup(int(g.state.MoveableBoxes[0].Content)); err != nil {
			return err
		}
		if boom {
			g.settings[4] = 1
		}
	}
	e.V.SetPageOff()
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	e.MoxUpdate()
	e.V.ClearPaletteChanges(0, 0xff)
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.SetMouseList(engine.MouseArrow)
	if m.reloadedShipPalette && !g.moveableBoxSelected(2) {
		e.SetFadePercent(0)
	} else {
		if err := e.FastFadeIn(); err != nil {
			return err
		}
		e.MoxUpdate()
	}
	m.reloadedShipPalette = false
	e.EnableDrawAllFields()
	return nil
}
