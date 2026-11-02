package game

import (
	"strconv"

	"github.com/openorion2/openorion2/internal/gfx"
)

// Scrap_Freighters_ (screen 0x23, MAINSCR.C) and the slider popup it uses
// (Slider_Popup_, MAINPUPS.C). Spec: docs/screens/scrap-freighters.md.

const sliderColors = 0xc6e56 // Draw_Slider_Popup_'s font shades

// sliderPopup holds Slider_Popup_'s globals.
type sliderPopup struct {
	min, max, value int16  // _g_min, _g_max, _g_var
	msg             string // _g_msg
	bg              *gfx.Picture
	minusPic        *gfx.Picture // _anims+0x88 (MAINPUPS 0x4a)
	plusPic         *gfx.Picture // +0x84 (0x4b)
	cancelPic       *gfx.Picture // +0x8c (0x4c)
	okPic           *gfx.Picture // +0x10 (0x4d)
	minus, plus     int          // _mp_fields+0x14, +0x12
	cancel, ok      int          // +0x4, +0x6
	scroll          int          // +0x16
}

// loadSliderPopup is Load_SLIDER_Popup_ (0xc99db).
func (g *Game) loadSliderPopup() error {
	sp := &g.slider
	var err error
	for _, p := range []struct {
		pic   **gfx.Picture
		entry int
	}{{&sp.bg, 0x34}, {&sp.minusPic, 0x4a}, {&sp.plusPic, 0x4b}, {&sp.cancelPic, 0x4c}, {&sp.okPic, 0x4d}} {
		if *p.pic, err = g.picture(mainpupsLBX, p.entry); err != nil {
			return err
		}
	}
	return nil
}

// sliderPopupDialog is Slider_Popup_ (0xc9eb0): pick a number in
// [min, max] starting at value; -1 for cancel. The + and − buttons step
// the value without clamping it.
func (g *Game) sliderPopupDialog(min, max, value int16, msg string, star int) (int16, error) {
	e, mp, sp := g.e, &g.mp, &g.slider
	mp.lastScanned = -1
	sp.min, sp.max, sp.value, sp.msg = min, max, value, msg
	if err := g.loadSliderPopup(); err != nil {
		return 0, err
	}
	if err := g.doPopupSetup(star, sp.bg); err != nil {
		return 0, err
	}
	if err := e.CalculatePictureRemapColors(sp.bg); err != nil {
		return 0, err
	}
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawSliderPopup, 1)
	g.ericPushFields()
	if err := g.addSliderFields(); err != nil {
		return 0, err
	}
	if err := g.drawSliderPopup(); err != nil {
		return 0, err
	}
	if err := e.TogglePages(); err != nil {
		return 0, err
	}
	mp.showing = "slider"
	defer func() { mp.showing = "" }()
	for done := false; !done; {
		in, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		if _, err := e.ScanInput(); err != nil {
			return 0, err
		}
		if err := g.drawSliderPopup(); err != nil {
			return 0, err
		}
		if err := e.TogglePages(); err != nil {
			return 0, err
		}
		if in == sp.ok {
			done = true
		}
		if in == sp.cancel {
			done = true
			sp.value = -1
		}
		if in == sp.plus {
			sp.value++
		}
		if in == sp.minus {
			sp.value--
		}
	}
	g.doPopupCleanup()
	return sp.value, nil
}

// addSliderFields is Add_Slider_Fields_ (0xc9aa4).
func (g *Game) addSliderFields() error {
	e, mp, sp := g.e, &g.mp, &g.slider
	es, err := g.eStrings()
	if err != nil {
		return err
	}
	x, y := mp.x, mp.y
	if sp.scroll, err = e.AddScrollField(x+0x25, y+0x95, int(sp.min), int(sp.max)+1, int(sp.min), int(sp.max), 0xcf, 0xd, &sp.value, es[0xc], 0); err != nil {
		return err
	}
	if sp.minus, err = e.AddButtonField(x+0xf, y+0x9a, es[0xc], sp.minusPic, es[0x89], 0x28); err != nil {
		return err
	}
	if sp.plus, err = e.AddButtonField(x+0x100, y+0x9a, es[0xc], sp.plusPic, es[0xb], 0x28); err != nil {
		return err
	}
	if sp.cancel, err = e.AddButtonField(x+0xf, y+0xae, es[0xc], sp.cancelPic, es[0x87], 0x28); err != nil {
		return err
	}
	if sp.ok, err = e.AddButtonField(x+0xd0, y+0xae, es[0xc], sp.okPic, es[0xc], 0x28); err != nil {
		return err
	}
	mp.hiddenField, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, es[0xc], 0)
	return err
}

// drawSliderPopup is Draw_Slider_Popup_ (0xc9c3d): the frame and message
// on a full draw, then min, value, max and the thumb.
func (g *Game) drawSliderPopup() error {
	e, mp, sp := g.e, &g.mp, &g.slider
	t := e.Text()
	g.setRemapFontStyle(2, g.exeColors(sliderColors))
	x, y := mp.x, mp.y
	if mp.fullDraw || g.fullDrawScreen {
		if err := g.mainpupsTestedExecuteAutofunction(); err != nil {
			return err
		}
		if err := g.remapDraw(x, y, sp.bg); err != nil {
			return err
		}
		if err := t.SqueezePrintFormattedParagraph(x+0x13, y+0x13+0x5c/2, 0xfc, 0x5c, sp.msg, 2, g.exe.bytes(spaceLimits, 6), true); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		mp.fullDraw = false
	}
	for _, n := range []struct {
		dx, dy int
		v      int16
	}{{0x20, 0x84, sp.min}, {0x91, 0x82, sp.value}, {0x101, 0x84, sp.max}} {
		if err := t.PrintCentered(x+n.dx, y+n.dy, strconv.Itoa(int(n.v))); err != nil {
			return err
		}
	}
	pos := (int(sp.value)-int(sp.min))*0xcf/(int(sp.max)-int(sp.min)) + 0x25
	for k, c := range []byte{0x20, 0x24, 0x28, 0x24, 0x20} {
		px := int(int16(x + pos + k))
		e.V.Line(px, y+0x99, px, y+0xa4, c)
	}
	return nil
}

// nFreightersPlayerCanScrap is N_Freighters_Player_Can_Scrap_ (0xff477):
// the freighters not carrying colonists (five per settler).
func (s *State) nFreightersPlayerCanScrap(p int) int16 {
	P := &s.Players[p]
	return int16(int32(P.FreightersTotal) - 5*int32(P.SettlersCount))
}

// scrapFreighters is Scrap_Freighters_ (0x8b519).
func (g *Game) scrapFreighters() error {
	e, s, m := g.e, &g.state, &g.ms
	p := int(s.PlayerNum)
	boxOpen := g.moveableBoxSelected(2)
	confirmed := 1
	n := s.nFreightersPlayerCanScrap(p)
	if n <= 0 {
		if _, err := g.userBox(&m.drawToBack, g.hMessage(0xe6), boxWarning); err != nil {
			return err
		}
	} else {
		e.AssignAutoFunction(g.drawMiniMainScreen, 1)
		g.setHelpListAt(0x1819f2, 1)
		e.ClearFields()
		// The box's corner is compared with 0x20f, 0x1a5 and ignored.
		s.MoveableBoxes[2].Content = 0
		e.DisableDrawAllFields()
		e.V.SetPageOff()
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := g.drawMiniMainScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		e.EnableDrawAllFields()
		r, err := g.sliderPopupDialog(0, n, 0, g.hMessage(0xed), 0)
		if err != nil {
			return err
		}
		P := &s.Players[p]
		plural := func(v int16) string {
			if v == 1 {
				return g.hMessage(0xf0)
			}
			return g.hMessage(0xf1)
		}
		if r >= 0 && (int32(P.FreightersTotal)-int32(r) <= 0 || int32(P.FreightersFree)-int32(r) <= 0) {
			msg := cSprintf(g.hMessage(0xee), int32(P.FreightersFree), plural(P.FreightersFree), int32(r), plural(r))
			if confirmed, err = g.userBox(&m.drawToBack, msg, boxConfirm); err != nil {
				return err
			}
		}
		if int16(confirmed) != 0 && r > 0 {
			if _, err := g.userBox(&m.drawToBack, cSprintf(g.hMessage(0xef), int32(r), plural(r)), boxMessage); err != nil {
				return err
			}
			P.FreightersTotal = int16(max(int32(P.FreightersTotal)-int32(r), 0))
			g.doPlayerColonyCalculations(p)
		}
	}
	g.currentScreen = g.returnScreen
	if boxOpen {
		s.MoveableBoxes[2].Content = 1
	}
	e.ClearFields()
	e.DeactivateAutoFunction()
	return nil
}
