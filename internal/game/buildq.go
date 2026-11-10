package game

import (
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// The build queue popup (screen 0x19, Build_Queue_Popup_): MOX\OTHERS\
// COLBLDG.C. Spec: docs/screens/build-queue.md.

// ScreenBuildQueue is screen 0x19; ScreenDesign is Design_Screen_'s.
const (
	ScreenBuildQueue = 0x19
	ScreenDesign     = 3
)

// Addresses of COLBLDG.C's data.
const (
	bqHelpList    = 0x1833bc // _building_queue_help_list, 13 entries
	bqBright      = 0x1827ad // font colours of names in the queue, and text
	bqDim         = 0x1827b5 // names not in the queue, unbuildable ships
	bqMouseDesign = 0x1827a1 // pointer while a design is picked (1 entry)
	bqMouseRepeat = 0x182795 // pointer while the repeat mark is placed
	bqMouseActive = 0x182765 // pointer while a queue entry is held (4)
	bqParaHeader  = 0xaf742  // 13 bytes of paragraph tab codes
	bqSpaceStr    = 0x179d88 // " "
	bqFmtBright   = 0x179d8a // the paragraph colour codes
	bqFmtNormal   = 0x179d8d
	bqFmtTab      = 0x179d90
	bqRepeatKey   = 0x179d92 // "e"
)

// Items of the build lists and the queue besides buildings 1..0x30, ship
// designs -50..-99 and ships -100 and below.
const (
	bqColonyBase  = 0xb
	bqSeparator   = -9
	bqColonyShip  = -0xc
	bqTransport   = -0xb
	bqCustomShip  = -0xd
	bqOutpost     = -0x11
	bqDoomStar    = -0x10
	bqRefitSlot   = 5     // design slot refits and custom designs pass through
	bqCheatField  = -1000 // the cheat button, never added
	bqActiveSlot0 = 0x31  // _active_prod of queue slot 0
)

// buildQueueState is COLBLDG.C's state.
type buildQueueState struct {
	anims [7]*gfx.Picture // _anims: COLBLDG.LBX 0..6

	cur        [7]int16 // _current_item (0x1828ac), the queue being edited
	active     int16    // _active_prod (0x1828aa): held slot+0x31, or -1
	scanned    int16    // 0x19deaa: the hovered item, -1
	lastScan   int      // 0x1828be: the last Scan_Input
	mode       byte     // 0x1827bf: 1 picking a design, 2 placing a repeat
	fromDesign bool     // _return_to_build_popup_from_custom_design (0x1827bd)
	fromRefit  bool     // 0x1827be
	refitAdd   bool     // 0x199ef1
	refitShip  int16    // 0x19dea8
	auto       int16    // _colony_auto_building (0x19deac)
	savedScr   int      // _colony_bldg_saved_screen (0x19dea6)
	firstCall  bool     // _first_design_screen_call (0x199ef2)

	bldg []int16 // _building_indexes (0x1827ce)
	mil  []int16 // _military_indexes (0x18283c)

	qF    [7]int // 0x1827c0
	bldgF []int  // 0x19ddb4
	milF  []int  // 0x19de20

	fCancel, fOK, fEsc, fAuto, fRefit, fDesign, fRepeat, fCheat int
	fPrev, fNext, fBuy                                          int
	fList, fListShift                                           [11]int // port-only: build lists
}

// setBuildingQueueScreenHelpList is Set_Building_Queue_Screen_Help_List_
// (0xcdb3c).
func (g *Game) setBuildingQueueScreenHelpList() {
	g.e.DeactivateHelpList()
	g.e.SetHelpList(g.helpListAt(bqHelpList, 0xd))
}

// bqFont is Set_Font_Style_Shadow_Down_ with one of COLBLDG's colour sets.
func (g *Game) bqFont(style int, colors uint32) {
	g.e.Text().SetStyleShadow(style, g.colors(colors), font.ShadowDown)
}

// buildQueuePopup is Build_Queue_Popup_ (0xb4041), screen 0x19. The RNG is
// seeded with the colony shown before and restored on leaving.
func (g *Game) buildQueuePopup() error {
	e, s, cs, bq := g.e, &g.state, &g.cscr, &g.bq
	seed := s.Random.Get()
	s.Random.Set(uint32(int32(g.colonyHandle)))
	cs.starHandle = int(g.screenData[0])
	cs.orbitN = int(g.screenData[1])
	cs.planet = int(s.Stars[cs.starHandle].Planets[cs.orbitN])
	g.colonyHandle = int(s.Planets[cs.planet].Colony)
	cs.usingPal = true
	g.ms.galaxyMap.colonyPalette = true
	ci := g.colonyHandle
	c := &s.Colonies[ci]
	if g.settings[0xd8] != 0 {
		g.loadDoomStarShipDesign(int(c.Owner), &s.Players[c.Owner].Designs[bqRefitSlot])
	}
	if g.currentScreen != g.returnScreen && !bq.fromDesign {
		bq.savedScr = g.returnScreen
	}
	cs.sysStar = cs.starHandle
	g.bqFont(4, bqBright)
	e.Text().SetSpacing(1)
	if err := g.loadColonyScreenSeg(); err != nil {
		return err
	}
	g.setBuildingQueueScreenHelpList()
	if !bq.fromDesign {
		bq.cur = c.Queue
		bq.active = -1
	}
	g.calculateBuildingArray()
	g.calculateMilitaryArray()
	if !bq.fromDesign {
		bq.auto = int16(c.AutoBuild)
	}
	if err := g.makeBldgArrayForColony(ci, false); err != nil {
		return err
	}
	if g.fastFadeOutNeededForColonyPalette() {
		if err := e.FastFadeOut(); err != nil {
			return err
		}
		if err := g.updateColonyPalette(); err != nil {
			return err
		}
	}
	e.ClearFields()
	if err := g.bqAddFields(); err != nil {
		return err
	}
	g.col.fullDraw = true
	e.AssignAutoFunction(g.drawBuildQueuePopup, 1)
	g.setBuildingQueueScreenHelpList()
	e.V.ClearPaletteChanges(0, 0xff)
	if err := g.drawBuildQueuePopup(); err != nil {
		return err
	}
	if err := e.FastFadeIn(); err != nil {
		return err
	}
	e.V.SetRefreshStencil()
	var r int
	for {
		if err := g.drawBuildQueuePopup(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		var err error
		if r, err = e.GetInput(); err != nil {
			return err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return err
		}
		if err := g.bqEvaluateInput(&r, scan); err != nil {
			return err
		}
		if r == bq.fOK || r == bq.fCancel || r == bq.fEsc || bq.fromDesign || r == bq.fNext || r == bq.fPrev {
			break
		}
	}
	e.ClearFields()
	if bq.active != -1 {
		g.resetMousePicture()
	}
	e.Text().SetSpacing(3)
	e.DeactivateHelpList()
	e.DeactivateAutoFunction()
	e.V.CopyBackToOff()
	if r != bq.fNext && r != bq.fPrev {
		switch {
		case bq.fromDesign:
			bq.firstCall = true
			g.returnScreen, g.currentScreen = g.currentScreen, ScreenDesign
		case g.currentScreen != g.returnScreen:
			g.currentScreen = g.returnScreen
		default:
			g.currentScreen, g.returnScreen = bq.savedScr, bq.savedScr
		}
	}
	s.Random.Set(seed)
	return nil
}

// updateColonyPalette is Update_Colony_Palette_ (0xbf55a).
func (g *Game) updateColonyPalette() error {
	e := g.e
	if err := e.LoadPalette(1, 0, 0xff); err != nil {
		return err
	}
	bg, err := g.cAnimsColony(0)
	if err != nil {
		return err
	}
	e.DrawPalette(bg)
	e.UpdateGlassRemapColors()
	return nil
}

// bqAddFields is Add_Fields_ (0xb3e7a): the buttons and hot keys, then the
// queue, building and military rows.
func (g *Game) bqAddFields() error {
	e, bq := g.e, &g.bq
	none := g.eStr(0xc)
	var err error
	if bq.fCancel, err = e.AddButtonField(0x1ed, 0x1bf, none, bq.anims[1], none, 0x28); err != nil {
		return err
	}
	if bq.fOK, err = e.AddButtonField(0x230, 0x1bf, none, bq.anims[3], none, 0x28); err != nil {
		return err
	}
	for _, k := range []struct {
		f *int
		n int
	}{{&bq.fEsc, 0x87}, {&bq.fPrev, 0x8b}, {&bq.fNext, 0x8c}, {&bq.fBuy, 0xc1}} {
		if *k.f, err = e.AddHotKey(g.eStr(k.n)); err != nil {
			return err
		}
	}
	if bq.fAuto, err = e.AddRadioButtonField(0x1ea, 0x156, none, bq.anims[2], &bq.auto, none, 0x29); err != nil {
		return err
	}
	if g.settings[0xd8] != 0 {
		bq.fRefit, bq.fDesign = bqCheatField, bqCheatField
	} else {
		if bq.fRefit, err = e.AddButtonField(0x1ec, 0x17b, none, bq.anims[4], g.eStr(0x1b4), 0x28); err != nil {
			return err
		}
		if bq.fDesign, err = e.AddButtonField(0x231, 0x17b, none, bq.anims[5], g.eStr(0xfb), 0x28); err != nil {
			return err
		}
	}
	if bq.fRepeat, err = e.AddButtonField(0x1f7, 0x19b, none, bq.anims[6], cStringAt(g.exe, bqRepeatKey), 0x28); err != nil {
		return err
	}
	bq.fCheat = bqCheatField
	if err := g.addBuildListKeys(&bq.fList, &bq.fListShift); err != nil {
		return err
	}
	if err := g.addBuildQueueFields(); err != nil {
		return err
	}
	if err := g.addBuildingsFields(); err != nil {
		return err
	}
	return g.addMilitarysFields()
}

// addBuildQueueFields is Add_Build_Queue_Fields_ (0xb325a).
func (g *Game) addBuildQueueFields() error {
	bq := &g.bq
	for i := 0; i < 7; i++ {
		f, err := g.e.AddHiddenField(0xcf, 0x149+0x14*i, 0x1ca, 0x15e+0x14*i, g.eStr(0xc), 0x29)
		if err != nil {
			return err
		}
		bq.qF[i] = f
	}
	return nil
}

// bqListFields is Add_Buildings_Fields_ and Add_Militarys_Fields_: rows of
// a squeezed list between y 0x14 and y2.
func (g *Game) bqListFields(x1, x2, y2, kind int, n int, out *[]int) error {
	g.calculateSquishStep(0xd, int16(y2), int16(n), 0x13)
	*out = (*out)[:0]
	h := int(int16(0x13 - g.col.stepSquish))
	for i := 0; i < n; i++ {
		f, err := g.e.AddHiddenField(x1, int(int16(h*i+0x14)), x2, int(int16(h*(i+1)+0x13)), g.eStr(0xc), 0x29)
		if err != nil {
			return err
		}
		*out = append(*out, f)
	}
	return nil
}

// addBuildingsFields is Add_Buildings_Fields_ (0xb08ca).
func (g *Game) addBuildingsFields() error {
	return g.bqListFields(0xd, 0xb8, 0x1d5, 1, len(g.bq.bldg), &g.bq.bldgF)
}

// addMilitarysFields is Add_Militarys_Fields_ (0xb094c).
func (g *Game) addMilitarysFields() error {
	return g.bqListFields(0x1e5, 0x26f, 0x134, 2, len(g.bq.mil), &g.bq.milF)
}

// bqRestore is the redraw COLBLDG does after a box: Deactivate_Auto_
// Function_, the draw as auto function, Update_Fields_, _full_draw.
func (g *Game) bqRestore() error {
	e := g.e
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawBuildQueuePopup, 1)
	e.ClearFields()
	if err := g.bqAddFields(); err != nil {
		return err
	}
	g.col.fullDraw = true
	return nil
}

// bqHelp is Help_ (a message box) with E string n, then the redraw.
func (g *Game) bqHelp(n int) error {
	if err := g.messageBox(g.eStr(n)); err != nil {
		return err
	}
	return g.bqRestore()
}

// bqSetMouseList is Set_Mouse_List_ with a list from the image.
func (g *Game) bqSetMouseList(addr uint32, n int) {
	g.e.SetMouseList(g.exeMouseList(addr, n))
}

// bqIsSetting reports trade goods, housing and the repeat mark, the items
// that never finish.
func bqIsSetting(v int16) bool { return v == -3 || v == -2 || v == -10 }

// bqIsDesign reports a ship design item (-50..-99).
func bqIsDesign(v int16) bool { return v > -100 && v <= -50 }

// bqEvaluateInput is Evaluate_Input_ (0xb2542). Most paths end with
// Colony_Calculation_; the early returns skip it.
func (g *Game) bqEvaluateInput(r *int, scan int) error {
	e, s, bq := g.e, &g.state, &g.bq
	ci := g.colonyHandle
	c := &s.Colonies[ci]
	bq.scanned = -1
	bq.lastScan = scan
	if bq.fromDesign || bq.fromRefit {
		bq.fromDesign, bq.fromRefit, bq.mode = false, false, 0
		g.resetMousePicture()
		if g.screenData[4] == 0 {
			bq.refitAdd = false
		} else if bq.refitAdd {
			if err := g.addRefitedShipToQueue(); err != nil {
				return err
			}
			bq.refitAdd = false
		}
		// Community patch 1.51: back from the design screen without a
		// refit, the sixth design returns.
		g.restoreSpare(int(c.Owner))
	}
	switch r0 := *r; {
	case r0 == bq.fOK:
		bq.mode = 0
		g.resetMousePicture()
		c.AutoBuild = uint8(bq.auto)
		for g.somethingBehindSetting() {
			var item int16
			for i := 0; i < 7; i++ {
				if bqIsSetting(bq.cur[i]) && g.somethingBehindSetting() {
					item = bq.cur[i]
					break
				}
			}
			name := g.selectionName(item)
			msg := cSprintf(g.eStr(0x3b), name, name)
			if g.settings[0xa] == 0 {
				ok, err := g.confirmationBox(msg)
				if err != nil {
					return err
				}
				if !ok {
					if err := g.bqRestore(); err != nil {
						return err
					}
					break
				}
			}
			for i := 0; i < 7; i++ {
				if bqIsSetting(bq.cur[i]) && g.somethingBehindSetting() {
					g.squishOutQueueSlot(i)
				}
			}
			if err := g.bqRestore(); err != nil {
				return err
			}
		}
		g.doExitScreenCleanup()
	case r0 == bq.fCancel || r0 == bq.fEsc:
		if bq.mode != 0 || bq.active != -1 {
			bq.mode = 0
			g.resetMousePicture()
			bq.active = -1
			*r = 0
			g.resetMousePicture()
			break
		}
		for i := 0; i < 7; i++ {
			v := bq.cur[i]
			if v > -100 || colonyQueueListHas(c, v, 0xe) || s.Ships[-(int(v)+100)].Status != 4 {
				continue
			}
			ok, err := g.confirmationBox(g.eStr(0x1dc))
			if err != nil {
				return err
			}
			if !ok {
				*r = 0
				return nil
			}
			if err := g.bqRestore(); err != nil {
				return err
			}
			break
		}
		g.removeShips(false)
	case r0 == bq.fDesign:
		g.bqSetMouseList(bqMouseDesign, 1)
		bq.mode = 1
	case r0 == bq.fRepeat:
		bq.mode = 2
		g.bqSetMouseList(bqMouseRepeat, 1)
	case r0 == bq.fRefit:
		if err := g.bqRefit(); err != nil {
			return err
		}
		g.col.fullDraw = true
	case r0 == bq.fAuto:
	case g.isBuildListField(r0, &bq.fList, &bq.fListShift):
		k, _ := g.buildListKey(r0, &bq.fList, &bq.fListShift)
		ops, err := g.loadBuildList(k)
		if err != nil {
			return err
		}
		bq.mode, bq.active = 0, -1
		g.resetMousePicture()
		g.applyBuildList(ops)
		if err := g.bqRestore(); err != nil {
			return err
		}
	case r0 == bq.fBuy:
		g.doExitScreenCleanup()
		if err := g.testedColonyBuysOutright(ci); err != nil {
			return err
		}
		if err := g.bqRestore(); err != nil {
			return err
		}
		g.clearMouseBuffer()
		e.SetInputDelay(3)
	case r0 == bq.fPrev:
		n := g.getNextColony(ci, 0)
		bq.mode = 0
		g.resetMousePicture()
		c.AutoBuild = uint8(bq.auto)
		g.doExitScreenCleanup()
		if n != ci {
			g.setColonyScreenToNewColony(n)
		}
		g.col.fullDraw = true
	case r0 == bq.fNext:
		if n := g.getNextColony(ci, 1); n != ci {
			g.setColonyScreenToNewColony(n)
		}
		bq.mode = 0
		g.resetMousePicture()
		c.AutoBuild = uint8(bq.auto)
		g.doExitScreenCleanup()
		g.col.fullDraw = true
	}
	if bq.mode != 1 {
		for i, f := range bq.bldgF {
			if i >= len(bq.bldg) {
				break
			}
			if done, err := g.bqBuildingRow(r, scan, i, f); err != nil || done {
				return err
			}
		}
	}
	for i := 0; i < len(bq.milF) && i < len(bq.mil); i++ {
		if done, err := g.bqMilitaryRow(r, scan, i); err != nil || done {
			return err
		}
	}
	for i := 0; i < 7; i++ {
		if done, err := g.bqQueueRow(r, scan, i); err != nil || done {
			return err
		}
	}
	if g.fullDrawScreen {
		if err := g.bqRestore(); err != nil {
			return err
		}
		g.fullDrawScreen = false
	}
	g.colonyCalculation(ci)
	return nil
}

// bqBoughtError is the bought-product refusal (E 0x23a): the held entry is
// dropped and Evaluate_Input_ returns at once.
func (g *Game) bqBoughtError() error {
	if err := g.bqHelp(0x23a); err != nil {
		return err
	}
	g.bq.active = -1
	g.resetMousePicture()
	return nil
}

// bqBuildingRow is Evaluate_Input_'s building row i: add the building, or
// take it out of the queue again.
func (g *Game) bqBuildingRow(r *int, scan, i, f int) (bool, error) {
	bq := &g.bq
	c := &g.state.Colonies[g.colonyHandle]
	item := bq.bldg[i]
	if scan == f {
		bq.scanned = item
	}
	if *r != f {
		return false, nil
	}
	if !g.isInQueue(item) {
		ok, err := g.addToQueue(item)
		if err != nil || ok {
			return false, err
		}
		return false, g.bqHelp(0x188)
	}
	slot := g.firstProdQueueSlot(item)
	if c.Pending != 0 && slot == 0 {
		return true, g.bqBoughtError()
	}
	if err := g.deleteFromQueue(int(slot)); err != nil {
		return false, err
	}
	if bq.active >= bqActiveSlot0 && bq.active-bqActiveSlot0 == slot {
		bq.active = -1
	}
	return false, nil
}

// bqMilitaryRow is Evaluate_Input_'s military row i (separators are inert):
// queue it, or in design-picking mode open the design screen on it.
func (g *Game) bqMilitaryRow(r *int, scan, i int) (bool, error) {
	s, bq := &g.state, &g.bq
	f := bq.milF[i]
	if scan == f && bq.mil[i] != bqSeparator {
		bq.scanned = bq.mil[i]
	}
	if *r != f || bq.mil[i] == bqSeparator {
		return false, nil
	}
	if bq.mil[i] == bqColonyBase {
		g.calculateMilitaryArray()
		g.col.fullDraw = true
	}
	if i >= len(bq.mil) {
		return false, nil
	}
	item := bq.mil[i]
	if item == bqColonyBase {
		star := int(s.Planets[s.Colonies[g.colonyHandle].Planet].Star)
		if g.starNUncolonizedWorlds(star) <= g.nProdsInQueue(bqColonyBase) {
			return false, g.bqHelp(0x232)
		}
	}
	if bq.mode != 1 {
		ok, err := g.addToQueue(item)
		if err != nil || ok {
			return false, err
		}
		return false, g.bqHelp(0x188)
	}
	if bqIsDesign(item) {
		bq.fromDesign = true
		g.screenData[4] = -(item + 50)
		g.resetMousePicture()
		return true, nil
	}
	return false, nil
}

// bqQueueRow is Evaluate_Input_'s queue row i: pick an entry up, or put the
// held one before it (on itself: delete it). A bought product stays (the
// 1.40 patch at 0x169020 also guards a repeat mark in slot 1).
func (g *Game) bqQueueRow(r *int, scan, i int) (bool, error) {
	bq := &g.bq
	c := &g.state.Colonies[g.colonyHandle]
	f := bq.qF[i]
	if scan == f {
		bq.scanned = bq.cur[i]
	}
	if *r != f {
		return false, nil
	}
	if bq.active != -1 {
		a := int(int16(bq.active - bqActiveSlot0))
		if c.Pending != 0 && (i == 0 || a == 0) {
			return true, g.bqBoughtError()
		}
		move := a != i
		if move {
			if a >= 0 && a < 7 && bq.cur[a] == -10 && a != 0 && a-1 == i {
				move = false
			} else if a+1 >= 0 && a+1 < 7 && bq.cur[a+1] == -10 && a+1 == i {
				move = false
			}
		}
		if move {
			g.insertBeforeQueueSlot(a, i)
			g.defragmentQueue()
		} else {
			if err := g.deleteFromQueue(i); err != nil {
				return false, err
			}
			bq.active = -1
		}
		g.resetMousePicture()
	} else {
		if c.Pending != 0 && (i == 0 || (i == 1 && bq.cur[1] == -10)) {
			if err := g.bqHelp(0x23a); err != nil {
				return false, err
			}
			return true, nil
		}
		bq.active = int16(i) + bqActiveSlot0
	}
	if bq.mode != 0 {
		bq.mode = 0
		g.resetMousePicture()
		bq.active = -1
		*r = 0
		g.resetMousePicture()
	}
	return false, nil
}

// bqRefit is Evaluate_Input_'s refit button: with the refit techs (fields
// 0x16, 0x17) the two refit popups pick a ship and its new design; -3 adds
// it at once, -2 sends the design screen.
func (g *Game) bqRefit() error {
	s, bq := &g.state, &g.bq
	c := &s.Colonies[g.colonyHandle]
	pl := &s.Players[c.Owner]
	if pl.FieldStatus[0x16] != 3 || pl.FieldStatus[0x17] != 3 {
		if err := g.messageBox(g.eStr(0x244)); err != nil {
			return err
		}
		return g.bqRestore()
	}
	bq.mode = 0
	g.resetMousePicture()
	k, err := g.colonyRefitPopup()
	if err != nil {
		return err
	}
	if k != -1 {
		if err := g.bqRestore(); err != nil {
			return err
		}
		res, err := g.colonyRefit2Popup(k)
		if err != nil {
			return err
		}
		switch res {
		case -3:
			bq.refitAdd, bq.fromRefit = true, true
			g.screenData[4] = 1
			bq.refitShip = k
		case -2:
			bq.refitShip = k
			bq.refitAdd = true
			g.keepSpare(&pl.Designs[bqRefitSlot]) // community patch 1.51
			pl.Designs[bqRefitSlot] = s.Ships[k].Design
			bq.fromDesign = true
			g.screenData[4] = bqRefitSlot
		}
	}
	e := g.e
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawBuildQueuePopup, 1)
	e.ClearFields()
	return g.bqAddFields()
}
