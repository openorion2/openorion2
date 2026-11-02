package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The colony screen (screen 1, Colony_Screen_): MOX\OTHERS\COLONY.C with the
// COLSYSDI.C system display and the COLXPORT.C transport popup. Spec:
// docs/screens/colony-screen.md.

// Addresses of COLONY.C's data.
const (
	colonyScreenHelpList    = 0x1831f0 // _colony_screen_help_list, 0x11 entries
	transportPopupHelpList  = 0x18343e // _colony_xport_pup_help_list, 7
	occupationPopupHelpList = 0x183484 // _occupation_policy_pup_help_list, 1
	colpupsLBX              = 0x1829b6
	colonyLBX               = 0x182979
	planetsLBX              = 0x1829c2
	colsysdiLBX             = 0x182a21
	shipRemapColors         = 0xba6fb // Remap_Draw_Ship_Centered_'s 31 colours
	shipSegRemapColors      = 0xba71a // Remap_Draw_Ship_Seg_N_Centered_'s
	bldgFieldOrder          = 0xba784 // Add_Bldg_Fields_'s 36 (c, r) pairs
)

// Indices of _colony_fields (0x182acf, 21 words).
const (
	cfWhole = iota
	cfReturn
	cfEsc
	cfBuy
	cfBuild
	cfCheat
	cfPrev
	cfNext
	cfOccupation
	cfDisplayOn  // 0x182ae1, never added
	cfDisplayOff // 0x182ae3, never added
	cfTransport
	cfMorale
	cfBC
	cfFood
	cfIndustry
	cfResearch
	cfLeaders
	cfAutobuild
	cfCrunch
	cfToggle
	cfCount
)

// colonyScreenState is COLONY.C's state.
type colonyScreenState struct {
	surf colonySurface

	valid      bool  // _valid_colony
	display    byte  // _drawing_display: 0 system, 1 galaxy map, 2 planet text
	fieldMode  byte  // _field_mode: 1 while colonists are picked up
	usingPal   bool  // _using_colony_screen_palette
	starHandle int   // _star_handle
	orbitN     int   // _orbit_n
	planet     int   // _planet_handle
	sysStar    int   // _sys_disp_star_handle
	planetDisp int16 // _planet_display_handle
	scanStar   int16 // _g_scanned_star
	scanPlanet int16 // _g_scanned_planet
	savedScr   int   // _colony_saved_screen

	scannedSat      int16 // _scanned_satellite
	scannedMilitary int16 // _scanned_military
	scannedC        int16 // _scanned_bldg_C
	scannedR        int16 // _scanned_bldg_R

	fields      [cfCount]int
	sysPlanetF  [5]int // _sys_disp_planet_fields (0x19f9ca)
	sysDispF    [3]int // _sys_disp_fields (0x19f9ba); only [0] is read
	sysSummF    [5]int // _sys_disp_summ_fields
	militaryF   [2]int
	jobF        [3]int
	satelliteF  [10]int
	bldgF       [6][6]int
	popRef      [3][42]int16
	popCount    [3]int16
	popCoord    [3]int16
	mapGridX    int // _galaxy_map_grid_x/y as the grid field writes them
	mapGridY    int
	officerPict *gfx.Picture // C_Anims_(0x1e)
	buttons     [0x24]*gfx.Picture
	shadow      *gfx.Picture    // _shadow_farmer_bitm
	shipBitm    *gfx.Picture    // _ship_bitm
	shipBitmN   int16           // _ship_bitm_n
	shipPlayerN int32           // _g_ship_player_n
	bldgBitm    *gfx.Picture    // _bldg_bitm
	xportAnims  [3]*gfx.Picture // RETURN (0x183194), up (0x18318c), down (0x183190)
	cheats      bool            // _cheats (0x182acb)
}

func (g *Game) initColonyScreenStatics() {
	c := &g.cscr
	c.valid = true
	c.planetDisp = -1
	c.scannedSat, c.scannedMilitary, c.scannedC, c.scannedR = -1, -1, -1, -1
	c.shipBitmN, c.shipPlayerN = -1, -1
}

// setColonyScreenHelpList is Set_Colony_Screen_Help_List_ (0xcdb01).
func (g *Game) setColonyScreenHelpList() {
	g.e.DeactivateHelpList()
	g.e.SetHelpList(g.helpListAt(colonyScreenHelpList, 0x11))
}

// colonyStarOf is the star of colony ci.
func (g *Game) colonyPlanetStar(ci int) (star, orbit int) {
	s := &g.state
	pl := &s.Planets[s.Colonies[ci].Planet]
	return int(pl.Star), int(pl.Orbit)
}

// doColonyScreen is Do_Colony_Screen_ (0x8310e): outposts only get a box,
// a colony of the player opens screen 1.
func (g *Game) doColonyScreen(ci int, leave *bool) error {
	s := &g.state
	if ci <= -1 || int(s.Colonies[ci].Owner) != int(int16(s.PlayerNum)) {
		return nil
	}
	c := &s.Colonies[ci]
	if c.IsOutpost != 0 {
		msg := cSprintf(cFormat(g.hMessage(0xd9)), g.planetName(int(c.Planet)))
		_, err := g.userBox(&g.ms.drawToBack, msg, boxMessage)
		return err
	}
	g.currentScreen, g.returnScreen = ScreenColony, ScreenMain
	star, orbit := g.colonyPlanetStar(ci)
	g.screenData[0], g.screenData[1] = int16(star), int16(orbit)
	*leave = true
	return nil
}

// colonyScreen is Colony_Screen_ (0xc058a), screen 1. The RNG is seeded with
// the colony shown before and restored on leaving, except when the planet
// has no colony.
func (g *Game) colonyScreen() error {
	e, s, cs := g.e, &g.state, &g.cscr
	seed := s.Random.Get()
	s.Random.Set(uint32(int32(g.colonyHandle)))
	e.V.ClipOff()
	cs.starHandle = int(g.screenData[0])
	cs.orbitN = int(g.screenData[1])
	cs.usingPal = true
	g.ms.galaxyMap.colonyPalette = true
	pl := int(s.Stars[cs.starHandle].Planets[cs.orbitN])
	cs.planet = pl
	quit := false
	g.colonyHandle = int(s.Planets[pl].Colony)
	cs.scanStar = -1
	cs.sysStar = cs.starHandle
	cs.valid = g.colonyHandle != -1
	if cs.valid {
		cs.display = 0
		cs.scanPlanet = -1
	} else {
		cs.display = 2
		cs.planetDisp = int16(pl)
	}
	cs.scanPlanet = -1
	g.clearMouseBuffer()
	e.SetInputDelay(3)
	g.setColonyScreenHelpList()
	savedPlayer := s.PlayerNum
	if cs.valid {
		s.PlayerNum = int16(s.Colonies[g.colonyHandle].Owner)
	}
	if err := g.loadColonyScreenSeg(); err != nil {
		return err
	}
	if !cs.valid {
		// The original jumps to the shared epilogue: no seed restore.
		g.currentScreen = g.returnScreen
		return nil
	}
	if err := g.makeBldgArrayForColony(g.colonyHandle, false); err != nil {
		return err
	}
	if g.currentScreen != g.returnScreen {
		cs.savedScr = g.returnScreen
	}
	e.ClearFields()
	g.setColonyFontTo(3)
	e.SetMouseList(engine.MouseNone)
	e.V.SetPageOff()
	e.V.SetRefreshStencil()
	if g.fastFadeOutNeededForColonyPalette() {
		if err := e.FastFadeOut(); err != nil {
			return err
		}
	}
	if err := e.LoadPalette(1, 0, 0xff); err != nil {
		return err
	}
	bg, err := g.cAnimsColony(0)
	if err != nil {
		return err
	}
	e.DrawPalette(bg)
	e.UpdateGlassRemapColors()
	if g.fastFadeOutNeededForColonyPalette() {
		if err := e.DarkenPalette(100); err != nil {
			return err
		}
		e.V.ClearPaletteChanges(0, 0xff)
	}
	e.RefreshPalette()
	g.resetFieldArrays()
	e.ClearFields()
	if err := g.colonyAddFields(); err != nil {
		return err
	}
	g.col.fullDraw = true
	e.AssignAutoFunction(g.drawColonyScreen, 1)
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	if err := g.drawColonyScreen(); err != nil {
		return err
	}
	if err := e.FastFadeIn(); err != nil {
		return err
	}
	e.V.SetRefreshStencil()
	e.SetMouseList(engine.MouseArrow)
	if err := g.doInformationalAndDecisionPopups(); err != nil {
		return err
	}
	refields := false
	r := 0
	for {
		e.MarkTime()
		g.col.count++
		if refields {
			e.ClearFields()
			if err := g.colonyAddFields(); err != nil {
				return err
			}
			r = 0
		}
		if err := g.drawColonyScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if r, err = e.GetInput(); err != nil {
			return err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return err
		}
		refields = r != 0
		ok, err := g.colonyEvaluateInput(&r, scan)
		if err != nil {
			return err
		}
		if !ok {
			quit = true
		}
		if err := e.ReleaseTime(1); err != nil {
			return err
		}
		if quit || r == cs.fields[cfReturn] {
			break
		}
	}
	if err := g.resetFieldMode(); err != nil {
		return err
	}
	e.ClearFields()
	e.DeactivateAutoFunction()
	cs.usingPal = false
	g.ms.galaxyMap.colonyPalette = false
	s.PlayerNum = savedPlayer
	if !quit {
		g.state.fixColonyQueue(g.colonyHandle)
		c := &s.Colonies[g.colonyHandle]
		if c.Queue[0] == -1 {
			c.Queue[0] = -2
		}
		if g.currentScreen == g.returnScreen {
			g.currentScreen, g.returnScreen = cs.savedScr, cs.savedScr
		} else {
			g.currentScreen = g.returnScreen
		}
	}
	e.DeactivateHelpList()
	s.Random.Set(seed)
	return nil
}

// loadColonyScreenSeg is the loading part of Colony_Screen_ and
// Load_Colony_Screen_Seg_ (0xbb954): pictures, bitmaps and the officer.
func (g *Game) loadColonyScreenSeg() error {
	cs := &g.cscr
	if err := g.loadBuildAnims(); err != nil {
		return err
	}
	if err := g.loadXportAnims(); err != nil {
		return err
	}
	cs.shipBitm = gfx.NewBlankBitmap(0, 0)
	cs.shipBitmN, cs.shipPlayerN = -1, -1
	cs.bldgBitm = nil
	if err := g.loadOfficerAnim(); err != nil {
		return err
	}
	g.loadRoadListAnims()
	gm := &g.ms.galaxyMap
	for i := 0; i < 10; i++ {
		p, err := g.picture(cStringAt(g.exe, gstarLBX), 0x17+i)
		if err != nil {
			return err
		}
		gm.colsumStarSeg[i] = p
	}
	return g.loadButtonAnims()
}

// loadButtonAnims is Load_Button_Anims_ (0xbb822): COLPUPS.LBX buttons into
// C_Anims_'s table.
func (g *Game) loadButtonAnims() error {
	for _, b := range [][2]int{{0x13, 1}, {0x14, 2}, {0x15, 4}, {0x16, 7}, {0x17, 8}, {0x18, 9}, {0x20, 0x15}} {
		p, err := g.picture(cStringAt(g.exe, colpupsLBX), b[1])
		if err != nil {
			return err
		}
		g.cscr.buttons[b[0]] = p
	}
	return nil
}

// loadOfficerAnim is Load_Officer_Anim_ (0xbb8dd): the governor's portrait.
func (g *Game) loadOfficerAnim() error {
	s, cs := &g.state, &g.cscr
	if o := s.Stars[cs.starHandle].Officer[s.PlayerNum]; o != -1 {
		p, err := g.loadOfficerPicture(int(o), 1)
		if err != nil {
			return err
		}
		cs.buttons[0x1e] = p
	}
	return nil
}

// cAnimsColony is C_Anims_ (0xbba8e) with the colony screen's table: the
// planet backdrop (0), COLONY2.LBX, COLPUPS.LBX and COLONY.LBX pictures,
// and the loaded slots.
func (g *Game) cAnimsColony(n int) (*gfx.Picture, error) {
	s := &g.state
	c2 := func(k int) (*gfx.Picture, error) { return g.colony2Pic(k) }
	pups := func(k int) (*gfx.Picture, error) { return g.cached(cStringAt(g.exe, colpupsLBX), k) }
	switch n {
	case 0:
		c := &s.Colonies[g.colonyHandle]
		k := int(int16(c.Climate))*3 + int(int16(s.Planets[c.Planet].Picture))
		return g.cached(cStringAt(g.exe, planetsLBX), k)
	case 1:
		return c2(0x31)
	case 3:
		return pups(5)
	case 4:
		return pups(6)
	case 5, 6, 7:
		return c2(0x10 + n - 5)
	case 8, 9, 10:
		return c2(0x2b + n - 8)
	case 13:
		return c2(0x32)
	case 14:
		return c2(0x33)
	case 15:
		return c2(0x34)
	case 0x1d:
		return nil, nil
	case 0x21:
		return pups(0x16)
	case 0x22:
		return g.cached(cStringAt(g.exe, colonyLBX), 0x11)
	case 0x23:
		return g.cached(cStringAt(g.exe, colonyLBX), 0x12)
	}
	if n < 0 || n >= len(g.cscr.buttons) {
		return nil, nil
	}
	return g.cscr.buttons[n], nil
}

// resetFieldArrays is Reset_Field_Arrays_ (0xbae88).
func (g *Game) resetFieldArrays() {
	cs := &g.cscr
	g.clearSysDispFields()
	for i := range cs.sysSummF {
		cs.sysSummF[i] = engine.NoField
	}
	for i := range cs.fields {
		cs.fields[i] = engine.NoField
	}
	cs.militaryF = [2]int{engine.NoField, engine.NoField}
	cs.jobF = [3]int{engine.NoField, engine.NoField, engine.NoField}
}

// colonyAddFields is Add_Fields_ (0xbf456): the job and military fields,
// the system display or galaxy map fields, then Add_Screen_Fields_.
func (g *Game) colonyAddFields() error {
	cs, gm := &g.cscr, &g.ms.galaxyMap
	g.clearSysDispFields()
	gm.gridField = engine.NoField
	n := int(g.state.NumStars)
	if len(gm.starField) < 0x48 {
		gm.starField = make([]int, max(n, 0x48))
	}
	for i := 0; i < 0x48; i++ {
		gm.starField[i] = engine.NoField
	}
	if cs.valid {
		if err := g.addJobFields(); err != nil {
			return err
		}
		if err := g.addMilitaryFields(); err != nil {
			return err
		}
	}
	switch cs.display {
	case 0:
		if err := g.addColSysDispFields(7, 0x18, 0x73); err != nil {
			return err
		}
	case 1:
		if err := g.addGalaxyMapFields2(7, 0x18, gm.width, gm.height, 0xb, 0x1f, false); err != nil {
			return err
		}
	}
	return g.addScreenFields()
}

// addScreenFields is Add_Screen_Fields_ (0xbf01b). The cheat hot key "c"
// comes before the BUILD button's, so the key never opens the build popup.
func (g *Game) addScreenFields() error {
	e, s, cs := g.e, &g.state, &g.cscr
	f := &cs.fields
	f[cfDisplayOff], f[cfDisplayOn] = engine.NoField, engine.NoField
	var err error
	add := func(i int, fn func() (int, error)) {
		if err == nil {
			f[i], err = fn()
		}
	}
	add(cfEsc, func() (int, error) { return e.AddHotKey(g.eStr(0x87)) })
	add(cfPrev, func() (int, error) { return e.AddHotKey(g.eStr(0x8b)) })
	add(cfNext, func() (int, error) { return e.AddHotKey(g.eStr(0x8c)) })
	add(cfCrunch, func() (int, error) { return e.AddMultiHotKeyField(g.eStr(0xf5)) })
	add(cfToggle, func() (int, error) { return e.AddMultiHotKeyField(g.eStr(0x218)) })
	add(cfReturn, func() (int, error) {
		p, _ := g.cAnimsColony(0x15)
		return e.AddButtonField(0x22c, 0x1cb, g.eStr(0xc), p, g.eStr(0xc), 0x28)
	})
	add(cfCheat, func() (int, error) { return e.AddHotKey(g.eStr(0xd5)) })
	if s.Stars[cs.starHandle].Officer[s.PlayerNum] == -1 {
		add(cfLeaders, func() (int, error) {
			p, _ := g.cAnimsColony(0x20)
			return e.AddButtonField(0x22c, 0x1ab, g.eStr(0xc), p, g.eStr(0x158), 0x28)
		})
	} else {
		add(cfLeaders, func() (int, error) { return e.AddHiddenField(0x226, 0x165, 0x26f, 0x1bd, g.eStr(0x158), 0) })
	}
	hidden := func(i, x1, y1, x2, y2 int) {
		add(i, func() (int, error) { return e.AddHiddenField(x1, y1, x2, y2, g.eStr(0xc), 0) })
	}
	hidden(cfMorale, 0x136, 0x20, 0x1fe, 0x3d)
	hidden(cfBC, 0x80, 0x20, 0x12d, 0x3d)
	hidden(cfFood, 0x80, 0x3e, 0x12d, 0x5b)
	hidden(cfIndustry, 0x80, 0x5c, 0x12d, 0x79)
	hidden(cfResearch, 0x80, 0x7a, 0x12d, 0x97)
	if !cs.valid {
		f[cfBuild], f[cfBuy] = engine.NoField, engine.NoField
	} else {
		add(cfBuild, func() (int, error) {
			p, _ := g.cAnimsColony(0x13)
			return e.AddButtonField(0x207, 0x7b, g.eStr(0xc), p, g.eStr(0xd5), 0x28)
		})
		add(cfAutobuild, func() (int, error) { return e.AddHiddenField(0x20d, 0x1a, 0x276, 0x25, g.eStr(0xa6), ')') })
		p, _ := g.cAnimsColony(0x14)
		if !g.colonyCanBuyProduct0(g.colonyHandle) {
			add(cfBuy, func() (int, error) {
				return e.AddHiddenField(0x24e, 0x7b, p.Width+0x24e, p.Height+0x7b, g.eStr(0xc1), '(')
			})
		} else {
			add(cfBuy, func() (int, error) { return e.AddButtonField(0x24e, 0x7b, g.eStr(0xc), p, g.eStr(0xc1), 0x28) })
		}
	}
	if err != nil {
		return err
	}
	if err := g.addStackingFields(); err != nil {
		return err
	}
	if err := g.addSatelliteFields(); err != nil {
		return err
	}
	if err := g.addBldgFields(); err != nil {
		return err
	}
	f[cfWhole], err = e.AddHiddenField(0, 0, 0x27f, 0x1df, g.eStr(0xc), 0)
	return err
}

// addStackingFields is Add_Stacking_Fields_ (0xbf399): the occupation
// banner of a colony being annihilated, and the transport button while
// colonists are picked up.
func (g *Game) addStackingFields() error {
	e, cs := g.e, &g.cscr
	c := &g.state.Colonies[g.colonyHandle]
	var err error
	if c.Occupation == 0 {
		cs.fields[cfOccupation], err = e.AddHiddenField(0, 0, 0x27f, 0x13, g.eStr(0xc), '(')
	} else {
		cs.fields[cfOccupation] = engine.NoField
	}
	if err != nil {
		return err
	}
	if g.getFirstUnemployed(g.colonyHandle) != -1 && cs.valid {
		p, _ := g.cAnimsColony(0x18)
		cs.fields[cfTransport], err = e.AddButtonField(0x133, 0x9f, g.eStr(0xc), p, g.eStr(0xc), 0x28)
		return err
	}
	cs.fields[cfTransport] = engine.NoField
	return nil
}

// satelliteX is a satellite slot's x: alternately right and left of 0x127.
func satelliteX(i int) int {
	s := 1
	if i%2 != 0 {
		s = -1
	}
	return int(int16(s*i*0x32 + 0x127))
}

// satelliteAnimIndex is Satellite_Bldg_N_TO_Satellite_Animation_Index_
// (0xbbbba).
func satelliteAnimIndex(id uint16) int {
	switch {
	case id == 3:
		return 7
	case id == 8:
		return 1
	case id == 0xe:
		return 3
	case id == 0x28:
		return 2
	case id == 0x29:
		return 0
	}
	return -1
}

// satelliteAnims is Satellite_Anims_ (0xbbb9f): COLONY.LBX 9+n.
func (g *Game) satelliteAnims(n int) (*gfx.Picture, error) {
	return g.cached(cStringAt(g.exe, colonyLBX), n+9)
}

// addSatelliteFields is Add_Satellite_Fields_ (0xbdfb9).
func (g *Game) addSatelliteFields() error {
	cs := &g.cscr
	for i, id := range cs.surf.satellites {
		if id == -1 {
			cs.satelliteF[i] = engine.NoField
			continue
		}
		p, err := g.satelliteAnims(satelliteAnimIndex(uint16(id)))
		if err != nil {
			return err
		}
		x := satelliteX(i)
		f, err := g.e.AddHiddenField(x, 0xa2, int(int16(x+int(int16(p.Width)))), int(int16(p.Height))+0xa2, g.eStr(0xc), 0)
		if err != nil {
			return err
		}
		cs.satelliteF[i] = f
	}
	return nil
}

// addBldgFields is Add_Bldg_Fields_ (0xbe44a): a field round each building,
// front to back.
func (g *Game) addBldgFields() error {
	cs := &g.cscr
	order := g.exe.bytes(bldgFieldOrder, 72)
	for i := 0x46; i != -2; i -= 2 {
		c, r := int(order[i]), int(order[i+1])
		if cs.surf.bldgs[c][r] == 0 {
			cs.bldgF[c][r] = engine.NoField
			continue
		}
		x := g.bldgCoordsToCenteredScreenCoord(c, r, 0)
		y := g.bldgCoordsToCenteredScreenCoord(c, r, 1)
		f, err := g.e.AddHiddenField(int(int16(x-0x14)), int(int16(y-0x1e)), int(int16(x+0x14)), int(int16(y+10)), g.eStr(0xc), 0)
		if err != nil {
			return err
		}
		cs.bldgF[c][r] = f
	}
	return nil
}

// colonyPopStuff calls Do_Colony_Info_Pop_Stuff_For_Pop_ for job row j of
// the colony screen (with the shadow farmers).
func (g *Game) colonyPopStuff(job, mode int) (int16, error) {
	cs := &g.cscr
	return g.doColonyInfoPopStuffForPop(g.colonyHandle, job, mode, 0x136, int16(job*0x1e+0x3e), 0x1fe, true,
		cs.popRef[job][:], &cs.popCount[job], &cs.popCoord[job], &cs.jobF[job])
}

// addJobFields is Add_Job_Fields_ (0xbcc3d): the job rows as scroll fields,
// or, while colonists are picked up, as plain hidden fields.
func (g *Game) addJobFields() error {
	cs := &g.cscr
	for j := 0; j < 3; j++ {
		switch cs.fieldMode {
		case 0:
			if _, err := g.colonyPopStuff(j, 1); err != nil {
				return err
			}
		case 1:
			f, err := g.e.AddHiddenField(0x136, j*0x1e+0x3c, 0x1fe, j*0x1e+0x58, g.eStr(0xc), 0)
			if err != nil {
				return err
			}
			cs.jobF[j] = f
		}
	}
	return nil
}

// colCalcWrapper is Col_Calc_Wrapper_ (0xbb1a6). _move_colonist is always
// -1, so it is Colony_Calculation_.
func (g *Game) colCalcWrapper(ci int) { g.colonyCalculation(ci) }

// remapDrawShipCentered is Remap_Draw_Ship_Centered_ (0xbaac5): ship k's
// picture recoloured into _ship_bitm (kept while the same picture and
// builder are asked for), centred at (x, y).
func (g *Game) remapDrawShipCentered(x, y, k int) error {
	cs := &g.cscr
	d := &g.state.Ships[k].Design
	pic, pl := int16(int8(d.Picture)), int32(int8(d.Builder))
	if pic != cs.shipBitmN || pl != cs.shipPlayerN {
		cs.shipBitmN, cs.shipPlayerN = pic, pl
		if err := g.remapShipBitm(int(pl), int(pic), shipRemapColors); err != nil {
			return err
		}
	}
	return g.drawCentered(x, y, cs.shipBitm)
}

// remapDrawShipSegNCentered is Remap_Draw_Ship_Seg_N_Centered_ (0xbaba4):
// ship picture n of the player.
func (g *Game) remapDrawShipSegNCentered(x, y int, n int16) error {
	cs := &g.cscr
	if n != cs.shipBitmN {
		cs.shipBitmN, cs.shipPlayerN = n, int32(int16(g.state.PlayerNum))
		if err := g.remapShipBitm(int(cs.shipPlayerN), int(n), shipSegRemapColors); err != nil {
			return err
		}
	}
	return g.drawCentered(x, y, cs.shipBitm)
}

// remapShipBitm draws Get_Ship_Picture_Seg_(player, n) into _ship_bitm and
// recolours it: 1..0x1f through the table, 0xc0+i and 0xd0+i to its entry 2i.
func (g *Game) remapShipBitm(player, n int, colors uint32) error {
	cs := &g.cscr
	p, err := g.shipPictureSeg(player, n)
	if err != nil {
		return err
	}
	cs.shipBitm.DrawPictureToBitmap(p)
	t := g.exe.bytes(colors, 0x1f)
	cs.shipBitm.ReplaceColorRange(t, 1, 0x1f)
	for i := 0; i < 0x10; i++ {
		if err := cs.shipBitm.ReplaceColor(0xc0+i, int(t[2*i])); err != nil {
			return err
		}
		if err := cs.shipBitm.ReplaceColor(0xd0+i, int(t[2*i])); err != nil {
			return err
		}
	}
	return nil
}

// shipPictureSeg is Get_Ship_Picture_Seg_ (0x58697): SHIPS.LBX by the
// player's banner (8 for monsters) and picture n.
func (g *Game) shipPictureSeg(player, n int) (*gfx.Picture, error) {
	banner := 8
	if int16(player) < 8 {
		banner = int(g.state.Players[player].Color)
	}
	return g.cached(cStringAt(g.exe, 0x178ed8), banner*0x32+int(int16(n)))
}

// drawCentered is Draw_Centered_ (0xc5cff).
func (g *Game) drawCentered(x, y int, p *gfx.Picture) error {
	return g.e.Draw(x-int(int16(p.Width))/2, y-int(int16(p.Height))/2, p)
}

// drawBottomCentered is Draw_Bottom_Centered_ (0xc5d55).
func (g *Game) drawBottomCentered(x, y int, p *gfx.Picture) error {
	return g.e.Draw(x-int(int16(p.Width))/2, y-int(int16(p.Height)), p)
}

var _ = save.Food

// loadBuildAnims is Load_Build_Anims_ (0xb0a24): COLBLDG.LBX 0..6 into
// _anims and COLSUM.LBX 0x15, the colony summary's palette.
func (g *Game) loadBuildAnims() error {
	bq := &g.bq
	for i := 0; i < 7; i++ {
		p, err := g.picture(cStringAt(g.exe, colbldgLBX), i)
		if err != nil {
			return err
		}
		bq.anims[i] = p
	}
	p, err := g.picture(cStringAt(g.exe, colsumLBX), 0x15)
	if err != nil {
		return err
	}
	g.cs.palAnim = p
	return nil
}

// loadXportAnims is Load_Xport_Anims_ (0xc52f6): the transport popup's
// RETURN and arrow buttons.
func (g *Game) loadXportAnims() error {
	for i, n := range []int{0x11, 0x13, 0x14} {
		p, err := g.picture(cStringAt(g.exe, colpupsLBX), n)
		if err != nil {
			return err
		}
		g.cscr.xportAnims[i] = p
	}
	return nil
}
