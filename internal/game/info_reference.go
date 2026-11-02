package game

import (
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The reference page of the info screen (INFO.C): the help categories and
// "how to" topics, a category's topics as a list, a topic's text. Spec:
// docs/screens/info-screen.md, "Reference".

const (
	helpCategoryLabels = 0x192c08 // _help_category_labels, E string pointers
	referenceItemFmt   = 0x17a2fb // "^ %s"
	infoCatHeader      = 0x183ee7 // _g_category_header
	infoCatDrops       = 0x183d73 // the category page's drop areas
	helpHowToEntry     = 0xf      // the help file's entry of "how to" topics
)

// helpTable is Far_Reload_Data's view of the help file's entry e.
func (g *Game) helpTable(e int) (count int, rec func(n int) []byte, err error) {
	a, err := g.archive(g.helpLBXName())
	if err != nil {
		return 0, nil, err
	}
	tab, err := a.Table(e)
	if err != nil {
		return 0, nil, err
	}
	return tab.Count, func(n int) []byte {
		if n < 0 || n >= tab.Count {
			g.logOnce(fmt.Sprintf("help entry %d record %d out of range", e, n))
			return make([]byte, tab.Size)
		}
		return tab.Record(n)
	}, nil
}

// referenceSubscreen is Reference_Subscreen_ (0x109331).
func (g *Game) referenceSubscreen(first int) (int, error) {
	is := &g.info
	title, err := g.textMessage(billTextLBX, 9, 0x28)
	if err != nil {
		return 0, err
	}
	t := g.e.Text()
	if err := t.PrintCentered(0x1a0, 0x1f, title); err != nil {
		return 0, err
	}
	t.SetStyle(4, g.exeColors(infoNormalColors))
	switch {
	case is.refType == 0:
		g.infoHelp(referenceHelp)
		return g.referenceMainSubscreen(first)
	case is.refType < 2:
		g.infoHelp(refCategoryHelp)
		return g.referenceCategorySubscreen(first)
	case is.refType == 2:
		g.infoHelp(refHowToHelp)
		return g.referenceHowToSubscreen(first)
	}
	return 0, nil
}

// referenceStyle is the style the reference lists use: 3 for German and
// French, else 4.
func (g *Game) referenceStyle() int {
	if l := g.settings[0xd2]; l == 1 || l == 2 {
		return 3
	}
	return 4
}

// referenceMainSubscreen is Reference_Main_Subscreen_ (0x1093cd): the
// categories (help labels not starting with a digit) on the left, the
// "how to" topics on the right; picking one returns 0 with the next page
// set in _g_reference_screen_type.
func (g *Game) referenceMainSubscreen(first int) (int, error) {
	e, is := g.e, &g.info
	g.drawDropAreas(infoReferenceDrops, infoReferenceDrops+4*dropAreaSize)
	t := e.Text()
	for k, x := range [2]int{0xe0, 0x1a8} {
		s, err := g.textMessage(billTextLBX, 0xa+k, 0x28)
		if err != nil {
			return 0, err
		}
		if err := t.Print(x, 0x43, s); err != nil {
			return 0, err
		}
	}
	t.SetStyle(g.referenceStyle(), g.exeColors(infoNormalColors))
	fmtStr := cFormat(cStringAt(g.exe, referenceItemFmt))
	firstCat := e.FieldsCount()
	var cats, hows []string
	var catIDs []int16
	var howRecs []int32
	y := 100
	for i := 1; i < 0x11; i++ {
		label, err := g.eStringAt(helpCategoryLabels + 4*uint32(i))
		if err != nil {
			return 0, err
		}
		if label == "" || label[0] <= '9' {
			continue
		}
		s := fmt.Sprintf(fmtStr, label)
		cats = append(cats, s)
		if err := t.Print(0xdd, y, s); err != nil {
			return 0, err
		}
		if _, err := e.AddHiddenField(0xdd, y-2, 0x19b, y+0x12, "", 0); err != nil {
			return 0, err
		}
		catIDs = append(catIDs, int16(i))
		y += 0x14
	}
	firstHow := e.FieldsCount()
	n, rec, err := g.helpTable(helpHowToEntry)
	if err != nil {
		return 0, err
	}
	y = 100
	for i := 0; i < int(uint16(n)); i++ {
		r := rec(i)
		howRecs = append(howRecs, int32(binary.LittleEndian.Uint32(r[0x50:])))
		s := fmt.Sprintf(fmtStr, save.CString(r[:0x50]))
		hows = append(hows, s)
		if err := t.Print(0x1a5, y, s); err != nil {
			return 0, err
		}
		if _, err := e.AddHiddenField(0x1a5, y-2, 0x263, y+0x12, "", 0); err != nil {
			return 0, err
		}
		y += 0x14
	}
	e.V.CopyOffToBack()
	draw := func() error {
		f, err := e.ScanInput()
		if err != nil {
			return err
		}
		f = int(int16(f))
		switch {
		case f >= firstHow:
			k := int(int16(f - firstHow))
			if err := t.Print(0x1a5, int(int16(k*0x14+100)), hows[k]); err != nil {
				return err
			}
		case f >= firstCat:
			k := int(int16(f - firstCat))
			if err := t.Print(0xdd, int(int16(k*0x14+100)), cats[k]); err != nil {
				return err
			}
		}
		e.CyclePaletteColor(0xbf, 8, 0x20, 0, 0x28, 0x3f, 8, 8)
		return nil
	}
	e.AssignAutoFunction(draw, 2)
	if err := g.introduceSubscreen(); err != nil {
		return 0, err
	}
	is.showing = infoReference
	t.SetStyle(g.referenceStyle(), g.exeColors(infoHighColors))
	for {
		in, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		if g.fullDrawScreen {
			return 0, nil
		}
		r := int(int16(in))
		if r >= first {
			if r >= firstHow {
				is.refIx, is.refType = int16(howRecs[r-firstHow]), 2
			} else {
				is.refIx, is.refType = catIDs[r-firstCat], 1
			}
			return 0, nil
		}
		if r == 0 {
			e.MarkTime()
			if err := draw(); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
			if err := e.ReleaseTime(2); err != nil {
				return 0, err
			}
			continue
		}
		return r, nil
	}
}

// referenceHowToSubscreen is Reference_How_To_Subscreen_ (0x109762): a
// topic's title and text with a button back to the main page.
func (g *Game) referenceHowToSubscreen(first int) (int, error) {
	e, is := g.e, &g.info
	g.drawDropAreas(infoHowToDrops, infoCategoryDrops)
	_, rec, err := g.helpTable(0)
	if err != nil {
		return 0, err
	}
	r := rec(int(is.refIx))
	title, err := g.textMessage(billTextLBX, 0xd, 0x50)
	if err != nil {
		return 0, err
	}
	t := e.Text()
	if err := t.PrintCentered(0x19e, 0x43, title+save.CString(r)); err != nil {
		return 0, err
	}
	if l := g.settings[0xd2]; l == 1 || l == 2 {
		t.SetStyle(3, g.exeColors(infoNormalColors))
	}
	if err := t.PrintFormatted(0xe0, 0x63, 0x17d, save.CString(r[0x67:]), 3); err != nil {
		return 0, err
	}
	e.V.CopyOffToBack()
	if _, err := e.AddButtonField(0x182, 0x1ab, "", is.pics[0x15], "", 0x28); err != nil {
		return 0, err
	}
	e.AssignAutoFunction(func() error { return nil }, 2) // Draw_Dummy_Turn_Summary_Subscreen_
	is.refType = 0
	if err := g.introduceSubscreen(); err != nil {
		return 0, err
	}
	is.showing = infoReference
	for {
		in, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		if r := int(int16(in)); r != 0 || g.fullDrawScreen {
			return r, nil
		}
		if err := e.TogglePages(); err != nil {
			return 0, err
		}
	}
}

// referenceCategorySubscreen is Reference_Category_Subscreen_ (0x10988e): a
// category's topics, sorted by name, as a paged list; the hovered topic's
// title and text beside it.
func (g *Game) referenceCategorySubscreen(first int) (int, error) {
	e, is, ls := g.e, &g.info, &g.info.list
	g.drawDropArea(infoCategoryDrops)
	title, err := g.textMessage(billTextLBX, 0xc, 0x50)
	if err != nil {
		return 0, err
	}
	label, err := g.eStringAt(helpCategoryLabels + 4*uint32(is.refIx))
	if err != nil {
		return 0, err
	}
	t := e.Text()
	if err := t.PrintCentered(0x1a0, 0x43, title+label); err != nil {
		return 0, err
	}
	bm := gfx.NewBlankBitmap(0xbe, 0x13a)
	count, rec, err := g.helpTable(int(is.refIx))
	if err != nil {
		return 0, err
	}
	h := &is.catHead
	*h = g.readListHeader(infoCatHeader, 2)
	n := int(uint8(count))
	h.entries = make([]listEntry, max(n, 1))
	recs := make([]int32, len(h.entries))
	for k := 0; ; {
		r := rec(k)
		en := &h.entries[k]
		en.text = save.CString(r[:0x50])
		en.state, en.n, en.id = 0, 1, uint8(k)
		recs[k] = int32(binary.LittleEndian.Uint32(r[0x50:]))
		k++
		if k >= int(int16(count)) {
			break
		}
	}
	h.entries = h.entries[:n]
	// The bubble sort swaps the names and their records, not the ids.
	for i := 0; i < len(h.entries); i++ {
		for j := 0; j < len(h.entries)-1; j++ {
			if h.entries[j].text > h.entries[j+1].text {
				h.entries[j].text, h.entries[j+1].text = h.entries[j+1].text, h.entries[j].text
				recs[j], recs[j+1] = recs[j+1], recs[j]
			}
		}
	}
	h.initListData()
	ls.header = h
	backF, err := e.AddButtonField(0x182, 0x1ab, "", is.pics[0x15], "", 0x28)
	if err != nil {
		return 0, err
	}
	upF, err := e.AddButtonField(0x18e, 0x60, "", is.pics[0xe], "\x1b", 0x28)
	if err != nil {
		return 0, err
	}
	downF, err := e.AddButtonField(0x18e, 0x18c, "", is.pics[0xf], "\x1b", 0x28)
	if err != nil {
		return 0, err
	}
	draw := func() error { return g.drawReferenceCategorySubscreen(bm) }
	e.AssignAutoFunction(draw, 2)
	is.refType = 0
	for {
		g.drawDropAreas(infoCatDrops, infoCatDrops+2*dropAreaSize)
		if err := e.Draw(0x18e, 0x60, is.pics[0xc]); err != nil {
			return 0, err
		}
		if err := e.Draw(0x18e, 0x18c, is.pics[0xd]); err != nil {
			return 0, err
		}
		if err := g.displayListPage(h); err != nil {
			return 0, err
		}
		e.V.CopyOffToBack()
		e.ClearFieldsAbove(downF)
		ls.firstField = e.FieldsCount()
		if ls.lastField, err = g.addFieldsToListPage(h); err != nil {
			return 0, err
		}
		g.setListUpDownFieldDrawing(h, upF, downF)
		f, err := e.ScanInput()
		if err != nil {
			return 0, err
		}
		if ls.prior = int(int16(f)); ls.prior < ls.firstField || ls.prior > ls.lastField {
			ls.prior = ls.firstField
		}
		h.setSelectedListEntry(ls.prior)
		ls.sel = h.selectedListEntry()
		ls.updateNeeded = true
		if err := g.introduceSubscreen(); err != nil {
			return 0, err
		}
		is.showing = infoReference
	input:
		for {
			e.MarkTime()
			if ls.updateNeeded && ls.sel != nil {
				*bm = *gfx.NewBlankBitmap(0xbe, 0x13a)
				_, rec0, err := g.helpTable(0)
				if err != nil {
					return 0, err
				}
				r := rec0(int(int16(recs[ls.sel.id])))
				t.SetStyle(4, g.exeColors(infoNormalColors))
				if err := g.print1OrMoreLinesCenteredToBitmap(0, 0x18, 0xba, save.CString(r[:0x50]), bm); err != nil {
					return 0, err
				}
				t.SetStyle(3, g.exeColors(infoNormalColors))
				if err := t.PrintFormattedToBitmap(0, 0x40, 0xba, save.CString(r[0x67:]), 3, bitmapTarget(bm)); err != nil {
					return 0, err
				}
			}
			in, err := e.GetInput()
			if err != nil {
				return 0, err
			}
			if g.fullDrawScreen {
				return 0, nil
			}
			r := int(int16(in))
			if r == downF && h.nextListPage() || r == upF && h.previousListPage() {
				break input
			}
			if r == 0 {
				if err := draw(); err != nil {
					return 0, err
				}
				if err := e.TogglePages(); err != nil {
					return 0, err
				}
				if err := e.ReleaseTime(2); err != nil {
					return 0, err
				}
				continue
			}
			if r <= backF {
				return r, nil
			}
		}
	}
}

// drawReferenceCategorySubscreen is Draw_Reference_Category_Subscreen_
// (0x109d25), the auto function.
func (g *Game) drawReferenceCategorySubscreen(bm *gfx.Picture) error {
	e, ls := g.e, &g.info.list
	h := ls.header
	f, err := e.ScanInput()
	if err != nil {
		return err
	}
	f = int(int16(f))
	if e.MouseButton() == 0 && !ls.updateNeeded && f >= ls.firstField && f <= ls.lastField && f != ls.prior {
		h.setSelectedListEntry(f)
		ls.sel = h.selectedListEntry()
		ls.updateNeeded = true
		ls.prior = f
	} else {
		ls.updateNeeded = false
		if err := e.Draw(0x1a6, 0x60, bm); err != nil {
			return err
		}
	}
	if ls.sel != nil {
		if err := g.displaySelectedListText(h, ls.sel); err != nil {
			return err
		}
	}
	e.CyclePaletteColor(0xbf, 8, 0x20, 0, 0x28, 0x3f, 8, 8)
	return nil
}

// print1OrMoreLinesCenteredToBitmap is Print_1_Or_More_Lines_Centered_To_
// Bitmap_ (0x1032d8): the paragraph centred on x+maxw/2 with its middle
// on y (not above 0).
func (g *Game) print1OrMoreLinesCenteredToBitmap(x, y, maxw int, s string, bm *gfx.Picture) error {
	t := g.e.Text()
	h0 := int16(t.Height())
	h, err := t.ParagraphMaxHeight(maxw, s)
	if err != nil {
		return err
	}
	yy := int16(y) - (int16(h)-h0)/2
	if yy < 0 {
		yy = 0
	}
	return t.PrintParagraphToBitmap(x, int(yy), maxw, s, engine.ParaCentered, bitmapTarget(bm))
}
