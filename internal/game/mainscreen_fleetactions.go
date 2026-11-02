package game

// The fleet box's COLONIZE, OUTPOST and TRANSPORT buttons: screens 0x1e,
// 0x1f and 0x22 (MAINSCR.C). Spec: docs/screens/fleet-box-actions.md.

// fleetActionSetup is the shared head of the three screens: settings[4] is
// saved and cleared, the box vanishes and the mini map fills both pages.
// The original compares the box's corner with 0x20f, 0x1a5 and ignores it.
func (g *Game) fleetActionSetup() (star int, saved byte, err error) {
	e := g.e
	saved = g.settings[4]
	star = int(g.fleetHead().Location)
	g.settings[4] = 0
	e.AssignAutoFunction(g.drawMiniMainScreen, 1)
	e.ClearFields()
	g.vanishFleetMovementBox()
	e.DisableDrawAllFields()
	e.V.SetPageOff()
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	if err = g.drawMiniMainScreen(); err != nil {
		return
	}
	if err = e.TogglePages(); err != nil {
		return
	}
	e.EnableDrawAllFields()
	return
}

// fleetBoxFirstShipOfType is the search loop of the colonize and outpost
// screens: the first ship of the box with that design type, from the top.
func (g *Game) fleetBoxFirstShipOfType(t byte) (int, bool) {
	for k := 0; k < int(g.fb.n); k++ {
		id := g.fleetMovementShipID(k)
		if g.state.Ships[id].Design.Type == t {
			return id, true
		}
	}
	return 0, false
}

// fleetActionUsedShip marks ship id as used up (status 5, removed at the
// next turn) and drops the box's cached stack; true when it was the last.
func (g *Game) fleetActionUsedShip(id int) bool {
	g.state.Ships[id].Status = 5
	last := g.fb.n == 1
	g.ms.firstNodePtrInStack = -1
	g.fb.added--
	return last
}

// fleetActionRedraw is the shared redraw when ships stay in the box.
func (g *Game) fleetActionRedraw(buildIcons, resetEta bool) error {
	e, s := g.e, &g.state
	e.V.SetPageOff()
	if err := g.drawMainScreenFilled(true); err != nil {
		return err
	}
	if err := g.drawMainScreen(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	if err := g.loadPlayerShipPalette(int(s.PlayerNum)); err != nil {
		return err
	}
	s.MoveableBoxes[2].Content = 1
	if buildIcons {
		g.buildShipIcons()
	}
	if resetEta {
		g.ms.etaStarID = -1
	}
	g.fb.stack = int16(g.shipStackFromSavedArray())
	if g.fb.stack == -1 {
		g.vanishFleetMovementBox()
	}
	return nil
}

// fleetActionTail is the shared tail at 0x8ab19.
func (g *Game) fleetActionTail(saved byte) {
	g.settings[4] = saved
	g.currentScreen = g.returnScreen
	g.e.ClearFields()
	g.e.DeactivateAutoFunction()
	g.clearSystemDisplayLoadedFlag()
}

// planetColonizationInMainScreen is Planet_Colonization_In_Main_Screen_
// (0x8b2de). Its Can_Change_Star_Name_ call's result is unused.
func (g *Game) planetColonizationInMainScreen() error {
	s := &g.state
	star, saved, err := g.fleetActionSetup()
	if err != nil {
		return err
	}
	g.canChangeStarName(star)
	pl, err := g.newColonySelectionPopup(star, true)
	if err != nil {
		return err
	}
	last, landing := false, false
	if int16(pl) > -1 {
		if id, ok := g.fleetBoxFirstShipOfType(1); ok {
			landing = g.settings[6] != 0
			g.makeNewColonyOrOutpost(pl, int(s.PlayerNum), false)
			last = g.fleetActionUsedShip(id)
		}
	}
	if landing {
		g.screenData[0] = int16(star)
		g.screenData[1] = int16(s.Planets[pl].Orbit)
		if err := g.colonyLandingScreen(); err != nil {
			return err
		}
		// Fade_Into_Screen_ gets EDX = &_draw_main_screen_to_back, kept.
		g.ms.drawToBack = true
		if err := g.fadeIntoScreen(g.drawMiniMainScreen, 0); err != nil {
			return err
		}
	}
	if !last {
		if err := g.fleetActionRedraw(true, false); err != nil {
			return err
		}
	}
	g.fleetActionTail(saved)
	return nil
}

// buildingOutpostsInMainScreen is Building_Outposts_In_Main_Screen_
// (0x8a97a).
func (g *Game) buildingOutpostsInMainScreen() error {
	s := &g.state
	star, saved, err := g.fleetActionSetup()
	if err != nil {
		return err
	}
	pl, err := g.newOutpostSelectionPopup(star)
	if err != nil {
		return err
	}
	last := false
	if int16(pl) > -1 {
		if id, ok := g.fleetBoxFirstShipOfType(4); ok {
			g.makeNewColonyOrOutpost(pl, int(s.PlayerNum), true)
			last = g.fleetActionUsedShip(id)
		}
	}
	if !last {
		if err := g.fleetActionRedraw(true, true); err != nil {
			return err
		}
	}
	g.fleetActionTail(saved)
	return nil
}

// dropTroopsSelectionPopup is Drop_Troops_Selection_Popup_ (0xc8d3c).
func (g *Game) dropTroopsSelectionPopup(st int) (int, error) {
	g.mp.selectionState = selectTroops
	defer func() { g.mp.selectionState = selectColony }()
	return g.newColonySelectionPopup(st, false)
}

// unloadingTransportsInMainScreen is Unloading_Transports_In_Main_Screen_
// (0x8b7a5): the selected transports land on one of the player's colonies.
func (g *Game) unloadingTransportsInMainScreen() error {
	s := &g.state
	star, saved, err := g.fleetActionSetup()
	if err != nil {
		return err
	}
	pl, err := g.dropTroopsSelectionPopup(star)
	if err != nil {
		return err
	}
	last := false
	if int16(pl) > -1 {
		ci := int(s.Planets[pl].Colony)
		if ci > -1 && int16(int8(s.Colonies[ci].Owner)) == s.PlayerNum {
			ids := g.fleetBoxSelectedShipIDs(2)
			last = int16(g.unloadTransports(ci, ids)) == g.fb.n
			g.ms.firstNodePtrInStack = -1
			g.fb.added--
		}
	}
	if !last {
		g.updateShipIconsInBetweenTurns()
		if err := g.fleetActionRedraw(false, false); err != nil {
			return err
		}
	}
	g.fleetActionTail(saved)
	return nil
}
