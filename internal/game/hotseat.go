package game

import (
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// Hotseat play: the hand-over picker (screen 0x11) and the per-player map
// state. Spec: docs/screens/hotseat.md.

const (
	multiGMLBX       = "MULTIGM.LBX" // 0x17a055
	hotseatColorBase = 0xefa52       // Hotseat_Get_Colors_'s 8 shade offsets
)

// hotseatBox is one 8-byte _hotseat_system_box / _hotseat_fleet_box record.
type hotseatBox struct {
	x, y, content int16
	ship          int16 // fleet box only: the ship of the box's stack
}

// hotseat holds the hotseat globals and _g_mp_screen (0x1aacb4) as the
// picker uses it.
type hotseat struct {
	systemBox [8]hotseatBox // _hotseat_system_box (0x19c0a0)
	fleetBox  [8]hotseatBox // _hotseat_fleet_box (0x19c0e0)
	mapScale  [8]byte       // _hotseat_map_scale (0x19c120)
	humans    int16         // _hotseat_human_players (0x19b880)
	mp        *mpScreen     // _mp_screen, nil outside the screens
}

// mpScreen is the part of _g_mp_screen the hotseat picker uses.
type mpScreen struct {
	back, top, mid, bottom *gfx.Picture    // +0xf, +0x23, +0x27, +0x2b
	banner                 [8]*gfx.Picture // +0x33
	bannerField, nameField [8]int          // +0xa7, +0xbb
	x, y                   int16           // +0xf3, +0xf5
	input                  int             // +0x101
	fontMode               byte            // +0x10e
	status                 [8]byte         // +0x110: 0 none, 1 done, 2 to play
	colors, shades         [8]font.Colors  // +0x118, +0x198
	cycle                  font.Colors     // +0x1d8
	nValid                 int16           // +0x1e1
	nHumans, lastHuman     int16           // +0x1e3, +0x1e5
	allDone                bool            // +0x1e7
	cyclePos, cycleDir     int8            // +0x1e8, +0x1e9

	// The network turn's waiting screen (Load_Net_Next_Turn_Screen_).
	fill                   *gfx.Picture   // +0x93: the ready plate
	chatField, hiddenField int            // +0xbb, +0xbd
	boxH                   int16          // +0xbf: the top and middle pictures' height
	chatOn                 bool           // +0x10c: the chat line is active
	lights                 [8]font.Colors // +0x158
	first                  bool           // +0x1e0: draw the box once more
	frame                  uint8          // +0x1ea: the animation's frame count

	// The network players' picks (Choose_Network_Plyrs_Screen_).
	startPic, cancelPic *gfx.Picture // +0x8f, +0x73
	startF              int          // +0xed
	canStart            bool         // +0x103: START is offered
	chosen              [8]byte      // +0x104: the empire is taken
}

// hotseatDefaults is Harold_Defaults_'s hotseat part (0x79386..0x793e5).
func (g *Game) hotseatDefaults() {
	h := &g.hs
	if g.gameType == save.GameHotseat {
		h.fleetBox = [8]hotseatBox{}
		h.systemBox = [8]hotseatBox{}
		h.mapScale = [8]byte{}
	}
	for i := range h.systemBox {
		h.systemBox[i].content = g.ms.boxOffFlag[0]
	}
}

// saveHotseatMapInfo is Save_Hotseat_Map_Info_ (0x88f5d): the current
// player's system and fleet boxes and map scale.
func (g *Game) saveHotseatMapInfo() {
	s, h := &g.state, &g.hs
	p := int(s.PlayerNum)
	b0, b2 := &s.MoveableBoxes[0], &s.MoveableBoxes[2]
	h.systemBox[p].x, h.systemBox[p].y, h.systemBox[p].content = b0.X, b0.Y, b0.Content
	h.fleetBox[p].x, h.fleetBox[p].y, h.fleetBox[p].content = b2.X, b2.Y, b2.Content
	g.saveHotseatFleetBoxShip(p)
	h.mapScale[p] = byte(s.CurMapScale)
}

// saveHotseatFleetBoxShip is Save_Hotseat_Fleet_Box_Ship_ (0x7872e): the
// ship of the fleet box's first icon, or -1 with the box closed.
func (g *Game) saveHotseatFleetBoxShip(p int) {
	if g.moveableBoxSelected(2) {
		g.hs.fleetBox[p].ship = g.stacks.node[g.fb.icon[0].node].ship
		return
	}
	g.hs.fleetBox[p].ship = -1
}

// restoreHotseatMapInfo is Restore_Hotseat_Map_Info_ (0x88e5a).
func (g *Game) restoreHotseatMapInfo() {
	s, h := &g.state, &g.hs
	p := int(s.PlayerNum)
	b0, b2 := &s.MoveableBoxes[0], &s.MoveableBoxes[2]
	b0.X = h.systemBox[p].x
	if b0.X <= 0x15 {
		b0.X = 0x16
	}
	b0.Y = h.systemBox[p].y
	if b0.Y <= 0x15 {
		b0.Y = 0x16
	}
	b0.Content = h.systemBox[p].content
	b2.X = h.fleetBox[p].x
	if b2.X <= 0x15 {
		b2.X = 0x16
	}
	// The original tests y against 0x71 but resets it to 0x16.
	b2.Y = h.fleetBox[p].y
	if b2.Y <= 0x71 {
		b2.Y = 0x16
	}
	b2.Content = h.fleetBox[p].content
	if g.moveableBoxSelected(2) {
		g.setHotseatFleetBoxToLastStack(p)
	}
	s.CurMapScale = int16(h.mapScale[p])
	if s.CurMapScale == 0 {
		s.CurMapScale = s.MaxMapScale
	}
	g.ms.zoomCount = int16(g.mapScaleToZoomLevel())
}

// setHotseatFleetBoxToLastStack is Set_Hotseat_Fleet_Box_To_Last_Stack_
// (0x78770): reopen p's fleet box on the stack of its saved ship.
func (g *Game) setHotseatFleetBoxToLastStack(p int) {
	s, m := &g.state, &g.ms
	ship := int(g.hs.fleetBox[p].ship)
	g.updateShipIconsInBetweenTurns()
	if ship > -1 && g.shipOwner(ship) == p && int8(s.Ships[ship].Status) < 3 {
		g.fb.stack = int16(g.shipStackFromShipID(ship))
	} else {
		g.fb.stack = -1
	}
	if g.fb.stack == -1 {
		g.vanishFleetMovementBox()
		m.savedBoxWasOpen[2] = false
		m.savedBoxes[2] = m.boxOffFlag[2]
	}
}

// hotseatPlayerStatus is Hotseat_Player_Status_ (0xf3e09): 0 not a living
// human, 1 done with the turn, 2 still to play.
func (g *Game) hotseatPlayerStatus(p int) byte {
	pl := &g.state.Players[p]
	if pl.Objective != save.ObjectiveHuman || pl.Eliminated != 0 {
		return 0
	}
	if pl.TurnDone != 0 {
		return 1
	}
	return 2
}

// hotseatSetNextPlayerStatus is Hotseat_Set_Next_Player_Status_ (0xf3d67).
func (g *Game) hotseatSetNextPlayerStatus() {
	s, mp := &g.state, g.hs.mp
	done := int16(0)
	if s.NumPlayers <= 0 {
		mp.allDone = mp.nHumans == 0
		return
	}
	for p := 0; p < 8; p++ {
		mp.status[p] = g.hotseatPlayerStatus(p)
		pl := &s.Players[p]
		if pl.Objective == save.ObjectiveHuman && pl.Eliminated == 0 {
			mp.lastHuman = int16(p)
			mp.nHumans++
		}
		if mp.status[p] == 1 && pl.Eliminated == 0 {
			done++
		}
	}
	mp.allDone = done == mp.nHumans
}

// hotseatGetNValidPlayers is Hotseat_Get_N_Valid_Players_ (0xf38dc).
func (g *Game) hotseatGetNValidPlayers() int16 {
	n := int16(0)
	for _, st := range g.hs.mp.status {
		if st == 1 || st == 2 {
			n++
		}
	}
	return n
}

// hotseatGetColors is Hotseat_Get_Colors_ (0xf37fc).
func (g *Game) hotseatGetColors() {
	s, mp := &g.state, g.hs.mp
	tbl := g.exe.bytes(hotseatColorBase, 8)
	for p := 0; p < int(s.NumPlayers); p++ {
		base := int16(s.Players[p].Color)*6 + 0x70
		for k := 0; k < 8; k++ {
			c := byte(base) + tbl[k]
			mp.colors[p][k] = c
			if k == 0 {
				mp.shades[p][0] = byte(base)
				continue
			}
			sh := int(base) + 1
			if sh < int(c)-3 {
				sh = int(c) - 3
			}
			mp.shades[p][k] = byte(sh)
		}
	}
}

// hotseatLoadNextPlayer is Hotseat_Load_Next_Player_ (0xf390e).
func (g *Game) hotseatLoadNextPlayer() error {
	mp := &mpScreen{}
	g.hs.mp = mp
	var err error
	if mp.back, err = g.picture(multiGMLBX, 0); err != nil {
		return err
	}
	if mp.top, err = g.picture(multiGMLBX, 0x13); err != nil {
		return err
	}
	topH := int16(mp.top.Height)
	mp.x = (0x280 - int16(mp.top.Width)) / 2
	if mp.mid, err = g.picture(multiGMLBX, 0x14); err != nil {
		return err
	}
	midH := int16(mp.mid.Height)
	if mp.bottom, err = g.picture(multiGMLBX, 0x15); err != nil {
		return err
	}
	botH := int16(mp.bottom.Height)
	g.hotseatSetNextPlayerStatus()
	mp.nValid = g.hotseatGetNValidPlayers()
	y := (0x1e0 - (mp.nValid*midH + topH + botH)) / 2
	if y <= 0 {
		y = 0
	}
	mp.y = y
	g.hotseatGetColors()
	return nil
}

// hotseatAddNextPlayerFields is Hotseat_Add_Next_Player_Fields_ (0xf33ee):
// a name field and a banner field for each human still in the game.
func (g *Game) hotseatAddNextPlayerFields() error {
	s, mp := &g.state, g.hs.mp
	yoff, row := int16(0x40), 0
	for p := 0; p < int(s.NumPlayers); p++ {
		pl := &s.Players[p]
		if pl.Objective != save.ObjectiveHuman {
			continue
		}
		if pl.Eliminated != 0 {
			mp.nameField[p], mp.bannerField[p] = -1000, -1000
			continue
		}
		if mp.status[p] != 1 && mp.status[p] != 2 {
			continue
		}
		y := mp.y + yoff
		f, err := g.e.AddHiddenField(int(mp.x+0x6a), int(y), int(mp.x+0x1b3), int(y+0x1d), "", 0x29)
		if err != nil {
			return err
		}
		yoff += 0x24
		mp.nameField[p] = f
		if err := g.addBannerField(p, row); err != nil {
			return err
		}
		row++
	}
	return nil
}

// addBannerField is Add_Banner_Field_ (0xefaba).
func (g *Game) addBannerField(p, row int) error {
	pl, mp := &g.state.Players[p], g.hs.mp
	y := int(mp.y) + row*0x24 + 0x41
	pic, err := g.picture(multiGMLBX, int(pl.Race)*8+0x2e+int(pl.Color))
	if err != nil {
		return err
	}
	x := int(mp.x) + 0x28
	f, err := g.e.AddHiddenField(x, y, x+pic.Width, y+pic.Height, "", 0x29)
	if err != nil {
		return err
	}
	mp.bannerField[p] = f
	return nil
}

// drawMultiPlayerBanner is Draw_Multi_Player_Banner_ (0xf27b7); grid
// lays the banners out two per row (the network screens).
func (g *Game) drawMultiPlayerBanner(p, row int, grid bool) error {
	pl, mp := &g.state.Players[p], g.hs.mp
	entry := int(pl.Race)*8 + 0x2e + int(pl.Color)
	x, y := int(mp.x)+0x28, int(mp.y)+row*0x24+0x41
	if grid {
		// Placed by the player, not by his row (0xf2845): a human after an
		// AI keeps his own box.
		entry = int(pl.Race)*8 + 0x96 + int(pl.Color)
		x = p%2*0x126 + int(mp.x) + 0x28
		y = p/2*0x24 + 0x41 + int(mp.y)
	}
	pic, err := g.picture(multiGMLBX, entry)
	if err != nil {
		return err
	}
	mp.banner[p] = pic
	return g.e.Draw(x, y, pic)
}

// getPlayerColors is Get_Player_Colors_ (0xf3119): three font colour sets
// of banner colour c in _mp_screen's font mode.
func (g *Game) getPlayerColors(c byte) (bright, dim, light font.Colors) {
	base := byte(c)*6 + 0x70
	mode := int(g.hs.mp.fontMode)
	bright = g.fontColors2(mode, base, base+4)
	dim = g.fontColors2(mode, base, base+2)
	light = g.fontColors2(mode, base, base+5)
	return
}

// hotseatDrawNextPlayer is Hotseat_Draw_Next_Player_ (0xf34ea).
func (g *Game) hotseatDrawNextPlayer() error {
	s, e, mp := &g.state, g.e, g.hs.mp
	x, y := int(mp.x), int(mp.y)
	savedMode := mp.fontMode
	var err error
	if mp.back, err = g.picture(multiGMLBX, 0); err != nil {
		return err
	}
	if err := e.Draw(0, 0, mp.back); err != nil {
		return err
	}
	if mp.top, err = g.picture(multiGMLBX, 0x13); err != nil {
		return err
	}
	if err := e.Draw(x, y, mp.top); err != nil {
		return err
	}
	y += mp.top.Height
	if mp.mid, err = g.picture(multiGMLBX, 0x14); err != nil {
		return err
	}
	for i := 0; int(mp.nValid)-1 > i; i++ {
		if err := e.Draw(x, y, mp.mid); err != nil {
			return err
		}
		y += mp.mid.Height
	}
	if mp.bottom, err = g.picture(multiGMLBX, 0x15); err != nil {
		return err
	}
	if err := e.Draw(x, y, mp.bottom); err != nil {
		return err
	}
	row := 0
	mp.fontMode = 4
	t := e.Text()
	for p := 0; p < 8; p++ {
		bright, dim, _ := g.getPlayerColors(s.Players[p].Color)
		switch mp.status[p] {
		case 1:
			t.SetStyle(4, dim)
		case 2:
			if err := g.drawMultiPlayerBanner(p, row, false); err != nil {
				return err
			}
			if mp.input == mp.nameField[p] || mp.input == mp.bannerField[p] {
				g.hotseatCycleColors(s.Players[p].Color)
				t.SetStyle(4, mp.cycle)
			} else {
				t.SetStyle(4, bright)
			}
		default:
			continue
		}
		t.SetSpacing(1)
		if err := g.hotseatPrintPlayerData(p, row); err != nil {
			return err
		}
		row++
	}
	mp.fontMode = savedMode
	return nil
}

// hotseatCycleColors is the pulse of the hovered player's line in
// Hotseat_Draw_Next_Player_ (0xf369d..0xf37a1).
func (g *Game) hotseatCycleColors(c byte) {
	mp := g.hs.mp
	base := int(c)*6 + 0x70
	mp.cycle = g.fontColors2(4, byte(base), byte(base+3))
	hi, lo := byte(base+5), byte(base+1)
	for k := 1; k < 8; k++ {
		mp.cycle[k] += byte(mp.cyclePos)
		if int(mp.cycle[k]) < base+1 {
			mp.cycle[k] = lo
		}
		if int(mp.cycle[k]) > base+5 {
			mp.cycle[k] = hi
		}
	}
	mp.cyclePos += mp.cycleDir
	if mp.cyclePos >= 2 {
		mp.cyclePos, mp.cycleDir = 2, -1
	} else if mp.cyclePos <= -2 {
		mp.cyclePos, mp.cycleDir = -2, 1
	}
}

// hotseatPrintPlayerData is Hotseat_Print_Player_Data_ (0xf3a27): name,
// race and score on the player's line.
func (g *Game) hotseatPrintPlayerData(p, row int) error {
	pl, mp := &g.state.Players[p], g.hs.mp
	x := int(mp.x) + 0x6a
	y := int(mp.y) + row*0x24 + 0x40 + 8
	h := hiScore{modifierPlayer: uint8(p)}
	if err := g.calculateHiScores(p, &h); err != nil { // Player_Score_
		return err
	}
	str := cSprintf(g.hMessage(0xfc), save.CString(pl.EmperorName[:]), save.CString(pl.RaceName[:]), commaString(int32(h.final)))
	return g.e.Text().Print(x+0xa, y, str)
}

// hotseatSelectPlayer is Hotseat_Select_Player_ (0xf3ae4), screen 0x11:
// the next human picks his banner; when all are done the turn runs.
func (g *Game) hotseatSelectPlayer() error {
	e, s := g.e, &g.state
	blank := func() error {
		e.V.Fill(0, 0, 0x27f, 0x1df, 0) // Fade_In_Function_
		return nil
	}
	if err := g.fadeIntoScreen(blank, 0); err != nil {
		return err
	}
	e.AssignAutoFunction(g.hotseatDrawNextPlayer, 2)
	e.ClearFields()
	e.SetInputDelay(3)
	if err := g.hotseatLoadNextPlayer(); err != nil {
		return err
	}
	mp := g.hs.mp
	e.SetFieldOutlineColor(0xf0)
	allDone, picked, last := false, int16(-1), -1
	mp.cyclePos, mp.cycleDir = 0, 1
	done := false
	switch {
	case mp.nHumans == 1:
		g.gameType = save.GameSingle
		done, picked = true, mp.lastHuman
	case mp.allDone:
		allDone, done = true, true
	default:
		if err := g.hotseatAddNextPlayerFields(); err != nil {
			return err
		}
		if err := g.fadeIntoScreen(g.hotseatDrawNextPlayer, 0xb); err != nil {
			return err
		}
	}
	for !done {
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		if mp.input, err = e.ScanInput(); err != nil {
			return err
		}
		for p := 0; p < int(s.NumPlayers) && !done; p++ {
			if mp.status[p] == 2 && (in == mp.nameField[p] || in == mp.bannerField[p]) {
				done = true
				g.ms.forceFade = true
				picked = int16(p)
			}
			if mp.status[p] == 2 && (mp.input == mp.nameField[p] || mp.input == mp.bannerField[p]) {
				if last != mp.input {
					mp.cyclePos, mp.cycleDir = 0, 1
				}
				last = mp.input
			}
		}
		if done {
			break
		}
		e.V.SetPageOff()
		if err := g.hotseatDrawNextPlayer(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	e.ClearFields()
	e.SetInputDelay(3)
	e.DeactivateAutoFunction()
	g.hs.mp = nil
	if allDone {
		g.currentScreen, g.returnScreen = ScreenNextTurn, ScreenHotseat
	} else {
		g.restoreHotseatMapInfo()
		if picked != -1 {
			s.PlayerNum = picked
		}
		g.currentScreen, g.returnScreen = ScreenReports, ScreenHotseat
		g.restoreHotseatMapInfo()
		st := &s.Stars[s.Planets[s.Players[s.PlayerNum].HomePlanet].Star]
		g.centerMap(st.X, st.Y)
	}
	r, err := g.mustDisplayReports(int(s.PlayerNum))
	if err != nil || byte(r) == 0 {
		return err
	}
	g.updateShipIconsInBetweenTurns()
	g.ms.stardateAtBegOfTurn = s.Stardate
	return g.fadeIntoMainScreen()
}
