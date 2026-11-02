package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/save"
)

// Input of the colony screen (COLONY.C). Spec: docs/screens/colony-screen.md,
// "Input".

// restoreColonyControl is Restore_Control_ (0xbb0c4) and the copies of it
// inlined after every popup: the whole screen again.
func (g *Game) restoreColonyControl() {
	g.fullDrawScreen = false
	g.e.DeactivateAutoFunction()
	g.col.fullDraw = true
	g.e.AssignAutoFunction(g.drawColonyScreen, 1)
	g.setColonyScreenHelpList()
}

// colonyEvaluateInput is Evaluate_Input_ (0xc02f9); false leaves the screen
// loop (another colony or screen).
func (g *Game) colonyEvaluateInput(r *int, scan int) (bool, error) {
	ok, err := g.checkSystemDisplayFields(r, scan)
	if err != nil || g.cscr.display == 2 || !ok {
		return ok, err
	}
	if ok, err = g.evaluateScreenInput(r); err != nil {
		return false, err
	}
	for _, fn := range []func(*int, int) error{g.checkJobFields, g.checkMilitaryFields, g.checkSatelliteFields, g.checkBldgFields} {
		if err := fn(r, scan); err != nil {
			return false, err
		}
	}
	return ok, nil
}

// evaluateScreenInput is Evaluate_Screen_Input_ (0xbfdee): the buttons and
// hot keys. "<" (E 0x8b) steps to the next colony and ">" to the previous.
func (g *Game) evaluateScreenInput(r *int) (bool, error) {
	e, s, cs := g.e, &g.state, &g.cscr
	f := &cs.fields
	ci := g.colonyHandle
	c := &s.Colonies[ci]
	switch *r {
	case f[cfOccupation]:
		if err := g.occupationPolicyPopup(); err != nil {
			return false, err
		}
		g.restoreColonyControl()
		g.colCalcWrapper(ci)
		return true, nil
	case f[cfLeaders]:
		g.screenData[3], g.screenData[4] = 1, int16(cs.starHandle)
		g.returnScreen = g.currentScreen
		g.currentScreen = ScreenLeaders
		return false, nil
	case f[cfDisplayOn]:
		if cs.display == 0 {
			cs.display = 1
			g.col.fullDraw = true
			return true, nil
		}
	case f[cfDisplayOff]:
		if cs.display == 1 {
			cs.display = 0
			cs.scanPlanet = -1
			g.col.fullDraw = true
			return true, nil
		}
	}
	switch *r {
	case f[cfTransport]:
		dst, err := g.xportPopup()
		if err != nil {
			return false, err
		}
		g.restoreColonyControl()
		if dst != -1 {
			if err := g.sendCluster(int(dst), -1); err != nil {
				return false, err
			}
		}
		g.restoreColonyControl()
		g.colCalcWrapper(ci)
		return true, nil
	case f[cfAutobuild]:
		c.AutoBuild = b2u8(c.AutoBuild == 0)
		g.restoreColonyControl()
		return true, nil
	case f[cfBuy]:
		if err := g.testedColonyBuysOutright(ci); err != nil {
			return false, err
		}
		e.ClearFields()
		if err := g.colonyAddFields(); err != nil {
			return false, err
		}
		g.restoreColonyControl()
		*r = 0
		g.clearMouseBuffer()
		e.SetInputDelay(3)
		return true, nil
	case f[cfCheat]:
		if !cs.cheats {
			g.colCalcWrapper(ci)
			return true, nil
		}
		if err := g.doColonyCheats(); err != nil {
			return false, err
		}
		g.restoreColonyControl()
		return true, nil
	case f[cfMorale], f[cfBC], f[cfFood], f[cfIndustry], f[cfResearch]:
		var err error
		switch *r {
		case f[cfMorale]:
			err = g.showMorale(ci)
		case f[cfBC]:
			err = g.showBCProduction(ci)
		case f[cfFood]:
			err = g.showFoodProduction(ci)
		case f[cfIndustry]:
			err = g.showIndustryProduction(ci)
		default:
			err = g.showResearchProduction(ci)
		}
		if err != nil {
			return false, err
		}
		g.restoreColonyControl()
		return true, nil
	case f[cfBuild]:
		g.returnScreen = g.currentScreen
		g.currentScreen = ScreenBuildQueue
		return false, nil
	case f[cfPrev]:
		return g.stepColony(0), nil
	case f[cfToggle]:
		// Check_Release_Version_ is 1 in this build.
		cs.cheats = !cs.cheats
		g.setPlayerCheatedFlag(int(s.PlayerNum), 6)
		g.moxPlaySound(0x33) // 0xc013e
		return true, nil
	case f[cfCrunch]:
		c.Production = int16(g.colonyProductCost(c, c.Queue[0]))
		g.setPlayerCheatedFlag(int(s.PlayerNum), 6)
		return true, nil
	case f[cfNext]:
		return g.stepColony(1), nil
	case f[cfEsc]:
		*r = f[cfReturn]
		return true, nil
	}
	if g.fullDrawScreen {
		g.restoreColonyControl()
	}
	return true, nil
}

// stepColony moves to Get_Next_Colony_'s colony, if another.
func (g *Game) stepColony(back int) bool {
	n := g.getNextColony(g.colonyHandle, back)
	if n == g.colonyHandle {
		return true
	}
	g.setColonyScreenToNewColony(n)
	return false
}

// setPlayerCheatedFlag is Set_Player_Cheated_Flag_ (0x8a552).
func (g *Game) setPlayerCheatedFlag(p int, flag byte) {
	pl := &g.state.Players[p]
	pl.Cheated, pl.CheatFlag = 1, flag
}

// getNextColony is Get_Next_Colony_ (0xc01fc): the next (back 0) or
// previous colony of the same owner, not an outpost. Going back from 0 it
// passes the slot after the last colony.
func (g *Game) getNextColony(ci, back int) int {
	s := &g.state
	u := ci
	for {
		if back == 0 {
			u = int(int16(u+1)) % int(s.NumColonies)
		} else {
			u--
			if int16(u) < 0 {
				u = int(s.NumColonies)
			}
		}
		c := &s.Colonies[int16(u)]
		if (s.Colonies[ci].Owner == c.Owner && c.IsOutpost == 0) || int16(u) == int16(ci) {
			return int(int16(u))
		}
	}
}

// setColonyScreenToNewColony is Set_Colony_Screen_To_New_Colony_ (0xc0257):
// the screen restarts on colony n.
func (g *Game) setColonyScreenToNewColony(n int) {
	s := &g.state
	g.state.fixColonyQueue(g.colonyHandle)
	if c := &s.Colonies[g.colonyHandle]; c.Queue[0] == -1 {
		c.Queue[0] = -2
	}
	star, orbit := g.colonyPlanetStar(n)
	g.screenData[0], g.screenData[1], g.screenData[2] = int16(star), int16(orbit), int16(n)
	if err := g.resetFieldMode(); err != nil {
		g.logOnce(err.Error())
	}
	g.e.ClearFields()
	g.e.DeactivateAutoFunction()
	g.col.fullDraw = true
	g.setLastColonySelected(n, int(s.PlayerNum))
}

// setLastColonySelected is Set_Last_Colony_Selected_ (0x786e9).
func (g *Game) setLastColonySelected(ci, p int) {
	s := &g.state
	c := &s.Colonies[ci]
	if c.IsOutpost != 0 {
		return
	}
	pl := &s.Planets[c.Planet]
	s.Stars[pl.Star].LastOrbit[int16(p)] = int8(pl.Orbit)
}

// checkSystemDisplayFields is Check_System_Display_Fields_ (0xbf802): the
// system display's planets (another colony, or where picked-up colonists
// go) and summaries, the small galaxy map, and the planet description.
func (g *Game) checkSystemDisplayFields(r *int, scan int) (bool, error) {
	s, cs := &g.state, &g.cscr
	switch cs.display {
	case 0:
		if *r == cs.sysDispF[0] {
			cs.display = 1
			g.col.fullDraw = true
			return true, nil
		}
		for o := 0; o < 5; o++ {
			pl := int(s.Stars[cs.sysStar].Planets[o])
			switch {
			case *r == cs.sysPlanetF[o] && cs.fieldMode == 1:
				if err := g.sendCluster(int(s.Planets[pl].Colony), -1); err != nil {
					return false, err
				}
				g.restoreColonyControl()
			case *r == cs.sysPlanetF[o]:
				ci := int(s.Planets[pl].Colony)
				if ci == -1 || s.Colonies[ci].IsOutpost == 1 || ci == g.colonyHandle || int16(s.Colonies[ci].Owner) != s.PlayerNum {
					return true, nil
				}
				g.screenData[0], g.screenData[1], g.screenData[2] = int16(cs.sysStar), int16(o), int16(ci)
				g.state.fixColonyQueue(g.colonyHandle)
				if c := &s.Colonies[g.colonyHandle]; c.Queue[0] == -1 {
					c.Queue[0] = -2
				}
				if err := g.resetFieldMode(); err != nil {
					return false, err
				}
				g.e.ClearFields()
				g.e.DeactivateAutoFunction()
				g.col.fullDraw = true
				g.setLastColonySelected(ci, int(s.PlayerNum))
				return false, nil
			case *r == cs.sysSummF[o]:
				if s.Planets[pl].Type == 3 {
					cs.display = 2
					cs.planetDisp = int16(pl)
					return true, nil
				}
			case scan == cs.sysPlanetF[o]:
				cs.scanPlanet = int16(pl)
			}
		}
		return true, nil
	case 1:
		res, hov, clk := g.scanGalaxyMapFields(scan, *r)
		switch {
		case res == 4:
			cs.scanStar = int16(hov)
			return true, nil
		case res == 2:
			gm := &g.ms.galaxyMap
			cx := int32(0x13c4) / int32(gm.width)
			cy := int32(0xfa0) / int32(gm.height)
			ms := int32(s.MaxMapScale)
			mx := (int32(int16(cs.mapGridX)) * ms / 10) * cx / 10
			my := (int32(int16(cs.mapGridY)) * ms / 10) * cy / 10
			g.centerMap(int16(mx), int16(my))
			g.col.fullDraw = true
			return true, nil
		case res == 0:
			cs.display = 0
			cs.scanPlanet = -1
			g.col.fullDraw = true
			cs.sysStar = clk
			return true, nil
		}
		return true, nil
	case 2:
		msg := g.planetDescription(int(cs.planetDisp))
		cs.display = 0
		cs.planetDisp = -1
		if err := g.messageBox(msg); err != nil {
			return false, err
		}
		g.restoreColonyControl()
	}
	return true, nil
}

// checkJobFields is Check_Job_Fields_ (0xbf627): pick up colonists from a
// job row, put picked-up ones down in a row, hover a colonist.
func (g *Game) checkJobFields(r *int, scan int) error {
	cs, cg := &g.cscr, &g.col
	g.setColonyFontTo(3)
	cg.scannedPop = -1
	switch cs.fieldMode {
	case 0:
		for j := 0; j < 3; j++ {
			if *r == cs.jobF[j] {
				p, err := g.colonyPopStuff(j, 3)
				if err != nil {
					return err
				}
				if p != -1 {
					cs.fieldMode = 1
					if err := g.getCluster(g.colonyHandle, int(p)); err != nil {
						return err
					}
					cg.fullDraw = true
					g.colCalcWrapper(g.colonyHandle)
					*r = 0
				}
			}
			if scan == cs.jobF[j] {
				cg.colonyN = int16(g.colonyHandle)
				p, err := g.colonyPopStuff(j, 4)
				if err != nil {
					return err
				}
				cg.scannedPop = p
			}
		}
	case 1:
		for j := 0; j < 3; j++ {
			if *r == cs.jobF[j] {
				if err := g.sendCluster(g.colonyHandle, j); err != nil {
					return err
				}
				g.restoreColonyControl()
				*r = 0
			}
		}
	}
	return nil
}

// checkMilitaryFields is Check_Military_Fields_ (0xbf72c): the troop popup;
// hovering names the troops. Its field mode 2 branch is dead.
func (g *Game) checkMilitaryFields(r *int, scan int) error {
	cs := &g.cscr
	cs.scannedMilitary = -1
	for k := 0; k < 2; k++ {
		if *r == cs.militaryF[k] {
			if err := g.troopPopup(); err != nil {
				return err
			}
			g.restoreColonyControl()
			*r = 0
		}
		if scan == cs.militaryF[k] {
			cs.scannedMilitary = int16(k)
		}
	}
	return nil
}

// sellBuilding is the sale both Check_Satellite_Fields_ and
// Check_Bldg_Fields_ make: one per colony and turn, for half the cost.
func (g *Game) sellBuilding(id int16, name string, rubble bool) error {
	s := &g.state
	c := &s.Colonies[g.colonyHandle]
	if uint32(c.SoldStardate) == s.Stardate {
		if err := g.messageBox(g.eStr(0x243)); err != nil {
			return err
		}
		g.fullDrawScreen = false
		return nil
	}
	value := g.buildingCost(int32(id)) / 2
	ok, err := g.confirmationBox(cSprintf(g.eStr(0x107), name, int(int16(value))))
	if err != nil || !ok {
		return err
	}
	c.SoldStardate = uint16(s.Stardate)
	if rubble {
		if col, row, found := g.getBldgCR(int32(id)); found {
			g.col.fullDraw = true
			g.cscr.surf.bldgs[col][row] = 8
		}
		g.removeBuilding(g.colonyHandle, int(id))
	} else {
		g.removeBuilding(g.colonyHandle, int(id))
		g.removeSatelliteFromArray(id)
	}
	g.colonyCalculation(g.colonyHandle)
	s.Players[s.PlayerNum].BC += int32(int16(value))
	return nil
}

// checkSatelliteFields is Check_Satellite_Fields_ (0xbe09c): sell a
// satellite, its description on a right click, hover.
func (g *Game) checkSatelliteFields(r *int, scan int) error {
	cs := &g.cscr
	cs.scannedSat = -1
	for i := 0; i < 10; i++ {
		id := cs.surf.satellites[i]
		switch {
		case *r == cs.satelliteF[i]:
			name, err := g.buildingName(int(id))
			if err != nil {
				return err
			}
			if err := g.sellBuilding(id, name, false); err != nil {
				return err
			}
			g.restoreColonyControl()
		case -*r == cs.satelliteF[i]:
			if err := g.buildingDescriptionBox(int(id), false); err != nil {
				return err
			}
		}
		if scan == cs.satelliteF[i] {
			cs.scannedSat = cs.surf.satellites[i]
		}
	}
	return nil
}

// buildingDescriptionBox is the right click on a building: Text_Box_ with
// its name and Get_App_Description_String_ of its application.
func (g *Game) buildingDescriptionBox(id int, crName bool) error {
	name, err := g.buildingName(id)
	if crName {
		name, err = g.crBuildingName(int16(id))
	}
	if err != nil {
		return err
	}
	app := int(g.exeI16(uint32(0x17eb43 + id*0x13)))
	desc, err := g.appDescriptionString(app)
	if err != nil {
		return err
	}
	return g.textBox(&name, desc)
}

// appDescriptionString is Get_App_Description_String_ (0xafbcb): the help
// file's text of record app.
func (g *Game) appDescriptionString(app int) (string, error) {
	rec, err := g.helpRecord(app)
	if err != nil {
		return "", err
	}
	return save.CString(rec.text), nil
}

// checkBldgFields is Check_Bldg_Fields_ (0xbe530): a building is clicked,
// right-clicked or hovered by its field or, through the whole-screen field,
// by its floor polygon. Houses, rubble and building 9 cannot be sold.
func (g *Game) checkBldgFields(r *int, scan int) error {
	cs := &g.cscr
	cs.scannedC, cs.scannedR = -1, -1
	px, py, _ := g.e.Sys.Mouse()
	whole := cs.fields[cfWhole]
	for c := 0; c < 6; c++ {
		for row := 0; row < 6; row++ {
			f := cs.bldgF[c][row]
			inPoly := func() bool {
				return g.xyInBldgPoly(px, py, c, row) && cs.surf.bldgs[c][row] != 0
			}
			id := int16(cs.surf.bldgs[c][row])
			if *r == f || (*r == whole && inPoly()) {
				if isHouse(int32(id)) || id == 8 || id == 9 {
					return nil
				}
				name, err := g.crBuildingName(id)
				if err != nil {
					return err
				}
				if err := g.sellBuilding(id, name, true); err != nil {
					return err
				}
				g.restoreColonyControl()
				return nil
			}
			if -*r == f || (-*r == whole && inPoly()) {
				if isHouse(int32(id)) || id == 8 {
					return nil
				}
				if err := g.buildingDescriptionBox(int(id), true); err != nil {
					return err
				}
				g.restoreColonyControl()
				return nil
			}
			if scan == f || inPoly() {
				cs.scannedC, cs.scannedR = int16(c), int16(row)
				return nil
			}
		}
	}
	return nil
}

// xyInBldgPoly is XY_In_Bldg_Poly_ (0xbc768): whether (x, y) lies inside the
// floor of cell (c, r).
func (g *Game) xyInBldgPoly(x, y, c, r int) bool {
	var xs, ys [4]int32
	corners := [4][2]int{{c, r}, {c + 1, r}, {c + 1, r + 1}, {c, r + 1}}
	for i, k := range corners {
		xs[i] = int32(g.crToXY(k[0], k[1], 0))
		ys[i] = int32(g.crToXY(k[0], k[1], 1))
	}
	n := 0
	for i := 0; i < 4; i++ {
		j, k := (i+1)%4, (i+2)%4
		if pointsOnSameSideOfLine(int32(int16(x)), int32(int16(y)), xs[i], ys[i], xs[j], ys[j], xs[k], ys[k]) > 0 {
			n++
		}
	}
	return n == 4
}

// removeSatelliteFromArray is Remove_Satellite_From_Array_ (0xbc27f): the
// sold satellite's slot is cleared and the surface rebuilt.
func (g *Game) removeSatelliteFromArray(id int16) {
	sats := &g.cscr.surf.satellites
	for i := 0; i < 10; i++ {
		if sats[i] != id {
			continue
		}
		sats[i] = 0
		// Slot 9 compares the word after the array (_planet_cache_entry).
		if i+1 < 10 && sats[i+1] != 0 {
			// The original copies the next slot nine times over this one.
			sats[i] = sats[i+1]
			sats[9] = -1
		}
	}
	g.col.fullDraw = true
	if err := g.makeBldgArrayForColony(g.colonyHandle, false); err != nil {
		g.logOnce(err.Error())
	}
}

// addColSysDispFields is Add_Col_Sys_Disp_Fields_ (0xc4ed1): each planet's
// picture and the summary beside it.
func (g *Game) addColSysDispFields(x, y, xEnd int) error {
	s, cs := &g.state, &g.cscr
	sysdi := cStringAt(g.exe, colsysdiLBX)
	for o := 0; o < 5; o++ {
		pl := int(s.Stars[cs.sysStar].Planets[o])
		if pl == -1 {
			cs.sysPlanetF[o] = engine.NoField
			continue
		}
		P := &s.Planets[pl]
		p, err := g.cached(sysdi, int(P.Climate)*5+0xb+int(P.Size))
		if err != nil {
			return err
		}
		w, h := int(int16(p.Width)), int(int16(p.Height))
		cy := int(int16(y + 0xe + o*0x18))
		x2 := int(int16(x + 0xf + w/2))
		y1, y2 := int(int16(cy-h/2)), int(int16(cy+h/2))
		f, err := g.e.AddHiddenField(int(int16(x+0xf-w/2)), y1, x2, y2, g.eStr(0xc), 0)
		if err != nil {
			return err
		}
		cs.sysPlanetF[o] = f
		if f, err = g.e.AddHiddenField(x2+2, y1, xEnd-2, y2, g.eStr(0xc), 0); err != nil {
			return err
		}
		cs.sysSummF[o] = f
	}
	return nil
}

// doInformationalAndDecisionPopups is Do_Informational_And_Decision_Popups_
// (0xc035e): the occupation policy of a conquered colony, then what was
// last built or why the colony is new.
func (g *Game) doInformationalAndDecisionPopups() error {
	s := &g.state
	ci := g.colonyHandle
	c := &s.Colonies[ci]
	if c.Occupation == 4 && (int16(g.sumColonists(ci, -1, -1, -1, -1, 1)) > 0 || g.colonyHasNatives(ci)) {
		if err := g.occupationPolicyPopup(); err != nil {
			return err
		}
		g.colCalcWrapper(ci)
		g.restoreColonyControl()
	}
	last := c.LastBuilt
	if last == -1 {
		return nil
	}
	name := g.planetName(int(c.Planet))
	var msg string
	switch last {
	case -0x27:
		msg = cSprintf(g.eStr(0x40), name)
	case -0x28:
		msg = cSprintf(g.eStr(0x41), name)
	case -0x29:
		msg = cSprintf(g.eStr(0x42), name)
	default:
		msg = cSprintf(g.eStr(0x39), name, g.colbldgSelectionName(last))
	}
	if err := g.messageBox(msg); err != nil {
		return err
	}
	g.restoreColonyControl()
	c.LastBuilt = -1
	return nil
}

// colonyHasNatives is Colony_Has_Natives_ (0xbc932).
func (g *Game) colonyHasNatives(ci int) bool {
	c := &g.state.Colonies[ci]
	for k := 0; k < int(c.PopCount); k++ {
		if c.Colonists[k]&0xf == 8 {
			return true
		}
	}
	return false
}

// occupationPolicyPopup is Occupation_Policy_Popup_ (0xcd969): unless the
// owner's government is 4 or 5, ask whether to annihilate the conquered
// (occupation 0), else 3.
func (g *Game) occupationPolicyPopup() error {
	e, s := g.e, &g.state
	pl := s.Stars[g.screenData[0]].Planets[g.screenData[1]]
	c := &s.Colonies[s.Planets[pl].Colony]
	colors := g.exe.bytes(0xcd8e1, 4)
	var fc font.Colors
	copy(fc[:], colors) // the rest of the array is stale stack in the original
	e.Text().SetStyleShadow(1, fc, font.ShadowDown)
	e.V.CopyOffToBack()
	e.DeactivateHelpList()
	e.SetHelpList(g.helpListAt(occupationPopupHelpList, 1))
	saved := e.GetAutoFunction()
	e.DeactivateAutoFunction()
	value := uint8(3)
	if gov := s.Players[c.Owner].Picks[save.PickGovernment]; gov != 4 && gov != 5 {
		yes, err := g.confirmationBox(g.eStr(0xb4))
		if err != nil {
			return err
		}
		if yes {
			value = 0
		}
	}
	c.Occupation = value
	e.DeactivateHelpList()
	e.PutAutoFunction(saved)
	e.V.CopyBackToOff()
	return nil
}
