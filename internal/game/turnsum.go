package game

// The turn summary popup (TURNSUM.C), screen 0x28: the human's messages of
// the turn as a paged list over the mini main screen. Spec:
// docs/screens/turn-summary.md.

const (
	turnSumLBX        = "turnsum.lbx"
	turnSumHeader     = 0x184324 // _g_turn_sum_header
	turnSumHighColors = 0x1842e0 // _turn_sum_high_color
	turnSumRows       = 15       // _turn_indent_y, _turn_field_y2
	infoNormalColors  = 0x183b1e // _info_normal_color
	turnSumBullet     = 0x17a3ec // "^"
)

// turnSummaryState holds TURNSUM.C's globals.
type turnSummaryState struct {
	header     listHeader // _g_turn_sum_header
	msgIx      uint8      // _g_turn_message_ix
	firstField int        // _g_first_list_field
	lastField  int        // _g_last_list_field
	sel        *listEntry // _g_selected_msg_p
	showing    bool       // port-only: in the input loop, for tests
}

// turnSummaryPopup is Turn_Summary_Popup_ (0x10fb02).
func (g *Game) turnSummaryPopup() error {
	s, ts := &g.state, &g.tsum
	g.currentScreen = ScreenMain
	n := int(s.NMsgs[s.PlayerNum])
	if n >= 1 {
		if err := g.turnSummaryLoop(n); err != nil {
			return err
		}
	}
	ts.showing = false
	if g.currentScreen == ScreenMain {
		g.currentScreen = ScreenReports
	}
	return nil
}

func (g *Game) turnSummaryLoop(n int) error {
	e, ts := g.e, &g.tsum
	if g.returnScreen == ScreenReports {
		ts.msgIx = 0
	} else {
		if err := e.FastFadeOut(); err != nil {
			return err
		}
		if err := e.LoadPalette(0, 0, 0xff); err != nil {
			return err
		}
		g.turnOffPalettes()
	}
	e.Sys.MouseBuffer()
	e.Sys.MouseBuffer2()
	e.DeactivateHelpList()
	saved := e.GetAutoFunction()
	e.ClearFields()
	e.V.SetRefreshStencil()
	bg, err := g.picture(turnSumLBX, 0)
	if err != nil {
		return err
	}
	up, err := g.picture(turnSumLBX, 1)
	if err != nil {
		return err
	}
	down, err := g.picture(turnSumLBX, 2)
	if err != nil {
		return err
	}
	ok, err := g.picture(turnSumLBX, 3)
	if err != nil {
		return err
	}
	h := &ts.header
redraw:
	for {
		g.fullDrawScreen = false
		e.ClearFields()
		e.AssignAutoFunction(g.drawTurnSummaryPopup, 2)
		upF, err := e.AddButtonField(0x1b9, 0x5f, "", up, "", 0)
		if err != nil {
			return err
		}
		downF, err := e.AddButtonField(0x1b9, 0x161, "", down, "", 0)
		if err != nil {
			return err
		}
		*h = g.readListHeader(turnSumHeader, turnSumRows)
		h.print = g.printTurnSummaryPopupMessage
		if err := g.initTurnSummaryList(h, n, 4, 10); err != nil {
			return err
		}
		ts.sel = &h.entries[h.selectAndPageByHeadID(ts.msgIx)]
		if err := g.drawMiniMainScreen(); err != nil {
			return err
		}
		// Turn_Summary_Popup_Help_ (0x1039d7): one entry at 0x1836f0.
		e.DeactivateHelpList()
		e.SetHelpList(g.helpListAt(0x1836f0, 1))
	page:
		for {
			e.ClearFieldsAbove(downF)
			ts.firstField = e.FieldsCount()
			if ts.lastField, err = g.addFieldsToListPage(h); err != nil {
				return err
			}
			okF, err := e.AddButtonField(0xf2, 0x176, "", ok, "\x1b", 0)
			if err != nil {
				return err
			}
			if _, err := e.AddHiddenField(0x54, 0x33, 0x1d0, 0x189, "\x1b", 0); err != nil {
				return err
			}
			fullF, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "\x1b", 0)
			if err != nil {
				return err
			}
			if err := e.Draw(0x54, 0x33, bg); err != nil {
				return err
			}
			if err := g.displayListPage(h); err != nil {
				return err
			}
			g.setListUpDownFieldDrawing(h, upF, downF)
			e.V.CopyOffToBack()
			e.V.ClearPaletteChanges(0, 0xff)
			if err := e.TogglePages(); err != nil {
				return err
			}
			if err := e.FastFadeIn(); err != nil {
				return err
			}
			ts.showing = true
			for {
				in, err := e.GetInput()
				if err != nil {
					return err
				}
				if g.fullDrawScreen {
					continue redraw
				}
				v := int(int16(in))
				if v < ts.firstField || v > ts.lastField {
					if v < 1 || v >= fullF || v == okF {
						if v == 0 {
							e.MarkTime()
							if err := g.drawTurnSummaryPopup(); err != nil {
								return err
							}
							if err := e.TogglePages(); err != nil {
								return err
							}
							if err := e.ReleaseTime(2); err != nil {
								return err
							}
						}
					} else if v == downF && h.nextListPage() || v == upF && h.previousListPage() {
						continue page
					}
				} else if sel := ts.sel; sel != nil {
					m := g.state.Msgs[int(sel.id)*msgSize:]
					if g.gotoMsgColony(m) {
						g.currentScreen, g.returnScreen = ScreenColony, ScreenTurnSummary
						ts.msgIx = sel.id
						v = fullF
					}
				}
				if v >= fullF || v == okF {
					e.PutAutoFunction(saved)
					e.ClearFields()
					return nil
				}
			}
		}
	}
}

// initTurnSummaryList is Init_Turn_Summary_List_ (0x108ec3): one entry per
// message of the human's, as many items as its text has lines.
func (g *Game) initTurnSummaryList(h *listHeader, n, style, lf int) error {
	s := &g.state
	if int16(n) == 0 {
		return nil
	}
	t := g.e.Text()
	h.entries = make([]listEntry, n)
	first := int(s.FirstMsg[s.PlayerNum])
	t.SetStyle(style, g.exeColors(infoNormalColors))
	t.SetLF(lf)
	for i := range h.entries {
		en := &h.entries[i]
		k := first + i
		en.text, en.state = "", 0
		en.id, en.ids[0] = uint8(k), uint8(k)
		msg, err := g.sprintfMsg(s.Msgs[k*msgSize : (k+1)*msgSize])
		if err != nil {
			return err
		}
		ht, err := t.FormattedMaxHeight(int(h.width), msg)
		if err != nil {
			return err
		}
		en.n = uint8(int(int16(ht)) / lf)
		for j := 0; ; {
			en.states[j] = 0
			j++
			if j < len(en.ids) {
				en.ids[j] = 0xff
			}
			if j >= int(en.n) {
				break
			}
		}
	}
	h.initListData()
	return nil
}

// printTurnSummaryPopupMessage is Print_Turn_Summary_Popup_Message_
// (0x10fa35), the list's item printer: a bullet and the message.
func (g *Game) printTurnSummaryPopupMessage(x, y int, id uint8) error {
	if id >= 0xfb {
		return nil
	}
	t := g.e.Text()
	t.SetLF(10)
	msg, err := g.sprintfMsg(g.state.Msgs[int(id)*msgSize : (int(id)+1)*msgSize])
	if err != nil {
		return err
	}
	if err := t.Print(x, y, cStringAt(g.exe, turnSumBullet)); err != nil {
		return err
	}
	return t.PrintFormatted(int(int16(x+0xd)), y, 0x140, msg, 0)
}

// drawTurnSummaryPopup is Draw_Turn_Summary_Popup_ (0x10faac), the auto
// function: the entry under the pointer becomes the selection, drawn in
// the high colours.
func (g *Game) drawTurnSummaryPopup() error {
	ts := &g.tsum
	h := &ts.header
	normal := h.colors[0]
	h.colors[0] = g.exeColors(turnSumHighColors)
	defer func() { h.colors[0] = normal }()
	f, err := g.e.ScanInput()
	if err != nil {
		return err
	}
	if f >= ts.firstField && f <= ts.lastField {
		h.setSelectedListEntry(f)
	}
	ts.sel = h.selectedListEntry()
	return g.displayListText(h, ts.sel)
}

// setListUpDownFieldDrawing is Set_List_Up_Down_Field_Drawing_ (0x10a689):
// the page buttons are drawn only when there is a page that way.
func (g *Game) setListUpDownFieldDrawing(h *listHeader, upF, downF int) {
	e := g.e
	if h.cur < h.pages {
		e.EnableDrawField(downF)
	} else {
		e.DisableDrawField(downF)
	}
	if h.cur < 2 {
		e.DisableDrawField(upF)
	} else {
		e.EnableDrawField(upF)
	}
}

// gotoMsgColony is Goto_Msg_Colony_ (0xef6f0): a colony message about one
// of the human's colonies opens the colony screen (returning to the info
// screen unless the caller says otherwise).
func (g *Game) gotoMsgColony(m []byte) bool {
	s := &g.state
	if m[0] != 0 {
		return false
	}
	c := s.Planets[m[2]].Colony
	if c == -1 || int16(s.Colonies[c].Owner) != s.PlayerNum {
		return false
	}
	P := &s.Planets[s.Colonies[c].Planet]
	g.currentScreen, g.returnScreen = ScreenColony, 9
	g.screenData[0], g.screenData[1] = int16(P.Star), int16(P.Orbit)
	return true
}
