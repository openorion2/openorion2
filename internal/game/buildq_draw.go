package game

import (
	"strings"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/save"
)

// The build queue popup's drawing: the gray colony behind the panel, the
// two lists, the queue and the selection's picture, costs and description.

// Name tables Load_Tech_Names_ fills after the weapons.
const (
	walksComputers = walksWeapons + 46 // 0x17fdec
	walksDrives    = walksComputers + 6
)

// drawBuildQueuePopup is Draw_Build_Queue_Popup_ (0xb3cf7).
func (g *Game) drawBuildQueuePopup() error {
	e, s, bq := g.e, &g.state, &g.bq
	t := e.Text()
	if g.col.fullDraw {
		e.V.Fill(0, 0, 0x280, 0x1df, 0)
		bg, err := g.cAnimsColony(0)
		if err != nil {
			return err
		}
		if err := e.Draw(0, 0, bg); err != nil {
			return err
		}
		if err := g.drawRoadList(); err != nil {
			return err
		}
		if err := g.drawColonyBldgs(); err != nil {
			return err
		}
		e.InterlacedGrayScaleOffPage(0xf6, 9, 0xf4, 10, 0)
		e.DrawPalette(bq.anims[0])
		e.V.SetPaletteChanges(0, 0xff)
		e.UpdateGlassRemapColors()
		g.darkenFill(0xca, 0x130, 0x1cf, 0x1d6)
		if err := e.Draw(0, 0, bq.anims[0]); err != nil {
			return err
		}
		title := cSprintf(g.eStr(0xd2), g.planetName(int(s.Colonies[g.colonyHandle].Planet)))
		g.bqFont(3, bqBright)
		if err := t.SqueezePrintParagraph(0xeb, 0x135+0x11/2, 0xc2, 0x11, title, 2, g.exe.bytes(spaceLimits, 6), true); err != nil {
			return err
		}
		if g.previousScreen == ScreenColonySummary || bq.savedScr == ScreenColonySummary {
			e.DrawPalette(bq.anims[0])
			e.UpdateGlassRemapColors()
			e.DrawPalette(g.cs.palAnim)
		} else {
			e.UpdateGlassRemapColors()
		}
		e.V.CopyOffToBack()
		g.col.fullDraw = false
	}
	g.bqFont(4, bqBright)
	t.SetSpacing(1)
	if err := g.drawBuildingsList(); err != nil {
		return err
	}
	if err := g.drawMilitarysList(); err != nil {
		return err
	}
	if err := g.drawBuildQueue(); err != nil {
		return err
	}
	if err := g.drawCurrentSelection(); err != nil {
		return err
	}
	g.drawActiveProd()
	return nil
}

// calcFontOff is Calc_Font_Off_ (0xb036d): smaller fonts for squeezed
// lists.
func (g *Game) calcFontOff() int {
	switch st := g.col.stepSquish; {
	case st == 0:
		return 0
	case st < 5:
		return 2
	}
	return 3
}

// getFontOff is Get_Font_Off_For_Buildings_ (0xb0397) and _For_Military_
// (0xb0499): smaller styles while a name is wider than maxw, down to style
// 2 below the start.
func (g *Game) getFontOff(list []int16, maxw int, colorOf func(int16) uint32, off *int) error {
	t := g.e.Text()
	for i := 0; i < len(list); {
		g.bqFont(4-*off, colorOf(list[i]))
		w, err := t.ParagraphMaxWidth(0x280, g.selectionName(list[i]), 0)
		if err != nil {
			return err
		}
		if int16(w) > int16(maxw) {
			*off++
		}
		if *off > 2 {
			w = 0
		}
		if int16(w) <= int16(maxw) {
			i++
		}
	}
	return nil
}

// bqQueueColor is the buildings list's colour: bright when queued.
func (g *Game) bqQueueColor(v int16) uint32 {
	if g.isInQueue(v) {
		return bqBright
	}
	return bqDim
}

// drawBuildingsList is Draw_Buildings_List_ (0xb04fd).
func (g *Game) drawBuildingsList() error {
	bq := &g.bq
	g.calculateSquishStep(0xd, 0x1d5, int16(len(bq.bldg)), 0x13)
	off := g.calcFontOff()
	if err := g.getFontOff(bq.bldg, 0xab, g.bqQueueColor, &off); err != nil {
		return err
	}
	g.bqFont(4-off, bqBright)
	h := int(int16(0x13 - g.col.stepSquish))
	for i, v := range bq.bldg {
		g.bqFont(4-off, g.bqQueueColor(v))
		if err := g.squeezePrint(0xd, int(int16(h*i+0x14)), g.selectionName(v), 0xab); err != nil {
			return err
		}
	}
	return nil
}

// drawMilitarysList is Draw_Militarys_List_ (0xb0663): ships the colony
// cannot build are dim; while a design is picked the hovered one blinks.
func (g *Game) drawMilitarysList() error {
	s, bq := &g.state, &g.bq
	ci := g.colonyHandle
	c := &s.Colonies[ci]
	g.calculateSquishStep(0xd, 0x134, int16(len(bq.mil)), 0x13)
	off := g.calcFontOff()
	if err := g.getFontOff(bq.mil, 0x8a, func(int16) uint32 { return bqBright }, &off); err != nil {
		return err
	}
	g.bqFont(4-off, bqBright)
	h := int(int16(0x13 - g.col.stepSquish))
	noYard := c.Buildings[0x28] == 0 && c.Buildings[0x29] == 0 && c.Buildings[8] == 0
	for i, v := range bq.mil {
		dim := false
		switch {
		case bqIsDesign(v) && noYard && s.Players[s.PlayerNum].Designs[-(v+50)].Size >= 2 && bq.mode != 1:
			dim = true
		case bq.mode == 1 && bq.scanned == v && bqIsDesign(v) && g.globalCycler(0x1e, 2) != 0:
			dim = true
		case bqIsDesign(v) && g.colonyCanBuildProduct(ci, v) == 0 && bq.mode != 1:
			dim = true
		case v == bqDoomStar && g.colonyCanBuildProduct(ci, -(bqRefitSlot+50)) == 0:
			dim = true
		}
		col := uint32(bqBright)
		if dim {
			col = bqDim
		}
		g.bqFont(4-off, col)
		if err := g.squeezePrint(0x1e5, int(int16(h*i+0x14)), g.selectionName(v), 0x8a); err != nil {
			return err
		}
	}
	return nil
}

// bqHeld reports whether the held queue entry a (with its repeat mark) is
// hovered, as Draw_Active_Prod_ and Draw_Build_Queue_ test it.
func (g *Game) bqHoverInQueue() bool {
	bq := &g.bq
	return bq.lastScan >= bq.qF[0] && bq.lastScan <= bq.qF[6]
}

// drawBuildQueue is Draw_Build_Queue_ (0xb306f): the seven entries, "delete
// ..." on the held one (and its repeat mark) while it is hovered.
func (g *Game) drawBuildQueue() error {
	bq := &g.bq
	q := &bq.cur
	at := func(i int) int16 {
		if i >= 0 && i < 7 {
			return q[i]
		}
		if i == -1 {
			return bq.active
		}
		return -1
	}
	a := -1
	if bq.active >= bqActiveSlot0 {
		a = int(int16(bq.active - bqActiveSlot0))
	}
	sc := bq.scanned
	for i := 0; i < 7; i++ {
		y := 0x14f + 0x14*i
		hl := false
		switch {
		case i == a && sc == at(a):
			hl = true
		case i == a && a < 6 && sc == at(a+1) && at(a+1) == -10:
			hl = true
		case i == a && i != 0 && sc == at(a-1) && q[i] == -10:
			hl = true
		case i != 0 && q[i] == -10 && (sc == q[i] || sc == q[i-1]) && (a == i-1 || i == a):
			hl = true
		case i < 6 && q[i+1] == -10 && (sc == q[i] || sc == q[i+1]) && (a == i+1 || i == a):
			hl = true
		}
		g.bqFont(3, bqBright)
		str := g.selectionName(q[i])
		if hl && g.bqHoverInQueue() && q[i] != -1 {
			str = cSprintf(g.eStr(0x101), str)
		}
		if err := g.squeezePrintCentered(0x14c, y, str, 0xfb); err != nil {
			return err
		}
	}
	return nil
}

// drawActiveProd is Draw_Active_Prod_ (0xb1458): a box round the held
// entry (and its repeat mark), bright while it is hovered.
func (g *Game) drawActiveProd() {
	e, bq := g.e, &g.bq
	if bq.active == -1 {
		return
	}
	if bq.active < bqActiveSlot0 {
		h := int(int16(0x13 - g.col.stepSquish))
		for i, v := range bq.bldg {
			if v == bq.active {
				e.Box(8, int(int16(h*i+0x12)), 0xab, int(int16(h-1)), 1, 0x64)
			}
		}
		return
	}
	a := int(int16(bq.active - bqActiveSlot0))
	q := &bq.cur
	sc := bq.scanned
	hover := sc == q[a] ||
		(a != 0 && q[a] == -10 && sc == q[a-1]) ||
		(a < 6 && q[a+1] == -10 && sc == q[a+1])
	col := byte(0x64)
	if hover && g.bqHoverInQueue() && q[a] != -1 {
		col = 0xef
	}
	box := func(i int) { e.Box(0xcf, 0x14c+0x14*i, 0xfc, 0x10, 1, col) }
	box(a)
	if a != 0 && q[a] == -10 {
		box(a - 1)
	}
	if a < 6 && q[a+1] == -10 {
		box(a + 1)
	}
	if col == 0xef {
		g.resetMousePicture()
	} else {
		g.bqSetMouseList(bqMouseActive, 4)
	}
}

// squeezePrintCentered is Squeeze_Print_Centered_ (0x102f4f).
func (g *Game) squeezePrintCentered(x, y int, s string, maxw int) error {
	return g.squeezePrintTyp(int(int16(x-maxw/2)), y, s, maxw, engine.ParaCentered)
}

// bqLine is the info column's line y.
func bqLine(n int) int { return int(int16(n*0xd + 0x15)) }

// drawCurrentSelection is Draw_Current_Selection_ (0xb3328): the hovered
// item, or the queue's first: name, picture, cost and time, description.
func (g *Game) drawCurrentSelection() error {
	e, s, bq := g.e, &g.state, &g.bq
	t := e.Text()
	ci := g.colonyHandle
	sel := bq.cur[0]
	if bq.scanned != -1 {
		sel = bq.scanned
	}
	line := 0
	race := int(s.Players[s.PlayerNum].Race)
	name := func(squeeze bool) error {
		g.bqFont(4, bqBright)
		n := g.selectionName(sel)
		y := bqLine(line)
		line++
		if squeeze {
			return g.squeezePrint(0x132, y, n, 0x9c)
		}
		return t.Print(0x132, y, n)
	}
	costs := func() error { return g.drawCostAndTimeInfo(ci, sel, &line) }
	app := func(n int) error { return g.printAppDescription(0xd1, 0x77, 0xf8, n) }
	android := func(n int) error {
		if err := name(true); err != nil {
			return err
		}
		line++
		g.bqFont(3, bqBright)
		if err := costs(); err != nil {
			return err
		}
		p, err := g.peopleAnim(0, 4, race)
		if err != nil {
			return err
		}
		if err := g.drawCentered(0xf4, 0x38, p); err != nil {
			return err
		}
		return app(n)
	}
	ship := func(squeeze bool, seg int16, n int) error {
		if err := name(squeeze); err != nil {
			return err
		}
		line++
		g.bqFont(3, bqBright)
		if err := costs(); err != nil {
			return err
		}
		if err := app(n); err != nil {
			return err
		}
		return g.remapDrawShipSegNCentered(0xf4, 0x38, seg)
	}
	special := func() string {
		return cSprintf(g.eStr(0x54), g.eStr(0xf3), g.eStr(0x1ea))
	}
	switch {
	case sel == -2:
		if err := name(true); err != nil {
			return err
		}
		line++
		p, err := g.prodAnims(3, 1, false)
		if err != nil {
			return err
		}
		if err := e.Draw(0xeb, 0x36, p); err != nil {
			return err
		}
		if err := costs(); err != nil {
			return err
		}
		return g.printSettingDescription(0xd1, 0x77, 0xf8, 0xc)
	case sel == -3:
		if err := name(true); err != nil {
			return err
		}
		line++
		if err := costs(); err != nil {
			return err
		}
		if err := g.drawBuildingCenteredInWindow(0xe); err != nil {
			return err
		}
		return g.printSettingDescription(0xd1, 0x77, 0xf8, 0xd)
	case sel == -10:
		if err := name(true); err != nil {
			return err
		}
		g.bqFont(3, bqBright)
		line++
		if err := t.Print(0x132, bqLine(line), special()); err != nil {
			return err
		}
		line++
		return t.PrintCentered(0xf4, 0x38, g.selectionName(sel))
	case sel == -4:
		return android(6)
	case sel == -5:
		return android(8)
	case sel == -6:
		return android(7)
	case sel == -7:
		if err := name(false); err != nil {
			return err
		}
		line++
		if err := costs(); err != nil {
			return err
		}
		if err := app(0xa6); err != nil {
			return err
		}
		p, err := g.cached(cStringAt(g.exe, raceiconLBX), race*0xd+0xb) // Spy_Anim_ (0xbb382)
		if err != nil {
			return err
		}
		return g.drawCentered(0xf4, 0x38, p)
	case sel == -8:
		g.bqFont(4, bqBright)
		if err := t.Print(0x132, bqLine(line), g.eStr(0x1bd)); err != nil {
			return err
		}
		line++
		g.bqFont(3, bqBright)
		line++
		if err := t.Print(0x132, bqLine(line), g.selectionName(sel)); err != nil {
			return err
		}
		line++
		if err := t.Print(0x132, bqLine(line), special()); err != nil {
			return err
		}
		line++
		g.bqFont(5, bqBright)
		if err := t.PrintCentered(0xf4, 0x38, g.eStr(0x7d)); err != nil {
			return err
		}
		g.bqFont(4, bqBright)
		return nil
	case sel == bqCustomShip:
		g.bqFont(3, bqBright)
		return g.drawShipDesignInfo(-(4 + 50))
	case sel == bqDoomStar:
		if err := name(true); err != nil {
			return err
		}
		g.bqFont(3, bqBright)
		return g.drawShipDesignInfo(-(bqRefitSlot + 50))
	case sel == bqOutpost:
		return ship(true, 0x2e, 0x6d)
	case sel == bqColonyShip:
		return ship(true, 0x2d, 0x29)
	case sel == -0xf:
		return ship(false, 0x30, 0x45)
	case sel == bqTransport:
		if err := name(false); err != nil {
			return err
		}
		line++
		g.bqFont(3, bqBright)
		if err := costs(); err != nil {
			return err
		}
		if err := g.remapDrawShipSegNCentered(0xf4, 0x38, 0x2f); err != nil {
			return err
		}
		return app(0xbd)
	case sel == -1 || sel == 0:
		return nil
	case sel > 0 && sel < 0x31:
		return g.drawBuildingInfo(sel)
	case sel <= -100:
		return g.drawShipInfo(sel)
	case bqIsDesign(sel):
		return g.drawShipDesignInfo(sel)
	}
	// The original prints a debug line and waits for a key (PFMT_, hgetch_).
	g.logOnce("Draw_Current_Selection_: no case for item " + itoa(int(sel)))
	return nil
}

// drawCostAndTimeInfo is Draw_Cost_And_Time_Info_ (0xb3b56): cost,
// maintenance for buildings, build time, then the turns left when the item
// is the colony's product, or the gravity for building 0x19.
func (g *Game) drawCostAndTimeInfo(ci int, item int16, line *int) error {
	s := &g.state
	t := g.e.Text()
	c := &s.Colonies[ci]
	cost := int16(g.colonyProductCost(c, item))
	if err := t.Print(0x132, bqLine(*line), cSprintf(g.eStr(0xf2), int(cost))); err != nil {
		return err
	}
	*line++
	if item > 0 && item < 0x31 {
		m := g.exeI16(rowAddr(bldgMaintTable, 0x13, int(item), 0))
		if err := t.Print(0x132, bqLine(*line), cSprintf(g.eStr(0x16f), int(m))); err != nil {
			return err
		}
		*line++
	}
	if err := g.squeezePrint(0x132, bqLine(*line), cSprintf(g.eStr(0xd3), int(g.calculateColonyTurnCountFromScrapForProd(ci, item))), 0x9c); err != nil {
		return err
	}
	*line++
	var msg string
	switch {
	case item == c.Queue[0]:
		msg = cSprintf(g.eStr(0x220), int(g.calculateCurrentProductionTurnCount(ci)))
	case item == 0x19:
		msg = cSprintf(g.eStr(0x13a), g.eStr(planetGravityEStr[s.Planets[c.Planet].Gravity]))
	default:
		return nil
	}
	if err := g.squeezePrint(0x132, bqLine(*line), msg, 0x9c); err != nil {
		return err
	}
	*line++
	return nil
}

// calculateColonyTurnCountFromScrapForProd is
// Calculate_Colony_Turn_Count_From_Scrap_For_Prod_ (0xbdbb9): turns to
// build item from nothing at the colony's industry.
func (g *Game) calculateColonyTurnCountFromScrapForProd(ci int, item int16) int16 {
	c := &g.state.Colonies[ci]
	cost := int16(g.colonyProductCost(c, item))
	ind := c.Output[1]
	switch {
	case ind >= cost:
		return 1
	case ind <= 0:
		return 10000
	}
	return int16(int32(cost)/int32(max(ind, 1)) + 1)
}

// drawBuildingCenteredInWindow is Draw_Building_Centered_In_Window_
// (0xb0a9c).
func (g *Game) drawBuildingCenteredInWindow(id int32) error {
	v := g.e.V
	v.SetWindow(0xcb, 9, 0x11d, 0x67)
	v.ClipOn()
	err := g.drawBuildingWithBottomCentered(0xf4, 0x5d, id, 5, 5)
	v.ClipOff()
	return err
}

// drawBuildingInfo is Draw_Building_Info_ (0xb0ae3).
func (g *Game) drawBuildingInfo(id int16) error {
	if g.buildingType(int32(id)) != 7 {
		if err := g.drawBuildingCenteredInWindow(int32(id)); err != nil {
			return err
		}
	} else {
		p, err := g.satelliteAnims(satelliteAnimIndex(uint16(id))) // Satellite_Anim_ (0xbe306)
		if err != nil {
			return err
		}
		if err := g.drawCentered(0xf4, 0x38, p); err != nil {
			return err
		}
	}
	g.bqFont(4, bqBright)
	name, err := g.buildingName(int(id))
	if err != nil {
		return err
	}
	if err := g.squeezePrint(0x132, bqLine(0), name, 0x9c); err != nil {
		return err
	}
	g.bqFont(3, bqBright)
	line := 2
	if err := g.drawCostAndTimeInfo(g.colonyHandle, id, &line); err != nil {
		return err
	}
	return g.printAppDescription(0xd1, 0x77, 0xf8, int(g.exeI16(rowAddr(bldgAppTable, 0x13, int(id), 0))))
}

// bqDescFont is the description paragraphs' font.
func (g *Game) bqDescFont() {
	t := g.e.Text()
	g.bqFont(3, bqBright)
	t.SetSpacing(1)
	t.SetSpaceWidth(2)
}

// printAppDescription is Print_App_Description_ (0xafb0d): help record
// app's text.
func (g *Game) printAppDescription(x, y, w, app int) error {
	g.bqDescFont()
	rec, err := g.helpRecord(app)
	if err != nil {
		return err
	}
	if err := g.e.Text().PrintParagraph(x, y, w, save.CString(rec.text), 3); err != nil {
		return err
	}
	g.bqFont(3, bqBright)
	return nil
}

// printSettingDescription is Print_Setting_Description_ (0xafa5c): record
// 0 of entry n of the file E 0x170 names.
func (g *Game) printSettingDescription(x, y, w, n int) error {
	g.bqDescFont()
	a, err := g.archive(strings.ToUpper(g.eStr(0x170)))
	if err != nil {
		return err
	}
	tab, err := a.Table(n)
	if err != nil {
		return err
	}
	if err := g.e.Text().PrintParagraph(x, y, w, save.CString(tab.Record(0)), 3); err != nil {
		return err
	}
	g.bqFont(3, bqBright)
	return nil
}

// drawShipDesignInfo is Draw_Ship_Design_Info_ (0xb12ac): design item of
// the player, then its picture.
func (g *Game) drawShipDesignInfo(item int16) error {
	s := &g.state
	t := g.e.Text()
	k := int(-(item + 50))
	d := &s.Players[s.PlayerNum].Designs[k]
	if bqIsDesign(item) {
		g.bqFont(4, bqBright)
		if err := t.Print(0x132, bqLine(0), save.CString(d.Name[:])); err != nil {
			return err
		}
		g.bqFont(3, bqBright)
		line := 2
		t.SetSpacing(1)
		t.SetSpaceWidth(2)
		if err := g.drawCostAndTimeInfo(g.colonyHandle, item, &line); err != nil {
			return err
		}
		if err := g.drawShipDesignInfo2(d); err != nil {
			return err
		}
	}
	return g.remapDrawShipSegNCentered(0xf4, 0x38, int16(int8(d.Picture)))
}

// drawShipInfo is Draw_Ship_Info_ (0xb13a0): a queued ship (a refit).
func (g *Game) drawShipInfo(item int16) error {
	s := &g.state
	t := g.e.Text()
	k := int(-(item + 100))
	sh := &s.Ships[k]
	g.bqFont(4, bqBright)
	if err := t.Print(0x132, bqLine(0), save.CString(sh.Design.Name[:])); err != nil {
		return err
	}
	g.bqFont(3, bqBright)
	line := 2
	t.SetSpacing(1)
	t.SetSpaceWidth(2)
	if err := g.drawCostAndTimeInfo(g.colonyHandle, item, &line); err != nil {
		return err
	}
	if err := g.drawShipDesignInfo2(&sh.Design); err != nil {
		return err
	}
	return g.remapDrawShipCentered(0xf4, 0x38, k)
}

// drawShipDesignInfo2 is Draw_Ship_Design_Info2_ (0xb124a).
func (g *Game) drawShipDesignInfo2(d *save.Design) error {
	t := g.e.Text()
	g.bqFont(1, bqBright)
	t.SetSpacing(1)
	t.SetSpaceWidth(2)
	p, err := g.buildShipDesignInfoFormattedParagraph(d, -1)
	if err != nil {
		return err
	}
	return t.SqueezePrintFormattedParagraph(0xd4, 0x72, 0xf8, 0xba, p, 0, g.exe.bytes(spaceLimits, 6), false)
}

// buildWeaponString is Build_Weapon_String_ (0xb0bae) without the suffix,
// as the paragraph calls it: modifications 1 and 2 before the name.
func (g *Game) buildWeaponString(w *save.WeaponSlot) (string, error) {
	walk := walksWeaponsPl
	if w.Count <= 1 {
		walk = walksWeapons
	}
	name, err := g.techName(walk + 1 + int(w.Weapon))
	if err != nil {
		return "", err
	}
	var pre string
	for bit := 1; bit <= 2; bit++ {
		if w.Mods>>bit&1 != 0 {
			m, err := g.techName(walksMods2 + 1 + bit)
			if err != nil {
				return "", err
			}
			pre += m + cStringAt(g.exe, bqSpaceStr)
		}
	}
	return cSprintf(g.eStr(0x51), pre, name, g.eStr(0xc)), nil
}

// buildShipDesignInfoFormattedParagraph is
// Build_Ship_Design_Info_Formatted_Paragraph_ (0xb0cf6): for a ship (ship
// >= 0, d its design) the name, officer and crew line first; then cost,
// shield, computer, armour, drive, weapons and specials in two columns,
// wide names after them.
func (g *Game) buildShipDesignInfoFormattedParagraph(d *save.Design, ship int) (string, error) {
	t := g.e.Text()
	n, tab := cStringAt(g.exe, bqFmtNormal), cStringAt(g.exe, bqFmtTab)
	hdr := g.exe.bytes(bqParaHeader, 13)
	var b strings.Builder
	b.WriteString(cSprintf(g.eStr(0x46), save.CString(hdr)))
	if ship >= 0 {
		name, crew := save.CString(d.Name[:]), g.crewDescriptionString(ship)
		// An officer index of 0 counts as none (jg 0).
		if o := g.state.Ships[ship].Officer; o > 0 {
			b.WriteString(cSprintf(g.eStr(0x4e), n, name, g.leaderName(int(o), false), crew))
		} else {
			b.WriteString(cSprintf(g.eStr(0x4f), n, name, crew, cStringAt(g.exe, bqFmtBright)))
		}
	}
	b.WriteString(cSprintf(g.eStr(0x63), n, tab, int(d.Cost)))
	for _, p := range []struct {
		e, walk int
		v       uint8
	}{{0x77, walksShields, d.Shield}, {0x62, walksComputers, d.Computer}, {0x5b, walksArmor, d.Armor}, {0x68, walksDrives, d.Drive}} {
		name, err := g.techName(p.walk + 1 + int(p.v))
		if err != nil {
			return "", err
		}
		b.WriteString(cSprintf(g.eStr(p.e), n, tab, name))
	}
	b.WriteString(cSprintf(g.eStr(0x82), n))
	var long []string
	var longN []int
	k := 0
	for i := range d.Weapons {
		w := &d.Weapons[i]
		if w.Weapon == 0 {
			continue
		}
		ws, err := g.buildWeaponString(w)
		if err != nil {
			return "", err
		}
		if t.Width([]byte(ws)) > 0x6e {
			long = append(long, ws)
			longN = append(longN, int(w.Count))
			continue
		}
		f := 0x57
		if k%2 != 0 {
			f = 0x58
		}
		b.WriteString(cSprintf(g.eStr(f), n, int(w.Count), ws))
		k++
	}
	if len(long) > 0 {
		if k%2 != 0 {
			b.WriteString(g.eStr(0x7f))
		}
		for i, ws := range long {
			b.WriteString(cSprintf(g.eStr(0x58), n, longN[i], ws))
		}
	}
	if k%2 == 1 {
		b.WriteString(g.eStr(0x7f))
	}
	long = long[:0]
	k = 0
	hdrDone := false
	for bit := 0; bit < 0x27; bit++ {
		if !testBit(d.Specials[:], bit) {
			continue
		}
		if !hdrDone {
			b.WriteString(cSprintf(g.eStr(0x81), n))
			hdrDone = true
		}
		name, err := g.techName(walksSpecials + 1 + bit)
		if err != nil {
			return "", err
		}
		if t.Width([]byte(name)) > 0x73 {
			long = append(long, name)
			continue
		}
		f := 0x59
		if k%2 != 0 {
			f = 0x5a
		}
		b.WriteString(cSprintf(g.eStr(f), n, name))
		k++
	}
	if len(long) > 0 {
		if k%2 != 0 {
			b.WriteString(g.eStr(0x7f))
		}
		for _, name := range long {
			b.WriteString(cSprintf(g.eStr(0x5a), n, name))
		}
	}
	return b.String(), nil
}
