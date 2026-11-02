package game

import (
	"fmt"
	"strings"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The fleet movement box, moveable box 2. Spec: docs/screens/fleet-box.md.

// fleetIcon is one 9-byte _fleet_movement_icon record.
type fleetIcon struct {
	x, y, field, node int16
}

// fleetBox holds _fleet_box and the box's other globals.
type fleetBox struct {
	x, y, fullH, width, gridH int16
	n, baseH, allY            int16
	small                     bool
	buttonsSet                bool    // +0x1b
	flags                     [5]bool // attack Antares, engage, colonize, transport, outpost
	nAdded                    int

	icon         [500]fleetIcon
	ptr          [9]int16 // _fleet_movement_box_ptr
	first        int16    // _first_fleet_movement_icon
	added        int16    // _fleet_icon_buttons_added
	stack        int16    // _fleet_box_ship_stack
	shipIDs      [63]byte // _fleet_box_ship_ids
	selection    [63]byte // _fleet_icon_selection_status
	iconsOn      int      // _fleet_movement_icons_on
	iconChosen   int16    // _fleet_icon_chosen
	firstTime    bool     // _first_time
	justMoved    bool     // _just_moved_stack
	expanding    bool     // _draw_expanding_box
	addHelp      int8     // _add_help_fields
	btnPressed   bool     // _mouse_btn_pressed
	paintValue   bool     // _fltpop_field_selection_status
	overlayX     int16    // _icon_overlay_x
	overlayY     int16    // _icon_overlay_y
	fadeFirstPop bool     // _set_fade_for_first_fltpop

	bar      scrollBar
	barField int        // _fleet_movement_bar_field
	curBar   *scrollBar // _scroll_bar, the bar Get_Vertical_Scroll_Bar_First_Item_ also reads

	allButton, upButton, downButton int
	buttons                         [5]int // attack, engage, colonize, transport, outpost
	moveInfo                        shipMoveInfo
}

// scrollBar is the generic vertical scroll bar record (_fltpop_scroll_bar
// for the fleet box). pos is the thumb centre measured up from the bottom
// of the track; an engine scroll field writes it while dragged.
type scrollBar struct {
	pos, last, start, h, rows, visible, first, track int16
	kind                                             uint8 // +0x18, 2 here; not read
	press                                            uint8 // +0x19, frames drawn with the button held
	step                                             uint8 // +0x1a
	saved                                            int16 // +0x1b, pos when the height was last computed
	upField, downField                               int   // +0x10, +0x12: the screen's scroll buttons, not read
}

// Fleet box pictures in BUFFER0.LBX.
const (
	fbNameLarge, fbNameSmall     = 0x33, 0x34
	fbGridLarge, fbGridSmall     = 0x35, 0x36
	fbBottomLarge, fbBottomSmall = 0x37, 0x38
	fbThumb                      = 0x39
	fbUp, fbDown, fbAll          = 0x3a, 0x3b, 0x3c
	fbSelection                  = 0x0b
)

// fbButtonPics are the extra buttons, large and small: attack Antares,
// colonize, engage, transport, outpost, close (large order) mapped to the
// flag order attack, engage, colonize, transport, outpost.
var (
	fbLargePics = [5]int{0x3d, 0x3f, 0x3e, 0x40, 0x41}
	fbSmallPics = [5]int{0x43, 0x44, 0x45, 0x46, 0x47}
)

const (
	fbCloseLarge, fbCloseSmall = 0x42, 0x48
	flagAttack                 = 0
	flagEngage                 = 1
	flagColonize               = 2
	flagTransport              = 3
	flagOutpost                = 4
)

// fleetMovementShipID is Fleet_Movement_Ship_Id_(k).
func (g *Game) fleetMovementShipID(k int) int {
	n := g.fb.icon[k].node
	if n < 0 || n >= maxNodes {
		return 0
	}
	return int(g.stacks.node[n].ship)
}

// nodePtrFromShipStack is Node_Ptr_From_Ship_Stack_ (0x7836a).
func (g *Game) nodePtrFromShipStack(s int) int16 {
	st := &g.stacks
	switch {
	case st.count == 0:
		return 0
	case s > int(st.count):
		g.vanishFleetMovementBox()
		return 0
	case s == int(st.count):
		return int16(g.shipStackFromSavedArray())
	case s < 0:
		return -1
	}
	return st.start[s]
}

// nFleetIcons is N_Fleet_Icons_.
func (g *Game) nFleetIcons(node int16) int {
	n := 1
	for k := node; k >= 0 && k < maxNodes; k = g.stacks.node[k].next {
		if g.stacks.node[k].next == -1 || n >= 500 {
			break
		}
		n++
	}
	return n
}

// rowsFor and the grid heights follow Fleet_Box_Grid_Height_.
func fleetRows(n int) int { return min((n-1)/3+1, 3) }

func (g *Game) fleetGridHeight(rows int) int {
	switch rows {
	case 1:
		return 0x38
	case 2:
		return 0x70
	case 3:
		p, _ := g.cached(buffer0LBX, fbGridSmall)
		return p.Height
	}
	return 0
}

// fleetBoxHeight is Fleet_Box_Height_.
func (g *Game) fleetBoxHeight(n int) int {
	fb := &g.fb
	name, _ := g.cached(buffer0LBX, ifs(fb.small, fbNameSmall, fbNameLarge))
	bottom, _ := g.cached(buffer0LBX, ifs(fb.small, fbBottomSmall, fbBottomLarge))
	cl, _ := g.cached(buffer0LBX, fbCloseLarge)
	grid := 169
	if n < 10 {
		grid = g.fleetGridHeight(fleetRows(n))
	}
	return name.Height + grid + bottom.Height + cl.Height
}

func ifs(c bool, a, b int) int {
	if c {
		return a
	}
	return b
}

// fleetBoxData is Fleet_Box_Data_ (0x71a37).
func (g *Game) fleetBoxData(n int) {
	fb, s := &g.fb, &g.state
	b := &s.MoveableBoxes[2]
	if fb.firstTime {
		fb.buttonsSet = false
	}
	fb.small = n < 10
	name, _ := g.cached(buffer0LBX, ifs(fb.small, fbNameSmall, fbNameLarge))
	fb.x, fb.y = b.X, b.Y
	fb.width = int16(name.Width)
	if fb.small {
		gh := g.fleetGridHeight(fleetRows(n))
		b.Height = int16(g.fleetBoxHeight(n))
		fb.gridH = int16(gh)
	} else {
		grid, _ := g.cached(buffer0LBX, fbGridLarge)
		fb.gridH = int16(grid.Height)
	}
	fb.fullH = int16(g.fleetBoxHeight(n))
	fb.allY = int16(name.Height) + fb.gridH + 4
	fb.baseH = fb.fullH
	fb.n = int16(n)
	if !fb.buttonsSet {
		g.setAddedButtonStuff()
	}
}

// setAddedButtonStuff is Set_Added_Button_Stuff_ (0x71c01).
func (g *Game) setAddedButtonStuff() {
	fb := &g.fb
	fb.nAdded = 1
	cl, _ := g.cached(buffer0LBX, fbCloseLarge)
	h := 0
	n := int(fb.n)
	fb.flags[flagColonize] = g.colonizeFlag(n)
	fb.flags[flagTransport] = g.transportFlag(n)
	fb.flags[flagOutpost] = g.outpostFlag(n)
	fb.flags[flagEngage] = g.engageFlag(n)
	fb.flags[flagAttack] = g.attackAntaresFlag()
	for _, i := range []int{flagColonize, flagTransport, flagOutpost, flagEngage, flagAttack} {
		if fb.flags[i] {
			fb.nAdded++
			h += cl.Height
		}
	}
	fb.fullH = int16(h) + fb.baseH
	g.setFleetBoxDataDimensions()
}

// setFleetBoxDataDimensions is Set_Fleet_Box_Data_Dimensions_ (0x721eb).
func (g *Game) setFleetBoxDataDimensions() {
	fb := &g.fb
	b := &g.state.MoveableBoxes[2]
	b.Width, b.Height = fb.width, fb.fullH
	if fb.width+fb.x > 0x20f {
		fb.x = 0x20f - fb.width
		b.X = fb.x
	}
	if fb.fullH+fb.y > 0x1a5 {
		fb.y = 0x1a5 - fb.fullH
		b.Y = fb.y
	}
}

// fleetChain walks the box's stack chain for at most n nodes.
func (g *Game) fleetChain(n int, fn func(ship int) bool) {
	fb := &g.fb
	if fb.stack < 0 {
		return
	}
	node := g.stacks.start[fb.stack]
	for k := 0; k < n && node >= 0; k++ {
		if fn(int(g.stacks.node[node].ship)) {
			return
		}
		node = g.stacks.node[node].next
	}
}

func (g *Game) fleetHead() *save.Ship {
	return &g.state.Ships[g.stacks.node[g.stacks.start[g.fb.stack]].ship]
}

// colonizeFlag is Set_Colonize_Flag_ (0x71f35).
func (g *Game) colonizeFlag(n int) bool { return g.colonyOrOutpostFlag(n, 1) }

// outpostFlag is Set_Outpost_Flag_ (0x71dd8).
func (g *Game) outpostFlag(n int) bool { return g.colonyOrOutpostFlag(n, 4) }

func (g *Game) colonyOrOutpostFlag(n int, shipType uint8) bool {
	s := &g.state
	if g.fb.stack < 0 {
		return false
	}
	h := g.fleetHead()
	loc := int(h.Location)
	atStar := loc >= 0 && loc <= 499
	leaving := loc >= 1001 && loc <= 1071
	if leaving {
		loc -= 1000
	}
	if int(h.Owner) != int(s.PlayerNum) || !(atStar || leaving) || loc >= len(s.Stars) || int(s.Stars[loc].ColonizePlayer) != int(s.PlayerNum) {
		return false
	}
	ok := false
	g.fleetChain(n, func(ship int) bool {
		if s.Ships[ship].Design.Type == shipType {
			ok = true
			return true
		}
		return false
	})
	if !ok {
		return false
	}
	for _, pl := range s.Stars[loc].Planets {
		if pl < 0 {
			continue
		}
		p := &s.Planets[pl]
		if shipType == 1 && p.Type == 3 && (p.Colony < 0 || s.Colonies[p.Colony].IsOutpost != 0) {
			return true
		}
		if shipType == 4 && p.Colony < 0 {
			return true
		}
	}
	return false
}

// transportFlag is Set_Transport_Flag_ (0x72346).
func (g *Game) transportFlag(n int) bool {
	s := &g.state
	if g.fb.stack < 0 {
		return false
	}
	h := g.fleetHead()
	loc := int(h.Location)
	if int(h.Owner) != int(s.PlayerNum) || loc < 0 || loc > 499 {
		return false
	}
	ok := false
	g.fleetChain(n, func(ship int) bool {
		ok = s.Ships[ship].Design.Type == 2
		return ok
	})
	if !ok {
		return false
	}
	found := false
	s.starColonies(loc, func(c *save.Colony) {
		if int(c.Owner) == int(s.PlayerNum) {
			found = true
		}
	})
	return found
}

// engageFlag is Set_Engage_Flag_ (0x7209d).
func (g *Game) engageFlag(n int) bool {
	s := &g.state
	if g.fb.stack < 0 {
		return false
	}
	h := g.fleetHead()
	loc := int(h.Location)
	atStar := loc >= 0 && loc <= 499
	leaving := loc >= 1001 && loc <= 1071
	star := loc
	if leaving {
		star = g.xyToStarID(h.X, h.Y)
	}
	if int(h.Owner) != int(s.PlayerNum) || !(atStar || leaving) || star < 0 || star >= len(s.Stars) {
		return false
	}
	ok := false
	g.fleetChain(n, func(ship int) bool {
		sh := &s.Ships[ship]
		ok = sh.Design.Type == 0 && int8(sh.Status) < 3 && int(sh.Owner) == int(s.PlayerNum)
		return ok
	})
	return ok && s.Stars[star].IgnoreCombat>>s.PlayerNum&1 != 0
}

// attackAntaresFlag is Set_Attack_Antares_Flag_ (0x71ce7): only a
// dimensional portal at the star matters in practice.
func (g *Game) attackAntaresFlag() bool {
	s := &g.state
	if g.fb.stack < 0 {
		return false
	}
	h := g.fleetHead()
	here := g.xyToStarID(h.X, h.Y)
	if here < 0 || h.Design.Type != 0 || absoluteLocation(h.Location) == s.NumStars {
		return false
	}
	return s.Stars[here].DimensionalPortal != 0
}

// doFleetPopup is Do_Fleet_Popup_ (0x831b1).
func (g *Game) doFleetPopup(icon int, overlap bool) error {
	fb, s, m := &g.fb, &g.state, &g.ms
	z := g.mapScaleToZoomLevel()
	old := -1
	g.clearSystemDisplayLoadedFlag()
	m.shipIconPlayerID, m.fleetShipScanned, m.etaStarID = -1, -1, -1
	fb.selection = [63]byte{}
	for i := range g.stacks.node {
		g.stacks.node[i].selected = 0
	}
	fb.iconsOn = 0
	fb.firstTime = true
	fb.justMoved = false
	if overlap && g.overlappedShipIconButton(&icon, 13-z, 9-z) {
		m.drawToBack = true
	}
	switch {
	case g.moveableBoxSelected(0):
		g.vanishSystemDisplayBox()
		g.e.DisableDrawAllFields()
		g.e.V.SetPageOff()
		if err := g.drawMiniMainScreen(); err != nil {
			return err
		}
		if err := g.e.TogglePages(); err != nil {
			return err
		}
		g.e.EnableDrawAllFields()
		m.firstNodePtrInStack = -1
	case g.moveableBoxSelected(2):
		old = g.shipStackOwner(g.stacks.start[fb.stack])
	}
	b := &s.MoveableBoxes[2]
	b.Content = 1
	fb.stack = int16(g.shipStackFromShipIcon(icon))
	if fb.stack < 0 {
		g.vanishFleetMovementBox()
		return nil
	}
	ic := &g.stacks.icon[icon]
	b.X, b.Y = g.popupXY2(ic.x-0x15, ic.y-0x15)
	if g.settings[0x04] != 0 {
		fb.expanding = true
	}
	owner := g.shipStackOwner(g.stacks.start[fb.stack])
	if owner != old || m.mainscrPalette != 2 {
		if fb.fadeFirstPop {
			g.e.SetFadePercent(0)
			fb.fadeFirstPop = false
			if err := g.drawMiniMainScreen(); err != nil {
				return err
			}
		} else {
			g.e.DisableDrawAllFields()
			g.vanishFleetMovementBox()
			if err := g.redrawMainScreen(); err != nil {
				return err
			}
			g.e.EnableDrawAllFields()
		}
		b.Content = 1
		if err := g.loadPlayerShipPalette(owner); err != nil {
			return err
		}
		m.mainscrPalette = 2
	}
	g.moveBoxToTop(2)
	fb.bar.track = 0x66
	fb.addHelp = 0
	if b.X <= 0x1f && b.Y <= 0x1f {
		ix, iy, err := g.shipIconCoords(icon)
		if err != nil {
			return err
		}
		if occluded(b.X, b.Y, b.X+b.Width, b.Y+b.Height, ix, iy, ix+10, ix+13) {
			b.X, b.Y = 0x26b, 0x1a5
		}
	}
	return nil
}

// popupXY2 is Popup_XY_ for box 2: the bottom cases store 0x1a5 and rely on
// the later clamp.
func (g *Game) popupXY2(x, y int16) (int16, int16) {
	w := g.state.MoveableBoxes[2].Width
	switch {
	case x < 0xfd && y < 200:
		return 0x20f - w, 0x1a5
	case x < 0xfd && y > 200:
		return 0x20f - w, 0x16
	case x > 0xfd && y < 200:
		return 0x16, 0x1a5
	}
	return 0x16, 0x16
}

// occluded is Occluded_ (0x8faf6).
func occluded(x1, y1, x2, y2, X1, Y1, X2, Y2 int16) bool {
	ox := (X1 <= x1 && x1 <= X2) || (X1 <= x2 && x2 <= X2) || (x1 <= X1 && x2 >= X1) || (x1 <= X2 && x2 >= X2)
	oy := (Y1 <= y1 && y1 <= Y2) || (Y1 <= y2 && y2 <= Y2) || (y1 <= Y1 && y2 >= Y1) || (y1 <= Y2 && y2 >= Y2)
	return ox && oy
}

// overlappedShipIconButton is Overlapped_Ship_Icon_Button_ (0xa0fa8):
// repeated clicks on overlapping icons cycle through them.
func (g *Game) overlappedShipIconButton(icon *int, dx, dy int) bool {
	fb, st := &g.fb, &g.stacks
	ic := &st.icon[*icon]
	abs := func(v int) int {
		if v < 0 {
			return -v
		}
		return v
	}
	if abs(int(fb.overlayX-ic.x)) > dx || abs(int(fb.overlayY-ic.y)) > dy {
		fb.overlayX, fb.overlayY = ic.x, ic.y
		return false
	}
	var l []int
	for j := 0; j < int(st.iconCount) && len(l) < 50; j++ {
		if abs(int(st.icon[j].x-ic.x)) < dx && abs(int(st.icon[j].y-ic.y)) < dy {
			l = append(l, j)
		}
	}
	if len(l) == 0 {
		return false
	}
	for k := 0; k+1 < len(l); k++ {
		st.icon[l[k]], st.icon[l[k+1]] = st.icon[l[k+1]], st.icon[l[k]]
		st.start[l[k]], st.start[l[k+1]] = st.start[l[k+1]], st.start[l[k]]
	}
	*icon = l[0]
	return true
}

// shipStackFromShipIcon is Ship_Stack_From_Ship_Icon_.
func (g *Game) shipStackFromShipIcon(icon int) int {
	st := &g.stacks
	for s := 0; s < int(st.count); s++ {
		if st.start[s] == st.icon[icon].node {
			return s
		}
	}
	return -1
}

// shipStackFromShipID is Ship_Stack_From_Ship_Id_ (0x78b0e): 0 if not
// found.
func (g *Game) shipStackFromShipID(ship int) int {
	st := &g.stacks
	for s := 0; s < int(st.count); s++ {
		for n := st.start[s]; n >= 0; n = st.node[n].next {
			if int(st.node[n].ship) == ship {
				return s
			}
		}
	}
	return 0
}

// shipStackFromSavedArray is Ship_Stack_From_Saved_Array_ (0x789d4).
func (g *Game) shipStackFromSavedArray() int {
	s, st, fb := &g.state, &g.stacks, &g.fb
	for i := 0; i < maxShips; i++ {
		if int8(s.Ships[i].Status) >= 3 || !testBit(fb.shipIDs[:], i) {
			continue
		}
		firstSame := -1
		for k := 0; k < int(st.count); k++ {
			head := int(st.node[st.start[k]].ship)
			if s.Ships[head].Owner == s.Ships[i].Owner && firstSame < 0 {
				firstSame = k
			}
			if head == i {
				return k
			}
		}
		return firstSame
	}
	return -1
}

// vanishFleetMovementBox is Vanish_Fleet_Movement_Box_ (0x91a40).
func (g *Game) vanishFleetMovementBox() {
	fb, s, m := &g.fb, &g.state, &g.ms
	b := &s.MoveableBoxes[2]
	b.Content = m.boxOffFlag[2]
	for k := 0; k < int(fb.n) && k < len(fb.icon); k++ {
		fb.icon[k].field = engine.NoField
	}
	fb.allButton = engine.NoField
	fb.buttons[flagColonize] = engine.NoField
	b.DragField, b.Field2 = engine.NoField, engine.NoField
	g.mm.shipClick = -1
	m.etaStarID, m.fleetShipScanned = -1, -1
	fb.selection = [63]byte{}
	g.setMainScreenHelpList()
	m.reloadedShipPalette = false
}

// fleetBoxHelpEntries is the box 2 part of Set_Main_Screen_Help_List_
// (0x6f366..0x6f510), from _fleet_box without the drag offset. Every button
// entry starts at the top of CLOSE; the first match wins, so each button
// still gets its own help.
func (g *Game) fleetBoxHelpEntries() []engine.HelpEntry {
	fb := &g.fb
	cl, err := g.cached(buffer0LBX, fbCloseLarge)
	if err != nil {
		g.Log("fleet box help: %v", err)
		return nil
	}
	h := int16(cl.Height)
	top := fb.baseH + fb.y - h // top of CLOSE
	x1 := fb.x + 2
	x2 := fb.width + x1
	e := func(id int, y1, y2 int16) engine.HelpEntry {
		return engine.HelpEntry{ID: id, X1: int(x1), Y1: int(y1), X2: int(x2), Y2: int(y2)}
	}
	list := []engine.HelpEntry{e(0x136, top-0x23, top), e(0x130, top, top+h)}
	bottom := h
	for _, b := range []struct {
		flag, id int
	}{{flagOutpost, 0x135}, {flagColonize, 0x132}, {flagEngage, 0x133}, {flagTransport, 0x134}, {flagAttack, 0x131}} {
		if fb.flags[b.flag] {
			list = append(list, e(b.id, top, top+h+bottom))
			bottom += h
		}
	}
	return list
}

// nullFleetBoxFields is the fleet box part of Set_Main_Screen_Fields_To_Null_
// (0x8a049..0x8a076): ALL and the five extra buttons. Without it a button
// field of 0 would match an idle Get_Input.
func (g *Game) nullFleetBoxFields() {
	g.fb.allButton = engine.NoField
	for i := range g.fb.buttons {
		g.fb.buttons[i] = engine.NoField
	}
}

// clearFleetBoxFields is the box 2 part of Clear_Moveable_Box_Fields_
// (0x908da): the scroll fields, and the icon fields while the box is open.
func (g *Game) clearFleetBoxFields() {
	fb, s := &g.fb, &g.state
	fb.upButton, fb.downButton, fb.barField = engine.NoField, engine.NoField, engine.NoField
	if s.MoveableBoxes[2].Content > g.ms.boxOffFlag[2] {
		for k := 0; k < int(fb.n) && k < len(fb.icon); k++ {
			fb.icon[k].field = engine.NoField
		}
	}
}

// fleetBoxFieldSelected is the box 2 part of Box_Field_Selected_ (0x90605)
// beyond the drag and whole-box fields.
func (g *Game) fleetBoxFieldSelected(r int) bool {
	return r == g.fb.barField || r == g.fb.allButton
}

// redrawMainScreen is Redraw_Main_Screen_(0) (0x88d39).
func (g *Game) redrawMainScreen() error {
	g.e.V.SetPageOff()
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	return g.e.TogglePages()
}

// shipPaletteEntry returns the SHIPS.LBX entry carrying owner's palette.
func (g *Game) shipPaletteEntry(owner int) int {
	if owner < 8 {
		return int(g.state.Players[owner].Color)*0x32 + 0x31
	}
	return [...]int{0x19d, 0x1a3, 0x1a1, 0x1a0, 0x19e, 0x19f, 0x1a2}[(owner-8)%7]
}

// loadPlayerShipPalette is Load_Player_Ship_Palette_ (0x585e0).
func (g *Game) loadPlayerShipPalette(owner int) error {
	p, err := g.cached("SHIPS.LBX", g.shipPaletteEntry(owner))
	if err != nil {
		return err
	}
	g.e.DrawPalette(p)
	g.ms.reloadedShipPalette = true
	return nil
}

// shipPicture is Get_Ship_Id_Picture_Seg_: SHIPS.LBX by builder banner and
// design picture.
func (g *Game) shipPicture(ship int) (*gfx.Picture, error) {
	d := &g.state.Ships[ship].Design
	banner := 8
	if d.Builder < 8 {
		banner = int(g.state.Players[d.Builder].Color)
	}
	return g.cached("SHIPS.LBX", int(d.Picture)+banner*0x32)
}

// drawFleetMovementBox is Draw_Fleet_Movement_Box_ (0x70875).
func (g *Game) drawFleetMovementBox() error {
	e, fb, s, m := g.e, &g.fb, &g.state, &g.ms
	b := &s.MoveableBoxes[2]
	dx, dy := int(b.DragDX), int(b.DragDY)
	head := g.nodePtrFromShipStack(int(fb.stack))
	rebuilt := false
	if head != m.firstNodePtrInStack {
		n := g.nFleetIcons(head)
		g.fleetBoxData(n)
		fb.bar.visible, fb.bar.track = 3, 0x66
		fb.bar.rows = int16(max((n+2)/3, 1))
		fb.bar.first = 0
		fb.bar.h = fb.bar.height()
		fb.bar.last, fb.bar.kind, fb.bar.step = -1, 2, 1
		fb.bar.pos = 0x66 - fb.bar.h/2
		fb.curBar = &fb.bar
		fb.selection = [63]byte{}
		rebuilt = true
	}
	if head < 0 {
		return nil
	}
	owner := g.shipStackOwner(head)
	m.shipIconPlayerID = int16(owner)
	X, Y := int(fb.x)+dx, int(fb.y)+dy
	t := e.Text()
	if !fb.expanding {
		if err := g.drawFleetBoxFrame(X, Y); err != nil {
			return err
		}
		if err := g.drawFleetPopAddedButtons(X, Y); err != nil {
			return err
		}
		var title string
		if owner < 9 {
			g.moxFontColorsOffset = 0x71
			c := g.fontColors2(4, 0x6f, 0x73)
			title = fmt.Sprintf(g.hMessage(0x66), g.raceName(owner))
			t.SetStyleShadow(4, c, font.Outline)
			t.SetOutlineColor(2, 1)
			t.SetSpacing(1)
		} else {
			c := g.fontColors2(4, 0x4b, 0x4e)
			title = strings.ToUpper(save.CString(s.Ships[g.stacks.node[head].ship].Design.Name[:]))
			t.SetStyleShadow(4, c, font.Outline)
			t.SetOutlineColor(2, 1)
		}
		if err := t.PrintCentered(X+int(fb.width)/2, Y+0xe, title); err != nil {
			return err
		}
	}
	if m.firstNodePtrInStack != head || m.firstNodePtrInStack == -1 {
		fb.icon = [500]fleetIcon{}
		fb.added = 0
	}
	fb.first = fb.bar.first * 3
	if m.firstNodePtrInStack != head {
		node := head
		for k := 0; node != -1 && k < 500; k++ {
			fb.added = min(fb.added+1, 9)
			fb.icon[k] = fleetIcon{field: engine.NoField, node: node}
			node = g.stacks.node[node].next
		}
	}
	if fb.firstTime && g.settings[0x05] != 0 && owner == int(s.PlayerNum) {
		g.setFleetMovementIconSelections(1, true)
	}
	if rebuilt && g.stacks.count > 0 {
		fb.shipIDs = [63]byte{}
		for k := 0; k < int(fb.n); k++ {
			ship := g.fleetMovementShipID(k)
			fb.shipIDs[ship>>3] |= 1 << (ship & 7)
		}
	}
	if !fb.expanding {
		small := 0
		if fb.small {
			small = 0 // the small set draws the glow at the same place
		}
		for k, sl := int(fb.first), 0; k < min(int(fb.first)+int(fb.added), int(fb.n)); k, sl = k+1, sl+1 {
			fb.ptr[sl] = int16(k)
			x := (sl%3)*0x3a + 0xe + X
			y := (sl/3)*0x38 + 0x26 + Y
			fb.icon[k].x, fb.icon[k].y = int16(x), int16(y)
			ship := g.fleetMovementShipID(k)
			oy := 2
			if s.Ships[ship].Design.Size == 5 {
				oy = 0
			}
			if g.stacks.node[fb.icon[k].node].selected != 0 {
				glow, err := g.cached(buffer0LBX, fbSelection)
				if err != nil {
					return err
				}
				if err := e.Draw(x+small, y, glow); err != nil {
					return err
				}
			}
			pic, err := g.shipPicture(ship)
			if err != nil {
				return err
			}
			if err := e.Draw(x+2, y+oy, pic); err != nil {
				return err
			}
			if owner == int(s.PlayerNum) && s.Ships[ship].Design.Type == 0 {
				g.drawDamageBars(x-1, y, 0x35, 0x35, 4, ship)
			}
		}
	}
	if err := g.loadPlayerShipPalette(int(s.Ships[g.stacks.node[head].ship].Owner)); err != nil {
		return err
	}
	m.firstNodePtrInStack = head
	if !fb.expanding {
		if err := g.printFleetBoxInfo(); err != nil {
			return err
		}
		if fb.n > 9 {
			if _, err := g.drawGenericVerticalScrollBar(&fb.bar, g.drawFltpopScrollBar); err != nil {
				return err
			}
		}
	}
	fb.firstTime = false
	if fb.expanding {
		fb.expanding = false
		g.logOnce("Draw_Exploding_Plasma_Box_ is not ported")
		return g.drawFleetMovementBox()
	}
	if fb.addHelp == 0 {
		fb.addHelp = -1
		g.setMainScreenHelpList()
	}
	return nil
}

// drawFleetBoxFrame is Draw_Small/Large_Fleet_Box_Frame_.
func (g *Game) drawFleetBoxFrame(X, Y int) error {
	e, fb, v := g.e, &g.fb, g.e.V
	ws := g.saveWindowStatus()
	v.SetWindow(X, Y, X+int(fb.width), min(Y+int(fb.baseH), 0x1a5))
	v.ClipOn()
	v.Fill(X+6, Y+0x29, X+int(fb.width)-6, Y+int(fb.baseH)+0x1d, 0)
	g.restoreWindowStatus(ws)
	name, err := g.cached(buffer0LBX, ifs(fb.small, fbNameSmall, fbNameLarge))
	if err != nil {
		return err
	}
	if err := e.Draw(X, Y, name); err != nil {
		return err
	}
	y := Y + name.Height
	if fb.small {
		gh := g.fleetGridHeight(fleetRows(int(fb.n)))
		grid, err := g.cached(buffer0LBX, fbGridSmall)
		if err != nil {
			return err
		}
		ws := g.saveWindowStatus()
		y2 := min(y+gh, 0x1a5)
		v.SetWindow(X, y, X+int(fb.width), y2)
		v.ClipOn()
		err = e.Draw(X, y, grid)
		g.restoreWindowStatus(ws)
		if err != nil {
			return err
		}
		y += gh
		bottom, err := g.cached(buffer0LBX, fbBottomSmall)
		if err != nil {
			return err
		}
		return e.Draw(X, y, bottom)
	}
	grid, err := g.cached(buffer0LBX, fbGridLarge)
	if err != nil {
		return err
	}
	if err := e.Draw(X, y, grid); err != nil {
		return err
	}
	bottom, err := g.cached(buffer0LBX, fbBottomLarge)
	if err != nil {
		return err
	}
	return e.Draw(X, y+grid.Height, bottom)
}

// fleetButtonLayout yields the CLOSE button and the set extra buttons from
// the bottom of the box up: close, outpost, colonize, engage, transport,
// attack Antares.
func (g *Game) fleetButtonLayout(X, Y int, fn func(flag, entry, x, y int) error) error {
	fb := &g.fb
	cl, err := g.cached(buffer0LBX, fbCloseLarge)
	if err != nil {
		return err
	}
	h := cl.Height
	bx, by := X+2, Y+int(fb.baseH)-h
	closeEntry := ifs(fb.small, fbCloseSmall, fbCloseLarge)
	if err := fn(-1, closeEntry, bx, by); err != nil {
		return err
	}
	for _, f := range []int{flagOutpost, flagColonize, flagEngage, flagTransport, flagAttack} {
		if !fb.flags[f] {
			continue
		}
		by += h
		entry := fbLargePics[f]
		if fb.small {
			entry = fbSmallPics[f]
		}
		if err := fn(f, entry, bx, by); err != nil {
			return err
		}
	}
	return nil
}

// drawFleetPopAddedButtons is Draw_Fleet_Pop_Added_Buttons_ (0x70f02).
func (g *Game) drawFleetPopAddedButtons(X, Y int) error {
	return g.fleetButtonLayout(X, Y, func(_, entry, x, y int) error {
		p, err := g.cached(buffer0LBX, entry)
		if err != nil {
			return err
		}
		p.CurrentFrame = 0
		return g.e.Draw(x, y, p)
	})
}

// drawDamageBars is Draw_Damage_Bars_ (0x702ea).
func (g *Game) drawDamageBars(x, y, w, h, off, ship int) {
	v := g.e.V
	L := int(int16(off + x))
	B := int(int16(y + h))
	mx := g.maxShipHits(ship)
	cur := g.currentShipHits(ship)
	if mx == 0 {
		return
	}
	gl := int(int16(w-4)) * cur / mx
	if gl < 3 {
		gl = 2
	}
	r := (w - 4) - gl
	if cur == mx {
		return
	}
	if r == 1 {
		gl, r = w-6, 2
	}
	if gl > 0 {
		e := L + gl
		v.Line(L, B-5, e, B-5, 0x70)
		v.Line(L, B-5, L, B-1, 0x70)
		v.Line(e, B-4, e, B-1, 0x72)
		v.Line(e, B-1, L+1, B-1, 0x72)
		v.Fill(L+1, B-4, e-1, B-2, 0x71)
	}
	if r > 1 {
		a := L + gl + 1
		e := a + r
		v.Line(a, B-5, e, B-5, 0x4b)
		v.Line(a, B-5, a, B-1, 0x4b)
		v.Line(e, B-4, e, B-1, 0x4e)
		v.Line(e, B-1, L+gl+2, B-1, 0x4e)
		v.Fill(L+gl+2, B-4, e-1, B-2, 0x4d)
	}
}

// raceName is Race_Name_ (0x784a0).
func (g *Game) raceName(owner int) string {
	if owner < 8 {
		return save.CString(g.state.Players[owner].RaceName[:])
	}
	// Owner 8 reads the slot at 0x192664, which is _race_names[13] too.
	n, err := g.eStringAt(raceNames + 4*uint32(owner+5))
	if err != nil {
		g.logOnce(err.Error())
		return "Human"
	}
	return n
}

// raceNames is _race_names, E string pointers; monsters start at owner + 5.
const raceNames = 0x192630

// height is Get_Vertical_Scroll_Bar_Height_ (0xa00f1).
func (b *scrollBar) height() int16 {
	b.saved = b.pos
	return b.barHeight()
}

// barHeight is the thumb height formula, inlined in
// Draw_Generic_Vertical_Scroll_Bar_ and Handle_Vertical_Scroll_Field_.
func (b *scrollBar) barHeight() int16 {
	if b.rows < b.visible {
		return b.track
	}
	return int16((int(b.visible) - 1 + int(b.track)*int(b.visible)) / int(b.rows))
}

// setMaxima is Set_Vertical_Scroll_Bar_Maxima_ (0xa0125): keep the thumb
// inside the track; true when pos was clamped.
func (b *scrollBar) setMaxima() bool {
	hh := int(b.h) / 2
	switch {
	case int(b.track) < int(b.pos)+hh:
		b.pos = b.track - int16(hh)
		return true
	case int(b.pos)-hh < 0:
		b.pos = int16(hh)
		return true
	}
	return false
}

// firstItem is Get_Vertical_Scroll_Bar_First_Item_ (0xa0040). t is
// _scroll_bar, the bar last set up, which decides the "at the bottom" case.
func (b *scrollBar) firstItem(t *scrollBar) int16 {
	if b.rows <= 0 {
		return 0
	}
	if t == nil {
		t = b
	}
	div := int16(int(b.step) * int(b.track) * 10 / int(b.rows))
	f := int16((int(b.track) - int(int16(int(b.h)/2+int(b.pos)))) * 10 / int(div))
	if int(b.visible)+int(f) > int(b.rows) ||
		(b.visible < b.rows && int(b.h)+int(int16(int(t.track)-(int(t.pos)+int(t.h)/2))) >= int(b.track)) {
		f = b.rows - b.visible
	}
	if f < 1 {
		return 0
	}
	return f
}

// handleVerticalScrollField is Handle_Vertical_Scroll_Field_ (0x9fff5).
func (b *scrollBar) handleVerticalScrollField(t *scrollBar) {
	b.saved = b.pos
	b.h = b.barHeight()
	b.setMaxima()
	b.first = b.firstItem(t)
}

// drawGenericVerticalScrollBar is Draw_Generic_Vertical_Scroll_Bar_
// (0xa0226): turn a dragged thumb into a first row once it moved more than
// half its height, keep it in the track, draw it with fn. It returns
// whether the first row changed.
func (g *Game) drawGenericVerticalScrollBar(b *scrollBar, fn func() error) (bool, error) {
	btn := g.e.MouseButton()
	b.press = 0
	b.saved = b.pos
	old := b.first
	b.h = b.barHeight()
	recompute := b.last == -1
	if !recompute && b.last != b.pos {
		d := int(b.pos) - int(b.start)
		if d < 0 {
			d = -d
		}
		if d > int(b.h)/2 {
			b.h = b.height()
			b.setMaxima()
			b.first = b.firstItem(g.fb.curBar)
			recompute = true
		}
	}
	if recompute {
		b.last, b.start = b.pos, b.pos
	}
	if b.setMaxima() {
		b.first = b.firstItem(g.fb.curBar)
		b.last, b.start = b.pos, b.pos
	}
	if err := fn(); err != nil {
		return false, err
	}
	if btn == 1 {
		b.press++
	}
	return old != b.first, nil
}

// drawFltpopScrollBar is Draw_Fltpop_Scroll_Bar_ (0x7078b).
func (g *Game) drawFltpopScrollBar() error {
	fb := &g.fb
	b := &g.state.MoveableBoxes[2]
	bar := &fb.bar
	X := int(fb.x) + int(b.DragDX) + 0xc0
	Yb := int(fb.y) + int(b.DragDY)
	y1 := Yb + 0x43 + int(bar.track-(bar.h/2+bar.pos))
	y2 := y1 + int(bar.h)
	c := byte(0x74)
	for i := 0; i <= 8; i++ {
		g.e.V.Line(X+i, y1, X+i, y2, c)
		if i%2 == 0 {
			c--
		}
	}
	p, err := g.cached(buffer0LBX, fbThumb)
	if err != nil {
		return err
	}
	return g.e.Draw(int(fb.x)+int(b.DragDX)+0xbd, Yb+0x42, p)
}

// scrollUpOneItem and scrollDownOneItem are Scroll_Up/Down_One_Item_.
func (b *scrollBar) scrollUpOneItem() {
	if b.visible >= b.rows {
		return
	}
	b.first -= int16(b.step)
	if b.first < 1 {
		b.first = 0
		b.pos = b.h/2 + b.track
	} else {
		b.pos += int16(b.step) * (b.track / b.rows)
	}
	b.last, b.start = b.pos, b.pos
}

func (b *scrollBar) scrollDownOneItem() {
	if b.visible >= b.rows {
		return
	}
	b.first += int16(b.step)
	if b.first > b.rows-b.visible {
		b.first = b.rows - b.visible
		b.pos = b.h / 2
	} else {
		b.pos -= (b.track / b.rows) * int16(b.step)
	}
	b.last, b.start = b.pos, b.pos
}

// printFleetBoxInfo is Print_Fleet_Box_Info_ (0x71506).
func (g *Game) printFleetBoxInfo() error {
	fb, s, m := &g.fb, &g.state, &g.ms
	b := &s.MoveableBoxes[2]
	t := g.e.Text()
	s0 := g.fleetMovementShipID(0)
	sel := g.someIconSelected()
	x := int(fb.x) + int(b.DragDX) + 0x48 + ifs(fb.small, 0x38, 0x42)
	limit, y := 0x84, 0xd6
	if fb.n < 10 {
		name, _ := g.cached(buffer0LBX, ifs(fb.small, fbNameSmall, fbNameLarge))
		limit, y = 0x70, name.Height+int(fb.gridH)+10
	}
	y += int(b.Y) + int(b.DragDY)
	hl := true
	text := ""
	star := func(i int) string { return save.CString(s.Stars[i].Name[:]) }
	locInfo := func(ship int) {
		sh := &s.Ships[ship]
		if sh.Status == 0 {
			text = fmt.Sprintf(g.hMessage(0x67), star(int(sh.Location)))
			return
		}
		d := int(absoluteLocation(sh.Location))
		eta := int(sh.ETA)
		if d != int(s.NumStars) && !s.playerHasVisited(int(s.PlayerNum), d) {
			text = fmt.Sprintf(g.hMessage(ifs(eta == 1, 0x6b, 0x6c)), eta)
			return
		}
		if d == int(s.NumStars) {
			text = g.hMessage(0x68)
		} else {
			text = fmt.Sprintf(g.hMessage(ifs(eta == 1, 0x69, 0x6a)), eta, star(d))
		}
	}
	switch {
	case m.etaStarID == -1 && m.fleetShipScanned != -1:
		text = g.fleetpopScannedShipInfo()
	case m.etaStarID == -1:
		if !sel && m.fleetShipScanned == -1 && fb.iconChosen != -1 {
			m.fleetShipScanned = fb.iconChosen
		} else if m.fleetShipScanned == -1 {
			locInfo(s0)
		}
	case sel:
		mi := &fb.moveInfo
		et := int(m.etaStarID)
		switch {
		case mi.canMove:
			switch {
			case mi.turns == 0:
				text = fmt.Sprintf(g.hMessage(0x67), star(et))
			case s.playerHasVisited(int(s.PlayerNum), et):
				text = fmt.Sprintf(g.hMessage(ifs(mi.turns == 1, 0x69, 0x6a)), int(mi.turns), star(et))
			default:
				text = fmt.Sprintf(g.hMessage(ifs(mi.turns == 1, 0x6b, 0x6c)), int(mi.turns))
			}
		case mi.blocked:
			hl, text = false, g.hMessage(0x6d)
		case mi.noFTL:
			hl, text = false, g.hMessage(0x6e)
		case mi.outOfRange:
			hl = false
			if s.playerHasVisited(int(s.PlayerNum), et) {
				text = fmt.Sprintf(g.hMessage(ifs(mi.parsecsShort == 1, 0x6f, 0x70)), int(mi.parsecsShort), star(et))
			} else if mi.parsecsShort == 0 {
				text = g.hMessage(0x71)
			} else {
				text = fmt.Sprintf(g.hMessage(0xeb), int(mi.parsecsShort))
			}
		case mi.flux:
			hl, text = false, g.hMessage(0x73)
		}
	}
	if text == "" {
		return nil
	}
	var c font.Colors
	g.setFittedFontStyle(text, c, 3, 2, font.ShadowNone, limit, 1)
	if hl {
		g.moxFontColorsOffset = 0x71
		c = g.fontColors2(t.Style, 0x70, 0x73)
	} else {
		c = g.fontColors2(t.Style, 0x4b, 0x50)
	}
	t.Normal, t.Current = c, c
	if t.Style == 1 {
		y += 2
	}
	if t.Width([]byte(text)) > limit {
		return t.PrintParagraph(x-limit/2, y-6, limit, text, engine.ParaCentered)
	}
	return t.PrintCentered(x, y, text)
}

// fleetpopScannedShipInfo is Get_Fleetpop_Scanned_Ship_Info_ (0x713ce).
func (g *Game) fleetpopScannedShipInfo() string {
	s, m := &g.state, &g.ms
	if m.fleetShipScanned < 0 {
		return ""
	}
	ship := g.fleetMovementShipID(int(m.fleetShipScanned))
	sh := &s.Ships[ship]
	if int(sh.Owner) != int(s.PlayerNum) && sh.Owner < 9 && sh.Design.Type == 0 {
		// The hull table's name pointers are filled from TECHNAME.LBX at
		// start-up (Load_Tech_Names_), see hullName.
		name, err := g.hullName(int(sh.Design.Size), false)
		if err != nil {
			g.Log("hull name: %v", err)
		}
		return name
	}
	return save.CString(sh.Design.Name[:])
}

// cStringAt reads a NUL-terminated string from the image.
func cStringAt(x *exeImage, addr uint32) string { return x.cstring(addr) }

// someIconSelected is Some_Icon_Selected_: the fleet box is open and has a
// selected ship.
func (g *Game) someIconSelected() bool {
	return g.moveableBoxSelected(2) && len(g.fleetBoxSelectedShipIDs(0xff)) > 0
}

// fleetBoxSelectedShipIDs is Get_Fleet_Box_Selected_Ship_Ids_ (0x7802a).
func (g *Game) fleetBoxSelectedShipIDs(shipType int) []int16 {
	s, st := &g.state, &g.stacks
	var ids []int16
	for k := 0; k < maxNodes; k++ {
		n := &st.node[k]
		if n.ship < 0 || int(n.ship) >= maxShips {
			continue
		}
		sh := &s.Ships[n.ship]
		if sh.Status == 5 {
			n.selected = 0
		}
		if n.selected != 0 && int(sh.Owner) == int(s.PlayerNum) && (shipType == 0xff || int(sh.Design.Type) == shipType) {
			ids = append(ids, n.ship)
		}
	}
	return ids
}

// setFleetMovementIconSelections is Set_Fleet_Movement_Icon_Selections_.
func (g *Game) setFleetMovementIconSelections(mode int, auto bool) {
	fb, s := &g.fb, &g.state
	for k := 0; k < int(fb.n); k++ {
		ship := g.fleetMovementShipID(k)
		// A stale icon of -1 makes the original write before the node array.
		if int(s.Ships[ship].Owner) != int(s.PlayerNum) || fb.icon[k].node < 0 {
			continue
		}
		n := &g.stacks.node[fb.icon[k].node]
		switch {
		case mode == 0:
			n.selected = 0
		case auto:
			if g.shipCanBeSelected(ship) && !fb.justMoved {
				n.selected = 1
			}
		default:
			if g.shipCanBeSelected(ship) {
				n.selected = 1
			}
		}
	}
	fb.iconsOn = mode
}

// fleetButtonsChosen is Fleet_Buttons_Chosen_.
func (g *Game) fleetButtonsChosen() int {
	n := 0
	for k := 0; k < int(g.fb.n); k++ {
		if g.stacks.node[g.fb.icon[k].node].selected != 0 {
			n++
		}
	}
	return n
}

// firstUnselectedIcon is First_Unselected_Icon_.
func (g *Game) firstUnselectedIcon() int {
	for k := 0; k < int(g.fb.n); k++ {
		if g.stacks.node[g.fb.icon[k].node].selected == 0 {
			return k
		}
	}
	return 0
}

// saveShipSelectionStatus is Save_Ship_Selection_Status_ (0x71b91).
func (g *Game) saveShipSelectionStatus() {
	fb, s := &g.fb, &g.state
	if fb.stack < 0 || int(fb.stack) >= int(g.stacks.count) || g.shipStackOwner(g.stacks.start[fb.stack]) != int(s.PlayerNum) {
		return
	}
	for k := 0; k < int(fb.n); k++ {
		node := fb.icon[k].node
		if node == -1 {
			continue
		}
		ship := int(g.stacks.node[node].ship)
		if g.stacks.node[node].selected != 0 {
			fb.selection[ship>>3] |= 1 << (ship & 7)
		} else {
			fb.selection[ship>>3] &^= 1 << (ship & 7)
		}
	}
}

// addFleetMovementBoxFields is Add_Fleet_Movement_Box_Fields_ (0x6ff08).
func (g *Game) addFleetMovementBoxFields(buttons bool) error {
	e, fb, s, m := g.e, &g.fb, &g.state, &g.ms
	head := g.nodePtrFromShipStack(int(fb.stack))
	if head != m.firstNodePtrInStack || head < 0 {
		return nil
	}
	g.fleetBoxData(g.nFleetIcons(head))
	b := &s.MoveableBoxes[2]
	bx, by := int(b.X), int(b.Y)
	b.Field3 = engine.NoField
	name, _ := g.cached(buffer0LBX, ifs(fb.small, fbNameSmall, fbNameLarge))
	df, err := e.AddHiddenField(bx, by, bx+name.Width, by+0x23, "", 0)
	if err != nil {
		return err
	}
	b.DragField = int16(df)
	for k := int(fb.first); k < min(int(fb.first)+int(fb.added), int(fb.n)); k++ {
		ic := &fb.icon[k]
		f, err := e.AddHiddenField(int(ic.x), int(ic.y), int(ic.x)+0x35, int(ic.y)+0x35, "", 0x29)
		if err != nil {
			return err
		}
		ic.field = int16(f)
	}
	headShip := int(g.stacks.node[head].ship)
	fb.allButton = engine.NoField
	if int(s.MoveableBoxOrder[0]) == 2 && buttons && int(s.Ships[headShip].Owner) == int(s.PlayerNum) && g.playerCanOrderShip(int(s.PlayerNum), headShip) {
		all, err := g.cached(buffer0LBX, fbAll)
		if err != nil {
			return err
		}
		small := 0
		if fb.small {
			small = 1
		}
		if fb.allButton, err = e.AddClippedButtonField(bx+0x10+small, by+int(fb.allY), "", all, "", 0x28); err != nil {
			return err
		}
	}
	if fb.n > 9 {
		fb.upButton, fb.downButton = engine.NoField, engine.NoField
		if buttons {
			up, _ := g.cached(buffer0LBX, fbUp)
			down, _ := g.cached(buffer0LBX, fbDown)
			if fb.upButton, err = e.AddButtonField(bx+0xbf, by+0x2a, "", up, "-", 0x28); err != nil {
				return err
			}
			if fb.downButton, err = e.AddButtonField(bx+0xbf, by+0xae, "", down, "+", 0x28); err != nil {
				return err
			}
		}
		if fb.barField, err = e.AddScrollField(bx+0xc0, by+0x42, 0, 0x66, 0, 0x66, 10, 0x66, &fb.bar.pos, "", 0); err != nil {
			return err
		}
	}
	for i := range fb.buttons {
		fb.buttons[i] = engine.NoField
	}
	err = g.fleetButtonLayout(bx, by, func(flag, entry, x, y int) error {
		p, err := g.cached(buffer0LBX, entry)
		if err != nil {
			return err
		}
		hot := ""
		if flag < 0 {
			hot = "\x1b"
		}
		f, err := e.AddClippedButtonField(x, y, "", p, hot, 0x28)
		if err != nil {
			return err
		}
		if flag < 0 {
			b.Field2 = int16(f)
		} else {
			fb.buttons[flag] = f
		}
		return nil
	})
	if err != nil {
		return err
	}
	f3, err := e.AddHiddenField(int(b.X), int(b.Y), int(b.X)+int(fb.width), int(b.Y)+int(fb.baseH), "", 0)
	if err != nil {
		return err
	}
	b.Field3 = int16(f3)
	return nil
}

// scanFleetMovementBoxFields is Scan_Fleet_Movement_Box_Fields_ (0x88fdd).
func (g *Game) scanFleetMovementBoxFields(scan, r int) (res int, hov, chosen int16) {
	fb, s := &g.fb, &g.state
	hov, chosen = -1, -1
	for k, sl := int(fb.first), 0; k < min(int(fb.first)+int(fb.added), int(fb.n)); k, sl = k+1, sl+1 {
		ic := &fb.icon[k]
		if g.shipStackOwner(ic.node) != int(s.PlayerNum) {
			continue
		}
		if r == int(ic.field) {
			return 0, int16(k), int16(sl)
		}
		if r == -int(ic.field) {
			return 1, -1, int16(sl)
		}
	}
	for k := int(fb.first); k < min(int(fb.first)+int(fb.added), int(fb.n)); k++ {
		if scan == int(fb.icon[k].field) {
			return 4, int16(k), -1
		}
	}
	return -1, -1, -1
}

// fleetBoxInput is the box 2 part of Scan_Moveable_Box_Fields_ (0x89183).
func (g *Game) fleetBoxInput(scan, r int, leave *bool) {
	fb, m, s := &g.fb, &g.ms, &g.state
	switch {
	case r == fb.upButton && r != engine.NoField:
		fb.bar.scrollUpOneItem()
	case r == fb.downButton && r != engine.NoField:
		fb.bar.scrollDownOneItem()
	case r == fb.allButton && r != engine.NoField:
		if fb.iconsOn == 1 || g.fleetButtonsChosen() == int(fb.n) {
			g.setFleetMovementIconSelections(0, false)
			fb.selection = [63]byte{}
			m.etaStarID = -1
		} else {
			g.setFleetMovementIconSelections(1, false)
		}
	default:
		res, hov, chosen := g.scanFleetMovementBoxFields(scan, r)
		fb.iconChosen = chosen
		m.fleetShipScanned = hov
		if hov >= 0 {
			m.etaStarID = -1
		}
		if res == 1 {
			g.e.DeactivateHelpList()
			if err := g.detailedViewShip(g.fleetMovementShipID(int(chosen) + int(fb.first))); err != nil {
				g.Log("detailed ship view: %v", err)
			}
			g.setMainScreenHelpList()
			m.drawToBack = true
		}
		btn := func(f int) bool { return fb.flags[f] && r == fb.buttons[f] && r != engine.NoField }
		switch {
		case btn(flagColonize):
			m.etaStarID = -1
			g.currentScreen, g.returnScreen, *leave = ScreenColonizeInMain, 0, true
		case btn(flagTransport):
			if len(g.fleetBoxSelectedShipIDs(2)) > 0 {
				g.currentScreen, g.returnScreen, *leave = ScreenTransportInMain, 0, true
			} else {
				g.logBoxError(g.mainUserBox(g.hMessage(0x20)))
			}
		case btn(flagOutpost):
			g.currentScreen, g.returnScreen, *leave = ScreenOutpostInMain, 0, true
		case btn(flagEngage):
			loc := int(g.fleetHead().Location)
			if loc >= 0 && loc < int(s.NumStars) {
				s.Stars[loc].IgnoreCombat &^= 1 << s.PlayerNum
			}
		case btn(flagAttack) && r > 0:
			g.attackAntaresButton()
		}
	}
}

// attackAntaresButton is the attack Antares branch of
// Scan_Moveable_Box_Fields_ (0x8956d..0x89684).
func (g *Game) attackAntaresButton() {
	fb, m := &g.fb, &g.ms
	if !g.someIconSelected() {
		_, err := g.userBox(&m.drawToBack, g.hMessage(0xe6), boxWarning)
		g.logBoxError(err)
		return
	}
	k := len(g.fleetBoxSelectedShipIDs(0xff))
	s0 := g.fleetMovementShipID(0)
	msg := fmt.Sprintf(g.hMessage(ifs(k == 1, 0xe4, 0xe5)), k)
	yes, err := g.userBox(&m.drawToBack, msg, boxConfirm)
	g.logBoxError(err)
	if yes != 1 {
		return
	}
	n := g.deployShipsToAttackAntares()
	m.firstNodePtrInStack = -1
	g.state.updateStarOwners()
	g.buildShipIcons()
	g.setShipIconXYs()
	g.setFleetMovementIconSelections(0, false)
	fb.selection = [63]byte{}
	if n == int(fb.n) {
		// Ship_Stack_From_Ship_Id_ returns 0 when not found, so the
		// original's -1 test (vanish the box) never fires.
		fb.stack = int16(g.shipStackFromShipID(s0))
	}
}

// deployShipsToAttackAntares is Deploy_Ships_To_Attack_Antares_ (0xa1afa):
// the selected ships of the box's stack are ordered to Antares (NUM_STARS).
// It returns how many were sent.
func (g *Game) deployShipsToAttackAntares() int {
	st := &g.stacks
	var ids []int16
	for n := st.start[g.fb.stack]; n != -1; n = st.node[n].next {
		if st.node[n].selected != 0 {
			ids = append(ids, st.node[n].ship)
		}
	}
	if len(ids) == 0 {
		// Unreachable behind Some_Icon_Selected_; the original would read
		// ship[-1] here.
		return 0
	}
	dest := int(g.state.NumStars)
	info := g.shipsTryToMoveTo(ids, dest)
	g.makeShipsMoveTo(ids, dest, &info)
	return len(ids)
}

// fleetBoxPaint is the selection painting of Draw_Main_Screen_ (0x8475x).
func (g *Game) fleetBoxPaint() error {
	fb, s := &g.fb, &g.state
	if g.e.MouseButton() != 1 || !g.moveableBoxSelected(2) || fb.stack < 0 {
		return nil
	}
	if len(fb.icon) == 0 || g.shipStackOwner(g.stacks.start[fb.stack]) != int(s.PlayerNum) {
		return nil
	}
	b := &s.MoveableBoxes[2]
	px, py := g.e.Pointer()
	if !(int(b.X) <= px && px <= int(b.X+b.Width) && int(b.Y) <= py && py <= min(int(b.Y+b.Height), 0x1a5)) {
		return nil
	}
	f, err := g.e.ScanInput()
	if err != nil {
		return err
	}
	if f != int(b.DragField) {
		if !fb.btnPressed {
			fb.paintValue = !g.iconSelectionStatus(f)
		}
		for k, sl := int(fb.first), 0; k < min(int(fb.first)+int(fb.added), int(fb.n)); k, sl = k+1, sl+1 {
			if int(fb.icon[k].field) != f {
				continue
			}
			node := fb.icon[fb.ptr[sl]].node
			ship := int(g.stacks.node[node].ship)
			if g.playerCanOrderShip(int(s.PlayerNum), ship) {
				g.stacks.node[node].selected = 0
				if fb.paintValue {
					g.stacks.node[node].selected = 1
				}
			}
		}
	}
	fb.btnPressed = true
	return nil
}

// iconSelectionStatus is Icon_Selection_Status_ (0x7252f).
func (g *Game) iconSelectionStatus(f int) bool {
	fb := &g.fb
	for k := int(fb.first); k < min(int(fb.first)+int(fb.added), int(fb.n)); k++ {
		if int(fb.icon[k].field) == f {
			return g.stacks.node[fb.icon[k].node].selected != 0
		}
	}
	return false
}

// drawFleetBoxSelectedShipSeg is Draw_Fleet_Box_Selected_Ship_Seg_ (0x843b3).
func (g *Game) drawFleetBoxSelectedShipSeg() error {
	i := g.shipIconIDFromNodePtr(g.nodePtrFromShipStack(int(g.fb.stack)))
	st := &g.stacks
	if i < 0 || i >= int(st.iconCount) {
		return nil
	}
	pic, err := g.shipIconPicture(g.shipStackOwner(st.icon[i].node))
	if err != nil || pic == nil {
		return err
	}
	return g.e.Draw(int(st.icon[i].x), int(st.icon[i].y), pic)
}

// shipIconIDFromNodePtr is Ship_Icon_Id_From_Node_Ptr_ (0x78800); the chain
// walk skips every second node, as the original does.
func (g *Game) shipIconIDFromNodePtr(n int16) int {
	st := &g.stacks
	for i := 0; i < int(st.iconCount); i++ {
		if st.icon[i].node == n {
			return i
		}
	}
	for i := 0; i < int(st.iconCount); i++ {
		for k := st.icon[i].node; k >= 0 && k < maxNodes; {
			if k == n {
				return i
			}
			k = st.node[k].next
			if k < 0 {
				break
			}
			k = st.node[k].next
		}
	}
	return 0
}

// moveOrder is Main_Screen_'s move order on a star click (0x867bf).
func (g *Game) moveOrder(t int) {
	fb, m := &g.fb, &g.ms
	ids := g.fleetBoxSelectedShipIDs(0xff)
	info := g.shipsTryToMoveTo(ids, t)
	fb.moveInfo = info
	if !info.canMove {
		var msg int
		switch {
		case info.blocked:
			msg = 0x21
		case info.noFTL:
			msg = 0x23
		case info.flux:
			msg = 0x24
		default:
			return
		}
		g.logBoxError(g.mainUserBox(g.hMessage(msg)))
		return
	}
	c := g.fleetButtonsChosen()
	g.makeShipsMoveTo(ids, t, &info)
	m.etaStarID = -1
	f := 0
	if c != int(fb.n) {
		f = g.firstUnselectedIcon()
	} else {
		fb.justMoved = true
	}
	g.buildShipIcons()
	g.setShipIconXYs()
	fb.stack = int16(g.shipStackFromShipID(g.fleetMovementShipID(f)))
	g.setFleetMovementIconSelections(0, false)
	fb.selection = [63]byte{}
	m.drawToBack = true
	m.firstNodePtrInStack = -1
}

// drawETADestinationLine is Draw_ETA_Destination_Line_ (0x83dea).
func (g *Game) drawETADestinationLine() {
	fb, m := &g.fb, &g.ms
	p := max(3-g.mapScaleToZoomLevel(), 0)
	i := g.shipIconIDFromNodePtr(g.nodePtrFromShipStack(int(fb.stack)))
	x, y, err := g.shipIconCoords(i)
	if err != nil {
		return
	}
	w, h, err := g.shipIconDimensions(p)
	if err != nil {
		return
	}
	sx, sy := g.starDrawCoords(int(m.etaStarID))
	mi := &fb.moveInfo
	var colors [8]byte
	switch {
	case mi.canMove && uint8(mi.turns) > 0:
		colors = shipLineGreen
	case mi.blocked || mi.flux || (mi.outOfRange && !mi.noFTL):
		colors = shipLineRed
	default:
		return
	}
	g.e.V.DirectionalMultiColoredLine(int(int16(int(x)+w/2)), int(int16(int(y)+h/2)), int(sx), int(sy), colors, int(m.multiColoredLineStart))
}

// getStackStartFromSavedShipArray is Get_Stack_Start_From_Saved_Ship_Array_
// (0x780d3): the stack holding a ship of the saved box, if any.
func (g *Game) getStackStartFromSavedShipArray() bool {
	st, fb := &g.stacks, &g.fb
	stack := -1
	for s := 0; s < int(st.count) && stack < 0; s++ {
		for n := st.start[s]; n >= 0; n = st.node[n].next {
			if testBit(fb.shipIDs[:], int(st.node[n].ship)) {
				stack = s
				break
			}
		}
	}
	if stack >= 0 {
		fb.stack = int16(stack)
	}
	return stack >= 0 && stack < int(st.count)
}

// reloadShipSelectionStatus is Reload_Ship_Selection_Status_ (0x71b1c).
func (g *Game) reloadShipSelectionStatus() {
	s, st, fb := &g.state, &g.stacks, &g.fb
	if g.shipStackOwner(g.nodePtrFromShipStack(int(fb.stack))) != int(s.PlayerNum) {
		return
	}
	for k := range st.node {
		ship := int(st.node[k].ship)
		st.node[k].selected = 0
		if ship >= 0 && ship < maxShips && testBit(fb.selection[:], ship) && g.shipCanBeSelected(ship) {
			st.node[k].selected = 1
		}
	}
}

// reloadFleetBoxData is Reload_Fleet_Box_Data_ (0x71a17), from Load_Game_.
func (g *Game) reloadFleetBoxData() {
	if g.getStackStartFromSavedShipArray() {
		g.fleetBoxData(g.nFleetIcons(g.stacks.start[g.fb.stack]))
		return
	}
	g.vanishFleetMovementBox()
}
