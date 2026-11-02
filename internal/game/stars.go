package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// Stars on the galaxy map. Spec: docs/screens/main-screen-stars.md.

// starFieldsDim is _star_fields_dim, the star picture size by size+zoom.
var starFieldsDim = [6]int16{0x21, 0x1d, 0x19, 0x17, 0x15, 0x11}

// blackHoleDims is the black hole draw size by zoom level (0x81c68).
var blackHoleDims = [4]int16{39, 33, 33, 24}

// mainPalettePlayerColors is _main_palette_player_colors, by banner.
var mainPalettePlayerColors = [8]byte{0x49, 0x62, 0x6e, 0x20, 0x3e, 0x94, 0x2d, 0x55}

// surrenderColors underline a star that surrendered, by banner (0x81c88).
var surrenderColors = [8]byte{0x80, 0x7b, 0x78, 0x2c, 0x86, 0x9f, 0x83, 0x7e}

// relocationColors is the colour cycle of relocation lines (0x81c80).
var relocationColors = [8]byte{0x6e, 0x6f, 0x70, 0x70, 0x6e, 0x6f, 0x70, 0x70}

// scaled is Get_Scaled_Value_: map units to pixels.
func (g *Game) scaled(v int16) int { return int(v) * 10 / int(g.state.CurMapScale) }

// upScaled is Get_Up_Scaled_Value_: pixels to map units.
func (g *Game) upScaled(v int16) int { return int(v) * int(g.state.CurMapScale) / 10 }

// mapScaleToZoomLevel is Map_Scale_To_Zoom_Level_: 0 is zoomed in.
func (g *Game) mapScaleToZoomLevel() int {
	s := &g.state
	mz := s.MaxZoomCount
	switch s.CurMapScale {
	case 10:
		if mz >= 0 {
			return 0
		}
	case 15:
		if mz > 0 {
			return 1
		}
	case 20:
		if mz > 1 {
			return 2
		}
	default:
		if mz > 2 {
			return 3
		}
	}
	return int(mz)
}

// starDrawCoords is Get_Star_Draw_Coords_.
func (g *Game) starDrawCoords(i int) (int16, int16) {
	s := &g.state
	st := &s.Stars[i]
	return int16(g.scaled(st.X-s.CurMapX) + 0x15), int16(g.scaled(st.Y-s.CurMapY) + 0x15)
}

// starOnScreen is Star_On_Screen_.
func (g *Game) starOnScreen(i int) bool {
	x, y := g.starDrawCoords(i)
	return x > 0x15 && x < 0x20f && y > 0x15 && y < 0x1a5
}

// starPicture is Get_Star_Picture_Seg_: the picture argument is unused.
func (g *Game) starPicture(class, size uint8) (*gfxPicture, error) {
	i := int(class)*6 + g.mapScaleToZoomLevel()
	if class != save.SpectralBlackHole {
		i += int(size)
	}
	return g.cached(buffer0LBX, 0x94+i)
}

// drawStars is Draw_Stars_.
func (g *Game) drawStars() error {
	s := &g.state
	g.ms.nTwinklingStars = 0
	for i := 0; i < int(s.NumStars); i++ {
		if s.Stars[i].SpectralClass != save.SpectralBlackHole && g.starOnScreen(i) {
			if err := g.drawAStar(i); err != nil {
				return err
			}
		}
	}
	return nil
}

// drawAStar is Draw_A_Star_. Nothing in this build starts a twinkle, so
// stars always show frame 0; the twinkle branch is ported anyway.
func (g *Game) drawAStar(i int) error {
	m, st := &g.ms, &g.state.Stars[i]
	dim := starFieldsDim[g.mapScaleToZoomLevel()+int(st.Size)]
	pic, err := g.starPicture(st.SpectralClass, st.Size)
	if err != nil {
		return err
	}
	x, y := g.starDrawCoords(i)
	x -= dim / 2
	y -= dim / 2
	if st.TwinkleFrame < 0 {
		pic.CurrentFrame = 0
	} else {
		st.TwinkleFrame++
		tf := st.TwinkleFrame
		if st.TwinkleEnd <= tf {
			m.nTwinklingStars = max(m.nTwinklingStars-1, 0)
			st.TwinkleFrame, st.TwinkleEnd = -1, -1
		}
		setAnimationFrame(pic, int(int16((int(tf)/2)%pic.Frames)))
	}
	return g.e.Draw(int(x), int(y), pic)
}

// drawBlackHoles is Draw_Black_Holes_.
func (g *Game) drawBlackHoles() error {
	m, s := &g.ms, &g.state
	z := g.mapScaleToZoomLevel()
	k := 0
	ws := g.saveWindowStatus()
	g.mapWindow()
	for i := 0; i < int(s.NumStars); i++ {
		st := &s.Stars[i]
		if st.SpectralClass != save.SpectralBlackHole {
			continue
		}
		x, y := g.starDrawCoords(i)
		x -= blackHoleDims[z] / 2
		y -= blackHoleDims[z] / 2
		pic, err := g.starPicture(st.SpectralClass, st.Size)
		if err != nil {
			return err
		}
		c := byte((int(m.blackHoleAnim[k]) + 1) % (2 * pic.Frames))
		m.blackHoleAnim[k] = c
		setAnimationFrame(pic, int(c)/2)
		k++
		if err := g.e.Draw(int(x), int(y), pic); err != nil {
			return err
		}
		if k > 10 {
			k = 10
		}
	}
	g.restoreWindowStatus(ws)
	return nil
}

// setAnimationFrame is Set_Animation_Frame: frames past the end clamp to
// the last one.
func setAnimationFrame(p *gfxPicture, f int) {
	if f >= p.Frames {
		f = p.Frames - 1
	}
	p.CurrentFrame = f
}

// playerHasVisited is Player_Has_Visited_.
func (s *State) playerHasVisited(p, star int) bool { return s.Stars[star].Visited>>p&1 != 0 }

// playerIsOmniscient is Player_Is_Omniscient_.
func (s *State) playerIsOmniscient(p int) bool {
	pl := &s.Players[p]
	return pl.GalacticLore != 0 || pl.Picks[save.PickOmniscient] != 0
}

// contactWithPlayer is Contact_With_Player_.
func (s *State) contactWithPlayer(p, other int) bool {
	if other >= 0 && other < int(s.NumPlayers) {
		return s.Players[p].Contact[other] != 0
	}
	return false
}

// starColonies calls fn for each colony at the star, orbit 0..4.
func (s *State) starColonies(star int, fn func(c *save.Colony)) {
	for orbit := 0; orbit < 5; orbit++ {
		pl := int(s.Stars[star].Planets[orbit])
		if pl < 0 {
			continue
		}
		c := int(s.Planets[pl].Colony)
		if c < 0 {
			continue
		}
		fn(&s.Colonies[c])
	}
}

// contactWithOneColony is Contact_With_One_Colony_.
func (s *State) contactWithOneColony(star int) bool {
	found := false
	s.starColonies(star, func(c *save.Colony) {
		if !found && s.contactWithPlayer(int(s.PlayerNum), int(c.Owner)) {
			found = true
		}
	})
	return found
}

// starIsOutpostStar is Star_Is_Outpost_Star_: colonies there and all of
// them outposts.
func (s *State) starIsOutpostStar(star int) bool {
	var outposts, colonies int
	s.starColonies(star, func(c *save.Colony) {
		if c.IsOutpost == 0 {
			colonies++
		} else {
			outposts++
		}
	})
	return byte(outposts) != 0 && byte(colonies) == 0
}

// nColonyOwners is N_Colony_Owners_. An owner of -1 indexes the byte before
// the local array in the original; it never occurs for a live colony.
func (s *State) nColonyOwners(star int) int {
	var seen [8]bool
	n := 0
	s.starColonies(star, func(c *save.Colony) {
		o := int(c.Owner)
		if o >= 0 && o < 8 && !seen[o] {
			n++
			seen[o] = true
		}
	})
	return n
}

// zoomLevelFontStyle is Zoom_Level_Font_Style_; other scales return the
// caller's EAX, which no scale the game uses reaches.
func (g *Game) zoomLevelFontStyle() int {
	switch g.state.CurMapScale {
	case 10:
		return 3
	case 15, 20:
		return 2
	case 30:
		return 1
	}
	return 1
}

// fontColors2 is Font_Colors2_ (0x8e5c5); it clears
// _mox_font_colors_offset.
func (g *Game) fontColors2(mode int, a, b byte) font.Colors {
	var c font.Colors
	off := g.moxFontColorsOffset
	g.moxFontColorsOffset = 0
	m := uint16(mode)
	fill := func() {
		c[0] = a
		for i := 1; i < 8; i++ {
			c[i] = b
		}
	}
	switch {
	case m < 2:
		fill()
	case m < 4:
		fill()
		if off != 0 {
			c[2] = off
		} else if int(a)+1 < int(b) {
			c[2] = a + 1
		} else {
			c[2] = b
		}
	case m == 4:
		fill()
		if off != 0 {
			c[2] = off
		} else {
			c[2] = b - 1
		}
	case m == 5:
		c[1] = a
		c[2], c[3], c[4] = b-1, b-1, b-1
		c[5], c[6], c[7] = b, b, b
	}
	return c
}

// playerFontColors is Get_Player_Font_Colors_.
func (g *Game) playerFontColors(p, style int, hl bool) font.Colors {
	s := &g.state
	off := 0
	if hl {
		off = int(g.ms.selectedStarColorOffset)
	}
	a, b, kind := 1, 0x0e, 0
	if p < 8 || p > 14 {
		pl := &s.Players[p&7] // p < 0 or p > 14 read outside the array in the original
		if pl.Color == 3 {
			a, b, kind = 0x0c, 0x16, 2
		} else if p < 8 {
			mc := int(mainPalettePlayerColors[pl.Color])
			a, b = mc+1, mc+6
			if pl.Color == 4 {
				b = mc + 5
			}
		}
	} else {
		a, b, kind = 4, 0x0b, 1
	}
	c := g.fontColors2(style, byte(a), byte(b))
	for i := 1; i < 8; i++ {
		switch kind {
		case 1:
			c[i] += byte(off)
		case 2:
			c[i] += byte(off * 7 / 4)
		default:
			c[i] += byte(off * 3 / 2)
		}
		if int(c[i]) > b+8 {
			c[i] = byte(b + 8)
		}
	}
	return c
}

// setPlayerFontStyle is Set_Player_Font_Style_.
func (g *Game) setPlayerFontStyle(p int, hl bool) {
	style := g.zoomLevelFontStyle()
	g.e.Text().SetStyle(style, g.playerFontColors(p, style, hl))
}

// decreaseFontColorRange is Decrease_Font_Color_Range_.
func (g *Game) decreaseFontColorRange(n int) {
	t := g.e.Text()
	c := t.Current
	for i := 1; i < 8; i++ {
		if int(c[i])-n < 1 {
			c[i] = 0
		} else {
			c[i] -= byte(n)
		}
	}
	t.Normal, t.Current = c, c
}

// starNameColors brightens shades 1..7 for a highlighted unowned name.
func brightenStarName(c *font.Colors, off int) {
	if off <= 0 {
		return
	}
	for i := 1; i < 8; i++ {
		c[i] += byte(off * 9 / 5)
		if c[i] < c[0] {
			c[i] = c[0]
		}
		if c[i] > 0x10 {
			c[i] = 0x10
		}
	}
}

// starName is Get_Star_Name_ (0x85c8a): the name to print, with the font
// set up for it; ok is false when the star is not named at all. bm is the
// name split by colony owners when Print_Star_Name_To_Bitmap_ made one.
func (g *Game) starName(i int, hl bool) (name string, bm *gfx.Picture, ok bool) {
	s := &g.state
	t := g.e.Text()
	omni := s.playerIsOmniscient(int(s.PlayerNum))
	off := 0
	if hl {
		off = int(g.ms.selectedStarColorOffset)
	}
	outpost := s.starIsOutpostStar(i)
	o := int(s.Stars[i].Owner)
	star := save.CString(s.Stars[i].Name[:])
	if !omni && (o < 0 || o > 7 || !s.contactWithOneColony(i)) {
		if !s.playerHasVisited(int(s.PlayerNum), i) {
			return "", nil, false
		}
		st := g.zoomLevelFontStyle()
		c := g.fontColors2(st, 1, 0x0c)
		brightenStarName(&c, off)
		t.SetStyle(st, c)
		name = star
		if outpost {
			g.decreaseFontColorRange(2)
			name = stringToLower(name)
		}
		return name, nil, true
	}
	name = star
	if omni && o != int(s.PlayerNum) && !s.playerHasVisited(int(s.PlayerNum), i) {
		name = fmt.Sprintf("(%s)", star)
	}
	if o < 0 || o > 7 {
		st := g.zoomLevelFontStyle()
		c := g.fontColors2(st, 1, 0x0c)
		brightenStarName(&c, off)
		t.SetStyle(st, c)
	} else {
		g.setPlayerFontStyle(o, hl)
	}
	if outpost {
		g.decreaseFontColorRange(1)
		name = stringToLower(name)
	}
	if s.nColonyOwners(i) > 1 {
		bm = g.printStarNameToBitmap(i, name)
	}
	return name, bm, true
}

// printStarNameToBitmap is Print_Star_Name_To_Bitmap_ (0x88b5b): the name
// printed into a 200x20 bitmap, each owner's share recoloured in its colours.
// It is nil when fewer than two owners are known.
func (g *Game) printStarNameToBitmap(i int, name string) *gfx.Picture {
	s, t := &g.state, g.e.Text()
	counts, n := s.colonyOwners(i, !s.playerIsOmniscient(int(s.PlayerNum)))
	if n <= 1 {
		return nil
	}
	var total int16
	for _, c := range counts {
		total += c
	}
	cur := t.Current
	t.SetSpacing(1)
	bm := gfx.NewBlankBitmap(200, 20)
	// Print_Display_To_Bitmap only fails for a non-bitmap target.
	_ = t.ToBitmap(font.Target{Pix: bm.Pixels(), Stride: bm.Width, H: bm.Height}, 0, 0, []byte(name))
	var x int16
	for p := 0; p < 8; p++ {
		if counts[p] <= 0 {
			continue
		}
		w, h := g.bitmapStringRange(counts[p], total, name)
		nc := g.playerFontColors(p, t.Style, false)
		bm.SwapColors(int(x), 0, int(int16(w+x)), int(h), cur[:], nc[:])
		x += w
	}
	return bm
}

// bitmapStringRange is Get_Bitmap_String_Range_ (0x8f4ca): the width of n
// of total equal shares of the name, capped at its width, and the font
// height. The range's end (start + one share) has no reader.
func (g *Game) bitmapStringRange(n, total int16, name string) (w, h int16) {
	t := g.e.Text()
	sw := int16(t.Width([]byte(name)))
	per := int16((int(sw)*10/int(total) + 9) / 10)
	w = n * per
	if sw < w {
		w = sw
	}
	return w, int16(t.Height())
}

// starNameHighlight holds _star_name_highlight_value and _direction (.data
// 0 and 1, only touched by Print_Highlighted_Star_Name_).
type starNameHighlight struct{ value, direction int8 }

func (g *Game) starNameHighlight() *starNameHighlight {
	if g.nameHL == nil {
		g.nameHL = &starNameHighlight{0, 1}
	}
	return g.nameHL
}

// stringToLower is String_To_Lower: A..Z only.
func stringToLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 0x20
		}
	}
	return string(b)
}

// printStarNames is Print_Star_Names_.
func (g *Game) printStarNames() error {
	s := &g.state
	ws := g.saveWindowStatus()
	g.mapWindow()
	for i := 0; i < int(s.NumStars); i++ {
		if s.Stars[i].SpectralClass == save.SpectralBlackHole {
			continue
		}
		name, bm, ok := g.starName(i, false)
		if ok && name != "" {
			if err := g.printAStarName(i, bm, name); err != nil {
				return err
			}
		}
	}
	g.restoreWindowStatus(ws)
	return nil
}

// printHighlightedStarName is Print_Highlighted_Star_Name_.
func (g *Game) printHighlightedStarName(i int) error {
	ws := g.saveWindowStatus()
	g.mapWindow()
	name, bm, ok := g.starName(i, true)
	if ok && name != "" {
		if bm != nil {
			h := g.starNameHighlight()
			bm.TintColors(0, 0, 0, 0, true, byte(h.value))
			h.value += h.direction
			if h.value == 0 || h.value == 3 {
				h.direction = -h.direction
			}
		}
		if err := g.printAStarName(i, bm, name); err != nil {
			return err
		}
	}
	g.restoreWindowStatus(ws)
	return nil
}

// printAStarName is Print_A_Star_Name_ (0x87768); a name bitmap is drawn
// as it is, without the outline or the surrender mark.
func (g *Game) printAStarName(i int, bm *gfx.Picture, name string) error {
	s, t := &g.state, g.e.Text()
	st := &s.Stars[i]
	dim := starFieldsDim[g.mapScaleToZoomLevel()+int(st.Size)]
	cx, cy := g.starDrawCoords(i)
	y := int(int16(dim/2 + cy - int16(st.Size)))
	x := int(cx + 1)
	if bm != nil {
		return g.e.Draw(int(int16(x-t.Width([]byte(name))/2)), y, bm)
	}
	style := g.zoomLevelFontStyle()
	c := t.Current
	t.SetOutlineColor(2, 1)
	t.SetStyleShadow(style, c, font.Outline)
	t.SetSpacing(1)
	w := t.Width([]byte(name))
	left := x - w/2
	var err error
	switch {
	case s.CurMapScale == s.MaxMapScale && left < 0x16:
		err = t.Print(0x16, y, name)
	case s.CurMapScale == s.MaxMapScale && w+left > 0x20f:
		err = t.PrintRight(0x20f, y, name)
	default:
		err = t.PrintCentered(x, y, name)
	}
	if err != nil {
		return err
	}
	q := s.Stars[i].SurrenderTo[s.PlayerNum]
	if uint8(q) < 8 {
		g.setPlayerFontStyle(0, false)
		t.SetSpacing(1)
		w2 := t.Width([]byte(name))
		pc := s.Players[q].Color
		col := surrenderColors[pc&7]
		lx := int(cx) - w2/2
		ly := int(int16(t.Height() + int(cy+dim/2) - int(st.Size)))
		g.e.V.Line(lx, ly, lx+w2, ly, col)
		if s.CurMapScale < 30 {
			c2 := col + 1
			if pc == 3 || pc == 5 {
				c2 = col - 3
			}
			g.e.V.Line(lx, ly+1, lx+w2, ly+1, c2)
		}
	}
	return nil
}

// drawWormholeLinks is Draw_Wormhole_Links_.
func (g *Game) drawWormholeLinks() {
	s := &g.state
	ws := g.saveWindowStatus()
	g.mapWindow()
	p := int(s.PlayerNum)
	for i := 0; i < int(s.NumStars); i++ {
		to := int(s.Stars[i].WormholeTo)
		if to == -1 || !(s.playerHasVisited(p, i) || s.playerIsOmniscient(p)) {
			continue
		}
		x1, y1 := g.starDrawCoords(i)
		x2, y2 := g.starDrawCoords(to)
		g.e.V.Line(int(x1), int(y1), int(x2), int(y2), 4)
	}
	g.restoreWindowStatus(ws)
}

// drawRelocationLinks is Draw_Relocation_Links_.
func (g *Game) drawRelocationLinks() {
	s := &g.state
	ws := g.saveWindowStatus()
	g.mapWindow()
	if g.settings[0x08] != 0 {
		p := int(s.PlayerNum)
		for i := 0; i < int(s.NumStars); i++ {
			d := int(s.Stars[i].Relocation[p])
			if d == -1 {
				continue
			}
			x1, y1 := g.starDrawCoords(i)
			x2, y2 := g.starDrawCoords(d)
			g.e.V.DirectionalMultiColoredLine(int(x1), int(y1), int(x2), int(y2), relocationColors, int(g.ms.multiColoredLineStart))
		}
	}
	g.restoreWindowStatus(ws)
}

// drawAGateIcon is Draw_A_Gate_Icon_ (0x83741): the Dimensional Portal
// and Artemis Net icons left and right of a visited star.
func (g *Game) drawAGateIcon(i int) error {
	s := &g.state
	p := int(s.PlayerNum)
	if !s.playerHasVisited(p, i) {
		return nil
	}
	st := &s.Stars[i]
	dim := starFieldsDim[g.mapScaleToZoomLevel()+int(st.Size)]
	z := 0
	if g.mapScaleToZoomLevel() != 0 {
		z = 1
	}
	// Player_Has_Stargate_ and Player_Has_Jumpgate_ results are discarded.
	art := st.ArtemisNet>>p&1 != 0
	por := st.DimensionalPortal>>p&1 != 0
	var b [4]*gfx.Picture
	for k := range b {
		b[k] = gfx.NewBlankBitmap(18, 18)
	}
	n := 0
	var maxh int16
	for _, ic := range []struct {
		on    bool
		entry int
	}{{por, 0x15 + z}, {art, 0x21 + z}} {
		if !ic.on {
			continue
		}
		pic, err := g.cached(buffer0LBX, ic.entry)
		if err != nil {
			return err
		}
		b[n].DrawPictureToBitmap(pic)
		if h := int16(pic.Height); maxh < h {
			maxh = h
		}
		n++
	}
	if n == 0 {
		return nil
	}
	cx, cy := g.starDrawCoords(i)
	base := cy + dim/2 - maxh
	switch {
	case n == 1:
		_, _, w, _ := b[0].ActualSize()
		y := base
		if por {
			y += 2
		}
		return g.e.Draw(int(cx-int16(w)-4), int(y), b[0])
	case n == 2:
		_, _, w, _ := b[0].ActualSize()
		y := base
		if por {
			y += 2
		}
		if err := g.e.Draw(int(cx-int16(w)-4), int(y), b[0]); err != nil {
			return err
		}
		return g.e.Draw(int(cx+4), int(base), b[1])
	case n <= 4: // unreachable with two icon kinds
		all := gfx.NewBlankBitmap(0x4b, 0x14)
		var x int16
		for k := 0; k < n; k++ {
			if err := all.AddPicture(int(x), 0, b[k]); err != nil {
				return err
			}
			_, _, w, _ := b[k].ActualSize()
			x += int16(w) + 1
		}
		_, _, w, _ := all.ActualSize()
		return g.e.Draw(int(cx-int16(w)/2), int(base), all)
	}
	return nil
}
