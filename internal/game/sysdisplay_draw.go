package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// Drawing the system display box. Spec: docs/screens/system-display.md,
// "Drawing".

// drawSystemDisplayBox is Wrapper_For_Normal_System_Popup_ (0xa4fbe).
func (g *Game) drawSystemDisplayBox() error {
	s := &g.state
	b := &s.MoveableBoxes[0]
	star := int(b.Content)
	title := fmt.Sprintf(g.hMessage(0x16a), save.CString(s.Stars[star].Name[:]))
	if err := g.initSystemDisplay(star); err != nil {
		return err
	}
	return g.drawSystemDisplayPopup(star, true, title, int(b.X), int(b.Y), int(b.DragDX), int(b.DragDY))
}

// drawSystemDisplayPopup is Draw_System_Display_Popup_ (0xa31da) for the
// main screen.
func (g *Game) drawSystemDisplayPopup(star int, border bool, title string, x, y, dx, dy int) error {
	e, s, sd := g.e, &g.state, &g.sd
	// community patch 1.51 (0xa31ed): read unsigned, so the -2 two ship views
	// leave (each a -1) becomes 1535, not the frame -1 1.40b23 drew.
	sd.shipFrame = int16((int(uint16(sd.shipFrame)) + 1) % 32000)
	if g.currentScreen == ScreenMain {
		g.mapWindow()
	}
	sd.baseX, sd.baseY = int16(x), int16(y)
	bx, by := x+dx, y+dy
	pn := int(s.PlayerNum)
	if !sd.lookAtColonies && !g.okToViewSystem(star, pn) {
		st := &s.Stars[star]
		if st.Special == 9 && s.playerHasVisited(pn, star) { // Player_Sighted_Monster_
			return g.drawSightedMonster(star, border, x, y, dx, dy)
		}
		if !border {
			return nil
		}
		if err := g.drawSystemDisplayBorder(bx, by); err != nil {
			return err
		}
		t := g.hMessage(0x16b)
		if s.contactWithOneColony(star) {
			t = fmt.Sprintf(g.hMessage(0x16a), save.CString(st.Name[:]))
		}
		if err := g.printSystemDisplayBoxTitle(x, y, dx, dy, t); err != nil {
			return err
		}
		return g.printEmptySystemData(star, bx, by)
	}
	if border {
		if err := g.drawSystemDisplayBorder(bx, by); err != nil {
			return err
		}
	}
	st := &s.Stars[star]
	sd.autoSelectStar = int16(star)
	if st.WormholeTo >= 0 {
		t := e.Text()
		t.SetStyle(1, g.fontColors2(1, 0x6e, 0x73))
		wx, wy := bx+0x149, by-t.Height()+0xe4
		msg := g.hMessage(0x16d)
		if to := int(st.WormholeTo); s.playerHasVisited(pn, to) {
			msg = fmt.Sprintf(g.hMessage(0x16c), save.CString(s.Stars[to].Name[:]))
		}
		if msg != "" {
			if err := t.PrintRight(wx, wy, msg); err != nil {
				return err
			}
		}
	}
	if sd.sunBeforePlanet {
		if err := g.drawSunSeg(star, bx, by); err != nil {
			return err
		}
	}
	if !sd.skipPlanets {
		if err := g.drawSystemDisplayPlanets(bx, by); err != nil {
			return err
		}
		e.V.ClipOn()
	} else {
		sd.skipPlanets = false
	}
	if !sd.sunBeforePlanet {
		if err := g.drawSunSeg(star, bx, by); err != nil {
			return err
		}
	}
	for i := 0; i < 5; i++ {
		rec := &sd.disp[i]
		if rec.planet < 0 {
			continue
		}
		p := &s.Planets[rec.planet]
		if p.Colony < 0 {
			continue
		}
		mx := bx + g.sysL() + 0xa1 + int(rec.x) - 2
		my := by + g.sysT() + 0x5a + int(rec.y) - 2
		if p.Type == 1 {
			d := int(planetDim(p)) / 2
			mx, my = mx+d, my+d
		}
		c := &s.Colonies[p.Colony]
		banner := int(s.Players[c.Owner&7].Color)
		entry, dxm := 0x23+banner, 2
		if c.IsOutpost != 0 {
			entry, dxm = 0x2b+banner, 0
		}
		pic, err := g.cached(buffer0LBX, entry)
		if err != nil {
			return err
		}
		if err := e.Draw(int(int16(mx-pic.Width+dxm)), int(int16(my)), pic); err != nil {
			return err
		}
	}
	for i := 0; i < 5; i++ {
		rec := &sd.disp[i]
		if rec.planet < 0 || int16(rec.slot) != sd.iconScanned {
			continue
		}
		p := &s.Planets[rec.planet]
		if p.Type != 1 {
			dim := int(planetDim(p))
			if err := g.drawSystemHighlight(int(rec.planet), dim, dim, bx+g.sysL()+0xa1+int(rec.x)-2, by+g.sysT()+0x5a+int(rec.y)-2); err != nil {
				return err
			}
		}
		if err := g.drawPlanetInfoBox(bx, by, int(rec.planet)); err != nil {
			return err
		}
	}
	if err := g.drawSystemShipIcons(bx, by); err != nil {
		return err
	}
	if err := g.drawGateIcons(star, bx, by); err != nil {
		return err
	}
	if sd.iconScanned >= 2000 && sd.iconScanned < 2004 {
		if err := g.printGateInfo(star, int(sd.iconScanned)-2000, bx, by); err != nil {
			return err
		}
	}
	return g.printSystemDisplayBoxTitle(x, y, dx, dy, title)
}

// drawSystemDisplayBorder is Draw_System_Display_Border_: the frame.
func (g *Game) drawSystemDisplayBorder(bx, by int) error {
	p, err := g.cached(buffer0LBX, 0x49)
	if err != nil {
		return err
	}
	return g.e.Draw(bx, by, p)
}

// drawSunSeg is Draw_Sun_Seg_ (0xa3025); it centres with width and height
// swapped, as the original does, except on the leaders screen.
func (g *Game) drawSunSeg(star, bx, by int) error {
	b := &g.state.MoveableBoxes[0]
	p, err := g.cached(buffer0LBX, 0x53+int(g.state.Stars[star].SpectralClass))
	if err != nil {
		return err
	}
	if g.currentScreen == ScreenLeaders {
		return g.e.Draw(0x1c0, 0x5c, p)
	}
	y := int(int16((int(b.Height)-p.Width)/2 + by))
	x := int(int16((int(b.Width)-p.Height)/2 + bx))
	return g.e.Draw(x, y, p)
}

// drawSystemDisplayPlanets is Draw_System_Display_Planets_ (0xa30fd).
func (g *Game) drawSystemDisplayPlanets(bx, by int) error {
	s, sd := &g.state, &g.sd
	for i := 0; i < 5; i++ {
		rec := &sd.disp[i]
		if rec.planet < 0 {
			continue
		}
		switch s.Planets[rec.planet].Type {
		case 2, 3:
			if err := g.drawPlanetWithPiercedRing(int(rec.planet), i, bx, by); err != nil {
				return err
			}
		case 1:
			p, err := g.cached(buffer0LBX, 0x5b)
			if err != nil {
				return err
			}
			setAnimationFrame(p, int(rec.slot))
			if err := g.e.Draw(bx+g.sysL()+0x10, by+g.sysT()+0xc, p); err != nil {
				return err
			}
		}
	}
	return nil
}

// drawPlanetWithPiercedRing is Draw_Planet_With_Pierced_Ring_ (0xa2d7c):
// the orbit ring is cut where the outlined planet covers it, then the
// planet is drawn through the remap table with its light mask.
func (g *Game) drawPlanetWithPiercedRing(pl, i, bx, by int) error {
	e, s, sd := g.e, &g.state, &g.sd
	rec := &sd.disp[i]
	p := &s.Planets[pl]
	sz := 4
	if p.Type == 3 {
		sz = int(p.Size)
	}
	x0, y0 := int(int16(bx+g.sysL()+0x10)), int(int16(by+g.sysT()+0xc))
	ringPic, err := g.cached(buffer0LBX, 0x4d+int(rec.slot))
	if err != nil {
		return err
	}
	ring := gfx.NewBlankBitmap(1, 1)
	ring.DrawPictureToBitmap(ringPic)
	px := int(int16(bx + g.sysL() + 0x9f + int(rec.x)))
	py := int(int16(by + g.sysT() + 0x58 + int(rec.y)))
	mask, err := g.cached(buffer0LBX, 0x8f+sz)
	if err != nil {
		return err
	}
	setAnimationFrame(mask, int(rec.pos))
	if rec.pict, err = g.rotatingPlanet(pl); err != nil {
		return err
	}
	if err := e.CalculatePictureRemapColors(rec.pict); err != nil {
		return err
	}
	pb := gfx.NewBlankBitmap(0x23, 0x23)
	if err := pb.AddPicture(2, 2, rec.pict); err != nil {
		return err
	}
	pb.Outline(pb, 0x1f, false)
	pb.Outline(pb, 0x1e, false)
	ring.MaskOut(int(int16(px-x0-2)), int(int16(py-y0-2)), pb, false)
	if err := e.Draw(x0, y0, ring); err != nil {
		return err
	}
	setAnimationFrame(mask, int(rec.pos))
	setAnimationFrame(rec.pict, int(sd.shipFrame)%rec.pict.Frames)
	return e.RemapDrawWithMask(px, py, rec.pict, mask, e.AnimationHeader())
}

// drawSystemHighlight is Draw_System_Highlight_ (0xa3a15): the corner
// brackets around the hovered planet, in the owner's colours.
func (g *Game) drawSystemHighlight(pl, dimx, dimy, hx, hy int) error {
	s := &g.state
	p, err := g.cached(buffer0LBX, 0x4b)
	if err != nil {
		return err
	}
	x := int(int16(hx - (p.Width-dimx)/2))
	y := int(int16(hy - (p.Height-dimy)/2))
	col := int(s.Planets[pl].Colony)
	base := -1
	if col < 0 {
		base = 6
	} else if banner := int(s.Players[s.Colonies[col].Owner&7].Color); banner != 0 {
		base = int(mainPalettePlayerColors[banner])
	}
	if base < 0 {
		return g.e.Draw(x, y, p)
	}
	for k := 0; k < 9; k++ {
		if err := p.ReplaceColor(0x49+k, base+k); err != nil {
			return err
		}
	}
	err = g.e.Draw(x, y, p)
	for k := 0; k < 9; k++ {
		if rerr := p.ReplaceColor(base+k, 0x49+k); rerr != nil && err == nil {
			err = rerr
		}
	}
	return err
}

// drawSystemShipIcons is Draw_System_Ship_Icons_ (0xa3bc4).
func (g *Game) drawSystemShipIcons(bx, by int) error {
	sd := &g.sd
	for o := range sd.ships {
		sh := &sd.ships[o]
		if sh.node < 0 {
			continue
		}
		pic, err := g.systemShipPicture(o)
		if err != nil || pic == nil {
			continue
		}
		if sd.iconScanned-1000 == sh.node {
			if err := g.drawShipInfoBox(bx, by, sh.node); err != nil {
				return err
			}
		} else {
			pic.CurrentFrame = 0
		}
		if err := g.e.Draw(bx+int(sh.x), by+int(sh.y), pic); err != nil {
			return err
		}
	}
	return nil
}

// drawGateIcons is Draw_Gate_Icons_ (0xa2c66).
func (g *Game) drawGateIcons(star, bx, by int) error {
	s := &g.state
	st, p := &s.Stars[star], int(s.PlayerNum)
	x, y := bx+0xe9, by+0xf3
	if st.ArtemisNet>>p&1 != 0 {
		x -= 12
		pic, err := g.cached(buffer0LBX, 0x21)
		if err != nil {
			return err
		}
		if err := g.e.Draw(x, y, pic); err != nil {
			return err
		}
	}
	if st.DimensionalPortal>>p&1 != 0 {
		pic, err := g.cached(buffer0LBX, 0x15)
		if err != nil {
			return err
		}
		if err := g.e.Draw(x-12, y+1, pic); err != nil {
			return err
		}
	}
	return nil
}

// printSystemDisplayBoxTitle is Print_System_Display_Box_Title_ (0xa4c41).
func (g *Game) printSystemDisplayBoxTitle(x, y, dx, dy int, s string) error {
	t := g.e.Text()
	c := g.fontColors2(4, 0x3c, 0x41)
	t.SetStyle(4, c)
	w := t.Width([]byte(s))
	if w > 0x109 {
		t.SetStyle(3, c)
	}
	if w < 0x3c {
		t.SetSpacing(4)
	} else {
		t.SetSpacing(2)
	}
	st, sp := t.Style, t.Spacing
	h := t.Height()
	frame, err := g.cached(buffer0LBX, 0x49)
	if err != nil {
		return err
	}
	cx := frame.Width/2 + dx + x
	cy := (0x14-(h+1))/2 + y + dy + 0x11
	g.setFittedFontStyle(s, c, st, sp, font.ShadowNone, 0x131, 1)
	st2, sp2 := t.Style, t.Spacing
	c = g.fontColors2(st2, 0x3c, 0x41)
	t.SetStyle(st2, c)
	t.SetSpacing(sp2)
	if st2 < 3 {
		cy += 3 - st2
	}
	return g.printCenteredShadowDown(int(int16(cx)), int(int16(cy)), 0x3d, s)
}

// printEmptySystemData is Print_Empty_System_Data_ (0xa573b): the star's
// description over the unexplored box.
func (g *Game) printEmptySystemData(star, bx, by int) error {
	b := &g.state.MoveableBoxes[0]
	x0, y0 := bx+g.sysL(), by+g.sysT()
	mw := int(b.Width) - 0x33
	aw, ah := int(b.Width)-0x1a, int(b.Height)-0x5b
	ws := g.saveWindowStatus()
	g.mapWindow()
	g.changeFontStyle(3)
	pw, err := g.e.Text().ParagraphMaxWidth(mw, g.sd.description, 3)
	if err != nil {
		g.restoreWindowStatus(ws)
		return err
	}
	err = g.printParagraphCenteredVertically((aw-pw)/2+x0, y0, mw, ah, g.sd.description, 3)
	g.restoreWindowStatus(ws)
	return err
}

// logOnce logs a message the first time it is seen.
func (g *Game) logOnce(msg string) {
	if g.logged == nil {
		g.logged = map[string]bool{}
	}
	if !g.logged[msg] {
		g.logged[msg] = true
		g.Log("%s", msg)
	}
}

// drawSightedMonster is Draw_System_Display_Popup_'s branch for a star whose
// guardian the player has seen (0xa3284): the box hatched over, the monster
// in Draw_System_Special_Popup_ and the star's name as the title.
func (g *Game) drawSightedMonster(star int, border bool, x, y, dx, dy int) error {
	g.hideSystemDisplay(x, y, dx, dy, 0xd, 0x2f, true)
	if border {
		if err := g.drawSystemDisplayBorder(x+dx, y+dy); err != nil {
			return err
		}
	}
	px, py := x+dx, y+dy+0x2f
	if g.currentScreen == ScreenLeaders {
		v := g.e.V // Save_Window_ / Restore_Window_ keep the window
		w := [4]int{v.WinX1, v.WinY1, v.WinX2, v.WinY2}
		v.SetWindow(0, 0, 0x27f, 0x1df)
		v.ClipOn()
		err := g.drawSystemSpecialPopup(px-5, py+0x28, 9, star)
		v.SetWindow(w[0], w[1], w[2], w[3])
		v.ClipOff()
		return err
	}
	if err := g.drawSystemSpecialPopup(px+5, py, 9, star); err != nil {
		return err
	}
	title := cSprintf(g.hMessage(0x16a), save.CString(g.state.Stars[star].Name[:]))
	return g.printSystemDisplayBoxTitle(x, y, dx, dy, title)
}

// hideSystemDisplay is Hide_System_Display_ (0xa3f21): with hide set the
// box's inside (fixed on the leaders screen) is hatched; font style 4
// takes Font_Colors2_(4, 0x6e, 0x73).
func (g *Game) hideSystemDisplay(x, y, dx, dy, ox, oy int, hide bool) {
	x1, y1, x2, y2 := 0x132, 0x11, 0x270, 0xc5
	if g.currentScreen != ScreenLeaders {
		bx, by := x+dx+ox, y+dy+oy
		x1, y1, x2, y2 = bx-1, by-1, bx+0x13f, by+0xb6
	}
	c := g.fontColors2(4, 0x6e, 0x73)
	if !hide {
		return
	}
	g.diagonalLineFill(x1, y1, x2-5, int(int16(min(g.e.V.WinY2, y2))), 0x10, 0x3d, 0x3b)
	g.e.Text().SetStyle(4, c)
}

// drawSystemSpecialPopup is Draw_System_Special_Popup_ (0xc6ff3): the
// special's description, picture and name in Draw_Special_Info_Popup_.
func (g *Game) drawSystemSpecialPopup(x, y int, special int8, star int) error {
	desc, err := g.systemSpecialDesc(special)
	if err != nil {
		return err
	}
	anim, err := g.systemSpecialAnim(star, special)
	if err != nil {
		return err
	}
	name, err := g.eStringAt(planetSpecialNames + 4*uint32(special))
	if err != nil {
		return err
	}
	return g.drawSpecialInfoPopup(x, y, anim, name, desc)
}
