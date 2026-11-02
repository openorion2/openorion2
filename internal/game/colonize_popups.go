package game

import (
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/save"
)

// Colonization_'s show path and its popups (COLONIZE.C, MAINPUPS.C): the
// planet selection over the system display, the confirmation box with the
// planet's prospects, trashing colony bases, settling. Spec:
// docs/screens/colonization.md.

const (
	mainScrFontColors = 0x1831b7 // _main_scr_font_colors
	potentialGravity  = 0xdd4ba  // Compute_Potential_Colony_Info_'s tables
	potentialMinerals = 0xdd4b5
	climateFood       = 0x17d81c // by climate
	colourCodeA       = 0x179e6e // "\x1a0"
	colourCodeB       = 0x179e6b // "\x1a1"
)

// Selection modes (_selection_state_flag).
const (
	selectColony = iota
	selectOutpost
	selectTroops
	selectArtificial
)

// colonizeShow is Colonization_'s show part (0xfdbe5) for star st, with
// base the colony base there (-1), colony and outpost p's ships.
func (g *Game) colonizeShow(p, st, base, colony, outpost int) (int, error) {
	s := &g.state
	bit := uint8(1) << uint(p)
	for {
		// Remove_Box_Check_ is empty.
		isColony := false
		target := -1
		var err error
		if base == -1 && (colony == -1 || outpost != -1) {
			target = s.colonyShipAutoColonization(outpost)
			if target == -1 || s.Planets[target].Colony != -1 {
				if target, err = g.newOutpostSelectionPopup(st); err != nil {
					return 0, err
				}
			} else if target, err = g.preselectedPlanet(st, target, 7); err != nil {
				return 0, err
			}
		} else {
			isColony = true
			auto := -1
			if base == -1 {
				auto = s.colonyShipAutoColonization(colony)
			}
			if auto != -1 && s.Planets[auto].Colony == -1 {
				if target, err = g.preselectedPlanet(st, auto, 6); err != nil {
					return 0, err
				}
			} else if target, err = g.newColonySelectionPopup(st, base == -1); err != nil {
				return 0, err
			}
			for base != -1 && int16(target) < 0 {
				refund := int32(g.exe.u32(colonyBaseRefund)) / 2
				rs, err := g.reportStrings()
				if err != nil {
					return 0, err
				}
				yes, err := g.confirmationBox(cSprintf(rs[5], refund))
				if err != nil {
					return 0, err
				}
				if !yes {
					if target, err = g.newColonySelectionPopup(st, base == -1); err != nil {
						return 0, err
					}
					continue
				}
				for base != -1 {
					s.Players[p].BC += refund
					s.Colonies[base].Buildings[0xb] = 0
					base = s.playerColonyBaseAtStar(p, st)
				}
			}
		}
		if colony != -1 {
			s.clearAutoColonizationFlag(colony)
		}
		if outpost != -1 {
			s.clearAutoColonizationFlag(outpost)
		}
		switch {
		case int16(target) < 0:
			setBit(s.DoneColonizeCheck[:], st)
			s.Stars[st].ColonizePromptOff |= bit
		case isColony:
			g.makeNewColonyOrOutpost(target, p, false)
			if base == -1 {
				g.killShip(colony)
				g.updateShipIconsInBetweenTurns()
			} else {
				g.removeBuilding(base, 0xb)
				base = s.playerColonyBaseAtStar(p, st)
			}
			if s.starNColonizablePlanets(st, p) == 0 {
				setBit(s.DoneColonizeCheck[:], st)
				for base != -1 {
					s.Players[p].BC += int32(g.exe.u32(colonyBaseRefund)) / 2
					s.Colonies[base].Buildings[0xb] = 0
					base = s.playerColonyBaseAtStar(p, st)
				}
			}
			if g.settings[6] == 0 {
				return reportPopup, nil
			}
			P := &s.Planets[target]
			g.screenData[0], g.screenData[1] = int16(P.Star), int16(P.Orbit)
			g.currentScreen, g.returnScreen = ScreenLanding, ScreenReports
			g.globalEsc = true
			return reportLeaves, nil
		default:
			g.makeNewColonyOrOutpost(target, p, true)
			g.killShip(outpost)
			g.updateShipIconsInBetweenTurns()
		}
		if base == -1 {
			return reportPopup, nil
		}
	}
}

// preselectedPlanet asks whether to settle the planet a colony or outpost
// ship was sent to (report string 6 or 7); -1 for no.
func (g *Game) preselectedPlanet(st, pl, msg int) (int, error) {
	rs, err := g.reportStrings()
	if err != nil {
		return 0, err
	}
	yes, err := g.confirmationBox(cSprintf(rs[msg], g.planetName(pl)))
	if err != nil {
		return 0, err
	}
	if !yes {
		return -1, nil
	}
	if g.canChangeStarName(st) && g.settings[6] == 0 {
		if _, err := g.changeStarNamePopup(starNamePopupX, starNamePopupY, st, false); err != nil {
			return 0, err
		}
	}
	return pl, nil
}

// colonyShipAutoColonization is Colony_Ship_Auto_Colonization_ (0x9952d):
// the planet a colony or outpost ship was sent to settle, or -1.
func (s *State) colonyShipAutoColonization(i int) int {
	d := &s.Ships[i].Design
	if d.Weapons[1].Weapon == 1 {
		return int(d.Weapons[0].Weapon)
	}
	return -1
}

// canChangeStarName is Can_Change_Star_Name_ (0x9222a): nobody has a colony
// at the star yet.
func (g *Game) canChangeStarName(st int) bool {
	s := &g.state
	for _, pl := range s.Stars[st].Planets {
		if pl >= 0 && s.Planets[pl].Colony >= 0 {
			return false
		}
	}
	return true
}

// newOutpostSelectionPopup is New_Outpost_Selection_Popup_ (0xc8d49).
func (g *Game) newOutpostSelectionPopup(st int) (int, error) {
	g.mp.selectionState = selectOutpost
	defer func() { g.mp.selectionState = selectColony }()
	return g.newColonySelectionPopup(st, false)
}

// newColonySelectionPopup is New_Colony_Selection_Popup_ (0xc8db8): the
// system display of star st; the planet clicked (and confirmed), or -1
// for the close button.
func (g *Game) newColonySelectionPopup(st int, usingShip bool) (int, error) {
	e, mp := g.e, &g.mp
	mp.agentSelecting = true // port-only, for the remote control's /state
	defer func() { mp.agentSelecting = false }()
	mp.lastScanned = -1
	mp.usingColonyShip = usingShip
	mp.sysDispStar = st
	e.Text().SetStyleShadow(1, font.Colors{0x9f, 0xa0, 0xa1, 0xa2}, font.ShadowDown)
	g.setGSpecialValue(st)
	mp.specialFrame, mp.specialAnim = nil, nil
	mp.specialDesc = ""
	bg, err := g.cached(buffer0LBX, 0x49)
	if err != nil {
		return 0, err
	}
	if err := g.doPopupSetup(st, bg); err != nil {
		return 0, err
	}
	e.DeactivateHelpList()
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawSelectionPopup, 1)
	g.ericPushFields()
	g.clearSysDispFields()
	if err := g.addSelectionFields(); err != nil {
		return 0, err
	}
	g.setPlanetSelectionPopupHelpList()
	if err := g.drawSelectionPopup(); err != nil {
		return 0, err
	}
	if err := e.TogglePages(); err != nil {
		return 0, err
	}
	mp.showing = "selection"
	defer func() { mp.showing = "" }()
	result, done := 0, false
	for !done {
		if err := e.ReleaseTime(2); err != nil {
			return 0, err
		}
		e.MarkTime()
		in, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		sc, err := e.ScanInput()
		if err != nil {
			return 0, err
		}
		if result, err = g.evaluateSelectionInput(in, sc, &done); err != nil {
			return 0, err
		}
		if err := g.drawSelectionPopup(); err != nil {
			return 0, err
		}
		if err := e.TogglePages(); err != nil {
			return 0, err
		}
	}
	if g.canChangeStarName(st) && g.settings[6] == 0 && int16(result) != -1 {
		if _, err := g.changeStarNamePopup(starNamePopupX, starNamePopupY, st, false); err != nil {
			return 0, err
		}
	}
	g.doPopupCleanup()
	return result, nil
}

// setGSpecialValue is Set_G_Special_Value_ (0xc8d56): a special newly seen
// at the star shows in the popup and is marked seen.
func (g *Game) setGSpecialValue(st int) {
	s, mp := &g.state, &g.mp
	S := &s.Stars[st]
	pn := uint(s.PlayerNum)
	if S.SpecialPopup>>pn&1 == 0 {
		mp.special = 0
		return
	}
	mp.special = g.drawSpecial(st, int(pn))
	S.SpecialPopup &^= 1 << pn
	if int8(S.Special) > '1' {
		S.Special = 0
	}
}

// addSelectionFields is Add_Selection_Fields_ (0xc87c1): the system
// display's fields (Wrapper_For_Informational_System_Fields_; its close
// button is the cancel) and a hidden field over the screen.
func (g *Game) addSelectionFields() error {
	mp := &g.mp
	g.sd.withoutButton = true
	if err := g.initSystemDisplay(mp.sysDispStar); err != nil {
		return err
	}
	var drag, whole int
	if err := g.addSystemDisplayPopupFields(mp.sysDispStar, mp.x, mp.y, 0, 0, &drag, &mp.closeField, &whole); err != nil {
		return err
	}
	return g.addOfficerGainsLevelFields()
}

// drawSelectionPopup is Draw_Selection_Popup_ (0xc87de).
func (g *Game) drawSelectionPopup() error {
	e, mp, s := g.e, &g.mp, &g.state
	t := e.Text()
	t.SetStyle(4, g.exeColors(mainScrFontColors))
	if mp.fullDraw || g.fullDrawScreen {
		if err := g.mainpupsTestedExecuteAutofunction(); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		mp.fullDraw = false
	}
	title := 0
	switch mp.selectionState {
	case selectColony:
		title = 0x1cd
		if mp.usingColonyShip {
			title = 0x1ce
		}
	case selectOutpost:
		title = 0x1cb
	case selectTroops:
		title = 0x1cf
	case selectArtificial:
		title = 0x1cc
	}
	es, err := g.eStrings()
	if err != nil {
		return err
	}
	if title != 0 {
		text := cSprintf(es[title], save.CString(s.Stars[mp.starN].Name[:]))
		if err := g.drawSystemDisplayPopup(mp.sysDispStar, true, text, mp.x, mp.y, 0, 0); err != nil {
			return err
		}
	}
	t.SetStyle(1, g.exeColors(mainScrFontColors))
	if mp.special == 0 || mp.special == 8 {
		return nil
	}
	desc, err := g.systemSpecialDesc(mp.special)
	if err != nil {
		return err
	}
	anim, err := g.systemSpecialAnim(int(mp.starN), mp.special)
	if err != nil {
		return err
	}
	name, err := g.eStringAt(planetSpecialNames + 4*uint32(mp.special))
	if err != nil {
		return err
	}
	return g.drawSpecialInfoPopup(mp.x, mp.y+0x32, anim, name, desc)
}

// evaluateSelectionInput is Evaluate_Selection_Input_ (0xc8be0).
func (g *Game) evaluateSelectionInput(in, sc int, done *bool) (int, error) {
	mp := &g.mp
	gx, gy := mp.x, mp.y
	r := 0
	switch {
	case mp.special == 0:
		if int16(in) == int16(mp.closeField) {
			r, *done = -1, true
		} else {
			var err error
			if r, err = g.checkSystemDisplayFieldsSelection(in, sc, done); err != nil {
				return 0, err
			}
		}
	case mp.special == 8:
		mp.special = 0
		var who int16
		mp.levelState = 2
		if g.maroonedHeroAtStarCanJoin(int(mp.starN), &who, int(g.state.PlayerNum)) {
			mp.levelState = 1
		}
		if err := g.officerGainsLevelPopup(int(mp.starN), int(who)); err != nil {
			return 0, err
		}
		mp.levelState = 0
		mp.fullDraw = true
		mp.x, mp.y = gx, gy
		if err := g.resetSelectionPopup(); err != nil {
			return 0, err
		}
	case in != 0:
		mp.special = 0
		mp.fullDraw = true
	}
	if g.fullDrawScreen {
		mp.fullDraw = true
	}
	return r, nil
}

// resetSelectionPopup is the field and drawing reset Evaluate_ and
// Check_System_Display_Fields_Selection_ repeat after a box.
func (g *Game) resetSelectionPopup() error {
	e := g.e
	g.ericPopFields()
	g.ericPushFields()
	g.clearSysDispFields()
	if err := g.addSelectionFields(); err != nil {
		return err
	}
	g.setPlanetSelectionPopupHelpList()
	e.V.CopyBackToOff()
	if err := g.drawSelectionPopup(); err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawSelectionPopup, 1)
	return nil
}

// checkSystemDisplayFieldsSelection is
// Check_System_Display_Fields_Selection_ (0xc8981): a click on a planet
// the mode allows (colony: an unsettled planet, outposts too; outpost: no
// colony at all), confirmed for colonies, ends the popup.
func (g *Game) checkSystemDisplayFieldsSelection(in, sc int, done *bool) (int, error) {
	s, mp, sd := &g.state, &g.mp, &g.sd
	ret, _, clk, hovS, _, icon := g.scanSystemDisplayFields(sc, in)
	sd.shipScanned = int16(hovS)
	if icon > -1 {
		sd.iconScanned = int16(icon)
	}
	if ret != 0 || clk < 0 {
		return -1, nil
	}
	pl := int(s.Stars[mp.sysDispStar].Planets[clk])
	P := &s.Planets[pl]
	settled := P.Colony != -1 && s.Colonies[P.Colony].IsOutpost == 0
	bad := 0
	switch mp.selectionState {
	case selectColony:
		if P.Type != 3 || settled {
			bad = 0x236
		}
	case selectOutpost:
		if P.Colony != -1 {
			bad = 0x236
		}
	case selectTroops:
		if !(settled && int16(s.Colonies[P.Colony].Owner) == s.PlayerNum) {
			bad = 0x23b
		}
	}
	if bad == 0 {
		ok := true
		switch mp.selectionState {
		case selectArtificial:
			if P.Type == 3 {
				ok, bad = false, 0x235
			}
		case selectColony:
			var err error
			if ok, err = g.newColonyConfirmationBox(pl); err != nil {
				return 0, err
			}
		}
		if ok {
			*done = true
			return pl, nil
		}
		if bad == 0 {
			return -1, g.resetSelectionPopup()
		}
	}
	es, err := g.eStrings()
	if err != nil {
		return 0, err
	}
	if err := g.messageBox(es[bad]); err != nil {
		return 0, err
	}
	mp.fullDraw = true
	if err := g.resetSelectionPopup(); err != nil {
		return 0, err
	}
	return -1, g.resetSelectionPopup()
}

// newColonyConfirmationBox is New_Colony_Confirmation_Box_ (0xc9fc4): the
// planet's food, industry and research per worker, penalties and maximum
// population, then yes or no.
func (g *Game) newColonyConfirmationBox(pl int) (bool, error) {
	s := &g.state
	g.e.DeactivateHelpList() // Set_New_Colony_Confirmation_Box_Help_List_ (0xcdbd9)
	g.e.SetHelpList(g.helpListAt(0x183506, 1))
	info := g.computePotentialColonyInfo(int(s.PlayerNum), pl)
	es, err := g.eStrings()
	if err != nil {
		return false, err
	}
	a, b := cStringAt(g.exe, colourCodeA), cStringAt(g.exe, colourCodeB)
	msg := cSprintf(es[0xd1], g.planetName(pl))
	if food := int(info[4]); food%2 == 0 {
		msg += cSprintf(es[0x65], a, b, food/2)
	} else {
		msg += cSprintf(es[0x64], a, b, food/2, food%2)
	}
	msg += cSprintf(es[0x6b], a, b, int(info[5]))
	msg += cSprintf(es[0x73], a, b, int(info[6]))
	msg += cSprintf(es[0x6c], a, b, int(info[1]))
	msg += cSprintf(es[0x78], a, b, int(info[3]))
	msg += cSprintf(es[0x6e], a, b, int(g.planetMaxPopulationForPlayer(pl, int(s.PlayerNum))))
	ok, err := g.confirmationBox(msg)
	g.e.DeactivateHelpList()
	return ok, err
}

// computePotentialColonyInfo is Compute_Potential_Colony_Info_ (0xe5430):
// seven words for player p settling planet pl: population pick, gravity
// maintenance, -, worker penalty, food per farmer (doubled), industry and
// research per worker.
func (g *Game) computePotentialColonyInfo(p, pl int) [7]int16 {
	s := &g.state
	P, pc := &s.Players[p], &s.Planets[pl]
	var o [7]int16
	b := func(addr uint32) int16 { return int16(g.exe.bytes(addr, 1)[0]) }
	o[0] = int16(P.Picks[save.PickPopulation])
	if P.Picks[save.PickTolerant] == 0 {
		o[1] += b(potentialGravity + uint32(pc.Climate))
	}
	switch pc.Gravity {
	case 0:
		if P.Picks[save.PickLowG] == 0 {
			o[3] = -25
		}
	case 1:
		if P.Picks[save.PickLowG] != 0 {
			o[3] = -25
		}
	case 2:
		if P.Picks[save.PickHighG] == 0 {
			o[3] = -50
		}
	}
	o[4] = int16(pc.FoodPerFarmer) * 2
	if o[4] == 0 && P.AppStatus[0x1d] == techKnown {
		o[4] = 2
	}
	if o[4] > 0 {
		o[4] += int16(P.Picks[save.PickFarming])
		if P.Picks[save.PickAquatic] != 0 {
			switch c := uint32(pc.Climate); c {
			case 4, 6:
				o[4] += 2 * (b(climateFood+8) - b(climateFood+c))
			case 5, 8:
				o[4] += 2 * (b(climateFood+9) - b(climateFood+c))
			}
		}
		if o[4] <= 0 {
			o[4] = 1
		}
	}
	o[5] = b(potentialMinerals + uint32(pc.Minerals))
	if P.AppStatus[0x6c] == techKnown {
		o[5]++
	}
	o[5] += int16(P.Picks[save.PickIndustry])
	if o[5] <= 0 {
		o[5] = 1
	}
	o[6] = 3 + int16(P.Picks[save.PickScience])
	if P.AppStatus[0x54] == techKnown {
		o[6]++
	}
	switch pc.Special {
	case 0xa:
		o[6] += 2
	case 0xb:
		o[6] += 5
	}
	return o
}

// setPlanetSelectionPopupHelpList is Set_Planet_Selection_Popup_Help_List_
// (0xcdc18): one entry of _planet_selection_popup_help_list (0x18352e).
func (g *Game) setPlanetSelectionPopupHelpList() {
	g.e.DeactivateHelpList()
	g.e.SetHelpList(g.helpListAt(0x18352e, 1))
}
