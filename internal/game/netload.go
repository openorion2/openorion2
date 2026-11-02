package game

import (
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// Loading a multiplayer game (NETSTART.C and the screens of module 111):
// the multiplayer screen's LOAD, the saved game list, the creator's lobby
// (Load_Net_Screen_), the empire picks (Choose_Network_Plyrs_Screen_,
// msgs 0x20, 0x21) and the joiner's side (Net_Pick_Position_). Ported from
// the assembly. Spec: docs/net/leaving.md, "Loading".

const (
	ScreenLoadNet = 0x29 // Load_Net_Screen_

	netPicking      = 0x1aaf59 // i32: the net id asked to pick an empire (msg 0x20), -1 none
	netLoadNameDef  = 0x17a0e0 // "MooII ": the game name when none was entered
	netReallyBeginS = 165      // RSTRING: "Really begin game? (AI will take over unpicked human slots)"
	netDateFmt      = 0x17a188 // "%s"
)

// getValidMPSaveGames is Get_Valid_MP_Save_Games_ (0xf33ae): each slot's
// game type when it is 1..3, else 0; whether any is.
func (g *Game) getValidMPSaveGames(valid []byte) bool {
	found := false
	for i := 0; i < 10; i++ {
		t := g.gameTypeFromSavedGame(i)
		if t >= 1 && t <= 3 {
			found = true
			valid[i] = t
		} else {
			valid[i] = 0
		}
	}
	return found
}

// multiPlayerLoad is Multi_Player_Screen_'s LOAD (0xf5086..0xf5143): the
// saved game list; a loaded game goes to Hotseat_Select_Player_ (game
// type 1) or Load_Net_Screen_. done ends the multiplayer screen.
func (g *Game) multiPlayerLoad() (done bool, err error) {
	var valid [10]byte
	g.currentScreen = ScreenMultiplayer
	ok := g.getValidMPSaveGames(valid[:])
	if !ok {
		if err := g.warningBox(g.hMessage(0xf4)); err != nil {
			return false, err
		}
		return false, nil
	}
	if g.settings[settingsMPLink] == 3 {
		yes, err := g.confirmationBox(g.hMessage(0x1c))
		if err != nil || !yes {
			return false, err
		}
	}
	g.currentScreen = ScreenLoadNet
	if err := g.saveGameSettings(); err != nil {
		return false, err
	}
	g.e.ClearFields()
	r, err := g.loadMultiPlayerGameScreen()
	if err != nil {
		return false, err
	}
	if r > -1 && r < 10 {
		g.continueLobbyGame(int(r)) // port-only
		g.currentScreen = ScreenLoadNet
		if g.gameType == save.GameHotseat {
			g.currentScreen = ScreenHotseat
		}
	} else {
		if err := g.loadMPSetupPictures(); err != nil {
			return false, err
		}
		g.currentScreen = ScreenMultiplayer
	}
	return true, nil
}

// changeMPGameName is Change_MP_Game_Name_ (0xf5777): the network game's
// name (settings+0x17, 8 characters), asked again while a listed game has it.
func (g *Game) changeMPGameName() error {
	e := g.e
	count := min(int(g.netGetGameCount()), 10)
	for {
		e.V.SetPageBack()
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := e.Draw(0, 0, g.mpSetup.back); err != nil {
			return err
		}
		e.V.CopyBackToOff()
		if err := e.TogglePages(); err != nil {
			return err
		}
		e.AssignAutoFunction(func() error { return nil }, 1) // Null_Autofunction_
		title := g.hMessage(0xfe)
		name := []byte(save.CString(g.settings[0x17:0x1f]))
		// Star_Name_Popup_Screen_Center_X_/_Y_: 0xb1, 0x7d.
		if _, err := g.inputBoxPopup(&name, 8, 0, title, 0xb1, 0x7d, true); err != nil {
			return err
		}
		strcpyField(g.settings[0x17:0x20], string(name))
		ok := true
		for i := 0; i < count && ok; i++ {
			cur := save.CString(g.settings[0x17:0x20])
			if cur == g.netGetGameName(int32(i)) {
				if err := g.warningBox(cSprintf(g.hMessage(0xff), cur)); err != nil {
					return err
				}
				ok = false
			}
		}
		if ok {
			break
		}
	}
	return g.saveSessionRelatedSettings()
}

// loadLoadMPGameScreen is Load_Load_MP_Game_Screen_ (0xf41ad).
func (g *Game) loadLoadMPGameScreen() error {
	g.e.MoxUpdate()
	m := &netScreen{}
	g.netScr = m
	var err error
	if m.back, err = g.picture(multiGMLBX, 0); err != nil {
		return err
	}
	if err := g.loadMPGameBox(); err != nil {
		return err
	}
	if m.cancelPic, err = g.picture(multiGMLBX, 0x101); err != nil {
		return err
	}
	w, h := 0, 0
	if m.listBox != nil {
		w, h = m.listBox.Width, m.listBox.Height
	}
	m.x = (0x280 - w) / 2
	m.hlStep, m.hl = 1, 0
	m.y = (0x1e0-h-0x51)/2 + 0x25
	return nil
}

// loadMPGameBox is the box both Load_Load_MP_Game_Screen_ and
// Draw_Load_MP_Game_Screen_ reload by game type (MULTIGM.LBX 0x12, 0xfe,
// 0xff); another type keeps the old one.
func (g *Game) loadMPGameBox() error {
	entry := -1
	switch g.gameType {
	case 2:
		entry = 0x12
	case 3:
		entry = 0xfe
	case 1:
		entry = 0xff
	}
	if entry < 0 {
		return nil
	}
	p, err := g.picture(multiGMLBX, entry)
	if err != nil {
		return err
	}
	g.netScr.listBox = p
	return nil
}

// addLoadMPGameScreenFields is Add_Load_MP_Game_Screen_Fields_ (0xefe7a):
// the ten rows and CANCEL; false (after a warning) without any
// multiplayer save.
func (g *Game) addLoadMPGameScreenFields() (bool, error) {
	e, m := g.e, g.netScr
	ok := g.getValidMPSaveGames(m.valid[:])
	if !ok {
		if err := g.warningBox(g.hMessage(0xf4)); err != nil {
			return false, err
		}
		return false, nil
	}
	var err error
	for i, dy := 0, 0x40; i < 10; i, dy = i+1, dy+0x1b {
		y := m.y + dy
		if m.rows[i], err = e.AddHiddenField(m.x+0x26, y, m.x+0x190, y+0x16, "", 0x29); err != nil {
			return false, err
		}
	}
	if m.listBox, err = g.picture(multiGMLBX, 0x12); err != nil {
		return false, err
	}
	m.cancelF, err = e.AddButtonField(m.x+0xbf, m.y+0x158, "", m.cancelPic, cStringAt(g.exe, mpKeyCancel), 0x28)
	return true, err
}

// drawLoadMPGameScreen is Draw_Load_MP_Game_Screen_ (0xf1ce6): the box,
// the green palette and each slot's description, stardate, file date and
// game type icon; the valid row under the pointer cycles its colours.
func (g *Game) drawLoadMPGameScreen() error {
	e, m, gp, s := g.e, g.netScr, &g.gp, &g.state
	if m.first || g.fullDrawScreen {
		m.first, g.fullDrawScreen = false, false
		var err error
		if m.back, err = g.picture(multiGMLBX, 0); err != nil {
			return err
		}
		if err := e.Draw(0, 0, m.back); err != nil {
			return err
		}
		e.V.CopyOffToBack()
	}
	if err := g.loadMPGameBox(); err != nil {
		return err
	}
	if m.listBox != nil {
		if err := e.Draw(m.x, m.y, m.listBox); err != nil {
			return err
		}
	}
	g.setGreenPalColors()
	t := e.Text()
	for i, dy := 0, 0x40; i < 10; i, dy = i+1, dy+0x1b {
		y := m.y + dy + 2
		x := m.x + 0x26
		removeEmbeddedSpecialCodes(s.Descriptions[i][:])
		sd, date := gp.stardateStr[i], gp.dates[i]
		colors := g.fontColors2(m.style, 0x90, 0x93)
		hl := false
		if m.valid[i] != 0 {
			if m.scanned == m.rows[i] {
				colors = g.fontColors2(m.style, 0x90, 0x99)
				hl = true
				m.hl += m.hlStep
			} else {
				colors = g.fontColors2(m.style, 0x90, 0x97)
			}
		}
		t.SetStyle(m.style, colors)
		if hl {
			g.increaseFontColorRange(int(m.hl))
			if m.hl >= 4 {
				m.hl, m.hlStep = 4, -1
			} else if m.hl <= -2 {
				m.hl, m.hlStep = -2, 1
			}
			m.hl += m.hlStep
		}
		desc := save.CString(s.Descriptions[i][:])
		if s.Descriptions[i][0] == 0 {
			desc = g.hMessage(0x184)
		}
		if err := t.Print(x+9, y+(0x16-t.Height())/2, desc); err != nil {
			return err
		}
		t.SetStyle(1, colors)
		if err := t.Print(x+0xf3, y, sd); err != nil {
			return err
		}
		if err := t.Print(x+0xf3, y+2+t.Height(), date); err != nil {
			return err
		}
		if s.Descriptions[i][0] == 0 {
			continue
		}
		icon := 0
		switch m.valid[i] {
		case 0:
			icon = 0x102
		case 1:
			icon = 0x103
		case 2, 3:
			icon = 0x104
		}
		if icon > 0 {
			p, err := g.picture(multiGMLBX, icon)
			if err != nil {
				return err
			}
			if err := e.Draw(x+0x17c, y+2+t.Height(), p); err != nil {
				return err
			}
		}
	}
	return nil
}

// setGreenPalColors is the copy of _green_pal_colors (0x183580, 64 bytes)
// into current_palette+0x240 the list screens draw with: 16 colours
// 0x90..0x9f of {changed, R, G, B}, as current_palette keeps them (as the
// game list's, compared with the original: docs/net/ui.md).
func (g *Game) setGreenPalColors() {
	green := g.exe.bytes(greenPalColors, 0x40)
	for i := range 0x10 {
		b := green[4*i:]
		g.e.V.Palette[0x90+i] = engine.Color{Changed: b[0], R: b[1], G: b[2], B: b[3]}
	}
}

// loadMultiPlayerGameScreen is Load_Multi_Player_Game_Screen_ (0xf4760):
// the slot picked and loaded (the game type stays the one chosen on the
// multiplayer screen, every net id -1), or -1.
func (g *Game) loadMultiPlayerGameScreen() (int, error) {
	e, n := g.e, &g.net
	if g.gameType == 2 {
		// port-only: a lobby game is named by its description, not asked.
		if !n.lobbyPlaying {
			if err := g.changeMPGameName(); err != nil {
				return 0, err
			}
		}
		n.name = [10]byte{}
		copy(n.name[:], save.CString(g.settings[0x17:0x20]))
		if n.name[0] == 0 {
			copy(n.name[:7], g.exe.bytes(netLoadNameDef, 7))
			n.name[5] = byte(g.clockTicks()%0x19) + 0x61
		}
	}
	saved := e.GetAutoFunction()
	e.ClearFields()
	e.SetFieldOutlineColor(0xf0)
	if err := g.loadLoadMPGameScreen(); err != nil {
		return 0, err
	}
	m := g.netScr
	e.AssignAutoFunction(g.drawLoadMPGameScreen, 1)
	m.style = 2
	g.getSavedGameDescriptions()
	g.getStarDateStrings()
	g.getSaveGameDateStrings()
	ok, err := g.addLoadMPGameScreenFields()
	if err != nil {
		return 0, err
	}
	if !ok {
		// The original returns here without putting the auto function back.
		return -1, nil
	}
	m.first = true // Fade_Into_Screen_'s flag
	if err := g.fadeIntoScreen(g.drawLoadMPGameScreen, 0xb); err != nil {
		return 0, err
	}
	r := -1
	for done := false; !done; {
		e.MarkTime()
		in, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		if m.scanned, err = e.ScanInput(); err != nil {
			return 0, err
		}
		for i := 0; i < 10; i++ {
			if in <= 0 || in != m.rows[i] {
				continue
			}
			if m.valid[i] == 0 {
				r = -1
				if err := g.warningBox(cSprintf(g.hMessage(0xfd), i)); err != nil {
					return 0, err
				}
				continue
			}
			gt := g.gameType
			ok, err := g.loadGame(i)
			if err != nil {
				return 0, err
			}
			done = true
			if !ok {
				r = -1
				continue
			}
			g.gameType = gt
			for k := 0; k < int(g.state.NumPlayers); k++ {
				g.state.Players[k].NetID = -1
			}
			r = i
		}
		if in == m.cancelF {
			r = -1
			g.currentScreen = ScreenMultiplayer
			done = true
		}
		if !done {
			e.V.SetPageOff()
			if err := g.drawLoadMPGameScreen(); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
		}
	}
	e.PutAutoFunction(saved)
	e.ClearFields()
	e.SetInputDelay(3)
	return r, nil
}

// loadChooseNetPlyrsScreen is Load_Choose_Net_Plyrs_Screen_ (0xf3fc6).
func (g *Game) loadChooseNetPlyrsScreen() error {
	g.e.MoxUpdate()
	mp := &mpScreen{first: true}
	g.hs.mp = mp
	var err error
	for _, p := range []struct {
		dst   **gfx.Picture
		entry int
	}{{&mp.back, 0}, {&mp.top, 0x1b}, {&mp.mid, 0x1c}, {&mp.bottom, 0x1d}, {&mp.startPic, 0x10}, {&mp.cancelPic, 0x101}} {
		if *p.dst, err = g.picture(multiGMLBX, p.entry); err != nil {
			return err
		}
	}
	mp.nValid = int16(g.nRemainingHumans())
	return nil
}

// addChooseNetPlyrsFields is Add_Choose_Net_Plyrs_Fields_ (0xefb50): a
// name and a banner field per human empire not taken yet; START when the
// screen may start the game.
func (g *Game) addChooseNetPlyrsFields() error {
	e, s, mp := g.e, &g.state, g.hs.mp
	e.ClearFields()
	for k := range mp.nameField {
		mp.nameField[k] = -1000
	}
	row, yoff := 0, 0x40
	for k := 0; k < int(s.NumPlayers); k++ {
		pl := &s.Players[k]
		if pl.Objective != save.ObjectiveHuman || mp.chosen[k] != 0 || pl.Eliminated != 0 {
			mp.nameField[k], mp.bannerField[k] = -1000, -1000
			continue
		}
		y := int(mp.y) + yoff
		f, err := e.AddHiddenField(int(mp.x)+0x6a, y, int(mp.x)+0x1b3, y+0x1d, "", 0x29)
		if err != nil {
			return err
		}
		mp.nameField[k] = f
		yoff += 0x24
		if err := g.addBannerField(k, row); err != nil {
			return err
		}
		row++
	}
	var err error
	if mp.top, err = g.picture(multiGMLBX, 0x1b); err != nil {
		return err
	}
	h := int16((int(mp.nValid)-1)*mp.mid.Height + mp.top.Height + mp.bottom.Height)
	mp.startF = -1000
	if mp.canStart {
		mp.startF, err = e.AddButtonField(int(mp.x), int(mp.y+h+1), "", mp.startPic, cStringAt(g.exe, mpKeyStart), 0x28)
	}
	return err
}

// drawChooseNetPlyrsScreen is Draw_Choose_Net_Plyrs_Screen_ (0xf166e): the
// animated top, a middle per human, the bottom, and each human empire's
// line: taken ones dim, the others bright with their banner, the one
// under the pointer cycling.
func (g *Game) drawChooseNetPlyrsScreen() error {
	e, s, mp := g.e, &g.state, g.hs.mp
	x, y := int(mp.x), int(mp.y)
	var err error
	if mp.first {
		e.V.SetPageOff()
		if mp.back, err = g.picture(multiGMLBX, 0); err != nil {
			return err
		}
		if err := e.Draw(0, 0, mp.back); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		e.V.SetRefreshStencil()
		mp.first = false
	}
	if mp.top, err = g.picture(multiGMLBX, 0x1b); err != nil {
		return err
	}
	mp.frame++
	for i := 0; i < int(mp.frame); i++ {
		g.setAnimationFrame(mp.top, i)
		if err := e.Draw(x, y, mp.top); err != nil {
			return err
		}
	}
	if int(mp.frame) >= mp.top.Frames {
		mp.frame = 0
	}
	y += mp.top.Height
	if mp.mid, err = g.picture(multiGMLBX, 0x1c); err != nil {
		return err
	}
	for i := 0; int(mp.nValid)-1 > i; i++ {
		if err := e.Draw(x, y, mp.mid); err != nil {
			return err
		}
		y += mp.mid.Height
	}
	if mp.bottom, err = g.picture(multiGMLBX, 0x1d); err != nil {
		return err
	}
	if err := e.Draw(x, y, mp.bottom); err != nil {
		return err
	}
	t := e.Text()
	row, yoff := 0, 0x40
	for k := 0; k < int(s.NumPlayers); k++ {
		pl := &s.Players[k]
		if pl.Objective != save.ObjectiveHuman || pl.Eliminated != 0 {
			continue
		}
		px, py := int(mp.x)+0x6a, int(mp.y)+yoff
		bright, dim, _ := g.getPlayerColors(pl.Color)
		banner := false
		if mp.chosen[k] != 0 {
			t.SetStyle(int(mp.fontMode), dim)
		} else {
			banner = true
			if mp.input == mp.nameField[k] || mp.input == mp.bannerField[k] {
				c0 := bright[0]
				for j := 1; j < 8; j++ {
					bright[j] += byte(mp.cyclePos)
					if int(bright[j]) < int(c0)+1 {
						bright[j] = c0 + 1
					}
					if int(c0)+5 < int(bright[j]) {
						bright[j] = c0 + 5
					}
				}
				mp.cyclePos += mp.cycleDir
				if mp.cyclePos >= 2 {
					mp.cyclePos, mp.cycleDir = 2, -1
				} else if mp.cyclePos <= -1 {
					mp.cyclePos, mp.cycleDir = -1, 1
				}
			}
			t.SetStyle(int(mp.fontMode), bright)
		}
		str := fmt.Sprintf("%s (%s)", save.CString(pl.EmperorName[:]), g.raceName(k))
		if err := t.Print(px, py+(0x1d-t.Height())/2, str); err != nil {
			return err
		}
		if banner {
			if err := g.drawMultiPlayerBanner(k, row, false); err != nil {
				return err
			}
		}
		yoff += 0x24
		row++
	}
	return nil
}

// chooseNetworkPlyrsScreen is Choose_Network_Plyrs_Screen_ (0xf0e17): the
// human empires of a loaded game; a click on one not taken picks it (*pos).
// With canStart, START (after a confirmation) returns 1; else 0.
func (g *Game) chooseNetworkPlyrsScreen(pos *int16, taken [8]byte, canStart, load bool) (int, error) {
	e, s := g.e, &g.state
	saved := e.GetAutoFunction()
	if load {
		if err := g.loadChooseNetPlyrsScreen(); err != nil {
			return 0, err
		}
	}
	mp := g.hs.mp
	mp.nValid = int16(g.nRemainingHumans())
	var err error
	if mp.top, err = g.picture(multiGMLBX, 0x1b); err != nil {
		return 0, err
	}
	mp.x = int16((0x280 - mp.top.Width) / 2)
	h := int16(mp.mid.Height)*mp.nValid - 1 + int16(mp.top.Height) + int16(mp.bottom.Height)
	mp.cyclePos, mp.cycleDir = 0, 1
	mp.y = (0x1e0 - h) / 2
	if err := g.addChooseNetPlyrsFields(); err != nil {
		return 0, err
	}
	e.AssignAutoFunction(g.drawChooseNetPlyrsScreen, 1)
	mp.fontMode = 4
	mp.first = true // Fade_Into_Screen_'s flag
	if err := g.fadeIntoScreen(g.drawChooseNetPlyrsScreen, 0xb); err != nil {
		return 0, err
	}
	mp.canStart = canStart
	mp.chosen = taken
	sel, result := false, 0
	for !sel {
		e.MarkTime()
		in, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		if mp.input, err = e.ScanInput(); err != nil {
			return 0, err
		}
		for k := 0; k < int(s.NumPlayers) && !sel; k++ {
			if (in == mp.nameField[k] || in == mp.bannerField[k]) && mp.chosen[k] == 0 {
				sel = true
				*pos = int16(k)
			}
		}
		if mp.canStart && in == mp.startF {
			yes, err := g.confirmationBox(g.hMessage(0xf5))
			if err != nil {
				return 0, err
			}
			if yes {
				sel, result = true, 1
			} else {
				e.V.SetPageBack()
				e.V.Fill(0, 0, 0x27f, 0x1df, 0)
				e.V.CopyBackToOff()
				if err := e.TogglePages(); err != nil {
					return 0, err
				}
				if err := g.addChooseNetPlyrsFields(); err != nil {
					return 0, err
				}
			}
		}
		if !sel {
			e.V.SetPageOff()
			if err := g.drawChooseNetPlyrsScreen(); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
		}
		if err := e.ReleaseTime(2); err != nil {
			return 0, err
		}
		// The loop also ends on the word at 0x199a0e, which nothing here sets.
	}
	e.PutAutoFunction(saved)
	e.ClearFields()
	e.SetInputDelay(3)
	g.hs.mp = nil
	return result, nil
}

// sendPickYourPosition is Send_Pick_Your_Position_ (0xfa05d): msg 0x20 to
// net id to, NUM_PLAYERS and every player record.
func (g *Game) sendPickYourPosition(to int32) {
	s := &g.state
	buf := binary.LittleEndian.AppendUint16(nil, uint16(s.NumPlayers))
	for k := 0; k < int(s.NumPlayers); k++ {
		buf = append(buf, g.playerRecord(k)...)
	}
	g.moxSendMessage(to, 0x20, buf)
}

// decodePickYourPosition is Decode_Pick_Your_Position_ (0xfa0d3): what case
// 0x20 does inline. Nothing calls it.
func (g *Game) decodePickYourPosition(buf []byte) {
	s := &g.state
	s.NumPlayers = int16(binary.LittleEndian.Uint16(buf))
	for k := 0; k < int(s.NumPlayers); k++ {
		setRec(&s.Players[k], buf[2+k*netPlayerSize:2+(k+1)*netPlayerSize])
	}
}

// loadNetScreen is Load_Net_Screen_ (0xfb27b), screen 0x29: the creator of
// a loaded network game picks his empire, asks each joiner to pick (msg
// 0x20), makes the unpicked humans AIs, sends the net ids (msg 0x2a) and the
// full broadcast.
func (g *Game) loadNetScreen() (bool, error) {
	e, n, s := g.e, &g.net, &g.state
	n.set8(netState, 0x24)
	g.netInitHostElection()
	defer func() { n.fadeIn = false }()
	if err := g.reloadInitializingNetInfo(); err != nil {
		return false, err
	}
	// Windows_Check_ (0xfb2c8) is false: a DOS box under Windows only.
	conn := int32(n.u8(netConnType))
	e.MoxUpdate()
	ok := g.netInit(conn, 0x17, g.receiveMessage)
	e.MoxUpdate()
	if ok == 0 {
		if err := g.netFadeInPending(); err != nil {
			return false, err
		}
		err := g.messageBox(g.netRString(netCommFailS, "Comm failure"))
		n.set8(netState, 0)
		g.currentScreen = ScreenMultiplayer
		return false, err
	}
	if conn == 0 {
		// port-only: the listening address before Net_Create_Game_.
		if go_, err := g.netCreateGameScreen(); err != nil || !go_ {
			if err == nil {
				n.set8(netState, 0)
				g.currentScreen = ScreenMultiplayer
				g.netUninit()
			}
			return false, err
		}
		if g.russNetCreateGame(0x17, g.gameName(), 8, "", g.netName()) == 0 {
			if err := g.netFadeInPending(); err != nil {
				return false, err
			}
			err := g.messageBox(g.netRString(netCommFailS, "Comm failure"))
			n.set8(netState, 0)
			g.currentScreen = ScreenMultiplayer
			g.netUninit()
			g.netClearNetIDs()
			return false, err
		}
	}
	e.ClearFields()
	if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0); err != nil {
		return false, err
	}
	e.AssignAutoFunction(func() error { return nil }, 2) // Null_Autofunction_
	n.set8(netActive, 1)
	n.set16(netOwnID, int16(g.netHostID()))
	n.set16(netGameHost, n.i16(netOwnID))
	n.set8(netNebulasOK, 1) // _nebula_screen_seg != 0
	var pos int16
	n.fadeIn = false // Choose_Network_Plyrs_Screen_ fades in by itself
	r, err := g.chooseNetworkPlyrsScreen(&pos, [8]byte{}, false, true)
	if err != nil {
		return false, err
	}
	e.ClearFields()
	if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0); err != nil {
		return false, err
	}
	if r == -1 {
		n.set8(netState, 0)
		g.currentScreen = ScreenMultiplayer
		return false, nil
	}
	s.Players[pos].NetID = n.i16(netOwnID)
	s.PlayerNum = pos
	n.set32(netPicking, -1)
	e.AssignAutoFunction(g.drawWaitForNewJoinersScreen, 2)
	if err := g.reloadWaitingForJoinersScreen(); err != nil {
		return false, err
	}
	e.ClearFields()
	if err := g.addWaitingForJoinersField(); err != nil {
		return false, err
	}
	n.set8(netHumans, 0)
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if s.Players[k].Objective == save.ObjectiveHuman {
			n.set8(netHumans, n.u8(netHumans)+1)
		}
	}
	asked := 1 // the humans placed or asked, this one included
	for done := false; !done; {
		if n.i32(netPicking) == -1 {
			var users [256]byte
			for k := g.netGetUsers(users[:]); k > 0 && n.i32(netPicking) == -1; {
				k--
				if g.netIDToPlayerID(int32(users[k])) != -1 {
					continue
				}
				asked++
				n.set32(netPicking, int32(users[k]))
				g.sendPickYourPosition(int32(users[k]))
			}
		}
		if err := g.drawWaitForNewJoinersScreen(); err != nil {
			return false, err
		}
		if err := e.TogglePages(); err != nil {
			return false, err
		}
		if n.i32(netPicking) == -1 {
			humans := int(n.u8(netHumans))
			switch {
			case asked == humans:
				done = true
			case asked == 2 && (conn == 1 || conn == 2):
				done = true
			case int32(asked) == g.netGetUsers(nil):
				in, err := e.GetInput()
				if err != nil {
					return false, err
				}
				if in == g.netScr.startF {
					if asked >= humans {
						done = true
					} else if done, err = g.confirmationBox(g.netRString(netReallyBeginS,
						"Really begin game? (AI will take over unpicked human slots)")); err != nil {
						return false, err
					}
				}
			}
		}
		e.MoxUpdate()
		if err := g.drawWaitForNewJoinersScreen(); err != nil {
			return false, err
		}
		if err := e.TogglePages(); err != nil {
			return false, err
		}
	}
	if conn == 0 || conn == 3 {
		g.netDisableNewPlayers()
	}
	return true, g.netStartLoadedGame()
}

// netStartLoadedGame is Load_Net_Screen_ once every human is placed: the
// unplaced become AIs, msg 0x2a and the full broadcast go out, the main
// screen (port-only split: a lost player's return ends the same way).
func (g *Game) netStartLoadedGame() error {
	e, n, s := g.e, &g.net, &g.state
	e.AssignAutoFunction(func() error { return nil }, 2) // Null_Autofunction_
	n.set8(netHumans, 0)
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		pl := &s.Players[k]
		if pl.NetID == -1 {
			if pl.Objective == save.ObjectiveHuman {
				g.makePlayerIntoAI(k)
			}
			continue
		}
		pl.Objective = save.ObjectiveHuman
		n.set8(netHumans, n.u8(netHumans)+1)
	}
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		g.doPlayerColonyCalculations(k)
	}
	if n.u8(netNebulasOK) == 0 {
		s.NumNebulas = 0
		for i := int(s.NumStars) - 1; i >= 0; i-- {
			s.Stars[i].InNebula = 0
		}
	}
	n.tempString = ""
	if err := g.reloadSendingDataInfo(); err != nil {
		return err
	}
	e.V.SetPageOff()
	if err := g.drawSendGetNetInfoScreen(); err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawSendGetNetInfoScreen, 2)
	if err := e.TogglePages(); err != nil {
		return err
	}
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		n.set8(netCombat+12*uint32(k), 4)
		n.set8(netCombatArg+12*uint32(k), 0xff)
	}
	msg := make([]byte, netMsg0Size)
	msg[0x28] = byte(s.NumPlayers)
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		binary.LittleEndian.PutUint32(msg[6+4*k:], uint32(int32(s.Players[k].NetID)))
	}
	date := g.netBuildDate(netDate)
	copy(msg[0x2e:], date)
	// The rest of the frame's 0x60 bytes is old stack.
	stale := []netSpan{{0, 6}, {6 + 4*int(s.NumPlayers), 0x28}, {0x29, 0x2e}, {0x2e + len(date) + 1, netMsg0Size}}
	g.moxSendMessageStale(netAllHumans, 0x2a, msg, stale)
	n.numNetPlayers = int32(n.u8(netHumans))
	n.set16(netLastHost, n.i16(netOwnID))
	if err := g.broadcastGameDataDifferential(true); err != nil {
		return err
	}
	n.set8(netGets22+uint32(s.PlayerNum), 1)
	for i := uint32(0); i < 8; i++ {
		n.set8(netSent36+i, 0)
	}
	n.set8(netState, 4)
	return g.netEnterMainScreen()
}

// netPickPosition is Net_Pick_Position_ (0xfac85): the joiner of a loaded
// game picks an empire the others have not (msg 0x21), then takes the
// creator's full broadcast; PLAYER_NUM is the human with its net id.
func (g *Game) netPickPosition() (bool, error) {
	e, n, s := g.e, &g.net, &g.state
	var taken [8]byte
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		taken[k] = byte(b2i(s.Players[k].NetID != -1))
	}
	var pos int16
	r, err := g.chooseNetworkPlyrsScreen(&pos, taken, false, true)
	if err != nil {
		return false, err
	}
	if r == -1 {
		g.moxSendMessage(int32(n.i16(netGameHost)), 0x21, binary.LittleEndian.AppendUint32(nil, 0xffffffff))
		return false, nil
	}
	e.ClearFields()
	if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0); err != nil {
		return false, err
	}
	n.set8(netState, 7)
	g.moxSendMessage(int32(n.i16(netGameHost)), 0x21, binary.LittleEndian.AppendUint32(nil, uint32(int32(pos))))
	return true, g.netTakeLoadedGame()
}

// netTakeLoadedGame is Net_Pick_Position_ after the pick: the creator's full
// broadcast, PLAYER_NUM, the main screen (port-only split, as above).
func (g *Game) netTakeLoadedGame() error {
	e, n, s := g.e, &g.net, &g.state
	e.AssignAutoFunction(g.drawSendGetNetInfoScreen, 2)
	if err := g.reloadGettingDataInfo(); err != nil {
		return err
	}
	n.tempString = g.netRString(netWaitingS, "waiting ...")
	for n.i16(netChunks) == 0 {
		if _, err := e.GetInput(); err != nil {
			return err
		}
		e.V.SetPageOff()
		if err := g.drawSendGetNetInfoScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	for n.i16(netRecvChunks) < n.i16(netChunks) {
		n.tempString = fmt.Sprintf(cFormat(g.netRString(netReceivingS, "Receiving chunk %d of %d")),
			int(n.i16(netRecvChunks))+1, int(n.i16(netChunks)))
		if _, err := e.GetInput(); err != nil {
			return err
		}
		if err := g.drawSendGetNetInfoScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	g.decodeBroadcastGameDataDifferential(n.recv, int(binary.LittleEndian.Uint32(n.mem[netRecvLen-netBase:])), true)
	for i := uint32(0); i < 8; i++ {
		n.set8(netSent36+i, 0)
	}
	n.recv = nil // Pop_Block_
	n.set16(netChunks, 0)
	n.tempString = ""
	n.set8(netHumans, 0)
	s.PlayerNum = -1
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if s.Players[k].Objective != save.ObjectiveHuman {
			continue
		}
		n.set8(netHumans, n.u8(netHumans)+1)
		if s.Players[k].NetID == n.i16(netOwnID) {
			s.PlayerNum = int16(k)
		}
	}
	n.set8(netState, 4)
	if err := g.loadNebulaPictures(); err != nil { // Init_New_Session_
		return err
	}
	return g.netEnterMainScreen()
}
