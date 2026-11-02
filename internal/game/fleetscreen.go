package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The fleet screen (screen 4) and the ship lists it shares with the leaders
// screen: MOX\FLT1.C, FLT2.C. Spec: docs/screens/fleet-screen.md.

const (
	fleetLBX = "FLEET.LBX"

	staticFleetScreenHelpList = 0x181b3c // _static_fleet_screen_help_list, 32 records
	mouseListFrom             = 0x181fe8 // FLT1.C's _mouse_list_from, 4 entries
	mouseListToFlt            = 0x182018 // FLT1.C's _mouse_list_to
	mouseListAllTo            = 0x182048 // _mouse_list_all_to
)

// Get_Input codes the fleet screen reads besides its fields.
const (
	keyCodeAltF5 = -1105
)

// fltBigIcon is one 13-byte _fltscrn_big_icon record (bytes 0..3 unread).
type fltBigIcon struct {
	sel   bool  // +4
	x, y  int16 // +5, +7
	field int   // +9
	ship  int16 // +0xb
}

// fltFields are Fleet_Screen_'s field locals.
type fltFields struct {
	relocate, scrap, ret, up, down, next, prev, officers, all, scroll int
}

// fltScreen holds the FLT1.C and FLT2.C globals; the leaders screen reuses
// most of them.
type fltScreen struct {
	tooManyBox              bool // port-only: "Too many ships" is up (buildFltscrnShipList)
	big                     [250]fltBigIcon
	nBig, nAdded, nSelected int16 // _n_fltscrn_big_icons, _added, _n_big_icons_selected
	nRows                   uint8
	selStatus               [250]byte  // _fltscrn_icon_selection_status
	shipList                [250]int16 // _fltscrn_ship_icon_selection_list
	officerIDs              [67]int16  // _officer_id_list
	officerPics             []*gfx.Picture
	bar                     scrollBar // the block _scroll_bar points to

	smallPtr, oldSmallPtr     int16 // _small_ship_stack_ptr, _old_small_ship_stack_ptr
	stackOwner, oldStackOwner int16
	scannedSmall, scannedBig  int16
	skipDrawShips             bool
	relocateMode              uint8 // 0 off, 1 picking from/to stars, 2 all to
	merging                   bool  // FLT1.C's _merging_relocations
	addScrap, addAll          bool
	addOfficers               bool
	supportStatus             int16 // radio button values, 1 after Harold_Defaults_
	combatStatus              int16
	supportBtn, combatBtn     int
	xOff, yOff                int16
	cols, rows, maxIcons      uint8
	oldFirst                  int16
	shipToMove                int16 // _fleet_icon_ship_id_to_move, 0 at program start
	selPaint                  int8  // _fleet_ship_selection_status
	nPlayerShips              int16
	badShip                   int16
	fields                    fltFields

	background, scrollThumb               *gfx.Picture
	upBtn, prevBtn, nextBtn, downBtn      *gfx.Picture
	allBtn, relocateBtn, scrapBtn         *gfx.Picture
	supportPic, combatPic                 *gfx.Picture
	officersBtn, returnBtn                *gfx.Picture
	dimAll, dimRelocate, dimScrap, dimOff *gfx.Picture
	selectedBox, scannedBox               *gfx.Picture
	galaxyStars                           [11]*gfx.Picture // _fleet_galaxy_star_seg
	smallIcon, flashIcon                  [8]*gfx.Picture
	monsterIcon, flashMonster             [7]*gfx.Picture
}

// fleetScreen is Fleet_Screen_ (0x73980).
func (g *Game) fleetScreen() error {
	e, f, s, m := g.e, &g.flt, &g.state, &g.ms
	savedScale := s.CurMapScale
	g.clearSystemDisplayLoadedFlag()
	g.fb.curBar = nil
	f.xOff, f.yOff, f.cols, f.rows, f.maxIcons = 0x15b, 0x35, 4, 5, 0x14
	f.relocateMode = 0
	if err := g.reloadFleetScreen(); err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawFleetScreen, 1)
	m.drawingOfficerPopup = false
	g.fb.overlayX, g.fb.overlayY = -1, -1
	m.galaxyMap.colonyPalette = false
	f.nAdded, f.nBig, f.nRows, f.nSelected = 0, 0, 0, 0
	f.scannedSmall, f.scannedBig = -1, -1
	m.galaxyMap.starAnimCount = 0
	f.nPlayerShips = g.nPlayerShips(int(s.PlayerNum))
	g.setFltscrnSmallShipIconXYs(0xf, 0x34, 0x131, 0xb6)
	g.setInitialStackPointer()
	f.oldSmallPtr = -1
	g.ub.globalVar = 0
	f.skipDrawShips = true
	f.badShip = -1
	e.ClearFields()
	resetBar, skipDraw, leave := false, false, false
	relFrom, relTo := int16(-1), int16(-1)
	if err := g.addFleetScreenFields(); err != nil {
		return err
	}
	f.bar.initialize(5, 0xea, 0)
	g.setFleetScreenHelpList(f.nAdded)
	e.DisableDrawAllFields()
	// _refresh_mode = 2: read only by Set_Refresh_, which nothing calls.
	e.V.SetRefreshStencil()
	m.drawFleetScreenToBack = true
	e.SetMouseList(engine.MouseNone)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	if err := e.LoadPalette(8, 0, 0xff); err != nil {
		return err
	}
	e.V.SetPaletteChanges(0, 0xff)
	if f.nPlayerShips > 0 {
		if err := g.loadPlayerShipPalette(int(s.Ships[f.big[0].ship].Owner)); err != nil {
			return err
		}
	}
	if err := e.DarkenPalette(100); err != nil {
		return err
	}
	e.V.SetPageOff()
	if err := g.drawFleetScreen(); err != nil {
		return err
	}
	if f.nPlayerShips > 0 {
		setAnimationFrame(f.supportPic, 1)
		if err := e.Draw(0x1a9, 0x1b3, f.supportPic); err != nil {
			return err
		}
		setAnimationFrame(f.combatPic, 1)
		if err := e.Draw(0x1e7, 0x1b3, f.combatPic); err != nil {
			return err
		}
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
	for !leave {
		f.selPaint = -1
		e.ClearFields()
		if err := g.addFleetScreenFields(); err != nil {
			return err
		}
		e.MarkTime()
		// _fix_palette = 0: nothing sets it, so Fix_Palette_ never runs.
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return err
		}
		ff := &f.fields
		switch {
		case r < 0 && f.relocateMode == 1:
			relFrom, relTo = -1, -1
			f.relocateMode = 0
			e.SetMouseList(engine.MouseArrow)
		case r == keyCodeAltF5:
			g.clearAllStarRelocations(int(s.PlayerNum))
			if _, err := g.userBox(&m.drawFleetScreenToBack, g.hMessage(0x76), boxMessage); err != nil {
				return err
			}
		case r == keyCodeF5:
			if f.merging {
				f.relocateMode = 0
				f.merging = false
				e.SetMouseList(engine.MouseArrow)
			} else {
				f.relocateMode = 2
				f.merging = true
				e.SetMouseList(g.exeMouseList(mouseListAllTo, 4))
			}
		}
		if f.stackOwner == s.PlayerNum {
			res, scanned, clicked := g.scanFltscrnBigIcons(scan, r)
			if scanned > -1 {
				m.galaxyMapScannedStar = -1
				f.scannedSmall = -1
				f.scannedBig = scanned
			}
			if res == 1 {
				e.DeactivateHelpList()
				if err := g.detailedViewShip(int(f.big[clicked].ship)); err != nil {
					return err
				}
				g.setFleetScreenHelpList(f.nAdded)
				m.drawFleetScreenToBack = true
			}
		}
		res, hov, clk := g.scanGalaxyMapFields(scan, r)
		if hov > -1 {
			f.scannedSmall = -1
			m.galaxyMapScannedStar = int16(hov)
		}
		switch {
		case res == 1 && f.relocateMode == 1:
			// Scan_Galaxy_Map_Fields_ never returns 1.
			if err := g.cancelStarRelocation(clk); err != nil {
				return err
			}
		case res == 0:
			if err := g.fleetScreenStarClicked(clk, &relFrom, &relTo); err != nil {
				return err
			}
		}
		sres, sscan := g.scanSmallShipIcons(scan, r)
		switch {
		case sres == 0:
			m.drawFleetScreenToBack = true
			f.bar.pos = f.bar.track
			if f.smallPtr != f.oldSmallPtr {
				if f.oldSmallPtr == -1 || f.smallPtr == -1 ||
					g.shipStackOwner(g.stacks.start[f.oldSmallPtr]) != g.shipStackOwner(g.stacks.start[f.smallPtr]) {
					if f.oldSmallPtr == -1 || f.smallPtr == -1 {
						m.drawFleetScreenToBack = true
					}
					f.skipDrawShips = true
				}
				f.scannedBig = -1
			}
		case sres == 4 && sscan > -1:
			m.galaxyMapScannedStar = -1
			f.scannedSmall = int16(sscan)
		}
		if r == ff.all && f.nBig > 0 && g.shipOwner(int(f.big[0].ship)) == int(s.PlayerNum) {
			g.setFltscrnBigIcons(f.nSelected != f.nBig)
		}
		if r == ff.ret {
			g.currentScreen = ScreenMain
			leave = true
			e.ClearFields()
			e.DeactivateAutoFunction()
			m.galaxyMapScannedStar = -1
			g.moveFromLimboToPool()
		}
		if r == ff.scrap {
			list := g.buildScrapList()
			done, err := g.scrapShips(int(s.PlayerNum), list, true)
			if err != nil {
				return err
			}
			all := g.nFltscrnShipsSelected() == f.nBig
			f.oldSmallPtr = -1
			m.drawFleetScreenToBack = true
			if all && done {
				f.nBig = 0
				m.galaxyMapScannedStar = -1
				g.buildShipIcons()
				g.setFltscrnSmallShipIconXYs(0xf, 0x34, 0x131, 0xb6)
				g.setSmallShipStackPointer(false)
			}
			f.scannedBig = -1
			f.nPlayerShips = g.nPlayerShips(int(s.PlayerNum))
		}
		if r == ff.officers {
			if !f.addOfficers {
				if _, err := g.userBox(&m.drawFleetScreenToBack, g.hMessage(0x79), boxMessage); err != nil {
					return err
				}
			} else {
				g.currentScreen, g.returnScreen = 0x1d, 4
				leave = true
				g.screenData[4] = g.sd.starID
			}
		}
		switch r {
		case ff.down:
			f.bar.scrollDownOneItem()
		case ff.up:
			f.bar.scrollUpOneItem()
		}
		if r == ff.relocate && f.relocateMode != 2 {
			switch f.relocateMode {
			case 0:
				e.SetMouseList(g.exeMouseList(mouseListFrom, 4))
				f.relocateMode = 1
			case 1:
				e.SetMouseList(engine.MouseArrow)
				f.relocateMode = 0
				relFrom, relTo = -1, -1
			}
		}
		if r == ff.next || r == keyCodeF2 {
			old := f.smallPtr
			g.nextShipIcon()
			if old != f.smallPtr {
				f.skipDrawShips = true
				f.bar.pos = -1
				f.scannedBig = -1
				m.drawFleetScreenToBack = true
				resetBar = true
			}
		}
		if r == ff.prev || r == keyCodeF1 {
			old := f.smallPtr
			g.previousShipIcon()
			if old != f.smallPtr {
				f.scannedBig = -1
				f.bar.pos = -1
				f.skipDrawShips = true
				m.drawFleetScreenToBack = true
				resetBar = true
			}
		}
		if r == f.supportBtn || r == f.combatBtn {
			if r == f.supportBtn && f.supportStatus == 0 && f.combatStatus == 0 {
				f.combatStatus = 1
			}
			if r == f.combatBtn && f.supportStatus == 0 && f.combatStatus == 0 {
				f.supportStatus = 1
			}
			if f.smallPtr > -1 && f.smallPtr < g.stacks.count && g.fltscrnShipsToBeDisplayed(g.stacks.start[f.smallPtr]) == 0 {
				g.nextShipIcon()
			}
			skipDraw = true
			f.bar.pos = f.bar.track
			f.skipDrawShips = true
			m.drawFleetScreenToBack = true
			f.bar.first = -1
		}
		if resetBar {
			f.bar.initialize(5, 0xea, 0)
			resetBar = false
		}
		g.setFleetScreenHelpList(f.nAdded)
		if !leave {
			if f.smallPtr > -1 {
				f.nSelected = g.updateSelectionFlags()
			}
			if skipDraw {
				skipDraw = false
			} else {
				e.V.SetPageOff()
				if err := g.drawFleetScreen(); err != nil {
					return err
				}
				if err := e.TogglePages(); err != nil {
					return err
				}
			}
		}
		if err := e.ReleaseTime(1); err != nil {
			return err
		}
	}
	if g.moveableBoxSelected(2) {
		g.fb.stack = f.smallPtr
		if f.smallPtr == -1 {
			g.clearFleetBoxSelectedShipStatus()
			g.vanishFleetMovementBox()
		}
	}
	e.SetMouseList(engine.MouseArrow)
	m.reloadedShipPalette = false
	s.CurMapScale = savedScale
	return nil
}

// fleetScreenStarClicked is Fleet_Screen_'s branch for a click on a star of
// the galaxy map.
func (g *Game) fleetScreenStarClicked(star int, relFrom, relTo *int16) error {
	f, s, m := &g.flt, &g.state, &g.ms
	switch {
	case f.merging:
		f.merging = false
		g.e.SetMouseList(engine.MouseArrow)
		f.relocateMode = 0
		g.setAllStarRelocations(star, int(s.PlayerNum))
		msg := fmt.Sprintf(cFormat(g.hMessage(0x77)), save.CString(s.Stars[star].Name[:]))
		if _, err := g.userBox(&m.drawFleetScreenToBack, msg, boxMessage); err != nil {
			return err
		}
		*relFrom, *relTo = -1, -1
	case f.relocateMode == 1:
		return g.starRelocation(relFrom, relTo, star)
	case s.Stars[star].SpectralClass == save.SpectralBlackHole:
		_, err := g.userBox(&m.drawFleetScreenToBack, g.hMessage(0x78), boxWarning)
		return err
	default:
		ship := g.firstSelectedShip()
		if ship > -1 && g.playerCanOrderShip(int(s.PlayerNum), int(ship)) {
			return g.fltscrnMoveShips(star)
		}
		if ship > -1 {
			msg := fmt.Sprintf(cFormat(g.hMessage(0x75)), save.CString(s.Ships[ship].Design.Name[:]))
			if _, err := g.userBox(&m.drawFleetScreenToBack, msg, boxMessage); err != nil {
				return err
			}
			f.badShip = -1
		}
	}
	return nil
}

// initialize is Initialize_Scroll_Bar_'s inlined form (0x72ea6) with the
// visible rows, track length and kind of the screen.
func (b *scrollBar) initialize(visible, track int16, kind uint8) {
	b.visible = visible
	b.track = track
	b.h = b.height()
	b.last = -1
	b.kind = kind
	b.step = 1
	b.pos = b.track - b.h/2
}

// exeMouseList reads an s_mouse_list of n entries from the image.
func (g *Game) exeMouseList(addr uint32, n int) []engine.MouseRegion {
	l := make([]engine.MouseRegion, n)
	for i := range l {
		a := addr + uint32(i*12)
		l[i] = engine.MouseRegion{Image: int(g.exeI16(a)), Center: int(g.exeI16(a + 2)), X1: int(g.exeI16(a + 4)),
			Y1: int(g.exeI16(a + 6)), X2: int(g.exeI16(a + 8)), Y2: int(g.exeI16(a + 10))}
	}
	return l
}

// reloadFleetScreen is Reload_Fleet_Screen_ (0x7464a).
func (g *Game) reloadFleetScreen() error {
	f, s := &g.flt, &g.state
	var err error
	load := func(entry int) *gfx.Picture {
		if err != nil {
			return nil
		}
		var p *gfx.Picture
		p, err = g.picture(fleetLBX, entry)
		return p
	}
	f.background = load(0)
	f.scrollThumb = load(1)
	f.upBtn = load(2)
	f.prevBtn = load(3)
	f.nextBtn = load(4)
	f.downBtn = load(5)
	f.allBtn = load(6)
	f.relocateBtn = load(7)
	f.scrapBtn = load(8)
	f.supportPic = load(9)
	f.combatPic = load(10)
	f.officersBtn = load(0xb)
	f.returnBtn = load(0xc)
	f.dimAll = load(0xd)
	f.dimRelocate = load(0xe)
	f.dimScrap = load(0xf)
	f.dimOff = load(0x10)
	f.selectedBox = load(0x11)
	f.scannedBox = load(0x12)
	if err != nil {
		return err
	}
	g.allocateFltscrnNextScreenSeg()
	f.officerPics = f.officerPics[:0]
	for i := 0; i < maxLeaders; i++ {
		l := &s.Leaders[i]
		if int(l.Owner) == int(s.PlayerNum) && l.Status == 1 && l.Type == 0 {
			p, err := g.picture(fleetLBX, int(l.Picture)+0x2d)
			if err != nil {
				return err
			}
			f.officerIDs[len(f.officerPics)] = int16(i)
			f.officerPics = append(f.officerPics, p)
		}
	}
	if err := g.reloadSmallShipIcons(fleetLBX, 0x13); err != nil {
		return err
	}
	for i := range f.galaxyStars {
		if f.galaxyStars[i] = load(0x22 + i); err != nil {
			return err
		}
	}
	return nil
}

// allocateFltscrnNextScreenSeg is Allocate_Fltscrn_Next_Screen_Seg_
// (0x72a65): only the scroll bar is cleared; the icon tables keep what the
// screen segment held before.
func (g *Game) allocateFltscrnNextScreenSeg() {
	g.flt.bar = scrollBar{}
	g.fb.curBar = &g.flt.bar
}

// reloadSmallShipIcons is Reload_Small_Ship_Icons_ (0x74937) and
// Reload_Offscrn_Small_Ship_Icons_: eight banner icons, each loaded twice
// (normal and flashing), then seven monster icons.
func (g *Game) reloadSmallShipIcons(file string, entry int) error {
	f := &g.flt
	var err error
	load := func() *gfx.Picture {
		if err != nil {
			return nil
		}
		var p *gfx.Picture
		p, err = g.picture(file, entry)
		return p
	}
	for i := range f.smallIcon {
		f.smallIcon[i] = load()
		f.flashIcon[i] = load()
		entry++
	}
	for i := range f.monsterIcon {
		f.monsterIcon[i] = load()
		f.flashMonster[i] = load()
		entry++
	}
	return err
}

// nPlayerShips is N_Player_Ships_ (0x79b73).
func (g *Game) nPlayerShips(p int) int16 {
	n := int16(0)
	for i := 0; i < maxShips; i++ {
		if g.shipOwner(i) == p && int8(g.state.Ships[i].Status) < 3 {
			n++
		}
	}
	return n
}

// setFltscrnSmallShipIconXYs is Set_Fltscrn_Small_Ship_Icon_XYs_ (0x77048):
// the ship icons placed on a galaxy map of w×h at (x, y); it leaves the map
// scale at its maximum.
func (g *Game) setFltscrnSmallShipIconXYs(x, y, w, h int16) {
	s, st := &g.state, &g.stacks
	zoom := g.mapScaleToZoomLevel()
	s.CurMapScale = s.MaxMapScale
	n := int(s.NumStars)
	sx, sy := make([]int16, n), make([]int16, n)
	for i := 0; i < n; i++ {
		sx[i], sy[i] = g.galaxyMapStarXY(i, x, y, w, h, 0, 0, true)
	}
	for i := 0; i < int(st.iconCount); i++ {
		ic := &st.icon[i]
		star := int(actualStarID(ic.location))
		iw, ih := g.smallIconAnimationSize(i)
		var px, py int16
		switch {
		case ic.slot == 0:
			px, py = sx[star]+4, sy[star]-3-ih/2
		case ic.slot == 1:
			sh := &s.Ships[st.node[ic.node].ship]
			star = g.xyToStarID(sh.X, sh.Y)
			px, py = sx[star]-iw/2, sy[star]-3-ih/2
		case ic.slot == 2:
			px, py = sx[star]-iw, sy[star]-5
		case ic.slot < 5:
			px, py = sx[star]-iw, sy[star]-5+ih
		default:
			dx := int32(1000*0x1fa) / int32(w)
			dy := int32(1000*400) / int32(h)
			sh := &s.Ships[st.node[ic.node].ship]
			a := int32(int16(g.scaled(sh.X)))
			b := int32(int16(g.scaled(sh.Y)))
			g.shipIconDimensions(zoom)
			px = int16(int32(x) + a*1000/dx - int32(iw/2))
			py = int16(int32(y) + b*1000/dy - int32(ih/2))
		}
		ic.x, ic.y = px, py
	}
}

// actualStarID is Actual_Star_Id_ (0x78e2d) without its kind.
func actualStarID(l int16) int16 {
	switch {
	case l >= 500 && l < 1000:
		return l - 500
	case l >= 1000 && l < 0x5dc:
		return l - 1000
	}
	return l
}

// smallIconPicture is the picture Get_Small_Icon_Animation_Size_ and the
// drawing pick for ship icon i.
func (g *Game) smallIconPicture(i int, flash bool) *gfx.Picture {
	f, st := &g.flt, &g.stacks
	node := st.icon[i].node
	owner := g.shipStackOwner(node)
	if owner < 8 {
		if flash {
			return f.flashIcon[g.state.Players[owner].Color]
		}
		return f.smallIcon[g.state.Players[owner].Color]
	}
	m := int(g.state.Ships[st.node[node].ship].Owner) - 8
	if flash {
		return f.flashMonster[m]
	}
	return f.monsterIcon[m]
}

// smallIconAnimationSize is Get_Small_Icon_Animation_Size_ (0x773b7).
func (g *Game) smallIconAnimationSize(i int) (w, h int16) {
	p := g.smallIconPicture(i, false)
	return int16(p.Width), int16(p.Height)
}

// setInitialStackPointer is Set_Initial_Stack_Pointer_ (0x772bf), which
// falls into Set_Small_Ship_Stack_Pointer_ with the flag set.
func (g *Game) setInitialStackPointer() { g.setSmallShipStackPointer(true) }

// setSmallShipStackPointer is Set_Small_Ship_Stack_Pointer_ (0x772c4): the
// fleet box's stack when it is open and box is set, else the player's first
// icon.
func (g *Game) setSmallShipStackPointer(box bool) {
	f, st := &g.flt, &g.stacks
	f.smallPtr, f.stackOwner = -1, -1
	if !g.moveableBoxSelected(2) || !box {
		for i := 0; i < int(st.iconCount); i++ {
			if g.shipStackOwner(st.icon[i].node) == int(g.state.PlayerNum) {
				f.smallPtr = int16(g.shipStackFromShipIcon(i))
				if f.smallPtr > -1 {
					break
				}
			}
		}
	} else {
		f.smallPtr = g.fb.stack
	}
	if f.smallPtr > -1 {
		g.setFleetScreenStackOwner()
		f.oldStackOwner = f.stackOwner
	}
}

// setFleetScreenStackOwner is Set_Fleet_Screen_Stack_Owner_ (0x77014).
func (g *Game) setFleetScreenStackOwner() {
	f := &g.flt
	f.stackOwner = -1
	if f.smallPtr > -1 {
		f.stackOwner = int16(g.shipOwner(int(g.stacks.node[g.stacks.start[f.smallPtr]].ship)))
	}
}

// setFleetScreenHelpList is Set_Fleet_Screen_Help_List_ (0x6f88f): the
// static entries plus one per empty big icon slot from n on.
func (g *Game) setFleetScreenHelpList(n int16) {
	list := g.helpListAt(staticFleetScreenHelpList, 32)
	count := 0xc
	if n == 0 {
		list[12] = engine.HelpEntry{ID: 0x168, X1: 0x158, Y1: 0x35, X2: 0x24f, Y2: 0x161}
		count = 13
	} else {
		for ; n < 0x14; n++ {
			c, r := int(n%4)*0x40, int(n/4)*0x40
			list[count] = engine.HelpEntry{ID: 0x168, X1: c + 0x156, Y1: r + 0x34, X2: c + 0x196, Y2: r + 0x72}
			count++
		}
	}
	g.e.DeactivateHelpList()
	g.e.SetHelpList(list[:count])
}

// addFleetScreenFields is Add_Fleet_Screen_Fields_ (0x7267d).
func (g *Game) addFleetScreenFields() error {
	e, f, s := g.e, &g.flt, &g.state
	ff := &f.fields
	var err error
	button := func(x, y int, p *gfx.Picture, hot string) int {
		if err != nil {
			return engine.NoField
		}
		var n int
		n, err = e.AddButtonField(x, y, "", p, hot, 0x28)
		return n
	}
	mine := f.stackOwner == s.PlayerNum
	f.addScrap = f.nSelected >= 1 && mine && f.relocateMode != 1
	ff.scrap = engine.NoField
	if f.addScrap {
		ff.scrap = button(0x225, 0x17c, f.scrapBtn, "S")
	}
	f.addAll = f.nBig >= 1 && mine && f.relocateMode != 1
	ff.all = engine.NoField
	if f.addAll {
		ff.all = button(0x15c, 0x17c, f.allBtn, "A")
	}
	ff.ret = button(0x22c, 0x1ae, f.returnBtn, "\x1b")
	ff.up, ff.down, ff.prev, ff.next = engine.NoField, engine.NoField, engine.NoField, engine.NoField
	if f.nPlayerShips > 0 {
		if f.nBig > 0x14 {
			ff.up = button(0x25e, 0x3b, f.upBtn, "-")
			ff.down = button(0x25d, 0x145, f.downBtn, "+")
		}
		ff.prev = button(0x13, 0xf9, f.prevBtn, ",")
		ff.next = button(0x11b, 0xf9, f.nextBtn, ".")
	}
	if err != nil {
		return err
	}
	f.bar.downField, f.bar.upField = ff.down, ff.up
	if f.relocateMode == 2 {
		ff.relocate = 0
	} else if ff.relocate, err = e.AddHiddenField(0x1b9, 0x17c, f.relocateBtn.Width+0x1b9, f.relocateBtn.Height+0x17c, "R", ')'); err != nil {
		return err
	}
	f.addOfficers = g.atLeastOneOfficer()
	if !f.addOfficers {
		ff.officers, err = e.AddHiddenField(0x156, 0x1ae, f.officersBtn.Width+0x156, f.officersBtn.Height+0x1ae, "", ')')
	} else {
		ff.officers, err = e.AddButtonField(0x156, 0x1ae, "", f.officersBtn, "L", 0x28)
	}
	if err != nil {
		return err
	}
	if err := g.addFltscrnSmallIconFields(); err != nil {
		return err
	}
	if f.smallPtr > -1 {
		if ff.scroll, err = g.fleetScreenBigIconFields(0x25d, 0x56, 0xe, 0xea); err != nil {
			return err
		}
		f.bar.rows = int16(f.nRows)
	}
	if err := g.addGalaxyMapFields2(0xf, 0x34, 0x131, 0xb6, 0, 0, true); err != nil {
		return err
	}
	if g.ms.fields.debug, err = e.AddHiddenField(0, 0x1d6, 10, 0x1df, "", ')'); err != nil {
		return err
	}
	if f.nPlayerShips < 1 {
		f.supportBtn, f.combatBtn = engine.NoField, engine.NoField
	} else {
		if f.supportBtn, err = e.AddRadioButtonField(0x1a9, 0x1b3, "", f.supportPic, &f.supportStatus, "U", 0x29); err != nil {
			return err
		}
		if f.combatBtn, err = e.AddRadioButtonField(0x1e7, 0x1b3, "", f.combatPic, &f.combatStatus, "C", 0x29); err != nil {
			return err
		}
	}
	_, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0)
	return err
}

// atLeastOneOfficer is At_Least_One_Officer_ (0x74f14): a ship officer or
// colony leader of the player not for hire.
func (g *Game) atLeastOneOfficer() bool {
	s := &g.state
	for i := 0; i < maxLeaders; i++ {
		l := &s.Leaders[i]
		if int(l.Owner) == int(s.PlayerNum) && l.Status > -1 {
			return true
		}
	}
	return false
}

// addFltscrnSmallIconFields is Add_Fltscrn_Small_Icon_Fields_ (0x755d2).
func (g *Game) addFltscrnSmallIconFields() error {
	st := &g.stacks
	for i := 0; i < int(st.iconCount); i++ {
		w, h := g.smallIconAnimationSize(i)
		ic := &st.icon[i]
		n, err := g.e.AddHiddenField(int(ic.x), int(ic.y), int(ic.x+w), int(ic.y+h), "", ')')
		if err != nil {
			return err
		}
		ic.field = int16(n)
	}
	return nil
}

// addGalaxyMapFields2 is Add_Galaxy_Map_Fields_2_ (0x9023b) into
// _galaxy_map_star_field and _galaxy_map_grid_field.
func (g *Game) addGalaxyMapFields2(x, y, w, h, ox, oy int16, big bool) error {
	e, s, gm := g.e, &g.state, &g.ms.galaxyMap
	e.V.SetWindow(int(x+ox), int(y+oy), int(w+x+ox), int(h+y+oy))
	if g.currentScreen == ScreenColony {
		gm.gridField = engine.NoField
	}
	n := int(s.NumStars)
	if len(gm.starField) < n {
		gm.starField = make([]int, n)
	}
	for i := 0; i < n; i++ {
		px, py := g.galaxyMapStarXY(i, x, y, w, h, ox, oy, big)
		x1, y1, x2, y2 := px, py, px+5, py+5
		if big {
			x1, y1, x2, y2 = px-3, py-3, px+8, py+9
		}
		f, err := e.AddHiddenField(int(x1), int(y1), int(x2), int(y2), "", ')')
		if err != nil {
			return err
		}
		gm.starField[i] = f
	}
	if g.currentScreen == ScreenColony {
		g.logOnce("Add_Galaxy_Map_Fields_2_'s grid field (colony screen) is not ported")
	}
	e.V.ResetWindow()
	return nil
}

// fleetScreenBigIconFields is Fleet_Screen_Big_Icon_Fields_ (0x75b80): the
// stack's big icons rebuilt when the stack or first row changed, their
// fields, and the scroll field when there is more than one row.
func (g *Game) fleetScreenBigIconFields(x, y, w, h int) (int, error) {
	f := &g.flt
	rebuild := f.smallPtr > -1 && (f.oldSmallPtr != f.smallPtr || f.oldFirst != f.bar.first || f.oldFirst == -1)
	if rebuild {
		f.oldSmallPtr = f.smallPtr
		f.oldFirst = f.bar.first
		g.buildFltscrnBigIconFields(f.smallPtr)
		if err := g.loadPlayerShipPalette(g.shipStackOwner(g.stacks.start[f.smallPtr])); err != nil {
			return 0, err
		}
	} else if f.shipToMove > -1 && f.nBig > 1 {
		for i := int16(0); i < f.nBig; i++ {
			if f.big[i].ship == f.shipToMove {
				g.moveShipToFrontOfList(i)
				break
			}
		}
	}
	if f.nBig > 0 {
		f.nRows = uint8(int(f.nBig) / int(f.cols))
		if int(f.nBig)%int(f.cols) > 0 {
			f.nRows++
		}
	}
	if f.nBig < 1 {
		f.bar.first = 0
	} else {
		if f.bar.first < 0 || f.bar.first > 499 {
			f.bar.first = 0
		}
		if err := g.addFltscrnBigIconFields(int16(f.cols) * f.bar.first); err != nil {
			return 0, err
		}
	}
	if int16(f.nRows) <= f.bar.first {
		f.bar.first = 0
	}
	if int16(f.cols) >= f.nBig {
		return engine.NoField, nil
	}
	return g.e.AddScrollField(x, y, 0, h, 0, h, w, h, &f.bar.pos, "", 0)
}

// buildFltscrnBigIconFields is Build_Fltscrn_Big_Icon_Fields_ (0x7563f): the
// big icons of the stack that pass the filter, keeping the selection of the
// ships listed before.
func (g *Game) buildFltscrnBigIconFields(ptr int16) {
	f, s, st := &g.flt, &g.state, &g.stacks
	f.nAdded = 0
	node := st.start[ptr]
	var prev []int16
	for i := int16(0); i < f.nBig; i++ {
		if f.big[i].sel {
			prev = append(prev, f.big[i].ship)
		}
	}
	for i := range f.big {
		f.big[i] = fltBigIcon{field: engine.NoField}
	}
	f.nBig = 0
	move := int16(-1)
	for node > -1 && f.nBig < 0xfa {
		ship := st.node[node].ship
		if g.okToAddShip(int(ship)) && int8(s.Ships[ship].Status) < 3 {
			f.big[f.nBig].ship = ship
			if f.shipToMove != -1 && ship == f.shipToMove {
				move = f.nBig
			}
			f.nBig++
		}
		node = st.node[node].next
	}
	for i := int16(0); i < f.nBig; i++ {
		for _, p := range prev {
			if p == f.big[i].ship {
				f.big[i].sel = true
				break
			}
		}
	}
	if move != -1 && f.nBig > 1 {
		g.moveShipToFrontOfList(move)
	}
	f.stackOwner = -1
	if f.smallPtr > -1 {
		f.stackOwner = int16(g.shipOwner(int(st.node[st.start[f.smallPtr]].ship)))
	}
}

// moveShipToFrontOfList is Move_Ship_To_Front_Of_List_ (0x75802): swaps
// big icon i with the first.
func (g *Game) moveShipToFrontOfList(i int16) {
	f := &g.flt
	if f.nBig > 1 {
		f.big[0], f.big[i] = f.big[i], f.big[0]
	}
	f.shipToMove = -1
}

// filterButtonStatus is Get_Filter_Button_Status_ (0x75fe2): 0 all ships,
// 1 support, 2 combat, 0xffff none (the caller compares unsigned).
func (g *Game) filterButtonStatus() uint16 {
	f := &g.flt
	if g.currentScreen == 0x1d || (f.combatStatus == 1 && f.supportStatus == 1) {
		return 0
	}
	if f.combatStatus == 1 {
		return 2
	}
	if f.supportStatus == 1 {
		return 1
	}
	return 0xffff
}

// okToAddShip is Ok_To_Add_Ship_ (0x76127).
func (g *Game) okToAddShip(ship int) bool {
	t := g.state.Ships[ship].Design.Type
	switch g.filterButtonStatus() {
	case 0:
		return true
	case 1:
		return t == 2 || t == 1 || t == 4
	case 2:
		return t == 0
	}
	return false
}

// fltscrnShipsToBeDisplayed is Fltscrn_Ships_To_Be_Displayed_ (0x75ee8).
func (g *Game) fltscrnShipsToBeDisplayed(node int16) int16 {
	sh := &g.state.Ships[g.stacks.node[node].ship]
	c := g.state.nShipsByTypeAtLocation(sh.X, sh.Y, g.shipStackOwner(node))
	support := c.colony + c.outpost + c.transport
	switch g.filterButtonStatus() {
	case 0:
		return c.total
	case 1:
		return support
	case 2:
		return c.total - support
	}
	return 0
}

// addFltscrnBigIconFields is Add_Fltscrn_Big_Icon_Fields_ (0x7545c): the
// icons from first on get their places and, for the player's own stack,
// hidden fields.
func (g *Game) addFltscrnBigIconFields(first int16) error {
	f, s := &g.flt, &g.state
	w := 0x39
	if g.currentScreen == 4 {
		w = 0x3a
	}
	k := first
	for f.big[k].ship == -1 {
		k++
	}
	owner := g.shipOwner(int(f.big[k].ship))
	for f.nAdded = 0; f.nAdded < int16(f.maxIcons) && f.nAdded < f.nBig && f.nAdded+first < f.nBig; f.nAdded++ {
		i := f.nAdded + first
		x, y := g.fltscrnBigIconXY(f.nAdded)
		b := &f.big[i]
		b.x, b.y = x, y
		if owner == int(s.PlayerNum) {
			n, err := g.e.AddHiddenField(int(x), int(y), int(x)+w, int(y)+0x39, "", ')')
			if err != nil {
				return err
			}
			b.field = n
		} else {
			b.field = engine.NoField
		}
	}
	return nil
}

// fltscrnBigIconXY is Get_Fltscrn_Big_Icon_XY_ (0x76028).
func (g *Game) fltscrnBigIconXY(i int16) (x, y int16) {
	f := &g.flt
	c := int16(f.cols)
	return f.xOff + i%c*0x3e, f.yOff + i/c*0x3c
}

// nextShipIcon is Next_Ship_Icon_ (0x74502): the next stack of the same
// side (the player's or anyone else's) with ships to show.
func (g *Game) nextShipIcon() { g.stepShipIcon(1) }

// previousShipIcon is Previous_Ship_Icon_ (0x745a5).
func (g *Game) previousShipIcon() { g.stepShipIcon(-1) }

func (g *Game) stepShipIcon(d int16) {
	f, st, p := &g.flt, &g.stacks, int(g.state.PlayerNum)
	start := f.smallPtr
	owner := g.shipStackOwner(st.start[f.smallPtr])
	i := f.smallPtr
	for {
		i += d
		if i >= st.count {
			i = 0
		}
		if i < 0 {
			i = st.count - 1
		}
		o := g.shipStackOwner(st.start[i])
		found := false
		if (o == p && owner == o) || (o != p && owner != p) {
			if g.fltscrnShipsToBeDisplayed(st.start[i]) > 0 {
				found = true
				f.smallPtr = i
			}
		}
		if i == start || found {
			return
		}
	}
}

// scanSmallShipIcons is Scan_Small_Ship_Icons_ (0x749f0): 0 an icon was
// clicked (its stack becomes the shown one), 4 one is hovered (scanned).
func (g *Game) scanSmallShipIcons(scan, r int) (res, scanned int) {
	f, st := &g.flt, &g.stacks
	res, scanned = -1, -1
	for i := 0; i < int(st.iconCount); i++ {
		if r == int(st.icon[i].field) {
			g.overlappedShipIconButton(&i, 7, 4)
			res = 0
			f.smallPtr = int16(g.shipStackFromShipIcon(i))
			if f.smallPtr != f.oldSmallPtr {
				f.oldSmallPtr = -1
			}
			break
		}
	}
	if res == -1 {
		for i := 0; i < int(st.iconCount); i++ {
			if scan == int(st.icon[i].field) {
				return 4, i
			}
		}
	}
	return res, scanned
}

// scanFltscrnBigIcons is Scan_Fltscrn_Big_Icons_ (0x76f22): 0 a big icon
// was left-clicked, 1 right-clicked (clicked), 4 one is hovered (scanned).
func (g *Game) scanFltscrnBigIcons(scan, r int) (res, scanned, clicked int16) {
	f := &g.flt
	first := f.bar.first * int16(f.cols)
	end := f.nBig
	if int(first)+0x14 <= int(f.nBig) {
		end = first + 0x14
	}
	res, scanned, clicked = -1, -1, -1
	stop := first < 0 || end < 0
	for i := first; i < end && !stop; i++ {
		switch {
		case r == f.big[i].field:
			clicked, res, stop, scanned = i, 0, true, i
		case -r == f.big[i].field:
			res, clicked, stop, scanned = 1, i, true, i
		}
	}
	if res == -1 {
		for i := first; i < end && !stop; i++ {
			if scan == f.big[i].field {
				res, scanned = 4, i
			}
		}
	}
	return
}

// setFltscrnBigIcons is Set_Fltscrn_Big_Icons_ (0x74e04).
func (g *Game) setFltscrnBigIcons(on bool) {
	f := &g.flt
	for i := int16(0); i < f.nBig; i++ {
		f.big[i].sel = on
	}
}

// updateSelectionFlags is Update_Selection_Flags_ (0x74e4a): the number of
// selected icons of the player's stack; the first two ships go to
// _screen_data[3] and [4].
func (g *Game) updateSelectionFlags() int16 {
	f := &g.flt
	n := int16(0)
	if g.shipOwner(int(g.stacks.node[g.stacks.start[f.smallPtr]].ship)) != int(g.state.PlayerNum) {
		return 0
	}
	for i := int16(0); i < f.nBig; i++ {
		f.selStatus[i] = b2u8(f.big[i].sel)
		if f.big[i].sel {
			switch n {
			case 0:
				g.screenData[3] = f.big[i].ship
			case 1:
				g.screenData[4] = f.big[i].ship
			}
			n++
		}
	}
	return n
}

func b2u8(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// firstSelectedShip is First_Selected_Ship_ (0x74ed8).
func (g *Game) firstSelectedShip() int16 {
	f := &g.flt
	for i := int16(0); i < f.nBig; i++ {
		if f.big[i].sel {
			return f.big[i].ship
		}
	}
	return -1
}

// nFltscrnShipsSelected is N_Fltscrn_Ships_Selected_ (0x75398): selected
// icons from the first shown row on.
func (g *Game) nFltscrnShipsSelected() int16 {
	f := &g.flt
	n := int16(0)
	for i := f.bar.first * int16(f.cols); i < f.nBig; i++ {
		if f.big[i].sel {
			n++
		}
	}
	return n
}

// fShipIconFromNodePtr is F_Ship_Icon_From_Node_Ptr_ (0x74f51).
func (g *Game) fShipIconFromNodePtr(node int16) int {
	st := &g.stacks
	for i := 0; i < int(st.iconCount); i++ {
		if st.icon[i].node == node {
			return i
		}
	}
	return -1
}

// buildFltscrnShipList is Build_Fltscrn_Ship_List_ (0x7585d): the selected
// ships into _fltscrn_ship_icon_selection_list; false when none.
func (g *Game) buildFltscrnShipList() (ids []int16, ok bool, err error) {
	f := &g.flt
	for i := range f.shipList {
		f.shipList[i] = -1
	}
	n := 0
	for i := int16(0); i < f.nBig; i++ {
		if f.big[i].sel {
			f.shipList[n] = f.big[i].ship
			n++
			if n > 0xf9 {
				// port-only: the box redraws the fleet screen behind it,
				// whose star name builds this list again: once only.
				if !f.tooManyBox {
					f.tooManyBox = true
					_, err = g.userBox(&g.ms.drawFleetScreenToBack, cStringAt(g.exe, tooManyShipsString), boxMessage)
					f.tooManyBox = false
				}
				return f.shipList[:n], false, err
			}
		}
	}
	return f.shipList[:n], n > 0, nil
}

const tooManyShipsString = 0x1792e9 // "Too many ships in list"

// fltscrnMoveShips is Fltscrn_Move_Ships_ (0x75d48).
func (g *Game) fltscrnMoveShips(star int) error {
	f, s := &g.flt, &g.state
	ids, ok, err := g.buildFltscrnShipList()
	if err != nil || !ok {
		return err
	}
	info := g.shipsTryToMoveTo(ids, star)
	g.fb.moveInfo = info
	if info.canMove {
		g.makeShipsMoveTo(ids, star, &g.fb.moveInfo)
		g.buildShipIcons()
		g.setFltscrnSmallShipIconXYs(0xf, 0x34, 0x131, 0xb6)
		f.oldSmallPtr = -1
		g.ms.drawFleetScreenToBack = true
		g.setFltscrnBigIcons(false)
		return nil
	}
	var msg string
	switch {
	case info.blocked:
		msg = fmt.Sprintf(cFormat(cStringAt(g.exe, toStarFormat)), g.hMessage(0x21), save.CString(s.Stars[star].Name[:]))
	case info.outOfRange:
		if !s.playerHasVisited(int(s.PlayerNum), star) && !s.playerIsOmniscient(int(s.PlayerNum)) {
			msg = g.hMessage(0x91)
		} else {
			msg = fmt.Sprintf(cFormat(g.hMessage(0x90)), save.CString(s.Stars[star].Name[:]))
		}
	case info.flux:
		msg = g.hMessage(0x24)
	default:
		return nil
	}
	_, err = g.userBox(&g.ms.drawFleetScreenToBack, msg, boxWarning)
	return err
}

const toStarFormat = 0x179303 // "%s to %s"

// moveFromLimboToPool is Move_From_Limbo_To_Pool_ (0x94376): dismissed
// leaders (status 2) go back to the pool.
func (g *Game) moveFromLimboToPool() {
	s := &g.state
	for i := 0; i < maxLeaders; i++ {
		if l := &s.Leaders[i]; l.Status == 2 {
			l.Status, l.ETA = 0, 0
		}
	}
}

// clearFleetBoxSelectedShipStatus is Clear_Fleet_Box_Selected_Ship_Status_
// (0x77bf1).
func (g *Game) clearFleetBoxSelectedShipStatus() {
	g.fb.selection = [63]byte{}
}
