package game

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The research choice (TECH.C): Tech_Select_ after a breakthrough or for
// the first field, and Tech_Change_ (screen 0x24). Eight boxes, one per
// technology group, offer the next researchable field and its
// applications. Spec: docs/screens/tech-select.md.

const (
	techSelectLBX     = "TECHSEL.LBX"
	techSelectPos     = 0x1841b5 // _tech_select_pos, 8 x (x, y)
	techChangePos     = 0x1841d5 // _tech_change_pos
	techFieldY1       = 0x1841f5 // _tech_field_y1, 4 shorts
	techFieldY2       = 0x1841fd
	techAppNameY      = 0x184205 // _app_name_y
	techListWindowX   = 0x18420f // _list_window_x_offsets, 8 shorts
	techButtonPos     = 0x18421f // _tech_button_pos, 8 x (x, y)
	techEntryToGroup  = 0x18423f // _entry_to_group, 8 bytes
	techSelectColors  = 0x184247 // _tech_select_colors, 5 x 8 shades
	techChangeColors  = 0x18426f // _tech_change_colors
	firstFieldInGroup = 0x17cf18 // _first_field_in_group
	techCostFmt       = 0x17a3cc // "%i RP", "%i FP", "%i PR"
	techCostSuffix    = 0x17a3de // " RP", " FP", " PR"
	techResearchDesc  = 0x1841a2 // _research_desc_fmt
	romanNumerals     = 0x17d5e0 // pointers, 1..20
	hyperFieldTitleE  = 0x284    // the E string _hyper_field_title points at
	// Help lists (10-byte records) of Technology_Select_Help_ (0x103a2d),
	// Technology_Change_Help_ (0x103a3f) and the four list helps.
	technologySelectHelp = 0x183718
	technologyChangeHelp = 0x183736
	techSelectListRight  = 0x183754 // Tech_Select_List_Right_Help_
	techSelectListLeft   = 0x183768 // Tech_Select_List_Left_Help_
	techChangeListRight  = 0x18377c // Tech_Change_List_Right_Help_
	techChangeListLeft   = 0x183790 // Tech_Change_List_Left_Help_
	numTechEntries       = 8
)

// techEntry is one box of the selection screen (0x188 bytes in the
// original; offsets in the spec).
type techEntry struct {
	x, y      int16  // +0
	group     uint16 // +4
	selected  bool   // +6
	app       uint16 // +7, the selected row
	name      string // +9, the field
	radio     int    // +0x49, the group button
	radioVar  int16  // +0x4b
	pic       *gfx.Picture
	field     int16     // +0x51, 0 for an empty box
	cost      int32     // +0x53
	costText  string    // +0x57
	napps     uint8     // +0x6b
	appNames  [4]string // +0x6c, 0x40 each
	appX      int16     // +0x16c
	appY      [4]int16  // +0x16e
	appFields [4]int16  // +0x176, the rows' hidden fields
	apps      [4]int16  // +0x17e
	listX     int16     // +0x186
}

// techSelectState holds TECH.C's globals.
type techSelectState struct {
	entries      [numTechEntries]techEntry // _g_entries_p
	changing     bool                      // _g_changing_tech
	scrnX, scrnY int16                     // _g_scrn_x, _g_scrn_y
	cursorColor  byte                      // _g_cursor_box_color
	colors       [5]font.Colors            // _tech_color
	firstField   int                       // _g_first_tech_field
	lastField    int                       // _g_last_tech_field
	savedHyper   [8]uint8                  // _saved_hyper_advanced_tech
	showing      bool                      // port-only: in the input loop
	listShowing  bool                      // port-only: in _Tech_List_'s loop
	radioFirst   int                       // port-only: the first group button
}

// techSelect is Tech_Select_ (0x10db69): the scientist shows what was
// researched, then the player picks the next field.
func (g *Game) techSelect() error {
	e := g.e
	e.SavePalette()
	if err := g.showOffResearchedTech(int(g.state.PlayerNum)); err != nil {
		return err
	}
	if err := g.techSelectScreen(false); err != nil {
		return err
	}
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	e.RestorePalette()
	g.turnOffPalettes()
	e.V.ClearPaletteChanges(0, 0xff)
	e.DeactivateHelpList()
	g.resetMousePicture()
	g.playBackgroundMusic() // 0x10dbc7
	return nil
}

// techChange is Tech_Change_ (0x10dbce), screen 0x24: the research choice
// over the main screen, opened from its research panel.
func (g *Game) techChange() error {
	e := g.e
	e.V.SetPageOff()
	if err := e.DrawVisibleFields(); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	if err := g.techSelectScreen(true); err != nil {
		return err
	}
	g.currentScreen = g.returnScreen
	return nil
}

// setHelpListAt is the Set_Help_List_ tail the help routines share.
func (g *Game) setHelpListAt(addr uint32, n int) {
	g.e.DeactivateHelpList()
	g.e.SetHelpList(g.helpListAt(addr, n))
}

// listHotKey is the hot key Options.ListKey gives a field whose own is "".
func (g *Game) listHotKey(key string) string {
	if g.listKey {
		return key
	}
	return ""
}

// billPictureToMouse is Bill_Picture_To_Mouse_ (0x24e96): pic becomes the
// arrow cursor.
func (g *Game) billPictureToMouse(pic *gfx.Picture) {
	e := g.e
	bm := gfx.NewBlankBitmap(1, 1)
	bm.DrawPictureToBitmap(pic)
	e.ClearMouseImage(0)
	e.DrawBitmapToMouse(0, 0, bm, 0)
	e.SetMouseList(engine.MouseArrow)
}

// techSelectScreen is _Tech_Select_ (0x10dc12), changing for Tech_Change_.
func (g *Game) techSelectScreen(changing bool) error {
	e, ts, s := g.e, &g.ts, &g.state
	p := int(s.PlayerNum)
	pl := &s.Players[p]
	e.ClearFields()
	e.V.SetRefreshStencil()
	saved := e.GetAutoFunction()
	ts.savedHyper = pl.HyperLevels
	for i := range pl.HyperLevels {
		pl.HyperLevels[i]++
	}
	e.DisableCancel()
	ts.changing = changing
	oldField := pl.ResearchField
	var panel, done *gfx.Picture
	var pics [numTechEntries]*gfx.Picture
	first, colors := 0, uint32(techSelectColors)
	ts.scrnX, ts.scrnY, ts.cursorColor = 0xa1, 0, 0xff
	if changing {
		first, colors = 0xe, techChangeColors
		ts.scrnX, ts.cursorColor = 0x50, 0xef
	}
	for i := range ts.colors {
		ts.colors[i] = g.exeColors(colors + uint32(8*i))
	}
	var err error
	if panel, err = g.picture(techSelectLBX, first); err != nil {
		return err
	}
	for i := range pics {
		if pics[i], err = g.picture(techSelectLBX, first+1+i); err != nil {
			return err
		}
	}
	doneEntry := 0xd
	if changing {
		doneEntry = 0x1b
	}
	if done, err = g.picture(techSelectLBX, doneEntry); err != nil {
		return err
	}
	doneField := -1
	var count int
	if changing {
		if doneField, err = e.AddButtonField(int(ts.scrnX)+0xbd, int(ts.scrnY)+0x1c4, "", done, "\x1b", 0x28); err != nil {
			return err
		}
		pl.ResearchField = 0
		g.initEntryPositions()
		if count, err = g.initEntryData(pl.ResearchPoints); err != nil {
			return err
		}
		pl.ResearchField = oldField
	} else {
		g.initEntryPositions()
		if pl.ResearchField >= 0x4b {
			pl.ResearchField = 0
		}
		if count, err = g.initEntryData(0); err != nil {
			return err
		}
	}
	ts.firstField = e.FieldsCount()
	for i := range ts.entries {
		en := &ts.entries[i]
		if ts.lastField, err = e.AddHiddenField(int(en.x)-2, int(en.y)+0x12, int(en.x)+0xd7, int(en.y)+0x63, "", 0); err != nil {
			return err
		}
	}
	if int16(count) == 0 {
		return g.techSelectExit(saved)
	}
	btnFirst := e.FieldsCount()
	if err := g.setupEntryButtons(pics); err != nil {
		return err
	}
	btnLast := e.FieldsCount() - 1
	ts.radioFirst = btnFirst
	if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0); err != nil {
		return err
	}
	var sciencePal *gfx.Picture
	if !changing {
		room, err := g.picture(fmt.Sprintf("SR_R%x_SC.LBX", pl.Race), 0)
		if err != nil {
			return err
		}
		e.OpenFileAnimation(room)
		if sciencePal, err = g.picture(scienceLBX, 0); err != nil {
			return err
		}
	}
redraw:
	for {
		if !changing {
			e.V.SetPageOff()
			e.V.Fill(0, 0, 0x27f, 0x1df, 0)
			frame := 0
			if pl.Race == 7 {
				frame = 0x11
			}
			e.SetFileAnimationFrame(frame)
			if err := e.DrawFileAnimation(e.FileAnimation()); err != nil {
				return err
			}
			e.V.Fill(int(ts.scrnX)+4, int(ts.scrnY)+4, int(ts.scrnX)+0x1d7, int(ts.scrnY)+0x1d8, 0)
			e.DrawPalette(panel)
			e.RefreshPalette()
			e.DrawPalette(sciencePal)
			e.RefreshPalette()
		}
		e.SetInputDelay(5)
		if changing {
			g.setHelpListAt(technologyChangeHelp, 3)
		} else {
			g.setHelpListAt(technologySelectHelp, 3)
		}
		g.fullDrawScreen = false
		e.AssignAutoFunction(g.drawTechSelect, 4)
		e.V.Fill(int(ts.scrnX)+4, int(ts.scrnY)+4, int(ts.scrnX)+0x1d7, int(ts.scrnY)+0x1d8, 0)
		if err := e.Draw(int(ts.scrnX), int(ts.scrnY), panel); err != nil {
			return err
		}
		for i := range ts.entries {
			if err := g.displayEntryText(&ts.entries[i]); err != nil {
				return err
			}
		}
		e.V.CopyOffToBack()
		if err := g.drawTechSelect(); err != nil {
			return err
		}
		if !changing {
			g.billPictureToMouse(done)
		}
		if err := e.FastFadeIn(); err != nil {
			return err
		}
		e.ResetCyclePaletteColor()
		e.Sys.MouseBuffer()
		e.Sys.MouseBuffer2()
		ts.showing = true
		for {
			e.MoxUpdate()
			if !changing {
				g.playStreamingMusic(0x75, -2, true) // 0x10e297
			}
			in, err := e.GetInput()
			if err != nil {
				return err
			}
			if g.fullDrawScreen {
				continue redraw
			}
			v := int(int16(in))
			if v < 0 {
				v = -v
				if v < btnFirst || v > btnLast {
					if g.setSelectedEntry(v) != nil {
						sel := g.selectedEntry()
						if app := sel.apps[sel.app]; app != 0 {
							if err := g.displaySelectedEntryText(sel); err != nil {
								return err
							}
							if err := g.drawApplicationDescription(int(app)); err != nil {
								return err
							}
							continue redraw
						}
					}
					v = 0
				}
			}
			switch {
			case v <= 0:
				e.MarkTime()
				if err := g.drawTechSelect(); err != nil {
					return err
				}
				if err := e.ReleaseTime(2); err != nil {
					return err
				}
				if err := e.TogglePages(); err != nil {
					return err
				}
			case v > doneField && v < btnFirst:
				sel := g.selectedEntry()
				for n := 3; n > 0; n-- {
					e.MarkTime()
					if n&1 == 0 {
						if err := g.displaySelectedEntryText(sel); err != nil {
							return err
						}
					}
					if err := e.TogglePages(); err != nil {
						return err
					}
					if err := e.ReleaseTime(4); err != nil {
						return err
					}
				}
				if sel == nil {
					return fmt.Errorf("_Tech_Select_: a row was picked with nothing selected")
				}
				pl.ResearchField = uint8(sel.field)
				pl.ResearchApp = uint8(sel.apps[sel.app])
				v = doneField
			case v >= btnFirst && v <= btnLast:
				i := v - btnFirst
				en := &ts.entries[i]
				setFrame(en.pic, 1)
				x := int(ts.scrnX + g.exeI16(uint32(techButtonPos+4*i)))
				y := int(ts.scrnY + g.exeI16(uint32(techButtonPos+4*i+2)))
				if err := e.Draw(x, y, en.pic); err != nil {
					return err
				}
				e.V.CopyOffToBack()
				en.radioVar = 0
				if err := g.techList(en, changing); err != nil {
					return err
				}
				continue redraw
			}
			if v == doneField {
				ts.showing = false
				return g.techSelectExit(saved)
			}
		}
	}
}

// techSelectExit is _Tech_Select_'s tail: all eight hyper levels are put
// back (the loop's index is the edx zeroed for Put_Auto_Function_).
func (g *Game) techSelectExit(saved engine.AutoFunction) error {
	e, ts, s := g.e, &g.ts, &g.state
	pl := &s.Players[s.PlayerNum]
	e.PutAutoFunction(saved)
	e.ClearFields()
	pl.HyperLevels = ts.savedHyper
	e.DeactivateHelpList()
	return nil
}

// initEntryPositions is Init_Entry_Positions_ (0x10efc3).
func (g *Game) initEntryPositions() {
	ts := &g.ts
	pos := uint32(techSelectPos)
	if ts.changing {
		pos = techChangePos
	}
	for i := range ts.entries {
		en := &ts.entries[i]
		*en = techEntry{}
		en.group = uint16(g.exe.bytes(techEntryToGroup+uint32(i), 1)[0])
		en.x = g.exeI16(pos + uint32(4*i))
		en.y = g.exeI16(pos + uint32(4*i+2))
		en.appX = en.x + 10
		en.listX = g.exeI16(techListWindowX+uint32(2*i)) + ts.scrnX
		for k := 0; k < 4; k++ {
			en.appY[k] = g.exeI16(techAppNameY+uint32(2*k)) + 0x15 + en.y
		}
	}
}

// initEntryData is Init_Entry_Data_ (0x10f089): each box gets the first
// researchable field of its group (not the one being researched), its
// obtainable applications as rows with hidden fields, and the cost less
// spent. Returns the number of boxes filled.
func (g *Game) initEntryData(spent int32) (int, error) {
	ts, s := &g.ts, &g.state
	p := int(s.PlayerNum)
	pl := &s.Players[p]
	nothing, err := g.textMessage(billTextLBX, 0x3e, 0x50)
	if err != nil {
		return 0, err
	}
	e := g.e
	n := 0
	for i := range ts.entries {
		en := &ts.entries[i]
		f := int(g.exe.bytes(firstFieldInGroup+uint32(en.group), 1)[0])
		for f != 0 && !(pl.FieldStatus[f] == techResearchable && f != int(pl.ResearchField)) {
			f = int(uint16(g.techFieldNextOf(f)))
		}
		if f == 0 {
			continue
		}
		rows := 0
		addRow := func(k int) error {
			y1 := int(en.y + g.exeI16(techFieldY1+uint32(2*k)) + 0x15)
			y2 := int(en.y + g.exeI16(techFieldY2+uint32(2*k)) + 0x15)
			id, err := e.AddHiddenField(int(en.x), y1, int(en.x)+0xda, y2, g.listHotKey("1"), 0)
			en.appFields[k] = int16(id)
			return err
		}
		if f < 0x4b {
			for k := 0; k < 4 && rows < 4; k++ {
				a := g.techFieldApp(f, k)
				if pl.AppStatus[uint16(a)] != techObtainable {
					continue
				}
				name, err := g.technologyApplicationsName(a, p)
				if err != nil {
					return 0, err
				}
				en.appNames[rows] = name
				en.apps[rows] = int16(a)
				if err := addRow(rows); err != nil {
					return 0, err
				}
				rows++
			}
			if rows == 0 {
				en.appNames[0] = nothing
				en.apps[0] = 0
				rows = 1
				if err := addRow(0); err != nil {
					return 0, err
				}
			}
			en.napps = uint8(rows)
		} else {
			a := hyperFieldToPseudoApp(f)
			name, err := g.technologyApplicationsName(a, p)
			if err != nil {
				return 0, err
			}
			en.appNames[0] = name
			en.apps[0] = int16(a)
			if err := addRow(0); err != nil {
				return 0, err
			}
			en.napps = 1
		}
		en.field = int16(f)
		en.cost = max(g.researchCost(p, f)-spent, 0)
		en.costText = cSprintf(g.techCostFormat(techCostFmt, 6), en.cost)
		name, err := g.technologyFieldsName(f)
		if err != nil {
			return 0, err
		}
		en.name = name
		n++
	}
	return n, nil
}

// techCostFormat picks the language's variant of a cost string (stride
// apart: RP for English, Spanish and Italian... as the switch has it).
func (g *Game) techCostFormat(base uint32, stride int) string {
	i := 0
	switch g.settings[0xd2] {
	case 1:
		i = 1
	case 4:
		i = 2
	}
	return cStringAt(g.exe, base+uint32(i*stride))
}

// hyperFieldToPseudoApp is Hyper_Field_To_Pseudo_App_Index_ (0x1032fb).
func hyperFieldToPseudoApp(f int) int {
	if int16(f) >= 0x4b {
		return f + 0x81
	}
	return f
}

// technologyFieldsName is Technology_Fields_Name_ (0x10f772): the field's
// name, or the hyper-advanced title for fields 0x4b and up.
func (g *Game) technologyFieldsName(f int) (string, error) {
	if int16(f) < 0x4b {
		return g.techName(f)
	}
	es, err := g.eStrings()
	if err != nil {
		return "", err
	}
	return es[hyperFieldTitleE], nil
}

// setupEntryButtons is Setup_Entry_Buttons_ (0x10f409): each filled box
// gets its group button, a radio button over its variable.
func (g *Game) setupEntryButtons(pics [numTechEntries]*gfx.Picture) error {
	ts := &g.ts
	for i := range ts.entries {
		en := &ts.entries[i]
		if en.field == 0 {
			continue
		}
		en.radioVar = 0
		x := int(ts.scrnX + g.exeI16(uint32(techButtonPos+4*i)))
		y := int(ts.scrnY + g.exeI16(uint32(techButtonPos+4*i+2)))
		id, err := g.e.AddRadioButtonField(x, y, "", pics[i], &en.radioVar, g.listHotKey("2"), 0x28)
		if err != nil {
			return err
		}
		en.radio = id
		en.pic = pics[i]
	}
	return nil
}

// setSelectedEntry is Set_Selected_Entry_ (0x10ef45): the row whose hidden
// field is f becomes the only selection; nil when no row has it.
func (g *Game) setSelectedEntry(f int) *techEntry {
	var found *techEntry
	for i := range g.ts.entries {
		en := &g.ts.entries[i]
		en.selected = false
		if found != nil || en.field == 0 {
			continue
		}
		for k := 0; k < int(en.napps); k++ {
			if int16(f) == en.appFields[k] {
				en.selected, en.app, found = true, uint16(k), en
			}
		}
	}
	return found
}

// selectedEntry is Get_Selected_Entry_ (0x10efa9).
func (g *Game) selectedEntry() *techEntry {
	for i := range g.ts.entries {
		if g.ts.entries[i].selected {
			return &g.ts.entries[i]
		}
	}
	return nil
}

// drawTechSelect is Draw_Tech_Select_ (0x10e4b2), the auto function: the
// row under the pointer (or the last row of the box under it) is selected,
// drawn highlighted with its arrow, and the cursor box colour cycles.
func (g *Game) drawTechSelect() error {
	e, ts := g.e, &g.ts
	f, err := e.ScanInput()
	if err != nil {
		return err
	}
	if f >= ts.firstField && f <= ts.lastField {
		en := &ts.entries[f-ts.firstField]
		if en.napps == 0 {
			f = int(en.appY[3]) // the word before appFields[0]
		} else {
			f = int(en.appFields[en.napps-1])
		}
	}
	if int16(f) > 0 {
		g.setSelectedEntry(f)
	}
	sel := g.selectedEntry()
	if err := g.displaySelectedEntryText(sel); err != nil {
		return err
	}
	g.drawLittleArrow(sel)
	e.CyclePaletteColor(int(ts.cursorColor), 8, 0x20, 0, 0x28, 0x3f, 8, 8)
	return nil
}

// allAppsField is the test Display_Entry_Text_ and Draw_Little_Arrow_
// share: creative races and these fields get every application at once.
func (g *Game) allAppsField(f int16) bool {
	pl := &g.state.Players[g.state.PlayerNum]
	return pl.Picks[save.PickCreative] == 1 || f == 0x37 || f == 0x39 || f == 0x1d || f == 0x16 || f == 0x1c || f == 0x17
}

// drawLittleArrow is Draw_Little_Arrow_ (0x10e563): the arrow pointing at
// the selected row (every row for all-application fields).
func (g *Game) drawLittleArrow(en *techEntry) {
	if en == nil || !en.selected || en.field == 0 {
		return
	}
	v, c := g.e.V, g.ts.cursorColor
	k, n := int16(en.app), int16(en.app)+1
	if g.allAppsField(en.field) {
		k, n = 0, int16(en.napps)
	}
	for ; k < n; k++ {
		x2 := int(en.appX - 2)
		ay := en.appY[k]
		y2 := int(ay + 5)
		x1 := int(en.appX - 5)
		xa := int(en.appX - 9)
		v.Line(xa, int(en.appY[0]-3), xa, y2, c)
		v.Line(xa, y2, x2, y2, c)
		v.Line(x1, int(ay+2), x2, y2, c)
		v.Line(x1, int(ay+8), x2, y2, c)
	}
}

// displayEntryText is Display_Entry_Text_ (0x10ed00): a box in the normal
// shades, or in the third set when it holds the field being researched.
func (g *Game) displayEntryText(en *techEntry) error {
	ts := &g.ts
	pl := &g.state.Players[g.state.PlayerNum]
	if en.field == 0 {
		return nil
	}
	t := g.e.Text()
	cur := uint16(en.field) == uint16(pl.ResearchField)
	c := ts.colors[0]
	if cur {
		c = ts.colors[2]
	}
	t.SetStyle(4, c)
	if err := g.squeezePrint(int(en.x), int(en.y+0x15), en.name, 0xda); err != nil {
		return err
	}
	t.SetStyle(3, c)
	if err := t.PrintRight(int(en.x+0xd4), int(en.y+2), en.costText); err != nil {
		return err
	}
	all := cur && g.allAppsField(en.field)
	current := ""
	if cur {
		var err error
		if current, err = g.techAppName(int(pl.ResearchApp)); err != nil {
			return err
		}
	}
	for k := 0; ; {
		c2 := ts.colors[0]
		if cur && (all || strings.EqualFold(en.appNames[k], current)) {
			c2 = ts.colors[2]
		}
		t.SetStyle(3, c2)
		if err := g.squeezePrint(int(en.appX), int(en.appY[k]), en.appNames[k], 0xd0); err != nil {
			return err
		}
		k++
		if k >= int(en.napps) {
			break
		}
	}
	return nil
}

// displaySelectedEntryText is Display_Selected_Entry_Text_ (0x10ee6f): the
// box in the second shades with the selected row, framed in the cycling
// colour.
func (g *Game) displaySelectedEntryText(en *techEntry) error {
	ts := &g.ts
	if en == nil || !en.selected || en.field == 0 {
		return nil
	}
	t := g.e.Text()
	t.SetStyle(4, ts.colors[1])
	if err := g.squeezePrint(int(en.x), int(en.y+0x15), en.name, 0xda); err != nil {
		return err
	}
	t.SetStyle(3, ts.colors[1])
	if err := t.PrintRight(int(en.x+0xd4), int(en.y+2), en.costText); err != nil {
		return err
	}
	if k := int(en.app); k < int(en.napps) {
		if err := g.squeezePrint(int(en.appX), int(en.appY[k]), en.appNames[k], 0xd0); err != nil {
			return err
		}
	}
	g.e.Box(int(en.x-2), int(en.y+0x12), 0xda, 0x52, 1, ts.cursorColor)
	return nil
}

// drawApplicationDescription is Draw_Application_Description_ (0x10f62f):
// the help text of application app and its field's cost in a text box.
func (g *Game) drawApplicationDescription(app int) error {
	e := g.e
	e.DeactivateAutoFunction()
	e.DisableDrawAllFields()
	rec, err := g.helpRecord(app)
	if err != nil {
		return err
	}
	field := int(g.exeI16(uint32(techAppTable + app*techAppRow + techAppField)))
	cost := g.researchCost(int(g.state.PlayerNum), field)
	label, err := g.textMessage(billTextLBX, 0x3d, 0x50)
	if err != nil {
		return err
	}
	label += strconv.Itoa(int(cost)) + g.techCostFormat(techCostSuffix, 4)
	msg := cSprintf(cStringAt(g.exe, techResearchDesc), save.CString(rec.text), label)
	title := save.CString(rec.title)
	if err := g.textBox(&title, msg); err != nil {
		return err
	}
	e.ActivateAutoFunction()
	e.Sys.MouseBuffer()
	e.Sys.MouseBuffer2()
	e.EnableDrawAllFields()
	return nil
}

// squeezePrint is Squeeze_Print_ (0x102f10), _Squeeze_Print_ (0x102dab)
// with left alignment: text wider than maxw is narrowed (space width down
// to the style's limit, then the glyph spacing down to 1, then smaller
// styles), printed, and the font put back.
func (g *Game) squeezePrint(x, y int, s string, maxw int) error {
	return g.squeezePrintTyp(x, y, s, maxw, engine.ParaLeft)
}

// squeezePrintTyp is _Squeeze_Print_ on screen with alignment typ; a
// smaller style is centred on the original line height.
func (g *Game) squeezePrintTyp(x, y int, s string, maxw, typ int) error {
	t := g.e.Text()
	b := []byte(s)
	if t.Width(b) <= maxw {
		return g.printStringBill(x, y, maxw, s, typ)
	}
	limits := g.exe.bytes(spaceLimits, 6)
	style, space, hsp, shadow := t.Style, t.SpaceWidth(), t.Spacing, t.Shadow
	h0, dy := t.Height(), 0
	smaller := false
	for {
		for t.Style < len(limits) && limits[t.Style] < byte(t.SpaceWidth()) && t.Width(b) > maxw {
			t.SetSpaceWidth(t.SpaceWidth() - 1)
		}
		for t.Width(b) > maxw && t.Spacing > 1 {
			t.SetSpacing(t.Spacing - 1)
		}
		if t.Width(b) <= maxw || t.Style < 1 {
			break
		}
		t.SetStyle(t.Style-1, t.Normal)
		dy = (int(uint16(h0)) - int(int16(t.Height()))) / 2
		smaller = true
	}
	t.Shadow = shadow
	err := g.printStringBill(x, int(int16(y+dy)), maxw, s, typ)
	t.SetSpacing(hsp)
	t.SetSpaceWidth(space)
	if smaller {
		t.SetStyle(style, t.Normal)
		t.Shadow = shadow
	}
	return err
}

// printStringBill is _Print_String_Bill_ (0x1035af) for screen text:
// nothing for a width outside 10..640 or a corner off screen.
func (g *Game) printStringBill(x, y, w int, s string, typ int) error {
	if int16(w) < 10 || int16(w) > 0x280 || int16(x) < 0 || int16(x) >= 0x280 || int16(y) < 0 || int16(y) >= 0x1e0 {
		return nil
	}
	t := g.e.Text()
	switch typ {
	case engine.ParaRight:
		return t.PrintRight(int(int16(x+w-1)), y, s)
	case engine.ParaCentered:
		return t.PrintCentered(int(int16(x+w/2)), y, s)
	}
	return t.Print(x, y, s)
}
