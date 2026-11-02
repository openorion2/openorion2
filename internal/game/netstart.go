package game

import (
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/lobby"
	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/save"
)

// The network lobby and the start of a new network game (NETSTART.C and
// the network screens of module 111), ported from the assembly. Spec:
// docs/net/lobby.md; the port-only screens: docs/net/ui.md.

const (
	ScreenStartNet = 0x15 // Start_Net_Screen_
	ScreenJoinNet  = 0x16 // Join_Net_Screen_

	netRaceFlagWait = 0x1ab0bf // u8: Unrequest_Race_ waits for msg 0x3f
	netNebulasOK    = 0x1ab0c1 // u8: every joiner can draw nebulas (msg 0x42)
	netRaceInfo     = 0x1aaf60 // u8[8]: the player's race record is in (msg 1)
	netNumNetPlay   = 0x1849e2 // num_net_players (kept in netGame)
	netGameName     = 0x1ab082 // the game's name (char[10] as Net_Get_Game_Name_)
	netDate         = 0x17a17c // "May 22 2006": the build date msgs 0 and 0x2a send
	netDateOwn      = 0x17a0f0 // "May 22 2006": the one a received date is compared with
	netWindowsS     = 158      // RSTRING: "Can't try DOS net game under Windows"
	netCommFailS    = 159      // "Comm failure"
	netGameFullS    = 160      // "Game is full (or nonexistent)"
	netReceivingS   = 161      // "Receiving chunk %d of %d"
	netNoSlotsS     = 163      // "Sorry, no more player slots available in that game!"
	netOnly1S       = 166      // "Really start with only 1 player?"
	netWaitingS     = 167      // "waiting ..."
	netVersionsS    = 152      // "Incompatible program versions (host %s, you %s)"
	netOneHostS     = 153      // "only 1 host allowed"
	netMsg0Size     = 0x60
	netNebulaSeg    = 0x006f5044 // _nebula_screen_seg in the oracle's runs (msg 0x42)
	netPlayerName   = "myname"   // the player name the original passes
)

// netScreen is the network screens' part of _g_mp_screen (0x192680 points
// to it), reset by Reload_Generic_Net_Info_.
type netScreen struct {
	back, anim  *gfx.Picture // +0xf, +0x2f
	startPic    *gfx.Picture // +0x8f
	cancelPic   *gfx.Picture // +0x73
	listBox     *gfx.Picture // +0x1b
	entry       int          // +0xf1: the animation's MULTIGM.LBX entry
	x, y        int          // +0xf3, +0xf5
	height      int          // +0x10c
	style       int          // +0x10e
	getting     bool         // +0x10f: the "getting" text position
	first       bool         // +0x1e0: draw the background once more
	frame       uint8        // +0x1ea
	startF      int          // +0xed
	cancelF     int          // +0xdf
	addrF       int          // port-only: the game list's ADDRESS (TCP)
	rows        [10]int      // +0xa7: the game list's rows
	valid       [10]byte     // +0xcb: the saves' multiplayer game types
	scanned     int          // +0x101
	hl, hlStep  int8         // +0x1e8, +0x1e9: the highlighted row's colour cycle
	allowChat   bool         // _allow_chat_mode (0x1aae9f)
	listRefresh int          // port-only: frames to the next game list refresh
}

// reloadGenericNetInfo is Reload_Generic_Net_Info_ (0xf53d7).
func (g *Game) reloadGenericNetInfo(entry int) error {
	g.net.tempString = ""
	m := &netScreen{entry: entry}
	g.netScr = m
	var err error
	if m.back, err = g.picture(multiGMLBX, 0); err != nil {
		return err
	}
	if m.anim, err = g.picture(multiGMLBX, entry); err != nil {
		return err
	}
	m.x = (0x280 - m.anim.Width) / 2
	m.style, m.first, m.height = 4, true, m.anim.Height
	m.y = (0x1e0 - m.height) / 2
	return nil
}

// reloadNetInfo is the Reload_*_Info_ wrappers (0xf53cb..0xf552a): the
// chat flag and the animation entry.
func (g *Game) reloadNetInfo(entry int, chat bool) error {
	if err := g.reloadGenericNetInfo(entry); err != nil {
		return err
	}
	g.netScr.allowChat = chat
	return nil
}

// reloadInitializingNetInfo is Reload_Initializing_Net_Info_ faded in, as the lobby screens
// open; TCP and the online lobby skip it, the next screen fades in instead
// (docs/net/ui.md).
func (g *Game) reloadInitializingNetInfo() error {
	e, n := g.e, &g.net
	_, tcp := n.network.(*netplay.TCP)
	_, lob := n.network.(*lobby.Network)
	if (tcp || lob) && n.u8(netConnType) == 0 {
		n.fadeIn = true
		e.AssignAutoFunction(g.drawNetFrame, 1)
		return nil
	}
	if err := g.reloadNetInfo(0x19, false); err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawJoinNetScreen, 1)
	if err := g.fadeIntoScreen(g.drawJoinNetScreen, 0xb); err != nil {
		return err
	}
	if err := g.drawJoinNetScreen(); err != nil {
		return err
	}
	return e.TogglePages()
}

// drawNetFrame is the multiplayer frame alone, behind a box or a message
// when the initializing screen was skipped (port-only).
func (g *Game) drawNetFrame() error {
	e := g.e
	e.V.SetPageOff()
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	p, err := g.picture(multiGMLBX, 0)
	if err != nil {
		return err
	}
	return e.Draw(0, 0, p)
}

// netFadeInto fades into the screen draw paints, in the initializing
// screen's palette 0xb, and clears fadeIn (port-only).
func (g *Game) netFadeInto(draw func() error) error {
	g.net.fadeIn = false
	return g.fadeIntoScreen(func() error {
		// draw may flip pages: the DAC stays dark until the fade, which
		// shows the screen draw left in off (its last flip may predate it).
		v := g.e.V
		v.ClearPaletteChanges(0, 0xff)
		if err := draw(); err != nil {
			return err
		}
		copy(v.Screen, v.Off)
		return nil
	}, 0xb)
}

// netFadeInPending fades the frame in if nothing has since the skipped
// initializing screen, so that a message box has its palette (port-only).
func (g *Game) netFadeInPending() error {
	if !g.net.fadeIn {
		return nil
	}
	return g.netFadeInto(g.drawNetFrame)
}

// reloadSendingDataInfo is Reload_Sending_Data_Info_ (0xf54d9).
func (g *Game) reloadSendingDataInfo() error {
	g.e.ClearFields()
	if err := g.reloadNetInfo(0x1a, false); err != nil {
		return err
	}
	g.netScr.getting = false
	return g.fadeIntoScreen(g.drawJoinNetScreen, 0xb)
}

// reloadGettingDataInfo is Reload_Getting_Data_Info_ (0xf54a0).
func (g *Game) reloadGettingDataInfo() error {
	if err := g.reloadNetInfo(0x1f, false); err != nil {
		return err
	}
	g.netScr.getting = true
	return nil
}

// reloadWaitingForJoinersScreen is Reload_Waiting_For_Joiners_Screen_
// (0xf552a): the START button too.
func (g *Game) reloadWaitingForJoinersScreen() error {
	g.e.ClearFields()
	if err := g.reloadNetInfo(0xf, true); err != nil {
		return err
	}
	m := g.netScr
	m.style = 4
	var err error
	m.startPic, err = g.picture(multiGMLBX, 0x10)
	return err
}

// drawNetAnimation is the animation every network screen draws: one more
// frame each time, all from the first (a fresh Buffer_Reload_ each draw).
func (g *Game) drawNetAnimation() error {
	m := g.netScr
	m.frame++
	p, err := g.picture(multiGMLBX, m.entry)
	if err != nil {
		return err
	}
	m.anim = p
	for i := 0; i < int(m.frame); i++ {
		g.setAnimationFrame(p, i)
		if err := g.e.Draw(m.x, m.y, p); err != nil {
			return err
		}
	}
	if p.Frames <= int(m.frame) {
		m.frame = 0
	}
	return nil
}

// drawNetBack is the Fill and MULTIGM.LBX 0 the screens start with.
func (g *Game) drawNetBack() error {
	g.e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	p, err := g.picture(multiGMLBX, 0)
	if err != nil {
		return err
	}
	g.netScr.back = p
	return g.e.Draw(0, 0, p)
}

// drawJoinNetScreen is Draw_Join_Net_Screen_ (0xf19c7).
func (g *Game) drawJoinNetScreen() error {
	e, m := g.e, g.netScr
	e.V.SetPageOff()
	if e.FadePercent() < 1 {
		if m.first {
			if err := g.drawNetBack(); err != nil {
				return err
			}
			e.V.CopyOffToBack()
			e.V.SetRefreshStencil()
			m.first = false
		}
	} else if err := g.drawNetBack(); err != nil {
		return err
	}
	return g.drawNetAnimation()
}

// netTextColors is the colours the screens print with: offset 0xf2,
// Font_Colors2_(4, 0x9c, 0xf8).
func (g *Game) netTextColors() {
	g.moxFontColorsOffset = 0xf2
	g.e.Text().SetStyle(g.netScr.style, g.fontColors2(4, 0x9c, 0xf8))
}

// drawWaitForNewJoinersScreen is Draw_Wait_For_New_Joiners_Screen_
// (0xf2de5): "n humans have joined the game" (H strings 0xf9 / 0xfa).
func (g *Game) drawWaitForNewJoinersScreen() error {
	e, m := g.e, g.netScr
	e.V.SetPageOff()
	if err := g.drawNetBack(); err != nil {
		return err
	}
	if err := g.drawNetAnimation(); err != nil {
		return err
	}
	n := g.netGetUsers(nil)
	id := 0xfa
	if int16(n) == 1 {
		id = 0xf9
	}
	msg := fmt.Sprintf(cFormat(g.hMessage(id)), int(int16(n)))
	g.netTextColors()
	if err := e.Text().Print(m.x+0x78, m.y+0x47, msg); err != nil {
		return err
	}
	if err := g.drawResumeText(); err != nil { // port-only: a lost player's return
		return err
	}
	return g.drawNetJoinersList() // port-only
}

// drawWaitForRaceInfoScreen is Draw_Wait_For_Race_Info_Screen_ (0xf2f4e):
// "have race info from n of m" (H string 0xfb).
func (g *Game) drawWaitForRaceInfoScreen() error {
	e, m, s := g.e, g.netScr, &g.state
	g.moxFontColorsOffset = 0xf2
	c := g.fontColors2(4, 0x9c, 0xf8)
	e.V.SetPageOff()
	if err := g.drawNetBack(); err != nil {
		return err
	}
	if err := g.drawNetAnimation(); err != nil {
		return err
	}
	k := 0
	for i := 0; i < int(s.NumPlayers); i++ {
		if g.net.u8(netRaceInfo+uint32(i)) != 0 {
			k++
		}
	}
	users := g.netGetUsers(nil)
	msg := fmt.Sprintf(cFormat(g.hMessage(0xfb)), int(int16(k)), int(users))
	e.Text().SetStyle(m.style, c)
	return e.Text().Print(m.x+0x76, m.y+0x49, msg)
}

// drawSendGetNetInfoScreen is Draw_SendGet_Net_Info_Screen_ (0xf2c8b):
// the animation and _temp_string.
func (g *Game) drawSendGetNetInfoScreen() error {
	e, m := g.e, g.netScr
	if err := g.drawNetBack(); err != nil {
		return err
	}
	if err := g.drawNetAnimation(); err != nil {
		return err
	}
	if g.net.tempString == "" {
		return nil
	}
	g.moxFontColorsOffset = 0xf2
	e.Text().SetStyle(4, g.fontColors2(4, 0x9c, 0xf8))
	x, y := m.x+0x72, m.y+0x42
	if m.getting {
		x, y = m.x+0x79, m.y+0x41
	}
	return e.Text().Print(x, y, g.net.tempString)
}

// addWaitingForJoinersField is Add_Waiting_For_Joiners_Field_ (0xf0801):
// START, hot key "S".
func (g *Game) addWaitingForJoinersField() error {
	m := g.netScr
	var err error
	m.startF, err = g.e.AddButtonField(m.x+0x9e, m.y+0x6a, "", m.startPic, cStringAt(g.exe, mpKeyStart), 0x28)
	return err
}

// netClearNetIDs is the loop the screens end with on failure: every
// player's net id to -1, and no network game set up.
func (g *Game) netClearNetIDs() {
	for k := range g.state.Players {
		g.state.Players[k].NetID = -1
	}
	g.net.set8(netActive, 0)
}

// netName is the player name for Net_Create_Game_ and Net_Join_Game_.
func (g *Game) netName() string {
	if g.net.player != "" {
		return g.net.player
	}
	return netPlayerName
}

// gameName is the game name at 0x1ab082.
func (g *Game) gameName() string { return save.CString(g.net.name[:]) }

// startNetScreen is Start_Net_Screen_ (0xfb7e5), screen 0x15: the
// creator's lobby, the race picks, Init_New_Game_ and the full broadcast.
func (g *Game) startNetScreen() (bool, error) {
	e, n, s := g.e, &g.net, &g.state
	n.set8(netState, 1)
	g.netInitHostElection()
	defer func() { n.fadeIn = false }()
	if err := g.reloadInitializingNetInfo(); err != nil {
		return false, err
	}
	fail := func(clear bool) (bool, error) {
		n.set8(netState, 0)
		if err := g.netFadeInPending(); err != nil {
			return false, err
		}
		err := g.messageBox(g.netRString(netCommFailS, "Comm failure"))
		g.currentScreen = ScreenMultiplayer
		if clear {
			g.netUninit()
			g.netClearNetIDs()
		}
		return false, err
	}
	conn := int32(n.u8(netConnType))
	e.MoxUpdate()
	ok := g.netInit(conn, 0x17, g.receiveMessage)
	e.MoxUpdate()
	if ok == 0 {
		return fail(false)
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
			return fail(true)
		}
	}
	n.set8(netActive, 1)
	n.set16(netOwnID, int16(g.netHostID()))
	n.set8(netNebulasOK, 1) // _nebula_screen_seg != 0
	n.set16(netGameHost, n.i16(netOwnID))
	if conn == 0 {
		if err := g.reloadWaitingForJoinersScreen(); err != nil {
			return false, err
		}
		e.ClearFields()
		if err := g.addWaitingForJoinersField(); err != nil {
			return false, err
		}
		e.AssignAutoFunction(g.drawWaitForNewJoinersScreen, 2)
		if n.fadeIn {
			// port-only: no box (-host): the waiting screen fades in instead.
			if err := g.netFadeInto(g.drawWaitForNewJoinersScreen); err != nil {
				return false, err
			}
		} else {
			if err := g.drawWaitForNewJoinersScreen(); err != nil {
				return false, err
			}
			if err := e.TogglePages(); err != nil {
				return false, err
			}
		}
		for {
			r, err := e.GetInput()
			if err != nil {
				return false, err
			}
			if err := g.drawWaitForNewJoinersScreen(); err != nil {
				return false, err
			}
			if err := e.TogglePages(); err != nil {
				return false, err
			}
			if r != 0 && g.netGetUsers(nil) == 1 {
				yes, err := g.confirmationBox(g.netRString(netOnly1S, "Really start with only 1 player?"))
				if err != nil {
					return false, err
				}
				r = 0
				if yes {
					r = 1
				}
			}
			if r != 0 || g.netGetUsers(nil) >= int32(g.settings[0xd5]) {
				break
			}
		}
		g.netDisableNewPlayers()
	}
	var users [256]byte
	k := g.netGetUsers(users[:])
	humans := uint8(k)
	if humans > g.settings[0xd5] {
		humans = g.settings[0xd5]
	}
	n.set8(netHumans, humans)
	n.set8(netState, 2)
	msg := make([]byte, netMsg0Size)
	s.PlayerNum = -1
	for i := int(humans); i > 0; i-- {
		id := users[i-1]
		s.Players[i-1].NetID = int16(id)
		binary.LittleEndian.PutUint32(msg[6+4*(i-1):], uint32(id))
		if uint16(id) == uint16(n.i16(netOwnID)) {
			s.PlayerNum = int16(i - 1)
		}
	}
	if s.PlayerNum == -1 {
		s.Players[0].NetID = n.i16(netOwnID)
		s.PlayerNum = 0
		binary.LittleEndian.PutUint32(msg[6:], uint32(int32(n.i16(netOwnID))))
	}
	s.NumPlayers = int16(g.settings[0xd5])
	binary.LittleEndian.PutUint32(msg[0:], uint32(int32(n.i16(netOwnID))))
	binary.LittleEndian.PutUint16(msg[4:], uint16(humans))
	msg[0x26] = g.settings[2]
	copy(msg[0x27:0x2d], g.settings[0xd4:0xda])
	msg[0x2d] = g.settings[0xd3]
	copy(msg[0x2e:], g.netBuildDate(netDate))
	// The rest of the 50-byte date buffer is the frame's old stack.
	stale := []netSpan{{0x2e + len(g.netBuildDate(netDate)) + 1, netMsg0Size}}
	for i := uint32(0); i < 8; i++ {
		n.set8(netRaceInfo+i, 0)
		n.set8(netSend2b+i, 0)
	}
	g.rsel.colorFlag = [8]bool{}
	g.rsel.raceFlag = [14]bool{}
	n.set8(netState, 3)
	for i := int(s.NumPlayers) - 1; i >= 0; i-- {
		s.Players[i].Color = 8
	}
	if _, err := g.raceSelectionScreen(&s.Players[s.PlayerNum]); err != nil {
		return false, err
	}
	n.set8(netSend2b+uint32(s.PlayerNum), 1)
	s.Players[s.PlayerNum].Objective = save.ObjectiveHuman
	e.MoxUpdate()
	g.moxSendMessageStale(netAllHumans, 0, msg, stale)
	e.ClearFields()
	if err := g.reloadNetInfo(0x18, true); err != nil {
		return false, err
	}
	if err := g.fadeIntoScreen(g.drawJoinNetScreen, 0xb); err != nil {
		return false, err
	}
	n.set8(netRaceInfo+uint32(s.PlayerNum), 1)
	for have := 0; have < int(n.u8(netHumans)); {
		if err := g.drawWaitForRaceInfoScreen(); err != nil {
			return false, err
		}
		have = 0
		for i := int(s.NumPlayers); i > 0; i-- {
			if n.u8(netRaceInfo+uint32(i-1)) != 0 {
				have++
			}
		}
		if _, err := e.GetInput(); err != nil {
			return false, err
		}
		if err := e.TogglePages(); err != nil {
			return false, err
		}
	}
	e.ClearFields()
	e.DeactivateAutoFunction()
	st := &g.settings
	if err := g.initNewGame(int16(st[0xd4]), int16(st[0xd6]), int16(st[0xd7]), int16(st[0xd5])); err != nil {
		return false, err
	}
	for i := int(n.u8(netHumans)); i < int(s.NumPlayers); i++ {
		s.Players[i].NetID = -1
	}
	// Banners still 8 get a random free one.
	var taken [8]bool
	for i := 0; i < int(s.NumPlayers); i++ {
		if c := s.Players[i].Color; c != 8 {
			taken[c] = true
		}
	}
	for i := 0; i < int(s.NumPlayers); i++ {
		if s.Players[i].Color != 8 {
			continue
		}
		pick, cnt := -1, 0
		for c := 7; c >= 0; c-- {
			if !taken[c] {
				cnt++
				if s.Random.Random(uint32(cnt)) == 1 {
					pick = c
				}
			}
		}
		s.Players[i].Color = uint8(pick)
		taken[pick] = true
	}
	if err := g.reloadSendingDataInfo(); err != nil {
		return false, err
	}
	e.V.SetPageOff()
	if err := g.drawSendGetNetInfoScreen(); err != nil {
		return false, err
	}
	e.AssignAutoFunction(g.drawSendGetNetInfoScreen, 2)
	if err := e.TogglePages(); err != nil {
		return false, err
	}
	if n.u8(netNebulasOK) == 0 {
		s.NumNebulas = 0
	}
	for i := int(s.NumPlayers) - 1; i >= 0; i-- {
		n.set8(netCombat+12*uint32(i), 4)
		n.set8(netCombatArg+12*uint32(i), 0xff)
	}
	n.numNetPlayers = int32(n.u8(netHumans))
	n.set16(netLastHost, n.i16(netOwnID))
	if err := g.broadcastGameDataDifferential(true); err != nil {
		return false, err
	}
	n.set8(netGets22+uint32(s.PlayerNum), 1)
	for i := uint32(0); i < 8; i++ {
		n.set8(netSent36+i, 0)
	}
	n.set8(netState, 4)
	return true, g.netEnterMainScreen()
}

// netEnterMainScreen is the end both net screens share: Init_Main_Screen_,
// Harold_Init_, Set_Defaults_, the main screen's fade and the ship icons.
func (g *Game) netEnterMainScreen() error {
	g.e.ClearFields()
	g.e.DeactivateAutoFunction()
	g.initMainScreenMap()
	g.haroldInit()
	g.setDefaults()
	g.ms.fadeInFlag = true
	g.updateShipIconsInBetweenTurns()
	g.netKeepTurnStart() // port-only: where a lost player's return goes back to
	return nil
}

// russNetCreateGame is Russ_Net_Create_Game_ (0xfaa71).
func (g *Game) russNetCreateGame(id int32, name string, players int32, password, player string) int32 {
	g.e.MoxUpdate()
	r := g.netCreateGame(id, name, players, password, player)
	g.e.MoxUpdate()
	return r
}

// joinNetScreen is Join_Net_Screen_ (0xfaedb), screen 0x16: pick a game,
// join it and wait for msg 0, then the race (a new game), or for msg 0x20
// and an empire (a loaded one, Net_Pick_Position_).
func (g *Game) joinNetScreen() (bool, error) {
	e, n := g.e, &g.net
	n.set8(netState, 5)
	g.netInitHostElection()
	defer func() { n.fadeIn = false }()
	if err := g.reloadInitializingNetInfo(); err != nil {
		return false, err
	}
	e.MoxUpdate()
	conn := int32(n.u8(netConnType))
	e.MoxUpdate()
	ok := g.netInit(conn, 0x17, g.receiveMessage)
	e.MoxUpdate()
	if ok == 0 {
		e.MoxUpdate()
		if err := g.netFadeInPending(); err != nil {
			return false, err
		}
		err := g.messageBox(g.netRString(netCommFailS, "Comm failure"))
		g.currentScreen = ScreenMultiplayer
		n.set8(netState, 0)
		return false, err
	}
	leave := func(msg string) (bool, error) {
		n.set8(netState, 0)
		g.currentScreen = ScreenMultiplayer
		g.netUninit()
		g.netClearNetIDs()
		if msg == "" {
			return false, nil
		}
		return false, g.messageBox(msg)
	}
	if conn == 0 {
		if err := g.chooseMultiNetworkGameScreen(); err != nil {
			return false, err
		}
		if n.name[0] == 0 {
			g.currentScreen = ScreenMultiplayer
			n.set8(netState, 0)
			return false, nil
		}
		if g.netJoinGame(g.gameName(), "", g.netName()) == 0 {
			n.set8(netState, 0)
			g.currentScreen = ScreenMultiplayer
			err := g.messageBox(g.netRString(netGameFullS, "Game is full (or nonexistent)"))
			g.netUninit()
			g.netClearNetIDs()
			return false, err
		}
	}
	n.set8(netActive, 1)
	n.set16(netOwnID, int16(g.netHostID()))
	if err := g.reloadNetInfo(0x17, true); err != nil {
		return false, err
	}
	e.ClearFields()
	if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0); err != nil {
		return false, err
	}
	e.AssignAutoFunction(g.drawJoinNetScreen, 2)
	g.rsel.colorFlag = [8]bool{}
	g.rsel.raceFlag = [14]bool{}
	for n.u8(netState) == 5 {
		if _, err := e.GetInput(); err != nil {
			return false, err
		}
		if err := g.drawJoinNetScreen(); err != nil {
			return false, err
		}
		if err := e.TogglePages(); err != nil {
			return false, err
		}
	}
	switch n.u8(netState) {
	case 0x2d:
		return leave(g.netRString(netNoSlotsS, "Sorry, no more player slots available in that game!"))
	case 6:
		var seg [4]byte
		binary.LittleEndian.PutUint32(seg[:], netNebulaSeg)
		g.moxSendMessage(int32(n.i16(netGameHost)), 0x42, seg[:])
		return g.netSelectRace()
	case 0x2c:
		var seg [4]byte
		binary.LittleEndian.PutUint32(seg[:], netNebulaSeg)
		g.moxSendMessage(int32(n.i16(netGameHost)), 0x42, seg[:])
		ok, err := g.netPickPosition()
		if err != nil || ok {
			return ok, err
		}
		return leave("")
	}
	return false, nil
}

// netSelectRace is Net_Select_Race_ (0xfaad8): the joiner's race, msg 1 to
// the creator, then the first broadcast.
func (g *Game) netSelectRace() (bool, error) {
	e, n, s := g.e, &g.net, &g.state
	if _, err := g.raceSelectionScreen(&s.Players[s.PlayerNum]); err != nil {
		return false, err
	}
	s.Players[s.PlayerNum].Objective = save.ObjectiveHuman
	if err := g.reloadNetInfo(0x1e, true); err != nil {
		return false, err
	}
	if err := g.fadeIntoScreen(g.drawJoinNetScreen, 0xb); err != nil {
		return false, err
	}
	if err := g.drawJoinNetScreen(); err != nil {
		return false, err
	}
	if err := e.TogglePages(); err != nil {
		return false, err
	}
	n.set16(netChunks, 0)
	n.set8(netState, 7)
	g.moxSendMessage(int32(n.i16(netGameHost)), 1, g.playerRecord(int(s.PlayerNum)))
	for n.i16(netChunks) == 0 {
		if _, err := e.GetInput(); err != nil {
			return false, err
		}
		if err := g.drawJoinNetScreen(); err != nil {
			return false, err
		}
		if err := e.TogglePages(); err != nil {
			return false, err
		}
	}
	if err := g.reloadGettingDataInfo(); err != nil {
		return false, err
	}
	g.net.tempString = g.netRString(netWaitingS, "waiting ...")
	e.V.SetPageOff()
	if err := g.drawSendGetNetInfoScreen(); err != nil {
		return false, err
	}
	e.AssignAutoFunction(g.drawSendGetNetInfoScreen, 2)
	for {
		if err := e.TogglePages(); err != nil {
			return false, err
		}
		if n.i16(netChunks) <= n.i16(netRecvChunks) {
			break
		}
		g.net.tempString = fmt.Sprintf(cFormat(g.netRString(netReceivingS, "Receiving chunk %d of %d")),
			int(n.i16(netRecvChunks))+1, int(n.i16(netChunks)))
		if err := g.drawSendGetNetInfoScreen(); err != nil {
			return false, err
		}
		if _, err := e.GetInput(); err != nil {
			return false, err
		}
	}
	g.decodeBroadcastGameDataDifferential(n.recv, int(binary.LittleEndian.Uint32(n.mem[netRecvLen-netBase:])), true)
	for i := uint32(0); i < 8; i++ {
		n.set8(netSent36+i, 0)
	}
	n.recv = nil
	n.set16(netChunks, 0)
	n.set8(netState, 4)
	if err := g.loadNebulaPictures(); err != nil { // Init_New_Session_
		return false, err
	}
	g.setDefaults()
	return true, g.netEnterMainScreen()
}

// requestBannerColor is Request_Banner_Color_ (0xfbe75): msg 0x2c to the
// creator, then wait for its decision (msg 0x2b).
func (g *Game) requestBannerColor(c uint8) (bool, error) {
	rs, n := &g.rsel, &g.net
	if rs.colorFlag[c] {
		return false, nil
	}
	g.moxSendMessage(int32(n.i16(netGameHost)), 0x2c, []byte{c})
	for !rs.colorFlag[c] {
		if err := g.netWait(); err != nil {
			return false, err
		}
	}
	return g.state.Players[g.state.PlayerNum].Color == c, nil
}

// requestRace is Request_Race_ (0xfbee1): msg 0x3a to the creator, then
// wait for its decision (msg 0x39).
func (g *Game) requestRace(r uint8) (bool, error) {
	rs, n, s := &g.rsel, &g.net, &g.state
	for n.u8(netRaceFlagWait) != 0 {
		if err := g.netWait(); err != nil {
			return false, err
		}
	}
	s.Players[s.PlayerNum].Race = 0xe
	if rs.raceFlag[r] {
		return false, nil
	}
	g.moxSendMessage(int32(n.i16(netGameHost)), 0x3a, []byte{r})
	for !rs.raceFlag[r] {
		if err := g.netWait(); err != nil {
			return false, err
		}
	}
	return s.Players[s.PlayerNum].Race == r, nil
}

// unrequestRace is Unrequest_Race_ (0xfbf76): msg 0x3e; the next request
// waits for the creator's msg 0x3f.
func (g *Game) unrequestRace(r uint8) {
	n := &g.net
	g.rsel.raceFlag[r] = false
	n.set8(netRaceFlagWait, 1)
	g.moxSendMessage(int32(n.i16(netGameHost)), 0x3e, []byte{r})
}

// netWait is the Mox_Update_ the request loops spin on. Port-only: the
// headless host gets a frame when a loop polls nothing else.
func (g *Game) netWait() error {
	g.e.MoxUpdate()
	g.e.Sys.KeyWaiting()
	if g.e.Sys.Closed() {
		return host.ErrQuit
	}
	return nil
}
