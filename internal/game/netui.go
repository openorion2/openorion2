package game

import (
	"fmt"
	"strconv"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/lobby"
	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/save"
)

// The game list (Choose_Multi_Network_Game_Screen_, ported) and the
// port-only address boxes of the TCP lobby (docs/net/ui.md).

const greenPalColors = 0x183580 // _green_pal_colors: 64 bytes into current_palette+0x240 (colours 0x90..0x9f)

// loadChooseMultiNetGameScreen is Load_Choose_Multi_Net_Game_Screen_ (0xf40d3).
func (g *Game) loadChooseMultiNetGameScreen() error {
	g.e.MoxUpdate()
	m := &netScreen{}
	g.netScr = m
	var err error
	if m.back, err = g.picture(multiGMLBX, 0); err != nil {
		return err
	}
	if m.listBox, err = g.picture(multiGMLBX, 0x29); err != nil {
		return err
	}
	if m.cancelPic, err = g.picture(multiGMLBX, 0x101); err != nil {
		return err
	}
	m.x = (0x280 - m.listBox.Width) / 2
	m.hl, m.hlStep = 0, 1
	m.y = (399-m.listBox.Height)/2 + 0x25
	return nil
}

// netListedGames is min(Net_Get_Game_Count_, 10).
func (g *Game) netListedGames() int { return int(min(g.netGetGameCount(), 10)) }

// addChooseMultiNetGameFields is Add_Choose_Multi_Net_Game_Fields_ (0xeff87).
func (g *Game) addChooseMultiNetGameFields() error {
	e, m := g.e, g.netScr
	n := g.netListedGames()
	for i := range m.rows {
		m.rows[i] = -1000
	}
	var err error
	for i, dy := 0, 0x40; i < n; i, dy = i+1, dy+0x1b {
		y := m.y + dy
		if m.rows[i], err = e.AddHiddenField(m.x+0x26, y, m.x+400, y+0x16, "", ')'); err != nil {
			return err
		}
	}
	if m.listBox, err = g.picture(multiGMLBX, 0x29); err != nil {
		return err
	}
	m.cancelF, err = e.AddButtonField(m.x+0xbf, m.y+0x158, "", m.cancelPic, cStringAt(g.exe, mpKeyCancel), ')')
	return err
}

// drawChooseMultiNetGameScreen is Draw_Choose_Multi_Net_Game_Screen_
// (0xf1af4): the list box, the green palette and the game names, the one
// under the pointer cycling its colours.
func (g *Game) drawChooseMultiNetGameScreen() error {
	e, m := g.e, g.netScr
	n := g.netListedGames()
	p, err := g.picture(multiGMLBX, 0x29)
	if err != nil {
		return err
	}
	m.listBox = p
	if err := e.Draw(m.x, m.y, p); err != nil {
		return err
	}
	// 0x40 bytes over current_palette+0x240: colours 0x90..0x9f, each
	// {changed, R, G, B} as current_palette keeps them.
	green := g.exe.bytes(greenPalColors, 0x40)
	for i := range 0x10 {
		b := green[4*i:]
		e.V.Palette[0x90+i] = engine.Color{Changed: b[0], R: b[1], G: b[2], B: b[3]}
	}
	t := e.Text()
	for i, dy := 0, 0x43; i < n; i, dy = i+1, dy+0x1b {
		name := g.netGetGameName(int32(i))
		if name == "" {
			continue
		}
		hl := m.scanned == m.rows[i]
		if hl {
			g.moxFontColorsOffset = 0x97
			t.SetStyle(m.style, g.fontColors2(m.style, 0x91, 0x99))
			g.increaseFontColorRange(int(m.hl))
			if m.hl >= 4 {
				m.hl, m.hlStep = 4, -1
			} else if m.hl < -2 {
				m.hl, m.hlStep = -3, 1
			}
			m.hl += m.hlStep
		} else {
			g.moxFontColorsOffset = 0x95
			t.SetStyle(m.style, g.fontColors2(m.style, 0x91, 0x97))
		}
		if err := t.Print(m.x+0x26+9, (0x16-t.Height())/2+m.y+dy, name); err != nil {
			return err
		}
	}
	return g.drawNetListAddress(0) // port-only
}

// chooseMultiNetworkGameScreen is Choose_Multi_Network_Game_Screen_
// (0xf0c8e): the joinable games; a pick copies its name to 0x1ab082,
// CANCEL clears it and leaves the network.
func (g *Game) chooseMultiNetworkGameScreen() error {
	e, n := g.e, &g.net
	saved := e.GetAutoFunction()
	e.ClearFields()
	e.SetInputDelay(3)
	e.SetFieldOutlineColor(0xf0)
	enter := func() error {
		e.V.SetPageBack()
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		back, err := g.picture(multiGMLBX, 0)
		if err != nil {
			return err
		}
		if err := e.Draw(0, 0, back); err != nil {
			return err
		}
		e.V.CopyBackToOff()
		if err := e.TogglePages(); err != nil {
			return err
		}
		e.AssignAutoFunction(g.drawChooseMultiNetGameScreen, 1)
		return nil
	}
	fade := n.fadeIn
	if !fade {
		if err := enter(); err != nil {
			return err
		}
	}
	if lob, ok := n.network.(*lobby.Network); ok {
		// port-only: the online lobby's game was picked there already.
		n.name = [10]byte{}
		copy(n.name[:9], lob.JoinedGame())
		e.PutAutoFunction(saved)
		e.ClearFields()
		e.SetInputDelay(3)
		return nil
	}
	if err := g.loadChooseMultiNetGameScreen(); err != nil {
		return err
	}
	m := g.netScr
	m.style = 4
	if fade {
		// port-only: no initializing screen (TCP), the list fades in.
		g.netUpdateGameList()
		m.listRefresh = 60
		if err := g.netFadeInto(func() error {
			if err := enter(); err != nil {
				return err
			}
			e.V.SetPageOff()
			return g.drawChooseMultiNetGameScreen()
		}); err != nil {
			return err
		}
	}
	for done := false; !done; {
		// port-only: IPX keeps the list current by itself; here it is
		// asked for again every second.
		if m.listRefresh--; m.listRefresh <= 0 {
			g.netUpdateGameList()
			m.listRefresh = 60
		}
		e.ClearFields()
		if err := g.addChooseMultiNetGameFields(); err != nil {
			return err
		}
		if err := g.addNetListAddressField(); err != nil {
			return err
		}
		e.MarkTime()
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		if m.scanned, err = e.ScanInput(); err != nil {
			return err
		}
		for i := 0; i < g.netListedGames(); i++ {
			if r > 0 && r == m.rows[i] {
				if name := g.netGetGameName(int32(i)); name != "" {
					n.name = [10]byte{}
					copy(n.name[:9], name)
					done = true
				}
			}
		}
		if r == m.cancelF {
			g.currentScreen = ScreenMultiplayer
			done = true
			g.netUninit()
			n.name[0] = 0
		}
		if r == m.addrF && !done {
			// port-only: another address, and the list asks again at once;
			// CANCEL keeps the address.
			e.V.SetPageOff()
			if err := g.drawNetListAddress(1); err != nil {
				return err
			}
			if err := e.TogglePages(); err != nil {
				return err
			}
			t, _ := g.netListAddress()
			ok, err := g.netJoinAddressBox(t)
			if err != nil {
				return err
			}
			if err := enter(); err != nil {
				return err
			}
			if ok {
				m.listRefresh = 0
			}
			continue
		}
		if done {
			break
		}
		e.V.SetPageOff()
		if err := g.drawChooseMultiNetGameScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	e.PutAutoFunction(saved)
	e.ClearFields()
	e.SetInputDelay(3)
	return nil
}

// port-only: the address boxes of the TCP lobby (docs/net/ui.md). They have
// no original screen, so they are built from the original's own art: the
// multiplayer frame (MULTIGM.LBX 0) with a heading, the input box popup of
// Change_MP_Game_Name_ (INBOX.LBX, Setup_Input_Box_Popup_ with remapping)
// holding the label and the line to edit, its ACCEPT as OK (also "O", or
// Enter in the line) and CANCEL, ACCEPT's art relabelled (Esc), apart.
type netAddressBox struct {
	heading, label string
	value          [66]byte // 64 characters (the lobby's longest password), the cursor, the NUL
	masked         bool     // the line shows '*' (the lobby's password)
	uni            *uniLine // the line is Unicode text (uniAddressScreen), not the engine's field
	uniBack        byte     // the line's background colour, under the Unicode text
	ok, cancel     int      // the port's fields on top of the popup's
}

const (
	nabX, nabY  = 0xb1, 0x7d // Star_Name_Popup_Screen_Center_X_/_Y_, as Change_MP_Game_Name_
	nabHeadingY = 0x5c       // port-only: the heading's baseline area above the box
	// ACCEPT and CANCEL (ACCEPT's art), 0x14 apart about the popup's centre.
	nabAcceptX, nabCancelX = nabX + 0x24, nabX + 0x9a
	// The popup's ACCEPT socket with its rim (x 0x60..0xc2, y 0x63..0x80),
	// and the plain metal left of it, the texture the old socket gets.
	nabSockX, nabSockY, nabSockW, nabSockH = 0x60, 0x63, 0x63, 0x1e
	nabMetalX, nabMetalW                   = 0x19, 0x46
)

// drawNetBoxSockets moves the ACCEPT socket drawn into the popup (INBOX.LBX
// 0) under the two buttons, metal filling the rest (port-only).
func (g *Game) drawNetBoxSockets() error {
	e, b := g.e, &g.rsel.box
	c := b.picture.NewCanvas()
	if err := b.picture.Draw(c, 0); err != nil {
		return err
	}
	if err := e.CalculatePictureRemapColors(b.picture); err != nil {
		return err
	}
	r, v, w := e.PictureRemap(), e.V, b.picture.Width
	ax, cx := nabAcceptX-nabX, nabCancelX-nabX
	for y := range nabSockH {
		row := c.Pix[(nabSockY+y)*w : (nabSockY+y+1)*w]
		for x := ax; x < cx+nabSockW; x++ {
			px := row[nabMetalX+(x-nabMetalX)%nabMetalW]
			switch {
			case x < ax+nabSockW:
				px = row[nabSockX+x-ax]
			case x >= cx:
				px = row[nabSockX+x-cx]
			}
			v.Current[(int(b.y)+nabSockY+y)*engine.W+int(b.x)+x] = r[px]
		}
	}
	if v.RefreshMode() > engine.RefreshMasked {
		v.AddSquare(int(b.x)+ax, int(b.y)+nabSockY, int(b.x)+cx+nabSockW-1, int(b.y)+nabSockY+nabSockH-1)
	}
	if err := e.CalculatePictureRemapColors(b.button); err != nil {
		return err
	}
	b.button.CurrentFrame = 0
	return g.remapDraw(int(b.buttonX), int(b.buttonY), b.button)
}

// drawLobbyBoxButtons draws the lobby's boxes' ACCEPT and CANCEL as the
// lobby's own metal buttons over the sockets, lit under the pointer;
// pressed is the one held down (1 ACCEPT, 2 CANCEL), 0 none (port-only).
func (g *Game) drawLobbyBoxButtons(pressed int) {
	b := &g.rsel.box
	w, h, y := b.button.Width, b.button.Height, int(b.buttonY)
	mx, my, _ := g.e.Sys.Mouse()
	for i, x := range []int{int(b.buttonX), nabCancelX} {
		label := []string{"ACCEPT", "CANCEL"}[i]
		state := buttonNormal
		switch {
		case pressed == i+1:
			state = buttonPressed
		case mx >= x && mx < x+w && my >= y && my < y+h:
			state = buttonHover
		}
		g.lobbyButtonIn(x, y, w, h, label, state)
	}
}

// drawNetAddressBack draws what lies under the popup: the frame and the
// heading (port-only).
func (g *Game) drawNetAddressBack(p *netAddressBox) error {
	e := g.e
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	back, err := g.picture(multiGMLBX, 0)
	if err != nil {
		return err
	}
	if err := e.Draw(0, 0, back); err != nil {
		return err
	}
	t := e.Text()
	g.moxFontColorsOffset = 0xf2
	t.SetStyle(4, g.fontColors2(4, 0x9c, 0xf8))
	return t.Print((0x280-t.Width([]byte(p.heading)))/2, nabHeadingY, p.heading)
}

// setupNetAddressBox is Setup_Input_Box_Popup_ over the port's back, plus
// the port's OK hot key and CANCEL (port-only).
func (g *Game) setupNetAddressBox(p *netAddressBox, back engine.AutoFunction) error {
	e, b := g.e, &g.rsel.box
	if err := g.setupInputBoxPopup(back); err != nil {
		return err
	}
	e.LongInputField(b.inputField) // addresses and game ids may be wider than the line
	if p.masked {
		e.MaskInputField(b.inputField)
	}
	if p.uni != nil {
		// The Unicode line is drawn by the box's loop: no engine field.
		e.ClearFields()
		bx, by := int(b.buttonX), int(b.buttonY)
		var err error
		if b.buttonField, err = e.AddHiddenField(bx, by, bx+b.button.Width, by+b.button.Height, "", 0x28); err != nil {
			return err
		}
		b.inputField = -1000
		p.uniBack = e.V.Back[(int(b.fieldY)+int(b.fieldH)/2)*engine.W+int(b.fieldX)+4]
	}
	var err error
	bx, by := int(b.buttonX), int(b.buttonY)
	if p.ok, err = e.AddHiddenField(bx, by, bx+b.button.Width, by+b.button.Height, "O", 0x28); err != nil {
		return err
	}
	for _, pg := range []func(){e.V.SetPageBack, e.V.SetPageOff} {
		pg()
		if err := g.drawNetBoxSockets(); err != nil {
			return err
		}
		if p.heading == lobbyHeading {
			g.drawLobbyBoxButtons(0)
			continue
		}
		if err := g.drawNetButton(&netAcceptArt, nabCancelX, by, 0, "CANCEL"); err != nil {
			return err
		}
	}
	if p.cancel, err = e.AddHiddenField(nabCancelX, by, nabCancelX+b.button.Width, by+b.button.Height, cStringAt(g.exe, escKey), 0x28); err != nil {
		return err
	}
	e.AssignAutoFunction(func() error {
		if !g.fullDrawScreen {
			return nil
		}
		return g.setupNetAddressBox(p, back)
	}, 1)
	return nil
}

// netAddressScreen runs an address box; false on CANCEL (port-only). The
// loop is Input_Box_Popup_'s, but only the buttons and Enter end it.
func (g *Game) netAddressScreen(p *netAddressBox) (bool, error) {
	e, b := g.e, &g.rsel.box
	saved := e.GetAutoFunction()
	helpList, helpOn := e.HelpList()
	cancelOff := e.CancelDisabled()
	g.net.box = p
	defer func() {
		g.net.box = nil
		if !cancelOff {
			e.EnableCancel()
		}
		e.ClearFields()
		if helpOn {
			e.SetHelpList(helpList)
		} else {
			e.DeactivateHelpList()
		}
		e.PutAutoFunction(saved)
		g.fullDrawScreen = true
	}()
	e.AssignAutoFunction(func() error { return g.drawNetAddressBack(p) }, 1)
	back := e.GetAutoFunction()
	e.DeactivateHelpList()
	*b = inputBox{titleText: p.label, maxChars: int16(len(p.value) - 1), remap: true,
		x: nabX, y: nabY, titleY: nabY + 3, titleH: 0x36,
		buttonX: nabAcceptX, buttonY: nabY + 0x64, fieldX: nabX + 0x22, fieldY: nabY + 0x36,
		fieldW: 0xcd, fieldH: 0x1a}
	copy(b.text[:], p.value[:])
	g.moxFontColorsOffset = 0x10
	b.colors = g.fontColors2(3, 0xc, 0x18)
	// A right click must not cancel: Esc then comes back as -line, with the
	// edit ended (a right click on the line leaves it active).
	e.DisableCancel()
	setup := func() error { return g.setupNetAddressBox(p, back) }
	if g.net.fadeIn {
		// No initializing screen (TCP): the box fades in.
		if err := g.netFadeInto(setup); err != nil {
			return false, err
		}
	} else if err := setup(); err != nil {
		return false, err
	}
	ok := false
	for done := false; !done; {
		r, err := e.GetInput()
		if err != nil {
			return false, err
		}
		editing := e.InputActive() == b.inputField
		switch {
		case r == b.inputField && !editing:
			ok, done = true, true // Enter; a click in the line keeps editing
		case r == b.buttonField || r == p.ok:
			ok, done = true, true
			e.V.SetPageOff()
			b.button.CurrentFrame = 1
			if p.heading == lobbyHeading {
				g.drawLobbyBoxButtons(1)
			} else if err := g.remapDraw(int(b.buttonX), int(b.buttonY), b.button); err != nil {
				return false, err
			}
			if err := e.TogglePages(); err != nil {
				return false, err
			}
		case r == p.cancel:
			done = true
			e.V.SetPageOff()
			if p.heading == lobbyHeading {
				g.drawLobbyBoxButtons(2)
			} else if err := g.drawNetButton(&netAcceptArt, nabCancelX, int(b.buttonY), 1, "CANCEL"); err != nil {
				return false, err
			}
			if err := e.TogglePages(); err != nil {
				return false, err
			}
		case r == -b.inputField && !editing:
			done = true // Esc
		default:
			if !editing {
				// A right click on a button ended the edit: the line takes keys again.
				e.ActivateInputField(b.inputField)
			}
			e.V.SetPageOff()
			if g.fullDrawScreen {
				if err := g.setupNetAddressBox(p, back); err != nil {
					return false, err
				}
			}
			if p.heading == lobbyHeading {
				g.drawLobbyBoxButtons(0) // lit under the pointer
			}
			if err := e.TogglePages(); err != nil {
				return false, err
			}
		}
	}
	if ok {
		p.value = [len(p.value)]byte{}
		copy(p.value[:len(p.value)-1], trimSpaces(save.CString(b.text[:])))
	}
	return ok, nil
}

// netCreateGameScreen is the port-only "create a game" box: the TCP port
// to listen on (game name from the new game screen). Without TCP (the
// in-memory transport of tests) there is nothing to ask.
func (g *Game) netCreateGameScreen() (bool, error) {
	t, ok := g.net.network.(*netplay.TCP)
	if !ok || g.net.noLobbyBoxes {
		return true, nil
	}
	p := &netAddressBox{heading: "CREATE NETWORK GAME  " + g.gameName(), label: "TCP Port To Listen On"}
	port := t.Listen
	if port == "" {
		port = ":" + strconv.Itoa(netplay.DefaultPort)
	}
	copy(p.value[:], port)
	ok, err := g.netAddressScreen(p)
	if ok {
		t.Listen = save.CString(p.value[:])
		if _, err := strconv.Atoi(t.Listen); err == nil {
			t.Listen = ":" + t.Listen
		}
	}
	return ok, err
}

// netJoinAddressBox is the port-only "join a game" box, opened by the game
// list's ADDRESS: the creator's address (host or host:port; empty: the LAN).
func (g *Game) netJoinAddressBox(t *netplay.TCP) (bool, error) {
	p := &netAddressBox{heading: "JOIN NETWORK GAME", label: "Host Or Host:Port To Join"}
	copy(p.value[:], t.Addr)
	ok, err := g.netAddressScreen(p)
	if ok {
		t.Addr = save.CString(p.value[:])
	}
	return ok, err
}

// The game list's ADDRESS (port-only, TCP only): left of CANCEL in the
// list box's bottom bar; it opens the join box over the list.
const netListAddrX, netListAddrY = 0x35, 0x158

func (g *Game) netListAddress() (*netplay.TCP, bool) {
	t, ok := g.net.network.(*netplay.TCP)
	return t, ok
}

// addNetListAddressField adds ADDRESS (hot key A) to the game list.
func (g *Game) addNetListAddressField() error {
	m := g.netScr
	m.addrF = -1000
	if _, ok := g.netListAddress(); !ok {
		return nil
	}
	p, err := g.picture(multiGMLBX, 0x101)
	if err != nil {
		return err
	}
	x, y := m.x+netListAddrX, m.y+netListAddrY
	m.addrF, err = g.e.AddHiddenField(x, y, x+p.Width-1, y+p.Height-1, "A", ')')
	return err
}

// drawNetListAddress draws ADDRESS over the list box, pressed or not.
func (g *Game) drawNetListAddress(frame int) error {
	if _, ok := g.netListAddress(); !ok {
		return nil
	}
	m := g.netScr
	return g.drawNetButton(&netCancelArt, m.x+netListAddrX, m.y+netListAddrY, frame, "ADDRESS")
}

// drawNetJoinersList is the port-only part of the waiting screen: who is
// in, by net id, under the original's box.
func (g *Game) drawNetJoinersList() error {
	m, t := g.netScr, g.e.Text()
	if g.net.ep == nil {
		return nil
	}
	y := m.y + m.height + 6
	for _, id := range g.net.ep.Users() {
		s := fmt.Sprintf("Player %d", id)
		if id == g.net.ep.ID() {
			s += " (you)"
		}
		if err := t.Print(m.x+0x20, y, s); err != nil {
			return err
		}
		y += t.Height() + 2
	}
	return nil
}

// uniAddressScreen is netAddressScreen with the line in Unicode (the
// lobby's game description): typed text in any language, Enter or ACCEPT
// take it, Esc or CANCEL drop it. p.uni holds the line.
func (g *Game) uniAddressScreen(p *netAddressBox) (bool, error) {
	e, b := g.e, &g.rsel.box
	saved := e.GetAutoFunction()
	helpList, helpOn := e.HelpList()
	cancelOff := e.CancelDisabled()
	g.net.box = p
	defer func() {
		g.net.box = nil
		if !cancelOff {
			e.EnableCancel()
		}
		e.ClearFields()
		if helpOn {
			e.SetHelpList(helpList)
		} else {
			e.DeactivateHelpList()
		}
		e.PutAutoFunction(saved)
		g.fullDrawScreen = true
	}()
	e.AssignAutoFunction(func() error { return g.drawNetAddressBack(p) }, 1)
	back := e.GetAutoFunction()
	e.DeactivateHelpList()
	*b = inputBox{titleText: p.label, maxChars: 1, remap: true,
		x: nabX, y: nabY, titleY: nabY + 3, titleH: 0x36,
		buttonX: nabAcceptX, buttonY: nabY + 0x64, fieldX: nabX + 0x22, fieldY: nabY + 0x36,
		fieldW: 0xcd, fieldH: 0x1a}
	g.moxFontColorsOffset = 0x10
	b.colors = g.fontColors2(3, 0xc, 0x18)
	e.DisableCancel()
	if err := g.setupNetAddressBox(p, back); err != nil {
		return false, err
	}
	for {
		var typed uniKey
		unhook := g.hookUniLine(p.uni, &typed)
		r, err := e.GetInput()
		unhook()
		if err != nil {
			return false, err
		}
		pressed := 0
		switch {
		case typed == uniEnter, r == b.buttonField, r == p.ok:
			pressed = 1
		case typed == uniEsc, r == p.cancel:
			pressed = 2
		}
		if pressed > 0 {
			e.V.SetPageOff()
			g.drawLobbyBoxButtons(pressed)
			if err := e.TogglePages(); err != nil {
				return false, err
			}
			return pressed == 1, nil
		}
		e.V.SetPageOff()
		if g.fullDrawScreen {
			if err := g.setupNetAddressBox(p, back); err != nil {
				return false, err
			}
		}
		x, y := int(b.fieldX), int(b.fieldY)
		e.V.Fill(x+4, y+3, x+int(b.fieldW)-4, y+int(b.fieldH)-3, p.uniBack)
		ink := byte(nearestColor(&e.V.Palette, [3]uint8{56, 54, 50})) // light grey, as typed text there
		g.drawUniLine(p.uni, x+8, y+(int(b.fieldH)-uniLineH)/2, x+int(b.fieldW)-6, ink)
		g.drawLobbyBoxButtons(0)
		if err := e.TogglePages(); err != nil {
			return false, err
		}
	}
}
