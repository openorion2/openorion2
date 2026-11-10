package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/save"
)

// F9 on the galaxy screen: Distance_Between_Stars_Screen_ (screen 0x1c,
// MAINSCR.C). Spec: docs/screens/star-distances.md.

const (
	starDistMouseFrom = 0x1821cc // MAINSCR.C's _mouse_list_from, 5 entries
	starDistMouseTo   = 0x182208 // _mouse_list_to
	starDistColors    = 0x8a83b  // the parsecs' font colours (code segment)
	starDistLine      = 0x182135 // _ship_direction_line_green_colors
	starBHFieldDim    = 0x81bfc  // Add_Star_Fields_'s black hole sizes by zoom
)

// addStarFields is Add_Star_Fields_ (0x822af): a disc field over each star,
// radius half the star's picture less two; -10000 for stars off the window.
func (g *Game) addStarFields() error {
	s := &g.state
	m := &g.ms
	m.starButton = m.starButton[:0]
	for i := 0; i < int(s.NumStars); i++ {
		x, y := g.starDrawCoords(i)
		z := g.mapScaleToZoomLevel()
		var dim int16
		if s.Stars[i].SpectralClass == save.SpectralBlackHole {
			dim = g.exeI16(starBHFieldDim + uint32(2*z))
		} else {
			dim = starFieldsDim[int(s.Stars[i].Size)+z]
		}
		n, err := g.e.AddClippedCircleField(int(x), int(y), int(int16(dim/2-2)), "", 0)
		if err != nil {
			return err
		}
		m.starButton = append(m.starButton, int16(n))
	}
	return nil
}

// starButtonAt is _star_button[i], -1000 once Set_Main_Screen_Fields_To_Null_
// has run.
func (g *Game) starButtonAt(i int) int16 {
	if i < len(g.ms.starButton) {
		return g.ms.starButton[i]
	}
	return engine.NoField
}

// distanceBetweenStarsScreen is Distance_Between_Stars_Screen_ (0x8ab6d): a
// click picks a star, hovering another shows the parsecs between the two.
func (g *Game) distanceBetweenStarsScreen() error {
	e, v, s := g.e, g.e.V, &g.state
	e.ClearFields()
	g.setMainScreenFieldsToNull()
	v.SetWindow(0x16, 0x16, 0x20f, 0x1a5)
	v.ClipOn()
	if err := g.addStarFields(); err != nil {
		return err
	}
	v.ClipOff()
	v.ResetWindow()
	esc, err := e.AddHotKey("\x1b")
	if err != nil {
		return err
	}
	mapF, err := e.AddHiddenField(0x16, 0x16, 0x1fa, 0x190, "", 0x29)
	if err != nil {
		return err
	}
	whole, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0)
	if err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawMainScreenForStarDistances, 2)
	g.screenData[3], g.screenData[4] = -1, -1
	v.SetPageOff()
	if err := g.drawMainScreenFilled(false); err != nil {
		return err
	}
	v.SetPaletteChanges(0, 0xff)
	e.UpdateGlassRemapColors()
	g.darkenFill(0, 0, 0x15, 0x1df)
	g.darkenFill(0, 0, 0x27f, 0x15)
	g.darkenFill(0x20f, 0, 0x27f, 0x1df)
	g.darkenFill(0x15, 0x1a5, 0x20f, 0x1df)
	v.CopyOffToBack()
	g.ub.globalVar = -1
	for done := false; !done; {
		e.MarkTime()
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		sf, err := e.ScanInput()
		if err != nil {
			return err
		}
		g.ms.scannedField = int16(sf)
		if g.screenData[3] == -1 {
			e.SetMouseList(g.exeMouseList(starDistMouseFrom, 5))
		} else {
			e.SetMouseList(g.exeMouseList(starDistMouseTo, 5))
		}
		r16 := int16(r)
		for i := 0; i < int(s.NumStars); i++ {
			b := g.starButtonAt(i)
			if r16 == b {
				g.screenData[3] = int16(i)
			} else if -int(b) == int(r16) {
				done = true
			}
		}
		rr := int(r16)
		if r16 == int16(whole) || rr == -int(int16(whole)) || rr == -int(int16(mapF)) || rr == keyCodeF9 || r16 == int16(esc) {
			done = true
		}
		if !done {
			v.SetPageOff()
			if err := g.drawMainScreenForStarDistances(); err != nil {
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
	e.SetMouseList(engine.MouseArrow)
	g.currentScreen = ScreenMain
	return nil
}

// drawMainScreenForStarDistances is Draw_Main_Screen_For_Star_Distances_
// (0x8af4e), also the screen's auto function.
func (g *Game) drawMainScreenForStarDistances() error {
	v, s, m := g.e.V, &g.state, &g.ms
	sel := g.screenData[3]
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	if err := g.drawMainMainScreen(); err != nil {
		return err
	}
	v.SetWindow(0x16, 0x16, 0x20f, 0x1a5)
	v.ClipOn()
	g.ub.globalVar = int8(g.screenData[4])
	if sel > -1 {
		if err := g.highlightStar(int(sel)); err != nil {
			return err
		}
		// The test below does not depend on i: the line and the label are
		// drawn once per star until the hovered star is reached.
		found := false
		for i := 0; i < int(s.NumStars) && !found; i++ {
			if g.starButtonAt(i) == m.scannedField && int16(i) != sel {
				found = true
				g.screenData[4] = int16(i)
			}
			to := g.screenData[4]
			if to <= -1 || sel == to || int16(g.ub.globalVar) != to {
				continue
			}
			if err := g.printStarDistance(int(sel), int(to)); err != nil {
				return err
			}
		}
	}
	m.multiColoredLineStart = (m.multiColoredLineStart + 1) % 8
	v.ClipOff()
	v.ResetWindow()
	return nil
}

// printStarDistance is the body of the loop above: the parsecs from a to b
// and the line between them.
func (g *Game) printStarDistance(a, b int) error {
	t := g.e.Text()
	x1, y1 := g.starDrawCoords(a)
	x2, y2 := g.starDrawCoords(b)
	d := g.parsecsBetweenStars(a, b)
	t.SetOutlineColor(0, 0)
	// community patch 1.51 (0x8b06e je -> jmp): the parsecs also across a
	// black hole, where 1.40b23 printed H 0xea in 0x8a843's colours.
	t.SetStyleShadow(2, g.exeColors(starDistColors), font.Outline)
	str := cSprintf(g.hMessage(0xeb), d)
	w := t.Width([]byte(str))
	var err error
	if w+int(x2)+10 > 0x20a {
		err = t.PrintRight(0x20a, int(y2), str)
	} else {
		err = t.Print(int(int16(x2+10)), int(y2), str)
	}
	if err != nil {
		return err
	}
	var colors [8]byte
	copy(colors[:], g.exe.bytes(starDistLine, 8))
	g.e.V.DirectionalMultiColoredLine(int(x1), int(y1), int(x2), int(y2), colors, int(g.ms.multiColoredLineStart))
	return nil
}

// highlightStar is Highlight_Star_ (0x86076).
func (g *Game) highlightStar(i int) error {
	if g.state.Stars[i].SpectralClass == save.SpectralBlackHole {
		return nil
	}
	if err := g.printHighlightedStarName(i); err != nil {
		return err
	}
	g.pulseSelectedStarColor()
	return nil
}
