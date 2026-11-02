package game

import (
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The colony landing screen (screen 0x21, Colony_Landing_Screen_): a colony
// ship lands on the new colony's planet, then the planet's data. COLONY.C.
// Spec: docs/screens/colony-screen.md, "Landing screen".

// colonyLandingScreen is Colony_Landing_Screen_ (0xb9802). It always goes
// on to the colony screen; sounds are not played.
func (g *Game) colonyLandingScreen() error {
	e, s, cs := g.e, &g.state, &g.cscr
	g.setColonyFont()
	if err := g.loadColonyScreenSeg(); err != nil {
		return err
	}
	e.V.ClipOff()
	cs.starHandle = int(g.screenData[0])
	cs.orbitN = int(g.screenData[1])
	cs.usingPal = true
	g.ms.galaxyMap.colonyPalette = true
	cs.planet = int(s.Stars[cs.starHandle].Planets[cs.orbitN])
	cs.valid = true
	g.colonyHandle = int(s.Planets[cs.planet].Colony)
	cs.sysStar = cs.starHandle
	cs.display = 0
	anim, err := g.picture(cStringAt(g.exe, colonyLBX), 6)
	if err != nil {
		return err
	}
	if g.fastFadeOutNeededForColonyPalette() {
		if err := e.FastFadeOut(); err != nil {
			return err
		}
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := g.updateColonyPalette(); err != nil {
			return err
		}
	}
	if g.fastFadeOutNeededForColonyPalette() {
		if err := e.DarkenPalette(100); err != nil {
			return err
		}
		e.V.ClearPaletteChanges(0, 0xff)
	}
	e.RefreshPalette()
	g.resetFieldArrays()
	e.ClearFields()
	if err := g.landingAddFields(); err != nil {
		return err
	}
	g.col.fullDraw = true
	saved := e.GetAutoFunction()
	draw := func() error { return g.drawColonyLandingScreen(anim) }
	e.AssignAutoFunction(draw, 1)
	if err := draw(); err != nil {
		return err
	}
	if err := e.FastFadeIn(); err != nil {
		return err
	}
	e.V.SetRefreshStencil()
	ci := g.colonyHandle
	if s.Colonies[ci].Occupation == 4 && g.sumColonists(ci, -1, -1, -1, -1, 1) != 0 {
		if err := g.occupationPolicyPopup(); err != nil {
			return err
		}
		g.colCalcWrapper(ci)
	}
	for {
		e.MarkTime()
		e.ClearFields()
		if err := g.landingAddFields(); err != nil {
			return err
		}
		if err := draw(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		if _, err := e.ScanInput(); err != nil {
			return err
		}
		if err := e.ReleaseTime(1); err != nil {
			return err
		}
		if r != 0 {
			break
		}
	}
	if err := g.resetFieldMode(); err != nil {
		return err
	}
	e.ClearFields()
	e.DeactivateAutoFunction()
	for _, h := range g.landSound { // 0xb989e..
		g.fadeoutSound(h, 0xa)
	}
	star := int(s.Planets[s.Colonies[ci].Planet].Star)
	if g.nColoniesAndOutpostsAtStar(star) == 1 && g.settings[6] != 0 {
		if _, err := g.changeStarNamePopup(starNamePopupX, starNamePopupY, star, true); err != nil {
			return err
		}
	}
	e.PutAutoFunction(saved)
	cs.usingPal = false
	g.ms.galaxyMap.colonyPalette = false
	g.currentScreen = ScreenColony
	return nil
}

// landingAddFields is Add_Fields_ (0xb937b): the whole screen.
func (g *Game) landingAddFields() error {
	var err error
	g.cscr.fields[cfWhole], err = g.e.AddHiddenField(0, 0, 0x27f, 0x1df, g.eStr(0xc), 0)
	return err
}

// nColoniesAndOutpostsAtStar is N_Colonies_And_Outposts_At_Star_ (0xba8bf).
func (g *Game) nColoniesAndOutpostsAtStar(star int) int16 {
	s := &g.state
	var n int16
	for _, pl := range s.Stars[star].Planets {
		if pl >= 0 && s.Planets[pl].Colony >= 0 {
			n++
		}
	}
	return n
}

// drawColonyLandingScreen is Draw_Colony_Landing_Screen_ (0xb93d1): the
// backdrop and title once, then the landing animation; its last two frames
// show the planet data.
func (g *Game) drawColonyLandingScreen(anim *gfx.Picture) error {
	e, cs := g.e, &g.cscr
	e.V.SetPageOff()
	if g.col.fullDraw {
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		for _, n := range []int{1, 0} {
			p, err := g.cAnimsColony(n)
			if err != nil {
				return err
			}
			if err := e.Draw(0, 0, p); err != nil {
				return err
			}
		}
		g.setColonyFontTo(4)
		if err := e.Text().PrintCentered(0x13f, 10, cSprintf(g.eStr(0xe5), g.planetName(cs.planet))); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		e.V.SetRefreshStencil()
		g.col.fullDraw = false
	}
	if err := e.Draw(0, 0, anim); err != nil {
		return err
	}
	var err error
	if int(int16(anim.CurrentFrame)) >= anim.Frames-2 {
		err = g.printPlanetData(cs.planet)
	}
	// 0xb94bc: the ship's two engine sounds, swapped for the landing one.
	h := &g.landSound
	switch anim.CurrentFrame {
	case 1:
		h[0] = g.moxPlaySound(0x3a)
	case 2:
		h[1] = g.moxPlaySound(0x3a)
	case 0x39:
		g.fadeoutSound(h[0], 2)
		h[2] = g.moxPlaySound(0x39)
	case 0x3e:
		g.fadeoutSound(h[1], 2)
		h[3] = g.moxPlaySound(0x39)
	}
	return err
}

// printPlanetData is Print_Planet_Data_ (0xb953e): name (and owner), size
// and climate, room, minerals, gravity and the special; gas giants and
// asteroids get one line.
func (g *Game) printPlanetData(pl int) error {
	s := &g.state
	t := g.e.Text()
	g.setColonyFontTo(4)
	p := &s.Planets[pl]
	var msg string
	if p.Colony == -1 {
		msg = cSprintf(g.eStr(0x46), g.planetName(pl))
	} else {
		owner := &s.Players[s.Colonies[p.Colony].Owner]
		msg = cSprintf(g.eStr(0x28), g.planetName(pl), save.CString(owner.RaceName[:]))
	}
	if err := t.Print(0x14, 0x28, msg); err != nil {
		return err
	}
	climate := int(p.Climate)
	switch {
	case p.Colony != -1:
		climate = int(s.Colonies[p.Colony].Climate)
	case p.Type != 3:
		return t.Print(0x14, 0x3c, g.eStr(0x132))
	}
	f := g.eStr(0x2d)
	if g.settings.Language() == 4 {
		f = cStringAt(g.exe, 0x179dcc)
	}
	msg = cSprintf(f, g.eStr(planetSizeEStr[p.Size]), g.eStr(planetClimateEStr[climate]))
	if err := t.PrintParagraph(0x14, 0x3c, 0x26b, msg, 0); err != nil {
		return err
	}
	msg = cSprintf(g.eStr(0x1f), int(g.planetMaxPopulationForPlayer(pl, int(s.PlayerNum))))
	if err := t.Print(0x14, 0x50, msg); err != nil {
		return err
	}
	if err := t.Print(0x14, 0x64, cSprintf(g.eStr(0x176), g.eStr(planetMineralEStr[p.Minerals]))); err != nil {
		return err
	}
	if err := t.Print(0x14, 0x78, g.eStr(planetGravityEStr[p.Gravity])); err != nil {
		return err
	}
	if p.Special != 0 {
		msg = cSprintf(g.eStr(0x46), g.eStr(planetSpecialEStr[int(int8(p.Special))]))
		return t.PrintParagraph(0x14, 0x8c, 0x26b, msg, 0)
	}
	return nil
}
