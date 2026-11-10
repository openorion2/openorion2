package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// Drawing the colony screen (COLONY.C, COLDRAW.C, COLSYSDI.C). Spec:
// docs/screens/colony-screen.md, "Drawing".

// drawColonyScreen is Draw_Colony_Screen_ (0xbed21), the auto function.
func (g *Game) drawColonyScreen() error {
	e, cs, cg := g.e, &g.cscr, &g.col
	v := e.V
	v.SetPageOff()
	if g.fullDrawScreen {
		g.fullDrawScreen = false
		cg.fullDraw = true
	}
	if cg.fullDraw {
		v.Fill(0, 0, 0x27f, 0x1df, 0)
		for _, n := range []int{1, 0} {
			p, err := g.cAnimsColony(n)
			if err != nil {
				return err
			}
			if err := e.Draw(0, 0, p); err != nil {
				return err
			}
		}
		if err := g.drawRoadList(); err != nil {
			return err
		}
		if cs.valid {
			if err := g.drawColonyInfoBackground(); err != nil {
				return err
			}
		}
		switch cs.display {
		case 0, 2:
			g.setColonyFontTo(3)
			if err := g.drawColSysDisp(7, 0x18); err != nil {
				return err
			}
		case 1:
			gm := &g.ms.galaxyMap
			if err := g.drawGalaxyMapBox(nil, 7, 0x18, gm.width, gm.height, 0xb, 0x1f, 0, 0, 3, 0, 0); err != nil {
				return err
			}
		}
		if cs.valid {
			if err := g.drawColonyBldgs(); err != nil {
				return err
			}
			if err := g.drawColonySatellites(); err != nil {
				return err
			}
		}
		v.CopyOffToBack()
		v.SetRefreshStencil()
		cg.fullDraw = false
	} else if cs.valid {
		if err := g.drawColonyBldgs(); err != nil {
			return err
		}
		if err := g.drawColonySatellites(); err != nil {
			return err
		}
	}
	if cs.display == 1 {
		if err := g.boxGalaxyMapStar(cs.starHandle); err != nil {
			return err
		}
		if cs.starHandle != cs.sysStar {
			if err := g.boxGalaxyMapStar(cs.sysStar); err != nil {
				return err
			}
		}
	}
	if cs.valid {
		for _, fn := range []func() error{g.drawColonyInfoMilitary, g.drawColonyInfoOfficer, g.drawColonyInfoProduction,
			g.drawColonyInfoPop, g.drawColonyInfoMorale, g.drawInfoNameAndPop, g.drawInfoBuild,
			g.drawSysDispScanned, g.drawColonyInfoScanned} {
			if err := fn(); err != nil {
				return err
			}
		}
	}
	// Draw_Fake_Buttons_: nothing adds fake buttons here.
	if err := e.DrawVisibleFields(); err != nil {
		return err
	}
	if cs.fieldMode == 1 {
		px, py, _ := e.Sys.Mouse()
		if err := g.drawCluster(px, py); err != nil {
			return err
		}
	}
	e.CyclePaletteColor(0xff, 0x13, 0x10, 9, 0x20, 0x28, 0x15, 2)
	return nil
}

// drawColonyInfoBackground is Draw_Colony_Info_Background_ (0xbd3e3): the
// top panel, with the transport button's frame while colonists are picked
// up.
func (g *Game) drawColonyInfoBackground() error {
	if !g.col.fullDraw {
		return nil
	}
	g.darkenFill(0, 0, 0x280, 0x96)
	n := 3
	if g.getFirstUnemployed(g.colonyHandle) != -1 {
		n = 4
	}
	p, err := g.cAnimsColony(n)
	if err != nil {
		return err
	}
	return g.e.Draw(0, 0, p)
}

// militaryAnim is Military_Anim_ (0xbb5db): marines (0), armour (1) or
// another troop picture of a race, upgraded by the screen player's
// technologies.
func (g *Game) militaryAnim(k int16, race int) (*gfx.Picture, error) {
	p := &g.state.Players[g.state.PlayerNum]
	n := race * 0xd
	switch k {
	case -1:
		n += 6
	case 0:
		n += 7
		if p.AppStatus[0x90] == 3 {
			n++
		}
	case 1:
		n += 9
		if p.AppStatus[0x18] == 3 {
			n++
		}
	default:
		n += int(k) + 6
	}
	return g.cached(cStringAt(g.exe, raceiconLBX), int(int16(n)))
}

// doColonyInfoMilitaryStuffFor is Do_Colony_Info_Military_Stuff_For_
// (0xbcea2): troops of kind k (marines, armour) from icon slot x on: draw
// them (mode 0), add their field (1) or compute the overlap of both kinds
// (2). Squeezed, one icon and a count are drawn, even in mode 1.
func (g *Game) doColonyInfoMilitaryStuffFor(k, x, mode int) (int, error) {
	e, s, cs, cg := g.e, &g.state, &g.cscr, &g.col
	c := &s.Colonies[g.colonyHandle]
	n := int(militaryCount(c, k))
	race := int(s.Players[c.Owner].Race)
	cg.stepSquish = 0
	if mode != 2 {
		if _, err := g.doColonyInfoMilitaryStuffFor(k, x, 2); err != nil {
			return 0, err
		}
	}
	g.setColonyFontTo(3)
	drawn := 0
	pic := func() (*gfx.Picture, error) { return g.militaryAnim(int16(k), race) }
	if cg.stepSquish == 0 {
		for i := 0; i < n; i++ {
			if mode == 0 {
				p, err := pic()
				if err != nil {
					return 0, err
				}
				if err := e.Draw(int(int16((0x1e-cg.stepSquish)*int16(x+i))), int(int16(0x1df-int16(p.Height))), p); err != nil {
					return 0, err
				}
			}
		}
		drawn = n
	} else {
		cg.stepSquish = 0
		if n > 0 {
			p, err := pic()
			if err != nil {
				return 0, err
			}
			y := int(int16(0x1df - int16(p.Height)))
			if err := e.Draw(int(int16(0x1e*x)), y, p); err != nil {
				return 0, err
			}
			t := e.Text()
			t.SetOutlineColor(0x50, 0x50)
			t.SetStyleShadow(3, g.colors(colorsFont), font.ShadowDown)
			t.SetHighlightColors(g.colors(colorsBrightBlue))
			t.SetSpecialColors(g.colors(colorsRed))
			if err := t.Print(int(int16((x+1)*0x1e)), y, cSprintf(g.eStr(0x7c), n)); err != nil {
				return 0, err
			}
			drawn = 3
		}
	}
	switch mode {
	case 1:
		if drawn > 0 {
			p, err := pic()
			if err != nil {
				return 0, err
			}
			step := int(0x1e - cg.stepSquish)
			f, err := e.AddHiddenField(int(int16(step*x)), int(int16(0x1df-int16(p.Height))), int(int16((drawn+x)*step-1)), 0x1df, g.eStr(0xc), 0)
			if err != nil {
				return 0, err
			}
			cs.militaryF[k] = f
		} else {
			cs.militaryF[k] = -1000
		}
	case 2:
		total := n + int(militaryCount(c, (k+1)%2))
		step := 1
		if q := 0x104 / max(total, 1); q > 1 {
			step = q
		}
		if int16(step) < 0x1e {
			cg.stepSquish = int16(0x1e - step)
		} else {
			cg.stepSquish = 0
		}
	}
	return drawn + x, nil
}

// militaryCount is a colony's marines (0) or armour (1).
func militaryCount(c *save.Colony, k int) int16 {
	if k == 0 {
		return c.Marines
	}
	return c.Armor
}

// drawColonyInfoMilitary is Draw_Colony_Info_Military_ (0xbd1fb).
func (g *Game) drawColonyInfoMilitary() error {
	x := 0
	for k := 0; k < 2; k++ {
		var err error
		if x, err = g.doColonyInfoMilitaryStuffFor(k, x, 0); err != nil {
			return err
		}
	}
	return nil
}

// addMilitaryFields is Add_Military_Fields_ (0xbd236).
func (g *Game) addMilitaryFields() error {
	x := 0
	for k := 0; k < 2; k++ {
		var err error
		if x, err = g.doColonyInfoMilitaryStuffFor(k, x, 1); err != nil {
			return err
		}
	}
	return nil
}

// drawColonyInfoOfficer is Draw_Colony_Info_Officer_ (0xbd26c): the
// governor's frame, portrait and name, or his ETA while on the way.
func (g *Game) drawColonyInfoOfficer() error {
	e, s, cs := g.e, &g.state, &g.cscr
	o := s.Stars[cs.starHandle].Officer[s.PlayerNum]
	if o == -1 {
		return nil
	}
	buf, err := g.cached(buffer0LBX, 0)
	if err != nil {
		return err
	}
	if err := e.CalculatePictureRemapColors(buf); err != nil {
		g.logOnce(err.Error())
	}
	for _, d := range [][3]int{{0x21, 0x220, 0x15f}, {0x1e, 0x226, 0x165}} {
		p, err := g.cAnimsColony(d[0])
		if err != nil {
			return err
		}
		if p == nil {
			continue
		}
		if err := e.Draw(d[1], d[2], p); err != nil {
			return err
		}
	}
	t := e.Text()
	t.SetOutlineColor(0x50, 0x50)
	t.SetStyleShadow(2, g.colors(colorsFont), font.ShadowDown)
	t.SetHighlightColors(g.colors(colorsBrightBlue))
	t.SetSpecialColors(g.colors(colorsRed))
	ld := &s.Leaders[o]
	var str string
	if ld.ETA > 0 {
		str = cSprintf(g.eStr(0x11d), int(ld.ETA))
	} else {
		g.darkenFill(0x226, 0x1b1, 0x26f, 0x1ba)
		str = save.CString(ld.Name[:])
	}
	return t.PrintCentered(0x24a, 0x1b1, str)
}

// drawColonyInfoProduction is Draw_Colony_Info_Production_ (0xbce5e): BC,
// food, industry and research rows.
func (g *Game) drawColonyInfoProduction() error {
	ci := g.colonyHandle
	for _, k := range []int{3, 0, 1} {
		y := int16(k*0x1e + 0x41 - 1)
		if k == 3 {
			y = int16(-1*0x1e + 0x41 - 3)
		}
		if err := g.drawColonyProdBoth(ci, k, 0x80, y, 0x12d, 0x14, false); err != nil {
			return err
		}
	}
	return g.drawColonyProdBoth(ci, 2, 0x80, 0x7c, 0x12d, 0x14, false)
}

// drawColonyInfoPop is Draw_Colony_Info_Pop_ (0xbcd70) and, falling into
// it, Draw_Colony_Info_Pop_Scanned_: the three job rows and the hovered
// colonist's description.
func (g *Game) drawColonyInfoPop() error {
	for j := 0; j < 3; j++ {
		if _, err := g.colonyPopStuff(j, 0); err != nil {
			return err
		}
	}
	k := g.col.scannedPop
	if k == -1 {
		return nil
	}
	str := g.colonyPopString(g.colonyHandle, int(k))
	g.setColonyFontTo(3)
	return g.printScannedString(str)
}

// drawColonyInfoMorale is Draw_Colony_Info_Morale_ (0xbd3c1).
func (g *Game) drawColonyInfoMorale() error {
	return g.drawInfoMoraleBoth(g.colonyHandle, 0x136, 0x21, 0x1fe, false)
}

// drawInfoNameAndPop is Draw_Info_Name_And_Pop_ (0xbd434): the title, the
// colony's state on the left (blockade, plague, boom) and the population.
func (g *Game) drawInfoNameAndPop() error {
	e, s, cs := g.e, &g.state, &g.cscr
	t := e.Text()
	ci := g.colonyHandle
	c := &s.Colonies[ci]
	normal := func(style int) {
		t.SetOutlineColor(0x50, 0x50)
		t.SetStyleShadow(style, g.colors(colorsFont), font.ShadowDown)
		t.SetHighlightColors(g.colors(colorsBrightBlue))
		t.SetSpecialColors(g.colors(colorsRed))
	}
	var title string
	if c.Occupation != 0 {
		title = cSprintf(g.eStr(0x61), g.colonyTypeString(ci), g.planetName(cs.planet))
		normal(4)
	} else {
		title = cSprintf(g.eStr(0xb5), g.planetName(cs.planet))
		g.setColonyFontToRed(4, g.globalCycler(0x1e, 2))
	}
	t.SetSpacing(2)
	if err := t.PrintCentered(0x140, 0, title); err != nil {
		return err
	}
	msg := -1
	switch {
	case s.Stars[g.colonyStar(ci)].Blockaded>>s.PlayerNum&1 != 0:
		g.setColonyFontToRed(4, g.globalCycler(0x1e, 2))
		t.SetSpacing(2)
		msg = 0xca
	case s.eventCheckPlague(ci):
		g.setColonyFontToRed(4, g.globalCycler(0x1e, 2))
		t.SetSpacing(2)
		msg = 0x19f
	case s.eventCheckPopulationBoom(ci):
		normal(4)
		t.SetSpacing(2)
		msg = 0x1aa
	}
	if msg != -1 {
		if err := t.Print(0, 0, g.eStr(msg)); err != nil {
			return err
		}
	}
	normal(4)
	t.SetSpacing(3)
	var frac, growth int32
	for i := range c.PopFraction {
		frac += int32(c.PopFraction[i])
		growth += int32(c.PopGrowth[i])
	}
	n := int32(int16(g.sumColonists(ci, -1, -1, -1, -1, -1)))
	str := cSprintf(g.eStr(0x1a8), int(frac/1000+n), int(frac%1000), int(growth))
	t.SetStyleShadow(2, g.colors(colorsFont), font.ShadowDown)
	if err := t.PrintRight(0x27e, 3, str); err != nil {
		return err
	}
	g.setColonyFontTo(3)
	return nil
}

// colonyTypeString is Colony_Type_ (0xbc8df): "Agricultural ", ... by the
// colony's specialty.
func (g *Game) colonyTypeString(ci int) string {
	switch uint8(g.state.Colonies[ci].Specialty) {
	case 0:
		return g.eStr(0x258)
	case 1:
		return g.eStr(0x259)
	case 0xff:
		return g.eStr(0xc)
	}
	return g.eStr(0x25a)
}

// setColonyFontToRed is Set_Colony_Font_To_Red_ (0xbb012): the red set,
// bright when n is 1.
func (g *Game) setColonyFontToRed(style, n int) {
	t := g.e.Text()
	t.SetOutlineColor(0x50, 0x50)
	red := g.colors(colorsRed + uint32(n)*8)
	t.SetStyleShadow(style, red, font.ShadowDown)
	t.SetHighlightColors(g.colors(colorsBrightBlue))
	t.SetSpecialColors(red)
}

// drawInfoBuild is Draw_Info_Build_ (0xbdd2f): the product's picture,
// progress bar, name, the autobuild flag and the turns left.
func (g *Game) drawInfoBuild() error {
	e, s := g.e, &g.state
	q := s.Colonies[g.colonyHandle].Queue[0]
	race := int(s.Players[s.PlayerNum].Race)
	var err error
	switch {
	case q == -0xf:
		err = g.remapDrawShipSegNCentered(0x242, 0x4d, 0x30)
	case q == -0xc:
		err = g.remapDrawShipSegNCentered(0x242, 0x39, 0x2d)
	case q == -0xb:
		err = g.remapDrawShipSegNCentered(0x242, 0x39, 0x2f)
	case q == -8, q == -2, q == -1, q == 0:
	case q == -7:
		var p *gfx.Picture
		if p, err = g.cached(cStringAt(g.exe, raceiconLBX), int(int16(race*0xd+0xb))); err == nil {
			err = g.drawCentered(0x242, 0x4d, p)
		}
	case q == -6, q == -5, q == -4:
		var p *gfx.Picture
		if p, err = g.peopleAnim(0, 4, race); err == nil {
			err = g.drawCentered(0x242, 0x4d, p)
		}
	case q == -3:
		err = g.drawCurrentBuilding2(0x242, 0x94, -3)
	default:
		g.setColonyFontTo(3)
		switch {
		case q > 0 && q < 0x31:
			err = g.drawCurrentBuilding2(0x242, 0x94, int32(q))
		case q <= -100:
			e.V.SetWindow(0x205, 0x11, 0x27f, 0x9e)
			e.V.ClipOn()
			err = g.remapDrawShipCentered(0x242, 0x4d, -(int(q) + 100))
			e.V.ClipOff()
		default:
			// PFMT_ and hgetch_ of the debug build: "current item == %d".
			g.logOnce(cSprintf(g.eStr(0xf7), int(q)))
		}
	}
	if err != nil {
		return err
	}
	if err := g.drawCurrentProductionGraph(); err != nil {
		return err
	}
	t := e.Text()
	name := g.colbldgSelectionName(q)
	t.SetStyleShadow(1, g.colors(colorsFont), font.ShadowDown)
	if err := t.SqueezePrintParagraph(0x20a, 0x26, 0x70, 0x8d, name, 2, g.exe.bytes(spaceLimits, 6), false); err != nil {
		return err
	}
	if s.Colonies[g.colonyHandle].AutoBuild != 0 {
		t.SetStyleShadow(1, g.colors(colorsBrightBlue), font.ShadowDown)
		if err := t.PrintCentered(0x242, 0x1c, g.eStr(0xbd)); err != nil {
			return err
		}
	}
	return g.drawTurnCount()
}

// colbldgSelectionName is COLBLDG.C's Selection_Name_ (0xb2ffa).
func (g *Game) colbldgSelectionName(p int16) string {
	return g.selectionName(p)
}

// drawCurrentBuilding2 is Draw_Current_Building2_ (0xbdab4): a building in
// the product window (a house for -3), satellites centred above.
func (g *Game) drawCurrentBuilding2(x, y int, id int32) error {
	e := g.e
	if id != -3 && g.buildingType(id) == 7 {
		p, err := g.satelliteAnims(satelliteAnimIndex(uint16(id)))
		if err != nil {
			return err
		}
		return g.drawCentered(x, int(int16(y-0x46)), p)
	}
	if id == -3 {
		id = 0xe
	}
	e.V.SetWindow(0x205, 0x11, 0x27f, 0x9e)
	e.V.ClipOn()
	err := g.drawBuildingWithBottomCentered(0x243, 0x6a, id, 5, 5)
	e.V.ClipOff()
	return err
}

// drawBuildingWithBottomCentered is Draw_Building_With_Bottom_Centered_
// (0xbd988): building id as drawn in cell (c, r), cut out of its frame and
// stood on (x, y).
func (g *Game) drawBuildingWithBottomCentered(x, y int, id int32, c, r int) error {
	p, err := g.cacheLoadBldg(c, r, id)
	if err != nil {
		return err
	}
	bx := int(g.crToXY(c, r, 0)) - 0x3c
	by := int(g.crToXY(c, r, 1)) - 0x78
	bm := gfx.NewBlankBitmap(0x78, 0x78)
	if err := bm.AddPicture(int(int16(-bx)), int(int16(-by)), p); err != nil {
		return err
	}
	bm.MaskOutColors(0xf0, 0xff)
	x1, y1, w, h := bm.ActualSize()
	dx := int(int16(x - int(int16(w))/2 - x1))
	dy := int(int16(y - y1 - h))
	return g.e.Draw(dx, dy, bm)
}

// drawCurrentProductionGraph is Draw_Current_Production_Graph_ (0xbda57).
func (g *Game) drawCurrentProductionGraph() error {
	c := &g.state.Colonies[g.colonyHandle]
	cost := int16(g.colonyProductCost(c, c.Queue[0]))
	prod := c.Production
	if c.Pending != 0 {
		prod = cost
	}
	return g.drawColonyPCBar(0x25e, 0x2b, cost, prod)
}

// drawColonyPCBar is Draw_Colony_PC_Bar_ (0xb5ed2): the production bar, two
// animation frames chosen by progress.
func (g *Game) drawColonyPCBar(x, y int, cost, prod int16) error {
	e := g.e
	d := int32(cost)
	if cost < 1000 {
		d = 1000
	}
	if prod > cost {
		prod = cost
	}
	p := int32(prod)
	f1 := p * 0x32 / d
	rem := int32(cost) - p
	f2 := rem*-0x32/d + 0x32
	a := int32(int16(rem << 6 / d))
	b := int32(cost) << 6 / d
	c := (d-p)*-10/d + 10
	y2 := int(int16(int32(int16(a-b+c)) + int32(y)))
	e.V.SetWindow(0, 0, 0x27f, 0x1df)
	e.V.ClipOn()
	top, err := g.cAnimsColony(0x23)
	if err != nil {
		return err
	}
	setFrame(top, int(int16(f1)))
	if err := e.Draw(x, y, top); err != nil {
		return err
	}
	bar, err := g.cAnimsColony(0x22)
	if err != nil {
		return err
	}
	setFrame(bar, int(int16(f2)))
	if err := e.Draw(x, y2, bar); err != nil {
		return err
	}
	e.V.ClipOff()
	return nil
}

// drawTurnCount is Draw_Turn_Count_ (0xbdcd8). Community patch 1.51 (0x40e5de):
// the turns go left at 531 and stored (with bought) / cost right at 625.
func (g *Game) drawTurnCount() error {
	n := g.calculateCurrentProductionTurnCount(g.colonyHandle)
	t := g.e.Text()
	t.SetStyleShadow(0, g.colors(colorsFont), font.ShadowDown)
	if err := t.Print(0x213, 0x67, cSprintf(g.eStr(0x20), int(n))); err != nil {
		return err
	}
	c := &g.state.Colonies[g.colonyHandle]
	cost := uint16(g.colonyProductCost(c, c.Queue[0]))
	stored := int(uint16(c.Production)) + int(uint16(c.Pending))
	return t.PrintRight(0x271, 0x67, fmt.Sprintf("%d / %d", stored, cost))
}

// printScannedString is Print_Scanned_String_ (0xbd727): a dark strip and
// the text centred at the bottom.
func (g *Game) printScannedString(str string) error {
	g.setColonyFontTo(3)
	t := g.e.Text()
	w, err := t.ParagraphMaxWidth(0x280, str, 0)
	if err != nil {
		return err
	}
	half := int(int16(w)) / 2
	g.darkenFill(int(int16(0x13f-half-10)), 0x1cf, int(int16(half+0x149)), 0x1de)
	return t.PrintCentered(0x13f, 0x1d3, str)
}

// drawColonyInfoScanned is Draw_Colony_Info_Scanned_ (0xbd7f2): the
// hovered troops, satellite or building.
func (g *Game) drawColonyInfoScanned() error {
	s, cs := &g.state, &g.cscr
	if k := cs.scannedMilitary; k != -1 {
		c := &s.Colonies[g.colonyHandle]
		str := cSprintf(g.eStr(0xe), int(militaryCount(c, int(k))), g.unitName(int(k)+1))
		if err := g.printScannedString(str); err != nil {
			return err
		}
	}
	if id := cs.scannedSat; id != -1 {
		n, err := g.buildingName(int(id))
		if err != nil {
			return err
		}
		if err := g.printScannedString(n); err != nil {
			return err
		}
	}
	if cs.scannedC != -1 && cs.scannedR != -1 {
		n, err := g.crBuildingName(int16(cs.surf.bldgs[cs.scannedC][cs.scannedR]))
		if err != nil {
			return err
		}
		return g.printScannedString(n)
	}
	return nil
}

// unitName is _units[k].name (0x17ffb2, 12-byte rows).
func (g *Game) unitName(k int) string {
	return cStringAt(g.exe, g.exe.u32(uint32(0x17ffa6+k*0xc)))
}

// crBuildingName is CR_Building_Name_ (0xbb3db): houses are "Housing",
// building 8 is "Rubble".
func (g *Game) crBuildingName(id int16) (string, error) {
	switch id {
	case 3, 0xe, 0x28, 0x29:
		return g.eStr(0x142), nil
	case 8:
		return g.eStr(0x1c4), nil
	}
	return g.buildingName(int(id))
}

// boxGalaxyMapStar is Box_Galaxy_Map_Star_ (0xbef4d): a pulsing box round
// a star of the small map, then a pause of two ticks.
func (g *Game) boxGalaxyMapStar(star int) error {
	gm := &g.ms.galaxyMap
	x, y := g.galaxyMapStarXY(star, 7, 0x18, gm.width, gm.height, 0xb, 0x1f, false)
	g.e.Box(int(x), int(y), 5, 5, 1, 0xff)
	g.e.CyclePaletteColor(0xff, 0x13, 0x10, 9, 0x20, 0x28, 0x15, 2)
	return g.e.ReleaseTime(2)
}

// drawSysDispScanned is Draw_Sys_Disp_Scanned_ (0xbf581).
func (g *Game) drawSysDispScanned() error {
	cs := &g.cscr
	switch cs.display {
	case 0:
		return g.drawColSysDispLastScanned(7, 0x18)
	case 1:
		if cs.scanStar != -1 {
			g.setColonyFontTo(3)
			name := save.CString(g.state.Stars[cs.scanStar].Name[:])
			if err := g.e.Text().PrintCentered(0x3d, 0x87, name); err != nil {
				return err
			}
			return g.boxGalaxyMapStar(int(cs.scanStar))
		}
	}
	return nil
}

// drawColSysDisp is Draw_Col_Sys_Disp_ (0xc49c9): the five orbits of the
// system display star, planet pictures and summaries.
func (g *Game) drawColSysDisp(x, y int) error {
	s, cs := &g.state, &g.cscr
	sysdi := cStringAt(g.exe, colsysdiLBX)
	ty := y + 7
	for o := 0; o < 5; o++ {
		pl := int(s.Stars[cs.sysStar].Planets[o])
		cy := o*0x18 + y + 0xe
		if pl != -1 {
			P := &s.Planets[pl]
			climate := P.Climate
			if P.Colony != -1 {
				climate = s.Colonies[P.Colony].Climate
			}
			var p *gfx.Picture
			var err error
			switch P.Type {
			case 3:
				p, err = g.cached(sysdi, int(climate)*5+int(P.Size)+0xb)
			case 2:
				var gas *gfx.Picture
				if gas, err = g.cached(sysdi, 0x3e); err == nil {
					err = g.drawCentered(x+0xf, cy, gas)
				}
			case 1:
				p, err = g.cached(sysdi, 0x3f)
			}
			if err != nil {
				return err
			}
			if p != nil {
				if err := g.drawCentered(x+0xf, cy, p); err != nil {
					return err
				}
			}
			g.setColonyFontTo(1)
			str := g.planetSummaryString(pl, 1)
			if err := g.e.Text().PrintParagraph(int(int16(x+0x1c)), int(int16(ty)), int(int16(0x75-(x+0x1c))), str, 0); err != nil {
				return err
			}
		}
		ring, err := g.cached(sysdi, 0x41)
		if err != nil {
			return err
		}
		if err := g.drawCentered(x+2, cy-2, ring); err != nil {
			return err
		}
		ty += 0x18
	}
	return nil
}

// planetSummaryString is Planet_Summary_String_ (0xc4b98): race, gravity,
// minerals and population of a colony, or size, climate, gravity and
// minerals of a free planet; fontPlayer sets the owner's colours.
func (g *Game) planetSummaryString(pl int, fontPlayer int16) string {
	s := &g.state
	P := &s.Planets[pl]
	grav := []string{g.eStr(0x169), g.eStr(0xc), g.eStr(0x25b)}
	min := []string{g.eStr(0x222), g.eStr(0x1a7), g.eStr(0xc), g.eStr(0x1c2), g.eStr(0x223)}
	size := []string{g.eStr(0x1f6), g.eStr(0x25c), g.eStr(0x25d), g.eStr(0x25e), g.eStr(0x25f)}
	switch P.Type {
	case 3:
		ci := P.Colony
		climate := P.Climate
		if ci != -1 {
			climate = s.Colonies[ci].Climate
		}
		pop := g.eStr(0xc)
		if ci != -1 {
			c := &s.Colonies[ci]
			if fontPlayer != -1 {
				g.setColonyFontToPlayer(int(fontPlayer), int(uint8(c.Owner)))
			}
			pop = cSprintf(g.eStr(8), int(c.PopCount), int(g.planetMaxPopulationForPlayer(pl, int(c.Owner))))
			return cSprintf(g.eStr(0x29), save.CString(s.Players[c.Owner].RaceName[:]), grav[P.Gravity], min[P.Minerals], pop)
		}
		return cSprintf(g.eStr(0x2c), size[P.Size], g.eStr(planetClimateEStr[climate]), grav[P.Gravity], min[P.Minerals], pop)
	case 2:
		return cSprintf(g.eStr(0x46), g.eStr(0x132))
	case 1:
		return cSprintf(g.eStr(0x46), g.eStr(0xba))
	}
	return ""
}

// playerFontColorArray is _player_font_color_array (0x182c56).
const playerFontColorArray = 0x182c56

// setColonyFontToPlayer is Set_Colony_Font_To_Player_ (0xbaf3c).
func (g *Game) setColonyFontToPlayer(style, player int) {
	t := g.e.Text()
	t.SetOutlineColor(0x50, 0x50)
	t.SetStyleShadow(style, g.colors(playerFontColorArray+uint32(g.state.Players[player].Color)*8), font.ShadowDown)
	t.SetHighlightColors(g.colors(colorsBrightBlue))
	t.SetSpecialColors(g.colors(colorsRed))
}

// drawColSysDispLastScanned is Draw_Col_Sys_Disp_Last_Scanned_ (0xc500b):
// the cycling marker at the colony's orbit.
func (g *Game) drawColSysDispLastScanned(x, y int) error {
	s := &g.state
	pl := s.Colonies[g.colonyHandle].Planet
	if pl == -1 {
		return nil
	}
	sysdi := cStringAt(g.exe, colsysdiLBX)
	P := &s.Planets[pl]
	climate := P.Climate
	if P.Colony != -1 {
		climate = s.Colonies[P.Colony].Climate
	}
	o := 0
	for o <= int(P.Orbit) {
		o++
	}
	// The planet's picture is loaded for nothing.
	if _, err := g.cached(sysdi, int(P.Size)+int(climate)*5+0xb); err != nil {
		return err
	}
	f := g.globalCycler(0x32, 6)
	p, err := g.cached(sysdi, 0x40)
	if err != nil {
		return err
	}
	setFrame(p, f)
	return g.drawCentered(x+2, (o-1)*0x18+y+0xc, p)
}

// drawColonySatellites is Draw_Colony_Satellites_ (0xbe366).
func (g *Game) drawColonySatellites() error {
	cs := &g.cscr
	if len(g.ub.savedMouseList) != 0 {
		return nil
	}
	for i, id := range cs.surf.satellites {
		if id == -1 {
			continue
		}
		x := satelliteX(i)
		if cs.scannedSat == id && g.globalCycler(0x1e, 2) != 0 {
			if err := g.drawSatelliteDark(x, 0xa2, id); err != nil {
				return err
			}
			continue
		}
		p, err := g.satelliteAnims(satelliteAnimIndex(uint16(id))) // Satellite_Anim_ (0xbe306)
		if err != nil {
			return err
		}
		if err := g.e.Draw(x, 0xa2, p); err != nil {
			return err
		}
	}
	return nil
}

// drawSatelliteDark is Draw_Satellite_Dark_ (0xbe271): the satellite
// tinted, for the hovered one.
func (g *Game) drawSatelliteDark(x, y int, id int16) error {
	p, err := g.satelliteAnims(satelliteAnimIndex(uint16(id)))
	if err != nil {
		return err
	}
	bm := gfx.NewBlankBitmap(0x78, 0x78)
	if err := bm.AddPicture(0, 0, p); err != nil {
		return err
	}
	bm.MaskOutColors(0xf0, 0xff)
	g.e.TintBitmap(bm, 2)
	g.e.V.SetWindow(0, 0, 0x27f, 0x1df)
	g.e.V.ClipOn()
	err = g.e.Draw(x, y, bm)
	g.e.V.ClipOff()
	return err
}

// drawColonyBldgs is Draw_Colony_Bldgs_ (0xbebdc): back to front, each
// cell's plants and building; between full draws only animated buildings
// and, while one is hovered, all of them.
func (g *Game) drawColonyBldgs() error {
	cs, cg := &g.cscr, &g.col
	scanned := cs.scannedC != -1 && cs.scannedR != -1
	for n := 0; n < 0x24; n++ {
		c, r := g.bldgBackToFront(n)
		if !cg.fullDraw && !scanned {
			continue
		}
		if err := g.drawVeggieCR(c, r, !scanned); err != nil {
			return err
		}
		id := cs.surf.bldgs[c][r]
		if id == 0 {
			continue
		}
		p, err := g.cacheLoadBldg(c, r, id)
		if err != nil {
			return err
		}
		if p.Frames > 1 || cg.fullDraw || scanned {
			if err := g.drawColonyBldg(c, r); err != nil {
				return err
			}
		}
	}
	return nil
}

// drawColonyBldg is Draw_Colony_Bldg_ (0xbea33): on a full draw the
// picture is played up to its current frame, else the hovered building is
// drawn tinted on alternate clock ticks and the others without glass.
func (g *Game) drawColonyBldg(c, r int) error {
	e, cs, cg := g.e, &g.cscr, &g.col
	if c >= 6 || r >= 6 || c < 0 || r < 0 {
		return nil
	}
	id := cs.surf.bldgs[c][r]
	if id == 0 {
		return nil
	}
	p, err := g.cacheLoadBldg(c, r, id)
	if err != nil {
		return err
	}
	if cg.fullDraw {
		cur := p.CurrentFrame
		p.CurrentFrame = 0
		for p.CurrentFrame < cur {
			if err := e.Draw(0, 0, p); err != nil {
				return err
			}
		}
		return e.Draw(0, 0, p)
	}
	if int16(c) == cs.scannedC && int16(r) == cs.scannedR && g.globalCycler(0x1e, 2) != 0 {
		return g.drawBldgCRDark(c, r)
	}
	return e.DrawNoGlass(0, 0, p)
}

// drawBldgCRDark is Draw_Bldg_CR_Dark_ (0xbe896): the building of cell
// (c, r) tinted.
func (g *Game) drawBldgCRDark(c, r int) error {
	p, err := g.cacheLoadBldg(c, r, g.cscr.surf.bldgs[c][r])
	if err != nil {
		return err
	}
	x := int(g.crToXY(c, r, 0))
	y := int(g.crToXY(c, r, 1))
	bm := gfx.NewBlankBitmap(0x78, 0x78)
	if err := bm.AddPicture(int(int16(-(x - 0x3c))), int(int16(-(y - 0x78))), p); err != nil {
		return err
	}
	bm.MaskOutColors(0xf0, 0xff)
	g.e.TintBitmap(bm, 2)
	g.e.V.SetWindow(0, 0, 0x27f, 0x1df)
	g.e.V.ClipOn()
	err = g.e.Draw(int(int16(x-0x3c)), int(int16(y-0x78)), bm)
	g.e.V.ClipOff()
	return err
}
