package game

import "github.com/openorion2/openorion2/internal/gfx"

// _Tech_List_ (0x10e6c7): a group's whole tree of fields over the research
// choice, opened with a group button; it only shows (and explains on a
// right click), it picks nothing. Spec: docs/screens/tech-select.md.

const (
	techListHeader  = 0x184297 // _g_tech_list_header
	techColorBase   = 0x1ad5a4 // _tech_color
	techIndentItem  = 0x17a3a0 // "^"
	techListTitle   = 0x3f     // billtext.lbx
	techGroupNames0 = 0x40     // billtext.lbx 0x40..0x49
)

// techList is _Tech_List_ (0x10e6c7) for box en.
func (g *Game) techList(en *techEntry, changing bool) error {
	e, ts := g.e, &g.ts
	title, err := g.textMessage(billTextLBX, techListTitle, 0x50)
	if err != nil {
		return err
	}
	group, err := g.textMessage(billTextLBX, techGroupNames0+int(en.group), 0x14)
	if err != nil {
		return err
	}
	saved := e.GetAutoFunction()
	fields := e.SaveFieldStats()
	e.DisableCancel()
	first := 9
	if changing {
		first = 0x17
	}
	var pics [4]*gfx.Picture
	for i := range pics {
		if pics[i], err = g.picture(techSelectLBX, first+i); err != nil {
			return err
		}
	}
	frame, closed, upPic, downPic := pics[0], pics[1], pics[2], pics[3]
	listX := int(en.listX)
	h := g.readListHeader(techListHeader, 5)
	h.x = int16(listX + 0x10)
	h.arrowColor = ts.cursorColor
	h.print = g.printTechIndentItem
	behind := e.ScreenBitmapCapture(0x54, 0x28, 0x207, 0x1b7)
	// Tech_{Select,Change}_List_{Right,Left}_Help_ by the list's side.
	help := uint32(techSelectListLeft)
	switch right := int(uint16(en.listX))-int(uint16(ts.scrnX)) > 6; {
	case right && changing:
		help = techChangeListRight
	case right:
		help = techSelectListRight
	case changing:
		help = techChangeListLeft
	}
	g.setHelpListAt(help, 2)
	g.getGroupList(&h, en.group)
	h.initListData()
	upF, err := e.AddButtonField(listX+0xf7, 0x4d, "", upPic, "", 0)
	if err != nil {
		return err
	}
	downF, err := e.AddButtonField(listX+0xf8, 0x18e, "", downPic, "", 0)
	if err != nil {
		return err
	}
	counter := e.FieldsCount()
	draw := func() error { return g.drawTechList(&h) }
	e.AssignAutoFunction(draw, 4)
	for {
		g.fullDrawScreen = false
		e.V.Fill(0x54, 0x28, 0x207, 0x1b7, 0)
		if err := e.Draw(0x54, 0x28, behind); err != nil {
			return err
		}
		e.V.Fill(listX, 0x1f, listX+0x10b, 0x1c1, 0)
		if err := e.Draw(listX, 0x1f, frame); err != nil {
			return err
		}
		t := e.Text()
		t.SetStyle(4, ts.colors[0])
		if err := t.PrintCentered(listX+0x86, 0x2f, group+title); err != nil {
			return err
		}
		if err := g.displayListPage(&h); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		g.setListUpDownFieldDrawing(&h, upF, downF)
		if err := e.TogglePages(); err != nil {
			return err
		}
		e.MarkTime()
		e.ClearFieldsAbove(counter - 1)
		last, err := g.addFieldsToListPage(&h)
		if err != nil {
			return err
		}
		closeF, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "\x1b", 0)
		if err != nil {
			return err
		}
		if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0); err != nil {
			return err
		}
		if err := e.ReleaseTime(4); err != nil {
			return err
		}
		ts.listShowing = true
	input:
		for {
			in, err := e.GetInput()
			if err != nil {
				return err
			}
			if g.fullDrawScreen {
				break
			}
			v := int(int16(in))
			switch {
			case v < 1:
				if v < 0 && v != -closeF {
					v = -v
					if v >= counter && v <= last {
						if sel := h.selectedListEntry(); sel != nil {
							if app := sel.ids[sel.item]; app != 0 {
								if err := g.displaySelectedListText(&h, sel); err != nil {
									return err
								}
								if err := g.drawApplicationDescription(int(app)); err != nil {
									return err
								}
								break input
							}
						}
					}
				} else {
					e.MarkTime()
					if err := draw(); err != nil {
						return err
					}
					if err := e.TogglePages(); err != nil {
						return err
					}
					if err := e.ReleaseTime(4); err != nil {
						return err
					}
				}
			case v == downF && h.nextListPage() || v == upF && h.previousListPage():
				break input
			}
			if v == closeF {
				if err := e.Draw(listX+0xbb, 0x1b2, closed); err != nil {
					return err
				}
				e.MarkTime()
				if err := e.TogglePages(); err != nil {
					return err
				}
				if err := e.ReleaseTime(4); err != nil {
					return err
				}
				ts.listShowing = false
				e.PutAutoFunction(saved)
				e.RestoreFieldStats(fields)
				e.DeactivateHelpList()
				return nil
			}
		}
	}
}

// getGroupList is Get_Group_List_ (0x10f49b): the group's fields in chain
// order, those with something to show, each with its applications by
// status (4 for the field being researched, 3 for a maxed hyper field).
func (g *Game) getGroupList(h *listHeader, group uint16) {
	s := &g.state
	pl := &s.Players[s.PlayerNum]
	h.entries = h.entries[:0]
	var en listEntry
	for f := int(g.exe.bytes(firstFieldInGroup+uint32(group), 1)[0]); f != 0; f = int(uint16(g.techFieldNextOf(f))) {
		st := pl.FieldStatus[f]
		en.id = uint8(f)
		en.text, _ = g.technologyFieldsName(f)
		if uint16(pl.ResearchField) == uint16(f) {
			st = 4
		} else if f > 0x4a && pl.HyperLevels[f-0x4b] > 0x14 {
			st = 3
		}
		if st == 3 {
			continue
		}
		if st == 0 {
			for k := 0; k < 4; k++ {
				if pl.AppStatus[uint16(g.techFieldApp(f, k))] == techKnown {
					st = 1
					break
				}
			}
		}
		if st == 0 {
			continue
		}
		n := 0
		for k := 0; k < 4; k++ {
			a := uint8(g.techFieldApp(f, k))
			s2 := pl.AppStatus[a]
			if s2 == 0 {
				continue
			}
			if st == 4 || s2 == 1 && st == 2 {
				s2 = st
			}
			en.ids[n], en.states[n] = a, s2
			n++
		}
		if n != 0 {
			en.state, en.n = st, uint8(n)
			h.entries = append(h.entries, en)
			en = listEntry{}
		}
	}
}

// printTechIndentItem is Print_Tech_Indent_Item_ (0x10db17), the group
// list's item printer: "^" and the application's name.
func (g *Game) printTechIndentItem(x, y int, app uint8) error {
	name, err := g.technologyApplicationsName(int(app), int(g.state.PlayerNum))
	if err != nil {
		return err
	}
	return g.squeezePrint(x, y, cStringAt(g.exe, techIndentItem)+name, 0xd5)
}

// drawTechList is Draw_Tech_List_ (0x10ec96), the auto function.
func (g *Game) drawTechList(h *listHeader) error {
	e := g.e
	f, err := e.ScanInput()
	if err != nil {
		return err
	}
	if int16(f) > 0 {
		h.setSelectedListEntry(f)
	}
	if sel := h.selectedListEntry(); sel != nil {
		t := e.Text()
		t.SetStyle(h.style, h.selColors)
		if err := g.squeezePrint(int(sel.x), int(sel.y), sel.text, int(h.width)); err != nil {
			return err
		}
		t.SetStyle(h.itemStyle, h.selColors)
		if err := h.print(int(sel.itemX), int(sel.y+sel.itemY[sel.item]), sel.ids[sel.item]); err != nil {
			return err
		}
	}
	e.CyclePaletteColor(int(g.ts.cursorColor), 8, 0x20, 0, 0x28, 0x3f, 8, 8)
	return nil
}

// displaySelectedListText is Display_Selected_List_Text_ (0x10a21d): the
// entry and its selected item in the selected colours, with an arrow.
func (g *Game) displaySelectedListText(h *listHeader, en *listEntry) error {
	t := g.e.Text()
	t.SetStyle(h.style, h.selColors)
	if err := g.squeezePrint(int(en.x), int(en.y), en.text, int(h.width)); err != nil {
		return err
	}
	if h.print == nil {
		return nil
	}
	t.SetStyle(h.itemStyle, h.selColors)
	iy := en.y + en.itemY[en.item]
	if err := h.print(int(en.itemX), int(iy), en.ids[en.item]); err != nil {
		return err
	}
	if c := h.arrowColor; c != 0 {
		v := g.e.V
		x1 := int(en.x + 2)
		y2 := int(iy + 6)
		x2 := int(en.itemX - 2)
		v.Line(x1, int(en.y+en.itemY[0]-3), x1, y2, c)
		v.Line(x1, y2, x2, y2, c)
		v.Line(int(en.itemX-5), int(iy+3), x2, y2, c)
		v.Line(int(en.itemX-5), int(iy+9), x2, y2, c)
	}
	return nil
}
