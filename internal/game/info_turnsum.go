package game

// The turn summary page of the info screen (INFO.C): the human's messages
// of the last turn as a paged list. Spec: docs/screens/info-screen.md,
// "Turn summary".

const (
	infoTurnHeader = 0x183ea3 // _g_turn_header
	infoTurnRows   = 15       // _turn_indent_y, _turn_field_y2
	infoTurnBullet = 0x17a2f9 // "^"
)

// turnSummarySubscreen is Turn_Summary_Subscreen_ (0x108f98).
func (g *Game) turnSummarySubscreen(first int) (int, error) {
	e, is, s := g.e, &g.info, &g.state
	empty, err := g.textMessage(billTextLBX, 0x10, 0x28)
	if err != nil {
		return 0, err
	}
	title, err := g.textMessage(billTextLBX, 8, 0x28)
	if err != nil {
		return 0, err
	}
	t := e.Text()
	if err := t.PrintCentered(0x1a0, 0x1f, title); err != nil {
		return 0, err
	}
	year, err := g.textMessage(billTextLBX, 0x1a, 0x3c)
	if err != nil {
		return 0, err
	}
	if year, err = g.decodeTextMessage(year, -1, -1, -1, -1, -1, -1, -1, 0x3c); err != nil {
		return 0, err
	}
	n := int(int16(s.NMsgs[s.PlayerNum]))
	h, ls := &is.turnHead, &is.list
	var upF, downF int
	if n > 0 {
		if upF, err = e.AddButtonField(0x250, 0x54, "", is.pics[0xe], "\x1b", 0x28); err != nil {
			return 0, err
		}
		if downF, err = e.AddButtonField(0x250, 0x18b, "", is.pics[0xf], "\x1b", 0x28); err != nil {
			return 0, err
		}
		*h = g.readListHeader(infoTurnHeader, infoTurnRows)
		h.print = g.printTurnSummaryMessage
		if err := g.initTurnSummaryList(h, n, 4, 10); err != nil {
			return 0, err
		}
		ls.sel = &h.entries[h.selectAndPageByHeadID(is.msgIx)]
		ls.header = h
	}
	for {
		g.drawDropArea(infoTurnSumDrop)
		t.SetStyle(4, g.exeColors(infoNormalColors))
		if err := t.PrintCentered(0x19e, 0x42, year); err != nil {
			return 0, err
		}
		e.V.Line(0xee, 0x51, 0x244, 0x51, 0xb8)
		if err := e.Draw(0x250, 0x54, is.pics[0xc]); err != nil {
			return 0, err
		}
		if err := e.Draw(0x250, 0x18b, is.pics[0xd]); err != nil {
			return 0, err
		}
		if n == 0 {
			t.SetStyle(4, g.exeColors(infoNormalColors))
			if err := t.PrintCentered(0x19e, 0xd8, empty); err != nil {
				return 0, err
			}
			e.AssignAutoFunction(func() error { return nil }, 2) // Draw_Dummy_Turn_Summary_Subscreen_
			e.V.CopyOffToBack()
			if err := g.introduceSubscreen(); err != nil {
				return 0, err
			}
			is.showing = infoTurnSummary
			for {
				in, err := e.GetInput()
				if err != nil {
					return 0, err
				}
				if g.fullDrawScreen {
					return 0, nil
				}
				e.MarkTime()
				if err := e.TogglePages(); err != nil {
					return 0, err
				}
				if err := e.ReleaseTime(2); err != nil {
					return 0, err
				}
				if r := int(int16(in)); r != 0 {
					return r, nil
				}
			}
		}
		e.ClearFieldsAbove(downF)
		ls.firstField = e.FieldsCount()
		if ls.lastField, err = g.addFieldsToListPage(h); err != nil {
			return 0, err
		}
		if err := g.displayListPage(h); err != nil {
			return 0, err
		}
		g.setListUpDownFieldDrawing(h, upF, downF)
		e.AssignAutoFunction(g.drawInfoTurnSummary, 2)
		e.V.CopyOffToBack()
		if err := g.introduceSubscreen(); err != nil {
			return 0, err
		}
		is.showing = infoTurnSummary
	input:
		for {
			in, err := e.GetInput()
			if err != nil {
				return 0, err
			}
			if g.fullDrawScreen {
				return 0, nil
			}
			r := int(int16(in))
			switch {
			case r >= ls.firstField && r <= ls.lastField && ls.sel != nil:
				m := s.Msgs[int(ls.sel.id)*msgSize:]
				if g.gotoMsgColony(m) {
					g.currentScreen, g.returnScreen = ScreenColony, ScreenInfo
					is.msgIx = ls.sel.id
					r = is.exitField
				}
			case r >= first && n > 0:
				if r == downF && h.nextListPage() || r == upF && h.previousListPage() {
					break input
				}
			case r == 0:
				e.MarkTime()
				if err := g.drawInfoTurnSummary(); err != nil {
					return 0, err
				}
				if err := e.TogglePages(); err != nil {
					return 0, err
				}
				if err := e.ReleaseTime(2); err != nil {
					return 0, err
				}
			}
			if r != 0 && r < first {
				return r, nil
			}
		}
	}
}

// drawInfoTurnSummary is Draw_Turn_Summary_Subscreen_ (0x1092bd), the auto
// function: the hovered message is drawn in the high colours.
func (g *Game) drawInfoTurnSummary() error {
	e, ls := g.e, &g.info.list
	h := ls.header
	h.colors[0] = g.exeColors(infoHighColors)
	f, err := e.ScanInput()
	if err != nil {
		return err
	}
	if f = int(int16(f)); f >= ls.firstField && f <= ls.lastField {
		h.setSelectedListEntry(f)
	}
	ls.sel = h.selectedListEntry()
	err = g.displayListText(h, ls.sel)
	h.colors[0] = g.exeColors(infoNormalColors)
	if err != nil {
		return err
	}
	e.CyclePaletteColor(0xbf, 8, 0x20, 0, 0x28, 0x3f, 8, 8)
	return nil
}

// printTurnSummaryMessage is Print_Turn_Summary_Message_ (0x108e63), the
// list's item printer: "^" and the message.
func (g *Game) printTurnSummaryMessage(x, y int, id uint8) error {
	if id > 0xfa {
		return nil
	}
	t := g.e.Text()
	t.SetLF(10)
	msg, err := g.sprintfMsg(g.state.Msgs[int(id)*msgSize : (int(id)+1)*msgSize])
	if err != nil {
		return err
	}
	if err := t.Print(x, y, cStringAt(g.exe, infoTurnBullet)); err != nil {
		return err
	}
	return t.PrintFormatted(int(int16(x+0xd)), y, 0x159, msg, 0)
}
