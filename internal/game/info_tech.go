package game

import (
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The tech review page of the info screen (INFO.C), also the right half of
// the race report. Spec: docs/screens/info-screen.md, "Tech review".

const (
	techReviewHeader = 0x183e15 // _tech_review_list_header
	techReviewLists  = 0x183b0e // _review_list, 4 pointers to (id, flags) pairs
	techRevField     = 0x183c31 // _tech_rev_field, x and y words
	techReviewChoice = 0x183e55 // the chosen list, a .data word
	appPicsLBX       = "APP_PICS.LBX"
	techReviewItems  = 17 // _tech_review_list_y: 16 items and the height
)

// techReviewSubscreen is Tech_Review_Subscreen_ (0x108057).
func (g *Game) techReviewSubscreen(first int) (int, error) {
	g.raceReportJimHack = true
	return g.techReviewPage(g.info.pics[0xc:0x14], first, int(g.state.PlayerNum))
}

// techReviewPage is _Tech_Review_Subscreen_ (0x108073): the known
// applications of player p in four lists (picked by four buttons), the
// hovered one's picture, name and description. pics are INFO.LBX 0xc..0x13.
func (g *Game) techReviewPage(pics []*gfx.Picture, first, p int) (int, error) {
	e, is := g.e, &g.info
	var heads [4]listHeader
	for i := range heads {
		heads[i] = g.readListHeader(techReviewHeader, techReviewItems)
		heads[i].print = g.printTechReviewItem
		if err := g.initTechReviewList(&heads[i], i, p); err != nil {
			return 0, err
		}
		if len(heads[i].entries) > 0 {
			heads[i].entries[0].selected = true
			heads[i].entries[0].item = 0
		}
	}
	bm1 := gfx.NewBlankBitmap(0xb4, 0x96)
	bm2 := gfx.NewBlankBitmap(0xb4, 0x30)
	var image *gfx.Picture
	blank, err := g.picture("app_pics.lbx", 0)
	if err != nil {
		return 0, err
	}
	ls := &is.list
	draw := func() error { return g.drawTechReviewSubscreen(&image, bm1, bm2) }
	e.AssignAutoFunction(draw, 2)
	lastMulti := 0
	for i := 0; i < 4; i++ {
		x, y := g.exeI16(techRevField+uint32(4*i)), g.exeI16(techRevField+uint32(4*i)+2)
		if lastMulti, err = e.AddMultiButtonField(int(x), int(y), "", pics[4+i], &is.techList, int16(i), "", 0x28); err != nil {
			return 0, err
		}
	}
	upF, err := e.AddButtonField(0x19a, 0x41, "", pics[2], "\x1b", 0x28)
	if err != nil {
		return 0, err
	}
	downF, err := e.AddButtonField(0x19a, 0x18b, "", pics[3], "\x1b", 0x28)
	if err != nil {
		return 0, err
	}
	title, err := g.textMessage(billTextLBX, 6, 0x28)
	if err != nil {
		return 0, err
	}
	if err := e.Text().PrintCentered(0x1a0, 0x1f, title); err != nil {
		return 0, err
	}
	g.drawDropAreas(infoTechDrops, infoTechDrops+4*dropAreaSize)
	e.V.Fill(0x1b1, 0x73, 0x264, 0xfd, 0)
	if err := e.Draw(0x1b1, 0x73, blank); err != nil {
		return 0, err
	}
	for {
		h := &heads[is.techList]
		ls.header = h
		reselect := false
	page:
		for {
			g.drawDropArea(infoTechDrops)
			if err := e.Draw(0x19a, 0x41, pics[0]); err != nil {
				return 0, err
			}
			if err := e.Draw(0x19a, 0x18b, pics[1]); err != nil {
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
			if reselect {
				h.setSelectedListEntry(ls.firstField)
			}
			g.setListUpDownFieldDrawing(h, upF, downF)
			ls.sel = h.selectedListEntry()
			ls.prior = 0
			ls.updateNeeded = true
			if err := g.introduceSubscreen(); err != nil {
				return 0, err
			}
			is.showing = infoTechReview
			for {
				e.MarkTime()
				if ls.updateNeeded && len(h.entries) > 0 {
					app := int(ls.sel.ids[ls.sel.item])
					if image, err = g.techReviewText(app, p, bm1, bm2); err != nil {
						return 0, err
					}
				}
				in, err := e.GetInput()
				if err != nil {
					return 0, err
				}
				r := int(int16(in))
				if g.fullDrawScreen {
					return r, nil
				}
				if r > lastMulti {
					if r == downF && h.nextListPage() || r == upF && h.previousListPage() {
						reselect = true
						continue page
					}
				} else if r == 0 {
					if err := draw(); err != nil {
						return 0, err
					}
					if err := e.TogglePages(); err != nil {
						return 0, err
					}
					if err := e.ReleaseTime(2); err != nil {
						return 0, err
					}
				}
				if r == 0 || r > lastMulti {
					continue
				}
				if r < first {
					if err := e.Draw(0x1b2, 0x108, bm1); err != nil {
						return 0, err
					}
					if err := e.Draw(0x1b2, 0x3c, bm2); err != nil {
						return 0, err
					}
					return r, e.TogglePages()
				}
				break page
			}
		}
	}
}

// techReviewText is the part of _Tech_Review_Subscreen_'s loop that
// prepares the chosen application: its description and name in the two
// bitmaps, its picture's palette in effect. Returns the picture.
func (g *Game) techReviewText(app, p int, bm1, bm2 *gfx.Picture) (*gfx.Picture, error) {
	e := g.e
	*bm1 = *gfx.NewBlankBitmap(0xb4, 0x96)
	*bm2 = *gfx.NewBlankBitmap(0xb4, 0x30)
	rec, err := g.helpRecord(app)
	if err != nil {
		return nil, err
	}
	t := e.Text()
	t.SetStyle(2, g.exeColors(infoNormalColors))
	if err := t.PrintFormattedToBitmap(2, 4, 0xad, save.CString(rec.text), 0, bitmapTarget(bm1)); err != nil {
		return nil, err
	}
	g.info.selApp = int16(app)
	t.SetStyle(4, g.exeColors(infoNormalColors))
	name, err := g.technologyApplicationsName(app, p)
	if err != nil {
		return nil, err
	}
	if err := t.PrintFormattedToBitmap(0, 0xc, 0xb0, name, 2, bitmapTarget(bm2)); err != nil {
		return nil, err
	}
	img, err := g.picture(appPicsLBX, app)
	if err != nil {
		return nil, err
	}
	e.DrawPalette(img)
	e.RefreshPalette()
	return img, nil
}

func bitmapTarget(bm *gfx.Picture) font.Target {
	return font.Target{Pix: bm.Pixels(), Stride: bm.Width, H: bm.Height}
}

// drawTechReviewSubscreen is Draw_Tech_Review_Subscreen_ (0x10852a), the
// auto function: the hovered item becomes the choice (no button held),
// otherwise the chosen one's picture and texts are drawn.
func (g *Game) drawTechReviewSubscreen(image **gfx.Picture, bm1, bm2 *gfx.Picture) error {
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
	} else if len(h.entries) > 0 {
		if *image != nil {
			if err := e.Draw(0x1b1, 0x73, *image); err != nil {
				return err
			}
		}
		ls.updateNeeded = false
		if err := e.Draw(0x1b2, 0x108, bm1); err != nil {
			return err
		}
		if err := e.Draw(0x1b2, 0x3c, bm2); err != nil {
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

// initTechReviewList is Init_Tech_Review_List_ (0x108611): list i's
// headings (BILLTEX2.LBX texts) with the applications player p knows.
// Items before the first heading start at slot i, as in the original.
func (g *Game) initTechReviewList(h *listHeader, i, p int) error {
	pl := &g.state.Players[p]
	src := g.exe.u32(techReviewLists + 4*uint32(i))
	h.entries = []listEntry{{}}
	en := &h.entries[0]
	k := i
	for ; ; src += 2 {
		id, flags := g.exeU8(src), g.exeU8(src+1)
		if flags&8 != 0 {
			break
		}
		if flags&4 == 0 {
			if pl.AppStatus[id] == techKnown {
				en.ids[k], en.states[k] = id, pl.AppStatus[id]
				k++
				en.n++
			}
			continue
		}
		if en.n > 0 {
			h.entries = append(h.entries, listEntry{})
			en = &h.entries[len(h.entries)-1]
		}
		text, err := g.textMessage("billtex2.lbx", int(id), 0x40)
		if err != nil {
			return err
		}
		en.text, en.state, en.n, en.id = text, 0, 0, id
		k = 0
	}
	if en.n == 0 {
		h.entries = h.entries[:len(h.entries)-1]
	}
	h.initListData()
	return nil
}

// printTechReviewItem is Print_Tech_Review_Item_ (0x108029), the list's
// item printer: the application's name (for the human, as in the
// original), formatted.
func (g *Game) printTechReviewItem(x, y int, app uint8) error {
	name, err := g.technologyApplicationsName(int(app), int(g.state.PlayerNum))
	if err != nil {
		return err
	}
	return g.e.Text().PrintFormatted(x, y, 0xc5, name, 0)
}
