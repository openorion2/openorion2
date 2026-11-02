package game

import (
	"github.com/openorion2/openorion2/internal/save"
)

// The home star's naming on the first turn (MOX\HAROLD\NAMESTAR.C and
// Change_Home_Star_Name_). Spec: docs/newgame/generation.md, "Turn 1".

const (
	starNameWidthProbe = 0x179ac3 // "WWWWWWW"
	homeStarNameTitle  = 0x101    // H_Message_: "Enter Home Star Name"
	starNameTitle      = 0x100    // H_Message_ of Change_Star_Name_Popup_
	// Star_Name_Popup_Screen_Center_X_ (0x923be) and _Y_ (0x923c4).
	starNamePopupX = 0xb1
	starNamePopupY = 0x7d
)

// changeStarNamePopup is Change_Star_Name_Popup_ (0x92281): naming a star
// on its first colony.
func (g *Game) changeStarNamePopup(x, y int16, star int, remap bool) (int16, error) {
	return g.doChangeStarName(x, y, star, g.hMessage(starNameTitle), remap)
}

// changeHomeStarName is Change_Home_Star_Name_ (0x8249c): the input box
// over the map, then the main screen's fields and picture again.
func (g *Game) changeHomeStarName() error {
	s := &g.state
	e := g.e
	star := int(s.Planets[s.Players[s.PlayerNum].HomePlanet].Star)
	if _, err := g.doChangeStarName(0x83, 0x92, star, g.hMessage(homeStarNameTitle), false); err != nil {
		return err
	}
	e.ClearFields()
	g.setMainScreenFieldsToNull()
	if err := g.addCheats(); err != nil {
		return err
	}
	if err := g.addMapFields(); err != nil {
		return err
	}
	e.V.SetPageOff()
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawMainScreen, 1)
	return nil
}

// doChangeStarName is Do_Change_Star_Name_ (0x922c2) with the 1.40 patch's
// hook around its Input_Box_Popup_ (0x16920d: star 0's name bytes 0xd and
// 0xe hold the patch's switches and survive a rename of star 0). An empty
// name keeps the old one.
func (g *Game) doChangeStarName(x, y int16, star int, title string, remap bool) (int16, error) {
	s := &g.state
	t := g.e.Text()
	t.SetStyle(3, t.Current)
	st := &s.Stars[star]
	w := int16(t.Width([]byte(save.CString(st.Name[:]))))
	probe := int16(t.Width([]byte(cStringAt(g.exe, starNameWidthProbe))))
	width := g.maxPixelWidthStarNameCanBe(star)
	if width < w {
		width = w
	}
	if width > probe {
		width = probe
	}
	if width > 0xcd {
		width = 0xcd
	}
	old := save.CString(st.Name[:])
	flags := s.Stars[0].Name[0xe]
	name := []byte(old)
	ok, err := g.inputBoxPopup(&name, 0xf, width, title, x, y, remap)
	if err != nil {
		return 0, err
	}
	strcpyField(st.Name[:], string(name))
	s.Stars[0].Name[0xe] = flags
	s.Stars[0].Name[0xd] = 0
	if st.Name[0] == 0 {
		strcpyField(st.Name[:], old)
	}
	return ok, nil
}

// starNameInfo is Get_Star_Name_Info_ (0x8a3e7): at the widest zoom, the
// name's left x and y on the map and its width.
func (g *Game) starNameInfo(i int) (x, y, w int16) {
	s := &g.state
	saved := s.CurMapScale
	s.CurMapScale = s.MaxMapScale
	h := starFieldsDim[g.mapScaleToZoomLevel()+int(s.Stars[i].Size)]
	g.setPlayerFontStyle(0, false)
	g.e.Text().SetSpacing(1)
	name, _, _ := g.starName(i, false)
	sx, sy := g.starDrawCoords(i)
	w = int16(g.e.Text().Width([]byte(name)))
	x = sx + 1 - w/2
	y = h/2 + sy - int16(s.Stars[i].Size)
	s.CurMapScale = saved
	return x, y, w
}

// maxPixelWidthStarNameCanBe is Max_Pixel_Width_Star_Name_Can_Be_
// (0x92457): twice the room to the nearest name on the same line (or the
// map's edge), plus the name's width.
func (g *Game) maxPixelWidthStarNameCanBe(star int) int16 {
	s := &g.state
	x0, y0, w0 := g.starNameInfo(star)
	fh := int16(g.e.Text().Height()) + 2
	minLeft, minRight := int16(0x3e7), int16(0x3e7)
	left, right := int16(-1), int16(-1)
	leftW := int16(0)
	abs16 := func(v int16) int16 {
		if v < 0 {
			return -v
		}
		return v
	}
	for si := 0; si < int(s.NumStars); si++ {
		if si != star {
			x, y, w := g.starNameInfo(si)
			if abs16(y-y0) < fh {
				d := x - x0
				switch {
				case d < 0 && abs16(d) < minLeft:
					minLeft, left, leftW = abs16(d), int16(si), w
				case d >= 0 && abs16(d) < minRight:
					right, minRight = int16(si), abs16(d)
				}
			}
		}
		if left == -1 {
			minLeft, leftW = abs16(x0-0x15), 0
		}
		if right == -1 {
			minRight = abs16(x0 - 0x20f)
			if minRight < w0 {
				minRight = w0
			}
		}
	}
	if minRight < w0 {
		minRight = w0
	}
	r := minRight - w0
	l := minLeft - leftW
	if l > r {
		l = r
	}
	v := l*2 + w0
	if v < w0 {
		v = w0 + 5
	}
	return v
}
