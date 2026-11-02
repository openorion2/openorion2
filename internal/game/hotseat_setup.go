package game

import (
	"strconv"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The hotseat setup (screen 0x10): each human picks a race in turn, then
// the game starts. Spec: docs/screens/hotseat.md.

const (
	hotseatSetupHelp = 0x181fba // Set_Hotseat_Setup_Screen_Help_List_'s 4 entries
	hotseatEscKey    = 0x178fd9 // "\x1b"
)

// hotseatSetup holds Reload_Hotseat_Screen_'s pictures.
type hotseatSetup struct {
	back, box, add, start, cancel *gfx.Picture // 0x19b87c, 874, 878, 86c, 870
}

// reloadHotseatScreen is Reload_Hotseat_Screen_ (0x625ca).
func (g *Game) reloadHotseatScreen() error {
	hs := &g.hsSetup
	for i, p := range []**gfx.Picture{&hs.back, &hs.box, &hs.add, &hs.start, &hs.cancel} {
		entry := []int{0, 0xb, 0xc, 0xd, 0xe}[i]
		var err error
		if *p, err = g.picture(multiGMLBX, entry); err != nil {
			return err
		}
	}
	return nil
}

// fadeIntoHotseatScreen is Fade_Into_Hotseat_Screen_ (0x6264e).
func (g *Game) fadeIntoHotseatScreen() error {
	e := g.e
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	if err := e.LoadPalette(0xb, 0, 0xff); err != nil {
		return err
	}
	e.V.SetPaletteChanges(0, 0xff)
	e.RefreshPalette()
	if err := e.DarkenPalette(100); err != nil {
		return err
	}
	e.SetMouseList(engine.MouseArrow)
	e.V.SetPageOff()
	if err := e.Draw(0, 0, g.hsSetup.back); err != nil {
		return err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	if err := e.TogglePages(); err != nil {
		return err
	}
	if err := e.FastFadeIn(); err != nil {
		return err
	}
	e.V.SetPageBack()
	if err := e.Draw(0, 0, g.hsSetup.back); err != nil {
		return err
	}
	e.V.CopyBackToOff()
	return nil
}

// hotseatBoxXY is the centred box of the setup screen.
func (g *Game) hotseatBoxXY() (int, int) {
	b := g.hsSetup.box
	return (0x280 - b.Width) / 2, (0x1e0 - b.Height) / 2
}

// drawHotseatScreen is Draw_Hotseat_Screen_ (0x626d6): the box and the
// player numbers, bright for the humans set up so far.
func (g *Game) drawHotseatScreen() error {
	e := g.e
	x, y := g.hotseatBoxXY()
	if err := e.Draw(x, y, g.hsSetup.box); err != nil {
		return err
	}
	dim := g.fontColors2(1, 0x9a, 0x9c)
	bright := g.fontColors2(1, 0x9a, 0x9f)
	t := e.Text()
	t.SetStyleShadow(1, bright, font.ShadowDown)
	for i := 0; i < int(g.settings[0xd5]); i++ {
		c := dim
		if int16(i) < g.hs.humans {
			c = bright
		}
		t.SetStyleShadow(1, c, font.ShadowDown)
		if err := t.Print(x+0xe, y+0x46+9*i, strconv.Itoa(i+1)); err != nil {
			return err
		}
	}
	return nil
}

// initHotseatScreen is Init_Hotseat_Screen_ (0x627dd): add a human (while
// fewer than 8), start, and cancel (Esc).
func (g *Game) initHotseatScreen() (add, start, cancel int, err error) {
	e := g.e
	if err = g.reloadHotseatScreen(); err != nil {
		return
	}
	if g.hs.humans > 0 {
		if err = g.fadeIntoHotseatScreen(); err != nil {
			return
		}
	}
	e.ClearFields()
	x, y := g.hotseatBoxXY()
	add = -1000
	if g.hs.humans < 8 {
		if add, err = e.AddButtonField(x+0x7d, y+0x63, "", g.hsSetup.add, "", 0x28); err != nil {
			return
		}
	}
	if start, err = e.AddButtonField(x+0x7d, y+0x88, "", g.hsSetup.start, "", 0x28); err != nil {
		return
	}
	if cancel, err = e.AddButtonField(x+0x8c, y+0xc8, "", g.hsSetup.cancel, cStringAt(g.exe, hotseatEscKey), 0x28); err != nil {
		return
	}
	e.AssignAutoFunction(g.drawHotseatScreen, 2)
	return
}

// setHotseatSetupScreenHelpList is Set_Hotseat_Setup_Screen_Help_List_
// (0x6fb88): its entries offset by the box's corner.
func (g *Game) setHotseatSetupScreenHelpList(x, y int) {
	list := g.helpListAt(hotseatSetupHelp, 4)
	for i := range list {
		list[i].X1 += x
		list[i].Y1 += y
		list[i].X2 += x
		list[i].Y2 += y
	}
	g.e.DeactivateHelpList()
	g.e.SetHelpList(list)
}

// hotseatScreen is Hotseat_Screen_ (0x628e2), screen 0x10: the humans
// pick their races one after another (the first at once), then the game
// starts; a single human makes it a single player game.
func (g *Game) hotseatScreen() error {
	e, s := g.e, &g.state
	g.hotSeatFlag = 3
	add, start, cancel, err := g.initHotseatScreen()
	if err != nil {
		return err
	}
	x, y := g.hotseatBoxXY()
	n := 0
	g.hs.humans = 0
	temp := new(save.Player) // Allocate_Next_Block_ + Clear_Structure_, once
	done := 0
	for done == 0 {
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		if g.hs.humans == 0 {
			in = add
		}
		if in == cancel {
			g.currentScreen, done = ScreenMultiplayer, 1
		}
		if in == start {
			if done, err = g.hotseatConfirmStart(done); err != nil {
				return err
			}
		}
		if in == add {
			s.PlayerNum = int16(n)
			r, err := g.raceSelectionScreen(temp)
			if err != nil {
				return err
			}
			g.setHotseatSetupScreenHelpList(x, y)
			if r != 0 {
				s.Players[n] = *temp
				n++
				g.hs.humans++
				s.Players[n-1].Objective = save.ObjectiveHuman
			} else if g.hs.humans == 0 {
				done, g.currentScreen = 1, ScreenMultiplayer
			}
			if add, start, cancel, err = g.initHotseatScreen(); err != nil {
				return err
			}
			if int16(g.settings[0xd5]) == g.hs.humans {
				done = 2
			}
		}
		if done == 0 {
			e.V.SetPageOff()
			if err := g.drawHotseatScreen(); err != nil {
				return err
			}
			if err := e.TogglePages(); err != nil {
				return err
			}
		}
	}
	e.ClearFields()
	e.DeactivateAutoFunction()
	if done != 2 {
		return nil
	}
	if g.newGameHook != nil {
		g.newGameHook(g)
	}
	st := &g.settings
	if err := g.initNewGame(int16(st[0xd4]), int16(st[0xd6]), int16(st[0xd7]), int16(st[0xd5])); err != nil {
		return err
	}
	g.currentScreen = ScreenHotseat
	return nil
}

// hotseatConfirmStart is START of Hotseat_Screen_ (0x629a1..0x62a97): one
// human asks to play alone, more ask to fill the rest with the computer.
func (g *Game) hotseatConfirmStart(done int) (int, error) {
	e := g.e
	var msg string
	if g.hs.humans == 1 {
		msg = g.hMessage(0xa2)
	} else {
		left := int(g.settings[0xd5]) - int(g.hs.humans)
		players, others := g.hMessage(0xa5), g.hMessage(0xa5)
		if left == 1 {
			others = g.hMessage(0xa4)
		}
		msg = cSprintf(g.hMessage(0xa3), int(g.hs.humans), players, left, others)
	}
	yes, err := g.confirmationBox(msg)
	if err != nil {
		return done, err
	}
	if yes {
		if g.hs.humans == 1 {
			g.gameType = save.GameSingle
		}
		return 2, nil
	}
	e.V.SetPageBack()
	if err := e.Draw(0, 0, g.hsSetup.back); err != nil {
		return done, err
	}
	e.V.CopyBackToOff()
	return done, nil
}
