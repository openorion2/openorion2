package game

import (
	"github.com/openorion2/openorion2/internal/font"
)

// Evolutionary Mutation (RACEOPT.C, the _EM_ routines): the human spends
// four more picks on the race over the picks screen; only additions are
// open. Spec: docs/screens/evolution.md.

const raceOptNoKey = 0x178f21 // "": ACCEPT has no hot key here

// setupEvolutionaryUpgrade is Setup_Evolutionary_Upgrade_ (0x5b0aa): a
// human's picks become the base the mutation screen spends from (its
// government byte keeps only bits 1..3), the AI's picks are reset to it.
func (g *Game) setupEvolutionaryUpgrade(p int) error {
	pl := &g.state.Players[p]
	gov := pl.Picks[0]
	if pl.Objective == 100 {
		copy(pl.BasePicks[1:], pl.Picks[1:])
		pl.BasePicks[0] = gov & 0xe
		if g.e == nil {
			g.logOnce("Evolutionary_Mutation_Screen_ needs the screen: the picks stay as they were")
		} else if err := g.evolutionaryMutationScreen(&pl.BasePicks); err != nil {
			return err
		}
		pl.Picks[0] = gov
	} else {
		copy(pl.Picks[1:], pl.BasePicks[1:])
	}
	pl.Evolved = 1
	return nil
}

// evolutionaryMutationScreen is Evolutionary_Mutation_Screen_ (0x5b649)
// over the picks at ptr (the player's base picks).
func (g *Game) evolutionaryMutationScreen(ptr *[31]int8) error {
	e, o := g.e, &g.rsel.opt
	g.rsel.emActive = true
	defer func() { g.rsel.emActive = false }()
	saved := e.GetAutoFunction()
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	if err := e.LoadPalette(9, 0, 0xff); err != nil {
		return err
	}
	g.convertPlayerToFlagsEM(ptr)
	orig := *ptr
	// The groups' values become their levels 1..3; a level found sticks
	// unless a later compare matches it again.
	for b := 1; b < 10; b++ {
		for d := 2; d >= 0; d-- {
			if orig[b] == g.raceSelPickValue(b, int8(d+1)) {
				orig[b] = int8(d + 1)
			}
		}
	}
	g.setRaceOptionHelpList()
	e.ClearFields()
	if err := g.reloadRacialOptionScreen(); err != nil {
		return err
	}
	g.calculateEMPicks()
	start := o.picks
	e.AssignAutoFunction(g.drawRacialOptionScreen, 1)
	o.colorOpt = font.Colors{0x8e, 0x9d, 0x9c, 0x9d, 0x9d, 0x9d, 0x9d, 0x9d}
	e.Text().SetStyle(4, o.colorOpt)
	if err := g.addRaceOptionsFieldsEM(&orig, start); err != nil {
		return err
	}
	savedB := o.button
	e.V.SetPageBack()
	if err := g.drawRaceOptionBackground(); err != nil {
		return err
	}
	e.V.CopyBackToOff()
	e.V.SetPageOff()
	if err := g.drawRacialOptionScreen(); err != nil {
		return err
	}
	if err := e.DarkenPalette(100); err != nil {
		return err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	if err := e.TogglePages(); err != nil {
		return err
	}
	if err := e.FastFadeIn(); err != nil {
		return err
	}
	m1, err := g.kenTextMax(0x4d, 0x64)
	if err != nil {
		return err
	}
	m2, err := g.kenTextMax(0x4e, 0x64)
	if err != nil {
		return err
	}
	if err := g.messageBox(m1 + m2); err != nil {
		return err
	}
	if err := g.raceOptRedraw(); err != nil {
		return err
	}
	g.rsel.emShowing = true
	defer func() { g.rsel.emShowing = false }()
	for done := false; !done; {
		g.fullDrawScreen = false
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		if done, err = g.evolutionaryMutationInput(r, &orig, &savedB); err != nil {
			return err
		}
		g.calculateEMPicks()
		if g.fullDrawScreen {
			e.V.SetPageBack()
			if err := g.drawRaceOptionBackground(); err != nil {
				return err
			}
			e.V.CopyBackToOff()
		}
		if done {
			break
		}
		if err := g.addRaceOptionsFieldsEM(&orig, start); err != nil {
			return err
		}
		e.V.SetPageOff()
		if err := g.drawRacialOptionScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		savedB = o.button
	}
	g.convertFlagsToPlayerEM(ptr)
	e.DeactivateHelpList()
	e.DeactivateAutoFunction()
	e.PutAutoFunction(saved)
	e.ClearFields()
	// Fade_Into_Screen_ gets EDX = &_draw_main_screen_to_back, left over.
	g.ms.drawToBack = true
	return g.fadeIntoScreen(g.drawMiniMainScreen, 0)
}

// evolutionaryMutationInput is the body of Evolutionary_Mutation_Screen_'s
// loop for field r (0x5b83a..0x5bb76); it reports ACCEPT with no debt.
func (g *Game) evolutionaryMutationInput(r int, orig *[31]int8, savedB *[10]int16) (bool, error) {
	o := &g.rsel.opt
	for si := 0; si < 0x24; si++ {
		if r != o.groupButton[si] {
			continue
		}
		grp := si / 4
		if grp == 1 && o.toggle[9] == 1 {
			if err := g.raceOptWarning(0x49); err != nil {
				return false, err
			}
			o.button[grp] = 0
		} else {
			l := int16(si % 4)
			switch {
			case savedB[grp] == l:
				o.button[grp] = 0
				g.calculateEMPicks()
			case g.cost(optGroupCost(si))+o.negativePicks < -10:
				if err := g.raceOptWarning(0x4a); err != nil {
					return false, err
				}
				o.button[grp] = savedB[grp]
			default:
				o.button[grp] = l
			}
		}
		// A group never drops below the race's own level, unless the level
		// clicked costs picks back.
		if lv := int16(orig[grp+1]); lv > o.button[grp] && g.cost(optGroupCost(si)) >= 0 {
			o.button[grp] = lv
		}
	}
	for si := 0; si < optToggles; si++ {
		if r != o.optionButton[si] {
			continue
		}
		o.toggle[si] = 1 - o.toggle[si]
		if o.toggle[si] == 0 {
			if g.cost(si+0x1f) < 0 {
				o.toggle[si] = 1
			}
			g.calculateEMPicks()
			continue
		}
		g.calculateEMPicks()
		if o.negativePicks < -10 {
			if err := g.raceOptWarning(0x4a); err != nil {
				return false, err
			}
			o.toggle[si] = 0
		}
		switch si {
		case 0:
			o.toggle[1] = 0
		case 1:
			o.toggle[0] = 0
		case 9:
			o.toggle[8] = 0
			o.button[1] = 0
		case 8:
			o.toggle[9] = 0
		case 10:
			o.toggle[11] = 0
		case 11:
			o.toggle[10] = 0
		case 12:
			o.toggle[13] = 0
		case 13:
			o.toggle[12] = 0
		}
	}
	if r != o.accept {
		return false, nil
	}
	if o.picks < 0 {
		return false, g.raceOptWarning(0x4b)
	}
	return true, nil
}

// calculateEMPicks is Calculate_EM_Picks_ (0x5b4bf): four picks more.
func (g *Game) calculateEMPicks() {
	g.calculateRacePicks()
	g.rsel.opt.picks += 4
}

// addRaceOptionsFieldsEM is Add_Race_Options_Fields_EM_ (0x5b14a): ACCEPT
// (no hot key), and a hidden field only for the levels and toggles the
// race may still take: a group at or above its own level, a negative
// group level only from level 1, a toggle only to add a positive pick or
// drop a negative one, nothing dearer than the picks at the start.
func (g *Game) addRaceOptionsFieldsEM(orig *[31]int8, start int16) error {
	e, o := g.e, &g.rsel.opt
	e.ClearFields()
	o.exitKen = -1000
	var err error
	if o.accept, err = e.AddButtonField(0x1fa, 0x1c0, "", o.acceptSeg, cStringAt(g.exe, raceOptNoKey), 0x28); err != nil {
		return err
	}
	o.defalt = -1000
	hidden := func(x1, y1, x2, y2 int) (int, error) {
		return e.AddHiddenField(int(int16(x1)), int(int16(y1)), int(int16(x2)), int(int16(y2)), "", 0x29)
	}
	row, col := 0, 0
	si := 0
	for si < 0x24 {
		if si%4 == 0 {
			o.groupButton[si] = -1000
			si++
		}
		for l, yoff := 0, 0; l < 3; l, yoff = l+1, yoff+0xd {
			grp := si / 4
			lv := orig[grp+1]
			var cur int16
			if lv != 0 {
				cur = g.cost(3*grp + int(lv) - 1)
			}
			c := g.cost(3*grp + l)
			ok := false
			switch {
			case c < 0:
				ok = lv == 1
			case c > 0:
				ok = int(lv) <= l+1 || o.button[grp] > int16(lv)
			}
			if ok && int32(c)-int32(cur) <= int32(start) {
				if o.groupButton[si], err = hidden(raceOptX[col]-0x14, raceOptY[row]+yoff, raceOptX[col+1]-0x32, raceOptY[row]+9+yoff); err != nil {
					return err
				}
			} else {
				o.groupButton[si] = -1000
			}
			si++
		}
		if row++; row >= 5 {
			row, col = 0, col+1
		}
	}
	for ; si < 0x29; si++ {
		o.groupButton[si] = -1000
	}
	col++
	y := raceOptY[0]
	off := 0
	for k := 0; k < optToggles; k++ {
		o.optionButton[k] = -1000
		if k < 4 || k > 7 {
			c := g.cost(k + 0x1f)
			has := orig[off+k+10]
			if !(c < 0 && has == 0) && !(c > 0 && has == 1) && c <= start {
				if o.optionButton[k], err = hidden(raceOptX[col]-0x14, y, raceOptX[col]+0x98, y+9); err != nil {
					return err
				}
			}
		}
		if k == 6 {
			off = -1
		}
		y += 0x10
	}
	return nil
}

// convertPlayerToFlagsEM loads Convert_Player_To_Flags_EM_'s result
// (score.go) into the picks screen's buttons and toggles.
func (g *Game) convertPlayerToFlagsEM(p *[31]int8) {
	o := &g.rsel.opt
	o.button, o.toggle = convertPlayerToFlagsEM(g, p)
}

// convertFlagsToPlayerEM is Convert_Flags_To_Player_EM_ (0x5b5a5): the
// government byte is left alone.
func (g *Game) convertFlagsToPlayerEM(p *[31]int8) {
	o := &g.rsel.opt
	for k := 1; k < 10; k++ {
		p[k] = 0
		if o.button[k-1] != 0 {
			p[k] = g.raceSelPickValue(k, int8(o.button[k-1]))
		}
	}
	off := 0
	for k := 10; k < 0x20; k++ {
		if k == 0xf {
			switch {
			case o.toggle[5] == 1:
				p[k] = 1
			case o.toggle[6] == 1:
				p[k] = -1
			default:
				p[k] = 0
			}
			off = 1
			k++
			continue
		}
		p[k-off] = int8(o.toggle[k-10])
	}
}
