package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/save"
)

// The waiting screen of a network turn and its chat box (module 111 and
// MOXCHAT), ported from the assembly. Spec: docs/net/turn.md.

const (
	netChatLines     = 0xe      // _chat_info: 14 lines of 0x52 bytes, then the count
	netChatLine      = 0x51     // a line's text, its NUL included
	netChatMax       = 0x50     // the chat field's characters
	netMainBaseTable = 0x10010  // Get_Main_Palette_Base_Colors_' table, by banner
	netMeFormat      = 0x17a064 // "(%s)  "
	netChatFormat    = 0x17a07d // "(%s)  %s"
	netGNNFormat     = 0x17a086 // "( GNN )  %s"
)

// chatInfo is Net_Next_Turn_'s frame buffer that _chat_info (0x1aa250)
// points to while the turn runs.
type chatInfo struct {
	speaker [netChatLines]byte
	text    [netChatLines][netChatLine]byte
	count   int32 // +0x47c
}

// getMainPaletteBaseColors is Get_Main_Palette_Base_Colors_ (0xf30b9):
// three colours of player p's banner.
func (g *Game) getMainPaletteBaseColors(p int) (a, b, c byte) {
	switch col := g.state.Players[p].Color; col {
	case 3:
		return 0xa, 0xe, 0x17
	case 4:
		return 0x3b, 0x3c, 0x3d
	default:
		base := g.exe.bytes(netMainBaseTable+uint32(col), 1)[0]
		return base, base + 2, base + 3
	}
}

// loadNetNextTurnScreen is Load_Net_Next_Turn_Screen_ (0xf3e42): the
// pictures and the layout, faded in when fade is set.
func (g *Game) loadNetNextTurnScreen(fade bool) error {
	g.e.MoxUpdate()
	g.net.chatString[0] = 0
	mp := &mpScreen{}
	g.hs.mp = mp
	var err error
	if mp.top, err = g.picture(multiGMLBX, 0x2a); err != nil {
		return err
	}
	h1 := mp.top.Height
	mp.x = int16((0x280 - mp.top.Width) / 2)
	if mp.mid, err = g.picture(multiGMLBX, 0x2b); err != nil {
		return err
	}
	h2 := mp.mid.Height
	if mp.back, err = g.picture(multiGMLBX, 0x28); err != nil {
		return err
	}
	h3 := mp.back.Height
	if mp.fill, err = g.picture(multiGMLBX, 0x2d); err != nil {
		return err
	}
	mp.first = true
	mp.nValid = int16(g.nRemainingHumans())
	g.getNetNextTurnPlayerColors()
	mp.boxH = int16(h1 + h2)
	y := (0x1e0 - int(int16(h1+h2+h3))) / 2
	if y < 1 {
		y = 0
	}
	mp.fontMode = 2
	mp.y = int16(y)
	g.e.AssignAutoFunction(g.drawNetNextTurnScreen, 1)
	g.fullDrawScreen = true
	g.e.ClearFields()
	if fade {
		if err := g.fadeIntoScreen(g.drawNetNextTurnScreen, 0); err != nil {
			return err
		}
	}
	return g.addNetNextTurnFields()
}

// getNetNextTurnPlayerColors is Get_Net_Next_Turn_Player_Colors_ (0xf31bb).
func (g *Game) getNetNextTurnPlayerColors() {
	mp, s := g.hs.mp, &g.state
	mode := int(mp.fontMode)
	for p := 0; p < int(s.NumPlayers); p++ {
		c := mainPalettePlayerColors[s.Players[p].Color]
		if s.Players[p].Color == 3 {
			g.moxFontColorsOffset = 0xc
			mp.colors[p] = g.fontColors2(mode, 10, 0x10)
			g.moxFontColorsOffset = 8
			mp.shades[p] = g.fontColors2(mode, 6, 0xc)
			g.moxFontColorsOffset = 0xc
			mp.lights[p] = g.fontColors2(mode, 10, 0x10)
			continue
		}
		g.moxFontColorsOffset = c + 2
		t := g.fontColors2(mode, c, c+5)
		g.moxFontColorsOffset = c + 1
		mp.colors[p] = t
		mp.shades[p] = g.fontColors2(mode, c, c+3)
		g.moxFontColorsOffset = c + 2
		mp.lights[p] = g.fontColors2(mode, c, c+6)
	}
}

// addNetNextTurnFields is Add_Net_Next_Turn_Fields_ (0xefcea): the chat
// line after "(name)" and a hidden field over the screen (hot key C).
func (g *Game) addNetNextTurnFields() error {
	e, mp, s := g.e, g.hs.mp, &g.state
	x, y := int(mp.x), int(mp.y)
	e.ClearFields()
	y += int(mp.boxH) + 0xbb
	a, b, c := g.getMainPaletteBaseColors(int(s.PlayerNum))
	g.moxFontColorsOffset = b + 1
	normal := g.fontColors2(int(mp.fontMode), a+1, c+2)
	g.moxFontColorsOffset = b + 3
	special := g.fontColors2(int(mp.fontMode), a+1, c+4)
	x += 0x19
	t := e.Text()
	t.SetStyle(int(mp.fontMode), normal)
	t.SetSpecialColors(special)
	label := fmt.Sprintf(cFormat(cStringAt(g.exe, netMeFormat)), save.CString(s.Players[s.PlayerNum].EmperorName[:]))
	w := t.Width([]byte(label))
	g.net.chatString[0] = 0
	var err error
	if mp.chatField, err = e.AddContinuousStringInputField(x+w, y, 0x237-w, 0x11, g.net.chatString[:], netChatMax, 0, nil, 0x29, 0); err != nil {
		return err
	}
	mp.hiddenField, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, cStringAt(g.exe, mpKeySetup), 0x29)
	return err
}

// drawFillSeg is Draw_Fill_Seg_ (0xf2742): the plate over player p's
// banner once he is ready (placed by p, not by his row).
func (g *Game) drawFillSeg(p int) error {
	mp := g.hs.mp
	x := p%2*0x126 + int(mp.x) + 0x28
	y := p/2*0x24 + 0x41 + int(mp.y)
	pic, err := g.picture(multiGMLBX, 0x2d)
	if err != nil {
		return err
	}
	mp.fill = pic
	return g.e.Draw(x, y, pic)
}

// drawNetNextTurnScreen is Draw_Net_Next_Turn_Screen_ (0xf1075): the map
// behind, the box, every human's banner (a plate once he is ready), the
// chat lines and "(name)" before the chat field.
func (g *Game) drawNetNextTurnScreen() error {
	e, mp, s := g.e, g.hs.mp, &g.state
	const style = 4
	x0, y0 := int(mp.x), int(mp.y)
	row := 0
	mp.frame++
	if g.fullDrawScreen {
		e.V.SetPageOff()
		g.ms.dontDrawToBack = true
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := g.drawMiniMainScreen(); err != nil {
			return err
		}
		g.fullDrawScreen = false
		mp.first = true
	}
	anim, err := g.picture(multiGMLBX, 0x2a)
	if err != nil {
		return err
	}
	mp.top = anim
	for i := 0; i < int(mp.frame); i++ {
		g.setAnimationFrame(anim, i)
		if err := e.Draw(x0, y0, anim); err != nil {
			return err
		}
	}
	if anim.Frames <= int(mp.frame) {
		mp.frame = 0
	}
	y := y0 + anim.Height
	if mp.mid, err = g.picture(multiGMLBX, 0x2b); err != nil {
		return err
	}
	if mp.first {
		if err := e.Draw(x0, y, mp.mid); err != nil {
			return err
		}
	}
	yBottom := y + mp.mid.Height
	y = yBottom
	if mp.first {
		if mp.back, err = g.picture(multiGMLBX, 0x28); err != nil {
			return err
		}
		if err := e.Draw(x0, y, mp.back); err != nil {
			return err
		}
	}
	t := e.Text()
	var a, b, c byte
	for p := 0; p < int(s.NumPlayers); p++ {
		a, b, c = g.getMainPaletteBaseColors(p)
		g.moxFontColorsOffset = b + 1
		waiting := g.fontColors2(style, a+1, c+2)
		g.moxFontColorsOffset = b
		ready := g.fontColors2(style, a, c)
		g.moxFontColorsOffset = b + 3
		g.fontColors2(style, a+1, c+4)
		pl := &s.Players[p]
		if pl.Objective != save.ObjectiveHuman || pl.Eliminated != 0 {
			continue
		}
		x := p%2*0x126 + int(mp.x) + 0x5c
		col := waiting
		if g.net.u8(netReady+uint32(p)) == 0 {
			if err := g.drawMultiPlayerBanner(p, row, true); err != nil {
				return err
			}
		} else {
			if err := g.drawFillSeg(p); err != nil {
				return err
			}
			col = ready
		}
		t.SetStyle(4, col)
		t.SetSpacing(1)
		ty := (0x1d-t.Height())/2 + int(mp.y) + p/2*0x24 + 0x3f
		name := save.CString(pl.EmperorName[:])
		if err := t.Print(x, ty, name); err != nil {
			return err
		}
		if t.Width([]byte(name)) < 0x98 {
			if err := t.PrintRight(x+0xcd, ty, g.raceName(p)); err != nil {
				return err
			}
		}
		row++
	}
	lx, ly := x0+0x18, y+0xe
	if ch := g.net.chat; ch != nil {
		for i := 0; i < int(ch.count); i++ {
			var line string
			text := save.CString(ch.text[i][:])
			if sp := int(ch.speaker[i]); sp < 8 {
				a, b, c = g.getMainPaletteBaseColors(sp)
				g.moxFontColorsOffset = b + 1
				t.SetStyle(int(mp.fontMode), g.fontColors2(int(mp.fontMode), a+1, c+2))
				line = fmt.Sprintf(cFormat(cStringAt(g.exe, netChatFormat)), save.CString(s.Players[sp].EmperorName[:]), text)
			} else {
				g.moxFontColorsOffset = 0x10
				t.SetStyle(int(mp.fontMode), g.fontColors2(int(mp.fontMode), 0xc, 0x17))
				line = fmt.Sprintf(cFormat(cStringAt(g.exe, netGNNFormat)), text)
			}
			if i == 0 {
				e.V.SetWindow(lx, ly, lx+0x23a, ly+0xb)
				e.V.ClipOn()
				if mp.back, err = g.picture(multiGMLBX, 0x28); err != nil {
					return err
				}
				if err := e.Draw(x0, yBottom, mp.back); err != nil {
					return err
				}
				e.V.ClipOff()
				e.V.ResetWindow()
			}
			py := ly
			ly += 0xc
			if err := t.Print(lx, py, line); err != nil {
				return err
			}
		}
	}
	label := fmt.Sprintf(cFormat(cStringAt(g.exe, netMeFormat)), save.CString(s.Players[s.PlayerNum].EmperorName[:]))
	a, b, c = g.getMainPaletteBaseColors(int(s.PlayerNum))
	col := g.fontColors2(style, a+1, c+2)
	// The offset is set after its use: it reaches the next Font_Colors2_.
	g.moxFontColorsOffset = b
	t.SetStyle(int(mp.fontMode), col)
	if err := t.Print(x0+0x19, y0+int(mp.boxH)+0xbe, label); err != nil {
		return err
	}
	if mp.first {
		e.V.CopyOffToBack()
		e.V.SetRefreshStencil()
	}
	mp.first = false
	return nil
}

// chatBoxInputLoop is Chat_Box_Input_Loop_ (0xf55a4): one input poll and
// one frame of the waiting screen. Enter in the chat line (or the hidden
// field) sends the line (msg 0x27).
func (g *Game) chatBoxInputLoop() error {
	e, mp, n := g.e, g.hs.mp, &g.net
	g.netRestartPoint() // port-only: the turn's host was lost
	r, err := e.GetInput()
	if err != nil {
		return err
	}
	if !mp.chatOn {
		n.chatString[0] = 0
		e.ActivateInputField(mp.chatField)
		mp.chatOn = true
	}
	if r == mp.chatField || r == mp.hiddenField {
		if n.chatString[0] != 0 {
			g.sendChatMsg(n.chatString[:])
			n.chatString[0] = 0
		}
		e.ActivateInputField(mp.chatField)
	}
	e.V.SetPageOff()
	if err := g.drawNetNextTurnScreen(); err != nil {
		return err
	}
	return e.TogglePages()
}

// sendChatMsg is Send_Chat_Msg_ (0xdd3b8): msg 0x27 with the line to every
// human, oneself included.
func (g *Game) sendChatMsg(str []byte) {
	s := &g.state
	msg := append([]byte(save.CString(str)), 0)
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if s.Players[k].Objective != save.ObjectiveHuman {
			continue
		}
		g.moxSendMessage(int32(s.Players[k].NetID), 0x27, msg)
		g.e.MoxUpdate()
	}
}

// sendGNNChatMsg is Send_GNN_Chat_Msg_ (0xdd42a): msg 0x2d, at most 0x50
// characters (the line is cut in place).
func (g *Game) sendGNNChatMsg(str []byte) {
	s := &g.state
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if s.Players[k].Objective != save.ObjectiveHuman {
			continue
		}
		n := len(save.CString(str))
		if n > netChatMax {
			n = netChatMax
			str[netChatMax] = 0
		}
		g.moxSendMessage(int32(s.Players[k].NetID), 0x2d, append([]byte(save.CString(str[:n])), 0))
		g.e.MoxUpdate()
	}
}

// receiveChatMsg is Receive_Chat_Msg_ (0xdd351): a line from speaker (8
// is GNN); a full box scrolls up by one.
func (g *Game) receiveChatMsg(speaker byte, text []byte) {
	ch := g.net.chat
	if ch.count == netChatLines {
		copy(ch.speaker[:], ch.speaker[1:])
		copy(ch.text[:], ch.text[1:])
		ch.count = netChatLines - 1
	}
	ch.speaker[ch.count] = speaker
	ch.text[ch.count] = [netChatLine]byte{}
	copyCStr(ch.text[ch.count][:netChatLine-1], text)
	ch.count++
}
