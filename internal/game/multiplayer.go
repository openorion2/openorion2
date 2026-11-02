package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/host"
)

// Multi_Player_Screen_ (screen 0xf): the connection radio buttons and the
// new game / load / join buttons. Network (over TCP, docs/net/lobby.md)
// and hotseat are ported, with loading their saves (netload.go); the modem
// radios are drawn disabled (port-only). Spec: docs/screens/hotseat.md.

const (
	mpSetupHelp    = 0x181ea2 // Set_Multi_Player_Setup_Screen_Help_List_'s 10 entries
	mpGameName     = 0x17a0e7 // "Moo II "
	mpHotKeys      = 0x17a06f // "N", "M", "U", "H": the radio buttons
	mpKeyStart     = 0x17a062 // "S"
	mpKeyLoad      = 0x17a079 // "L"
	mpKeyJoin      = 0x17a07b // "J"
	mpKeySetup     = 0x17a06b // "C"
	mpKeyCancel    = 0x17a06d // "\x1b"
	mpKeyName      = 0x17a077 // "T"
	settingsMPLink = 0x16     // _settings byte: 1 network .. 4 hotseat
)

// mpSetup is the setup screen's part of _g_mp_screen.
type mpSetup struct {
	box                              *gfx.Picture    // +0x13
	radio                            [4]*gfx.Picture // +0x5f..+0x6b
	cancel, start, load, join, setup *gfx.Picture    // +0x73, +0x7b, +0x7f, +0x83, +0x87
	name                             *gfx.Picture    // +0x8b
	radioField                       [4]int          // +0xd7..+0xdd
	cancelF, startF, loadF, joinF    int             // +0xdf, +0xe3, +0xe5, +0xe7
	setupF, nameF                    int             // +0xe9, +0xeb
	radioOn                          [4]int16        // +0xf7..+0xfd
	x, y                             int             // +0xf3, +0xf5
	back                             *gfx.Picture    // +0xf
}

// loadMPSetupPictures is Load_MP_Setup_Pictures_ (0xf42ca); the modem
// defaults it copies into _net (Copy_Modem_Defaults_To_Net_Struct_) are
// network state and not ported.
func (g *Game) loadMPSetupPictures() error {
	g.e.MoxUpdate()
	m := &mpSetup{}
	g.mpSetup = m
	var err error
	if m.back, err = g.picture(multiGMLBX, 0); err != nil {
		return err
	}
	if m.box, err = g.picture(multiGMLBX, 1); err != nil {
		return err
	}
	m.x, m.y = (0x280-m.box.Width)/2, (0x1e0-m.box.Height)/2
	pics := []**gfx.Picture{&m.radio[0], &m.radio[1], &m.radio[2], &m.radio[3],
		&m.cancel, &m.start, &m.load, &m.join, &m.setup, &m.name}
	for i, p := range pics {
		entry := []int{2, 3, 4, 5, 6, 7, 8, 9, 0xa, 0x100}[i]
		if *p, err = g.picture(multiGMLBX, entry); err != nil {
			return err
		}
	}
	return nil
}

// setMultiPlayerGameType is Set_Multi_Player_Game_Type_ (0xf5691): the
// radio buttons and _game_type from the connection setting.
func (g *Game) setMultiPlayerGameType() {
	m := g.mpSetup
	switch g.settings[settingsMPLink] {
	case 2:
		m.radioOn = [4]int16{0, 1, 0, 0}
		g.gameType = 3
		g.net.set8(netConnType, 1)
	case 3:
		m.radioOn = [4]int16{0, 0, 1, 0}
		g.gameType = 3
		g.net.set8(netConnType, 2)
	case 4:
		m.radioOn = [4]int16{0, 0, 0, 1}
		g.gameType = 1
	default:
		m.radioOn = [4]int16{1, 0, 0, 0}
		g.gameType = 2
		g.net.set8(netConnType, 0)
	}
}

// addMPSetupScreenFields is Add_MP_Setup_Screen_Fields_ (0xf009a).
func (g *Game) addMPSetupScreenFields() error {
	e, m := g.e, g.mpSetup
	e.ClearFields()
	var err error
	ys := [4]int{0x5b, 0x7a, 0x9b, 0xbb}
	for i := range m.radio {
		x, y := m.x+0x3b, m.y+ys[i]
		key := cStringAt(g.exe, mpHotKeys+2*uint32(i))
		if i == 1 || i == 2 {
			m.radioField[i] = -1000 // port-only: no modem
			continue
		}
		if m.radioField[i], err = e.AddHiddenField(x, y, x+m.radio[i].Width, y+m.radio[i].Height, key, 0x28); err != nil {
			return err
		}
	}
	// port-only: TEN's button is the online lobby's, drawn by
	// drawLobbyEntry; the same place and hot key.
	x, y := m.x+0x6e, m.y+0xdc
	if m.nameF, err = e.AddHiddenField(x, y, x+m.name.Width-1, y+m.name.Height-1, cStringAt(g.exe, mpKeyName), 0x28); err != nil {
		return err
	}
	if m.cancelF, err = e.AddButtonField(m.x+0xb0, m.y+0x11e, "", m.cancel, cStringAt(g.exe, mpKeyCancel), 0x28); err != nil {
		return err
	}
	m.startF, m.loadF, m.joinF, m.setupF = -1000, -1000, -1000, -1000
	if m.radioOn[0] == 1 || m.radioOn[1] == 1 || m.radioOn[2] == 1 || m.radioOn[3] == 1 {
		if m.startF, err = e.AddButtonField(m.x+0x10d, m.y+0x5b, "", m.start, cStringAt(g.exe, mpKeyStart), 0x28); err != nil {
			return err
		}
		if m.loadF, err = e.AddButtonField(m.x+0x10d, m.y+0x7a, "", m.load, cStringAt(g.exe, mpKeyLoad), 0x28); err != nil {
			return err
		}
		if m.radioOn[3] != 1 {
			if m.joinF, err = e.AddButtonField(m.x+0x10d, m.y+0x9b, "", m.join, cStringAt(g.exe, mpKeyJoin), 0x28); err != nil {
				return err
			}
		}
		if m.radioOn[1] == 2 || m.radioOn[2] == 1 {
			if m.setupF, err = e.AddButtonField(m.x+0x10d, m.y+0xbb, "", m.setup, cStringAt(g.exe, mpKeySetup), 0x28); err != nil {
				return err
			}
		}
	}
	g.setMultiPlayerGameType()
	return nil
}

// drawMPSetupScreen is Draw_MP_Setup_Screen_ (0xf2082) with
// Draw_MP_Buttons_Down_ (0xf21b2): absent buttons drawn pressed (frame 2).
func (g *Game) drawMPSetupScreen() error {
	e, m := g.e, g.mpSetup
	var err error
	if m.box, err = g.picture(multiGMLBX, 1); err != nil {
		return err
	}
	if err := e.Draw(m.x, m.y, m.box); err != nil {
		return err
	}
	ys := [4]int{0x5b, 0x7a, 0x9b, 0xbb}
	for i, p := range m.radio {
		g.setAnimationFrame(p, int(m.radioOn[i]))
		if i == 1 || i == 2 {
			g.setAnimationFrame(p, 2) // port-only: modem and null modem drawn disabled
		}
		if err := e.Draw(m.x+0x3b, m.y+ys[i], p); err != nil {
			return err
		}
	}
	for i, b := range []struct {
		f int
		p *gfx.Picture
	}{{m.startF, m.start}, {m.loadF, m.load}, {m.joinF, m.join}, {m.setupF, m.setup}} {
		if b.f != -1000 {
			continue
		}
		g.setAnimationFrame(b.p, 2)
		if err := e.Draw(m.x+0x10d, m.y+ys[i], b.p); err != nil {
			return err
		}
	}
	g.drawLobbyEntry()
	return nil
}

// lobbyEntryLabel is the online lobby's button on the multiplayer screen.
const lobbyEntryLabel = "GALACTIC COUNCIL ONLINE LOBBY"

// drawLobbyEntry draws the online lobby's button where TEN's was (port-only):
// the lobby's metal button, lit under the pointer, pressed while held.
func (g *Game) drawLobbyEntry() {
	m := g.mpSetup
	x, y, w, h := m.x+0x6e, m.y+0xdc, m.name.Width, m.name.Height
	state := buttonNormal
	if mx, my, b := g.e.Sys.Mouse(); mx >= x && mx < x+w && my >= y && my < y+h {
		state = buttonHover
		if b&host.ButtonLeft != 0 {
			state = buttonPressed
		}
	}
	g.lobbyButtonIn(x, y, w, h, lobbyEntryLabel, state)
}

// multiPlayerScreen is Multi_Player_Screen_ (0xf4d99), screen 0xf.
func (g *Game) multiPlayerScreen() error {
	e := g.e
	// port-only: back from a game of the online lobby: into the lobby again.
	backToLobby := g.endLobbyGame()
	e.SetMouseList(engine.MouseNone)
	saved := e.GetAutoFunction()
	e.SetMouseList(engine.MouseNone)
	e.ClearFields()
	if err := g.loadMPSetupPictures(); err != nil {
		return err
	}
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
	e.DeactivateHelpList()
	e.SetHelpList(g.helpListAt(mpSetupHelp, 10))
	m := g.mpSetup
	if l := g.settings[settingsMPLink]; l == 2 || l == 3 {
		g.settings[settingsMPLink] = 1 // port-only: a modem setting becomes the network
	}
	e.V.SetPageOff()
	if err := e.Draw(0, 0, m.back); err != nil {
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
	if err := e.Draw(0, 0, m.back); err != nil {
		return err
	}
	e.V.CopyBackToOff()
	e.AssignAutoFunction(g.drawMPSetupScreen, 1)
	e.SetMouseList(engine.MouseArrow)
	if err := g.addMPSetupScreenFields(); err != nil {
		return err
	}
	for done := false; !done; {
		e.MarkTime()
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		if backToLobby {
			backToLobby, r = false, m.nameF
		}
		for i := range m.radioField {
			if r == m.radioField[i] && m.radioOn[i] == 0 {
				g.settings[settingsMPLink] = byte(i + 1)
			}
		}
		switch {
		case r == m.cancelF:
			done, g.currentScreen = true, ScreenMainMenu
		case r == m.setupF:
			g.Log("Multi_Player_Screen_: the modem setup is not ported")
		case r == m.loadF:
			if done, err = g.multiPlayerLoad(); err != nil {
				return err
			}
		case r == m.startF:
			if g.settings[0x17] == 0 {
				copy(g.settings[0x17:0x1f], g.exe.bytes(mpGameName, 8))
				g.settings[0x1d] = byte(g.clockTicks()&0x1f) + 0x61
			}
			done, g.currentScreen = true, ScreenNewGame
		case r == m.joinF:
			done, g.currentScreen = true, 0x16
		case r == m.nameF:
			// port-only: the online lobby. A game made there is created
			// as START does (loaded as LOAD does), one joined as JOIN
			// does; otherwise back here.
			j, load, err := g.lobbyScreen()
			if err != nil {
				return err
			}
			done, g.currentScreen = true, ScreenMultiplayer
			switch {
			case j == nil:
			case load:
				g.playLobbyGame(j)
				// The save picker opens over this screen, not the lobby's
				// (its boxes keep the back page under them).
				e.V.SetPageBack()
				if err := e.Draw(0, 0, m.back); err != nil {
					return err
				}
				if err := g.drawMPSetupScreen(); err != nil {
					return err
				}
				e.V.CopyBackToOff()
				if err := e.TogglePages(); err != nil {
					return err
				}
				if done, err = g.multiPlayerLoad(); err != nil {
					return err
				}
				if !done {
					// No save picked: back to the lobby.
					done, g.currentScreen = true, ScreenMultiplayer
				}
			case j.NetId == 1:
				g.playLobbyGame(j)
				copy(g.settings[0x17:0x1f], g.exe.bytes(mpGameName, 8))
				g.settings[0x1d] = byte(g.clockTicks()&0x1f) + 0x61
				g.currentScreen = ScreenNewGame
			default:
				g.playLobbyGame(j)
				g.currentScreen = 0x16
			}
		}
		if done {
			break
		}
		if err := g.addMPSetupScreenFields(); err != nil {
			return err
		}
		e.V.SetPageOff()
		if err := g.drawMPSetupScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.ReleaseTime(2); err != nil {
			return err
		}
	}
	e.ClearFields()
	e.PutAutoFunction(saved)
	e.ClearFields()
	g.mpSetup = nil
	e.DeactivateHelpList()
	return g.saveGameSettings()
}
