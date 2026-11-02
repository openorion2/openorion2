package game

import (
	"fmt"
	"strings"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/save"
)

// Drawing of the fleet screen (FLT1.C, FLT2.C) and the galaxy map pieces
// it shares with the leaders screen. Spec: docs/screens/fleet-screen.md.

// FLT1.C colour tables in the image.
const (
	fltNormalColors  = 0x182078 // the module's _normal_colors
	fltRedColors     = 0x182080
	fltNeutralColors = 0x182088
	fltGreenLine     = 0x182135 // _ship_direction_line_green_colors
	fltRedLine       = 0x18213d
	fltRelocLine     = 0x1823ec // unused; the relocation lines use local colours
	captainBoxIdle   = 0x753d6  // Draw_Captains_Picture_'s box colours, read as data
	captainBoxMoving = 0x753da
	shipNameFormat   = 0x179300 // "%s"
	crewFormat       = 0x179314 // "%s (%d EP)"
	plusString       = 0x17931f // "+"
	signedFormat     = 0x179321 // "%s%d"
	weaponFormat     = 0x179326 // "%d %s (%s)"
	starLeaderFormat = 0x17930c // "%s (%s)"
	arcEmpty         = 0x1791eb
	arcAll           = 0x17925f
	hullCountFormat  = 0x179b03 // "%d %s, "
)

// fltColors reads one of FLT1.C's colour sets.
func (g *Game) fltColors(addr uint32) font.Colors {
	var c font.Colors
	copy(c[:], g.exe.bytes(addr, 8))
	return c
}

// setFontColors is Set_Font_Colors_ (0x8fe0a): the current and normal
// colours, the style kept.
func (g *Game) setFontColors(c font.Colors) {
	t := g.e.Text()
	t.Current, t.Normal = c, c
}

// drawFleetScreen is Draw_Fleet_Screen_ (0x72f92), the auto function.
func (g *Game) drawFleetScreen() error {
	e, f, m := g.e, &g.flt, &g.ms
	v := e.V
	if m.drawFleetScreenToBack || g.fullDrawScreen {
		v.ResetWindow()
		v.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := e.Draw(0, 0, f.background); err != nil {
			return err
		}
		if err := g.drawFltscrnSmallShipIcons(); err != nil {
			return err
		}
		v.CopyOffToBack()
		m.drawFleetScreenToBack = false
		g.fullDrawScreen = false
	}
	if m.galaxyMapScannedStar > -1 {
		if err := g.printFltscrnScannedStarName(); err != nil {
			return err
		}
	}
	if f.scannedBig > -1 && f.scannedBig < 0xfa && g.currentScreen == 4 {
		v.SetWindow(0xf, 0x11a, 0x140, 0x1d1)
		v.ClipOn()
		if err := g.printScannedShipData(int(f.big[f.scannedBig].ship)); err != nil {
			return err
		}
		v.ClipOff()
		v.ResetWindow()
	}
	g.drawFltscrnRelocationLines(4)
	g.drawFltscrnShipDestinationLines(4)
	if err := g.drawGalaxyMapBox(nil, 0xf, 0x34, 0x131, 0xb6, 0, 0, 0, 0, 1, -1, -1); err != nil {
		return err
	}
	if f.relocateMode != 1 && e.MouseButton() == 1 {
		k := 0
		for i := int16(f.cols) * f.bar.first; i < f.nBig; i++ {
			b := &f.big[i]
			if e.AutoInput() == b.field {
				if f.selPaint == -1 {
					f.selPaint = int8(b2u8(!b.sel))
				}
				b.sel = f.selPaint != 0
				if !b.sel {
					m.drawToBack = true
				}
				if err := g.drawBoxAroundScannedShip(i); err != nil {
					return err
				}
			}
			k++
			if k >= int(f.rows)*int(f.cols) {
				break
			}
		}
	}
	if f.scannedSmall > -1 {
		if err := g.printGalmapScannedShip(int(f.scannedSmall)); err != nil {
			return err
		}
	}
	if f.smallPtr > -1 {
		if err := g.drawFlashingSmallShipIcon(); err != nil {
			return err
		}
	}
	if f.nBig > 0 {
		changed, err := g.drawGenericVerticalScrollBar(&f.bar, g.fillFltscrnScrollBar)
		if err != nil {
			return err
		}
		if changed {
			f.scannedBig = -1
		}
	}
	if f.stackOwner < 0 || f.skipDrawShips || f.smallPtr < 0 {
		f.skipDrawShips = false
	} else if err := g.drawFltscrnBigShipIcons(4); err != nil {
		return err
	}
	pic := f.dimRelocate
	switch f.relocateMode {
	case 0:
		setAnimationFrame(f.relocateBtn, 0)
		pic = f.relocateBtn
	case 1:
		setAnimationFrame(f.relocateBtn, 1)
		pic = f.relocateBtn
	}
	if err := e.Draw(0x1b9, 0x17c, pic); err != nil {
		return err
	}
	if !f.addScrap {
		if err := e.Draw(0x225, 0x17c, f.dimScrap); err != nil {
			return err
		}
	}
	if !f.addAll {
		if err := e.Draw(0x15c, 0x17c, f.dimAll); err != nil {
			return err
		}
	}
	if !f.addOfficers {
		if err := e.Draw(0x156, 0x1ae, f.dimOff); err != nil {
			return err
		}
	}
	m.multiColoredLineStart = (m.multiColoredLineStart + 1) % 8
	return nil
}

// drawFltscrnSmallShipIcons is Draw_Fltscrn_Small_Ship_Icons_ (0x737a2):
// the ship icons on the galaxy map, last first.
func (g *Game) drawFltscrnSmallShipIcons() error {
	st := &g.stacks
	for i := int(st.iconCount) - 1; i >= 0; i-- {
		p := g.smallIconPicture(i, false)
		setAnimationFrame(p, 0)
		if err := g.e.Draw(int(st.icon[i].x), int(st.icon[i].y), p); err != nil {
			return err
		}
	}
	return nil
}

// drawFlashingSmallShipIcon is Draw_Flashing_Small_Ship_Icon_ (0x72f19):
// the shown stack's icon in its animated version.
func (g *Game) drawFlashingSmallShipIcon() error {
	f, s, st := &g.flt, &g.state, &g.stacks
	node := st.start[f.smallPtr]
	owner := g.shipOwner(int(st.node[node].ship))
	p := f.flashMonster[(owner-8)&7%7]
	if owner < 8 {
		p = f.flashIcon[s.Players[owner].Color]
	}
	i := g.fShipIconFromNodePtr(node)
	if i == -1 {
		return nil
	}
	return g.e.Draw(int(st.icon[i].x), int(st.icon[i].y), p)
}

// drawFltscrnBigShipIcons is Draw_Fltscrn_Big_Ship_Icons_ (0x732d6): the
// shown rows of big icons with selection frames and damage bars.
func (g *Game) drawFltscrnBigShipIcons(screen int) error {
	e, f := g.e, &g.flt
	barW, off := 0x38, 1
	if screen == 0x1d {
		barW, off = 0x39, 3
	}
	k := int16(0)
	for i := f.bar.first * int16(f.cols); i < f.nBig; i++ {
		b := &f.big[i]
		p, err := g.shipPicture(int(b.ship))
		if err != nil {
			return err
		}
		x, y := g.fltscrnBigIconXY(k)
		dx := (0x39 - p.Width) / 2
		dy := (0x39 - p.Height) / 2
		g.shipOwner(int(b.ship))
		if i == f.scannedBig && g.currentScreen == 4 {
			if err := g.drawBoxAroundScannedShip(i); err != nil {
				return err
			}
		}
		if b.sel {
			sx, sy := x-1, y-1
			if g.currentScreen == 4 {
				sx, sy = x-2, y+1
			}
			if err := e.Draw(int(sx), int(sy), f.selectedBox); err != nil {
				return err
			}
		}
		if err := e.Draw(int(x)+dx+1, int(y)+dy+2, p); err != nil {
			return err
		}
		if g.isCombatShip(int(b.ship)) {
			g.drawDamageBars(int(x), int(y), barW, 0x39, off, int(b.ship))
		}
		k++
		if int(k) >= int(f.cols)*int(f.rows) {
			break
		}
	}
	return nil
}

// drawBoxAroundScannedShip is Draw_Box_Around_Scanned_Ship_ (0x734a9).
func (g *Game) drawBoxAroundScannedShip(i int16) error {
	b := &g.flt.big[i]
	x, y := b.x-2, b.y-2
	if g.currentScreen == 4 {
		x, y = b.x-3, b.y-1
	}
	return g.e.Draw(int(x), int(y), g.flt.scannedBox)
}

// fillFltscrnScrollBar is Fill_FltScrn_Scroll_Bar_ (0x73873): the thumb as
// shaded vertical lines, then the scroll track picture over it.
func (g *Game) fillFltscrnScrollBar() error {
	f := &g.flt
	v := g.e.V
	b := &f.bar
	x, y, w, c, tx := 0x25e, 0x56, 0xc, byte(0xa6), 0x25d
	if g.currentScreen != 4 {
		x, y, w, c, tx = 0x265, 0x2d, 10, 0xaf, 0x264
	}
	top := y + int(b.track-(b.h/2+b.pos))
	v.SetWindow(x, y, w+x, int(b.track)+y)
	v.ClipOn()
	for i := 0; i < w; i++ {
		v.Line(x, top+1, x, top+int(b.h), c)
		if i%2 == 0 {
			c--
		}
		x++
	}
	v.ClipOff()
	v.ResetWindow()
	return g.e.Draw(tx, y, f.scrollThumb)
}

// fltscrnMap is the galaxy map rectangle of a screen: the fleet screen's
// or the leaders screen's.
func fltscrnMap(screen int) (x, y, w, h int16) {
	if screen == 4 {
		return 0xf, 0x34, 0x131, 0xb6
	}
	return 0x132, 0xeb, 0x13e, 0xa9
}

// drawFltscrnRelocationLines is Draw_Fltscrn_Relocation_Lines_ (0x7527a).
func (g *Game) drawFltscrnRelocationLines(screen int) {
	s := &g.state
	colors := [8]byte{6, 7, 7, 8, 8, 9, 9, 0xa}
	x, y, w, h := fltscrnMap(screen)
	p := int(s.PlayerNum)
	for i := 0; i < int(s.NumStars); i++ {
		d := s.Stars[i].Relocation[p]
		if d == -1 {
			continue
		}
		x1, y1 := g.galaxyMapStarXY(i, x, y, w, h, 0, 0, true)
		x2, y2 := g.galaxyMapStarXY(int(d), x, y, w, h, 0, 0, true)
		g.e.V.DirectionalMultiColoredLine(int(int16(x2+3)), int(int16(y2+3)), int(x1), int(y1), colors, int(g.ms.multiColoredLineStart))
	}
}

// drawFltscrnShipDestinationLines is Draw_Fltscrn_Ship_Destination_Lines_
// (0x734f1): on the fleet screen a line from the shown stack to the hovered
// star (green if the move is possible), then every own stack in flight
// (green) and every enemy stack heading for one of our colonies (red).
func (g *Game) drawFltscrnShipDestinationLines(screen int) {
	f, s, st, m := &g.flt, &g.state, &g.stacks, &g.ms
	green, red := g.fltColors(fltGreenLine), g.fltColors(fltRedLine)
	x, y, w, h := fltscrnMap(screen)
	icon := -1
	if screen == 4 && f.smallPtr > -1 && g.firstSelectedShip() != -1 && m.galaxyMapScannedStar > -1 && m.galaxyMapScannedStar < s.NumStars {
		icon = g.fShipIconFromNodePtr(st.start[f.smallPtr])
	}
	if icon > -1 {
		c := red
		if g.fb.moveInfo.canMove && s.Stars[m.galaxyMapScannedStar].SpectralClass != save.SpectralBlackHole {
			c = green
		}
		sx, sy := g.galaxyMapStarXY(int(m.galaxyMapScannedStar), x, y, w, h, 0, 0, true)
		g.drawShipDestinationLine(sx+3, sy+3, st.icon[icon].x, st.icon[icon].y, c)
	}
	for i := 0; i < int(st.iconCount); i++ {
		ship := g.iconShip(i)
		owner := g.shipOwner(ship)
		sh := &s.Ships[ship]
		var c [8]byte
		draw := false
		if owner == int(s.PlayerNum) && absoluteLocation(sh.Location) != s.NumStars && sh.Location >= 500 && sh.Location < 1500 {
			c, draw = green, true
		} else if owner != int(s.PlayerNum) && g.enemyHeadingTowardOurColony(st.icon[i].node) {
			c, draw = red, true
		}
		if !draw {
			continue
		}
		d := actualStarID(sh.Location)
		sx, sy := g.galaxyMapStarXY(int(d), x, y, w, h, 0, 0, true)
		g.drawShipDestinationLine(sx+3, sy+3, st.icon[i].x, st.icon[i].y, c)
	}
}

// doFleetScreenStuff is Do_Fleet_Screen_Stuff_ (0x909af), reached from
// Draw_Galaxy_Map_Box_ only while a star is hovered: frames around that star
// and around a hovered ship icon of another stack.
func (g *Game) doFleetScreenStuff() {
	f, s, m := &g.flt, &g.state, &g.ms
	if m.galaxyMapScannedStar > -1 {
		st := &s.Stars[m.galaxyMapScannedStar]
		ms := int32(s.MaxMapScale)
		x := (int32(st.X)*1000/ms)*10/0x67b + 0xf
		y := (int32(st.Y)*1000/ms)*10/0x895 + 0x34
		g.box(int(int16(x-5)), int(int16(y-5)), 0xb, 0xc, 1, 0x71)
	}
	if f.scannedSmall > -1 {
		stk := int16(g.shipStackFromShipIcon(int(f.scannedSmall)))
		if stk > -1 && stk != f.smallPtr {
			ic := &g.stacks.icon[f.scannedSmall]
			g.box(int(ic.x+4), int(ic.y+2), 0xb, 0xb, 1, 0x8d)
		}
	}
}

// printFltscrnScannedStarName is Print_Fltscrn_Scanned_Star_Name_ (0x761a5):
// the hovered star's name or what a move order there would do, centred in
// the bar under the map.
func (g *Game) printFltscrnScannedStarName() error {
	s, m := &g.state, &g.ms
	t := g.e.Text()
	star := int(m.galaxyMapScannedStar)
	p := int(s.PlayerNum)
	visited := s.playerHasVisited(p, star)
	colors := g.fltColors(fltNormalColors)
	owner := int16(-1)
	kind, a, b := -1, 0, 0
	ids, listed, err := g.buildFltscrnShipList()
	if err != nil {
		return err
	}
	mi := &g.fb.moveInfo
	if listed {
		*mi = g.shipsTryToMoveTo(ids, star)
	} else {
		*mi = shipMoveInfo{}
		if !visited && s.Players[p].Picks[save.PickOmniscient] == 0 && !g.oneLeaderWithGalacticLore(p) && !s.contactWithOneColony(star) {
			kind = 8
		} else {
			kind, a = 7, star
		}
	}
	switch {
	case mi.blocked:
		kind = 3
	case mi.noFTL:
		kind = 4
	case mi.outOfRange:
		a = int(mi.parsecsShort)
		kind = 6
		if visited {
			kind, b = 5, star
		}
	case mi.flux:
		kind = 9
	case mi.canMove:
		if s.Stars[star].SpectralClass == save.SpectralBlackHole {
			kind = 3
			break
		}
		owner = int16(s.Stars[star].Owner)
		switch {
		case mi.turns == 0:
			kind, a = 0, star
		case visited:
			kind, a, b = 1, int(uint8(mi.turns)), star
		default:
			kind, a = 2, int(uint8(mi.turns))
		}
	}
	name := func(i int) string { return save.CString(s.Stars[i].Name[:]) }
	var msg string
	neutral := g.fltColors(fltNeutralColors)
	switch kind {
	case 0:
		msg = fmt.Sprintf(cFormat(g.hMessage(0x67)), name(a))
	case 1:
		id := 0x6a
		if a == 1 {
			id = 0x69
		}
		msg = fmt.Sprintf(cFormat(g.hMessage(id)), a, name(b))
	case 2:
		id := 0x6c
		if a == 1 {
			id = 0x6b
		}
		msg = fmt.Sprintf(cFormat(g.hMessage(id)), a)
	case 3, 4, 8, 9:
		msg = g.hMessage(map[int]int{3: 0x21, 4: 0x6e, 8: 0x94, 9: 0x73}[kind])
		colors = neutral
	case 5:
		id := 0x70
		if a == 1 {
			id = 0x6f
		}
		msg = fmt.Sprintf(cFormat(g.hMessage(id)), a, name(b))
		colors = neutral
	case 6:
		msg = fmt.Sprintf(cFormat(g.hMessage(0xeb)), a)
		colors = neutral
	case 7:
		owner = int16(s.Stars[star].Owner)
		if owner < 0 || owner >= 8 {
			colors, owner = neutral, -1
		}
		msg = g.starWithGovernor(a)
	default:
		g.logOnce("Print_Fltscrn_Scanned_Star_Name_ prints an uninitialised buffer")
	}
	if kind < 0 || kind > 9 {
		return nil
	}
	g.setFittedFontStyle(msg, colors, 3, 2, 0, 0xbd, 1)
	if owner != -1 && !listed {
		c := int(mainPalettePlayerColors[s.Players[owner].Color])
		if c == 0x20 {
			c = 0xf
		}
		g.setFontColors(g.fontColors2(t.Style, byte(c), byte(c+6)))
	}
	w := t.Width([]byte(msg))
	return t.Print(0xa9-w/2, (0x17-t.Height())/2+0xf8, msg)
}

// starWithGovernor is the star name Print_Fltscrn_Scanned_Star_Name_ and
// Do_Officer_Screen_Stuff_ print: with the player's governor, and his
// ETA while he travels there, when the player has a colony there.
func (g *Game) starWithGovernor(star int) string {
	s := &g.state
	p := int(s.PlayerNum)
	st := &s.Stars[star]
	name := save.CString(st.Name[:])
	gov := int(st.Officer[p])
	if gov == -1 || !s.playerHasColony(p, star) {
		return name
	}
	l := &s.Leaders[gov]
	ln := save.CString(l.Name[:])
	switch {
	case l.ETA <= 0:
		return fmt.Sprintf(cFormat(cStringAt(g.exe, starLeaderFormat)), name, ln)
	case l.ETA == 1:
		return fmt.Sprintf(cFormat(g.hMessage(0x92)), name, ln, int(l.ETA))
	}
	return fmt.Sprintf(cFormat(g.hMessage(0x93)), name, ln, int(l.ETA))
}

// oneLeaderWithGalacticLore is One_Leader_With_Galactic_Lore_ (0x94877).
func (g *Game) oneLeaderWithGalacticLore(p int) bool {
	return g.state.Players[p].GalacticLore != 0
}

// weaponArcString is Weapon_Arc_String_ (0x6eff8).
func (g *Game) weaponArcString(arc uint8) (string, error) {
	s := cStringAt(g.exe, arcEmpty)
	id := 0
	switch {
	case arc&1 != 0:
		id = 3
	case arc&2 != 0:
		id = 4
	case arc&4 != 0:
		id = 6
	case arc&8 != 0:
		id = 5
	case arc&0x10 != 0:
		return cStringAt(g.exe, arcAll), nil
	default:
		return s, nil
	}
	k, err := g.kenText(id)
	if err != nil {
		return "", err
	}
	return s + k[:min(len(k), 6)], nil
}

// printScannedShipData is Print_Scanned_Ship_Data_ (0x7670e): the hovered
// big icon's ship in the panel under the map; support ships get their help
// text instead.
func (g *Game) printScannedShipData(ship int) error {
	s := &g.state
	t := g.e.Text()
	sh := &s.Ships[ship]
	typ := sh.Design.Type
	defense, _, attack := g.getShipCombatBonuses(ship)
	normal := g.fltColors(fltNormalColors)
	if typ == 2 || typ == 1 || typ == 4 {
		entry := 0
		switch typ {
		case 1:
			entry = 0x29
		case 2:
			entry = 0xbd
		case 4:
			entry = 0x6d
		}
		rec, err := g.helpRecord(entry)
		if err != nil {
			return err
		}
		g.moxFontColorsOffset = 0x72
		t.SetStyle(3, g.fontColors2(3, 0x6f, 0x74))
		return g.printParagraphCenteredVertically(0xf+3, 0x11a, 299, 0xb7, save.CString(rec.text), 0)
	}
	captain, err := g.drawCaptainsPicture(ship)
	if err != nil {
		return err
	}
	x, y0 := 0x12, 0x11f
	t.SetStyle(3, normal)
	if err := t.Print(x, y0, save.CString(sh.Design.Name[:])); err != nil {
		return err
	}
	t.SetStyle(2, normal)
	dy := t.Height()
	t.SetStyle(2, normal)
	t.SetSpacing(1)
	line := fmt.Sprintf(cFormat(cStringAt(g.exe, crewFormat)), g.crewDescriptionString(ship), int(sh.Experience))
	dy += 2
	if err := t.Print(x, y0+dy, line); err != nil {
		return err
	}
	dy += t.Height() + 2
	shield, err := g.techName(walksShields + 1 + int(sh.Design.Shield))
	if err != nil {
		return err
	}
	if err := t.Print(x, y0+dy, shield); err != nil {
		return err
	}
	dy += t.Height() + 2
	if err := t.Print(x, y0+dy, g.hMessage(0x99)); err != nil {
		return err
	}
	signed := func(v int16) string {
		sg := ""
		if v >= 0 {
			sg = cStringAt(g.exe, plusString)
		}
		return fmt.Sprintf(cFormat(cStringAt(g.exe, signedFormat)), sg, int(v))
	}
	if err := t.PrintRight(x+0x73, y0+dy, signed(attack)); err != nil {
		return err
	}
	label := g.hMessage(0x9a)
	w := t.Width([]byte(label))
	if captain {
		dy += t.Height() + 2
		if err := t.Print(x, y0+dy, label); err != nil {
			return err
		}
		w = max(w, 0x73)
		if err := t.Print(w+x, y0+dy, signed(defense)); err != nil {
			return err
		}
	} else {
		if w+0x9b < 0x10e {
			w = 0x73
		}
		if err := t.Print(x+0x9b, y0+dy, label); err != nil {
			return err
		}
		if err := t.Print(x+0x9b+w, y0+dy, signed(defense)); err != nil {
			return err
		}
	}
	dy += t.Height() + 2
	if sh.Location >= 500 && int(sh.Location) <= int(s.NumStars)+1000 {
		dest := int(absoluteLocation(sh.Location))
		p := int(s.PlayerNum)
		known := s.playerHasVisited(p, dest) || s.Players[p].Picks[save.PickOmniscient] != 0 || g.oneLeaderWithGalacticLore(p) || s.contactWithOneColony(dest)
		var c font.Colors
		for i := range c {
			c[i] = normal[i] + 3
		}
		t.SetStyle(2, c)
		var msg string
		switch {
		case dest == int(s.NumStars):
			msg = g.hMessage(0x68)
		case known:
			msg = fmt.Sprintf(cFormat(g.hMessage(0x9b)), save.CString(s.Stars[dest].Name[:]))
		default:
			msg = g.hMessage(0x9c)
		}
		if err := t.Print(x, y0+dy, msg); err != nil {
			return err
		}
		t.SetStyle(2, normal)
	}
	dy += t.Height() + 2
	if err := t.Print(x, y0+dy, g.hMessage(0x9d)); err != nil {
		return err
	}
	if err := t.Print(x+0x9b, y0+dy, g.hMessage(0x9e)); err != nil {
		return err
	}
	dy += t.Height() + 1
	t.SetStyle(1, normal)
	t.SetSpacing(1)
	wy, n := dy, 0
	for k := 0; k < 8; k++ {
		ws := &sh.Design.Weapons[k]
		if ws.Weapon <= -1 || ws.Count == 0 {
			break
		}
		arc, err := g.weaponArcString(ws.Arc)
		if err != nil {
			return err
		}
		walk := walksWeaponsPl
		if ws.Count == 1 {
			walk = walksWeapons
		}
		name, err := g.techName(walk + 1 + int(ws.Weapon))
		if err != nil {
			return err
		}
		line := fmt.Sprintf(cFormat(cStringAt(g.exe, weaponFormat)), int(ws.Count), name, arc)
		if err := t.Print(x+5, y0+wy, line); err != nil {
			return err
		}
		wy += t.Height() + 1
		n++
	}
	if n == 0 {
		if err := t.Print(x+5, wy+y0, g.hMessage(0x9f)); err != nil {
			return err
		}
	}
	sy, n := dy, 0
	for bit := 0; bit < 0x27; bit++ {
		if !testBit(sh.Design.Specials[:], bit) {
			continue
		}
		c := normal
		if testBit(sh.DamagedSpecials[:], bit) {
			c = g.fltColors(fltRedColors)
		}
		t.SetStyle(1, c)
		t.SetSpacing(1)
		name, err := g.techName(walksSpecials + 1 + bit)
		if err != nil {
			return err
		}
		if err := t.Print(x+0xaa, y0+sy, name); err != nil {
			return err
		}
		sy += t.Height() + 1
		n++
	}
	if n == 0 {
		return t.Print(x+0xaa, sy+y0, g.hMessage(0x9f))
	}
	return nil
}

// crewDescriptionString is Crew_Description_String_ (0x753f2).
func (g *Game) crewDescriptionString(ship int) string {
	if l := g.state.Ships[ship].CrewLevel; l <= 4 {
		return g.hMessage(0x8a + int(l))
	}
	return g.ub.tempString
}

// drawCaptainsPicture is Draw_Captains_Picture_ (0x759d9): the officer
// aboard the ship, if the player's, with his name left of the portrait and
// a frame that shows whether he is still on his way.
func (g *Game) drawCaptainsPicture(ship int) (bool, error) {
	f, s := &g.flt, &g.state
	k, id := 0, -1
	for i := 0; i < maxLeaders; i++ {
		l := &s.Leaders[i]
		if int(l.Owner) != int(s.PlayerNum) || l.Status != 1 || l.Type != 0 {
			continue
		}
		if int(l.Location) == ship {
			id = i
			break
		}
		k++
	}
	if id == -1 {
		return false, nil
	}
	if k >= len(f.officerPics) {
		g.logOnce("Draw_Captains_Picture_ reads past the officer pictures")
		return true, nil
	}
	p := f.officerPics[k]
	x, y := 0xf2, 0x11c
	t := g.e.Text()
	t.SetStyle(0, g.fltColors(fltNormalColors))
	name := fmt.Sprintf("%s", g.leaderName(id, true))
	if err := g.printParagraphCenteredVertically(x-0x42-3, y+3, 0x42, int(uint16(p.Height)), name, 2); err != nil {
		return false, err
	}
	if err := g.e.Draw(x, y, p); err != nil {
		return false, err
	}
	var c [4]byte
	if s.Leaders[id].ETA > 0 {
		if err := g.printETAOnOfficerPicture(id, x, y, p.Width); err != nil {
			return false, err
		}
		copy(c[:], g.exe.bytes(captainBoxMoving, 4))
	} else {
		copy(c[:], g.exe.bytes(captainBoxIdle, 4))
	}
	g.drawHighlightBox(0xf0, 0x11a, p.Width+2, p.Height+2, c)
	return true, nil
}

// printGalmapScannedShip is Print_Galmap_Scanned_Ship_ (0x957e3): what the
// hovered ship icon is, under the map.
func (g *Game) printGalmapScannedShip(icon int) error {
	s, st := &g.state, &g.stacks
	t := g.e.Text()
	node := st.icon[icon].node
	sh := &s.Ships[st.node[node].ship]
	owner := int(sh.Owner)
	t.SetOutlineColor(0, 0)
	x, y, w := 0x43, 0xf8, 199
	if g.currentScreen == 0x1d {
		x, y, w = 0x16c, 0x19c, 0xcb
	}
	h := 0x15
	var race string
	if node >= 5000 || owner >= 9 {
		if node >= 5000 {
			node -= 0x137f
		}
		race = g.raceName(g.shipStackOwner(node))
	}
	if race != "" {
		c := g.fontColors2(4, 0x4c, 0x4f)
		t.SetStyleShadow(4, c, font.Outline)
		return t.PrintCentered(x+0x66, y+5, strings.ToUpper(race))
	}
	cnt := s.nShipsByTypeAtLocation(sh.X, sh.Y, owner)
	base := byte(0x49)
	if owner < 8 {
		base = mainPalettePlayerColors[s.Players[owner].Color]
	}
	c := g.fontColors2(1, base, base+5)
	// sp[0] stays as the stack left it in the original; 0 here.
	var sp font.Colors
	for i := 1; i < 8; i++ {
		sp[i] = c[i] + 4
		c[i] += 2
	}
	t.Special = sp
	t.SetStyleShadow(1, c, font.Outline)
	t.SetSpacing(1)
	buf := strings.ToUpper(fmt.Sprintf(cFormat(g.hMessage(0x131)), g.raceName(owner)))
	add := func(n int16, one, many int) {
		if n <= 0 {
			return
		}
		id := many
		if n == 1 {
			id = one
		}
		buf += fmt.Sprintf(cFormat(g.hMessage(id)), int(n))
	}
	add(cnt.outpost, 0x102, 0x103)
	add(cnt.transport, 0x104, 0x105)
	add(cnt.colony, 0x106, 0x107)
	for i := 0; i < 6; i++ {
		if n := cnt.hulls[i]; n > 0 {
			hn, err := g.hullName(i, n != 1)
			if err != nil {
				return err
			}
			buf += fmt.Sprintf(cFormat(cStringAt(g.exe, hullCountFormat)), int(n), hn)
		}
	}
	if len(buf) > 2 {
		buf = buf[:len(buf)-2]
	}
	mh, err := t.FormattedMaxHeight(w, buf)
	if err != nil {
		return err
	}
	if h < mh {
		t.Special = sp
		t.SetStyleShadow(0, c, font.Outline)
	}
	return g.printParagraphCentered(x, y, w, h+1, buf)
}
