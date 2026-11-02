package game

import (
	"strconv"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The event news of the report phase (EVENTS.C): Check_For_Event_ words
// every event with news (state 4..6) through Setup_Next_Event_ and
// Get_Event_Message_ and shows it on the GNN screen or, with GNN off, in a
// text box over the mini main screen. Spec: docs/screens/gnn.md.

const (
	eventMsgFile   = 0x178b40 // "EVENTMSE.LBX", then G, F, S, I, P
	eventMsgStride = 0xd
	jimTextLBX     = "jimtext.lbx"
	jimText2LBX    = "jimtext2.lbx"
	eventPlural    = 0x178b8e // "s"
	eventsLBX      = 0x178b28 // "EVENTS.LBX"
	eventRankNL    = 0x178b9d // "\n"
	eventRankHead  = 0x178b9f // "\x1a0"
	eventRankFmt   = 0x178ba2 // "\x07X%d.%d. %s"
	eventMouseList = 0x180ea8 // the blank cursor over the whole screen
	eventColors    = 0x180e7c // the news text's outline colours
)

// gnnState holds the GNN screen's globals (EVENTS.C).
type gnnState struct {
	picture   *gfx.Picture        // _event_picture (0x19aa70)
	text      string              // the message buffer (0x19aa74)
	picShown  bool                // 0x19acef: the picture's palette is in
	savedAuto engine.AutoFunction // _jim_saved_autofunction (0x19aba0)
	showing   bool                // port-only: in Drive_Event_Screen_, for tests
}

// event returns event i's 9-byte record: +0 player (s8), +1 state, +2 s8,
// +3, +5, +7 s16.
func (s *State) event(i int) []byte { return s.EventData[9*i : 9*i+9] }

func ev16(e []byte, o int) int16 { return int16(uint16(e[o]) | uint16(e[o+1])<<8) }

func setEv16(e []byte, o int, v int16) { e[o], e[o+1] = byte(v), byte(uint16(v)>>8) }

// checkForEvent is Check_For_Event_ (0x2027e).
func (g *Game) checkForEvent() error {
	s := &g.state
	g.currentEvent = 0
	for i := 0; i < 0x24; i++ {
		if st := s.event(i)[1]; st == 4 || st == 6 || st == 5 {
			if err := g.setupNextEvent(i); err != nil {
				return err
			}
		}
	}
	if g.currentEvent != 0 {
		e := g.e
		e.DeactivateAutoFunction()
		e.ClearFields()
		if g.settings[9] == 1 {
			e.SetFadePercent(0)
			e.SetFadeSpeedPercent(10)
			if err := e.FastFadeOut(); err != nil {
				return err
			}
		}
		g.playBackgroundMusic() // 0x202ef
		g.ms.autoTurnExit = true
		e.PutAutoFunction(g.gnn.savedAuto)
	}
	g.ms.autoTurnExit = true
	g.resetEvents()
	return nil
}

// resetEvents is Reset_Events_ (0x201f9): every 20 stardates, with no idle
// event left, all events start over.
func (g *Game) resetEvents() {
	s := &g.state
	if s.Stardate%0x14 != 0 {
		return
	}
	for i := 0; i < 0x24; i++ {
		if s.event(i)[1] == 0 {
			return
		}
	}
	g.initEvents()
}

// initEvents is Init_Events_ (0x201a4).
func (g *Game) initEvents() {
	s := &g.state
	s.EventDelayCount = 0
	s.LastEventYear = s.Stardate
	for i := 0; i < 0x24; i++ {
		e := s.event(i)
		e[0], e[1], e[2] = 0xff, 0, 0
		setEv16(e, 3, -1)
		setEv16(e, 5, -1)
		setEv16(e, 7, -1)
	}
}

// setupNextEvent is Setup_Next_Event_ (0x21371): the message variant and
// arguments of event i's news by its state; -1 means no news. Locals the
// event's case does not set are stale stack in the original; the port
// passes 0.
func (g *Game) setupNextEvent(i int) error {
	s := &g.state
	si, args := g.eventNews(i)
	if si == -1 {
		return nil
	}
	gnn := g.settings[9] == 1
	if gnn {
		// The opening message, or "in other news" between two.
		n := 1
		if g.currentEvent == 0 {
			n = 0
			if err := g.startMainEvent(); err != nil {
				return err
			}
		}
		g.gnn.picture = nil
		v := int(int16(s.Random.Random(4) - 1))
		msg, err := g.eventMessage(n, v, [12]int16{})
		if err != nil {
			return err
		}
		g.gnn.text = msg
		if n == 1 {
			g.gnn.picShown = false
		}
		if err := g.driveEventScreen(); err != nil {
			return err
		}
	}
	g.currentEvent++
	if gnn {
		pic, err := g.picture(cStringAt(g.exe, eventsLBX), i+2)
		if err != nil {
			return err
		}
		g.gnn.picture = pic
	}
	msg, err := g.eventMessage(i+2, int(si), args)
	if err != nil {
		return err
	}
	if gnn {
		g.gnn.text = msg
		g.gnn.picShown = false
		return g.driveEventScreen()
	}
	title, err := g.textMessage(jimTextLBX, 0x1d, 0x28)
	if err != nil {
		return err
	}
	if err := g.miniMainScreenTextBox(title, msg); err != nil {
		return err
	}
	g.e.ClearFields()
	return nil
}

// startMainEvent is Start_Main_Event_ (0x2031d): the GNN screen comes up.
func (g *Game) startMainEvent() error {
	e := g.e
	e.DeactivateHelpList()
	e.EnableDrawAllFields()
	e.V.ResetWindow()
	e.V.ClipOff()
	e.DeactivateAutoFunction()
	e.ClearFields()
	e.SetMouseList(g.mouseListAt(eventMouseList))
	g.playStreamingMusic(0x76, -1, true) // 0x2035b
	e.SetFadePercent(0)
	e.SetFadeSpeedPercent(10)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	e.SetInputDelay(1)
	e.ClearFields()
	e.V.SetRefreshStencil()
	g.gnn.savedAuto = e.GetAutoFunction()
	e.DeactivateAutoFunction()
	if err := g.eventFadeIn(); err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawEventScreen, 2)
	t := e.Text()
	t.SetOutlineColor(0, 0xf8)
	t.SetStyleShadow(4, g.exeColors(eventColors), font.Outline)
	return nil
}

// eventFadeIn is Event_Fade_In_ (0x20538): the cursor and palette from
// EVENTS.LBX 0, the studio (entry 1) as a file animation on both pages.
func (g *Game) eventFadeIn() error {
	e := g.e
	file := cStringAt(g.exe, eventsLBX)
	pic, err := g.picture(file, 0)
	if err != nil {
		return err
	}
	e.ClearMouseImage(0)
	cursor := gfx.NewBlankBitmap(0x1e, 0x1e)
	cursor.DrawPictureToBitmap(pic)
	g.gnn.picShown = true
	e.V.SetPageOff()
	e.DrawPalette(pic)
	e.V.SetPaletteChanges(0, 0xff)
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	studio, err := g.picture(file, 1)
	if err != nil {
		return err
	}
	e.OpenFileAnimation(studio)
	e.V.SetPageOff()
	if err := e.DrawFileAnimation(studio); err != nil {
		return err
	}
	e.RefreshPalette()
	e.V.CopyOffToBack()
	e.DrawBitmapToMouse(0, 0, cursor, 0)
	e.SetMouseList(engine.MouseArrow)
	return nil
}

// driveEventScreen is Drive_Event_Screen_ (0x20400): until a click; keys
// do nothing.
func (g *Game) driveEventScreen() error {
	e := g.e
	g.gnn.showing = true
	defer func() { g.gnn.showing = false }()
	for {
		e.MarkTime()
		e.ClearFields()
		f, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0)
		if err != nil {
			return err
		}
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		if in == f {
			return nil
		}
		if err := g.drawEventScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.ReleaseTime(2); err != nil {
			return err
		}
	}
}

// drawEventScreen is Draw_Event_Screen_ (0x20460). A new message's first
// frame only brings in its picture's palette.
func (g *Game) drawEventScreen() error {
	e := g.e
	e.V.SetPageOff()
	if g.settings[6] == 1 {
		if err := e.DrawFileAnimationStencil(); err != nil {
			return err
		}
		if e.FileAnimationFrame() == 0 {
			e.SetFileAnimationFrame(1)
		}
	}
	if !g.gnn.picShown {
		g.gnn.picShown = true
		if g.gnn.picture != nil {
			e.DrawPalette(g.gnn.picture)
		}
		g.playStreamingMusic(0x76, -1, true) // 0x2052f: the news theme
		return nil
	}
	if p := g.gnn.picture; p != nil {
		if err := e.Draw(0x140, 0xe, p); err != nil {
			return err
		}
	}
	t := e.Text()
	h, err := t.FormattedMaxHeight(0x226, g.gnn.text)
	if err != nil {
		return err
	}
	h = int(int16(h))
	switch {
	case h > 0x3c:
		h += 0xe
	case h > 0x12:
		h -= 4
	}
	y := int(int16(0x1bd - int(int16(h))/2))
	return t.PrintFormatted(0x37, y, 0x226, g.gnn.text, 2)
}

// eventNews is Setup_Next_Event_'s switch: the variant (-1 for none) and
// the arguments of Get_Event_Message_ (BX, CX, then 5..13).
func (g *Game) eventNews(i int) (int16, [12]int16) {
	s := &g.state
	e := s.event(i)
	st := e[1]
	e3, e5, e7 := ev16(e, 3), ev16(e, 5), ev16(e, 7)
	var l4, l8, lc, l10, l14, l18, l1c, l24, l28, di int16
	l2c := int16(int8(e[0]))
	si := int16(-1)
	planetStar := func(pl int16) int16 { return int16(s.Planets[pl].Star) }
	byState := func(s4, s5, s6 int16) {
		switch st {
		case 4:
			si = s4
		case 5:
			si = s5
		case 6:
			si = s6
		}
	}
	// The cases 0x13..0x17 share this tail for states 4 (a planet), 5 and 6.
	planetEvent := func(state5Only bool) {
		switch {
		case st == 4:
			if e3 != -1 {
				l8 = planetStar(e3)
				si = 3
				setEv16(e, 3, -1)
			}
		case st == 5 && !state5Only:
			si = 2
			if e5 != -1 {
				si = 1
			}
		case st == 6:
			setEv16(e, 3, -1)
			si = 0
		}
	}
	switch i {
	case 0:
		l28, si, lc = e3, 0, 0
		if e5 != 0 {
			lc, si = e5, 1
		}
	case 1:
		l10, si = s.Planets[e3].Colony, 0
	case 2:
		l10, di = e3, int16(int8(e[2]))
		l4 = int16((int32(e7) - int32(e5)) * 100 / int32(e7))
		switch st {
		case 4:
			si = -1
			if e5 != 0 {
				si = 1
			}
		case 5:
			si = 3
			if e5 <= 0 {
				si = 2
			}
		case 6:
			si = 0
		}
	case 3:
		si, l1c, di = 0, e5, e3
	case 4:
		l18, si = e3, 0
	case 5:
		di, l18, si = 100, e3, 0
	case 6:
		si, l14, di = 0, int16(int8(e[0])), e3
	case 7:
		l10, si, l4, di = e3, 0, e7, e5
		if di < 1 {
			si, di = 1, l4
		}
	case 8:
		si = 0
		if e5 != -1 {
			l24, si = e5, 1
		}
	case 9:
		byState(-1, 2, 3)
	case 0xa:
		l8, si = planetStar(e3), 0
	case 0xb, 0xc:
		if i == 0xb {
			l14 = int16(int8(e[0]))
		}
		l8, di, si = planetStar(e3), int16(s.Planets[e3].Minerals), 0
	case 0xd:
	case 0xe:
		l8 = e3
		byState(-1, 2, 0)
	case 0xf:
		si, l8, di = 0, e3, e5
	case 0x10:
		l10, di = e3, e5
		byState(1, 2, 0)
	case 0x11:
		l10 = e3
		byState(-1, 2, 0)
	case 0x12:
		l1c, si = e3, 0
	case 0x13:
		switch st {
		case 4:
			if e3 != -1 {
				l8, si = planetStar(e3), 3
			}
		case 5:
			si = 2
			if e5 != -1 {
				si = 1
			}
		case 6:
			setEv16(e, 3, si)
			si = 0
		}
	case 0x14, 0x15, 0x16:
		planetEvent(false)
	case 0x17:
		if st == 3 {
			si = 2
			if e5 != -1 {
				si = 1
			}
		} else {
			planetEvent(true)
		}
	case 0x18:
		l8, l4, di = e3, e5, int16(int8(e[2]))
		switch st {
		case 4:
			si = 1
		case 5:
			si = 3
			if l4 <= 0 {
				si = 2
			}
		case 6:
			si = 0
		}
	case 0x19, 0x1a, 0x1b:
		if i == 0x19 {
			l8 = e3
		}
		byState(-1, 2, 3)
	case 0x1c:
		di, l8, si = e3, e5, 0
	case 0x1d, 0x21:
		si = int16(s.Random.Random(4)) - 1
	case 0x1e:
		si, di = min(e5, 3), e3
	case 0x1f:
		si, di = e5, 0
	case 0x20:
		si = int16(s.Random.Random(4)) - 1
		l14 = int16(int8(e[0]))
	case 0x22:
		l18 = e3
		si = int16(s.Random.Random(4)) - 1
	case 0x23:
		si = int16(s.Random.Random(4)) - 1
		l14 = e3
	}
	return si, [12]int16{l10, di, l4, l2c, l1c, l14, l24, l18, l8, l28, lc}
}

// miniMainScreenTextBox is Mini_Main_Screen_Text_Box_ (0x87698).
func (g *Game) miniMainScreenTextBox(title, msg string) error {
	e := g.e
	e.V.SetRefreshFull()
	g.ms.gnnShowing = true
	err := g.textBox(&title, msg)
	g.ms.gnnShowing = false
	if err != nil {
		return err
	}
	e.V.SetPageOff()
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.V.SetRefreshStencil()
	g.ms.drawToBack = true
	return nil
}

// eventMessage is Get_Event_Message_ (0x21b6d): record 0 (300 bytes) of
// entry n*4 + variant of the language's EVENTMS?.LBX with its codes
// replaced. a holds BX, CX and the stack arguments 5..13.
func (g *Game) eventMessage(n, variant int, a [12]int16) (string, error) {
	s := &g.state
	bx, cx := a[0], a[1]
	p5, p6, p7, p8, p9, p10, p11, p12, p13 := a[2], a[3], a[4], a[5], a[6], a[7], a[8], a[9], a[10]
	lang := 0
	if l := int(g.settings[0xd2]); l >= 1 && l <= 5 {
		lang = l
	}
	file := cStringAt(g.exe, eventMsgFile+uint32(lang*eventMsgStride))
	arc, err := g.archive(file)
	if err != nil {
		return "", err
	}
	t, err := arc.Table(n*4 + variant)
	if err != nil {
		return "", err
	}
	rec := t.Record(0)
	es, err := g.eStrings()
	if err != nil {
		return "", err
	}
	race := func(p int16) string { return save.CString(s.Players[p].RaceName[:]) }
	out := []byte{}
	for k := 0; k < 300 && k < len(rec) && rec[k] != 0; k++ {
		var ins string
		switch c := rec[k]; c {
		case 0x80:
			ins = g.planetName(int(s.Colonies[bx].Planet))
		case 0x81:
			ins = strconv.Itoa(int(cx))
		case 0x82:
			ins = strconv.Itoa(int(p5))
		case 0x83, 0x84:
			v := cx
			if c == 0x84 {
				v = p5
			}
			if (v > 1 || v == 0) && g.settings[0xd2] == 0 {
				ins = cStringAt(g.exe, eventPlural)
			}
		case 0x85:
			ins = race(p6)
		case 0x86, 0x87:
			f := p7
			if c == 0x87 {
				f = cx
			}
			if ins, err = g.techName(int(f)); err != nil {
				return "", err
			}
		case 0x88:
			ins = save.CString(s.Players[p8].EmperorName[:])
		case 0x89:
			ins = save.CString(s.Leaders[p9].Name[:])
		case 0x8a:
			ins = race(p10)
		case 0x8b:
			ins = save.CString(s.Stars[p11].Name[:])
		case 0x8c:
			ins = es[planetMineralEStr[cx]]
		case 0x8d, 0x8e:
			app := p12
			if c == 0x8e {
				app = p13
			}
			if ins, err = g.techAppName(int(app)); err != nil {
				return "", err
			}
		case 0x8f, 0x90, 0x91, 0x92:
			p := p6
			if c >= 0x91 {
				p = p10
			}
			you := p == s.PlayerNum
			switch {
			case (c == 0x8f || c == 0x91) && !you:
				if ins, err = g.textMessage(jimText2LBX, 0x3d, 0x28); err != nil {
					return "", err
				}
			case (c == 0x90 || c == 0x92) && you:
				if ins, err = g.textMessage(jimText2LBX, 0x3e, 0x28); err != nil {
					return "", err
				}
			case (c == 0x90 || c == 0x92) && p >= 0:
				ins = race(p)
			}
			if c <= 0x90 && len(out) == 0 && ins != "" && (c == 0x8f && !you || c == 0x90 && you) {
				// The capital: for 0x8f the second byte, for 0x90 the first.
				b := []byte(ins)
				i := 0
				if c == 0x8f {
					i = 1
				}
				if i < len(b) {
					b[i] -= 0x20
				}
				ins = string(b)
			}
		default:
			out = append(out, c)
			continue
		}
		out = append(out, ins...)
	}
	if n-2 == 0x1f {
		out = append(out, g.eventRanking(variant)...)
	}
	return string(out), nil
}

// eventRanking is the tail of Get_Event_Message_ for event 0x1f: the six
// leading empires by buildings, population, known tech or fleet
// (variant 0..3), three to a line.
func (g *Game) eventRanking(variant int) string {
	s := &g.state
	nl := cStringAt(g.exe, eventRankNL)
	out := nl
	var costs [8]int32
	if variant == 0 {
		costs = g.computeTotalBuildingCosts()
	}
	var who [8]int16
	var val [8]int32
	n := 0
	for p := 0; p < int(s.NumPlayers); p++ {
		pl := &s.Players[p]
		if pl.Eliminated != 0 {
			continue
		}
		who[n] = int16(p)
		switch uint16(variant) {
		case 0:
			val[n] = costs[p]
		case 1:
			val[n] = int32(pl.Population)
		case 2:
			val[n] = int32(pl.KnownTechCost)
		case 3:
			val[n] = g.eventGetFleetStrength(p)
		}
		n++
	}
	for range n {
		for k := 0; k < n-1; k++ {
			if val[k] < val[k+1] {
				val[k], val[k+1] = val[k+1], val[k]
				who[k], who[k+1] = who[k+1], who[k]
			}
		}
	}
	out += cStringAt(g.exe, eventRankHead)
	base := 10
	if g.settings[9] == 1 {
		base = 0x78
	}
	x := base
	f := cStringAt(g.exe, eventRankFmt)
	for k := 0; k < min(n, 6); k++ {
		out += cSprintf(f, x, k+1, save.CString(s.Players[who[k]].RaceName[:]))
		if (k+1)%3 != 0 {
			x += 0x7d
		} else {
			out += nl
			x = base
		}
	}
	return out
}

// eventGetFleetStrength is Event_Get_Fleet_Strength_ (0x2223d): one plus
// the hull size of each of p's ships in orbit or in transit.
func (g *Game) eventGetFleetStrength(p int) int32 {
	s := &g.state
	var v int32
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		if int(sh.Owner) == p && int8(sh.Status) <= 1 {
			v += int32(sh.Design.Size) + 1
		}
	}
	return v
}
