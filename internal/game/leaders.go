package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The leaders screen (screen 0x1d): MOX\OFFICER.C. Ship officers with the
// fleet screen's ship lists, or colony leaders with a system display.
// Spec: docs/screens/leaders-screen.md.

// OFFICER.C data in the image.
const (
	staticColoffHelpList  = 0x181c7c // 16 records
	staticShipoffHelpList = 0x181d1c // 33 records
	mouseListWho          = 0x18228c // 1 entry
	offRedColors          = 0x18227c
	offBrightTextColors   = 0x182298
	famousMask            = 0x17d411 // skill 0x1c's mask
	leaderNameFormat      = 0x179b0b // "%s"
	moneyFormat           = 0x179b0e // "%d %s"
	skillNameFormat       = 0x179b17 // "%s"
	commaChar             = 0x179af9 // ","
	offStarLeaderFormat   = 0x179afb // "%s (%s)"
)

// prevLeader is one 5-byte _prev_leaders record: a leader as the leaders
// screen found him, which Get_Ship_Leader_ETA_ and Get_Star_Leader_ETA_ read.
type prevLeader struct {
	status   int8
	location int16
	eta      int16
}

// offFields are Officers_Screen_'s field locals.
type offFields struct {
	hire, cancel, shipLdrs, colLdrs, ret int
	names, pics                          [4]int
	up, down, next, prev, scroll         int
}

// offScreen holds OFFICER.C's screen globals.
type offScreen struct {
	buttonMode                            int8  // _officer_button_mode: -1 none, 0 hire, 1 pool, 2 dismiss
	selected                              int8  // _officer_selected
	scanned                               int8  // _officer_scanned
	offpopSel                             int16 // _offpop_big_icon_selected
	starShown                             int16 // _officer_star_displayed
	starChosen                            int16 // _officer_star_chosen
	flashValue                            uint8
	flashIncr                             uint8
	flashDir                              int8
	skipShips                             bool // _offscrn_skip_draw_ships
	fullDraw                              bool // _full_draw
	drawToBack                            bool // _draw_officer_to_back
	nOfficers                             int16
	dismissBtn                            int
	poolBtn                               int
	skillHelp                             [4][5]int
	skillOf                               [4][5]int
	origStar                              [maxLeaders]int16
	origShip                              [maxLeaders]int16
	fields                                offFields
	ldrpict                               [8]*gfx.Picture
	skillNames                            [0x1b]string
	skillDescs                            [0x1b]string
	main, fleet                           *gfx.Picture
	colony                                *gfx.Picture
	colBtn, shBtn, upBtn, dnBtn           *gfx.Picture
	prevBtn, nextBtn, hireBtn, poolBtnPic *gfx.Picture
	dismissPic, retBtn, cancelHire        *gfx.Picture
	hireReplace, dullHire, dullPool       *gfx.Picture
	dullDismiss, normalWindow, brightWin  *gfx.Picture
}

// fleetType is _officer_scrn_type == 0, the ship officers page.
func (g *Game) offFleetType() bool { return g.ms.officerScrnType == 0 }

// officersScreen is Officers_Screen_ (0x94c1d).
func (g *Game) officersScreen() error {
	e, o, f, s, m := g.e, &g.off, &g.flt, &g.state, &g.ms
	savedScale := s.CurMapScale
	resetBar := false
	e.V.ResetWindow()
	e.V.ClipOff()
	e.ClearFields()
	m.galaxyMap.colonyPalette = false
	o.flashValue, o.flashIncr, o.flashDir = 2, 0, 1
	o.skipShips, o.fullDraw = false, false
	f.shipToMove = -1
	g.offscreenAskForHirePopup = false
	o.buttonMode = -1
	f.nAdded, f.nBig, f.nRows, f.nSelected = 0, 0, 0, 0
	f.smallPtr, f.oldSmallPtr, f.oldFirst = -1, -1, -1
	m.galaxyMapScannedStar, m.systemIconScanned = -1, -1
	f.scannedBig, f.scannedSmall = -1, -1
	m.galaxyMap.starAnimCount = 0
	g.clearSystemDisplayLoadedFlag()
	e.V.ClearPaletteChanges(0, 0xff)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	if g.previousScreen == ScreenColony {
		m.officerScrnType = 1
		if g.screenData[4] > -1 {
			f.smallPtr = g.screenData[4]
		}
	}
	if m.officerScrnType == -1 {
		m.officerScrnType = 0
	}
	if err := g.reloadOfficerScreenPictures(); err != nil {
		return err
	}
	g.buildShipIcons()
	g.setFltscrnSmallShipIconXYs(0x132, 0xeb, 0x13e, 0xa9)
	if g.offFleetType() {
		if f.smallPtr < 0 {
			g.setInitialStackPointer()
		}
	} else {
		o.starShown, o.starChosen = g.screenData[4], g.screenData[4]
		if o.starChosen == -1 {
			o.starChosen = int16(s.Planets[s.Players[s.PlayerNum].HomePlanet].Star)
		}
		if err := g.initSystemDisplay(int(o.starShown)); err != nil {
			return err
		}
	}
	o.drawToBack = true
	if f.smallPtr != -1 && g.shipStackOwner(g.stacks.start[f.smallPtr]) != int(s.PlayerNum) {
		f.smallPtr = -1
	}
	e.AssignAutoFunction(g.drawOfficerScreen, 1)
	f.xOff, f.yOff, f.cols, f.rows, f.maxIcons = 0x12e, 0x13, 5, 3, 0xf
	if err := g.addOfficerScreenFields(); err != nil {
		return err
	}
	f.bar.initialize(3, 0x79, 2)
	m.drawingOfficerPopup = true
	o.selected, o.scanned, o.offpopSel = -1, -1, -1
	g.sd.starID = -1
	g.clearSystemDisplayLoadedFlag()
	for i := range s.Leaders {
		o.origStar[i] = s.Leaders[i].Location
	}
	for i := range s.Leaders {
		o.origShip[i] = s.Leaders[i].Location
	}
	o.drawToBack = true
	g.ub.globalVar = 1
	g.returnScreen = g.previousScreen
	leave := false
	if err := g.fadeIntoLeadersScreen(); err != nil {
		return err
	}
	g.ub.globalVar = 0
	g.setOfficerScreenHelp()
	for !leave {
		fleet := g.offFleetType()
		e.MarkTime()
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return err
		}
		ff := &o.fields
		if r == ff.cancel {
			e.SetMouseList(engine.MouseArrow)
			o.buttonMode, o.offpopSel, o.selected = -1, -1, -1
			o.drawToBack = true
		}
		if r == ff.hire {
			switch {
			case o.selected > -1:
				if _, err := g.doHireOfficer(int(o.selected)); err != nil {
					return err
				}
			case o.buttonMode == 0:
				e.SetMouseList(engine.MouseArrow)
				o.buttonMode, o.offpopSel = -1, -1
				o.drawToBack = true
			default:
				e.SetMouseList(g.exeMouseList(mouseListWho, 1))
				o.buttonMode, o.offpopSel, o.selected = 0, -1, -1
			}
		}
		if err := g.checkOfficerFields(r, scan); err != nil {
			return err
		}
		if o.buttonMode == -1 {
			res, sc := g.scanSmallShipIcons(scan, r)
			if res == 0 && g.offFleetType() {
				o.drawToBack = true
				if err := g.reloadOfficerScreenPictures(); err != nil {
					return err
				}
			} else if sc > -1 {
				f.scannedSmall = int16(sc)
				m.galaxyMapScannedStar = -1
				f.scannedBig = -1
			}
		}
		if o.buttonMode == -1 && fleet {
			if err := g.officerBigIcons(scan, r); err != nil {
				return err
			}
		}
		if o.buttonMode == -1 {
			res, hov, clk := g.scanGalaxyMapFields(scan, r)
			if hov > -1 {
				f.scannedSmall, f.scannedBig = -1, -1
				m.galaxyMapScannedStar = int16(hov)
			}
			if res == 0 && !fleet && clk > -1 && s.playerHasColony(int(s.PlayerNum), clk) {
				if int16(clk) == o.starChosen {
					o.starChosen = -1
				} else {
					o.selected = -1
					o.drawToBack = true
					o.starChosen = int16(clk)
				}
				o.starShown = int16(clk)
			}
		}
		if o.buttonMode == -1 && !fleet {
			g.sd.offscreen = true
			res, _, clkP, hovS, _, icon := g.scanSystemDisplayFields(scan, r)
			g.sd.offscreen = false
			g.sd.shipScanned = int16(hovS)
			if icon > -1 {
				m.systemIconScanned = int16(icon)
			}
			if res == 0 && clkP > -1 {
				c := s.Planets[s.Stars[o.starShown].Planets[clkP]].Colony
				if c > -1 && c < s.NumColonies && m.officerScreenReturnScreen == ScreenReports {
					if _, err := g.officerScreenUserBox(g.hMessage(0x12e), 0); err != nil {
						return err
					}
				}
			}
		}
		if o.buttonMode == -1 && (r == ff.next || r == ff.prev) {
			back := r == ff.prev
			if g.offFleetType() {
				g.cycleShipIcons(&f.smallPtr, back)
				resetBar = true
				f.scannedBig = -1
			} else {
				o.starShown = int16(g.cycleStarIcons(int(o.starShown), int(s.PlayerNum), back))
				o.starChosen = o.starShown
			}
		}
		if r == o.dismissBtn {
			if o.buttonMode == 2 {
				o.buttonMode = -1
				e.SetMouseList(engine.MouseArrow)
			} else {
				o.buttonMode = 2
				e.SetMouseList(g.exeMouseList(mouseListWho, 1))
			}
		}
		if r == o.poolBtn {
			switch {
			case o.buttonMode == 1:
				o.buttonMode = -1
				e.SetMouseList(engine.MouseArrow)
			case o.selected > -1:
				if err := g.moveOfficerToLimbo(int(o.selected)); err != nil {
					return err
				}
			default:
				o.buttonMode = 1
				e.SetMouseList(g.exeMouseList(mouseListWho, 1))
			}
		}
		switch {
		case o.buttonMode == -1 && r == ff.down:
			f.bar.scrollDownOneItem()
		case o.buttonMode == -1 && r == ff.up:
			f.bar.scrollUpOneItem()
		}
		if o.buttonMode == -1 && (r == ff.colLdrs || r == ff.shipLdrs) {
			old := m.officerScrnType
			m.officerScrnType = int(b2u8(r == ff.colLdrs))
			if m.officerScrnType != old {
				if err := g.changeScreenType(&resetBar); err != nil {
					return err
				}
			}
		}
		if r == ff.ret {
			leave = true
			g.currentScreen = g.returnScreen
		}
		e.ClearFields()
		if err := g.addOfficerScreenFields(); err != nil {
			return err
		}
		if resetBar {
			resetBar = false
			g.setOfficerScreenHelp()
			if g.offFleetType() {
				f.bar.initialize(3, 0x79, 2)
			}
		}
		if !leave {
			e.V.SetPageOff()
			if err := g.drawOfficerScreen(); err != nil {
				return err
			}
			if err := e.TogglePages(); err != nil {
				return err
			}
		}
		if err := e.ReleaseTime(1); err != nil {
			return err
		}
	}
	e.ClearFields()
	e.DeactivateAutoFunction()
	e.SetMouseList(engine.MouseArrow)
	g.moveFromLimboToPool()
	g.checkForColonyCalculationUpdate()
	g.doPlayerColonyCalculations(int(s.PlayerNum))
	m.reloadedShipPalette = false
	g.prevLeaders = nil
	g.fb.curBar = nil
	s.CurMapScale = savedScale
	return nil
}

// officerBigIcons is Officers_Screen_'s part for the big ship icons: a
// click picks the ship for the selected officer (combat ships only), a
// right click shows it.
func (g *Game) officerBigIcons(scan, r int) error {
	o, f, m := &g.off, &g.flt, &g.ms
	res, scanned, clicked := g.scanOffscrnBigIconFields(scan, r)
	f.scannedBig = scanned
	if scanned > -1 {
		m.galaxyMapScannedStar = -1
		f.scannedSmall = -1
	}
	switch res {
	case 0:
		o.drawToBack = true
		if clicked == o.offpopSel {
			o.offpopSel = -1
			f.big[clicked].sel = false
			return nil
		}
		if clicked <= -1 {
			return nil
		}
		if !g.isCombatShip(int(f.big[clicked].ship)) {
			_, err := g.officerScreenUserBox(g.hMessage(0x12c), 3)
			return err
		}
		o.offpopSel = clicked
		for i := int16(0); i < f.nBig; i++ {
			f.big[i].sel = false
		}
		f.big[o.offpopSel].sel = true
		if o.selected <= -1 || o.selected >= maxLeaders {
			return nil
		}
		if g.leaderCanBeAssigned(int(o.selected)) {
			if err := g.assignCaptain(int(o.selected), int(f.big[clicked].ship)); err != nil {
				return err
			}
			o.drawToBack = true
			f.big[o.offpopSel].sel = false
			o.selected, o.offpopSel = -1, -1
			return nil
		}
		msg := fmt.Sprintf(cFormat(g.hMessage(0x12d)), g.leaderName(int(o.selected), false))
		if _, err := g.officerScreenUserBox(msg, 0); err != nil {
			return err
		}
		if o.offpopSel > -1 {
			f.big[o.offpopSel].sel = false
			o.offpopSel = -1
		}
	case 1:
		ship := int(f.big[clicked].ship)
		g.e.DeactivateHelpList()
		if err := g.detailedViewShip(ship); err != nil {
			return err
		}
		g.setOfficerScreenHelp()
		o.drawToBack = true
	}
	return nil
}

// scanOffscrnBigIconFields is Scan_Offscrn_Big_Icon_Fields_ (0x97041):
// Scan_Fltscrn_Big_Icons_ for icons with a ship.
func (g *Game) scanOffscrnBigIconFields(scan, r int) (res, scanned, clicked int16) {
	f := &g.flt
	first := f.bar.first * int16(f.cols)
	end := f.nBig
	if int(first)+0x14 <= int(f.nBig) {
		end = first + 0x14
	}
	res, scanned, clicked = -1, -1, -1
	stop := first < 0 || end < 0
	for i := first; i < end && !stop; i++ {
		b := &f.big[i]
		switch {
		case r == b.field && b.ship > -1:
			res, stop, clicked = 0, true, i
		case -r == b.field && b.ship > -1:
			res, stop, clicked = 1, true, i
		}
	}
	if res == -1 {
		for i := first; i < end && !stop; i++ {
			if b := &f.big[i]; scan == b.field && b.ship > -1 {
				res, stop, scanned = 4, true, i
			}
		}
	}
	return
}

// fadeIntoLeadersScreen is Fade_Into_Leaders_Screen_ (0x93bb0).
func (g *Game) fadeIntoLeadersScreen() error {
	e, f := g.e, &g.flt
	e.DisableDrawAllFields()
	e.V.SetRefreshStencil()
	g.off.drawToBack = true
	e.SetMouseList(engine.MouseNone)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	if err := e.LoadPalette(8, 0, 0xff); err != nil {
		return err
	}
	e.V.SetPaletteChanges(0, 0xff)
	if g.offFleetType() && f.smallPtr > -1 {
		if err := g.loadPlayerShipPalette(g.shipStackOwner(g.stacks.start[f.smallPtr])); err != nil {
			return err
		}
	}
	if err := e.DarkenPalette(100); err != nil {
		return err
	}
	e.V.SetPageOff()
	if err := g.drawOfficerScreen(); err != nil {
		return err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.SetMouseList(engine.MouseArrow)
	if err := e.FastFadeIn(); err != nil {
		return err
	}
	e.EnableDrawAllFields()
	return nil
}

// reloadOfficerScreenPictures is Reload_Officer_Screen_Pictures_ (0x968d1).
func (g *Game) reloadOfficerScreenPictures() error {
	o, s := &g.off, &g.state
	home := int16(s.Planets[s.Players[s.PlayerNum].HomePlanet].Star)
	g.allocateFltscrnNextScreenSeg()
	o.ldrpict = [8]*gfx.Picture{}
	prev := new([maxLeaders]prevLeader)
	for i := range s.Leaders {
		l := &s.Leaders[i]
		p := &prev[i]
		p.status, p.location, p.eta = l.Status, l.Location, int16(l.ETA)
		if l.Type == 1 && (p.status == 0 || p.status == 4) {
			p.location = home
		}
	}
	g.prevLeaders = prev
	g.buildCaptainIDList()
	file := cStringAt(g.exe, g.exe.u32(officerLBXName))
	var err error
	load := func(entry int) *gfx.Picture {
		if err != nil {
			return nil
		}
		var p *gfx.Picture
		p, err = g.picture(file, entry)
		return p
	}
	o.main, o.fleet, o.colony = load(0), load(1), load(2)
	o.colBtn, o.shBtn, o.upBtn, o.dnBtn = load(3), load(4), load(5), load(6)
	o.prevBtn, o.nextBtn, o.hireBtn, o.poolBtnPic = load(7), load(8), load(9), load(10)
	o.dismissPic, o.retBtn, o.cancelHire = load(0xb), load(0xc), load(0xd)
	g.flt.selectedBox, g.flt.scannedBox, g.flt.scrollThumb = load(0xe), load(0xf), load(0x10)
	o.hireReplace, o.dullHire, o.dullPool, o.dullDismiss = load(0x11), load(0x12), load(0x13), load(0x14)
	o.normalWindow, o.brightWin = load(0x8d), load(0x8e)
	if err != nil {
		return err
	}
	g.flt.officerPics = g.flt.officerPics[:0]
	if err := g.getOfficerPictures(); err != nil {
		return err
	}
	if err := g.loadOfficerSkillNameDescriptions(); err != nil {
		return err
	}
	if err := g.reloadOfficerSkillIcons(); err != nil {
		return err
	}
	if err := g.reloadSmallShipIcons(file, 0x73); err != nil {
		return err
	}
	for i := range g.flt.galaxyStars {
		if g.flt.galaxyStars[i] = load(0x82 + i); err != nil {
			return err
		}
	}
	return nil
}

// buildCaptainIDList is Build_Captain_Id_List_ (0x92e72): up to four of
// the player's leaders of the page's kind, hired or offered.
func (g *Game) buildCaptainIDList() {
	o, f, s := &g.off, &g.flt, &g.state
	o.nOfficers = 0
	for i := range f.officerIDs {
		f.officerIDs[i] = -1
	}
	want := uint8(b2u8(!g.offFleetType()))
	for i := 0; i < maxLeaders && o.nOfficers <= 3; i++ {
		l := &s.Leaders[i]
		ok := !(l.Status < 0 && l.Status != 4)
		if ok && int(l.Owner) == int(s.PlayerNum) && l.Type == want {
			f.officerIDs[o.nOfficers] = int16(i)
			o.nOfficers++
		}
	}
}

// getOfficerPictures is Get_Officer_Pictures_ (0x93df7): each row's
// portrait and its darkened version.
func (g *Game) getOfficerPictures() error {
	o, f, s := &g.off, &g.flt, &g.state
	file := cStringAt(g.exe, g.exe.u32(officerLBXName))
	for i := 0; i < int(o.nOfficers); i++ {
		id := int(f.officerIDs[i])
		if id < 0 {
			continue
		}
		p, err := g.loadOfficerPicture(id, 0)
		if err != nil {
			return err
		}
		d, err := g.picture(file, int(s.Leaders[id].Picture)+0x8f)
		if err != nil {
			return err
		}
		o.ldrpict[i], o.ldrpict[i+4] = p, d
	}
	return nil
}

// loadOfficerSkillNameDescriptions is Load_Officer_Skill_Name_Descriptions_
// (0x942fd): the 27 skill names (records of entry 0) and descriptions
// (entry 1) of the skill description file.
func (g *Game) loadOfficerSkillNameDescriptions() error {
	o := &g.off
	a, err := g.archive(g.skillDescriptionLBXName())
	if err != nil {
		return err
	}
	for k, dst := range [][]string{o.skillNames[:], o.skillDescs[:]} {
		t, err := a.Table(k)
		if err != nil {
			return err
		}
		for i := range dst {
			dst[i] = save.CString(t.Record(i))
		}
	}
	return nil
}

// skillDescriptionLBXName is Get_Skill_Description_LBX_Name_ (0x99403).
func (g *Game) skillDescriptionLBXName() string {
	names := map[byte]uint32{1: 0x179b32, 2: 0x179b3f, 3: 0x179b4c, 4: 0x179b59}
	a, ok := names[g.settings[0xd2]]
	if !ok {
		a = 0x179b25 // SKILDESC.LBX
	}
	return cStringAt(g.exe, a)
}

// setOfficerScreenHelp is Set_Officer_Screen_Help_ (0x9872c).
func (g *Game) setOfficerScreenHelp() {
	n := g.nSkillsPerOfficerDisplayed()
	var list []engine.HelpEntry
	count := 0
	if g.ms.officerScrnType == 1 {
		list = g.helpListAt(staticColoffHelpList, 16)
		for k := 0; k < 4; k++ {
			if n[k] > 0 {
				list[k+3].Y1 += n[k]*0x11 + 0x10
			}
		}
		count = 0x10
	} else {
		list = g.helpListAt(staticShipoffHelpList, 33)
		for k := 0; k < 4; k++ {
			if n[k] > 0 {
				list[k+2].Y1 += n[k]*0x11 + 0x10
			}
		}
		count = 0x12
		if a := g.flt.nAdded; a == 0 {
			list[count] = engine.HelpEntry{ID: 0x14d, X1: 0x12d, Y1: 0x10, X2: 0x262, Y2: 0xc6}
			count++
		} else {
			for ; a < 0xf; a++ {
				c, r := int(a%5)*0x3f, int(a/5)*0x3e
				list[count] = engine.HelpEntry{ID: 0x14d, X1: c + 0x12d, Y1: r + 0x10, X2: c + 0x16c, Y2: r + 0x4e}
				count++
			}
		}
	}
	g.e.DeactivateHelpList()
	g.e.SetHelpList(list[:count])
}

// nSkillsPerOfficerDisplayed is Get_N_Skills_Per_Officer_Displayed_
// (0x98633): the skill lines each row shows.
func (g *Game) nSkillsPerOfficerDisplayed() [4]int {
	var n [4]int
	for row := 0; row < 4; row++ {
		id := int(g.flt.officerIDs[row])
		if id < 0 {
			continue
		}
		special := uint8(1)
		switch g.state.Leaders[id].Type {
		case 0:
		case 1:
			special = 2
		default:
			continue
		}
		for skill := 0; skill < 0x36; skill++ {
			kind := g.exeU8(uint32(tblSkillData + skill*tblSkillDataRow + 5))
			if (kind == 0 && g.leaderHasSkill(id, skill)) || (kind == special && g.leaderHasSpecialSkill(id, skill)) {
				n[row]++
			}
		}
	}
	return n
}

// addOfficerScreenFields is Add_Officer_Screen_Fields_ (0x9264e).
func (g *Game) addOfficerScreenFields() error {
	e, o, f := g.e, &g.off, &g.flt
	ff := &o.fields
	fleet := g.offFleetType()
	pool := g.leadersInHiringPool()
	have := o.nOfficers > 0
	var err error
	hidden := func(x, y int, p *gfx.Picture, hot string) int {
		if err != nil {
			return engine.NoField
		}
		var n int
		n, err = e.AddHiddenField(x, y, p.Width+x, p.Height+y, hot, ')')
		return n
	}
	button := func(x, y int, p *gfx.Picture, hot string) int {
		if err != nil {
			return engine.NoField
		}
		var n int
		n, err = e.AddButtonField(x, y, "", p, hot, 0x28)
		return n
	}
	o.dismissBtn, o.poolBtn = engine.NoField, engine.NoField
	if o.buttonMode != 0 && have {
		o.dismissBtn = hidden(0x1cf, 0x1b9, o.dismissPic, "D")
	}
	if o.buttonMode != 0 && have {
		o.poolBtn = hidden(0x184, 0x1b9, o.poolBtnPic, "P")
	}
	ff.hire = engine.NoField
	if pool && o.buttonMode != 0 {
		ff.hire = button(0x139, 0x1b9, o.hireBtn, "H")
	}
	ff.cancel = engine.NoField
	if o.buttonMode == 0 && pool {
		ff.cancel = button(0x1cc, 0x1b7, o.cancelHire, "X")
	}
	ff.shipLdrs = hidden(0x9c, 0xb, o.shBtn, "S")
	ff.colLdrs = hidden(9, 0xb, o.colBtn, "C")
	ff.ret = button(0x21a, 0x1b9, o.retBtn, "\x1b")
	if err != nil {
		return err
	}
	if err := g.offscrnHelpFields(); err != nil {
		return err
	}
	for row := 0; row < 4; row++ {
		ff.names[row], ff.pics[row] = engine.NoField, engine.NoField
		if row < int(o.nOfficers) {
			y := row*0x6d + 0x22
			if ff.names[row], err = e.AddHiddenField(0x5c, y, 0x128, row*0x6d+0x8e, "", ')'); err != nil {
				return err
			}
			if ff.pics[row], err = e.AddHiddenField(8, y, 0x5b, row*0x6d+0x82, "", ')'); err != nil {
				return err
			}
		}
	}
	ff.up, ff.down, ff.scroll = engine.NoField, engine.NoField, engine.NoField
	if !fleet {
		var drag, whole int
		g.sd.offscreen = true
		err := g.addSystemDisplayPopupFields(int(o.starShown), 0x132, 0x11, 0, 0, &drag, nil, &whole)
		g.sd.offscreen = false
		if err != nil {
			return err
		}
	} else {
		ff.up = button(0x265, 0x16, o.upBtn, "-")
		ff.down = button(0x265, 0xaa, o.dnBtn, "+")
		if err != nil {
			return err
		}
		if ff.scroll, err = g.fleetScreenBigIconFields(0x264, 0x2d, 10, 0x79); err != nil {
			return err
		}
		f.bar.rows = int16(f.nRows)
	}
	f.bar.upField, f.bar.downField = ff.up, ff.down
	ff.next = button(0x238, 0xcd, o.nextBtn, ".")
	ff.prev = button(0x147, 0xcd, o.prevBtn, ",")
	if err != nil {
		return err
	}
	if g.ms.fields.debug, err = e.AddHiddenField(0, 0x1d6, 10, 0x1df, "", ')'); err != nil {
		return err
	}
	if err := g.addFltscrnSmallIconFields(); err != nil {
		return err
	}
	if err := g.addGalaxyMapFields2(0x132, 0xeb, 0x13e, 0xa9, 0, 0, true); err != nil {
		return err
	}
	_, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0)
	return err
}

// leadersInHiringPool is Leaders_In_Hiring_Pool_ (0x9427b): one of the
// page's kind is offered to the player.
func (g *Game) leadersInHiringPool() bool {
	s := &g.state
	want := uint8(b2u8(!g.offFleetType()))
	for i := 0; i < maxLeaders; i++ {
		l := &s.Leaders[i]
		if l.Status == 4 && int(l.Owner) == int(s.PlayerNum) && l.Type == want {
			return true
		}
	}
	return false
}

// offscrnHelpFields is Wrapper_For_Offscrn_Help_Fields_ (0x97400).
func (g *Game) offscrnHelpFields() error {
	o := &g.off
	for row := 0; row < 4; row++ {
		id := int(g.flt.officerIDs[row])
		if id < 0 {
			for k := 0; k < 5; k++ {
				o.skillHelp[row][k], o.skillOf[row][k] = engine.NoField, 0
			}
			continue
		}
		if err := g.addSkillDescriptionHelpFields(0x5e, 0x32+row*0x6d, id, &o.skillHelp[row], &o.skillOf[row]); err != nil {
			return err
		}
	}
	return nil
}

// drawOfficerScreen is Draw_Officer_Screen_ (0x93550), the auto function.
func (g *Game) drawOfficerScreen() error {
	e, o, f, s, m := g.e, &g.off, &g.flt, &g.state, &g.ms
	fleet := g.offFleetType()
	if o.drawToBack || g.fullDrawScreen || o.fullDraw {
		if err := g.drawOfficerScreenToBack(); err != nil {
			return err
		}
	}
	e.V.ResetWindow()
	e.V.ClipOff()
	switch {
	case !fleet:
		if err := g.offscrnSystemPopup(int(o.starShown)); err != nil {
			return err
		}
	case o.skipShips:
		o.skipShips = false
	default:
		changed, err := g.drawGenericVerticalScrollBar(&f.bar, g.fillFltscrnScrollBar)
		if err != nil {
			return err
		}
		if changed {
			f.scannedBig = -1
		}
		if f.smallPtr > -1 {
			if err := g.drawFltscrnBigShipIcons(0x1d); err != nil {
				return err
			}
		}
	}
	if err := g.drawGalaxyMapBox(nil, 0x132, 0xeb, 0x13e, 0xa9, 0, 0, 0, 0, 1, 0, -1); err != nil {
		return err
	}
	g.drawFltscrnShipDestinationLines(0x1d)
	if err := g.drawFltscrnSmallShipIcons(); err != nil {
		return err
	}
	if f.scannedSmall > -1 {
		if err := g.printGalmapScannedShip(int(f.scannedSmall)); err != nil {
			return err
		}
	}
	if f.smallPtr > -1 && fleet {
		if err := g.drawFlashingSmallShipIcon(); err != nil {
			return err
		}
	}
	if f.scannedBig > -1 && fleet {
		if err := g.printScannedShipName(); err != nil {
			return err
		}
	}
	for row := int16(0); row < o.nOfficers; row++ {
		if id := f.officerIDs[row]; id > -1 {
			if err := g.offscrnData(int(row), int(id)); err != nil {
				return err
			}
		}
	}
	switch {
	case o.buttonMode == 2:
		setAnimationFrame(o.dismissPic, 1)
		if err := e.Draw(0x1cf, 0x1b9, o.dismissPic); err != nil {
			return err
		}
	case o.dismissBtn == engine.NoField:
		if err := e.Draw(0x1cf, 0x1b9, o.dullDismiss); err != nil {
			return err
		}
	}
	switch {
	case o.buttonMode == 1:
		setAnimationFrame(o.poolBtnPic, 1)
		if err := e.Draw(0x184, 0x1b9, o.poolBtnPic); err != nil {
			return err
		}
	case o.poolBtn == engine.NoField:
		if err := e.Draw(0x184, 0x1b9, o.dullPool); err != nil {
			return err
		}
	}
	if !g.leadersInHiringPool() {
		if err := e.Draw(0x139, 0x1b9, o.dullHire); err != nil {
			return err
		}
	}
	t := e.Text()
	normal := g.exeColors(normalTextColors)
	if o.buttonMode == 0 {
		if err := e.Draw(300, 0x1b9, o.hireReplace); err != nil {
			return err
		}
		if o.scanned > -1 {
			cost, maint := g.officerCosts(int(o.scanned))
			var msg string
			if maint == 0 {
				msg = fmt.Sprintf(cFormat(g.hMessage(0x129)), int(cost))
			} else {
				msg = fmt.Sprintf(cFormat(g.hMessage(0x121)), int(cost), int(maint))
			}
			g.setFittedFontStyle(msg, normal, 4, 2, 0, 0x84, 1)
			if err := t.Print(0x13b, 0x1c2, msg); err != nil {
				return err
			}
		}
	}
	if !fleet && o.starShown != -1 {
		st := &s.Stars[o.starShown]
		name := save.CString(st.Name[:])
		msg := name
		if gov := st.Officer[s.PlayerNum]; gov != -1 {
			l := &s.Leaders[gov]
			ln := save.CString(l.Name[:])
			switch {
			case l.ETA <= 0:
				msg = fmt.Sprintf(cFormat(cStringAt(g.exe, offStarLeaderFormat)), name, ln)
			case l.ETA == 1:
				msg = fmt.Sprintf(cFormat(g.hMessage(0x92)), name, ln, int(l.ETA))
			default:
				msg = fmt.Sprintf(cFormat(g.hMessage(0x93)), name, ln, int(l.ETA))
			}
		}
		g.setFittedFontStyle(msg, normal, 3, 2, 0, 0xcb, 1)
		if err := t.PrintCentered(0x1d2, 0xd2, msg); err != nil {
			return err
		}
	}
	if g.ub.globalVar == 'c' {
		p, x := o.colBtn, 10
		if fleet {
			p, x = o.shBtn, 0x9c
		}
		setAnimationFrame(p, 1)
		if err := e.Draw(x, 0xb, p); err != nil {
			return err
		}
	}
	colFrame := 0
	if m.officerScrnType == 1 {
		setAnimationFrame(o.shBtn, 0)
		colFrame = 1
	} else {
		setAnimationFrame(o.shBtn, 1)
	}
	if err := e.Draw(0xa0, 10, o.shBtn); err != nil {
		return err
	}
	setAnimationFrame(o.colBtn, colFrame)
	if err := e.Draw(7, 10, o.colBtn); err != nil {
		return err
	}
	o.flashIncr++
	if o.flashIncr%0x19 == 0 {
		o.flashDir = -o.flashDir
		o.flashIncr = 0
	}
	if o.flashIncr%5 == 0 {
		o.flashValue += uint8(o.flashDir)
	}
	m.multiColoredLineStart = (m.multiColoredLineStart + 1) % 8
	return nil
}

// drawOfficerScreenToBack is Draw_Officer_Screen_To_Back_ (0x93a1e): the
// portraits (darkened while a leader travels or is only offered) under
// the screen picture, on the back page.
func (g *Game) drawOfficerScreenToBack() error {
	e, o, f, s := g.e, &g.off, &g.flt, &g.state
	v := e.V
	v.SetPageBack()
	v.ResetWindow()
	v.ClipOff()
	v.Fill(0, 0, 0x27f, 0x1df, 0)
	for row := 0; row < int(o.nOfficers); row++ {
		id := f.officerIDs[row]
		if id < 0 {
			continue
		}
		p := o.ldrpict[row]
		w, h := p.Width, p.Height
		l := &s.Leaders[id]
		if l.ETA > 0 || l.Status == 4 {
			p = o.ldrpict[row+4]
		}
		if err := e.Draw((0x4b-w)/2+0xc, row*0x6d+(0x5a-h)/2+0x25, p); err != nil {
			return err
		}
	}
	if err := e.Draw(0, 0, o.main); err != nil {
		return err
	}
	box := o.colony
	if g.offFleetType() {
		box = o.fleet
	}
	if err := e.Draw(300, 0xc, box); err != nil {
		return err
	}
	v.CopyBackToOff()
	o.drawToBack = false
	g.fullDrawScreen = false
	v.SetPageOff()
	return nil
}

// offscrnSystemPopup is Wrapper_For_Offscrn_System_Popup_ (0x974a6).
func (g *Game) offscrnSystemPopup(star int) error {
	if star > -1 && g.okToViewSystem(star, int(g.state.PlayerNum)) {
		if err := g.initSystemDisplay(star); err != nil {
			return err
		}
	}
	if star < 0 {
		g.logOnce("Draw_System_Display_Popup_ for no star is not ported")
		return nil
	}
	g.sd.offscreen = true
	defer func() { g.sd.offscreen = false }()
	return g.drawSystemDisplayPopup(star, false, "", 0x132, 0x11, 0, 0)
}

// offscrnData is Wrapper_For_Offscrn_Data_ (0x973cc).
func (g *Game) offscrnData(row, id int) error {
	return g.printOfficerData(row, id, 0x5c, row*0x6d+0x26, 8, row*0x6d+0x22, 0x53, 0x60, 0xcc)
}

// printScannedShipName is Print_Scanned_Ship_Name_ (0x964bb): the hovered
// big icon's ship and its captain.
func (g *Game) printScannedShipName() error {
	s := &g.state
	t := g.e.Text()
	ship := int(g.flt.big[g.flt.scannedBig].ship)
	g.shipOwner(ship)
	sh := &s.Ships[ship]
	t.SetOutlineColor(0, 0)
	name := save.CString(sh.Design.Name[:])
	msg := name
	if sh.Officer >= 0 {
		l := &s.Leaders[sh.Officer]
		ln := save.CString(l.Name[:])
		switch {
		case l.ETA < 1:
			msg = fmt.Sprintf(cFormat(cStringAt(g.exe, offStarLeaderFormat)), name, ln)
		case l.ETA == 1:
			msg = fmt.Sprintf(cFormat(g.hMessage(0x92)), name, ln, int(l.ETA))
		default:
			msg = fmt.Sprintf(cFormat(g.hMessage(0x93)), name, ln, int(l.ETA))
		}
	}
	g.setFittedFontStyle(msg, g.exeColors(normalTextColors), 3, 2, font.Outline, 0xcc, 1)
	return t.PrintCentered(0x1d2, 0xd3, msg)
}

// officerCosts is Get_Officer_Costs_ (0x93d8e): the hiring cost and the
// maintenance, none for famous leaders and megawealth.
func (g *Game) officerCosts(l int) (cost, maint int16) {
	p := int(g.state.PlayerNum)
	cost = g.officerCost(p, l)
	maint = g.officerMaintenance(p, l)
	if g.exe.u32(famousMask)&g.state.Leaders[l].CommonSkills != 0 || g.leaderHasSkill(l, 0x1d) {
		maint = 0
	}
	return
}

// officerScreenUserBox is Officer_Screen_User_Box_ (0x94b3a): a box of the
// type, then the screen redrawn; the answer for a confirmation, else -1.
func (g *Game) officerScreenUserBox(msg string, typ int) (int, error) {
	e, u := g.e, &g.ub
	res := -1
	var err error
	switch typ {
	case 1:
		var yes bool
		yes, err = g.confirmationBox(msg)
		res = int(b2u8(yes))
	case 3:
		err = g.warningBox(msg)
	case 5:
		if u.globalVar < 0 {
			u.globalVar = 0x7e
		}
		err = g.doTextBox(nil, msg, int(u.globalVar))
		u.globalVar = 0
	default:
		err = g.messageBox(msg)
	}
	if err != nil {
		return res, err
	}
	e.V.SetPageOff()
	g.off.drawToBack = true
	if err := g.drawOfficerScreen(); err != nil {
		return res, err
	}
	return res, e.TogglePages()
}

// checkOfficerFields is Check_Officer_Fields_ (0x9301e): clicks on the
// leader rows: hire, dismiss, send to the pool or assign, by the mode.
func (g *Game) checkOfficerFields(r, scan int) error {
	o, f, s := &g.off, &g.flt, &g.state
	fleet := g.offFleetType()
	res, leader, skill := g.scanOfficerFields(scan, r)
	switch res {
	case 0:
	case 1:
		if skill != -1 {
			bonus := int16(g.officerSkillBonus(int(leader), int(skill)))
			return g.printOfficerSkillHelp(int(skill), bonus, int(leader))
		}
		return g.findSelectedLeader(int(leader))
	default:
		return nil
	}
	if int16(o.selected) == leader {
		o.selected = -1
		return nil
	}
	if leader <= -1 {
		return nil
	}
	o.selected = int8(leader)
	sel := int(o.selected)
	if o.buttonMode == 0 || (o.buttonMode == -1 && s.Leaders[sel].Status == 4) {
		if _, err := g.doHireOfficer(sel); err != nil {
			return err
		}
		o.drawToBack = true
		if o.buttonMode != 0 {
			o.selected = -1
		}
		return nil
	}
	switch o.buttonMode {
	case 2:
		msg := fmt.Sprintf(cFormat(g.hMessage(0x126)), g.leaderName(sel, true))
		g.e.SetMouseList(engine.MouseArrow)
		yes, err := g.officerScreenUserBox(msg, 1)
		if err != nil {
			return err
		}
		if yes != 0 {
			g.dismissOfficer(sel)
			o.selected = -1
		} else {
			g.e.SetMouseList(g.exeMouseList(mouseListWho, 1))
		}
		o.drawToBack = true
		return nil
	case 1:
		if st := s.Leaders[sel].Status; st != 0 && st != 2 {
			if err := g.moveOfficerToLimbo(sel); err != nil {
				return err
			}
		} else {
			o.selected, o.buttonMode = -1, -1
			g.e.SetMouseList(engine.MouseArrow)
		}
		o.drawToBack = true
		return nil
	}
	if fleet {
		if o.offpopSel <= -1 {
			return nil
		}
		ship := int(f.big[o.offpopSel].ship)
		if g.leaderCanBeAssigned(sel) {
			if err := g.assignCaptain(sel, ship); err != nil {
				return err
			}
			o.drawToBack = true
			f.big[o.offpopSel].sel = false
			o.offpopSel = -1
		} else {
			msg := fmt.Sprintf(cFormat(g.hMessage(0x127)), g.leaderName(sel, false))
			if _, err := g.officerScreenUserBox(msg, 0); err != nil {
				return err
			}
		}
		o.selected = -1
		return nil
	}
	if o.starChosen <= -1 || !s.playerHasColony(int(s.PlayerNum), int(o.starChosen)) {
		return nil
	}
	if g.leaderCanBeAssigned(sel) {
		if s.Leaders[sel].Location != o.starChosen {
			if err := g.assignCaptain(sel, int(o.starChosen)); err != nil {
				return err
			}
			o.drawToBack = true
		}
		o.selected = -1
		return nil
	}
	var msg string
	if s.Leaders[sel].Status == 4 {
		msg = fmt.Sprintf(cFormat(g.hMessage(0x127)), g.leaderName(sel, false))
	} else {
		msg = fmt.Sprintf(cFormat(g.hMessage(0x128)), g.leaderName(sel, false), save.CString(s.Stars[o.starChosen].Name[:]))
	}
	if _, err := g.officerScreenUserBox(msg, 0); err != nil {
		return err
	}
	o.selected = -1
	return nil
}

// scanOfficerFields is Scan_Officer_Fields_ (0x96e95): 0 a row was
// clicked, 1 right-clicked (on a skill line: skill is set), 4 a row is
// hovered (_officer_scanned).
func (g *Game) scanOfficerFields(scan, r int) (res, leader, skill int16) {
	o, f := &g.off, &g.flt
	ff := &o.fields
	res, leader, skill = -1, -1, -1
	o.scanned = -1
	for row := 0; row < 4 && res == -1; row++ {
		for k := 0; k < 5 && res == -1; k++ {
			switch {
			case r == o.skillHelp[row][k]:
				res, leader = 0, f.officerIDs[row]
			case r == -o.skillHelp[row][k]:
				res, leader, skill = 1, f.officerIDs[row], int16(o.skillOf[row][k])
			}
		}
	}
	for row := 0; row < 4 && res == -1; row++ {
		switch {
		case r == ff.names[row] || r == ff.pics[row]:
			res, leader = 0, f.officerIDs[row]
		case -r == ff.pics[row]:
			res, leader = 1, f.officerIDs[row]
		}
	}
	if res != -1 {
		return
	}
	for row := 0; row < 4; row++ {
		if scan == ff.names[row] || scan == ff.pics[row] {
			res, o.scanned = 4, int8(f.officerIDs[row])
		}
		for k := 0; k < 5 && res == -1; k++ {
			if scan == o.skillHelp[row][k] {
				res, o.scanned = 4, int8(f.officerIDs[row])
			}
		}
	}
	return
}

// leaderCanBeAssigned is Leader_Can_Be_Assigned_ (0x94254).
func (g *Game) leaderCanBeAssigned(l int) bool {
	st := g.state.Leaders[l].Status
	return st == 0 || st == 1 || st == 2
}

// doHireOfficer is Do_Hire_Officer_ (0x98062).
func (g *Game) doHireOfficer(l int) (bool, error) {
	s := &g.state
	cost := g.officerCost(int(s.PlayerNum), l)
	g.e.SetMouseList(engine.MouseArrow)
	if s.Leaders[l].Status != 4 {
		msg := fmt.Sprintf(cFormat(g.hMessage(0x122)), g.leaderName(l, true))
		_, err := g.officerScreenUserBox(msg, 0)
		g.off.selected = -1
		return false, err
	}
	if int32(int16(cost)) > s.Players[s.PlayerNum].BC {
		// Not_Enough_BCs_Message_ (0x92cf8) prints the cost from EDX, which
		// Do_Hire_Officer_ left holding it (movsx edx, si).
		msg := fmt.Sprintf(cFormat(g.hMessage(0x123)), int(s.Players[s.PlayerNum].BC), g.leaderName(l, true), int(int16(cost)))
		_, err := g.officerScreenUserBox(msg, 0)
		g.off.selected = -1
		return false, err
	}
	return g.askForHire(l)
}

// askForHire is Ask_For_Hire_ (0x92d72): the hire popup for leader l.
func (g *Game) askForHire(l int) (bool, error) {
	o, mp, s := &g.off, &g.mp, &g.state
	g.e.DeactivateHelpList()
	o.fullDraw = true
	cost, maint := g.officerCosts(l)
	f := g.hMessage(0x125)
	if maint == 1 {
		f = g.hMessage(0x124)
	}
	// Hire_Officer_Popup_ (0xc7ab8): the popup on the officer screen.
	mp.hireMsg = cSprintf(f, g.leaderName(l, true), int(cost), int(maint))
	mp.onOfficerScreen = true
	mp.officer = l
	hired, err := g.randomNewOfficerPopup(int(s.PlayerNum), l)
	mp.onOfficerScreen = false
	if err != nil {
		return false, err
	}
	if !hired {
		if o.buttonMode == 0 {
			g.e.SetMouseList(g.exeMouseList(mouseListWho, 1))
			o.selected = -1
		}
	} else {
		g.hireOfficer(int(s.PlayerNum), l)
		o.buttonMode = -1
	}
	g.setOfficerScreenHelp()
	o.fullDraw = false
	g.e.AssignAutoFunction(g.drawOfficerScreen, 1)
	o.drawToBack = true
	return hired, nil
}

// hireOfficer is Hire_Officer_ (0x9412b).
func (g *Game) hireOfficer(p, l int) {
	s := &g.state
	cost, _ := g.officerCosts(l)
	s.Players[p].BC -= int32(cost)
	g.setOfficerToPlayer(l, p, true)
	if s.Leaders[l].Type == 0 && (g.leaderHasSpecialSkill(l, 0x12) || g.leaderHasSpecialSkill(l, 0x13)) {
		s.Players[p].GalacticLore = 1
	}
}

// dismissOfficer is Dismiss_Officer_ (0x98771).
func (g *Game) dismissOfficer(l int) {
	s := &g.state
	ld := &s.Leaders[l]
	loc := ld.Location
	if ld.Type == 0 {
		if loc > -1 && loc < s.NumShips {
			s.Ships[loc].Officer = -1
		}
	} else if loc > -1 && loc < s.NumStars {
		s.Stars[loc].Officer[ld.Owner] = -1
	}
	g.deassignOfficer(l)
	g.buildCaptainIDList()
	if err := g.getOfficerPictures(); err != nil {
		g.Log("Get_Officer_Pictures_: %v", err)
	}
	g.setOfficerScreenHelp()
	g.off.buttonMode = -1
}

// moveOfficerToLimbo is Move_Officer_To_Limbo_ (0x943a0): the leader leaves
// his post for the pool (status 2) until the screen is left.
func (g *Game) moveOfficerToLimbo(l int) error {
	o, s := &g.off, &g.state
	ld := &s.Leaders[l]
	if ld.Status == 4 {
		msg := fmt.Sprintf(cFormat(g.hMessage(299)), g.leaderName(l, false))
		_, err := g.officerScreenUserBox(msg, 3)
		return err
	}
	loc := ld.Location
	g.e.SetMouseList(engine.MouseArrow)
	if g.offFleetType() {
		if loc > -1 && loc < s.NumShips {
			s.Ships[loc].Officer = -1
		}
	} else if loc > -1 {
		s.Stars[loc].Officer[ld.Owner] = -1
	}
	o.buttonMode = -1
	ld.Status = 2
	o.selected = -1
	ld.Location = -1
	return nil
}

// checkForColonyCalculationUpdate is Check_For_Colony_Calculation_Update_
// (0x92f8a): stars a colony leader left are recalculated.
func (g *Game) checkForColonyCalculationUpdate() {
	o, s := &g.off, &g.state
	for i := range s.Leaders {
		l := &s.Leaders[i]
		if l.Type == 1 && l.Location != o.origStar[i] && l.Status == 0 {
			g.starColonyCalculation(int(o.origStar[i]), int(s.PlayerNum))
		}
	}
}

// changeScreenType is Change_Screen_Type_ (0x9453c): to the ship officers
// with the first own stack, or to the colony leaders with a star.
func (g *Game) changeScreenType(reset *bool) error {
	o, f, s, m := &g.off, &g.flt, &g.state, &g.ms
	f.smallPtr = -1
	m.systemIconScanned, f.scannedBig, f.scannedSmall = -1, -1, -1
	g.sd.starID = -1
	g.clearSystemDisplayLoadedFlag()
	if m.officerScrnType != 1 {
		g.setSmallShipStackPointer(false)
		if f.smallPtr != -1 && g.shipStackOwner(g.stacks.start[f.smallPtr]) != int(s.PlayerNum) {
			f.smallPtr = -1
		}
		if f.smallPtr > -1 {
			g.buildFltscrnBigIconFields(f.smallPtr)
		} else {
			g.logOnce("Build_Fltscrn_Big_Icon_Fields_(-1) reads before _ship_stack_start")
		}
		if err := g.reloadOfficerScreenPictures(); err != nil {
			return err
		}
		o.drawToBack = true
		o.skipShips = true
		if err := g.drawOfficerScreen(); err != nil {
			return err
		}
		if err := g.e.TogglePages(); err != nil {
			return err
		}
		if err := g.loadPlayerShipPalette(int(s.PlayerNum)); err != nil {
			return err
		}
		o.selected = -1
		f.oldSmallPtr = -1
	} else {
		switch {
		case o.starShown == -1 && g.previousScreen == ScreenColony:
			o.selected = -1
			o.starShown = g.screenData[4]
		case o.starShown == -1:
			o.selected = -1
			o.starShown = int16(g.starIDForOfficersScreen())
		}
		if err := g.initSystemDisplay(int(o.starShown)); err != nil {
			return err
		}
		o.drawToBack = true
		o.starChosen = o.starShown
		if err := g.reloadOfficerScreenPictures(); err != nil {
			return err
		}
	}
	*reset = true
	o.drawToBack = true
	return nil
}

// starIDForOfficersScreen is Get_Star_Id_For_Officers_Screen_ (0x85bdd):
// the system display's star if the player has a colony there, else the
// first such star from the capitol's on.
func (g *Game) starIDForOfficersScreen() int {
	s := &g.state
	p := int(s.PlayerNum)
	home := int(s.Planets[s.Players[p].HomePlanet].Star)
	star := -1
	if g.moveableBoxSelected(0) {
		star = int(s.MoveableBoxes[0].Content)
	}
	if star != -1 && s.playerHasColony(p, star) {
		return star
	}
	star = home
	for k := 0; k < int(s.NumStars); k++ {
		if s.playerHasColony(p, star) {
			break
		}
		star = (star + 1) % int(s.NumStars)
	}
	if star == -1 {
		star = home
	}
	return star
}

// findSelectedLeader is Find_Selected_Leader_ (0x97d59): a right click on a
// portrait shows where the leader is.
func (g *Game) findSelectedLeader(l int) error {
	o, f, s := &g.off, &g.flt, &g.state
	ld := &s.Leaders[l]
	loc := ld.Location
	var warn, info string
	if loc == -1 {
		id := 0x112
		if ld.Status == 0 {
			id = 0x111
		}
		warn = fmt.Sprintf(cFormat(g.hMessage(id)), g.leaderName(l, false))
	}
	if warn == "" {
		if ld.Type == 0 {
			sh := &s.Ships[loc]
			if sh.Status == 4 {
				d := absoluteLocation(sh.Location)
				warn = fmt.Sprintf(cFormat(g.hMessage(0x113)), g.leaderName(l, false), save.CString(sh.Design.Name[:]), save.CString(s.Stars[d].Name[:]))
			} else if stk := g.shipStackFromShipID(int(loc)); stk == -1 {
				warn = fmt.Sprintf(cFormat(g.hMessage(0x114)), g.leaderName(l, false))
			} else {
				same := int16(stk) == f.smallPtr
				f.smallPtr = int16(stk)
				f.shipToMove = loc
				if !same {
					info = fmt.Sprintf(cFormat(g.hMessage(0x115)), save.CString(sh.Design.Name[:]))
				}
			}
		} else {
			o.starChosen = absoluteLocation(loc)
			same := o.starChosen == o.starShown
			o.starShown = o.starChosen
			if !same {
				info = fmt.Sprintf(cFormat(g.hMessage(0x116)), save.CString(s.Stars[o.starChosen].Name[:]))
			}
		}
	}
	if warn != "" {
		_, err := g.officerScreenUserBox(warn, 3)
		return err
	}
	if info == "" {
		return nil
	}
	g.ub.globalVar = 0xf
	if g.settings[0xd4] == 0 {
		g.ub.globalVar = 0
	}
	_, err := g.officerScreenUserBox(info, 5)
	return err
}

// assignCaptain is Assign_Captain_ (0x987fb): leader l to a ship (ship
// officers) or a star (colony leaders), after the player confirms the
// travel time and the replacement of the one there.
func (g *Game) assignCaptain(l, target int) error {
	s := &g.state
	p := int(s.PlayerNum)
	var eta uint8
	var place string
	if g.offFleetType() {
		if g.shipOwner(target) != p {
			msg := fmt.Sprintf(cFormat(g.hMessage(0x117)), g.leaderName(l, false))
			_, err := g.officerScreenUserBox(msg, 3)
			return err
		}
		eta = g.getShipLeaderETA(p, l, target)
		place = save.CString(s.Ships[target].Design.Name[:])
	} else {
		eta = g.getStarLeaderETA(p, l, target)
		place = save.CString(s.Stars[target].Name[:])
	}
	var msg string
	if eta == 0 {
		msg = fmt.Sprintf(cFormat(g.hMessage(0x119)), g.leaderName(l, false), place)
	} else if g.settings[0xd2] == 4 {
		msg = fmt.Sprintf(cFormat(g.hMessage(0x118)), g.leaderName(l, false), int(eta), place)
	} else {
		msg = fmt.Sprintf(cFormat(g.hMessage(0x118)), int(eta), g.leaderName(l, false), place)
	}
	yes, err := g.officerScreenUserBox(msg, 1)
	if err != nil {
		return err
	}
	old := int16(-1)
	if g.offFleetType() {
		old = s.Ships[target].Officer
	} else if o := s.Stars[target].Officer[p]; int(o) != l {
		old = int16(o)
	}
	if yes != 0 && old != -1 {
		id := 0x11b
		if s.Leaders[old].ETA >= 1 {
			id = 0x11a
		}
		msg := fmt.Sprintf(cFormat(g.hMessage(id)), g.leaderName(int(old), true), place)
		if yes, err = g.officerScreenUserBox(msg, 1); err != nil {
			return err
		}
	}
	if yes == 0 {
		return nil
	}
	if g.offFleetType() {
		g.assignLeaderToShip(l, target, eta)
	} else {
		g.assignLeaderToStar(p, l, target, eta)
	}
	return nil
}

// printOfficerSkillHelp is Print_Officer_Skill_Help_ (0x9636d): the skill's
// description for the leader in a text box.
func (g *Game) printOfficerSkillHelp(skill int, bonus int16, l int) error {
	o, s := &g.off, &g.state
	name := fmt.Sprintf("%s", g.leaderName(l, true))
	if g.settings[0xd2] != 4 && s.Leaders[l].Title[0] != 0 {
		name += cStringAt(g.exe, commaChar)
	}
	switch skill {
	case 0x1c:
		bonus = 10
	case 0x1d:
		bonus = 0xf
	}
	desc := cSprintf(cFormat(o.skillDescs[skill/2]), name, bonus)
	title := []byte(o.skillNames[skill/2])
	for i, c := range title {
		if c == '_' {
			title[i] = ' '
		}
	}
	t := string(title)
	if err := g.textBox(&t, desc); err != nil {
		return err
	}
	if g.currentScreen == 0x1d && !o.fullDraw {
		g.e.V.SetPageOff()
		o.drawToBack = true
		if err := g.drawOfficerScreen(); err != nil {
			return err
		}
		return g.e.TogglePages()
	}
	return nil
}

// doOfficerScreenStuff is Do_Officer_Screen_Stuff_ (0x909fa), reached from
// Draw_Galaxy_Map_Box_: the hovered star framed and named under the map,
// and on the colony page the shown star framed, flashing when chosen.
func (g *Game) doOfficerScreenStuff(ox, oy int16, sx, sy int32) error {
	o, s, m := &g.off, &g.state, &g.ms
	t := g.e.Text()
	fleet := g.offFleetType()
	if star := int(m.galaxyMapScannedStar); star > -1 {
		st := &s.Stars[star]
		a, b := byte(0x21), byte(0x26)
		if st.Owner > -1 && st.Owner < 8 {
			c := mainPalettePlayerColors[s.Players[st.Owner].Color]
			if c == 0x20 {
				a, b = 8, 0x16
			} else {
				a, b = c+1, c+5
			}
		}
		colors := g.fontColors2(3, a, b)
		ms := int32(s.MaxMapScale)
		bx := (int32(st.X)*1000/ms)*10/0x637 + 0x132
		by := (int32(st.Y)*1000/ms)*10/0x93e + 0xeb
		g.box(int(int16(bx-5)), int(int16(by-5)), 0xb, 0xc, 1, 0x71)
		t.SetStyle(3, colors)
		p := int(s.PlayerNum)
		msg := g.hMessage(0x94)
		if s.playerHasVisited(p, star) || s.playerIsOmniscient(p) || s.contactWithOneColony(star) {
			msg = g.starWithGovernor(star)
		}
		g.setFittedFontStyle(msg, colors, 3, 2, 0, 0xcb, 1)
		if err := t.PrintCentered(0x1d2, 0x1a2, msg); err != nil {
			return err
		}
	}
	if !fleet && o.starShown > -1 {
		st := &s.Stars[o.starShown]
		ms := int32(s.MaxMapScale)
		x := int32(ox) + 0x132 + (int32(st.X)*1000/ms)*10/sx
		y := (int32(st.Y)*1000/ms)*10/sy + int32(oy) + 0xeb
		c := byte(0x1e)
		if o.starChosen == o.starShown {
			c = o.flashValue + 0x6e
		}
		g.box(int(int16(x-5)), int(int16(y-5)), 0xb, 0xc, 1, c)
	}
	return nil
}
