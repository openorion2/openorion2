package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
	"github.com/openorion2/openorion2/internal/update"
)

// Main menu, MOX\OTHERS\MAINMENU.C. Spec: docs/screens/main-menu.md.

const mainMenuLBX = "MAINMENU.LBX"

// The six buttons, top to bottom: Continue, Load, New, Multiplayer, Hall of
// Fame, Quit.
const (
	btnContinue = iota
	btnLoad
	btnNew
	btnMultiplayer
	btnHallOfFame
	btnQuit
	buttonCount
)

// mainMenuButtonY are the buttons' top rows; they all start at x 0x19f.
var mainMenuButtonY = [buttonCount]int{0xac, 0xc3, 0xd9, 0xf0, 0x106, 0x11d}

// mainMenuFieldY are the hidden fields' rows, inclusive.
var mainMenuFieldY = [buttonCount][2]int{
	{0xac, 0xc1}, {0xc3, 0xd7}, {0xd9, 0xee}, {0xf0, 0x104}, {0x106, 0x11b}, {0x11d, 0x132},
}

var mainMenuHotkeys = [buttonCount]string{"C", "L", "N", "M", "H", "Q"}

// mainMenuHelp is _main_menu_screen_help_list.
var mainMenuHelp = []engine.HelpEntry{
	{ID: 645, X1: 415, Y1: 172, X2: 567, Y2: 193}, {ID: 646, X1: 415, Y1: 195, X2: 567, Y2: 215}, {ID: 647, X1: 415, Y1: 217, X2: 567, Y2: 238},
	{ID: 648, X1: 415, Y1: 240, X2: 567, Y2: 260}, {ID: 649, X1: 415, Y1: 262, X2: 567, Y2: 283}, {ID: 650, X1: 415, Y1: 285, X2: 567, Y2: 306},
}

// mainMenuButton is one 14-byte record of the _mainmenu block.
type mainMenuButton struct {
	pics  [3]*gfx.Picture // MAINMENU.LBX 3+3i, 4+3i (highlighted), 5+3i (disabled)
	field int
}

type mainMenuState struct {
	background *gfx.Picture // _main_menu_background_seg, entry 2
	first      *gfx.Picture // _mainmenu_first_seg, entry 1
	buttons    [buttonCount]mainMenuButton
	scanned    int // _scanned_field
}

// MainMenuScreen is Main_Menu_Screen_.
func (g *Game) MainMenuScreen() error {
	e, m := g.e, &g.mainMenu
	e.V.SetRefreshFull()
	e.ClearFields()
	e.MoxUpdate()
	var err error
	if m.first, err = g.picture(mainMenuLBX, 1); err != nil {
		return err
	}
	e.MoxUpdate()
	if err := g.loadCredits(); err != nil {
		return err
	}
	e.MoxUpdate()
	g.gameType = 0
	for i := range m.buttons {
		m.buttons[i].field = -1 // the memset of _mainmenu to 0xff
	}
	e.MoxUpdate()

	animKey := -1
	switch {
	case g.panMainMenuPanel:
		if err := g.reloadMainMenuScreen(); err != nil {
			return err
		}
		g.panMainMenuPanel = false
		var flag bool
		if err := e.FadeIntoScreen(g.mainMenuFadeFunction2, &flag, 5); err != nil {
			return err
		}
		e.V.SetRefreshFull()
		if animKey, err = g.doMainMenuFileAnimation(); err != nil {
			return err
		}
		if err := g.addMainMenuFields(); err != nil {
			return err
		}
	case g.previousScreen == 8 || g.previousScreen == 14 || g.previousScreen == 15 ||
		g.previousScreen == 12 || g.previousScreen == 13 || g.fullDrawScreen:
		g.fullDrawScreen = false
		if err := g.reloadMainMenuScreen(); err != nil {
			return err
		}
		var flag bool
		if err := e.FadeIntoScreen(g.mainMenuFadeFunction, &flag, 5); err != nil {
			return err
		}
		if err := g.addMainMenuFields(); err != nil {
			return err
		}
	case g.previousScreen == ScreenMainMenu:
		if err := g.reloadMainMenuScreen(); err != nil {
			return err
		}
		e.V.SetPageOff()
		e.V.Fill(0, 0, engine.W-1, engine.H-1, 0)
		if err := e.Draw(0, 0, m.background); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := g.addMainMenuFields(); err != nil {
			return err
		}
	default:
		// The original skips the fields here (open question in the spec).
		g.Log("main menu entered from screen %d without fields", g.previousScreen)
	}

	e.V.SetRefreshStencil()
	e.V.SetPageBack()
	e.V.Fill(0, 0, engine.W-1, engine.H-1, 0)
	if err := e.Draw(0, 0, m.background); err != nil {
		return err
	}
	e.V.CopyBackToOff()
	if err := e.TogglePages(); err != nil {
		return err
	}
	hot, err := e.AddHotKey("N")
	if err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawMainMenuScreen, 2)
	e.SetHelpList(mainMenuHelp)
	g.credits.resetCredits()

	for done := false; ; {
		if done {
			g.fullDrawScreen = false
			e.DeactivateAutoFunction()
			return nil
		}
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		if m.scanned, err = e.ScanInput(); err != nil {
			return err
		}
		// port-only: a newer release, offered once (updateui.go); after its
		// boxes the menu comes again, redrawn.
		if shown, err := g.offerUpdate(); err != nil {
			return err
		} else if shown {
			g.currentScreen, g.fullDrawScreen, done = ScreenMainMenu, true, true
			continue
		}
		// port-only: a crash report from the last run, offered once (crashui.go).
		if shown, err := g.offerCrashReport(); err != nil {
			return err
		} else if shown {
			g.currentScreen, g.fullDrawScreen, done = ScreenMainMenu, true, true
			continue
		}
		e.MarkTime()
		b := &m.buttons
		switch {
		case r == b[btnContinue].field || (animKey == 'C' && b[btnContinue].field != engine.NoField):
			ok, err := g.loadGame(9)
			if err != nil {
				return err
			}
			g.afterMainMenuLoad(ok, 0xcf)
			done = true
		case r == b[btnNew].field || r == hot || animKey == 'N':
			g.currentScreen, g.returnScreen, done = ScreenNewGame, 0, true
		case r == b[btnLoad].field || animKey == 'L':
			loaded, err := g.doMainmenuLoadScreen()
			if err != nil {
				return err
			}
			switch {
			case loaded:
				g.afterMainMenuLoad(true, 0xd0)
			case g.previousScreen == 0:
				g.currentScreen = 0
			default:
				g.currentScreen = ScreenMainMenu
			}
			done = true
		case r == b[btnMultiplayer].field || animKey == 'M':
			g.currentScreen, g.returnScreen, done = ScreenMultiplayer, ScreenMainMenu, true
		case r == b[btnHallOfFame].field || animKey == 'H':
			g.currentScreen, g.returnScreen, done = ScreenHallOfFame, ScreenMainMenu, true
		case r == b[btnQuit].field || animKey == 'Q':
			g.currentScreen, done = ScreenQuit, true
		}
		if !done {
			e.V.SetPageOff()
			if err := g.drawMainMenuScreen(); err != nil {
				return err
			}
			if err := e.TogglePages(); err != nil {
				return err
			}
		}
		if err := e.ReleaseTime(3); err != nil {
			return err
		}
	}
}

// reloadMainMenuScreen is Reload_Main_Menu_Screen_.
func (g *Game) reloadMainMenuScreen() error {
	m := &g.mainMenu
	g.e.MoxUpdate()
	var err error
	if m.background, err = g.picture(mainMenuLBX, 2); err != nil {
		return err
	}
	g.e.MoxUpdate()
	for i := range m.buttons {
		for k := 0; k < 3; k++ {
			if m.buttons[i].pics[k], err = g.picture(mainMenuLBX, 3+3*i+k); err != nil {
				return err
			}
		}
		m.buttons[i].field = engine.NoField
		g.e.MoxUpdate()
	}
	return nil
}

// addMainMenuFields is Add_Main_Menu_Fields_. Continue needs save10.gam (and
// a single-player game type in it, read by Get_Game_Type_From_Saved_Game_,
// not ported: taken as single player); Load needs any save.
func (g *Game) addMainMenuFields() error {
	e, b := g.e, &g.mainMenu.buttons
	e.ClearFields()
	add := func(i int) error {
		n, err := e.AddHiddenField(0x19f, mainMenuFieldY[i][0], 0x237, mainMenuFieldY[i][1], mainMenuHotkeys[i], ')')
		b[i].field = n
		return err
	}
	if g.dirMatch("save10.gam") {
		if err := add(btnContinue); err != nil {
			return err
		}
	} else {
		b[btnContinue].field = engine.NoField
	}
	if !g.disallowSavesCode && g.dirMatch("save?.gam") {
		if err := add(btnLoad); err != nil {
			return err
		}
	} else {
		b[btnLoad].field = engine.NoField
	}
	for i := btnNew; i < buttonCount; i++ {
		if err := add(i); err != nil {
			return err
		}
	}
	return nil
}

// mainMenuFadeFunction2 is Main_Menu_Fade_Function2_: the station without
// the button panel, before the panel slides out.
func (g *Game) mainMenuFadeFunction2() error {
	return g.e.Draw(0, 0, g.mainMenu.first)
}

// doMainMenuFileAnimation is Do_Main_Menu_File_Animation_: play the panel
// sliding out (MAINMENU.LBX entry 0) one frame per tick with the palette of
// the finished menu, until its last frame or until the player clicks or
// presses a key. Hot keys C, S, L, M, H, Q end it with that letter as the
// result, which Main_Menu_Screen_ then acts on; otherwise the result is -1.
// Sound 0x35 plays at frame 5, 0x36 at frame 0x2e and on an early stop.
func (g *Game) doMainMenuFileAnimation() (int, error) {
	e, m := g.e, &g.mainMenu
	anim, err := g.picture(mainMenuLBX, 0)
	if err != nil {
		return 0, err
	}
	last := anim.Frames - 1
	e.V.SetPaletteChanges(0, 255)
	e.DrawPalette(m.background)
	e.ClearFields()
	var keys [6]int
	for i, k := range []string{"C", "S", "L", "M", "H", "Q"} {
		// Tiny fields stepped down the top left corner; only their hot keys matter.
		if keys[i], err = e.AddHiddenField(10+10*i, 0x14+10*i, 0x19+10*i, 0x23+10*i, k, ')'); err != nil {
			return 0, err
		}
	}
	space, err := e.AddHotKey(" ")
	if err != nil {
		return 0, err
	}
	anywhere, err := e.AddHiddenField(0, 0, engine.W-1, engine.H-1, "\x1b", ')')
	if err != nil {
		return 0, err
	}
	e.AssignAutoFunction(func() error { return e.DrawFileAnimation(anim) }, 1)
	result := -1
	snd, early := 0, false
	for done := false; ; {
		if done {
			if early {
				e.StopSound(snd)
				g.moxPlaySound(0x36)
			}
			return result, nil
		}
		e.MarkTime()
		r, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		frame := anim.CurrentFrame
		switch frame {
		case 5:
			snd = g.moxPlaySound(0x35)
		case 0x2e:
			e.StopSound(snd)
			snd = g.moxPlaySound(0x36)
		}
		if r == anywhere || r == space {
			done, early = true, true
		}
		if frame == last {
			done = true
		}
		for i := 0; i < len(keys) && !done; i++ {
			if r == keys[i] {
				result, done, early = int("CSLMHQ"[i]), true, true
			}
		}
		if !done {
			e.V.SetPageOff()
			if err := e.DrawFileAnimation(anim); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
		}
		if err := e.ReleaseTime(1); err != nil {
			return 0, err
		}
	}
}

// mainMenuFadeFunction is Main_Menu_Fade_Function_: the background only.
func (g *Game) mainMenuFadeFunction() error {
	return g.e.Draw(0, 0, g.mainMenu.background)
}

// drawMainMenuScreen is Draw_Main_Menu_Screen_.
func (g *Game) drawMainMenuScreen() error {
	e, m := g.e, &g.mainMenu
	if g.fullDrawScreen {
		e.V.SetPageOff()
		e.V.Fill(0, 0, engine.W-1, engine.H-1, 0)
		if err := e.Draw(0, 0, m.background); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		if err := e.TogglePages(); err != nil {
			return err
		}
		g.credits.resetCredits()
		g.fullDrawScreen = false
	}
	for i := range m.buttons {
		if m.scanned == m.buttons[i].field {
			if err := e.Draw(0x19f, mainMenuButtonY[i], m.buttons[i].pics[1]); err != nil {
				return err
			}
			break
		}
	}
	for _, i := range []int{btnContinue, btnLoad} {
		if m.buttons[i].field == engine.NoField {
			if err := e.Draw(0x19f, mainMenuButtonY[i], m.buttons[i].pics[2]); err != nil {
				return err
			}
		}
	}
	if g.credits.enabled {
		if err := g.drawCredits(); err != nil {
			return err
		}
	}
	t := e.Text()
	t.SetStyle(1, fontColors2(1, 0xd2, 0xd7))
	if err := t.PrintCentered(0x205, 0x1bb-(t.Font.Styles[1].Height+1), "Ver "+BaseVersion+"-"+update.Version); err != nil {
		return err
	}
	return g.drawAnnouncement() // port-only
}

// BaseVersion is the original's version the port follows (its main menu's
// "Ver 1.40b23"); port-only: the menu adds the port's own release after it.
const BaseVersion = "1.40b23"

// fontColors2 is Font_Colors2_ for the modes ported so far: mode 0 or 1 puts
// a in shade 1 and b in the other seven.
func fontColors2(mode int, a, b byte) font.Colors {
	_ = mode
	return font.Colors{a, b, b, b, b, b, b, b}
}

// afterMainMenuLoad picks the next screen after Continue or Load: network
// and modem games cannot be resumed here (H_Message_ msg, Warning_Box_).
func (g *Game) afterMainMenuLoad(ok bool, msg int) {
	switch {
	case !ok:
		g.currentScreen = ScreenMainMenu
	case g.gameType == save.GameNetwork || g.gameType == save.GameModem:
		g.logBoxError(g.warningBox(g.hMessage(msg)))
		g.currentScreen = ScreenMainMenu
	case g.gameType == save.GameHotseat:
		g.currentScreen = ScreenHotseat
	default:
		g.currentScreen = ScreenReports
	}
}
