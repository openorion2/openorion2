package game

import (
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The planet summary (screen 0x20, the PLANETS button): MOX\OTHERS\PLNTSUM.C.
// Spec: docs/screens/planet-summary.md.

// Addresses of PLNTSUM.C's data and of the tables it reads.
const (
	plntsumLBXName      = 0x1822fa // _plntsum_lbx_name, a pointer to "PLNTSUM.LBX"
	plntsumHelpList     = 0x181862 // _plntsum_screen_help_list, 15 entries
	plntscrnDeltaY      = 0x1822b8 // _plntscrn_delta_y: row tops, read one past the end
	plntsumNeutral      = 0x1822a0 // _hi_neutral_colors
	plntsumRed          = 0x1822a8 // _red_colors
	plntsumGreen        = 0x1822b0 // _green_colors
	plntsumMarkerColors = 0x99502  // 8 bytes in the code after _max_planet_flags
	plntsumLineColors   = 0x99512  // the last 4 of Draw_Nearest_Ship_Directional_Line_'s colours
	plntsumStrEmpty     = 0x179b78
	plntsumHotKeys      = 0x179b79 // "1".."5", "C", "O", "-", "+", Esc, 2 bytes apart
	plntsumStrOC        = 0x179b8d // "oc"
	plntsumStrO         = 0x179b90 // "o"
	plntsumStrC         = 0x179b92 // "c"
	plntsumOwnerFmt     = 0x179b94 // "(%s)"
	plntsumScanFmt      = 0x179b99 // "%s%s"

	planetClimateNames = 0x18f990 // _planet_climate_string
	planetSizeNames    = 0x192be0 // _planet_size_string
	planetMineralNames = 0x192bf4 // _mineral_class_string
	planetGravityNames = 0x192c74 // _planet_gravity_string
)

// Sizes of PLNTSUM.C's blocks in screen_seg.
const (
	psRecSize     = 0x1c
	psSummarySize = 0x2760
	psShipsSize   = 4000
	// psPre is the part of screen_seg before _planet_summary that index -1
	// reaches: Allocate_Next_Block_'s header {0, 0x2760, 0} at its end.
	psPre = psRecSize
)

// Offsets in a _planet_summary record (0x1c bytes).
const (
	psMaxPop      = 0x00
	psFood        = 0x02 // the planet's food per farmer
	psMinerals    = 0x04 // _minerals_per_mine of its class
	psMineralCl   = 0x06
	psClimate     = 0x08
	psPlanet      = 0x0a
	psColonyDist  = 0x0c // ETA of the nearest colony ship, +1000/+2000 by where it is
	psOutpostDist = 0x0e
	psColonyShip  = 0x10
	psOutpostShip = 0x12
	psUnreachable = 0x14 // byte: 1 when a black hole blocks every ship
	psShipIndex   = 0x15 // its _ship_summary row
	psMarkerCount = 0x17 // record 0 only: the marker colour cycle
	psMarkerDir   = 0x19 // record 0 only: its direction
	psColonyETA   = 0x1a // byte: ETA of a colony ship sent there, -1 none
	psOutpostETA  = 0x1b
)

// planetSummaryStatics are PLNTSUM.C's initialised statics, kept between
// visits: the sort (0 climate, 1 minerals, 2 max population) and filters.
type planetSummaryStatics struct {
	sortChoice int16    // _sort_choice (0x182306)
	filters    [5]int16 // enemy, gravity, hostile, mineral, range
}

// initPlanetSummaryStatics reads PLNTSUM.C's statics from the image's
// .data (0x182306..0x182311): Planets In Range starts on.
func (g *Game) initPlanetSummaryStatics() {
	st := &g.psStat
	st.sortChoice = g.exeI16(0x182306)
	for i, a := range [5]uint32{0x18230e, 0x182308, 0x18230a, 0x18230c, 0x182310} {
		st.filters[i] = g.exeI16(a)
	}
}

// planetSummaryState is PLNTSUM.C's screen state: the blocks in screen_seg
// (_planet_summary, _screen, _ship_summary, _scroll_bar) and the globals.
type planetSummaryState struct {
	sum    []byte // psPre bytes, then _planet_summary
	ships  []byte // _ship_summary, 8-byte rows: colony ship, outpost ship, icon, flags
	nShips int16  // _n_ships_in_summary
	bar    scrollBar

	pics      [14]*gfx.Picture // _screen +0..+0x34
	sendPics  [2]*gfx.Picture  // _send_colony/outpost_ship_button_seg
	rotating  [50]*gfx.Picture // _p_rotating_planet, by climate*5+size
	explored  [0x2d]byte       // _planet_explored
	reachStar [0x48]bool       // _can_reach_star

	sortBtn                      [3]int // _screen +0x38
	filterBtn                    [5]int // +0x3e
	sendColonyField, sendOutpost int    // +0x48, +0x4a
	rowField                     [8]int // +0x4c
	colonyIcon, outpostIcon      [8]int // +0x5c, +0x6c
	upBtn, downBtn, returnBtn    int    // +0x7c, +0x7e, +0x80
	starField                    [0x48]int
	barField                     int  // _plntsum_scroll_bar
	refields                     bool // +0x112
	sendChoice                   int16
	lastShipMoved                int16
	colonyBtn, outpostBtn        uint8 // _draw_colony/outpost_button_status
	foundColony, foundOutpost    bool
	canReachScanned, recalcFirst bool
	fadeFlag                     bool
	boxScanned                   int8
}

// Picture slots of _screen (offset / 4).
const (
	psBackground = 0
	psSortPics   = 1 // 1..3
	psFilterPics = 4 // 4..8
	psUpPic      = 9
	psDownPic    = 10
	psReturnPic  = 11
	psThumbPic   = 12
	psShipIcon   = 13
)

func get16(b []byte, off int) int16 { return int16(binary.LittleEndian.Uint16(b[off:])) }
func put16(b []byte, off int, v int16) {
	binary.LittleEndian.PutUint16(b[off:], uint16(v))
}

// rec returns record i of _planet_summary (i = -1 reaches the block header).
func (ps *planetSummaryState) rec(i int) []byte { return ps.sum[psPre+i*psRecSize:] }

// ship returns row i of _ship_summary.
func (ps *planetSummaryState) ship(i int) []byte { return ps.ships[i*8:] }

func (g *Game) psString(addr uint32) string { return cStringAt(g.exe, addr) }

// psEString is an E string from a pointer table Load_H_And_Pro_File_Strings_ fills.
func (g *Game) psEString(table uint32, i int) string {
	s, err := g.eStringAt(table + 4*uint32(i))
	if err != nil {
		g.logOnce(err.Error())
	}
	return s
}

func (g *Game) psColors(addr uint32) font.Colors {
	var c font.Colors
	copy(c[:], g.exe.bytes(addr, 8))
	return c
}

// planetSummaryScreen is Planet_Summary_Screen_ (0x9d252), screen 0x20.
func (g *Game) planetSummaryScreen() error {
	e, ps := g.e, &g.ps
	*ps = planetSummaryState{}
	leave := false
	ps.fadeFlag = true
	e.SetFieldOutlineColor(0xd7)
	g.setPlntSumScreenHelpList()
	g.ms.galaxyMap.colonyPalette = false
	ps.recalcFirst = false
	ps.sendChoice = -1
	g.ms.scannedField = -1
	last := int16(-1)
	g.ms.galaxyMap.starAnimCount = 0
	ps.lastShipMoved = -1
	// _climate_bit_field, _incr_value, _wait_ctr, _flash_color_value and
	// _skip_draw_planets are set here and never read on this screen.
	ps.canReachScanned = false
	ps.colonyBtn, ps.outpostBtn = 0, 0
	ps.nShips = 0
	g.ms.multiColoredLineStart = 0
	e.V.SetRefreshFull()
	g.checkForColonyAndOutpostShips()
	if ps.foundColony {
		ps.colonyBtn = 1
	}
	if ps.foundOutpost {
		ps.outpostBtn = 1
	}
	e.AssignAutoFunction(g.drawPlanetSummaryScreen, 1)
	e.ClearFields()
	if err := g.fadeToBlack(); err != nil {
		return err
	}
	if err := e.LoadPalette(6, 0, 0xfe); err != nil {
		return err
	}
	g.allocateSpaceForSummaryScreen()
	if err := g.loadPlanetSummaryPictures(); err != nil {
		return err
	}
	if err := g.addPlntsumFields(); err != nil {
		return err
	}
	r0 := ps.rec(0)
	put16(r0, psMarkerCount, 0)
	r0[psMarkerDir] = 1
	g.setPlanetExploredFlags()
	g.filterExploredPlanets(&ps.bar.rows)
	b := &ps.bar
	b.pos, b.visible, b.track = 0x197, 8, 0x197
	b.h = b.height()
	b.last = -1
	b.kind = 2
	b.pos = b.track - b.h/2
	b.step = 1
	for i := range ps.rotating {
		p, err := g.picture(g.psString(g.exe.u32(plntsumLBXName)), 0x1a+i)
		if err != nil {
			return err
		}
		ps.rotating[i] = p
	}
	g.determineButtonStatus()
	for {
		sort := g.psStat.sortChoice
		if leave {
			e.ClearFields()
			e.DisableDrawAllFields()
			e.V.SetRefreshStencil()
			e.SetMouseList(engine.MouseArrow)
			if err := g.fadeToBlack(); err != nil {
				return err
			}
			e.EnableDrawAllFields()
			g.fb.curBar = nil
			if g.moveableBoxSelected(2) && ps.lastShipMoved != -1 {
				g.ms.firstNodePtrInStack = -1
			}
			if err := g.drawMiniMainScreen(); err != nil {
				return err
			}
			e.SetFieldOutlineColor(10)
			return nil
		}
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return err
		}
		if r == ps.returnBtn {
			leave = true
			g.currentScreen = ScreenMain
		}
		if sort != g.psStat.sortChoice && (r == ps.sortBtn[0] || r == ps.sortBtn[1] || r == ps.sortBtn[2]) {
			g.sortPlanetSummary()
			g.ms.scannedField = -1
		}
		if r == ps.filterBtn[0] || r == ps.filterBtn[1] || r == ps.filterBtn[2] || r == ps.filterBtn[3] || r == ps.filterBtn[4] {
			g.filterExploredPlanets(&b.rows)
			last = -1
			switch {
			case b.rows < 8:
				b.first = 0
			case int(b.rows) < int(b.first)+8:
				b.first = b.rows - 8
				b.pos = int16(0x197 - int(int16(int(int16(b.first))*1000/int(b.rows)))*0x197/1000)
			}
			if ps.foundColony || ps.foundOutpost {
				g.buildShipSummary()
			}
		}
		if r == ps.upBtn && b.rows > 8 {
			b.scrollUpOneItem()
			last = -1
		}
		if r == ps.downBtn && b.rows > 8 {
			b.scrollDownOneItem()
			last = -1
		}
		if r == ps.barField {
			ps.recalcFirst = true
			last = -1
		}
		hit := false
		for i := 0; i < int(g.state.NumStars) && !hit; i++ {
			if r == ps.starField[i] && b.rows > 8 {
				if f := g.firstPlanetInSummaryList(i); f < 0 {
					msg := fmt.Sprintf(cFormat(g.hMessage(0x14f)), save.CString(g.state.Stars[i].Name[:]))
					if err := g.warningBox(msg); err != nil {
						return err
					}
				} else {
					b.first = int16(f)
					b.pos = 0x197 - int16(0x197/int(b.rows)+1)*b.first
				}
				hit = true
			}
			if scan == ps.starField[i] {
				g.ms.scannedField = int16(i) + 1000
				hit = true
			}
		}
		if err := g.scanSummaryBoxFields(scan, r, &last); err != nil {
			return err
		}
		if r == ps.sendColonyField && ps.colonyBtn != 0 {
			if err := g.psSendButton(&ps.colonyBtn, &ps.outpostBtn, 0); err != nil {
				return err
			}
		}
		if r == ps.sendOutpost && ps.outpostBtn != 0 {
			if err := g.psSendButton(&ps.outpostBtn, &ps.colonyBtn, 1); err != nil {
				return err
			}
		}
		if ps.refields {
			e.ClearFields()
			if err := g.addPlntsumFields(); err != nil {
				return err
			}
			ps.refields = false
		}
		if leave {
			continue
		}
		e.V.SetPageOff()
		if err := g.drawPlanetSummaryScreen(); err != nil {
			return err
		}
		if ps.fadeFlag {
			e.V.CopyOffToBack()
			if err := e.FastFadeIn(); err != nil {
				return err
			}
			ps.fadeFlag = false
			continue
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
}

// psSendButton is the SEND COLONY SHIP / SEND OUTPOST SHIP part of
// Planet_Summary_Screen_: pick this kind of ship, or drop the choice.
func (g *Game) psSendButton(mine, other *uint8, choice int16) error {
	ps := &g.ps
	if g.hyperspaceFlux() {
		if err := g.warningBox(g.hMessage(0x13f)); err != nil {
			return err
		}
	} else if *mine != 2 {
		*mine, *other = 2, 1
		ps.sendChoice = choice
		return nil
	}
	*mine = 1
	ps.sendChoice = -1
	return nil
}

// fadeToBlack is Fade_To_Black_ (0x8f26e).
func (g *Game) fadeToBlack() error {
	e := g.e
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	e.V.SetPageOff()
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	return e.TogglePages()
}

// setPlntSumScreenHelpList is Set_PlntSum_Screen_Help_List_ (0x6f52d).
func (g *Game) setPlntSumScreenHelpList() {
	g.e.DeactivateHelpList()
	g.e.SetHelpList(g.helpListAt(plntsumHelpList, 0xf))
}

// allocateSpaceForSummaryScreen is Allocate_Space_For_Summary_Screen_
// (0x99a6f): the summary and the ship rows filled with -1, the rest zero.
func (g *Game) allocateSpaceForSummaryScreen() {
	ps := &g.ps
	ps.sum = make([]byte, psPre+psSummarySize)
	binary.LittleEndian.PutUint32(ps.sum[psPre-8:], psSummarySize)
	for i := psPre; i < len(ps.sum); i++ {
		ps.sum[i] = 0xff
	}
	ps.ships = make([]byte, psShipsSize)
	for i := range ps.ships {
		ps.ships[i] = 0xff
	}
	ps.bar = scrollBar{}
	g.fb.curBar = &ps.bar
}

// loadPlanetSummaryPictures is Load_Planet_Summary_Pictures_ (0x9a709).
func (g *Game) loadPlanetSummaryPictures() error {
	ps, gm := &g.ps, &g.ms.galaxyMap
	lbx := g.psString(g.exe.u32(plntsumLBXName))
	load := func(dst **gfx.Picture, entry int) error {
		p, err := g.picture(lbx, entry)
		*dst = p
		return err
	}
	slots := []struct {
		dst   **gfx.Picture
		entry int
	}{
		{&ps.pics[psBackground], 0}, {&ps.pics[1], 1}, {&ps.pics[2], 2}, {&ps.pics[3], 3},
		{&ps.pics[4], 4}, {&ps.pics[5], 5}, {&ps.pics[6], 6}, {&ps.pics[7], 7}, {&ps.pics[8], 8},
		{&ps.sendPics[0], 9}, {&ps.sendPics[1], 10}, {&ps.pics[psUpPic], 0xb}, {&ps.pics[psDownPic], 0xc},
		{&ps.pics[psThumbPic], 0xd}, {&ps.pics[psReturnPic], 0xe},
	}
	for _, s := range slots {
		if err := load(s.dst, s.entry); err != nil {
			return err
		}
	}
	for i := range gm.summaryStarSeg {
		if err := load(&gm.summaryStarSeg[i], 0xf+i); err != nil {
			return err
		}
	}
	color := int(g.state.Players[g.state.PlayerNum].Color)
	if err := load(&ps.pics[psShipIcon], 0x4c+color); err != nil {
		return err
	}
	g.e.V.SetPaletteChanges(0, 0xff)
	g.e.ClearReducedPaletteGroups()
	return nil
}

// addPlntsumFields is Add_Plntsum_Fields_ (0x995d1).
func (g *Game) addPlntsumFields() error {
	e, ps := g.e, &g.ps
	hot := func(i int) string { return g.psString(plntsumHotKeys + 2*uint32(i)) }
	var err error
	add := func(dst *int, fn func() (int, error)) {
		if err == nil {
			*dst, err = fn()
		}
	}
	for i := range ps.sortBtn {
		x := []int{0x1b9, 0x1f5, 0x237}[i]
		add(&ps.sortBtn[i], func() (int, error) {
			return e.AddMultiButtonField(x, 200, "", ps.pics[psSortPics+i], &g.psStat.sortChoice, int16(i), "", 0x29)
		})
	}
	// The filters' variables, in field order: enemy, gravity, hostile, mineral, range.
	for i := range ps.filterBtn {
		y := 0x10a + 0x17*i
		add(&ps.filterBtn[i], func() (int, error) {
			return e.AddRadioButtonField(0x1b9, y, "", ps.pics[psFilterPics+i], &g.psStat.filters[i], hot(i), 0x29)
		})
	}
	ps.sendColonyField, ps.sendOutpost = engine.NoField, engine.NoField
	if ps.foundColony {
		add(&ps.sendColonyField, func() (int, error) { return e.AddHiddenField(0x1c6, 0x182, 0x261, 0x198, hot(5), ')') })
	}
	if ps.foundOutpost {
		add(&ps.sendOutpost, func() (int, error) { return e.AddHiddenField(0x1c6, 0x19d, 0x262, 0x1b5, hot(6), ')') })
	}
	if err != nil {
		return err
	}
	icon := ps.pics[psShipIcon]
	for i := 0; i < 8; i++ {
		ps.colonyIcon[i], ps.outpostIcon[i] = engine.NoField, engine.NoField
	}
	for i := 0; i < 8; i++ {
		r := ps.rec(int(ps.bar.first) + i)
		// Add_Colony/Outpost_Ship_Icon_Field_ (0x9953f, 0x9958b); the icons are
		// drawn elsewhere (Summary_Screen_Print_Data_), as in the original.
		if int8(r[psColonyETA]) >= 0 {
			x, y := 100-icon.Width, i*0x37-icon.Height+0x52
			// _global_var takes the field's low byte; nothing reads it here.
			add(&ps.colonyIcon[i], func() (int, error) { return e.AddHiddenField(x, y, x+icon.Width, y+icon.Height, "", 0) })
		}
		if int8(r[psOutpostETA]) >= 0 {
			y := i*0x37 + 0x54 - icon.Height
			add(&ps.outpostIcon[i], func() (int, error) { return e.AddHiddenField(0x66, y, 0x66+icon.Width, y+icon.Height, "", 0) })
		}
	}
	for i := 0; i < 8; i++ {
		dy := int(g.exeI16(plntscrnDeltaY + 2*uint32(i)))
		add(&ps.rowField[i], func() (int, error) { return e.AddHiddenField(0x12, dy+0x24, 0x19e, dy+0x56, "", ')') })
	}
	add(&ps.upBtn, func() (int, error) { return e.AddButtonField(0x1a6, 0x10, "", ps.pics[psUpPic], hot(7), 0x28) })
	add(&ps.downBtn, func() (int, error) { return e.AddButtonField(0x1a5, 0x1c0, "", ps.pics[psDownPic], hot(8), 0x28) })
	if err != nil {
		return err
	}
	if err := g.addPlntSumGalaxyMapFields(); err != nil {
		return err
	}
	// Add_Planet_Summary_Bar_Field_ (0x99981).
	add(&ps.barField, func() (int, error) {
		return e.AddScrollField(0x1a2, 0x25, 0, 0x197, 0, 0x197, 0xf, 0x197, &ps.bar.pos, "", 0)
	})
	add(&ps.returnBtn, func() (int, error) { return e.AddButtonField(0x1c6, 0x1b8, "", ps.pics[psReturnPic], hot(9), 0x28) })
	// _scroll_bar +0x10/+0x12 keep the arrows' fields; nothing reads them.
	return err
}

// addPlntSumGalaxyMapFields is Add_PlntSum_Galaxy_Map_Fields_ (0x999bf): a
// 5x5 field on each star of the small map.
func (g *Game) addPlntSumGalaxyMapFields() error {
	ps := &g.ps
	for i := range ps.starField {
		ps.starField[i] = engine.NoField
	}
	for i := 0; i < int(g.state.NumStars); i++ {
		x, y := g.galaxyMapStarXY(i, 0x1bb, 0x11, 0xb4, 0x74, 0, 0, false)
		n, err := g.e.AddHiddenField(int(x), int(y), int(x)+5, int(y)+5, "", ')')
		if err != nil {
			return err
		}
		ps.starField[i] = n
	}
	return nil
}

// checkForColonyAndOutpostShips is Check_For_Colony_And_Outpost_Ships_
// (0x99e07). An outpost ship sets _found_colony_ship too, so the loop only
// ends with the stacks.
func (g *Game) checkForColonyAndOutpostShips() {
	s, ps, st := &g.state, &g.ps, &g.stacks
	done := false
	for i := 0; i < int(st.count) && !done; i++ {
		stop := false
		n := st.start[i]
		for !stop {
			sh := int(st.node[n].ship)
			ok := g.playerCanOrderShip(int(s.PlayerNum), sh)
			d := &s.Ships[sh]
			if d.Design.Type == 1 && ok && d.Status != 1 && g.shipIsNotColonizing(sh) {
				ps.colonyBtn, ps.foundColony = 1, true
			}
			if d.Design.Type == 4 && ok && d.Status != 1 && g.shipIsNotColonizing(sh) {
				ps.outpostBtn, ps.foundColony = 1, true
			}
			if ps.foundColony && ps.foundOutpost {
				stop, done = true, true
			}
			if !stop {
				if n = st.node[n].next; n == -1 {
					stop = true
				}
			}
		}
	}
}

// shipIsNotColonizing is Ship_Is_Not_Colonizing_ (0x9bb95): in orbit, or
// in flight without an order to settle.
func (g *Game) shipIsNotColonizing(sh int) bool {
	l := g.state.Ships[sh].Location
	return (l >= 0 && l < 500) || (l >= 1000 && l < 0x5dc && g.state.colonyShipAutoColonization(sh) == -1)
}

// setPlanetExploredFlags is Set_Planet_Explored_Flags_ (0x9bb40).
func (g *Game) setPlanetExploredFlags() {
	ps, s := &g.ps, &g.state
	ps.explored = [0x2d]byte{}
	for i := 0; i < int(s.PlanetCount); i++ {
		if g.okToViewSystem(int(s.Planets[i].Star), int(s.PlayerNum)) {
			ps.explored[i>>3] |= 1 << (i & 7)
		}
	}
}

// filterExploredPlanets is Filter_Explored_Planets_ (0x9a2ba): the planets
// the player may colonise and that pass the filters. The new list replaces
// the old one only when its length changed.
func (g *Game) filterExploredPlanets(n *int16) {
	ps, s := &g.ps, &g.state
	p := int(s.PlayerNum)
	old := *n
	tmp := make([]byte, psSummarySize)
	for i := range tmp {
		tmp[i] = 0xff
	}
	*n = 0
	f := &g.psStat.filters
	for i := 0; i < int(s.PlanetCount); i++ {
		pl := &s.Planets[i]
		ok := pl.Type == 3 && !g.planetHasPlayersColony(i) && !g.planetIsOutpostPlanet(i) && g.planetHasBeenVisited(i)
		if ok && !testBit(ps.explored[:], i) {
			ok = false
		}
		if ok && f[4] != 0 && !s.starInExtendedRangeOfPlayer(int(pl.Star), p) {
			ok = false
		}
		if ok && f[1] != 0 && g.productivityPenalty(p, int(pl.Gravity)) > 0 {
			ok = false
		}
		if ok && f[0] != 0 && g.planetIsEnemyControlled(i) {
			ok = false
		}
		if ok && f[2] != 0 && pl.FoodPerFarmer == 0 {
			ok = false
		}
		if ok && f[3] != 0 && pl.Minerals <= 1 {
			ok = false
		}
		if !ok {
			continue
		}
		r := tmp[int(*n)*psRecSize:]
		put16(r, psMaxPop, g.planetMaxPopulationForPlayer(i, p))
		put16(r, psFood, int16(pl.FoodPerFarmer))
		put16(r, psMinerals, int16(g.exe.bytes(tblMineralsPerMine+uint32(pl.Minerals), 1)[0]))
		put16(r, psClimate, int16(pl.Climate))
		put16(r, psMineralCl, int16(pl.Minerals))
		put16(r, psPlanet, int16(i))
		*n++
	}
	if old != *n {
		copy(ps.sum[psPre:], tmp)
		g.sortPlanetSummary()
		g.ms.scannedField = -1
	}
}

// planetHasPlayersColony is Planet_Has_Players_Colony_ (0x9a9f8).
func (g *Game) planetHasPlayersColony(pl int) bool {
	s := &g.state
	c := s.Planets[pl].Colony
	return c != -1 && int(s.Colonies[c].Owner) == int(s.PlayerNum)
}

// planetIsOutpostPlanet is Planet_Is_Outpost_Planet_ (0x9ac0d).
func (g *Game) planetIsOutpostPlanet(pl int) bool {
	c := g.state.Planets[pl].Colony
	return c >= 0 && g.state.Colonies[c].IsOutpost != 0
}

// planetHasBeenVisited is Planet_Has_Been_Visited_ (0x9a9ac).
func (g *Game) planetHasBeenVisited(pl int) bool {
	s := &g.state
	v := s.playerHasVisited(int(s.PlayerNum), int(s.Planets[pl].Star))
	return s.playerIsOmniscient(int(s.PlayerNum)) || v
}

// productivityPenalty is Productivity_Penalty_ (0x9af07): the gravity
// penalty in percent of player p on a planet of gravity grav.
func (g *Game) productivityPenalty(p, grav int) int16 {
	pick := &g.state.Players[p].Picks
	low, high := pick[save.PickLowG] != 0, pick[save.PickHighG] != 0
	normal := !low && !high
	switch {
	case low && grav == 2, normal && grav == 2, high && grav == 0:
		return 0x32
	case low && grav == 1, normal && grav == 0:
		return 0x19
	}
	return 0
}

// planetIsEnemyControlled is Planet_Is_Enemy_Controlled_ (0x9aaed): another
// owner's colony in the system, or another owner's ship there.
func (g *Game) planetIsEnemyControlled(pl int) bool {
	s, st := &g.state, &g.stacks
	star := int(s.Planets[pl].Star)
	for k := 0; k < 5; k++ {
		q := s.Stars[star].Planets[k]
		if q >= 0 {
			if c := s.Planets[q].Colony; c >= 0 && int(s.Colonies[c].Owner) != int(s.PlayerNum) {
				return true
			}
		}
	}
	for i := 0; i < int(st.count); i++ {
		for n := st.start[i]; n != -1; n = st.node[n].next {
			sh := int(st.node[n].ship)
			if int(uint16(s.Ships[sh].Location)) == star && g.shipOwner(sh) != int(s.PlayerNum) {
				return true
			}
		}
	}
	return false
}

// sortPlanetSummary is Sort_Planet_Summary_ (0x9bbed): Watcom qsort_ of
// the records by climate, minerals or max population, descending.
func (g *Game) sortPlanetSummary() {
	ps := &g.ps
	var off int
	switch c := g.psStat.sortChoice; {
	case c == 0:
		off = psClimate
	case c < 2:
		off = psMinerals
	case c == 2:
		off = psMaxPop
	default:
		return
	}
	n := int(ps.bar.rows)
	if n <= 0 {
		return
	}
	qsortRecords(ps.sum[psPre:psPre+n*psRecSize], func(a, b []byte) int {
		return int(get16(b, off)) - int(get16(a, off))
	})
}

// buildShipSummary is Build_Ship_Summary_ (0x99afa): a row per ship of
// every stack the player commands, colony and outpost ships with their
// distances to the listed planets. Once both kinds turned up, each later
// stack contributes only its first ship.
func (g *Game) buildShipSummary() {
	ps, s, st := &g.ps, &g.state, &g.stacks
	ps.nShips = 0
	for i := range ps.ships {
		ps.ships[i] = 0xff
	}
	ps.reachStar = [0x48]bool{}
	ps.foundColony, ps.foundOutpost = false, false
	for i := 0; i < int(st.count); i++ {
		n := st.start[i]
		if !g.playerCanOrderShip(int(s.PlayerNum), int(st.node[n].ship)) || n == -1 {
			continue
		}
		stop := false
		for !stop {
			row := ps.ship(int(ps.nShips))
			put16(row, 4, int16(g.shipIconIDFromNodePtr(n)))
			sh := int(st.node[n].ship)
			owner := g.shipOwner(sh)
			d := &s.Ships[sh]
			l := d.Location
			usable := (l >= 0 && l < 500) || (l >= 1000 && l < 0x5dc && s.colonyShipAutoColonization(sh) == -1)
			switch {
			case d.Design.Type == 1 && owner == int(s.PlayerNum) && int8(d.Status) < 3:
				put16(row, 0, int16(sh))
				row[6] = 0
				if usable {
					ps.foundColony = true
					g.setShipToStarDistances(int(ps.nShips), sh, psColonyDist, psColonyShip)
					row[6] = 1
				}
			case d.Design.Type == 4 && owner == int(s.PlayerNum) && int8(d.Status) < 3:
				put16(row, 2, int16(sh))
				row[7] = 0
				if usable {
					ps.foundOutpost = true
					g.setShipToStarDistances(int(ps.nShips), sh, psOutpostDist, psOutpostShip)
					row[7] = 1
				}
			}
			ps.nShips++
			if ps.foundColony && ps.foundOutpost {
				stop = true
			}
			if !stop {
				if n = st.node[n].next; n == -1 {
					stop = true
				}
			}
		}
	}
	g.updateShipEtas()
}

// setShipToStarDistances is Set_Colony_Ship_To_Star_Distances_ (0x9b7f1)
// and Set_Outpost_Ship_To_Star_Distances_ (0x9b998), which differ only in
// the record fields: for each star with a listed planet, the ship's ETA
// (+1000 in flight, +2000 leaving), kept when it beats the current one.
func (g *Game) setShipToStarDistances(row, sh, distOff, shipOff int) {
	ps, s := &g.ps, &g.state
	l := s.Ships[sh].Location
	flying := l >= 1000 && l < 0x5dc
	leaving := l >= 500 && l < 1000
	id := get16(ps.ship(row), shipOff-psColonyShip)
	for star := 0; star < 0x48; star++ {
		k := g.summaryPlanetIndex(star)
		if k == -1 {
			continue
		}
		info := g.shipsTryToMoveTo([]int16{id, -1}, star)
		if !info.canMove {
			if info.blocked && get16(ps.rec(k), distOff) == -1 {
				ps.rec(k)[psUnreachable] = 1
			}
			continue
		}
		d := int16(uint8(info.turns))
		ps.reachStar[star] = true
		switch {
		case flying:
			d += 1000
		case leaving:
			d += 2000
		}
		if cur := get16(ps.rec(k), distOff); cur != -1 && d >= cur {
			continue
		}
		for j := 0; j < 5; j++ {
			q := s.Stars[star].Planets[j]
			if q < 0 {
				continue
			}
			m := -1
			for i := 0; i < int(ps.bar.rows) && m == -1; i++ {
				if get16(ps.rec(i), psPlanet) == q {
					m = i
				}
			}
			if m >= 0 {
				r := ps.rec(m)
				r[psUnreachable] = 0
				put16(r, distOff, d)
				put16(r, shipOff, int16(sh))
				put16(r, psShipIndex, int16(row))
			}
		}
	}
}

// summaryPlanetIndex is Summary_Planet_Index_ (0x9bf1c) and
// First_Planet_In_Summary_List_ (0x9aa2e): the first record whose planet is
// at star (a byte compared with the argument, so never for 1000+).
func (g *Game) summaryPlanetIndex(star int) int {
	ps := &g.ps
	for i := 0; i < int(ps.bar.rows); i++ {
		if int(g.state.Planets[get16(ps.rec(i), psPlanet)].Star) == int(uint16(star)) {
			return i
		}
	}
	return -1
}

func (g *Game) firstPlanetInSummaryList(star int) int { return g.summaryPlanetIndex(star) }

// planetSummaryID is Planet_Summary_Id_ (0x9ac37) and
// Planet_Summary_List_Id_ (0x9aa83): the record of planet pl.
func (g *Game) planetSummaryID(pl int) int {
	ps := &g.ps
	for i := 0; i < int(ps.bar.rows); i++ {
		if int(get16(ps.rec(i), psPlanet)) == pl {
			return i
		}
	}
	return -1
}

// updateShipEtas is Update_Ship_Etas_ (0x9cfa8): the ETA of each colony
// and outpost ship already sent to settle a listed planet.
func (g *Game) updateShipEtas() {
	ps, s := &g.ps, &g.state
	for i := 0; i < int(ps.bar.rows); i++ {
		r := ps.rec(i)
		r[psColonyETA], r[psOutpostETA] = 0xff, 0xff
	}
	for sh := 0; sh < int(s.NumShips); sh++ {
		d := &s.Ships[sh]
		dest := absoluteLocation(d.Location)
		if g.shipOwner(sh) != int(s.PlayerNum) || dest < 0 || (d.Design.Type != 1 && d.Design.Type != 4) {
			continue
		}
		eta := g.nTurnsToStar(sh, int(dest))
		k := -1
		if pl := s.colonyShipAutoColonization(sh); pl >= 0 {
			k = g.planetSummaryID(pl)
		}
		if k < 0 {
			continue
		}
		if d.Design.Type == 1 {
			ps.rec(k)[psColonyETA] = byte(eta)
		} else {
			ps.rec(k)[psOutpostETA] = byte(eta)
		}
	}
}

// nTurnsToStar is N_Turns_To_Star_ (0x79beb): the turns ship sh needs to
// reach star, or -1.
func (g *Game) nTurnsToStar(sh, star int) int16 {
	info := g.shipsTryToMoveTo([]int16{int16(sh), -1}, star)
	if !info.canMove {
		return -1
	}
	return int16(uint8(info.turns))
}

// determineButtonStatus is Determine_Button_Status_ (0x9a07b).
func (g *Game) determineButtonStatus() {
	ps := &g.ps
	g.buildShipSummary()
	ps.colonyBtn, ps.outpostBtn = 0, 0
	if ps.bar.rows <= 0 || g.hyperspaceFlux() {
		return
	}
	switch {
	case ps.foundOutpost && ps.foundColony:
		ps.sendChoice, ps.colonyBtn, ps.outpostBtn = 0, 2, 1
	case ps.foundColony && !ps.foundOutpost:
		ps.sendChoice, ps.colonyBtn = 0, 2
	case !ps.foundColony && ps.foundOutpost:
		ps.sendChoice, ps.outpostBtn = 1, 2
	}
}

// drawPlanetSummaryScreen is Draw_Planet_Summary_Screen_ and
// Draw_Mini_Planet_Summary_Screen_ (0x9d0c6), the auto function.
func (g *Game) drawPlanetSummaryScreen() error {
	e, ps := g.e, &g.ps
	if g.fullDrawScreen {
		e.V.SetPageBack()
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := e.Draw(0, 0, ps.pics[psBackground]); err != nil {
			return err
		}
		e.V.CopyBackToOff()
		g.fullDrawScreen = false
		e.V.SetPageOff()
	}
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	if err := e.Draw(0, 0, ps.pics[psBackground]); err != nil {
		return err
	}
	e.V.ResetWindow()
	e.V.ClipOff()
	if e.AutoInput() == ps.barField {
		ps.recalcFirst = true
	}
	if err := g.drawRotatingPlanets(); err != nil {
		return err
	}
	if err := g.wrapperForPlanetSummaryGalaxyMap(); err != nil {
		return err
	}
	if _, err := g.drawGenericVerticalScrollBar(&ps.bar, g.fillSummaryScreenScrollBar); err != nil {
		return err
	}
	if err := g.summaryScreenPrintData(); err != nil {
		return err
	}
	if g.ms.scannedField > -1 {
		if err := g.summaryScreenPrintScanned(); err != nil {
			return err
		}
	}
	// The ships' destinations are 500+, which Summary_Planet_Index_ never
	// finds: this draws nothing, as in the original.
	for i := 0; i < int(ps.nShips) && !ps.fadeFlag; i++ {
		ic := get16(ps.ship(i), 4)
		sh := g.stacks.node[g.stacks.icon[ic].node].ship
		d := &g.state.Ships[sh]
		if d.Location >= 500 && (d.Design.Type == 1 || d.Design.Type == 4) {
			g.drawNearestShipDirectionalLine(int(d.Location))
		}
	}
	setFrame(ps.sendPics[0], int(ps.colonyBtn))
	if err := e.Draw(0x1c6, 0x182, ps.sendPics[0]); err != nil {
		return err
	}
	setFrame(ps.sendPics[1], int(ps.outpostBtn))
	return e.Draw(0x1c6, 0x19d, ps.sendPics[1])
}

// drawRotatingPlanets is Draw_Rotating_Planets_ (0x9a125).
func (g *Game) drawRotatingPlanets() error {
	ps, s := &g.ps, &g.state
	end := min(int(ps.bar.rows)+int(ps.bar.first), int(ps.bar.first)+8)
	for i := 0; i < end-int(ps.bar.first); i++ {
		pl := get16(ps.rec(int(ps.bar.first)+i), psPlanet)
		if pl == -1 {
			continue
		}
		p := ps.rotating[int(s.Planets[pl].Climate)*5+int(s.Planets[pl].Size)]
		dy := int(g.exeI16(plntscrnDeltaY + 2*uint32(i)))
		if err := g.e.Draw((0x56-p.Width)/2+0x12, (0x19-p.Height)/2+dy+0x2d, p); err != nil {
			return err
		}
	}
	return nil
}

// fillSummaryScreenScrollBar is Fill_Summary_Screen_Scroll_Bar_ (0x9a1f0).
func (g *Game) fillSummaryScreenScrollBar() error {
	v, ps := g.e.V, &g.ps
	b := &ps.bar
	v.SetWindow(0x1a5, 0x25, 0x1af, 0x1bc)
	v.ClipOn()
	c := byte(0x96)
	top := int(b.track - (b.pos + b.h/2))
	for i, x := 0, 0x1a7; i < 10; i, x = i+1, x+1 {
		v.Line(x, int(int16(top+0x26)), x, int(int16(top+int(b.h)+0x25)), c)
		if i%2 == 0 {
			c--
		}
	}
	v.ClipOff()
	v.ResetWindow()
	ps.recalcFirst = false
	return g.e.Draw(0x1a5, 0x25, ps.pics[psThumbPic])
}

// wrapperForPlanetSummaryGalaxyMap is Wrapper_For_Planet_Summary_Galaxy_Map_
// (0x9caa0): the scanned star's box and route, the ships' markers, the map.
func (g *Game) wrapperForPlanetSummaryGalaxyMap() error {
	v, ps, s := g.e.V, &g.ps, &g.state
	v.SetWindow(0x1bb, 0x11, 0x26f, 0x85)
	v.ClipOn()
	c := byte(0xe2)
	if sf := g.ms.scannedField; sf != -1 {
		switch {
		case sf < 1000:
			g.drawNearestShipDirectionalLine(int(sf))
		case int(sf) < int(s.NumStars)+1000:
			g.drawNearestShipDirectionalLine(int(sf - 1000))
		}
		star := sf
		if sf >= 1000 && int(sf) < int(s.NumStars)+1000 {
			star -= 1000
		}
		x, y := g.galaxyMapStarXY(int(star), 0x1bb, 0x11, 0xb4, 0x74, 0, 0, true)
		if o := s.starRecord(int(star)).Owner; o >= 0 {
			c = s.Players[o].Color*6 + 0xd4
		}
		g.e.Box(int(x)-1, int(y)-1, 9, 9, 1, c)
	}
	var si int16
	for si = 0; si < ps.nShips; si++ {
		row := ps.ship(int(si))
		col := get16(row, 0) != -1 && row[6] != 0
		out := get16(row, 2) != -1 && row[7] != 0
		if col || out {
			x, y := g.colonyOutpostMarkerXY(int(si))
			if err := g.printColonyOutpostMarker(int(x), int(y), col, out); err != nil {
				return err
			}
		}
	}
	v.ClipOff()
	v.ResetWindow()
	if err := g.drawGalaxyMapBox(nil, 0x1bb, 0x11, 0xb4, 0x74, 0, 0, 0, 0, 2, -1, si); err != nil {
		return err
	}
	r0 := ps.rec(0)
	put16(r0, psMarkerCount, get16(r0, psMarkerCount)+int16(int8(r0[psMarkerDir]))*2)
	if (int8(r0[psMarkerDir]) > 0 && get16(r0, psMarkerCount) > 2) || (int8(r0[psMarkerDir]) < 0 && get16(r0, psMarkerCount) < 1) {
		r0[psMarkerDir] = byte(-int8(r0[psMarkerDir]))
	}
	return nil
}

// colonyOutpostMarkerXY is Get_Colony_Outpost_Marker_XY_ (0x9a4f1): where
// a ship row's marker goes: its icon in space, else its star.
func (g *Game) colonyOutpostMarkerXY(row int) (int16, int16) {
	ps, s := &g.ps, &g.state
	r := ps.ship(row)
	sh := max(get16(r, 0), get16(r, 2))
	l := s.Ships[sh].Location
	var x, y int16
	switch {
	case l >= 500 && l < 1000:
		ic := &g.stacks.icon[get16(r, 4)]
		x = 0x1bb + int16((int32(ic.x)-0x15)*(0xb4*1000/0x1fa+1)/1000)
		y = 0x11 + int16((int32(ic.y)-0x15)*(0x74*1000/400+1)/1000)
	default:
		star := int(l)
		if l >= 1000 && l < 0x5dc {
			star = g.xyToStarID(s.Ships[sh].X, s.Ships[sh].Y)
		}
		x, y = g.galaxyMapStarXY(star, 0x1bb, 0x11, 0xb4, 0x74, 0, 0, true)
	}
	return x + 2, y
}

// printColonyOutpostMarker is Print_Colony_Outpost_Marker_ (0x9ac79): "c",
// "o" or "oc" by a ship row's marker, in cycling colours on the chosen kind
// while a planet row is hovered.
func (g *Game) printColonyOutpostMarker(x, y int, col, out bool) error {
	t, ps := g.e.Text(), &g.ps
	var base, cyc font.Colors
	copy(base[:], g.exe.bytes(plntsumMarkerColors, 8))
	cyc[0] = 0
	for i := 1; i < 8; i++ {
		k := min(max(int(get16(ps.rec(0), psMarkerCount))+i, 1), 7)
		cyc[i] = base[k]
	}
	sf := g.ms.scannedField
	hover := sf >= 0 && sf < 1000
	oc, o, c := g.psString(plntsumStrOC), g.psString(plntsumStrO), g.psString(plntsumStrC)
	if col && out {
		x, y = x+3, y-1
		if !hover || (ps.sendChoice != 0 && ps.sendChoice != 1) {
			t.SetStyle(0, base)
			return t.Print(x, y, oc)
		}
		first, second := cyc, base
		if ps.sendChoice == 0 {
			first, second = base, cyc
		}
		t.SetStyle(0, first)
		g.e.V.Fill(x, y, t.Width([]byte(oc))+x, t.Height()+y, 1)
		if err := t.Print(x, y, o); err != nil {
			return err
		}
		t.SetStyle(0, second)
		return t.Print(t.Width([]byte(o))+x, y, c)
	}
	x, y = x+6, y-1
	str := o
	if !out {
		if !col {
			return nil
		}
		str = c
	}
	if !hover {
		t.SetStyle(0, base)
	} else {
		t.SetStyle(0, cyc)
		g.e.V.Fill(x, y, t.Width([]byte(str))+x, t.Height()+y, 1)
	}
	return t.Print(x, y, str)
}

// drawNearestShipDirectionalLine is Draw_Nearest_Ship_Directional_Line_
// (0x9bc46): the route of the nearest ship of the chosen kind to star.
func (g *Game) drawNearestShipDirectionalLine(star int) {
	ps, s := &g.ps, &g.state
	k := g.summaryPlanetIndex(star)
	colors := [8]byte{0xe2, 0xe3, 0xe4, 0xe5}
	copy(colors[4:], g.exe.bytes(plntsumLineColors, 4))
	if k == -1 || (!ps.foundColony && !ps.foundOutpost) {
		return
	}
	colony, outpost := false, false
	if ps.colonyBtn == 1 || (ps.outpostBtn != 1 && (ps.foundColony || !ps.foundOutpost)) {
		colony = true
	} else {
		outpost = true
	}
	sh := get16(ps.rec(k), psColonyShip)
	if !colony {
		sh = get16(ps.rec(k), psOutpostShip)
	}
	if sh == -1 {
		return
	}
	departing := s.Ships[sh].Status == 1
	mx, my := g.colonyOutpostMarkerXY(int(get16(ps.rec(k), psShipIndex)))
	m := &g.ms
	line := func(dx int16) {
		if sh < 0 || !ps.reachStar[star] || departing {
			return
		}
		x, y := g.galaxyMapStarXY(star, 0x1bb, 0x11, 0xb4, 0x74, 0, 0, false)
		if rangeXY(mx, my, x+4, y+3) > 10 {
			g.e.V.DirectionalMultiColoredLine(int(int16(mx+dx)), int(int16(my+2)), int(int16(x+2)), int(int16(y+2)), colors, int(m.multiColoredLineStart))
			m.multiColoredLineStart = (m.multiColoredLineStart + 1) % 8
		}
	}
	if colony {
		line(7)
	}
	if outpost {
		line(8)
	}
}

// qsortRecords is qsort_ over the summary's 0x1c-byte records.
func qsortRecords(b []byte, cmp func(a, b []byte) int) {
	n := len(b) / psRecSize
	recs := make([][psRecSize]byte, n)
	for i := range recs {
		copy(recs[i][:], b[i*psRecSize:])
	}
	clib.Qsort(recs, psRecSize, func(x, y *[psRecSize]byte) int { return cmp(x[:], y[:]) })
	for i := range recs {
		copy(b[i*psRecSize:], recs[i][:])
	}
}

// scanSummaryBoxFields is Scan_Summary_Box_Fields_ (0x9b04d): a click on a
// row sends the nearest ship, on a ship icon recalls it; hovering a row
// marks it, and a new hover tries the move to show its ETA.
func (g *Game) scanSummaryBoxFields(scan, r int, last *int16) error {
	ps, s := &g.ps, &g.state
	b := &ps.bar
	end := min(int(b.rows)+int(b.first), int(b.first)+8)
	ps.canReachScanned = false
	ps.boxScanned = -1
	for i := 0; end-int(b.first) > i; i++ {
		k := int(b.first) + i
		pl := get16(ps.rec(k), psPlanet)
		if pl == -1 {
			continue
		}
		star := int(s.Planets[pl].Star)
		choosing := ps.sendChoice == 0 || ps.sendChoice == 1
		switch {
		case r == ps.rowField[i]:
			if !choosing || g.nearestShipID(g.planetSummaryID(int(pl))) <= -1 {
				continue
			}
			if err := g.sendNearestShip(int(pl)); err != nil {
				return err
			}
			g.determineButtonStatus()
			g.resetShipToStarDistances()
		case r == ps.colonyIcon[i], r == ps.outpostIcon[i]:
			kind := 1
			if r != ps.colonyIcon[i] {
				kind = 4
			}
			if err := g.undoSendOrder(int(pl), kind); err != nil {
				return err
			}
			g.determineButtonStatus()
			g.resetShipToStarDistances()
		case scan == ps.rowField[i] && int(b.rows) > k:
			ps.boxScanned = int8(i)
			if g.nearestShipID(g.planetSummaryID(int(pl))) != -1 && choosing {
				ps.canReachScanned = true
			}
			if int16(scan) == *last {
				continue
			}
			g.ms.scannedField = int16(star) + 1000
			off := -1
			switch ps.sendChoice {
			case 0:
				off = psColonyShip
			case 1:
				off = psOutpostShip
			}
			if off >= 0 {
				if sh := get16(ps.rec(k), off); sh != -1 {
					g.fb.moveInfo = g.shipsTryToMoveTo([]int16{sh, -1}, star)
					g.ms.scannedField = int16(star)
				}
			}
			*last = int16(scan)
		case scan == ps.colonyIcon[i], scan == ps.outpostIcon[i]:
			ps.canReachScanned = true
		}
	}
	return nil
}

// nearestShipID is Nearest_Ship_Id_ (0x9b287): record k's ship of the
// chosen kind, -1 with no choice.
func (g *Game) nearestShipID(k int) int16 {
	ps := &g.ps
	switch ps.sendChoice {
	case 0:
		return get16(ps.rec(k), psColonyShip)
	case 1:
		return get16(ps.rec(k), psOutpostShip)
	}
	return -1
}

// resetShipToStarDistances is Reset_Ship_To_Star_Distances_ (0x9af72).
func (g *Game) resetShipToStarDistances() {
	ps := &g.ps
	kind := 4
	if ps.sendChoice == 0 {
		kind = 1
	}
	for i := 0; i < int(ps.bar.rows); i++ {
		if kind == 1 {
			put16(ps.rec(i), psColonyDist, 9999)
		} else {
			put16(ps.rec(i), psOutpostDist, 9999)
		}
	}
	n := int16(g.state.NumShips)
	for i := 0; i < int(ps.nShips); i++ {
		row := ps.ship(i)
		if kind == 1 && get16(row, 0) >= 0 && get16(row, 0) < n {
			g.setShipToStarDistances(i, int(get16(row, 0)), psColonyDist, psColonyShip)
		} else if kind == 4 && get16(row, 2) >= 0 && get16(row, 2) < n {
			g.setShipToStarDistances(i, int(get16(row, 2)), psOutpostDist, psOutpostShip)
		}
	}
}

// sendNearestShip is Send_Nearest_Ship_ (0x9b2c7): after the confirmations,
// the chosen kind's nearest ship goes to settle planet pl.
func (g *Game) sendNearestShip(pl int) error {
	ps, s := &g.ps, &g.state
	p := int(s.PlayerNum)
	star := int(s.Planets[pl].Star)
	omni := s.playerIsOmniscient(p)
	k := -1
	for i := 0; i < int(ps.bar.rows) && k == -1; i++ {
		if int(get16(ps.rec(i), psPlanet)) == pl {
			k = i
		}
	}
	sh := int16(-1)
	switch ps.sendChoice {
	case 0:
		sh = get16(ps.rec(k), psColonyShip)
	case 1:
		sh = get16(ps.rec(k), psOutpostShip)
	}
	d := &s.Ships[sh]
	typ := d.Design.Type
	from := int(absoluteLocation(d.Location))
	starName := save.CString(s.Stars[star].Name[:])
	shipName := save.CString(d.Design.Name[:])
	ok := true
	if (from != star && typ == 1 && int8(ps.rec(k)[psColonyETA]) > 0) || (typ == 4 && int8(ps.rec(k)[psOutpostETA]) > 0) {
		g.e.SetMouseList(engine.MouseArrow)
		var err error
		if ok, err = g.confirmationBox(cSprintf(g.hMessage(0x134), starName)); err != nil {
			return err
		}
		ps.refields = true
		if !ok {
			return nil
		}
	}
	if sh < 0 || d.Location < 500 {
		if sh == -1 {
			ok = false
		}
	} else {
		vFrom := s.playerHasVisited(p, from)
		vTo := s.playerHasVisited(p, star)
		if from == star {
			if err := g.messageBox(cSprintf(g.hMessage(0x138), shipName, starName)); err != nil {
				return err
			}
			ps.refields = true
			return nil
		}
		fromName := save.CString(s.Stars[from].Name[:])
		var msg string
		switch {
		case omni || (vTo && vFrom):
			msg = cSprintf(g.hMessage(0x139), shipName, fromName, starName)
		case !vTo && !vFrom:
			msg = cSprintf(g.hMessage(0x13a), shipName)
		case !vTo:
			msg = cSprintf(g.hMessage(0x139), shipName, fromName)
		default:
			msg = cSprintf(g.hMessage(0x13a), shipName, starName)
		}
		g.e.SetMouseList(engine.MouseArrow)
		var err error
		if ok, err = g.confirmationBox(msg); err != nil {
			return err
		}
		ps.refields = true
	}
	if !ok {
		return nil
	}
	if ps.sendChoice == 1 {
		sh = get16(ps.rec(k), psOutpostShip)
	} else {
		sh = get16(ps.rec(k), psColonyShip)
	}
	ids := []int16{sh, -1}
	info := g.shipsTryToMoveTo(ids, star)
	if !info.canMove {
		if info.flux {
			if err := g.warningBox(g.hMessage(0x13f)); err != nil {
				return err
			}
			ps.refields = true
		}
		return nil
	}
	go_ := true
	if owner, guarded := g.starGuardedByMonster(star); guarded {
		g.e.SetMouseList(engine.MouseArrow)
		h := 0x13b
		if ps.sendChoice == 0 {
			h = 0x13c
		}
		var err error
		if go_, err = g.confirmationBox(cSprintf(g.hMessage(h), starName, g.raceName(owner))); err != nil {
			return err
		}
	}
	if !go_ {
		return nil
	}
	g.makeShipsMoveTo(ids, star, &info)
	h := 0x13e
	if info.turns == 1 {
		h = 0x13d
	}
	if err := g.messageBox(cSprintf(g.hMessage(h), save.CString(s.Ships[sh].Design.Name[:]), starName, int(uint8(info.turns)))); err != nil {
		return err
	}
	ps.lastShipMoved = sh
	ps.refields = true
	g.updateShipEtas()
	// Set_Auto_Colonize_Flag_ (0x99522).
	s.Ships[sh].Design.Weapons[1].Weapon = 1
	s.Ships[sh].Design.Weapons[0].Weapon = int16(pl)
	return nil
}

// undoSendOrder is Undo_Send_Order_ (0x9cd24): offer to call back the
// ship of kind (1 colony, 4 outpost) sent to planet pl.
func (g *Game) undoSendOrder(pl, kind int) error {
	ps, s := &g.ps, &g.state
	sh := int16(-1)
	for i := 0; i < int(ps.bar.rows); i++ {
		r := ps.rec(i)
		if int(get16(r, psPlanet)) != pl {
			continue
		}
		sh = get16(r, psOutpostShip)
		h := 0x14a
		if kind == 1 {
			sh, h = get16(r, psColonyShip), 0x149
		}
		// port-only: no ship (-1) is one that cannot be selected; the
		// original reads the record before Ships for it.
		if sh < 0 || !g.shipCanBeSelected(int(sh)) {
			if err := g.warningBox(cSprintf(g.hMessage(h), g.planetNameNoPrime(pl))); err != nil {
				return err
			}
		}
		break
	}
	if sh == -1 {
		return nil
	}
	ids := []int16{sh, -1}
	dest := g.xyToStarID(s.Ships[sh].X, s.Ships[sh].Y)
	if dest == -1 {
		if dest = g.nearestStarToXY(ids); dest == -1 {
			dest = int(s.Planets[s.Players[s.PlayerNum].HomePlanet].Star)
		}
	}
	if dest == -1 {
		return nil
	}
	g.fb.moveInfo = g.shipsTryToMoveTo(ids, dest)
	t := g.fb.moveInfo.turns
	h := 0x14c
	if t == 1 {
		h = 0x14b
	}
	eta := cSprintf(g.hMessage(h), int(uint8(t)))
	if t == 0 {
		eta = ""
	}
	h = 0x14e
	if kind == 1 {
		h = 0x14d
	}
	ok, err := g.confirmationBox(cSprintf(g.hMessage(h), g.planetNameNoPrime(pl), save.CString(s.Stars[dest].Name[:]), eta))
	if err != nil {
		return err
	}
	if ok {
		g.makeShipsMoveTo(ids, dest, &g.fb.moveInfo)
		g.updateShipEtas()
		s.clearAutoColonizationFlag(int(sh))
	}
	ps.refields = true
	return nil
}

// nearestStarToXY is Nearest_Star_To_XY_ (0x78d86): the player's
// unblockaded colony star the ships reach soonest.
func (g *Game) nearestStarToXY(ids []int16) int {
	s := &g.state
	p := int(s.PlayerNum)
	best, at := int16(0xff), -1
	for i := 0; i < int(s.NumStars); i++ {
		if !s.playerHasColony(p, i) || s.Stars[i].Blockaded>>p&1 != 0 {
			continue
		}
		info := g.shipsTryToMoveTo(ids, i)
		if info.canMove && int16(uint8(info.turns)) < best {
			best, at = int16(uint8(info.turns)), i
		}
	}
	return at
}

// getETAToStar is Get_ETA_To_Star_ (0x9a5e8): the turns the higher-numbered
// of the first listed planet's two ships needs to reach star.
func (g *Game) getETAToStar(star int) int16 {
	ps := &g.ps
	sh := int16(-1)
	for i := 0; i < int(ps.bar.rows) && sh == -1; i++ {
		r := ps.rec(i)
		if int(g.state.Planets[get16(r, psPlanet)].Star) == int(uint16(star)) {
			sh = max(get16(r, psColonyShip), get16(r, psOutpostShip))
		}
	}
	if sh == -1 {
		return 0
	}
	return g.nTurnsToStar(int(sh), star)
}

// playerColors is Player_Colors_ (0x99f38) through Colony_Owner_Colors_
// (0x9a04b): a row's colours by owner (-1 none) and mode (0 plain, 1
// hovered, 2 hovered but out of reach).
func (g *Game) playerColors(owner int, mode uint8) font.Colors {
	if owner != -1 {
		base := g.state.Players[owner].Color*6 - 0x31
		switch mode {
		case 0:
			g.moxFontColorsOffset = base + 2
			return g.fontColors2(4, base, base+4)
		case 1:
			g.moxFontColorsOffset = base + 3
			return g.fontColors2(4, base, base+5)
		default:
			g.moxFontColorsOffset = base + 1
			return g.fontColors2(4, base, base+3)
		}
	}
	switch mode {
	case 1:
		g.moxFontColorsOffset = 0x42
		return g.fontColors2(4, 0x40, 0xff)
	case 2:
		g.moxFontColorsOffset = 0x40
		return g.fontColors2(4, 0, 0x8a)
	}
	g.moxFontColorsOffset = 0x43
	return g.fontColors2(4, 0x40, 0x41)
}

// specialIsPlanetSpecial is Special_Is_Planet_Special_ (0x78c5b).
func specialIsPlanetSpecial(sp int) bool {
	return sp == 4 || sp == 5 || sp == 6 || sp == 7 || sp == 10
}

// summaryScreenPrintData is Summary_Screen_Print_Data_ (0x9bf70): the
// eight visible planet rows.
func (g *Game) summaryScreenPrintData() error {
	e, ps, s := g.e, &g.ps, &g.state
	t, v := e.Text(), e.V
	b := &ps.bar
	end := min(int(b.rows)+int(b.first), int(b.first)+8)
	for i := 0; end-int(b.first) > i; i++ {
		k := int(b.first) + i
		dy := int(g.exeI16(plntscrnDeltaY + 2*uint32(i)))
		y := dy + 0x27
		r := ps.rec(k)
		pl := int(get16(r, psPlanet))
		if pl == -1 {
			continue
		}
		P := &s.Planets[pl]
		star := int(P.Star)
		mode := uint8(0)
		if ps.sendChoice == 0 || ps.sendChoice == 1 {
			switch {
			case int(ps.boxScanned) == i && ps.canReachScanned:
				mode = 1
			case int(ps.boxScanned) == i:
				mode = 2
			}
		} else if int(ps.boxScanned) == i {
			mode = 1
		}
		owner := -1
		if P.Colony != -1 {
			owner = int(s.Colonies[P.Colony].Owner)
		}
		c := g.playerColors(owner, mode)
		// The window is one row taller per row: it reads the next row's top.
		v.SetWindow(0x12, y, 0x68, int(int16(y+int(g.exeI16(plntscrnDeltaY+2+2*uint32(i))))))
		v.ClipOn()
		t.SetStyle(1, c)
		sp := 0
		if x := int8(s.Stars[star].Special); x > 0 && x != 0xb {
			sp = int(x)
		}
		if sp > 0 && specialIsPlanetSpecial(sp) && int(int8(P.Special)) == sp {
			// Star special 9 would name the first ship orbiting; 9 is not a
			// planet special, so that branch never runs.
			if err := t.PrintCentered(0x3d, y, g.psEString(planetSpecialNames, sp)); err != nil {
				return err
			}
		}
		t.SetStyle(2, c)
		mid := (0x32-2*t.Height())/2 - 4
		name := g.planetNameNoPrime(pl)
		g.setFittedFontStyle(name, c, 2, 2, 0, 0x56, 1)
		ny := y + 0x32
		if err := t.PrintCentered(0x3d, int(int16(ny-2*t.Height())), name); err != nil {
			return err
		}
		label := ""
		if owner != -1 {
			label = save.CString(s.Players[owner].RaceName[:])
		} else if o, ok := g.starGuardedByMonster(star); ok {
			label = g.raceName(o)
		}
		if owner != -1 || label != "" {
			t.SetStyle(1, c)
			if err := t.PrintCentered(0x3d, int(int16(ny-t.Height()-2)), cSprintf(g.psString(plntsumOwnerFmt), label)); err != nil {
				return err
			}
		}
		v.ClipOff()
		v.ResetWindow()
		y2 := y + mid
		climate := g.psEString(planetClimateNames, int(get16(r, psClimate)))
		g.setFittedFontStyle(climate, c, 2, 2, -1, 0x41, 1)
		fh := t.Height()
		if err := t.PrintCentered(0x8c, int(int16(y2)), climate); err != nil {
			return err
		}
		t.SetStyle(1, c)
		if err := t.PrintCentered(0x8c, int(int16(y2+fh+4)), cSprintf(g.hMessage(0x141), int(get16(r, psFood)))); err != nil {
			return err
		}
		grav := g.psEString(planetGravityNames, int(P.Gravity))
		t.SetStyle(2, c)
		fh = t.Height()
		if pen := g.productivityPenalty(int(s.PlayerNum), int(P.Gravity)); pen == 0 {
			if err := t.PrintCentered(0xd9, int(int16(t.Height()/2+y2)), grav); err != nil {
				return err
			}
		} else {
			if err := t.PrintCentered(0xd9, int(int16(y2)), grav); err != nil {
				return err
			}
			t.SetStyle(1, c)
			if err := t.PrintCentered(0xd9, int(int16(fh+y2+4)), cSprintf(g.hMessage(0x142), int(pen))); err != nil {
				return err
			}
		}
		t.SetStyle(2, c)
		if err := t.PrintCentered(0x137, int(int16(y2)), g.psEString(planetMineralNames, int(get16(r, psMineralCl)))); err != nil {
			return err
		}
		under := int(int16(t.Height() + y2 + 4))
		t.SetStyle(1, c)
		if err := t.PrintCentered(0x137, under, cSprintf(g.hMessage(0x152), int(get16(r, psMinerals)))); err != nil {
			return err
		}
		t.SetStyle(2, c)
		under = int(int16(t.Height() + y2 + 4))
		if err := t.PrintCentered(0x182, int(int16(y2)), g.psEString(planetSizeNames, int(P.Size))); err != nil {
			return err
		}
		t.SetStyle(1, c)
		if err := t.PrintCentered(0x182, under, cSprintf(g.hMessage(0x143), int(get16(r, psMaxPop)))); err != nil {
			return err
		}
		icon := ps.pics[psShipIcon]
		if int8(r[psColonyETA]) >= 0 {
			if err := e.Draw(0x66-icon.Width, dy+0x54-icon.Height, icon); err != nil {
				return err
			}
		}
		if int8(r[psOutpostETA]) >= 0 {
			if err := e.Draw(0x12, dy+0x54-icon.Height, icon); err != nil {
				return err
			}
		}
	}
	return nil
}

// summaryScreenPrintScanned is Summary_Screen_Print_Scanned_ (0x9c600):
// the line under the small map about the scanned star.
func (g *Game) summaryScreenPrintScanned() error {
	ps, s := &g.ps, &g.state
	t := g.e.Text()
	sf := g.ms.scannedField
	p := int(s.PlayerNum)
	k := g.summaryPlanetIndex(int(sf))
	if ps.rec(k)[psUnreachable] == 1 {
		msg := cSprintf(g.hMessage(0x144), save.CString(s.Stars[sf].Name[:]))
		t.SetStyle(1, g.psColors(plntsumRed))
		return t.PrintCentered(0x215, t.Height()/2+0x8f, msg)
	}
	choosing := ps.sendChoice == 0 || ps.sendChoice == 1
	var msg string
	col := 0
	if sf >= 1000 && int(sf) < int(s.NumStars)+1000 {
		star := int(sf - 1000)
		seen := s.playerHasVisited(p, star) || s.playerIsOmniscient(p)
		name := save.CString(s.Stars[star].Name[:])
		switch {
		case s.Stars[star].SpectralClass == save.SpectralBlackHole:
			msg, col = g.hMessage(0x145), 1
		case seen:
			eta := ""
			if choosing {
				if !ps.reachStar[star] {
					eta, col = g.hMessage(0x146), 1
				} else {
					if n := g.getETAToStar(star); n > 0 {
						h := 0x151
						if n == 1 {
							h = 0x150
						}
						eta = cSprintf(g.hMessage(h), int(n))
					}
					col = 2
				}
			}
			msg = cSprintf(g.psString(plntsumScanFmt), eta, name)
		case !choosing:
			msg = g.hMessage(0x94)
		case !ps.reachStar[star]:
			msg, col = g.hMessage(0x147), 1
		default:
			// With no ETA the original prints its uninitialised buffer.
			if n := g.getETAToStar(star); n > 0 {
				h := 0x14c
				if n == 1 {
					h = 0x14b
				}
				msg = cSprintf(g.hMessage(h), int(n))
			}
			col = 2
		}
	} else {
		if !choosing {
			return nil
		}
		star := int(sf)
		seen := s.playerHasVisited(p, star) || s.playerIsOmniscient(p)
		mi := &g.fb.moveInfo
		switch {
		case !mi.canMove:
			msg, col = g.hMessage(0x148), 1
		case seen:
			h := 0x6a
			if mi.turns == 1 {
				h = 0x69
			}
			msg, col = cSprintf(g.hMessage(h), int(uint8(mi.turns)), save.CString(s.Stars[star].Name[:])), 2
		default:
			msg, col = cSprintf(g.hMessage(0x14c), int(uint8(mi.turns))), 2
		}
	}
	c := g.psColors(plntsumNeutral)
	switch col {
	case 1:
		c = g.psColors(plntsumRed)
	case 2:
		c = g.psColors(plntsumGreen)
	}
	t.SetStyle(2, c)
	return t.PrintCentered(0x215, (t.Height()+2)/2+0x8f, msg)
}
