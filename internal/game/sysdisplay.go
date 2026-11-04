package game

import (
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The system display, moveable box 0: a star's planets on their orbits.
// Spec: docs/screens/system-display.md.

const orbitConsts = 0xeb4c1 // _orbit_consts, int16[7][2] in the code segment

// g.sysL() and g.sysT() are the inner frame offsets the wrappers pass: 0xd, 0x2f,
// or 3, 3 for the leaders screen's display (sd.offscreen).
func (g *Game) sysL() int {
	if g.sd.offscreen {
		return 3
	}
	return 0xd
}

func (g *Game) sysT() int {
	if g.sd.offscreen {
		return 3
	}
	return 0x2f
}

// sysDispRec is one 14-byte _sys_disp record.
type sysDispRec struct {
	planet int16
	x, y   int16
	pos    uint8 // angular position 0..31
	slot   uint8 // index in star.planets
	frame  int16
	pict   *gfx.Picture
}

// sysDisplayShip is one _system_display_ships entry, per owner.
type sysDisplayShip struct {
	x, y, field, node int16
	pict              *gfx.Picture
}

// systemDisplay holds the box's globals.
type systemDisplay struct {
	loaded          string // _star_name_loaded_string; "" when cleared
	hasLoaded       bool
	disp            [5]sysDispRec
	starID          int16 // _system_star_id
	offscreen       bool  // the leaders screen draws it with the margins 3, 3
	autoSelectStar  int16 // _system_star_id_for_auto_select
	iconScanned     int16 // _system_icon_scanned
	shipScanned     int16 // _system_ship_scanned
	shipFrame       int16 // _ship_frame
	skipPlanets     bool  // _skip_draw_system_planets
	sunBeforePlanet bool  // _draw_sun_before_planet
	skipDisplay     bool  // _skip_draw_system_display
	playerHasShip   bool  // _player_has_ship_in_system
	lookAtColonies  bool  // _look_at_colonies
	withoutButton   bool  // _draw_system_display_without_button (0x199f21)
	description     string
	ships           [15]sysDisplayShip
	planetButtons   [5]int
	gateFields      [4]int
	gridField       int
	gridX, gridY    int
	baseX, baseY    int16
}

// rotPlanDim is _rot_plan_dim: planet hit size by size, 4 = gas giant.
var rotPlanDim = [5]int16{19, 20, 22, 22, 24}

// orbitRadius returns _orbit_consts[row]: the ellipse radii in tenths of a
// pixel.
func (g *Game) orbitRadius(row int) (int, int) {
	b := g.exe.bytes(uint32(orbitConsts+4*row), 4)
	return int(int16(binary.LittleEndian.Uint16(b))), int(int16(binary.LittleEndian.Uint16(b[2:])))
}

// orbitDisplayXY is Get_Orbit_Display_X_Y_ (0xebeeb): where a planet stands
// on its orbit this stardate, and its angular position 0..31.
func (g *Game) orbitDisplayXY(star, orbit int) (ox, oy int16, pos int) {
	sd := int32(g.state.Stardate)
	a := int((int32(star*7)+sd)*0x14bf/int32(orbit*100+0x96+star)) % 360
	rx, ry := g.orbitRadius(orbit + 1)
	c := g.cos(int(int16(a)), rx)
	s := g.sin(int(int16(a)), ry)
	if c+5 < 0 {
		c = -((-c) / 10)
	} else {
		c = (c + 10) / 10
	}
	if s-5 < 0 {
		s = -((10 - s) / 10)
	} else {
		s = s / 10
	}
	pos = (a*32 + 0xc0) / 360
	if pos == 32 {
		pos = 0
	}
	return int16(c), int16(-s), pos
}

// orbitFromXY is Get_Orbit_From_X_Y_ (0xebfcb): the orbit nearest to a
// point relative to the orbit centre, or -1.
func (g *Game) orbitFromXY(dx, dy int) int {
	if dx == 0 && dy == 0 {
		return -1
	}
	best, bi := uint32(0xffffffff), -1
	for i := 6; i >= 0; i-- {
		rx, ry := g.orbitRadius(i)
		a := g.getAngle(dx*ry, rx*-dy)
		c := g.cos(a, rx)
		if c+5 >= 0 {
			c = (c + 10) / 10
		} else {
			c = -((-c) / 10)
		}
		s := g.sin(a, ry)
		if s-5 >= 0 {
			s = s / 10
		} else {
			s = -((10 - s) / 10)
		}
		d := uint32((s+dy)*(s+dy) + (c-dx)*(c-dx))
		if d < best {
			best, bi = d, i
		}
	}
	if bi >= 1 && bi <= 5 {
		return bi - 1
	}
	return -1
}

// systemTag is the "loaded" tag of a star: "%d> %s", cut to 20 chars.
func (g *Game) systemTag(star int) string {
	t := fmt.Sprintf("%d> %s", star, save.CString(g.state.Stars[star].Name[:]))
	if len(t) > 20 {
		t = t[:20]
	}
	return t
}

// initSystemDisplay is Init_System_Display_ (0xa3fe6): reload when the tag
// differs.
func (g *Game) initSystemDisplay(star int) error {
	sd := &g.sd
	t := g.systemTag(star)
	if sd.hasLoaded && len(sd.loaded) >= len(t) && sd.loaded[:len(t)] == t {
		return nil
	}
	sd.starID = int16(star)
	sd.loaded, sd.hasLoaded = t, true
	sd.skipPlanets = true
	if g.state.MoveableBoxes[0].Content < g.ms.boxOffFlag[0] {
		sd.skipDisplay = true
	}
	if err := g.loadNewSystemDisplay(star); err != nil {
		return err
	}
	sd.iconScanned = -1
	st := &g.state.Stars[star]
	if st.Special == 9 && g.state.playerHasVisited(int(g.state.PlayerNum), star) {
		g.clearSystemSpecialSlots()
	}
	return nil
}

// clearSystemDisplayLoadedFlag is Clear_System_Display_Loaded_Flag_.
func (g *Game) clearSystemDisplayLoadedFlag() {
	if g.sd.hasLoaded {
		g.sd.loaded = ""
	}
}

// loadNewSystemDisplay is Load_New_System_Display_ (0xa404e).
func (g *Game) loadNewSystemDisplay(star int) error {
	e, s, sd := g.e, &g.state, &g.sd
	e.V.SetPaletteChanges(0, 0xff)
	e.ClearReducedPaletteGroups()
	e.MoxUpdate()
	frame, err := g.cached(buffer0LBX, 0x49)
	if err != nil {
		return err
	}
	s.MoveableBoxes[0].Width, s.MoveableBoxes[0].Height = int16(frame.Width), int16(frame.Height)
	st := &s.Stars[star]
	if _, err := g.cached(buffer0LBX, 0x53+int(st.SpectralClass)); err != nil {
		return err
	}
	e.MoxUpdate()
	sd.sunBeforePlanet = true
	any := false
	for i := 0; i < 5; i++ {
		rec := &sd.disp[i]
		pl := int(st.Planets[i])
		if pl < 0 {
			rec.planet, rec.x, rec.y = -1, 999, 999
			continue
		}
		p := &s.Planets[pl]
		dim := planetDim(p)
		ox, oy, pos := g.orbitDisplayXY(star, int(p.Orbit))
		if i == 0 && (pos <= 2 || (pos >= 12 && pos <= 15)) {
			sd.sunBeforePlanet = false
		}
		rec.planet, rec.x, rec.y = int16(pl), ox-dim/2, oy-dim/2
		rec.slot, rec.pos = uint8(i), uint8(pos)
		if rec.pict, err = g.rotatingPlanet(pl); err != nil {
			return err
		}
		rec.frame = int16((i * 5) % rec.pict.Frames)
		e.AddReducedPaletteGroup(rec.pict)
		any = true
	}
	e.MoxUpdate()
	if any {
		e.CompressReducedPaletteGroups(0xc0, 0x40)
		e.UpdateGlassRemapColors()
		g.calculateRemapGrayPalettes()
	}
	g.ms.mainscrPalette = 0
	e.MoxUpdate()
	clib.Qsort(sd.disp[:], 14, func(a, b *sysDispRec) int { return int(a.y) - int(b.y) })
	if !sd.lookAtColonies && !g.okToViewSystem(star, int(s.PlayerNum)) {
		sd.description = g.hMessage(0x0f + int(st.SpectralClass))
	}
	return nil
}

// planetDim is the hit size of a planet: _rot_plan_dim by size, gas giants
// use slot 4.
func planetDim(p *save.Planet) int16 {
	if p.Type == 2 {
		return rotPlanDim[4]
	}
	return rotPlanDim[p.Size%5]
}

// rotatingPlanet is Get_Rotating_Planet_Segment_ (0xa3d84).
func (g *Game) rotatingPlanet(pl int) (*gfx.Picture, error) {
	s := &g.state
	p := &s.Planets[pl]
	c := int(p.Climate)
	if col := int(p.Colony); col >= 0 && col < int(s.NumColonies) && p.Type != 2 && s.Colonies[col].IsOutpost == 0 {
		c = int(s.Colonies[col].Climate)
	}
	n := 50
	if p.Type != 2 {
		n = c*5 + min(int(p.Size), 4)
	}
	return g.cached(buffer0LBX, 0x5c+n)
}

// okToViewSystem is Ok_To_View_System_.
func (g *Game) okToViewSystem(star, p int) bool {
	s := &g.state
	if s.Stars[star].SpectralClass == save.SpectralBlackHole {
		return false
	}
	return s.playerHasVisited(p, star) || s.playerIsOmniscient(p)
}

// calculateRemapGrayPalettes is Calculate_Remap_Gray_Palettes_ (0xa5050)
// with _remap_gray_percentages as Harold_Defaults_ leaves them.
func (g *Game) calculateRemapGrayPalettes() {
	g.e.CalculateRemapGrayPalettes(g.ms.remapGrayPercentages)
}

// popupXY is Popup_XY_ for box 0: the corner opposite the star.
func (g *Game) popupXY(sx, sy int16) (int16, int16) {
	b := &g.state.MoveableBoxes[0]
	switch {
	case sx < 0xfd && sy < 200:
		return 0x20f - b.Width, 0x1a5 - b.Height
	case sx < 0xfd && sy > 200:
		return 0x20f - b.Width, 0x16
	case sx > 0xfd && sy < 200:
		return 0x16, 0x1a5 - b.Height
	}
	return 0x16, 0x16
}

// doSystemPopup is Do_System_Popup_ (0x83669).
func (g *Game) doSystemPopup(star int) error {
	s := &g.state
	reopened := false
	g.setFleetMovementIconSelections(0, false)
	s.MoveableBoxes[0].Content = int16(star)
	if g.moveableBoxSelected(2) {
		g.vanishFleetMovementBox()
		g.clearSystemDisplayLoadedFlag()
		reopened = true
	}
	if err := g.initSystemDisplay(star); err != nil {
		return err
	}
	sx, sy := g.starDrawCoords(star)
	b := &s.MoveableBoxes[0]
	b.X, b.Y = g.popupXY(sx, sy)
	if g.settings[0x04] != 0 {
		g.Log("Draw_Exploding_Plasma_Box_ is not ported")
	} else if reopened {
		g.e.ClearFields()
		g.setMainScreenFieldsToNull()
		g.e.DisableDrawAllFields()
		g.e.V.SetPageOff()
		if err := g.drawMiniMainScreen(); err != nil {
			return err
		}
		if err := g.e.TogglePages(); err != nil {
			return err
		}
		g.e.EnableDrawAllFields()
	}
	g.moveBoxToTop(0)
	return nil
}

// moveBoxToTop is Move_Box_To_Top_: box b goes to index 0 of the order.
func (g *Game) moveBoxToTop(b int) {
	o := &g.state.MoveableBoxOrder
	k := -1
	for i, v := range o {
		if int(v) == b {
			k = i
			break
		}
	}
	if k < 0 {
		return
	}
	copy(o[1:k+1], o[0:k])
	o[0] = int16(b)
}

// vanishSystemDisplayBox is Vanish_System_Display_Box_ (0x91abc).
func (g *Game) vanishSystemDisplayBox() {
	s, sd := &g.state, &g.sd
	s.MoveableBoxes[0].Content = g.ms.boxOffFlag[0]
	for i := range sd.planetButtons {
		sd.planetButtons[i] = engine.NoField
	}
	for i := 0; i < 8; i++ {
		sd.ships[i].field = engine.NoField
	}
	s.MoveableBoxes[0].DragField, s.MoveableBoxes[0].Field2 = engine.NoField, engine.NoField
}

// buildSystemPopupShips is Build_System_Popup_Ships_ (0xa200e): one icon per
// owner among the map icons at the star.
func (g *Game) buildSystemPopupShips(star int) error {
	s, st, sd := &g.state, &g.stacks, &g.sd
	for i := range sd.ships {
		sd.ships[i] = sysDisplayShip{field: engine.NoField, node: -1}
	}
	sd.playerHasShip = false
	cur := int16(0)
	for i := 0; i < int(st.iconCount); i++ {
		n := st.icon[i].node
		sh := &s.Ships[st.node[n].ship]
		if int(sh.Location) != star || int8(sh.Status) >= 3 {
			continue
		}
		o := int(sh.Owner)
		if o < 0 || o >= len(sd.ships) {
			continue
		}
		sd.ships[o].node = n
		if o == int(s.PlayerNum) {
			sd.playerHasShip = true
		}
		pic, err := g.systemShipPicture(o)
		if err != nil {
			return err
		}
		if pic == nil {
			continue
		}
		// Get_System_Display_Ship_Icon_XY_ (0xa3e3d).
		if g.currentScreen == ScreenLeaders {
			sd.ships[o].x = cur + 5
			sd.ships[o].y = int16(0xb4 - pic.Height - 3)
		} else {
			sd.ships[o].x = cur + 0x12
			sd.ships[o].y = int16((0x19-pic.Height)/2 + 0xec)
		}
		cur += int16(pic.Width + 3)
	}
	return nil
}

// systemShipPicture is Buffer_Reload_System_Ships_ (0xa5844).
func (g *Game) systemShipPicture(o int) (*gfx.Picture, error) {
	switch {
	case o < 8:
		return g.cached(buffer0LBX, 0x111+int(g.state.Players[o].Color))
	case o == 8:
		return g.cached(buffer0LBX, 0x119)
	case o < 15:
		return g.cached(buffer0LBX, 0x11a+o-9)
	}
	return nil, nil
}

// addSystemDisplayFields is Wrapper_For_Normal_System_Fields_ (0xa4f58):
// the box's fields, the drag, close and whole-box fields going to its record.
func (g *Game) addSystemDisplayFields() error {
	s := &g.state
	b := &s.MoveableBoxes[0]
	star := int(b.Content)
	if err := g.initSystemDisplay(star); err != nil {
		return err
	}
	drag, closeF, whole := int(b.DragField), int(b.Field2), int(b.Field3)
	if err := g.addSystemDisplayPopupFields(star, int(b.X), int(b.Y), int(b.DragDX), int(b.DragDY), &drag, &closeF, &whole); err != nil {
		return err
	}
	b.DragField, b.Field2, b.Field3 = int16(drag), int16(closeF), int16(whole)
	return nil
}

// addSystemDisplayPopupFields is Add_System_Display_Popup_Fields_ (0xa1c74)
// with the margins 0xd, 0x2f: close button and drag bar when closeF is set,
// planets, ships, gates, the map grid and the whole box. The box's size is
// box 0's, also for the discovery popup.
func (g *Game) addSystemDisplayPopupFields(star, x, y, dx, dy int, drag, closeF, whole *int) error {
	e, s, sd := g.e, &g.state, &g.sd
	b := &s.MoveableBoxes[0]
	if star < 0 || star >= int(s.NumStars) || sd.skipDisplay {
		return nil
	}
	if err := g.buildSystemPopupShips(star); err != nil {
		return err
	}
	bx, by := x+dx, y+dy
	w, h := int(b.Width), int(b.Height)
	if closeF == nil {
		*drag = engine.NoField
	} else {
		closeBtn, err := g.cached(buffer0LBX, 0x52)
		if err != nil {
			return err
		}
		if *closeF, err = e.AddClippedButtonField(bx+0x107, by+0xec, "", closeBtn, "\x1b", 0x28); err != nil {
			return err
		}
		if *drag, err = e.AddHiddenField(bx, by, bx+w, by+0x2f, "", 0); err != nil {
			return err
		}
	}
	var err error
	st := &s.Stars[star]
	for i := 4; i >= 0; i-- {
		rec := &sd.disp[i]
		if rec.planet < 0 || s.Planets[rec.planet].Type == 1 {
			sd.planetButtons[i] = engine.NoField
			continue
		}
		p := &s.Planets[rec.planet]
		dim := int(planetDim(p))
		ox, oy, _ := g.orbitDisplayXY(star, int(p.Orbit))
		x1 := int(ox) + g.sysL() + bx + 0xa1 - dim/2
		y1 := g.sysT() + by + 0x5a + int(oy) - dim/2
		if sd.planetButtons[i], err = e.AddHiddenField(x1, y1, x1+dim, y1+dim, "", 0x29); err != nil {
			return err
		}
	}
	for o := range sd.ships {
		sh := &sd.ships[o]
		if sh.node < 0 {
			continue
		}
		pic, err := g.systemShipPicture(o)
		if err != nil || pic == nil {
			continue
		}
		sh.pict = pic
		f, err := e.AddHiddenField(bx+int(sh.x), by+int(sh.y), bx+int(sh.x)+pic.Width, by+int(sh.y)+pic.Height, "", 0x29)
		if err != nil {
			return err
		}
		sh.field = int16(f)
	}
	for i := range sd.gateFields {
		sd.gateFields[i] = engine.NoField
	}
	gx, gy := bx+0xe9, by+0xf3
	p := int(s.PlayerNum)
	if st.ArtemisNet>>p&1 != 0 {
		if sd.gateFields[3], err = e.AddHiddenField(gx-12, gy, gx+3, gy+15, "", 0); err != nil {
			return err
		}
		gx -= 12
	}
	if st.DimensionalPortal>>p&1 != 0 {
		if sd.gateFields[0], err = e.AddHiddenField(gx-12, gy, gx+3, gy+15, "", 0); err != nil {
			return err
		}
	}
	if g.currentScreen == ScreenMain {
		if sd.gridField, err = e.AddGridField(bx+g.sysL(), by+g.sysT(), 1, 1, 0x140, 0xb7, &sd.gridX, &sd.gridY, 0); err != nil {
			return err
		}
	}
	if sd.offscreen {
		// port-only: no whole-box field on the leaders screen, which drops
		// it: box 0's size there reaches over the map below the system
		// display, whose stars then never take the pointer
		// (docs/screens/leaders-screen.md).
		*whole = engine.NoField
		return nil
	}
	// The original's call (0xa1fda..0xa1ffd) reads (y, x)-(y+width,
	// x+height); kept as the port had it, over the box, as the main screen
	// plays (clicks on other stars beside the box go through).
	*whole, err = e.AddHiddenField(bx, by, bx+w, by+h, "", 0)
	return err
}

// scanSystemDisplayFields is Scan_System_Display_Fields_ (0x899ab).
func (g *Game) scanSystemDisplayFields(h, r int) (ret, hovP, clkP, hovS, clkS, icon int) {
	s, sd := &g.state, &g.sd
	ret, hovP, clkP, hovS, clkS, icon = -1, -1, -1, -1, -1, -1
	gate := -1
	for k := range sd.ships {
		sh := &sd.ships[k]
		if sh.pict != nil && r == int(sh.field) {
			clkS, ret = int(sh.node), 0
			break
		}
	}
	if ret == -1 {
		for k := range sd.ships {
			sh := &sd.ships[k]
			if sh.pict != nil && h == int(sh.field) {
				hovS, ret = int(sh.node), 4
				break
			}
		}
	}
	star := int(sd.starID)
	if ret == -1 {
		for i := 0; i < 5; i++ {
			rec := &sd.disp[i]
			switch {
			case r == sd.planetButtons[i]:
				clkP = int(rec.slot)
				ret = 0
			case r == -sd.planetButtons[i]:
				if rec.planet >= 0 && s.Planets[rec.planet].Type == 2 {
					return -1, -1, -1, -1, -1, -1
				}
				if err := g.potentialColonyInfoPopup(int(rec.planet)); err != nil {
					g.Log("Potential_Colony_Info_Popup_: %v", err)
				}
				g.fullDrawScreen = true
				return -1, -1, -1, -1, -1, -1
			case h == sd.planetButtons[i]:
				hovP, ret = int(rec.slot), 4
			}
			if ret == 0 {
				break
			}
		}
	}
	px, py := g.e.Pointer()
	cx, cy := int(sd.baseX)+0xab, int(sd.baseY)+0x86
	asteroidAt := func(o int) int {
		for j := 0; j < 5; j++ {
			rec := &sd.disp[j]
			if rec.planet >= 0 && int(s.Planets[rec.planet].Orbit) == o && s.Planets[rec.planet].Type == 1 {
				return int(rec.slot)
			}
		}
		return -1
	}
	if ret == -1 && r == sd.gridField {
		if j := asteroidAt(g.orbitFromXY(cx-px, cy-py)); j >= 0 {
			clkP, ret = j, 0
		}
	}
	if ret == -1 {
		b := &s.MoveableBoxes[0]
		o := -1
		if int(sd.baseX) < px && px < int(sd.baseX)+int(b.Width) && int(sd.baseY) < py && py < int(sd.baseY)+int(b.Height) {
			o = g.orbitFromXY(cx-px, cy-py)
		}
		if j := asteroidAt(o); j >= 0 {
			hovP, ret = j, 4
		}
	}
	if ret == -1 {
		for k, f := range sd.gateFields {
			if h == f {
				gate, ret = k, 4
				break
			}
		}
	}
	_ = star
	switch {
	case hovP >= 0:
		icon = hovP
	case hovS >= 0:
		icon = hovS + 1000
	case gate >= 0:
		icon = gate + 2000
	}
	return
}

// addMoveableBoxFields is Add_Moveable_Box_Fields_ (0x90430) for the boxes
// ported so far.
func (g *Game) addMoveableBoxFields() error {
	s := &g.state
	v := g.e.V
	g.mapWindow()
	for i := 0; i < numBoxes; i++ {
		id := int(s.MoveableBoxOrder[i])
		g.clearMoveableBoxFields(id)
		if !g.moveableBoxSelected(id) {
			continue
		}
		switch id {
		case 0:
			if err := g.addSystemDisplayFields(); err != nil {
				return err
			}
		case 2:
			if g.nodePtrFromShipStack(int(g.fb.stack)) == -1 {
				g.vanishFleetMovementBox()
			} else if err := g.addFleetMovementBoxFields(s.MoveableBoxes[2].Dragging == 0); err != nil {
				return err
			}
		}
	}
	v.ResetWindow()
	v.ClipOff()
	return nil
}

// clearMoveableBoxFields is Clear_Moveable_Box_Fields_.
func (g *Game) clearMoveableBoxFields(id int) {
	if id < 0 || id >= numBoxes {
		return
	}
	b := &g.state.MoveableBoxes[id]
	b.DragField, b.Field2, b.Field3 = engine.NoField, engine.NoField, engine.NoField
	if id == 0 {
		for i := range g.sd.planetButtons {
			g.sd.planetButtons[i] = engine.NoField
		}
	}
	if id == 1 {
		g.clearGalaxyMapFields()
	}
	if id == 2 {
		g.clearFleetBoxFields()
	}
}

// systemDisplayInput is the box 0 part of Main_Screen_'s pass
// (0x86987..0x86b09).
func (g *Game) systemDisplayInput(h, r int, leave *bool) error {
	s, sd := &g.state, &g.sd
	b := &s.MoveableBoxes[0]
	if !g.moveableBoxSelected(0) || g.ms.skipFields || sd.starID < 0 || sd.starID >= s.NumStars || r == int(b.Field2) {
		return nil
	}
	px, py := g.e.Pointer()
	ret, _, clkP, hovS, clkS, icon := g.scanSystemDisplayFields(h, r)
	sd.shipScanned = int16(hovS)
	if icon == -1 && sd.iconScanned > 2000 && sd.iconScanned < 2004 {
		sd.iconScanned = -1
	} else if icon > -1 {
		sd.iconScanned = int16(icon)
	}
	if px < int(b.X) || px > int(b.X+b.Width) || py < int(b.Y) || py > int(b.Y+b.Height) {
		sd.iconScanned = -1
	}
	if ret != 0 {
		return nil
	}
	if clkS >= 0 {
		// 0x86aa0: a ship icon of the system display opens its fleet box.
		return g.doFleetPopup(g.shipIconIDFromNodePtr(int16(clkS)), false)
	}
	pl := int(s.Stars[sd.starID].Planets[clkP&7%5])
	if pl < 0 {
		return nil
	}
	if c := int(s.Planets[pl].Colony); c >= 0 && c < int(s.NumColonies) && int(s.Colonies[c].Owner) == int(s.PlayerNum) {
		return g.doColonyScreen(c, leave)
	}
	return nil
}

// scanMoveableBoxFields is Scan_Moveable_Box_Fields_ (0x89183) for box 0.
func (g *Game) scanMoveableBoxFields(scan, r int, leave *bool) error {
	s := &g.state
	for i := 0; i < numBoxes; i++ {
		if !g.moveableBoxSelected(i) {
			continue
		}
		b := &s.MoveableBoxes[i]
		if r == int(b.Field2) && !g.ms.skipFields {
			switch i {
			case 0:
				g.e.V.SetPageOff()
				if err := g.drawMainScreen(); err != nil {
					return err
				}
				if err := g.e.TogglePages(); err != nil {
					return err
				}
				g.vanishSystemDisplayBox()
			case 2:
				g.fb.selection = [63]byte{}
				for k := range g.stacks.node {
					g.stacks.node[k].selected = 0
				}
				g.fb.iconsOn = 0
				g.ms.drawToBack = true
				g.vanishFleetMovementBox()
			default:
				b.Content = g.ms.boxOffFlag[i]
			}
			continue
		}
		if !g.ms.skipFields && int(s.MoveableBoxOrder[0]) != i && (r == int(b.DragField) || r == int(b.Field3) ||
			(i == 1 && r == g.ms.galaxyMap.gridField) || (i == 2 && g.fleetBoxFieldSelected(r))) {
			g.moveBoxToTop(i)
		}
		if i == 1 {
			g.scanGalaxyMapBox(scan, r)
		}
		if i == 2 && !g.ms.skipFields {
			g.fleetBoxInput(scan, r, leave)
		}
	}
	return nil
}

// potentialColonyInfoPopup is Potential_Colony_Info_Popup_ (0xa453f): a
// right click on a planet of the system display shows what a colony of
// the player would make there. H 0x175 and 0x177 are overwritten before
// use (the original's sprintf into the same buffer), so they never show.
func (g *Game) potentialColonyInfoPopup(pl int) error {
	s := &g.state
	p := int(s.PlayerNum)
	info := g.computePotentialColonyInfo(p, pl)
	str := func(a uint32) string { return cStringAt(g.exe, a) }
	head := cSprintf(str(0x179c4d), g.hMessage(0x170)) + str(0x179c5a) + str(0x179c5d)
	msg := cSprintf(head, int(g.planetMaxPopulationForPlayer(pl, p)))
	if info[4] == 0 {
		msg += cSprintf(str(0x179c61), g.hMessage(0x171))
	} else {
		msg += cSprintf(str(0x179c6a), g.hMessage(0x171), int(info[4]/2))
	}
	msg += cSprintf(str(0x179c6a), g.hMessage(0x172), int(info[5]))
	msg += cSprintf(str(0x179c6a), g.hMessage(0x173), int(info[6]))
	row := func(v int16, none, word, plus, minus int) {
		if v == 0 {
			msg += str(0x179c74) + cSprintf(str(0x179c7d), g.hMessage(none))
			return
		}
		msg += str(0x179c87)
		_ = cSprintf(str(0x179c90), g.hMessage(word)) // overwritten next
		h := minus
		if v > 0 {
			h = plus
		}
		a := int(v)
		if a < 0 {
			a = -a
		}
		msg += cSprintf(str(0x179c95), g.hMessage(h)) + cSprintf(str(0x179c9c), a)
	}
	row(info[1], 0x174, 0x175, 0x18a, 0x189)
	row(info[3], 0x176, 0x177, 0x18b, 0x18c)
	title := cSprintf(g.hMessage(0x178), g.planetNameNoPrime(pl))
	return g.textBox(&title, msg)
}

// clearSystemSpecialSlots is Clear_System_Special_Slots_ (0xc6f26): the
// special's pictures and _g_msg_ptr (0x1831b2) are dropped.
func (g *Game) clearSystemSpecialSlots() {
	mp := &g.mp
	mp.specialAnim, mp.specialFrame = nil, nil
	mp.specialDesc = ""
}
