package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The galaxy map box (moveable box 1) and the shared galaxy map routines of
// MOX module 56. Spec: docs/screens/galaxy-map-box.md. Box 1 never opens on
// the galaxy screen in this build; it is ported for fidelity.

// galaxyMapState holds the galaxy map globals.
type galaxyMapState struct {
	width, height int16 // _galaxy_map_width/_height
	gridX, gridY  int16 // _galaxy_map_grid_x/_y
	gridField     int   // _galaxy_map_grid_field
	starField     []int // _galaxy_map_star_field[NUM_STARS]

	starSeg [11]*gfx.Picture // _galaxy_map_star_seg, gstar.lbx 1..11

	// summaryStarSeg is _planet_summary_galaxy_star_seg (mode 2, PLNTSUM.LBX
	// 0xf..0x19); colsumStarSeg the colony summary's (mode 3, 0x190688).
	summaryStarSeg [11]*gfx.Picture
	colsumStarSeg  [11]*gfx.Picture
	starAnimCount  uint8 // _galaxy_star_anim_count (0x199f29)

	// boxPict is _moveable_box_pict[1]; nothing ever loads it.
	boxPict *gfx.Picture
	// colonyPalette is _using_colony_screen_palette (colony screen only).
	colonyPalette bool
}

// loadGalaxyMapPictures is the gstar.lbx part of Load_Main_Screen_Pictures_
// (0x135d5).
func (g *Game) loadGalaxyMapPictures() error {
	gm := &g.ms.galaxyMap
	for i := range gm.starSeg {
		p, err := g.picture("GSTAR.LBX", 1+i)
		if err != nil {
			return err
		}
		gm.starSeg[i] = p
	}
	return nil
}

// galaxyMapDefaults is the galaxy map part of Harold_Defaults_ (0x795aa).
func (g *Game) galaxyMapDefaults() {
	gm := &g.ms.galaxyMap
	gm.width, gm.height = 0xa0, 0x75
	gm.gridField = engine.NoField // as Set_Main_Screen_Fields_To_Null_ leaves it
}

// clearGalaxyMapFields is the box 1 part of Clear_Moveable_Box_Fields_
// (0x908ae).
func (g *Game) clearGalaxyMapFields() {
	gm := &g.ms.galaxyMap
	gm.gridField = engine.NoField
	n := int(g.state.NumStars)
	if cap(gm.starField) < n {
		gm.starField = make([]int, n)
	}
	gm.starField = gm.starField[:n]
	for i := range gm.starField {
		gm.starField[i] = engine.NoField
	}
}

// starFieldOf returns _galaxy_map_star_field[i], -1000 before any clear.
func (gm *galaxyMapState) starFieldOf(i int) int {
	if i < len(gm.starField) {
		return gm.starField[i]
	}
	return engine.NoField
}

// drawGalaxyMapBox1 is Draw_Moveable_Box_'s case 1 (0x910f6).
func (g *Game) drawGalaxyMapBox1() error {
	b := &g.state.MoveableBoxes[1]
	gm := &g.ms.galaxyMap
	return g.drawGalaxyMapBox(gm.boxPict, b.X, b.Y, gm.width, gm.height, 8, 0x26, b.DragDX, b.DragDY, 0, 0, b.X)
}

// drawGalaxyMapBox is Draw_Galaxy_Map_Box_ (0x90c4f). si is ESI on entry:
// the dot picture index used when a star picks none.
func (g *Game) drawGalaxyMapBox(pict *gfx.Picture, x, y, w, h, ox, oy, dx, dy, mode, flag, si int16) error {
	v, s, gm := g.e.V, &g.state, &g.ms.galaxyMap
	X := int32(x) + int32(dx)
	Y := int32(y) + int32(dy)
	sx := int32(1000*0x1fa) / int32(w)
	sy := int32(1000*400) / int32(h)
	if flag == 1 {
		return nil
	}
	if flag >= 0 && mode != 4 {
		b1 := &s.MoveableBoxes[1]
		bw, bh := int32(b1.Width), int32(b1.Height)
		if gm.colonyPalette {
			bw, bh = int32(w), int32(h)
		}
		if pict == nil || gm.colonyPalette {
			x1, y1 := int(int16(X)), int(int16(Y))
			x2, y2 := int(int16(bw+X)), int(int16(bh+Y))
			v.SetWindow(x1, y1, x2, y2)
			v.ClipOn()
			if g.currentScreen != 6 && !gm.colonyPalette {
				v.Fill(x1, y1, x2, y2, 0)
			}
			v.ClipOff()
			v.ResetWindow()
		} else if g.currentScreen == ScreenMain {
			t := g.e.Text()
			c := font.Colors{0x62, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x6b}
			t.SetSpacing(2)
			t.SetStyleShadow(3, c, font.ShadowDown)
			t.SetOutlineColor(0x62, 0x65)
			if err := t.PrintCentered(int(int16(X+0x57)), int(int16(Y+0xd)), g.hMessage(0xf3)); err != nil {
				return err
			}
			t.SetSpacing(t.Font.Styles[t.Style].HSpacing) // Set_Font_Default_Spacing_
		}
	}
	wx, wy := int32(int16(X))+int32(ox), int32(int16(Y))+int32(oy)
	v.SetWindow(int(int16(wx)), int(int16(wy)), int(int16(wx+int32(w))), int(int16(wy+int32(h))))
	v.ClipOn()
	scale := int32(s.MaxMapScale)
	for i := 0; i < int(s.NumStars); i++ {
		st := &s.Stars[i]
		lx := int32(st.X) * 1000 / scale
		ly := int32(st.Y) * 1000 / scale
		px := int32(ox) + int32(int16(X)) + lx*10/sx
		py := int32(int16(Y)) + int32(oy) + ly*10/sy
		o := int16(st.Owner)
		switch {
		case st.SpectralClass == save.SpectralBlackHole && gm.colonyPalette:
			si = 9
		case st.SpectralClass == save.SpectralBlackHole:
			si = 10
		case st.SpectralClass == 5:
			si = 8
		case o >= 0 && o < 8:
			si = int16(s.Players[o].Color)
		case s.playerHasVisited(int(s.PlayerNum), i) || o == -1 || o == -2:
			si = 8
		}
		if si < 0 || int(si) >= len(gm.starSeg) {
			g.logOnce("Draw_Galaxy_Map_Box_ star picture out of range")
			continue
		}
		var p *gfx.Picture
		d := int32(1)
		switch mode {
		case 0:
			p = gm.starSeg[si]
		case 1, 4:
			p, d = g.flt.galaxyStars[si], 2
			setAnimationFrame(p, 0)
		case 2:
			// The scanned star (a planet row's star, or a hovered one) blinks.
			p, d = gm.summaryStarSeg[si], 2
			if t := g.ms.scannedField; int16(i) == t || (t >= 1000 && int16(i) == t-1000) {
				setFrame(p, int(gm.starAnimCount))
				gm.starAnimCount = uint8((int(gm.starAnimCount) + 1) % p.Frames)
			} else {
				p.CurrentFrame = 0
			}
		case 3:
			p = gm.colsumStarSeg[si]
			if int16(i) != g.ms.galaxyMapScannedStar {
				p.CurrentFrame = 0
			}
		default:
			g.logOnce(fmt.Sprintf("Draw_Galaxy_Map_Box_ mode %d is not ported", mode))
			continue
		}
		if p == nil {
			continue
		}
		if err := g.e.Draw(int(int16(px-d)), int(int16(py-d)), p); err != nil {
			return err
		}
	}
	v.ClipOff()
	v.ResetWindow()
	switch {
	case g.currentScreen == 4 && g.ms.galaxyMapScannedStar > -1:
		g.doFleetScreenStuff()
	case g.currentScreen == 0x1d:
		return g.doOfficerScreenStuff(ox, oy, sx, sy)
	}
	return nil
}

// galaxyMapStarXY is Get_Galaxy_Map_Star_XY_ (0x911d5).
func (g *Game) galaxyMapStarXY(star int, x, y, w, h, ox, oy int16, big bool) (px, py int16) {
	s := &g.state
	st := &s.Stars[star]
	scale := int32(s.MaxMapScale)
	dw := int32(0x7b890)/int32(w) + 1
	dh := int32(400000)/int32(h) + 1
	half := int32(2)
	if big {
		half = 3
	}
	px = int16(int32(ox) + int32(x) + (int32(st.X)*1000/scale)*10/dw - half)
	py = int16((int32(st.Y)*1000/scale)*10/dh + int32(y) + int32(oy) - half)
	return
}

// scanGalaxyMapFields is Scan_Galaxy_Map_Fields_ (0x890ef): 0 a star was
// clicked (clk), 4 a star is hovered (hov), 2 the grid was clicked, else -1.
func (g *Game) scanGalaxyMapFields(scan, r int) (res, hov, clk int) {
	s, gm := &g.state, &g.ms.galaxyMap
	res, hov, clk = -1, -1, -1
	n := int(s.NumStars)
	for i := 0; i < n; i++ {
		if r == gm.starFieldOf(i) {
			res, clk = 0, i
			break
		}
	}
	if clk == -1 {
		for i := 0; i < n; i++ {
			if scan == gm.starFieldOf(i) && s.Stars[i].SpectralClass != save.SpectralBlackHole {
				res, hov = 4, i
				break
			}
		}
	}
	if res == -1 && r == gm.gridField {
		res = 2
	}
	return
}

// scanGalaxyMapBox is the box id 1 part of Scan_Moveable_Box_Fields_
// (0x89261..0x8933b): hover the star, centre the big map on a click.
func (g *Game) scanGalaxyMapBox(scan, r int) {
	s, gm := &g.state, &g.ms.galaxyMap
	res, hov, clk := g.scanGalaxyMapFields(scan, r)
	g.ms.galaxyMapScannedStar = int16(hov)
	if int16(g.mapScaleToZoomLevel()) == s.MaxZoomCount {
		return
	}
	switch res {
	case 2:
		cx := int32(int16(int32(0x13c4) / int32(gm.width)))
		cy := int32(int16(int32(0xfa0) / int32(gm.height)))
		ms := int32(s.MaxMapScale)
		mx := (int32(gm.gridX) * ms / 10) * cx / 10
		my := (int32(gm.gridY) * ms / 10) * cy / 10
		g.centerMap(int16(mx), int16(my))
	case 0:
		g.centerMap(s.Stars[clk].X, s.Stars[clk].Y)
	}
}

// printGalaxyMapScannedStar is Print_Galaxy_Map_Scanned_Star_ (0x879bc).
func (g *Game) printGalaxyMapScannedStar() error {
	s := &g.state
	t := g.e.Text()
	st := &s.Stars[g.ms.galaxyMapScannedStar]
	b1 := &s.MoveableBoxes[1]
	X := b1.X + b1.DragDX
	Y := b1.Y + b1.DragDY
	name := save.CString(st.Name[:])
	var buf string
	var c font.Colors
	switch o := int(st.Owner); {
	case o >= 0 && o < 8:
		// The original leaves c uninitialised; keep what Set_Player_Font_Style_ set.
		g.setPlayerFontStyle(o, false)
		c = t.Current
		buf = fmt.Sprintf("%s (%s)", name, g.raceName(o))
	case o == -1:
		c = font.Colors{8, 8, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc}
		buf = name
	case o == -2:
		c = font.Colors{0x14, 0x14, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f}
		buf = name + " (contested)"
	default:
		g.logOnce("Print_Galaxy_Map_Scanned_Star_ with an odd owner prints garbage")
		return nil
	}
	t.SetStyle(2, c)
	return t.Print(int(X+0xf), int(b1.Height+Y-0x19), buf)
}
